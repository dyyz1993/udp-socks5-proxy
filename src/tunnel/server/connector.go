package server

import (
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/tealife/proxy-cs3/src/tunnel"
	"github.com/things-go/go-socks5"
)

// ServerConnector 服务端隧道连接器
type ServerConnector struct {
	*tunnel.BaseConnector

	// UDP连接
	conn           *net.UDPConn
	remoteAddr     *net.UDPAddr
	lastActiveTime time.Time

	// 状态
	isRunning   bool
	closeChan   chan struct{}
	processLock sync.Mutex

	// SOCKS5服务器
	socks5Server *socks5.Server
}

// NewServerConnector 创建一个新的服务端连接器
func NewServerConnector(conn *net.UDPConn, remoteAddr *net.UDPAddr) *ServerConnector {
	// 创建SOCKS5服务器
	stdLogger := log.Default()
	socks5Server := socks5.NewServer(
		socks5.WithLogger(socks5.NewLogger(stdLogger)),
	)

	sc := &ServerConnector{
		BaseConnector:  tunnel.NewBaseConnector(),
		conn:           conn,
		remoteAddr:     remoteAddr,
		lastActiveTime: time.Now(),
		closeChan:      make(chan struct{}),
		socks5Server:   socks5Server,
	}
	return sc
}

// Start 启动连接器
func (sc *ServerConnector) Start() error {
	sc.BaseConnector.SetState(tunnel.StateConnected)
	sc.isRunning = true
	return nil
}

// Close 关闭连接
func (sc *ServerConnector) Close() error {
	sc.processLock.Lock()
	defer sc.processLock.Unlock()

	if !sc.isRunning {
		return nil
	}

	sc.isRunning = false
	sc.BaseConnector.SetState(tunnel.StateClosed)
	close(sc.closeChan)

	return nil
}

// SendData 发送数据
func (sc *ServerConnector) SendData(streamID string, data []byte) error {
	// 组装数据包
	packet := tunnel.NewDataPacket(sc.BaseConnector.GetConnectionID(), streamID, data)
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
				if err := sc.SendPacket(fragmentPacketBytes); err != nil {
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
	return sc.SendPacket(packetBytes)
}

// SendPacket 发送原始数据包
func (sc *ServerConnector) SendPacket(packetData []byte) error {
	_, err := sc.conn.WriteToUDP(packetData, sc.remoteAddr)
	if err != nil {
		return err
	}

	// 更新活跃时间
	sc.lastActiveTime = time.Now()

	return nil
}

// // CreateStream 创建一个新的数据流
// func (sc *ServerConnector) CreateStream() (string, tunnel.TunnelStream, error) {
// 	// 生成流ID
// 	streamID := fmt.Sprintf("stream-server-%d", time.Now().UnixNano())

// 	// 创建流
// 	stream := newServerStream(streamID, sc)

// 	// 添加流
// 	sc.BaseConnector.AddStream(streamID, stream)

// 	return streamID, stream, nil
// }

// ProcessIncomingData 处理传入的数据
func (sc *ServerConnector) ProcessIncomingData(data []byte) error {
	sc.processLock.Lock()
	defer sc.processLock.Unlock()

	if !sc.isRunning {
		return tunnel.ErrConnClosed
	}

	// 更新活跃时间
	sc.lastActiveTime = time.Now()

	// 解析数据包
	packet, err := tunnel.ParsePacket(data)
	if err != nil {
		return err
	}

	switch packet.Header.Type {
	case tunnel.PacketTypeHandshake:
		// 处理握手包
		handshakePacket, err := tunnel.ParseHandshakePacket(packet)
		if err != nil {
			return err
		}
		return sc.handleHandshakePacket(handshakePacket)
	case tunnel.PacketTypeData:
		// 数据包处理
		dataPacket, err := tunnel.ParseDataPacket(packet)
		if err != nil {
			return err
		}
		return sc.handleDataPacket(dataPacket)

	case tunnel.PacketTypeFragmented:
		// 处理分片数据包
		return sc.handleFragmentPacket(packet)

	case tunnel.PacketTypeClose:
		// 关闭包处理
		closePacket, err := tunnel.ParseClosePacket(packet)
		if err != nil {
			return err
		}
		return sc.handleClosePacket(closePacket)

	case tunnel.PacketTypeHeartbeat:
		// 心跳包处理
		return sc.handleHeartbeatPacket()

	case tunnel.PacketTypeError:
		// 错误包处理
		return sc.handleErrorPacket(packet)

	default:
		log.Printf("收到未知类型的数据包: %d", packet.Header.Type)
		return tunnel.ErrInvalidPacket
	}
}

// 新增：分片缓存管理
type fragmentCache struct {
	fragments  map[uint32][]*tunnel.FragmentPacket // 按序列ID分组的分片
	expireTime map[uint32]time.Time                // 分片组过期时间
	mu         sync.Mutex                          // 互斥锁
}

// 创建分片缓存
var fragmentManager = &fragmentCache{
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
			log.Printf("[分片] 序列 %d 的分片 %d/%d 尚未收到",
				sequenceID, i+1, len(fragments))
			complete = false
			break
		}
	}

	// 如果完整，合并分片并返回
	if complete {
		log.Printf("[分片] 序列 %d 的所有分片已收到，合并中... (共 %d 个分片)",
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
			log.Printf("[分片] 序列 %d 的分片已过期，清理中", seqID)
			delete(fc.fragments, seqID)
			delete(fc.expireTime, seqID)
		}
	}
}

