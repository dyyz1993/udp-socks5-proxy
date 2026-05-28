package testing

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"sync"
	"testing"
	"time"

	tunnelClient "github.com/tealife/proxy-cs3/src/tunnel/client"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tealife/proxy-cs3/internal/client"
	"github.com/tealife/proxy-cs3/internal/common"
	"github.com/tealife/proxy-cs3/internal/server"
)

// TestDataAccuracyAndEfficiency 测试数据传输的准确率、错误率和效率
func TestDataAccuracyAndEfficiency(t *testing.T) {
	// 创建日志记录器
	serverLogger := common.NewSimpleLogger("SERVER-PERF", common.InfoLevel)
	clientLogger := common.NewSimpleLogger("CLIENT-PERF", common.InfoLevel)

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

	// 获取客户端的SOCKS5代理地址
	socksAddr := fmt.Sprintf("127.0.0.1:%d", clientPort)
	t.Logf("客户端启动成功，SOCKS5代理地址: %s", socksAddr)

	// 等待客户端完全初始化
	time.Sleep(500 * time.Millisecond)

	// 3. 设置测试参数
	concurrentClients := 10 // 并发客户端数量
	requestsPerClient := 10 // 每个客户端请求数量
	dataSizes := []int{
		1024,        // 1KB
		10 * 1024,   // 10KB
		100 * 1024,  // 100KB
		1024 * 1024, // 1MB
	}

	// 设置测试HTTP服务器，用于测试数据准确性
	httpServer, httpServerURL := setupTestHTTPServer(t)
	defer httpServer.Close()

	// 4. 执行测试
	var wg sync.WaitGroup
	var mu sync.Mutex

	// 统计数据
	type Result struct {
		Success       int
		Failed        int
		DataCorrupted int
		TotalBytes    int64
		TotalTime     time.Duration
	}

	results := make(map[int]*Result)
	for _, size := range dataSizes {
		results[size] = &Result{}
	}

	// 开始计时
	startTime := time.Now()

	for i := 0; i < concurrentClients; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()

			// 为每个客户端创建SOCKS5代理连接
			proxyAddr, err := net.ResolveTCPAddr("tcp", socksAddr)
			if err != nil {
				t.Logf("客户端 %d 解析代理地址失败: %v", clientID, err)
				return
			}

			// 创建使用SOCKS5代理的HTTP客户端
			httpTransport := &http.Transport{
				Proxy: http.ProxyURL(&url.URL{
					Scheme: "socks5",
					Host:   proxyAddr.String(),
				}),
			}

			httpClient := &http.Client{
				Transport: httpTransport,
				Timeout:   10 * time.Second,
			}

			// 执行多次请求
			for j := 0; j < requestsPerClient; j++ {
				// 随机选择一个数据大小
				sizeIndex := rand.Intn(len(dataSizes))
				dataSize := dataSizes[sizeIndex]

				// 构建URL，包含请求的数据大小
				reqURL := fmt.Sprintf("%s/data?size=%d&client=%d&req=%d", httpServerURL, dataSize, clientID, j)

				// 记录开始时间
				reqStart := time.Now()

				// 发送请求
				resp, err := httpClient.Get(reqURL)
				if err != nil {
					mu.Lock()
					results[dataSize].Failed++
					mu.Unlock()
					t.Logf("客户端 %d 请求 %d 失败 (大小: %d): %v", clientID, j, dataSize, err)
					continue
				}

				// 读取响应
				data, err := io.ReadAll(resp.Body)
				resp.Body.Close()

				// 记录请求完成时间
				reqDuration := time.Since(reqStart)

				if err != nil {
					mu.Lock()
					results[dataSize].Failed++
					mu.Unlock()
					t.Logf("客户端 %d 请求 %d 读取响应失败 (大小: %d): %v", clientID, j, dataSize, err)
					continue
				}

				// 检查数据大小是否正确
				if len(data) != dataSize {
					mu.Lock()
					results[dataSize].DataCorrupted++
					mu.Unlock()
					t.Logf("客户端 %d 请求 %d 数据大小不匹配: 期望 %d, 实际 %d", clientID, j, dataSize, len(data))
					continue
				}

				// 检查数据内容是否正确 (通过校验和)
				expectedHash := resp.Header.Get("X-Data-Hash")
				actualHash := calculateMD5(data)

				if expectedHash != actualHash {
					mu.Lock()
					results[dataSize].DataCorrupted++
					mu.Unlock()
					t.Logf("客户端 %d 请求 %d 数据校验和不匹配: 期望 %s, 实际 %s",
						clientID, j, expectedHash, actualHash)
					continue
				}

				// 记录成功的请求
				mu.Lock()
				results[dataSize].Success++
				results[dataSize].TotalBytes += int64(len(data))
				results[dataSize].TotalTime += reqDuration
				mu.Unlock()

				// t.Logf("客户端 %d 请求 %d 成功: 大小 %d 字节, 耗时 %v",
				//    clientID, j, len(data), reqDuration)
			}
		}(i)
	}

	// 等待所有请求完成
	wg.Wait()

	// 计算总测试时间
	totalTestTime := time.Since(startTime)

	// 5. 报告结果
	t.Logf("性能测试完成，总耗时: %v", totalTestTime)
	t.Logf("并发客户端数: %d, 每客户端请求数: %d", concurrentClients, requestsPerClient)

	totalRequests := concurrentClients * requestsPerClient
	var totalSuccess, totalFailed, totalCorrupted int
	var totalBytes int64
	var avgThroughput float64
	var validSizes int

	for _, size := range dataSizes {
		result := results[size]

		// 该大小下请求的数量（估计值，因为随机选择）
		sizeRequests := totalRequests / len(dataSizes)
		successRate := float64(result.Success) / float64(sizeRequests) * 100
		errorRate := float64(result.Failed) / float64(sizeRequests) * 100
		corruptionRate := float64(result.DataCorrupted) / float64(sizeRequests) * 100

		// 计算吞吐量
		var throughput float64
		if result.TotalTime > 0 {
			throughput = float64(result.TotalBytes) / result.TotalTime.Seconds() / 1024 / 1024 // MB/s
		}

		t.Logf("数据大小: %d bytes", size)
		t.Logf("  - 成功率: %.2f%% (%d/%d)", successRate, result.Success, sizeRequests)
		t.Logf("  - 错误率: %.2f%% (%d/%d)", errorRate, result.Failed, sizeRequests)
		t.Logf("  - 数据损坏率: %.2f%% (%d/%d)", corruptionRate, result.DataCorrupted, sizeRequests)
		if result.Success > 0 {
			t.Logf("  - 吞吐量: %.2f MB/s", throughput)
			t.Logf("  - 平均请求时间: %v", result.TotalTime/time.Duration(result.Success))
		} else {
			t.Logf("  - 吞吐量: N/A (无成功请求)")
		}

		totalSuccess += result.Success
		totalFailed += result.Failed
		totalCorrupted += result.DataCorrupted
		totalBytes += result.TotalBytes

		// 累计吞吐量（只统计成功的大小）
		if result.Success > 0 {
			avgThroughput += throughput
			validSizes++
		}
	}

	// 总体统计
	t.Logf("\n总体统计:")
	t.Logf("  - 总请求数: %d", totalRequests)
	t.Logf("  - 总成功数: %d (%.2f%%)", totalSuccess, float64(totalSuccess)/float64(totalRequests)*100)
	t.Logf("  - 总失败数: %d (%.2f%%)", totalFailed, float64(totalFailed)/float64(totalRequests)*100)
	t.Logf("  - 总数据损坏数: %d (%.2f%%)", totalCorrupted, float64(totalCorrupted)/float64(totalRequests)*100)
	t.Logf("  - 总传输数据: %.2f MB", float64(totalBytes)/1024/1024)
	t.Logf("  - 平均吞吐量: %.2f MB/s", avgThroughput/float64(validSizes))
	t.Logf("  - 总体传输速率: %.2f MB/s", float64(totalBytes)/totalTestTime.Seconds()/1024/1024)

	// 验证测试结果满足最低标准
	successRate := float64(totalSuccess) / float64(totalRequests) * 100
	assert.GreaterOrEqual(t, successRate, 90.0, "总成功率应至少为90%")

	corruptionRate := float64(totalCorrupted) / float64(totalRequests) * 100
	assert.LessOrEqual(t, corruptionRate, 1.0, "数据损坏率应低于1%")
}

