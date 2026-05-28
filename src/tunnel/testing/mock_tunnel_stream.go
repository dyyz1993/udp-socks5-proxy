package testing

import (
	"bytes"
	"errors"
	"io"
	"net"
	"sync"
	"time"

	"github.com/tealife/proxy-cs3/src/tunnel"
)

var (
	// ErrStreamClosed 流已关闭
	ErrStreamClosed = errors.New("stream closed")
	// ErrSimulatedStreamError 模拟的流错误
	ErrSimulatedStreamError = errors.New("simulated stream error")
)

// MockTunnelStreamOptions 模拟隧道流的配置选项
type MockTunnelStreamOptions struct {
	// 流ID
	StreamID string
	// 目标地址
	TargetAddr string
	// 是否应该立即关闭成功
	ShouldCloseSucceed bool
	// 是否应该数据投递成功
	ShouldPutDataSucceed bool
	// 是否应该服务连接成功
	ShouldServeConnSucceed bool
	// 模拟的错误
	CloseError     error
	PutDataError   error
	ServeConnError error
}

// DefaultMockTunnelStreamOptions 默认的模拟隧道流配置
var DefaultMockTunnelStreamOptions = MockTunnelStreamOptions{
	StreamID:               "mock-stream-id",
	TargetAddr:             "example.com:80",
	ShouldCloseSucceed:     true,
	ShouldPutDataSucceed:   true,
	ShouldServeConnSucceed: true,
}

// MockTunnelStream 实现TunnelStream接口的模拟隧道流
type MockTunnelStream struct {
	options    MockTunnelStreamOptions
	connector  tunnel.TunnelConnector // 关联的连接器
	closed     bool
	dataBuffer bytes.Buffer // 存储通过PutData投递的数据

	// 同步
	mutex     sync.Mutex
	closeChan chan struct{}

	// 记录调用历史，用于测试验证
	closeCalled    bool
	putDataCalls   []PutDataCall
	serveConnCalls []ServeConnCall

	// 用于测试的回调函数
	OnClose     func() error
	OnPutData   func(data []byte) error
	OnServeConn func(conn net.Conn) error
}

// PutDataCall 记录PutData调用信息
type PutDataCall struct {
	Data []byte
	Time time.Time
}

// ServeConnCall 记录ServeConn调用信息
type ServeConnCall struct {
	Conn net.Conn
	Time time.Time
}

// NewMockTunnelStream 创建一个新的模拟隧道流
func NewMockTunnelStream(streamID string, connector tunnel.TunnelConnector, targetAddr string) *MockTunnelStream {
	opts := DefaultMockTunnelStreamOptions
	opts.StreamID = streamID
	opts.TargetAddr = targetAddr

	return NewMockTunnelStreamWithOptions(connector, opts)
}

// NewMockTunnelStreamWithOptions 使用自定义选项创建模拟隧道流
func NewMockTunnelStreamWithOptions(connector tunnel.TunnelConnector, opts MockTunnelStreamOptions) *MockTunnelStream {
	return &MockTunnelStream{
		options:        opts,
		connector:      connector,
		closed:         false,
		closeChan:      make(chan struct{}),
		putDataCalls:   make([]PutDataCall, 0),
		serveConnCalls: make([]ServeConnCall, 0),
	}
}

// ID 实现ID方法
func (ms *MockTunnelStream) ID() string {
	return ms.options.StreamID
}

// TargetAddr 实现TargetAddr方法
func (ms *MockTunnelStream) TargetAddr() string {
	return ms.options.TargetAddr
}

// Close 实现Close方法
func (ms *MockTunnelStream) Close() error {
	ms.mutex.Lock()
	defer ms.mutex.Unlock()

	ms.closeCalled = true

	// 如果有自定义回调，使用回调
	if ms.OnClose != nil {
		return ms.OnClose()
	}

	// 根据配置决定是否成功
	if !ms.options.ShouldCloseSucceed {
		if ms.options.CloseError != nil {
			return ms.options.CloseError
		}
		return ErrSimulatedStreamError
	}

	if ms.closed {
		return nil // 已经关闭
	}

	ms.closed = true
	close(ms.closeChan)

	// 从连接器中移除流
	if ms.connector != nil {
		ms.connector.(*MockConnector).BaseConnector.RemoveStream(ms.options.StreamID)
	}

	return nil
}

