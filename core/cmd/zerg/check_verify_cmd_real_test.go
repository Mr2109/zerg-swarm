// check_verify_cmd_real_test.go —— 判据件 `scripts/gates/check-verify-cmd-real.py` Go 真搬的成对负控。
//
// 覆盖面（每条断言都带「牙」：改动实现即红 · `go test` 报出本文件的行号）：
//
//	TestCheckVerifyCmdRealHelp        · `-h` ⇒ rc=0 且 stdout 逐字 = 钉版帮助块
//	TestCheckVerifyCmdRealUsageErrs   · 未知旗标 / 歧义前缀 / 缺值 ⇒ rc=2 + argparse usage+error 正文
//	TestCheckVerifyCmdRealMutex       · `--json --self-test` 互斥 ⇒ rc=2 + 具名文案
//	TestCheckVerifyCmdRealSelfTest    · `--self-test` ⇒ rc=0 且含「自检结论: 全过（断言 25 条 · 失败 0 条）」
//	TestCheckVerifyCmdRealReport      · 夹具报告档：clean rc=0 / vio 明细逐条点名 / big --strict rc=1
//	TestCheckVerifyCmdRealJSON        · `--json` 信封（sort_keys + py 分隔符）逐字
//	TestCheckVerifyCmdRealFailClosed  · 缺账 / 空账 / 缺命令树 ⇒ rc=2（绝不报绿）
//	TestCheckVerifyCmdRealPyPrims     · py 词法原语：join 三差 / normpath 双斜杠 / basename / splitlines / 闭集 / 解析
//	TestCheckVerifyCmdRealGoldenInternalized · 十五格 argv 的 (rc,stdout,stderr) 冻结金标自证（哨兵锚 Go 半 · ✗ 不读 py）
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func cvrTestRun(argv ...string) (int, string, string) {
	var out, errb bytes.Buffer
	rc := checkVerifyCmdRealGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

func cvrTestWrite(t *testing.T, p, body string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("写夹具失败 %s: %v", p, err)
	}
}

// cvrTestFixture —— py `_fixture` 的等价物：建 root/core/.../cli-matrix.json + root/ledger.jsonl。
func cvrTestFixture(t *testing.T, cmds []string, rows []map[string]any) (string, string) {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "core", "cmd", "zerg", "testdata")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("建夹具失败: %v", err)
	}
	cases := make([]any, 0, len(cmds))
	for _, c := range cmds {
		cases = append(cases, map[string]any{"command": c})
	}
	mb, _ := json.Marshal(map[string]any{"cases": cases})
	cvrTestWrite(t, filepath.Join(root, "core", "cmd", "zerg", "testdata", "cli-matrix.json"), string(mb))
	ledger := filepath.Join(root, "ledger.jsonl")
	var sb strings.Builder
	for _, r := range rows {
		rb, _ := json.Marshal(r)
		sb.Write(rb)
		sb.WriteByte('\n')
	}
	cvrTestWrite(t, ledger, sb.String())
	return root, ledger
}

func TestCheckVerifyCmdRealHelp(t *testing.T) {
	rc, out, errb := cvrTestRun("-h")
	if rc != cvrRCOK {
		t.Fatalf("`-h` 应 rc=0，得 %d", rc)
	}
	if out != cvrHelp {
		t.Fatalf("帮助正文不逐字：\n got=%q\nwant=%q", out, cvrHelp)
	}
	if errb != "" {
		t.Fatalf("`-h` 不该写 stderr：%q", errb)
	}
}

func TestCheckVerifyCmdRealUsageErrs(t *testing.T) {
	rc, out, errb := cvrTestRun("--bogus")
	if rc != cvrRCBlocked {
		t.Fatalf("未知旗标应 rc=2，得 %d", rc)
	}
	want := cvrUsage + "\n" + cvrProg + ": error: unrecognized arguments: --bogus\n"
	if errb != want {
		t.Fatalf("未知旗标 stderr 不逐字：\n got=%q\nwant=%q", errb, want)
	}
	if out != "" {
		t.Fatalf("未知旗标不写 stdout：%q", out)
	}

	rc, _, errb = cvrTestRun("--s")
	if rc != cvrRCBlocked {
		t.Fatalf("歧义前缀应 rc=2，得 %d", rc)
	}
	wantA := cvrUsage + "\n" + cvrProg + ": error: ambiguous option: --s could match --self-test, --strict\n"
	if errb != wantA {
		t.Fatalf("歧义 stderr 不逐字：\n got=%q\nwant=%q", errb, wantA)
	}

	rc, _, errb = cvrTestRun("--max-examples")
	if rc != cvrRCBlocked {
		t.Fatalf("缺值应 rc=2，得 %d", rc)
	}
	wantM := cvrUsage + "\n" + cvrProg + ": error: argument --max-examples: expected one argument\n"
	if errb != wantM {
		t.Fatalf("缺值 stderr 不逐字：\n got=%q\nwant=%q", errb, wantM)
	}
}

