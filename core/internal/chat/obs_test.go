// obs_test.go — OBS 观测面最小实证（v2.5.10 前置档②）
//
// 为什么是测试而不是"起隔离实例"：主控的 8580/8082 在 main.go 里写死（无 env 覆盖），
// 起第二个实例必撞端口 ⇒ 改用**最小实证**：喂假时间序列/假错误，断言观测面真的落盘。
//
// 纪律：测试**绝不写用户真实状态** ⇒ 每个用例都用 t.Setenv("ZERG_STATE_DIR", t.TempDir()) 隔离。
package chat

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// readObs — 读隔离目录里的 chat_obs.jsonl（不存在返回空串）
func readObs(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(os.Getenv("ZERG_STATE_DIR"), obsFileName))
	if err != nil {
		return ""
	}
	return string(b)
}

// A1 类：一轮时序必须落一行，且字段齐全
func TestObsTimerWritesTurnRecord(t *testing.T) {
	t.Setenv("ZERG_STATE_DIR", t.TempDir())

	tm := NewObsTimer("sess-A1", 1, "gemma-4-26B")
	tm.MarkChunk()
	time.Sleep(30 * time.Millisecond) // 制造可观测的"最长停顿"
	tm.MarkChunk()
	tm.MarkChunk()
	tm.Finish("finish")

	out := readObs(t)
	if out == "" {
		t.Fatal("观测文件未写出（应为一条 turn 记录）")
	}
	for _, want := range []string{`"kind":"turn"`, `"session":"sess-A1"`, `"round":1`, `"model":"gemma-4-26B"`,
		`"first_byte_ms"`, `"max_gap_ms"`, `"total_ms"`, `"chunks":3`, `"end_reason":"finish"`} {
		if !strings.Contains(out, want) {
			t.Errorf("记录缺少字段 %s\n原文：%s", want, out)
		}
	}
}

// A2 类：结束原因必须分类（OBS-2 判据：非正常收尾不许无分类）
func TestObsEndReasonClassifies(t *testing.T) {
	t.Setenv("ZERG_STATE_DIR", t.TempDir())

	cases := []struct {
		name string
		err  error
		want string
	}{
		{"正常收尾", nil, "finish"},
		{"上游超时", &ChatInferError{Code: ChatErrUpstreamTimeout}, ChatErrUpstreamTimeout},
		{"客户端中断", &ChatInferError{Code: ChatErrClientAborted}, ChatErrClientAborted},
		{"流截断", &ChatInferError{Code: ChatErrStreamTruncated}, ChatErrStreamTruncated},
		{"裸 DeadlineExceeded", context.DeadlineExceeded, ChatErrUpstreamTimeout},
		{"裸 Canceled", context.Canceled, ChatErrClientAborted},
	}
	for _, c := range cases {
		if got := ObsEndReason(c.err); got != c.want {
			t.Errorf("%s：ObsEndReason=%q，期望 %q", c.name, got, c.want)
		}
	}
}

// A4 类：压缩成功/失败各落一行；counter_reset 字段按传入值落（不编造）
func TestObsCompactWritesRecords(t *testing.T) {
	t.Setenv("ZERG_STATE_DIR", t.TempDir())

	obsCompact("sess-A4", "threshold", 0, 0, 1234, 0, "ok", "", false)
	obsCompact("sess-A4", "threshold", 0, 0, 0, 0, "fail", "HTTP 500 (streak=2)", false)

	out := readObs(t)
	if strings.Count(out, `"kind":"compact"`) != 2 {
		t.Fatalf("应落 2 条 compact，实际：%s", out)
	}
	for _, want := range []string{`"summary_chars":1234`, `"result":"ok"`, `"result":"fail"`, `"fail_reason":"HTTP 500 (streak=2)"`} {
		if !strings.Contains(out, want) {
			t.Errorf("缺少字段 %s\n原文：%s", want, out)
		}
	}
	// counter_reset=false 且 omitempty ⇒ 不出现（宁可缺字段，不编造值）
	if strings.Contains(out, `"counter_reset"`) {
		t.Errorf("counter_reset 未确证时不应出现：%s", out)
	}
}

// A5 类：工具轮次一行（含成败与轮数）
func TestObsToolWritesRecord(t *testing.T) {
	t.Setenv("ZERG_STATE_DIR", t.TempDir())

	ObsTool("sess-A5", 2, "bash", "12ms", true, 2, MaxToolRounds)
	ObsTool("sess-A5", 3, "grep", "3ms", false, 3, MaxToolRounds)

	out := readObs(t)
	if strings.Count(out, `"kind":"tool"`) != 2 {
		t.Fatalf("应落 2 条 tool，实际：%s", out)
	}
	for _, want := range []string{`"tool":"bash"`, `"dur":"12ms"`, `"result":"ok"`, `"tool":"grep"`, `"result":"err"`} {
		if !strings.Contains(out, want) {
			t.Errorf("缺少字段 %s\n原文：%s", want, out)
		}
	}
}

// A6 类：观测**不可写**时，绝不 panic、绝不外抛（对话不受影响）
func TestObsWriteFailureIsBestEffort(t *testing.T) {
	// 把 state 指向一个不可能创建的位置：/dev/null 之下
	t.Setenv("ZERG_STATE_DIR", "/dev/null/nope")

	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("观测写失败**不得 panic**，却 panic 了：%v", r)
			}
		}()
		NewObsTimer("sess-A6", 1, "m").Finish("finish")
		obsCompact("sess-A6", "threshold", 0, 0, 0, 0, "ok", "", false)
		ObsTool("sess-A6", 1, "bash", "1ms", true, 1, MaxToolRounds)
	}()
}

// 分类码常量不得退化（判据字符串是"公开契约"，改动须同步观测面文档）
func TestObsCodeStringsStable(t *testing.T) {
	if ChatErrUpstreamTimeout != "upstream_timeout" ||
		ChatErrClientAborted != "client_aborted" ||
		ChatErrStreamTruncated != "stream_truncated" {
		t.Fatalf("分类码已变：%s / %s / %s", ChatErrUpstreamTimeout, ChatErrClientAborted, ChatErrStreamTruncated)
	}
	_ = errors.Is // 保留导入（Unwrap 依赖 errors 语义）
}
