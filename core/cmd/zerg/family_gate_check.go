// family_gate_check.go —— `zerg gate check <判据名>`：**判据转发壳**（v2.5.14 · M1 首项 · 设计-判据 CLI 化-v1.7 §9.1）。
//
// 形状（写死）：**动词做命令、名词做参数** ⇒ `zerg gate check <判据名>`；判据名 ⇒ 件 是**参数**，
// 不是 189 条平级命令（§5.3 硬边「189 件不做 189 条平级命令」）。本壳在 M1 只做「**打得着**」——
// 真搬 Go 是 M2，本壳**不改判据实现**（子件一字不动）。
//
// 三条纪律（逐条对应已拍表 · 设计 §四）：
//
//	① **只转发、不翻译**：子件退多少、壳就退多少（`ee.ExitCode()` 原样转出）——
//	   0/1/2 三码逐字透传；**零分支**改写退码。子件 stdout/stderr/stdin 逐字对接，
//	   故态 A（既有门步口径）与态 B（本壳）的输出**逐字相同**（验收 §9.3「两态对拍」）。
//	② **件自身崩 / 跑不起来**（exec 层：解释器找不到 / 不可执行）⇒ 壳给 `8`（不给结论）
//	   ＋ `detail=judge_broken` ＋ `raw_rc` 留痕。**不抄 Docker 的 125/126/127**（那是它自家约定）；
//	   透传会把「壳故障」冒充成「件判负」✗（拍定第二条）。
//	③ **名字 ⇒ 件** 的解析面**保守且现读可机检**：名单真源 = **现读** `<仓根>/scripts/gates/` 的
//	   目录列举（`*.py` / `*.sh`；名字 ＝ 文件名去扩展名，§6「判据名 ＝ 件名去扩展名，逐字相同」）。
//	   **不手敲名录**、**不模糊匹配**、**不猜**：名字不在册 ⇒ 拒（`2`）并**给可用名列表**。
//	   ★ 名录由目录列举派生 ⇒ 新增判据件**自动**在册，**不动命令树**（§5.3）。
//
// 路径边界（承 §6）：解析出的件必须在 `<仓根>/scripts/` 前缀内，越界 ⇒ `2`（壳自身错，
// `judge_resolved=false`），不 exec。
//
// ★ 未做（照实 · 不在本刀）：退码台账 / 件级锁 / 槽池 / `--timeout` / `--json` 机器面 ——
// 它们是 M1 后续面（§9.3），本刀只交「打得着」的最小转发闭环。
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// gateCheckRosterDirRel —— 判据件名录的**目录真源**（相对仓根）。只此一处拼它，别处不许再拼。
const gateCheckRosterDirRel = "scripts/gates"

// gateCheckInterpreter —— 判据件扩展名 ⇒ 解释器（**不是**按位可执行位判 —— 件在仓库里多为 0644）。
// 闭集只有两种：`.py` ⇒ `python3`；`.sh` ⇒ `bash`（设计 §五「库类 21 件 + sh 42 件不特殊化」）。
var gateCheckInterpreter = map[string]string{".py": "python3", ".sh": "bash"}

// gateCheckRoster —— 现读名录：`{判据名: 件绝对路径}` ＋ 同名字列表（保守面）＋ 读不动的原因。
//
// 判据名 ＝ 文件名去扩展名（§6）；只收 `.py` / `.sh` 两种扩展名。同名两件（`x.py` 与 `x.sh` 并存）
// 逐条登记在 `dupes` —— 调用方**保守拒**（不许挑一个），因为「挑」就是猜。
func gateCheckRoster(root string) (map[string]string, []string, error) {
	dir := filepath.Join(root, gateCheckRosterDirRel)
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, err
	}
	byName := map[string]string{}
	dupes := []string{}
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		if _, ok := gateCheckInterpreter[filepath.Ext(n)]; !ok {
			continue
		}
		name := strings.TrimSuffix(n, filepath.Ext(n))
		if prev, exists := byName[name]; exists {
			dupes = append(dupes, fmt.Sprintf("%s（%s · %s 同名两件）", name,
				filepath.Base(prev), n))
			continue
		}
		byName[name] = filepath.Join(dir, n)
	}
	sort.Strings(dupes)
	return byName, dupes, nil
}

