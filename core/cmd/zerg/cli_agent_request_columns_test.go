// cli_agent_request_columns_test.go —— `agent show` 两列（最近 N 分钟请求数 / 当前请求来源）
// 的回归牙（缺口 `GAP-20261010-16` · want③）。
//
// 为什么要它：`agent show` 的这一面改前**只有字段名、没有值**（`recent_requests` 恒空串，
// 判据 `zerg agent show edge-node --json recent_requests` 退 0 而值空 = 「面在值缺」）。真修要三件事
// 同时成立，本件逐条钉住：
//
//	① **有真值时就报真值**：子端 `/status` 回 `recent_requests` / `request_source` 时，
//	   两列逐字照报（含 `--window-min N` 真的被带成 `?window_s=<N*60>`）；
//	② **打不到时是「未知」而不是 0**：子端不在听 ⇒ 两列都 `未知`（0 会被读成「真的没请求」）；
//	③ **只增不改**：载荷原有 18 格一个不少，且人面/行式面列数 == 列名数。
//
// 判据件**不连真网**：主控走本进程内的合成主控（`newSyntheticMaster`），子端走 `httptest`，
// 名册（`gateway/fleet.yaml`）落 `t.TempDir()` 并由 `ZERG_REPO` 指过去。
package main_test

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	zerg "github.com/Mr2109/zerg-swarm/core/cmd/zerg"
)

// agentReqMasterJSON —— 合成 `GET /api/fleet/status`：一台 edge-node（两列的真实来源在子端，不在这里）。
const agentReqMasterJSON = `{"machines":{"edge-node":{"machine":"edge-node","healthy":true,"code_version":"2.5.15",` +
	`"code_sha":"abc123","cpu_pct":3.8,"gpu_pct":0,"gpu_used_gb":0.2,"vram_known":true,` +
	`"backend_state":"idle","active_requests":0,"models":["m1"],"last_seen":"2026-10-18T13:25:32+08:00"}}}`

// agentReqNamesRepo 落一份只含 `fleet:` 段的名册，并把 `ZERG_REPO` 指过去（`repoRoot()` 认它）。
func agentReqNamesRepo(t *testing.T, host string, port int) {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "gateway"), 0o755); err != nil {
		t.Fatalf("建夹具目录失败：%v", err)
	}
	body := "# 夹具（判据件用）\nfleet:\n  edge-node: { host: " + host + ", port: " + strconv.Itoa(port) + ", os: ubuntu }\n"
	if err := os.WriteFile(filepath.Join(dir, "gateway", "fleet.yaml"), []byte(body), 0o644); err != nil {
		t.Fatalf("写夹具名册失败：%v", err)
	}
	t.Setenv("ZERG_REPO", dir)
}

// agentReqRun 跑一条命令，返回 (rc, stdout, stderr)（退码由调用方判 —— 本件要测负控的 2）。
func agentReqRun(argv ...string) (int, string, string) {
	var out, errb strings.Builder
	rc := zerg.RunForTest(argv, &out, &errb)
	return rc, out.String(), errb.String()
}

