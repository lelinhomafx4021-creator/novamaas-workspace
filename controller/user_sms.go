package controller

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var sendPhoneSMS = service.SendAliyunSMS

func smsLoginAvailable(userId int) bool {
	if !system_setting.GetSMSSettings().IsReady() {
		return false
	}
	_, err := model.GetVerifiedPhoneByUser(userId)
	return err == nil
}

type phoneVerificationRequest struct {
	Phone          string `json:"phone"`
	WeChatCode     string `json:"wx_code"`
	ChallengeToken string `json:"challenge_token"`
	Code           string `json:"code"`
	FlowToken      string `json:"flow_token"`
}

func phoneError(c *gin.Context, err error) {
	status, code := http.StatusUnauthorized, "PHONE_VERIFICATION_INVALID"
	switch {
	case errors.Is(err, model.ErrSMSRateLimit):
		status, code = http.StatusTooManyRequests, "PHONE_RATE_LIMITED"
	case errors.Is(err, service.ErrSMSUnavailable):
		status, code = http.StatusServiceUnavailable, "PHONE_SMS_UNAVAILABLE"
	case errors.Is(err, model.ErrVerifiedPhoneProtected):
		status, code = http.StatusConflict, "PHONE_BINDING_PROTECTED"
	case errors.Is(err, model.ErrVerifiedPhoneInvalid):
		status, code = http.StatusBadRequest, "PHONE_NUMBER_INVALID"
	case errors.Is(err, model.ErrVerifiedPhoneAlreadyClaimed), errors.Is(err, model.ErrPhoneAlreadyTaken):
		status, code = http.StatusConflict, "PHONE_ALREADY_BOUND"
	case !errors.Is(err, model.ErrSMSInvalid), err == nil:
		if err != nil {
			common.SysError("phone operation failed: " + err.Error())
		}
		status, code = http.StatusInternalServerError, "PHONE_OPERATION_FAILED"
	}
	setAuthNoStore(c)
	c.JSON(status, gin.H{"success": false, "code": code, "message": code})
}

func phoneTarget(c *gin.Context) (*model.User, bool) {
	id := c.GetInt("id")
	if raw := c.Param("id"); raw != "" {
		var err error
		id, err = strconv.Atoi(raw)
		if err != nil || id <= 0 {
			phoneError(c, model.ErrSMSInvalid)
			return nil, false
		}
	}
	user, err := model.GetUserById(id, false)
	if err != nil {
		phoneError(c, err)
		return nil, false
	}
	if c.Param("id") != "" && !canManageTargetRole(c.GetInt("role"), user.Role) {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "Permission denied"})
		return nil, false
	}
	return user, true
}

func PhoneBindingStatus(c *gin.Context) {
	setAuthNoStore(c)
	user, ok := phoneTarget(c)
	if !ok {
		return
	}
	claim, err := model.GetVerifiedPhoneByUser(user.Id)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		phoneError(c, err)
		return
	}
	setAuthNoStore(c)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"phone": user.Phone, "verified": claim, "sms_enabled": system_setting.GetSMSSettings().IsReady()}})
}

// PhoneSecurityCode proves possession of the current binding, scoped to the
// authenticated session; clients cannot select a different destination.
func PhoneSecurityCode(c *gin.Context) {
	identity, ok := middleware.GetSessionAuthIdentity(c)
	if !ok {
		phoneError(c, model.ErrSMSInvalid)
		return
	}
	claim, err := model.GetVerifiedPhoneByUser(identity.UserID)
	if err != nil {
		phoneError(c, model.ErrSMSInvalid)
		return
	}
	token, err := service.IssueSMS(model.SMSChallengeInput{Purpose: "security", Phone: claim.Phone, UserId: identity.UserID,
		AuthVersion: identity.UserAuthVersion, SessionId: identity.SessionID, IP: c.ClientIP()}, sendPhoneSMS)
	if err != nil {
		phoneError(c, err)
		return
	}
	writePhoneChallenge(c, token)
}

type phoneSecurityRequest struct {
	Password       string `json:"password"`
	TwoFactorCode  string `json:"two_factor_code"`
	ChallengeToken string `json:"challenge_token"`
	Code           string `json:"code"`
	Scope          string `json:"scope"`
}

