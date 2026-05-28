package tunnel

import (
	"io"
	"log"
	"net"
	"sync"
	"time"
)

// TunnelStream 接口定义
type TunnelStream interface {
	// ServeConn 处理连接转发
	ServeConn(conn net.Conn) error
	// Close 关闭流
	Close() error
	// PutData 投递数据到流
	PutData(data []byte) error
	// GetData 获取数据，用于测试
	GetData() ([]byte, error)
	// GetStreamID 获取流ID
	GetStreamID() string
}

// TunnelConnector 定义了隧道连接器接口
type TunnelConnector interface {
	SendData(streamID string, data []byte) error
	RemoveStream(streamID string)
}

// TunnelStreamImpl 实现 TunnelStream 接口
type TunnelStreamImpl struct {
	streamID string
	conn     TunnelConnector
	// targetAddr string
	readBuffer chan []byte
	closeChan  chan struct{}
	closeOnce  sync.Once
	closed     bool
	mu         sync.RWMutex
}

// NewTunnelStreamImpl 创建新的隧道流
func NewTunnelStreamImpl(streamID string, conn TunnelConnector) *TunnelStreamImpl {
	log.Printf("[NewTunnelStreamImpl] 创建新的隧道流: streamID=%s", streamID)
	return &TunnelStreamImpl{
		streamID:   streamID,
		conn:       conn,
		readBuffer: make(chan []byte, 1024),
		closeChan:  make(chan struct{}),
	}
}

// ServeConn 实现连接转发
func (s *TunnelStreamImpl) ServeConn(conn net.Conn) error {
	log.Printf("[ServeConn] 开始处理连接转发: streamID=%s", s.streamID)

	// 设置连接超时
	// conn.SetDeadline(time.Now().Add(5 * time.Second))
	// log.Printf("[ServeConn] 设置连接超时: 5秒")

	// 启动读写协程
	errChan := make(chan error, 2)

	// 从客户端读取数据并发送到隧道
	go func() {
		log.Printf("[ServeConn] 启动客户端读取协程: streamID=%s", s.streamID)
		buf := make([]byte, 4096)
		for {
			select {
			case <-s.closeChan:
				log.Printf("[ServeConn] 收到关闭信号，退出客户端读取协程: streamID=%s", s.streamID)
				errChan <- nil
				return
			default:
				n, err := conn.Read(buf)
				if err != nil {
					if err != io.EOF {
						log.Printf("[ServeConn] 读取客户端数据错误: streamID=%s, error=%v", s.streamID, err)
						errChan <- err
					} else {
						log.Printf("[ServeConn] 客户端连接关闭: streamID=%s", s.streamID)
						errChan <- nil
					}
					return
				}
				log.Printf("[ServeConn] 读取客户端数据: streamID=%s, 长度=%d", s.streamID, n)
				// 更详细的字节级日志
				log.Printf("[ServeConn] 原始客户端数据(十六进制): % x", buf[:n])
				if n > 0 {
					log.Printf("[ServeConn] 客户端数据前16个字节(逐字节): ")
					for i := 0; i < n && i < 16; i++ {
						log.Printf("[ServeConn] byte[%d]=%d (0x%02x)", i, buf[i], buf[i])
					}
				}
				// 十六进制数据
				log.Printf("[ServeConn] 十六进制数据: %x", buf[:n])
				// 发送数据到隧道
				if err := s.conn.SendData(s.streamID, buf[:n]); err != nil {
					log.Printf("[ServeConn] 发送数据到隧道错误: streamID=%s, error=%v", s.streamID, err)
					errChan <- err
					return
				}
				log.Printf("[ServeConn] 数据发送到隧道成功: streamID=%s, 长度=%d", s.streamID, n)

				// 更新超时
				conn.SetDeadline(time.Now().Add(5 * time.Second))
			}
		}
	}()

	// 从隧道读取数据并发送到客户端
	go func() {
		log.Printf("[ServeConn] 启动隧道读取协程: streamID=%s", s.streamID)
		for {
			select {
			case <-s.closeChan:
				log.Printf("[ServeConn] 收到关闭信号，退出隧道读取协程: streamID=%s", s.streamID)
				errChan <- nil
				return
			case data := <-s.readBuffer:
				log.Printf("[ServeConn] 从隧道接收数据: streamID=%s, 长度=%d", s.streamID, len(data))
				_, err := conn.Write(data)
				if err != nil {
					log.Printf("[ServeConn] 写入客户端数据错误: streamID=%s, error=%v", s.streamID, err)
					errChan <- err
					return
				}
				log.Printf("[ServeConn] 数据写入客户端成功: streamID=%s, 长度=%d", s.streamID, len(data))
				// 更新超时
				conn.SetDeadline(time.Now().Add(5 * time.Second))
			}
		}
	}()

	// 等待错误或关闭
	log.Printf("[ServeConn] 等待错误或关闭信号: streamID=%s", s.streamID)
	err := <-errChan
	if err != nil {
		log.Printf("[ServeConn] 收到错误信号: streamID=%s, error=%v", s.streamID, err)
	} else {
		log.Printf("[ServeConn] 收到正常关闭信号: streamID=%s", s.streamID)
	}
	s.Close()
	return err
}

