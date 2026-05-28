package testing

import (
	"bytes"
	"errors"
	"io"
	"math/rand"
	"net"
	"sync"
	"time"
)

var (
	// ErrConnClosed 连接已关闭
	ErrConnClosed = errors.New("connection closed")
	// ErrSimulatedReadError 模拟的读取错误
	ErrSimulatedReadError = errors.New("simulated read error")
	// ErrSimulatedWriteError 模拟的写入错误
	ErrSimulatedWriteError = errors.New("simulated write error")
	// ErrSimulatedTimeout 模拟的超时错误
	ErrSimulatedTimeout = errors.New("simulated timeout")
)

// MockNetConnOptions 模拟网络连接的配置选项
type MockNetConnOptions struct {
	// 读取延迟
	ReadDelay time.Duration
	// 写入延迟
	WriteDelay time.Duration
	// 丢包率 (0-1)
	PacketLossRate float32
	// 读取错误率 (0-1)
	ReadErrorRate float32
	// 写入错误率 (0-1)
	WriteErrorRate float32
	// 提前设置的初始读取数据
	InitialReadData []byte
	// 本地地址
	LocalAddr net.Addr
	// 远程地址
	RemoteAddr net.Addr
}

// DefaultMockNetConnOptions 默认的模拟网络连接配置
var DefaultMockNetConnOptions = MockNetConnOptions{
	ReadDelay:      0,
	WriteDelay:     0,
	PacketLossRate: 0,
	ReadErrorRate:  0,
	WriteErrorRate: 0,
	LocalAddr:      &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12345},
	RemoteAddr:     &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 54321},
}

// 预定义的网络条件
var (
	// GoodNetworkCondition 良好的网络条件
	GoodNetworkCondition = MockNetConnOptions{
		ReadDelay:      time.Millisecond,
		WriteDelay:     time.Millisecond,
		PacketLossRate: 0,
		ReadErrorRate:  0,
		WriteErrorRate: 0,
	}

	// PoorNetworkCondition 较差的网络条件
	PoorNetworkCondition = MockNetConnOptions{
		ReadDelay:      50 * time.Millisecond,
		WriteDelay:     50 * time.Millisecond,
		PacketLossRate: 0.05,
		ReadErrorRate:  0.01,
		WriteErrorRate: 0.01,
	}

	// UnstableNetworkCondition 不稳定的网络条件
	UnstableNetworkCondition = MockNetConnOptions{
		ReadDelay:      100 * time.Millisecond,
		WriteDelay:     100 * time.Millisecond,
		PacketLossRate: 0.1,
		ReadErrorRate:  0.05,
		WriteErrorRate: 0.05,
	}
)

// MockNetConn 实现net.Conn接口的模拟网络连接
type MockNetConn struct {
	readBuf       bytes.Buffer
	writeBuf      bytes.Buffer
	closed        bool
	options       MockNetConnOptions
	readDeadline  time.Time
	writeDeadline time.Time
	mutex         sync.Mutex

	// 用于测试的回调函数
	OnRead  func([]byte) (int, error)
	OnWrite func([]byte) (int, error)
	OnClose func() error
}

// NewMockNetConn 创建一个新的模拟网络连接
func NewMockNetConn() *MockNetConn {
	return NewMockNetConnWithOptions(DefaultMockNetConnOptions)
}

// NewMockNetConnWithOptions 使用自定义选项创建模拟网络连接
func NewMockNetConnWithOptions(opts MockNetConnOptions) *MockNetConn {
	conn := &MockNetConn{
		options: opts,
	}

	if opts.InitialReadData != nil {
		conn.readBuf.Write(opts.InitialReadData)
	}

	return conn
}

// Read 实现io.Reader接口
func (m *MockNetConn) Read(b []byte) (n int, err error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 如果有自定义的Read回调，优先使用
	if m.OnRead != nil {
		return m.OnRead(b)
	}

	// 检查连接状态
	if m.closed {
		return 0, ErrConnClosed
	}

	// 检查是否设置了读取截止时间，并且已经过期
	if !m.readDeadline.IsZero() && m.readDeadline.Before(time.Now()) {
		return 0, ErrSimulatedTimeout
	}

	// 模拟读取延迟
	if m.options.ReadDelay > 0 {
		time.Sleep(m.options.ReadDelay)
	}

	// 随机模拟读取错误
	if m.options.ReadErrorRate > 0 && rand.Float32() < m.options.ReadErrorRate {
		return 0, ErrSimulatedReadError
	}

	// 模拟丢包（直接丢弃部分数据）
	if m.options.PacketLossRate > 0 && m.readBuf.Len() > 0 && rand.Float32() < m.options.PacketLossRate {
		// 丢弃部分数据
		discardSize := rand.Intn(m.readBuf.Len())
		if discardSize > 0 {
			discardData := make([]byte, discardSize)
			m.readBuf.Read(discardData) // 丢弃数据
		}
	}

	// 如果没有数据可读，返回EOF
	if m.readBuf.Len() == 0 {
		return 0, io.EOF
	}

	// 读取数据
	return m.readBuf.Read(b)
}

