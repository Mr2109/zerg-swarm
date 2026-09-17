// obs_behavior_test.go — T1.3 实证：早退/空转是**可判定的观测**，不是事后翻 git 才知道。
//
// 要治的盲区（写在这里，改测试前先读）：
//
//	1:1 真考题里「执行模型零改动、只读不写就交差」与「正常干完活」在观测面**同形** ——
//	都只有若干条 OBS-3 工具行 + 一条正常收尾的 turn 记录 ⇒ 只能事后 git diff 才知道它没干活。
//	本文件守住四条用例（含两条**防误报反例**，全部走**真内核循环**，不 mock 观测链路）：
//	  ① 零工具调用的会话   ⇒ d) 落显式标记（first_tool_call_seen=false + 非空轮数）、e) early_exit_suspected=true
//	  ② 有读无写的会话     ⇒ c) turns_since_last_write 逐轮递增、b) 连续零调用归零/再累
//	  ③ 正常写类会话       ⇒ e) **false**（反例：防把真干活误报成早退）
//	  ④ 单轮多工具         ⇒ a) tool_calls_this_turn 准确（3 就是 3，且一轮一条、不重复计）
//	  另有两条负控：写了但失败 / 轮数用尽未交答案 ⇒ 各自的判据成立（防"零写即报"的鲁莽结论）
//
// 纪律（与 obs_test.go / obs_tool_decision_test.go 同源）：
//   - 观测一律落隔离目录：t.Setenv("ZERG_STATE_DIR", t.TempDir())，绝不写用户真实状态。
//   - 判据分两层：**wire 层**（键真的在 JSONL 原文里）+ **read 层**（解回来值是真的）——只断 Go 字段=自证。
//   - 本文件只**读**内核产出的行为信号；不改任何工具行为/安全语义。
package chat

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/loopcore"
)

// obsBehaviorLine — 行为记录读回结构（键缺席 ⇒ 指针为 nil，这正是"这类块没落"的判据）
type obsBehaviorLine struct {
	TS              string              `json:"ts"`
	Kind            string              `json:"kind"`
	Session         string              `json:"session"`
	Round           int                 `json:"round"`
	EventName       string              `json:"event_name"`
	TraceID         string              `json:"trace_id"`
	SpanID          string              `json:"span_id"`
	Behavior        *BehaviorObs        `json:"behavior"`
	BehaviorSession *BehaviorSessionObs `json:"behavior_session"`
}

// readBehaviorLines — 只挑 kind=behavior 的行（其余事件不干扰断言）
func readBehaviorLines(t *testing.T) []obsBehaviorLine {
	t.Helper()
	raw := readObs(t)
	if raw == "" {
		return nil
	}
	var out []obsBehaviorLine
	for _, ln := range strings.Split(strings.TrimSpace(raw), "\n") {
		if strings.TrimSpace(ln) == "" {
			continue
		}
		var r obsBehaviorLine
		if err := json.Unmarshal([]byte(ln), &r); err != nil {
			t.Fatalf("观测行不是合法 JSON（读侧必须宽容）：%v\n%s", err, ln)
		}
		if r.Kind == obsKindBehavior {
			out = append(out, r)
		}
	}
	return out
}

// behaviorRoundOf — 取某一轮的轮级块（同一轮只许有一条 —— 重复落盘会让"跑了几轮"失真）
func behaviorRoundOf(t *testing.T, round int) *BehaviorObs {
	t.Helper()
	lines := readBehaviorLines(t)
	var hit *BehaviorObs
	n := 0
	for _, ln := range lines {
		if ln.EventName != obsEventBehaviorRound || ln.Round != round {
			continue
		}
		n++
		hit = ln.Behavior
		if ln.Behavior == nil {
			t.Fatalf("第 %d 轮的轮级记录缺 behavior 块：%+v", round, ln)
		}
		if len(ln.TraceID) != obsIDHexLen || len(ln.SpanID) != obsIDHexLen {
			t.Errorf("行为记录必须挂 T1.1 追踪骨架（否则拼不进父子树）：trace=%q span=%q", ln.TraceID, ln.SpanID)
		}
	}
	if n == 0 {
		t.Fatalf("第 %d 轮没有轮级行为记录（观测缺口）：%s", round, readObs(t))
	}
	if n > 1 {
		t.Fatalf("第 %d 轮落了 %d 条轮级行为记录（一轮一条，不许重复计）", round, n)
	}
	return hit
}

