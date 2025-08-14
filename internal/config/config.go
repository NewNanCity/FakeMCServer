package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config 主配置结构
type Config struct {
	Server          ServerConfig          `yaml:"server"`
	Upstream        UpstreamConfig        `yaml:"upstream"`
	RateLimit       RateLimitConfig       `yaml:"rate_limit"`
	Delay           DelayConfig           `yaml:"delay"`
	Messages        MessagesConfig        `yaml:"messages"`
	Logging         LoggingConfig         `yaml:"logging"`
	HoneypotLogging HoneypotLoggingConfig `yaml:"honeypot_logging"`
	Monitoring      MonitoringConfig      `yaml:"monitoring"`
	Security        SecurityConfig        `yaml:"security"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Host           string        `yaml:"host"`
	Port           int           `yaml:"port"`
	MaxConnections int           `yaml:"max_connections"`
	ReadTimeout    time.Duration `yaml:"read_timeout"`
	IdleTimeout    time.Duration `yaml:"idle_timeout"`
	NumLoops       int           `yaml:"num_loops"`
}

// UpstreamConfig 上游服务器配置
type UpstreamConfig struct {
	Enabled       bool          `yaml:"enabled"`
	Address       string        `yaml:"address"` // 服务器地址（支持 IP、域名、SRV 记录等）
	SyncInterval  time.Duration `yaml:"sync_interval"`
	Timeout       time.Duration `yaml:"timeout"`
	RetryCount    int           `yaml:"retry_count"`
	RetryInterval time.Duration `yaml:"retry_interval"`
}

// RateLimitConfig 限流配置
type RateLimitConfig struct {
	IPLimit         int           `yaml:"ip_limit"`
	GlobalLimit     int           `yaml:"global_limit"`
	Window          time.Duration `yaml:"window"`
	CleanupInterval time.Duration `yaml:"cleanup_interval"`
}

// DelayConfig 延迟配置
type DelayConfig struct {
	BaseDelay            time.Duration `yaml:"base_delay"`
	MaxIPPenalty         time.Duration `yaml:"max_ip_penalty"`
	MaxGlobalPenalty     time.Duration `yaml:"max_global_penalty"`
	IPFrequencyFactor    float64       `yaml:"ip_frequency_factor"`
	GlobalLoadFactor     float64       `yaml:"global_load_factor"`
	IPRateMultiplier     float64       `yaml:"ip_rate_multiplier"`
	GlobalRateMultiplier float64       `yaml:"global_rate_multiplier"`
}

// MessagesConfig 消息配置
type MessagesConfig struct {
	MOTD            string `yaml:"motd"`
	KickMessage     string `yaml:"kick_message"`
	VersionName     string `yaml:"version_name"`
	ProtocolVersion int    `yaml:"protocol_version"`
	MaxPlayers      int    `yaml:"max_players"`
	OnlinePlayers   int    `yaml:"online_players"`
}

// LoggingConfig 日志配置
type LoggingConfig struct {
	Level             string `yaml:"level"`
	Format            string `yaml:"format"`
	Output            string `yaml:"output"`
	FilePath          string `yaml:"file_path"`
	MaxSize           int    `yaml:"max_size"`
	MaxBackups        int    `yaml:"max_backups"`
	MaxAge            int    `yaml:"max_age"`
	Compress          bool   `yaml:"compress"`
	RecordAllAttempts bool   `yaml:"record_all_attempts"`
}

// HoneypotLoggingConfig 蜜罐日志配置
type HoneypotLoggingConfig struct {
	Enabled    bool   `yaml:"enabled"`
	FilePath   string `yaml:"file_path"`
	MaxSize    int    `yaml:"max_size"`
	MaxBackups int    `yaml:"max_backups"`
	MaxAge     int    `yaml:"max_age"`
	Compress   bool   `yaml:"compress"`
	Format     string `yaml:"format"` // json, csv
}

// MonitoringConfig 监控配置
type MonitoringConfig struct {
	Enabled         bool   `yaml:"enabled"`
	MetricsPort     int    `yaml:"metrics_port"`
	HealthCheckPath string `yaml:"health_check_path"`
	MetricsPath     string `yaml:"metrics_path"`
}

// SecurityConfig 安全配置
type SecurityConfig struct {
	EnableIPWhitelist bool          `yaml:"enable_ip_whitelist"`
	IPWhitelist       []string      `yaml:"ip_whitelist"`
	EnableIPBlacklist bool          `yaml:"enable_ip_blacklist"`
	IPBlacklist       []string      `yaml:"ip_blacklist"`
	MaxPacketSize     int           `yaml:"max_packet_size"`
	ConnectionTimeout time.Duration `yaml:"connection_timeout"`
}

// Load 从文件加载配置
func Load(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	// 设置默认值
	setDefaults(&config)

	// 验证配置
	if err := validate(&config); err != nil {
		return nil, fmt.Errorf("配置验证失败: %w", err)
	}

	return &config, nil
}

// setDefaults 设置默认值
func setDefaults(config *Config) {
	if config.Server.Host == "" {
		config.Server.Host = "0.0.0.0"
	}
	if config.Server.Port == 0 {
		config.Server.Port = 25565
	}
	if config.Server.MaxConnections == 0 {
		config.Server.MaxConnections = 10000
	}
	if config.Server.ReadTimeout == 0 {
		config.Server.ReadTimeout = 30 * time.Second
	}
	if config.Server.IdleTimeout == 0 {
		config.Server.IdleTimeout = 10 * time.Minute
	}

	if config.RateLimit.IPLimit == 0 {
		config.RateLimit.IPLimit = 5
	}
	if config.RateLimit.GlobalLimit == 0 {
		config.RateLimit.GlobalLimit = 100
	}
	if config.RateLimit.Window == 0 {
		config.RateLimit.Window = time.Second
	}
	if config.RateLimit.CleanupInterval == 0 {
		config.RateLimit.CleanupInterval = time.Minute
	}

	if config.Delay.BaseDelay == 0 {
		config.Delay.BaseDelay = 100 * time.Millisecond
	}
	if config.Delay.MaxIPPenalty == 0 {
		config.Delay.MaxIPPenalty = 5 * time.Second
	}
	if config.Delay.MaxGlobalPenalty == 0 {
		config.Delay.MaxGlobalPenalty = 2 * time.Second
	}
	if config.Delay.IPFrequencyFactor == 0 {
		config.Delay.IPFrequencyFactor = 1.5
	}
	if config.Delay.GlobalLoadFactor == 0 {
		config.Delay.GlobalLoadFactor = 1.2
	}
	if config.Delay.IPRateMultiplier == 0 {
		config.Delay.IPRateMultiplier = 2.0
	}
	if config.Delay.GlobalRateMultiplier == 0 {
		config.Delay.GlobalRateMultiplier = 1.5
	}

	if config.Messages.MOTD == "" {
		config.Messages.MOTD = "§6Welcome to the Fake Minecraft Server!"
	}
	if config.Messages.KickMessage == "" {
		config.Messages.KickMessage = "§cServer is under maintenance. Try again later."
	}
	if config.Messages.VersionName == "" {
		config.Messages.VersionName = "1.20.6"
	}
	if config.Messages.ProtocolVersion == 0 {
		config.Messages.ProtocolVersion = 766
	}
	if config.Messages.MaxPlayers == 0 {
		config.Messages.MaxPlayers = 100
	}

	if config.Logging.Level == "" {
		config.Logging.Level = "info"
	}
	if config.Logging.Format == "" {
		config.Logging.Format = "json"
	}
	if config.Logging.Output == "" {
		config.Logging.Output = "stdout"
	}

	if config.Security.MaxPacketSize == 0 {
		config.Security.MaxPacketSize = 1048576 // 1MB
	}
	if config.Security.ConnectionTimeout == 0 {
		config.Security.ConnectionTimeout = 30 * time.Second
	}
}

// validate 验证配置
func validate(config *Config) error {
	if config.Server.Port < 1 || config.Server.Port > 65535 {
		return fmt.Errorf("无效的端口号: %d", config.Server.Port)
	}

	if config.Server.MaxConnections < 1 {
		return fmt.Errorf("最大连接数必须大于 0")
	}

	if config.RateLimit.IPLimit < 1 {
		return fmt.Errorf("IP 限流值必须大于 0")
	}

	if config.RateLimit.GlobalLimit < 1 {
		return fmt.Errorf("全局限流值必须大于 0")
	}

	if config.Delay.IPFrequencyFactor <= 0 {
		return fmt.Errorf("IP 频率因子必须大于 0")
	}

	if config.Delay.GlobalLoadFactor <= 0 {
		return fmt.Errorf("全局负载因子必须大于 0")
	}

	if config.Messages.ProtocolVersion < 1 {
		return fmt.Errorf("协议版本必须大于 0")
	}

	return nil
}

// GetAddress 获取监听地址
func (c *Config) GetAddress() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}

// GetMetricsAddress 获取监控地址
func (c *Config) GetMetricsAddress() string {
	return fmt.Sprintf(":%d", c.Monitoring.MetricsPort)
}
