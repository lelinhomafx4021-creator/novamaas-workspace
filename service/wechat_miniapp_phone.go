package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
)

const weChatMiniAppAccessTokenEndpoint = "https://api.weixin.qq.com/cgi-bin/token"
const weChatMiniAppPhoneEndpoint = "https://api.weixin.qq.com/wxa/business/getuserphonenumber"

var ErrWeChatMiniAppPhoneRejected = errors.New("WeChat rejected the phone authorization code")
var ErrWeChatMiniAppPhoneUpstream = errors.New("WeChat phone service is unavailable")

// Only numeric diagnostics cross the service boundary; URLs and raw WeChat
// messages may contain credentials, authorization codes or personal data.
type WeChatPhoneExchangeError struct {
	Stage  string
	Code   int
	reason error
	token  string
}

func (err *WeChatPhoneExchangeError) Error() string {
	return fmt.Sprintf("WeChat phone exchange failed: stage=%s errcode=%d", err.Stage, err.Code)
}
func (err *WeChatPhoneExchangeError) Unwrap() error { return err.reason }

type WeChatMiniAppPhoneClient struct {
	httpClient    *http.Client
	tokenEndpoint string
	phoneEndpoint string
	mutex         sync.Mutex
	token         string
	tokenAppId    string
	tokenExpiry   time.Time
}

type weChatMiniAppAccessTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	ErrorCode   int    `json:"errcode"`
}

type weChatMiniAppPhoneResponse struct {
	ErrorCode int `json:"errcode"`
	PhoneInfo struct {
		CountryCode     string `json:"countryCode"`
		PurePhoneNumber string `json:"purePhoneNumber"`
		Watermark       struct {
			AppId string `json:"appid"`
		} `json:"watermark"`
	} `json:"phone_info"`
}

var defaultWeChatMiniAppPhoneClient = &WeChatMiniAppPhoneClient{
	httpClient:    &http.Client{Timeout: 5 * time.Second},
	tokenEndpoint: weChatMiniAppAccessTokenEndpoint,
	phoneEndpoint: weChatMiniAppPhoneEndpoint,
}

func (client *WeChatMiniAppPhoneClient) accessToken(ctx context.Context, appId, appSecret string) (string, error) {
	client.mutex.Lock()
	defer client.mutex.Unlock()
	if client.token != "" && client.tokenAppId == appId && time.Now().Before(client.tokenExpiry) {
		return client.token, nil
	}

	endpoint, err := url.Parse(client.tokenEndpoint)
	if err != nil || endpoint.Scheme != "https" || endpoint.Host == "" {
		return "", ErrWeChatMiniAppConfiguration
	}
	query := endpoint.Query()
	query.Set("grant_type", "client_credential")
	query.Set("appid", appId)
	query.Set("secret", appSecret)
	endpoint.RawQuery = query.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return "", ErrWeChatMiniAppConfiguration
	}
	response, err := client.httpClient.Do(request)
	if err != nil {
		// The request URL contains AppSecret; transport errors must not be returned.
		return "", ErrWeChatMiniAppPhoneUpstream
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", ErrWeChatMiniAppPhoneUpstream
	}
	var payload weChatMiniAppAccessTokenResponse
	if err := common.DecodeJson(response.Body, &payload); err != nil {
		return "", ErrWeChatMiniAppPhoneUpstream
	}
	if payload.ErrorCode != 0 {
		return "", &WeChatPhoneExchangeError{Stage: "access_token", Code: payload.ErrorCode, reason: ErrWeChatMiniAppPhoneUpstream}
	}
	if payload.AccessToken == "" || payload.ExpiresIn <= 0 {
		return "", ErrWeChatMiniAppPhoneUpstream
	}
	client.token = payload.AccessToken
	client.tokenAppId = appId
	cacheSeconds := payload.ExpiresIn - 60
	if cacheSeconds < 1 {
		cacheSeconds = 1
	}
	client.tokenExpiry = time.Now().Add(time.Duration(cacheSeconds) * time.Second)
	return client.token, nil
}

