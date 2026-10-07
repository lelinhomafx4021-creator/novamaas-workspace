package asyncwebhook

import (
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
	webhookEventTest        = "webhook.test"

	MaxWebhookEndpointsPerUser = 5
)

var supportedWebhookEvents = map[string]struct{}{
	WebhookEventAssetActive:             {},
	WebhookEventAssetFailed:             {},
	model.WebhookEventTaskStatusChanged: {},
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
	publicID, err := model.NewWebhookValue("we_", 18)
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
	if err = model.CreateAssetWebhookEndpointWithinLimit(&endpoint, MaxWebhookEndpointsPerUser); err != nil {
		if errors.Is(err, model.ErrAssetWebhookEndpointLimit) {
			return nil, &RequestError{StatusCode: http.StatusConflict, Err: errors.New("a user can have at most 5 active webhook endpoints")}
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

func UpdateWebhookEndpoint(ownerUserID int, publicID string, input WebhookEndpointInput) (*WebhookEndpointView, error) {
	name, callbackURL, eventTypes, err := validateWebhookEndpointInput(ownerUserID, input)
	if err != nil {
		return nil, err
	}
	encoded, err := common.Marshal(eventTypes)
	if err != nil {
		return nil, err
	}
	var endpoint model.AssetWebhookEndpoint
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("public_id = ? AND owner_user_id = ? AND status = ?", strings.TrimSpace(publicID), ownerUserID, model.AssetWebhookEndpointStatusEnabled).First(&endpoint).Error; err != nil {
			return err
		}
		endpoint.Name, endpoint.URL, endpoint.EventTypes, endpoint.UpdatedAt = name, callbackURL, string(encoded), common.GetTimestamp()
		return tx.Model(&endpoint).Select("name", "url", "event_types", "updated_at").Updates(&endpoint).Error
	})
	if err != nil {
		return nil, err
	}
	view, err := webhookEndpointView(endpoint)
	return &view, err
}

func QueueWebhookEndpointTest(ownerUserID int, publicID string) (string, error) {
	return queueWebhookEndpointTest(ownerUserID, publicID, webhookEventTest)
}

// QueueAssetWebhookEndpointTest preserves the legacy API's test event type.
func QueueAssetWebhookEndpointTest(ownerUserID int, publicID string) (string, error) {
	return queueWebhookEndpointTest(ownerUserID, publicID, "asset.webhook_test")
}

func queueWebhookEndpointTest(ownerUserID int, publicID, eventType string) (string, error) {
	var endpoint model.AssetWebhookEndpoint
	if err := model.DB.Where("public_id = ? AND owner_user_id = ? AND status = ?", strings.TrimSpace(publicID), ownerUserID, model.AssetWebhookEndpointStatusEnabled).
		First(&endpoint).Error; err != nil {
		return "", err
	}
	var events []string
	if err := common.UnmarshalJsonStr(endpoint.EventTypes, &events); err != nil {
		return "", err
	}
	if eventType == "asset.webhook_test" {
		events = append(events, model.WebhookEventAssetActive)
	}
	if err := RequireWebhookEventsEnabled(model.DB, events); err != nil {
		return "", err
	}
	now := common.GetTimestamp()
	eventID, err := model.NewWebhookValue("evt_", 24)
	if err != nil {
		return "", err
	}
	payload, err := common.Marshal(model.WebhookEvent{
		ID: eventID, Object: "event", Type: eventType, CreatedAt: now,
		Data: map[string]any{"id": endpoint.PublicID, "status": "test", "updated_at": now},
	})
	if err != nil {
		return "", err
	}
	delivery, err := model.NewWebhookDelivery(endpoint, eventID, eventType, 0, payload, now)
	if err != nil {
		return "", err
	}
	return eventID, model.DB.Create(delivery).Error
}

func validateWebhookEndpointInput(ownerUserID int, input WebhookEndpointInput) (string, string, []string, error) {
	if ownerUserID <= 0 {
		return "", "", nil, &RequestError{StatusCode: http.StatusUnauthorized, Err: errors.New("authentication is required")}
	}
	name := strings.TrimSpace(input.Name)
	if name == "" || len([]rune(name)) > 64 {
		return "", "", nil, &RequestError{StatusCode: http.StatusBadRequest, Err: errors.New("webhook endpoint name must contain 1 to 64 characters")}
	}
	callbackURL, err := ValidateCallbackURL(input.URL)
	if err != nil {
		return "", "", nil, err
	}
	eventSet := make(map[string]struct{}, len(input.EventTypes))
	for _, value := range input.EventTypes {
		eventType := strings.TrimSpace(value)
		if _, ok := supportedWebhookEvents[eventType]; !ok {
			return "", "", nil, &RequestError{StatusCode: http.StatusBadRequest, Err: errors.New("unsupported webhook event type")}
		}
		eventSet[eventType] = struct{}{}
	}
	if len(eventSet) == 0 {
		return "", "", nil, &RequestError{StatusCode: http.StatusBadRequest, Err: errors.New("select at least one webhook event type")}
	}
	eventTypes := make([]string, 0, len(eventSet))
	for eventType := range eventSet {
		eventTypes = append(eventTypes, eventType)
	}
	sort.Strings(eventTypes)
	if err := RequireWebhookEventsEnabled(model.DB, eventTypes); err != nil {
		return "", "", nil, err
	}
	return name, callbackURL, eventTypes, nil
}

func RequireWebhookEventsEnabled(db *gorm.DB, events []string) error {
	availability, err := model.GetWebhookFeatureAvailability(db)
	if err != nil {
		return err
	}
	if len(events) == 0 {
		return &RequestError{StatusCode: http.StatusForbidden, Err: errors.New("webhook categories are disabled by the administrator")}
	}
	for _, event := range events {
		if !availability.AllowsEvent(event) {
			return &RequestError{StatusCode: http.StatusForbidden, Err: errors.New("webhook category is disabled by the administrator")}
		}
	}
	return nil
}

func ValidateCallbackURL(value string) (string, error) {
	callbackURL := strings.TrimSpace(value)
	parsedURL, err := url.Parse(callbackURL)
	if err != nil || parsedURL.Scheme != "https" || parsedURL.Host == "" || parsedURL.User != nil || parsedURL.Fragment != "" || len(callbackURL) > 2048 {
		return "", &RequestError{StatusCode: http.StatusBadRequest, Err: errors.New("webhook endpoint must be an absolute HTTPS URL without credentials or fragments")}
	}
	return parsedURL.String(), nil
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

type RequestError struct {
	StatusCode int
	Err        error
}

func (err *RequestError) Error() string       { return err.Err.Error() }
func (err *RequestError) Unwrap() error       { return err.Err }
func (err *RequestError) HTTPStatusCode() int { return err.StatusCode }