// PutData 实现PutData方法
func (ms *MockTunnelStream) PutData(data []byte) error {
	ms.mutex.Lock()
	defer ms.mutex.Unlock()

	ms.putDataCalls = append(ms.putDataCalls, PutDataCall{
		Data: data,
		Time: time.Now(),
	})

	// 如果有自定义回调，使用回调
	if ms.OnPutData != nil {
		return ms.OnPutData(data)
	}

	// 根据配置决定是否成功
	if !ms.options.ShouldPutDataSucceed {
		if ms.options.PutDataError != nil {
			return ms.options.PutDataError
		}
		return ErrSimulatedStreamError
	}

	if ms.closed {
		return ErrStreamClosed
	}

	// 存储数据
	ms.dataBuffer.Write(data)
	return nil
}

// GetData 实现GetData方法
func (ms *MockTunnelStream) GetData() ([]byte, error) {
	ms.mutex.Lock()
	defer ms.mutex.Unlock()

	if ms.closed {
		return nil, ErrStreamClosed
	}

	// 如果缓冲区为空，返回空数据
	if ms.dataBuffer.Len() == 0 {
		return nil, nil
	}

	// 读取缓冲区中的所有数据
	data := ms.dataBuffer.Bytes()
	ms.dataBuffer.Reset() // 清空缓冲区
	return data, nil
}

// ServeConn 实现ServeConn方法
func (ms *MockTunnelStream) ServeConn(conn net.Conn) error {
	ms.mutex.Lock()

	// 检查流状态
	if ms.closed {
		ms.mutex.Unlock()
		return ErrStreamClosed
	}

	// 记录调用
	ms.serveConnCalls = append(ms.serveConnCalls, ServeConnCall{
		Conn: conn,
		Time: time.Now(),
	})

	// 如果有自定义回调，使用回调
	if ms.OnServeConn != nil {
		ms.mutex.Unlock()
		return ms.OnServeConn(conn)
	}

	// 根据配置决定是否成功
	if !ms.options.ShouldServeConnSucceed {
		ms.mutex.Unlock()
		if ms.options.ServeConnError != nil {
			return ms.options.ServeConnError
		}
		return ErrSimulatedStreamError
	}

	// 创建一个本地副本以避免死锁
	closeChan := ms.closeChan
	dataBuffer := ms.dataBuffer.Bytes()
	ms.mutex.Unlock()

	// 如果有预先存储的数据，发送给连接
	if len(dataBuffer) > 0 {
		_, err := conn.Write(dataBuffer)
		if err != nil {
			return err
		}
	}

	// 简单的IO复制，从连接读取数据并发送到连接器
	go func() {
		buffer := make([]byte, 1024)
		for {
			// 从连接读取数据
			n, err := conn.Read(buffer)
			if err != nil {
				if err != io.EOF {
					// TODO: 记录错误
				}
				break
			}

			// 通过连接器发送数据
			if n > 0 && ms.connector != nil {
				ms.connector.SendData(ms.options.StreamID, buffer[:n])
			}
		}
	}()

	// 等待关闭信号
	<-closeChan

	return nil
}

// WasCloseCalled 检查Close方法是否被调用
func (ms *MockTunnelStream) WasCloseCalled() bool {
	ms.mutex.Lock()
	defer ms.mutex.Unlock()

	return ms.closeCalled
}

// GetPutDataCalls 获取PutData方法的调用记录
func (ms *MockTunnelStream) GetPutDataCalls() []PutDataCall {
	ms.mutex.Lock()
	defer ms.mutex.Unlock()

	return ms.putDataCalls
}

// GetServeConnCalls 获取ServeConn方法的调用记录
func (ms *MockTunnelStream) GetServeConnCalls() []ServeConnCall {
	ms.mutex.Lock()
	defer ms.mutex.Unlock()

	return ms.serveConnCalls
}

// IsClosed 检查流是否已关闭
func (ms *MockTunnelStream) IsClosed() bool {
	ms.mutex.Lock()
	defer ms.mutex.Unlock()

	return ms.closed
}

// GetBuffer 获取数据缓冲区的内容
func (ms *MockTunnelStream) GetBuffer() []byte {
	ms.mutex.Lock()
	defer ms.mutex.Unlock()

	return ms.dataBuffer.Bytes()
}

// GetStreamID 返回流ID，实现TunnelStream接口
func (ms *MockTunnelStream) GetStreamID() string {
	return ms.options.StreamID
}
