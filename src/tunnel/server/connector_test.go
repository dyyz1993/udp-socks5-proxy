package server

import (
	"net"
	"testing"
	"time"

	"github.com/tealife/proxy-cs3/src/tunnel"
)

func newTestSC(t *testing.T) (*ServerConnector, *net.UDPConn) {
	t.Helper()
	listenAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	listener, err := net.ListenUDP("udp", listenAddr)
	if err != nil {
		t.Skipf("cannot listen UDP: %v", err)
	}
	remoteAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:12345")
	sc := NewServerConnector(listener, remoteAddr)
	return sc, listener
}

func TestNewServerConnector(t *testing.T) {
	sc, conn := newTestSC(t)
	defer conn.Close()
	if sc == nil {
		t.Fatal("nil")
	}
}

func TestServerConnector_StartClose(t *testing.T) {
	sc, conn := newTestSC(t)
	defer conn.Close()
	if err := sc.Start(); err != nil {
		t.Errorf("Start: %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	if err := sc.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}

func TestServerConnector_CloseWithoutStart(t *testing.T) {
	sc, conn := newTestSC(t)
	defer conn.Close()
	if err := sc.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}

func TestServerConnector_SetRemoteAddr(t *testing.T) {
	sc, conn := newTestSC(t)
	defer conn.Close()
	addr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:9999")
	sc.SetRemoteAddr(addr)
}

func TestServerConnector_SendData(t *testing.T) {
	sc, conn := newTestSC(t)
	defer conn.Close()
	err := sc.SendData("s1", []byte("hello"))
	t.Logf("SendData: %v", err)
}

func TestServerConnector_SendPacket(t *testing.T) {
	sc, conn := newTestSC(t)
	defer conn.Close()
	pkt := tunnel.NewDataPacket("c1", "s1", []byte("test"))
	err := sc.SendPacket(pkt.Bytes())
	t.Logf("SendPacket: %v", err)
}

func TestServerConnector_ProcessDataPacket(t *testing.T) {
	sc, conn := newTestSC(t)
	defer conn.Close()
	sc.Start()
	defer sc.Close()
	pkt := tunnel.NewDataPacket("c1", "s1", []byte("hello"))
	err := sc.ProcessIncomingData(pkt.Bytes())
	t.Logf("ProcessData: %v", err)
}

func TestServerConnector_ProcessHandshakePacket(t *testing.T) {
	sc, conn := newTestSC(t)
	defer conn.Close()
	sc.Start()
	defer sc.Close()
	pkt := tunnel.NewHandshakePacket("c1", [32]byte{}, "grp", 1, "1.0")
	err := sc.ProcessIncomingData(pkt.Bytes())
	t.Logf("ProcessHandshake: %v", err)
}

func TestServerConnector_ProcessClosePacket(t *testing.T) {
	sc, conn := newTestSC(t)
	defer conn.Close()
	sc.Start()
	defer sc.Close()
	pkt := tunnel.NewClosePacket("c1", "s1")
	err := sc.ProcessIncomingData(pkt.Bytes())
	t.Logf("ProcessClose: %v", err)
}

func TestServerConnector_ProcessHeartbeatPacket(t *testing.T) {
	sc, conn := newTestSC(t)
	defer conn.Close()
	sc.Start()
	defer sc.Close()
	pkt := tunnel.NewHeartbeatPacket("c1", 1, 0.5)
	err := sc.ProcessIncomingData(pkt.Bytes())
	t.Logf("ProcessHeartbeat: %v", err)
}

func TestServerConnector_ProcessErrorPacket(t *testing.T) {
	sc, conn := newTestSC(t)
	defer conn.Close()
	sc.Start()
	defer sc.Close()
	pkt := tunnel.NewErrorPacket("c1", 1001, "test", "s1")
	err := sc.ProcessIncomingData(pkt.Bytes())
	t.Logf("ProcessError: %v", err)
}

func TestServerConnector_ProcessFragmentPacket(t *testing.T) {
	sc, conn := newTestSC(t)
	defer conn.Close()
	sc.Start()
	defer sc.Close()
	frag := tunnel.NewFragmentPacket("c1", "s1", tunnel.PacketTypeData, 1, 2, 0, 0, []byte("frag"))
	err := sc.ProcessIncomingData(frag.Bytes())
	t.Logf("ProcessFragment: %v", err)
}

func TestServerConnector_ProcessInvalidData(t *testing.T) {
	sc, conn := newTestSC(t)
	defer conn.Close()
	err := sc.ProcessIncomingData([]byte{0xFF, 0xFF})
	t.Logf("ProcessInvalid: %v", err)
}

func TestServerConnector_ProcessEmptyData(t *testing.T) {
	sc, conn := newTestSC(t)
	defer conn.Close()
	err := sc.ProcessIncomingData(nil)
	t.Logf("ProcessEmpty: %v", err)
}
