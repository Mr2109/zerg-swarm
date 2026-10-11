// check_symbol_delete_ripple_test.go —— Go 真搬的**成对负控**定向单测（M2 · 负控成对）。
//
// ✗ 禁恒绿：同时断言「必然判绿」「必然判红」「不给结论」三态 —— 任一态被写死都必失败。
// 用例名：TestCheckSymbolDeleteRippleGatePort（定向：go test -run TestCheckSymbolDeleteRippleGatePort）。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// sdrFixturePlan —— 仓外夹具：把一份干跑单落进 t.TempDir()，回其路径。
func sdrFixturePlan(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "plan.txt")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func sdrRun(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := symbolDeleteRippleGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

// sdrPlanBody —— 与 py 夹具同形的干跑单正文（一行一事实）。
func sdrPlanBody(tier, cls string) string {
	return "something\n" +
		"impact_hook tier=" + tier + " class=" + cls +
		" retrievable=yes tracked=yes archive=Zerg-归档/x git_sha=deadbeef outside=no rel=a/b.go\n" +
		"impact_criteria untracked=no no_archive_copy=no outside_repo=no criteria_hits=0\n"
}

func TestCheckSymbolDeleteRippleGatePort(t *testing.T) {
	// ── 正控（必然判绿）：改 ⇒ 只记 ⇒ rc=0 ────────────────────────────────
	rcGreen, outGreen, _ := sdrRun(t, "--plan", sdrFixturePlan(t, sdrPlanBody("只记", "改")))
	if rcGreen != 0 || !strings.Contains(outGreen, "全绿") {
		t.Fatalf("正控失败：改⇒只记 应 rc=0，得 rc=%d out=%q", rcGreen, outGreen)
	}

	// ── 负控（必然判红）：符号级删 ⇒ 必拦 ⇒ rc=1 ───────────────────────────
	rcRed, outRed, _ := sdrRun(t, "--plan", sdrFixturePlan(t, sdrPlanBody("必拦", "符号级删")))
	if rcRed != 1 || !strings.Contains(outRed, "必拦") {
		t.Fatalf("负控-符号级删 失败：得 rc=%d out=%q", rcRed, outRed)
	}
	// 成对（✗ 禁恒绿）：绿 ↔ 红 必须不同码
	if rcGreen == rcRed {
		t.Fatalf("成对断言失败：正控 rc=%d 与负控 rc=%d 同码 ⇒ 无鉴别力", rcGreen, rcRed)
	}

	// ── 负控：件级 + 可找回 ⇒ 只报 ⇒ rc=0（不许红） ───────────────────────
	rcReport, outReport, _ := sdrRun(t, "--plan", sdrFixturePlan(t, sdrPlanBody("只报", "件级（整件）")))
	if rcReport != 0 || !strings.Contains(outReport, "只报") {
		t.Fatalf("件级可找回 失败：得 rc=%d out=%q", rcReport, outReport)
	}

	// ── 负控：件级 + 无归档副本 ⇒ 升必拦 ⇒ rc=1 ──────────────────────────
	body := "impact_hook tier=必拦 class=件级（整件） retrievable=yes tracked=yes archive=Zerg-归档/x" +
		" git_sha=deadbeef outside=no rel=a/b.go\n" +
		"impact_criteria untracked=no no_archive_copy=yes outside_repo=no criteria_hits=1\n"
	rcNoArch, outNoArch, _ := sdrRun(t, "--plan", sdrFixturePlan(t, body))
	if rcNoArch != 1 || !strings.Contains(outNoArch, "不可找回") {
		t.Fatalf("负控-无归档副本 失败：得 rc=%d out=%q", rcNoArch, outNoArch)
	}

	// ── 不给结论（≠ 绿）：`class` 表外 ⇒ rc=2 ─────────────────────────────
	rcOOD, _, _ := sdrRun(t, "--plan", sdrFixturePlan(t, sdrPlanBody("必拦", "重命名")))
	if rcOOD != 2 {
		t.Fatalf("表外组合 失败：应 rc=2，得 rc=%d", rcOOD)
	}

	// ── 不给结论（≠ 绿）：0 件（空转）⇒ rc=2 ─────────────────────────────
	rcEmpty, outEmpty, _ := sdrRun(t, "--plan", sdrFixturePlan(t, "没有单\n"))
	if rcEmpty != 2 || !strings.Contains(outEmpty, "0 件") {
		t.Fatalf("空转 失败：得 rc=%d out=%q", rcEmpty, outEmpty)
	}

	// ── 取不到输入：显式单读不到 ⇒ rc=2（缺件不许当绿） ───────────────────
	rcMiss, outMiss, _ := sdrRun(t, "--plan", filepath.Join(t.TempDir(), "nope.txt"))
	if rcMiss != 2 || !strings.Contains(outMiss, "读不到") {
		t.Fatalf("取不到输入 失败：得 rc=%d out=%q", rcMiss, outMiss)
	}

	// ── Q-176：本门无机器读面 ⇒ --json 拒收 rc=2（stderr 明说） ───────────
	rcJSON, _, errJSON := sdrRun(t, "--json")
	if rcJSON != 2 || !strings.Contains(errJSON, "本门无机器读面") {
		t.Fatalf("--json 拒收 失败：得 rc=%d stderr=%q", rcJSON, errJSON)
	}

	// ── --list-rules ⇒ rc=0 ──────────────────────────────────────────────
	rcRules, outRules, _ := sdrRun(t, "--list-rules")
	if rcRules != 0 || !strings.Contains(outRules, "删前波纹门") {
		t.Fatalf("--list-rules 失败：得 rc=%d out=%q", rcRules, outRules)
	}

	// ── --self-test 十三格自身必须全过（防恒绿/恒红） ───────────────────────
	rcST, outST, _ := sdrRun(t, "--self-test")
	if rcST != 0 || !strings.Contains(outST, "自检结论: 全过") {
		t.Fatalf("--self-test 失败：得 rc=%d out=%q", rcST, outST)
	}
}
