// family_gate_show.go —— `zerg gate show <步名> --json` 的**闭集四格**机器面（缺口 `Q-061` / `B-3` · 2026-09-23）。
//
// 病灶（`缺口-命令面-20260921.md:223` 逐字）：`zerg gate show "门⑪"` ⇒ rc=0，只输出
// `python3 scripts/gates/check-cli-contract.py` 一行 —— **给不了** `scope` / `mode` / `判据` / `日志路径`。
//
// 本件补的**只是机器面**（`--json` 那一条路），默认面**一个字节不动**：
//
//	`zerg gate show <步名>`         → `bash scripts/gates/precommit-gates.sh --emit-cmd <步名>`（逐字透传 · 未改）
//	`zerg gate show <步名> --json`  → 单份 JSON 包封（六键）· `items[0]` 是**闭集四格**
//
// 四格（`Q-061` 的可核条件逐字：`scope` / `mode` / `判据` / `日志路径`）：
//
//	scope            —— 脚本那一行 `add_step` 的 scope（逐字，不翻译）
//	mode             —— 脚本那一行 `add_step` 的模式（四档闭集值**在前**）+ 档位词（阻断 / 只报告）
//	criterion(判据)  —— 这一步**判什么**（模式语义 + 被调的命令串）
//	log(日志路径)     —— 这一步的日志会落在哪（序号以脚本自己的 `--list` 为准）
//
// 为什么后两格与 `gate explain` 逐字同源：两处都从**脚本自己的 `add_step` 行**读（不另写一份步骤表 ·
// `G1-a`），共用 `gateStepCriterion` / `gateStepLogNote` / `parseAddSteps` —— 判据与日志路径在本仓
// 只有**一处**口径（两处各写一份必然漂）。
//
// ★ 可核条件 ③（名字不与 `gate show` 同义冲突）：本面**没有**新名字 —— 它是 `gate show` 上的一枚
//
//	`--json`；`gate show` / `gate explain` 两个名字一个都没动，默认面仍是那个「命令串直取口」。
//
// ★ `--json` **不给字段**这一格（与本仓 K2 的通例**不同**，理由写在这里）：本命令的字段面是
//
//	**闭集四格**（`gateShowFields`）—— 四格恒有值、恒全部有意义 ⇒ 不给字段 = 取闭集全量，退 0 出四格。
//	K2 的「1 + stderr 列全部字段」是为**开放字段面**的命令写的（不点名就不知道该给哪些）；本命令
//	不存在「该给哪些」的歧义。给了字段 ⇒ 投影到子集（未知字段 ⇒ 2 + stderr 列四格合法字段 · `I5`）。
//	—— 这一条与 `Q-061` 的可核条件逐字一致：`zerg gate show <步名> --json` ⇒ 输出**闭集四格**。
package main

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// gateShowFields —— `gate show --json` 的**闭集四格**（K1：机器面先定；键名与 `gate explain` 的
// 同义字段逐字相同 —— 两个命令的「判据」与「日志路径」只有一处口径）。
var gateShowFields = []string{"scope", "mode", "criterion", "log"}

// gateShowModeCell —— `mode` 那一格：**脚本声明的四档闭集值在前**（机读面按前缀取，值域不新增），
// 括号里是同一件事的档位词（`Q-061` 的「mode（阻断/只报告）」逐字）。
func gateShowModeCell(mode string) string {
	switch mode {
	case "rc", "empty", "tri":
		return mode + "（阻断）"
	case "tri-report":
		return mode + "（只报告）"
	}
	return mode + "（不在四档闭集 rc/empty/tri/tri-report 里 ⇒ 脚本会硬红：这一步的档位申报有错）"
}

