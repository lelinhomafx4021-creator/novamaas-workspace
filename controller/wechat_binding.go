package controller

import (
	"bytes"
	"encoding/base64"
	"errors"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
	"gorm.io/gorm"
)

func WeChatBindingStatus(c *gin.Context) {
	setAuthNoStore(c)
	user, ok := phoneTarget(c)
	if !ok {
		return
	}
	bindings, err := model.GetWeChatBindings(user.Id, c.Param("id") != "")
	if err != nil {
		writeMiniAppInternalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": bindings})
}

// normalizeWeChatAvatar stores a bounded JPEG, never a client URL or metadata.
// Dimensions are checked before decoding to bound memory as well as input size.
func normalizeWeChatAvatar(encoded string) ([]byte, error) {
	if len(encoded) > 2800000 {
		return nil, model.ErrAuthFlowInvalid
	}
	input, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(input) > 2*1024*1024 {
		return nil, model.ErrAuthFlowInvalid
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(input))
	if err != nil || config.Width <= 0 || config.Height <= 0 || config.Width > 4096 || config.Height > 4096 || config.Width*config.Height > 16000000 {
		return nil, model.ErrAuthFlowInvalid
	}
	source, _, err := image.Decode(bytes.NewReader(input))
	if err != nil {
		return nil, model.ErrAuthFlowInvalid
	}
	width, height := config.Width, config.Height
	if width > 256 || height > 256 {
		if width >= height {
			height = max(1, height*256/width)
			width = 256
		} else {
			width = max(1, width*256/height)
			height = 256
		}
	}
	normalized := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.CatmullRom.Scale(normalized, normalized.Bounds(), source, source.Bounds(), draw.Src, nil)
	var output bytes.Buffer
	if err := jpeg.Encode(&output, normalized, &jpeg.Options{Quality: 85}); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func UpdateWeChatProfile(c *gin.Context) {
	setAuthNoStore(c)
	identity, ok := middleware.GetSessionAuthIdentity(c)
	if !ok {
		writeMiniAppAuthError(c, http.StatusUnauthorized, "MINI_AUTH_REQUIRED")
		return
	}
	var request struct {
		AppId        string  `json:"app_id"`
		Nickname     string  `json:"nickname"`
		AvatarBase64 *string `json:"avatar_base64"`
	}
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		writeMiniAppBindingError(c, model.ErrAuthFlowInvalid)
		return
	}
	request.Nickname = strings.TrimSpace(request.Nickname)
	if !utf8.ValidString(request.Nickname) || utf8.RuneCountInString(request.Nickname) > 64 || strings.ContainsAny(request.Nickname, "\r\n\x00") {
		writeMiniAppBindingError(c, model.ErrAuthFlowInvalid)
		return
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
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		if err := model.ValidateUserAuthVersionWithTx(tx, identity.UserID, identity.UserAuthVersion); err != nil {
			return model.ErrAuthFlowInvalid
		}
		if err := model.RecordWeChatBindingWithTx(tx, identity.UserID, request.AppId, "", ""); err != nil {
			return err
		}
		updates := map[string]interface{}{"nickname": request.Nickname, "profile_source": "user_selected", "updated_at": time.Now()}
		if request.AvatarBase64 != nil {
			updates["avatar"] = avatar
		}
		return tx.Model(&model.WeChatMiniAppProfile{}).Where("user_id = ? AND app_id = ?", identity.UserID, request.AppId).Updates(updates).Error
	})
	if err != nil {
		writeMiniAppBindingError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func WeChatAvatar(c *gin.Context) {
	setAuthNoStore(c)
	user, ok := phoneTarget(c)
	if !ok {
		return
	}
	appId := c.Query("app_id")
	var claim model.ExternalIdentityClaim
	if err := model.DB.Where("provider = ? AND user_id = ? AND app_id = ?", model.ExternalIdentityProviderWeChatMiniApp, user.Id, appId).First(&claim).Error; err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	var profile model.WeChatMiniAppProfile
	if err := model.DB.Select("avatar").Where("user_id = ? AND app_id = ?", user.Id, appId).First(&profile).Error; err != nil || len(profile.Avatar) == 0 {
		c.Status(http.StatusNotFound)
		return
	}
	c.Header("X-Content-Type-Options", "nosniff")
	c.Data(http.StatusOK, "image/jpeg", profile.Avatar)
}

func UnbindWeChat(c *gin.Context) {
	setAuthNoStore(c)
	methods := []string{"password", "2fa", "passkey"}
	if c.Param("id") == "" {
		methods = append(methods, "sms")
	}
	if !middleware.RequireSecurityProof(c, "wechat.manage", methods) {
		return
	}
	user, ok := phoneTarget(c)
	if !ok {
		return
	}
	var request struct {
		AppId string `json:"app_id"`
	}
	if err := common.DecodeJson(c.Request.Body, &request); err != nil || request.AppId == "" {
		writeMiniAppBindingError(c, model.ErrAuthFlowInvalid)
		return
	}
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		if err := model.ValidateUserAuthVersionWithTx(tx, user.Id, user.AuthVersion); err != nil {
			return err
		}
		var claim model.ExternalIdentityClaim
		if err := tx.Where("provider = ? AND app_id = ? AND user_id = ?", model.ExternalIdentityProviderWeChatMiniApp, request.AppId, user.Id).First(&claim).Error; err != nil {
			return err
		}
		available, err := hasAlternativeWeChatLogin(tx, user, request.AppId)
		if err != nil {
			return err
		}
		if !available {
			return errWeChatLastCredential
		}
		if err := tx.Delete(&claim).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ? AND app_id = ?", user.Id, request.AppId).Delete(&model.WeChatMiniAppProfile{}).Error; err != nil {
			return err
		}
		_, err = model.IncrementUserAuthVersionWithTx(tx, user.Id)
		return err
	})
	if errors.Is(err, errWeChatLastCredential) {
		writeMiniAppAuthError(c, http.StatusConflict, "MINI_AUTH_LAST_CREDENTIAL")
		return
	}
	if err != nil {
		writeMiniAppBindingError(c, err)
		return
	}
	if err := model.PublishUserAuthCache(user.Id); err != nil {
		writeMiniAppInternalError(c, err)
		return
	}
	if _, err := model.RevokeAllUserSessions(user.Id, "wechat_unbound"); err != nil {
		writeMiniAppInternalError(c, err)
		return
	}
	model.RecordLog(user.Id, model.LogTypeSystem, "WeChat mini program binding removed")
	if c.Param("id") != "" {
		recordManageAuditFor(c, user.Id, "user.wechat_miniapp_unbind", nil)
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"reauthenticate": c.Param("id") == ""}})
}

