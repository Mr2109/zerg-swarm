// two_states_gate_mutate_test.go —— 主件 `core/cmd/zerg/two_states_gate_mutate.go` 的**成对负控**测试件。
//
// 覆盖（每条都**真跑**、✗ 无恒绿）：
//
//	· `tsmApplyMutation` 锚点恰一次判据：命中 0 / 1 / 2 次三态 ⇒ **不写盘**（纯函数面）。
//	· `tsmPyJoin` 三语义差（后段绝对 / 空段 / `..` 原样留串）—— 与 CPython posixpath.join 逐格对照。
//	· 夹具门禁生成面：三条锚点各恰命中一次 · 探测串**倒序存**（未变异时不得自检出）。
//	· 变异环端到端（仓外夹具 + **注入的 Go 本机镜子**，不依赖 python）：红 ⇒ 还原 sha 一致 ⇒ 复跑绿，rc=0。
//	· ★ 成对负控：镜子对变异无感（恒绿）⇒ rc=1 且逐条点名「镜子抓不住」（不许报绿）。
//	· ★ 还原校验的牙：还原不落地 ⇒ rc=2 且点名「还原后 sha 不一致」（删 `tsmSkipRestor` guard 即红）。
//	· ★ 缺省档**只读**：不传 `--overlay` 时目标件 sha 逐字节不变、报「未搬」、退 2（判不了 ≠ 绿）。
//	· ★ 仓内件拒写：overlay 自身含 `.git` ⇒ rc=2 拒在仓内改件。
//	· 真子进程面（钉 `/usr/bin/python3`）：`tsmCompiles` 对好件判过 / 对坏件判不过；茧壁缺位 ⇒ tsmRunGate 退 2。
//	· ★ 重入形态的牙：夹具驱动必须以**真子命令形态**起（`gate check <键>` 前置）—— 去掉前缀即红
//	  （复制出的 `zerg` 二进制按 `args[0]` 派发、直跑 `--overlay …` 落「未知旗标」rc=2）。
//	· ★ ⑲ 新搬格的牙：`--only <拼错 id>` ⇒ rc=2 且逐字点名「未知的变异 id」· 不报绿（拼错 ≠ 全绿）。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tsmTestRunEntry —— 进程内跑入口（与 py 的 `_run_fixture_inproc` 同法）。
func tsmTestRunEntry(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := twoStatesMutateGateMain(&out, &errb, args)
	return rc, out.String(), errb.String()
}

// tsmTestMirrorGate —— 注入用的**Go 本机镜子**（不依赖 python）：目标件里出现任一探测串 ⇒ 判红 rc=1。
func tsmTestMirrorGate(root string) (int, string) {
	b, err := os.ReadFile(tsmPyJoin(root, tsmTargetRel))
	if err != nil {
		return tsmExitHard, err.Error()
	}
	src := string(b)
	for _, tok := range tsmFixtureTokens() {
		if tok != "" && strings.Contains(src, tok) {
			return 1, "✗ 夹具门禁判红：检出变异串「" + tok + "」⇒ 这条路线没有真的经过茧壁 ⇒ 硬失败\n"
		}
	}
	return 0, "夹具门禁：绿（未检出变异）\n"
}

// tsmTestInjectMirror —— 把镜子与「恒判编译过」装上 · 跑完复原（成对：测试件自身也走 finally 语义）。
func tsmTestInjectMirror(t *testing.T) {
	t.Helper()
	savedG, savedC := tsmRunGateFn, tsmCompilesFn
	savedSkip := tsmSkipRestor
	tsmRunGateFn = tsmTestMirrorGate
	tsmCompilesFn = func(string) (bool, string) { return true, "" }
	t.Cleanup(func() {
		tsmRunGateFn, tsmCompilesFn, tsmSkipRestor = savedG, savedC, savedSkip
	})
}

// tsmTestFixture —— 仓外最小仓树（硬拷贝面不在此：镜像直接读树内目标件）。
func tsmTestFixture(t *testing.T, drop, dup string) string {
	t.Helper()
	fix := filepath.Join(t.TempDir(), "fx")
	if err := os.MkdirAll(filepath.Join(fix, "scripts", "sandbox-probes"), 0o755); err != nil {
		t.Fatalf("搭夹具失败：%v", err)
	}
	if err := os.MkdirAll(filepath.Join(fix, "scripts", "gates"), 0o755); err != nil {
		t.Fatalf("搭夹具失败：%v", err)
	}
	wallDir := filepath.Join(fix, "wall", "target", "debug")
	if err := os.MkdirAll(wallDir, 0o755); err != nil {
		t.Fatalf("搭夹具失败：%v", err)
	}
	if e := tsmWriteFixtureGate(fix, nil, drop, dup); e != "" {
		t.Fatalf("写夹具门禁失败：%s", e)
	}
	if err := os.WriteFile(filepath.Join(wallDir, "zerg-wall"), []byte(tsmFixWallStub), 0o755); err != nil {
		t.Fatalf("写哑茧壁件失败：%v", err)
	}
	return fix
}