func TestCheckVerifyCmdRealMutex(t *testing.T) {
	rc, out, errb := cvrTestRun("--json", "--self-test")
	if rc != cvrRCBlocked {
		t.Fatalf("互斥应 rc=2，得 %d", rc)
	}
	if out != "" {
		t.Fatalf("互斥档 stdout 应为空：%q", out)
	}
	want := "✗ 用法错（rc=2）：`--json` 与 `--self-test` 互斥（后者只产人读文本、不产 JSON）\n"
	if errb != want {
		t.Fatalf("互斥 stderr 不逐字：\n got=%q\nwant=%q", errb, want)
	}
}

func TestCheckVerifyCmdRealSelfTest(t *testing.T) {
	rc, out, _ := cvrTestRun("--self-test")
	if rc != cvrRCOK {
		t.Fatalf("`--self-test` 应 rc=0，得 %d", rc)
	}
	if !strings.HasPrefix(out, "── check-verify-cmd-real 自检（成对负控 · 真退码）──\n── check-verify-cmd-real 自检（成对负控 · 真退码）──\n") {
		t.Fatalf("自检 stdout 头不逐字（应两次表头）：%q", out[:cvrMinTest(len(out), 160)])
	}
	if !strings.Contains(out, "自检结论: 全过（断言 25 条 · 失败 0 条）") {
		t.Fatalf("自检结论不符：%q", out)
	}
}

