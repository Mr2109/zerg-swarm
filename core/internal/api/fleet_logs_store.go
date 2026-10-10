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

// fleetLogsSchemaSQL — 建表 + 索引（列与索引逐字照 protocol.md:82-83 定案）。
const fleetLogsSchemaSQL = `
CREATE TABLE IF NOT EXISTS logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    ts TEXT, machine TEXT, level TEXT, component TEXT, model TEXT, request_id TEXT, message TEXT
);
CREATE INDEX IF NOT EXISTS idx_logs_ts ON logs(ts);
CREATE INDEX IF NOT EXISTS idx_logs_machine ON logs(machine);
CREATE INDEX IF NOT EXISTS idx_logs_component ON logs(component);
CREATE INDEX IF NOT EXISTS idx_logs_request_id ON logs(request_id);
`

// FleetLogEntry — 一条机群日志（字段名逐字照 protocol.md 定案）。
type FleetLogEntry struct {
	TS        string `json:"ts"`
	Machine   string `json:"machine"`
	Level     string `json:"level"`
	Component string `json:"component"`
	Model     string `json:"model"`
	RequestID string `json:"request_id"`
	Message   string `json:"message"`
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
	if _, err := db.Exec(fleetLogsSchemaSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("fleet_logs: 建表失败: %w", err)
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

// Insert — 一批日志落盘（一个事务；machine 取自上报顶层字段）。
func (s *FleetLogStore) Insert(machine string, entries []FleetLogEntry) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("fleet_logs: 存储未初始化")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("fleet_logs: 事务开始失败: %w", err)
	}
	stmt, err := tx.Prepare("INSERT INTO logs(ts, machine, level, component, model, request_id, message) VALUES(?,?,?,?,?,?,?)")
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("fleet_logs: 预编语句失败: %w", err)
	}
	defer stmt.Close()
	for _, e := range entries {
		if _, err := stmt.Exec(fleetNormalizeTS(e.TS), machine, e.Level, e.Component, e.Model, e.RequestID, e.Message); err != nil {
			tx.Rollback()
			return fmt.Errorf("fleet_logs: 写入失败: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("fleet_logs: 提交失败: %w", err)
	}
	return nil
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
	q := "SELECT ts, machine, level, component, model, request_id, message FROM logs"
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
		if err := rows.Scan(&e.TS, &e.Machine, &e.Level, &e.Component, &e.Model, &e.RequestID, &e.Message); err != nil {
			return nil, fmt.Errorf("fleet_logs: 扫描失败: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
