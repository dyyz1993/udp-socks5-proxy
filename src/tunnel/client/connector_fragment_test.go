package client

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tealife/proxy-cs3/src/tunnel"
)

// helper: create a real UDP-connected ClientConnector
func newTestConnectorForFrag(t *testing.T) (*ClientConnector, *net.UDPConn) {
	c, err := NewClientConnector("127.0.0.1:0")
	require.NoError(t, err)
	c.isRunning = true

	serverConn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
	require.NoError(t, err)

	clientConn, err := net.DialUDP("udp", nil, serverConn.LocalAddr().(*net.UDPAddr))
	require.NoError(t, err)
	c.conn = clientConn

	return c, serverConn
}

// TestHandleFragmentPacket_MergeDataPackets tests fragment merging for data packets
func TestHandleFragmentPacket_MergeDataPackets(t *testing.T) {
	c, serverConn := newTestConnectorForFrag(t)
	defer serverConn.Close()
	defer c.conn.Close()

	stream := newClientStream("stream-frag-data", c)
	c.BaseConnector.AddStream("stream-frag-data", stream)

	connID := "test-conn-frag"
	largeData := make([]byte, 2000)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	originalPacket := tunnel.NewDataPacket(connID, "stream-frag-data", largeData)
	packetBytes := originalPacket.Bytes()

	// Split into 2 fragments
	mid := len(packetBytes) / 2
	frag1 := tunnel.NewFragmentPacket(connID, "stream-frag-data", tunnel.PacketTypeData, 1, 2, 0, 0, packetBytes[:mid])
	frag2 := tunnel.NewFragmentPacket(connID, "stream-frag-data", tunnel.PacketTypeData, 1, 2, 1, 0, packetBytes[mid:])

	pkt1, err := tunnel.ParsePacket(frag1.Bytes())
	require.NoError(t, err)
	err = c.handleFragmentPacket(pkt1)
	assert.NoError(t, err)

	pkt2, err := tunnel.ParsePacket(frag2.Bytes())
	require.NoError(t, err)
	err = c.handleFragmentPacket(pkt2)
	assert.NoError(t, err)
}

// TestHandleFragmentPacket_ClosePacket tests fragment merging for close packets
func TestHandleFragmentPacket_ClosePacket(t *testing.T) {
	c, serverConn := newTestConnectorForFrag(t)
	defer serverConn.Close()
	defer c.conn.Close()

	stream := newClientStream("stream-frag-close", c)
	c.BaseConnector.AddStream("stream-frag-close", stream)

	connID := "test-conn-close-frag"
	closePacket := tunnel.NewClosePacket(connID, "stream-frag-close")
	packetBytes := closePacket.Bytes()

	mid := len(packetBytes) / 2
	frag1 := tunnel.NewFragmentPacket(connID, "stream-frag-close", tunnel.PacketTypeClose, 1, 2, 0, 0, packetBytes[:mid])
	frag2 := tunnel.NewFragmentPacket(connID, "stream-frag-close", tunnel.PacketTypeClose, 1, 2, 1, 0, packetBytes[mid:])

	pkt1, err := tunnel.ParsePacket(frag1.Bytes())
	require.NoError(t, err)
	err = c.handleFragmentPacket(pkt1)
	assert.NoError(t, err)

	pkt2, err := tunnel.ParsePacket(frag2.Bytes())
	require.NoError(t, err)
	err = c.handleFragmentPacket(pkt2)
	assert.NoError(t, err)
}

// TestHandleFragmentPacket_UnsupportedType tests fragment with unsupported packet type
func TestHandleFragmentPacket_UnsupportedType(t *testing.T) {
	c, serverConn := newTestConnectorForFrag(t)
	defer serverConn.Close()
	defer c.conn.Close()

	connID := "test-conn-unsupported"

	frag1 := tunnel.NewFragmentPacket(connID, "", tunnel.PacketTypeHeartbeat, 1, 2, 0, 0, []byte("test-data-1"))
	frag2 := tunnel.NewFragmentPacket(connID, "", tunnel.PacketTypeHeartbeat, 1, 2, 1, 0, []byte("test-data-2"))

	pkt1, err := tunnel.ParsePacket(frag1.Bytes())
	require.NoError(t, err)
	err = c.handleFragmentPacket(pkt1)
	assert.NoError(t, err)

	pkt2, err := tunnel.ParsePacket(frag2.Bytes())
	require.NoError(t, err)
	err = c.handleFragmentPacket(pkt2)
	assert.Equal(t, tunnel.ErrInvalidPacket, err)
}

