package assetlibrary

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

const (
	WebhookEventAssetActive = model.WebhookEventAssetActive
	WebhookEventAssetFailed = model.WebhookEventAssetFailed
)

type assetWebhookEvent struct {
	ID        string                `json:"id"`
	Object    string                `json:"object"`
	Type      string                `json:"type"`
	CreatedAt int64                 `json:"created_at"`
	Data      assetWebhookEventData `json:"data"`
}

type assetWebhookEventData struct {
	ID            string `json:"id"`
	GroupID       string `json:"group_id,omitempty"`
	Status        string `json:"status,omitempty"`
	FailureReason string `json:"failure_reason,omitempty"`
	UpdatedAt     int64  `json:"updated_at,omitempty"`
	// Asset IDs are never reused and failure is terminal. This ordering is
	// independent of second-resolution timestamps and HTTP arrival order.
	StateVersion int `json:"state_version,omitempty"`
}

func queueAssetWebhookDeliveries(tx *gorm.DB, asset *model.MediaAsset, eventType string) error {
	if asset == nil {
		return errors.New("asset is required for webhook delivery")
	}
	if eventType == WebhookEventAssetFailed {
		if err := tx.Model(&model.AssetWebhookDelivery{}).
			Where("asset_id = ? AND event_type = ? AND status IN ?", asset.ID, WebhookEventAssetActive, []string{
				model.AssetWebhookDeliveryStatusPending,
				model.AssetWebhookDeliveryStatusFailed,
			}).
			Updates(map[string]any{
				"status":          model.AssetWebhookDeliveryStatusSuperseded,
				"next_attempt_at": 0,
				"last_error":      "superseded by asset.failed",
				"updated_at":      common.GetTimestamp(),
			}).Error; err != nil {
			return err
		}
	}
	var group model.AssetGroup
	if err := tx.Select("public_id").First(&group, asset.GroupID).Error; err != nil {
		return err
	}
	status := "Active"
	stateVersion := 1
	if eventType == WebhookEventAssetFailed {
		status = "Failed"
		stateVersion = 2
	}
	_, err := model.QueueWebhookEvent(tx, asset.OwnerUserID, eventType, asset.ID, assetWebhookEventData{
		ID: asset.PublicID, GroupID: group.PublicID, Status: status,
		FailureReason: asset.UnavailableReason, UpdatedAt: asset.UpdatedAt, StateVersion: stateVersion,
	})
	return err
}
