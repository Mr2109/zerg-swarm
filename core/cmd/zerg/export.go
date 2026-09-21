// export.go —— 帮助导出（`zerg help export`）：把**命令树**渲染成一份 markdown 落 `Zerg-内部文档`。
//
// 三条纪律（§6.3 S4 · §九 M12 `E-M1` · §4.3 `U2`/`U4`）：
//
//	① 导出物里**命令名逐字来自命令树** —— 不许手写 ✗（本文件里没有任何命令名字面量）；
//	② 导出物与 `/api/capabilities`、`/api/openapi.json` **三面同源**：导出物每条命令都标出它投影的端点，
//	   对账（哪条命令在另两面上找不到对应）由 `zerg help export` 的输出与《开工记录》的复跑命令给；
//	③ 导出物是**产物**：它落 `Zerg-内部文档`（不开源侧），不进主仓 ✗。
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/version"
)

func cmdHelpExport(inv *invocation, stdout, stderr io.Writer) int {
	md := renderHelpMarkdown()
	root := repoRoot()

	// 默认落点：<仓根>/../Zerg-内部文档/项目文档/v2.5.10（**不写死私有绝对路径** —— 走相对推导）
	outDir := ""
	if v, ok := flagValue(inv.orig, "--out"); ok {
		outDir = v
	} else if root != "" {
		outDir = filepath.Join(root, "..", "Zerg-内部文档", "项目文档", "v2.5.10")
	}
	if outDir == "" {
		fmt.Fprintf(stderr, "%s: 解析不到落点 —— 给 --out <目录>，或在仓内跑\n", progName)
		return exitUsage
	}
	if st, err := os.Stat(outDir); err != nil || !st.IsDir() {
		fmt.Fprintf(stderr, "%s: 落点不是目录：%s（给 --out <已存在的目录>）\n", progName, outDir)
		return exitUsage
	}
	name := fmt.Sprintf("导出-命令面帮助-%s.md", time.Now().Format("20060102"))
	path := filepath.Join(outDir, name)
	if err := os.WriteFile(path, []byte(md), 0o644); err != nil {
		fmt.Fprintf(stderr, "%s: 写不进导出物：%v\n", progName, err)
		return exitFail
	}
	nCmd, nDanger := countCommands()
	if inv.jsonGiven {
		if !requireFields(inv, stderr) {
			return exitFail
		}
		return selectJSON(stdout, stderr, inv, inv.path, inv.fields, helpExportRow(path))
	}
	fmt.Fprintln(stdout, path) // stdout 只出结果：写哪儿了
	fmt.Fprintf(stderr, "导出 %d 条命令（其中危险动作 %d 条）→ %s\n", nCmd, nDanger, path)
	return exitOK
}

// helpExportRow —— `zerg help export --json` 的**机器面**（单一来源：命令树 + 层级计数 + 契约 schema）。
//
// 为什么抽成函数（§二十一 已红第 17 条 · 开工单 T-39）：`*.schema.json` 要与导出物**对拍得上**，
// 对拍就必须喂**同一份**真值 —— 若测试自己另拼一份 map，那份 map 与真跑的输出可以悄悄漂。
// 本函数于是成为唯一来源：命令面（cmdHelpExport）与对拍测试（export_schema_test.go）都调它。
func helpExportRow(path string) map[string]string {
	nCmd, nDanger := countCommands()
	return map[string]string{
		"path":      path,
		"commands":  fmt.Sprintf("%d", nCmd),
		"dangerous": fmt.Sprintf("%d", nDanger),
		"schema":    contractSchema,
		"layers":    layerCounts(), // §九 M10 X1：三档层级的机器可读计数（space 恒为 0）
	}
}

func countCommands() (open, danger int) {
	for _, c := range catalog() {
		if c.danger != nil {
			danger++
		} else {
			open++
		}
	}
	return open, danger
}

// dangerOpen —— 危险动作里**已开放**（`opened`）的条数：导出物的表头/计数行按它现算，
// 与 `zerg help dangerous` 的逐条标记同源（一个真源：命令树）。
func dangerOpen() int {
	n := 0
	for _, c := range catalog() {
		if c.danger != nil && c.opened {
			n++
		}
	}
	return n
}

// flagValue 从原始命令行里取 `--k v` 或 `--k=v` 的值（导出命令自己也走透传面）。
func flagValue(orig []string, key string) (string, bool) {
	for i, a := range orig {
		if a == key && i+1 < len(orig) {
			return orig[i+1], true
		}
		if strings.HasPrefix(a, key+"=") {
			return strings.TrimPrefix(a, key+"="), true
		}
	}
	return "", false
}

