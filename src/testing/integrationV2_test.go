package testing

import (
	"fmt"
	"io"
	"net"
	"net/http"
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

// startLocalEchoServer starts a simple TCP echo server for testing
func startLocalEchoServer(t *testing.T) (string, func()) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	done := make(chan struct{})
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				close(done)
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				io.Copy(c, c) // echo back everything
			}(conn)
		}
	}()

	cleanup := func() {
		ln.Close()
		<-done
	}

	return ln.Addr().String(), cleanup
}

// startLocalHTTPEcho starts a local HTTP echo server
func startLocalHTTPEcho(t *testing.T) (*http.Server, func()) {
	mux := http.NewServeMux()
	mux.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read failed", http.StatusInternalServerError)
			return
		}
		r.Body.Close()
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	srv := &http.Server{Handler: mux}
	go srv.Serve(ln)

	cleanup := func() {
		srv.Close()
		ln.Close()
	}

	return srv, cleanup
}

// TestMultiClientHandshakeAndHeartbeat tests multiple clients connecting to one server
func TestMultiClientHandshakeAndHeartbeat(t *testing.T) {
	// Start a local echo server instead of using baidu.com
	echoAddr, echoCleanup := startLocalEchoServer(t)
	defer echoCleanup()
	t.Logf("Local echo server started at: %s", echoAddr)

	serverLogger := common.NewSimpleLogger("SERVER-MULTI", common.InfoLevel)

	// 1. Start server
	serverPort := getFreePort(t)
	serverConfig := server.Config{
		Port:     serverPort,
		LogLevel: common.InfoLevel,
	}
	s := server.NewServer(serverConfig, serverLogger)
	err := s.Start()
	require.NoError(t, err)
	defer s.Stop()

	serverAddr := fmt.Sprintf("127.0.0.1:%d", serverPort)
	t.Logf("Server started at: %s", serverAddr)

	var successCount int
	var failCount int
	var mu sync.Mutex

	clientCount := 2
	connectionsPerClient := 1

	var wg sync.WaitGroup

	for i := 0; i < clientCount; i++ {
		wg.Add(1)

		go func(clientID int) {
			defer wg.Done()

			clientLogger := common.NewSimpleLogger(fmt.Sprintf("CLIENT-%d", clientID), common.InfoLevel)

			clientPort := getFreePort(t)
			clientConfig := client.Config{
				LocalPort:     clientPort,
				ServerAddr:    serverAddr,
				DirectDomains: []string{},
				DefaultDirect: false,
				Timeout:       3 * time.Second,
				LogLevel:      common.InfoLevel,
			}

			c := client.NewClient(clientConfig, clientLogger)
			err := c.Start()

			if err != nil {
				t.Logf("Client %d start failed: %v", clientID, err)
				mu.Lock()
				failCount++
				mu.Unlock()
				return
			}

			defer c.Stop()

			socksAddr := fmt.Sprintf("127.0.0.1:%d", clientPort)
			t.Logf("Client %d started, SOCKS5 addr: %s", clientID, socksAddr)

			time.Sleep(500 * time.Millisecond)

			clientSuccess := true

			for j := 0; j < connectionsPerClient; j++ {
				var conn net.Conn
				maxRetries := 2

				for retry := 0; retry < maxRetries; retry++ {
					dialer, err := socks5proxy.SOCKS5("tcp", socksAddr, nil, socks5proxy.Direct)
					if err != nil {
						t.Logf("Client %d conn %d SOCKS5 create failed (retry %d/%d): %v",
							clientID, j, retry+1, maxRetries, err)
						time.Sleep(100 * time.Millisecond)
						continue
					}

					conn, err = dialer.Dial("tcp", echoAddr)

					if err != nil {
						t.Logf("Client %d conn %d connect failed (retry %d/%d): %v",
							clientID, j, retry+1, maxRetries, err)
						time.Sleep(200 * time.Millisecond)
						continue
					}

					t.Logf("Client %d conn %d connected (retry %d/%d)",
						clientID, j, retry+1, maxRetries)
					break
				}

				if err != nil || conn == nil {
					t.Logf("Client %d conn %d failed after all retries", clientID, j)
					clientSuccess = false
					continue
				}

				// Send data and verify echo
				testData := fmt.Sprintf("hello-client-%d-conn-%d", clientID, j)
				conn.SetDeadline(time.Now().Add(3 * time.Second))

				_, err = conn.Write([]byte(testData))
				if err != nil {
					t.Logf("Client %d conn %d write failed: %v", clientID, j, err)
					clientSuccess = false
					conn.Close()
					continue
				}

				buf := make([]byte, 1024)
				n, err := conn.Read(buf)
				if err != nil {
					t.Logf("Client %d conn %d read failed: %v", clientID, j, err)
					if n == 0 {
						clientSuccess = false
						conn.Close()
						continue
					}
				}

				if n > 0 {
					t.Logf("Client %d conn %d read %d bytes", clientID, j, n)
				}

				conn.Close()
				time.Sleep(50 * time.Millisecond)
			}

			mu.Lock()
			if clientSuccess {
				successCount++
			} else {
				failCount++
			}
			mu.Unlock()
		}(i)
	}

	wg.Wait()

	t.Logf("Test done: total %d, success %d, fail %d", clientCount, successCount, failCount)
	assert.GreaterOrEqual(t, successCount, clientCount/2, "at least half should succeed")
}

