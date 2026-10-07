package model

import (
	"crypto/rand"
	"encoding/base64"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

type WebhookFeatureAvailability struct {
	AssetLibraryEnabled bool `json:"asset_library_enabled"`
	MediaTasksEnabled   bool `json:"media_tasks_enabled"`
}

func GetWebhookFeatureAvailability(db *gorm.DB) (*WebhookFeatureAvailability, error) {
	var endpoints []AssetWebhookEndpoint
	if err := db.Select("public_id", "status").Where("owner_user_id = 0 AND public_id IN ?", []string{SystemAssetWebhookEndpointID, SystemTaskWebhookEndpointID}).Find(&endpoints).Error; err != nil {
		return nil, err
	}
	availability := &WebhookFeatureAvailability{}
	for _, endpoint := range endpoints {
		if endpoint.PublicID == SystemAssetWebhookEndpointID {
			availability.AssetLibraryEnabled = endpoint.Status == AssetWebhookEndpointStatusEnabled
		} else {
			availability.MediaTasksEnabled = endpoint.Status == AssetWebhookEndpointStatusEnabled
		}
	}
	return availability, nil
}

func (availability WebhookFeatureAvailability) AllowsEvent(eventType string) bool {
	switch eventType {
	case WebhookEventAssetActive, WebhookEventAssetFailed:
		return availability.AssetLibraryEnabled
	case WebhookEventTaskStatusChanged:
		return availability.MediaTasksEnabled
	default:
		return false
	}
}

const (
	WebhookEventAssetActive       = "asset.active"
	WebhookEventAssetFailed       = "asset.failed"
	WebhookEventTaskStatusChanged = "task.status_changed"
	SystemAssetWebhookEndpointID  = "we_system_assets"
	SystemTaskWebhookEndpointID   = "we_system_tasks"
)

// WebhookEvent is the shared envelope for independently produced domain events.
type WebhookEvent struct {
	ID          string `json:"id"`
	Object      string `json:"object"`
	Type        string `json:"type"`
	CreatedAt   int64  `json:"created_at"`
	OwnerUserID int    `json:"owner_user_id,omitempty"`
	Data        any    `json:"data"`
}

func NewWebhookValue(prefix string, bytesLength int) (string, error) {
	value := make([]byte, bytesLength)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return prefix + base64.RawURLEncoding.EncodeToString(value), nil
}

func NewWebhookDelivery(endpoint AssetWebhookEndpoint, eventID, eventType string, assetID int64, payload []byte, now int64) (*AssetWebhookDelivery, error) {
	webhookID, err := NewWebhookValue("wh_", 24)
	if err != nil {
		return nil, err
	}
	return &AssetWebhookDelivery{
		EventID: eventID, WebhookID: webhookID, EndpointID: endpoint.ID, EndpointPublicID: endpoint.PublicID,
		OwnerUserID: endpoint.OwnerUserID, AssetID: assetID, EventType: eventType, Payload: string(payload),
		Status: AssetWebhookDeliveryStatusPending, NextAttemptAt: now,
	}, nil
}

// QueueWebhookEvent writes an immutable outbox in the caller's state transaction.
// The existing asset webhook tables remain the shared store to preserve endpoints
// and pending deliveries across upgrades. AssetID is only used by asset ordering.
func QueueWebhookEvent(tx *gorm.DB, ownerUserID int, eventType string, assetID int64, data any) (string, error) {
	availability, err := GetWebhookFeatureAvailability(tx)
	if err != nil {
		return "", err
	}
	if !availability.AllowsEvent(eventType) {
		return "", nil
	}
	var endpoints []AssetWebhookEndpoint
	systemEndpointID := ""
	switch eventType {
	case WebhookEventAssetActive, WebhookEventAssetFailed:
		systemEndpointID = SystemAssetWebhookEndpointID
	case WebhookEventTaskStatusChanged:
		systemEndpointID = SystemTaskWebhookEndpointID
	}
	if err := tx.Where("status = ? AND (owner_user_id = ? OR (owner_user_id = 0 AND public_id = ?))", AssetWebhookEndpointStatusEnabled, ownerUserID, systemEndpointID).Find(&endpoints).Error; err != nil {
		return "", err
	}
	subscribers := make([]AssetWebhookEndpoint, 0, len(endpoints))
	for _, endpoint := range endpoints {
		if endpoint.OwnerUserID == 0 && endpoint.URL == "" {
			continue
		}
		var events []string
		if err := common.UnmarshalJsonStr(endpoint.EventTypes, &events); err != nil {
			return "", err
		}
		for _, event := range events {
			if event == eventType {
				subscribers = append(subscribers, endpoint)
				break
			}
		}
	}
	if len(subscribers) == 0 {
		return "", nil
	}
	eventID, err := NewWebhookValue("evt_", 24)
	if err != nil {
		return "", err
	}
	now := common.GetTimestamp()
	payload, err := common.Marshal(WebhookEvent{ID: eventID, Object: "event", Type: eventType, CreatedAt: now, OwnerUserID: ownerUserID, Data: data})
	if err != nil {
		return "", err
	}
	for _, endpoint := range subscribers {
		delivery, err := NewWebhookDelivery(endpoint, eventID, eventType, assetID, payload, now)
		if err != nil {
			return "", err
		}
		if err := tx.Create(delivery).Error; err != nil {
			return "", err
		}
	}
	return eventID, nil
}
