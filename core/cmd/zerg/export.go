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
	"strconv"
	"strings"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/version"
)

// 档位（本命令的两档行为，逐条写在用法与 `zerg help export` 的输出里）：
//
//	默认档（**裸跑**）  —— 真写一件（`导出-命令面帮助-<YYYYMMDD>.md`），stdout 出落点路径（**旧行为一字不变**）；
//	只读档（`--dry-run`）—— **一个字节都不写**：把「逐条清单 + 它若真写会落哪件」打到 stdout（缺口 `G-17` ①）。
func cmdHelpExport(inv *invocation, stdout, stderr io.Writer) int {
	at := time.Now()

	// ① 落点（**版本无关**：不写任何版本号字面量）——
	//    优先级：`--out <目录>` > `--docs-ver <X.Y.Z>`（显式钉版 = 兼容旧行为）> 版本档案目录里「版本号最大且 ≥3 篇」的那个。
	outDir := ""
	if v, ok := flagValue(inv.orig, "--out"); ok {
		outDir = v
	} else {
		d, why := devDocsCurrentVersionDir(strings.TrimSpace(inv.flagVal("--docs-ver")))
		if why != "" {
			fmt.Fprintf(stderr, "%s: 解析不到落点 —— %s\n", progName, why)
			fmt.Fprintf(stderr, "（给 `--out <目录>` 或 `--docs-ver <X.Y.Z>` 钉一版；两种都不给时按「版本号最大且 ≥3 篇」现算）\n")
			// ★ 2026-09-24（缺口 `Q-070` · 组3 §一 序52 · 波10 序83）：§4.1 K14 四件套的「下一步」那一件。
			//   上面那句是**判据说明**（说缺什么），不是**可照抄的下一步**（说怎么改）—— 补一句能直接敲的。
			fmt.Fprintf(stderr, "下一步：`%s help export --dry-run --out <已存在的目录>` 先核一遍（不给 --out 时本命令按「版本号最大且 ≥3 篇」现算落点，那条路今天走不通）\n", progName)
			return exitUsage
		}
		outDir = d
	}
	if st, err := os.Stat(outDir); err != nil || !st.IsDir() {
		fmt.Fprintf(stderr, "%s: 落点不是目录：%s（给 --out <已存在的目录>）\n", progName, outDir)
		// ★ 2026-09-24（缺口 `Q-070` · 组3 §一 序52 · 波10 序83）：§4.1 K14 四件套的「下一步」那一件。
		//   原文案只报「给 --out <已存在的目录>」= 指出**哪里错**；源件（`缺口-命令面-20260921.md:227`）
		//   要的正是这一句 —— 补上「怎么改」（含可照抄的 `mkdir -p`）。
		fmt.Fprintf(stderr, "下一步：`mkdir -p %s` 再跑，或换一个已存在的目录（例：`--out ../Zerg-内部文档/项目文档/v2.5.12`）\n", outDir)
		return exitUsage
	}
	name := fmt.Sprintf("导出-命令面帮助-%s.md", at.Format("20060102"))
	path := filepath.Join(outDir, name)
	facts := helpExportFacts{
		Path:        path,
		Root:        devDocsBase(),
		DocsVersion: filepath.Base(outDir),
		GeneratedAt: at.Format(time.RFC3339),
		Written:     !inv.dryRun,
		DryRun:      inv.dryRun,
	}
	md := renderHelpMarkdown(outDir)

	// ② 默认档：真写（**旧行为一字不变**）；只读档：`--dry-run` ⇒ 这一步整个跳过。
	if !inv.dryRun {
		if err := os.WriteFile(path, []byte(md), 0o644); err != nil {
			fmt.Fprintf(stderr, "%s: 写不进导出物：%v\n", progName, err)
			return exitFail
		}
	}
	nCmd, nDanger := countCommands()
	if inv.jsonGiven {
		if rc := requireFields(inv, stderr); rc != exitOK {
			return rc
		}
		return selectJSONList(stdout, stderr, inv, inv.path, helpExportFieldList(inv.fields), helpExportRows(facts))
	}
	if inv.dryRun {
		// 只读档：stdout = 落点预告 + 逐条清单（与人面/导出物**同源**：都现算自命令树）。
		fmt.Fprintf(stdout, "（干跑 · 只读档：**一个字节都不写** · 由 `--dry-run` 打开）\n")
		fmt.Fprintf(stdout, "它若真写会落：%s（版本档案目录 `%s` · 生成时刻 %s）\n", path, facts.DocsVersion, facts.GeneratedAt)
		helpExportList(stdout)
		fmt.Fprintf(stderr, "干跑（零副作用）：命令 %d 条 · 危险动作 %d 条（已开放 %d · 未开放 %d）· **未写任何件** · 若真写落 %s\n",
			nCmd, nDanger, dangerOpen(), nDanger-dangerOpen(), path)
		return exitOK
	}
	fmt.Fprintln(stdout, path) // stdout 只出结果：写哪儿了
	fmt.Fprintf(stderr, "导出 %d 条命令（其中危险动作 %d 条 · 已开放 %d · 未开放 %d）→ %s（版本档案目录 %s · 生成时刻 %s）\n",
		nCmd, nDanger, dangerOpen(), nDanger-dangerOpen(), path, facts.DocsVersion, facts.GeneratedAt)
	return exitOK
}