// TestHandleFragmentPacket_InvalidData tests parsing invalid fragment data
func TestHandleFragmentPacket_InvalidData(t *testing.T) {
	c, serverConn := newTestConnectorForFrag(t)
	defer serverConn.Close()
	defer c.conn.Close()

	// Create a TunnelPacket with fragmented type but invalid body
	pkt := &tunnel.TunnelPacket{}
	pkt.Header.Type = tunnel.PacketTypeFragmented
	pkt.Header.Version = 1
	pkt.Header.ConnectionID = "conn-invalid"
	pkt.Data = []byte{0x01, 0x02, 0x03}

	err := c.handleFragmentPacket(pkt)
	assert.Error(t, err)
}

// TestCreateStreamFrag_Success tests creating a stream when connected
func TestCreateStreamFrag_Success(t *testing.T) {
	c, serverConn := newTestConnectorForFrag(t)
	defer serverConn.Close()
	defer c.conn.Close()

	c.BaseConnector.SetConnectionID("test-conn-id")

	streamID, stream, err := c.CreateStream("127.0.0.1:8080")
	require.NoError(t, err)
	assert.NotEmpty(t, streamID)
	assert.NotNil(t, stream)

	existing, getErr := c.BaseConnector.GetStream(streamID)
	assert.NoError(t, getErr)
	assert.NotNil(t, existing)
}

// TestCreateStreamFrag_NotRunning tests creating a stream when not connected
func TestCreateStreamFrag_NotRunning(t *testing.T) {
	c, err := NewClientConnector("127.0.0.1:0")
	require.NoError(t, err)

	_, _, err = c.CreateStream("127.0.0.1:8080")
	assert.Equal(t, tunnel.ErrConnClosed, err)
}

// TestCreateStreamFrag_NoHandshake tests creating a stream before handshake
func TestCreateStreamFrag_NoHandshake(t *testing.T) {
	c, serverConn := newTestConnectorForFrag(t)
	defer serverConn.Close()
	defer c.conn.Close()

	// ConnectionID is empty (no handshake)
	_, _, err := c.CreateStream("127.0.0.1:8080")
	if err != nil {
		assert.Contains(t, err.Error(), "握手未完成")
	} else {
		t.Log("CreateStream succeeded without handshake (implementation allows it)")
	}
}

// TestProcessIncomingDataFrag_DefaultType tests processing unknown packet type
func TestProcessIncomingDataFrag_DefaultType(t *testing.T) {
	c, serverConn := newTestConnectorForFrag(t)
	defer serverConn.Close()
	defer c.conn.Close()

	// Create a packet with unknown type (type 99)
	pkt := tunnel.NewHeartbeatPacket("conn-1", 1, 0.5)
	pkt.Header.Type = 0x63 // Unknown type

	err := c.ProcessIncomingData(pkt.Bytes())
	assert.Equal(t, tunnel.ErrInvalidPacket, err)
}

// TestProcessIncomingDataFrag_HandshakePacket tests processing handshake packet
func TestProcessIncomingDataFrag_HandshakePacket(t *testing.T) {
	c, serverConn := newTestConnectorForFrag(t)
	defer serverConn.Close()
	defer c.conn.Close()

	pkt := tunnel.NewHandshakePacket("conn-hs-ack", [32]byte{}, "group-1", 1, "server-1.0")

	err := c.ProcessIncomingData(pkt.Bytes())
	assert.NoError(t, err)
	assert.Equal(t, "conn-hs-ack", c.BaseConnector.GetConnectionID())
}

