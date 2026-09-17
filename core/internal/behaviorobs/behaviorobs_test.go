// behaviorobs_test.go — T1.3 叶包实证：行为反模式信号**算得对**（纯累加、无 IO、无副作用）。
//
// 为什么叶包要有自己的用例（而不是只在 chat 侧断 JSONL）：计数器是**纯函数级**的事实，
// 在这里能直接构造"零工具调用 / 有读无写 / 写成功 / 单轮多工具"四种会话形态，
// 把"轮与轮之间的累加规则"钉死；chat 侧用例负责证明这些数**真的落进了 chat_obs.jsonl**。
//
// 纪律：本包不碰文件系统、不 import chat（叶子包）；每个用例用**独立会话名**（累加器按会话隔离）。
package behaviorobs

import (
	"testing"
)

// capture — 把落点换成"收到就存"的探针（不改生产语义；用例结束还原为空落点）
type capture struct {
	rounds   []Snapshot
	sessions []Summary
}

func (c *capture) install(t *testing.T) {
	t.Helper()
	SetSink(Sink{
		Round:   func(s Snapshot) { c.rounds = append(c.rounds, s) },
		Session: func(s Summary) { c.sessions = append(c.sessions, s) },
	})
	t.Cleanup(func() { SetSink(Sink{}) })
}

// ① 写类工具白名单是**闭集**：读类工具绝不能被误判成写（这是防误报的第一道闸）
func TestIsWriteToolClosedSet(t *testing.T) {
	for _, name := range []string{"write", "edit", "apply_patch", "copy_file", "move_file", "delete_file"} {
		if !IsWriteTool(name) {
			t.Errorf("%q 是写类工具，应被判为 true（否则「写过」会被读成「没写」）", name)
		}
	}
	// 反例（防误报）：读类/查询类工具一律 false。**bash 必须在里面** —— 它既能读也能写，
	// 本层无法判别 ⇒ 宁可不算（信号取「写」的下界），绝不能让 bash 把「零写会话」洗白。
	for _, name := range []string{"read", "ls", "glob", "grep", "bash", "web_search", "web_fetch", "memory", "tool_search", "", "not_a_tool"} {
		if IsWriteTool(name) {
			t.Errorf("%q 不是写类工具，应被判为 false（否则零写会话会被误判成「干过活」）", name)
		}
	}
}

// ② 零工具调用的会话（d 的显式标记）：连续零调用轮数递增；first_tool_call_seen 恒 false；
// turns_before_first_tool_call 记「至今 N 轮仍无调用」（**非空**）；收尾结论 early_exit_suspected=true。
func TestZeroToolCallSessionExplicitMarker(t *testing.T) {
	var c capture
	c.install(t)
	const sess = "sess-T13-zero"

	EmitRound(Fact{Session: sess, Round: 1})
	EmitRound(Fact{Session: sess, Round: 2})
	EmitRound(Fact{Session: sess, Round: 3})

	if len(c.rounds) != 3 {
		t.Fatalf("应落 3 条轮级快照，实际 %d", len(c.rounds))
	}
	last := c.rounds[2]
	if last.ToolCallsThisTurn != 0 || last.TotalToolCalls != 0 {
		t.Errorf("零工具调用会话的调用数应为 0：this=%d total=%d", last.ToolCallsThisTurn, last.TotalToolCalls)
	}
	if last.TurnsWithoutToolCall != 3 {
		t.Errorf("连续零工具调用轮数应为 3，实际 %d", last.TurnsWithoutToolCall)
	}
	if last.FirstToolCallSeen {
		t.Error("整会话零工具调用 ⇒ first_tool_call_seen 必须为 false（d 的显式标记）")
	}
	if last.TurnsBeforeFirstToolCall != 3 {
		t.Errorf("零调用时应记「至今 3 轮无调用」（非空且非编造），实际 %d", last.TurnsBeforeFirstToolCall)
	}
	if last.WroteEver {
		t.Error("零工具调用的会话不可能写过东西")
	}

	EmitSession(SessionFact{Session: sess, Rounds: 3, ExitKind: "natural", FinalAnswer: true})
	if len(c.sessions) != 1 {
		t.Fatalf("应收 1 条会话结论，实际 %d", len(c.sessions))
	}
	sum := c.sessions[0]
	if !sum.Tracked {
		t.Error("本包收到过轮次事实 ⇒ tracked 应为 true")
	}
	if !sum.EarlyExitSuspected {
		t.Error("整会话零写类工具成功 且 已给出终答 ⇒ early_exit_suspected 必须为 true")
	}
	if sum.FirstToolCallSeen || sum.TurnsBeforeFirstToolCall != 3 {
		t.Errorf("会话级也要带 d 的显式标记：seen=%v before=%d", sum.FirstToolCallSeen, sum.TurnsBeforeFirstToolCall)
	}
	if sum.TotalToolCalls != 0 || sum.WroteEver {
		t.Errorf("会话级计数应为 0/未写：total=%d wrote=%v", sum.TotalToolCalls, sum.WroteEver)
	}
}

