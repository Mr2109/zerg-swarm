// check_registry_vs_disk_test.go —— Go 真搬的**成对负控**定向单测（M2-0 · 4.2 附加四条之「负控成对」）。
//
// ✗ 禁恒绿：本件同时断言「必然判绿」「必然判红」「不给结论（空转 / 缺件 / 取不到输入）」「机器面」
// 四态 —— 任一态被写死（恒 0 / 恒 2）都必失败。
// 用例名：TestCheckRegistryVsDiskGatePort（定向：go test -run TestCheckRegistryVsDiskGatePort）。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const crsTestWhitelist = `# 合成白名单（自检夹具 · 不是真仓件）
# 说明：这些在脚本 EXCLUDES 中以模式表达 —— ★ **本节只解释 EXCLUDES 的模式**（不是「本节之后的行都排除」）
#   gateway/fleet.yaml  部署痕迹
#   sub/drop.txt  夹具排除件
sub/
gateway/
keep.txt
# gone/ ← ★ 空指：盘上无此目录（夹具）
`

const crsTestPublishSh = `#!/usr/bin/env bash
# 合成 EXCLUDES（自检夹具）
EXCLUDES=(
  "gateway/fleet.yaml"
  "sub/drop.txt"
)
`

// crsTestFixture —— 在 t.TempDir() 下造一棵合成夹具树（仓外）。mutate ∈ {"dangling","uncovered","stale"}。
func crsTestFixture(t *testing.T, mutate string, dropWL bool) string {
	t.Helper()
	root := t.TempDir()
	for _, d := range []string{"publish", "scripts/build", "sub", "gateway"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write := func(rel, body string) {
		if err := os.WriteFile(filepath.Join(root, rel), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("keep.txt", "keep\n")
	write("sub/drop.txt", "drop\n")
	write("gateway/fleet.yaml", "fleet\n")
	write("publish/whitelist.txt", crsTestWhitelist)
	write("scripts/build/publish-public.sh", crsTestPublishSh)
	if mutate == "dangling" {
		write("scripts/build/publish-public.sh",
			strings.Replace(crsTestPublishSh, "\n)", "\n  \"never-existed/\"\n)", 1))
	}
	if mutate == "uncovered" {
		write("loose.txt", "loose\n")
		write("scripts/build/publish-public.sh",
			strings.Replace(crsTestPublishSh, "\n)", "\n  \"loose.txt\"\n)", 1))
	}
	if mutate == "stale" {
		if err := os.MkdirAll(filepath.Join(root, "gone"), 0o755); err != nil {
			t.Fatal(err)
		}
		write("gone/x.txt", "x\n")
	}
	if dropWL {
		if err := os.Remove(filepath.Join(root, "publish", "whitelist.txt")); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func crsTestRun(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := registryVsDiskGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

func TestCheckRegistryVsDiskGatePort(t *testing.T) {
	// ── 正控（必然判绿）：干净夹具（报告档）⇒ rc=0 ─────────────────────────
	clean := crsTestFixture(t, "", false)
	rcGreen, outGreen, _ := crsTestRun(t, "--repo", clean)
	if rcGreen != 0 || !strings.Contains(outGreen, "三格全 0（rc=0）") {
		t.Fatalf("正控失败：好件应 rc=0 且三格全 0，得 rc=%d out=%q", rcGreen, outGreen)
	}

	// ── 负控（必然判红）：造一空指行 + --strict ⇒ rc=1 ────────────────────
	dang := crsTestFixture(t, "dangling", false)
	rcRed, outRed, _ := crsTestRun(t, "--repo", dang, "--strict")
	if rcRed != 1 || !strings.Contains(outRed, "阻断档") || !strings.Contains(outRed, "空指") {
		t.Fatalf("负控-空指 失败：得 rc=%d out=%q", rcRed, outRed)
	}
	// 成对（✗ 禁恒绿）：绿 ↔ 红 必须不同码
	if rcGreen == rcRed {
		t.Fatalf("成对断言失败：正控 rc=%d 与负控 rc=%d 同码 ⇒ 无鉴别力", rcGreen, rcRed)
	}
	// 同一件、报告档 ⇒ 只报不拦 ⇒ 必须回 0（「阻断档才有 1」这一格有牙）
	rcReport, outReport, _ := crsTestRun(t, "--repo", dang)
	if rcReport != 0 || !strings.Contains(outReport, "只报不拦") {
		t.Fatalf("报告档失败：同一件不给 --strict 应 rc=0「只报不拦」，得 rc=%d out=%q", rcReport, outReport)
	}
	if rcReport == rcRed {
		t.Fatalf("成对断言失败：报告档 rc=%d 与阻断档 rc=%d 同码 ⇒ --strict 无牙", rcReport, rcRed)
	}

	// ── 负控 · 悬空（EXCLUDES 有物、不在导出面）⇒ --strict rc=1 ─────────────
	unc := crsTestFixture(t, "uncovered", false)
	rcUnc, outUnc, _ := crsTestRun(t, "--repo", unc, "--strict")
	if rcUnc != 1 || !strings.Contains(outUnc, "悬空") {
		t.Fatalf("负控-悬空 失败：得 rc=%d out=%q", rcUnc, outUnc)
	}

	// ── 负控 · C1 注释说「盘上无」而盘上有 ⇒ --strict rc=1 ─────────────────
	stale := crsTestFixture(t, "stale", false)
	rcStale, outStale, _ := crsTestRun(t, "--repo", stale, "--strict")
	if rcStale != 1 || !strings.Contains(outStale, "文案与实际不符") || !strings.Contains(outStale, "C1") {
		t.Fatalf("负控-C1 失败：得 rc=%d out=%q", rcStale, outStale)
	}

	// ── 不给结论（空转 ≠ 绿）：白名单 0 条活跃条目 ⇒ rc=2 ──────────────────
	empty := t.TempDir()
	for _, d := range []string{"publish", "scripts/build"} {
		if err := os.MkdirAll(filepath.Join(empty, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	_ = os.WriteFile(filepath.Join(empty, "publish", "whitelist.txt"), []byte("# 全注释\n\n"), 0o644)
	_ = os.WriteFile(filepath.Join(empty, "scripts", "build", "publish-public.sh"), []byte(crsTestPublishSh), 0o644)
	rcVac, outVac, _ := crsTestRun(t, "--repo", empty)
	if rcVac != 2 || !strings.Contains(outVac, "空转") {
		t.Fatalf("空转失败（空转必须 ≠ 绿）：得 rc=%d out=%q", rcVac, outVac)
	}
	if rcVac == rcGreen {
		t.Fatalf("成对断言失败：空转 rc=%d 与正控 rc=%d 同码", rcVac, rcGreen)
	}

	// ── 不给结论（缺件）：白名单不在 ⇒ rc=2 ───────────────────────────────
	noWL := crsTestFixture(t, "", true)
	rcNoWL, outNoWL, _ := crsTestRun(t, "--repo", noWL)
	if rcNoWL != 2 || !strings.Contains(outNoWL, "白名单不在盘") {
		t.Fatalf("缺件失败：得 rc=%d out=%q", rcNoWL, outNoWL)
	}

	// ── 不给结论（取不到输入）：--repo 指向不存在 ⇒ rc=2 ───────────────────
	rcNoRepo, outNoRepo, _ := crsTestRun(t, "--repo", filepath.Join(t.TempDir(), "nope"))
	if rcNoRepo != 2 || !strings.Contains(outNoRepo, "不给结论") {
		t.Fatalf("取不到输入失败：得 rc=%d out=%q", rcNoRepo, outNoRepo)
	}

	// ── 机器面（合法字段集）⇒ rc=0 ＋ 一行 JSON（与 py ensure_ascii=False 同形）──
	rcJSON, outJSON, _ := crsTestRun(t, "--repo", clean, "--json", "schema,whitelist_entries,excludes_entries")
	if rcJSON != 0 || !strings.Contains(outJSON,
		`{"schema": "registry-vs-disk/1", "whitelist_entries": 3, "excludes_entries": 2}`) {
		t.Fatalf("机器面失败：得 rc=%d out=%q", rcJSON, outJSON)
	}

	// ── 机器面（表外字段名）⇒ 用法错 rc=2 ＋ stderr 明说（≠「零命中」）──────
	rcBadJSON, _, errBadJSON := crsTestRun(t, "--repo", clean, "--json", "zzz")
	if rcBadJSON != 2 || !strings.Contains(errBadJSON, "不认的字段名") {
		t.Fatalf("表外字段失败：得 rc=%d stderr=%q", rcBadJSON, errBadJSON)
	}

	// ── `--self-test` 自身必须全过（防恒绿/恒红）──────────────────────────
	rcST, outST, _ := crsTestRun(t, "--self-test")
	if rcST != 0 || !strings.Contains(outST, "自检结论: 全过") {
		t.Fatalf("--self-test 失败：得 rc=%d out=%q", rcST, outST)
	}
}
