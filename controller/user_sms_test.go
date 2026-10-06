package controller

import (
	"fmt"
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

func setupSMSControllerTest(t *testing.T) (*gorm.DB, *model.User) {
	t.Helper()
	db := setupMiniAppAuthTest(t)
	require.NoError(t, db.AutoMigrate(&model.SMSChallenge{}, &model.SMSBudget{}))
	for key, value := range map[string]string{"SMS_ENABLED": "true", "ALIYUN_SMS_ACCESS_KEY_ID": "test-key", "ALIYUN_SMS_ACCESS_KEY_SECRET": "test-secret", "ALIYUN_SMS_SIGN_NAME": "test", "ALIYUN_SMS_TEMPLATE_CODE": "SMS_TEST"} {
		t.Setenv(key, value)
	}
	return db, createMiniAppPasswordUser(t, db, "sms-controller-user", "password123")
}

func deliveredSMS(t *testing.T, input model.SMSChallengeInput) (string, string) {
	t.Helper()
	token, code, challenge, err := model.ReserveSMSChallenge(input)
	require.NoError(t, err)
	require.NoError(t, model.MarkSMSDelivered(challenge.Id))
	return token, code
}

func TestSMSLoginAcceptsVerifiedPhoneWithMFAAndRejectsReplay(t *testing.T) {
	db, user := setupSMSControllerTest(t)
	require.NoError(t, db.Create(&model.VerifiedPhone{Phone: "+8613800138000", UserId: user.Id, Source: "aliyun_sms", VerifiedAt: time.Now()}).Error)
	require.NoError(t, db.Create(&model.TwoFA{UserId: user.Id, Secret: "test", IsEnabled: true}).Error)
	token, code := deliveredSMS(t, model.SMSChallengeInput{Purpose: "login", Phone: "13800138000", UserId: user.Id, AuthVersion: user.AuthVersion, IP: "test"})
	_, result := callMiniAppAuthHandler(t, http.MethodPost, "/api/user/login/sms", fmt.Sprintf(`{"challenge_token":%q,"code":%q}`, token, code), SMSLogin)
	require.True(t, result.Success)
	assert.NotEmpty(t, result.Data.AccessToken)
	recorder, result := callMiniAppAuthHandler(t, http.MethodPost, "/api/user/login/sms", fmt.Sprintf(`{"challenge_token":%q,"code":%q}`, token, code), SMSLogin)
	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
	assert.False(t, result.Success)
}

func TestSMSLoginRejectsUnverifiedLegacyPhoneAndSecurityChanges(t *testing.T) {
	for _, scenario := range []string{"legacy-only", "disabled", "version-changed", "binding-changed"} {
		t.Run(scenario, func(t *testing.T) {
			db, user := setupSMSControllerTest(t)
			require.NoError(t, db.Model(user).Update("phone", "13800138000").Error)
			if scenario != "legacy-only" {
				require.NoError(t, db.Create(&model.VerifiedPhone{Phone: "+8613800138000", UserId: user.Id, Source: "aliyun_sms", VerifiedAt: time.Now()}).Error)
			}
			token, code := deliveredSMS(t, model.SMSChallengeInput{Purpose: "login", Phone: "13800138000", UserId: user.Id, AuthVersion: user.AuthVersion, IP: "test"})
			switch scenario {
			case "disabled":
				require.NoError(t, db.Model(user).Update("status", common.UserStatusDisabled).Error)
			case "version-changed":
				require.NoError(t, db.Model(user).Update("auth_version", 2).Error)
			case "binding-changed":
				require.NoError(t, db.Model(&model.VerifiedPhone{}).Where("user_id = ?", user.Id).Update("phone", "+8613900139000").Error)
			}
			recorder, response := callMiniAppAuthHandler(t, http.MethodPost, "/api/user/login/sms", fmt.Sprintf(`{"challenge_token":%q,"code":%q}`, token, code), SMSLogin)
			assert.Equal(t, http.StatusUnauthorized, recorder.Code)
			assert.False(t, response.Success)
			var count int64
			require.NoError(t, db.Model(&model.UserSession{}).Count(&count).Error)
			assert.Zero(t, count)
		})
	}
}

func TestSMSMFARequiresMatchingPasswordLoginFlow(t *testing.T) {
	db, user := setupSMSControllerTest(t)
	require.NoError(t, db.Create(&model.VerifiedPhone{Phone: "+8613800138000", UserId: user.Id, Source: "aliyun_sms", VerifiedAt: time.Now()}).Error)
	payload, err := common.Marshal(twoFALoginFlowPayload{AuthVersion: user.AuthVersion})
	require.NoError(t, err)
	flow, _, err := model.CreateAuthFlow(model.AuthFlowCreate{Purpose: model.AuthFlowPurposeTwoFALogin, UserId: user.Id, Payload: string(payload), ExpiresAt: time.Now().Add(time.Minute)})
	require.NoError(t, err)
	token, code := deliveredSMS(t, model.SMSChallengeInput{Purpose: "mfa_login", Phone: "13800138000", UserId: user.Id, AuthVersion: user.AuthVersion, FlowToken: flow, IP: "test"})
	_, rejected := callMiniAppAuthHandler(t, http.MethodPost, "/api/user/login/sms/mfa", fmt.Sprintf(`{"challenge_token":%q,"code":%q,"flow_token":"forged-flow"}`, token, code), SMSMFALogin)
	assert.False(t, rejected.Success)
	_, response := callMiniAppAuthHandler(t, http.MethodPost, "/api/user/login/sms/mfa", fmt.Sprintf(`{"challenge_token":%q,"code":%q,"flow_token":%q}`, token, code, flow), SMSMFALogin)
	require.True(t, response.Success)
	assert.NotEmpty(t, response.Data.AccessToken)
	_, err = model.GetAuthFlow(flow, model.AuthFlowMatch{Purpose: model.AuthFlowPurposeTwoFALogin})
	assert.ErrorIs(t, err, model.ErrAuthFlowConsumed)
}

func TestMiniSMSRequiresFreshWeChatCodeAndNeverBindsWeChat(t *testing.T) {
	db, user := setupSMSControllerTest(t)
	require.NoError(t, db.Create(&model.VerifiedPhone{Phone: "+8613800138000", UserId: user.Id, Source: "aliyun_sms", VerifiedAt: time.Now()}).Error)
	previousSender := sendPhoneSMS
	t.Cleanup(func() { sendPhoneSMS = previousSender })
	var sentCode string
	sendPhoneSMS = func(phone, code string) error { sentCode = code; return nil }
	recorder, rejected := callMiniAppAuthHandler(t, http.MethodPost, "/api/mini/auth/sms/code", `{"phone":"13800138000"}`, SendSMSLoginCode)
	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.False(t, rejected.Success)
	assert.Empty(t, sentCode)
	recorder, _ = callMiniAppAuthHandler(t, http.MethodPost, "/api/mini/auth/sms/code", `{"phone":"13800138000","wx_code":"native-sms"}`, SendSMSLoginCode)
	require.Equal(t, http.StatusOK, recorder.Code)
	var challenge struct {
		Data struct {
			Token string `json:"challenge_token"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &challenge))
	_, replay := callMiniAppAuthHandler(t, http.MethodPost, "/api/mini/auth/sms/code", `{"phone":"13900139000","wx_code":"native-sms"}`, SendSMSLoginCode)
	assert.False(t, replay.Success)
	_, login := callMiniAppAuthHandler(t, http.MethodPost, "/api/mini/auth/sms", fmt.Sprintf(`{"challenge_token":%q,"code":%q}`, challenge.Data.Token, sentCode), SMSLogin)
	require.True(t, login.Success)
	assert.NotEmpty(t, login.Data.RefreshToken)
	var count int64
	require.NoError(t, db.Model(&model.ExternalIdentityClaim{}).Where("user_id = ?", user.Id).Count(&count).Error)
	assert.Zero(t, count)
}

func TestMiniPasswordLoginOffersFlowBoundSMSAsAlternativeToMFA(t *testing.T) {
	db, user := setupSMSControllerTest(t)
	require.NoError(t, db.Create(&model.VerifiedPhone{Phone: "+8613800138000", UserId: user.Id, Source: "aliyun_sms", VerifiedAt: time.Now()}).Error)
	require.NoError(t, db.Create(&model.TwoFA{UserId: user.Id, Secret: "test", IsEnabled: true}).Error)
	recorder, pending := callMiniAppAuthHandler(t, http.MethodPost, "/api/mini/auth/password", `{"username":"sms-controller-user","password":"password123"}`, MiniAppPasswordLogin)
	require.Equal(t, http.StatusForbidden, recorder.Code)
	require.NotEmpty(t, pending.Data.FlowToken)
	token, code := deliveredSMS(t, model.SMSChallengeInput{Purpose: "mfa_login", Phone: "13800138000", UserId: user.Id, AuthVersion: user.AuthVersion, FlowToken: pending.Data.FlowToken, IP: "test"})
	body := fmt.Sprintf(`{"flow_token":%q,"challenge_token":%q,"code":%q}`, pending.Data.FlowToken, token, code)
	_, browser := callMiniAppAuthHandler(t, http.MethodPost, "/api/user/login/sms/mfa", body, SMSMFALogin)
	assert.False(t, browser.Success)
	_, mini := callMiniAppAuthHandler(t, http.MethodPost, "/api/mini/auth/sms/mfa", body, SMSMFALogin)
	require.True(t, mini.Success)
	assert.NotEmpty(t, mini.Data.RefreshToken)
}

func TestCurrentPhoneSMSProofIsSessionBoundAndCannotAuthorizeAdministrativeActions(t *testing.T) {
	db, user := setupSMSControllerTest(t)
	require.NoError(t, db.Create(&model.VerifiedPhone{Phone: "+8613800138000", UserId: user.Id, Source: "aliyun_sms", VerifiedAt: time.Now()}).Error)
	bundle, err := service.CreateLoginSession(user.Id, "password", "test", "test")
	require.NoError(t, err)
	identity, err := service.ParseAccessToken(bundle.AccessToken)
	require.NoError(t, err)
	token, code := deliveredSMS(t, model.SMSChallengeInput{Purpose: "security", Phone: "13800138000", UserId: user.Id, AuthVersion: user.AuthVersion, SessionId: identity.SessionID, IP: "test"})
	body := fmt.Sprintf(`{"challenge_token":%q,"code":%q,"scope":"phone.manage"}`, token, code)
	other := identity
	other.SessionID = "another-session"
	response := callWeChatAccountHandler(t, user, other, "", "", "/api/user/self/phone/security/verify", body, PhoneSecurityVerify)
	assert.Equal(t, http.StatusUnauthorized, response.Code)
	t.Setenv("SMS_ENABLED", "false")
	response = callWeChatAccountHandler(t, user, identity, "", "", "/api/user/self/phone/security/verify", body, PhoneSecurityVerify)
	assert.Equal(t, http.StatusServiceUnavailable, response.Code)
	t.Setenv("SMS_ENABLED", "true")
	response = callWeChatAccountHandler(t, user, identity, "", "", "/api/user/self/phone/security/verify", body, PhoneSecurityVerify)
	require.Equal(t, http.StatusOK, response.Code)
	var result miniAppAuthTestResponse
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &result))
	method, err := service.VerifySecurityProof(result.Data.ProofToken, identity, "phone.manage", []string{"sms"})
	require.NoError(t, err)
	assert.Equal(t, "sms", method)
	_, err = service.VerifySecurityProof(result.Data.ProofToken, identity, "phone.manage", []string{"password", "2fa", "passkey"})
	assert.ErrorIs(t, err, service.ErrProofMethod)
	_, err = service.VerifySecurityProof(result.Data.ProofToken, identity, "channel.key.read", []string{"sms"})
	assert.ErrorIs(t, err, service.ErrProofScope)
}

func TestPhoneBindingRotatesCurrentSessionAndRejectsGenericEdits(t *testing.T) {
	db, user := setupSMSControllerTest(t)
	bundle, err := service.CreateLoginSession(user.Id, "password", "127.0.0.1", "test")
	require.NoError(t, err)
	identity, err := service.ParseAccessToken(bundle.AccessToken)
	require.NoError(t, err)
	proof, _, err := service.IssueSecurityProof(identity, "password", []string{"phone.manage"})
	require.NoError(t, err)
	token, code := deliveredSMS(t, model.SMSChallengeInput{Purpose: "phone_bind", Phone: "13800138000", UserId: user.Id, AuthVersion: user.AuthVersion, SessionId: identity.SessionID, IP: "test"})
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/user/self/phone/bind", strings.NewReader(fmt.Sprintf(`{"challenge_token":%q,"code":%q}`, token, code)))
	c.Request.Header.Set("X-Security-Proof", proof)
	c.Set("id", user.Id)
	c.Set("session_id", identity.SessionID)
	c.Set("auth_version", identity.UserAuthVersion)
	c.Set("session_version", identity.SessionVersion)
	ConfirmPhoneBinding(c)
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"access_token"`)
	claim, err := model.GetVerifiedPhoneByUser(user.Id)
	require.NoError(t, err)
	assert.Equal(t, "+8613800138000", claim.Phone)
	require.NoError(t, db.First(user, user.Id).Error)
	assert.EqualValues(t, 2, user.AuthVersion)
	user.Phone = "13900139000"
	err = user.Edit(false)
	assert.ErrorIs(t, err, model.ErrVerifiedPhoneProtected)
	unchanged, err := model.GetVerifiedPhoneByUser(user.Id)
	require.NoError(t, err)
	assert.Equal(t, claim.Phone, unchanged.Phone)
}

func TestSMSProviderFailureDoesNotActivateChallenge(t *testing.T) {
	db, _ := setupSMSControllerTest(t)
	_, err := service.IssueSMS(model.SMSChallengeInput{Purpose: "login", Phone: "13800138000", IP: "test"}, func(string, string) error { return service.ErrSMSUnavailable })
	assert.ErrorIs(t, err, service.ErrSMSUnavailable)
	var challenge model.SMSChallenge
	require.NoError(t, db.First(&challenge).Error)
	assert.False(t, challenge.Delivered)
	_, err = service.IssueSMS(model.SMSChallengeInput{Purpose: "login", Phone: "13800138000", IP: "test"}, func(string, string) error { t.Fatal("limited request must not reach provider"); return nil })
	assert.ErrorIs(t, err, model.ErrSMSRateLimit)
}
