package client

import (
	"bytes"
	"io"
	"net"
	"sync"
	"time"
)

// Logger 定义日志接口，便于依赖注入和测试
type Logger interface {
	Debug(args ...interface{})
	Debugf(format string, args ...interface{})
	Info(args ...interface{})
	Infof(format string, args ...interface{})
	Error(args ...interface{})
	Errorf(format string, args ...interface{})
}

// VirtualSocks5Conn 虚拟的SOCKS5连接，用于处理SOCKS5协议
// 它包装了一个底层连接，并模拟SOCKS5协议的握手流程
type VirtualSocks5Conn struct {
	net.Conn                    // 原始连接
	readBuf        bytes.Buffer // 读取缓冲区
	hasSentAuth    bool         // 是否已发送认证响应
	hasSentConnect bool         // 是否已发送连接响应
	hasRecvAuth    bool         // 是否已收到认证响应
	originalData   []byte       // 原始请求数据
	currentPos     int          // 当前读取位置
	log            Logger       // 日志记录器
	closed         bool         // 连接是否已关闭
	mu             sync.Mutex   // 互斥锁
	originalConn   net.Conn     // 原始连接，用于写回数据

	// 新增字段：用于等待认证响应的通道
	authRespReceivedChan chan struct{}
	authTimeout          time.Duration // 认证响应等待超时
}

// NewVirtualSocks5Conn 创建一个新的虚拟SOCKS5连接
func NewVirtualSocks5Conn(conn net.Conn, originalData []byte, log Logger) *VirtualSocks5Conn {
	log.Debugf("创建VirtualSocks5Conn: 原始数据长度=%d, 原始数据=%x", len(originalData), originalData)
	return &VirtualSocks5Conn{
		Conn:                 conn,
		originalData:         originalData,
		log:                  log,
		originalConn:         conn,
		authRespReceivedChan: make(chan struct{}, 1),
		authTimeout:          3 * time.Second, // 设置3秒认证响应超时
	}
}

// Read 实现了 io.Reader 接口，按照SOCKS5协议顺序读取数据
func (v *VirtualSocks5Conn) Read(b []byte) (n int, err error) {
	v.log.Debugf("==================== READ ====================")
	defer v.log.Debugf("==================== READ END ====================")

	v.mu.Lock()
	// 暂时不解锁，等需要读取实际连接时再解锁

	if v.closed {
		v.mu.Unlock()
		return 0, io.EOF
	}

	// 首先检查读缓冲区是否有数据
	if v.readBuf.Len() > 0 {
		n, err = v.readBuf.Read(b)
		if err != nil {
			v.log.Debugf("读缓冲区读取失败: %v", err)
		}
		v.log.Debugf("[Read-Buffer] 从读缓冲区读取数据: %d字节, 数据: % x", n, b[:n])
		v.mu.Unlock()
		return n, err
	}

	// 记录当前状态
	currentPos := v.currentPos
	hasRecvAuth := v.hasRecvAuth
	hasSentAuth := v.hasSentAuth
	hasSentConnect := v.hasSentConnect
	totalData := len(v.originalData)

	v.log.Debugf("[Read-Status] 当前状态: pos=%d, totalData=%d, hasRecvAuth=%v, hasSentAuth=%v, hasSentConnect=%v",
		currentPos, totalData, hasRecvAuth, hasSentAuth, hasSentConnect)

	// SOCKS5协议步骤1: 发送握手请求 (VER + NMETHODS + METHODS[NMETHODS])
	// greeting长度 = 2 + originalData[1]
	greetingLen := 2
	if len(v.originalData) >= 2 {
		greetingLen = 2 + int(v.originalData[1])
		if greetingLen > len(v.originalData) {
			greetingLen = len(v.originalData)
		}
	}

	if currentPos < greetingLen {
		remaining := greetingLen - currentPos
		if remaining > len(b) {
			remaining = len(b)
		}
		copy(b, v.originalData[currentPos:currentPos+remaining])
		v.currentPos = currentPos + remaining
		v.hasSentAuth = (v.currentPos >= greetingLen)
		v.log.Debugf("[Read-Handshake] 发送握手请求: %d字节 (greetingLen=%d), 数据: % x", remaining, greetingLen, b[:remaining])
		v.mu.Unlock()
		return remaining, nil
	}

	// 步骤2: 等待认证响应 - 使用通道真正阻塞
	if v.hasSentAuth && !v.hasRecvAuth {
		// 先解锁，避免在等待过程中占用锁
		v.mu.Unlock()

		v.log.Debugf("[Read-Auth] 等待认证响应...")

		// 使用select实现带超时的等待
		select {
		case <-v.authRespReceivedChan:
			v.log.Debugf("[Read-Auth] 收到认证响应信号")
			// 不需要做任何事，继续执行
		case <-time.After(v.authTimeout):
			v.log.Debugf("[Read-Auth] 等待认证响应超时，模拟接收认证响应")
			// 超时后，我们模拟接收认证响应并发送信号
			close(v.authRespReceivedChan) // 关闭通道表示已完成
		}

		// 再次加锁，更新状态
		v.mu.Lock()
		v.hasRecvAuth = true
	}

	// 步骤3: 发送连接请求 - 如果尚未发送
	if v.hasRecvAuth && !v.hasSentConnect && currentPos < totalData {
		remaining := totalData - currentPos
		if remaining > len(b) {
			remaining = len(b)
		}
		copy(b, v.originalData[currentPos:currentPos+remaining])
		v.currentPos = currentPos + remaining
		v.hasSentConnect = (v.currentPos >= totalData)
		v.log.Debugf("[Read-Connect] 发送连接请求: %d字节, 数据: % x", remaining, b[:remaining])
		v.mu.Unlock()
		return remaining, nil
	}

	// 步骤4: 应用数据阶段
	v.mu.Unlock() // 解锁，允许并发读写
	v.log.Debugf("[Read-App] 准备读取应用数据")
	n, err = v.Conn.Read(b)
	if err != nil {
		if err != io.EOF {
			v.log.Errorf("[Read-App] 读取应用数据失败: %v", err)
		}
		return n, err
	}

	v.log.Debugf("[Read-App] 读取应用数据成功: %d字节, 数据: % x", n, b[:n])
	return n, nil
}

