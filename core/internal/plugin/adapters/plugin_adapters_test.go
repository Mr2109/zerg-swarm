package adapters

import (
	"testing"

	"zerg/core/internal/plugin"
)

// ─── SkillPlugin 测试 ───

type mockSkillLoader struct{}

func (m *mockSkillLoader) Load(name string) (string, error) {
	if name == "test-skill" {
		return "技能内容", nil
	}
	return "", nil
}

func (m *mockSkillLoader) ListDescriptions() string { return "test-skill: 测试技能" }

func (m *mockSkillLoader) SkillNames() []string { return []string{"test-skill"} }

func TestSkillPlugin_Interface(t *testing.T) {
	p := NewSkillPlugin(&mockSkillLoader{})
	if p.Name() == "" {
		t.Error("Name 不应为空")
	}
	if p.Type() != plugin.PluginTypeSkill {
		t.Errorf("Type = %s, want skill", p.Type())
	}
}

func TestSkillPlugin_Execute(t *testing.T) {
	p := NewSkillPlugin(&mockSkillLoader{})
	p.Init(nil)
	p.Start()
	out, err := p.Execute(plugin.PluginInput{
		Data: map[string]interface{}{"skill_name": "test-skill"},
	})
	if err != nil {
		t.Fatalf("Execute error = %v", err)
	}
	if out.Result == nil {
		t.Error("Result 不应为空")
	}
}

func TestSkillPlugin_Register(t *testing.T) {
	reg := plugin.NewRegistry()
	p := NewSkillPlugin(&mockSkillLoader{})
	reg.Register(p)
	skills := reg.ByType(plugin.PluginTypeSkill)
	if len(skills) != 1 {
		t.Fatalf("skill 插件应 1 个，实际 %d", len(skills))
	}
}

// ─── McpPlugin 测试 ───

type mockMCPClient struct{}

func (m *mockMCPClient) Connect(name string, command string, args ...string) error { return nil }

func (m *mockMCPClient) ConnectHTTP(name string, baseURL string, headers map[string]string) error {
	return nil
}

func (m *mockMCPClient) Call(serverName, toolName string, args map[string]interface{}) (string, error) {
	if toolName == "test-tool" {
		return "工具结果", nil
	}
	return "", nil
}

func (m *mockMCPClient) Close() {}

func (m *mockMCPClient) ToolDefs(extended bool) []MCPToolDef { return nil }

func TestMcpPlugin_Interface(t *testing.T) {
	p := NewMcpPlugin(&mockMCPClient{})
	if p.Name() == "" {
		t.Error("Name 不应为空")
	}
	if p.Type() != plugin.PluginTypeMCP {
		t.Errorf("Type = %s, want mcp", p.Type())
	}
}

func TestMcpPlugin_Execute(t *testing.T) {
	p := NewMcpPlugin(&mockMCPClient{})
	p.Init(nil)
	p.Start()
	out, err := p.Execute(plugin.PluginInput{
		Data: map[string]interface{}{
			"server_name": "srv",
			"tool_name":   "test-tool",
			"args":        map[string]interface{}{},
		},
	})
	if err != nil {
		t.Fatalf("Execute error = %v", err)
	}
	if out.Result == nil {
		t.Error("Result 不应为空")
	}
}

func TestMcpPlugin_Register(t *testing.T) {
	reg := plugin.NewRegistry()
	p := NewMcpPlugin(&mockMCPClient{})
	reg.Register(p)
	mcps := reg.ByType(plugin.PluginTypeMCP)
	if len(mcps) != 1 {
		t.Fatalf("mcp 插件应 1 个，实际 %d", len(mcps))
	}
}

// ─── CaPlugin 测试 ───

type mockAgentRunner struct{}

func (m *mockAgentRunner) RunTask(task string, workdir string) (string, error) {
	return "agent完成: " + task, nil
}

