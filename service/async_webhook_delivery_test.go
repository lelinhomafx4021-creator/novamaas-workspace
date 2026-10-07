package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/glebarez/sqlite"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestDisabledCategoryStopsPreviouslyQueuedPersonalDelivery(t *testing.T) {
	db := setupWebhookDeliveryTestDB(t)
	require.NoError(t, db.Where("owner_user_id = 0").Delete(&model.AssetWebhookEndpoint{}).Error)
	requests := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests <- struct{}{}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	fetch := system_setting.GetFetchSetting()
	original := *fetch
	fetch.EnableSSRFProtection = false
	t.Cleanup(func() { *fetch = original })
	InitHttpClient()
	endpoint := model.AssetWebhookEndpoint{PublicID: "we_paused", OwnerUserID: 7, URL: server.URL, EventTypes: `["task.status_changed"]`, Status: model.AssetWebhookEndpointStatusEnabled}
	require.NoError(t, db.Create(&endpoint).Error)
	delivery := model.AssetWebhookDelivery{EventID: "evt_paused", WebhookID: "wh_paused", EndpointID: endpoint.ID, EndpointPublicID: endpoint.PublicID, EventType: model.WebhookEventTaskStatusChanged, Payload: `{}`, Status: model.AssetWebhookDeliveryStatusPending}
	require.NoError(t, db.Create(&delivery).Error)
	RunWebhookDeliveryPass("policy-worker")
	assert.Empty(t, requests, "a pending personal callback cannot bypass the category switch")
	require.NoError(t, db.First(&delivery, delivery.ID).Error)
	assert.Equal(t, model.AssetWebhookDeliveryStatusExhausted, delivery.Status)
	assert.Zero(t, delivery.NextAttemptAt)
}

func TestWebhookExpiryStopsBeforeEndpointLookup(t *testing.T) {
	db := setupWebhookDeliveryTestDB(t)
	delivery := model.AssetWebhookDelivery{WebhookID: "wh_expired", EndpointID: 999,
		Status: model.AssetWebhookDeliveryStatusDelivering, LockedBy: "expired-worker",
		LeaseUntil: common.GetTimestamp() + 60,
		CreatedAt:  common.GetTimestamp() - int64((72*time.Hour)/time.Second)}
	require.NoError(t, db.Create(&delivery).Error)
	deliverWebhook("expired-worker", &delivery)
	require.NoError(t, db.First(&delivery, delivery.ID).Error)
	assert.Equal(t, model.AssetWebhookDeliveryStatusExhausted, delivery.Status)
	assert.Equal(t, "webhook delivery expired after 72 hours", delivery.LastError)
	assert.Zero(t, delivery.NextAttemptAt)
}

