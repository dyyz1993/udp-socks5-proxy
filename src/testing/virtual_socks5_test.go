package testing

import (
	"bytes"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/tealife/proxy-cs3/src/tunnel/client"
	tunnelTesting "github.com/tealife/proxy-cs3/src/tunnel/testing"
)

// 实现Logger接口的测试日志记录器
type virtualSocks5TestLogger struct{}

func (l *virtualSocks5TestLogger) Debug(args ...interface{})                 {}
func (l *virtualSocks5TestLogger) Debugf(format string, args ...interface{}) {}
func (l *virtualSocks5TestLogger) Info(args ...interface{})                  {}
func (l *virtualSocks5TestLogger) Infof(format string, args ...interface{})  {}
func (l *virtualSocks5TestLogger) Error(args ...interface{})                 {}
func (l *virtualSocks5TestLogger) Errorf(format string, args ...interface{}) {}

func newVirtualSocks5TestLogger() *virtualSocks5TestLogger {
	return &virtualSocks5TestLogger{}
}

// 测试NewVirtualSocks5Conn函数
func TestNewVirtualSocks5Conn(t *testing.T) {
	mockConn := tunnelTesting.NewMockNetConn()
	logger := newVirtualSocks5TestLogger()
	originalData := []byte{0x05, 0x01, 0x00, 0x01, 0x7f, 0x00, 0x00, 0x01, 0x1f, 0x90} // SOCKS5请求数据

	conn := client.NewVirtualSocks5Conn(mockConn, originalData, logger)
	assert.NotNil(t, conn, "VirtualSocks5Conn不应为nil")
}

// 测试Read方法 - 握手阶段
func TestVirtualSocks5ConnRead_Handshake(t *testing.T) {
	mockConn := tunnelTesting.NewMockNetConn()
	logger := newVirtualSocks5TestLogger()
	originalData := []byte{0x05, 0x01, 0x00, 0x01, 0x7f, 0x00, 0x00, 0x01, 0x1f, 0x90} // SOCKS5请求数据

	conn := client.NewVirtualSocks5Conn(mockConn, originalData, logger)
	assert.NotNil(t, conn, "VirtualSocks5Conn不应为nil")

	// 读取握手数据(前3字节)
	buf := make([]byte, 3)
	n, err := conn.Read(buf)
	assert.NoError(t, err, "读取握手数据应成功")
	assert.Equal(t, 3, n, "应读取3字节握手数据")
	assert.Equal(t, []byte{0x05, 0x01, 0x00}, buf, "握手数据应匹配")

	// 模拟认证响应
	_, err = conn.Write([]byte{0x05, 0x00}) // 写入认证响应
	assert.NoError(t, err, "写入认证响应应成功")

	// 读取连接请求数据
	buf = make([]byte, 10)
	n, err = conn.Read(buf)
	assert.NoError(t, err, "读取连接请求数据应成功")
	assert.Equal(t, 7, n, "应读取剩余的连接请求数据") // 握手后的数据
	assert.Equal(t, originalData[3:10], buf[:n], "连接请求数据应匹配")
}

// 测试Write方法 - 认证响应
func TestVirtualSocks5ConnWrite_AuthResponse(t *testing.T) {
	mockConn := tunnelTesting.NewMockNetConn()
	logger := newVirtualSocks5TestLogger()
	originalData := []byte{0x05, 0x01, 0x00} // 只有握手数据

	conn := client.NewVirtualSocks5Conn(mockConn, originalData, logger)

	// 读取握手数据使状态前进
	buf := make([]byte, 3)
	n, err := conn.Read(buf)
	assert.NoError(t, err, "读取握手数据应成功")
	assert.Equal(t, 3, n, "应读取3字节握手数据")

	// 写入认证响应 - 这应该不会实际写入mockConn，而是内部处理
	n, err = conn.Write([]byte{0x05, 0x00})
	assert.NoError(t, err, "写入认证响应应成功")
	assert.Equal(t, 2, n, "应报告2字节已写入")

	// 验证没有实际写入原始连接
	assert.Empty(t, mockConn.GetWrittenData(), "不应该向原始连接写入认证响应")
}

