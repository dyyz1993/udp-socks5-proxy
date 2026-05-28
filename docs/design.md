# UDP隧道Socks5代理系统设计方案

**记忆ID: PROXY-CS3-UDP-TUNNEL-SOCKS5-20240710**

## 1. 系统概述

本系统实现了一个基于UDP隧道的Socks5代理服务，具有以下特点：

- 客户端接收Socks5协议数据，对外暴露8010端口
- 服务端对外暴露8080端口
- 客户端与服务端之间通过UDP建立通道
- 域名请求在客户端本地直连转发，IP地址请求通过UDP隧道代理转发
- 单一UDP连接支持多请求多路复用

## 2. 系统架构

```mermaid
graph TD
    A[客户端应用] -->|Socks5 协议| B[Client 8010端口]
    B -->|域名请求| C[本地直连]
    B -->|IP请求| D[UDP隧道]
    D -->|UDP协议| E[Server 8080端口]
    E -->|Socks5 协议| F[目标服务器]
```

### 2.1 核心组件

1. **客户端**
   - 接收并解析Socks5请求
   - 实现域名/IP分流逻辑
   - 维护与服务端的UDP隧道连接
   - 使用VirtualSocks5Conn处理回放数据

2. **服务端**
   - 监听UDP端口接收隧道连接
   - 管理多客户端连接
   - 处理隧道数据流转发到目标服务器

3. **隧道通信**
   - 定义数据包格式
   - 实现可靠传输机制
   - 支持流的多路复用

4. **虚拟Socks5连接**
   - 处理已部分读取的Socks5数据
   - 模拟Socks5协议握手过程

## 3. 目录结构

```
proxy-cs3/
├── go.mod
├── go.sum
├── cmd/
│   ├── client/
│   │   └── main.go     # 客户端入口
│   └── server/
│       └── main.go     # 服务端入口
├── internal/
│   ├── client/
│   │   ├── client.go   # 客户端实现
│   │   └── rules.go    # 规则引擎
│   ├── server/
│   │   └── server.go   # 服务端实现
│   └── common/
│       └── logger.go   # 日志实现
└── src/
    ├── tunnel/
    │   ├── interface_connector.go       # 连接器接口
    │   ├── base_connector.go  # 基础连接器
    │   ├── base_stream.go          # 数据流基础实现
    │   ├── packet.go          # 数据包定义
    │   ├── types.go           # 常量与类型
    │   ├── client/            # 客户端实现
    │   │   ├── connector.go   # 客户端连接器
    │   │   └── stream.go      # 客户端流
    │   │   ├── virtual_socks5.go
    │   │   └── virtual_socks5_test.go
    │   └── server/            # 服务端实现
    │       ├── connector.go   # 服务端连接器
    │       └── stream.go      # 服务端流

```

## 4. 详细数据流转

### 4.1 初始化与连接建立阶段

```mermaid
sequenceDiagram
    participant ClientApp as 客户端应用
    participant ClientMain as cmd/client/main.go
    participant ClientImpl as internal/client/client.go
    participant Rules as internal/client/rules.go
    participant Socks5 as src/tunnel/client/virtual_socks5.go
    participant TunnelConnector as src/tunnel/client/connector.go
    participant Logger as internal/common/logger.go
    participant ServerMain as cmd/server/main.go
    participant ServerImpl as internal/server/server.go
    
    %% 客户端初始化
    ClientApp->>+ClientMain: 1. 启动应用
    ClientMain->>+Logger: 2. 创建日志记录器
    Logger-->>-ClientMain: 3. 返回日志实例
    ClientMain->>+ClientImpl: 4. 创建客户端(config, logger)
    ClientImpl->>+Rules: 5. 初始化规则引擎
    Rules-->>-ClientImpl: 6. 返回规则引擎
    ClientImpl->>+TunnelConnector: 7. 创建隧道连接器
    TunnelConnector-->>-ClientImpl: 8. 连接到服务器(UDP)
    ClientImpl-->>-ClientMain: 9. 客户端初始化完成
    
    %% 服务端初始化
    note over ServerMain: 同时进行
    ServerMain->>+Logger: 1. 创建日志记录器
    Logger-->>-ServerMain: 2. 返回日志实例
    ServerMain->>+ServerImpl: 3. 创建服务端
    ServerImpl-->>-ServerMain: 4. 监听UDP端口(8080)
    
    %% SOCKS5连接与判断
    ClientApp->>+ClientImpl: 10. 发起SOCKS5连接(端口8010)
    ClientImpl->>+ClientImpl: 11. 读取初始数据
    ClientImpl->>+ClientImpl: 12. 解析SOCKS5握手
    
    note right of ClientImpl: SOCKS5握手过程:\nVER(5)+NMETHODS+METHODS\n响应: VER(5)+METHOD
    
    ClientImpl->>+ClientImpl: 13. 读取连接请求
    
    note right of ClientImpl: 连接请求:\nVER+CMD+RSV+ATYP+DST.ADDR+DST.PORT
    
    ClientImpl->>+ClientImpl: 14. 提取目标地址
    ClientImpl->>+Rules: 15. 检查规则(域名/IP?)
    Rules-->>-ClientImpl: 16. 返回分流判断
    
    alt 17a. 域名请求
        note over ClientImpl: 转入本地代理流程
    else 17b. IP地址请求
        note over ClientImpl: 转入远程代理流程
    end
```

