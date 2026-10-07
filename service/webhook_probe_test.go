package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestUserAndSystemWebhookProbeReturnsCallbackOutcomeWithoutQueuing(t *testing.T) {
	db := setupWebhookDeliveryTestDB(t)
	requests := make(chan string, 3)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(400)
			return
		}
		requests <- string(body)
		if r.URL.Path == "/failure" {
			w.WriteHeader(503)
			return
		}
		w.WriteHeader(204)
	}))
	defer server.Close()
	savedClient, savedProtectedClient, savedWorker := httpClient, ssrfProtectedHTTPClient, system_setting.WorkerUrl
	httpClient = server.Client()
	ssrfProtectedHTTPClient = server.Client()
	system_setting.WorkerUrl = ""
	fetch := system_setting.GetFetchSetting()
	savedFetch := *fetch
	fetch.EnableSSRFProtection = false
	t.Cleanup(func() {
		httpClient = savedClient
		ssrfProtectedHTTPClient = savedProtectedClient
		system_setting.WorkerUrl = savedWorker
		*fetch = savedFetch
	})
	endpoint := model.AssetWebhookEndpoint{PublicID: "we_probe", OwnerUserID: 7, URL: server.URL, EventTypes: `["asset.failed"]`, Status: model.AssetWebhookEndpointStatusEnabled}
	require.NoError(t, db.Create(&endpoint).Error)
	_, err := ProbeUserWebhook(context.Background(), 8, endpoint.PublicID, false)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	assert.Empty(t, requests)
	result, err := ProbeUserWebhook(context.Background(), 7, endpoint.PublicID, false)
	require.NoError(t, err)
	require.True(t, result.Reachable, result.Error)
	assert.Equal(t, 204, result.HTTPStatus)
	assert.NotEmpty(t, result.EventID)
	assert.NotEmpty(t, result.WebhookID)
	var event model.WebhookEvent
	require.NoError(t, common.UnmarshalJsonStr(<-requests, &event))
	assert.Equal(t, 7, event.OwnerUserID)
	assert.Equal(t, "webhook.test", event.Type)
	result, err = ProbeSystemWebhook(context.Background(), "media_tasks", server.URL+"/failure?token=private-value")
	require.NoError(t, err)
	assert.False(t, result.Reachable)
	require.Equal(t, 503, result.HTTPStatus)
	assert.NotContains(t, result.Error, "private-value")
	<-requests
	result, err = ProbeUserWebhook(context.Background(), 7, endpoint.PublicID, true)
	require.NoError(t, err)
	require.True(t, result.Reachable, result.Error)
	require.NoError(t, common.UnmarshalJsonStr(<-requests, &event))
	assert.Equal(t, "asset.webhook_test", event.Type)
	var count int64
	require.NoError(t, db.Model(&model.AssetWebhookDelivery{}).Count(&count).Error)
	assert.Zero(t, count)
}