// 创建一个测试HTTP服务器，用于生成随机数据
func setupTestHTTPServer(t *testing.T) (*http.Server, string) {
	// 创建一个HTTP服务器监听随机端口
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	mux := http.NewServeMux()
	mux.HandleFunc("/data", func(w http.ResponseWriter, r *http.Request) {
		// 解析查询参数
		query := r.URL.Query()
		sizeStr := query.Get("size")
		clientID := query.Get("client")
		reqID := query.Get("req")

		// 转换大小参数
		var size int
		_, err := fmt.Sscanf(sizeStr, "%d", &size)
		if err != nil || size <= 0 {
			http.Error(w, "无效的大小参数", http.StatusBadRequest)
			return
		}

		// 生成指定大小的随机数据
		data := generateRandomData(size, clientID, reqID)

		// 计算数据哈希值，用于校验
		hash := calculateMD5(data)

		// 设置响应头
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", size))
		w.Header().Set("X-Data-Hash", hash)

		// 写入响应
		w.Write(data)
	})

	// 启动HTTP服务器
	server := &http.Server{Handler: mux}
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			t.Logf("HTTP服务器错误: %v", err)
		}
	}()

	// 获取服务器URL
	url := fmt.Sprintf("http://%s", listener.Addr().String())
	t.Logf("测试HTTP服务器启动成功: %s", url)

	return server, url
}

