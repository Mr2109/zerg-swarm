// eval_chat_harness_test.go —— 件 `core/cmd/zerg/eval_chat_harness.go`（`chat-harness.py` 真搬）的
// **金标内化**（旧 py 对拍退场 ⇒ 逐格金标收成 Go 常量 · 两态 · ✗ 不 Skip）＋ 无害靶子（stub）取证
// （✗ 不真打生产主控 · ✗ 不真起生产对话面）。
//
// 为什么这么测（M2 口径 · 与 `atomic_write_test.go` 的 TestAwTwoStatesGoldenInternalized /
// `check_verify_cmd_real_test.go` 的 TestCheckVerifyCmdRealGoldenInternalized /
// `check_manifest_freshness_test.go` 的 TestCheckManifestFreshnessGoldenInternalized 同族）：
// 本件原「py 原件 ⟷ Go 真身」逐字对拍（旧名 `TestEvalChatHarness_ArgFaceTwoState` /
// `…_DryRunAndNoTurnAgainstStub` / `…_MissingTokenRCFace`）的对拍源 `scripts/evals/chat-harness.py`
// 已退场 ⇒ 哨兵 `chhPy` 里 `os.Stat(script)` 恒失败 ⇒ 断言恒 Skip（假绿、零保护）。按先例把金标
// 真源**收回 Go 半现役行为本身**：
//
//	★ 金标计算式（✗ 不臆造 · 逐格由「Go 半现役行为现算」得到）：
//	  逐格「输入（argv / env + 无害 stub 靶子）⇒ (rc, stdout, stderr)」由 `evalChatHarnessGateMain`
//	  现跑捕获（一次性现算、只读、✗ 不落仓），冻成下表常量；**本机时点类量**在比对前归一化（✗ 不冻结）：
//	    ① 夹具根 `t.TempDir()`（随机路径）⇒ `<TMP>`；
//	    ② stub 服务 URL（`httptest` ephemeral 端口）⇒ `<SRV>`。
//	  （`--help` / usage 正文另以**逐字字面量**入表 —— ✗ 不引用实现里的 `chhHelpText`，免自证。）
//
// 两态（✗ 不许 Skip）：A 态 = 真仓（退役 py 不在盘）现跑；B 态 = 合成「旧态」树（把退役 py 路径
// `scripts/evals/chat-harness.py` 的 stub 放回）现跑 ⇒ 两态**对同一套金标逐字同命中**，证 py 在/不在
// 盘都不改 Go 半读数（Go 真身 ✗ 不读原脚本）。
//
//	· 靶子 = 本进程内 `net/http/httptest` 的 stub HTTP 服务（端点走 `ZERG_CORE`/`ZERG_LOCAL_AGENT`
//	  旋钮指向它）⇒ 「本机模型」与「对话链路」两面全由 stub 顶替，生产面一个字节不动。
//	· 覆盖面 = **确定性面**：`--help`/usage/未知职业/未知参/非法 int/歧义缩写 —— 全在**触网前**判定；
//	  外加 `--dry-run` 与 `--max-turns 0`（后者走完整主流程但**零墙钟**）—— 均对 stub 逐字冻结。
//	· 未覆盖面（写清）：逐轮驱动面（`send` SSE + 本机 `/infer` + 读消息）含墙钟 `用时 %.1fs` 与
//	  会话标题里的 `int(time.time())` ⇒ **不逐字节可比**，真验需**跑着的对话面**（见件头「真验条件」）。
//
// ★ 哑化负控（"牙" · 见交付读数）：`-overlay` 把 `eval_chat_harness.go` 的 argparse 一臂 /
// 帮助常量哑化 ⇒ 本用例**必 FAIL 且带行号**（✗ 不真改仓内源 · 免 `dev edit` 前的仓内污染）。
package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// chhGoHalfRel —— 哨兵锚（Go 半 · 本件真源）。✗ 不锚退役 py（退场即失锚 ⇒ 恒 Skip）。
const chhGoHalfRel = "core/cmd/zerg/eval_chat_harness.go"

