package protocol

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/Tnze/go-mc/net/packet"
	"github.com/rs/zerolog"

	"fake-mc-server/internal/config"
	"fake-mc-server/internal/logger"
	"fake-mc-server/internal/network"
	"fake-mc-server/internal/pool"
	"fake-mc-server/internal/sync"
)

// FastHandler 快速协议处理器
type FastHandler struct {
	config         *config.Config
	logger         zerolog.Logger
	syncer         *sync.UpstreamSyncer
	limiter        RateLimiter
	responsePool   *pool.ResponsePool
	honeypotLogger *logger.HoneypotLogger
}

// NewFastHandler 创建快速协议处理器
func NewFastHandler(cfg *config.Config, logger zerolog.Logger, syncer *sync.UpstreamSyncer, limiter RateLimiter, honeypotLogger *logger.HoneypotLogger) *FastHandler {
	return &FastHandler{
		config:         cfg,
		logger:         logger.With().Str("component", "fast_protocol_handler").Logger(),
		syncer:         syncer,
		limiter:        limiter,
		responsePool:   pool.NewResponsePool(),
		honeypotLogger: honeypotLogger,
	}
}

// HandleConnection 处理连接
func (h *FastHandler) HandleConnection(ctx context.Context, conn *network.Connection) error {
	// 生产环境不记录连接尝试，减少日志量

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

	// 处理多个数据包（类似 SimpleHandler）
	buffer := make([]byte, MaxPacketSize)

	for {
		// 设置读取超时
		conn.SetReadDeadline(time.Now().Add(ReadTimeout * time.Second))

		n, err := conn.Read(buffer)
		if err != nil {
			if err == io.EOF {
				break
			}
			// 超时或其他错误，结束处理
			break
		}

		// 快速处理数据包
		if n > 0 {
			err := h.processPacketFast(conn, buffer[:n], delay)
			if err != nil {
				// 处理失败，结束连接
				return err
			}
		}
	}

	return nil
}

// processPacketFast 快速处理数据包（简化版，类似原始实现）
func (h *FastHandler) processPacketFast(conn *network.Connection, data []byte, baseDelay time.Duration) error {
	// 1. 快速预检查
	if err := h.quickPreCheck(data); err != nil {
		return h.rejectSilently(conn, err.Error(), baseDelay)
	}

	// 2. 对于1字节的数据包，直接发送状态响应（兼容简单查询工具）
	if len(data) == 1 {
		conn.Logger.Debug().Msg("收到1字节数据包，发送状态响应")
		return h.handleStatusRequestFast(conn)
	}

	// 3. 检查是否是状态相关包（包ID 0x00）- 包括握手包和状态请求包
	if data[1] == 0x00 {
		// 尝试解析握手包（如果是长包）
		if len(data) >= 7 {
			handshake, err := h.parseHandshakeFast(data)
			if err == nil {
				// 成功解析握手包，记录信息
				conn.Logger.Info().
					Int("protocol", handshake.ProtocolVersion).
					Str("address", handshake.ServerAddress).
					Int("port", int(handshake.ServerPort)).
					Int("intention", handshake.NextState).
					Msg("收到握手包")

				// 记录蜜罐事件（优化版：不记录connID和dataHex）
				if h.honeypotLogger.IsEnabled() {
					h.honeypotLogger.LogHandshake(
						conn.RemoteIP,
						handshake.ProtocolVersion,
						handshake.ServerAddress,
						handshake.ServerPort,
						handshake.NextState,
					)
				}

				// 如果是登录意图，直接处理
				if handshake.NextState == 2 {
					return h.handleLoginFast(conn)
				}
			}
		}

		// 对所有包ID为0x00的包（握手包或状态请求包）都发送状态响应
		return h.handleStatusRequestFast(conn)
	}

	// 4. 检查是否是 Ping 包（包ID 0x01）
	if data[1] == 0x01 {
		return h.handlePingRequestFast(conn, data)
	}

	// 5. 未知协议包，但不立即拒绝，先尝试发送状态响应（更宽松的处理）
	conn.Logger.Debug().Bytes("data", data).Msg("收到未知协议包，尝试发送状态响应")
	return h.handleStatusRequestFast(conn)
}

