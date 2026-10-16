package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/runtimewatch"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRuntimeMonitorKeepsMemoryHistoryWithoutTouchingLegacyFiles(t *testing.T) {
	previousEnabled := runtimewatch.Enabled()
	runtimewatch.SetEnabled(true)
	t.Cleanup(func() { runtimewatch.SetEnabled(previousEnabled) })
	directory := t.TempDir()
	legacyFile := filepath.Join(directory, "runtime-v1.bin")
	legacyData := []byte("previous monitoring history must remain untouched")
	require.NoError(t, os.WriteFile(legacyFile, legacyData, 0600))
	t.Setenv("RUNTIME_MONITOR_ENABLED", "true")
	t.Setenv("RUNTIME_MONITOR_DIR", directory)
	t.Setenv("RUNTIME_MONITOR_RETENTION_HOURS", "")
	previousStatus := common.GetSystemStatus()
	t.Cleanup(func() {
		require.NoError(t, StopRuntimeMonitoring(context.Background()))
		common.SetSystemStatus(previousStatus)
	})

	StartRuntimeMonitoring()
	report := runtimewatch.Snapshot(10080)
	assert.True(t, report.Meta.Started)
	assert.Equal(t, "memory", report.Meta.Storage.Mode)
	assert.Equal(t, 24, report.Meta.RetentionHours)
	assert.Equal(t, 24*60, report.Meta.Storage.CapacityPoints)
	assert.Equal(t, 24*60, report.Meta.RequestedMinutes)
	assert.NotZero(t, report.Meta.SampledAt)
	require.NoError(t, StopRuntimeMonitoring(context.Background()))
	data, err := os.ReadFile(legacyFile)
	require.NoError(t, err)
	assert.Equal(t, legacyData, data)
	entries, err := os.ReadDir(directory)
	require.NoError(t, err)
	require.Len(t, entries, 1, "monitor startup and shutdown must not create history or lock files")
	assert.Equal(t, "runtime-v1.bin", entries[0].Name())
}

func TestRuntimeMonitorBoundsConfiguredMemoryWindow(t *testing.T) {
	for _, fixture := range []struct {
		name  string
		value string
		hours int
	}{
		{name: "short window", value: "1", hours: 1},
		{name: "maximum window", value: "168", hours: 168},
		{name: "oversized falls back", value: "169", hours: 24},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			previousEnabled := runtimewatch.Enabled()
			runtimewatch.SetEnabled(true)
			t.Cleanup(func() { runtimewatch.SetEnabled(previousEnabled) })
			t.Setenv("RUNTIME_MONITOR_ENABLED", "true")
			t.Setenv("RUNTIME_MONITOR_RETENTION_HOURS", fixture.value)
			previousStatus := common.GetSystemStatus()
			t.Cleanup(func() {
				require.NoError(t, StopRuntimeMonitoring(context.Background()))
				common.SetSystemStatus(previousStatus)
			})

			StartRuntimeMonitoring()
			report := runtimewatch.Snapshot(10080)
			assert.Equal(t, fixture.hours, report.Meta.RetentionHours)
			assert.Equal(t, fixture.hours*60, report.Meta.Storage.CapacityPoints)
			assert.Equal(t, fixture.hours*60, report.Meta.RequestedMinutes)
		})
	}
}
