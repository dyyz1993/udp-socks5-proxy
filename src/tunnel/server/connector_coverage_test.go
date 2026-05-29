package server

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tealife/proxy-cs3/src/tunnel"
)

// TestServerHandleFragmentPacket_MergeData tests server-side fragment merging for data packets
func TestServerHandleFragmentPacket_MergeData(t *testing.T) {
	sc, conn := newTestSC(t)
	defer conn.Close()
	sc.Start()
	defer sc.Close()

	// Add a stream to receive data
	stream := newServerStream("stream-frag-srv", sc)
	sc.BaseConnector.AddStream("stream-frag-srv", stream)

	connID := "conn-frag-srv"
	largeData := make([]byte, 2000)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	originalPacket := tunnel.NewDataPacket(connID, "stream-frag-srv", largeData)
	packetBytes := originalPacket.Bytes()

	mid := len(packetBytes) / 2
	frag1 := tunnel.NewFragmentPacket(connID, "stream-frag-srv", tunnel.PacketTypeData, 1, 2, 0, 0, packetBytes[:mid])
	frag2 := tunnel.NewFragmentPacket(connID, "stream-frag-srv", tunnel.PacketTypeData, 1, 2, 1, 0, packetBytes[mid:])

	// Send fragments via ProcessIncomingData
	err := sc.ProcessIncomingData(frag1.Bytes())
	assert.NoError(t, err)

	err = sc.ProcessIncomingData(frag2.Bytes())
	assert.NoError(t, err)
}

// TestServerHandleFragmentPacket_Close tests server-side fragment merging for close packets
func TestServerHandleFragmentPacket_Close(t *testing.T) {
	sc, conn := newTestSC(t)
	defer conn.Close()
	sc.Start()
	defer sc.Close()

	stream := newServerStream("stream-close-frag-srv", sc)
	sc.BaseConnector.AddStream("stream-close-frag-srv", stream)

	connID := "conn-close-frag-srv"
	closePacket := tunnel.NewClosePacket(connID, "stream-close-frag-srv")
	packetBytes := closePacket.Bytes()

	mid := len(packetBytes) / 2
	frag1 := tunnel.NewFragmentPacket(connID, "stream-close-frag-srv", tunnel.PacketTypeClose, 1, 2, 0, 0, packetBytes[:mid])
	frag2 := tunnel.NewFragmentPacket(connID, "stream-close-frag-srv", tunnel.PacketTypeClose, 1, 2, 1, 0, packetBytes[mid:])

	err := sc.ProcessIncomingData(frag1.Bytes())
	assert.NoError(t, err)

	err = sc.ProcessIncomingData(frag2.Bytes())
	assert.NoError(t, err)
}

// TestServerHandleFragmentPacket_UnsupportedType tests server fragment with unsupported type
func TestServerHandleFragmentPacket_UnsupportedType(t *testing.T) {
	sc, conn := newTestSC(t)
	defer conn.Close()
	sc.Start()
	defer sc.Close()

	connID := "conn-unsupported-srv"

	frag1 := tunnel.NewFragmentPacket(connID, "", tunnel.PacketTypeHeartbeat, 1, 2, 0, 0, []byte("data-1"))
	frag2 := tunnel.NewFragmentPacket(connID, "", tunnel.PacketTypeHeartbeat, 1, 2, 1, 0, []byte("data-2"))

	err := sc.ProcessIncomingData(frag1.Bytes())
	assert.NoError(t, err)

	err = sc.ProcessIncomingData(frag2.Bytes())
	// Should result in ErrInvalidPacket since heartbeat can't be fragmented
	assert.Error(t, err)
}

// TestServerSendData_LargeData tests server SendData with fragmentation
func TestServerSendData_LargeData(t *testing.T) {
	sc, conn := newTestSC(t)
	defer conn.Close()
	sc.Start()
	defer sc.Close()

	// Drain server-side UDP to prevent blocking
	go func() {
		buf := make([]byte, 65535)
		for {
			_, _, err := conn.ReadFromUDP(buf)
			if err != nil {
				return
			}
		}
	}()

	sc.BaseConnector.SetConnectionID("conn-send-large")

	// Send large data that triggers fragmentation
	largeData := make([]byte, 10000)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	err := sc.SendData("stream-large", largeData)
	assert.NoError(t, err)
}

// TestServerSendData_SmallData tests server SendData without fragmentation
func TestServerSendData_SmallData(t *testing.T) {
	sc, conn := newTestSC(t)
	defer conn.Close()
	sc.Start()
	defer sc.Close()

	// Drain server-side UDP
	go func() {
		buf := make([]byte, 65535)
		for {
			_, _, err := conn.ReadFromUDP(buf)
			if err != nil {
				return
			}
		}
	}()

	sc.BaseConnector.SetConnectionID("conn-send-small")

	err := sc.SendData("stream-small", []byte("hello world"))
	assert.NoError(t, err)
}

