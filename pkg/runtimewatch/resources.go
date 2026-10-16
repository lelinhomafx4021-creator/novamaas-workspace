package runtimewatch

import (
	"context"
	"math"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/mem"
	"github.com/shirou/gopsutil/net"
	"github.com/shirou/gopsutil/process"
)

// CPU and NIC baselines belong to this sampler. They never use the global
// cpu.Percent baseline or scan process socket tables.
type systemSampler struct {
	process    *process.Process
	processCPU float64
	processAt  time.Time
	hostCPU    cpu.TimesStat
	hostReady  bool
	network    map[string]net.IOCountersStat
	networkAt  time.Time
	interfaces map[string]struct{}
}

func newSystemSampler() *systemSampler {
	proc, _ := process.NewProcess(int32(os.Getpid()))
	return &systemSampler{process: proc}
}

func (sampler *systemSampler) sample(ctx context.Context, now time.Time) ResourcePoint {
	point := ResourcePoint{Timestamp: now.Unix()}
	if sampler.process != nil {
		info, err := sampler.process.MemoryInfoWithContext(ctx)
		if err == nil && info != nil {
			point.Process.MemoryAvailable, point.Process.RSSBytes = true, info.RSS
		} else {
			point.Process.Error = "process memory unavailable"
		}
		times, err := sampler.process.TimesWithContext(ctx)
		if err == nil && times != nil {
			total := times.User + times.System
			if !sampler.processAt.IsZero() && now.After(sampler.processAt) && total >= sampler.processCPU {
				point.Process.CPUAvailable = true
				point.Process.CPUPercent = math.Min(100, 100*(total-sampler.processCPU)/now.Sub(sampler.processAt).Seconds()/float64(runtime.NumCPU()))
			}
			sampler.processCPU, sampler.processAt = total, now
		} else {
			point.Process.Error = "process CPU unavailable"
			sampler.processAt = time.Time{}
		}
	} else {
		point.Process.Error = "process metrics unavailable"
	}
	point.Process.Available = point.Process.CPUAvailable || point.Process.MemoryAvailable

	times, err := cpu.TimesWithContext(ctx, false)
	if err == nil && len(times) > 0 {
		current := times[0]
		previous := sampler.hostCPU
		total := current.User + current.System + current.Idle + current.Nice + current.Iowait + current.Irq + current.Softirq + current.Steal
		oldTotal := previous.User + previous.System + previous.Idle + previous.Nice + previous.Iowait + previous.Irq + previous.Softirq + previous.Steal
		idle := current.Idle + current.Iowait - previous.Idle - previous.Iowait
		if sampler.hostReady && total > oldTotal && idle >= 0 {
			point.Host.CPUAvailable = true
			point.Host.CPUPercent = math.Max(0, math.Min(100, 100*(1-idle/(total-oldTotal))))
		}
		sampler.hostCPU, sampler.hostReady = current, true
	} else {
		point.Host.Error = "OS CPU metrics unavailable"
		sampler.hostReady = false
	}
	if info, err := mem.VirtualMemoryWithContext(ctx); err == nil && info != nil {
		point.Host.MemoryAvailable = true
		point.Host.MemoryTotalBytes, point.Host.MemoryUsedBytes, point.Host.MemoryPercent = info.Total, info.Used, info.UsedPercent
	} else {
		point.Host.Error = "OS memory metrics unavailable"
	}
	// The OS-visible root filesystem is deliberately not described as a cgroup
	// quota or as the complete physical host in a container namespace.
	if info, err := disk.UsageWithContext(ctx, string(os.PathSeparator)); err == nil && info != nil {
		point.Host.DiskAvailable = true
		point.Host.DiskTotalBytes, point.Host.DiskUsedBytes, point.Host.DiskPercent = info.Total, info.Used, info.UsedPercent
	} else {
		point.Host.Error = "OS disk metrics unavailable"
	}
	point.Host.Available = point.Host.CPUAvailable || point.Host.MemoryAvailable || point.Host.DiskAvailable
	if counters, err := net.IOCountersWithContext(ctx, true); err == nil {
		point.Network = sampler.networkRates(counters, now)
	} else {
		point.Network.Error = "OS network metrics unavailable"
		sampler.network, sampler.networkAt = nil, time.Time{}
	}
	return point
}

func (sampler *systemSampler) networkRates(counters []net.IOCountersStat, now time.Time) NetworkResources {
	result := NetworkResources{}
	current := make(map[string]net.IOCountersStat, len(counters))
	elapsed := now.Sub(sampler.networkAt).Seconds()
	for _, counter := range counters {
		name := strings.ToLower(counter.Name)
		if name == "lo" || name == "lo0" || strings.HasPrefix(name, "loopback") {
			continue
		}
		if len(sampler.interfaces) > 0 {
			if _, selected := sampler.interfaces[counter.Name]; !selected {
				continue
			}
		}
		current[counter.Name] = counter
		previous, exists := sampler.network[counter.Name]
		if !exists || sampler.networkAt.IsZero() || elapsed <= 0 {
			continue
		}
		// Reset/hotplug observations establish a fresh per-NIC baseline. Never
		// turn unsigned wraparound or the initial cumulative total into a rate.
		if counter.BytesRecv < previous.BytesRecv || counter.BytesSent < previous.BytesSent ||
			counter.PacketsRecv < previous.PacketsRecv || counter.PacketsSent < previous.PacketsSent ||
			counter.Errin < previous.Errin || counter.Errout < previous.Errout ||
			counter.Dropin < previous.Dropin || counter.Dropout < previous.Dropout {
			continue
		}
		result.Available = true
		result.ReceivedBytesPerSecond += float64(counter.BytesRecv-previous.BytesRecv) / elapsed
		result.SentBytesPerSecond += float64(counter.BytesSent-previous.BytesSent) / elapsed
		result.ReceivedPacketsPerSecond += float64(counter.PacketsRecv-previous.PacketsRecv) / elapsed
		result.SentPacketsPerSecond += float64(counter.PacketsSent-previous.PacketsSent) / elapsed
		result.ReceiveErrorsPerSecond += float64(counter.Errin-previous.Errin) / elapsed
		result.SendErrorsPerSecond += float64(counter.Errout-previous.Errout) / elapsed
		result.ReceiveDropsPerSecond += float64(counter.Dropin-previous.Dropin) / elapsed
		result.SendDropsPerSecond += float64(counter.Dropout-previous.Dropout) / elapsed
	}
	sampler.network, sampler.networkAt = current, now
	return result
}

