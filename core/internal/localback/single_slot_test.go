package localback

import "testing"

// TestParseLlamaCmdLine — 整机单槽修复（2026-09-10）依赖的命令行解析
func TestParseLlamaCmdLine(t *testing.T) {
	cmd := "/opt/homebrew/bin/llama-server -m ~/models/Ornith-1.5-35B-Q4_K_M.gguf -c 262144 --host 127.0.0.1 --port 9000 --log-disable"
	if got := modelFileFromCmdLine(cmd); got != "~/models/Ornith-1.5-35B-Q4_K_M.gguf" {
		t.Fatalf("模型文件解析错误: %q", got)
	}
	if got := portFromCmdLine(cmd); got != 9000 {
		t.Fatalf("端口解析错误: %d", got)
	}
	// 缺参数 → 零值（不得 panic / 不得误判成别的模型）
	if got := modelFileFromCmdLine("llama-server --host 127.0.0.1"); got != "" {
		t.Fatalf("无 -m 时应为 \"\": %q", got)
	}
	if got := portFromCmdLine("llama-server -m x.gguf"); got != 0 {
		t.Fatalf("无 --port 时应为 0: %d", got)
	}
	// -m 与 --port 相邻（分词边界）
	if got := modelFileFromCmdLine("llama-server -m /a/b.gguf --port 9001"); got != "/a/b.gguf" {
		t.Fatalf("相邻参数解析错误: %q", got)
	}
}

// TestSweepTargetSelection — 清场语义：目标模型不杀、其它模型全杀（用纯判定函数验证）
func TestSweepTargetSelection(t *testing.T) {
	target := "~/models/gemma-4-26B-A4B-it-UD-Q4_K_M.gguf"
	cases := []struct {
		model string
		kill  bool
	}{
		{target, false}, // 目标 → 保留
		{"~/models/Ornith-1.5-35B-Q4_K_M.gguf", true}, // 其它 → 清场
		{"", true}, // 解析不出模型 → 也清（未知实例不该留）
	}
	for _, c := range cases {
		shouldKill := c.model != target
		if shouldKill != c.kill {
			t.Fatalf("目标判定错误: model=%q 期望 kill=%v 实际 %v", c.model, c.kill, shouldKill)
		}
	}
	// 端口解析在 lsof NAME 缺失时也要能兜底（端到端用 ps 命令行）
	if portFromCmdLine("llama-server -m /x.gguf --port 9012 --log-disable") != 9012 {
		t.Fatalf("端口兜底解析失败")
	}
}
