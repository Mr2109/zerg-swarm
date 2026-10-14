package api

// fleet_logs_store.go — 机群日志落盘存储（GAP-20260927-202 ① · 段1 主控侧）。
//
// 定案真源：gateway/protocol.md:73-83 ——
//   POST /api/fleet/logs 子端日志批量接收（校验 token，落 SQLite）；
//   表 logs(id, ts, machine, level, component, model, request_id, message)；
//   索引 ts / machine / component / request_id；保留 30 天；
//   GET query / trace / errors / stats 四条查询面。
//
// 体例照本仓既有 SQLite 件（core/internal/chat/chat_store.go）：modernc.org/sqlite
// （纯 Go、无 CGO）+ database/sql + WAL。**不新造第二套存储层**。
//
// 库位置照既有状态目录口径（core/internal/statepath）：默认
// <ZERG_STATE_DIR 或 家目录/.zerg/state>/fleet_logs.db —— 随 ZERG_STATE_DIR 可迁、
// 第二实例天然隔离。（protocol.md 原文给的是 Swift 原件的固定目录；本仓口径统一走
// statepath。）取不到/打不开 ⇒ 返回错误，调用方（handler）fail-closed（退 503），
// ✗ 不静默吞、✗ 不假装成功。

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
	_ "modernc.org/sqlite"
)

// fleetLogsDBFile — 状态目录下的库文件名（protocol.md 定案：fleet_logs.db）。
const fleetLogsDBFile = "fleet_logs.db"

// fleetLogsMaxBatch — 单批上限（设计稿 §三.5 附 H①：500 条/批 · 超出 ⇒ 切多批连续投 · ✗ 不攒巨 payload）。
const fleetLogsMaxBatch = 500

// fleetLogsSchemaSQL — 建表 + 索引（四条索引逐字照 protocol.md:82-83 定案）。
//
// ★ T5-c 扩两列 ＋ 一条唯一约束（设计稿 §三.5 附 H③/④ · §六 Q6 / Q18）：
//
//	· `event_id`    —— 子端生成的 **ULID**（入 spool 前生成一次、重投复用）⇒ 幂等键的**节点侧一半**；
//	· `received_at` —— **主控收件钟**（`time.Now()` 的 UTC RFC3339 文本）⇒ 与节点钟 `ts` 交叉校验；
//	· `UNIQUE(machine, event_id)` —— 幂等键落成**表级唯一约束**（照 zdb 幂等体例
//	  `zdb/migrations/0001_init.sql` 的 `UNIQUE(slice_id, idempotency_key)`；Q18 复名：`node_id` ≡ 现列 `machine`）
//	  ⇒ 唯一仲裁点 = **DB 唯一约束**（✗ 不是「先查后插」，那在并发下必然双插）。
const fleetLogsSchemaSQL = `
CREATE TABLE IF NOT EXISTS logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    ts TEXT, machine TEXT, level TEXT, component TEXT, model TEXT, request_id TEXT, message TEXT,
    event_id TEXT,
    received_at TEXT,
    UNIQUE(machine, event_id)
);
CREATE INDEX IF NOT EXISTS idx_logs_ts ON logs(ts);
CREATE INDEX IF NOT EXISTS idx_logs_machine ON logs(machine);
CREATE INDEX IF NOT EXISTS idx_logs_component ON logs(component);
CREATE INDEX IF NOT EXISTS idx_logs_request_id ON logs(request_id);
`

// fleetLogsAddedColumns — T5-c 新加的两列；**本特性之前建好的老库**由 ensureFleetLogsSchema 逐列补
// （`CREATE TABLE IF NOT EXISTS` 对已存在的表是 no-op ⇒ 光改建表语句补不上老库）。
var fleetLogsAddedColumns = []struct{ name, decl string }{
	{"event_id", "TEXT"},
	{"received_at", "TEXT"},
}

