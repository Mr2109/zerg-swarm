// gen_cli_reference.go —— 生成器件 `scripts/docs/gen-cli-reference.py`（1159 行）的 **Go 真搬**
// （v2.5.14 · M2 · ③ 类生成器搬运 · 入口 `genCliReferenceGateMain`）。
//
// 适格口径（【生成器档】· ✗ 不是判据件的「零 subprocess」）：本件真跑子进程是**本职**
// （它要读 `bin/zerg help` / `help export` / `version`）⇒ 适格 = **产物可逐字节对拍**：
// 同输入（同一棵仓树 + 同一 `bin/zerg` 制品）下，py 原件 ⟷ 本入口 四份产物逐字节相同
// （同输入比产出 sha256）＋ 逐格 rc/stdout/stderr 三面相同。
//
// 本件形态：生成器 · 依赖现役 `bin/zerg` 制品（只读调用）· git 读面（源码面身份对拍）·
// 写面仅 doc 产物（`--emit`）。逐字节对齐要点：
//
//	① `os.path.join/dirname/abspath` 语义 ⇒ 自写 `gcrPyJoin` / `gcrPyDirname` / `gcrPyAbspath`
//	   （✗ 不用 `filepath.Join`）。
//	② py 文本模式读取 = **universal newlines**（`\r\n` 与裸 `\r` 折成 `\n`）⇒ `gcrRead` 显式折。
//	③ `str.strip()` 剥 **Python 的 Unicode 空白**（含 `\x1c-0x1f` / `\x85` / `\xa0`）⇒ 自写 `gcrPyStrip`。
//	④ `str.splitlines()` 的分行集（`\n \r \v \f \x1c \x1d \x1e \x85 \u2028 \u2029`）⇒ 自写 `gcrPySplitlines`。
//	⑤ `%-28s` 按**码点**补位 ⇒ 自写 `gcrPadRight`（✗ 不用 fmt `%-28s` —— 它按字节）。
//	⑥ 目录读序：py `sorted(os.listdir)` / `sorted(glob.glob)` ⇒ 一律排序后读（排序面 = 码点序）。
//	⑦ `L`（locale 模板）逐字嵌入为 JSON 常量 `gcrLJSON`，由 py 原件现读导出（✗ 不手抄）。
//	⑧ argparse 面：`--emit|--check` 互斥必填 + `--target` 追加 + 唯一前缀缩写 ⇒ 自写 `gcrParseArgs`。
//
// 两条标准声明行：
//
//	(a) **默认根定位差异**：py `find_root(os.getcwd())` 从 **cwd** 上溯 ≤8 层找含
//	    `core/cmd/zerg/main.go` 的目录；本件同样从 cwd 上溯（无 `__file__` 语境，行为一致）。
//	(b) **自指检查**：本件落 `core/cmd/zerg/`，会进同包他门（如 `check-error-kinds` 的 R3）
//	    的扫描面 ⇒ 件内**零**形如「JSON 键名连续引号原串」的写法（一律描述式）· 退码仅
//	    具名常量 0/1/2（⊆ 真源退码表已登记集）· 零绝对私有路径字面量。
//
// ── 身份判据收窄（`Q-211` 方案 B）──────────────────────────────────────
//
// 旧判据 = 页内身份行与现跑件**逐字相等**（含 `build_time`）+ 整目录 `core/cmd/zerg` diff ⇒
// 任何一笔 `core/**` 提交都逼着重编 + 重生成（命令页随无关改动翻红）。收窄后：
//
//	· **窄集** = `main.go` + `exitcodes.go` + `export.go` + `core/internal/version`（`gcrSourceFaceRels`）；
//	· **判据** = 页内 commit 是 HEAD **祖先** ∧ **窄集**自该 commit **零改动**（`gcrPageCommitVerdict`）；
//	· `build_time` **不比**（每次编译都变）；`commit` 也**不再**要求逐字相等。
//	· **保留**：产物 drift 逐字节 · 命令名对拍（读 `main.go` 源码）· 禁 `+dirty` · 窄集工作树
//	  未提交改动判红 · 祖先/查无此提交判红。
//
// **剩余盲窗（照实点名 · ✗ 装作没有 · 已实测）**：`main.go` 的树条目里有**按标识符引用**的
// 字面量（如 `fields: gateRunStepFields` · `usage: <常量名>`），它们的**定义**落在窄集外的
// `family_*.go` 里。改这类定义（如给某命令的 `--json` 字段表加一格）**不碰 `main.go`**，
// 而该列只经 `zerg help export` 从**制品**读出（源码侧没有第二读法）⇒ 若这笔提交**既不重编
// `bin/`、也不 `--emit`**，本门仍是绿（页与旧制品自洽）。实测：`family_gate_run_step.go` 的
// `gateRunStepFields` 加一格 ⇒ 本门 rc=0（绿）；重编一次 ⇒ 四件产物 drift 全红（牙回来了）。
// 收窄前这一格靠「整目录 diff」间接兜住（代价是全员翻红）。兜底门（收窄后必须一起在场，
// ✗ 只靠本门）：
//
//	① **命令名/用法串源码侧对拍**（本件内建 · 已现读，不靠窄集）：新命令必须改 `main.go`
//	   的 `commands = []*command{…}` ⇒ 名字对拍红；usage 常量改了 ⇒ `gcrConstUsages` 现读
//	   包内全部 `*.go`，与 `zerg help` 现跑逐字比 ⇒ `两处真源不一致` rc=2；
//	② **`zerg gate check cli-contract --verify-live`（R-6 逐格现跑）** 与 **`check-newcmd-followup`**
//	   （命令树条数 ⟷ 基线）：读的是**现编制品** ⇒ 新命令/新旗标没登记就红；
//	③ **`check-help-shape`**（门69）冻结 `help`/`help dangerous` 的行数/字节/表尾 ⇒ 导出面那几列
//	   变了必然对不上（其基线由**源码现编件** `--emit-baseline` 取数）；
//	④ **发布前 `publish-preflight.sh` ⑧e** 调本门 `--check`，但它的 `$ZERGCLI` 是**当次新编**
//	   制品 ⇒ 真实发布路径上「制品 vs 页」的 drift 一定被比到（盲窗只在「不重编也不生成」的
//	   本地/提交期存在）。
//
// 落仓顺序：**本件与命令页四件同笔**（`--emit` 产物必须与判据同批落地，否则提交期的
// `gen-cli-reference --check` 自相矛盾）；窄集变更后第一次收口需**干净重编 + `--emit`**。
//
// 本单边界（照实）：**已接线（inproc 表分派）** ——
//
//	· 不在 `core/cmd/zerg/main.go` 注册命令（✗ 不进命令树）；
//	· 不删 py 原件（退场规则：对拍通过后才逐件点名退场）；
//	· **已接线**：入口 `genCliReferenceGateMain` 由 `gateCheckGoEntry["gen-cli-reference"]` 分派（`zerg gate check gen-cli-reference`）· 原文此处断言与现读相反（只改文案 · 未改行为）。
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// ── 常量（照 py 逐字）────────────────────────────────────────────────

const (
	gcrZHRel          = "docs/zh/参考-命令行.md"
	gcrENRel          = "docs/en/参考-命令行.md"
	gcrMainRel        = pathCmdMainGo
	gcrGenRel         = "core/cmd/zerg/gen_cli_reference.go"
	gcrIdentPrefix    = "> 身份 / identity："
	gcrMarkBegin      = "<!-- ===== 生成区 BEGIN"
	gcrMarkEnd        = "<!-- ===== 生成区 END"
	gcrPublishDocsRel = pathPublishDocs
	gcrNoFields       = "（无机器面）"

	gcrRCok      = 0
	gcrRCfail    = 1
	gcrRCblocked = 2

	gcrProg      = "gen-cli-reference.py"
	gcrUsageLine = "usage: gen-cli-reference.py [-h] (--emit | --check) [--target REL]"
)

var (
	// gcrSourceFaceRels —— **窄集**（`Q-211` 方案 B）：只有这四件能改变「本页所描述的那张面」。
	//
	//	· `core/cmd/zerg/main.go`      —— 命令树本体（命令名 / usage / 说明 / `--json` 字段 / 端点 / 茧壁层级）
	//	· `core/cmd/zerg/exitcodes.go` —— 退码表（本页那张表的唯一真源）
	//	· `core/cmd/zerg/export.go`    —— `zerg help export` 的导出面（本页四列逐字取自它）
	//	· `core/internal/version`      —— 身份串（版本 / commit / build_time）
	//
	// ★ **✗ 整个 `core/cmd/zerg` 目录**：目录里还有上百件 `family_*.go` / `check_*.go` —— 改它们
	//   动实现体与判据体、不动本页，旧判据（逐字相等 + 整目录 diff）却把每一笔这样的提交判成红
	//   ⇒ 无关改动也逼着重编 + 重生成（收窄前的真实成本：命令页随每一次 `core/**` 提交翻红）。
	//   收窄后的判据 = 页内 commit 是 HEAD 祖先 ∧ 窄集自该提交零改动（`gcrPageCommitVerdict`），
	//   ✗ 不再是逐字相等；`build_time` 每次编译都变 ⇒ 也✗ 再逐字比。
	// ★ 窄集外仍能漂的那一面（见「剩余盲窗」注释）由本件别的牙兜：命令名对拍读的是 `main.go`
	//   **源码**（✗ 不看制品）· 用法串与 `zerg help` 现跑逐字对拍（`gcrConstUsages` 现读包内
	//   全部 `*.go`——窄集外的 usage 常量也逃不掉）⇒「命令面动了」在源码侧就红，不靠这四件。
	gcrSourceFaceRels = []string{
		gcrMainRel,                   // core/cmd/zerg/main.go
		"core/cmd/zerg/exitcodes.go", // 退码表
		"core/cmd/zerg/export.go",    // 导出面
		"core/internal/version",      // 身份串
	}
	gcrTargetSuffix  = [][2]string{{".zh-CN.md", "zh"}, {".en.md", "en"}}
	gcrLayers        = []string{"host", "node", "space"}
	gcrCountsMarkers = []string{"> **现读真源数**", "> **Counts as read**"}

	gcrCitationRE = regexp.MustCompile("[（(][^（）()]*§[^（）()]*[）)]")
	gcrPathRE     = regexp.MustCompile(`path:\s*\[\]string\{([^}]*)\}`)
	gcrConstRE    = regexp.MustCompile(`(?m)^\s*(?:const\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*:?=\s*"([^"]*)"\s*$`)
	gcrUsageLitRE = regexp.MustCompile(`usage:\s*"([^"]*)"`)
	gcrUsageIdRE  = regexp.MustCompile(`usage:\s*([A-Za-z_][A-Za-z0-9_]*)`)
	gcrPartsRE    = regexp.MustCompile(`"([^"]+)"`)
	gcrNumRE      = regexp.MustCompile(`\*\*(\d+)\*\*`)
	gcrCodeRE     = regexp.MustCompile("`([^`\n]+)`")
	gcrCommentRE  = regexp.MustCompile(`(?s)<!--.*?-->`)

	gcrColAnchors = map[string]*regexp.Regexp{
		"命令":          regexp.MustCompile("^`zerg(?: [^`]+)?`$"),
		"档":           regexp.MustCompile(`^D[123]$`),
		"本版":          regexp.MustCompile(`^\*{0,2}(?:已开放|未开放)\*{0,2}$`),
		"三态":          regexp.MustCompile(`^--dry-run(?: |·|$)`),
		"`--json` 字段": regexp.MustCompile("^(?:（无机器面）|[A-Za-z_][A-Za-z0-9_]*(?:,[A-Za-z_][A-Za-z0-9_]*)+)$"),
		"茧壁层级":        regexp.MustCompile("^`?(?:host|node|space)`?$"),
		// 「副作用面」列（新增 · 2026-10-13 · 承接项⑧）：闭集六值 —— 锚判该列取值不是自由文本。
		"副作用面": regexp.MustCompile(`^(?:只读|仓内写|仓外写|远端写|系统写|不确定)$`),
	}

	gcrPublicCols = map[string][]string{
		"命令清单": {"命令", "说明", "`--json` 字段", "投影的远端端点", "茧壁层级"},
		"危险动作": {"命令", "档", "三态", "`--confirm` 的目标", "副作用面", "它会动什么", "茧壁层级"},
		"退码表":  {"码", "名", "语义", "可重试性"},
	}
)