func TestMediaTaskWebhookRetriesAnImmutableResponseWithStableDeliveryID(t *testing.T) {
	db := setupWebhookDeliveryTestDB(t)
	fetch := system_setting.GetFetchSetting()
	original := *fetch
	fetch.EnableSSRFProtection = false
	t.Cleanup(func() { *fetch = original })
	InitHttpClient()
	type received struct{ body, id string }
	requests := make(chan received, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		requests <- received{string(body), r.Header.Get("webhook-id")}
		if len(requests) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	endpoint := model.AssetWebhookEndpoint{PublicID: "we_task", OwnerUserID: 7, URL: server.URL, EventTypes: `["task.status_changed"]`, Status: model.AssetWebhookEndpointStatusEnabled}
	require.NoError(t, db.Create(&endpoint).Error)
	task := model.Task{TaskID: "task_public", UserId: 7, Status: model.TaskStatusSuccess}
	task.SetData(map[string]any{"url": "https://media.example/result.mp4", "status": "completed"})
	require.NoError(t, task.Insert())
	RunWebhookDeliveryPass("task-first")
	var delivery model.AssetWebhookDelivery
	require.NoError(t, db.First(&delivery).Error)
	assert.Equal(t, model.AssetWebhookDeliveryStatusFailed, delivery.Status)
	assert.Equal(t, 1, delivery.Attempts)
	require.NoError(t, db.Model(&task).Update("data", `{"status":"different snapshot"}`).Error)
	require.NoError(t, db.Model(&delivery).Update("next_attempt_at", common.GetTimestamp()).Error)
	RunWebhookDeliveryPass("task-retry")
	require.NoError(t, db.First(&delivery, delivery.ID).Error)
	assert.Equal(t, model.AssetWebhookDeliveryStatusSucceeded, delivery.Status)
	assert.Equal(t, 2, delivery.Attempts)
	require.Len(t, requests, 2)
	first, second := <-requests, <-requests
	assert.Equal(t, first, second)
	assert.Equal(t, delivery.WebhookID, first.id)
	var event struct {
		Type string `json:"type"`
		Data struct {
			TaskID   string         `json:"task_id"`
			Status   string         `json:"status"`
			Response map[string]any `json:"response"`
		} `json:"data"`
	}
	require.NoError(t, common.UnmarshalJsonStr(first.body, &event))
	assert.Equal(t, model.WebhookEventTaskStatusChanged, event.Type)
	assert.Equal(t, "task_public", event.Data.TaskID)
	assert.Equal(t, "SUCCESS", event.Data.Status)
	assert.Equal(t, "https://media.example/result.mp4", event.Data.Response["url"])
}

func TestWebhookRetryDoesNotSchedulePastExpiry(t *testing.T) {
	db := setupWebhookDeliveryTestDB(t)
	delivery := model.AssetWebhookDelivery{WebhookID: "wh_near_expiry",
		Status: model.AssetWebhookDeliveryStatusDelivering, LockedBy: "retry-worker", Attempts: 10,
		LeaseUntil: common.GetTimestamp() + 60,
		CreatedAt:  common.GetTimestamp() - int64((71*time.Hour)/time.Second)}
	require.NoError(t, db.Create(&delivery).Error)
	finishWebhookDeliveryFailure(context.Background(), "retry-worker", &delivery, 503, errors.New("temporary failure"), false)
	require.NoError(t, db.First(&delivery, delivery.ID).Error)
	assert.Equal(t, model.AssetWebhookDeliveryStatusFailed, delivery.Status)
	assert.Equal(t, delivery.CreatedAt+int64((72*time.Hour)/time.Second), delivery.NextAttemptAt)
}

func TestWebhookEndpointLookupFailureRemainsRetryable(t *testing.T) {
	db := setupWebhookDeliveryTestDB(t)
	delivery := model.AssetWebhookDelivery{
		WebhookID: "wh_lookup_failure", EndpointID: 1, EventType: model.WebhookEventAssetFailed,
		Status: model.AssetWebhookDeliveryStatusDelivering, LockedBy: "lookup-worker",
		LeaseUntil: common.GetTimestamp() + 60,
	}
	require.NoError(t, db.Create(&delivery).Error)
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register("endpoint_lookup_failure", func(tx *gorm.DB) {
		if tx.Statement.Table == "asset_webhook_endpoints" {
			tx.AddError(errors.New("temporary database connection failure"))
		}
	}))
	t.Cleanup(func() { _ = db.Callback().Query().Remove("endpoint_lookup_failure") })
	deliverWebhook("lookup-worker", &delivery)
	require.NoError(t, db.First(&delivery, delivery.ID).Error)
	assert.Equal(t, model.AssetWebhookDeliveryStatusFailed, delivery.Status)
	assert.Greater(t, delivery.NextAttemptAt, common.GetTimestamp())
}

