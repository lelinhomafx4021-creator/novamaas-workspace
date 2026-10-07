package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/bytedance/gopkg/util/gopool"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"gorm.io/gorm"
)

const (
	asyncWebhookDeliveryLease  = time.Minute
	asyncWebhookLeaseMargin    = 5 * time.Second
	asyncWebhookDeliveryBatch  = 20
	asyncWebhookDeliveryMaxAge = 72 * time.Hour
)

func RunWebhookDeliveryPass(runnerID string) {
	for range asyncWebhookDeliveryBatch {
		// Acquire the lease immediately before sending, not while earlier
		// callbacks are still using up the batch's lease time.
		now := common.GetTimestamp()
		attemptID := runnerID + "-" + common.GetRandomString(16)
		deliveries, err := model.ClaimAssetWebhookDeliveries(now, now+int64(asyncWebhookDeliveryLease/time.Second), attemptID, 1)
		if err != nil {
			logger.LogWarn(context.Background(), fmt.Sprintf("claim webhook deliveries failed: %v", err))
			return
		}
		if len(deliveries) == 0 {
			return
		}
		deliverWebhook(attemptID, deliveries[0])
	}
}

func deliverWebhook(runnerID string, delivery *model.AssetWebhookDelivery) {
	// Include database lookups in the lease budget. A paused worker must not
	// resume with a fresh HTTP timeout after another node has taken over.
	leaseDeadline := time.Unix(delivery.LeaseUntil, 0).Add(-asyncWebhookLeaseMargin)
	if !time.Now().Before(leaseDeadline) {
		return
	}
	leaseCtx, cancelLease := context.WithDeadline(context.Background(), leaseDeadline)
	defer cancelLease()
	deadline := time.Unix(delivery.CreatedAt, 0).Add(asyncWebhookDeliveryMaxAge)
	if !time.Now().Before(deadline) {
		finishWebhookDeliveryFailure(leaseCtx, runnerID, delivery, 0, errors.New("webhook delivery expired after 72 hours"), true)
		return
	}
	if delivery.EventType == model.WebhookEventAssetActive && delivery.AssetID > 0 {
		var asset model.MediaAsset
		err := model.DB.WithContext(leaseCtx).Select("status").First(&asset, delivery.AssetID).Error
		if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && asset.Status != model.AssetStatusReady) {
			if finishErr := model.SupersedeAssetWebhookDelivery(leaseCtx, delivery.ID, runnerID, "superseded by current asset status"); finishErr != nil {
				logger.LogWarn(context.Background(), fmt.Sprintf("supersede stale webhook delivery failed: delivery_id=%d error=%v", delivery.ID, finishErr))
			}
			return
		}
		if err != nil {
			finishWebhookDeliveryFailure(leaseCtx, runnerID, delivery, 0, err, false)
			return
		}
	}
	var endpoint model.AssetWebhookEndpoint
	if err := model.DB.WithContext(leaseCtx).First(&endpoint, delivery.EndpointID).Error; err != nil {
		finishWebhookDeliveryFailure(leaseCtx, runnerID, delivery, 0, err, errors.Is(err, gorm.ErrRecordNotFound))
		return
	}
	if endpoint.Status != model.AssetWebhookEndpointStatusEnabled || endpoint.URL == "" {
		finishWebhookDeliveryFailure(leaseCtx, runnerID, delivery, 0, errors.New("webhook endpoint is disabled"), true)
		return
	}
	availability, err := model.GetWebhookFeatureAvailability(model.DB.WithContext(leaseCtx))
	if err != nil {
		finishWebhookDeliveryFailure(leaseCtx, runnerID, delivery, 0, err, false)
		return
	}
	events := []string{delivery.EventType}
	if delivery.EventType == "webhook.test" || delivery.EventType == "asset.webhook_test" {
		if err := common.UnmarshalJsonStr(endpoint.EventTypes, &events); err != nil {
			finishWebhookDeliveryFailure(leaseCtx, runnerID, delivery, 0, err, true)
			return
		}
		if delivery.EventType == "asset.webhook_test" {
			events = append(events, model.WebhookEventAssetActive)
		}
	}
	allowed := len(events) > 0
	for _, event := range events {
		if !availability.AllowsEvent(event) {
			allowed = false
			break
		}
	}
	if !allowed {
		finishWebhookDeliveryFailure(leaseCtx, runnerID, delivery, 0, errors.New("webhook category is disabled by the administrator"), true)
		return
	}
	owned, err := model.OwnsAssetWebhookDeliveryLease(leaseCtx, delivery.ID, runnerID, common.GetTimestamp())
	if err != nil {
		finishWebhookDeliveryFailure(leaseCtx, runnerID, delivery, 0, err, false)
		return
	}
	if !owned {
		return
	}
	if requestDeadline := time.Now().Add(10 * time.Second); requestDeadline.Before(deadline) {
		deadline = requestDeadline
	}
	ctx, cancel := context.WithDeadline(leaseCtx, deadline)
	defer cancel()
	responseStatus, err := SendWebhookRequest(ctx, endpoint.URL, delivery.Payload, delivery.WebhookID)
	if err != nil {
		finishWebhookDeliveryFailure(leaseCtx, runnerID, delivery, responseStatus, err, false)
		return
	}
	if err = model.FinishAssetWebhookDelivery(leaseCtx, delivery.ID, runnerID, model.AssetWebhookDeliveryStatusSucceeded, responseStatus, 0, ""); err != nil {
		logger.LogWarn(context.Background(), fmt.Sprintf("finish webhook delivery failed: delivery_id=%d error=%v", delivery.ID, err))
	}
}

