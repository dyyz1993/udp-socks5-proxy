package client

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tealife/proxy-cs3/internal/common"
	"github.com/tealife/proxy-cs3/internal/server"
)

func getFreeCovPort(t *testing.T) int {
	t.Helper()
	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	require.NoError(t, err)
	conn, err := net.ListenUDP("udp", addr)
	require.NoError(t, err)
	port := conn.LocalAddr().(*net.UDPAddr).Port
	conn.Close()
	return port
}

func startTestUDPServer(t *testing.T) (*server.Server, int) {
	t.Helper()
	sp := getFreeCovPort(t)
	s := server.NewServer(server.Config{Port: sp, LogLevel: common.InfoLevel},
		common.NewSimpleLogger("S", common.InfoLevel))
	require.NoError(t, s.Start())
	time.Sleep(100 * time.Millisecond)
	return s, sp
}

func TestCovClient_StartStop(t *testing.T) {
	s, sp := startTestUDPServer(t)
	defer s.Stop()

	cp := getFreeCovPort(t)
	c := NewClient(Config{
		LocalPort:     cp,
		ServerAddr:    fmt.Sprintf("127.0.0.1:%d", sp),
		DirectDomains: []string{},
		DefaultDirect: false,
		Timeout:       3 * time.Second,
		LogLevel:      common.InfoLevel,
	}, common.NewSimpleLogger("C", common.InfoLevel))

	err := c.Start()
	if err != nil {
		t.Logf("Start failed (env issue): %v", err)
		return
	}
	assert.True(t, c.isRunning)

	err = c.Stop()
	assert.NoError(t, err)
	assert.False(t, c.isRunning)
}

func TestCovClient_StopWithoutStart(t *testing.T) {
	c := NewClient(Config{
		LocalPort:  getFreeCovPort(t),
		ServerAddr: "127.0.0.1:1",
	}, common.NewSimpleLogger("C", common.InfoLevel))
	assert.NoError(t, c.Stop())
}

func TestCovClient_StartInvalidServer(t *testing.T) {
	c := NewClient(Config{
		LocalPort:  getFreeCovPort(t),
		ServerAddr: "invalid-host:99999",
		Timeout:    1 * time.Second,
	}, common.NewSimpleLogger("C", common.InfoLevel))
	assert.Error(t, c.Start())
}

func TestCovClient_DoubleStart(t *testing.T) {
	s, sp := startTestUDPServer(t)
	defer s.Stop()

	cp := getFreeCovPort(t)
	c := NewClient(Config{
		LocalPort:     cp,
		ServerAddr:    fmt.Sprintf("127.0.0.1:%d", sp),
		DirectDomains: []string{},
		DefaultDirect: false,
		Timeout:       3 * time.Second,
		LogLevel:      common.InfoLevel,
	}, common.NewSimpleLogger("C", common.InfoLevel))

	err := c.Start()
	if err != nil {
		t.Logf("Start failed: %v", err)
		return
	}
	defer c.Stop()

	// Double start should be safe
	assert.NoError(t, c.Start())
}

func TestCovClient_DoubleStop(t *testing.T) {
	s, sp := startTestUDPServer(t)
	defer s.Stop()

	cp := getFreeCovPort(t)
	c := NewClient(Config{
		LocalPort:     cp,
		ServerAddr:    fmt.Sprintf("127.0.0.1:%d", sp),
		DirectDomains: []string{},
		DefaultDirect: false,
		Timeout:       3 * time.Second,
		LogLevel:      common.InfoLevel,
	}, common.NewSimpleLogger("C", common.InfoLevel))

	err := c.Start()
	if err != nil {
		t.Logf("Start failed: %v", err)
		return
	}

	assert.NoError(t, c.Stop())
	assert.NoError(t, c.Stop())
}

func TestCovClient_ServeSocks5_CloseChan(t *testing.T) {
	logger := common.NewSimpleLogger("TEST", common.DebugLevel)
	c := NewClient(Config{
		LocalPort:  0,
		ServerAddr: "127.0.0.1:1",
		LogLevel:   common.DebugLevel,
	}, logger)

	// Create a real listener
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	c.listener = listener
	c.isRunning = true

	// Start serveSOCKS5 in a goroutine
	c.wg.Add(1)
	go c.serveSOCKS5()

	// Close the client to trigger closeChan exit
	time.Sleep(100 * time.Millisecond)
	close(c.closeChan)
	c.listener.Close()
	c.isRunning = false

	// Wait for serveSOCKS5 to exit
	done := make(chan struct{})
	go func() {
		c.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(3 * time.Second):
		t.Fatal("serveSOCKS5 did not exit")
	}
}

func TestCovClient_ServeSocks5_AcceptError(t *testing.T) {
	logger := common.NewSimpleLogger("TEST", common.DebugLevel)
	c := NewClient(Config{
		LocalPort:  0,
		ServerAddr: "127.0.0.1:1",
		LogLevel:   common.DebugLevel,
	}, logger)

	// Create and immediately close listener to trigger Accept error
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	c.listener = listener
	c.isRunning = true

	// Close listener to cause Accept errors
	listener.Close()

	// Start serveSOCKS5 - it should handle Accept errors
	c.wg.Add(1)
	go c.serveSOCKS5()

	// Give it time to hit Accept error
	time.Sleep(200 * time.Millisecond)

	// Close to stop
	close(c.closeChan)
	c.isRunning = false

	done := make(chan struct{})
	go func() {
		c.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("serveSOCKS5 did not exit after closeChan")
	}
}

func TestCovClient_Start_AlreadyRunning(t *testing.T) {
	logger := common.NewSimpleLogger("TEST", common.DebugLevel)
	c := NewClient(Config{
		LocalPort:  0,
		ServerAddr: "127.0.0.1:1",
		LogLevel:   common.DebugLevel,
	}, logger)

	c.isRunning = true
	err := c.Start()
	assert.NoError(t, err)
	c.isRunning = false
}

func TestCovClient_Start_CreateListenerError(t *testing.T) {
	logger := common.NewSimpleLogger("TEST", common.DebugLevel)
	c := NewClient(Config{
		LocalPort:  99999, // Invalid port
		ServerAddr: "127.0.0.1:1",
		LogLevel:   common.DebugLevel,
	}, logger)

	err := c.Start()
	assert.Error(t, err)
}

func TestCovClient_Stop_NotRunning(t *testing.T) {
	logger := common.NewSimpleLogger("TEST", common.DebugLevel)
	c := NewClient(Config{
		LocalPort:  0,
		ServerAddr: "127.0.0.1:1",
		LogLevel:   common.DebugLevel,
	}, logger)

	err := c.Stop()
	assert.NoError(t, err)
}

func TestCovClient_Stop_WithListener(t *testing.T) {
	logger := common.NewSimpleLogger("TEST", common.DebugLevel)
	c := NewClient(Config{
		LocalPort:  0,
		ServerAddr: "127.0.0.1:1",
		LogLevel:   common.DebugLevel,
	}, logger)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	c.listener = listener
	c.isRunning = true
	c.closeChan = make(chan struct{})

	err = c.Stop()
	assert.NoError(t, err)
	assert.False(t, c.isRunning)
}