// Close 关闭流
func (s *TunnelStreamImpl) Close() error {
	s.closeOnce.Do(func() {
		log.Printf("[Close] 开始关闭流: streamID=%s", s.streamID)
		s.mu.Lock()
		s.closed = true
		s.mu.Unlock()

		close(s.closeChan)
		s.conn.RemoveStream(s.streamID)
		log.Printf("[Close] 流关闭完成: streamID=%s", s.streamID)
	})
	return nil
}

// PutData 投递数据到流
func (s *TunnelStreamImpl) PutData(data []byte) error {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return ErrConnClosed
	}
	s.mu.RUnlock()

	// 打印详细的数据流向日志
	log.Printf("[数据流向-流] PutData 被调用: streamID=%s, 数据长度=%d", s.streamID, len(data))

	// 数据类型分析
	dataType := "未知数据"
	if len(data) > 0 {
		if len(data) <= 32 {
			log.Printf("[数据流向-流-HEX] 完整数据: %x", data)
		} else {
			log.Printf("[数据流向-流-HEX] 前32字节: %x...", data[:32])
		}

		// 分析SOCKS5协议数据
		if len(data) >= 1 && data[0] == 0x05 {
			// 认证响应: 05 00
			if len(data) == 2 && data[1] == 0x00 {
				dataType = "SOCKS5认证响应"
				log.Printf("[数据流向-流-分析] 数据类型: %s - 传递给客户端", dataType)
				// 注释掉过滤逻辑，确保认证响应能被传递给客户端
				// 过滤掉认证响应，不传递给客户端
				// return nil
			} else if len(data) == 3 && data[1] == 0x01 && data[2] == 0x00 {
				// 握手请求: 05 01 00
				dataType = "SOCKS5握手请求"
			} else if len(data) > 3 && data[1] == 0x01 && data[2] == 0x00 {
				// 连接请求: 05 01 00 [地址类型] ...
				dataType = "SOCKS5连接请求"
			} else if len(data) > 3 && data[1] == 0x00 && data[2] == 0x00 {
				// 连接响应: 05 00 00 ...
				dataType = "SOCKS5连接响应"
			} else {
				// 其他SOCKS5数据
				dataType = "其他SOCKS5数据"
			}

			log.Printf("[数据流向-流-分析] 数据类型: %s", dataType)
		}

		// 打印每个字节的详细信息（最多打印前16个字节）
		maxBytes := 16
		if len(data) < maxBytes {
			maxBytes = len(data)
		}
		log.Printf("[数据流向-流-字节分析] 数据前%d字节:", maxBytes)
		for i, b := range data[:maxBytes] {
			log.Printf("  字节[%d] = %d (0x%02x)", i, b, b)
		}
	}

	// 将数据写入读缓冲区
	select {
	case s.readBuffer <- data:
		log.Printf("[数据流向-流] 数据已放入流的读缓冲区: streamID=%s, 数据长度=%d, 类型=%s",
			s.streamID, len(data), dataType)
		return nil
	case <-s.closeChan:
		log.Printf("[数据流向-流] 流已关闭，无法投递数据: streamID=%s", s.streamID)
		return ErrConnClosed
	}
}

// GetData 从流中获取数据
func (s *TunnelStreamImpl) GetData() ([]byte, error) {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return nil, ErrConnClosed
	}
	s.mu.RUnlock()

	// 从读缓冲区获取数据
	select {
	case data := <-s.readBuffer:
		return data, nil
	case <-s.closeChan:
		return nil, ErrConnClosed
	case <-time.After(100 * time.Millisecond):
		// 如果一段时间内没有数据，返回空数据
		return nil, nil
	}
}

// GetStreamID 获取流ID
func (s *TunnelStreamImpl) GetStreamID() string {
	return s.streamID
}
