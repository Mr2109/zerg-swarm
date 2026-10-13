// wall_ping_identity_test.go —— `zerg agent ping` 探活面「谁答的」＝**对端自报且可核**
// （缺口 GAP-20260925-20）的回归牙。
//
// 病根（改前）：`--direct` 只做一次 TCP 连接即关闭（**零协议交互**）⇒ 结构上不可能知道对端是谁；
// 回显只能拿**调用方输入的地址 / 名册名**顶替 ⇒「声称 A · 实答 B」无从分辨，外部服务占了端口也
// 照报 `reachable=true`。治本：连上后向对端**只读面** `GET /ready` 读一次**自报身份**，收方只
// **校验并透传**，再与本方**声称**的机器名对照。本件判四档（含成对负控）：
//
//	① 声称≠自报 ⇒ `identity=mismatch` 且 `machine` = **自报值**（✗ 不许回落成声称值 —— 改前的病）
//	② 反例（声称==自报）⇒ `identity=verified`
//	③ 对端未自报（401 外部服务 / 非 JSON / 虫族形状但无 machine）⇒ `identity=unverified`、
//	   `machine=（未自报）`（✗ 任何一档都不拿声称值顶替）
//	④ 纯判据：`parsePeerSelfReport` 六例（信封校验）+ `answeredByOf` 五例（判词 + 「claimed 不许占答话者位」）
//
// 全程**不连真网**：对端是本进程内的 `httptest` 合成子端；名册落在 `t.TempDir()`。
package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// wpRoster 造一份最小名册并把它设为 `$HOME` —— `--direct` 档进门要读 `~/.zerg/contexts/default.yaml`
// （内容与本判据无关，只为过前置闸）。
func wpRoster(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	dir := filepath.Join(home, ".zerg", "contexts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("建名册目录: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "default.yaml"),
		[]byte("context: default\nnodes:\n  edge-node:\n  mini01:\n"), 0o644); err != nil {
		t.Fatalf("写名册: %v", err)
	}
	t.Setenv("HOME", home)
}

