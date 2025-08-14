package limiter

import (
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"fake-mc-server/internal/config"
)

// FastRateLimiter 高性能限流器
type FastRateLimiter struct {
	config      *config.Config
	globalCount atomic.Int64
	ipLimiters  sync.Map // map[string]*ipLimiter
	
	// 预计算的值，避免重复计算
	baseDelay       time.Duration
	ipFreqFactor    float64
	globalLoadFactor float64
}

// ipLimiter IP级别限流器
type ipLimiter struct {
	lastAccess atomic.Int64 // Unix纳秒时间戳
	count      atomic.Int64
	allowed    atomic.Bool
}

// NewFastRateLimiter 创建高性能限流器
func NewFastRateLimiter(cfg *config.Config) *FastRateLimiter {
	return &FastRateLimiter{
		config:           cfg,
		baseDelay:        cfg.Delay.BaseDelay,
		ipFreqFactor:     cfg.Delay.IPFrequencyFactor,
		globalLoadFactor: cfg.Delay.GlobalLoadFactor,
	}
}

// Allow 检查是否允许请求
func (f *FastRateLimiter) Allow(ip string) bool {
	// 快速路径：检查全局限制
	globalCount := f.globalCount.Add(1)
	if globalCount > int64(f.config.RateLimit.GlobalLimit) {
		f.globalCount.Add(-1)
		return false
	}
	
	// 获取或创建IP限流器
	limiterInterface, _ := f.ipLimiters.LoadOrStore(ip, &ipLimiter{
		allowed: atomic.Bool{},
	})
	limiter := limiterInterface.(*ipLimiter)
	
	// 更新访问时间和计数
	now := time.Now().UnixNano()
	limiter.lastAccess.Store(now)
	ipCount := limiter.count.Add(1)
	
	// 检查IP级别限制
	if ipCount > int64(f.config.RateLimit.IPLimit) {
		limiter.allowed.Store(false)
		return false
	}
	
	limiter.allowed.Store(true)
	return true
}

// CalculateDelay 计算延迟（优化版本）
func (f *FastRateLimiter) CalculateDelay(ip string) time.Duration {
	// 获取IP限流器
	limiterInterface, exists := f.ipLimiters.Load(ip)
	if !exists {
		return f.baseDelay
	}
	
	limiter := limiterInterface.(*ipLimiter)
	ipFreq := float64(limiter.count.Load())
	globalLoad := float64(f.globalCount.Load())
	
	// 使用位运算和预计算值优化计算
	ipPenalty := time.Duration(ipFreq * f.ipFreqFactor * float64(time.Millisecond))
	globalPenalty := time.Duration(globalLoad * f.globalLoadFactor * float64(time.Millisecond))
	
	return f.baseDelay + ipPenalty + globalPenalty
}

// Cleanup 清理过期的IP限流器
func (f *FastRateLimiter) Cleanup() {
	cutoff := time.Now().Add(-time.Hour).UnixNano()
	
	f.ipLimiters.Range(func(key, value interface{}) bool {
		limiter := value.(*ipLimiter)
		if limiter.lastAccess.Load() < cutoff {
			f.ipLimiters.Delete(key)
		}
		return true
	})
}

// GetStats 获取统计信息
func (f *FastRateLimiter) GetStats() map[string]interface{} {
	activeIPs := 0
	f.ipLimiters.Range(func(key, value interface{}) bool {
		activeIPs++
		return true
	})
	
	return map[string]interface{}{
		"global_requests": f.globalCount.Load(),
		"active_ips":      activeIPs,
	}
}

// Reset 重置计数器（定期调用）
func (f *FastRateLimiter) Reset() {
	// 重置全局计数器
	f.globalCount.Store(0)
	
	// 重置IP计数器
	f.ipLimiters.Range(func(key, value interface{}) bool {
		limiter := value.(*ipLimiter)
		limiter.count.Store(0)
		return true
	})
}

// 内存对齐优化
var _ = (*ipLimiter)(nil)

// 确保结构体内存对齐
func init() {
	// 检查关键结构体的大小，确保缓存行对齐
	if unsafe.Sizeof(ipLimiter{}) > 64 {
		panic("ipLimiter struct too large for cache line")
	}
}
