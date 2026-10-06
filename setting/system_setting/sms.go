package system_setting

import (
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
)

type SMSSettings struct {
	Enabled         bool   `json:"enabled"`
	AccessKeyId     string `json:"access_key_id"`
	AccessKeySecret string `json:"access_key_secret"`
	SecurityToken   string `json:"security_token"`
	SignName        string `json:"sign_name"`
	TemplateCode    string `json:"template_code"`
	CodeParameter   string `json:"code_parameter"`
}

var defaultSMSSettings = SMSSettings{CodeParameter: "code"}
var smsCodeParameterPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{0,31}$`)

// SMS configuration follows the database-backed option registry. Explicit
// environment variables override saved values for deployment secret injection.
func init() { config.GlobalConfig.Register("sms", &defaultSMSSettings) }

func GetStoredSMSSettings() SMSSettings {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()
	return defaultSMSSettings
}

func GetSMSSettings() SMSSettings { return GetStoredSMSSettings().Resolve() }

func (settings SMSSettings) EnvironmentOverrides() map[string]string {
	overrides := make(map[string]string)
	for field, name := range map[string]string{
		"enabled": "SMS_ENABLED", "access_key_id": "ALIYUN_SMS_ACCESS_KEY_ID",
		"access_key_secret": "ALIYUN_SMS_ACCESS_KEY_SECRET", "security_token": "ALIYUN_SMS_SECURITY_TOKEN",
		"sign_name": "ALIYUN_SMS_SIGN_NAME", "template_code": "ALIYUN_SMS_TEMPLATE_CODE",
		"code_parameter": "ALIYUN_SMS_CODE_PARAMETER",
	} {
		if value, ok := os.LookupEnv(name); ok && strings.TrimSpace(value) != "" {
			overrides[field] = name
		}
	}
	return overrides
}

func (settings SMSSettings) Resolve() SMSSettings {
	for field, name := range settings.EnvironmentOverrides() {
		value := strings.TrimSpace(os.Getenv(name))
		switch field {
		case "enabled":
			enabled, err := strconv.ParseBool(value)
			settings.Enabled = err == nil && enabled
		case "access_key_id":
			settings.AccessKeyId = value
		case "access_key_secret":
			settings.AccessKeySecret = value
		case "security_token":
			settings.SecurityToken = value
		case "sign_name":
			settings.SignName = value
		case "template_code":
			settings.TemplateCode = value
		case "code_parameter":
			settings.CodeParameter = value
		}
	}
	return settings
}

func ValidSMSCodeParameter(value string) bool { return smsCodeParameterPattern.MatchString(value) }

func (settings SMSSettings) HasCredentials() bool {
	return settings.AccessKeyId != "" && len(settings.AccessKeyId) <= 128 &&
		settings.AccessKeySecret != "" && len(settings.AccessKeySecret) <= 256 &&
		len(settings.SecurityToken) <= 4096 && settings.SignName != "" && len(settings.SignName) <= 128 &&
		settings.TemplateCode != "" && len(settings.TemplateCode) <= 128 && ValidSMSCodeParameter(settings.CodeParameter)
}

func (settings SMSSettings) IsReady() bool { return settings.Enabled && settings.HasCredentials() }
