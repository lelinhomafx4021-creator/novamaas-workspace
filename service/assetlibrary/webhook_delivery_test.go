package assetlibrary

import (
	"context"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	coreService "github.com/QuantumNous/new-api/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	cutoff := now - int64((30*24*time.Hour)/time.Second)
	deleted, err := model.PurgeAssetWebhookDeliveriesBefore(context.Background(),
		[]string{model.AssetWebhookDeliveryStatusSucceeded, model.AssetWebhookDeliveryStatusSuperseded}, cutoff, 2)
	require.NoError(t, err)
	assert.EqualValues(t, 2, deleted)
	assert.Error(t, func() error {
		_, err := model.PurgeAssetWebhookDeliveriesBefore(context.Background(),
			[]string{model.AssetWebhookDeliveryStatusPending}, cutoff, 2)
		return err
	}())

	coreService.PurgeOldWebhookDeliveries()
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