// 生成随机数据
func generateRandomData(size int, clientID, reqID string) []byte {
	// 创建指定大小的字节切片
	data := make([]byte, size)

	// 使用确定性随机数生成器填充数据，确保同一请求的数据一致
	seed := clientID + reqID
	r := rand.New(rand.NewSource(int64(hash(seed))))

	// 填充随机数据
	r.Read(data)

	return data
}

// 计算数据的MD5哈希值
func calculateMD5(data []byte) string {
	hash := md5.New()
	hash.Write(data)
	return hex.EncodeToString(hash.Sum(nil))
}

// 简单的字符串哈希函数
func hash(s string) uint32 {
	h := uint32(0)
	for i := 0; i < len(s); i++ {
		h = h*31 + uint32(s[i])
	}
	return h
}

// TestNetworkStability 测试不同网络条件下的稳定性
func TestNetworkStability(t *testing.T) {
	// 该测试暂时忽略，需要实际网络环境来测试网络稳定性
	t.Skip("该测试需要在实际网络环境中运行")
}

// TestUDPMessageTooLong 测试不同大小的UDP消息，找出何时会出现"message too long"错误
func TestUDPMessageTooLong(t *testing.T) {
	// 创建UDP地址
	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	require.NoError(t, err, "解析UDP地址失败")

	// 创建UDP监听器
	conn, err := net.ListenUDP("udp", addr)
	require.NoError(t, err, "创建UDP监听失败")
	defer conn.Close()

	// 创建发送连接
	sender, err := net.DialUDP("udp", nil, conn.LocalAddr().(*net.UDPAddr))
	require.NoError(t, err, "创建发送连接失败")
	defer sender.Close()

	t.Logf("开始测试UDP消息大小限制...")

	// 测试不同大小的消息
	messageSizes := []int{
		512,       // 0.5KB
		1024,      // 1KB
		8 * 1024,  // 8KB
		16 * 1024, // 16KB
		32 * 1024, // 32KB
		64 * 1024, // 64KB
	}

	for _, size := range messageSizes {
		// 创建指定大小的消息
		message := make([]byte, size)
		// 填充一些数据以便识别
		for i := 0; i < size; i++ {
			message[i] = byte(i % 256)
		}

		// 尝试发送消息
		t.Logf("尝试发送 %d 字节的UDP消息...", size)
		_, err := sender.Write(message)

		if err != nil {
			t.Logf("发送 %d 字节消息失败: %v", size, err)
		} else {
			t.Logf("成功发送 %d 字节的UDP消息", size)

			// 尝试接收消息，验证是否完整接收
			received := make([]byte, size+100) // 多分配一些空间
			conn.SetReadDeadline(time.Now().Add(1 * time.Second))
			n, _, err := conn.ReadFromUDP(received)

			if err != nil {
				t.Logf("接收 %d 字节消息失败: %v", size, err)
			} else {
				t.Logf("成功接收 %d 字节的UDP消息", n)
				// 验证接收到的数据大小
				if n != size {
					t.Logf("警告: 接收到的消息大小(%d)与发送的大小(%d)不一致", n, size)
				}
			}
		}
	}

	// 验证完成
	t.Logf("UDP消息大小限制测试完成")
}