// behaviorSessionOf — 取会话级结论（T1.3 e 的唯一出处）
func behaviorSessionOf(t *testing.T) *BehaviorSessionObs {
	t.Helper()
	lines := readBehaviorLines(t)
	var hit *BehaviorSessionObs
	n := 0
	for _, ln := range lines {
		if ln.EventName != obsEventBehaviorSession {
			continue
		}
		n++
		hit = ln.BehaviorSession
	}
	if n != 1 {
		t.Fatalf("会话级结论应恰好 1 条，实际 %d 条：%s", n, readObs(t))
	}
	if hit == nil {
		t.Fatal("会话级记录缺 behavior_session 块")
	}
	return hit
}

// ── 假 Infer / Exec / 终结仲裁（只造事实，不 mock 观测链路 —— 观测走真 loopcore→behaviorobs→chat）──

// fbInfer — 按剧本逐轮返回响应；剧本用尽则**重复最后一个**（便于造"永远不停"的会话）
func fbInfer(script []*loopcore.Response) loopcore.Infer {
	i := 0
	return func(ctx context.Context, model, sysPrompt string, msgs []map[string]any,
		onDelta func(deltaType, text string), tools []map[string]any) (*loopcore.Response, error) {
		if i >= len(script) {
			i = len(script) - 1
		}
		r := script[i]
		i++
		return r, nil
	}
}

// fbExec — 工具执行桩：fail 里的工具名返回错误（用于"写了但失败"的负控）
func fbExec(fail map[string]bool, calls *int) loopcore.ToolExec {
	return func(ctx context.Context, name string, args map[string]any) (string, string, error) {
		if calls != nil {
			*calls++
		}
		if fail[name] {
			return "", "1ms", errString("写失败：磁盘只读")
		}
		return "ok", "1ms", nil
	}
}

type errString string

func (e errString) Error() string { return string(e) }

// fbTerm — 终结仲裁桩：前 allow 次「继续」，之后终结（模拟 CA 契约判定/引导轮）
type fbTerm struct {
	allow int
	calls int
}

func (t *fbTerm) OnNoToolCall(r *loopcore.Response) (bool, string) {
	t.calls++
	if t.calls <= t.allow {
		return false, "（还没干完，继续）"
	}
	return true, ""
}

func fbCall(id, name string) loopcore.ToolCall {
	return loopcore.ToolCall{ID: id, Name: name, Args: map[string]any{"path": "x.txt"}}
}

func fbMsg() []map[string]any {
	return []map[string]any{{"role": "user", "content": "去办这件事"}}
}

