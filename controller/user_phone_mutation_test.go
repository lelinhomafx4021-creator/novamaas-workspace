package controller

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupManagedPhoneTest(t *testing.T) (*gorm.DB, *model.User, service.AuthIdentity, string) {
	t.Helper()
	db, admin := setupSMSControllerTest(t)
	admin.Role = common.RoleAdminUser
	require.NoError(t, db.Model(admin).Update("role", admin.Role).Error)
	bundle, err := service.CreateLoginSession(admin.Id, "password", "test", "test")
	require.NoError(t, err)
	identity, err := service.ParseAccessToken(bundle.AccessToken)
	require.NoError(t, err)
	proof, _, err := service.IssueSecurityProof(identity, "password", []string{"phone.manage"})
	require.NoError(t, err)
	return db, admin, identity, proof
}

func sendManagedPhoneTestCode(t *testing.T, admin *model.User, identity service.AuthIdentity, proof, username, phone string, userID int) (string, string) {
	t.Helper()
	previous := sendPhoneSMS
	t.Cleanup(func() { sendPhoneSMS = previous })
	var deliveredCode string
	sendPhoneSMS = func(destination, code string) error {
		assert.Equal(t, "+8613800138000", destination)
		deliveredCode = code
		return nil
	}
	body, err := common.Marshal(map[string]any{"username": username, "phone": phone, "user_id": userID})
	require.NoError(t, err)
	recorder := callWeChatAccountHandler(t, admin, identity, proof, "", "/api/user/phone/code", string(body), SendUserMutationPhoneCode)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var response struct {
		Success bool `json:"success"`
		Data    struct {
			Token string `json:"challenge_token"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	require.NotEmpty(t, deliveredCode)
	return response.Data.Token, deliveredCode
}

func TestCreateUserVerifiesPhoneAtSaveAndRejectsDraftReuse(t *testing.T) {
	db, admin, identity, proof := setupManagedPhoneTest(t)
	missing := callWeChatAccountHandler(t, admin, identity, proof, "", "/api/user/", `{"username":"phone-create-user","password":"NewPassword123","role":1,"phone":"13800138000"}`, CreateUser)
	assert.Equal(t, http.StatusBadRequest, missing.Code)
	assert.Contains(t, missing.Body.String(), `"success":false`)
	token, code := sendManagedPhoneTestCode(t, admin, identity, proof, "phone-create-user", "+86 13800138000", 0)
	body := map[string]any{"username": "another-draft", "password": "NewPassword123", "role": 1, "phone": "+86 13800138000", "phone_challenge_token": token, "phone_verification_code": code}
	encoded, err := common.Marshal(body)
	require.NoError(t, err)
	rejected := callWeChatAccountHandler(t, admin, identity, proof, "", "/api/user/", string(encoded), CreateUser)
	assert.Contains(t, rejected.Body.String(), `"success":false`)
	var count int64
	require.NoError(t, db.Model(&model.User{}).Where("username = ?", "another-draft").Count(&count).Error)
	assert.Zero(t, count)
	body["username"] = "phone-create-user"
	encoded, err = common.Marshal(body)
	require.NoError(t, err)
	recorder := callWeChatAccountHandler(t, admin, identity, proof, "", "/api/user/", string(encoded), CreateUser)
	require.Contains(t, recorder.Body.String(), `"success":true`)
	var created model.User
	require.NoError(t, db.Where("username = ?", "phone-create-user").First(&created).Error)
	assert.Equal(t, "+8613800138000", created.Phone)
	claim, err := model.GetVerifiedPhoneByUser(created.Id)
	require.NoError(t, err)
	assert.Equal(t, created.Phone, claim.Phone)
	assert.Equal(t, "aliyun_sms", claim.Source)
	var challenge model.SMSChallenge
	require.NoError(t, db.First(&challenge).Error)
	assert.NotNil(t, challenge.ConsumedAt)
	recorder = callWeChatAccountHandler(t, admin, identity, proof, "", "/api/user/", string(encoded), CreateUser)
	assert.Contains(t, recorder.Body.String(), `"success":false`)
}

func TestUpdateUserPhoneVerificationCommitsClaimAndRevokesSessions(t *testing.T) {
	for _, scenario := range []string{"bound-replacement", "legacy-verification", "disabled-account"} {
		t.Run(scenario, func(t *testing.T) {
			db, admin, identity, proof := setupManagedPhoneTest(t)
			target := createMiniAppPasswordUser(t, db, "phone-update-user", "password123")
			oldPhone := "+8613900139000"
			if scenario == "legacy-verification" {
				oldPhone = "+8613800138000"
			}
			require.NoError(t, db.Model(target).Updates(map[string]any{"phone": oldPhone, "display_name": "Before"}).Error)
			if scenario != "legacy-verification" {
				require.NoError(t, db.Transaction(func(tx *gorm.DB) error { return model.ClaimVerifiedPhoneWithTx(tx, oldPhone, target.Id, "aliyun_sms") }))
			}
			bundle, err := service.CreateLoginSession(target.Id, "password", "test", "test")
			require.NoError(t, err)
			if scenario == "disabled-account" {
				require.NoError(t, db.Model(target).Update("status", common.UserStatusDisabled).Error)
			}
			token, code := sendManagedPhoneTestCode(t, admin, identity, proof, target.Username, "13800138000", target.Id)
			body := fmt.Sprintf(`{"id":%d,"username":%q,"display_name":"After","group":"default","phone":"+86 13800138000","phone_challenge_token":%q,"phone_verification_code":%q}`, target.Id, target.Username, token, code)
			recorder := callWeChatAccountHandler(t, admin, identity, proof, "", "/api/user/", body, UpdateUser)
			require.Contains(t, recorder.Body.String(), `"success":true`)
			var updated model.User
			require.NoError(t, db.First(&updated, target.Id).Error)
			assert.Equal(t, "+8613800138000", updated.Phone)
			assert.Equal(t, "After", updated.DisplayName)
			assert.Equal(t, int64(2), updated.AuthVersion)
			claim, err := model.GetVerifiedPhoneByUser(target.Id)
			require.NoError(t, err)
			assert.Equal(t, updated.Phone, claim.Phone)
			oldIdentity, err := service.ParseAccessToken(bundle.AccessToken)
			require.NoError(t, err)
			var session model.UserSession
			require.NoError(t, db.Where("sid = ?", oldIdentity.SessionID).First(&session).Error)
			assert.Equal(t, model.UserSessionStatusRevoked, session.Status)
		})
	}
}

func TestManagedPhoneFailuresKeepUserAndOldClaimIntact(t *testing.T) {
	for _, scenario := range []string{"wrong-code", "wrong-phone", "wrong-target", "expired-code", "stale-target", "foreign-session", "permissions-failure", "number-claimed"} {
		t.Run(scenario, func(t *testing.T) {
			db, admin, identity, proof := setupManagedPhoneTest(t)
			target := createMiniAppPasswordUser(t, db, "phone-update-user", "password123")
			oldPhone := "+8613900139000"
			require.NoError(t, db.Model(target).Updates(map[string]any{"phone": oldPhone, "display_name": "Before"}).Error)
			require.NoError(t, db.Transaction(func(tx *gorm.DB) error { return model.ClaimVerifiedPhoneWithTx(tx, oldPhone, target.Id, "aliyun_sms") }))
			token, code := sendManagedPhoneTestCode(t, admin, identity, proof, target.Username, "13800138000", target.Id)
			body := map[string]any{"id": target.Id, "username": target.Username, "display_name": "After", "group": "default", "phone": "+8613800138000", "phone_challenge_token": token, "phone_verification_code": code}
			expectedVersion := int64(1)
			switch scenario {
			case "wrong-code":
				body["phone_verification_code"] = "000000"
				if code == "000000" {
					body["phone_verification_code"] = "000001"
				}
			case "wrong-phone":
				body["phone"] = "+8613700137000"
			case "wrong-target":
				other := createMiniAppPasswordUser(t, db, "phone-other-target", "password123")
				body["id"], body["username"] = other.Id, other.Username
			case "expired-code":
				require.NoError(t, db.Model(&model.SMSChallenge{}).Where("purpose = ?", "user_mutation").Update("expires_at", time.Now().Add(-time.Minute)).Error)
			case "stale-target":
				expectedVersion = 2
				require.NoError(t, db.Model(target).Update("auth_version", expectedVersion).Error)
			case "foreign-session":
				bundle, err := service.CreateLoginSession(admin.Id, "password", "test", "test")
				require.NoError(t, err)
				identity, err = service.ParseAccessToken(bundle.AccessToken)
				require.NoError(t, err)
				proof, _, err = service.IssueSecurityProof(identity, "password", []string{"phone.manage"})
				require.NoError(t, err)
			case "permissions-failure":
				body["admin_permissions"] = map[string]any{}
			case "number-claimed":
				other := createMiniAppPasswordUser(t, db, "phone-new-owner", "password123")
				require.NoError(t, db.Model(other).Update("phone", "+8613800138000").Error)
			}
			encoded, err := common.Marshal(body)
			require.NoError(t, err)
			recorder := callWeChatAccountHandler(t, admin, identity, proof, "", "/api/user/", string(encoded), UpdateUser)
			require.Contains(t, recorder.Body.String(), `"success":false`)
			if scenario == "wrong-code" {
				assert.Equal(t, http.StatusBadRequest, recorder.Code)
			}
			var unchanged model.User
			require.NoError(t, db.First(&unchanged, target.Id).Error)
			assert.Equal(t, oldPhone, unchanged.Phone)
			assert.Equal(t, "Before", unchanged.DisplayName)
			assert.Equal(t, expectedVersion, unchanged.AuthVersion)
			claim, err := model.GetVerifiedPhoneByUser(target.Id)
			require.NoError(t, err)
			assert.Equal(t, oldPhone, claim.Phone)
			var challenge model.SMSChallenge
			require.NoError(t, db.First(&challenge).Error)
			assert.Nil(t, challenge.ConsumedAt)
		})
	}
}

func TestUnchangedBoundPhoneNeedsNoSMSAndCannotBeCleared(t *testing.T) {
	db, admin, identity, _ := setupManagedPhoneTest(t)
	target := createMiniAppPasswordUser(t, db, "unchanged-phone", "password123")
	require.NoError(t, db.Model(target).Update("phone", "+8613800138000").Error)
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		return model.ClaimVerifiedPhoneWithTx(tx, "+8613800138000", target.Id, "aliyun_sms")
	}))
	t.Setenv("SMS_ENABLED", "false")
	for _, phone := range []string{"13800138000", "+86 13800138000"} {
		body := fmt.Sprintf(`{"id":%d,"username":%q,"display_name":"After","group":"default","phone":%q}`, target.Id, target.Username, phone)
		recorder := callWeChatAccountHandler(t, admin, identity, "", "", "/api/user/", body, UpdateUser)
		require.Contains(t, recorder.Body.String(), `"success":true`)
	}
	body := fmt.Sprintf(`{"id":%d,"username":%q,"display_name":"Cleared","group":"default","phone":""}`, target.Id, target.Username)
	recorder := callWeChatAccountHandler(t, admin, identity, "", "", "/api/user/", body, UpdateUser)
	assert.Contains(t, recorder.Body.String(), `"success":false`)
	var updated model.User
	require.NoError(t, db.First(&updated, target.Id).Error)
	assert.Equal(t, "+8613800138000", updated.Phone)
	assert.Equal(t, "After", updated.DisplayName)
	assert.Equal(t, int64(1), updated.AuthVersion)
}

func TestCreateWithoutPhoneRemainsAvailableWhenSMSIsDisabled(t *testing.T) {
	db, admin, identity, _ := setupManagedPhoneTest(t)
	t.Setenv("SMS_ENABLED", "false")
	recorder := callWeChatAccountHandler(t, admin, identity, "", "", "/api/user/", `{"username":"no-phone-user","password":"NewPassword123","role":1}`, CreateUser)
	require.Contains(t, recorder.Body.String(), `"success":true`)
	var user model.User
	require.NoError(t, db.Where("username = ?", "no-phone-user").First(&user).Error)
	assert.Empty(t, user.Phone)
	_, err := model.GetVerifiedPhoneByUser(user.Id)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestManagedPhoneCodeRejectsPeerAccountAndOccupiedNumber(t *testing.T) {
	for _, scenario := range []string{"peer-account", "occupied-number"} {
		t.Run(scenario, func(t *testing.T) {
			db, admin, identity, proof := setupManagedPhoneTest(t)
			target := createMiniAppPasswordUser(t, db, "code-target", "password123")
			userID := target.Id
			if scenario == "peer-account" {
				require.NoError(t, db.Model(target).Update("role", common.RoleAdminUser).Error)
			} else {
				require.NoError(t, db.Model(target).Update("phone", "+8613800138000").Error)
				userID = 0
			}
			previous := sendPhoneSMS
			t.Cleanup(func() { sendPhoneSMS = previous })
			sends := 0
			sendPhoneSMS = func(phone, code string) error { sends++; return nil }
			body := fmt.Sprintf(`{"user_id":%d,"username":"new-draft","phone":"13800138000"}`, userID)
			recorder := callWeChatAccountHandler(t, admin, identity, proof, "", "/api/user/phone/code", body, SendUserMutationPhoneCode)
			expectedStatus := http.StatusForbidden
			if scenario == "occupied-number" {
				expectedStatus = http.StatusConflict
			}
			assert.Equal(t, expectedStatus, recorder.Code)
			assert.Zero(t, sends)
		})
	}
}
