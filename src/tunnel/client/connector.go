package client

import (
	"fmt"
	"log"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/tealife/proxy-cs3/src/tunnel"
)

// 确保我们可以使用包中的函数
func init() {
	// 这里不做任何事情，只是确保导入的包被使用
}

// ClientConnector 客户端隧道连接器
type ClientConnector struct {
	*tunnel.BaseConnector

	// 服务器地址
	serverAddr string

	// UDP连接
	conn           *net.UDPConn
	lastActiveTime time.Time

	// 控制和状态
	isRunning       bool
	reconnectTicker *time.Ticker
	closeChan       chan struct{}
	processMutex    sync.Mutex
}

// NewClientConnector 创建一个新的客户端连接器
func NewClientConnector(serverAddr string) (*ClientConnector, error) {
	c := &ClientConnector{
		BaseConnector: tunnel.NewBaseConnector(),
		serverAddr:    serverAddr,
		isRunning:     false,
		closeChan:     make(chan struct{}),
	}
	return c, nil
}

// Connect 连接到远程服务器
func (c *ClientConnector) Connect() error {
	// 解析服务器地址
	serverAddr, err := net.ResolveUDPAddr("udp", c.serverAddr)
	if err != nil {
		return err
	}

	// 创建UDP连接
	conn, err := net.DialUDP("udp", nil, serverAddr)
	if err != nil {
		return err
	}

	c.conn = conn
	c.lastActiveTime = time.Now()

	log.Printf("已连接到服务器: %s", c.serverAddr)

	// 发送握手包
	if err := c.sendHandshake(); err != nil {
		return fmt.Errorf("发送握手包失败: %v", err)
	}

	// 等待握手确认
	if err := c.waitForHandshakeResponse(); err != nil {
		return fmt.Errorf("等待握手确认失败: %v", err)
	}

	return nil
}

// sendHandshake 发送握手包
func (c *ClientConnector) sendHandshake() error {
	log.Printf("发送握手包...")

	// 创建一个随机的握手密钥
	key := [32]byte{}
	rand.Read(key[:])

	// 使用临时连接ID
	tempConnID := fmt.Sprintf("temp-%d", time.Now().UnixNano())

	// 创建握手包
	packet := tunnel.NewHandshakePacket(
		tempConnID,
		key,
		"default",      // 分组
		0,              // 特性标志
		"client-1.0.0", // 客户端版本
	)

	// 发送握手包
	_, err := c.conn.Write(packet.Bytes())
	if err != nil {
		return err
	}

	log.Printf("已发送握手包, 临时连接ID: %s", tempConnID)
	return nil
}

// waitForHandshakeResponse 等待握手确认
func (c *ClientConnector) waitForHandshakeResponse() error {
	buffer := make([]byte, 4096)

	// 设置握手超时
	c.conn.SetReadDeadline(time.Now().Add(5 * time.Second))

	// 读取响应
	n, _, err := c.conn.ReadFromUDP(buffer)
	if err != nil {
		return err
	}

	// 重置读取超时
	c.conn.SetReadDeadline(time.Time{})

	// 解析数据包
	packet, err := tunnel.ParsePacket(buffer[:n])
	if err != nil {
		return err
	}

	// 验证是否为握手包
	if packet.Header.Type != tunnel.PacketTypeHandshake {
		return fmt.Errorf("收到非握手包响应: %d", packet.Header.Type)
	}

	// 解析握手包
	handshakePacket, err := tunnel.ParseHandshakePacket(packet)
	if err != nil {
		return err
	}

	// 更新连接ID
	c.BaseConnector.SetConnectionID(handshakePacket.Header.ConnectionID)

	log.Printf("握手成功, 获得连接ID: %s", c.BaseConnector.GetConnectionID())
	return nil
}

// Start 启动连接器
func (c *ClientConnector) Start() error {
	c.processMutex.Lock()
	defer c.processMutex.Unlock()

	if c.isRunning {
		return nil
	}

	// 连接服务器
	if err := c.Connect(); err != nil {
		return err
	}

	c.isRunning = true
	c.BaseConnector.SetState(tunnel.StateConnected)

	// 启动接收协程
	go c.receiveLoop()

	// 启动心跳协程
	go c.heartbeatLoop()

	return nil
}

