package runtimewatch

import (
	"context"
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestByteCounterShardsAndLegacyBytesAreImmediatelyVisible(t *testing.T) {
	collector := NewCollector()
	seen := make(map[*atomic.Uint64]struct{})
	for i := 1; i <= byteShardCount+1; i++ {
		counter := collector.NewByteCounter(ScopeOutbound, CategoryVideo)
		seen[counter.received] = struct{}{}
		counter.Add(int64(i), int64(2*i))
	}
	assert.Greater(t, len(seen), 1, "independent streams must distribute their byte updates")
	collector.AddBytes(ScopeOutbound, CategoryVideo, 7, 11)
	row := trafficRow(t, collector.Snapshot(60), ScopeOutbound, CategoryVideo)
	count := uint64((byteShardCount + 1) * (byteShardCount + 2) / 2)
	assert.Equal(t, count+7, row.ReceivedBytes)
	assert.Equal(t, count*2+11, row.SentBytes)
	assert.Zero(t, row.Completed, "bytes must be observable without EOF, Close or request completion")
	var disabled ByteCounter
	before := collector.readCounters()
	disabled.Add(math.MaxInt64, math.MaxInt64)
	collector.NewByteCounter(ScopeOutbound, CategoryVideo).Add(-1, 0)
	assert.Equal(t, before, collector.readCounters(), "zero handles and non-positive amounts must not change bytes")
}

func TestByteCounterSaturatesBothIndividualShardsAndCombinedTotals(t *testing.T) {
	collector := NewCollector()
	first := collector.NewByteCounter(ScopeCache, CategoryOther)
	second := collector.NewByteCounter(ScopeCache, CategoryOther)
	first.received.Store(math.MaxUint64 - 3)
	first.sent.Store(math.MaxUint64 - 3)
	first.Add(9, 9)
	second.Add(5, 7)
	collector.AddBytes(ScopeCache, CategoryOther, 3, 4)
	row := trafficRow(t, collector.Snapshot(60), ScopeCache, CategoryOther)
	assert.Equal(t, uint64(math.MaxUint64), row.ReceivedBytes)
	assert.Equal(t, uint64(math.MaxUint64), row.SentBytes)
	first.Add(1, 1)
	assert.Equal(t, row, trafficRow(t, collector.Snapshot(60), ScopeCache, CategoryOther), "further updates must not wrap saturated totals")
}

func TestByteCounterSupportsSharedDuplexHandlesAndConcurrentStreams(t *testing.T) {
	collector := NewCollector()
	handles := [3]ByteCounter{
		collector.NewByteCounter(ScopeOutbound, CategoryRelay),
		collector.NewByteCounter(ScopeOutbound, CategoryRelay),
		collector.NewByteCounter(ScopeOutbound, CategoryRelay),
	}
	var workers sync.WaitGroup
	start := make(chan struct{})
	for worker := 0; worker < 6; worker++ {
		workers.Add(1)
		go func(index int) {
			defer workers.Done()
			<-start
			finish := collector.Begin(ScopeOutbound, CategoryRelay)
			handle := handles[index%len(handles)]
			handle.Add(3, 1)
			handle.Add(5, 2)
			handle.Add(7, 3)
			if index%2 == 0 {
				finish(500)
			} else {
				finish(200)
			}
			finish(500)
		}(worker)
	}
	close(start)
	workers.Wait()
	row := trafficRow(t, collector.Snapshot(60), ScopeOutbound, CategoryRelay)
	assert.Equal(t, uint64(90), row.ReceivedBytes)
	assert.Equal(t, uint64(36), row.SentBytes)
	assert.Equal(t, uint64(6), row.Requests)
	assert.Equal(t, uint64(6), row.Completed)
	assert.Equal(t, uint64(3), row.Errors)
	assert.Zero(t, row.Active)
}

func TestByteCounterRemainsValidAcrossMemorySessionBoundaries(t *testing.T) {
	collector := NewCollector()
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	collector.now = func() time.Time { return now }
	collector.collectResources = fixtureResources
	handle := collector.NewByteCounter(ScopeDatabase, CategoryOther)
	handle.Add(100, 200)
	require.NoError(t, collector.Start(context.Background(), Options{SampleInterval: time.Minute}))
	handle.Add(5, 7)
	row := trafficRow(t, collector.Snapshot(60), ScopeDatabase, CategoryOther)
	assert.Equal(t, uint64(5), row.ReceivedBytes)
	assert.Equal(t, uint64(7), row.SentBytes)
	require.NoError(t, collector.Stop(context.Background()))
	assert.Equal(t, row, trafficRow(t, collector.Snapshot(60), ScopeDatabase, CategoryOther), "shutdown must capture shard bytes in its unsampled tail")
	handle.Add(20, 30)
	now = now.Add(time.Minute)
	require.NoError(t, collector.Start(context.Background(), Options{SampleInterval: time.Minute}))
	t.Cleanup(func() { require.NoError(t, collector.Stop(context.Background())) })
	assert.Empty(t, collector.Snapshot(60).Traffic)
	handle.Add(3, 4)
	row = trafficRow(t, collector.Snapshot(60), ScopeDatabase, CategoryOther)
	assert.Equal(t, uint64(3), row.ReceivedBytes)
	assert.Equal(t, uint64(4), row.SentBytes)
}

func TestDisabledGlobalsBypassCountersAndConnectionsButInstancesStillWork(t *testing.T) {
	previous := Enabled()
	SetEnabled(false)
	t.Cleanup(func() { SetEnabled(previous) })
	before := defaultCollector.readCounters()
	active := defaultCollector.readActivity()
	Begin(ScopeInbound, CategoryRelay)(500)
	AddBytes(ScopeInbound, CategoryRelay, 11, 13)
	AddRedirect(ScopeInbound, CategoryRelay)
	TrackConnection(ScopeInbound, 1)
	NewByteCounter(ScopeInbound, CategoryRelay).Add(17, 19)
	underlying := &fixtureConn{}
	assert.Same(t, underlying, WrapConn(ScopeDatabase, underlying))
	assert.Same(t, underlying, WrapConnection(ScopeOutbound, underlying))
	assert.Equal(t, before, defaultCollector.readCounters())
	assert.Equal(t, active, defaultCollector.readActivity())
	collector := NewCollector()
	collector.Begin(ScopeInbound, CategoryRelay)(200)
	collector.NewByteCounter(ScopeInbound, CategoryRelay).Add(5, 7)
	row := trafficRow(t, collector.Snapshot(60), ScopeInbound, CategoryRelay)
	assert.Equal(t, uint64(1), row.Completed)
	assert.Equal(t, uint64(5), row.ReceivedBytes)
	assert.Equal(t, uint64(7), row.SentBytes)
}

func TestExistingHandlesFinishTheirLifecycleAfterStartupFlagChanges(t *testing.T) {
	previous := Enabled()
	SetEnabled(true)
	t.Cleanup(func() { SetEnabled(previous) })
	before := defaultCollector.readCounters()[counterIndex(ScopeOutbound, CategoryVideo)]
	connectionCount := defaultCollector.connections[counterIndex(ScopeDatabase, CategoryOther)/categoryCount].Load()
	handle := NewByteCounter(ScopeOutbound, CategoryVideo)
	finish := Begin(ScopeOutbound, CategoryVideo)
	connection := WrapConn(ScopeDatabase, &fixtureConn{})
	SetEnabled(false)
	handle.Add(13, 17)
	finish(200)
	require.NoError(t, connection.Close())
	require.NoError(t, connection.Close())
	after := defaultCollector.readCounters()[counterIndex(ScopeOutbound, CategoryVideo)]
	assert.Equal(t, uint64(13), after.Received-before.Received)
	assert.Equal(t, uint64(17), after.Sent-before.Sent)
	assert.Equal(t, uint64(1), after.Completed-before.Completed)
	assert.Equal(t, connectionCount, defaultCollector.connections[counterIndex(ScopeDatabase, CategoryOther)/categoryCount].Load())
}
