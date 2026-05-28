# 网络测试框架设计文档

## 概述

本文档描述了代理系统中的网络测试框架，主要包括网络连接模拟、网络事件模拟和网络场景管理三个核心组件。这些组件共同提供了一个强大的网络环境模拟系统，可以在测试环境中模拟各种复杂的网络情况，以验证代理系统在不同网络条件下的稳定性和可靠性。

## 核心组件

### 1. 模拟网络连接 (MockNetConn)

`MockNetConn` 实现了标准的 `net.Conn` 接口，但可以模拟现实网络中常见的各种情况：

- **延迟模拟**：可以设置读取和写入操作的延迟，模拟网络延迟
- **丢包模拟**：可以设置丢包率，随机丢弃一部分数据包
- **错误模拟**：可以设置读取和写入操作的错误率，随机产生错误
- **连接中断模拟**：可以模拟连接中断和恢复
- **超时模拟**：可以模拟读写超时

预定义了三种网络条件配置：

1. **良好网络**：低延迟，无丢包，无错误
2. **较差网络**：中等延迟，低丢包率，低错误率
3. **不稳定网络**：高延迟，高丢包率，高错误率

### 2. 网络模拟器 (NetworkSimulator)

`NetworkSimulator` 管理一组模拟连接，并根据预设的事件序列改变这些连接的网络条件：

- **事件系统**：支持多种网络事件类型，包括连接断开、重连、延迟增加/减少、丢包率增加/减少、错误率增加/减少等
- **时间调度**：事件按照预设的时间顺序执行，模拟随时间变化的网络环境
- **集中管理**：集中管理多个连接的状态，一次改变可以影响所有受管理的连接
- **自定义事件**：支持添加自定义事件和事件处理函数

### 3. 场景管理器 (SimulatorManager)

`SimulatorManager` 在网络模拟器的基础上提供了更高层次的抽象，使用预定义的场景来模拟完整的网络环境变化过程：

- **场景管理**：预定义多种常见的网络场景，如稳定网络、不稳定网络、间歇性连接等
- **场景切换**：支持在不同场景之间切换，模拟网络环境的变化
- **自定义场景**：允许用户定义自己的网络场景
- **自动重置**：支持场景结束后自动恢复网络状态

## 预定义场景

框架预定义了以下网络场景：

1. **稳定场景 (stable)**：
   - 稳定的网络环境，低延迟，无丢包
   - 适合测试系统在理想网络条件下的性能和正确性

2. **不稳定场景 (unstable)**：
   - 网络条件波动，延迟时高时低，偶尔丢包
   - 适合测试系统对网络不稳定的适应能力

3. **间歇性连接场景 (intermittent)**：
   - 连接周期性断开和恢复
   - 适合测试系统对连接中断的处理和恢复能力

4. **逐渐恶化场景 (degrading)**：
   - 网络条件从好到坏逐渐恶化，最终断开
   - 适合测试系统在网络逐渐变差情况下的降级策略

5. **逐渐改善场景 (improving)**：
   - 网络条件从差到好逐渐改善
   - 适合测试系统在网络恢复过程中的行为

## 使用指南

### 基本使用

```go
// 创建模拟网络连接
conn := testing.NewMockNetConn()

// 创建网络模拟器
simulator := testing.NewNetworkSimulator()

// 添加连接到模拟器
simulator.AddConnection(conn)

// 添加网络事件
simulator.AddEvent(30*time.Second, testing.NetworkEvent{
    Type: testing.EventTypeLatencyIncrease,
    Params: map[string]interface{}{"factor": 2.0},
})

// 启动模拟器
simulator.Start()

// ... 执行测试 ...

// 停止模拟器
simulator.Stop()
```

### 使用场景管理器

```go
// 创建场景管理器
manager := testing.NewSimulatorManager()

// 创建网络连接
conn := testing.NewMockNetConn()

// 添加连接到管理器
manager.AddConnection(conn)

// 运行预定义场景
manager.RunScenario("unstable")

// 等待场景完成
manager.WaitForCompletion()

// 或手动停止场景
// manager.Stop()
```

### 自定义场景

```go
// 创建自定义场景
myScenario := testing.SimulationScenario{
    Name:        "my_custom_scenario",
    Description: "自定义场景示例",
    Events: []testing.NetworkEvent{
        {
            Type: testing.EventTypeLatencyIncrease,
            Time: 10 * time.Second,
            Params: map[string]interface{}{"factor": 1.5},
        },
        {
            Type: testing.EventTypeDisconnect,
            Time: 30 * time.Second,
        },
        {
            Type: testing.EventTypeReconnect,
            Time: 40 * time.Second,
        },
    },
    Duration: 1 * time.Minute,
}

// 添加到场景管理器
manager.AddScenario(myScenario)

// 运行自定义场景
manager.RunScenario("my_custom_scenario")
```

## 在集成测试中的应用

网络测试框架可以在以下几种测试场景中发挥重要作用：

1. **单元测试**：测试组件在各种网络条件下的行为
2. **集成测试**：测试系统在复杂网络环境下的整体表现
3. **压力测试**：通过模拟极端网络条件，测试系统的稳定性和容错能力
4. **恢复测试**：测试系统从网络故障中恢复的能力
5. **长时间稳定性测试**：模拟长时间的网络波动，测试系统的长期稳定性

示例：测试客户端连接器在不稳定网络中的重连能力

```go
func TestClientConnectorReconnect(t *testing.T) {
    // 创建场景管理器
    manager := testing.NewSimulatorManager()
    
    // 创建模拟网络连接
    conn := testing.NewMockNetConn()
    
    // 创建客户端连接器，使用模拟连接
    connector := client.NewClientConnector(conn)
    
    // 添加连接到管理器
    manager.AddConnection(conn)
    
    // 启动连接器
    connector.Start()
    
    // 运行间歇性连接场景
    manager.RunScenario("intermittent")
    
    // 等待场景完成
    manager.WaitForCompletion()
    
    // 验证连接器是否成功处理了所有断开/重连事件
    stats := connector.GetStats()
    if stats.ReconnectCount < 3 {
        t.Error("连接器应该至少尝试重连3次")
    }
    
    if !connector.IsConnected() {
        t.Error("场景结束后，连接器应该处于已连接状态")
    }
    
    // 停止连接器
    connector.Stop()
}
```

## 最佳实践

1. **合理设置场景持续时间**：场景持续时间应该足够长，以便观察系统对网络变化的反应，但也不应过长，以免延长测试时间
2. **使用预定义场景**：优先使用预定义场景，以保证测试的一致性和可比性
3. **自定义场景要有针对性**：自定义场景应该针对特定的测试目标，模拟与该目标相关的网络行为
4. **关注边界条件**：重点测试网络条件突然变化的边界情况，如连接突然断开、延迟突然增大等
5. **集成到CI/CD流程**：将网络场景测试集成到CI/CD流程中，自动验证每次代码变更对系统稳定性的影响

## 扩展方向

1. **更丰富的网络条件模拟**：如带宽限制、包乱序、网络抖动等
2. **更复杂的场景组合**：支持场景的串联和并行执行，模拟更复杂的网络环境变化
3. **基于真实网络数据的场景**：收集和分析真实网络环境数据，生成更贴近实际的网络场景
4. **可视化工具**：开发网络场景可视化设计和监控工具，使测试过程更直观

## 结论

本测试框架通过模拟现实世界中的各种网络条件，为代理系统提供了一个可控、可重复的测试环境。通过系统地测试代理系统在不同网络条件下的表现，可以有效提高系统的稳定性和可靠性，确保系统在各种网络环境中都能正常工作。 