func cvrMinTest(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func TestCheckVerifyCmdRealReport(t *testing.T) {
	root, ledger := cvrTestFixture(t, []string{"gate ls", "gate find", "task ls", "help"},
		[]map[string]any{
			{"id": "G-T1", "verify_cmd": "zerg gate ls", "repro_cmd": "zerg task ls"},
			{"id": "G-T2", "verify_cmd": "zerg gate find check-x", "repro_cmd": "go test ./... -run X"},
		})
	rc, out, _ := cvrTestRun("--no-self-test", "--root", root, "--ledger", ledger)
	if rc != cvrRCOK {
		t.Fatalf("报告档应 rc=0，得 %d", rc)
	}
	if !strings.Contains(out, "不可判 = **1 / 2**（其中 通用命令 1 · 不解析命令树 0）") {
		t.Fatalf("verify 计数不符：%q", out)
	}
	if !strings.Contains(out, "     - [通用] G-T1  ← zerg gate ls") {
		t.Fatalf("verify 明细未点名 G-T1：%q", out)
	}
	if !strings.Contains(out, "   样板 = **1 / 2**（通用命令）") {
		t.Fatalf("repro 计数不符：%q", out)
	}

	// --strict 红档：700 条通用 > 基线 635 ⇒ rc=1。
	rows := make([]map[string]any, 0, 700)
	for i := 0; i < 700; i++ {
		rows = append(rows, map[string]any{"id": "G-X", "verify_cmd": "zerg gate ls", "repro_cmd": "zerg task ls"})
	}
	broot, bledger := cvrTestFixture(t, []string{"gate ls"}, rows)
	rc, out, _ = cvrTestRun("--no-self-test", "--strict", "--root", broot, "--ledger", bledger)
	if rc != cvrRCFail {
		t.Fatalf("`--strict` 超额应 rc=1，得 %d", rc)
	}
	if !strings.Contains(out, "报告档 · 超额（报告档不阻断）：verify 样板 700 / 700（通用 700 · 不解析 0）· repro 样板 700 / 700") {
		t.Fatalf("strict 报告正文不符：%q", out)
	}
	// RED 文案只在 --json 档的 stderr 出现（py print_report 内部固定 strict=False）。
	jrc, _, jerr := cvrTestRun("--no-self-test", "--json", "--strict", "--root", broot, "--ledger", bledger)
	if jrc != cvrRCFail || !strings.Contains(jerr, "rc=1 · 超额：verify 700 > 基线 635（超 65）· repro 700 > 基线 526（超 174）") {
		t.Fatalf("strict RED stderr 不符：rc=%d err=%q", jrc, jerr)
	}
}

func TestCheckVerifyCmdRealJSON(t *testing.T) {
	root, ledger := cvrTestFixture(t, []string{"gate ls", "gate find"},
		[]map[string]any{{"id": "G-A", "verify_cmd": "zerg gate ls", "repro_cmd": "zerg task ls"}})
	rc, out, errb := cvrTestRun("--no-self-test", "--json", "--root", root, "--ledger", ledger)
	if rc != cvrRCOK {
		t.Fatalf("--json 应 rc=0，得 %d", rc)
	}
	// 信封：sort_keys（baseline,code,kind,rc,reason,repro,strict,total,verify）+ py 分隔符。
	if !strings.HasPrefix(out, "{\"baseline\": {\"repro\": 526, \"verify\": 635}, \"code\": \"REPORT\", \"kind\": \"VerifyCmdReal\", \"rc\": 0, ") {
		t.Fatalf("JSON 头不符（sort_keys/分隔符）：%q", out[:cvrMinTest(len(out), 180)])
	}
	if !strings.Contains(out, "\"verify\": {\"bad\": 1, \"generic\": 1, \"items\": [{\"cmd\": \"zerg gate ls\", \"id\": \"G-A\", \"why\": \"通用命令（闭集）\"}], \"unresolved\": 0}") {
		t.Fatalf("JSON verify 项不符：%q", out)
	}
	if !strings.HasSuffix(out, "\n") || !strings.Contains(errb, "rc=0 · 报告档 · ") {
		t.Fatalf("JSON stderr 摘要不符：out=%q err=%q", out, errb)
	}
}

func TestCheckVerifyCmdRealFailClosed(t *testing.T) {
	root, _ := cvrTestFixture(t, []string{"gate ls"}, []map[string]any{{"id": "G-1", "verify_cmd": "zerg gate ls"}})

	rc, _, errb := cvrTestRun("--no-self-test", "--root", root, "--ledger", filepath.Join(root, "nope.jsonl"))
	if rc != cvrRCBlocked || !strings.Contains(errb, "缺口账真源缺件：") {
		t.Fatalf("缺账应 rc=2 + 具名：rc=%d err=%q", rc, errb)
	}

	eroot, eledger := cvrTestFixture(t, []string{"gate ls"}, nil)
	rc, _, errb = cvrTestRun("--no-self-test", "--root", eroot, "--ledger", eledger)
	if rc != cvrRCBlocked || !strings.Contains(errb, "缺口账为空（0 行）⇒ 空转，不给结论") {
		t.Fatalf("空账应 rc=2 + 具名：rc=%d err=%q", rc, errb)
	}

	nroot, nledger := cvrTestFixture(t, []string{"gate ls"}, []map[string]any{{"id": "G-1", "verify_cmd": "zerg gate ls"}})
	_ = os.Remove(filepath.Join(nroot, "core", "cmd", "zerg", "testdata", "cli-matrix.json"))
	rc, _, errb = cvrTestRun("--no-self-test", "--root", nroot, "--ledger", nledger)
	if rc != cvrRCBlocked || !strings.Contains(errb, "命令树机器面缺件：") {
		t.Fatalf("缺命令树应 rc=2 + 具名：rc=%d err=%q", rc, errb)
	}
}

func TestCheckVerifyCmdRealPyPrims(t *testing.T) {
	// join 三条语义差
	if got := cvrPyJoin("a", "..", "b"); got != "a/../b" {
		t.Fatalf("join `..` 应原样留：%q", got)
	}
	if got := cvrPyJoin("repo", "/abs"); got != "/abs" {
		t.Fatalf("join 绝对后段应丢前段：%q", got)
	}
	if got := cvrPyJoin("a", "", "b"); got != "a/b" {
		t.Fatalf("join 空段：%q", got)
	}
	if got := cvrPyJoin("a", ""); got != "a/" {
		t.Fatalf("join 尾空段应留斜杠：%q", got)
	}
	// normpath 保留恰两个前导斜杠
	if got := cvrPyNormpath("//a/b/../c"); got != "//a/c" {
		t.Fatalf("normpath 双斜杠语义：%q", got)
	}
	if got := cvrPyNormpath("///a/b"); got != "/a/b" {
		t.Fatalf("normpath 三斜杠应折成单：%q", got)
	}
	if got := cvrPyBasename("a/"); got != "" {
		t.Fatalf("basename 尾斜杠应空：%q", got)
	}
	if got := cvrPyBasename("./bin/zerg"); got != "zerg" {
		t.Fatalf("basename 带路径：%q", got)
	}
	if got := cvrSplitLinesPy("a\r\nb\rc\n"); len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Fatalf("universal newlines 行切不符：%q", got)
	}
	// 闭集 + 解析
	if cvrIsGeneric(cvrTokensOf("zerg gate find check-x")) {
		t.Fatalf("闭集外不该咬：gate find check-x")
	}
	if !cvrIsGeneric(cvrTokensOf("./bin/zerg gate ls")) {
		t.Fatalf("basename 剥法失效：./bin/zerg gate ls")
	}
	tree := [][]string{cvrPySplit("gate find"), cvrPySplit("gate ls")}
	if !cvrResolves(cvrTokensOf("zerg gate find kw"), tree) {
		t.Fatalf("前缀解析失效：gate find kw")
	}
	if cvrResolves(cvrTokensOf("zerg"), tree) {
		t.Fatalf("只有 zerg 前缀不该解析")
	}
}

