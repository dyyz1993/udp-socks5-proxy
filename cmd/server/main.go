package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/tealife/proxy-cs3/internal/common"
	"github.com/tealife/proxy-cs3/internal/server"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

// run 是 main 的可测试版本，返回退出码
func run(args []string) int {
	fs := flag.NewFlagSet("server", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	port := fs.Int("port", 1080, "服务监听端口")
	logLevel := fs.String("log", "info", "日志级别: debug, info, warn, error, fatal")

	if err := fs.Parse(args); err != nil {
		return 2
	}

	level := parseLogLevel(*logLevel)
	logger := common.NewSimpleLogger("SERVER", level)

	config := server.Config{
		Port:     *port,
		LogLevel: level,
	}

	srv := server.NewServer(config, logger)

	if err := srv.Start(); err != nil {
		logger.Errorf("启动服务器失败: %v", err)
		return 1
	}

	return waitForInterrupt(srv, logger)
}

// parseLogLevel 解析日志级别
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

// waitForInterrupt 等待中断信号并优雅关闭，返回退出码
func waitForInterrupt(srv *server.Server, logger common.Logger) int {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigCh
	logger.Infof("收到信号: %v，正在关闭服务器...", sig)

	if err := srv.Stop(); err != nil {
		logger.Errorf("关闭服务器时出错: %v", err)
		return 1
	}

	logger.Info("服务器已关闭")
	return 0
}
