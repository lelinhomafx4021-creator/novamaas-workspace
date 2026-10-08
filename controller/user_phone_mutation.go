package controller

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// A draft is tied to its username; an existing account is tied to its ID and
// credential epoch. Neither can borrow a code from another save operation.
func managedPhoneMutationFlow(username string, target *model.User) string {
	if target == nil {
		return "create:" + strings.TrimSpace(username)
	}
	return fmt.Sprintf("update:%d:%d", target.Id, target.AuthVersion)
}

func SendUserMutationPhoneCode(c *gin.Context) {
	setAuthNoStore(c)
	identity, ok := middleware.GetSessionAuthIdentity(c)
	if !ok {
		phoneMutationError(c, model.ErrSMSInvalid)
		return
	}
	var request struct {
		UserId   int    `json:"user_id"`
		Username string `json:"username"`
		Phone    string `json:"phone"`
	}
	if err := common.DecodeJson(c.Request.Body, &request); err != nil || request.UserId < 0 {
		phoneMutationError(c, model.ErrSMSInvalid)
		return
	}
	var target *model.User
	if request.UserId > 0 {
		var err error
		target, err = model.GetUserById(request.UserId, false)
		if err != nil {
			phoneMutationError(c, err)
			return
		}
		if !canManageTargetRole(c.GetInt("role"), target.Role) {
			c.JSON(http.StatusForbidden, gin.H{"success": false})
			return
		}
	} else if strings.TrimSpace(request.Username) == "" || len(strings.TrimSpace(request.Username)) > 20 {
		phoneMutationError(c, model.ErrSMSInvalid)
		return
	}
	phone, err := model.CanonicalMobilePhone(request.Phone)
	if err != nil {
		phoneMutationError(c, err)
		return
	}
	excludeUserID := 0
	if target != nil {
		excludeUserID = target.Id
		claim, err := model.GetVerifiedPhoneByUser(target.Id)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			phoneMutationError(c, err)
			return
		}
		if claim != nil && claim.Phone == phone {
			phoneMutationError(c, model.ErrSMSInvalid)
			return
		}
	}
	available, err := model.IsPhoneAvailable(phone, excludeUserID)
	if err != nil {
		phoneMutationError(c, err)
		return
	}
	if !available {
		phoneMutationError(c, model.ErrPhoneAlreadyTaken)
		return
	}
	token, err := service.IssueSMS(model.SMSChallengeInput{Purpose: "user_mutation", Phone: phone,
		UserId: identity.UserID, AuthVersion: identity.UserAuthVersion, SessionId: identity.SessionID,
		FlowToken: managedPhoneMutationFlow(request.Username, target), IP: c.ClientIP()}, sendPhoneSMS)
	if err != nil {
		phoneMutationError(c, err)
		return
	}
	writePhoneChallenge(c, token)
}

// The code and the entire user mutation commit together. A rejected code,
// conflicting number or failed permissions update leaves the old account intact.
func commitManagedPhoneMutation(c *gin.Context, request userMutationRequest, phone string, target *model.User, save func(*gorm.DB) error) bool {
	if !system_setting.GetSMSSettings().IsReady() {
		phoneMutationError(c, service.ErrSMSUnavailable)
		return false
	}
	identity, ok := middleware.GetSessionAuthIdentity(c)
	if !ok {
		phoneMutationError(c, model.ErrSMSInvalid)
		return false
	}
	_, err := model.ConsumeSMSChallenge(request.PhoneChallengeToken, request.PhoneVerificationCode,
		model.SMSChallengeInput{Purpose: "user_mutation", Phone: phone, UserId: identity.UserID,
			SessionId: identity.SessionID, FlowToken: managedPhoneMutationFlow(request.Username, target)},
		func(tx *gorm.DB, challenge *model.SMSChallenge) error { return save(tx) })
	if err == nil {
		return true
	}
	if errors.Is(err, model.ErrSMSInvalid) || errors.Is(err, model.ErrPhoneAlreadyTaken) || errors.Is(err, model.ErrVerifiedPhoneAlreadyClaimed) || errors.Is(err, model.ErrVerifiedPhoneInvalid) {
		phoneMutationError(c, err)
	} else {
		common.ApiError(c, err)
	}
	return false
}

// Invalid SMS codes are form validation failures, not expired login sessions.
func phoneMutationError(c *gin.Context, err error) {
	if errors.Is(err, model.ErrSMSInvalid) {
		setAuthNoStore(c)
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": "PHONE_VERIFICATION_INVALID", "message": "PHONE_VERIFICATION_INVALID"})
		return
	}
	phoneError(c, err)
}
