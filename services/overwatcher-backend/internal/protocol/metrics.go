package protocol

// HostMetricsHeader carries a compact-JSON HostMetrics snapshot on every
// agent request, piggybacking on the poll loop the same way X-Agent-Type and
// X-Agent-Version do. Descriptive metadata only — identity is the token.
const HostMetricsHeader = "X-Agent-Metrics"

// HostMetrics is a point-in-time snapshot of the agent VM's resources.
// Disk figures are for the root filesystem.
type HostMetrics struct {
	CPUPercent     float64 `json:"cpu_percent"`
	MemUsedBytes   uint64  `json:"mem_used_bytes"`
	MemTotalBytes  uint64  `json:"mem_total_bytes"`
	DiskUsedBytes  uint64  `json:"disk_used_bytes"`
	DiskTotalBytes uint64  `json:"disk_total_bytes"`
}
