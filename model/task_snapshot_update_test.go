package model

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskPollingCannotRestoreAnArchivedLegacyRequestBody(t *testing.T) {
	db := setupLegacySnapshotCleanupDB(t)
	task := Task{TaskID: "task-poll-cleanup", Status: TaskStatusInProgress, Progress: "20%",
		Properties: Properties{Input: "keep input", RequestBody: json.RawMessage(`{"prompt":"legacy body"}`)}}
	require.NoError(t, db.Create(&task).Error)
	var stale Task
	require.NoError(t, db.First(&stale, task.ID).Error)
	assert.False(t, stale.RequestBodyAvailable)
	digest := saveLegacySnapshotIndex(t, task.TaskID, "", stale.Properties.RequestBody)
	encoded, err := stale.Properties.Value()
	require.NoError(t, err)
	var properties map[string]json.RawMessage
	require.NoError(t, common.Unmarshal(encoded.([]byte), &properties))
	properties["future_metric"] = json.RawMessage(`9007199254740993`)
	encoded, err = common.Marshal(properties)
	require.NoError(t, err)
	require.NoError(t, db.Model(&Task{}).Where("id = ?", task.ID).Update("properties", encoded).Error)
	cleared, err := ClearLegacyTaskSnapshotBody(context.Background(), task.ID, digest)
	require.NoError(t, err)
	require.True(t, cleared)

	stale.Progress = "60%"
	won, err := stale.UpdateWithStatus(TaskStatusInProgress)
	require.NoError(t, err)
	require.True(t, won)
	var stored Task
	require.NoError(t, db.First(&stored, task.ID).Error)
	assert.Empty(t, stored.Properties.RequestBody)
	assert.Empty(t, stale.Properties.RequestBody)
	assert.True(t, stored.RequestBodyAvailable)
	assert.Equal(t, "60%", stored.Progress)
	assert.Equal(t, "keep input", stored.Properties.Input)
	var raw struct {
		Properties json.RawMessage `gorm:"type:json"`
	}
	require.NoError(t, db.Model(&Task{}).Select("properties").Where("id = ?", task.ID).Scan(&raw).Error)
	properties = nil
	require.NoError(t, common.Unmarshal(raw.Properties, &properties))
	assert.NotContains(t, properties, "request_body")
	assert.Equal(t, "9007199254740993", string(properties["future_metric"]))
}
