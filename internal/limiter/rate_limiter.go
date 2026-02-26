package limiter

import (
	"math"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"golang.org/x/time/rate"

	"fake-mc-server/internal/config"
)

// RateLimiter 限流器
type RateLimiter struct {
	config        *config.Config
	logger        zerolog.Logger
	globalLimiter *rate.Limiter
	ipLimiters    sync.Map // map[string]*IPLimiterInfo
	mu            sync.RWMutex

	// 统计信息
	globalRequests int64
	totalRequests  int64
	startTime      time.Time
}

// IPLimiterInfo IP 限流器信息
type IPLimiterInfo struct {
	Limiter      *rate.Limiter
	RequestCount int64
	FirstRequest time.Time
	LastRequest  time.Time
	mu           sync.RWMutex
}

// NewRateLimiter 创建限流器
func NewRateLimiter(cfg *config.Config, logger zerolog.Logger) *RateLimiter {
	return &RateLimiter{
		config: cfg,
		logger: logger.With().Str("component", "rate_limiter").Logger(),
		globalLimiter: rate.NewLimiter(
			rate.Limit(cfg.RateLimit.GlobalLimit),
			cfg.RateLimit.GlobalLimit,
		),
		startTime: time.Now(),
	}
}

// Allow 检查是否允许请求
func (rl *RateLimiter) Allow(ip string) bool {
	// 检查全局限流
	if !rl.globalLimiter.Allow() {
		rl.logger.Debug().
			Str("ip", ip).
			Msg("全局限流触发")
		return false
	}

	// 检查 IP 限流
	ipLimiter := rl.getOrCreateIPLimiter(ip)
	if !ipLimiter.Limiter.Allow() {
		rl.logger.Debug().
			Str("ip", ip).
			Msg("IP 限流触发")
		return false
	}

	// 更新统计信息
	rl.updateStats(ip, ipLimiter)

	return true
}

// CalculateDelay 计算延迟时间
func (rl *RateLimiter) CalculateDelay(ip string) time.Duration {
	// 获取 IP 限流器信息
	ipLimiter := rl.getOrCreateIPLimiter(ip)

	// 计算 IP 频率因子
	ipFrequency := rl.calculateIPFrequency(ipLimiter)

	// 计算全局负载因子
	globalLoad := rl.calculateGlobalLoad()

	// 改进的延迟计算公式
	baseDelay := float64(rl.config.Delay.BaseDelay.Nanoseconds())

	// IP 惩罚延迟
	ipPenalty := math.Min(
		float64(rl.config.Delay.MaxIPPenalty.Nanoseconds()),
		ipFrequency*rl.config.Delay.IPRateMultiplier*baseDelay,
	)

	// 全局惩罚延迟
	globalPenalty := math.Min(
		float64(rl.config.Delay.MaxGlobalPenalty.Nanoseconds()),
		globalLoad*rl.config.Delay.GlobalRateMultiplier*baseDelay,
	)

	// 总延迟
	totalDelay := time.Duration(baseDelay + ipPenalty + globalPenalty)

	rl.logger.Debug().
		Str("ip", ip).
		Float64("ip_frequency", ipFrequency).
		Float64("global_load", globalLoad).
		Dur("base_delay", rl.config.Delay.BaseDelay).
		Dur("ip_penalty", time.Duration(ipPenalty)).
		Dur("global_penalty", time.Duration(globalPenalty)).
		Dur("total_delay", totalDelay).
		Msg("计算延迟")

	return totalDelay
}

// getOrCreateIPLimiter 获取或创建 IP 限流器
func (rl *RateLimiter) getOrCreateIPLimiter(ip string) *IPLimiterInfo {
	if value, ok := rl.ipLimiters.Load(ip); ok {
		return value.(*IPLimiterInfo)
	}

	// 创建新的 IP 限流器
	ipLimiter := &IPLimiterInfo{
		Limiter: rate.NewLimiter(
			rate.Limit(rl.config.RateLimit.IPLimit),
			rl.config.RateLimit.IPLimit,
		),
		FirstRequest: time.Now(),
		LastRequest:  time.Now(),
	}

	// 尝试存储，如果已存在则使用已存在的
	if actual, loaded := rl.ipLimiters.LoadOrStore(ip, ipLimiter); loaded {
		return actual.(*IPLimiterInfo)
	}

	rl.logger.Debug().
		Str("ip", ip).
		Msg("创建新的 IP 限流器")

	return ipLimiter
}

// calculateIPFrequency 计算 IP 频率因子
func (rl *RateLimiter) calculateIPFrequency(ipLimiter *IPLimiterInfo) float64 {
	ipLimiter.mu.RLock()
	defer ipLimiter.mu.RUnlock()

	// 计算时间窗口内的请求频率
	duration := time.Since(ipLimiter.FirstRequest)
	if duration == 0 {
		return 1.0
	}

	// 每秒请求数
	requestsPerSecond := float64(ipLimiter.RequestCount) / duration.Seconds()

	// 频率因子 = 实际频率 / 限制频率
	frequencyFactor := requestsPerSecond / float64(rl.config.RateLimit.IPLimit)

	// 应用配置的频率因子
	return math.Max(1.0, frequencyFactor*rl.config.Delay.IPFrequencyFactor)
}

