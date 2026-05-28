package testing

import (
	"sort"
	"sync"
	"time"
)

// NetworkEventType 网络事件类型
type NetworkEventType string

const (
	// EventTypeDisconnect 断开连接事件
	EventTypeDisconnect NetworkEventType = "disconnect"
	// EventTypeReconnect 重新连接事件
	EventTypeReconnect NetworkEventType = "reconnect"
	// EventTypeLatencyIncrease 延迟增加事件
	EventTypeLatencyIncrease NetworkEventType = "latency_increase"
	// EventTypeLatencyDecrease 延迟减少事件
	EventTypeLatencyDecrease NetworkEventType = "latency_decrease"
	// EventTypePacketLossIncrease 丢包率增加事件
	EventTypePacketLossIncrease NetworkEventType = "packet_loss_increase"
	// EventTypePacketLossDecrease 丢包率减少事件
	EventTypePacketLossDecrease NetworkEventType = "packet_loss_decrease"
	// EventTypeErrorRateIncrease 错误率增加事件
	EventTypeErrorRateIncrease NetworkEventType = "error_rate_increase"
	// EventTypeErrorRateDecrease 错误率减少事件
	EventTypeErrorRateDecrease NetworkEventType = "error_rate_decrease"
	// EventTypeCustom 自定义事件，可以执行任意回调
	EventTypeCustom NetworkEventType = "custom"
)

// NetworkEvent 网络事件
type NetworkEvent struct {
	// 事件类型
	Type NetworkEventType
	// 事件执行时间（相对于模拟器启动时间）
	Time time.Duration
	// 事件参数
	Params map[string]interface{}
	// 自定义回调函数，用于Custom类型事件
	Callback func()
}

// NetworkSimulatorOptions 网络模拟器配置选项
type NetworkSimulatorOptions struct {
	// 初始网络状态
	InitialNetworkCondition MockNetConnOptions
	// 是否自动启动
	AutoStart bool
}

// DefaultNetworkSimulatorOptions 默认的网络模拟器配置
var DefaultNetworkSimulatorOptions = NetworkSimulatorOptions{
	InitialNetworkCondition: GoodNetworkCondition,
	AutoStart:               false,
}

// NetworkSimulator 网络条件模拟器
type NetworkSimulator struct {
	options     NetworkSimulatorOptions
	events      []NetworkEvent
	currentTime time.Duration
	startTime   time.Time
	running     bool

	// 当前网络状态
	currentNetworkCondition MockNetConnOptions

	// 关联的网络连接
	connections []*MockNetConn

	// 事件处理函数
	eventHandlers map[NetworkEventType]func(event NetworkEvent)

	// 同步
	mutex            sync.Mutex
	simulationDoneCh chan struct{}
	stopCh           chan struct{}

	// 用于测试的回调函数
	OnNetworkConditionChange func(oldCondition, newCondition MockNetConnOptions)
}

// NewNetworkSimulator 创建一个新的网络模拟器
func NewNetworkSimulator() *NetworkSimulator {
	return NewNetworkSimulatorWithOptions(DefaultNetworkSimulatorOptions)
}

// NewNetworkSimulatorWithOptions 使用自定义选项创建网络模拟器
func NewNetworkSimulatorWithOptions(opts NetworkSimulatorOptions) *NetworkSimulator {
	simulator := &NetworkSimulator{
		options:                 opts,
		events:                  make([]NetworkEvent, 0),
		currentNetworkCondition: opts.InitialNetworkCondition,
		connections:             make([]*MockNetConn, 0),
		eventHandlers:           make(map[NetworkEventType]func(event NetworkEvent)),
		simulationDoneCh:        make(chan struct{}),
		stopCh:                  make(chan struct{}),
	}

	// 注册默认事件处理函数
	simulator.registerDefaultEventHandlers()

	// 如果配置为自动启动，则立即启动
	if opts.AutoStart {
		simulator.Start()
	}

	return simulator
}

