// gen_gap_ledger_test.go —— `gen_gap_ledger.go`（`scripts/docs/gen-gap-ledger.py` 的 Go 真搬）
// 成对负控。断言全是「只有实现对才过」的**逐字**等式（哑化任一语义面必红 —— 见回执的
// 仓外副本哑化实证：把主件语义改坏 ⇒ 本件 FAIL 带行号）＋ 空值/缺输入/两态各成一子用例。
//
// ★ 金标内化（件头 ⑨ · 未覆盖面）：原 `TestGglPyParityPinnedInterpreter`（py 真身 ⟷ 本入口
//
//	三面逐字对拍）的对拍源 `scripts/docs/gen-gap-ledger.py` 已退场（M2 批）⇒ 断言恒 Skip（假绿、
//	零保护）。按先例（atomic_write_test.go / pack_release_test.go / eval_run_all_test.go）把金标
//	真源**收回 Go 半现役行为本身**：逐格 argv ⇒ (rc, stdout, stderr) 由 `genGapLedgerGateMain`
//	现跑捕获、冻成 Go 常量（见 `TestGenGapLedgerGoldenInternalized` 段）。`--help` **不入金标集**：
//	其正文逐**解释器版本**变（≤3.9 `optional arguments:` ⟷ ≥3.10 `options:`）⇒ 归件头 ① 的
//	**未覆盖面（环境依赖）**，本集只用**版本无关** argv。
//
// ★ 无 TestMain / 无包级 init（撞名规避，照 `gen_tools_md_test.go` 同规）；helper 一律 `ggl` 前缀。
package main

import (
	"bytes"
	"fmt"
	"os"
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

// ── 内化金标：旧 py 对拍（恒 Skip · 零保护）的逐格金标收成 Go 常量（现算现冻 · ✗ 不臆造）──────
//
// ★ 金标计算式（✗ 不臆造 · 逐格由「Go 半现役行为现算」得到）：逐格「argv ⇒ (rc, stdout,
//   stderr)」由 `genGapLedgerGateMain` 现跑捕获（一次性现算、✗ 不落仓），冻成下表 `gglGoldenCases`。
//   本组 argv 均**版本无关**、输出**不含时刻/随机路径** ⇒ 无归一化需求（照实记录：本组无本机
//   时点类量可比，故不设归一函数）。
//
// 两态（✗ 不许 Skip）：A 态 = 合成夹具无 py 真身（= 真仓现态）；B 态 = 合成「旧态」夹具把退役 py
//   路径 `scripts/docs/gen-gap-ledger.py` 的 stub 放回 ⇒ 两态对同一套金标逐字同命中（Go 真身
//   ✗ 读 py）。
//
// ★ 哑化负控（"牙" · 见交付读数）：`-overlay` 把 gen_gap_ledger.go 的一条出口臂哑化 ⇒ 本用例
//   **必 FAIL 且带行号**（✗ 不真改仓内源 · 免 `dev edit` 前的仓内污染）。

// gglGoHalfRel —— 哨兵锚：Go 半（✗ 不锚已退场的 py —— 一退役即失锚）。
const gglGoHalfRel = "core/cmd/zerg/gen_gap_ledger.go"

// gglPyRetiredRel —— 已退场 py 真身的仓根相对路径；「两态」就钉在它「在 / 不在盘」。
const gglPyRetiredRel = "scripts/docs/gen-gap-ledger.py"

// gglGoldenSelfTestOut —— `--self-test` 档的冻结 stdout（由 `genGapLedgerGateMain` 现跑捕获 ·
// 见段头金标计算式；`gglSelfTest` 只打固定行、无时刻/随机路径 ⇒ 逐字节可复现）。
const gglGoldenSelfTestOut = "  ✓ 版本线挑选（v2.5.13 > v2.5.9 · 豁免件不算）（got='v2.5.13' · want='v2.5.13'）\n" +
	"  ✓ 落点形状（got='项目文档/v2.5.13/缺口总账-20260926.md' · want='项目文档/v2.5.13/缺口总账-20260926.md'）\n" +
	"  ✓ 日期取读（首行称日期）（got='20260926' · want='20260926'）\n" +
	"  ✓ 日期取读（取不到 ⇒ None · 不给结论）（got=None · want=None）\n" +
	"  ✓ 空文档仓 ⇒ 版本线判不出（got=None · want=None）\n" +
	"  ✓ qid 未启用 ⇒ enabled=False · rc=None（不判红）（got=(False, None) · want=(False, None)）\n" +
	"  ✓ qid 合法 ⇒ rc=0 · 计 2/3（got=(True, 0, 2) · want=(True, 0, 2)）\n" +
	"  ✓ qid 形态非法 ⇒ rc=1 · 两处点名（got=(1, [('a', 'Q1'), ('b', 'q-002')]) · want=(1, [('a', 'Q1'), ('b', 'q-002')])）\n" +
	"  ✓ qid 重号 ⇒ rc=1 · 点名 Q-007×2（got=(1, [('Q-007', ['a', 'b'])]) · want=(1, [('Q-007', ['a', 'b'])])）\n" +
	"  ✓ 非 JSON 行 ⇒ 不给结论（rows=None · 有说明）（got=(True, True) · want=(True, True)）\n" +
	"  ✓ 真源不存在 ⇒ 不给结论（got=(True, True) · want=(True, True)）\n" +
	"自检结论: 全过\n"

// gglSentinel —— 哨兵：上溯找 **Go 半**（本件真源 · 编译进测试二进制）。找不到 ⇒ fail-closed
// （✗ 不留 Skip）：上溯见不到它 = 真仓结构被破 ⇒ 硬错。旧版对上溯不到 py 就 `t.Skipf`（恒 Skip ·
// 零保护）—— 本改把它换成硬前置。
func gglSentinel(t *testing.T) string {
	t.Helper()
	want := filepath.FromSlash(gglGoHalfRel)
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("取 cwd 失败：%v", err)
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
	t.Fatalf("哨兵失守：上溯未见 Go 半 %s（不是真仓树）", gglGoHalfRel)
	return ""
}

// gglIsFile —— 判在盘且是普通件（哨兵腿用）。
func gglIsFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.Mode().IsRegular()
}

