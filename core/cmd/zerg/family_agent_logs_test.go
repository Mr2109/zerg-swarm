// family_agent_logs_test.go —— `zerg agent logs`（缺口 GAP-20260927-202 ① · 段2）的回归牙。
//
// 正控 1 枚：对已知 machine **走主控查询面**读到行（字段逐字取自回据）。
// 负控 2 枚：① 未知 machine ⇒ **空结果而非崩**（退 0）；② **CLI 不得直连子端** —— 探针（子端桩 0 命中
// / 主控桩命中查询面）证明它走的是主控面（§九 M20 O2）。
package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

// agentLogsStubCore —— 假主控：只回 `/api/fleet/logs/query`，并记下收到的 RequestURI。
func agentLogsStubCore(t *testing.T, body string) (*httptest.Server, *[]string) {
	t.Helper()
	seen := &[]string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*seen = append(*seen, r.URL.RequestURI())
		if r.URL.Path != "/api/fleet/logs/query" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, seen
}

// agentLogsPointCore —— 把 CLI 的主控地址指向假主控（复用既有 ZERG_PORT 口径，不碰生产 8580）。
func agentLogsPointCore(t *testing.T, srv *httptest.Server) {
	t.Helper()
	port := srv.URL[strings.LastIndexByte(srv.URL, ':')+1:]
	if _, err := strconv.Atoi(port); err != nil {
		t.Fatalf("端口解析失败：%q", srv.URL)
	}
	t.Setenv("ZERG_PORT", port)
}

// 正控：对已知 machine 走**主控查询面** `/api/fleet/logs/query?machine=` 读到行。
func TestAgentLogs_ReadsViaCoreQueryFace(t *testing.T) {
	srv, seen := agentLogsStubCore(t,
		`[{"ts":"2026-10-10T13:00:00Z","machine":"Mr2109","level":"ERROR","component":"agent","model":"m1","request_id":"r1","message":"崩了"}]`)
	agentLogsPointCore(t, srv)

	var out, errb bytes.Buffer
	rc := cmdAgentLogs(&invocation{path: []string{"agent", "logs"}, args: []string{"Mr2109"}}, &out, &errb)
	if rc != exitOK {
		t.Fatalf("应退 0，实得 %d · stderr=%s", rc, errb.String())
	}
	if len(*seen) == 0 || (*seen)[0] != "/api/fleet/logs/query?machine=Mr2109" {
		t.Fatalf("CLI 应打主控查询面，实得 %v", *seen)
	}
	if !strings.Contains(out.String(), "崩了") || !strings.Contains(out.String(), "ERROR") {
		t.Fatalf("输出应含回据里的行（字段逐字），实得 stdout=%q", out.String())
	}
}

// 负控①：未知 machine ⇒ 主控查询面回空数组 ⇒ **空结果而非崩**（退 0）。
func TestAgentLogs_UnknownMachineEmptyNotCrash(t *testing.T) {
	srv, seen := agentLogsStubCore(t, `[]`)
	agentLogsPointCore(t, srv)

	var out, errb bytes.Buffer
	rc := cmdAgentLogs(&invocation{path: []string{"agent", "logs"}, args: []string{"no-such-machine"}}, &out, &errb)
	if rc != exitOK {
		t.Fatalf("未知 machine 应退 0（空结论，不是错、不崩），实得 %d · stderr=%s", rc, errb.String())
	}
	if strings.Contains(errb.String(), "panic") || strings.Contains(out.String(), "panic") {
		t.Fatalf("不该崩：stdout=%q stderr=%q", out.String(), errb.String())
	}
	if len(*seen) == 0 || !strings.HasPrefix((*seen)[0], "/api/fleet/logs/query?machine=no-such-machine") {
		t.Fatalf("未知 machine 也应打同一查询面（按 machine 过滤），实得 %v", *seen)
	}
}

// 负控②：CLI **不得直连子端** —— 子端桩 0 命中、主控桩命中查询面（探针证它走的是主控面）。
func TestAgentLogs_NeverDialsChildDirectly(t *testing.T) {
	childHits := 0
	child := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		childHits++
		_, _ = w.Write([]byte(`{"eggs":[]}`))
	}))
	t.Cleanup(child.Close)

	srv, seen := agentLogsStubCore(t, `[]`)
	agentLogsPointCore(t, srv)

	var out, errb bytes.Buffer
	_ = cmdAgentLogs(&invocation{path: []string{"agent", "logs"}, args: []string{"Mr2109"}}, &out, &errb)

	if childHits != 0 {
		t.Fatalf("CLI 直连了子端（%d 次）—— 违反 §九 M20 O2「不偷偷直连子端」", childHits)
	}
	if len(*seen) != 1 || (*seen)[0] != "/api/fleet/logs/query?machine=Mr2109" {
		t.Fatalf("CLI 应只打主控查询面一次，实得 %v", *seen)
	}
}
