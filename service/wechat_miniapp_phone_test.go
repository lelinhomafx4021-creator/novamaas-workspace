package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type weChatPhoneRoundTrip func(*http.Request) (*http.Response, error)

func (roundTrip weChatPhoneRoundTrip) RoundTrip(request *http.Request) (*http.Response, error) {
	return roundTrip(request)
}

func TestWeChatMiniAppPhoneExchangeUsesDistinctCodeAndValidatesWatermark(t *testing.T) {
	tokenRequests := 0
	phoneRequests := 0
	client := &WeChatMiniAppPhoneClient{
		httpClient: &http.Client{Transport: weChatPhoneRoundTrip(func(request *http.Request) (*http.Response, error) {
			body := ""
			switch request.URL.Path {
			case "/cgi-bin/token":
				tokenRequests++
				assert.Equal(t, "wx-test-app", request.URL.Query().Get("appid"))
				assert.Equal(t, "test-secret", request.URL.Query().Get("secret"))
				body = `{"access_token":"test-token","expires_in":7200}`
			case "/wxa/business/getuserphonenumber":
				phoneRequests++
				assert.Equal(t, "test-token", request.URL.Query().Get("access_token"))
				requestBody, err := io.ReadAll(request.Body)
				require.NoError(t, err)
				assert.JSONEq(t, `{"code":"phone-code"}`, string(requestBody))
				body = `{"errcode":0,"phone_info":{"countryCode":"86","purePhoneNumber":"13800001234","watermark":{"appid":"wx-test-app"}}}`
			default:
				t.Fatalf("unexpected WeChat endpoint: %s", request.URL.Path)
			}
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}, nil
		})},
		tokenEndpoint: weChatMiniAppAccessTokenEndpoint,
		phoneEndpoint: weChatMiniAppPhoneEndpoint,
	}
	phone, err := client.ExchangeCode(context.Background(), "wx-test-app", "test-secret", "phone-code")
	require.NoError(t, err)
	assert.Equal(t, "+8613800001234", phone)
	_, err = client.ExchangeCode(context.Background(), "wx-test-app", "test-secret", "phone-code")
	require.NoError(t, err)
	assert.Equal(t, 1, tokenRequests)
	assert.Equal(t, 2, phoneRequests)
}

func TestWeChatMiniAppPhoneExchangeRejectsWrongAppId(t *testing.T) {
	client := &WeChatMiniAppPhoneClient{
		httpClient: &http.Client{Transport: weChatPhoneRoundTrip(func(request *http.Request) (*http.Response, error) {
			body := `{"access_token":"test-token","expires_in":7200}`
			if request.URL.Path == "/wxa/business/getuserphonenumber" {
				body = `{"errcode":0,"phone_info":{"countryCode":"86","purePhoneNumber":"13800001234","watermark":{"appid":"other-app"}}}`
			}
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}, nil
		})},
		tokenEndpoint: weChatMiniAppAccessTokenEndpoint,
		phoneEndpoint: weChatMiniAppPhoneEndpoint,
	}
	_, err := client.ExchangeCode(context.Background(), "wx-test-app", "test-secret", "phone-code")
	assert.ErrorIs(t, err, ErrWeChatMiniAppPhoneRejected)
}
