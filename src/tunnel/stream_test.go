package tunnel

import (
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type mockConnector struct {
	mu      sync.Mutex
	sent    map[string][][]byte
	removed []string
}

func newMockConn() *mockConnector {
	return &mockConnector{sent: make(map[string][][]byte)}
}
func (m *mockConnector) SendData(streamID string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent[streamID] = append(m.sent[streamID], data)
	return nil
}
func (m *mockConnector) RemoveStream(streamID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.removed = append(m.removed, streamID)
}

func TestNewTunnelStreamImpl(t *testing.T) {
	mc := newMockConn()
	s := NewTunnelStreamImpl("s1", mc)
	if s == nil {
		t.Fatal("NewTunnelStreamImpl returned nil")
	}
	if s.GetStreamID() != "s1" {
		t.Errorf("streamID = %q, want %q", s.GetStreamID(), "s1")
	}
}

func TestTunnelStreamImpl_PutData_GetData(t *testing.T) {
	mc := newMockConn()
	s := NewTunnelStreamImpl("s1", mc)

	data := []byte("hello world")
	if err := s.PutData(data); err != nil {
		t.Fatalf("PutData failed: %v", err)
	}

	got, err := s.GetData()
	if err != nil {
		t.Fatalf("GetData failed: %v", err)
	}
	if string(got) != string(data) {
		t.Errorf("GetData = %q, want %q", got, data)
	}
}

func TestTunnelStreamImpl_PutData_SOCKS5Types(t *testing.T) {
	mc := newMockConn()
	s := NewTunnelStreamImpl("s1", mc)

	cases := []struct {
		name string
		data []byte
	}{
		{"auth resp", []byte{0x05, 0x00}},
		{"handshake", []byte{0x05, 0x01, 0x00}},
		{"connect resp", []byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0x50}},
		{"app data", []byte("GET / HTTP/1.1")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := s.PutData(tc.data); err != nil {
				t.Fatalf("PutData(%s) failed: %v", tc.name, err)
			}
		})
	}
}

func TestTunnelStreamImpl_Close(t *testing.T) {
	mc := newMockConn()
	s := NewTunnelStreamImpl("s1", mc)

	if err := s.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Double close should not panic
	if err := s.Close(); err != nil {
		t.Fatalf("Double Close failed: %v", err)
	}

	// Verify RemoveStream was called
	mc.mu.Lock()
	found := false
	for _, id := range mc.removed {
		if id == "s1" {
			found = true
		}
	}
	mc.mu.Unlock()
	if !found {
		t.Error("RemoveStream not called on Close")
	}
}

func TestTunnelStreamImpl_PutDataAfterClose(t *testing.T) {
	mc := newMockConn()
	s := NewTunnelStreamImpl("s1", mc)
	s.Close()

	// PutData on closed stream should return error
	if err := s.PutData([]byte("data")); err == nil {
		t.Error("expected error on closed stream")
	}
}

func TestTunnelStreamImpl_GetDataAfterClose(t *testing.T) {
	mc := newMockConn()
	s := NewTunnelStreamImpl("s1", mc)
	s.Close()

	_, err := s.GetData()
	if err == nil {
		t.Error("expected error on closed stream")
	}
}

func TestTunnelStreamImpl_ServeConn(t *testing.T) {
	mc := newMockConn()
	s := NewTunnelStreamImpl("s1", mc)

	client, server := net.Pipe()
	defer client.Close()

	done := make(chan error, 1)
	go func() {
		done <- s.ServeConn(server)
	}()

	// Write data from client
	client.Write([]byte("test data"))
	time.Sleep(50 * time.Millisecond)

	// Put data to stream
	s.PutData([]byte("response"))
	time.Sleep(50 * time.Millisecond)

	// Close
	client.Close()
	s.Close()

	select {
	case err := <-done:
		if err != nil {
			t.Logf("ServeConn returned: %v (acceptable)", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("ServeConn timed out")
	}
}

func TestTunnelStreamImpl_ConcurrentPutData(t *testing.T) {
	mc := newMockConn()
	s := NewTunnelStreamImpl("s1", mc)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			s.PutData([]byte{byte(i)})
		}(i)
	}
	wg.Wait()

	// Should be able to read some data
	got, err := s.GetData()
	if err != nil || got == nil {
		t.Error("expected data after concurrent PutData")
	}
	s.Close()
}