// receiveLoop 接收循环
func (c *ClientConnector) receiveLoop() {
	buffer := make([]byte, 4096)
	for {
		select {
		case <-c.closeChan:
			log.Printf("接收循环退出")
			return
		default:
			// 设置读取超时
			c.conn.SetReadDeadline(time.Now().Add(5 * time.Second))

			// 读取数据
			n, _, err := c.conn.ReadFromUDP(buffer)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					// 读取超时，继续下一次循环
					continue
				}
				log.Printf("读取数据错误: %v", err)
				continue
			}

			// 处理数据
			if err := c.ProcessIncomingData(buffer[:n]); err != nil {
				log.Printf("处理数据包错误: %v", err)
			}

			// 更新活跃时间
			c.lastActiveTime = time.Now()
		}
	}
}

// heartbeatLoop 心跳循环
func (c *ClientConnector) heartbeatLoop() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	sequence := uint32(0)

	for {
		select {
		case <-c.closeChan:
			log.Printf("心跳循环退出")
			return
		case <-ticker.C:
			// 发送心跳包
			log.Printf("发送心跳包")
			packet := tunnel.NewHeartbeatPacket(c.BaseConnector.GetConnectionID(), sequence, 0)
			if _, err := c.conn.Write(packet.Bytes()); err != nil {
				log.Printf("发送心跳包错误: %v", err)
			}
			sequence++
		}
	}
}

// Close 关闭连接
func (c *ClientConnector) Close() error {
	c.processMutex.Lock()
	defer c.processMutex.Unlock()

	if !c.isRunning {
		return nil
	}

	c.isRunning = false
	c.BaseConnector.SetState(tunnel.StateClosed)
	close(c.closeChan)

	if c.conn != nil {
		return c.conn.Close()
	}

	return nil
}

// SendData 发送数据
func (c *ClientConnector) SendData(streamID string, data []byte) error {
	// 确保连接已打开
	if !c.isRunning || c.conn == nil {
		return tunnel.ErrConnClosed
	}

	// 如果连接ID为空，说明握手未完成
	if c.BaseConnector.GetConnectionID() == "" {
		return fmt.Errorf("握手未完成，不能发送数据")
	}

	// 组装数据包
	packet := tunnel.NewDataPacket(c.BaseConnector.GetConnectionID(), streamID, data)
	packetBytes := packet.Bytes()

	// 检查是否需要分片
	if len(packetBytes) > tunnel.MaxUDPPacketSize {
		// 大数据包需要分片发送
		fragments := tunnel.SplitPacket(&packet.TunnelPacket)
		if fragments != nil {
			log.Printf("[数据分片] 数据包被分片为 %d 个片段: streamID=%s, 数据总长度=%d字节",
				len(fragments), streamID, len(data))

			// 逐个发送分片
			for i, fragment := range fragments {
				fragmentPacketBytes := fragment.TunnelPacket.Bytes()
				log.Printf("[数据分片] 发送分片 %d/%d: streamID=%s, 分片大小=%d字节",
					i+1, len(fragments), streamID, len(fragmentPacketBytes))

				// 发送分片数据包
				if _, err := c.conn.Write(fragmentPacketBytes); err != nil {
					log.Printf("[数据分片] 发送分片 %d/%d 失败: %v", i+1, len(fragments), err)
					return err
				}

				// 短暂延迟，避免网络拥塞
				if i < len(fragments)-1 {
					time.Sleep(2 * time.Millisecond)
				}
			}

			return nil
		}
	}

	// 对于小数据包，直接发送
	_, err := c.conn.Write(packetBytes)
	return err
}

// CreateStream 创建一个新的数据流
func (c *ClientConnector) CreateStream(targetAddr string) (string, tunnel.TunnelStream, error) {
	// 确保连接已打开
	if !c.isRunning || c.conn == nil {
		return "", nil, tunnel.ErrConnClosed
	}

	// 确保握手已完成
	if c.BaseConnector.GetConnectionID() == "" {
		return "", nil, fmt.Errorf("握手未完成，不能创建数据流")
	}

	// 生成流ID
	streamID := fmt.Sprintf("stream-%d", time.Now().UnixNano())

	// 创建流，目标地址信息存储在Stream对象中
	stream := newClientStream(streamID, c)

	// 添加流
	c.BaseConnector.AddStream(streamID, stream)

	// 直接返回流，不发送任何特殊请求
	// 服务器端会在收到普通数据包时按需创建流
	return streamID, stream, nil
}

