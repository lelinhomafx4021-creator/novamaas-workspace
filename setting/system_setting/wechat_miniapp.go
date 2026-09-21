package system_setting

import (
	"os"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/setting/config"
)

const DefaultWeChatMiniAppTimeoutSeconds = 5

const (
	weChatMiniAppAppIdEnv                 = "WECHAT_MINIAPP_APP_ID"
	weChatMiniAppAppSecretEnv             = "WECHAT_MINIAPP_APP_SECRET"
	weChatMiniAppEnabledEnv               = "WECHAT_MINIAPP_ENABLED"
	weChatMiniAppRequestTimeoutSecondsEnv = "WECHAT_MINIAPP_REQUEST_TIMEOUT_SECONDS"
)

type WeChatMiniAppSettings struct {
	Enabled               bool   `json:"enabled"`
	AppId                 string `json:"app_id"`
	AppSecret             string `json:"app_secret"`
	RequestTimeoutSeconds int    `json:"request_timeout_seconds"`
}

var defaultWeChatMiniAppSettings = WeChatMiniAppSettings{
	RequestTimeoutSeconds: DefaultWeChatMiniAppTimeoutSeconds,
}

func init() {
	config.GlobalConfig.Register("wechat_miniapp", &defaultWeChatMiniAppSettings)
}

func GetWeChatMiniAppSettings() *WeChatMiniAppSettings {
	return &defaultWeChatMiniAppSettings
}

// Resolve returns the effective runtime configuration. Explicit environment
// variables take precedence over database-backed settings. This lets local
// development use .env while deployments inject secrets through their normal
// environment or secret manager.
func (settings *WeChatMiniAppSettings) Resolve() WeChatMiniAppSettings {
	effective := WeChatMiniAppSettings{
		RequestTimeoutSeconds: DefaultWeChatMiniAppTimeoutSeconds,
	}
	if settings != nil {
		effective = *settings
	}

	if value, ok := os.LookupEnv(weChatMiniAppAppIdEnv); ok && strings.TrimSpace(value) != "" {
		effective.AppId = strings.TrimSpace(value)
	}
	if value, ok := os.LookupEnv(weChatMiniAppAppSecretEnv); ok && strings.TrimSpace(value) != "" {
		effective.AppSecret = strings.TrimSpace(value)
	}
	if value, ok := os.LookupEnv(weChatMiniAppEnabledEnv); ok && strings.TrimSpace(value) != "" {
		enabled, err := strconv.ParseBool(strings.TrimSpace(value))
		effective.Enabled = err == nil && enabled
	}
	if value, ok := os.LookupEnv(weChatMiniAppRequestTimeoutSecondsEnv); ok && strings.TrimSpace(value) != "" {
		timeoutSeconds, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil || timeoutSeconds < 1 || timeoutSeconds > 15 {
			effective.RequestTimeoutSeconds = DefaultWeChatMiniAppTimeoutSeconds
		} else {
			effective.RequestTimeoutSeconds = timeoutSeconds
		}
	}

	return effective
}

func (settings *WeChatMiniAppSettings) HasEffectiveCredentials() bool {
	effective := settings.Resolve()
	return hasValidWeChatMiniAppCredentials(effective)
}

func hasValidWeChatMiniAppCredentials(settings WeChatMiniAppSettings) bool {
	appId := strings.TrimSpace(settings.AppId)
	appSecret := strings.TrimSpace(settings.AppSecret)
	return appId != "" && len(appId) <= 64 && appSecret != "" && len(appSecret) <= 128
}

func (settings *WeChatMiniAppSettings) IsReady() bool {
	effective := settings.Resolve()
	return effective.Enabled && hasValidWeChatMiniAppCredentials(effective)
}

func (settings *WeChatMiniAppSettings) EffectiveTimeoutSeconds() int {
	effective := settings.Resolve()
	if effective.RequestTimeoutSeconds < 1 || effective.RequestTimeoutSeconds > 15 {
		return DefaultWeChatMiniAppTimeoutSeconds
	}
	return effective.RequestTimeoutSeconds
}