// TestPreciseUDPSizeLimit 使用二分查找确定精确的UDP大小限制
func TestPreciseUDPSizeLimit(t *testing.T) {
	// 创建UDP地址
	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	require.NoError(t, err, "解析UDP地址失败")

	// 创建UDP监听器
	conn, err := net.ListenUDP("udp", addr)
	require.NoError(t, err, "创建UDP监听失败")
	defer conn.Close()

	// 创建发送连接
	sender, err := net.DialUDP("udp", nil, conn.LocalAddr().(*net.UDPAddr))
	require.NoError(t, err, "创建发送连接失败")
	defer sender.Close()

	t.Logf("开始精确测定UDP消息大小限制...")

	// 二分查找的范围
	low := 8000   // 确定能成功的值
	high := 17000 // 确定会失败的值

	var exactLimit int

	// 二分查找循环
	for low <= high {
		mid := (low + high) / 2

		// 创建并填充消息
		message := make([]byte, mid)
		for i := 0; i < mid; i++ {
			message[i] = byte(i % 256)
		}

		// 尝试发送消息
		t.Logf("测试 %d 字节的UDP消息...", mid)
		_, err := sender.Write(message)

		if err != nil {
			t.Logf("  失败: %v", err)
			// 如果发送失败，尝试更小的大小
			high = mid - 1
		} else {
			t.Logf("  成功")
			// 如果发送成功，尝试更大的大小
			low = mid + 1
			exactLimit = mid // 记录最后一个成功的大小
		}
	}

	// 再次确认找到的限制
	if exactLimit > 0 {
		t.Logf("确认的UDP消息大小限制: %d 字节", exactLimit)

		// 尝试发送exactLimit大小的消息（应该成功）
		message := make([]byte, exactLimit)
		_, err := sender.Write(message)
		if err != nil {
			t.Logf("  确认测试失败 (%d 字节): %v", exactLimit, err)
		} else {
			t.Logf("  确认测试成功 (%d 字节)", exactLimit)
		}

		// 尝试发送exactLimit+1大小的消息（应该失败）
		message = make([]byte, exactLimit+1)
		_, err = sender.Write(message)
		if err != nil {
			t.Logf("  确认测试成功 (%d 字节超限): %v", exactLimit+1, err)
		} else {
			t.Logf("  确认测试失败 (%d 字节超限但发送成功)", exactLimit+1)
		}
	} else {
		t.Logf("未能确定精确的UDP消息大小限制")
	}
}