func TestWebhookPassClaimsOnlyTheDeliveryBeingSent(t *testing.T) {
	db := setupWebhookDeliveryTestDB(t)
	fetch := system_setting.GetFetchSetting()
	original := *fetch
	fetch.EnableSSRFProtection = false
	t.Cleanup(func() { *fetch = original })
	InitHttpClient()
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
	endpoint := model.AssetWebhookEndpoint{PublicID: "we_claim", URL: server.URL, Status: model.AssetWebhookEndpointStatusEnabled}
	require.NoError(t, db.Create(&endpoint).Error)
	for _, id := range []string{"wh_claim_1", "wh_claim_2"} {
		require.NoError(t, db.Create(&model.AssetWebhookDelivery{WebhookID: id, EndpointID: endpoint.ID,
			EventType: model.WebhookEventAssetFailed, Payload: `{}`, Status: model.AssetWebhookDeliveryStatusPending}).Error)
	}
	RunWebhookDeliveryPass("claim-worker")
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

func TestWebhookDeliveryPostsUnsignedJSONWithoutEncryptionConfiguration(t *testing.T) {
	db := setupWebhookDeliveryTestDB(t)
	t.Setenv("STORAGE_CREDENTIAL_ENCRYPTION_KEY", "")
	t.Setenv("CRYPTO_SECRET", "")
	t.Setenv("SESSION_SECRET", "")
	fetchSetting := system_setting.GetFetchSetting()
	originalFetchSetting := *fetchSetting
	fetchSetting.EnableSSRFProtection = false
	InitHttpClient()
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

	endpoint := model.AssetWebhookEndpoint{
		PublicID: "we_verify", OwnerUserID: 42, Name: "verification", URL: server.URL,
		EventTypes: `["asset.failed"]`,
		Status:     model.AssetWebhookEndpointStatusEnabled,
	}
	require.NoError(t, db.Create(&endpoint).Error)
	delivery := model.AssetWebhookDelivery{
		EventID: "evt_verify", WebhookID: "wh_verify", EndpointID: endpoint.ID,
		EndpointPublicID: endpoint.PublicID, OwnerUserID: 42, EventType: model.WebhookEventAssetFailed,
		Payload: payload, Status: model.AssetWebhookDeliveryStatusDelivering, LockedBy: "verify-runner",
		LeaseUntil: common.GetTimestamp() + 60,
	}
	require.NoError(t, db.Create(&delivery).Error)

	deliverWebhook("verify-runner", &delivery)

	assert.Equal(t, payload, receivedBody)
	assert.Equal(t, delivery.WebhookID, receivedID)
	require.NotEmpty(t, receivedTimestamp)
	assert.Empty(t, receivedSignature)

	require.NoError(t, db.First(&delivery, delivery.ID).Error)
	assert.Equal(t, model.AssetWebhookDeliveryStatusSucceeded, delivery.Status)
	assert.Equal(t, 1, delivery.Attempts)
	assert.Equal(t, http.StatusNoContent, delivery.ResponseStatus)
	assert.NotZero(t, delivery.DeliveredAt)
}

func TestWebhookDeliveryPersistsRetryAfterNon2xx(t *testing.T) {
	db := setupWebhookDeliveryTestDB(t)
	fetchSetting := system_setting.GetFetchSetting()
	originalFetchSetting := *fetchSetting
	fetchSetting.EnableSSRFProtection = false
	InitHttpClient()
	t.Cleanup(func() { *fetchSetting = originalFetchSetting })

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	endpoint := model.AssetWebhookEndpoint{
		PublicID: "we_retry_verify", OwnerUserID: 42, Name: "retry verification", URL: server.URL,
		EventTypes: `["asset.failed"]`,
		Status:     model.AssetWebhookEndpointStatusEnabled,
	}
	require.NoError(t, db.Create(&endpoint).Error)
	delivery := model.AssetWebhookDelivery{
		EventID: "evt_retry_verify", WebhookID: "wh_retry_verify", EndpointID: endpoint.ID,
		EndpointPublicID: endpoint.PublicID, OwnerUserID: 42, EventType: model.WebhookEventAssetFailed,
		Payload: `{"id":"evt_retry_verify"}`, Status: model.AssetWebhookDeliveryStatusDelivering,
		LockedBy: "retry-runner", CreatedAt: common.GetTimestamp(),
		LeaseUntil: common.GetTimestamp() + 60,
	}
	require.NoError(t, db.Create(&delivery).Error)

	deliverWebhook("retry-runner", &delivery)

	require.NoError(t, db.First(&delivery, delivery.ID).Error)
	assert.Equal(t, model.AssetWebhookDeliveryStatusFailed, delivery.Status)
	assert.Equal(t, 1, delivery.Attempts)
	assert.Equal(t, http.StatusInternalServerError, delivery.ResponseStatus)
	assert.Greater(t, delivery.NextAttemptAt, common.GetTimestamp())
}

func TestWebhookDeliveryUsesConfiguredWorker(t *testing.T) {
	db := setupWebhookDeliveryTestDB(t)
	fetchSetting := system_setting.GetFetchSetting()
	originalFetchSetting := *fetchSetting
	fetchSetting.EnableSSRFProtection = false
	originalWorkerURL := system_setting.WorkerUrl
	originalWorkerKey := system_setting.WorkerValidKey
	InitHttpClient()
	t.Cleanup(func() {
		*fetchSetting = originalFetchSetting
		system_setting.WorkerUrl = originalWorkerURL
		system_setting.WorkerValidKey = originalWorkerKey
	})

	received := make(chan WorkerRequest, 1)
	worker := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var workerRequest WorkerRequest
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

	endpoint := model.AssetWebhookEndpoint{
		PublicID: "we_worker_verify", OwnerUserID: 42, Name: "worker verification",
		URL: "https://customer.example.com/webhooks/assets", EventTypes: `["asset.failed"]`,
		Status: model.AssetWebhookEndpointStatusEnabled,
	}
	require.NoError(t, db.Create(&endpoint).Error)
	delivery := model.AssetWebhookDelivery{
		EventID: "evt_worker_verify", WebhookID: "wh_worker_verify", EndpointID: endpoint.ID,
		EndpointPublicID: endpoint.PublicID, OwnerUserID: 42, EventType: model.WebhookEventAssetFailed,
		Payload: `{"id":"evt_worker_verify"}`, Status: model.AssetWebhookDeliveryStatusDelivering,
		LockedBy:   "worker-runner",
		LeaseUntil: common.GetTimestamp() + 60,
	}
	require.NoError(t, db.Create(&delivery).Error)

	deliverWebhook("worker-runner", &delivery)

	var workerRequest WorkerRequest
	select {
	case workerRequest = <-received:
	default:
		require.FailNow(t, "worker did not receive the webhook delivery")
	}
	assert.Equal(t, endpoint.URL, workerRequest.URL)
	assert.Equal(t, system_setting.WorkerValidKey, workerRequest.Key)
	assert.Equal(t, http.MethodPost, workerRequest.Method)
	assert.Equal(t, delivery.WebhookID, workerRequest.Headers["webhook-id"])
	assert.Empty(t, workerRequest.Headers["webhook-signature"])
	assert.JSONEq(t, delivery.Payload, string(workerRequest.Body))
	require.NoError(t, db.First(&delivery, delivery.ID).Error)
	assert.Equal(t, model.AssetWebhookDeliveryStatusSucceeded, delivery.Status)
}

func TestWebhookDeliverySupersedesStaleActiveEvent(t *testing.T) {
	db := setupWebhookDeliveryTestDB(t)
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
		EventType: model.WebhookEventAssetActive, Payload: `{"id":"evt_stale_active"}`,
		Status: model.AssetWebhookDeliveryStatusDelivering, LockedBy: "stale-runner",
		LeaseUntil: common.GetTimestamp() + 60,
	}
	require.NoError(t, db.Create(&delivery).Error)

	deliverWebhook("stale-runner", &delivery)

	require.NoError(t, db.First(&delivery, delivery.ID).Error)
	assert.Equal(t, model.AssetWebhookDeliveryStatusSuperseded, delivery.Status)
	assert.Equal(t, 0, delivery.Attempts)
	assert.Contains(t, delivery.LastError, "current asset status")
}