// ③ 有读无写的会话（c 递增）：每轮 +1，且严格递增
func TestReadOnlySessionWithoutWriteIncrements(t *testing.T) {
	var c capture
	c.install(t)
	const sess = "sess-T13-readonly"

	EmitRound(Fact{Session: sess, Round: 1, ToolCalls: 0})
	EmitRound(Fact{Session: sess, Round: 2, ToolCalls: 1}) // read
	EmitRound(Fact{Session: sess, Round: 3, ToolCalls: 2}) // read + grep

	prev := -1
	for i, snap := range c.rounds {
		wantSince := i + 1 // 1,2,3
		if snap.TurnsSinceLastWrite != wantSince {
			t.Errorf("第 %d 轮 turns_since_last_write 应为 %d，实际 %d（有读无写 ⇒ 递增）", i+1, wantSince, snap.TurnsSinceLastWrite)
		}
		if snap.TurnsSinceLastWrite <= prev {
			t.Errorf("第 %d 轮未递增：%d <= %d", i+1, snap.TurnsSinceLastWrite, prev)
		}
		prev = snap.TurnsSinceLastWrite
		if snap.WroteEver {
			t.Errorf("第 %d 轮不该有 wrote_ever=true（本轮与之前都没有写成功）", i+1)
		}
	}
	if c.rounds[0].TurnsWithoutToolCall != 1 {
		t.Errorf("第 1 轮零调用 ⇒ 连续零调用轮数 1，实际 %d", c.rounds[0].TurnsWithoutToolCall)
	}
	if c.rounds[1].TurnsWithoutToolCall != 0 {
		t.Errorf("第 2 轮有调用 ⇒ 连续零调用轮数归零，实际 %d", c.rounds[1].TurnsWithoutToolCall)
	}
	if c.rounds[1].TurnsBeforeFirstToolCall != 1 {
		t.Errorf("首次调用出现在第 2 轮 ⇒ 之前的轮数应为 1，实际 %d", c.rounds[1].TurnsBeforeFirstToolCall)
	}
	if c.rounds[2].TurnsBeforeFirstToolCall != 1 {
		t.Errorf("d 记的是「首次之前的轮数」⇒ 定住不涨，实际 %d", c.rounds[2].TurnsBeforeFirstToolCall)
	}
	if c.rounds[2].ToolCallsThisTurn != 2 {
		t.Errorf("a) 本轮调用数应为 2，实际 %d", c.rounds[2].ToolCallsThisTurn)
	}
}

// ④ 正常写类会话（反例：防误报）——写成功即归零，会话结论必须为**假**
func TestWriteSessionResetsCounterAndNotSuspected(t *testing.T) {
	var c capture
	c.install(t)
	const sess = "sess-T13-write"

	EmitRound(Fact{Session: sess, Round: 1, ToolCalls: 2})
	EmitRound(Fact{Session: sess, Round: 2, ToolCalls: 3, WriteOK: 1})

	snap := c.rounds[1]
	if snap.TurnsSinceLastWrite != 0 {
		t.Errorf("本轮有写成功 ⇒ 距上次写应为 0，实际 %d", snap.TurnsSinceLastWrite)
	}
	if !snap.WroteEver || snap.WriteCallsThisTurn != 1 {
		t.Errorf("写成功应记 wrote_ever=true/write_calls_this_turn=1，实际 %v/%d", snap.WroteEver, snap.WriteCallsThisTurn)
	}
	if snap.TotalWriteOK != 1 {
		t.Errorf("累计写成功应为 1，实际 %d", snap.TotalWriteOK)
	}

	EmitSession(SessionFact{Session: sess, Rounds: 2, ExitKind: "natural", FinalAnswer: true})
	if c.sessions[0].EarlyExitSuspected {
		t.Error("**反例**：本会话写过东西 ⇒ early_exit_suspected 必须为 false（否则真干活也会被误报）")
	}
}

