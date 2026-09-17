// slice_events_test.go — B 项⑤ 接线面用例（② 打回带原因码时）。
//
// 取证纪律（与 sendback_code_test.go 同一套）：裁决结果由**真实的**四态裁决算出来
// （`VerifyClaims`，一行没改），不手搓假结果；观测面=**真读回落盘的那一行**。
//
// 覆盖：
//  1. 成功打回 ⇒ 恰一行 `slice_rejected`（带原因码 + R 编号 + 判据版本 + 可行动 detail）
//  2. 被拒的打回（缺码/无裁决/无 send_back）⇒ **一行都不落**（没有判定就没有结局）
//  3. 观测写失败 ⇒ SendBack 的返回值/错误**逐字节不变**（best-effort 自证）
package loopcore

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/policy"
	"github.com/Mr2109/zerg-swarm/core/internal/sliceobs"
)

// readSliceObsEvents — 真读回片观测事件（不 mock；文件不存在 ⇒ nil）。
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

// ── 1. 成功打回 ⇒ slice_rejected ────────────────────────────────────────────

func TestSliceEvents_RejectedOnSendBackWithReasonCode(t *testing.T) {
	// 落点是**包级共享**（TestMain 钉一个临时目录）⇒ 用「增量」取证，不假设文件是空的
	// （同包的既有 sendback_code_test.go 用例也会经 SendBack 落行）。
	before := len(readSliceObsEvents(t))
	req := SendBackRequest{
		SliceID:  "S9-C1",
		Code:     codeFor(t, policy.RespPreSlice, 3, "补齐 criteria 三件套：命令 + 期望输出 + 当前红/绿"),
		Detail:   "自由文本说明（**不得**进观测面）",
		Verdicts: realVerdicts(),
	}
	rec, err := SendBack(req)
	if err != nil {
		t.Fatalf("带原因码的打回应成功：%v", err)
	}
	if rec.Code != "PRE_R3" {
		t.Fatalf("记录里的主原因码应为 PRE_R3，实际 %q", rec.Code)
	}

	evs := readSliceObsEvents(t)
	if len(evs) != before+1 {
		t.Fatalf("一次成功打回应恰多 1 条事件（%d ⇒ %d），实际共 %d 条", before, before+1, len(evs))
	}
	ev := evs[len(evs)-1]
	if ev.Event != "slice_rejected" { // 字面断言（防事件名漂移）
		t.Fatalf("打回的事件名应为 slice_rejected（设计稿 §6.1），实际 %q", ev.Event)
	}
	if ev.OK || ev.Outcome != "rejected" {
		t.Fatalf("打回事件的 ok/outcome 应为 false/rejected，实际 %v/%q", ev.OK, ev.Outcome)
	}
	if ev.SliceID != "S9-C1" {
		t.Fatalf("片归因不符：%q", ev.SliceID)
	}
	if ev.Code != "PRE_R3" {
		t.Fatalf("原因码应如实落 PRE_R3，实际 %q", ev.Code)
	}
	if ev.R == nil || *ev.R != 3 {
		t.Fatalf("R 编号应如实落 3（打回才有的那一位），实际 %v", ev.R)
	}
	if ev.CriteriaVersion != "未标定" {
		t.Fatalf("判据版本无来源 ⇒ 必须写「未标定」（不得编造），实际 %q", ev.CriteriaVersion)
	}
	if !strings.Contains(ev.Detail, "PRE_R3") || !strings.Contains(ev.Detail, "重交") {
		t.Fatalf("detail 必须可行动（点明码 + 下一步重交）：%q", ev.Detail)
	}
	// 硬规则 ④：不回显调用方给的自由文本
	if strings.Contains(ev.Detail, "自由文本说明") {
		t.Fatalf("观测事件不得回显打回说明原文：%q", ev.Detail)
	}
	t.Logf("打回事件 = %+v", ev)
}

// ── 2. 被拒的打回 ⇒ 一行都不落（判定与观测同向 fail-closed）────────────────

func TestSliceEvents_NoEventWhenSendBackRejected(t *testing.T) {
	before := len(readSliceObsEvents(t))
	// ① 缺原因码 ⇒ 拒绝发回
	if _, err := SendBack(SendBackRequest{SliceID: "S9-C2", Verdicts: realVerdicts()}); err == nil {
		t.Fatal("缺原因码必须拒绝发回")
	}
	// ② 没有裁决 ⇒ 拒绝发回
	if _, err := SendBack(SendBackRequest{
		SliceID: "S9-C3",
		Code:    codeFor(t, policy.RespPostExec, 10, "贴出回执原文片段"),
	}); err == nil {
		t.Fatal("没有裁决必须拒绝发回")
	}
	// ③ 缺 slice_id ⇒ 拒绝发回（沿用铁律：无 id 的委派一律拒绝）
	if _, err := SendBack(SendBackRequest{
		Code:     codeFor(t, policy.RespInvEnv, 13, "重启工具链后重跑"),
		Verdicts: realVerdicts(),
	}); err == nil {
		t.Fatal("缺 slice_id 必须拒绝发回")
	}
	if evs := readSliceObsEvents(t); len(evs) != before {
		t.Fatalf("三次被拒的打回不得落任何事件（没有判定就没有结局）：应仍为 %d 条，实际 %d 条", before, len(evs))
	}
}

// ── 3. 观测写失败 ⇒ 打回判定逐字节不变 ──────────────────────────────────────

func TestSliceEvents_SendBackUnaffectedWhenSinkUnwritable(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("not-a-dir"), 0o644); err != nil {
		t.Fatalf("造堵塞文件失败：%v", err)
	}
	t.Setenv("ZERG_STATE_DIR", filepath.Join(blocker, "state")) // MkdirAll 必失败

	req := SendBackRequest{
		SliceID:  "S9-C4",
		Code:     codeFor(t, policy.RespPostExec, 10, "贴出回执原文片段"),
		Verdicts: realVerdicts(),
	}
	rec, err := SendBack(req)
	if err != nil {
		t.Fatalf("观测写失败不得让打回失败：%v", err)
	}
	if rec.Code != "POST_R10" || rec.Judge != 10 || rec.Responsibility != "POST" {
		t.Fatalf("记录不得被观测影响：%+v", rec)
	}
	if note, err := rec.Note(); err != nil || !strings.Contains(note, "【打回重做】") {
		t.Fatalf("打回指令不得被观测影响：%q / %v", note, err)
	}
	if _, err := os.Stat(filepath.Join(blocker, "state", "slice-mount-events.jsonl")); err == nil {
		t.Fatal("落点写不进去时不得凭空造文件（父路径是普通文件 ⇒ stat 必失败）")
	}
	t.Logf("观测写失败：打回仍成功 cod=%s judge=%d（判定不变）", rec.Code, rec.Judge)
}