// 注册默认事件处理函数
func (ns *NetworkSimulator) registerDefaultEventHandlers() {
	// 断开连接事件处理
	ns.eventHandlers[EventTypeDisconnect] = func(event NetworkEvent) {
		ns.mutex.Lock()
		defer ns.mutex.Unlock()

		for _, conn := range ns.connections {
			// 关闭连接但不调用Close方法，只是模拟连接断开
			conn.closed = true
		}
	}

	// 重新连接事件处理
	ns.eventHandlers[EventTypeReconnect] = func(event NetworkEvent) {
		ns.mutex.Lock()
		defer ns.mutex.Unlock()

		for _, conn := range ns.connections {
			// 重新连接
			conn.closed = false
		}
	}

	// 延迟增加事件处理
	ns.eventHandlers[EventTypeLatencyIncrease] = func(event NetworkEvent) {
		ns.mutex.Lock()
		defer ns.mutex.Unlock()

		// 获取参数
		factor, ok := event.Params["factor"].(float64)
		if !ok {
			factor = 2.0 // 默认增加一倍
		}

		// 更新网络状态
		oldCondition := ns.currentNetworkCondition
		ns.currentNetworkCondition.ReadDelay = time.Duration(float64(ns.currentNetworkCondition.ReadDelay) * factor)
		ns.currentNetworkCondition.WriteDelay = time.Duration(float64(ns.currentNetworkCondition.WriteDelay) * factor)

		// 更新所有连接
		for _, conn := range ns.connections {
			conn.options.ReadDelay = ns.currentNetworkCondition.ReadDelay
			conn.options.WriteDelay = ns.currentNetworkCondition.WriteDelay
		}

		// 触发回调
		if ns.OnNetworkConditionChange != nil {
			ns.OnNetworkConditionChange(oldCondition, ns.currentNetworkCondition)
		}
	}

	// 延迟减少事件处理
	ns.eventHandlers[EventTypeLatencyDecrease] = func(event NetworkEvent) {
		ns.mutex.Lock()
		defer ns.mutex.Unlock()

		// 获取参数
		factor, ok := event.Params["factor"].(float64)
		if !ok {
			factor = 0.5 // 默认减少一半
		}

		// 更新网络状态
		oldCondition := ns.currentNetworkCondition
		ns.currentNetworkCondition.ReadDelay = time.Duration(float64(ns.currentNetworkCondition.ReadDelay) * factor)
		ns.currentNetworkCondition.WriteDelay = time.Duration(float64(ns.currentNetworkCondition.WriteDelay) * factor)

		// 更新所有连接
		for _, conn := range ns.connections {
			conn.options.ReadDelay = ns.currentNetworkCondition.ReadDelay
			conn.options.WriteDelay = ns.currentNetworkCondition.WriteDelay
		}

		// 触发回调
		if ns.OnNetworkConditionChange != nil {
			ns.OnNetworkConditionChange(oldCondition, ns.currentNetworkCondition)
		}
	}

	// 丢包率增加事件处理
	ns.eventHandlers[EventTypePacketLossIncrease] = func(event NetworkEvent) {
		ns.mutex.Lock()
		defer ns.mutex.Unlock()

		// 获取参数
		increase, ok := event.Params["increase"].(float32)
		if !ok {
			increase = 0.1 // 默认增加0.1
		}

		// 更新网络状态
		oldCondition := ns.currentNetworkCondition
		ns.currentNetworkCondition.PacketLossRate += increase
		if ns.currentNetworkCondition.PacketLossRate > 1.0 {
			ns.currentNetworkCondition.PacketLossRate = 1.0
		}

		// 更新所有连接
		for _, conn := range ns.connections {
			conn.options.PacketLossRate = ns.currentNetworkCondition.PacketLossRate
		}

		// 触发回调
		if ns.OnNetworkConditionChange != nil {
			ns.OnNetworkConditionChange(oldCondition, ns.currentNetworkCondition)
		}
	}

	// 丢包率减少事件处理
	ns.eventHandlers[EventTypePacketLossDecrease] = func(event NetworkEvent) {
		ns.mutex.Lock()
		defer ns.mutex.Unlock()

		// 获取参数
		decrease, ok := event.Params["decrease"].(float32)
		if !ok {
			decrease = 0.1 // 默认减少0.1
		}

		// 更新网络状态
		oldCondition := ns.currentNetworkCondition
		ns.currentNetworkCondition.PacketLossRate -= decrease
		if ns.currentNetworkCondition.PacketLossRate < 0.0 {
			ns.currentNetworkCondition.PacketLossRate = 0.0
		}

		// 更新所有连接
		for _, conn := range ns.connections {
			conn.options.PacketLossRate = ns.currentNetworkCondition.PacketLossRate
		}

		// 触发回调
		if ns.OnNetworkConditionChange != nil {
			ns.OnNetworkConditionChange(oldCondition, ns.currentNetworkCondition)
		}
	}

	// 错误率增加事件处理
	ns.eventHandlers[EventTypeErrorRateIncrease] = func(event NetworkEvent) {
		ns.mutex.Lock()
		defer ns.mutex.Unlock()

		// 获取参数
		increase, ok := event.Params["increase"].(float32)
		if !ok {
			increase = 0.05 // 默认增加0.05
		}

		// 更新网络状态
		oldCondition := ns.currentNetworkCondition
		ns.currentNetworkCondition.ReadErrorRate += increase
		ns.currentNetworkCondition.WriteErrorRate += increase
		if ns.currentNetworkCondition.ReadErrorRate > 1.0 {
			ns.currentNetworkCondition.ReadErrorRate = 1.0
		}
		if ns.currentNetworkCondition.WriteErrorRate > 1.0 {
			ns.currentNetworkCondition.WriteErrorRate = 1.0
		}

		// 更新所有连接
		for _, conn := range ns.connections {
			conn.options.ReadErrorRate = ns.currentNetworkCondition.ReadErrorRate
			conn.options.WriteErrorRate = ns.currentNetworkCondition.WriteErrorRate
		}

		// 触发回调
		if ns.OnNetworkConditionChange != nil {
			ns.OnNetworkConditionChange(oldCondition, ns.currentNetworkCondition)
		}
	}

	// 错误率减少事件处理
	ns.eventHandlers[EventTypeErrorRateDecrease] = func(event NetworkEvent) {
		ns.mutex.Lock()
		defer ns.mutex.Unlock()

		// 获取参数
		decrease, ok := event.Params["decrease"].(float32)
		if !ok {
			decrease = 0.05 // 默认减少0.05
		}

		// 更新网络状态
		oldCondition := ns.currentNetworkCondition
		ns.currentNetworkCondition.ReadErrorRate -= decrease
		ns.currentNetworkCondition.WriteErrorRate -= decrease
		if ns.currentNetworkCondition.ReadErrorRate < 0.0 {
			ns.currentNetworkCondition.ReadErrorRate = 0.0
		}
		if ns.currentNetworkCondition.WriteErrorRate < 0.0 {
			ns.currentNetworkCondition.WriteErrorRate = 0.0
		}

		// 更新所有连接
		for _, conn := range ns.connections {
			conn.options.ReadErrorRate = ns.currentNetworkCondition.ReadErrorRate
			conn.options.WriteErrorRate = ns.currentNetworkCondition.WriteErrorRate
		}

		// 触发回调
		if ns.OnNetworkConditionChange != nil {
			ns.OnNetworkConditionChange(oldCondition, ns.currentNetworkCondition)
		}
	}

	// 自定义事件处理
	ns.eventHandlers[EventTypeCustom] = func(event NetworkEvent) {
		if event.Callback != nil {
			event.Callback()
		}
	}
}

