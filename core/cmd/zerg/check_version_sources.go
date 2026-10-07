// check_version_sources.go —— 门禁件 `scripts/gates/check-version-sources.py` 的 **Go 真搬**（M2 · 第四枚 A 类纯静态件）。
//
// 形态判定（现读）：本件**纯静态只读** —— 只读仓内四件（VERSION / version.go / ui 与 wall 的 Cargo.toml），
// 外加只报告档扫 `docs/**` 与仓根 md；**不起子进程 · 不联网 · 不计时 · 不取运行期读数 · 默认档不写盘**。
// 自检档在仓库外临时目录里造合成夹具，跑完回收。⇒ 属 M2-0「无输入物料、纯静态检查」档，适格。
//
// 落件边界（照实）：**只落码、不接线** ——
//
//	· 不在 core/cmd/zerg/main.go 注册命令（不进命令树）；不改任何 add_step 步骤表；
//	· 不删 scripts/gates/check-version-sources.py 原件（退场另批逐件点名）；
//	· 入口 checkVersionSourcesGateMain 目前无调用点 —— M2 的刻意状态。
//
// 入口签名：func checkVersionSourcesGateMain(stdout, stderr io.Writer, argv []string) int
//
//	argv 与 py 的 sys.argv[1:] 同形；退码 0/1/2 与 py 三码逐字对齐；stdout/stderr 与 py 逐字节对齐（见下）。
//
// ── 出厂声明 (a) 默认根定位差异 ───────────────────────────────────────────
//
//	py `repo_root()` 无参时从 `__file__`（脚本目录 scripts/gates）上溯 ≤6 层找同时含 core/ 与 ui/ 的目录；
//	Go 入口无 `__file__` 语境（未来接线后是 CLI 子命令）⇒ 本件默认从**当前工作目录**上溯 ≤6 层、找不到回落 cwd。
//	落在同一真仓且 cwd=仓根时两者读数逐字相同；「脚本位置 ≠ cwd」时路径推导面不同 ⇒ 该面不属逐字对齐承诺，
//	对拍一律显式传根。
//
// ── 出厂声明 (b) 自指检查（真判 + 前瞻）─────────────────────────────────
//
//	本件落在三道他门扫描面内（一旦被 git 跟踪即进面）：check-hardcoded-private-paths（扫 core/）、
//	check-error-kinds（扫 core/cmd/zerg/*.go 的用法面字面量）、check-wall-layer（扫 core/cmd/zerg/*.go 的层标记）。
//	逐条消解：① 本件正文**零绝对私有路径字面量**（本机家目录 / 卷根一类一律以描述式指代，不作原串落地）；
//	② 本件**不写**错误 kind 的用法原串、也不写层标记原串（一律描述式）⇒ 两道他门的命中面贡献 0。
//	③ 本件自身不扫 core/ ⇒ 无自扫描面。收口批一提交即真进扫描面，故按「一旦被跟踪」逐条消解并报遗留悬空字面计数。
//
// ── Join / 词法面逐处交代（✗ 不用 filepath.Join/Abs/Rel/EvalSymlinks）──────
//
//	逐处现读 py 原件的路径拼接：P_GO/P_UI/P_WALL（posixpath.join 三段）· docs_snapshot_root 的
//	`os.path.join(root, ".."|"docs/项目文档")` ＋ `os.path.abspath`（← `..` 段在此触发：第二候选带前导 `..`
//	⇒ 必须自写 join/abspath 词法面）· collect_corpus 的 `os.path.relpath(p, root)` 与仓根取件 ·
//	need/scan 的 `os.path.join(root, rel)` · main 的 `os.path.abspath(os.path.join(root, rel))`。
//	⇒ 本件带自写 vsPyJoin / vsPyNormpath / vsPyAbspath / vsPyRelpath / vsPyDirname（CPython posixpath 语义，
//	含 **normpath 保留恰两个前导斜杠** 的语义：`//a/b` 不折成 `/a/b`，本件已钉单测）。
//
// ── 遍历 / 排序点逐处交代 ──────────────────────────────────────────────
//
//	T1 `derive_stale_set` 的 `sorted(os.listdir(d))` ⇒ 用 sort.Strings 复现（码点序＝UTF-8 字节序）。
//	T2 `collect_corpus` 的 `os.walk(docs)` ⇒ **必须同时保两件事**：(a) `f.ReadDir(-1)` 保 readdir 原序；
//	   (b) **先产出当前层全部文件、再按 dirs 原序下钻**（✗ 边遍历边下钻 ⇒ 子目录夹在 readdir 原序中间时「早钻」）。
//	   本件产出序对 `excluded_dirs` 字典的**插入序**可见 ⇒ 逐字对齐该序。
//	T3 `collect_corpus` 结尾 `sorted(set(files))` ⇒ 这是**输出行序（清单序）的最终决定者**：files 的遍历序对它不可见。
//	T4 仓根 md：`sorted(os.listdir(root))` ⇒ sort.Strings。
//	T5 只报告明细：`sorted(byfile, key=lambda p: (-bare, -len, p))` ⇒ 计数降序 + 同数保 path 码点序。
//	T6 `scan_live_docs` 的 hits 顺序 = files（已排序）→ 行号升序 → stale_set 顺序 → 行内匹配序。
//	T7 `derive_stale_set` 的 `sorted(..., key=version_key)` ⇒ 用 sort.SliceStable + version_key 复现。
//	★ 无 `Counter.most_common`；零 Go map 迭代进输出（dict 序一律用切片保）。
//
// ── 正则面逐处交代 ────────────────────────────────────────────────────
//
//	py 的 `\s` 是 Unicode 空白（含 \x1c-\x1f / \x85 / \xa0 / \u1680 / \u2000-\u200a / \u2028-\u2029 /
//	\u202f / \u205f / \u3000）⇒ 本件把 `\s` 展开为显式 Unicode 空白类（vsWSClass）。
//	· parse_go_const：`\s*const\s+<名>\s*=\s*"([^"]+)"\s*$` ⇒ 逐行 RE2 对译，`\s` 已展开。
//	· parse_cargo_pkg_version：`version\s*=\s*"([^"]+)"` ⇒ 同上（re.match 起点锚定、非终点锚定）。
//	· NUM_RE `(?<![\d.])(\d+\.\d+\.\d+)(?![\d.])`：含**环视断言** ⇒ 手写扫描器 vsNumFindAll（`\d` 按
//	  Unicode 数字 \p{Nd} ＝ unicode.IsDigit；前后界逐码点现判）。fullmatch 亦然。
//	· 只报告档 `re.escape(token) + r"(?![\d.])"` ⇒ 自写字面量扫描器 vsLiteralFindAll（后界非数字/点）。
//
// ── py 字符串原语逐处交代 ─────────────────────────────────────────────
//
//	· `str.splitlines()` ⇒ vsSplitLines 按 py 的行边界集（\n \v \f \x1c \x1d \x1e \x85 \u2028 \u2029）切。
//	· `str.strip()` 是 py Unicode 语义（含 \x1c-\x1f / \x85 / \xa0）⇒ 自写 vsPyStrip，✗ 非 strings.TrimSpace。
//	· `s[:200]` 按**码点**切 ⇒ hits 的 text 走 []rune 截断。
//	· `open()` 文本模式 = universal newlines ⇒ vsUniversalNewlines 折 \r\n 与裸 \r。
//
// ── 符号链接判定面 ────────────────────────────────────────────────────
//
//	py 的 is_dir/isfile/exists **跟随**符号链接 ⇒ 用 os.Stat（✗ 非 DirEntry.IsDir）；os.walk(followlinks=False)
//	⇒ 链接目录录入 dirnames 但**不下钻** ⇒ 本件 vsWalkPy 对「指向目录的链接」按 Stat 归 dir、按 Lstat 判不下钻。
//
// ── 未覆盖面（✗ 不声称逐字对齐）────────────────────────────────────────
//
//	① argparse 帮助/用法正文：py 用 argparse（add_help=False + 手加 -h/--help）⇒ 未知旗标时 argparse 先往
//	   stderr 打 usage 正文再抛 SystemExit，py 捕获后另打一行。本件同打该行、rc 同 2，但**未复刻 argparse 的
//	   usage 正文**（其正文含程序名与选项表，逐字复刻收益低且易与 py 版本漂移）。
//	② 读失败正文：py `read_text` 把 OSError 的 `[Errno N] strerror: 'path'` 原样嵌进「读不到（…）」/「不可解析」。
//	   本件复刻常见 errno（2/13/20/21/40）的 py 文案；**非 UTF-8 的 UnicodeDecodeError 正文**按近似复刻
//	   （invalid start/continuation/unexpected end），与 CPython 逐字一致的把握较低 ⇒ 该档只对齐 rc，正文列未覆盖。
//	③ `version_key` 里 py `int()` 认 Unicode 数字/下划线/前后空白 ⇒ 本件用 strconv.Atoi（ASCII）⇒ 极端串不同
//	   （真仓版本号恒 ASCII ⇒ 不触发）。
//	④ `derive_stale_set` 里 `set(known)` 的迭代序在「两个不同名、version_key 相同」时会影响 sort 稳定性 ⇒ 本件
//	   用「保首遇序去重」，与 py 的 set 序在正常数据（名唯一、key 各异）下等价。
//	⑤ py `print_self_test` 在自检**自身抛异常**时提前返回、**不回收** mkdtemp 目录（真口子 C/A 类，交回入账）；
//	   本件一律 defer 回收（行为差仅在异常路径，输出不可见）。
//
// ★ 真口子（交回入账，✗ 不改 py）：A 类 —— `print_self_test` 异常路径 tempdir 不回收（见未覆盖⑤）；
//
//	B 类 —— `scan_live_docs` 对 getsize/read 失败**静默少判**（`skipped` 只登记、分母照长，被跳过件既不进「可判」
//	也不进「判不了」⇒ 缺件/读不了的件 ≠ 绿）；C 类 —— `__main__` 裸 `sys.exit(main(...))`、main 内未捕异常 ⇒ 回溯
//	rc=1 而非「不给结论 rc=2」（本件对读失败一律 fail-closed 到 rc=2 并写未覆盖②）；D 类 —— 未见死旗标/死正则/
//	零捕获组；`--stale` / `--stale-all` / `--strict-docs` / `--list-rules` / `--json` / `--self-test` 均被读取且有效。
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"unicode"
	"unicode/utf8"
)

// ─────────────────────────── 常量 ───────────────────────────

const vsTool = "check-version-sources.py"
const vsToolVersion = "v1.0.0"

const (
	vsRCOK      = 0
	vsRCFail    = 1
	vsRCBlocked = 2
)

var vsRCName = map[int]string{0: "OK（全绿）", 1: "FAIL（有失败项）", 2: "BLOCKED（不给结论）"}

var vsPVersionFile = "VERSION"
var vsPGo = vsPyJoin("core", "internal", "version", "version.go")
var vsPUi = vsPyJoin("ui", "Cargo.toml")
var vsPWall = vsPyJoin("wall", "Cargo.toml")

var vsVersionPointers = []string{"core/internal/version/version.go", "ui/Cargo.toml"}

var vsDocsRelCandidates = []string{"docs/项目文档", vsPyJoin("..", "Zerg-内部文档", "项目文档")}

