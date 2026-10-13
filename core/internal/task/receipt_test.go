package task

// receipt_test.go —— 回执 14 组字段 ＋ 「留痕 ≠ 当前态 / 现读」两列并列的判据。
//
// 设计真源：设计-虫族子代理-20261014 §九.2/§九.3（回执 14 组）· §二.8 DQ5（两列并列）· §二.10（两条文案硬规矩）
//
// 本文件的四类判据：
//
//	① **字段齐全**：14 组一组不少、每组的字段名逐字在册（把"少一组"变成可机检的读数）；
//	② **必填/成对**：`caller` 不得空 · `model` 与 `resp_model` 两个都给 · 两把锁都在 · 现场只留指针；
//	③ **出口可辨**：`exit_code` ＋ `signal` 能把 SIGTERM / SIGKILL 分开（这是复盘"优雅退还是硬断"的唯一依据）；
//	④ **两列并列**：同一条输出里**两列都出**，表头逐字，且不一致项能被逐字点出来。

import (
	"bytes"
	"strings"
	"testing"
)

// good 一份**全字段合规**的回执（作为各用例的基准；逐用例只改一格）。
func good() Receipt {
	return Receipt{
		TaskID: "T-20261014-0001", RunID: "R-0001", Attempt: 1,
		Caller: "hermes", SubmittedAt: "2026-10-14T01:00:00+08:00",
		Model: "qwen3.8-27b", RespModel: "qwen3.8-27b@q4_k_m",
		State: "failed", ExitReason: "stalled", FailureReason: FailStalled,
		FinishReasons: []string{"length"},
		InputTokens:   198698, OutputTokens: 128, ReasoningOutputTokens: 64, CacheReadInputTokens: 0,
		APICalls: 3, Turns: 2, AttemptsMade: 1, StalledCounter: 1,
		CreatedAt: "2026-10-14T01:00:00+08:00", StartedAt: "2026-10-14T01:00:01+08:00",
		FinishedAt:      "2026-10-14T01:23:01+08:00",
		LastProgressAt:  "2026-10-14T01:00:02+08:00",
		LastHeartbeatAt: "2026-10-14T01:23:00+08:00",
		DurationMillis:  1380000, QueueMillis: 100, PrefillMillis: 1370000, TTFTMillis: 1379900,
		ResultHash: "sha256:aaaa", SchemaValid: false, Truncated: true,
		ExitCode: 137, Signal: "SIGKILL",
		Worktree: "/tmp/wt/R-0001", DiffStat: " 3 files changed, 41 insertions(+)", LogPath: "/tmp/logs/R-0001.log",
		AuditSeq: 4211, AuditHash: "hash:bbbb", PrevHash: "hash:aaaa",
		IdempotencyKey: "slice-9/k-1", RequestHash: "req:cccc",
	}
}

// ① 字段齐全：14 组、逐组字段名逐字在册；且**每个字段都能取到值**（ToRow 不缺键）。
func TestReceipt_FourteenGroupsComplete(t *testing.T) {
	groups := ReceiptGroupNames()
	if len(groups) != ReceiptGroupCount {
		t.Fatalf("回执必须 **14 组**（设计 §九.3）：实得 %d 组 → %v", len(groups), groups)
	}
	fields := ReceiptFieldNames()
	if len(fields) == 0 {
		t.Fatal("字段面为空 ⇒ 14 组是空壳")
	}
	row := good().ToRow()
	for _, f := range fields {
		if _, ok := row[f]; !ok {
			t.Fatalf("字段 %q 在 ToRow 里取不到值（字段表与取值口径漂了）", f)
		}
	}
	// 逐组的"点名件"必须在册（少一组就红）。
	must := []string{
		"task_id", "run_id", "attempt", "caller", "model", "resp_model", "failure_reason",
		"finish_reasons", "input_tokens", "reasoning_output_tokens", "cache_read_input_tokens",
		"api_calls", "stalled_counter", "last_progress_at", "last_heartbeat_at",
		"duration_ms", "prefill_ms", "ttft_ms", "result_hash", "schema_valid", "truncated",
		"exit_code", "signal", "worktree", "diff_stat", "log_path",
		"audit_seq", "audit_hash", "prev_hash", "idempotency_key", "request_hash",
	}
	set := map[string]bool{}
	for _, f := range fields {
		set[f] = true
	}
	for _, m := range must {
		if !set[m] {
			t.Fatalf("14 组里缺字段 %q", m)
		}
	}
}

