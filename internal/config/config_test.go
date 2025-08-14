package config

import (
	"os"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	// 创建临时配置文件
	configContent := `
server:
  host: "127.0.0.1"
  port: 25566
  max_connections: 1000

upstream:
  enabled: true
  address: "test.example.com"

rate_limit:
  ip_limit: 10
  global_limit: 200

delay:
  base_delay: "200ms"
  max_ip_penalty: "3s"

messages:
  motd: "Test Server"
  version_name: "1.20.6"
  protocol_version: 766
`

	// 写入临时文件
	tmpFile, err := os.CreateTemp("", "config_test_*.yml")
	if err != nil {
		t.Fatalf("创建临时文件失败: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(configContent); err != nil {
		t.Fatalf("写入临时文件失败: %v", err)
	}
	tmpFile.Close()

	// 加载配置
	cfg, err := Load(tmpFile.Name())
	if err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}

	// 验证配置
	if cfg.Server.Host != "127.0.0.1" {
		t.Errorf("期望 host 为 '127.0.0.1'，实际为 '%s'", cfg.Server.Host)
	}

	if cfg.Server.Port != 25566 {
		t.Errorf("期望 port 为 25566，实际为 %d", cfg.Server.Port)
	}

	if cfg.Server.MaxConnections != 1000 {
		t.Errorf("期望 max_connections 为 1000，实际为 %d", cfg.Server.MaxConnections)
	}

	if cfg.Upstream.Address != "test.example.com" {
		t.Errorf("期望 upstream address 为 'test.example.com'，实际为 '%s'", cfg.Upstream.Address)
	}

	if cfg.RateLimit.IPLimit != 10 {
		t.Errorf("期望 ip_limit 为 10，实际为 %d", cfg.RateLimit.IPLimit)
	}

	if cfg.Delay.BaseDelay != 200*time.Millisecond {
		t.Errorf("期望 base_delay 为 200ms，实际为 %v", cfg.Delay.BaseDelay)
	}

	if cfg.Messages.MOTD != "Test Server" {
		t.Errorf("期望 motd 为 'Test Server'，实际为 '%s'", cfg.Messages.MOTD)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "有效配置",
			config: &Config{
				Server: ServerConfig{
					Port:           25565,
					MaxConnections: 1000,
				},
				RateLimit: RateLimitConfig{
					IPLimit:     5,
					GlobalLimit: 100,
				},
				Delay: DelayConfig{
					IPFrequencyFactor: 1.5,
					GlobalLoadFactor:  1.2,
				},
				Messages: MessagesConfig{
					ProtocolVersion: 766,
				},
			},
			wantErr: false,
		},
		{
			name: "无效端口",
			config: &Config{
				Server: ServerConfig{
					Port:           70000,
					MaxConnections: 1000,
				},
				RateLimit: RateLimitConfig{
					IPLimit:     5,
					GlobalLimit: 100,
				},
				Delay: DelayConfig{
					IPFrequencyFactor: 1.5,
					GlobalLoadFactor:  1.2,
				},
				Messages: MessagesConfig{
					ProtocolVersion: 766,
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetAddress(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{
			Host: "192.168.1.100",
			Port: 25565,
		},
	}

	expected := "192.168.1.100:25565"
	actual := cfg.GetAddress()

	if actual != expected {
		t.Errorf("期望地址为 '%s'，实际为 '%s'", expected, actual)
	}
}

func TestSetDefaults(t *testing.T) {
	cfg := &Config{}
	setDefaults(cfg)

	// 检查一些默认值
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("期望默认 host 为 '0.0.0.0'，实际为 '%s'", cfg.Server.Host)
	}

	if cfg.Server.Port != 25565 {
		t.Errorf("期望默认 port 为 25565，实际为 %d", cfg.Server.Port)
	}

	if cfg.RateLimit.IPLimit != 5 {
		t.Errorf("期望默认 ip_limit 为 5，实际为 %d", cfg.RateLimit.IPLimit)
	}

	if cfg.Delay.BaseDelay != 100*time.Millisecond {
		t.Errorf("期望默认 base_delay 为 100ms，实际为 %v", cfg.Delay.BaseDelay)
	}
}
