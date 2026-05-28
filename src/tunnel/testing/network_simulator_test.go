package testing

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestNetworkSimulatorCreation(t *testing.T) {
	// 测试默认创建
	simulator := NewNetworkSimulator()
	if simulator == nil {
		t.Fatal("创建默认网络模拟器失败")
	}

	if simulator.IsRunning() {
		t.Error("新创建的默认模拟器不应该处于运行状态")
	}

	// 测试使用自定义选项创建
	customOptions := NetworkSimulatorOptions{
		InitialNetworkCondition: PoorNetworkCondition,
		AutoStart:               true,
	}

	simulator = NewNetworkSimulatorWithOptions(customOptions)
	if simulator == nil {
		t.Fatal("创建自定义网络模拟器失败")
	}

	// 由于AutoStart为true，模拟器应该处于运行状态
	if !simulator.IsRunning() {
		t.Error("设置了AutoStart的模拟器应该处于运行状态")
	}

	// 检查初始网络条件是否正确设置
	condition := simulator.GetCurrentNetworkCondition()
	if condition.ReadDelay != PoorNetworkCondition.ReadDelay ||
		condition.WriteDelay != PoorNetworkCondition.WriteDelay ||
		condition.PacketLossRate != PoorNetworkCondition.PacketLossRate ||
		condition.ReadErrorRate != PoorNetworkCondition.ReadErrorRate ||
		condition.WriteErrorRate != PoorNetworkCondition.WriteErrorRate {
		t.Error("模拟器的初始网络条件设置不正确")
	}

	// 测试完成后停止模拟器
	simulator.Stop()
}

func TestNetworkSimulatorStartStop(t *testing.T) {
	simulator := NewNetworkSimulator()

	// 启动模拟器
	simulator.Start()
	if !simulator.IsRunning() {
		t.Error("启动模拟器后应该处于运行状态")
	}

	// 再次启动，不应该有影响
	simulator.Start()
	if !simulator.IsRunning() {
		t.Error("重复启动后模拟器应该仍然处于运行状态")
	}

	// 停止模拟器
	simulator.Stop()
	if simulator.IsRunning() {
		t.Error("停止模拟器后不应该处于运行状态")
	}

	// 再次停止，不应该有影响
	simulator.Stop()
	if simulator.IsRunning() {
		t.Error("重复停止后模拟器不应该处于运行状态")
	}
}

func TestNetworkSimulatorReset(t *testing.T) {
	// 使用较差的网络条件创建模拟器
	customOptions := NetworkSimulatorOptions{
		InitialNetworkCondition: PoorNetworkCondition,
		AutoStart:               true,
	}

	simulator := NewNetworkSimulatorWithOptions(customOptions)

	// 更改当前网络条件
	simulator.SetNetworkCondition(UnstableNetworkCondition)

	// 添加一些事件
	simulator.AddEvent(time.Second, NetworkEvent{
		Type: EventTypeDisconnect,
	})

	// 重置模拟器
	simulator.Reset()

	// 检查重置后的状态
	if simulator.IsRunning() {
		t.Error("重置后模拟器不应该处于运行状态")
	}

	// 检查网络条件是否恢复为初始值
	condition := simulator.GetCurrentNetworkCondition()
	if condition.ReadDelay != PoorNetworkCondition.ReadDelay ||
		condition.WriteDelay != PoorNetworkCondition.WriteDelay ||
		condition.PacketLossRate != PoorNetworkCondition.PacketLossRate ||
		condition.ReadErrorRate != PoorNetworkCondition.ReadErrorRate ||
		condition.WriteErrorRate != PoorNetworkCondition.WriteErrorRate {
		t.Error("重置后模拟器的网络条件应该恢复为初始值")
	}
}

func TestNetworkSimulatorNetworkConditionChange(t *testing.T) {
	simulator := NewNetworkSimulator()

	// 设置网络条件变化的回调
	conditionChangeCount := 0
	var lastOldCondition, lastNewCondition MockNetConnOptions

	simulator.OnNetworkConditionChange = func(oldCondition, newCondition MockNetConnOptions) {
		conditionChangeCount++
		lastOldCondition = oldCondition
		lastNewCondition = newCondition
	}

	// 更改网络条件
	simulator.SetNetworkCondition(PoorNetworkCondition)

	// 检查回调是否被调用
	if conditionChangeCount != 1 {
		t.Errorf("网络条件变化回调应该被调用一次，实际调用次数: %d", conditionChangeCount)
	}

	// 检查回调参数是否正确
	if lastOldCondition.ReadDelay != GoodNetworkCondition.ReadDelay ||
		lastNewCondition.ReadDelay != PoorNetworkCondition.ReadDelay {
		t.Error("网络条件变化回调的参数不正确")
	}
}

