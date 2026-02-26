package monitor

import (
	"runtime"
	"sync/atomic"
	"time"
)

// PerformanceMonitor 性能监控器
type PerformanceMonitor struct {
	// 计数器
	totalConnections  atomic.Int64
	activeConnections atomic.Int64
	totalRequests     atomic.Int64
	totalBytes        atomic.Int64

	// 时间统计
	startTime     time.Time
	lastResetTime atomic.Int64

	// 延迟统计
	totalResponseTime atomic.Int64 // 纳秒
	responseCount     atomic.Int64
}

// NewPerformanceMonitor 创建性能监控器
func NewPerformanceMonitor() *PerformanceMonitor {
	now := time.Now()
	return &PerformanceMonitor{
		startTime:     now,
		lastResetTime: atomic.Int64{},
	}
}

// RecordConnection 记录连接
func (pm *PerformanceMonitor) RecordConnection() {
	pm.totalConnections.Add(1)
	pm.activeConnections.Add(1)
}

// RecordConnectionClose 记录连接关闭
func (pm *PerformanceMonitor) RecordConnectionClose() {
	pm.activeConnections.Add(-1)
}

// RecordRequest 记录请求
func (pm *PerformanceMonitor) RecordRequest(bytes int, responseTime time.Duration) {
	pm.totalRequests.Add(1)
	pm.totalBytes.Add(int64(bytes))
	pm.totalResponseTime.Add(int64(responseTime))
	pm.responseCount.Add(1)
}

// GetStats 获取性能统计
func (pm *PerformanceMonitor) GetStats() map[string]any {
	now := time.Now()
	uptime := now.Sub(pm.startTime)

	// 内存统计
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// 计算平均响应时间
	totalRespTime := pm.totalResponseTime.Load()
	respCount := pm.responseCount.Load()
	var avgResponseTime float64
	if respCount > 0 {
		avgResponseTime = float64(totalRespTime) / float64(respCount) / float64(time.Millisecond)
	}

	// 计算请求速率
	totalReqs := pm.totalRequests.Load()
	requestsPerSecond := float64(totalReqs) / uptime.Seconds()

	return map[string]any{
		// 连接统计
		"total_connections":    pm.totalConnections.Load(),
		"active_connections":   pm.activeConnections.Load(),
		"total_requests":       totalReqs,
		"requests_per_second":  requestsPerSecond,
		"total_bytes":          pm.totalBytes.Load(),
		"avg_response_time_ms": avgResponseTime,

		// 系统统计
		"uptime_seconds":  uptime.Seconds(),
		"goroutines":      runtime.NumGoroutine(),
		"memory_alloc_mb": float64(m.Alloc) / 1024 / 1024,
		"memory_sys_mb":   float64(m.Sys) / 1024 / 1024,
		"gc_count":        m.NumGC,
		"gc_pause_ns":     m.PauseTotalNs,

		// CPU统计
		"cpu_count": runtime.NumCPU(),
	}
}

// Reset 重置统计（保留累计值）
func (pm *PerformanceMonitor) Reset() {
	pm.lastResetTime.Store(time.Now().UnixNano())
	// 不重置累计统计，只重置可重置的计数器
}

// GetMemoryUsage 获取内存使用情况
func (pm *PerformanceMonitor) GetMemoryUsage() map[string]any {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return map[string]any{
		"alloc_mb":       float64(m.Alloc) / 1024 / 1024,
		"total_alloc_mb": float64(m.TotalAlloc) / 1024 / 1024,
		"sys_mb":         float64(m.Sys) / 1024 / 1024,
		"heap_alloc_mb":  float64(m.HeapAlloc) / 1024 / 1024,
		"heap_sys_mb":    float64(m.HeapSys) / 1024 / 1024,
		"heap_objects":   m.HeapObjects,
		"gc_count":       m.NumGC,
		"gc_pause_ms":    float64(m.PauseTotalNs) / 1e6,
	}
}

// ForceGC 强制垃圾回收
func (pm *PerformanceMonitor) ForceGC() {
	runtime.GC()
}

// GetConnectionRate 获取连接速率
func (pm *PerformanceMonitor) GetConnectionRate() float64 {
	uptime := time.Since(pm.startTime).Seconds()
	if uptime == 0 {
		return 0
	}
	return float64(pm.totalConnections.Load()) / uptime
}

// GetThroughput 获取吞吐量（字节/秒）
func (pm *PerformanceMonitor) GetThroughput() float64 {
	uptime := time.Since(pm.startTime).Seconds()
	if uptime == 0 {
		return 0
	}
	return float64(pm.totalBytes.Load()) / uptime
}
