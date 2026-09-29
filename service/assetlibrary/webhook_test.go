package assetlibrary

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAssetWebhookSignatureMatchesDocumentedHeaderContract(t *testing.T) {
	key := []byte("01234567890123456789012345678901")
	secret := "whsec_" + base64.StdEncoding.EncodeToString(key)
	payload := []byte(`{"id":"evt_example","object":"event","type":"asset.failed"}`)

	signature, err := signAssetWebhook(secret, "wh_example", "1790580000", payload)
	require.NoError(t, err)

	mac := hmac.New(sha256.New, key)
	_, err = mac.Write([]byte("wh_example.1790580000." + string(payload)))
	require.NoError(t, err)
	assert.Equal(t, "v1,"+base64.StdEncoding.EncodeToString(mac.Sum(nil)), signature)
}

func TestCreateWebhookEndpointReturnsSecretOnceAndStoresItEncrypted(t *testing.T) {
	db := setupAssetLibraryTestDB(t)
	t.Setenv("STORAGE_CREDENTIAL_ENCRYPTION_KEY", "asset-webhook-test-master-secret")

	created, err := CreateWebhookEndpoint(42, WebhookEndpointInput{
		Name:       "production",
		URL:        "https://customer.example.com/webhooks/assets",
		EventTypes: []string{WebhookEventAssetActive, WebhookEventAssetFailed},
	})
	require.NoError(t, err)
	require.NotEmpty(t, created.SigningSecret)

	var stored model.AssetWebhookEndpoint
	require.NoError(t, db.Where("public_id = ?", created.ID).First(&stored).Error)
	assert.NotContains(t, stored.EncryptedSecret, created.SigningSecret)
	assert.Equal(t, model.AssetWebhookEndpointStatusEnabled, stored.Status)

	listed, err := ListWebhookEndpoints(42)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.Equal(t, created.ID, listed[0].ID)
	assert.Empty(t, listed[0].SigningSecret)
	assert.Equal(t, []string{WebhookEventAssetActive, WebhookEventAssetFailed}, listed[0].EventTypes)
}

func TestInitialActivationQueuesWebhookAndPromotesAsset(t *testing.T) {
	db := setupAssetLibraryTestDB(t)
	t.Setenv("STORAGE_CREDENTIAL_ENCRYPTION_KEY", "asset-webhook-activation-test-key")

	_, err := CreateWebhookEndpoint(42, WebhookEndpointInput{
		Name: "production", URL: "https://customer.example.com/webhooks/assets",
		EventTypes: []string{WebhookEventAssetActive},
	})
	require.NoError(t, err)
	group := model.AssetGroup{PublicID: "group-activation", OwnerUserID: 42, Name: "Campaign", Status: model.AssetStatusReady}
	require.NoError(t, db.Create(&group).Error)
	asset := model.MediaAsset{PublicID: "asset-processing", OwnerUserID: 42, GroupID: group.ID, StorageObjectID: 7002, Name: "portrait", AssetType: model.AssetTypeImage, ContentType: "image/png", Size: 10, Status: model.AssetStatusProcessing}
	require.NoError(t, db.Create(&asset).Error)
	replica := model.AssetReplica{AssetID: asset.ID, ChannelID: 9, Operation: model.AssetReplicaOperationSync, Status: model.AssetReplicaStatusSyncing, UpstreamAssetID: "upstream-456", LockedBy: "worker-1", Progress: 90, CreatedAt: common.GetTimestamp()}
	require.NoError(t, db.Create(&replica).Error)

	require.NoError(t, activateReplicaAndQueueWebhook("worker-1", &replica))
	require.NoError(t, db.First(&asset, asset.ID).Error)
	assert.Equal(t, model.AssetStatusReady, asset.Status)
	require.NoError(t, db.First(&replica, replica.ID).Error)
	assert.Equal(t, model.AssetReplicaStatusActive, replica.Status)
	assert.Greater(t, replica.NextSyncAt, common.GetTimestamp())

	var delivery model.AssetWebhookDelivery
	require.NoError(t, db.First(&delivery).Error)
	assert.Equal(t, WebhookEventAssetActive, delivery.EventType)
	assert.Contains(t, delivery.Payload, `"status":"Active"`)
	assert.Contains(t, delivery.Payload, `"state_version":1`)
}

