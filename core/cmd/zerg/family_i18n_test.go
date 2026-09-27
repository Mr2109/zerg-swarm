// family_i18n_test.go —— `zerg ui i18n` 的薄壳口径：脚本退多少、命令面退多少（§九 M11 `G1-a`）。
//
// 进程内直调 cmdI18n（不引真二进制、不依赖构建）：给合成仓根一个可控退码的 stub 门禁脚本，
// 验「脚本退 1 ⇒ 命令面非 0」「脚本退 0 ⇒ 命令面 0」—— 薄壳不吞门禁失败。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// i18nTestRepo 建最小合成仓根：仓根判据件 + 一个可控退码的 stub 门禁脚本。
func i18nTestRepo(t *testing.T, rc int) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "core", "internal", "version"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "core", "internal", "version", "version.go"),
		[]byte("package version\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	scriptPath := filepath.Join(root, i18nScriptRel)
	if err := os.MkdirAll(filepath.Dir(scriptPath), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "#!/usr/bin/env python3\nimport sys\nprint('i18n 门禁：G1..G4 通过')\nsys.exit(" + strconv.Itoa(rc) + ")\n"
	if err := os.WriteFile(scriptPath, []byte(body), 0o644); err != nil {
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

// TestI18nScriptRc1IsNonZero —— **薄壳不翻译门禁失败**（§九 M11 `G1-a`）：
// 脚本退 1（白名单超差）⇒ 命令面退码非 0（不许吞）。
func TestI18nScriptRc1IsNonZero(t *testing.T) {
	rc := runI18n(t, i18nTestRepo(t, 1))
	if rc == 0 {
		t.Fatalf("脚本退 1 ⇒ 命令面退 0（薄壳把门禁失败吞掉了，违反 `G1-a`）")
	}
}

// TestI18nScriptRc0IsZero —— 脚本退 0 ⇒ 命令面退 0（通过即绿）。
func TestI18nScriptRc0IsZero(t *testing.T) {
	rc := runI18n(t, i18nTestRepo(t, 0))
	if rc != 0 {
		t.Fatalf("脚本退 0 ⇒ 命令面退 %d（要 0）", rc)
	}
}
