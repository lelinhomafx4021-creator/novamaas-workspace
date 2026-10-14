package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	storageService "github.com/QuantumNous/new-api/service/storage"
)

type RequestSnapshotMaintenanceInput struct {
	BatchSize         int  `json:"batch_size"`
	ClearDatabaseBody bool `json:"clear_database_body"`
}

type RequestSnapshotMaintenanceState struct {
	Status         string                            `json:"status"`
	Mode           string                            `json:"mode"`
	Cursor         int64                             `json:"cursor"`
	Processed      int64                             `json:"processed"`
	Failed         int64                             `json:"failed"`
	Scanned        int64                             `json:"scanned"`
	UpdatedAt      int64                             `json:"updated_at"`
	ActiveFiles    int                               `json:"active_files"`
	Workers        int                               `json:"workers"`
	Error          string                            `json:"error,omitempty"`
	Phase          string                            `json:"phase,omitempty"`
	Initialized    bool                              `json:"initialized"`
	TaskCursor     int64                             `json:"task_cursor"`
	TaskUpperID    int64                             `json:"task_upper_id"`
	LogCursor      model.LogRequestBodyArchiveCursor `json:"log_cursor"`
	LogUpperCursor model.LogRequestBodyArchiveCursor `json:"log_upper_cursor"`
	ArchiveUpperID int64                             `json:"archive_upper_id"`
	RequestSnapshotMaintenanceInput
}

