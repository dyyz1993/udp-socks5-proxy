package client

import (
	"github.com/tealife/proxy-cs3/internal/common"
)

// GoSocks5Logger 适配go-socks5库需要的Logger接口
type GoSocks5Logger struct {
	logger common.Logger
}

// NewGoSocks5Logger 创建一个新的SOCKS5日志适配器
func NewGoSocks5Logger(logger common.Logger) *GoSocks5Logger {
	return &GoSocks5Logger{logger: logger}
}

// Printf 实现go-socks5库的Logger接口
func (l *GoSocks5Logger) Printf(format string, args ...interface{}) {
	l.logger.Debugf(format, args...)
}

// Errorf 实现go-socks5库的Logger接口
func (l *GoSocks5Logger) Errorf(format string, args ...interface{}) {
	l.logger.Errorf(format, args...)
}
