# 服务端通讯协议规范 (PROXY-CS3-UDP-TUNNEL-SOCKS5)

本文档定义了客户端与服务端之间通过UDP隧道进行通讯时，服务端所期望接收的数据包格式和协议行为。

## 1. 协议概述

客户端与服务端之间通过UDP进行通信。为了保证数据传输的可靠性，实现流的多路复用，并处理大数据包，定义了一套自定义的隧道协议。

- **连接管理**: 通过 `ConnectionID` (UUID) 区分不同的UDP连接。
- **流管理**: 在单个UDP连接上，通过 `StreamID` (UUID) 实现多个逻辑数据流的并发传输。
- **数据包类型**: 定义了多种数据包类型，用于不同的通讯目的（握手、数据传输、心跳、关闭、错误、分片）。
- **分片机制**: 对于超过UDP MTU（本项目设定为 `MaxUDPPacketSize = 8192` 字节）的数据包，进行分片传输，并在接收端重组。

## 2. 核心常量

```golang
// src/tunnel/packet.go
const (
  ProtocolVersion = 1 // 当前协议版本

  // 数据包类型
  PacketTypeHandshake  = 1 // 握手包
  PacketTypeData       = 2 // 数据包
  PacketTypeHeartbeat  = 3 // 心跳包
  PacketTypeClose      = 4 // 关闭流包
  PacketTypeError      = 5 // 错误包
  PacketTypeFragmented = 6 // 分片数据包

  // 分片常量
  MaxUDPPacketSize    = 8192 // UDP包大小限制 (理论值约9K, 保守取8K)
  FragmentHeaderSize  = 20   // 预估分片头部额外开销大小
  MaxFragmentDataSize = MaxUDPPacketSize - FragmentHeaderSize // 每个分片能承载的最大业务数据
)
```

## 3. 通用数据包结构

所有通过隧道传输的数据包都遵循以下基本结构：

**基础头部 (Header)**

| 字段           | 类型   | 大小 (字节) | 描述                                               |
| -------------- | ------ | ----------- | -------------------------------------------------- |
| Version        | uint8  | 1           | 协议版本 (当前为 `1`)                              |
| Type           | uint8  | 1           | 数据包类型 (见 `PacketType*` 常量)               |
| Flags          | uint8  | 1           | 标志位 (当前主要用于分片，其他类型为0)              |
| ConnectionID长度 | uint16 | 2           | `ConnectionID` 字段的字节长度 (大端序)              |
| ConnectionID   | string | 可变        | 标识UDP连接的UUID字符串                           |
| StreamID长度   | uint16 | 2           | `StreamID` 字段的字节长度 (大端序，**仅在Data, Close, Fragmented包中存在**) |
| StreamID       | string | 可变        | 标识逻辑数据流的UUID字符串 (**仅在Data, Close, Fragmented包中存在**) |

**数据包体 (Payload)**

| 字段 | 类型   | 大小 (字节) | 描述                                                   |
| ---- | ------ | ----------- | ------------------------------------------------------ |
| Data | []byte | 可变        | 数据包的实际载荷，具体格式取决于 `Type` 字段          |

**注意:** 序列化时，所有多字节整数（如长度字段）均使用 **大端序 (BigEndian)**。

## 4. 特定数据包类型详解

### 4.1 握手包 (PacketTypeHandshake = 1)

用于客户端与服务端建立隧道连接时的初始认证和信息交换。

- **头部 (Header)**:
  - `Type`: 1
  - `StreamID`: **不存在**
- **数据体 (Data)**:
  | 字段         | 类型   | 大小 (字节) | 描述                                         |
  | ------------ | ------ | ----------- | -------------------------------------------- |
  | Key          | [32]byte | 32          | 握手密钥 (预共享或动态生成)                    |
  | Group长度    | uint16 | 2           | `Group` 字段的字节长度 (大端序)                |
  | Group        | string | 可变        | 客户端所属分组名称 (用于管理或策略)            |
  | Features     | uint32 | 4           | 客户端支持的特性标志位 (大端序)                |
  | Version长度  | uint16 | 2           | `Version` 字段的字节长度 (大端序)              |
  | Version      | string | 可变        | 客户端软件版本号                              |

