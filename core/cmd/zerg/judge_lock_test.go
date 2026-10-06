package main

// judge_lock_test.go —— judge_lock.go 的成对负控测试件（甲案 C 类）。
//
// 覆盖面：① 帮助面（py `-h` stdout 的 sha256 钉成 golden）② 无参 / 互斥组 / 未知旗标 /
// 非法 int 的 argparse 措辞面 ③ 判据名非法面 ④ 超槽池预算面 ⑤ 正控（含写面：锁件落哪）
// ⑥ `--self-test` 全绿 ＋ A 类夹具回收（跑后同前缀目录增量 = 0）⑦ **跨真进程**的同名互斥 /
// 不同名不互阻 / 槽池占满三态 ⑧ py 路径与空白原语的同构面（含 `\x1c-\x1f` 这枚 Go 不覆盖的空白）。
//
// ★ 子进程面：py 走 [sys.executable, abspath(__file__)]；Go 主件默认走 os.Executable()，
//   而**测试二进制不是 CLI 入口** ⇒ 本件用一枚仓外探针壳（把 CLI 参数转回测试二进制里的
//   TestJlkHelperProcess，在该进程内直调 judgeLockGateMain）对齐该面。主件默认档一字未改。

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// jlkHelpGoldenSHA256 —— py `-h` 的 stdout 字节流 sha256（`/usr/bin/python3` 3.9.6 · COLUMNS 未设）。
const jlkHelpGoldenSHA256 = "9a6589a9380103767c570012c61b0423191ee46d7db2098ddbf302baf91ae668"

func jlkSHA256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func jlkRunCLI(argv ...string) (int, string, string) {
	var out, errb bytes.Buffer
	rc := judgeLockGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

// ── 子进程壳 ────────────────────────────────────────────────────────────────

func jlkShQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// jlkTestProbe —— 造一枚仓外探针壳（等价于 py 的「自身可执行件」）。
func jlkTestProbe(t *testing.T) string {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	script := filepath.Join(t.TempDir(), "jl-probe.sh")
	body := "#!/bin/sh\nexec " + jlkShQuote(exe) + " '-test.run=^TestJlkHelperProcess$' -- \"$@\"\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatalf("写探针壳: %v", err)
	}
	return script
}

func jlkChildArgs() []string {
	for i, a := range os.Args {
		if a == "--" {
			return os.Args[i+1:]
		}
	}
	return nil
}

// TestJlkHelperProcess —— 探针壳的落点。仅当 argv 里带 `--` 分界（即被探针壳调起）时才当 CLI 入口。
func TestJlkHelperProcess(t *testing.T) {
	args := jlkChildArgs()
	if args == nil {
		return
	}
	// ★ 回归牙（真命令行面）：探针壳收到的 argv 必须带 inproc 路由前缀（`gate check judge-lock`）——
	//   缺前缀 = 主件重跑面破形（`bin/zerg` 会把本门旗标当未知命令 ⇒ 子进程退 2）⇒ 这里**不**兜底，让牙当场红。
	if !(len(args) >= 3 && args[0] == "gate" && args[1] == "check" && args[2] == "judge-lock") {
		io.WriteString(os.Stderr, "探针壳：argv 缺 inproc 路由前缀（gate check judge-lock）⇒ 重跑面破形\n")
		os.Exit(2)
	}
	os.Exit(judgeLockGateMain(os.Stdout, os.Stderr, args[3:]))
}

func jlkUseProbe(t *testing.T) {
	t.Helper()
	probe := jlkTestProbe(t)
	old := jlkSelfExe
	jlkSelfExe = func() (string, error) { return probe, nil }
	t.Cleanup(func() { jlkSelfExe = old })
}

func jlkCountPrefixed(t *testing.T, dir, prefix string) int {
	t.Helper()
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir(%s): %v", dir, err)
	}
	n := 0
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), prefix) {
			n++
		}
	}
	return n
}

// ── ① 帮助面 / argparse 措辞面 / 判据名非法 / 超预算 / 正控写面 ─────────────────

