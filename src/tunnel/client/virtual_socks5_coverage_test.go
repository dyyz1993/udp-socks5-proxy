package client

import (
	"bytes"
	"io"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testLog struct {
	t *testing.T
}

func (l *testLog) Debug(args ...interface{}) { l.t.Log(args...) }
func (l *testLog) Debugf(f string, a ...interface{}) {
	l.t.Logf(f, a...)
}
func (l *testLog) Info(args ...interface{})          { l.t.Log(args...) }
func (l *testLog) Infof(f string, a ...interface{})  { l.t.Logf(f, a...) }
func (l *testLog) Error(args ...interface{})         { l.t.Log(args...) }
func (l *testLog) Errorf(f string, a ...interface{}) { l.t.Logf(f, a...) }

// TestVirtualSocks5Conn_Read_Handshake tests reading the initial handshake bytes
func TestVirtualSocks5Conn_Read_Handshake(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	originalData := []byte{0x05, 0x01, 0x00}
	logger := &testLog{t}
	vconn := NewVirtualSocks5Conn(client, originalData, logger)

	// Read should return the first 3 bytes (SOCKS5 handshake)
	buf := make([]byte, 10)
	n, err := vconn.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, 3, n)
	assert.Equal(t, []byte{0x05, 0x01, 0x00}, buf[:n])
}

// TestVirtualSocks5Conn_Read_BufferData tests reading from internal buffer
func TestVirtualSocks5Conn_Read_BufferData(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	originalData := []byte{0x05, 0x01, 0x00}
	logger := &testLog{t}
	vconn := NewVirtualSocks5Conn(client, originalData, logger)

	// Put data in the read buffer
	vconn.readBuf.Write([]byte("buffered-data"))

	buf := make([]byte, 20)
	n, err := vconn.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, "buffered-data", string(buf[:n]))
}

// TestVirtualSocks5Conn_Read_AuthWait tests the auth response wait flow
func TestVirtualSocks5Conn_Read_AuthWait(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	originalData := []byte{0x05, 0x01, 0x00, 0x05, 0x01, 0x00, 0x01, 0x7f, 0x00, 0x00, 0x01, 0x00, 0x50}
	logger := &testLog{t}
	vconn := NewVirtualSocks5Conn(client, originalData, logger)
	vconn.SetAuthTimeout(100 * time.Millisecond)

	// First read: handshake
	buf := make([]byte, 10)
	n, err := vconn.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, 3, n)

	// Second read: should wait for auth response, then timeout
	// The auth wait will timeout after 100ms
	done := make(chan struct{})
	go func() {
		defer close(done)
		n2, err2 := vconn.Read(buf)
		t.Logf("Auth wait read: n=%d, err=%v", n2, err2)
	}()

	// Wait for the read to complete (should timeout)
	select {
	case <-done:
		t.Log("Auth wait completed")
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Auth wait timed out")
	}
}

// TestVirtualSocks5Conn_Read_Closed tests reading from a closed connection
func TestVirtualSocks5Conn_Read_Closed(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	originalData := []byte{0x05, 0x01, 0x00}
	logger := &testLog{t}
	vconn := NewVirtualSocks5Conn(client, originalData, logger)

	// Close the connection
	err := vconn.Close()
	require.NoError(t, err)

	// Read should return EOF
	buf := make([]byte, 10)
	_, err = vconn.Read(buf)
	assert.Equal(t, io.EOF, err)
}

// TestVirtualSocks5Conn_Read_AppData tests reading application data after handshake
func TestVirtualSocks5Conn_Read_AppData(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	originalData := []byte{0x05, 0x01, 0x00}
	logger := &testLog{t}
	vconn := NewVirtualSocks5Conn(client, originalData, logger)
	vconn.SetAuthTimeout(50 * time.Millisecond)

	// Set state to past handshake
	vconn.mu.Lock()
	vconn.currentPos = len(originalData)
	vconn.hasSentAuth = true
	vconn.hasRecvAuth = true
	vconn.hasSentConnect = true
	vconn.mu.Unlock()

	// Write test data from server side
	go func() {
		server.Write([]byte("hello-app-data"))
	}()

	// Read application data
	buf := make([]byte, 100)
	n, err := vconn.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, "hello-app-data", string(buf[:n]))
}

// TestVirtualSocks5Conn_Write_AuthResp tests writing auth response
func TestVirtualSocks5Conn_Write_AuthResp(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	originalData := []byte{0x05, 0x01, 0x00}
	logger := &testLog{t}
	vconn := NewVirtualSocks5Conn(client, originalData, logger)

	// Write auth response (05 00)
	n, err := vconn.Write([]byte{0x05, 0x00})
	require.NoError(t, err)
	assert.Equal(t, 2, n)

	// Verify auth received state
	vconn.mu.Lock()
	assert.True(t, vconn.hasRecvAuth)
	vconn.mu.Unlock()
}

