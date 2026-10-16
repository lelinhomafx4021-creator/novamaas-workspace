package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUsageAuditPDFPreservesModelCategoryCostsAndExistingLetterhead(t *testing.T) {
	statement, branding := frozenBillingExportFixture(t)
	branding.Issuer = "new-api 聚合平台"
	branding.CompanyTitle = "示例客户有限公司"
	report := &UsageAuditReport{RecalculationBasis: "legacy_openai_cache_write_tokens", Filter: model.UsageAuditFilter{UserID: 42, TokenID: 7, StartAt: statement.StartAt, EndAt: statement.EndAt}, GeneratedAt: statement.CreatedAt, Currency: branding.Currency, Timezone: "Asia/Shanghai"}
	for _, source := range []model.Log{
		{ModelName: "kimi-k3", Type: model.LogTypeConsume, PromptTokens: 4921, CompletionTokens: 86, Quota: 951, Group: "kimi", Other: `{"request_path":"/v1/messages","request_conversion":["Claude Messages","OpenAI Compatible"],"usage_semantic":"openai","cache_tokens":4825,"model_ratio":1.4285715,"group_ratio":0.66,"completion_ratio":4.99999965,"cache_ratio":0.1}`},
		{ModelName: "claude-test", Type: model.LogTypeConsume, PromptTokens: 100, CompletionTokens: 20, Quota: 405, Group: "standard", Other: `{"usage_semantic":"anthropic","cache_tokens":800,"cache_creation_tokens":100,"model_ratio":2,"group_ratio":0.5,"completion_ratio":5,"cache_ratio":0.1,"cache_creation_ratio":1.25}`},
	} {
		row, err := buildUsageAuditRow(&source, report.Currency)
		require.NoError(t, err)
		report.Rows = append(report.Rows, row)
		addUsageAuditRow(&report.Summary, row)
		group := UsageAuditGroup{Label: source.ModelName}
		addUsageAuditRow(&group.UsageAuditSummary, row)
		report.Models = append(report.Models, group)
	}
	report.Rates = []UsageAuditRate{{Model: "kimi-k3", Group: "kimi", Ratio: "0.66"}, {Model: "claude-test", Group: "standard", Ratio: "0.5"}}
	var err error
	report.SHA256, err = usageAuditReportDigest(report)
	require.NoError(t, err)
	body, err := RenderUsageAuditPDF(report, branding)
	require.NoError(t, err)
	cover := billingPDFPageText(t, body, 0)
	for _, text := range []string{"new-api 聚合平台", "Token 用量与费用核对单", "示例客户有限公司", "平台费用 / PLATFORM", "官价费用 / LIST PRICE", "费用差额 / DIFFERENCE", "0.018984", "0.031514", "0.012530", "全部模型分类汇总"} {
		assert.Contains(t, cover, text)
	}
	assert.Contains(t, cover, "缓存读取重算：1 笔 / 4825 Token")
	assert.Contains(t, cover, "cache_write_tokens")
	assert.Contains(t, cover, "有缓存写入的请求排除")
	assert.NotContains(t, cover, "实际记账金额")
	assert.NotContains(t, cover, "客户确认（盖章）")
	assert.Less(t, strings.Index(cover, "缓存读取"), strings.Index(cover, "平台费用 / PLATFORM"))
	assert.Contains(t, cover, "0.0026672")
	models := billingPDFPageText(t, body, 1)
	for _, text := range []string{"kimi-k3", "claude-test", "kimi × 0.66", "standard × 0.5", "4825", "800", "100", "普通输入（不含缓存）", "缓存读取", "缓存写入", "输出"} {
		assert.Contains(t, models, text)
	}
	assert.Contains(t, models, "0.0012672")
	if output := os.Getenv("USAGE_AUDIT_PREVIEW_DIR"); output != "" {
		require.NoError(t, os.MkdirAll(output, 0700))
		require.NoError(t, os.WriteFile(filepath.Join(output, "usage-audit-preview.pdf"), body, 0600))
	}
}

func TestUsageAuditPDFPaginatesModelDetailsAndDoesNotInventUnknownPrices(t *testing.T) {
	statement, branding := frozenBillingExportFixture(t)
	report := &UsageAuditReport{Filter: model.UsageAuditFilter{StartAt: statement.StartAt, EndAt: statement.EndAt, Group: "kimi", ChannelID: 40, Protocol: "anthropic"}, GeneratedAt: statement.CreatedAt, Currency: branding.Currency, SHA256: strings.Repeat("a", 64), Summary: UsageAuditSummary{Count: 3, UnknownUsageCount: 3}}
	labels := []string{"model-first", "model-second", strings.Repeat("long-model-name-", 12)}
	for _, label := range labels {
		report.Models = append(report.Models, UsageAuditGroup{Label: label, UsageAuditSummary: UsageAuditSummary{Count: 1, UnknownUsageCount: 1}})
	}
	body, err := RenderUsageAuditPDF(report, branding)
	require.NoError(t, err)
	cover := billingPDFPageText(t, body, 0)
	assert.Contains(t, cover, "未核价")
	assert.Contains(t, cover, "分组 kimi / 渠道 #40 / 协议 anthropic")
	assert.Contains(t, cover, "01 / 03")
	first := billingPDFPageText(t, body, 1)
	assert.Contains(t, first, labels[0])
	assert.Contains(t, first, labels[1])
	last := billingPDFPageText(t, body, 2)
	assert.Contains(t, strings.ReplaceAll(last, "\n", ""), labels[2])
	assert.Contains(t, last, "03 / 03")
	assert.Contains(t, last, "缓存读取")
	assert.Contains(t, last, "未核价")
}

func TestUsageAuditExportRejectsChangedEvidenceInsteadOfExportingDifferentAmounts(t *testing.T) {
	truncate(t)
	const at = int64(1700000000)
	log := model.Log{UserId: 812, TokenId: 29, Type: model.LogTypeConsume, CreatedAt: at, Quota: 10, PromptTokens: 10, Other: `{"usage_semantic":"openai","cache_tokens":0,"model_ratio":1,"group_ratio":1,"completion_ratio":5}`}
	require.NoError(t, model.LOG_DB.Create(&log).Error)
	report, err := GetUsageAuditReport(context.Background(), model.UsageAuditFilter{UserID: 812, StartAt: at, EndAt: at + 1})
	require.NoError(t, err)
	input := UsageAuditExportInput{Filter: report.Filter, GeneratedAt: report.GeneratedAt, SHA256: report.SHA256}
	export, err := GetUsageAuditExportReport(context.Background(), input)
	require.NoError(t, err)
	assert.Equal(t, report.SHA256, export.SHA256)
	require.NoError(t, model.LOG_DB.Model(&log).Update("quota", 11).Error)
	_, err = GetUsageAuditExportReport(context.Background(), input)
	assert.ErrorIs(t, err, ErrUsageAuditChanged)
	input.SHA256 = strings.Repeat("a", 64)
	_, err = GetUsageAuditExportReport(context.Background(), input)
	assert.ErrorIs(t, err, ErrUsageAuditChanged)
}