func TestServeConn_ClientEOF(t *testing.T) {
	mc := newMockConn()
	stream := NewTunnelStreamImpl("serve-eof", mc)

	clientConn, serverConn := net.Pipe()

	done := make(chan error, 1)
	go func() {
		done <- stream.ServeConn(serverConn)
	}()

	clientConn.Write([]byte("hello"))
	time.Sleep(100 * time.Millisecond)
	clientConn.Close()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("ServeConn did not exit in time")
	}
}

func TestServeConn_DataTransfer(t *testing.T) {
	mc := newMockConn()
	stream := NewTunnelStreamImpl("serve-transfer", mc)

	clientConn, serverConn := net.Pipe()

	done := make(chan error, 1)
	go func() {
		done <- stream.ServeConn(serverConn)
	}()

	// Put data into stream's readBuffer (simulating tunnel data)
	stream.PutData([]byte("from-tunnel"))
	time.Sleep(100 * time.Millisecond)

	// Read from client side
	buf := make([]byte, 1024)
	clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := clientConn.Read(buf)
	require.NoError(t, err)
	require.Equal(t, "from-tunnel", string(buf[:n]))

	// Close to exit ServeConn
	clientConn.Close()
	stream.Close()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("ServeConn did not exit in time")
	}
}

func TestServeConn_WriteError(t *testing.T) {
	mc := newMockConn()
	stream := NewTunnelStreamImpl("serve-write-err", mc)

	clientConn, serverConn := net.Pipe()
	clientConn.Close()

	done := make(chan error, 1)
	go func() {
		done <- stream.ServeConn(serverConn)
	}()

	stream.PutData([]byte("will-fail"))

	select {
	case err := <-done:
		// Either error from write failure, or nil from EOF - both are acceptable
		_ = err
	case <-time.After(5 * time.Second):
		t.Fatal("ServeConn did not exit in time")
	}
	serverConn.Close()
}

func TestPutData_ClosedStream(t *testing.T) {
	mc := newMockConn()
	stream := NewTunnelStreamImpl("put-closed", mc)
	stream.Close()

	err := stream.PutData([]byte("test"))
	require.Error(t, err)
}

func TestPutData_SOCKS5DataTypes(t *testing.T) {
	mc := newMockConn()
	stream := NewTunnelStreamImpl("put-socks5", mc)

	tests := []struct {
		name string
		data []byte
	}{
		{"auth_response", []byte{0x05, 0x00}},
		{"handshake_request", []byte{0x05, 0x01, 0x00}},
		{"connect_request_ipv4", []byte{0x05, 0x01, 0x00, 0x01, 127, 0, 0, 1, 0, 80}},
		{"connect_response", []byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 80}},
		{"other_socks5", []byte{0x05, 0x03, 0x01, 0x00}},
		{"long_data", append([]byte{0x05, 0x01, 0x00, 0x03, byte(len("example.com"))}, []byte("example.com")...)},
		{"non_socks5", []byte("GET / HTTP/1.1\r\nHost: example.com\r\n")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := stream.PutData(tt.data)
			require.NoError(t, err)
		})
	}
}

func TestGetData_BufRead(t *testing.T) {
	mc := newMockConn()
	stream := NewTunnelStreamImpl("get-buf", mc)

	// Put data first
	stream.PutData([]byte("test-data"))

	// Get it back
	data, err := stream.GetData()
	require.NoError(t, err)
	require.Equal(t, "test-data", string(data))
}

func TestGetData_Closed(t *testing.T) {
	mc := newMockConn()
	stream := NewTunnelStreamImpl("get-closed", mc)
	stream.Close()

	_, err := stream.GetData()
	require.Error(t, err)
}