// ② 基准必须全绿（否则后面的负控没有意义）。
func TestReceipt_Validate_BaselineGreen(t *testing.T) {
	if bad := good().Validate(); len(bad) != 0 {
		t.Fatalf("基准回执应零缺失，实得 %v", bad)
	}
}

// ② 负控：`caller` 不得空（「✗ 不能出现谁派的查不到」）。
func TestReceipt_Validate_CallerRequired(t *testing.T) {
	r := good()
	r.Caller = ""
	if !contains(r.Validate(), "caller") {
		t.Fatalf("空 caller 必须被判缺：%v", r.Validate())
	}
}

// ② 负控：`model` 与 `resp_model` 两个都给（`resp_model` 是**回读到的实际模型**）。
func TestReceipt_Validate_ModelAndRespModel(t *testing.T) {
	r1 := good()
	r1.Model = ""
	if !contains(r1.Validate(), "model") {
		t.Fatal("空 model 必须被判缺")
	}
	r2 := good()
	r2.RespModel = ""
	if !contains(r2.Validate(), "resp_model") {
		t.Fatal("空 resp_model 必须被判缺（两个都给，不许只给一个）")
	}
}

// ② 负控：两把锁都在（判卡只看 progress，但报告里两把都要出）。
func TestReceipt_Validate_BothClocks(t *testing.T) {
	r := good()
	r.LastProgressAt = ""
	if !contains(r.Validate(), "last_progress_at") {
		t.Fatal("缺 progress 锁必须被判缺")
	}
	r = good()
	r.LastHeartbeatAt = ""
	if !contains(r.Validate(), "last_heartbeat_at") {
		t.Fatal("缺 liveness 锁必须被判缺")
	}
}

// ② 负控：现场**只留指针** —— 三件都在，且 `diff_stat` 只能是摘要（不许把整篇 diff 塞回执）。
func TestReceipt_Validate_PointersOnly(t *testing.T) {
	r := good()
	r.Worktree = ""
	if !contains(r.Validate(), "worktree") {
		t.Fatal("缺 worktree 指针必须被判缺")
	}
	r = good()
	r.LogPath = ""
	if !contains(r.Validate(), "log_path") {
		t.Fatal("缺 log_path 指针必须被判缺")
	}
	r = good()
	r.DiffStat = "diff --git a/x b/x\n+1\n-1\n+2\n-2\n" // 把整篇 diff 塞进来 = 违反"只留指针"
	if !contains(r.Validate(), "diff_stat") {
		t.Fatalf("多行 diff 必须被判不合规（现场只留指针）：%v", r.Validate())
	}
}

// ③ 出口可辨：SIGTERM / SIGKILL 必须分得开，且信号号写法也要认。
func TestExitOutcome_DistinguishesTermVsKill(t *testing.T) {
	r := good()
	r.ExitCode, r.Signal = 143, "SIGTERM"
	term := r.ExitOutcome()
	if !strings.Contains(term, "SIGTERM") || strings.Contains(term, "SIGKILL") {
		t.Fatalf("SIGTERM 档必须说清是优雅终止请求：%s", term)
	}
	r.ExitCode, r.Signal = 137, "SIGKILL"
	kill := r.ExitOutcome()
	if !strings.Contains(kill, "SIGKILL") || !strings.Contains(kill, "硬断") {
		t.Fatalf("SIGKILL 档必须说清是硬断：%s", kill)
	}
	// 信号号写法（143/137 之外的 `15` / `9`）也要归到同一档。
	if SignalKind("15") != SignalTERM || SignalKind("-15") != SignalTERM {
		t.Fatal("信号号 15 必须归到 SIGTERM 档")
	}
	if SignalKind("9") != SignalKILL || SignalKind("Kill") != SignalKILL {
		t.Fatal("信号号/大小写 9 必须归到 SIGKILL 档")
	}
	if SignalKind("") != SignalNone || SignalKind("0") != SignalNone {
		t.Fatal("无信号必须归到 None 档（正常退出码路径）")
	}
	// 自由文本不许混进来冒充信号。
	r = good()
	r.Signal = "被我们请退了"
	if !contains(r.Validate(), "signal") {
		t.Fatalf("自由文本 signal 必须被判不合规（否则 SIGKILL 判不出来）：%v", r.Validate())
	}
}

