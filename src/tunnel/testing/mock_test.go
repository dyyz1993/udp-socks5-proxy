package testing

import (
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tealife/proxy-cs3/src/tunnel"
)

// ============================================================
// MockConnector tests
// ============================================================

func TestNewMockConnector(t *testing.T) {
	mc := NewMockConnector()
	require.NotNil(t, mc)
}

func TestNewMockConnectorWithOptions(t *testing.T) {
	mc := NewMockConnectorWithOptions(MockConnectorOptions{})
	require.NotNil(t, mc)
}

func TestMockConnector_Lifecycle(t *testing.T) {
	mc := NewMockConnector()

	require.NoError(t, mc.Connect())
	require.True(t, mc.WasConnectCalled())

	require.NoError(t, mc.Start())

	require.NoError(t, mc.SendData("s1", []byte("hello")))
	require.Equal(t, []byte("hello"), mc.GetSentData("s1"))
	require.Len(t, mc.GetAllSentData(), 1)
	require.Len(t, mc.GetSendDataCalls(), 1)

	id, stream, err := mc.CreateStream("example.com:80")
	require.NoError(t, err)
	require.NotEmpty(t, id)
	require.NotNil(t, stream)
	require.Len(t, mc.GetCreateStreamCalls(), 1)

	require.NoError(t, mc.ProcessIncomingData([]byte{1}))

	require.NoError(t, mc.Close())
	require.True(t, mc.WasCloseCalled())
}

func TestMockConnector_Errors(t *testing.T) {
	mc := NewMockConnectorWithOptions(MockConnectorOptions{
		ConnectError: net.UnknownNetworkError("test"),
	})
	require.Error(t, mc.Connect())
}

func TestMockConnector_GetConn(t *testing.T) {
	mc := NewMockConnector()
	require.NotNil(t, mc.GetConn())
}

func TestMockConnector_SetInitialState(t *testing.T) {
	mc := NewMockConnector()
	mc.SetInitialState(tunnel.StateConnected)
	require.True(t, mc.IsRunning())

	mc.SetInitialState(tunnel.StateClosed)
	require.False(t, mc.IsRunning())
}

func TestMockConnector_IsRunning(t *testing.T) {
	mc := NewMockConnector()
	require.False(t, mc.IsRunning())
	mc.Start()
	require.True(t, mc.IsRunning())
	mc.Close()
	require.False(t, mc.IsRunning())
}

// --- Connector: Connect branches ---

func TestCovMockConnector_ConnectFailDefault(t *testing.T) {
	mc := NewMockConnectorWithOptions(MockConnectorOptions{
		ShouldConnectSucceed: false,
	})
	err := mc.Connect()
	require.Error(t, err)
	require.Contains(t, err.Error(), "模拟连接失败")
}

func TestCovMockConnector_ConnectCallback(t *testing.T) {
	mc := NewMockConnector()
	called := false
	mc.OnConnect = func() error {
		called = true
		return errors.New("callback-err")
	}
	err := mc.Connect()
	require.True(t, called)
	require.EqualError(t, err, "callback-err")
}

// --- Connector: Close branches ---

func TestCovMockConnector_CloseFailDefault(t *testing.T) {
	mc := NewMockConnectorWithOptions(MockConnectorOptions{
		ShouldCloseSucceed: false,
	})
	err := mc.Close()
	require.Error(t, err)
	require.Contains(t, err.Error(), "模拟关闭失败")
}

func TestCovMockConnector_CloseFailCustomError(t *testing.T) {
	mc := NewMockConnectorWithOptions(MockConnectorOptions{
		ShouldCloseSucceed: false,
		CloseError:         errors.New("custom-close-err"),
	})
	err := mc.Close()
	require.EqualError(t, err, "custom-close-err")
}

func TestCovMockConnector_CloseCallback(t *testing.T) {
	mc := NewMockConnector()
	called := false
	mc.OnClose = func() error {
		called = true
		return errors.New("close-cb-err")
	}
	err := mc.Close()
	require.True(t, called)
	require.EqualError(t, err, "close-cb-err")
}

// --- Connector: SendData branches ---

func TestCovMockConnector_SendDataNotRunning(t *testing.T) {
	mc := NewMockConnector()
	err := mc.SendData("s1", []byte("test"))
	require.Error(t, err)
}

