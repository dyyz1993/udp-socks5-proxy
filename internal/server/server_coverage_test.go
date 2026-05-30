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
	logger := common.NewSimpleLogger("TEST-SERVER", common.ErrorLevel)
	port := getFreeServerPort(t)
	config := Config{
		Port:     port,
		LogLevel: common.ErrorLevel,
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
	require.NoError(t, s.Start())
	time.Sleep(30 * time.Millisecond)
	require.NoError(t, s.Stop())
	assert.False(t, s.isRunning)
}

func TestServer_StopWithoutStart(t *testing.T) {
	s, _ := newTestServer(t)
	assert.NoError(t, s.Stop())
}

func TestServer_DoubleStart(t *testing.T) {
	s, _ := newTestServer(t)
	require.NoError(t, s.Start())
	defer s.Stop()
	assert.NoError(t, s.Start())
}

// TestServer_AllPacketTypes tests all packet types in a single server instance
// to avoid the overhead of starting/stopping multiple UDP servers under -race.
func TestServer_AllPacketTypes(t *testing.T) {
	s, port := newTestServer(t)
	require.NoError(t, s.Start())
	defer s.Stop()
	time.Sleep(30 * time.Millisecond)

	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("127.0.0.1:%d", port))
	require.NoError(t, err)

	conn, err := net.DialUDP("udp", nil, addr)
	require.NoError(t, err)
	defer conn.Close()

	// 1. Send handshake to establish connection
	hsPkt := tunnel.NewHandshakePacket("conn-1", [32]byte{}, "group", 1, "client-1.0")
	_, _ = conn.Write(hsPkt.Bytes())
	time.Sleep(50 * time.Millisecond)

	// 2. Send heartbeat
	hbPkt := tunnel.NewHeartbeatPacket("conn-1", 1, 0.5)
	_, _ = conn.Write(hbPkt.Bytes())
	time.Sleep(50 * time.Millisecond)

	// 3. Send data packet
	dataPkt := tunnel.NewDataPacket("conn-1", "stream-1", []byte("hello"))
	_, _ = conn.Write(dataPkt.Bytes())
	time.Sleep(50 * time.Millisecond)

	// 4. Send close packet
	closePkt := tunnel.NewClosePacket("conn-1", "stream-1")
	_, _ = conn.Write(closePkt.Bytes())
	time.Sleep(50 * time.Millisecond)

	// 5. Send error packet
	errPkt := tunnel.NewErrorPacket("conn-1", 1001, "test error", "stream-1")
	_, _ = conn.Write(errPkt.Bytes())
	time.Sleep(50 * time.Millisecond)

	// 6. Send fragment packet
	fragPkt := tunnel.NewFragmentPacket("conn-1", "stream-1", tunnel.PacketTypeData, 1, 2, 0, 0, []byte("frag"))
	_, _ = conn.Write(fragPkt.Bytes())
	time.Sleep(50 * time.Millisecond)

	// 7. Send invalid data (too short)
	_, _ = conn.Write([]byte{0x01, 0x02})
	time.Sleep(50 * time.Millisecond)

	// 8. Send completely invalid packet
	_, _ = conn.Write([]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF})
	time.Sleep(50 * time.Millisecond)

	// Verify connector was created
	s.mu.Lock()
	assert.GreaterOrEqual(t, len(s.clientConn), 1)
	s.mu.Unlock()
}

// TestServer_MultipleClients tests multiple clients on a single server
func TestServer_MultipleClients(t *testing.T) {
	s, port := newTestServer(t)
	require.NoError(t, s.Start())
	defer s.Stop()
	time.Sleep(30 * time.Millisecond)

	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("127.0.0.1:%d", port))
	require.NoError(t, err)

	for i := 0; i < 2; i++ {
		conn, err := net.DialUDP("udp", nil, addr)
		require.NoError(t, err)
		hsPkt := tunnel.NewHandshakePacket(fmt.Sprintf("multi-%d", i), [32]byte{}, "group", 1, "client-1.0")
		conn.Write(hsPkt.Bytes())
		time.Sleep(30 * time.Millisecond)
		conn.Close()
	}
	time.Sleep(50 * time.Millisecond)

	s.mu.Lock()
	assert.Equal(t, 2, len(s.clientConn))
	s.mu.Unlock()
}

func TestServer_parsePacket(t *testing.T) {
	s, _ := newTestServer(t)
	result, err := s.parsePacket([]byte{0x01, 0x02, 0x03, 0x04, 0x05})
	assert.NoError(t, err)
	assert.NotNil(t, result)
	_, err = s.parsePacket([]byte{0x01, 0x02})
	assert.Error(t, err)
	_, err = s.parsePacket([]byte{})
	assert.Error(t, err)
}

// TestServer_HandleUDP_Response reads a response from the server
func TestServer_HandleUDP_Response(t *testing.T) {
	s, port := newTestServer(t)
	require.NoError(t, s.Start())
	defer s.Stop()
	time.Sleep(30 * time.Millisecond)

	conn, err := net.DialUDP("udp", nil, &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: port})
	require.NoError(t, err)
	defer conn.Close()

	hsPkt := tunnel.NewHandshakePacket("conn-resp", [32]byte{}, "group-1", 1, "client-1.0")
	_, _ = conn.Write(hsPkt.Bytes())

	// Try to read response
	buf := make([]byte, 4096)
	conn.SetReadDeadline(time.Now().Add(1 * time.Second))
	n, _ := conn.Read(buf)
	// Response may or may not come, just ensure no panic
	_ = n
}