func TestJlkGateMainFaces(t *testing.T) {
	base := t.TempDir()
	t.Setenv("ZERG_GATE_LOCK_DIR", base)

	if got := jlkSHA256Hex(jlkHelp); got != jlkHelpGoldenSHA256 {
		t.Fatalf("帮助常量 sha256 漂移：%s", got)
	}
	rc, out, errS := jlkRunCLI("-h")
	if rc != 0 || errS != "" {
		t.Fatalf("-h：rc=%d stderr=%q", rc, errS)
	}
	if out != jlkHelp {
		t.Fatalf("-h stdout 与帮助常量逐字不等")
	}
	if got := jlkSHA256Hex(out); got != jlkHelpGoldenSHA256 {
		t.Fatalf("现跑 -h stdout sha256=%s ≠ golden %s", got, jlkHelpGoldenSHA256)
	}

	rc, out, errS = jlkRunCLI()
	if rc != 2 || out != "" || errS != jlkUsageErr {
		t.Fatalf("无参档：rc=%d out=%q err=%q", rc, out, errS)
	}

	cases := []struct {
		name  string
		argv  []string
		rc    int
		errS  string
		isOut bool
	}{
		{"互斥组", []string{"--self-test", "--name", "x"}, 2,
			jlkUsageHead + jlkProg + ": error: argument --name: not allowed with argument --self-test\n", true},
		{"互斥组反向", []string{"--name", "x", "--self-test"}, 2,
			jlkUsageHead + jlkProg + ": error: argument --self-test: not allowed with argument --name\n", true},
		{"未知旗标", []string{"--zzz"}, 2,
			jlkUsageHead + jlkProg + ": error: unrecognized arguments: --zzz\n", true},
		{"非法 int", []string{"--slots", "abc"}, 2,
			jlkUsageHead + jlkProg + ": error: argument --slots: invalid int value: 'abc'\n", true},
		{"非法 float", []string{"--wait", "abc"}, 2,
			jlkUsageHead + jlkProg + ": error: argument --wait: invalid float value: 'abc'\n", true},
		{"判据名含斜杠", []string{"--name", "a/b"}, 2, "⛔ 判据名非法（空 / 含路径分隔符 / 首尾空白）⇒ 不给结论（rc=2）\n", true},
		{"判据名首尾空白", []string{"--name", " a"}, 2, "⛔ 判据名非法（空 / 含路径分隔符 / 首尾空白）⇒ 不给结论（rc=2）\n", true},
		{"判据名空串走无参档", []string{"--name", ""}, 2, jlkUsageErr, true},
		{"超槽池预算", []string{"--name", "alpha", "--slots", "9"}, 2,
			"⛔ 超槽池预算：请求 --slots 9 > 容量 8 ⇒ **不硬起 / 不超发** ⇒ 不给结论（rc=2）\n", true},
	}
	for _, c := range cases {
		rc, out, errS := jlkRunCLI(c.argv...)
		if rc != c.rc {
			t.Fatalf("%s：rc=%d 期望 %d（stderr=%q）", c.name, rc, c.rc, errS)
		}
		if c.isOut && out != "" {
			t.Fatalf("%s：该档不该有 stdout：%q", c.name, out)
		}
		if errS != c.errS {
			t.Fatalf("%s：stderr\n got=%q\nwant=%q", c.name, errS, c.errS)
		}
	}

	// 正控（放在最后：本进程会一直持锁到退出 ⇒ 不污染上面的格）
	rc, out, errS = jlkRunCLI("--name", "alpha", "--slots", "4", "--wait", "0")
	want := "✓ 件级锁 judge-alpha.lock ＋ 槽位 1/8 已取（rc=0）· 锁目录=" + base + "\n"
	if rc != 0 || errS != "" || out != want {
		t.Fatalf("正控：rc=%d out=%q err=%q", rc, out, errS)
	}
	if _, err := os.Stat(filepath.Join(base, "judge-alpha.lock")); err != nil {
		t.Fatalf("件级锁件未落：%v", err)
	}
	if _, err := os.Stat(filepath.Join(base, "judge-slot-0.lock")); err != nil {
		t.Fatalf("槽位锁件未落：%v", err)
	}
}

// ── ⑥ 自检全绿 ＋ A 类夹具回收（delta=0）─────────────────────────────────────

func TestJlkSelfTestGreenAndFixtureReclaimed(t *testing.T) {
	jlkUseProbe(t)
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)

	before := jlkCountPrefixed(t, tmp, "judge-lock-self-")
	rc, out, errS := jlkRunCLI("--self-test")
	if rc != 0 {
		t.Fatalf("--self-test：rc=%d stderr=%q stdout=%q", rc, errS, out)
	}
	for _, w := range []string{
		"✓ 正控：无竞争",
		"✓ 负控A（同名两路竞争）",
		"✓ 负控B（不同件不互阻）",
		"✓ 负控C（槽池占满",
		"✓ 负控D（超预算",
	} {
		if !strings.Contains(out, w) {
			t.Fatalf("自检缺行 %q：stdout=%q", w, out)
		}
	}
	after := jlkCountPrefixed(t, tmp, "judge-lock-self-")
	if after != before {
		t.Fatalf("A 类夹具未回收：同前缀计数 before=%d after=%d", before, after)
	}
}

