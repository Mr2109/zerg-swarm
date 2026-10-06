// gen_gap_ledger_test.go —— `gen_gap_ledger.go`（`scripts/docs/gen-gap-ledger.py` 的 Go 真搬）
// 成对负控。断言全是「只有实现对才过」的**逐字**等式（哑化任一语义面必红 —— 见回执的
// 仓外副本哑化实证：把主件语义改坏 ⇒ 本件 FAIL 带行号）＋ 空值/缺输入/两态各成一子用例。
//
// ★ stderr 面钉法（件头 ⑨ · 未覆盖面）：`TestGglPyParityPinnedInterpreter` 按**仓约定解释器**
//
//	`/usr/bin/python3`（3.9.6 · 见 `baseline_ratchet_test.go:21 pyInterp`）把 py 真身与本入口
//	摆到同一 argv 下**三面逐字对拍** —— 该版本 ≤3.11 ⇒ py 真身**不发** `gen-gap-ledger.py:29`
//	的 `SyntaxWarning` ⇒ 两态 stderr **皆空 · 逐字同**；哑化本件（让 Go 侧合成该解释器告警）
//	 ⇒ 本条 stderr 断言必红（带行号）。★ `--help` 不入对拍集：其正文逐**解释器版本**变
//	（≤3.9 `optional arguments:` ⟷ ≥3.10 `options:`）⇒ 归件头 ① 的**未覆盖面（环境依赖）**。
//
// ★ 无 TestMain / 无包级 init（撞名规避，照 `gen_tools_md_test.go` 同规）；helper 一律 `ggl` 前缀。
package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gglTestExport —— 夹具"导出器"的确定产物（亦 = 生成件应逐字节相同的内容）。
const gglTestExport = "## 缺口真源导出（**只列未闭**） · 2026-09-26\n\n| id | 状态 |\n|---|---|\n| GAP-1 | 仍缺 |\n"

