// slice_events_test.go — B 项⑤ 接线面用例（① 挂板放行/拒绝）与「观测写失败不影响判定」。
//
// 与 slice_schema_test.go 同一套取证纪律：判定看**队列长度**（maxConcurrent=0 ⇒ 入队只排队、
// 不起进程），观测看**真读回落盘的那一行**（不 mock 判定、不看返回值）。
//
// 覆盖：
//  1. 挂板放行 ⇒ 恰一行 `slice_created`（+ 旧名归并进 alias），r 缺席
//  2. 挂板拒绝 ⇒ 恰一行 `slice_rejected`（带低基数原因码 + 可行动 detail），r 缺席
//  3. ★ 观测写失败 ⇒ **判定逐条不变**（该拒的仍拒、该放的仍放），且不凭空造文件、不 panic
package api

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── 1. 挂板放行 ⇒ slice_created ─────────────────────────────────────────────

func TestSliceEvents_CreatedWhenMountAdmitted(t *testing.T) {
	s := newSliceTestScheduler(t)
	acc := []string{"go test ./internal/api/ -count=1"}
	if err := s.SubmitSlice(&Task{ID: "t-c1", SliceID: "S1", Acceptance: &acc}); err != nil {
		t.Fatalf("合法片应放行入队，实际被拒：%v", err)
	}
	if got := s.QueueLen(); got != 1 {
		t.Fatalf("放行的片应入队（队列 1），实际 %d", got)
	}
	evs := readSliceMountEvents(t)
	if len(evs) != 1 {
		t.Fatalf("放行应恰有 1 条事件，实际 %d 条：%+v", len(evs), evs)
	}
	ev := evs[0]
	// 事件名写**字面**（防常量被改坏后自证）
	if ev.Event != "slice_created" {
		t.Fatalf("放行的事件名应为 slice_created（设计稿 §6.1），实际 %q", ev.Event)
	}
	if ev.Alias != "slice_mount" {
		t.Fatalf("旧名 slice_mount 应归并进 alias（历史读取方仍认得出），实际 %q", ev.Alias)
	}
	if !ev.OK || ev.Outcome != "created" {
		t.Fatalf("放行事件的 ok/outcome 应为 true/created，实际 %v/%q", ev.OK, ev.Outcome)
	}
	if ev.SliceID != "S1" || ev.TaskID != "t-c1" {
		t.Fatalf("片/任务归因不符：%+v", ev)
	}
	if ev.CriteriaVersion != "未标定" {
		t.Fatalf("判据版本无来源 ⇒ 必须写「未标定」（不得编造），实际 %q", ev.CriteriaVersion)
	}
	if ev.R != nil {
		t.Fatalf("挂板校验用 SLICE_* 码、没有 R 编号 ⇒ r 必须缺席，实际 %d", *ev.R)
	}
	if ev.Detail == "" {
		t.Fatal("每条事件都要带可行动 detail")
	}
	t.Logf("放行事件 = %+v", ev)
}

// ── 2. 挂板拒绝 ⇒ slice_rejected ────────────────────────────────────────────

func TestSliceEvents_RejectedWhenMountDenied(t *testing.T) {
	empty := []string{}
	s := newSliceTestScheduler(t)
	// 板上为空 ⇒ depends_on 里的 S1 悬空 ⇒ 拒
	s.Submit(&Task{ID: "t-r1", SliceID: "S3", DependsOn: []string{"S1"}, Acceptance: &empty})
	if got := s.QueueLen(); got != 0 {
		t.Fatalf("悬空依赖的片必须拒绝入队（队列 0），实际 %d", got)
	}
	evs := readSliceMountEvents(t)
	if len(evs) != 1 {
		t.Fatalf("拒绝应恰有 1 条事件，实际 %d 条：%+v", len(evs), evs)
	}
	ev := evs[0]
	if ev.Event != "slice_rejected" {
		t.Fatalf("拒绝的事件名应为 slice_rejected（设计稿 §6.1），实际 %q", ev.Event)
	}
	if ev.Alias != "slice_mount" {
		t.Fatalf("拒绝事件同样要带归并别名，实际 %q", ev.Alias)
	}
	if ev.OK || ev.Outcome != "rejected" {
		t.Fatalf("拒绝事件的 ok/outcome 应为 false/rejected，实际 %v/%q", ev.OK, ev.Outcome)
	}
	if ev.Code != SliceErrDependsDangling || ev.Field != "depends_on" {
		t.Fatalf("低基数原因码/字段不符：code=%q field=%q", ev.Code, ev.Field)
	}
	if ev.CriteriaVersion != "未标定" {
		t.Fatalf("判据版本应「未标定」，实际 %q", ev.CriteriaVersion)
	}
	if ev.R != nil {
		t.Fatalf("挂板拒绝没有 R 编号 ⇒ r 必须缺席，实际 %d", *ev.R)
	}
	if !strings.Contains(ev.Detail, "怎么改") {
		t.Fatalf("拒绝 detail 必须可行动（含「怎么改」）：%q", ev.Detail)
	}
	t.Logf("拒绝事件 = %+v", ev)
}