// helpExportList —— **逐条清单**（人面只读档用）：与人面/导出物**同源** —— 全部现算自 `catalog()`。
// 危险动作逐条带**档位**（`D2`/`D3`）与**本版开没开**（`opened`）—— 这正是 `G-17` 说「只在人面」的那两格。
func helpExportList(w io.Writer) {
	nOpen, nDanger := countCommands()
	fmt.Fprintf(w, "命令清单（%d 条 · 已开放 · 名字逐字来自命令树）：\n", nOpen)
	for _, c := range catalog() {
		if c.danger != nil {
			continue
		}
		fmt.Fprintf(w, "  zerg %s ｜ 茧壁:%s ｜ %s\n", strings.Join(c.path, " "), c.layer, c.summary)
	}
	fmt.Fprintf(w, "危险动作（%d 条 · 已开放 %d · 未开放 %d · 档位逐条）：\n", nDanger, dangerOpen(), nDanger-dangerOpen())
	for _, c := range catalog() {
		if c.danger == nil {
			continue
		}
		opened := "未开放"
		if c.opened {
			opened = "已开放"
		}
		fmt.Fprintf(w, "  zerg %s ｜ %s ｜ %s ｜ --confirm=<%s> ｜ %s\n",
			strings.Join(c.path, " "), c.danger.Level, opened, c.danger.Target, c.danger.Effect)
	}
}

// helpExportFieldList —— `help export --json <字段>` 的**投影字段**（§一 序70 · `Q-012` · 组3 §一 序12）。
//
// 为什么要它：清单面的每一条都得**能被指认**。旧行为里 `--json commands` 把**摘要格**逐个投影，
// `items` 就变成同一个数重复（`{"commands":"82"}` × 120 ⇒ 逐条无命令名，消费方指不到是哪条）。
// ⇒ 只要请求里**没点名**行身份 `command`，就**补上**它：**请求的字段一个不少**（§九 M6 字段只增不改）、
// 而 `items` **逐条含命令名** ✓；旧消费方按 `items[0].commands` 读那个数**照旧可用** ✓。
func helpExportFieldList(fields []string) []string {
	for _, f := range fields {
		if f == "command" {
			return fields
		}
	}
	return append(append([]string{}, fields...), "command")
}

// helpExportFields —— `zerg help export --json` 的**合法字段**（机器面 = 逐条清单：一行一条命令）。
// 前 10 格是**摘要格**（每行都带，旧五格一格里都在 —— 字段只增不改）；后 9 格是**逐条格**（缺口 `G-17` ②）。
var helpExportFields = []string{
	"path", "root", "docs_version", "commands", "dangerous", "schema", "layers", "generated_at", "written", "dry_run",
	"command", "is_dangerous", "danger_level", "opened", "confirm_target", "summary", "layer", "endpoint", "fields",
}

// helpExportFacts —— 导出物的**四个事实**（全部现算，不是各算各的）：落点 · 取源根 · 版本档案目录 · 生成时刻 · 写没写。
type helpExportFacts struct {
	Path        string // 导出物完整路径（`--json` 的 `path`）
	Root        string // 版本档案取源根（`statepath.DocsBase()` · 缺件时为空串）
	DocsVersion string // 版本档案目录的**目录名**（如 `v2.5.11` · **现算**，不是写死的字面量）
	GeneratedAt string // 生成时刻（RFC3339）
	Written     bool   // 本档真写没写真（只读档 ⇒ false）
	DryRun      bool   // 只读档（`--dry-run`）开着没
}

