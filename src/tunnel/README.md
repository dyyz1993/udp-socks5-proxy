# 隧道通信系统

本包实现了一个完整的隧道通信系统，用于在不同网络节点之间建立安全、可靠的数据传输通道。主要包含了数据包定义、连接管理和数据流处理等核心组件。

## 主要组件

### 1. 数据包系统 (packet.go)

定义了隧道通信使用的各种数据包类型及其序列化/反序列化方法：

- **基础数据包** (TunnelPacket)：所有数据包的基类
- **握手包** (HandshakePacket)：用于建立连接时的认证和参数协商
- **数据包** (DataPacket)：用于传输实际业务数据
- **心跳包** (HeartbeatPacket)：用于保持连接活跃和检测连接状态
- **关闭包** (ClosePacket)：用于优雅关闭连接或数据流
- **错误包** (ErrorPacket)：用于传递错误信息

### 2. 连接管理 (connection.go)

实现了隧道连接的核心功能：

- 连接建立和维护
- 数据包的发送和接收
- 连接状态管理
- 多路复用支持
- 错误处理机制

### 3. 数据流处理 (stream.go)

实现了在单个隧道连接上多路复用多个数据流的能力：

- 数据流创建和管理
- 双向数据传输
- 流的生命周期管理
- 错误恢复机制

## 使用示例

### 建立隧道连接

```go
// 创建连接
conn, err := net.Dial("tcp", "server.example.com:8080")
if err != nil {
    log.Fatalf("连接失败: %v", err)
}

// 创建唯一连接ID
connID := tunnel.GenerateConnectionID()

// 创建数据包处理函数
handler := func(packet *tunnel.TunnelPacket) error {
    // 处理接收到的数据包
    log.Printf("收到数据包，类型: %d", packet.Header.Type)
    return nil
}

// 创建隧道连接
tunnelConn := tunnel.NewTunnelConnection(connID, conn, handler)

// 设置连接模式
tunnelConn.SetMode("client")

// 启动连接处理循环（在goroutine中运行）
go tunnelConn.Run()

// 当不再需要连接时关闭
defer tunnelConn.Close()
```

### 创建和使用数据流

```go
// 创建到指定目标的流
stream, err := tunnelConn.CreateStreamTo("target.example.com:80")
if err != nil {
    log.Fatalf("创建流失败: %v", err)
}

// 处理本地连接
localConn, err := listener.Accept()
if err != nil {
    log.Fatalf("接受连接失败: %v", err)
}

// 使用流处理连接
go stream.ServeConn(localConn)
```

## 设计特点

1. **模块化设计**：各组件职责明确，便于维护和扩展
2. **高并发支持**：使用goroutine和适当的锁机制支持高并发场景
3. **强类型系统**：利用Go的类型系统提供类型安全
4. **完善的错误处理**：详细的错误码和信息，帮助快速定位问题
5. **丰富的日志**：关键操作都有详细日志记录，便于调试和问题排查

## 配置选项

可以通过以下方法配置隧道连接的行为：

- `SetReadTimeout`/`SetWriteTimeout`：设置读写超时时间
- `SetMode`：设置连接模式（客户端/服务端）
- `EnableEncryption`：启用数据加密（注：当前版本加密功能尚未完全实现）

## 注意事项

- 确保在使用连接前启动`Run()`方法
- 所有流使用完毕后需要调用`Close()`方法释放资源
- 数据包处理函数应尽可能高效，避免阻塞
- 错误处理应考虑网络异常、超时等常见问题 