package logger

import (
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
)

// RateLimitedLogger 限流日志器，避免相同消息刷屏
type RateLimitedLogger struct {
	logger    zerolog.Logger
	lastLog   atomic.Int64 // Unix 纳秒时间戳
	interval  time.Duration
	lastMsg   atomic.Value // string
	skipCount atomic.Int64
}

// NewRateLimitedLogger 创建限流日志器
func NewRateLimitedLogger(logger zerolog.Logger, interval time.Duration) *RateLimitedLogger {
	return &RateLimitedLogger{
		logger:   logger,
		interval: interval,
	}
}

// Info 限流的 Info 日志
func (rl *RateLimitedLogger) Info(msg string) *zerolog.Event {
	if rl.shouldLog(msg) {
		return rl.logger.Info()
	}
	return rl.logger.Debug() // 降级为 debug 级别
}

// Warn 限流的 Warn 日志
func (rl *RateLimitedLogger) Warn(msg string) *zerolog.Event {
	if rl.shouldLog(msg) {
		return rl.logger.Warn()
	}
	return rl.logger.Debug() // 降级为 debug 级别
}

// Error 限流的 Error 日志（错误日志不限流）
func (rl *RateLimitedLogger) Error(msg string) *zerolog.Event {
	return rl.logger.Error()
}

// shouldLog 检查是否应该记录日志
func (rl *RateLimitedLogger) shouldLog(msg string) bool {
	now := time.Now().UnixNano()
	lastLog := rl.lastLog.Load()

	// 如果距离上次日志时间超过间隔，允许记录
	if now-lastLog > int64(rl.interval) {
		rl.lastLog.Store(now)

		// 如果有跳过的日志，记录跳过数量
		skipped := rl.skipCount.Swap(0)
		if skipped > 0 {
			rl.logger.Debug().
				Int64("skipped_count", skipped).
				Str("last_message", msg).
				Msg("跳过重复日志")
		}

		rl.lastMsg.Store(msg)
		return true
	}

	// 如果是相同消息，增加跳过计数
	if lastMsgInterface := rl.lastMsg.Load(); lastMsgInterface != nil {
		if lastMsg, ok := lastMsgInterface.(string); ok && lastMsg == msg {
			rl.skipCount.Add(1)
		}
	}

	return false
}

// ConnectionLogger 连接专用日志器，减少连接相关的噪音
type ConnectionLogger struct {
	logger       zerolog.Logger
	rateLimited  *RateLimitedLogger
	connID       string
	remoteIP     string
	logImportant bool // 是否记录重要事件
}

// NewConnectionLogger 创建连接日志器
func NewConnectionLogger(logger zerolog.Logger, connID, remoteIP string) *ConnectionLogger {
	connLogger := logger.With().
		Str("conn_id", connID).
		Str("remote_ip", remoteIP).
		Logger()

	return &ConnectionLogger{
		logger:       connLogger,
		rateLimited:  NewRateLimitedLogger(connLogger, 5*time.Second),
		connID:       connID,
		remoteIP:     remoteIP,
		logImportant: true, // 默认记录重要事件
	}
}

// SetImportantOnly 设置是否只记录重要事件
func (cl *ConnectionLogger) SetImportantOnly(important bool) {
	cl.logImportant = important
}

// Debug 调试日志（通常被过滤）
func (cl *ConnectionLogger) Debug() *zerolog.Event {
	if cl.logImportant {
		return cl.logger.Debug().Str("level", "filtered") // 标记为已过滤
	}
	return cl.logger.Debug()
}

// Info 信息日志（限流）
func (cl *ConnectionLogger) Info() *zerolog.Event {
	return cl.rateLimited.Info("connection_info")
}

// Warn 警告日志
func (cl *ConnectionLogger) Warn() *zerolog.Event {
	return cl.logger.Warn()
}

// Error 错误日志
func (cl *ConnectionLogger) Error() *zerolog.Event {
	return cl.logger.Error()
}

// LogConnectionEvent 记录连接事件
func (cl *ConnectionLogger) LogConnectionEvent(event string, important bool) {
	if important || !cl.logImportant {
		cl.logger.Info().
			Str("event", event).
			Msg("连接事件")
	}
}

// LogDataTransfer 记录数据传输（限流）
func (cl *ConnectionLogger) LogDataTransfer(bytes int, direction string) {
	if !cl.logImportant {
		cl.rateLimited.Info("data_transfer").
			Int("bytes", bytes).
			Str("direction", direction).
			Msg("数据传输")
	}
}

// LogProtocolEvent 记录协议事件
func (cl *ConnectionLogger) LogProtocolEvent(event string, success bool) {
	level := cl.logger.Info()
	if !success {
		level = cl.logger.Warn()
	}

	level.
		Str("protocol_event", event).
		Bool("success", success).
		Msg("协议事件")
}
