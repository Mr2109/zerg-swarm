// family_model_record.go —— `zerg model record verify|probe|ls`（模型登记记录工具 · **归入命令面** · 批 B6）。
//
// 源件 = 独立入口 `core/cmd/zerg-model/main.go`（668 行 · 三个子命令 `verify` / `probe` / `list`）。
// 本件是那一件的**逐面搬**：三子命令的旗标解析、判词、退码语义、逐条输出、落盘路径（`--out` 原子写 /
// `--store` 进模型目录 / 兄弟文件 `<版>.trace.json` / `<版>.capabilities.json`）**逐字**照源件；
// 旗标名一枚不增不删（源件 16 枚：`--strict` / `--json` / `--integrity` / `--in-place` / `--base-dir` ·
// `--out` / `--store` / `--store-root` / `--endpoint` / `--engine` / `--id` / `--timeout` / `--parent` /
// `--base` · `--root` / `--manifests`）。
//
// 形态：
//
//	zerg model record verify <记录件> [--strict] [--in-place] [--integrity] [--base-dir <目录>] [--json <字段>]
//	zerg model record probe  <路径|端点URL> [--out <件>] [--store] [--store-root <目录>]
//	                         [--endpoint <URL>] [--engine <引擎>] [--id <id>] [--timeout <时长>]
//	                         [--parent <上一版>] [--base <基座 id>] [--json <字段>]
//	zerg model record ls     [--root <模型目录根>] [--manifests <目录>] [--json <字段>]
//
// 为什么并入**既有** `model` 族、✗ 不新开族：`model` 族**已经在册**（`ls` / `show` / `opts` / `verify` /
// `put` / `pull` / `add` / `rm` / `set` / `start` / `stop` —— 打主控的模型投影与控制面）。本件搬的是
// **另一件事**：模型登记**记录件**（标准 §三 的 `manifests/` 布局 · `core/internal/modelreg` 的实施面）。
// 两者共用族名、动作名分家 —— 这是本笔的**照实差异 ①**（源件的 `verify` / `list` 两个名字在 `model`
// 族里都被占了，且**语义不同**）：
//
//	源件 `zerg-model verify <记录件>`  = 校验**一条记录**是否符合标准（本件 `model record verify`）
//	命令面 `zerg model verify 模型@机`  = **四方一次对账**（主控下发 / 子端注册表 / 档案意图 / 运行期实占 ——
//	                                     `cmdModelVerify`，只读，对象是「模型@机」，与记录件无关）
//	源件 `zerg-model list`             = 列**模型目录里的记录**（本件 `model record ls`）
//	命令面 `zerg model ls`             = 列**主控 `/api/fleet/models`** 的机型投影（`cmdModelLs`）
//
// ⇒ 撞名的两个动作**一律不覆盖既有命令**（硬约束：既有命令的退码与输出一字不动）；本件挂一层
// `record` 中间段（3 段路径，`resolve()` 已支持），把「记录件」这一面整块收纳进去。✗ 不新开顶层族
// （`model record …` 与 `model ls/verify` 的分工在 `zerg help model` 里一眼可读）。
//
// 退码（★ 与源件的码空间**不同** —— 源件是**组件自有码空间**，命令面读 `exitcodes.go` 的唯一真源）：
//
//	0  成功（记录合格 / 探测成功且记录合格 / 列目录有 error 0 条）
//	1  跑到了但没成功：记录不合标准（源件 2）· 探测失败（源件 1）· 读不到 / 写不进（源件 3 里的环境类）
//	2  用法错（未知旗标 · 多余/缺位置参数 · 取值旗标裸给 · `--json` 不给字段（K2 甲档）·
//	   两个互斥旗标同给 · `--timeout` 不是合法时长 · `--base-dir` 不是可用目录 · 不认的契约主号）
//	12 引擎不可达（源件 4）—— 走退码表的 `unreachable`（**可重试**），✗ 不取 4（命令面 `4` = `unauthenticated`）
//	14 目录里已有同摘要但内容不同的记录（源件 3 的冲突档）—— 走退码表的 `conflict`（幂等优先）
//
// 与源件的**照实差异**（六处 · 逐条写清，别处不猜）：
//
//	① 动作名：源件 `verify` / `probe` / `list` ⇒ `model record verify|probe|ls`（理由见上：族内撞名，
//	   `verify` 与 `list` 在 `model` 族里已有**别的语义**）。`--json` 的字段表另见差异 ④。
//	② 退码重取号（源件 `0/1/2/3/4` ⇒ 命令面 `0/1/2/12/14`，逐项见上表）：源件的 `3` 是「参数**或**
//	   环境错误」的合档，命令面把它**拆成三档**（用法错 2 / 读不到写不进 1 / 冲突 14）—— 源件自己在
//	   同一条码里塞了三件事，命令面禁止同数字两义（`P-013` ⑤）。
//	③ 互斥旗标**明拒**（源件静默取一枚、把另一枚当没给）：`--out` 与 `--store` 同给（源件 `--store` 赢、
//	   `--out` 被吞）· `--root` 与 `--manifests` 同给（源件 `--manifests` 赢）· `--in-place` 与
//	   `--integrity` / `--base-dir` 同给（源件 in-place 档整段忽略它们）⇒ 命令面按 `K14` 逐条**明拒** 2
//	   （「给了却被静默吞」比「拒执」更坏）。
//	④ `--json` 面按命令面 `K2` 口径：输出**登记字段表的行** ＋ 标准包封（`emitSelected`），不再是源件
//	   那份「整份 `modelreg.Record` ＋ 能力快照 ＋ 探测留痕 ＋ findings」的裸 JSON。要看整份结构件：
//	   `--out <件>` 落盘后读它（记录的**正文**一字未变 —— 落盘走的仍是源件那条 `WriteFileAtomic`）。
//	⑤ `--strict` 的**形态**：命令面里 `--strict` 是解析层的专用布尔字段（`inv.strict`，`doc` 面 C 档
//	   同款），不在 `inv.kv` 里 ⇒ 本件在入口处**精确点名**一遍（只收自己 usage 里登过的那几枚旗标）；
//	   否则「同族旗标名互为前缀」（`--base` / `--base-dir`）会让子串式的收口把 `--base` 当成已登记
//	   ⇒ 静默吞。本条是**收口形态**的差异，不改变任何一条判词。
//	⑥ 位置参数形态：源件 `verify` 只收一枚记录件（多余的 ⇒ 3）、`probe` 只收一枚目标（多余的 ⇒ 3）；
//	   命令面同一口径但归用法错 **2**（`arity=any` ⇒ 基数由本件判，逐条给判词）。
//
// ★ 双轨期：`core/cmd/zerg-model` 二进制**一字不动**（本件只搬逻辑、不删入口）。它的 `0/1/2/3/4`
// 仍是**它自己**的码空间（组件自有码空间，`exitcode-namespaces.json` 里挂着号）—— 两条轨的码空间
// 不同、不许互换（同 `zerg agent run` 与 `zerg-agent` 的分家口径）。
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/modelreg"
)

