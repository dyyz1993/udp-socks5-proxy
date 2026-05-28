package client

import (
	"bytes"
	"net"
	"sync"
	"testing"
	"time"
)

// testLogger 用于测试的 Logger mock
type testLogger struct {
	buf bytes.Buffer
	mu  sync.Mutex
}

func (l *testLogger) Debug(args ...interface{})                 {}
func (l *testLogger) Debugf(format string, args ...interface{}) {}
func (l *testLogger) Info(args ...interface{})                  {}
func (l *testLogger) Infof(format string, args ...interface{})  {}
func (l *testLogger) Error(args ...interface{})                 {}
func (l *testLogger) Errorf(format string, args ...interface{}) {}

func newTestLogger() *testLogger { return &testLogger{} }

func TestGetDataType(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want string
	}{
		{"empty", nil, "EMPTY"},
		{"empty slice", []byte{}, "EMPTY"},
		{"auth resp", []byte{0x05, 0x00}, "AUTH_RESP"},
		{"connect resp", []byte{0x05, 0x00, 0x00, 0x01}, "CONNECT_RESP"},
		{"connect resp long", []byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0x50}, "CONNECT_RESP"},
		{"app data", []byte("GET / HTTP/1.1"), "APP_DATA"},
		{"app data 05xx", []byte{0x05, 0x01}, "APP_DATA"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getDataType(tt.data)
			if got != tt.want {
				t.Errorf("getDataType(%v) = %q, want %q", tt.data, got, tt.want)
			}
		})
	}
}

func TestNewVirtualSocks5Conn(t *testing.T) {
	client, _ := net.Pipe()
	defer client.Close()

	log := newTestLogger()
	originalData := []byte{0x05, 0x01, 0x00, 0x05, 0x01, 0x00, 0x01, 0x7f, 0x00, 0x00, 0x01, 0x1f, 0x90}
	v := NewVirtualSocks5Conn(client, originalData, log)

	if v == nil {
		t.Fatal("NewVirtualSocks5Conn returned nil")
	}
	if v.authTimeout != 3*time.Second {
		t.Errorf("default authTimeout = %v, want 3s", v.authTimeout)
	}
}

func TestVirtualSocks5Conn_SetAuthTimeout(t *testing.T) {
	client, _ := net.Pipe()
	defer client.Close()

	v := NewVirtualSocks5Conn(client, nil, newTestLogger())
	v.SetAuthTimeout(100 * time.Millisecond)
	if v.authTimeout != 100*time.Millisecond {
		t.Errorf("authTimeout = %v, want 100ms", v.authTimeout)
	}
}

func TestVirtualSocks5Conn_GetReadBuffer(t *testing.T) {
	client, _ := net.Pipe()
	defer client.Close()

	v := NewVirtualSocks5Conn(client, nil, newTestLogger())
	buf := v.GetReadBuffer()
	if buf == nil {
		t.Error("GetReadBuffer returned nil")
	}
}

func TestVirtualSocks5Conn_ReadHandshake(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	// originalData: 3 bytes handshake (05 01 00)
	originalData := []byte{0x05, 0x01, 0x00}
	v := NewVirtualSocks5Conn(server, originalData, newTestLogger())
	v.SetAuthTimeout(50 * time.Millisecond)

	// Read handshake bytes
	buf := make([]byte, 10)
	n, err := v.Read(buf)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if n != 3 {
		t.Errorf("Read returned %d bytes, want 3", n)
	}
	if buf[0] != 0x05 || buf[1] != 0x01 || buf[2] != 0x00 {
		t.Errorf("handshake bytes = %v, want [05 01 00]", buf[:n])
	}
}

func TestVirtualSocks5Conn_WriteAuthResp(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()

	v := NewVirtualSocks5Conn(server, []byte{0x05, 0x01, 0x00}, newTestLogger())
	v.SetAuthTimeout(50 * time.Millisecond)

	// Write auth response
	n, err := v.Write([]byte{0x05, 0x00})
	if err != nil {
		t.Fatalf("Write auth resp failed: %v", err)
	}
	if n != 2 {
		t.Errorf("Write returned %d, want 2", n)
	}
}

func TestVirtualSocks5Conn_WriteEmpty(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	v := NewVirtualSocks5Conn(server, nil, newTestLogger())
	n, err := v.Write(nil)
	if err != nil {
		t.Fatalf("Write empty failed: %v", err)
	}
	if n != 0 {
		t.Errorf("Write empty returned %d, want 0", n)
	}
}

func TestVirtualSocks5Conn_WriteAppData(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	v := NewVirtualSocks5Conn(server, nil, newTestLogger())

	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 1024)
		client.Read(buf)
	}()

	n, err := v.Write([]byte("hello"))
	if err != nil {
		t.Fatalf("Write app data failed: %v", err)
	}
	if n != 5 {
		t.Errorf("Write returned %d, want 5", n)
	}
	<-done
}

func TestVirtualSocks5Conn_CloseTwice(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()

	v := NewVirtualSocks5Conn(server, nil, newTestLogger())

	if err := v.Close(); err != nil {
		t.Fatalf("first Close failed: %v", err)
	}
	if err := v.Close(); err != nil {
		t.Fatalf("second Close failed: %v", err)
	}
}

func TestVirtualSocks5Conn_ReadAfterClose(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()

	v := NewVirtualSocks5Conn(server, nil, newTestLogger())
	v.Close()

	buf := make([]byte, 10)
	_, err := v.Read(buf)
	if err == nil {
		t.Error("expected error reading from closed conn")
	}
}

func TestVirtualSocks5Conn_WriteLargeData(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()

	v := NewVirtualSocks5Conn(server, nil, newTestLogger())

	// Write data larger than MaxWriteSize (8000)
	largeData := make([]byte, 10000)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 20000)
		total := 0
		for total < 10000 {
			n, _ := client.Read(buf[total:])
			total += n
		}
	}()

	n, err := v.Write(largeData)
	if err != nil {
		t.Fatalf("Write large data failed: %v", err)
	}
	if n != 10000 {
		t.Errorf("Write returned %d, want 10000", n)
	}
	<-done
}
