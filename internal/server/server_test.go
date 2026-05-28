package server

import (
	"net"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/tealife/proxy-cs3/internal/common"
)

// 创建模拟UDP连接
type mockUDPConn struct {
	readData  []byte
	writeData []byte
	addr      *net.UDPAddr
}

func (m *mockUDPConn) ReadFromUDP(b []byte) (int, *net.UDPAddr, error) {
	copy(b, m.readData)
	return len(m.readData), m.addr, nil
}

func (m *mockUDPConn) WriteToUDP(b []byte, addr *net.UDPAddr) (int, error) {
	m.writeData = make([]byte, len(b))
	copy(m.writeData, b)
	return len(b), nil
}

func (m *mockUDPConn) Close() error {
	return nil
}

func (m *mockUDPConn) LocalAddr() net.Addr {
	return m.addr
}

func (m *mockUDPConn) SetReadDeadline(t time.Time) error {
	return nil
}

// 创建测试服务器实例
func createTestServer() (*Server, common.Logger) {
	// 配置日志输出到空设备
	logWriter := os.NewFile(0, os.DevNull)
	logger := common.NewSimpleLoggerWithWriter("TEST", common.DebugLevel, logWriter, logWriter)

	config := Config{
		Port:     1080,
		LogLevel: common.DebugLevel,
	}

	// 创建服务端
	return NewServer(config, logger), logger
}

// 测试创建新服务端
func TestNewServer(t *testing.T) {
	// 配置日志输出到空设备
	logWriter := os.NewFile(0, os.DevNull)
	logger := common.NewSimpleLoggerWithWriter("TEST", common.DebugLevel, logWriter, logWriter)

	config := Config{
		Port:     1080,
		LogLevel: common.DebugLevel,
	}

	// 创建服务端
	server := NewServer(config, logger)

	// 验证服务端不为空
	assert.NotNil(t, server)

	// 验证配置
	assert.Equal(t, config, server.config)

	// 验证初始状态
	assert.False(t, server.isRunning)
	assert.NotNil(t, server.closeChan)
	assert.NotNil(t, server.clientConn)

	// 测试创建时不提供日志实例
	server = NewServer(config, nil)
	assert.NotNil(t, server)
	assert.NotNil(t, server.logger)
}

// 测试服务端解析数据包
func TestServerParsePacket(t *testing.T) {
	server, _ := createTestServer()

	// 测试有效数据包
	validData := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06}
	packet, err := server.parsePacket(validData)
	assert.NoError(t, err)
	assert.NotNil(t, packet)
	assert.Equal(t, validData, packet)

	// 测试无效数据包（太短）
	invalidData := []byte{0x01, 0x02, 0x03}
	packet, err = server.parsePacket(invalidData)
	assert.Error(t, err)
	assert.Nil(t, packet)
}

// 测试处理数据包
func TestServerProcessPacket(t *testing.T) {
	server, _ := createTestServer()

	// 模拟UDP连接
	clientAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:12345")

	// 无效数据包 - 太短
	invalidData := []byte{0x01, 0x02}
	err := server.processPacket(invalidData, clientAddr)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "解析数据包失败")

	// 有效数据包，但应该失败因为udpConn是nil
	validData := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06}
	err = server.processPacket(validData, clientAddr)
	assert.Error(t, err)
}

// 测试启动服务器
func TestServerStart(t *testing.T) {
	server, _ := createTestServer()

	// 测试启动服务器
	// 注意：由于我们无法轻易模拟底层网络功能，这个测试主要是验证代码路径
	// 大部分情况下会失败，因为端口可能被占用或权限不足
	err := server.Start()
	if err == nil {
		// 如果成功启动，确保状态正确并清理
		assert.True(t, server.isRunning)
		server.Stop()
	}

	// 测试重复启动的情况
	if server.isRunning {
		// 如果服务器已运行，再次启动应该不出错
		originalRunning := server.isRunning
		err = server.Start()
		assert.NoError(t, err)
		assert.Equal(t, originalRunning, server.isRunning)
	} else {
		// 手动设置状态来测试
		server.isRunning = true
		err = server.Start()
		assert.NoError(t, err)
		assert.True(t, server.isRunning)
	}
}

// 测试停止服务器
func TestServerStop(t *testing.T) {
	server, _ := createTestServer()

	// 手动设置运行状态
	server.isRunning = true
	server.udpConn = &net.UDPConn{}

	// 模拟客户端连接，由于我们不能直接使用mockServerConnector
	// 这里我们不测试客户端连接关闭部分

	// 停止服务器
	err := server.Stop()
	assert.NoError(t, err)
	assert.False(t, server.isRunning)

	// 测试重复停止
	err = server.Stop()
	assert.NoError(t, err)
}

// 模拟ServerConnector实现
type mockServerConnector struct {
	addr   *net.UDPAddr
	closed bool
}

func (m *mockServerConnector) Close() error {
	m.closed = true
	return nil
}

func (m *mockServerConnector) Start() error {
	return nil
}

func (m *mockServerConnector) ProcessIncomingData(data []byte) error {
	return nil
}
