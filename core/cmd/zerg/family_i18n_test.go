// family_i18n_test.go —— `zerg ui i18n` 的薄壳口径：门禁失败 ⇒ 命令面非 0（§九 M11 `G1-a`）。
//
// ★ S7（2026-10-07）：后端从「`exec python3 ui/scripts/check-i18n.py`」换成 Go 真搬体 `ci18Run`
// （= `checkI18nGateMain` 的实现体）⇒ 本件的夹具随之从「可控退码的 stub 脚本」改成「最小 locale 对」：
// 进程内直调 cmdI18n（不引真二进制、不依赖构建），给合成仓根一对 locale 文件 —— 对称 ⇒ 门禁过（rc=0）；
// 不对称（G1 键失配）⇒ 门禁失败（rc≠0）。**不吞门禁失败** 这条口径一字不动。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// i18nTestRepo 建最小合成仓根：仓根判据件 + 自举脚本占位 + 一对 locale 文件（symmetric 控 G1 是否失配）。
func i18nTestRepo(t *testing.T, symmetric bool) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "core", "internal", "version"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "core", "internal", "version", "version.go"),
		[]byte("package version\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// 自举脚本占位：cmdI18n 的「最小入口不在 ⇒ 退 8」这一支仍在（语义未改宽），故夹具仍备一枚。
	scriptPath := filepath.Join(root, i18nScriptRel)
	if err := os.MkdirAll(filepath.Dir(scriptPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(scriptPath, []byte("#!/usr/bin/env python3\nimport sys\nsys.exit(0)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	locDir := filepath.Join(root, "ui", "locales")
	if err := os.MkdirAll(locDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(locDir, "zh-CN.yml"), []byte("k.a: \"\u4e00\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	en := "k.a: \"x\"\n" // 对称 ⇒ G1 过
	if !symmetric {
		en = "k.b: \"y\"\n" // 不对称 ⇒ G1「仅 zh-CN 有」⇒ 门禁失败
	}
	if err := os.WriteFile(filepath.Join(locDir, "en.yml"), []byte(en), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func runI18n(t *testing.T, root string) int {
	t.Helper()
	t.Setenv("ZERG_REPO", root)
	var out, errb bytes.Buffer
	inv := &invocation{}
	return cmdI18n(inv, &out, &errb)
}

// TestI18nGateFailIsNonZero —— **薄壳不翻译门禁失败**（§九 M11 `G1-a`）：
// locale 键失配（G1）⇒ 命令面退码非 0（不许吞）。
func TestI18nGateFailIsNonZero(t *testing.T) {
	rc := runI18n(t, i18nTestRepo(t, false))
	if rc == 0 {
		t.Fatalf("门禁失败 ⇒ 命令面退 0（薄壳把门禁失败吞掉了，违反 `G1-a`）")
	}
}

// TestI18nGatePassIsZero —— locale 对称 ⇒ 门禁过 ⇒ 命令面退 0（通过即绿）。
func TestI18nGatePassIsZero(t *testing.T) {
	rc := runI18n(t, i18nTestRepo(t, true))
	if rc != 0 {
		t.Fatalf("门禁通过 ⇒ 命令面退 %d（要 0）", rc)
	}
}
