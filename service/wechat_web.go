package service

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

type WeChatWebSettings struct {
	AppId       string
	AppSecret   string
	RedirectURI string
}

func ResolveWeChatWebSettings() WeChatWebSettings {
	return WeChatWebSettings{
		AppId:       strings.TrimSpace(os.Getenv("WECHAT_WEB_APP_ID")),
		AppSecret:   strings.TrimSpace(os.Getenv("WECHAT_WEB_APP_SECRET")),
		RedirectURI: strings.TrimSpace(os.Getenv("WECHAT_WEB_REDIRECT_URI")),
	}
}

func (settings WeChatWebSettings) IsReady() bool {
	callback, err := url.Parse(settings.RedirectURI)
	return err == nil && callback.Host != "" && callback.Scheme == "https" && callback.Path == "/oauth/wechat-web" && callback.RawQuery == "" && callback.Fragment == "" && callback.User == nil && settings.AppId != "" && settings.AppSecret != ""
}

func (settings WeChatWebSettings) AuthorizationURL(state string) string {
	query := url.Values{"appid": {settings.AppId}, "redirect_uri": {settings.RedirectURI}, "response_type": {"code"}, "scope": {"snsapi_login"}, "state": {state}, "login_type": {"jssdk"}, "self_redirect": {"false"}}
	return "https://open.weixin.qq.com/connect/qrconnect?" + query.Encode() + "#wechat_redirect"
}

// Website codes and mini program codes are scoped to different applications.
// Only WeChat's server-provided UnionID can correlate the two identities.
func ExchangeWeChatWebCode(ctx context.Context, settings WeChatWebSettings, code string) (string, error) {
	if !settings.IsReady() || code == "" || len(code) > 128 {
		return "", errors.New("WeChat website login is not configured or code is invalid")
	}
	query := url.Values{"appid": {settings.AppId}, "secret": {settings.AppSecret}, "code": {code}, "grant_type": {"authorization_code"}}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.weixin.qq.com/sns/oauth2/access_token?"+query.Encode(), nil)
	if err != nil {
		return "", errors.New("WeChat login service is unavailable")
	}
	client := &http.Client{Timeout: 10 * time.Second}
	response, err := client.Do(request)
	// HTTP errors may embed the URL containing the secret and code.
	if err != nil {
		return "", errors.New("WeChat login service is unavailable")
	}
	defer response.Body.Close()
	var payload struct {
		ErrorCode   int    `json:"errcode"`
		UnionId     string `json:"unionid"`
		OpenId      string `json:"openid"`
		AccessToken string `json:"access_token"`
	}
	if response.StatusCode != http.StatusOK || common.DecodeJson(response.Body, &payload) != nil || payload.ErrorCode != 0 || payload.OpenId == "" || payload.AccessToken == "" {
		return "", errors.New("WeChat authorization failed")
	}
	unionId := strings.TrimSpace(payload.UnionId)
	if unionId != "" {
		return unionId, nil
	}
	userQuery := url.Values{"access_token": {payload.AccessToken}, "openid": {payload.OpenId}, "lang": {"zh_CN"}}
	profileRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.weixin.qq.com/sns/userinfo?"+userQuery.Encode(), nil)
	if err != nil {
		return "", errors.New("WeChat login service is unavailable")
	}
	profileResponse, err := client.Do(profileRequest)
	if err != nil {
		return "", errors.New("WeChat login service is unavailable")
	}
	defer profileResponse.Body.Close()
	var profile struct {
		UnionId   string `json:"unionid"`
		OpenId    string `json:"openid"`
		ErrorCode int    `json:"errcode"`
	}
	if profileResponse.StatusCode != http.StatusOK || common.DecodeJson(profileResponse.Body, &profile) != nil || profile.ErrorCode != 0 || profile.OpenId != payload.OpenId {
		return "", errors.New("WeChat authorization failed")
	}
	return strings.TrimSpace(profile.UnionId), nil
}
