package controller

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const miniAppBindingFlowTTL = 10 * time.Minute

var exchangeMiniAppCode = service.ExchangeWeChatMiniAppCode
var exchangeMiniAppPhoneCode = service.ExchangeWeChatMiniAppPhoneCode

type miniAppLoginRequest struct {
	Code string `json:"code"`
}

type miniAppPasswordLoginRequest struct {
	Username      string `json:"username"`
	Password      string `json:"password"`
	TwoFactorCode string `json:"two_factor_code,omitempty"`
}

type miniAppPhoneLoginRequest struct {
	AppId     string `json:"app_id,omitempty"`
	Code      string `json:"code"`
	PhoneCode string `json:"phone_code"`
}

type miniAppBindingFlowPayload struct {
	AppId                 string `json:"app_id"`
	OpenId                string `json:"open_id"`
	UnionId               string `json:"union_id,omitempty"`
	Phone                 string `json:"phone,omitempty"`
	RequiredUserId        int    `json:"required_user_id,omitempty"`
	AuthVersion           int64  `json:"auth_version,omitempty"`
	AccountHint           string `json:"account_hint,omitempty"`
	AccountUsername       string `json:"account_username,omitempty"`
	ConfirmationAvailable bool   `json:"confirmation_available,omitempty"`
}

type miniAppBindRequest struct {
	FlowToken     string  `json:"flow_token"`
	Username      string  `json:"username"`
	Password      string  `json:"password"`
	TwoFactorCode string  `json:"two_factor_code,omitempty"`
	AcceptTerms   bool    `json:"accept_terms"`
	ConfirmOnly   bool    `json:"confirm_only"`
	Nickname      *string `json:"nickname,omitempty"`
	AvatarBase64  *string `json:"avatar_base64,omitempty"`
}

type miniAppEmailVerificationRequest struct {
	FlowToken string `json:"flow_token"`
	Email     string `json:"email"`
}

type miniAppRefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
	SID          string `json:"sid"`
}

func MiniAppLogin(c *gin.Context) {
	setAuthNoStore(c)
	var request miniAppLoginRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil || strings.TrimSpace(request.Code) == "" {
		writeMiniAppAuthError(c, http.StatusBadRequest, "MINI_AUTH_CODE_REQUIRED")
		return
	}

	identity, err := exchangeMiniAppCode(c.Request.Context(), request.Code)
	if err != nil {
		writeMiniAppCodeExchangeError(c, err)
		return
	}
	user, err := model.GetWeChatLoginUser(identity.AppId, identity.OpenId, "")
	if errors.Is(err, gorm.ErrRecordNotFound) {
		writeMiniAppBindingRequired(c, miniAppBindingFlowPayload{AppId: identity.AppId, OpenId: identity.OpenId, UnionId: identity.UnionId})
		return
	}
	if err != nil {
		writeMiniAppAuthError(c, http.StatusUnauthorized, "MINI_AUTH_ACCOUNT_UNAVAILABLE")
		return
	}
	finishWeChatMiniAppLogin(c, user, identity.AppId, identity.OpenId, identity.UnionId)
}

// MiniAppPasswordLogin is independent of WeChat authorization. Existing users
// can still sign in when the phone-number permission or wx.login is unavailable.
func MiniAppPasswordLogin(c *gin.Context) {
	setAuthNoStore(c)
	if !common.PasswordLoginEnabled {
		writeMiniAppAuthError(c, http.StatusForbidden, "MINI_AUTH_PASSWORD_LOGIN_DISABLED")
		return
	}
	var request miniAppPasswordLoginRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil ||
		strings.TrimSpace(request.Username) == "" || request.Password == "" {
		writeMiniAppAuthError(c, http.StatusBadRequest, "MINI_AUTH_INVALID_REQUEST")
		return
	}
	user, ok := verifyMiniAppAccountCredentials(c, request.Username, request.Password, request.TwoFactorCode)
	if !ok {
		return
	}
	bundle, err := service.CreateLoginSessionAtAuthVersion(user.Id, user.AuthVersion, "password_miniapp", c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		writeAuthSessionError(c, err)
		return
	}
	writeMiniAppAuthBundle(c, user, bundle)
}

