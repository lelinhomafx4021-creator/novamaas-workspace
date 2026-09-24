package controller

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type miniAppAuthTestResponse struct {
	Success bool   `json:"success"`
	Code    string `json:"code"`
	Data    struct {
		BindingRequired bool   `json:"binding_required"`
		FlowToken       string `json:"flow_token"`
		AccessToken     string `json:"access_token"`
		RefreshToken    string `json:"refresh_token"`
		ProofToken      string `json:"proof_token"`
		Key             string `json:"key"`
		Session         struct {
			SID string `json:"sid"`
		} `json:"session"`
		User map[string]any `json:"user"`
	} `json:"data"`
}

func TestMiniAppTokenKeyRequiresPasswordBoundSecurityProof(t *testing.T) {
	db := setupMiniAppAuthTest(t)
	user := createMiniAppPasswordUser(t, db, "mini-key-user", "password123")
	bundle, err := service.CreateLoginSession(user.Id, "wechat_miniapp", "127.0.0.1", "miniapp-test")
	require.NoError(t, err)
	identity, err := service.ParseAccessToken(bundle.AccessToken)
	require.NoError(t, err)
	token := &model.Token{
		UserId: user.Id, Key: "sk-mini-secret", Name: "mobile-key", Status: common.TokenStatusEnabled,
		CreatedTime: time.Now().Unix(), AccessedTime: time.Now().Unix(), ExpiredTime: -1,
	}
	require.NoError(t, db.Create(token).Error)

	verifyRecorder := httptest.NewRecorder()
	verifyContext, _ := gin.CreateTestContext(verifyRecorder)
	verifyContext.Request = httptest.NewRequest(http.MethodPost, "/api/mini/security/verify", strings.NewReader(`{"password":"password123"}`))
	verifyContext.Set("id", identity.UserID)
	verifyContext.Set("session_id", identity.SessionID)
	verifyContext.Set("auth_version", identity.UserAuthVersion)
	verifyContext.Set("session_version", identity.SessionVersion)
	MiniAppVerifySecurity(verifyContext)
	require.Equal(t, http.StatusOK, verifyRecorder.Code)
	var verified miniAppAuthTestResponse
	require.NoError(t, common.Unmarshal(verifyRecorder.Body.Bytes(), &verified))
	require.NotEmpty(t, verified.Data.ProofToken)

	keyRecorder := httptest.NewRecorder()
	keyContext, _ := gin.CreateTestContext(keyRecorder)
	keyContext.Request = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/mini/token/%d/key", token.Id), nil)
	keyContext.Request.Header.Set("X-Security-Proof", verified.Data.ProofToken)
	keyContext.Params = gin.Params{{Key: "id", Value: fmt.Sprint(token.Id)}}
	keyContext.Set("id", identity.UserID)
	keyContext.Set("session_id", identity.SessionID)
	keyContext.Set("auth_version", identity.UserAuthVersion)
	keyContext.Set("session_version", identity.SessionVersion)
	MiniAppGetTokenKey(keyContext)
	require.Equal(t, http.StatusOK, keyRecorder.Code)
	var revealed miniAppAuthTestResponse
	require.NoError(t, common.Unmarshal(keyRecorder.Body.Bytes(), &revealed))
	assert.Equal(t, "sk-mini-secret", revealed.Data.Key)

	otherUser := createMiniAppPasswordUser(t, db, "mini-key-other", "password123")
	foreignToken := &model.Token{
		UserId: otherUser.Id, Key: "sk-foreign-secret", Name: "foreign-key", Status: common.TokenStatusEnabled,
		CreatedTime: time.Now().Unix(), AccessedTime: time.Now().Unix(), ExpiredTime: -1,
	}
	require.NoError(t, db.Create(foreignToken).Error)
	foreignRecorder := httptest.NewRecorder()
	foreignContext, _ := gin.CreateTestContext(foreignRecorder)
	foreignContext.Request = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/mini/token/%d/key", foreignToken.Id), nil)
	foreignContext.Request.Header.Set("X-Security-Proof", verified.Data.ProofToken)
	foreignContext.Params = gin.Params{{Key: "id", Value: fmt.Sprint(foreignToken.Id)}}
	foreignContext.Set("id", identity.UserID)
	foreignContext.Set("session_id", identity.SessionID)
	foreignContext.Set("auth_version", identity.UserAuthVersion)
	foreignContext.Set("session_version", identity.SessionVersion)
	MiniAppGetTokenKey(foreignContext)
	assert.NotContains(t, foreignRecorder.Body.String(), "sk-foreign-secret")
}