// modelRecordVerifyUsage / modelRecordProbeUsage / modelRecordLsUsage —— 用法串
// （与命令树 `usage` 字段**逐字同源**：一处写错两处就分家）。
//
// ★ 三条**各写一行 `const NAME = "…"`**（✗ 不塞进 `const ( … )` 组）：门73 的取数面按
// `const (\w+) = "…"` 逐条现读用法串常量名（非字面量 usage 走这条路）⇒ 组形态它读不到，
// 于是七枚取值旗标会被判成「上了户口却无人可见（多一处）」、`--json` 被判成「声明未列」。
const modelRecordVerifyUsage = "zerg model record verify <记录件> [--strict] [--in-place] [--integrity] [--base-dir <目录>] [--json <字段>]"

const modelRecordProbeUsage = "zerg model record probe <路径|端点URL> [--out <件>] [--store] [--store-root <目录>] [--endpoint <URL>] [--engine <引擎>] [--id <id>] [--timeout <时长>] [--parent <上一版>] [--base <基座 id>] [--json <字段>]"

const modelRecordLsUsage = "zerg model record ls [--root <模型目录根>] [--manifests <目录>] [--json <字段>]"

// modelRecordFlagRE —— 从用法串里抠旗标名（`--x` / `--x-y`）；差异 ⑤ 的精确点名面。
var modelRecordFlagRE = regexp.MustCompile(`--[A-Za-z0-9][A-Za-z0-9-]*`)

// modelRecordDeclared —— 本命令**自己**用法串里登过的旗标集合（精确名，不是子串）。
func modelRecordDeclared(usage string) map[string]bool {
	m := map[string]bool{}
	for _, f := range modelRecordFlagRE.FindAllString(usage, -1) {
		m[f] = true
	}
	return m
}

// modelRecordRejectForeign —— 入口处的**精确**旗标点名（差异 ⑤）。
//
// 为什么需要它：dispatch 的收口用的是 `usageDeclaresFlag`（子串判）⇒ 同族里互为前缀的两枚
// （`--base` / `--base-dir`）会互相「证明」对方已登记；而 `--strict` 根本不在 `inv.kv` 里
// （它是 `inv.strict` 这个专用字段）⇒ 子串收口看不见它。两者都会让「给了却被静默吞」重演。
func modelRecordRejectForeign(inv *invocation, stderr io.Writer, usage string) int {
	decl := modelRecordDeclared(usage)
	var names []string
	for name := range inv.kv {
		if !decl[name] {
			names = append(names, name)
		}
	}
	if inv.strict && !decl["--strict"] {
		names = append(names, "--strict")
	}
	if len(names) == 0 {
		return exitOK
	}
	sort.Strings(names)
	inv.setErr("usage", "flag_not_for_command", "这枚旗标不归本命令")
	fmt.Fprintf(stderr, "%s: 命令 `%s` 不认旗标 %q（本命令的旗标在此一览）\n",
		progName, strings.Join(inv.path, " "), names[0])
	fmt.Fprintf(stderr, "用法: %s\n", usage)
	return exitUsage
}

// modelRecordValue —— 取值旗标的**三态**读法（伞件纪律：给了旗标 ≠ 给了值）。
//
// 返回 (值, 是否可继续)；`!has` ⇒ ("", true)；裸给/空值 ⇒ 判用法错 2 并返回 ("", false)。
func modelRecordValue(inv *invocation, stderr io.Writer, name, hint, usage string) (string, bool) {
	if !inv.hasFlag(name) {
		return "", true
	}
	v := strings.TrimSpace(inv.flagVal(name))
	if v == "" {
		inv.setErr("usage", "flag_value_empty", "取值旗标裸给或空值")
		fmt.Fprintf(stderr, "%s: `%s` 需要一个%s（裸给/空值 ⇒ 不执行）\n", progName, name, hint)
		fmt.Fprintf(stderr, "用法: %s\n", usage)
		return "", false
	}
	return v, true
}