// ProcessIncomingData 处理传入的数据
func (c *ClientConnector) ProcessIncomingData(data []byte) error {
	c.processMutex.Lock()
	defer c.processMutex.Unlock()

	if c.isRunning == false {
		return tunnel.ErrConnClosed
	}

	// 更新活跃时间
	c.lastActiveTime = time.Now()

	// 解析数据包
	packet, err := tunnel.ParsePacket(data)
	if err != nil {
		return err
	}

	switch packet.Header.Type {
	case tunnel.PacketTypeHandshake:
		handshakePacket, err := tunnel.ParseHandshakePacket(packet)
		if err != nil {
			return err
		}
		return c.handleHandshakePacket(handshakePacket)

	case tunnel.PacketTypeData:
		dataPacket, err := tunnel.ParseDataPacket(packet)
		if err != nil {
			return err
		}
		return c.handleDataPacket(dataPacket)

	case tunnel.PacketTypeFragmented:
		// 处理分片数据包
		return c.handleFragmentPacket(packet)

	case tunnel.PacketTypeClose:
		closePacket, err := tunnel.ParseClosePacket(packet)
		if err != nil {
			return err
		}
		return c.handleClosePacket(closePacket)

	case tunnel.PacketTypeHeartbeat:
		return c.handleHeartbeatPacket()

	case tunnel.PacketTypeError:
		errorPacket, err := tunnel.ParseErrorPacket(packet)
		if err != nil {
			return err
		}
		return c.handleErrorPacket(errorPacket)

	default:
		log.Printf("收到未知类型的数据包: %d", packet.Header.Type)
		return tunnel.ErrInvalidPacket
	}
}

// handleDataPacket 处理数据包
func (c *ClientConnector) handleDataPacket(packet *tunnel.DataPacket) error {
	// 从Header获取流ID
	streamID := packet.Header.StreamID

	// 获取流
	stream, err := c.BaseConnector.GetStream(streamID)
	if err != nil {
		log.Printf("找不到流: %s", streamID)
		return err
	}

	// 投递数据
	return stream.PutData(packet.Data)
}

// handleClosePacket 处理关闭包
func (c *ClientConnector) handleClosePacket(packet *tunnel.ClosePacket) error {
	// 从Header获取流ID
	streamID := packet.Header.StreamID

	// 获取流
	stream, err := c.BaseConnector.GetStream(streamID)
	if err != nil {
		return nil // 流可能已经关闭，忽略错误
	}

	// 关闭流
	return stream.Close()
}

// handleHandshakePacket 处理握手包
func (c *ClientConnector) handleHandshakePacket(packet *tunnel.HandshakePacket) error {
	log.Printf("收到握手确认包: 连接ID=%s", packet.Header.ConnectionID)

	// 更新连接ID
	c.BaseConnector.SetConnectionID(packet.Header.ConnectionID)

	return nil
}

// handleHeartbeatPacket 处理心跳包
func (c *ClientConnector) handleHeartbeatPacket() error {
	// 服务器心跳包已收到，不需要特别处理
	// 只需更新最后活跃时间，这已在ProcessIncomingData中完成
	return nil
}

// handleErrorPacket 处理错误包
func (c *ClientConnector) handleErrorPacket(packet *tunnel.ErrorPacket) error {
	// 记录错误信息
	log.Printf("收到错误包: 错误码=%d, 错误信息=%s, 相关ID=%s",
		packet.Code, packet.Message, packet.RelatedID)

	// 如果相关ID是流ID，关闭对应的流
	if stream, err := c.BaseConnector.GetStream(packet.RelatedID); err == nil {
		stream.Close()
	}

	return nil
}

// SetConn 设置UDP连接（仅用于测试）
func (c *ClientConnector) SetConn(conn *net.UDPConn) {
	c.conn = conn
	c.lastActiveTime = time.Now()
}