// TestVirtualSocks5LargeWrite 测试VirtualSocks5Conn对大数据包的分块写入
func TestVirtualSocks5LargeWrite(t *testing.T) {
	// 设置超时时间为10秒
	timeout := time.After(10 * time.Second)
	done := make(chan bool)

	go func() {
		// 创建一个测试日志记录器
		logger := &testLogger{t: t}

		// 创建一个管道模拟连接
		clientConn, serverConn := net.Pipe()
		defer clientConn.Close()
		defer serverConn.Close()

		// 模拟SOCKS5握手数据
		originalData := []byte{0x05, 0x01, 0x00}

		// 创建VirtualSocks5Conn
		vconn := tunnelClient.NewVirtualSocks5Conn(clientConn, originalData, logger)

		// 在goroutine中读取数据，模拟另一端接收
		dataCh := make(chan []byte, 10)
		errCh := make(chan error, 1)
		readDone := make(chan bool)

		go func() {
			defer close(dataCh)
			defer close(errCh)
			defer close(readDone)

			// 先处理握手请求
			handshake := make([]byte, 3)
			n, err := serverConn.Read(handshake)
			if err != nil {
				errCh <- fmt.Errorf("读取握手请求失败: %v", err)
				return
			}
			t.Logf("收到握手请求: %x (%d字节)", handshake[:n], n)

			// 发送认证响应
			_, err = serverConn.Write([]byte{0x05, 0x00})
			if err != nil {
				errCh <- fmt.Errorf("发送认证响应失败: %v", err)
				return
			}

			// 读取分块传输的大数据包
			totalReceived := 0
			buffer := make([]byte, 0, 16000)

			for {
				chunk := make([]byte, 10000)
				n, err := serverConn.Read(chunk)
				if err != nil {
					if err == io.EOF {
						break
					}
					errCh <- fmt.Errorf("读取数据失败: %v", err)
					return
				}

				t.Logf("收到数据块: %d字节", n)
				buffer = append(buffer, chunk[:n]...)
				totalReceived += n

				// 我们只需要接收完整的数据
				if totalReceived >= 12000 {
					break
				}
			}

			dataCh <- buffer
			readDone <- true
		}()

		// 让读取goroutine有时间启动
		time.Sleep(100 * time.Millisecond)

		// 发送认证请求，触发Read方法
		var readBuf [10]byte
		_, err := vconn.Read(readBuf[:])
		require.NoError(t, err, "读取操作失败")

		// 等待认证响应处理完成
		time.Sleep(100 * time.Millisecond)

		// 创建较小的大数据包 (>8000字节但不要太大)
		largeData := make([]byte, 12000)
		for i := range largeData {
			largeData[i] = byte(i % 256)
		}

		// 写入大数据包，应该触发分块写入
		t.Logf("开始写入大数据包，大小: %d字节", len(largeData))
		n, err := vconn.Write(largeData)
		require.NoError(t, err, "写入大数据包失败")
		require.Equal(t, len(largeData), n, "写入字节数不匹配")

		// 等待接收完成，带超时
		select {
		case err := <-errCh:
			t.Logf("接收数据过程出错: %v", err)
			t.Errorf("接收数据过程出错: %v", err)
		case receivedData := <-dataCh:
			t.Logf("接收数据完成，大小: %d字节", len(receivedData))

			// 验证数据完整性
			require.Equal(t, len(largeData), len(receivedData), "接收的数据大小不匹配")

			// 验证前100个和后100个字节，确保数据正确传输
			require.Equal(t, largeData[:100], receivedData[:100], "前100个字节不匹配")
			require.Equal(t, largeData[len(largeData)-100:], receivedData[len(receivedData)-100:], "后100个字节不匹配")
		case <-time.After(5 * time.Second):
			t.Logf("接收数据超时")
			t.Errorf("接收数据超时")
		}

		// 等待读取完成
		select {
		case <-readDone:
			// 正常完成
		case <-time.After(1 * time.Second):
			t.Logf("等待读取完成超时")
			t.Errorf("等待读取完成超时")
		}

		t.Log("大数据包分块写入测试成功")
		done <- true
	}()

	// 外层超时控制
	select {
	case <-timeout:
		t.Logf("测试整体超时")
		t.Errorf("测试整体超时")
	case <-done:
		// 测试正常完成
	}
}

// testLogger 实现了Logger接口的测试日志记录器
type testLogger struct {
	t *testing.T
}

func (l *testLogger) Debug(args ...interface{}) {
	l.t.Log(args...)
}

func (l *testLogger) Debugf(format string, args ...interface{}) {
	l.t.Logf(format, args...)
}

func (l *testLogger) Info(args ...interface{}) {
	l.t.Log(args...)
}

func (l *testLogger) Infof(format string, args ...interface{}) {
	l.t.Logf(format, args...)
}

func (l *testLogger) Error(args ...interface{}) {
	l.t.Error(args...)
}

func (l *testLogger) Errorf(format string, args ...interface{}) {
	l.t.Errorf(format, args...)
}