// gateCheckRosterSorted —— 名录的名字表（给人读的可用名列表；排序稳定）。
func gateCheckRosterSorted(byName map[string]string) []string {
	names := make([]string, 0, len(byName))
	for n := range byName {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// gateCheckAvailableLines —— 可用名列表的人读渲染（每行若干名，按显示宽度折行）。
func gateCheckAvailableLines(names []string, perLine int) []string {
	out := []string{}
	for i := 0; i < len(names); i += perLine {
		j := i + perLine
		if j > len(names) {
			j = len(names)
		}
		out = append(out, "  "+strings.Join(names[i:j], "  "))
	}
	return out
}

// gateCheck —— `zerg gate check <判据名> [子件参数…]` 的转发壳。
//
// `tail` = 用户敲在 `zerg gate check` 之后的**一切**（首枚 = 判据名，其余**逐字**交给子件）。
func gateCheck(inv *invocation, stdout, stderr io.Writer, root string) int {
	tail := []string{}
	if len(inv.orig) > 2 {
		tail = inv.orig[2:]
	}
	if len(tail) == 0 {
		fmt.Fprintf(stderr, "%s: `gate check` 要给判据名（例：zerg gate check check-affected-map）\n", progName)
		fmt.Fprintf(stderr, "%s: 名字 ⇒ 件 的现读名录 = %s/（判据名 ＝ 件名去扩展名）\n", progName, gateCheckRosterDirRel)
		fmt.Fprintf(stderr, "%s: judge_resolved=false\n", progName)
		return exitUsage
	}
	name := tail[0]
	if strings.HasPrefix(name, "-") {
		// 首枚是旗标（缺判据名）：与「无参数」同档 —— 用法错。
		fmt.Fprintf(stderr, "%s: `gate check` 首枚位置参数 = 判据名，不给旗标（收到 %q）\n", progName, name)
		fmt.Fprintf(stderr, "%s: judge_resolved=false\n", progName)
		return exitUsage
	}
	rest := tail[1:]

	byName, dupes, err := gateCheckRoster(root)
	if err != nil {
		fmt.Fprintf(stderr, "%s: ⛔ 判据名录读不到（%s/ 列举失败：%v）⇒ **不给结论**\n",
			progName, gateCheckRosterDirRel, err)
		fmt.Fprintf(stderr, "%s: judge_resolved=false\n", progName)
		return exitBlocked
	}
	if len(dupes) > 0 {
		fmt.Fprintf(stderr, "%s: ⛔ 判据名录里有**同名两件**（名字 ⇒ 件 不唯一）⇒ **不给结论**（禁猜）：\n", progName)
		for _, d := range dupes {
			fmt.Fprintf(stderr, "  · %s\n", d)
		}
		fmt.Fprintf(stderr, "%s: judge_resolved=false\n", progName)
		return exitBlocked
	}

	full, ok := byName[name]
	if !ok {
		// **保守拒**：名字不在册 ⇒ 拒并给可用名列表（✗ 不猜 · 不模糊匹配）。
		fmt.Fprintf(stderr, "%s: 判据名 %q **不在册**（名字 ⇒ 件 只认逐字同名；不模糊匹配）\n", progName, name)
		names := gateCheckRosterSorted(byName)
		fmt.Fprintf(stderr, "%s: 可用名 %d 条（现读 %s/ 列举）：\n", progName, len(names), gateCheckRosterDirRel)
		for _, ln := range gateCheckAvailableLines(names, 4) {
			fmt.Fprintln(stderr, ln)
		}
		fmt.Fprintf(stderr, "%s: judge_resolved=false\n", progName)
		return exitUsage
	}

	// 路径边界（承 §6）：必须在 `<仓根>/scripts/` 前缀内 —— 越界 ⇒ 壳自身错，不 exec。
	prefix := filepath.Join(root, "scripts") + string(os.PathSeparator)
	if !strings.HasPrefix(filepath.Clean(full)+string(os.PathSeparator), prefix) {
		fmt.Fprintf(stderr, "%s: ⛔ 解析出的件越出 <仓根>/scripts/ 前缀（%s）⇒ **不给结论**（不 exec）\n",
			progName, full)
		fmt.Fprintf(stderr, "%s: judge_resolved=false\n", progName)
		return exitUsage
	}
	interp := gateCheckInterpreter[filepath.Ext(full)]

	cmd := armChildGroup(exec.Command(interp, append([]string{full}, rest...)...))
	// 与 `cmdGate` 的其余转发同规：登记在跑的这一步（人打断时 `--cancel-on-interrupt` 有东西可取消）。
	runningChild = cmd
	defer func() { runningChild = nil }()
	cmd.Dir = root
	cmd.Stdin = os.Stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			// ★ 只转发、不翻译：子件的码原样返回（连异常码也不改写）。
			return ee.ExitCode()
		}
		// exec 层跑不起来（解释器找不到 / 件不可执行）：**不给结论**（拍定第二条）。
		// `raw_rc=-1` = **没有进程退码可言**（不是把故障写成 0 —— 硬边「禁用 raw_rc 顶替退码」）；留痕。
		fmt.Fprintf(stderr, "%s: ⛔ 判据件跑不起来（exec 层 · %s）：%v\n", progName, filepath.Base(full), err)
		fmt.Fprintf(stderr, "%s: detail=judge_broken · raw_rc=-1 · judge_resolved=true\n", progName)
		fmt.Fprintf(stderr, "%s: ⇒ 不给结论（%d）：壳故障与「件判负」不许同形\n", progName, exitBlocked)
		return exitBlocked
	}
	return exitOK
}
