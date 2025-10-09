package protocol

import (
	"context"
	"fmt"
	"time"

	"github.com/Tnze/go-mc/chat"
	"github.com/Tnze/go-mc/net"
	pk "github.com/Tnze/go-mc/net/packet"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"fake-mc-server/internal/config"
	"fake-mc-server/internal/logger"
	"fake-mc-server/internal/network"
	"fake-mc-server/internal/sync"
)

// GoMCHandler 基于go-mc库的处理器
// 使用go-mc的标准服务器框架，提供更好的兼容性
type GoMCHandler struct {
	config         *config.Config
	logger         zerolog.Logger
	upstreamSyncer *sync.UpstreamSyncer
	honeypotLogger *logger.HoneypotLogger
	limiter        RateLimiter
}

// NewGoMCHandler 创建新的GoMC处理器
func NewGoMCHandler(
	cfg *config.Config,
	logger zerolog.Logger,
	upstreamSyncer *sync.UpstreamSyncer,
	honeypotLogger *logger.HoneypotLogger,
	limiter RateLimiter,
) *GoMCHandler {
	return &GoMCHandler{
		config:         cfg,
		logger:         logger.With().Str("handler", "gomc").Logger(),
		upstreamSyncer: upstreamSyncer,
		honeypotLogger: honeypotLogger,
		limiter:        limiter,
	}
}

// HandleConnection 处理连接（实现network.ConnectionHandler接口）
func (h *GoMCHandler) HandleConnection(ctx context.Context, conn *network.Connection) error {
	// 检查限流
	if !h.limiter.Allow(conn.RemoteIP) {
		conn.Logger.Warn().Msg("触发限流，直接断开连接")
		return fmt.Errorf("限流")
	}

	// 计算并应用延迟
	delay := h.limiter.CalculateDelay(conn.RemoteIP)
	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	// 将network.Connection转换为go-mc的net.Conn
	mcConn := h.wrapConnection(conn)
	defer mcConn.Close()

	// 处理握手
	protocol, intention, err := h.handleHandshake(mcConn)
	if err != nil {
		conn.Logger.Debug().Err(err).Msg("握手失败")
		return err
	}

	// 根据意图处理
	switch intention {
	case 1: // 状态查询
		return h.handleStatusQuery(mcConn, conn, protocol)
	case 2: // 登录
		return h.handleLogin(mcConn, conn, protocol, delay)
	default:
		conn.Logger.Warn().Int("intention", int(intention)).Msg("未知意图")
		return fmt.Errorf("unknown intention: %d", intention)
	}
}

// wrapConnection 将network.Connection包装为go-mc的net.Conn
func (h *GoMCHandler) wrapConnection(conn *network.Connection) *net.Conn {
	// go-mc 的 net.Conn 需要 Socket、Reader 和 Writer 三个字段
	// network.Connection 嵌入了 net.Conn，实现了 io.Reader 和 io.Writer
	mcConn := &net.Conn{
		Socket: conn,
		Reader: conn,
		Writer: conn,
	}
	// 设置压缩阈值为 -1（禁用压缩，握手阶段不使用压缩）
	mcConn.SetThreshold(-1)
	return mcConn
}

// handleHandshake 处理握手包
func (h *GoMCHandler) handleHandshake(conn *net.Conn) (protocol int32, intention int32, err error) {
	var p pk.Packet
	err = conn.ReadPacket(&p)
	if err != nil {
		return
	}

	// 握手包ID是0x00，不需要检查
	if p.ID != 0x00 {
		err = fmt.Errorf("expected handshake packet, got %#02X", p.ID)
		return
	}

	var (
		protocolVersion pk.VarInt
		serverAddress   pk.String
		serverPort      pk.UnsignedShort
		nextState       pk.VarInt
	)

	err = p.Scan(&protocolVersion, &serverAddress, &serverPort, &nextState)
	if err != nil {
		return
	}

	protocol = int32(protocolVersion)
	intention = int32(nextState)

	h.logger.Debug().
		Int32("protocol", protocol).
		Str("address", string(serverAddress)).
		Int("port", int(serverPort)).
		Int32("intention", intention).
		Msg("收到握手包")

	return
}