func TestTsmApplyMutationExactlyOnce(t *testing.T) {
	text := "a\n" + tsmMutations[0].anchor + "b\n"
	if msg, newText, ok := tsmApplyMutation(text, tsmMutations[0].anchor, "X\n"); !ok || msg != "" || newText != "a\nX\nb\n" {
		t.Fatalf("恰一次应成功：msg=%q ok=%v text=%q", msg, ok, newText)
	}
	if msg, newText, ok := tsmApplyMutation(text, "不存在的锚点", "X"); ok || newText != "" || !strings.Contains(msg, "锚点命中 0 次") {
		t.Fatalf("命中 0 次应判不了且不产正文：msg=%q ok=%v", msg, ok)
	}
	twice := text + tsmMutations[0].anchor
	if msg, newText, ok := tsmApplyMutation(twice, tsmMutations[0].anchor, "X"); ok || newText != "" || !strings.Contains(msg, "锚点命中 2 次") {
		t.Fatalf("命中 2 次应判不了且不产正文：msg=%q ok=%v", msg, ok)
	}
}

func TestTsmPyJoinThreeSemantics(t *testing.T) {
	cases := [][4]string{
		{"a", "../b", "", "a/../b/"},
		{"repo", "/abs", "", "/abs/"},
		{"a", "", "", "a/"},
		{"a", "", "b", "a/b"},
		{"", "a", "", "a/"},
	}
	for _, c := range cases {
		if got := tsmPyJoin(c[0], c[1], c[2]); got != c[3] {
			t.Fatalf("tsmPyJoin(%q,%q,%q) = %q（应 %q）", c[0], c[1], c[2], got, c[3])
		}
	}
}

func TestTsmFixtureGateAnchorsAndTokens(t *testing.T) {
	src := tsmFixtureGateSrc(nil, "", "")
	for _, m := range tsmMutations {
		if n := strings.Count(src, m.anchor); n != 1 {
			t.Fatalf("夹具门禁里 %s 锚点应恰命中 1 次，实 %d 次", m.id, n)
		}
	}
	for _, tok := range tsmFixtureTokens() {
		if tok == "" {
			t.Fatalf("探测串不得为空（自维护面）")
		}
		if strings.Contains(src, tok) {
			t.Fatalf("探测串 %q 以**正序**落在未变异的夹具门禁里 ⇒ 自指陷阱（应倒序存）", tok)
		}
		if !strings.Contains(src, tsmReverseStr(tok)) {
			t.Fatalf("探测串 %q 的倒序形态不在夹具门禁里", tok)
		}
	}
}

func TestTsmDriveRedRestoreRerunGreen(t *testing.T) {
	tsmTestInjectMirror(t)
	fix := tsmTestFixture(t, "", "")
	target := tsmFixtureTarget(fix)
	pre, _ := tsmShaStr(target)

	rc, out, errb := tsmTestRunEntry(t, "--overlay", fix)
	if rc != tsmExitOK {
		t.Fatalf("仓外夹具变异环应 rc=0，实 rc=%d\nstdout=%s\nstderr=%s", rc, out, errb)
	}
	if n := strings.Count(out, "（期望非 0）"); n != len(tsmMutations) {
		t.Fatalf("变异档应 %d 条，实 %d\n%s", len(tsmMutations), n, out)
	}
	if n := strings.Count(out, "红的原文"); n < len(tsmMutations) {
		t.Fatalf("红原文档应 ≥%d 条，实 %d\n%s", len(tsmMutations), n, out)
	}
	if !strings.Contains(out, "变异验证全绿") {
		t.Fatalf("末行应报全绿\n%s", out)
	}
	post, _ := tsmShaStr(target)
	if post != pre {
		t.Fatalf("变异环必须还原：post=%s pre=%s", post, pre)
	}
}