func TestNetworkSimulatorAddingConnections(t *testing.T) {
	simulator := NewNetworkSimulator()

	// 创建一个网络连接
	conn := NewMockNetConn()

	// 将连接添加到模拟器
	simulator.AddConnection(conn)

	// 设置网络条件
	simulator.SetNetworkCondition(PoorNetworkCondition)

	// 检查连接的网络条件是否被更新
	opts := conn.GetOptions()
	if opts.ReadDelay != PoorNetworkCondition.ReadDelay ||
		opts.WriteDelay != PoorNetworkCondition.WriteDelay ||
		opts.PacketLossRate != PoorNetworkCondition.PacketLossRate ||
		opts.ReadErrorRate != PoorNetworkCondition.ReadErrorRate ||
		opts.WriteErrorRate != PoorNetworkCondition.WriteErrorRate {
		t.Error("添加到模拟器的连接网络条件没有被正确更新")
	}
}

func TestNetworkSimulatorCustomEvents(t *testing.T) {
	simulator := NewNetworkSimulator()
	simulator.Start()

	// 添加一个自定义事件
	var customEventCalled atomic.Bool
	simulator.AddEvent(50*time.Millisecond, NetworkEvent{
		Type: EventTypeCustom,
		Callback: func() {
			customEventCalled.Store(true)
		},
	})

	// 等待事件被处理
	time.Sleep(100 * time.Millisecond)

	// 检查事件是否被调用
	if !customEventCalled.Load() {
		t.Error("自定义事件回调没有被调用")
	}

	simulator.Stop()
}

func TestNetworkSimulatorDisconnectReconnect(t *testing.T) {
	simulator := NewNetworkSimulator()
	simulator.Start()

	// 创建一个网络连接
	conn := NewMockNetConn()

	// 将连接添加到模拟器
	simulator.AddConnection(conn)

	// 添加断开连接事件
	simulator.AddEvent(50*time.Millisecond, NetworkEvent{
		Type: EventTypeDisconnect,
	})

	// 添加重新连接事件
	simulator.AddEvent(100*time.Millisecond, NetworkEvent{
		Type: EventTypeReconnect,
	})

	// 等待断开连接事件被处理
	time.Sleep(75 * time.Millisecond)

	// 检查连接是否被断开
	if !conn.IsClosed() {
		t.Error("断开连接事件没有正确断开连接")
	}

	// 等待重新连接事件被处理
	time.Sleep(50 * time.Millisecond)

	// 检查连接是否被重新连接
	if conn.IsClosed() {
		t.Error("重新连接事件没有正确重新建立连接")
	}

	simulator.Stop()
}

func TestNetworkSimulatorLatencyEvents(t *testing.T) {
	simulator := NewNetworkSimulator()
	simulator.Start()

	// 创建一个网络连接
	conn := NewMockNetConn()

	// 将连接添加到模拟器
	simulator.AddConnection(conn)

	// 记录初始延迟
	initialReadDelay := conn.GetOptions().ReadDelay

	// 添加延迟增加事件
	simulator.AddEvent(50*time.Millisecond, NetworkEvent{
		Type: EventTypeLatencyIncrease,
		Params: map[string]interface{}{
			"factor": 2.0,
		},
	})

	// 等待延迟增加事件被处理
	time.Sleep(75 * time.Millisecond)

	// 检查延迟是否增加
	if opts := conn.GetOptions(); opts.ReadDelay != initialReadDelay*2 {
		t.Errorf("延迟增加事件没有正确增加延迟, 期望: %v, 实际: %v", initialReadDelay*2, opts.ReadDelay)
	}

	// 添加延迟减少事件
	simulator.AddEvent(100*time.Millisecond, NetworkEvent{
		Type: EventTypeLatencyDecrease,
		Params: map[string]interface{}{
			"factor": 0.5,
		},
	})

	// 等待延迟减少事件被处理
	time.Sleep(50 * time.Millisecond)

	// 检查延迟是否减少
	if opts := conn.GetOptions(); opts.ReadDelay != initialReadDelay {
		t.Errorf("延迟减少事件没有正确减少延迟, 期望: %v, 实际: %v", initialReadDelay, opts.ReadDelay)
	}

	simulator.Stop()
}

