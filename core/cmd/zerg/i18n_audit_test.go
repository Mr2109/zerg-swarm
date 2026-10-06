// i18n_audit_test.go —— Go 真搬的**成对负控**定向单测（M2 · 批 · `ui/scripts/i18n_audit.py`）。
//
// 定向：`go test -run TestI18nAudit ./cmd/zerg/ -count=1`（前缀 `TestI18nAudit`，现读本件自带）。
//
// 牙口（负控有牙实证）：把主件的对应实现改动（例：`iaudStripLineComment` 直 `return s`、
// `iaudCut` 改按 byte 切、`iaudMainWithRoot` 的缺件档退 1 代退 2、fail-closed 改静默 rc0）
// ⇒ 本件对应用例 **FAIL 并带行号**（仓外副本哑化实证，见回执）。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// iaudTMkFixture —— 造「同时含 ui/ 与 core/」的仓根夹具（故 iaudRepoRoot 也能认出它）。
func iaudTMkFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, d := range []string{"ui/src", "ui/locales", "core"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "ui", "locales", "zh-CN.yml"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "ui", "locales", "en.yml"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func iaudTWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// ── ① classify 逐字节（含「注释」与「UI文案」两档）──────────────────────────

func TestI18nAuditClassifyByteExact(t *testing.T) {
	root := iaudTMkFixture(t)
	out := t.TempDir()
	t.Setenv("ZERG_I18N_AUDIT_OUT", out)
	iaudTWrite(t, filepath.Join(root, "ui", "src", "a.rs"),
		"/* 中文注释 */\nfn f() {\n    let a = text(\"按钮\");\n}\n")

	var so, se bytes.Buffer
	rc := iaudMainWithRoot(&so, &se, root, []string{"classify"})
	if rc != 0 {
		t.Fatalf("rc=%d want 0 (stderr=%s)", rc, se.String())
	}
	got, err := os.ReadFile(filepath.Join(out, "L2-现状-中文字面量-审计口径.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	want := "文件	行	分类	字面量	该行代码\n" +
		"ui/src/a.rs	1	注释	(comment)	/* 中文注释 */\n" +
		"ui/src/a.rs	3	UI文案	按钮	let a = text(\"按钮\");\n"
	if string(got) != want {
		t.Fatalf("TSV 逐字节不同：\n got=%q\nwant=%q", string(got), want)
	}
}

// ── ② keys 面：md 逐字节 + stdout 关键行（未定义/未使用/动态/多引用）──────────

func TestI18nAuditKeysByteExact(t *testing.T) {
	root := iaudTMkFixture(t)
	out := t.TempDir()
	t.Setenv("ZERG_I18N_AUDIT_OUT", out)
	// zh=2 键，en=1 键（仅 zh 有 unused.key）
	iaudTWrite(t, filepath.Join(root, "ui", "locales", "zh-CN.yml"), "used.rep:\nunused.key:\n")
	iaudTWrite(t, filepath.Join(root, "ui", "locales", "en.yml"), "used.rep:\n")
	iaudTWrite(t, filepath.Join(root, "ui", "src", "m.rs"),
		"// 注释 t!(\"ignored.key\") 不算\n"+
			"fn g() {\n"+
			"    let _ = t!(\"used.rep\");\n"+
			"    let _ = t!(\"used.rep\");\n"+
			"    let _ = t!(\"undef.key\");\n"+
			"    let _ = t!(m.name_key);\n"+
			"    let _ = format!(\"末 t!(\");\n"+
			"}\n")

	var so, se bytes.Buffer
	rc := iaudMainWithRoot(&so, &se, root, []string{"keys"})
	if rc != 0 {
		t.Fatalf("rc=%d want 0 (stderr=%s)", rc, se.String())
	}
	for _, line := range []string{
		"   zh=2 en=1 仅zh=['unused.key'] 仅en=无",
		"   引用未定义: ['undef.key']",
		"   定义未引用(1,注:动态键调用 1 处不可见): ['unused.key']",
		"   动态键调用点: ['ui/src/m.rs:6']",
		"   多次引用的键: 1 个（详见报告）",
	} {
		if !strings.Contains(so.String(), line) {
			t.Fatalf("stdout 缺行 %q\nfull=%s", line, so.String())
		}
	}
	got, err := os.ReadFile(filepath.Join(out, "P0-3-键审计.md"))
	if err != nil {
		t.Fatal(err)
	}
	want := "# P0-3 i18n 键健康度审计\n" +
		"\n" +
		"| 检查 | 结果 |\n" +
		"|---|---|\n" +
		"| zh-CN 键数 | 2 |\n" +
		"| en 键数 | 1 |\n" +
		"| 仅 zh 有 | ['unused.key'] |\n" +
		"| 仅 en 有 | 无 |\n" +
		"| 代码引用但未定义 | ['undef.key'] |\n" +
		"| 定义了但未被引用 | ['unused.key'] |\n" +
		"\n" +
		"## 复用点（疑似一键多义；同一键跨文件/跨语义使用）\n" +
		"\n" +
		"| 键 | 引用次数 | 涉及文件数 | 引用点 |\n" +
		"|---|---|---|---|\n" +
		"| `used.rep` | 2 | 1 | ui/src/m.rs:3<br>ui/src/m.rs:4 |\n" +
		"\n" +
		"## 动态键调用（静态分析不可见）\n" +
		"\n" +
		"`t!(<表达式>)` 形式的调用 1 处——这些键无法被静态扫描确认，因此上方「定义了但未被引用」清单**可能包含被动态引用的键**：\n" +
		"\n" +
		"| 位置 | 代码 |\n" +
		"|---|---|\n" +
		"| `ui/src/m.rs:6` | `let _ = t!(m.name_key);` |\n" +
		"\n" +
		"> 判读：同一键出现在**语义不同**的位置即为一键多义（GPUI 官方明令禁止）。同一文件内相邻行的重复调用通常正常（列表逐行渲染）。\n" +
		""
	if string(got) != want {
		t.Fatalf("MD 逐字节不同：\n got=%q\nwant=%q", string(got), want)
	}
}

// ── ③ py `//.*$` 语义（`$` 匹配尾换行之前，✗ 非 Go RE2 `$`）─────────────────

func TestI18nAuditStripLineCommentSemantics(t *testing.T) {
	cases := []struct{ in, want string }{
		{"// foo\n", "\n"}, // 删 `// foo`，保留尾换行
		{"a//b\n", "a\n"},
		{"a//b", "a"},
		{"x\n", "x\n"}, // 无 `//` 不变
		{"\n", "\n"},
	}
	for _, c := range cases {
		if got := iaudStripLineComment(c.in); got != c.want {
			t.Errorf("iaudStripLineComment(%q)=%q want %q", c.in, got, c.want)
		}
	}
}

// ── ④ py 字符串原语：strip（含 \x1c-\x1f）+ 按码点切片 ──────────────────────

func TestI18nAuditPyStripAndCut(t *testing.T) {
	if got := iaudStrip("\x1c中\x1f\t"); got != "中" {
		t.Errorf("iaudStrip 未剥 \\x1c-\\x1f: got %q", got)
	}
	if got := iaudCut("汉字ab", 2); got != "汉字" {
		t.Errorf("iaudCut 按码点切: got %q want 汉字", got)
	}
	if got := iaudCut("ab", 5); got != "ab" {
		t.Errorf("iaudCut 越界取全: got %q", got)
	}
}

// ── ⑤ 遍历序 + 剔 /target/ + 跳隐藏项 ──────────────────────────────────────

func TestI18nAuditRustFilesOrderSkip(t *testing.T) {
	root := iaudTMkFixture(t)
	iaudTWrite(t, filepath.Join(root, "ui", "src", "zz.rs"), "z")
	iaudTWrite(t, filepath.Join(root, "ui", "src", "aa.rs"), "a")
	iaudTWrite(t, filepath.Join(root, "ui", "src", "target", "t.rs"), "t")
	iaudTWrite(t, filepath.Join(root, "ui", "src", ".hidden.rs"), "h")
	got := iaudRustFiles(root)
	want := []string{filepath.Join(root, "ui", "src", "aa.rs"), filepath.Join(root, "ui", "src", "zz.rs")}
	if len(got) != len(want) {
		t.Fatalf("rust_files=%v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("rust_files[%d]=%q want %q", i, got[i], want[i])
		}
	}
}

// ── ⑥ 未知模式：rc 0，仅头两行，零产出件 ───────────────────────────────────

func TestI18nAuditUnknownModeHeaderOnly(t *testing.T) {
	root := iaudTMkFixture(t)
	out := t.TempDir()
	t.Setenv("ZERG_I18N_AUDIT_OUT", out)
	var so, se bytes.Buffer
	rc := iaudMainWithRoot(&so, &se, root, []string{"bogus"})
	if rc != 0 {
		t.Fatalf("rc=%d want 0", rc)
	}
	want := "repo = " + root + "\noutdir = " + out + "\n"
	if so.String() != want {
		t.Fatalf("stdout=%q want %q", so.String(), want)
	}
	if ents, _ := os.ReadDir(out); len(ents) != 0 {
		t.Fatalf("未知模式不该落件：%v", ents)
	}
}

// ── ⑦ 取源根两处都缺 ⇒ rc=2 不给结论 ──────────────────────────────────────

func TestI18nAuditMissingDocsRootRc2(t *testing.T) {
	parent := t.TempDir() // 夹具父目录受控：无 Zerg-内部文档 兄弟
	root := filepath.Join(parent, "repo")
	for _, d := range []string{"ui/src", "core"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("ZERG_I18N_AUDIT_OUT", "")
	t.Setenv("ZERG_DOCS_ALT", "")
	var so, se bytes.Buffer
	rc := iaudMainWithRoot(&so, &se, root, []string{"all"})
	if rc != 2 {
		t.Fatalf("rc=%d want 2（取不到取源根 ⇒ 不给结论）", rc)
	}
	if !strings.Contains(so.String(), "⇒ 输出目录不可判（不给结论）· rc=2") {
		t.Fatalf("缺 rc=2 声明行: %s", so.String())
	}
}

// ── ⑧ 缺 locales 件 ⇒ fail-closed rc=2（py 侧为回溯 rc=1，声明口径差）───────

func TestI18nAuditMissingLocalesFailClosed(t *testing.T) {
	root := iaudTMkFixture(t)
	if err := os.RemoveAll(filepath.Join(root, "ui", "locales")); err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	t.Setenv("ZERG_I18N_AUDIT_OUT", out)
	var so, se bytes.Buffer
	rc := iaudMainWithRoot(&so, &se, root, []string{"keys"})
	if rc != 2 {
		t.Fatalf("rc=%d want 2（fail-closed）", rc)
	}
	if se.Len() == 0 {
		t.Fatalf("fail-closed 应有 stderr 文案")
	}
}

// ── ⑨ Counter.most_common 序：计数降序 + 同数保首次遇到序 ───────────────────

func TestI18nAuditMostCommonTieOrder(t *testing.T) {
	order := []string{"b", "a", "c"}
	counts := map[string]int{"a": 2, "b": 2, "c": 2}
	got := iaudMostCommon(order, counts, -1)
	var keys []string
	for _, c := range got {
		keys = append(keys, c.key)
	}
	if strings.Join(keys, ",") != "b,a,c" {
		t.Fatalf("most_common 同数应保首次遇到序: got %v", keys)
	}
}
