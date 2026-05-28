package testing

import (
	"fmt"
	"sync"
	"time"
)

// SimulationScenario 表示一个网络模拟场景
type SimulationScenario struct {
	// 场景名称
	Name string
	// 场景描述
	Description string
	// 场景中包含的事件
	Events []NetworkEvent
	// 场景持续时间
	Duration time.Duration
}

// DefaultScenarios 预定义的网络场景
var DefaultScenarios = map[string]SimulationScenario{
	"stable": {
		Name:        "stable",
		Description: "稳定网络环境，低延迟，无丢包",
		Events: []NetworkEvent{
			{
				Type: EventTypeCustom,
				Time: 0,
				Callback: func() {
					fmt.Println("开始模拟稳定网络环境...")
				},
			},
		},
		Duration: 5 * time.Minute,
	},
	"unstable": {
		Name:        "unstable",
		Description: "不稳定网络环境，延迟波动，偶尔丢包",
		Events: []NetworkEvent{
			{
				Type: EventTypeCustom,
				Time: 0,
				Callback: func() {
					fmt.Println("开始模拟不稳定网络环境...")
				},
			},
			{
				Type:   EventTypeLatencyIncrease,
				Time:   30 * time.Second,
				Params: map[string]interface{}{"factor": 2.0},
			},
			{
				Type:   EventTypeLatencyDecrease,
				Time:   60 * time.Second,
				Params: map[string]interface{}{"factor": 0.5},
			},
			{
				Type:   EventTypePacketLossIncrease,
				Time:   90 * time.Second,
				Params: map[string]interface{}{"increase": float32(0.05)},
			},
			{
				Type:   EventTypePacketLossDecrease,
				Time:   120 * time.Second,
				Params: map[string]interface{}{"decrease": float32(0.05)},
			},
		},
		Duration: 5 * time.Minute,
	},
	"intermittent": {
		Name:        "intermittent",
		Description: "间歇性连接，周期性断开和重连",
		Events: []NetworkEvent{
			{
				Type: EventTypeCustom,
				Time: 0,
				Callback: func() {
					fmt.Println("开始模拟间歇性连接网络环境...")
				},
			},
			{
				Type: EventTypeDisconnect,
				Time: 30 * time.Second,
			},
			{
				Type: EventTypeReconnect,
				Time: 40 * time.Second,
			},
			{
				Type: EventTypeDisconnect,
				Time: 90 * time.Second,
			},
			{
				Type: EventTypeReconnect,
				Time: 100 * time.Second,
			},
			{
				Type: EventTypeDisconnect,
				Time: 150 * time.Second,
			},
			{
				Type: EventTypeReconnect,
				Time: 160 * time.Second,
			},
		},
		Duration: 5 * time.Minute,
	},
	"degrading": {
		Name:        "degrading",
		Description: "逐渐恶化的网络环境",
		Events: []NetworkEvent{
			{
				Type: EventTypeCustom,
				Time: 0,
				Callback: func() {
					fmt.Println("开始模拟逐渐恶化的网络环境...")
				},
			},
			{
				Type:   EventTypeLatencyIncrease,
				Time:   30 * time.Second,
				Params: map[string]interface{}{"factor": 1.5},
			},
			{
				Type:   EventTypeLatencyIncrease,
				Time:   60 * time.Second,
				Params: map[string]interface{}{"factor": 1.5},
			},
			{
				Type:   EventTypePacketLossIncrease,
				Time:   90 * time.Second,
				Params: map[string]interface{}{"increase": float32(0.03)},
			},
			{
				Type:   EventTypePacketLossIncrease,
				Time:   120 * time.Second,
				Params: map[string]interface{}{"increase": float32(0.03)},
			},
			{
				Type:   EventTypeErrorRateIncrease,
				Time:   150 * time.Second,
				Params: map[string]interface{}{"increase": float32(0.02)},
			},
			{
				Type:   EventTypeErrorRateIncrease,
				Time:   180 * time.Second,
				Params: map[string]interface{}{"increase": float32(0.03)},
			},
			{
				Type: EventTypeDisconnect,
				Time: 210 * time.Second,
			},
		},
		Duration: 5 * time.Minute,
	},
	"improving": {
		Name:        "improving",
		Description: "逐渐改善的网络环境",
		Events: []NetworkEvent{
			{
				Type: EventTypeCustom,
				Time: 0,
				Callback: func() {
					fmt.Println("开始模拟逐渐改善的网络环境...")
				},
			},
			{
				Type:   EventTypeLatencyIncrease,
				Time:   0,
				Params: map[string]interface{}{"factor": 3.0},
			},
			{
				Type:   EventTypePacketLossIncrease,
				Time:   0,
				Params: map[string]interface{}{"increase": float32(0.1)},
			},
			{
				Type:   EventTypeErrorRateIncrease,
				Time:   0,
				Params: map[string]interface{}{"increase": float32(0.05)},
			},
			{
				Type:   EventTypeLatencyDecrease,
				Time:   60 * time.Second,
				Params: map[string]interface{}{"factor": 0.7},
			},
			{
				Type:   EventTypePacketLossDecrease,
				Time:   90 * time.Second,
				Params: map[string]interface{}{"decrease": float32(0.05)},
			},
			{
				Type:   EventTypeLatencyDecrease,
				Time:   120 * time.Second,
				Params: map[string]interface{}{"factor": 0.7},
			},
			{
				Type:   EventTypeErrorRateDecrease,
				Time:   150 * time.Second,
				Params: map[string]interface{}{"decrease": float32(0.03)},
			},
			{
				Type:   EventTypePacketLossDecrease,
				Time:   180 * time.Second,
				Params: map[string]interface{}{"decrease": float32(0.05)},
			},
		},
		Duration: 5 * time.Minute,
	},
}