func writeMiniAppBindingRequired(c *gin.Context, binding miniAppBindingFlowPayload) {
	payload, err := common.Marshal(binding)
	if err != nil {
		writeMiniAppInternalError(c, err)
		return
	}
	flowToken, flow, err := model.CreateAuthFlow(model.AuthFlowCreate{
		Purpose:   model.AuthFlowPurposeWeChatMiniAppBind,
		Provider:  model.ExternalIdentityProviderWeChatMiniApp,
		Intent:    model.AuthFlowIntentBind,
		Payload:   string(payload),
		ExpiresAt: time.Now().Add(miniAppBindingFlowTTL),
	})
	if err != nil {
		writeMiniAppInternalError(c, err)
		return
	}
	legalSettings := system_setting.GetLegalSettings()
	phoneHint := ""
	if len(binding.Phone) == 14 && strings.HasPrefix(binding.Phone, "+86") {
		phoneHint = "+86 " + binding.Phone[3:6] + "****" + binding.Phone[10:]
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"binding_required":       true,
			"confirmation_available": binding.ConfirmationAvailable,
			"account_hint":           binding.AccountHint,
			"matched_account": gin.H{
				"username":     binding.AccountUsername,
				"display_name": binding.AccountHint,
				"phone_hint":   phoneHint,
			},
			"two_factor_required":        false,
			"flow_token":                 flowToken,
			"flow_expires_at":            flow.ExpiresAt.Unix(),
			"password_login_enabled":     common.PasswordLoginEnabled,
			"registration_enabled":       false,
			"email_verification_enabled": common.EmailVerificationEnabled,
			"user_agreement_enabled":     legalSettings.UserAgreement != "",
			"privacy_policy_enabled":     legalSettings.PrivacyPolicy != "",
		},
	})
}

func MiniAppPhoneLogin(c *gin.Context) {
	setAuthNoStore(c)
	var request miniAppPhoneLoginRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil ||
		strings.TrimSpace(request.Code) == "" || strings.TrimSpace(request.PhoneCode) == "" {
		writeMiniAppAuthError(c, http.StatusBadRequest, "MINI_AUTH_CODE_REQUIRED")
		return
	}
	// AppID is public client metadata, never a selector for server credentials.
	// Older clients may omit it; updated clients fail before consuming either code.
	if appId := strings.TrimSpace(request.AppId); appId != "" {
		settings := system_setting.GetWeChatMiniAppSettings().Resolve()
		if !settings.Enabled {
			writeMiniAppAuthError(c, http.StatusServiceUnavailable, "MINI_AUTH_DISABLED")
			return
		}
		if !settings.IsReady() {
			writeMiniAppAuthError(c, http.StatusServiceUnavailable, "MINI_AUTH_NOT_CONFIGURED")
			return
		}
		if appId != strings.TrimSpace(settings.AppId) {
			writeMiniAppAuthError(c, http.StatusConflict, "MINI_AUTH_APP_ID_MISMATCH")
			return
		}
	}
	identity, err := exchangeMiniAppCode(c.Request.Context(), request.Code)
	if err != nil {
		writeMiniAppCodeExchangeError(c, err)
		return
	}
	phone, err := exchangeMiniAppPhoneCode(c.Request.Context(), request.PhoneCode)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrWeChatMiniAppPhoneRejected):
			writeMiniAppAuthError(c, http.StatusUnauthorized, "MINI_AUTH_PHONE_CODE_INVALID")
		case errors.Is(err, service.ErrWeChatMiniAppDisabled), errors.Is(err, service.ErrWeChatMiniAppConfiguration):
			writeMiniAppAuthError(c, http.StatusServiceUnavailable, "MINI_AUTH_NOT_CONFIGURED")
		default:
			writeMiniAppAuthError(c, http.StatusBadGateway, "MINI_AUTH_UPSTREAM_UNAVAILABLE")
		}
		return
	}
	canonicalPhone, err := model.CanonicalMobilePhone(phone)
	if err != nil {
		writeMiniAppBindingError(c, err)
		return
	}
	user, err := model.GetWeChatLoginUser(identity.AppId, identity.OpenId, canonicalPhone)
	if err == nil {
		finishWeChatMiniAppLogin(c, user, identity.AppId, identity.OpenId, identity.UnionId)
		return
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		writeMiniAppBindingError(c, err)
		return
	}
	binding := miniAppBindingFlowPayload{AppId: identity.AppId, OpenId: identity.OpenId, UnionId: identity.UnionId, Phone: canonicalPhone}
	ownerId, err := model.GetUserIdByVerifiedPhone(canonicalPhone)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		writeMiniAppAuthError(c, http.StatusNotFound, "MINI_AUTH_PHONE_ACCOUNT_NOT_FOUND")
		return
	}
	if err != nil {
		writeMiniAppInternalError(c, err)
		return
	}
	owner, lookupErr := model.GetUserById(ownerId, false)
	if lookupErr != nil || owner.Status != common.UserStatusEnabled {
		writeMiniAppAuthError(c, http.StatusUnauthorized, "MINI_AUTH_ACCOUNT_UNAVAILABLE")
		return
	}
	binding.RequiredUserId, binding.AuthVersion = owner.Id, owner.AuthVersion
	binding.AccountUsername = owner.Username
	binding.AccountHint = owner.DisplayName
	if binding.AccountHint == "" {
		binding.AccountHint = owner.Username
	}
	// The verified WeChat phone is sufficient for explicit confirmation for all
	// roles, including MFA accounts. Ownership is rechecked when consuming the flow.
	binding.ConfirmationAvailable = true
	writeMiniAppBindingRequired(c, binding)
}