// agentReqItem 从 `--json` 包封里取点名那台机的一行。
func agentReqItem(t *testing.T, stdout, machine string) map[string]string {
	t.Helper()
	var env struct {
		Items []map[string]string `json:"items"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("`--json` 不是合法包封：%v · stdout=%s", err, stdout)
	}
	for _, it := range env.Items {
		if it["machine"] == machine {
			return it
		}
	}
	t.Fatalf("包封里没有 %s：stdout=%s", machine, stdout)
	return nil
}

// agentReqClosedPort 拿一个**此刻没人在听**的端口（先听一次拿到号，再关掉）。
func agentReqClosedPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("拿空闲端口失败：%v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	if err := ln.Close(); err != nil {
		t.Fatalf("关空闲端口失败：%v", err)
	}
	return port
}

// TestAgentShowRequestColumnsUnknownWhenSubEndUnreachable —— 负控：子端打不到 ⇒ 两列「未知」，✗ 不是 0。
func TestAgentShowRequestColumnsUnknownWhenSubEndUnreachable(t *testing.T) {
	srv := newSyntheticMaster(t, map[string]string{"/api/fleet/status": agentReqMasterJSON})
	defer srv.Close()
	agentReqNamesRepo(t, "127.0.0.1", agentReqClosedPort(t))

	rc, out, errs := agentReqRun("agent", "show", "edge-node", "--json", "machine,recent_requests,request_source")
	if rc != 0 {
		t.Fatalf("`agent show` 退码 %d（要 0 —— 子端打不到只该让两列未知，✗ 不该整体失败）· stderr=%s", rc, errs)
	}
	it := agentReqItem(t, out, "edge-node")
	for _, f := range []string{"recent_requests", "request_source"} {
		if it[f] != "未知" {
			t.Fatalf("子端打不到时 `%s` 该为「未知」，实得 %q（0 会被读成「真的没请求」）· 行=%v", f, it[f], it)
		}
	}
}

// TestAgentShowRequestColumnsCountRealRequests —— 两态真测：真打一发请求 ⇒ 请求数 +1。
func TestAgentShowRequestColumnsCountRealRequests(t *testing.T) {
	srv := newSyntheticMaster(t, map[string]string{"/api/fleet/status": agentReqMasterJSON})
	defer srv.Close()

	var hits int64
	var lastWindow string
	sub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/infer":
			atomic.AddInt64(&hits, 1)
			fmt.Fprint(w, `{"ok":true}`)
		case "/status":
			lastWindow = r.URL.Query().Get("window_s")
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"machine":"edge-node","active_requests":0,"recent_requests":%d,"request_source":"<cluster-ip>"}`,
				atomic.LoadInt64(&hits))
		default:
			http.NotFound(w, r)
		}
	}))
	defer sub.Close()

	_, portStr, err := net.SplitHostPort(strings.TrimPrefix(sub.URL, "http://"))
	if err != nil {
		t.Fatalf("拆 httptest 地址失败：%v", err)
	}
	port, _ := strconv.Atoi(portStr)
	agentReqNamesRepo(t, "127.0.0.1", port)

	read := func() map[string]string {
		t.Helper()
		rc, out, errs := agentReqRun("agent", "show", "edge-node", "--json", "machine,recent_requests,request_source")
		if rc != 0 {
			t.Fatalf("`agent show` 退码 %d（要 0）· stderr=%s", rc, errs)
		}
		return agentReqItem(t, out, "edge-node")
	}

	before := read()
	if before["recent_requests"] != "0" {
		t.Fatalf("起始该为真读数 0（子端在听、真回 0），实得 %q", before["recent_requests"])
	}
	if before["request_source"] != "<cluster-ip>" {
		t.Fatalf("来源该逐字照报 `<cluster-ip>`，实得 %q", before["request_source"])
	}
	if lastWindow != "300" {
		t.Fatalf("默认窗口该带 `?window_s=300`（5 分钟），实得 %q", lastWindow)
	}

	// ★ 两态里的「打一发」：真 HTTP 打到子端 `/infer`（不是改夹具里的数）。
	resp, err := http.Get(sub.URL + "/infer")
	if err != nil {
		t.Fatalf("真打一发 /infer 失败：%v", err)
	}
	resp.Body.Close()

	after := read()
	if after["recent_requests"] != "1" {
		t.Fatalf("真打一发请求后该 +1（1），实得 %q（起始 %q）", after["recent_requests"], before["recent_requests"])
	}
	if after["recent_requests"] == before["recent_requests"] {
		t.Fatalf("两态读数未变（都是 %q）⇒ 这一列没接线", after["recent_requests"])
	}
}