// 新增：分片缓存管理
type fragmentCache struct {
	fragments  map[uint32][]*tunnel.FragmentPacket // 按序列ID分组的分片
	expireTime map[uint32]time.Time                // 分片组过期时间
	mu         sync.Mutex                          // 互斥锁
}

// 创建分片缓存
var clientFragmentManager = &fragmentCache{
	fragments:  make(map[uint32][]*tunnel.FragmentPacket),
	expireTime: make(map[uint32]time.Time),
}

// 添加分片并尝试合并
func (fc *fragmentCache) addFragment(fragment *tunnel.FragmentPacket) (*tunnel.TunnelPacket, error) {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	sequenceID := fragment.SequenceID

	// 更新过期时间
	fc.expireTime[sequenceID] = time.Now().Add(30 * time.Second)

	// 如果没有这个序列的分片列表，创建一个
	if _, ok := fc.fragments[sequenceID]; !ok {
		fc.fragments[sequenceID] = make([]*tunnel.FragmentPacket, fragment.TotalFragments)
	}

	// 保存分片到正确的位置
	fc.fragments[sequenceID][fragment.FragmentIndex] = fragment

	// 清理过期的分片
	fc.cleanExpired()

	// 检查是否所有分片都已收到
	fragments := fc.fragments[sequenceID]
	complete := true
	for i, f := range fragments {
		if f == nil {
			log.Printf("[客户端分片] 序列 %d 的分片 %d/%d 尚未收到",
				sequenceID, i+1, len(fragments))
			complete = false
			break
		}
	}

	// 如果完整，合并分片并返回
	if complete {
		log.Printf("[客户端分片] 序列 %d 的所有分片已收到，合并中... (共 %d 个分片)",
			sequenceID, len(fragments))
		packet, err := tunnel.MergeFragments(fragments)

		// 无论成功还是失败，都清理这个序列的分片
		delete(fc.fragments, sequenceID)
		delete(fc.expireTime, sequenceID)

		return packet, err
	}

	return nil, nil // 分片尚未完整
}

// 清理过期的分片
func (fc *fragmentCache) cleanExpired() {
	now := time.Now()
	for seqID, expireTime := range fc.expireTime {
		if now.After(expireTime) {
			log.Printf("[客户端分片] 序列 %d 的分片已过期，清理中", seqID)
			delete(fc.fragments, seqID)
			delete(fc.expireTime, seqID)
		}
	}
}

// 处理分片数据包
func (c *ClientConnector) handleFragmentPacket(packet *tunnel.TunnelPacket) error {
	// 解析分片数据包
	fragment, err := tunnel.ParseFragmentPacket(packet)
	if err != nil {
		return err
	}

	log.Printf("[客户端分片] 收到分片数据包: 序列=%d, 索引=%d/%d, 原始类型=%d",
		fragment.SequenceID, fragment.FragmentIndex+1, fragment.TotalFragments, fragment.OriginalType)

	// 添加分片到缓存，并尝试合并
	mergedPacket, err := clientFragmentManager.addFragment(fragment)
	if err != nil {
		log.Printf("[客户端分片] 合并分片失败: %v", err)
		return err
	}

	// 如果分片尚未完整，直接返回
	if mergedPacket == nil {
		return nil
	}

	// 分片已完整合并，根据原始类型处理
	log.Printf("[客户端分片] 分片合并成功，处理原始类型 %d 的数据包", mergedPacket.Header.Type)

	switch mergedPacket.Header.Type {
	case tunnel.PacketTypeData:
		// 处理数据包
		dataPacket := &tunnel.DataPacket{TunnelPacket: *mergedPacket}
		return c.handleDataPacket(dataPacket)

	case tunnel.PacketTypeClose:
		// 处理关闭包
		closePacket, err := tunnel.ParseClosePacket(mergedPacket)
		if err != nil {
			return err
		}
		return c.handleClosePacket(closePacket)

	// 其他类型的包不应该被分片
	default:
		log.Printf("[客户端分片] 不支持处理类型 %d 的分片包", mergedPacket.Header.Type)
		return tunnel.ErrInvalidPacket
	}
}