func TestCovMockConnector_SendDataFailDefault(t *testing.T) {
	mc := NewMockConnectorWithOptions(MockConnectorOptions{
		ShouldSendDataSucceed: false,
	})
	mc.Start()
	err := mc.SendData("s1", []byte("test"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "模拟发送数据失败")
}

func TestCovMockConnector_SendDataFailCustomError(t *testing.T) {
	mc := NewMockConnectorWithOptions(MockConnectorOptions{
		ShouldSendDataSucceed: false,
		SendDataError:         errors.New("send-err"),
	})
	mc.Start()
	err := mc.SendData("s1", []byte("test"))
	require.EqualError(t, err, "send-err")
}

func TestCovMockConnector_SendDataCallback(t *testing.T) {
	mc := NewMockConnector()
	mc.Start()
	called := false
	mc.OnSendData = func(sid string, d []byte) error {
		called = true
		return errors.New("cb-send-err")
	}
	err := mc.SendData("s1", []byte("test"))
	require.True(t, called)
	require.EqualError(t, err, "cb-send-err")
}

// --- Connector: CreateStream branches ---

func TestCovMockConnector_CreateStreamNotRunning(t *testing.T) {
	mc := NewMockConnector()
	_, _, err := mc.CreateStream("target:80")
	require.Error(t, err)
}

func TestCovMockConnector_CreateStreamFailDefault(t *testing.T) {
	mc := NewMockConnectorWithOptions(MockConnectorOptions{
		ShouldCreateStreamSucceed: false,
	})
	mc.Start()
	_, _, err := mc.CreateStream("target:80")
	require.Error(t, err)
	require.Contains(t, err.Error(), "模拟创建流失败")
}

func TestCovMockConnector_CreateStreamFailCustomError(t *testing.T) {
	mc := NewMockConnectorWithOptions(MockConnectorOptions{
		ShouldCreateStreamSucceed: false,
		CreateStreamError:         errors.New("cs-err"),
	})
	mc.Start()
	_, _, err := mc.CreateStream("target:80")
	require.EqualError(t, err, "cs-err")
}

func TestCovMockConnector_CreateStreamCallback(t *testing.T) {
	mc := NewMockConnector()
	mc.Start()
	called := false
	mc.OnCreateStream = func(addr string) (string, tunnel.TunnelStream, error) {
		called = true
		return "", nil, errors.New("cb-cs-err")
	}
	_, _, err := mc.CreateStream("target:80")
	require.True(t, called)
	require.EqualError(t, err, "cb-cs-err")
}

// ============================================================
// MockTunnelStream tests
// ============================================================

func TestMockTunnelStream_Basic(t *testing.T) {
	mc := NewMockConnector()
	ms := NewMockTunnelStream("s1", mc, "example.com:80")
	require.Equal(t, "s1", ms.GetStreamID())
	require.Equal(t, "s1", ms.ID())
	require.Equal(t, "example.com:80", ms.TargetAddr())
}

func TestMockTunnelStream_DataOps(t *testing.T) {
	mc := NewMockConnector()
	ms := NewMockTunnelStream("s1", mc, "addr")

	require.NoError(t, ms.PutData([]byte("test")))
	got, err := ms.GetData()
	require.NoError(t, err)
	require.Equal(t, []byte("test"), got)
	require.Len(t, ms.GetPutDataCalls(), 1)
	_ = ms.GetBuffer()
}

func TestMockTunnelStream_Close(t *testing.T) {
	mc := NewMockConnector()
	ms := NewMockTunnelStream("s1", mc, "addr")
	require.NoError(t, ms.Close())
	require.True(t, ms.WasCloseCalled())
	require.True(t, ms.IsClosed())
}

func TestMockTunnelStream_WithOptions(t *testing.T) {
	mc := NewMockConnector()
	ms := NewMockTunnelStreamWithOptions(mc, MockTunnelStreamOptions{
		StreamID:   "s2",
		TargetAddr: "test.com:443",
	})
	require.Equal(t, "s2", ms.GetStreamID())
}

// --- TunnelStream: Close branches ---

func TestCovMockTunnelStream_DoubleClose(t *testing.T) {
	mc := NewMockConnector()
	ms := NewMockTunnelStream("s1", mc, "addr")
	require.NoError(t, ms.Close())
	// Second close should return nil (already closed)
	require.NoError(t, ms.Close())
}

func TestCovMockTunnelStream_CloseFail(t *testing.T) {
	mc := NewMockConnector()
	ms := NewMockTunnelStreamWithOptions(mc, MockTunnelStreamOptions{
		StreamID:           "s1",
		TargetAddr:         "addr",
		ShouldCloseSucceed: false,
	})
	err := ms.Close()
	require.Error(t, err)
}

func TestCovMockTunnelStream_CloseFailCustomError(t *testing.T) {
	mc := NewMockConnector()
	ms := NewMockTunnelStreamWithOptions(mc, MockTunnelStreamOptions{
		StreamID:     "s1",
		TargetAddr:   "addr",
		CloseError:   errors.New("custom-close-err"),
	})
	require.EqualError(t, ms.Close(), "custom-close-err")
}

func TestCovMockTunnelStream_CloseCallback(t *testing.T) {
	mc := NewMockConnector()
	ms := NewMockTunnelStream("s1", mc, "addr")
	called := false
	ms.OnClose = func() error {
		called = true
		return errors.New("cb-close-err")
	}
	require.EqualError(t, ms.Close(), "cb-close-err")
	require.True(t, called)
}

// --- TunnelStream: PutData branches ---

func TestCovMockTunnelStream_PutDataClosed(t *testing.T) {
	mc := NewMockConnector()
	ms := NewMockTunnelStream("s1", mc, "addr")
	ms.Close()
	err := ms.PutData([]byte("data"))
	require.Error(t, err)
}

func TestCovMockTunnelStream_PutDataFail(t *testing.T) {
	mc := NewMockConnector()
	ms := NewMockTunnelStreamWithOptions(mc, MockTunnelStreamOptions{
		StreamID:             "s1",
		TargetAddr:           "addr",
		ShouldPutDataSucceed: false,
	})
	err := ms.PutData([]byte("data"))
	require.Error(t, err)
}

func TestCovMockTunnelStream_PutDataFailCustomError(t *testing.T) {
	mc := NewMockConnector()
	ms := NewMockTunnelStreamWithOptions(mc, MockTunnelStreamOptions{
		StreamID:       "s1",
		TargetAddr:     "addr",
		PutDataError:   errors.New("pd-err"),
	})
	require.EqualError(t, ms.PutData([]byte("data")), "pd-err")
}

func TestCovMockTunnelStream_PutDataCallback(t *testing.T) {
	mc := NewMockConnector()
	ms := NewMockTunnelStream("s1", mc, "addr")
	called := false
	ms.OnPutData = func(d []byte) error {
		called = true
		return errors.New("cb-pd-err")
	}
	require.EqualError(t, ms.PutData([]byte("data")), "cb-pd-err")
	require.True(t, called)
}

// --- TunnelStream: GetData branches ---

func TestCovMockTunnelStream_GetDataEmpty(t *testing.T) {
	mc := NewMockConnector()
	ms := NewMockTunnelStream("s1", mc, "addr")
	data, err := ms.GetData()
	require.NoError(t, err)
	require.Nil(t, data)
}

func TestCovMockTunnelStream_GetDataClosed(t *testing.T) {
	mc := NewMockConnector()
	ms := NewMockTunnelStream("s1", mc, "addr")
	ms.Close()
	_, err := ms.GetData()
	require.Error(t, err)
}

// --- TunnelStream: ServeConn branches ---

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
	require.Len(t, ms.GetServeConnCalls(), 1)
}