// TestAgentShowRequestWindowFlag —— `--window-min N` 真被带成 `?window_s=<N*60>`；非法值 fail-closed。
func TestAgentShowRequestWindowFlag(t *testing.T) {
	srv := newSyntheticMaster(t, map[string]string{"/api/fleet/status": agentReqMasterJSON})
	defer srv.Close()

	var lastWindow string
	sub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/status" {
			http.NotFound(w, r)
			return
		}
		lastWindow = r.URL.Query().Get("window_s")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"machine":"edge-node","active_requests":0,"recent_requests":2,"request_source":"Mr2109"}`)
	}))
	defer sub.Close()

	_, portStr, _ := net.SplitHostPort(strings.TrimPrefix(sub.URL, "http://"))
	port, _ := strconv.Atoi(portStr)
	agentReqNamesRepo(t, "127.0.0.1", port)

	if rc, _, errs := agentReqRun("agent", "show", "edge-node", "--window-min", "1"); rc != 0 {
		t.Fatalf("`--window-min 1` 退码 %d（要 0）· stderr=%s", rc, errs)
	}
	if lastWindow != "60" {
		t.Fatalf("`--window-min 1` 该带 `?window_s=60`，实得 %q", lastWindow)
	}

	for _, bad := range []string{"0", "-3", "abc", "99999"} {
		rc, out, errs := agentReqRun("agent", "show", "edge-node", "--window-min", bad)
		if rc != 2 {
			t.Fatalf("`--window-min %s` 该 fail-closed 退 2，实得 %d（stdout=%q stderr=%q）", bad, rc, out, errs)
		}
		if !strings.Contains(errs, "--window-min") {
			t.Fatalf("`--window-min %s` 的拒执理由该点名旗标，实得 stderr=%q", bad, errs)
		}
	}

	// 裸形（给了但没带值）也 fail-closed —— 「没给」与「给了空值」是两件事。
	rc, _, errs := agentReqRun("agent", "show", "edge-node", "--window-min")
	if rc != 2 || !strings.Contains(errs, "--window-min") {
		t.Fatalf("`--window-min` 裸形该退 2 并点名，实得 rc=%d stderr=%q", rc, errs)
	}
}

// TestAgentShowPayloadFieldsUnchanged —— 只增不改：原有 18 格一个不少，行/列数对得上。
func TestAgentShowPayloadFieldsUnchanged(t *testing.T) {
	srv := newSyntheticMaster(t, map[string]string{"/api/fleet/status": agentReqMasterJSON})
	defer srv.Close()
	agentReqNamesRepo(t, "127.0.0.1", agentReqClosedPort(t))

	oldFields := []string{"machine", "healthy", "code_version", "code_sha", "origin_sha", "same_origin",
		"cpu_pct", "gpu_pct", "mem_available_gb", "mem_total_gb", "models", "model", "backend_state",
		"active_requests", "gpu_used_gb", "backend_rss_gb", "gpu_temp_c", "last_seen"}
	rc, out, errs := agentReqRun("agent", "show", "edge-node", "--json", strings.Join(oldFields, ","))
	if rc != 0 {
		t.Fatalf("旧 18 格 `--json` 合计该仍退 0（只增不改），实得 %d · stderr=%s", rc, errs)
	}
	it := agentReqItem(t, out, "edge-node")
	for _, f := range oldFields {
		if _, ok := it[f]; !ok && f != "origin_sha" && f != "same_origin" {
			// 载荷缺的格按既有口径整键略去（`emitSelected` 的「登记即认」）—— 只要求「不报错」。
			t.Fatalf("旧字段 `%s` 从 `--json` 面消失了（行=%v）", f, it)
		}
	}
	if it["machine"] != "edge-node" || it["code_version"] != "2.5.15" || it["active_requests"] != "0" {
		t.Fatalf("旧字段值不该被这次改动波及：实得 %v", it)
	}

	// 行式面（非 TTY）：列名数 == 数据格数，且两列都在表头里。
	rc, out, errs = agentReqRun("agent", "show", "edge-node")
	if rc != 0 {
		t.Fatalf("人面/行式面退码 %d（要 0）· stderr=%s", rc, errs)
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("行式面该是「表头 + 一行」，实得 %d 行：%q", len(lines), out)
	}
	cols := strings.Split(lines[0], "\t")
	if len(cols) != 20 {
		t.Fatalf("列数该为 20（18 旧 + 2 新），实得 %d：%q", len(cols), lines[0])
	}
	if !strings.Contains(lines[0], "recent_requests") || !strings.Contains(lines[0], "request_source") {
		t.Fatalf("表头缺新两列：%q", lines[0])
	}
	if n := len(strings.Split(lines[1], "\t")); n != len(cols) {
		t.Fatalf("数据行格数 %d ≠ 表头格数 %d（行=%q）", n, len(cols), lines[1])
	}
}
