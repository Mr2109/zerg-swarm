package zdb

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
)

// StateChange —— 一次改态请求（actor = 谁请求的 · reason = 为什么 · ✗ 不落无主事件）。
type StateChange struct {
	TaskID string
	From   string // 期望的当前态（乐观校验）
	To     string // 目标态（须在闭集内）
	Reason string
	Actor  string
	Detail string
	At     float64 // 0 ⇒ 取当前时刻
}

// auditHashFn —— 审计链摘要函数（包级可替换缝：测试注入失败以证「同一事务 ⇒ 一并回滚」）。
var auditHashFn = func(payload string) (string, error) {
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:]), nil
}

// auditPayload —— 摘要的确定性输入（链锚 + 事件全字段，按固定序拼）。
func auditPayload(taskID string, seq int64, prevHash, from, to, reason, actor string, ts float64) string {
	return fmt.Sprintf("%s|%s|%d|%s|%s|%s|%s|%.3f", prevHash, taskID, seq, from, to, reason, actor, ts)
}

// ApplyStateChange —— **两表法同事务写**（设计稿 §十.1 ①）：
//
//	同一个事务里：① UPDATE tasks（原地改态 · 乐观 WHERE state=from）② INSERT task_events（append-only 审计 + 链锚）。
//	任一步失败 ⇒ 整体回滚（✗ 不许出现「态改了、事件没落」或反之）。
//	★ 写侧门 = CheckTransition：拦「终态不可回活态/改态」（done/failed/cancelled 只读）；
//	  重跑走 `task retry` 建**新任务**，✗ 不原地回 queued（§九.2 配套①）。
//
// 返回新落的审计锚（audit_seq / audit_hash）。
func (d *DB) ApplyStateChange(sc StateChange) (int64, string, error) {
	if sc.TaskID == "" {
		return 0, "", errors.New("zdb: 改态缺 task_id")
	}
	// 写侧门（唯一正门）：闭集校验 ＋ **终态不可回活态/改态**（CheckTransition · 见 tasks.go · §九.2 配套①）。
	if err := CheckTransition(sc.From, sc.To); err != nil {
		return 0, "", err
	}
	ts := sc.At
	if ts == 0 {
		ts = nowUnix()
	}
	tx, err := d.db.Begin()
	if err != nil {
		return 0, "", fmt.Errorf("zdb: 改态事务开始失败: %w", err)
	}
	// 读当前态（乐观校验）。
	var cur string
	err = tx.QueryRow(`SELECT state FROM tasks WHERE id = ?`, sc.TaskID).Scan(&cur)
	if errors.Is(err, sql.ErrNoRows) {
		tx.Rollback()
		return 0, "", fmt.Errorf("zdb: 任务不存在（%s）", sc.TaskID)
	}
	if err != nil {
		tx.Rollback()
		return 0, "", fmt.Errorf("zdb: 读任务态失败: %w", err)
	}
	if cur != sc.From {
		tx.Rollback()
		return 0, "", fmt.Errorf("%w：库内=%q 期望 from=%q", ErrStateConflict, cur, sc.From)
	}
	// 读链尾（下一 audit_seq 与 prev_hash）。
	var lastSeq int64
	var lastHash sql.NullString
	err = tx.QueryRow(`SELECT audit_seq, audit_hash FROM task_events WHERE task_id = ? ORDER BY audit_seq DESC LIMIT 1`, sc.TaskID).Scan(&lastSeq, &lastHash)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		tx.Rollback()
		return 0, "", fmt.Errorf("zdb: 读审计链尾失败: %w", err)
	}
	if errors.Is(err, sql.ErrNoRows) {
		lastSeq, lastHash = 0, sql.NullString{}
	}
	next := lastSeq + 1

	// ① 原地改态（乐观：WHERE 带 from 守卫；DB CHECK 同时兜住非法 to）。
	res, err := tx.Exec(`UPDATE tasks SET state = ?, updated_at = ? WHERE id = ? AND state = ?`, sc.To, ts, sc.TaskID, sc.From)
	if err != nil {
		tx.Rollback()
		return 0, "", fmt.Errorf("zdb: 改态失败: %w", err)
	}
	if n, _ := res.RowsAffected(); n != 1 {
		tx.Rollback()
		return 0, "", fmt.Errorf("%w：改态影响 %d 行（期望 1）", ErrStateConflict, n)
	}

	// ② 插事件（同事务）。摘要函数失败 ⇒ 回滚 ⇒ 上面那步的 UPDATE 一并撤销（这正是「同事务」的牙）。
	payload := auditPayload(sc.TaskID, next, lastHash.String, sc.From, sc.To, sc.Reason, sc.Actor, ts)
	hash, err := auditHashFn(payload)
	if err != nil {
		tx.Rollback()
		return 0, "", fmt.Errorf("zdb: 算审计摘要失败: %w", err)
	}
	if _, err := tx.Exec(`INSERT INTO task_events(task_id, audit_seq, from_state, to_state, reason, actor, detail, ts, prev_hash, audit_hash)
        VALUES(?,?,?,?,?,?,?,?,?,?)`,
		sc.TaskID, next, nullStr(sc.From), sc.To, nullStr(sc.Reason), nullStr(sc.Actor), nullStr(sc.Detail), ts, nullStr(lastHash.String), hash); err != nil {
		tx.Rollback()
		return 0, "", fmt.Errorf("zdb: 插事件失败: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, "", fmt.Errorf("zdb: 改态提交失败: %w", err)
	}
	return next, hash, nil
}

