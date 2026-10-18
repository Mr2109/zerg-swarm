// cli_machine_target_test.go —— 缺口 GAP-20260927-424 的判据件：
// 「显示串」与「机器目标名」**分字段**（此前 `dangerSpec.Target` 一字段两义）。
//
// 病（改前现读坐实）：`remote.go` 的机器对象判定用的是 `machineTargets[cmd.danger.Target]`
// —— 读的是**人面显示串** ⇒ 显示串与机器判定同源：想让某命令的显示串更贴「某台机」，
// 就不得不挑一个**不在** `machineTargets` 里的措辞（`dev edit` 被迫用第三值「本机名」绕开）。
//
// 治法（本单）：机器判定改为只读**机器目标名** `command.machine`（机读闭集），
// 与显示串 `dangerSpec.Target` 彻底分字段。判据四格（正控 2 + 负控 2，负控有牙）：
//
//	① 正控：真命令树里机器对象命令**恰好**一批，且逐条 `machine` 名逐字正确、属闭集；
//	② 正控：同批命令的**显示串** `dangerSpec.Target` 逐条非空、仍可读；
//	③ 负控（有牙）：显示串与机器字段**不一致**时，判定只认 `machine`（旧耦合口径必然判反）；
//	④ 负控（兼容）：显示串仍从 `dangerSpec.Target` 读（人面/导出同源），未被机器字段顶替。
package main

import (
	"strings"
	"testing"
)

// mtSplitWant —— 真命令树里**机器对象**命令的期望机器目标名（逐条现读派生 · 本单改动面）。
var mtSplitWant = map[string]string{
	"agent bootstrap":  machineNodeName,
	"agent deploy":     machineNodeName,
	"agent load":       machineNodeName,
	"agent reap":       machineNodeName,
	"agent registry":   machineNodeName,
	"agent reload":     machineNodeName,
	"agent restart":    machineNodeName,
	"agent unload":     machineNodeName,
	"build all":        machineHostName,
	"build release":    machineHostName,
	"context set-port": machineNodeName,
	"core restart":     machineHostName,
	"core start":       machineHostName,
	"core stop":        machineHostName,
	"core update":      machineHostName,
	"publish run":      machineHostName,
	"update":           machineHostName,
}

func TestMachineTargetFieldSplit(t *testing.T) {
	// ① 正控：真树里机器对象命令**恰好**这一批，且 machine 名逐字正确。
	got := map[string]string{}
	for _, c := range catalog() {
		if c.machine != "" {
			got[strings.Join(c.path, " ")] = c.machine
		}
	}
	if len(got) != len(mtSplitWant) {
		t.Errorf("机器对象命令条数：得 %d 要 %d（多出的键 %v）",
			len(got), len(mtSplitWant), mtSplitExtra(got, mtSplitWant))
	}
	for k, v := range mtSplitWant {
		if g := got[k]; g != v {
			t.Errorf("命令 %q 的机器目标名：得 %q 要 %q", k, g, v)
		}
	}
	// ①b 闭集：每条机器目标名必须是机读闭集成员（写错值 ⇒ fail-closed 不当作机器对象）。
	for k, v := range got {
		if !machineTargets[v] {
			t.Errorf("命令 %q 的机器目标名 %q 不在闭集 machineTargets 里", k, v)
		}
	}
	// ② 正控：同批命令的**显示串**逐条非空、仍可读（现与机器名同值，但分属两个字段）。
	for _, c := range catalog() {
		if c.machine == "" {
			continue
		}
		if c.danger == nil {
			t.Errorf("命令 %s 有 machine 却没有 danger 登记", strings.Join(c.path, " "))
			continue
		}
		if strings.TrimSpace(c.danger.Target) == "" {
			t.Errorf("命令 %s 的显示串 dangerSpec.Target 空了", strings.Join(c.path, " "))
		}
	}

	// ③ 负控（有牙）：显示串与机器字段**不一致** ⇒ 判定只认 machine（旧耦合口径必然判反）。
	//   甲：显示串写成机器词、机器字段为空 ⇒ 必须**不判**机器对象（旧口径：判机器 ⇒ 红）。
	if isMachineObject(&command{path: []string{"合成", "甲"},
		danger: &dangerSpec{Level: dangerD2, Target: machineHostName}}) {
		t.Error("负控③甲失守：显示串写了机器词、machine 为空却被判成机器对象 ⇒ 机器判定仍在读显示串（旧耦合回流）")
	}
	//   乙：机器字段是机器名、显示串与机器无关 ⇒ 必须**判**机器对象（旧口径：不判 ⇒ 红）。
	if !isMachineObject(&command{path: []string{"合成", "乙"},
		danger:  &dangerSpec{Level: dangerD2, Target: "（落点目录）"},
		machine: machineNodeName}) {
		t.Error("负控③乙失守：machine 是机器名、显示串无关却没被判成机器对象 ⇒ 机器判定没读 machine 字段")
	}

	// ④ 负控（兼容）：显示串仍从 dangerSpec.Target 读（人面/导出同源），未被机器字段顶替。
	syn := &command{path: []string{"合成", "丙"},
		danger:  &dangerSpec{Level: dangerD3, Target: "主机名（群级）", ChecksConfirm: true},
		machine: machineHostName}
	if f := dangerFlags(syn, true); !strings.Contains(f, "--confirm=<主机名（群级）>") {
		t.Errorf("负控④失守：人面三态形状没从 dangerSpec.Target 取显示串 ⇒ 旧字段名语义被改坏：%q", f)
	}
	// ④b 真树抽一条：机器命令的显示串确实进了人面三态形状（现读）。
	for _, c := range catalog() {
		if c.machine == "" || c.danger == nil {
			continue
		}
		if !strings.Contains(dangerFlags(c, true), "--confirm=<"+c.danger.Target+">") {
			t.Errorf("命令 %s：显示串 %q 没进人面三态形状", strings.Join(c.path, " "), c.danger.Target)
		}
		break
	}
}

func mtSplitExtra(got, want map[string]string) []string {
	var out []string
	for k := range got {
		if _, ok := want[k]; !ok {
			out = append(out, k)
		}
	}
	return out
}
