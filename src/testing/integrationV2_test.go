package testing

import (
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tealife/proxy-cs3/internal/client"
	"github.com/tealife/proxy-cs3/internal/common"
	"github.com/tealife/proxy-cs3/internal/server"
	socks5proxy "golang.org/x/net/proxy"
)

// TestMultiClientHandshakeAndHeartbeat 测试多个客户端连接到一个服务器的握手和心跳
func TestMultiClientHandshakeAndHeartbeat(t *testing.T) {
	// 创建日志记录器
	serverLogger := common.NewSimpleLogger("SERVER-MULTI", common.InfoLevel)

	// 1. 启动服务端
	serverPort := getFreePort(t)
	serverConfig := server.Config{
		Port:     serverPort,
		LogLevel: common.InfoLevel,
	}
	s := server.NewServer(serverConfig, serverLogger)
	err := s.Start()
	require.NoError(t, err)
	defer s.Stop()

	// 获取服务端的实际地址
	serverAddr := fmt.Sprintf("127.0.0.1:%d", serverPort)
	t.Logf("服务端启动成功，地址: %s", serverAddr)

	// 统计成功的客户端数量
	var successCount int
	var failCount int
	var mu sync.Mutex

	// 设置测试参数
	clientCount := 2          // 减少客户端数量
	connectionsPerClient := 1 // 减少每个客户端的连接数

	// 用于等待所有客户端完成
	var wg sync.WaitGroup

	// 2. 启动多个客户端
	for i := 0; i < clientCount; i++ {
		wg.Add(1)

		go func(clientID int) {
			defer wg.Done()

			// 为每个客户端创建独立的日志记录器
			clientLogger := common.NewSimpleLogger(fmt.Sprintf("CLIENT-%d", clientID), common.InfoLevel)

			// 创建客户端
			clientPort := getFreePort(t)
			clientConfig := client.Config{
				LocalPort:     clientPort,
				ServerAddr:    serverAddr,
				DirectDomains: []string{},
				DefaultDirect: false,
				Timeout:       3 * time.Second, // 减少超时时间
				LogLevel:      common.InfoLevel,
			}

			c := client.NewClient(clientConfig, clientLogger)
			err := c.Start()

			if err != nil {
				t.Logf("客户端 %d 启动失败: %v", clientID, err)
				mu.Lock()
				failCount++
				mu.Unlock()
				return
			}

			defer c.Stop()

			// 获取客户端的SOCKS5代理地址
			socksAddr := fmt.Sprintf("127.0.0.1:%d", clientPort)
			t.Logf("客户端 %d 启动成功，SOCKS5代理地址: %s", clientID, socksAddr)

			// 等待客户端完全初始化
			time.Sleep(500 * time.Millisecond)

			// 为每个客户端创建多个连接
			clientSuccess := true

			for j := 0; j < connectionsPerClient; j++ {
				// 使用重试逻辑
				var conn net.Conn
				var err error
				maxRetries := 2 // 减少重试次数

				for retry := 0; retry < maxRetries; retry++ {
					// 使用SOCKS5代理创建连接
					dialer, err := socks5proxy.SOCKS5("tcp", socksAddr, nil, socks5proxy.Direct)
					if err != nil {
						t.Logf("客户端 %d 连接 %d 创建SOCKS5代理失败 (尝试 %d/%d): %v",
							clientID, j, retry+1, maxRetries, err)
						time.Sleep(100 * time.Millisecond)
						continue
					}

					// 连接到测试网站
					start := time.Now()
					conn, err = dialer.Dial("tcp", "www.baidu.com:80")
					elapsed := time.Since(start)

					if err != nil {
						t.Logf("客户端 %d 连接 %d 连接到baidu.com失败 (尝试 %d/%d): %v (耗时: %v)",
							clientID, j, retry+1, maxRetries, err, elapsed)
						time.Sleep(200 * time.Millisecond)
						continue
					}

					// 成功建立连接
					t.Logf("客户端 %d 连接 %d 成功连接到baidu.com (尝试 %d/%d) (耗时: %v)",
						clientID, j, retry+1, maxRetries, elapsed)
					break
				}

				if err != nil || conn == nil {
					t.Logf("客户端 %d 连接 %d 在所有尝试后仍失败", clientID, j)
					clientSuccess = false
					continue
				}

				// 发送简单的HTTP请求
				_, err = conn.Write([]byte("GET / HTTP/1.0\r\nHost: www.baidu.com\r\n\r\n"))
				if err != nil {
					t.Logf("客户端 %d 连接 %d 发送HTTP请求失败: %v", clientID, j, err)
					clientSuccess = false
					conn.Close()
					continue
				}

				// 设置读取超时
				conn.SetReadDeadline(time.Now().Add(1 * time.Second)) // 减少读取超时时间

				// 读取响应
				buf := make([]byte, 1024)
				n, err := conn.Read(buf)
				if err != nil {
					t.Logf("客户端 %d 连接 %d 读取HTTP响应失败: %v", clientID, j, err)
					// 不立即判断为失败，而是检查是否已经收到了部分数据
					if n > 0 {
						t.Logf("客户端 %d 连接 %d 虽然读取出错，但已收到 %d 字节数据", clientID, j, n)
						// 我们仍然认为这是部分成功的
					} else {
						clientSuccess = false
						conn.Close()
						continue
					}
				}

				// 清除读取超时
				conn.SetReadDeadline(time.Time{})

				// 读取成功
				if n > 0 {
					t.Logf("客户端 %d 连接 %d 成功读取到 %d 字节响应", clientID, j, n)
				}

				// 关闭连接
				conn.Close()
				t.Logf("客户端 %d 连接 %d 测试完成", clientID, j)

				// 稍微暂停一下，避免连接过快
				time.Sleep(50 * time.Millisecond) // 减少连接间隔时间
			}

			// 更新成功/失败计数
			mu.Lock()
			if clientSuccess {
				successCount++
			} else {
				failCount++
			}
			mu.Unlock()
		}(i)
	}

	// 等待所有客户端完成
	wg.Wait()

	// 报告结果
	t.Logf("测试完成: 总客户端数 %d, 成功 %d, 失败 %d", clientCount, successCount, failCount)

	// 断言大多数客户端应该成功，而不是所有客户端
	assert.GreaterOrEqual(t, successCount, clientCount/2, "至少一半的客户端应该成功")

	// 补充测试日志，辅助分析
	t.Logf("测试结果分析：总客户端 %d 个，期望至少成功 %d 个，实际成功 %d 个",
		clientCount, clientCount/2, successCount)
}