// ── R6 · 运行时默认根（目录）点名覆盖（py 同名段的逐字镜像）─────────────────────
// 本门**运行时**要解析的默认根（目录）。名册 vsDefaultRootRoster 逐个点名「默认根常量/位点」；
// 被点名的目录必须**在盘**，否则必须在 vsRelocatedRoots 里**登记迁出**（含理由 + 落点），落点也须在盘。
// ✗ 禁静默兜底：既不在盘、又未登记 ⇒ rc=1。名册与 vsRuntimeDefaultRootConstants()（本门真源常量）
//
//	**双向闭合**：谁多谁少都红。
const vsCorpusPrefix = "docs/" // = py CORPUS_DOCS_PREFIX（语料域根前缀）

var vsRepoRootMarkers = []string{"core", "ui"} // repo_root() 的标记目录

type vsDefRootEnt struct {
	id, role  string
	named     []string
	fallbacks []string
}

var vsDefaultRootRoster = []vsDefRootEnt{
	{"repo-root", "仓根探测（repo_root 同时要求这两个标记目录在盘）", []string{"core", "ui"}, nil},
	{"corpus", "活文档语料域根（CORPUS_DOCS_PREFIX）", []string{"docs"}, nil},
	{"snapshot", "快照取源根（DOCS_REL_CANDIDATES：仓内优先、仓外兜底）",
		[]string{vsDocsRelCandidates[0]}, vsDocsRelCandidates[1:]},
}

type vsRelocEnt struct{ reason, landed string }

// 迁出登记（名册式）：{被点名的默认根目录: (迁出理由, 落点——必须也在盘)}
var vsRelocatedRoots = map[string]vsRelocEnt{
	"docs/项目文档": {"2026-09-26「批3」项目文档整树分家至仓外同级目录（仓内零命中）", vsDocsRelCandidates[1]},
}

type vsDefEntry struct {
	root, dir, state string
	landed           string
	hasLanded        bool
}

// vsRuntimeDefaultRootConstants —— 本门**真源常量**里的运行时默认根目录（名册的双向闭合对象）。
func vsRuntimeDefaultRootConstants() []string {
	var out []string
	add := func(d string) {
		for _, x := range out {
			if x == d {
				return
			}
		}
		out = append(out, d)
	}
	for _, d := range vsRepoRootMarkers {
		add(d)
	}
	add(strings.TrimSuffix(vsCorpusPrefix, "/"))
	for _, d := range vsDocsRelCandidates {
		add(d)
	}
	return out
}

// vsCheckDefaultRoots —— R6：运行时默认根（目录）点名覆盖。返回 (problems, blockers, entries)。
func vsCheckDefaultRoots(root string, roster []vsDefRootEnt, relocated map[string]vsRelocEnt) ([]string, []string, []vsDefEntry) {
	if roster == nil {
		roster = vsDefaultRootRoster
	}
	if relocated == nil {
		relocated = vsRelocatedRoots
	}
	var problems, blockers []string
	var entries []vsDefEntry
	var namedAll []string
	for _, e := range roster {
		for _, d := range append(append([]string{}, e.named...), e.fallbacks...) {
			found := false
			for _, x := range namedAll {
				if x == d {
					found = true
				}
			}
			if !found {
				namedAll = append(namedAll, d)
			}
		}
	}
	if len(roster) == 0 || len(namedAll) == 0 {
		blockers = append(blockers, "空转：运行时默认根名册为空（判据不可判 ⇒ 不给结论）")
		return problems, blockers, entries
	}
	consts := vsRuntimeDefaultRootConstants()
	for _, d := range consts {
		if !vsContains(namedAll, d) {
			problems = append(problems, fmt.Sprintf("FAIL 名册漏点运行时默认根 %s（代码点名了它、名册里没有 ⇒ 名册式点名不成立）", d))
		}
	}
	for _, d := range namedAll {
		if !vsContains(consts, d) {
			problems = append(problems, fmt.Sprintf("FAIL 名册点名了非真源常量 %s（凭空点名 ⇒ 名册与代码分叉）", d))
		}
	}
	for _, e := range roster {
		for _, d := range e.named {
			ap := vsPyAbspath("", vsPyJoin(root, d))
			if vsIsDir(ap) {
				entries = append(entries, vsDefEntry{root: e.id, dir: d, state: "在盘"})
				continue
			}
			reg, ok := relocated[d]
			if !ok {
				problems = append(problems, fmt.Sprintf("FAIL 默认根 %s 点名的目录 %s 既不在盘、又未登记迁出（静默兜底 ⇒ 未点名覆盖）", e.id, d))
				entries = append(entries, vsDefEntry{root: e.id, dir: d, state: "不在盘·未登记"})
				continue
			}
			lap := ""
			if reg.landed != "" {
				lap = vsPyAbspath("", vsPyJoin(root, reg.landed))
			}
			if reg.reason == "" || reg.landed == "" || !vsIsDir(lap) {
				problems = append(problems, fmt.Sprintf("FAIL 默认根 %s 的 %s 已登记迁出，但落点 %s 不在盘（登记指向死人）", e.id, d, reg.landed))
				entries = append(entries, vsDefEntry{root: e.id, dir: d, state: "登记·落点不在盘", landed: reg.landed, hasLanded: true})
			} else {
				entries = append(entries, vsDefEntry{root: e.id, dir: d, state: "登记迁出", landed: reg.landed, hasLanded: true})
			}
		}
	}
	for d := range relocated {
		if !vsContains(namedAll, d) {
			problems = append(problems, fmt.Sprintf("FAIL 迁出登记 %s 不在运行时默认根名册内（悬空登记）", d))
		}
	}
	return problems, blockers, entries
}

const vsCorpusRootMD = true
const vsMaxFileBytes = 2 * 1024 * 1024

var vsSkipDirNames = map[string]bool{
	".obsidian": true, ".git": true, "target": true,
	"node_modules": true, ".venv": true, "__pycache__": true,
}

// 静态文案（由生成器按 py 原件逐字填入 · 见构建脚本）。
const vsDocText = "check-version-sources.py v1.0.0 —— 版本源一致性门禁（门② · 目标：阻断档）\n\n断言**四处版本源同版**（真源 = version.go；理由见仓根 VERSION 的指针文本）：\n  ① 仓根 VERSION                        —— **指针**：只断言「**无数值**」且「**文本指向真源**」\n  ② core/internal/version/version.go    —— `const Version = \"X\"`（**基准**）\n  ③ ui/Cargo.toml                       —— `[package] version = \"X\"`\n  ④ wall/Cargo.toml                     —— `[package] version = \"X\"`（茧壁 crate，升级比较要用）\n\n另**只报告**（**不进退码**）：**活文档里仍写着旧版本号字面量**的清单\n  · 旧版集合默认 = 「直接前驱版」= 快照目录里已知的 3 段版本号中 < 当前版的最大者\n    （★ 2026-09-19 二次分家批2：快照目录**取源根双认** = 仓内 `docs/项目文档` 优先、缺则仓外\n     `../Zerg-内部文档/项目文档`；两处都缺 ⇒ **rc=2 不给结论**，不给「没有历史版本可比」的假绿）\n  · 排除域（冻结/镜像/证据/Tool 目录）见 RULES 的 doc_excludes；命中逐条列 `文件:行`\n  · 命中**分两类**：`版本名/路径形态`（`v2.5.9`、`…/v2.5.9/…`、`变更-v2.5.9.md` —— 指的是旧版的名字，登记为主）\n    与 `裸数字形态`（`2.5.9 → 2.5.10`、`2.5.9 增补` 之类 —— 需人判读：可能是陈旧陈述、也可能是沿革陈述）\n  · ★ 本项**只报告**：不构成判决；判读由人做（要不要加严、门槛多少待拍板）。`--strict-docs` 可当场加严。\n\n退码（三档，与 scripts/gates/precommit-gates.sh 的 `tri` 模式同口径 —— 该脚本第 29 行起）：\n  0 全绿（成对判定：四处同版 + VERSION 无数值 + 指针指向真源）\n  1 有失败项（版本不一致 / VERSION 出现数值 / 指针不指真源 / `--strict-docs` 下的活文档命中）\n  2 **不给结论**（缺件 · 不可解析 · 空转 · 用法错）—— 空转不许当绿\n\n用法：\n  python3 scripts/gates/check-version-sources.py                    # 扫真仓（退出码见上）\n  python3 scripts/gates/check-version-sources.py [仓库根]           # 指定根（自检夹具就走这条）\n  python3 scripts/gates/check-version-sources.py --json             # 机器可读（stdout 只有 JSON）\n  python3 scripts/gates/check-version-sources.py --list-rules       # 规则表（判据 / 排除域 / 旧版集合 / 退码映射）\n  python3 scripts/gates/check-version-sources.py --self-test        # 成对负控自检（好件必绿 ↔ 每处改一个数必红）\n  python3 scripts/gates/check-version-sources.py --strict-docs      # 活文档命中 ⇒ 计入 rc=1（默认只报告）\n  python3 scripts/gates/check-version-sources.py --stale 2.5.9 --stale 2.5.8   # 手工指定旧版集合\n  python3 scripts/gates/check-version-sources.py --stale-all        # 旧版集合 = 全部已知 3 段历史版本\n\n硬纪律：**只读**（不写仓内任何路径；自检一律落 `tempfile.mkdtemp()` 的临时目录）· **不挂门禁**\n（挂接由父代理做：进 `scripts/gates/precommit-gates.sh` 的步骤表，模式 `tri` —— rc=2 记 BLOCKED、不计失败项）。\n"
const vsRulesBlock1 = "\n[判据 · 阻断档]\n  R1 四处同版   core/internal/version/version.go 的 `const Version = \"X\"` = **基准**；\n               ui/Cargo.toml 与 wall/Cargo.toml 的 **[package] 段** version 必须等于基准；\n               不等 ⇒ rc=1，逐条打印 `文件:行 值 ≠ 基准`（不含基准自身）\n  R2 VERSION 无数值   仓根 VERSION 里出现任何 `X.Y.Z` 字面量 ⇒ rc=1（逐行点名）。\n               理由：VERSION 已**指针化**（真源是 version.go + Cargo.toml），写数字就是第二份真相\n  R3 VERSION 指真源   指针文本必须同时点名 `core/internal/version/version.go` 与 `ui/Cargo.toml`\n               （判据是**路径串**，不是措辞；缺一 ⇒ rc=1）\n  R4 缺件/不可解析   任一源缺失 · 不是 UTF-8 · version.go 无 `const Version` ·\n               Cargo.toml 的 [package] 段无 version · VERSION 是空文件 ⇒ rc=2（**不给结论**）\n  R5 空转防护   活文档语料 0 个文件被扫到（而旧版集合非空）⇒ rc=2（0 个可比对 ≠ 绿）\n  R6 默认根点名  运行时默认根（仓根标记 core/ui · 语料域根 docs · 快照取源根 docs/项目文档）逐个点名：\n               被点名的目录必须在盘，否则必须登记迁出（RELOCATED_ROOTS：理由 + 落点在盘）。\n               名册与真源常量双向闭合；名册为空 ⇒ rc=2，未在盘且未登记 / 登记落点不在盘 / 悬空登记 ⇒ rc=1\n\n[只报告 · 不进退出码]\n  D1 活文档旧版本号   语料域 = docs/**（走下方排除域）+ 仓根 *.md；\n               旧版集合默认 = 直接前驱版（`--stale X.Y.Z` 可指定 / `--stale-all` 收全部历史版）；\n               逐处分类：REF=版本名/路径形态（`v2.5.9`、`…/v2.5.9/`、`变更-v2.5.9.md`）·\n               BARE=裸数字形态（`2.5.9 → 2.5.10`、`2.5.9 增补`）—— 后者才是需人判读的候选；\n               命中只列清单。`--strict-docs` 可当场把命中计进 rc=1（加严开关，默认关）"
const vsRulesBlock2 = "\n[退码映射（三档；与 scripts/gates/precommit-gates.sh 的 tri 模式同口径）]\n  0  全绿            四处同版 + VERSION 无数值 + 指针指真源\n  1  有失败项        R1/R2/R3 任一红（或 --strict-docs 下的 D1 命中）\n  2  **不给结论**    R4/R5（缺件/不可解析/空转/用法错）—— BLOCKED 不许当绿\n  挂接口径：进 precommit-gates.sh 步骤表时用 `tri`（rc=2 记 BLOCKED、不计失败项）；\n  若哪一天 D1 要转阻断档，**改判定档位、不改判据**（门脚本一字不动）。"