// renderHelpMarkdown —— 导出物的全部内容都由命令树现算（名字、用法、字段、端点、档位）。
func renderHelpMarkdown() string {
	var b strings.Builder
	open, danger := countCommands()
	fmt.Fprintf(&b, "# 导出：命令面帮助（自动生成 · 勿手改）\n\n")
	fmt.Fprintf(&b, "> 生成命令：`zerg help export`（`core/cmd/zerg/` 的命令树**逐字**渲染，一个名字都不是手写的）\n")
	fmt.Fprintf(&b, "> 命令面身份：`%s`\n", version.Line(progName))
	fmt.Fprintf(&b, "> 契约：`zerg help` / `zerg help exit-codes` / `zerg help config` / `zerg help dangerous`\n")
	fmt.Fprintf(&b, "> 本版：命令清单 **%d** 条 · 危险动作 **%d** 条（其中**已开放** %d 条 · 未开放 %d 条）\n\n", open, danger, dangerOpen(), danger-dangerOpen())
	fmt.Fprintf(&b, "## 一、命令清单（已开放 · 名字逐字来自命令树）\n\n")
	fmt.Fprintf(&b, "> 茧壁层级（§九 M10 `X1` · 闭集三值）：%s（`space` 恒为 0 —— 空间内零命令面）\n\n", layerCounts())
	b.WriteString("| 命令 | 说明 | `--json` 字段 | 投影的远端端点 | 茧壁层级 |\n|---|---|---|---|---|\n")
	for _, c := range catalog() {
		if c.danger != nil {
			continue
		}
		ep := c.endpoint
		if ep == "" {
			ep = "本机（无远端对应）"
		}
		f := strings.Join(c.fields, ",")
		if f == "" {
			f = "（无机器面）"
		}
		fmt.Fprintf(&b, "| `%s` | %s | %s | %s | `%s` |\n", "zerg "+strings.Join(c.path, " "), c.summary, f, ep, c.layer)
	}
	fmt.Fprintf(&b, "\n## 二、危险动作（已登记 · 逐条标**本版已开放 / 未开放**）\n\n")
	b.WriteString("| 命令 | 档 | 本版 | 三态 | `--confirm` 的目标 | 它会动什么 | 出处 | 茧壁层级 |\n|---|---|---|---|---|---|---|---|\n")
	for _, c := range catalog() {
		if c.danger == nil {
			continue
		}
		opened := "未开放"
		if c.opened {
			opened = "**已开放**"
		}
		fmt.Fprintf(&b, "| `%s` | %s | %s | --dry-run · --confirm · --yes | %s | %s | %s | `%s` |\n",
			"zerg "+strings.Join(c.path, " "), c.danger.Level, opened, c.danger.Target, c.danger.Effect, c.danger.Source, c.layer)
	}
	b.WriteString("\n## 三、退码表（唯一真源）\n\n")
	b.WriteString("| 码 | 名 | 语义 | 可重试性 |\n|---|---|---|---|\n")
	for _, rows := range [][]exitcodeRow{exitcodeTable, exitcodeReserved} {
		for _, r := range rows {
			fmt.Fprintf(&b, "| `%d` | `%s` | %s | %s |\n", r.Code, r.Name, r.Meaning, r.Retryable)
		}
	}
	b.WriteString("\n" + exitcodePolicy + "\n")
	b.WriteString("\n门禁直通档（不翻译）：" + gateExitLine + "\n")
	b.WriteString("\n## 四、配置优先级\n\n")
	b.WriteString(configPriorityLine + "\n")
	b.WriteString("\n## 五、三面同源对账（导出物 · `/api/capabilities` · `/api/openapi.json`）\n\n")
	b.WriteString("本节的表由命令树现算：每条命令的端点列**就是**它的远端对应；对账（哪条在另两面上找不到）用：\n\n")
	b.WriteString("    导出物端点集 · /api/capabilities 的 endpoint 集 · /api/openapi.json 的 paths 集 —— 三集求差\n\n")
	b.WriteString("导出物端点集（现算）：\n\n")
	eps := map[string]bool{}
	for _, c := range catalog() {
		if c.endpoint != "" {
			eps[c.endpoint] = true
		}
	}
	keys := make([]string, 0, len(eps))
	for k := range eps {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(&b, "- `%s`\n", k)
	}
	return b.String()
}
