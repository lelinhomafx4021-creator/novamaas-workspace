package runtimewatch

import "sync/atomic"

const byteShardCount = 16

// A full 128-byte gap keeps adjacent hot fields on different cache lines even
// when the containing allocation is not cache-line aligned. No runtime-private
// APIs or architecture-specific alignment assumptions are needed.
type byteCounterShard struct {
	received atomic.Uint64
	sent     atomic.Uint64
	_        [128]byte
}

type byteCounterCell struct {
	next   atomic.Uint64
	_      [128]byte
	shards [byteShardCount]byteCounterShard
}

// ByteCounter is a reusable, allocation-free byte-counting handle for one
// stream or connection. Copies share its fixed shard. Each Add is immediately
// visible to snapshots; there is no delayed flush or per-chunk label lookup.
// The zero value is a no-op.
type ByteCounter struct {
	received *atomic.Uint64
	sent     *atomic.Uint64
}

func (counter ByteCounter) Add(received, sent int64) {
	if received > 0 && counter.received != nil {
		saturatingAdd(counter.received, uint64(received))
	}
	if sent > 0 && counter.sent != nil {
		saturatingAdd(counter.sent, uint64(sent))
	}
}

// NewByteCounter distributes streams across fixed shards once at construction.
// Keep the returned value for all request, response and replay body operations.
func (collector *Collector) NewByteCounter(scope, category string) ByteCounter {
	cell := &collector.byteCounters[counterIndex(scope, category)]
	shard := &cell.shards[(cell.next.Add(1)-1)%byteShardCount]
	return ByteCounter{received: &shard.received, sent: &shard.sent}
}

func NewByteCounter(scope, category string) ByteCounter {
	if !Enabled() {
		return ByteCounter{}
	}
	return defaultCollector.NewByteCounter(scope, category)
}