// ─────────────────────────── 路径语义（CPython posixpath 同构）───────────────────────────

// vsPyJoin 复刻 posixpath.join（逐段规则 · 后段绝对则丢前段 · 空段留分隔符）。
func vsPyJoin(parts ...string) string {
	out := ""
	for _, p := range parts {
		switch {
		case strings.HasPrefix(p, "/"):
			out = p
		case out == "" || strings.HasSuffix(out, "/"):
			out += p
		default:
			out += "/" + p
		}
	}
	return out
}

// vsPyNormpath 复刻 posixpath.normpath。★ 保留恰两个前导斜杠（`//a/b` 不折成 `/a/b`）。
func vsPyNormpath(path string) string {
	if path == "" {
		return "."
	}
	initialSlashes := 0
	if strings.HasPrefix(path, "/") {
		initialSlashes = 1
		if strings.HasPrefix(path, "//") && !strings.HasPrefix(path, "///") {
			initialSlashes = 2
		}
	}
	var comps []string
	for _, comp := range strings.Split(path, "/") {
		switch {
		case comp == "" || comp == ".":
		case comp != ".." || (initialSlashes == 0 && len(comps) == 0) || (len(comps) > 0 && comps[len(comps)-1] == ".."):
			comps = append(comps, comp)
		default:
			if len(comps) > 0 {
				comps = comps[:len(comps)-1]
			}
		}
	}
	out := strings.Join(comps, "/")
	if initialSlashes > 0 {
		out = strings.Repeat("/", initialSlashes) + out
	}
	if out == "" {
		return "."
	}
	return out
}

// vsPyAbspath 复刻 posixpath.abspath（纯词法 · cwd 由调用方传入）。
func vsPyAbspath(cwd, path string) string {
	return vsPyNormpath(vsPyJoin(cwd, path))
}

// vsPyRelpath 复刻 posixpath.relpath（start 传 cwd）。
func vsPyRelpath(cwd, path, start string) string {
	if path == "" {
		return ""
	}
	var split = func(p string) []string {
		var out []string
		for _, c := range strings.Split(p, "/") {
			if c != "" {
				out = append(out, c)
			}
		}
		return out
	}
	s, p := split(vsPyAbspath(cwd, start)), split(vsPyAbspath(cwd, path))
	i := 0
	for i < len(s) && i < len(p) && s[i] == p[i] {
		i++
	}
	var rel []string
	for k := 0; k < len(s)-i; k++ {
		rel = append(rel, "..")
	}
	rel = append(rel, p[i:]...)
	if len(rel) == 0 {
		return "."
	}
	return strings.Join(rel, "/")
}

// vsPyDirname 复刻 posixpath.dirname（= split 的 head 段 · 剥尾斜杠）。
func vsPyDirname(p string) string {
	i := strings.LastIndex(p, "/") + 1
	head := p[:i]
	if head != "" && head != strings.Repeat("/", len(head)) {
		head = strings.TrimRight(head, "/")
	}
	return head
}

// ─────────────────────────── py 字符串原语 ───────────────────────────

// vsPyIsSpace 复刻 py str.isspace() 的码点集（含 \x1c-\x1f / \x85 / \xa0）。
func vsPyIsSpace(r rune) bool {
	switch {
	case r == 0x09 || r == 0x0a || r == 0x0b || r == 0x0c || r == 0x0d || r == 0x20:
		return true
	case r >= 0x1c && r <= 0x1f:
		return true
	case r == 0x85 || r == 0xa0 || r == 0x1680:
		return true
	case r >= 0x2000 && r <= 0x200a:
		return true
	case r == 0x2028 || r == 0x2029 || r == 0x202f || r == 0x205f || r == 0x3000:
		return true
	}
	return false
}

func vsPyStrip(s string) string {
	return strings.TrimFunc(s, vsPyIsSpace)
}

// vsSplitLines 复刻 py str.splitlines()（行边界集按 py）。
func vsSplitLines(s string) []string {
	var out []string
	var cur []rune
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == '\n' || r == '\v' || r == '\f' || r == 0x1c || r == 0x1d || r == 0x1e ||
			r == 0x85 || r == 0x2028 || r == 0x2029 {
			out = append(out, string(cur))
			cur = cur[:0]
			i += size
			continue
		}
		cur = append(cur, r)
		i += size
	}
	if len(cur) > 0 {
		out = append(out, string(cur))
	}
	return out
}

// vsUniversalNewlines 复刻 open() 文本模式的通用换行翻译。
func vsUniversalNewlines(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}

// ─────────────────────────── 正则（`\s` 展开为显式 Unicode 空白类）───────────────────────────

const vsWSClass = `\t\n\v\f\r \x{1c}-\x{1f}\x{85}\x{a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}`

var vsREGoConst = regexp.MustCompile(`^[` + vsWSClass + `]*const[` + vsWSClass + `]+Version[` + vsWSClass + `]*=[` + vsWSClass + `]*"([^"]+)"[` + vsWSClass + `]*$`)
var vsRECargoVer = regexp.MustCompile(`^version[` + vsWSClass + `]*=[` + vsWSClass + `]*"([^"]+)"`)

type vsSpan struct{ start, end int } // 字节偏移

func vsIsNd(r rune) bool { return unicode.IsDigit(r) }

func vsIsDigitOrDot(r rune) bool { return unicode.IsDigit(r) || r == '.' }

func vsScanDigits(s string, j int) int {
	for j < len(s) {
		r, size := utf8.DecodeRuneInString(s[j:])
		if !vsIsNd(r) {
			break
		}
		j += size
	}
	return j
}

// vsNumFindAll 手写 NUM_RE：`(?<![\d.])(\d+\.\d+\.\d+)(?![\d.])` 的全部匹配（左起不重叠）。
func vsNumFindAll(s string) []vsSpan {
	var out []vsSpan
	n := len(s)
	i := 0
	for i < n {
		r, sz := utf8.DecodeRuneInString(s[i:])
		if i > 0 {
			pr, _ := utf8.DecodeLastRuneInString(s[:i])
			if vsIsDigitOrDot(pr) {
				i += sz
				continue
			}
		}
		if !vsIsNd(r) {
			i += sz
			continue
		}
		j := vsScanDigits(s, i)
		if j >= n || s[j] != '.' {
			i += sz
			continue
		}
		j++
		k := vsScanDigits(s, j)
		if k == j {
			i += sz
			continue
		}
		j = k
		if j >= n || s[j] != '.' {
			i += sz
			continue
		}
		j++
		k = vsScanDigits(s, j)
		if k == j {
			i += sz
			continue
		}
		j = k
		if j < n {
			jr, _ := utf8.DecodeRuneInString(s[j:])
			if vsIsDigitOrDot(jr) {
				i += sz
				continue
			}
		}
		out = append(out, vsSpan{i, j})
		i = j
	}
	return out
}

func vsNumFullmatch(s string) bool {
	sp := vsNumFindAll(s)
	return len(sp) == 1 && sp[0].start == 0 && sp[0].end == len(s)
}

// vsLiteralFindAll 复刻 `re.finditer(re.escape(token) + r"(?![\d.])", line)` 的匹配区间。
func vsLiteralFindAll(line, token string) []vsSpan {
	var out []vsSpan
	if token == "" {
		// re.escape("") == "" ⇒ 模式退化为 `(?![\d.])`（零宽 · 在每个后界非数字/点的位置命中）。
		for p := 0; p <= len(line); {
			ok := true
			if p < len(line) {
				r, _ := utf8.DecodeRuneInString(line[p:])
				if vsIsDigitOrDot(r) {
					ok = false
				}
			}
			if ok {
				out = append(out, vsSpan{p, p})
			}
			if p >= len(line) {
				break
			}
			_, sz := utf8.DecodeRuneInString(line[p:])
			p += sz
		}
		return out
	}
	pos := 0
	for pos <= len(line) {
		idx := strings.Index(line[pos:], token)
		if idx < 0 {
			break
		}
		abs := pos + idx
		end := abs + len(token)
		if end >= len(line) {
			out = append(out, vsSpan{abs, end})
			pos = end
			continue
		}
		jr, _ := utf8.DecodeRuneInString(line[end:])
		if vsIsDigitOrDot(jr) {
			pos = abs + 1
			continue
		}
		out = append(out, vsSpan{abs, end})
		pos = end
	}
	return out
}

// ─────────────────────────── 解析器 ───────────────────────────

// vsReadText 读 UTF-8 文本；返回 (text, err)。err != "" ⇒ 不可解析（BLOCKED）。
func vsReadText(path string) (string, string) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", "读不到（" + vsOSErrStr(err, path) + "）"
	}
	if !utf8.Valid(b) {
		return "", "不是 UTF-8 文本（" + vsUnicodeDecodeErrMsg(b) + "）"
	}
	return vsUniversalNewlines(string(b)), ""
}

func vsStrerror(e int) string {
	switch e {
	case 1:
		return "Operation not permitted"
	case 2:
		return "No such file or directory"
	case 13:
		return "Permission denied"
	case 20:
		return "Not a directory"
	case 21:
		return "Is a directory"
	case 40:
		return "Too many levels of symbolic links"
	}
	return syscall.Errno(e).Error()
}

func vsOSErrStr(err error, path string) string {
	var pe *os.PathError
	if errors.As(err, &pe) {
		var errno syscall.Errno
		if errors.As(pe.Err, &errno) {
			return fmt.Sprintf("[Errno %d] %s: '%s'", int(errno), vsStrerror(int(errno)), path)
		}
	}
	return fmt.Sprintf("%v", err)
}

// vsUnicodeDecodeErrMsg 近似复刻 CPython UnicodeDecodeError 的正文（未覆盖面②）。
func vsUnicodeDecodeErrMsg(b []byte) string {
	i := 0
	for i < len(b) {
		c := b[i]
		if c < 0x80 {
			i++
			continue
		}
		var need int
		var lo, hi byte = 0x80, 0xbf
		switch {
		case c >= 0xc2 && c <= 0xdf:
			need = 1
		case c == 0xe0:
			need = 2
			lo = 0xa0
		case c >= 0xe1 && c <= 0xec:
			need = 2
		case c == 0xed:
			need = 2
			hi = 0x9f
		case c >= 0xee && c <= 0xef:
			need = 2
		case c == 0xf0:
			need = 3
			lo = 0x90
		case c >= 0xf1 && c <= 0xf3:
			need = 3
		case c == 0xf4:
			need = 3
			hi = 0x8f
		default:
			return fmt.Sprintf("'utf-8' codec can't decode byte 0x%02x in position %d: invalid start byte", c, i)
		}
		for k := 1; k <= need; k++ {
			if i+k >= len(b) {
				return fmt.Sprintf("'utf-8' codec can't decode bytes in position %d-%d: unexpected end of data", i, len(b)-1)
			}
			cc := b[i+k]
			ok := cc >= 0x80 && cc <= 0xbf
			if k == 1 {
				ok = ok && cc >= lo && cc <= hi
			}
			if !ok {
				return fmt.Sprintf("'utf-8' codec can't decode byte 0x%02x in position %d: invalid continuation byte", cc, i+k)
			}
		}
		i += need + 1
	}
	return "'utf-8' codec can't decode: invalid data"
}

