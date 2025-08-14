package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"fake-mc-server/internal/config"
	"fake-mc-server/internal/limiter"
	"fake-mc-server/internal/logger"
	"fake-mc-server/internal/network"
	"fake-mc-server/internal/protocol"
	"fake-mc-server/internal/sync"
)

// 构建时注入的版本信息
var (
	version   = "dev"     // 通过 -ldflags 注入
	buildTime = "unknown" // 通过 -ldflags 注入
	gitCommit = "unknown" // 通过 -ldflags 注入
)

var (
	configPath  = flag.String("config", "config/config.yml", "配置文件路径")
	showVersion = flag.Bool("version", false, "显示版本信息")
)

const (
	AppName = "FakeMCServer"
)

// printVersion 显示详细的版本信息
func printVersion() {
	fmt.Printf("🎮 %s\n", AppName)
	fmt.Printf("📦 Version: %s\n", version)
	if gitCommit != "unknown" {
		fmt.Printf("🔄 Git Commit: %s\n", gitCommit)
	}
	if buildTime != "unknown" {
		fmt.Printf("🕒 Build Time: %s\n", buildTime)
	}
	fmt.Printf("🔧 Go Version: %s\n", runtime.Version())
	fmt.Printf("💻 Platform: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("🧮 CPUs: %d\n", runtime.NumCPU())
}

func main() {
	flag.Parse()

	if *showVersion {
		printVersion()
		os.Exit(0)
	}

	// 设置主上下文和信号处理
	ctx, cancel := context.WithCancel(context.Background())

	// 加载配置
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ 加载配置失败: %v\n", err)
		os.Exit(1)
	}

	// 设置日志
	loggerManager, err := logger.NewLoggerManager(ctx, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "初始化日志失败: %v\n", err)
		os.Exit(1)
	}

	mainLogger := loggerManager.GetMainLogger()
	attackLogger := loggerManager.GetAttackLogger()
	performanceLogger := loggerManager.GetPerformanceLogger()

	mainLogger.Info().
		Str("app", AppName).
		Str("version", version).
		Str("config_path", *configPath).
		Msg("启动服务器")

	// 创建限流器
	rateLimiter := limiter.NewRateLimiter(cfg, mainLogger)
	rateLimiter.StartCleanupRoutine()

	// 创建上游同步器
	upstreamSyncer := sync.NewUpstreamSyncer(cfg, mainLogger, ctx)
	go func() {
		if err := upstreamSyncer.Start(); err != nil {
			mainLogger.Error().Err(err).Msg("启动上游同步器失败")
		}
	}()

	// 创建快速协议处理器
	protocolHandler := protocol.NewFastHandler(cfg, mainLogger, upstreamSyncer, rateLimiter, loggerManager.GetHoneypotLogger())

	// 创建网络服务器
	server, err := network.NewServer(cfg, mainLogger, protocolHandler, ctx)
	if err != nil {
		mainLogger.Error().Err(err).Msg("创建网络服务器失败")
		os.Exit(1)
	}
	if server == nil {
		mainLogger.Fatal().Msg("网络服务器创建返回 nil")
	}

	// 启动服务器
	go func() {
		println("启动服务器于 " + cfg.GetAddress())
		if err := server.Start(); err != nil {
			// 检查是否是因为 context 取消导致的正常关闭
			select {
			case <-ctx.Done():
				// 这是正常关闭，不记录错误
				mainLogger.Debug().Msg("服务器因上下文取消而停止")
			default:
				// 这是异常错误
				mainLogger.Error().Err(err).Msg("服务器启动失败")
				cancel()
			}
		}
	}()

	// 等待一小段时间确保服务器启动
	time.Sleep(100 * time.Millisecond)

	// 设置信号处理
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 启动基于 context 的监控服务
	go startPerformanceMonitoring(ctx, performanceLogger, server, rateLimiter)
	go startAttackMonitoring(ctx, attackLogger, rateLimiter)

	mainLogger.Info().
		Str("address", cfg.GetAddress()).
		Msg("服务器启动成功")

	// 等待停止信号或 context 取消
	select {
	case sig := <-sigChan:
		mainLogger.Info().
			Str("signal", sig.String()).
			Msg("收到停止信号")
	case <-ctx.Done():
		mainLogger.Info().Msg("上下文取消")
	}

	// 取消 context，通知所有组件停止（包括 loggerManager）
	cancel()

	// 给所有基于 context 的组件时间来处理取消信号
	time.Sleep(1 * time.Second)

	mainLogger.Info().Msg("服务器已停止")
}

// startPerformanceMonitoring 启动性能监控
func startPerformanceMonitoring(ctx context.Context, perfLogger *logger.PerformanceLogger, server *network.Server, rateLimiter *limiter.RateLimiter) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// 获取内存统计
			var m runtime.MemStats
			runtime.ReadMemStats(&m)

			// 记录内存使用情况
			perfLogger.LogMemoryUsage(
				m.Alloc/1024/1024, // MB
				m.Sys/1024/1024,   // MB
				uint64(m.NumGC),   // GC 次数
			)

			// 获取服务器统计
			serverStats := server.GetStats()
			if activeConns, ok := serverStats["connection_count"].(int64); ok {
				perfLogger.LogConnectionMetrics(
					activeConns,
					activeConns, // 这里简化处理，实际应该记录总连接数
					0,           // 平均响应时间，需要从其他地方获取
				)
			}

			// 获取限流器统计
			limiterStats := rateLimiter.GetStats()
			if globalReqs, ok := limiterStats["global_requests"].(int64); ok {
				if totalReqs, ok := limiterStats["total_requests"].(int64); ok {
					if activeIPs, ok := limiterStats["active_ip_count"].(int); ok {
						if avgReqsPerSec, ok := limiterStats["avg_requests_per_second"].(float64); ok {
							perfLogger.LogRateLimitMetrics(globalReqs, totalReqs, activeIPs, avgReqsPerSec)
						}
					}
				}
			}
		}
	}
}

// startAttackMonitoring 启动攻击监控
func startAttackMonitoring(ctx context.Context, attackLogger *logger.AttackLogger, rateLimiter *limiter.RateLimiter) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// 检查熔断器状态
			if rateLimiter.IsCircuitBreakerTriggered() {
				metrics := rateLimiter.GetStats()
				attackLogger.LogCircuitBreakerTriggered("全局限流触发", metrics)
			}
		}
	}
}
