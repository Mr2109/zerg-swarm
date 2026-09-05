package chat

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestInferLive — 真网关实测（C4b——工具循环全链路——依赖网关 8082 + X3 模型在线）
// 不是单元测试——本地集成验证（CI 环境网关不在线时会失败——正常）
// 2026-09-05: 加 env 开关（X3 熔断/离线时不红 CI——手动 ZERG_LIVE=1 跑）
func TestInferLive(t *testing.T) {
	if os.Getenv("ZERG_LIVE") == "" {
		t.Skip("真网关集成测试——ZERG_LIVE=1 才跑（依赖 X3 在线）")
	}
	c := NewChatInfer("http://127.0.0.1:8082", "x3gw-shared-2026")
	msgs := []map[string]any{
		{"role": "user", "content": "帮我查一下当前项目目录下的文件列表（用工具）"},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	res, traces, err := RunToolLoop(ctx, c, "example-35b-v2", "你是虫族 AI。", msgs, nil, nil)
	if err != nil {
		t.Fatalf("RunToolLoop 失败: %v", err)
	}
	t.Logf("最终回复: %s", truncateArgs(res.Content, 200))
	t.Logf("工具轨迹: %d 条", len(traces))
	for _, tr := range traces {
		t.Logf("  [%s] %s args=%s → %s", tr.Name, tr.CallID, truncateArgs(tr.Args, 80), truncateArgs(tr.Result, 120))
	}
}