// chhSrcRel —— 对拍真身（已退场 py）的仓根相对路径；两态就钉在它「在 / 不在盘」。
const chhSrcRel = "scripts/evals/chat-harness.py"

// chhSentinel —— 哨兵：上溯找 **Go 半**（本件真源）；找不到 ⇒ **fail-closed**（✗ 不留 Skip）。
func chhSentinel(t *testing.T) string {
	t.Helper()
	want := filepath.FromSlash(chhGoHalfRel)
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("取 cwd 失败: %v", err)
	}
	for i := 0; i < 8; i++ {
		if st, e := os.Stat(filepath.Join(dir, want)); e == nil && st.Mode().IsRegular() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("哨兵失守：上溯未见 Go 半 %s（不是真仓树）", chhGoHalfRel)
	return ""
}

type chhRunResult struct {
	rc   int
	out  string
	errb string
}

// chhRunGo ——— 进程内直调 Go 入口（env 显式覆盖；未覆盖项沿用本进程环境）。
func chhRunGo(t *testing.T, env map[string]string, argv ...string) chhRunResult {
	t.Helper()
	for k, v := range env {
		t.Setenv(k, v)
	}
	var o, e bytes.Buffer
	rc := evalChatHarnessGateMain(&o, &e, argv)
	return chhRunResult{rc, o.String(), e.String()}
}

// ── 内化金标：旧 py 对拍的逐格金标收成 Go 常量 · 现算自证 ──────────────────────────

// chhHelpLiteral / chhUsageLiteral —— argparse `--help` 正文与 usage 块（逐字 · ✗ 引用实现常量免自证）。
const chhHelpLiteral = "usage: chat-harness.py [-h] [--profession PROFESSION] [--max-turns MAX_TURNS]\n" +
	"                       [--model MODEL] [--dry-run]\n" +
	"\n" +
	"optional arguments:\n" +
	"  -h, --help            show this help message and exit\n" +
	"  --profession PROFESSION\n" +
	"  --max-turns MAX_TURNS\n" +
	"  --model MODEL\n" +
	"  --dry-run\n"

const chhUsageLiteral = "usage: chat-harness.py [-h] [--profession PROFESSION] [--max-turns MAX_TURNS]\n" +
	"                       [--model MODEL] [--dry-run]\n"

// chhArgGoldenCase —— 触网前的确定性面（argparse / help / usage / 未知职业）逐格金标。
type chhArgGoldenCase struct {
	name    string
	argv    []string
	wantRC  int
	wantOut string
	wantErr string
}

func chhArgGoldenCases() []chhArgGoldenCase {
	u := chhUsageLiteral
	return []chhArgGoldenCase{
		{"help", []string{"--help"}, 0, chhHelpLiteral, ""},
		{"-h", []string{"-h"}, 0, chhHelpLiteral, ""},
		{"bogus", []string{"--bogus"}, 2, "", u + "chat-harness.py: error: unrecognized arguments: --bogus\n"},
		{"positional", []string{"foo"}, 2, "", u + "chat-harness.py: error: unrecognized arguments: foo\n"},
		{"maxturns_space", []string{"--max-turns", "abc"}, 2, "",
			u + "chat-harness.py: error: argument --max-turns: invalid int value: 'abc'\n"},
		{"maxturns_eq", []string{"--max-turns=abc"}, 2, "",
			u + "chat-harness.py: error: argument --max-turns: invalid int value: 'abc'\n"},
		{"ambig", []string{"--m", "3"}, 2, "",
			u + "chat-harness.py: error: ambiguous option: --m could match --max-turns, --model\n"},
		{"profession_missing", []string{"--profession"}, 2, "",
			u + "chat-harness.py: error: argument --profession: expected one argument\n"},
		{"dryrun_eq", []string{"--dry-run=1"}, 2, "",
			u + "chat-harness.py: error: argument --dry-run: ignored explicit argument '1'\n"},
		{"profession_unknown", []string{"--profession", "ZZ"}, 1, "", "未知职业：ZZ\n"},
		{"profession_unknown_eq", []string{"--profession=P2"}, 1, "", "未知职业：P2\n"},
	}
}

