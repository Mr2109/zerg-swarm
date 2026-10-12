// cli_gap_sweep_test.go —— `gap idea sweep`（守卫拦截**回收面** · 批2 第三片 · 2-3 守卫腿）的三态牙。
//
// 落点：`package main_test` + `zerg.RunForTest`（跑**当前源码**的行为，不是盘上旧制品）＋ 合成状态目录
// （`t.TempDir()` 当状态目录 · **不碰真 `~/.zerg/state`**）。复用同族 `cli_gap_test.go` 的 `gapRun`。
//
// 口径（缺口 `GAP-20260928-174` 已拍）：**落点定 control 层**（`core/internal/control/guardlog.go`
// 记事件 · 本件走 control 层真记录一条拦截）· **回收面走 `gap idea sweep --source guard`**。
//
// 三例（逐条拄 PASS/FAIL）：
//
//	① **有拦截事件 ⇒ 能回收候选**：走 control 层真记一条拦截 ⇒ `gap idea sweep --source guard --yes`
//	   rc=0 · 池内新增一行 `source=guard` / `status=pending`；再跑一次 ⇒ rc=0 且**不新增**
//	   （同 source 同 fp 幂等 · 设计稿 §3.A3）；`gap idea ls --source guard`（缺口判据栏）能看见它；
//	   `--dry-run` 零副作用。
//	② **没有事件 ⇒ 空且退码正确**：事件件不在盘 ⇒ rc=1（零命中 · 「没有」不是绿）· **不建池件**。
//	③ **非法 `--source` ⇒ 退码正确**（✗ 不静默忽略）：`--source bogus` / `--source gate` ⇒ rc=2 并点名闭集，
//	   且**一个字节不写**（池件不动、事件件不动）。
package main_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/control"
)

// gapSweepGuardRules —— 最小规则表：`read` 放行 · 其余落默认档 `block`（拦截可造）。
const gapSweepGuardRules = `version: 1
rules:
  default: block
  tools:
    - name: "read"
      action: allow
`

// gapSweepStateDir —— 合成状态目录（池件 / 事件件 / 真源都在这里）。
func gapSweepStateDir(t *testing.T) string {
	t.Helper()
	state := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", state)
	t.Setenv("ZERG_GAP_GUARD_INTAKE", "1")
	return state
}

// gapSweepRecordInterception —— 走 **control 层**真记一条拦截事件（落 `control.GuardEventPath()`）。
func gapSweepRecordInterception(t *testing.T, tool string) string {
	t.Helper()
	g, err := control.NewGateFromYAML([]byte(gapSweepGuardRules))
	if err != nil {
		t.Fatalf("造规则表：%v", err)
	}
	g.EnableGuardIntake(control.GuardEventPath())
	if d := g.Check(tool, "", "zerg"); d.Action != control.ActionBlock {
		t.Fatalf("造拦截事件：`%s` 该被拦，实得 %q", tool, d.Action)
	}
	return control.GuardEventPath()
}

// gapSweepPoolRows —— 读池件行（不在 ⇒ 空）。
func gapSweepPoolRows(t *testing.T, state string) []string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(state, "zerg-cli-gap-candidates.jsonl"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("读池件：%v", err)
	}
	var out []string
	for _, ln := range strings.Split(string(b), "\n") {
		if strings.TrimSpace(ln) != "" {
			out = append(out, ln)
		}
	}
	return out
}

// gapSweepFileSHA —— 件快照（不在 ⇒ ""）—— 用来判「一个字节不写」。
func gapSweepFileSHA(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return ""
		}
		t.Fatalf("读 %s：%v", p, err)
	}
	return string(b)
}

