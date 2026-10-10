// cli_help_danger_test.go —— 缺口 `GAP-20260927-335`（命令面 · P1）的**真二进制**判据：
// `zerg help <族> --json` 的**逐动作危险档机器面**。
//
// 病（改前现读 · 源码现编）：`zerg help task --json command,danger` **rc=0 却把 `--json` 静默吞掉** ——
// 走的是族级帮助**人面**文本（stdout 里既没有包封、也没有 `danger` 格）。「某动作是不是危险档 / 档位几」
// 只能肉眼从 `zerg help <族>` 人面数 `[危险档 D2]`（本条病灶「`gap export` 真写盘却判安全档」就是这么才
// 被发现的）。治法（**只增机器面**）：把 `command`/`danger`/`opened` 登进 `help` 的**可选字段层**（`optFields`，
// 与 `code find` 的 `count` / `gate ls` 的 `name` 同一体例）并在**点了非主题族名**那一态产出逐动作行；
// 人面（`zerg help <族>`）与 `help export` 文档面一行不动（冻结帮助面逐字节不变 —— 见两态读数）。
//
// ★ 本件从源码现编（`zergBinaryFromSource`）—— 本单硬约束禁重编 `bin/zerg`（行为面待重编）；拿旧制品
//
//	会把「本面未落制品」误读成「本面不存在」（与 `cli_code_find_count_test.go` 同法）。
//
// 成对四档：
//
//	① 正控：`zerg help task --json command,danger,opened` 收下（改前 rc=0 但静默吞 --json ⇒ 无包封），逐行动作一行
//	   带 `command`；`zerg task cancel` 那行的 `danger` == `D2`，且与 `zerg help dangerous` 人面里那行的档位
//	   **逐字相符**（**看出来的 == 机读到的**）。
//	② 负控：非危险命令 `zerg task ls` 那一行**无 `danger` 键**（如实报「非危险档」），且 `opened`=true。
//	③ 负控：人面 `zerg help task` 未被机器面短路（不出 JSON）· 首行形状未变 · 「动作 N 条」与机器面
//	   `meta.count` 同源相等 · 逐条 `[危险档 D2]` 标记仍在。
//	④ 负控：非法字段 `--json bogus_zz` 仍按既有档拒（rc=2 · detail `json_field_unknown:`）；不给主题那一态
//	   （`help --json family,actions`）与既有矩阵 must-fail 格（`help --json name` ⇒ rc=2）一字不动。
package main_test

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

var helpFamilyHeaderRE = regexp.MustCompile(`^zerg (\S+) —— 族级用法（动作 (\d+) 条 · 已开放 (\d+) · 拒执 (\d+) · 危险档 (\d+)）$`)

var helpDangerCancelRE = regexp.MustCompile(`(?m)^  zerg task cancel\s+(D[23])\s`)

