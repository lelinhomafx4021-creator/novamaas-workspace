package assetlibrary

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"gorm.io/gorm"
)

const (
	WebhookEventAssetActive = "asset.active"
	WebhookEventAssetFailed = "asset.failed"
	webhookEventTest        = "asset.webhook_test"

	maxWebhookEndpointsPerUser = 5
)

var supportedAssetWebhookEvents = map[string]struct{}{
	WebhookEventAssetActive: {},
	WebhookEventAssetFailed: {},
}

type WebhookEndpointInput struct {
	Name       string   `json:"name"`
	URL        string   `json:"url"`
	EventTypes []string `json:"event_types"`
}

type WebhookEndpointView struct {
	ID         string   `json:"id"`
	Object     string   `json:"object"`
	Name       string   `json:"name"`
	URL        string   `json:"url"`
	EventTypes []string `json:"event_types"`
	Status     string   `json:"status"`
	CreatedAt  int64    `json:"created_at"`
	UpdatedAt  int64    `json:"updated_at"`
}

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

func ListWebhookEndpoints(ownerUserID int) ([]WebhookEndpointView, error) {
	var endpoints []model.AssetWebhookEndpoint
	if err := model.DB.Where("owner_user_id = ? AND status = ?", ownerUserID, model.AssetWebhookEndpointStatusEnabled).
		Order("created_at desc").Find(&endpoints).Error; err != nil {
		return nil, err
	}
	views := make([]WebhookEndpointView, 0, len(endpoints))
	for _, endpoint := range endpoints {
		view, err := webhookEndpointView(endpoint)
		if err != nil {
			return nil, err
		}
		views = append(views, view)
	}
	return views, nil
}

func CreateWebhookEndpoint(ownerUserID int, input WebhookEndpointInput) (*WebhookEndpointView, error) {
	name, callbackURL, eventTypes, err := validateWebhookEndpointInput(ownerUserID, input)
	if err != nil {
		return nil, err
	}
	publicID, err := randomWebhookValue("we_", 18)
	if err != nil {
		return nil, err
	}
	encodedEventTypes, err := common.Marshal(eventTypes)
	if err != nil {
		return nil, err
	}
	endpoint := model.AssetWebhookEndpoint{
		PublicID: publicID, OwnerUserID: ownerUserID, Name: name, URL: callbackURL,
		EventTypes: string(encodedEventTypes),
		Status:     model.AssetWebhookEndpointStatusEnabled,
	}
	if err = model.CreateAssetWebhookEndpointWithinLimit(&endpoint, maxWebhookEndpointsPerUser); err != nil {
		if errors.Is(err, model.ErrAssetWebhookEndpointLimit) {
			return nil, &RequestError{StatusCode: http.StatusConflict, Err: errors.New("a user can have at most 5 active asset webhook endpoints")}
		}
		return nil, err
	}
	view, err := webhookEndpointView(endpoint)
	if err != nil {
		return nil, err
	}
	return &view, nil
}

