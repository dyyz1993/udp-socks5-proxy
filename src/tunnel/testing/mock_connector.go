package testing

import (
	"fmt"
	"sync"
	"time"

	"github.com/tealife/proxy-cs3/src/tunnel"
)

// MockConnectorOptions 模拟连接器的配置选项
type MockConnectorOptions struct {
	// 模拟的连接ID
	ConnectionID string
	// 初始状态
	InitialState tunnel.ConnectionState
	// 是否应该立即连接成功
	ShouldConnectSucceed bool
	// 是否应该立即关闭成功
	ShouldCloseSucceed bool
	// 是否应该创建流成功
	ShouldCreateStreamSucceed bool
	// 是否应该发送数据成功
	ShouldSendDataSucceed bool
	// 网络连接选项
	NetConnOptions MockNetConnOptions
	// 模拟的错误
	ConnectError      error
	CloseError        error
	SendDataError     error
	CreateStreamError error
}

// DefaultMockConnectorOptions 默认的模拟连接器配置
var DefaultMockConnectorOptions = MockConnectorOptions{
	ConnectionID:              "mock-connection-id",
	InitialState:              tunnel.StateInitialized,
	ShouldConnectSucceed:      true,
	ShouldCloseSucceed:        true,
	ShouldCreateStreamSucceed: true,
	ShouldSendDataSucceed:     true,
	NetConnOptions:            DefaultMockNetConnOptions,
}

// MockConnector 实现TunnelConnector接口的模拟连接器
type MockConnector struct {
	*tunnel.BaseConnector
	options MockConnectorOptions

	// 状态跟踪
	isRunning bool
	sentData  map[string][]byte

	// 网络连接
	conn *MockNetConn

	// 同步锁
	mutex sync.Mutex

	// 记录调用历史，用于测试验证
	connectCalled     bool
	closeCalled       bool
	sendDataCalls     []SendDataCall
	createStreamCalls []CreateStreamCall

	// 用于测试的回调函数
	OnConnect      func() error
	OnClose        func() error
	OnSendData     func(streamID string, data []byte) error
	OnCreateStream func(targetAddr string) (string, tunnel.TunnelStream, error)
}

// SendDataCall 记录SendData调用信息
type SendDataCall struct {
	StreamID string
	Data     []byte
	Time     time.Time
}

// CreateStreamCall 记录CreateStream调用信息
type CreateStreamCall struct {
	TargetAddr string
	Time       time.Time
}

// NewMockConnector 创建一个新的模拟连接器
func NewMockConnector() *MockConnector {
	return NewMockConnectorWithOptions(DefaultMockConnectorOptions)
}

// NewMockConnectorWithOptions 使用自定义选项创建模拟连接器
func NewMockConnectorWithOptions(opts MockConnectorOptions) *MockConnector {
	baseConnector := tunnel.NewBaseConnector()

	// 如果设置了特定的连接ID，覆盖基础连接器生成的ID
	if opts.ConnectionID != "" {
		// 由于BaseConnector没有提供直接设置connectionID的方法，
		// 这里采用反射或其他方式设置可能不太安全，所以我们先保留基础连接器生成的ID
	}

	conn := NewMockNetConnWithOptions(opts.NetConnOptions)

	return &MockConnector{
		BaseConnector:     baseConnector,
		options:           opts,
		isRunning:         false,
		sentData:          make(map[string][]byte),
		conn:              conn,
		sendDataCalls:     make([]SendDataCall, 0),
		createStreamCalls: make([]CreateStreamCall, 0),
	}
}

// Connect 实现Connect方法
func (mc *MockConnector) Connect() error {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	mc.connectCalled = true

	// 如果有自定义回调，使用回调
	if mc.OnConnect != nil {
		return mc.OnConnect()
	}

	// 根据配置决定是否成功
	if !mc.options.ShouldConnectSucceed {
		if mc.options.ConnectError != nil {
			return mc.options.ConnectError
		}
		return fmt.Errorf("模拟连接失败")
	}

	mc.isRunning = true
	mc.BaseConnector.SetState(tunnel.StateConnected)
	return nil
}

// Start 实现Start方法
func (mc *MockConnector) Start() error {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	mc.isRunning = true
	mc.BaseConnector.SetState(tunnel.StateConnected)
	return nil
}

// Close 实现Close方法
func (mc *MockConnector) Close() error {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	mc.closeCalled = true

	// 如果有自定义回调，使用回调
	if mc.OnClose != nil {
		return mc.OnClose()
	}

	// 根据配置决定是否成功
	if !mc.options.ShouldCloseSucceed {
		if mc.options.CloseError != nil {
			return mc.options.CloseError
		}
		return fmt.Errorf("模拟关闭失败")
	}

	mc.isRunning = false
	mc.BaseConnector.SetState(tunnel.StateClosed)
	return nil
}

