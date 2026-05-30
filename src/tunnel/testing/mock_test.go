package testing

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tealife/proxy-cs3/src/tunnel"
)

func TestNewMockConnector(t *testing.T) {
	mc := NewMockConnector()
	if mc == nil {
		t.Fatal("NewMockConnector returned nil")
	}
}

func TestNewMockConnectorWithOptions(t *testing.T) {
	mc := NewMockConnectorWithOptions(MockConnectorOptions{})
	if mc == nil {
		t.Fatal("NewMockConnectorWithOptions returned nil")
	}
}

func TestMockConnector_Lifecycle(t *testing.T) {
	mc := NewMockConnector()

	// Connect
	if err := mc.Connect(); err != nil {
		t.Errorf("Connect: %v", err)
	}
	if !mc.WasConnectCalled() {
		t.Error("Connect not recorded")
	}

	// Start
	if err := mc.Start(); err != nil {
		t.Errorf("Start: %v", err)
	}

	// SendData
	if err := mc.SendData("s1", []byte("hello")); err != nil {
		t.Errorf("SendData: %v", err)
	}
	got := mc.GetSentData("s1")
	if string(got) != "hello" {
		t.Errorf("GetSentData = %q", got)
	}
	if len(mc.GetAllSentData()) != 1 {
		t.Errorf("GetAllSentData = %d entries", len(mc.GetAllSentData()))
	}
	if len(mc.GetSendDataCalls()) != 1 {
		t.Errorf("GetSendDataCalls = %d", len(mc.GetSendDataCalls()))
	}

	// CreateStream
	id, stream, err := mc.CreateStream("example.com:80")
	if err != nil {
		t.Errorf("CreateStream: %v", err)
	}
	if id == "" || stream == nil {
		t.Errorf("CreateStream: id=%q stream=%v", id, stream)
	}
	if len(mc.GetCreateStreamCalls()) != 1 {
		t.Errorf("GetCreateStreamCalls = %d", len(mc.GetCreateStreamCalls()))
	}

	// ProcessIncomingData
	if err := mc.ProcessIncomingData([]byte{1}); err != nil {
		t.Errorf("ProcessIncomingData: %v", err)
	}

	// Close
	if err := mc.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
	if !mc.WasCloseCalled() {
		t.Error("Close not recorded")
	}
}

func TestMockConnector_Errors(t *testing.T) {
	mc := NewMockConnectorWithOptions(MockConnectorOptions{
		ConnectError: net.UnknownNetworkError("test"),
	})
	if err := mc.Connect(); err == nil {
		t.Error("expected Connect error")
	}
}

func TestMockConnector_GetConn(t *testing.T) {
	mc := NewMockConnector()
	if mc.GetConn() == nil {
		t.Error("GetConn nil")
	}
}

func TestMockConnector_SetInitialState(t *testing.T) {
	mc := NewMockConnector()
	mc.SetInitialState(tunnel.StateConnected)
}

func TestMockTunnelStream_Basic(t *testing.T) {
	mc := NewMockConnector()
	ms := NewMockTunnelStream("s1", mc, "example.com:80")

	if ms.GetStreamID() != "s1" {
		t.Errorf("GetStreamID = %q", ms.GetStreamID())
	}
	if ms.ID() != "s1" {
		t.Errorf("ID = %q", ms.ID())
	}
	if ms.TargetAddr() != "example.com:80" {
		t.Errorf("TargetAddr = %q", ms.TargetAddr())
	}
}

func TestMockTunnelStream_DataOps(t *testing.T) {
	mc := NewMockConnector()
	ms := NewMockTunnelStream("s1", mc, "addr")

	if err := ms.PutData([]byte("test")); err != nil {
		t.Errorf("PutData: %v", err)
	}
	got, err := ms.GetData()
	if err != nil || string(got) != "test" {
		t.Errorf("GetData = %q err=%v", got, err)
	}
	if len(ms.GetPutDataCalls()) != 1 {
		t.Errorf("GetPutDataCalls = %d", len(ms.GetPutDataCalls()))
	}
	// GetBuffer returns data buffer bytes, data was consumed by GetData
	_ = ms.GetBuffer()
}