// modelRecordPositional —— 位置参数三态（缺 / 多）：源件两个子命令都只收**一枚**。
func modelRecordPositional(inv *invocation, stderr io.Writer, wantWhat, usage string) (string, int) {
	if len(inv.args) == 0 {
		inv.setErr("usage", "target_missing", "缺位置参数")
		fmt.Fprintf(stderr, "%s: 需要一个%s（不给 ⇒ 不执行）\n", progName, wantWhat)
		fmt.Fprintf(stderr, "用法: %s\n", usage)
		return "", exitUsage
	}
	if len(inv.args) > 1 {
		inv.setErr("usage", "target_extra", "多余的位置参数")
		fmt.Fprintf(stderr, "%s: 只接受一个%s，多余的：%q\n", progName, wantWhat, inv.args[1])
		fmt.Fprintf(stderr, "用法: %s\n", usage)
		return "", exitUsage
	}
	return inv.args[0], exitOK
}

// ---- `zerg model record verify` ------------------------------------------------------------

// cmdModelRecordVerify —— 校验一条模型登记记录（或目录）是否符合标准（源件 `cmdVerify` 的逐面搬）。
//
// 退码：0 合格 / 1 不合格（记录格式或 files[] 完整性不符）/ 2 用法错。
func cmdModelRecordVerify(inv *invocation, stdout, stderr io.Writer) int {
	if rc := modelRecordRejectForeign(inv, stderr, modelRecordVerifyUsage); rc != exitOK {
		return rc
	}
	if inv.jsonGiven {
		if len(inv.fields) == 0 {
			return requireFields(inv, stderr)
		}
		if rc := validateSelected(inv, stderr, inv.path, inv.fields); rc != exitOK {
			return rc
		}
	}
	// `--strict` 是解析层的专用布尔字段（差异 ⑤）；`--integrity` / `--in-place` 是本件刚上户口的布尔旗标。
	strict := inv.strict
	integrity := inv.hasFlag("--integrity")
	inPlace := inv.hasFlag("--in-place")
	baseDir, ok := modelRecordValue(inv, stderr, "--base-dir", "目录", modelRecordVerifyUsage)
	if !ok {
		return exitUsage
	}
	path, rc := modelRecordPositional(inv, stderr, "记录件路径", modelRecordVerifyUsage)
	if rc != exitOK {
		return rc
	}
	// 差异 ③：源件在 in-place 档整段忽略 `--integrity` / `--base-dir`（静默吞）⇒ 命令面明拒。
	if inPlace && integrity {
		inv.setErr("usage", "in_place_conflicts_integrity", "`--in-place` 与 `--integrity` 同给")
		fmt.Fprintf(stderr, "%s: `--in-place` 与 `--integrity` 不能同给 —— in-place 档只核「位置与身份是否自洽」，不做 files[] 完整性核对\n", progName)
		fmt.Fprintf(stderr, "用法: %s\n", modelRecordVerifyUsage)
		return exitUsage
	}
	if inPlace && baseDir != "" {
		inv.setErr("usage", "in_place_conflicts_base_dir", "`--in-place` 与 `--base-dir` 同给")
		fmt.Fprintf(stderr, "%s: `--in-place` 与 `--base-dir` 不能同给 —— in-place 档不收基准目录\n", progName)
		fmt.Fprintf(stderr, "用法: %s\n", modelRecordVerifyUsage)
		return exitUsage
	}
	// `--base-dir` 给错（不是目录）属参数错 ⇒ 2（源件 3；✗ 不静默降级成逐文件 unverifiable）。
	if baseDir != "" {
		if fi, err := os.Stat(baseDir); err != nil || !fi.IsDir() {
			inv.setErr("usage", "base_dir_unusable", "`--base-dir` 不是可用目录")
			fmt.Fprintf(stderr, "%s: `--base-dir` 不是可用目录：%s\n", progName, baseDir)
			return exitUsage
		}
	}
	if inPlace {
		return modelRecordVerifyInPlace(inv, path, strict, stdout, stderr)
	}

	rec, err := modelreg.Load(path)
	if err != nil {
		inv.setErr("failed", "record_unreadable", "记录件读不到")
		fmt.Fprintf(stderr, "✗ 读取失败: %v\n", err)
		return exitFail
	}
	findings := modelreg.Verify(rec, strict)
	nErr := modelreg.CountErrors(findings)

	// `--integrity`：额外核对 files[] 每个文件是否真的与记录一致（流式 sha256 + 大小比对）。
	var irep *modelreg.IntegrityReport
	if integrity {
		r := modelreg.VerifyIntegrity(rec, baseDir)
		irep = &r
	}
	integrityFailed := irep != nil && irep.HasFailures(strict)

	code := exitOK
	failed := nErr > 0 || (strict && len(findings) > 0) || integrityFailed
	if failed {
		// 源件把这一档记成 2「记录不合标准」⇒ 命令面 1（`declaration_rejected`：跑到了、被拒了）。
		inv.setErr("declaration_rejected", "record_not_conforming", "记录不合标准")
		code = exitFail
	}
	verdict := "合格"
	if failed {
		verdict = "不合格"
	}

	if inv.jsonGiven {
		rows := modelRecordVerifyRows(path, rec.Schema, verdict, findings, irep)
		if ec := selectJSONList(stdout, stderr, inv, inv.path, inv.fields, rows); ec != exitOK {
			return ec
		}
		return code
	}

	// 人面（逐字照源件）。
	if len(findings) == 0 {
		fmt.Fprintf(stdout, "✅ 合格：%s（schema=%s）\n", path, rec.Schema)
		if irep == nil {
			fmt.Fprintln(stdout, "   注意：本命令只校验【记录格式】，不校验权重文件本身是否可信（加 --integrity 可一并核对 files[] 与磁盘文件是否一致）")
		}
	} else {
		for _, f := range findings {
			mark := "✗"
			if f.Level == "warn" {
				mark = "⚠️"
			}
			fmt.Fprintf(stdout, "%s [%s] %s：%s\n", mark, f.Level, f.Field, f.Detail)
		}
		fmt.Fprintf(stdout, "—— error %d 条，warn %d 条\n", nErr, len(findings)-nErr)
	}
	if irep != nil {
		modelRecordPrintIntegrity(stdout, *irep, strict)
	}
	return code
}

