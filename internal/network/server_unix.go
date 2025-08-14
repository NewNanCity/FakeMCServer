//go:build !windows

package network

import (
	"context"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cloudwego/netpoll"
	"github.com/rs/zerolog"

	"fake-mc-server/internal/config"
)

// Server 网络服务器 (Unix 版本，使用 netpoll)
type Server struct {
	config      *config.Config
	logger      zerolog.Logger
	eventLoop   netpoll.EventLoop
	listener    netpoll.Listener
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

// Connection 连接包装器 (Unix 版本)
type Connection struct {
	netpoll.Connection
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

// NewServer 创建新的服务器 (Unix 版本)
func NewServer(cfg *config.Config, logger zerolog.Logger, handler ConnectionHandler, ctx context.Context) (*Server, error) {
	server := &Server{
		config:  cfg,
		logger:  logger.With().Str("component", "network").Logger(),
		handler: handler,
		ctx:     ctx,
	}

	// 创建监听器
	listener, err := netpoll.CreateListener("tcp", cfg.GetAddress())
	if err != nil {
		return nil, fmt.Errorf("创建监听器失败: %w", err)
	}
	server.listener = listener

	// 创建事件循环
	eventLoop, err := netpoll.NewEventLoop(
		server.onRequest,
		netpoll.WithOnPrepare(server.onPrepare),
		netpoll.WithReadTimeout(cfg.Server.ReadTimeout),
		netpoll.WithIdleTimeout(cfg.Server.IdleTimeout),
	)
	if err != nil {
		listener.Close()
		return nil, fmt.Errorf("创建事件循环失败: %w", err)
	}
	if eventLoop == nil {
		listener.Close()
		return nil, fmt.Errorf("事件循环创建返回 nil")
	}
	server.eventLoop = eventLoop

	logger.Debug().Msg("网络服务器创建成功 (Unix)")
	return server, nil
}

// Start 启动服务器 (Unix 版本)
func (s *Server) Start() error {
	if s == nil {
		return fmt.Errorf("服务器实例为 nil")
	}
	if s.eventLoop == nil {
		return fmt.Errorf("事件循环为 nil")
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
		Msg("启动网络服务器 (Unix)")

	// 启动连接清理协程
	go s.cleanupConnections()

	// 启动生命周期管理协程
	go s.lifecycleManager()

	// 启动事件循环
	s.logger.Debug().Msg("开始启动事件循环")
	return s.eventLoop.Serve(s.listener)
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

	// 停止事件循环
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.eventLoop.Shutdown(shutdownCtx); err != nil {
		s.logger.Error().Err(err).Msg("停止事件循环失败")
	} else {
		s.logger.Info().Msg("网络服务器已停止")
	}
}

// onPrepare 连接准备回调
func (s *Server) onPrepare(connection netpoll.Connection) context.Context {
	// 检查连接数限制
	if s.connCount.Load() >= int64(s.config.Server.MaxConnections) {
		s.logger.Warn().
			Str("remote_addr", connection.RemoteAddr().String()).
			Msg("连接数达到上限，拒绝连接")
		connection.Close()
		return nil
	}

	// 获取远程 IP
	remoteAddr := connection.RemoteAddr().String()
	remoteIP, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		s.logger.Error().
			Err(err).
			Str("remote_addr", remoteAddr).
			Msg("解析远程地址失败")
		connection.Close()
		return nil
	}

	// 创建连接包装器
	connID := fmt.Sprintf("%s-%d", remoteIP, time.Now().UnixNano())
	conn := &Connection{
		Connection: connection,
		ID:         connID,
		RemoteIP:   remoteIP,
		StartTime:  time.Now(),
		State:      StateHandshaking, // 初始状态为握手状态
		Logger: s.logger.With().
			Str("conn_id", connID).
			Str("remote_ip", remoteIP).
			Logger(),
	}

	// 添加关闭回调
	connection.AddCloseCallback(func(connection netpoll.Connection) error {
		s.onConnectionClose(conn)
		return nil
	})

	// 存储连接
	s.connections.Store(connID, conn)
	s.connCount.Add(1)

	// 移除每个连接的建立日志，避免刷屏

	// 返回带有连接信息的上下文
	ctx := context.WithValue(s.ctx, "connection", conn)
	return ctx
}

// onRequest 请求处理回调
func (s *Server) onRequest(ctx context.Context, connection netpoll.Connection) error {
	// 从上下文获取连接信息
	conn, ok := ctx.Value("connection").(*Connection)
	if !ok {
		s.logger.Error().Msg("无法从上下文获取连接信息")
		connection.Close()
		return nil
	}

	// 调用处理器
	if err := s.handler.HandleConnection(ctx, conn); err != nil {
		conn.Logger.Error().Err(err).Msg("处理连接失败")
		connection.Close()
		return err
	}

	return nil
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
