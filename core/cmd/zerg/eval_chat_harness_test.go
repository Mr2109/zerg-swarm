// eval_chat_harness_test.go —— 件 eval_chat_harness.go（chat-harness.py 真搬）的
// **两态对拍（py 原件 ⟷ Go 真身）+ 无害靶子（stub）取证**（✗ 不真打生产主控 · ✗ 不真起生产对话面）。
//
//	· 靶子 = 本进程内 `net/http/httptest` 的 stub HTTP 服务（端点走 `ZERG_CORE`/`ZERG_LOCAL_AGENT`
//	  旋钮指向它）⇒ 「本机模型」与「对话链路」两面全由 stub 顶替，生产面一个字节不动。
//	· 两态 = 同一条 argv 各跑一遍：① py 原件（子进程 · 与使用者同一份 `python3`）；② 本件 Go 入口
//	  （进程内直调 `evalChatHarnessGateMain`）。比 rc / stdout / stderr 三面逐字节。
//	· 覆盖面 = **确定性面**：`--help`/usage/未知职业/未知参/非法 int/歧义缩写 —— 全在**触网前**判定；
//	  外加 `--dry-run` 与 `--max-turns 0`（后者走完整主流程但**零墙钟**）—— 均对 stub 逐字节同。
//	· 未覆盖面（写清）：逐轮驱动面（`send` SSE + 本机 `/infer` + 读消息）含墙钟 `用时 %.1fs` 与
//	  会话标题里的 `int(time.time())` ⇒ **不逐字节可比**，真验需**跑着的对话面**（见件头「真验条件」）。
package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const chhSrcRel = "scripts/evals/chat-harness.py"

// chhPy ——— 现读现役 python3 与原脚本（缺一即跳过对拍）。
func chhPy(t *testing.T) (string, string) {
	t.Helper()
	py, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("无 python3 ⇒ 跳过两态对拍")
	}
	root := repoRoot()
	if root == "" {
		t.Fatal("现读不到仓根（repoRoot 空）")
	}
	script := filepath.Join(root, filepath.FromSlash(chhSrcRel))
	if _, err := os.Stat(script); err != nil {
		t.Skipf("原脚本不在盘（%v）⇒ 跳过对拍", err)
	}
	return py, script
}

type chhRunResult struct {
	rc   int
	out  string
	errb string
}

// chhRunPy ——— 子进程跑 py 原件（env 显式覆盖；未覆盖项沿用本进程环境）。
func chhRunPy(t *testing.T, py, script string, env map[string]string, argv ...string) chhRunResult {
	t.Helper()
	cmd := exec.Command(py, append([]string{script}, argv...)...)
	cmd.Env = append([]string{}, os.Environ()...)
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	var o, e bytes.Buffer
	cmd.Stdout, cmd.Stderr = &o, &e
	err := cmd.Run()
	rc := 0
	if ee, ok := err.(*exec.ExitError); ok {
		rc = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("跑 py 起不来：%v", err)
	}
	return chhRunResult{rc, o.String(), e.String()}
}

// chhRunGo ——— 进程内直调 Go 入口（env 显式覆盖）。
func chhRunGo(t *testing.T, env map[string]string, argv ...string) chhRunResult {
	t.Helper()
	for k, v := range env {
		t.Setenv(k, v)
	}
	var o, e bytes.Buffer
	rc := evalChatHarnessGateMain(&o, &e, argv)
	return chhRunResult{rc, o.String(), e.String()}
}

func chhCompare(t *testing.T, label string, p, g chhRunResult) {
	t.Helper()
	if p.rc != g.rc {
		t.Errorf("%s：rc 两态分叉 py=%d go=%d", label, p.rc, g.rc)
	}
	if p.out != g.out {
		t.Errorf("%s：stdout 两态分叉\n  py=%q\n  go=%q", label, p.out, g.out)
	}
	if p.errb != g.errb {
		t.Errorf("%s：stderr 两态分叉\n  py=%q\n  go=%q", label, p.errb, g.errb)
	}
}

// TestEvalChatHarness_ArgFaceTwoState —— argv/help/usage/未知职业 全在**触网前**判定 ⇒ 不需任何服务。
func TestEvalChatHarness_ArgFaceTwoState(t *testing.T) {
	py, script := chhPy(t)
	cases := [][]string{
		{"--help"},
		{"-h"},
		{"--bogus"},
		{"foo"},
		{"--max-turns", "abc"},
		{"--max-turns=abc"},
		{"--m", "3"},
		{"--profession"},
		{"--dry-run=1"},
		{"--profession", "ZZ"},
		{"--profession=P2"},
	}
	for _, argv := range cases {
		p := chhRunPy(t, py, script, nil, argv...)
		g := chhRunGo(t, nil, argv...)
		chhCompare(t, "argv="+strings.Join(argv, " "), p, g)
	}
}

