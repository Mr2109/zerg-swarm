package main

// check-py-go-pairing-drift (Go 真搬体) 的成对负控测试。
// ✗ 不定义 TestMain（包内已有他件占用）· ✗ 加包级 init（用带前缀的显式 helper）。

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// TestCpdpSelfTestPasses —— 自检 20 格全过；★ 断言「20 格」这个数（少一格即红）。
func TestCpdpSelfTestPasses(t *testing.T) {
	var out, errb bytes.Buffer
	rc := pgpdSelfTest(&out, &errb)
	if rc != 0 {
		t.Fatalf("自检 rc=%d（应 0）· stderr=%s", rc, errb.String())
	}
	got := out.String()
	if n := strings.Count(got, "  ✓"); n != pgpdWantCases {
		t.Fatalf("自检 ✓ 用例数 = %d（应 %d）", n, pgpdWantCases)
	}
	if !strings.Contains(got, "自证用例：20 条") {
		t.Fatalf("缺「自证用例：20 条」行")
	}
	if !strings.Contains(got, "⇒ 自检 全过（rc=0）") {
		t.Fatalf("缺全过结语")
	}
	// ★ ⑪/⑫ 的内层 main() stderr 泄漏（py _run_inproc 只重定向 stdout）—— 这条是真行为，✗ 不得被静默。
	if !strings.Contains(errb.String(), "inproc 表读不到") {
		t.Fatalf("缺 ⑪ 泄漏行（inproc 表读不到）")
	}
	if !strings.Contains(errb.String(), "棘轮 probe_argv") {
		t.Fatalf("缺 ⑫ 泄漏行（棘轮 probe_argv 不符）")
	}
}

// TestCpdpMaskAndRealDiff —— 成对：已声明非确定面被掩同 ／ 真差不得被掩。
func TestCpdpMaskAndRealDiff(t *testing.T) {
	a := pgpdMask("done 2026-10-05 12:00:00 in 0.31s at /var/folders/ab/T/xyz")
	b := pgpdMask("done 2026-10-06 01:02:03 in 9.99s at /var/folders/zz/T/qwerty")
	if a != b {
		t.Fatalf("掩码未归一非确定面: %q vs %q", a, b)
	}
	// 负控：真差在掩码后必须仍然不同。
	c := pgpdMask("done 2026-10-05 12:00:00 in 0.31s REAL-DIFF")
	if c == a {
		t.Fatalf("掩码吞掉了真差（假绿）: %q", c)
	}
	// 毫秒墙钟面（批-2.1 新增）。
	if pgpdMask("cost 12 ms") != pgpdMask("cost 34 ms") {
		t.Fatalf("毫秒墙钟未被掩")
	}
	// 帧行 / 回溯脚手架面。
	tb := pgpdMask("Traceback (most recent call last):\n  File \"/x/os.py\", line 679, in __getitem__\n    w = os.environ[\"K\"]\nKeyError: 'K'\n")
	if !strings.Contains(tb, "KeyError: 'K'") {
		t.Fatalf("回溯折叠吃掉了末行正文: %q", tb)
	}
	if strings.Contains(tb, "Traceback") || strings.Contains(tb, "line 679") {
		t.Fatalf("回溯脚手架未被折叠: %q", tb)
	}
}

