package common

import (
	"fmt"
	"io"
	"log"
	"os"
	"time"
)

// Logger 定义日志接口
type Logger interface {
	Debug(args ...interface{})
	Debugf(format string, args ...interface{})
	Info(args ...interface{})
	Infof(format string, args ...interface{})
	Warn(args ...interface{})
	Warnf(format string, args ...interface{})
	Error(args ...interface{})
	Errorf(format string, args ...interface{})
	Fatal(args ...interface{})
	Fatalf(format string, args ...interface{})
}

// LogLevel 日志级别
type LogLevel int

const (
	// DebugLevel 调试级别
	DebugLevel LogLevel = iota
	// InfoLevel 信息级别
	InfoLevel
	// WarnLevel 警告级别
	WarnLevel
	// ErrorLevel 错误级别
	ErrorLevel
	// FatalLevel 致命错误级别
	FatalLevel
)

// SimpleLogger 简单日志实现
type SimpleLogger struct {
	level    LogLevel
	tag      string
	stdLog   *log.Logger
	errorLog *log.Logger
}

// NewSimpleLogger 创建一个新的简单日志记录器
func NewSimpleLogger(tag string, level LogLevel) *SimpleLogger {
	return &SimpleLogger{
		level:    level,
		tag:      tag,
		stdLog:   log.New(os.Stdout, "", 0),
		errorLog: log.New(os.Stderr, "", 0),
	}
}

// NewSimpleLoggerWithWriter 使用自定义输出创建一个新的简单日志器
func NewSimpleLoggerWithWriter(tag string, level LogLevel, out io.Writer, errOut io.Writer) *SimpleLogger {
	return &SimpleLogger{
		level:    level,
		tag:      tag,
		stdLog:   log.New(out, "", 0),
		errorLog: log.New(errOut, "", 0),
	}
}

// formatMsg 格式化日志消息
func (l *SimpleLogger) formatMsg(level string, msg string) string {
	return fmt.Sprintf("[%s] [%s] [%s] %s", time.Now().Format("2006-01-02 15:04:05.000"), level, l.tag, msg)
}

// Debug 输出调试日志
func (l *SimpleLogger) Debug(args ...interface{}) {
	if l.level <= DebugLevel {
		l.stdLog.Println(l.formatMsg("DEBUG", fmt.Sprint(args...)))
	}
}

// Debugf 输出格式化调试日志
func (l *SimpleLogger) Debugf(format string, args ...interface{}) {
	if l.level <= DebugLevel {
		l.stdLog.Println(l.formatMsg("DEBUG", fmt.Sprintf(format, args...)))
	}
}

// Info 输出信息日志
func (l *SimpleLogger) Info(args ...interface{}) {
	if l.level <= InfoLevel {
		l.stdLog.Println(l.formatMsg("INFO", fmt.Sprint(args...)))
	}
}

// Infof 输出格式化信息日志
func (l *SimpleLogger) Infof(format string, args ...interface{}) {
	if l.level <= InfoLevel {
		l.stdLog.Println(l.formatMsg("INFO", fmt.Sprintf(format, args...)))
	}
}

// Warn 输出警告日志
func (l *SimpleLogger) Warn(args ...interface{}) {
	if l.level <= WarnLevel {
		l.stdLog.Println(l.formatMsg("WARN", fmt.Sprint(args...)))
	}
}

// Warnf 输出格式化警告日志
func (l *SimpleLogger) Warnf(format string, args ...interface{}) {
	if l.level <= WarnLevel {
		l.stdLog.Println(l.formatMsg("WARN", fmt.Sprintf(format, args...)))
	}
}

// Error 输出错误日志
func (l *SimpleLogger) Error(args ...interface{}) {
	if l.level <= ErrorLevel {
		l.errorLog.Println(l.formatMsg("ERROR", fmt.Sprint(args...)))
	}
}

// Errorf 输出格式化错误日志
func (l *SimpleLogger) Errorf(format string, args ...interface{}) {
	if l.level <= ErrorLevel {
		l.errorLog.Println(l.formatMsg("ERROR", fmt.Sprintf(format, args...)))
	}
}

// Fatal 输出致命错误日志并退出
func (l *SimpleLogger) Fatal(args ...interface{}) {
	if l.level <= FatalLevel {
		msg := l.formatMsg("FATAL", fmt.Sprint(args...))
		l.errorLog.Println(msg)
		os.Exit(1)
	}
}

// Fatalf 输出格式化致命错误日志并退出
func (l *SimpleLogger) Fatalf(format string, args ...interface{}) {
	if l.level <= FatalLevel {
		msg := l.formatMsg("FATAL", fmt.Sprintf(format, args...))
		l.errorLog.Println(msg)
		os.Exit(1)
	}
}
