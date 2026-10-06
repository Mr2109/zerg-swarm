package main

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// wlkRun 跑入口，返回 (rc, stdout, stderr)。
func wlkRun(argv ...string) (int, string, string) {
	var out, errb bytes.Buffer
	rc := writerLockGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

// wlkSeedOwner 在 d 预置一枚 owner 件（who 指定 · 其余字段钉常量 ⇒ 除「已持有 Ns」外全确定）。
func wlkSeedOwner(t *testing.T, d, who string) {
	t.Helper()
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "who=" + who + "\npid=99999\nhost=fixturehost\n" +
		"since=2020-01-01T00:00:00+0000\nsince_epoch=1577836800\ndir=" + d + "\n"
	if err := os.WriteFile(filepath.Join(d, "owner"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func wlkLockPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), ".writer-lock")
}

func TestWlkHelp(t *testing.T) {
	for _, a := range []string{"-h", "--help"} {
		rc, out, errb := wlkRun(a)
		if rc != 0 || out != wlkUsage || errb != "" {
			t.Fatalf("%s：rc=%d err=%q（要 0 / usage / 空）", a, rc, errb)
		}
	}
}

func TestWlkUsageErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		argv []string
		want string
	}{
		{"未知旗标", []string{"--bogus"}, "用法错：未知旗标 --bogus（见 --help）\n"},
		{"多余位置参数", []string{"foo"}, "用法错：多余的位置参数 foo（见 --help）\n"},
		{"缺子命令", nil, "用法错：缺子命令（acquire|release|status · 见 --help）\n"},
		{"--dir 缺实参", []string{"--dir"}, "用法错：--dir 缺实参（见 --help）\n"},
		{"--who 缺实参", []string{"--who"}, "用法错：--who 缺实参（见 --help）\n"},
	} {
		rc, out, errb := wlkRun(tc.argv...)
		if rc != 2 || out != "" || errb != tc.want {
			t.Fatalf("%s：rc=%d out=%q err=%q", tc.name, rc, out, errb)
		}
	}
}

func TestWlkAcquireFresh(t *testing.T) {
	d := wlkLockPath(t)
	rc, out, errb := wlkRun("acquire", "--dir", d, "--who", "wA")
	if rc != 0 || errb != "" {
		t.Fatalf("空目录取锁：rc=%d err=%q", rc, errb)
	}
	if !strings.Contains(out, "✅ 取锁成功："+d+"（持有者 wA）") || !strings.Contains(out, "持有者：wA") {
		t.Fatalf("取锁 stdout 不符：%q", out)
	}
	if !wlkHasLine(filepath.Join(d, "owner"), "who=wA") {
		t.Fatalf("owner 未记下持有者")
	}
}

// TestWlkAcquireWhileHeld —— 负控：锁已存在必拒（rc=2）且逐字点名持有者。
// 哑化（把 mkdir 失败的 rc 也当成功）⇒ 本用例 rc 从 2 变 0 ⇒ 必红。
func TestWlkAcquireWhileHeld(t *testing.T) {
	d := wlkLockPath(t)
	wlkSeedOwner(t, d, "wA")
	rc, out, errb := wlkRun("acquire", "--dir", d, "--who", "wB")
	if rc != 2 || !strings.Contains(errb, "取锁被拒：锁已存在") {
		t.Fatalf("已持有再取：rc=%d err=%q", rc, errb)
	}
	if !strings.Contains(out, "持有者：wA") || !strings.Contains(out, "持有时刻：") {
		t.Fatalf("未逐字点名持有者/时刻：%q", out)
	}
}

// TestWlkNonHolderRelease —— 负控：非持有者放锁必拒（rc=1）、锁不许被放掉。
// 哑化（去掉 who 相等判定）⇒ rc 从 1 变 0、锁被删 ⇒ 必红。
func TestWlkNonHolderRelease(t *testing.T) {
	d := wlkLockPath(t)
	wlkSeedOwner(t, d, "wA")
	rc, _, errb := wlkRun("release", "--dir", d, "--who", "wZ")
	if rc != 1 || !strings.Contains(errb, "你不是持有者") || !strings.Contains(errb, "不许代放") {
		t.Fatalf("非持有者放锁：rc=%d err=%q", rc, errb)
	}
	if !wlkIsDir(d) || !wlkIsRegular(filepath.Join(d, "owner")) {
		t.Fatalf("锁被误放：目录=%v owner=%v", wlkIsDir(d), wlkIsRegular(filepath.Join(d, "owner")))
	}
}