// 测试Write方法 - 连接响应
func TestVirtualSocks5ConnWrite_ConnectResponse(t *testing.T) {
	mockConn := tunnelTesting.NewMockNetConn()
	logger := newVirtualSocks5TestLogger()
	originalData := []byte{0x05, 0x01, 0x00, 0x01, 0x7f, 0x00, 0x00, 0x01, 0x1f, 0x90} // SOCKS5请求数据

	conn := client.NewVirtualSocks5Conn(mockConn, originalData, logger)

	// 读取握手数据
	buf := make([]byte, 3)
	conn.Read(buf)

	// 写入认证响应
	conn.Write([]byte{0x05, 0x00})

	// 读取连接请求
	buf = make([]byte, 10)
	conn.Read(buf)

	// 构造一个连接响应
	connectResp := []byte{0x05, 0x00, 0x00, 0x01, 0x7f, 0x00, 0x00, 0x01, 0x1f, 0x90}
	n, err := conn.Write(connectResp)
	assert.NoError(t, err, "写入连接响应应成功")
	assert.Equal(t, len(connectResp), n, "应报告全部字节已写入")

	// 验证实际写入原始连接
	assert.Equal(t, connectResp, mockConn.GetWrittenData(), "应该向原始连接写入连接响应")
}

// 测试Write方法 - 应用数据
func TestVirtualSocks5ConnWrite_AppData(t *testing.T) {
	mockConn := tunnelTesting.NewMockNetConn()
	logger := newVirtualSocks5TestLogger()
	originalData := []byte{0x05, 0x01, 0x00, 0x01, 0x7f, 0x00, 0x00, 0x01, 0x1f, 0x90} // SOCKS5请求数据

	conn := client.NewVirtualSocks5Conn(mockConn, originalData, logger)

	// 读取握手数据
	buf := make([]byte, 3)
	conn.Read(buf)

	// 写入认证响应
	conn.Write([]byte{0x05, 0x00})

	// 读取连接请求
	buf = make([]byte, 10)
	conn.Read(buf)

	// 写入连接响应
	connectResp := []byte{0x05, 0x00, 0x00, 0x01, 0x7f, 0x00, 0x00, 0x01, 0x1f, 0x90}
	conn.Write(connectResp)
	mockConn.ClearWrittenData() // 清除之前写入的数据

	// 写入应用数据
	appData := []byte("HTTP/1.1 200 OK\r\nContent-Length: 5\r\n\r\nHello")
	n, err := conn.Write(appData)
	assert.NoError(t, err, "写入应用数据应成功")
	assert.Equal(t, len(appData), n, "应报告全部字节已写入")

	// 验证实际写入原始连接
	assert.Equal(t, appData, mockConn.GetWrittenData(), "应该向原始连接写入应用数据")
}

// 测试Write方法 - 大数据包分块写入
func TestVirtualSocks5ConnWrite_LargeData(t *testing.T) {
	mockConn := tunnelTesting.NewMockNetConn()
	logger := newVirtualSocks5TestLogger()
	originalData := []byte{0x05, 0x01, 0x00, 0x01, 0x7f, 0x00, 0x00, 0x01, 0x1f, 0x90} // SOCKS5请求数据

	conn := client.NewVirtualSocks5Conn(mockConn, originalData, logger)

	// 读取握手数据
	buf := make([]byte, 3)
	conn.Read(buf)

	// 写入认证响应
	conn.Write([]byte{0x05, 0x00})

	// 读取连接请求
	buf = make([]byte, 10)
	conn.Read(buf)

	// 写入连接响应
	connectResp := []byte{0x05, 0x00, 0x00, 0x01, 0x7f, 0x00, 0x00, 0x01, 0x1f, 0x90}
	conn.Write(connectResp)
	mockConn.ClearWrittenData() // 清除之前写入的数据

	// 创建大数据包(10KB)
	largeData := bytes.Repeat([]byte("A"), 10000)
	n, err := conn.Write(largeData)
	assert.NoError(t, err, "写入大数据包应成功")
	assert.Equal(t, len(largeData), n, "应报告全部字节已写入")

	// 验证实际写入原始连接的数据大小
	assert.Equal(t, len(largeData), len(mockConn.GetWrittenData()), "应该向原始连接写入全部数据")
}

