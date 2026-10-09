package main

// check_gap_axis_reconcile_test.go —— `check_gap_axis_reconcile.go`（门75 Go 真搬）的**成对负控**测试件。
//
// 记法：正控（本门该绿）＋ 负控（本门该红 / 该 rc=2）各成子用例；每条「顺手止血」都配一条能咬住它的回归牙。
// 夹具一律落 `t.TempDir()`（仓外）＋ 一件**仓外合成 git 仓**（戊/己 的取数面：`git ls-files` 与
// 真源件 `core/cmd/zerg/family_gap.go`），**不碰真仓、不碰真账**。
// 测件里 python3 一律钉成 /usr/bin/python3（GAP-20261001-61）；跑后计数 0（自检临时目录不泄漏，
// TestGapAxisRecSelfTestAndTempdirDelta 是那一格的回归牙）。

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const gaaTestPython = "/usr/bin/python3"

// gaaTestFamilyGap —— 合成真源件：`gapModuleOfUnit` 的四个锚点逐字齐（三常量 + 手写占位前缀 +
// `filepath.Dir` + `dir == "."` 兜底），口径与真源同形。
const gaaTestFamilyGap = `package main

import "path/filepath"

const (
	gapUnitNoFile  = "无件(命令面)"
	gapUnitNoneMod = "无件"
	gapUnitRootMod = "仓根"
)

func gapModuleOfUnit(unit string) string {
	if unit == "" || unit == gapUnitNoFile || strings.HasPrefix(unit, "拟(") {
		return gapUnitNoneMod
	}
	dir := filepath.Dir(unit)
	if dir == "." || dir == "" || dir == string(filepath.Separator) {
		return gapUnitRootMod
	}
	return dir
}
`

// gaaTestFakeRepo —— 仓外合成仓：`git init` + 两件入索引（core/a.go 与合成 family_gap.go）。
func gaaTestFakeRepo(t *testing.T) string {
	t.Helper()
	repo := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(filepath.Join(repo, "core", "cmd", "zerg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "core", "cmd", "zerg", "family_gap.go"), []byte(gaaTestFamilyGap), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "core", "a.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "-A"}} {
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v 失败：%v\n%s", args, err, out)
		}
	}
	return repo
}