func TestWlkHolderRelease(t *testing.T) {
	d := wlkLockPath(t)
	wlkSeedOwner(t, d, "wA")
	rc, out, errb := wlkRun("release", "--dir", d, "--who", "wA")
	if rc != 0 || errb != "" || !strings.Contains(out, "✅ 放锁成功：") {
		t.Fatalf("持有者放锁：rc=%d out=%q err=%q", rc, out, errb)
	}
	if wlkIsDir(d) {
		t.Fatalf("放锁后锁目录未清")
	}
}

func TestWlkReleaseNoLock(t *testing.T) {
	d := wlkLockPath(t)
	rc, _, errb := wlkRun("release", "--dir", d, "--who", "wA")
	if rc != 1 || !strings.Contains(errb, "没有锁可放") {
		t.Fatalf("无锁放锁：rc=%d err=%q", rc, errb)
	}
}

func TestWlkReleaseOwnerUnreadable(t *testing.T) {
	// 锁目录在、owner 件缺 ⇒ 判不了 ⇒ rc=2（不许当成功）。
	d := wlkLockPath(t)
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	rc, _, errb := wlkRun("release", "--dir", d, "--who", "wA")
	if rc != 2 || !strings.Contains(errb, "owner 件读不到") {
		t.Fatalf("owner 读不到：rc=%d err=%q", rc, errb)
	}
}

func TestWlkStatusNoLock(t *testing.T) {
	d := wlkLockPath(t)
	rc, out, errb := wlkRun("status", "--dir", d)
	if rc != 0 || errb != "" || !strings.Contains(out, "锁状态：无锁（"+d+" 不存在）") {
		t.Fatalf("查无锁：rc=%d out=%q err=%q", rc, out, errb)
	}
}

func TestWlkStatusHeldOwnerless(t *testing.T) {
	// 锁目录在、owner 缺 ⇒ 报「持有者不明」但仍 rc=0（只读）。
	d := wlkLockPath(t)
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	rc, out, _ := wlkRun("status", "--dir", d)
	if rc != 0 || !strings.Contains(out, "持有者不明（owner 件读不到）") {
		t.Fatalf("查无 owner：rc=%d out=%q", rc, out)
	}
}