// Write 实现io.Writer接口，处理SOCKS5服务器的响应
func (v *VirtualSocks5Conn) Write(data []byte) (n int, err error) {
	if len(data) == 0 {
		return 0, nil
	}

	// 获取数据类型
	dataType := getDataType(data)
	v.log.Debugf("[Write] 写入数据，类型: %s，长度: %d", dataType, len(data))

	v.mu.Lock()
	defer v.mu.Unlock()

	// 处理SOCKS5认证响应
	if dataType == "AUTH_RESP" && !v.hasRecvAuth {
		v.log.Debugf("[Write] 收到SOCKS5认证响应: %v", data)
		v.hasRecvAuth = true

		// 发送信号通知Read方法认证响应已收到
		close(v.authRespReceivedChan)

		// 不转发认证响应给客户端，只是假装写入成功
		v.log.Debugf("[Write] 假装写入认证响应成功: %d字节", len(data))
		return len(data), nil
	}

	// 处理连接响应
	if dataType == "CONNECT_RESP" && !v.hasSentConnect {
		v.log.Debugf("[Write-Connect] 发送连接响应: % x", data)
		v.hasSentConnect = true
		n, err = v.originalConn.Write(data)
		if err != nil {
			v.log.Errorf("[Write-Connect] 写入连接响应失败: %v", err)
			return n, err
		}
		v.log.Debugf("[Write-Connect] 写入连接响应成功: %d字节", n)
		return n, nil
	}

	// 处理应用数据
	v.log.Debugf("[Write-App] 写入应用数据: %d字节", len(data))

	// 分块写入大数据包，避免"message too long"错误
	const MaxWriteSize = 8000 // 略小于MaxUDPPacketSize以留出安全边界
	if len(data) > MaxWriteSize {
		v.log.Debugf("[Write-App] 数据过大，分块写入: 总大小=%d字节", len(data))
		remaining := len(data)
		offset := 0

		for remaining > 0 {
			chunkSize := remaining
			if chunkSize > MaxWriteSize {
				chunkSize = MaxWriteSize
			}

			chunk := data[offset : offset+chunkSize]
			v.log.Debugf("[Write-App] 写入数据块: 大小=%d字节, 偏移=%d", chunkSize, offset)

			chunkN, chunkErr := v.originalConn.Write(chunk)
			if chunkErr != nil {
				v.log.Errorf("[Write-App] 写入数据块失败: %v", chunkErr)
				return offset + chunkN, chunkErr
			}

			v.log.Debugf("[Write-App] 写入数据块成功: %d字节", chunkN)
			offset += chunkSize
			remaining -= chunkSize

			// 短暂延迟，避免网络拥塞
			if remaining > 0 {
				time.Sleep(2 * time.Millisecond)
			}
		}

		return len(data), nil
	}

	// 对于小数据包，直接写入
	n, err = v.originalConn.Write(data)
	if err != nil {
		v.log.Errorf("[Write-App] 写入应用数据失败: %v", err)
	} else {
		v.log.Debugf("[Write-App] 写入应用数据成功: %d字节", n)
	}

	return n, err
}

// Close 实现了 io.Closer 接口
func (v *VirtualSocks5Conn) Close() error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.closed {
		return nil
	}

	// 先标记为已关闭
	v.closed = true

	// 检查通道是否已关闭，如果没有关闭且需要，则关闭它
	// 这样可以避免向已关闭的通道发送数据
	if v.hasRecvAuth {
		// 如果已经收到了认证响应，通道应该已经被关闭了
		// 不需要再做任何事情
	} else {
		// 如果还没收到认证响应，关闭通道以取消阻塞读取
		close(v.authRespReceivedChan)
	}

	v.log.Debug("关闭虚拟SOCKS5连接")
	return v.Conn.Close()
}

// getDataType 辅助函数，用于识别数据类型
func getDataType(data []byte) string {
	if len(data) == 0 {
		return "EMPTY"
	}
	if len(data) == 2 && data[0] == 0x05 && data[1] == 0x00 {
		return "AUTH_RESP"
	}
	if len(data) >= 4 && data[0] == 0x05 && data[1] == 0x00 && data[2] == 0x00 {
		return "CONNECT_RESP"
	}
	return "APP_DATA"
}

// GetReadBuffer 返回内部读缓冲区，仅用于测试
func (v *VirtualSocks5Conn) GetReadBuffer() *bytes.Buffer {
	return &v.readBuf
}

// SetAuthTimeout 设置认证超时时间，仅用于测试
func (v *VirtualSocks5Conn) SetAuthTimeout(timeout time.Duration) {
	v.authTimeout = timeout
}