func MiniAppBind(c *gin.Context) {
	setAuthNoStore(c)
	var request miniAppBindRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		writeMiniAppAuthError(c, http.StatusBadRequest, "MINI_AUTH_INVALID_REQUEST")
		return
	}
	if !miniAppTermsAccepted(request.AcceptTerms) {
		writeMiniAppAuthError(c, http.StatusBadRequest, "MINI_AUTH_TERMS_REQUIRED")
		return
	}
	flow, payload, ok := readMiniAppBindingFlow(c, request.FlowToken)
	if !ok {
		return
	}
	// Old phone flows without an already verified platform owner must not be
	// usable to enroll a phone or connect an arbitrary account after this change.
	if payload.Phone != "" && payload.RequiredUserId <= 0 {
		writeMiniAppAuthError(c, http.StatusNotFound, "MINI_AUTH_PHONE_ACCOUNT_NOT_FOUND")
		return
	}
	if request.Nickname != nil {
		*request.Nickname = strings.TrimSpace(*request.Nickname)
		if !utf8.ValidString(*request.Nickname) || utf8.RuneCountInString(*request.Nickname) > 64 || strings.ContainsAny(*request.Nickname, "\r\n\x00") {
			writeMiniAppAuthError(c, http.StatusBadRequest, "MINI_AUTH_PROFILE_INVALID")
			return
		}
	}
	var avatar []byte
	if request.AvatarBase64 != nil && *request.AvatarBase64 != "" {
		var err error
		avatar, err = normalizeWeChatAvatar(*request.AvatarBase64)
		if err != nil {
			writeMiniAppAuthError(c, http.StatusBadRequest, "MINI_AUTH_PROFILE_INVALID")
			return
		}
	}

	var user *model.User
	if payload.Phone != "" {
		var err error
		user, err = model.GetUserById(payload.RequiredUserId, false)
		if err != nil || user.Status != common.UserStatusEnabled || user.AuthVersion != payload.AuthVersion {
			writeMiniAppBindingError(c, model.ErrAuthFlowInvalid)
			return
		}
	} else {
		// A code-only legacy flow has no phone proof and still needs credentials.
		if request.ConfirmOnly {
			writeMiniAppBindingError(c, model.ErrAuthFlowInvalid)
			return
		}
		if !common.PasswordLoginEnabled {
			writeMiniAppAuthError(c, http.StatusForbidden, "MINI_AUTH_PASSWORD_LOGIN_DISABLED")
			return
		}
		var valid bool
		user, valid = verifyMiniAppAccountCredentials(c, request.Username, request.Password, request.TwoFactorCode)
		if !valid {
			return
		}
	}
	if payload.RequiredUserId != 0 && (user.Id != payload.RequiredUserId || user.AuthVersion != payload.AuthVersion) {
		writeMiniAppAuthError(c, http.StatusConflict, "MINI_AUTH_PHONE_CONFLICT")
		return
	}
	var nextVersion int64

	_, err := model.ConsumeAuthFlowWithAction(request.FlowToken, model.AuthFlowMatch{
		Purpose:  model.AuthFlowPurposeWeChatMiniAppBind,
		Provider: model.ExternalIdentityProviderWeChatMiniApp,
		Intent:   model.AuthFlowIntentBind,
	}, func(tx *gorm.DB, consumed *model.AuthFlow) error {
		if consumed.Id != flow.Id {
			return model.ErrAuthFlowInvalid
		}
		if err := model.ValidateUserAuthVersionWithTx(tx, user.Id, user.AuthVersion); err != nil {
			return model.ErrAuthFlowInvalid
		}
		if payload.RequiredUserId > 0 {
			var claim model.VerifiedPhone
			if err := tx.Where("user_id = ? AND phone = ?", user.Id, payload.Phone).First(&claim).Error; err != nil {
				return model.ErrAuthFlowInvalid
			}
		}
		if payload.Phone != "" {
			if err := model.ClaimVerifiedPhoneWithTx(tx, payload.Phone, user.Id, "wechat_miniapp"); err != nil {
				return err
			}
		}
		if err := model.ClaimScopedExternalIdentityWithTx(tx, model.ExternalIdentityProviderWeChatMiniApp, payload.AppId, payload.OpenId, user.Id); err != nil {
			return err
		}
		if err := model.RecordWeChatBindingWithTx(tx, user.Id, payload.AppId, payload.UnionId, payload.Phone); err != nil {
			return err
		}
		if request.Nickname != nil || request.AvatarBase64 != nil {
			updates := map[string]interface{}{"profile_source": "user_selected", "updated_at": time.Now()}
			if request.Nickname != nil {
				updates["nickname"] = *request.Nickname
			}
			if request.AvatarBase64 != nil {
				updates["avatar"] = avatar
			}
			if err := tx.Model(&model.WeChatMiniAppProfile{}).Where("user_id = ? AND app_id = ?", user.Id, payload.AppId).Updates(updates).Error; err != nil {
				return err
			}
		}
		var err error
		nextVersion, err = model.IncrementUserAuthVersionWithTx(tx, user.Id)
		return err
	})
	if err != nil {
		writeMiniAppBindingError(c, err)
		return
	}
	if err := model.PublishUserAuthCache(user.Id); err != nil {
		writeMiniAppInternalError(c, err)
		return
	}
	if _, err := model.RevokeAllUserSessions(user.Id, "wechat_bound"); err != nil {
		writeMiniAppInternalError(c, err)
		return
	}
	user.AuthVersion = nextVersion
	model.RecordLog(user.Id, model.LogTypeSystem, "WeChat mini program binding confirmed")
	finishWeChatMiniAppLogin(c, user, payload.AppId, payload.OpenId, payload.UnionId)
}

