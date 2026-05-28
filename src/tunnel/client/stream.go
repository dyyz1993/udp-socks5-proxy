package client

import (
	"net"

	"github.com/tealife/proxy-cs3/src/tunnel"
)

// ClientStream 客户端隧道流
type clientStream struct {
	*tunnel.TunnelStreamImpl
}

// newClientStream 创建新的客户端流
func newClientStream(streamID string, conn tunnel.TunnelConnector) tunnel.TunnelStream {
	return &clientStream{
		TunnelStreamImpl: tunnel.NewTunnelStreamImpl(streamID, conn),
	}
}

// ServeConn 实现连接转发
func (s *clientStream) ServeConn(conn net.Conn) error {
	return s.TunnelStreamImpl.ServeConn(conn)
}

// Close 关闭流
func (s *clientStream) Close() error {
	return s.TunnelStreamImpl.Close()
}

// PutData 投递数据到流
func (s *clientStream) PutData(data []byte) error {
	return s.TunnelStreamImpl.PutData(data)
}
