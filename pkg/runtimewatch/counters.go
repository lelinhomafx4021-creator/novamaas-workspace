package runtimewatch

import (
	"math"
	"sync/atomic"
	"time"
)

type trafficCounters struct {
	requests  atomic.Uint64
	completed atomic.Uint64
	errors    atomic.Uint64
	received  atomic.Uint64
	sent      atomic.Uint64
	redirects atomic.Uint64
	duration  atomic.Uint64
	active    atomic.Int64
}

type trafficValues struct {
	Requests  uint64
	Completed uint64
	Errors    uint64
	Received  uint64
	Sent      uint64
	Redirects uint64
	Duration  uint64
}

// Invalid labels collapse into the fixed other bucket; arbitrary labels never
// allocate a series or retain a request path, URL, address, or payload.
func counterIndex(scope, category string) int {
	scopeIndex := 0
	for i, value := range scopes {
		if scope == value {
			scopeIndex = i
			break
		}
	}
	categoryIndex := categoryCount - 1
	for i, value := range categories {
		if category == value {
			categoryIndex = i
			break
		}
	}
	return scopeIndex*categoryCount + categoryIndex
}

func saturatingAdd(counter *atomic.Uint64, amount uint64) {
	for {
		previous := counter.Load()
		next := previous + amount
		if next < previous {
			next = math.MaxUint64
		}
		if counter.CompareAndSwap(previous, next) {
			return
		}
	}
}

func addGauge(counter *atomic.Int64, delta int64) {
	for {
		previous := counter.Load()
		next := int64(0)
		if delta > 0 {
			if previous > math.MaxInt64-delta {
				next = math.MaxInt64
			} else {
				next = previous + delta
			}
		} else if delta > -previous {
			next = previous + delta
		}
		if counter.CompareAndSwap(previous, next) {
			return
		}
	}
}

func addValue(a, b uint64) uint64 {
	if math.MaxUint64-a < b {
		return math.MaxUint64
	}
	return a + b
}

func (values trafficValues) plus(other trafficValues) trafficValues {
	return trafficValues{
		Requests: addValue(values.Requests, other.Requests), Completed: addValue(values.Completed, other.Completed),
		Errors: addValue(values.Errors, other.Errors), Received: addValue(values.Received, other.Received),
		Sent: addValue(values.Sent, other.Sent), Redirects: addValue(values.Redirects, other.Redirects),
		Duration: addValue(values.Duration, other.Duration),
	}
}

func (values trafficValues) since(previous trafficValues) trafficValues {
	// Each counter is monotonic and saturates instead of wrapping.
	return trafficValues{Requests: values.Requests - previous.Requests, Completed: values.Completed - previous.Completed,
		Errors: values.Errors - previous.Errors, Received: values.Received - previous.Received, Sent: values.Sent - previous.Sent,
		Redirects: values.Redirects - previous.Redirects, Duration: values.Duration - previous.Duration}
}

func (collector *Collector) readCounters() [cellCount]trafficValues {
	var values [cellCount]trafficValues
	for i := range collector.counters {
		cell := &collector.counters[i]
		values[i] = trafficValues{Requests: cell.requests.Load(), Completed: cell.completed.Load(), Errors: cell.errors.Load(),
			Received: cell.received.Load(), Sent: cell.sent.Load(), Redirects: cell.redirects.Load(), Duration: cell.duration.Load()}
		for j := range collector.byteCounters[i].shards {
			shard := &collector.byteCounters[i].shards[j]
			values[i].Received = addValue(values[i].Received, shard.received.Load())
			values[i].Sent = addValue(values[i].Sent, shard.sent.Load())
		}
	}
	return values
}

// Begin accounts an operation from start through streaming body completion.
// Finish is safe to call more than once. Status >=400 or <0 counts as an error;
// status 0 supports successful database/cache operations without HTTP status.
func (collector *Collector) Begin(scope, category string) func(status int) {
	cell := &collector.counters[counterIndex(scope, category)]
	saturatingAdd(&cell.requests, 1)
	addGauge(&cell.active, 1)
	started := time.Now()
	var finished atomic.Bool
	return func(status int) {
		if !finished.CompareAndSwap(false, true) {
			return
		}
		addGauge(&cell.active, -1)
		saturatingAdd(&cell.completed, 1)
		if status >= 400 || status < 0 {
			saturatingAdd(&cell.errors, 1)
		}
		saturatingAdd(&cell.duration, uint64(time.Since(started)))
	}
}

func (collector *Collector) AddBytes(scope, category string, received, sent int64) {
	cell := &collector.counters[counterIndex(scope, category)]
	if received > 0 {
		saturatingAdd(&cell.received, uint64(received))
	}
	if sent > 0 {
		saturatingAdd(&cell.sent, uint64(sent))
	}
}

func (collector *Collector) AddRedirect(scope, category string) {
	saturatingAdd(&collector.counters[counterIndex(scope, category)].redirects, 1)
}

func (collector *Collector) TrackConnection(scope string, delta int64) {
	addGauge(&collector.connections[counterIndex(scope, CategoryOther)/categoryCount], delta)
}

func Begin(scope, category string) func(status int) {
	if !Enabled() {
		return func(int) {}
	}
	return defaultCollector.Begin(scope, category)
}
func AddBytes(scope, category string, received, sent int64) {
	if !Enabled() {
		return
	}
	defaultCollector.AddBytes(scope, category, received, sent)
}
func AddRedirect(scope, category string) {
	if Enabled() {
		defaultCollector.AddRedirect(scope, category)
	}
}
func TrackConnection(scope string, delta int64) {
	if Enabled() {
		defaultCollector.TrackConnection(scope, delta)
	}
}
