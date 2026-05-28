# 客户端通讯协议规范 (PROXY-CS3-UDP-TUNNEL-SOCKS5)

本文档定义了客户端与服务端之间通过UDP隧道进行通讯时，客户端发送数据包的格式和处理服务端响应的协议行为。

## 1. 协议概述

客户端通过UDP与服务端通信。为保证可靠性、实现多路复用和处理大数据，客户端使用自定义的隧道协议。

- **连接建立**: 客户端首先向服务端发送 `HandshakePacket`，成功后获取 `ConnectionID`。
- **流创建与管理**: 客户端通过向 `Connector` 请求创建流 (`CreateStream`)，并通过 `StreamID` 区分不同的逻辑数据流。
- **数据传输**: SOCKS5请求和应用数据通过 `DataPacket` 发送，包含 `StreamID`。
- **心跳维持**: 客户端定期发送 `HeartbeatPacket` 以保持连接活跃并检测中断。
- **流关闭**: 当本地应用关闭连接时，客户端发送 `ClosePacket` 通知服务端。
- **分片机制**: 当要发送的数据包（通常是SOCKS5数据）超过 `MaxUDPPacketSize` 时，客户端将其拆分为 `FragmentPacket` 发送。
- **响应处理**: 客户端接收并处理来自服务端的各种数据包，包括数据、心跳响应、关闭确认、错误信息以及分片数据。

## 2. 核心常量

(与服务端相同，详见 `docs/server_protocol.md` 第2节)

```golang
// src/tunnel/packet.go
const (
  ProtocolVersion = 1
  PacketTypeHandshake  = 1
  PacketTypeData       = 2
  PacketTypeHeartbeat  = 3
  PacketTypeClose      = 4
  PacketTypeError      = 5
  PacketTypeFragmented = 6
  MaxUDPPacketSize    = 8192
  FragmentHeaderSize  = 20
  MaxFragmentDataSize = MaxUDPPacketSize - FragmentHeaderSize
)
```

## 3. 通用数据包结构

(与服务端相同，详见 `docs/server_protocol.md` 第3节)

**基础头部 (Header)**

| 字段           | 类型   | 大小 (字节) | 描述                                               |
| -------------- | ------ | ----------- | -------------------------------------------------- |
| Version        | uint8  | 1           | 协议版本 (当前为 `1`)                              |
| Type           | uint8  | 1           | 数据包类型                                         |
| Flags          | uint8  | 1           | 标志位 (主要用于分片)                              |
| ConnectionID长度 | uint16 | 2           | `ConnectionID` 字段的字节长度 (大端序)              |
| ConnectionID   | string | 可变        | 连接UUID                                          |
| StreamID长度   | uint16 | 2           | `StreamID` 字段字节长度 (大端序, **仅Data, Close, Fragmented包**) |
| StreamID       | string | 可变        | 流UUID (**仅Data, Close, Fragmented包**)             |

**数据包体 (Payload)**

| 字段 | 类型   | 大小 (字节) | 描述                               |
| ---- | ------ | ----------- | ---------------------------------- |
| Data | []byte | 可变        | 具体载荷，取决于 `Type`             |

**注意:** 序列化使用 **大端序 (BigEndian)**。

## 4. 客户端发送的数据包详解

### 4.1 握手包 (PacketTypeHandshake = 1)

客户端建立UDP连接后发送的第一个包，用于认证和获取 `ConnectionID`。

- **头部 (Header)**:
  - `Type`: 1
  - `ConnectionID`: 发送时通常包含一个临时ID（如 `temp-时间戳`）。
  - `StreamID`: **不存在**
- **数据体 (Data)**: (结构同服务端文档4.1节)
  | 字段         | 类型   | 大小 (字节) | 描述             |
  | ------------ | ------ | ----------- | ---------------- |
  | Key          | [32]byte | 32          | 握手密钥         |
  | Group长度    | uint16 | 2           |                  |
  | Group        | string | 可变        | 客户端分组名     |
  | Features     | uint32 | 4           | 支持特性标志位   |
  | Version长度  | uint16 | 2           |                  |
  | Version      | string | 可变        | 客户端版本号     |