// 测试Read方法 - 应用数据
func TestVirtualSocks5ConnRead_AppData(t *testing.T) {
	mockConn := tunnelTesting.NewMockNetConn()
	logger := newVirtualSocks5TestLogger()
	originalData := []byte{0x05, 0x01, 0x00, 0x01, 0x7f, 0x00, 0x00, 0x01, 0x1f, 0x90} // SOCKS5请求数据

	conn := client.NewVirtualSocks5Conn(mockConn, originalData, logger)

	// 读取握手数据
	buf := make([]byte, 3)
	conn.Read(buf)

	// 写入认证响应
	conn.Write([]byte{0x05, 0x00})

	// 读取连接请求
	buf = make([]byte, 10)
	conn.Read(buf)

	// 预先添加一些模拟数据供后续Read读取
	appData := []byte("HTTP/1.1 200 OK\r\nContent-Length: 5\r\n\r\nHello")
	mockConn.AddReadData(appData)

	// 读取应用数据
	buf = make([]byte, 1024)
	n, err := conn.Read(buf)
	assert.NoError(t, err, "读取应用数据应成功")
	assert.Equal(t, len(appData), n, "应读取全部应用数据")
	assert.Equal(t, appData, buf[:n], "应用数据应匹配")
}

// 测试认证超时
func TestVirtualSocks5ConnAuthTimeout(t *testing.T) {
	mockConn := tunnelTesting.NewMockNetConn()
	logger := newVirtualSocks5TestLogger()
	originalData := []byte{0x05, 0x01, 0x00, 0x01, 0x7f, 0x00, 0x00, 0x01, 0x1f, 0x90} // SOCKS5请求数据

	// 创建一个短超时时间的连接
	conn := client.NewVirtualSocks5Conn(mockConn, originalData, logger)
	conn.SetAuthTimeout(100 * time.Millisecond) // 使用100ms超时

	// 读取握手数据
	buf := make([]byte, 3)
	conn.Read(buf)

	// 不写入认证响应，等待超时
	time.Sleep(200 * time.Millisecond)

	// 尝试读取连接请求，应该会成功，因为超时后会模拟认证成功
	buf = make([]byte, 10)
	n, err := conn.Read(buf)
	assert.NoError(t, err, "超时后读取连接请求应成功")
	assert.Equal(t, 7, n, "应读取剩余的连接请求数据") // 握手后的数据
}

// 测试Close方法
func TestVirtualSocks5ConnClose(t *testing.T) {
	mockConn := tunnelTesting.NewMockNetConn()
	logger := newVirtualSocks5TestLogger()
	originalData := []byte{0x05, 0x01, 0x00, 0x01, 0x7f, 0x00, 0x00, 0x01, 0x1f, 0x90} // SOCKS5请求数据

	conn := client.NewVirtualSocks5Conn(mockConn, originalData, logger)

	// 关闭连接
	err := conn.Close()
	assert.NoError(t, err, "关闭连接应成功")
	assert.True(t, mockConn.IsClosed(), "原始连接应该被关闭")

	// 重复关闭
	err = conn.Close()
	assert.NoError(t, err, "重复关闭连接应成功")
}

