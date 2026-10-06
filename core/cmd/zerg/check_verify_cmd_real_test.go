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
//	TestCheckVerifyCmdRealParityPy    · 仓外对拍台（不落地）：同 argv 跑 py 原件 ⟷ 本入口，三面逐字（≥8 格）
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
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

	// --strict 红档：700 条通用 > 基线 675 ⇒ rc=1。
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
	if jrc != cvrRCFail || !strings.Contains(jerr, "rc=1 · 超额：verify 700 > 基线 675（超 25）· repro 700 > 基线 526（超 174）") {
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
	if !strings.HasPrefix(out, "{\"baseline\": {\"repro\": 526, \"verify\": 675}, \"code\": \"REPORT\", \"kind\": \"VerifyCmdReal\", \"rc\": 0, ") {
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

// ── 仓外对拍台（不落地）：py 原件 ⟷ 本入口 三面逐字 ──

func cvrTestRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Skip("取 cwd 失败")
	}
	for i := 0; i < 8; i++ {
		p := filepath.Join(dir, "scripts", "gates", "check-verify-cmd-real.py")
		if st, err := os.Stat(p); err == nil && st.Mode().IsRegular() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Skip("不在真仓内，跳过 py 对拍")
	return ""
}

func cvrTestPyRun(t *testing.T, repo string, argv []string) (int, string, string) {
	t.Helper()
	py := "/usr/bin/python3"
	if _, err := os.Stat(py); err != nil {
		t.Skip("无 /usr/bin/python3，跳过对拍")
	}
	pyPath := filepath.Join(repo, "scripts", "gates", "check-verify-cmd-real.py")
	cmd := exec.Command(py, append([]string{pyPath}, argv...)...)
	cmd.Dir = repo
	var so, se bytes.Buffer
	cmd.Stdout = &so
	cmd.Stderr = &se
	err := cmd.Run()
	rc := 0
	if ee, ok := err.(*exec.ExitError); ok {
		rc = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("起 py 失败: %v", err)
	}
	return rc, so.String(), cvrTestMaskSelfDir(se.String())
}

var cvrTestSelfDirRE = regexp.MustCompile(`vcr-selftest-[^/\n]+`)

func cvrTestMaskSelfDir(s string) string {
	// mkdtemp 随机后缀（py 与 Go 各一个）⇒ 掩码该段（非确定面）。
	return cvrTestSelfDirRE.ReplaceAllString(s, "vcr-selftest-MASK")
}

func TestCheckVerifyCmdRealParityPy(t *testing.T) {
	repo := cvrTestRepoRoot(t)
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

	cases := []struct {
		name string
		argv []string
	}{
		{"help", []string{"-h"}},
		{"unknown", []string{"--bogus"}},
		{"ambig", []string{"--s"}},
		{"maxnoval", []string{"--max-examples"}},
		{"mutex", []string{"--json", "--self-test"}},
		{"rules", []string{"--list-rules"}},
		{"selftest", []string{"--self-test"}},
		{"clean", []string{"--no-self-test", "--root", croot, "--ledger", cledger}},
		{"vio-report", []string{"--no-self-test", "--root", vroot, "--ledger", vledger}},
		{"vio-json", []string{"--json", "--root", vroot, "--ledger", vledger}},
		{"clean-baseline", []string{"--no-self-test", "--baseline", "--root", croot, "--ledger", cledger}},
		{"miss-ledger", []string{"--no-self-test", "--root", croot, "--ledger", filepath.Join(croot, "nope.jsonl")}},
		{"empty-ledger", []string{"--no-self-test", "--root", eroot, "--ledger", eledger}},
		{"miss-matrix", []string{"--no-self-test", "--root", nroot, "--ledger", nledger}},
		{"miss-ledger-json", []string{"--json", "--no-self-test", "--root", croot, "--ledger", filepath.Join(croot, "nope.jsonl")}},
	}
	if len(cases) < 8 {
		t.Fatalf("对拍格数 %d < 8", len(cases))
	}
	for _, c := range cases {
		pyRC, pyOut, pyErr := cvrTestPyRun(t, repo, c.argv)
		goRC, goOut, goErr := cvrTestRun(c.argv...)
		goErr = cvrTestMaskSelfDir(goErr)
		if pyRC != goRC || pyOut != goOut || pyErr != goErr {
			t.Fatalf("对拍失配 [%s] argv=%v\n rc: py=%d go=%d\n stdout: py=%q\n         go=%q\n stderr: py=%q\n         go=%q",
				c.name, c.argv, pyRC, goRC, pyOut, goOut, pyErr, goErr)
		}
	}
}