var errWeChatLastCredential = errors.New("cannot remove the last available login method")

// Include only methods whose provider is enabled. An authenticator alone is
// a second factor and cannot recover an account after its last login is removed.
func hasAlternativeWeChatLogin(tx *gorm.DB, user *model.User, excludedAppId string) (bool, error) {
	var credentials model.User
	if err := tx.Select("password").Where("id = ?", user.Id).First(&credentials).Error; err != nil {
		return false, err
	}
	if common.PasswordLoginEnabled && credentials.Password != "" {
		return true, nil
	}
	if (common.GitHubOAuthEnabled && user.GitHubId != "") || (common.LinuxDOOAuthEnabled && user.LinuxDOId != "") || (common.WeChatAuthEnabled && user.WeChatId != "") || (common.TelegramOAuthEnabled && user.TelegramId != "") || (system_setting.GetDiscordSettings().Enabled && user.DiscordId != "") || (system_setting.GetOIDCSettings().Enabled && user.OidcId != "") {
		return true, nil
	}
	var count int64
	if system_setting.GetSMSSettings().IsReady() {
		if err := tx.Model(&model.VerifiedPhone{}).Where("user_id = ?", user.Id).Count(&count).Error; err != nil {
			return false, err
		}
		if count > 0 {
			return true, nil
		}
	}
	if system_setting.GetPasskeySettings().Enabled {
		if err := tx.Model(&model.PasskeyCredential{}).Where("user_id = ?", user.Id).Count(&count).Error; err != nil {
			return false, err
		}
		if count > 0 {
			return true, nil
		}
	}
	if err := tx.Table("user_oauth_bindings").Joins("JOIN custom_oauth_providers ON custom_oauth_providers.id = user_oauth_bindings.provider_id").Where("user_oauth_bindings.user_id = ? AND custom_oauth_providers.enabled = ?", user.Id, true).Count(&count).Error; err != nil {
		return false, err
	}
	if count > 0 {
		return true, nil
	}
	// Other AppID bindings are usable only for the currently configured app.
	app := system_setting.GetWeChatMiniAppSettings().Resolve()
	if app.Enabled && app.AppId != excludedAppId {
		if err := tx.Model(&model.ExternalIdentityClaim{}).Where("user_id = ? AND provider = ? AND app_id = ?", user.Id, model.ExternalIdentityProviderWeChatMiniApp, app.AppId).Count(&count).Error; err != nil {
			return false, err
		}
		return count > 0, nil
	}
	return false, nil
}