### 4.2 本地代理转发流程（域名请求）

```mermaid
sequenceDiagram
    participant ClientApp as 客户端应用
    participant ClientImpl as internal/client/client.go
    participant Socks5 as src/tunnel/client/virtual_socks5.go
    participant Socks5Lib as github.com/things-go/go-socks5
    participant Logger as internal/common/logger.go
    participant TargetServer as 目标服务器
    
    %% 已经确定为域名请求，需要本地代理
    ClientApp->>+ClientImpl: 1. SOCKS5请求(域名)
    ClientImpl->>+Socks5: 2. 创建VirtualSocks5Conn
    
    note right of Socks5: 包装已部分读取的SOCKS5请求数据，\n模拟完整的SOCKS5连接
    
    Socks5-->>-ClientImpl: 3. 返回虚拟连接
    
    ClientImpl->>+Socks5Lib: 4. 调用ServeConn(virtualConn)
    
    Socks5Lib->>+Socks5: 5. 读取SOCKS5握手数据
    
    note right of Socks5: 虚拟连接Read()方法:\n- 如果!hasRecvAuth，仅返回握手部分\n- 分段返回VER+NMETHODS和METHODS
    
    Socks5-->>-Socks5Lib: 6. 返回握手数据(VER+NMETHODS+METHODS)
    
    Socks5Lib->>+Socks5: 7. 写入握手响应
    
    note right of Socks5: 虚拟连接Write()方法:\n- 检测到握手响应(长度=2,VER=5)\n- 设置hasRecvAuth=true\n- 设置hasSentAuth=true\n- 不实际写入，仅返回成功
    
    Socks5-->>-Socks5Lib: 8. 写入成功
    
    Socks5Lib->>+Socks5: 9. 读取连接请求
    
    note right of Socks5: 如果hasRecvAuth=true:\n- 返回剩余originalData数据
    
    Socks5-->>-Socks5Lib: 10. 返回请求数据(VER+CMD+RSV+...)
    
    Socks5Lib->>+TargetServer: 11. 直接TCP连接目标服务器
    TargetServer-->>-Socks5Lib: 12. 建立连接
    
    Socks5Lib->>+Socks5: 13. 写入连接响应
    
    note right of Socks5: 虚拟连接Write()方法:\n- 检测到连接响应\n- 设置hasSentConnect=true\n- 实际写入到原始连接
    
    Socks5-->>-Socks5Lib: 14. 写入成功
    
    %% 数据交换阶段
    Socks5Lib->>+Socks5: 15. 从客户端读取应用数据
    
    note right of Socks5: 如果所有原始数据已读取完:\n- 从原始连接读取新数据\n- 同时转发到originalConn
    
    Socks5-->>-Socks5Lib: 16. 返回应用数据
    
    Socks5Lib->>+TargetServer: 17. 转发到目标服务器
    TargetServer->>+Socks5Lib: 18. 返回响应数据
    Socks5Lib->>+Socks5: 19. 写入响应数据
    
    note right of Socks5: 常规数据直接写入原始连接
    
    Socks5-->>-Socks5Lib: 20. 写入成功
    
    note over Socks5Lib, TargetServer: 双向数据流持续...\nClientApp <-> VirtualSocks5Conn <-> TargetServer
    
    Socks5Lib-->>-ClientImpl: 21. 连接处理完成
    ClientImpl-->>-ClientApp: 22. 会话结束
```

