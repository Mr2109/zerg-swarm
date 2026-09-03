package plugin

import (
	"fmt"
	"sync"
)

// PluginState 插件生命周期状态（设计定稿第四节）。
type PluginState string

const (
	StateLoaded   PluginState = "LOADED"   // 已加载（未启动）
	StateStarting PluginState = "STARTING" // 启动中
	StateRunning  PluginState = "RUNNING"  // 运行中
	StateStopping PluginState = "STOPPING" // 停止中
	StateFailed   PluginState = "FAILED"   // 失败
	StateCrashed  PluginState = "CRASHED"  // 崩溃
	StateUnloaded PluginState = "UNLOADED" // 已卸载
)

// ManagedPlugin 带状态的插件包装。
type ManagedPlugin struct {
	Plugin
	state PluginState
}

// State 返回插件状态。
func (mp *ManagedPlugin) State() PluginState {
	return mp.state
}

// Manager 插件管理器——生命周期管理（Init/Start/Stop/Close——状态机）。
type Manager struct {
	mu      sync.Mutex
	managed map[string]*ManagedPlugin
	order   []string // 启动顺序（拓扑）
	// v2.5.5 P2 可逆副作用（2026-08-21 Mr2109）: 卸载钩子（撤销插件注册）
	unregisterHook func(pluginName string)
}

// NewManager 创建管理器。
func NewManager() *Manager {
	return &Manager{managed: make(map[string]*ManagedPlugin)}
}

// Add 添加插件到管理器（LOADED 状态）。
func (m *Manager) Add(p Plugin) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if p == nil {
		return fmt.Errorf("plugin 不能为 nil")
	}
	if _, exists := m.managed[p.Name()]; exists {
		return fmt.Errorf("插件 %s 已存在", p.Name())
	}
	m.managed[p.Name()] = &ManagedPlugin{Plugin: p, state: StateLoaded}
	m.order = append(m.order, p.Name())
	return nil
}

// Init 初始化所有插件（LOADED → 保持 LOADED，配置已传入）。
func (m *Manager) Init(cfgs map[string]map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, name := range m.order {
		mp := m.managed[name]
		cfg := cfgs[name]
		if cfg == nil {
			cfg = map[string]interface{}{}
		}
		if err := mp.Init(cfg); err != nil {
			mp.state = StateFailed
			return fmt.Errorf("插件 %s 初始化失败: %w", name, err)
		}
	}
	return nil
}

// StartAll 启动所有插件（LOADED → STARTING → RUNNING）。
func (m *Manager) StartAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, name := range m.order {
		mp := m.managed[name]
		mp.state = StateStarting
		if err := mp.Start(); err != nil {
			mp.state = StateFailed
			return fmt.Errorf("插件 %s 启动失败: %w", name, err)
		}
		mp.state = StateRunning
	}
	return nil
}

// Start 启动单个插件（热插拔——不重启主控）。
func (m *Manager) Start(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	mp, ok := m.managed[name]
	if !ok {
		return fmt.Errorf("插件 %s 不存在", name)
	}
	if mp.state == StateRunning {
		return nil
	}
	mp.state = StateStarting
	if err := mp.Start(); err != nil {
		mp.state = StateFailed
		return err
	}
	mp.state = StateRunning
	return nil
}

// Stop 停止单个插件（优雅——等待当前任务完成）。
func (m *Manager) Stop(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	mp, ok := m.managed[name]
	if !ok {
		return fmt.Errorf("插件 %s 不存在", name)
	}
	if mp.state != StateRunning {
		return nil
	}
	mp.state = StateStopping
	if err := mp.Stop(); err != nil {
		mp.state = StateFailed
		return err
	}
	mp.state = StateLoaded
	return nil
}

// StopAll 停止所有插件。
func (m *Manager) StopAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := len(m.order) - 1; i >= 0; i-- {
		name := m.order[i]
		mp := m.managed[name]
		if mp.state != StateRunning {
			continue
		}
		mp.state = StateStopping
		if err := mp.Stop(); err != nil {
			mp.state = StateFailed
			return err
		}
		mp.state = StateLoaded
	}
	return nil
}

// Remove 移除插件（热插拔卸载——先 Stop 再 Close）。
func (m *Manager) Remove(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	mp, ok := m.managed[name]
	if !ok {
		return fmt.Errorf("插件 %s 不存在", name)
	}
	if mp.state == StateRunning {
		mp.state = StateStopping
		if err := mp.Stop(); err != nil {
			mp.state = StateFailed
			return err
		}
	}
	if err := mp.Close(); err != nil {
		return err
	}
	delete(m.managed, name)
	// 从 order 移除
	for i, n := range m.order {
		if n == name {
			m.order = append(m.order[:i], m.order[i+1:]...)
			break
		}
	}
	// v2.5.5 P2 可逆副作用（2026-08-21 Mr2109——dsh Cordis 借鉴）: 插件卸载撤销注册
	// 插件注册的工具从工具集移除（防僵尸工具）——通过回调（避免循环依赖）
	if m.unregisterHook != nil {
		m.unregisterHook(name)
	}
	return nil
}

// SetUnregisterHook 设置卸载钩子（manager 外注入——撤销插件副作用）
// v2.5.5 P2 可逆副作用（2026-08-21 Mr2109）
func (m *Manager) SetUnregisterHook(fn func(pluginName string)) {
	m.unregisterHook = fn
}

// Get 获取受管插件。
func (m *Manager) Get(name string) (*ManagedPlugin, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	mp, ok := m.managed[name]
	return mp, ok
}

// State 查询插件状态。
func (m *Manager) State(name string) PluginState {
	m.mu.Lock()
	defer m.mu.Unlock()
	if mp, ok := m.managed[name]; ok {
		return mp.state
	}
	return StateUnloaded
}