func (client *WeChatMiniAppPhoneClient) ExchangeCode(ctx context.Context, appId, appSecret, code string) (string, error) {
	for attempt := 0; attempt < 2; attempt++ {
		phone, err := client.exchangeCode(ctx, appId, appSecret, code)
		var rejected *WeChatPhoneExchangeError
		if attempt != 0 || !errors.As(err, &rejected) || rejected.Stage != "phone" || (rejected.Code != 40001 && rejected.Code != 40014 && rejected.Code != 42001) {
			return phone, err
		}
		// A token rejection does not consume the phone code. Refresh once, never
		// retry invalid/consumed codes or uncertain transport failures.
		client.mutex.Lock()
		if client.token == rejected.token && client.tokenAppId == appId {
			client.token = ""
			client.tokenExpiry = time.Time{}
		}
		client.mutex.Unlock()
	}
	return "", ErrWeChatMiniAppPhoneUpstream
}

func (client *WeChatMiniAppPhoneClient) exchangeCode(ctx context.Context, appId, appSecret, code string) (string, error) {
	if client == nil || client.httpClient == nil || strings.TrimSpace(appId) == "" || strings.TrimSpace(appSecret) == "" {
		return "", ErrWeChatMiniAppConfiguration
	}
	code = strings.TrimSpace(code)
	if code == "" || len(code) > 128 {
		return "", ErrWeChatMiniAppPhoneRejected
	}
	token, err := client.accessToken(ctx, appId, appSecret)
	if err != nil {
		return "", err
	}
	endpoint, err := url.Parse(client.phoneEndpoint)
	if err != nil || endpoint.Scheme != "https" || endpoint.Host == "" {
		return "", ErrWeChatMiniAppConfiguration
	}
	query := endpoint.Query()
	query.Set("access_token", token)
	endpoint.RawQuery = query.Encode()
	body, err := common.Marshal(map[string]string{"code": code})
	if err != nil {
		return "", ErrWeChatMiniAppPhoneUpstream
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return "", ErrWeChatMiniAppPhoneUpstream
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := client.httpClient.Do(request)
	if err != nil {
		return "", ErrWeChatMiniAppPhoneUpstream
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", ErrWeChatMiniAppPhoneUpstream
	}
	var payload weChatMiniAppPhoneResponse
	if err := common.DecodeJson(response.Body, &payload); err != nil {
		return "", ErrWeChatMiniAppPhoneUpstream
	}
	if payload.ErrorCode != 0 {
		return "", &WeChatPhoneExchangeError{Stage: "phone", Code: payload.ErrorCode, reason: ErrWeChatMiniAppPhoneRejected, token: token}
	}
	if payload.PhoneInfo.Watermark.AppId != appId {
		return "", &WeChatPhoneExchangeError{Stage: "watermark", reason: ErrWeChatMiniAppPhoneRejected}
	}
	phone, err := model.WeChatVerifiedPhone(payload.PhoneInfo.CountryCode, payload.PhoneInfo.PurePhoneNumber)
	if err != nil {
		return "", &WeChatPhoneExchangeError{Stage: "phone_number", reason: ErrWeChatMiniAppPhoneRejected}
	}
	return phone, nil
}

func ExchangeWeChatMiniAppPhoneCode(ctx context.Context, code string) (string, error) {
	settings := system_setting.GetWeChatMiniAppSettings().Resolve()
	if !settings.Enabled {
		return "", ErrWeChatMiniAppDisabled
	}
	if !settings.IsReady() {
		return "", ErrWeChatMiniAppConfiguration
	}
	timeout := time.Duration(settings.EffectiveTimeoutSeconds()) * time.Second
	requestContext, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return defaultWeChatMiniAppPhoneClient.ExchangeCode(requestContext, settings.AppId, settings.AppSecret, code)
}
