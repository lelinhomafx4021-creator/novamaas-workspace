package runtimewatch

import (
	"testing"
	"time"

	"github.com/shirou/gopsutil/net"
	"github.com/stretchr/testify/assert"
)

func TestNetworkRatesExcludeLoopbackNewInterfacesAndCounterResets(t *testing.T) {
	sampler := &systemSampler{}
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	initial := []net.IOCountersStat{{Name: "lo0", BytesRecv: 9999}, {Name: "en0", BytesRecv: 100, BytesSent: 200, PacketsRecv: 10, PacketsSent: 20, Errin: 1, Dropout: 2}}
	assert.False(t, sampler.networkRates(initial, now).Available, "initial cumulative totals are not rates")
	second := []net.IOCountersStat{{Name: "lo0", BytesRecv: 9999999}, {Name: "en0", BytesRecv: 200, BytesSent: 400, PacketsRecv: 20, PacketsSent: 40, Errin: 3, Dropout: 6}, {Name: "new0", BytesRecv: 999999999}}
	rates := sampler.networkRates(second, now.Add(5*time.Second))
	assert.True(t, rates.Available)
	assert.Equal(t, 20.0, rates.ReceivedBytesPerSecond)
	assert.Equal(t, 40.0, rates.SentBytesPerSecond)
	assert.Equal(t, 2.0, rates.ReceivedPacketsPerSecond)
	assert.Equal(t, 4.0, rates.SentPacketsPerSecond)
	assert.Equal(t, 0.4, rates.ReceiveErrorsPerSecond)
	assert.Equal(t, 0.8, rates.SendDropsPerSecond)
	reset := []net.IOCountersStat{{Name: "en0", BytesRecv: 1, BytesSent: 1}}
	assert.False(t, sampler.networkRates(reset, now.Add(10*time.Second)).Available)
	resumed := []net.IOCountersStat{{Name: "en0", BytesRecv: 51, BytesSent: 101}}
	rates = sampler.networkRates(resumed, now.Add(15*time.Second))
	assert.True(t, rates.Available)
	assert.Equal(t, 10.0, rates.ReceivedBytesPerSecond)
	assert.Equal(t, 20.0, rates.SentBytesPerSecond)
}

func TestNetworkInterfaceAllowlistAvoidsVirtualInterfaceDoubleCounting(t *testing.T) {
	sampler := &systemSampler{interfaces: map[string]struct{}{"en0": {}}}
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	sampler.networkRates([]net.IOCountersStat{{Name: "en0", BytesRecv: 100}, {Name: "veth0", BytesRecv: 100}}, now)
	rates := sampler.networkRates([]net.IOCountersStat{{Name: "en0", BytesRecv: 200}, {Name: "veth0", BytesRecv: 200}}, now.Add(5*time.Second))
	assert.True(t, rates.Available)
	assert.Equal(t, 20.0, rates.ReceivedBytesPerSecond)
}

func TestResourceAggregationKeepsUnavailableMetricsAsGaps(t *testing.T) {
	var aggregate resourceAggregate
	aggregate.add(ResourcePoint{}, 10)
	assert.False(t, aggregate.mean(1).Process.CPUAvailable)
	assert.False(t, aggregate.mean(1).Activity.Available, "v1 or unavailable samples must remain an activity gap")
	aggregate.add(ResourcePoint{Process: ProcessResources{CPUAvailable: true, CPUPercent: 20, MemoryAvailable: true, RSSBytes: 1000}}, 1)
	aggregate.add(ResourcePoint{Process: ProcessResources{CPUAvailable: true, CPUPercent: 40}}, 3)
	point := aggregate.mean(2)
	assert.True(t, point.Process.CPUAvailable)
	assert.Equal(t, 35.0, point.Process.CPUPercent)
	assert.Equal(t, uint64(1000), point.Process.RSSBytes)
	assert.False(t, point.Host.MemoryAvailable)
	assert.False(t, point.Network.Available)
}
