package api

// task_state_truth.go —— 任务态真源面（缺口 `GAP-20260926-18` 第一步 · 只读投影）
//
// 病（缺口原文）：`zerg task ls` 的 `queued` ≠ 未做 —— 队列态与产物不同步，取单会取到已完成件。
// 机制（账内两轮订正后的现读事实）：
//   · `task ls` 直接吐**内存态**（scheduler 的 queue/running/history），不与任何「产物」对账；
//   · 台账件（`zerg-tasks.json`）另记一套词：`queued/done/failed`，其中 **`failed` 不在状态机的
//     10 值里**（它是 `ExitStatus` 的值）⇒ 同一个词三处含义不同。
//
// 本步**只读**：给每条任务补三格（`issue_status` / `artifact_present` / `status_conflict`），
// **不动任何写路径**（12 处 `os.WriteFile` 的收敛是第二步，单独一批）。
//
// 拍板（Mr2109 2026-10-23）：产物定义 = 方案 A（调度器自己写下的东西）；真源 = **台账件**；
// 两步走（先投影、后收敛）。

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
)

// issueVocab —— 状态机的 10 个值（`core/internal/agent/issue_tracker.go` 的闭集）。
// 本文件只做**投影**（把别的词表映射进来），✗ 不自己定义第二套词表。
var issueVocab = map[string]bool{
	"open": true, "queued": true, "running": true, "fixing": true, "verified": true,
	"waiting_review": true, "done": true, "retry": true, "escalated": true, "dead": true,
}

// issueStatusOf —— 把任一来源的态映射到那 10 个值上；映射不了 ⇒ "unknown"（✗ 不猜）。
//
// 台账实测分布：`queued` 106 · `done` 23 · `failed` 20。`failed` 是**退出状态**的词，
// 在状态机里对应的终态是 `dead`（失败且不可重跑）—— 映射是**单向**的，只用于显示。
func issueStatusOf(s string) string {
	s = strings.TrimSpace(s)
	if issueVocab[s] {
		return s
	}
	switch s {
	case "failed":
		return "dead"
	case "cancelled":
		return "dead"
	case "paused":
		return "queued" // 暂停 = 还没做完 ⇒ 语义上仍在「排队」那一侧
	case "waiting_retry":
		return "retry"
	}
	return "unknown"
}

// ledgerStatusByID —— 台账件里的 id → status（真源；读不到 ⇒ nil，调用方照旧只出内存态）。
//
// ★ 只读：打开文件读一遍就关，✗ 不写、✗ 不加锁（台账的写面归 scheduler，本面不碰）。
func ledgerStatusByID() map[string]string {
	b, err := os.ReadFile(statepath.File(tasksStateFileName))
	if err != nil {
		return nil
	}
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil
	}
	out := map[string]string{}
	for _, part := range []string{"queue", "running", "waiting", "history"} {
		list, _ := raw[part].([]any)
		for _, it := range list {
			o, _ := it.(map[string]any)
			if o == nil {
				continue
			}
			id, _ := o["id"].(string)
			st, _ := o["status"].(string)
			if id != "" && st != "" {
				out[id] = st
			}
		}
	}
	return out
}

// taskArtifactState —— 方案 A：任务目录里有没有**执行产生**的东西。
//
// 只认这几件（都是调度器自己写的）：
//
//	internal-task-report.md · review-report.md · result.json · plan.json · crystallization.jsonl
//	· work/ 非空 · _crystals/ 非空
//
// ★ 不认 `audit.jsonl` 与 `logs/` —— 它们任务一开始就有，拿来判「做完了」会恒真。
func taskArtifactState(taskDir string) (bool, string) {
	for _, f := range []string{
		"internal-task-report.md", "review-report.md", "result.json",
		"plan.json", "crystallization.jsonl",
	} {
		if fi, err := os.Stat(filepath.Join(taskDir, f)); err == nil && fi.Size() > 0 {
			return true, f
		}
	}
	for _, d := range []string{"work", "_crystals"} {
		if ents, err := os.ReadDir(filepath.Join(taskDir, d)); err == nil && len(ents) > 0 {
			return true, d + "/"
		}
	}
	return false, ""
}

// annotateTaskStateTruth —— 给列表逐条补齐三格（就地改，调用方一次遍历）。
//
// 矛盾判据（两条，任一成立即 `status_conflict`）：
//  1. 内存态说 `queued`（还没开始），盘上却已有产物 ⇒ 这就是缺口原文那一句「取单会取到已完成件」；
//  2. 内存态与台账态（各自映射到 10 值后）**不一致** ⇒ 两个真源已经分家，读数不可信。
func annotateTaskStateTruth(tasks []*Task) {
	ledger := ledgerStatusByID()
	root := statepath.TaskRoot()
	for _, t := range tasks {
		if t == nil {
			continue
		}
		if v, ok := ledger[t.ID]; ok {
			t.IssueStatus = issueStatusOf(v)
		}
		present, which := taskArtifactState(filepath.Join(root, sanitizeID(t.ID)))
		t.ArtifactPresent = present

		var why []string
		if t.Status == "queued" && present {
			why = append(why, "status=queued 但产物已在盘（"+which+"）")
		}
		if t.IssueStatus != "" {
			mem := issueStatusOf(t.Status)
			if mem != "unknown" && mem != t.IssueStatus {
				why = append(why, "内存态="+t.Status+"（→"+mem+"）⟷ 台账="+t.IssueStatus)
			}
		}
		if len(why) > 0 {
			t.StatusConflict = true
			t.ConflictReason = strings.Join(why, " · ")
		}
	}
}
