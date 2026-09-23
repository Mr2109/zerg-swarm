// helptext.go —— `zerg help` 的渲染面（批 A · S1）。
//
// 三条纪律：
//
//	① 命令名与用法**逐字来自命令树**（`commands`），本文件不旁写第二份命令清单 ✗；
//	② 退码行与**门禁头注释第 35 行逐字对齐**（不改字、不换词序）—— 对齐检查见
//	   `zerg help exit-codes` 的「对齐」段与《开工记录-批A》T-01/T-04 两节；
//	③ 五级配置优先级照 §4.1 K10 序写死一份（clig.dev 五级序），不加不减。
package main

import (
	"fmt"
	"strings"
)

// gateExitLine —— `scripts/gates/precommit-gates.sh` 第 35 行的正文（去掉行首 `# `）。
// 门禁直通档（`zerg gate run` 不翻译脚本退码）逐字用它，故这里**一个字都不许改**。
const gateExitLine = "退出码：0 全绿 · 1 有失败项 · 2 **不给结论**（用法错/前置缺件/自检不过/**有步骤报 BLOCKED**）"

// configPriorityLine —— 五级配置优先级（§4.1 K10）：旗标 > 环境变量 > 项目 .env > 用户配置 > 内置。
const configPriorityLine = "旗标 > ZERG_* 环境变量 > 项目 .env > ~/.zerg/config.yaml > 内置默认"

// helpText 渲染整篇帮助。命令段由命令树现算 ⇒ 加一条命令只改一处。
// **只列本版已开放的**（`danger == nil`）；危险动作只给计数与入口 —— 逐条清单是 `zerg help dangerous`。
//
// `all` = `zerg help --all`（2026-09-24 · 波11 序93 · 缺口 `Q-071` · 同面 `W-08`）：
// 危险动作那一段**逐条列全** —— 命令树里 `danger != nil` 的每条各出一行（用法行逐字来自
// `command.usage`，不旁写第二份清单）。口径（判据的**唯一**读法，不写死数字）：
//
//	`help --all` 里以「  zerg 」开头的行数 = `help` 里同形状的行数 + 危险档条数（两者都由命令树现算）。
//
// 不带 `--all` 时**一个字节的旧行都不动**（只多一行指路，且那行不以「  zerg 」开头 ⇒ 不动上面那条等式）。
func helpText(all bool) string {
	var b strings.Builder
	b.WriteString(progName + " —— 虫族命令面（唯一入口）· 形态 zerg <对象> <动作> [参数] [旗标] · 深度 ≤ 3 层\n\n")
	b.WriteString("用法:\n")
	b.WriteString("  zerg <对象> <动作> [参数] [旗标]\n")
	for _, c := range catalog() {
		if c.danger == nil {
			fmt.Fprintf(&b, "  %s\n", c.usage)
		}
	}
	b.WriteString("\n命令（本版已开放 · 名字逐字来自命令树）:\n")
	w := 0
	for _, c := range catalog() {
		if c.danger == nil && len(c.usage) > w {
			w = len(c.usage)
		}
	}
	for _, c := range catalog() {
		if c.danger == nil {
			fmt.Fprintf(&b, "  %-*s  %s\n", w, c.usage, c.summary)
		}
	}
	nDanger, nOpen := 0, 0
	for _, c := range catalog() {
		if c.danger != nil {
			nDanger++
			if c.opened {
				nOpen++
			}
		}
	}
	// **不许一律写「本版未开放」**：危险档里有真实现的（`opened`）—— 逐数报，别让主帮助与 `help dangerous` 两处自相矛盾。
	fmt.Fprintf(&b, "\n危险动作（已登记 %d 条 · 已开放 %d 条 · 未开放 %d 条 · 逐条三态见 'zerg help dangerous'）:\n", nDanger, nOpen, nDanger-nOpen)
	b.WriteString("  zerg <危险动作> [参数] [--dry-run | --confirm=<目标> --yes]\n")
	// 指路行**不以「  zerg 」开头**：`help --all` 的条数等式（= `help` 条数 + 危险档条数）靠这一条守住。
	fmt.Fprintf(&b, "  （列全这 %d 条的逐条清单 ⇒ `zerg help --all`）\n", nDanger)
	if all {
		// `--all`：把危险档**逐条**列全（用法行逐字来自命令树；行数与危险档条数**恒等** —— 少一行由判据件抓）。
		dw := 0
		for _, c := range catalog() {
			if c.danger != nil && len(c.usage) > dw {
				dw = len(c.usage)
			}
		}
		for _, c := range catalog() {
			if c.danger != nil {
				fmt.Fprintf(&b, "  %-*s  %s\n", dw, c.usage, c.summary)
			}
		}
	}
	b.WriteString("\n机器面:\n")
	b.WriteString(fmt.Sprintf("  --json <字段>   必给逗号分隔字段；不给 ⇒ exit %d + 字段清单走 stderr + stdout 0 字节\n",
		exitCodeOf("usage")))
	b.WriteString("  --plain         非 TTY 默认档（行式）\n")
	b.WriteString("\n退码（单一真源）:\n")
	b.WriteString("  0 成功 · 1 一般失败 · 2 用法错/不给结论 · 4 未认证 · 8 有 BLOCKED\n")
	b.WriteString("  门禁直通档（不翻译）：" + gateExitLine + "\n")
	b.WriteString("\n配置优先级:\n  " + configPriorityLine + "\n")
	b.WriteString("\n危险动作（三态）:\n")
	b.WriteString("  --dry-run（零副作用）· --confirm=<目标>（值必须匹配目标）· --yes（只对第二危险档）\n")
	b.WriteString("\n见 'zerg help exit-codes' / 'zerg help config' / 'zerg help dangerous'。\n")
	// §九 M2 `C2`/`C4`（契约 §八）：令牌**永不进 argv / 永不入库** —— 这句必须在主帮助里
	// （判据：`zerg help` 里能查到这句话，而不是只藏在 `help credentials` 里）。
	b.WriteString("凭据：令牌**永不进 argv、永不入库**（优先级链 env:ZERG_TOKEN > ~/.zerg/token；\n")
	b.WriteString("      非交互用 `--token-stdin`；详见 `zerg help credentials`）。\n")
	return b.String()
}