// TestCpdpWriteClassYesPaired —— 成对负控：写类件补壳级 --yes ／ 非写类件 ✗ 不带。
func TestCpdpWriteClassYesPaired(t *testing.T) {
	fix := pgpdMkFixture(t.TempDir(), []string{"k-w"}, nil, nil, nil, nil)
	pgpdWrite(pgpdPyJoin(fix, pgpdGatesRel, "k-w.sh"), "#!/bin/sh\n#zerg-class: write D2\necho OUT-k-w\n")

	orig := pgpdRunSide
	defer func() { pgpdRunSide = orig }()

	var seen [][]string
	pgpdRunSide = func(argv []string, cwd string, timeout int) (*int, string, string, bool) {
		seen = append(seen, append([]string{}, argv...))
		z := 0
		return &z, "", "", false
	}
	pgpdProbe(fix, "k-w", []string{"--self-test"}, "/bin/zerg", "python3", "scripts/gates/k-w.sh", "bash")
	if !pgpdIsWriteClass(fix, "scripts/gates/k-w.sh") {
		t.Fatal("写类件未被识别（is_write_class=False）")
	}
	if len(seen) == 0 || !pgpdContains(seen[0], "--yes") {
		t.Fatalf("写类件 side_a 缺 --yes: %v", seen)
	}
	if len(seen[0]) >= 6 && seen[0][4] != "--yes" {
		t.Fatalf("--yes 未紧贴键名: %v", seen[0])
	}

	// ── 非写类（成对负控）──
	pgpdWrite(pgpdPyJoin(fix, pgpdGatesRel, "k-plain.sh"), "#!/bin/sh\necho hi\n")
	seen = nil
	pgpdRunSide = func(argv []string, cwd string, timeout int) (*int, string, string, bool) {
		seen = append(seen, append([]string{}, argv...))
		z := 0
		return &z, "", "", false
	}
	pgpdProbe(fix, "k-plain", []string{"--self-test"}, "/bin/zerg", "python3", "scripts/gates/k-plain.sh", "bash")
	if pgpdIsWriteClass(fix, "scripts/gates/k-plain.sh") {
		t.Fatal("非写类件被误判为写类")
	}
	if pgpdContains(seen[0], "--yes") {
		t.Fatalf("非写类件被误加 --yes（放宽了）: %v", seen[0])
	}
}

// TestCpdpArgsErrorAndHelp —— 帮助面逐字 ／ 未知旗标 ／ 缺参 ／ 歧义前缀（argparse 契约面）。
func TestCpdpArgsErrorAndHelp(t *testing.T) {
	var out, errb bytes.Buffer
	if rc := pgpdMain([]string{"-h"}, &out, &errb); rc != 0 || out.String() != pgpdHelpText {
		t.Fatalf("-h 帮助面不逐字（rc=%d）", rc)
	}
	out.Reset()
	errb.Reset()
	if rc := pgpdMain([]string{"--nope"}, &out, &errb); rc != 2 || !strings.Contains(errb.String(), "unrecognized arguments: --nope") {
		t.Fatalf("未知旗标档 rc=%d err=%q", rc, errb.String())
	}
	out.Reset()
	errb.Reset()
	if rc := pgpdMain([]string{"--root"}, &out, &errb); rc != 2 || !strings.Contains(errb.String(), "argument --root: expected one argument") {
		t.Fatalf("缺参档 rc=%d err=%q", rc, errb.String())
	}
	out.Reset()
	errb.Reset()
	if rc := pgpdMain([]string{"--s"}, &out, &errb); rc != 2 || !strings.Contains(errb.String(), "ambiguous option: --s could match --strict, --self-test") {
		t.Fatalf("歧义前缀档 rc=%d err=%q", rc, errb.String())
	}
}

// TestCpdpStrictRatchetPaired —— 成对：棘轮涨 ⇒ rc=1 ／ 差集已在棘轮 ⇒ rc=0。
func TestCpdpStrictRatchetPaired(t *testing.T) {
	base := t.TempDir()

	f2 := pgpdMkFixture(base, nil, nil, nil, []string{}, nil)
	pgpdBumpSide(f2, "go", "k-alpha", "k-alphb")
	var o, e bytes.Buffer
	if rc := pgpdMain([]string{"--root", f2, "--strict"}, &o, &e); rc != 1 || !strings.Contains(o.String(), "棘轮涨了") {
		t.Fatalf("棘轮涨档 rc=%d out=%q", rc, o.String())
	}

	f3 := pgpdMkFixture(base, nil, nil, nil, []string{"k-alpha"}, nil)
	pgpdBumpSide(f3, "go", "k-alpha", "k-alphb")
	o.Reset()
	e.Reset()
	if rc := pgpdMain([]string{"--root", f3, "--strict"}, &o, &e); rc != 0 {
		t.Fatalf("棘轮已知档 rc=%d（应 0）", rc)
	}
}

// TestCpdpArgvMismatchRc2 —— 棘轮 probe_argv 与本次 argv 不符 ⇒ 不给结论 rc=2。
func TestCpdpArgvMismatchRc2(t *testing.T) {
	f := pgpdMkFixture(t.TempDir(), nil, nil, []string{"--self-test"}, nil, nil)
	var o, e bytes.Buffer
	if rc := pgpdMain([]string{"--root", f, "--argv="}, &o, &e); rc != 2 || !strings.Contains(e.String(), "不可比") {
		t.Fatalf("argv 不符档 rc=%d err=%q", rc, e.String())
	}
}

