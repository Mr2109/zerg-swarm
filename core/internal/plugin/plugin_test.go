package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

// mockPlugin 测试用假插件。
type mockPlugin struct {
	name    string
	ptype   PluginType
	started bool
	stopped bool
	closed  bool
}

func (m *mockPlugin) Name() string                          { return m.name }
func (m *mockPlugin) Type() PluginType                      { return m.ptype }
func (m *mockPlugin) Version() string                       { return "1.0.0" }
func (m *mockPlugin) Capabilities() []string                { return []string{"test"} }
func (m *mockPlugin) Init(cfg map[string]interface{}) error { return nil }
func (m *mockPlugin) Start() error                          { m.started = true; return nil }
func (m *mockPlugin) Stop() error                           { m.stopped = true; return nil }
func (m *mockPlugin) Close() error                          { m.closed = true; return nil }
func (m *mockPlugin) Execute(input PluginInput) (PluginOutput, error) {
	return PluginOutput{Result: "ok:" + input.TaskID}, nil
}

func TestRegistryRegisterGet(t *testing.T) {
	r := NewRegistry()
	p := &mockPlugin{name: "test1", ptype: PluginTypeSkill}
	if err := r.Register(p); err != nil {
		t.Fatalf("注册失败: %v", err)
	}
	got, ok := r.Get("test1")
	if !ok || got != p {
		t.Fatal("获取失败")
	}
	// 重名注册报错
	if err := r.Register(p); err == nil {
		t.Fatal("重名注册应报错")
	}
}

func TestRegistryByType(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockPlugin{name: "a", ptype: PluginTypeSkill})
	r.Register(&mockPlugin{name: "b", ptype: PluginTypeMCP})
	r.Register(&mockPlugin{name: "c", ptype: PluginTypeSkill})
	skills := r.ByType(PluginTypeSkill)
	if len(skills) != 2 {
		t.Fatalf("skill 插件应 2 个，实际 %d", len(skills))
	}
	if r.Count() != 3 {
		t.Fatalf("总数应 3，实际 %d", r.Count())
	}
}

func TestRegistryUnregister(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockPlugin{name: "x", ptype: PluginTypeTool})
	r.Unregister("x")
	if r.Count() != 0 {
		t.Fatal("注销后应为 0")
	}
}

func TestRegistryListStableOrder(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockPlugin{name: "a", ptype: PluginTypeSkill})
	r.Register(&mockPlugin{name: "b", ptype: PluginTypeMCP})
	r.Register(&mockPlugin{name: "c", ptype: PluginTypeTool})
	got := r.List()
	if len(got) != 3 {
		t.Fatalf("List 应 3 个，实际 %d", len(got))
	}
	// 顺序稳定（a,b,c——注册顺序）
	if got[0].Name() != "a" || got[1].Name() != "b" || got[2].Name() != "c" {
		t.Errorf("List 顺序不稳定: %s,%s,%s——应为 a,b,c",
			got[0].Name(), got[1].Name(), got[2].Name())
	}
	// 多次调用顺序一致
	got2 := r.List()
	for i := range got2 {
		if got2[i].Name() != got[i].Name() {
			t.Fatalf("List 两次调用顺序不一致: %v vs %v", got2[i].Name(), got[i].Name())
		}
	}
	// 注销中间——顺序仍稳定（a,c）
	r.Unregister("b")
	got3 := r.List()
	if len(got3) != 2 || got3[0].Name() != "a" || got3[1].Name() != "c" {
		t.Errorf("注销后顺序错误: %v", got3)
	}
}

func TestManagerLifecycle(t *testing.T) {
	m := NewManager()
	p := &mockPlugin{name: "life", ptype: PluginTypeLlama}
	if err := m.Add(p); err != nil {
		t.Fatalf("Add 失败: %v", err)
	}
	if m.State("life") != StateLoaded {
		t.Fatalf("初始状态应为 LOADED，实际 %s", m.State("life"))
	}
	// 启动
	if err := m.StartAll(); err != nil {
		t.Fatalf("StartAll 失败: %v", err)
	}
	if m.State("life") != StateRunning {
		t.Fatalf("启动后应为 RUNNING，实际 %s", m.State("life"))
	}
	if !p.started {
		t.Fatal("插件未真正启动")
	}
	// 停止
	if err := m.Stop("life"); err != nil {
		t.Fatalf("Stop 失败: %v", err)
	}
	if m.State("life") != StateLoaded {
		t.Fatalf("停止后应为 LOADED，实际 %s", m.State("life"))
	}
	if !p.stopped {
		t.Fatal("插件未真正停止")
	}
}

func TestManagerHotReload(t *testing.T) {
	m := NewManager()
	p := &mockPlugin{name: "hot", ptype: PluginTypeCA}
	m.Add(p)
	m.StartAll()
	// 热卸载
	if err := m.Remove("hot"); err != nil {
		t.Fatalf("Remove 失败: %v", err)
	}
	if !p.closed {
		t.Fatal("插件未 Close")
	}
	if _, ok := m.Get("hot"); ok {
		t.Fatal("移除后不应存在")
	}
	// 热加载（新插件）
	p2 := &mockPlugin{name: "hot2", ptype: PluginTypeCA}
	if err := m.Add(p2); err != nil {
		t.Fatalf("热加载失败: %v", err)
	}
	if err := m.Start("hot2"); err != nil {
		t.Fatalf("热启动失败: %v", err)
	}
	if m.State("hot2") != StateRunning {
		t.Fatal("热加载后应 RUNNING")
	}
}

func TestExecute(t *testing.T) {
	p := &mockPlugin{name: "exec", ptype: PluginTypeModelAdapter}
	out, err := p.Execute(PluginInput{TaskID: "t1"})
	if err != nil {
		t.Fatalf("Execute 失败: %v", err)
	}
	if out.Result != "ok:t1" {
		t.Fatalf("执行结果错误: %v", out.Result)
	}
}

func TestLoadConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plugins.yaml")
	content := `plugins:
  - name: ornith-adapter
    type: model-adapter
    version: 1.0.0
    enabled: true
    settings:
      temperature: "0.8"
  - name: llama-runtime
    type: llama
    enabled: true
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("写配置失败: %v", err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}
	if len(cfg.Plugins) != 2 {
		t.Fatalf("应 2 个插件配置，实际 %d", len(cfg.Plugins))
	}
	if cfg.Plugins[0].Name != "ornith-adapter" || cfg.Plugins[0].Type != PluginTypeModelAdapter {
		t.Fatalf("第一个插件解析错误: %+v", cfg.Plugins[0])
	}
	if cfg.Plugins[0].Settings["temperature"] != "0.8" {
		t.Fatalf("settings 解析错误: %v", cfg.Plugins[0].Settings)
	}
}
