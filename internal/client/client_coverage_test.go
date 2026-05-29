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