// 测试已关闭连接的读取
func TestVirtualSocks5ConnReadClosed(t *testing.T) {
	mockConn := tunnelTesting.NewMockNetConn()
	logger := newVirtualSocks5TestLogger()
	originalData := []byte{0x05, 0x01, 0x00} // 只有握手数据

	conn := client.NewVirtualSocks5Conn(mockConn, originalData, logger)

	// 关闭连接
	conn.Close()

	// 尝试读取
	buf := make([]byte, 3)
	n, err := conn.Read(buf)
	assert.Equal(t, 0, n, "从已关闭连接读取应返回0字节")
	assert.Equal(t, io.EOF, err, "从已关闭连接读取应返回EOF")
}

// 测试从缓冲区读取数据
func TestVirtualSocks5ConnReadFromBuffer(t *testing.T) {
	mockConn := tunnelTesting.NewMockNetConn()
	logger := newVirtualSocks5TestLogger()
	originalData := []byte{0x05, 0x01, 0x00, 0x01, 0x7f, 0x00, 0x00, 0x01, 0x1f, 0x90} // SOCKS5请求数据

	conn := client.NewVirtualSocks5Conn(mockConn, originalData, logger)

	// 直接向读缓冲区添加数据
	conn.GetReadBuffer().Write([]byte("test data"))

	// 从缓冲区读取数据
	buf := make([]byte, 100)
	n, err := conn.Read(buf)
	assert.NoError(t, err, "从缓冲区读取数据应成功")
	assert.Equal(t, 9, n, "应读取全部缓冲区数据")
	assert.Equal(t, []byte("test data"), buf[:n], "缓冲区数据应匹配")
}

// 测试写入空数据
func TestVirtualSocks5ConnWriteEmptyData(t *testing.T) {
	mockConn := tunnelTesting.NewMockNetConn()
	logger := newVirtualSocks5TestLogger()
	originalData := []byte{0x05, 0x01, 0x00}

	conn := client.NewVirtualSocks5Conn(mockConn, originalData, logger)

	// 写入空数据
	n, err := conn.Write([]byte{})
	assert.NoError(t, err, "写入空数据应成功")
	assert.Equal(t, 0, n, "应报告0字节已写入")
	assert.Empty(t, mockConn.GetWrittenData(), "不应该向原始连接写入数据")
}

// 测试GetDataType函数（通过观察Write行为）
func TestVirtualSocks5ConnGetDataType(t *testing.T) {
	mockConn := tunnelTesting.NewMockNetConn()
	logger := newVirtualSocks5TestLogger()
	originalData := []byte{0x05, 0x01, 0x00}

	conn := client.NewVirtualSocks5Conn(mockConn, originalData, logger)

	// 测试空数据
	n, err := conn.Write([]byte{})
	assert.NoError(t, err, "写入空数据应成功")
	assert.Equal(t, 0, n, "应报告0字节已写入")

	// 测试AUTH_RESP数据
	authResp := []byte{0x05, 0x00}
	n, err = conn.Write(authResp)
	assert.NoError(t, err, "写入认证响应应成功")
	assert.Equal(t, len(authResp), n, "应报告正确的字节数已写入")

	// 测试错误格式的CONNECT_RESP数据（不匹配CONNECT_RESP格式，将被视为APP_DATA）
	wrongConnectResp := []byte{0x05, 0x00, 0x01} // 不符合CONNECT_RESP格式
	n, err = conn.Write(wrongConnectResp)
	assert.NoError(t, err, "写入应用数据应成功")
	assert.Equal(t, len(wrongConnectResp), n, "应报告正确的字节数已写入")
}