func TestMiniAppSecurityProofRejectsIncorrectPassword(t *testing.T) {
	db := setupMiniAppAuthTest(t)
	user := createMiniAppPasswordUser(t, db, "mini-key-reject", "password123")
	bundle, err := service.CreateLoginSession(user.Id, "wechat_miniapp", "127.0.0.1", "miniapp-test")
	require.NoError(t, err)
	identity, err := service.ParseAccessToken(bundle.AccessToken)
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/mini/security/verify", strings.NewReader(`{"password":"incorrect"}`))
	context.Set("id", identity.UserID)
	context.Set("session_id", identity.SessionID)
	context.Set("auth_version", identity.UserAuthVersion)
	context.Set("session_version", identity.SessionVersion)
	MiniAppVerifySecurity(context)

	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
	var response miniAppAuthTestResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, "MINI_SECURITY_INVALID_CREDENTIALS", response.Code)
}

func setupMiniAppAuthTest(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB := model.DB
	previousLogDB := model.LOG_DB
	previousRedis := common.RedisEnabled
	previousSecret := common.SessionSecret
	previousRegister := common.RegisterEnabled
	previousPasswordRegister := common.PasswordRegisterEnabled
	previousPasswordLogin := common.PasswordLoginEnabled
	previousEmailVerification := common.EmailVerificationEnabled
	previousGenerateToken := constant.GenerateDefaultToken
	previousExchange := exchangeMiniAppCode
	legalSettings := system_setting.GetLegalSettings()
	previousAgreement := legalSettings.UserAgreement
	previousPrivacy := legalSettings.PrivacyPolicy

	dsn := fmt.Sprintf("file:miniapp-auth-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(
		&model.User{}, &model.UserSession{}, &model.AuthFlow{}, &model.ExternalIdentityClaim{},
		&model.TwoFA{}, &model.TwoFABackupCode{}, &model.Token{}, &model.Log{},
	))
	model.DB = db
	model.LOG_DB = db
	common.RedisEnabled = false
	common.SessionSecret = "miniapp-auth-controller-test-secret"
	common.RegisterEnabled = true
	common.PasswordRegisterEnabled = true
	common.PasswordLoginEnabled = true
	common.EmailVerificationEnabled = false
	constant.GenerateDefaultToken = false
	legalSettings.UserAgreement = ""
	legalSettings.PrivacyPolicy = ""
	exchangeMiniAppCode = func(_ context.Context, code string) (service.WeChatMiniAppIdentity, error) {
		return service.WeChatMiniAppIdentity{AppId: "wx-test-app", OpenId: "openid-" + code}, nil
	}
	gin.SetMode(gin.TestMode)

	t.Cleanup(func() {
		model.DB = previousDB
		model.LOG_DB = previousLogDB
		common.RedisEnabled = previousRedis
		common.SessionSecret = previousSecret
		common.RegisterEnabled = previousRegister
		common.PasswordRegisterEnabled = previousPasswordRegister
		common.PasswordLoginEnabled = previousPasswordLogin
		common.EmailVerificationEnabled = previousEmailVerification
		constant.GenerateDefaultToken = previousGenerateToken
		legalSettings.UserAgreement = previousAgreement
		legalSettings.PrivacyPolicy = previousPrivacy
		exchangeMiniAppCode = previousExchange
		_ = sqlDB.Close()
	})
	return db
}

func callMiniAppAuthHandler(t *testing.T, method, path, body string, handler gin.HandlerFunc) (*httptest.ResponseRecorder, miniAppAuthTestResponse) {
	t.Helper()
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(method, path, strings.NewReader(body))
	context.Request.Header.Set("Content-Type", "application/json")
	handler(context)
	var response miniAppAuthTestResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	return recorder, response
}

func createMiniAppPasswordUser(t *testing.T, db *gorm.DB, username, password string) *model.User {
	t.Helper()
	passwordHash, err := common.Password2Hash(password)
	require.NoError(t, err)
	user := &model.User{
		Username: username, Password: passwordHash, DisplayName: username,
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default",
		AffCode: username + "-aff", AuthVersion: 1,
	}
	require.NoError(t, db.Create(user).Error)
	return user
}

