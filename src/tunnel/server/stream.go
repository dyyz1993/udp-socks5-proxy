package server

import (
	"errors"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"github.com/tealife/proxy-cs3/src/tunnel"
)

// ServerStream 服务端隧道流，同时实现 net.Conn 接口
type serverStream struct {
	*tunnel.TunnelStreamImpl
	serverConnector *ServerConnector
	readBuffer      chan []byte // 读缓冲区
	mu              sync.Mutex  // 互斥锁
	closed          bool        // 连接是否关闭
}

// newServerStream 创建新的服务端流
func newServerStream(streamID string, conn *ServerConnector) tunnel.TunnelStream {
	return &serverStream{
		TunnelStreamImpl: tunnel.NewTunnelStreamImpl(streamID, conn),
		serverConnector:  conn,
		readBuffer:       make(chan []byte, 1024), // 创建读缓冲区
	}
}

// Read 实现 io.Reader 接口
func (s *serverStream) Read(b []byte) (n int, err error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return 0, io.EOF
	}
	s.mu.Unlock()

	log.Printf("[ServerStream] Read 被调用: streamID=%s", s.GetStreamID())

	// 从缓冲区读取数据
	select {
	case data, ok := <-s.readBuffer:
		if !ok {
			return 0, io.EOF
		}
		log.Printf("[ServerStream] Read 读取数据: streamID=%s, 数据长度=%d", s.GetStreamID(), len(data))
		n = copy(b, data)
		return n, nil
	case <-time.After(5 * time.Second):
		log.Printf("[ServerStream] Read 超时: streamID=%s", s.GetStreamID())
		return 0, io.EOF
	}
}

// Write 实现 io.Writer 接口
func (s *serverStream) Write(b []byte) (n int, err error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return 0, io.ErrClosedPipe
	}
	s.mu.Unlock()

	// 记录写入操作
	log.Printf("[ServerStream] Write: streamID=%s, 数据长度=%d bytes", s.GetStreamID(), len(b))
	if len(b) <= 32 {
		log.Printf("[ServerStream] Write 完整数据: %x", b)
	} else {
		log.Printf("[ServerStream] Write 数据前32字节: %x...", b[:32])
	}

	// 创建数据包并发送
	connID := s.serverConnector.BaseConnector.GetConnectionID()
	packet := tunnel.NewDataPacket(connID, s.GetStreamID(), b)
	err = s.SendPacket(packet.Bytes())

	if err != nil {
		log.Printf("[ServerStream] Write 发送失败: %v", err)
		return 0, err
	}
	return len(b), nil
}

// Close 关闭流
func (s *serverStream) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	close(s.readBuffer)
	s.mu.Unlock()

	log.Printf("[ServerStream] 关闭: streamID=%s", s.GetStreamID())
	return s.TunnelStreamImpl.Close()
}

// PutData 投递数据到流的读缓冲区
func (s *serverStream) PutData(data []byte) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return io.ErrClosedPipe
	}
	s.mu.Unlock()

	// 记录数据接收
	log.Printf("[ServerStream] PutData: streamID=%s, 数据长度=%d bytes", s.GetStreamID(), len(data))
	if len(data) <= 32 {
		log.Printf("[ServerStream] PutData 完整数据: %x", data)
	} else {
		log.Printf("[ServerStream] PutData 数据前32字节: %x...", data[:32])
	}

	// 放入读缓冲区
	select {
	case s.readBuffer <- data:
		return nil
	default:
		// 检查是否已关闭（再次检查以避免竞态条件）
		s.mu.Lock()
		if s.closed {
			s.mu.Unlock()
			return io.ErrClosedPipe
		}
		s.mu.Unlock()
		log.Printf("[ServerStream] 读缓冲区已满，丢弃数据: streamID=%s", s.GetStreamID())
		return errors.New("读缓冲区已满")
	}
}

// LocalAddr 实现 net.Conn 接口
func (s *serverStream) LocalAddr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}
}

// RemoteAddr 实现 net.Conn 接口
func (s *serverStream) RemoteAddr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}
}

// SetDeadline 实现 net.Conn 接口
func (s *serverStream) SetDeadline(t time.Time) error {
	return nil
}

// SetReadDeadline 实现 net.Conn 接口
func (s *serverStream) SetReadDeadline(t time.Time) error {
	return nil
}

// SetWriteDeadline 实现 net.Conn 接口
func (s *serverStream) SetWriteDeadline(t time.Time) error {
	return nil
}

// SendPacket 发送原始数据包
func (s *serverStream) SendPacket(packetData []byte) error {
	if s.serverConnector != nil {
		return s.serverConnector.SendPacket(packetData)
	}
	return nil
}

// SendErrorPacket 发送错误数据包
func (s *serverStream) SendErrorPacket(code int, message string) error {
	if s.serverConnector == nil {
		return nil
	}

	packet := tunnel.NewErrorPacket(
		s.serverConnector.BaseConnector.GetConnectionID(),
		code,
		message,
		s.GetStreamID(),
	)

	return s.serverConnector.SendPacket(packet.Bytes())
}