// vsParseGoConst `const <name> = "X"` ⇒ (值, 行号, 命中)。
func vsParseGoConst(text string) (string, int, bool) {
	for i, line := range vsSplitLines(text) {
		if m := vsREGoConst.FindStringSubmatch(line); m != nil {
			return m[1], i + 1, true
		}
	}
	return "", 0, false
}

// vsParseCargoPkgVersion 只在 [package] 段取 version。
func vsParseCargoPkgVersion(text string) (string, int, bool) {
	section := ""
	for i, raw := range vsSplitLines(text) {
		line := vsPyStrip(raw)
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = vsPyStrip(strings.Trim(line, "[]"))
			continue
		}
		if section == "package" {
			if m := vsRECargoVer.FindStringSubmatch(line); m != nil {
				return m[1], i + 1, true
			}
		}
	}
	return "", 0, false
}

// vsVK 版本号排序键（3 段数字；失败给 [-1]，对应 py 的 (-1,)）。
func vsVK(v string) []int {
	parts := strings.Split(v, ".")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return []int{-1}
		}
		out = append(out, n)
	}
	return out
}

// vsCmpKeys py 元组字典序比较。
func vsCmpKeys(a, b []int) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	switch {
	case len(a) < len(b):
		return -1
	case len(a) > len(b):
		return 1
	}
	return 0
}

// ─────────────────────────── 文件系统谓词（跟随/不跟随符号链接）───────────────────────────

func vsExists(p string) bool { _, err := os.Stat(p); return err == nil }
func vsIsDir(p string) bool  { fi, err := os.Stat(p); return err == nil && fi.IsDir() }
func vsIsFile(p string) bool { fi, err := os.Stat(p); return err == nil && !fi.IsDir() }
func vsIsSymlink(p string) bool {
	_, err := os.Lstat(p)
	if err != nil {
		return false
	}
	fi, err := os.Lstat(p)
	return err == nil && fi.Mode()&os.ModeSymlink != 0
}

func vsListDir(d string) ([]string, error) {
	f, err := os.Open(d)
	if err != nil {
		return nil, err
	}
	des, err := f.ReadDir(-1)
	f.Close()
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(des))
	for _, de := range des {
		out = append(out, de.Name())
	}
	return out, nil
}

// vsWalkPy 复刻 os.walk(followlinks=False) 的产出序：先当前层 files（已按 py sorted 排）、再按 dirs 原序下钻。
func vsWalkPy(dir string, fn func(dirpath string, filenames []string)) {
	des, err := vsReadDirRaw(dir)
	if err != nil {
		return
	}
	var dirs, files []string
	for _, name := range des {
		if vsIsDir(vsPyJoin(dir, name)) {
			dirs = append(dirs, name)
		} else {
			files = append(files, name)
		}
	}
	sort.Strings(files)
	fn(dir, files)
	for _, d := range dirs {
		if vsSkipDirNames[d] || strings.HasPrefix(d, ".") {
			continue
		}
		if vsIsSymlink(vsPyJoin(dir, d)) {
			continue
		}
		vsWalkPy(vsPyJoin(dir, d), fn)
	}
}

func vsReadDirRaw(dir string) ([]string, error) {
	f, err := os.Open(dir)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	des, err := f.ReadDir(-1)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(des))
	for _, de := range des {
		out = append(out, de.Name())
	}
	return out, nil
}

// ─────────────────────────── 只报告：活文档旧版本号 ───────────────────────────

type vsOrderedCounts struct {
	keys []string
	vals []int
}

func (o *vsOrderedCounts) add(k string) {
	for i := range o.keys {
		if o.keys[i] == k {
			o.vals[i]++
			return
		}
	}
	o.keys = append(o.keys, k)
	o.vals = append(o.vals, 1)
}

type vsSkipped struct{ path, why string }

type vsExcl struct{ pref, why string }

var vsDocExcludesFixed = []vsExcl{
	{"Zerg-内部文档/", "开发文档分家（2026-09-19）：设计稿/调研稿/评审稿/问题单整树在**工作树之外**的同级目录 ⇒ 仓内若出现同名路径（搬回 / 软链 / 副本），其版本号属归档料，只登记不判"},
}

type vsExclRule struct{ pat, why string }

var vsDocExcludesRule = []vsExclRule{
	{"docs/项目文档/v<X>/（X ≠ 当前版）", "他版快照目录（17 套历史快照，冻结归档，勿改）"},
	{"docs/项目文档/v<X>/（X = 当前版）", "**不排除** —— 它是活文档，命中要进清单"},
	{"docs/.* 与 */.hidden", "工具/隐藏目录（如 docs/.obsidian/）"},
}

const vsDocExcludesNote = "排除域只影响**只报告**那一段；四处版本源的断言不受任何排除域影响。"

// vsDocsSnapshotRoot 快照目录取源根（仓内优先、缺则仓外同级）。
func vsDocsSnapshotRoot(cwd, root string) (string, string) {
	for _, rel := range vsDocsRelCandidates {
		p := vsPyAbspath(cwd, vsPyJoin(root, rel))
		if vsIsDir(p) {
			src := "仓内 docs/项目文档"
			if strings.HasPrefix(rel, "..") {
				src = "仓外取源根 ../Zerg-内部文档/项目文档"
			}
			return p, src
		}
	}
	return "", "两处取源根都缺（仓内 docs/项目文档 · 仓外 ../Zerg-内部文档/项目文档）⇒ 快照目录不在盘上"
}

func vsDeriveStaleSet(cwd, root, canonical string, useAll bool) ([]string, []string, string) {
	d, dsrc := vsDocsSnapshotRoot(cwd, root)
	var known []string
	if d != "" {
		names, err := vsListDir(d)
		if err == nil {
			sort.Strings(names)
			for _, name := range names {
				if !strings.HasPrefix(name, "v") {
					continue
				}
				v := name[1:]
				if vsNumFullmatch(v) {
					known = append(known, v)
				}
			}
		}
	}
	if canonical == "" {
		return []string{}, known, "当前版未知 ⇒ 旧版集合为空（只报告项，不进退出码）"
	}
	seen := map[string]bool{}
	var cand []string
	for _, v := range known {
		if seen[v] {
			continue
		}
		seen[v] = true
		if v != canonical && vsCmpKeys(vsVK(v), vsVK(canonical)) < 0 {
			cand = append(cand, v)
		}
	}
	sort.SliceStable(cand, func(i, j int) bool { return vsCmpKeys(vsVK(cand[i]), vsVK(cand[j])) < 0 })
	if len(cand) == 0 {
		src := "（无）"
		if d != "" {
			src = dsrc
		}
		return []string{}, known, "没有历史版本目录可比 ⇒ 旧版集合为空（只报告项，不进退出码）·取源根 = " + src
	}
	if useAll {
		return cand, known, "旧版集合 = 全部历史版本（--stale-all）·取源根 = " + dsrc
	}
	return []string{cand[len(cand)-1]}, known, "旧版集合 = 直接前驱版（默认；要收全部用 --stale-all）·取源根 = " + dsrc
}

func vsIsExcludedZone(rel, canonical string) (string, bool) {
	for _, e := range vsDocExcludesFixed {
		if strings.HasPrefix(rel, e.pref) {
			return "固定排除域", true
		}
	}
	if strings.HasPrefix(rel, "docs/项目文档/") {
		rest := rel[len("docs/项目文档/"):]
		dirver := strings.Split(rest, "/")[0]
		if strings.HasPrefix(dirver, "v") && canonical != "" && dirver[1:] != canonical {
			return "他版快照", true
		}
		if strings.HasPrefix(dirver, "v") && dirver[1:] != canonical {
			return "他版快照", true
		}
	}
	segs := strings.Split(rel, "/")
	for _, s := range segs[:len(segs)-1] {
		if strings.HasPrefix(s, ".") {
			return "隐藏/工具目录", true
		}
	}
	if strings.HasPrefix(segs[len(segs)-1], ".") {
		return "隐藏文件", true
	}
	return "", false
}

func vsCollectCorpus(cwd, root, canonical string) ([]string, []vsSkipped, *vsOrderedCounts) {
	var files []string
	excluded := &vsOrderedCounts{}
	docs := vsPyJoin(root, "docs")
	if vsIsDir(docs) {
		vsWalkPy(docs, func(dirpath string, filenames []string) {
			for _, fn := range filenames {
				p := vsPyJoin(dirpath, fn)
				rel := strings.ReplaceAll(vsPyRelpath(cwd, p, root), string(os.PathSeparator), "/")
				if _, ok := vsIsExcludedZone(rel, canonical); ok {
					parts := strings.Split(rel, "/")
					second := ""
					if len(parts) > 2 {
						second = parts[1]
					}
					excluded.add(parts[0] + "/" + second)
					continue
				}
				files = append(files, rel)
			}
		})
	}
	if vsCorpusRootMD {
		names, err := vsListDir(root)
		if err == nil {
			sort.Strings(names)
			for _, fn := range names {
				if strings.HasSuffix(fn, ".md") && vsIsFile(vsPyJoin(root, fn)) {
					files = append(files, fn)
				}
			}
		}
	}
	uniq := vsUniqueSorted(files)
	return uniq, nil, excluded
}

func vsUniqueSorted(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	if out == nil {
		out = []string{}
	}
	return out
}

type vsHit struct {
	path  string
	line  int
	stale string
	class string
	text  string
}

func vsClassifyOccurrence(line string, sp vsSpan) string {
	before := line[:sp.start]
	after := line[sp.end:]
	if before != "" {
		r, _ := utf8.DecodeLastRuneInString(before)
		if r == 'v' || r == 'V' {
			return "REF"
		}
	}
	if strings.HasPrefix(after, "/") {
		return "REF"
	}
	if strings.HasSuffix(before, "/") {
		return "REF"
	}
	if after != "" {
		r, _ := utf8.DecodeRuneInString(after)
		if r == '-' || r == '_' || r == '.' || r == '+' {
			return "REF"
		}
	}
	return "BARE"
}

type vsDocs struct {
	staleVersions []string
	corpusFiles   int
	scannedFiles  int
	hitFiles      int
	hits          int
	bare          int
	ref           int
	hitList       []vsHit
	skipped       []vsSkipped
	excluded      *vsOrderedCounts
}

func vsScanLiveDocs(cwd, root string, staleSet []string, canonical string) *vsDocs {
	files, skipped, excluded := vsCollectCorpus(cwd, root, canonical)
	var hits []vsHit
	scanned := 0
	for _, rel := range files {
		p := vsPyJoin(root, rel)
		fi, err := os.Stat(p)
		if err != nil {
			skipped = append(skipped, vsSkipped{rel, "取不到大小"})
			continue
		}
		if fi.Size() > int64(vsMaxFileBytes) {
			skipped = append(skipped, vsSkipped{rel, fmt.Sprintf("超过 %d 字节", vsMaxFileBytes)})
			continue
		}
		text, rerr := vsReadText(p)
		if rerr != "" {
			skipped = append(skipped, vsSkipped{rel, rerr})
			continue
		}
		scanned++
		for i, line := range vsSplitLines(text) {
			stripped := vsPyStrip(line)
			for _, token := range staleSet {
				for _, sp := range vsLiteralFindAll(line, token) {
					txt := stripped
					if rr := []rune(txt); len(rr) > 200 {
						txt = string(rr[:200])
					}
					hits = append(hits, vsHit{
						path: rel, line: i + 1, stale: token,
						class: vsClassifyOccurrence(line, sp), text: txt,
					})
				}
			}
		}
	}
	fileSet := map[string]bool{}
	bare, refc := 0, 0
	for _, h := range hits {
		fileSet[h.path] = true
		if h.class == "BARE" {
			bare++
		} else {
			refc++
		}
	}
	return &vsDocs{
		staleVersions: append([]string{}, staleSet...),
		corpusFiles:   len(files),
		scannedFiles:  scanned,
		hitFiles:      len(fileSet),
		hits:          len(hits),
		bare:          bare,
		ref:           refc,
		hitList:       hits,
		skipped:       skipped,
		excluded:      excluded,
	}
}