func createMiniAppBindingFlow(t *testing.T, appId, openId string, expiresAt time.Time) string {
	t.Helper()
	payload, err := common.Marshal(miniAppBindingFlowPayload{AppId: appId, OpenId: openId})
	require.NoError(t, err)
	token, _, err := model.CreateAuthFlow(model.AuthFlowCreate{
		Purpose:   model.AuthFlowPurposeWeChatMiniAppBind,
		Provider:  model.ExternalIdentityProviderWeChatMiniApp,
		Intent:    model.AuthFlowIntentBind,
		Payload:   string(payload),
		ExpiresAt: expiresAt,
	})
	require.NoError(t, err)
	return token
}

func TestMiniAppLoginCreatesOpaqueBindingFlowForUnknownIdentity(t *testing.T) {
	setupMiniAppAuthTest(t)
	recorder, response := callMiniAppAuthHandler(t, http.MethodPost, "/api/mini/auth/login", `{"code":"new-user"}`, MiniAppLogin)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.True(t, response.Success)
	assert.True(t, response.Data.BindingRequired)
	assert.NotEmpty(t, response.Data.FlowToken)
	assert.NotContains(t, recorder.Body.String(), "openid-new-user")
	flow, err := model.GetAuthFlow(response.Data.FlowToken, model.AuthFlowMatch{
		Purpose:  model.AuthFlowPurposeWeChatMiniAppBind,
		Provider: model.ExternalIdentityProviderWeChatMiniApp,
		Intent:   model.AuthFlowIntentBind,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, flow.Payload)
}

func TestMiniAppBindRefreshLogoutAndReplayProtection(t *testing.T) {
	db := setupMiniAppAuthTest(t)
	user := createMiniAppPasswordUser(t, db, "mini-bind-user", "password123")
	flowToken := createMiniAppBindingFlow(t, "wx-test-app", "openid-bind", time.Now().Add(time.Minute))

	bindBody := fmt.Sprintf(`{"flow_token":%q,"username":"mini-bind-user","password":"password123","accept_terms":true}`, flowToken)
	recorder, bound := callMiniAppAuthHandler(t, http.MethodPost, "/api/mini/auth/bind", bindBody, MiniAppBind)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.True(t, bound.Success)
	require.NotEmpty(t, bound.Data.AccessToken)
	require.NotEmpty(t, bound.Data.RefreshToken)
	require.NotEmpty(t, bound.Data.Session.SID)
	owner, err := model.GetUserIdByScopedExternalIdentity(model.ExternalIdentityProviderWeChatMiniApp, "wx-test-app", "openid-bind")
	require.NoError(t, err)
	assert.Equal(t, user.Id, owner)

	replayRecorder, replay := callMiniAppAuthHandler(t, http.MethodPost, "/api/mini/auth/bind", bindBody, MiniAppBind)
	assert.Equal(t, http.StatusConflict, replayRecorder.Code)
	assert.Equal(t, "MINI_AUTH_FLOW_CONSUMED", replay.Code)

	refreshBody := fmt.Sprintf(`{"refresh_token":%q,"sid":%q}`, bound.Data.RefreshToken, bound.Data.Session.SID)
	refreshRecorder, refreshed := callMiniAppAuthHandler(t, http.MethodPost, "/api/mini/auth/refresh", refreshBody, MiniAppRefresh)
	require.Equal(t, http.StatusOK, refreshRecorder.Code)
	require.True(t, refreshed.Success)
	assert.NotEqual(t, bound.Data.RefreshToken, refreshed.Data.RefreshToken)

	logoutBody := fmt.Sprintf(`{"refresh_token":%q,"sid":%q}`, refreshed.Data.RefreshToken, refreshed.Data.Session.SID)
	logoutRecorder, logout := callMiniAppAuthHandler(t, http.MethodPost, "/api/mini/auth/logout", logoutBody, MiniAppLogout)
	assert.Equal(t, http.StatusOK, logoutRecorder.Code)
	assert.True(t, logout.Success)
	stored, err := model.GetUserSessionBySID(refreshed.Data.Session.SID)
	require.NoError(t, err)
	assert.Equal(t, model.UserSessionStatusRevoked, stored.Status)
}

func TestMiniAppRegisterCreatesUserAndClaimsIdentityAtomically(t *testing.T) {
	db := setupMiniAppAuthTest(t)
	flowToken := createMiniAppBindingFlow(t, "wx-test-app", "openid-register", time.Now().Add(time.Minute))
	body := fmt.Sprintf(`{"flow_token":%q,"username":"mini-register","password":"password123","accept_terms":true}`, flowToken)

	recorder, response := callMiniAppAuthHandler(t, http.MethodPost, "/api/mini/auth/register", body, MiniAppRegister)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.True(t, response.Success)
	require.NotEmpty(t, response.Data.AccessToken)
	var user model.User
	require.NoError(t, db.Where("username = ?", "mini-register").First(&user).Error)
	owner, err := model.GetUserIdByScopedExternalIdentity(model.ExternalIdentityProviderWeChatMiniApp, "wx-test-app", "openid-register")
	require.NoError(t, err)
	assert.Equal(t, user.Id, owner)
}

func TestMiniAppEmailVerificationRequiresEnabledSettingAndValidFlow(t *testing.T) {
	db := setupMiniAppAuthTest(t)
	disabledRecorder, disabled := callMiniAppAuthHandler(
		t,
		http.MethodPost,
		"/api/mini/auth/verification",
		`{"flow_token":"unused","email":"person@example.com"}`,
		MiniAppSendEmailVerification,
	)
	assert.Equal(t, http.StatusForbidden, disabledRecorder.Code)
	assert.Equal(t, "MINI_AUTH_EMAIL_VERIFICATION_DISABLED", disabled.Code)

	common.EmailVerificationEnabled = true
	flowToken := createMiniAppBindingFlow(t, "wx-test-app", "openid-verification", time.Now().Add(time.Minute))
	require.NoError(t, db.Model(&model.AuthFlow{}).
		Where("purpose = ?", model.AuthFlowPurposeWeChatMiniAppBind).
		Update("expires_at", time.Now().Add(-time.Minute)).Error)
	body := fmt.Sprintf(`{"flow_token":%q,"email":"person@example.com"}`, flowToken)
	expiredRecorder, expired := callMiniAppAuthHandler(
		t,
		http.MethodPost,
		"/api/mini/auth/verification",
		body,
		MiniAppSendEmailVerification,
	)
	assert.Equal(t, http.StatusGone, expiredRecorder.Code)
	assert.Equal(t, "MINI_AUTH_FLOW_EXPIRED", expired.Code)
}

func TestMiniAppBindRejectsExpiredFlowAndIdentityConflict(t *testing.T) {
	db := setupMiniAppAuthTest(t)
	first := createMiniAppPasswordUser(t, db, "mini-first", "password123")
	second := createMiniAppPasswordUser(t, db, "mini-second", "password123")
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		return model.ClaimScopedExternalIdentityWithTx(tx, model.ExternalIdentityProviderWeChatMiniApp, "wx-test-app", "openid-conflict", first.Id)
	}))

	conflictFlow := createMiniAppBindingFlow(t, "wx-test-app", "openid-conflict", time.Now().Add(time.Minute))
	conflictBody := fmt.Sprintf(`{"flow_token":%q,"username":%q,"password":"password123","accept_terms":true}`, conflictFlow, second.Username)
	conflictRecorder, conflict := callMiniAppAuthHandler(t, http.MethodPost, "/api/mini/auth/bind", conflictBody, MiniAppBind)
	assert.Equal(t, http.StatusConflict, conflictRecorder.Code)
	assert.Equal(t, "MINI_AUTH_IDENTITY_CONFLICT", conflict.Code)

	expiredFlow := createMiniAppBindingFlow(t, "wx-test-app", "openid-expired", time.Now().Add(time.Minute))
	require.NoError(t, db.Model(&model.AuthFlow{}).Where("token_hash <> ?", "").Update("expires_at", time.Now().Add(-time.Minute)).Error)
	expiredBody := fmt.Sprintf(`{"flow_token":%q,"username":%q,"password":"password123","accept_terms":true}`, expiredFlow, second.Username)
	expiredRecorder, expired := callMiniAppAuthHandler(t, http.MethodPost, "/api/mini/auth/bind", expiredBody, MiniAppBind)
	assert.Equal(t, http.StatusGone, expiredRecorder.Code)
	assert.Equal(t, "MINI_AUTH_FLOW_EXPIRED", expired.Code)
}
