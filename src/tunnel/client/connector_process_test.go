package client

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tealife/proxy-cs3/src/tunnel"
)

// helper: create a connector in "running" state for ProcessIncomingData
func newRunningConnector(t *testing.T) *ClientConnector {
	t.Helper()
	c, err := NewClientConnector("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	c.isRunning = true
	c.SetConnectionID("test-conn")
	return c
}

func TestClientConnector_ProcessHandshake(t *testing.T) {
	c := newRunningConnector(t)
	defer c.Close()

	pkt := tunnel.NewHandshakePacket("conn-1", [32]byte{}, "grp", 0, "1.0")
	err := c.ProcessIncomingData(pkt.Bytes())
	t.Logf("ProcessHandshake: %v", err)
}

func TestClientConnector_ProcessDataPacket(t *testing.T) {
	c := newRunningConnector(t)
	defer c.Close()

	pkt := tunnel.NewDataPacket("test-conn", "s1", []byte("hello world"))
	err := c.ProcessIncomingData(pkt.Bytes())
	t.Logf("ProcessData: %v", err)
}

func TestClientConnector_ProcessClosePacket(t *testing.T) {
	c := newRunningConnector(t)
	defer c.Close()

	pkt := tunnel.NewClosePacket("test-conn", "s1")
	err := c.ProcessIncomingData(pkt.Bytes())
	t.Logf("ProcessClose: %v", err)
}

func TestClientConnector_ProcessHeartbeat(t *testing.T) {
	c := newRunningConnector(t)
	defer c.Close()

	pkt := tunnel.NewHeartbeatPacket("test-conn", 1, 0.5)
	err := c.ProcessIncomingData(pkt.Bytes())
	t.Logf("ProcessHeartbeat: %v", err)
}

func TestClientConnector_ProcessError(t *testing.T) {
	c := newRunningConnector(t)
	defer c.Close()

	pkt := tunnel.NewErrorPacket("test-conn", 1001, "test error", "s1")
	err := c.ProcessIncomingData(pkt.Bytes())
	t.Logf("ProcessError: %v", err)
}

func TestClientConnector_ProcessFragment(t *testing.T) {
	c := newRunningConnector(t)
	defer c.Close()

	frag := tunnel.NewFragmentPacket("test-conn", "s1", tunnel.PacketTypeData, 1, 2, 0, 0, []byte("frag"))
	err := c.ProcessIncomingData(frag.Bytes())
	t.Logf("ProcessFragment: %v", err)
	_ = frag
}

func TestClientConnector_ProcessInvalidData(t *testing.T) {
	c := newRunningConnector(t)
	defer c.Close()

	err := c.ProcessIncomingData([]byte{0xFF, 0xFF})
	t.Logf("ProcessInvalid: %v", err)
}

func TestClientConnector_ProcessEmptyData(t *testing.T) {
	c := newRunningConnector(t)
	defer c.Close()

	err := c.ProcessIncomingData(nil)
	t.Logf("ProcessEmpty: %v", err)
}

func TestClientConnector_ProcessNotRunning(t *testing.T) {
	c, _ := NewClientConnector("127.0.0.1:0")
	defer c.Close()

	// Should return ErrConnClosed when not running
	err := c.ProcessIncomingData([]byte{0x05, 0x01})
	assert.Error(t, err)
}

func TestClientConnector_SendDataNotRunning(t *testing.T) {
	c, _ := NewClientConnector("127.0.0.1:0")
	defer c.Close()

	err := c.SendData("s1", []byte("hello"))
	assert.Error(t, err)
}

func TestClientConnector_SendDataEmptyStream(t *testing.T) {
	c := newRunningConnector(t)
	defer c.Close()

	err := c.SendData("", []byte("hello"))
	t.Logf("SendData empty stream: %v", err)
}

func TestClientConnector_SendDataNilData(t *testing.T) {
	c := newRunningConnector(t)
	defer c.Close()

	err := c.SendData("s1", nil)
	t.Logf("SendData nil data: %v", err)
}

func TestClientConnector_CreateStreamNotRunning(t *testing.T) {
	c, _ := NewClientConnector("127.0.0.1:0")
	defer c.Close()

	_, _, err := c.CreateStream("example.com:80")
	assert.Error(t, err)
}

func TestClientConnector_CreateStreamRunning(t *testing.T) {
	c := newRunningConnector(t)
	defer c.Close()

	id, stream, err := c.CreateStream("example.com:80")
	t.Logf("CreateStream: id=%q stream=%v err=%v", id, stream, err)
}

func TestClientConnector_SetConn(t *testing.T) {
	c, _ := NewClientConnector("127.0.0.1:0")
	defer c.Close()

	addr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		t.Skipf("cannot listen: %v", err)
	}
	defer conn.Close()

	c.SetConn(conn)
}

func TestClientConnector_CloseWithConn(t *testing.T) {
	c := newRunningConnector(t)

	addr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		t.Skipf("cannot listen: %v", err)
	}
	defer conn.Close()

	c.conn = conn
	c.Close()
}

func TestClientConnector_DataPacketWithStream(t *testing.T) {
	c := newRunningConnector(t)
	defer c.Close()

	// Create a stream first
	c.CreateStream("example.com:80")

	// Then process a data packet for that stream
	pkt := tunnel.NewDataPacket("test-conn", "s1", []byte("hello"))
	err := c.ProcessIncomingData(pkt.Bytes())
	t.Logf("DataPacketWithStream: %v", err)
}

func TestClientConnector_SendDataRunning(t *testing.T) {
	c := newRunningConnector(t)
	defer c.Close()

	// Set up a real UDP conn for sending
	addr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		t.Skipf("cannot listen: %v", err)
	}
	defer conn.Close()

	// Start a reader on the server side to consume packets
	go func() {
		buf := make([]byte, 65536)
		for {
			_, _, err := conn.ReadFromUDP(buf)
			if err != nil {
				return
			}
		}
	}()

	c.conn = conn
	c.BaseConnector.SetConnectionID("test-conn")

	err = c.SendData("s1", []byte("hello"))
	t.Logf("SendData running: %v", err)
}

func TestClientConnector_SendDataLarge(t *testing.T) {
	c := newRunningConnector(t)
	defer c.Close()

	addr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		t.Skipf("cannot listen: %v", err)
	}
	defer conn.Close()

	go func() {
		buf := make([]byte, 65536)
		for {
			_, _, err := conn.ReadFromUDP(buf)
			if err != nil {
				return
			}
		}
	}()

	c.conn = conn
	c.BaseConnector.SetConnectionID("test-conn")

	// Send data larger than MaxUDPPacketSize to trigger fragmentation
	largeData := make([]byte, tunnel.MaxUDPPacketSize+1000)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	err = c.SendData("s1", largeData)
	t.Logf("SendData large: %v", err)
}

func TestClientConnector_SendDataNoHandshake(t *testing.T) {
	c := newRunningConnector(t)
	defer c.Close()

	// Running but no connection ID (handshake not done)
	c.BaseConnector.SetConnectionID("")

	err := c.SendData("s1", []byte("hello"))
	assert.Error(t, err)
	t.Logf("SendData no handshake: %v", err)
}

func TestClientConnector_CloseRunning(t *testing.T) {
	c := newRunningConnector(t)

	addr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		t.Skipf("cannot listen: %v", err)
	}
	defer conn.Close()

	c.conn = conn
	c.BaseConnector.SetConnectionID("test-conn")

	err = c.Close()
	assert.NoError(t, err)
	assert.False(t, c.isRunning)
}