// ensureFleetLogsSchema — 建表 / 补列 / 建唯一索引（幂等；**新库与老库两态都过**）。
//
//   - **新库**：建表语句里已带 `UNIQUE(machine, event_id)` ⇒ 一条 `CREATE TABLE` 到位；
//   - **老库**（本特性之前建的 `logs` 表）：`CREATE TABLE IF NOT EXISTS` 是 no-op ⇒ 逐列
//     `ALTER TABLE … ADD COLUMN` 补 `event_id` / `received_at`，再补一条**显式唯一索引**
//     （表级 `UNIQUE` **加不进**已存在的表 —— SQLite 的 `ALTER TABLE` 只支持 ADD/DROP COLUMN / RENAME）
//     ⇒ 两态都拿得到 `(machine, event_id)` 上的唯一约束，`ON CONFLICT(machine, event_id)` 才仲裁得住。
//     ★ 老库里已存的行 `event_id` 为 NULL：SQLite 唯一索引里 **NULL 互不相等** ⇒ 老行不互撞、
//     也不被新行顶掉（去重只对**带得上 `event_id`** 的新行成立 —— 时效边界见 §三.5 附 H③）。
func ensureFleetLogsSchema(db *sql.DB) error {
	var pre int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='logs'`).Scan(&pre); err != nil {
		return fmt.Errorf("fleet_logs: 查 logs 表是否存在失败: %w", err)
	}
	if _, err := db.Exec(fleetLogsSchemaSQL); err != nil {
		return fmt.Errorf("fleet_logs: 建表失败: %w", err)
	}
	if pre == 0 {
		return nil // 新库：建表语句里已带 UNIQUE(machine, event_id)
	}
	for _, col := range fleetLogsAddedColumns {
		has, err := fleetLogsHasColumn(db, col.name)
		if err != nil {
			return err
		}
		if has {
			continue
		}
		if _, err := db.Exec("ALTER TABLE logs ADD COLUMN " + col.name + " " + col.decl); err != nil {
			return fmt.Errorf("fleet_logs: 补列 %s 失败: %w", col.name, err)
		}
	}
	if _, err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS uq_logs_machine_event_id ON logs(machine, event_id)`); err != nil {
		return fmt.Errorf("fleet_logs: 建唯一索引失败: %w", err)
	}
	return nil
}

// fleetLogsHasColumn — `PRAGMA table_info(logs)` 里有没有该列。
func fleetLogsHasColumn(db *sql.DB, col string) (bool, error) {
	rows, err := db.Query(`PRAGMA table_info(logs)`)
	if err != nil {
		return false, fmt.Errorf("fleet_logs: 读表结构失败: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notnull, pk int
		var name, typ string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			return false, fmt.Errorf("fleet_logs: 扫描表结构失败: %w", err)
		}
		if name == col {
			return true, nil
		}
	}
	return false, rows.Err()
}

// FleetLogEntry — 一条机群日志（字段名逐字照 protocol.md 定案）。
type FleetLogEntry struct {
	TS        string `json:"ts"`
	Machine   string `json:"machine"`
	Level     string `json:"level"`
	Component string `json:"component"`
	Model     string `json:"model"`
	RequestID string `json:"request_id"`
	Message   string `json:"message"`
	// ★ T5-c 两格（§三.5 附 H③/④）：
	//   · `event_id`    = 子端 ULID（请求体带上来 ⇒ 主控照存；空 ⇒ 存 NULL ⇒ 该行**不参与去重**）；
	//   · `received_at` = **主控收件钟**（请求体带不带都以此为准 —— 由 InsertWithCounts 现算填入，
	//     ✗ 不信节点送来的值；见 h④「排序权威不信节点钟」）。
	EventID    string `json:"event_id,omitempty"`
	ReceivedAt string `json:"received_at,omitempty"`
}

// FleetLogStore — 机群日志 SQLite 存储。
type FleetLogStore struct {
	db *sql.DB
}

// OpenFleetLogStore — 按既有状态目录口径打开/初始化机群日志库（ZERG_STATE_DIR 可覆盖）。
func OpenFleetLogStore() (*FleetLogStore, error) {
	dir := statepath.Dir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("fleet_logs: 创建状态目录失败: %w", err)
	}
	return openFleetLogStoreAt(filepath.Join(dir, fleetLogsDBFile))
}

// openFleetLogStoreAt — 打开指定路径的库（测试注入临时目录用）。
func openFleetLogStoreAt(path string) (*FleetLogStore, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("fleet_logs: 创建目录失败: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("fleet_logs: 打开数据库失败: %w", err)
	}
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("fleet_logs: 设置 WAL 失败: %w", err)
	}
	if err := ensureFleetLogsSchema(db); err != nil {
		db.Close()
		return nil, err
	}
	return &FleetLogStore{db: db}, nil
}