func TestWebhookDeliveryDoesNotSendAfterLeaseLoss(t *testing.T) {
	for _, scenario := range []string{"expired", "taken_over", "taken_over_during_lookup"} {
		t.Run(scenario, func(t *testing.T) {
			db := setupWebhookDeliveryTestDB(t)
			fetch := system_setting.GetFetchSetting()
			previousFetch, previousWorkerURL := *fetch, system_setting.WorkerUrl
			fetch.EnableSSRFProtection = false
			system_setting.WorkerUrl = ""
			InitHttpClient()
			t.Cleanup(func() {
				*fetch = previousFetch
				system_setting.WorkerUrl = previousWorkerURL
			})
			received := make(chan string, 2)
			callback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				received <- r.Header.Get("webhook-id")
				w.WriteHeader(http.StatusNoContent)
			}))
			defer callback.Close()
			endpoint := model.AssetWebhookEndpoint{PublicID: "we_lease", URL: callback.URL, Status: model.AssetWebhookEndpointStatusEnabled}
			require.NoError(t, db.Create(&endpoint).Error)
			delivery := model.AssetWebhookDelivery{WebhookID: "wh_lease", EndpointID: endpoint.ID,
				EventType: model.WebhookEventAssetFailed, Payload: `{}`, Status: model.AssetWebhookDeliveryStatusPending}
			require.NoError(t, db.Create(&delivery).Error)
			now := common.GetTimestamp()
			first, err := model.ClaimAssetWebhookDeliveries(now, now+60, "node-A", 1)
			require.NoError(t, err)
			require.Len(t, first, 1)

			if scenario == "taken_over_during_lookup" {
				require.NoError(t, db.Callback().Query().After("gorm:query").Register("take_over_during_lookup", func(tx *gorm.DB) {
					if tx.Statement.Table != "asset_webhook_endpoints" {
						return
					}
					if err := db.Model(&model.AssetWebhookDelivery{}).Where("id = ?", delivery.ID).
						Update("lease_until", now-1).Error; err != nil {
						tx.AddError(err)
						return
					}
					_, err := model.ClaimAssetWebhookDeliveries(now, now+60, "node-B", 1)
					if err != nil {
						tx.AddError(err)
					}
				}))
				t.Cleanup(func() { _ = db.Callback().Query().Remove("take_over_during_lookup") })
			} else {
				require.NoError(t, db.Model(&model.AssetWebhookDelivery{}).Where("id = ?", delivery.ID).
					Update("lease_until", now-1).Error)
				if scenario == "expired" {
					first[0].LeaseUntil = now - 1
				} else {
					second, err := model.ClaimAssetWebhookDeliveries(now, now+60, "node-B", 1)
					require.NoError(t, err)
					require.Len(t, second, 1)
				}
			}

			deliverWebhook("node-A", first[0])
			assert.Empty(t, received, "a stale worker must not POST to the customer")
			if scenario == "taken_over_during_lookup" {
				require.NoError(t, db.Callback().Query().Remove("take_over_during_lookup"))
			}
			if scenario == "expired" {
				second, err := model.ClaimAssetWebhookDeliveries(now, now+60, "node-B", 1)
				require.NoError(t, err)
				require.Len(t, second, 1)
			}
			require.NoError(t, db.First(&delivery, delivery.ID).Error)
			assert.Equal(t, "node-B", delivery.LockedBy)
			assert.Zero(t, delivery.Attempts)
			deliverWebhook("node-B", &delivery)
			require.Len(t, received, 1, "the new owner must deliver exactly once in this recovery scenario")
			assert.Equal(t, delivery.WebhookID, <-received)
			require.NoError(t, db.First(&delivery, delivery.ID).Error)
			assert.Equal(t, model.AssetWebhookDeliveryStatusSucceeded, delivery.Status)
		})
	}
}

