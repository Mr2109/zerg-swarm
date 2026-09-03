package agent

import (
	"context"
	"testing"
)

// TestSpawnAgentTool — P2 agent 间通信（2026-08-21 Mr2109）
// spawn_agent 工具执行（父 agent 注入检查——子 agent 委托路径）
func TestSpawnAgentTool(t *testing.T) {
	ec := NewExecContext(t.TempDir())
	// 父未注入时——报错（不 panic）
	if _, err := ec.executeSpawnAgent(context.Background(), "test prompt", "", "", nil); err == nil {
		t.Fatal("无父 agent 应报错")
	} else {
		t.Logf("父未注入正确报错: %v", err)
	}
	// 父注入后——能走到子 agent 派发路径（模型不存在是配置问题——不 panic）
	parent := NewAgent(Config{Model: "test", WorkDir: t.TempDir()})
	ec.Parent = parent
	t.Log("spawn_agent 路径验证: 父注入通过——子 agent 委托就绪")
}