### 4.3 远程代理转发流程（IP地址请求）

```mermaid
sequenceDiagram
    participant ClientApp as 客户端应用
    participant ClientImpl as internal/client/client.go
    participant Socks5 as src/tunnel/client/virtual_socks5.go
    participant TunnelConnector as src/tunnel/client/connector.go
    participant TunnelStream as src/tunnel/client/stream.go 
    participant TunnelPacket as src/tunnel/packet.go
    participant ServerImpl as internal/server/server.go
    participant ServerConnector as src/tunnel/server/connector.go
    participant ServerStream as src/tunnel/server/stream.go
    participant Socks5Lib as github.com/things-go/go-socks5
    participant TargetServer as 目标服务器
    
    %% 服务端初始化阶段已创建SOCKS5服务器
    note over ServerImpl: 初始化阶段已创建\nSOCKS5服务器实例
    
    %% 已经确定为IP地址请求，需要远程代理
    ClientApp->>+ClientImpl: 1. SOCKS5请求(IP地址)
    ClientImpl->>+Socks5: 2. 创建VirtualSocks5Conn
    Socks5-->>-ClientImpl: 3. 返回虚拟连接
    
    ClientImpl->>+TunnelConnector: 4. 获取隧道连接
    TunnelConnector-->>-ClientImpl: 5. 返回连接实例
    
    ClientImpl->>+TunnelConnector: 6. 创建到目标的流
    
    note right of TunnelConnector: 目标格式: IP:PORT\n将目标地址封装在数据包中
    
    TunnelConnector->>+TunnelStream: 7. 创建流实例
    TunnelStream-->>-TunnelConnector: 8. 返回流
    
    TunnelConnector->>+TunnelPacket: 9. 创建目标请求数据包
    TunnelPacket-->>-TunnelConnector: 10. 返回封装的数据包
    
    TunnelConnector->>+ServerImpl: 11. 通过UDP发送数据包
    
    note over TunnelConnector, ServerImpl: UDP传输:\n加密 -> 发送 -> 接收 -> 解密
    
    ServerImpl->>+ServerConnector: 12. 处理接收的数据包
    ServerConnector->>+ServerStream: 13. 创建服务端流
    ServerStream-->>-ServerConnector: 14. 返回流实例
    
    ServerConnector-->>-ServerImpl: 15. 流创建完成
    
    ClientImpl->>+Socks5: 16. 发送SOCKS5成功响应
    Socks5-->>-ClientImpl: 17. 响应发送完成
    
    %% 服务端直接使用SOCKS5库处理流
    ServerImpl->>+Socks5Lib: 18. 调用ServeConn(stream)
    
    note right of Socks5Lib: 服务端直接使用现有SOCKS5服务器\n处理流作为连接，因为流中\n已包含完整SOCKS5协议数据
    
    Socks5Lib->>+TargetServer: 19. 建立TCP连接
    TargetServer-->>-Socks5Lib: 20. 连接建立
    
    note over ServerImpl, TunnelConnector: 数据转发通道已建立
    
    %% 数据传输
    ClientApp->>+ClientImpl: 21. 发送请求数据
    ClientImpl->>+TunnelStream: 22. 写入数据
    TunnelStream->>+TunnelConnector: 23. 封装数据包
    TunnelConnector->>+ServerConnector: 24. UDP传输
    ServerConnector->>+ServerStream: 25. 解包数据
    ServerStream->>+Socks5Lib: 26. 传递给SOCKS5服务器
    Socks5Lib->>+TargetServer: 27. 发送到目标
    
    TargetServer->>+Socks5Lib: 28. 返回响应
    Socks5Lib->>+ServerStream: 29. 返回到流
    ServerStream->>+ServerConnector: 30. 封装响应
    ServerConnector->>+TunnelConnector: 31. UDP返回
    TunnelConnector->>+TunnelStream: 32. 接收响应
    TunnelStream->>+ClientImpl: 33. 解包数据
    ClientImpl->>+ClientApp: 34. 转发响应
    
    note over ClientApp, TargetServer: 完整数据路径:\nClientApp <-> ClientImpl <-> TunnelStream <->\nUDP通道 <-> ServerStream <-> SOCKS5服务器 <-> TargetServer
```

## 5. 核心设计要点

