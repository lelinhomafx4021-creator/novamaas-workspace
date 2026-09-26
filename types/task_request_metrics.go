package types

// TaskRequestMetrics captures the submit-request path for asynchronous tasks.
// Durations are milliseconds and byte counts refer to the client payload and
// the final payload sent to the upstream provider.
type TaskRequestMetrics struct {
	RequestBodyBytes               int64 `json:"request_body_bytes"`
	UpstreamBodyBytes              int64 `json:"upstream_body_bytes"`
	BodyReadMilliseconds           int64 `json:"body_read_ms"`
	RequestPreparationMilliseconds int64 `json:"request_preparation_ms"`
	TemporaryStorageMilliseconds   int64 `json:"temporary_storage_ms"`
	UpstreamRequestMilliseconds    int64 `json:"upstream_request_ms"`
	TotalMilliseconds              int64 `json:"total_ms"`
	Attempts                       int   `json:"attempts"`
}

func (m TaskRequestMetrics) HasData() bool {
	return m != (TaskRequestMetrics{})
}
