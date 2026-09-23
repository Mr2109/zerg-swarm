// family_calib.go —— `zerg calib`：标定线（`scripts/calib/` 3 件）的命令面（T-56 余项 · 2026-09-21 · DEV-0010）。
//
// 收编依据（**不自编归属**）：导出物 `Zerg-内部文档/项目文档/v2.5.10/归属-收编与退役-20260920.md` §④ 的
// 「脚本 27 条」里有「标定线 3 件」一组；逐件归 **① 收编为命令**（3 件全收）。族名 `calib` **逐字沿用**
// 任务单 §五 第 20 行的原话「是否开 `zerg calib`」（本批拍定「开」⇒ 不另编第二个族名）。
//
// 形态（对象 × 动作 · 2 段族 · 与 `doc meta` / `approve` 两族同一形状）：
//
//	zerg calib ls                        列 3 件的声明面（名字 · 件 · 角色 · 归属）
//	zerg calib show <名>                 单件的现状（含它的执行面与退码口径）
//	zerg calib run <名> [位置参数…] [--dry-run | --yes]     D2 三态：**真调旧脚本**，退码原样转出
//
// 三条硬口径（与 `gate run` 同一条纪律 · 不另立第二套）：
//
//	① **不搬实现**：执行面永远是 `bash|python3 scripts/calib/<件>` —— 命令面只做「名字逐字校验 +
//	   契约形状的包封 + 退码转发」；那 3 个脚本**一个字节不改**（仍在盘上、仍可直接跑）。
//	② **退码原样转出**：脚本退多少就返多少（含异常码）；命令面自己只在「执行前判」退 2、
//	   「`--json` 不给字段」退码**取自退码表**（`usage` · 归一后 = 2）、「件不在盘上」退 8。
//	③ **名字逐字匹配**：未知名 ⇒ 退 2 + 「最像的合法输入」（§4.1 K14 第三件 · 与 `gate run --step` 同一方言）。
//
// 干跑语义（门⑩ `dryrun.v1` · 登记在 `core/internal/contract/dryrun-semantics.json`）：
// `--dry-run` ⇒ 只出计划件（rc=0 · **不 exec 任何脚本**）；缺 `--yes`（且没给 `--dry-run`）⇒ fail-closed 退 2、
// 计划件照出、**不执行**。
package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// calibEntry —— 标定线的一件（命令面上的**对象**）。
type calibEntry struct {
	Name  string // 命令面上的对象名（`zerg calib show <名>`）
	File  string // 件（仓内相对路径 —— 执行面与出处都读它）
	Run   string // 执行器（`bash` / `python3`）—— 与脚本自己的 shebang 一致
	Role  string // 它在标定线里的角色（一句话 · 取自件头注释）
	State string // 归属（本批 = ① 收编为命令）
}

// calibStateAdopted —— 归属列的唯一取值（3 件全 ① · ②/④ 不在这条线上）。
const calibStateAdopted = "① 收编为命令（`zerg calib` 立族 · 2026-09-21）"

// calibRoster —— 标定线 3 件（**真源 = 盘上这 3 个件**；本表只登记命令面给它们起的名字与执行面）。
var calibRoster = []calibEntry{
	{
		Name: "calib-local-model", File: "scripts/calib/calib-local-model.sh", Run: "bash",
		Role:  "本机（macOS）模型标定：3 轮取上界 → 产出实测档案（§8.4 标定铁律：容量/预算/阈值必须实测得出）",
		State: calibStateAdopted,
	},
	{
		Name: "calib-task-budget", File: "scripts/calib/calib-task-budget.py", Run: "python3",
		Role:  "预算标定：跑 N 次零副作用靶子任务，量出 步数/token/墙钟 上界 → 产出 task-budget-calib.yaml（闸门只读档案）",
		State: calibStateAdopted,
	},
	{
		Name: "calib-x3-model", File: "scripts/calib/calib-x3-model.sh", Run: "bash",
		Role:  "X3（Linux/Radeon · 真 GTT）模型标定：3 轮取上界；端口 9699（刻意避开子端池 9400–9499 与沙箱池）",
		State: calibStateAdopted,
	},
}

// calibLsFields / calibShowFields / calibRunFields —— 三条命令的 `--json` 字段（K1：机器面先定）。
var calibLsFields = []string{"name", "script", "role", "state"}
var calibShowFields = []string{"name", "script", "role", "state", "runner", "exit"}
var calibRunFields = []string{"name", "script", "mode", "result", "rc", "note"}

// calibExitNote —— 退码口径（`show` 面与计划件共用同一句，不各写一份）。
const calibExitNote = "脚本的退码**原样转出**（0 跑完 / 1 有失败项 / 2 不给结论 / 其它异常码照转）；" +
	"命令面自己只退 2（用法错 · 含未知名）与 8（件不在盘上 · 不给结论）"