// ── 内化金标自证（旧对拍源 `scripts/gates/check-verify-cmd-real.py` 已退场 · 金标真源收回 Go 常量）──

var cvrTestSelfDirRE = regexp.MustCompile(`vcr-selftest-[^/\n]+`)

// cvrTestNorm —— 把夹具绝对路径（t.TempDir()）与自检临时目录（mkdtemp）归一为固定占位
// ⇒ 冻值与运行机的 TMPDIR / 随机段无关（跨机可复现）。替换次序：先具体夹具根，后 TMPDIR。
func cvrTestNorm(s, croot, vroot, eroot, nroot string) string {
	s = strings.ReplaceAll(s, nroot, "<NROOT>")
	s = strings.ReplaceAll(s, vroot, "<VROOT>")
	s = strings.ReplaceAll(s, eroot, "<EROOT>")
	s = strings.ReplaceAll(s, croot, "<CROOT>")
	if td := os.TempDir(); td != "" {
		s = strings.ReplaceAll(s, strings.TrimRight(td, "/")+"/", "<TMP>")
	}
	return cvrTestSelfDirRE.ReplaceAllString(s, "vcr-selftest-MASK")
}

// cvrTestRepoRoot —— 哨兵：锚 **Go 半** `core/cmd/zerg/check_verify_cmd_real.go`（✗ 不再锚 py）。
// 旧版锚 `scripts/gates/check-verify-cmd-real.py`：该 py 已退场 ⇒ 上溯恒找不到 ⇒ 恒 Skip，且跳因
// 「不在真仓内」名不副实（整测零断言）。改认 Go 半后它在盘（与本件同包）⇒ 真跑断言。
// ✗ 不再 Skip：Go 半是编译进本测试二进制的真源，上溯找不到它 = 真仓结构被破 ⇒ 硬错（fail-closed）。
func cvrTestRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("取 cwd 失败: %v", err)
	}
	for i := 0; i < 8; i++ {
		p := filepath.Join(dir, "core", "cmd", "zerg", "check_verify_cmd_real.go")
		if st, err := os.Stat(p); err == nil && st.Mode().IsRegular() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("哨兵失守：上溯未见 Go 半 core/cmd/zerg/check_verify_cmd_real.go（不是真仓树）")
	return ""
}

type cvrGoldenCase struct {
	name    string
	wantRC  int
	wantOut string
	wantErr string
}

// cvrGoldenFloor —— ② 对拍格数的**冻结下限**（棘轮 · 只许增不许减；本值 = 当次现读）。
// 十五格 argv 的冻结金标现读 15 格（见下 `cvrGoldenCases`）⇒ 结构静默变小（格数 < 15）即判红。
const cvrGoldenFloor = 15

