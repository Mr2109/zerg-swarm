// excludes_rules_test.go —— `excludes_rules.go`（排除表唯一 Go 解析件）的成对单测。
//
// 覆盖：行口径（跳空行/`#`、rstrip 语义由调用方定）· 命中口径（精确 / 目录前缀以 `/` 收口 /
// ✗ 前缀不误伤 `dir2`）· 读表（正常 / 缺件 ⇒ err，非静默零排除）。
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseExcludesRules(t *testing.T) {
	raw := []string{
		"# 整行注释",
		"",
		"   ",
		"gateway/deploy",
		"  scripts/gates/x.py  ",
		"#tools/ocr/venv",
	}
	got := parseExcludesRules(raw, strings.TrimSpace)
	want := []string{"gateway/deploy", "scripts/gates/x.py"}
	if len(got) != len(want) {
		t.Fatalf("条数 %d ≠ %d：%q", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("第 %d 条 %q ≠ %q", i, got[i], want[i])
		}
	}
	// 空输入 ⇒ 非 nil 空切片（与现读读者可观察行为一致）
	if e := parseExcludesRules(nil, strings.TrimSpace); e == nil || len(e) != 0 {
		t.Fatalf("空输入应得非 nil 空切片，实得 %#v", e)
	}
}

func TestExcludesRulesMatch(t *testing.T) {
	entries := []string{"gateway/deploy", "scripts/gates/x.py", "tools/ocr/venv"}
	cases := []struct {
		rel  string
		want bool
	}{
		{"gateway/deploy", true},          // 精确
		{"gateway/deploy/a.sh", true},     // 目录前缀（以 / 收口）
		{"gateway/deploy2/a.sh", false},   // ★ 前缀须以 / 收口 ⇒ 不误伤 deploy2
		{"scripts/gates/x.py", true},      // 精确（单件）
		{"scripts/gates/x.pyz", false},    // ✗ 单件前缀不命中（非目录）
		{"tools/ocr/venv/lib/x.so", true}, // 目录前缀
		{"elsewhere/y", false},
	}
	for _, c := range cases {
		if got := excludesRulesMatch(c.rel, entries); got != c.want {
			t.Errorf("excludesRulesMatch(%q) = %v，want %v", c.rel, got, c.want)
		}
	}
}

func TestLoadExcludesTable(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "excludes.txt")
	if err := os.WriteFile(p, []byte("# 头\n\ngateway/deploy\nscripts/gates/x.py\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := loadExcludesTable(p)
	if err != nil {
		t.Fatalf("读表 err：%v", err)
	}
	if len(got) != 2 || got[0] != "gateway/deploy" || got[1] != "scripts/gates/x.py" {
		t.Fatalf("读表结果 %q", got)
	}
	// 缺件 ⇒ err（fail-closed · ✗ 不静默零排除）
	miss, err := loadExcludesTable(filepath.Join(dir, "nope.txt"))
	if err == nil {
		t.Fatal("缺件应返回 err（不给结论）")
	}
	if miss == nil || len(miss) != 0 {
		t.Fatalf("缺件应返回非 nil 空切片，实得 %#v", miss)
	}
}
