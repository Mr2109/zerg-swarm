// reporter_test.go —— 子端批量上报（GAP-20260927-202 ① · 段2）的定向单测。
//
// 正控 1 枚（假主控收到上报载荷、字段逐字照 protocol.md；offset 推进、不重复上报）
// ＋ 负控 2 枚（主控不可达 ⇒ **不静默吞**：出声 + 计数 + 游标不推进；无本机日志源 ⇒ 点名一次）
// ＋ 解析面 1 枚（logx 行 → 协议条目逐字）。
package fleetlogs

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/agent/internal/logx"
)

// writeLog 追加若干行 logx 形态日志（逐字照 logx.logf 的 `%s %-5s [%-10s] %s %s`）。
func writeLog(t *testing.T, path string, lines ...string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatalf("开日志源失败: %v", err)
	}
	defer f.Close()
	for _, l := range lines {
		if _, err := f.WriteString(l + "\n"); err != nil {
			t.Fatalf("写日志源失败: %v", err)
		}
	}
}

// withLogxFile 把 logx 的输出指到临时文件（读回「出声」证据），测试结束复原。
func withLogxFile(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "logx.log")
	if err := logx.SetFile(p); err != nil {
		t.Fatalf("logx.SetFile 失败: %v", err)
	}
	t.Cleanup(func() { _ = logx.SetFile("") })
	return p
}

