package runtimewatch

import (
	"context"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func trafficRow(t *testing.T, report Report, scope, category string) TrafficRow {
	t.Helper()
	for _, row := range report.Traffic {
		if row.Scope == scope && row.Category == category {
			return row
		}
	}
	t.Fatalf("missing traffic row %s/%s", scope, category)
	return TrafficRow{}
}

func fixtureResources(_ context.Context, now time.Time) ResourcePoint {
	return ResourcePoint{Timestamp: now.Unix(), Process: ProcessResources{Available: true, CPUAvailable: true, MemoryAvailable: true, CPUPercent: 12, RSSBytes: 1024},
		Host:    HostResources{Available: true, MemoryAvailable: true, MemoryTotalBytes: 8192, MemoryUsedBytes: 4096, MemoryPercent: 50},
		Network: NetworkResources{Available: true, ReceivedBytesPerSecond: 200, SentBytesPerSecond: 100}}
}

func TestTrafficCompletionIsConcurrentAndIdempotent(t *testing.T) {
	collector := NewCollector()
	statuses := []int{200, 0, 302, 400, 503, -1}
	var workers sync.WaitGroup
	for _, status := range statuses {
		workers.Add(1)
		go func(status int) {
			defer workers.Done()
			finish := collector.Begin(ScopeOutbound, CategoryVideo)
			collector.AddBytes(ScopeOutbound, CategoryVideo, 10, 4)
			collector.AddBytes(ScopeOutbound, CategoryVideo, -1, -1)
			finish(status)
			finish(500)
		}(status)
	}
	workers.Wait()
	row := trafficRow(t, collector.Snapshot(60), ScopeOutbound, CategoryVideo)
	assert.Equal(t, uint64(6), row.Requests)
	assert.Equal(t, uint64(6), row.Completed)
	assert.Equal(t, uint64(3), row.Errors)
	assert.Equal(t, uint64(60), row.ReceivedBytes)
	assert.Equal(t, uint64(24), row.SentBytes)
	assert.Zero(t, row.Active)
	assert.GreaterOrEqual(t, row.DurationMS, 0.0)
	assert.False(t, collector.Snapshot(60).Meta.Started)

	finish := collector.Begin(ScopeInbound, "https://private.example/request-id")
	collector.AddRedirect(ScopeInbound, "unknown")
	assert.Equal(t, int64(1), trafficRow(t, collector.Snapshot(60), ScopeInbound, CategoryOther).Active)
	finish(200)
	assert.Equal(t, uint64(1), trafficRow(t, collector.Snapshot(60), ScopeInbound, CategoryOther).Redirects)
}

func TestTrafficCountersAndConnectionGaugesNeverWrap(t *testing.T) {
	collector := NewCollector()
	cell := &collector.counters[counterIndex(ScopeCache, CategoryOther)]
	cell.received.Store(math.MaxUint64 - 3)
	collector.AddBytes(ScopeCache, CategoryOther, 10, 0)
	collector.TrackConnection(ScopeCache, math.MaxInt64)
	collector.TrackConnection(ScopeCache, 2)
	assert.Equal(t, uint64(math.MaxUint64), trafficRow(t, collector.Snapshot(60), ScopeCache, CategoryOther).ReceivedBytes)
	assert.Equal(t, int64(math.MaxInt64), trafficRow(t, collector.Snapshot(60), ScopeCache, CategoryOther).Connections)
	collector.TrackConnection(ScopeCache, math.MinInt64)
	collector.TrackConnection(ScopeCache, -1)
	assert.Zero(t, trafficRow(t, collector.Snapshot(60), ScopeCache, CategoryOther).Connections)
}

func TestHistoryDownsamplingAndWindowTrafficPreserveTotals(t *testing.T) {
	collector := NewCollector()
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	collector.now = func() time.Time { return now }
	// One more minute than the maximum response point count exercises the
	// downsampling boundary, while counts must retain every selected operation.
	for i := 0; i <= maxHistoryPoints; i++ {
		at := now.Add(-time.Duration(maxHistoryPoints-i) * time.Minute)
		resources := fixtureResources(context.Background(), at)
		resources.Process.CPUPercent = float64(i%2) * 20
		if i == 0 {
			resources.Activity = ActivityResources{Available: true, ActiveRequests: 3, Connections: 9}
		}
		if i == 1 {
			resources.Activity = ActivityResources{Available: true, ActiveRequests: 7, Connections: 2}
		}
		record := sampleRecord{Timestamp: at.Unix(), Samples: 1, Resources: resources}
		record.Traffic[counterIndex(ScopeOutbound, CategoryRelay)] = trafficValues{Requests: 1, Completed: 1, Received: 5}
		collector.history.append(record)
	}
	report := collector.Snapshot(maxHistoryPoints)
	assert.Len(t, report.History, maxHistoryPoints)
	row := trafficRow(t, report, ScopeOutbound, CategoryRelay)
	assert.Equal(t, uint64(maxHistoryPoints+1), row.Requests)
	assert.Equal(t, uint64((maxHistoryPoints+1)*5), row.ReceivedBytes)
	assert.Equal(t, now.Add(-maxHistoryPoints*time.Minute).Unix(), report.Meta.WindowStart)
	assert.Equal(t, ActivityResources{Available: true, ActiveRequests: 7, Connections: 9}, report.History[0].Resources.Activity,
		"downsampling must preserve independently sampled activity peaks")
	assert.Equal(t, 24*60, collector.Snapshot(math.MaxInt).Meta.RequestedMinutes)
	assert.Equal(t, 60, collector.Snapshot(0).Meta.RequestedMinutes)
	for i := 1; i < len(report.History); i++ {
		assert.Greater(t, report.History[i].Timestamp, report.History[i-1].Timestamp)
	}
	assert.Less(t, trafficRow(t, collector.Snapshot(1), ScopeOutbound, CategoryRelay).Requests, row.Requests)
}

func TestMinuteRingBoundsAndExpiredHistoryDoesNotLeakIntoWindow(t *testing.T) {
	collector := NewCollector()
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	collector.now = func() time.Time { return now }
	collector.meta.RetentionHours = 1
	collector.history = recordRing{values: make([]sampleRecord, 0, 60), capacity: 60}
	for i := 60; i >= 0; i-- {
		at := now.Add(-time.Duration(i) * time.Minute)
		record := sampleRecord{Timestamp: at.Unix(), Samples: 1, Resources: fixtureResources(context.Background(), at)}
		record.Traffic[0].Requests = 1
		collector.history.append(record)
	}
	assert.Len(t, collector.history.values, 60)
	assert.Equal(t, 60, cap(collector.history.values), "minute rollover must not grow the reserved history backing array")
	report := collector.Snapshot(10080)
	assert.Equal(t, 60, report.Meta.RequestedMinutes)
	assert.Equal(t, uint64(60), trafficRow(t, report, ScopeInbound, CategoryRelay).Requests)
	now = now.Add(2 * time.Hour)
	assert.Empty(t, collector.Snapshot(60).Traffic)
	assert.Empty(t, collector.Snapshot(60).History)
}

func TestMemorySessionsResetHistoryAndKeepLiveBoundaryOperations(t *testing.T) {
	now := time.Date(2026, 10, 15, 12, 0, 5, 0, time.UTC)
	collector := NewCollector()
	collector.now = func() time.Time { return now }
	collector.collectResources = fixtureResources
	collector.Begin(ScopeOutbound, CategoryStorage)(200)
	collector.AddBytes(ScopeOutbound, CategoryStorage, 113, 7)
	finishInbound := collector.Begin(ScopeInbound, CategoryRelay)
	collector.TrackConnection(ScopeDatabase, 3)
	callbackCount := 0
	require.NoError(t, collector.Start(context.Background(), Options{RetentionHours: 1, SampleInterval: time.Minute,
		Interfaces: []string{" en0 ", "en0"}, OnSample: func(point ResourcePoint) {
			callbackCount++
			assert.Equal(t, point, collector.Snapshot(60).Current, "callback must run outside the collector lock")
		}}))
	assert.Equal(t, 1, callbackCount)
	report := collector.Snapshot(60)
	assert.Equal(t, []string{"en0"}, report.Meta.NetworkInterfaces)
	assert.Equal(t, ActivityResources{Available: true, ActiveRequests: 1, Connections: 3}, report.Current.Activity)
	assert.Zero(t, trafficRow(t, report, ScopeInbound, CategoryRelay).Requests, "pre-Start operation starts belong to the baseline")
	assert.Len(t, report.Traffic, 2, "only live gauges remain from before Start")
	finishInbound(500)
	collector.TrackConnection(ScopeDatabase, -3)
	collector.Begin(ScopeOutbound, CategoryStorage)(200)
	collector.AddBytes(ScopeOutbound, CategoryStorage, 5, 3)
	boundary := collector.Begin(ScopeInbound, CategoryVideo)
	collector.TrackConnection(ScopeOutbound, 2)
	now = now.Add(10 * time.Second)
	require.NoError(t, collector.Stop(context.Background()))
	report = collector.Snapshot(60)
	assert.False(t, report.Meta.Started)
	assert.Equal(t, uint64(5), trafficRow(t, report, ScopeOutbound, CategoryStorage).ReceivedBytes, "shutdown must capture the unsampled tail")
	assert.NotEmpty(t, report.History)
	// Events while stopped become part of the next baseline, not the next
	// monitored window. Real in-flight gauges must survive that same reset.
	collector.Begin(ScopeOutbound, CategoryStorage)(500)
	collector.AddBytes(ScopeOutbound, CategoryStorage, 60, 30)
	now = now.Add(time.Minute)
	require.NoError(t, collector.Start(context.Background(), Options{RetentionHours: 1, SampleInterval: time.Minute}))
	t.Cleanup(func() { require.NoError(t, collector.Stop(context.Background())) })
	report = collector.Snapshot(60)
	assert.Equal(t, now.Unix(), report.Meta.StartedAt)
	assert.Equal(t, now.Unix(), report.Meta.SampledAt)
	assert.Equal(t, now.Unix(), report.Current.Timestamp)
	require.Len(t, report.History, 1, "only the new session's initial sample is present")
	assert.Equal(t, now.Unix(), report.History[0].Timestamp)
	assert.Equal(t, ActivityResources{Available: true, ActiveRequests: 1, Connections: 2}, report.Current.Activity)
	assert.Len(t, report.Traffic, 2, "old counts and stopped-interval traffic must not return")
	collector.AddBytes(ScopeInbound, CategoryVideo, 7, 2)
	boundary(500)
	collector.TrackConnection(ScopeOutbound, -2)
	row := trafficRow(t, collector.Snapshot(60), ScopeInbound, CategoryVideo)
	assert.Zero(t, row.Requests)
	assert.Equal(t, uint64(1), row.Completed, "a request crossing Start can naturally finish in the new window")
	assert.Equal(t, uint64(1), row.Errors)
	assert.Equal(t, uint64(7), row.ReceivedBytes)
	assert.Zero(t, row.Active)
}

func TestActivitySamplesInboundRequestsAndAllTrackedSocketScopes(t *testing.T) {
	collector := NewCollector()
	now := time.Date(2026, 10, 16, 12, 0, 5, 0, time.UTC)
	collector.now = func() time.Time { return now }
	first := collector.Begin(ScopeInbound, CategoryRelay)
	second := collector.Begin(ScopeInbound, CategoryAdmin)
	outbound := collector.Begin(ScopeOutbound, CategoryVideo)
	database := collector.Begin(ScopeDatabase, CategoryOther)
	cache := collector.Begin(ScopeCache, CategoryOther)
	collector.TrackConnection(ScopeInbound, 2)
	collector.TrackConnection(ScopeOutbound, 3)
	collector.TrackConnection(ScopeDatabase, 4)
	collector.TrackConnection(ScopeCache, 5)
	collector.sample(context.Background(), fixtureResources, now)
	assert.Equal(t, ActivityResources{Available: true, ActiveRequests: 2, Connections: 14}, collector.Snapshot(60).Current.Activity)
	first(200)
	collector.TrackConnection(ScopeOutbound, 6)
	now = now.Add(5 * time.Second)
	collector.sample(context.Background(), fixtureResources, now)
	assert.Equal(t, ActivityResources{Available: true, ActiveRequests: 1, Connections: 20}, collector.Snapshot(60).Current.Activity)
	second(200)
	outbound(200)
	database(0)
	cache(0)
	collector.TrackConnection(ScopeInbound, -2)
	collector.TrackConnection(ScopeOutbound, -9)
	collector.TrackConnection(ScopeDatabase, -4)
	collector.TrackConnection(ScopeCache, -5)
	now = now.Add(time.Minute)
	collector.sample(context.Background(), fixtureResources, now)
	report := collector.Snapshot(60)
	assert.Equal(t, ActivityResources{Available: true}, report.Current.Activity)
	require.NotEmpty(t, report.History)
	assert.Equal(t, ActivityResources{Available: true, ActiveRequests: 2, Connections: 20}, report.History[0].Resources.Activity,
		"minute history must retain sampled peaks rather than the final or averaged gauge")
}

func TestNewCollectorHasNoPreviousSessionToRestore(t *testing.T) {
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	first := NewCollector()
	first.now = func() time.Time { return now }
	first.collectResources = fixtureResources
	require.NoError(t, first.Start(context.Background(), Options{SampleInterval: time.Minute}))
	first.Begin(ScopeInbound, CategoryAdmin)(200)
	first.AddBytes(ScopeInbound, CategoryAdmin, 20, 30)
	require.NoError(t, first.Stop(context.Background()))
	assert.Equal(t, uint64(20), trafficRow(t, first.Snapshot(60), ScopeInbound, CategoryAdmin).ReceivedBytes)
	assert.NotEmpty(t, first.Snapshot(60).History)
	second := NewCollector()
	second.now = func() time.Time { return now }
	second.collectResources = fixtureResources
	assert.Empty(t, second.Snapshot(60).History)
	assert.Empty(t, second.Snapshot(60).Traffic)
	now = now.Add(time.Minute)
	require.NoError(t, second.Start(context.Background(), Options{SampleInterval: time.Minute}))
	t.Cleanup(func() { require.NoError(t, second.Stop(context.Background())) })
	report := second.Snapshot(60)
	assert.True(t, report.Meta.Started)
	assert.Equal(t, StorageStatus{Mode: "memory", CapacityPoints: 24 * 60}, report.Meta.Storage)
	assert.Empty(t, report.Traffic)
	require.Len(t, report.History, 1)
	assert.Equal(t, now.Unix(), report.History[0].Timestamp)
}

func TestMemoryRetentionPreallocatesAndBoundsTheHistoryWindow(t *testing.T) {
	for _, test := range []struct {
		name            string
		hours, expected int
	}{
		{name: "default day", expected: 24},
		{name: "one hour", hours: 1, expected: 1},
		{name: "seven days", hours: 168, expected: 168},
		{name: "capped retention", hours: 169, expected: 168},
	} {
		t.Run(test.name, func(t *testing.T) {
			collector := NewCollector()
			collector.collectResources = fixtureResources
			require.NoError(t, collector.Start(context.Background(), Options{RetentionHours: test.hours, SampleInterval: time.Minute}))
			t.Cleanup(func() { require.NoError(t, collector.Stop(context.Background())) })
			report := collector.Snapshot(10080)
			assert.Equal(t, test.expected, report.Meta.RetentionHours)
			assert.Equal(t, StorageStatus{Mode: "memory", CapacityPoints: test.expected * 60}, report.Meta.Storage)
			assert.Equal(t, test.expected*60, report.Meta.RequestedMinutes)
			assert.Equal(t, test.expected*60, cap(collector.history.values), "the minute ring must reserve a fixed backing array at startup")
		})
	}
}

func BenchmarkTrafficAccounting(b *testing.B) {
	collector := NewCollector()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		finish := collector.Begin(ScopeOutbound, CategoryRelay)
		collector.AddBytes(ScopeOutbound, CategoryRelay, 4096, 512)
		collector.AddBytes(ScopeOutbound, CategoryRelay, 4096, 0)
		finish(200)
	}
}
