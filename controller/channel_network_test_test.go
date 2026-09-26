package controller

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type channelNetworkRoundTripper func(*http.Request) (*http.Response, error)

func (roundTripper channelNetworkRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return roundTripper(request)
}

type channelNetworkTestConnection struct{}

func (channelNetworkTestConnection) Read([]byte) (int, error)       { return 0, io.EOF }
func (channelNetworkTestConnection) Write(data []byte) (int, error) { return len(data), nil }
func (channelNetworkTestConnection) Close() error                   { return nil }
func (channelNetworkTestConnection) LocalAddr() net.Addr {
	return channelNetworkTestAddress("127.0.0.1:3000")
}
func (channelNetworkTestConnection) RemoteAddr() net.Addr {
	return channelNetworkTestAddress("203.0.113.10:443")
}
func (channelNetworkTestConnection) SetDeadline(time.Time) error      { return nil }
func (channelNetworkTestConnection) SetReadDeadline(time.Time) error  { return nil }
func (channelNetworkTestConnection) SetWriteDeadline(time.Time) error { return nil }

type channelNetworkTestAddress string

func (address channelNetworkTestAddress) Network() string { return "tcp" }
func (address channelNetworkTestAddress) String() string  { return string(address) }

func TestProbeChannelNetworkCapturesConnectionDetails(t *testing.T) {
	client := &http.Client{Transport: channelNetworkRoundTripper(func(request *http.Request) (*http.Response, error) {
		assert.Equal(t, http.MethodHead, request.Method)
		trace := httptrace.ContextClientTrace(request.Context())
		require.NotNil(t, trace)
		trace.ConnectStart("tcp", "video.example.com:443")
		trace.ConnectDone("tcp", "video.example.com:443", nil)
		trace.GotConn(httptrace.GotConnInfo{Conn: channelNetworkTestConnection{}})
		trace.GotFirstResponseByte()
		return &http.Response{
			StatusCode: http.StatusNoContent,
			Proto:      "HTTP/1.1",
			Body:       io.NopCloser(strings.NewReader("")),
			Header:     make(http.Header),
			Request:    request,
		}, nil
	})}

	result, err := probeChannelNetwork(context.Background(), client, "https://video.example.com/health", false)
	require.NoError(t, err)
	assert.Equal(t, "https://video.example.com/health", result.TargetURL)
	assert.Equal(t, "203.0.113.10:443", result.RemoteAddress)
	assert.Equal(t, http.StatusNoContent, result.HTTPStatus)
	assert.Equal(t, "HTTP/1.1", result.Protocol)
	assert.NotNil(t, result.ConnectMilliseconds)
	assert.NotNil(t, result.TTFBMilliseconds)
	assert.GreaterOrEqual(t, result.TotalMilliseconds, int64(0))
	assert.False(t, result.ViaProxy)
}

func TestProbeChannelNetworkRejectsUnsupportedScheme(t *testing.T) {
	result, err := probeChannelNetwork(context.Background(), http.DefaultClient, "ftp://example.com", false)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "http or https")
	assert.Empty(t, result.TargetURL)
}
