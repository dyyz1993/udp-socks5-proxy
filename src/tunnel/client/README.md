# VirtualSocks5Conn - 虚拟SOCKS5连接组件

这个组件提供了一个虚拟SOCKS5连接的实现，主要用于模拟SOCKS5协议的握手流程。它可以在不修改现有SOCKS5服务器代码的情况下，帮助处理已经部分读取的SOCKS5协议数据。

## 主要功能

1. 模拟SOCKS5协议的握手流程
2. 处理已部分读取的SOCKS5协议数据
3. 实现标准的`net.Conn`接口，可以无缝集成到现有代码
4. 支持完整的SOCKS5握手和连接请求处理

## 使用场景

在代理服务器中，有时候我们需要先读取一部分数据来确定连接类型（如是否是SOCKS5请求），然后再进行相应处理。这种情况下，如果确定是SOCKS5请求，我们又希望能够将已读取的数据"放回去"，以便SOCKS5服务器能够正常处理完整的协议流程。

`VirtualSocks5Conn`正是为解决这一问题而设计的。它包装了原始连接，并保存了已读取的SOCKS5协议数据，当SOCKS5服务器从这个虚拟连接读取数据时，它会先返回这些预先保存的数据，然后再从实际连接读取后续数据。

## 使用示例

```go
package main

import (
    "net"
    "your-project/src/socks5"
    "your-project/pkg/logger"
)

func handleConnection(conn net.Conn) {
    // 1. 先读取一部分数据，判断是否是SOCKS5请求
    buffer := make([]byte, 1024)
    n, err := conn.Read(buffer)
    if err != nil {
        // 处理错误
        return
    }
    
    // 2. 判断是否是SOCKS5请求（简化判断逻辑）
    if n >= 3 && buffer[0] == 0x05 {
        // 是SOCKS5请求
        log := logger.NewLogger() // 创建日志记录器
        
        // 3. 创建虚拟SOCKS5连接
        virtualConn := socks5.NewVirtualSocks5Conn(conn, buffer[:n], log)
        
        // 4. 使用现有的SOCKS5服务器处理这个连接
        socks5Server := GetSocks5Server()
        socks5Server.ServeConn(virtualConn)
    } else {
        // 不是SOCKS5请求，按其他协议处理
        // ...
    }
}
```

## 注意事项

1. **分段读取行为**: `VirtualSocks5Conn`的读取行为是基于SOCKS5协议结构设计的，它会按照协议格式分段返回数据：
   - 首先返回前2字节(VER+NMETHODS)
   - 然后返回METHODS字段
   - 在认证完成后，返回剩余的请求数据

   这意味着调用方需要做好分多次读取的准备，不能期望一次调用就读取所有数据。

2. **状态管理**: 组件内部会维护协议状态，包括认证、连接等阶段的状态标记，以确保按照SOCKS5协议的正确顺序处理数据。

3. **测试考虑**: 在编写测试时，应该模拟SOCKS5协议的实际交互流程，分步骤读取和处理数据，而不是期望一次性读取整个握手请求。

## 依赖

- 标准库：`io`, `net`, `bytes`, `sync`
- 自定义Logger接口，需要实现以下方法：
  - `Debug(args ...interface{})`
  - `Debugf(format string, args ...interface{})`
  - `Info(args ...interface{})`
  - `Infof(format string, args ...interface{})`
  - `Error(args ...interface{})`
  - `Errorf(format string, args ...interface{})`

## 实现细节

### SOCKS5协议流程

1. **握手阶段**：
   - 客户端发送握手请求：`VER(5) | NMETHODS | METHODS`
   - 服务端回复：`VER(5) | METHOD`

2. **请求阶段**：
   - 客户端发送请求：`VER(5) | CMD | RSV | ATYP | DST.ADDR | DST.PORT`
   - 服务端回复：`VER(5) | REP | RSV | ATYP | BND.ADDR | BND.PORT`

3. **数据传输阶段**：
   - 建立连接后，开始双向数据传输

`VirtualSocks5Conn`会跟踪当前所处的协议阶段，根据不同阶段采取不同的处理逻辑。

## 许可证

MIT 