### 4.2 数据包 (PacketTypeData = 2)

用于在已建立的逻辑流上传输实际的应用数据（如SOCKS5协议数据）。

- **头部 (Header)**:
  - `Type`: 2
  - `StreamID`: **存在**，标识数据所属的流。
- **数据体 (Data)**:
  | 字段     | 类型   | 大小 (字节) | 描述                                 |
  | -------- | ------ | ----------- | ------------------------------------ |
  | AppData  | []byte | 可变        | 实际的应用层数据（例如SOCKS5请求/响应） |

### 4.3 心跳包 (PacketTypeHeartbeat = 3)

用于维持UDP连接的活跃状态，检测连接是否中断。

- **头部 (Header)**:
  - `Type`: 3
  - `StreamID`: **不存在**
- **数据体 (Data)**:
  | 字段      | 类型    | 大小 (字节) | 描述                               |
  | --------- | ------- | ----------- | ---------------------------------- |
  | Timestamp | int64   | 8           | 发送心跳时的时间戳 (Unix Nano, 大端序) |
  | Sequence  | uint32  | 4           | 心跳序列号 (大端序)                   |
  | Load      | float32 | 4           | 客户端报告的负载信息 (可选, 大端序)    |

**服务端行为**: 收到心跳包后，通常会回复一个心跳包，并更新对应 `ConnectionID` 的最后活跃时间。

### 4.4 关闭流包 (PacketTypeClose = 4)

用于通知对端某个逻辑数据流需要关闭。

- **头部 (Header)**:
  - `Type`: 4
  - `StreamID`: **存在**，标识需要关闭的流。
- **数据体 (Data)**:
  - **为空**

**服务端行为**: 收到关闭包后，找到对应的 `Stream` 对象，执行关闭逻辑（例如关闭与目标服务器的连接），并从 `Connector` 中移除该流。

### 4.5 错误包 (PacketTypeError = 5)

用于在发生错误时通知对端。

- **头部 (Header)**:
  - `Type`: 5
  - `StreamID`: **不存在**
- **数据体 (Data)**:
  | 字段          | 类型   | 大小 (字节) | 描述                                          |
  | ------------- | ------ | ----------- | --------------------------------------------- |
  | Code          | int32  | 4           | 错误码 (大端序)                                |
  | Message长度   | uint16 | 2           | `Message` 字段的字节长度 (大端序)               |
  | Message       | string | 可变        | 错误的详细描述信息                            |
  | RelatedID长度 | uint16 | 2           | `RelatedID` 字段的字节长度 (大端序)             |
  | RelatedID     | string | 可变        | 与错误相关的ID (可能是 ConnectionID 或 StreamID) |

### 4.6 分片数据包 (PacketTypeFragmented = 6)

当原始数据包（通常是 `PacketTypeData`）的大小超过 `MaxUDPPacketSize` 时，会被拆分成多个分片包进行传输。

- **头部 (Header)**:
  - `Type`: 6
  - `Flags`: 用于标识分片状态 (见下文)。
  - `StreamID`: **存在**，标识分片所属的流。
- **数据体 (Data)**:
  | 字段           | 类型   | 大小 (字节) | 描述                                        |
  | -------------- | ------ | ----------- | ------------------------------------------- |
  | SequenceID     | uint32 | 4           | 唯一标识一组分片的序列号 (大端序)             |
  | TotalFragments | uint32 | 4           | 当前这组分片总共有多少个 (大端序)             |
  | FragmentIndex  | uint32 | 4           | 当前分片是第几个 (从0开始, 大端序)            |
  | OriginalType   | uint8  | 1           | 被分片的原始数据包类型 (通常是 `PacketTypeData=2`) |
  | FragmentData   | []byte | 可变        | 当前分片包含的数据块                          |

