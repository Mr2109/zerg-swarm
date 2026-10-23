package api

// task_state_truth_test.go —— 缺口 `GAP-20260926-18` 第一步（只读投影）的判据。
//
// 三条牙：
//   〔甲〕词表映射：`failed ⇒ dead`（退出状态的词不是状态机的词）· `paused ⇒ queued` · 认不出的 ⇒ unknown（✗ 不猜）
//   〔乙〕产物面（方案 A）：`result.json`/报告/`work` 非空 ⇒ true；**只有 `audit.jsonl`/`logs` ⇒ false**
//        （那两件任务一开始就有，拿来判「做完了」会恒真 —— 本项目的永真判据红线）
//   〔丙〕矛盾面：`status=queued` 且产物已在盘 ⇒ `status_conflict=true` 且原因点名；
//        内存态 ⟷ 台账态不一致 ⇒ 同样报矛盾；两者都不成立 ⇒ 不报（✗ 不许一见任务就报）

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIssueStatusVocabulary(t *testing.T) {
	cases := map[string]string{
		"queued":        "queued",
		"done":          "done",
		"failed":        "dead",   // 退出状态的词 ⇒ 终态
		"cancelled":     "dead",   // 取消也是终态
		"paused":        "queued", // 暂停 = 还没做完
		"waiting_retry": "retry",
		"":              "unknown",
		"胡说":            "unknown",
	}
	for in, want := range cases {
		if got := issueStatusOf(in); got != want {
			t.Errorf("issueStatusOf(%q) = %q，want %q", in, got, want)
		}
	}
	// 10 值原样回传（✗ 不要「顺便规范化」）
	for v := range issueVocab {
		if got := issueStatusOf(v); got != v {
			t.Errorf("词表内的 %q 被改写成 %q", v, got)
		}
	}
}

func TestTaskArtifactState(t *testing.T) {
	dir := t.TempDir()

	if ok, which := taskArtifactState(dir); ok {
		t.Fatalf("空目录必须无产物（实得 ok=%v which=%q）", ok, which)
	}

	// ★ 只有 audit.jsonl + logs/ ⇒ 仍然无产物（任务一开始就有）
	if err := os.WriteFile(filepath.Join(dir, "audit.jsonl"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "logs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "logs", "run.log"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if ok, which := taskArtifactState(dir); ok {
		t.Fatalf("只有 audit.jsonl/logs 不许算产物（实得 which=%q）—— 那会变成恒真判据", which)
	}

	// 报告在盘 ⇒ 有产物
	if err := os.WriteFile(filepath.Join(dir, "internal-task-report.md"), []byte("# done"), 0o644); err != nil {
		t.Fatal(err)
	}
	if ok, which := taskArtifactState(dir); !ok || which != "internal-task-report.md" {
		t.Fatalf("报告在盘必须算产物（实得 ok=%v which=%q）", ok, which)
	}

	// 换成 work/ 非空
	dir2 := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir2, "work"), 0o755); err != nil {
		t.Fatal(err)
	}
	if ok, _ := taskArtifactState(dir2); ok {
		t.Fatal("空 work/ 不许算产物")
	}
	if err := os.WriteFile(filepath.Join(dir2, "work", "out.txt"), []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}
	if ok, which := taskArtifactState(dir2); !ok || which != "work/" {
		t.Fatalf("work/ 非空必须算产物（实得 ok=%v which=%q）", ok, which)
	}
}

func TestAnnotateTaskStateTruth(t *testing.T) {
	stateDir := t.TempDir()
	taskRoot := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", stateDir)
	t.Setenv("ZERG_TASK_ROOT", taskRoot)

	// 台账：t1 = done，t2 = failed，t3 缺席
	ledger := map[string]any{
		"queue": []any{}, "running": []any{}, "waiting": []any{},
		"history": []any{
			map[string]any{"id": "t1", "status": "done"},
			map[string]any{"id": "t2", "status": "failed"},
		},
	}
	raw, _ := json.Marshal(ledger)
	if err := os.WriteFile(filepath.Join(stateDir, "zerg-tasks.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}

	// t1：内存态 queued + 产物已在盘 ⇒ 矛盾（缺口原文那一句）
	d1 := filepath.Join(taskRoot, "t1")
	if err := os.MkdirAll(d1, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d1, "result.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	// t2：内存态 failed（→dead）与台账 failed（→dead）一致，且无产物 ⇒ 不矛盾
	d2 := filepath.Join(taskRoot, "t2")
	if err := os.MkdirAll(d2, 0o755); err != nil {
		t.Fatal(err)
	}
	// t3：内存态 queued、台账缺席、无产物 ⇒ 不矛盾
	d3 := filepath.Join(taskRoot, "t3")
	if err := os.MkdirAll(d3, 0o755); err != nil {
		t.Fatal(err)
	}

	tasks := []*Task{
		{ID: "t1", Status: "queued"},
		{ID: "t2", Status: "failed"},
		{ID: "t3", Status: "queued"},
	}
	annotateTaskStateTruth(tasks)

	if !tasks[0].ArtifactPresent || !tasks[0].StatusConflict {
		t.Errorf("t1 必须报「queued 但产物已在盘」（artifact=%v conflict=%v reason=%q）",
			tasks[0].ArtifactPresent, tasks[0].StatusConflict, tasks[0].ConflictReason)
	}
	if !strings.Contains(tasks[0].ConflictReason, "queued") || !strings.Contains(tasks[0].ConflictReason, "result.json") {
		t.Errorf("t1 的原因要点名 status 与产物件（实得 %q）", tasks[0].ConflictReason)
	}
	if tasks[0].IssueStatus != "done" {
		t.Errorf("t1 的 issue_status 必须取台账（done），实得 %q", tasks[0].IssueStatus)
	}

	if tasks[1].StatusConflict {
		t.Errorf("t2 内存态与台账一致且有产物缺失，不该报矛盾（reason=%q）", tasks[1].ConflictReason)
	}
	if tasks[1].IssueStatus != "dead" {
		t.Errorf("t2 台账 failed 必须映射成 dead，实得 %q", tasks[1].IssueStatus)
	}

	if tasks[2].StatusConflict || tasks[2].ArtifactPresent {
		t.Errorf("t3 无台账无产物，不该报矛盾（conflict=%v artifact=%v）", tasks[2].StatusConflict, tasks[2].ArtifactPresent)
	}
}