func TestMockTunnelStream_Close(t *testing.T) {
	mc := NewMockConnector()
	ms := NewMockTunnelStream("s1", mc, "addr")

	if err := ms.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
	if !ms.WasCloseCalled() || !ms.IsClosed() {
		t.Error("close state wrong")
	}
}

func TestMockTunnelStream_ServeConn(t *testing.T) {
	mc := NewMockConnector()
	ms := NewMockTunnelStream("s1", mc, "addr")

	client, _ := net.Pipe()
	defer client.Close()

	done := make(chan error, 1)
	go func() { done <- ms.ServeConn(client) }()

	time.Sleep(50 * time.Millisecond)
	client.Close()
	ms.Close()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Error("ServeConn timeout")
	}
	if len(ms.GetServeConnCalls()) != 1 {
		t.Errorf("ServeConnCalls = %d", len(ms.GetServeConnCalls()))
	}
}

func TestMockTunnelStream_WithOptions(t *testing.T) {
	mc := NewMockConnector()
	ms := NewMockTunnelStreamWithOptions(mc, MockTunnelStreamOptions{
		StreamID:   "s2",
		TargetAddr: "test.com:443",
	})
	if ms.GetStreamID() != "s2" {
		t.Errorf("GetStreamID = %q", ms.GetStreamID())
	}
}

func TestMockNetConn_ReadWrite(t *testing.T) {
	conn := NewMockNetConn()

	n, err := conn.Write([]byte("hello"))
	if err != nil || n != 5 {
		t.Fatalf("Write: n=%d err=%v", n, err)
	}
	if string(conn.GetWrittenData()) != "hello" {
		t.Errorf("WrittenData = %q", conn.GetWrittenData())
	}

	conn.AddReadData([]byte("world"))
	buf := make([]byte, 10)
	n, err = conn.Read(buf)
	if err != nil || n != 5 || string(buf[:n]) != "world" {
		t.Errorf("Read: n=%d data=%q err=%v", n, buf[:n], err)
	}
}

func TestMockNetConn_Close(t *testing.T) {
	conn := NewMockNetConn()
	conn.Close()
	if !conn.IsClosed() {
		t.Error("not closed")
	}
}

func TestMockNetConn_Deadlines(t *testing.T) {
	conn := NewMockNetConn()
	now := time.Now()
	conn.SetDeadline(now)
	conn.SetReadDeadline(now)
	conn.SetWriteDeadline(now)
}

func TestMockNetConn_Addrs(t *testing.T) {
	conn := NewMockNetConn()
	if conn.LocalAddr() == nil || conn.RemoteAddr() == nil {
		t.Error("nil addr")
	}
}

func TestMockNetConn_ClearWritten(t *testing.T) {
	conn := NewMockNetConn()
	conn.Write([]byte("x"))
	conn.ClearWrittenData()
	if len(conn.GetWrittenData()) != 0 {
		t.Error("clear failed")
	}
}

func TestMockNetConn_Options(t *testing.T) {
	conn := NewMockNetConn()
	_ = conn.GetOptions()

	conn.ApplyCondition(MockNetConnOptions{
		ReadDelay:      10 * time.Millisecond,
		WriteDelay:     10 * time.Millisecond,
		PacketLossRate: 0.5,
	})
	got := conn.GetOptions()
	if got.ReadDelay != 10*time.Millisecond {
		t.Errorf("ReadDelay = %v", got.ReadDelay)
	}
}

func TestMockNetConn_SetClosed(t *testing.T) {
	conn := NewMockNetConn()
	conn.SetClosed(true)
	if !conn.IsClosed() {
		t.Error("not closed")
	}
	conn.SetClosed(false)
	if conn.IsClosed() {
		t.Error("still closed")
	}
}

