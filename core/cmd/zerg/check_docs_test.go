// check_docs_test.go —— `check_docs.go`（判据件 `scripts/gates/check_docs.py` 的 Go 真搬）的成对负控测试件。
//
// 纪律（M2）：
//
//	· ✗ 禁恒绿：本件含**必红**用例（正控 rc=1 的负控夹具、`--strict` 抬档、读不了的件 fail-closed）。
//	· 三类 `os.path.join` 语义差各钉一格（空段 / 后段绝对 / 不归一 `..`）；词法面 join/normpath/abspath/relpath 各一格。
//	· `--self-test` 自回收：跑前后**同前缀临时目录 delta == 0**。
//	· ★ 真口子钉子：读不了的 `.md` ⇒ 本件 rc=2（不给结论），日后谁改回「静默/崩溃」即成红。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// docsWrite 在 fixture 根下写一个含给定内容的件（自动建父目录）。
func docsWrite(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// docsRun 用给定 argv 跑入口，返回 (rc, stdout, stderr)。
func docsRun(argv []string) (int, string, string) {
	var so, se bytes.Buffer
	rc := checkDocsGateMain(&so, &se, argv)
	return rc, so.String(), se.String()
}

// ── ① 三类 join 语义差 + 词法面：各钉一格 ───────────────────────────────────────

func TestCheckDocsPyPathSemantics(t *testing.T) {
	t.Run("join_空段语义", func(t *testing.T) {
		cases := []struct {
			in   []string
			want string
		}{
			{[]string{"a", ""}, "a/"},       // py 保留分隔符（✗ filepath.Join 会归一成 "a"）
			{[]string{"", "a"}, "a"},        // 首段空 ⇒ 直接接
			{[]string{"a", "", "b"}, "a/b"}, // 中段空 ⇒ 单斜杠
			{[]string{"/x", ""}, "/x/"},     // 绝对首段 + 空段
		}
		for _, c := range cases {
			if got := cdocPyJoin(c.in...); got != c.want {
				t.Errorf("cdocPyJoin(%q) = %q · 要 %q", c.in, got, c.want)
			}
		}
	})
	t.Run("join_后段绝对丢前段", func(t *testing.T) {
		// ★ 本件唯一运行期可触发面（G3 链接目标）：py 丢弃前段，filepath.Join 会拼错。
		if got := cdocPyJoin("/repo/docs", "/abs/x.md"); got != "/abs/x.md" {
			t.Errorf("join 后段绝对 = %q · 要 /abs/x.md", got)
		}
	})
	t.Run("join_不归一_dotdot", func(t *testing.T) {
		if got := cdocPyJoin("a", "../b"); got != "a/../b" {
			t.Errorf("join 不归一 .. = %q · 要 a/../b", got)
		}
	})
	t.Run("normpath_与斜杠数", func(t *testing.T) {
		cases := []struct{ in, want string }{
			{"", "."},
			{"a/../b", "b"},
			{"/a//b/", "/a/b"},
			{"//a", "//a"}, // 保留开头双斜杠（✗ filepath.Clean 会变 /a）
			{"a/./b", "a/b"},
		}
		for _, c := range cases {
			if got := cdocPyNormpath(c.in); got != c.want {
				t.Errorf("cdocPyNormpath(%q) = %q · 要 %q", c.in, got, c.want)
			}
		}
	})
	t.Run("abspath_纯词法", func(t *testing.T) {
		if got := cdocPyAbspath("/cwd", "rel/x.md"); got != "/cwd/rel/x.md" {
			t.Errorf("abspath 相对 = %q", got)
		}
		if got := cdocPyAbspath("/cwd", "/abs/../y"); got != "/y" {
			t.Errorf("abspath 绝对(不碰盘) = %q · 要 /y", got)
		}
	})
	t.Run("relpath_前缀与相同", func(t *testing.T) {
		if got := cdocPyRelpath("/r/a/b.md", "/r"); got != "a/b.md" {
			t.Errorf("relpath = %q", got)
		}
		if got := cdocPyRelpath("/r", "/r"); got != "." {
			t.Errorf("relpath 相同 = %q · 要 .", got)
		}
		if got := cdocPyRelpath("/other/x.md", "/r"); got != "../other/x.md" {
			t.Errorf("relpath 兄弟 = %q", got)
		}
	})
	t.Run("dirname_剥尾斜杠", func(t *testing.T) {
		cases := []struct{ in, want string }{
			{"/a/b/c.md", "/a/b"},
			{"/a", "/"},
			{"x.md", ""},
			{"/a/b/", "/a/b"},
		}
		for _, c := range cases {
			if got := cdocPyDirname(c.in); got != c.want {
				t.Errorf("cdocPyDirname(%q) = %q · 要 %q", c.in, got, c.want)
			}
		}
	})
}

// ── ② 主面：正控 / 负控（必红） / 空面 / 未知旗标 / --strict ──────────────────────

func TestCheckDocsMainFaces(t *testing.T) {
	t.Run("正控_快照全绿_rc0", func(t *testing.T) {
		root := t.TempDir()
		docsWrite(t, root, "README.zh-CN.md", "# 标题\n\n唯一真相源\n\n[指南](guide.zh-CN.md)\n")
		docsWrite(t, root, "README.md", "# Title\n\nSource of truth\n\n[Guide](guide.en.md)\n")
		docsWrite(t, root, "guide.zh-CN.md", "# 标题\n\n唯一真相源\n")
		docsWrite(t, root, "guide.en.md", "# Title\n\nSource of truth\n")
		rc, out, _ := docsRun([]string{root})
		if rc != 0 {
			t.Fatalf("rc=%d 要 0\n%s", rc, out)
		}
		if !strings.Contains(out, "双语对: 2 对") {
			t.Errorf("双语对数不对:\n%s", out)
		}
		if !strings.Contains(out, "  ✓ 配对完整 / 标题对齐 / 链接有效 / 源声明齐备") {
			t.Errorf("缺全绿行:\n%s", out)
		}
	})
	t.Run("负控_标题层级不一致_rc1", func(t *testing.T) {
		root := t.TempDir()
		docsWrite(t, root, "guide.zh-CN.md", "# 标题\n\n## 小节\n")
		docsWrite(t, root, "guide.en.md", "# Title\n")
		rc, out, _ := docsRun([]string{root})
		if rc != 1 {
			t.Fatalf("rc=%d 要 1\n%s", rc, out)
		}
		if !strings.Contains(out, "[G2] guide.en.md — 标题层级序列不一致：中文 2 个 [1, 2] / 英文 1 个 [1]") {
			t.Errorf("G2 判词不对:\n%s", out)
		}
	})
	t.Run("负控_代码块数不一致_rc1", func(t *testing.T) {
		root := t.TempDir()
		docsWrite(t, root, "g.zh-CN.md", "# A\n\n```\ncode\n```\n")
		docsWrite(t, root, "g.en.md", "# A\n")
		rc, out, _ := docsRun([]string{root})
		if rc != 1 || !strings.Contains(out, "[G2] g.en.md — 代码块数不一致：中文 1 / 英文 0") {
			t.Fatalf("rc=%d\n%s", rc, out)
		}
	})
	t.Run("负控_缺英文版_rc1", func(t *testing.T) {
		root := t.TempDir()
		docsWrite(t, root, "guide.zh-CN.md", "# A\n")
		rc, out, _ := docsRun([]string{root})
		if rc != 1 || !strings.Contains(out, "[G1] guide.zh-CN.md — 缺少对应英文版 guide.en.md") {
			t.Fatalf("rc=%d\n%s", rc, out)
		}
	})
	t.Run("负控_缺中文版_rc1", func(t *testing.T) {
		root := t.TempDir()
		docsWrite(t, root, "guide.en.md", "# A\n")
		rc, out, _ := docsRun([]string{root})
		if rc != 1 || !strings.Contains(out, "[G1] guide.en.md — 缺少对应中文版 guide.zh-CN.md") {
			t.Fatalf("rc=%d\n%s", rc, out)
		}
	})
	t.Run("空面_零件_rc2_空域守卫", func(t *testing.T) {
		root := t.TempDir()
		rc, out, _ := docsRun([]string{root})
		// ★ 空集守卫（GAP-20260927-108）随动：空域（扫面 0 件）从旧「rc=0 + 满绿」改为 fail-closed rc=2
		if rc != 2 || strings.Contains(out, "双语对: 0 对") {
			t.Fatalf("rc=%d\\n%s", rc, out)
		}
	})
	t.Run("未知旗标_rc2_不静默", func(t *testing.T) {
		rc, out, errOut := docsRun([]string{"--no-such-flag"})
		if rc != 2 {
			t.Fatalf("rc=%d 要 2", rc)
		}
		if out != "" {
			t.Errorf("未知旗标不许进 stdout: %q", out)
		}
		if !strings.Contains(errOut, "用法错：本件不认的旗标 --no-such-flag ⇒ rc=2（**不许**静默忽略）") {
			t.Errorf("用法错正文不对: %q", errOut)
		}
	})
	t.Run("strict_仅warn时抬档", func(t *testing.T) {
		root := t.TempDir()
		docsWrite(t, root, "guide.zh-CN.md", "# A\n") // 无「唯一真相源」⇒ G4 warn
		docsWrite(t, root, "guide.en.md", "# A\n")    // 无 "Source of truth" ⇒ G4 warn
		if rc, out, _ := docsRun([]string{root}); rc != 0 {
			t.Fatalf("非 strict 仅 warn ⇒ rc=0，实得 %d\n%s", rc, out)
		}
		if rc, out, _ := docsRun([]string{root, "--strict"}); rc != 1 {
			t.Fatalf("--strict 下 warn ⇒ rc=1，实得 %d\n%s", rc, out)
		}
	})
	t.Run("子目录夹在readdir原序中间_输出行序=sorted", func(t *testing.T) {
		// 非字典序创建 + 子目录不在末尾 ⇒ 保原序读法 + 末尾 sorted 才对齐。
		root := t.TempDir()
		docsWrite(t, root, "zz.zh-CN.md", "# A\n")
		docsWrite(t, root, "zz.en.md", "# A\n")
		docsWrite(t, root, "sub/mm.zh-CN.md", "# A\n")
		docsWrite(t, root, "sub/mm.en.md", "# A\n")
		docsWrite(t, root, "aa.zh-CN.md", "# A\n")
		docsWrite(t, root, "aa.en.md", "# A\n")
		rc, out, _ := docsRun([]string{root})
		if rc != 0 {
			t.Fatalf("rc=%d\n%s", rc, out)
		}
		// G4 warn 行序 = sorted 后的 files 序（aa, sub/mm, zz）⇒ 输出应逐字如此。
		want := "  ! [G4] aa.en.md — 英文版缺少 'Source of truth' 源声明\n" +
			"  ! [G4] aa.zh-CN.md — 中文版缺少 '唯一真相源' 声明\n" +
			"  ! [G4] sub/mm.en.md — 英文版缺少 'Source of truth' 源声明\n" +
			"  ! [G4] sub/mm.zh-CN.md — 中文版缺少 '唯一真相源' 声明\n" +
			"  ! [G4] zz.en.md — 英文版缺少 'Source of truth' 源声明\n" +
			"  ! [G4] zz.zh-CN.md — 中文版缺少 '唯一真相源' 声明\n"
		if !strings.Contains(out, want) {
			t.Errorf("行序（sorted 决定）不符:\n%s", out)
		}
	})
}

// ── ③ 真口子钉子：读不了的件 ⇒ fail-closed rc=2 ─────────────────────────────────

func TestCheckDocsUnreadableFailClosed(t *testing.T) {
	t.Run("读不了的件_rc2_不给结论", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root 无视权限位 ⇒ 本钉子在 root 下不适用")
		}
		root := t.TempDir()
		docsWrite(t, root, "guide.zh-CN.md", "# A\n")
		docsWrite(t, root, "guide.en.md", "# A\n")
		p := filepath.Join(root, "guide.zh-CN.md")
		if err := os.Chmod(p, 0o000); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(p, 0o644) })
		rc, _, errOut := docsRun([]string{root})
		if rc != 2 {
			t.Fatalf("读不了的件 ⇒ 要 rc=2（不给结论），实得 rc=%d · stderr=%q", rc, errOut)
		}
		if !strings.Contains(errOut, "读不了输入件") {
			t.Errorf("fail-closed 应具名报出读不了的件: %q", errOut)
		}
	})
}