// modelRecordVerifyRows —— `--json` 面的行（差异 ④）：findings 每条一行 + 完整性每条一行；
// 两者都空 ⇒ 一行「合格」汇总（否则「干净」这一档在机器面上是零行，读起来像没跑）。
func modelRecordVerifyRows(path, schema, verdict string, findings []modelreg.Finding, irep *modelreg.IntegrityReport) []map[string]string {
	rows := []map[string]string{}
	for _, f := range findings {
		rows = append(rows, map[string]string{
			"path": path, "schema": schema, "verdict": verdict,
			"level": f.Level, "field": f.Field, "detail": f.Detail,
		})
	}
	if irep != nil {
		for _, f := range irep.Files {
			rows = append(rows, map[string]string{
				"path": path, "schema": schema, "verdict": verdict,
				"level": "integrity", "field": "files[]", "detail": f.Status,
				"integrity_status": f.Status, "integrity_name": f.Name, "integrity_note": f.Note,
			})
		}
	}
	if len(rows) == 0 {
		rows = append(rows, map[string]string{"path": path, "schema": schema, "verdict": verdict})
	}
	return rows
}

// modelRecordVerifyInPlace —— 源件 `cmdVerifyInPlace` 的逐面搬：记录**落盘位置**与它自己的身份是否自洽。
//
// 诚实边界（源件逐字保留）：digest 只由 files[] 合成，不覆盖 capabilities/license 声明 ⇒ 本命令抓的是
// 「digest 被改」「记录被挪位置/冒充别的 id」；对「声明被改而 digest 与路径都不动」管不到（那是签名，阶段 2）。
func modelRecordVerifyInPlace(inv *invocation, path string, strict bool, stdout, stderr io.Writer) int {
	results, err := modelreg.CheckPlacement(path, strict)
	if err != nil {
		inv.setErr("failed", "record_unreadable", "记录件读不到")
		fmt.Fprintf(stderr, "✗ 读取失败: %v\n", err)
		return exitFail
	}
	if len(results) == 0 {
		if inv.jsonGiven {
			row := map[string]string{"path": path, "verdict": "空", "detail": "该路径下没有任何记录正文"}
			if ec := selectJSONList(stdout, stderr, inv, inv.path, inv.fields, []map[string]string{row}); ec != exitOK {
				return ec
			}
			return exitOK
		}
		fmt.Fprintf(stdout, "✅ 空：%s 下没有任何记录正文（目录不存在，或不含 .json 记录）\n", path)
		return exitOK
	}
	failed := 0
	for _, r := range results {
		if r.Failed(strict) {
			failed++
		}
	}
	if failed > 0 {
		inv.setErr("declaration_rejected", "record_not_conforming", "记录不合标准")
	}
	if inv.jsonGiven {
		rows := make([]map[string]string, 0, len(results))
		for _, r := range results {
			verdict := "合格"
			if r.Failed(strict) {
				verdict = "不合格"
			}
			row := map[string]string{
				"path": r.Path, "id": r.ID, "verdict": verdict,
				"format_errors": strconv.Itoa(r.FormatErrors),
				"format_warns":  strconv.Itoa(r.FormatWarns),
			}
			if r.ReadErr != "" {
				row["level"] = "error"
				row["field"] = "read"
				row["detail"] = r.ReadErr
			} else if fs := r.Findings; len(fs) > 0 {
				row["level"] = fs[0].Level
				row["field"] = fs[0].Field
				row["detail"] = fs[0].Detail
			}
			rows = append(rows, row)
		}
		if ec := selectJSONList(stdout, stderr, inv, inv.path, inv.fields, rows); ec != exitOK {
			return ec
		}
		if failed > 0 {
			return exitFail
		}
		return exitOK
	}
	for _, r := range results {
		switch {
		case r.ReadErr != "":
			fmt.Fprintf(stdout, "✗ [读不动] %s：%s\n", r.Path, r.ReadErr)
		case r.PlacementErrors() == 0:
			fmt.Fprintf(stdout, "✅ 身份自洽：%s（id=%s，文件名 %s）\n", r.Path, r.ID, filepath.Base(r.Path))
		default:
			for _, f := range r.Findings {
				fmt.Fprintf(stdout, "✗ [%s] %s：%s\n", f.Level, f.Field, f.Detail)
			}
		}
		if r.FormatErrors > 0 || r.FormatWarns > 0 {
			fmt.Fprintf(stdout, "   （附：记录格式 error %d / warn %d —— 位置没问题也一并暴露；详见 `zerg model record verify %s`）\n",
				r.FormatErrors, r.FormatWarns, r.Path)
		}
	}
	fmt.Fprintf(stdout, "—— 共 %d 条记录；身份自洽/记录格式不通过 %d 条\n", len(results), failed)
	fmt.Fprintln(stdout, "   说明：本命令只校验「记录位置与身份是否自洽」（阶段 1 廉价篡改检出），不做签名，也不代表权重可信。")
	if failed > 0 {
		return exitFail
	}
	return exitOK
}