// gcrLJSON —— py `L`（locale 模板）的逐字 JSON（由 py 原件现读导出 · ✗ 不手抄）。
const gcrLJSON = "{\"zh\": {\"title\": \"参考-命令行\", \"h1\": \"参考-命令行\", \"lead\": [\"Code generated by core/cmd/zerg/gen_cli_reference.go — DO NOT EDIT. **本页由命令树现算生成**（`zerg gate check gen-cli-reference --emit`）：数据来自 `zerg help export` 与 `zerg help` 的现跑输出 —— 命令名、旗标、`--json` 字段、投影端点、茧壁层级、退码表**逐字来自命令树，一个都不是手写的**。\", \"改命令面 ⇒ 重跑生成器（`--check` 会判 drift：产物与现跑不一致就红）；本页的命令名与命令树源码**现算对拍 = 0 差**。\", \"本版现跑：命令面 **{total}** 条 = 已开放 **{open}** 条 + 危险动作 **{danger}** 条（危险动作里**一部分已开放**、其余未开放 —— 「哪些已开放」逐条以 `zerg help dangerous` 为准，未开放的那部分真跑一律拒执）。\", \"本页只讲**本版**：不写设计稿/调研稿/待办里的未来形状；导出物里指向过程档案的「出处」列**不进公开面**（公开面只讲虫族是什么）。\"], \"h_cmds\": \"命令族与全部命令\", \"cmds_note\": [\"「命令」列 = 命令树里的命令名；「用法（旗标）」列 = 命令树 `usage` 字段**逐字**（`zerg help` 的用法块是它的投影，生成器断言两者**逐字一致**）；\", \"其余四列（说明 / `--json` 字段 / 投影的远端端点 / 茧壁层级）逐字来自 `zerg help export`。\", \"单元格里的 `|` 按 markdown 规矩转义成 `\\\\|`（**唯一**的排版改写，未动任何字符）。\"], \"counts_note\": \"> **现读真源数**：命令树 **{total}** 条（开放命令 **{open}** 条 + 危险动作 **{danger}** 条）· 契约矩阵 **{matrix}** 格（must-fail {matrix}）· 空白 **{blanks}** · 豁免 **{exempt}**。\", \"h_open\": \"已开放（{n} 条）\", \"a_open\": \"已开放\", \"h_danger\": \"危险动作（{n} 条 · 已登记）\", \"a_danger\": \"危险动作\", \"th_open\": [\"命令\", \"用法（旗标）\", \"说明\", \"`--json` 字段\", \"投影的远端端点\", \"茧壁层级\"], \"th_danger\": [\"命令\", \"档\", \"三态\", \"`--confirm` 的目标\", \"副作用面\", \"它会动什么\", \"茧壁层级\"], \"danger_note\": \"危险动作的「档 / 三态 / `--confirm` 的目标 / 副作用面 / 它会动什么」逐字来自 `zerg help export`（导出物里还有一列「出处」= 把命令对回设计稿的追溯面，**本页不承载**）；导出物抬头逐字：`{counts}` —— 「哪些已开放」逐条以 `zerg help dangerous` 为准，未开放的那部分真跑一律**拒执**（退出码 2 = 不给结论），只有 `--dry-run` 的计划面可用。\", \"h_exit\": \"退出码表\", \"exit_note\": \"逐字来自 `zerg help export` 的退码表（唯一真源在 `core/cmd/zerg/exitcodes.go`；`zerg help exit-codes` 是它的自描述面）。\", \"th_exit\": [\"码\", \"名\", \"语义\", \"可重试性\"], \"h_json\": \"机器面形状要点\", \"json_note\": \"本节是**形状契约**的要点（真源 = 代码内 schema；`zerg help contract` 是它的自描述面）。\", \"json_body\": [\"**三层版本**（不许混成一层）：① 契约号 `schema:\\\"zerg/v1\\\"` —— 这份 `--json` 输出的**形状契约**是第几版；② 对象 schema 版本 —— 这枚卵/这份档案的**数据**是哪版；③ 组件版本（semver）—— 哪台机跑的哪个二进制。`zerg version --json` **三个都报**。\", \"**统一外层包封**（`--json` 的唯一出口，六键恒在）：`schema`（恒 `\\\"zerg/v1\\\"`）· `kind`（与命令一一对应、单数 CamelCase）· `items`（**恒数组**，空为 `[]`、永不为 `null`）· `meta`（`count` / `source`；多机目标进 `node`）· `warnings`（降级/未证实一律在这里**显式**报，不静默）· `truncated`（bool）；出错时才多一块 `__error__`。\", \"**字段稳定性承诺**：① 既有字段的名字/类型/语义**只增不改**（消费侧必须忽略未识别键）；② **不承诺字段序**——对象序只由显式排序旗标决定；③ 破坏性变更**走大版本**（旧主号继续可读）；④ 废弃四步：标注 deprecated（进 `warnings`）⇒ 文档删示例 ⇒ 导出处删字段 ⇒ 下一个大版本删字段。\", \"**拒绝规则**：请求未知字段 ⇒ 退码 `2` + 列全部合法字段；`--json` 不给字段 ⇒ 退码 `2` + stderr 列全部字段（schema 自描述）；`--schema` 主号不认 ⇒ 退码 `2`；`--json '*'` **不存在** —— 要的是逗号分隔的字段名。\", \"**三态输出**：人面（TTY 下的表格，只是渲染）· 行式面（非 TTY 或 `--plain`：制表符分隔，字段集与字段序与人面**一模一样**）· 机器面（`--json <字段>`：包封 + 点名字段，不因 TTY 改形状）。\"], \"h_config\": \"配置优先级\", \"config_note\": \"逐字来自 `zerg help export`（旗标最优先，内置默认兜底）。\", \"blk_decl\": \"Code generated by core/cmd/zerg/gen_cli_reference.go — DO NOT EDIT. **本块为生成物（勿手改）**：由 `core/cmd/zerg/gen_cli_reference.go` 照命令树**现算**后注入 —— 改命令面后重跑生成器（`--emit --target {rel}`）；判 drift 用同名 `--check --target {rel}`。本块里的命令名、用法串（旗标）、说明、`--json` 字段、投影端点与茧壁层级**逐字来自命令树，一个都不是手写的**。\", \"blk_danger_head\": \"危险动作（{n} 条 · 已登记）\", \"blk_danger_note\": \"导出物抬头逐字：`{counts}` —— 「哪些已开放」逐条以 `zerg help dangerous` 为准；本表五列（档 / 三态 / `--confirm` 的目标 / 副作用面 / 它会动什么）逐字来自 `zerg help export`；未开放的那部分真跑一律**拒执**（退出码 2 = 不给结论），只有 `--dry-run` 的计划面可用。\", \"footer\": \"本页是**导出物**：真源永远是命令树源码（`{main}`）与代码内 schema；改命令面请重跑 `zerg gate check gen-cli-reference --emit`，并用 `--check` 复判。\"}, \"en\": {\"title\": \"CLI reference (参考-命令行)\", \"h1\": \"参考-命令行\", \"lead\": [\"Code generated by core/cmd/zerg/gen_cli_reference.go — DO NOT EDIT. **This page is generated from the command tree** (`zerg gate check gen-cli-reference --emit`): the data comes from live runs of `zerg help export` and `zerg help` — command names, flags, `--json` fields, projected endpoints, wall layer and the exit-code table are quoted **verbatim from the command tree; not one name is hand-written**.\", \"Change the command surface ⇒ re-run the generator (`--check` fails on drift); the command names on this page are cross-checked against the command tree source with **0 diff**.\", \"This build (live run): **{total}** commands = **{open}** open + **{danger}** dangerous (**a part of them is already open** and the rest is not — for which ones are open, `zerg help dangerous` is the per-command authority, and the rest refuse to run for real).\", \"This page documents **this build only**: it carries no future shapes from design notes, research notes or to-do lists, and the export's provenance column (design-doc references) **is left out of the published face**.\", \"Cells are quoted verbatim from the CLI's own (Chinese) help output — the contract on this page is the command names, flags, field lists and exit codes; the CLI's own prose is not translated here.\"], \"h_cmds\": \"命令族与全部命令\", \"cmds_note\": [\"The “command” column is the command name from the tree; the “usage (flags)” column quotes the tree's own `usage` field verbatim (the `zerg help` usage block is its projection; the generator asserts the two agree character for character).\", \"The other four columns (summary / `--json` fields / projected endpoint / wall layer) are quoted verbatim from `zerg help export`.\", \"A literal `|` inside a cell is escaped as `\\\\|` for markdown (the **only** typographic change; no character was altered).\"], \"counts_note\": \"> **Counts as read** — command tree **{total}** (open commands **{open}** + dangerous actions **{danger}**) · contract matrix **{matrix}** cases (must-fail {matrix}) · blank **{blanks}** · exempt **{exempt}**.\", \"h_open\": \"Open ({n})\", \"a_open\": \"已开放\", \"h_danger\": \"Dangerous actions ({n} · registered)\", \"a_danger\": \"危险动作\", \"th_open\": [\"command\", \"usage (flags)\", \"summary\", \"`--json` fields\", \"projected endpoint\", \"wall layer\"], \"th_danger\": [\"command\", \"tier\", \"three states\", \"`--confirm` target\", \"side effect\", \"what it touches\", \"wall layer\"], \"danger_note\": \"“tier / three states / `--confirm` target / side effect / what it touches” are quoted verbatim from `zerg help export` (the export also carries a provenance column mapping a command back to the design doc — **this page does not carry it**); the export's header, verbatim: `{counts}` — for which ones are open, `zerg help dangerous` is the per-command authority; the ones not open yet refuse to run for real (exit 2 = no verdict) and only their `--dry-run` plan face is available.\", \"h_exit\": \"退出码表\", \"exit_note\": \"Quoted verbatim from `zerg help export` (single source of truth: `core/cmd/zerg/exitcodes.go`; `zerg help exit-codes` is its self-describing face).\", \"th_exit\": [\"code\", \"name\", \"meaning\", \"retryable\"], \"h_json\": \"机器面形状要点\", \"json_note\": \"This section is the outline of the **shape contract** (source of truth: the schema in code; `zerg help contract` is its self-describing face).\", \"json_body\": [\"**Three version layers** (never collapsed into one): (1) contract id `schema:\\\"zerg/v1\\\"` — which version of the `--json` **shape contract** this is; (2) object schema version — which version of the **data** (an egg, an archive); (3) component version (semver) — which binary on which machine. `zerg version --json` reports **all three**.\", \"**One outer envelope** (the only `--json` exit, six keys always present): `schema` (always `\\\"zerg/v1\\\"`) · `kind` (one per command, singular CamelCase) · `items` (**always an array**, `[]` when empty, never `null`) · `meta` (`count` / `source`; a multi-machine target adds `node`) · `warnings` (degradation is always reported **explicitly** here, never silently) · `truncated` (bool); an `__error__` block appears only on failure.\", \"**Field stability promise**: (1) existing field names/types/meanings are **only added to, never changed** (consumers must ignore unknown keys); (2) **field order is not promised** — object order is decided only by an explicit sort flag; (3) breaking changes **take a major version bump** (the old major keeps working); (4) the four deprecation steps: mark deprecated (into `warnings`) ⇒ drop examples from docs ⇒ drop the field from the export ⇒ drop it in the next major.\", \"**Rejection rules**: an unknown field ⇒ exit `2` + list every legal field; `--json` with no fields ⇒ exit `2` + list every field on stderr (the schema describing itself); an unrecognised `--schema` major ⇒ exit `2`; `--json '*'` **does not exist** — what it wants is a comma-separated field list.\", \"**Three output modes**: human face (aligned table on a TTY, rendering only) · line face (non-TTY or `--plain`: tab-separated, with a field set and field order **identical** to the machine face (`--json <字段>`: envelope + named fields, whose shape does not change with the TTY).\"], \"h_config\": \"配置优先级\", \"config_note\": \"Quoted verbatim from `zerg help export` (flags first, built-in defaults last).\", \"blk_decl\": \"Code generated by core/cmd/zerg/gen_cli_reference.go — DO NOT EDIT. **This block is a generated artifact (do not hand-edit)**: injected by `core/cmd/zerg/gen_cli_reference.go`, computed live from the command tree — after changing the command surface, re-run the generator (`--emit --target {rel}`); judge drift with `--check --target {rel}`. Command names, usage strings (flags), summaries, `--json` fields, projected endpoints and wall layer are quoted **verbatim from the command tree; not one of them is hand-written**.\", \"blk_danger_head\": \"Dangerous actions ({n} · registered)\", \"blk_danger_note\": \"The export's header, verbatim: `{counts}` — for which actions are open, `zerg help dangerous` is the per-command authority; this table's five columns (tier / three states / `--confirm` target / side effect / what it touches) are quoted verbatim from `zerg help export`; the ones not open yet refuse to run for real (exit 2 = no verdict) and only their `--dry-run` plan face is available.\", \"footer\": \"This page is an **export product**: the source of truth is always the command tree source (`{main}`) plus the schema in code. After changing the command surface, re-run `zerg gate check gen-cli-reference --emit` and re-judge with `--check`.\"}}"