// TaskEvent —— 一条审计事件（task_events 行）。
type TaskEvent struct {
	RowID     int64
	TaskID    string
	AuditSeq  int64
	FromState string
	ToState   string
	Reason    string
	Actor     string
	Detail    string
	TS        float64
	PrevHash  string
	AuditHash string
}

// Events —— 读某任务的审计流（按 audit_seq 升序）。
func (d *DB) Events(taskID string) ([]TaskEvent, error) {
	rows, err := d.db.Query(`SELECT row_id, task_id, audit_seq, from_state, to_state, reason, actor, detail, ts, prev_hash, audit_hash
        FROM task_events WHERE task_id = ? ORDER BY audit_seq ASC`, taskID)
	if err != nil {
		return nil, fmt.Errorf("zdb: 读事件流失败: %w", err)
	}
	defer rows.Close()
	out := []TaskEvent{}
	for rows.Next() {
		var e TaskEvent
		var from, reason, actor, detail, prev sql.NullString
		if err := rows.Scan(&e.RowID, &e.TaskID, &e.AuditSeq, &from, &e.ToState, &reason, &actor, &detail, &e.TS, &prev, &e.AuditHash); err != nil {
			return nil, fmt.Errorf("zdb: 扫事件行失败: %w", err)
		}
		e.FromState, e.Reason, e.Actor, e.Detail, e.PrevHash = from.String, reason.String, actor.String, detail.String, prev.String
		out = append(out, e)
	}
	return out, rows.Err()
}

// CountEvents —— 事件条数。
func (d *DB) CountEvents(taskID string) (int, error) {
	var n int
	q := `SELECT COUNT(*) FROM task_events`
	args := []any{}
	if taskID != "" {
		q += ` WHERE task_id = ?`
		args = append(args, taskID)
	}
	if err := d.db.QueryRow(q, args...).Scan(&n); err != nil {
		return 0, fmt.Errorf("zdb: 数事件行失败: %w", err)
	}
	return n, nil
}

// AuditHead —— 链尾摘要（「链头另存」的取数口：调用方拿它与自己另存的头对拍，作四问④）。
func (d *DB) AuditHead(taskID string) (int64, string, error) {
	var seq int64
	var hash sql.NullString
	err := d.db.QueryRow(`SELECT audit_seq, audit_hash FROM task_events WHERE task_id = ? ORDER BY audit_seq DESC LIMIT 1`, taskID).Scan(&seq, &hash)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, "", nil
	}
	if err != nil {
		return 0, "", fmt.Errorf("zdb: 读链头失败: %w", err)
	}
	return seq, hash.String, nil
}

// VerifyAuditChain —— 审计链「四问」验证（设计稿 §九.2 牙）：
//
//	① audit_seq 无缺口（恰为 1..N）
//	② 每条摘要可复算（按同口径重算 == 落库值）
//	③ prev_hash 指向上一行的 audit_hash（首条为 NULL）
//	④ 末条 hash == 链外链头（本包只给 AuditHead 取数口；是否与另存头一致由调用方判）
//
// 失败按 CHAIN_BREAK / OUT_OF_ORDER_SEQ / HASH_MISMATCH 命名返回。
func (d *DB) VerifyAuditChain(taskID string) error {
	evs, err := d.Events(taskID)
	if err != nil {
		return err
	}
	prev := ""
	for i, e := range evs {
		want := int64(i + 1)
		if e.AuditSeq != want {
			return fmt.Errorf("OUT_OF_ORDER_SEQ：%s 第 %d 条 audit_seq=%d（期望 %d）", taskID, i, e.AuditSeq, want)
		}
		if e.PrevHash != prev {
			return fmt.Errorf("CHAIN_BREAK：%s audit_seq=%d 的 prev_hash=%q 不指上一行 %q", taskID, e.AuditSeq, e.PrevHash, prev)
		}
		payload := auditPayload(taskID, e.AuditSeq, e.PrevHash, e.FromState, e.ToState, e.Reason, e.Actor, e.TS)
		// 复算用真摘要函数（不受测试缝影响）。
		sum := sha256.Sum256([]byte(payload))
		if hex.EncodeToString(sum[:]) != e.AuditHash {
			return fmt.Errorf("HASH_MISMATCH：%s audit_seq=%d 摘要复算不符", taskID, e.AuditSeq)
		}
		prev = e.AuditHash
	}
	return nil
}
