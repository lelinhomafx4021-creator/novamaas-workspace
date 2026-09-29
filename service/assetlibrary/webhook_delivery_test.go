package assetlibrary

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	coreService "github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestWebhookRetentionDeletesOnlyOldTerminalDeliveriesInBatches(t *testing.T) {
	db := setupAssetLibraryTestDB(t)
	now := time.Now().Unix()
	old := now - int64((91*24*time.Hour)/time.Second)
	for _, item := range []struct {
		id        string
		status    string
		createdAt int64
	}{
		{"wh_old_success_1", model.AssetWebhookDeliveryStatusSucceeded, old},
		{"wh_old_success_2", model.AssetWebhookDeliveryStatusSucceeded, old},
		{"wh_old_superseded", model.AssetWebhookDeliveryStatusSuperseded, old},
		{"wh_old_exhausted", model.AssetWebhookDeliveryStatusExhausted, old},
		{"wh_old_pending", model.AssetWebhookDeliveryStatusPending, old},
		{"wh_old_retryable", model.AssetWebhookDeliveryStatusFailed, old},
		{"wh_recent_success", model.AssetWebhookDeliveryStatusSucceeded, now - int64((29*24*time.Hour)/time.Second)},
		{"wh_recent_exhausted", model.AssetWebhookDeliveryStatusExhausted, now - int64((89*24*time.Hour)/time.Second)},
	} {
		require.NoError(t, db.Create(&model.AssetWebhookDelivery{
			WebhookID: item.id, Status: item.status, CreatedAt: item.createdAt,
		}).Error)
	}
	cutoff := now - int64(assetWebhookSuccessRetention/time.Second)
	deleted, err := model.PurgeAssetWebhookDeliveriesBefore(context.Background(),
		[]string{model.AssetWebhookDeliveryStatusSucceeded, model.AssetWebhookDeliveryStatusSuperseded}, cutoff, 2)
	require.NoError(t, err)
	assert.EqualValues(t, 2, deleted)
	assert.Error(t, func() error {
		_, err := model.PurgeAssetWebhookDeliveriesBefore(context.Background(),
			[]string{model.AssetWebhookDeliveryStatusPending}, cutoff, 2)
		return err
	}())

	purgeOldAssetWebhookDeliveries()
	var remaining []model.AssetWebhookDelivery
	require.NoError(t, db.Order("webhook_id").Find(&remaining).Error)
	require.Len(t, remaining, 4)
	var ids []string
	for _, delivery := range remaining {
		ids = append(ids, delivery.WebhookID)
	}
	assert.Equal(t, []string{
		"wh_old_pending", "wh_old_retryable", "wh_recent_exhausted", "wh_recent_success",
	}, ids)
}

func TestWebhookExpiryStopsBeforeEndpointLookup(t *testing.T) {
	db := setupAssetLibraryTestDB(t)
	delivery := model.AssetWebhookDelivery{WebhookID: "wh_expired", EndpointID: 999,
		Status: model.AssetWebhookDeliveryStatusDelivering, LockedBy: "expired-worker",
		CreatedAt: common.GetTimestamp() - int64((72*time.Hour)/time.Second)}
	require.NoError(t, db.Create(&delivery).Error)
	deliverAssetWebhook("expired-worker", &delivery)
	require.NoError(t, db.First(&delivery, delivery.ID).Error)
	assert.Equal(t, model.AssetWebhookDeliveryStatusExhausted, delivery.Status)
	assert.Equal(t, "webhook delivery expired after 72 hours", delivery.LastError)
	assert.Zero(t, delivery.NextAttemptAt)
}

func TestWebhookRetryDoesNotSchedulePastExpiry(t *testing.T) {
	db := setupAssetLibraryTestDB(t)
	delivery := model.AssetWebhookDelivery{WebhookID: "wh_near_expiry",
		Status: model.AssetWebhookDeliveryStatusDelivering, LockedBy: "retry-worker", Attempts: 10,
		CreatedAt: common.GetTimestamp() - int64((71*time.Hour)/time.Second)}
	require.NoError(t, db.Create(&delivery).Error)
	finishWebhookDeliveryFailure("retry-worker", &delivery, 503, errors.New("temporary failure"), false)
	require.NoError(t, db.First(&delivery, delivery.ID).Error)
	assert.Equal(t, model.AssetWebhookDeliveryStatusFailed, delivery.Status)
	assert.Equal(t, delivery.CreatedAt+int64((72*time.Hour)/time.Second), delivery.NextAttemptAt)
}