var cvrGoldenCases = []cvrGoldenCase{
	{name: "help", wantRC: 0, wantOut: "usage: check-verify-cmd-real.py [-h] [--root ROOT] [--ledger LEDGER]\n                                [--matrix MATRIX] [--json] [--self-test]\n                                [--no-self-test] [--baseline] [--list-rules]\n                                [--strict] [--max-examples MAX_EXAMPLES]\n\n判据须能真判门（GAP-20260926-55）\n\noptional arguments:\n  -h, --help            show this help message and exit\n  --root ROOT\n  --ledger LEDGER\n  --matrix MATRIX\n  --json\n  --self-test\n  --no-self-test\n  --baseline\n  --list-rules\n  --strict\n  --max-examples MAX_EXAMPLES\n", wantErr: ""},
	{name: "unknown", wantRC: 2, wantOut: "", wantErr: "usage: check-verify-cmd-real.py [-h] [--root ROOT] [--ledger LEDGER]\n                                [--matrix MATRIX] [--json] [--self-test]\n                                [--no-self-test] [--baseline] [--list-rules]\n                                [--strict] [--max-examples MAX_EXAMPLES]\ncheck-verify-cmd-real.py: error: unrecognized arguments: --bogus\n"},
	{name: "ambig", wantRC: 2, wantOut: "", wantErr: "usage: check-verify-cmd-real.py [-h] [--root ROOT] [--ledger LEDGER]\n                                [--matrix MATRIX] [--json] [--self-test]\n                                [--no-self-test] [--baseline] [--list-rules]\n                                [--strict] [--max-examples MAX_EXAMPLES]\ncheck-verify-cmd-real.py: error: ambiguous option: --s could match --self-test, --strict\n"},
	{name: "maxnoval", wantRC: 2, wantOut: "", wantErr: "usage: check-verify-cmd-real.py [-h] [--root ROOT] [--ledger LEDGER]\n                                [--matrix MATRIX] [--json] [--self-test]\n                                [--no-self-test] [--baseline] [--list-rules]\n                                [--strict] [--max-examples MAX_EXAMPLES]\ncheck-verify-cmd-real.py: error: argument --max-examples: expected one argument\n"},
	{name: "mutex", wantRC: 2, wantOut: "", wantErr: "✗ 用法错（rc=2）：`--json` 与 `--self-test` 互斥（后者只产人读文本、不产 JSON）\n"},
	{name: "rules", wantRC: 0, wantOut: "check-verify-cmd-real.py 规则表（判据须能真判门 · GAP-20260926-55）\n  判据：一条账某列「能真判」 ⟺ ① 解析到命令树  且  ② 不是通用命令\n  通用命令**闭集**（去 zerg 前缀后 · 逐 token 比对 · 无子串匹配）：\n     · 1 词：help / doctor\n     · 2 词：首词 ∈ help / doctor  或  尾词 ∈ ls / status\n  面分：verify 面 ①② 都判 · repro 面 只判 ②（解析面不适用）\n  棘轮：BASELINE verify=635 · repro=526（只许下调）\n  退码：0 报告完成 · 1（仅 --strict 超额）· 2 不给结论\n", wantErr: ""},
	{name: "selftest", wantRC: 0, wantOut: "── check-verify-cmd-real 自检（成对负控 · 真退码）──\n── check-verify-cmd-real 自检（成对负控 · 真退码）──\n   ✓ 闭集：`zerg task ls` ⇒ 通用\n   ✓ 闭集：`zerg gate ls` ⇒ 通用\n   ✓ 闭集：`zerg gap ls` ⇒ 通用\n   ✓ 闭集：`zerg help dangerous` ⇒ 通用\n   ✓ 闭集：`zerg help` ⇒ 通用\n   ✓ 闭集：`zerg doctor` ⇒ 通用\n   ✓ 闭集：`zerg doctor --json` ⇒ 通用\n   ✓ 闭集：`zerg repo status` ⇒ 通用\n   ✓ 闭集：`zerg gate find check-x` ⇒ **非**通用（闭集外不咬）\n   ✓ 闭集：`zerg gate run --fast` ⇒ **非**通用\n   ✓ 闭集：`zerg model show <名>` ⇒ **非**通用\n   ✓ 闭集：`./bin/zerg gate ls`（带路径前缀）⇒ 通用（basename 剥法）\n   ✓ 解析：`zerg gate find kw` ⇒ 前缀命中 `gate find`\n   ✓ 解析：`zerg gate ls` ⇒ 命中 `gate ls`\n   ✓ 解析：`zerg 不存在命令abc` ⇒ **不解析**（负控 ⊥ 上一格）\n   ✓ 解析：只有 `zerg` 前缀 ⇒ 不解析\n   ✓ 混合夹具：verify 不可判 = 1（G-T1 通用 · G-T2 真判）\n   ✓ 混合夹具：repro 样板 = 1（G-T1 通用 · G-T2 自由文本不判）\n   ✓ 报告档：超额（1 > 0）仍 rc=0（**不阻断**）\n   ✓ --strict：超额 ⇒ rc=1（与上一格成对）\n   ✓ --strict：不超额（1 ≤ 99）⇒ rc=0（闭集判据非恒红）\n   ✓ 负控：无样板夹具 ⇒ verify 0 · repro 0\n   ✓ 缺件：账读不到 ⇒ rc=2\n   ✓ 空转：账 0 行 ⇒ rc=2（绝不报绿）\n   ✓ 缺件：命令树缺件 ⇒ rc=2\n自检结论: 全过（断言 25 条 · 失败 0 条）\n", wantErr: "✗ 不给结论（rc=2）：缺口账真源缺件：<TMP>vcr-selftest-MASK/mix/nope.jsonl（缺件 ⇒ 不给结论）\n✗ 不给结论（rc=2）：缺口账为空（0 行）⇒ 空转，不给结论\n✗ 不给结论（rc=2）：命令树机器面缺件：<TMP>vcr-selftest-MASK/nomx/core/cmd/zerg/testdata/cli-matrix.json（缺件 ⇒ 不给结论）\n"},
	{name: "clean", wantRC: 0, wantOut: "── 判据须能真判门（GAP-20260926-55 · 报告档首版）──\n   缺口账真源：<CROOT>/ledger.jsonl（1 行）\n   命令树真源：<CROOT>/core/cmd/zerg/testdata/cli-matrix.json\n   基线（棘轮 · 只许下调）：verify=635 · repro=526\n\n== verify_cmd 面（①② 两关都判）==\n   不可判 = **0 / 1**（其中 通用命令 0 · 不解析命令树 0）\n\n== repro_cmd 面（只判 ②「非通用命令」· 解析面不判）==\n   样板 = **0 / 1**（通用命令）\n   ★ 未覆盖形态：`--repro-cmd` 设计上收**自由文本** ⇒ 「解析到命令树」这一关**不适用**（属 `GAP-20260927-246` 的写侧面）。\n\n== 结论 ==\n   报告档 · 不超额：verify 样板 0 / 1（通用 0 · 不解析 0）· repro 样板 0 / 1\n   ⇒ 报告档（rc=0）；`--strict` 时超额 ⇒ rc=1\n", wantErr: ""},
	{name: "vio-report", wantRC: 0, wantOut: "── 判据须能真判门（GAP-20260926-55 · 报告档首版）──\n   缺口账真源：<VROOT>/ledger.jsonl（2 行）\n   命令树真源：<VROOT>/core/cmd/zerg/testdata/cli-matrix.json\n   基线（棘轮 · 只许下调）：verify=635 · repro=526\n\n== verify_cmd 面（①② 两关都判）==\n   不可判 = **2 / 2**（其中 通用命令 1 · 不解析命令树 1）\n     - [通用] G-A  ← zerg gate ls\n     - [不解析] G-B  ← zerg nonexistent-zz\n\n== repro_cmd 面（只判 ②「非通用命令」· 解析面不判）==\n   样板 = **1 / 2**（通用命令）\n     - [通用] G-A  ← zerg task ls\n   ★ 未覆盖形态：`--repro-cmd` 设计上收**自由文本** ⇒ 「解析到命令树」这一关**不适用**（属 `GAP-20260927-246` 的写侧面）。\n\n== 结论 ==\n   报告档 · 不超额：verify 样板 2 / 2（通用 1 · 不解析 1）· repro 样板 1 / 2\n   ⇒ 报告档（rc=0）；`--strict` 时超额 ⇒ rc=1\n", wantErr: ""},
	{name: "vio-json", wantRC: 0, wantOut: "{\"baseline\": {\"repro\": 526, \"verify\": 635}, \"code\": \"REPORT\", \"kind\": \"VerifyCmdReal\", \"rc\": 0, \"reason\": \"报告档 · 不超额：verify 样板 2 / 2（通用 1 · 不解析 1）· repro 样板 1 / 2\", \"repro\": {\"bad\": 1, \"generic\": 1, \"items\": [{\"cmd\": \"zerg task ls\", \"id\": \"G-A\", \"why\": \"通用命令（闭集）\"}]}, \"schema\": \"zerg/v1\", \"strict\": false, \"total\": 2, \"verify\": {\"bad\": 2, \"generic\": 1, \"items\": [{\"cmd\": \"zerg gate ls\", \"id\": \"G-A\", \"why\": \"通用命令（闭集）\"}, {\"cmd\": \"zerg nonexistent-zz\", \"id\": \"G-B\", \"why\": \"不解析命令树\"}], \"unresolved\": 1}}\n", wantErr: "── 自检（成对负控）──\n✗ 不给结论（rc=2）：缺口账真源缺件：<TMP>vcr-selftest-MASK/mix/nope.jsonl（缺件 ⇒ 不给结论）\n✗ 不给结论（rc=2）：缺口账为空（0 行）⇒ 空转，不给结论\n✗ 不给结论（rc=2）：命令树机器面缺件：<TMP>vcr-selftest-MASK/nomx/core/cmd/zerg/testdata/cli-matrix.json（缺件 ⇒ 不给结论）\n── check-verify-cmd-real 自检（成对负控 · 真退码）──\n   ✓ 闭集：`zerg task ls` ⇒ 通用\n   ✓ 闭集：`zerg gate ls` ⇒ 通用\n   ✓ 闭集：`zerg gap ls` ⇒ 通用\n   ✓ 闭集：`zerg help dangerous` ⇒ 通用\n   ✓ 闭集：`zerg help` ⇒ 通用\n   ✓ 闭集：`zerg doctor` ⇒ 通用\n   ✓ 闭集：`zerg doctor --json` ⇒ 通用\n   ✓ 闭集：`zerg repo status` ⇒ 通用\n   ✓ 闭集：`zerg gate find check-x` ⇒ **非**通用（闭集外不咬）\n   ✓ 闭集：`zerg gate run --fast` ⇒ **非**通用\n   ✓ 闭集：`zerg model show <名>` ⇒ **非**通用\n   ✓ 闭集：`./bin/zerg gate ls`（带路径前缀）⇒ 通用（basename 剥法）\n   ✓ 解析：`zerg gate find kw` ⇒ 前缀命中 `gate find`\n   ✓ 解析：`zerg gate ls` ⇒ 命中 `gate ls`\n   ✓ 解析：`zerg 不存在命令abc` ⇒ **不解析**（负控 ⊥ 上一格）\n   ✓ 解析：只有 `zerg` 前缀 ⇒ 不解析\n   ✓ 混合夹具：verify 不可判 = 1（G-T1 通用 · G-T2 真判）\n   ✓ 混合夹具：repro 样板 = 1（G-T1 通用 · G-T2 自由文本不判）\n   ✓ 报告档：超额（1 > 0）仍 rc=0（**不阻断**）\n   ✓ --strict：超额 ⇒ rc=1（与上一格成对）\n   ✓ --strict：不超额（1 ≤ 99）⇒ rc=0（闭集判据非恒红）\n   ✓ 负控：无样板夹具 ⇒ verify 0 · repro 0\n   ✓ 缺件：账读不到 ⇒ rc=2\n   ✓ 空转：账 0 行 ⇒ rc=2（绝不报绿）\n   ✓ 缺件：命令树缺件 ⇒ rc=2\n自检结论: 全过（断言 25 条 · 失败 0 条）\n\nrc=0 · 报告档 · 不超额：verify 样板 2 / 2（通用 1 · 不解析 1）· repro 样板 1 / 2\n"},
	{name: "clean-baseline", wantRC: 0, wantOut: "── 判据须能真判门（GAP-20260926-55 · 报告档首版）──\n   缺口账真源：<CROOT>/ledger.jsonl（1 行）\n   命令树真源：<CROOT>/core/cmd/zerg/testdata/cli-matrix.json\n   基线（棘轮 · 只许下调）：verify=635 · repro=526\n\n== verify_cmd 面（①② 两关都判）==\n   不可判 = **0 / 1**（其中 通用命令 0 · 不解析命令树 0）\n\n== repro_cmd 面（只判 ②「非通用命令」· 解析面不判）==\n   样板 = **0 / 1**（通用命令）\n   ★ 未覆盖形态：`--repro-cmd` 设计上收**自由文本** ⇒ 「解析到命令树」这一关**不适用**（属 `GAP-20260927-246` 的写侧面）。\n\n== 结论 ==\n   报告档 · 不超额：verify 样板 0 / 1（通用 0 · 不解析 0）· repro 样板 0 / 1\n   ⇒ 报告档（rc=0）；`--strict` 时超额 ⇒ rc=1\n── 棘轮视图（只读 · 不改任何文件）──\n   verify 面不可判 现测 = 0 · 基线 = 635 · 剩余（可下调到）= 0\n   repro  面样板   现测 = 0 · 基线 = 526 · 剩余 = 0\n   自述 = 2026-10-05 首测（本门新建当天）：账 1180 行 · 命令树 188 条 ⇒ verify 面不可判 = 675（其中 通用命令 675 · 不解析命令树 0）· repro 面样板 = 526。★ 2026-10-12 棘轮收紧（缺口 `GAP-20261010-34`）：账 1295 行 ⇒ verify 通用命令 = 635 · 不解析命令树 = 0（现读即顶格）· repro 面样板 = 526（未变）。★ 只认 `BASELINE` 那两格；本段历史值仅供追溯。\n   号令：一次只许**下调**基线（改成现测值），**永不上调**；超额 ⇒ 先补真判据或挪出闭集，✗ 不许抬基线。\n", wantErr: ""},
	{name: "miss-ledger", wantRC: 2, wantOut: "", wantErr: "✗ 不给结论（rc=2）：缺口账真源缺件：<CROOT>/nope.jsonl（缺件 ⇒ 不给结论）\n"},
	{name: "empty-ledger", wantRC: 2, wantOut: "", wantErr: "✗ 不给结论（rc=2）：缺口账为空（0 行）⇒ 空转，不给结论\n"},
	{name: "miss-matrix", wantRC: 2, wantOut: "", wantErr: "✗ 不给结论（rc=2）：命令树机器面缺件：<NROOT>/core/cmd/zerg/testdata/cli-matrix.json（缺件 ⇒ 不给结论）\n"},
	{name: "miss-ledger-json", wantRC: 2, wantOut: "{\"kind\": \"VerifyCmdRealError\", \"rc\": 2, \"reason\": \"缺口账真源缺件：<CROOT>/nope.jsonl（缺件 ⇒ 不给结论）\", \"schema\": \"zerg/v1\"}\n", wantErr: "✗ 不给结论（rc=2）：缺口账真源缺件：<CROOT>/nope.jsonl（缺件 ⇒ 不给结论）\n"},
}

