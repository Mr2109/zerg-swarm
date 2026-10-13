package zdb

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ClosedStates —— `tasks.state` 状态机闭集（用户已拍 · 2026-10-14；**活态 2 ＋ 终态 3**）。
// 设计真源：Zerg-内部文档/01-设计/设计-虫族子代理-20261014.md §九.2（状态机）· §十.3（库侧落地）。
//
//	活态：queued / running
//	终态：done / failed / cancelled（**不可回活态** ⇒ 见 CheckTransition）
//
// ★ 三条归并（旧口径一律收进本闭集，✗ 不再占态名）：
//
//	steered       → running（降为**事件 ＋ 字段** `last_steer_at`）
//	paused        → queued （＋ `paused` 旗标 / `paused_at` / `paused_reason`）
//	waiting_retry → queued （＋ `retry_count` / `next_attempt_at`）
//	cancelled     提为**正式终态**（✗ 不与 failed 混为一谈）
var ClosedStates = []string{"queued", "running", "done", "failed", "cancelled"}

// ActiveStates —— 活态（还可再推进的态）。
var ActiveStates = []string{"queued", "running"}

// TerminalStates —— 终态（一生到此为止；**不可回活态**，✗ 也不可终态→终态）。
var TerminalStates = []string{"done", "failed", "cancelled"}

// FailedExitReasons —— 落 `failed` 的结局词闭集（配套③：与回执 `exit_reason` 对齐）。
// 依据 §九.2：`timeout` / `stalled` / `max_attempts_exceeded`（＋ `worker_lost`）⇒ 终态 `failed`。
var FailedExitReasons = []string{"timeout", "stalled", "max_attempts_exceeded", "worker_lost"}

// CancelledExitReason —— 落 `cancelled` 的结局词（**单独**一档，✗ 不并入 failed · 配套③）。
const CancelledExitReason = "cancelled"

// IsClosedState —— 是否落在状态机闭集内。
func IsClosedState(s string) bool {
	for _, c := range ClosedStates {
		if c == s {
			return true
		}
	}
	return false
}

// IsTerminalState —— 是否终态（done/failed/cancelled）。
func IsTerminalState(s string) bool {
	for _, c := range TerminalStates {
		if c == s {
			return true
		}
	}
	return false
}

// IsActiveState —— 是否活态（queued/running）。
func IsActiveState(s string) bool {
	for _, c := range ActiveStates {
		if c == s {
			return true
		}
	}
	return false
}

// NormalizeLegacyState —— **读侧门**：旧值归一（把旧状态机的值映进新五值闭集）。
//
//	paused / waiting_retry → queued（旧文件机的可恢复两态）
//	steered                → running（0001 闭集里的注入态 ⇒ 降为事件 ＋ last_steer_at）
//	其余原值返回（含 done/failed/cancelled ⇒ 幂等）
//
// ★ 与 0002 迁移 SQL 的 `CASE` **同一口径**（两处都改才不漂）。
func NormalizeLegacyState(s string) string {
	switch s {
	case "paused", "waiting_retry":
		return "queued"
	case "steered":
		return "running"
	}
	return s
}

// TerminalStateForExitReason —— 结局词 → 终态（配套③：回执 `exit_reason` 闭集 ⟷ 状态闭集）。
// 命中返终态名；不在闭集 ⇒ ("", false)（调用方 fail-closed，✗ 别默认塞 failed）。
func TerminalStateForExitReason(reason string) (string, bool) {
	if reason == CancelledExitReason {
		return "cancelled", true
	}
	for _, r := range FailedExitReasons {
		if r == reason {
			return "failed", true
		}
	}
	return "", false
}

// CheckTransition —— **写侧门 ＝ 「终态不可回活态」判据**（唯一正门；`ApplyStateChange` 必过它）。
//
// 牙（配套①）：
//   - from / to 必须都在五值闭集内（✗ 不收旧值名 —— 写侧只认新闭集，旧值走 NormalizeLegacyState）；
//   - **from ∈ 终态 ⇒ 一律拒**（`ErrTerminalImmutable`）：done/failed/cancelled 一经落定即只读，
//     既不可回活态，也不可终态→终态。
//     ⇒ 推论：「重跑」只能走 `task retry` **建新任务**（新 task_id ＋ `parent_task_id` 链回旧任务），
//     ✗ 不是把终态行改回 `queued`（§九.2 配套①）。
//   - 活态之间（queued ⇄ running）与 活态→终态 一律放行（软停 running→queued、认领 queued→running …）。
//
// ✗ 别在别处另写一份改态前置 —— 两把尺必漂。
func CheckTransition(from, to string) error {
	if !IsClosedState(from) {
		return fmt.Errorf("zdb: from 态 %q 不在闭集 %v 内（旧值先走 NormalizeLegacyState）", from, ClosedStates)
	}
	if !IsClosedState(to) {
		return fmt.Errorf("zdb: to 态 %q 不在闭集 %v 内（旧值先走 NormalizeLegacyState）", to, ClosedStates)
	}
	if IsTerminalState(from) {
		return fmt.Errorf("%w：终态不可回活态/改态（%s → %s）—— done/failed/cancelled 一经落定即只读；重跑走 `task retry` 建**新任务**（§九.2 配套①）",
			ErrTerminalImmutable, from, to)
	}
	return nil
}

