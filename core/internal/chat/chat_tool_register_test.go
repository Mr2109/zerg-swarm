package chat

// chat_tool_register_test.go — P4-49 统一工具库单测（CA 执行器路由验证）

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/agent"
)

// TestRegisterChatExtraTools — 注册后 agent.AllTools 包含对话工具 + ExecuteTool 可路由执行
func TestRegisterChatExtraTools(t *testing.T) {
	RegisterChatExtraTools()

	// 1. AllTools 合并（CA 工具列表包含对话工具）
	all := agent.AllTools()
	names := map[string]bool{}
	for _, td := range all {
		names[td.Function.Name] = true
	}
	for _, want := range []string{"task_list", "fleet_status", "footage_search", "file_count", "tree"} {
		if !names[want] {
			t.Errorf("AllTools 缺少对话工具 %s", want)
		}
	}

	// 2. ExecuteTool 路由执行（无参工具——task_list 连 core API）
	// 这一步**依赖活主控**：CI/公开快照上没有 127.0.0.1:8580，实测会 "connection refused" 而整作业变红。
	// 因此先探测端口：不通就跳过该断言（本机开发时主控在跑，断言照旧执行，覆盖不丢）。
	if !coreAPIAvailable("127.0.0.1:8580") {
		t.Log("未检测到主控 API（127.0.0.1:8580）——跳过 task_list 执行断言（CI/快照形态）")
	} else {
		ec := agent.NewExecContext(t.TempDir())
		res := ec.ExecuteTool(context.Background(), "task_list", map[string]any{"limit": 3}, nil)
		if res.Error != "" {
			t.Errorf("task_list 执行失败: %s", res.Error)
		}
		if res.Content == "" {
			t.Errorf("task_list 结果为空")
		}
		t.Logf("task_list 执行结果: %.120s", res.Content)
	}

	// 3. 未注册工具仍报未知（与主控无关，任何形态都必须成立）
	ec2 := agent.NewExecContext(t.TempDir())
	res2 := ec2.ExecuteTool(context.Background(), "__no_such_tool_xyz__", map[string]any{}, nil)
	if res2.Error == "" {
		t.Errorf("未注册工具应报错")
	}
}

// TestExecuteChatToolCounts — 对话路径计数（ExecuteChatTool 成功 → RecordToolUse）
func TestExecuteChatToolCounts(t *testing.T) {
	before := agent.ToolUses("tree")
	res := ExecuteChatTool("tree", map[string]any{"path": "."}, t.TempDir())
	if res.Error != "" {
		t.Errorf("tree 执行失败: %s", res.Error)
	}
	after := agent.ToolUses("tree")
	if after != before+1 {
		t.Errorf("tree 计数未增: before=%d after=%d", before, after)
	}
	t.Logf("tree 计数: %d → %d", before, after)
}

// coreAPIAvailable 探测主控 API 是否可连（用于让"依赖活服务"的断言在 CI 上跳过而不是失败）。
func coreAPIAvailable(addr string) bool {
	c, err := net.DialTimeout("tcp", addr, 300*time.Millisecond)
	if err != nil {
		return false
	}
	_ = c.Close()
	return true
}