// Close — 关闭库。
func (s *FleetLogStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// fleetLogsLazy — 每实例惰性打开器（sync.Once 保证并发安全）。
type fleetLogsLazy struct {
	once sync.Once
	val  *FleetLogStore
	err  error
}

// fleetLogsOpenFn — 打开默认库的入口；仅测试替换（模拟「库取不到」）。
var fleetLogsOpenFn = OpenFleetLogStore

// fleetLogStore — 取库：优先注入的实例（测试/主控显式注入），否则惰性打开默认库（一次）。
// 返回 nil 且 err != nil 时调用方必须 fail-closed（✗ 不静默吞）。
func (h *Handlers) fleetLogStore() (*FleetLogStore, error) {
	if h != nil && h.FleetLogs != nil {
		return h.FleetLogs, nil
	}
	h.fleetLogsLazy.once.Do(func() {
		h.fleetLogsLazy.val, h.fleetLogsLazy.err = fleetLogsOpenFn()
	})
	return h.fleetLogsLazy.val, h.fleetLogsLazy.err
}

// FleetLogsInsertResult — 一批落盘的真新 / 真重复计数（§三.5 附 H③：判定唯一手段 = `RowsAffected()`）。
type FleetLogsInsertResult struct {
	Stored    int // 真新（`ON CONFLICT DO NOTHING` 真插进去的行数）
	Duplicate int // 真重复（命中已存 `(machine, event_id)` ⇒ 没插的行数）
}

// Insert — 一批日志落盘（**向后兼容壳**：不计数的调用方仍用它；新调用方用 InsertWithCounts 拿 duplicate）。
func (s *FleetLogStore) Insert(machine string, entries []FleetLogEntry) error {
	_, err := s.InsertWithCounts(machine, entries)
	return err
}

// InsertWithCounts — 一批日志落盘并**计真新 / 真重复**（§三.5 附 H①③）：
//
//	· 每批 ≤ `fleetLogsMaxBatch`（500）条 —— 超出 ⇒ **切多批**，每批**各自一个事务**，计数累加；
//	· 每条 `INSERT … ON CONFLICT(machine, event_id) DO NOTHING` —— 唯一仲裁点 = 表上的唯一约束；
//	· 真新 / 真重复 = `res.RowsAffected()`：`1` ⇒ 真新 · `0` ⇒ 真重复（照 zdb 幂等体例 `zdb/idempotency.go`）；
//	· `received_at` = **主控收件钟**（一次批量算一次 · `time.Now()` 的 UTC RFC3339 文本）⇒ 请求体里带不带它都以此为准。
//	· ★ 空 `event_id` 的行：唯一索引下 NULL 互不相等 ⇒ 每投都算真新（老 v1 档 / 无 event_id 的客户端），
//	  这是**写明的时效边界**（去重只在「带得上 event_id 且在保留窗内」时成立 · §三.5 附 H③）。
func (s *FleetLogStore) InsertWithCounts(machine string, entries []FleetLogEntry) (FleetLogsInsertResult, error) {
	var out FleetLogsInsertResult
	if s == nil || s.db == nil {
		return out, fmt.Errorf("fleet_logs: 存储未初始化")
	}
	if len(entries) == 0 {
		return out, nil
	}
	received := time.Now().UTC().Format(time.RFC3339)
	for start := 0; start < len(entries); start += fleetLogsMaxBatch {
		end := start + fleetLogsMaxBatch
		if end > len(entries) {
			end = len(entries)
		}
		stored, dup, err := s.insertChunk(machine, received, entries[start:end])
		if err != nil {
			return out, err
		}
		out.Stored += stored
		out.Duplicate += dup
	}
	return out, nil
}

// insertChunk — 一批（≤500）条 = **一个事务**（§三.5 附 H①「单批 = 一个事务」；写池 MaxOpenConns=1）。
func (s *FleetLogStore) insertChunk(machine, received string, entries []FleetLogEntry) (stored, dup int, err error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, 0, fmt.Errorf("fleet_logs: 事务开始失败: %w", err)
	}
	stmt, err := tx.Prepare(`INSERT INTO logs(ts, machine, level, component, model, request_id, message, event_id, received_at)
        VALUES(?,?,?,?,?,?,?,?,?)
        ON CONFLICT(machine, event_id) DO NOTHING`)
	if err != nil {
		tx.Rollback()
		return 0, 0, fmt.Errorf("fleet_logs: 预编语句失败: %w", err)
	}
	defer stmt.Close()
	for _, e := range entries {
		res, err := stmt.Exec(fleetNormalizeTS(e.TS), machine, e.Level, e.Component, e.Model, e.RequestID, e.Message, nullFleetStr(e.EventID), received)
		if err != nil {
			tx.Rollback()
			return 0, 0, fmt.Errorf("fleet_logs: 写入失败: %w", err)
		}
		if n, _ := res.RowsAffected(); n == 0 {
			dup++
		} else {
			stored++
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, 0, fmt.Errorf("fleet_logs: 提交失败: %w", err)
	}
	return stored, dup, nil
}

// nullFleetStr — 空串存 NULL（`event_id` 留 NULL ⇒ 唯一约束下多行并存 = 不误判重复）。
func nullFleetStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// fleetNormalizeTS — ts 归一到 UTC RFC3339（让 since 过滤可按文本比较；不可解析则原样存）。
func fleetNormalizeTS(ts string) string {
	ts = strings.TrimSpace(ts)
	if ts == "" {
		return ts
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if t, err := time.Parse(layout, ts); err == nil {
			return t.UTC().Format(time.RFC3339)
		}
	}
	return ts
}

// fleetSinceCanon — 解析 since（10m/1h/30s 时长，或 ISO8601）为 UTC RFC3339 文本；
// 空或不可解析 ⇒ ""（该维不过滤）。
func fleetSinceCanon(since string) string {
	since = strings.TrimSpace(since)
	if since == "" {
		return ""
	}
	if d, err := time.ParseDuration(since); err == nil {
		return time.Now().Add(-d).UTC().Format(time.RFC3339)
	}
	if t, err := time.Parse(time.RFC3339, since); err == nil {
		return t.UTC().Format(time.RFC3339)
	}
	return ""
}

// QueryLogs — 按 machine/component/since/level 过滤（空值 = 该维不过滤），按 id 升序。
func (s *FleetLogStore) QueryLogs(machine, component, since, level string) ([]FleetLogEntry, error) {
	where := []string{}
	args := []any{}
	if machine != "" {
		where = append(where, "machine = ?")
		args = append(args, machine)
	}
	if component != "" {
		where = append(where, "component = ?")
		args = append(args, component)
	}
	if level != "" {
		where = append(where, "level = ?")
		args = append(args, level)
	}
	if c := fleetSinceCanon(since); c != "" {
		where = append(where, "ts >= ?")
		args = append(args, c)
	}
	return s.selectLogs(where, args)
}

// TraceByRequest — 按 request_id 拉全链路（按 id 升序）。
func (s *FleetLogStore) TraceByRequest(requestID string) ([]FleetLogEntry, error) {
	return s.selectLogs([]string{"request_id = ?"}, []any{requestID})
}

// ErrorLogs — 只看 ERROR（since 可选）。
func (s *FleetLogStore) ErrorLogs(since string) ([]FleetLogEntry, error) {
	where := []string{"level = ?"}
	args := []any{"ERROR"}
	if c := fleetSinceCanon(since); c != "" {
		where = append(where, "ts >= ?")
		args = append(args, c)
	}
	return s.selectLogs(where, args)
}

// StatsByModel — 某模型（空 = 全部）按 level 计数。
func (s *FleetLogStore) StatsByModel(model string) (map[string]int, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("fleet_logs: 存储未初始化")
	}
	q := "SELECT level, COUNT(*) FROM logs"
	args := []any{}
	if model != "" {
		q += " WHERE model = ?"
		args = append(args, model)
	}
	q += " GROUP BY level"
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("fleet_logs: 统计查询失败: %w", err)
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var lvl string
		var n int
		if err := rows.Scan(&lvl, &n); err != nil {
			return nil, fmt.Errorf("fleet_logs: 统计扫描失败: %w", err)
		}
		out[lvl] = n
	}
	return out, rows.Err()
}

// selectLogs — 共用查询（结果恒为非 nil 切片 ⇒ JSON 编出 [] 而非 null）。
func (s *FleetLogStore) selectLogs(where []string, args []any) ([]FleetLogEntry, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("fleet_logs: 存储未初始化")
	}
	q := "SELECT ts, machine, level, component, model, request_id, message, event_id, received_at FROM logs"
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	q += " ORDER BY id ASC"
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("fleet_logs: 查询失败: %w", err)
	}
	defer rows.Close()
	out := []FleetLogEntry{}
	for rows.Next() {
		var e FleetLogEntry
		var ev, ra sql.NullString
		if err := rows.Scan(&e.TS, &e.Machine, &e.Level, &e.Component, &e.Model, &e.RequestID, &e.Message, &ev, &ra); err != nil {
			return nil, fmt.Errorf("fleet_logs: 扫描失败: %w", err)
		}
		e.EventID, e.ReceivedAt = ev.String, ra.String
		out = append(out, e)
	}
	return out, rows.Err()
}