// quickPreCheck 快速预检查
func (h *FastHandler) quickPreCheck(data []byte) error {
	// 大小检查
	if len(data) > MaxPacketSize {
		return fmt.Errorf("packet too large: %d", len(data))
	}

	if len(data) < 1 { // 最小数据包大小（放宽到1字节，兼容更多查询工具）
		return fmt.Errorf("packet too small: %d", len(data))
	}

	return nil
}

// parseHandshakeFast 快速解析握手包
func (h *FastHandler) parseHandshakeFast(data []byte) (*HandshakeInfo, error) {
	r := bytes.NewReader(data)

	// 跳过包长度
	var packetLen packet.VarInt
	if _, err := packetLen.ReadFrom(r); err != nil {
		return nil, fmt.Errorf("invalid packet length")
	}

	// 跳过包ID
	var packetID packet.VarInt
	if _, err := packetID.ReadFrom(r); err != nil {
		return nil, fmt.Errorf("invalid packet id")
	}

	// 解析协议版本
	var protocol packet.VarInt
	if _, err := protocol.ReadFrom(r); err != nil {
		return nil, fmt.Errorf("invalid protocol")
	}

	// 验证协议版本
	if protocol < 47 || protocol > 1000 {
		return nil, fmt.Errorf("unsupported protocol: %d", protocol)
	}

	// 解析服务器地址
	var address packet.String
	if _, err := address.ReadFrom(r); err != nil {
		return nil, fmt.Errorf("invalid address")
	}

	// 验证地址长度
	if len(address) > MaxStringLen {
		return nil, fmt.Errorf("address too long: %d", len(address))
	}

	// 解析端口
	var port packet.UnsignedShort
	if _, err := port.ReadFrom(r); err != nil {
		return nil, fmt.Errorf("invalid port")
	}

	// 解析意图
	var intention packet.VarInt
	if _, err := intention.ReadFrom(r); err != nil {
		return nil, fmt.Errorf("invalid intention")
	}

	// 验证意图
	if intention != 1 && intention != 2 {
		return nil, fmt.Errorf("invalid intention: %d", intention)
	}

	return &HandshakeInfo{
		ProtocolVersion: int(protocol),
		ServerAddress:   string(address),
		ServerPort:      uint16(port),
		NextState:       int(intention),
	}, nil
}

// handleLoginFast 快速处理登录请求
func (h *FastHandler) handleLoginFast(conn *network.Connection) error {
	// 应用额外的登录延迟
	loginDelay := h.limiter.CalculateDelay(conn.RemoteIP)
	if loginDelay > 0 {
		time.Sleep(loginDelay)
	}

	// 构建断开连接包
	kickJSON := fmt.Sprintf(`{"text":"%s"}`, h.config.Messages.KickMessage)
	response := packet.Marshal(0x00, packet.String(kickJSON))

	var buf bytes.Buffer
	if err := response.Pack(&buf, -1); err != nil {
		return fmt.Errorf("pack login disconnect failed: %w", err)
	}

	// 发送断开连接包
	if _, err := conn.Write(buf.Bytes()); err != nil {
		return fmt.Errorf("send login disconnect failed: %w", err)
	}

	// 记录蜜罐登录尝试事件（优化版：不记录connID和kickMsg）
	if h.honeypotLogger.IsEnabled() {
		delayMs := loginDelay.Milliseconds()
		h.honeypotLogger.LogLoginAttempt(conn.RemoteIP, "", delayMs) // 没有用户名
	}

	conn.Logger.Info().
		Str("kick_message", h.config.Messages.KickMessage).
		Msg("发送登录断开连接包")

	return nil
}

// buildServerStatus 构建服务器状态 JSON
func (h *FastHandler) buildServerStatus() string {
	// 优先使用上游同步的响应
	if h.syncer != nil {
		cachedResp := h.syncer.GetRawResponse()
		if len(cachedResp) > 0 {
			return string(cachedResp)
		}
	}

	// 构建默认状态响应
	return fmt.Sprintf(`{"version":{"name":"%s","protocol":%d},"players":{"max":%d,"online":%d},"description":{"text":"%s"}}`,
		h.config.Messages.VersionName,
		h.config.Messages.ProtocolVersion,
		h.config.Messages.MaxPlayers,
		h.config.Messages.OnlinePlayers,
		h.config.Messages.MOTD)
}

