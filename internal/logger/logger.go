package logger

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gopkg.in/natefinch/lumberjack.v2"

	"fake-mc-server/internal/config"
)

// Setup 设置日志
func Setup(cfg *config.Config) (zerolog.Logger, error) {
	// 设置日志级别
	level, err := zerolog.ParseLevel(cfg.Logging.Level)
	if err != nil {
		return zerolog.Logger{}, fmt.Errorf("无效的日志级别 '%s': %w", cfg.Logging.Level, err)
	}
	zerolog.SetGlobalLevel(level)

	// 设置时间格式
	zerolog.TimeFieldFormat = time.RFC3339

	// 创建输出写入器
	var writers []io.Writer

	// 根据配置选择输出目标
	switch strings.ToLower(cfg.Logging.Output) {
	case "stdout":
		if cfg.Logging.Format == "console" {
			writers = append(writers, zerolog.ConsoleWriter{
				Out:        os.Stdout,
				TimeFormat: time.RFC3339,
			})
		} else {
			writers = append(writers, os.Stdout)
		}

	case "stderr":
		if cfg.Logging.Format == "console" {
			writers = append(writers, zerolog.ConsoleWriter{
				Out:        os.Stderr,
				TimeFormat: time.RFC3339,
			})
		} else {
			writers = append(writers, os.Stderr)
		}

	case "file":
		// 确保日志目录存在
		logDir := filepath.Dir(cfg.Logging.FilePath)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return zerolog.Logger{}, fmt.Errorf("创建日志目录失败: %w", err)
		}

		// 配置日志轮转
		fileWriter := &lumberjack.Logger{
			Filename:   cfg.Logging.FilePath,
			MaxSize:    cfg.Logging.MaxSize,
			MaxBackups: cfg.Logging.MaxBackups,
			MaxAge:     cfg.Logging.MaxAge,
			Compress:   cfg.Logging.Compress,
		}
		writers = append(writers, fileWriter)

		// 如果是控制台格式，同时输出到控制台
		if cfg.Logging.Format == "console" {
			writers = append(writers, zerolog.ConsoleWriter{
				Out:        os.Stdout,
				TimeFormat: time.RFC3339,
			})
		}

	default:
		return zerolog.Logger{}, fmt.Errorf("不支持的日志输出类型: %s", cfg.Logging.Output)
	}

	// 创建多写入器
	var writer io.Writer
	if len(writers) == 1 {
		writer = writers[0]
	} else {
		writer = io.MultiWriter(writers...)
	}

	// 创建日志器
	logger := zerolog.New(writer).With().
		Timestamp().
		Str("service", "fake-mc-server").
		Logger()

	// 设置全局日志器
	log.Logger = logger

	return logger, nil
}

// AttackLogger 攻击行为日志记录器
type AttackLogger struct {
	logger zerolog.Logger
}

// NewAttackLogger 创建攻击日志记录器
func NewAttackLogger(logger zerolog.Logger) *AttackLogger {
	return &AttackLogger{
		logger: logger.With().Str("component", "attack_logger").Logger(),
	}
}

// LogConnectionAttempt 记录连接尝试
func (al *AttackLogger) LogConnectionAttempt(ip, userAgent string, success bool) {
	event := al.logger.Info()
	if !success {
		event = al.logger.Warn()
	}

	event.
		Str("event_type", "connection_attempt").
		Str("ip", ip).
		Str("user_agent", userAgent).
		Bool("success", success).
		Msg("连接尝试")
}

// LogLoginAttempt 记录登录尝试
func (al *AttackLogger) LogLoginAttempt(ip, username string, delay time.Duration) {
	al.logger.Warn().
		Str("event_type", "login_attempt").
		Str("ip", ip).
		Str("username", username).
		Dur("delay_applied", delay).
		Msg("登录尝试")
}

// LogRateLimitTriggered 记录限流触发
func (al *AttackLogger) LogRateLimitTriggered(ip string, limitType string, requestCount int64) {
	al.logger.Warn().
		Str("event_type", "rate_limit_triggered").
		Str("ip", ip).
		Str("limit_type", limitType).
		Int64("request_count", requestCount).
		Msg("限流触发")
}