// ⑤ 没交答案就不下结论（反例：超时/轮数用尽 ≠ 早退）+ 没有轮次事实就不编造
func TestNoFinalAnswerOrNoRoundsNotConcluded(t *testing.T) {
	var c capture
	c.install(t)
	const sess = "sess-T13-nofinal"

	EmitRound(Fact{Session: sess, Round: 1, ToolCalls: 1}) // 只读
	EmitSession(SessionFact{Session: sess, Rounds: 1, ExitKind: "max_rounds", FinalAnswer: false})
	if len(c.sessions) != 1 {
		t.Fatalf("应收 1 条会话结论，实际 %d", len(c.sessions))
	}
	if c.sessions[0].EarlyExitSuspected {
		t.Error("**反例**：轮数用尽（未给出终答）不是早退 ⇒ early_exit_suspected 必须为 false")
	}

	// 会话从未上报过任何轮次事实 ⇒ tracked=false 且**不下结论**（0 与「未知」可分）
	EmitSession(SessionFact{Session: "sess-T13-never-seen", Rounds: 0, ExitKind: "natural", FinalAnswer: true})
	last := c.sessions[len(c.sessions)-1]
	if last.Tracked {
		t.Error("从未收到轮次事实 ⇒ tracked 必须为 false")
	}
	if last.EarlyExitSuspected {
		t.Error("没有轮次事实就不许下结论（不编造）")
	}
}

// ⑥ 无会话 ⇒ 忽略（行为信号按会话归因才有意义）+ 收尾后计数器回收（表有界）
func TestEmptySessionIgnoredAndStateRecycled(t *testing.T) {
	var c capture
	c.install(t)

	EmitRound(Fact{Session: "", Round: 1, ToolCalls: 5})
	EmitSession(SessionFact{Session: "", Rounds: 1, ExitKind: "natural", FinalAnswer: true})
	if len(c.rounds) != 0 || len(c.sessions) != 0 {
		t.Fatalf("无会话应被忽略，实际 rounds=%d sessions=%d", len(c.rounds), len(c.sessions))
	}

	const sess = "sess-T13-recycle"
	EmitRound(Fact{Session: sess, Round: 1, ToolCalls: 0})
	EmitSession(SessionFact{Session: sess, Rounds: 1, ExitKind: "natural", FinalAnswer: true})
	// 同会话的「下一次 Run」必须**从零**开始计数（各自判定，不是继承上次的轮数）
	EmitRound(Fact{Session: sess, Round: 1, ToolCalls: 0})
	got := c.rounds[len(c.rounds)-1]
	if got.TurnsWithoutToolCall != 1 || got.TurnsSinceLastWrite != 1 || got.TotalToolCalls != 0 {
		t.Errorf("会话收尾后计数应回收重来，实际 %+v", got)
	}
}

// ⑦ sink 出岔子（panic）绝不能被带回循环/工具路径（best-effort 纪律）
func TestSinkPanicDoesNotEscape(t *testing.T) {
	SetSink(Sink{
		Round:   func(Snapshot) { panic("探针故意炸") },
		Session: func(Summary) { panic("探针故意炸") },
	})
	t.Cleanup(func() { SetSink(Sink{}) })

	EmitRound(Fact{Session: "sess-T13-panic", Round: 1, ToolCalls: 1})                // 不得 panic
	EmitSession(SessionFact{Session: "sess-T13-panic", Rounds: 1, FinalAnswer: true}) // 不得 panic
}

// ⑧ 写失败（WriteOK=0）不得把会话洗白：调了写工具但失败了 ⇒ 仍是「零写类工具成功」
func TestFailedWriteDoesNotMarkWrote(t *testing.T) {
	var c capture
	c.install(t)
	const sess = "sess-T13-failwrite"

	EmitRound(Fact{Session: sess, Round: 1, ToolCalls: 1, WriteOK: 0})
	EmitSession(SessionFact{Session: sess, Rounds: 1, ExitKind: "natural", FinalAnswer: true})

	if c.rounds[0].WroteEver {
		t.Error("写失败不算写过（wrote_ever 必须为 false）")
	}
	if !c.sessions[0].EarlyExitSuspected {
		t.Error("调了写工具但失败 ⇒ 仍是「零写类工具成功」，应判 early_exit_suspected=true")
	}
}
