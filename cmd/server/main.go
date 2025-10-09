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
	version   = "gomc-v1" // GoMC版本
	buildTime = "unknown" // 通过 -ldflags 注入
	gitCommit = "unknown" // 通过 -ldflags 注入
)

var (
	configPath  = flag.String("config", "config/config.yml", "配置文件路径")
	showVersion = flag.Bool("version", false, "显示版本信息")
)

const (
	AppName = "FakeMCServer (GoMC Edition)"
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

	// 显示版本信息
	if *showVersion {
		printVersion()
		return
	}

	// 加载配置
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Printf("❌ 加载配置失败: %v\n", err)
		os.Exit(1)
	}

	// 初始化日志
	mainLogger, err := logger.Setup(cfg)
	if err != nil {
		fmt.Printf("❌ 初始化日志失败: %v\n", err)
		os.Exit(1)
	}
	// 使用 fmt 直接输出启动信息（不受日志级别限制）
	fmt.Printf("🚀 启动 FakeMCServer (GoMC Edition)\n")
	fmt.Printf("📦 版本: %s\n", version)
	fmt.Printf("📝 配置: %s\n", *configPath)
	fmt.Printf("📊 日志级别: %s\n", cfg.Logging.Level)
	fmt.Println()

	// 创建上下文
	fmt.Println("⏳ 创建上下文...")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 初始化蜜罐日志
	fmt.Println("⏳ 初始化蜜罐日志...")
	honeypotLogger, err := logger.NewHoneypotLogger(&cfg.HoneypotLogging)
	if err != nil {
		fmt.Printf("❌ 初始化蜜罐日志失败: %v\n", err)
		os.Exit(1)
	}
	defer honeypotLogger.Close()

	// 初始化限流器
	fmt.Println("⏳ 初始化限流器...")
	rateLimiter := limiter.NewRateLimiter(cfg, mainLogger)

	// 初始化上游同步器
	fmt.Println("⏳ 初始化上游同步器...")
	var upstreamSyncer *sync.UpstreamSyncer
	if cfg.Upstream.Enabled {
		upstreamSyncer = sync.NewUpstreamSyncer(cfg, mainLogger, ctx)
		fmt.Println("⏳ 启动上游同步器...")
		if err := upstreamSyncer.Start(); err != nil {
			fmt.Printf("❌ 启动上游同步器失败: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✅ 上游同步器已启动: %s\n", cfg.Upstream.Address)
	}

	// 创建GoMC处理器
	fmt.Println("⏳ 创建GoMC处理器...")
	handler := protocol.NewGoMCHandler(
		cfg,
		mainLogger,
		upstreamSyncer,
		honeypotLogger,
		rateLimiter,
	)

	// 创建网络服务器
	fmt.Println("⏳ 创建网络服务器...")
	server, err := network.NewServer(cfg, mainLogger, handler, ctx)
	if err != nil {
		fmt.Printf("❌ 创建网络服务器失败: %v\n", err)
		os.Exit(1)
	}

	// 启动服务器
	go func() {
		fmt.Printf("🌐 网络服务器启动中...\n")
		fmt.Printf("   监听地址: %s\n", cfg.GetAddress())

		if err := server.Start(); err != nil {
			mainLogger.Error().Err(err).Msg("网络服务器错误")
			cancel()
		}
	}()

	// 等待启动完成
	time.Sleep(500 * time.Millisecond)

	// 显示启动信息
	fmt.Println()
	fmt.Println("✨ FakeMCServer (GoMC Edition) 启动完成")
	fmt.Println("📊 服务器状态:")
	fmt.Printf("   - 监听地址: %s\n", cfg.GetAddress())
	fmt.Printf("   - 最大连接数: %d\n", cfg.Server.MaxConnections)
	fmt.Printf("   - IP限流: %d/s\n", cfg.RateLimit.IPLimit)
	fmt.Printf("   - 全局限流: %d/s\n", cfg.RateLimit.GlobalLimit)
	if cfg.Upstream.Enabled {
		fmt.Printf("   - 上游服务器: %s\n", cfg.Upstream.Address)
	}
	fmt.Println("🎯 使用 Ctrl+C 停止服务器")
	fmt.Println()

	// 等待信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigChan:
		fmt.Printf("\n📡 收到停止信号: %s\n", sig.String())
	case <-ctx.Done():
		fmt.Println("\n📡 上下文已取消")
	}

	// 优雅关闭
	fmt.Println("🛑 正在停止服务器...")

	// 取消上下文
	cancel()

	// 等待清理
	time.Sleep(1 * time.Second)

	// 显示统计信息
	stats := server.GetStats()
	fmt.Println("📈 服务器统计:")
	fmt.Printf("   - 当前连接数: %v\n", stats["connection_count"])

	fmt.Println("👋 FakeMCServer (GoMC Edition) 已停止")
}