// TestVirtualSocks5Conn_Write_ConnectResp tests writing connect response
func TestVirtualSocks5Conn_Write_ConnectResp(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()

	originalData := []byte{0x05, 0x01, 0x00}
	logger := &testLog{t}
	vconn := NewVirtualSocks5Conn(client, originalData, logger)

	// Mark auth as received
	vconn.mu.Lock()
	vconn.hasRecvAuth = true
	vconn.mu.Unlock()

	// Read from server side to verify data arrives
	done := make(chan []byte, 1)
	go func() {
		buf := make([]byte, 100)
		n, _ := server.Read(buf)
		done <- buf[:n]
		server.Close()
	}()

	// Write connect response
	connectResp := []byte{0x05, 0x00, 0x00, 0x01, 0x7f, 0x00, 0x00, 0x01, 0x00, 0x50}
	n, err := vconn.Write(connectResp)
	require.NoError(t, err)
	assert.Equal(t, len(connectResp), n)

	// Verify data received on server
	select {
	case data := <-done:
		assert.Equal(t, connectResp, data)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for connect response")
	}
}

// TestVirtualSocks5Conn_Write_AppData tests writing application data
func TestVirtualSocks5Conn_Write_AppData(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()

	originalData := []byte{0x05, 0x01, 0x00}
	logger := &testLog{t}
	vconn := NewVirtualSocks5Conn(client, originalData, logger)

	// Mark as past handshake
	vconn.mu.Lock()
	vconn.hasRecvAuth = true
	vconn.hasSentConnect = true
	vconn.mu.Unlock()

	// Read from server side
	done := make(chan []byte, 1)
	go func() {
		buf := make([]byte, 1000)
		n, _ := server.Read(buf)
		done <- buf[:n]
		server.Close()
	}()

	// Write app data
	testData := []byte("test application data")
	n, err := vconn.Write(testData)
	require.NoError(t, err)
	assert.Equal(t, len(testData), n)

	select {
	case data := <-done:
		assert.Equal(t, testData, data)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}
}

// TestVirtualSocks5Conn_Write_LargeData tests writing data > 8000 bytes
func TestVirtualSocks5Conn_Write_LargeData(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()

	originalData := []byte{0x05, 0x01, 0x00}
	logger := &testLog{t}
	vconn := NewVirtualSocks5Conn(client, originalData, logger)

	vconn.mu.Lock()
	vconn.hasRecvAuth = true
	vconn.hasSentConnect = true
	vconn.mu.Unlock()

	// Read all data from server side
	done := make(chan int, 1)
	go func() {
		total := 0
		buf := make([]byte, 65535)
		for {
			n, err := server.Read(buf)
			total += n
			if err != nil {
				break
			}
		}
		done <- total
		server.Close()
	}()

	// Write large data (>8000 bytes triggers chunking)
	largeData := make([]byte, 12000)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	n, err := vconn.Write(largeData)
	require.NoError(t, err)
	assert.Equal(t, 12000, n)

	// Close to signal EOF
	client.Close()

	select {
	case total := <-done:
		assert.Equal(t, 12000, total)
	case <-time.After(5 * time.Second):
		t.Fatal("timeout")
	}
}

// TestVirtualSocks5Conn_Write_Empty tests writing empty data
func TestVirtualSocks5Conn_Write_Empty(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	originalData := []byte{0x05, 0x01, 0x00}
	logger := &testLog{t}
	vconn := NewVirtualSocks5Conn(client, originalData, logger)

	n, err := vconn.Write([]byte{})
	require.NoError(t, err)
	assert.Equal(t, 0, n)
}

// TestVirtualSocks5Conn_Close tests closing the connection
func TestVirtualSocks5Conn_Close(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	originalData := []byte{0x05, 0x01, 0x00}
	logger := &testLog{t}
	vconn := NewVirtualSocks5Conn(client, originalData, logger)

	err := vconn.Close()
	require.NoError(t, err)

	// Double close should be safe
	err = vconn.Close()
	require.NoError(t, err)

	vconn.mu.Lock()
	assert.True(t, vconn.closed)
	vconn.mu.Unlock()
}

// TestVirtualSocks5Conn_Read_ConnectPhase tests reading connect request data
func TestVirtualSocks5Conn_Read_ConnectPhase(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	// Original data includes handshake + connect request
	originalData := []byte{0x05, 0x01, 0x00, 0x05, 0x01, 0x00, 0x01, 0x7f, 0x00, 0x00, 0x01, 0x00, 0x50}
	logger := &testLog{t}
	vconn := NewVirtualSocks5Conn(client, originalData, logger)

	// First read: handshake (3 bytes)
	buf := make([]byte, 10)
	n, err := vconn.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, 3, n)

	// Set auth received so we skip the wait
	vconn.mu.Lock()
	vconn.hasRecvAuth = true
	vconn.mu.Unlock()

	// Second read: connect request (remaining bytes)
	n, err = vconn.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, 10, n)
	assert.Equal(t, originalData[3:], buf[:n])
}

// TestGetDataType_Coverage tests additional getDataType cases
func TestGetDataType_Coverage(t *testing.T) {
	assert.Equal(t, "EMPTY", getDataType([]byte{}))
	assert.Equal(t, "AUTH_RESP", getDataType([]byte{0x05, 0x00}))
	assert.Equal(t, "CONNECT_RESP", getDataType([]byte{0x05, 0x00, 0x00, 0x01}))
	assert.Equal(t, "APP_DATA", getDataType([]byte{0x01, 0x02, 0x03}))
}

