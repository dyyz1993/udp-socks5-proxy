package server

import (
	"io"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/tealife/proxy-cs3/src/tunnel"
)

// TestServerStreamBasics 测试ServerStream的基本功能
func TestServerStreamBasics(t *testing.T) {
	baseConnector := tunnel.NewBaseConnector()
	baseConnector.SetConnectionID("test-conn")

	// 创建测试用ServerConnector
	sc := &ServerConnector{
		BaseConnector: baseConnector,
	}

	// 创建一个ServerStream
	streamID := "test-stream"
	stream := newServerStream(streamID, sc).(*serverStream)

	// 测试基本属性
	assert.Equal(t, streamID, stream.GetStreamID())
	assert.NotNil(t, stream.readBuffer)

	// 测试Close
	err := stream.Close()
	assert.NoError(t, err)
	assert.True(t, stream.closed)

	// 再次关闭应该没问题
	err = stream.Close()
	assert.NoError(t, err)
}

// TestServerStreamReadWrite 测试读写功能
func TestServerStreamReadWrite(t *testing.T) {
	baseConnector := tunnel.NewBaseConnector()
	baseConnector.SetConnectionID("test-conn")

	// 创建测试用ServerConnector
	sc := &ServerConnector{
		BaseConnector: baseConnector,
	}

	// 创建一个ServerStream
	streamID := "test-stream"
	stream := newServerStream(streamID, sc).(*serverStream)

	// 测试读数据前，先放入一些数据
	testData := []byte("test-data")
	err := stream.PutData(testData)
	assert.NoError(t, err)

	// 读取数据
	buf := make([]byte, 100)
	n, err := stream.Read(buf)
	assert.NoError(t, err)
	assert.Equal(t, len(testData), n)
	assert.Equal(t, testData, buf[:n])

	// 关闭后读写应该失败
	err = stream.Close()
	assert.NoError(t, err)

	_, err = stream.Read(buf)
	assert.Error(t, err)
	assert.Equal(t, io.EOF, err)

	err = stream.PutData(testData)
	assert.Error(t, err)
}

// TestServerStreamNetConn 测试作为net.Conn的功能
func TestServerStreamNetConn(t *testing.T) {
	baseConnector := tunnel.NewBaseConnector()
	baseConnector.SetConnectionID("test-conn")

	// 创建测试用ServerConnector
	sc := &ServerConnector{
		BaseConnector: baseConnector,
	}

	// 创建一个ServerStream
	streamID := "test-stream"
	stream := newServerStream(streamID, sc).(*serverStream)

	// 测试是否实现了net.Conn接口
	var conn net.Conn = stream

	// 测试接口方法
	addr := conn.LocalAddr()
	assert.NotNil(t, addr)

	addr = conn.RemoteAddr()
	assert.NotNil(t, addr)

	// 测试设置超时
	err := conn.SetDeadline(time.Now().Add(time.Second))
	assert.NoError(t, err)

	err = conn.SetReadDeadline(time.Now().Add(time.Second))
	assert.NoError(t, err)

	err = conn.SetWriteDeadline(time.Now().Add(time.Second))
	assert.NoError(t, err)
}
