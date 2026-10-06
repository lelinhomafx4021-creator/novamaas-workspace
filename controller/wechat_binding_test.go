package controller

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func callWeChatAccountHandler(t *testing.T, user *model.User, identity service.AuthIdentity, proof, target, path, body string, handler gin.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	c.Request.Header.Set("X-Security-Proof", proof)
	c.Set("id", user.Id)
	c.Set("role", user.Role)
	c.Set("session_id", identity.SessionID)
	c.Set("auth_version", identity.UserAuthVersion)
	c.Set("session_version", identity.SessionVersion)
	if target != "" {
		c.Params = gin.Params{{Key: "id", Value: target}}
	}
	handler(c)
	return recorder
}

func TestWeChatPhoneMatchRequiresConfirmationAndRecordsScopedProfile(t *testing.T) {
	db := setupMiniAppAuthTest(t)
	user := createMiniAppPasswordUser(t, db, "phone-confirm", "password123")
	require.NoError(t, db.Create(&model.VerifiedPhone{Phone: "+8613800001234", UserId: user.Id, Source: "aliyun_sms", VerifiedAt: time.Now()}).Error)
	_, required := callMiniAppAuthHandler(t, http.MethodPost, "/api/mini/auth/phone", `{"code":"confirm","phone_code":"1234"}`, MiniAppPhoneLogin)
	require.True(t, required.Data.BindingRequired)
	require.NotEmpty(t, required.Data.FlowToken)
	assert.Equal(t, user.Username, required.Data.MatchedAccount.Username)
	assert.Equal(t, user.DisplayName, required.Data.MatchedAccount.DisplayName)
	assert.Equal(t, "+86 138****1234", required.Data.MatchedAccount.PhoneHint)
	assert.True(t, required.Data.ConfirmationAvailable)
	assert.False(t, required.Data.RegistrationEnabled)
	var imageBytes bytes.Buffer
	require.NoError(t, png.Encode(&imageBytes, image.NewRGBA(image.Rect(0, 0, 32, 32))))
	encoded := base64.StdEncoding.EncodeToString(imageBytes.Bytes())
	badRecorder, bad := callMiniAppAuthHandler(t, http.MethodPost, "/api/mini/auth/bind", fmt.Sprintf(`{"flow_token":%q,"confirm_only":true,"nickname":"Selected","avatar_base64":"not-an-image"}`, required.Data.FlowToken), MiniAppBind)
	assert.Equal(t, http.StatusBadRequest, badRecorder.Code)
	assert.Equal(t, "MINI_AUTH_PROFILE_INVALID", bad.Code)
	_, err := model.GetUserIdByScopedExternalIdentity(model.ExternalIdentityProviderWeChatMiniApp, "wx-test-app", "openid-confirm")
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	old, err := service.CreateLoginSession(user.Id, "password", "test", "test")
	require.NoError(t, err)
	_, result := callMiniAppAuthHandler(t, http.MethodPost, "/api/mini/auth/bind", fmt.Sprintf(`{"flow_token":%q,"confirm_only":true,"nickname":" Selected name ","avatar_base64":%q}`, required.Data.FlowToken, encoded), MiniAppBind)
	require.True(t, result.Success)
	require.NotEmpty(t, result.Data.AccessToken)
	profiles, err := model.GetWeChatBindings(user.Id, true)
	require.NoError(t, err)
	require.Len(t, profiles, 1)
	assert.Equal(t, "openid-confirm", profiles[0].OpenId)
	assert.Equal(t, "phone-link-v1", profiles[0].ConsentVersion)
	assert.Equal(t, "Selected name", profiles[0].Nickname)
	assert.True(t, profiles[0].HasAvatar)
	assert.Equal(t, "user_selected", profiles[0].ProfileSource)
	public, err := model.GetWeChatBindings(user.Id, false)
	require.NoError(t, err)
	assert.Empty(t, public[0].OpenId)
	assert.Empty(t, public[0].UnionId)
	oldIdentity, err := service.ParseAccessToken(old.AccessToken)
	require.NoError(t, err)
	_, err = service.CreateLoginSessionAtAuthVersion(user.Id, oldIdentity.UserAuthVersion, "wechat_miniapp", "test", "test")
	assert.Error(t, err)
	_, replay := callMiniAppAuthHandler(t, http.MethodPost, "/api/mini/auth/bind", fmt.Sprintf(`{"flow_token":%q,"confirm_only":true}`, required.Data.FlowToken), MiniAppBind)
	assert.False(t, replay.Success)
}