// ★ 成对负控：镜子对变异无感 ⇒ 必须判红（否则「变异后必须红」这条判据是死的）。
func TestTsmMirrorBlindIsJudgedRed(t *testing.T) {
	tsmTestInjectMirror(t)
	tsmRunGateFn = func(string) (int, string) { return 0, "夹具：镜子恒绿" }
	fix := tsmTestFixture(t, "", "")

	rc, out, _ := tsmTestRunEntry(t, "--overlay", fix)
	if rc != tsmExitUnexpected {
		t.Fatalf("镜子恒绿 ⇒ 应 rc=1，实 rc=%d\n%s", rc, out)
	}
	if n := strings.Count(out, "镜子抓不住"); n != len(tsmMutations) {
		t.Fatalf("应逐条点名「镜子抓不住」%d 条，实 %d\n%s", len(tsmMutations), n, out)
	}
	if strings.Contains(out, "变异验证全绿") {
		t.Fatalf("抓不住时不得报绿\n%s", out)
	}
}

// ★ 还原校验的牙：还原不落地 ⇒ rc=2 且点名「还原后 sha 不一致」。
func TestTsmRestoreSkippedIsCaught(t *testing.T) {
	tsmTestInjectMirror(t)
	tsmSkipRestor = true
	tsmRunGateFn = func(string) (int, string) { return 0, "夹具：镜子恒绿（此格只考还原校验）" }
	fix := tsmTestFixture(t, "", "")

	rc, out, _ := tsmTestRunEntry(t, "--overlay", fix)
	if rc != tsmExitHard {
		t.Fatalf("还原不落地 ⇒ 应 rc=2，实 rc=%d\n%s", rc, out)
	}
	if !strings.Contains(out, "还原后 sha 不一致") {
		t.Fatalf("应点名「还原后 sha 不一致」\n%s", out)
	}
}

// ★ 缺省档只读：不传 --overlay ⇒ 绝不写仓内件、报「未搬」、退 2。
func TestTsmDefaultIsReadOnly(t *testing.T) {
	tsmTestInjectMirror(t)
	fix := tsmTestFixture(t, "", "")
	t.Chdir(fix)
	target := tsmFixtureTarget(fix)
	pre, _ := tsmShaStr(target)

	rc, out, _ := tsmTestRunEntry(t)
	if rc != tsmExitHard {
		t.Fatalf("缺省只读档应 rc=2（判不了 ≠ 绿），实 rc=%d\n%s", rc, out)
	}
	if !strings.Contains(out, "未搬") || !strings.Contains(out, "只读结构断言：全过") {
		t.Fatalf("缺省档应报只读结构 + 「未搬」\n%s", out)
	}
	post, _ := tsmShaStr(target)
	if post != pre {
		t.Fatalf("缺省档绝不改仓内件：post=%s pre=%s", post, pre)
	}
}

// ★ 仓内件拒写：overlay 自身含 `.git` ⇒ 拒在仓内改件。
func TestTsmRefusesRepoWithGit(t *testing.T) {
	tsmTestInjectMirror(t)
	fix := tsmTestFixture(t, "", "")
	if err := os.MkdirAll(filepath.Join(fix, ".git"), 0o755); err != nil {
		t.Fatalf("造 .git 失败：%v", err)
	}
	target := tsmFixtureTarget(fix)
	pre, _ := tsmShaStr(target)

	rc, out, _ := tsmTestRunEntry(t, "--overlay", fix)
	if rc != tsmExitHard {
		t.Fatalf("含 .git 的 overlay 应 rc=2，实 rc=%d\n%s", rc, out)
	}
	if !strings.Contains(out, "拒在仓内改件") {
		t.Fatalf("应逐字点名「拒在仓内改件」\n%s", out)
	}
	post, _ := tsmShaStr(target)
	if post != pre {
		t.Fatalf("拒写档不得动件：post=%s pre=%s", post, pre)
	}
}

func TestTsmPairGuardHalfIsNotGreen(t *testing.T) {
	tsmTestInjectMirror(t)
	fix := tsmTestFixture(t, "", "")
	t.Chdir(fix)

	rc, out, _ := tsmTestRunEntry(t, "--pair-guard", "--only", "G1")
	if rc != tsmExitHard || !strings.Contains(out, "缺半即判不了") {
		t.Fatalf("--pair-guard --only G1 应 rc=2 且点名「缺半即判不了」，实 rc=%d\n%s", rc, out)
	}
	rc2, _, err2 := tsmTestRunEntry(t, "--pair-guard", "--only", "G9")
	// ★ 与 py 同序：`--only` 在册校验排在 pair-guard **之前**（py :171-174 < :191）⇒ 拼错 id 走「未知的变异 id」。
	if rc2 != tsmExitHard || !strings.Contains(err2, "未知的变异 id") {
		t.Fatalf("--pair-guard --only G9（不在册）应 rc=2 且点名「未知的变异 id」于 stderr，实 rc=%d stderr=%q", rc2, err2)
	}
}