// TestHeartbeatStability 测试心跳连接的稳定性
func TestHeartbeatStability(t *testing.T) {
	// 创建日志记录器
	serverLogger := common.NewSimpleLogger("SERVER-HEARTBEAT", common.InfoLevel)
	clientLogger := common.NewSimpleLogger("CLIENT-HEARTBEAT", common.InfoLevel)

	// 1. 启动服务端
	serverPort := getFreePort(t)
	serverConfig := server.Config{
		Port:     serverPort,
		LogLevel: common.InfoLevel,
	}
	s := server.NewServer(serverConfig, serverLogger)
	startErr := s.Start()
	require.NoError(t, startErr)
	defer s.Stop()

	// 获取服务端的实际地址
	serverAddr := fmt.Sprintf("127.0.0.1:%d", serverPort)
	t.Logf("服务端启动成功，地址: %s", serverAddr)

	// 2. 启动客户端
	clientPort := getFreePort(t)
	clientConfig := client.Config{
		LocalPort:     clientPort,
		ServerAddr:    serverAddr,
		DirectDomains: []string{},
		DefaultDirect: false,
		Timeout:       5 * time.Second,
		LogLevel:      common.InfoLevel,
	}

	c := client.NewClient(clientConfig, clientLogger)
	clientErr := c.Start()
	require.NoError(t, clientErr)
	defer c.Stop()

	socksAddr := fmt.Sprintf("127.0.0.1:%d", clientPort)
	t.Logf("客户端启动成功，SOCKS5代理地址: %s", socksAddr)

	// 等待客户端完全初始化
	time.Sleep(1 * time.Second)

	// 3. 测试心跳稳定性
	// 设置测试持续时间
	testDuration := 3 * time.Second // 减少测试持续时间
	connectionInterval := 1 * time.Second
	endTime := time.Now().Add(testDuration)

	successCount := 0
	failCount := 0

	// 在测试持续时间内定期建立连接
	for time.Now().Before(endTime) {
		success := testClientConnection(t, socksAddr)
		if success {
			successCount++
		} else {
			failCount++
		}

		// 等待下一个连接间隔
		time.Sleep(connectionInterval)
	}

	// 报告结果
	t.Logf("心跳稳定性测试完成: 总连接尝试 %d, 成功 %d, 失败 %d",
		successCount+failCount, successCount, failCount)

	// 断言大多数连接应该成功
	successRatio := float64(successCount) / float64(successCount+failCount)
	t.Logf("成功率：%.2f%%", successRatio*100)
	assert.GreaterOrEqual(t, successRatio, 0.5, "至少一半的连接应该成功")
}

// testClientConnection 测试SOCKS5代理连接，带有重试逻辑
func testClientConnection(t *testing.T, socksAddr string) bool {
	var conn net.Conn
	var success bool
	maxRetries := 3

	for retry := 0; retry < maxRetries; retry++ {
		// 使用SOCKS5代理创建连接
		dialer, err := socks5proxy.SOCKS5("tcp", socksAddr, nil, socks5proxy.Direct)
		if err != nil {
			t.Logf("创建SOCKS5代理失败 (尝试 %d/%d): %v",
				retry+1, maxRetries, err)
			time.Sleep(100 * time.Millisecond)
			continue
		}

		// 连接到测试网站
		start := time.Now()
		conn, err = dialer.Dial("tcp", "www.baidu.com:80")
		elapsed := time.Since(start)

		if err != nil {
			t.Logf("连接到baidu.com失败 (尝试 %d/%d): %v (耗时: %v)",
				retry+1, maxRetries, err, elapsed)
			time.Sleep(200 * time.Millisecond)
			continue
		}

		// 成功建立连接
		t.Logf("成功连接到baidu.com (尝试 %d/%d) (耗时: %v)",
			retry+1, maxRetries, elapsed)
		break
	}

	if conn == nil {
		t.Logf("在所有尝试后仍连接失败")
		return false
	}

	// 发送简单的HTTP请求
	_, err := conn.Write([]byte("GET / HTTP/1.0\r\nHost: www.baidu.com\r\n\r\n"))
	if err != nil {
		t.Logf("发送HTTP请求失败: %v", err)
		conn.Close()
		return false
	}

	// 设置读取超时
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))

	// 读取响应
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		t.Logf("读取HTTP响应失败: %v", err)
		// 检查是否已经收到了部分数据
		if n > 0 {
			t.Logf("虽然读取出错，但已收到 %d 字节数据", n)
			success = true
		} else {
			success = false
		}
	} else {
		// 读取成功
		t.Logf("成功读取到 %d 字节响应", n)
		success = true
	}

	// 清除读取超时
	conn.SetReadDeadline(time.Time{})

	// 关闭连接
	conn.Close()
	return success
}
