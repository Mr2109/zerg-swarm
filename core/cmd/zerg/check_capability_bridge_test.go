// check_capability_bridge_test.go —— 定向成对负控单测（✗ 禁恒绿）。
//
// 本件同时断言「必然判绿」「必然判红（含主负控：两套命名且未接桥）」「不给结论」三态 ——
// 任一态被写死都必失败。用例名：TestCapabilityBridgeGatePort。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func cbrTFixture(t *testing.T, sub, gw, mp string, withGW, withMap bool) string {
	t.Helper()
	root := t.TempDir()
	d := filepath.Join(root, sub)
	if err := os.MkdirAll(filepath.Join(d, "core", "internal", "gateway"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(d, "core", "internal", "contract"), 0o755); err != nil {
		t.Fatal(err)
	}
	if withGW {
		if err := os.WriteFile(filepath.Join(d, cbrGWRel), []byte(gw), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if withMap {
		if err := os.WriteFile(filepath.Join(d, cbrMapRel), []byte(mp), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return d
}

func cbrTRun(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := capabilityBridgeGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

func TestCapabilityBridgeGatePort(t *testing.T) {
	// ── 正控（必然判绿）：网关接桥齐 + 桥真源非空 ⇒ rc=0 ────────────────────
	green := cbrTFixture(t, "green", cbrFixtureBridged, cbrFixtureMap2, true, true)
	rcGreen, outGreen, _ := cbrTRun(t, "--root", green)
	if rcGreen != 0 || !strings.Contains(outGreen, "判红 0 项") {
		t.Fatalf("正控失败：接桥齐应 rc=0，得 rc=%d out=%q", rcGreen, outGreen)
	}

	// ── 负控①（主病 · 必然判红）：网关 id 与快照 id 两套命名且**未接桥** ⇒ rc=1 且点名 ──
	bad := cbrTFixture(t, "unbridged", cbrFixtureUnbridged, cbrFixtureMap2, true, true)
	rcBad, outBad, _ := cbrTRun(t, "--root", bad)
	if rcBad != 1 || !strings.Contains(outBad, "桥接线") ||
		!strings.Contains(outBad, "RegistryIDForFleet(") || !strings.Contains(outBad, cbrGWRel) {
		t.Fatalf("负控-未接桥 失败：得 rc=%d out=%q", rcBad, outBad)
	}
	// 成对（✗ 禁恒绿）：绿 ↔ 红 必须不同码
	if rcGreen == rcBad {
		t.Fatalf("成对断言失败：正控 rc=%d 与负控 rc=%d 同码 ⇒ 无鉴别力", rcGreen, rcBad)
	}

	// ── 负控②（必然判红）：桥真源 entries 空 ⇒ rc=1 且点名桥真源 ─────────────
	emptyMap := cbrTFixture(t, "mapempty", cbrFixtureBridged, cbrFixtureMapEmpty, true, true)
	rcEmpty, outEmpty, _ := cbrTRun(t, "--root", emptyMap)
	if rcEmpty != 1 || !strings.Contains(outEmpty, "桥真源") {
		t.Fatalf("负控-桥真源空 失败：得 rc=%d out=%q", rcEmpty, outEmpty)
	}

	// ── 负控③（必然判红）：桥真源不在盘 ⇒ rc=1 ──────────────────────────────
	noMap := cbrTFixture(t, "nomap", cbrFixtureBridged, "", true, false)
	rcNoMap, outNoMap, _ := cbrTRun(t, "--root", noMap)
	if rcNoMap != 1 || !strings.Contains(outNoMap, "不在盘") {
		t.Fatalf("负控-桥真源不在盘 失败：得 rc=%d out=%q", rcNoMap, outNoMap)
	}

	// ── 报告档（负控 · 不进退码）：无 fail-closed 留痕 ⇒ 并列 1 项但 rc 仍 0 ──
	noLog := cbrTFixture(t, "nolog", cbrFixtureBridgedNoFailClosed, cbrFixtureMap2, true, true)
	rcNoLog, outNoLog, _ := cbrTRun(t, "--root", noLog)
	if rcNoLog != 0 || !strings.Contains(outNoLog, "真源读不动仍 fail-closed") {
		t.Fatalf("报告档-无留痕 失败：得 rc=%d out=%q", rcNoLog, outNoLog)
	}

	// ── 不给结论（≠ 绿）：网关来源件不在盘 ⇒ rc=2 ───────────────────────────
	noGW := cbrTFixture(t, "nogw", "", cbrFixtureMap2, false, true)
	rcNoGW, outNoGW, _ := cbrTRun(t, "--root", noGW)
	if rcNoGW != 2 || !strings.Contains(outNoGW, "判不了") {
		t.Fatalf("缺件失败：得 rc=%d out=%q", rcNoGW, outNoGW)
	}

	// ── 未知旗标（用法错）⇒ rc=2 · stderr 明说 ──────────────────────────────
	rcFlag, _, errFlag := cbrTRun(t, "--no-such-flag")
	if rcFlag != 2 || !strings.Contains(errFlag, "unrecognized arguments: --no-such-flag") {
		t.Fatalf("未知旗标失败：得 rc=%d stderr=%q", rcFlag, errFlag)
	}

	// ── 自检面 ⇒ rc=0 且末行可被集中解析器吃下（断言 N 条 · 失败 0 条）────────
	rcST, outST, _ := cbrTRun(t, "--self-test")
	if rcST != 0 || !strings.Contains(outST, "失败 0 条") {
		t.Fatalf("自检面失败：得 rc=%d out=%q", rcST, outST)
	}

	// ── 现仓真跑（必然判绿）：本仓 capability_gate.go 已接真源 ⇒ rc=0 ────────
	repo := filepath.Join("..", "..", "..")
	rcRepo, outRepo, _ := cbrTRun(t, "--root", repo)
	if rcRepo != 0 || !strings.Contains(outRepo, "判红 0 项") {
		t.Fatalf("现仓真跑失败：应 rc=0，得 rc=%d out=%q", rcRepo, outRepo)
	}
}
