package storage

import (
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/runtimewatch"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type runtimeStorageRoundTripper func(*http.Request) (*http.Response, error)

func (roundTrip runtimeStorageRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return roundTrip(request)
}

func TestDisabledOSSMonitoringKeepsSDKTransportPolicy(t *testing.T) {
	previousEnabled := runtimewatch.Enabled()
	runtimewatch.SetEnabled(false)
	t.Cleanup(func() { runtimewatch.SetEnabled(previousEnabled) })
	t.Setenv("OSS_ACCESS_KEY_ID", "disabled-storage-test-key")
	t.Setenv("OSS_ACCESS_KEY_SECRET", "disabled-storage-test-secret")
	driver, err := newAliyunOSSDriver(&model.StorageProfile{Region: "cn-hangzhou", Bucket: "test-media-bucket", Endpoint: "https://oss-cn-hangzhou.aliyuncs.com"},
		&model.StorageCredential{AuthType: model.StorageCredentialAuthEnvironment})
	require.NoError(t, err)
	t.Cleanup(driver.CloseIdleConnections)
	client, ok := driver.idleConnections.(*http.Client)
	require.True(t, ok)
	transport, ok := client.Transport.(*http.Transport)
	require.True(t, ok, "disabled monitoring must leave the SDK's native transport in place")
	assert.Equal(t, 10*time.Second, transport.TLSHandshakeTimeout)
	assert.Equal(t, uint16(tls.VersionTLS12), transport.TLSClientConfig.MinVersion)
	assert.ErrorIs(t, client.CheckRedirect(nil, nil), http.ErrUseLastResponse)
	before := runtimewatch.Snapshot(1).Traffic
	client.Transport = runtimeStorageRoundTripper(func(request *http.Request) (*http.Response, error) {
		assert.Equal(t, http.MethodGet, request.Method)
		assert.Equal(t, "/original.mp4", request.URL.Path)
		assert.Contains(t, request.Header.Get("Authorization"), "disabled-storage-test-key")
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("video")), ContentLength: 5, Request: request}, nil
	})
	body, err := driver.GetObject(context.Background(), "original.mp4")
	require.NoError(t, err)
	content, err := io.ReadAll(body)
	require.NoError(t, err)
	require.NoError(t, body.Close())
	assert.Equal(t, "video", string(content))
	assert.Equal(t, before, runtimewatch.Snapshot(1).Traffic)
}
