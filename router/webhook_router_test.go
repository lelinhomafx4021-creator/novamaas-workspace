package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/asyncwebhook"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSystemWebhookRoutesEnforceRootAndGateBrandedManual(t *testing.T) {
	setupRelayRouterTestDB(t)
	require.NoError(t, model.DB.AutoMigrate(&model.UserSession{}, &model.Log{}, &model.AssetWebhookEndpoint{}, &model.Option{}))
	savedSecret, savedLogo, savedOperatingLogo := common.SessionSecret, common.Logo, common.OperatingEntityLogo
	common.SessionSecret, common.Logo, common.OperatingEntityLogo = "webhook-root-routes-test", "", ""
	t.Cleanup(func() {
		common.SessionSecret, common.Logo, common.OperatingEntityLogo = savedSecret, savedLogo, savedOperatingLogo
	})
	tokens := make(map[int]string)
	for _, role := range []int{common.RoleCommonUser, common.RoleAdminUser, common.RoleRootUser} {
		username := map[int]string{common.RoleCommonUser: "hook-common", common.RoleAdminUser: "hook-admin", common.RoleRootUser: "hook-root"}[role]
		user := model.User{Username: username, AffCode: username, Role: role, Status: common.UserStatusEnabled, Group: "default", AuthVersion: 1}
		require.NoError(t, model.DB.Create(&user).Error)
		session, err := service.CreateLoginSession(user.Id, "password", "127.0.0.1", "test")
		require.NoError(t, err)
		tokens[role] = session.AccessToken
	}
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetApiRouter(engine)
	input := `{"asset_library":{"enabled":true,"url":"https://operator.example/assets"},"media_tasks":{"enabled":true,"url":"https://operator.example/tasks"},"manual_enabled":true}`
	for _, role := range []int{common.RoleCommonUser, common.RoleAdminUser} {
		for _, route := range []struct{ method, path string }{{http.MethodGet, "/api/webhooks/system"}, {http.MethodPut, "/api/webhooks/system"}, {http.MethodPost, "/api/webhooks/system/media_tasks/test"}} {
			request := httptest.NewRequest(route.method, route.path, strings.NewReader(input))
			request.Header.Set("Authorization", "Bearer "+tokens[role])
			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, request)
			assert.Equal(t, http.StatusForbidden, recorder.Code, recorder.Body.String())
			assert.NotContains(t, recorder.Body.String(), "operator.example")
		}
	}
	for _, path := range []string{"/api/webhooks/system", "/api/webhooks/capabilities", "/api/webhooks/manual.pdf"} {
		anonymous := httptest.NewRecorder()
		engine.ServeHTTP(anonymous, httptest.NewRequest(http.MethodGet, path, nil))
		assert.Equal(t, http.StatusUnauthorized, anonymous.Code)
	}
	for _, path := range []string{"/api/webhooks/capabilities", "/api/webhooks/manual.pdf"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.Header.Set("Authorization", "Bearer "+tokens[common.RoleCommonUser])
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, request)
		if path == "/api/webhooks/capabilities" {
			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			assert.JSONEq(t, `{"success":true,"data":{"asset_library_enabled":false,"media_tasks_enabled":false,"manual_enabled":false}}`, recorder.Body.String())
		} else {
			assert.Equal(t, http.StatusForbidden, recorder.Code, "manual generation must be disabled by default")
		}
	}
	for _, path := range []string{"/api/webhook-endpoints", "/api/asset-library/webhook-endpoints"} {
		for _, eventType := range []string{model.WebhookEventAssetFailed, model.WebhookEventTaskStatusChanged} {
			body, err := common.Marshal(map[string]any{"name": "callback", "url": "https://customer.example/events", "event_types": []string{eventType}})
			require.NoError(t, err)
			request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(string(body)))
			request.Header.Set("Authorization", "Bearer "+tokens[common.RoleCommonUser])
			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, request)
			assert.Equal(t, http.StatusForbidden, recorder.Code, recorder.Body.String())
		}
	}
	save := httptest.NewRequest(http.MethodPut, "/api/webhooks/system", strings.NewReader(input))
	save.Header.Set("Authorization", "Bearer "+tokens[common.RoleRootUser])
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, save)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	stored, err := asyncwebhook.GetSystemWebhookSettings()
	require.NoError(t, err)
	assert.True(t, stored.AssetLibrary.Enabled)
	assert.True(t, stored.MediaTasks.Enabled)
	assert.True(t, stored.ManualEnabled)
	assert.Equal(t, "https://operator.example/tasks", stored.MediaTasks.URL)
	partial := httptest.NewRequest(http.MethodPut, "/api/webhooks/system", strings.NewReader(`{"media_tasks":{"enabled":false,"url":""}}`))
	partial.Header.Set("Authorization", "Bearer "+tokens[common.RoleRootUser])
	recorder = httptest.NewRecorder()
	engine.ServeHTTP(recorder, partial)
	assert.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
	unchanged, err := asyncwebhook.GetSystemWebhookSettings()
	require.NoError(t, err)
	assert.Equal(t, stored, unchanged, "partial PUT must not silently turn off another category")
	missingPolicy := httptest.NewRequest(http.MethodPut, "/api/webhooks/system", strings.NewReader(strings.Replace(input, `,"manual_enabled":true`, "", 1)))
	missingPolicy.Header.Set("Authorization", "Bearer "+tokens[common.RoleRootUser])
	recorder = httptest.NewRecorder()
	engine.ServeHTTP(recorder, missingPolicy)
	assert.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
	unchanged, err = asyncwebhook.GetSystemWebhookSettings()
	require.NoError(t, err)
	assert.Equal(t, stored, unchanged, "an omitted policy cannot silently disable manual downloads")

	read := httptest.NewRequest(http.MethodGet, "/api/webhooks/system", nil)
	read.Header.Set("Authorization", "Bearer "+tokens[common.RoleRootUser])
	recorder = httptest.NewRecorder()
	engine.ServeHTTP(recorder, read)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	assert.Contains(t, recorder.Body.String(), stored.MediaTasks.URL)
	assert.Contains(t, recorder.Header().Get("Cache-Control"), "no-store")

	for _, manual := range []struct{ scope, filename string }{
		{"asset_library", "asset-library-webhook-api-manual.pdf"},
		{"media_tasks", "media-task-webhook-api-manual.pdf"},
	} {
		download := httptest.NewRequest(http.MethodGet, "/api/webhooks/manual.pdf?scope="+manual.scope, nil)
		download.Header.Set("Authorization", "Bearer "+tokens[common.RoleCommonUser])
		recorder = httptest.NewRecorder()
		engine.ServeHTTP(recorder, download)
		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		assert.Equal(t, "application/pdf", recorder.Header().Get("Content-Type"))
		assert.Contains(t, recorder.Header().Get("Content-Disposition"), manual.filename)
		assert.True(t, strings.HasPrefix(recorder.Body.String(), "%PDF-"))
	}
	for _, path := range []string{"/api/webhooks/manual.pdf", "/api/webhooks/manual.pdf?scope=other"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.Header.Set("Authorization", "Bearer "+tokens[common.RoleCommonUser])
		recorder = httptest.NewRecorder()
		engine.ServeHTTP(recorder, request)
		assert.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
	}

	for _, enabled := range []bool{true, false} {
		if !enabled {
			disable := httptest.NewRequest(http.MethodPut, "/api/webhooks/system", strings.NewReader(strings.Replace(input, `"manual_enabled":true`, `"manual_enabled":false`, 1)))
			disable.Header.Set("Authorization", "Bearer "+tokens[common.RoleRootUser])
			recorder = httptest.NewRecorder()
			engine.ServeHTTP(recorder, disable)
			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		}
		capabilities := httptest.NewRequest(http.MethodGet, "/api/webhooks/capabilities", nil)
		capabilities.Header.Set("Authorization", "Bearer "+tokens[common.RoleCommonUser])
		recorder = httptest.NewRecorder()
		engine.ServeHTTP(recorder, capabilities)
		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		var body struct {
			Data asyncwebhook.WebhookCapabilities `json:"data"`
		}
		require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &body))
		assert.Equal(t, enabled, body.Data.ManualEnabled)
		assert.NotContains(t, recorder.Body.String(), "operator.example")
		assert.Contains(t, recorder.Header().Get("Cache-Control"), "no-store")
	}
	// A stale visible button or a direct URL cannot bypass a newly disabled policy.
	common.Logo = "invalid-branding-url"
	download := httptest.NewRequest(http.MethodGet, "/api/webhooks/manual.pdf?scope=asset_library", nil)
	download.Header.Set("Authorization", "Bearer "+tokens[common.RoleCommonUser])
	recorder = httptest.NewRecorder()
	engine.ServeHTTP(recorder, download)
	assert.Equal(t, http.StatusForbidden, recorder.Code, recorder.Body.String())
	assert.NotContains(t, recorder.Body.String(), "%PDF-")
	assert.NotContains(t, recorder.Body.String(), "branding", "disabled requests must stop before rendering")
	// Policy-read failures must also stop generation.
	require.NoError(t, model.DB.Migrator().DropTable(&model.Option{}))
	download = httptest.NewRequest(http.MethodGet, "/api/webhooks/manual.pdf?scope=media_tasks", nil)
	download.Header.Set("Authorization", "Bearer "+tokens[common.RoleCommonUser])
	recorder = httptest.NewRecorder()
	engine.ServeHTTP(recorder, download)
	assert.Equal(t, http.StatusInternalServerError, recorder.Code, recorder.Body.String())
	assert.NotContains(t, recorder.Body.String(), "%PDF-")
}