func PhoneSecurityVerify(c *gin.Context) {
	setAuthNoStore(c)
	identity, ok := middleware.GetSessionAuthIdentity(c)
	if !ok {
		phoneError(c, model.ErrSMSInvalid)
		return
	}
	var req phoneSecurityRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil || (req.Scope != "phone.manage" && req.Scope != "wechat.manage") {
		phoneError(c, model.ErrSMSInvalid)
		return
	}
	method := "password"
	if req.ChallengeToken != "" {
		if !system_setting.GetSMSSettings().IsReady() {
			phoneError(c, service.ErrSMSUnavailable)
			return
		}
		// An old-phone proof may authorize a phone change or personal WeChat
		// unlink; it does not grant unrelated privileged security scopes.
		_, err := model.ConsumeSMSChallenge(req.ChallengeToken, req.Code, model.SMSChallengeInput{Purpose: "security", UserId: identity.UserID, SessionId: identity.SessionID}, func(tx *gorm.DB, challenge *model.SMSChallenge) error {
			return validatePhoneChallengeUser(tx, challenge, true)
		})
		if err != nil {
			phoneError(c, err)
			return
		}
		method = "sms"
	} else {
		user, err := model.GetUserById(identity.UserID, true)
		if err != nil || req.Password == "" || user.AuthVersion != identity.UserAuthVersion || !common.ValidatePasswordAndHash(req.Password, user.Password) {
			phoneError(c, model.ErrSMSInvalid)
			return
		}
		enabled, err := model.IsTwoFAEnabled(user.Id)
		if err != nil {
			phoneError(c, err)
			return
		}
		if enabled && !validateMiniAppTwoFactor(user, req.TwoFactorCode) {
			phoneError(c, model.ErrSMSInvalid)
			return
		}
	}
	token, expiresAt, err := service.IssueSecurityProof(identity, method, []string{req.Scope})
	if err != nil {
		phoneError(c, err)
		return
	}
	setAuthNoStore(c)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"proof_token": token, "expires_at": expiresAt}})
}

func validatePhoneChallengeUser(tx *gorm.DB, challenge *model.SMSChallenge, requireBinding bool) error {
	var user model.User
	if err := tx.Where("id = ? AND status = ? AND auth_version = ?", challenge.UserId, common.UserStatusEnabled, challenge.AuthVersion).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.ErrSMSInvalid
		}
		return err
	}
	if !requireBinding {
		return nil
	}
	var claim model.VerifiedPhone
	if err := tx.Where("user_id = ? AND phone = ?", user.Id, challenge.Phone).First(&claim).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.ErrSMSInvalid
		}
		return err
	}
	return nil
}

func SendPhoneBindingCode(c *gin.Context) {
	methods := []string{"password", "2fa", "passkey"}
	if c.Param("id") == "" {
		methods = append(methods, "sms")
	}
	if !middleware.RequireSecurityProof(c, "phone.manage", methods) {
		return
	}
	user, ok := phoneTarget(c)
	if !ok {
		return
	}
	identity, ok := middleware.GetSessionAuthIdentity(c)
	if !ok {
		phoneError(c, model.ErrSMSInvalid)
		return
	}
	var req phoneVerificationRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		phoneError(c, model.ErrSMSInvalid)
		return
	}
	phone, err := model.CanonicalMobilePhone(req.Phone)
	if err != nil {
		phoneError(c, err)
		return
	}
	token, err := service.IssueSMS(model.SMSChallengeInput{Purpose: "phone_bind", Phone: phone, UserId: user.Id,
		AuthVersion: user.AuthVersion, SessionId: identity.SessionID, IP: c.ClientIP()}, sendPhoneSMS)
	if err != nil {
		phoneError(c, err)
		return
	}
	writePhoneChallenge(c, token)
}