// ① 零工具调用的会话 ⇒ d) 落**显式标记**（不是留空）、e) early_exit_suspected 为真
func TestBehaviorWireZeroToolCallSessionFlagged(t *testing.T) {
	t.Setenv("ZERG_STATE_DIR", t.TempDir())
	const sess = "sess-T13w-zero"

	res := loopcore.Run(context.Background(), loopcore.Config{MaxRounds: 4}, "m", "sys", fbMsg(), loopcore.Deps{
		Infer:      fbInfer([]*loopcore.Response{{Content: "我看了一遍，应该没问题。"}}),
		Exec:       fbExec(nil, nil),
		Terminator: &fbTerm{allow: 1}, // 第一轮零工具被"打回继续"，第二轮零工具才收尾
		Session:    sess,
	})
	if res.ExitKind != "natural" {
		t.Fatalf("本用例应正常收尾（交了答案），实际 exit=%q", res.ExitKind)
	}

	// wire 层：五个判据键必须真的在落盘原文里
	raw := readObs(t)
	for _, key := range []string{
		`"kind":"behavior"`, `"event_name":"behavior_round"`, `"event_name":"behavior_session"`,
		`"session":"sess-T13w-zero"`, `"tool_calls_this_turn":0`, `"first_tool_call_seen":false`,
		`"early_exit_suspected":true`, `"total_tool_calls":0`, `"final_answer":true`,
	} {
		if !strings.Contains(raw, key) {
			t.Errorf("落盘原文缺少 %s\n原文：%s", key, raw)
		}
	}

	// read 层：轮级（a/d）
	r2 := behaviorRoundOf(t, 2)
	if r2.ToolCallsThisTurn != 0 {
		t.Errorf("a) 零工具调用轮应记 0，实际 %d", r2.ToolCallsThisTurn)
	}
	if r2.TurnsWithoutToolCall != 2 {
		t.Errorf("b) 连续零工具调用轮数应为 2，实际 %d", r2.TurnsWithoutToolCall)
	}
	if r2.TurnsSinceLastWrite != 2 {
		t.Errorf("c) 零写会话应逐轮递增（第 2 轮=2），实际 %d", r2.TurnsSinceLastWrite)
	}
	if r2.FirstToolCallSeen {
		t.Error("d) 整会话零工具调用 ⇒ first_tool_call_seen 必须为 false（显式标记）")
	}
	if r2.TurnsBeforeFirstToolCall != 2 {
		t.Errorf("d) 零调用时应记「至今 2 轮无调用」（非空，不许留空），实际 %d", r2.TurnsBeforeFirstToolCall)
	}

	// read 层：会话级（d/e）
	sum := behaviorSessionOf(t)
	if !sum.EarlyExitSuspected {
		t.Error("e) 整会话零写类工具成功 且 已给出终答 ⇒ early_exit_suspected 必须为 true")
	}
	if sum.FirstToolCallSeen || sum.TurnsBeforeFirstToolCall != 2 {
		t.Errorf("会话级 d 标记不完整：seen=%v before=%d", sum.FirstToolCallSeen, sum.TurnsBeforeFirstToolCall)
	}
	if !sum.Tracked || !sum.FinalAnswer || sum.ExitKind != "natural" {
		t.Errorf("会话级事实不完整：tracked=%v final=%v exit=%q", sum.Tracked, sum.FinalAnswer, sum.ExitKind)
	}
	if sum.TotalToolCalls != 0 || sum.WroteEver || sum.TotalWriteOK != 0 {
		t.Errorf("零工具调用会话的计数应为 0：total=%d wrote=%v writeOK=%d", sum.TotalToolCalls, sum.WroteEver, sum.TotalWriteOK)
	}
}