// gcrHelpText —— py argparse `--help`/`-h` 的逐字 stdout（宽度 80 · 非 TTY 时 argparse 的取值）。
const gcrHelpText = "usage: gen-cli-reference.py [-h] (--emit | --check) [--target REL]\n\n正式面「参考-命令行」页的生成器（zh/en 同构）\n\noptions:\n  -h, --help    show this help message and exit\n  --emit        生成/重生成两份页\n  --check       判 drift + 命令名对拍（不改仓）\n  --target REL  **追加**生成区目标：把手写正文那件的**生成区**（BEGIN/END 标记之间）也生成/复判（可重复；件名须以\n                `*.zh-CN.md` / `*.en.md` 结尾 —— 语种按件名现判）。★ 默认档已把\n                `publish/docs/` 下带成对标记的那几件**现读派生**进来（缺口 `Q-151`）⇒\n                本旗标只在要**额外**指定时用\n"

// ── 数据结构 ────────────────────────────────────────────────────────

type gcrLocale struct {
	Title         string   `json:"title"`
	H1            string   `json:"h1"`
	Lead          []string `json:"lead"`
	HCmds         string   `json:"h_cmds"`
	CmdsNote      []string `json:"cmds_note"`
	CountsNote    string   `json:"counts_note"`
	HOpen         string   `json:"h_open"`
	AOpen         string   `json:"a_open"`
	HDanger       string   `json:"h_danger"`
	ADanger       string   `json:"a_danger"`
	ThOpen        []string `json:"th_open"`
	ThDanger      []string `json:"th_danger"`
	DangerNote    string   `json:"danger_note"`
	HExit         string   `json:"h_exit"`
	ExitNote      string   `json:"exit_note"`
	ThExit        []string `json:"th_exit"`
	HJson         string   `json:"h_json"`
	JsonNote      string   `json:"json_note"`
	JsonBody      []string `json:"json_body"`
	HConfig       string   `json:"h_config"`
	ConfigNote    string   `json:"config_note"`
	BlkDecl       string   `json:"blk_decl"`
	BlkDangerHead string   `json:"blk_danger_head"`
	BlkDangerNote string   `json:"blk_danger_note"`
	Footer        string   `json:"footer"`
}

type gcrIdent struct {
	component string
	version   string
	commit    string
	buildTime string
	line      string
}

type gcrFace struct {
	sha         string
	dirtySuffix bool
	head        string
	// moved —— 窄集自 `sha` 到 HEAD 的**已提交**改动（✗ 不是整个 `core/cmd/zerg` 目录）。
	moved []string
	// workDirty —— 窄集在**工作树**里未提交的改动（含未跟踪件）。
	// `git diff <sha>..HEAD` 只看得见已提交历史 ⇒ 这一格补上未提交漂移（GAP-20261014-64）。
	workDirty []string
}

type gcrData struct {
	ident      string
	counts     string
	nOpen      int
	nDanger    int
	matrix     map[string]int
	open       [][]string
	danger     [][]string
	exit       [][]string
	exitTail   []string
	cfg        string
	usage      map[string]string
	namesPage  []string
	namesTree  []string
	nHelpAgree int
	liveIdent  gcrIdent
	sourceFace gcrFace
	// pageCommits —— 页里读到的那几枚 commit（去重保序 · `gcrCheck` 现读填 · 只为收尾那行人话）。
	pageCommits []string
}

type gcrPages struct {
	zh   string
	en   string
	data *gcrData
}

// ── Python 字符串原语（逐处自写，✗ 不用标准库近似）─────────────────────

func gcrPyIsSpace(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', ' ', 0x1c, 0x1d, 0x1e, 0x1f, 0x85, 0xa0,
		0x1680, 0x2028, 0x2029, 0x202f, 0x205f, 0x3000:
		return true
	}
	return r >= 0x2000 && r <= 0x200a
}

func gcrPyStrip(s string) string { return strings.TrimFunc(s, gcrPyIsSpace) }

// gcrPyText —— Python 文本模式读取：universal newlines（`\r\n` / `\r` ⇒ `\n`）。
func gcrPyText(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return s
}

// gcrPySplitlines —— py str.splitlines 的分行集。
func gcrPySplitlines(s string) []string {
	var out []string
	var cur strings.Builder
	rs := []rune(s)
	for i := 0; i < len(rs); i++ {
		r := rs[i]
		switch r {
		case '\n', '\v', '\f', 0x1c, 0x1d, 0x1e, 0x85, 0x2028, 0x2029:
			out = append(out, cur.String())
			cur.Reset()
		case '\r':
			if i+1 < len(rs) && rs[i+1] == '\n' {
				i++
			}
			out = append(out, cur.String())
			cur.Reset()
		default:
			cur.WriteRune(r)
		}
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

// gcrRuneHead —— py 切片 `s[:n]`（按码点）。
func gcrRuneHead(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		r = r[:n]
	}
	return string(r)
}

// gcrPadRight —— py `"%-Ns"`：按**码点**右侧补空格到 n。
func gcrPadRight(s string, n int) string {
	c := len([]rune(s))
	if c >= n {
		return s
	}
	return s + strings.Repeat(" ", n-c)
}

// gcrPyJoin —— posixpath.join：后段绝对路径丢弃前段 · 空段按 py 语义（`join("a","")=="a/"`）·
// `..` 原样留串。★ 照 CPython posixpath.join 的逐段规则（✗ 不「跳过空段」）。
func gcrPyJoin(a string, p ...string) string {
	path := a
	for _, b := range p {
		if strings.HasPrefix(b, "/") {
			path = b
		} else if path == "" || strings.HasSuffix(path, "/") {
			path += b
		} else {
			path += "/" + b
		}
	}
	return path
}

// gcrPyDirname —— posixpath.split 头部语义（剥尾斜杠）。
func gcrPyDirname(p string) string {
	i := strings.LastIndex(p, "/")
	if i < 0 {
		return ""
	}
	head := p[:i+1]
	if strings.Trim(head, "/") != "" {
		head = strings.TrimRight(head, "/")
	}
	return head
}

// gcrPyAbspath —— posixpath.abspath：相对则并 cwd，再词法归一。
func gcrPyAbspath(cwd, start string) string {
	if !strings.HasPrefix(start, "/") {
		start = gcrPyJoin(cwd, start)
	}
	return gcrPyNormpath(start)
}

// gcrPyNormpath —— posixpath.normpath（保留恰两个前导斜杠 · 折叠 `//` 与 `.`/`..`）。
func gcrPyNormpath(p string) string {
	if p == "" {
		return "."
	}
	initial := ""
	if strings.HasPrefix(p, "/") {
		initial = "/"
		if strings.HasPrefix(p, "//") && !strings.HasPrefix(p, "///") {
			initial = "//"
		}
	}
	var comps []string
	for _, c := range strings.Split(p, "/") {
		if c == "" || c == "." {
			continue
		}
		comps = append(comps, c)
	}
	var out []string
	for _, c := range comps {
		if c == ".." && len(out) > 0 && out[len(out)-1] != ".." {
			out = out[:len(out)-1]
			continue
		}
		if c == ".." && initial == "/" {
			continue
		}
		out = append(out, c)
	}
	joined := strings.Join(out, "/")
	if initial == "//" {
		if joined == "" {
			return "//"
		}
		return "//" + joined
	}
	if initial == "/" {
		return "/" + joined
	}
	if joined == "" {
		return "."
	}
	return joined
}

// gcrPyStrRepr —— py repr(str) 的常规子集（用于异常文案里回印字段）。
func gcrPyStrRepr(s string) string {
	quote := byte('\'')
	if strings.Contains(s, "'") && !strings.Contains(s, "\"") {
		quote = '"'
	}
	var b strings.Builder
	b.WriteByte(quote)
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString("\\\\")
		case '\n':
			b.WriteString("\\n")
		case '\r':
			b.WriteString("\\r")
		case '\t':
			b.WriteString("\\t")
		default:
			if r == rune(quote) {
				b.WriteByte('\\')
			}
			b.WriteRune(r)
		}
	}
	b.WriteByte(quote)
	return b.String()
}

// ── 文件系统小工具（照 py os.path 语义）─────────────────────────────

func gcrIsFile(p string) bool {
	st, err := os.Stat(p) // py os.path.isfile 跟随符号链接 ⇒ os.Stat
	return err == nil && !st.IsDir()
}

func gcrIsDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

// gcrRead —— py `open(p, encoding="utf-8").read()`（universal newlines）。
func gcrRead(p string) (string, bool) {
	b, err := os.ReadFile(p)
	if err != nil {
		return "", false
	}
	return gcrPyText(string(b)), true
}

func gcrReadOr(p string) string {
	s, _ := gcrRead(p)
	return s
}

func gcrGlob(pattern string) []string {
	m, err := filepath.Glob(pattern)
	if err != nil {
		return nil
	}
	sort.Strings(m)
	return m
}

func gcrContains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func gcrUniq(list []string) []string {
	var out []string
	for _, x := range list {
		if !gcrContains(out, x) {
			out = append(out, x)
		}
	}
	return out
}

func gcrCoalesce(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// gcrRunCmd —— py `subprocess.run([name]+args, capture_output=True, text=True, stdin=DEVNULL)`。
func gcrRunCmd(name string, args []string) (int, string, string) {
	cmd := exec.Command(name, args...)
	cmd.Stdin = nil // DEVNULL 语义
	var ob, eb bytes.Buffer
	cmd.Stdout = &ob
	cmd.Stderr = &eb
	err := cmd.Run()
	rc := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			rc = ee.ExitCode()
		} else {
			rc = -1
		}
	}
	return rc, gcrPyText(ob.String()), gcrPyText(eb.String())
}

// ══ 取数：现跑的命令面 ═══════════════════════════════════════════════

func gcrFindRoot(start string) string {
	d := start
	for i := 0; i < 8; i++ {
		if gcrIsFile(gcrPyJoin(d, gcrMainRel)) {
			return d
		}
		parent := gcrPyDirname(d)
		if parent == d {
			break
		}
		d = parent
	}
	return ""
}

// gcrStripCitations —— 剥掉单元格里的 `（§…）` 类引用（循环至不动点）。
func gcrStripCitations(text string) string {
	prev := "\x00"
	out := text
	for prev != out {
		prev = out
		out = gcrCitationRE.ReplaceAllString(out, "")
	}
	return gcrPyStrip(out)
}

// gcrConstUsages —— 包内「`名字 = "字面量"`」表（首见为准 · 遍历序 = 排序后的 *.go）。
func gcrConstUsages(root string) map[string]string {
	out := map[string]string{}
	pkg := gcrPyDirname(gcrPyJoin(root, gcrMainRel))
	for _, name := range gcrGlob(gcrPyJoin(pkg, "*.go")) {
		src := gcrReadOr(name)
		for _, m := range gcrConstRE.FindAllStringSubmatch(src, -1) {
			if _, ok := out[m[1]]; !ok {
				out[m[1]] = m[2]
			}
		}
	}
	return out
}

