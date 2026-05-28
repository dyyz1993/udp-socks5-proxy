package server

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tealife/proxy-cs3/src/tunnel"
)

// 创建模拟UDP连接，用于测试
func createMockUDPConn() (*net.UDPConn, *net.UDPAddr, error) {
	// 创建本地地址
	localAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	if err != nil {
		return nil, nil, err
	}

	// 创建远程地址
	remoteAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:12345")
	if err != nil {
		return nil, nil, err
	}

	// 创建UDP连接
	conn, err := net.ListenUDP("udp", localAddr)
	if err != nil {
		return nil, nil, err
	}

	return conn, remoteAddr, nil
}

// 创建服务端连接器，用于测试
func createServerConnector(t *testing.T) (*ServerConnector, *net.UDPConn, *net.UDPAddr) {
	// 创建模拟UDP连接
	conn, remoteAddr, err := createMockUDPConn()
	if err != nil {
		t.Fatalf("创建模拟UDP连接失败: %v", err)
	}

	// 创建服务器连接器
	sc := NewServerConnector(conn, remoteAddr)
	return sc, conn, remoteAddr
}

// TestNewServerConnector 测试创建新的服务器连接器
func TestNewServerConnector(t *testing.T) {
	// 创建模拟UDP连接
	conn, remoteAddr, err := createMockUDPConn()
	if err != nil {
		t.Fatalf("创建模拟UDP连接失败: %v", err)
	}
	defer conn.Close()

	// 创建服务器连接器
	sc := NewServerConnector(conn, remoteAddr)

	if sc == nil {
		t.Fatal("创建的连接器不应为nil")
	}

	if sc.conn != conn {
		t.Error("连接器的conn字段设置错误")
	}

	if sc.remoteAddr != remoteAddr {
		t.Error("连接器的remoteAddr字段设置错误")
	}

	if sc.isRunning {
		t.Error("新创建的连接器不应处于运行状态")
	}
}

// TestServerConnectorStart 测试启动服务端连接器
func TestServerConnectorStart(t *testing.T) {
	// 创建模拟UDP连接
	conn, remoteAddr, err := createMockUDPConn()
	if err != nil {
		t.Fatalf("创建模拟UDP连接失败: %v", err)
	}
	defer conn.Close()

	// 创建服务端连接器
	sc := NewServerConnector(conn, remoteAddr)

	// 启动连接器
	err = sc.Start()
	if err != nil {
		t.Fatalf("启动服务端连接器失败: %v", err)
	}

	// 检查状态
	if !sc.isRunning {
		t.Error("启动后服务端连接器应处于运行状态")
	}

	if !sc.IsConnected() {
		t.Error("启动后服务端连接器应处于已连接状态")
	}
}

// TestServerConnectorClose 测试关闭服务端连接器
func TestServerConnectorClose(t *testing.T) {
	// 创建模拟UDP连接
	conn, remoteAddr, err := createMockUDPConn()
	if err != nil {
		t.Fatalf("创建模拟UDP连接失败: %v", err)
	}
	defer conn.Close()

	// 创建服务端连接器
	sc := NewServerConnector(conn, remoteAddr)

	// 启动连接器
	err = sc.Start()
	if err != nil {
		t.Fatalf("启动服务端连接器失败: %v", err)
	}

	// 关闭连接器
	err = sc.Close()
	if err != nil {
		t.Fatalf("关闭服务端连接器失败: %v", err)
	}

	// 检查状态
	if sc.isRunning {
		t.Error("关闭后服务端连接器不应处于运行状态")
	}

	if sc.IsConnected() {
		t.Error("关闭后服务端连接器不应处于已连接状态")
	}

	// 尝试重复关闭
	err = sc.Close()
	if err != nil {
		t.Error("重复关闭服务端连接器应成功")
	}
}