**分片标志位 (Header.Flags)**:

| 标志位名称         | 值    | 描述                       |
| ------------------ | ----- | -------------------------- |
| FragmentFlagStart  | 1<<0  | 表示这是该组分片的第一个分片 |
| FragmentFlagEnd    | 1<<1  | 表示这是该组分片的最后一个分片 |

**注意**: `Flags` 字段可以包含多个标志位的组合 (例如，如果只有一个分片，则 `Flags` 会同时包含 `FragmentFlagStart` 和 `FragmentFlagEnd`)。

## 5. 分片与重组机制 (服务端视角)

1.  **接收分片**: 服务端 `ServerConnector` 收到 `Type` 为 `PacketTypeFragmented` 的数据包。
2.  **解析分片头**: 从数据体中解析出 `SequenceID`, `TotalFragments`, `FragmentIndex`, `OriginalType`。
3.  **缓存分片**:
    - 使用一个内部缓存 (`fragmentManager`)，以 `SequenceID` 为键，存储收到的分片。
    - 每个 `SequenceID` 对应一个预分配好大小 (`TotalFragments`) 的分片数组（或切片）。
    - 将当前收到的分片 `fragmentData` 存放到数组中 `FragmentIndex` 对应的位置。
    - 同时记录每个 `SequenceID` 的过期时间（例如，收到该序列的第一个分片后的30秒）。
4.  **检查完整性**: 每次收到一个分片后，检查对应 `SequenceID` 的分片数组是否所有位置都已填满（即所有分片都已收到）。
5.  **合并分片**:
    - 如果所有分片都已收到，则按照 `FragmentIndex` 的顺序将所有 `FragmentData` 拼接起来，形成原始的数据体。
    - 清理缓存中该 `SequenceID` 的所有分片数据和过期时间记录。
6.  **构造原始包**: 使用合并后的数据和分片头中的 `OriginalType`，以及通用头部的 `ConnectionID`, `StreamID` 等信息，重新构造出原始的 `TunnelPacket`。
7.  **处理原始包**: 将重组后的原始数据包（通常是 `DataPacket`）交给相应的处理函数（如 `handleDataPacket`）进行后续处理，就像处理未分片的包一样。
8.  **超时清理**: 定期（或在每次添加分片时）检查缓存，移除超过过期时间仍未完整的 `SequenceID` 及其对应的所有分片，防止内存泄漏。

## 6. 服务端关键处理逻辑

- **`ServerConnector.ProcessIncomingData`**:
  - 解析收到的UDP数据，识别基础包头。
  - 根据 `Header.Type` 将数据包分发给相应的处理函数 (`handleHandshakePacket`, `handleDataPacket`, `handleFragmentPacket` 等)。
- **`ServerConnector.handleHandshakePacket`**:
  - 验证握手信息（如Key）。
  - 设置 `ConnectionID`。
  - 回复握手成功的响应（或错误）。
- **`ServerConnector.handleDataPacket`**:
  - 根据 `Header.StreamID` 查找或创建 `ServerStream` 实例。
  - 如果是新流的第一个数据包，通常包含SOCKS5的连接请求，需要解析目标地址并连接目标服务器。
  - 将 `DataPacket.Data` 放入对应 `ServerStream` 的缓冲区，供上层（SOCKS5服务器逻辑）读取。
- **`ServerConnector.handleFragmentPacket`**:
  - 调用 `fragmentManager.addFragment` 将分片存入缓存。
  - 如果 `addFragment` 返回了合并后的完整数据包，则将该包交给对应的原始类型处理函数（通常是 `handleDataPacket`）。
- **`ServerConnector.handleClosePacket`**:
  - 根据 `Header.StreamID` 查找 `ServerStream`。
  - 调用 `Stream.Close()` 关闭流和相关资源。
  - 从连接器中移除该流。
- **`ServerConnector.handleHeartbeatPacket`**:
  - 更新连接的最后活跃时间。
  - 可以选择回复一个心跳响应。
