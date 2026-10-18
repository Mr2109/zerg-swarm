// family_egg.go —— 卵族**具名入口**：`zerg egg <info|verify|gc|expand|release>`（卵开工 · 卵7①）。
//
// 出处（✗ 不新造口径）：
//
//	· 任务单 `项目文档/v2.5.15/任务单-卵单文件封装-v2-20261015.md` 卵7①（「卵族 CLI 具名入口」）；
//	· 设计真源 `01-设计/设计-茧壁-统一封闭契约与等级自证.md` —— §9.5 判据 3（逐段 sha256 不符即拒）
//	  与判据 5（二次孵化不再全量校验 ⇒ 校验缓存）· §11.2（不变量）· §16.2（生命周期与体量 ·
//	  引用计数 / GC / df 报数）· §13.1（按卵 sha 建目录并复用）。
//
// 落点口径（★ 照同族先例 `family_core_db.go`，✗ 不新造体例）：一条命令一个动作 ·
// 子动作在 `path` 里（`[]string{"egg", "<动作>"}`）· 转发壳只做「参数 → 已有接口 → 人面/机器面」。
//
// # 本件是**薄壳**（✗ 不在命令层重写任何逻辑）
//
//	info    → `eggstore.Store.Roster`（`Get` / `List`）＋ `eggstore.RefGraph.Count`（只读名册 ＋ 引用格）
//	verify  → `zerg_egg_verify.go` 的 `zegvVerifyEntry`（逐段并行 sha256 ＋ 内容寻址校验缓存）
//	gc      → `eggstore.Store.GC`（引用计数归零才真删 · 报数只认 df）
//	expand  → `eggstore.Store.Expand`（先登记引用 → 再 CoW 整文件落件）
//	release → `eggstore.Store.ReleaseExpansion`（先删件 → 再解引用）
//
// ★ 判据（问题串 / 退码 / 拒执理由）**一律不在这里重写** —— 转发体的 `RefuseError` /
// `problems` / `GCReport` 逐字上抛，命令层只做「取数 ＋ 排版 ＋ 退码投影」。
//
// # 卵库根（`--root`）
//
// `--root <目录>`（**已在名字表上**的值旗标 ⇒ ✗ 不新开旗标）；缺省 `~/.zerg/eggs`（与
// `~/.zerg/state` / `~/.zerg/backup` 同族形态）。★ 解析不出来（既没给 `--root`、也没有 HOME）
// ⇒ fail-closed 退 8，✗ 不回落 cwd（照本包既有口径）。
//
// # 危险档（照 `family_core_db.go` 的族先例 · 三态齐）
//
//	· `gc`      —— **D3**：真删卵件（不可逆 ⇒ `--confirm=<卵库根>` 与 `--yes` **同时到**才执行；
//	               缺任一 ⇒ fail-closed 退 2）；`--dry-run` ⇒ `GC(true)` 计划件 · 零副作用 · 退 0。
//	· `expand`  —— **D2**：写一件（落点）＋ 登记一条引用 ⇒ `--yes` 即可；`--dry-run` ⇒ 只读探针计划件。
//	· `release` —— **D2**：删副本件 ＋ 解引用 ⇒ `--yes` 即可；`--dry-run` ⇒ 只读探针计划件。
//	· `info` / `verify` —— 非危险档（`danger == nil`）；`verify` 另有其**运行态缓存**写面
//	               （`$ZERG_EGG_VERIFY_CACHE` / `<userCache>/zerg/egg-verify-cache`），
//	               ★ 它**不在**卵库面内、✗ 不碰卵一个字节（口径见 `zerg_egg_verify.go` 件头 ⑥）。
//
// 退码：0 成 · 1 判红/动作失败（`problems` 非空 / 转发体报错）· 2 用法错（含确认档缺项）·
// 8 不给结论（卵库打不开 / 名册里没有这枚卵 / 卵库根解析不出来）。
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Mr2109/zerg-swarm/core/internal/store/eggstore"
)

// eggActions —— 卵库面五动作的闭集（用法错的报面与帮助共用这一份 ⇒ ✗ 不在第二处再判一次）。
var eggActions = []string{"info", "verify", "gc", "expand", "release"}

// eggUsageErr —— 用法错的统一报面（K14：指出哪里错 ＋ 下一步；✗ 不在五处各写一遍）。
func eggUsageErr(inv *invocation, stderr io.Writer, why string) {
	act := strings.Join(inv.path[1:], " ")
	msg := fmt.Sprintf("`egg %s` 用法错：%s", act, why)
	inv.setErr("usage", "bad_action", msg)
	fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
	fmt.Fprintf(stderr, "卵库面动作闭集 = %s\n", strings.Join(eggActions, " | "))
	fmt.Fprintf(stderr, "先看用法：%s egg %s --help\n", progName, act)
}

