package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tealife/proxy-cs3/internal/client"
	"github.com/tealife/proxy-cs3/internal/common"
)

var (
	localPort     = flag.Int("local", 1080, "本地SOCKS5服务端口")
	serverAddr    = flag.String("server", "127.0.0.1:1081", "服务器地址")
	directDomains = flag.String("direct", "", "直连域名列表，用逗号分隔")
	defaultDirect = flag.Bool("default-direct", false, "默认直连策略")
	timeout       = flag.Duration("timeout", 5*time.Minute, "连接超时时间")
	logLevel      = flag.String("log", "info", "日志级别: debug, info, warn, error, fatal")
)

func main() {
	flag.Parse()

	// 解析日志级别
	level := parseLogLevel(*logLevel)

	// 创建日志记录器
	logger := common.NewSimpleLogger("CLIENT", level)

	// 解析直连域名
	var domains []string
	if *directDomains != "" {
		domains = splitDomains(*directDomains)
	}

	// 创建客户端配置
	config := client.Config{
		LocalPort:     *localPort,
		ServerAddr:    *serverAddr,
		DirectDomains: domains,
		DefaultDirect: *defaultDirect,
		Timeout:       *timeout,
		LogLevel:      level,
	}

	// 创建客户端实例
	cli := client.NewClient(config, logger)

	// 启动客户端
	if err := cli.Start(); err != nil {
		logger.Fatalf("启动客户端失败: %v", err)
	}

	// 等待中断信号
	waitForInterrupt(cli, logger)
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

// 分割域名列表
func splitDomains(domains string) []string {
	var result []string

	// TODO: 使用更健壮的分割方法，支持引号内的逗号
	result = append(result, domains)

	return result
}

// 等待中断信号
func waitForInterrupt(cli *client.Client, logger common.Logger) {
	// 创建通道接收信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// 等待信号
	sig := <-sigCh
	logger.Infof("收到信号: %v，正在关闭客户端...", sig)

	// 优雅关闭
	if err := cli.Stop(); err != nil {
		logger.Errorf("关闭客户端时出错: %v", err)
		os.Exit(1)
	}

	logger.Info("客户端已关闭")
}
