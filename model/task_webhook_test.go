package model

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupTaskWebhookDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&Task{}, &AssetWebhookEndpoint{}, &AssetWebhookDelivery{}))
	original := DB
	DB = db
	t.Cleanup(func() { DB = original; assert.NoError(t, sqlDB.Close()) })
	return db
}

func TestDisabledCategoriesDoNotQueuePersonalOrSystemEvents(t *testing.T) {
	db := setupTaskWebhookDB(t)
	require.NoError(t, db.Where("owner_user_id = 0").Delete(&AssetWebhookEndpoint{}).Error)
	personal := AssetWebhookEndpoint{PublicID: "we_policy", OwnerUserID: 7, URL: "https://customer.example/events", Status: AssetWebhookEndpointStatusEnabled, EventTypes: `["asset.failed","task.status_changed"]`}
	require.NoError(t, db.Create(&personal).Error)
	for _, eventType := range []string{WebhookEventAssetFailed, WebhookEventTaskStatusChanged} {
		id, err := QueueWebhookEvent(db, 7, eventType, 0, map[string]any{"status": "done"})
		require.NoError(t, err)
		assert.Empty(t, id, "unconfigured categories default to off")
	}
	assetFlag := AssetWebhookEndpoint{PublicID: SystemAssetWebhookEndpointID, OwnerUserID: 0, Status: AssetWebhookEndpointStatusEnabled, EventTypes: `["asset.failed"]`}
	require.NoError(t, db.Create(&assetFlag).Error)
	id, err := QueueWebhookEvent(db, 7, WebhookEventTaskStatusChanged, 0, map[string]any{})
	require.NoError(t, err)
	assert.Empty(t, id, "enabling assets does not enable tasks")
	id, err = QueueWebhookEvent(db, 7, WebhookEventAssetFailed, 0, map[string]any{})
	require.NoError(t, err)
	assert.NotEmpty(t, id)
	var deliveries []AssetWebhookDelivery
	require.NoError(t, db.Find(&deliveries).Error)
	require.Len(t, deliveries, 1, "a blank operator URL does not create a system delivery")
	assert.Equal(t, personal.ID, deliveries[0].EndpointID)
}

func TestTaskWebhookStatusTransitionsAreOwnedVersionedAndImmutable(t *testing.T) {
	db := setupTaskWebhookDB(t)
	enableTaskWebhookCategory(t, db)
	endpoints := []AssetWebhookEndpoint{
		{PublicID: "we_owner_a", OwnerUserID: 7, EventTypes: `["task.status_changed"]`, Status: AssetWebhookEndpointStatusEnabled},
		{PublicID: "we_owner_b", OwnerUserID: 7, EventTypes: `["asset.failed","task.status_changed"]`, Status: AssetWebhookEndpointStatusEnabled},
		{PublicID: "we_other_user", OwnerUserID: 8, EventTypes: `["task.status_changed"]`, Status: AssetWebhookEndpointStatusEnabled},
		{PublicID: "we_asset_only", OwnerUserID: 7, EventTypes: `["asset.active"]`, Status: AssetWebhookEndpointStatusEnabled},
		{PublicID: "we_disabled", OwnerUserID: 7, EventTypes: `["task.status_changed"]`, Status: AssetWebhookEndpointStatusDisabled},
	}
	require.NoError(t, db.Create(&endpoints).Error)
	task := Task{TaskID: "task_public", UserId: 7, Platform: constant.TaskPlatform("video"), Status: TaskStatusSubmitted,
		PrivateData: TaskPrivateData{Key: "private-provider-key", UpstreamTaskID: "upstream-private-id"},
		Data:        json.RawMessage(`{"id":"provider-response","status":"queued"}`)}
	require.NoError(t, task.Insert())
	loser := task
	task.Status, task.Progress = TaskStatusInProgress, "50%"
	task.Data = json.RawMessage(`{"status":"processing"}`)
	won, err := task.UpdateWithStatus(TaskStatusSubmitted)
	require.NoError(t, err)
	require.True(t, won)
	loser.Status = TaskStatusFailure
	won, err = loser.UpdateWithStatus(TaskStatusSubmitted)
	require.NoError(t, err)
	assert.False(t, won, "a CAS loser must not emit an event")
	task.Progress = "75%"
	_, err = task.UpdateWithStatus(TaskStatusInProgress)
	require.NoError(t, err)
	task.Status, task.Progress = TaskStatusSuccess, "100%"
	task.Data = json.RawMessage(`{"status":"completed","url":"https://media.example/video.mp4"}`)
	won, err = task.UpdateWithStatus(TaskStatusInProgress)
	require.NoError(t, err)
	require.True(t, won)

	var deliveries []AssetWebhookDelivery
	require.NoError(t, db.Order("id").Find(&deliveries).Error)
	require.Len(t, deliveries, 6, "three transitions fan out only to two own subscribers")
	statuses := []TaskStatus{TaskStatusSubmitted, TaskStatusInProgress, TaskStatusSuccess}
	responses := []string{`{"id":"provider-response","status":"queued"}`, `{"status":"processing"}`, `{"status":"completed","url":"https://media.example/video.mp4"}`}
	for index, status := range statuses {
		first, second := deliveries[index*2], deliveries[index*2+1]
		assert.Equal(t, first.EventID, second.EventID)
		assert.NotEqual(t, first.WebhookID, second.WebhookID)
		assert.Equal(t, 7, first.OwnerUserID)
		assert.Equal(t, WebhookEventTaskStatusChanged, first.EventType)
		assert.NotContains(t, first.Payload, "private-provider-key")
		assert.NotContains(t, first.Payload, "upstream-private-id")
		var event struct {
			ID   string          `json:"id"`
			Data taskWebhookData `json:"data"`
		}
		require.NoError(t, common.UnmarshalJsonStr(first.Payload, &event))
		assert.Equal(t, first.EventID, event.ID)
		assert.Equal(t, "task_public", event.Data.TaskID)
		assert.Equal(t, status, event.Data.Status)
		assert.EqualValues(t, index+1, event.Data.StateVersion)
		assert.JSONEq(t, responses[index], string(event.Data.Response))
	}
}