func TestErrorMockNetConn(t *testing.T) {
	conn := NewErrorMockNetConn("test")
	buf := make([]byte, 10)
	if _, err := conn.Read(buf); err == nil {
		t.Error("expected error")
	}
}

func TestMockNetConn_WithOptions(t *testing.T) {
	conn := NewMockNetConnWithOptions(MockNetConnOptions{
		ReadDelay:  5 * time.Millisecond,
		WriteDelay: 5 * time.Millisecond,
	})
	if conn == nil {
		t.Fatal("nil conn")
	}
}

func TestMockConnector_IsRunning(t *testing.T) {
	mc := NewMockConnector()
	if mc.IsRunning() {
		t.Error("should not be running initially")
	}
	mc.Start()
	if !mc.IsRunning() {
		t.Error("should be running after Start")
	}
	mc.Close()
	if mc.IsRunning() {
		t.Error("should not be running after Close")
	}
}

// === Coverage boost: MockNetConn Read/Write ===

func TestCovMockNetConn_WriteAndRead(t *testing.T) {
	conn := NewMockNetConn()
	defer conn.Close()

	// Write stores data in writeBuffer
	n, err := conn.Write([]byte("hello world"))
	require.NoError(t, err)
	require.Equal(t, 11, n)

	// Verify written data
	require.Equal(t, []byte("hello world"), conn.GetWrittenData())

	// Add data to read buffer, then read it
	conn.AddReadData([]byte("response"))
	buf := make([]byte, 100)
	conn.SetReadDeadline(time.Now().Add(time.Second))
	n, err = conn.Read(buf)
	require.NoError(t, err)
	require.Equal(t, "response", string(buf[:n]))
}

func TestCovMockNetConn_ReadTimeout(t *testing.T) {
	conn := NewMockNetConn()
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
	buf := make([]byte, 100)
	_, err := conn.Read(buf)
	require.Error(t, err)
}

func TestCovMockTunnelStream_ServeConn(t *testing.T) {
	mc := NewMockConnector()
	stream := NewMockTunnelStream("serve-test", mc, "target:80")

	clientConn, serverConn := net.Pipe()

	done := make(chan error, 1)
	go func() {
		done <- stream.ServeConn(serverConn)
	}()

	stream.PutData([]byte("from-tunnel"))
	time.Sleep(100 * time.Millisecond)

	buf := make([]byte, 100)
	clientConn.SetReadDeadline(time.Now().Add(time.Second))
	n, err := clientConn.Read(buf)
	require.NoError(t, err)
	require.Equal(t, "from-tunnel", string(buf[:n]))

	clientConn.Close()
	stream.Close()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("ServeConn did not exit")
	}
}

func TestCovMockTunnelStream_PutDataClosed(t *testing.T) {
	mc := NewMockConnector()
	stream := NewMockTunnelStream("put-closed", mc, "target:80")
	stream.Close()

	err := stream.PutData([]byte("test"))
	require.Error(t, err)
}

func TestCovMockConnector_Close(t *testing.T) {
	mc := NewMockConnector()
	err := mc.Close()
	require.NoError(t, err)
}

func TestCovMockConnector_SendData(t *testing.T) {
	mc := NewMockConnector()
	require.NoError(t, mc.Start())
	defer mc.Close()

	err := mc.SendData("stream-1", []byte("hello"))
	require.NoError(t, err)

	sent := mc.GetSentData("stream-1")
	require.Equal(t, []byte("hello"), sent)
}

func TestCovMockConnector_CreateStream(t *testing.T) {
	mc := NewMockConnector()
	require.NoError(t, mc.Start())
	defer mc.Close()

	_, stream, err := mc.CreateStream("target:80")
	require.NoError(t, err)
	require.NotNil(t, stream)
}
