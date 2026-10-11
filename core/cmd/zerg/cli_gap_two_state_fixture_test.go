// cli_gap_two_state_fixture_test.go —— 缺口「销案两态放宽」的**第二档**机检：
// `gap set-state` 的 `--before-read`/`--after-read` 再允许「**仓外夹具命令**」读数 —— 真修的真实差异
// 只有**仓外夹具**才暴露（真仓无该形态 ⇒ 正门命令两态读数必然相同）时，`cmd=` 可用仓外夹具脚本/命令，
// 但 `reading=` **必须**注明这是夹具读数（含 `夹具`）⇒ 否则仍按「首词必须是 `zerg`」拒收。
//
// ★ 与已落地的「测试件面」档**不重复**：本档只收 shell 脚本（`.sh`/`.bash`/`.zsh`）与路径形直跑件；
//
//	`go test …` / `python3 <件>.py …` 仍归测试件面（`.py` 不属本档）。
//
// 六格（全合成盘面 · `t.TempDir()` 当状态目录 ⇒ 不碰真 `~/.zerg/state`）：
//
//	① 正控：仓外夹具 `cmd=` + 夹具读数标注 ⇒ 0 且真改态、`solved_evidence` 落夹具三格
//	② 负控①：仓外夹具 `cmd=` **缺**夹具读数标注 ⇒ 2 · 真源 sha 与审计零动 · 仍 仍缺
//	③ 负控②：非 zerg、非测试件面、非夹具命令（`ls -l`）⇒ 2 · 零动
//	④ 负控③：非夹具的其它工具（`go build`）⇒ 2 · 零动
//	⑤ 旁证①：`bash <路径形 .sh>` 直跑形态 ⇒ 0（放宽面覆盖解释器形态）
//	⑥ 旁证②：`zerg` 正门命令（无需标注）⇒ 0（回归 · 行为一字不动）
package main_test

import (
	"fmt"
	"strings"
	"testing"
)

