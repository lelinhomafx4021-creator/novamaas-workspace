package controller

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
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
	Code      string `json:"code"`
	PhoneCode string `json:"phone_code"`
}

type miniAppBindingFlowPayload struct {
	AppId          string `json:"app_id"`
	OpenId         string `json:"open_id"`
	UnionId        string `json:"union_id,omitempty"`
	Phone          string `json:"phone,omitempty"`
	RequiredUserId int    `json:"required_user_id,omitempty"`
}

type miniAppBindRequest struct {
	FlowToken     string `json:"flow_token"`
	Username      string `json:"username"`
	Password      string `json:"password"`
	TwoFactorCode string `json:"two_factor_code,omitempty"`
	AcceptTerms   bool   `json:"accept_terms"`
}

type miniAppRegisterRequest struct {
	FlowToken        string `json:"flow_token"`
	Username         string `json:"username"`
	Password         string `json:"password"`
	Email            string `json:"email,omitempty"`
	VerificationCode string `json:"verification_code,omitempty"`
	AffCode          string `json:"aff_code,omitempty"`
	AcceptTerms      bool   `json:"accept_terms"`
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
	userId, err := model.GetUserIdByScopedExternalIdentity(
		model.ExternalIdentityProviderWeChatMiniApp,
		identity.AppId,
		identity.OpenId,
	)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		writeMiniAppBindingRequired(c, miniAppBindingFlowPayload{
			AppId: identity.AppId, OpenId: identity.OpenId, UnionId: identity.UnionId,
		})
		return
	}
	if err != nil {
		writeMiniAppInternalError(c, err)
		return
	}

	user, err := model.GetUserById(userId, false)
	if err != nil || user.Status != common.UserStatusEnabled {
		writeMiniAppAuthError(c, http.StatusUnauthorized, "MINI_AUTH_ACCOUNT_UNAVAILABLE")
		return
	}
	bundle, err := service.CreateLoginSession(user.Id, "wechat_miniapp", c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		writeAuthSessionError(c, err)
		return
	}
	writeMiniAppAuthBundle(c, user, bundle)
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
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"binding_required":           true,
			"flow_token":                 flowToken,
			"flow_expires_at":            flow.ExpiresAt.Unix(),
			"password_login_enabled":     common.PasswordLoginEnabled,
			"registration_enabled":       binding.Phone != "" && binding.RequiredUserId == 0 && common.RegisterEnabled && common.PasswordRegisterEnabled,
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
	userId, err := model.GetUserIdByScopedExternalIdentity(
		model.ExternalIdentityProviderWeChatMiniApp, identity.AppId, identity.OpenId,
	)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		var owner *model.User
		var requiredUserId int
		var unavailable bool
		err = model.DB.Transaction(func(tx *gorm.DB) error {
			candidate, lookupErr := model.GetTrustedPhoneOwnerWithTx(tx, phone)
			if errors.Is(lookupErr, gorm.ErrRecordNotFound) {
				return nil
			}
			if lookupErr != nil {
				return lookupErr
			}
			if candidate.Status != common.UserStatusEnabled {
				unavailable = true
				return nil
			}
			requiredUserId = candidate.Id
			twoFAEnabled, checkErr := model.IsTwoFAEnabledWithTx(tx, candidate.Id)
			if checkErr != nil {
				return checkErr
			}
			if candidate.Role != common.RoleCommonUser || twoFAEnabled {
				return nil
			}
			phoneSource := "wechat_miniapp"
			if candidate.Phone != "" {
				phoneSource = "system_assigned_wechat"
			}
			if claimErr := model.ClaimVerifiedPhoneWithTx(tx, phone, candidate.Id, phoneSource); claimErr != nil {
				return claimErr
			}
			if claimErr := model.ClaimScopedExternalIdentityWithTx(
				tx, model.ExternalIdentityProviderWeChatMiniApp, identity.AppId, identity.OpenId, candidate.Id,
			); claimErr != nil {
				return claimErr
			}
			owner = candidate
			return nil
		})
		if err != nil {
			writeMiniAppBindingError(c, err)
			return
		}
		if unavailable {
			writeMiniAppAuthError(c, http.StatusUnauthorized, "MINI_AUTH_ACCOUNT_UNAVAILABLE")
			return
		}
		if owner != nil {
			bundle, sessionErr := service.CreateLoginSession(owner.Id, "wechat_miniapp", c.ClientIP(), c.Request.UserAgent())
			if sessionErr != nil {
				writeAuthSessionError(c, sessionErr)
				return
			}
			writeMiniAppAuthBundle(c, owner, bundle)
			return
		}
		writeMiniAppBindingRequired(c, miniAppBindingFlowPayload{
			AppId: identity.AppId, OpenId: identity.OpenId, UnionId: identity.UnionId,
			Phone: phone, RequiredUserId: requiredUserId,
		})
		return
	}
	if err != nil {
		writeMiniAppInternalError(c, err)
		return
	}
	user, err := model.GetUserById(userId, false)
	if err != nil || user.Status != common.UserStatusEnabled {
		writeMiniAppAuthError(c, http.StatusUnauthorized, "MINI_AUTH_ACCOUNT_UNAVAILABLE")
		return
	}
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		return model.ClaimVerifiedPhoneWithTx(tx, phone, user.Id, "wechat_miniapp")
	})
	if err != nil {
		writeMiniAppBindingError(c, err)
		return
	}
	bundle, err := service.CreateLoginSession(user.Id, "wechat_miniapp", c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		writeAuthSessionError(c, err)
		return
	}
	writeMiniAppAuthBundle(c, user, bundle)
}