// ① + ② 正控 / 负控：逐动作危险档机器面。
func TestHelpFamilyDanger_JSONFace(t *testing.T) {
	bin := zergBinaryFromSource(t)
	root := syntheticRepo(t, "exit 0\n")

	rc, out, errb := execCase(t, bin, root, "help", "task", "--json", "command,danger,opened")
	if rc != 0 {
		t.Fatalf("`zerg help task --json command,danger` 要 rc=0（改前：--json 被静默吞 ⇒ 出人面文本、无 danger 面）；得到 %d · stderr=%s", rc, errb)
	}
	if !strings.Contains(out, `"schema"`) {
		t.Fatalf("改前 `--json` 在族级面被静默吞 —— 改后必须出包封：%q", out)
	}
	var env struct {
		Items []map[string]string `json:"items"`
		Meta  struct {
			Count int `json:"count"`
		} `json:"meta"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("不是合法包封：%v · %q", err, out)
	}
	if env.Meta.Count != 13 {
		t.Errorf("`task` 族应有 13 条动作（meta.count），得到 %d", env.Meta.Count)
	}
	byCmd := map[string]map[string]string{}
	for _, it := range env.Items {
		byCmd[it["command"]] = it
	}

	// ① 正控：已知危险命令 `zerg task cancel`（D2）机读得到，且与 `zerg help dangerous` 人面逐字相符。
	cancel, ok := byCmd["zerg task cancel"]
	if !ok {
		t.Fatalf("机器面缺 `zerg task cancel` 一行：%q", out)
	}
	if cancel["danger"] != "D2" {
		t.Errorf("`zerg task cancel` 的 danger=%q（要 D2）", cancel["danger"])
	}
	rcD, outD, _ := execCase(t, bin, root, "help", "dangerous")
	if rcD != 0 {
		t.Fatalf("`zerg help dangerous` 要 rc=0，得到 %d", rcD)
	}
	m := helpDangerCancelRE.FindStringSubmatch(outD)
	if m == nil {
		t.Fatalf("`help dangerous` 人面里找不到 `zerg task cancel` 那行的档位：%q", outD)
	}
	if cancel["danger"] != m[1] {
		t.Errorf("机读 danger=%q ≠ `help dangerous` 人面档位 %q（看出来的 != 机读到的）", cancel["danger"], m[1])
	}
	if rm := byCmd["zerg task rm"]; rm == nil || rm["danger"] != "D3" {
		t.Errorf("`zerg task rm` 应是 D3：%v", rm)
	}

	// ② 负控：非危险命令 `zerg task ls` 那一行**无 `danger` 键**（如实报「非危险档」）。
	ls, ok := byCmd["zerg task ls"]
	if !ok {
		t.Fatalf("机器面缺 `zerg task ls` 一行")
	}
	if _, has := ls["danger"]; has {
		t.Errorf("非危险档 `zerg task ls` 不该有 `danger` 键：%v", ls)
	}
	if ls["opened"] != "true" {
		t.Errorf("`zerg task ls` 的 opened=%q（要 true）", ls["opened"])
	}
}

// ③ 负控：人面 `zerg help <族>` 逐字未变（未被机器面短路 · 与机器面同源）。
func TestHelpFamilyDanger_HumanFaceUnchanged(t *testing.T) {
	bin := zergBinaryFromSource(t)
	root := syntheticRepo(t, "exit 0\n")

	rc, out, errb := execCase(t, bin, root, "help", "task")
	if rc != 0 {
		t.Fatalf("`zerg help task` 要 rc=0，得到 %d · stderr=%s", rc, errb)
	}
	if strings.Contains(out, `"schema"`) || strings.Contains(out, `"items"`) {
		t.Errorf("人面被机器面短路了（`zerg help task` 出了 JSON）：%q", out)
	}
	first := strings.SplitN(out, "\n", 2)[0]
	m := helpFamilyHeaderRE.FindStringSubmatch(first)
	if m == nil {
		t.Fatalf("`zerg help task` 首行形状变了：%q", first)
	}
	// 人面报的「动作 N 条」必须与机器面 meta.count 同源相等（两读同一棵命令树）。
	rcJ, outJ, _ := execCase(t, bin, root, "help", "task", "--json", "command")
	if rcJ != 0 {
		t.Fatalf("`zerg help task --json command` 要 rc=0，得到 %d", rcJ)
	}
	var env struct {
		Meta struct {
			Count int `json:"count"`
		} `json:"meta"`
	}
	if err := json.Unmarshal([]byte(outJ), &env); err != nil {
		t.Fatalf("不是合法包封：%v · %q", err, outJ)
	}
	n, _ := strconv.Atoi(m[2])
	if n != env.Meta.Count {
		t.Errorf("人面「动作 %d 条」≠ 机器面 meta.count %d（人面与机器面必须同源）", n, env.Meta.Count)
	}
	// 人面仍带逐条危险档标记（这一格是人面既有形状，不因机器面而消失）。
	if !strings.Contains(out, "[危险档 D2]") {
		t.Errorf("人面丢了逐条 `[危险档 D2]` 标记（人面形状该逐字不变）：%q", out)
	}
}

// ④ 负控 + 对界：非法字段仍拒 · 无主题那一态与既有矩阵 must-fail 格一字不动。
func TestHelpFamilyDanger_BadFieldAndNoArgInvariants(t *testing.T) {
	bin := zergBinaryFromSource(t)
	root := syntheticRepo(t, "exit 0\n")

	// 负控：非法字段仍按既有档拒（rc=2 · detail 同源 `json_field_unknown:`）。
	rc, out, _ := execCase(t, bin, root, "help", "task", "--json", "bogus_zz")
	if rc != 2 {
		t.Errorf("非法字段 `--json bogus_zz` 要 rc=2，得到 %d", rc)
	}
	var env struct {
		Error struct {
			Detail string `json:"detail"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("非法字段应出包封（既有形态）：%v · %q", err, out)
	}
	if env.Error.Detail != "json_field_unknown:bogus_zz" {
		t.Errorf("非法字段 detail 要 %q，得到 %q", "json_field_unknown:bogus_zz", env.Error.Detail)
	}

	// 对界：不给主题那一态（族名清单机器面）一字不动。
	rc2, out2, _ := execCase(t, bin, root, "help", "--json", "family,actions")
	if rc2 != 0 || !strings.Contains(out2, `"family"`) {
		t.Errorf("`help --json family,actions`（无主题）该照旧出族名清单：rc=%d · %q", rc2, out2)
	}

	// 对界：`help --json name` 仍按既有矩阵格拒（rc=2）—— optFields 只增 `command`/`danger`/`opened`，不含 `name`。
	rcN, _, _ := execCase(t, bin, root, "help", "--json", "name")
	if rcN != 2 {
		t.Errorf("`help --json name` 仍该 rc=2（既有矩阵 must-fail 格），得到 %d", rcN)
	}
}