// calculateGlobalLoad 计算全局负载因子
func (rl *RateLimiter) calculateGlobalLoad() float64 {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	// 计算全局请求频率
	duration := time.Since(rl.startTime)
	if duration == 0 {
		return 1.0
	}

	// 每秒请求数
	requestsPerSecond := float64(rl.totalRequests) / duration.Seconds()

	// 负载因子 = 实际频率 / 限制频率
	loadFactor := requestsPerSecond / float64(rl.config.RateLimit.GlobalLimit)

	// 应用配置的负载因子
	return math.Max(1.0, loadFactor*rl.config.Delay.GlobalLoadFactor)
}

// updateStats 更新统计信息
func (rl *RateLimiter) updateStats(ip string, ipLimiter *IPLimiterInfo) {
	now := time.Now()

	// 更新 IP 统计
	ipLimiter.mu.Lock()
	ipLimiter.RequestCount++
	ipLimiter.LastRequest = now
	ipLimiter.mu.Unlock()

	// 更新全局统计
	rl.mu.Lock()
	rl.totalRequests++
	rl.globalRequests++
	rl.mu.Unlock()
}

// CleanupExpiredLimiters 清理过期的限流器
func (rl *RateLimiter) CleanupExpiredLimiters() {
	now := time.Now()
	expiredIPs := make([]string, 0)

	rl.ipLimiters.Range(func(key, value any) bool {
		ip := key.(string)
		ipLimiter := value.(*IPLimiterInfo)

		ipLimiter.mu.RLock()
		lastRequest := ipLimiter.LastRequest
		ipLimiter.mu.RUnlock()

		// 如果超过清理间隔没有请求，则标记为过期
		if now.Sub(lastRequest) > rl.config.RateLimit.CleanupInterval {
			expiredIPs = append(expiredIPs, ip)
		}

		return true
	})

	// 删除过期的限流器
	for _, ip := range expiredIPs {
		rl.ipLimiters.Delete(ip)
	}

	if len(expiredIPs) > 0 {
		rl.logger.Debug().
			Int("count", len(expiredIPs)).
			Msg("清理过期的 IP 限流器")
	}
}

// StartCleanupRoutine 启动清理协程
func (rl *RateLimiter) StartCleanupRoutine() {
	go func() {
		ticker := time.NewTicker(rl.config.RateLimit.CleanupInterval)
		defer ticker.Stop()

		for range ticker.C {
			rl.CleanupExpiredLimiters()
		}
	}()
}

// GetStats 获取统计信息
func (rl *RateLimiter) GetStats() map[string]any {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	// 计算活跃 IP 数量
	activeIPs := 0
	rl.ipLimiters.Range(func(key, value any) bool {
		activeIPs++
		return true
	})

	// 计算平均请求频率
	duration := time.Since(rl.startTime)
	avgRequestsPerSecond := float64(rl.totalRequests) / duration.Seconds()

	return map[string]any{
		"total_requests":          rl.totalRequests,
		"global_requests":         rl.globalRequests,
		"active_ip_count":         activeIPs,
		"avg_requests_per_second": avgRequestsPerSecond,
		"uptime":                  duration,
		"global_limit":            rl.config.RateLimit.GlobalLimit,
		"ip_limit":                rl.config.RateLimit.IPLimit,
	}
}

// GetIPStats 获取指定 IP 的统计信息
func (rl *RateLimiter) GetIPStats(ip string) map[string]any {
	if value, ok := rl.ipLimiters.Load(ip); ok {
		ipLimiter := value.(*IPLimiterInfo)
		ipLimiter.mu.RLock()
		defer ipLimiter.mu.RUnlock()

		duration := time.Since(ipLimiter.FirstRequest)
		requestsPerSecond := float64(ipLimiter.RequestCount) / duration.Seconds()

		return map[string]any{
			"ip":                  ip,
			"request_count":       ipLimiter.RequestCount,
			"first_request":       ipLimiter.FirstRequest,
			"last_request":        ipLimiter.LastRequest,
			"duration":            duration,
			"requests_per_second": requestsPerSecond,
			"current_tokens":      ipLimiter.Limiter.Tokens(),
		}
	}

	return map[string]any{
		"ip":    ip,
		"found": false,
	}
}

// GetIPFrequency 获取IP访问频率
func (rl *RateLimiter) GetIPFrequency(ip string) float64 {
	if limiterInfo, ok := rl.ipLimiters.Load(ip); ok {
		ipLimiter := limiterInfo.(*IPLimiterInfo)
		ipLimiter.mu.RLock()
		defer ipLimiter.mu.RUnlock()

		duration := time.Since(ipLimiter.FirstRequest)
		if duration.Seconds() == 0 {
			return 0
		}
		return float64(ipLimiter.RequestCount) / duration.Seconds()
	}
	return 0
}

// IsCircuitBreakerTriggered 检查熔断器是否触发
func (rl *RateLimiter) IsCircuitBreakerTriggered() bool {
	// 简单的熔断逻辑：如果全局限流器的令牌数为 0，则触发熔断
	return rl.globalLimiter.Tokens() == 0
}