func TestCovMockTunnelStream_ServeConnClosed(t *testing.T) {
	mc := NewMockConnector()
	ms := NewMockTunnelStream("s1", mc, "addr")
	ms.Close()
	err := ms.ServeConn(NewMockNetConn())
	require.Error(t, err)
}

func TestCovMockTunnelStream_ServeConnCallback(t *testing.T) {
	mc := NewMockConnector()
	ms := NewMockTunnelStream("s1", mc, "addr")
	called := false
	ms.OnServeConn = func(c net.Conn) error {
		called = true
		return errors.New("cb-sc-err")
	}
	require.EqualError(t, ms.ServeConn(NewMockNetConn()), "cb-sc-err")
	require.True(t, called)
}

func TestCovMockTunnelStream_ServeConnFail(t *testing.T) {
	mc := NewMockConnector()
	ms := NewMockTunnelStreamWithOptions(mc, MockTunnelStreamOptions{
		StreamID:               "s1",
		TargetAddr:             "addr",
		ShouldServeConnSucceed: false,
	})
	err := ms.ServeConn(NewMockNetConn())
	require.Error(t, err)
}

func TestCovMockTunnelStream_ServeConnFailCustomError(t *testing.T) {
	mc := NewMockConnector()
	ms := NewMockTunnelStreamWithOptions(mc, MockTunnelStreamOptions{
		StreamID:        "s1",
		TargetAddr:      "addr",
		ServeConnError:  errors.New("sc-err"),
	})
	require.EqualError(t, ms.ServeConn(NewMockNetConn()), "sc-err")
}