// SimulatorManager 网络模拟器管理器
type SimulatorManager struct {
	mu sync.Mutex
	// 当前的网络模拟器
	simulator *NetworkSimulator
	// 所有可用的模拟场景
	scenarios map[string]SimulationScenario
	// 当前场景
	currentScenario string
	// 是否正在运行
	running bool
	// 场景完成通知通道
	done chan struct{}
	// 是否应该自动重置网络条件
	shouldAutoReset bool
}

// NewSimulatorManager 创建新的网络模拟器管理器
func NewSimulatorManager() *SimulatorManager {
	return &SimulatorManager{
		simulator:       NewNetworkSimulator(),
		scenarios:       DefaultScenarios,
		running:         false,
		done:            make(chan struct{}),
		shouldAutoReset: true,
	}
}

// AddScenario 添加新的模拟场景
func (sm *SimulatorManager) AddScenario(scenario SimulationScenario) error {
	if _, exists := sm.scenarios[scenario.Name]; exists {
		return fmt.Errorf("场景 '%s' 已存在", scenario.Name)
	}

	sm.scenarios[scenario.Name] = scenario
	return nil
}

// RunScenario 运行指定的模拟场景
func (sm *SimulatorManager) RunScenario(scenarioName string) error {
	sm.mu.Lock()
	if sm.running {
		sm.mu.Unlock()
		return fmt.Errorf("已有场景正在运行，请先停止")
	}

	scenario, exists := sm.scenarios[scenarioName]
	if !exists {
		sm.mu.Unlock()
		return fmt.Errorf("场景 '%s' 不存在", scenarioName)
	}

	// 重置模拟器
	sm.simulator.Reset()

	// 添加场景中的所有事件
	for _, event := range scenario.Events {
		sm.simulator.AddEvent(event.Time, event)
	}

	// 启动模拟器
	sm.currentScenario = scenarioName
	sm.running = true
	sm.simulator.Start()
	sm.mu.Unlock()

	// 启动一个协程等待场景结束
	go func() {
		timer := time.NewTimer(scenario.Duration)
		<-timer.C

		sm.mu.Lock()
		if sm.shouldAutoReset {
			// 场景完成后，重置网络条件
			sm.simulator.SetNetworkCondition(GoodNetworkCondition)
		}

		sm.running = false
		close(sm.done)
		sm.done = make(chan struct{})
		sm.mu.Unlock()
	}()

	return nil
}

// Stop 停止当前正在运行的场景
func (sm *SimulatorManager) Stop() {
	sm.mu.Lock()
	if !sm.running {
		sm.mu.Unlock()
		return
	}

	sm.simulator.Stop()
	sm.running = false
	close(sm.done)
	sm.done = make(chan struct{})
	sm.mu.Unlock()
}

// IsRunning 检查是否有场景正在运行
func (sm *SimulatorManager) IsRunning() bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.running
}

// GetCurrentScenario 获取当前正在运行的场景名称
func (sm *SimulatorManager) GetCurrentScenario() string {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if !sm.running {
		return ""
	}
	return sm.currentScenario
}

// WaitForCompletion 等待当前场景完成
func (sm *SimulatorManager) WaitForCompletion() {
	sm.mu.Lock()
	if !sm.running {
		sm.mu.Unlock()
		return
	}
	done := sm.done
	sm.mu.Unlock()

	<-done
}

// SetAutoReset 设置是否应该在场景完成后自动重置网络条件
func (sm *SimulatorManager) SetAutoReset(autoReset bool) {
	sm.shouldAutoReset = autoReset
}

// GetSimulator 获取当前使用的网络模拟器
func (sm *SimulatorManager) GetSimulator() *NetworkSimulator {
	return sm.simulator
}

// AddConnection 将连接添加到模拟器中
func (sm *SimulatorManager) AddConnection(conn *MockNetConn) {
	sm.simulator.AddConnection(conn)
}

// ListScenarios 列出所有可用的场景
func (sm *SimulatorManager) ListScenarios() []string {
	var names []string
	for name := range sm.scenarios {
		names = append(names, name)
	}
	return names
}

// GetScenarioDescription 获取场景的描述
func (sm *SimulatorManager) GetScenarioDescription(scenarioName string) (string, error) {
	scenario, exists := sm.scenarios[scenarioName]
	if !exists {
		return "", fmt.Errorf("场景 '%s' 不存在", scenarioName)
	}

	return scenario.Description, nil
}