// LogSuspiciousActivity 记录可疑活动
func (al *AttackLogger) LogSuspiciousActivity(ip string, activity string, details map[string]interface{}) {
	event := al.logger.Warn().
		Str("event_type", "suspicious_activity").
		Str("ip", ip).
		Str("activity", activity)

	// 添加详细信息
	for key, value := range details {
		event = event.Interface(key, value)
	}

	event.Msg("可疑活动")
}

// LogCircuitBreakerTriggered 记录熔断器触发
func (al *AttackLogger) LogCircuitBreakerTriggered(reason string, metrics map[string]interface{}) {
	event := al.logger.Error().
		Str("event_type", "circuit_breaker_triggered").
		Str("reason", reason)

	// 添加指标信息
	for key, value := range metrics {
		event = event.Interface(key, value)
	}

	event.Msg("熔断器触发")
}

// LogUpstreamSync 记录上游同步
func (al *AttackLogger) LogUpstreamSync(host string, port int, success bool, responseTime time.Duration, playerCount int) {
	event := al.logger.Debug()
	if !success {
		event = al.logger.Warn()
	}

	event.
		Str("event_type", "upstream_sync").
		Str("upstream_host", host).
		Int("upstream_port", port).
		Bool("success", success).
		Dur("response_time", responseTime).
		Int("player_count", playerCount).
		Msg("上游同步")
}

// PerformanceLogger 性能日志记录器
type PerformanceLogger struct {
	logger zerolog.Logger
}

// NewPerformanceLogger 创建性能日志记录器
func NewPerformanceLogger(logger zerolog.Logger) *PerformanceLogger {
	return &PerformanceLogger{
		logger: logger.With().Str("component", "performance_logger").Logger(),
	}
}

// LogConnectionMetrics 记录连接指标
func (pl *PerformanceLogger) LogConnectionMetrics(activeConnections, totalConnections int64, avgResponseTime time.Duration) {
	pl.logger.Info().
		Str("metric_type", "connection_metrics").
		Int64("active_connections", activeConnections).
		Int64("total_connections", totalConnections).
		Dur("avg_response_time", avgResponseTime).
		Msg("连接指标")
}

// LogMemoryUsage 记录内存使用情况
func (pl *PerformanceLogger) LogMemoryUsage(allocMB, sysMB, gcCount uint64) {
	pl.logger.Debug().
		Str("metric_type", "memory_usage").
		Uint64("alloc_mb", allocMB).
		Uint64("sys_mb", sysMB).
		Uint64("gc_count", gcCount).
		Msg("内存使用情况")
}

// LogRateLimitMetrics 记录限流指标
func (pl *PerformanceLogger) LogRateLimitMetrics(globalRequests, totalRequests int64, activeIPs int, avgRequestsPerSecond float64) {
	pl.logger.Info().
		Str("metric_type", "rate_limit_metrics").
		Int64("global_requests", globalRequests).
		Int64("total_requests", totalRequests).
		Int("active_ips", activeIPs).
		Float64("avg_requests_per_second", avgRequestsPerSecond).
		Msg("限流指标")
}

// SecurityLogger 安全日志记录器
type SecurityLogger struct {
	logger zerolog.Logger
}

// NewSecurityLogger 创建安全日志记录器
func NewSecurityLogger(logger zerolog.Logger) *SecurityLogger {
	return &SecurityLogger{
		logger: logger.With().Str("component", "security_logger").Logger(),
	}
}

// LogIPBlocked 记录 IP 被阻止
func (sl *SecurityLogger) LogIPBlocked(ip, reason string) {
	sl.logger.Warn().
		Str("event_type", "ip_blocked").
		Str("ip", ip).
		Str("reason", reason).
		Msg("IP 被阻止")
}

// LogIPWhitelisted 记录 IP 白名单
func (sl *SecurityLogger) LogIPWhitelisted(ip string) {
	sl.logger.Info().
		Str("event_type", "ip_whitelisted").
		Str("ip", ip).
		Msg("IP 在白名单中")
}