var snapshotMaintenance struct {
	sync.Mutex
	state  RequestSnapshotMaintenanceState
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// Each node owns its spool and maintenance checkpoint. A restart resumes from
// the last completed batch; object publication and cleanup are idempotent.
func InitRequestSnapshotMaintenance() error {
	snapshotMaintenance.Lock()
	defer snapshotMaintenance.Unlock()
	snapshotMaintenance.ctx, snapshotMaintenance.cancel = context.WithCancel(context.Background())
	snapshotMaintenance.state = RequestSnapshotMaintenanceState{Status: "idle"}
	dir := storageService.SnapshotWorkerStatus().SpoolDirectory
	if dir == "" {
		return nil
	}
	body, err := os.ReadFile(filepath.Join(dir, "maintenance-state.json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := common.Unmarshal(body, &snapshotMaintenance.state); err != nil {
		return err
	}
	state := snapshotMaintenance.state
	if state.Status == "running" {
		if state.Mode != "migrate" && state.Mode != "cleanup" || state.BatchSize < 1 || state.BatchSize > 100 || state.Cursor < 0 {
			return errors.New("invalid snapshot maintenance checkpoint")
		}
		snapshotMaintenance.wg.Add(1)
		go runRequestSnapshotMaintenance(snapshotMaintenance.ctx, state)
	}
	return nil
}

func StopRequestSnapshotMaintenance(ctx context.Context) error {
	snapshotMaintenance.Lock()
	if snapshotMaintenance.cancel != nil {
		snapshotMaintenance.cancel()
	}
	snapshotMaintenance.Unlock()
	done := make(chan struct{})
	go func() {
		snapshotMaintenance.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func RequestSnapshotMaintenanceStatus() RequestSnapshotMaintenanceState {
	snapshotMaintenance.Lock()
	defer snapshotMaintenance.Unlock()
	state := snapshotMaintenance.state
	if state.Status == "" {
		state.Status = "idle"
	}
	return state
}

func StartRequestSnapshotMaintenance(mode string, input RequestSnapshotMaintenanceInput) (RequestSnapshotMaintenanceState, error) {
	if mode != "migrate" && mode != "cleanup" {
		return RequestSnapshotMaintenanceState{}, errors.New("invalid maintenance mode")
	}
	if input.BatchSize < 1 || input.BatchSize > 100 {
		return RequestSnapshotMaintenanceState{}, errors.New("batch size must be between 1 and 100")
	}
	if storageService.SnapshotWorkerStatus().SpoolDirectory == "" {
		return RequestSnapshotMaintenanceState{}, errors.New("snapshot spool is unavailable on this node")
	}
	snapshotMaintenance.Lock()
	defer snapshotMaintenance.Unlock()
	if snapshotMaintenance.ctx == nil || snapshotMaintenance.ctx.Err() != nil {
		return RequestSnapshotMaintenanceState{}, errors.New("snapshot maintenance is unavailable")
	}
	if snapshotMaintenance.state.Status == "running" {
		return RequestSnapshotMaintenanceState{}, errors.New("snapshot maintenance is already running on this node")
	}
	state := RequestSnapshotMaintenanceState{Status: "running", Mode: mode,
		Workers: min(max(common.GetEnvOrDefault("SNAPSHOT_MIGRATION_WORKERS", 2), 1), 4), UpdatedAt: time.Now().Unix(),
		RequestSnapshotMaintenanceInput: input}
	if err := persistRequestSnapshotMaintenance(state); err != nil {
		return RequestSnapshotMaintenanceState{}, err
	}
	snapshotMaintenance.state = state
	snapshotMaintenance.wg.Add(1)
	go runRequestSnapshotMaintenance(snapshotMaintenance.ctx, state)
	return state, nil
}

func runRequestSnapshotMaintenance(ctx context.Context, state RequestSnapshotMaintenanceState) {
	defer snapshotMaintenance.wg.Done()
	state.Workers = min(max(common.GetEnvOrDefault("SNAPSHOT_MIGRATION_WORKERS", 2), 1), 4)
	state.ActiveFiles = 0
	for ctx.Err() == nil {
		_, done, err := advanceRequestSnapshotMaintenance(ctx, &state)
		if ctx.Err() != nil {
			// Preserve running checkpoint for recovery on the same persistent disk.
			return
		}
		if err != nil {
			state.Status, state.Error, state.Failed = "failed", err.Error(), state.Failed+1
		} else if done {
			state.Status = "completed"
		}
		state.UpdatedAt = time.Now().Unix()
		snapshotMaintenance.Lock()
		checkpointErr := persistRequestSnapshotMaintenance(state)
		if checkpointErr != nil {
			state.Status, state.Error, state.Failed = "failed", checkpointErr.Error(), state.Failed+1
			common.SysError("snapshot maintenance checkpoint failed: " + checkpointErr.Error())
		}
		snapshotMaintenance.state = state
		snapshotMaintenance.Unlock()
		if state.Status != "running" {
			return
		}
		interval := min(max(common.GetEnvOrDefault("SNAPSHOT_MIGRATION_BATCH_INTERVAL_MS", 100), 0), 5000)
		timer := time.NewTimer(time.Duration(interval) * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func advanceRequestSnapshotMaintenance(ctx context.Context, state *RequestSnapshotMaintenanceState) (int, bool, error) {
	metadataCtx, cancel := context.WithTimeout(ctx, time.Duration(min(max(common.GetEnvOrDefault("SNAPSHOT_MIGRATION_OBJECT_TIMEOUT_SECONDS", 300), 30), 1800))*time.Second)
	defer cancel()
	if !state.Initialized {
		upperID, err := model.GetTaskRequestBodyArchiveUpperID(metadataCtx)
		if err != nil {
			return 0, false, err
		}
		upperLog, err := model.GetLogRequestBodyArchiveUpperCursor(metadataCtx)
		if err != nil {
			return 0, false, err
		}
		state.Initialized, state.Phase = true, "legacy_tasks"
		state.TaskUpperID, state.LogUpperCursor = upperID, upperLog
	}
	// Older checkpoints did not record an archive-table upper bound. Capture it
	// once when resuming, so new database snapshots cannot extend this job forever.
	if state.ArchiveUpperID == 0 {
		upperID, err := model.GetDatabaseSnapshotArchiveUpperID(metadataCtx)
		if err != nil {
			return 0, false, err
		}
		state.ArchiveUpperID = upperID
	}
	state.UpdatedAt = time.Now().Unix()
	snapshotMaintenance.Lock()
	snapshotMaintenance.state = *state
	snapshotMaintenance.Unlock()
	items := make([]snapshotMaintenanceItem, 0, state.BatchSize)
	if state.Phase == "legacy_tasks" {
		rows, err := model.FindLegacyTaskRequestBodyBatch(metadataCtx, state.TaskCursor, state.TaskUpperID, state.BatchSize, true)
		if err != nil {
			return 0, false, err
		}
		for _, row := range rows {
			items = append(items, snapshotMaintenanceItem{key: row.TaskID, id: row.ID})
		}
		count, err := runRequestSnapshotMaintenanceBatch(ctx, state, items, processRequestSnapshotMaintenanceItem)
		if err == nil && (len(rows) < state.BatchSize || state.TaskCursor >= state.TaskUpperID) {
			state.Phase = "legacy_logs"
		}
		return count, false, err
	}
	if state.Phase == "legacy_logs" {
		rows, err := model.FindLogRequestBodyArchiveBatch(metadataCtx, state.LogCursor, state.LogUpperCursor, state.BatchSize, true)
		if err != nil {
			return 0, false, err
		}
		previous := state.LogCursor
		for _, row := range rows {
			cursor := model.LogRequestBodyArchiveCursor{ID: row.ID, CreatedAt: row.CreatedAt, RequestID: row.RequestID, OtherSHA256: row.OtherSHA256}
			// Request-less logs may share a task reference hidden in their body.
			// Sending all of them to the same lane keeps publication ordered.
			items = append(items, snapshotMaintenanceItem{key: row.RequestID, logBefore: previous, logCursor: cursor})
			previous = cursor
		}
		count, err := runRequestSnapshotMaintenanceBatch(ctx, state, items, processRequestSnapshotMaintenanceItem)
		if err == nil && len(rows) < state.BatchSize {
			state.Phase = "archive_table"
		}
		return count, false, err
	}
	if state.Phase != "archive_table" {
		return 0, false, errors.New("invalid snapshot maintenance phase")
	}
	rows, err := model.FindDatabaseSnapshotBatch(state.Cursor, state.BatchSize, metadataCtx)
	if err != nil {
		return 0, false, err
	}
	for _, row := range rows {
		if row.ID > state.ArchiveUpperID {
			break
		}
		items = append(items, snapshotMaintenanceItem{key: row.ReferenceID, id: row.ID, size: row.BodySize})
	}
	count, err := runRequestSnapshotMaintenanceBatch(ctx, state, items, processRequestSnapshotMaintenanceItem)
	done := err == nil && (len(rows) < state.BatchSize || len(items) < len(rows) || state.Cursor >= state.ArchiveUpperID)
	return count, done, err
}

func persistRequestSnapshotMaintenance(state RequestSnapshotMaintenanceState) error {
	dir := storageService.SnapshotWorkerStatus().SpoolDirectory
	body, err := common.Marshal(state)
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(dir, ".maintenance-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(body); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(file.Name(), filepath.Join(dir, "maintenance-state.json")); err != nil {
		return err
	}
	directory, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}
