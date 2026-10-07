package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type weChatWebsiteTransport struct {
	roundTrip func(*http.Request) (*http.Response, error)
}

func (transport weChatWebsiteTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport.roundTrip(request)
}

func TestWeChatWebsiteConfigRequiresItsOwnHTTPSCallback(t *testing.T) {
	for _, callback := range []string{"", "http://example.com/oauth/wechat-web", "https://example.com/other", "https://example.com/oauth/wechat-web?redirect=external"} {
		assert.False(t, (WeChatWebSettings{AppId: "website-app", AppSecret: "secret", RedirectURI: callback}).IsReady())
	}
	settings := WeChatWebSettings{AppId: "website-app", AppSecret: "secret", RedirectURI: "https://example.com/oauth/wechat-web"}
	require.True(t, settings.IsReady())
	authorize, err := url.Parse(settings.AuthorizationURL("state-value"))
	require.NoError(t, err)
	assert.Equal(t, "website-app", authorize.Query().Get("appid"))
	assert.Equal(t, "snsapi_login", authorize.Query().Get("scope"))
	assert.Equal(t, "jssdk", authorize.Query().Get("login_type"))
	assert.Equal(t, "false", authorize.Query().Get("self_redirect"))
	assert.Equal(t, "state-value", authorize.Query().Get("state"))
	assert.NotContains(t, authorize.String(), "secret")
}

func TestWeChatWebsiteCodeUsesServerUnionIDAndUserInfoFallback(t *testing.T) {
	for _, fallback := range []bool{false, true} {
		t.Run(map[bool]string{false: "token_unionid", true: "userinfo_unionid"}[fallback], func(t *testing.T) {
			previous := http.DefaultTransport
			calls := 0
			http.DefaultTransport = weChatWebsiteTransport{roundTrip: func(request *http.Request) (*http.Response, error) {
				calls++
				body := `{"access_token":"upstream-token","openid":"website-openid","unionid":"server-union"}`
				if request.URL.Path == "/sns/oauth2/access_token" {
					assert.Equal(t, "website-secret", request.URL.Query().Get("secret"))
					assert.Equal(t, "website-code", request.URL.Query().Get("code"))
					if fallback {
						body = `{"access_token":"upstream-token","openid":"website-openid"}`
					}
				} else {
					assert.Equal(t, "/sns/userinfo", request.URL.Path)
					assert.Equal(t, "upstream-token", request.URL.Query().Get("access_token"))
					assert.Equal(t, "website-openid", request.URL.Query().Get("openid"))
					body = `{"openid":"website-openid","unionid":"server-union"}`
				}
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
			}}
			t.Cleanup(func() { http.DefaultTransport = previous })
			identity, err := ExchangeWeChatWebCode(context.Background(), WeChatWebSettings{AppId: "website-app", AppSecret: "website-secret", RedirectURI: "https://example.com/oauth/wechat-web"}, "website-code")
			require.NoError(t, err)
			assert.Equal(t, "server-union", identity)
			expected := 1
			if fallback {
				expected = 2
			}
			assert.Equal(t, expected, calls)
		})
	}
}

func TestWeChatWebsiteTransportErrorsDoNotExposeCredentials(t *testing.T) {
	previous := http.DefaultTransport
	http.DefaultTransport = weChatWebsiteTransport{roundTrip: func(request *http.Request) (*http.Response, error) { return nil, errors.New(request.URL.String()) }}
	t.Cleanup(func() { http.DefaultTransport = previous })
	_, err := ExchangeWeChatWebCode(context.Background(), WeChatWebSettings{AppId: "website-app", AppSecret: "private-secret", RedirectURI: "https://example.com/oauth/wechat-web"}, "private-code")
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "private-secret")
	assert.NotContains(t, err.Error(), "private-code")
}

func TestUnspecifiedLoginMethodIsRecordedAsBrowser(t *testing.T) {
	user := setupAuthSessionTestDB(t)
	bundle, err := CreateLoginSession(user.Id, "", "127.0.0.1", "Browser")
	require.NoError(t, err)
	assert.Equal(t, "browser", bundle.Session.LoginMethod)
}
