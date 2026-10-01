// check_doc_section_numbers_test.go —— Go 真搬的**成对负控**定向单测（M2）。
//
// ✗ 禁恒绿：本件同时断言「必然判绿」「必然判红」「不给结论（rc=2）」三态 —— 任一态被写死都必失败。
// 用例名：TestCheckDocSectionNumbersGatePort（定向：go test -run TestCheckDocSectionNumbersGatePort）。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// cdsnT_put —— 在 t.TempDir() 下写一件，返回绝对路径。
func cdsnT_put(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// cdsnT_run —— 跑入口，返回 (rc, stdout, stderr)。
func cdsnT_run(argv []string) (int, string, string) {
	var so, se bytes.Buffer
	rc := checkDocSectionNumbersGateMain(&so, &se, argv)
	return rc, so.String(), se.String()
}

// cdsnT_fixture —— 一套合成件（正控 + 两条负控）。
func cdsnT_fixture(t *testing.T) (clean, dup, dangling string) {
	t.Helper()
	d := t.TempDir()
	clean = cdsnT_put(t, d, "clean.md", "## "+"附录一"+"\n正文甲\n\n## "+"附录二"+"\n正文乙\n")
	dup = cdsnT_put(t, d, "dup.md", "## "+"附录一"+"\n正文甲\n\n## "+"附录一"+"\n正文乙\n")
	dangling = cdsnT_put(t, d, "dangling.md",
		"## "+"附录一"+"\n正文\n\n见 §"+"附录二"+" 的说法。\n")
	return
}

// TestCheckDocSectionNumbersGatePort —— 定向主用例（正控 / 负控 / 不给结论 / 旗标 / JSON / 自检）。
func TestCheckDocSectionNumbersGatePort(t *testing.T) {
	clean, dup, dangling := cdsnT_fixture(t)

	t.Run("正控_两格皆空_rc0", func(t *testing.T) {
		rc, out, errS := cdsnT_run([]string{clean})
		if rc != 0 {
			t.Fatalf("rc=%d 期望 0（out=%q err=%q）", rc, out, errS)
		}
		if !strings.Contains(out, "⇒ 两格皆空（rc=0）") {
			t.Fatalf("缺终局行：%q", out)
		}
		if !strings.Contains(out, "标题共 2 个") {
			t.Fatalf("标题计数错：%q", out)
		}
	})

	t.Run("负控_标题号重复_rc1", func(t *testing.T) {
		rc, out, _ := cdsnT_run([]string{dup})
		if rc != 1 {
			t.Fatalf("rc=%d 期望 1", rc)
		}
		if !strings.Contains(out, "① 号 \x60一\x60 出现 2 次 ⇒ 行 1, 4") {
			t.Fatalf("重复明细行错：%q", out)
		}
		if !strings.Contains(out, "⇒ 两格有命中（rc=1）") {
			t.Fatalf("缺终局行：%q", out)
		}
	})

	t.Run("负控_引用缺失_rc1", func(t *testing.T) {
		rc, out, _ := cdsnT_run([]string{dangling})
		if rc != 1 {
			t.Fatalf("rc=%d 期望 1", rc)
		}
		if !strings.Contains(out, "② 引用 \x60§附录二\x60（行 4）本件无同名标题") {
			t.Fatalf("悬空引用明细行错：%q", out)
		}
	})

	t.Run("空转非绿_不给件_rc2", func(t *testing.T) {
		rc, out, errS := cdsnT_run(nil)
		if rc != 2 {
			t.Fatalf("rc=%d 期望 2", rc)
		}
		if out != "" {
			t.Fatalf("stdout 应为空：%q", out)
		}
		if errS != "用法错：至少要给一件（rc=2）\n" {
			t.Fatalf("stderr 错：%q", errS)
		}
	})

	t.Run("取不到输入_缺件_rc2", func(t *testing.T) {
		miss := filepath.Join(t.TempDir(), "nope-xyz.md")
		rc, _, errS := cdsnT_run([]string{miss})
		if rc != 2 {
			t.Fatalf("rc=%d 期望 2", rc)
		}
		if !strings.Contains(errS, "BLOCKED —— 读不到：") {
			t.Fatalf("缺 BLOCKED 行：%q", errS)
		}
		if !strings.Contains(errS, "[Errno 2] No such file or directory: '"+miss+"'") {
			t.Fatalf("OSError 串不符：%q", errS)
		}
	})

	t.Run("真口子_读不了的件_判blocked非静默绿", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root 下 chmod 000 仍可读 ⇒ 跳过")
		}
		p := cdsnT_put(t, t.TempDir(), "unread.md", "## "+"附录一"+"\n正文\n")
		if err := os.Chmod(p, 0o000); err != nil {
			t.Fatal(err)
		}
		rc, _, errS := cdsnT_run([]string{p})
		if rc != 2 {
			t.Fatalf("读不了的件 rc=%d 期望 2（现状：记 blocked、✗ 不当绿）", rc)
		}
		if !strings.Contains(errS, "[Errno 13] Permission denied: '"+p+"'") {
			t.Fatalf("OSError 串不符：%q", errS)
		}
	})

	t.Run("未知旗标_rc2_带用法", func(t *testing.T) {
		rc, out, errS := cdsnT_run([]string{"--bogus"})
		if rc != 2 {
			t.Fatalf("rc=%d 期望 2", rc)
		}
		if out != "" {
			t.Fatalf("stdout 应为空：%q", out)
		}
		if !strings.HasPrefix(errS, "usage: check-doc-section-numbers.py [-h] [--json] [files ...]\n") ||
			!strings.Contains(errS, ": error: unrecognized arguments: --bogus\n") {
			t.Fatalf("用法/错误正文不符：%q", errS)
		}
	})

	t.Run("帮助_rc0_正文逐字", func(t *testing.T) {
		rc, out, _ := cdsnT_run([]string{"-h"})
		if rc != 0 {
			t.Fatalf("rc=%d 期望 0", rc)
		}
		const want = "usage: check-doc-section-numbers.py [-h] [--json] [files ...]\n\n" +
			"positional arguments:\n  files\n\noptional arguments:\n" +
			"  -h, --help  show this help message and exit\n  --json\n"
		if out != want {
			t.Fatalf("帮助正文不符：\n%q\nwant\n%q", out, want)
		}
	})

	t.Run("JSON_空数组形态在场", func(t *testing.T) {
		rc, out, _ := cdsnT_run([]string{"--json", clean})
		if rc != 0 {
			t.Fatalf("rc=%d 期望 0", rc)
		}
		for _, want := range []string{
			`"schema": "zerg/v1"`, `"kind": "DocSectionNumbers"`,
			`"duplicate_titles": []`, `"dangling_refs": []`, `"blocked": []`,
		} {
			if !strings.Contains(out, want) {
				t.Fatalf("JSON 缺 %s：%q", want, out)
			}
		}
	})

	t.Run("JSON_有blocked_rc2", func(t *testing.T) {
		miss := filepath.Join(t.TempDir(), "nope-json.md")
		rc, out, _ := cdsnT_run([]string{"--json", dup, miss})
		if rc != 2 {
			t.Fatalf("rc=%d 期望 2", rc)
		}
		if !strings.Contains(out, `"duplicate_titles": [{"num": "一", "lines": [1, 4]}]`) {
			t.Fatalf("results 明细错：%q", out)
		}
		if !strings.Contains(out, `"blocked": [{"file": "`+miss+`", "err": "[Errno 2]`) {
			t.Fatalf("blocked 明细错：%q", out)
		}
	})

	t.Run("自检_rc0_且夹具自回收", func(t *testing.T) {
		base := cdsnTmpBase()
		if base == "" {
			base = os.TempDir()
		}
		before := cdsnT_countPrefix(base, "docsec-selftest-")
		rc, out, _ := cdsnT_run([]string{"--self-test"})
		if rc != 0 {
			t.Fatalf("rc=%d 期望 0（out=%q）", rc, out)
		}
		if !strings.Contains(out, "── 自检：全过 ✓") {
			t.Fatalf("自检终局行错：%q", out)
		}
		after := cdsnT_countPrefix(base, "docsec-selftest-")
		if after != before {
			t.Fatalf("自检夹具未回收：before=%d after=%d", before, after)
		}
	})
}