func TestCaPlugin_Interface(t *testing.T) {
	p := NewCaPlugin(&mockAgentRunner{})
	if p.Name() != "ca" {
		t.Errorf("Name = %s, want ca", p.Name())
	}
	if p.Type() != plugin.PluginTypeCA {
		t.Errorf("Type = %s, want ca", p.Type())
	}
}

func TestCaPlugin_Execute(t *testing.T) {
	p := NewCaPlugin(&mockAgentRunner{})
	p.Start()
	out, err := p.Execute(plugin.PluginInput{
		TaskID:  "t1",
		Context: map[string]interface{}{"task": "写测试"},
	})
	if err != nil {
		t.Fatalf("Execute error = %v", err)
	}
	r := out.Result.(map[string]interface{})
	if r["output"] != "agent完成: 写测试" {
		t.Errorf("output = %v", r["output"])
	}
	if r["task_id"] != "t1" {
		t.Errorf("task_id = %v", r["task_id"])
	}
}

func TestCaPlugin_ExecuteNoTask(t *testing.T) {
	p := NewCaPlugin(&mockAgentRunner{})
	p.Start()
	_, err := p.Execute(plugin.PluginInput{Context: map[string]interface{}{}})
	if err == nil {
		t.Error("空 task 应报错")
	}
}

func TestCaPlugin_Register(t *testing.T) {
	reg := plugin.NewRegistry()
	p := NewCaPlugin(&mockAgentRunner{})
	reg.Register(p)
	cas := reg.ByType(plugin.PluginTypeCA)
	if len(cas) != 1 {
		t.Fatalf("ca 插件应 1 个，实际 %d", len(cas))
	}
}

// ─── 全部并存验证 ───

func TestAllPluginTypesCoexist(t *testing.T) {
	reg := plugin.NewRegistry()
	reg.Register(NewSkillPlugin(&mockSkillLoader{}))
	reg.Register(NewMcpPlugin(&mockMCPClient{}))
	reg.Register(NewCaPlugin(&mockAgentRunner{}))
	reg.Register(NewOrnithAdapter())

	if len(reg.ByType(plugin.PluginTypeSkill)) != 1 {
		t.Error("skill 应 1 个")
	}
	if len(reg.ByType(plugin.PluginTypeMCP)) != 1 {
		t.Error("mcp 应 1 个")
	}
	if len(reg.ByType(plugin.PluginTypeCA)) != 1 {
		t.Error("ca 应 1 个")
	}
	if len(reg.ByType(plugin.PluginTypeModelAdapter)) != 1 {
		t.Error("model-adapter 应 1 个")
	}
	if reg.Count() != 4 {
		t.Errorf("总数应 4，实际 %d", reg.Count())
	}
}

// ─── ToolPlugin 测试 ───

func TestToolPlugin_Interface(t *testing.T) {
	p := NewToolPlugin()
	if p.Name() == "" {
		t.Error("Name 不应为空")
	}
	if p.Type() != plugin.PluginTypeTool {
		t.Errorf("Type = %s, want tool", p.Type())
	}
}

func TestToolPlugin_Execute(t *testing.T) {
	p := NewToolPlugin()
	p.Init(nil)
	p.Start()
	out, err := p.Execute(plugin.PluginInput{
		Context: map[string]interface{}{"command": "echo plugin-ok"},
	})
	if err != nil {
		t.Fatalf("Execute error = %v", err)
	}
	if out.Result == nil {
		t.Error("Result 不应为空")
	}
}

func TestToolPlugin_Register(t *testing.T) {
	reg := plugin.NewRegistry()
	p := NewToolPlugin()
	reg.Register(p)
	tools := reg.ByType(plugin.PluginTypeTool)
	if len(tools) != 1 {
		t.Fatalf("tool 插件应 1 个，实际 %d", len(tools))
	}
}

// ─── ClusterPlugin 测试 ───

func TestClusterPlugin_Interface(t *testing.T) {
	p := NewClusterPlugin(nil)
	if p.Name() != "cluster" {
		t.Errorf("Name = %s, want cluster", p.Name())
	}
	if p.Type() != plugin.PluginTypeCluster {
		t.Errorf("Type = %s, want cluster", p.Type())
	}
}