// TestWlkNonDirOccupant —— 负控：锁落点被非目录占着 ⇒ rc=1（确实的坏形态）、且不许删它。
func TestWlkNonDirOccupant(t *testing.T) {
	d := wlkLockPath(t)
	if err := os.WriteFile(d, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rc, _, errb := wlkRun("acquire", "--dir", d, "--who", "wA")
	if rc != 1 || !strings.Contains(errb, "非目录占着") {
		t.Fatalf("非目录占着：rc=%d err=%q", rc, errb)
	}
	if wlkIsDir(d) {
		t.Fatalf("占位件被误换成目录")
	}
}

// TestWlkDefaultDirViaRepoEnv —— 缺省锁落点 = <仓根>/scripts/gates/.writer-lock（repoRoot 首选项）。
func TestWlkDefaultDirViaRepoEnv(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ZERG_REPO", root)
	t.Setenv("ZERG_WRITER", "wEnv")
	rc, out, _ := wlkRun("acquire")
	want := filepath.Join(root, "scripts", "gates", ".writer-lock")
	if rc != 0 || !strings.Contains(out, "✅ 取锁成功："+want+"（持有者 wEnv）") {
		t.Fatalf("缺省落点：rc=%d out=%q（want %s）", rc, out, want)
	}
}

// TestWlkOwnerFieldAwkSemantics —— 负控：钉住 awk `-F= '$1==k {sub(/^[^=]*=/,"");print;exit}'` 的三态。
// 哑化（改按最后一个 = 拆 / 不取首个命中）⇒ 必红。
func TestWlkOwnerFieldAwkSemantics(t *testing.T) {
	p := filepath.Join(t.TempDir(), "owner")
	// k=extra：首个 = 之后全部保留（值里含 =）。
	// dup：同名两行 ⇒ 取首个命中行。
	// noeq：无 = 的行 ⇒ 整行即值。
	body := "who=wA=extra\ndup=first\ndup=second\nnoeq\n"
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if v, ok := wlkOwnerField(p, "who"); !ok || v != "wA=extra" {
		t.Fatalf("首个 = 之后全部：got %q ok=%v", v, ok)
	}
	if v, _ := wlkOwnerField(p, "dup"); v != "first" {
		t.Fatalf("同名取首个命中：got %q", v)
	}
	if v, ok := wlkOwnerField(p, "noeq"); !ok || v != "noeq" {
		t.Fatalf("无 = 的行回整行：got %q ok=%v", v, ok)
	}
	if _, ok := wlkOwnerField(p, "absent"); ok {
		t.Fatalf("缺席键不得命中")
	}
}

// TestWlkSelfTest —— 自检 18 格全过（进程内手段）。
func TestWlkSelfTest(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	rc, out, errb := wlkRun("--self-test")
	if rc != 0 || errb != "" {
		t.Fatalf("自检：rc=%d err=%q", rc, errb)
	}
	if !strings.Contains(out, "18 格过 · 0 格红") {
		t.Fatalf("自检格数不符：%q", out)
	}
}

// TestWlkSelftestTmpJoinKeepsDoubleSlash —— 口径差②（路径拼接语义差）钉一格。
// 原件 `tmp="$(mktemp -d "${TMPDIR:-/tmp}/wl-selftest.XXXXXX")"` 是**字符串拼接** ⇒ `$TMPDIR` 尾带 `/` 时夹具路径原样留 `//`。
// 哑化（把 wlkMkdtempRaw 改回 os.MkdirTemp / 内部改走 filepath.Join|Clean）⇒ 收敛成单 `/`
// ⇒ 下方 strings.Contains 断言 FAIL（本函数那一行），且两态 stdout 差一字节。
func TestWlkSelftestTmpJoinKeepsDoubleSlash(t *testing.T) {
	base := t.TempDir() + "/" // 尾带 `/`（照本机 TMPDIR 形态：/var/folders/…/T/）
	t.Setenv("TMPDIR", base)
	rc, out, errb := wlkRun("--self-test")
	if rc != 0 || errb != "" {
		t.Fatalf("自检：rc=%d err=%q", rc, errb)
	}
	want := base + "/wl-selftest." // base 已含尾 `/` ⇒ want 含 `//`
	if !strings.Contains(out, want) {
		t.Fatalf("夹具路径未保住尾斜杠（要含 %q）：%q", want, out)
	}
}

// TestWlkSelftestSuffixAlnumRandom —— 口径差③（非确定面）钉一格：随机后缀两侧各自现造 ⇒ py 两发即异 ⇒ 声明掩码即可。
// 钉住后缀形状 = 6 位 [A-Za-z0-9] 且**两发不同**。
// 哑化（把后缀写成定值 / 零长 / 非 alnum）⇒ 下方形状断言或「两发不同」断言 FAIL（本函数对应行）。
func TestWlkSelftestSuffixAlnumRandom(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	rx := regexp.MustCompile(`wl-selftest\.([A-Za-z0-9]+)`)
	shape := regexp.MustCompile(`^[A-Za-z0-9]{6}$`)
	get := func() string {
		rc, out, errb := wlkRun("--self-test")
		if rc != 0 || errb != "" {
			t.Fatalf("自检：rc=%d err=%q", rc, errb)
		}
		m := rx.FindStringSubmatch(out)
		if m == nil {
			t.Fatalf("未见夹具路径（要含 wl-selftest.<SFX>）：%q", out)
		}
		return m[1]
	}
	a := get()
	if !shape.MatchString(a) {
		t.Fatalf("后缀形状不符（要 6 位 [A-Za-z0-9]）：%q", a)
	}
	if a == get() {
		t.Fatalf("两发后缀相同（%q）—— 随机面丢了（非确定面应两发即异）", a)
	}
}

// TestWlkShellConfirmFlagStaysShellOnly —— 口径差①（写工具契约差）钉一格。
// `--yes` / `--dry-run` 是**壳级确认盾**（`family_gate_check.go` 在 exec 前消费、✗ 不透传子件）⇒ 落到本件 argv 时
// 与原件同判：未知旗标 ⇒ rc=2。这钉住「两态对齐 = 步命令位带 `--yes`」而**不是**「把 --yes 塞进本件参数面」。
// 哑化（让本件吞下 --yes/--dry-run 当合法旗标）⇒ 下方 rc/err 断言 FAIL（本函数那一行）。
func TestWlkShellConfirmFlagStaysShellOnly(t *testing.T) {
	for _, a := range []string{"--yes", "--dry-run"} {
		rc, out, errb := wlkRun(a)
		want := "用法错：未知旗标 " + a + "（见 --help）\n"
		if rc != 2 || out != "" || errb != want {
			t.Fatalf("%s：rc=%d out=%q err=%q（要 2 / 空 / %q）", a, rc, out, errb, want)
		}
	}
}