// TestVirtualSocks5Conn_Write_AfterAuthReceived tests writing connect resp when auth already received
func TestVirtualSocks5Conn_Write_AuthRespTwice(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	originalData := []byte{0x05, 0x01, 0x00}
	logger := &testLog{t}
	vconn := NewVirtualSocks5Conn(client, originalData, logger)

	// First write: auth response
	n, err := vconn.Write([]byte{0x05, 0x00})
	require.NoError(t, err)
	assert.Equal(t, 2, n)

	// Second write of auth response: should go through as app data
	// since hasRecvAuth is already true
	go func() {
		buf := make([]byte, 100)
		server.Read(buf)
	}()

	n, err = vconn.Write([]byte{0x05, 0x00})
	require.NoError(t, err)
	assert.Equal(t, 2, n)
}

// TestVirtualSocks5Conn_ReadAfterCloseChannel tests that Read returns EOF when closed via channel
func TestVirtualSocks5Conn_ReadAfterCloseChannel(t *testing.T) {
	client, server := net.Pipe()

	originalData := []byte{0x05, 0x01, 0x00}
	logger := &testLog{t}
	vconn := NewVirtualSocks5Conn(client, originalData, logger)

	// Close both connections
	vconn.Close()
	server.Close()

	buf := make([]byte, 10)
	_, err := vconn.Read(buf)
	assert.Equal(t, io.EOF, err)
}

// TestVirtualSocks5Conn_ReadPartialHandshake tests reading handshake in smaller buffer
func TestVirtualSocks5Conn_ReadPartialHandshake(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	originalData := []byte{0x05, 0x01, 0x00}
	logger := &testLog{t}
	vconn := NewVirtualSocks5Conn(client, originalData, logger)

	// Read into 1-byte buffer (partial read)
	buf := make([]byte, 1)
	n, err := vconn.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.Equal(t, byte(0x05), buf[0])

	// Read remaining bytes
	var remaining []byte
	for {
		n, err := vconn.Read(buf)
		if err != nil {
			break
		}
		remaining = append(remaining, buf[:n]...)
		if len(remaining) >= 2 {
			break
		}
	}
	assert.Equal(t, []byte{0x01, 0x00}, remaining)
}

// TestVirtualSocks5Conn_ConcurrentReadWrite tests concurrent read and write
func TestVirtualSocks5Conn_ConcurrentReadWrite(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	originalData := []byte{0x05, 0x01, 0x00}
	logger := &testLog{t}
	vconn := NewVirtualSocks5Conn(client, originalData, logger)

	// Set past handshake
	vconn.mu.Lock()
	vconn.currentPos = len(originalData)
	vconn.hasSentAuth = true
	vconn.hasRecvAuth = true
	vconn.hasSentConnect = true
	vconn.mu.Unlock()

	// Server echoes data back
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := server.Read(buf)
			if err != nil {
				return
			}
			server.Write(buf[:n])
		}
	}()

	// Write and read concurrently
	done := make(chan struct{})
	go func() {
		defer close(done)
		data := []byte("concurrent-test")
		n, err := vconn.Write(data)
		assert.NoError(t, err)
		assert.Equal(t, len(data), n)
	}()

	// Read echoed data
	buf := make([]byte, 100)
	n, err := vconn.Read(buf)
	assert.NoError(t, err)
	assert.Equal(t, "concurrent-test", string(buf[:n]))

	<-done
}

// TestVirtualSocks5Conn_WriteWithNilData tests writing nil
func TestVirtualSocks5Conn_WriteWithNilData(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	originalData := []byte{0x05, 0x01, 0x00}
	logger := &testLog{t}
	vconn := NewVirtualSocks5Conn(client, originalData, logger)

	n, err := vconn.Write(nil)
	require.NoError(t, err)
	assert.Equal(t, 0, n)
}

// TestVirtualSocks5Conn_WriteAppDataWithChunking tests write chunking with exact boundary
func TestVirtualSocks5Conn_WriteAppDataWithChunking(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()

	originalData := []byte{0x05, 0x01, 0x00}
	logger := &testLog{t}
	vconn := NewVirtualSocks5Conn(client, originalData, logger)

	vconn.mu.Lock()
	vconn.hasRecvAuth = true
	vconn.hasSentConnect = true
	vconn.mu.Unlock()

	// Read all data from server
	done := make(chan int, 1)
	go func() {
		total := 0
		buf := make([]byte, 65535)
		for {
			n, err := server.Read(buf)
			total += n
			if err != nil {
				break
			}
		}
		done <- total
		server.Close()
	}()

	// Write exactly 8001 bytes (just over chunking threshold)
	data := bytes.Repeat([]byte{0xAA}, 8001)
	n, err := vconn.Write(data)
	require.NoError(t, err)
	assert.Equal(t, 8001, n)

	client.Close()

	select {
	case total := <-done:
		assert.Equal(t, 8001, total)
	case <-time.After(5 * time.Second):
		t.Fatal("timeout")
	}
}