func TestNetworkSimulatorPacketLossEvents(t *testing.T) {
	simulator := NewNetworkSimulator()
	simulator.Start()

	// 创建一个网络连接
	conn := NewMockNetConn()

	// 将连接添加到模拟器
	simulator.AddConnection(conn)

	// 记录初始丢包率
	initialPacketLossRate := conn.GetOptions().PacketLossRate

	// 添加丢包率增加事件
	simulator.AddEvent(50*time.Millisecond, NetworkEvent{
		Type: EventTypePacketLossIncrease,
		Params: map[string]interface{}{
			"increase": float32(0.1),
		},
	})

	// 等待丢包率增加事件被处理
	time.Sleep(75 * time.Millisecond)

	// 检查丢包率是否增加
	if opts := conn.GetOptions(); opts.PacketLossRate != initialPacketLossRate+0.1 {
		t.Errorf("丢包率增加事件没有正确增加丢包率, 期望: %v, 实际: %v", initialPacketLossRate+0.1, opts.PacketLossRate)
	}

	// 添加丢包率减少事件
	simulator.AddEvent(100*time.Millisecond, NetworkEvent{
		Type: EventTypePacketLossDecrease,
		Params: map[string]interface{}{
			"decrease": float32(0.1),
		},
	})

	// 等待丢包率减少事件被处理
	time.Sleep(50 * time.Millisecond)

	// 检查丢包率是否减少
	if opts := conn.GetOptions(); opts.PacketLossRate != initialPacketLossRate {
		t.Errorf("丢包率减少事件没有正确减少丢包率, 期望: %v, 实际: %v", initialPacketLossRate, opts.PacketLossRate)
	}

	simulator.Stop()
}

func TestNetworkSimulatorErrorRateEvents(t *testing.T) {
	simulator := NewNetworkSimulator()
	simulator.Start()

	// 创建一个网络连接
	conn := NewMockNetConn()

	// 将连接添加到模拟器
	simulator.AddConnection(conn)

	// 记录初始错误率
	initialOpts := conn.GetOptions()
	initialReadErrorRate := initialOpts.ReadErrorRate
	initialWriteErrorRate := initialOpts.WriteErrorRate

	// 添加错误率增加事件
	simulator.AddEvent(50*time.Millisecond, NetworkEvent{
		Type: EventTypeErrorRateIncrease,
		Params: map[string]interface{}{
			"increase": float32(0.05),
		},
	})

	// 等待错误率增加事件被处理
	time.Sleep(75 * time.Millisecond)

	// 检查错误率是否增加
	if opts := conn.GetOptions(); opts.ReadErrorRate != initialReadErrorRate+0.05 ||
		opts.WriteErrorRate != initialWriteErrorRate+0.05 {
		t.Errorf("错误率增加事件没有正确增加错误率, 期望: %v/%v, 实际: %v/%v",
			initialReadErrorRate+0.05, initialWriteErrorRate+0.05,
			opts.ReadErrorRate, opts.WriteErrorRate)
	}

	// 添加错误率减少事件
	simulator.AddEvent(100*time.Millisecond, NetworkEvent{
		Type: EventTypeErrorRateDecrease,
		Params: map[string]interface{}{
			"decrease": float32(0.05),
		},
	})

	// 等待错误率减少事件被处理
	time.Sleep(50 * time.Millisecond)

	// 检查错误率是否减少
	if opts := conn.GetOptions(); opts.ReadErrorRate != initialReadErrorRate ||
		opts.WriteErrorRate != initialWriteErrorRate {
		t.Errorf("错误率减少事件没有正确减少错误率, 期望: %v/%v, 实际: %v/%v",
			initialReadErrorRate, initialWriteErrorRate,
			opts.ReadErrorRate, opts.WriteErrorRate)
	}

	simulator.Stop()
}

func TestNetworkSimulatorCustomEventHandler(t *testing.T) {
	simulator := NewNetworkSimulator()
	simulator.Start()

	// 注册自定义事件处理函数
	customEventType := NetworkEventType("custom_test_event")
	var customEventHandled atomic.Bool

	simulator.RegisterEventHandler(customEventType, func(event NetworkEvent) {
		customEventHandled.Store(true)
		// 可以处理event.Params中的参数
	})

	// 添加自定义类型事件
	simulator.AddEvent(50*time.Millisecond, NetworkEvent{
		Type: customEventType,
	})

	// 等待事件被处理
	time.Sleep(100 * time.Millisecond)

	// 检查事件是否被处理
	if !customEventHandled.Load() {
		t.Error("自定义事件处理函数没有被调用")
	}

	simulator.Stop()
}
