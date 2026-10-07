package service

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/asyncwebhook"
)

type WebhookProbeResult struct {
	Reachable  bool   `json:"reachable"`
	HTTPStatus int    `json:"http_status"`
	DurationMS int64  `json:"duration_ms"`
	EventID    string `json:"event_id"`
	WebhookID  string `json:"webhook_id"`
	Error      string `json:"error,omitempty"`
}

func ProbeUserWebhook(ctx context.Context, owner int, publicID string, legacy bool) (*WebhookProbeResult, error) {
	var endpoint model.AssetWebhookEndpoint
	if err := model.DB.WithContext(ctx).Where("owner_user_id = ? AND public_id = ? AND status = ?", owner, publicID, model.AssetWebhookEndpointStatusEnabled).First(&endpoint).Error; err != nil {
		return nil, err
	}
	var events []string
	if err := common.UnmarshalJsonStr(endpoint.EventTypes, &events); err != nil {
		return nil, err
	}
	if legacy {
		events = append(events, model.WebhookEventAssetActive)
	}
	if err := asyncwebhook.RequireWebhookEventsEnabled(model.DB.WithContext(ctx), events); err != nil {
		return nil, err
	}
	eventType := "webhook.test"
	if legacy {
		eventType = "asset.webhook_test"
	}
	return probeWebhook(ctx, endpoint.URL, endpoint.PublicID, eventType, owner)
}

func ProbeSystemWebhook(ctx context.Context, topic, callbackURL string) (*WebhookProbeResult, error) {
	var id string
	switch topic {
	case "asset_library":
		id = model.SystemAssetWebhookEndpointID
	case "media_tasks":
		id = model.SystemTaskWebhookEndpointID
	default:
		return nil, &asyncwebhook.RequestError{StatusCode: http.StatusBadRequest, Err: errors.New("unsupported system webhook topic")}
	}
	url, err := asyncwebhook.ValidateCallbackURL(callbackURL)
	if err != nil {
		return nil, err
	}
	return probeWebhook(ctx, url, id, "webhook.test", 0)
}

func probeWebhook(ctx context.Context, callbackURL, endpointID, eventType string, owner int) (*WebhookProbeResult, error) {
	eventID, err := model.NewWebhookValue("evt_", 24)
	if err != nil {
		return nil, err
	}
	webhookID, err := model.NewWebhookValue("wh_", 24)
	if err != nil {
		return nil, err
	}
	payload, err := common.Marshal(model.WebhookEvent{ID: eventID, Object: "event", Type: eventType, CreatedAt: common.GetTimestamp(), OwnerUserID: owner,
		Data: map[string]any{"id": endpointID, "status": "test", "updated_at": common.GetTimestamp()}})
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	started := time.Now()
	status, deliveryErr := SendWebhookRequest(ctx, callbackURL, string(payload), webhookID)
	result := &WebhookProbeResult{Reachable: deliveryErr == nil, HTTPStatus: status, DurationMS: time.Since(started).Milliseconds(), EventID: eventID, WebhookID: webhookID}
	if deliveryErr != nil {
		// Do not echo a URL (which may contain a customer token) or response body.
		result.Error = "webhook request failed; check callback URL, network policy and service availability"
		if status > 0 {
			result.Error = "callback did not return a complete HTTP 2xx response"
		}
	}
	return result, nil
}