// 正控：假主控收到 POST /api/fleet/logs，载荷（顶层 machine ＋ 每格字段）逐字照 protocol.md；
// offset 推进到文件末尾；失败计数 0；已上报的行不再重复上报。
func TestReporter_PostsBatchVerbatim(t *testing.T) {
	var got []byte
	var gotPath, gotToken string
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		gotPath = r.URL.Path
		gotToken = r.Header.Get("X-Auth-Token")
		got, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true,"stored":2}`))
	}))
	t.Cleanup(srv.Close)

	logPath := filepath.Join(t.TempDir(), "zerg-agent.log")
	writeLog(t, logPath,
		"2026-10-10T13:00:00.000+08:00 INFO  [agent     ] 子端启动 version=2.5.14",
		"2026-10-10T13:00:05.000+08:00 ERROR [backend   ] 引擎崩溃 model=example-35b request_id=req-9",
	)

	r := NewReporter(srv.URL, "tok-xyz", "node-a", logPath)
	r.mu.Lock()
	r.offset = 0 // 从 0 起读整份夹具（Start 默认置游标到末尾）
	r.mu.Unlock()

	r.RunOnce()

	if hits != 1 {
		t.Fatalf("假主控应收到 1 次 POST，实得 %d", hits)
	}
	if gotPath != "/api/fleet/logs" {
		t.Fatalf("上报路径应为 /api/fleet/logs（protocol.md:62），实得 %q", gotPath)
	}
	if gotToken != "tok-xyz" {
		t.Fatalf("X-Auth-Token 应逐字带上，实得 %q", gotToken)
	}
	var p struct {
		Machine string  `json:"machine"`
		Logs    []Entry `json:"logs"`
	}
	if err := json.Unmarshal(got, &p); err != nil {
		t.Fatalf("上报体不是合法 JSON：%v（%s）", err, got)
	}
	if p.Machine != "node-a" {
		t.Fatalf("顶层 machine 应为 node-a（protocol.md:64），实得 %q", p.Machine)
	}
	if len(p.Logs) != 2 {
		t.Fatalf("应上报 2 条，实得 %d（%+v）", len(p.Logs), p.Logs)
	}
	if e := p.Logs[0]; e.TS != "2026-10-10T13:00:00.000+08:00" || e.Level != "INFO" || e.Component != "agent" || e.Message != "子端启动 version=2.5.14" {
		t.Fatalf("第 0 条字段不逐字：%+v", e)
	}
	if e := p.Logs[1]; e.Level != "ERROR" || e.Component != "backend" || e.Model != "example-35b" || e.RequestID != "req-9" {
		t.Fatalf("第 1 条字段不逐字：%+v", e)
	}
	if r.FailCount() != 0 {
		t.Fatalf("成功上报后失败计数应为 0，实得 %d", r.FailCount())
	}
	if r.Offset() == 0 {
		t.Fatalf("成功上报后游标应推进，实得 0")
	}
	before := hits
	r.RunOnce()
	if hits != before {
		t.Fatalf("已上报的行不该重复上报（游标未推进）")
	}
}

// 负控①：主控不可达 ⇒ **不静默吞**（出声 + 计数），游标不推进（下轮仍能重试同一批）。
func TestReporter_UnreachableNotSilent(t *testing.T) {
	logFile := withLogxFile(t)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("占位监听失败：%v", err)
	}
	dead := "http://" + ln.Addr().String()
	_ = ln.Close() // 立刻放掉 ⇒ 无人应答（主控不可达）

	logPath := filepath.Join(t.TempDir(), "zerg-agent.log")
	writeLog(t, logPath, "2026-10-10T13:00:00.000+08:00 INFO  [agent     ] 一行")

	r := NewReporter(dead, "tok", "node-a", logPath)
	r.mu.Lock()
	r.offset = 0
	r.mu.Unlock()

	r.RunOnce()

	if r.FailCount() != 1 {
		t.Fatalf("上报失败应计 1 次（不静默吞），实得 %d", r.FailCount())
	}
	if r.Offset() != 0 {
		t.Fatalf("失败后游标不该推进（条目不丢），实得 offset=%d", r.Offset())
	}
	ents, _, _ := r.scan()
	if len(ents) != 1 {
		t.Fatalf("失败后应仍可重试读到 1 条，实得 %d", len(ents))
	}
	b, _ := os.ReadFile(logFile)
	if !strings.Contains(string(b), "日志上报失败") {
		t.Fatalf("上报失败必须出声（logx），实得日志：%s", string(b))
	}
}

// 负控②：无本机日志源（`--log-file` 空）⇒ 点名一次、不上报（不静默当无事）。
func TestReporter_NoSourceSpeaksUp(t *testing.T) {
	logFile := withLogxFile(t)
	r := NewReporter("http://127.0.0.1:1", "tok", "node-a", "")
	r.Start() // 无源 ⇒ 点名后返回，不空转
	r.Stop()
	b, _ := os.ReadFile(logFile)
	if !strings.Contains(string(b), "无本机日志源") {
		t.Fatalf("无本机日志源必须出声，实得：%s", string(b))
	}
	if r.FailCount() != 0 {
		t.Fatalf("无源不该计失败，实得 %d", r.FailCount())
	}
}

// 解析面：logx 行 → 协议条目逐字（ts/level/component 逐字；model/request_id 从行内字段取）。
func TestParseLogLine(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want Entry
	}{
		{"普通行", "2026-10-10T13:00:00.000+08:00 INFO  [agent     ] 子端启动 version=2.5.14",
			Entry{TS: "2026-10-10T13:00:00.000+08:00", Level: "INFO", Component: "agent", Message: "子端启动 version=2.5.14"}},
		{"带 model/request_id", "2026-10-10T13:00:05.000+08:00 ERROR [backend   ] 崩了 model=m1 request_id=r1",
			Entry{TS: "2026-10-10T13:00:05.000+08:00", Level: "ERROR", Component: "backend", Model: "m1", RequestID: "r1", Message: "崩了 model=m1 request_id=r1"}},
	}
	for _, c := range cases {
		got, ok := ParseLogLine(c.in)
		if !ok || got != c.want {
			t.Errorf("%s：got=%+v ok=%v want=%+v", c.name, got, ok, c.want)
		}
	}
	if _, ok := ParseLogLine("没有空格的整行"); ok {
		t.Fatalf("不成形的行应解析失败（跳过）")
	}
}