func TestWebhookEndpointLookupFailureRemainsRetryable(t *testing.T) {
	db := setupAssetLibraryTestDB(t)
	delivery := model.AssetWebhookDelivery{
		WebhookID: "wh_lookup_failure", EndpointID: 1, EventType: WebhookEventAssetFailed,
		Status: model.AssetWebhookDeliveryStatusDelivering, LockedBy: "lookup-worker",
	}
	require.NoError(t, db.Create(&delivery).Error)
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register("endpoint_lookup_failure", func(tx *gorm.DB) {
		if tx.Statement.Table == "asset_webhook_endpoints" {
			tx.AddError(errors.New("temporary database connection failure"))
		}
	}))
	t.Cleanup(func() { _ = db.Callback().Query().Remove("endpoint_lookup_failure") })
	deliverAssetWebhook("lookup-worker", &delivery)
	require.NoError(t, db.First(&delivery, delivery.ID).Error)
	assert.Equal(t, model.AssetWebhookDeliveryStatusFailed, delivery.Status)
	assert.Greater(t, delivery.NextAttemptAt, common.GetTimestamp())
}

func TestWebhookPassClaimsOnlyTheDeliveryBeingSent(t *testing.T) {
	db := setupAssetLibraryTestDB(t)
	t.Setenv("STORAGE_CREDENTIAL_ENCRYPTION_KEY", "claim-at-send-key")
	fetch := system_setting.GetFetchSetting()
	original := *fetch
	fetch.EnableSSRFProtection = false
	t.Cleanup(func() { *fetch = original })
	coreService.InitHttpClient()
	type counts struct {
		Delivering, Pending int64
		Err                 error
	}
	observed := make(chan counts, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		var c counts
		c.Err = db.Model(&model.AssetWebhookDelivery{}).Where("status = ?", model.AssetWebhookDeliveryStatusDelivering).Count(&c.Delivering).Error
		if c.Err == nil {
			c.Err = db.Model(&model.AssetWebhookDelivery{}).Where("status = ?", model.AssetWebhookDeliveryStatusPending).Count(&c.Pending).Error
		}
		observed <- c
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	secret, err := randomWebhookSecret()
	require.NoError(t, err)
	encrypted, err := encryptWebhookEndpointSecret(secret)
	require.NoError(t, err)
	endpoint := model.AssetWebhookEndpoint{PublicID: "we_claim", URL: server.URL, EncryptedSecret: encrypted, Status: model.AssetWebhookEndpointStatusEnabled}
	require.NoError(t, db.Create(&endpoint).Error)
	for _, id := range []string{"wh_claim_1", "wh_claim_2"} {
		require.NoError(t, db.Create(&model.AssetWebhookDelivery{WebhookID: id, EndpointID: endpoint.ID,
			EventType: WebhookEventAssetFailed, Payload: `{}`, Status: model.AssetWebhookDeliveryStatusPending}).Error)
	}
	runWebhookDeliveryPass("claim-worker")
	require.Len(t, observed, 2)
	first := <-observed
	require.NoError(t, first.Err)
	assert.EqualValues(t, 1, first.Delivering)
	assert.EqualValues(t, 1, first.Pending)
	second := <-observed
	require.NoError(t, second.Err)
	assert.EqualValues(t, 1, second.Delivering)
	assert.Zero(t, second.Pending)
}

func TestWebhookDeliveryPostsVerifiableHeadersAndPersistsSuccess(t *testing.T) {
	db := setupAssetLibraryTestDB(t)
	t.Setenv("STORAGE_CREDENTIAL_ENCRYPTION_KEY", "delivery-verification-key")
	fetchSetting := system_setting.GetFetchSetting()
	originalFetchSetting := *fetchSetting
	fetchSetting.EnableSSRFProtection = false
	coreService.InitHttpClient()
	t.Cleanup(func() { *fetchSetting = originalFetchSetting })

	payload := `{"id":"evt_verify","object":"event","type":"asset.failed"}`
	var receivedBody string
	var receivedID, receivedTimestamp, receivedSignature string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, err := io.ReadAll(request.Body)
		require.NoError(t, err)
		receivedBody = string(body)
		receivedID = request.Header.Get("webhook-id")
		receivedTimestamp = request.Header.Get("webhook-timestamp")
		receivedSignature = request.Header.Get("webhook-signature")
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	secret, err := randomWebhookSecret()
	require.NoError(t, err)
	encryptedSecret, err := encryptWebhookEndpointSecret(secret)
	require.NoError(t, err)
	endpoint := model.AssetWebhookEndpoint{
		PublicID: "we_verify", OwnerUserID: 42, Name: "verification", URL: server.URL,
		EventTypes: `["asset.failed"]`, EncryptedSecret: encryptedSecret,
		Status: model.AssetWebhookEndpointStatusEnabled,
	}
	require.NoError(t, db.Create(&endpoint).Error)
	delivery := model.AssetWebhookDelivery{
		EventID: "evt_verify", WebhookID: "wh_verify", EndpointID: endpoint.ID,
		EndpointPublicID: endpoint.PublicID, OwnerUserID: 42, EventType: WebhookEventAssetFailed,
		Payload: payload, Status: model.AssetWebhookDeliveryStatusDelivering, LockedBy: "verify-runner",
	}
	require.NoError(t, db.Create(&delivery).Error)

	deliverAssetWebhook("verify-runner", &delivery)

	assert.Equal(t, payload, receivedBody)
	assert.Equal(t, delivery.WebhookID, receivedID)
	require.NotEmpty(t, receivedTimestamp)
	expectedSignature, err := signAssetWebhook(secret, receivedID, receivedTimestamp, []byte(payload))
	require.NoError(t, err)
	assert.Equal(t, expectedSignature, receivedSignature)

	require.NoError(t, db.First(&delivery, delivery.ID).Error)
	assert.Equal(t, model.AssetWebhookDeliveryStatusSucceeded, delivery.Status)
	assert.Equal(t, 1, delivery.Attempts)
	assert.Equal(t, http.StatusNoContent, delivery.ResponseStatus)
	assert.NotZero(t, delivery.DeliveredAt)
}

func TestWebhookDeliveryPersistsRetryAfterNon2xx(t *testing.T) {
	db := setupAssetLibraryTestDB(t)
	t.Setenv("STORAGE_CREDENTIAL_ENCRYPTION_KEY", "delivery-retry-verification-key")
	fetchSetting := system_setting.GetFetchSetting()
	originalFetchSetting := *fetchSetting
	fetchSetting.EnableSSRFProtection = false
	coreService.InitHttpClient()
	t.Cleanup(func() { *fetchSetting = originalFetchSetting })

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	secret, err := randomWebhookSecret()
	require.NoError(t, err)
	encryptedSecret, err := encryptWebhookEndpointSecret(secret)
	require.NoError(t, err)
	endpoint := model.AssetWebhookEndpoint{
		PublicID: "we_retry_verify", OwnerUserID: 42, Name: "retry verification", URL: server.URL,
		EventTypes: `["asset.failed"]`, EncryptedSecret: encryptedSecret,
		Status: model.AssetWebhookEndpointStatusEnabled,
	}
	require.NoError(t, db.Create(&endpoint).Error)
	delivery := model.AssetWebhookDelivery{
		EventID: "evt_retry_verify", WebhookID: "wh_retry_verify", EndpointID: endpoint.ID,
		EndpointPublicID: endpoint.PublicID, OwnerUserID: 42, EventType: WebhookEventAssetFailed,
		Payload: `{"id":"evt_retry_verify"}`, Status: model.AssetWebhookDeliveryStatusDelivering,
		LockedBy: "retry-runner", CreatedAt: common.GetTimestamp(),
	}
	require.NoError(t, db.Create(&delivery).Error)

	deliverAssetWebhook("retry-runner", &delivery)

	require.NoError(t, db.First(&delivery, delivery.ID).Error)
	assert.Equal(t, model.AssetWebhookDeliveryStatusFailed, delivery.Status)
	assert.Equal(t, 1, delivery.Attempts)
	assert.Equal(t, http.StatusInternalServerError, delivery.ResponseStatus)
	assert.Greater(t, delivery.NextAttemptAt, common.GetTimestamp())
}

func TestWebhookDeliveryUsesConfiguredWorker(t *testing.T) {
	db := setupAssetLibraryTestDB(t)
	t.Setenv("STORAGE_CREDENTIAL_ENCRYPTION_KEY", "delivery-worker-verification-key")
	fetchSetting := system_setting.GetFetchSetting()
	originalFetchSetting := *fetchSetting
	fetchSetting.EnableSSRFProtection = false
	originalWorkerURL := system_setting.WorkerUrl
	originalWorkerKey := system_setting.WorkerValidKey
	coreService.InitHttpClient()
	t.Cleanup(func() {
		*fetchSetting = originalFetchSetting
		system_setting.WorkerUrl = originalWorkerURL
		system_setting.WorkerValidKey = originalWorkerKey
	})

	received := make(chan coreService.WorkerRequest, 1)
	worker := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var workerRequest coreService.WorkerRequest
		if err := common.DecodeJson(request.Body, &workerRequest); err != nil {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		received <- workerRequest
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer worker.Close()
	system_setting.WorkerUrl = worker.URL
	system_setting.WorkerValidKey = "worker-verification-key"

	secret, err := randomWebhookSecret()
	require.NoError(t, err)
	encryptedSecret, err := encryptWebhookEndpointSecret(secret)
	require.NoError(t, err)
	endpoint := model.AssetWebhookEndpoint{
		PublicID: "we_worker_verify", OwnerUserID: 42, Name: "worker verification",
		URL: "https://customer.example.com/webhooks/assets", EventTypes: `["asset.failed"]`,
		EncryptedSecret: encryptedSecret, Status: model.AssetWebhookEndpointStatusEnabled,
	}
	require.NoError(t, db.Create(&endpoint).Error)
	delivery := model.AssetWebhookDelivery{
		EventID: "evt_worker_verify", WebhookID: "wh_worker_verify", EndpointID: endpoint.ID,
		EndpointPublicID: endpoint.PublicID, OwnerUserID: 42, EventType: WebhookEventAssetFailed,
		Payload: `{"id":"evt_worker_verify"}`, Status: model.AssetWebhookDeliveryStatusDelivering,
		LockedBy: "worker-runner",
	}
	require.NoError(t, db.Create(&delivery).Error)

	deliverAssetWebhook("worker-runner", &delivery)

	var workerRequest coreService.WorkerRequest
	select {
	case workerRequest = <-received:
	default:
		require.FailNow(t, "worker did not receive the webhook delivery")
	}
	assert.Equal(t, endpoint.URL, workerRequest.URL)
	assert.Equal(t, system_setting.WorkerValidKey, workerRequest.Key)
	assert.Equal(t, http.MethodPost, workerRequest.Method)
	assert.Equal(t, delivery.WebhookID, workerRequest.Headers["webhook-id"])
	assert.NotEmpty(t, workerRequest.Headers["webhook-signature"])
	assert.JSONEq(t, delivery.Payload, string(workerRequest.Body))
	require.NoError(t, db.First(&delivery, delivery.ID).Error)
	assert.Equal(t, model.AssetWebhookDeliveryStatusSucceeded, delivery.Status)
}

func TestWebhookDeliverySupersedesStaleActiveEvent(t *testing.T) {
	db := setupAssetLibraryTestDB(t)
	asset := model.MediaAsset{
		PublicID: "asset-rejected-before-delivery", OwnerUserID: 42, GroupID: 1,
		StorageObjectID: 9001, Name: "portrait", AssetType: model.AssetTypeImage,
		ContentType: "image/png", Size: 10, Status: model.AssetStatusUnavailable,
		UnavailableReason: model.AssetUnavailableSensitiveContent,
	}
	require.NoError(t, db.Create(&asset).Error)
	delivery := model.AssetWebhookDelivery{
		EventID: "evt_stale_active", WebhookID: "wh_stale_active", EndpointID: 999,
		EndpointPublicID: "we_stale_active", OwnerUserID: 42, AssetID: asset.ID,
		EventType: WebhookEventAssetActive, Payload: `{"id":"evt_stale_active"}`,
		Status: model.AssetWebhookDeliveryStatusDelivering, LockedBy: "stale-runner",
	}
	require.NoError(t, db.Create(&delivery).Error)

	deliverAssetWebhook("stale-runner", &delivery)

	require.NoError(t, db.First(&delivery, delivery.ID).Error)
	assert.Equal(t, model.AssetWebhookDeliveryStatusSuperseded, delivery.Status)
	assert.Equal(t, 0, delivery.Attempts)
	assert.Contains(t, delivery.LastError, "current asset status")
}