func TestWeChatConfirmationRejectsChangedAccountAndUnverifiedPhone(t *testing.T) {
	db := setupMiniAppAuthTest(t)
	user := createMiniAppPasswordUser(t, db, "stale-confirm", "password123")
	require.NoError(t, db.Create(&model.VerifiedPhone{Phone: "+8613800001234", UserId: user.Id, Source: "aliyun_sms", VerifiedAt: time.Now()}).Error)
	_, flow := callMiniAppAuthHandler(t, http.MethodPost, "/api/mini/auth/phone", `{"code":"stale","phone_code":"1234"}`, MiniAppPhoneLogin)
	require.NoError(t, db.Model(user).Update("auth_version", 2).Error)
	_, result := callMiniAppAuthHandler(t, http.MethodPost, "/api/mini/auth/bind", fmt.Sprintf(`{"flow_token":%q,"confirm_only":true}`, flow.Data.FlowToken), MiniAppBind)
	assert.False(t, result.Success)
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		return model.ClaimScopedExternalIdentityWithTx(tx, model.ExternalIdentityProviderWeChatMiniApp, "wx-test-app", "openid-unverified", user.Id)
	}))
	require.NoError(t, db.Where("user_id = ?", user.Id).Delete(&model.VerifiedPhone{}).Error)
	_, result = callMiniAppAuthHandler(t, http.MethodPost, "/api/mini/auth/phone", `{"code":"unverified","phone_code":"1234"}`, MiniAppPhoneLogin)
	assert.False(t, result.Success)
	assert.Empty(t, result.Data.AccessToken)
}

func TestWeChatUnlinkRevokesSessionsAndCannotRestoreUsingOldFlow(t *testing.T) {
	db := setupMiniAppAuthTest(t)
	user := createMiniAppPasswordUser(t, db, "unlink-user", "password123")
	require.NoError(t, db.Create(&model.VerifiedPhone{Phone: "+8613800001234", UserId: user.Id, Source: "aliyun_sms", VerifiedAt: time.Now()}).Error)
	_, pending := callMiniAppAuthHandler(t, http.MethodPost, "/api/mini/auth/phone", `{"code":"pending","phone_code":"1234"}`, MiniAppPhoneLogin)
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		if err := model.ClaimScopedExternalIdentityWithTx(tx, model.ExternalIdentityProviderWeChatMiniApp, "wx-test-app", "openid-unlink", user.Id); err != nil {
			return err
		}
		return model.RecordWeChatBindingWithTx(tx, user.Id, "wx-test-app", "union-user", "+8613800001234")
	}))
	bundle, err := service.CreateLoginSession(user.Id, "wechat_miniapp", "test", "test")
	require.NoError(t, err)
	identity, err := service.ParseAccessToken(bundle.AccessToken)
	require.NoError(t, err)
	proof, _, err := service.IssueSecurityProof(identity, "password", []string{"wechat.manage"})
	require.NoError(t, err)
	result := callWeChatAccountHandler(t, user, identity, proof, "", "/api/user/self/wechat-miniapp/unbind", `{"app_id":"wx-test-app"}`, UnbindWeChat)
	require.Equal(t, http.StatusOK, result.Code)
	var count int64
	require.NoError(t, db.Model(&model.UserSession{}).Where("user_id = ? AND status = ?", user.Id, model.UserSessionStatusActive).Count(&count).Error)
	assert.Zero(t, count)
	profiles, err := model.GetWeChatBindings(user.Id, true)
	require.NoError(t, err)
	assert.Empty(t, profiles)
	_, stale := callMiniAppAuthHandler(t, http.MethodPost, "/api/mini/auth/bind", fmt.Sprintf(`{"flow_token":%q,"confirm_only":true}`, pending.Data.FlowToken), MiniAppBind)
	assert.False(t, stale.Success)
	_, next := callMiniAppAuthHandler(t, http.MethodPost, "/api/mini/auth/phone", `{"code":"unlink","phone_code":"1234"}`, MiniAppPhoneLogin)
	assert.True(t, next.Data.BindingRequired)
	assert.Empty(t, next.Data.AccessToken)
}