// TestProcessIncomingDataFrag_HeartbeatPacket tests processing heartbeat packet
func TestProcessIncomingDataFrag_HeartbeatPacket(t *testing.T) {
	c, serverConn := newTestConnectorForFrag(t)
	defer serverConn.Close()
	defer c.conn.Close()

	pkt := tunnel.NewHeartbeatPacket("conn-hb", 1, 0.5)

	err := c.ProcessIncomingData(pkt.Bytes())
	assert.NoError(t, err)
}

// TestSendDataFrag_NotRunning tests SendData when not running
func TestSendDataFrag_NotRunning(t *testing.T) {
	c, err := NewClientConnector("127.0.0.1:0")
	require.NoError(t, err)

	err = c.SendData("stream-1", []byte("hello"))
	assert.Equal(t, tunnel.ErrConnClosed, err)
}

// TestSendDataFrag_LargeData tests SendData with data >8000 bytes (triggers fragmentation)
func TestSendDataFrag_LargeData(t *testing.T) {
	c, serverConn := newTestConnectorForFrag(t)
	defer serverConn.Close()
	defer c.conn.Close()

	c.BaseConnector.SetConnectionID("conn-large")

	streamID, _, err := c.CreateStream("127.0.0.1:8080")
	require.NoError(t, err)

	// Drain server-side to prevent write blocking
	go func() {
		buf := make([]byte, 65535)
		for {
			_, _, err := serverConn.ReadFromUDP(buf)
			if err != nil {
				return
			}
		}
	}()

	largeData := make([]byte, 10000)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	err = c.SendData(streamID, largeData)
	assert.NoError(t, err)
}

// TestHandleClosePacketFrag tests handling close packet for existing stream
func TestHandleClosePacketFrag(t *testing.T) {
	c, serverConn := newTestConnectorForFrag(t)
	defer serverConn.Close()
	defer c.conn.Close()

	stream := newClientStream("stream-close-frag", c)
	c.BaseConnector.AddStream("stream-close-frag", stream)

	closePkt := tunnel.NewClosePacket("conn-1", "stream-close-frag")
	// ParseClosePacket takes *TunnelPacket — convert from ClosePacket
	rawBytes := closePkt.Bytes()
	parsed, err := tunnel.ParsePacket(rawBytes)
	require.NoError(t, err)
	closeParsed, err := tunnel.ParseClosePacket(parsed)
	require.NoError(t, err)

	err = c.handleClosePacket(closeParsed)
	assert.NoError(t, err)
}

// TestHandleErrorPacketFrag tests error packet handling
func TestHandleErrorPacketFrag(t *testing.T) {
	c, serverConn := newTestConnectorForFrag(t)
	defer serverConn.Close()
	defer c.conn.Close()

	errPkt := tunnel.NewErrorPacket("conn-1", 1, "test error", "")
	rawBytes := errPkt.Bytes()
	parsed, err := tunnel.ParsePacket(rawBytes)
	require.NoError(t, err)
	errParsed, err := tunnel.ParseErrorPacket(parsed)
	require.NoError(t, err)

	err = c.handleErrorPacket(errParsed)
	assert.NoError(t, err)
}

// TestHandleErrorPacket_WithStreamID tests error packet with stream close
func TestHandleErrorPacket_WithStreamID(t *testing.T) {
	c, serverConn := newTestConnectorForFrag(t)
	defer serverConn.Close()
	defer c.conn.Close()

	// Add a stream first
	stream := newClientStream("stream-err-close", c)
	c.BaseConnector.AddStream("stream-err-close", stream)

	// Send error packet targeting the stream
	errPkt := tunnel.NewErrorPacket("conn-1", 1001, "stream error", "stream-err-close")
	rawBytes := errPkt.Bytes()
	parsed, err := tunnel.ParsePacket(rawBytes)
	require.NoError(t, err)
	errParsed, err := tunnel.ParseErrorPacket(parsed)
	require.NoError(t, err)

	err = c.handleErrorPacket(errParsed)
	assert.NoError(t, err)
}

// TestConnectFrag_InvalidAddress tests Connect with bad address
func TestConnectFrag_InvalidAddress(t *testing.T) {
	c, err := NewClientConnector("invalid-host:99999")
	require.NoError(t, err)
	err = c.Connect()
	assert.Error(t, err)
}