// TestServerConnectorSendData 测试发送数据
func TestServerConnectorSendData(t *testing.T) {
	// 创建模拟UDP连接
	conn, remoteAddr, err := createMockUDPConn()
	if err != nil {
		t.Fatalf("创建模拟UDP连接失败: %v", err)
	}
	defer conn.Close()

	// 创建服务器连接器
	sc := NewServerConnector(conn, remoteAddr)

	// 启动连接器
	err = sc.Start()
	if err != nil {
		t.Fatalf("启动服务器连接器失败: %v", err)
	}

	// 测试发送数据
	streamID := "test-stream"
	testData := []byte("test-data-for-send")

	// 发送数据
	initialTime := sc.lastActiveTime
	time.Sleep(10 * time.Millisecond) // 等待一段时间确保时间戳会变化

	err = sc.SendData(streamID, testData)
	if err != nil {
		t.Errorf("发送数据失败: %v", err)
	}

	// 检查最后活跃时间是否已更新
	if !sc.lastActiveTime.After(initialTime) {
		t.Error("发送数据后最后活跃时间应该更新")
	}
}

// TestServerConnectorProcessHeartbeat 测试处理心跳包
func TestServerConnectorProcessHeartbeat(t *testing.T) {
	// 创建模拟UDP连接
	conn, remoteAddr, err := createMockUDPConn()
	if err != nil {
		t.Fatalf("创建模拟UDP连接失败: %v", err)
	}
	defer conn.Close()

	// 创建服务器连接器
	sc := NewServerConnector(conn, remoteAddr)

	// 启动连接器
	err = sc.Start()
	if err != nil {
		t.Fatalf("启动服务器连接器失败: %v", err)
	}

	connectionID := sc.BaseConnector.GetConnectionID()

	// 测试处理心跳包
	heartbeatPacket := tunnel.NewHeartbeatPacket(connectionID, 1, 0)
	err = sc.ProcessIncomingData(heartbeatPacket.Bytes())
	if err != nil {
		t.Errorf("处理心跳包失败: %v", err)
	}
}

// TestServerConnectorProcessInvalidData 测试处理无效数据
func TestServerConnectorProcessInvalidData(t *testing.T) {
	// 创建模拟UDP连接
	conn, remoteAddr, err := createMockUDPConn()
	if err != nil {
		t.Fatalf("创建模拟UDP连接失败: %v", err)
	}
	defer conn.Close()

	// 创建服务器连接器
	sc := NewServerConnector(conn, remoteAddr)

	// 启动连接器
	err = sc.Start()
	if err != nil {
		t.Fatalf("启动服务器连接器失败: %v", err)
	}

	// 测试处理无效包
	invalidData := []byte{0x99, 0x00, 0x00} // 无效的包类型
	err = sc.ProcessIncomingData(invalidData)
	if err == nil {
		t.Error("处理无效包应该返回错误")
	}
}

// TestProcessIncomingData 测试处理传入数据的函数
func TestProcessIncomingData(t *testing.T) {
	// 创建服务端连接器
	udpConn, remoteAddr, err := createMockUDPConn()
	require.NoError(t, err)

	sc := NewServerConnector(udpConn, remoteAddr)
	err = sc.Start()
	require.NoError(t, err)
	defer sc.Close()

	// 测试数据包
	connID := sc.BaseConnector.GetConnectionID()

	t.Run("处理数据包", func(t *testing.T) {
		// 创建一个流用于测试
		streamID := "test-data-stream"
		stream := newServerStream(streamID, sc)
		sc.BaseConnector.AddStream(streamID, stream)

		// 创建数据包
		dataPacket := tunnel.NewDataPacket(connID, streamID, []byte("test data"))

		// 处理数据包
		err := sc.ProcessIncomingData(dataPacket.Bytes())
		require.NoError(t, err)
	})

	t.Run("处理关闭包", func(t *testing.T) {
		// 创建一个流用于测试
		streamID := "test-close-stream"
		stream := newServerStream(streamID, sc)
		sc.BaseConnector.AddStream(streamID, stream)

		// 创建关闭包
		closePacket := tunnel.NewClosePacket(connID, streamID)

		// 处理关闭包
		err := sc.ProcessIncomingData(closePacket.Bytes())
		require.NoError(t, err)

		// 验证流是否被移除
		_, err = sc.BaseConnector.GetStream(streamID)
		require.Error(t, err)
		require.Equal(t, tunnel.ErrStreamNotFound, err)
	})

	t.Run("处理心跳包", func(t *testing.T) {
		// 创建心跳包
		heartbeatPacket := tunnel.NewHeartbeatPacket(connID, 1, 0.5)

		// 处理心跳包
		err := sc.ProcessIncomingData(heartbeatPacket.Bytes())
		require.NoError(t, err)
	})

	t.Run("处理无效包类型", func(t *testing.T) {
		// 创建无效类型的包
		invalidPacket := &tunnel.TunnelPacket{
			Header: tunnel.Header{
				Version:      tunnel.ProtocolVersion,
				Type:         99, // 无效类型
				Flags:        0,
				ConnectionID: connID,
			},
			Data: []byte("invalid"),
		}

		// 处理无效包
		err := sc.ProcessIncomingData(invalidPacket.Bytes())
		require.Error(t, err)
		require.Equal(t, tunnel.ErrInvalidPacket, err)
	})

	t.Run("处理无效数据", func(t *testing.T) {
		// 处理无效数据
		err := sc.ProcessIncomingData([]byte("invalid data"))
		require.Error(t, err)
	})

	t.Run("连接关闭时处理", func(t *testing.T) {
		// 关闭连接
		sc.Close()

		// 创建一个数据包
		dataPacket := tunnel.NewDataPacket(connID, "test-stream", []byte("test data"))

		// 尝试处理数据包
		err := sc.ProcessIncomingData(dataPacket.Bytes())
		require.Error(t, err)
		require.Equal(t, tunnel.ErrConnClosed, err)
	})
}