// modelRecordPrintIntegrity —— 逐文件完整性结论 + 汇总一行（源件 `printIntegrity` 的逐字搬）。
func modelRecordPrintIntegrity(stdout io.Writer, rep modelreg.IntegrityReport, strict bool) {
	fmt.Fprintln(stdout, "完整性校验（--integrity）：")
	if rep.BaseDir != "" {
		fmt.Fprintf(stdout, "  基准目录：%s\n", rep.BaseDir)
	}
	mark := map[string]string{
		modelreg.IntegrityOK:           "✓",
		modelreg.IntegritySizeMismatch: "✗",
		modelreg.IntegritySHAMismatch:  "✗",
		modelreg.IntegrityMissing:      "✗",
		modelreg.IntegrityUnverifiable: "⚠️",
	}
	for _, f := range rep.Files {
		line := fmt.Sprintf("  %s [%s] role=%s name=%s", mark[f.Status], f.Status, f.Role, f.Name)
		if f.Path != "" {
			line += " path=" + f.Path
		}
		if f.SizeChecked {
			line += fmt.Sprintf(" size=%d/%d", f.SizeRecorded, f.SizeActual)
		} else {
			line += fmt.Sprintf(" size=%d", f.SizeRecorded)
		}
		if f.Status == modelreg.IntegritySHAMismatch {
			line += " sha期望=" + modelRecordShortHash(f.SHAExpected) + " 实际=" + modelRecordShortHash(f.SHAComputed)
		}
		if f.Note != "" {
			line += " —— " + f.Note
		}
		fmt.Fprintln(stdout, line)
	}
	fmt.Fprintf(stdout, "  —— integrity: ok %d 条，不符 %d 条，不可定位 %d 条\n", rep.OK, rep.Problems, rep.Unverifiable)
	if strict && rep.Unverifiable > 0 {
		fmt.Fprintln(stdout, "   （--strict：不可定位视为不通过）")
	}
}

// modelRecordShortHash —— 截短 sha256 便于打印（前 12 位 + 省略号）。
func modelRecordShortHash(s string) string {
	if len(s) > 12 {
		return s[:12] + "…"
	}
	return s
}

// ---- `zerg model record probe` -------------------------------------------------------------

