package api

// fleet_logs_hardening_test.go — T5-c 主控侧加固的定向判据（段1 · `core/internal/api`）。
//
// 覆盖（设计稿 §三.5 附 H①③④ · §六 Q5 / Q6 / Q10 / Q18）：
//   · 重复投递计数：同一批（带 `event_id`）连投两次 ⇒ 第二次 `stored=0` / `duplicate=批大小`、库内行数不增；
//   · 批上限 500：501 条一批 ⇒ 切两批落完（`stored=501`）；再投 ⇒ `duplicate=501`、行数仍 501；
//   · 库面：`.schema logs` 含 `event_id TEXT` ＋ `UNIQUE(machine, event_id)`（新库）；**老库**（旧形态建的表）
//     经 `ensureFleetLogsSchema` 补列 ＋ 补唯一索引（两态都过）；
//   · `received_at` = 主控收件钟（请求体送来的那一格**不作数**）；
//   · 时效边界：无 `event_id` 的行不参与去重（每投都算真新）。

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// fleetLogsResp — POST /api/fleet/logs 的响应体三格。
type fleetLogsResp struct {
	OK        bool `json:"ok"`
	Stored    int  `json:"stored"`
	Duplicate int  `json:"duplicate"`
}

func decodeFleetLogsResp(t *testing.T, body string) fleetLogsResp {
	t.Helper()
	var r fleetLogsResp
	if err := json.Unmarshal([]byte(body), &r); err != nil {
		t.Fatalf("响应体不是合法 JSON：%v（%s）", err, body)
	}
	return r
}

func fleetLogsRowCount(t *testing.T, st *FleetLogStore) int {
	t.Helper()
	var n int
	if err := st.db.QueryRow(`SELECT COUNT(*) FROM logs`).Scan(&n); err != nil {
		t.Fatalf("数 logs 行失败: %v", err)
	}
	return n
}

// batchBody — 造一批带 `event_id` 的上报体（event_id 逐条唯一）。
func batchBody(machine string, n int, prefix string) string {
	parts := make([]string, 0, n)
	for i := 0; i < n; i++ {
		parts = append(parts, fmt.Sprintf(
			`{"ts":"2026-10-14T02:0%d:00Z","level":"INFO","component":"agent","message":"m%d","event_id":"%s%03d"}`,
			i%10, i, prefix, i))
	}
	return fmt.Sprintf(`{"machine":%q,"logs":[%s]}`, machine, strings.Join(parts, ","))
}

// ★ 判据格 1：同一批连投两次 ⇒ 第二次 duplicate = 批大小、库内行数不增（真新/真重复靠 RowsAffected）。
func TestFleetLogsBatch_DuplicateCounting(t *testing.T) {
	h := newFleetLogsHandlers(t)
	st := h.FleetLogs
	body := batchBody("node-a", 3, "01J8")

	w1 := postLogs(h, body)
	if w1.Code != 200 {
		t.Fatalf("首投应 200，得 %d body=%s", w1.Code, w1.Body.String())
	}
	r1 := decodeFleetLogsResp(t, w1.Body.String())
	if !r1.OK || r1.Stored != 3 || r1.Duplicate != 0 {
		t.Fatalf("首投应 {ok:true,stored:3,duplicate:0}，得 %+v", r1)
	}
	if n := fleetLogsRowCount(t, st); n != 3 {
		t.Fatalf("首投后库内应 3 行，得 %d", n)
	}

	w2 := postLogs(h, body)
	if w2.Code != 200 {
		t.Fatalf("重投应 200（幂等不报错），得 %d body=%s", w2.Code, w2.Body.String())
	}
	r2 := decodeFleetLogsResp(t, w2.Body.String())
	if !r2.OK || r2.Stored != 0 || r2.Duplicate != 3 {
		t.Fatalf("重投应 {ok:true,stored:0,duplicate:3}，得 %+v", r2)
	}
	if n := fleetLogsRowCount(t, st); n != 3 {
		t.Fatalf("重投后库内**不该增加**（应仍 3 行），得 %d", n)
	}
	if r1.Stored+r1.Duplicate != 3 || r2.Stored+r2.Duplicate != 3 {
		t.Fatalf("N+M 应恒 = 本批条数（3）：首投 %d+%d · 重投 %d+%d", r1.Stored, r1.Duplicate, r2.Stored, r2.Duplicate)
	}
	t.Logf("读数：首投 stored=%d dup=%d · 重投 stored=%d dup=%d · 行数=%d",
		r1.Stored, r1.Duplicate, r2.Stored, r2.Duplicate, fleetLogsRowCount(t, st))
}

