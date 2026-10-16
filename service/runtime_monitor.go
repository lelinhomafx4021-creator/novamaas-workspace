package service

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/runtimewatch"
)

// StartRuntimeMonitoring keeps a bounded, process-local history in memory.
// The sampler also feeds admission control without writing monitoring files.
func StartRuntimeMonitoring() {
	if !runtimewatch.Enabled() {
		common.StartSystemMonitor()
		return
	}
	retentionHours := common.GetEnvOrDefault("RUNTIME_MONITOR_RETENTION_HOURS", 24)
	if retentionHours < 1 || retentionHours > 168 {
		common.SysError("RUNTIME_MONITOR_RETENTION_HOURS must be between 1 and 168; using 24")
		retentionHours = 24
	}
	err := runtimewatch.Start(context.Background(), runtimewatch.Options{
		NodeName:       common.NodeName,
		RetentionHours: retentionHours,
		SampleInterval: 5 * time.Second,
		Interfaces:     strings.Split(os.Getenv("RUNTIME_MONITOR_INTERFACES"), ","),
		OnSample: func(point runtimewatch.ResourcePoint) {
			// Keep the existing admission-control fallback when an OS read fails;
			// the monitoring report separately preserves sensor availability.
			var status common.SystemStatus
			if point.Host.CPUAvailable {
				status.CPUUsage = point.Host.CPUPercent
			}
			if point.Host.MemoryAvailable {
				status.MemoryUsage = point.Host.MemoryPercent
			}
			if disk := common.GetDiskSpaceInfo(); disk.Total > 0 {
				status.DiskUsage = disk.UsedPercent
			}
			common.SetSystemStatus(status)
		},
	})
	if err != nil {
		common.SysError("runtime monitoring could not start: " + err.Error())
	}
}

func StopRuntimeMonitoring(ctx context.Context) error {
	return runtimewatch.Stop(ctx)
}
