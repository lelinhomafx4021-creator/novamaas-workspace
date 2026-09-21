package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/system_setting"
)

const weChatCode2SessionEndpoint = "https://api.weixin.qq.com/sns/jscode2session"

var (
	ErrWeChatMiniAppDisabled      = errors.New("WeChat mini program login is disabled")
	ErrWeChatMiniAppConfiguration = errors.New("WeChat mini program login is not configured")
	ErrWeChatMiniAppCodeRejected  = errors.New("WeChat rejected the login code")
	ErrWeChatMiniAppEmptyOpenId   = errors.New("WeChat returned an empty OpenID")
	ErrWeChatMiniAppUpstream      = errors.New("WeChat login service is unavailable")
)

type WeChatMiniAppIdentity struct {
	AppId   string
	OpenId  string
	UnionId string
}

type WeChatMiniAppCodeClient struct {
	endpoint   string
	httpClient *http.Client
}

type weChatCode2SessionResponse struct {
	OpenId     string `json:"openid"`
	SessionKey string `json:"session_key"`
	UnionId    string `json:"unionid"`
	ErrorCode  int    `json:"errcode"`
	ErrorMsg   string `json:"errmsg"`
}

func NewWeChatMiniAppCodeClient(endpoint string, timeout time.Duration) *WeChatMiniAppCodeClient {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		endpoint = weChatCode2SessionEndpoint
	}
	if timeout <= 0 {
		timeout = time.Duration(system_setting.DefaultWeChatMiniAppTimeoutSeconds) * time.Second
	}
	return &WeChatMiniAppCodeClient{
		endpoint:   endpoint,
		httpClient: &http.Client{Timeout: timeout},
	}
}

func (client *WeChatMiniAppCodeClient) ExchangeCode(ctx context.Context, appId, appSecret, code string) (WeChatMiniAppIdentity, error) {
	appId = strings.TrimSpace(appId)
	appSecret = strings.TrimSpace(appSecret)
	code = strings.TrimSpace(code)
	if client == nil || client.httpClient == nil || appId == "" || appSecret == "" {
		return WeChatMiniAppIdentity{}, ErrWeChatMiniAppConfiguration
	}
	if code == "" || len(code) > 128 {
		return WeChatMiniAppIdentity{}, ErrWeChatMiniAppCodeRejected
	}

	endpoint, err := url.Parse(client.endpoint)
	if err != nil || endpoint.Scheme == "" || endpoint.Host == "" {
		return WeChatMiniAppIdentity{}, ErrWeChatMiniAppConfiguration
	}
	query := endpoint.Query()
	query.Set("appid", appId)
	query.Set("secret", appSecret)
	query.Set("js_code", code)
	query.Set("grant_type", "authorization_code")
	endpoint.RawQuery = query.Encode()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return WeChatMiniAppIdentity{}, ErrWeChatMiniAppConfiguration
	}
	response, err := client.httpClient.Do(request)
	if err != nil {
		// net/http errors may include the full URL and query. Never wrap them,
		// because the query contains AppSecret and the one-time login code.
		return WeChatMiniAppIdentity{}, ErrWeChatMiniAppUpstream
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return WeChatMiniAppIdentity{}, ErrWeChatMiniAppUpstream
	}

	var payload weChatCode2SessionResponse
	if err := common.DecodeJson(response.Body, &payload); err != nil {
		return WeChatMiniAppIdentity{}, ErrWeChatMiniAppUpstream
	}
	if payload.ErrorCode != 0 {
		return WeChatMiniAppIdentity{}, fmt.Errorf("%w: errcode %d", ErrWeChatMiniAppCodeRejected, payload.ErrorCode)
	}
	openId := strings.TrimSpace(payload.OpenId)
	if openId == "" {
		return WeChatMiniAppIdentity{}, ErrWeChatMiniAppEmptyOpenId
	}
	return WeChatMiniAppIdentity{
		AppId:   appId,
		OpenId:  openId,
		UnionId: strings.TrimSpace(payload.UnionId),
	}, nil
}

func ExchangeWeChatMiniAppCode(ctx context.Context, code string) (WeChatMiniAppIdentity, error) {
	settings := system_setting.GetWeChatMiniAppSettings().Resolve()
	if !settings.Enabled {
		return WeChatMiniAppIdentity{}, ErrWeChatMiniAppDisabled
	}
	if !settings.IsReady() {
		return WeChatMiniAppIdentity{}, ErrWeChatMiniAppConfiguration
	}
	client := NewWeChatMiniAppCodeClient(
		weChatCode2SessionEndpoint,
		time.Duration(settings.EffectiveTimeoutSeconds())*time.Second,
	)
	return client.ExchangeCode(ctx, settings.AppId, settings.AppSecret, code)
}
