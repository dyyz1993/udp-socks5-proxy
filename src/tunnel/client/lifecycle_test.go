package client

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tealife/proxy-cs3/src/tunnel"
)

// TestReceiveLoop_Exit tests receiveLoop exits on closeChan
func TestReceiveLoop_Exit(t *testing.T) {
	c, serverConn := newTestConnectorForFrag(t)
	defer serverConn.Close()
	defer c.conn.Close()

	c.isRunning = true
	c.BaseConnector.SetState(tunnel.StateConnected)

	done := make(chan struct{})
	go func() {
		c.receiveLoop()
		close(done)
	}()

	// Give it time to start and do one iteration
	time.Sleep(200 * time.Millisecond)

	// Close the conn to unblock ReadFromUDP, then close closeChan
	c.conn.Close()
	close(c.closeChan)

	select {
	case <-done:
		// Success
	case <-time.After(3 * time.Second):
		t.Fatal("receiveLoop did not exit")
	}
}

// TestReceiveLoop_ProcessData tests receiveLoop processes incoming data
func TestReceiveLoop_ProcessData(t *testing.T) {
	c, serverConn := newTestConnectorForFrag(t)
	defer serverConn.Close()
	defer c.conn.Close()

	c.isRunning = true
	c.BaseConnector.SetState(tunnel.StateConnected)
	c.BaseConnector.SetConnectionID("conn-rl")

	// Start receiveLoop
	done := make(chan struct{})
	go func() {
		c.receiveLoop()
		close(done)
	}()

	// Send a heartbeat packet from server
	time.Sleep(100 * time.Millisecond)
	hbPkt := tunnel.NewHeartbeatPacket("conn-rl", 1, 0.5)
	serverConn.WriteToUDP(hbPkt.Bytes(), c.conn.LocalAddr().(*net.UDPAddr))

	// Give it time to process
	time.Sleep(200 * time.Millisecond)

	// Close conn to unblock ReadFromUDP, then close closeChan
	c.conn.Close()
	close(c.closeChan)
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("receiveLoop did not exit")
	}
}

// TestHeartbeatLoop_Exit tests heartbeatLoop exits on closeChan
func TestHeartbeatLoop_Exit(t *testing.T) {
	c, serverConn := newTestConnectorForFrag(t)
	defer serverConn.Close()
	defer c.conn.Close()

	c.isRunning = true
	c.BaseConnector.SetState(tunnel.StateConnected)
	c.BaseConnector.SetConnectionID("conn-hb")

	// Drain heartbeat packets from UDP
	go func() {
		buf := make([]byte, 4096)
		for {
			_, _, err := serverConn.ReadFromUDP(buf)
			if err != nil {
				return
			}
		}
	}()

	done := make(chan struct{})
	go func() {
		c.heartbeatLoop()
		close(done)
	}()

	// Wait for at least one heartbeat
	time.Sleep(1500 * time.Millisecond)

	// Close should cause heartbeatLoop to exit
	close(c.closeChan)

	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("heartbeatLoop did not exit")
	}
}

// TestHeartbeatLoop_SendsPackets tests heartbeatLoop sends heartbeat packets
func TestHeartbeatLoop_SendsPackets(t *testing.T) {
	c, serverConn := newTestConnectorForFrag(t)
	defer serverConn.Close()
	defer c.conn.Close()

	c.isRunning = true
	c.BaseConnector.SetState(tunnel.StateConnected)
	c.BaseConnector.SetConnectionID("conn-hb-send")

	// Collect heartbeat packets
	receivedCount := 0
	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 4096)
		for {
			n, _, err := serverConn.ReadFromUDP(buf)
			if err != nil {
				return
			}
			pkt, err := tunnel.ParsePacket(buf[:n])
			if err == nil && pkt.Header.Type == tunnel.PacketTypeHeartbeat {
				receivedCount++
				if receivedCount >= 2 {
					return
				}
			}
		}
	}()

	go c.heartbeatLoop()

	// Wait for heartbeats
	select {
	case <-done:
		assert.GreaterOrEqual(t, receivedCount, 2, "should receive at least 2 heartbeat packets")
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for heartbeat packets")
	}

	// Close
	close(c.closeChan)
}

// TestStart_AlreadyRunning tests Start when already running
func TestStart_AlreadyRunning(t *testing.T) {
	c, serverConn := newTestConnectorForFrag(t)
	defer serverConn.Close()
	defer c.conn.Close()

	c.isRunning = true

	err := c.Start()
	assert.NoError(t, err)
}

// TestStart_ConnectFailure tests Start when Connect fails
func TestStart_ConnectFailure(t *testing.T) {
	c, err := NewClientConnector("invalid-host:99999")
	require.NoError(t, err)

	err = c.Start()
	assert.Error(t, err)
}

// TestClose_NotRunning tests Close when not running
func TestClose_NotRunning(t *testing.T) {
	c, err := NewClientConnector("127.0.0.1:1")
	require.NoError(t, err)

	err = c.Close()
	assert.NoError(t, err)
}

// TestClose_WhileRunning tests Close while running
func TestClose_WhileRunning(t *testing.T) {
	c, serverConn := newTestConnectorForFrag(t)
	defer serverConn.Close()

	c.isRunning = true
	c.BaseConnector.SetState(tunnel.StateConnected)

	// Start receiveLoop and heartbeatLoop
	go c.receiveLoop()
	go c.heartbeatLoop()

	time.Sleep(100 * time.Millisecond)

	err := c.Close()
	assert.NoError(t, err)
}