// wpPeer 起一个**合成对端**：只有 `/ready` 回给定的状态码与体（其余路径 404）。
func wpPeer(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ready" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// wpPing 走**真实现**（`cmdAgentPing` 直连档 + `--json`），返回退码与包封里的第一条。
func wpPing(t *testing.T, claimed, addr string) (int, map[string]string) {
	t.Helper()
	var out, errb strings.Builder
	inv := &invocation{
		path:      []string{"agent", "ping"},
		args:      []string{claimed, addr},
		direct:    true,
		jsonGiven: true,
		fields:    []string{"machine", "answered_by", "via"},
	}
	rc := cmdAgentPing(inv, &out, &errb)
	if rc != 0 {
		return rc, nil
	}
	var env struct {
		Items []map[string]string `json:"items"`
	}
	if err := json.Unmarshal([]byte(out.String()), &env); err != nil {
		t.Fatalf("`--json` 不是合法包封：%v · stdout=%q", err, out.String())
	}
	for _, it := range env.Items {
		return rc, it
	}
	t.Fatalf("包封里没有条目：stdout=%q", out.String())
	return rc, nil
}

// wpZergReadyBody —— 虫族子端 `/ready` 的**自报**信封（含 `machine` = 本进程自报的机器名）。
const wpZergReadyBody = `{"machine":"mini01","ready":true,"phase":"HTTP 已监听","elapsed_ms":1,"phases":[]}`

// TestAgentPingDirect_ClaimedVsReportedMismatch —— 判据①②：声称≠自报当场可见；反例（相符）通过。
func TestAgentPingDirect_ClaimedVsReportedMismatch(t *testing.T) {
	wpRoster(t)
	srv := wpPeer(t, http.StatusOK, wpZergReadyBody)
	addr := srv.Listener.Addr().String()

	rc, row := wpPing(t, "edge-node", addr) // 声称 edge-node，对端自报 mini01
	if rc != 0 {
		t.Fatalf("退码 %d（要 0）", rc)
	}
	if !strings.Contains(row["answered_by"], "identity=mismatch") {
		t.Fatalf("「声称 A · 实答 B」未被标出：answered_by=%q", row["answered_by"])
	}
	if row["machine"] != "mini01" {
		t.Fatalf("machine 必须是**对端自报**的 mini01，实得 %q（回落成声称值 = 改前的病）", row["machine"])
	}

	rc2, row2 := wpPing(t, "mini01", addr) // 反例：声称 == 自报
	if rc2 != 0 {
		t.Fatalf("反例退码 %d（要 0）", rc2)
	}
	if !strings.Contains(row2["answered_by"], "identity=verified") {
		t.Fatalf("真自报应判 verified：answered_by=%q", row2["answered_by"])
	}
	if row2["machine"] != "mini01" {
		t.Fatalf("反例 machine=%q（要 mini01）", row2["machine"])
	}
}

// TestAgentPingDirect_ForeignServiceNotEchoed —— 判据③：对端未自报 ⇒ `（未自报）`/`unverified`，
// ✗ 任何一档都不拿声称值顶替。
func TestAgentPingDirect_ForeignServiceNotEchoed(t *testing.T) {
	wpRoster(t)
	cases := []struct {
		name   string
		status int
		body   string
	}{
		{"外部服务 401", http.StatusUnauthorized, `{"error":"unauthorized"}`},
		{"非 JSON 体", http.StatusOK, "not json at all"},
		{"虫族形状但无 machine", http.StatusOK, `{"ready":true,"phase":"HTTP 已监听"}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := wpPeer(t, c.status, c.body)
			rc, row := wpPing(t, "edge-node", srv.Listener.Addr().String())
			if rc != 0 {
				t.Fatalf("退码 %d（要 0 —— 端口是通的）", rc)
			}
			if !strings.Contains(row["answered_by"], "identity=unverified") {
				t.Fatalf("应判 unverified：answered_by=%q", row["answered_by"])
			}
			if row["machine"] != "（未自报）" {
				t.Fatalf("machine 应=（未自报），实得 %q（✗ 拿声称值顶替）", row["machine"])
			}
			if strings.Contains(row["answered_by"], "machine=edge-node") {
				t.Fatalf("answered_by 里不许把声称值当答话者：%q", row["answered_by"])
			}
		})
	}
}

// TestPeerSelfReportParseAndVerdict —— 判据④：纯函数的六例（信封校验）与五例（判词 + 铁律）。
func TestPeerSelfReportParseAndVerdict(t *testing.T) {
	parse := []struct {
		name   string
		status int
		body   string
		want   string
		ok     bool
	}{
		{"真自报", http.StatusOK, wpZergReadyBody, "mini01", true},
		{"未就绪 503 仍是自称", http.StatusServiceUnavailable, `{"ready":false,"phase":"注册表加载","machine":"edge-node"}`, "edge-node", true},
		{"401 外部服务", http.StatusUnauthorized, `{"error":"unauthorized"}`, "", false},
		{"非 JSON", http.StatusOK, "<html>", "", false},
		{"缺 phase ⇒ 非本族", http.StatusOK, `{"machine":"edge-node","ready":true}`, "", false},
		{"形状对但无 machine", http.StatusOK, `{"ready":true,"phase":"HTTP 已监听"}`, "", false},
	}
	for _, c := range parse {
		got, ok, why := parsePeerSelfReport(c.status, []byte(c.body))
		if got != c.want || ok != c.ok {
			t.Fatalf("%s: parsePeerSelfReport=(%q,%v) 要 (%q,%v) · why=%s", c.name, got, ok, c.want, c.ok, why)
		}
	}

	verdicts := []struct {
		claimed, reported string
		selfReported      bool
		wantIdentity      string
	}{
		{"edge-node", "mini01", true, "mismatch"},
		{"mini01", "mini01", true, "verified"},
		{"", "mini01", true, "verified"},
		{"edge-node", "", false, "unverified"},
		{"", "", false, "unverified"},
	}
	for _, v := range verdicts {
		ab, id := answeredByOf(v.claimed, v.reported, v.selfReported)
		if id != v.wantIdentity {
			t.Fatalf("answeredByOf(%q,%q,%v) identity=%q 要 %q · answered_by=%q",
				v.claimed, v.reported, v.selfReported, id, v.wantIdentity, ab)
		}
		if !v.selfReported {
			if ab != "（未自报）" {
				t.Fatalf("未自报时 answered_by 必须是（未自报），实得 %q", ab)
			}
			continue
		}
		// 铁律：**claimed 不许占「答话者」位**（只能出现在「与声称 X 不符」的对照里）。
		if v.claimed != "" && v.claimed != v.reported {
			if strings.Contains(ab, "machine="+v.claimed) {
				t.Fatalf("claimed 占了答话者位：%q", ab)
			}
		}
	}
}