func TestWeChatAdminUnlinkRejectsPeersAndLastCredential(t *testing.T) {
	db := setupMiniAppAuthTest(t)
	require.NoError(t, db.AutoMigrate(&model.UserOAuthBinding{}, &model.CustomOAuthProvider{}))
	admin := createMiniAppPasswordUser(t, db, "unlink-admin", "password123")
	admin.Role = common.RoleAdminUser
	require.NoError(t, db.Model(admin).Update("role", admin.Role).Error)
	target := createMiniAppPasswordUser(t, db, "unlink-target", "password123")
	target.Role = common.RoleAdminUser
	require.NoError(t, db.Model(target).Update("role", target.Role).Error)
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		return model.ClaimScopedExternalIdentityWithTx(tx, model.ExternalIdentityProviderWeChatMiniApp, "wx-test-app", "openid-last", target.Id)
	}))
	bundle, err := service.CreateLoginSession(admin.Id, "password", "test", "test")
	require.NoError(t, err)
	identity, err := service.ParseAccessToken(bundle.AccessToken)
	require.NoError(t, err)
	proof, _, err := service.IssueSecurityProof(identity, "password", []string{"wechat.manage"})
	require.NoError(t, err)
	result := callWeChatAccountHandler(t, admin, identity, proof, fmt.Sprint(target.Id), "/api/user/target/wechat-miniapp/unbind", `{"app_id":"wx-test-app"}`, UnbindWeChat)
	assert.Equal(t, http.StatusForbidden, result.Code)
	admin.Role = common.RoleRootUser
	require.NoError(t, db.Model(target).Update("password", "").Error)
	result = callWeChatAccountHandler(t, admin, identity, proof, fmt.Sprint(target.Id), "/api/user/target/wechat-miniapp/unbind", `{"app_id":"wx-test-app"}`, UnbindWeChat)
	assert.Equal(t, http.StatusConflict, result.Code)
	assert.Contains(t, result.Body.String(), "MINI_AUTH_LAST_CREDENTIAL")
	require.NoError(t, db.Model(target).Update("password", admin.Password).Error)
	// The target now has an independent password login; root can unlink it.
	result = callWeChatAccountHandler(t, admin, identity, proof, fmt.Sprint(target.Id), "/api/user/target/wechat-miniapp/unbind", `{"app_id":"wx-test-app"}`, UnbindWeChat)
	assert.Equal(t, http.StatusOK, result.Code)
}

func TestWeChatProfileRestrictsOwnershipAndNormalizesAvatar(t *testing.T) {
	db := setupMiniAppAuthTest(t)
	user := createMiniAppPasswordUser(t, db, "profile-user", "password123")
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		return model.ClaimScopedExternalIdentityWithTx(tx, model.ExternalIdentityProviderWeChatMiniApp, "wx-test-app", "openid-profile", user.Id)
	}))
	bundle, err := service.CreateLoginSession(user.Id, "password", "test", "test")
	require.NoError(t, err)
	identity, err := service.ParseAccessToken(bundle.AccessToken)
	require.NoError(t, err)
	var imageBytes bytes.Buffer
	require.NoError(t, png.Encode(&imageBytes, image.NewRGBA(image.Rect(0, 0, 512, 256))))
	encoded := base64.StdEncoding.EncodeToString(imageBytes.Bytes())
	response := callWeChatAccountHandler(t, user, identity, "", "", "/api/user/self/wechat-miniapp/profile", fmt.Sprintf(`{"app_id":"wx-test-app","nickname":"微信昵称","avatar_base64":%q}`, encoded), UpdateWeChatProfile)
	require.Equal(t, http.StatusOK, response.Code)
	var profile model.WeChatMiniAppProfile
	require.NoError(t, db.Where("user_id = ?", user.Id).First(&profile).Error)
	assert.Equal(t, "微信昵称", profile.Nickname)
	config, format, err := image.DecodeConfig(bytes.NewReader(profile.Avatar))
	require.NoError(t, err)
	assert.Equal(t, "jpeg", format)
	assert.Equal(t, 256, config.Width)
	assert.Equal(t, 128, config.Height)
	response = callWeChatAccountHandler(t, user, identity, "", "", "/api/user/self/wechat-miniapp/profile", `{"app_id":"other-app","nickname":"bad"}`, UpdateWeChatProfile)
	assert.NotEqual(t, http.StatusOK, response.Code)
	_, err = normalizeWeChatAvatar(base64.StdEncoding.EncodeToString([]byte("not an image")))
	assert.Error(t, err)
}