func verifyMiniAppAccountCredentials(c *gin.Context, username, password, twoFactorCode string) (*model.User, bool) {
	user := &model.User{Username: strings.TrimSpace(username), Password: password}
	if err := user.ValidateAndFill(); err != nil {
		if errors.Is(err, model.ErrDatabase) {
			writeMiniAppInternalError(c, err)
		} else {
			writeMiniAppAuthError(c, http.StatusUnauthorized, "MINI_AUTH_CREDENTIALS_INVALID")
		}
		return nil, false
	}
	twoFAEnabled, err := model.IsTwoFAEnabled(user.Id)
	if err != nil {
		writeMiniAppInternalError(c, err)
		return nil, false
	}
	if twoFAEnabled {
		if strings.TrimSpace(twoFactorCode) == "" && c.Request.URL.Path == "/api/mini/auth/password" {
			payload, err := common.Marshal(twoFALoginFlowPayload{AuthVersion: user.AuthVersion})
			if err != nil {
				writeMiniAppInternalError(c, err)
				return nil, false
			}
			token, _, err := model.CreateAuthFlow(model.AuthFlowCreate{Purpose: model.AuthFlowPurposeTwoFALogin, Provider: "miniapp", UserId: user.Id, Payload: string(payload), ExpiresAt: time.Now().Add(5 * time.Minute)})
			if err != nil {
				writeMiniAppInternalError(c, err)
				return nil, false
			}
			c.JSON(http.StatusForbidden, gin.H{"success": false, "code": "MINI_AUTH_2FA_REQUIRED", "data": gin.H{"flow_token": token, "sms_available": smsLoginAvailable(user.Id)}})
			return nil, false
		}
		if strings.TrimSpace(twoFactorCode) == "" {
			writeMiniAppAuthError(c, http.StatusForbidden, "MINI_AUTH_2FA_REQUIRED")
			return nil, false
		}
		if !validateMiniAppTwoFactor(user, twoFactorCode) {
			writeMiniAppAuthError(c, http.StatusUnauthorized, "MINI_AUTH_2FA_INVALID")
			return nil, false
		}
	}
	return user, true
}