func gaaTestWrite(t *testing.T, path string, lines ...string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func gaaTestWriteRaw(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func gaaTestCall(args ...string) (int, string, string) {
	var out, errb strings.Builder
	rc := checkGapAxisReconcileGateMain(&out, &errb, args)
	return rc, out.String(), errb.String()
}

// gaaTestLedger —— 一正一「无件」的合成账（两轴等式都恰好成立 ⇒ 该绿）。
func gaaTestLedger(t *testing.T, lines ...string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "gap.jsonl")
	gaaTestWrite(t, p, lines...)
	return p
}

const gaaGoodYou = `{"id":"A1","state":"仍缺","unit":"core/a.go","module":"core"}`
const gaaGoodWu = `{"id":"A2","state":"仍缺","unit":"无件(命令面)","module":"无件"}`

// ── 正控 ─────────────────────────────────────────────────────────────────────

// 正控：两轴等式全成立 · 戊/己 无新命中 ⇒ rc=0。
func TestCheckGapAxisReconcileGatePort(t *testing.T) {
	repo := gaaTestFakeRepo(t)
	led := gaaTestLedger(t, gaaGoodYou, gaaGoodWu)
	rc, out, errb := gaaTestCall("--repo", repo, "--ledger", led)
	if rc != 0 {
		t.Fatalf("该 rc=0，实 rc=%d\nstdout:\n%s\nstderr:\n%s", rc, out, errb)
	}
	for _, want := range []string{"差 = 0", "Σ=1", "戊 件面现读存在性 : 受检 1 条", "己 轴间一致性", "⇒ rc=0 PASS"} {
		if !strings.Contains(out, want) {
			t.Fatalf("stdout 该含 %q：\n%s", want, out)
		}
	}
}

// 正控：`--json` 末尾追加机器面一行（含两轴 diff 与两枚现读计数）。
func TestGapAxisRecJSONFace(t *testing.T) {
	repo := gaaTestFakeRepo(t)
	led := gaaTestLedger(t, gaaGoodYou, gaaGoodWu)
	rc, out, _ := gaaTestCall("--repo", repo, "--ledger", led, "--json")
	if rc != 0 {
		t.Fatalf("该 rc=0，实 rc=%d\n%s", rc, out)
	}
	last := out[strings.LastIndex(strings.TrimRight(out, "\n"), "\n")+1:]
	for _, want := range []string{`"rem_total": 2`, `"diff": 0`, `"existence_hits": 0`, `"consistency_hits": 0`, `"rc": 0`} {
		if !strings.Contains(last, want) {
			t.Fatalf("机器面该含 %q：\n%s", want, last)
		}
	}
}

// ── 负控（该红 · rc=1）───────────────────────────────────────────────────────

// 负控：删掉一条的 `unit` 键 ⇒ 不入任一桶 ⇒ 甲 差 = 1（逐条点名）。
func TestGapAxisRecNegMissingUnitKey(t *testing.T) {
	repo := gaaTestFakeRepo(t)
	led := gaaTestLedger(t, `{"id":"GAP-20260928-9101","state":"仍缺","module":"core"}`, gaaGoodWu)
	rc, out, _ := gaaTestCall("--repo", repo, "--ledger", led)
	if rc != 1 {
		t.Fatalf("该 rc=1，实 rc=%d\n%s", rc, out)
	}
	for _, want := range []string{"差 = 1", "键面缺失", "GAP-20260928-9101"} {
		if !strings.Contains(out, want) {
			t.Fatalf("stdout 该含 %q：\n%s", want, out)
		}
	}
}

// 负控：无件桶名落在闭集外 ⇒ 逐条点名。
func TestGapAxisRecNegOutsideClosedSet(t *testing.T) {
	repo := gaaTestFakeRepo(t)
	led := gaaTestLedger(t, gaaGoodYou, `{"id":"GAP-20260928-9102","state":"仍缺","unit":"无件(乱写)","module":"无件"}`)
	rc, out, _ := gaaTestCall("--repo", repo, "--ledger", led)
	if rc != 1 || !strings.Contains(out, "闭集外形") || !strings.Contains(out, "无件(乱写)") {
		t.Fatalf("该 rc=1 且点名闭集外形态，实 rc=%d\n%s", rc, out)
	}
}

// 负控（戊）：`unit` 指向索引里没有的件 ⇒ 失效件逐条点名（GAP-id 作主指针）。
func TestGapAxisRecNegDeadFile(t *testing.T) {
	repo := gaaTestFakeRepo(t)
	led := gaaTestLedger(t, gaaGoodYou,
		`{"id":"GAP-20260928-9103","state":"仍缺","unit":"core/dead.go","module":"core","unit_source":"auto"}`)
	rc, out, _ := gaaTestCall("--repo", repo, "--ledger", led)
	if rc != 1 {
		t.Fatalf("该 rc=1，实 rc=%d\n%s", rc, out)
	}
	for _, want := range []string{"失效件 1 条", "GAP-20260928-9103", "失效件 `core/dead.go`"} {
		if !strings.Contains(out, want) {
			t.Fatalf("stdout 该含 %q：\n%s", want, out)
		}
	}
}

// 负控（己）：账里 `module` 与由 `unit` 推出来的值矛盾 ⇒ 逐条点名。
func TestGapAxisRecNegAxisMismatch(t *testing.T) {
	repo := gaaTestFakeRepo(t)
	led := gaaTestLedger(t, `{"id":"GAP-20260928-9104","state":"仍缺","unit":"core/a.go","module":"scripts"}`, gaaGoodWu)
	rc, out, _ := gaaTestCall("--repo", repo, "--ledger", led)
	if rc != 1 {
		t.Fatalf("该 rc=1，实 rc=%d\n%s", rc, out)
	}
	for _, want := range []string{"不一致 1 条", "GAP-20260928-9104", "账里 module='scripts' ≠ 口径推导 'core'"} {
		if !strings.Contains(out, want) {
			t.Fatalf("stdout 该含 %q：\n%s", want, out)
		}
	}
}

// 负控（己 · 真源口径字面边界）：闭集里的**其他**无件形态在真源 `gapModuleOfUnit` 里不落 `无件`
// 那一支 ⇒ 推成 `仓根`；命中行必须**带说明**（不静默）。
func TestGapAxisRecNegModRuleLiteralEdge(t *testing.T) {
	repo := gaaTestFakeRepo(t)
	led := gaaTestLedger(t, gaaGoodYou, `{"id":"GAP-20260928-9105","state":"仍缺","unit":"无件(门禁面)","module":"无件"}`)
	rc, out, _ := gaaTestCall("--repo", repo, "--ledger", led)
	if rc != 1 {
		t.Fatalf("该 rc=1，实 rc=%d\n%s", rc, out)
	}
	for _, want := range []string{"真源口径边界", "⇒ `仓根`", "闭集内其他无件形态不在真源这一支里"} {
		if !strings.Contains(out, want) {
			t.Fatalf("stdout 该含 %q：\n%s", want, out)
		}
	}
}

// ── 负控（取数面拿不到 · rc=2 · 不许当绿）────────────────────────────────────

func TestGapAxisRecRC2MissingLedger(t *testing.T) {
	repo := gaaTestFakeRepo(t)
	rc, out, _ := gaaTestCall("--repo", repo, "--ledger", filepath.Join(t.TempDir(), "nope.jsonl"))
	if rc != 2 || !strings.Contains(out, "⇒ rc=2 BLOCKED") || !strings.Contains(out, "账本不在盘") {
		t.Fatalf("该 rc=2 且含「账本不在盘」，实 rc=%d\n%s", rc, out)
	}
}

// 空账 = 空转 = 假绿 ⇒ rc=2（**不给结论**），不是 rc=0。
func TestGapAxisRecRC2EmptyLedger(t *testing.T) {
	repo := gaaTestFakeRepo(t)
	led := filepath.Join(t.TempDir(), "empty.jsonl")
	gaaTestWriteRaw(t, led, "")
	rc, out, _ := gaaTestCall("--repo", repo, "--ledger", led)
	if rc != 2 || !strings.Contains(out, "账本空转（0 行）") {
		t.Fatalf("该 rc=2 且含「账本空转（0 行）」，实 rc=%d\n%s", rc, out)
	}
}

// 有行解不开 ⇒ rc=2（取数面问题，不当绿也不当红）。
func TestGapAxisRecRC2BadJSON(t *testing.T) {
	repo := gaaTestFakeRepo(t)
	led := filepath.Join(t.TempDir(), "bad.jsonl")
	gaaTestWriteRaw(t, led, gaaGoodYou+"\n}{ 这不是 json\n")
	rc, out, _ := gaaTestCall("--repo", repo, "--ledger", led)
	if rc != 2 || !strings.Contains(out, "有 1 行解不开") || !strings.Contains(out, "行 2") {
		t.Fatalf("该 rc=2 且点名解不开的行，实 rc=%d\n%s", rc, out)
	}
}

// 仓根不是 git 仓 ⇒ 戊 取数面拿不到 ⇒ rc=2（不拿读不到当没问题）。
func TestGapAxisRecRC2NoSuchRepo(t *testing.T) {
	led := gaaTestLedger(t, gaaGoodYou, gaaGoodWu)
	rc, out, _ := gaaTestCall("--repo", t.TempDir(), "--ledger", led)
	if rc != 2 || !strings.Contains(out, "戊 取数面拿不到") {
		t.Fatalf("该 rc=2 且含「戊 取数面拿不到」，实 rc=%d\n%s", rc, out)
	}
}

// 真源件不在盘（合成仓里删掉 family_gap.go）⇒ 己 取数面拿不到 ⇒ rc=2。
func TestGapAxisRecRC2NoModRuleSource(t *testing.T) {
	repo := gaaTestFakeRepo(t)
	if err := os.Remove(filepath.Join(repo, "core", "cmd", "zerg", "family_gap.go")); err != nil {
		t.Fatal(err)
	}
	led := gaaTestLedger(t, gaaGoodYou, gaaGoodWu)
	rc, out, _ := gaaTestCall("--repo", repo, "--ledger", led)
	if rc != 2 || !strings.Contains(out, "己 取数面拿不到") {
		t.Fatalf("该 rc=2 且含「己 取数面拿不到」，实 rc=%d\n%s", rc, out)
	}
}

// ── 用法面 ───────────────────────────────────────────────────────────────────

func TestGapAxisRecListRulesAndHelp(t *testing.T) {
	rc, out, _ := gaaTestCall("--list-rules")
	if rc != 0 || !strings.Contains(out, "判据己  轴间一致性") || !strings.Contains(out, "无件闭集:") {
		t.Fatalf("--list-rules 该 rc=0 且打判据/闭集，实 rc=%d\n%s", rc, out)
	}
	rc, out, _ = gaaTestCall("-h")
	if rc != 0 || !strings.Contains(out, "usage: check-gap-axis-reconcile.py") {
		t.Fatalf("-h 该 rc=0 且打 usage，实 rc=%d\n%s", rc, out)
	}
	rc, _, errb := gaaTestCall("--nope")
	if rc != 2 || !strings.Contains(errb, "unrecognized arguments: --nope") {
		t.Fatalf("未知旗标该 rc=2 且落 error 行，实 rc=%d\n%s", rc, errb)
	}
}

// 正控 + 回归牙：--self-test 全过，且**自检夹具零残留**（钉住 defer os.RemoveAll 这一止血动作）。
func TestGapAxisRecSelfTestAndTempdirDelta(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir()) // GAP-20261003-16：私有计数面（只数本用例自建 tmp 目录）
	before := gaaCountSelfTestDirs()
	rc, out, _ := gaaTestCall("--self-test")
	after := gaaCountSelfTestDirs()
	if rc != 0 {
		t.Fatalf("--self-test 该 rc=0，实 rc=%d\n%s", rc, out)
	}
	if !strings.Contains(out, "⇒ 自检 PASS（17 条断言") {
		t.Fatalf("--self-test 该打「17 条断言 PASS」：\n%s", out)
	}
	if after != before {
		t.Fatalf("自检夹具泄漏：跑前 %d 个 · 跑后 %d 个（defer os.RemoveAll 失效？）", before, after)
	}
}

