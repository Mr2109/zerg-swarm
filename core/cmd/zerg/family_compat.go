// family_compat.go —— `zerg compat list|check|migrate|manifest`（跨版本状态文件兼容层 · **归入命令面** · 批 B7）。
//
// 源件 = 独立入口 `core/cmd/zerg-compat/main.go`（243 行 · 四个子命令 `list` / `check` / `migrate` /
// `manifest`）。本件是那一件的**逐面搬**：四子命令的旗标解析、人面判词、逐条输出、清单原文字节、
// 目录解析口径（`ZERG_STATE_DIR` / `ZERG_RECEIPTS_DIR`，或 `--state-dir` / `--receipts-dir`）逐字照源件；
// 判定与落盘一行不改 —— 仍是 `core/internal/compat` 的 `Entries` / `StatusAll` / `MigrateAll` /
// `ManifestJSONForDump`（本件只做**取数与输出**，不重写任何一条判定）。
//
// 形态：
//
//	zerg compat list     [--state-dir <目录>] [--receipts-dir <目录>] [--json <字段>]
//	zerg compat check    [--state-dir <目录>] [--receipts-dir <目录>] [--json <字段>]
//	zerg compat migrate  [--state-dir <目录>] [--receipts-dir <目录>] [--dry-run] [--json <字段>]
//	zerg compat manifest
//
// 为什么**新开顶层族** `compat`（照实记一笔，✗ 不粉饰派单里的「并入既有族」）：
// 派单原话是「并入**既有** `compat` 族」，而**现读没有这个族** —— `zerg help compat` 逐字
// 「未知帮助主题」· `zerg help --all` 里 `compat` 零命中 · `core/cmd/zerg/main.go` 里 `compat` 零命中
// （命令树 280 条逐个 path 现读）⇒「既有族」不存在，无处可并。三条可选形态里取「族名 = 源件名的主体」：
//
//	① 并到 `build` 族 —— `build --only compat` 是**构建档名**（编那个二进制），与本面
//	   （查/迁**状态文件**）语义完全不同 ⇒ 撞名比新开族更坏；
//	② 并到 `doctor` / `gate` 族 —— 本面是**状态文件读写正门**，不是体检、也不是判据门；
//	③ 新开 `compat` 族 —— 族名与源件同名（人机两边一眼对得上），四动作逐字照源件。⇒ 取 ③。
//
// 退码（★ 与源件的码空间**不同** —— 源件是**组件自有码空间**，命令面读 `exitcodes.go` 的唯一真源）：
//
//	0  就绪 / 迁移成功 / `list`·`manifest` 出表 / `migrate --dry-run` 报得出计划
//	1  跑到了但没成功：状态文件读不动（`corrupt`）· 迁移失败（源件 1）
//	2  用法错（未知旗标 · 多余位置参数 · 取值旗标裸给 · `--json` 不给字段（K2 甲档）·
//	   旗标不归本子命令 · 不给动作）
//	8  有前置未满足（`blocked`）：存在待迁移文件（源件 **2**）· 存在高于本机认知的 schema（源件 **3**）
//	   —— 两档以 `error.detail` 分家（`compat_pending_migration` / `compat_future_schema`）
//
// 与源件的**照实差异**（六处 · 逐条写清，别处不猜）：
//
//	① 族形态：源件是**独立二进制的四子命令**；命令面**新开顶层族 `compat`**（理由见上）。`zerg compat`
//	   裸给（不给动作）在源件里是 `usage()` + 退 **4**；命令面走族群节点的既有判词 + 退 **2**
//	   （`exitUsage`）—— 码值变、语义同（「没给动作 ⇒ 不给结论」）。
//	② 退码重取号（源件 `0/1/2/3/4` ⇒ 命令面 `0/1/2/8/8`，逐项见上表）：源件的 `4`（用法错）与命令面
//	   `4`（`unauthenticated`）**撞号**，`2`（待迁移）与命令面 `2`（`usage`）**撞号** ⇒ 一律回表取码
//	   （`exitCodeOf`），✗ 不就地改数字。★ 源件头注那句「值面对齐要先给自有语义重新取号（`P-013` ⑨ 的
//	   拍板 · 批 C 的 T-26）」**今天仍未拍** ⇒ 本件**只做映射**，不动那张表、不给命令面新开码。
//	③ `--json` 面按命令面 `K2` 口径：输出**登记字段表的行** ＋ 标准包封（`emitSelected`），不再是源件
//	   那份「整份 `[]compat.Entry` / `[]FileStatus` / `[]Report` 的裸 JSON」。要看整份结构件：
//	   `zerg compat manifest` 打清单原文；状态面逐行由 `--json` 的字段表点出来。
//	④ `manifest` **不收任何旗标**（源件把四枚旗标照收不误、在 manifest 档一律**静默忽略**）⇒ 命令面按
//	   `K14` 明拒 2（「给了却被静默吞」比「拒执」更坏）。
//	⑤ 位置参数一律拒（四个动作都 `arity: none`）：源件 `flag.Parse` 之后的多余位置参数**静默忽略**。
//	⑥ `migrate --dry-run` 的退码：源件**恒 0**（哪怕盘上有读不动的状态文件 —— 那是假绿）；命令面按
//	   「读不到不许当健康」判 —— 有 `corrupt` ⇒ 1，否则 0（**待迁移不判 8**：dry-run 的产出正是
//	   「哪些要迁」，那是它的正常结论，不是「被前置挡住」）。
//
// ★ 双轨期：`core/cmd/zerg-compat` 二进制**一字不动**（本件只搬逻辑、不删入口）。它的 `0/1/2/3/4`
// 仍是**它自己**的码空间（`exitcode-namespaces.json` 里挂着号）—— 两条轨的码空间不同、不许互换
// （同 `zerg model record` 与 `zerg-model` 的分家口径）。
package main

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/Mr2109/zerg-swarm/core/internal/compat"
)

