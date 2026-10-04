// check_version_test.go —— Go 真搬的**成对负控**定向单测（M2-0 · 4.2 附加四条之「负控成对」）。
//
// ✗ 禁恒绿：本件同时断言「必然判绿」「必然判红」「不给结论」三态 —— 任一态被写死都必失败。
// 用例名：TestCheckVersionGatePort（定向：go test -run TestCheckVersionGatePort）。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// verFixture —— 在 t.TempDir() 下造一棵含 `ui/Cargo.toml`（+ 可选 `core/internal/version/version.go`）的夹具树。
func verFixture(t *testing.T, cargoV, goV string, writeGo bool) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "ui"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "ui", "Cargo.toml"),
		[]byte("[package]\nversion = \""+cargoV+"\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if writeGo {
		if err := os.MkdirAll(filepath.Join(root, "core", "internal", "version"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "core", "internal", "version", "version.go"),
			[]byte("package version\n\nconst Version = \""+goV+"\"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func verRun(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := versionGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

func TestCheckVersionGatePort(t *testing.T) {
	// ── 正控（必然判绿）：两侧版本号一致 ⇒ rc=0 ──────────────────────────
	rcGreen, outGreen, _ := verRun(t, verFixture(t, "2.5.13", "2.5.13", true))
	if rcGreen != 0 || !strings.Contains(outGreen, "结果: 通过") {
		t.Fatalf("正控失败：一致应 rc=0，得 rc=%d out=%q", rcGreen, outGreen)
	}

	// ── 负控（必然判红）：两侧版本号不一致 ⇒ rc=1 ────────────────────────
	rcRed, outRed, _ := verRun(t, verFixture(t, "2.5.13", "9.9.9", true))
	if rcRed != 1 || !strings.Contains(outRed, "版本号不一致") {
		t.Fatalf("负控-不一致 失败：得 rc=%d out=%q", rcRed, outRed)
	}
	// 成对（✗ 禁恒绿）：绿 ↔ 红 必须不同码
	if rcGreen == rcRed {
		t.Fatalf("成对断言失败：正控 rc=%d 与负控 rc=%d 同码 ⇒ 无鉴别力", rcGreen, rcRed)
	}

	// ── 空集守卫（GAP-20260927-108）：version.go 缺失 ⇒ 版本真源取不到 ⇒ rc=2（不给结论）──
	rcNoGo, outNoGo, _ := verRun(t, verFixture(t, "2.5.13", "", false))
	if rcNoGo != 2 || !strings.Contains(outNoGo, "不给结论（rc=2）") {
		t.Fatalf("空集守卫-缺 version.go 失败：得 rc=%d out=%q", rcNoGo, outNoGo)
	}

	// ── 空集守卫（GAP-20260927-108）：Cargo.toml 存在但无 version 行 ⇒ 两侧皆空 ⇒ rc=2 ──
	rootNoVer := t.TempDir()
	if err := os.MkdirAll(filepath.Join(rootNoVer, "ui"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rootNoVer, "ui", "Cargo.toml"), []byte("[package]\nname = \"x\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rcNone, outNone, _ := verRun(t, rootNoVer)
	if rcNone != 2 || !strings.Contains(outNone, "= None") {
		t.Fatalf("空集守卫-无 version 行 失败：得 rc=%d out=%q", rcNone, outNone)
	}

	// ── 取不到输入：根路径不存在 ⇒ 两件皆缺 ⇒ rc=2（不给结论 · 与 py 同口径） ──
	rcNoRoot, outNoRoot, _ := verRun(t, filepath.Join(t.TempDir(), "nope"))
	if rcNoRoot != 2 || !strings.Contains(outNoRoot, "= None") {
		t.Fatalf("取不到输入失败：得 rc=%d out=%q", rcNoRoot, outNoRoot)
	}

	// ── 不给结论（≠ 绿）：未知旗标 ⇒ rc=2（stderr 明说） ─────────────────
	rcFlag, _, errFlag := verRun(t, "--no-such-flag")
	if rcFlag != 2 || !strings.Contains(errFlag, "不认的旗标") {
		t.Fatalf("未知旗标失败：得 rc=%d stderr=%q", rcFlag, errFlag)
	}
	// ── `-h` 同为未知旗标档 ⇒ rc=2 ───────────────────────────────────────
	rcH, _, errH := verRun(t, "-h")
	if rcH != 2 || !strings.Contains(errH, "不认的旗标") {
		t.Fatalf("-h 失败：得 rc=%d stderr=%q", rcH, errH)
	}

	// ── --self-test 三档自身必须全过（防恒绿/恒红） ───────────────────────
	rcST, outST, _ := verRun(t, "--self-test")
	if rcST != 0 || !strings.Contains(outST, "自检：全过") {
		t.Fatalf("--self-test 失败：得 rc=%d out=%q", rcST, outST)
	}
}
