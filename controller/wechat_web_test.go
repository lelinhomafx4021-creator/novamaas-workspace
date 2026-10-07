package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWeChatWebsiteOnlyLogsInUniquelyBoundAccounts(t *testing.T) {
	for _, scenario := range []struct {
		name, union                  string
		claimed, duplicate, disabled bool
		wantSuccess                  bool
	}{
		{name: "bound", union: "union-bound", claimed: true, wantSuccess: true},
		{name: "missing_union", claimed: true},
		{name: "unbound", union: "union-missing"},
		{name: "stale_profile", union: "union-stale"},
		{name: "ambiguous_identity", union: "union-duplicate", claimed: true, duplicate: true},
		{name: "disabled_account", union: "union-disabled", claimed: true, disabled: true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			db := setupMiniAppAuthTest(t)
			t.Setenv("WECHAT_WEB_APP_ID", "wx-website")
			t.Setenv("WECHAT_WEB_APP_SECRET", "test-secret")
			t.Setenv("WECHAT_WEB_REDIRECT_URI", "https://gateway.example/oauth/wechat-web")
			previous := exchangeWeChatWebCode
			exchangeWeChatWebCode = func(_ context.Context, _ service.WeChatWebSettings, code string) (string, error) {
				assert.Equal(t, "website-code", code)
				return scenario.union, nil
			}
			t.Cleanup(func() { exchangeWeChatWebCode = previous })
			user := createMiniAppPasswordUser(t, db, "web-wechat-user", "password123")
			if scenario.disabled {
				require.NoError(t, db.Model(user).Update("status", common.UserStatusDisabled).Error)
			}
			require.NoError(t, db.Create(&model.WeChatMiniAppProfile{UserId: user.Id, AppId: "wx-mini", UnionId: scenario.union}).Error)
			if scenario.claimed {
				require.NoError(t, db.Create(&model.ExternalIdentityClaim{UserId: user.Id, Provider: model.ExternalIdentityProviderWeChatMiniApp, AppId: "wx-mini", Subject: "mini-openid"}).Error)
			}
			if scenario.duplicate {
				other := createMiniAppPasswordUser(t, db, "second-owner", "password123")
				require.NoError(t, db.Create(&model.WeChatMiniAppProfile{UserId: other.Id, AppId: "wx-other-mini", UnionId: scenario.union}).Error)
				require.NoError(t, db.Create(&model.ExternalIdentityClaim{UserId: other.Id, Provider: model.ExternalIdentityProviderWeChatMiniApp, AppId: "wx-other-mini", Subject: "other-openid"}).Error)
			}
			state, _, err := model.CreateAuthFlow(model.AuthFlowCreate{Purpose: model.AuthFlowPurposeOAuth, Provider: "wechat-web", Intent: model.AuthFlowIntentLogin, ExpiresAt: time.Now().Add(time.Minute)})
			require.NoError(t, err)
			router := gin.New()
			router.GET("/api/oauth/wechat-web", WeChatWebLogin)
			request := httptest.NewRequest(http.MethodGet, "/api/oauth/wechat-web?code=website-code&state="+state, nil)
			request.AddCookie(&http.Cookie{Name: "wechat_web_state", Value: state})
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			var result struct {
				Success bool `json:"success"`
				Data    struct {
					AccessToken string `json:"access_token"`
				} `json:"data"`
			}
			require.NoError(t, common.Unmarshal(response.Body.Bytes(), &result))
			assert.Equal(t, scenario.wantSuccess, result.Success, response.Body.String())
			if scenario.wantSuccess {
				assert.NotEmpty(t, result.Data.AccessToken)
			} else {
				assert.Empty(t, result.Data.AccessToken)
			}
			replay := httptest.NewRecorder()
			router.ServeHTTP(replay, request)
			assert.Equal(t, http.StatusForbidden, replay.Code)
			var userCount int64
			require.NoError(t, db.Model(&model.User{}).Count(&userCount).Error)
			expectedUsers := int64(1)
			if scenario.duplicate {
				expectedUsers = 2
			}
			assert.Equal(t, expectedUsers, userCount)
		})
	}
}

func TestWeChatWebsiteRejectsStateFromAnotherBrowser(t *testing.T) {
	setupMiniAppAuthTest(t)
	t.Setenv("WECHAT_WEB_APP_ID", "wx-website")
	t.Setenv("WECHAT_WEB_APP_SECRET", "test-secret")
	t.Setenv("WECHAT_WEB_REDIRECT_URI", "https://gateway.example/oauth/wechat-web")
	router := gin.New()
	router.GET("/api/oauth/wechat-web", WeChatWebLogin)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/oauth/wechat-web?state=foreign&code=code", nil))
	assert.Equal(t, http.StatusForbidden, response.Code)
}

func TestBrowserLoginAuditUsesBrowserInsteadOfUnknown(t *testing.T) {
	setupMiniAppAuthTest(t)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/user/login", nil)
	user := createMiniAppPasswordUser(t, model.DB, "browser-audit", "password123")
	recordLoginAudit(user, c)
	var log model.Log
	require.NoError(t, model.LOG_DB.Where("user_id = ?", user.Id).First(&log).Error)
	assert.Equal(t, "Logged in successfully via browser", log.Content)
}

func TestWeChatWebsiteStartBindsStateToTheInitiatingBrowser(t *testing.T) {
	setupMiniAppAuthTest(t)
	t.Setenv("WECHAT_WEB_APP_ID", "website-app")
	t.Setenv("WECHAT_WEB_APP_SECRET", "website-secret")
	t.Setenv("WECHAT_WEB_REDIRECT_URI", "https://example.com/oauth/wechat-web")
	router := gin.New()
	router.POST("/api/oauth/wechat-web/start", WeChatWebLoginStart)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/oauth/wechat-web/start", nil))
	require.Equal(t, http.StatusOK, recorder.Code)
	var result struct {
		Success bool `json:"success"`
		Data    struct {
			AuthorizeURL string `json:"authorize_url"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &result))
	require.True(t, result.Success)
	cookies := recorder.Result().Cookies()
	require.Len(t, cookies, 1)
	assert.True(t, cookies[0].HttpOnly)
	assert.Equal(t, http.SameSiteLaxMode, cookies[0].SameSite)
	assert.NotContains(t, result.Data.AuthorizeURL, "website-secret")
	assert.Contains(t, result.Data.AuthorizeURL, "state="+cookies[0].Value)
	_, err := model.GetAuthFlow(cookies[0].Value, model.AuthFlowMatch{Purpose: model.AuthFlowPurposeOAuth, Provider: "wechat-web", Intent: model.AuthFlowIntentLogin})
	require.NoError(t, err)
}