func DeleteWebhookEndpoint(ownerUserID int, publicID string) error {
	result := model.DB.Model(&model.AssetWebhookEndpoint{}).
		Where("public_id = ? AND owner_user_id = ? AND status = ?", strings.TrimSpace(publicID), ownerUserID, model.AssetWebhookEndpointStatusEnabled).
		Updates(map[string]any{"status": model.AssetWebhookEndpointStatusDisabled, "updated_at": common.GetTimestamp()})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func QueueWebhookEndpointTest(ownerUserID int, publicID string) (string, error) {
	var endpoint model.AssetWebhookEndpoint
	if err := model.DB.Where("public_id = ? AND owner_user_id = ? AND status = ?", strings.TrimSpace(publicID), ownerUserID, model.AssetWebhookEndpointStatusEnabled).
		First(&endpoint).Error; err != nil {
		return "", err
	}
	now := common.GetTimestamp()
	eventID, err := randomWebhookValue("evt_", 24)
	if err != nil {
		return "", err
	}
	payload, err := common.Marshal(assetWebhookEvent{
		ID: eventID, Object: "event", Type: webhookEventTest, CreatedAt: now,
		Data: assetWebhookEventData{ID: endpoint.PublicID, Status: "test", UpdatedAt: now},
	})
	if err != nil {
		return "", err
	}
	delivery, err := newAssetWebhookDelivery(endpoint, eventID, webhookEventTest, 0, payload, now)
	if err != nil {
		return "", err
	}
	return eventID, model.DB.Create(delivery).Error
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
	var endpoints []model.AssetWebhookEndpoint
	if err := tx.Where("owner_user_id = ? AND status = ?", asset.OwnerUserID, model.AssetWebhookEndpointStatusEnabled).
		Find(&endpoints).Error; err != nil {
		return err
	}
	if len(endpoints) == 0 {
		return nil
	}
	var group model.AssetGroup
	if err := tx.Select("public_id").First(&group, asset.GroupID).Error; err != nil {
		return err
	}
	now := common.GetTimestamp()
	eventID, err := randomWebhookValue("evt_", 24)
	if err != nil {
		return err
	}
	status := "Active"
	stateVersion := 1
	if eventType == WebhookEventAssetFailed {
		status = "Failed"
		stateVersion = 2
	}
	payload, err := common.Marshal(assetWebhookEvent{
		ID: eventID, Object: "event", Type: eventType, CreatedAt: now,
		Data: assetWebhookEventData{
			ID: asset.PublicID, GroupID: group.PublicID, Status: status,
			FailureReason: asset.UnavailableReason, UpdatedAt: asset.UpdatedAt,
			StateVersion: stateVersion,
		},
	})
	if err != nil {
		return err
	}
	for _, endpoint := range endpoints {
		subscribed, err := endpointSubscribes(endpoint, eventType)
		if err != nil {
			return err
		}
		if !subscribed {
			continue
		}
		delivery, err := newAssetWebhookDelivery(endpoint, eventID, eventType, asset.ID, payload, now)
		if err != nil {
			return err
		}
		if err = tx.Create(delivery).Error; err != nil {
			return err
		}
	}
	return nil
}

func newAssetWebhookDelivery(endpoint model.AssetWebhookEndpoint, eventID string, eventType string, assetID int64, payload []byte, now int64) (*model.AssetWebhookDelivery, error) {
	webhookID, err := randomWebhookValue("wh_", 24)
	if err != nil {
		return nil, err
	}
	return &model.AssetWebhookDelivery{
		EventID: eventID, WebhookID: webhookID, EndpointID: endpoint.ID, EndpointPublicID: endpoint.PublicID,
		OwnerUserID: endpoint.OwnerUserID, AssetID: assetID, EventType: eventType, Payload: string(payload),
		Status: model.AssetWebhookDeliveryStatusPending, NextAttemptAt: now,
	}, nil
}

func validateWebhookEndpointInput(ownerUserID int, input WebhookEndpointInput) (string, string, []string, error) {
	if ownerUserID <= 0 {
		return "", "", nil, &RequestError{StatusCode: http.StatusUnauthorized, Err: errors.New("authentication is required")}
	}
	name := strings.TrimSpace(input.Name)
	if name == "" || len([]rune(name)) > 64 {
		return "", "", nil, &RequestError{StatusCode: http.StatusBadRequest, Err: errors.New("webhook endpoint name must contain 1 to 64 characters")}
	}
	callbackURL := strings.TrimSpace(input.URL)
	parsedURL, err := url.Parse(callbackURL)
	if err != nil || parsedURL.Scheme != "https" || parsedURL.Host == "" || parsedURL.User != nil || parsedURL.Fragment != "" || len(callbackURL) > 2048 {
		return "", "", nil, &RequestError{StatusCode: http.StatusBadRequest, Err: errors.New("webhook endpoint must be an absolute HTTPS URL without credentials or fragments")}
	}
	eventSet := make(map[string]struct{}, len(input.EventTypes))
	for _, value := range input.EventTypes {
		eventType := strings.TrimSpace(value)
		if _, ok := supportedAssetWebhookEvents[eventType]; !ok {
			return "", "", nil, &RequestError{StatusCode: http.StatusBadRequest, Err: errors.New("unsupported asset webhook event type")}
		}
		eventSet[eventType] = struct{}{}
	}
	if len(eventSet) == 0 {
		return "", "", nil, &RequestError{StatusCode: http.StatusBadRequest, Err: errors.New("select at least one asset webhook event type")}
	}
	eventTypes := make([]string, 0, len(eventSet))
	for eventType := range eventSet {
		eventTypes = append(eventTypes, eventType)
	}
	sort.Strings(eventTypes)
	return name, parsedURL.String(), eventTypes, nil
}

func webhookEndpointView(endpoint model.AssetWebhookEndpoint) (WebhookEndpointView, error) {
	var eventTypes []string
	if err := common.UnmarshalJsonStr(endpoint.EventTypes, &eventTypes); err != nil {
		return WebhookEndpointView{}, err
	}
	return WebhookEndpointView{
		ID: endpoint.PublicID, Object: "webhook_endpoint", Name: endpoint.Name, URL: endpoint.URL,
		EventTypes: eventTypes, Status: endpoint.Status,
		CreatedAt: endpoint.CreatedAt, UpdatedAt: endpoint.UpdatedAt,
	}, nil
}

func endpointSubscribes(endpoint model.AssetWebhookEndpoint, eventType string) (bool, error) {
	var eventTypes []string
	if err := common.UnmarshalJsonStr(endpoint.EventTypes, &eventTypes); err != nil {
		return false, err
	}
	for _, value := range eventTypes {
		if value == eventType {
			return true, nil
		}
	}
	return false, nil
}

func randomWebhookValue(prefix string, bytesLength int) (string, error) {
	value := make([]byte, bytesLength)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return prefix + base64.RawURLEncoding.EncodeToString(value), nil
}
