package service

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSnapshotMaintenanceParallelBatchKeepsReferenceOrder(t *testing.T) {
	t.Setenv("SNAPSHOT_MIGRATION_WORKERS", "2")
	t.Setenv("SNAPSHOT_MIGRATION_MEMORY_MIB", "32")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	items := []snapshotMaintenanceItem{
		{key: "same-reference", id: 1, size: 1024 * 1024},
		{key: "independent-reference", id: 2, size: 1024 * 1024},
		{key: "same-reference", id: 3, size: 1024 * 1024},
	}
	gates := map[int64]chan struct{}{1: make(chan struct{}), 2: make(chan struct{}), 3: make(chan struct{})}
	entered := make(chan int64, 3)
	var active atomic.Int32
	process := func(ctx context.Context, _, _ string, _ bool, item snapshotMaintenanceItem) (bool, error) {
		active.Add(1)
		defer active.Add(-1)
		entered <- item.id
		select {
		case <-gates[item.id]:
			return true, nil
		case <-ctx.Done():
			return false, ctx.Err()
		}
	}
	state := RequestSnapshotMaintenanceState{Status: "running", Mode: "migrate", Phase: "archive_table"}
	done := make(chan error, 1)
	go func() {
		_, err := runRequestSnapshotMaintenanceBatch(ctx, &state, items, process)
		done <- err
	}()
	started := make([]int64, 0, 2)
	for range 2 {
		select {
		case id := <-entered:
			started = append(started, id)
		case <-ctx.Done():
			require.FailNow(t, "independent references did not start concurrently")
		}
	}
	assert.ElementsMatch(t, []int64{1, 2}, started)
	assert.EqualValues(t, 2, active.Load())
	close(gates[1])
	select {
	case id := <-entered:
		require.EqualValues(t, 3, id, "the next attempt on this reference must follow its predecessor")
	case <-ctx.Done():
		require.FailNow(t, "same-reference successor did not start")
	}
	close(gates[2])
	close(gates[3])
	require.NoError(t, <-done)
	assert.EqualValues(t, 3, state.Cursor)
	assert.EqualValues(t, 3, state.Processed)
	assert.EqualValues(t, 3, state.Scanned)
	assert.Zero(t, state.ActiveFiles)
	assert.Equal(t, 2, state.Workers)
	assert.Positive(t, state.UpdatedAt)
}

func TestSnapshotMaintenanceCheckpointStopsBeforeFailedSource(t *testing.T) {
	t.Setenv("SNAPSHOT_MIGRATION_WORKERS", "1")
	failure := errors.New("remote checksum differs")
	state := RequestSnapshotMaintenanceState{Status: "running", Mode: "migrate", Phase: "archive_table"}
	items := []snapshotMaintenanceItem{{id: 1, size: 1}, {id: 2, size: 1}, {id: 3, size: 1}}
	process := func(ctx context.Context, _, _ string, _ bool, item snapshotMaintenanceItem) (bool, error) {
		if item.id == 2 {
			return false, failure
		}
		return true, ctx.Err()
	}
	processed, err := runRequestSnapshotMaintenanceBatch(context.Background(), &state, items, process)
	require.ErrorIs(t, err, failure)
	assert.Equal(t, 1, processed)
	assert.EqualValues(t, 1, state.Cursor, "a later success cannot skip the failed source on recovery")
	assert.EqualValues(t, 1, state.Processed)
	assert.EqualValues(t, 1, state.Scanned)
	assert.Zero(t, state.ActiveFiles)
}

func TestSnapshotMaintenancePassesDeadlineAndPreservesCanceledSource(t *testing.T) {
	t.Setenv("SNAPSHOT_MIGRATION_WORKERS", "1")
	state := RequestSnapshotMaintenanceState{Status: "running", Mode: "migrate", Phase: "archive_table"}
	process := func(ctx context.Context, _, _ string, _ bool, _ snapshotMaintenanceItem) (bool, error) {
		if _, bounded := ctx.Deadline(); !bounded {
			return false, errors.New("missing per-file deadline")
		}
		return true, nil
	}
	processed, err := runRequestSnapshotMaintenanceBatch(context.Background(), &state, []snapshotMaintenanceItem{{id: 1, size: 1}}, process)
	require.NoError(t, err)
	assert.Equal(t, 1, processed)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	processed, err = runRequestSnapshotMaintenanceBatch(ctx, &state, []snapshotMaintenanceItem{{id: 2, size: 1}}, process)
	require.ErrorIs(t, err, context.Canceled)
	assert.Zero(t, processed)
	assert.EqualValues(t, 1, state.Cursor)
}

func TestSnapshotMigrationUnknownAndOversizedBodiesRunExclusively(t *testing.T) {
	for _, size := range []int64{0, 64 * 1024 * 1024} {
		budget := snapshotMigrationMemoryBudget{limit: 32 * 1024 * 1024, changed: make(chan struct{})}
		reservation, err := budget.acquire(context.Background(), size)
		require.NoError(t, err)
		assert.Equal(t, budget.limit, reservation)
		assert.Equal(t, budget.limit, budget.used)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = budget.acquire(ctx, 1)
		require.ErrorIs(t, err, context.Canceled)
		budget.release(reservation)
		reservation, err = budget.acquire(context.Background(), 1024)
		require.NoError(t, err)
		assert.EqualValues(t, 1024, reservation)
		budget.release(reservation)
		assert.Zero(t, budget.used)
	}
}