// LogPacketSizeExceeded 记录数据包大小超限
func (sl *SecurityLogger) LogPacketSizeExceeded(ip string, packetSize, maxSize int) {
	sl.logger.Warn().
		Str("event_type", "packet_size_exceeded").
		Str("ip", ip).
		Int("packet_size", packetSize).
		Int("max_size", maxSize).
		Msg("数据包大小超限")
}

// LogProtocolViolation 记录协议违规
func (sl *SecurityLogger) LogProtocolViolation(ip string, violation string, details map[string]interface{}) {
	event := sl.logger.Warn().
		Str("event_type", "protocol_violation").
		Str("ip", ip).
		Str("violation", violation)

	// 添加详细信息
	for key, value := range details {
		event = event.Interface(key, value)
	}

	event.Msg("协议违规")
}

// LoggerManager 日志管理器
type LoggerManager struct {
	mainLogger        zerolog.Logger
	attackLogger      *AttackLogger
	performanceLogger *PerformanceLogger
	securityLogger    *SecurityLogger
	honeypotLogger    *HoneypotLogger
	ctx               context.Context
	cancel            context.CancelFunc
}

// NewLoggerManager 创建日志管理器
func NewLoggerManager(ctx context.Context, cfg *config.Config) (*LoggerManager, error) {
	mainLogger, err := Setup(cfg)
	if err != nil {
		return nil, err
	}

	// 创建蜜罐日志记录器
	honeypotLogger, err := NewHoneypotLogger(&cfg.HoneypotLogging)
	if err != nil {
		return nil, fmt.Errorf("创建蜜罐日志记录器失败: %w", err)
	}

	// 创建内部 context，继承自外部 context
	managerCtx, cancel := context.WithCancel(ctx)

	manager := &LoggerManager{
		mainLogger:        mainLogger,
		attackLogger:      NewAttackLogger(mainLogger),
		performanceLogger: NewPerformanceLogger(mainLogger),
		securityLogger:    NewSecurityLogger(mainLogger),
		honeypotLogger:    honeypotLogger,
		ctx:               managerCtx,
		cancel:            cancel,
	}

	// 启动生命周期管理 goroutine
	go manager.lifecycleManager()

	return manager, nil
}

// GetMainLogger 获取主日志器
func (lm *LoggerManager) GetMainLogger() zerolog.Logger {
	return lm.mainLogger
}

// GetAttackLogger 获取攻击日志器
func (lm *LoggerManager) GetAttackLogger() *AttackLogger {
	return lm.attackLogger
}

// GetPerformanceLogger 获取性能日志器
func (lm *LoggerManager) GetPerformanceLogger() *PerformanceLogger {
	return lm.performanceLogger
}

// GetSecurityLogger 获取安全日志器
func (lm *LoggerManager) GetSecurityLogger() *SecurityLogger {
	return lm.securityLogger
}

// GetHoneypotLogger 获取蜜罐日志器
func (lm *LoggerManager) GetHoneypotLogger() *HoneypotLogger {
	return lm.honeypotLogger
}

// lifecycleManager 生命周期管理
func (lm *LoggerManager) lifecycleManager() {
	<-lm.ctx.Done()

	// 当 context 被取消时，自动关闭日志管理器
	lm.mainLogger.Debug().Msg("日志管理器收到关闭信号，开始自动关闭")
	if err := lm.closeInternal(); err != nil {
		lm.mainLogger.Error().Err(err).Msg("自动关闭日志管理器失败")
	} else {
		lm.mainLogger.Debug().Msg("日志管理器已自动关闭")
	}
}

// closeInternal 内部关闭方法
func (lm *LoggerManager) closeInternal() error {
	if lm.honeypotLogger != nil {
		return lm.honeypotLogger.Close()
	}
	return nil
}

// Close 关闭所有日志器（手动调用）
func (lm *LoggerManager) Close() error {
	// 取消内部 context，这会触发 lifecycleManager 中的自动关闭
	lm.cancel()
	return nil
}