// calibNames —— 对象名的现读清单（`ls` 与「最像的合法输入」共用一份）。
func calibNames() []string {
	out := make([]string, 0, len(calibRoster))
	for _, e := range calibRoster {
		out = append(out, e.Name)
	}
	sort.Strings(out)
	return out
}

// calibFind 按**名字逐字**找（模糊匹配等于没匹配 ⇒ 不猜）。
func calibFind(name string) *calibEntry {
	for i := range calibRoster {
		if calibRoster[i].Name == name {
			return &calibRoster[i]
		}
	}
	return nil
}

// nameCandidates —— 未知名时列的「相干候选」（前 n 个）：先按「谁是谁的前缀 / 谁包含谁」挑，
// 一个都不相干 ⇒ 退回全部候选的前 n 个。**只用于报面，不参与判定**。
func nameCandidates(names []string, want string, n int) []string {
	cands := []string{}
	for _, s := range names {
		if want != "" && (strings.HasPrefix(s, want) || strings.HasPrefix(want, s) || strings.Contains(s, want)) {
			cands = append(cands, s)
		}
	}
	if len(cands) == 0 {
		cands = append(cands, names...)
	}
	sort.Strings(cands)
	if len(cands) > n {
		cands = cands[:n]
	}
	return cands
}

// emitNameMiss —— 未知名/缺名的统一报面（K14 四件套：自报是谁 · 下一步 · 最像的合法输入 · 上下文定位）。
func emitNameMiss(inv *invocation, stderr io.Writer, family, action, name string, names []string, what string) int {
	inv.setErr("usage", "name_absent", "对象名不存在")
	if name == "" {
		fmt.Fprintf(stderr, "%s: `%s %s` 要给一枚%s名（`%s %s ls` 列的就是它）⇒ 退码 2\n",
			progName, family, action, what, progName, family)
	} else {
		fmt.Fprintf(stderr, "%s: %s里没有 %q（**逐字**匹配 · 现读 %d 件）⇒ 退码 2\n",
			progName, family, name, len(names))
	}
	if s := nearestName(name, names); s != "" && name != "" {
		fmt.Fprintf(stderr, "最像的合法输入: %s\n", s)
	}
	for _, s := range nameCandidates(names, name, 3) {
		fmt.Fprintf(stderr, "  候选：%s\n", s)
	}
	return exitUsage
}

// cmdCalibLs —— `zerg calib ls`：列 3 件的声明面。
func cmdCalibLs(inv *invocation, stdout, stderr io.Writer) int {
	rows := []map[string]string{}
	for _, e := range calibRoster {
		rows = append(rows, map[string]string{
			"name": e.Name, "script": e.File, "role": e.Role, "state": e.State,
		})
	}
	if len(rows) == 0 {
		// 声明面为空 = 空转（假覆盖）⇒ 不给结论，不当绿（§17.6 同一口径）。
		inv.setErr("blocked", "roster_empty", "标定线声明面为空")
		fmt.Fprintf(stderr, "%s: 标定线声明面一件都没有 ⇒ 不给结论（退码 8）\n", progName)
		return exitBlocked
	}
	return listCmd(inv, stdout, stderr, calibLsFields, rows)
}

// cmdCalibShow —— `zerg calib show <名>`：单件的现状（名字逐字匹配 · 未知名 ⇒ 2 + 最像的）。
func cmdCalibShow(inv *invocation, stdout, stderr io.Writer) int {
	name := ""
	if len(inv.args) > 0 {
		name = strings.TrimSpace(inv.args[0])
	}
	if name == "" {
		return emitNameMiss(inv, stderr, "calib", "show", "", calibNames(), "件")
	}
	e := calibFind(name)
	if e == nil {
		return emitNameMiss(inv, stderr, "calib", "show", name, calibNames(), "件")
	}
	row := map[string]string{
		"name": e.Name, "script": e.File, "role": e.Role, "state": e.State,
		"runner": e.Run + " " + e.File,
		"exit":   calibExitNote,
	}
	return listCmd(inv, stdout, stderr, calibShowFields, []map[string]string{row})
}

