package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/runtimewatch"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRuntimeMonitoringRequiresRootPrivileges(t *testing.T) {
	setupRelayRouterTestDB(t)
	require.NoError(t, model.DB.AutoMigrate(&model.UserSession{}, &model.Log{}))
	previousSecret := common.SessionSecret
	common.SessionSecret = "runtime-monitor-auth-test-secret"
	t.Cleanup(func() { common.SessionSecret = previousSecret })
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetApiRouter(engine)

	unauthorized := httptest.NewRecorder()
	engine.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/api/system-info/monitor", nil))
	assert.Equal(t, http.StatusUnauthorized, unauthorized.Code)
	assert.NotContains(t, unauthorized.Body.String(), "hostname")

	for _, role := range []int{common.RoleCommonUser, common.RoleAdminUser, common.RoleRootUser} {
		username := map[int]string{common.RoleCommonUser: "mon-user", common.RoleAdminUser: "mon-admin", common.RoleRootUser: "mon-root"}[role]
		user := model.User{Username: username, AffCode: username, Role: role, Status: common.UserStatusEnabled, Group: "default", AuthVersion: 1}
		require.NoError(t, model.DB.Create(&user).Error)
		bundle, err := service.CreateLoginSession(user.Id, "password", "127.0.0.1", "monitor-test")
		require.NoError(t, err)
		request := httptest.NewRequest(http.MethodGet, "/api/system-info/monitor", nil)
		request.Header.Set("Authorization", "Bearer "+bundle.AccessToken)
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, request)
		if role < common.RoleRootUser {
			assert.Equal(t, http.StatusForbidden, recorder.Code)
			assert.NotContains(t, recorder.Body.String(), "hostname")
			continue
		}
		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		assert.Contains(t, recorder.Body.String(), `"success":true`)
		var response struct {
			Data runtimewatch.Report `json:"data"`
		}
		require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
		assert.Equal(t, runtimewatch.StorageStatus{Mode: "memory", CapacityPoints: 1440}, response.Data.Meta.Storage)
		assert.Equal(t, "private, no-store", recorder.Header().Get("Cache-Control"))
	}
}