func TestClusterPlugin_Execute(t *testing.T) {
	p := NewClusterPlugin(nil)
	p.Start()
	out, err := p.Execute(plugin.PluginInput{})
	if err != nil {
		t.Fatalf("Execute error = %v", err)
	}
	r := out.Result.(map[string]interface{})
	if r["count"] != 2 {
		t.Errorf("count = %v, want 2", r["count"])
	}
}

func TestClusterPlugin_Register(t *testing.T) {
	reg := plugin.NewRegistry()
	p := NewClusterPlugin(nil)
	reg.Register(p)
	clusters := reg.ByType(plugin.PluginTypeCluster)
	if len(clusters) != 1 {
		t.Fatalf("cluster 插件应 1 个，实际 %d", len(clusters))
	}
}

// ─── StorePlugin 测试 ───

func TestStorePlugin_Interface(t *testing.T) {
	p := NewStorePlugin()
	if p.Name() != "store" {
		t.Errorf("Name = %s, want store", p.Name())
	}
	if p.Type() != plugin.PluginTypeStore {
		t.Errorf("Type = %s, want store", p.Type())
	}
}

func TestStorePlugin_Execute(t *testing.T) {
	p := NewStorePlugin()
	p.Start()
	// set
	_, err := p.Execute(plugin.PluginInput{
		Data: map[string]interface{}{"action": "set", "key": "k1", "value": "v1"},
	})
	if err != nil {
		t.Fatalf("Set error = %v", err)
	}
	// get
	out, err := p.Execute(plugin.PluginInput{
		Data: map[string]interface{}{"action": "get", "key": "k1"},
	})
	if err != nil {
		t.Fatalf("Get error = %v", err)
	}
	r := out.Result.(map[string]interface{})
	if r["value"] != "v1" {
		t.Errorf("value = %v, want v1", r["value"])
	}
}

func TestStorePlugin_Register(t *testing.T) {
	reg := plugin.NewRegistry()
	p := NewStorePlugin()
	reg.Register(p)
	stores := reg.ByType(plugin.PluginTypeStore)
	if len(stores) != 1 {
		t.Fatalf("store 插件应 1 个，实际 %d", len(stores))
	}
}

// ─── 全插件并存验证（2.5.4 收官）───

func TestAllPluginsCoexist(t *testing.T) {
	reg := plugin.NewRegistry()
	reg.Register(NewSkillPlugin(&mockSkillLoader{}))
	reg.Register(NewMcpPlugin(&mockMCPClient{}))
	reg.Register(NewCaPlugin(&mockAgentRunner{}))
	reg.Register(NewOrnithAdapter())
	reg.Register(NewDs4Adapter())
	reg.Register(NewToolPlugin())
	reg.Register(NewClusterPlugin(nil))
	reg.Register(NewStorePlugin())

	// 8 类插件
	if len(reg.ByType(plugin.PluginTypeSkill)) != 1 {
		t.Error("skill 应 1 个")
	}
	if len(reg.ByType(plugin.PluginTypeMCP)) != 1 {
		t.Error("mcp 应 1 个")
	}
	if len(reg.ByType(plugin.PluginTypeCA)) != 1 {
		t.Error("ca 应 1 个")
	}
	if len(reg.ByType(plugin.PluginTypeModelAdapter)) != 2 {
		t.Error("model-adapter 应 2 个")
	}
	if len(reg.ByType(plugin.PluginTypeTool)) != 1 {
		t.Error("tool 应 1 个")
	}
	if len(reg.ByType(plugin.PluginTypeCluster)) != 1 {
		t.Error("cluster 应 1 个")
	}
	if len(reg.ByType(plugin.PluginTypeStore)) != 1 {
		t.Error("store 应 1 个")
	}
	if reg.Count() != 8 {
		t.Errorf("总数应 8，实际 %d", reg.Count())
	}
}
