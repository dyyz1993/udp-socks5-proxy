package common

import (
	"bytes"
	"log"
	"strings"
	"testing"
)

func TestNewSimpleLogger(t *testing.T) {
	logger := NewSimpleLogger("TEST", DebugLevel)

	if logger == nil {
		t.Fatal("NewSimpleLogger返回nil")
	}

	if logger.level != DebugLevel {
		t.Errorf("日志级别设置错误，期望: %v, 实际: %v", DebugLevel, logger.level)
	}

	if logger.tag != "TEST" {
		t.Errorf("日志标签设置错误，期望: TEST, 实际: %s", logger.tag)
	}

	if logger.stdLog == nil || logger.errorLog == nil {
		t.Error("日志记录器未初始化")
	}
}

func TestSimpleLoggerFormatMsg(t *testing.T) {
	logger := NewSimpleLogger("TEST", DebugLevel)

	msg := logger.formatMsg("INFO", "测试消息")

	// 检查消息格式
	if !strings.Contains(msg, "INFO") {
		t.Error("格式化消息应包含级别")
	}

	if !strings.Contains(msg, "TEST") {
		t.Error("格式化消息应包含标签")
	}

	if !strings.Contains(msg, "测试消息") {
		t.Error("格式化消息应包含原消息")
	}
}

func TestSimpleLoggerLevelFiltering(t *testing.T) {
	// 捕获标准输出
	var stdBuf, errBuf bytes.Buffer

	logger := &SimpleLogger{
		level:    InfoLevel,
		tag:      "TEST",
		stdLog:   log.New(&stdBuf, "", 0),
		errorLog: log.New(&errBuf, "", 0),
	}

	// 调试级别消息应被过滤
	logger.Debug("调试消息")
	if stdBuf.Len() > 0 {
		t.Error("调试消息不应输出")
	}

	// 信息级别消息应输出
	logger.Info("信息消息")
	if stdBuf.Len() == 0 {
		t.Error("信息消息应输出")
	}
	stdBuf.Reset()

	// 警告级别消息应输出
	logger.Warn("警告消息")
	if stdBuf.Len() == 0 {
		t.Error("警告消息应输出")
	}
	stdBuf.Reset()

	// 错误级别消息应输出到错误输出
	logger.Error("错误消息")
	if errBuf.Len() == 0 {
		t.Error("错误消息应输出")
	}
	errBuf.Reset()
}

func TestSimpleLoggerFormatOutput(t *testing.T) {
	// 捕获标准输出
	var stdBuf bytes.Buffer

	logger := &SimpleLogger{
		level:    DebugLevel,
		tag:      "TEST",
		stdLog:   log.New(&stdBuf, "", 0),
		errorLog: log.New(&stdBuf, "", 0),
	}

	// 测试格式化输出
	logger.Debugf("这是一个%s", "调试消息")
	if !strings.Contains(stdBuf.String(), "这是一个调试消息") {
		t.Error("格式化输出不正确")
	}
	stdBuf.Reset()

	logger.Infof("数字: %d", 123)
	if !strings.Contains(stdBuf.String(), "数字: 123") {
		t.Error("格式化输出不正确")
	}
}

// TestNewSimpleLoggerWithWriter 测试带自定义输出的日志记录器
func TestNewSimpleLoggerWithWriter(t *testing.T) {
	// 自定义输出缓冲区
	var stdBuf, errBuf bytes.Buffer

	// 创建日志记录器
	logger := NewSimpleLoggerWithWriter("CUSTOM", DebugLevel, &stdBuf, &errBuf)

	if logger == nil {
		t.Fatal("NewSimpleLoggerWithWriter返回nil")
	}

	if logger.level != DebugLevel {
		t.Errorf("日志级别设置错误，期望: %v, 实际: %v", DebugLevel, logger.level)
	}

	if logger.tag != "CUSTOM" {
		t.Errorf("日志标签设置错误，期望: CUSTOM, 实际: %s", logger.tag)
	}

	// 测试输出到自定义写入器
	logger.Debug("测试自定义输出")
	if !strings.Contains(stdBuf.String(), "测试自定义输出") {
		t.Error("日志未输出到自定义写入器")
	}

	logger.Error("测试错误输出")
	if !strings.Contains(errBuf.String(), "测试错误输出") {
		t.Error("错误日志未输出到自定义错误写入器")
	}
}

// TestSimpleLoggerErrorAndWarnf 测试Warnf和Errorf方法
func TestSimpleLoggerErrorAndWarnf(t *testing.T) {
	// 捕获标准输出和错误输出
	var stdBuf, errBuf bytes.Buffer

	logger := &SimpleLogger{
		level:    DebugLevel,
		tag:      "TEST",
		stdLog:   log.New(&stdBuf, "", 0),
		errorLog: log.New(&errBuf, "", 0),
	}

	// 测试Warnf方法
	logger.Warnf("警告：%s，数字：%d", "测试", 123)
	output := stdBuf.String()
	if !strings.Contains(output, "警告：测试，数字：123") {
		t.Errorf("Warnf输出不正确: %s", output)
	}
	stdBuf.Reset()

	// 测试Errorf方法
	logger.Errorf("错误：%s，代码：%d", "测试失败", 500)
	output = errBuf.String()
	if !strings.Contains(output, "错误：测试失败，代码：500") {
		t.Errorf("Errorf输出不正确: %s", output)
	}
}
