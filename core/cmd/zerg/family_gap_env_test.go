// family_gap_env_test.go —— gap 族环境旋钮三档 + 状态目录可视输出的**机检**
// （缺口 `GAP-20260927-75` · 2026-10-10 · `package main_test` · 走 `zerg.RunForTest` 跑当前源码行为）。
//
// 一格正控 + 三条负控（逐条对缺口的「三档」硬线）：
//
//	正控：正常态（`ZERG_STATE_DIR` 指合成状态目录）⇒ stderr 明示「当前状态目录 + 来源令牌」。
//	负控①：夹具专用口被设 ⇒ **无条件清**（值真被 unset）且**清理可见**（点名变量）。
//	负控②：`REPO_ROOT` 类（`ZERG_REPO`）被设成**非仓根** ⇒ 显式申报（报真仓根 + 来源，或逐字报「取不到」），
//	       ✗ 不静默采信继承值。
//	负控③：来源申报缺失那一半 —— 状态目录被毒化到不存在处 ⇒ **如实申报被毒化的目录 + 来源**且
//	       fail-closed（退 8 不给结论），**✗ 不静默回落到别的真源**。
package main_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// gapEnvStateDirSrcEnv —— 来源令牌（闭集值 · 与 `family_gap_env.go` 的 `gapStateDirSrcEnv` 逐字同）。
const gapEnvStateDirSrcEnv = "env:ZERG_STATE_DIR"

// gapEnvLedger 造一具最小真源（一条）写进合成状态目录，令 `gap ls` 不因缺真源退 8。
func gapEnvLedger(t *testing.T, state string) {
	t.Helper()
	line := `{"id":"GAP-20200101-01","fp":"fp-min","symptom":"s","handmade":"h","impact":"命令面",` +
		`"prio":"P1","state":"仍缺","want_family":"gap","want_action":"ls","repro_cmd":"zerg gap ls",` +
		`"verify_cmd":"zerg gap ls","found_at":"2026-01-01T00:00:00+08:00"}`
	if err := os.WriteFile(filepath.Join(state, "zerg-cli-gaps.jsonl"), []byte(line+"\n"), 0o644); err != nil {
		t.Fatalf("造真源失败：%v", err)
	}
}

// 正控：正常态 ⇒ stderr 明示当前状态目录与来源（机读令牌闭集；stdout 一字不动）。
func TestGapEnvTiers_StateDirVisible(t *testing.T) {
	state := t.TempDir()
	gapEnvLedger(t, state)
	t.Setenv("ZERG_STATE_DIR", state)

	rc, out, errb := gapRun(t, "gap", "ls")
	if rc != 0 {
		t.Fatalf("正控：`gap ls` 要 0，得到 %d · stderr=%s", rc, errb)
	}
	if !strings.Contains(errb, "状态目录 = "+state) {
		t.Errorf("正控：stderr 要明示当前状态目录 %q：%q", state, errb)
	}
	if !strings.Contains(errb, "来源："+gapEnvStateDirSrcEnv) {
		t.Errorf("正控：stderr 要明示来源令牌 %q：%q", gapEnvStateDirSrcEnv, errb)
	}
	// stdout 面：本改只加 stderr ⇒ 账那一行照旧（含那条 id）
	if !strings.Contains(out, "GAP-20200101-01") {
		t.Errorf("正控：stdout 应仍出账（id 应在）：%q", out)
	}
}

// 负控①：夹具专用口被设 ⇒ 无条件清（值真被 unset）且清理可见（点名变量）。
func TestGapEnvTiers_FixtureClearedVisible(t *testing.T) {
	state := t.TempDir()
	gapEnvLedger(t, state)
	t.Setenv("ZERG_STATE_DIR", state)
	t.Setenv("ZERG_SVCDECL_DECL", "/tmp/夹具声明件.json") // 夹具专用口被设

	_, _, errb := gapRun(t, "gap", "ls")
	if !strings.Contains(errb, "ZERG_SVCDECL_DECL") {
		t.Errorf("负控①：夹具专用口被设 ⇒ 清理必须可见（逐字点名变量）：%q", errb)
	}
	if _, ok := os.LookupEnv("ZERG_SVCDECL_DECL"); ok {
		t.Errorf("负控①：夹具专用口必须**无条件清**（已 unset）—— 仍在环境里")
	}
}

// 负控②：REPO_ROOT 类（`ZERG_REPO`）被设成非仓根 ⇒ 显式申报（真仓根 + 来源，或逐字「取不到」）。
func TestGapEnvTiers_RepoRootDeclared(t *testing.T) {
	state := t.TempDir()
	gapEnvLedger(t, state)
	t.Setenv("ZERG_STATE_DIR", state)
	t.Setenv("ZERG_REPO", "/tmp/这不是仓根-xyz") // REPO_ROOT 类被毒化

	_, _, errb := gapRun(t, "gap", "ls")
	if !strings.Contains(errb, "② 仓根") {
		t.Fatalf("负控②：REPO_ROOT 类被设 ⇒ 必须显式申报（点名仓根/来源）：%q", errb)
	}
	// 要么给真仓根 + 来源，要么逐字报「取不到」—— 都不许静默采信那个非仓根值
	if !strings.Contains(errb, "取不到") && !strings.Contains(errb, "来源：") {
		t.Errorf("负控②：要申报仓根 + 来源，或明报「取不到」：%q", errb)
	}
}

// 负控③：来源申报缺失那一半 —— 状态目录毒化到不存在处 ⇒ 如实申报 + fail-closed（✗ 不静默换源）。
func TestGapEnvTiers_NoSilentSourceSwap(t *testing.T) {
	t.Setenv("ZERG_STATE_DIR", "/tmp/不存在的状态目录-zzz")

	rc, out, errb := gapRun(t, "gap", "ls")
	if rc != 8 {
		t.Errorf("负控③：毒化状态目录 ⇒ fail-closed 8（不给结论），得到 %d · out=%q", rc, out)
	}
	if !strings.Contains(errb, "状态目录 = /tmp/不存在的状态目录-zzz") {
		t.Errorf("负控③：必须如实申报被毒化的目录（✗ 不许静默换源）：%q", errb)
	}
	if !strings.Contains(errb, "来源："+gapEnvStateDirSrcEnv) {
		t.Errorf("负控③：来源申报必须可见：%q", errb)
	}
}
