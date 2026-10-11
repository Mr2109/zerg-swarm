// cli_gap_two_state_testface_test.go —— 缺口「销案两态放宽」的机检：
// `gap set-state` 的 `--before-read`/`--after-read` 允许「**测试件面**」读数 —— 真修只由测试件
// （`go test …` / `python3 <件>.py …`）承载、CLI 行为一字不变时，`cmd=` 可用测试命令，但 `reading=`
// **必须**注明这是等价读数（含 `等价读数`）⇒ 否则仍按「首词必须是 `zerg`」拒收。
//
// 四格（全合成盘面 · `t.TempDir()` 当状态目录 ⇒ 不碰真 `~/.zerg/state`）：
//
//	① 正控：测试件面 `cmd=` + 等价读数标注 ⇒ 0 且真改态、`solved_evidence` 落测试件面三格
//	② 负控①：测试件面 `cmd=` **缺**等价读数标注 ⇒ 2 · 真源 sha 与审计零动 · 仍 仍缺
//	③ 负控②：非 `zerg`、非测试件面命令（`ls -l`）⇒ 2 · 零动
//	④ 旁证：`zerg` 正门命令（无需标注）⇒ 0（回归 · 行为一字不动）
package main_test

import (
	"fmt"
	"strings"
	"testing"
)

func TestGapTwoStateTestFace(t *testing.T) {
	_, ledger, audit := gapEnv(t)

	// 两条靶子（走同族写面 `add` 造 · id 由 add 生成 ⇒ **新账** · 销案须带两态）。
	for _, sym := range []string{"测试件面靶子甲", "测试件面靶子乙"} {
		if rc, _, errb := gapRun(t, append(gapAddArgv("手搓："+sym, sym), "--yes")...); rc != 0 {
			t.Fatalf("造靶子（%s）失败：rc=%d stderr=%q", sym, rc, errb)
		}
	}
	ids := gapAllIDs(t, ledger)
	if len(ids) != 2 {
		t.Fatalf("靶子条数不对（要 2）：%v", ids)
	}
	id1, id2 := ids[0], ids[1]

	const mark = "等价读数"

	// ① 正控：测试件面 `cmd=` + 等价读数标注 ⇒ 0 且真改态。
	brPos := "cmd=go test -run TestTwoState ./cmd/zerg/ · rc=1 · reading=测试件面" + mark + " 改前 FAIL"
	arPos := "cmd=go test -run TestTwoState ./cmd/zerg/ · rc=0 · reading=测试件面" + mark + " 改后 PASS"
	if rc, _, errb := gapRun(t, "gap", "set-state", id1, "--state", "已解", "--solved-kind", "销案",
		"--before-read", brPos, "--after-read", arPos, "--evidence", "测试件面正控", "--yes"); rc != 0 {
		t.Fatalf("正控（测试件面 cmd + 等价读数标注）⇒ 0，得到 %d · %q", rc, errb)
	}
	if r := gapRecOf(t, ledger, id1); r["state"] != "已解" {
		t.Errorf("正控：真跑后 state 要 已解，得到 %v", r["state"])
	} else if ev := fmt.Sprint(r["solved_evidence"]); !strings.Contains(ev, "cmd=go test -run TestTwoState") ||
		!strings.Contains(ev, mark) {
		t.Errorf("正控：测试件面三格没落进 solved_evidence：%q", ev)
	}

	// ② 负控①：测试件面 cmd 但 reading 缺等价读数标注 ⇒ 2 · 零动 · 仍 仍缺。
	sha0 := gapSHAOf(t, ledger)
	na0 := len(gapAuditLines(t, audit))
	brNo := "cmd=go test -run TestTwoState ./cmd/zerg/ · rc=1 · reading=改前 FAIL"
	arNo := "cmd=go test -run TestTwoState ./cmd/zerg/ · rc=0 · reading=改后 PASS"
	if rc, _, errb := gapRun(t, "gap", "set-state", id2, "--state", "已解", "--solved-kind", "销案",
		"--before-read", brNo, "--after-read", arNo, "--evidence", "缺标注", "--yes"); rc != 2 {
		t.Errorf("负控①（测试件面 cmd 缺标注）⇒ 2，得到 %d · %q", rc, errb)
	}
	if gapSHAOf(t, ledger) != sha0 || len(gapAuditLines(t, audit)) != na0 {
		t.Errorf("负控①：拒写却动了盘（真源 sha 或审计行数变了）")
	}
	if r := gapRecOf(t, ledger, id2); r["state"] != "仍缺" {
		t.Errorf("负控①：state 要仍 仍缺，得到 %v", r["state"])
	}

	// ③ 负控②：非 `zerg`、非测试件面命令（`ls -l`）—— 带标注也拒 ⇒ 2 · 零动。
	brBad := "cmd=ls -l · rc=1 · reading=测试件面" + mark + " 改前 FAIL"
	arBad := "cmd=ls -l · rc=0 · reading=测试件面" + mark + " 改后 PASS"
	if rc, _, errb := gapRun(t, "gap", "set-state", id2, "--state", "已解", "--solved-kind", "销案",
		"--before-read", brBad, "--after-read", arBad, "--evidence", "非测试件面", "--yes"); rc != 2 {
		t.Errorf("负控②（非 zerg 非测试件面）⇒ 2，得到 %d · %q", rc, errb)
	}
	if gapSHAOf(t, ledger) != sha0 || len(gapAuditLines(t, audit)) != na0 {
		t.Errorf("负控②：拒写却动了盘（真源 sha 或审计行数变了）")
	}

	// ④ 旁证：`zerg` 正门命令（无标注）⇒ 0（回归 · 放宽不误伤正门面）。
	brZ := "cmd=zerg gap ls --state 仍缺 · rc=1 · reading=改前列于仍缺"
	arZ := "cmd=zerg gap ls --state 已解 · rc=0 · reading=改后列于已解"
	if rc, _, errb := gapRun(t, "gap", "set-state", id2, "--state", "已解", "--solved-kind", "销案",
		"--before-read", brZ, "--after-read", arZ, "--evidence", "zerg 回归", "--yes"); rc != 0 {
		t.Errorf("旁证（zerg 正门命令）⇒ 0，得到 %d · %q", rc, errb)
	}
	if r := gapRecOf(t, ledger, id2); r["state"] != "已解" {
		t.Errorf("旁证：真跑后 state 要 已解，得到 %v", r["state"])
	}
}
