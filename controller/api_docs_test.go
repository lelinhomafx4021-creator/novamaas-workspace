package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	apidocs "github.com/QuantumNous/new-api/docs/platform-api"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestAPIDocumentationAccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.UserSession{}))
	previousDB, previousRedis := model.DB, common.RedisEnabled
	model.DB, common.RedisEnabled = db, false
	user := model.User{Id: 7984, Username: "docs-reader", Password: "unused-hash", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default", AuthVersion: 1}
	user.SetAccessToken("docs-reader-pat")
	require.NoError(t, db.Create(&user).Error)
	bundle, err := service.CreateLoginSession(user.Id, "password", "127.0.0.1", "docs-test")
	require.NoError(t, err)
	common.OptionMapRWMutex.Lock()
	if common.OptionMap == nil {
		common.OptionMap = map[string]string{}
	}
	previousOption, existed := common.OptionMap["HeaderNavModules"]
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		model.DB, common.RedisEnabled = previousDB, previousRedis
		common.OptionMapRWMutex.Lock()
		defer common.OptionMapRWMutex.Unlock()
		if existed {
			common.OptionMap["HeaderNavModules"] = previousOption
		} else {
			delete(common.OptionMap, "HeaderNavModules")
		}
	})
	engine := gin.New()
	engine.GET("/api/docs", middleware.DisableCache(), middleware.HeaderNavModuleAuth("docs"), GetAPIDocumentation)
	for _, tt := range []struct {
		name, option string
		credential   string
		status       int
	}{
		{name: "not enabled", status: http.StatusForbidden},
		{name: "invalid configuration", option: "{", status: http.StatusForbidden},
		{name: "disabled for visitor", option: `{"docs":false}`, status: http.StatusForbidden},
		{name: "disabled for user", option: `{"docs":false}`, credential: "docs-reader-pat", status: http.StatusForbidden},
		{name: "legacy enabled", option: `{"docs":true}`, status: http.StatusOK},
		{name: "public enabled", option: `{"docs":{"enabled":true,"requireAuth":false}}`, status: http.StatusOK},
		{name: "login required", option: `{"docs":{"enabled":true,"requireAuth":true}}`, status: http.StatusUnauthorized},
		{name: "browser signed in", option: `{"docs":{"enabled":true,"requireAuth":true}}`, credential: bundle.AccessToken, status: http.StatusOK},
		{name: "signed in", option: `{"docs":{"enabled":true,"requireAuth":true}}`, credential: "docs-reader-pat", status: http.StatusOK},
	} {
		t.Run(tt.name, func(t *testing.T) {
			common.OptionMapRWMutex.Lock()
			common.OptionMap["HeaderNavModules"] = tt.option
			common.OptionMapRWMutex.Unlock()
			request := httptest.NewRequest(http.MethodGet, "/api/docs", nil)
			if tt.credential != "" {
				request.Header.Set("Authorization", "Bearer "+tt.credential)
			}
			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, request)
			require.Equal(t, tt.status, recorder.Code)
			assert.Contains(t, recorder.Header().Get("Cache-Control"), "no-store")
			var response struct {
				Success bool               `json:"success"`
				Data    []apidocs.Document `json:"data"`
			}
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
			if tt.status != http.StatusOK {
				assert.Empty(t, response.Data, "denied requests must not receive document content")
				return
			}
			assert.True(t, response.Success)
			require.NotEmpty(t, response.Data)
			assert.Contains(t, response.Data[0].Content, "YOUR_API_KEY")
			for _, doc := range response.Data {
				for _, private := range []string{"/api/channel", "/api/option", "/api/authz", "JWT", "PAT", "STORAGE_CREDENTIAL_ENCRYPTION_KEY"} {
					assert.NotContains(t, doc.Content, private, "customer docs must not expose administration instructions")
				}
			}
		})
	}
}