func TestConfirmedDelayedRejectionQueuesWebhookAfterLocalStateChange(t *testing.T) {
	db := setupAssetLibraryTestDB(t)
	t.Setenv("STORAGE_CREDENTIAL_ENCRYPTION_KEY", "asset-webhook-rejection-test-key")

	endpoint, err := CreateWebhookEndpoint(42, WebhookEndpointInput{
		Name:       "production",
		URL:        "https://customer.example.com/webhooks/assets",
		EventTypes: []string{WebhookEventAssetFailed},
	})
	require.NoError(t, err)
	group := model.AssetGroup{PublicID: "group-webhook", OwnerUserID: 42, Name: "Campaign", Status: model.AssetStatusReady}
	require.NoError(t, db.Create(&group).Error)
	asset := model.MediaAsset{PublicID: "asset-delayed-rejection", OwnerUserID: 42, GroupID: group.ID, StorageObjectID: 7001, Name: "portrait", AssetType: model.AssetTypeImage, ContentType: "image/png", Size: 10, Status: model.AssetStatusReady}
	require.NoError(t, db.Create(&asset).Error)
	replica := model.AssetReplica{AssetID: asset.ID, ChannelID: 9, Operation: model.AssetReplicaOperationSync, Status: model.AssetReplicaStatusSyncing, UpstreamAssetID: "upstream-123", LockedBy: "worker-1", Progress: 100}
	require.NoError(t, db.Create(&replica).Error)

	require.NoError(t, rejectReplicaAndQueueWebhook("worker-1", &replica, model.AssetUnavailableSensitiveContent))
	require.NoError(t, db.First(&asset, asset.ID).Error)
	assert.Equal(t, model.AssetStatusUnavailable, asset.Status)
	assert.Equal(t, model.AssetUnavailableSensitiveContent, asset.UnavailableReason)

	var delivery model.AssetWebhookDelivery
	require.NoError(t, db.First(&delivery).Error)
	assert.Equal(t, endpoint.ID, delivery.EndpointPublicID)
	assert.Equal(t, WebhookEventAssetFailed, delivery.EventType)
	assert.Equal(t, model.AssetWebhookDeliveryStatusPending, delivery.Status)
	assert.Contains(t, delivery.Payload, `"id":"asset-delayed-rejection"`)
	assert.Contains(t, delivery.Payload, `"failure_reason":"sensitive_content"`)
	assert.Contains(t, delivery.Payload, `"state_version":2`)
	assert.NotZero(t, delivery.NextAttemptAt)
}