// ─────────────────────────── 四处版本源断言 ───────────────────────────

type vsSource struct {
	id, role, path string
	line           int
	hasLine        bool
	value          string
	hasValue       bool
	ok             bool
	note           string
	hasNote        bool
}

type vsResult struct {
	root         string
	canonical    string
	hasCanonical bool
	sources      []vsSource
	problems     []string
	blockers     []string
	docs         *vsDocs
	defEntries   []vsDefEntry
	ddEntries    []vsDDEnt
	rc           int
}

type vsSiteEnt struct{ file, pat, role, want, note, reviewDate string }

type vsDDEnt struct{ file, role, state, got, want, note, reviewDate string }

// vsDerivedDefaultSites —— R7 派生默认层版本位点登记册（缺口 GAP-20260927-89 · py 同名段逐字镜像）。
var vsReReviewDate = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
var vsDerivedDefaultSites = []vsSiteEnt{
	{"scripts/gates/check-doc-freshness.py", `os\.path\.join\(sibling, "项目文档", "(v[0-9.]+)"\)`, "pinned", "v2.5.10", "乙·沿革/引证：v2.5.10 是文档面上一次定稿版（改真源 2.5.13 破 devdocs:meta/freshness D1 两阻断步）· Mr2109 2026-10-08 拍·GAP-20260927-89", "2026-10-08"},
	{"core/cmd/zerg/check_doc_freshness.go", `cdfSlugVersionDir = "(v[0-9.]+)"`, "pinned", "v2.5.10", "乙·沿革同上（Go 半 · 缺口 GAP-20260926-174）", "2026-10-08"},
	{"core/cmd/zerg/check_doc_meta.go", `cdmPyJoin\(sibling, "项目文档", "(v[0-9.]+)"\)`, "pinned", "v2.5.10", "乙·沿革同上（devdocs 默认根）", "2026-10-08"},
	{"core/cmd/zerg/check_doc_meta.go", `\{"docs/项目文档/(v[0-9.]+)/"`, "pinned", "v2.5.10", "FORMAL_PREFIX 首棵（面登记 · 沿革同上）", "2026-10-08"},
	{"core/cmd/zerg/check_glossary.go", `chglJoin\(sibling, "项目文档", "(v[0-9.]+)"\)`, "pinned", "v2.5.10", "乙·沿革同上（devdocs 默认根）", "2026-10-08"},
	{"core/cmd/zerg/check_approve_ci.go", `caciDevdocsTail = caciPyJoin\("项目文档", "(v[0-9.]+)"\)`, "pinned", "v2.5.11", "乙·引证：口径-CI永不人签-20260923.md 实际只在 v2.5.11（他版 0 份）", "2026-10-08"},
	{"core/cmd/zerg/check_approve_closedset.go", `cacsDevdocsTail = caciPyJoin\("项目文档", "(v[0-9.]+)"\)`, "pinned", "v2.5.11", "乙·引证：S2 单向铁律台账实际只在 v2.5.11", "2026-10-08"},
	{"core/cmd/zerg/check_approve_freeze.go", `cafrDocsVer\s+= "(v[0-9.]+)"`, "pinned", "v2.5.11", "乙·引证：缺口-命令面-20260922.md 顶账 16 条实际只在 v2.5.11", "2026-10-08"},
}

// vsCheckDerivedDefaults —— R7：派生默认层版本位点登记册。
// derived 档：定位串是**真源钩子**，捕获值须 == `v<canonical>`；pinned 档：捕获值须 == 登记值。
func vsCheckDerivedDefaults(root, canonical string, roster []vsSiteEnt) ([]string, []string, []vsDDEnt) {
	if roster == nil {
		roster = vsDerivedDefaultSites
	}
	var problems, blockers []string
	var entries []vsDDEnt
	if len(roster) == 0 {
		blockers = append(blockers, "空转：派生默认层位点登记册为空（判据不可判 ⇒ 不给结论）")
		return problems, blockers, entries
	}
	for _, e := range roster {
		if !vsReReviewDate.MatchString(e.reviewDate) {
			problems = append(problems, fmt.Sprintf("FAIL %s 缺/坏 review_date=%s（要 `YYYY-MM-DD` —— 钉版不许无复查日）", e.file, e.reviewDate))
		}
		ap := vsPyJoin(root, e.file)
		if st, err := os.Stat(ap); err != nil || st.IsDir() {
			entries = append(entries, vsDDEnt{file: e.file, role: e.role, state: "件缺·跳过", got: "", want: e.want, note: e.note, reviewDate: e.reviewDate})
			continue
		}
		b, err := os.ReadFile(ap)
		if err != nil {
			problems = append(problems, fmt.Sprintf("FAIL %s 读不了：%v（位点判不了）", e.file, err))
			entries = append(entries, vsDDEnt{file: e.file, role: e.role, state: "读失败", got: "", want: e.want, note: e.note, reviewDate: e.reviewDate})
			continue
		}
		re, cerr := regexp.Compile(e.pat)
		if cerr != nil {
			problems = append(problems, fmt.Sprintf("FAIL %s 位点定位串坏：%v", e.file, cerr))
			entries = append(entries, vsDDEnt{file: e.file, role: e.role, state: "定位串坏", got: "", want: e.want, note: e.note, reviewDate: e.reviewDate})
			continue
		}
		ms := re.FindAllStringSubmatch(string(b), -1)
		if len(ms) != 1 {
			problems = append(problems, fmt.Sprintf("FAIL %s 位点定位串命中 %d 次（应为 1）⇒ 位点丢失/歧义：%s", e.file, len(ms), e.pat))
			entries = append(entries, vsDDEnt{file: e.file, role: e.role, state: fmt.Sprintf("定位%dx", len(ms)), got: "", want: e.want, note: e.note, reviewDate: e.reviewDate})
			continue
		}
		got := ms[0][1]
		want := e.want
		if e.role == "derived" {
			if canonical == "" {
				blockers = append(blockers, fmt.Sprintf("不给结论：真源版本未解出 ⇒ 派生位点判不了（%s）", e.file))
				entries = append(entries, vsDDEnt{file: e.file, role: e.role, state: "真源未解", got: got, want: "", note: e.note, reviewDate: e.reviewDate})
				continue
			}
			want = "v" + canonical
		}
		state := "命中"
		if got != want {
			state = "漂移"
			problems = append(problems, fmt.Sprintf("FAIL %s 版本目录位点 = %s ≠ 登记期望 %s（%s · %s）", e.file, got, want, e.role, e.note))
		}
		entries = append(entries, vsDDEnt{file: e.file, role: e.role, state: state, got: got, want: want, note: e.note, reviewDate: e.reviewDate})
	}
	return problems, blockers, entries
}

func vsCheckSources(cwd, root string, staleSet []string, strictDocs bool, roster []vsDefRootEnt, relocated map[string]vsRelocEnt) *vsResult {
	var problems, blockers []string
	var sources []vsSource

	need := func(path string) (string, bool) {
		ap := vsPyJoin(root, path)
		if !vsExists(ap) {
			blockers = append(blockers, fmt.Sprintf("缺件：%s 不存在（判据不可判 ⇒ 不给结论）", path))
			return "", false
		}
		text, err := vsReadText(ap)
		if err != "" {
			blockers = append(blockers, fmt.Sprintf("不可解析：%s %s", path, err))
			return "", false
		}
		return text, true
	}

	canonical := ""
	if txtGo, ok := need(vsPGo); ok {
		val, ln, found := vsParseGoConst(txtGo)
		if !found || val == "" {
			blockers = append(blockers, fmt.Sprintf("不可解析：%s 里找不到 `const Version = \"X\"`（判据不可判）", vsPGo))
		} else {
			canonical = val
		}
		sources = append(sources, vsSource{id: "go", role: "基准（真源）", path: vsPGo,
			line: ln, hasLine: found, value: val, hasValue: found, ok: found && val != ""})
	} else {
		sources = append(sources, vsSource{id: "go", role: "基准（真源）", path: vsPGo, ok: false})
	}

	if txtVer, ok := need(vsPVersionFile); ok {
		var nums []string
		for _, sp := range vsNumFindAll(txtVer) {
			nums = append(nums, txtVer[sp.start:sp.end])
		}
		var numLines []int
		for i, l := range vsSplitLines(txtVer) {
			if len(vsNumFindAll(l)) > 0 {
				numLines = append(numLines, i+1)
			}
		}
		var ptrMissing []string
		for _, p := range vsVersionPointers {
			if !strings.Contains(txtVer, p) {
				ptrMissing = append(ptrMissing, p)
			}
		}
		if vsPyStrip(txtVer) == "" {
			blockers = append(blockers, fmt.Sprintf("空转：%s 是空文件（既无数值也无指针 ⇒ 判据不可判）", vsPVersionFile))
		}
		if len(nums) > 0 {
			lineStrs := make([]string, len(numLines))
			for i, n := range numLines {
				lineStrs[i] = strconv.Itoa(n)
			}
			problems = append(problems, fmt.Sprintf("FAIL %s:%s 出现版本数值 %s —— 本文件已**指针化**，不许再写数值",
				vsPVersionFile, strings.Join(lineStrs, ","), strings.Join(vsUniqueSorted(nums), ", ")))
		}
		if vsPyStrip(txtVer) != "" && len(ptrMissing) > 0 {
			problems = append(problems, fmt.Sprintf("FAIL %s 的指针文本没点名真源：缺 %s（指针必须指出真源在哪）",
				vsPVersionFile, strings.Join(ptrMissing, " / ")))
		}
		hasNumLine := len(numLines) > 0
		ln := 0
		if hasNumLine {
			ln = numLines[0]
		}
		sources = append(sources, vsSource{id: "version_file", role: "指针（非真源）", path: vsPVersionFile,
			line: ln, hasLine: hasNumLine,
			ok:   len(nums) == 0 && len(ptrMissing) == 0 && vsPyStrip(txtVer) != "",
			note: fmt.Sprintf("无数值行=%s · 指向真源=%s", vsBool(len(nums) == 0), vsBool(len(ptrMissing) == 0)), hasNote: true})
	} else {
		sources = append(sources, vsSource{id: "version_file", role: "指针（非真源）", path: vsPVersionFile, ok: false})
	}

	for _, spec := range []struct{ sid, path, label string }{
		{"ui", vsPUi, "UI 侧（egui 桌面端）"},
		{"wall", vsPWall, "茧壁 crate（升级比较用）"},
	} {
		txt, ok := need(spec.path)
		if !ok {
			sources = append(sources, vsSource{id: spec.sid, role: spec.label, path: spec.path, ok: false})
			continue
		}
		val, ln, found := vsParseCargoPkgVersion(txt)
		if !found {
			blockers = append(blockers, fmt.Sprintf("不可解析：%s 的 [package] 段里没有 version（判据不可判）", spec.path))
		}
		sources = append(sources, vsSource{id: spec.sid, role: spec.label, path: spec.path,
			line: ln, hasLine: found, value: val, hasValue: found, ok: found && val != "",
			note: "取自 [package] 段", hasNote: true})
	}

	if canonical != "" {
		base := sources[0]
		for i := range sources {
			s := &sources[i]
			if s.id == "go" || !s.hasValue {
				continue
			}
			if s.value != canonical {
				lineTxt := ""
				if s.hasLine {
					lineTxt = strconv.Itoa(s.line)
				}
				baseLine := ""
				if base.hasLine {
					baseLine = strconv.Itoa(base.line)
				}
				problems = append(problems, fmt.Sprintf("FAIL %s:%s version = %s ≠ 基准 %s（基准真源 = %s:%s 的 `const Version`）—— 四处版本源（version.go / ui / wall）必须同版",
					s.path, lineTxt, s.value, canonical, base.path, baseLine))
				s.ok = false
			}
		}
	}

	var docs *vsDocs
	if len(staleSet) > 0 {
		docs = vsScanLiveDocs(cwd, root, staleSet, canonical)
		if docs.scannedFiles == 0 {
			blockers = append(blockers, "空转：活文档语料 0 个文件被扫到（旧版集合非空 ⇒ 判据不可判，不给结论）")
		}
		if strictDocs && docs.hits > 0 {
			problems = append(problems, fmt.Sprintf("FAIL --strict-docs：活文档里仍有 %d 处旧版本号（%d 个文件）", docs.hits, docs.hitFiles))
		}
	}

	// ⑥ R6 运行时默认根（目录）点名覆盖（阻断档：未点名覆盖 = rc=1；名册空 = rc=2）
	drProblems, drBlockers, drEntries := vsCheckDefaultRoots(root, roster, relocated)
	problems = append(problems, drProblems...)
	blockers = append(blockers, drBlockers...)

	// ⑦ R7 派生默认层版本位点登记册（GAP-20260927-89）
	ddProblems, ddBlockers, ddEntries := vsCheckDerivedDefaults(root, canonical, nil)
	problems = append(problems, ddProblems...)
	blockers = append(blockers, ddBlockers...)

	rc := vsRCOK
	if len(problems) > 0 {
		rc = vsRCFail
	} else if len(blockers) > 0 {
		rc = vsRCBlocked
	}
	if problems == nil {
		problems = []string{}
	}
	if blockers == nil {
		blockers = []string{}
	}
	return &vsResult{
		root: vsPyAbspath(cwd, root), canonical: canonical, hasCanonical: canonical != "",
		sources: sources, problems: problems, blockers: blockers, docs: docs,
		defEntries: drEntries, ddEntries: ddEntries, rc: rc,
	}
}