// ── ⑦ 跨真进程三态（负控有牙）──────────────────────────────────────────────

func TestJlkJudgeLockContentionCrossProcess(t *testing.T) {
	jlkUseProbe(t)
	base := t.TempDir()
	t.Setenv("ZERG_GATE_LOCK_DIR", base)

	holder, err := jlkRunChild([]string{"--name", "dup", "--slots", "2", "--wait", "0",
		jlkHoldFlag, "5"}, base)
	if err != nil {
		t.Fatalf("起持锁子进程: %v", err)
	}
	hout, err := holder.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	holder.Stderr = &bytes.Buffer{}
	if err := holder.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() {
		_ = holder.Process.Signal(syscall.SIGTERM)
		done := make(chan struct{})
		go func() { _, _ = holder.Process.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			_ = holder.Process.Kill()
			<-done
		}
	}()
	sc := bufio.NewScanner(hout)
	held := false
	for sc.Scan() {
		if strings.TrimSpace(sc.Text()) == "HELD" {
			held = true
			break
		}
	}
	if !held {
		t.Fatalf("持锁子进程未就绪（未读到 HELD）")
	}

	// 同名 ⇒ 必 rc=2 且点名 detail
	rc, out, errS := jlkRunCLI("--name", "dup", "--slots", "1", "--wait", "0")
	if rc != 2 {
		t.Fatalf("同名竞争应 rc=2，实得 %d（stdout=%q stderr=%q）", rc, out, errS)
	}
	if !strings.Contains(errS, "detail=judge_locked_by_other") {
		t.Fatalf("同名档缺具名文案：%q", errS)
	}
	if out != "" {
		t.Fatalf("同名档不该有 stdout：%q", out)
	}

	// 不同名 ⇒ 照拿（rc=0）
	rc, out, errS = jlkRunCLI("--name", "other", "--slots", "2", "--wait", "0")
	if rc != 0 {
		t.Fatalf("不同名应 rc=0，实得 %d（stderr=%q）", rc, errS)
	}
	if !strings.Contains(out, "judge-other.lock") {
		t.Fatalf("不同名档 stdout 未点名锁件：%q", out)
	}

	// 槽池被占满（slot0 由子进程持）⇒ 排队过久 rc=2
	rc, _, errS = jlkRunCLI("--name", "third", "--slots", "1", "--wait", "0")
	if rc != 2 {
		t.Fatalf("槽池占满应 rc=2，实得 %d（stderr=%q）", rc, errS)
	}
	if !strings.Contains(errS, "槽池全占") {
		t.Fatalf("槽池档缺具名文案：%q", errS)
	}
}

// ── ⑧ py 原语同构面（含 Go unicode.IsSpace 不覆盖的 \x1c-\x1f）───────────────

func TestJlkPyPathPrimitives(t *testing.T) {
	joins := []struct {
		a    string
		p    []string
		want string
	}{
		{"/tmp", []string{"zerg-gate-instance"}, "/tmp/zerg-gate-instance"},
		{"a", []string{"", "b"}, "a/b"},
		{"a", []string{""}, "a/"},
		{"", []string{"a"}, "a"},
		{"repo", []string{"/abs"}, "/abs"},
		{"a", []string{"../b"}, "a/../b"},
		{"d/", []string{"x"}, "d/x"},
	}
	for _, c := range joins {
		if got := jlkPyJoin(c.a, c.p...); got != c.want {
			t.Fatalf("jlkPyJoin(%q,%q)=%q 期望 %q", c.a, c.p, got, c.want)
		}
	}
	strips := []struct{ in, want string }{
		{" a", "a"},
		{"a ", "a"},
		{"\ta\n", "a"},
		{" a\x1c", "a"},
		{"\x1f a\x1f", "a"},
		{"a\u00a0", "a"},
		{"a\u0085", "a"},
		{"\x00a", "\x00a"}, // NUL 不是 py 的空白 ⇒ 不剥
	}
	for _, c := range strips {
		if got := jlkPyStrip(c.in); got != c.want {
			t.Fatalf("jlkPyStrip(%q)=%q 期望 %q", c.in, got, c.want)
		}
	}
	if got := jlkPyDirname("/a/b"); got != "/a" {
		t.Fatalf("jlkPyDirname(/a/b)=%q", got)
	}
	if got := jlkPyDirname("a"); got != "" {
		t.Fatalf("jlkPyDirname(a)=%q", got)
	}
	if got := jlkPyFloatStr(0); got != "0.0" {
		t.Fatalf("jlkPyFloatStr(0)=%q", got)
	}
	if got := jlkPyFloatStr(2.5); got != "2.5" {
		t.Fatalf("jlkPyFloatStr(2.5)=%q", got)
	}
}