// TestEvalChatHarness_DryRunAndNoTurnAgainstStub —— 无害靶子（stub）上的完整主流程两态对拍。
func TestEvalChatHarness_DryRunAndNoTurnAgainstStub(t *testing.T) {
	py, script := chhPy(t)

	hits := 0
	badPaths := []string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == "POST" && r.URL.Path == "/api/chat/sessions":
			_, _ = w.Write([]byte(`{"session_id":"S-42"}`))
		case r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/model"):
			_, _ = w.Write([]byte(`{"ok":true,"model":"Qwen3.8-27B"}`))
		default:
			badPaths = append(badPaths, r.Method+" "+r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, ".zerg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, ".zerg", "token"), []byte("TOK-2state\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	env := map[string]string{
		"HOME":                  tmp,
		"ZERG_CORE":             srv.URL,
		"ZERG_LOCAL_AGENT":      srv.URL,
		"ZERG_STATE_DIR":        filepath.Join(tmp, "state"),
		"ZERG_UNDER_TEST_MODEL": "Qwen3.8-27B",
	}

	// ① dry-run：建会话 + 设模型 + 退出（零墙钟 ⇒ 逐字节可比）。
	p := chhRunPy(t, py, script, env, "--dry-run")
	g := chhRunGo(t, env, "--dry-run")
	chhCompare(t, "--dry-run(stub)", p, g)
	if !strings.Contains(g.out, "[harness] dry-run：只建会话，退出 ✓") {
		t.Errorf("dry-run 未走到退出档：%q", g.out)
	}

	// ② --max-turns 0：走**完整主流程**（建会话/设模型/读观测/出判据/收尾行）但零轮 ⇒ 零墙钟 ⇒ 逐字节可比。
	hitsBefore := hits
	p2 := chhRunPy(t, py, script, env, "--max-turns", "0")
	g2 := chhRunGo(t, env, "--max-turns", "0")
	chhCompare(t, "--max-turns 0(stub)", p2, g2)
	if !strings.Contains(g2.out, "[harness] 新增观测 0 行") {
		t.Errorf("主流程未走到观测判据档：%q", g2.out)
	}

	// ★ 未打生产证明：stub 真被命中（hits 增长），且**全部**路径都是本 stub 的 /api/chat 面；
	//   没有任何请求落到生产端口（端点就是 stub 的 ephemeral 端口 ⇒ 结构上不可能打到 :8580/:8100）。
	if hits <= hitsBefore {
		t.Fatalf("stub 未被命中（hits=%d）⇒ 两态根本没走网络", hits)
	}
	if len(badPaths) != 0 {
		t.Fatalf("stub 收到计划外路径（说明打到了别处协议）：%v", badPaths)
	}
}

// TestEvalChatHarness_MissingTokenRCFace —— 令牌读不到 ⇒ 两态都 rc=1（py 打 Python 回溯 ⇒ 只比 rc 与 stdout）。
func TestEvalChatHarness_MissingTokenRCFace(t *testing.T) {
	py, script := chhPy(t)
	tmp := t.TempDir() // 空家目录 ⇒ 无 .zerg/token
	env := map[string]string{
		"HOME":             tmp,
		"ZERG_CORE":        "http://127.0.0.1:1",
		"ZERG_LOCAL_AGENT": "http://127.0.0.1:1",
		"ZERG_STATE_DIR":   filepath.Join(tmp, "state"),
	}
	p := chhRunPy(t, py, script, env, "--dry-run")
	g := chhRunGo(t, env, "--dry-run")
	if p.rc != g.rc {
		t.Fatalf("缺令牌档 rc 分叉 py=%d go=%d", p.rc, g.rc)
	}
	if p.out != g.out {
		t.Fatalf("缺令牌档 stdout 分叉 py=%q go=%q", p.out, g.out)
	}
	if p.rc != 1 {
		t.Fatalf("缺令牌档应 rc=1，得 rc=%d", p.rc)
	}
}

// TestEvalChatHarness_HelpIsRetireProof —— 退役态合成副本：`--help` 走件内常量、**不读原脚本** ⇒ 原脚本移开也同。
func TestEvalChatHarness_HelpIsRetireProof(t *testing.T) {
	var o, e bytes.Buffer
	rc := evalChatHarnessGateMain(&o, &e, []string{"--help"})
	if rc != 0 {
		t.Fatalf("--help 应 rc=0，得 %d", rc)
	}
	if o.String() != chhHelpText {
		t.Fatalf("--help 正文 ≠ 合成副本常量：\n%q", o.String())
	}
	if !strings.Contains(o.String(), "usage: chat-harness.py") {
		t.Fatalf("--help 缺 argparse usage 头")
	}
}
