package agent

// checker_test.go — 验证器核心测试（2026-08-29 q5 覆盖补齐）
// 目标: TestChecker.Pass(0%) / LLMJudge.Pass(0%) / ComboChecker.Pass(0%) / defaultContext(0%)

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestTestChecker_Pass 命令成功 → 通过
func TestTestChecker_Pass(t *testing.T) {
	tc := TestChecker{Command: "echo hello"}
	ok, out := tc.Pass(nil, "契约")
	if !ok {
		t.Fatalf("echo 应通过: %s", out)
	}
	if !strings.Contains(out, "hello") {
		t.Fatalf("输出应含 hello: %s", out)
	}
}

// TestTestChecker_Fail 命令失败 → 不通过 + 附契约
func TestTestChecker_Fail(t *testing.T) {
	tc := TestChecker{Command: "exit 3"}
	ok, out := tc.Pass(nil, "期望通过")
	if ok {
		t.Fatal("exit 3 应失败")
	}
	if !strings.Contains(out, "契约: 期望通过") {
		t.Fatalf("失败输出应附契约: %s", out)
	}
}

// TestLLMJudge_ConfigIncomplete 配置缺失 → 直接不通过（不发请求）
func TestLLMJudge_ConfigIncomplete(t *testing.T) {
	lj := LLMJudge{} // 无 GatewayURL/Model
	ok, out := lj.Pass(nil, "契约")
	if ok {
		t.Fatal("配置不完整应不通过")
	}
	if !strings.Contains(out, "配置不完整") {
		t.Fatalf("应报配置错误: %s", out)
	}
}

// TestComboChecker_Empty 无验证器 → 视为通过
func TestComboChecker_Empty(t *testing.T) {
	cc := ComboChecker{}
	ok, out := cc.Pass(nil, "契约")
	if !ok || !strings.Contains(out, "无验证器") {
		t.Fatalf("空 ComboChecker 应通过: %s", out)
	}
}

// TestComboChecker_AllPass 全部通过 → 通过
func TestComboChecker_AllPass(t *testing.T) {
	cc := ComboChecker{
		Checkers: []Checker{
			TestChecker{Command: "true"},
			TestChecker{Command: "echo ok"},
		},
	}
	ok, out := cc.Pass(nil, "契约")
	if !ok {
		t.Fatalf("全部通过应 done: %s", out)
	}
	if !strings.Contains(out, "2 项验证通过") {
		t.Fatalf("应报 2 项: %s", out)
	}
}

// TestComboChecker_FirstFail 第一项失败 → 终止
func TestComboChecker_FirstFail(t *testing.T) {
	cc := ComboChecker{
		Checkers: []Checker{
			TestChecker{Command: "false"},
			TestChecker{Command: "true"},
		},
	}
	ok, out := cc.Pass(nil, "契约")
	if ok {
		t.Fatal("第一项失败应不通过")
	}
	if !strings.Contains(out, "第 1/2 项验证失败") {
		t.Fatalf("应报第 1 项: %s", out)
	}
}

// TestDefaultContext_Nil nil → 60s 默认超时
func TestDefaultContext_Nil(t *testing.T) {
	ctx, cancel := defaultContext(nil)
	defer cancel()
	if ctx == nil {
		t.Fatal("nil → 应返回 context")
	}
}

// TestDefaultContext_Context context.Context → 直接使用
func TestDefaultContext_Context(t *testing.T) {
	base := context.Background()
	ctx, _ := defaultContext(base)
	if ctx != base {
		t.Fatal("context.Context 应直接返回")
	}
}

// TestDefaultContext_Other 其他类型 → 60s 默认超时
func TestDefaultContext_Other(t *testing.T) {
	ctx, cancel := defaultContext("not a context")
	defer cancel()
	if ctx == nil {
		t.Fatal("其他类型 → 应返回 context")
	}
	// 应有超时（非 background）
	if _, ok := ctx.Deadline(); !ok {
		t.Fatal("应带超时")
	}
}

// TestDefaultContext_Timeout 超时应生效
func TestDefaultContext_Timeout(t *testing.T) {
	ctx, cancel := defaultContext(nil)
	defer cancel()
	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("应有 deadline")
	}
	if time.Until(deadline) > 61*time.Second {
		t.Fatalf("超时应 ~60s: %v", time.Until(deadline))
	}
}
