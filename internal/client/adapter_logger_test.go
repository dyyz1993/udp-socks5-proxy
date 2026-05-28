package client

import (
	"testing"

	"github.com/tealife/proxy-cs3/internal/common"
)

// TestGoSocks5Logger 测试日志适配器
func TestGoSocks5Logger(t *testing.T) {
	// 创建测试用的Logger实例
	logger := common.NewSimpleLogger("TEST", common.DebugLevel)

	// 创建适配器
	adapter := NewGoSocks5Logger(logger)

	// 测试Printf方法
	adapter.Printf("测试日志消息: %s", "printf")

	// 测试Errorf方法
	adapter.Errorf("测试错误消息: %s", "errorf")

	// 这个测试只是确保方法调用不会崩溃，
	// 因为我们无法直接验证日志输出内容
}