// chhNormGolden —— 归一本机时点类量（夹具根 ⇒ `<TMP>` · stub 服务 URL ⇒ `<SRV>`）⇒ 金标跨机可复现。
func chhNormGolden(s, tmp, srv string) string {
	if srv != "" {
		s = strings.ReplaceAll(s, srv, "<SRV>")
	}
	if tmp != "" {
		s = strings.ReplaceAll(s, tmp, "<TMP>")
	}
	return s
}

// chhRunArgGolden —— 逐格现跑触网前确定性面，归一后与冻值逐字比；返回失配格数。
func chhRunArgGolden(t *testing.T, tag string) int {
	t.Helper()
	mism := 0
	for _, c := range chhArgGoldenCases() {
		g := chhRunGo(t, nil, c.argv...)
		if g.rc != c.wantRC {
			mism++
			t.Errorf("[%s/%s] rc=%d（冻值 %d）", tag, c.name, g.rc, c.wantRC)
		}
		if g.out != c.wantOut {
			mism++
			t.Errorf("[%s/%s] stdout 漂移\n实得=%q\n冻值=%q", tag, c.name, g.out, c.wantOut)
		}
		if g.errb != c.wantErr {
			mism++
			t.Errorf("[%s/%s] stderr 漂移\n实得=%q\n冻值=%q", tag, c.name, g.errb, c.wantErr)
		}
	}
	return mism
}

// TestEvalChatHarness_ArgFaceGolden —— argv/help/usage/未知职业 全在**触网前**判定 ⇒ 不需任何服务。
// 哨兵锚 **Go 半**（✗ 不锚退役 py · ✗ 不 Skip）。
func TestEvalChatHarness_ArgFaceGolden(t *testing.T) {
	_ = chhSentinel(t) // 哨兵：锚 Go 半在盘 ⇒ 真断言
	if n := chhRunArgGolden(t, "argface"); n != 0 {
		t.Fatalf("argv 面金标失配 %d 格", n)
	}
}

// ── 无害靶子（stub）上的完整主流程金标 ─────────────────────────────────────────────

// chhStubGolden* —— stub 靶子档的金标（见件头金标计算式 · ✗ 手写猜）。
const chhStubDryRunOut = "[harness] 职业=P1(程序员) 目标=定位仓里一处小问题并说明改法（指到 file:line）\n" +
	"[harness] 被测=<SRV> 驱动=<SRV> 观测=<TMP>/state/chat_obs.jsonl\n" +
	"[harness] 会话 = S-42\n" +
	"[harness] 模型设为 Qwen3.8-27B（{\"ok\":true,\"model\":\"Qwen3.8-27B\"}）\n" +
	"[harness] dry-run：只建会话，退出 ✓\n"

const chhStubNoTurnOut = "[harness] 职业=P1(程序员) 目标=定位仓里一处小问题并说明改法（指到 file:line）\n" +
	"[harness] 被测=<SRV> 驱动=<SRV> 观测=<TMP>/state/chat_obs.jsonl\n" +
	"[harness] 会话 = S-42\n" +
	"[harness] 模型设为 Qwen3.8-27B（{\"ok\":true,\"model\":\"Qwen3.8-27B\"}）\n" +
	"\n" +
	"[harness] 新增观测 0 行\n" +
	"           turn     0\n" +
	"           compact  0\n" +
	"           tool     0\n" +
	"\n" +
	"[harness] 发现 0 条（甲表=硬结论；乙表=评审线索，未接）\n" +
	"\n" +
	"[harness] 会话 S-42 保留待查（清理：DELETE /api/chat/sessions/S-42）\n"