type resourceAggregate struct {
	sums     [17]float64
	weights  [6]uint64
	activity ActivityResources
}

func (aggregate *resourceAggregate) add(point ResourcePoint, weight uint64) {
	if point.Activity.Available {
		aggregate.activity.Available = true
		aggregate.activity.ActiveRequests = max(aggregate.activity.ActiveRequests, point.Activity.ActiveRequests)
		aggregate.activity.Connections = max(aggregate.activity.Connections, point.Activity.Connections)
	}
	amount := float64(weight)
	if point.Process.CPUAvailable {
		aggregate.sums[0] += point.Process.CPUPercent * amount
		aggregate.weights[0] += weight
	}
	if point.Process.MemoryAvailable {
		aggregate.sums[1] += float64(point.Process.RSSBytes) * amount
		aggregate.weights[1] += weight
	}
	if point.Host.CPUAvailable {
		aggregate.sums[2] += point.Host.CPUPercent * amount
		aggregate.weights[2] += weight
	}
	if point.Host.MemoryAvailable {
		aggregate.sums[3] += float64(point.Host.MemoryTotalBytes) * amount
		aggregate.sums[4] += float64(point.Host.MemoryUsedBytes) * amount
		aggregate.sums[5] += point.Host.MemoryPercent * amount
		aggregate.weights[3] += weight
	}
	if point.Host.DiskAvailable {
		aggregate.sums[6] += float64(point.Host.DiskTotalBytes) * amount
		aggregate.sums[7] += float64(point.Host.DiskUsedBytes) * amount
		aggregate.sums[8] += point.Host.DiskPercent * amount
		aggregate.weights[4] += weight
	}
	if point.Network.Available {
		values := [8]float64{point.Network.ReceivedBytesPerSecond, point.Network.SentBytesPerSecond,
			point.Network.ReceivedPacketsPerSecond, point.Network.SentPacketsPerSecond, point.Network.ReceiveErrorsPerSecond,
			point.Network.SendErrorsPerSecond, point.Network.ReceiveDropsPerSecond, point.Network.SendDropsPerSecond}
		for i, value := range values {
			aggregate.sums[i+9] += value * amount
		}
		aggregate.weights[5] += weight
	}
}

// mean averages resources and keeps the sampled activity peak for the bucket.
func (aggregate *resourceAggregate) mean(timestamp int64) ResourcePoint {
	point := ResourcePoint{Timestamp: timestamp, Activity: aggregate.activity}
	if n := aggregate.weights[0]; n > 0 {
		point.Process.CPUAvailable = true
		point.Process.CPUPercent = aggregate.sums[0] / float64(n)
	}
	if n := aggregate.weights[1]; n > 0 {
		point.Process.MemoryAvailable = true
		point.Process.RSSBytes = uint64(aggregate.sums[1] / float64(n))
	}
	if n := aggregate.weights[2]; n > 0 {
		point.Host.CPUAvailable = true
		point.Host.CPUPercent = aggregate.sums[2] / float64(n)
	}
	if n := aggregate.weights[3]; n > 0 {
		point.Host.MemoryAvailable = true
		point.Host.MemoryTotalBytes = uint64(aggregate.sums[3] / float64(n))
		point.Host.MemoryUsedBytes = uint64(aggregate.sums[4] / float64(n))
		point.Host.MemoryPercent = aggregate.sums[5] / float64(n)
	}
	if n := aggregate.weights[4]; n > 0 {
		point.Host.DiskAvailable = true
		point.Host.DiskTotalBytes = uint64(aggregate.sums[6] / float64(n))
		point.Host.DiskUsedBytes = uint64(aggregate.sums[7] / float64(n))
		point.Host.DiskPercent = aggregate.sums[8] / float64(n)
	}
	if n := aggregate.weights[5]; n > 0 {
		point.Network.Available = true
		point.Network.ReceivedBytesPerSecond = aggregate.sums[9] / float64(n)
		point.Network.SentBytesPerSecond = aggregate.sums[10] / float64(n)
		point.Network.ReceivedPacketsPerSecond = aggregate.sums[11] / float64(n)
		point.Network.SentPacketsPerSecond = aggregate.sums[12] / float64(n)
		point.Network.ReceiveErrorsPerSecond = aggregate.sums[13] / float64(n)
		point.Network.SendErrorsPerSecond = aggregate.sums[14] / float64(n)
		point.Network.ReceiveDropsPerSecond = aggregate.sums[15] / float64(n)
		point.Network.SendDropsPerSecond = aggregate.sums[16] / float64(n)
	}
	point.Process.Available = point.Process.CPUAvailable || point.Process.MemoryAvailable
	point.Host.Available = point.Host.CPUAvailable || point.Host.MemoryAvailable || point.Host.DiskAvailable
	return point
}
