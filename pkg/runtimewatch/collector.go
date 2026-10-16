package runtimewatch

import (
	"context"
	"errors"
	"math"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type sampleRecord struct {
	Timestamp int64
	Samples   uint32
	Resources ResourcePoint
	Traffic   [cellCount]trafficValues
}

type recordRing struct {
	values   []sampleRecord
	start    int
	capacity int
}

func (ring *recordRing) append(record sampleRecord) {
	if len(ring.values) < ring.capacity {
		ring.values = append(ring.values, record)
		return
	}
	ring.values[ring.start] = record
	ring.start = (ring.start + 1) % ring.capacity
}

type minuteAggregate struct {
	minute        int64
	lastTimestamp int64
	samples       uint32
	resources     resourceAggregate
	traffic       [cellCount]trafficValues
}

// Collector owns fixed counters and a bounded minute history. Request hooks
// touch only atomics; OS reads and history run in the sampler. History is never
// persisted and is discarded when starting a new monitoring session.
type Collector struct {
	counters     [cellCount]trafficCounters
	byteCounters [cellCount]byteCounterCell
	connections  [scopeCount]atomic.Int64
	mu           sync.RWMutex
	now          func() time.Time
	meta         Meta
	current      ResourcePoint
	history      recordRing
	pending      minuteAggregate
	lastCounters [cellCount]trafficValues
	cancel       context.CancelFunc
	done         chan struct{}
	onSample     func(ResourcePoint)
	// Replaced only by deterministic package test fixtures, before Start.
	collectResources func(context.Context, time.Time) ResourcePoint
}

func NewCollector() *Collector {
	now := time.Now()
	hostname, _ := os.Hostname()
	return &Collector{now: time.Now,
		meta: Meta{Hostname: hostname, OS: runtime.GOOS, Arch: runtime.GOARCH, CPUCores: runtime.NumCPU(),
			NetworkInterfaces: []string{}, StartedAt: now.Unix(), RetentionHours: defaultRetentionHours, SampleIntervalSeconds: 5, AggregationIntervalSeconds: 60,
			Storage: StorageStatus{Mode: "memory", CapacityPoints: defaultRetentionHours * 60}},
		history: recordRing{capacity: defaultRetentionHours * 60}}
}

var defaultCollector = NewCollector()

// Start begins a fresh memory-only monitoring window. Previous counter totals
// become the baseline, while real in-flight operations and sockets remain live.
// Operations crossing the boundary complete in the new window; their duration
// still describes the original request's full lifecycle, as in time queries.
func (collector *Collector) Start(ctx context.Context, options Options) error {
	collector.mu.Lock()
	if collector.meta.Started {
		collector.mu.Unlock()
		return errors.New("runtime monitoring is already started")
	}
	if options.RetentionHours <= 0 {
		options.RetentionHours = defaultRetentionHours
	}
	if options.RetentionHours > maxRetentionHours {
		options.RetentionHours = maxRetentionHours
	}
	if options.SampleInterval <= 0 {
		options.SampleInterval = 5 * time.Second
	}
	if options.SampleInterval < time.Second {
		options.SampleInterval = time.Second
	}
	if options.SampleInterval > time.Minute {
		options.SampleInterval = time.Minute
	}
	collector.meta.NodeName = options.NodeName
	collector.meta.NetworkInterfaces = make([]string, 0, len(options.Interfaces))
	selectedInterfaces := make(map[string]struct{})
	for _, name := range options.Interfaces {
		name = strings.TrimSpace(name)
		if name == "" || len(name) > 128 || len(selectedInterfaces) >= 64 {
			continue
		}
		if _, exists := selectedInterfaces[name]; exists {
			continue
		}
		selectedInterfaces[name] = struct{}{}
		collector.meta.NetworkInterfaces = append(collector.meta.NetworkInterfaces, name)
	}
	collector.meta.RetentionHours = options.RetentionHours
	collector.meta.SampleIntervalSeconds = options.SampleInterval.Seconds()
	collector.meta.Started = true
	collector.meta.StartedAt = collector.now().Unix()
	collector.meta.SampledAt = 0
	collector.current = ResourcePoint{}
	capacity := options.RetentionHours * 60
	collector.meta.Storage = StorageStatus{Mode: "memory", CapacityPoints: capacity}
	if cap(collector.history.values) != capacity {
		collector.history.values = make([]sampleRecord, 0, capacity)
	} else {
		clear(collector.history.values)
		collector.history.values = collector.history.values[:0]
	}
	collector.history.start, collector.history.capacity = 0, capacity
	collector.pending = minuteAggregate{}
	collector.lastCounters = collector.readCounters()
	runCtx, cancel := context.WithCancel(ctx)
	collector.cancel, collector.done, collector.onSample = cancel, make(chan struct{}), options.OnSample
	done := collector.done
	collect := collector.collectResources
	collector.mu.Unlock()
	sampler := newSystemSampler()
	sampler.interfaces = selectedInterfaces
	if collect == nil {
		collect = sampler.sample
	}
	collector.sample(runCtx, collect, collector.now())
	go collector.run(runCtx, collect, options.SampleInterval, done)
	return nil
}

func (collector *Collector) run(ctx context.Context, collect func(context.Context, time.Time) ResourcePoint, interval time.Duration, done chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	defer close(done)
	for {
		select {
		case now := <-ticker.C:
			collector.sample(ctx, collect, now)
		case <-ctx.Done():
			collector.mu.Lock()
			// Capture tail bytes/completions even if the final five-second sample
			// has not happened. No OS calls are needed during shutdown.
			now := collector.now()
			collector.captureCounters(now)
			collector.flushPending()
			collector.meta.Started = false
			collector.mu.Unlock()
			return
		}
	}
}

func (collector *Collector) sample(ctx context.Context, collect func(context.Context, time.Time) ResourcePoint, now time.Time) {
	point := collect(ctx, now)
	point.Activity = collector.readActivity()
	collector.recordSample(now, point)
	if collector.onSample != nil {
		collector.onSample(point)
	}
}

func (collector *Collector) readActivity() ActivityResources {
	var requests, connections uint64
	for i := 0; i < categoryCount; i++ {
		requests = addValue(requests, uint64(collector.counters[i].active.Load()))
	}
	for i := range collector.connections {
		connections = addValue(connections, uint64(collector.connections[i].Load()))
	}
	return ActivityResources{Available: true,
		ActiveRequests: int64(min(requests, uint64(math.MaxInt64))),
		Connections:    int64(min(connections, uint64(math.MaxInt64)))}
}

// recordSample separates the deterministic accounting/retention boundary from
// OS collection and ticker scheduling.
func (collector *Collector) recordSample(now time.Time, point ResourcePoint) {
	collector.mu.Lock()
	defer collector.mu.Unlock()
	collector.captureCounters(now)
	collector.current = point
	collector.meta.SampledAt = now.Unix()
	collector.pending.resources.add(point, 1)
	collector.pending.samples++
}

func (collector *Collector) captureCounters(now time.Time) {
	minute := now.Unix() / 60
	if collector.pending.lastTimestamp != 0 && collector.pending.minute != minute {
		collector.flushPending()
	}
	collector.pending.minute, collector.pending.lastTimestamp = minute, now.Unix()
	current := collector.readCounters()
	for i, value := range current {
		collector.pending.traffic[i] = collector.pending.traffic[i].plus(value.since(collector.lastCounters[i]))
	}
	collector.lastCounters = current
}

func (collector *Collector) flushPending() {
	if collector.pending.lastTimestamp == 0 {
		return
	}
	record := sampleRecord{Timestamp: collector.pending.lastTimestamp, Samples: collector.pending.samples,
		Resources: collector.pending.resources.mean(collector.pending.lastTimestamp), Traffic: collector.pending.traffic}
	collector.history.append(record)
	collector.pending = minuteAggregate{}
}

// Stop cancels sampling, captures the final memory-only minute and waits within
// ctx. A subsequent Start discards that stopped session's history.
func (collector *Collector) Stop(ctx context.Context) error {
	collector.mu.RLock()
	cancel, done := collector.cancel, collector.done
	collector.mu.RUnlock()
	if cancel == nil {
		return nil
	}
	cancel()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (collector *Collector) Snapshot(minutes int) Report {
	collector.mu.RLock()
	defer collector.mu.RUnlock()
	if minutes <= 0 {
		minutes = 60
	}
	if minutes > collector.meta.RetentionHours*60 {
		minutes = collector.meta.RetentionHours * 60
	}
	now := collector.now()
	start := now.Add(-time.Duration(minutes) * time.Minute).Unix()
	report := Report{Meta: collector.meta, Current: collector.current, Traffic: make([]TrafficRow, 0, cellCount), History: make([]HistoryPoint, 0, maxHistoryPoints)}
	report.Meta.RequestedMinutes, report.Meta.WindowStart, report.Meta.WindowEnd = minutes, start, now.Unix()
	var totals [cellCount]trafficValues
	var bucketResources [maxHistoryPoints]resourceAggregate
	var bucketTimestamps [maxHistoryPoints]int64
	// Minute values and unsampled tail counters are deltas from this session's
	// baseline, so earlier monitoring sessions cannot leak into the new window.
	for i := 0; i < len(collector.history.values); i++ {
		record := collector.history.values[(collector.history.start+i)%len(collector.history.values)]
		if record.Timestamp < start || record.Timestamp > now.Unix() {
			continue
		}
		for j, value := range record.Traffic {
			totals[j] = totals[j].plus(value)
		}
		bucket := int((record.Timestamp - start) * maxHistoryPoints / (int64(minutes)*60 + 1))
		weight := uint64(record.Samples)
		if weight == 0 {
			weight = 1
		}
		bucketResources[bucket].add(record.Resources, weight)
		bucketTimestamps[bucket] = record.Timestamp
	}
	if collector.pending.lastTimestamp >= start && collector.pending.lastTimestamp <= now.Unix() {
		for i, value := range collector.pending.traffic {
			totals[i] = totals[i].plus(value)
		}
		bucket := int((collector.pending.lastTimestamp - start) * maxHistoryPoints / (int64(minutes)*60 + 1))
		weight := uint64(collector.pending.samples)
		if weight == 0 {
			weight = 1
		}
		bucketResources[bucket].add(collector.pending.resources.mean(collector.pending.lastTimestamp), weight)
		bucketTimestamps[bucket] = collector.pending.lastTimestamp
	}
	current := collector.readCounters()
	for i, value := range current {
		totals[i] = totals[i].plus(value.since(collector.lastCounters[i]))
		total := totals[i]
		active := collector.counters[i].active.Load()
		connections := int64(0)
		if i%categoryCount == categoryCount-1 {
			connections = collector.connections[i/categoryCount].Load()
		}
		if total == (trafficValues{}) && active == 0 && connections == 0 {
			continue
		}
		row := TrafficRow{Scope: scopes[i/categoryCount], Category: categories[i%categoryCount], Requests: total.Requests,
			Completed: total.Completed, Errors: total.Errors, Active: active, ReceivedBytes: total.Received, SentBytes: total.Sent,
			Redirects: total.Redirects, DurationMS: float64(total.Duration) / float64(time.Millisecond), Connections: connections}
		if total.Completed > 0 {
			row.AvgDurationMS = row.DurationMS / float64(total.Completed)
		}
		report.Traffic = append(report.Traffic, row)
	}
	for i, timestamp := range bucketTimestamps {
		if timestamp != 0 {
			report.History = append(report.History, HistoryPoint{Timestamp: timestamp, Resources: bucketResources[i].mean(timestamp)})
		}
	}
	return report
}

func Start(ctx context.Context, options Options) error { return defaultCollector.Start(ctx, options) }
func Stop(ctx context.Context) error                   { return defaultCollector.Stop(ctx) }
func Snapshot(minutes int) Report                      { return defaultCollector.Snapshot(minutes) }