func vsBool(b bool) string {
	if b {
		return "True"
	}
	return "False"
}

// ─────────────────────────── 输出 ───────────────────────────

func vsPad(s string, w int) string {
	n := utf8.RuneCountInString(s)
	if n >= w {
		return s
	}
	return s + strings.Repeat(" ", w-n)
}

func vsPadNum(n, w int) string {
	s := strconv.Itoa(n)
	if len(s) >= w {
		return s
	}
	return s + strings.Repeat(" ", w-len(s))
}

func vsPrintReport(w io.Writer, res *vsResult, strictDocs bool) int {
	fmt.Fprintf(w, "== 版本源一致性（门② %s %s）==\n", vsTool, vsToolVersion)
	fmt.Fprintf(w, "仓库根: %s\n", res.root)
	canon := "（未解出）"
	if res.hasCanonical {
		canon = res.canonical
	}
	fmt.Fprintf(w, "基准（真源）: %s 里 const Version = \"%s\"\n", vsPGo, canon)
	fmt.Fprintln(w)
	for _, s := range res.sources {
		mark := "✗"
		if s.ok {
			mark = "✓"
		}
		val := "—"
		if s.hasValue {
			val = s.value
		}
		loc := ""
		if s.hasLine {
			loc = fmt.Sprintf(":%d", s.line)
		}
		extra := ""
		if s.hasNote {
			extra = fmt.Sprintf("（%s）", s.note)
		}
		fmt.Fprintf(w, "  %s  %s %s %s%s\n", mark, vsPad(s.path+loc, 38), vsPad(val, 8), s.role, extra)
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "-- 运行时默认根（R6 · 点名覆盖：%d 条）--\n", len(res.defEntries))
	for _, e := range res.defEntries {
		mark := "✗"
		tail := e.state
		if e.state == "在盘" {
			mark = "✓"
		} else if e.state == "登记迁出" {
			mark = "✓"
			tail = "登记迁出 ⇒ " + e.landed
		}
		fmt.Fprintf(w, "  %s  %s  %s\n", mark, e.dir, tail)
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "-- 派生默认层（R7 · 位点登记册：%d 条）--\n", len(res.ddEntries))
	for _, e := range res.ddEntries {
		mark := "✗"
		if e.state == "命中" {
			mark = "✓"
		} else if e.state == "件缺·跳过" {
			mark = "·"
		}
		got := e.got
		if got == "" {
			got = "—"
		}
		fmt.Fprintf(w, "  %s  %s [%s] %s = %s（复查 %s）\n", mark, e.file, e.role, e.state, got, e.reviewDate)
	}
	fmt.Fprintln(w)
	if len(res.problems) > 0 {
		fmt.Fprintf(w, "问题（%d 条）：\n", len(res.problems))
		for _, p := range res.problems {
			fmt.Fprintf(w, "  %s\n", p)
		}
	} else {
		fmt.Fprint(w, "问题（0 条）：四处版本源同版 ✓\n")
	}
	if len(res.blockers) > 0 {
		fmt.Fprintf(w, "不给结论（%d 条）：\n", len(res.blockers))
		for _, b := range res.blockers {
			fmt.Fprintf(w, "  %s\n", b)
		}
	}
	d := res.docs
	fmt.Fprintln(w)
	if d == nil {
		fmt.Fprint(w, "-- 只报告：活文档里的旧版本号字面量 —— 未扫（旧版集合为空）--\n")
	} else {
		fmt.Fprintf(w, "-- 只报告：活文档里的旧版本号字面量（旧版 = %s）--\n", strings.Join(d.staleVersions, ", "))
		fmt.Fprintf(w, "  语料: %d 个文件（活文档域）· 实扫 %d 个 · 跳过 %d 个\n", d.corpusFiles, d.scannedFiles, len(d.skipped))
		fmt.Fprintf(w, "  命中: %d 个文件 / %d 处（裸数字 %d 处 · 版本名/路径形态 %d 处）\n", d.hitFiles, d.hits, d.bare, d.ref)
		strictTxt := "不计失败项、不影响退出码；判读由人做"
		if strictDocs {
			strictTxt = "（--strict-docs 已开：命中计入 rc=1）"
		}
		fmt.Fprintf(w, "  ★ 本项**只报告**：%s\n", strictTxt)
		if len(d.hitList) > 0 {
			type grp struct {
				path string
				hits []vsHit
			}
			var groups []*grp
			idx := map[string]*grp{}
			for _, h := range d.hitList {
				g := idx[h.path]
				if g == nil {
					g = &grp{path: h.path}
					idx[h.path] = g
					groups = append(groups, g)
				}
				g.hits = append(g.hits, h)
			}
			countBare := func(hs []vsHit) int {
				n := 0
				for _, h := range hs {
					if h.class == "BARE" {
						n++
					}
				}
				return n
			}
			sort.SliceStable(groups, func(i, j int) bool {
				bi, bj := countBare(groups[i].hits), countBare(groups[j].hits)
				if bi != bj {
					return bi > bj
				}
				if len(groups[i].hits) != len(groups[j].hits) {
					return len(groups[i].hits) > len(groups[j].hits)
				}
				return groups[i].path < groups[j].path
			})
			for _, g := range groups {
				fmt.Fprintf(w, "    %s  %d 处（裸数字 %d）\n", g.path, len(g.hits), countBare(g.hits))
				for _, h := range g.hits {
					if h.class == "BARE" {
						fmt.Fprintf(w, "       L%s [裸] %s\n", vsPadNum(h.line, 5), h.text)
					}
				}
			}
			fmt.Fprint(w, "    （版本名/路径形态的逐行清单用 --json 取；本表只展开需人判读的「裸数字」行）\n")
		}
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "结果: %s  rc=%d\n", vsRCName[res.rc], res.rc)
	return res.rc
}

type vsRulesExtra struct {
	canonical string
	stale     []string
	known     []string
	staleNote string
}

func vsPrintRules(w io.Writer, re *vsRulesExtra) {
	fmt.Fprintf(w, "== 规则表（%s %s）==\n", vsTool, vsToolVersion)
	fmt.Fprintln(w, vsRulesBlock1)
	fmt.Fprint(w, "[活文档语料的排除域]\n")
	for _, e := range vsDocExcludesFixed {
		fmt.Fprintf(w, "  ✗ %s %s\n", vsPad(e.pref, 34), e.why)
	}
	for _, e := range vsDocExcludesRule {
		mark := "✗"
		if strings.Contains(e.why, "不排除") {
			mark = "✓"
		}
		fmt.Fprintf(w, "  %s %s %s\n", mark, vsPad(e.pat, 34), e.why)
	}
	fmt.Fprintf(w, "  %s\n", vsDocExcludesNote)
	if re != nil {
		fmt.Fprint(w, "\n[本仓现算]\n")
		canon := "（未解出）"
		if re.canonical != "" {
			canon = re.canonical
		}
		k := strings.Join(re.known, ", ")
		if k == "" {
			k = "（无）"
		}
		st := strings.Join(re.stale, ", ")
		if st == "" {
			st = "（空）"
		}
		fmt.Fprintf(w, "  当前版（真源）= %s\n", canon)
		fmt.Fprintf(w, "  已知 3 段历史版本目录: %s\n", k)
		fmt.Fprintf(w, "  旧版集合: %s   —— %s\n", st, re.staleNote)
	}
	fmt.Fprintln(w, vsRulesBlock2)
}

// ─────────────────────────── 自检（成对负控）───────────────────────────

const vsFixGo = "package version\n\n// Version 当前发布版（不带前缀）。\nconst Version = \"%s\"\n\nconst Tag = \"v\" + Version\n"
const vsFixUI = "[package]\nname = \"zerg-ui\"\nversion = \"%s\"\nedition = \"2021\"\n\n[dependencies]\nserde = { version = \"1\", features = [\"derive\"] }\n"
const vsFixWall = "[package]\nname = \"zerg-wall\"\nversion = \"%s\"   # 锚到版本真源\nedition = \"2021\"\n"
const vsFixVersion = "# 虫族 Zerg 版本号\n# 本文件已非真源：真源见 core/internal/version/version.go 与 ui/Cargo.toml\n"
const vsFixUINoPkgVersion = "[dependencies]\nserde = { version = \"%s\", features = [\"derive\"] }\n"