// Write 实现io.Writer接口
func (m *MockNetConn) Write(b []byte) (n int, err error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 如果有自定义的Write回调，优先使用
	if m.OnWrite != nil {
		return m.OnWrite(b)
	}

	// 检查连接状态
	if m.closed {
		return 0, ErrConnClosed
	}

	// 检查是否设置了写入截止时间，并且已经过期
	if !m.writeDeadline.IsZero() && m.writeDeadline.Before(time.Now()) {
		return 0, ErrSimulatedTimeout
	}

	// 模拟写入延迟
	if m.options.WriteDelay > 0 {
		time.Sleep(m.options.WriteDelay)
	}

	// 随机模拟写入错误
	if m.options.WriteErrorRate > 0 && rand.Float32() < m.options.WriteErrorRate {
		return 0, ErrSimulatedWriteError
	}

	// 模拟丢包（直接丢弃部分数据）
	if m.options.PacketLossRate > 0 && rand.Float32() < m.options.PacketLossRate {
		discardSize := rand.Intn(len(b) + 1)
		if discardSize == len(b) {
			// 整个包都丢失了，但是我们报告成功写入，这模拟了网络上的静默丢包
			return len(b), nil
		}
		// 部分丢包
		b = b[discardSize:]
	}

	// 写入数据
	return m.writeBuf.Write(b)
}

// Close 关闭连接
func (m *MockNetConn) Close() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 如果有自定义的Close回调，优先使用
	if m.OnClose != nil {
		return m.OnClose()
	}

	if m.closed {
		return nil // 已经关闭，直接返回成功
	}

	m.closed = true
	return nil
}

// LocalAddr 返回本地地址
func (m *MockNetConn) LocalAddr() net.Addr {
	return m.options.LocalAddr
}

// RemoteAddr 返回远程地址
func (m *MockNetConn) RemoteAddr() net.Addr {
	return m.options.RemoteAddr
}

// SetDeadline 设置读写截止时间
func (m *MockNetConn) SetDeadline(t time.Time) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.readDeadline = t
	m.writeDeadline = t
	return nil
}

// SetReadDeadline 设置读取截止时间
func (m *MockNetConn) SetReadDeadline(t time.Time) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.readDeadline = t
	return nil
}

// SetWriteDeadline 设置写入截止时间
func (m *MockNetConn) SetWriteDeadline(t time.Time) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.writeDeadline = t
	return nil
}

// GetWrittenData 获取已写入的数据，便于测试
func (m *MockNetConn) GetWrittenData() []byte {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	return m.writeBuf.Bytes()
}

// AddReadData 添加可读取的数据，便于测试
func (m *MockNetConn) AddReadData(data []byte) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.readBuf.Write(data)
}

// ClearWrittenData 清除已写入的数据，便于测试
func (m *MockNetConn) ClearWrittenData() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.writeBuf.Reset()
}

// IsClosed 检查连接是否已关闭
func (m *MockNetConn) IsClosed() bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	return m.closed
}

// GetOptions 线程安全地获取连接选项
func (m *MockNetConn) GetOptions() MockNetConnOptions {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	return m.options
}

// ApplyCondition 线程安全地应用网络条件到连接
func (m *MockNetConn) ApplyCondition(condition MockNetConnOptions) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.options.ReadDelay = condition.ReadDelay
	m.options.WriteDelay = condition.WriteDelay
	m.options.PacketLossRate = condition.PacketLossRate
	m.options.ReadErrorRate = condition.ReadErrorRate
	m.options.WriteErrorRate = condition.WriteErrorRate
}

// SetClosed 线程安全地设置关闭状态
func (m *MockNetConn) SetClosed(closed bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.closed = closed
}

// NewErrorMockNetConn 创建一个在写入时总是返回指定错误的MockNetConn
func NewErrorMockNetConn(errMsg string) *MockNetConn {
	conn := NewMockNetConn()

	// 设置写入操作始终返回错误
	conn.OnWrite = func(b []byte) (int, error) {
		return 0, errors.New(errMsg)
	}

	return conn
}
