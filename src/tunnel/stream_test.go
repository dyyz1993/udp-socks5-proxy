package tunnel

import (
	"bytes"
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// mockConn 实现 net.Conn 接口，用于测试
type mockConn struct {
	readData  []byte
	writeData []byte
	readPos   int
	closed    bool
	readErr   error
	writeErr  error
	mu        sync.Mutex
}

func newMockConn(readData []byte) *mockConn {
	return &mockConn{
		readData:  readData,
		writeData: make([]byte, 0),
	}
}

func (m *mockConn) Read(b []byte) (n int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.readErr != nil {
		return 0, m.readErr
	}

	if m.closed {
		return 0, io.EOF
	}

	if m.readPos >= len(m.readData) {
		return 0, io.EOF
	}

	n = copy(b, m.readData[m.readPos:])
	m.readPos += n
	return n, nil
}

func (m *mockConn) Write(b []byte) (n int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.writeErr != nil {
		return 0, m.writeErr
	}

	if m.closed {
		return 0, errors.New("connection closed")
	}

	m.writeData = append(m.writeData, b...)
	return len(b), nil
}

func (m *mockConn) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

func (m *mockConn) LocalAddr() net.Addr { return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 12345} }
func (m *mockConn) RemoteAddr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 54321}
}
func (m *mockConn) SetDeadline(t time.Time) error      { return nil }
func (m *mockConn) SetReadDeadline(t time.Time) error  { return nil }
func (m *mockConn) SetWriteDeadline(t time.Time) error { return nil }

// mockConnector 实现 TunnelConnector 接口，用于测试
type mockConnector struct {
	sentData       map[string][]byte
	removedStreams []string
	sendErr        error
	mu             sync.Mutex
}

func newMockConnector() *mockConnector {
	return &mockConnector{
		sentData: make(map[string][]byte),
	}
}

func (m *mockConnector) SendData(streamID string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.sendErr != nil {
		return m.sendErr
	}

	if _, ok := m.sentData[streamID]; !ok {
		m.sentData[streamID] = make([]byte, 0)
	}
	m.sentData[streamID] = append(m.sentData[streamID], data...)
	return nil
}

func (m *mockConnector) RemoveStream(streamID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.removedStreams = append(m.removedStreams, streamID)
}

// TestNewTunnelStreamImpl 测试创建新的隧道流
func TestNewTunnelStreamImpl(t *testing.T) {
	streamID := "test-stream-123"
	conn := newMockConnector()
	// targetAddr := "example.com:80"

	stream := NewTunnelStreamImpl(streamID, conn)

	if stream.streamID != streamID {
		t.Errorf("流ID不匹配: got %s, want %s", stream.streamID, streamID)
	}

	if stream.closed {
		t.Error("新创建的流不应该是关闭状态")
	}
}

// TestTunnelStreamImplPutData 测试数据投递功能
func TestTunnelStreamImplPutData(t *testing.T) {
	streamID := "test-stream-123"
	conn := newMockConnector()
	// targetAddr := "example.com:80"

	stream := NewTunnelStreamImpl(streamID, conn)

	testData := []byte("hello, world")

	// 测试正常投递
	err := stream.PutData(testData)
	if err != nil {
		t.Errorf("投递数据应成功，但返回错误: %v", err)
	}

	// 测试关闭后投递
	stream.Close()
	err = stream.PutData(testData)
	if err != ErrConnClosed {
		t.Errorf("关闭后投递应返回ErrConnClosed，但返回: %v", err)
	}
}

// TestTunnelStreamImplClose 测试关闭功能
func TestTunnelStreamImplClose(t *testing.T) {
	streamID := "test-stream-123"
	conn := newMockConnector()
	// targetAddr := "example.com:80"

	stream := NewTunnelStreamImpl(streamID, conn)

	// 测试正常关闭
	err := stream.Close()
	if err != nil {
		t.Errorf("关闭流应成功，但返回错误: %v", err)
	}

	if !stream.closed {
		t.Error("关闭后流的closed标志应为true")
	}

	// 确认连接器已被通知移除流
	if len(conn.removedStreams) != 1 || conn.removedStreams[0] != streamID {
		t.Errorf("关闭后应通知连接器移除流，但未收到通知, removedStreams=%v", conn.removedStreams)
	}

	// 测试重复关闭
	err = stream.Close()
	if err != nil {
		t.Errorf("重复关闭应成功，但返回错误: %v", err)
	}
}

