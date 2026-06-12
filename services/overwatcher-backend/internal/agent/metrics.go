package agent

import (
	"log/slog"
	"sync"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"

	"github.com/lwlee2608/overwatcher/internal/protocol"
)

var metricsWarnOnce sync.Once

// collectHostMetrics snapshots CPU, memory and root-filesystem disk usage.
// Returns nil when collection fails (e.g. unsupported platform) — metrics are
// best-effort and must never get in the way of polling. The failure is logged
// once, not every poll.
func collectHostMetrics() *protocol.HostMetrics {
	// Interval 0 reports usage since the previous call, i.e. averaged over the
	// poll interval. The first reading after startup covers process start to
	// now, which is close enough.
	cpuPcts, cpuErr := cpu.Percent(0, false)
	vm, memErr := mem.VirtualMemory()
	du, diskErr := disk.Usage("/")
	if cpuErr != nil || memErr != nil || diskErr != nil || len(cpuPcts) == 0 {
		metricsWarnOnce.Do(func() {
			slog.Warn("host metrics collection failed, not reporting metrics",
				"cpu_error", cpuErr, "mem_error", memErr, "disk_error", diskErr)
		})
		return nil
	}
	return &protocol.HostMetrics{
		CPUPercent:     cpuPcts[0],
		MemUsedBytes:   vm.Used,
		MemTotalBytes:  vm.Total,
		DiskUsedBytes:  du.Used,
		DiskTotalBytes: du.Total,
	}
}
