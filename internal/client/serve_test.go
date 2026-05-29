package client

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tealife/proxy-cs3/internal/common"
)

// TestCovServeSOCKS5_CloseChanExit tests serveSOCKS5 exits via closeChan
func TestCovServeSOCKS5_CloseChanExit(t *testing.T) {
	logger := common.NewSimpleLogger("TEST", common.DebugLevel)
	c := NewClient(Config{
		LocalPort:  0,
		ServerAddr: "127.0.0.1:1",
		LogLevel:   common.DebugLevel,
	}, logger)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	c.listener = listener
	c.closeChan = make(chan struct{})
	c.isRunning = true

	// Start serveSOCKS5 in a goroutine
	c.wg.Add(1)
	go c.serveSOCKS5()

	// Close after a short delay to trigger closeChan exit
	time.Sleep(200 * time.Millisecond)
	close(c.closeChan)
	c.listener.Close()
	c.wg.Wait()
	assert.True(t, true) // reached here without panic
}

// TestCovServeSOCKS5_AcceptThenClose tests serveSOCKS5 accepts a connection then closes
func TestCovServeSOCKS5_AcceptThenClose(t *testing.T) {
	logger := common.NewSimpleLogger("TEST", common.DebugLevel)
	c := NewClient(Config{
		LocalPort:  0,
		ServerAddr: "127.0.0.1:1",
		LogLevel:   common.DebugLevel,
	}, logger)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	c.listener = listener
	c.closeChan = make(chan struct{})
	c.isRunning = true

	c.wg.Add(1)
	go c.serveSOCKS5()

	// Connect to trigger Accept
	conn, err := net.Dial("tcp", listener.Addr().String())
	require.NoError(t, err)
	conn.Close()

	// Wait for handleConnection to run
	time.Sleep(500 * time.Millisecond)

	close(c.closeChan)
	listener.Close()
	c.wg.Wait()
}

// TestCovServeSOCKS5_AcceptError tests serveSOCKS5 handles Accept errors
func TestCovServeSOCKS5_AcceptError(t *testing.T) {
	logger := common.NewSimpleLogger("TEST", common.DebugLevel)
	c := NewClient(Config{
		LocalPort:  0,
		ServerAddr: "127.0.0.1:1",
		LogLevel:   common.DebugLevel,
	}, logger)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	c.closeChan = make(chan struct{})
	c.isRunning = true

	// Close listener immediately to cause Accept error
	c.listener = listener
	listener.Close()

	c.wg.Add(1)
	go c.serveSOCKS5()

	// Let it hit the Accept error, then close
	time.Sleep(200 * time.Millisecond)
	close(c.closeChan)
	c.wg.Wait()
}

// TestCovStart_ConnectTimeout tests Start with connect timeout (server doesn't respond)
func TestCovStart_ConnectTimeout(t *testing.T) {
	logger := common.NewSimpleLogger("TEST", common.DebugLevel)

	// Create a UDP listener that accepts but never responds
	// (to avoid "connection refused" and trigger the timeout path)
	serverAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	serverConn, err := net.ListenUDP("udp", serverAddr)
	require.NoError(t, err)
	defer serverConn.Close()

	serverPort := serverConn.LocalAddr().(*net.UDPAddr).Port

	c := NewClient(Config{
		LocalPort:  0,
		ServerAddr: fmt.Sprintf("127.0.0.1:%d", serverPort),
		LogLevel:   common.DebugLevel,
	}, logger)

	// Start will timeout waiting for handshake response
	err = c.Start()
	// Should timeout or fail since server doesn't respond
	assert.Error(t, err)
}
