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

func TestServerStream_Write(t *testing.T) {
	// Need a real UDP conn for Write (it calls SendPacket)
	addr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		t.Skipf("cannot listen: %v", err)
	}
	defer conn.Close()

	remoteAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:12345")
	sc := NewServerConnector(conn, remoteAddr)
	stream := newServerStream("s1", sc).(*serverStream)
	defer stream.Close()

	n, err := stream.Write([]byte("hello"))
	t.Logf("Write: n=%d err=%v", n, err)

	// Write to closed stream
	stream.Close()
	n, err = stream.Write([]byte("world"))
	assert.Equal(t, 0, n)
	assert.Error(t, err)
}

func TestServerStream_SendPacket(t *testing.T) {
	// Create a UDP listener to get a real conn
	addr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		t.Skipf("cannot listen: %v", err)
	}
	defer conn.Close()

	remoteAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:12345")
	sc := NewServerConnector(conn, remoteAddr)
	stream := newServerStream("s1", sc).(*serverStream)

	pkt := tunnel.NewDataPacket("c1", "s1", []byte("test"))
	err = stream.SendPacket(pkt.Bytes())
	t.Logf("SendPacket: %v", err)
}

func TestServerStream_SendPacketNilConnector(t *testing.T) {
	stream := newServerStream("s1", nil).(*serverStream)
	err := stream.SendPacket([]byte("data"))
	assert.NoError(t, err) // nil connector returns nil
}

func TestServerStream_SendErrorPacket(t *testing.T) {
	addr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		t.Skipf("cannot listen: %v", err)
	}
	defer conn.Close()

	remoteAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:12345")
	sc := NewServerConnector(conn, remoteAddr)
	stream := newServerStream("s1", sc).(*serverStream)

	err = stream.SendErrorPacket(1001, "test error")
	t.Logf("SendErrorPacket: %v", err)
}

func TestServerStream_SendErrorPacketNilConnector(t *testing.T) {
	stream := newServerStream("s1", nil).(*serverStream)
	err := stream.SendErrorPacket(1001, "test")
	assert.NoError(t, err)
}

func TestServerStream_PutDataAfterClose(t *testing.T) {
	sc := &ServerConnector{BaseConnector: tunnel.NewBaseConnector()}
	stream := newServerStream("s1", sc).(*serverStream)
	stream.Close()

	err := stream.PutData([]byte("data"))
	assert.Error(t, err)
}

func TestServerStream_ReadAfterClose(t *testing.T) {
	sc := &ServerConnector{BaseConnector: tunnel.NewBaseConnector()}
	stream := newServerStream("s1", sc).(*serverStream)
	stream.Close()

	buf := make([]byte, 10)
	_, err := stream.Read(buf)
	assert.Error(t, err)
}

func TestServerStream_DataFlow(t *testing.T) {
	sc := &ServerConnector{BaseConnector: tunnel.NewBaseConnector()}
	stream := newServerStream("s1", sc).(*serverStream)
	defer stream.Close()

	// Put data and read it back
	stream.PutData([]byte("hello"))
	buf := make([]byte, 10)
	n, err := stream.Read(buf)
	assert.NoError(t, err)
	assert.Equal(t, 5, n)
	assert.Equal(t, "hello", string(buf[:n]))
}