### 5.1 VirtualSocks5Conn设计

VirtualSocks5Conn是一个模拟SOCKS5连接的特殊组件，它可以处理已部分读取的SOCKS5协议数据：

```go
// VirtualSocks5Conn 虚拟的SOCKS5连接
type VirtualSocks5Conn struct {
    net.Conn                    // 原始连接
    readBuf        bytes.Buffer // 读取缓冲区
    hasSentAuth    bool         // 是否已发送认证响应
    hasSentConnect bool         // 是否已发送连接响应
    hasRecvAuth    bool         // 是否已收到认证响应
    originalData   []byte       // 原始请求数据
    currentPos     int          // 当前读取位置
    log            Logger       // 日志记录器
    closed         bool         // 连接是否已关闭
    mu             sync.Mutex   // 互斥锁
    originalConn   net.Conn     // 原始连接，用于写回数据
}
```

它的关键功能：
- 分段提供SOCKS5协议数据，模拟完整握手过程
- 智能处理SOCKS5响应，确保协议正确性
- 读取完预缓存数据后，透明传递底层连接的数据

### 5.2 隧道数据包设计

隧道数据包定义了客户端和服务端之间通信的协议格式：

```go
// TunnelPacket 隧道数据包
type TunnelPacket struct {
    Header PacketHeader
    Payload []byte
}

// PacketHeader 数据包头部
type PacketHeader struct {
    Type     PacketType // 数据包类型
    Version  uint8      // 协议版本
    StreamID string     // 流ID
    Length   uint16     // 负载长度
}
```

数据包类型：
- **握手包(HandshakePacket)**: 建立连接时使用
- **数据包(DataPacket)**: 传输实际业务数据
- **目标请求包(TargetRequestPacket)**: 指定目标地址
- **关闭包(ClosePacket)**: 关闭流或连接
- **心跳包(HeartbeatPacket)**: 保持连接活跃
- **错误包(ErrorPacket)**: 传递错误信息

### 5.3 规则分流机制

系统实现了一个简单但高效的分流规则：
- 域名请求本地直连，减少延迟
- IP地址请求通过隧道代理转发
- 支持配置自定义域名规则，包括通配符匹配

```go
// ShouldDirectConnect 判断是否应该直连
func (r *RuleEngine) ShouldDirectConnect(addr string) bool {
    host, _, err := net.SplitHostPort(addr)
    if err != nil {
        return r.defaultDirect
    }
    
    // 判断是否是IP地址
    ip := net.ParseIP(host)
    if ip != nil {
        // IP地址使用代理
        return false
    }
    
    // 域名检查规则
    for _, rule := range r.domainRules {
        if matchDomain(host, rule) {
            return true
        }
    }
    
    return r.defaultDirect
}
```

### 5.4 单UDP连接多路复用

系统在单一UDP连接上实现了多个客户端请求的多路复用：
- 每个请求分配唯一流ID
- 数据包中包含流ID，用于区分不同请求
- 服务端为每个流单独处理目标连接

## 6. 部署和使用

### 6.1 编译

```bash
# 编译客户端
go build -o client ./cmd/client

# 编译服务端
go build -o server ./cmd/server
```

### 6.2 配置

客户端配置示例 (config.yaml):
```yaml
client:
  local_port: 8010
  server_addr: "example.com:8080"
  timeout: 60s
  rules:
    direct_domains:
      - "*.baidu.com"
      - "*.qq.com"
    default_direct: true
```

### 6.3 运行

启动服务端:
```bash
./server -port 8080
```

启动客户端:
```bash
./client -config config.yaml
```

使用代理:
```bash
# 设置SOCKS5代理
export http_proxy=socks5://127.0.0.1:8010
export https_proxy=socks5://127.0.0.1:8010

# 测试
curl https://www.example.com
```

## 7. 总结

本设计方案实现了一个高效的UDP隧道Socks5代理系统，具有以下优势：

1. **智能分流**: 域名本地直连，IP地址隧道代理，平衡性能与隐私
2. **资源高效**: 单一UDP连接支持多请求多路复用
3. **协议兼容**: 完全兼容标准SOCKS5协议
4. **模块化设计**: 清晰的接口定义和组件职责分离

该系统适用于需要在确保网络安全性的同时，优化性能和资源利用的场景。

---

**记忆ID: PROXY-CS3-UDP-TUNNEL-SOCKS5-20240710** 