func TestCovMockTunnelStream_ServeConnWithData(t *testing.T) {
	mc := NewMockConnector()
	ms := NewMockTunnelStream("s1", mc, "addr")
	// Put data before ServeConn — triggers the "send pre-stored data" path
	ms.PutData([]byte("pre-stored"))

	clientConn, serverConn := net.Pipe()

	done := make(chan error, 1)
	go func() { done <- ms.ServeConn(serverConn) }()

	// Read the pre-stored data from client side
	buf := make([]byte, 100)
	clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := clientConn.Read(buf)
	require.NoError(t, err)
	require.Equal(t, "pre-stored", string(buf[:n]))

	// Write data from client — triggers the goroutine read path
	clientConn.Write([]byte("from-client"))
	time.Sleep(100 * time.Millisecond)

	// Close to exit ServeConn
	clientConn.Close()
	ms.Close()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("ServeConn did not exit")
	}
}

func TestCovMockTunnelStream_ServeConnWriteError(t *testing.T) {
	mc := NewMockConnector()
	ms := NewMockTunnelStream("s1", mc, "addr")
	ms.PutData([]byte("pre-stored"))

	// Use a conn that will error on Write
	errConn := NewErrorMockNetConn("write-err")
	done := make(chan error, 1)
	go func() { done <- ms.ServeConn(errConn) }()

	select {
	case err := <-done:
		require.Error(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("ServeConn did not exit on write error")
	}
}

func TestCovMockTunnelStream_ServeConnReadEOF(t *testing.T) {
	mc := NewMockConnector()
	ms := NewMockTunnelStream("s1", mc, "addr")

	clientConn, serverConn := net.Pipe()

	done := make(chan error, 1)
	go func() { done <- ms.ServeConn(serverConn) }()

	time.Sleep(50 * time.Millisecond)
	// Close client → server sees EOF → goroutine exits
	clientConn.Close()
	time.Sleep(100 * time.Millisecond)

	ms.Close()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("ServeConn did not exit")
	}
}

// ============================================================
// MockNetConn tests
// ============================================================

func TestMockNetConn_ReadWrite(t *testing.T) {
	conn := NewMockNetConn()
	n, err := conn.Write([]byte("hello"))
	require.NoError(t, err)
	require.Equal(t, 5, n)
	require.Equal(t, []byte("hello"), conn.GetWrittenData())

	conn.AddReadData([]byte("world"))
	buf := make([]byte, 10)
	n, err = conn.Read(buf)
	require.NoError(t, err)
	require.Equal(t, "world", string(buf[:n]))
}

func TestMockNetConn_Close(t *testing.T) {
	conn := NewMockNetConn()
	conn.Close()
	require.True(t, conn.IsClosed())
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
	require.NotNil(t, conn.LocalAddr())
	require.NotNil(t, conn.RemoteAddr())
}

func TestMockNetConn_ClearWritten(t *testing.T) {
	conn := NewMockNetConn()
	conn.Write([]byte("x"))
	conn.ClearWrittenData()
	require.Empty(t, conn.GetWrittenData())
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
	require.Equal(t, 10*time.Millisecond, got.ReadDelay)
}

func TestMockNetConn_SetClosed(t *testing.T) {
	conn := NewMockNetConn()
	conn.SetClosed(true)
	require.True(t, conn.IsClosed())
	conn.SetClosed(false)
	require.False(t, conn.IsClosed())
}

