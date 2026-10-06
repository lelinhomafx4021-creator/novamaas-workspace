package controller

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupSMSOptionsTest(t *testing.T) *gorm.DB {
	t.Helper()
	db := setupMiniAppAuthTest(t)
	require.NoError(t, db.AutoMigrate(&model.Option{}))
	for _, name := range []string{"SMS_ENABLED", "ALIYUN_SMS_ACCESS_KEY_ID", "ALIYUN_SMS_ACCESS_KEY_SECRET", "ALIYUN_SMS_SECURITY_TOKEN", "ALIYUN_SMS_SIGN_NAME", "ALIYUN_SMS_TEMPLATE_CODE", "ALIYUN_SMS_CODE_PARAMETER"} {
		t.Setenv(name, "")
	}
	settings := config.GlobalConfig.Get("sms").(*system_setting.SMSSettings)
	previous := system_setting.GetStoredSMSSettings()
	common.OptionMapRWMutex.Lock()
	previousMap := common.OptionMap
	common.OptionMap = make(map[string]string)
	*settings = system_setting.SMSSettings{CodeParameter: "code"}
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		*settings = previous
		common.OptionMap = previousMap
		common.OptionMapRWMutex.Unlock()
	})
	return db
}

func callSMSOptions(handler gin.HandlerFunc, body string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPut, "/api/option/sms", strings.NewReader(body))
	handler(ctx)
	return recorder
}

func TestSMSOptionsSaveRequiresCompleteConfigurationAndDoesNotPartiallyEnable(t *testing.T) {
	db := setupSMSOptionsTest(t)
	recorder := callSMSOptions(SaveSMSOptions, `{"enabled":true,"sign_name":"approved-signature"}`)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "SMS_CONFIG_INCOMPLETE")
	assert.False(t, system_setting.GetSMSSettings().IsReady())
	var count int64
	require.NoError(t, db.Model(&model.Option{}).Count(&count).Error)
	assert.Zero(t, count)
}

func TestSMSOptionsPreserveWriteOnlyCredentialsAndSupportExplicitSTSRemoval(t *testing.T) {
	db := setupSMSOptionsTest(t)
	var sqlLog bytes.Buffer
	db.Logger = logger.New(log.New(&sqlLog, "", 0), logger.Config{LogLevel: logger.Info})
	recorder := callSMSOptions(SaveSMSOptions, `{"enabled":true,"access_key_id":"private-access-id","access_key_secret":"private-access-secret","security_token":"private-sts-token","sign_name":"approved-signature","template_code":"SMS_TEST","code_parameter":"code"}`)
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.True(t, system_setting.GetSMSSettings().IsReady())
	assert.NotContains(t, sqlLog.String(), "private-access-id")
	assert.NotContains(t, sqlLog.String(), "private-access-secret")
	assert.NotContains(t, sqlLog.String(), "private-sts-token")
	for _, handler := range []gin.HandlerFunc{GetSMSOptions, GetOptions} {
		response := callSMSOptions(handler, "")
		require.Equal(t, http.StatusOK, response.Code)
		for _, value := range []string{"private-access-id", "private-access-secret", "private-sts-token"} {
			assert.NotContains(t, response.Body.String(), value)
		}
	}
	assert.Contains(t, recorder.Header().Get("Cache-Control"), "no-store")
	assert.Contains(t, recorder.Body.String(), `"access_key_secret_configured":true`)
	recorder = callSMSOptions(SaveSMSOptions, `{"access_key_id":"","access_key_secret":" ","security_token":"","sign_name":"new-signature"}`)
	require.Equal(t, http.StatusOK, recorder.Code)
	settings := system_setting.GetSMSSettings()
	assert.Equal(t, "private-access-id", settings.AccessKeyId)
	assert.Equal(t, "private-access-secret", settings.AccessKeySecret)
	assert.Equal(t, "private-sts-token", settings.SecurityToken)
	assert.Equal(t, "new-signature", settings.SignName)
	recorder = callSMSOptions(SaveSMSOptions, `{"clear_security_token":true}`)
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Empty(t, system_setting.GetSMSSettings().SecurityToken)
	assert.True(t, system_setting.GetSMSSettings().IsReady())
}

func TestSMSOptionsShowEnvironmentOverridesAndKeepDeploymentDisableAuthoritative(t *testing.T) {
	setupSMSOptionsTest(t)
	t.Setenv("SMS_ENABLED", "false")
	t.Setenv("ALIYUN_SMS_ACCESS_KEY_SECRET", "environment-secret")
	recorder := callSMSOptions(SaveSMSOptions, `{"enabled":true,"access_key_id":"saved-id","sign_name":"approved-signature","template_code":"SMS_TEST"}`)
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.True(t, system_setting.GetStoredSMSSettings().Enabled)
	assert.False(t, system_setting.GetSMSSettings().IsReady())
	assert.Contains(t, recorder.Body.String(), `"enabled":"SMS_ENABLED"`)
	assert.Contains(t, recorder.Body.String(), `"access_key_secret":"ALIYUN_SMS_ACCESS_KEY_SECRET"`)
	assert.NotContains(t, recorder.Body.String(), "environment-secret")
	t.Setenv("SMS_ENABLED", "true")
	assert.True(t, system_setting.GetSMSSettings().IsReady())
}

func TestSMSOptionsRejectInvalidTemplateVariableAndRollbackDatabaseFailure(t *testing.T) {
	db := setupSMSOptionsTest(t)
	recorder := callSMSOptions(SaveSMSOptions, `{"sign_name":"new-signature","code_parameter":"code.other"}`)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Equal(t, "code", system_setting.GetSMSSettings().CodeParameter)
	assert.Empty(t, system_setting.GetSMSSettings().SignName)
	require.NoError(t, db.Migrator().DropTable(&model.Option{}))
	recorder = callSMSOptions(SaveSMSOptions, `{"sign_name":"new-signature"}`)
	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	assert.Empty(t, system_setting.GetSMSSettings().SignName)
}

func TestSMSOptionsCannotBeEnabledThroughSingleOptionUpdates(t *testing.T) {
	setupSMSOptionsTest(t)
	recorder := callSMSOptions(UpdateOption, `{"key":"sms.enabled","value":true}`)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "SMS_CONFIG_ATOMIC_UPDATE_REQUIRED")
	assert.False(t, system_setting.GetSMSSettings().Enabled)
}
