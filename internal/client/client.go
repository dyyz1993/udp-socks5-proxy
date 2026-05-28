package client

import (
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/tealife/proxy-cs3/internal/common"
	tunnelclient "github.com/tealife/proxy-cs3/src/tunnel/client"

	gsocks "github.com/things-go/go-socks5"
)

// Config 客户端配置
type Config struct {
	// 本地监听端口
	LocalPort int

	// 服务器地址
	ServerAddr string

	// 直连域名规则
	DirectDomains []string

	// 默认直连策略
	DefaultDirect bool

	// 连接超时
	Timeout time.Duration

	// 日志级别
	LogLevel common.LogLevel
}

// Client SOCKS5代理客户端
type Client struct {
	config Config
	logger common.Logger

	tunnelConnector *tunnelclient.ClientConnector
	ruleEngine      *RuleEngine

	listener net.Listener
	wg       sync.WaitGroup

	isRunning bool
	closeChan chan struct{}
	mu        sync.Mutex
}

// NewClient 创建一个新的客户端
func NewClient(config Config, logger common.Logger) *Client {
	if logger == nil {
		logger = common.NewSimpleLogger("CLIENT", config.LogLevel)
	}

	ruleEngine := NewRuleEngine(config.DirectDomains, config.DefaultDirect)

	return &Client{
		config:     config,
		logger:     logger,
		ruleEngine: ruleEngine,
		closeChan:  make(chan struct{}),
	}
}

// Start 启动客户端
func (c *Client) Start() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.isRunning {
		return nil
	}

	c.logger.Info("启动客户端...")

	// 创建隧道连接器
	tunnelConnector, err := tunnelclient.NewClientConnector(c.config.ServerAddr)
	if err != nil {
		return fmt.Errorf("创建隧道连接器失败: %v", err)
	}

	// 启动隧道连接器
	if err := tunnelConnector.Start(); err != nil {
		return fmt.Errorf("启动隧道连接器失败: %v", err)
	}

	c.tunnelConnector = tunnelConnector

	// 创建SOCKS5监听器
	addr := fmt.Sprintf("127.0.0.1:%d", c.config.LocalPort)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		tunnelConnector.Close()
		return fmt.Errorf("创建监听器失败: %v", err)
	}

	c.listener = listener
	c.isRunning = true

	// 启动SOCKS5服务
	c.wg.Add(1)
	go c.serveSOCKS5()

	c.logger.Infof("客户端已启动，监听地址: %s", addr)

	return nil
}

// Stop 停止客户端
func (c *Client) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.isRunning {
		return nil
	}

	c.logger.Info("停止客户端...")

	// 关闭监听器
	if c.listener != nil {
		c.listener.Close()
	}

	// 关闭隧道连接器
	if c.tunnelConnector != nil {
		c.tunnelConnector.Close()
	}

	close(c.closeChan)
	c.isRunning = false

	// 等待所有协程退出
	c.wg.Wait()

	c.logger.Info("客户端已停止")

	return nil
}

// serveSOCKS5 启动SOCKS5服务
func (c *Client) serveSOCKS5() {
	defer c.wg.Done()

	// 创建SOCKS5服务器
	server := gsocks.NewServer(
		gsocks.WithLogger(NewGoSocks5Logger(c.logger)),
	)

	for {
		conn, err := c.listener.Accept()
		if err != nil {
			select {
			case <-c.closeChan:
				// 正常关闭
				return
			default:
				c.logger.Errorf("接受连接失败: %v", err)
				continue
			}
		}

		// 处理SOCKS5连接
		c.wg.Add(1)
		go func() {
			defer c.wg.Done()
			c.handleConnection(conn, server)
		}()
	}
}

// handleConnection 处理客户端连接
func (c *Client) handleConnection(conn net.Conn, server *gsocks.Server) error {
	defer conn.Close()

	c.logger.Debugf("接受新连接: %s", conn.RemoteAddr())

	// 设置超时
	if c.config.Timeout > 0 {
		conn.SetDeadline(time.Now().Add(c.config.Timeout))
	}

	// 解析目标地址
	addr, readData, err := parseTargetAddress(conn, c.logger)
	if err != nil {
		c.logger.Errorf("解析目标地址失败: %v", err)
		return err
	}

	// 打印readData
	c.logger.Debugf("readData: %x", readData)

	c.logger.Debugf("目标地址: %s", addr)

	// 创建虚拟SOCKS5连接
	vConn := tunnelclient.NewVirtualSocks5Conn(conn, readData, c.logger)

	// 判断是否直连
	if c.ruleEngine.ShouldDirectConnect(addr) {
		c.logger.Debugf("直连地址: %s", addr)
		return server.ServeConn(vConn)
	} else {
		c.logger.Debugf("通过隧道代理地址: %s", addr)
		// 创建隧道流
		streamID, stream, err := c.tunnelConnector.CreateStream(addr)
		if err != nil {
			c.logger.Errorf("创建隧道流失败: %v", err)
			return err
		}

		c.logger.Debugf("创建隧道流成功，ID: %s", streamID)
		// defer stream.Close()
		// 转发数据
		return stream.ServeConn(vConn)
	}
}