// ★ 判据格 2：批上限 500 —— 501 条一批 ⇒ 切多批落完；再投 ⇒ duplicate=501、行数不增。
func TestFleetLogsBatch_Cap500SplitsAndDedupes(t *testing.T) {
	h := newFleetLogsHandlers(t)
	st := h.FleetLogs
	body := batchBody("node-cap", 501, "01J9")

	w1 := postLogs(h, body)
	if w1.Code != 200 {
		t.Fatalf("501 条一批应 200（切多批），得 %d body=%.200s", w1.Code, w1.Body.String())
	}
	r1 := decodeFleetLogsResp(t, w1.Body.String())
	if r1.Stored != 501 || r1.Duplicate != 0 {
		t.Fatalf("501 条首投应 stored=501 dup=0，得 %+v", r1)
	}
	if n := fleetLogsRowCount(t, st); n != 501 {
		t.Fatalf("首投后库内应 501 行，得 %d", n)
	}

	r2 := decodeFleetLogsResp(t, postLogs(h, body).Body.String())
	if r2.Stored != 0 || r2.Duplicate != 501 {
		t.Fatalf("501 条重投应 stored=0 dup=501，得 %+v", r2)
	}
	if n := fleetLogsRowCount(t, st); n != 501 {
		t.Fatalf("重投后库内应仍 501 行，得 %d", n)
	}
	t.Logf("读数：501 条一批 · 首投 stored=%d · 重投 dup=%d · 行数=%d", r1.Stored, r2.Duplicate, fleetLogsRowCount(t, st))
}