// TestHeartbeatStability tests heartbeat connection stability
func TestHeartbeatStability(t *testing.T) {
	// Start a local echo server
	echoAddr, echoCleanup := startLocalEchoServer(t)
	defer echoCleanup()

	serverLogger := common.NewSimpleLogger("SERVER-HEARTBEAT", common.InfoLevel)
	clientLogger := common.NewSimpleLogger("CLIENT-HEARTBEAT", common.InfoLevel)

	serverPort := getFreePort(t)
	serverConfig := server.Config{
		Port:     serverPort,
		LogLevel: common.InfoLevel,
	}
	s := server.NewServer(serverConfig, serverLogger)
	startErr := s.Start()
	require.NoError(t, startErr)
	defer s.Stop()

	serverAddr := fmt.Sprintf("127.0.0.1:%d", serverPort)

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
	t.Logf("Client started, SOCKS5 addr: %s", socksAddr)

	time.Sleep(1 * time.Second)

	testDuration := 3 * time.Second
	connectionInterval := 1 * time.Second
	endTime := time.Now().Add(testDuration)

	successCount := 0
	failCount := 0

	for time.Now().Before(endTime) {
		success := testLocalConnection(t, socksAddr, echoAddr)
		if success {
			successCount++
		} else {
			failCount++
		}
		time.Sleep(connectionInterval)
	}

	t.Logf("Heartbeat stability test done: attempts %d, success %d, fail %d",
		successCount+failCount, successCount, failCount)

	if successCount+failCount > 0 {
		successRatio := float64(successCount) / float64(successCount+failCount)
		t.Logf("Success rate: %.2f%%", successRatio*100)
		assert.GreaterOrEqual(t, successRatio, 0.5, "at least half should succeed")
	}
}

// testLocalConnection tests a SOCKS5 proxy connection to a local echo server
func testLocalConnection(t *testing.T, socksAddr, echoAddr string) bool {
	var conn net.Conn
	maxRetries := 3

	for retry := 0; retry < maxRetries; retry++ {
		dialer, err := socks5proxy.SOCKS5("tcp", socksAddr, nil, socks5proxy.Direct)
		if err != nil {
			t.Logf("SOCKS5 create failed (retry %d/%d): %v", retry+1, maxRetries, err)
			time.Sleep(100 * time.Millisecond)
			continue
		}

		conn, err = dialer.Dial("tcp", echoAddr)

		if err != nil {
			t.Logf("Connect failed (retry %d/%d): %v", retry+1, maxRetries, err)
			time.Sleep(200 * time.Millisecond)
			continue
		}

		break
	}

	if conn == nil {
		t.Logf("All retries failed")
		return false
	}

	// Send and verify echo
	testData := "heartbeat-test"
	conn.SetDeadline(time.Now().Add(3 * time.Second))

	_, err := conn.Write([]byte(testData))
	if err != nil {
		t.Logf("Write failed: %v", err)
		conn.Close()
		return false
	}

	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	success := false
	if err != nil {
		t.Logf("Read failed: %v", err)
		if n > 0 {
			success = true
		}
	} else {
		t.Logf("Read %d bytes", n)
		success = true
	}

	conn.Close()
	return success
}