// RegisterEventHandler 注册自定义事件处理函数
func (ns *NetworkSimulator) RegisterEventHandler(eventType NetworkEventType, handler func(event NetworkEvent)) {
	ns.mutex.Lock()
	defer ns.mutex.Unlock()

	ns.eventHandlers[eventType] = handler
}

// AddEvent 添加网络事件
func (ns *NetworkSimulator) AddEvent(time time.Duration, event NetworkEvent) {
	ns.mutex.Lock()
	defer ns.mutex.Unlock()

	event.Time = time
	ns.events = append(ns.events, event)

	// 按时间排序事件
	sort.Slice(ns.events, func(i, j int) bool {
		return ns.events[i].Time < ns.events[j].Time
	})
}

// AddConnection 添加需要模拟的网络连接
func (ns *NetworkSimulator) AddConnection(conn *MockNetConn) {
	ns.mutex.Lock()
	defer ns.mutex.Unlock()

	// 设置连接的网络条件为当前模拟器的网络条件
	conn.options.ReadDelay = ns.currentNetworkCondition.ReadDelay
	conn.options.WriteDelay = ns.currentNetworkCondition.WriteDelay
	conn.options.PacketLossRate = ns.currentNetworkCondition.PacketLossRate
	conn.options.ReadErrorRate = ns.currentNetworkCondition.ReadErrorRate
	conn.options.WriteErrorRate = ns.currentNetworkCondition.WriteErrorRate

	ns.connections = append(ns.connections, conn)
}