// eggStoreRoot —— 卵库根：`--root <目录>` ⇒ 用它；不给 ⇒ `~/.zerg/eggs`。
//
// 返回 (根, 来源说明, 退码)；退码 != 0 ⇒ 调用方直接退（✗ 不猜、✗ 不回落 cwd）。
func eggStoreRoot(inv *invocation, stderr io.Writer) (string, string, int) {
	if p := strings.TrimSpace(inv.flagVal("--root")); p != "" {
		return p, "--root", exitOK
	}
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		msg := "卵库根解析不出来（既没给 `--root`、也取不到 HOME）⇒ 不给结论（退码 8）"
		inv.setErr("blocked", "store_root_unresolved", msg)
		fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
		return "", "", exitBlocked
	}
	return filepath.Join(home, ".zerg", "eggs"), "缺省 ~/.zerg/eggs", exitOK
}

// eggStoreOpen —— 打开卵库。★ 件不存在 ⇒ **空库**（`eggstore.Open` 定；✗ 本函数不报错、
// ✗ 也不创建 —— 创建只发生在写入路径）。打不开（件在盘但不可解析 / 读不了）⇒ 退 8 出声。
func eggStoreOpen(inv *invocation, stderr io.Writer, root string) (*eggstore.Store, int) {
	st, err := eggstore.Open(root)
	if err != nil {
		msg := fmt.Sprintf("卵库打不开（%s）：%v ⇒ 不给结论（退码 8）", root, err)
		inv.setErr("blocked", "store_open_failed", msg)
		fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
		return nil, exitBlocked
	}
	return st, exitOK
}

// ── info（只读名册 ＋ 引用格）──────────────────────────────────────────────────────────────

