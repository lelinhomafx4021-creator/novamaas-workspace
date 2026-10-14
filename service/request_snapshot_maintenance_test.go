package service

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	storageService "github.com/QuantumNous/new-api/service/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSnapshotMaintenanceLoadsCompletedCheckpointAfterRestart(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SNAPSHOT_SPOOL_DIR", dir)
	require.NoError(t, storageService.StartRequestSnapshotWorkers())
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		require.NoError(t, StopRequestSnapshotMaintenance(ctx))
		require.NoError(t, storageService.StopRequestSnapshotWorkers(ctx))
	})
	state := RequestSnapshotMaintenanceState{
		Status: "completed", Mode: "migrate", Cursor: 17, Processed: 12,
		RequestSnapshotMaintenanceInput: RequestSnapshotMaintenanceInput{BatchSize: 20},
	}
	require.NoError(t, persistRequestSnapshotMaintenance(state))
	require.NoError(t, InitRequestSnapshotMaintenance())
	assert.Equal(t, state, RequestSnapshotMaintenanceStatus())
	info, err := os.Stat(filepath.Join(dir, "maintenance-state.json"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
}

func TestSnapshotMaintenanceRejectsInvalidRecoveryBatch(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SNAPSHOT_SPOOL_DIR", dir)
	require.NoError(t, storageService.StartRequestSnapshotWorkers())
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		require.NoError(t, StopRequestSnapshotMaintenance(ctx))
		require.NoError(t, storageService.StopRequestSnapshotWorkers(ctx))
	})
	body, err := common.Marshal(RequestSnapshotMaintenanceState{Status: "running", Mode: "migrate", RequestSnapshotMaintenanceInput: RequestSnapshotMaintenanceInput{BatchSize: 1000}})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "maintenance-state.json"), body, 0600))
	assert.ErrorContains(t, InitRequestSnapshotMaintenance(), "invalid snapshot maintenance checkpoint")
}

func TestSnapshotCleanupResumesAllSourcesAndPreservesUnverifiedBodies(t *testing.T) {
	truncate(t)
	task := model.Task{TaskID: "maintenance-hot-task", Properties: model.Properties{RequestBody: json.RawMessage(`{"source":"task"}`)}}
	require.NoError(t, model.DB.Create(&task).Error)
	log := model.Log{CreatedAt: 100, RequestId: "maintenance-hot-log", Other: `{"request_body":{"source":"log"},"keep":true}`}
	require.NoError(t, model.LOG_DB.Create(&log).Error)
	require.NoError(t, model.SaveTaskRequestBody("", "maintenance-inline", []byte(`{"source":"archive"}`)))
	state := RequestSnapshotMaintenanceState{
		Status: "running", Mode: "cleanup",
		RequestSnapshotMaintenanceInput: RequestSnapshotMaintenanceInput{BatchSize: 1, ClearDatabaseBody: true},
	}
	for step, phase := range []string{"legacy_logs", "legacy_logs", "archive_table", "archive_table"} {
		processed, done, err := advanceRequestSnapshotMaintenance(context.Background(), &state)
		require.NoError(t, err)
		assert.Zero(t, processed, "an unverified object must never authorize removal")
		assert.Equal(t, phase, state.Phase)
		assert.Equal(t, step == 3, done)
		// Recovery must continue from the serialized phase and source cursor.
		checkpoint, err := common.Marshal(state)
		require.NoError(t, err)
		state = RequestSnapshotMaintenanceState{}
		require.NoError(t, common.Unmarshal(checkpoint, &state))
	}
	var storedTask model.Task
	require.NoError(t, model.DB.First(&storedTask, task.ID).Error)
	assert.JSONEq(t, `{"source":"task"}`, string(storedTask.Properties.RequestBody))
	var storedLog model.Log
	require.NoError(t, model.LOG_DB.First(&storedLog, log.Id).Error)
	assert.JSONEq(t, log.Other, storedLog.Other)
	body, exists, err := model.GetArchivedRequestBody("", "maintenance-inline")
	require.NoError(t, err)
	require.True(t, exists)
	assert.JSONEq(t, `{"source":"archive"}`, string(body))
}