func ConfirmPhoneBinding(c *gin.Context) {
	setAuthNoStore(c)
	if !system_setting.GetSMSSettings().IsReady() {
		phoneError(c, service.ErrSMSUnavailable)
		return
	}
	methods := []string{"password", "2fa", "passkey"}
	if c.Param("id") == "" {
		methods = append(methods, "sms")
	}
	if !middleware.RequireSecurityProof(c, "phone.manage", methods) {
		return
	}
	user, ok := phoneTarget(c)
	if !ok {
		return
	}
	identity, ok := middleware.GetSessionAuthIdentity(c)
	if !ok {
		phoneError(c, model.ErrSMSInvalid)
		return
	}
	var req phoneVerificationRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		phoneError(c, model.ErrSMSInvalid)
		return
	}
	_, err := model.ConsumeSMSChallenge(req.ChallengeToken, req.Code, model.SMSChallengeInput{Purpose: "phone_bind", UserId: user.Id, SessionId: identity.SessionID}, func(tx *gorm.DB, challenge *model.SMSChallenge) error {
		if err := validatePhoneChallengeUser(tx, challenge, false); err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", user.Id).Delete(&model.VerifiedPhone{}).Error; err != nil {
			return err
		}
		if err := model.ClaimVerifiedPhoneWithTx(tx, challenge.Phone, user.Id, "aliyun_sms"); err != nil {
			return err
		}
		if _, err := model.IncrementUserAuthVersionWithTx(tx, user.Id); err != nil {
			return err
		}
		return tx.Model(&model.User{}).Where("id = ?", user.Id).Update("phone", challenge.Phone).Error
	})
	if err != nil {
		phoneError(c, err)
		return
	}
	if err := model.PublishUserAuthCache(user.Id); err != nil {
		phoneError(c, err)
		return
	}
	model.RecordLog(user.Id, model.LogTypeSystem, "Phone binding verified via SMS")
	if c.Param("id") != "" {
		if _, err := model.RevokeAllUserSessions(user.Id, "admin_phone_changed"); err != nil {
			phoneError(c, err)
			return
		}
		recordManageAuditFor(c, user.Id, "user.phone_bind", nil)
		c.JSON(http.StatusOK, gin.H{"success": true})
		return
	}
	bundle, err := service.AdvanceCurrentSessionToUserVersion(identity, "phone_changed")
	if err != nil {
		phoneError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": authRotationData(bundle)})
}

func writePhoneChallenge(c *gin.Context, token string) {
	setAuthNoStore(c)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"challenge_token": token, "expires_in": int(model.SMSChallengeTTL.Seconds()), "retry_after": 60}})
}

func SendSMSLoginCode(c *gin.Context) {
	if !system_setting.GetSMSSettings().IsReady() {
		phoneError(c, service.ErrSMSUnavailable)
		return
	}
	var req phoneVerificationRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		phoneError(c, model.ErrSMSInvalid)
		return
	}
	nativeSubject := ""
	if strings.HasPrefix(c.Request.URL.Path, "/api/mini/") {
		if strings.TrimSpace(req.WeChatCode) == "" {
			writeMiniAppAuthError(c, http.StatusBadRequest, "MINI_AUTH_CODE_REQUIRED")
			return
		}
		identity, err := exchangeMiniAppCode(c.Request.Context(), req.WeChatCode)
		if err != nil {
			writeMiniAppCodeExchangeError(c, err)
			return
		}
		if err := model.ClaimExternalAuthAssertion("mini_sms_send", identity.AppId+":"+req.WeChatCode, time.Now().Add(10*time.Minute)); err != nil {
			phoneError(c, model.ErrSMSInvalid)
			return
		}
		nativeSubject = identity.AppId + ":" + identity.OpenId
	}
	phone, err := model.CanonicalMobilePhone(req.Phone)
	if err != nil {
		phoneError(c, err)
		return
	}
	input := model.SMSChallengeInput{Purpose: "login", Phone: phone, IP: c.ClientIP(), NativeSubject: nativeSubject}
	id, err := model.GetUserIdByVerifiedPhone(phone)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		phoneError(c, err)
		return
	}
	if id > 0 {
		user, err := model.GetUserById(id, false)
		if err != nil {
			phoneError(c, err)
			return
		}
		input.UserId, input.AuthVersion = id, user.AuthVersion
	}
	// The same challenge response and send behavior for unknown numbers
	// prevents account enumeration. Verification still requires a binding.
	token, err := service.IssueSMS(input, sendPhoneSMS)
	if err != nil {
		phoneError(c, err)
		return
	}
	writePhoneChallenge(c, token)
}

func SMSLogin(c *gin.Context) {
	if !system_setting.GetSMSSettings().IsReady() {
		phoneError(c, service.ErrSMSUnavailable)
		return
	}
	var req phoneVerificationRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		phoneError(c, model.ErrSMSInvalid)
		return
	}
	challenge, err := model.GetSMSChallenge(req.ChallengeToken, "login")
	if err != nil {
		phoneError(c, err)
		return
	}
	_, err = model.ConsumeSMSChallenge(req.ChallengeToken, req.Code, model.SMSChallengeInput{Purpose: "login", UserId: challenge.UserId}, func(tx *gorm.DB, verified *model.SMSChallenge) error {
		return validatePhoneChallengeUser(tx, verified, true)
	})
	if err != nil {
		phoneError(c, err)
		return
	}
	user, err := model.GetUserById(challenge.UserId, false)
	if err != nil {
		phoneError(c, err)
		return
	}
	if strings.HasPrefix(c.Request.URL.Path, "/api/mini/") {
		bundle, err := service.CreateLoginSessionAtAuthVersion(user.Id, challenge.AuthVersion, "sms_miniapp", c.ClientIP(), c.Request.UserAgent())
		if err != nil {
			writeAuthSessionError(c, err)
			return
		}
		writeMiniAppAuthBundle(c, user, bundle)
		return
	}
	setupLoginAtAuthVersion(user, challenge.AuthVersion, c)
}