// 四条用法串（与命令树 `usage` 字段**逐字同源**：一处写错两处就分家）。
//
// ★ 四条各写一行 `const NAME = "…"`（✗ 不塞进 `const ( … )` 组）：门73 的取数面按
// `const (\w+) = "…"` 逐条现读用法串常量名（非字面量 usage 走这条路）⇒ 组形态它读不到，
// 于是 `--state-dir` / `--receipts-dir` 会被判成「上了户口却无人可见（多一处）」。
const compatListUsage = "zerg compat list [--state-dir <目录>] [--receipts-dir <目录>] [--json <字段>]"

const compatCheckUsage = "zerg compat check [--state-dir <目录>] [--receipts-dir <目录>] [--json <字段>]"

const compatMigrateUsage = "zerg compat migrate [--state-dir <目录>] [--receipts-dir <目录>] [--dry-run] [--json <字段>]"

const compatManifestUsage = "zerg compat manifest"

// compatFlagAllowed —— 本子命令的旗标**精确**白名单（入口处点名用）。
//
// 为什么不用 `usageDeclaresFlag`（子串判）：`--state-dir` 与 `--state` 互为前缀、
// `--receipts-dir` 与 `--repo` 无关但同族旗标名还会互相「证明」对方已登记 ⇒ 子串收口会静默吞
// （同 `family_model_record.go:modelRecordRejectForeign` 那条先例）。
func compatFlagAllowed(inv *invocation, stderr io.Writer, usage string, own ...string) int {
	decl := make(map[string]bool, len(own))
	for _, n := range own {
		decl[n] = true
	}
	var names []string
	for n := range inv.kv {
		if !decl[n] {
			names = append(names, n)
		}
	}
	// `--json` / `--dry-run` 是**解析层专用字段**（`inv.jsonGiven` / `inv.dryRun`），不在 `inv.kv` 里
	// ⇒ 子串式收口看不见它们，本处逐条点名。
	if inv.jsonGiven && !decl["--json"] {
		names = append(names, "--json")
	}
	if inv.dryRun && !decl["--dry-run"] {
		names = append(names, "--dry-run")
	}
	if len(names) == 0 {
		return exitOK
	}
	sort.Strings(names) // 判词确定性：map 的乱序不许决定报哪一枚
	inv.setErr("usage", "flag_not_for_command", "这枚旗标不归本命令")
	fmt.Fprintf(stderr, "%s: 命令 `%s` 不认旗标 %q（本命令的旗标一律在下面这行用法里）\n",
		progName, strings.Join(inv.path, " "), names[0])
	fmt.Fprintf(stderr, "用法: %s\n", usage)
	return exitUsage
}

