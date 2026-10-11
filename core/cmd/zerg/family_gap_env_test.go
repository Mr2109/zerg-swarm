// family_gap_env_test.go —— gap 族环境旋钮三档 + 状态目录可视输出的**机检**
// （缺口 `GAP-20260927-75` · 2026-10-10 · `package main_test` · 走 `zerg.RunForTest` 跑当前源码行为）。
//
// 一格正控 + 若干负控（逐条对缺口的「三档」硬线）：
//
//	正控：正常态（`ZERG_STATE_DIR` 指合成状态目录）⇒ stderr 明示「当前状态目录 + 来源令牌」。
//	负控①：夹具专用口被设 ⇒ **无条件清**（值真被 unset）且**清理可见**（点名变量）。
//	负控②：`REPO_ROOT` 类（`ZERG_REPO`）被设成**非仓根** ⇒ 显式申报（报真仓根 + 来源，或逐字报「取不到」），
//	       ✗ 不静默采信继承值。
//	负控③：来源申报缺失那一半 —— 状态目录被毒化到不存在处 ⇒ **如实申报被毒化的目录 + 来源**且
//	       fail-closed（退 8 不给结论），**✗ 不静默回落到别的真源**。
//	负控④（本单 b1 · 2026-10-10）：**非默认根来源点名** —— `ZERG_STATE_DIR` 指仓外 ⇒
//	       stderr **响亮申报一行**（点名旋钮 + 当前值）；系统临时根（`TMPDIR`）**不进名单**。
//	       哑化 ④ 分支（去掉那段）⇒ 负控④ 真红。
//	负控⑤（本单 b1 全局推广 · 2026-10-10）：**非 gap 族**命令（`version`）在非默认根态下也由
//	       `main.go` 咽喉点响亮申报 ④；默认态**一字不加**。哑化咽喉点的非 gap 支 ⇒ 负控⑤ 真红。
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

// 负控④（本单 b1）：非默认根来源点名 —— `ZERG_STATE_DIR` 指**仓外**目录 ⇒ stderr 响亮申报一行
// （点名旋钮名 + 当前值）；且**系统临时根**（`TMPDIR` = `/var/folders/…` 一族）**不进名单**。
// ★ 哑化 `family_gap_env.go` 的 ④ 分支（去掉「非默认根来源点名」那一段）⇒ 本用例**真红**。
func TestGapEnvTiers_NonDefaultRootNamedLoudly(t *testing.T) {
	outside := t.TempDir() // 仓外目录 = 非默认状态根
	gapEnvLedger(t, outside)
	t.Setenv("ZERG_STATE_DIR", outside)

	_, _, errb := gapRun(t, "gap", "ls")
	if !strings.Contains(errb, "非默认根来源点名") {
		t.Fatalf("负控④：非默认根 ⇒ 必须响亮申报一行（点名「非默认根来源点名」）：%q", errb)
	}
	if !strings.Contains(errb, "ZERG_STATE_DIR="+outside) {
		t.Errorf("负控④：响亮申报名单要点名该旋钮的当前值 %q：%q", outside, errb)
	}
	// 系统临时根（本机 `TMPDIR` = `/var/folders/…/T/`）⇒ 不算非默认 ⇒ **不进名单**。
	if strings.Contains(errb, "TMPDIR=") {
		t.Errorf("负控④：TMPDIR 走系统临时根 ⇒ 不该进非默认根名单：%q", errb)
	}
}

// 负控⑤（本单 b1 · 全局推广）：**非 gap 族**命令（`version`）在非默认根态下也由 `main.go` 咽喉点
// 响亮申报 ④（点名旋钮名 + 当前值）；**默认态一字不加**（既有默认输出逐字不动）。
// ★ 哑化 `main.go` 咽喉点的 **非 gap 支**（去掉那半 `gapNonDefaultRootLine()` 调用）⇒ 本用例**真红**。
func TestGapEnvTiers_GlobalNonDefaultRootNamedLoudly(t *testing.T) {
	// 非默认根态：`ZERG_STATE_DIR` 指**仓外**目录 ⇒ 非 gap 族命令的 stderr 也必须响亮申报一行。
	outside := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", outside)

	rc, _, errb := gapRun(t, "version", "--json", "name")
	if rc != 0 {
		t.Fatalf("全局 ④：非 gap 族（version）非默认根态应仍 rc=0，得到 %d · stderr=%q", rc, errb)
	}
	if !strings.Contains(errb, "非默认根来源点名") {
		t.Fatalf("全局 ④ 破：非 gap 族命令没在咽喉点响亮申报：%q", errb)
	}
	if !strings.Contains(errb, "ZERG_STATE_DIR="+outside) {
		t.Errorf("全局 ④ 破：点名单要点名该旋钮的当前值 %q：%q", outside, errb)
	}
	// 系统临时根（本机 `TMPDIR` = `/var/folders/…/T/`）⇒ 不算非默认 ⇒ **不进名单**。
	if strings.Contains(errb, "TMPDIR=") {
		t.Errorf("全局 ④ 破：TMPDIR 走系统临时根 ⇒ 不该进名单：%q", errb)
	}

	// 默认态：显式设空（两项都不指非默认根）⇒ **一字不加**（既有默认输出逐字不动）。
	t.Setenv("ZERG_STATE_DIR", "")
	t.Setenv("ZERG_REPO", "")
	_, _, errb2 := gapRun(t, "version", "--json", "name")
	if strings.Contains(errb2, "非默认根来源点名") {
		t.Errorf("全局 ④ 破：默认态不该申报：%q", errb2)
	}
}