func TestWebhookDeliveryRetriesWithSameIDAfterSuccessWriteFailure(t *testing.T) {
	db := setupWebhookDeliveryTestDB(t)
	fetch := system_setting.GetFetchSetting()
	previousFetch, previousWorkerURL := *fetch, system_setting.WorkerUrl
	fetch.EnableSSRFProtection = false
	system_setting.WorkerUrl = ""
	InitHttpClient()
	t.Cleanup(func() {
		*fetch = previousFetch
		system_setting.WorkerUrl = previousWorkerURL
	})
	type receipt struct{ id, body string }
	received := make(chan receipt, 2)
	callback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		received <- receipt{r.Header.Get("webhook-id"), string(body)}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer callback.Close()
	endpoint := model.AssetWebhookEndpoint{PublicID: "we_write_failure", URL: callback.URL, Status: model.AssetWebhookEndpointStatusEnabled}
	require.NoError(t, db.Create(&endpoint).Error)
	delivery := model.AssetWebhookDelivery{WebhookID: "wh_write_failure", EndpointID: endpoint.ID,
		EventType: model.WebhookEventAssetFailed, Payload: `{"id":"evt_write_failure","type":"asset.failed"}`, Status: model.AssetWebhookDeliveryStatusPending}
	require.NoError(t, db.Create(&delivery).Error)
	now := common.GetTimestamp()
	first, err := model.ClaimAssetWebhookDeliveries(now, now+60, "node-A", 1)
	require.NoError(t, err)
	require.Len(t, first, 1)
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register("webhook_success_write_failure", func(tx *gorm.DB) {
		values, ok := tx.Statement.Dest.(map[string]any)
		if tx.Statement.Table == "asset_webhook_deliveries" && ok && values["status"] == model.AssetWebhookDeliveryStatusSucceeded {
			tx.AddError(errors.New("simulated database write failure"))
		}
	}))
	t.Cleanup(func() { _ = db.Callback().Update().Remove("webhook_success_write_failure") })
	deliverWebhook("node-A", first[0])
	require.Len(t, received, 1)
	require.NoError(t, db.First(&delivery, delivery.ID).Error)
	assert.Equal(t, model.AssetWebhookDeliveryStatusDelivering, delivery.Status)
	require.NoError(t, db.Callback().Update().Remove("webhook_success_write_failure"))

	// If success cannot be recorded, recovery must retry rather than lose the
	// event. The customer's deduplication key and payload must remain stable.
	require.NoError(t, db.Model(&delivery).Update("lease_until", now-1).Error)
	second, err := model.ClaimAssetWebhookDeliveries(now, now+60, "node-B", 1)
	require.NoError(t, err)
	require.Len(t, second, 1)
	deliverWebhook("node-B", second[0])
	require.Len(t, received, 2)
	a, b := <-received, <-received
	assert.Equal(t, receipt{delivery.WebhookID, delivery.Payload}, a)
	assert.Equal(t, a, b)
	require.NoError(t, db.First(&delivery, delivery.ID).Error)
	assert.Equal(t, model.AssetWebhookDeliveryStatusSucceeded, delivery.Status)
}

func setupWebhookDeliveryTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	previous := model.DB
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:webhook-delivery-%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&model.AssetWebhookEndpoint{}, &model.AssetWebhookDelivery{}, &model.MediaAsset{}, &model.Task{}))
	require.NoError(t, db.Create(&[]model.AssetWebhookEndpoint{
		{PublicID: model.SystemAssetWebhookEndpointID, OwnerUserID: 0, Status: model.AssetWebhookEndpointStatusEnabled, EventTypes: `["asset.active","asset.failed"]`},
		{PublicID: model.SystemTaskWebhookEndpointID, OwnerUserID: 0, Status: model.AssetWebhookEndpointStatusEnabled, EventTypes: `["task.status_changed"]`},
	}).Error)
	model.DB = db
	t.Cleanup(func() { model.DB = previous; assert.NoError(t, sqlDB.Close()) })
	return db
}