// 测试当buffer容量小于读缓冲区长度时的读取
func TestVirtualSocks5ConnReadSmallBuffer(t *testing.T) {
	mockConn := tunnelTesting.NewMockNetConn()
	logger := newVirtualSocks5TestLogger()
	originalData := []byte{0x05, 0x01, 0x00, 0x01, 0x7f, 0x00, 0x00, 0x01, 0x1f, 0x90}

	conn := client.NewVirtualSocks5Conn(mockConn, originalData, logger)

	// 直接向读缓冲区添加数据
	testData := []byte("test data with longer content")
	conn.GetReadBuffer().Write(testData)

	// 使用小容量buffer读取
	buf := make([]byte, 4) // 只能一次读取4字节
	n, err := conn.Read(buf)
	assert.NoError(t, err, "从缓冲区读取数据应成功")
	assert.Equal(t, 4, n, "应读取buffer容量大小的数据")
	assert.Equal(t, []byte("test"), buf[:n], "读取的数据应匹配")

	// 继续读取剩余数据
	n, err = conn.Read(buf)
	assert.NoError(t, err, "从缓冲区读取数据应成功")
	assert.Equal(t, 4, n, "应读取buffer容量大小的数据")
	assert.Equal(t, []byte(" dat"), buf[:n], "读取的数据应匹配")
}

// 测试读取小容量数据缓冲区时的握手
func TestVirtualSocks5ConnReadHandshakeSmallBuffer(t *testing.T) {
	mockConn := tunnelTesting.NewMockNetConn()
	logger := newVirtualSocks5TestLogger()
	originalData := []byte{0x05, 0x01, 0x00, 0x01, 0x7f, 0x00, 0x00, 0x01, 0x1f, 0x90}

	conn := client.NewVirtualSocks5Conn(mockConn, originalData, logger)

	// 使用只能容纳1字节的buffer读取握手数据
	buf := make([]byte, 1)

	// 第一次读取，只读取1字节
	n, err := conn.Read(buf)
	assert.NoError(t, err, "读取握手数据应成功")
	assert.Equal(t, 1, n, "应读取1字节握手数据")
	assert.Equal(t, byte(0x05), buf[0], "第一个字节应匹配")

	// 第二次读取，读取下一个字节
	n, err = conn.Read(buf)
	assert.NoError(t, err, "读取握手数据应成功")
	assert.Equal(t, 1, n, "应读取1字节握手数据")
	assert.Equal(t, byte(0x01), buf[0], "第二个字节应匹配")

	// 第三次读取，读取最后一个握手字节
	n, err = conn.Read(buf)
	assert.NoError(t, err, "读取握手数据应成功")
	assert.Equal(t, 1, n, "应读取1字节握手数据")
	assert.Equal(t, byte(0x00), buf[0], "第三个字节应匹配")
}

// 测试写入错误情况
func TestVirtualSocks5ConnWriteError(t *testing.T) {
	// 创建一个会在写入时返回错误的mock连接
	mockConn := tunnelTesting.NewErrorMockNetConn("写入错误测试")
	logger := newVirtualSocks5TestLogger()
	originalData := []byte{0x05, 0x01, 0x00, 0x01, 0x7f, 0x00, 0x00, 0x01, 0x1f, 0x90}

	conn := client.NewVirtualSocks5Conn(mockConn, originalData, logger)

	// 读取握手数据
	buf := make([]byte, 3)
	conn.Read(buf)

	// 写入认证响应
	conn.Write([]byte{0x05, 0x00})

	// 读取连接请求
	buf = make([]byte, 10)
	conn.Read(buf)

	// 尝试写入连接响应，但会失败
	connectResp := []byte{0x05, 0x00, 0x00, 0x01, 0x7f, 0x00, 0x00, 0x01, 0x1f, 0x90}
	_, err := conn.Write(connectResp)
	assert.Error(t, err, "写入应该失败并返回错误")
	assert.Contains(t, err.Error(), "写入错误测试", "错误消息应包含预期内容")

	// 尝试写入应用数据，也会失败
	appData := []byte("test data")
	_, err = conn.Write(appData)
	assert.Error(t, err, "写入应该失败并返回错误")
}