// helpExportRows —— `zerg help export --json` 的**唯一真值行来源**（命令面与对拍测试 export_schema_test.go 都调它）。
//
// 为什么是**一行一条命令**（缺口 `G-17` ②）：旧机器面 `items[0]` 只给两个计数串
// （`{"commands":"73","dangerous":"35"}`）⇒ 消费方**判不了「这条命令是不是危险档、档位是几」**。
// 现在：逐条清单进机器面（`command` / `is_dangerous` / `danger_level` / `opened` / `confirm_target` / `layer` …），
// 与人工面（`helpExportList` / 导出物 markdown 的两张表）**同源** —— 三者都现算自同一个 `catalog()`。
//
// **字段只增不改**（§九 M6）：旧五格 `path`/`commands`/`dangerous`/`schema`/`layers` 一格里都在（每行都带），
// 旧消费方按 `items[0].commands` 读数照旧可用；`meta.count` 从 1 变成清单行数（= 命令树条数）。
func helpExportRows(f helpExportFacts) []map[string]string {
	nCmd, nDanger := countCommands()
	summary := map[string]string{
		"path":         f.Path,
		"root":         f.Root,
		"docs_version": f.DocsVersion,
		"commands":     fmt.Sprintf("%d", nCmd),
		"dangerous":    fmt.Sprintf("%d", nDanger),
		"schema":       contractSchema,
		"layers":       layerCounts(), // §九 M10 X1：三档层级的机器可读计数（space 恒为 0）
		"generated_at": f.GeneratedAt,
		"written":      strconv.FormatBool(f.Written),
		"dry_run":      strconv.FormatBool(f.DryRun),
	}
	rows := make([]map[string]string, 0, len(catalog()))
	for _, c := range catalog() {
		row := make(map[string]string, len(summary)+8)
		for k, v := range summary {
			row[k] = v
		}
		// `opened` = **真跑开没开**（缺口 序33 · 2026-09-24 已拍）—— 口径 = 单一真源 `openedForRun`：
		// 危险档看逐条 `opened` 标记；非危险档默认开放，但**声明了拒执**的（`refuses`）不算。
		// 修前这一格对非危险档**恒写 `"true"`** ⇒ 一条真跑拒执的命令在机器面上也报「已开放」（假绿）。
		isDanger, level, opened, target := "false", "—", strconv.FormatBool(openedForRun(c)), ""
		if c.danger != nil {
			isDanger, level, opened, target = "true", c.danger.Level, strconv.FormatBool(openedForRun(c)), c.danger.Target
		}
		row["command"] = "zerg " + strings.Join(c.path, " ")
		row["is_dangerous"] = isDanger
		row["danger_level"] = level
		row["opened"] = opened
		row["confirm_target"] = target
		row["summary"] = c.summary
		row["layer"] = c.layer
		row["endpoint"] = c.endpoint
		row["fields"] = strings.Join(c.fields, ",")
		rows = append(rows, row)
	}
	return rows
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
// `landing`（可选）= 落点目录：给了才写「版本档案目录」那一行（缺省调用点 = 对拍测试，判的是两张表的行数）。
func renderHelpMarkdown(landing ...string) string {
	var b strings.Builder
	open, danger := countCommands()
	fmt.Fprintf(&b, "# 导出：命令面帮助（自动生成 · 勿手改）\n\n")
	fmt.Fprintf(&b, "> 生成命令：`zerg help export`（`core/cmd/zerg/` 的命令树**逐字**渲染，一个名字都不是手写的）\n")
	fmt.Fprintf(&b, "> 命令面身份：`%s`\n", version.Line(progName))
	fmt.Fprintf(&b, "> 契约：`zerg help` / `zerg help exit-codes` / `zerg help config` / `zerg help dangerous`\n")
	fmt.Fprintf(&b, "> 本版：命令清单 **%d** 条 · 危险动作 **%d** 条（其中**已开放** %d 条 · 未开放 %d 条）\n", open, danger, dangerOpen(), danger-dangerOpen())
	// ★ 档位与身份两行（缺口 `G-17` ④：写入件要含**版本 / 时刻**）——
	//   「本版」那一行**一字不动**（scripts/docs/gen-cli-reference.py 逐字读它、且要求恰有两个 `**N**`），
	//   新信息一律**另起行**追加（新行都不带 `**N**` 粗体计数，免得被那条正则多抓）。
	if len(landing) > 0 && landing[0] != "" {
		fmt.Fprintf(&b, "> 版本档案目录：`%s`（**版本无关**：取「版本号最大且非递归 md ≥3 篇」的版本目录 —— 本文件不含任何版本号字面量）\n",
			filepath.Base(landing[0]))
	}
	fmt.Fprintf(&b, "> 生成时刻：`%s` · 组件版本：`%s`\n", time.Now().Format(time.RFC3339), version.Tag)
	fmt.Fprintf(&b, "> 危险档档位口径：`D3` = `--confirm=<目标>` + `--yes` 同时到 · `D2` = `--yes`（逐条档位见 §二 的「档」列）\n\n")
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
