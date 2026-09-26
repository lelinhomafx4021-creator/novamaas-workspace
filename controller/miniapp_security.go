package controller

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
)

const miniAppTokenKeyReadScope = "token.key.read"

type miniAppSecurityVerifyRequest struct {
	Password string `json:"password"`
	TwoFA    string `json:"two_fa"`
}

func MiniAppVerifySecurity(c *gin.Context) {
	identity, ok := middleware.GetSessionAuthIdentity(c)
	if !ok {
		writeMiniAppAuthError(c, http.StatusUnauthorized, "MINI_AUTH_INVALID")
		return
	}

	var req miniAppSecurityVerifyRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		writeMiniAppAuthError(c, http.StatusBadRequest, "MINI_SECURITY_INVALID_REQUEST")
		return
	}

	user, err := model.GetUserById(identity.UserID, true)
	if err != nil || user == nil || strings.TrimSpace(req.Password) == "" ||
		!common.ValidatePasswordAndHash(req.Password, user.Password) {
		writeMiniAppAuthError(c, http.StatusUnauthorized, "MINI_SECURITY_INVALID_CREDENTIALS")
		return
	}

	twoFAEnabled, err := model.IsTwoFAEnabled(user.Id)
	if err != nil {
		writeMiniAppInternalError(c, err)
		return
	}
	if twoFAEnabled {
		if strings.TrimSpace(req.TwoFA) == "" {
			writeMiniAppAuthError(c, http.StatusUnauthorized, "MINI_SECURITY_2FA_REQUIRED")
			return
		}
		if !validateMiniAppTwoFactor(user, req.TwoFA) {
			writeMiniAppAuthError(c, http.StatusUnauthorized, "MINI_SECURITY_2FA_INVALID")
			return
		}
	}

	proofToken, expiresAt, err := service.IssueSecurityProof(identity, "password", []string{miniAppTokenKeyReadScope})
	if err != nil {
		writeMiniAppInternalError(c, err)
		return
	}
	model.RecordLog(identity.UserID, model.LogTypeSystem, "小程序 API Key 安全验证成功")

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"proof_token": proofToken,
			"expires_at":  expiresAt,
		},
	})
}

func MiniAppGetTokenKey(c *gin.Context) {
	if !middleware.RequireSecurityProof(c, miniAppTokenKeyReadScope, []string{"password"}) {
		return
	}
	GetTokenKey(c)
}