- **`ServerConnector.handleErrorPacket`**:
  - 记录错误日志。
  - 根据错误严重程度可能关闭连接或特定流。

## 7. 注意事项

- 所有字符串（如ID、Group、Version、Message、RelatedID）的编码应为UTF-8。
- 服务端实现需要处理并发，确保对共享资源（如流映射、分片缓存）的访问是线程安全的。
- 分片重组需要设置超时机制，防止因丢包导致内存无限增长。
- 错误处理需要健壮，能够向客户端反馈有意义的错误信息。

## 8. 最小服务端实现示例

下面是一个使用Go语言实现的、遵循此协议的最小服务端骨架示例。它展示了如何监听端口、接收和解析数据包，并对几种关键包类型进行基础处理。

**注意:** 此示例仅为演示协议处理流程，省略了完整的分片重组、流管理、SOCKS5逻辑、错误处理、超时和并发安全等生产级实现细节。

```go
package main

import (
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tealife/proxy-cs3/src/tunnel" // 假设你的项目路径配置正确
)

const (
	listenAddr = "0.0.0.0:8080" // 服务端监听地址和端口
)

// 简单模拟连接状态管理
var connections = make(map[string]*net.UDPAddr) // Key: ConnectionID, Value: 客户端地址

func main() {
	addr, err := net.ResolveUDPAddr("udp", listenAddr)
	if err != nil {
		log.Fatalf("无法解析监听地址: %v", err)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Fatalf("无法监听UDP端口 %s: %v", listenAddr, err)
	}
	defer conn.Close()
	log.Printf("服务端正在监听 %s", listenAddr)

	buffer := make([]byte, tunnel.MaxUDPPacketSize*2) // 缓冲区需要足够大以容纳最大UDP包

	for {
		n, remoteAddr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			log.Printf("读取UDP数据错误: %v", err)
			continue
		}

		data := make([]byte, n)
		copy(data, buffer[:n]) // 复制数据以供处理

		// 异步处理数据包，避免阻塞监听循环
		go handlePacket(conn, remoteAddr, data)
	}
}

// handlePacket 处理接收到的单个UDP数据包
func handlePacket(conn *net.UDPConn, remoteAddr *net.UDPAddr, data []byte) {
	packet, err := tunnel.ParsePacket(data)
	if err != nil {
		log.Printf("解析数据包错误 from %s: %v", remoteAddr, err)
		return
	}

	connectionID := packet.Header.ConnectionID
	log.Printf("收到来自 %s 的数据包: Type=%d, ConnID=%s, StreamID=%s, Size=%d",
		remoteAddr, packet.Header.Type, connectionID, packet.Header.StreamID, len(data))

	// 简单连接管理：记录第一次看到某个ConnectionID的地址
	if _, ok := connections[connectionID]; !ok && connectionID != "" {
		log.Printf("记录新连接: ConnID=%s, RemoteAddr=%s", connectionID, remoteAddr)
		connections[connectionID] = remoteAddr
	}

	switch packet.Header.Type {
	case tunnel.PacketTypeHandshake:
		handleHandshake(conn, remoteAddr, packet)
	case tunnel.PacketTypeData:
		handleData(conn, remoteAddr, packet)
	case tunnel.PacketTypeHeartbeat:
		handleHeartbeat(conn, remoteAddr, packet)
	case tunnel.PacketTypeFragmented:
		handleFragment(conn, remoteAddr, packet)
	case tunnel.PacketTypeClose:
		handleClose(conn, remoteAddr, packet)
	default:
		log.Printf("收到未处理的包类型: %d from %s", packet.Header.Type, remoteAddr)
	}
}

// handleHandshake 处理握手包 (最小示例：仅记录和回复)
func handleHandshake(conn *net.UDPConn, remoteAddr *net.UDPAddr, packet *tunnel.TunnelPacket) {
	// 实际应用中需要解析HandshakePacket内容并验证Key等
	log.Printf("处理握手包: ConnID=%s", packet.Header.ConnectionID)

	// 简单回复一个握手确认 (实际需要构建正确的Handshake响应包)
	// 这里仅用一个简单的错误包示意回复
	errMsg := fmt.Sprintf("Handshake received for %s", packet.Header.ConnectionID)
	respPacket := tunnel.NewErrorPacket(packet.Header.ConnectionID, 0, errMsg, packet.Header.ConnectionID)
	_, err := conn.WriteToUDP(respPacket.Bytes(), remoteAddr)
	if err != nil {
		log.Printf("回复握手失败 to %s: %v", remoteAddr, err)
	}
}

// handleData 处理数据包 (最小示例：仅记录，实际需转发给SOCKS5逻辑)
func handleData(conn *net.UDPConn, remoteAddr *net.UDPAddr, packet *tunnel.TunnelPacket) {
	log.Printf("处理数据包: ConnID=%s, StreamID=%s, DataLen=%d",
		packet.Header.ConnectionID, packet.Header.StreamID, len(packet.Data))

	// 实际应用中:
	// 1. 根据StreamID找到对应的Stream实例 (如果不存在则创建)
	// 2. 如果是新流的第一个包，解析SOCKS5目标地址，连接目标服务器
	// 3. 将packet.Data写入Stream的缓冲区，供SOCKS5处理逻辑读取
	// 4. 从目标服务器读取响应，通过SendData发送回客户端 (可能需要分片)

	// 示例：简单回显部分数据
	echoData := []byte("Server received your data on stream " + packet.Header.StreamID)
	// 注意：实际回复需要使用隧道协议包装，这里仅作示意
	// _, err := conn.WriteToUDP(echoData, remoteAddr)
	// if err != nil {
	//  log.Printf("回复数据失败 to %s: %v", remoteAddr, err)
	// }
}

// handleHeartbeat 处理心跳包 (最小示例：仅记录和回复)
func handleHeartbeat(conn *net.UDPConn, remoteAddr *net.UDPAddr, packet *tunnel.TunnelPacket) {
	log.Printf("处理心跳包: ConnID=%s", packet.Header.ConnectionID)
	// 更新连接活跃时间 (实际应用需要)

	// 回复心跳
	// 实际应用中需要解析请求中的Sequence并包含在响应中
	respPacket := tunnel.NewHeartbeatPacket(packet.Header.ConnectionID, 0, 0.0) // sequence和load为示例值
	_, err := conn.WriteToUDP(respPacket.Bytes(), remoteAddr)
	if err != nil {
		log.Printf("回复心跳失败 to %s: %v", remoteAddr, err)
	}
}

// handleFragment 处理分片包 (最小示例：仅记录，实际需要缓存和重组)
func handleFragment(conn *net.UDPConn, remoteAddr *net.UDPAddr, packet *tunnel.TunnelPacket) {
	// 实际应用中需要解析FragmentPacket头部
	// fragment, err := tunnel.ParseFragmentPacket(packet)
	// if err != nil { ... }
	log.Printf("处理分片包: ConnID=%s, StreamID=%s, Flags=%d, DataLen=%d",
		packet.Header.ConnectionID, packet.Header.StreamID, packet.Header.Flags, len(packet.Data))

	// 实际应用中:
	// 1. 调用分片管理器(fragmentManager)添加分片
	// 2. 如果分片完整，管理器会返回重组后的原始包
	// 3. 将重组后的包交给原始类型的处理函数 (如handleData)
}

// handleClose 处理关闭流包 (最小示例：仅记录)
func handleClose(conn *net.UDPConn, remoteAddr *net.UDPAddr, packet *tunnel.TunnelPacket) {
	log.Printf("处理关闭流包: ConnID=%s, StreamID=%s",
		packet.Header.ConnectionID, packet.Header.StreamID)
	// 实际应用中:
	// 1. 找到对应的Stream实例
	// 2. 关闭与目标服务器的连接等资源
	// 3. 从连接管理中移除该Stream
}

``` 