// TestServerSendPacket tests SendPacket directly
func TestServerSendPacket_Direct(t *testing.T) {
	sc, conn := newTestSC(t)
	defer conn.Close()

	// Drain UDP
	go func() {
		buf := make([]byte, 65535)
		for {
			_, _, err := conn.ReadFromUDP(buf)
			if err != nil {
				return
			}
		}
	}()

	pkt := tunnel.NewHeartbeatPacket("conn-1", 1, 0.5)
	err := sc.SendPacket(pkt.Bytes())
	assert.NoError(t, err)
}

// TestServerProcessIncomingData_NotRunning tests processing when stopped
func TestServerProcessIncomingData_NotRunning(t *testing.T) {
	sc, conn := newTestSC(t)
	defer conn.Close()
	// sc.isRunning = false

	pkt := tunnel.NewHeartbeatPacket("conn-1", 1, 0.5)
	err := sc.ProcessIncomingData(pkt.Bytes())
	assert.Equal(t, tunnel.ErrConnClosed, err)
}

// TestServerSetRemoteAddr tests SetRemoteAddr
func TestServerSetRemoteAddr(t *testing.T) {
	sc, conn := newTestSC(t)
	defer conn.Close()

	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:54321")
	require.NoError(t, err)
	sc.SetRemoteAddr(addr)
	assert.Equal(t, addr, sc.remoteAddr)
}

// TestServerConnector_HandleHandshake tests handshake handling
func TestServerConnector_HandleHandshake(t *testing.T) {
	sc, conn := newTestSC(t)
	defer conn.Close()
	sc.Start()
	defer sc.Close()

	// Drain UDP responses
	go func() {
		buf := make([]byte, 65535)
		for {
			_, _, err := conn.ReadFromUDP(buf)
			if err != nil {
				return
			}
		}
	}()

	pkt := tunnel.NewHandshakePacket("conn-hs", [32]byte{}, "group-1", 1, "client-1.0")
	err := sc.ProcessIncomingData(pkt.Bytes())
	assert.NoError(t, err)

	// Verify connection ID was set
	assert.NotEmpty(t, sc.BaseConnector.GetConnectionID())
}

// TestServerConnector_HandleHeartbeat tests heartbeat handling
func TestServerConnector_HandleHeartbeat(t *testing.T) {
	sc, conn := newTestSC(t)
	defer conn.Close()
	sc.Start()
	defer sc.Close()

	pkt := tunnel.NewHeartbeatPacket("conn-hb", 1, 0.5)
	err := sc.ProcessIncomingData(pkt.Bytes())
	assert.NoError(t, err)
}

// TestServerConnector_HandleError tests error packet handling
func TestServerConnector_HandleError(t *testing.T) {
	sc, conn := newTestSC(t)
	defer conn.Close()
	sc.Start()
	defer sc.Close()

	pkt := tunnel.NewErrorPacket("conn-err", 1001, "test error", "stream-1")
	err := sc.ProcessIncomingData(pkt.Bytes())
	assert.NoError(t, err)
}

// TestServerConnector_HandleCloseStream tests close packet with existing stream
func TestServerConnector_HandleCloseStream(t *testing.T) {
	sc, conn := newTestSC(t)
	defer conn.Close()
	sc.Start()
	defer sc.Close()

	// Add a stream first
	stream := newServerStream("stream-close-srv", sc)
	sc.BaseConnector.AddStream("stream-close-srv", stream)

	pkt := tunnel.NewClosePacket("conn-close", "stream-close-srv")
	err := sc.ProcessIncomingData(pkt.Bytes())
	assert.NoError(t, err)
}

// TestServerConnector_DoubleClose tests closing twice
func TestServerConnector_DoubleClose(t *testing.T) {
	sc, conn := newTestSC(t)
	defer conn.Close()
	sc.Start()

	err := sc.Close()
	assert.NoError(t, err)

	// Close again — should be safe
	err = sc.Close()
	assert.NoError(t, err)
}

// TestServerSendData_NoConn tests SendData with nil connection
func TestServerSendData_NoConn(t *testing.T) {
	// Create a connector with a closed listener
	listenAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	listener, _ := net.ListenUDP("udp", listenAddr)
	remoteAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:12345")
	sc := NewServerConnector(listener, remoteAddr)
	listener.Close() // Close the connection

	sc.BaseConnector.SetConnectionID("conn-nil")
	err := sc.SendData("stream-1", []byte("hello"))
	// Should error because conn is closed
	assert.Error(t, err)
}

