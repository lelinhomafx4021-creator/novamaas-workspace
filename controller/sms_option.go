package controller

import (
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
)

var smsOptionsMutex sync.Mutex

// Credentials are write-only. Configuration responses include presence flags
// and deployment override names, never the actual access keys or STS token.
func GetSMSOptions(c *gin.Context) {
	setAuthNoStore(c)
	stored := system_setting.GetStoredSMSSettings()
	effective := stored.Resolve()
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
		"settings": gin.H{"enabled": stored.Enabled, "sign_name": stored.SignName,
			"template_code": stored.TemplateCode, "code_parameter": stored.CodeParameter,
			"access_key_id_configured": stored.AccessKeyId != "", "access_key_secret_configured": stored.AccessKeySecret != "",
			"security_token_configured": stored.SecurityToken != ""},
		"effective": gin.H{"enabled": effective.Enabled, "ready": effective.IsReady(),
			"credentials_complete": effective.HasCredentials(), "sign_name": effective.SignName,
			"template_code": effective.TemplateCode, "code_parameter": effective.CodeParameter,
			"access_key_id_configured": effective.AccessKeyId != "", "access_key_secret_configured": effective.AccessKeySecret != "",
			"security_token_configured": effective.SecurityToken != ""},
		"environment_overrides": stored.EnvironmentOverrides(),
	}})
}

func SaveSMSOptions(c *gin.Context) {
	setAuthNoStore(c)
	var request struct {
		Enabled            *bool   `json:"enabled"`
		AccessKeyId        *string `json:"access_key_id"`
		AccessKeySecret    *string `json:"access_key_secret"`
		SecurityToken      *string `json:"security_token"`
		ClearSecurityToken bool    `json:"clear_security_token"`
		SignName           *string `json:"sign_name"`
		TemplateCode       *string `json:"template_code"`
		CodeParameter      *string `json:"code_parameter"`
	}
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": "SMS_CONFIG_INVALID"})
		return
	}
	smsOptionsMutex.Lock()
	defer smsOptionsMutex.Unlock()
	stored := system_setting.GetStoredSMSSettings()
	values := make(map[string]string)
	if request.Enabled != nil {
		stored.Enabled = *request.Enabled
		values["sms.enabled"] = strconv.FormatBool(stored.Enabled)
	}
	// Empty secret inputs preserve saved credentials, including browser autofill
	// and ordinary edits to the signature/template. STS removal is explicit.
	for _, field := range []struct {
		key         string
		value       *string
		destination *string
		limit       int
		secret      bool
	}{
		{"access_key_id", request.AccessKeyId, &stored.AccessKeyId, 128, true},
		{"access_key_secret", request.AccessKeySecret, &stored.AccessKeySecret, 256, true},
		{"security_token", request.SecurityToken, &stored.SecurityToken, 4096, true},
		{"sign_name", request.SignName, &stored.SignName, 128, false},
		{"template_code", request.TemplateCode, &stored.TemplateCode, 128, false},
		{"code_parameter", request.CodeParameter, &stored.CodeParameter, 32, false},
	} {
		if field.value == nil {
			continue
		}
		value := strings.TrimSpace(*field.value)
		if field.secret && value == "" {
			continue
		}
		if len(value) > field.limit || strings.ContainsAny(value, "\r\n\x00") || (field.key == "code_parameter" && !system_setting.ValidSMSCodeParameter(value)) {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": "SMS_CONFIG_INVALID"})
			return
		}
		*field.destination = value
		values["sms."+field.key] = value
	}
	if request.ClearSecurityToken {
		if request.SecurityToken != nil && strings.TrimSpace(*request.SecurityToken) != "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": "SMS_CONFIG_INVALID"})
			return
		}
		stored.SecurityToken = ""
		values["sms.security_token"] = ""
	}
	effective := stored.Resolve()
	if (stored.Enabled || effective.Enabled) && !effective.HasCredentials() {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": "SMS_CONFIG_INCOMPLETE"})
		return
	}
	if err := model.UpdateOptionsBulk(values); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "code": "SMS_CONFIG_SAVE_FAILED"})
		return
	}
	GetSMSOptions(c)
}
