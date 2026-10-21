package orchestrator

// fixture_test.go — MoA 参考输出截断的 UTF-8 边界判据（两态同形断言）
//
// 被测点：fanout.go `callReferenceWithRetry` 的 `out[:cfg.RefMaxTokens]`（默认 2000）。
// 同一份断言在两个状态下各跑一次：PRE（仓内原样）应红、POST（退字符边界）应绿。
// 判据三条：① 截断结果必须是合法 UTF-8 ② 不超字节预算 ③ 别把预算浪费掉（≥ limit-3）。

import (
	"context"
	"strings"
	"testing"
	"unicode/utf8"
)

// moaStubExecutor — 固定回一段超长内容，用来打截断的切口。
type moaStubExecutor struct {
	content   string
	reasoning string
}

func (s *moaStubExecutor) Execute(ctx context.Context, model, prompt string, maxTokens int) (string, error) {
	return s.content, nil
}

func (s *moaStubExecutor) ExecuteWithResponse(ctx context.Context, model, prompt string, maxTokens int) (*ModelResponse, error) {
	return &ModelResponse{Content: s.content, Reasoning: s.reasoning}, nil
}

// straddleByte — 造一段内容，使第 cut 字节**正落在某个字符中间**（这是红的必要条件）。
// 造不出就 Fatal —— 夹具门槛先过，再谈被测代码红不红。
func straddleByte(t *testing.T, unit string, cut, total int) string {
	t.Helper()
	for pad := 0; pad < 4; pad++ {
		var sb strings.Builder
		sb.WriteString(strings.Repeat("x", pad))
		for sb.Len() <= total {
			sb.WriteString(unit)
		}
		s := sb.String()
		if len(s) > cut && !utf8.RuneStart(s[cut]) { // RuneStart=false ⇒ 续字节 ⇒ 字符中间
			return s
		}
	}
	t.Fatalf("夹具有问题：造不出「第 %d 字节落字符中间」的内容（unit=%q）", cut, unit)
	return ""
}

// tailForMsg — 只用于报错时看清楚切口附近 12 字节。
func tailForMsg(s string) string {
	if len(s) <= 12 {
		return s
	}
	return s[len(s)-12:]
}

func TestMoARefTruncationUTF8Boundary(t *testing.T) {
	cfg := DefaultMoAConfig()
	limit := cfg.RefMaxTokens
	if limit != 2000 {
		t.Fatalf("夹具有问题：默认 RefMaxTokens=%d，本格按 2000 造切口", limit)
	}
	cases := []struct{ name, unit string }{
		{"中文", "测"},     // 3 字节
		{"emoji", "🙂"},  // 4 字节
		{"四字节生僻字", "𠀀"}, // 4 字节（U+20000）
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			full := straddleByte(t, c.unit, limit, 3000)
			o := &Orchestrator{config: DefaultOrchestratorConfig(), executor: &moaStubExecutor{content: full}}
			out, err := o.callReferenceWithRetry(context.Background(), "gemma-4-26B", "用户请求", cfg)
			if err != nil {
				t.Fatalf("参考调用失败: %v", err)
			}
			if !utf8.ValidString(out) {
				t.Fatalf("截断切在字符中间（非法 UTF-8）：%q（limit=%d，Total=%d 字节）", tailForMsg(out), limit, len(full))
			}
			if len(out) > limit {
				t.Fatalf("超字节预算：%d > %d", len(out), limit)
			}
			if len(out) < limit-3 {
				t.Fatalf("预算浪费过头：%d 字节（limit=%d，最多只该少 3 字节）", len(out), limit)
			}
			if !strings.HasPrefix(full, out) {
				t.Fatalf("截断不是原内容前缀：%q", tailForMsg(out))
			}
		})
	}
}

// TestMoARefTruncationReasoningPath — 参考输出优先取 Reasoning（fanout.go 的既有口径），
// 那条路径同样要退字符边界。
func TestMoARefTruncationReasoningPath(t *testing.T) {
	cfg := DefaultMoAConfig()
	full := straddleByte(t, "测", cfg.RefMaxTokens, 3000)
	o := &Orchestrator{config: DefaultOrchestratorConfig(), executor: &moaStubExecutor{
		content:   "短内容",
		reasoning: full,
	}}
	out, err := o.callReferenceWithRetry(context.Background(), "example-8b-quant", "用户请求", cfg)
	if err != nil {
		t.Fatalf("参考调用失败: %v", err)
	}
	if !utf8.ValidString(out) {
		t.Fatalf("Reasoning 截断切在字符中间（非法 UTF-8）：%q", tailForMsg(out))
	}
	if len(out) > cfg.RefMaxTokens {
		t.Fatalf("超字节预算：%d > %d", len(out), cfg.RefMaxTokens)
	}
}

// TestMoARefTruncationASCIIUnchanged — ASCII 行为必须逐字不变（切口永不退）。
func TestMoARefTruncationASCIIUnchanged(t *testing.T) {
	cfg := DefaultMoAConfig()
	limit := cfg.RefMaxTokens
	full := strings.Repeat("a", 3000)
	o := &Orchestrator{config: DefaultOrchestratorConfig(), executor: &moaStubExecutor{content: full}}
	out, err := o.callReferenceWithRetry(context.Background(), "gemma-4-26B", "req", cfg)
	if err != nil {
		t.Fatalf("参考调用失败: %v", err)
	}
	if out != full[:limit] {
		t.Fatalf("ASCII 截断口径变了：len=%d（应 %d）、尾 8 字节=%q", len(out), limit, tailForMsg(out))
	}

	// 正好等于预算（不截）
	exact := strings.Repeat("b", limit)
	o2 := &Orchestrator{config: DefaultOrchestratorConfig(), executor: &moaStubExecutor{content: exact}}
	got, err := o2.callReferenceWithRetry(context.Background(), "gemma-4-26B", "req", cfg)
	if err != nil {
		t.Fatalf("参考调用失败: %v", err)
	}
	if got != exact {
		t.Fatalf("长度正好等于预算时不该截断：%d vs %d", len(got), len(exact))
	}
}