// eggRow —— 名册一行 → 机器面一格（取不到的格**整键略去**，✗ 不落 `0` / 空串）。
func eggRow(e eggstore.Egg, refs int) map[string]string {
	return map[string]string{
		"egg":        e.Name,
		"sha256":     e.SHA256,
		"state":      string(e.State),
		"path":       e.Path,
		"bytes":      strconv.FormatInt(e.Bytes, 10),
		"refs":       strconv.Itoa(refs),
		"complete":   strconv.FormatBool(e.Complete),
		"blocks":     strconv.Itoa(e.Blocks),
		"added_at":   e.AddedAt.UTC().Format("2006-01-02T15:04:05Z"),
		"updated_at": e.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}

// cmdEggInfo —— `zerg egg info [<卵名>] [--root <卵库根>] [--json <字段>]`（只读）。
//
// 两态：给卵名 ⇒ 该枚的档案；不给 ⇒ 卵库名册态（全册清单）。★ 名册里没有那枚 ⇒ 退 8 **点名**
// （✗ 不猜、✗ 不凭空造读数、✗ 不顺手登记）。
//
// ★ 为什么名册清单挂在 `info` 而不新开一条 `ls`：`zerg egg ls` 这一格**现读已被占用**
// （`main.go` 的「虫卵 × 设备矩阵」只读投影面，同族自注逐字「`egg` 已被【虫卵（沙箱/容器）投影面】
// 占用」）⇒ 同路径再加一条 ＝ 命令树同路径两条 ✗ ⇒ 按「✗ 不两处各留一套」把名册态落在 `info` 的无参档。
func cmdEggInfo(inv *invocation, stdout, stderr io.Writer) int {
	if inv.jsonGiven {
		if rc := requireFields(inv, stderr); rc != exitOK {
			return rc
		}
	}
	if len(inv.args) > 1 {
		eggUsageErr(inv, stderr, fmt.Sprintf("多给了位置参数 %q（本命令只收**至多一个**卵名）", inv.args[1]))
		return exitUsage
	}
	name := ""
	if len(inv.args) == 1 {
		name = strings.TrimSpace(inv.args[0])
	}
	// ★ 案一（GAP-20261019-04）：**先判参数是不是盘上已存在的普通文件** ⇒ 转文件读面。
	// 「同一实现」＝ 直接调 `zegCmdInfo`（它的调用链正是 `zerg cocoon info`：cmdCocoonEgg →
	// zergEggGateMain(["info", …]) → zegCmdInfo）⇒ ✗ 不复制第二套读端。
	// 判据只认「os.Stat 成功且 mode.IsRegular()」：目录 / 不存在 / 打不开 一律回落名册面（零回归）。
	if name != "" {
		if fi, serr := os.Stat(name); serr == nil && fi.Mode().IsRegular() {
			if inv.jsonGiven {
				// 文件读面只有人面读数（zegCmdInfo 不投影机器面）⇒ ✗ 静默吞 `--json`。
				eggUsageErr(inv, stderr, "参数指向**文件**，而文件读面只有人面读数、没有 `--json` 机器面（要机器面：`"+progName+" egg verify <文件> --json <字段>`）")
				return exitUsage
			}
			fmt.Fprintf(stderr, "%s: 参数指向盘上文件 ⇒ 按文件读（转 `zegCmdInfo`，与 `%s cocoon info` 同一实现；本次不走卵库名册）\n", progName, progName)
			return zegCmdInfo(stdout, name)
		}
	}
	root, src, rc := eggStoreRoot(inv, stderr)
	if rc != exitOK {
		return rc
	}
	st, rc := eggStoreOpen(inv, stderr, root)
	if rc != exitOK {
		return rc
	}
	if name == "" {
		return eggInfoRoster(inv, stdout, stderr, st, root, src)
	}
	e, ok := st.Roster.Get(name)
	if !ok {
		msg := fmt.Sprintf("卵库（%s）的名册里没有这枚卵 %q ⇒ 不给结论（退码 8）—— 现读册内 %d 条",
			root, name, st.Roster.Count())
		inv.setErr("blocked", "egg_absent", msg)
		fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
		fmt.Fprintf(stderr, "看全册：%s egg info --root %s\n", progName, root)
		return exitBlocked
	}
	row := eggRow(e, st.Refs.Count(e.Name))
	row["action"] = "info"
	row["root"] = root
	row["root_source"] = src
	row["count"] = strconv.Itoa(st.Roster.Count())
	row["verdict"] = "ok"
	if inv.jsonGiven {
		if jrc := selectJSON(stdout, stderr, inv, inv.path, inv.fields, row); jrc != exitOK {
			return jrc
		}
		return exitOK
	}
	fmt.Fprintf(stdout, "卵库      : %s（%s）\n", root, src)
	fmt.Fprintf(stdout, "卵        : %s\n", row["egg"])
	fmt.Fprintf(stdout, "态 / 完工 : %s / %s（★ 只有 verified 可孵）\n", row["state"], row["complete"])
	fmt.Fprintf(stdout, "sha256    : %s\n", row["sha256"])
	fmt.Fprintf(stdout, "路径      : %s\n", orX(row["path"], "（空 —— 无源可展开）"))
	fmt.Fprintf(stdout, "引用 / 块 : %s / %s\n", row["refs"], row["blocks"])
	fmt.Fprintf(stdout, "名义字节  : %s（★ 只作登记：报「已释放」只认 df）\n", row["bytes"])
	fmt.Fprintf(stdout, "判词      : ok\n")
	fmt.Fprintf(stderr, "%s: 只读（名册 ＋ 引用图 —— 一个字节都没写）\n", progName)
	return exitOK
}

// eggInfoRoster —— 名册态（无参）：全册清单（**空册 = 空结论**，✗ 不当「读不到」）。
func eggInfoRoster(inv *invocation, stdout, stderr io.Writer, st *eggstore.Store, root, src string) int {
	eggs := st.Roster.List()
	rows := make([]map[string]string, 0, len(eggs))
	for _, e := range eggs {
		row := eggRow(e, st.Refs.Count(e.Name))
		row["action"] = "info"
		row["root"] = root
		row["root_source"] = src
		row["count"] = strconv.Itoa(len(eggs))
		row["verdict"] = "ok"
		rows = append(rows, row)
	}
	if inv.jsonGiven {
		return selectJSONList(stdout, stderr, inv, inv.path, inv.fields, rows)
	}
	fmt.Fprintf(stdout, "卵库  : %s（%s）\n", root, src)
	fmt.Fprintf(stdout, "条数  : %d\n", len(rows))
	if len(rows) == 0 {
		fmt.Fprintf(stdout, "（名册是空的 —— 空结论，不是「读不到」）\n")
		fmt.Fprintf(stdout, "判词  : ok\n")
		return exitOK
	}
	fmt.Fprintf(stdout, "%-24s %-12s %-6s %-6s %s\n", "卵", "态", "完工", "引用", "路径")
	for _, row := range rows {
		fmt.Fprintf(stdout, "%-24s %-12s %-6s %-6s %s\n",
			row["egg"], row["state"], row["complete"], row["refs"], orX(row["path"], "（空）"))
	}
	fmt.Fprintf(stdout, "判词  : ok（回收看 `egg gc --dry-run`；报「已释放」只认 df）\n")
	fmt.Fprintf(stderr, "%s: 只读（名册 ＋ 引用图 —— 一个字节都没写）\n", progName)
	return exitOK
}

// ── verify（转发 `zerg_egg_verify.go` · 逐段并行 ＋ 内容寻址缓存）───────────────────────────

// cmdEggVerify —— `zerg egg verify <卵包.egg> [--json <字段>]`。
//
// ★ 转发体是 `zegvVerifyEntry`（**唯一入口**）：① 命中判定 → ② 并行全量校验 → ③ 绿才落条目。
// 本函数 ✗ 不重写任何判据：`problems` 逐字来自转发体（问题串自带**段名** ⇒ 点名到哪一段）。
func cmdEggVerify(inv *invocation, stdout, stderr io.Writer) int {
	if inv.jsonGiven {
		if rc := requireFields(inv, stderr); rc != exitOK {
			return rc
		}
	}
	if len(inv.args) != 1 {
		eggUsageErr(inv, stderr, "要**恰好一个**位置参数：卵包 .egg 件")
		return exitUsage
	}
	p := strings.TrimSpace(inv.args[0])
	if p == "" {
		eggUsageErr(inv, stderr, "卵包路径是空的")
		return exitUsage
	}
	// ★ 机器面：转发体的人面观测行走 `io.Discard` ⇒ ✗ 不许污染 stdout 的 JSON 包封。
	out := stdout
	if inv.jsonGiven {
		out = io.Discard
	}
	res, err := zegvVerifyEntry(out, p, "", 0)
	if err != nil {
		msg := fmt.Sprintf("校验跑不起来（%s）：%v ⇒ 退码 1", p, err)
		inv.setErr("failed", "verify_failed", msg)
		fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
		return exitFail
	}
	row := map[string]string{
		"action":        "verify",
		"path":          p,
		"problems":      strings.Join(res.Problems, " ; "),
		"problem_count": strconv.Itoa(len(res.Problems)),
		"cache_hit":     strconv.FormatBool(res.CacheHit),
		"segments":      "0",
		"data_bytes":    "0",
	}
	if st, serr := os.Stat(p); serr == nil && !st.IsDir() {
		row["size_bytes"] = strconv.FormatInt(st.Size(), 10)
	}
	if res.Index != nil {
		var data int64
		for _, e := range res.Index.Entries {
			data += e.Size
		}
		row["segments"] = strconv.Itoa(len(res.Index.Entries))
		row["data_bytes"] = strconv.FormatInt(data, 10)
		row["align"] = strconv.FormatInt(res.Index.Align, 10)
	}
	rc := exitOK
	if len(res.Problems) > 0 {
		row["verdict"] = "failed"
		rc = exitFail
	} else {
		row["verdict"] = "ok"
	}
	// ★ 机器面也必带**判红退码**：`--json` 只是字段投影，✗ 不许把 rc=1 吞成 0
	// （§九 M1：`--json` 只决定内容与裁剪，与退码不是同一条 —— 判红退 0 就是假绿）。
	if inv.jsonGiven {
		if jrc := selectJSON(stdout, stderr, inv, inv.path, inv.fields, row); jrc != exitOK {
			return jrc
		}
		return rc
	}
	if len(res.Problems) > 0 {
		fmt.Fprintf(stdout, "✗ 校验失败 %d 项（点名到段）：\n", len(res.Problems))
		for _, pr := range res.Problems {
			fmt.Fprintf(stdout, "   - %s\n", pr)
		}
		fmt.Fprintf(stderr, "%s: 逐段 sha256 与索引不符 ⇒ 拒（§9.5 判据 3）\n", progName)
		return rc
	}
	mark := ""
	if res.CacheHit {
		mark = "（缓存命中 · 未重跑全量 —— §9.5 判据 5）"
	}
	fmt.Fprintf(stdout, "✓ 校验通过（%s 段 · align=%s · 数据 %s 字节）%s\n",
		row["segments"], row["align"], row["data_bytes"], mark)
	return exitOK
}

// ── gc（转发 `eggstore.Store.GC` · D3）────────────────────────────────────────────────────

// eggGCRow —— `GCReport` → 机器面（★ 报数口径**逐字取转发体**，✗ 不在这里另算一套）。
//
// ★ 干跑档**没有** df 读数（`GC(true)` 不采样）⇒ 那几个键**整键略去**（✗ 不许落 `0` —— 那会
// 把「没量过」伪装成「量为 0」）。
func eggGCRow(rep eggstore.GCReport) map[string]string {
	row := map[string]string{
		"action":                  "gc",
		"dry_run":                 strconv.FormatBool(rep.DryRun),
		"deleted":                 strings.Join(rep.Deleted, ","),
		"deleted_count":           strconv.Itoa(len(rep.Deleted)),
		"pinned_count":            strconv.Itoa(len(rep.Pinned)),
		"nominal_bytes_not_freed": strconv.FormatInt(rep.NominalBytesNotFreed, 10),
		"nominal_note":            rep.NominalNote,
		"note":                    rep.Note,
	}
	if !rep.DryRun {
		row["df_before"] = strconv.FormatUint(rep.DFBefore, 10)
		row["df_after"] = strconv.FormatUint(rep.DFAfter, 10)
		row["freed_bytes_df"] = strconv.FormatInt(rep.FreedBytesDF, 10)
		row["freed_source"] = rep.FreedSource
	}
	if len(rep.Pinned) > 0 {
		var b []string
		for _, p := range rep.Pinned {
			b = append(b, fmt.Sprintf("%s(refs=%d,state=%s):%s", p.Egg, p.Refs, string(p.State), p.Why))
		}
		row["pinned"] = strings.Join(b, " ; ")
	}
	return row
}

// cmdEggGC —— `zerg egg gc [--root <卵库根>] [--dry-run | --confirm=<卵库根> --yes] [--json <字段>]`。
func cmdEggGC(inv *invocation, stdout, stderr io.Writer) int {
	if inv.jsonGiven {
		if rc := requireFields(inv, stderr); rc != exitOK {
			return rc
		}
	}
	if rc := dryRunYesConflict(inv, stderr); rc != exitOK {
		return rc
	}
	if len(inv.args) > 0 {
		eggUsageErr(inv, stderr, fmt.Sprintf("不收位置参数（多给了 %q）—— 卵库根走 `--root`", inv.args[0]))
		return exitUsage
	}
	root, src, rc := eggStoreRoot(inv, stderr)
	if rc != exitOK {
		return rc
	}
	st, rc := eggStoreOpen(inv, stderr, root)
	if rc != exitOK {
		return rc
	}
	if inv.dryRun {
		rep, err := st.GC(true)
		if err != nil {
			msg := fmt.Sprintf("回收干跑失败（`eggstore.Store.GC(true)`）：%v ⇒ 退码 1", err)
			inv.setErr("failed", "gc_failed", msg)
			fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
			return exitFail
		}
		row := eggGCRow(rep)
		row["root"] = root
		row["root_source"] = src
		row["verdict"] = "planned"
		if inv.jsonGiven {
			return selectJSON(stdout, stderr, inv, inv.path, inv.fields, row)
		}
		fmt.Fprintf(stdout, "计划件（--dry-run · 零副作用 —— 未删任何件、未取 df 读数）\n")
		fmt.Fprintf(stdout, "  卵库     : %s（%s）\n", root, src)
		fmt.Fprintf(stdout, "  将真删   : %d 枚%s\n", len(rep.Deleted), eggMaybeList(rep.Deleted))
		fmt.Fprintf(stdout, "  钉住     : %d 枚（引用计数未归零 / 态不在回收面 —— 删前必须先解引用）\n", len(rep.Pinned))
		for _, p := range rep.Pinned {
			fmt.Fprintf(stdout, "     - %s（refs=%d · %s）：%s\n", p.Egg, p.Refs, p.State, p.Why)
		}
		fmt.Fprintf(stdout, "  真跑要   : %s egg gc --root %s --confirm=%s --yes\n", progName, root, root)
		fmt.Fprintf(stdout, "  真删口径 : 引用计数**归零**才删；报「已释放」只认 df（%s）\n", rep.FreedSource)
		return exitOK
	}
	// D3：`--confirm=<卵库根>` 与 `--yes` **同时到**才执行（照 `core db restore` 的三态）。
	if !inv.confirmGiven {
		msg := "`egg gc` 是 D3 档（**真删卵件 · 不可逆**）——**缺确认档 ⇒ 不执行**"
		inv.setErr("usage", "confirm_required", msg)
		guardMarkDecision(inv, guardDecisionApproval)
		fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
		fmt.Fprintf(stderr, "要执行得给：--confirm=%s --yes\n", root)
		fmt.Fprintf(stderr, "先看计划件：%s egg gc --root %s --dry-run\n", progName, root)
		fmt.Fprintf(stderr, "error.kind=usage · detail=confirm_required · retryable=false · remedy=fix_usage\n")
		return exitUsage
	}
	if inv.confirm != root {
		msg := fmt.Sprintf("确认值不匹配卵库根（--confirm 给的是 %q，卵库根是 %q）⇒ 不执行", inv.confirm, root)
		inv.setErr("usage", "confirm_mismatch", msg)
		guardMarkDecision(inv, guardDecisionBlock)
		fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
		fmt.Fprintf(stderr, "`--confirm` 的值必须与卵库根**逐字相同** —— 给错值一律拒绝，不许「当没给」\n")
		return exitUsage
	}
	if !inv.yes {
		msg := "`egg gc` 是 D3 档 —— **缺 --yes ⇒ 不执行**"
		inv.setErr("usage", "yes_required", msg)
		guardMarkDecision(inv, guardDecisionApproval)
		fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
		fmt.Fprintf(stderr, "确认档齐了才真跑：%s egg gc --root %s --confirm=%s --yes\n", progName, root, root)
		fmt.Fprintf(stderr, "先看计划件：%s egg gc --root %s --dry-run\n", progName, root)
		fmt.Fprintf(stderr, "error.kind=usage · detail=yes_required · retryable=false · remedy=fix_usage\n")
		return exitUsage
	}
	rep, err := st.GC(false)
	if err != nil {
		msg := fmt.Sprintf("回收失败（`eggstore.Store.GC(false)`）：%v ⇒ 退码 1", err)
		inv.setErr("failed", "gc_failed", msg)
		fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
		return exitFail
	}
	row := eggGCRow(rep)
	row["root"] = root
	row["root_source"] = src
	row["verdict"] = "ok"
	if inv.jsonGiven {
		if jrc := selectJSON(stdout, stderr, inv, inv.path, inv.fields, row); jrc != exitOK {
			return jrc
		}
		return exitOK
	}
	fmt.Fprintf(stdout, "卵库      : %s（%s）\n", root, src)
	fmt.Fprintf(stdout, "真删      : %d 枚%s\n", len(rep.Deleted), eggMaybeList(rep.Deleted))
	fmt.Fprintf(stdout, "钉住      : %d 枚\n", len(rep.Pinned))
	for _, p := range rep.Pinned {
		fmt.Fprintf(stdout, "   - %s（refs=%d · %s）：%s\n", p.Egg, p.Refs, p.State, p.Why)
	}
	fmt.Fprintf(stdout, "df 前/后  : %s / %s（来源 %s）\n", row["df_before"], row["df_after"], rep.FreedSource)
	fmt.Fprintf(stdout, "已释放    : %s 字节（**只认 df** —— ✗ 不用 du、✗ 不用名义字节）\n", row["freed_bytes_df"])
	fmt.Fprintf(stdout, "名义字节  : %s（★ 不是已释放读数；%s）\n", row["nominal_bytes_not_freed"], rep.NominalNote)
	fmt.Fprintf(stdout, "判词      : ok（%s）\n", rep.Note)
	return exitOK
}

// ── expand（转发 `eggstore.Store.Expand` · D2）───────────────────────────────────────────────

// eggParseKind —— `--kind` 取值闸（闭集 = `eggstore.RefKinds`；非法 ⇒ 用法错 **点名闭集**）。
func eggParseKind(inv *invocation, stderr io.Writer) (eggstore.RefKind, int) {
	k := strings.TrimSpace(inv.flagVal("--kind"))
	if k == "" {
		return eggstore.RefExpansion, exitOK
	}
	kind := eggstore.RefKind(k)
	if !kind.Valid() {
		var names []string
		for _, x := range eggstore.RefKinds {
			names = append(names, string(x))
		}
		eggUsageErr(inv, stderr, fmt.Sprintf("`--kind %s` 不在闭集内（只许 %s）", k, strings.Join(names, " | ")))
		return "", exitUsage
	}
	return kind, exitOK
}

// cmdEggExpand —— `zerg egg expand <卵名> <引用者 id> <落点> [--kind expansion|clone]
// [--root <卵库根>] [--dry-run | --yes] [--json <字段>]`。
func cmdEggExpand(inv *invocation, stdout, stderr io.Writer) int {
	if inv.jsonGiven {
		if rc := requireFields(inv, stderr); rc != exitOK {
			return rc
		}
	}
	if rc := dryRunYesConflict(inv, stderr); rc != exitOK {
		return rc
	}
	if len(inv.args) != 3 {
		eggUsageErr(inv, stderr, "要**恰好三个**位置参数：<卵名> <引用者 id> <落点>")
		return exitUsage
	}
	egg := strings.TrimSpace(inv.args[0])
	ref := strings.TrimSpace(inv.args[1])
	dst := strings.TrimSpace(inv.args[2])
	if egg == "" || ref == "" || dst == "" {
		eggUsageErr(inv, stderr, "卵名 / 引用者 id / 落点 三者都不许为空（✗ 不猜落点）")
		return exitUsage
	}
	kind, rc := eggParseKind(inv, stderr)
	if rc != exitOK {
		return rc
	}
	root, src, rc := eggStoreRoot(inv, stderr)
	if rc != exitOK {
		return rc
	}
	st, rc := eggStoreOpen(inv, stderr, root)
	if rc != exitOK {
		return rc
	}
	e, inRoster := st.Roster.Get(egg)
	if inv.dryRun {
		// 计划件 = **只读探针**（✗ 不登记引用、✗ 不落件、✗ 不建目录）。
		row := map[string]string{
			"action":       "expand",
			"root":         root,
			"root_source":  src,
			"egg":          egg,
			"ref":          ref,
			"kind":         string(kind),
			"dst":          dst,
			"dst_exists":   strconv.FormatBool(eggPathExists(dst)),
			"egg_known":    strconv.FormatBool(inRoster),
			"egg_state":    eggStateOr(e, inRoster),
			"egg_complete": strconv.FormatBool(inRoster && e.Complete),
			"egg_src":      e.Path,
			"refs_now":     strconv.Itoa(st.Refs.Count(egg)),
			"dry_run":      "true",
			"verdict":      "planned",
			"note":         "计划件：先登记引用 → 再 CoW 整文件落件；真跑要 --yes（✗ 本档未登记、未落件）",
		}
		if inv.jsonGiven {
			return selectJSON(stdout, stderr, inv, inv.path, inv.fields, row)
		}
		fmt.Fprintf(stdout, "计划件（--dry-run · 零副作用 —— 未登记引用、未落件、未建目录）\n")
		fmt.Fprintf(stdout, "  卵库     : %s（%s）\n", root, src)
		fmt.Fprintf(stdout, "  动作     : expand（先登记引用 → 再 CoW 整文件落件）\n")
		fmt.Fprintf(stdout, "  卵 / 引用 : %s / %s（种类 %s）\n", egg, ref, string(kind))
		fmt.Fprintf(stdout, "  落点     : %s（在盘=%t）\n", dst, eggPathExists(dst))
		fmt.Fprintf(stdout, "  卵在册   : %t（完工=%t · 态=%s · 源=%s）\n", inRoster,
			inRoster && e.Complete, eggStateOr(e, inRoster), orX(e.Path, "（空）"))
		if !inRoster {
			fmt.Fprintf(stdout, "  ★ 现读在册与否：**不在册** ⇒ 真跑会拒（`eggstore.Store.Expand` 的 RefuseError）\n")
		}
		fmt.Fprintf(stdout, "  现引用数 : %d（本动作 = ＋1）\n", st.Refs.Count(egg))
		fmt.Fprintf(stdout, "  真跑要   : %s egg expand %s %s %s --kind %s --root %s --yes\n",
			progName, egg, ref, dst, string(kind), root)
		fmt.Fprintf(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未登记引用、未落件）\n")
		return exitOK
	}
	if rc := writeGuardYes(inv, stderr); rc != exitOK {
		return rc
	}
	rep, err := st.Expand(egg, ref, dst, kind, eggstore.CowOptions{})
	if err != nil {
		msg := fmt.Sprintf("展开未做（`eggstore.Store.Expand`）：%v ⇒ 退码 1（条件不足 ⇒ 拒 · ✗ 不静默降级）", err)
		inv.setErr("failed", "expand_failed", msg)
		fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
		return exitFail
	}
	row := map[string]string{
		"action":      "expand",
		"root":        root,
		"root_source": src,
		"egg":         rep.Egg,
		"ref":         rep.Ref,
		"kind":        string(rep.Kind),
		"dst":         rep.Path,
		"tier":        string(rep.Tier),
		"cow":         strconv.FormatBool(rep.CoW),
		"bytes":       strconv.FormatInt(rep.Bytes, 10),
		"refs_after":  strconv.Itoa(rep.RefsAfter),
		"dur_ms":      strconv.FormatFloat(rep.Result.DurMS, 'f', -1, 64),
		"fell_back":   strconv.FormatBool(rep.Result.FellBack),
		"verdict":     "ok",
	}
	if inv.jsonGiven {
		if jrc := selectJSON(stdout, stderr, inv, inv.path, inv.fields, row); jrc != exitOK {
			return jrc
		}
		return exitOK
	}
	fmt.Fprintf(stdout, "卵        : %s\n", row["egg"])
	fmt.Fprintf(stdout, "引用      : %s（种类 %s）\n", row["ref"], row["kind"])
	fmt.Fprintf(stdout, "落点      : %s\n", row["dst"])
	fmt.Fprintf(stdout, "档        : %s（共享物理块=%s · 回退过=%s · %s ms）\n",
		row["tier"], row["cow"], row["fell_back"], row["dur_ms"])
	fmt.Fprintf(stdout, "真拷贝字节: %s（共享块档 ⇒ 0）\n", row["bytes"])
	fmt.Fprintf(stdout, "引用计数  : %s（展开物 / 克隆副本纳入**同一**引用计数 —— 删前必须先解引用）\n", row["refs_after"])
	fmt.Fprintf(stdout, "判词      : ok\n")
	return exitOK
}

// ── release（转发 `eggstore.Store.ReleaseExpansion` · D2）───────────────────────────────────

// cmdEggRelease —— `zerg egg release <卵名> <引用者 id> [<副本路径>] [--root <卵库根>]
// [--dry-run | --yes] [--json <字段>]`。
func cmdEggRelease(inv *invocation, stdout, stderr io.Writer) int {
	if inv.jsonGiven {
		if rc := requireFields(inv, stderr); rc != exitOK {
			return rc
		}
	}
	if rc := dryRunYesConflict(inv, stderr); rc != exitOK {
		return rc
	}
	if len(inv.args) < 2 || len(inv.args) > 3 {
		eggUsageErr(inv, stderr, "要**两到三个**位置参数：<卵名> <引用者 id> [<副本路径>]")
		return exitUsage
	}
	egg := strings.TrimSpace(inv.args[0])
	ref := strings.TrimSpace(inv.args[1])
	path := ""
	if len(inv.args) == 3 {
		path = strings.TrimSpace(inv.args[2])
	}
	if egg == "" || ref == "" {
		eggUsageErr(inv, stderr, "卵名 / 引用者 id 都不许为空")
		return exitUsage
	}
	root, src, rc := eggStoreRoot(inv, stderr)
	if rc != exitOK {
		return rc
	}
	st, rc := eggStoreOpen(inv, stderr, root)
	if rc != exitOK {
		return rc
	}
	if inv.dryRun {
		row := map[string]string{
			"action":      "release",
			"root":        root,
			"root_source": src,
			"egg":         egg,
			"ref":         ref,
			"path":        path,
			"path_exists": strconv.FormatBool(path != "" && eggPathExists(path)),
			"refs_now":    strconv.Itoa(st.Refs.Count(egg)),
			"dry_run":     "true",
			"verdict":     "planned",
			"note":        "计划件：先删副本件 → 再解引用；真跑要 --yes（✗ 本档未删件、未改引用图）",
		}
		if inv.jsonGiven {
			return selectJSON(stdout, stderr, inv, inv.path, inv.fields, row)
		}
		fmt.Fprintf(stdout, "计划件（--dry-run · 零副作用 —— 未删任何件、未改引用图、未取 df 读数）\n")
		fmt.Fprintf(stdout, "  卵库     : %s（%s）\n", root, src)
		fmt.Fprintf(stdout, "  动作     : release（先删副本件 → 再解引用）\n")
		fmt.Fprintf(stdout, "  卵 / 引用 : %s / %s\n", egg, ref)
		fmt.Fprintf(stdout, "  副本件   : %s（在盘=%t）\n", orX(path, "（未给 ⇒ 只解引用、不删件）"),
			path != "" && eggPathExists(path))
		fmt.Fprintf(stdout, "  现引用数 : %d（解引用后归零 ⇒ 才允许 `egg gc` 真删）\n", st.Refs.Count(egg))
		fmt.Fprintf(stdout, "  真跑要   : %s egg release %s %s %s --root %s --yes\n",
			progName, egg, ref, path, root)
		fmt.Fprintf(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未删件、未改引用图）\n")
		return exitOK
	}
	if rc := writeGuardYes(inv, stderr); rc != exitOK {
		return rc
	}
	rep, err := st.ReleaseExpansion(egg, ref, path)
	if err != nil {
		msg := fmt.Sprintf("解引用未做（`eggstore.Store.ReleaseExpansion`）：%v ⇒ 退码 1", err)
		inv.setErr("failed", "release_failed", msg)
		fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
		return exitFail
	}
	row := map[string]string{
		"action":         "release",
		"root":           root,
		"root_source":    src,
		"egg":            rep.Egg,
		"ref":            rep.Ref,
		"path":           rep.Path,
		"released":       strconv.FormatBool(rep.Released),
		"refs_left":      strconv.Itoa(rep.RefsLeft),
		"zero":           strconv.FormatBool(rep.Zero),
		"df_before":      strconv.FormatUint(rep.DFBefore, 10),
		"df_after":       strconv.FormatUint(rep.DFAfter, 10),
		"freed_bytes_df": strconv.FormatInt(rep.FreedBytesDF, 10),
		"freed_source":   rep.FreedSource,
		"nominal_note":   rep.NominalNote,
		"verdict":        "ok",
	}
	if inv.jsonGiven {
		if jrc := selectJSON(stdout, stderr, inv, inv.path, inv.fields, row); jrc != exitOK {
			return jrc
		}
		return exitOK
	}
	fmt.Fprintf(stdout, "卵 / 引用 : %s / %s\n", row["egg"], row["ref"])
	fmt.Fprintf(stdout, "副本件    : %s\n", orX(row["path"], "（未给 —— 只解引用）"))
	fmt.Fprintf(stdout, "解掉引用  : %s\n", row["released"])
	fmt.Fprintf(stdout, "引用计数  : %s（归零=%s ⇒ 归零后才允许 `egg gc` 真删）\n", row["refs_left"], row["zero"])
	fmt.Fprintf(stdout, "df 前/后  : %s / %s（来源 %s）\n", row["df_before"], row["df_after"], rep.FreedSource)
	fmt.Fprintf(stdout, "已释放    : %s 字节（**只认 df**）\n", row["freed_bytes_df"])
	fmt.Fprintf(stdout, "判词      : ok\n")
	return exitOK
}

// ── 只读小工具 ──────────────────────────────────────────────────────────────────────────────

// eggPathExists —— 路径在盘（只读）。
func eggPathExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// eggStateOr —— 人面一格的态（不在册 ⇒ 逐字「（不在册）」）。
func eggStateOr(e eggstore.Egg, ok bool) string {
	if !ok {
		return "（不在册）"
	}
	return string(e.State)
}

// eggMaybeList —— 人面把卵名清单摊成一行（空 ⇒ 「（无）」）。
func eggMaybeList(xs []string) string {
	if len(xs) == 0 {
		return "（无）"
	}
	return "：" + strings.Join(xs, " · ")
}