// ③ 出口：正常退出（无信号）与"它自己退的"必须分开说。
func TestExitOutcome_NormalExit(t *testing.T) {
	r := good()
	r.ExitCode, r.Signal = 0, ""
	if got := r.ExitOutcome(); !strings.Contains(got, "正常退出") {
		t.Fatalf("exit_code=0 且无信号 ⇒ 正常退出：%s", got)
	}
	r.ExitCode = 2
	if got := r.ExitOutcome(); !strings.Contains(got, "非零退出码") {
		t.Fatalf("非零退出码 + 无信号 ⇒ 它自己退的：%s", got)
	}
}

// ④ ★ 两列并列：**同一条输出里两列都出**，表头逐字，且不一致项能被逐字点出来。
//
// 这一格直接复刻 §二.10 的**今晚实证**：回执印 `内核=rollback`（最近一次运行的留痕），
// 而实况在跑新件（`code_sha=ca565e927061` ＝ 目标）⇒ 用 `resp_model` 装这个例子。
func TestWire_TwoColumns_RetainedVsLive_Verbatim(t *testing.T) {
	retained := good()
	retained.RespModel = "rollback" // ← 最近一次运行的留痕（✗ 不是当前态）
	live := good()
	live.RespModel = "ca565e927061" // ← 现读：实况在跑的那个

	w := Wire{Retained: retained, Live: live}
	var buf bytes.Buffer
	w.Render(&buf, []string{"task_id", "resp_model", "audit_seq"})
	out := buf.String()
	// ★ 两列并列的**取证样本**（判据的读数就是它；`-v` 时逐字打出来）。
	t.Logf("两列并列样本（§二.10 的今晚实证 · resp_model 就是那一格）：\n%s", out)

	// ① 两列都在同一条输出里（✗ 不许只出一列）
	if !strings.Contains(out, RetainedHeader) {
		t.Fatalf("回执那一列的表头必须逐字是「%s」：\n%s", RetainedHeader, out)
	}
	if !strings.Contains(out, LiveHeader) {
		t.Fatalf("现读那一列的表头必须逐字是「%s」：\n%s", LiveHeader, out)
	}
	// ② 两侧的值逐字都在（留痕不许被现读掩盖，反之亦然）
	for _, want := range []string{"rollback", "ca565e927061"} {
		if !strings.Contains(out, want) {
			t.Fatalf("两列必须**都**出值，缺 %q：\n%s", want, out)
		}
	}
	// ③ 逐字对比一例：不一致项恰好是 resp_model 那一格
	if got := w.DiffFields([]string{"task_id", "resp_model", "audit_seq"}); len(got) != 1 || got[0] != "resp_model" {
		t.Fatalf("逐字对比应恰指出 resp_model 一格不一致，实得 %v", got)
	}
	// ④ 一致的字段不许被报成不一致（负控：探针会区分）
	same := Wire{Retained: good(), Live: good()}
	if got := same.DiffFields(nil); len(got) != 0 {
		t.Fatalf("两侧逐字相同时不许报差异：%v", got)
	}
	// ⑤ 渲染里那行必须标出不一致（人面一眼看到"该信哪一列"）
	if !strings.Contains(out, "不一致") {
		t.Fatalf("不一致的那一行要有标记：\n%s", out)
	}
}

// ④ 两列并列：字段面默认出**全部 14 组**（✗ 不许默认只出一列/只出摘要那几个格）。
func TestWire_RenderDefaultsToAllFields(t *testing.T) {
	var buf bytes.Buffer
	Wire{Retained: good(), Live: good()}.Render(&buf, nil)
	out := buf.String()
	for _, f := range ReceiptFieldNames() {
		if !strings.Contains(out, f+" ") && !strings.Contains(out, f+"\n") && !strings.Contains(out, "| "+f) {
			t.Fatalf("默认渲染应出全 14 组字段，缺 %q", f)
		}
	}
}

func contains(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}