func gaaCountSelfTestDirs() int {
	ents, err := os.ReadDir(os.TempDir())
	if err != nil {
		return 0
	}
	n := 0
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), "gap-axis-selftest-") {
			n++
		}
	}
	return n
}

// ── 两态对拍（仓外夹具 · py 原件 ↔ 本件入口 · GAP-20261001-61 钉死 /usr/bin/python3）────────

var gaaTSRE = regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}[+-]\d{2}:\d{2}`)

func gaaTestEnvClean() []string {
	out := make([]string, 0, len(os.Environ()))
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "ZERG_STATE_DIR=") || strings.HasPrefix(kv, "ZERG_REPO=") {
			continue
		}
		out = append(out, kv)
	}
	return out
}

func gaaTestRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for i := 0; i < 10 && dir != "" && dir != "/"; i++ {
		if st, err := os.Stat(filepath.Join(dir, "scripts", "gates", "check-gap-axis-reconcile.py")); err == nil && !st.IsDir() {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	return ""
}

func gaaTestIsFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func TestGapAxisRecPyParity(t *testing.T) {
	root := gaaTestRepoRoot(t)
	pyPath := ""
	if root != "" {
		pyPath = filepath.Join(root, "scripts", "gates", "check-gap-axis-reconcile.py")
	}
	gaaPyParityOrRetired(t, root, pyPath)
}

// TestGapAxisRecRetiredNoPy —— 退役态（B轨 · 2026-10-07）：本件同族 py 原件**不在盘** ⇒ 判据照常出结论
// （✗ 不得 fail-closed / rc=2）。本件（Go 半）现读正文零 `ReadFile`/`exec` 该 py ⇒ 退役不改 Go 侧结论；
// 本用例把它钉成断言（✗ 不再静默 skip）。
// ★ 哑化负控：把 `gaaPyParityOrRetired` 退役分支改成 `t.Fatalf("py 原件不在盘")` ⇒ 本用例红（带行号）。
func TestGapAxisRecRetiredNoPy(t *testing.T) {
	gaaPyParityOrRetired(t, "", filepath.Join(t.TempDir(), "check-gap-axis-reconcile.py"))
}

// gaaPyParityOrRetired —— 同族 py 原件在盘 ⇒ 两态逐字对拍；**退役（不在盘）** ⇒ 断言 Go 半不依赖该 py、
// 照常出结论（rc=0，✗ 不得 rc=2）。退役态是件退役后的稳态，钉成断言而非 skip。
func gaaPyParityOrRetired(t *testing.T, root, pyPath string) {
	t.Helper()
	if root == "" || !gaaTestIsFile(pyPath) {
		// ── 退役态：Go 半不读同族 py ⇒ 仍出结论（与 py 在盘时同 rc）──
		repo := gaaTestFakeRepo(t)
		led := gaaTestLedger(t, gaaGoodYou, gaaGoodWu)
		rc, out, errb := gaaTestCall("--repo", repo, "--ledger", led)
		if rc != 0 {
			t.Fatalf("退役态 Go 半该 rc=0（✗ 不得 fail-closed / rc=2），实 rc=%d\nstdout:\n%s\nstderr:\n%s", rc, out, errb)
		}
		return
	}
	if st, err := os.Stat(gaaTestPython); err != nil || st.IsDir() {
		t.Fatalf("哨兵失守：未钉到解释器 %s（✗ 不留 Skip · fail-closed）", gaaTestPython)
	}
	repo := gaaTestFakeRepo(t)
	led := gaaTestLedger(t, gaaGoodYou, gaaGoodWu,
		`{"id":"GAP-20260928-9106","state":"仍缺","unit":"无件(命令面)","module":"无件"}`,
		`{"id":"GAP-20260928-9107","state":"已解","unit":"core/a.go","module":"core"}`)
	args := []string{"--repo", repo, "--ledger", led, "--json"}

	cmd := exec.Command(gaaTestPython, append([]string{pyPath}, args...)...)
	cmd.Dir = root
	cmd.Env = gaaTestEnvClean()
	var pyOut, pyErr strings.Builder
	cmd.Stdout, cmd.Stderr = &pyOut, &pyErr
	runErr := cmd.Run()
	pyRC := 0
	if runErr != nil {
		if ee, ok := runErr.(*exec.ExitError); ok {
			pyRC = ee.ExitCode()
		} else {
			t.Fatalf("py 起不来：%v", runErr)
		}
	}

	goRC, goOut, goErr := gaaTestCall(args...)

	if pyRC != goRC {
		t.Fatalf("退码不同：py=%d go=%d", pyRC, goRC)
	}
	if gaaTSRE.ReplaceAllString(pyOut.String(), "TIMESTAMP") != gaaTSRE.ReplaceAllString(goOut, "TIMESTAMP") {
		t.Fatalf("stdout 不同（掩码后）：\n--- py ---\n%s\n--- go ---\n%s", pyOut.String(), goOut)
	}
	if pyErr.String() != goErr {
		t.Fatalf("stderr 不同：\n--- py ---\n%s\n--- go ---\n%s", pyErr.String(), goErr)
	}
}