func vsMkFixture(base, goV, uiV, wallV, versionText string) error {
	for _, d := range []string{"core/internal/version", "ui", "wall", "docs/zh", "docs/项目文档/v9.9.8", "Zerg-内部文档/01-设计"} {
		if err := os.MkdirAll(vsPyJoin(base, d), 0o755); err != nil {
			return err
		}
	}
	wr := func(rel, txt string) error {
		return os.WriteFile(vsPyJoin(base, rel), []byte(txt), 0o644)
	}
	if err := wr(vsPGo, fmt.Sprintf(vsFixGo, goV)); err != nil {
		return err
	}
	if err := wr(vsPUi, fmt.Sprintf(vsFixUI, uiV)); err != nil {
		return err
	}
	if err := wr(vsPWall, fmt.Sprintf(vsFixWall, wallV)); err != nil {
		return err
	}
	if err := wr(vsPVersionFile, versionText); err != nil {
		return err
	}
	if err := wr("docs/zh/活文档.md", "# 活文档\n当前版本 9.9.8 已发布。\n见 `docs/项目文档/v9.9.8/变更-v9.9.8.md`。\n"); err != nil {
		return err
	}
	if err := wr("docs/项目文档/v9.9.8/变更-v9.9.8.md", "旧账：9.9.8 于昨日发布。\n"); err != nil {
		return err
	}
	return wr("Zerg-内部文档/01-设计/旧文.md", "旧设计（已分家到工作树之外）：9.9.8。\n")
}

type vsCaseSpec struct {
	name   string
	wantRC int
	needle string
	fn     func() *vsResult
}

type vsCaseResult struct {
	name          string
	ok            bool
	detail        string
	wantRC, gotRC int
}

func vsHitPaths(res *vsResult) []string {
	var out []string
	if res.docs != nil {
		for _, h := range res.docs.hitList {
			out = append(out, h.path)
		}
	}
	return out
}

func vsContains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

func vsSelfTest() ([]vsCaseResult, int, int, string) {
	base, err := os.MkdirTemp("", "check-version-sources-selftest-")
	if err != nil {
		base = os.TempDir()
	}

	fresh := func() error {
		names, _ := vsListDir(base)
		for _, d := range names {
			p := vsPyJoin(base, d)
			if vsIsDir(p) {
				os.RemoveAll(p)
			} else {
				os.Remove(p)
			}
		}
		return vsMkFixture(base, "9.9.9", "9.9.9", "9.9.9", vsFixVersion)
	}
	patch := func(rel, old, new string) error {
		p := vsPyJoin(base, rel)
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		txt := string(b)
		if !strings.Contains(txt, old) {
			return fmt.Errorf("夹具锚点未命中：%s 里没有 %q", rel, old)
		}
		return os.WriteFile(p, []byte(strings.Replace(txt, old, new, 1)), 0o644)
	}
	rm := func(rel string) error { return os.Remove(vsPyJoin(base, rel)) }
	run := func(strict bool, stale []string) *vsResult {
		return vsCheckSources(base, base, stale, strict, nil, nil)
	}
	runR := func(strict bool, stale []string, roster []vsDefRootEnt, relocated map[string]vsRelocEnt) *vsResult {
		return vsCheckSources(base, base, stale, strict, roster, relocated)
	}

	var specs []*vsCaseSpec
	mk := func(name string, want int, needle string, fn func() *vsResult) {
		specs = append(specs, &vsCaseSpec{name: name, wantRC: want, needle: needle, fn: fn})
	}

	mk("正控① 四处同版 9.9.9 + VERSION 纯指针 ⇒ 必绿（活文档命中只报告）", 0, "", func() *vsResult {
		_ = fresh()
		return run(false, []string{"9.9.8"})
	})
	mk("负控② version.go 改一个数 ⇒ 必红", 1, vsPGo, func() *vsResult {
		_ = fresh()
		_ = patch(vsPGo, "const Version = \"9.9.9\"", "const Version = \"9.9.8\"")
		return run(false, []string{"9.9.8"})
	})
	mk("负控③ ui/Cargo.toml 改一个数 ⇒ 必红", 1, vsPUi, func() *vsResult {
		_ = fresh()
		_ = patch(vsPUi, "version = \"9.9.9\"\nedition", "version = \"9.9.8\"\nedition")
		return run(false, []string{"9.9.8"})
	})
	mk("负控④ wall/Cargo.toml 改一个数 ⇒ 必红", 1, vsPWall, func() *vsResult {
		_ = fresh()
		_ = patch(vsPWall, "version = \"9.9.9\"", "version = \"9.9.8\"")
		return run(false, []string{"9.9.8"})
	})
	mk("负控⑤ VERSION 里又出现数值行 ⇒ 必红", 1, vsPVersionFile, func() *vsResult {
		_ = fresh()
		f, e := os.OpenFile(vsPyJoin(base, vsPVersionFile), os.O_APPEND|os.O_WRONLY, 0o644)
		if e == nil {
			_, _ = f.WriteString("9.9.9\n")
			f.Close()
		}
		return run(false, []string{"9.9.8"})
	})
	mk("负控⑥ VERSION 指针不再点名真源 ⇒ 必红", 1, "没点名真源", func() *vsResult {
		_ = fresh()
		_ = os.WriteFile(vsPyJoin(base, vsPVersionFile), []byte("# 版本号见某处\n"), 0o644)
		return run(false, []string{"9.9.8"})
	})

	for _, spec := range []struct {
		rel  string
		name string
	}{
		{vsPVersionFile, "负控⑦ 缺 VERSION ⇒ rc=2"},
		{vsPGo, "负控⑧ 缺 version.go ⇒ rc=2"},
		{vsPUi, "负控⑨ 缺 ui/Cargo.toml ⇒ rc=2"},
		{vsPWall, "负控⑩ 缺 wall/Cargo.toml ⇒ rc=2"},
	} {
		rel := spec.rel
		mk(spec.name, 2, "缺件", func() *vsResult {
			_ = fresh()
			_ = rm(rel)
			return run(false, []string{"9.9.8"})
		})
	}

	mk("负控⑪ Cargo.toml 的 [package] 段无 version（只有依赖行有）⇒ rc=2（不许回落猜）", 2, "不可解析", func() *vsResult {
		_ = fresh()
		_ = os.WriteFile(vsPyJoin(base, vsPUi), []byte(fmt.Sprintf(vsFixUINoPkgVersion, "9.9.9")), 0o644)
		return run(false, []string{"9.9.8"})
	})
	mk("负控⑫ VERSION 是空文件 ⇒ rc=2（空转/不可判）", 2, "空文件", func() *vsResult {
		_ = fresh()
		_ = os.WriteFile(vsPyJoin(base, vsPVersionFile), []byte(""), 0o644)
		return run(false, []string{"9.9.8"})
	})
	mk("负控⑬ 活文档语料 0 个文件被扫到 ⇒ rc=2（空转不许当绿）", 2, "空转", func() *vsResult {
		_ = fresh()
		_ = os.Remove(vsPyJoin(base, "docs", "zh", "活文档.md"))
		return run(false, []string{"9.9.8"})
	})
	mk("成对⑭ 活文档写着旧版号 ⇒ 默认**只报告**（rc 仍 0，但清单里有它）", 0, "__DOC_HIT__", func() *vsResult {
		_ = fresh()
		return run(false, []string{"9.9.8"})
	})
	mk("成对⑮ 同一夹具 + --strict-docs ⇒ rc=1（加严档有牙齿）", 1, "--strict-docs", func() *vsResult {
		_ = fresh()
		return run(true, []string{"9.9.8"})
	})
	mk("成对⑯ 冻结节/排除域里的旧版号**不入清单**（排除域生效）", 0, "__EXCLUDE_ONLY__", func() *vsResult {
		_ = fresh()
		return run(false, []string{"9.9.8"})
	})

	mk("负控⑰ R6：默认根目录不在盘且未登记迁出 ⇒ 必红（静默兜底未点名覆盖）", 1, "未登记迁出", func() *vsResult {
		_ = fresh()
		_ = os.RemoveAll(vsPyJoin(base, "docs", "项目文档"))
		return runR(false, []string{"9.9.8"}, nil, map[string]vsRelocEnt{})
	})
	mk("负控⑱ R6：登记迁出但落点不在盘 ⇒ 必红（登记指向死人）", 1, "落点", func() *vsResult {
		_ = fresh()
		_ = os.RemoveAll(vsPyJoin(base, "docs", "项目文档"))
		return runR(false, []string{"9.9.8"}, nil, map[string]vsRelocEnt{"docs/项目文档": {"迁出", "no-such-landed-dir"}})
	})
	mk("负控⑲ R6：名册漏点真源常量（快照根）⇒ 必红（名册式点名不成立）", 1, "名册漏点", func() *vsResult {
		_ = fresh()
		return runR(false, []string{"9.9.8"}, []vsDefRootEnt{{"repo-root", "r", []string{"core", "ui"}, nil}}, nil)
	})
	mk("负控⑳ R6：运行时默认根名册为空 ⇒ rc=2（空转不给结论）", 2, "空转", func() *vsResult {
		_ = fresh()
		return runR(false, []string{"9.9.8"}, []vsDefRootEnt{}, nil)
	})

	results := make([]vsCaseResult, 0, len(specs))
	nPass, nTotal := 0, len(specs)
	for _, c := range specs {
		res := c.fn()
		got := res.rc
		ok := got == c.wantRC
		detail := fmt.Sprintf("rc=%d（期望 %d）", got, c.wantRC)
		switch {
		case ok && c.needle == "__DOC_HIT__":
			paths := vsHitPaths(res)
			in := vsContains(paths, "docs/zh/活文档.md")
			ok = in
			detail += fmt.Sprintf(" · 清单含活文档=%s", vsBool(in))
		case ok && c.needle == "__EXCLUDE_ONLY__":
			paths := vsHitPaths(res)
			prefixes := []string{}
			for _, e := range vsDocExcludesFixed {
				prefixes = append(prefixes, e.pref)
			}
			prefixes = append(prefixes, "docs/项目文档/")
			bad := false
			for _, p := range paths {
				for _, x := range prefixes {
					if strings.HasPrefix(p, x) {
						bad = true
					}
				}
			}
			var vac []string
			for _, pfx := range prefixes {
				n := 0
				vsWalkPy(vsPyJoin(base, pfx), func(_ string, fns []string) { n += len(fns) })
				if n == 0 {
					vac = append(vac, pfx)
				}
			}
			in := vsContains(paths, "docs/zh/活文档.md")
			ok = !bad && in && len(vac) == 0
			suffix := ""
			if len(vac) > 0 {
				suffix = fmt.Sprintf("（空转域：%s）", strings.Join(vac, ", "))
			}
			detail += fmt.Sprintf(" · 排除域零命中=%s · 夹具覆盖各排除域=%s%s", vsBool(!bad), vsBool(len(vac) == 0), suffix)
		case ok && c.needle != "":
			blob := strings.Join(append(append([]string{}, res.problems...), res.blockers...), "\n")
			in := strings.Contains(blob, c.needle)
			ok = in
			detail += fmt.Sprintf(" · 判词含「%s」=%s", c.needle, vsBool(in))
		}
		results = append(results, vsCaseResult{name: c.name, ok: ok, detail: detail, wantRC: c.wantRC, gotRC: got})
		if ok {
			nPass++
		}
	}
	return results, nPass, nTotal, base
}

func vsPrintSelfTest(w io.Writer, cases []vsCaseResult, nPass, nTotal int) int {
	fmt.Fprintf(w, "== 成对负控自检（%s %s）==\n", vsTool, vsToolVersion)
	fmt.Fprint(w, "夹具：临时目录里的合成四源 + 活文档/冻结节（**不碰真仓**）\n\n")
	for _, r := range cases {
		mark := "FAIL"
		if r.ok {
			mark = "PASS"
		}
		fmt.Fprintf(w, "  %s %s\n      %s\n", mark, r.name, r.detail)
	}
	greens, reds := 0, 0
	for _, c := range cases {
		if c.wantRC == 0 {
			greens++
		} else {
			reds++
		}
	}
	fmt.Fprintf(w, "\n  区分度：好件/只报告格 %d 个（期望绿）↔ 负控格 %d 个（期望非绿）—— 好件绿而每个负控非绿才叫有牙齿\n", greens, reds)
	resWord := "不通过"
	rcOut := 1
	if nPass == nTotal {
		resWord = "通过"
		rcOut = 0
	}
	fmt.Fprintf(w, "\n结果: %s（%d/%d 条断言通过）  rc=%d\n", resWord, nPass, nTotal, rcOut)
	if nPass == nTotal {
		return vsRCOK
	}
	return vsRCFail
}