// cmdModelRecordProbe —— 探测模型并生成登记记录（源件 `cmdProbe` 的逐面搬）。
//
// 退码：0 成功且记录合格 / 1 探测失败 · 记录不合标准 · 写不进 / 2 用法错 / 12 引擎不可达 / 14 记录冲突。
func cmdModelRecordProbe(inv *invocation, stdout, stderr io.Writer) int {
	if rc := modelRecordRejectForeign(inv, stderr, modelRecordProbeUsage); rc != exitOK {
		return rc
	}
	if inv.jsonGiven {
		if len(inv.fields) == 0 {
			return requireFields(inv, stderr)
		}
		if rc := validateSelected(inv, stderr, inv.path, inv.fields); rc != exitOK {
			return rc
		}
	}
	useStore := inv.hasFlag("--store")
	var vals = map[string]string{}
	for _, name := range []string{"--out", "--store-root", "--endpoint", "--engine", "--id", "--timeout", "--parent", "--base"} {
		hint := "值"
		switch name {
		case "--out":
			hint = "落件路径"
		case "--store-root":
			hint = "模型目录根"
		case "--endpoint":
			hint = "端点 URL"
		case "--engine":
			hint = "引擎名"
		case "--id":
			hint = "模型 id"
		case "--timeout":
			hint = "时长（如 60s）"
		case "--parent":
			hint = "上一版（version 或 digest）"
		case "--base":
			hint = "基座 id"
		}
		v, ok := modelRecordValue(inv, stderr, name, hint, modelRecordProbeUsage)
		if !ok {
			return exitUsage
		}
		vals[name] = v
	}
	// 差异 ③：源件两处静默胜出 ⇒ 命令面明拒。
	if useStore && vals["--out"] != "" {
		inv.setErr("usage", "out_conflicts_store", "`--out` 与 `--store` 同给")
		fmt.Fprintf(stderr, "%s: `--out` 与 `--store` 不能同给 —— `--store` 进模型目录，`--out` 落到显式件/显式目录，二选一\n", progName)
		fmt.Fprintf(stderr, "用法: %s\n", modelRecordProbeUsage)
		return exitUsage
	}
	// 差异 ③（续）：`--store-root` 只在 `--store` 档有意义（源件里单独给它 = 静默忽略 ⇒ 「吃了却当没吃」）。
	if !useStore && vals["--store-root"] != "" {
		inv.setErr("usage", "store_root_without_store", "`--store-root` 缺 `--store`")
		fmt.Fprintf(stderr, "%s: `--store-root` 只在 `--store` 档有意义（单独给 ⇒ 不执行 —— 要落进模型目录就加 `--store`；要落到显式位置用 `--out`）\n", progName)
		fmt.Fprintf(stderr, "用法: %s\n", modelRecordProbeUsage)
		return exitUsage
	}
	target, rc := modelRecordPositional(inv, stderr, "目标（本地路径或端点 URL）", modelRecordProbeUsage)
	if rc != exitOK {
		return rc
	}
	timeout := modelreg.DefaultProbeTimeout
	if txt := vals["--timeout"]; txt != "" {
		d, err := time.ParseDuration(txt)
		if err != nil {
			inv.setErr("usage", "timeout_not_duration", "`--timeout` 不是合法时长")
			fmt.Fprintf(stderr, "%s: `--timeout` 不是合法时长：%v\n", progName, err)
			fmt.Fprintf(stderr, "用法: %s\n", modelRecordProbeUsage)
			return exitUsage
		}
		timeout = d
	}

	rec, rep, err := modelreg.Probe(modelreg.ProbeOptions{
		Target:    target,
		Endpoint:  vals["--endpoint"],
		Engine:    vals["--engine"],
		ID:        vals["--id"],
		Timeout:   timeout,
		Parent:    vals["--parent"], // 血缘：上一版（version 或 digest）
		BaseModel: vals["--base"],   // 血缘：基座 id
	})
	if err != nil {
		var un *modelreg.UnreachableError
		if errors.As(err, &un) {
			// 源件 4「引擎不可达」⇒ 命令面 12 `unreachable`（可重试；✗ 不取 4 —— 那是 `unauthenticated`）。
			inv.setErr("unreachable", "engine_unreachable", "引擎不可达")
			fmt.Fprintf(stderr, "✗ 引擎不可达：%v\n", err)
			return exitCodeOf("unreachable")
		}
		inv.setErr("failed", "probe_failed", "探测失败")
		fmt.Fprintf(stderr, "✗ 探测失败：%v\n", err)
		return exitFail
	}

	recJSON, _ := json.MarshalIndent(rec, "", "  ")
	findings := modelreg.Verify(rec, false)
	nErr := modelreg.CountErrors(findings)
	nWarn := len(findings) - nErr

	// 留痕到 stderr（对应标准 §九「三样证据」）—— 逐字照源件。
	fmt.Fprintf(stderr, "探测目标：%s\n", rep.Target)
	if rep.Endpoint != "" {
		fmt.Fprintf(stderr, "端点：%s（在线探测%s）\n", rep.Endpoint, map[bool]string{true: "已执行", false: "未执行"}[rep.OnlineProbed])
	}
	fmt.Fprintln(stderr, "探测器留痕：")
	for _, t := range rep.Traces {
		mark := "✓"
		if !t.OK {
			mark = "✗"
		}
		line := fmt.Sprintf("  %s %s ok=%v status=%d ms=%d", mark, t.Probe, t.OK, t.HTTPStatus, t.ElapsedMS)
		if t.Budget > 0 {
			line += fmt.Sprintf(" budget=%d", t.Budget)
		}
		if t.FailureClass != "" {
			line += " class=" + t.FailureClass
		}
		if t.Summary != "" {
			line += " resp=" + t.Summary
		}
		fmt.Fprintln(stderr, line)
	}
	fmt.Fprintf(stderr, "生成记录：id=%s digest=%s files=%d capabilities_snapshot=%d\n", rec.ID, rec.Digest, len(rec.Files), len(rep.Capabilities))
	fmt.Fprintf(stderr, "自校验：error %d 条，warn %d 条\n", nErr, nWarn)
	for _, f := range findings {
		mark := "✗"
		if f.Level == "warn" {
			mark = "⚠️"
		}
		fmt.Fprintf(stderr, "%s [%s] %s：%s\n", mark, f.Level, f.Field, f.Detail)
	}

	// 落盘目标：`--store`，或 `--out` 给的是目录 ⇒ 进模型目录；`--out` 给文件 ⇒ 原子写该文件。
	out := vals["--out"]
	storeDir := ""
	if useStore {
		storeDir = modelreg.NewStore(vals["--store-root"]).ManifestsDir()
	} else if out != "" && modelRecordLooksLikeDir(out) {
		storeDir = strings.TrimRight(out, "/")
	}

	written := ""
	changed := false
	row := func() map[string]string {
		verdict := "合格"
		if nErr > 0 {
			verdict = "不合格"
		}
		r := map[string]string{
			"id": rec.ID, "version": modelreg.VersionOf(rec), "digest": rec.Digest,
			"files": strconv.Itoa(len(rec.Files)), "capabilities": strconv.Itoa(len(rep.Capabilities)),
			"errors": strconv.Itoa(nErr), "warnings": strconv.Itoa(nWarn),
			"target": rep.Target, "endpoint": rep.Endpoint, "verdict": verdict,
		}
		if v := vals["--engine"]; v != "" {
			r["engine"] = v
		}
		if written != "" {
			r["out"] = written
			r["changed"] = strconv.FormatBool(changed)
		}
		return r
	}

	if !inv.jsonGiven && out == "" && storeDir == "" {
		// 默认只打印，不写盘（源件逐字）。
		fmt.Fprintln(stdout, string(recJSON))
	}

	if out != "" && storeDir == "" {
		c, werr := modelreg.WriteFileAtomic(out, append(recJSON, '\n'))
		if werr != nil {
			inv.setErr("failed", "record_write_failed", "记录件写不进")
			fmt.Fprintf(stderr, "✗ 写记录失败: %v\n", werr)
			return exitFail
		}
		written, changed = out, c
		if c {
			fmt.Fprintf(stderr, "已写入：%s\n", out)
		} else {
			fmt.Fprintf(stderr, "内容一致，未重写：%s\n", out)
		}
		// 易变留痕与能力快照另写兄弟文件（记录正文只装身份 ⇒ 正文逐字节确定）。
		modelRecordWriteTraceSibling(stderr, out, rec, rep)
		modelRecordWriteCapabilitySibling(stderr, out, rec, rep)
	}

	code := exitOK
	if storeDir != "" {
		res, perr := modelreg.NewStoreAtManifests(storeDir).Put(rec, false)
		var adm *modelreg.AdmissionError
		var conf *modelreg.ConflictError
		switch {
		case perr == nil && res.Changed:
			written, changed = res.Path, true
			fmt.Fprintf(stderr, "已入目录：%s（version=%s）\n", res.Path, res.Version)
		case perr == nil:
			written, changed = res.Path, false
			fmt.Fprintf(stderr, "目录里已有同内容记录，未重写：%s（version=%s）\n", res.Path, res.Version)
		case errors.As(perr, &adm):
			// 门禁如实拒绝（标准 §十二.1「verify 不过直接拒」）：不降级、不写占位值凑绿。
			inv.setErr("declaration_rejected", "record_not_conforming", "记录不合标准")
			fmt.Fprintf(stderr, "✗ 拒绝入目录：%s\n", perr)
			for _, f := range adm.Findings {
				fmt.Fprintf(stderr, "   [%s] %s：%s\n", f.Level, f.Field, f.Detail)
			}
			fmt.Fprintf(stderr, "   （未写入任何文件；按上面每条 error 修好记录后重跑）\n")
			code = exitFail
		case errors.As(perr, &conf):
			// 源件把冲突记 3；命令面走退码表 `conflict`（14 · 幂等优先）—— 差异 ②。
			inv.setErr("conflict", "record_conflict", "同摘要不同内容")
			fmt.Fprintf(stderr, "✗ %v\n", perr)
			code = exitCodeOf("conflict")
		default:
			inv.setErr("failed", "record_write_failed", "记录件写不进")
			fmt.Fprintf(stderr, "✗ 写模型目录失败：%v\n", perr)
			code = exitFail
		}
		if perr == nil {
			modelRecordWriteTraceSibling(stderr, res.Path, rec, rep)
			modelRecordWriteCapabilitySibling(stderr, res.Path, rec, rep)
		}
	}

	if inv.jsonGiven {
		if ec := selectJSON(stdout, stderr, inv, inv.path, inv.fields, row()); ec != exitOK {
			return ec
		}
	}
	// 源件末段：记录自校验有 error ⇒ 2（不合标准）—— 命令面 1（差异 ②）。
	if nErr > 0 {
		inv.setErr("declaration_rejected", "record_not_conforming", "记录不合标准")
		return exitFail
	}
	return code
}