// gglTestFixture —— 建一棵仓外假仓：`<base>/repo/{bin/zerg, scripts/docs/}` ＋ 兄弟文档树
// `<base>/Zerg-内部文档/项目文档/v2.5.13/缺口总账-20260926.md` ＋ `<base>/state/gaps.jsonl`。
// 返回 (base, repoRoot, gapsPath)。
func gglTestFixture(t *testing.T) (string, string, string) {
	t.Helper()
	base := t.TempDir()
	root := filepath.Join(base, "repo")
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	exporter := "#!/bin/sh\nprintf '%s' " + shSingleQuote(gglTestExport) + "\n"
	if err := os.WriteFile(filepath.Join(root, "bin", "zerg"), []byte(exporter), 0o755); err != nil {
		t.Fatal(err)
	}
	docs := filepath.Join(base, "Zerg-内部文档", "项目文档", "v2.5.13")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docs, "缺口总账-20260926.md"), []byte("# placeholder\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	state := filepath.Join(base, "state")
	if err := os.MkdirAll(state, 0o755); err != nil {
		t.Fatal(err)
	}
	gaps := filepath.Join(state, "gaps.jsonl")
	if err := os.WriteFile(gaps, []byte("{\"id\":\"a\",\"qid\":\"Q-001\"}\n{\"id\":\"b\",\"qid\":\"Q-002\"}\n{\"id\":\"c\"}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return base, root, gaps
}

func shSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// ── 路径语义（posixpath 同构）────────────────────────────────────────────

func TestGglPathSemantics(t *testing.T) {
	if got := gglPyJoin("a", "b"); got != "a/b" {
		t.Errorf("gglPyJoin(a,b) = %q, want a/b", got)
	}
	// 后段绝对路径丢弃前段（py 语义 · ✗ filepath.Join）
	if got := gglPyJoin("repo", "/abs"); got != "/abs" {
		t.Errorf("gglPyJoin(repo,/abs) = %q, want /abs", got)
	}
	// 空段：py join('a','')=='a/' · join('','a')=='a'
	if got := gglPyJoin("a", ""); got != "a/" {
		t.Errorf("gglPyJoin(a,'') = %q, want a/", got)
	}
	if got := gglPyJoin("", "a"); got != "a" {
		t.Errorf("gglPyJoin('',a) = %q, want a", got)
	}
	// `..` 原样留串（py join('a','../b')=='a/../b'）
	if got := gglPyJoin("a", "../b"); got != "a/../b" {
		t.Errorf("gglPyJoin(a,../b) = %q, want a/../b", got)
	}
	// normpath 保留恰两个前导斜杠
	if got := gglPyNormpath("//a/b/../c"); got != "//a/c" {
		t.Errorf("gglPyNormpath(//a/b/../c) = %q, want //a/c", got)
	}
	if got := gglPyNormpath("/a/b/../c"); got != "/a/c" {
		t.Errorf("gglPyNormpath(/a/b/../c) = %q, want /a/c", got)
	}
	if got := gglPyRelpath("/x/y/z", "/x"); got != "y/z" {
		t.Errorf("gglPyRelpath = %q, want y/z", got)
	}
}

// ── 版本线排序（数字元组优先 · ✗ 字符串比）───────────────────────────────

func TestGglVkeyOrder(t *testing.T) {
	if !gglVerKeyLess(gglVkey("v2.5.9"), gglVkey("v2.5.13")) {
		t.Errorf("v2.5.9 应小于 v2.5.13（纯字符串比会反序）")
	}
	if gglVerKeyLess(gglVkey("v2.5.13"), gglVkey("v2.5.9")) {
		t.Errorf("v2.5.13 不该小于 v2.5.9")
	}
}

func TestGglPickVersionDirExcludesExempt(t *testing.T) {
	docs := t.TempDir()
	for _, rel := range []string{"v2.5.12/缺口总账-20260923.md", "v2.5.9/缺口总账-20260901.md",
		"v2.5.13/缺口总账-20260925.md", "v2.5.12/承接自-v2.5.11/缺口总账-20260922.md"} {
		p := filepath.Join(docs, "项目文档", filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("# x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, ok := gglPickVersionDir(docs)
	if !ok || got != "v2.5.13" {
		t.Errorf("gglPickVersionDir = (%q,%v), want (v2.5.13,true)（豁免件不算 · v2.5.13>v2.5.9）", got, ok)
	}
	if _, ok := gglPickVersionDir(filepath.Join(docs, "没有此目录")); ok {
		t.Errorf("空文档仓应判不出（ok=false）")
	}
}

// ── 日期取读 ────────────────────────────────────────────────────────────

func TestGglExportDate(t *testing.T) {
	if d, ok := gglExportDate("## 缺口真源导出（**只列未闭**） · 2026-09-26\n"); !ok || d != "20260926" {
		t.Errorf("gglExportDate 首行 = (%q,%v), want (20260926,true)", d, ok)
	}
	if d, ok := gglExportDate("## 无日期行\n"); ok || d != "" {
		t.Errorf("gglExportDate 无日期行 = (%q,%v), want ('',false)", d, ok)
	}
}

// ── qid 读面（成对：未启用不红 / 合法过 / 形态坏红 / 重号红）──────────────

func TestGglQidValidate(t *testing.T) {
	q := gglQidValidate([]map[string]interface{}{{"id": "GAP-1"}, {"id": "GAP-2"}})
	if q.enabled || q.rc != -1 {
		t.Errorf("未启用 = (enabled=%v,rc=%d), want (false,-1)", q.enabled, q.rc)
	}
	if got := gglQidVerdict(q); got != "qid 字段未启用 · 0/2 条带 qid（未启用态不判红）" {
		t.Errorf("未启用判词 = %q", got)
	}

	q = gglQidValidate([]map[string]interface{}{{"id": "a", "qid": "Q-001"}, {"id": "b", "qid": "Q-002"}, {"id": "c"}})
	if !q.enabled || q.rc != 0 || q.withQid != 2 || q.total != 3 {
		t.Errorf("合法 = (%v,%d,%d,%d), want (true,0,2,3)", q.enabled, q.rc, q.withQid, q.total)
	}
	if got := gglQidVerdict(q); got != "qid 面过 · 2/3 条带 qid（形态合法 · 号唯一）" {
		t.Errorf("合法判词 = %q", got)
	}

	q = gglQidValidate([]map[string]interface{}{{"id": "a", "qid": "Q1"}, {"id": "b", "qid": "q-002"}})
	if q.rc != 1 || len(q.badFormat) != 2 || q.badFormat[0] != [2]string{"a", "Q1"} || q.badFormat[1] != [2]string{"b", "q-002"} {
		t.Errorf("形态非法 = (rc=%d,bad=%v), want (1,[[a Q1] [b q-002]])", q.rc, q.badFormat)
	}
	if got := gglQidVerdict(q); got != "qid 面判红 · 形态非法 2 处（a→Q1 b→q-002）· 号重复 0 处（-）" {
		t.Errorf("形态非法判词 = %q", got)
	}

	q = gglQidValidate([]map[string]interface{}{{"id": "a", "qid": "Q-007"}, {"id": "b", "qid": "Q-007"}})
	if q.rc != 1 || len(q.dup) != 1 || q.dup[0][0].(string) != "Q-007" {
		t.Errorf("重号 = (rc=%d,dup=%v), want (1,[Q-007×2])", q.rc, q.dup)
	}
	if got := gglQidVerdict(q); got != "qid 面判红 · 形态非法 0 处（-）· 号重复 1 处（Q-007×2）" {
		t.Errorf("重号判词 = %q", got)
	}
}

// ── 「空 ≠ 取不到」两态 ─────────────────────────────────────────────────

func TestGglSourceRowsEmptyVsMissing(t *testing.T) {
	dir := t.TempDir()
	empty := filepath.Join(dir, "empty.jsonl")
	if err := os.WriteFile(empty, []byte("\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rows, errMsg := gglReadSourceRows(empty)
	if rows == nil {
		t.Errorf("空真源应返回非 nil 空表（空 ≠ 取不到）")
	}
	if errMsg != "" {
		t.Errorf("空真源 errMsg = %q, want ''", errMsg)
	}
	rows, errMsg = gglReadSourceRows(filepath.Join(dir, "nope.jsonl"))
	if rows != nil {
		t.Errorf("真源缺件应返回 nil（取不到）")
	}
	if errMsg == "" {
		t.Errorf("真源缺件应有说明串")
	}
}

// ── argparse 面（精确 / 前缀 / 缺值 / 显式赋值拒）────────────────────────

func TestGglParseArgs(t *testing.T) {
	if a, e := gglParseArgs([]string{"--help"}); e != "" || !a.help {
		t.Errorf("--help = (%+v,%q)", a, e)
	}
	if a, e := gglParseArgs([]string{"--version"}); e != "" || !a.version {
		t.Errorf("--version = (%+v,%q)", a, e)
	}
	if _, e := gglParseArgs([]string{"--nope"}); e != "unrecognized arguments: --nope" {
		t.Errorf("未知旗标 err = %q", e)
	}
	if _, e := gglParseArgs([]string{"--docs-root"}); e != "argument --docs-root: expected one argument" {
		t.Errorf("缺值 err = %q", e)
	}
	if _, e := gglParseArgs([]string{"--emit=1"}); e != "argument --emit: ignored explicit argument '1'" {
		t.Errorf("显式赋值 err = %q", e)
	}
	if a, e := gglParseArgs([]string{"--em", "--source", "x"}); e != "" || !a.emit || a.source != "x" {
		t.Errorf("前缀 --em = (%+v,%q), want emit", a, e)
	}
}

// ── 入口编排（help/version 逐字 · self-test）────────────────────────────

func TestGglDispatchHelpVersion(t *testing.T) {
	var so, se bytes.Buffer
	if rc := gglDispatch(&so, &se, []string{"--help"}, "/x"); rc != 0 {
		t.Fatalf("--help rc = %d", rc)
	}
	if so.String() != gglHelpText {
		t.Errorf("--help stdout 与嵌入文本不逐字同")
	}
	so.Reset()
	if rc := gglDispatch(&so, &se, []string{"--version"}, "/x"); rc != 0 || so.String() != gglVersion+"\n" {
		t.Errorf("--version = rc %d / %q", rc, so.String())
	}
	so.Reset()
	if rc := gglDispatch(&so, &se, []string{"--nope"}, "/x"); rc != 2 {
		t.Fatalf("未知旗标 rc = %d, want 2", rc)
	}
	if se.String() != gglUsage+gglSelfName+": error: unrecognized arguments: --nope\n" {
		t.Errorf("未知旗标 stderr = %q", se.String())
	}
}

func TestGglSelfTestAllPass(t *testing.T) {
	var so bytes.Buffer
	if rc := gglSelfTest(&so); rc != 0 {
		t.Fatalf("gglSelfTest rc = %d, want 0\n%s", rc, so.String())
	}
	if !strings.Contains(so.String(), "自检结论: 全过") {
		t.Errorf("自检结论行缺：%q", so.String())
	}
	if !strings.HasSuffix(so.String(), "自检结论: 全过\n") {
		t.Errorf("自检尾行不逐字：%q", so.String())
	}
}

// ── 主逻辑：产物逐字节同 + 两态（漂移/绿/落盘）──────────────────────────

func TestGglMainProductByteIdentical(t *testing.T) {
	base, root, gaps := gglTestFixture(t)
	tgt := filepath.Join(base, "Zerg-内部文档", "项目文档", "v2.5.13", "缺口总账-20260926.md")

	// 态① 漂移（占位 14B ≠ 直出）⇒ check rc=1
	var so, se bytes.Buffer
	rc := gglMain(&so, &se, gglArgs{check: true, source: gaps}, root)
	if rc != 1 {
		t.Fatalf("drift check rc = %d, want 1\n%s", rc, so.String())
	}
	if !strings.Contains(so.String(), "⇒ 判红（漂移）") {
		t.Errorf("drift stdout 缺判红：%q", so.String())
	}

	// 态② emit 落盘 ⇒ 产物逐字节 == 直出
	so.Reset()
	se.Reset()
	rc = gglMain(&so, &se, gglArgs{emit: true, source: gaps}, root)
	if rc != 0 {
		t.Fatalf("emit rc = %d, want 0\n%s", rc, so.String())
	}
	got, err := os.ReadFile(tgt)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != gglTestExport {
		t.Errorf("生成件与直出不逐字节同\n got=%q\nwant=%q", string(got), gglTestExport)
	}
	if !strings.Contains(so.String(), fmt.Sprintf("✓ 落盘（覆盖漂移件 · 旧件 14 字节 ⇒ 新件 %d 字节）", len([]byte(gglTestExport)))) {
		t.Errorf("emit stdout 判定行不逐字：%q", so.String())
	}
	// 两态：落盘后重跑 check ⇒ 绿（逐字节相同）
	so.Reset()
	rc = gglMain(&so, &se, gglArgs{check: true, source: gaps}, root)
	if rc != 0 {
		t.Fatalf("green check rc = %d, want 0\n%s", rc, so.String())
	}
	if !strings.Contains(so.String(), fmt.Sprintf("✓ 生成件 == 直出（逐字节相同 · %d 字节）", len([]byte(gglTestExport)))) {
		t.Errorf("green stdout 不逐字：%q", so.String())
	}
}

func TestGglMainMissingDocsBail(t *testing.T) {
	_, root, gaps := gglTestFixture(t)
	var so, se bytes.Buffer
	rc := gglMain(&so, &se, gglArgs{check: true, source: gaps, docsRoot: filepath.Join(root, "nope")}, root)
	if rc != 2 {
		t.Fatalf("缺文档仓 rc = %d, want 2", rc)
	}
	if !strings.Contains(so.String(), "⛔ 不给结论（rc=2）：文档仓根不在盘：") {
		t.Errorf("bail stdout 不逐字：%q", so.String())
	}
}

func TestGglQidCheckMode(t *testing.T) {
	_, _, _ = gglTestFixture(t)
	dir := t.TempDir()
	ok := filepath.Join(dir, "ok.jsonl")
	_ = os.WriteFile(ok, []byte("{\"id\":\"a\",\"qid\":\"Q-1\"}\n{\"id\":\"b\",\"qid\":\"Q-2\"}\n"), 0o644)
	if rc := gglQidCheckMode(gglArgs{qidCheck: true, source: ok}); rc != 0 {
		t.Errorf("qid 合法 rc = %d, want 0", rc)
	}
	bad := filepath.Join(dir, "bad.jsonl")
	_ = os.WriteFile(bad, []byte("{\"id\":\"a\",\"qid\":\"Q1\"}\n"), 0o644)
	if rc := gglQidCheckMode(gglArgs{qidCheck: true, source: bad}); rc != 1 {
		t.Errorf("qid 形态坏 rc = %d, want 1", rc)
	}
	if rc := gglQidCheckMode(gglArgs{qidCheck: true, source: filepath.Join(dir, "nope.jsonl")}); rc != 2 {
		t.Errorf("qid 真源缺 rc = %d, want 2", rc)
	}
}

// ── py⟷Go 三面逐字对拍（钉 stderr 面 · 解释器钉仓约定 /usr/bin/python3）─────────────

func gglTestRepoRoot(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("仓根解析失败：%v", err)
	}
	return abs
}

// TestGglPyParityPinnedInterpreter —— 件头 ⑨ 的**机器钉**：同一 argv 下，本入口 ⟷ py 真身
// （解释器钉仓约定 `/usr/bin/python3` = 3.9.6）三面 rc/stdout/stderr **逐字同**。
//
// 该解释器 ≤3.11 ⇒ py 真身**不发** `gen-gap-ledger.py:29` 的 `SyntaxWarning`（该告警只在 ≥3.12
// 出现 · 且 3.12/3.13 与 3.14 正文不同）⇒ 两侧 stderr **皆空**。★ 成对负控：哑化本件（让
// `genGapLedgerGateMain` 向 stderr 多写一字节 —— 例如"复刻"该解释器告警）⇒ 本条 stderr 断言
// **必红带行号**（见回执的仓外 overlay 哑化实证）。
//
// ★ `--help` **不入对拍集**：其正文随解释器版本变（≤3.9 `optional arguments:` ⟷ ≥3.10 `options:`）
// ⇒ 归件头 ① 的未覆盖面（环境依赖），不在本面承诺内；本集只用**版本无关** argv。
func TestGglPyParityPinnedInterpreter(t *testing.T) {
	py := filepath.Join(gglTestRepoRoot(t), "scripts", "docs", "gen-gap-ledger.py")
	if _, err := os.Stat(py); err != nil {
		t.Skipf("py 真身不在（可能已退场）：%s", py)
	}
	const pyInterp = "/usr/bin/python3" // 仓约定解释器（3.9.6 · 见 baseline_ratchet_test.go:21 pyInterp）
	if _, err := os.Stat(pyInterp); err != nil {
		t.Skipf("钉死的解释器不在：%s", pyInterp)
	}
	cases := [][]string{
		{"--self-test"}, {"--version"}, {"--nope"}, {"--docs-root"}, {"--emit=1"},
	}
	for _, args := range cases {
		args := args
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			var so, se bytes.Buffer
			rcG := genGapLedgerGateMain(&so, &se, args)

			cmd := exec.Command(pyInterp, append([]string{py}, args...)...)
			var po, pe bytes.Buffer
			cmd.Stdout = &po
			cmd.Stderr = &pe
			err := cmd.Run()
			rcP := 0
			if ee, ok := err.(*exec.ExitError); ok {
				rcP = ee.ExitCode()
			} else if err != nil {
				t.Fatalf("跑 py 真身失败：%v", err)
			}
			if rcG != rcP {
				t.Fatalf("退码失配：py=%d go=%d\npy.stderr=%q\ngo.stderr=%q", rcP, rcG, pe.String(), se.String())
			}
			if po.String() != so.String() {
				t.Fatalf("stdout 失配：\n--py--\n%s\n--go--\n%s", po.String(), so.String())
			}
			if pe.String() != se.String() {
				t.Fatalf("stderr 失配（件头 ⑨：≤3.11 解释器应两侧皆空 · 哑化本件可致此红）：\n--py--\n%q\n--go--\n%q",
					pe.String(), se.String())
			}
		})
	}
}