// ── ④ `--self-test`：三档全过 + 自回收 delta=0 ───────────────────────────────────

func TestCheckDocsSelfTestDeltaZero(t *testing.T) {
	base := t.TempDir()
	t.Setenv("TMPDIR", base)
	count := func() int {
		ents, err := os.ReadDir(base)
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		for _, e := range ents {
			if strings.HasPrefix(e.Name(), "docs-selftest-") {
				n++
			}
		}
		return n
	}
	before := count()
	rc, out, _ := docsRun([]string{"--self-test"})
	if rc != 0 {
		t.Fatalf("--self-test ⇒ rc=0，实得 %d\n%s", rc, out)
	}
	if !strings.Contains(out, "档① 正控：双语配对 + 标题层级对齐 ⇒ rc=0") ||
		!strings.Contains(out, "档② 负控：英文版少一个标题层级 ⇒ G2 判红 rc=1") ||
		!strings.Contains(out, "档③ 用法错：不认的旗标 ⇒ rc=2（不静默忽略）") ||
		!strings.Contains(out, "── 自检：全过 ✓") {
		t.Errorf("自检正文不全:\n%s", out)
	}
	if after := count(); after != before {
		t.Fatalf("自回收失败：跑前 %d 个 docs-selftest-* / 跑后 %d 个（delta=%d）", before, after, after-before)
	}
}