func TestWeChatPhoneWithoutVerifiedPlatformAccountStopsBeforeCreatingFlow(t *testing.T) {
	db := setupMiniAppAuthTest(t)
	recorder, response := callMiniAppAuthHandler(t, http.MethodPost, "/api/mini/auth/phone", `{"code":"unmatched","phone_code":"1234"}`, MiniAppPhoneLogin)
	assert.Equal(t, http.StatusNotFound, recorder.Code)
	assert.Equal(t, "MINI_AUTH_PHONE_ACCOUNT_NOT_FOUND", response.Code)
	assert.Empty(t, response.Data.FlowToken)
	assert.Empty(t, response.Data.AccessToken)
	for _, table := range []interface{}{&model.AuthFlow{}, &model.UserSession{}, &model.ExternalIdentityClaim{}, &model.WeChatMiniAppProfile{}} {
		var count int64
		require.NoError(t, db.Model(table).Count(&count).Error)
		assert.Zero(t, count)
	}
	user := createMiniAppPasswordUser(t, db, "arbitrary-account", "password123")
	token := createMiniAppBindingFlow(t, "wx-test-app", "openid-cached-unmatched", time.Now().Add(time.Minute), "+8613800001234")
	recorder, response = callMiniAppAuthHandler(t, http.MethodPost, "/api/mini/auth/bind", fmt.Sprintf(`{"flow_token":%q,"username":%q,"password":"password123"}`, token, user.Username), MiniAppBind)
	assert.Equal(t, http.StatusNotFound, recorder.Code)
	assert.Equal(t, "MINI_AUTH_PHONE_ACCOUNT_NOT_FOUND", response.Code)
	_, err := model.GetVerifiedPhoneByUser(user.Id)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	_, err = model.GetUserIdByScopedExternalIdentity(model.ExternalIdentityProviderWeChatMiniApp, "wx-test-app", "openid-cached-unmatched")
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestWeChatPhoneConfirmationUsesVerifiedPhoneForEveryAccountRole(t *testing.T) {
	for _, scenario := range []struct {
		name          string
		role          int
		mfa           bool
		passwordLogin bool
		confirmOnly   bool
	}{
		{"admin", common.RoleAdminUser, false, true, true},
		{"mfa", common.RoleCommonUser, true, true, true},
		{"root-mfa", common.RoleRootUser, true, true, true},
		{"password-login-disabled", common.RoleAdminUser, true, false, true},
		{"legacy-client", common.RoleAdminUser, true, true, false},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			db := setupMiniAppAuthTest(t)
			owner := createMiniAppPasswordUser(t, db, "matched-owner", "ownerpassword")
			other := createMiniAppPasswordUser(t, db, "other-account", "otherpassword")
			require.NoError(t, db.Model(owner).Update("role", scenario.role).Error)
			require.NoError(t, db.Create(&model.VerifiedPhone{Phone: "+8613800001234", UserId: owner.Id, Source: "aliyun_sms", VerifiedAt: time.Now()}).Error)
			if scenario.mfa {
				require.NoError(t, db.Create(&model.TwoFA{UserId: owner.Id, Secret: "test-secret", IsEnabled: true}).Error)
			}
			common.PasswordLoginEnabled = scenario.passwordLogin
			_, flow := callMiniAppAuthHandler(t, http.MethodPost, "/api/mini/auth/phone", `{"code":"matched","phone_code":"1234"}`, MiniAppPhoneLogin)
			require.True(t, flow.Success)
			assert.Equal(t, owner.Username, flow.Data.MatchedAccount.Username)
			assert.True(t, flow.Data.ConfirmationAvailable)
			assert.False(t, flow.Data.TwoFactorRequired)
			assert.Empty(t, flow.Data.AccessToken)
			// Client-supplied account names never select the target. No password
			// or authenticator is needed after the server verifies both phones.
			_, confirmed := callMiniAppAuthHandler(t, http.MethodPost, "/api/mini/auth/bind", fmt.Sprintf(`{"flow_token":%q,"confirm_only":%t,"username":%q}`, flow.Data.FlowToken, scenario.confirmOnly, other.Username), MiniAppBind)
			require.True(t, confirmed.Success)
			require.NotEmpty(t, confirmed.Data.AccessToken)
			linked, err := model.GetUserIdByScopedExternalIdentity(model.ExternalIdentityProviderWeChatMiniApp, "wx-test-app", "openid-matched")
			require.NoError(t, err)
			assert.Equal(t, owner.Id, linked)
			var otherClaims int64
			require.NoError(t, db.Model(&model.ExternalIdentityClaim{}).Where("user_id = ?", other.Id).Count(&otherClaims).Error)
			assert.Zero(t, otherClaims)
			_, replay := callMiniAppAuthHandler(t, http.MethodPost, "/api/mini/auth/bind", fmt.Sprintf(`{"flow_token":%q,"confirm_only":true}`, flow.Data.FlowToken), MiniAppBind)
			assert.False(t, replay.Success)
			assert.Equal(t, "MINI_AUTH_FLOW_CONSUMED", replay.Code)
		})
	}
}