**客户端行为**: 发送后，等待服务端的 `HandshakePacket` 响应，从中获取正式的 `ConnectionID` 并保存。

### 4.2 数据包 (PacketTypeData = 2)

用于发送SOCKS5请求和后续的应用数据。

- **头部 (Header)**:
  - `Type`: 2
  - `ConnectionID`: 使用握手后获取的正式ID。
  - `StreamID`: **存在**，标识数据所属的流。
- **数据体 (Data)**: (结构同服务端文档4.2节)
  | 字段     | 类型   | 大小 (字节) | 描述                                         |
  | -------- | ------ | ----------- | -------------------------------------------- |
  | AppData  | []byte | 可变        | SOCKS5请求数据或应用数据                      |

**客户端行为**: 通过 `ClientConnector.SendData(streamID, data)` 发送。如果 `data` 过大，`SendData` 内部会自动调用分片逻辑。

### 4.3 心跳包 (PacketTypeHeartbeat = 3)

客户端定期发送以维持连接活跃。

- **头部 (Header)**:
  - `Type`: 3
  - `ConnectionID`: 使用正式ID。
  - `StreamID`: **不存在**
- **数据体 (Data)**: (结构同服务端文档4.3节)
  | 字段      | 类型    | 大小 (字节) | 描述             |
  | --------- | ------- | ----------- | ---------------- |
  | Timestamp | int64   | 8           | 发送时间戳       |
  | Sequence  | uint32  | 4           | 序列号           |
  | Load      | float32 | 4           | 客户端负载 (可选) |

**客户端行为**: 由 `ClientConnector.heartbeatLoop` 定时发送。

### 4.4 关闭流包 (PacketTypeClose = 4)

当本地应用关闭连接，对应的 `ClientStream` 关闭时发送。

- **头部 (Header)**:
  - `Type`: 4
  - `ConnectionID`: 使用正式ID。
  - `StreamID`: **存在**，标识要关闭的流。
- **数据体 (Data)**:
  - **为空**

**客户端行为**: 调用流的 `Close()` 方法时，通常会触发发送此包。

### 4.5 错误包 (PacketTypeError = 5)

客户端一般不主动发送错误包，主要是接收和处理来自服务端的错误包。

### 4.6 分片数据包 (PacketTypeFragmented = 6)

当 `ClientConnector.SendData` 检测到原始数据包（通常是 `PacketTypeData`）超过 `MaxUDPPacketSize` 时，客户端会自动将其拆分并发送多个分片包。

- **头部 (Header)**:
  - `Type`: 6
  - `Flags`: 包含 `FragmentFlagStart` 和/或 `FragmentFlagEnd`。
  - `ConnectionID`: 使用正式ID。
  - `StreamID`: **存在**，标识分片所属的流。
- **数据体 (Data)**: (结构同服务端文档4.6节)
  | 字段           | 类型   | 大小 (字节) | 描述                   |
  | -------------- | ------ | ----------- | ---------------------- |
  | SequenceID     | uint32 | 4           | 分片组序列号           |
  | TotalFragments | uint32 | 4           | 分片总数               |
  | FragmentIndex  | uint32 | 4           | 当前分片索引 (从0开始) |
  | OriginalType   | uint8  | 1           | 原始包类型 (通常为2)   |
  | FragmentData   | []byte | 可变        | 分片数据块             |

**客户端行为**: 由 `tunnel.SplitPacket` 函数实现拆分逻辑，并在 `ClientConnector.SendData` 中调用，然后逐个发送生成的 `FragmentPacket`。

## 5. 客户端分片发送逻辑

