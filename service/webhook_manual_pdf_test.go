package service

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebhookManualRefusesBrokenConfiguredBrandInsteadOfDefaulting(t *testing.T) {
	transport := configureBillingBranding(t)
	common.OperatingEntityName = "示例运营有限公司"
	common.OperatingEntityLogo = "https://platform.example/operator.png"
	_, err := RenderWebhookManualPDF(context.Background(), WebhookManualAssets)
	require.NoError(t, err)
	require.Len(t, transport.requests, 1)
	assert.Equal(t, common.OperatingEntityLogo, transport.requests[0].URL.String())
	transport.status = http.StatusNotFound
	_, err = RenderWebhookManualPDF(context.Background(), WebhookManualAssets)
	require.ErrorContains(t, err, "404")
}

func TestWebhookManualValidatesLetterheadAndFooterRenderingBounds(t *testing.T) {
	configureBillingBranding(t)
	for _, name := range []string{"unsupported name \U0010FFFF", strings.Repeat("超长平台名称", 20)} {
		common.SystemName = name
		_, err := RenderWebhookManualPDF(context.Background(), WebhookManualAssets)
		require.Error(t, err, "unrenderable branding %q must not silently produce a clipped or missing-glyph PDF", name)
	}
	common.SystemName = "示例服务平台"
	common.Footer = strings.Repeat("长", 300)
	_, err := RenderWebhookManualPDF(context.Background(), WebhookManualAssets)
	require.Error(t, err, "a footer exceeding page space must not produce clipped text")
}

func TestWebhookManualScopesAndJSONExamples(t *testing.T) {
	for _, test := range []struct {
		scope     WebhookManualScope
		required  []string
		forbidden []string
	}{
		{WebhookManualAssets, []string{"asset.active", "asset.failed", "real_person", "sensitive_content", "policy_rejected", "Active", "Failed", "HTTP 2xx", "state_version"}, []string{"task.status_changed", "media_tasks", "Suno", "IN_PROGRESS"}},
		{WebhookManualTasks, []string{"task.status_changed", "NOT_START", "SUBMITTED", "QUEUED", "IN_PROGRESS", "SUCCESS", "FAILURE", "UNKNOWN", "response_omitted", "HTTP 2xx", "state_version"}, []string{"asset.active", "asset.failed", "asset_library", "real_person"}},
	} {
		t.Run(string(test.scope), func(t *testing.T) {
			pages := webhookManualPages(test.scope)
			require.NotEmpty(t, pages)
			var content strings.Builder
			jsonExamples := 0
			for _, page := range pages {
				content.WriteString(page.Title)
				for _, section := range page.Sections {
					content.WriteString(section.Title)
					for _, body := range section.Body {
						content.WriteString(body)
					}
					for _, row := range section.Rows {
						for _, cell := range row {
							content.WriteString(cell)
						}
					}
					if section.Code == "" {
						continue
					}
					content.WriteString(section.Code)
					var value map[string]any
					require.NoError(t, common.Unmarshal([]byte(section.Code), &value), "every example must be valid JSON")
					assert.Contains(t, section.Code, "\n  \"", "JSON examples must show fields on separate indented lines")
					jsonExamples++
				}
			}
			assert.Equal(t, 1, jsonExamples, "customer manual should only show the business callback JSON example")
			for _, value := range test.required {
				assert.Contains(t, content.String(), value)
			}
			for _, value := range test.forbidden {
				assert.NotContains(t, content.String(), value)
			}
			for _, internal := range []string{"个人端点管理 API", "端点响应与字段", "连通性测试事件", "连通性测试结果", "/api/webhook-endpoints", "webhook.test", "/api/webhooks/manual.pdf"} {
				assert.NotContains(t, content.String(), internal)
			}
		})
	}
}
