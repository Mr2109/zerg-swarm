// family_agent_forget_test.go —— `zerg agent forget <机>`（`cmdAgentForget` · 缺口 GAP-20261010-21 B 单）的回归牙。
//
// 正控：`--yes` 打到主控端点 `POST /api/fleet/forget`，且请求体**逐字** `{"machine":"<机>","confirm":true}` ⇒ rc=0。
// 负控（**真红**）：① `--dry-run` ⇒ rc=0 且**一个请求都不发**（零副作用 —— 本地计划件）·
//
//	② 缺 `--yes` ⇒ rc=2 且不发请求（fail-closed）· ③ `--dry-run` 与 `--yes` 同给 ⇒ rc=2（两道确认档自相矛盾）。
//
// ★ 不碰真主控（`ZERG_PORT` 指本机合成 server）· ✗ 不真摘除真机 · ✗ 不重启服务。
package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// forgetStub —— 假主控：只回 `POST /api/fleet/forget`，逐发记下「方法 路径 请求体」。
func forgetStub(t *testing.T) (*httptest.Server, *[]string) {
	t.Helper()
	var mu sync.Mutex
	seen := &[]string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/fleet/forget" {
			http.NotFound(w, r)
			return
		}
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r.Body)
		mu.Lock()
		*seen = append(*seen, r.Method+" "+r.URL.Path+" "+buf.String())
		mu.Unlock()
		var req map[string]any
		_ = json.Unmarshal(buf.Bytes(), &req)
		machine, _ := req["machine"].(string)
		confirmed, _ := req["confirm"].(bool)
		present := machine == "ms02"
		out := map[string]any{
			"dry_run": false, "machine": machine, "present": present,
			"changed": confirmed && present, "would_forget": confirmed && present,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(out)
	}))
	t.Cleanup(srv.Close)
	return srv, seen
}

// forgetPointCore —— 把 CLI 的主控地址指向假主控（复用既有 ZERG_PORT 口径，不碰生产 8580）。
func forgetPointCore(t *testing.T, srv *httptest.Server) {
	t.Helper()
	port := srv.URL[strings.LastIndexByte(srv.URL, ':')+1:]
	if _, err := strconv.Atoi(port); err != nil {
		t.Fatalf("端口解析失败：%q", srv.URL)
	}
	t.Setenv("ZERG_PORT", port)
}

// forgetInv 装配一次 `agent forget <机> …` 的 invocation（与解析面同形：手工落 args / --dry-run / --yes）。
func forgetInv(machine string, dryRun, yes bool) *invocation {
	inv := &invocation{path: []string{"agent", "forget"}}
	if machine != "" {
		inv.args = []string{machine}
	}
	inv.dryRun = dryRun
	inv.yes = yes
	return inv
}

// TestAgentForgetRealPathSendsConfirmTrue —— 正控：`--yes` 打端点且请求体逐字含 confirm:true ⇒ rc=0。
func TestAgentForgetRealPathSendsConfirmTrue(t *testing.T) {
	srv, seen := forgetStub(t)
	forgetPointCore(t, srv)
	var out, errb bytes.Buffer
	rc := cmdAgentForget(forgetInv("ms02", false, true), &out, &errb)
	if rc != exitOK {
		t.Fatalf("应退 0，实得 %d · stderr=%s", rc, errb.String())
	}
	if len(*seen) != 1 {
		t.Fatalf("应恰好打一次端点，实得 %d 次：%v", len(*seen), *seen)
	}
	want := `POST /api/fleet/forget {"machine":"ms02","confirm":true}`
	if (*seen)[0] != want {
		t.Fatalf("请求面应逐字 %q，实得 %q", want, (*seen)[0])
	}
}

// TestAgentForgetDryRunStaysLocal —— 负控①：`--dry-run` ⇒ rc=0 且**一个请求都不发**（零副作用 · 本地计划件）。
func TestAgentForgetDryRunStaysLocal(t *testing.T) {
	srv, seen := forgetStub(t)
	forgetPointCore(t, srv)
	var out, errb bytes.Buffer
	rc := cmdAgentForget(forgetInv("ms02", true, false), &out, &errb)
	if rc != exitOK {
		t.Fatalf("干跑应退 0，实得 %d · stderr=%s", rc, errb.String())
	}
	if len(*seen) != 0 {
		t.Fatalf("干跑**不许**发请求（零副作用），实得：%v", *seen)
	}
	if !strings.Contains(out.String(), "/api/fleet/forget") {
		t.Fatalf("计划件应点名端点，实得 stdout=%q", out.String())
	}
}

// TestAgentForgetMissingYesFailsClosed —— 负控②：缺 `--yes` ⇒ rc=2 且不发请求（stdout 0 字节）。
func TestAgentForgetMissingYesFailsClosed(t *testing.T) {
	srv, seen := forgetStub(t)
	forgetPointCore(t, srv)
	var out, errb bytes.Buffer
	rc := cmdAgentForget(forgetInv("ms02", false, false), &out, &errb)
	if rc != exitUsage {
		t.Fatalf("缺 --yes 应退 2，实得 %d", rc)
	}
	if len(*seen) != 0 {
		t.Fatalf("缺 --yes 不许发请求，实得：%v", *seen)
	}
	if out.Len() != 0 {
		t.Fatalf("缺 --yes ⇒ stdout 应 0 字节，实得 %d", out.Len())
	}
}

// TestAgentForgetDryRunYesConflict —— 负控③：`--dry-run` 与 `--yes` 同给 ⇒ rc=2（自相矛盾 · 不发请求）。
func TestAgentForgetDryRunYesConflict(t *testing.T) {
	srv, seen := forgetStub(t)
	forgetPointCore(t, srv)
	var out, errb bytes.Buffer
	rc := cmdAgentForget(forgetInv("ms02", true, true), &out, &errb)
	if rc != exitUsage {
		t.Fatalf("两档同给应退 2，实得 %d", rc)
	}
	if len(*seen) != 0 {
		t.Fatalf("两档同给不许发请求，实得：%v", *seen)
	}
}