// TestCpdpEmptyTableRc2 —— 空域 ≠ 全绿（inproc 表在盘但解析 0 键 ⇒ rc=2）。
func TestCpdpEmptyTableRc2(t *testing.T) {
	fix := t.TempDir()
	os.MkdirAll(pgpdPyJoin(fix, "core", "cmd", "zerg"), 0o755)
	os.MkdirAll(pgpdPyJoin(fix, "scripts", "gates"), 0o755)
	os.MkdirAll(pgpdPyJoin(fix, "bin"), 0o755)
	pgpdWrite(pgpdPyJoin(fix, pgpdInprocRel), "package main\n\nvar x = 1\n")
	var o, e bytes.Buffer
	if rc := pgpdMain([]string{"--root", fix}, &o, &e); rc != 2 || !strings.Contains(e.String(), "解析出 0 键") {
		t.Fatalf("空表档 rc=%d err=%q", rc, e.String())
	}
}

// TestCpdpDefaultRootPairedRoot —— 缺省根口径（DEV-1872）：无 `--root` ⟷ 有 `--root`（=仓根）**同根** ·
// env `ZERG_REPO_ROOT` 优先 · 仓外无锚 ⇒ fail-closed。
// 哑化靶 = `pgpdDefaultRoot()`（件 `check_py_go_pairing_drift.go`）：若把它的 env 支 / 上溯支退回
// 「cwd/`..`, `..`」旧写法 ⇒ ②③ 两格转红（成对负控）。
func TestCpdpDefaultRootPairedRoot(t *testing.T) {
	// ① env 支优先：置 ZERG_REPO_ROOT 到一个含 scripts/gates 的夹具 ⇒ 缺省根 = 该夹具（cwd 无关）。
	envFix := t.TempDir()
	os.MkdirAll(pgpdPyJoin(envFix, pgpdGatesRel), 0o755)
	t.Setenv("ZERG_REPO_ROOT", envFix)
	if got := pgpdDefaultRoot(); got != pgpdAbspath(envFix) {
		t.Fatalf("env 支未优先：缺省根=%q（应 %q）", got, pgpdAbspath(envFix))
	}

	// ② cwd 上溯支：清 env（空串 ⇒ 视同未设）⇒ cwd=本包目录上溯必落到真仓根（含 scripts/gates）。
	t.Setenv("ZERG_REPO_ROOT", "")
	noRoot := pgpdDefaultRoot()
	if noRoot == "" {
		t.Fatalf("cwd 在仓内时缺省根为空（应上溯到仓根）")
	}
	if st, err := os.Stat(pgpdPyJoin(noRoot, pgpdGatesRel)); err != nil || !st.IsDir() {
		t.Fatalf("缺省根 %q 下没有 %s", noRoot, pgpdGatesRel)
	}
	// ★ 硬断言：无 `--root` ⟷ 有 `--root <同值>` ⇒ 入口两态 rc/stdout/stderr 逐字同。
	var o1, e1, o2, e2 bytes.Buffer
	rc1 := pgpdMain([]string{"--only", "zzz-absent-key"}, &o1, &e1)
	rc2 := pgpdMain([]string{"--root", noRoot, "--only", "zzz-absent-key"}, &o2, &e2)
	if rc1 != rc2 || o1.String() != o2.String() || e1.String() != e2.String() {
		t.Fatalf("无 --root ⟷ 有 --root 不同根：rc %d/%d\nout1=%q\nout2=%q\nerr1=%q\nerr2=%q",
			rc1, rc2, o1.String(), o2.String(), e1.String(), e2.String())
	}

	// ③ 负控：仓外无锚（env 未设 ＋ cwd 不在仓内）⇒ fail-closed 出声（rc=2），✗ 不回落 cwd/`..` 猜。
	t.Chdir(t.TempDir())
	if got := pgpdDefaultRoot(); got != "" {
		t.Fatalf("仓外无锚应 fail-closed（''），得 %q", got)
	}
	var o, e bytes.Buffer
	if rc := pgpdMain([]string{"--only", "zzz-absent-key"}, &o, &e); rc != 2 || !strings.Contains(e.String(), "仓根定位失败") {
		t.Fatalf("仓外无锚 ⇒ 退码 2 ＋ 出声；得 rc=%d err=%q", rc, e.String())
	}
}
