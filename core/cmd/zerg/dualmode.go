// dualmode.go —— 人机双模（§九 M14 · 调研-M14 条文 `T1`–`T6`）。
//
// 一句话：**三态，不是两态** —— 人面 / 行式面 / 机器面，契约要一起写。
//
//	`T1` **人面是行式面的渲染**：字段集 / 字段序 / 值文本**逐字节可对应**（差别只在装饰）。
//	`T2` **非 TTY 默认降级为行式**（`isatty` 判 `stdout`；`stderr` 另判 —— 两者分开）。
//	`T3` 颜色三档 + 四个环境变量（`NO_COLOR` / `CLICOLOR` / `CLICOLOR_FORCE` / `TERM=dumb`）。
//	`T4` **默认不开分页**；非 TTY 零动画、零进度条、零分页器。
//	`T5` `--quiet` **只减装饰**（不做 docker 式「只出 id」）。
//	`T6` `--verbose` 长名必有、**不给短别名 `-v`**。
//
// 承诺切成**两层**（§十二 `P-074`）：
//
//	**数据层**（字段集 / 序 / 值文本）**不因谁在读而变** —— 这是承诺面（`T1` 可判）。
//	**渲染层**（对齐空格 / 表头装饰 / 颜色 / 进度）**可降** —— 非 TTY 一律降到最朴素的形态。
//
// 三态用 `--json` / `--plain` 两旗 + 默认行为表达：**不给 `--format <…>`**（§十二 `P-072` 定案）。
package main

import (
	"io"
	"os"
	"strings"
)

// colorMode —— 颜色三档（`T3`：auto / always / never；四个环境变量参与判定）。
func colorMode() string {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return "never"
	}
	if os.Getenv("CLICOLOR_FORCE") != "" && os.Getenv("CLICOLOR_FORCE") != "0" {
		return "always"
	}
	if os.Getenv("CLICOLOR") == "0" {
		return "never"
	}
	return "auto"
}

// helpHumanMachine —— `zerg help human-machine`（§九 M14 的自描述面）。
func helpHumanMachine() string {
	var b strings.Builder
	b.WriteString("人机双模（§九 M14 · 调研-M14 `T1`–`T6`）\n\n")
	b.WriteString("**三态**（不是两态）：\n")
	b.WriteString("  人面      TTY 下的表格：同名字段 + 空格对齐（只是**渲染**）\n")
	b.WriteString("  行式面    非 TTY / `--plain`：制表符分隔，字段集与字段序**一模一样**\n")
	b.WriteString("  机器面    `--json <字段>`：包封 + 点名字段（不因 TTY 改形状）\n\n")
	b.WriteString("承诺**切成两层**（§十二 `P-074`）：\n")
	b.WriteString("  · **数据层**（字段集 / 序 / 值文本）**不因谁在读而变** —— 这是承诺面（`T1` 可逐字节对拍）。\n")
	b.WriteString("  · **渲染层**（对齐空格 / 表头装饰 / 颜色 / 进度条）**可降** —— 非 TTY 一律降到最朴素的形态。\n\n")
	b.WriteString("六条规矩：\n")
	b.WriteString("  `T1` 人面 = 行式面的渲染（长值只许折叠 + `--plain` 逃生）\n")
	b.WriteString("  `T2` 非 TTY 默认降级为行式；`isatty` 判 `stdout`，`stderr` **分开**判\n")
	b.WriteString("  `T3` 颜色三档（`--color=auto|always|never` · `--no-color` · `NO_COLOR` · `CLICOLOR` ·\n")
	b.WriteString("       `CLICOLOR_FORCE` · `TERM=dumb`）；非 TTY **自动无色**、不按屏宽截断\n")
	b.WriteString("  `T4` **分页默认关**；非 TTY 零动画 / 零进度条 / 零分页器\n")
	b.WriteString("  `T5` `--quiet` **只减装饰**（不做 docker 式「只出 id」—— 那会把数据层也改了）\n")
	b.WriteString("  `T6` `--verbose` 长名必有、**不给短别名 `-v`**\n\n")
	b.WriteString("三态怎么表达：`--json` / `--plain` 两旗 + 默认行为；**不给 `--format <…>`**（§十二 `P-072`）。\n")
	b.WriteString("  ⇒ 给了 `--format` = 未知旗标 = 退码 `2`（本版实测命令见开工记录的判据）。\n\n")
	b.WriteString("本机现读：\n")
	b.WriteString("  · 颜色档（环境变量判定）= " + colorMode() + "\n")
	b.WriteString("  · 分页器 = **无**（本命令面不调任何分页器、不设 `PAGER`）\n")
	return b.String()
}

// isTTYStdout 供文档与判据共用（`isatty` 只看 stdout —— 不看 stderr）。
func isTTYStdout(w io.Writer) bool { return ttyOf(w) }