func TestDelayedRejectionSupersedesUndeliveredActiveWebhook(t *testing.T) {
	db := setupAssetLibraryTestDB(t)
	t.Setenv("STORAGE_CREDENTIAL_ENCRYPTION_KEY", "asset-webhook-ordering-test-key")

	_, err := CreateWebhookEndpoint(42, WebhookEndpointInput{
		Name: "production", URL: "https://customer.example.com/webhooks/assets",
		EventTypes: []string{WebhookEventAssetActive, WebhookEventAssetFailed},
	})
	require.NoError(t, err)
	group := model.AssetGroup{PublicID: "group-ordering", OwnerUserID: 42, Name: "Campaign", Status: model.AssetStatusReady}
	require.NoError(t, db.Create(&group).Error)
	asset := model.MediaAsset{
		PublicID: "asset-ordering", OwnerUserID: 42, GroupID: group.ID, StorageObjectID: 7003,
		Name: "portrait", AssetType: model.AssetTypeImage, ContentType: "image/png", Size: 10,
		Status: model.AssetStatusReady,
	}
	require.NoError(t, db.Create(&asset).Error)
	require.NoError(t, queueAssetWebhookDeliveries(db, &asset, WebhookEventAssetActive))
	replica := model.AssetReplica{
		AssetID: asset.ID, ChannelID: 9, Operation: model.AssetReplicaOperationSync,
		Status: model.AssetReplicaStatusSyncing, UpstreamAssetID: "upstream-ordering",
		LockedBy: "worker-ordering", Progress: 100,
	}
	require.NoError(t, db.Create(&replica).Error)

	require.NoError(t, rejectReplicaAndQueueWebhook("worker-ordering", &replica, model.AssetUnavailableSensitiveContent))

	var activeDelivery model.AssetWebhookDelivery
	require.NoError(t, db.Where("event_type = ?", WebhookEventAssetActive).First(&activeDelivery).Error)
	assert.Equal(t, model.AssetWebhookDeliveryStatusSuperseded, activeDelivery.Status)
	assert.Equal(t, int64(0), activeDelivery.NextAttemptAt)

	var failedDelivery model.AssetWebhookDelivery
	require.NoError(t, db.Where("event_type = ?", WebhookEventAssetFailed).First(&failedDelivery).Error)
	assert.Equal(t, model.AssetWebhookDeliveryStatusPending, failedDelivery.Status)
	var activeEvent, failedEvent assetWebhookEvent
	require.NoError(t, common.UnmarshalJsonStr(activeDelivery.Payload, &activeEvent))
	require.NoError(t, common.UnmarshalJsonStr(failedDelivery.Payload, &failedEvent))
	assert.Equal(t, activeEvent.Data.ID, failedEvent.Data.ID)
	assert.Greater(t, failedEvent.Data.StateVersion, activeEvent.Data.StateVersion,
		"receivers must be able to reject late Active events independently of timestamp resolution")
}

func TestActivationCannotReviveTerminalAsset(t *testing.T) {
	for _, status := range []string{model.AssetStatusUnavailable, model.AssetStatusDeleted} {
		t.Run(status, func(t *testing.T) {
			db := setupAssetLibraryTestDB(t)
			asset := model.MediaAsset{PublicID: "asset-terminal", StorageObjectID: 8001, Status: status}
			require.NoError(t, db.Create(&asset).Error)
			replica := model.AssetReplica{AssetID: asset.ID, ChannelID: 1, Status: model.AssetReplicaStatusSyncing, LockedBy: "activation-worker"}
			require.NoError(t, db.Create(&replica).Error)
			require.Error(t, activateReplicaAndQueueWebhook("activation-worker", &replica))
			require.NoError(t, db.First(&asset, asset.ID).Error)
			assert.Equal(t, status, asset.Status)
			require.NoError(t, db.First(&replica, replica.ID).Error)
			assert.Equal(t, model.AssetReplicaStatusSyncing, replica.Status)
			var count int64
			require.NoError(t, db.Model(&model.AssetWebhookDelivery{}).Count(&count).Error)
			assert.Zero(t, count)
		})
	}
}

func TestActiveReplicaRemainsScheduledForDelayedReview(t *testing.T) {
	db := setupAssetLibraryTestDB(t)
	now := common.GetTimestamp()
	replica := model.AssetReplica{
		AssetID: 1, ChannelID: 9, Operation: model.AssetReplicaOperationSync,
		Status: model.AssetReplicaStatusActive, NextSyncAt: now - 1,
	}
	require.NoError(t, db.Create(&replica).Error)

	claimed, err := model.ClaimAssetReplicas(now, now+60, "review-worker", 10)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	assert.Equal(t, replica.ID, claimed[0].ID)
	assert.Equal(t, model.AssetReplicaStatusActive, claimed[0].Status)
	assert.Equal(t, model.AssetReplicaStatusActive, claimed[0].ClaimedFromStatus)

	var stored model.AssetReplica
	require.NoError(t, db.First(&stored, replica.ID).Error)
	assert.Equal(t, model.AssetReplicaStatusActive, stored.Status)
	assert.Equal(t, "review-worker", stored.LockedBy)

	claimedAgain, err := model.ClaimAssetReplicas(now, now+60, "other-worker", 10)
	require.NoError(t, err)
	assert.Empty(t, claimedAgain)
}
