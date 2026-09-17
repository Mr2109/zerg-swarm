package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGrepHidden — G3: hidden=true 能搜到隐藏文件，默认搜不到
func TestGrepHidden(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("SECRET_KEY=abc123\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.go"), []byte("SECRET_KEY=abc123\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ec := &ExecContext{WorkDir: dir}

	// hidden=false（默认）—— .env 搜不到，config.go 能搜到
	out, err := ec.executeGrep(context.Background(), ".", "SECRET_KEY", map[string]any{"hidden": false}, nil)
	if err != nil {
		t.Fatalf("grep hidden=false: %v", err)
	}
	if !strings.Contains(out, "config.go") {
		t.Errorf("普通文件应搜到——got:\n%s", out)
	}
	if strings.Contains(out, ".env") {
		t.Errorf("hidden=false 时 .env 不应搜到——got:\n%s", out)
	}

	// hidden=true—— .env 能搜到
	outH, err := ec.executeGrep(context.Background(), ".", "SECRET_KEY", map[string]any{"hidden": true}, nil)
	if err != nil {
		t.Fatalf("grep hidden=true: %v", err)
	}
	if !strings.Contains(outH, ".env") {
		t.Errorf("hidden=true 时 .env 应搜到——got:\n%s", outH)
	}
}

// TestGrepKeepIndent — G4：命中行保留行首空白（不 TrimSpace），行号前缀格式仍为「路径:行号: 原文」
// 设计依据：docs/01-设计/设计-grep工具升级-v1.0.1-20260917.md §2.1-3 与边界案例 B3。
// 变异自证：把 exec.go 目录分支的 `fmt.Sprintf("%s:%d: %s", rel, i+1, line)` 改回
// `strings.TrimSpace(line)` 时，本用例必须 FAIL（缩进丢失 ⇒ 正向断言与反向断言同时报错）。
func TestGrepKeepIndent(t *testing.T) {
	dir := t.TempDir()
	// 行 2：行首两空格 · 行 3：行首一个 tab · 行 4：行首「空格+tab」 · 行 5：无缩进（前缀格式对照）
	content := "package main\n" +
		"  foo NEEDLE\n" +
		"	bar NEEDLE\n" +
		" 	baz NEEDLE\n" +
		"qux NEEDLE\n"
	if err := os.WriteFile(filepath.Join(dir, "sample.txt"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	ec := &ExecContext{WorkDir: dir}
	out, err := ec.executeGrep(context.Background(), ".", "NEEDLE", map[string]any{}, nil)
	if err != nil {
		t.Fatalf("grep NEEDLE: %v", err)
	}

	// ① 正向：命中行原样保留行首空白，且前缀就是「路径:行号: 原文」（行号紧贴路径、无补白）
	for _, want := range []string{
		"sample.txt:2:   foo NEEDLE", // ": " 之后仍是原文的两个空格
		"sample.txt:3: 	bar NEEDLE",  // 行首 tab 原样
		"sample.txt:4:  	baz NEEDLE", // ": " 的空格 + 原文的行首空格 + tab
		"sample.txt:5: qux NEEDLE",   // 无缩进行：前缀格式不变的对照锚点
	} {
		if !strings.Contains(out, want) {
			t.Errorf("命中行应保留行首空白、前缀为「路径:行号: 原文」——缺少 %q\ngot:\n%s", want, out)
		}
	}

	// ② 反向（有牙齿）：被 TrimSpace 掉的形态一律不得出现——出现即说明缩进被吃掉
	for _, bad := range []string{
		"sample.txt:2: foo NEEDLE",
		"sample.txt:3: bar NEEDLE",
		"sample.txt:4: baz NEEDLE",
	} {
		if strings.Contains(out, bad) {
			t.Errorf("命中行被 TrimSpace（行首空白丢失）——不应出现 %q\ngot:\n%s", bad, out)
		}
	}

	// ③ 命中条数：目录内仅 1 个文件、4 行命中，不得多出别的内容
	if n := strings.Count(out, "sample.txt:"); n != 4 {
		t.Errorf("命中条数应为 4——got %d:\n%s", n, out)
	}
}

// TestGrepIgnoreCase — G5: ignore_case=true 搜小写能命中大写；ignore_case=false（默认）时不命中
func TestGrepIgnoreCase(t *testing.T) {
	dir := t.TempDir()
	// 文件中只有大写 NEEDLE
	if err := os.WriteFile(filepath.Join(dir, "data.txt"), []byte("hello NEEDLE world\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ec := &ExecContext{WorkDir: dir}

	// ignore_case=true——搜小写 needle 应命中大写 NEEDLE
	outTrue, err := ec.executeGrep(context.Background(), ".", "needle", map[string]any{"ignore_case": true}, nil)
	if err != nil {
		t.Fatalf("grep ignore_case=true: %v", err)
	}
	if !strings.Contains(outTrue, "NEEDLE") {
		t.Errorf("ignore_case=true 时搜小写 needle 应命中大写 NEEDLE——got:\n%s", outTrue)
	}

	// ignore_case=false（默认）——搜小写 needle 不应命中大写 NEEDLE
	outFalse, err := ec.executeGrep(context.Background(), ".", "needle", map[string]any{"ignore_case": false}, nil)
	if err != nil {
		t.Fatalf("grep ignore_case=false: %v", err)
	}
	if strings.Contains(outFalse, "NEEDLE") {
		t.Errorf("ignore_case=false 时搜小写 needle 不应命中大写 NEEDLE——got:\n%s", outFalse)
	}
}
