package testing

import (
	"testing"
	"time"
)

func TestSimulatorManagerCreation(t *testing.T) {
	manager := NewSimulatorManager()
	if manager == nil {
		t.Fatal("创建模拟器管理器失败")
	}

	// 检查默认状态
	if manager.IsRunning() {
		t.Error("新创建的管理器不应该处于运行状态")
	}

	// 检查默认场景
	scenarios := manager.ListScenarios()
	if len(scenarios) == 0 {
		t.Error("管理器应该包含预定义的场景")
	}

	// 验证一些关键场景是否存在
	expectedScenarios := []string{"stable", "unstable", "intermittent"}
	for _, name := range expectedScenarios {
		found := false
		for _, scenarioName := range scenarios {
			if scenarioName == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("预期的场景 '%s' 不存在", name)
		}
	}
}

func TestSimulatorManagerScenarioDescription(t *testing.T) {
	manager := NewSimulatorManager()

	// 测试获取有效场景的描述
	desc, err := manager.GetScenarioDescription("stable")
	if err != nil {
		t.Errorf("获取有效场景描述时出错: %v", err)
	}
	if desc == "" {
		t.Error("有效场景的描述不应该为空")
	}

	// 测试获取无效场景的描述
	_, err = manager.GetScenarioDescription("non_existent")
	if err == nil {
		t.Error("获取不存在的场景描述时应该返回错误")
	}
}

func TestSimulatorManagerAddScenario(t *testing.T) {
	manager := NewSimulatorManager()

	// 添加新场景
	newScenario := SimulationScenario{
		Name:        "test_scenario",
		Description: "用于测试的场景",
		Events: []NetworkEvent{
			{
				Type: EventTypeDisconnect,
				Time: 10 * time.Second,
			},
		},
		Duration: 30 * time.Second,
	}

	err := manager.AddScenario(newScenario)
	if err != nil {
		t.Errorf("添加新场景时出错: %v", err)
	}

	// 检查场景是否已添加
	scenarios := manager.ListScenarios()
	found := false
	for _, name := range scenarios {
		if name == "test_scenario" {
			found = true
			break
		}
	}
	if !found {
		t.Error("新添加的场景未出现在场景列表中")
	}

	// 尝试再次添加同名场景，应该失败
	err = manager.AddScenario(newScenario)
	if err == nil {
		t.Error("添加重复场景时应该返回错误")
	}
}

func TestSimulatorManagerRunStop(t *testing.T) {
	manager := NewSimulatorManager()

	// 运行不存在的场景，应该返回错误
	err := manager.RunScenario("non_existent")
	if err == nil {
		t.Error("运行不存在的场景时应该返回错误")
	}

	// 运行有效场景
	err = manager.RunScenario("stable")
	if err != nil {
		t.Errorf("运行有效场景时出错: %v", err)
	}

	// 检查状态
	if !manager.IsRunning() {
		t.Error("运行场景后，管理器应该处于运行状态")
	}

	// 获取当前场景
	currentScenario := manager.GetCurrentScenario()
	if currentScenario != "stable" {
		t.Errorf("当前场景应该是 'stable'，实际是: %s", currentScenario)
	}

	// 停止场景
	manager.Stop()

	// 检查状态
	if manager.IsRunning() {
		t.Error("停止场景后，管理器不应该处于运行状态")
	}

	// 再次停止，不应该有任何问题
	manager.Stop()
}

func TestSimulatorManagerConcurrentScenarios(t *testing.T) {
	manager := NewSimulatorManager()

	// 运行第一个场景
	err := manager.RunScenario("stable")
	if err != nil {
		t.Errorf("运行第一个场景时出错: %v", err)
	}

	// 尝试运行第二个场景，应该失败
	err = manager.RunScenario("unstable")
	if err == nil {
		t.Error("当已有场景运行时，运行另一个场景应该返回错误")
	}

	// 停止第一个场景
	manager.Stop()

	// 现在应该可以运行第二个场景
	err = manager.RunScenario("unstable")
	if err != nil {
		t.Errorf("停止前一个场景后运行新场景时出错: %v", err)
	}

	// 检查当前场景
	currentScenario := manager.GetCurrentScenario()
	if currentScenario != "unstable" {
		t.Errorf("当前场景应该是 'unstable'，实际是: %s", currentScenario)
	}

	// 停止场景
	manager.Stop()
}