// gateShowJSON —— `gate show <步名> --json`（默认面**不经这里** —— 见 `gate.go` 的 `show` 分支）。
//
// 退码：0 出四格 · 2 用法错（缺步名 / 字段闭集外 / 步名逐字不命中 / 重名歧义）。
// 全程**只读**：读脚本、跑脚本自己的 `--list`（只列不跑）—— 不写任何件、不跑任何步骤。
func gateShowJSON(inv *invocation, stdout, stderr io.Writer, root, script string) int {
	name := ""
	if len(inv.args) > 0 {
		name = strings.TrimSpace(inv.args[0])
	}
	if name == "" {
		inv.setErr("usage", "missing_step_name", "缺步名")
		fmt.Fprintf(stderr, "%s: `gate show --json` 要给步名（例：%s gate show 'gofmt -l core' --json）\n", progName, progName)
		fmt.Fprintf(stderr, "口径：不给 `--json` 的默认面仍是**命令串直取口**（`--emit-cmd` 逐字透传）；本面只补四格\n")
		return exitUsage
	}
	// ① 字段面（**执行前判**）：不给字段 = 闭集全量；给了就逐枚核，点到闭集外 ⇒ 2 + 列四格。
	fields := inv.fields
	if len(fields) == 0 {
		fields = gateShowFields
	}
	for _, f := range fields {
		if !gateShowFieldLegal(f) {
			// `error.kind` 是 AI 自愈的入口（门⑤ 的闭集）⇒ 用法错这一档要**自己**报 usage，
			// 别让它走「命令没报出 kind ⇒ 按退码兜底」那条兜底路（实读会变成 failed/rc=1，与退码 2 打架）。
			inv.setErr("usage", "unknown_field", "字段不在闭集四格里")
			return reportBadField(stderr, inv.path, f)
		}
	}

	// ② 步名**逐字**校验（执行前判 · 与 `gate explain` 同一份解析）：未知 / 重名都不许静默放过。
	src, err := os.ReadFile(script)
	if err != nil {
		inv.setErr("blocked", "gate_script_unreadable", "门禁脚本读不到")
		fmt.Fprintf(stderr, "%s: 读不到门禁脚本 %s（%v）⇒ 不给结论（退码 8）\n", progName, gateScriptRel, err)
		return exitBlocked
	}
	decls := parseAddSteps(string(src))
	hits := []gateStepDecl{}
	for _, d := range decls {
		if d.Name == name {
			hits = append(hits, d)
		}
	}
	if len(hits) == 0 {
		inv.setErr("usage", "step_absent", "步名不存在")
		fmt.Fprintf(stderr, "%s: 步骤名 %q **逐字**找不到（现读脚本里的 add_step 行 %d 条）⇒ 退码 2\n", progName, name, len(decls))
		fmt.Fprintf(stderr, "口径：本命令**精确匹配**（默认面的子串匹配查不到精确名 —— 见缺口稿 §十二 `B-4`）\n")
		if s := nearestSteps(decls, name, 1); len(s) > 0 {
			fmt.Fprintf(stderr, "最像的合法输入: %s\n", s[0])
		}
		for _, s := range nearestSteps(decls, name, 3) {
			fmt.Fprintf(stderr, "  候选：%s\n", s)
		}
		return exitUsage
	}
	if len(hits) > 1 {
		inv.setErr("usage", "step_ambiguous", "步名歧义")
		fmt.Fprintf(stderr, "%s: 步骤名 %q 在脚本里出现 %d 次 ⇒ **歧义**（判据要一个答案，不是一坨 · 退码 2）：\n",
			progName, name, len(hits))
		for _, d := range hits {
			fmt.Fprintf(stderr, "  %s:%d  scope=%s mode=%s\n", gateScriptRel, d.Line, d.Scope, d.Mode)
		}
		return exitUsage
	}
	d := hits[0]

	// ③ 四格齐了才把「闭集全量」落到 `inv.fields`（**失败路径仍走 K2 的 0 字节档** ——
	//    见文件头：用法错那一档 stdout 一个字节都不写，报错只走 stderr）。
	if len(inv.fields) == 0 {
		inv.fields = fields
	}
	logNote, _ := gateStepLogNote(root, d.Name)
	row := map[string]string{
		"scope":     d.Scope,
		"mode":      gateShowModeCell(d.Mode),
		"criterion": gateStepCriterion(d, string(src)),
		"log":       logNote,
	}
	return selectJSON(stdout, stderr, inv, inv.path, fields, row)
}

// gateShowFieldLegal —— 字段是否在闭集四格内。
func gateShowFieldLegal(f string) bool {
	for _, ok := range gateShowFields {
		if f == ok {
			return true
		}
	}
	return false
}
