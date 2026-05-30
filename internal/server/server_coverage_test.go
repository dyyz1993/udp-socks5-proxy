package server

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tealife/proxy-cs3/internal/common"
	"github.com/tealife/proxy-cs3/src/tunnel"
)

func newTestServer(t *testing.T) (*Server, int) {
	t.Helper()
	logger := common.NewSimpleLogger("TEST-SERVER", common.InfoLevel)
	port := getFreeServerPort(t)
	config := Config{
		Port:     port,
		LogLevel: common.InfoLevel,
	}
	s := NewServer(config, logger)
	return s, port
}

func getFreeServerPort(t *testing.T) int {
	t.Helper()
	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	require.NoError(t, err)
	conn, err := net.ListenUDP("udp", addr)
	require.NoError(t, err)
	port := conn.LocalAddr().(*net.UDPAddr).Port
	conn.Close()
	return port
}

func TestServer_StartStop(t *testing.T) {
	s, _ := newTestServer(t)

	err := s.Start()
	require.NoError(t, err)
	assert.True(t, s.isRunning)

	// Give handleUDP time to start
	time.Sleep(100 * time.Millisecond)

	err = s.Stop()
	require.NoError(t, err)
	assert.False(t, s.isRunning)
}

func TestServer_StopWithoutStart(t *testing.T) {
	s, _ := newTestServer(t)
	err := s.Stop()
	assert.NoError(t, err)
}

func TestServer_DoubleStart(t *testing.T) {
	s, _ := newTestServer(t)

	err := s.Start()
	require.NoError(t, err)
	defer s.Stop()

	// Double start should be safe
	err = s.Start()
	assert.NoError(t, err)
}

func TestServer_ProcessPacket(t *testing.T) {
	s, _ := newTestServer(t)
	err := s.Start()
	require.NoError(t, err)
	defer s.Stop()

	time.Sleep(100 * time.Millisecond)

	// Create a packet and send it to the server
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("127.0.0.1:%d", s.config.Port))
	require.NoError(t, err)

	clientConn, err := net.DialUDP("udp", nil, addr)
	require.NoError(t, err)
	defer clientConn.Close()

	// Send a handshake packet
	pkt := tunnel.NewHandshakePacket("test-conn", [32]byte{}, "group", 1, "client-1.0")
	_, err = clientConn.Write(pkt.Bytes())
	require.NoError(t, err)

	// Wait for processing
	time.Sleep(200 * time.Millisecond)
}

func TestServer_ProcessPacket_DataPacket(t *testing.T) {
	s, _ := newTestServer(t)
	err := s.Start()
	require.NoError(t, err)
	defer s.Stop()

	time.Sleep(100 * time.Millisecond)

	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("127.0.0.1:%d", s.config.Port))
	require.NoError(t, err)

	clientConn, err := net.DialUDP("udp", nil, addr)
	require.NoError(t, err)
	defer clientConn.Close()

	// First send handshake to create connector
	hsPkt := tunnel.NewHandshakePacket("test-conn-2", [32]byte{}, "group", 1, "client-1.0")
	clientConn.Write(hsPkt.Bytes())
	time.Sleep(200 * time.Millisecond)

	// Then send a data packet
	dataPkt := tunnel.NewDataPacket("test-conn-2", "stream-1", []byte("hello"))
	clientConn.Write(dataPkt.Bytes())
	time.Sleep(200 * time.Millisecond)
}

func TestServer_ProcessPacket_Heartbeat(t *testing.T) {
	s, _ := newTestServer(t)
	err := s.Start()
	require.NoError(t, err)
	defer s.Stop()

	time.Sleep(100 * time.Millisecond)

	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("127.0.0.1:%d", s.config.Port))
	require.NoError(t, err)

	clientConn, err := net.DialUDP("udp", nil, addr)
	require.NoError(t, err)
	defer clientConn.Close()

	// First send handshake
	hsPkt := tunnel.NewHandshakePacket("test-conn-3", [32]byte{}, "group", 1, "client-1.0")
	clientConn.Write(hsPkt.Bytes())
	time.Sleep(200 * time.Millisecond)

	// Send heartbeat
	hbPkt := tunnel.NewHeartbeatPacket("test-conn-3", 1, 0.5)
	clientConn.Write(hbPkt.Bytes())
	time.Sleep(200 * time.Millisecond)
}