func TestSimulatorManagerWaitForCompletion(t *testing.T) {
	manager := NewSimulatorManager()

	// 运行一个短时间的场景
	shortScenario := SimulationScenario{
		Name:        "short_test",
		Description: "一个短时的测试场景",
		Events:      []NetworkEvent{},
		Duration:    100 * time.Millisecond,
	}

	err := manager.AddScenario(shortScenario)
	if err != nil {
		t.Errorf("添加短时场景时出错: %v", err)
	}

	err = manager.RunScenario("short_test")
	if err != nil {
		t.Errorf("运行短时场景时出错: %v", err)
	}

	// 等待场景完成
	done := make(chan struct{})
	go func() {
		manager.WaitForCompletion()
		close(done)
	}()

	// 等待一段时间，场景应该会完成
	select {
	case <-done:
		// 场景正确完成
	case <-time.After(200 * time.Millisecond):
		t.Error("等待场景完成超时")
	}

	// 检查场景是否已停止
	if manager.IsRunning() {
		t.Error("场景完成后，管理器不应该处于运行状态")
	}
}

func TestSimulatorManagerAddConnection(t *testing.T) {
	manager := NewSimulatorManager()

	// 创建一个网络连接
	conn := NewMockNetConn()

	// 将连接添加到管理器
	manager.AddConnection(conn)

	// 运行场景
	err := manager.RunScenario("unstable")
	if err != nil {
		t.Errorf("运行场景时出错: %v", err)
	}

	// 短暂等待，让一些事件发生
	time.Sleep(100 * time.Millisecond)

	// 停止场景
	manager.Stop()
}

func TestSimulatorManagerAutoReset(t *testing.T) {
	manager := NewSimulatorManager()

	// 获取模拟器
	simulator := manager.GetSimulator()

	// 创建一个测试连接以便观察网络条件变化
	conn := NewMockNetConn()
	manager.AddConnection(conn)

	// 运行一个会修改网络条件的短时场景
	degradingScenario := SimulationScenario{
		Name:        "quick_degrade",
		Description: "一个快速恶化网络的场景",
		Events: []NetworkEvent{
			{
				Type:   EventTypeLatencyIncrease,
				Time:   0,
				Params: map[string]interface{}{"factor": 3.0},
			},
		},
		Duration: 100 * time.Millisecond,
	}

	err := manager.AddScenario(degradingScenario)
	if err != nil {
		t.Errorf("添加测试场景时出错: %v", err)
	}

	// 启用自动重置
	manager.SetAutoReset(true)

	// 运行场景
	err = manager.RunScenario("quick_degrade")
	if err != nil {
		t.Errorf("运行场景时出错: %v", err)
	}

	// 等待场景完成
	manager.WaitForCompletion()

	// 检查网络条件是否已重置
	currentCondition := simulator.GetCurrentNetworkCondition()
	if currentCondition.ReadDelay != GoodNetworkCondition.ReadDelay {
		t.Errorf("自动重置后，网络条件应该恢复为GoodNetworkCondition，但ReadDelay是: %v", currentCondition.ReadDelay)
	}

	// 再次运行，但禁用自动重置
	manager.SetAutoReset(false)

	// 先将条件改变
	simulator.SetNetworkCondition(GoodNetworkCondition)

	err = manager.RunScenario("quick_degrade")
	if err != nil {
		t.Errorf("运行场景时出错: %v", err)
	}

	// 等待场景完成
	manager.WaitForCompletion()

	// 检查网络条件是否保持为恶化状态
	currentCondition = simulator.GetCurrentNetworkCondition()
	expectedReadDelay := time.Duration(float64(GoodNetworkCondition.ReadDelay) * 3.0)
	if currentCondition.ReadDelay != expectedReadDelay {
		t.Errorf("禁用自动重置后，网络条件应该保持恶化状态，期望ReadDelay: %v, 实际: %v", expectedReadDelay, currentCondition.ReadDelay)
	}
}
