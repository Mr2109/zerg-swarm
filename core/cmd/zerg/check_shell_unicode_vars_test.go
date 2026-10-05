// check_shell_unicode_vars_test.go —— 判据件 `scripts/gates/check-shell-unicode-vars` `.py` 的 Go 真搬
// （`core/cmd/zerg/check_shell_unicode_vars.go`）的**成对负控**测试（✗ 禁恒绿）。
//
// 覆盖（成对：正控 / 负控 / 边界各一格）：
//
//	NoArgs_UsageRc2        · 无位置参 ⇒ rc=2 + 用法正文（stderr）
//	CleanRc0               · `${VAR}` 花括号写法 ⇒ rc=0「命中 0 处」
//	DirtyRc1_Stdout        · `$VAR` 紧跟非 ASCII ⇒ rc=1 + 逐行命中（**钉死列宽 34**）
//	CommentMasked_NoHit    · 整行注释里的 `$VAR中` ⇒ **不许红**（防假红 · py 掩码）
//	MissingFailClosedRc2   · 全缺件 ⇒ **空域 fail-closed rc=2**（stderr `BLOCKED …` · 与 py `require_domain` 逐字同）
//	MixedPresentMissing_SkipsMissing · 真件 + 缺件混 ⇒ 缺件**静默跳过**（rc 随真件命中，不因缺件翻红）
//	OrderPreserved         · 多件 ⇒ 输出行序 = **位置参原序**（非排序）
//	MultipleVarsSorted     · 同件多名 ⇒ 行内名序 = **码点序**
//	JSONRefusedRc2         · `--json` ⇒ rc=2（本门无机器读面 Q-176）
//	SelfTestRc0_AndSelfCleans · `--self-test` ⇒ rc=0 且**临时目录自回收 delta=0**（A 类）
//	DefaultReadOnly_NoWrite· 缺省（无 `--check`/`--write`）⇒ 只读**不写盘**（GAP-20260927-93 · 文件字节不变）
//	FixModeRewritesSingle  · `--write` ⇒ 就地改写（单命中）
//	FixModeMultiHit_RewritesBoth · ★ 多命中 ⇒ **与 py 现读同构**（正向钉住 · GAP-20261001-48 已修）
//	DirFailClosedRc2       · 位置参给目录 ⇒ **空域 fail-closed rc=2**（stderr `BLOCKED …` · 与 py `isfile` 判定同）
//	UnreadableFailClosedRc2· 读不了的件（chmod 000）⇒ **fail-closed rc=2**（C 类）
//	LexerEdges             · `$1` / `$中文` / `${VAR}` 非命中；`$A$B中` ⇒ 只命 `B`
//	JoinFixtureParity      · 坐实 `filepath.Join` ⟷ `os.path.join` 三语义差**不触发**（本件唯一 join 点）
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func csuT(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var o, e bytes.Buffer
	rc := checkShellUnicodeVarsGateMain(&o, &e, args)
	return rc, o.String(), e.String()
}

func csuFixture(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("写夹具失败: %v", err)
	}
	return p
}