func MiniAppBind(c *gin.Context) {
	setAuthNoStore(c)
	if !common.PasswordLoginEnabled {
		writeMiniAppAuthError(c, http.StatusForbidden, "MINI_AUTH_PASSWORD_LOGIN_DISABLED")
		return
	}
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

	user, ok := verifyMiniAppAccountCredentials(c, request.Username, request.Password, request.TwoFactorCode)
	if !ok {
		return
	}
	if payload.RequiredUserId != 0 && user.Id != payload.RequiredUserId {
		writeMiniAppAuthError(c, http.StatusConflict, "MINI_AUTH_PHONE_CONFLICT")
		return
	}

	_, err := model.ConsumeAuthFlowWithAction(request.FlowToken, model.AuthFlowMatch{
		Purpose:  model.AuthFlowPurposeWeChatMiniAppBind,
		Provider: model.ExternalIdentityProviderWeChatMiniApp,
		Intent:   model.AuthFlowIntentBind,
	}, func(tx *gorm.DB, consumed *model.AuthFlow) error {
		if consumed.Id != flow.Id {
			return model.ErrAuthFlowInvalid
		}
		if payload.Phone != "" {
			if err := model.ClaimVerifiedPhoneWithTx(tx, payload.Phone, user.Id, "wechat_miniapp"); err != nil {
				return err
			}
		}
		return model.ClaimScopedExternalIdentityWithTx(
			tx,
			model.ExternalIdentityProviderWeChatMiniApp,
			payload.AppId,
			payload.OpenId,
			user.Id,
		)
	})
	if err != nil {
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
	if !common.RegisterEnabled || !common.PasswordRegisterEnabled {
		writeMiniAppAuthError(c, http.StatusForbidden, "MINI_AUTH_REGISTRATION_DISABLED")
		return
	}
	var request miniAppRegisterRequest
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
	if payload.Phone == "" {
		writeMiniAppAuthError(c, http.StatusForbidden, "MINI_AUTH_PHONE_REQUIRED")
		return
	}
	if payload.RequiredUserId != 0 {
		writeMiniAppAuthError(c, http.StatusConflict, "MINI_AUTH_PHONE_CONFLICT")
		return
	}

	request.Username = strings.TrimSpace(request.Username)
	request.Email = model.NormalizeEmail(request.Email)
	validationUser := model.User{Username: request.Username, Password: request.Password, Email: request.Email}
	if request.Username == "" || common.Validate.Struct(&validationUser) != nil {
		writeMiniAppAuthError(c, http.StatusBadRequest, "MINI_AUTH_REGISTRATION_INVALID")
		return
	}
	if common.EmailVerificationEnabled {
		if request.Email == "" || request.VerificationCode == "" ||
			!common.VerifyCodeWithKey(request.Email, request.VerificationCode, common.EmailVerificationPurpose) {
			writeMiniAppAuthError(c, http.StatusBadRequest, "MINI_AUTH_EMAIL_VERIFICATION_INVALID")
			return
		}
	} else {
		request.Email = ""
	}
	exists, err := model.CheckUserExistOrDeleted(request.Username, request.Email)
	if err != nil {
		writeMiniAppInternalError(c, err)
		return
	}
	if exists {
		writeMiniAppAuthError(c, http.StatusConflict, "MINI_AUTH_ACCOUNT_EXISTS")
		return
	}
	inviterId, _ := model.GetUserIdByAffCode(strings.TrimSpace(request.AffCode))
	cleanUser := model.User{
		Username: request.Username, Password: request.Password, DisplayName: request.Username,
		Email: request.Email, InviterId: inviterId, Role: common.RoleCommonUser,
	}
	defaultTokenKey := ""
	if constant.GenerateDefaultToken {
		defaultTokenKey, err = common.GenerateKey()
		if err != nil {
			writeMiniAppInternalError(c, err)
			return
		}
	}

	_, err = model.ConsumeAuthFlowWithAction(request.FlowToken, model.AuthFlowMatch{
		Purpose:  model.AuthFlowPurposeWeChatMiniAppBind,
		Provider: model.ExternalIdentityProviderWeChatMiniApp,
		Intent:   model.AuthFlowIntentBind,
	}, func(tx *gorm.DB, consumed *model.AuthFlow) error {
		if consumed.Id != flow.Id {
			return model.ErrAuthFlowInvalid
		}
		if err := cleanUser.InsertWithTx(tx, inviterId); err != nil {
			return err
		}
		if err := model.ClaimVerifiedPhoneWithTx(tx, payload.Phone, cleanUser.Id, "wechat_miniapp"); err != nil {
			return err
		}
		if err := model.ClaimScopedExternalIdentityWithTx(
			tx,
			model.ExternalIdentityProviderWeChatMiniApp,
			payload.AppId,
			payload.OpenId,
			cleanUser.Id,
		); err != nil {
			return err
		}
		if defaultTokenKey != "" {
			token := model.Token{
				UserId: cleanUser.Id, Name: cleanUser.Username + "的初始令牌", Key: defaultTokenKey,
				CreatedTime: common.GetTimestamp(), AccessedTime: common.GetTimestamp(), ExpiredTime: -1,
				RemainQuota: 500000, UnlimitedQuota: true, ModelLimitsEnabled: false,
			}
			if setting.DefaultUseAutoGroup {
				token.Group = "auto"
			}
			if err := tx.Create(&token).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, model.ErrExternalIdentityAlreadyClaimed) || errors.Is(err, model.ErrVerifiedPhoneAlreadyClaimed) {
			writeMiniAppBindingError(c, err)
			return
		}
		writeMiniAppAuthError(c, http.StatusConflict, "MINI_AUTH_ACCOUNT_EXISTS")
		return
	}
	cleanUser.FinalizeOAuthUserCreation(inviterId)
	bundle, err := service.CreateLoginSession(cleanUser.Id, "wechat_miniapp", c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		writeAuthSessionError(c, err)
		return
	}
	writeMiniAppAuthBundle(c, &cleanUser, bundle)
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
