//go:build windows

package network

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"

	"fake-mc-server/internal/config"
)

// Server 网络服务器 (Windows 版本，使用标准库 net)
type Server struct {
	config      *config.Config
	logger      zerolog.Logger
	listener    net.Listener
	handler     ConnectionHandler
	running     atomic.Bool
	connections sync.Map // map[string]*Connection
	connCount   atomic.Int64
	ctx         context.Context
}

// ConnectionHandler 连接处理器接口
type ConnectionHandler interface {
	HandleConnection(ctx context.Context, conn *Connection) error
}

// ConnectionState 连接状态
type ConnectionState int

const (
	StateHandshaking ConnectionState = iota
	StateStatus
	StateLogin
)

// Connection 连接包装器 (Windows 版本)
type Connection struct {
	net.Conn
	ID        string
	RemoteIP  string
	StartTime time.Time
	Logger    zerolog.Logger
	State     ConnectionState
	stateMu   sync.RWMutex
}

// GetState 获取连接状态
func (c *Connection) GetState() ConnectionState {
	c.stateMu.RLock()
	defer c.stateMu.RUnlock()
	return c.State
}

// SetState 设置连接状态
func (c *Connection) SetState(state ConnectionState) {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	c.State = state
}

// NewServer 创建新的服务器 (Windows 版本)
func NewServer(cfg *config.Config, logger zerolog.Logger, handler ConnectionHandler, ctx context.Context) (*Server, error) {
	server := &Server{
		config:  cfg,
		logger:  logger.With().Str("component", "network").Logger(),
		handler: handler,
		ctx:     ctx,
	}

	// 创建监听器
	listener, err := net.Listen("tcp", cfg.GetAddress())
	if err != nil {
		return nil, fmt.Errorf("创建监听器失败: %w", err)
	}
	server.listener = listener

	logger.Debug().Msg("网络服务器创建成功 (Windows)")
	return server, nil
}

// Start 启动服务器 (Windows 版本)
func (s *Server) Start() error {
	if s == nil {
		return fmt.Errorf("服务器实例为 nil")
	}
	if s.listener == nil {
		return fmt.Errorf("监听器为 nil")
	}

	if !s.running.CompareAndSwap(false, true) {
		return fmt.Errorf("服务器已经在运行")
	}

	s.logger.Info().
		Str("address", s.config.GetAddress()).
		Int("max_connections", s.config.Server.MaxConnections).
		Msg("启动网络服务器 (Windows)")

	// 启动连接清理协程
	go s.cleanupConnections()

	// 启动生命周期管理协程
	go s.lifecycleManager()

	// 启动接受连接的循环
	s.logger.Debug().Msg("开始接受连接")
	return s.acceptConnections()
}

// lifecycleManager 生命周期管理
func (s *Server) lifecycleManager() {
	<-s.ctx.Done()

	s.logger.Info().Msg("收到关闭信号，开始停止网络服务器")

	// 标记为停止状态
	s.running.Store(false)

	// 关闭所有连接
	s.connections.Range(func(key, value interface{}) bool {
		if conn, ok := value.(*Connection); ok {
			conn.Close()
		}
		return true
	})

	// 关闭监听器
	if s.listener != nil {
		if err := s.listener.Close(); err != nil {
			s.logger.Error().Err(err).Msg("关闭监听器失败")
		} else {
			s.logger.Info().Msg("网络服务器已停止")
		}
	}
}

// acceptConnections 接受连接的循环
func (s *Server) acceptConnections() error {
	for s.running.Load() {
		select {
		case <-s.ctx.Done():
			return nil
		default:
		}

		conn, err := s.listener.Accept()
		if err != nil {
			if !s.running.Load() || errors.Is(err, net.ErrClosed) || errors.Is(err, context.Canceled) {
				return nil
			}

			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				s.logger.Warn().Err(err).Msg("接受连接出现临时错误，重试")
				time.Sleep(50 * time.Millisecond)
				continue
			}

			select {
			case <-s.ctx.Done():
				return nil
			default:
			}

			s.logger.Error().Err(err).Msg("接受连接失败")
			continue
		}

		go s.handleConnection(conn)
	}

	return nil
}

// handleConnection 处理单个连接
func (s *Server) handleConnection(conn net.Conn) {
	// 检查连接数限制
	if s.connCount.Load() >= int64(s.config.Server.MaxConnections) {
		s.logger.Warn().
			Str("remote_addr", conn.RemoteAddr().String()).
			Msg("连接数达到上限，拒绝连接")
		conn.Close()
		return
	}

	// 获取远程 IP
	remoteAddr := conn.RemoteAddr().String()
	remoteIP, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		s.logger.Error().
			Err(err).
			Str("remote_addr", remoteAddr).
			Msg("解析远程地址失败")
		conn.Close()
		return
	}

	// 创建连接包装器
	connID := fmt.Sprintf("%s-%d", remoteIP, time.Now().UnixNano())
	connection := &Connection{
		Conn:      conn,
		ID:        connID,
		RemoteIP:  remoteIP,
		StartTime: time.Now(),
		State:     StateHandshaking, // 初始状态为握手状态
		Logger: s.logger.With().
			Str("conn_id", connID).
			Str("remote_ip", remoteIP).
			Logger(),
	}

	// 存储连接
	s.connections.Store(connID, connection)
	s.connCount.Add(1)

	// 移除每个连接的建立日志，避免刷屏

	// 处理连接
	ctx := context.WithValue(s.ctx, "connection", connection)
	if err := s.handler.HandleConnection(ctx, connection); err != nil {
		connection.Logger.Error().Err(err).Msg("处理连接失败")
	}

	// 清理连接
	s.onConnectionClose(connection)
}

// onConnectionClose 连接关闭回调
func (s *Server) onConnectionClose(conn *Connection) {
	// 只记录长连接的关闭信息
	duration := time.Since(conn.StartTime)
	if duration > 30*time.Second {
		conn.Logger.Info().
			Dur("duration", duration).
			Msg("长连接关闭")
	}

	// 从连接映射中移除
	s.connections.Delete(conn.ID)
	s.connCount.Add(-1)
}

// cleanupConnections 清理过期连接
func (s *Server) cleanupConnections() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.cleanupExpiredConnections()
		}
	}
}

// cleanupExpiredConnections 清理过期连接
func (s *Server) cleanupExpiredConnections() {
	now := time.Now()
	maxIdleTime := s.config.Server.IdleTimeout

	s.connections.Range(func(key, value interface{}) bool {
		if conn, ok := value.(*Connection); ok {
			if now.Sub(conn.StartTime) > maxIdleTime {
				conn.Logger.Info().Msg("清理过期连接")
				conn.Close()
				s.connections.Delete(key)
				s.connCount.Add(-1)
			}
		}
		return true
	})
}

// GetStats 获取服务器统计信息
func (s *Server) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"connection_count": s.connCount.Load(),
		"running":          s.running.Load(),
	}
}