// TestEvalChatHarness_StubGolden —— 无害靶子（stub）上的完整主流程两档金标 ＋「未打生产」证明。
func TestEvalChatHarness_StubGolden(t *testing.T) {
	_ = chhSentinel(t)

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

	cases := []struct {
		name    string
		argv    []string
		wantOut string
	}{
		{"--dry-run(stub)", []string{"--dry-run"}, chhStubDryRunOut},
		{"--max-turns 0(stub)", []string{"--max-turns", "0"}, chhStubNoTurnOut},
	}
	hitsBefore := hits
	for _, c := range cases {
		g := chhRunGo(t, env, c.argv...)
		gotOut := chhNormGolden(g.out, tmp, srv.URL)
		gotErr := chhNormGolden(g.errb, tmp, srv.URL)
		if g.rc != 0 {
			t.Errorf("[%s] rc=%d（冻值 0）", c.name, g.rc)
		}
		if gotOut != c.wantOut {
			t.Errorf("[%s] stdout 漂移\n实得=%q\n冻值=%q", c.name, gotOut, c.wantOut)
		}
		if gotErr != "" {
			t.Errorf("[%s] stderr 漂移\n实得=%q", c.name, gotErr)
		}
	}
	if !strings.Contains(cases[0].wantOut, "dry-run：只建会话，退出 ✓") {
		t.Fatalf("dry-run 金标没走到退出档")
	}
	if !strings.Contains(cases[1].wantOut, "新增观测 0 行") {
		t.Fatalf("max-turns 0 金标没走到观测判据档")
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

// chhMissingTokenOut —— 缺令牌档金标（stdout 空 · stderr 逐字 · `<TMP>` = 空家目录占位）。
const chhMissingTokenErr = "chat-harness.py: 读不到共享令牌：open <TMP>/.zerg/token: no such file or directory\n"

// TestEvalChatHarness_MissingTokenGolden —— 令牌读不到 ⇒ Go 半 rc=1 ＋ 逐字 stderr（旧 py 对拍来源
// 退役后收成 Go 常量 · ✗ 不 Skip）。
func TestEvalChatHarness_MissingTokenGolden(t *testing.T) {
	_ = chhSentinel(t)

	tmp := t.TempDir() // 空家目录 ⇒ 无 .zerg/token
	env := map[string]string{
		"HOME":             tmp,
		"ZERG_CORE":        "http://127.0.0.1:1",
		"ZERG_LOCAL_AGENT": "http://127.0.0.1:1",
		"ZERG_STATE_DIR":   filepath.Join(tmp, "state"),
	}
	g := chhRunGo(t, env, "--dry-run")
	if g.rc != 1 {
		t.Fatalf("缺令牌档应 rc=1，得 rc=%d", g.rc)
	}
	if g.out != "" {
		t.Errorf("缺令牌档 stdout 应空，得 %q", g.out)
	}
	if got := chhNormGolden(g.errb, tmp, ""); got != chhMissingTokenErr {
		t.Errorf("缺令牌档 stderr 漂移\n实得=%q\n冻值=%q", got, chhMissingTokenErr)
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

// TestEvalChatHarness_GoldenTwoStates —— 两态逐字同：A 态 = 真仓（退役 py 不在盘）现跑；B 态 = 合成
// 「旧态」树（`scripts/evals/chat-harness.py` 的 stub 放回）现跑 ⇒ 两态**对同一套金标逐字同命中**
// （证 py 在/不在盘都不改 Go 半读数 · ✗ 不因退役 Skip、✗ 不放宽断言）。
func TestEvalChatHarness_GoldenTwoStates(t *testing.T) {
	root := chhSentinel(t)
	py := filepath.Join(root, filepath.FromSlash(chhSrcRel))
	if _, err := os.Stat(py); err == nil {
		t.Fatalf("A 态前提破：退役 py 竟在盘 %s（本件前提 = 内化后无 py）", py)
	}
	if n := chhRunArgGolden(t, "A真仓无py"); n != 0 {
		t.Fatalf("A 态金标失配 %d 格", n)
	}

	// B 态：合成「旧态」树（把退役 py 路径的 stub 放回）⇒ 逐字同（Go 半 ✗ 不读它）。
	old := t.TempDir()
	p := filepath.Join(old, filepath.FromSlash(chhSrcRel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("#!/usr/bin/env python3\n# [退役占位] 旧态 stub\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ZERG_REPO", old)
	if n := chhRunArgGolden(t, "B旧态有py"); n != 0 {
		t.Fatalf("B 态金标失配 %d 格（py 在盘不得改判据结论）", n)
	}
}
