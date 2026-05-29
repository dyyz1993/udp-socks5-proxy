package server

import (
	"io"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tealife/proxy-cs3/src/tunnel"
)

// TestStreamCov_PutData_BufferFull tests PutData when buffer is full
func TestStreamCov_PutData_BufferFull(t *testing.T) {
	sc, conn := newTestSC(t)
	defer conn.Close()
	sc.Start()
	defer sc.Close()

	stream := newServerStream("s-buf-full", sc).(*serverStream)

	// Fill the buffer (readBuffer has capacity of 1024)
	for i := 0; i < 1024; i++ {
		err := stream.PutData([]byte("x"))
		if err != nil {
			break
		}
	}

	// Next PutData should return "读缓冲区已满" error
	err := stream.PutData([]byte("overflow"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "缓冲区已满")
}

// TestStreamCov_Read_Blocking tests that Read blocks until data is available
func TestStreamCov_Read_Blocking(t *testing.T) {
	sc, conn := newTestSC(t)
	defer conn.Close()
	sc.Start()
	defer sc.Close()

	stream := newServerStream("s-read-block", sc).(*serverStream)

	go func() {
		time.Sleep(100 * time.Millisecond)
		stream.PutData([]byte("delayed-data"))
	}()

	buf := make([]byte, 100)
	n, err := stream.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, "delayed-data", string(buf[:n]))
}

// TestStreamCov_Read_LargeData tests Read with large data
func TestStreamCov_Read_LargeData(t *testing.T) {
	sc, conn := newTestSC(t)
	defer conn.Close()
	sc.Start()
	defer sc.Close()

	stream := newServerStream("s-read-large", sc).(*serverStream)

	largeData := make([]byte, 5000)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	go func() {
		time.Sleep(50 * time.Millisecond)
		stream.PutData(largeData)
	}()

	buf := make([]byte, 10000)
	n, err := stream.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, 5000, n)
	assert.Equal(t, largeData, buf[:n])
}

// TestStreamCov_Write_MultipleChunks tests Write with multiple chunks
func TestStreamCov_Write_MultipleChunks(t *testing.T) {
	sc, conn := newTestSC(t)
	defer conn.Close()
	sc.Start()
	defer sc.Close()

	sc.BaseConnector.SetConnectionID("conn-multi")

	go func() {
		buf := make([]byte, 65535)
		for {
			_, _, err := conn.ReadFromUDP(buf)
			if err != nil {
				return
			}
		}
	}()

	stream := newServerStream("s-write-multi", sc).(*serverStream)

	for i := 0; i < 5; i++ {
		data := []byte{byte(i)}
		n, err := stream.Write(data)
		assert.NoError(t, err)
		assert.Equal(t, 1, n)
	}
}

// TestStreamCov_PutData_AfterClose tests PutData after close
func TestStreamCov_PutData_AfterClose(t *testing.T) {
	sc, conn := newTestSC(t)
	defer conn.Close()
	sc.Start()
	defer sc.Close()

	stream := newServerStream("s-put-close", sc).(*serverStream)
	stream.Close()

	err := stream.PutData([]byte("after-close"))
	assert.Equal(t, io.ErrClosedPipe, err)
}

// TestStreamCov_Write_AfterClose tests Write after close
func TestStreamCov_Write_AfterClose(t *testing.T) {
	sc, conn := newTestSC(t)
	defer conn.Close()
	sc.Start()
	defer sc.Close()

	sc.BaseConnector.SetConnectionID("conn-wc")

	stream := newServerStream("s-write-close", sc).(*serverStream)
	stream.Close()

	_, err := stream.Write([]byte("after-close"))
	assert.Equal(t, io.ErrClosedPipe, err)
}

// TestStreamCov_PutData_Empty tests PutData with empty data
func TestStreamCov_PutData_Empty(t *testing.T) {
	sc, conn := newTestSC(t)
	defer conn.Close()
	sc.Start()
	defer sc.Close()

	stream := newServerStream("s-empty", sc).(*serverStream)

	err := stream.PutData([]byte{})
	assert.NoError(t, err)
}