func TestWeChatBindingWithoutPhoneStillRequiresAccountCredentials(t *testing.T) {
	db := setupMiniAppAuthTest(t)
	owner := createMiniAppPasswordUser(t, db, "code-only-owner", "ownerpassword")
	require.NoError(t, db.Create(&model.TwoFA{UserId: owner.Id, Secret: "test-secret", IsEnabled: true}).Error)
	token := createMiniAppBindingFlow(t, "wx-test-app", "openid-code-only", time.Now().Add(time.Minute))
	_, response := callMiniAppAuthHandler(t, http.MethodPost, "/api/mini/auth/bind", fmt.Sprintf(`{"flow_token":%q,"confirm_only":true}`, token), MiniAppBind)
	assert.False(t, response.Success)
	assert.Equal(t, "MINI_AUTH_FLOW_INVALID", response.Code)
	_, response = callMiniAppAuthHandler(t, http.MethodPost, "/api/mini/auth/bind", fmt.Sprintf(`{"flow_token":%q,"username":%q,"password":"ownerpassword"}`, token, owner.Username), MiniAppBind)
	assert.False(t, response.Success)
	assert.Equal(t, "MINI_AUTH_2FA_REQUIRED", response.Code)
	_, err := model.GetUserIdByScopedExternalIdentity(model.ExternalIdentityProviderWeChatMiniApp, "wx-test-app", "openid-code-only")
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestWeChatPhoneConfirmationRejectsOwnershipChanges(t *testing.T) {
	for _, reassigned := range []bool{false, true} {
		t.Run(fmt.Sprint(reassigned), func(t *testing.T) {
			db := setupMiniAppAuthTest(t)
			owner := createMiniAppPasswordUser(t, db, "changed-owner", "password123")
			require.NoError(t, db.Create(&model.VerifiedPhone{Phone: "+8613800001234", UserId: owner.Id, Source: "aliyun_sms", VerifiedAt: time.Now()}).Error)
			_, flow := callMiniAppAuthHandler(t, http.MethodPost, "/api/mini/auth/phone", `{"code":"changed","phone_code":"1234"}`, MiniAppPhoneLogin)
			require.True(t, flow.Success)
			require.NoError(t, db.Where("user_id = ?", owner.Id).Delete(&model.VerifiedPhone{}).Error)
			if reassigned {
				other := createMiniAppPasswordUser(t, db, "new-phone-owner", "password123")
				require.NoError(t, db.Create(&model.VerifiedPhone{Phone: "+8613800001234", UserId: other.Id, Source: "aliyun_sms", VerifiedAt: time.Now()}).Error)
			}
			_, response := callMiniAppAuthHandler(t, http.MethodPost, "/api/mini/auth/bind", fmt.Sprintf(`{"flow_token":%q,"confirm_only":true}`, flow.Data.FlowToken), MiniAppBind)
			assert.False(t, response.Success)
			assert.Equal(t, "MINI_AUTH_FLOW_INVALID", response.Code)
			assert.Empty(t, response.Data.AccessToken)
			var count int64
			require.NoError(t, db.Model(&model.ExternalIdentityClaim{}).Count(&count).Error)
			assert.Zero(t, count)
			require.NoError(t, db.Model(&model.UserSession{}).Count(&count).Error)
			assert.Zero(t, count)
		})
	}
}