// Task —— 一行 = 一枚子代理的一生（tasks 当前态表）。字段与 CLI 字段面（--json）同源自设计稿 §十.1。
type Task struct {
	ID           string
	SliceID      string
	DependsOn    string
	ParentTaskID string
	SessionID    string
	Model        string
	Caller       string
	State        string
	Descr        string
	CtxHash      string
	CreatedAt    float64
}

// InsertTask —— 插一行任务（调用方给 id；默认态 queued）。**写侧门**：只收新五值闭集
// （旧值名不给过，报话点名 NormalizeLegacyState；DB CHECK 之外的应用层早退，✗ 不替代 CHECK）。
func (d *DB) InsertTask(t Task) error {
	if t.ID == "" {
		return errors.New("zdb: 任务 id 不能为空")
	}
	if t.State == "" {
		t.State = "queued"
	}
	if !IsClosedState(t.State) {
		if n := NormalizeLegacyState(t.State); n != t.State {
			return fmt.Errorf("zdb: 状态 %q 是旧口径（NormalizeLegacyState 现归一为 %q）—— 写侧只收新五值闭集 %v", t.State, n, ClosedStates)
		}
		return fmt.Errorf("zdb: 状态 %q 不在闭集 %v 内", t.State, ClosedStates)
	}
	ts := t.CreatedAt
	if ts == 0 {
		ts = nowUnix()
	}
	_, err := d.db.Exec(`INSERT INTO tasks(
        id, slice_id, depends_on, parent_task_id, session_id, model, caller, state, descr, ctx_hash, created_at, updated_at
    ) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`,
		t.ID, t.SliceID, nullStr(t.DependsOn), nullStr(t.ParentTaskID), nullStr(t.SessionID),
		nullStr(t.Model), nullStr(t.Caller), t.State, nullStr(t.Descr), nullStr(t.CtxHash), ts, ts)
	if err != nil {
		return fmt.Errorf("zdb: 插任务失败: %w", err)
	}
	return nil
}

// GetTaskState —— 读某任务当前态（不存在 ⇒ ErrNoRows 包装）。
// ★ 本口返回**库内原值**（迁移后必为五值闭集之一）；要按新闭集判活/判终用 IsActiveState / IsTerminalState。
func (d *DB) GetTaskState(id string) (string, error) {
	var s string
	err := d.db.QueryRow(`SELECT state FROM tasks WHERE id = ?`, id).Scan(&s)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("zdb: 任务不存在（%s）", id)
	}
	if err != nil {
		return "", fmt.Errorf("zdb: 读任务态失败: %w", err)
	}
	return s, nil
}

// CountTasks —— 任务行数（M0 只读镜像的「文件读数 == 库读数」对拍用）。
func (d *DB) CountTasks() (int, error) {
	var n int
	if err := d.db.QueryRow(`SELECT COUNT(*) FROM tasks`).Scan(&n); err != nil {
		return 0, fmt.Errorf("zdb: 数任务行失败: %w", err)
	}
	return n, nil
}

// TaskExists —— 是否存在（幂等/兼容判断用）。
func (d *DB) TaskExists(id string) (bool, error) {
	var n int
	if err := d.db.QueryRow(`SELECT COUNT(*) FROM tasks WHERE id = ?`, id).Scan(&n); err != nil {
		return false, fmt.Errorf("zdb: 查任务存在失败: %w", err)
	}
	return n > 0, nil
}

// nullStr —— 空串存 NULL（可空列的干净语义）。
func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// nowUnix —— 秒级 float 时间戳（与 chat_store 的 now() 同口径）。
func nowUnix() float64 {
	return float64(time.Now().UnixMilli()) / 1000.0
}