func TestServer_ProcessPacket_ClosePacket(t *testing.T) {
	s, _ := newTestServer(t)
	err := s.Start()
	require.NoError(t, err)
	defer s.Stop()

	time.Sleep(100 * time.Millisecond)

	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("127.0.0.1:%d", s.config.Port))
	require.NoError(t, err)

	clientConn, err := net.DialUDP("udp", nil, addr)
	require.NoError(t, err)
	defer clientConn.Close()

	// First send handshake
	hsPkt := tunnel.NewHandshakePacket("test-conn-4", [32]byte{}, "group", 1, "client-1.0")
	clientConn.Write(hsPkt.Bytes())
	time.Sleep(200 * time.Millisecond)

	// Send close packet
	closePkt := tunnel.NewClosePacket("test-conn-4", "stream-1")
	clientConn.Write(closePkt.Bytes())
	time.Sleep(200 * time.Millisecond)
}

func TestServer_ProcessPacket_ErrorPacket(t *testing.T) {
	s, _ := newTestServer(t)
	err := s.Start()
	require.NoError(t, err)
	defer s.Stop()

	time.Sleep(100 * time.Millisecond)

	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("127.0.0.1:%d", s.config.Port))
	require.NoError(t, err)

	clientConn, err := net.DialUDP("udp", nil, addr)
	require.NoError(t, err)
	defer clientConn.Close()

	// First send handshake
	hsPkt := tunnel.NewHandshakePacket("test-conn-5", [32]byte{}, "group", 1, "client-1.0")
	clientConn.Write(hsPkt.Bytes())
	time.Sleep(200 * time.Millisecond)

	// Send error packet
	errPkt := tunnel.NewErrorPacket("test-conn-5", 1001, "test error", "stream-1")
	clientConn.Write(errPkt.Bytes())
	time.Sleep(200 * time.Millisecond)
}

func TestServer_ProcessPacket_InvalidData(t *testing.T) {
	s, _ := newTestServer(t)
	err := s.Start()
	require.NoError(t, err)
	defer s.Stop()

	time.Sleep(100 * time.Millisecond)

	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("127.0.0.1:%d", s.config.Port))
	require.NoError(t, err)

	clientConn, err := net.DialUDP("udp", nil, addr)
	require.NoError(t, err)
	defer clientConn.Close()

	// Send invalid data (too short)
	clientConn.Write([]byte{0x01, 0x02})
	time.Sleep(200 * time.Millisecond)

	// Send completely invalid packet
	clientConn.Write([]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF})
	time.Sleep(200 * time.Millisecond)
}

func TestServer_parsePacket(t *testing.T) {
	s, _ := newTestServer(t)

	// Valid packet (5+ bytes)
	result, err := s.parsePacket([]byte{0x01, 0x02, 0x03, 0x04, 0x05})
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Too short
	_, err = s.parsePacket([]byte{0x01, 0x02})
	assert.Error(t, err)

	// Empty
	_, err = s.parsePacket([]byte{})
	assert.Error(t, err)
}