func TestGapTwoStateFixture(t *testing.T) {
	_, ledger, audit := gapEnv(t)

	// 三条靶子（走同族写面 `add` 造 · id 由 add 生成 ⇒ **新账** · 销案须带两态）。
	for _, sym := range []string{"仓外夹具靶子甲", "仓外夹具靶子乙", "仓外夹具靶子丙"} {
		if rc, _, errb := gapRun(t, append(gapAddArgv("手搓："+sym, sym), "--yes")...); rc != 0 {
			t.Fatalf("造靶子（%s）失败：rc=%d stderr=%q", sym, rc, errb)
		}
	}
	ids := gapAllIDs(t, ledger)
	if len(ids) != 3 {
		t.Fatalf("靶子条数不对（要 3）：%v", ids)
	}
	id1, id2, id3 := ids[0], ids[1], ids[2]

	const mark = "夹具"

	// ① 正控：仓外夹具 `cmd=`（路径形直跑）+ 夹具读数标注 ⇒ 0 且真改态。
	brPos := "cmd=/private/tmp/zerg-fixture/probe.sh · rc=1 · reading=" + mark + "读数 改前全红"
	arPos := "cmd=/private/tmp/zerg-fixture/probe.sh · rc=0 · reading=" + mark + "读数 改后全绿"
	if rc, _, errb := gapRun(t, "gap", "set-state", id1, "--state", "已解", "--solved-kind", "销案",
		"--before-read", brPos, "--after-read", arPos, "--evidence", "仓外夹具正控", "--yes"); rc != 0 {
		t.Fatalf("正控（仓外夹具 cmd + 夹具读数标注）⇒ 0，得到 %d · %q", rc, errb)
	}
	if r := gapRecOf(t, ledger, id1); r["state"] != "已解" {
		t.Errorf("正控：真跑后 state 要 已解，得到 %v", r["state"])
	} else if ev := fmt.Sprint(r["solved_evidence"]); !strings.Contains(ev, "cmd=/private/tmp/zerg-fixture/probe.sh") ||
		!strings.Contains(ev, mark) {
		t.Errorf("正控：仓外夹具三格没落进 solved_evidence：%q", ev)
	}

	// ② 负控①：仓外夹具 cmd 但 reading 缺夹具读数标注 ⇒ 2 · 零动 · 仍 仍缺。
	sha0 := gapSHAOf(t, ledger)
	na0 := len(gapAuditLines(t, audit))
	brNo := "cmd=/private/tmp/zerg-fixture/probe.sh · rc=1 · reading=改前全红"
	arNo := "cmd=/private/tmp/zerg-fixture/probe.sh · rc=0 · reading=改后全绿"
	if rc, _, errb := gapRun(t, "gap", "set-state", id2, "--state", "已解", "--solved-kind", "销案",
		"--before-read", brNo, "--after-read", arNo, "--evidence", "缺标注", "--yes"); rc != 2 {
		t.Errorf("负控①（仓外夹具 cmd 缺标注）⇒ 2，得到 %d · %q", rc, errb)
	}
	if gapSHAOf(t, ledger) != sha0 || len(gapAuditLines(t, audit)) != na0 {
		t.Errorf("负控①：拒写却动了盘（真源 sha 或审计行数变了）")
	}
	if r := gapRecOf(t, ledger, id2); r["state"] != "仍缺" {
		t.Errorf("负控①：state 要仍 仍缺，得到 %v", r["state"])
	}

	// ③ 负控②：非 `zerg`、非测试件面、非夹具命令（`ls -l`）—— 带标注也拒 ⇒ 2 · 零动。
	brLs := "cmd=ls -l · rc=1 · reading=" + mark + "读数 改前全红"
	arLs := "cmd=ls -l · rc=0 · reading=" + mark + "读数 改后全绿"
	if rc, _, errb := gapRun(t, "gap", "set-state", id2, "--state", "已解", "--solved-kind", "销案",
		"--before-read", brLs, "--after-read", arLs, "--evidence", "非夹具", "--yes"); rc != 2 {
		t.Errorf("负控②（裸 ls -l）⇒ 2，得到 %d · %q", rc, errb)
	}

	// ④ 负控③：非夹具的其它工具（`go build` —— 首词 go 但非 test）⇒ 2 · 零动。
	brGo := "cmd=go build ./cmd/zerg · rc=1 · reading=" + mark + "读数 改前全红"
	arGo := "cmd=go build ./cmd/zerg · rc=0 · reading=" + mark + "读数 改后全绿"
	if rc, _, errb := gapRun(t, "gap", "set-state", id2, "--state", "已解", "--solved-kind", "销案",
		"--before-read", brGo, "--after-read", arGo, "--evidence", "go build", "--yes"); rc != 2 {
		t.Errorf("负控③（裸 go build）⇒ 2，得到 %d · %q", rc, errb)
	}
	if gapSHAOf(t, ledger) != sha0 || len(gapAuditLines(t, audit)) != na0 {
		t.Errorf("负控②/③：拒写却动了盘（真源 sha 或审计行数变了）")
	}

	// ⑤ 旁证①：`bash <路径形 .sh>`（解释器形态）+ 夹具读数标注 ⇒ 0。
	brBash := "cmd=bash /tmp/zerg-fixture/run.sh · rc=1 · reading=" + mark + "读数 改前全红"
	arBash := "cmd=bash /tmp/zerg-fixture/run.sh · rc=0 · reading=" + mark + "读数 改后全绿"
	if rc, _, errb := gapRun(t, "gap", "set-state", id3, "--state", "已解", "--solved-kind", "销案",
		"--before-read", brBash, "--after-read", arBash, "--evidence", "仓外夹具解释器形态", "--yes"); rc != 0 {
		t.Errorf("旁证①（bash <路径形 .sh>）⇒ 0，得到 %d · %q", rc, errb)
	}
	if r := gapRecOf(t, ledger, id3); r["state"] != "已解" {
		t.Errorf("旁证①：真跑后 state 要 已解，得到 %v", r["state"])
	}

	// ⑥ 旁证②：`zerg` 正门命令（无标注）⇒ 0（回归 · 放宽不误伤正门面）。
	brZ := "cmd=zerg gap ls --state 仍缺 · rc=1 · reading=改前列于仍缺"
	arZ := "cmd=zerg gap ls --state 已解 · rc=0 · reading=改后列于已解"
	if rc, _, errb := gapRun(t, "gap", "set-state", id2, "--state", "已解", "--solved-kind", "销案",
		"--before-read", brZ, "--after-read", arZ, "--evidence", "zerg 回归", "--yes"); rc != 0 {
		t.Errorf("旁证②（zerg 正门命令）⇒ 0，得到 %d · %q", rc, errb)
	}
	if r := gapRecOf(t, ledger, id2); r["state"] != "已解" {
		t.Errorf("旁证②：真跑后 state 要 已解，得到 %v", r["state"])
	}
}
