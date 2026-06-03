package main

import (
	"flag"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tealife/proxy-cs3/internal/client"
	"github.com/tealife/proxy-cs3/internal/common"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

// run 是 main 的可测试版本，返回退出码
func run(args []string) int {
	fs := flag.NewFlagSet("client", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	localPort := fs.Int("local", 1080, "本地SOCKS5服务端口")
	serverAddr := fs.String("server", "127.0.0.1:1081", "服务器地址")
	directDomains := fs.String("direct", "", "直连域名列表，用逗号分隔")
	defaultDirect := fs.Bool("default-direct", false, "默认直连策略")
	timeout := fs.Duration("timeout", 5*time.Minute, "连接超时时间")
	logLevel := fs.String("log", "info", "日志级别: debug, info, warn, error, fatal")

	if err := fs.Parse(args); err != nil {
		return 2
	}

	level := parseLogLevel(*logLevel)
	logger := common.NewSimpleLogger("CLIENT", level)

	var domains []string
	if *directDomains != "" {
		domains = splitDomains(*directDomains)
	}

	config := client.Config{
		LocalPort:     *localPort,
		ServerAddr:    *serverAddr,
		DirectDomains: domains,
		DefaultDirect: *defaultDirect,
		Timeout:       *timeout,
		LogLevel:      level,
	}

	cli := client.NewClient(config, logger)

	if err := cli.Start(); err != nil {
		logger.Errorf("启动客户端失败: %v", err)
		return 1
	}

	return waitForInterrupt(cli, logger)
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

// splitDomains 分割域名列表
func splitDomains(domains string) []string {
	var result []string
	result = append(result, domains)
	return result
}

// waitForInterrupt 等待中断信号并优雅关闭，返回退出码
func waitForInterrupt(cli *client.Client, logger common.Logger) int {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigCh
	logger.Infof("收到信号: %v，正在关闭客户端...", sig)

	if err := cli.Stop(); err != nil {
		logger.Errorf("关闭客户端时出错: %v", err)
		return 1
	}

	logger.Info("客户端已关闭")
	return 0
}
