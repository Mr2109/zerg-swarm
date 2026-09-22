// cli_model_adapter_test.go —— 模型面适配器（`Q-102` · 波① `T1b`）的判据机检。
//
// 病根（§自排补遗 `S2` 逐字）：`Q-102` 是**本轮唯一新开的号**却是**唯一的模型面写口**
// （`PUT /api/models/{name}/adapter-opts`）⇒ **今天只能 `curl` 绕开命令面**。
// 本件把它钉成契约：**读面只读**（`get` 一条 GET、不改任何状态）· **写面唯一**（`set` = 那一条 PUT）·
// **干跑零请求**（`--dry-run` 下合成主控的计数必须是 0）· **缺 `--yes` 不写**。
//
// 合成主控：本机随机端口 + `ZERG_PORT` 指过来（**不碰生产 8580**）；`ZERG_STATE_DIR` 指到临时目录，
// 跑前后逐件对拍（读面不许落盘）。
package main_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// adapterProbe —— 合成主控 + 计数（每次请求的 method/path/body 都记下来）。
type adapterProbe struct {
	srv      *httptest.Server
	hits     []string
	bodies   []string
	status   int
	response string
}

func newAdapterProbe(t *testing.T, status int, response string) *adapterProbe {
	t.Helper()
	p := &adapterProbe{status: status, response: response}
	p.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		p.hits = append(p.hits, r.Method+" "+r.URL.Path)
		p.bodies = append(p.bodies, string(b))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(p.status)
		fmt.Fprint(w, p.response)
	}))
	t.Cleanup(p.srv.Close)
	t.Setenv("ZERG_PORT", strings.TrimPrefix(p.srv.URL, "http://127.0.0.1:"))
	return p
}

// stateDirFiles 列出状态目录里的件（读面判据：跑前后逐件不变）。
func stateDirFiles(t *testing.T, dir string) []string {
	t.Helper()
	out := []string{}
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		out = append(out, path)
		return nil
	})
	return out
}

// 判据 ① 读面只读：`model opts get` = 一条 GET，落在契约登记的那条路径上，且状态目录一件不多。
func TestModelAdapterOptsGetIsReadOnly(t *testing.T) {
	state := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", state)
	before := stateDirFiles(t, state)
	p := newAdapterProbe(t, 200, `{"model":"m1","schema":"adapter-opts/v1","note":"本机适配器在册"}`)
	rc, out, errb := runCapture("model", "opts", "get", "m1")
	if rc != 0 {
		t.Fatalf("读面 rc=%d（要 0）· stderr=%s", rc, errb)
	}
	if len(p.hits) != 1 || p.hits[0] != "GET /api/models/m1/adapter-opts" {
		t.Fatalf("读面请求不对：%v（要恰好一条 `GET /api/models/m1/adapter-opts`）", p.hits)
	}
	for _, k := range []string{"m1", "adapter-opts/v1"} {
		if !strings.Contains(out, k) {
			t.Errorf("读面投影里没有 %q：%s", k, out)
		}
	}
	if got := stateDirFiles(t, state); len(got) != len(before) {
		t.Fatalf("读面落了盘：%v → %v", before, got)
	}
}

// 判据 ② 主控说「没适配器」⇒ **码逐字透传**（命令面不翻译、不吞），退码按契约为 2。
func TestModelAdapterOptsGetPassesCodeThrough(t *testing.T) {
	p := newAdapterProbe(t, 404, `{"error":{"type":"MODEL_NO_ADAPTER"}}`)
	rc, out, errb := runCapture("model", "opts", "get", "no-such")
	if rc != 2 {
		t.Fatalf("`MODEL_NO_ADAPTER` ⇒ rc=%d（要 2 · unsupported_on_node 那一档）", rc)
	}
	if out != "" {
		t.Fatalf("拒绝时 stdout 该空，得到 %q", out)
	}
	if !strings.Contains(errb, "MODEL_NO_ADAPTER") {
		t.Fatalf("码没逐字透传：%s", errb)
	}
	if len(p.hits) != 1 {
		t.Fatalf("请求数 %d（要 1）", len(p.hits))
	}
}

