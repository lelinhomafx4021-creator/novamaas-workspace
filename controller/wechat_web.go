package controller

import (
	"crypto/subtle"
	"errors"
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

var exchangeWeChatWebCode = service.ExchangeWeChatWebCode

func WeChatWebLoginStart(c *gin.Context) {
	setAuthNoStore(c)
	settings := service.ResolveWeChatWebSettings()
	if !settings.IsReady() {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "WeChat website login is not configured"})
		return
	}
	state, _, err := model.CreateAuthFlow(model.AuthFlowCreate{Purpose: model.AuthFlowPurposeOAuth, Provider: "wechat-web", Intent: model.AuthFlowIntentLogin, ExpiresAt: time.Now().Add(10 * time.Minute)})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	http.SetCookie(c.Writer, &http.Cookie{Name: "wechat_web_state", Value: state, Path: "/api/oauth/wechat-web", MaxAge: 600, HttpOnly: true, Secure: common.SessionCookieSecure, SameSite: http.SameSiteLaxMode})
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"authorize_url": settings.AuthorizationURL(state)}})
}

func WeChatWebLogin(c *gin.Context) {
	setAuthNoStore(c)
	settings := service.ResolveWeChatWebSettings()
	state := c.Query("state")
	cookie, err := c.Cookie("wechat_web_state")
	if !settings.IsReady() || err != nil || state == "" || subtle.ConstantTimeCompare([]byte(cookie), []byte(state)) != 1 {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "WeChat authorization expired. Please try again."})
		return
	}
	if _, err := model.ConsumeAuthFlow(state, model.AuthFlowMatch{Purpose: model.AuthFlowPurposeOAuth, Provider: "wechat-web", Intent: model.AuthFlowIntentLogin}); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "WeChat authorization expired. Please try again."})
		return
	}
	http.SetCookie(c.Writer, &http.Cookie{Name: "wechat_web_state", Path: "/api/oauth/wechat-web", MaxAge: -1, HttpOnly: true, Secure: common.SessionCookieSecure, SameSite: http.SameSiteLaxMode})
	unionId, err := exchangeWeChatWebCode(c.Request.Context(), settings, c.Query("code"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "WeChat authorization failed"})
		return
	}
	user, err := model.GetWeChatWebLoginUser(unionId)
	if errors.Is(err, model.ErrWeChatWebBindingRequired) {
		c.JSON(http.StatusOK, gin.H{"success": false, "code": "WECHAT_BINDING_REQUIRED", "message": model.ErrWeChatWebBindingRequired.Error()})
		return
	}
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "WeChat account is unavailable"})
		return
	}
	setupLoginAtAuthVersion(user, user.AuthVersion, c)
}