func MiniAppRegister(c *gin.Context) {
	setAuthNoStore(c)
	// Registration and phone enrollment take place on the platform. Keep the
	// endpoint explicit for older clients instead of accepting cached phone flows.
	writeMiniAppAuthError(c, http.StatusForbidden, "MINI_AUTH_PHONE_ACCOUNT_NOT_FOUND")
}

func finishWeChatMiniAppLogin(c *gin.Context, user *model.User, appId, openId, unionId string) {
	if err := model.RecordWeChatLogin(user.Id, user.AuthVersion, appId, openId, unionId); err != nil {
		writeMiniAppBindingError(c, err)
		return
	}
	bundle, err := service.CreateLoginSessionAtAuthVersion(user.Id, user.AuthVersion, "wechat_miniapp", c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		writeAuthSessionError(c, err)
		return
	}
	writeMiniAppAuthBundle(c, user, bundle)
}

func MiniAppSendEmailVerification(c *gin.Context) {
	setAuthNoStore(c)
	if !common.EmailVerificationEnabled {
		writeMiniAppAuthError(c, http.StatusForbidden, "MINI_AUTH_EMAIL_VERIFICATION_DISABLED")
		return
	}
	var request miniAppEmailVerificationRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		writeMiniAppAuthError(c, http.StatusBadRequest, "MINI_AUTH_INVALID_REQUEST")
		return
	}
	if _, _, ok := readMiniAppBindingFlow(c, request.FlowToken); !ok {
		return
	}
	sendEmailVerification(c, request.Email)
}

func MiniAppRefresh(c *gin.Context) {
	setAuthNoStore(c)
	var request miniAppRefreshRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil || strings.TrimSpace(request.SID) == "" {
		writeMiniAppAuthError(c, http.StatusBadRequest, "MINI_AUTH_REFRESH_INVALID")
		return
	}
	bundle, user, err := service.RefreshLoginSession(request.RefreshToken, request.SID, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		writeAuthSessionError(c, err)
		return
	}
	writeMiniAppAuthBundle(c, user, bundle)
}

func MiniAppLogout(c *gin.Context) {
	setAuthNoStore(c)
	var request miniAppRefreshRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil || strings.TrimSpace(request.SID) == "" {
		writeMiniAppAuthError(c, http.StatusBadRequest, "MINI_AUTH_REFRESH_INVALID")
		return
	}
	if err := service.RevokeByRefreshToken(request.RefreshToken, request.SID, "miniapp_logout"); err != nil {
		writeAuthSessionError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": gin.H{"revoked_sid": request.SID}})
}

func readMiniAppBindingFlow(c *gin.Context, token string) (*model.AuthFlow, miniAppBindingFlowPayload, bool) {
	flow, err := model.GetAuthFlow(strings.TrimSpace(token), model.AuthFlowMatch{
		Purpose:  model.AuthFlowPurposeWeChatMiniAppBind,
		Provider: model.ExternalIdentityProviderWeChatMiniApp,
		Intent:   model.AuthFlowIntentBind,
	})
	if err != nil {
		writeMiniAppBindingError(c, err)
		return nil, miniAppBindingFlowPayload{}, false
	}
	var payload miniAppBindingFlowPayload
	if err := common.UnmarshalJsonStr(flow.Payload, &payload); err != nil || strings.TrimSpace(payload.AppId) == "" || strings.TrimSpace(payload.OpenId) == "" {
		writeMiniAppBindingError(c, model.ErrAuthFlowInvalid)
		return nil, miniAppBindingFlowPayload{}, false
	}
	return flow, payload, true
}

func validateMiniAppTwoFactor(user *model.User, code string) bool {
	twoFA, err := model.GetTwoFAByUserId(user.Id)
	if err != nil || twoFA == nil || !twoFA.IsEnabled {
		return false
	}
	if cleanCode, codeErr := common.ValidateNumericCode(code); codeErr == nil {
		valid, _ := twoFA.ValidateTOTPAndUpdateUsage(cleanCode)
		if valid {
			return true
		}
	}
	valid, err := twoFA.ValidateBackupCodeAndUpdateUsage(code)
	return err == nil && valid
}

func miniAppTermsAccepted(accepted bool) bool {
	legalSettings := system_setting.GetLegalSettings()
	if legalSettings.UserAgreement == "" && legalSettings.PrivacyPolicy == "" {
		return true
	}
	return accepted
}

func writeMiniAppAuthBundle(c *gin.Context, user *model.User, bundle *service.AuthBundle) {
	model.UpdateUserLastLoginAt(user.Id)
	recordLoginAudit(user, c)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"binding_required":  false,
			"access_token":      bundle.AccessToken,
			"token_type":        bundle.TokenType,
			"access_expires_at": bundle.AccessExpiresAt,
			"refresh_token":     bundle.RefreshToken,
			"session":           bundle.Session,
			"user":              buildSelfUserData(user),
		},
	})
}

func writeMiniAppCodeExchangeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrWeChatMiniAppDisabled):
		writeMiniAppAuthError(c, http.StatusServiceUnavailable, "MINI_AUTH_DISABLED")
	case errors.Is(err, service.ErrWeChatMiniAppConfiguration):
		writeMiniAppAuthError(c, http.StatusServiceUnavailable, "MINI_AUTH_NOT_CONFIGURED")
	case errors.Is(err, service.ErrWeChatMiniAppCodeRejected), errors.Is(err, service.ErrWeChatMiniAppEmptyOpenId):
		writeMiniAppAuthError(c, http.StatusUnauthorized, "MINI_AUTH_CODE_INVALID")
	default:
		writeMiniAppAuthError(c, http.StatusBadGateway, "MINI_AUTH_UPSTREAM_UNAVAILABLE")
	}
}

func writeMiniAppBindingError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, model.ErrWeChatAccountUnavailable):
		writeMiniAppAuthError(c, http.StatusUnauthorized, "MINI_AUTH_ACCOUNT_UNAVAILABLE")
	case errors.Is(err, model.ErrUserAuthVersionConflict), errors.Is(err, gorm.ErrRecordNotFound):
		writeMiniAppAuthError(c, http.StatusUnauthorized, "MINI_AUTH_FLOW_INVALID")
	case errors.Is(err, model.ErrAuthFlowExpired):
		writeMiniAppAuthError(c, http.StatusGone, "MINI_AUTH_FLOW_EXPIRED")
	case errors.Is(err, model.ErrAuthFlowConsumed):
		writeMiniAppAuthError(c, http.StatusConflict, "MINI_AUTH_FLOW_CONSUMED")
	case errors.Is(err, model.ErrExternalIdentityAlreadyClaimed):
		writeMiniAppAuthError(c, http.StatusConflict, "MINI_AUTH_IDENTITY_CONFLICT")
	case errors.Is(err, model.ErrVerifiedPhoneAlreadyClaimed):
		writeMiniAppAuthError(c, http.StatusConflict, "MINI_AUTH_PHONE_CONFLICT")
	case errors.Is(err, model.ErrVerifiedPhoneInvalid):
		writeMiniAppAuthError(c, http.StatusBadRequest, "MINI_AUTH_PHONE_CODE_INVALID")
	case errors.Is(err, model.ErrAuthFlowInvalid):
		writeMiniAppAuthError(c, http.StatusBadRequest, "MINI_AUTH_FLOW_INVALID")
	default:
		writeMiniAppInternalError(c, err)
	}
}

func writeMiniAppInternalError(c *gin.Context, err error) {
	logger.LogError(c.Request.Context(), fmt.Sprintf("miniapp authentication failed path=%s error=%q", c.Request.URL.Path, err.Error()))
	writeMiniAppAuthError(c, http.StatusInternalServerError, "MINI_AUTH_INTERNAL_ERROR")
}

func writeMiniAppAuthError(c *gin.Context, status int, code string) {
	c.JSON(status, gin.H{"success": false, "code": code, "message": http.StatusText(status)})
}
