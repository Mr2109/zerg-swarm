// slice_events_test.go — B 项⑤ 接线面用例（③ 第 4 态进态 / 超时）。
//
// 取证纪律（与 fourth_state_test.go 同一套）：`now` 一律**注入**（不看系统时间）；
// 观测面=**真读回落盘的那一行**（不 mock 判定）。每条都带对侧断言：
// 既断「升级类结局必须落事件」，也断「非升级结局**不得**落事件」（只断前者的用例会被
// 「什么都落」伪证）。
//
// 覆盖：
//  1. 进态（追问/阻滞）⇒ 恰一行 `slice_escalated`（outcome=raised，带原因码 + R 编号 + 判据版本）
//  2. 未答结算（awaiting）⇒ **不落**（不是升级）
//  3. 到点超时结算（fail closed ⇒ 升级给人）⇒ 恰一行 `slice_escalated`（outcome=timeout）
//  4. 上游补规格（supplied ⇒ 放行）⇒ **不落**（不是升级）
package policy

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/sliceobs"
)

// readSliceObsEvents — 真读回片观测事件（文件不存在 ⇒ nil）。
func readSliceObsEvents(t *testing.T) []sliceobs.Event {
	t.Helper()
	data, err := os.ReadFile(sliceobs.EventsFile())
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("读片观测事件失败: %v", err)
	}
	var out []sliceobs.Event
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var ev sliceobs.Event
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			t.Fatalf("片观测事件不是合法 JSON 行: %q (%v)", line, err)
		}
		out = append(out, ev)
	}
	return out
}

// resetSliceObs — 每次用例开跑前清掉落点（TestMain 已把状态目录钉在临时目录里 ⇒ 清的是临时文件）。
func resetSliceObs(t *testing.T) {
	t.Helper()
	if err := os.Remove(sliceobs.EventsFile()); err != nil && !os.IsNotExist(err) {
		t.Fatalf("清理片观测落点失败：%v", err)
	}
}

// ── 1. 进态 ⇒ slice_escalated(raised) ──────────────────────────────────────

func TestSliceEvents_EscalatedOnFourthStateRaise(t *testing.T) {
	resetSliceObs(t)
	now := pendingT0()
	s := NewMemoryPendingStore()
	out, err := RaiseClarification(s, fourthReq(t), now)
	if err != nil {
		t.Fatalf("进第 4 态失败：%v", err)
	}
	if out.AllowedToProceed() {
		t.Fatal("进态本身不放行任何东西（片停 awaiting）")
	}

	evs := readSliceObsEvents(t)
	if len(evs) != 1 {
		t.Fatalf("进态应恰有 1 条事件，实际 %d 条：%+v", len(evs), evs)
	}
	ev := evs[0]
	if ev.Event != "slice_escalated" { // 字面断言（防事件名漂移）
		t.Fatalf("进态的事件名应为 slice_escalated（设计稿 §6.1），实际 %q", ev.Event)
	}
	if ev.OK || ev.Outcome != "raised" {
		t.Fatalf("进态事件的 ok/outcome 应为 false/raised，实际 %v/%q", ev.OK, ev.Outcome)
	}
	if ev.SliceID != "S3-C1" {
		t.Fatalf("片归因不符：%q", ev.SliceID)
	}
	if ev.Code != "PRE_R5" || ev.R == nil || *ev.R != 5 {
		t.Fatalf("原因码/R 编号应如实落 PRE_R5 / 5，实际 code=%q r=%v", ev.Code, ev.R)
	}
	if ev.CriteriaVersion != "未标定" {
		t.Fatalf("判据版本无来源 ⇒ 必须写「未标定」（不得编造），实际 %q", ev.CriteriaVersion)
	}
	if !strings.Contains(ev.Detail, "补规格") || !strings.Contains(ev.Detail, "不放行") {
		t.Fatalf("detail 必须可行动（点明退回补规格 + 不放行）：%q", ev.Detail)
	}
	t.Logf("进态事件 = %+v", ev)

	// ★ 对侧：未答结算（awaiting）**不是升级** ⇒ 不得再落一行
	if got, err := SettleClarification(s, out.PendingID, pendingAt(1*time.Minute)); err != nil {
		t.Fatalf("未答结算不应报错：%v", err)
	} else if got.Disposition != FourthAwaiting {
		t.Fatalf("未到点应为 awaiting，实际 %s", string(got.Disposition))
	}
	if evs := readSliceObsEvents(t); len(evs) != 1 {
		t.Fatalf("awaiting 不是升级 ⇒ 不得落事件（否则升级流被稀释），实际 %d 条：%+v", len(evs), evs)
	}
}