// modelRecordWriteTraceSibling —— 探测留痕（生成时间 / 每项探测器耗时 / 原始响应片段）
// 写成记录旁的兄弟文件 `<版>.trace.json`（源件 `writeTraceSibling` 的逐字搬）。
//
// 易变信息绝不进记录正文（正文才是内容寻址的锚）⇒ 写失败只告警，不改变「记录是否入库」的结论。
func modelRecordWriteTraceSibling(stderr io.Writer, recordPath string, rec *modelreg.Record, rep *modelreg.ProbeReport) {
	p, err := modelreg.WriteProbeTraceSibling(recordPath, rec, rep)
	if err != nil {
		fmt.Fprintf(stderr, "⚠️ 留痕兄弟文件写入失败：%v\n", err)
		return
	}
	fmt.Fprintf(stderr, "留痕已写：%s\n", p)
}

// modelRecordWriteCapabilitySibling —— 能力快照写成 `<版>.capabilities.json`（源件同函数逐字搬）。
//
// 能力快照是「它现在能干什么」，**允许**随探测刷新 ⇒ 写失败只告警；记录正文一字未动。
func modelRecordWriteCapabilitySibling(stderr io.Writer, recordPath string, rec *modelreg.Record, rep *modelreg.ProbeReport) {
	p, err := modelreg.WriteCapabilitySnapshot(recordPath, rec, rep)
	if err != nil {
		fmt.Fprintf(stderr, "⚠️ capability snapshot write failed: %v\n", err)
		return
	}
	fmt.Fprintf(stderr, "capability snapshot written: %s\n", p)
}