// ② 有读无写的会话 ⇒ c) turns_since_last_write **逐轮递增**；b) 有调用即归零、零调用再累
func TestBehaviorWireReadOnlySessionSinceLastWriteIncrements(t *testing.T) {
	t.Setenv("ZERG_STATE_DIR", t.TempDir())
	const sess = "sess-T13w-readonly"

	res := loopcore.Run(context.Background(), loopcore.Config{MaxRounds: 6}, "m", "sys", fbMsg(), loopcore.Deps{
		Infer: fbInfer([]*loopcore.Response{
			{Content: "我先看看。"},                                                         // r1：零工具（被打回继续）
			{ToolCalls: []loopcore.ToolCall{fbCall("1", "read")}},                      // r2：读
			{ToolCalls: []loopcore.ToolCall{fbCall("2", "read"), fbCall("3", "grep")}}, // r3：读
			{Content: "看完了，结论如下。"},                                                     // r4：零工具 ⇒ 收尾
		}),
		Exec:       fbExec(nil, nil),
		Terminator: &fbTerm{allow: 1},
		Session:    sess,
	})
	if res.ExitKind != "natural" {
		t.Fatalf("应正常收尾，实际 exit=%q", res.ExitKind)
	}

	// 逐轮递增（c 的核心判据）：1 → 2 → 3 → 4
	prev := -1
	for round := 1; round <= 4; round++ {
		b := behaviorRoundOf(t, round)
		if b.TurnsSinceLastWrite != round {
			t.Errorf("c) 第 %d 轮 turns_since_last_write 应为 %d（有读无写 ⇒ 递增），实际 %d", round, round, b.TurnsSinceLastWrite)
		}
		if b.TurnsSinceLastWrite <= prev {
			t.Errorf("c) 第 %d 轮未递增：%d <= %d", round, b.TurnsSinceLastWrite, prev)
		}
		prev = b.TurnsSinceLastWrite
		if b.WroteEver {
			t.Errorf("第 %d 轮不该 wrote_ever=true（全程只读）", round)
		}
	}
	// b) 连续零工具调用轮数：r1=1（清零前）→ r2=0（归零）→ r4=1（再累）
	if b1 := behaviorRoundOf(t, 1); b1.TurnsWithoutToolCall != 1 {
		t.Errorf("b) r1 零调用 ⇒ 1，实际 %d", b1.TurnsWithoutToolCall)
	}
	if b2 := behaviorRoundOf(t, 2); b2.TurnsWithoutToolCall != 0 {
		t.Errorf("b) r2 有调用 ⇒ 归零，实际 %d", b2.TurnsWithoutToolCall)
	}
	if b4 := behaviorRoundOf(t, 4); b4.TurnsWithoutToolCall != 1 {
		t.Errorf("b) r4 零调用 ⇒ 重新累计为 1，实际 %d", b4.TurnsWithoutToolCall)
	}
	// d) 「首次之前的轮数」定住不涨：首次调用在第 2 轮 ⇒ 恒为 1
	for round := 2; round <= 4; round++ {
		if b := behaviorRoundOf(t, round); b.TurnsBeforeFirstToolCall != 1 || !b.FirstToolCallSeen {
			t.Errorf("d) 第 %d 轮：首次之前的轮数应恒为 1 且 seen=true，实际 %d/%v",
				round, b.TurnsBeforeFirstToolCall, b.FirstToolCallSeen)
		}
	}
	// a) 准确：r3 是 2 次调用
	if b3 := behaviorRoundOf(t, 3); b3.ToolCallsThisTurn != 2 {
		t.Errorf("a) r3 本轮调用数应为 2，实际 %d", b3.ToolCallsThisTurn)
	}
	// 会话级：只读交差 ⇒ 命中早退怀疑
	sum := behaviorSessionOf(t)
	if !sum.EarlyExitSuspected {
		t.Error("e) 全程只读 + 已交答案 ⇒ early_exit_suspected 必须为 true")
	}
	if sum.TotalToolCalls != 3 || sum.TotalWriteOK != 0 || sum.WroteEver {
		t.Errorf("会话级计数不符：total=%d writeOK=%d wrote=%v", sum.TotalToolCalls, sum.TotalWriteOK, sum.WroteEver)
	}
	if !strings.Contains(readObs(t), `"turns_since_last_write":4`) {
		t.Errorf("落盘原文缺 `\"turns_since_last_write\":4`：%s", readObs(t))
	}
}