// cmdCalibRun —— `zerg calib run <名> [位置参数…] [--dry-run | --yes]`：D2 三态，真调旧脚本。
//
// 执行面**一个字都没搬进来**：跑的永远是 `bash|python3 scripts/calib/<件>`（脚本本体是唯一真源）。
// 命令面只做三件事：名字校验（执行前判）· 计划件 · **把脚本的退码原样转出**。
func cmdCalibRun(inv *invocation, stdout, stderr io.Writer) int {
	name := ""
	if len(inv.args) > 0 {
		name = strings.TrimSpace(inv.args[0])
	}
	if name == "" {
		return emitNameMiss(inv, stderr, "calib", "run", "", calibNames(), "件")
	}
	e := calibFind(name)
	if e == nil {
		return emitNameMiss(inv, stderr, "calib", "run", name, calibNames(), "件")
	}
	root := repoRoot()
	if root == "" {
		inv.setErr("blocked", "repo_root_absent", "解析不到仓根")
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 不给结论（退码 8）\n", progName)
		return exitBlocked
	}
	script := filepath.Join(root, filepath.FromSlash(e.File))
	if fi, err := os.Stat(script); err != nil || fi.IsDir() {
		inv.setErr("blocked", "script_absent", "件不在盘上")
		fmt.Fprintf(stderr, "%s: 件不在盘上：%s（%v）⇒ 不给结论（退码 8 · **不假装跑过**）\n", progName, e.File, err)
		return exitBlocked
	}
	// 位置参数**原样**交给脚本（对象名后面给什么就是什么）；命令面不解释、不改写。
	extra := []string{}
	if len(inv.args) > 1 {
		extra = inv.args[1:]
	}
	argv := append([]string{e.Run, script}, extra...)

	row := map[string]string{
		"name": e.Name, "script": e.File, "mode": "real", "result": "planned",
		"rc": "未跑", "note": "",
	}

	// ① fail-closed 那一态：没给 `--dry-run` 也没给 `--yes` ⇒ 计划件走 stderr、**不执行**、退 2。
	if !inv.dryRun && !inv.yes {
		inv.setErr("usage", "missing_yes", "缺 --yes（D2 写面）")
		emitCalibPlan(stderr, e, argv, true)
		fmt.Fprintf(stderr, "%s: 这一步是 D2 档（真跑一个标定脚本 · 会占机器/模型槽）—— **缺 `--yes` ⇒ 不执行**（退码 2）；"+
			"先看 `%s calib run %s --dry-run` 的计划件\n", progName, progName, e.Name)
		return exitUsage
	}

	// ② 干跑那一态：只出计划件、零副作用（`--json` 时计划件走 stderr，stdout 只留机器面）。
	if inv.dryRun {
		planOut := stdout
		if inv.jsonGiven {
			planOut = stderr
		}
		emitCalibPlan(planOut, e, argv, false)
		row["mode"], row["result"], row["rc"] = "dry-run", "planned", "0"
		row["note"] = "（--dry-run：零副作用 · 未 exec 任何脚本）"
		if inv.jsonGiven {
			if rc := requireFields(inv, stderr); rc != exitOK {
				return rc
			}
			return selectJSON(stdout, stderr, inv, inv.path, calibRunFields, row)
		}
		return exitOK
	}

	// ③ 真跑那一态：执行面 = 旧脚本自己；命令面只转出退码（**连异常码也不改写**）。
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = root
	cmd.Stdin = os.Stdin
	var human bytes.Buffer
	cmd.Stdout = &human // 脚本自己的输出先收着：`--json` 时转 stderr，否则原样进 stdout
	cmd.Stderr = stderr
	runningChild = cmd
	err := cmd.Run()
	runningChild = nil
	rc := exitOK
	if ee, ok := err.(*exec.ExitError); ok {
		rc = ee.ExitCode() // ★ 只转发、不翻译
	} else if err != nil {
		inv.setErr("failed", "exec_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 跑不动 %s：%v\n", progName, e.File, err)
		return exitFail
	}
	if inv.jsonGiven {
		_, _ = io.Copy(stderr, &human)
	} else {
		_, _ = io.Copy(stdout, &human)
	}
	row["mode"], row["result"] = "real", "ran"
	row["rc"] = fmt.Sprintf("%d", rc)
	row["note"] = "（退码原样转出：脚本退多少就返多少）"
	if rc != exitOK {
		fmt.Fprintf(stderr, "%s: %s 退码 %d —— **原样转出**（命令面不改写任何码）\n", progName, e.File, rc)
	}
	if inv.jsonGiven {
		if rc := requireFields(inv, stderr); rc != exitOK {
			return rc
		}
		if jrc := selectJSON(stdout, stderr, inv, inv.path, calibRunFields, row); jrc != exitOK {
			return jrc
		}
	}
	return rc
}

// emitCalibPlan —— 计划件（干跑与 fail-closed 两态共用同一份读数 · 零副作用）。
func emitCalibPlan(w io.Writer, e *calibEntry, argv []string, blocked bool) {
	fmt.Fprintln(w, "计划件（标定线 · 只列一件要跑的脚本 · 未执行任何东西）")
	fmt.Fprintf(w, "  动作     : %s calib run %s\n", progName, e.Name)
	fmt.Fprintf(w, "  件       : %s（归属：%s）\n", e.File, e.State)
	fmt.Fprintf(w, "  角色     : %s\n", e.Role)
	fmt.Fprintf(w, "  执行面   : %s（脚本本体是唯一真源 —— 命令面不搬它的实现）\n", strings.Join(argv, " "))
	fmt.Fprintf(w, "  退码口径 : %s\n", calibExitNote)
	if blocked {
		fmt.Fprintln(w, "  未执行   : D2 档 —— 要 `--yes`（或先 `--dry-run` 看这一份）")
	}
}