func TestTaskWebhookOutboxFailureRollsBackState(t *testing.T) {
	for _, operation := range []string{"insert", "transition", "bulk"} {
		t.Run(operation, func(t *testing.T) {
			db := setupTaskWebhookDB(t)
			enableTaskWebhookCategory(t, db)
			endpoint := AssetWebhookEndpoint{PublicID: "we_rollback", OwnerUserID: 7, EventTypes: `["task.status_changed"]`, Status: AssetWebhookEndpointStatusEnabled}
			require.NoError(t, db.Create(&endpoint).Error)
			task := Task{TaskID: "task_rollback", UserId: 7, Status: TaskStatusSubmitted}
			if operation != "insert" {
				require.NoError(t, task.Insert())
			}
			require.NoError(t, db.Callback().Create().Before("gorm:create").Register("outbox_failure", func(tx *gorm.DB) {
				if tx.Statement.Table == "asset_webhook_deliveries" {
					tx.AddError(errors.New("outbox unavailable"))
				}
			}))
			t.Cleanup(func() { _ = db.Callback().Create().Remove("outbox_failure") })
			switch operation {
			case "insert":
				require.ErrorContains(t, task.Insert(), "outbox unavailable")
			case "transition":
				task.Status = TaskStatusSuccess
				won, err := task.UpdateWithStatus(TaskStatusSubmitted)
				require.ErrorContains(t, err, "outbox unavailable")
				assert.False(t, won)
			case "bulk":
				require.ErrorContains(t, TaskBulkUpdateByID([]int64{task.ID}, map[string]any{"status": TaskStatusFailure}), "outbox unavailable")
			}
			var stored []Task
			require.NoError(t, db.Find(&stored).Error)
			if operation == "insert" {
				assert.Empty(t, stored)
				return
			}
			require.Len(t, stored, 1)
			assert.EqualValues(t, TaskStatusSubmitted, stored[0].Status)
			assert.EqualValues(t, 1, stored[0].WebhookVersion)
		})
	}
}

func TestTaskWebhookBulkFailureAndLegacyUpdateEmitStatus(t *testing.T) {
	db := setupTaskWebhookDB(t)
	enableTaskWebhookCategory(t, db)
	require.NoError(t, db.Create(&AssetWebhookEndpoint{PublicID: "we_failure", OwnerUserID: 7, EventTypes: `["task.status_changed"]`, Status: AssetWebhookEndpointStatusEnabled}).Error)
	task := Task{TaskID: "task_failure", UserId: 7, Status: TaskStatusSubmitted}
	require.NoError(t, task.Insert())
	require.NoError(t, TaskBulkUpdateByID([]int64{task.ID}, map[string]any{"status": TaskStatusFailure, "fail_reason": "channel removed", "progress": "100%"}))
	require.NoError(t, db.First(&task, task.ID).Error)
	task.Status = TaskStatusSuccess
	require.NoError(t, task.Update())
	var deliveries []AssetWebhookDelivery
	require.NoError(t, db.Order("id").Find(&deliveries).Error)
	require.Len(t, deliveries, 3)
	var failure struct {
		Data taskWebhookData `json:"data"`
	}
	require.NoError(t, common.UnmarshalJsonStr(deliveries[1].Payload, &failure))
	assert.EqualValues(t, TaskStatusFailure, failure.Data.Status)
	assert.Equal(t, "channel removed", failure.Data.FailReason)
	assert.EqualValues(t, 2, failure.Data.StateVersion)
}