// gcrTreeFromSource —— 命令树源码 → [(命令名, 用法串)]（第二份真源）。
//
// ★ 拆件后（GAP-20261011-03）：表搬进 cmdtable_*.go ⇒ 读**合成真源**（形状与拆件前逐字同构）。
func gcrTreeFromSource(root string) [][2]string {
	src, err := readCmdTreeSynth(root)
	if err != nil {
		return nil
	}
	start := strings.Index(src, "commands = []*command{")
	if start < 0 {
		return nil
	}
	end := strings.Index(src[start:], "\n\t}\n")
	if end < 0 {
		return nil
	}
	end += start
	seg := src[start:end]
	consts := gcrConstUsages(root)
	matches := gcrPathRE.FindAllStringSubmatchIndex(seg, -1)
	var out [][2]string
	for i, m := range matches {
		parts := gcrPartsRE.FindAllStringSubmatch(seg[m[2]:m[3]], -1)
		if len(parts) == 0 {
			continue
		}
		nxt := len(seg)
		if i+1 < len(matches) {
			nxt = matches[i+1][0]
		}
		block := seg[m[1]:nxt]
		var usage string
		if u := gcrUsageLitRE.FindStringSubmatch(block); u != nil {
			usage = u[1]
		} else if id := gcrUsageIdRE.FindStringSubmatch(block); id != nil {
			usage = consts[id[1]]
		}
		var names []string
		for _, p := range parts {
			names = append(names, p[1])
		}
		out = append(out, [2]string{strings.Join(names, " "), usage})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// gcrUsageLines —— `zerg help` 的用法串 → [用法行…]。
func gcrUsageLines(binz string) ([]string, string) {
	rc, out, errOut := gcrRunCmd(binz, []string{"help"})
	if rc != 0 {
		return nil, fmt.Sprintf("`zerg help` 退码 %d（%s）", rc,
			gcrRuneHead(gcrPyStrip(gcrCoalesce(errOut, out)), 200))
	}
	var lines []string
	inUsage := false
	for _, line := range gcrPySplitlines(out) {
		if gcrPyStrip(line) == "用法:" {
			inUsage = true
			continue
		}
		if inUsage {
			if gcrPyStrip(line) == "" {
				inUsage = false
				continue
			}
			s := gcrPyStrip(line)
			if strings.HasPrefix(s, "zerg ") || s == "zerg" {
				lines = append(lines, s)
			}
		}
	}
	if len(lines) == 0 {
		return nil, "`zerg help` 里没找到用法行（空转 = 假覆盖）"
	}
	return lines, ""
}

// gcrMapUsage —— 用法行按**最长命令名**归属到各条命令。
func gcrMapUsage(paths, lines []string) map[string]string {
	order := append([]string{}, paths...)
	sort.SliceStable(order, func(i, j int) bool {
		return len(strings.Fields(order[i])) > len(strings.Fields(order[j]))
	})
	out := map[string]string{}
	used := map[int]bool{}
	for _, path := range order {
		pref := "zerg " + path
		for i, line := range lines {
			if used[i] {
				continue
			}
			if line == pref || strings.HasPrefix(line, pref+" ") {
				out[path] = line
				used[i] = true
				break
			}
		}
	}
	return out
}

// ── 导出物表格解析（列锚按表头名）─────────────────────────────────────

func gcrParseExportTable(lines []string, heading string) ([]string, [][]string, error) {
	start := -1
	for i, l := range lines {
		if strings.HasPrefix(l, "## ") && strings.Contains(l, heading) {
			start = i
			break
		}
	}
	if start < 0 {
		return nil, nil, fmt.Errorf("导出物里找不到小节：%s", heading)
	}
	var header []string
	var rows [][]string
	for _, l := range lines[start+1:] {
		if strings.HasPrefix(l, "## ") {
			break
		}
		if !strings.HasPrefix(l, "|") {
			continue
		}
		if gcrOnlyDashesSpace(l) {
			continue
		}
		cells := gcrSplitCells(l)
		if header == nil {
			header = cells
			continue
		}
		rows = append(rows, cells)
	}
	if len(header) == 0 {
		return nil, nil, fmt.Errorf("导出物小节里没有表：%s", heading)
	}
	return header, rows, nil
}

// gcrOnlyDashesSpace —— py `set(l) <= set("|- ")`（分隔行）。
func gcrOnlyDashesSpace(l string) bool {
	for _, r := range l {
		if r != '|' && r != '-' && r != ' ' {
			return false
		}
	}
	return true
}

// gcrSplitCells —— py `[c.strip().replace("\\|","|") for c in re.split(r"(?<!\\)\|", l.strip().strip("|"))]`。
func gcrSplitCells(l string) []string {
	body := strings.Trim(gcrPyStrip(l), "|")
	// 手写按「未被转义的 `|`」切分（RE2 无后顾）。
	var segs []string
	var cur strings.Builder
	rs := []rune(body)
	for i := 0; i < len(rs); i++ {
		if rs[i] == '\\' && i+1 < len(rs) && rs[i+1] == '|' {
			cur.WriteRune('\\')
			cur.WriteRune('|')
			i++
			continue
		}
		if rs[i] == '|' {
			segs = append(segs, cur.String())
			cur.Reset()
			continue
		}
		cur.WriteRune(rs[i])
	}
	segs = append(segs, cur.String())
	out := make([]string, len(segs))
	for i, c := range segs {
		out[i] = strings.ReplaceAll(gcrPyStrip(c), "\\|", "|")
	}
	return out
}

func gcrAnchorOf(name string) *regexp.Regexp { return gcrColAnchors[name] }

// gcrDistributions —— 把 surplus 个多出来的格子分给 k 个自由列的全部方案（靠前的先分）。
func gcrDistributions(surplus, k int) [][]int {
	if k == 1 {
		return [][]int{{surplus}}
	}
	var out [][]int
	for first := surplus; first >= 0; first-- {
		for _, rest := range gcrDistributions(surplus-first, k-1) {
			out = append(out, append([]int{first}, rest...))
		}
	}
	return out
}

// gcrAlignRow —— 把被单元格内 `|` 切开的行按锚列拼回表头列尺寸。
func gcrAlignRow(cells, header []string) ([]string, error) {
	n := len(header)
	if len(cells) == n {
		return cells, nil
	}
	if len(cells) < n {
		return nil, fmt.Errorf("列数 %d < 表头 %d 列：%s", len(cells), n, strings.Join(cells, " | "))
	}
	var free []int
	for i, name := range header {
		if gcrAnchorOf(name) == nil {
			free = append(free, i)
		}
	}
	if len(free) == 0 {
		return nil, fmt.Errorf("列数 %d ≠ 表头 %d 列，且表头里没有自由列可吸收多出的格子：%s",
			len(cells), n, strings.Join(header, " | "))
	}
	for _, combo := range gcrDistributions(len(cells)-n, len(free)) {
		take := make([]int, n)
		for i := range take {
			take[i] = 1
		}
		for k, i := range free {
			take[i] += combo[k]
		}
		var out []string
		p := 0
		ok := true
		for j, name := range header {
			seg := cells[p : p+take[j]]
			p += take[j]
			rx := gcrAnchorOf(name)
			if rx != nil && !rx.MatchString(seg[0]) {
				ok = false
				break
			}
			out = append(out, strings.Join(seg, " | "))
		}
		if ok {
			return out, nil
		}
	}
	return nil, fmt.Errorf("列数 %d ≠ 表头 %d 列且按表头名拼不回去（锚列对不上）：%s",
		len(cells), n, strings.Join(cells, " | "))
}

// gcrCellsForTable —— 按表头名取公开列 · 每格先剥设计稿引用。
func gcrCellsForTable(lines []string, heading string, cols []string, layerCol string) ([][]string, error) {
	header, rawRows, err := gcrParseExportTable(lines, heading)
	if err != nil {
		return nil, err
	}
	var idx []int
	for _, name := range cols {
		found := -1
		for i, h := range header {
			if h == name {
				found = i
				break
			}
		}
		if found < 0 {
			return nil, fmt.Errorf("导出物表头里找不到列名 %q（现读表头：%s）", name, strings.Join(header, " | "))
		}
		idx = append(idx, found)
	}
	var out [][]string
	for _, raw := range rawRows {
		cells, err := gcrAlignRow(raw, header)
		if err != nil {
			return nil, err
		}
		if layerCol != "" {
			li := -1
			for i, h := range header {
				if h == layerCol {
					li = i
					break
				}
			}
			layer := strings.Trim(cells[li], "`")
			if !gcrContains(gcrLayers, layer) {
				return nil, fmt.Errorf("茧壁层级列不是闭集三值（%q）：%s", layer, strings.Join(cells, " | "))
			}
		}
		row := make([]string, len(idx))
		for k, i := range idx {
			row[k] = gcrStripCitations(cells[i])
		}
		out = append(out, row)
	}
	return out, nil
}

// ══ 计数 ════════════════════════════════════════════════════════════

func gcrMatrixCounts(root string) (map[string]int, string) {
	bp := gcrPyJoin(root, "scripts", "gates", "cli-contract-baseline.json")
	mp := gcrPyJoin(root, "core", "cmd", "zerg", "testdata", "cli-matrix.json")
	bb, err := os.ReadFile(bp)
	if err != nil {
		return nil, fmt.Sprintf("契约矩阵计数读不出来（%s）—— 不猜，拒绝出页", err.Error())
	}
	mb, err := os.ReadFile(mp)
	if err != nil {
		return nil, fmt.Sprintf("契约矩阵计数读不出来（%s）—— 不猜，拒绝出页", err.Error())
	}
	var b map[string]any
	var m map[string]any
	if err := json.Unmarshal(bb, &b); err != nil {
		return nil, fmt.Sprintf("契约矩阵计数读不出来（%s）—— 不猜，拒绝出页", err.Error())
	}
	if err := json.Unmarshal(mb, &m); err != nil {
		return nil, fmt.Sprintf("契约矩阵计数读不出来（%s）—— 不猜，拒绝出页", err.Error())
	}
	mf, ok1 := b["mustfail_cases"]
	bl, ok2 := b["blanks"]
	if !ok1 || !ok2 {
		return nil, "契约矩阵计数格读不出来（missing key）—— 不猜，拒绝出页"
	}
	mfF, ok1 := gcrToInt(mf)
	blF, ok2 := gcrToInt(bl)
	if !ok1 || !ok2 {
		return nil, "契约矩阵计数格读不出来（not int）—— 不猜，拒绝出页"
	}
	exempt := 0
	if ex, ok := m["exemptions"]; ok && ex != nil {
		if arr, ok := ex.([]any); ok {
			exempt = len(arr)
		}
	}
	return map[string]int{"mustfail": mfF, "blanks": blF, "exempt": exempt}, ""
}

func gcrToInt(v any) (int, bool) {
	switch x := v.(type) {
	case float64:
		return int(x), true
	case string:
		n, err := strconv.Atoi(x)
		return n, err == nil
	case bool:
		return int(map[bool]int{false: 0, true: 1}[x]), true
	}
	return 0, false
}

func gcrRenderCountsNote(loc string, d *gcrData) string {
	return gcrFmt(gcrLoc(loc).CountsNote, map[string]string{
		"total": strconv.Itoa(d.nOpen + d.nDanger), "open": strconv.Itoa(d.nOpen),
		"danger": strconv.Itoa(d.nDanger), "matrix": strconv.Itoa(d.matrix["mustfail"]),
		"blanks": strconv.Itoa(d.matrix["blanks"]), "exempt": strconv.Itoa(d.matrix["exempt"]),
	})
}

// ══ 取数：live_data ═════════════════════════════════════════════════

func gcrLiveData(root string) (*gcrData, string) {
	binz := gcrPyJoin(root, "bin", "zerg")
	if !gcrIsFile(binz) {
		return nil, "找不到 bin/zerg —— 先跑 `zerg build all --only cli` " +
			"出**新**制品（旧制品会把新命令当未知命令静默漏掉）"
	}
	paths := gcrTreeFromSource(root)
	if paths == nil {
		return nil, fmt.Sprintf("命令树读不出来（%s 空/形状变了）—— 判据不可判", gcrMainRel)
	}
	umapAll := map[string]string{}
	var pathKeys []string
	for _, p := range paths {
		umapAll[p[0]] = p[1]
		pathKeys = append(pathKeys, p[0])
	}

	tmp, terr := os.MkdirTemp("", "zerg-cli-ref-")
	if terr != nil {
		return nil, fmt.Sprintf("临时目录建不出来（%s）—— 不猜", terr.Error())
	}
	defer os.RemoveAll(tmp)

	rc, out, errOut := gcrRunCmd(binz, []string{"help", "export", "--out", tmp, "--yes"})
	if rc != 0 {
		return nil, fmt.Sprintf("`zerg help export --out` 退码 %d（%s）", rc,
			gcrRuneHead(gcrPyStrip(gcrCoalesce(errOut, out)), 200))
	}
	cands := gcrGlob(gcrPyJoin(tmp, "导出-命令面帮助-*.md"))
	if len(cands) == 0 {
		return nil, "导出物没落盘（空转 = 假覆盖）"
	}
	exp := gcrPySplitlines(gcrReadOr(cands[len(cands)-1]))

	ident, counts := "", ""
	// ★ 缺口 `GAP-20261018-13`（本笔）：原来只看导出物的**前 10 行**（py 原件的 `exp[:10]`），
	//   而 `zerg help export` 此后加了 8 行 YAML frontmatter ⇒ `> 命令面身份：` / `> 本版：`
	//   被推到第 13/15 行 ⇒ 生成器**读不到计数**，`--check` 恒 rc=2「导出物抬头的计数读不出来」，
	//   命令页从此**无法重生成**（前端门 `check-public-face-commands` 因此对新命令恒红）。
	//   修法只改**扫描窗口**，判据（前缀逐字 + 计数正则）一字未动：前 40 行足够覆盖
	//   frontmatter ＋ 两个抬头行，且不含任何表格行（表格在空行之后）。
	lim := len(exp)
	if lim > 40 {
		lim = 40
	}
	for _, l := range exp[:lim] {
		if strings.HasPrefix(l, "> 命令面身份：") {
			parts := strings.SplitN(l, "：", 2)
			ident = strings.Trim(gcrPyStrip(parts[1]), "`")
		}
		if strings.HasPrefix(l, "> 本版：") {
			parts := strings.SplitN(l, "：", 2)
			counts = gcrPyStrip(parts[1])
		}
	}
	nums := gcrNumRE.FindAllStringSubmatch(counts, -1)
	if len(nums) != 2 {
		return nil, fmt.Sprintf("导出物抬头的计数读不出来：%s", gcrPyStrRepr(counts))
	}
	nOpen, _ := strconv.Atoi(nums[0][1])
	nDanger, _ := strconv.Atoi(nums[1][1])

	openRows, err := gcrCellsForTable(exp, "命令清单", gcrPublicCols["命令清单"], "茧壁层级")
	if err != nil {
		return nil, fmt.Sprintf("导出物表格解析不上（%s）—— 不猜，拒绝出页", err.Error())
	}
	dangerRows, err := gcrCellsForTable(exp, "危险动作", gcrPublicCols["危险动作"], "茧壁层级")
	if err != nil {
		return nil, fmt.Sprintf("导出物表格解析不上（%s）—— 不猜，拒绝出页", err.Error())
	}
	exitRows, err := gcrCellsForTable(exp, "退码表", gcrPublicCols["退码表"], "")
	if err != nil {
		return nil, fmt.Sprintf("导出物表格解析不上（%s）—— 不猜，拒绝出页", err.Error())
	}
	// ② 冻结下限：退码表 ≥ 2 行（`占号纪律` / `门禁直通档` 两条纪律行）· 命令/危险两表须与抬头
	//   现读计数逐格相等（`nOpen` / `nDanger` 即当次读数）—— 任一不符 ⇒ 判「导出物自相矛盾」（下限只许增）。
	if len(openRows) != nOpen || len(dangerRows) != nDanger || len(exitRows) < 2 {
		return nil, fmt.Sprintf("导出物自相矛盾：抬头说 %d 开/%d 危，表里读到 %d/%d，退码 %d 行",
			nOpen, nDanger, len(openRows), len(dangerRows), len(exitRows))
	}

	var tail []string
	for _, l := range exp {
		s := gcrPyStrip(l)
		if strings.HasPrefix(s, "占号纪律") || strings.HasPrefix(s, "门禁直通档") {
			tail = append(tail, s)
		}
	}
	if len(tail) != 2 {
		return nil, fmt.Sprintf("退码表的两条纪律行读不出来（读到 %d 条）", len(tail))
	}
	var cfg []string
	for _, l := range exp {
		s := gcrPyStrip(l)
		if s != "" && !strings.HasPrefix(l, "#") && !strings.HasPrefix(l, ">") &&
			!strings.HasPrefix(l, "|") && !strings.HasPrefix(l, "-") {
			cfg = append(cfg, s)
		}
	}
	cfgLine := ""
	for _, l := range cfg {
		if strings.HasPrefix(l, "旗标 >") || strings.Contains(l, "环境变量") {
			cfgLine = l
			break
		}
	}
	if cfgLine == "" {
		return nil, "配置优先级行读不出来"
	}

	usages, why := gcrUsageLines(binz)
	if usages == nil {
		return nil, why
	}
	helpMap := gcrMapUsage(pathKeys, usages)
	var hmKeys []string
	for k := range helpMap {
		hmKeys = append(hmKeys, k)
	}
	sort.Strings(hmKeys)
	for _, path := range hmKeys {
		line := helpMap[path]
		if umapAll[path] != line {
			return nil, fmt.Sprintf("两处真源不一致：`zerg help` 的用法行与命令树 usage 不同（%s）\n    help: %s\n    tree: %s",
				path, line, umapAll[path])
		}
	}
	nHelp := 0
	for _, p := range pathKeys {
		if _, ok := helpMap[p]; ok {
			nHelp++
		}
	}
	var missUsage []string
	for _, p := range pathKeys {
		if umapAll[p] == "" {
			missUsage = append(missUsage, p)
		}
	}
	if len(missUsage) > 0 {
		return nil, fmt.Sprintf("这些命令在命令树里没有 usage 串（旗标列没法现算）：%s", strings.Join(missUsage, " / "))
	}

	var namesPage []string
	for _, r := range append(append([][]string{}, openRows...), dangerRows...) {
		n := strings.ReplaceAll(strings.Trim(r[0], "`"), "zerg ", "")
		namesPage = append(namesPage, n)
	}

	mc, mcErr := gcrMatrixCounts(root)
	if mcErr != "" {
		return nil, mcErr
	}

	return &gcrData{
		ident: ident, counts: counts, nOpen: nOpen, nDanger: nDanger,
		matrix: mc, open: openRows, danger: dangerRows, exit: exitRows,
		exitTail: tail, cfg: cfgLine, usage: umapAll,
		namesPage: namesPage, namesTree: pathKeys, nHelpAgree: nHelp,
	}, ""
}

// ══ 身份行对拍（三腿）════════════════════════════════════════════════

func gcrPageIdentityLines(text string) []string {
	var out []string
	for _, l := range gcrPySplitlines(text) {
		s := gcrPyStrip(l)
		if strings.HasPrefix(s, gcrIdentPrefix) {
			v := gcrPyStrip(strings.Trim(strings.TrimSpace(s[len(gcrIdentPrefix):]), "`"))
			if v != "" && !gcrContains(out, v) {
				out = append(out, v)
			}
		}
	}
	return out
}

func gcrIdentFields(ident string) (gcrIdent, bool) {
	t := strings.Fields(strings.Trim(gcrPyStrip(ident), "`"))
	if len(t) != 4 || t[2] == "" || t[3] == "" {
		return gcrIdent{}, false
	}
	return gcrIdent{component: t[0], version: t[1], commit: t[2], buildTime: t[3]}, true
}

func gcrLivePieceIdentity(root string) (gcrIdent, string) {
	binz := gcrPyJoin(root, "bin", "zerg")
	if !gcrIsFile(binz) {
		return gcrIdent{}, "找不到 bin/zerg（身份行对拍缺「现跑」那一读 ⇒ 判不了）"
	}
	rc, out, errOut := gcrRunCmd(binz, []string{"version"})
	if rc != 0 {
		return gcrIdent{}, fmt.Sprintf("`bin/zerg version` 退码 %d（%s）⇒ 判不了", rc,
			gcrRuneHead(gcrPyStrip(gcrCoalesce(errOut, out)), 200))
	}
	var lines []string
	for _, l := range gcrPySplitlines(out) {
		if s := gcrPyStrip(l); s != "" {
			lines = append(lines, s)
		}
	}
	if len(lines) == 0 {
		return gcrIdent{}, "`bin/zerg version` 没有输出 ⇒ 判不了"
	}
	f, ok := gcrIdentFields(lines[0])
	if !ok {
		return gcrIdent{}, fmt.Sprintf("现跑件身份串形状不认（`bin/zerg version` 首行 = %s ⇒ 期望 `<件> <版本> <commit> <build_time>` 四段）",
			gcrPyStrRepr(lines[0]))
	}
	f.line = lines[0]
	return f, ""
}

// gcrGitNarrowDiff —— 窄集自 `<sha>` 到 HEAD 的**已提交**改动（件名 · 顺序照 git）。
func gcrGitNarrowDiff(root, sha string) ([]string, string) {
	args := []string{"-C", root, "-c", "core.quotepath=false", "diff", "--name-only", sha + "..HEAD", "--"}
	args = append(args, gcrSourceFaceRels...)
	rc, out, errOut := gcrRunCmd("git", args)
	if rc != 0 {
		return nil, fmt.Sprintf("`git diff %s..HEAD` 退码 %d（%s）⇒ 那个提交取不到，判不了",
			sha, rc, gcrRuneHead(gcrPyStrip(gcrCoalesce(errOut, out)), 200))
	}
	var moved []string
	for _, l := range gcrPySplitlines(out) {
		if s := gcrPyStrip(l); s != "" {
			moved = append(moved, s)
		}
	}
	return moved, ""
}

// gcrGitCommitExists —— `<sha>` 在本仓里是不是一枚提交（`rev-parse --verify --quiet`：
// 查不到 ⇒ rc=1 且**不打印**，与「真出错」分得开）。
func gcrGitCommitExists(root, sha string) (bool, string) {
	rc, out, errOut := gcrRunCmd("git",
		[]string{"-C", root, "rev-parse", "--verify", "--quiet", sha + "^{commit}"})
	switch rc {
	case 0:
		return gcrPyStrip(out) != "", ""
	case 1:
		return false, ""
	}
	return false, fmt.Sprintf("`git rev-parse --verify %s^{commit}` 退码 %d（%s）⇒ 页内 commit 判不了",
		sha, rc, gcrRuneHead(gcrPyStrip(gcrCoalesce(errOut, out)), 200))
}

// gcrPageCommitVerdict —— 页里那一枚 commit 还作不作数（`Q-211` **方案 B** 的收窄判据）。
//
// 三格缺一不可：
//
//	① **形态**：`+dirty` **不许** —— 脏树制品不对应任何提交，正式面不承载它；
//	② **祖先**：那枚 commit 必须是 HEAD 的祖先 ⇒ 页出自本仓这条历史（不是别处搬来 / 历史被改写）；
//	③ **窄集零改动**：自那枚 commit 到 HEAD，窄集四件**一件都没动** ⇒ 页所描述的那张面与现读
//	   源码面是**同一个面**。★ 逐字相等**不再**要求；`build_time` **不比**（每次编译都变）。
//
// 返回 `""` = 通过；否则一句人话（判红）。第二返回 `true` = **判不了**（git plumbing 出事 ⇒ rc=2）。
func gcrPageCommitVerdict(root, commit, head string) (string, bool) {
	if strings.HasSuffix(commit, "+dirty") {
		return fmt.Sprintf("页内 commit 带 `+dirty`（%s）⇒ 正式面不承载**脏树产物**（先提交，再 `--only-cli` 重编 + 重生成）",
			gcrPyStrRepr(commit)), false
	}
	sha := strings.Split(commit, "+")[0]
	exists, why := gcrGitCommitExists(root, sha)
	if why != "" {
		return why, true
	}
	if !exists {
		return fmt.Sprintf("页内 commit `%s` 在本仓历史里**查不到** ⇒ 祖先判定无从谈起（页不是从本仓这条历史生成的）", sha), false
	}
	rc, out, errOut := gcrRunCmd("git", []string{"-C", root, "merge-base", "--is-ancestor", sha, "HEAD"})
	switch rc {
	case 0:
		// 是祖先 ⇒ 继续判窄集。
	case 1:
		return fmt.Sprintf("页内 commit `%s` **不是** HEAD（`%s`）的祖先 ⇒ 页出自另一条历史（被改写/搬来的页），收窄判据不认",
			sha, head), false
	default:
		return fmt.Sprintf("`git merge-base --is-ancestor %s HEAD` 退码 %d（%s）⇒ 祖先判定判不了",
			sha, rc, gcrRuneHead(gcrPyStrip(gcrCoalesce(errOut, out)), 200)), true
	}
	moved, why := gcrGitNarrowDiff(root, sha)
	if why != "" {
		return why, true
	}
	if len(moved) > 0 {
		return fmt.Sprintf("页内 commit `%s` 之后**窄集**又动过 %d 件（%s）⇒ 命令页所描述的那张面已漂（该重编 + `--emit`）",
			sha, len(moved), strings.Join(gcrHead3(moved), " / ")), false
	}
	return "", false
}

func gcrSourceFaceState(root, commit string) (gcrFace, string) {
	sha := strings.Split(commit, "+")[0]
	fact := gcrFace{sha: sha, dirtySuffix: strings.HasSuffix(commit, "+dirty")}
	if _, err := exec.LookPath("git"); err != nil {
		return gcrFace{}, "本机无 `git` ⇒ 源码面身份取不到（判不了，不给结论）"
	}
	rc, out, errOut := gcrRunCmd("git", []string{"-C", root, "rev-parse", "--short", "HEAD"})
	if rc != 0 {
		return gcrFace{}, fmt.Sprintf("`git rev-parse --short HEAD` 退码 %d（%s）⇒ 源码面判不了（不是仓 / 无提交）",
			rc, gcrRuneHead(gcrPyStrip(gcrCoalesce(errOut, out)), 200))
	}
	fact.head = gcrPyStrip(out)
	var why string
	if fact.moved, why = gcrGitNarrowDiff(root, sha); why != "" {
		return gcrFace{}, fmt.Sprintf("%s ⇒ 源码面判不了", why)
	}
	// ── 窄集**工作树**（未提交改动 · 含未跟踪件 · GAP-20261014-64）──────────────────
	// 上面那条 `git diff <sha>..HEAD` 只比**已提交**历史 —— 工作树里未提交的窄集改动
	// （改了命令面却既没提交、也没重编 `bin/`）在旧判定里是盲区 ⇒ 命令页相对新源码
	// 已陈旧却仍报绿（陈旧假绿）。这里把 `git status --porcelain -uall` 现读算进源码面：
	// **窄集工作树不干净 ⇒ 不许报绿**（✗ 不放宽）。
	//
	//	· 只认窄集路径（gcrSourceFaceRels）；运行期落点（如
	//	  scripts/gates/judge-run-ledger.jsonl · 每跑一门就追加）不在此列 ⇒ 不误红。
	//	· `-c core.quotepath=false` 免疫件名转义（门68 口径 · 五类件名面须带免疫开关）。
	args := []string{"-C", root, "-c", "core.quotepath=false", "status", "--porcelain", "-uall", "--"}
	args = append(args, gcrSourceFaceRels...)
	rc, out, errOut = gcrRunCmd("git", args)
	if rc != 0 {
		return gcrFace{}, fmt.Sprintf("`git status --porcelain` 退码 %d（%s）⇒ 源码面工作树取不到，判不了",
			rc, gcrRuneHead(gcrPyStrip(gcrCoalesce(errOut, out)), 200))
	}
	for _, l := range gcrPySplitlines(out) {
		if gcrPyStrip(l) == "" {
			continue
		}
		// porcelain v1 每行 `XY <路径>`（状态码两列 + 一个空格）⇒ 第 4 字节起是路径。
		// ★ **不先剥行首空格**：未暂存改动的 X 列本身就是空格（` M path`），
		//   先 strip 会把状态位吃掉、路径整体错位一位。
		p := l
		if len(l) > 3 {
			p = gcrPyStrip(l[3:])
		} else {
			p = gcrPyStrip(l)
		}
		fact.workDirty = append(fact.workDirty, p)
	}
	return fact, ""
}

// ══ 渲染 ════════════════════════════════════════════════════════════

func gcrLoc(loc string) gcrLocale {
	var m map[string]gcrLocale
	if err := json.Unmarshal([]byte(gcrLJSON), &m); err != nil {
		panic(err)
	}
	return m[loc]
}

func gcrFmt(tmpl string, kv map[string]string) string {
	out := tmpl
	for k, v := range kv {
		out = strings.ReplaceAll(out, "{"+k+"}", v)
	}
	return out
}

func gcrEsc(cell string) string { return strings.ReplaceAll(cell, "|", "\\|") }

func gcrFm(loc string, d *gcrData, zhBody string) string {
	t := gcrLoc(loc)
	today := "2026-09-21"
	var head []string
	head = append(head, "---", "title: "+t.Title, "type: reference")
	if loc == "zh" {
		head = append(head, "status: authoritative")
	} else {
		head = append(head, "status: draft")
	}
	if loc == "zh" {
		head = append(head, "reviewed_at: "+today)
	}
	head = append(head, "source_of_truth:", "  - "+gcrMainRel, "  - core/cmd/zerg/exitcodes.go",
		"  - core/cmd/zerg/export.go", "  - "+gcrGenRel, "owner: Mr2109",
		"updated_at: "+today, "ai_assisted: ai")
	if loc == "en" {
		head = append(head, "source_lang: zh",
			"translation_of: "+strings.ReplaceAll(gcrZHRel, string(os.PathSeparator), "/"),
			"translation_status: human_draft")
	}
	head = append(head, "---", "")
	if loc == "en" {
		head = append(head,
			"<!-- ===== 源文 / source text（"+strings.ReplaceAll(gcrZHRel, string(os.PathSeparator), "/")+
				" · 段落级保留：双语正式面「源文注释」条 + docs/en/README.md 的 rule 5）=====",
			"     本页正文的行内代码标识符集合与本块**逐字相同**（生成器 `--check` 现算对拍）。",
			"",
			zhBody,
			"===== 源文结束 / end of source text ===== -->", "")
	}
	head = append(head, "")
	return strings.Join(head, "\n")
}

func gcrRender(loc string, d *gcrData, zhBody string) string {
	t := gcrLoc(loc)
	b := []string{gcrFm(loc, d, zhBody)}
	b = append(b, "# "+t.H1+" {: #"+t.H1+" }")
	b = append(b, "")
	leadKV := map[string]string{
		"total": strconv.Itoa(d.nOpen + d.nDanger), "open": strconv.Itoa(d.nOpen),
		"danger": strconv.Itoa(d.nDanger), "counts": d.counts,
	}
	for _, line := range t.Lead {
		b = append(b, "> "+gcrFmt(line, leadKV))
	}
	b = append(b, "> 身份 / identity："+d.ident)
	b = append(b, "")

	b = append(b, "## "+t.HCmds+" {: #"+t.HCmds+" }")
	b = append(b, "")
	for _, line := range t.CmdsNote {
		b = append(b, "- "+line)
	}
	b = append(b, "")
	b = append(b, "### "+gcrFmt(t.HOpen, map[string]string{"n": strconv.Itoa(d.nOpen)})+" {: #"+t.AOpen+" }")
	b = append(b, "")
	b = append(b, "| "+strings.Join(t.ThOpen, " | ")+" |")
	b = append(b, "|"+strings.Repeat("---|", len(t.ThOpen)))
	for _, cells := range d.open {
		cmd := strings.ReplaceAll(strings.Trim(cells[0], "`"), "zerg ", "")
		line := "| `" + strings.Trim(cells[0], "`") + "` | `" + gcrEsc(d.usage[cmd]) + "` | " + gcrEsc(cells[1]) + " |"
		line += " " + gcrEsc(cells[2]) + " | " + gcrEsc(cells[3]) + " | " + gcrEsc(cells[4]) + " |"
		b = append(b, line)
	}
	b = append(b, "")
	b = append(b, "### "+gcrFmt(t.HDanger, map[string]string{"n": strconv.Itoa(d.nDanger)})+" {: #"+t.ADanger+" }")
	b = append(b, "")
	b = append(b, "> "+gcrFmt(t.DangerNote, map[string]string{"counts": d.counts}))
	b = append(b, "")
	b = append(b, "| "+strings.Join(t.ThDanger, " | ")+" |")
	b = append(b, "|"+strings.Repeat("---|", len(t.ThDanger)))
	for _, cells := range d.danger {
		b = append(b, "| `"+strings.Trim(cells[0], "`")+"` | "+gcrEsc(cells[1])+" | "+gcrEsc(cells[2])+
			" | "+gcrEsc(cells[3])+" | "+gcrEsc(cells[4])+" | "+gcrEsc(cells[5])+" | "+gcrEsc(cells[6])+" |")
	}
	b = append(b, "")

	b = append(b, "## "+t.HExit+" {: #"+t.HExit+" }")
	b = append(b, "")
	b = append(b, "> "+t.ExitNote)
	b = append(b, "")
	b = append(b, "| "+strings.Join(t.ThExit, " | ")+" |")
	b = append(b, "|"+strings.Repeat("---|", len(t.ThExit)))
	for _, cells := range d.exit {
		parts := make([]string, len(cells))
		for i, c := range cells {
			parts[i] = gcrEsc(c)
		}
		b = append(b, "| "+strings.Join(parts, " | ")+" |")
	}
	b = append(b, "")
	for _, l := range d.exitTail {
		b = append(b, l)
		b = append(b, "")
	}
	b = append(b, "## "+t.HJson+" {: #"+t.HJson+" }")
	b = append(b, "")
	b = append(b, "> "+t.JsonNote)
	b = append(b, "")
	for _, line := range t.JsonBody {
		b = append(b, "- "+line)
	}
	b = append(b, "")
	b = append(b, "## "+t.HConfig+" {: #"+t.HConfig+" }")
	b = append(b, "")
	b = append(b, "> "+t.ConfigNote)
	b = append(b, "")
	b = append(b, "    "+d.cfg)
	b = append(b, "")
	b = append(b, "---")
	b = append(b, "")
	b = append(b, gcrFmt(t.Footer, map[string]string{"main": gcrMainRel, "gen": gcrGenRel}))
	b = append(b, "")
	return strings.Join(b, "\n")
}

func gcrBodyOf(page string) string {
	if !strings.HasPrefix(page, "---\n") {
		return page
	}
	idx := strings.Index(page[3:], "\n---\n")
	if idx < 0 {
		return page
	}
	end := 3 + idx
	return page[end+5:]
}

// ══ 注入：标记区 ════════════════════════════════════════════════════

func gcrLocaleOf(path string) string {
	for _, suf := range gcrTargetSuffix {
		if strings.HasSuffix(path, suf[0]) {
			return suf[1]
		}
	}
	return ""
}

func gcrRegionSpan(page string) (int, int, bool) {
	bi, ei := -1, -1
	for i, l := range gcrPySplitlines(page) {
		s := gcrPyStrip(l)
		if bi < 0 && strings.HasPrefix(s, gcrMarkBegin) {
			bi = i
		} else if bi >= 0 && strings.HasPrefix(s, gcrMarkEnd) {
			ei = i
			break
		}
	}
	if bi >= 0 && ei >= 0 && bi < ei {
		return bi, ei, true
	}
	return 0, 0, false
}

func gcrRegionNotes(body string) []string {
	var out []string
	for _, l := range gcrPySplitlines(body) {
		s := gcrPyStrip(l)
		if len(out) == 0 && s == "" {
			continue
		}
		if strings.HasPrefix(s, ">") {
			out = append(out, l)
			continue
		}
		break
	}
	return out
}

func gcrRenderList(loc string, d *gcrData, rel string) string {
	t := gcrLoc(loc)
	b := []string{gcrFmt(t.BlkDecl, map[string]string{"rel": rel}), "",
		"### " + gcrFmt(t.HOpen, map[string]string{"n": strconv.Itoa(d.nOpen)}), "",
		"| " + strings.Join(t.ThOpen, " | ") + " |", "|" + strings.Repeat("---|", len(t.ThOpen))}
	for _, cells := range d.open {
		cmd := strings.ReplaceAll(strings.Trim(cells[0], "`"), "zerg ", "")
		b = append(b, "| `"+strings.Trim(cells[0], "`")+"` | `"+gcrEsc(d.usage[cmd])+"` | "+gcrEsc(cells[1])+
			" | "+gcrEsc(cells[2])+" | "+gcrEsc(cells[3])+" | "+gcrEsc(cells[4])+" |")
	}
	b = append(b, "", "### "+gcrFmt(t.BlkDangerHead, map[string]string{"n": strconv.Itoa(d.nDanger)}), "",
		"> "+gcrFmt(t.BlkDangerNote, map[string]string{"counts": d.counts}), "",
		"| "+strings.Join(t.ThDanger, " | ")+" |", "|"+strings.Repeat("---|", len(t.ThDanger)))
	for _, cells := range d.danger {
		b = append(b, "| `"+strings.Trim(cells[0], "`")+"` | "+gcrEsc(cells[1])+" | "+gcrEsc(cells[2])+
			" | "+gcrEsc(cells[3])+" | "+gcrEsc(cells[4])+" | "+gcrEsc(cells[5])+" | "+gcrEsc(cells[6])+" |")
	}
	return strings.Join(b, "\n")
}

func gcrDiscoverTargets(root string) ([]string, []string) {
	d := gcrPyJoin(root, gcrPublishDocsRel)
	if !gcrIsDir(d) {
		return nil, nil
	}
	ents, err := os.ReadDir(d)
	if err != nil {
		return nil, nil
	}
	var names []string
	for _, e := range ents {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	var targets, errs []string
	for _, name := range names {
		p := gcrPyJoin(d, name)
		if !gcrIsFile(p) || gcrLocaleOf(name) == "" {
			continue
		}
		rel := gcrPyJoin(gcrPublishDocsRel, name)
		page := gcrReadOr(p)
		if _, _, ok := gcrRegionSpan(page); ok {
			targets = append(targets, rel)
		} else if strings.Contains(page, gcrMarkBegin) || strings.Contains(page, gcrMarkEnd) {
			errs = append(errs, fmt.Sprintf("%s 只有**半对**生成区标记（`%s` / `%s` 缺一）⇒ 判不了（不猜）",
				rel, gcrMarkBegin, gcrMarkEnd))
		}
	}
	return targets, errs
}

func gcrInjectTargets(root string, d *gcrData, targets []string, write bool, stdout io.Writer) []string {
	var errs []string
	for _, rel := range targets {
		p := gcrPyJoin(root, rel)
		loc := gcrLocaleOf(rel)
		if loc == "" {
			errs = append(errs, fmt.Sprintf("目标件名判不出语种（只认 `*.zh-CN.md` / `*.en.md`）：%s", rel))
			continue
		}
		if !gcrIsFile(p) {
			errs = append(errs, fmt.Sprintf("缺目标件：%s", rel))
			continue
		}
		page := gcrReadOr(p)
		bi, ei, ok := gcrRegionSpan(page)
		if !ok {
			errs = append(errs, fmt.Sprintf("目标件里找不到成对的生成区标记：%s", rel))
			continue
		}
		lines := gcrPySplitlines(page)
		body := strings.Join(lines[bi+1:ei], "\n")
		var notes []string
		replaced := false
		for _, l := range gcrRegionNotes(body) {
			hit := false
			for _, m := range gcrCountsMarkers {
				if strings.HasPrefix(gcrPyStrip(l), m) {
					hit = true
					break
				}
			}
			if hit {
				notes = append(notes, gcrRenderCountsNote(loc, d))
				replaced = true
			} else {
				notes = append(notes, l)
			}
		}
		if !replaced {
			notes = append([]string{gcrRenderCountsNote(loc, d)}, notes...)
		}
		want := strings.Join(append(notes, "", gcrRenderList(loc, d, rel)), "\n")
		if !write {
			if body != want {
				errs = append(errs, fmt.Sprintf("drift：%s 的生成区与现跑不一致（改了命令面没重跑注入？重跑 `--emit --target %s`）", rel, rel))
			}
			continue
		}
		if body == want {
			fmt.Fprintf(stdout, "  已是最新（生成区逐字节无差）：%s\n", rel)
			continue
		}
		out := strings.Join(append(append(append([]string{}, lines[:bi+1]...), gcrPySplitlines(want)...), lines[ei:]...), "\n")
		_ = os.WriteFile(p, []byte(out), 0o644)
		fmt.Fprintf(stdout, "  已注入 %s 的生成区（%d → %d 字节）\n", rel, len([]byte(page)), len([]byte(out)))
	}
	return errs
}

// ══ build ═══════════════════════════════════════════════════════════

func gcrBuild(root string) (*gcrPages, string) {
	d, err := gcrLiveData(root)
	if err != "" {
		return nil, err
	}
	live, err := gcrLivePieceIdentity(root)
	if err != "" {
		return nil, err
	}
	face, err := gcrSourceFaceState(root, live.commit)
	if err != "" {
		return nil, err
	}
	d.liveIdent = live
	d.sourceFace = face
	zh := gcrRender("zh", d, "")
	en := gcrRender("en", d, gcrBodyOf(zh))
	return &gcrPages{zh: zh, en: en, data: d}, ""
}

// ══ check ═══════════════════════════════════════════════════════════

func gcrCodeSpans(text string) map[string]bool {
	stripped := gcrCommentRE.ReplaceAllString(text, "")
	out := map[string]bool{}
	for _, m := range gcrCodeRE.FindAllStringSubmatch(stripped, -1) {
		out[m[1]] = true
	}
	return out
}

func gcrSetDiff(a, b map[string]bool) []string {
	var out []string
	for k := range a {
		if !b[k] {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

func gcrSetEq(a, b map[string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}

// gcrCheck —— 判据面。第二返回 `true` = 有「判不了」的格子（调用方须退 **2**，✗ 混进红）。
func gcrCheck(root string, pages *gcrPages, targets []string) ([]string, bool) {
	var errs []string
	blk := false
	for _, lr := range [][2]string{{"zh", gcrZHRel}, {"en", gcrENRel}} {
		loc, rel := lr[0], lr[1]
		p := gcrPyJoin(root, rel)
		if !gcrIsFile(p) {
			errs = append(errs, fmt.Sprintf("缺产物：%s（本版按 --emit 生成过吗）", rel))
			continue
		}
		if gcrReadOr(p) != pages.forLoc(loc) {
			errs = append(errs, fmt.Sprintf("drift：%s 与现跑不一致（改了命令面没重生成？重跑 --emit）", rel))
		}
	}
	d := pages.data
	live := d.liveIdent
	face := d.sourceFace
	for _, lr := range [][2]string{{"zh", gcrZHRel}, {"en", gcrENRel}} {
		rel := lr[1]
		p := gcrPyJoin(root, rel)
		if !gcrIsFile(p) {
			continue
		}
		found := gcrPageIdentityLines(gcrReadOr(p))
		if len(found) == 0 {
			errs = append(errs, fmt.Sprintf("身份行对拍（`Q-211`）：%s 里一行身份行都没有（`%s` 那一行）—— 正式面页必须带现跑件身份", rel, gcrIdentPrefix))
			continue
		}
		for _, v := range found {
			f, ok := gcrIdentFields(v)
			if !ok {
				errs = append(errs, fmt.Sprintf("身份行对拍（`Q-211`）：%s 的身份行形状不认（%s ⇒ 期望 `<件> <版本> <commit> <build_time>` 四段）", rel, gcrPyStrRepr(v)))
				continue
			}
			// ★ `Q-211` 方案 B：判据 = 页内 commit **是 HEAD 祖先** ∧ **窄集自该提交零改动**
			//   （✗ 不再逐字相等；`build_time` ✗ 不比 —— 每次编译都变，逐字比只会逼着重生成）。
			if !gcrContains(d.pageCommits, f.commit) {
				d.pageCommits = append(d.pageCommits, f.commit)
			}
			msg, unjudged := gcrPageCommitVerdict(root, f.commit, face.head)
			if unjudged {
				blk = true
			}
			if msg != "" {
				errs = append(errs, fmt.Sprintf("身份行对拍（`Q-211` 方案 B）：%s 的%s", rel, msg))
			}
		}
	}
	// 导出物抬头 ⟷ `bin/zerg version`：同一次现跑的两条读法**自己**不许打架。
	// ★ `commit` 仍逐字比（同一枚件两处报不同 commit = 判据面自相矛盾）；`build_time` ✗ 不比。
	if exp, ok := gcrIdentFields(d.ident); ok {
		if exp.commit != live.commit {
			errs = append(errs, fmt.Sprintf("身份行对拍（`Q-211`）：导出物抬头与 `bin/zerg version` **两读不一致**（导出 %s · 现跑 %s）—— 判据面的两条读法自己打架，不猜", gcrPyStrRepr(d.ident), gcrPyStrRepr(live.line)))
		}
	}
	if face.dirtySuffix {
		errs = append(errs, fmt.Sprintf("身份行对拍（`Q-211`）：现跑件身份第三格带 `+dirty`（%s）⇒ 该件是**脏树产物**、不对应任何提交 —— 正式面不承载它（先提交，再 `--only-cli` 重编 + 重生成）", gcrPyStrRepr(live.commit)))
	}
	if len(face.moved) > 0 {
		errs = append(errs, fmt.Sprintf("身份行对拍（`Q-211` 方案 B）：现跑件那个提交（`%s`）之后**窄集**又动过 %d 件（%s）⇒ `bin/` 旧了（页与现跑件**同源过期** —— 这一格就是它的牙：重编 + 重生成）",
			face.sha, len(face.moved), strings.Join(gcrHead3(face.moved), " / ")))
	}
	// GAP-20261014-64：`git diff <sha>..HEAD` 只看得见**已提交**历史 ⇒ 工作树里未提交的
	// 窄集改动（旧判定盲区）在此判红 —— 命令页顶多与**现跑件**同源，证明不了与**源码面**一致。
	if len(face.workDirty) > 0 {
		errs = append(errs, fmt.Sprintf("身份行对拍（`Q-211` 方案 B）：现跑件那个提交（`%s`）之后**窄集工作树**还有 %d 处**未提交**改动（%s）⇒ 命令页只能证明与**现跑件**同源，不能证明与**源码面**一致（源码已漂 · 须先提交/或确认无源码漂移，再 `--only-cli` 重编 + 重生成）",
			face.sha, len(face.workDirty), strings.Join(gcrHead3(face.workDirty), " / ")))
	}
	pageSet := map[string]bool{}
	for _, n := range d.namesPage {
		pageSet[n] = true
	}
	treeSet := map[string]bool{}
	for _, n := range d.namesTree {
		treeSet[n] = true
	}
	if !gcrSetEq(pageSet, treeSet) {
		var miss, extra []string
		for k := range treeSet {
			if !pageSet[k] {
				miss = append(miss, k)
			}
		}
		for k := range pageSet {
			if !treeSet[k] {
				extra = append(extra, k)
			}
		}
		sort.Strings(miss)
		sort.Strings(extra)
		errs = append(errs, fmt.Sprintf("命令名对拍：页里缺 %d 条（%s…）· 页里多 %d 条（%s…）",
			len(miss), strings.Join(gcrHead5(miss), " / "), len(extra), strings.Join(gcrHead5(extra), " / ")))
	}
	if len(pageSet) != d.nOpen+d.nDanger {
		errs = append(errs, fmt.Sprintf("计数对拍：页里 %d 条 ≠ 导出物抬头 %d + %d", len(pageSet), d.nOpen, d.nDanger))
	}
	for _, lr := range [][2]string{{"zh", gcrZHRel}, {"en", gcrENRel}} {
		loc, rel := lr[0], lr[1]
		if strings.Contains(pages.forLoc(loc), "§") {
			errs = append(errs, fmt.Sprintf("%s 里残留设计稿引用（§）—— 公开面不承载过程档案（fail-closed）", rel))
		}
	}
	sz := gcrCodeSpans(pages.zh)
	se := gcrCodeSpans(pages.en)
	if !gcrSetEq(sz, se) {
		zdo := strings.Join(gcrHead5(gcrSetDiff(sz, se)), " / ")
		edo := strings.Join(gcrHead5(gcrSetDiff(se, sz)), " / ")
		if zdo == "" {
			zdo = "无"
		}
		if edo == "" {
			edo = "无"
		}
		errs = append(errs, fmt.Sprintf("双语标识符集合不一致：zh 独有 %s · en 独有 %s", zdo, edo))
	}
	if len(targets) > 0 {
		errs = append(errs, gcrInjectTargets(root, d, targets, false, io.Discard)...)
	}
	return errs, blk
}

func (p *gcrPages) forLoc(loc string) string {
	if loc == "zh" {
		return p.zh
	}
	return p.en
}

func gcrHead3(s []string) []string {
	if len(s) > 3 {
		return s[:3]
	}
	return s
}

func gcrHead5(s []string) []string {
	if len(s) > 5 {
		return s[:5]
	}
	return s
}

// ══ argparse（`--emit|--check` 互斥必填 + `--target` 追加 + 唯一前缀缩写）══

type gcrOpts struct {
	emit     bool
	check    bool
	target   []string
	selfTest bool // `--self-test`：合成夹具成对负控（不碰真目标 · 不读 `bin/` 制品）
}

func gcrArgError(stderr io.Writer, msg string) {
	fmt.Fprintf(stderr, "%s\n", gcrUsageLine)
	fmt.Fprintf(stderr, "%s: error: %s\n", gcrProg, msg)
}

func gcrParseArgs(stdout, stderr io.Writer, argv []string) (gcrOpts, int, bool) {
	var o gcrOpts
	seenEmit, seenCheck := false, false
	var extras []string
	known := []string{"--emit", "--check", "--self-test", "--target", "--help"}
	i := 0
	for i < len(argv) {
		tok := argv[i]
		i++
		if tok == "--" {
			extras = append(extras, argv[i:]...)
			break
		}
		if strings.HasPrefix(tok, "--") && len(tok) > 2 {
			name := tok[2:]
			val := ""
			hasVal := false
			if eq := strings.IndexByte(name, '='); eq >= 0 {
				val = name[eq+1:]
				name = name[:eq]
				hasVal = true
			}
			var match string
			cnt := 0
			for _, cand := range known {
				if strings.HasPrefix(cand, "--"+name) {
					match = cand
					cnt++
				}
			}
			if cnt == 0 {
				extras = append(extras, tok)
				continue
			}
			if cnt > 1 {
				gcrArgError(stderr, fmt.Sprintf("ambiguous option: --%s", name))
				return o, gcrRCblocked, true
			}
			switch match {
			case "--help":
				if hasVal {
					gcrArgError(stderr, fmt.Sprintf("argument --help: ignored explicit argument '%s'", val))
					return o, gcrRCblocked, true
				}
				fmt.Fprint(stdout, gcrHelpText)
				return o, gcrRCok, true
			case "--emit":
				if hasVal {
					gcrArgError(stderr, fmt.Sprintf("argument --emit: ignored explicit argument '%s'", val))
					return o, gcrRCblocked, true
				}
				if seenCheck {
					gcrArgError(stderr, "argument --emit: not allowed with argument --check")
					return o, gcrRCblocked, true
				}
				seenEmit, o.emit = true, true
			case "--check":
				if hasVal {
					gcrArgError(stderr, fmt.Sprintf("argument --check: ignored explicit argument '%s'", val))
					return o, gcrRCblocked, true
				}
				if seenEmit {
					gcrArgError(stderr, "argument --check: not allowed with argument --emit")
					return o, gcrRCblocked, true
				}
				seenCheck, o.check = true, true
			case "--self-test":
				// ③ 件级负控入口：合成夹具成对负控（照 `check_record_not_evidence.go` 的
				// `crneParseArgs` 同形 —— 门自带 `--self-test` 子档）。
				if hasVal {
					gcrArgError(stderr, fmt.Sprintf("argument --self-test: ignored explicit argument '%s'", val))
					return o, gcrRCblocked, true
				}
				o.selfTest = true
			case "--target":
				if !hasVal {
					if i >= len(argv) {
						gcrArgError(stderr, "argument --target: expected one argument")
						return o, gcrRCblocked, true
					}
					val = argv[i]
					i++
				}
				o.target = append(o.target, val)
			}
			continue
		}
		if strings.HasPrefix(tok, "-") && tok != "-" {
			if tok == "-h" {
				fmt.Fprint(stdout, gcrHelpText)
				return o, gcrRCok, true
			}
			extras = append(extras, tok)
			continue
		}
		extras = append(extras, tok)
	}
	if !seenEmit && !seenCheck && !o.selfTest {
		gcrArgError(stderr, "one of the arguments --emit --check is required")
		return o, gcrRCblocked, true
	}
	if len(extras) > 0 {
		gcrArgError(stderr, "unrecognized arguments: "+strings.Join(extras, " "))
		return o, gcrRCblocked, true
	}
	return o, gcrRCok, false
}

// ══ 入口 ════════════════════════════════════════════════════════════

func gcrRun(stdout, stderr io.Writer, argv []string) int {
	o, rc, done := gcrParseArgs(stdout, stderr, argv)
	if done {
		return rc
	}
	if o.selfTest {
		return gcrSelfTest(stdout, stderr)
	}
	wd, _ := os.Getwd()
	root := gcrFindRoot(gcrPyAbspath(wd, wd))
	if root == "" {
		fmt.Fprintf(stdout, "⛔ 不给结论：找不到仓根（上溯找不到 %s）\n", gcrMainRel)
		return gcrRCblocked
	}
	derived, derrs := gcrDiscoverTargets(root)
	if len(derrs) > 0 {
		fmt.Fprint(stdout, "⛔ 不给结论（rc=2）：生成区目标**现读派生**时撞到认不出的形状：\n")
		for _, e := range derrs {
			fmt.Fprintf(stdout, "  ✗ %s\n", e)
		}
		return gcrRCblocked
	}
	var extra []string
	for _, t := range o.target {
		if !gcrContains(derived, t) {
			extra = append(extra, t)
		}
	}
	targets := append(append([]string{}, derived...), extra...)
	extraSuffix := ""
	if len(extra) > 0 {
		extraSuffix = fmt.Sprintf(" · `--target` 追加 %d 件", len(extra))
	}
	targetsJoined := "（现读派生 0 件 · 也没传 --target）"
	if len(targets) > 0 {
		targetsJoined = strings.Join(targets, " · ")
	}
	fmt.Fprintf(stdout, "  生成区目标（现读派生 `%s` 下带成对标记的 `*.zh-CN.md` / `*.en.md` **%d** 件%s）：%s\n",
		gcrPublishDocsRel, len(derived), extraSuffix, targetsJoined)

	pages, err := gcrBuild(root)
	if err != "" {
		fmt.Fprintf(stdout, "⛔ 不给结论（rc=2）：%s\n", err)
		return gcrRCblocked
	}
	d := pages.data
	if o.emit {
		for _, lr := range [][2]string{{"zh", gcrZHRel}, {"en", gcrENRel}} {
			loc, rel := lr[0], lr[1]
			p := gcrPyJoin(root, rel)
			_ = os.MkdirAll(gcrPyDirname(p), 0o777)
			_ = os.WriteFile(p, []byte(pages.forLoc(loc)), 0o644)
			fmt.Fprintf(stdout, "  已生成 %s（%d 字节）\n", rel, len([]byte(pages.forLoc(loc))))
		}
		fmt.Fprintf(stdout, "  命令树 %d 条 = 已开放 %d + 危险动作 %d · 退码 %d 格 · 用法串 %d 条（其中 %d 条与 `zerg help` 的用法块逐字一致）\n",
			len(d.namesTree), d.nOpen, d.nDanger, len(d.exit), len(d.usage), d.nHelpAgree)
		errs := gcrInjectTargets(root, d, targets, true, stdout)
		cerrs, blk := gcrCheck(root, pages, targets)
		errs = append(errs, cerrs...)
		if len(errs) > 0 {
			fmt.Fprint(stdout, "⇒ 生成后立刻复判就有问题（rc=1）：\n")
			for _, e := range errs {
				fmt.Fprintf(stdout, "  ✗ %s\n", e)
			}
			if blk {
				return gcrRCblocked
			}
			return gcrRCfail
		}
		fmt.Fprint(stdout, "⇒ 已生成并复判通过（drift 0 · 命令名对拍 0 差 · 窄集身份对拍 0 差）rc=0\n")
		return gcrRCok
	}
	errs, blk := gcrCheck(root, pages, targets)
	fmt.Fprintf(stdout, "参考-命令行 · 仓根 %s\n", root)
	fmt.Fprintf(stdout, "  现跑：命令树 %d 条（页 %d 条）· 退码 %d 格 · 用法串 %d 条（%d 条与 `zerg help` 逐字一致）\n",
		len(d.namesTree), len(d.namesPage), len(d.exit), len(d.usage), d.nHelpAgree)
	same := "有差"
	ps := map[string]bool{}
	for _, n := range d.namesPage {
		ps[n] = true
	}
	ts := map[string]bool{}
	for _, n := range d.namesTree {
		ts[n] = true
	}
	if gcrSetEq(ps, ts) {
		same = "0 差"
	}
	fmt.Fprintf(stdout, "  命令名对拍（页 ⟷ %s）：%s\n", gcrMainRel, same)
	movedStr := "无改动"
	if len(d.sourceFace.moved) > 0 {
		movedStr = fmt.Sprintf("%d 件改动（%s…）", len(d.sourceFace.moved),
			strings.Join(gcrHead3(d.sourceFace.moved), " / "))
	}
	workStr := "无"
	if len(d.sourceFace.workDirty) > 0 {
		workStr = fmt.Sprintf("%d 件（%s…）", len(d.sourceFace.workDirty),
			strings.Join(gcrHead3(d.sourceFace.workDirty), " / "))
	}
	dirty := "无"
	if d.sourceFace.dirtySuffix {
		dirty = "有"
	}
	fmt.Fprintf(stdout, "  身份行对拍（`Q-211` 方案 B）：现跑件 %s · 窄集自 `%s` 起 已提交 %s · 工作树未提交 %s · 脏树后缀 %s\n",
		d.liveIdent.line, d.sourceFace.sha, movedStr, workStr, dirty)
	pageStr := "（页里一行身份行都没有）"
	if len(d.pageCommits) > 0 {
		pageStr = strings.Join(d.pageCommits, " · ")
	}
	fmt.Fprintf(stdout, "    页内 commit（判据 = HEAD 祖先 ∧ 窄集零改动 ∧ 无 `+dirty`；`build_time` 不比）：%s · 窄集 %d 件\n",
		pageStr, len(gcrSourceFaceRels))
	for _, lr := range [][2]string{{"zh", gcrZHRel}, {"en", gcrENRel}} {
		loc, rel := lr[0], lr[1]
		p := gcrPyJoin(root, rel)
		sameFile := gcrIsFile(p) && gcrReadOr(p) == pages.forLoc(loc)
		drift := "无"
		if !sameFile {
			drift = "**有**"
		}
		fmt.Fprintf(stdout, "  %s drift=%s\n", gcrPadRight(rel, 28), drift)
	}
	for _, rel := range targets {
		drift := "无"
		for _, e := range errs {
			if strings.Contains(e, rel) {
				drift = "**有**"
				break
			}
		}
		fmt.Fprintf(stdout, "  %s drift=%s\n", gcrPadRight(rel, 28), drift)
	}
	if len(errs) > 0 {
		if blk {
			fmt.Fprint(stdout, "⛔ 不给结论（rc=2）：下面有判不了的格子（✗ 不混进红 · 更不当绿）\n")
		} else {
			fmt.Fprint(stdout, "⇒ 判红（rc=1）：逐条看上面\n")
		}
		for _, e := range errs {
			fmt.Fprintf(stdout, "  ✗ %s\n", e)
		}
		if blk {
			return gcrRCblocked
		}
		return gcrRCfail
	}
	fmt.Fprint(stdout, "⇒ 全过（rc=0）：产物与现跑逐字节一致 · 命令名与命令树 0 差 · 页内 commit 在窄集上仍等于现读源码面\n")
	return gcrRCok
}

// genCliReferenceGateMain —— 本件入口（inproc 口径：argv **不含程序名**）。
func genCliReferenceGateMain(stdout, stderr io.Writer, argv []string) int {
	return gcrRun(stdout, stderr, argv)
}

// gcrSelfTest —— `gen-cli-reference.py --self-test`：合成夹具**成对负控**（不碰真目标 · 零写盘）。
//
// 覆盖本件几条 py 同构原语（把实现哑化 / 换标准库近似就会红）：
//
//	① 正控 = 普通输入判等；② 负控 = py 与标准库**判准差异**处（`gcrPyStrip` 的 U+00A0 / `\x1c`、
//	   `gcrPyDirname` 尾斜杠、`gcrPyJoin` 后段绝对、`gcrPyNormpath` 双前斜杠）⇒ 期望值即 py 真值。
//
// 任一格与期望不符 ⇒ rc=1（本自检**自己就能红** —— 负控不是装饰）。
func gcrSelfTest(stdout, stderr io.Writer) int {
	bad := 0
	chk := func(cond bool, label string) {
		if cond {
			fmt.Fprintf(stdout, "   ✓ %s\n", label)
			return
		}
		fmt.Fprintf(stderr, "   ✗ %s\n", label)
		bad++
	}
	// ① 正控
	chk(gcrPyStrip(" x ") == "x", "正控：gcrPyStrip 剥普通空格 ⇒ x")
	chk(gcrPyDirname("/a/b/") == "/a/b", "正控：gcrPyDirname 尾斜杠 ⇒ /a/b")
	// ② 负控（期望 = py 真值 · 换 stdlib 近似即红）
	chk(gcrPyStrip("\u00a0x\u00a0") == "x", "负控：gcrPyStrip 剥 U+00A0（TrimSpace 会漏）")
	chk(gcrPyJoin("repo", "/abs") == "/abs", "负控：gcrPyJoin 后段绝对 ⇒ 丢前段（filepath.Join 会拼）")
	chk(gcrPyNormpath("//a/b/../c") == "//a/c", "负控：gcrPyNormpath 双前斜杠原样保留")
	if bad > 0 {
		fmt.Fprintf(stderr, "%s: `--self-test` 判红：%d 格不过\n", gcrProg, bad)
		return gcrRCfail
	}
	fmt.Fprintln(stdout, "gen-cli-reference.py --self-test：成对负控全过（正控 2 + 负控 3 · 合成夹具 · 未碰真目标）")
	return gcrRCok
}