// ── 3. ★ 观测写失败 ⇒ 判定逐条不变 ─────────────────────────────────────────
//
// 手法：把 ZERG_STATE_DIR 指到「一个普通文件」下面 —— `os.MkdirAll(父目录)` 必失败（ENOTDIR），
// 于是每一次 Emit 都在第一道 IO 上失败。此时：
//
//	· 该拒的仍拒（队列 0）、该放的仍放（队列 1）—— 判定与 `serr` 逐字节不变；
//	· 不凭空造出事件文件、不 panic、不把错误往外传（Emit 无返回值）。
func TestSliceEvents_FailSoftWhenSinkUnwritable(t *testing.T) {
	s := newSliceTestScheduler(t) // 已隔离任务文件/任务目录；下面再把状态目录改成写不进去的
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("not-a-dir"), 0o644); err != nil {
		t.Fatalf("造堵塞文件失败：%v", err)
	}
	blockedState := filepath.Join(blocker, "state")
	t.Setenv("ZERG_STATE_DIR", blockedState)

	empty := []string{}
	// ① 拒绝路径：缺 slice_id（声明了 depends_on）⇒ 仍必须拒绝入队
	s.Submit(&Task{ID: "t-f1", DependsOn: []string{"S0"}, Acceptance: &empty})
	if got := s.QueueLen(); got != 0 {
		t.Fatalf("观测写失败时「拒绝」判定被改动 —— 队列应为 0，实际 %d", got)
	}
	// ② 放行路径：合法片 ⇒ 仍必须放行入队
	s.Submit(&Task{ID: "t-f2", SliceID: "S1", Acceptance: &empty})
	if got := s.QueueLen(); got != 1 {
		t.Fatalf("观测写失败时「放行」判定被改动 —— 队列应为 1，实际 %d", got)
	}
	// ③ 拒绝路径（悬空）⇒ 仍必须拒绝，且仍带着可行动错误码
	if err := s.SubmitSlice(&Task{ID: "t-f3", SliceID: "S9", DependsOn: []string{"NOPE"}, Acceptance: &empty}); err == nil {
		t.Fatal("观测写失败时悬空依赖不再被拒 —— 判定被观测改动了")
	} else if serr, ok := err.(*SliceValidationError); !ok || serr.Code != SliceErrDependsDangling {
		t.Fatalf("错误码不该受影响，实际 %T %v", err, err)
	}
	if got := s.QueueLen(); got != 1 {
		t.Fatalf("三次提交后队列应仍为 1，实际 %d", got)
	}
	// ④ 观测面：确实一行都没落（写不进去），也没凭空造出文件
	//    注意：父路径是普通文件 ⇒ stat 报 ENOTDIR（不是 ENOENT）——「stat 不成功」就是「没造出来」的判据。
	if _, err := os.Stat(filepath.Join(blockedState, "slice-mount-events.jsonl")); err == nil {
		t.Fatal("落点写不进去时不得凭空造出事件文件")
	}
	t.Logf("观测写失败：队列=%d（判定不变）· 落点=%s（未创建）", s.QueueLen(), SliceMountEventsFile())
}