// TestTunnelStreamImplPutDataAndRead 测试数据投递和读取
func TestTunnelStreamImplPutDataAndRead(t *testing.T) {
	streamID := "test-stream-data"
	conn := newMockConnector()
	// targetAddr := "example.com:80"

	stream := NewTunnelStreamImpl(streamID, conn)

	// 向流中投递数据
	testData := []byte("test-tunnel-data")
	err := stream.PutData(testData)
	if err != nil {
		t.Errorf("投递数据应成功，但返回错误: %v", err)
	}

	// 检查数据是否添加到读缓冲区
	select {
	case data := <-stream.readBuffer:
		if !bytes.Equal(data, testData) {
			t.Errorf("读取的数据不匹配: got %v, want %v", data, testData)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("无法从读缓冲区读取数据")
	}

	// 再次投递数据
	testData2 := []byte("more-tunnel-data")
	err = stream.PutData(testData2)
	if err != nil {
		t.Errorf("第二次投递数据应成功，但返回错误: %v", err)
	}

	// 检查第二次数据是否添加到读缓冲区
	select {
	case data := <-stream.readBuffer:
		if !bytes.Equal(data, testData2) {
			t.Errorf("第二次读取的数据不匹配: got %v, want %v", data, testData2)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("无法从读缓冲区读取第二次数据")
	}

	// 关闭流后不应该能够投递数据
	stream.Close()
	err = stream.PutData([]byte("should-not-work"))
	if err != ErrConnClosed {
		t.Errorf("关闭后投递应返回ErrConnClosed，但返回: %v", err)
	}
}

// TestTunnelStreamImplServeConnBasic 测试基本的ServeConn功能
func TestTunnelStreamImplServeConnBasic(t *testing.T) {
	streamID := "test-stream-close"
	conn := newMockConnector()
	// targetAddr := "example.com:80"

	stream := NewTunnelStreamImpl(streamID, conn)

	// 创建模拟连接
	mockClientConn := newMockConn([]byte("test"))

	// 在后台运行ServeConn
	done := make(chan struct{})
	go func() {
		_ = stream.ServeConn(mockClientConn)
		close(done)
	}()

	// 给ServeConn一些时间开始运行
	time.Sleep(100 * time.Millisecond)

	// 关闭客户端连接
	mockClientConn.Close()

	// 等待ServeConn完成
	select {
	case <-done:
		// 正常退出
	case <-time.After(1 * time.Second):
		t.Error("ServeConn未能在连接关闭后退出")
	}

	// 测试流关闭的情况
	if !stream.closed {
		t.Error("连接关闭后流应该也被关闭")
	}
}

// TestTunnelStreamImpl_PutGetData 测试流的数据放入和获取功能
func TestTunnelStreamImpl_PutGetData(t *testing.T) {
	// 创建模拟连接器
	connector := &mockConnector{}

	// 创建流
	streamID := "test-stream"
	// targetAddr := "test-target"
	stream := NewTunnelStreamImpl(streamID, connector)

	// 准备测试数据
	testData := []byte("hello tunnel stream")

	// 测试 PutData
	err := stream.PutData(testData)
	require.NoError(t, err, "应该能正常放入数据")

	// 测试 GetData
	data, err := stream.GetData()
	require.NoError(t, err, "应该能正常获取数据")
	require.True(t, bytes.Equal(testData, data), "获取的数据应该与放入的数据相同")

	// 测试空数据情况
	data, err = stream.GetData()
	require.NoError(t, err, "在没有数据时也不应该报错")
	require.Empty(t, data, "没有数据时应该返回空")

	// 测试关闭后的情况
	err = stream.Close()
	require.NoError(t, err, "应该能正常关闭")

	err = stream.PutData(testData)
	require.Error(t, err, "关闭后不应该能放入数据")

	_, err = stream.GetData()
	require.Error(t, err, "关闭后不应该能获取数据")
}