// ③ 正常写类会话 ⇒ e) **必须为假**（反例：防把真干活误报成早退）
func TestBehaviorWireWriteSessionNotFlagged(t *testing.T) {
	t.Setenv("ZERG_STATE_DIR", t.TempDir())
	const sess = "sess-T13w-write"

	res := loopcore.Run(context.Background(), loopcore.Config{MaxRounds: 4}, "m", "sys", fbMsg(), loopcore.Deps{
		Infer: fbInfer([]*loopcore.Response{
			{ToolCalls: []loopcore.ToolCall{fbCall("1", "write")}}, // r1：写
			{Content: "改好了。"},                                      // r2：交差
		}),
		Exec:    fbExec(nil, nil),
		Session: sess,
	})
	if res.ExitKind != "natural" {
		t.Fatalf("应正常收尾，实际 exit=%q", res.ExitKind)
	}

	r1 := behaviorRoundOf(t, 1)
	if r1.WriteCallsThisTurn != 1 {
		t.Errorf("r1 应有 1 次写类工具成功，实际 %d", r1.WriteCallsThisTurn)
	}
	if r1.TurnsSinceLastWrite != 0 {
		t.Errorf("c) 本轮写成功 ⇒ 距上次写归零，实际 %d", r1.TurnsSinceLastWrite)
	}
	if !r1.WroteEver || r1.TotalWriteOK != 1 {
		t.Errorf("写成功应记 wrote_ever=true/total_write_ok=1，实际 %v/%d", r1.WroteEver, r1.TotalWriteOK)
	}

	sum := behaviorSessionOf(t)
	if sum.EarlyExitSuspected {
		t.Error("**反例**：本会话真写了东西 ⇒ early_exit_suspected 必须为 false（否则真干活会被误报）")
	}
	if !sum.WroteEver || sum.TotalWriteOK != 1 || sum.TurnsSinceLastWrite != 1 {
		t.Errorf("会话级写事实不符：wrote=%v writeOK=%d since=%d", sum.WroteEver, sum.TotalWriteOK, sum.TurnsSinceLastWrite)
	}
	if !strings.Contains(readObs(t), `"early_exit_suspected":false`) {
		t.Errorf("落盘原文缺 `\"early_exit_suspected\":false`：%s", readObs(t))
	}
}

// ④ 单轮多工具 ⇒ a) tool_calls_this_turn **准确**（3 就是 3，且一轮一条不重复）
func TestBehaviorWireSingleRoundMultiToolCount(t *testing.T) {
	t.Setenv("ZERG_STATE_DIR", t.TempDir())
	const sess = "sess-T13w-multi"

	execCalls := 0
	res := loopcore.Run(context.Background(), loopcore.Config{MaxRounds: 4}, "m", "sys", fbMsg(), loopcore.Deps{
		Infer: fbInfer([]*loopcore.Response{
			{ToolCalls: []loopcore.ToolCall{fbCall("1", "read"), fbCall("2", "grep"), fbCall("3", "ls")}}, // r1：一轮三调
			{Content: "看完了。"}, // r2：交差
		}),
		Exec:    fbExec(nil, &execCalls),
		Session: sess,
	})
	if res.ExitKind != "natural" {
		t.Fatalf("应正常收尾，实际 exit=%q", res.ExitKind)
	}
	if execCalls != 3 {
		t.Errorf("本轮三次调用都应真的执行（观测不得影响执行）：实际执行 %d 次", execCalls)
	}

	r1 := behaviorRoundOf(t, 1) // 内部同时断言"一轮恰好一条"
	if r1.ToolCallsThisTurn != 3 {
		t.Errorf("a) 单轮 3 次调用应记 3，实际 %d", r1.ToolCallsThisTurn)
	}
	if r1.TotalToolCalls != 3 {
		t.Errorf("a) 累计调用数应为 3，实际 %d", r1.TotalToolCalls)
	}
	if r1.TurnsWithoutToolCall != 0 {
		t.Errorf("b) 本轮有调用 ⇒ 连续零调用轮数应为 0，实际 %d", r1.TurnsWithoutToolCall)
	}
	if r1.WriteCallsThisTurn != 0 || r1.WroteEver {
		t.Errorf("本轮全是读类工具 ⇒ 写计数应为 0，实际 %d/%v", r1.WriteCallsThisTurn, r1.WroteEver)
	}
	// 第 2 轮（零工具收尾）必须**只有一条**轮级记录且调用数归零
	if r2 := behaviorRoundOf(t, 2); r2.ToolCallsThisTurn != 0 {
		t.Errorf("a) 第 2 轮调用数应为 0，实际 %d", r2.ToolCallsThisTurn)
	}
}

