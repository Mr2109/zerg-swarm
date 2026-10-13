package zdb

import (
	"database/sql"
	"errors"
	"fmt"
)

// IdempotencyTTL —— 幂等键保留窗口（设计稿 §十.1 ③ 建议 72h）。
const IdempotencyTTL = 72 * 3600.0

// SubmitResult —— 派单结果。Created=false ⇒ 命中已存在的幂等键（回原 task_id · 未重复派单）。
type SubmitResult struct {
	TaskID  string
	Created bool
}

// SubmitTask —— 幂等派单（设计稿 §十.1 ③ · 片 6 三态）：
//
//	① 首同键 ⇒ Created=true、给新 task_id（rc=0）
//	② 二同键同参 ⇒ Created=false、**回原 task_id**（标「已存在 · 未重复派单」）
//	③ 同键异参 ⇒ ErrIdempotencyConflict（明报冲突 · 靠 fingerprint）
//
// ★ 唯一性靠**原子插入**：`INSERT … ON CONFLICT(slice_id, idempotency_key) DO NOTHING`，
// 以 **DB 唯一约束当唯一仲裁点** —— ✗ 不是「先查后插」两步（那在并发下必然双插）。
// 建任务 + 首条事件（queued）与幂等行落库在**同一事务**里。
func (d *DB) SubmitTask(t Task, idemKey, fingerprint string) (SubmitResult, error) {
	if t.ID == "" {
		return SubmitResult{}, errors.New("zdb: 派单缺 task_id")
	}
	if t.State == "" {
		t.State = "queued"
	}
	if !IsClosedState(t.State) {
		return SubmitResult{}, fmt.Errorf("zdb: 状态 %q 不在闭集 %v 内", t.State, ClosedStates)
	}
	ts := t.CreatedAt
	if ts == 0 {
		ts = nowUnix()
	}
	tx, err := d.db.Begin()
	if err != nil {
		return SubmitResult{}, fmt.Errorf("zdb: 派单事务开始失败: %w", err)
	}
	if idemKey != "" {
		res, err := tx.Exec(`INSERT INTO idempotency_keys(slice_id, idempotency_key, fingerprint, state, task_id, created_at, expires_at)
            VALUES(?,?,?,?,?,?,?)
            ON CONFLICT(slice_id, idempotency_key) DO NOTHING`,
			t.SliceID, idemKey, nullStr(fingerprint), t.State, t.ID, ts, ts+IdempotencyTTL)
		if err != nil {
			tx.Rollback()
			return SubmitResult{}, fmt.Errorf("zdb: 幂等插入失败: %w", err)
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			// 命中已存在 —— 回原 task_id。
			var prevTask string
			var prevFP sql.NullString
			if err := tx.QueryRow(`SELECT task_id, fingerprint FROM idempotency_keys WHERE slice_id = ? AND idempotency_key = ?`,
				t.SliceID, idemKey).Scan(&prevTask, &prevFP); err != nil {
				tx.Rollback()
				return SubmitResult{}, fmt.Errorf("zdb: 读已存幂等行失败: %w", err)
			}
			if prevFP.String != fingerprint {
				tx.Rollback()
				return SubmitResult{}, fmt.Errorf("%w：slice=%s 键=%s 已存在且指纹不同（库=%s 请求=%s）",
					ErrIdempotencyConflict, t.SliceID, idemKey, prevFP.String, fingerprint)
			}
			if err := tx.Commit(); err != nil {
				return SubmitResult{}, fmt.Errorf("zdb: 派单命中提交失败: %w", err)
			}
			return SubmitResult{TaskID: prevTask, Created: false}, nil
		}
	}
	// 建任务行 + 首条事件（同事务）。
	if _, err := tx.Exec(`INSERT INTO tasks(
            id, slice_id, depends_on, parent_task_id, session_id, model, caller, state, descr, ctx_hash, idempotency_key, request_hash, created_at, updated_at
        ) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		t.ID, t.SliceID, nullStr(t.DependsOn), nullStr(t.ParentTaskID), nullStr(t.SessionID), nullStr(t.Model),
		nullStr(t.Caller), t.State, nullStr(t.Descr), nullStr(t.CtxHash),
		nullStr(idemKey), nullStr(fingerprint), ts, ts); err != nil {
		tx.Rollback()
		return SubmitResult{}, fmt.Errorf("zdb: 插任务失败: %w", err)
	}
	payload := auditPayload(t.ID, 1, "", "", t.State, "submit", t.Caller, ts)
	hash, err := auditHashFn(payload)
	if err != nil {
		tx.Rollback()
		return SubmitResult{}, fmt.Errorf("zdb: 算首事件摘要失败: %w", err)
	}
	if _, err := tx.Exec(`INSERT INTO task_events(task_id, audit_seq, from_state, to_state, reason, actor, ts, prev_hash, audit_hash)
        VALUES(?,?,?,?,?,?,?,?,?)`,
		t.ID, 1, nil, t.State, "submit", nullStr(t.Caller), ts, nil, hash); err != nil {
		tx.Rollback()
		return SubmitResult{}, fmt.Errorf("zdb: 插首事件失败: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return SubmitResult{}, fmt.Errorf("zdb: 派单提交失败: %w", err)
	}
	return SubmitResult{TaskID: t.ID, Created: true}, nil
}

// CountIdempotencyKeys —— 幂等表行数。
func (d *DB) CountIdempotencyKeys() (int, error) {
	var n int
	if err := d.db.QueryRow(`SELECT COUNT(*) FROM idempotency_keys`).Scan(&n); err != nil {
		return 0, fmt.Errorf("zdb: 数幂等行失败: %w", err)
	}
	return n, nil
}

// PurgeExpiredIdempotency —— 清过期幂等键（TTL 到点由调度面定期调；返回删了几行）。
func (d *DB) PurgeExpiredIdempotency(now float64) (int64, error) {
	if now == 0 {
		now = nowUnix()
	}
	res, err := d.db.Exec(`DELETE FROM idempotency_keys WHERE expires_at < ?`, now)
	if err != nil {
		return 0, fmt.Errorf("zdb: 清过期幂等键失败: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}