// rejectSilently 静默拒绝连接
func (h *FastHandler) rejectSilently(conn *network.Connection, reason string, delay time.Duration) error {
	conn.Logger.Warn().Str("reason", reason).Msg("静默拒绝连接")

	// 记录蜜罐协议违规事件（优化版：不记录connID和dataHex）
	if h.honeypotLogger.IsEnabled() {
		h.honeypotLogger.LogProtocolViolation(conn.RemoteIP, reason)
	}

	// 应用延迟让攻击者以为服务器在处理
	if delay > 0 {
		time.Sleep(delay)
	}

	return fmt.Errorf("rejected: %s", reason)
}

// handleStatusRequestFast 快速处理状态请求
func (h *FastHandler) handleStatusRequestFast(conn *network.Connection) error {
	conn.Logger.Debug().Msg("收到状态请求包")

	// 检查连接是否仍然有效
	if !h.isConnectionValid(conn) {
		conn.Logger.Debug().Msg("连接已关闭，跳过状态响应")
		return fmt.Errorf("connection closed")
	}

	// 构建并发送状态响应
	statusJSON := h.buildServerStatus()
	response := packet.Marshal(0x00, packet.String(statusJSON))

	var buf bytes.Buffer
	if err := response.Pack(&buf, -1); err != nil {
		return fmt.Errorf("pack status response failed: %w", err)
	}

	// 再次检查连接状态，然后发送响应
	if !h.isConnectionValid(conn) {
		conn.Logger.Debug().Msg("连接在构建响应时已关闭")
		return fmt.Errorf("connection closed during response building")
	}

	if _, err := conn.Write(buf.Bytes()); err != nil {
		return fmt.Errorf("send status response failed: %w", err)
	}

	conn.Logger.Debug().Msg("发送状态响应")
	return nil // 继续处理后续数据包（可能的 ping）
}

// isConnectionValid 检查连接是否仍然有效
func (h *FastHandler) isConnectionValid(conn *network.Connection) bool {
	// 简化的连接检查：尝试设置读取超时
	// 如果设置失败，说明连接已经关闭
	err := conn.SetReadDeadline(time.Now().Add(ReadTimeout * time.Second))
	return err == nil
}

// handlePingRequestFast 快速处理 ping 请求（采用原始实现的方式）
func (h *FastHandler) handlePingRequestFast(conn *network.Connection, data []byte) error {
	// 检查连接是否仍然有效
	if !h.isConnectionValid(conn) {
		conn.Logger.Debug().Msg("连接已关闭，跳过Ping响应")
		return fmt.Errorf("connection closed")
	}

	// 提取时间戳（跳过包长度和包ID）- 采用原始实现的逻辑
	var timestamp []byte
	if len(data) >= 10 { // 包长度(1) + 包ID(1) + 时间戳(8)
		timestamp = data[2:10]
	} else {
		// 如果没有时间戳，使用简单填充（与原始实现一致）
		timestamp = make([]byte, 8)
		for i := range 8 {
			timestamp[i] = byte(i)
		}
	}

	// 构建 Pong 响应包（采用原始实现的方式）
	// 包长度 = 1(包ID) + 8(时间戳)
	packetLen := 1 + 8
	packetLenVarInt := h.encodeVarInt(packetLen)

	response := make([]byte, 0, len(packetLenVarInt)+packetLen)
	response = append(response, packetLenVarInt...) // 包长度
	response = append(response, 0x01)               // 包ID (Pong)
	response = append(response, timestamp...)       // 时间戳

	// 再次检查连接状态，然后发送响应
	if !h.isConnectionValid(conn) {
		conn.Logger.Debug().Msg("连接在构建Pong响应时已关闭")
		return fmt.Errorf("connection closed during pong building")
	}

	// 发送响应
	if _, err := conn.Write(response); err != nil {
		return fmt.Errorf("发送 Pong 响应失败: %w", err)
	}

	conn.Logger.Debug().Msg("发送 pong 响应")
	return nil // 继续处理后续数据包
}

// encodeVarInt 编码 VarInt（从原始实现复制）
func (h *FastHandler) encodeVarInt(value int) []byte {
	var result []byte
	for {
		temp := byte(value & 0x7F)
		value >>= 7
		if value != 0 {
			temp |= 0x80
		}
		result = append(result, temp)
		if value == 0 {
			break
		}
	}
	return result
}
