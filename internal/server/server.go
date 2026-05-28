package server

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/tealife/proxy-cs3/internal/common"
	"github.com/tealife/proxy-cs3/src/tunnel/server"
)

// Config 服务端配置
type Config struct {
	// 监听端口
	Port int

	// 日志级别
	LogLevel common.LogLevel
}

// Server SOCKS5代理服务端
type Server struct {
	config     Config
	logger     common.Logger
	udpConn    *net.UDPConn
	clientConn map[string]*server.ServerConnector

	isRunning bool
	closeChan chan struct{}
	wg        sync.WaitGroup
	mu        sync.Mutex
}

// NewServer 创建一个新的服务端
func NewServer(config Config, logger common.Logger) *Server {
	if logger == nil {
		logger = common.NewSimpleLogger("SERVER", config.LogLevel)
	}

	return &Server{
		config:     config,
		logger:     logger,
		clientConn: make(map[string]*server.ServerConnector),
		closeChan:  make(chan struct{}),
	}
}

// Start 启动服务端
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.isRunning {
		return nil
	}

	s.logger.Info("启动服务端...")

	// 创建UDP监听
	addr := fmt.Sprintf("0.0.0.0:%d", s.config.Port)
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return fmt.Errorf("解析UDP地址失败: %v", err)
	}

	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return fmt.Errorf("监听UDP失败: %v", err)
	}

	s.udpConn = conn
	s.isRunning = true

	// 启动UDP处理
	s.wg.Add(1)
	go s.handleUDP()

	s.logger.Infof("服务端已启动，监听地址: %s", addr)

	return nil
}

// Stop 停止服务端
func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isRunning {
		return nil
	}

	s.logger.Info("停止服务端...")

	// 关闭UDP连接
	if s.udpConn != nil {
		s.udpConn.Close()
	}

	// 关闭所有客户端连接器
	for _, conn := range s.clientConn {
		conn.Close()
	}

	close(s.closeChan)
	s.isRunning = false

	// 等待所有协程退出
	s.wg.Wait()

	s.logger.Info("服务端已停止")

	return nil
}

// handleUDP 处理UDP数据
func (s *Server) handleUDP() {

	defer s.wg.Done()

	buffer := make([]byte, 65536)

	for {
		s.udpConn.SetReadDeadline(time.Now().Add(5 * time.Second))

		n, addr, err := s.udpConn.ReadFromUDP(buffer)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				// 读取超时，检查是否应该退出
				select {
				case <-s.closeChan:
					return
				default:
					continue
				}
			}

			select {
			case <-s.closeChan:
				return
			default:
				s.logger.Errorf("读取UDP数据失败: %v", err)
				continue
			}
		}

		// 处理数据
		s.wg.Add(1)
		go func(data []byte, clientAddr *net.UDPAddr) {
			defer s.wg.Done()
			s.logger.Debug("开始处理UDP数据")
			// 十六进制数据
			s.logger.Debugf("十六进制数据: %x", data)
			// 处理数据包
			if err := s.processPacket(data, clientAddr); err != nil {
				s.logger.Errorf("处理数据包失败: %v", err)
			}
		}(buffer[:n], addr)
	}
}

// processPacket 处理数据包
func (s *Server) processPacket(data []byte, clientAddr *net.UDPAddr) error {
	// 解析数据包
	_, err := s.parsePacket(data)
	if err != nil {
		return fmt.Errorf("解析数据包失败: %v", err)
	}

	// 获取或创建客户端连接器
	clientAddrKey := clientAddr.String()
	s.mu.Lock()
	clientConn, ok := s.clientConn[clientAddrKey]
	if !ok {
		// 创建新的连接器
		clientConn = server.NewServerConnector(s.udpConn, clientAddr)
		if err := clientConn.Start(); err != nil {
			s.mu.Unlock()
			return fmt.Errorf("启动连接器失败: %v", err)
		}

		// 添加到映射
		s.clientConn[clientAddrKey] = clientConn
		s.logger.Debugf("创建新连接器，客户端地址: %s", clientAddrKey)
	}
	s.mu.Unlock()

	// 处理数据包
	if err := clientConn.ProcessIncomingData(data); err != nil {
		return fmt.Errorf("处理数据失败: %v", err)
	}

	return nil
}

// parsePacket 简单解析数据包，验证有效性
func (s *Server) parsePacket(data []byte) (interface{}, error) {
	if len(data) < 5 {
		return nil, fmt.Errorf("数据包太短")
	}

	// 简单返回，实际解析在ProcessIncomingData中完成
	return data, nil
}
