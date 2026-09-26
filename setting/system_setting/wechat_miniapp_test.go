package system_setting

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func clearWeChatMiniAppEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv(weChatMiniAppAppIdEnv, "")
	t.Setenv(weChatMiniAppAppSecretEnv, "")
	t.Setenv(weChatMiniAppEnabledEnv, "")
	t.Setenv(weChatMiniAppRequestTimeoutSecondsEnv, "")
}

func TestWeChatMiniAppSettingsRequireCompleteCredentials(t *testing.T) {
	clearWeChatMiniAppEnvironment(t)

	tests := []struct {
		name     string
		settings WeChatMiniAppSettings
		ready    bool
	}{
		{name: "disabled", settings: WeChatMiniAppSettings{AppId: "app", AppSecret: "secret"}},
		{name: "missing app id", settings: WeChatMiniAppSettings{Enabled: true, AppSecret: "secret"}},
		{name: "missing secret", settings: WeChatMiniAppSettings{Enabled: true, AppId: "app"}},
		{name: "ready", settings: WeChatMiniAppSettings{Enabled: true, AppId: " app ", AppSecret: " secret "}, ready: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.ready, test.settings.IsReady())
		})
	}
}

func TestWeChatMiniAppTimeoutIsBounded(t *testing.T) {
	clearWeChatMiniAppEnvironment(t)

	assert.Equal(t, DefaultWeChatMiniAppTimeoutSeconds, (&WeChatMiniAppSettings{}).EffectiveTimeoutSeconds())
	assert.Equal(t, 8, (&WeChatMiniAppSettings{RequestTimeoutSeconds: 8}).EffectiveTimeoutSeconds())
	assert.Equal(t, DefaultWeChatMiniAppTimeoutSeconds, (&WeChatMiniAppSettings{RequestTimeoutSeconds: 16}).EffectiveTimeoutSeconds())
}

func TestWeChatMiniAppEnvironmentOverridesDatabaseSettings(t *testing.T) {
	clearWeChatMiniAppEnvironment(t)
	t.Setenv(weChatMiniAppAppIdEnv, " wx-env-app ")
	t.Setenv(weChatMiniAppAppSecretEnv, " env-secret ")
	t.Setenv(weChatMiniAppEnabledEnv, "true")
	t.Setenv(weChatMiniAppRequestTimeoutSecondsEnv, "9")

	settings := WeChatMiniAppSettings{
		Enabled:               false,
		AppId:                 "wx-database-app",
		AppSecret:             "database-secret",
		RequestTimeoutSeconds: 3,
	}
	effective := settings.Resolve()

	assert.True(t, effective.Enabled)
	assert.Equal(t, "wx-env-app", effective.AppId)
	assert.Equal(t, "env-secret", effective.AppSecret)
	assert.Equal(t, 9, effective.RequestTimeoutSeconds)
	assert.True(t, settings.IsReady())
}

func TestWeChatMiniAppEmptyEnvironmentFallsBackToDatabaseSettings(t *testing.T) {
	clearWeChatMiniAppEnvironment(t)
	settings := WeChatMiniAppSettings{
		Enabled:               true,
		AppId:                 "wx-database-app",
		AppSecret:             "database-secret",
		RequestTimeoutSeconds: 7,
	}

	effective := settings.Resolve()

	assert.Equal(t, settings, effective)
	assert.True(t, settings.IsReady())
	assert.Equal(t, 7, settings.EffectiveTimeoutSeconds())
}

func TestWeChatMiniAppInvalidEnvironmentUsesSafeValues(t *testing.T) {
	clearWeChatMiniAppEnvironment(t)
	t.Setenv(weChatMiniAppEnabledEnv, "not-a-boolean")
	t.Setenv(weChatMiniAppRequestTimeoutSecondsEnv, "30")
	t.Setenv(weChatMiniAppAppIdEnv, strings.Repeat("a", 65))

	settings := WeChatMiniAppSettings{
		Enabled:               true,
		AppId:                 "wx-database-app",
		AppSecret:             "database-secret",
		RequestTimeoutSeconds: 7,
	}

	assert.False(t, settings.Resolve().Enabled)
	assert.False(t, settings.IsReady())
	assert.Equal(t, DefaultWeChatMiniAppTimeoutSeconds, settings.EffectiveTimeoutSeconds())
}
