package assetlibrary

import (
	"context"
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

func TestDisabledProviderMonitoringKeepsDefaultPoolAndProviderBehavior(t *testing.T) {
	previousEnabled := runtimewatch.Enabled()
	runtimewatch.SetEnabled(false)
	t.Cleanup(func() { runtimewatch.SetEnabled(previousEnabled) })
	t.Setenv("STORAGE_CREDENTIAL_ENCRYPTION_KEY", "disabled-asset-monitoring-test-key")
	secret, err := encryptChannelCredential("provider-secret")
	require.NoError(t, err)
	previousTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = previousTransport })
	http.DefaultTransport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		assert.Equal(t, "Bearer provider-secret", request.Header.Get("Authorization"))
		assert.Equal(t, "CreateAssetGroup", request.URL.Query().Get("Action"))
		body, err := io.ReadAll(request.Body)
		require.NoError(t, err)
		assert.Contains(t, string(body), `"Name":"original group"`)
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"Result":{"Id":"group-created"}}`))}, nil
	})
	before := runtimewatch.Snapshot(1).Traffic
	provider, err := newProviderClient(&model.AssetChannelConfig{ChannelID: 90713,
		Protocol: model.AssetChannelProtocolVolcAction, AuthType: model.AssetChannelAuthBearer,
		BaseURL: "https://assets.example.com", EncryptedCredential: secret, QPM: 1000000})
	require.NoError(t, err)
	client, ok := provider.(*volcActionClient)
	require.True(t, ok)
	assert.Nil(t, client.httpClient.Transport, "disabled monitoring must keep the existing default HTTP pool")
	assert.Equal(t, 45*time.Second, client.httpClient.Timeout)
	require.ErrorContains(t, client.httpClient.CheckRedirect(nil, nil), "redirects are not allowed")
	groupID, err := provider.createGroup(context.Background(), "original group", "original description")
	require.NoError(t, err)
	assert.Equal(t, "group-created", groupID)
	assert.Equal(t, before, runtimewatch.Snapshot(1).Traffic)
}