func TestTaskWebhookLargeResponseIsBoundedWithPublicResultRetained(t *testing.T) {
	db := setupTaskWebhookDB(t)
	enableTaskWebhookCategory(t, db)
	require.NoError(t, db.Create(&AssetWebhookEndpoint{PublicID: "we_bounded", OwnerUserID: 7, EventTypes: `["task.status_changed"]`, Status: AssetWebhookEndpointStatusEnabled}).Error)
	response, err := common.Marshal(map[string]any{"status": "completed", "url": "https://media.example/result.mp4", "base64": strings.Repeat("A", 8192)})
	require.NoError(t, err)
	task := Task{TaskID: "task_bounded", UserId: 7, Status: TaskStatusSuccess, Data: response}
	require.NoError(t, task.Insert())
	var delivery AssetWebhookDelivery
	require.NoError(t, db.First(&delivery).Error)
	var event struct {
		Data taskWebhookData `json:"data"`
	}
	require.NoError(t, common.UnmarshalJsonStr(delivery.Payload, &event))
	assert.True(t, event.Data.ResponseTruncated)
	assert.NotEmpty(t, event.Data.ResponseSHA256)
	assert.Contains(t, string(event.Data.Response), "https://media.example/result.mp4")
	assert.NotContains(t, string(event.Data.Response), strings.Repeat("A", 8192))
	assert.LessOrEqual(t, len(event.Data.Response), MaxTaskPollResponseBytes)
}

func TestSystemWebhookReceivesEveryOwnerAlongsideTenantSubscription(t *testing.T) {
	db := setupTaskWebhookDB(t)
	endpoints := []AssetWebhookEndpoint{
		{PublicID: SystemTaskWebhookEndpointID, OwnerUserID: 0, URL: "https://operator.example/tasks", Status: AssetWebhookEndpointStatusEnabled, EventTypes: `["task.status_changed"]`},
		{PublicID: SystemAssetWebhookEndpointID, OwnerUserID: 0, URL: "https://operator.example/assets", Status: AssetWebhookEndpointStatusEnabled, EventTypes: `["asset.active","asset.failed"]`},
		{PublicID: "we_personal", OwnerUserID: 7, Status: AssetWebhookEndpointStatusEnabled, EventTypes: `["task.status_changed"]`},
	}
	require.NoError(t, db.Create(&endpoints).Error)
	for _, owner := range []int{7, 8} {
		task := Task{TaskID: GenerateTaskID(), UserId: owner, Status: TaskStatusSubmitted}
		require.NoError(t, task.Insert())
	}
	_, err := QueueWebhookEvent(db, 8, WebhookEventAssetFailed, 99, map[string]any{"id": "asset_8", "status": "Failed"})
	require.NoError(t, err)
	var deliveries []AssetWebhookDelivery
	require.NoError(t, db.Order("id").Find(&deliveries).Error)
	require.Len(t, deliveries, 4)
	var owners []int
	for _, delivery := range deliveries {
		var event WebhookEvent
		require.NoError(t, common.UnmarshalJsonStr(delivery.Payload, &event))
		if delivery.EndpointPublicID == SystemTaskWebhookEndpointID {
			owners = append(owners, event.OwnerUserID)
		}
	}
	assert.Equal(t, []int{7, 8}, owners)
	assert.Equal(t, deliveries[0].EventID, deliveries[1].EventID, "tenant and system share the event ID")
	require.NoError(t, db.Model(&endpoints[0]).Update("status", AssetWebhookEndpointStatusDisabled).Error)
	task := Task{TaskID: "task_after_disable", UserId: 7, Status: TaskStatusSubmitted}
	require.NoError(t, task.Insert())
	var count int64
	require.NoError(t, db.Model(&AssetWebhookDelivery{}).Count(&count).Error)
	assert.EqualValues(t, 4, count, "a disabled category stops personal and system notifications")
}

func enableTaskWebhookCategory(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Create(&AssetWebhookEndpoint{PublicID: SystemTaskWebhookEndpointID, OwnerUserID: 0, Status: AssetWebhookEndpointStatusEnabled, EventTypes: `["task.status_changed"]`}).Error)
}