// Start 启动网络模拟器
func (ns *NetworkSimulator) Start() {
	ns.mutex.Lock()
	if ns.running {
		ns.mutex.Unlock()
		return
	}

	ns.running = true
	ns.startTime = time.Now()
	ns.mutex.Unlock()

	go ns.runSimulation()
}

// Stop 停止网络模拟器
func (ns *NetworkSimulator) Stop() {
	ns.mutex.Lock()
	if !ns.running {
		ns.mutex.Unlock()
		return
	}

	ns.running = false
	close(ns.stopCh)
	ns.mutex.Unlock()

	// 等待模拟完成
	<-ns.simulationDoneCh
}

// Reset 重置网络模拟器
func (ns *NetworkSimulator) Reset() {
	ns.Stop()

	ns.mutex.Lock()
	defer ns.mutex.Unlock()

	ns.events = make([]NetworkEvent, 0)
	ns.currentTime = 0
	ns.currentNetworkCondition = ns.options.InitialNetworkCondition

	// 重置所有连接的网络条件
	for _, conn := range ns.connections {
		conn.options.ReadDelay = ns.currentNetworkCondition.ReadDelay
		conn.options.WriteDelay = ns.currentNetworkCondition.WriteDelay
		conn.options.PacketLossRate = ns.currentNetworkCondition.PacketLossRate
		conn.options.ReadErrorRate = ns.currentNetworkCondition.ReadErrorRate
		conn.options.WriteErrorRate = ns.currentNetworkCondition.WriteErrorRate
		conn.closed = false
	}

	ns.stopCh = make(chan struct{})
	ns.simulationDoneCh = make(chan struct{})
}

// IsRunning 检查模拟器是否正在运行
func (ns *NetworkSimulator) IsRunning() bool {
	ns.mutex.Lock()
	defer ns.mutex.Unlock()

	return ns.running
}

// GetCurrentNetworkCondition 获取当前网络条件
func (ns *NetworkSimulator) GetCurrentNetworkCondition() MockNetConnOptions {
	ns.mutex.Lock()
	defer ns.mutex.Unlock()

	return ns.currentNetworkCondition
}

// SetNetworkCondition 设置当前网络条件
func (ns *NetworkSimulator) SetNetworkCondition(condition MockNetConnOptions) {
	ns.mutex.Lock()
	defer ns.mutex.Unlock()

	oldCondition := ns.currentNetworkCondition
	ns.currentNetworkCondition = condition

	// 更新所有连接的网络条件
	for _, conn := range ns.connections {
		conn.options.ReadDelay = condition.ReadDelay
		conn.options.WriteDelay = condition.WriteDelay
		conn.options.PacketLossRate = condition.PacketLossRate
		conn.options.ReadErrorRate = condition.ReadErrorRate
		conn.options.WriteErrorRate = condition.WriteErrorRate
	}

	// 触发回调
	if ns.OnNetworkConditionChange != nil {
		ns.OnNetworkConditionChange(oldCondition, condition)
	}
}

// 运行模拟
func (ns *NetworkSimulator) runSimulation() {
	defer close(ns.simulationDoneCh)

	ticker := time.NewTicker(10 * time.Millisecond) // 10ms粒度
	defer ticker.Stop()

	for {
		select {
		case <-ns.stopCh:
			return
		case <-ticker.C:
			ns.processEvents()
		}
	}
}

// 处理事件
func (ns *NetworkSimulator) processEvents() {
	ns.mutex.Lock()

	// 计算当前时间
	ns.currentTime = time.Since(ns.startTime)

	// 找出所有应该执行的事件
	var eventsToProcess []NetworkEvent
	var remainingEvents []NetworkEvent

	for _, event := range ns.events {
		if event.Time <= ns.currentTime {
			eventsToProcess = append(eventsToProcess, event)
		} else {
			remainingEvents = append(remainingEvents, event)
		}
	}

	// 更新事件列表
	ns.events = remainingEvents

	ns.mutex.Unlock()

	// 执行事件
	for _, event := range eventsToProcess {
		handler, exists := ns.eventHandlers[event.Type]
		if exists {
			handler(event)
		}
	}
}