// compatValue —— 取值旗标的**三态**读法（伞件纪律：给了旗标 ≠ 给了值）。
//
// 返回 (值, 是否可继续)；`!has` ⇒ ("", true)；裸给/空值 ⇒ 判用法错 2 并返回 ("", false)。
func compatValue(inv *invocation, stderr io.Writer, name, hint, usage string) (string, bool) {
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

// compatConfig —— 入口收口 → `compat.Config`（目录覆盖只走显式旗标；缺省照源件 = DefaultConfig）。
//
// 目录**不写死**：源件那句「本命令会打印它将要操作的目录 —— 不写死私有路径，也不猜用户的家目录」
// 逐字保留（`list` / `check` / `migrate` 人面都先打两行目录）。
func compatConfig(inv *invocation, stderr io.Writer, usage string, own ...string) (compat.Config, int) {
	if rc := compatFlagAllowed(inv, stderr, usage, own...); rc != exitOK {
		return compat.Config{}, rc
	}
	stateDir, ok := compatValue(inv, stderr, "--state-dir", "状态目录", usage)
	if !ok {
		return compat.Config{}, exitUsage
	}
	receiptsDir, ok := compatValue(inv, stderr, "--receipts-dir", "升级回执目录", usage)
	if !ok {
		return compat.Config{}, exitUsage
	}
	cfg := compat.DefaultConfig()
	if stateDir != "" {
		cfg.Dirs.State = stateDir
	}
	if receiptsDir != "" {
		cfg.Dirs.Receipts = receiptsDir
	}
	return cfg, exitOK
}

// compatJSONReady —— `--json` 面两档闸（K2 甲：不给字段 ⇒ **2**（`requireFields` 回 `usage` 档，
// 与矩阵里 `model/record/*#--json 不给字段` 那几格的现跑读数一致）；点错字段 ⇒ 2）。
func compatJSONReady(inv *invocation, stderr io.Writer) int {
	if !inv.jsonGiven {
		return exitOK
	}
	if len(inv.fields) == 0 {
		return requireFields(inv, stderr)
	}
	return validateSelected(inv, stderr, inv.path, inv.fields)
}

// ---- `zerg compat list` ---------------------------------------------------------------------

// cmdCompatList —— 列出清单（文件 → 当前 schema → 最低可读 → 信封 → 归属）。只读。
//
// 退码：0（清单是**编译进二进制**的资产，读不到就是构建错 —— 源件同档）。
func cmdCompatList(inv *invocation, stdout, stderr io.Writer) int {
	cfg, rc := compatConfig(inv, stderr, compatListUsage, "--state-dir", "--receipts-dir", "--json")
	if rc != exitOK {
		return rc
	}
	if rc := compatJSONReady(inv, stderr); rc != exitOK {
		return rc
	}
	entries := compat.Entries()
	if inv.jsonGiven {
		rows := make([]map[string]string, 0, len(entries))
		for _, e := range entries {
			rows = append(rows, map[string]string{
				"name":           e.Name,
				"file":           e.File,
				"dir":            e.Dir,
				"current_schema": strconv.Itoa(e.CurrentSchema),
				"min_readable":   strconv.Itoa(e.MinReadable),
				"envelope":       e.Envelope,
				"stamps_schema":  e.StampsSchema,
				"owner":          e.Owner,
			})
		}
		return selectJSONList(stdout, stderr, inv, inv.path, inv.fields, rows)
	}
	// 人面逐字照源件（列宽 / 表头 / 末尾那行 excluded 计数都不动）。
	fmt.Fprintf(stdout, "状态目录: %s\n回执目录: %s\n\n", cfg.Dirs.State, cfg.Dirs.Receipts)
	fmt.Fprintf(stdout, "%-20s %-28s %-7s %-7s %-8s %-7s %s\n", "名称", "文件", "当前", "最低读", "信封", "盖版本", "归属")
	for _, e := range entries {
		fmt.Fprintf(stdout, "%-20s %-28s %-7d %-7d %-8s %-7s %s\n",
			e.Name, e.File, e.CurrentSchema, e.MinReadable, e.Envelope, e.StampsSchema, e.Owner)
	}
	fmt.Fprintf(stdout, "\n（明确不登记：%d 项——理由见 core/internal/compat/compat.json 的 excluded）\n", len(compat.MustManifest().Excluded))
	return exitOK
}

// ---- `zerg compat check` --------------------------------------------------------------------

// compatVerdict —— 逐文件结论 → 退码（优先级：失败 1 > 更高版本 8 > 待迁移 8 > 就绪 0）。
//
// 为什么「失败」压过一切：一个读不出来的状态文件是**眼下就要处理的**（源件同一句理由）。
// 结论 → 退码与「哪一支 8」（`compatSetVerdictErr` 负责把 detail 以**字面量**落进 `inv.setErr`）。
type compatVerdictResult struct {
	code   int  // 退码（0 / 1 / 8）
	future bool // 8 里是不是「更高版本 schema」那一支（人面结论与 error.detail 都要分家）
}

func compatVerdict(statuses []compat.FileStatus) compatVerdictResult {
	corrupt, future, pending := false, false, false
	for _, st := range statuses {
		switch st.Outcome {
		case compat.OutcomeCorrupt:
			corrupt = true
		case compat.OutcomeFuture:
			future = true
		case compat.OutcomeMigrated:
			pending = true
		}
	}
	switch {
	case corrupt:
		return compatVerdictResult{code: exitFail}
	case future:
		// 源件 3「出现更高版本 schema」⇒ 命令面 `blocked`（8）—— 前置未满足（要升级本机二进制）。
		return compatVerdictResult{code: exitBlocked, future: true}
	case pending:
		// 源件 2「有待迁移文件」⇒ 命令面 `blocked`（8）—— 前置未满足（先跑 migrate 或让主控启动时自动迁移）。
		return compatVerdictResult{code: exitBlocked}
	}
	return compatVerdictResult{code: exitOK}
}

// compatSetVerdictErr —— 把结论落到 `inv.setErr`（**detail 一律就地写字符串字面量**）。
//
// 为什么不让 `compatVerdict` 把 detail 一起返回：门⑪ 的 `error.detail` 登记面按 `setErr` 的
// **字面量实参**取数 —— 从变量传进来的 detail 不进登记表 ⇒ 那两档 8 在审计面上「查不到」。
func compatSetVerdictErr(inv *invocation, v compatVerdictResult) {
	switch {
	case v.code == exitFail:
		inv.setErr("failed", "compat_state_corrupt", "存在不可解析/不可用的状态文件")
	case v.code == exitBlocked && v.future:
		inv.setErr("blocked", "compat_future_schema", "存在高于本机认知的 schema（请升级本机二进制）")
	case v.code == exitBlocked:
		inv.setErr("blocked", "compat_pending_migration", "存在待迁移文件")
	}
}

// cmdCompatCheck —— 只读体检：逐文件报现状。**不迁移、不备份、不写盘**（源件 `StatusAll` 同一条）。
func cmdCompatCheck(inv *invocation, stdout, stderr io.Writer) int {
	cfg, rc := compatConfig(inv, stderr, compatCheckUsage, "--state-dir", "--receipts-dir", "--json")
	if rc != exitOK {
		return rc
	}
	if rc := compatJSONReady(inv, stderr); rc != exitOK {
		return rc
	}
	statuses := compat.StatusAll(cfg)
	v := compatVerdict(statuses)
	compatSetVerdictErr(inv, v)
	code := v.code
	if inv.jsonGiven {
		rows := make([]map[string]string, 0, len(statuses))
		for _, st := range statuses {
			rows = append(rows, compatStatusRow(st))
		}
		if rc := selectJSONList(stdout, stderr, inv, inv.path, inv.fields, rows); rc != exitOK {
			return rc
		}
		return code
	}
	fmt.Fprintf(stdout, "状态目录: %s\n回执目录: %s\n\n", cfg.Dirs.State, cfg.Dirs.Receipts)
	fmt.Fprintf(stdout, "%-20s %-52s %-9s %s\n", "名称", "文件", "结论", "磁盘版本")
	for _, st := range statuses {
		ver := "—"
		if st.OnDisk >= 0 {
			ver = fmt.Sprint(st.OnDisk)
		}
		line := fmt.Sprintf("%-20s %-52s %-9s %s", st.Name, st.Path, st.Outcome, ver)
		if st.Err != "" {
			line += "  ⚠ " + st.Err
		}
		fmt.Fprintln(stdout, line)
	}
	fmt.Fprintln(stdout)
	switch code {
	case exitFail:
		fmt.Fprintln(stdout, "结论：存在不可解析/不可用的状态文件（见上面的 ⚠）")
	case exitBlocked:
		if v.future {
			fmt.Fprintln(stdout, "结论：存在高于本机二进制认知的 schema —— 请升级本机二进制（不猜、不回写）")
		} else {
			fmt.Fprintln(stdout, "结论：存在待迁移文件（跑 zerg compat migrate，或让主控启动时自动迁移）")
		}
	default:
		fmt.Fprintln(stdout, "结论：全部就绪")
	}
	return code
}

// compatStatusRow —— `check` / `migrate --dry-run` 的 `--json` 行（同一形状：两档都点得出来）。
func compatStatusRow(st compat.FileStatus) map[string]string {
	row := map[string]string{
		"name":           st.Name,
		"path":           st.Path,
		"outcome":        string(st.Outcome),
		"on_disk_schema": strconv.Itoa(st.OnDisk),
		"current_schema": strconv.Itoa(st.Expected),
		"min_readable":   strconv.Itoa(st.MinReadable),
		"envelope":       st.Envelope,
		"owner":          st.Owner,
	}
	if st.Err != "" {
		row["error"] = st.Err
	}
	return row
}

// ---- `zerg compat migrate` ------------------------------------------------------------------

// cmdCompatMigrate —— 执行一次性迁移（幂等；可反复跑）。`--dry-run` 只报计划、不落盘。
//
// 退码：0 迁移成功（或幂等短路）/ 1 有失败（源件同档）/ 2 用法错；`--dry-run` ⇒ 0（有 corrupt ⇒ 1）。
func cmdCompatMigrate(inv *invocation, stdout, stderr io.Writer) int {
	cfg, rc := compatConfig(inv, stderr, compatMigrateUsage,
		"--state-dir", "--receipts-dir", "--dry-run", "--json")
	if rc != exitOK {
		return rc
	}
	if rc := compatJSONReady(inv, stderr); rc != exitOK {
		return rc
	}
	if inv.dryRun {
		return compatMigrateDry(inv, cfg, stdout, stderr)
	}
	reports := compat.MigrateAll(cfg)
	corrupt := 0
	for _, r := range reports {
		if r.Outcome == compat.OutcomeCorrupt {
			corrupt++
		}
	}
	if corrupt > 0 {
		inv.setErr("failed", "compat_migrate_failed", "迁移失败")
	}
	if inv.jsonGiven {
		rows := make([]map[string]string, 0, len(reports))
		for _, r := range reports {
			row := map[string]string{"name": r.Name, "path": r.Path, "outcome": string(r.Outcome)}
			if r.Err != "" {
				row["error"] = r.Err
			}
			rows = append(rows, row)
		}
		if rc := selectJSONList(stdout, stderr, inv, inv.path, inv.fields, rows); rc != exitOK {
			return rc
		}
		if corrupt > 0 {
			return exitFail
		}
		return exitOK
	}
	fmt.Fprintf(stdout, "状态目录: %s\n回执目录: %s\n\n", cfg.Dirs.State, cfg.Dirs.Receipts)
	var migrated, current, missing, corruptN int
	for _, r := range reports {
		switch r.Outcome {
		case compat.OutcomeMigrated:
			migrated++
			fmt.Fprintf(stdout, "  ✅ 已迁移  %s\n", r.Path)
		case compat.OutcomeCurrent:
			current++
		case compat.OutcomeMissing:
			missing++
		case compat.OutcomeFuture:
			fmt.Fprintf(stdout, "  ⏫ 跳过    %s（schema 高于本机认知——请升级本机二进制）\n", r.Path)
		default:
			corruptN++
			fmt.Fprintf(stdout, "  ❌ 失败    %s：%s\n", r.Path, r.Err)
		}
	}
	fmt.Fprintf(stdout, "\n迁移 %d · 已是当前 %d · 不存在 %d · 失败 %d\n", migrated, current, missing, corruptN)
	if corruptN > 0 {
		return exitFail
	}
	return exitOK
}

// compatMigrateDry —— `--dry-run`：只报告将要迁移的文件，**不落盘**（源件同一条；差异 ⑥ = 退码）。
func compatMigrateDry(inv *invocation, cfg compat.Config, stdout, stderr io.Writer) int {
	statuses := compat.StatusAll(cfg)
	code := exitOK
	for _, st := range statuses {
		if st.Outcome == compat.OutcomeCorrupt {
			code = exitFail
		}
	}
	if code != exitOK {
		inv.setErr("failed", "compat_state_corrupt", "存在不可解析/不可用的状态文件")
	}
	if inv.jsonGiven {
		// 待迁移 + 读不动两种行都出（读不动那一行带 error ⇒ 「dry-run 说得出哪里卡住」）。
		rows := make([]map[string]string, 0, len(statuses))
		for _, st := range statuses {
			if st.Outcome == compat.OutcomeMigrated || st.Outcome == compat.OutcomeCorrupt {
				rows = append(rows, compatStatusRow(st))
			}
		}
		if rc := selectJSONList(stdout, stderr, inv, inv.path, inv.fields, rows); rc != exitOK {
			return rc
		}
		return code
	}
	fmt.Fprintln(stdout, "--dry-run：只报告将要迁移的文件，不落盘")
	pending := 0
	for _, st := range statuses {
		switch st.Outcome {
		case compat.OutcomeMigrated:
			fmt.Fprintf(stdout, "  待迁移  %s（磁盘 schema=%d → 目标 %d）\n", st.Path, st.OnDisk, st.Expected)
			pending++
		case compat.OutcomeCorrupt:
			fmt.Fprintf(stdout, "  ❌ 读不动 %s：%s\n", st.Path, st.Err)
		}
	}
	fmt.Fprintf(stdout, "共 %d 个文件待迁移\n", pending)
	return code
}

// ---- `zerg compat manifest` -----------------------------------------------------------------

// cmdCompatManifest —— 打印内嵌清单原文（CI 门禁 / 排障的对照面）。
//
// 直接写原始字节而不再 `json.Marshal` 一遍：`RawMessage` 再序列化会把 `<` 转义成 `\u003c`，
// 让人读的清单变成难读的一行（源件同一条理由，逐字保留）。
func cmdCompatManifest(inv *invocation, stdout, stderr io.Writer) int {
	// 差异 ④：源件在 manifest 档把四枚旗标**静默忽略** ⇒ 命令面一律明拒。
	if rc := compatFlagAllowed(inv, stderr, compatManifestUsage); rc != exitOK {
		return rc
	}
	if _, err := stdout.Write(append(compat.ManifestJSONForDump(), '\n')); err != nil {
		inv.setErr("failed", "compat_manifest_write_failed", "清单原文写不出去")
		fmt.Fprintf(stderr, "%s: 写输出失败: %v\n", progName, err)
		return exitFail
	}
	return exitOK
}