// parseTargetAddress 解析SOCKS5请求中的目标地址
func parseTargetAddress(conn net.Conn, logger common.Logger) (string, []byte, error) {
	// 1. 读取握手请求
	handshakeReq := make([]byte, 2)
	if _, err := io.ReadFull(conn, handshakeReq); err != nil {
		return "", nil, fmt.Errorf("读取握手请求头失败: %v", err)
	}

	// 检查版本
	if handshakeReq[0] != 0x05 {
		return "", nil, fmt.Errorf("不支持的协议版本: %d", handshakeReq[0])
	}

	// 读取认证方法列表
	methods := make([]byte, int(handshakeReq[1]))
	if _, err := io.ReadFull(conn, methods); err != nil {
		return "", nil, fmt.Errorf("读取认证方法失败: %v", err)
	}

	// 保存完整的握手请求
	handshakeBuffer := make([]byte, 0, 2+len(methods))
	handshakeBuffer = append(handshakeBuffer, handshakeReq...)
	handshakeBuffer = append(handshakeBuffer, methods...)

	// 2. 发送认证响应
	authResp := []byte{0x05, 0x00}
	if _, err := conn.Write(authResp); err != nil {
		return "", nil, fmt.Errorf("发送认证响应失败: %v", err)
	}

	// 3. 读取连接请求
	reqHeader := make([]byte, 4)
	if _, err := io.ReadFull(conn, reqHeader); err != nil {
		return "", nil, fmt.Errorf("读取请求头失败: %v", err)
	}

	// 检查版本和命令
	if reqHeader[0] != 0x05 {
		return "", nil, fmt.Errorf("不支持的协议版本: %d", reqHeader[0])
	}
	if reqHeader[1] != 0x01 {
		return "", nil, fmt.Errorf("不支持的命令类型: %d", reqHeader[1])
	}
	if reqHeader[2] != 0x00 {
		return "", nil, fmt.Errorf("保留字段必须为0，当前值: %d", reqHeader[2])
	}

	// 根据地址类型读取地址和端口
	var addrPort []byte
	var targetAddr string

	switch reqHeader[3] {
	case 0x01: // IPv4
		addr := make([]byte, 4)
		if _, err := io.ReadFull(conn, addr); err != nil {
			return "", nil, fmt.Errorf("读取IPv4地址失败: %v", err)
		}
		targetAddr = net.IP(addr).String()
		addrPort = addr

	case 0x03: // 域名
		lenBuf := make([]byte, 1)
		if _, err := io.ReadFull(conn, lenBuf); err != nil {
			return "", nil, fmt.Errorf("读取域名长度失败: %v", err)
		}
		length := int(lenBuf[0])

		addr := make([]byte, length)
		if _, err := io.ReadFull(conn, addr); err != nil {
			return "", nil, fmt.Errorf("读取域名失败: %v", err)
		}
		targetAddr = string(addr)
		addrPort = append(lenBuf, addr...)

	case 0x04: // IPv6
		addr := make([]byte, 16)
		if _, err := io.ReadFull(conn, addr); err != nil {
			return "", nil, fmt.Errorf("读取IPv6地址失败: %v", err)
		}
		targetAddr = fmt.Sprintf("[%s]", net.IP(addr).String())
		addrPort = addr

	default:
		return "", nil, fmt.Errorf("不支持的地址类型: %d", reqHeader[3])
	}

	// 读取端口
	portBuf := make([]byte, 2)
	if _, err := io.ReadFull(conn, portBuf); err != nil {
		return "", nil, fmt.Errorf("读取端口失败: %v", err)
	}
	port := int(portBuf[0])<<8 | int(portBuf[1])
	addrPort = append(addrPort, portBuf...)

	// // 4. 发送连接成功响应
	// resp := []byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0}
	// if _, err := conn.Write(resp); err != nil {
	// 	return "", nil, fmt.Errorf("发送连接响应失败: %v", err)
	// }

	// 5. 构建完整的请求数据
	fullReq := make([]byte, 0, len(handshakeBuffer)+len(reqHeader)+len(addrPort))
	fullReq = append(fullReq, handshakeBuffer...)
	// fullReq = append(fullReq, authResp...)
	fullReq = append(fullReq, reqHeader...)
	fullReq = append(fullReq, addrPort...)
	// fullReq = append(fullReq, resp...)

	return fmt.Sprintf("%s:%d", targetAddr, port), fullReq, nil
}