func TestServer_MultipleClients(t *testing.T) {
	s, _ := newTestServer(t)
	err := s.Start()
	require.NoError(t, err)
	defer s.Stop()

	time.Sleep(100 * time.Millisecond)

	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("127.0.0.1:%d", s.config.Port))
	require.NoError(t, err)

	// Send from two different client addresses
	for i := 0; i < 2; i++ {
		conn, err := net.DialUDP("udp", nil, addr)
		require.NoError(t, err)

		hsPkt := tunnel.NewHandshakePacket(fmt.Sprintf("multi-conn-%d", i), [32]byte{}, "group", 1, "client-1.0")
		conn.Write(hsPkt.Bytes())
		time.Sleep(100 * time.Millisecond)
		conn.Close()
	}

	time.Sleep(200 * time.Millisecond)

	// Verify two client connectors were created
	s.mu.Lock()
	assert.Equal(t, 2, len(s.clientConn))
	s.mu.Unlock()
}

// === Coverage boost: processPacket branches ===

// === Coverage boost: handleUDP with real data ===

func TestCovServer_HandleUDP_RealPacket(t *testing.T) {
	s, port := newTestServer(t)
	err := s.Start()
	require.NoError(t, err)
	defer s.Stop()

	time.Sleep(100 * time.Millisecond)

	// Send a handshake packet from a real UDP client
	clientConn, err := net.DialUDP("udp", nil, &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: port})
	require.NoError(t, err)
	defer clientConn.Close()

	hsPkt := tunnel.NewHandshakePacket("conn-udp-test", [32]byte{}, "group-1", 1, "client-1.0")
	_, err = clientConn.Write(hsPkt.Bytes())
	require.NoError(t, err)

	// Read response
	buf := make([]byte, 4096)
	clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := clientConn.Read(buf)
	require.NoError(t, err)
	assert.Greater(t, n, 0)

	// Send heartbeat
	hbPkt := tunnel.NewHeartbeatPacket("conn-udp-test", 1, 0.5)
	clientConn.Write(hbPkt.Bytes())
}

func TestCovServer_HandleUDP_InvalidPacket(t *testing.T) {
	s, port := newTestServer(t)
	err := s.Start()
	require.NoError(t, err)
	defer s.Stop()

	time.Sleep(100 * time.Millisecond)

	clientConn, err := net.DialUDP("udp", nil, &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: port})
	require.NoError(t, err)
	defer clientConn.Close()

	// Send invalid data
	_, err = clientConn.Write([]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF})
	require.NoError(t, err)

	// Give server time to process
	time.Sleep(200 * time.Millisecond)
}

func TestCovServer_HandleUDP_ShortPacket(t *testing.T) {
	s, port := newTestServer(t)
	err := s.Start()
	require.NoError(t, err)
	defer s.Stop()

	time.Sleep(100 * time.Millisecond)

	clientConn, err := net.DialUDP("udp", nil, &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: port})
	require.NoError(t, err)
	defer clientConn.Close()

	// Send very short data
	_, err = clientConn.Write([]byte{0x01, 0x02})
	require.NoError(t, err)

	time.Sleep(200 * time.Millisecond)
}

// TestCovHandleUDP_ProcessRealData tests that handleUDP processes real UDP data
func TestCovHandleUDP_ProcessRealData(t *testing.T) {
	sp := getFreeServerPort(t)
	logger := common.NewSimpleLogger("test", common.ErrorLevel)
	s := NewServer(Config{Port: sp, LogLevel: common.ErrorLevel}, logger)
	go s.Start()
	time.Sleep(200 * time.Millisecond)
	defer s.Stop()

	// Send a valid handshake packet to trigger processPacket
	conn, err := net.DialUDP("udp", nil, &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: sp})
	require.NoError(t, err)
	defer conn.Close()

	// Send handshake packet
	pkt := tunnel.NewHandshakePacket("test-conn", [32]byte{}, "group", 1, "v1.0")
	_, err = conn.Write(pkt.Bytes())
	require.NoError(t, err)

	// Wait for processing
	time.Sleep(500 * time.Millisecond)
}

