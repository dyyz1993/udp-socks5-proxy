package tunnel

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// BaseConnector 实现TunnelConnector接口的基础结构
type BaseConnector struct {
	// 连接ID
	connectionID string

	// 连接状态
	state ConnectionState

	// 流映射
	streams     map[string]TunnelStream
	streamMutex sync.RWMutex

	// 连接互斥锁
	connMutex sync.Mutex
}

// NewBaseConnector 创建一个新的基础连接器
func NewBaseConnector() *BaseConnector {
	// 简单生成一个随机ID
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	connID := fmt.Sprintf("conn-%d-%d", time.Now().Unix(), r.Intn(100000))

	return &BaseConnector{
		connectionID: connID,
		state:        StateInitialized,
		streams:      make(map[string]TunnelStream),
	}
}

// GetConnectionID 获取连接ID
func (bc *BaseConnector) GetConnectionID() string {
	return bc.connectionID
}

// SetConnectionID 设置连接ID
func (bc *BaseConnector) SetConnectionID(id string) {
	bc.connectionID = id
}

// IsConnected 检查是否已连接
func (bc *BaseConnector) IsConnected() bool {
	bc.connMutex.Lock()
	defer bc.connMutex.Unlock()
	return bc.state == StateConnected
}

// SetState 设置连接状态
func (bc *BaseConnector) SetState(state ConnectionState) {
	bc.connMutex.Lock()
	defer bc.connMutex.Unlock()
	bc.state = state
}

// AddStream 添加一个数据流
func (bc *BaseConnector) AddStream(streamID string, stream TunnelStream) {
	bc.streamMutex.Lock()
	defer bc.streamMutex.Unlock()
	bc.streams[streamID] = stream
}

// GetStream 获取一个数据流
func (bc *BaseConnector) GetStream(streamID string) (TunnelStream, error) {
	bc.streamMutex.RLock()
	defer bc.streamMutex.RUnlock()

	stream, ok := bc.streams[streamID]
	if !ok {
		return nil, ErrStreamNotFound
	}

	return stream, nil
}

// RemoveStream 移除一个数据流
func (bc *BaseConnector) RemoveStream(streamID string) {
	bc.streamMutex.Lock()
	defer bc.streamMutex.Unlock()

	delete(bc.streams, streamID)
}

// Connect 连接到远程服务器 (空实现，需要在子类中实现)
func (bc *BaseConnector) Connect() error {
	return nil
}

// Close 关闭连接 (空实现，需要在子类中实现)
func (bc *BaseConnector) Close() error {
	return nil
}

// SendData 发送数据 (空实现，需要在子类中实现)
func (bc *BaseConnector) SendData(streamID string, data []byte) error {
	return nil
}

// CreateStream 创建新的数据流 (空实现，需要在子类中实现)
func (bc *BaseConnector) CreateStream(targetAddr string) (string, TunnelStream, error) {
	return "", nil, nil
}

// ProcessIncomingData 处理传入的数据 (空实现，需要在子类中实现)
func (bc *BaseConnector) ProcessIncomingData(data []byte) error {
	return nil
}

// Start 启动连接器 (空实现，需要在子类中实现)
func (bc *BaseConnector) Start() error {
	return nil
}
