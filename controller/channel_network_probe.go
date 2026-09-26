package controller

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

const channelNetworkTestTimeout = 15 * time.Second

type channelNetworkTestResult struct {
	TargetURL           string   `json:"target_url"`
	ResolvedAddresses   []string `json:"resolved_addresses,omitempty"`
	RemoteAddress       string   `json:"remote_address,omitempty"`
	DNSMilliseconds     *int64   `json:"dns_ms,omitempty"`
	ConnectMilliseconds *int64   `json:"connect_ms,omitempty"`
	TLSMilliseconds     *int64   `json:"tls_ms,omitempty"`
	TTFBMilliseconds    *int64   `json:"ttfb_ms,omitempty"`
	TotalMilliseconds   int64    `json:"total_ms"`
	HTTPStatus          int      `json:"http_status,omitempty"`
	Protocol            string   `json:"protocol,omitempty"`
	ConnectionReused    bool     `json:"connection_reused"`
	ViaProxy            bool     `json:"via_proxy"`
}

func probeChannelNetwork(ctx context.Context, client *http.Client, rawURL string, viaProxy bool) (channelNetworkTestResult, error) {
	result := channelNetworkTestResult{ViaProxy: viaProxy}
	parsedURL, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return result, fmt.Errorf("invalid channel address: %w", err)
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return result, fmt.Errorf("channel address must use http or https")
	}
	if parsedURL.Hostname() == "" {
		return result, fmt.Errorf("channel address is missing a host")
	}

	displayURL := *parsedURL
	displayURL.User = nil
	result.TargetURL = displayURL.String()

	requestContext, cancel := context.WithTimeout(ctx, channelNetworkTestTimeout)
	defer cancel()

	startedAt := time.Now()
	var mu sync.Mutex
	var dnsStartedAt time.Time
	var connectStartedAt time.Time
	var tlsStartedAt time.Time
	trace := &httptrace.ClientTrace{
		DNSStart: func(httptrace.DNSStartInfo) {
			mu.Lock()
			dnsStartedAt = time.Now()
			mu.Unlock()
		},
		DNSDone: func(info httptrace.DNSDoneInfo) {
			mu.Lock()
			if !dnsStartedAt.IsZero() {
				milliseconds := time.Since(dnsStartedAt).Milliseconds()
				result.DNSMilliseconds = &milliseconds
			}
			for _, address := range info.Addrs {
				result.ResolvedAddresses = append(result.ResolvedAddresses, address.IP.String())
			}
			mu.Unlock()
		},
		ConnectStart: func(_, _ string) {
			mu.Lock()
			connectStartedAt = time.Now()
			mu.Unlock()
		},
		ConnectDone: func(_, _ string, connectErr error) {
			mu.Lock()
			if connectErr == nil && !connectStartedAt.IsZero() {
				milliseconds := time.Since(connectStartedAt).Milliseconds()
				result.ConnectMilliseconds = &milliseconds
			}
			mu.Unlock()
		},
		TLSHandshakeStart: func() {
			mu.Lock()
			tlsStartedAt = time.Now()
			mu.Unlock()
		},
		TLSHandshakeDone: func(_ tls.ConnectionState, handshakeErr error) {
			mu.Lock()
			if handshakeErr == nil && !tlsStartedAt.IsZero() {
				milliseconds := time.Since(tlsStartedAt).Milliseconds()
				result.TLSMilliseconds = &milliseconds
			}
			mu.Unlock()
		},
		GotConn: func(info httptrace.GotConnInfo) {
			mu.Lock()
			result.ConnectionReused = info.Reused
			if info.Conn != nil {
				result.RemoteAddress = info.Conn.RemoteAddr().String()
			}
			mu.Unlock()
		},
		GotFirstResponseByte: func() {
			mu.Lock()
			milliseconds := time.Since(startedAt).Milliseconds()
			result.TTFBMilliseconds = &milliseconds
			mu.Unlock()
		},
	}

	request, err := http.NewRequestWithContext(httptrace.WithClientTrace(requestContext, trace), http.MethodHead, parsedURL.String(), nil)
	if err != nil {
		return result, fmt.Errorf("build channel network request: %w", err)
	}
	request.Close = true

	testClient := *client
	testClient.Timeout = channelNetworkTestTimeout
	testClient.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}
	response, requestErr := testClient.Do(request)

	mu.Lock()
	result.TotalMilliseconds = time.Since(startedAt).Milliseconds()
	if response != nil {
		result.HTTPStatus = response.StatusCode
		result.Protocol = response.Proto
	}
	mu.Unlock()

	if response != nil {
		_ = response.Body.Close()
	}
	if requestErr != nil {
		return result, fmt.Errorf("channel network request failed: %w", requestErr)
	}
	return result, nil
}

func TestChannelNetwork(c *gin.Context) {
	channelID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	channel, err := model.CacheGetChannel(channelID)
	if err != nil {
		channel, err = model.GetChannelById(channelID, true)
		if err != nil {
			common.ApiError(c, err)
			return
		}
	}

	settings := channel.GetSetting()
	client, err := service.GetHttpClientWithProxySettings(settings.Proxy, settings)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	targetURL := channel.GetBaseURL()
	if targetURL == "" && channel.Type >= 0 && channel.Type < len(constant.ChannelBaseURLs) {
		targetURL = constant.ChannelBaseURLs[channel.Type]
	}
	result, probeErr := probeChannelNetwork(c.Request.Context(), client, targetURL, strings.TrimSpace(settings.Proxy) != "")
	if probeErr != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": probeErr.Error(),
			"data":    result,
		})
		return
	}
	common.ApiSuccess(c, result)
}
