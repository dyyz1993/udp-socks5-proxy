package server

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tealife/proxy-cs3/src/tunnel"
)

func newStartedServer(t *testing.T) (*Server, int) {
	t.Helper()
	s, port := newTestServer(t)
	err := s.Start()
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond) // let handleUDP start
	return s, port
}

// TestCovProcessPacket_ShortData tests processPacket with short data
func TestCovProcessPacket_ShortData(t *testing.T) {
	s, _ := newStartedServer(t)
	defer s.Stop()

	err := s.processPacket([]byte{0x01, 0x02}, &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 30001})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "数据包太短")
}

// TestCovProcessPacket_InvalidPacket tests processPacket with invalid data
func TestCovProcessPacket_InvalidPacket(t *testing.T) {
	s, _ := newStartedServer(t)
	defer s.Stop()

	invalidData := make([]byte, 10)
	invalidData[0] = 0xFF

	err := s.processPacket(invalidData, &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 30002})
	assert.Error(t, err)
}

// TestCovProcessPacket_NewClient tests processPacket creates new client connector
func TestCovProcessPacket_NewClient(t *testing.T) {
	s, _ := newStartedServer(t)
	defer s.Stop()

	hsPkt := tunnel.NewHandshakePacket("conn-new", [32]byte{}, "group-1", 1, "client-1.0")
	addr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 30003}

	err := s.processPacket(hsPkt.Bytes(), addr)
	assert.NoError(t, err)

	s.mu.Lock()
	_, ok := s.clientConn[addr.String()]
	count := len(s.clientConn)
	s.mu.Unlock()
	assert.True(t, ok)
	assert.Equal(t, 1, count)
}

// TestCovProcessPacket_ExistingClient tests processPacket with existing client
func TestCovProcessPacket_ExistingClient(t *testing.T) {
	s, _ := newStartedServer(t)
	defer s.Stop()

	hsPkt := tunnel.NewHandshakePacket("conn-exist", [32]byte{}, "group-1", 1, "client-1.0")
	addr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 30004}

	err := s.processPacket(hsPkt.Bytes(), addr)
	require.NoError(t, err)

	err = s.processPacket(hsPkt.Bytes(), addr)
	assert.NoError(t, err)

	s.mu.Lock()
	count := len(s.clientConn)
	s.mu.Unlock()
	assert.Equal(t, 1, count)
}

// TestCovProcessPacket_MultipleClients tests processPacket with multiple clients
func TestCovProcessPacket_MultipleClients(t *testing.T) {
	s, _ := newStartedServer(t)
	defer s.Stop()

	hsPkt := tunnel.NewHandshakePacket("conn-multi", [32]byte{}, "group-1", 1, "client-1.0")

	addr1 := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 30011}
	addr2 := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 30012}

	err1 := s.processPacket(hsPkt.Bytes(), addr1)
	err2 := s.processPacket(hsPkt.Bytes(), addr2)

	assert.NoError(t, err1)
	assert.NoError(t, err2)

	s.mu.Lock()
	count := len(s.clientConn)
	s.mu.Unlock()
	assert.Equal(t, 2, count)
}

// TestCovProcessPacket_DataPacket tests processPacket with data packet for existing client
func TestCovProcessPacket_DataPacket(t *testing.T) {
	s, _ := newStartedServer(t)
	defer s.Stop()

	addr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 30021}

	hsPkt := tunnel.NewHandshakePacket("conn-data", [32]byte{}, "group-1", 1, "client-1.0")
	err := s.processPacket(hsPkt.Bytes(), addr)
	require.NoError(t, err)

	hbPkt := tunnel.NewHeartbeatPacket("conn-data", 1, 0.5)
	err = s.processPacket(hbPkt.Bytes(), addr)
	assert.NoError(t, err)
}