// cdsnT_countPrefix —— 数 base 下以 prefix 开头的目录数（自回收证）。
func cdsnT_countPrefix(base, prefix string) int {
	ents, err := os.ReadDir(base)
	if err != nil {
		return -1
	}
	n := 0
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), prefix) {
			n++
		}
	}
	return n
}

// TestCdsnPyJoinSemantics —— `posixpath.join` 三类语义差定向单测（✗ 与 filepath.Join 不同）。
func TestCdsnPyJoinSemantics(t *testing.T) {
	cases := []struct {
		in   []string
		want string
	}{
		{[]string{"/tmp/x", "clean.md"}, "/tmp/x/clean.md"},
		{[]string{"a", "../b"}, "a/../b"}, // ① 中间 `..` 段不归一（filepath.Join ⇒ "b"）
		{[]string{"a", "/b"}, "/b"},       // ② 后段绝对 ⇒ 丢弃前面所有段
		{[]string{"a", ""}, "a/"},         // ③ 空段 ⇒ 累积串尾追加一个 `/`（filepath.Join ⇒ "a"）
		{[]string{"a", "", "b"}, "a/b"},
		{[]string{"a/", "b"}, "a/b"},
		{[]string{"", "a"}, "a"}, // out=="" ⇒ 走 +p 分支 ⇒ "a"（同 py；✗ 不是 "/a"）
	}
	for _, c := range cases {
		if got := cdsnPyJoin(c.in...); got != c.want {
			t.Fatalf("pyJoin(%q)=%q 期望 %q", c.in, got, c.want)
		}
	}
}

// TestCdsnPySplitLines —— py `str.splitlines()` 边界（✗ 与 strings.Split(s,"\n") 不同）。
func TestCdsnPySplitLines(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"a", []string{"a"}},
		{"a\n", []string{"a"}},
		{"\n", []string{""}},
		{"a\r\nb", []string{"a", "b"}},
		{"a\vb", []string{"a", "b"}},
		{"a\u2028b", []string{"a", "b"}},
		{"a\n\n", []string{"a", ""}},
	}
	for _, c := range cases {
		got := cdsnPySplitLines(c.in)
		if len(got) != len(c.want) {
			t.Fatalf("splitlines(%q)=%q 期望 %q", c.in, got, c.want)
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Fatalf("splitlines(%q)[%d]=%q 期望 %q", c.in, i, got[i], c.want[i])
			}
		}
	}
}

// TestCdsnUnicodeSpaceParity —— py `\s` 是 Unicode 语义（NBSP U+00A0 认）；夹非 ASCII 空白坐实对齐。
func TestCdsnUnicodeSpaceParity(t *testing.T) {
	d := t.TempDir()
	p := cdsnT_put(t, d, "nbsp.md", "## 一\u00a0标题\n正文\n")
	rc, out, _ := cdsnT_run([]string{p})
	if rc != 0 {
		t.Fatalf("rc=%d 期望 0", rc)
	}
	if !strings.Contains(out, "标题共 1 个") {
		t.Fatalf("NBSP 后标题未被识别（py \\s 语义未对齐）：%q", out)
	}
}