// === Coverage boost: ProcessIncomingData branches ===

func TestCovServer_ProcessIncomingData_ClosePacket(t *testing.T) {
	sc, conn := newTestSC(t)
	defer conn.Close()
	sc.Start()
	defer sc.Close()

	closePkt := tunnel.NewClosePacket("conn-close", "")
	err := sc.ProcessIncomingData(closePkt.Bytes())
	assert.NoError(t, err)
}

func TestCovServer_ProcessIncomingData_HeartbeatPacket(t *testing.T) {
	sc, conn := newTestSC(t)
	defer conn.Close()
	sc.Start()
	defer sc.Close()

	hbPkt := tunnel.NewHeartbeatPacket("conn-hb", 1, 0.5)
	err := sc.ProcessIncomingData(hbPkt.Bytes())
	assert.NoError(t, err)
}

func TestCovServer_ProcessIncomingData_ErrorPacket(t *testing.T) {
	sc, conn := newTestSC(t)
	defer conn.Close()
	sc.Start()
	defer sc.Close()

	errPkt := tunnel.NewErrorPacket("conn-err", 1001, "test error", "")
	err := sc.ProcessIncomingData(errPkt.Bytes())
	assert.NoError(t, err)
}

func TestCovServer_ProcessIncomingData_UnknownType(t *testing.T) {
	sc, conn := newTestSC(t)
	defer conn.Close()
	sc.Start()
	defer sc.Close()

	// Create a packet with unknown type
	pkt := tunnel.NewHeartbeatPacket("conn-unk", 1, 0.5)
	pkt.Header.Type = 0xFF // Unknown type
	err := sc.ProcessIncomingData(pkt.Bytes())
	assert.Equal(t, tunnel.ErrInvalidPacket, err)
}

func TestCovServer_ProcessIncomingData_HandshakePacket(t *testing.T) {
	sc, conn := newTestSC(t)
	defer conn.Close()
	sc.Start()
	defer sc.Close()

	hsPkt := tunnel.NewHandshakePacket("conn-hs", [32]byte{}, "group-1", 1, "client-1.0")
	err := sc.ProcessIncomingData(hsPkt.Bytes())
	assert.NoError(t, err)
	// handleHandshakePacket generates a UUID, not our input ID
	assert.NotEmpty(t, sc.GetConnectionID())
}

func TestCovServer_ProcessIncomingData_DataPacket(t *testing.T) {
	sc, conn := newTestSC(t)
	defer conn.Close()
	sc.Start()
	defer sc.Close()

	// Add a stream first
	stream := newServerStream("stream-data", sc)
	sc.AddStream("stream-data", stream)

	dataPkt := tunnel.NewDataPacket("conn-data", "stream-data", []byte("hello"))
	err := sc.ProcessIncomingData(dataPkt.Bytes())
	assert.NoError(t, err)
}

func TestCovServer_ProcessIncomingData_NotRunning(t *testing.T) {
	sc, conn := newTestSC(t)
	defer conn.Close()

	err := sc.ProcessIncomingData([]byte{0x01})
	assert.Equal(t, tunnel.ErrConnClosed, err)
}

func TestCovServer_ProcessIncomingData_InvalidData(t *testing.T) {
	sc, conn := newTestSC(t)
	defer conn.Close()
	sc.Start()
	defer sc.Close()

	err := sc.ProcessIncomingData([]byte{0xFF, 0xFF})
	assert.Error(t, err)
}

func TestCovServer_HandleHandshakePacket_Duplicate(t *testing.T) {
	sc, conn := newTestSC(t)
	defer conn.Close()
	sc.Start()
	defer sc.Close()

	hsPkt := tunnel.NewHandshakePacket("conn-dup", [32]byte{}, "group-1", 1, "client-1.0")

	// First handshake
	err := sc.ProcessIncomingData(hsPkt.Bytes())
	assert.NoError(t, err)

	// Second handshake (duplicate)
	hsPkt2 := tunnel.NewHandshakePacket("conn-dup2", [32]byte{}, "group-1", 1, "client-1.0")
	err = sc.ProcessIncomingData(hsPkt2.Bytes())
	assert.NoError(t, err)
}

func TestCovServer_HandleClosePacket_WithStream(t *testing.T) {
	sc, conn := newTestSC(t)
	defer conn.Close()
	sc.Start()
	defer sc.Close()

	// Add a stream
	stream := newServerStream("stream-close", sc)
	sc.AddStream("stream-close", stream)

	closePkt := tunnel.NewClosePacket("conn-close", "")
	err := sc.ProcessIncomingData(closePkt.Bytes())
	assert.NoError(t, err)
}
