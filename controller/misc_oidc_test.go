package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetStatusReturnsEffectiveOIDCDisplayName(t *testing.T) {
	settings := system_setting.GetOIDCSettings()
	originalDisplayName := settings.DisplayName
	originalOptionMap := common.OptionMap
	t.Cleanup(func() {
		settings.DisplayName = originalDisplayName
		common.OptionMap = originalOptionMap
	})
	common.OptionMap = map[string]string{}

	tests := []struct {
		name        string
		displayName string
		want        string
	}{
		{
			name:        "custom name is trimmed",
			displayName: "  Acme SSO  ",
			want:        "Acme SSO",
		},
		{
			name:        "whitespace-only name falls back",
			displayName: "   ",
			want:        "OIDC",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings.DisplayName = tt.displayName
			response := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(response)
			context.Request = httptest.NewRequest(http.MethodGet, "/api/status", nil)

			GetStatus(context)

			var payload struct {
				Success bool           `json:"success"`
				Data    map[string]any `json:"data"`
			}
			require.NoError(t, common.Unmarshal(response.Body.Bytes(), &payload))
			require.True(t, payload.Success)
			assert.Equal(t, tt.want, payload.Data["oidc_display_name"])
		})
	}
}

func TestWeChatMiniAppStatusExposesReadinessWithoutCredentials(t *testing.T) {
	t.Setenv("WECHAT_MINIAPP_APP_ID", "")
	t.Setenv("WECHAT_MINIAPP_APP_SECRET", "")
	t.Setenv("WECHAT_MINIAPP_ENABLED", "")
	t.Setenv("WECHAT_MINIAPP_REQUEST_TIMEOUT_SECONDS", "")

	settings := system_setting.GetWeChatMiniAppSettings()
	previousSettings := *settings
	previousOptionMap := common.OptionMap
	t.Cleanup(func() {
		*settings = previousSettings
		common.OptionMap = previousOptionMap
	})
	settings.Enabled = true
	settings.AppId = "wx-private-app-id"
	settings.AppSecret = "private-app-secret"
	common.OptionMap = map[string]string{
		"wechat_miniapp.app_id":     settings.AppId,
		"wechat_miniapp.app_secret": settings.AppSecret,
	}

	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/status", nil)
	GetStatus(context)

	var payload struct {
		Success bool           `json:"success"`
		Data    map[string]any `json:"data"`
	}
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &payload))
	require.True(t, payload.Success)
	assert.Equal(t, true, payload.Data["wechat_miniapp_login"])
	assert.NotContains(t, response.Body.String(), settings.AppId)
	assert.NotContains(t, response.Body.String(), settings.AppSecret)

	optionsResponse := httptest.NewRecorder()
	optionsContext, _ := gin.CreateTestContext(optionsResponse)
	optionsContext.Request = httptest.NewRequest(http.MethodGet, "/api/option", nil)
	GetOptions(optionsContext)
	assert.Contains(t, optionsResponse.Body.String(), settings.AppId)
	assert.NotContains(t, optionsResponse.Body.String(), settings.AppSecret)
	assert.NotContains(t, optionsResponse.Body.String(), "wechat_miniapp.app_secret")
}

func TestWeChatMiniAppStatusUsesEnvironmentWithoutExposingCredentials(t *testing.T) {
	t.Setenv("WECHAT_MINIAPP_APP_ID", "wx-environment-app-id")
	t.Setenv("WECHAT_MINIAPP_APP_SECRET", "environment-app-secret")
	t.Setenv("WECHAT_MINIAPP_ENABLED", "true")
	t.Setenv("WECHAT_MINIAPP_REQUEST_TIMEOUT_SECONDS", "5")

	settings := system_setting.GetWeChatMiniAppSettings()
	previousSettings := *settings
	t.Cleanup(func() {
		*settings = previousSettings
	})
	*settings = system_setting.WeChatMiniAppSettings{}

	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/status", nil)
	GetStatus(context)

	var payload struct {
		Success bool           `json:"success"`
		Data    map[string]any `json:"data"`
	}
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &payload))
	require.True(t, payload.Success)
	assert.Equal(t, true, payload.Data["wechat_miniapp_login"])
	assert.NotContains(t, response.Body.String(), "wx-environment-app-id")
	assert.NotContains(t, response.Body.String(), "environment-app-secret")
}