// SendData 实现SendData方法
func (mc *MockConnector) SendData(streamID string, data []byte) error {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	// 记录调用
	mc.sendDataCalls = append(mc.sendDataCalls, SendDataCall{
		StreamID: streamID,
		Data:     data,
		Time:     time.Now(),
	})

	// 如果有自定义回调，使用回调
	if mc.OnSendData != nil {
		return mc.OnSendData(streamID, data)
	}

	// 检查连接状态
	if !mc.isRunning {
		return tunnel.ErrConnClosed
	}

	// 根据配置决定是否成功
	if !mc.options.ShouldSendDataSucceed {
		if mc.options.SendDataError != nil {
			return mc.options.SendDataError
		}
		return fmt.Errorf("模拟发送数据失败")
	}

	// 存储发送的数据，用于测试验证
	if mc.sentData[streamID] == nil {
		mc.sentData[streamID] = make([]byte, 0)
	}
	mc.sentData[streamID] = append(mc.sentData[streamID], data...)

	return nil
}

// CreateStream 实现CreateStream方法
func (mc *MockConnector) CreateStream(targetAddr string) (string, tunnel.TunnelStream, error) {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	// 记录调用
	mc.createStreamCalls = append(mc.createStreamCalls, CreateStreamCall{
		TargetAddr: targetAddr,
		Time:       time.Now(),
	})

	// 如果有自定义回调，使用回调
	if mc.OnCreateStream != nil {
		return mc.OnCreateStream(targetAddr)
	}

	// 检查连接状态
	if !mc.isRunning {
		return "", nil, tunnel.ErrConnClosed
	}

	// 根据配置决定是否成功
	if !mc.options.ShouldCreateStreamSucceed {
		if mc.options.CreateStreamError != nil {
			return "", nil, mc.options.CreateStreamError
		}
		return "", nil, fmt.Errorf("模拟创建流失败")
	}

	// 创建一个模拟的流ID
	streamID := fmt.Sprintf("mock-stream-%d", time.Now().UnixNano())

	// 创建一个模拟的流
	stream := NewMockTunnelStream(streamID, mc, targetAddr)

	// 添加到基础连接器管理
	mc.BaseConnector.AddStream(streamID, stream)

	return streamID, stream, nil
}

// ProcessIncomingData 实现ProcessIncomingData方法
func (mc *MockConnector) ProcessIncomingData(data []byte) error {
	// 这里不需要详细实现处理逻辑，因为测试通常只验证方法被调用，
	// 而不验证具体处理逻辑。实际处理逻辑在实际的连接器中实现。
	return nil
}

// IsRunning 返回连接器是否运行中
func (mc *MockConnector) IsRunning() bool {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	return mc.isRunning
}

// GetSentData 获取发送到指定流的数据
func (mc *MockConnector) GetSentData(streamID string) []byte {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	return mc.sentData[streamID]
}

// GetAllSentData 获取所有已发送的数据
func (mc *MockConnector) GetAllSentData() map[string][]byte {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	// 返回数据的副本，避免并发修改
	result := make(map[string][]byte)
	for k, v := range mc.sentData {
		dataCopy := make([]byte, len(v))
		copy(dataCopy, v)
		result[k] = dataCopy
	}

	return result
}

// WasConnectCalled 检查Connect方法是否被调用
func (mc *MockConnector) WasConnectCalled() bool {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	return mc.connectCalled
}

// WasCloseCalled 检查Close方法是否被调用
func (mc *MockConnector) WasCloseCalled() bool {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	return mc.closeCalled
}

// GetSendDataCalls 获取SendData方法的调用记录
func (mc *MockConnector) GetSendDataCalls() []SendDataCall {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	return mc.sendDataCalls
}

// GetCreateStreamCalls 获取CreateStream方法的调用记录
func (mc *MockConnector) GetCreateStreamCalls() []CreateStreamCall {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	return mc.createStreamCalls
}

// SetInitialState 设置初始状态
func (mc *MockConnector) SetInitialState(state tunnel.ConnectionState) {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	mc.BaseConnector.SetState(state)
	if state == tunnel.StateConnected {
		mc.isRunning = true
	} else if state == tunnel.StateClosed {
		mc.isRunning = false
	}
}

// GetConn 获取模拟的网络连接
func (mc *MockConnector) GetConn() *MockNetConn {
	return mc.conn
}