// SendWebhookRequest applies the same timeout-independent transport policy to
// durable deliveries and caller-bounded connectivity probes.
func SendWebhookRequest(ctx context.Context, callbackURL, payload, webhookID string) (int, error) {
	headers := map[string]string{
		"Content-Type": "application/json", "User-Agent": "New-API/1.0",
		"webhook-id": webhookID, "webhook-timestamp": strconv.FormatInt(time.Now().Unix(), 10),
	}
	var response *http.Response
	var err error
	if system_setting.EnableWorker() {
		response, err = DoWorkerRequestWithContext(ctx, &WorkerRequest{
			URL: callbackURL, Key: system_setting.WorkerValidKey, Method: http.MethodPost, Headers: headers, Body: []byte(payload),
		})
	} else {
		if err = ValidateSSRFProtectedFetchURLWithContext(ctx, callbackURL); err != nil {
			return 0, fmt.Errorf("webhook URL rejected: %w", err)
		}
		request, requestErr := http.NewRequestWithContext(ctx, http.MethodPost, callbackURL, bytes.NewBufferString(payload))
		if requestErr != nil {
			return 0, requestErr
		}
		for key, value := range headers {
			request.Header.Set(key, value)
		}
		client := *GetSSRFProtectedHTTPClient()
		client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
		response, err = client.Do(request)
	}
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	if _, err = io.Copy(io.Discard, io.LimitReader(response.Body, 64*1024)); err != nil {
		return response.StatusCode, err
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return response.StatusCode, fmt.Errorf("webhook endpoint returned HTTP %d", response.StatusCode)
	}
	return response.StatusCode, nil
}

func finishWebhookDeliveryFailure(ctx context.Context, runnerID string, delivery *model.AssetWebhookDelivery, responseStatus int, deliveryErr error, permanent bool) {
	now := common.GetTimestamp()
	message := deliveryErr.Error()
	if len(message) > 2000 {
		message = message[:2000]
	}
	status := model.AssetWebhookDeliveryStatusFailed
	nextAttemptAt := now + int64(asyncWebhookRetryDelay(delivery.Attempts)/time.Second)
	nextAttemptAt = min(nextAttemptAt, delivery.CreatedAt+int64(asyncWebhookDeliveryMaxAge/time.Second))
	if permanent || now-delivery.CreatedAt >= int64(asyncWebhookDeliveryMaxAge/time.Second) {
		status = model.AssetWebhookDeliveryStatusExhausted
		nextAttemptAt = 0
	}
	if err := model.FinishAssetWebhookDelivery(ctx, delivery.ID, runnerID, status, responseStatus, nextAttemptAt, message); err != nil {
		logger.LogWarn(context.Background(), fmt.Sprintf("record webhook delivery failure: delivery_id=%d error=%v", delivery.ID, err))
		return
	}
	logger.LogWarn(context.Background(), fmt.Sprintf("webhook delivery failed: delivery_id=%d endpoint_id=%s error=%v", delivery.ID, delivery.EndpointPublicID, deliveryErr))
}

func asyncWebhookRetryDelay(attempts int) time.Duration {
	if attempts < 0 {
		attempts = 0
	}
	delay := time.Minute * time.Duration(1<<min(attempts, 10))
	if delay > 12*time.Hour {
		return 12 * time.Hour
	}
	return delay
}

var webhookDeliveryOnce sync.Once

// StartWebhookDeliveryTask serves all event producers independently of asset polling.
func StartWebhookDeliveryTask() {
	webhookDeliveryOnce.Do(func() {
		for lane := range 4 {
			runnerID := fmt.Sprintf("%s-webhooks-%s-%d", common.NodeName, common.GetRandomString(8), lane)
			gopool.Go(func() {
				ticker := time.NewTicker(2 * time.Second)
				defer ticker.Stop()
				for {
					RunWebhookDeliveryPass(runnerID)
					<-ticker.C
				}
			})
		}
		if common.IsMasterNode {
			gopool.Go(func() {
				ticker := time.NewTicker(time.Hour)
				defer ticker.Stop()
				for {
					PurgeOldWebhookDeliveries()
					<-ticker.C
				}
			})
		}
	})
}

const (
	asyncWebhookRetentionBatch   = 500
	asyncWebhookRetentionPasses  = 10
	asyncWebhookSuccessRetention = 30 * 24 * time.Hour
	asyncWebhookFailureRetention = 90 * 24 * time.Hour
)

func PurgeOldWebhookDeliveries() {
	for _, policy := range []struct {
		statuses []string
		maxAge   time.Duration
	}{
		{[]string{model.AssetWebhookDeliveryStatusSucceeded, model.AssetWebhookDeliveryStatusSuperseded}, asyncWebhookSuccessRetention},
		{[]string{model.AssetWebhookDeliveryStatusExhausted}, asyncWebhookFailureRetention},
	} {
		cutoff := time.Now().Add(-policy.maxAge).Unix()
		for range asyncWebhookRetentionPasses {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			deleted, err := model.PurgeAssetWebhookDeliveriesBefore(ctx, policy.statuses, cutoff, asyncWebhookRetentionBatch)
			cancel()
			if err != nil {
				logger.LogWarn(context.Background(), fmt.Sprintf("purge old webhook deliveries failed: %v", err))
				break
			}
			if deleted < asyncWebhookRetentionBatch {
				break
			}
		}
	}
}
