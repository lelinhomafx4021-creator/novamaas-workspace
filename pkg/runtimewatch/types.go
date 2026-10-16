// Package runtimewatch collects node-local, fixed-cardinality runtime metrics.
// It deliberately has no application, database, or Redis dependencies.
package runtimewatch

import "time"

const (
	ScopeInbound    = "inbound"
	ScopeOutbound   = "outbound"
	ScopeDatabase   = "database"
	ScopeCache      = "cache"
	CategoryRelay   = "relay"
	CategoryVideo   = "video"
	CategoryAssets  = "assets"
	CategoryStorage = "storage"
	CategoryWebhook = "webhook"
	CategoryAdmin   = "admin"
	CategoryOther   = "other"
)

const (
	defaultRetentionHours = 24
	maxRetentionHours     = 7 * 24
	maxHistoryPoints      = 360
	scopeCount            = 4
	categoryCount         = 7
	cellCount             = scopeCount * categoryCount
)

var scopes = [scopeCount]string{ScopeInbound, ScopeOutbound, ScopeDatabase, ScopeCache}
var categories = [categoryCount]string{CategoryRelay, CategoryVideo, CategoryAssets, CategoryStorage, CategoryWebhook, CategoryAdmin, CategoryOther}

type Options struct {
	NodeName       string
	Interfaces     []string
	RetentionHours int
	SampleInterval time.Duration
	// OnSample runs outside the collector lock. It must return promptly.
	OnSample func(ResourcePoint)
}

type StorageStatus struct {
	Mode           string `json:"mode"`
	CapacityPoints int    `json:"capacity_points"`
}

type Meta struct {
	Started                    bool          `json:"started"`
	NodeName                   string        `json:"node_name"`
	Hostname                   string        `json:"hostname"`
	OS                         string        `json:"os"`
	Arch                       string        `json:"arch"`
	CPUCores                   int           `json:"cpu_cores"`
	NetworkInterfaces          []string      `json:"network_interfaces"`
	StartedAt                  int64         `json:"started_at"`
	SampledAt                  int64         `json:"sampled_at"`
	RequestedMinutes           int           `json:"requested_minutes"`
	WindowStart                int64         `json:"window_start"`
	WindowEnd                  int64         `json:"window_end"`
	SampleIntervalSeconds      float64       `json:"sample_interval_seconds"`
	AggregationIntervalSeconds int           `json:"aggregation_interval_seconds"`
	RetentionHours             int           `json:"retention_hours"`
	Storage                    StorageStatus `json:"storage"`
}

type ProcessResources struct {
	Available       bool   `json:"available"`
	CPUAvailable    bool   `json:"cpu_available"`
	MemoryAvailable bool   `json:"memory_available"`
	Error           string `json:"error,omitempty"`
	// CPUPercent is normalized to the OS-visible logical CPU count (0-100).
	CPUPercent float64 `json:"cpu_percent"`
	RSSBytes   uint64  `json:"rss_bytes"`
}

type HostResources struct {
	Available        bool    `json:"available"`
	CPUAvailable     bool    `json:"cpu_available"`
	MemoryAvailable  bool    `json:"memory_available"`
	DiskAvailable    bool    `json:"disk_available"`
	Error            string  `json:"error,omitempty"`
	CPUPercent       float64 `json:"cpu_percent"`
	MemoryTotalBytes uint64  `json:"memory_total_bytes"`
	MemoryUsedBytes  uint64  `json:"memory_used_bytes"`
	MemoryPercent    float64 `json:"memory_percent"`
	DiskTotalBytes   uint64  `json:"disk_total_bytes"`
	DiskUsedBytes    uint64  `json:"disk_used_bytes"`
	DiskPercent      float64 `json:"disk_percent"`
}

type NetworkResources struct {
	Available                bool    `json:"available"`
	Error                    string  `json:"error,omitempty"`
	ReceivedBytesPerSecond   float64 `json:"received_bytes_per_second"`
	SentBytesPerSecond       float64 `json:"sent_bytes_per_second"`
	ReceivedPacketsPerSecond float64 `json:"received_packets_per_second"`
	SentPacketsPerSecond     float64 `json:"sent_packets_per_second"`
	ReceiveErrorsPerSecond   float64 `json:"receive_errors_per_second"`
	SendErrorsPerSecond      float64 `json:"send_errors_per_second"`
	ReceiveDropsPerSecond    float64 `json:"receive_drops_per_second"`
	SendDropsPerSecond       float64 `json:"send_drops_per_second"`
}

type ResourcePoint struct {
	Timestamp int64             `json:"timestamp"`
	Process   ProcessResources  `json:"process"`
	Host      HostResources     `json:"host"`
	Network   NetworkResources  `json:"network"`
	Activity  ActivityResources `json:"activity"`
}

// ActivityResources contains sampled gauges. Historical aggregation keeps the
// largest observed value; operations between samples are not an exact peak.
type ActivityResources struct {
	Available bool `json:"available"`
	// ActiveRequests includes inbound HTTP requests only. Outbound, database
	// and cache operations can belong to that same request and are not added.
	ActiveRequests int64 `json:"active_requests"`
	Connections    int64 `json:"connections"`
}

type TrafficRow struct {
	Scope         string  `json:"scope"`
	Category      string  `json:"category"`
	Requests      uint64  `json:"requests"`
	Completed     uint64  `json:"completed"`
	Errors        uint64  `json:"errors"`
	Active        int64   `json:"active"`
	ReceivedBytes uint64  `json:"received_bytes"`
	SentBytes     uint64  `json:"sent_bytes"`
	Redirects     uint64  `json:"redirects"`
	DurationMS    float64 `json:"duration_ms"`
	AvgDurationMS float64 `json:"avg_duration_ms"`
	// Connections is a live socket gauge, present only on each scope's other row.
	Connections int64 `json:"connections"`
}

type HistoryPoint struct {
	Timestamp int64         `json:"timestamp"`
	Resources ResourcePoint `json:"resources"`
}

type Report struct {
	Meta    Meta           `json:"meta"`
	Current ResourcePoint  `json:"current"`
	Traffic []TrafficRow   `json:"traffic"`
	History []HistoryPoint `json:"history"`
}
