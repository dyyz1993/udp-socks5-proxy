# UDP通道SOCKS5代理系统

一个基于UDP通道的SOCKS5代理系统，支持智能分流和高效连接复用。

**项目ID**: PROXY-CS3-UDP-TUNNEL-SOCKS5-20240710

## 主要特点

- 基于UDP通道的高效传输
- 支持标准SOCKS5协议
- 智能流量分流
- 单UDP连接多通道复用
- 模拟各种网络环境的测试框架

## 组件说明

- **客户端**：运行在本地，提供SOCKS5代理服务，支持直连和远程代理智能分流
- **服务端**：部署在远程服务器，负责处理客户端转发的请求
- **通道**：客户端和服务端之间的UDP通信隧道
- **测试框架**：模拟各种网络环境的测试工具，用于验证系统稳定性

## 项目结构

```
.
├── cmd/                 # 命令行入口
│   ├── client/          # 客户端入口
│   └── server/          # 服务端入口
├── internal/            # 内部实现
│   ├── client/          # 客户端核心
│   ├── common/          # 共享代码
│   └── server/          # 服务端核心
├── src/                 # 源代码
│   └── tunnel/          # 通道实现
│       ├── client/      # 客户端通道
│       ├── server/      # 服务端通道
│       └── testing/     # 测试辅助工具
├── docs/                # 文档
│   ├── design.md        # 详细设计文档
│   └── testing_framework.md # 测试框架文档
├── Makefile             # 编译脚本
└── README.md            # 本文件
```

## 编译说明

使用提供的Makefile进行编译：

```bash
# 编译全部
make all

# 仅编译客户端
make client

# 仅编译服务端
make server

# 运行测试
make test

# 清理编译产物
make clean
```

## 使用方法

### 服务端

```bash
./bin/server -port 8888 -log-level info
```

### 客户端

```bash
./bin/client -local 1080 -server server.example.com:8888 -direct="*.cn,*.local" -log-level info
```

配置参数：
- `-local`：本地SOCKS5监听端口
- `-server`：远程服务器地址和端口
- `-direct`：直连域名列表，逗号分隔，支持通配符
- `-default-direct`：默认是否直连，不符合规则的域名采用此策略
- `-timeout`：连接超时时间(秒)
- `-log-level`：日志级别(debug, info, warn, error)

## 网络测试框架

项目包含一个强大的网络测试框架，用于在各种网络条件下测试系统的稳定性和可靠性。测试框架主要由以下组件组成：

1. **模拟网络连接 (MockNetConn)**：模拟真实网络环境中的各种情况，包括延迟、丢包、错误等
2. **网络模拟器 (NetworkSimulator)**：管理一组模拟连接，根据预设的事件序列改变这些连接的网络条件
3. **场景管理器 (SimulatorManager)**：提供更高层次的抽象，使用预定义的场景来模拟完整的网络环境变化过程

详细说明请参考[测试框架设计文档](docs/testing_framework.md)。

## 核心设计

- **虚拟SOCKS5连接**：将标准SOCKS5连接转换为虚拟连接，在UDP通道中传输
- **通道数据包**：定义了不同类型的数据包，包括控制、连接管理和数据传输
- **规则引擎**：根据域名和IP规则决定连接策略
- **多路复用**：单UDP连接支持多个并发SOCKS5请求

详细设计请参考[详细设计文档](docs/design.md)。

## 开发计划

- [x] 基础通道实现
- [x] SOCKS5协议支持
- [x] 智能分流规则
- [x] 网络测试框架
- [ ] 流量统计与监控
- [ ] Web管理界面
- [ ] 数据压缩与加密
- [ ] 更多协议适配

## 许可证

[MIT License](LICENSE)
