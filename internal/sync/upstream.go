package sync

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Tnze/go-mc/bot"
	"github.com/bytedance/sonic"
	"github.com/rs/zerolog"

	"fake-mc-server/internal/config"
)

// UpstreamSyncer 上游服务器状态同步器
type UpstreamSyncer struct {
	config              *config.Config
	logger              zerolog.Logger
	cachedResponse      []byte // 缓存的原始响应（唯一重要的状态）
	upstreamUnavailable bool   // 上游服务器是否不可用
	mu                  sync.RWMutex
	ctx                 context.Context
	running             bool
}

// NewUpstreamSyncer 创建上游同步器
func NewUpstreamSyncer(cfg *config.Config, logger zerolog.Logger, ctx context.Context) *UpstreamSyncer {
	syncer := &UpstreamSyncer{
		config:              cfg,
		logger:              logger.With().Str("component", "upstream_syncer").Logger(),
		upstreamUnavailable: false,
		ctx:                 ctx,
	}

	// 初始化默认响应
	syncer.cachedResponse = syncer.createDefaultResponse()

	return syncer
}

// createDefaultResponse 创建默认的 JSON 响应
func (us *UpstreamSyncer) createDefaultResponse() []byte {
	defaultResponse := map[string]any{
		"version": map[string]any{
			"name":     us.config.Messages.VersionName,
			"protocol": us.config.Messages.ProtocolVersion,
		},
		"players": map[string]any{
			"max":    us.config.Messages.MaxPlayers,
			"online": us.config.Messages.OnlinePlayers,
		},
		"description": map[string]any{
			"text": us.config.Messages.MOTD,
		},
		"favicon": "", // 默认无图标
	}

	resp, err := sonic.Marshal(defaultResponse)
	if err != nil {
		us.logger.Error().Err(err).Msg("创建默认响应失败")
		// 返回最基本的响应
		return []byte(`{"version":{"name":"1.20.6","protocol":766},"players":{"max":100,"online":0},"description":{"text":"Minecraft Server"}}`)
	}

	// 移除默认响应创建的详细日志

	return resp
}

// Start 启动同步器
func (us *UpstreamSyncer) Start() error {
	if !us.config.Upstream.Enabled {
		us.logger.Info().Msg("上游同步已禁用")
		return nil
	}

	if us.running {
		return fmt.Errorf("同步器已在运行")
	}

	us.running = true
	us.logger.Info().
		Str("address", us.config.Upstream.Address).
		Dur("interval", us.config.Upstream.SyncInterval).
		Msg("启动上游状态同步")

	// 立即执行一次同步
	us.syncOnce()

	// 在 goroutine 中启动定时同步（非阻塞）
	go us.syncLoop()

	return nil
}

// GetRawResponse 获取缓存的原始响应（用于直接响应客户端）
func (us *UpstreamSyncer) GetRawResponse() []byte {
	us.mu.RLock()
	defer us.mu.RUnlock()
	return us.cachedResponse
}

// syncLoop 同步循环
func (us *UpstreamSyncer) syncLoop() {
	ticker := time.NewTicker(us.config.Upstream.SyncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-us.ctx.Done():
			return
		case <-ticker.C:
			us.syncOnce()
		}
	}
}

// syncOnce 执行一次同步
func (us *UpstreamSyncer) syncOnce() {
	start := time.Now()

	// 解析服务器地址
	addr, err := us.resolveAddress()
	if err != nil {
		us.logger.Error().Err(err).Msg("解析服务器地址失败")
		us.updateStateOffline()
		return
	}

	// 尝试多次重试
	var lastErr error
	for attempt := 0; attempt <= us.config.Upstream.RetryCount; attempt++ {
		if attempt > 0 {
			us.logger.Debug().
				Int("attempt", attempt).
				Dur("delay", us.config.Upstream.RetryInterval).
				Msg("重试同步")
			time.Sleep(us.config.Upstream.RetryInterval)
		}

		resp, err := us.pingServer(addr)
		if err != nil {
			lastErr = err
			us.logger.Debug().
				Err(err).
				Int("attempt", attempt).
				Msg("同步失败")
			continue
		}

		// 同步成功
		us.updateState(resp)

		// 只记录重要的同步成功信息
		us.logger.Info().
			Str("upstream", addr).
			Dur("response_time", time.Since(start)).
			Msg("上游同步成功")
		return
	}

	// 所有重试都失败了
	us.logger.Warn().
		Err(lastErr).
		Str("addr", addr).
		Int("retry_count", us.config.Upstream.RetryCount).
		Msg("同步失败，所有重试都已用尽")

	us.updateStateOffline()
}

// resolveAddress 解析服务器地址
func (us *UpstreamSyncer) resolveAddress() (string, error) {
	// go-mc 库会自动处理各种地址格式：
	// - IP 地址: "192.168.1.1" 或 "192.168.1.1:25565"
	// - 域名: "example.com" 或 "example.com:25565"
	// - SRV 记录: "mc.example.com" (自动查询 _minecraft._tcp.mc.example.com)
	return us.config.Upstream.Address, nil
}

