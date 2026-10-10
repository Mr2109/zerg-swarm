package api

// fleet_logs_store_test.go — 机群日志面（GAP-20260927-202 ① · 段1）的正控与负控。
//
// 库一律落临时目录（openFleetLogStoreAt(t.TempDir()...)），✗ 绝不碰生产状态库。

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newFleetLogsTestStore — 临时库（每用例独占）。
func newFleetLogsTestStore(t *testing.T) *FleetLogStore {
	t.Helper()
	st, err := openFleetLogStoreAt(filepath.Join(t.TempDir(), "fleet_logs.db"))
	if err != nil {
		t.Fatalf("打开临时 fleet 日志库失败: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

// newFleetLogsHandlers — 注入临时库的 Handlers。
func newFleetLogsHandlers(t *testing.T) *Handlers {
	t.Helper()
	h := newTestHandlers()
	h.FleetLogs = newFleetLogsTestStore(t)
	return h
}

func postLogs(h *Handlers, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/fleet/logs", bytes.NewReader([]byte(body)))
	w := httptest.NewRecorder()
	h.LogsHandler(w, req)
	return w
}

// 正控：往临时库写一条上报 ⇒ query 查得到、字段逐字相符。
func TestFleetLogs_ReportThenQuery(t *testing.T) {
	h := newFleetLogsHandlers(t)
	body := `{"machine":"node-a","logs":[{"ts":"2026-08-07T15:00:00Z","level":"INFO","component":"agent","model":"deepseek-v4-flash","request_id":"uuid-1","message":"hello"}]}`
	if w := postLogs(h, body); w.Code != http.StatusOK {
		t.Fatalf("上报应 200，得 %d body=%s", w.Code, w.Body.String())
	}

	qr := httptest.NewRequest(http.MethodGet, "/api/fleet/logs/query?machine=node-a", nil)
	qw := httptest.NewRecorder()
	h.FleetLogsQueryHandler(qw, qr)
	if qw.Code != http.StatusOK {
		t.Fatalf("query 应 200，得 %d body=%s", qw.Code, qw.Body.String())
	}
	var rows []map[string]any
	if err := json.Unmarshal(qw.Body.Bytes(), &rows); err != nil {
		t.Fatalf("query 返回非 JSON 数组: %v body=%s", err, qw.Body.String())
	}
	if len(rows) != 1 {
		t.Fatalf("应查到 1 条，得 %d（body=%s）", len(rows), qw.Body.String())
	}
	want := map[string]string{
		"ts": "2026-08-07T15:00:00Z", "machine": "node-a", "level": "INFO",
		"component": "agent", "model": "deepseek-v4-flash", "request_id": "uuid-1", "message": "hello",
	}
	for k, v := range want {
		if got, _ := rows[0][k].(string); got != v {
			t.Errorf("字段 %s 应逐字相符：期望 %q 得 %q", k, v, got)
		}
	}

	// trace / errors / stats 三面同源取数。
	tw := httptest.NewRecorder()
	h.FleetLogsTraceHandler(tw, httptest.NewRequest(http.MethodGet, "/api/fleet/logs/trace?request_id=uuid-1", nil))
	var traced []map[string]any
	if err := json.Unmarshal(tw.Body.Bytes(), &traced); err != nil || len(traced) != 1 {
		t.Fatalf("trace 应查到 1 条：err=%v body=%s", err, tw.Body.String())
	}
	sw := httptest.NewRecorder()
	h.FleetLogsStatsHandler(sw, httptest.NewRequest(http.MethodGet, "/api/fleet/logs/stats?model=deepseek-v4-flash", nil))
	var stats map[string]any
	if err := json.Unmarshal(sw.Body.Bytes(), &stats); err != nil {
		t.Fatalf("stats 返回非 JSON 对象: %v body=%s", err, sw.Body.String())
	}
	if n, _ := stats["INFO"].(float64); n != 1 {
		t.Errorf("stats INFO 应为 1，得 %v（body=%s）", stats["INFO"], sw.Body.String())
	}
}

// 负控 1：缺必填字段的上报 ⇒ 按协议档拒（400）并点名。
func TestFleetLogs_MissingRequiredFieldRejected(t *testing.T) {
	h := newFleetLogsHandlers(t)
	cases := []struct{ name, body, wantField string }{
		{"缺 machine", `{"logs":[{"ts":"T","level":"INFO","component":"agent"}]}`, "machine"},
		{"缺 logs", `{"machine":"node-a"}`, "logs"},
		{"logs 空数组", `{"machine":"node-a","logs":[]}`, "logs"},
		{"缺 ts", `{"machine":"node-a","logs":[{"level":"INFO","component":"agent"}]}`, "logs[0].ts"},
		{"缺 level", `{"machine":"node-a","logs":[{"ts":"T","component":"agent"}]}`, "logs[0].level"},
		{"缺 component", `{"machine":"node-a","logs":[{"ts":"T","level":"INFO"}]}`, "logs[0].component"},
	}
	for _, tc := range cases {
		w := postLogs(h, tc.body)
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s：应 400 拒收，得 %d body=%s", tc.name, w.Code, w.Body.String())
			continue
		}
		if !strings.Contains(w.Body.String(), tc.wantField) {
			t.Errorf("%s：拒话应点名 %q，得 %s", tc.name, tc.wantField, w.Body.String())
		}
	}
}

// 负控 2：库取不到/写不进 ⇒ 不给结论（非 0），✗ 不静默吞。
func TestFleetLogs_StoreUnavailableFailClosed(t *testing.T) {
	// 前置：父路径是普通文件 ⇒ 打开库必然失败（真红前置）。
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := openFleetLogStoreAt(filepath.Join(blocker, "fleet_logs.db")); err == nil {
		t.Fatal("前置不成立：父路径是普通文件，打开库本应失败")
	}

	// 未初始化的库（db==nil）注入 ⇒ 写不进 ⇒ 上报不得报成功。
	h := newTestHandlers()
	h.FleetLogs = &FleetLogStore{}
	if w := postLogs(h, `{"machine":"n","logs":[{"ts":"T","level":"INFO","component":"a"}]}`); w.Code == http.StatusOK {
		t.Fatalf("库不可用竟报成功（静默吞）：code=%d body=%s", w.Code, w.Body.String())
	}
	// 查询面同样 fail-closed。
	qw := httptest.NewRecorder()
	h.FleetLogsQueryHandler(qw, httptest.NewRequest(http.MethodGet, "/api/fleet/logs/query?machine=n", nil))
	if qw.Code == http.StatusOK {
		t.Fatalf("库不可用 query 竟报成功：code=%d body=%s", qw.Code, qw.Body.String())
	}
}

// 负控 3：查询未知 machine ⇒ 空结果（[]）而非报错崩。
func TestFleetLogs_UnknownMachineEmptyResult(t *testing.T) {
	h := newFleetLogsHandlers(t)
	postLogs(h, `{"machine":"node-a","logs":[{"ts":"2026-08-07T15:00:00Z","level":"INFO","component":"agent"}]}`)

	w := httptest.NewRecorder()
	h.FleetLogsQueryHandler(w, httptest.NewRequest(http.MethodGet, "/api/fleet/logs/query?machine=does-not-exist", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("未知 machine 应 200，得 %d body=%s", w.Code, w.Body.String())
	}
	body := strings.TrimSpace(w.Body.String())
	if !strings.HasPrefix(body, "[") {
		t.Fatalf("未知 machine 应空数组而非报错对象，得 %s", body)
	}
	var rows []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &rows); err != nil {
		t.Fatalf("未知 machine 返回非 JSON 数组：%v body=%s", err, body)
	}
	if len(rows) != 0 {
		t.Fatalf("未知 machine 应 0 条，得 %d", len(rows))
	}
}

// 负控 2b：库**取不到**（打开失败）⇒ 同样 fail-closed（非 0），✗ 不静默吞。
func TestFleetLogs_StoreOpenFailureFailClosed(t *testing.T) {
	old := fleetLogsOpenFn
	fleetLogsOpenFn = func() (*FleetLogStore, error) { return nil, fmt.Errorf("模拟：状态目录取不到") }
	defer func() { fleetLogsOpenFn = old }()

	h := newTestHandlers() // FleetLogs 未注入 ⇒ 走惰性打开（此处被替换为必失败）
	if w := postLogs(h, `{"machine":"n","logs":[{"ts":"T","level":"INFO","component":"a"}]}`); w.Code == http.StatusOK {
		t.Fatalf("库取不到竟报成功（静默吞）：code=%d body=%s", w.Code, w.Body.String())
	}
	qw := httptest.NewRecorder()
	h.FleetLogsQueryHandler(qw, httptest.NewRequest(http.MethodGet, "/api/fleet/logs/query?machine=n", nil))
	if qw.Code == http.StatusOK {
		t.Fatalf("库取不到 query 竟报成功：code=%d", qw.Code)
	}
}

// 第四条查询面 errors：只看 ERROR（空 since = 不过滤）。
func TestFleetLogs_ErrorsFace(t *testing.T) {
	h := newFleetLogsHandlers(t)
	postLogs(h, `{"machine":"node-a","logs":[`+
		`{"ts":"2026-08-07T15:00:00Z","level":"INFO","component":"agent","message":"ok"},`+
		`{"ts":"2026-08-07T15:01:00Z","level":"ERROR","component":"backend","message":"boom"}]}`)

	w := httptest.NewRecorder()
	h.FleetLogsErrorsHandler(w, httptest.NewRequest(http.MethodGet, "/api/fleet/logs/errors", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("errors 应 200，得 %d body=%s", w.Code, w.Body.String())
	}
	var rows []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &rows); err != nil {
		t.Fatalf("errors 返回非 JSON 数组：%v body=%s", err, w.Body.String())
	}
	if len(rows) != 1 || rows[0]["level"] != "ERROR" {
		t.Fatalf("errors 应只回 1 条 ERROR，得 %s", w.Body.String())
	}
}