func TestTsmRealCompilesAndWallShortCircuit(t *testing.T) {
	d := t.TempDir()
	good := filepath.Join(d, "good.py")
	bad := filepath.Join(d, "bad.py")
	if err := os.WriteFile(good, []byte("x = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bad, []byte("def broken(:\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if ok, _ := tsmCompiles(good); !ok {
		t.Fatalf("好件应判编译过（解释器 %s）", tsmPython)
	}
	if ok, _ := tsmCompiles(bad); ok {
		t.Fatalf("坏件应判编译不过（成对向：不恒判过）")
	}
	// 茧壁缺位 ⇒ tsmRunGate 短路退 2（不真起子进程）。
	if rc, msg := tsmRunGate(filepath.Join(d, "空树")); rc != tsmExitHard || !strings.Contains(msg, "茧壁二进制不在") {
		t.Fatalf("茧壁缺位应退 2 且逐字说明，实 rc=%d msg=%q", rc, msg)
	}
}

func TestTsmParseArgsUnknownFlag(t *testing.T) {
	rc, _, errb := tsmTestRunEntry(t, "--nope")
	if rc != tsmExitHard || !strings.Contains(errb, "unrecognized arguments: --nope") {
		t.Fatalf("未知旗标应 rc=2 且错误进 stderr，实 rc=%d stderr=%q", rc, errb)
	}
	rc2, out2, _ := tsmTestRunEntry(t, "--help")
	if rc2 != tsmExitOK || !strings.Contains(out2, "usage:") {
		t.Fatalf("--help 应 rc=0 且打 usage，实 rc=%d", rc2)
	}
}

// ★ ⑲ 新搬格的回归牙：`--only <拼错的 id>` ⇒ rc=2 且逐字点名「未知的变异 id」· 不报绿。
// 哑化主入口的 `--only` 在册校验（py :168-174）⇒ 本用例 FAIL（`t.Fatalf` 带本行号）。
// 校验排在根/目标解析**之前** ⇒ 本用例不落盘、不碰仓。
func TestTsmOnlyTypoIsNotGreen(t *testing.T) {
	rc, out, errb := tsmTestRunEntry(t, "--only", "ZZ")
	if rc != tsmExitHard {
		t.Fatalf("拼错 id 应 rc=2（判不了 ≠ 全绿），实 rc=%d\nstdout=%s\nstderr=%s", rc, out, errb)
	}
	if !strings.Contains(errb, "未知的变异 id") {
		t.Fatalf("应逐字点名「未知的变异 id」于 stderr，实 stderr=%q", errb)
	}
	if strings.Contains(out, "变异验证全绿") {
		t.Fatalf("拼错 id 不得报绿\nstdout=%s", out)
	}
	// 成对向（纯成员判定，不落盘）：在册 id 不得被判「未知」。
	if !tsmOnlyInRoster("G1") || tsmOnlyInRoster("ZZ") {
		t.Fatalf("在册判定错：G1=%v（应 true）· ZZ=%v（应 false）", tsmOnlyInRoster("G1"), tsmOnlyInRoster("ZZ"))
	}
}

// ★ 重入形态的回归牙：夹具驱动必须以**真子命令形态**起（`gate check <键> --yes` 前置 ＋ `--overlay <fix>`）。
// 去掉 `gate check <键>` 前缀 ⇒ 复制出的 `zerg` 二进制按 `args[0]` 派发、直跑 `--overlay …` 落
// 「未知旗标」rc=2 ⇒ 本用例（探针回显 argv）FAIL 带行号。
// ★ 带 `--yes`：本判据登中央表 write D2（受治理 · GAP-20261001-64）⇒ 缺 `--yes` 壳侧拒执 ⇒
//
//	内部重入须自带确认盾（哑化 ⇒ 本用例 FAIL）。
func TestTsmFixtureDriverUsesSubcommandShape(t *testing.T) {
	fix := t.TempDir()
	dir := filepath.Join(fix, "scripts", "gates")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	probe := filepath.Join(dir, "two-states-gate-mutate.py") // 探针壳（回显 argv）· 与主件 tsmMutDriverRel 同名
	if err := os.WriteFile(probe, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	rc, out := tsmRunFixtureDriver(fix, "", "--only", "G1")
	if rc != tsmExitOK {
		t.Fatalf("探针应 rc=0，实 rc=%d out=%q", rc, out)
	}
	got := strings.Split(strings.TrimRight(out, "\n"), "\n")
	want := []string{"gate", "check", "two-states-gate-mutate", "--yes", "--overlay", fix, "--only", "G1"}
	if len(got) != len(want) {
		t.Fatalf("argv 段数不对：\n got=%q\nwant=%q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("argv[%d]=%q（应 %q）· 全 argv=%q", i, got[i], want[i], got)
		}
	}
}

// ★ 退役容忍（缺口 `GAP-20261007-16`）：本件**运行时/自检不读**仓内 `scripts/gates/two-states-gate-mutate.py`
// 的正文 —— 夹具里的「本件」是【硬拷贝 `tsmDriverPath()`（默认可执行自身）】⇒ 该 py 退役（不在盘）对本件零影响。
// 本用例坐实：驱动**源** = 可执行自身（非该 py）；且夹具驱动从**给定源**拷入（不是从仓内该 py）。
// 哑化 `tsmMkFixture` 改从仓内该 py 取源 ⇒ 本用例 FAIL（带本行号）。
func TestTsmDriverSourcedNotFromJudgePy(t *testing.T) {
	if tsmMutDriverRel != filepath.Join("scripts", "gates", "two-states-gate-mutate.py") {
		t.Fatalf("驱动**落点**应是该 py 路径（落点≠源），实 %q", tsmMutDriverRel)
	}
	saved, had := os.LookupEnv("ZERG_ZTS_MUT_DRIVER")
	os.Unsetenv("ZERG_ZTS_MUT_DRIVER")
	defer func() {
		if had {
			os.Setenv("ZERG_ZTS_MUT_DRIVER", saved)
		}
	}()
	drv := tsmDriverPath()
	if drv == "" {
		t.Fatal("驱动源（默认可执行自身）为空")
	}
	if strings.HasSuffix(drv, "two-states-gate-mutate.py") {
		t.Fatalf("驱动源不得是仓内该 py（应可执行自身）：%q", drv)
	}
	src := filepath.Join(t.TempDir(), "drv-probe")
	if err := os.WriteFile(src, []byte("#drv-probe\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	fix, e := tsmMkFixture(t.TempDir(), src, "fx", "", "")
	if e != "" {
		t.Fatalf("建夹具失败：%s", e)
	}
	got, _ := tsmReadText(filepath.Join(fix, tsmMutDriverRel))
	if got != "#drv-probe\n" {
		t.Fatalf("夹具驱动应逐字等于**给定源**（不是仓内该 py）：%q", got)
	}
}

// ★ 受治理的仓外写档（缺口 GAP-20261001-64 · Mr2109 2026-10-10 拍板「另立受治理的仓外写档」）：本判据名登
// **中央分类表** write D2 ⇒ 壳侧 `gate check` 缺 `--yes` 一律拒执（整面受治）；且夹具驱动重入自带 `--yes`
// （壳侧盾剔掉后才落给判据体 ⇒ 内部重入不被盾挡成 rc=2）。
// 哑化任一处（拿掉中央表登记 / 拿掉 argv 里的 `--yes`）⇒ 本用例 FAIL —— 「新增写工具同批登记」这一格的牙。
func TestTsmGovernedOutOfTreeWriteFace(t *testing.T) {
	cls, lvl := gateCheckClassOf("two-states-gate-mutate")
	if cls != "write" || lvl != dangerD2 {
		t.Fatalf("本判据名未登中央分类表 write D2 ⇒ 仓外写档未受治理：cls=%q lvl=%q", cls, lvl)
	}
	spec, hit := gateCheckClassEntry["two-states-gate-mutate"]
	if !hit || spec.Class != "write" || spec.Level != dangerD2 {
		t.Fatalf("中央表无 write D2 登记：hit=%v spec=%+v", hit, spec)
	}
	// 成对向：未登记者一律 read（默认不受盾）—— 免得「谁都 write」这种永真形。
	if c, l := gateCheckClassOf("no-such-judge-zzz"); c != "read" || l != "" {
		t.Fatalf("成对向：未登记者应为 read：cls=%q lvl=%q", c, l)
	}
}
