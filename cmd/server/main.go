package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/tealife/proxy-cs3/internal/common"
	"github.com/tealife/proxy-cs3/internal/server"
)

var (
	port     = flag.Int("port", 1080, "服务监听端口")
	logLevel = flag.String("log", "info", "日志级别: debug, info, warn, error, fatal")
)

func main() {
	flag.Parse()

	// 解析日志级别
	level := parseLogLevel(*logLevel)

	// 创建日志记录器
	logger := common.NewSimpleLogger("SERVER", level)

	// 创建服务器配置
	config := server.Config{
		Port:     *port,
		LogLevel: level,
	}

	// 创建服务器实例
	srv := server.NewServer(config, logger)

	// 启动服务器
	if err := srv.Start(); err != nil {
		logger.Fatalf("启动服务器失败: %v", err)
	}

	// 等待中断信号
	waitForInterrupt(srv, logger)
}

// 解析日志级别
func parseLogLevel(level string) common.LogLevel {
	switch level {
	case "debug":
		return common.DebugLevel
	case "info":
		return common.InfoLevel
	case "warn":
		return common.WarnLevel
	case "error":
		return common.ErrorLevel
	case "fatal":
		return common.FatalLevel
	default:
		return common.InfoLevel
	}
}

// 等待中断信号
func waitForInterrupt(srv *server.Server, logger common.Logger) {
	// 创建通道接收信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// 等待信号
	sig := <-sigCh
	logger.Infof("收到信号: %v，正在关闭服务器...", sig)

	// 优雅关闭
	if err := srv.Stop(); err != nil {
		logger.Errorf("关闭服务器时出错: %v", err)
		os.Exit(1)
	}

	logger.Info("服务器已关闭")
}
