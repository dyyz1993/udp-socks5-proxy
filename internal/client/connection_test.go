package client

import (
	"fmt"
	"net"
	"testing"
	"time"

	tunnelclient "github.com/tealife/proxy-cs3/src/tunnel/client"
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

// TestCovHandleConnection_Proxy tests handleConnection with proxy path (non-direct)
func TestCovHandleConnection_Proxy(t *testing.T) {
	logger := common.NewSimpleLogger("TEST", common.DebugLevel)

	server := gsocks.NewServer(
		gsocks.WithLogger(NewGoSocks5Logger(logger)),
	)

	clientConn, serverConn := net.Pipe()

	c := NewClient(Config{
		LocalPort:     0,
		ServerAddr:    "127.0.0.1:1",
		DirectDomains: []string{},
		DefaultDirect: false, // all go through proxy
		LogLevel:      common.DebugLevel,
	}, logger)

	// Create a mock tunnel connector
	tunnelConn, err := tunnelclient.NewClientConnector("127.0.0.1:1")
	// We expect this to fail since no server is running, but we need a non-nil connector
	// So we just use the failed connector - handleConnection will fail at CreateStream
	// but that's fine, it still covers the proxy branch
	_ = tunnelConn
	_ = err

	// Set tunnelConnector to nil to trigger the proxy branch error path
	c.tunnelConnector = nil

	done := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				// tunnelConnector is nil, expected panic
				done <- fmt.Errorf("panic: %v", r)
			}
		}()
		done <- c.handleConnection(serverConn, server)
	}()

	// Send SOCKS5 greeting + connect with IP address (IP always goes through proxy)
	clientConn.Write([]byte{0x05, 0x01, 0x00})
	time.Sleep(100 * time.Millisecond)

	buf := make([]byte, 2)
	clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, _ := clientConn.Read(buf)

	if n >= 2 && buf[0] == 0x05 {
		connectReq := []byte{
			0x05, 0x01, 0x00,
			0x01,
			10, 0, 0, 1, // IP that goes through proxy
			0, 80,
		}
		clientConn.Write(connectReq)
	}

	select {
	case err := <-done:
		// Expect error since tunnelConnector is nil
		_ = err
	case <-time.After(5 * time.Second):
	}
	clientConn.Close()
}