// pingServer 查询服务器状态，返回原始响应
func (us *UpstreamSyncer) pingServer(addr string) ([]byte, error) {
	// 使用 go-mc 的 PingAndListTimeout 函数
	resp, _, err := bot.PingAndListTimeout(addr, us.config.Upstream.Timeout)
	if err != nil {
		return nil, fmt.Errorf("ping 失败: %w", err)
	}

	// 成功获取响应，直接返回原始 byte[]
	// 移除每次同步的详细日志，避免刷屏

	return resp, nil
}

// updateState 更新状态（成功获取上游响应时调用）
func (us *UpstreamSyncer) updateState(resp []byte) {
	us.mu.Lock()
	defer us.mu.Unlock()

	// 根据配置决定是否覆盖版本信息
	if us.config.Upstream.OverrideVersion {
		// 覆盖上游响应中的版本信息为配置的版本
		modifiedResp := us.overrideVersionInfo(resp)
		if modifiedResp != nil {
			// 缓存修改后的响应
			us.cachedResponse = modifiedResp
			us.logger.Debug().Msg("已覆盖上游版本信息")
		} else {
			// 如果修改失败，使用原始响应
			us.cachedResponse = resp
			us.logger.Warn().Msg("版本信息覆盖失败，使用原始响应")
		}
	} else {
		// 直接使用上游响应（保留 Velocity 的版本信息）
		us.cachedResponse = resp
		us.logger.Debug().Msg("使用上游原始版本信息")
	}

	// 重置上游不可用标志
	us.upstreamUnavailable = false

	// 移除状态更新的详细日志
}

// updateStateOffline 更新为离线状态（上游不可用时调用）
func (us *UpstreamSyncer) updateStateOffline() {
	us.mu.Lock()
	defer us.mu.Unlock()

	// 如果上游已经标记为不可用，直接返回，不重复处理
	if us.upstreamUnavailable {
		// 静默跳过，避免重复日志
		return
	}

	// 如果有缓存的响应，解析并修改在线人数为 0
	if len(us.cachedResponse) > 0 {
		modifiedResponse := us.createOfflineResponse(us.cachedResponse)
		if modifiedResponse != nil {
			// 更新缓存为修改后的响应
			us.cachedResponse = modifiedResponse
			us.logger.Info().Msg("上游不可用，已将缓存响应的在线人数设为 0")
		}
	} else {
		// 没有缓存，使用默认响应（在线人数为 0）
		us.cachedResponse = us.createDefaultResponse()
		us.logger.Info().Msg("上游不可用且无缓存，使用默认离线状态")
	}

	// 标记上游不可用
	us.upstreamUnavailable = true
}

// overrideVersionInfo 覆盖上游响应中的版本信息为配置的版本
func (us *UpstreamSyncer) overrideVersionInfo(upstreamResp []byte) []byte {
	var serverInfo map[string]any
	if err := sonic.Unmarshal(upstreamResp, &serverInfo); err != nil {
		us.logger.Error().Err(err).Msg("解析上游响应失败，使用原始响应")
		return nil
	}

	// 替换版本信息为配置的版本
	if version, ok := serverInfo["version"].(map[string]any); ok {
		version["name"] = us.config.Messages.VersionName
		version["protocol"] = us.config.Messages.ProtocolVersion
		us.logger.Debug().
			Str("version_name", us.config.Messages.VersionName).
			Int("protocol_version", us.config.Messages.ProtocolVersion).
			Msg("已覆盖上游响应的版本信息")
	} else {
		// 如果上游响应中没有 version 字段，添加它
		serverInfo["version"] = map[string]any{
			"name":     us.config.Messages.VersionName,
			"protocol": us.config.Messages.ProtocolVersion,
		}
		us.logger.Warn().Msg("上游响应缺少 version 字段，已添加配置的版本信息")
	}

	// 重新序列化
	modifiedResp, err := sonic.Marshal(serverInfo)
	if err != nil {
		us.logger.Error().Err(err).Msg("序列化修改后的响应失败")
		return nil
	}

	return modifiedResp
}

// createOfflineResponse 从缓存的响应创建离线响应（在线人数为 0）
func (us *UpstreamSyncer) createOfflineResponse(cachedResp []byte) []byte {
	var serverInfo map[string]any
	if err := sonic.Unmarshal(cachedResp, &serverInfo); err != nil {
		us.logger.Error().Err(err).Msg("解析缓存响应失败")
		return nil
	}

	// 修改在线人数为 0
	if players, ok := serverInfo["players"].(map[string]any); ok {
		players["online"] = 0
	}

	// 重新序列化
	modifiedResp, err := sonic.Marshal(serverInfo)
	if err != nil {
		us.logger.Error().Err(err).Msg("序列化修改后的响应失败")
		return nil
	}

	return modifiedResp
}

// IsRunning 检查是否在运行
func (us *UpstreamSyncer) IsRunning() bool {
	return us.running
}

// GetStats 获取统计信息
func (us *UpstreamSyncer) GetStats() map[string]any {
	us.mu.RLock()
	defer us.mu.RUnlock()

	return map[string]any{
		"running":              us.running,
		"enabled":              us.config.Upstream.Enabled,
		"upstream_address":     us.config.Upstream.Address,
		"upstream_available":   !us.upstreamUnavailable,
		"cached_response_size": len(us.cachedResponse),
	}
}