// 处理分片数据包
func (sc *ServerConnector) handleFragmentPacket(packet *tunnel.TunnelPacket) error {
	// 解析分片数据包
	fragment, err := tunnel.ParseFragmentPacket(packet)
	if err != nil {
		return err
	}

	log.Printf("[分片] 收到分片数据包: 序列=%d, 索引=%d/%d, 原始类型=%d",
		fragment.SequenceID, fragment.FragmentIndex+1, fragment.TotalFragments, fragment.OriginalType)

	// 添加分片到缓存，并尝试合并
	mergedPacket, err := fragmentManager.addFragment(fragment)
	if err != nil {
		log.Printf("[分片] 合并分片失败: %v", err)
		return err
	}

	// 如果分片尚未完整，直接返回
	if mergedPacket == nil {
		return nil
	}

	// 分片已完整合并，根据原始类型处理
	log.Printf("[分片] 分片合并成功，处理原始类型 %d 的数据包", mergedPacket.Header.Type)

	switch mergedPacket.Header.Type {
	case tunnel.PacketTypeData:
		// 处理数据包
		dataPacket := &tunnel.DataPacket{TunnelPacket: *mergedPacket}
		return sc.handleDataPacket(dataPacket)

	case tunnel.PacketTypeClose:
		// 处理关闭包
		closePacket, err := tunnel.ParseClosePacket(mergedPacket)
		if err != nil {
			return err
		}
		return sc.handleClosePacket(closePacket)

	// 其他类型的包不应该被分片
	default:
		log.Printf("[分片] 不支持处理类型 %d 的分片包", mergedPacket.Header.Type)
		return tunnel.ErrInvalidPacket
	}
}

// 处理握手包
func (sc *ServerConnector) handleHandshakePacket(packet *tunnel.HandshakePacket) error {
	log.Printf("收到握手包: 客户端版本=%s, 分组=%s, 特性=%d",
		packet.Version, packet.Group, packet.Features)

	// 生成新的连接ID
	connID := tunnel.GenerateConnectionID()

	// 更新连接器的连接ID
	sc.BaseConnector = tunnel.NewBaseConnector()
	sc.BaseConnector.SetConnectionID(connID)
	sc.BaseConnector.SetState(tunnel.StateConnected)

	// 回复握手确认包
	// 使用相同的密钥，但更新连接ID
	responsePacket := tunnel.NewHandshakePacket(
		connID,
		packet.Key,
		packet.Group,
		packet.Features,
		"server-1.0.0", // 服务器版本
	)

	// 发送确认包
	if err := sc.SendPacket(responsePacket.Bytes()); err != nil {
		log.Printf("发送握手确认包失败: %v", err)
		return err
	}

	log.Printf("已发送握手确认包，分配连接ID: %s", connID)
	return nil
}