// ─────────────────────────── 入口 ───────────────────────────

func vsRepoRoot(cwd string, arg *string) string {
	if arg != nil && *arg != "" {
		return vsPyAbspath(cwd, *arg)
	}
	d := vsPyAbspath(cwd, cwd)
	for k := 0; k < 6; k++ {
		all := true
		for _, m := range vsRepoRootMarkers {
			if !vsIsDir(vsPyJoin(d, m)) {
				all = false
			}
		}
		if all {
			return d
		}
		d = vsPyDirname(d)
	}
	return cwd
}

func checkVersionSourcesGateMain(stdout, stderr io.Writer, argv []string) int {
	cwd, _ := os.Getwd()

	var rootArg *string
	jsonOut, listRules, selfTest, strictDocs, staleAll, help := false, false, false, false, false, false
	var staleVals []string

	usageErr := func() int {
		fmt.Fprint(stderr, "✗ 用法错（参数不认识）⇒ rc=2（不给结论）\n")
		return vsRCBlocked
	}

	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "--json":
			jsonOut = true
		case a == "--list-rules":
			listRules = true
		case a == "--self-test":
			selfTest = true
		case a == "--strict-docs":
			strictDocs = true
		case a == "--stale-all":
			staleAll = true
		case a == "-h" || a == "--help":
			help = true
		case a == "--stale":
			if i+1 >= len(argv) {
				return usageErr()
			}
			i++
			staleVals = append(staleVals, argv[i])
		case strings.HasPrefix(a, "--stale="):
			staleVals = append(staleVals, a[len("--stale="):])
		case strings.HasPrefix(a, "-") && a != "-":
			return usageErr()
		default:
			if rootArg != nil {
				return usageErr()
			}
			s := a
			rootArg = &s
		}
	}

	if help {
		fmt.Fprint(stdout, vsDocText+"\n")
		return vsRCOK
	}

	root := vsRepoRoot(cwd, rootArg)

	_, known, _ := vsDeriveStaleSet(cwd, root, "", staleAll)

	canonical := ""
	goPath := vsPyJoin(root, vsPGo)
	if vsExists(goPath) {
		if t, err := vsReadText(goPath); err == "" && t != "" {
			if v, _, found := vsParseGoConst(t); found {
				canonical = v
			}
		}
	}

	var stale []string
	staleNote := ""
	if len(staleVals) > 0 {
		stale = append([]string{}, staleVals...)
		staleNote = "旧版集合由 --stale 指定"
	} else {
		stale, known, staleNote = vsDeriveStaleSet(cwd, root, canonical, staleAll)
	}

	if selfTest {
		return vsRunSelfTest(stdout, jsonOut)
	}

	if listRules {
		vsPrintRules(stdout, &vsRulesExtra{canonical: canonical, stale: stale, known: known, staleNote: staleNote})
		return vsRCOK
	}

	d, dsrc := vsDocsSnapshotRoot(cwd, root)
	if d == "" {
		fmt.Fprintf(stdout, "✗ %s\n", dsrc)
		for _, rel := range vsDocsRelCandidates {
			fmt.Fprintf(stdout, "    缺件路径：%s（解析 = %s）\n", rel, vsPyAbspath(cwd, vsPyJoin(root, rel)))
		}
		fmt.Fprint(stdout, "  口径：旧版集合判据不可判 ⇒ rc=2（不给结论；不假绿、也不空转成绿）\n")
		return vsRCBlocked
	}

	res := vsCheckSources(cwd, root, stale, strictDocs, nil, nil)
	if jsonOut {
		obj := res.toJSON()
		obj.set("stale_note", staleNote)
		fmt.Fprintln(stdout, vsJSONString(obj))
		return res.rc
	}
	return vsPrintReport(stdout, res, strictDocs)
}

func vsRunSelfTest(stdout io.Writer, jsonOut bool) int {
	cases, nPass, nTotal, base := vsSelfTest()
	defer os.RemoveAll(base)
	if jsonOut {
		obj := &vsJobj{}
		obj.set("tool", vsTool)
		obj.set("tool_version", vsToolVersion)
		obj.set("mode", "self-test")
		obj.set("pass", nPass)
		obj.set("total", nTotal)
		arr := make([]vsJval, 0, len(cases))
		for _, c := range cases {
			co := &vsJobj{}
			co.set("case", c.name)
			co.set("ok", c.ok)
			co.set("detail", c.detail)
			co.set("want_rc", c.wantRC)
			co.set("got_rc", c.gotRC)
			arr = append(arr, co)
		}
		obj.set("cases", arr)
		fmt.Fprintln(stdout, vsJSONString(obj))
		return vsRCOK
	}
	return vsPrintSelfTest(stdout, cases, nPass, nTotal)
}

// ─────────────────────────── JSON（复刻 py json.dumps(ensure_ascii=False, indent=2)）───────────────────────────

type vsJval interface{}

type vsJobj struct {
	k []string
	v []vsJval
}

func (o *vsJobj) set(key string, val vsJval) {
	o.k = append(o.k, key)
	o.v = append(o.v, val)
}

func vsWriteIndent(b *strings.Builder, n int) {
	for i := 0; i < n; i++ {
		b.WriteByte(' ')
	}
}

func vsJSONString(v vsJval) string {
	var b strings.Builder
	vsJSONEncode(&b, v, 0)
	return b.String()
}

func vsJSONEncode(b *strings.Builder, v vsJval, ind int) {
	switch x := v.(type) {
	case nil:
		b.WriteString("null")
	case bool:
		if x {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	case int:
		b.WriteString(strconv.Itoa(x))
	case string:
		vsJSONStr(b, x)
	case *vsJobj:
		if len(x.k) == 0 {
			b.WriteString("{}")
			return
		}
		b.WriteString("{\n")
		for i := range x.k {
			if i > 0 {
				b.WriteString(",\n")
			}
			vsWriteIndent(b, ind+2)
			vsJSONStr(b, x.k[i])
			b.WriteString(": ")
			vsJSONEncode(b, x.v[i], ind+2)
		}
		b.WriteString("\n")
		vsWriteIndent(b, ind)
		b.WriteString("}")
	case []vsJval:
		if len(x) == 0 {
			b.WriteString("[]")
			return
		}
		b.WriteString("[\n")
		for i := range x {
			if i > 0 {
				b.WriteString(",\n")
			}
			vsWriteIndent(b, ind+2)
			vsJSONEncode(b, x[i], ind+2)
		}
		b.WriteString("\n")
		vsWriteIndent(b, ind)
		b.WriteString("]")
	default:
		b.WriteString("null")
	}
}

func vsJSONStr(b *strings.Builder, s string) {
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString("\\\"")
		case '\\':
			b.WriteString("\\\\")
		case '\n':
			b.WriteString("\\n")
		case '\r':
			b.WriteString("\\r")
		case '\t':
			b.WriteString("\\t")
		case '\b':
			b.WriteString("\\b")
		case '\f':
			b.WriteString("\\f")
		default:
			if r < 0x20 {
				b.WriteString(fmt.Sprintf("\\u%04x", r))
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
}

func (r *vsResult) toJSON() *vsJobj {
	obj := &vsJobj{}
	obj.set("tool", vsTool)
	obj.set("tool_version", vsToolVersion)
	obj.set("root", r.root)
	if r.hasCanonical {
		obj.set("canonical", r.canonical)
	} else {
		obj.set("canonical", nil)
	}
	srcArr := make([]vsJval, 0, len(r.sources))
	for _, s := range r.sources {
		so := &vsJobj{}
		so.set("id", s.id)
		so.set("role", s.role)
		so.set("path", s.path)
		if s.hasLine {
			so.set("line", s.line)
		} else {
			so.set("line", nil)
		}
		if s.hasValue {
			so.set("value", s.value)
		} else {
			so.set("value", nil)
		}
		so.set("ok", s.ok)
		if s.hasNote {
			so.set("note", s.note)
		}
		srcArr = append(srcArr, so)
	}
	obj.set("sources", srcArr)
	pArr := make([]vsJval, 0, len(r.problems))
	for _, p := range r.problems {
		pArr = append(pArr, p)
	}
	obj.set("problems", pArr)
	bArr := make([]vsJval, 0, len(r.blockers))
	for _, b := range r.blockers {
		bArr = append(bArr, b)
	}
	obj.set("blockers", bArr)
	if r.docs == nil {
		obj.set("docs", nil)
	} else {
		d := &vsJobj{}
		sv := make([]vsJval, 0, len(r.docs.staleVersions))
		for _, s := range r.docs.staleVersions {
			sv = append(sv, s)
		}
		d.set("stale_versions", sv)
		t := &vsJobj{}
		t.set("corpus_files", r.docs.corpusFiles)
		t.set("scanned_files", r.docs.scannedFiles)
		t.set("hit_files", r.docs.hitFiles)
		t.set("hits", r.docs.hits)
		t.set("bare", r.docs.bare)
		t.set("ref", r.docs.ref)
		d.set("totals", t)
		hl := make([]vsJval, 0, len(r.docs.hitList))
		for _, h := range r.docs.hitList {
			ho := &vsJobj{}
			ho.set("path", h.path)
			ho.set("line", h.line)
			ho.set("stale", h.stale)
			ho.set("class", h.class)
			ho.set("text", h.text)
			hl = append(hl, ho)
		}
		d.set("hits", hl)
		sk := make([]vsJval, 0, len(r.docs.skipped))
		for _, s := range r.docs.skipped {
			so := &vsJobj{}
			so.set("path", s.path)
			so.set("why", s.why)
			sk = append(sk, so)
		}
		d.set("skipped", sk)
		eo := &vsJobj{}
		if r.docs.excluded != nil {
			for i := range r.docs.excluded.keys {
				eo.set(r.docs.excluded.keys[i], r.docs.excluded.vals[i])
			}
		}
		d.set("excluded_dirs", eo)
		obj.set("docs", d)
	}
	drObj := &vsJobj{}
	drEnt := make([]vsJval, 0, len(r.defEntries))
	for _, e := range r.defEntries {
		eo := &vsJobj{}
		eo.set("root", e.root)
		eo.set("dir", e.dir)
		eo.set("state", e.state)
		if e.hasLanded {
			eo.set("landed", e.landed)
		} else {
			eo.set("landed", nil)
		}
		drEnt = append(drEnt, eo)
	}
	drObj.set("entries", drEnt)
	obj.set("default_roots", drObj)
	ddObj := &vsJobj{}
	ddEnt := make([]vsJval, 0, len(r.ddEntries))
	for _, e := range r.ddEntries {
		eo := &vsJobj{}
		eo.set("file", e.file)
		eo.set("role", e.role)
		eo.set("state", e.state)
		eo.set("got", e.got)
		eo.set("want", e.want)
		eo.set("note", e.note)
		eo.set("review_date", e.reviewDate)
		ddEnt = append(ddEnt, eo)
	}
	ddObj.set("entries", ddEnt)
	obj.set("derived_defaults", ddObj)
	obj.set("rc", r.rc)
	return obj
}