// handleStatusQuery 处理状态查询
func (h *GoMCHandler) handleStatusQuery(mcConn *net.Conn, conn *network.Connection, protocol int32) error {
	var p pk.Packet

	// 最多处理2个包（状态请求和Ping）
	for i := 0; i < 2; i++ {
		err := mcConn.ReadPacket(&p)
		if err != nil {
			return err
		}

		switch p.ID {
		case 0x00: // 状态请求
			conn.Logger.Debug().Msg("收到状态请求")

			// 构建状态响应
			statusJSON := h.buildStatusResponse(protocol)

			// 发送响应
			err = mcConn.WritePacket(pk.Marshal(
				0x00, // ClientboundStatusStatusResponse
				pk.String(statusJSON),
			))
			if err != nil {
				return fmt.Errorf("发送状态响应失败: %w", err)
			}

			conn.Logger.Debug().Msg("发送状态响应")

		case 0x01: // Ping请求
			conn.Logger.Debug().Msg("收到Ping请求")

			// 直接回显Ping包
			err = mcConn.WritePacket(p)
			if err != nil {
				return fmt.Errorf("发送Pong响应失败: %w", err)
			}

			conn.Logger.Debug().Msg("发送Pong响应")
		}
	}

	return nil
}

// buildStatusResponse 构建状态响应JSON
func (h *GoMCHandler) buildStatusResponse(protocol int32) string {
	// 如果有上游同步的状态，使用上游状态
	if h.upstreamSyncer != nil && h.upstreamSyncer.IsRunning() {
		if cachedStatus := h.upstreamSyncer.GetRawResponse(); len(cachedStatus) > 0 {
			return string(cachedStatus)
		}
	}

	// 否则使用配置的默认状态
	return fmt.Sprintf(`{
		"version": {
			"name": "%s",
			"protocol": %d
		},
		"players": {
			"max": %d,
			"online": %d,
			"sample": []
		},
		"description": {
			"text": "%s"
		}
	}`,
		h.config.Messages.VersionName,
		h.config.Messages.ProtocolVersion,
		h.config.Messages.MaxPlayers,
		h.config.Messages.OnlinePlayers,
		h.config.Messages.MOTD,
	)
}

// handleLogin 处理登录请求
func (h *GoMCHandler) handleLogin(mcConn *net.Conn, conn *network.Connection, protocol int32, baseDelay time.Duration) error {
	// 应用额外的登录延迟
	loginDelay := h.limiter.CalculateDelay(conn.RemoteIP)
	if loginDelay > 0 {
		time.Sleep(loginDelay)
	}

	// 读取登录开始包
	var p pk.Packet
	err := mcConn.ReadPacket(&p)
	if err != nil {
		return err
	}

	if p.ID != 0x00 { // ServerboundLoginHello
		return fmt.Errorf("expected login hello packet, got %#02X", p.ID)
	}

	var (
		username pk.String
		playerID pk.UUID
	)

	err = p.Scan(&username, &playerID)
	if err != nil {
		return err
	}

	conn.Logger.Info().
		Str("username", string(username)).
		Str("uuid", uuid.UUID(playerID).String()).
		Msg("收到登录请求")

	// 记录蜜罐登录尝试事件
	if h.honeypotLogger.IsEnabled() {
		delayMs := loginDelay.Milliseconds()
		h.honeypotLogger.LogLoginAttempt(conn.RemoteIP, string(username), delayMs)
	}

	// 构建并发送断开连接包
	kickMessage := chat.Message{Text: h.config.Messages.KickMessage}
	err = mcConn.WritePacket(pk.Marshal(
		0x00, // ClientboundLoginLoginDisconnect
		kickMessage,
	))
	if err != nil {
		return fmt.Errorf("发送登录断开连接包失败: %w", err)
	}

	conn.Logger.Info().
		Str("kick_message", h.config.Messages.KickMessage).
		Msg("发送登录断开连接包")

	return nil
}
