package logger

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"
	"github.com/bytedance/sonic"
	"fake-mc-server/internal/config"
)

// HoneypotEvent 蜜罐事件结构（优化版：不记录connID、dataHex、kickMsg）
type HoneypotEvent struct {
	Timestamp       time.Time `json:"timestamp"`
	ClientIP        string    `json:"client_ip"`
	EventType       string    `json:"event_type"` // "connection", "handshake", "login_attempt", "status_query", "protocol_violation"
	ProtocolVersion int       `json:"protocol_version,omitempty"`
	ServerAddress   string    `json:"server_address,omitempty"`
	ServerPort      uint16    `json:"server_port,omitempty"`
	NextState       int       `json:"next_state,omitempty"` // 1=status, 2=login
	Username        string    `json:"username,omitempty"`
	DelayApplied    int64     `json:"delay_applied_ms,omitempty"` // 延迟时间(毫秒)
	IPFrequency     float64   `json:"ip_frequency,omitempty"`
	ErrorMessage    string    `json:"error_message,omitempty"`
	UserAgent       string    `json:"user_agent,omitempty"`
	GeoLocation     string    `json:"geo_location,omitempty"` // 预留地理位置字段
}

// HoneypotLogger 蜜罐专用日志记录器
type HoneypotLogger struct {
	config    *config.HoneypotLoggingConfig
	writer    io.Writer
	csvWriter *csv.Writer
	mutex     sync.Mutex
	enabled   bool
}

// NewHoneypotLogger 创建蜜罐日志记录器
func NewHoneypotLogger(cfg *config.HoneypotLoggingConfig) (*HoneypotLogger, error) {
	if !cfg.Enabled {
		return &HoneypotLogger{enabled: false}, nil
	}

	// 确保日志目录存在
	logDir := filepath.Dir(cfg.FilePath)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("创建蜜罐日志目录失败: %w", err)
	}

	// 配置日志轮转
	fileWriter := &lumberjack.Logger{
		Filename:   cfg.FilePath,
		MaxSize:    cfg.MaxSize,
		MaxBackups: cfg.MaxBackups,
		MaxAge:     cfg.MaxAge,
		Compress:   cfg.Compress,
	}

	logger := &HoneypotLogger{
		config:  cfg,
		writer:  fileWriter,
		enabled: true,
	}

	// 如果是CSV格式，初始化CSV写入器并写入表头
	if strings.ToLower(cfg.Format) == "csv" {
		logger.csvWriter = csv.NewWriter(fileWriter)
		if err := logger.writeCSVHeader(); err != nil {
			return nil, fmt.Errorf("写入CSV表头失败: %w", err)
		}
	}

	return logger, nil
}

// writeCSVHeader 写入CSV表头（优化版）
func (hl *HoneypotLogger) writeCSVHeader() error {
	headers := []string{
		"timestamp", "client_ip", "event_type",
		"protocol_version", "server_address", "server_port", "next_state",
		"username", "delay_applied_ms", "ip_frequency",
		"error_message", "user_agent", "geo_location",
	}
	return hl.csvWriter.Write(headers)
}

// LogEvent 记录蜜罐事件
func (hl *HoneypotLogger) LogEvent(event *HoneypotEvent) error {
	if !hl.enabled {
		return nil
	}

	hl.mutex.Lock()
	defer hl.mutex.Unlock()

	// 设置时间戳
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	switch strings.ToLower(hl.config.Format) {
	case "csv":
		return hl.writeCSV(event)
	default: // json
		return hl.writeJSON(event)
	}
}

// writeJSON 写入JSON格式
func (hl *HoneypotLogger) writeJSON(event *HoneypotEvent) error {
	data, err := sonic.Marshal(event)
	if err != nil {
		return fmt.Errorf("序列化蜜罐事件失败: %w", err)
	}

	_, err = hl.writer.Write(append(data, '\n'))
	return err
}

// writeCSV 写入CSV格式（优化版）
func (hl *HoneypotLogger) writeCSV(event *HoneypotEvent) error {
	record := []string{
		event.Timestamp.Format(time.RFC3339),
		event.ClientIP,
		event.EventType,
		fmt.Sprintf("%d", event.ProtocolVersion),
		event.ServerAddress,
		fmt.Sprintf("%d", event.ServerPort),
		fmt.Sprintf("%d", event.NextState),
		event.Username,
		fmt.Sprintf("%d", event.DelayApplied),
		fmt.Sprintf("%.2f", event.IPFrequency),
		event.ErrorMessage,
		event.UserAgent,
		event.GeoLocation,
	}

	if err := hl.csvWriter.Write(record); err != nil {
		return err
	}

	// 立即刷新到文件
	hl.csvWriter.Flush()
	return hl.csvWriter.Error()
}

// LogConnection 记录连接事件（优化版：不记录connID）
func (hl *HoneypotLogger) LogConnection(clientIP string, delayMs int64, ipFreq float64) error {
	return hl.LogEvent(&HoneypotEvent{
		ClientIP:     clientIP,
		EventType:    "connection",
		DelayApplied: delayMs,
		IPFrequency:  ipFreq,
	})
}

// LogHandshake 记录握手包事件（优化版：不记录connID和dataHex）
func (hl *HoneypotLogger) LogHandshake(clientIP string, protocolVer int, serverAddr string, serverPort uint16, nextState int) error {
	return hl.LogEvent(&HoneypotEvent{
		ClientIP:        clientIP,
		EventType:       "handshake",
		ProtocolVersion: protocolVer,
		ServerAddress:   serverAddr,
		ServerPort:      serverPort,
		NextState:       nextState,
	})
}

// LogLoginAttempt 记录登录尝试事件（优化版：不记录connID和kickMsg）
func (hl *HoneypotLogger) LogLoginAttempt(clientIP, username string, delayMs int64) error {
	return hl.LogEvent(&HoneypotEvent{
		ClientIP:     clientIP,
		EventType:    "login_attempt",
		Username:     username,
		DelayApplied: delayMs,
	})
}

// LogStatusQuery 记录状态查询事件（优化版：不记录connID）
func (hl *HoneypotLogger) LogStatusQuery(clientIP string, protocolVer int, serverAddr string, serverPort uint16) error {
	return hl.LogEvent(&HoneypotEvent{
		ClientIP:        clientIP,
		EventType:       "status_query",
		ProtocolVersion: protocolVer,
		ServerAddress:   serverAddr,
		ServerPort:      serverPort,
		NextState:       1,
	})
}

// LogProtocolViolation 记录协议违规事件（优化版：不记录connID和dataHex）
func (hl *HoneypotLogger) LogProtocolViolation(clientIP, errorMsg string) error {
	return hl.LogEvent(&HoneypotEvent{
		ClientIP:     clientIP,
		EventType:    "protocol_violation",
		ErrorMessage: errorMsg,
	})
}

// Close 关闭日志记录器
func (hl *HoneypotLogger) Close() error {
	if !hl.enabled {
		return nil
	}

	hl.mutex.Lock()
	defer hl.mutex.Unlock()

	if hl.csvWriter != nil {
		hl.csvWriter.Flush()
	}

	if closer, ok := hl.writer.(io.Closer); ok {
		return closer.Close()
	}

	return nil
}

// IsEnabled 检查是否启用
func (hl *HoneypotLogger) IsEnabled() bool {
	return hl.enabled
}
