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

type miniAppLoginRequest struct {
	Code string `json:"code"`
}

type miniAppBindingFlowPayload struct {
	AppId   string `json:"app_id"`
	OpenId  string `json:"open_id"`
	UnionId string `json:"union_id,omitempty"`
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
		payload, marshalErr := common.Marshal(miniAppBindingFlowPayload{
			AppId: identity.AppId, OpenId: identity.OpenId, UnionId: identity.UnionId,
		})
		if marshalErr != nil {
			writeMiniAppInternalError(c, marshalErr)
			return
		}
		flowToken, flow, flowErr := model.CreateAuthFlow(model.AuthFlowCreate{
			Purpose:   model.AuthFlowPurposeWeChatMiniAppBind,
			Provider:  model.ExternalIdentityProviderWeChatMiniApp,
			Intent:    model.AuthFlowIntentBind,
			Payload:   string(payload),
			ExpiresAt: time.Now().Add(miniAppBindingFlowTTL),
		})
		if flowErr != nil {
			writeMiniAppInternalError(c, flowErr)
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
				"registration_enabled":       common.RegisterEnabled && common.PasswordRegisterEnabled,
				"email_verification_enabled": common.EmailVerificationEnabled,
				"user_agreement_enabled":     legalSettings.UserAgreement != "",
				"privacy_policy_enabled":     legalSettings.PrivacyPolicy != "",
			},
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

	user := model.User{Username: strings.TrimSpace(request.Username), Password: request.Password}
	if err := user.ValidateAndFill(); err != nil {
		writeMiniAppAuthError(c, http.StatusUnauthorized, "MINI_AUTH_CREDENTIALS_INVALID")
		return
	}
	twoFAEnabled, err := model.IsTwoFAEnabled(user.Id)
	if err != nil {
		writeMiniAppInternalError(c, err)
		return
	}
	if twoFAEnabled {
		if strings.TrimSpace(request.TwoFactorCode) == "" {
			writeMiniAppAuthError(c, http.StatusForbidden, "MINI_AUTH_2FA_REQUIRED")
			return
		}
		if !validateMiniAppTwoFactor(&user, request.TwoFactorCode) {
			writeMiniAppAuthError(c, http.StatusUnauthorized, "MINI_AUTH_2FA_INVALID")
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
	writeMiniAppAuthBundle(c, &user, bundle)
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
		if errors.Is(err, model.ErrExternalIdentityAlreadyClaimed) {
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