// ── 2. 到点超时 ⇒ slice_escalated(timeout) ─────────────────────────────────

func TestSliceEvents_EscalatedOnFourthStateTimeout(t *testing.T) {
	resetSliceObs(t)
	now := pendingT0()
	s := NewMemoryPendingStore()
	raised, err := RaiseClarification(s, fourthReq(t), now)
	if err != nil {
		t.Fatalf("进第 4 态失败：%v", err)
	}
	// 恰等于到期时刻即算超时（沿用 pending.go 的边界取严）
	out, err := SettleClarification(s, raised.PendingID, pendingAt(10*time.Minute))
	if err != nil {
		t.Fatalf("到点结算不应报错：%v", err)
	}
	if out.Disposition != FourthVoid || out.AllowedToProceed() {
		t.Fatalf("超时必须作废且不得放行，得到 %s / allowed=%v", string(out.Disposition), out.Allowed)
	}

	evs := readSliceObsEvents(t)
	if len(evs) != 2 {
		t.Fatalf("应为「进态 1 行 + 超时 1 行」= 2 行，实际 %d 条：%+v", len(evs), evs)
	}
	last := evs[len(evs)-1]
	if last.Event != "slice_escalated" {
		t.Fatalf("超时事件名应为 slice_escalated，实际 %q", last.Event)
	}
	if last.OK || last.Outcome != "timeout" {
		t.Fatalf("超时事件的 ok/outcome 应为 false/timeout，实际 %v/%q", last.OK, last.Outcome)
	}
	if last.SliceID != "S3-C1" || last.Code != "PRE_R5" || last.R == nil || *last.R != 5 {
		t.Fatalf("超时事件的归因不符：slice=%q code=%q r=%v", last.SliceID, last.Code, last.R)
	}
	if last.CriteriaVersion != "未标定" {
		t.Fatalf("判据版本应「未标定」，实际 %q", last.CriteriaVersion)
	}
	if !strings.Contains(last.Detail, "不得自行解除阻滞") || !strings.Contains(last.Detail, "升级给人") {
		t.Fatalf("超时 detail 必须可行动（点明 fail closed + 升级给人）：%q", last.Detail)
	}
	t.Logf("超时事件 = %+v", last)
}

// ── 3. 上游补规格（supplied ⇒ 放行）⇒ 不落事件 ─────────────────────────────

func TestSliceEvents_NoEventWhenUpstreamSupplies(t *testing.T) {
	resetSliceObs(t)
	now := pendingT0()
	s := NewMemoryPendingStore()
	raised, err := RaiseClarification(s, fourthReq(t), now)
	if err != nil {
		t.Fatalf("进第 4 态失败：%v", err)
	}
	if _, err := s.Answer(raised.PendingID, AnswerApprove, pendingAt(1*time.Minute)); err != nil {
		t.Fatalf("补规格答复应被接受：%v", err)
	}
	out, err := SettleClarification(s, raised.PendingID, pendingAt(2*time.Minute))
	if err != nil {
		t.Fatalf("结算不应报错：%v", err)
	}
	if out.Disposition != FourthSupplied || !out.AllowedToProceed() {
		t.Fatalf("上游补了规格 ⇒ supplied 且放行，得到 %s / allowed=%v", string(out.Disposition), out.Allowed)
	}
	// 进态那 1 行仍在，但结算**不得**再落（supplied 是解除阻滞，不是升级）
	if evs := readSliceObsEvents(t); len(evs) != 1 || evs[0].Outcome != "raised" {
		t.Fatalf("supplied 不是升级 ⇒ 结算不得落事件，实际 %+v", evs)
	}
}
