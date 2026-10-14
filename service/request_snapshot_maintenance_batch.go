package service

import (
	"context"
	"hash/crc32"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	storageService "github.com/QuantumNous/new-api/service/storage"
)

type snapshotMaintenanceItem struct {
	key       string
	id        int64
	size      int64
	logBefore model.LogRequestBodyArchiveCursor
	logCursor model.LogRequestBodyArchiveCursor
}

type snapshotMaintenanceResult struct {
	index     int
	started   bool
	completed bool
	processed bool
	err       error
}

type snapshotMaintenanceWork struct {
	index int
	item  snapshotMaintenanceItem
}

// Each reference belongs to one FIFO lane. This preserves source order while
// allowing different objects to upload concurrently without loading a page of
// large bodies into memory. Checkpoints advance only over a successful prefix.
func runRequestSnapshotMaintenanceBatch(ctx context.Context, state *RequestSnapshotMaintenanceState, items []snapshotMaintenanceItem, process func(context.Context, string, string, bool, snapshotMaintenanceItem) (bool, error)) (int, error) {
	if len(items) == 0 {
		return 0, nil
	}
	workers := min(max(common.GetEnvOrDefault("SNAPSHOT_MIGRATION_WORKERS", 2), 1), 4)
	state.Workers = workers
	workers = min(workers, len(items))
	lanes := make([]chan snapshotMaintenanceWork, workers)
	events := make(chan snapshotMaintenanceResult, len(items)*2)
	batchCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	budget := snapshotMigrationMemoryBudget{
		limit:   int64(min(max(common.GetEnvOrDefault("SNAPSHOT_MIGRATION_MEMORY_MIB", 256), 32), 2048)) * 1024 * 1024,
		changed: make(chan struct{}),
	}
	var wg sync.WaitGroup
	for lane := range lanes {
		lanes[lane] = make(chan snapshotMaintenanceWork, len(items))
		wg.Add(1)
		go requestSnapshotMaintenanceWorker(batchCtx, state.Mode, state.Phase, state.ClearDatabaseBody, lanes[lane], events, process, &budget, &wg)
	}
	for index, item := range items {
		lane := int(crc32.ChecksumIEEE([]byte(item.key)) % uint32(workers))
		lanes[lane] <- snapshotMaintenanceWork{index: index, item: item}
	}
	for _, lane := range lanes {
		close(lane)
	}
	go func() {
		wg.Wait()
		close(events)
	}()

	results := make([]snapshotMaintenanceResult, len(items))
	frontier, processed := 0, 0
	var cause error
	for event := range events {
		if event.started {
			state.ActiveFiles++
		} else {
			state.ActiveFiles--
			results[event.index] = event
			if event.err != nil && cause == nil {
				cause = event.err
				cancel()
			}
		}
		for frontier < len(results) && results[frontier].completed && results[frontier].err == nil {
			state.Scanned++
			if results[frontier].processed {
				processed++
				state.Processed++
			}
			switch state.Phase {
			case "legacy_tasks":
				state.TaskCursor = items[frontier].id
			case "legacy_logs":
				state.LogCursor = items[frontier].logCursor
			case "archive_table":
				state.Cursor = items[frontier].id
			}
			frontier++
		}
		state.UpdatedAt = time.Now().Unix()
		snapshotMaintenance.Lock()
		snapshotMaintenance.state = *state
		snapshotMaintenance.Unlock()
	}
	if frontier < len(results) {
		return processed, cause
	}
	return processed, nil
}

func requestSnapshotMaintenanceWorker(ctx context.Context, mode, phase string, clearBody bool, jobs <-chan snapshotMaintenanceWork, events chan<- snapshotMaintenanceResult, process func(context.Context, string, string, bool, snapshotMaintenanceItem) (bool, error), budget *snapshotMigrationMemoryBudget, wg *sync.WaitGroup) {
	defer wg.Done()
	timeout := time.Duration(min(max(common.GetEnvOrDefault("SNAPSHOT_MIGRATION_OBJECT_TIMEOUT_SECONDS", 300), 30), 1800)) * time.Second
	for job := range jobs {
		fileCtx, cancel := context.WithTimeout(ctx, timeout)
		reservation, err := budget.acquire(fileCtx, job.item.size)
		events <- snapshotMaintenanceResult{index: job.index, started: true}
		processed := false
		if err == nil {
			processed, err = process(fileCtx, mode, phase, clearBody, job.item)
			budget.release(reservation)
		}
		cancel()
		events <- snapshotMaintenanceResult{index: job.index, completed: true, processed: processed, err: err}
	}
}

// This bounds the estimated bodies in flight, not total process RAM. Oversized
// files and legacy sources whose size is unknown run alone instead of starving
// indefinitely or multiplying a large payload by the worker count.
type snapshotMigrationMemoryBudget struct {
	mu      sync.Mutex
	used    int64
	limit   int64
	changed chan struct{}
}

func (budget *snapshotMigrationMemoryBudget) acquire(ctx context.Context, size int64) (int64, error) {
	if size <= 0 || size > budget.limit {
		size = budget.limit
	}
	for {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		budget.mu.Lock()
		if size <= budget.limit-budget.used {
			budget.used += size
			budget.mu.Unlock()
			return size, nil
		}
		changed := budget.changed
		budget.mu.Unlock()
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-changed:
		}
	}
}

func (budget *snapshotMigrationMemoryBudget) release(size int64) {
	budget.mu.Lock()
	defer budget.mu.Unlock()
	budget.used -= size
	close(budget.changed)
	budget.changed = make(chan struct{})
}

func processRequestSnapshotMaintenanceItem(ctx context.Context, mode, phase string, clearBody bool, item snapshotMaintenanceItem) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	switch phase {
	case "legacy_tasks":
		if mode == "cleanup" {
			_, count, _, err := storageService.CleanupLegacyTaskRequestSnapshots(ctx, item.id-1, item.id, 1, true)
			return count > 0, err
		}
		_, count, _, err := storageService.MigrateLegacyTaskRequestSnapshots(ctx, item.id-1, item.id, 1, clearBody)
		return count > 0, err
	case "legacy_logs":
		if mode == "cleanup" {
			_, count, _, err := storageService.CleanupLegacyLogRequestSnapshots(ctx, item.logBefore, item.logCursor, 1, true)
			return count > 0, err
		}
		_, count, _, err := storageService.MigrateLegacyLogRequestSnapshots(ctx, item.logBefore, item.logCursor, 1, clearBody)
		return count > 0, err
	default:
		if mode == "cleanup" {
			return storageService.CleanupDatabaseRequestSnapshot(ctx, item.id, true)
		}
		return storageService.MigrateDatabaseRequestSnapshot(ctx, item.id, clearBody)
	}
}