// ① 有拦截事件 ⇒ sweep 能出候选（且幂等 · 判据栏可见 · 干跑零副作用）。
func TestGapIdeaSweep_RecoversGuardInterceptions(t *testing.T) {
	state := gapSweepStateDir(t)
	evPath := gapSweepRecordInterception(t, "edit_rules_yaml")

	rc, out, errb := gapRun(t, "gap", "idea", "sweep", "--source", "guard", "--yes")
	if rc != 0 {
		t.Fatalf("① 有事件 ⇒ 该 rc=0，得 %d（out=%q stderr=%q）", rc, out, errb)
	}
	rows := gapSweepPoolRows(t, state)
	if len(rows) != 1 {
		t.Fatalf("① 池内该有 1 行，实得 %d：%v", len(rows), rows)
	}
	if !strings.Contains(rows[0], `"source":"guard"`) || !strings.Contains(rows[0], `"status":"pending"`) {
		t.Fatalf("① 那一行该是 source=guard/status=pending：%s", rows[0])
	}
	if !strings.Contains(rows[0], "守卫拦截") {
		t.Fatalf("① 症状该取自控制层事件（含「守卫拦截」）：%s", rows[0])
	}

	// 幂等：同一条拦截重放 ⇒ rc=0 且不新增（同 source 同 fp）。
	if rc, _, errb := gapRun(t, "gap", "idea", "sweep", "--source", "guard", "--yes"); rc != 0 {
		t.Fatalf("① 重放该 rc=0（幂等档），得 %d（stderr=%q）", rc, errb)
	}
	if rows2 := gapSweepPoolRows(t, state); len(rows2) != 1 {
		t.Fatalf("① 幂等失效：重放后池内 %d 行（要 1）", len(rows2))
	}

	// 判据栏：`gap idea ls --source guard`（缺口 `GAP-20260928-174` 的判据）能看见它。
	if rc, out, errb := gapRun(t, "gap", "idea", "ls", "--source", "guard"); rc != 0 || !strings.Contains(out, "guard") {
		t.Fatalf("① 判据 `gap idea ls --source guard`：rc=%d out=%q stderr=%q", rc, out, errb)
	}

	// `--dry-run` 零副作用（池件 / 事件件逐字节不变）。
	poolBefore := gapSweepFileSHA(t, filepath.Join(state, "zerg-cli-gap-candidates.jsonl"))
	evBefore := gapSweepFileSHA(t, evPath)
	if rc, _, errb := gapRun(t, "gap", "idea", "sweep", "--source", "guard", "--dry-run"); rc != 0 {
		t.Fatalf("① --dry-run 该 rc=0，得 %d（stderr=%q）", rc, errb)
	}
	if got := gapSweepFileSHA(t, filepath.Join(state, "zerg-cli-gap-candidates.jsonl")); got != poolBefore {
		t.Fatal("① --dry-run 动了池件")
	}
	if got := gapSweepFileSHA(t, evPath); got != evBefore {
		t.Fatal("① 回收面动了事件件（该只读）")
	}
}

// ② 没有事件 ⇒ 空且退码正确（rc=1 · 不建池件）。
func TestGapIdeaSweep_NoEventsIsZeroHit(t *testing.T) {
	state := gapSweepStateDir(t)
	if _, err := os.Stat(control.GuardEventPath()); !os.IsNotExist(err) {
		t.Fatalf("② 前置不成立：事件件不该在盘（%v）", err)
	}
	rc, out, errb := gapRun(t, "gap", "idea", "sweep", "--source", "guard", "--yes")
	if rc != 1 {
		t.Fatalf("② 无事件 ⇒ 该 rc=1（零命中），得 %d（out=%q stderr=%q）", rc, out, errb)
	}
	if !strings.Contains(errb, "零命中") {
		t.Fatalf("② 判词要点名零命中：stderr=%q", errb)
	}
	if rows := gapSweepPoolRows(t, state); len(rows) != 0 {
		t.Fatalf("② 无事件不该建池件/落行，实得 %v", rows)
	}
}

// ③ 非法 `--source` ⇒ 退码正确（rc=2 · 点名闭集 · 一个字节不写）。
func TestGapIdeaSweep_BadSourceIsUsage(t *testing.T) {
	state := gapSweepStateDir(t)
	evPath := gapSweepRecordInterception(t, "edit_rules_yaml")
	evBefore := gapSweepFileSHA(t, evPath)

	for _, bad := range []string{"bogus", "gate", "human"} {
		rc, _, errb := gapRun(t, "gap", "idea", "sweep", "--source", bad, "--yes")
		if rc != 2 {
			t.Fatalf("③ `--source %s` ⇒ 该 rc=2（用法错 · 不静默忽略），得 %d（stderr=%q）", bad, rc, errb)
		}
		if !strings.Contains(errb, "不在闭集") {
			t.Fatalf("③ `--source %s` 判词要点名闭集：stderr=%q", bad, errb)
		}
	}
	if rows := gapSweepPoolRows(t, state); len(rows) != 0 {
		t.Fatalf("③ 拒执却动了池件：%v", rows)
	}
	if got := gapSweepFileSHA(t, evPath); got != evBefore {
		t.Fatal("③ 拒执却动了事件件")
	}
}
