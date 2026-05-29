package client

import (
	"net"
	"testing"
	"time"

	gsocks "github.com/things-go/go-socks5"

	"github.com/tealife/proxy-cs3/internal/common"
)

// TestCovHandleConnection_Direct tests handleConnection with direct connect
func TestCovHandleConnection_Direct(t *testing.T) {
	logger := common.NewSimpleLogger("TEST", common.DebugLevel)

	server := gsocks.NewServer(
		gsocks.WithLogger(NewGoSocks5Logger(logger)),
	)

	clientConn, serverConn := net.Pipe()

	c := NewClient(Config{
		LocalPort:     0,
		ServerAddr:    "127.0.0.1:1",
		DirectDomains: []string{},
		DefaultDirect: true, // default direct means all go direct
		LogLevel:      common.DebugLevel,
	}, logger)

	c.tunnelConnector = nil

	done := make(chan error, 1)
	go func() {
		done <- c.handleConnection(serverConn, server)
	}()

	// Send SOCKS5 greeting
	clientConn.Write([]byte{0x05, 0x01, 0x00})
	time.Sleep(100 * time.Millisecond)

	buf := make([]byte, 2)
	clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, _ := clientConn.Read(buf)

	if n >= 2 && buf[0] == 0x05 {
		connectReq := []byte{
			0x05, 0x01, 0x00,
			0x03,                     // domain
			byte(len("example.com")), // domain length
		}
		connectReq = append(connectReq, []byte("example.com")...)
		connectReq = append(connectReq, 0x00, 0x50) // port 80
		clientConn.Write(connectReq)
	}

	select {
	case <-done:
	case <-time.After(3 * time.Second):
	}
	clientConn.Close()
}