// 处理数据包
func (sc *ServerConnector) handleDataPacket(packet *tunnel.DataPacket) error {
	// 从Header获取流ID
	streamID := packet.Header.StreamID

	// 详细记录数据内容
	logDataLen := len(packet.Data)
	var logMsg string
	if logDataLen <= 64 {
		logMsg = fmt.Sprintf("handleDataPacket 收到数据包: streamID=%s, 数据长度=%d, 数据内容: % x",
			streamID, logDataLen, packet.Data)
	} else {
		logMsg = fmt.Sprintf("handleDataPacket 收到数据包: streamID=%s, 数据长度=%d, 前64字节: % x...",
			streamID, logDataLen, packet.Data[:64])
	}
	log.Printf("[DEBUG] %s", logMsg)

	// 获取流
	stream, err := sc.BaseConnector.GetStream(streamID)
	if err != nil {
		// 流不存在，这是第一个数据包，自动创建流
		log.Printf("[DEBUG] 创建新流: streamID=%s", streamID)
		log.Printf("[DEBUG] SOCKS5数据分析: 首字节=%d [% x]", packet.Data[0], packet.Data[0])
		if len(packet.Data) >= 3 {
			log.Printf("[DEBUG] SOCKS5协议分析: VER=%d, CMD/NMETHODS=%d, 第三字节=%d [% x % x % x]",
				packet.Data[0], packet.Data[1], packet.Data[2], packet.Data[0], packet.Data[1], packet.Data[2])
		}

		// 创建新的流
		serverStream := newServerStream(streamID, sc).(*serverStream)
		sc.BaseConnector.AddStream(streamID, serverStream)

		// 直接使用serverStream作为连接传递给SOCKS5服务器
		go func() {
			if err := sc.socks5Server.ServeConn(serverStream); err != nil {
				log.Printf("SOCKS5处理连接错误: %v", err)
				log.Printf("[DEBUG] SOCKS5处理连接错误详细数据: 收到的数据长度=%d, 内容: % x",
					len(packet.Data), packet.Data)
			}
		}()

		log.Printf("收到新流的第一个数据包，自动创建流: %s", streamID)
		stream = serverStream
	}

	// 投递数据
	return stream.PutData(packet.Data)
}

// 处理关闭包
func (sc *ServerConnector) handleClosePacket(packet *tunnel.ClosePacket) error {
	// 从Header获取流ID
	streamID := packet.Header.StreamID

	// 获取流
	stream, err := sc.BaseConnector.GetStream(streamID)
	if err != nil {
		return nil // 流可能已经关闭，忽略错误
	}

	// 关闭流
	return stream.Close()
}

// 处理心跳包
func (sc *ServerConnector) handleHeartbeatPacket() error {
	// 回复心跳包
	packet := tunnel.NewHeartbeatPacket(sc.BaseConnector.GetConnectionID(), 0, 0)
	return sc.SendPacket(packet.Bytes())
}

// handleErrorPacket 处理错误包
func (sc *ServerConnector) handleErrorPacket(packet *tunnel.TunnelPacket) error {
	// 简单记录错误包信息
	log.Printf("收到错误包: 连接ID=%s", packet.Header.ConnectionID)
	return nil
}

// SetRemoteAddr 设置远程地址（仅用于测试）
func (sc *ServerConnector) SetRemoteAddr(addr *net.UDPAddr) {
	sc.remoteAddr = addr
}