1.  **检查大小**: 在 `ClientConnector.SendData` 中，获取 `DataPacket` 的完整字节大小。
2.  **判断是否分片**: 如果大小超过 `MaxUDPPacketSize`，则需要分片。
3.  **调用拆分**: 调用 `tunnel.SplitPacket(originalPacket)`。
    - 该函数会生成一个 `[]*FragmentPacket` 切片。
    - 每个 `FragmentPacket` 包含部分原始数据，并设置好 `SequenceID` (同一组分片相同), `TotalFragments`, `FragmentIndex` (从0递增), `OriginalType`, 以及 `Flags` (第一个包有 `Start` 标志，最后一个包有 `End` 标志)。
4.  **逐个发送**: 遍历 `FragmentPacket` 切片，将每个分片包序列化为字节，并通过UDP连接发送出去。
5.  **发送间隔**: 可以在发送每个分片后加入短暂的延时（如几毫秒），以避免网络拥塞或接收端处理不过来。

## 6. 客户端响应处理逻辑

客户端通过 `ClientConnector.receiveLoop` 持续接收UDP数据，并通过 `ProcessIncomingData` 处理。

- **`ClientConnector.ProcessIncomingData`**:
  - 解析收到的UDP数据，识别基础包头。
  - 根据 `Header.Type` 分发给相应的处理函数。
- **`ClientConnector.handleHandshakePacket`**:
  - 在 `waitForHandshakeResponse` 中特殊处理：验证响应，获取并存储正式的 `ConnectionID`。
- **`ClientConnector.handleDataPacket`**:
  - 根据 `Header.StreamID` 找到对应的 `ClientStream` 实例。
  - 将 `DataPacket.Data` 放入流的接收缓冲区 (`PutData`)，供 `ClientStream.ServeConn` 中的逻辑读取并写入原始的客户端应用连接。
- **`ClientConnector.handleFragmentPacket`**:
  - 与服务端类似，调用分片管理器 (`fragmentManager`) 的 `addFragment` 方法缓存分片。
  - 如果分片完整，管理器返回重组后的原始包。
  - 将重组后的包交给对应类型的处理函数（如 `handleDataPacket`）。
- **`ClientConnector.handleClosePacket`**:
  - 根据 `Header.StreamID` 找到 `ClientStream`。
  - 调用流的 `Close()` 方法，关闭流并通知本地应用连接已断开。
  - 从连接器中移除该流。
- **`ClientConnector.handleHeartbeatPacket`**:
  - 通常是服务端对客户端心跳的响应，记录日志或更新延迟统计。
- **`ClientConnector.handleErrorPacket`**:
  - 解析错误信息 (`Code`, `Message`, `RelatedID`)。
  - 记录日志。
  - 根据错误类型可能关闭相关流或整个连接。

## 7. 注意事项

- 客户端需要管理好 `ConnectionID` 和 `StreamID` 的生成与使用。
- 客户端也需要实现分片重组逻辑来处理服务端发送的大数据包。
- 心跳机制对于维持UDP"连接"状态和检测超时至关重要。
- 错误处理应能优雅地关闭流或连接，并可能需要通知用户。

## 8. 最小客户端实现示例

下面是一个使用Go语言实现的、遵循此协议的最小客户端骨架示例。它展示了如何连接服务器、发送握手包、发送数据包（包括简单分片演示）和心跳，并接收处理基本响应。

**注意:** 此示例仅为演示协议处理流程，省略了完整的分片重组、流管理、SOCKS5集成、错误处理、超时和并发安全等生产级实现细节。