func TestErrorMockNetConn(t *testing.T) {
	conn := NewErrorMockNetConn("test")
	_, err := conn.Write([]byte("x"))
	require.Error(t, err)
	_, err = conn.Read(make([]byte, 10))
	require.Error(t, err)
}

func TestMockNetConn_WithOptions(t *testing.T) {
	conn := NewMockNetConnWithOptions(MockNetConnOptions{
		ReadDelay:  5 * time.Millisecond,
		WriteDelay: 5 * time.Millisecond,
	})
	require.NotNil(t, conn)
}

// --- MockNetConn: Read branches ---

func TestCovMockNetConn_ReadClosed(t *testing.T) {
	conn := NewMockNetConn()
	conn.Close()
	_, err := conn.Read(make([]byte, 10))
	require.Equal(t, ErrConnClosed, err)
}

func TestCovMockNetConn_ReadDeadlineExpired(t *testing.T) {
	conn := NewMockNetConn()
	conn.SetReadDeadline(time.Now().Add(-time.Second))
	_, err := conn.Read(make([]byte, 10))
	require.Error(t, err)
}

func TestCovMockNetConn_ReadWithDelay(t *testing.T) {
	conn := NewMockNetConnWithOptions(MockNetConnOptions{
		ReadDelay: 1 * time.Millisecond,
	})
	defer conn.Close()
	conn.AddReadData([]byte("delayed"))
	buf := make([]byte, 100)
	n, err := conn.Read(buf)
	require.NoError(t, err)
	require.Equal(t, "delayed", string(buf[:n]))
}

func TestCovMockNetConn_ReadEOF(t *testing.T) {
	conn := NewMockNetConn()
	defer conn.Close()
	_, err := conn.Read(make([]byte, 10))
	require.Equal(t, io.EOF, err)
}

func TestCovMockNetConn_ReadCallback(t *testing.T) {
	conn := NewMockNetConn()
	defer conn.Close()
	conn.OnRead = func(b []byte) (int, error) {
		copy(b, []byte("custom"))
		return 6, nil
	}
	buf := make([]byte, 100)
	n, err := conn.Read(buf)
	require.NoError(t, err)
	require.Equal(t, "custom", string(buf[:n]))
}

func TestCovMockNetConn_InitialReadData(t *testing.T) {
	conn := NewMockNetConnWithOptions(MockNetConnOptions{
		InitialReadData: []byte("initial"),
	})
	defer conn.Close()
	buf := make([]byte, 100)
	n, err := conn.Read(buf)
	require.NoError(t, err)
	require.Equal(t, "initial", string(buf[:n]))
}

// --- MockNetConn: Write branches ---

func TestCovMockNetConn_WriteClosed(t *testing.T) {
	conn := NewMockNetConn()
	conn.Close()
	_, err := conn.Write([]byte("test"))
	require.Equal(t, ErrConnClosed, err)
}

func TestCovMockNetConn_WriteDeadlineExpired(t *testing.T) {
	conn := NewMockNetConn()
	conn.SetWriteDeadline(time.Now().Add(-time.Second))
	defer conn.Close()
	_, err := conn.Write([]byte("test"))
	require.Error(t, err)
}

func TestCovMockNetConn_WriteWithDelay(t *testing.T) {
	conn := NewMockNetConnWithOptions(MockNetConnOptions{
		WriteDelay: 1 * time.Millisecond,
	})
	defer conn.Close()
	n, err := conn.Write([]byte("delayed"))
	require.NoError(t, err)
	require.Equal(t, 7, n)
}

func TestCovMockNetConn_WriteCallback(t *testing.T) {
	conn := NewMockNetConn()
	defer conn.Close()
	conn.OnWrite = func(b []byte) (int, error) {
		return len(b), nil
	}
	n, err := conn.Write([]byte("test"))
	require.NoError(t, err)
	require.Equal(t, 4, n)
}

// --- MockNetConn: Close branches ---

func TestCovMockNetConn_DoubleClose(t *testing.T) {
	conn := NewMockNetConn()
	require.NoError(t, conn.Close())
	// Second close — hits "already closed" path
	require.NoError(t, conn.Close())
}

func TestCovMockNetConn_CloseCallback(t *testing.T) {
	conn := NewMockNetConn()
	called := false
	conn.OnClose = func() error {
		called = true
		return nil
	}
	require.NoError(t, conn.Close())
	require.True(t, called)
}