// TestStreamCov_NetAddr_Detailed tests all net.Conn interface methods
func TestStreamCov_NetAddr_Detailed(t *testing.T) {
	sc, conn := newTestSC(t)
	defer conn.Close()
	sc.Start()
	defer sc.Close()

	stream := newServerStream("s-addr", sc).(*serverStream)

	// Verify net.Conn implementation
	var _ net.Conn = stream

	local := stream.LocalAddr()
	assert.NotNil(t, local)
	assert.Contains(t, local.Network(), "tcp")

	remote := stream.RemoteAddr()
	assert.NotNil(t, remote)

	assert.NoError(t, stream.SetDeadline(time.Now()))
	assert.NoError(t, stream.SetReadDeadline(time.Now()))
	assert.NoError(t, stream.SetWriteDeadline(time.Now()))
}

// TestStreamCov_SendErrorPacket_WithConn tests SendErrorPacket with connector
func TestStreamCov_SendErrorPacket_WithConn(t *testing.T) {
	sc, conn := newTestSC(t)
	defer conn.Close()
	sc.Start()
	defer sc.Close()

	sc.BaseConnector.SetConnectionID("conn-err-pkt")

	go func() {
		buf := make([]byte, 65535)
		for {
			_, _, err := conn.ReadFromUDP(buf)
			if err != nil {
				return
			}
		}
	}()

	stream := newServerStream("s-err-pkt", sc).(*serverStream)
	err := stream.SendErrorPacket(1001, "test error")
	assert.NoError(t, err)
}

// TestStreamCov_FragmentPacket_Creator tests creating fragment packets
func TestStreamCov_FragmentPacket_Creator(t *testing.T) {
	frag := tunnel.NewFragmentPacket("conn-1", "stream-1", tunnel.PacketTypeData, 1, 3, 0, 0, []byte("data"))
	assert.NotNil(t, frag)

	parsed, err := tunnel.ParsePacket(frag.Bytes())
	require.NoError(t, err)
	assert.True(t, parsed.Header.Type == tunnel.PacketTypeFragmented, "expected fragmented type")
}

// TestStreamCov_ConcurrentPutRead tests concurrent put and read
func TestStreamCov_ConcurrentPutRead(t *testing.T) {
	sc, conn := newTestSC(t)
	defer conn.Close()
	sc.Start()
	defer sc.Close()

	stream := newServerStream("s-concurrent", sc).(*serverStream)

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 10; i++ {
			stream.PutData([]byte{byte(i)})
			time.Sleep(10 * time.Millisecond)
		}
	}()

	for i := 0; i < 10; i++ {
		buf := make([]byte, 10)
		n, err := stream.Read(buf)
		if err != nil {
			break
		}
		assert.Equal(t, 1, n)
	}

	<-done
}

func TestCovServerStream_Read_Closed(t *testing.T) {
	sc, conn := newTestSC(t)
	defer conn.Close()
	sc.Start()
	defer sc.Close()

	stream := newServerStream("read-closed", sc).(*serverStream)
	stream.Close()

	buf := make([]byte, 1024)
	_, err := stream.Read(buf)
	assert.Equal(t, io.EOF, err)
}

func TestCovServerStream_Read_ChannelClosed(t *testing.T) {
	sc, conn := newTestSC(t)
	defer conn.Close()
	sc.Start()
	defer sc.Close()

	stream := newServerStream("read-chclosed", sc).(*serverStream)
	impl := stream

	// Close the readBuffer channel directly
	close(impl.readBuffer)

	buf := make([]byte, 1024)
	_, err := stream.Read(buf)
	assert.Equal(t, io.EOF, err)
}

func TestCovServerStream_Write_Closed(t *testing.T) {
	sc, conn := newTestSC(t)
	defer conn.Close()
	sc.Start()
	defer sc.Close()

	stream := newServerStream("write-closed", sc).(*serverStream)
	stream.Close()

	_, err := stream.Write([]byte("test"))
	assert.Equal(t, io.ErrClosedPipe, err)
}

func TestCovServerStream_Close_Twice(t *testing.T) {
	sc, conn := newTestSC(t)
	defer conn.Close()
	sc.Start()
	defer sc.Close()

	stream := newServerStream("close-twice", sc).(*serverStream)
	err := stream.Close()
	assert.NoError(t, err)

	err = stream.Close()
	assert.NoError(t, err)
}
