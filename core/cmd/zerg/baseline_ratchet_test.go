// baseline_ratchet_test.go —— 成对负控（✗ 禁恒绿）
//
// 被测件：baseline_ratchet.go（M2 真搬 scripts/gates/baseline-ratchet.py）
// 面：① 纯函数级夹具行为（判红/绿/写面/退码档）② 与 py 原件的逐字节对拍（解释器一律钉
//
//	/usr/bin/python3，GAP-20261001-61）③ 自检档。
//
// ★ 写面（--write/--anchor）一律只在 t.TempDir() 夹具上走，✗ 不写真实基线件与真实台账。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// pyInterp 钉死解释器（GAP-20261001-61）：✗ 不用 PATH 上的 python3。
const pyInterp = "/usr/bin/python3"

func brTestRepoRoot(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("仓根解析失败：%v", err)
	}
	return abs
}

// brTestFixture 造仓外夹具（两件仓根标记 + 四个基线件）。全部落在 t.TempDir()。
func brTestFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, d := range []string{"core", "scripts/gates", "ui/scripts"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	must := func(rel, body string) {
		if err := os.WriteFile(filepath.Join(root, rel), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	must("core/go.mod", "")
	must("scripts/gates/precommit-gates.sh", "")
	must("scripts/gates/gate-coverage.config.json", "{\n  \"id\": \"gc.v1\",\n  \"baseline\": 5,\n  \"note\": \"夹具\"\n}\n")
	must("scripts/gates/cli-contract-baseline.json", "{\n  \"blanks\": 0\n}\n")
	must("scripts/gates/doc-cmds-baseline.json", "{\n  \"devdocs_report_hits\": 183\n}\n")
	must("ui/scripts/i18n-baseline.json", "{\n  \"allowed_count\": 7\n}\n")
	return root
}

func brTestRun(root string, args ...string) (int, string, string) {
	var so, se bytes.Buffer
	full := append([]string{"--root", root}, args...)
	rc := baselineRatchetGateMain(&so, &se, full)
	return rc, so.String(), se.String()
}

func brTestLedgerPath(root string) string {
	return filepath.Join(root, "scripts", "gates", "baseline-audit.jsonl")
}

// ① 纯用法档：退码 + stdout/stderr 分面（✗ 禁恒绿：每格钉死 rc）
func TestBaselineRatchet_UsageSurface(t *testing.T) {
	root := brTestFixture(t)

	rc, so, se := brTestRun(root, "--help")
	if rc != 0 || !strings.Contains(so, "usage: baseline-ratchet.py") || se != "" {
		t.Fatalf("help 档：rc=%d stdout含usage=%v stderr=%q", rc, strings.Contains(so, "usage:"), se)
	}

	rc, so, se = brTestRun(root, "--bogus")
	if rc != 2 || so != "" || !strings.Contains(se, "unrecognized arguments: --bogus") {
		t.Fatalf("未知旗标：rc=%d stdout=%q stderr=%q", rc, so, se)
	}
	if !strings.Contains(se, "baseline-ratchet.py: error:") {
		t.Fatalf("未知旗标：stderr 缺 argparse 报话头：%q", se)
	}

	rc, _, _ = brTestRun(root, "--view", "--diff")
	if rc != 2 {
		t.Fatalf("档位互斥应 rc=2，得 %d", rc)
	}

	rc, so, _ = brTestRun(root, "--only", "nope")
	if rc != 2 || !strings.Contains(so, "条件名不认：nope") {
		t.Fatalf("不认的条件名应 rc=2，得 rc=%d stdout=%q", rc, so)
	}

	rc, so, _ = brTestRun(root, "--conditions")
	if rc != 0 || !strings.Contains(so, "baseline-conditions.v1") || !strings.Contains(so, "measure_re") {
		t.Fatalf("条件表档：rc=%d stdout=%q", rc, so)
	}

	// --v 是 --view 的唯一前缀 ⇒ argparse 应消歧为 --view（不是用法错 rc=2）
	rc, _, se = brTestRun(root, "--v")
	if rc == 2 || strings.Contains(se, "error:") {
		t.Fatalf("--v 应消歧为 --view（非用法错），得 rc=%d stderr=%q", rc, se)
	}
}

// ② 台账两态 + 手改 + 件不在（判红/判不了 分面）
func TestBaselineRatchet_Verdicts(t *testing.T) {
	// 甲态：台账件不在盘上 ⇒ rc=1 且 reason=ledger_absent
	root := brTestFixture(t)
	rc, _, _ := brTestRun(root, "--check")
	if rc != 1 {
		t.Fatalf("无台账应 rc=1，得 %d", rc)
	}
	_, so, _ := brTestRun(root, "--check", "--json")
	if !strings.Contains(so, "\"reason\": \"ledger_absent\"") {
		t.Fatalf("甲态 reason 应为 ledger_absent：%q", so)
	}

	// 乙态：台账件在盘上但 0 条记录 ⇒ rc=1 且 reason=no_record
	if err := os.WriteFile(brTestLedgerPath(root), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	rc, _, _ = brTestRun(root, "--check")
	if rc != 1 {
		t.Fatalf("空台账应 rc=1，得 %d", rc)
	}
	_, so, _ = brTestRun(root, "--check", "--json")
	if !strings.Contains(so, "\"reason\": \"no_record\"") {
		t.Fatalf("乙态 reason 应为 no_record：%q", so)
	}

	// 锚之后 ⇒ 绿
	rc, _, _ = brTestRun(root, "--anchor", "--reason", "夹具锚", "--by", "tester")
	if rc != 0 {
		t.Fatalf("--anchor 应 rc=0，得 %d", rc)
	}
	rc, _, _ = brTestRun(root, "--check")
	if rc != 0 {
		t.Fatalf("锚后应绿 rc=0，得 %d", rc)
	}

	// ⑦ 手改一个字节、没记录 ⇒ 判红（本件最要紧那一格）
	p := filepath.Join(root, "scripts/gates/gate-coverage.config.json")
	raw, _ := os.ReadFile(p)
	_ = os.WriteFile(p, bytes.Replace(raw, []byte("\"baseline\": 5"), []byte("\"baseline\": 9"), 1), 0o644)
	rc, so, _ = brTestRun(root, "--check")
	if rc != 1 || !strings.Contains(so, "没记录的手改") {
		t.Fatalf("手改无记录应 rc=1 且点明「没记录的手改」：rc=%d stdout=%q", rc, so)
	}

	// ⑧ 件被删 ⇒ 判不了（判不了 ≠ 绿：n/a 档不进红，rc 回落 0 —— 真口子，如实钉现状）
	_ = os.Remove(p)
	rc, so, _ = brTestRun(root, "--check")
	if !strings.Contains(so, "判不了") {
		t.Fatalf("件不在应点明「判不了」：rc=%d stdout=%q", rc, so)
	}
	if rc != 0 {
		t.Fatalf("件不在档 rc 现状为 0（B 类真口子：判不了 ≠ 绿 但 rc 不翻红），得 %d", rc)
	}
}

// ③ 写面（唯一通道）· 全部只在夹具上走
func TestBaselineRatchet_WriteFace(t *testing.T) {
	root := brTestFixture(t)
	cond := filepath.Join(root, "scripts/gates/gate-coverage.config.json")
	shBefore := brTestSha(t, cond)

	// 缺 --reason ⇒ rc=2 且件一字不动
	rc, _, _ := brTestRun(root, "--write", "--only", "gate-coverage.baseline", "--by", "tester")
	if rc != 2 {
		t.Fatalf("--write 缺理由应 rc=2，得 %d", rc)
	}
	if brTestSha(t, cond) != shBefore {
		t.Fatal("缺理由时件 sha256 变了（应一字不动）")
	}

	rd := filepath.Join(t.TempDir(), "r.tsv")
	_ = os.WriteFile(rd, []byte("gate-coverage.baseline = 99\n"), 0o644)

	// 上调不带 --allow-raise ⇒ rc=2 且不写
	rc, _, _ = brTestRun(root, "--write", "--reason", "夹具", "--only", "gate-coverage.baseline", "--readings", rd, "--by", "tester")
	if rc != 2 {
		t.Fatalf("上调不带 --allow-raise 应 rc=2，得 %d", rc)
	}
	if brTestSha(t, cond) != shBefore {
		t.Fatal("上调被拦时件 sha256 变了")
	}

	// 下调 ⇒ rc=0：只改那一个数字 + 台账真追一条
	rd2 := filepath.Join(t.TempDir(), "d.tsv")
	_ = os.WriteFile(rd2, []byte("gate-coverage.baseline = 2\n"), 0o644)
	rc, so, _ := brTestRun(root, "--write", "--reason", "夹具下调", "--only", "gate-coverage.baseline", "--readings", rd2, "--by", "tester")
	if rc != 0 {
		t.Fatalf("下调应 rc=0，得 %d；stdout=%q", rc, so)
	}
	got, _ := os.ReadFile(cond)
	want := "{\n  \"id\": \"gc.v1\",\n  \"baseline\": 2,\n  \"note\": \"夹具\"\n}\n"
	if string(got) != want {
		t.Fatalf("只应改那一个数字：\n got=%q\nwant=%q", string(got), want)
	}
	lb, _ := os.ReadFile(brTestLedgerPath(root))
	lines := strings.Split(strings.TrimSpace(string(lb)), "\n")
	if len(lines) != 1 {
		t.Fatalf("台账应恰追 1 条，得 %d 条", len(lines))
	}
	for _, frag := range []string{"\"kind\": \"write\"", "\"old\": 5", "\"new\": 2", "\"reason\": \"夹具下调\"", "\"by\": \"tester\"", "\"sha256_before\"", "\"sha256_after\""} {
		if !strings.Contains(lines[0], frag) {
			t.Fatalf("台账行缺 %s：%s", frag, lines[0])
		}
	}
	// 写后 ⇒ 绿（只对刚写的那一条；其余三条夹具里本就无记录）
	rc, _, _ = brTestRun(root, "--check", "--only", "gate-coverage.baseline")
	if rc != 0 {
		t.Fatalf("写后应绿 rc=0，得 %d", rc)
	}

	// 台账断号 ⇒ 硬错 ⇒ --write 一个字节不写（rc=2）
	_ = os.WriteFile(brTestLedgerPath(root), []byte("{\"seq\": 9, \"name\": \"x\"}\n"), 0o644)
	rc, _, _ = brTestRun(root, "--write", "--reason", "夹具", "--only", "gate-coverage.baseline", "--readings", rd2, "--allow-raise", "--by", "tester")
	if rc != 2 {
		t.Fatalf("台账断号应 rc=2，得 %d", rc)
	}
}

// ④ 自检档：合成件成对负控（不碰真目标）
func TestBaselineRatchet_SelfTest(t *testing.T) {
	var so, se bytes.Buffer
	rc := baselineRatchetGateMain(&so, &se, []string{"--self-test"})
	if rc != 0 {
		t.Fatalf("--self-test 应 rc=0，得 %d\n%s", rc, so.String())
	}
	if !strings.Contains(so.String(), "自检结论: 全过") {
		t.Fatalf("自检未全过：%s", so.String())
	}
	if strings.Contains(so.String(), "✗") {
		t.Fatalf("自检输出含失败行：%s", so.String())
	}
}

// ⑤ 内化金标（py 原件退役 ⇒ 对拍台无对象 · 照 check_doc_name / check_flag_registry_parity 先例）：
// 旧「py 原件 ⟷ Go 真搬」逐字对拍（其 Skip 落在本件旧第 243 行 · py `scripts/gates/baseline-ratchet.py`
// 已退场 ⇒ 恒真 Skip · 假绿零保护）⇒ 把金标真源**收回 Go 半现役行为本身**：
//
//	① 哨兵改锚 **Go 半** core/cmd/zerg/baseline_ratchet.go（✗ 不锚已退场 py —— 一退役即失锚）；
//	   找不到 ⇒ fail-closed（✗ 不留 Skip）。
//	② 金标 brRetiredGolden：逐格 (rc · 关键串) 由 Go 半现役行为**现算现冻**（取时夹具 = brTestFixture）。
//	③ 两态（✗ 不许 Skip）：A 态 = 真仓现态（无 py）；B 态 = 合成「旧态」（把退役 py 路径放回 stub）
//	   ⇒ 两态金标逐格同命中（证 py 在/不在盘不改判据结论 · 内化彻底）。
//	★ 哑化负控（"牙"）：`-overlay` 把 baseline_ratchet.go 的退码判据一臂哑化 ⇒ 本用例必 FAIL 带行号。
func TestBaselineRatchet_RetiredGolden(t *testing.T) {
	root := brTestRepoRoot(t)
	if fi, err := os.Stat(filepath.Join(root, "core", "cmd", "zerg", "baseline_ratchet.go")); err != nil || fi.IsDir() {
		t.Fatalf("哨兵失守：上溯未见 Go 半 core/cmd/zerg/baseline_ratchet.go ⇒ fail-closed（✗ 不留 Skip）：%v", err)
	}
	brRunRetiredGolden(t, "A真仓无py", nil)
	brRunRetiredGolden(t, "B旧态有py", func(fx string) {
		pd := filepath.Join(fx, "scripts", "gates")
		if err := os.MkdirAll(pd, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(pd, "baseline-ratchet.py"),
			[]byte("# stub：「旧态」——退役 py 在位（本件只证 py 在/不在盘不改金标结论）\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	})
}

// brRetiredGolden —— 退役态金标（现算自证 · 取时夹具 = brTestFixture · 见件头金标计算式 · ✗ 手写）。
var brRetiredGolden = []struct {
	name string
	argv []string
	rc   int
	sub  string
}{
	{"help", []string{"--help"}, 0, "usage: baseline-ratchet.py"},
	{"conditions", []string{"--conditions"}, 0, "baseline-conditions.v1"},
	{"check_json", []string{"--check", "--json"}, 1, `"reason": "ledger_absent"`},
	{"check_human", []string{"--check"}, 1, "判红（rc=1）"},
	{"view", []string{"--view"}, 1, "现值 5"},
	{"bogus", []string{"--bogus"}, 2, "unrecognized arguments: --bogus"},
	{"mutex", []string{"--view", "--diff"}, 2, "档位互斥"},
	{"only_unknown", []string{"--only", "nope"}, 2, "条件名不认：nope"},
	{"diff_readings", []string{"--diff", "--readings", "<RD>"}, 0, "将升 1 条"},
	{"anchor", []string{"--anchor", "--reason", "夹具锚", "--by", "tester"}, 0, "已补 4 条现状锚"},
}

// brRunRetiredGolden —— 现建夹具、现跑 Go 半、逐格钉 (rc · 关键串)。plant 非 nil ⇒ 先落「旧态」py（两态用）。
func brRunRetiredGolden(t *testing.T, tag string, plant func(fx string)) {
	t.Helper()
	fx := brTestFixture(t)
	rd := filepath.Join(fx, "r.tsv")
	if err := os.WriteFile(rd, []byte("gate-coverage.baseline = 68\ncli-contract.blanks = 0\ndoc-cmds.devdocs_report_hits = 183\ni18n.allowed_count = 7\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if plant != nil {
		plant(fx)
	}
	mism := 0
	for _, c := range brRetiredGolden {
		argv := make([]string, 0, len(c.argv))
		for _, a := range c.argv {
			if a == "<RD>" {
				a = rd
			}
			argv = append(argv, a)
		}
		var so, se bytes.Buffer
		full := append([]string{"--root", fx}, argv...)
		rc := baselineRatchetGateMain(&so, &se, full)
		if rc != c.rc {
			mism++
			t.Errorf("[%s/%s] rc=%d（冻值 %d）\nstdout=%q\nstderr=%q", tag, c.name, rc, c.rc, so.String(), se.String())
		}
		if !strings.Contains(so.String()+se.String(), c.sub) {
			mism++
			t.Errorf("[%s/%s] 输出缺关键串 %q：stdout=%q stderr=%q", tag, c.name, c.sub, so.String(), se.String())
		}
	}
	if mism != 0 {
		t.Fatalf("[%s] 内化金标失配 %d 处（py 在/不在盘都不许改结论）", tag, mism)
	}
}

func brTestSha(t *testing.T, path string) string {
	t.Helper()
	sum, err := brSha256File(path)
	if err != nil {
		t.Fatalf("sha 失败 %s：%v", path, err)
	}
	return sum
}

// TestBrMeasureValueRetiredGoFallback —— G19（2026-10-07 退役批）：现测命令的 py 件退役
// （不在盘）而条件带 Go 半 ⇒ brMeasureValue 回落**进程内跑同判据 Go 半**取同一读数
// （✗ 不因缺 py 而「判不了 ⇒ 不给结论」fail-closed）。本用例以「指向一件不在盘的 py」逼出该支。
// ★ 成对负控：Go 半也移开（goMain=nil）⇒ 仍真起 python3（件不在）⇒ 判不了（fail-closed 保留）。
// ★ 哑化负控：抽掉 brMeasureValue 里 `if cond.goMain != nil …` 那段 ⇒ 本用例 FAIL（带行号）。
func TestBrMeasureValueRetiredGoFallback(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Clean(filepath.Join(wd, "..", "..", ".."))
	if _, err := os.Stat(filepath.Join(root, "core", "cmd", "zerg", "main.go")); err != nil {
		t.Fatalf("哨兵失守：仓根未定位到 core/cmd/zerg/main.go（✗ 不留 Skip · fail-closed）：%v", err)
	}
	cond := brCond{
		name:      "doc-cmds.devdocs_report_hits",
		measure:   []string{"python3", brPyJoin("scripts", "gates", "__retired_probe__.py"), "--scope", "devdocs"},
		measureRe: `命中\s*(\d+)\s*处`,
		goMain:    checkDocCmdsGateMain,
	}
	v, src, merr := brMeasureValue(root, cond, nil, false)
	if merr != nil {
		t.Fatalf("退役态该回落 Go 半（✗ 不 fail-closed）：%v", *merr)
	}
	if v == nil || src == nil || !strings.Contains(*src, "退役态回落") {
		t.Fatalf("未走退役态回落：v=%v src=%v", v, src)
	}
	// 成对负控：Go 半也移开 ⇒ 真起 python3（件不在）⇒ 判不了。
	cond.goMain = nil
	if _, _, e2 := brMeasureValue(root, cond, nil, false); e2 == nil {
		t.Fatalf("负控：无 Go 半该判不了（fail-closed 保留）")
	}
}

// TestBrConditionsRetiredGoFallbackTabled —— ★ 退役态回落面的登记牙：**退场批**里的现测门格必须带
// goMain（其 Go 半）⇒ 退役态照常出结论（✗ 不 fail-closed）。逐格点名，缺哪格报哪格。
// ★ 哑化负控（带行号）：
//
//	· 抽掉 `brConditions` 里 `gate-coverage.baseline` 格的 `goMain: checkGateCoverageGateMain,` 行
//	  （紧邻该格 `measureRe: ③…` 那行之下）⇒ 本用例 FAIL，逐字「`gate-coverage.baseline` 格缺 goMain」。
//	· 抽掉 `cli-contract.blanks` 格的 `goMain: cliContractGateMain,` 行 ⇒ 同法 FAIL（原覆盖保留）。
func TestBrConditionsRetiredGoFallbackTabled(t *testing.T) {
	// 表：格名 ⇒ 待退 py（退场批 · 引用它当现测件的格必须带 Go 半）。
	want := []struct{ name, py string }{
		{"cli-contract.blanks", "check-cli-contract.py"},
		{"gate-coverage.baseline", "check-gate-coverage.py"},
	}
	byName := map[string]brCond{}
	for _, c := range brConditions {
		byName[c.name] = c
	}
	for _, w := range want {
		c, ok := byName[w.name]
		if !ok {
			t.Fatalf("`%s` 格不见了（表被改名/删）", w.name)
		}
		if c.goMain == nil {
			t.Fatalf("`%s` 格缺 goMain ⇒ %s 退场后该格 fail-closed", w.name, w.py)
		}
		if len(c.measure) < 2 || c.measure[0] != "python3" || !strings.HasSuffix(c.measure[1], w.py) {
			t.Fatalf("`%s` 格现测命令形态变了：%v（回落前置是 [python3, <py>]）", w.name, c.measure)
		}
	}
}

// TestBrGateCoverageRetiredGoFallback —— 判据「gate-coverage.baseline」的退役态回落牙（G21）：
// 现测件 `scripts/gates/check-gate-coverage.py` 退役（不在盘）而本格带 Go 半 ⇒ `brMeasureValue`
// 回落**进程内跑 `checkGateCoverageGateMain --baseline`** 取同一读数（同一 `measureRe` · ✗ 不 fail-closed）。
// 本用例以「指向一件不在盘的 py」逼出该支（**真仓 py 不动** · 与 `TestBrMeasureValueRetiredGoFallback` 同法）。
// ★ 成对负控：Go 半也移开（goMain=nil）⇒ 真起 python3（件不在）⇒ 判不了（fail-closed 保留）。
// ★ 哑化负控（带行号）：抽掉 `brConditions` 里 `gate-coverage.baseline` 格的 `goMain:` 行 ⇒ 本用例 FAIL。
func TestBrGateCoverageRetiredGoFallback(t *testing.T) {
	root := brTestRepoRoot(t)
	if _, err := os.Stat(filepath.Join(root, "core", "cmd", "zerg", "main.go")); err != nil {
		t.Fatalf("哨兵失守：仓根未定位到 core/cmd/zerg/main.go（✗ 不留 Skip · fail-closed）：%v", err)
	}
	cond := brCond{
		name:      "gate-coverage.baseline",
		measure:   []string{"python3", brPyJoin("scripts", "gates", "__retired_check_gate_coverage__.py"), "--baseline"},
		measureRe: `③\s*实测命中数[^=]*=\s*(\d+)`,
		goMain:    checkGateCoverageGateMain,
	}
	v, src, merr := brMeasureValue(root, cond, nil, false)
	if merr != nil {
		t.Fatalf("退役态该回落 Go 半（✗ 不 fail-closed）：%v", *merr)
	}
	if v == nil || src == nil || !strings.Contains(*src, "退役态回落") {
		t.Fatalf("未走退役态回落：v=%v src=%v", v, src)
	}
	// 成对负控：Go 半也移开 ⇒ 真起 python3（件不在）⇒ 判不了（fail-closed 保留）。
	cond.goMain = nil
	if _, _, e2 := brMeasureValue(root, cond, nil, false); e2 == nil {
		t.Fatal("负控：无 Go 半该判不了（fail-closed 保留）")
	}
}