```go
package main

import (
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/tealife/proxy-cs3/src/tunnel" // 假设你的项目路径配置正确
)

const (
	serverAddr = "127.0.0.1:8080" // 服务端地址和端口
)

var connectionID string // 保存从服务端获取的连接ID
var udpConn *net.UDPConn

func main() {
	// 1. 连接服务器
	sAddr, err := net.ResolveUDPAddr("udp", serverAddr)
	if err != nil {
		log.Fatalf("无法解析服务端地址: %v", err)
	}
	conn, err := net.DialUDP("udp", nil, sAddr)
	if err != nil {
		log.Fatalf("无法连接到服务端 %s: %v", serverAddr, err)
	}
	udpConn = conn
	defer udpConn.Close()
	log.Printf("已连接到服务端 %s", serverAddr)

	// 2. 发送并等待握手
	err = performHandshake()
	if err != nil {
		log.Fatalf("握手失败: %v", err)
	}
	log.Printf("握手成功，ConnectionID: %s", connectionID)

	// 3. 启动接收循环 (goroutine)
	go receiveLoop()

	// 4. 启动心跳循环 (goroutine)
	go heartbeatLoop()

	// 5. 模拟发送一些数据
	simulateSendData()

	// 保持主线程运行 (实际应用中会有其他逻辑)
	select {}
}

// performHandshake 发送握手包并等待响应
func performHandshake() error {
	key := [32]byte{}
	rand.Read(key[:])
	tempConnID := fmt.Sprintf("temp-%d", time.Now().UnixNano())
	handshakeReq := tunnel.NewHandshakePacket(tempConnID, key, "default", 0, "client-min-1.0")

	log.Printf("发送握手请求...")
	_, err := udpConn.Write(handshakeReq.Bytes())
	if err != nil {
		return fmt.Errorf("发送握手包错误: %v", err)
	}

	// 等待响应
	buffer := make([]byte, 4096)
	udpConn.SetReadDeadline(time.Now().Add(5 * time.Second)) // 设置超时
	n, _, err := udpConn.ReadFromUDP(buffer)
	if err != nil {
		return fmt.Errorf("读取握手响应错误: %v", err)
	}
	udpConn.SetReadDeadline(time.Time{}) // 清除超时

	// 解析响应
	packet, err := tunnel.ParsePacket(buffer[:n])
	if err != nil {
		return fmt.Errorf("解析握手响应错误: %v", err)
	}

	// 检查响应类型 (简化：假设直接是Error包作为确认)
	if packet.Header.Type != tunnel.PacketTypeError { // 假设服务端用Error Code 0表示成功
	// if packet.Header.Type != tunnel.PacketTypeHandshake { // 如果服务端回复Handshake包
		return fmt.Errorf("收到的握手响应类型不正确: %d", packet.Header.Type)
	}

	// 从响应中获取真实的ConnectionID (在我们的最小服务端示例中，它包含在Error包的RelatedID里)
	errorPacket, _ := tunnel.ParseErrorPacket(packet)
	if errorPacket != nil && errorPacket.Code == 0 {
		connectionID = errorPacket.RelatedID // 或者 packet.Header.ConnectionID 如果服务端回的是Handshake
		return nil
	}

	return fmt.Errorf("握手响应无效或包含错误: Type=%d", packet.Header.Type)
}

// receiveLoop 持续接收来自服务端的数据包
func receiveLoop() {
	buffer := make([]byte, tunnel.MaxUDPPacketSize*2)
	for {
		n, _, err := udpConn.ReadFromUDP(buffer)
		if err != nil {
			// 检查是否是连接关闭错误
			if strings.Contains(err.Error(), "use of closed network connection") {
				log.Println("连接已关闭，退出接收循环")
				return
			}
			log.Printf("读取UDP数据错误: %v", err)
			// 可以考虑加入重连逻辑
			time.Sleep(1 * time.Second)
			continue
		}

		packet, err := tunnel.ParsePacket(buffer[:n])
		if err != nil {
			log.Printf("解析服务端数据包错误: %v", err)
			continue
		}

		log.Printf("收到服务端数据包: Type=%d, ConnID=%s, StreamID=%s, Size=%d",
			packet.Header.Type, packet.Header.ConnectionID, packet.Header.StreamID, n)

		switch packet.Header.Type {
		case tunnel.PacketTypeData:
			log.Printf("  Data Payload: %s", string(packet.Data)) // 仅作示例
			// 实际应用：交给对应的Stream处理
		case tunnel.PacketTypeHeartbeat:
			log.Printf("  收到心跳响应")
		case tunnel.PacketTypeClose:
			log.Printf("  收到流关闭通知: StreamID=%s", packet.Header.StreamID)
			// 实际应用：关闭本地对应的流
		case tunnel.PacketTypeError:
			errorPacket, _ := tunnel.ParseErrorPacket(packet)
			log.Printf("  收到错误: Code=%d, Msg='%s', RelatedID=%s", errorPacket.Code, errorPacket.Message, errorPacket.RelatedID)
		case tunnel.PacketTypeFragmented:
			log.Printf("  收到分片数据包，需要重组 (本示例未实现)")
			// 实际应用：交给分片管理器处理
		default:
			log.Printf("  收到未处理的服务端包类型: %d", packet.Header.Type)
		}
	}
}

// heartbeatLoop 定期发送心跳包
func heartbeatLoop() {
	ticker := time.NewTicker(5 * time.Second) // 每5秒发一次心跳
	defer ticker.Stop()
	sequence := uint32(0)

	for {
		select {
		case <-ticker.C:
			if connectionID == "" || udpConn == nil {
				continue // 尚未完成握手或连接已关闭
			}
			heartbeatPacket := tunnel.NewHeartbeatPacket(connectionID, sequence, 0.0)
			log.Printf("发送心跳包 #%d...", sequence)
			_, err := udpConn.Write(heartbeatPacket.Bytes())
			if err != nil {
				log.Printf("发送心跳包错误: %v", err)
				// 可以考虑触发重连等
			}
			sequence++
		// 可以添加一个关闭通道来优雅退出
		}
	}
}

// simulateSendData 模拟发送数据，包括一个可能需要分片的大数据包
func simulateSendData() {
	if connectionID == "" {
		log.Println("尚未握手，无法发送数据")
		return
	}

	streamID := "stream-" + uuid.NewString()[:8] // 创建一个模拟流ID

	// 1. 发送一个小数据包
	smallData := []byte("Hello from client on stream " + streamID)
	sendDataPacket(streamID, smallData)

	// 2. 发送一个需要分片的大数据包
	largeData := make([]byte, tunnel.MaxUDPPacketSize+100) // 超过MTU
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}
	log.Printf("准备发送大数据包 (大小: %d 字节)...", len(largeData))
	sendDataPacket(streamID, largeData)

	// 3. 模拟关闭流
	time.Sleep(1 * time.Second)
	sendClosePacket(streamID)
}

// sendDataPacket 负责发送数据包，处理分片
func sendDataPacket(streamID string, data []byte) {
	dataPacket := tunnel.NewDataPacket(connectionID, streamID, data)
	packetBytes := dataPacket.Bytes()

	if len(packetBytes) > tunnel.MaxUDPPacketSize {
		log.Printf("  数据包大小 (%d) 超过 MTU (%d)，进行分片...", len(packetBytes), tunnel.MaxUDPPacketSize)
		fragments := tunnel.SplitPacket(&dataPacket.TunnelPacket)
		log.Printf("  拆分为 %d 个分片", len(fragments))
		for i, fragment := range fragments {
			log.Printf("  发送分片 %d/%d (大小: %d)...", i+1, len(fragments), len(fragment.Bytes()))
			_, err := udpConn.Write(fragment.Bytes())
			if err != nil {
				log.Printf("发送分片 %d 错误: %v", i+1, err)
				return // 简化处理：一个分片失败则停止
			}
			time.Sleep(2 * time.Millisecond) // 短暂间隔
		}
		log.Printf("  大数据包分片发送完毕")
	} else {
		log.Printf("发送小数据包 (大小: %d)...", len(packetBytes))
		_, err := udpConn.Write(packetBytes)
		if err != nil {
			log.Printf("发送小数据包错误: %v", err)
		}
	}
}

// sendClosePacket 发送关闭流包
func sendClosePacket(streamID string) {
	closePacket := tunnel.NewClosePacket(connectionID, streamID)
	log.Printf("发送关闭流包: StreamID=%s", streamID)
	_, err := udpConn.Write(closePacket.Bytes())
	if err != nil {
		log.Printf("发送关闭流包错误: %v", err)
	}
}

``` 