// ★ 判据格 3：库面 —— 新库 `.schema logs` 含 event_id ＋ UNIQUE(machine, event_id)；
// **老库**（T5-c 之前建的旧形态表）⇒ 补列 ＋ 补唯一索引，且老行不丢、老行不被新行顶掉。
func TestFleetLogsSchema_EventIDUniqueAndLegacyUpgrade(t *testing.T) {
	// ① 新库
	st := newFleetLogsTestStore(t)
	var ddl string
	if err := st.db.QueryRow(`SELECT sql FROM sqlite_master WHERE type='table' AND name='logs'`).Scan(&ddl); err != nil {
		t.Fatalf("读 logs 建表语句失败: %v", err)
	}
	if !strings.Contains(ddl, "event_id TEXT") {
		t.Errorf("新库 logs 建表语句应含 `event_id TEXT`，实得：%s", ddl)
	}
	if !strings.Contains(ddl, "UNIQUE(machine, event_id)") {
		t.Errorf("新库 logs 建表语句应含 `UNIQUE(machine, event_id)`，实得：%s", ddl)
	}

	// ② 老库（旧形态：无 event_id / received_at / 无唯一约束）＋ 一行老数据
	legacy := t.TempDir() + "/fleet_logs.db"
	raw, err := sql.Open("sqlite", legacy)
	if err != nil {
		t.Fatalf("开老库失败: %v", err)
	}
	if _, err := raw.Exec(`CREATE TABLE logs (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        ts TEXT, machine TEXT, level TEXT, component TEXT, model TEXT, request_id TEXT, message TEXT
    );
    INSERT INTO logs(ts, machine, level, component, message) VALUES('2026-10-01T00:00:00Z','node-old','INFO','agent','老行');`); err != nil {
		t.Fatalf("造老库失败: %v", err)
	}
	if err := raw.Close(); err != nil {
		t.Fatalf("关老库失败: %v", err)
	}

	up, err := openFleetLogStoreAt(legacy)
	if err != nil {
		t.Fatalf("打开老库（应就地补列/补索引）失败: %v", err)
	}
	defer up.Close()
	cols := map[string]bool{}
	rows, err := up.db.Query(`PRAGMA table_info(logs)`)
	if err != nil {
		t.Fatalf("读老库表结构失败: %v", err)
	}
	for rows.Next() {
		var cid, notnull, pk int
		var name, typ string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			t.Fatalf("扫描老库表结构失败: %v", err)
		}
		cols[name] = true
	}
	rows.Close()
	if !cols["event_id"] || !cols["received_at"] {
		t.Errorf("老库应被补上 event_id / received_at 两列，实得列集 %v", cols)
	}
	var idxN int
	if err := up.db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name='uq_logs_machine_event_id'`).Scan(&idxN); err != nil {
		t.Fatalf("查老库唯一索引失败: %v", err)
	}
	if idxN != 1 {
		t.Errorf("老库应补上显式唯一索引 uq_logs_machine_event_id，命中 %d", idxN)
	}
	if n := fleetLogsRowCount(t, up); n != 1 {
		t.Errorf("老库补列后老行不该丢，应仍 1 行，得 %d", n)
	}
	// 老库补列后：带 event_id 的批量仍能去重（唯一索引真在位）。
	if _, err := up.InsertWithCounts("node-old", []FleetLogEntry{
		{TS: "2026-10-14T00:00:00Z", Level: "INFO", Component: "agent", Message: "n1", EventID: "01JA"},
	}); err != nil {
		t.Fatalf("老库补列后插新行失败: %v", err)
	}
	res, err := up.InsertWithCounts("node-old", []FleetLogEntry{
		{TS: "2026-10-14T00:00:00Z", Level: "INFO", Component: "agent", Message: "n1", EventID: "01JA"},
	})
	if err != nil {
		t.Fatalf("老库重投失败: %v", err)
	}
	if res.Stored != 0 || res.Duplicate != 1 {
		t.Errorf("老库补列后重投应 stored=0 dup=1，得 %+v", res)
	}
	t.Logf("读数：老库补列后列集=%v · 唯一索引命中=%d · 行数=%d", cols, idxN, fleetLogsRowCount(t, up))
}

// ★ 判据格 4：`received_at` = 主控收件钟（请求体送来的那一格不作数）。
func TestFleetLogsBatch_ReceivedAtIsMasterClock(t *testing.T) {
	h := newFleetLogsHandlers(t)
	st := h.FleetLogs
	body := `{"machine":"node-r","logs":[{"ts":"2026-10-14T02:00:00Z","level":"INFO","component":"agent",` +
		`"message":"x","event_id":"01JR","received_at":"1999-01-01T00:00:00Z"}]}`
	if w := postLogs(h, body); w.Code != 200 {
		t.Fatalf("上报应 200，得 %d body=%s", w.Code, w.Body.String())
	}
	var ts, ra sql.NullString
	if err := st.db.QueryRow(`SELECT ts, received_at FROM logs WHERE event_id='01JR'`).Scan(&ts, &ra); err != nil {
		t.Fatalf("读回该行失败: %v", err)
	}
	if strings.TrimSpace(ra.String) == "" {
		t.Fatalf("received_at 应由主控填（非空），实得 %q", ra.String)
	}
	if strings.HasPrefix(ra.String, "1999-") {
		t.Errorf("received_at 不该取请求体送来的值（1999-… ⇒ 信了节点/请求方），实得 %q", ra.String)
	}
	if !strings.HasPrefix(ra.String, "20") || !strings.Contains(ra.String, "T") {
		t.Errorf("received_at 应是 UTC RFC3339 文本，实得 %q", ra.String)
	}
	if ts.String != "2026-10-14T02:00:00Z" {
		t.Errorf("节点钟 ts 应原样保留（只归一不覆盖），实得 %q", ts.String)
	}
	t.Logf("读数：received_at=%s · ts=%s", ra.String, ts.String)
}

// ★ 判据格 5（时效/形态边界）：无 event_id 的行不参与去重 ⇒ 每投都算真新（写明在册）。
func TestFleetLogsBatch_NoEventIDStoresEveryTime(t *testing.T) {
	h := newFleetLogsHandlers(t)
	st := h.FleetLogs
	body := `{"machine":"node-v1","logs":[{"ts":"2026-10-14T02:00:00Z","level":"INFO","component":"agent","message":"老客户端无 event_id"}]}`
	r1 := decodeFleetLogsResp(t, postLogs(h, body).Body.String())
	r2 := decodeFleetLogsResp(t, postLogs(h, body).Body.String())
	if r1.Stored != 1 || r2.Stored != 1 || r2.Duplicate != 0 {
		t.Fatalf("无 event_id 应每投都算真新（stored=1 ×2），得 首投 %+v 重投 %+v", r1, r2)
	}
	n := fleetLogsRowCount(t, st)
	if n != 2 {
		t.Fatalf("无 event_id 两次上报应留 2 行，得 %d", n)
	}
	t.Logf("读数：无 event_id · 两次上报 stored=1/1 · 行数=%d（去重对该形态不成立 = 写明的边界）", n)
}