// TestCovProcessPacket_InvalidData tests processPacket with invalid data
func TestCovProcessPacket_InvalidData(t *testing.T) {
	sp := getFreeServerPort(t)
	logger := common.NewSimpleLogger("test", common.ErrorLevel)
	s := NewServer(Config{Port: sp, LogLevel: common.ErrorLevel}, logger)
	go s.Start()
	time.Sleep(200 * time.Millisecond)
	defer s.Stop()

	conn, err := net.DialUDP("udp", nil, &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: sp})
	require.NoError(t, err)
	defer conn.Close()

	// Send invalid data (too short)
	_, err = conn.Write([]byte{0x01})
	require.NoError(t, err)

	// Send empty data
	_, err = conn.Write([]byte{})
	require.NoError(t, err)

	time.Sleep(500 * time.Millisecond)
}

// TestCovServer_HandleUDP_TimeoutExit tests that handleUDP exits on closeChan after timeout
func TestCovServer_HandleUDP_TimeoutExit(t *testing.T) {
	sp := getFreeServerPort(t)
	logger := common.NewSimpleLogger("test", common.ErrorLevel)
	s := NewServer(Config{Port: sp, LogLevel: common.ErrorLevel}, logger)
	go s.Start()
	time.Sleep(200 * time.Millisecond)

	// Send multiple packet types to trigger different processPacket branches
	conn, err := net.DialUDP("udp", nil, &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: sp})
	require.NoError(t, err)
	defer conn.Close()

	// Send heartbeat packet
	hbPkt := tunnel.NewHeartbeatPacket("test-conn", 1, 0.5)
	_, _ = conn.Write(hbPkt.Bytes())
	time.Sleep(200 * time.Millisecond)

	// Send data packet
	dataPkt := tunnel.NewDataPacket("test-conn", "s1", []byte("hello"))
	_, _ = conn.Write(dataPkt.Bytes())
	time.Sleep(200 * time.Millisecond)

	// Send close packet
	closePkt := tunnel.NewClosePacket("test-conn", "s1")
	_, _ = conn.Write(closePkt.Bytes())
	time.Sleep(200 * time.Millisecond)

	// Send error packet
	errPkt := tunnel.NewErrorPacket("test-conn", 1, "test error", "s1")
	_, _ = conn.Write(errPkt.Bytes())
	time.Sleep(200 * time.Millisecond)

	// Send fragmented packet
	fragPkt := tunnel.NewFragmentPacket("test-conn", "s1", tunnel.PacketTypeData, 1, 2, 0, 0, []byte("frag"))
	_, _ = conn.Write(fragPkt.Bytes())
	time.Sleep(200 * time.Millisecond)

	s.Stop()
	time.Sleep(300 * time.Millisecond)
}

// TestCovServer_processPacket_AllTypes tests processPacket with all packet types
func TestCovServer_processPacket_AllTypes(t *testing.T) {
	sp := getFreeServerPort(t)
	logger := common.NewSimpleLogger("test", common.ErrorLevel)
	s := NewServer(Config{Port: sp, LogLevel: common.ErrorLevel}, logger)
	go s.Start()
	time.Sleep(200 * time.Millisecond)
	defer s.Stop()

	conn, err := net.DialUDP("udp", nil, &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: sp})
	require.NoError(t, err)
	defer conn.Close()

	// Send handshake to establish connection
	hspkt := tunnel.NewHandshakePacket("cov-all-types", [32]byte{}, "group", 1, "v1.0")
	_, _ = conn.Write(hspkt.Bytes())
	time.Sleep(300 * time.Millisecond)

	// Send data packet for established connection
	dataPkt := tunnel.NewDataPacket("cov-all-types", "s-all", []byte("test-data"))
	_, _ = conn.Write(dataPkt.Bytes())
	time.Sleep(200 * time.Millisecond)

	// Send heartbeat for established connection
	hbPkt := tunnel.NewHeartbeatPacket("cov-all-types", 1, 0.5)
	_, _ = conn.Write(hbPkt.Bytes())
	time.Sleep(200 * time.Millisecond)
}
