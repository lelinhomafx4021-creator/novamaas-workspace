package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type weChatMiniAppRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn weChatMiniAppRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestWeChatMiniAppCodeClientExchangesCodeWithoutReturningSessionKey(t *testing.T) {
	client := NewWeChatMiniAppCodeClient("https://wechat.test/code", time.Second)
	client.httpClient.Transport = weChatMiniAppRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		query := request.URL.Query()
		assert.Equal(t, "wx-app", query.Get("appid"))
		assert.Equal(t, "top-secret", query.Get("secret"))
		assert.Equal(t, "one-time-code", query.Get("js_code"))
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"openid":"openid-1","session_key":"session-secret","unionid":"union-1"}`)),
			Header:     make(http.Header),
		}, nil
	})

	identity, err := client.ExchangeCode(context.Background(), "wx-app", "top-secret", "one-time-code")
	require.NoError(t, err)
	assert.Equal(t, WeChatMiniAppIdentity{AppId: "wx-app", OpenId: "openid-1", UnionId: "union-1"}, identity)
}

func TestWeChatMiniAppCodeClientMapsProviderErrorsAndEmptyOpenId(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		expected error
	}{
		{name: "provider rejection", body: `{"errcode":40029,"errmsg":"invalid code"}`, expected: ErrWeChatMiniAppCodeRejected},
		{name: "empty open id", body: `{"session_key":"hidden"}`, expected: ErrWeChatMiniAppEmptyOpenId},
		{name: "invalid json", body: `{`, expected: ErrWeChatMiniAppUpstream},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := NewWeChatMiniAppCodeClient("https://wechat.test/code", time.Second)
			client.httpClient.Transport = weChatMiniAppRoundTripFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(test.body)), Header: make(http.Header)}, nil
			})
			_, err := client.ExchangeCode(context.Background(), "wx-app", "top-secret", "one-time-code")
			assert.ErrorIs(t, err, test.expected)
			assert.NotContains(t, err.Error(), "top-secret")
			assert.NotContains(t, err.Error(), "one-time-code")
			assert.NotContains(t, err.Error(), "session_key")
		})
	}
}

func TestWeChatMiniAppCodeClientSanitizesTransportErrorsAndHonorsTimeout(t *testing.T) {
	client := NewWeChatMiniAppCodeClient("https://wechat.test/code", 10*time.Millisecond)
	client.httpClient.Transport = weChatMiniAppRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		<-request.Context().Done()
		return nil, errors.New(request.URL.String())
	})

	startedAt := time.Now()
	_, err := client.ExchangeCode(context.Background(), "wx-app", "top-secret", "one-time-code")
	assert.ErrorIs(t, err, ErrWeChatMiniAppUpstream)
	assert.Less(t, time.Since(startedAt), time.Second)
	assert.NotContains(t, err.Error(), "top-secret")
	assert.NotContains(t, err.Error(), "one-time-code")
}