func SendSMSMFACode(c *gin.Context) {
	var req phoneVerificationRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		phoneError(c, model.ErrSMSInvalid)
		return
	}
	flow, err := model.GetAuthFlow(req.FlowToken, model.AuthFlowMatch{Purpose: model.AuthFlowPurposeTwoFALogin})
	if err != nil || (strings.HasPrefix(c.Request.URL.Path, "/api/mini/") != (flow.Provider == "miniapp")) {
		phoneError(c, model.ErrSMSInvalid)
		return
	}
	var payload twoFALoginFlowPayload
	if err := common.UnmarshalJsonStr(flow.Payload, &payload); err != nil {
		phoneError(c, model.ErrSMSInvalid)
		return
	}
	user, err := model.GetUserById(flow.UserId, false)
	if err != nil || user.Status != common.UserStatusEnabled || user.AuthVersion != payload.AuthVersion {
		phoneError(c, model.ErrSMSInvalid)
		return
	}
	claim, err := model.GetVerifiedPhoneByUser(user.Id)
	if err != nil {
		phoneError(c, model.ErrSMSInvalid)
		return
	}
	token, err := service.IssueSMS(model.SMSChallengeInput{Purpose: "mfa_login", Phone: claim.Phone, UserId: user.Id,
		AuthVersion: user.AuthVersion, FlowToken: req.FlowToken, IP: c.ClientIP()}, sendPhoneSMS)
	if err != nil {
		phoneError(c, err)
		return
	}
	writePhoneChallenge(c, token)
}

func SMSMFALogin(c *gin.Context) {
	if !system_setting.GetSMSSettings().IsReady() {
		phoneError(c, service.ErrSMSUnavailable)
		return
	}
	var req phoneVerificationRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		phoneError(c, model.ErrSMSInvalid)
		return
	}
	flow, err := model.GetAuthFlow(req.FlowToken, model.AuthFlowMatch{Purpose: model.AuthFlowPurposeTwoFALogin})
	if err != nil || (strings.HasPrefix(c.Request.URL.Path, "/api/mini/") != (flow.Provider == "miniapp")) {
		phoneError(c, model.ErrSMSInvalid)
		return
	}
	var payload twoFALoginFlowPayload
	if err := common.UnmarshalJsonStr(flow.Payload, &payload); err != nil {
		phoneError(c, model.ErrSMSInvalid)
		return
	}
	_, err = model.ConsumeSMSChallenge(req.ChallengeToken, req.Code, model.SMSChallengeInput{Purpose: "mfa_login", UserId: flow.UserId, FlowToken: req.FlowToken}, func(tx *gorm.DB, challenge *model.SMSChallenge) error {
		if challenge.AuthVersion != payload.AuthVersion {
			return model.ErrSMSInvalid
		}
		if err := validatePhoneChallengeUser(tx, challenge, true); err != nil {
			return err
		}
		result := tx.Model(&model.AuthFlow{}).Where("id = ? AND consumed_at IS NULL AND expires_at > ?", flow.Id, time.Now()).Update("consumed_at", time.Now())
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return model.ErrSMSInvalid
		}
		return nil
	})
	if err != nil {
		phoneError(c, err)
		return
	}
	user, err := model.GetUserById(flow.UserId, false)
	if err != nil {
		phoneError(c, err)
		return
	}
	if strings.HasPrefix(c.Request.URL.Path, "/api/mini/") {
		bundle, err := service.CreateLoginSessionAtAuthVersion(user.Id, payload.AuthVersion, "password_sms_miniapp", c.ClientIP(), c.Request.UserAgent())
		if err != nil {
			writeAuthSessionError(c, err)
			return
		}
		writeMiniAppAuthBundle(c, user, bundle)
		return
	}
	setupLoginAtAuthVersion(user, payload.AuthVersion, c)
}