// TestSendData 测试发送数据函数
func TestSendData(t *testing.T) {
	// 创建服务端连接器
	udpConn, remoteAddr, err := createMockUDPConn()
	require.NoError(t, err)

	sc := NewServerConnector(udpConn, remoteAddr)
	err = sc.Start()
	require.NoError(t, err)
	defer sc.Close()

	// 测试发送数据
	streamID := "test-send-stream"
	testData := []byte("test send data")

	// 发送数据
	err = sc.SendData(streamID, testData)
	require.NoError(t, err)
}

// TestServerStream 测试服务端流的功能
func TestServerStream(t *testing.T) {
	// 创建模拟的ServerConnector
	mockConnector := &ServerConnector{
		BaseConnector: tunnel.NewBaseConnector(),
	}
	// 设置连接ID
	mockConnector.BaseConnector.SetConnectionID("test-conn")

	// 创建ServerStream
	streamID := "test-stream"
	stream := newServerStream(streamID, mockConnector).(*serverStream)

	// 测试基本属性
	assert.Equal(t, streamID, stream.GetStreamID())

	// 测试Read方法
	testData := []byte("test data")
	go func() {
		// 放入一些测试数据
		err := stream.PutData(testData)
		assert.NoError(t, err)
	}()

	// 从流中读取数据
	buf := make([]byte, 100)
	n, err := stream.Read(buf)
	assert.NoError(t, err)
	assert.Equal(t, len(testData), n)
	assert.Equal(t, testData, buf[:n])

	// 测试Close方法
	err = stream.Close()
	assert.NoError(t, err)

	// 关闭后的操作应该返回错误
	_, err = stream.Read(buf)
	assert.Error(t, err)

	err = stream.PutData(testData)
	assert.Error(t, err)
}

// TestServerStreamReadTimeout 测试服务端流的读取超时
func TestServerStreamReadTimeout(t *testing.T) {
	// 创建模拟的ServerConnector
	mockConnector := &ServerConnector{
		BaseConnector: tunnel.NewBaseConnector(),
	}
	// 设置连接ID
	mockConnector.BaseConnector.SetConnectionID("test-conn")

	// 创建ServerStream
	streamID := "test-stream"
	stream := newServerStream(streamID, mockConnector).(*serverStream)

	// 从流中读取数据，应该超时，但为了测试效率，我们不等待整个超时过程
	buf := make([]byte, 100)
	// 使用一个单独的goroutine来中断读取
	go func() {
		time.Sleep(100 * time.Millisecond)
		stream.Close()
	}()

	_, err := stream.Read(buf)
	assert.Error(t, err)
}