// TestCheckVerifyCmdRealGoldenInternalized —— 十五格 argv 的 (rc, stdout, stderr) **冻结金标自证**
// （旧名 TestCheckVerifyCmdRealParityPy：py ⟷ Go 逐字对拍）。对拍源 `scripts/gates/check-verify-cmd-real.py`
// 已退场 ⇒ 旧测恒 Skip；按本仓先例（archive_mount_test.go · TestArchiveMountTwoStatesGoldenInternalized）
// 把金标真源**收回 Go 常量本身**：合成夹具现建 · 现跑本入口 · 与冻值逐字比
// （✗ 不再读任何 py · ✗ 不放宽为「不检/只判非空」）。仿 archive_mount 的 amNorm，夹具绝对路径归一为
// <CROOT>/<VROOT>/<EROOT>/<NROOT>、自检 mkdtemp 归一为 <TMP>…vcr-selftest-MASK ⇒ 跨机可复现。
// ★ 哑化负控：改被测判据任一臂（cvrIsGeneric / cvrBaseline / 报告文案 / JSON 形态）⇒ 本用例 FAIL（带行号）。
func TestCheckVerifyCmdRealGoldenInternalized(t *testing.T) {
	_ = cvrTestRepoRoot(t) // 哨兵：锚 Go 半在盘 ⇒ 真断言（✗ 不 Skip）
	croot, cledger := cvrTestFixture(t, []string{"gate ls", "gate find", "task ls", "help", "doctor", "gap ls", "repo status"},
		[]map[string]any{{"id": "G-OK", "verify_cmd": "zerg gate find check-x", "repro_cmd": "go test ./... -run X"}})
	vroot, vledger := cvrTestFixture(t, []string{"gate ls", "gate find"},
		[]map[string]any{
			{"id": "G-A", "verify_cmd": "zerg gate ls", "repro_cmd": "zerg task ls"},
			{"id": "G-B", "verify_cmd": "zerg nonexistent-zz", "repro_cmd": "go test"},
		})
	eroot, eledger := cvrTestFixture(t, []string{"gate ls"}, nil)
	nroot, nledger := cvrTestFixture(t, []string{"gate ls"}, []map[string]any{{"id": "G-1", "verify_cmd": "zerg gate ls"}})
	_ = os.Remove(filepath.Join(nroot, "core", "cmd", "zerg", "testdata", "cli-matrix.json"))

	argvs := map[string][]string{
		"help":             {"-h"},
		"unknown":          {"--bogus"},
		"ambig":            {"--s"},
		"maxnoval":         {"--max-examples"},
		"mutex":            {"--json", "--self-test"},
		"rules":            {"--list-rules"},
		"selftest":         {"--self-test"},
		"clean":            {"--no-self-test", "--root", croot, "--ledger", cledger},
		"vio-report":       {"--no-self-test", "--root", vroot, "--ledger", vledger},
		"vio-json":         {"--json", "--root", vroot, "--ledger", vledger},
		"clean-baseline":   {"--no-self-test", "--baseline", "--root", croot, "--ledger", cledger},
		"miss-ledger":      {"--no-self-test", "--root", croot, "--ledger", filepath.Join(croot, "nope.jsonl")},
		"empty-ledger":     {"--no-self-test", "--root", eroot, "--ledger", eledger},
		"miss-matrix":      {"--no-self-test", "--root", nroot, "--ledger", nledger},
		"miss-ledger-json": {"--json", "--no-self-test", "--root", croot, "--ledger", filepath.Join(croot, "nope.jsonl")},
	}
	if len(cvrGoldenCases) < 8 {
		t.Fatalf("对拍格数 %d < 8", len(cvrGoldenCases))
	}
	// ② 冻结下限（棘轮 · 只许增）：对拍格数 < 当次现读数下限 15 ⇒ 判红（结构静默变小）。
	if n := len(cvrGoldenCases); n < cvrGoldenFloor {
		t.Fatalf("对拍格数 %d < 冻结下限 %d（棘轮只许增 · 结构静默变小）", n, cvrGoldenFloor)
	}
	for _, c := range cvrGoldenCases {
		t.Run(c.name, func(t *testing.T) {
			argv, ok := argvs[c.name]
			if !ok {
				t.Fatalf("缺 argv: %s", c.name)
			}
			rc, out, errb := cvrTestRun(argv...)
			gotOut := cvrTestNorm(out, croot, vroot, eroot, nroot)
			gotErr := cvrTestNorm(errb, croot, vroot, eroot, nroot)
			if rc != c.wantRC {
				t.Errorf("[%s] rc: 实得 %d（冻值 %d）", c.name, rc, c.wantRC)
			}
			if gotOut != c.wantOut {
				t.Errorf("[%s] stdout 漂移\n实得=%q\n冻值=%q", c.name, gotOut, c.wantOut)
			}
			if gotErr != c.wantErr {
				t.Errorf("[%s] stderr 漂移\n实得=%q\n冻值=%q", c.name, gotErr, c.wantErr)
			}
		})
	}
}