// ⑤ 负控：调了写工具但**执行失败** ⇒ 不算写过（仍是零写成功）；轮级与会话级必须一致
func TestBehaviorWireFailedWriteNotCounted(t *testing.T) {
	t.Setenv("ZERG_STATE_DIR", t.TempDir())
	const sess = "sess-T13w-failwrite"

	res := loopcore.Run(context.Background(), loopcore.Config{MaxRounds: 4}, "m", "sys", fbMsg(), loopcore.Deps{
		Infer: fbInfer([]*loopcore.Response{
			{ToolCalls: []loopcore.ToolCall{fbCall("1", "write")}}, // r1：写——失败
			{Content: "写不了，我直接给你结论。"},                              // r2：交差
		}),
		Exec:    fbExec(map[string]bool{"write": true}, nil),
		Session: sess,
	})
	if res.ExitKind != "natural" {
		t.Fatalf("应正常收尾，实际 exit=%q", res.ExitKind)
	}

	r1 := behaviorRoundOf(t, 1)
	if r1.ToolCallsThisTurn != 1 {
		t.Errorf("a) 模型确实调了一次（失败也要记调用），实际 %d", r1.ToolCallsThisTurn)
	}
	if r1.WriteCallsThisTurn != 0 || r1.WroteEver {
		t.Errorf("写失败 ⇒ 不计写成功（否则「没写」会被洗白），实际 %d/%v", r1.WriteCallsThisTurn, r1.WroteEver)
	}
	if !behaviorSessionOf(t).EarlyExitSuspected {
		t.Error("调了写工具但失败 ⇒ 仍是「零写类工具成功」+ 已交答案 ⇒ 应判 early_exit_suspected=true")
	}
}

// ⑥ 负控（反例二）：轮数用尽、**未给出终答** ⇒ 不许判早退（超时/耗尽 ≠ 早退交差）
func TestBehaviorWireNoFinalAnswerNotFlagged(t *testing.T) {
	t.Setenv("ZERG_STATE_DIR", t.TempDir())
	const sess = "sess-T13w-maxrounds"

	res := loopcore.Run(context.Background(), loopcore.Config{MaxRounds: 2}, "m", "sys", fbMsg(), loopcore.Deps{
		Infer: fbInfer([]*loopcore.Response{
			{ToolCalls: []loopcore.ToolCall{fbCall("1", "read")}}, // 一直只读，从不停手
		}),
		Exec:    fbExec(nil, nil),
		Session: sess,
	})
	if res.ExitKind != "max_rounds" {
		t.Fatalf("本用例应轮数用尽退出，实际 exit=%q", res.ExitKind)
	}

	sum := behaviorSessionOf(t)
	if sum.FinalAnswer {
		t.Error("轮数用尽不是「已给出终答」⇒ final_answer 必须为 false")
	}
	if sum.EarlyExitSuspected {
		t.Error("**反例**：没交答案就退出的不是早退（是没干完）⇒ early_exit_suspected 必须为 false")
	}
	if sum.WroteEver || sum.TotalToolCalls != 2 {
		t.Errorf("会话级事实：wrote=%v total=%d（应 false/2）", sum.WroteEver, sum.TotalToolCalls)
	}
	if sum.ExitKind != "max_rounds" || !sum.Tracked {
		t.Errorf("退出原因/追踪标记不符：exit=%q tracked=%v", sum.ExitKind, sum.Tracked)
	}
}
