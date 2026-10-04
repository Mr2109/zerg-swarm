// check_gotoolchain_test.go —— Go 真搬样板的**成对负控**定向单测（M2-0 · 4.2 附加四条之「负控成对」）。
//
// ✗ 禁恒绿：本件同时断言「必然判绿」「必然判红」「不给结论」三态 —— 任一态被写死都必失败。
// 用例名：TestCheckGotoolchainGatePort（定向：go test -run TestCheckGotoolchainGatePort）。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// gttFixture —— 在 t.TempDir() 下造一棵含 `core/go.mod` 与 `agent/go.mod` 的夹具树（仓外）。
func gttFixture(t *testing.T, coreMod, agentMod string) string {
	t.Helper()
	root := t.TempDir()
	for _, d := range []string{"core", "agent"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "core", "go.mod"), []byte(coreMod), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "agent", "go.mod"), []byte(agentMod), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func gttRun(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := gotoolchainGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

func TestCheckGotoolchainGatePort(t *testing.T) {
	// ── 正控（必然判绿）：两模块同值（core 的 go 指令为 pin） ──────────────
	rcGreen, outGreen, _ := gttRun(t, gttFixture(t,
		"module core\n\ngo 1.22.0\n", "module agent\n\ngo 1.22.0\n"))
	if rcGreen != 0 || !strings.Contains(outGreen, "结果: 通过") {
		t.Fatalf("正控失败：好件应 rc=0，得 rc=%d out=%q", rcGreen, outGreen)
	}

	// ── 负控（必然判红）：agent toolchain 高于 pin ⇒ rc=1 ─────────────────
	rcHigh, outHigh, _ := gttRun(t, gttFixture(t,
		"module core\n\ngo 1.22.0\n",
		"module agent\n\ngo 1.22.0\n\ntoolchain go1.23.0\n"))
	if rcHigh != 1 || !strings.Contains(outHigh, "高于 pin") {
		t.Fatalf("负控-高 pin 失败：得 rc=%d out=%q", rcHigh, outHigh)
	}
	// 成对（✗ 禁恒绿）：绿 ↔ 红 必须不同码
	if rcGreen == rcHigh {
		t.Fatalf("成对断言失败：正控 rc=%d 与负控 rc=%d 同码 ⇒ 无鉴别力", rcGreen, rcHigh)
	}

	// ── 负控 · toolchain 分叉（单一真源被破坏） ───────────────────────────
	rcDiverge, outDiverge, _ := gttRun(t, gttFixture(t,
		"module core\n\ngo 1.22.0\n\ntoolchain go1.22.0\n",
		"module agent\n\ngo 1.23.0\n\ntoolchain go1.23.0\n"))
	if rcDiverge != 1 || !strings.Contains(outDiverge, "单一真源被破坏") {
		t.Fatalf("负控-分叉 失败：得 rc=%d out=%q", rcDiverge, outDiverge)
	}

	// ── 负控 · toolchain 低于自身 go 指令 ─────────────────────────────────
	rcLow, outLow, _ := gttRun(t, gttFixture(t,
		"module core\n\ngo 1.30.0\n\ntoolchain go1.29.0\n",
		"module agent\n\ngo 1.29.0\n\ntoolchain go1.29.0\n"))
	if rcLow != 1 || !strings.Contains(outLow, "低于自身 go 指令") {
		t.Fatalf("负控-低于自身 失败：得 rc=%d out=%q", rcLow, outLow)
	}

	// ── 负控 · core/go.mod 缺失 ───────────────────────────────────────────
	rootMiss := t.TempDir()
	if err := os.MkdirAll(filepath.Join(rootMiss, "agent"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rootMiss, "agent", "go.mod"), []byte("module agent\n\ngo 1.22.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rcMiss, outMiss, _ := gttRun(t, rootMiss)
	if rcMiss != 1 || !strings.Contains(outMiss, "core/go.mod 缺失") {
		t.Fatalf("负控-缺 core 失败：得 rc=%d out=%q", rcMiss, outMiss)
	}

	// ── 负控 · go.mod 无可解析的 go 指令 ──────────────────────────────────
	rcNoGo, outNoGo, _ := gttRun(t, gttFixture(t,
		"module core\n\ngo 1.22.0\n", "module agent\n\n"))
	if rcNoGo != 1 || !strings.Contains(outNoGo, "没有可解析的 go 指令") {
		t.Fatalf("负控-无 go 指令 失败：得 rc=%d out=%q", rcNoGo, outNoGo)
	}

	// ── 空集守卫（GAP-20260927-108）：版本号 trip() 解析不出（`go 1`）⇒ rc=2（不给结论，≠ 绿） ──
	rcUnparse, outUnparse, _ := gttRun(t, gttFixture(t,
		"module core\n\ngo 1\n", "module agent\n\ngo 1\n"))
	if rcUnparse != 2 || !strings.Contains(outUnparse, "不给结论（rc=2）") {
		t.Fatalf("空集守卫-版本号不可解析 失败：得 rc=%d out=%q", rcUnparse, outUnparse)
	}

	// ── 不给结论（≠ 绿）：未知旗标 ⇒ rc=2（stderr 明说） ─────────────────
	rcFlag, _, errFlag := gttRun(t, gttFixture(t,
		"module core\n\ngo 1.22.0\n", "module agent\n\ngo 1.22.0\n"), "--no-such-flag")
	if rcFlag != 2 || !strings.Contains(errFlag, "未知旗标") {
		t.Fatalf("未知旗标失败：得 rc=%d stderr=%q", rcFlag, errFlag)
	}

	// ── Q-176：本门无机器读面 ⇒ --json 拒收 rc=2（stderr 明说） ──────────
	rcJSON, _, errJSON := gttRun(t, "--json")
	if rcJSON != 2 || !strings.Contains(errJSON, "本门无机器读面") {
		t.Fatalf("--json 拒收失败：得 rc=%d stderr=%q", rcJSON, errJSON)
	}

	// ── 取不到输入：读不到的根（不存在）⇒ core/go.mod 缺失 ⇒ rc=1 ───────
	rcNoRoot, outNoRoot, _ := gttRun(t, filepath.Join(t.TempDir(), "nope"))
	if rcNoRoot != 1 || !strings.Contains(outNoRoot, "core/go.mod 缺失") {
		t.Fatalf("取不到输入失败：得 rc=%d out=%q", rcNoRoot, outNoRoot)
	}

	// ── --self-test 三档自身必须全过（防恒绿/恒红） ───────────────────────
	rcST, outST, _ := gttRun(t, "--self-test")
	if rcST != 0 || !strings.Contains(outST, "自检：全过") {
		t.Fatalf("--self-test 失败：得 rc=%d out=%q", rcST, outST)
	}
}