// TestShellUnicodeVarsGate —— 成对负控总入口（定向跑：`-run '^TestShellUnicodeVarsGate$'`）。
func TestShellUnicodeVarsGate(t *testing.T) {
	t.Run("NoArgs_UsageRc2", func(t *testing.T) {
		rc, out, errs := csuT(t)
		if rc != 2 {
			t.Fatalf("rc=%d，期望 2", rc)
		}
		if out != "" {
			t.Fatalf("stdout 非空：%q", out)
		}
		if !strings.HasPrefix(errs, "✗ 必须显式传入要扫描的脚本（例如 scripts/*.sh）。\n") {
			t.Fatalf("usage 正文不符：%q", errs)
		}
	})

	t.Run("CleanRc0", func(t *testing.T) {
		d := t.TempDir()
		f := csuFixture(t, d, "clean.sh", "echo \"${VAR}中文\"\n")
		rc, out, errs := csuT(t, "--check", f)
		if rc != 0 || out != "命中 0 处\n" || errs != "" {
			t.Fatalf("rc=%d out=%q err=%q", rc, out, errs)
		}
	})

	t.Run("DirtyRc1_Stdout", func(t *testing.T) {
		d := t.TempDir()
		f := csuFixture(t, d, "dirty.sh", "echo \"$VAR中文\"\n")
		rc, out, errs := csuT(t, "--check", f)
		if rc != 1 {
			t.Fatalf("rc=%d，期望 1", rc)
		}
		if errs != "" {
			t.Fatalf("stderr 非空：%q", errs)
		}
		if out != "  dirty.sh                           修 1 处: VAR\n命中 1 处\n" {
			t.Fatalf("命中文案/列宽不符：%q", out)
		}
	})

	t.Run("CommentMasked_NoHit", func(t *testing.T) {
		d := t.TempDir()
		f := csuFixture(t, d, "c.sh", "# echo \"$VAR中文\"\n#\t$X中文\n")
		rc, out, _ := csuT(t, "--check", f)
		if rc != 0 || out != "命中 0 处\n" {
			t.Fatalf("注释行被误判：rc=%d out=%q", rc, out)
		}
	})

	t.Run("MissingFailClosedRc2", func(t *testing.T) {
		d := t.TempDir()
		rc, out, errs := csuT(t, "--check", filepath.Join(d, "nope.sh"))
		if rc != 2 || out != "" || !strings.Contains(errs, "BLOCKED —— 空域 / 取不到输入：要扫描的脚本件（实参 1 只）") {
			t.Fatalf("全缺件应空域 fail-closed rc=2 + BLOCKED：rc=%d out=%q err=%q", rc, out, errs)
		}
	})

	// ★ 成对（all-missing vs any-missing）：真件 + 缺件混入 ⇒ 缺件**静默跳过**、域非空 ⇒ 不触发空集守卫。
	//   旧 Go 曾「任一缺件 ⇒ rc=2」（比 py 严）⇒ 本用例钉住「按 py 现件：只有**全缺件**才 rc=2」。
	t.Run("MixedPresentMissing_SkipsMissing", func(t *testing.T) {
		d := t.TempDir()
		good := csuFixture(t, d, "good.sh", "echo \"${VAR}中文\"\n")
		rc, out, errs := csuT(t, "--check", good, filepath.Join(d, "nope.sh"))
		if rc != 0 || out != "命中 0 处\n" || errs != "" {
			t.Fatalf("真件+缺件混应跳过缺件、rc=0：rc=%d out=%q err=%q", rc, out, errs)
		}
	})

	t.Run("OrderPreserved", func(t *testing.T) {
		d := t.TempDir()
		a := csuFixture(t, d, "b.sh", "echo \"$A中文\"\n")
		b := csuFixture(t, d, "a.sh", "echo \"$B中文\"\n")
		rc, out, _ := csuT(t, "--check", a, b) // ★ 传 b 再 a ⇒ 输出须 b 先
		if rc != 1 {
			t.Fatalf("rc=%d，期望 1", rc)
		}
		if !strings.HasPrefix(out, "  b.sh ") {
			t.Fatalf("行序被改成排序：%q", out)
		}
		if strings.Index(out, "b.sh") > strings.Index(out, "a.sh") {
			t.Fatalf("行序非位置参原序：%q", out)
		}
	})

	t.Run("MultipleVarsSorted", func(t *testing.T) {
		d := t.TempDir()
		f := csuFixture(t, d, "m.sh", "echo \"$ZZ中$AA中\"\n")
		rc, out, _ := csuT(t, "--check", f)
		if rc != 1 {
			t.Fatalf("rc=%d，期望 1", rc)
		}
		if !strings.Contains(out, "修 2 处: AA, ZZ") {
			t.Fatalf("行内名序非码点序：%q", out)
		}
	})

	t.Run("JSONRefusedRc2", func(t *testing.T) {
		rc, out, errs := csuT(t, "--json", "x.sh")
		if rc != 2 || out != "" {
			t.Fatalf("rc=%d out=%q", rc, out)
		}
		if !strings.Contains(errs, "本门无机器读面") || !strings.Contains(errs, "字段表：无（本门只出人读面）") {
			t.Fatalf("Q-176 正文不符：%q", errs)
		}
	})

	t.Run("SelfTestRc0_AndSelfCleans", func(t *testing.T) {
		t.Setenv("TMPDIR", t.TempDir()) // GAP-20261003-16：私有计数面（只数本用例自建 tmp 目录）
		before := csuCountSelftestDirs(t)
		rc, out, errs := csuT(t, "--self-test")
		after := csuCountSelftestDirs(t)
		if rc != 0 {
			t.Fatalf("rc=%d，期望 0（out=%q err=%q）", rc, out, errs)
		}
		if !strings.HasSuffix(out, "── 自检：全过 ✓\n") {
			t.Fatalf("自检结语不符：%q", out)
		}
		if errs != "" {
			t.Fatalf("stderr 非空：%q", errs)
		}
		if after != before {
			t.Fatalf("自检临时目录未回收：before=%d after=%d（A 类泄漏）", before, after)
		}
	})

	t.Run("FixModeRewritesSingle", func(t *testing.T) {
		d := t.TempDir()
		f := csuFixture(t, d, "fix.sh", "echo \"$VAR中文\"\n")
		rc, out, _ := csuT(t, "--write", f) // ★ --write ⇒ 就地改写（缺省已改只读）
		if rc != 0 {
			t.Fatalf("rc=%d，期望 0", rc)
		}
		if !strings.Contains(out, "合计修复 1 处") {
			t.Fatalf("结语文案不符：%q", out)
		}
		got, _ := os.ReadFile(f)
		if string(got) != "echo \"${VAR}中文\"\n" {
			t.Fatalf("改写结果不符：%q", got)
		}
	})

	// ★★ 正向断言（GAP-20261001-48 修后）：多命中改写档须「按**原串**偏移单趟拼接、一次性落盘」，
	// 本件与 py **现读**输出逐字同构（py 早已修 —— 其 `--self-test` 档④ 断言正确产物 `echo "${A}中${B}文"`）。
	// 输入 `echo "$A中$B中"` ⇒ 两处命中 A、B ⇒ 期望 `echo "${A}中${B}中"`（两处各改一处、无错位残字节）。
	// ── 历史（旧口径 · 已废 · 保留备查）───────────────────────────────────────────
	// 旧用例名 `FixModeMultiHit_SameCorruptionAsPy`，逐字钉死 GAP-20261001-48 **修前**的错位产物：
	//   want := "echo \"${A}\xE4${B}$B\xE4\xB8\xAD\"\n" // ← 旧 py/Go 错位原样（\xE4 = 「中」首字节被切）
	// 其注释曾写「谁修了口子，本用例立刻红」——即**反向**钉住真口子。★ 现读 py 已修 ⇒ 钉错误的肠线随之作废，改钉**正确**产物（Go == py 现读）。
	t.Run("FixModeMultiHit_RewritesBoth", func(t *testing.T) {
		d := t.TempDir()
		f := csuFixture(t, d, "two.sh", "echo \"$A中$B中\"\n")
		rc, _, _ := csuT(t, "--write", f)
		if rc != 0 {
			t.Fatalf("rc=%d，期望 0", rc)
		}
		got, _ := os.ReadFile(f)
		want := "echo \"${A}中${B}中\"\n" // ← py 现读原样（两处各改一处、无错位）
		if string(got) != want {
			t.Fatalf("多命中改写与 py 现读不同构：\n got=%q\nwant=%q", got, want)
		}
	})

	// ★★ 默认档不写盘（GAP-20260927-93）：缺省（无 `--check`/`--write`）⇒ 只读；命中 ⇒ rc=1、「命中」结语、文件**字节不变**。
	t.Run("DefaultReadOnly_NoWrite", func(t *testing.T) {
		d := t.TempDir()
		f := csuFixture(t, d, "ro.sh", "echo \"$VAR中文\"\n")
		before, _ := os.ReadFile(f)
		rc, out, errs := csuT(t, f) // ★ 不给任何档旗 ⇒ py 缺省 = 只读
		if rc != 1 || errs != "" || !strings.Contains(out, "命中 1 处") {
			t.Fatalf("缺省应只读 rc=1 + 命中结语：rc=%d out=%q err=%q", rc, out, errs)
		}
		after, _ := os.ReadFile(f)
		if string(after) != string(before) {
			t.Fatalf("缺省档**写盘了**（应只读）：before=%q after=%q", before, after)
		}
	})

	t.Run("DirFailClosedRc2", func(t *testing.T) {
		d := t.TempDir()
		sub := filepath.Join(d, "adir")
		if err := os.Mkdir(sub, 0o755); err != nil {
			t.Fatalf("建目录失败: %v", err)
		}
		rc, out, errs := csuT(t, "--check", sub)
		if rc != 2 || out != "" || !strings.Contains(errs, "BLOCKED —— 空域 / 取不到输入：要扫描的脚本件（实参 1 只）") {
			t.Fatalf("目录作位置参应空域 fail-closed rc=2 + BLOCKED：rc=%d out=%q err=%q", rc, out, errs)
		}
	})

	t.Run("UnreadableFailClosedRc2", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root 无 EACCES ⇒ 跳过")
		}
		d := t.TempDir()
		f := csuFixture(t, d, "locked.sh", "echo \"$VAR中文\"\n")
		if err := os.Chmod(f, 0o000); err != nil {
			t.Fatalf("chmod 失败: %v", err)
		}
		defer func() { _ = os.Chmod(f, 0o644) }()
		rc, out, errs := csuT(t, "--check", f)
		if rc != 2 || out != "" || !strings.Contains(errs, "读不了") {
			t.Fatalf("不可读件未 fail-closed：rc=%d out=%q err=%q", rc, out, errs)
		}
	})

	t.Run("LexerEdges", func(t *testing.T) {
		d := t.TempDir()
		// 非命中三态：位置参 `$1` / `$中文` / 花括号 `${VAR}`
		f0 := csuFixture(t, d, "no.sh", "echo \"$1中\"\necho \"$中文\"\necho \"${VAR}中\"\n")
		if rc, out, _ := csuT(t, "--check", f0); rc != 0 || out != "命中 0 处\n" {
			t.Fatalf("非命中三态被误判：rc=%d out=%q", rc, out)
		}
		// `$A$B中`：`$A` 后是 ASCII `$` ⇒ 不命；`$B中` ⇒ 只命 B
		f1 := csuFixture(t, d, "ab.sh", "echo \"$A$B中\"\n")
		rc, out, _ := csuT(t, "--check", f1)
		if rc != 1 || !strings.Contains(out, "修 1 处: B") {
			t.Fatalf("leftmost 语义不符：rc=%d out=%q", rc, out)
		}
	})

	// 坐实「本件唯一 join 点的三语义差均不触发」：入参形状恒为 (绝对目录, 普通名)。
	t.Run("JoinFixtureParity", func(t *testing.T) {
		dir := "/tmp/csu-join-x" // 词法绝对目录
		for _, n := range []string{"clean.sh", "dirty.sh"} {
			if got := filepath.Join(dir, n); got != dir+"/"+n {
				t.Fatalf("join(%q,%q)=%q，与 os.path.join 不同构", dir, n, got)
			}
		}
	})
}

func csuCountSelftestDirs(t *testing.T) int {
	t.Helper()
	ents, err := os.ReadDir(os.TempDir())
	if err != nil {
		return 0
	}
	n := 0
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), "shellunicode-selftest-") {
			n++
		}
	}
	return n
}