// 判据 ③ 写面唯一：`set --yes` = 恰好一条 PUT 到那条路径，请求体就是给的那些键（标量已认型）。
func TestModelAdapterOptsSetIsTheSingleWritePort(t *testing.T) {
	p := newAdapterProbe(t, 200, `{"ok":true}`)
	rc, out, errb := runCapture("model", "opts", "set", "m1", "--set", "temperature=0.8", "--set", "top_p=0.95", "--yes")
	if rc != 0 {
		t.Fatalf("写面 rc=%d（要 0）· stderr=%s", rc, errb)
	}
	if len(p.hits) != 1 || p.hits[0] != "PUT /api/models/m1/adapter-opts" {
		t.Fatalf("写面请求不对：%v（要恰好一条 PUT）", p.hits)
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(p.bodies[0]), &body); err != nil {
		t.Fatalf("请求体不是 JSON：%q", p.bodies[0])
	}
	if body["temperature"] != 0.8 || body["top_p"] != 0.95 {
		t.Fatalf("请求体不对（标量该认型）：%v", body)
	}
	if !strings.Contains(out, "m1") {
		t.Errorf("写面回执里没有模型 id：%s", out)
	}
}

// 判据 ④ 干跑零请求：`--dry-run` 一个请求都不发（计数必须是 0）—— 这是「干跑 ≠ 真跑」的牙。
func TestModelAdapterOptsSetDryRunSendsNoRequest(t *testing.T) {
	p := newAdapterProbe(t, 200, `{"ok":true}`)
	rc, out, errb := runCapture("model", "opts", "set", "m1", "--set", "temperature=0.8", "--dry-run")
	if rc != 0 {
		t.Fatalf("干跑 rc=%d（要 0）· stderr=%s", rc, errb)
	}
	if len(p.hits) != 0 {
		t.Fatalf("干跑发了 %d 个请求（要 0 · 零副作用）：%v", len(p.hits), p.hits)
	}
	if !strings.Contains(out, "PUT") || !strings.Contains(errb, "零副作用") {
		t.Fatalf("干跑计划件不完整：out=%s err=%s", out, errb)
	}
}

// 判据 ⑤ 缺 `--yes`（D2 档）⇒ 2 且**不打主控**（fail-closed）。
func TestModelAdapterOptsSetRequiresYes(t *testing.T) {
	p := newAdapterProbe(t, 200, `{"ok":true}`)
	rc, out, errb := runCapture("model", "opts", "set", "m1", "--set", "temperature=0.8")
	if rc != 2 || out != "" {
		t.Fatalf("缺 --yes：rc=%d stdout=%q（要 2 + 空）· stderr=%s", rc, out, errb)
	}
	if len(p.hits) != 0 {
		t.Fatalf("缺 --yes 却打了主控 %d 次（fail-closed 破功）", len(p.hits))
	}
}

// 判据 ⑥ 空体不给 ⇒ 2 且不打主控（主控对空体会回 `NO_PARAMS`，命令面先在本地拦住）。
func TestModelAdapterOptsSetNoParams(t *testing.T) {
	p := newAdapterProbe(t, 200, `{"ok":true}`)
	rc, out, errb := runCapture("model", "opts", "set", "m1", "--yes")
	if rc != 2 || out != "" {
		t.Fatalf("无 --set：rc=%d stdout=%q（要 2 + 空）· stderr=%s", rc, out, errb)
	}
	if !strings.Contains(errb, "NO_PARAMS") {
		t.Errorf("判词里没点名主控的 NO_PARAMS：%s", errb)
	}
	if len(p.hits) != 0 {
		t.Fatalf("空体却打了主控 %d 次", len(p.hits))
	}
}
