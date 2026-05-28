package client

import (
	"bytes"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/tealife/proxy-cs3/src/tunnel"
	tunnelTesting "github.com/tealife/proxy-cs3/src/tunnel/testing"
)

// mockConnector 模拟TunnelConnector用于测试
type mockConnector struct {
	mu            sync.Mutex
	sentData      map[string][]byte
	streamCreated bool
	streamClosed  bool
	streamID      string
	targetAddr    string
}

func newMockConnector() *mockConnector {
	return &mockConnector{
		sentData: make(map[string][]byte),
	}
}

func (c *mockConnector) Start() error {
	return nil
}

func (c *mockConnector) Close() error {
	return nil
}

func (c *mockConnector) IsConnected() bool {
	return true
}

func (c *mockConnector) GetConnectionID() string {
	return "mock-connection-id"
}

func (c *mockConnector) CreateStream(targetAddr string) (string, tunnel.TunnelStream, error) {
	c.streamCreated = true
	c.targetAddr = targetAddr
	c.streamID = "mock-stream-id"
	stream := newClientStream(c.streamID, c)
	return c.streamID, stream, nil
}

func (c *mockConnector) SendData(streamID string, data []byte) error {
	c.mu.Lock()
	c.sentData[streamID] = data
	c.mu.Unlock()
	return nil
}

func (c *mockConnector) GetSentData(streamID string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	data, ok := c.sentData[streamID]
	return data, ok
}

func (c *mockConnector) AddStream(streamID string, stream tunnel.TunnelStream) {
}

func (c *mockConnector) GetStream(streamID string) (tunnel.TunnelStream, error) {
	return nil, nil
}

func (c *mockConnector) RemoveStream(streamID string) {
	c.streamClosed = true
}

func (c *mockConnector) ProcessIncomingData(data []byte) error {
	return nil
}

// TestNewClientStream 测试创建新的客户端流
func TestNewClientStream(t *testing.T) {
	connector := newMockConnector()
	streamID := "test-stream-id"
	// targetAddr := "example.com:80"

	stream := newClientStream(streamID, connector)

	if stream == nil {
		t.Fatal("创建的流不应为nil")
	}

	clientStream, ok := stream.(*clientStream)
	if !ok {
		t.Fatal("流应该是clientStream类型")
	}

	if clientStream.TunnelStreamImpl == nil {
		t.Fatal("TunnelStreamImpl不应为nil")
	}
}

// TestClientStreamServeConn 测试客户端流的ServeConn方法
func TestClientStreamServeConn(t *testing.T) {
	// 创建模拟连接器
	connector := newMockConnector()
	streamID := "test-stream-id"
	// targetAddr := "example.com:80"

	// 创建客户端流
	stream := newClientStream(streamID, connector)

	// 创建模拟连接
	mockConn := tunnelTesting.NewMockNetConn()

	// 设置要读取的测试数据
	testData := []byte("test-data-for-serveconn")
	mockConn.AddReadData(testData)

	// 在goroutine中调用ServeConn
	done := make(chan error)
	go func() {
		err := stream.ServeConn(mockConn)
		done <- err
	}()

	// 等待数据处理完成
	time.Sleep(100 * time.Millisecond)

	// 验证数据是否被转发到连接器
	if sentData, ok := connector.GetSentData(streamID); ok {
		if !bytes.Equal(sentData, testData) {
			t.Errorf("数据未正确转发，期望: %v, 实际: %v", testData, sentData)
		}
	} else {
		t.Error("连接器未收到数据")
	}

	// 关闭连接，让ServeConn返回
	mockConn.Close()

	// 等待ServeConn完成
	select {
	case err := <-done:
		if err != nil && err != io.EOF && err != tunnelTesting.ErrConnClosed {
			t.Errorf("ServeConn返回非预期的错误: %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("ServeConn没有在连接关闭后及时返回")
	}
}

// TestClientStreamPutData 测试客户端流的PutData方法
func TestClientStreamPutData(t *testing.T) {
	// 创建模拟连接器
	connector := newMockConnector()
	streamID := "test-stream-putdata"
	// targetAddr := "example.com:80"

	// 创建客户端流
	stream := newClientStream(streamID, connector)

	// 测试PutData
	testData := []byte("test-data-for-putdata")
	err := stream.PutData(testData)
	if err != nil {
		t.Fatalf("PutData返回错误: %v", err)
	}

	// 不再测试readBuffer中的数据，直接测试关闭后的PutData
	clientStream, ok := stream.(*clientStream)
	if !ok {
		t.Fatal("无法转换为clientStream类型")
	}

	// 现在关闭流
	clientStream.Close()

	// 测试关闭后的PutData
	err = stream.PutData([]byte("should-fail"))
	if err == nil {
		t.Error("关闭后的PutData应该返回错误")
	} else if err != tunnel.ErrConnClosed {
		t.Errorf("期望错误类型为ErrConnClosed，实际错误为: %v", err)
	}
}

// TestClientStreamClose 测试客户端流的Close方法
func TestClientStreamClose(t *testing.T) {
	connector := newMockConnector()
	streamID := "test-stream-id"
	// targetAddr := "example.com:80"

	stream := newClientStream(streamID, connector)

	// 关闭流
	err := stream.Close()
	if err != nil {
		t.Fatalf("Close应成功，但返回错误: %v", err)
	}

	// 再次关闭应该也不会返回错误
	err = stream.Close()
	if err != nil {
		t.Errorf("再次Close应成功，但返回错误: %v", err)
	}
}

// 使用 tunnelTesting 包中的 MockNetConn 替换自定义实现
