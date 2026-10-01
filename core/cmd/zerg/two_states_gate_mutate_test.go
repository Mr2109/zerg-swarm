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

func TestTwoStatesMutateGateApplyMutationExactlyOnce(t *testing.T) {
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

func TestTwoStatesMutateGatePyJoinThreeSemantics(t *testing.T) {
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

func TestTwoStatesMutateGateFixtureGateAnchorsAndTokens(t *testing.T) {
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

func TestTwoStatesMutateGateDriveRedRestoreRerunGreen(t *testing.T) {
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
func TestTwoStatesMutateGateMirrorBlindIsJudgedRed(t *testing.T) {
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
func TestTwoStatesMutateGateRestoreSkippedIsCaught(t *testing.T) {
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
func TestTwoStatesMutateGateDefaultIsReadOnly(t *testing.T) {
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
func TestTwoStatesMutateGateRefusesRepoWithGit(t *testing.T) {
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

func TestTwoStatesMutateGatePairGuardHalfIsNotGreen(t *testing.T) {
	tsmTestInjectMirror(t)
	fix := tsmTestFixture(t, "", "")
	t.Chdir(fix)

	rc, out, _ := tsmTestRunEntry(t, "--pair-guard", "--only", "G1")
	if rc != tsmExitHard || !strings.Contains(out, "缺半即判不了") {
		t.Fatalf("--pair-guard --only G1 应 rc=2 且点名「缺半即判不了」，实 rc=%d\n%s", rc, out)
	}
	rc2, out2, _ := tsmTestRunEntry(t, "--pair-guard", "--only", "G9")
	if rc2 != tsmExitHard || !strings.Contains(out2, "只跑 0/3 条") {
		t.Fatalf("--only 不存在的 id ⇒ 0 条应 rc=2，实 rc=%d\n%s", rc2, out2)
	}
}

func TestTwoStatesMutateGateRealCompilesAndWallShortCircuit(t *testing.T) {
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

func TestTwoStatesMutateGateParseArgsUnknownFlag(t *testing.T) {
	rc, _, errb := tsmTestRunEntry(t, "--nope")
	if rc != tsmExitHard || !strings.Contains(errb, "unrecognized arguments: --nope") {
		t.Fatalf("未知旗标应 rc=2 且错误进 stderr，实 rc=%d stderr=%q", rc, errb)
	}
	rc2, out2, _ := tsmTestRunEntry(t, "--help")
	if rc2 != tsmExitOK || !strings.Contains(out2, "usage:") {
		t.Fatalf("--help 应 rc=0 且打 usage，实 rc=%d", rc2)
	}
}
