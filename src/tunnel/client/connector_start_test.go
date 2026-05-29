package client

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tealife/proxy-cs3/src/tunnel"
)

// TestConnector_WaitForHandshakeResponse_Success tests successful handshake
func TestConnector_WaitForHandshakeResponse_Success(t *testing.T) {
	c, serverConn := newTestConnectorForFrag(t)
	defer serverConn.Close()
	defer c.conn.Close()

	// Server goroutine: respond with handshake
	go func() {
		buf := make([]byte, 4096)
		n, addr, err := serverConn.ReadFromUDP(buf)
		if err != nil {
			return
		}
		t.Logf("Server received %d bytes from %s", n, addr)

		// Parse the handshake request
		pkt, err := tunnel.ParsePacket(buf[:n])
		if err != nil {
			t.Logf("Server parse error: %v", err)
			return
		}

		// Send handshake response back with the same connection ID
		resp := tunnel.NewHandshakePacket(pkt.Header.ConnectionID, [32]byte{}, "test-group", 1, "server-1.0")
		serverConn.WriteToUDP(resp.Bytes(), addr)
	}()

	// Client: send handshake then wait for response
	c.sendHandshake()
	err := c.waitForHandshakeResponse()
	assert.NoError(t, err)
	assert.NotEmpty(t, c.BaseConnector.GetConnectionID())
}

// TestConnector_WaitForHandshakeResponse_Timeout tests handshake timeout
func TestConnector_WaitForHandshakeResponse_Timeout(t *testing.T) {
	c, serverConn := newTestConnectorForFrag(t)
	defer serverConn.Close()
	defer c.conn.Close()

	// Server does NOT respond — client should timeout
	// But we need to send handshake first to trigger the read
	go func() {
		time.Sleep(6 * time.Second) // Wait longer than the 5s timeout
	}()

	c.sendHandshake()
	err := c.waitForHandshakeResponse()
	assert.Error(t, err) // Should timeout
}

// TestConnector_WaitForHandshakeResponse_InvalidPacket tests invalid response
func TestConnector_WaitForHandshakeResponse_InvalidPacket(t *testing.T) {
	c, serverConn := newTestConnectorForFrag(t)
	defer serverConn.Close()
	defer c.conn.Close()

	// Server responds with non-handshake packet
	go func() {
		buf := make([]byte, 4096)
		n, addr, _ := serverConn.ReadFromUDP(buf)
		if n > 0 {
			// Send a heartbeat instead of handshake
			resp := tunnel.NewHeartbeatPacket("conn-test", 1, 0.5)
			serverConn.WriteToUDP(resp.Bytes(), addr)
		}
	}()

	c.sendHandshake()
	err := c.waitForHandshakeResponse()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "非握手包")
}

// TestConnector_WaitForHandshakeResponse_InvalidData tests completely invalid data
func TestConnector_WaitForHandshakeResponse_InvalidData(t *testing.T) {
	c, serverConn := newTestConnectorForFrag(t)
	defer serverConn.Close()
	defer c.conn.Close()

	// Server responds with garbage
	go func() {
		buf := make([]byte, 4096)
		n, addr, _ := serverConn.ReadFromUDP(buf)
		if n > 0 {
			serverConn.WriteToUDP([]byte{0xFF, 0xFF, 0xFF}, addr)
		}
	}()

	c.sendHandshake()
	err := c.waitForHandshakeResponse()
	assert.Error(t, err)
}

// TestConnector_Start_WithServer tests Start with a real server
func TestConnector_Start_WithServer(t *testing.T) {
	c, serverConn := newTestConnectorForFrag(t)
	defer serverConn.Close()

	// Server responds to handshake
	go func() {
		buf := make([]byte, 4096)
		for {
			n, addr, err := serverConn.ReadFromUDP(buf)
			if err != nil {
				return
			}
			pkt, err := tunnel.ParsePacket(buf[:n])
			if err != nil {
				continue
			}
			switch pkt.Header.Type {
			case tunnel.PacketTypeHandshake:
				resp := tunnel.NewHandshakePacket(pkt.Header.ConnectionID, [32]byte{}, "group", 1, "server-1.0")
				serverConn.WriteToUDP(resp.Bytes(), addr)
			case tunnel.PacketTypeHeartbeat:
				// Respond to heartbeat
				resp := tunnel.NewHeartbeatPacket(pkt.Header.ConnectionID, 2, 0.5)
				serverConn.WriteToUDP(resp.Bytes(), addr)
			}
		}
	}()

	// Start connector
	err := c.Start()
	require.NoError(t, err)
	assert.True(t, c.isRunning)

	// Give receiveLoop and heartbeatLoop time to run
	time.Sleep(2 * time.Second)

	// Stop connector
	err = c.Close()
	assert.NoError(t, err)
	assert.False(t, c.isRunning)
}