// modelRecordLooksLikeDir —— `--out` 给的是目录还是文件（源件 `looksLikeDir`）：
//   - 以 "/" 结尾 → 目录（可以是还没创建的目录）
//   - 已存在的目录 → 目录
//
// 其余一律当文件路径。
func modelRecordLooksLikeDir(p string) bool {
	if strings.HasSuffix(p, "/") {
		return true
	}
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

// ---- `zerg model record ls` ---------------------------------------------------------------

// cmdModelRecordLs —— 列出模型目录里的记录（源件 `cmdList` 的逐面搬）。
//
// 退码：0（目录里 error 0 条）/ 1（目录里有不合标准的记录 —— 标准 §十二.1 不该留在目录里）/ 2 用法错。
func cmdModelRecordLs(inv *invocation, stdout, stderr io.Writer) int {
	if rc := modelRecordRejectForeign(inv, stderr, modelRecordLsUsage); rc != exitOK {
		return rc
	}
	if inv.jsonGiven {
		if len(inv.fields) == 0 {
			return requireFields(inv, stderr)
		}
		if rc := validateSelected(inv, stderr, inv.path, inv.fields); rc != exitOK {
			return rc
		}
	}
	if len(inv.args) > 0 {
		inv.setErr("usage", "target_extra", "多余的位置参数")
		fmt.Fprintf(stderr, "%s: `model record ls` 不吃位置参数（给的是 %q）\n", progName, inv.args[0])
		fmt.Fprintf(stderr, "用法: %s\n", modelRecordLsUsage)
		return exitUsage
	}
	root, ok := modelRecordValue(inv, stderr, "--root", "模型目录根", modelRecordLsUsage)
	if !ok {
		return exitUsage
	}
	manifests, ok := modelRecordValue(inv, stderr, "--manifests", "manifests 目录", modelRecordLsUsage)
	if !ok {
		return exitUsage
	}
	// 差异 ③：源件 `--manifests` 静默胜出 ⇒ 命令面明拒。
	if root != "" && manifests != "" {
		inv.setErr("usage", "root_conflicts_manifests", "`--root` 与 `--manifests` 同给")
		fmt.Fprintf(stderr, "%s: `--root` 与 `--manifests` 不能同给 —— 前者是模型目录根（记录在 <根>/manifests/），后者直接指 manifests 目录\n", progName)
		fmt.Fprintf(stderr, "用法: %s\n", modelRecordLsUsage)
		return exitUsage
	}

	st := modelreg.NewStore(root)
	if manifests != "" {
		st = modelreg.NewStoreAtManifests(manifests)
	}
	rows, err := st.List()
	if err != nil {
		inv.setErr("failed", "manifests_unreadable", "模型目录读不到")
		fmt.Fprintf(stderr, "✗ 读取模型目录失败: %v\n", err)
		return exitFail
	}

	totalErr, totalWarn, notEligible := 0, 0, 0
	for _, r := range rows {
		totalErr += r.Errors
		totalWarn += r.Warns
		if !r.DefaultEligible {
			notEligible++
		}
	}
	code := exitOK
	if totalErr > 0 {
		// 源件 2「目录里有不合标准的记录」⇒ 命令面 1（`declaration_rejected`）。
		inv.setErr("declaration_rejected", "record_not_conforming", "记录不合标准")
		code = exitFail
	}

	if inv.jsonGiven {
		out := make([]map[string]string, 0, len(rows))
		for _, r := range rows {
			row := map[string]string{
				"id": r.ID, "version": r.Version, "files": strconv.Itoa(r.Files),
				"errors": strconv.Itoa(r.Errors), "warns": strconv.Itoa(r.Warns),
				"path": r.Path, "default_eligible": strconv.FormatBool(r.DefaultEligible),
			}
			if r.Commercial != "" {
				row["commercial"] = r.Commercial
			}
			if len(r.Capabilities) > 0 {
				row["capabilities"] = strings.Join(r.Capabilities, ",")
			}
			if r.Err != "" {
				row["detail"] = r.Err
			}
			out = append(out, row)
		}
		if ec := selectJSONList(stdout, stderr, inv, inv.path, inv.fields, out); ec != exitOK {
			return ec
		}
		return code
	}

	fmt.Fprintf(stdout, "模型目录（manifests）：%s\n", st.ManifestsDir())
	if len(rows) == 0 {
		fmt.Fprintln(stdout, "（空目录：还没有任何记录。用 `zerg model record probe <路径> --store` 入目录）")
		return code
	}
	fmt.Fprintf(stdout, "%-28s %-16s %-13s %-6s %-24s %-8s %s\n", "ID", "VERSION", "COMMERCIAL", "FILES", "CAPABILITIES", "ERR/WARN", "PATH")
	for _, r := range rows {
		caps := strings.Join(r.Capabilities, ",")
		if caps == "" {
			caps = "-"
		}
		fmt.Fprintf(stdout, "%-28s %-16s %-13s %-6d %-24s %d/%-6d %s\n", r.ID, r.Version, modelRecordOrDash(r.Commercial), r.Files, caps, r.Errors, r.Warns, r.Path)
		if r.Err != "" {
			fmt.Fprintf(stdout, "%-28s %s\n", "", "读取失败："+r.Err)
		}
	}
	fmt.Fprintf(stdout, "—— 共 %d 条记录；error %d / warn %d；commercial != yes 的 %d 条不得作为默认项（标准 §五 红线）\n", len(rows), totalErr, totalWarn, notEligible)
	if totalErr > 0 {
		fmt.Fprintln(stdout, "   目录里有不合标准的记录（标准 §十二.1 不该留在目录里）——需人工处置")
	}
	return code
}

// modelRecordOrDash —— 空值显示成 "-"（源件 `orDash`）。
func modelRecordOrDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