type gglGoldenCase struct {
	name    string
	argv    []string
	wantRC  int
	wantOut string // 现算自证（由 Go 半现跑捕获）· ✗ 手写猜
	wantErr string
}

// gglGoldenCases —— 金标真源（5 格 · 与旧对拍逐格同 argv）。want* 由 `genGapLedgerGateMain`
// 现跑捕获（见段头金标计算式）。复用 `gglVersion` / `gglUsage` / `gglSelfName` 常量（✗ 不手抄，
// 免双份漂移）。
var gglGoldenCases = []gglGoldenCase{
	{name: "self-test", argv: []string{"--self-test"}, wantRC: 0, wantOut: gglGoldenSelfTestOut, wantErr: ""},
	{name: "version", argv: []string{"--version"}, wantRC: 0, wantOut: gglVersion + "\n", wantErr: ""},
	{name: "nope", argv: []string{"--nope"}, wantRC: 2, wantOut: "",
		wantErr: gglUsage + gglSelfName + ": error: unrecognized arguments: --nope\n"},
	{name: "docs-root-missing-arg", argv: []string{"--docs-root"}, wantRC: 2, wantOut: "",
		wantErr: gglUsage + gglSelfName + ": error: argument --docs-root: expected one argument\n"},
	{name: "emit-explicit-arg", argv: []string{"--emit=1"}, wantRC: 2, wantOut: "",
		wantErr: gglUsage + gglSelfName + ": error: argument --emit: ignored explicit argument '1'\n"},
}

// gglGoldenFixture —— 仓外金标夹具（= cwd）；withPy 时把退役 py 的合成 stub 落回（B 态「旧态」）。
func gglGoldenFixture(t *testing.T, withPy bool) string {
	t.Helper()
	fx := t.TempDir()
	if withPy {
		p := filepath.Join(fx, filepath.FromSlash(gglPyRetiredRel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("#!/usr/bin/env python3\n# [退役占位] scripts/docs/gen-gap-ledger.py —— 真件已退场（Go 半承载）；stub 仅保夹具形状。\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return fx
}

// gglRunGoldenIn —— 逐格现跑 Go 半（cwd = 夹具根），与冻值逐字比；返回失配格数。
func gglRunGoldenIn(t *testing.T, tag, root string) int {
	t.Helper()
	t.Chdir(root)
	mism := 0
	for _, c := range gglGoldenCases {
		var so, se bytes.Buffer
		rc := genGapLedgerGateMain(&so, &se, c.argv)
		if rc != c.wantRC {
			mism++
			t.Errorf("[%s/%s] rc=%d（冻值 %d）", tag, c.name, rc, c.wantRC)
		}
		if so.String() != c.wantOut {
			mism++
			t.Errorf("[%s/%s] stdout 漂移\n实得=%q\n冻值=%q", tag, c.name, so.String(), c.wantOut)
		}
		if se.String() != c.wantErr {
			mism++
			t.Errorf("[%s/%s] stderr 漂移\n实得=%q\n冻值=%q", tag, c.name, se.String(), c.wantErr)
		}
	}
	return mism
}

// TestGenGapLedgerGoldenInternalized —— A 态（真仓 · 退役 py 不在盘）金标自证
// （✗ 不 Skip · 逐格真断言 · ✗ 不放宽为「只判非空」）。
func TestGenGapLedgerGoldenInternalized(t *testing.T) {
	repo := gglSentinel(t) // 哨兵：锚 Go 半在盘 ⇒ fail-closed（✗ 不 Skip）
	if gglIsFile(filepath.Join(repo, filepath.FromSlash(gglPyRetiredRel))) {
		t.Fatalf("A 态前提破：退役 py 竟在盘 %s（本件前提 = 内化后无 py）", gglPyRetiredRel)
	}
	if n := gglRunGoldenIn(t, "A夹具无py", gglGoldenFixture(t, false)); n != 0 {
		t.Fatalf("A 态金标失配 %d 格", n)
	}
}

// TestGenGapLedgerGoldenTwoStates —— 两态逐字同：A 态 = 夹具无 py（= 真仓现态）；B 态 = 合成「旧态」
// 夹具（把退役 py 路径 `scripts/docs/gen-gap-ledger.py` 的 stub 放回）⇒ 两态对同一套金标逐字同
// 命中（✗ 不 Skip）。证 py 在/不在盘都不改 Go 半读数（Go 真身 ✗ 读 py）。
func TestGenGapLedgerGoldenTwoStates(t *testing.T) {
	if n := gglRunGoldenIn(t, "A夹具无py", gglGoldenFixture(t, false)); n != 0 {
		t.Fatalf("A 态金标失配 %d 格", n)
	}
	if n := gglRunGoldenIn(t, "B旧态有py", gglGoldenFixture(t, true)); n != 0 {
		t.Fatalf("B 态金标失配 %d 格（py 在盘不得改判据结论）", n)
	}
}