// TestConnector_Start_AlreadyRunning tests Start when already running
func TestConnector_Start_AlreadyRunning(t *testing.T) {
	c, serverConn := newTestConnectorForFrag(t)
	defer serverConn.Close()

	go func() {
		buf := make([]byte, 4096)
		for {
			n, addr, err := serverConn.ReadFromUDP(buf)
			if err != nil {
				return
			}
			pkt, _ := tunnel.ParsePacket(buf[:n])
			if pkt != nil && pkt.Header.Type == tunnel.PacketTypeHandshake {
				resp := tunnel.NewHandshakePacket(pkt.Header.ConnectionID, [32]byte{}, "group", 1, "server-1.0")
				serverConn.WriteToUDP(resp.Bytes(), addr)
			}
		}
	}()

	err := c.Start()
	require.NoError(t, err)
	defer c.Close()

	// Start again — should return nil
	err = c.Start()
	assert.NoError(t, err)
}

// TestConnector_Close_WithoutStart tests Close without starting
func TestConnector_Close_WithoutStart(t *testing.T) {
	c, _ := newTestConnectorForFrag(t)
	err := c.Close()
	assert.NoError(t, err)
}

// TestConnector_DoubleClose tests closing twice
func TestConnector_DoubleClose(t *testing.T) {
	c, serverConn := newTestConnectorForFrag(t)
	defer serverConn.Close()

	go func() {
		buf := make([]byte, 4096)
		for {
			n, addr, err := serverConn.ReadFromUDP(buf)
			if err != nil {
				return
			}
			pkt, _ := tunnel.ParsePacket(buf[:n])
			if pkt != nil && pkt.Header.Type == tunnel.PacketTypeHandshake {
				resp := tunnel.NewHandshakePacket(pkt.Header.ConnectionID, [32]byte{}, "group", 1, "server-1.0")
				serverConn.WriteToUDP(resp.Bytes(), addr)
			}
		}
	}()

	err := c.Start()
	require.NoError(t, err)

	err = c.Close()
	assert.NoError(t, err)

	err = c.Close()
	assert.NoError(t, err)
}

// TestConnector_SendHandshake tests the sendHandshake method directly
func TestConnector_SendHandshake(t *testing.T) {
	c, serverConn := newTestConnectorForFrag(t)
	defer serverConn.Close()
	defer c.conn.Close()

	done := make(chan []byte, 1)
	go func() {
		buf := make([]byte, 4096)
		n, _, _ := serverConn.ReadFromUDP(buf)
		done <- buf[:n]
	}()

	err := c.sendHandshake()
	require.NoError(t, err)

	select {
	case data := <-done:
		pkt, err := tunnel.ParsePacket(data)
		require.NoError(t, err)
		assert.True(t, pkt.Header.Type == tunnel.PacketTypeHandshake)
		assert.Equal(t, uint8(1), pkt.Header.Version)
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for handshake")
	}
}

// TestConnector_SendHandshake_Multiple tests multiple handshake sends
func TestConnector_SendHandshake_Multiple(t *testing.T) {
	c, serverConn := newTestConnectorForFrag(t)
	defer serverConn.Close()
	defer c.conn.Close()

	// Drain server
	go func() {
		buf := make([]byte, 4096)
		for {
			_, _, err := serverConn.ReadFromUDP(buf)
			if err != nil {
				return
			}
		}
	}()

	for i := 0; i < 3; i++ {
		err := c.sendHandshake()
		assert.NoError(t, err)
	}
}

// TestConnector_Connect_ToRealServer tests Connect with real UDP
func TestConnector_Connect_ToRealServer(t *testing.T) {
	// Create a real UDP listener that responds to handshake
	serverAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	serverConn, err := net.ListenUDP("udp", serverAddr)
	require.NoError(t, err)
	defer serverConn.Close()

	actualAddr := serverConn.LocalAddr().(*net.UDPAddr)

	// Server goroutine: respond to handshake
	go func() {
		buf := make([]byte, 4096)
		for {
			n, addr, err := serverConn.ReadFromUDP(buf)
			if err != nil {
				return
			}
			pkt, err := tunnel.ParsePacket(buf[:n])
			if err != nil {
				continue
			}
			if pkt.Header.Type == tunnel.PacketTypeHandshake {
				resp := tunnel.NewHandshakePacket(pkt.Header.ConnectionID, [32]byte{}, "group", 1, "server-1.0")
				serverConn.WriteToUDP(resp.Bytes(), addr)
			}
		}
	}()

	c, err := NewClientConnector(fmt.Sprintf("127.0.0.1:%d", actualAddr.Port))
	require.NoError(t, err)

	err = c.Connect()
	require.NoError(t, err)
	assert.NotNil(t, c.conn)

	c.conn.Close()
}

// TestConnector_SendHandshake tests the sendHandshake method directly
func TestConnector_SendHandshake_Direct(t *testing.T) {
	c, serverConn := newTestConnectorForFrag(t)
	defer serverConn.Close()
	defer c.conn.Close()

	done := make(chan []byte, 1)
	go func() {
		buf := make([]byte, 4096)
		n, _, _ := serverConn.ReadFromUDP(buf)
		done <- buf[:n]
	}()

	err := c.sendHandshake()
	require.NoError(t, err)

	select {
	case data := <-done:
		pkt, err := tunnel.ParsePacket(data)
		require.NoError(t, err)
		assert.True(t, pkt.Header.Type == tunnel.PacketTypeHandshake)
	case <-time.After(3 * time.Second):
		t.Fatal("timeout")
	}
}
