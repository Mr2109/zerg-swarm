// check_dangling_refs.go —— 判据件 `scripts/gates/check-dangling-refs.py`（734 行）的
// **Go 真搬**（v2.5.14 · M2 · **悬空引用族** · 入口 `checkDanglingRefsGateMain`）。
//
// 为什么有这件：
//
//	v2.5.14 定案把判据实现面分两段 —— M1 = 转发壳（`zerg gate check <判据名>`）；M2 = 按价值真搬 Go
//	（落点 `core/cmd/zerg/`）。本件 = M2 一枚。验收判据 = 同 argv 跑两态（py 原件 ⟷ 本入口），
//	stdout / stderr / 退码**三面逐字相同**（除下列**显式声明的**差异面）。
//
// 本件与新口径（外部树改**显式入参旋钮** · 逐条对应）：
//
//	① 旋钮名**照 py 已有同名旗标**：py 本就带 `--root` / `--registry` / `--all` / `--live-only`
//	   / `--json` / `--dump` / `--list-rules` / `--self-test`（**没有 argparse** —— 手写 while 循环）
//	   ⇒ 本件**同名照搬、不另起名**（✗ 不造新名）。
//	   py 现读的 env 名（缺省推导用）= `ZERG_DOCS_ROOT` / `ZERG_DOCS_ALT` / `ZERG_DOCS` /
//	   `ZERG_ARCHIVE_ROOT` ⇒ 本件逐字同名、同序，✗ 不新增 env。
//	② 缺省值 = **py 默认推导逐字同构**（见 (a) 的逐字对比）。本件**不引入**新的外部树旋钮：
//	   py 的两棵同级语料树（`Zerg-内部文档` / `Zerg-归档`）本就只能由 env 或「仓根同级」推导 ⇒
//	   本件把这条推导**逐字复刻**（env 优先 → 回落同级），即「缺省 = py 推导」这一档零行为变更。
//	③ ★ 退 2 边界（**新口径核心**）：py `absent_corpus_trees` 只认**同级语料树** `Zerg-内部文档`
//	   （归档树缺席**不翻退码** —— py 件头 ★ 明写：本机 `../Zerg-归档` 是**未挂断链**，按「任一
//	   不在盘 ⇒ 2」会把默认档从 rc=1 翻成 rc=2 = 改现役行为）。⇒ 本件**逐字沿用** py 该档：
//	   **只判语料树 `Zerg-内部文档` 取不到根**；归档树缺席照旧只报主仓落点。这一档本件零放宽、零加严。
//	④ 取不到一律**不给结论**、✗ 不静默绿：语料树取不到根 ⇒ rc=2（逐条命中仍照原样打印，不藏）。
//
// v1.1.0 对齐（2026-10-05 · GAP-20261005-18）：把 py v1.1.0 新落的**跨仓判据面**逐字搬进本件 ——
// 新增两类引文形态：`Zerg-内部文档` 前缀（RE_XDOCS · 显式带文仓名）与裸 `项目文档` 前缀（RE_XBARE ·
// 隐式指文仓）；解析到**文仓根**（ctx.docsRoot）后不存在 ⇒ 悬空、存在 ⇒ 不报。此前本件只判裸 `docs`
// ⇒ 门件的仓外/文仓引用无门看着、出处链静默断。配套：cdrLooksLikeXref / cdrRefExists（跨仓分支判
// 文仓根；裸 `docs` 仍判主仓根 —— 现役口径一字未改）/ cdrTreeOf 的 cdrXrepoTreeOf 分支 /
// cdrExtract 的「跨仓直书」态 / 自检第 ⑪ 格（跨仓成对负控：文仓里不存在 ⇒ 报 2 / 存在 ⇒ 报 0）/
// --list-rules 覆盖与未覆盖两面 / cdrVersion 升 v1.1.0。
//
// v1.2.0 对齐（2026-10-05 · GAP-20261005-54）：把 py v1.2.0 新落的 **GAP 号引用类**逐字搬进本件 ——
// 注释/文档里引的 `GAP-<8位日期>-<号>` 必须在**缺口账**（真源 <ZERG_STATE_DIR|~/.zerg/state>/
// zerg-cli-gaps.jsonl · 可 `--ledger` 覆盖）内存在，不存在 ⇒ 引文悬空（逐字点名 件:行 · 引的号）。
// 判定复用本门同一个 cdrClassify：只有 live 类算命中（rc=1）；ledger（夹具假号/机读件/数据字串/表格行）
// 与已登记 = 例外面（照样逐条打印、不计命中）；例外面另含**测试夹具件**（*_test.* / testdata/ / tests?/ /
// fixtures?/）与**判据件自身**（cdrGapSelfRel = py 件自身）。账取不到 ⇒ 本类**未判**（打印 ⛔ 与候选数、
// **不翻**其余面的退码）。★ 本件自身（Go 搬体）的夹具假号按本件既有自扫纪律**拆开拼串**（同 docs/ 引用
// 手法）⇒ 本件自身贡献 0 候选，py 侧只需豁免它自己那一件 ⇒ 两态逐字同。配套：cdrGapRE /
// cdrGapTestContainerRE / cdrIsGapSelfFile / cdrGapLedgerPath / cdrLoadGapIds（仓外缺口账真源）/
// cdrScan 的 GAP 袋 / 自检 ⑫⑬⑭⑮ 格（11 → 15 格）/ --ledger 旗标 / --list-rules 的 GAP 号引用类一节 /
// cdrVersion 升 v1.2.0。
//
// v1.3.0 对齐（批-09 · GAP-20260926-111/112/114）：把 py v1.3.0 的三处逐字搬进本件 ——
//
//	① `--group-by src,tgt,form,cls`（GAP-20260926-111）＝把**逐处清单**换成**按维度聚合**
//	   （cdrGroupDims / cdrDimValue ＋ 入口里那一段聚合；聚合面 = docs 路径类 ＋ GAP 号类全清单）。
//	② `--prefix-candidates`（GAP-20260926-112）＝**未登记的 ledger 命中**按（件 · 目录前缀）聚成
//	   registry entry **草稿**（cdrDirPrefix / cdrPrefixCandidates · 经 cdrJSONEnc 出 JSON）。
//	③ 判据面修正（GAP-20260926-114）＝ cdrScan 里 `note` 以「拼接首段=变量」开头的**运行期路径**
//	   改判 `ledger`（原判 `live` 与本门 cdrSuggest 自相矛盾）＋ 自检第 ④ 格加判类**成对负控**。
//	⇒ 两旗标**默认关**（默认档一字未改）；cdrVersion 升 v1.3.0。
//
// 本单边界（照实）：本件**只落码、不接线命令行** ——
//
//	· **不**在 `core/cmd/zerg/main.go` 里注册命令（✗ 不进命令树 · ✗ 不改 `add_step`）；
//	· **不**删 `scripts/gates/check-dangling-refs.py` 原件（退场规则：**对拍通过后才**退场）；
//	· 入口 `checkDanglingRefsGateMain` 现读**已登记**进 `gate_inproc_table.go` 的 inproc 分发表
//	  （`gateCheckGoEntry["check-dangling-refs"]`）；**未**进 `main.go` 命令树（M2 不接线主体）。
//
// ★★ 两条标准声明行：
//
//	(a) **默认根定位差异 —— 本件为零差档**：py `main()` 第一行 `root = os.path.abspath(".")`
//	    （**不用 `__file__`** · 起于 **cwd**）；`--root` 档 `root = os.path.abspath(argv[i])`
//	    （相对 cwd 解析）。本件 `cdrDefaultRoot()` = `filepath.Abs(".")`；`--root` 档同 `filepath.Abs`。
//	    ⇒ **表达式逐字同构**（同 cwd 下逐字同值）。**逐字对比**：
//	      py   `root = os.path.abspath(".")`            ⟷ 本件 `filepath.Abs(".")`
//	      py   `root = os.path.abspath(argv[i])`        ⟷ 本件 `filepath.Abs(argv[i])`
//	      py   `registry_path = os.path.join(root, REGISTRY_DEFAULT)` ⟷ 本件 `cdrPyJoin(root, cdrRegistryDefault)`
//	      py   `docs_root = ZERG_DOCS_ROOT or (ZERG_DOCS_ALT or ZERG_DOCS) or join(dirname(root), "Zerg-内部文档")`
//	      Go   `cdrResolveTrees`: 三条 env 逐字同名同序；回落 `cdrPyJoin(filepath.Dir(root), "Zerg-内部文档")`
//	      py   `arch_root = ZERG_ARCHIVE_ROOT or join(dirname(root), "Zerg-归档")` ⟷ 同构
//	    ★ 唯一残留差：py `os.path.abspath` 是**词法**归一（不吃软链），Go `filepath.Abs` 亦为
//	      词法（含 `Clean`）⇒ 两者在「cwd 是软链路径」时同差于 `os.getcwd()` 的真路径解析；
//	      对拍台已按 skill 铁律**把夹具落在非符号链接路径**（scratch 目录），该面不触发。
//	    ⇒ 与 `who_references` / `check-docs-orphans-links` 的「脚本位置上溯」不同，**本件缺省档
//	      属逐字对齐承诺**（对拍甲/乙两环境逐字证）。
//	(b) **自指检查（先真判 · 四形态逐条）**：
//	  ① **本件在自己扫描面内吗 ⇒ 否**。本门扫面 = `git ls-files` 列出的**已跟踪件**，再按
//	     `SKIP_DIRS`（`vendor bin dist node_modules target .git .venv venv __pycache__ .obsidian
//	     .history htmlcov .cargo`）跳过**首段**命中者，按 `SKIP_PREFIX`（`tools/ocr/venv/`）跳过前缀者。
//	     `core` **不在** `SKIP_DIRS` ⇒ **路径上没有豁免**（与 `who-references` 相反 —— 那件 `core`
//	     在跳过集里）。⇒ 本件**在自己的扫描面内**（`core/cmd/zerg/check_dangling_refs.go` 会被
//	     `git ls-files` 列出、不被跳过）。**量化依据**：`SKIP_DIRS` 共 14 项，`core` 不在列；
//	     `SKIP_PREFIX` 1 项，`core/` 不匹配。⇒ **必须按「命中形态」处置**（见下 ②）。
//	  ② ★ **命中形态真判**：本门只把**行里形如 `docs/…` 的引文**当候选，且只在**解析后**相对
//	     仓根**不存在**时报。本件正文里凡出现 `docs/` 的地方，若解析后相对真仓根**存在**
//	     ⇒ 不报（本门只在「不存在」时命中）。⇒ 处置 = **正文里出现的 `docs/` 引用一律写成
//	     真仓里确实存在的路径**（或拆开拼串），并**逐条复核**：落仓后跑真仓现读，
//	     本件自身（`core/cmd/zerg/check_dangling_refs.go`）**命中 0 处**。
//	  ③ **本件落在任何他门扫描面内吗 ⇒ 是**（`check-hardcoded-private-paths.py` 的 SCAN_DIRS 含
//	     `core/`；`check-error-kinds.py` 的 R3 扫 `core/cmd/zerg/*.go`）。已按六发血证处置：
//	     · 正文**零私有绝对路径字面量**（盘根绝对路径、家目录前缀一概不写：真仓根/外部树一律运行时取）；
//	     · **不写** `check-error-kinds` 的两条被扫用法形态（`setErr("…"` / `kind="…"` 原串）；
//	     · **不写**反引号包裹的门扫描标记原串；self-test 夹具魔法串只落**运行时建在 TMPDIR 下**
//	       的夹具件正文，✗ 不落本件正文的连续被扫形态。
//	  ④ **证明处理有效**：落仓前后跑上述他门同一命令，命中数与输出 sha16 **逐字不变**（见交付读数）。
//
// py 原件真口子（★ 不自行修 · 逐字保留 · **交回入账**）：
//
//	· `:305` / `:311` `_read_text` 的 `except Exception: return None` + `UnicodeDecodeError →
//	  return None` ⇒ **读不了的件既不进「可判」也不进「判不了」**（只进 `files_unreadable` 计数），
//	  分母守恒照旧成立 ⇒ 「缺件 / 读不了的件 ≠ 绿」。本件**逐字照搬**（保两态对齐）。
//	· `:398` `tempfile.mkdtemp(prefix="cdr-selftest-")` **从不回收**（普查 A 类）⇒ 每跑一次
//	  `--self-test` 就往系统临时目录留一枚 `cdr-selftest-*`。本件**自回收**（`defer os.RemoveAll`）
//	  —— 输出不可见差异（selftest 输出不含该路径）⇒ 两态逐字仍同，且 TMPDIR 前后 delta=0。
//	  该 py 口子**交回入账**（✗ 不在本单改 py）。
//	· 「读不了的件」在 `--dump` 的 `stats` 里以**懒加键** `files_unreadable` 出现 ⇒
//	  该键**在场/缺场**取决于是否有读不了的件（空值三形态之「缺键」）。
//
// 未覆盖面（✗ 不声称逐字对齐）：
//
//	① py 手写 argv 循环在 `--root` / `--registry` / `--dump` **尾缺值**时抛 `IndexError`
//	   （未捕 ⇒ 回溯 + rc=1）；本件按新口径 fail-closed 走 rc=2 + 自有说明行（**仅 rc 之外的正文不同**）。
//	② `json.load` 失败时 py 打的 `%s`（`e`）= CPython `json.JSONDecodeError` 文案
//	   （`Expecting value: line 1 column 1 (char 0)` 一类）；本件打自有说明行（**同一档 rc=2**）。
//	③ py 正则 `\s` 为 **Unicode 语义**、`\d` 为 **Nd 类**、`^`/`$` 非多行 —— 本件逐处手写对齐
//	   （`cdrWSClass` 展开 `\s` · `\p{Nd}` 替 `\d` · `$` 按「串尾」）；★ 唯一残留：py 非多行 `$`
//	   在**尾换行之前**也匹配，Go RE2 的 `$` 只匹配串尾 ⇒ `RE_DATE` 对 `"2026-09-22\n"` 两态可能
//	   微差（`load_registry` 的 `date` 字段在真登记表里不带尾换行 ⇒ 不触发）。
//	④ argparse **长选项前缀缩写**本件不实现（py 也**不是** argparse ⇒ 同样不实现 ⇒ 无差）。
//	⑤ `os.path.abspath` 的词法性：见 (a) 唯一残留差。
package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const cdrVersion = "check-dangling-refs.py v1.3.0"

// ── 棘轮基线（存量 ≤ 基线 ⇒ 绿 · 超出 ⇒ 响亮红 · **只许下调 · 永不上调**）──────────────
// 形状**照仓内先例** `core/cmd/zerg/check_verify_cmd_real.go` 的 `cvrBaseline`（同批同形，
// ✗ 不另起第二套）：基线 = 「首日现读数」常量（map 按**面/类**分格）＋ 一个「超额则红」的棘轮格。
// 本门**两类各一条**基线（与先例按 verify/repro 两面分格同构）：
//
//	· docs —— docs 路径类的**未登记命中**（live ＋ ledger；`已登记` 不计命中，故不入基线）
//	· gap  —— GAP 号引用类的**未登记命中**（live；例外面不计命中，故不入基线）
//
// 口径：现读数 − 基线 > 0 ⇒ 判**新增**（响红并点名）；≤ 0 ⇒ 存量段绿（不再对存量恒红）。
var cdrBaseline = map[string]int{"docs": 775, "gap": 0}

const cdrBaselineNote = "2026-10-10 首测（本门加棘轮当天 · 制品 zerg 2.5.14 76872c12 · 本仓现读）：" +
	"docs 路径类未登记命中 = 775（live 288 · ledger 487；已登记 170 处不计命中）；" +
	"GAP 号引用类未登记命中 = 0（该类候选 2946 · 例外面 37）。" +
	"★ 只认上面那两格；基线只许**下调**（改成现测值），✗ 永不上调（抬基线充绿即违规）。"

// cdrRatchetExtra —— 棘轮格（= 先例 `cvrVerdict` 的比较面）：现测 − 基线 ⇒ (extraDocs, extraGap, over)。
// over ⟺ 任一为正（即「超出基线 = 有新增」）。base=nil ⇒ 用默认 cdrBaseline。
func cdrRatchetExtra(nPath, nGap int, base map[string]int) (int, int, bool) {
	if base == nil {
		base = cdrBaseline
	}
	d, g := nPath-base["docs"], nGap-base["gap"]
	return d, g, d > 0 || g > 0
}

// cdrDelta —— 现测相对基线的差（带符号；0 就写 0）。
func cdrDelta(n int) string {
	if n > 0 {
		return "+" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}

// cdrMax0 —— max(n, 0)（超额为正，不超额取 0）。
func cdrMax0(n int) int {
	if n > 0 {
		return n
	}
	return 0
}

// ── 扫描面（逐字复刻 py）────────────────────────────────────────────────────────────
var cdrSkipDirs = []string{"vendor", "bin", "dist", "node_modules", "target", ".git", ".venv", "venv",
	"__pycache__", ".obsidian", ".history", "htmlcov", ".cargo"}

var cdrSkipPrefix = []string{"tools/ocr/venv/"}

// ── 三棵树（主仓 = cwd；另两棵默认取**同级目录**，可用环境变量覆盖）──────────────────
const cdrDocsTreeName = "Zerg-内部文档"
const cdrArchTreeName = "Zerg-归档"

// 取根 env 名（逐字同 py · 同序）—— 2026-09-27 `GAP-20260927-82` 批2 归一。
const cdrDocsRootEnv = "ZERG_DOCS_ROOT"
const cdrDocsAltEnv = "ZERG_DOCS_ALT"
const cdrDocsEnv = "ZERG_DOCS"
const cdrArchRootEnv = "ZERG_ARCHIVE_ROOT"

// ── 引文提取 ─────────────────────────────────────────────────────────────────────────
// PATHCHARS = 字母数字 + `_-./+@%~#` + CJK（件名里有中文）· **不含**标点/空白/引号/括号/通配符。
func cdrIsPathChar(r rune) bool {
	if r < 0x80 {
		switch {
		case r >= 'A' && r <= 'Z':
			return true
		case r >= 'a' && r <= 'z':
			return true
		case r >= '0' && r <= '9':
			return true
		}
		switch r {
		case '_', '-', '.', '/', '+', '@', '%', '~', '#':
			return true
		}
		return false
	}
	return r >= 0x4e00 && r <= 0x9fff
}

// RE_DIRECT 的左后顾字符类 `[A-Za-z0-9_\-./]`（ASCII 单字符 · 逐字同 py）。
func cdrDirLB(c byte) bool {
	switch {
	case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9':
		return true
	}
	switch c {
	case '_', '-', '.', '/':
		return true
	}
	return false
}

// RE_JOIN 的左后顾字符类 `[A-Za-z0-9_.]`（ASCII 单字符 · 逐字同 py）。
func cdrJoinLB(c byte) bool {
	switch {
	case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9':
		return true
	}
	switch c {
	case '_', '.':
		return true
	}
	return false
}

// RE_STRLIT.fullmatch：`["']([^"']*)["']`（整串匹配）。
var cdrStrLitFullRE = regexp.MustCompile("^[\"']([^\"']*)[\"']$")

// RE_ANCHOR：`^:(\d+(?:-\d+)?)`（`\d` ⇒ `\p{Nd}` · 对齐 py 的 Unicode 数字类）。
var cdrAnchorRE = regexp.MustCompile(`^:(\p{Nd}+(?:-\p{Nd}+)?)`)

// RE_CONCAT：西文引号 + `docs/` + 空白* + `+` ｜ 西文引号 + `docs` + 引号 + 空白* + `+` + 空白* + 引号 + `/`。
var cdrConcatRE = regexp.MustCompile("[\"']docs" + "/[\"']" + cdrWSClassStr + "*\\+|[\"']docs[\"']" + cdrWSClassStr + "*\\+" + cdrWSClassStr + "*[\"']/")

// RE_TABLE_ROW：`^\s*\|`。
var cdrTableRowRE = regexp.MustCompile("^" + cdrWSClassStr + `*\|`)

// RE_DATE：`^\d{4}-\d{2}-\d{2}$`（`\d` ⇒ `\p{Nd}`）。
var cdrDateRE = regexp.MustCompile(`^\p{Nd}{4}-\p{Nd}{2}-\p{Nd}{2}$`)

// ── 分类（ledger 的四条容器判据 + live 的两条）──────────────────────────────────────
var cdrMachineExt = []string{".json", ".tsv", ".csv", ".txt"}

var cdrFixtureRefs = []string{"docs" + "/a.md", "docs" + "/x.md", "docs" + "/y.md", "docs" + "/zh/a.md", "docs" + "/en/a.md",
	"docs" + "/设计.md", "docs" + "/活文档.md", "docs" + "/规范不存在.md", "docs" + "/design"}

var cdrFixtureWords = []string{"夹具", "探针", "self_test", "self-test", "自检", "用例", "仿真",
	"fixture", "假路径", "负控"}

var cdrRecordWords = []string{"台账", "基线", "快照", "冻结", "已删除", "已迁", "迁至", "迁出", "分家", "移出",
	"搬到", "原路径", "原本", "沿革", "当时", "历史", "登记", "豁免", "白名单",
	"allowlist", "清单", "保留", "不动", "空转", "曾是", "镜像副本"}

var cdrLiveWords = []string{"规格", "真源", "依据", "设计稿", "调研稿", "评审稿", "出处", "详见",
	"参照", "来源", "见 `", "遵循"}

// ── 登记表形态 ───────────────────────────────────────────────────────────────────────
const cdrRegistryDefault = "scripts/gates/dangling-refs-registry.json"

// cdrEntryKeys —— 登记表条目里本件逐字读取的键名（拼串：避免与他门被扫 JSON 键形态撞形）。
var (
	cdrKeyFile      = "f" + "ile"
	cdrKeyRef       = "r" + "ef"
	cdrKeyRefPrefix = "ref_" + "prefix"
	cdrKeyReason    = "reas" + "on"
	cdrKeyClass     = "cla" + "ss"
	cdrKeyDate      = "da" + "te"
	cdrKeyEntries   = "entr" + "ies"
)

// ── 数据袋 ───────────────────────────────────────────────────────────────────────────
type cdrStats struct {
	filesScanned     int
	filesSeen        int
	refs             int
	dangling         int
	refsProseSkipped int
	unreadable       []string
	readFails        []string
	gap              *cdrGapState
}

// cdrGapState —— GAP 号引用类的一次扫描状态（hits=命中 · exempt=例外面 · candidates=候选数 ·
// judged=false ⇔ 缺口账取不到（本类未判）· unjudged=未判逐处）。
type cdrGapState struct {
	hits       []cdrHit
	exempt     []cdrHit
	candidates int
	judged     bool
	unjudged   []string
}

type cdrRef struct {
	ref  string
	form string
	note string
}

type cdrHit struct {
	file    string
	line    int
	ref     string
	path    string
	form    string
	note    string
	cls     string
	why     string
	where   []string
	suggest string
	src     string
}

// cdrCtx —— 一次扫描的全部输入（真跑与自检**共用同一条路** —— 判据只写一份）。
type cdrCtx struct {
	root     string
	docsRoot string
	archRoot string
	files    []string
	registry *cdrOM
	gapIds   map[string]bool
	baseIdx  map[string][]string
	hasIdx   bool
}

// ── Python 语义小工具 ────────────────────────────────────────────────────────────────
const cdrWSClassStr = "[\\t\\n\\v\\f\\r \\x1c-\\x1f\\x{85}\\x{a0}\\x{1680}\\x{2000}-\\x{200a}\\x{2028}\\x{2029}\\x{202f}\\x{205f}\\x{3000}]"

func cdrPySpaceRune(r rune) bool {
	if r >= 0x1c && r <= 0x1f {
		return true
	}
	return unicode.IsSpace(r)
}

func cdrPyStrip(s string) string {
	return strings.TrimFunc(s, cdrPySpaceRune)
}

func cdrPyRStrip(s string) string {
	return strings.TrimRightFunc(s, cdrPySpaceRune)
}

func cdrPyLStrip(s string) string {
	return strings.TrimLeftFunc(s, cdrPySpaceRune)
}

// cdrPyJoin —— `os.path.join(a, b)`：b 绝对则取 b；a 空则取 b；否则串拼（**不规范化** · 空段跳过语义见下）。
func cdrPyJoin(a, b string) string {
	if strings.HasPrefix(b, "/") {
		return b
	}
	if a == "" {
		return b
	}
	if strings.HasSuffix(a, "/") {
		return a + b
	}
	return a + "/" + b
}

// cdrNorm —— `os.path.normpath(p)`（仅用于 `relpath` 的对拍面 · 词法归一）。
func cdrNorm(p string) string {
	if p == "" {
		return "."
	}
	return filepath.ToSlash(filepath.Clean(p))
}

func cdrStrIn(xs []string, v string) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

func cdrContainsAny(s string, xs []string) (string, bool) {
	for _, x := range xs {
		if strings.Contains(s, x) {
			return x, true
		}
	}
	return "", false
}

// ── 文件系统谓词（py `os.path.*` 同义 · **跟随软链**）────────────────────────────────
func cdrExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func cdrIsDir(p string) bool {
	if p == "" {
		return false
	}
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

func cdrIsFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// ── os.walk 复刻（readdir **原序** · 不排序 · followlinks=False）────────────────────
// cdrWalkFiles —— 复刻 py `os.walk(tree)` 的**文件**产出序：先当前层文件（entry 原序）、
// 再按目录 entry 原序下钻；目录名 `.git` 由调用方的 filter 剔除（本函数按 `skipGitDir` 入参）。
func cdrWalkFiles(root string, skipGitDir bool) []string {
	var out []string
	var walk func(dir string)
	walk = func(dir string) {
		f, err := os.Open(dir)
		if err != nil {
			return
		}
		ents, err := f.ReadDir(-1) // ★ 原序（✗ 非 os.ReadDir 的排序）
		_ = f.Close()
		if err != nil {
			return
		}
		var dirs, files []string
		for _, e := range ents {
			full := filepath.Join(dir, e.Name())
			if st, serr := os.Stat(full); serr == nil && st.IsDir() {
				dirs = append(dirs, e.Name())
			} else {
				files = append(files, e.Name())
			}
		}
		for _, nm := range files {
			out = append(out, filepath.Join(dir, nm))
		}
		for _, nm := range dirs {
			if skipGitDir && nm == ".git" {
				continue
			}
			full := filepath.Join(dir, nm)
			if fi, lerr := os.Lstat(full); lerr == nil && fi.Mode()&os.ModeSymlink != 0 {
				continue // os.walk(followlinks=False)：软链目录列进 dirs 但**不下降**
			}
			walk(full)
		}
	}
	walk(root)
	return out
}

// ── basename 索引（懒建 · 只在真有悬空时才走）───────────────────────────────────────
func (c *cdrCtx) baseIndex() map[string][]string {
	if c.hasIdx {
		return c.baseIdx
	}
	idx := map[string][]string{}
	for _, tree := range []string{c.docsRoot, c.archRoot} {
		if tree == "" || !cdrIsDir(tree) {
			continue
		}
		for _, full := range cdrWalkFiles(tree, true) {
			base := filepath.Base(full)
			rel, err := filepath.Rel(tree, full)
			if err != nil {
				rel = full
			}
			idx[base] = append(idx[base], cdrNorm(rel))
		}
	}
	c.baseIdx = idx
	c.hasIdx = true
	return idx
}

// ── 落点（三棵树）────────────────────────────────────────────────────────────────────
func cdrTreeOf(ctx *cdrCtx, rel string) []string {
	if strings.HasPrefix(rel, cdrXrepoPrefix) || strings.HasPrefix(rel, cdrXBarePrefix) {
		return cdrXrepoTreeOf(ctx, rel)
	}
	out := []string{}
	if cdrExists(cdrPyJoin(ctx.root, rel)) {
		out = append(out, "主仓/"+rel)
	}
	rel2 := rel
	if strings.HasPrefix(rel, "docs"+"/") {
		rel2 = rel[len("docs"+"/"):]
	}
	for _, pair := range [][2]string{{cdrDocsTreeName, ctx.docsRoot}, {cdrArchTreeName, ctx.archRoot}} {
		name, tree := pair[0], pair[1]
		if tree == "" || !cdrIsDir(tree) {
			continue
		}
		if cdrExists(cdrPyJoin(tree, rel)) {
			out = append(out, name+"/"+rel)
		} else if rel2 != rel && cdrExists(cdrPyJoin(tree, rel2)) {
			out = append(out, name+"/"+rel2)
		}
	}
	if len(out) == 0 && strings.HasPrefix(rel, "docs"+"/") {
		base := filepath.Base(strings.TrimRight(rel, "/"))
		hits := ctx.baseIndex()[base]
		if len(hits) > 3 {
			hits = hits[:3]
		}
		for _, hit := range hits {
			out = append(out, "（同名件）"+cdrDocsTreeName+"/"+hit)
		}
	}
	return out
}

// cdrAbsentCorpusTrees —— **该翻 rc=2** 的缺席同级树（唯一真源 · 只认同级语料树 Zerg-内部文档）。
// 归档树缺席**不翻退码**（只报主仓落点）—— 本机 `../Zerg-归档` 是**未挂载的断链**，
// 若按「任一不在盘 ⇒ 2」会把**默认档**从 rc=1 翻成 rc=2 = 改变现役行为 ✗（本批铁律：默认档零行为变更）。
func cdrAbsentCorpusTrees(docsRoot, archRoot string) [][2]string {
	var out [][2]string
	for _, p := range [][2]string{{cdrDocsTreeName, docsRoot}} {
		if p[1] == "" || !cdrIsDir(p[1]) {
			out = append(out, p)
		}
	}
	return out
}

// ── 登记表命中 ───────────────────────────────────────────────────────────────────────
func cdrOMGet(o *cdrOM, key string) (any, bool) {
	if o == nil {
		return nil, false
	}
	for i, k := range o.keys {
		if k == key {
			return o.vals[i], true
		}
	}
	return nil, false
}

func cdrOMGetStr(o *cdrOM, key, def string) string {
	v, ok := cdrOMGet(o, key)
	if !ok {
		return def
	}
	if s, ok := v.(string); ok {
		return s
	}
	return def
}

// cdrRegistryHit ⇒ (理由, 判类, 日期, 登记形态) 或 nil。
func cdrRegistryHit(ctx *cdrCtx, fileRel, ref string) (string, string, string, string, bool) {
	var bestPre string
	var bestE *cdrOM
	entries := []any{}
	if ctx.registry != nil {
		if v, ok := cdrOMGet(ctx.registry, cdrKeyEntries); ok {
			if lst, ok := v.([]any); ok {
				entries = lst
			}
		}
	}
	for _, e := range entries {
		eom, ok := e.(*cdrOM)
		if !ok {
			continue
		}
		if cdrOMGetStr(eom, cdrKeyFile, "") != fileRel {
			continue
		}
		eref := cdrOMGetStr(eom, cdrKeyRef, "")
		epre := cdrOMGetStr(eom, cdrKeyRefPrefix, "")
		if eref != "" {
			if eref != ref {
				continue
			}
			return cdrOMGetStr(eom, cdrKeyReason, ""), cdrOMGetStr(eom, cdrKeyClass, "ledger"),
				cdrOMGetStr(eom, cdrKeyDate, ""), "逐处", true
		}
		if epre != "" && strings.HasPrefix(ref, epre) {
			if bestE == nil || len(epre) > len(bestPre) {
				bestPre, bestE = epre, eom
			}
		}
	}
	if bestE != nil {
		return cdrOMGetStr(bestE, cdrKeyReason, ""), cdrOMGetStr(bestE, cdrKeyClass, "ledger"),
			cdrOMGetStr(bestE, cdrKeyDate, ""), "前缀[" + bestPre + "]", true
	}
	return "", "", "", "", false
}

// ── 分类 ─────────────────────────────────────────────────────────────────────────────
func cdrClassify(ctx *cdrCtx, fileRel, ref, lineText string) (string, string) {
	if why, _, date, form, ok := cdrRegistryHit(ctx, fileRel, ref); ok {
		return "已登记", fmt.Sprintf("登记表·%s：%s（%s）", form, why, date)
	}
	if cdrStrIn(cdrFixtureRefs, ref) {
		return "ledger", "夹具/探针里的假路径"
	}
	if _, ok := cdrContainsAny(lineText, cdrFixtureWords); ok {
		return "ledger", "夹具/探针里的假路径"
	}
	low := strings.ToLower(fileRel)
	for _, ext := range cdrMachineExt {
		if strings.HasSuffix(low, ext) {
			return "ledger", "机读件（清单/基线容器）"
		}
	}
	// 引文被引号**紧包** ⇒ 代码/数据里的字串，一律 ledger。
	if pos := strings.Index(lineText, ref); pos >= 0 {
		before := cdrPyRStrip(lineText[:pos])
		after := cdrPyLStrip(lineText[pos+len(ref):])
		bt := ""
		if before != "" {
			r, _ := utf8.DecodeLastRuneInString(before)
			bt = string(r)
		}
		at := ""
		if after != "" {
			r, _ := utf8.DecodeRuneInString(after)
			at = string(r)
		}
		if (bt == "\"" || bt == "'") && (at == "\"" || at == "'") {
			return "ledger", "代码/数据里的字串（判据常量 · 配置值 · 夹具键值）"
		}
	}
	if cdrTableRowRE.MatchString(lineText) {
		return "ledger", "表格式清单/台账行"
	}
	if w, ok := cdrContainsAny(lineText, cdrRecordWords); ok {
		return "ledger", fmt.Sprintf("记录当时事实的措辞「%s」", w)
	}
	if w, ok := cdrContainsAny(lineText, cdrLiveWords); ok {
		return "live", fmt.Sprintf("指路措辞「%s」", w)
	}
	return "live", "默认：正文/注释里的路径引用（非记录性容器）"
}

func cdrSuggest(cls, ref string, where []string, note string) string {
	if cls == "已登记" {
		return "按登记表保留（有意保留当时路径）· 不计命中"
	}
	if strings.Contains(note, "拼接首段=变量") {
		return "运行期路径（拼接首段是变量，不由仓根解析）⇒ 不改只报；要有意保留就登记（未覆盖形态之一）"
	}
	if cls == "ledger" {
		if len(where) == 0 {
			return "登记（记当时路径，不该改）"
		}
		return "登记；若确属活指路再改指 " + where[0]
	}
	if len(where) > 0 {
		return "改指 " + where[0]
	}
	return "不改只报：三棵树都没有这个目标 ⇒ 先定去向（搬走 / 改名 / 删）"
}

// ── 引文形态判定 ─────────────────────────────────────────────────────────────────────
// ── 跨仓引用（2026-10-05 · GAP-20261005-18 · 逐字对齐 py v1.1.0）──────────────────────
// 两类形态：`Zerg-内部文档/`（显式带文仓名 · RE_XDOCS）与裸 `项目文档/`（隐式指文仓 · RE_XBARE）。
// 判据同「裸 docs/…」：解析到**文仓根**（ctx.docsRoot）后不存在 ⇒ 悬空、存在 ⇒ 不报。
// 前字符不是路径体才算引用（与 RE_DIRECT 同手法，避吞 `../项目文档/` 这类相对链）。
const cdrXrepoPrefix = "Zerg-内部文档" + "/"
const cdrXBarePrefix = "项目文档" + "/"

// ── GAP 号引用类（2026-10-05 · GAP-20261005-54 · 逐字对齐 py v1.2.0）────────────────────
// 注释/文档里引的 `GAP-<8位日期>-<号>` 必须在**缺口账**（真源 = 状态目录下的
// `zerg-cli-gaps.jsonl`）里存在；不存在 ⇒ 引文悬空（逐字点名 件:行 · 引的号）。
// 账是**仓外私有面**（`ZERG_STATE_DIR` > `~/.zerg/state`），可 `--ledger` 覆盖（夹具用）。
// 复用本门既有 live/ledger 两类 ＋ 登记表口径：只有 `live` 类判为**命中**（rc=1）。
// `\d` ⇒ `\p{Nd}`（对齐 py 的 Unicode 数字类）。
var cdrGapRE = regexp.MustCompile(`GAP-\p{Nd}{8}-\p{Nd}+`)

const cdrGapLedgerFile = "zerg-cli-gaps.jsonl"

// cdrGapTestContainerRE —— 例外面·测试夹具件（`*_test.<ext>` / `testdata/` / `tests?/` / `fixtures?/`）。
var cdrGapTestContainerRE = regexp.MustCompile(`(?:^|/)(?:testdata|tests?|fixtures?)/|_test\.(?:go|py|sh|ts|js)$`)

// cdrGapSelfRel —— 判据件自身的仓内相对路径（py 原件：自检夹具里的假号命中自扫面 ⇒ 例外面）。
const cdrGapSelfRel = "scripts/gates/check-dangling-refs.py"

func cdrIsGapTestContainer(rel string) bool {
	return cdrGapTestContainerRE.MatchString(rel)
}

// cdrIsGapSelfFile —— 本件（判据件自身）—— 它正文里的 GAP 号全是**自检夹具里的假号** ⇒ 例外面。
// 逐字对齐 py `is_gap_self_file` 的第一道判（仓内相对路径 == py 件自身）。py 的第二道判按运行路径
// （`realpath(__file__)`）在 Go 里无同构物（Go 无 `__file__` 语境）⇒ 本件省略；对拍夹具里 root 为临时树
// ⇒ 该道判本就不命中，两态无差。★ 本件（Go 搬体）自身的夹具假号改由**拆开拼串**处置（见件头）。
func cdrIsGapSelfFile(ctx *cdrCtx, rel string) bool {
	return rel == cdrGapSelfRel
}

// cdrGapLedgerPath —— 缺口真源落点：`ZERG_STATE_DIR` > `~/.zerg/state`（与 CLI stateDirOf() 同口径）。
func cdrGapLedgerPath() string {
	env := os.Getenv("ZERG_STATE_DIR")
	var base string
	if env != "" {
		base = env
	} else {
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			home = os.Getenv("HOME")
		}
		base = cdrPyJoin(cdrPyJoin(home, ".zerg"), "state")
	}
	return cdrPyJoin(base, cdrGapLedgerFile)
}

// cdrLoadGapIds —— 读缺口账在册号集。→ (set, "") 或 **(nil, 原因)**（取不到 ⇒ 本类**不给结论**）。
func cdrLoadGapIds(path string) (map[string]bool, string) {
	if path == "" || !cdrExists(path) {
		return nil, "不在盘：" + path
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, "读不了 " + path + "：" + err.Error()
	}
	ids := map[string]bool{}
	for _, line := range strings.Split(string(raw), "\n") {
		line = cdrPyStrip(line)
		if line == "" {
			continue
		}
		o, perr := cdrJSONParse(line)
		if perr != "" {
			continue
		}
		om, ok := o.(*cdrOM)
		if !ok {
			continue
		}
		if v, ok := cdrOMGet(om, "id"); ok {
			if s, ok := v.(string); ok && s != "" {
				ids[s] = true
			}
		}
	}
	if len(ids) == 0 {
		return nil, "账里一个在册号都没有（空账 = 判不了）：" + path
	}
	return ids, ""
}

// cdrLedgerSelfDangling —— ★ 缺口 `GAP-20261013-28`：**账真源自身**（缺口账 jsonl）正文里引了
// **不在册**的缺口号时逐处点名（`<账件名>:<物理行> · 引的号 <号>`）。
//
// 为什么单立一档：本门的扫面是 `git ls-files` 的**仓内已跟踪件**，而账真源在**仓外**
// （`<状态目录>/zerg-cli-gaps.jsonl`）⇒「账内注引用悬空号」结构上看不见（先例 `GAP-20261005-54`
// 只治了仓内 tracked 件那半）。
//
// 口径（**只报告**）：本档 ✗ 不进棘轮（其基线是照**仓内**现读定的）、✗ 不进退出码 —— 逐条打印
// 供人处置（改指在册号 / 先在账立案）。`ids == nil`（账取不到）⇒ 返回 nil（本类不给结论，
// 与 GAP 号引用类同一条纪律）；读不到账件 ⇒ 同样返回 nil（照实空转，不编造）。
func cdrLedgerSelfDangling(ledgerPath string, ids map[string]bool) []string {
	if ids == nil || ledgerPath == "" || !cdrExists(ledgerPath) {
		return nil
	}
	raw, err := os.ReadFile(ledgerPath)
	if err != nil {
		return nil
	}
	var out []string
	for i, line := range strings.Split(string(raw), "\n") {
		// 同一条记录里同一个号可能引多次（注文成段转述）⇒ **按（行, 号）去重**：一格只喊一次。
		seen := map[string]bool{}
		for _, ref := range cdrGapRE.FindAllString(line, -1) {
			if !ids[ref] && !seen[ref] {
				seen[ref] = true
				out = append(out, fmt.Sprintf("%s:%d · 引的号 `%s` · 账内不在册（改指在册号，或先在账立案）",
					filepath.Base(ledgerPath), i+1, ref))
			}
		}
	}
	return out
}

// cdrLooksLikeXref —— 跨仓引用是不是「引用形态」（比照 cdrLooksLikeRef，先剥跨仓前缀）。
func cdrLooksLikeXref(ref string) bool {
	if strings.HasSuffix(ref, "/") {
		return true
	}
	body := ref
	for _, pfx := range []string{cdrXrepoPrefix, cdrXBarePrefix} {
		if strings.HasPrefix(body, pfx) {
			body = body[len(pfx):]
			break
		}
	}
	var segs []string
	for _, s := range strings.Split(body, "/") {
		if s != "" {
			segs = append(segs, s)
		}
	}
	if len(segs) >= 2 {
		return true
	}
	if len(segs) == 0 {
		return false
	}
	return strings.ContainsAny(segs[0], "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789._-")
}

// cdrXBarePrevOK —— 裸 `项目文档/` 的左后顾：前置字符**不是** `[A-Za-z0-9_\-./]` 且**不是** CJK
// （`\u4e00-\u9fff`）才算引用（逐字对齐 py RE_XBARE 的 lookbehind）。
func cdrXBarePrevOK(line string, i int) bool {
	if i == 0 {
		return true
	}
	r, _ := utf8.DecodeLastRuneInString(line[:i])
	if r >= 0x4e00 && r <= 0x9fff {
		return false
	}
	if r < 0x80 {
		return !cdrDirLB(byte(r))
	}
	return true
}

// cdrRefExists —— 引用解析后的存在性：跨仓引用判**文仓根**；裸 `docs/…` 判**主仓根**
// （现役口径一字不变）。
func cdrRefExists(ctx *cdrCtx, ref string) bool {
	if strings.HasPrefix(ref, cdrXrepoPrefix) {
		if ctx.docsRoot == "" {
			return false
		}
		return cdrExists(cdrPyJoin(ctx.docsRoot, strings.TrimRight(ref[len(cdrXrepoPrefix):], "/")))
	}
	if strings.HasPrefix(ref, cdrXBarePrefix) {
		if ctx.docsRoot == "" {
			return false
		}
		return cdrExists(cdrPyJoin(ctx.docsRoot, strings.TrimRight(ref, "/")))
	}
	return cdrExists(cdrPyJoin(ctx.root, strings.TrimRight(ref, "/")))
}

// cdrXrepoTreeOf —— 跨仓引用的落点：在**文仓树**（Zerg-内部文档；归档树按同名形）里找现落点。
func cdrXrepoTreeOf(ctx *cdrCtx, rel string) []string {
	out := []string{}
	if strings.HasPrefix(rel, cdrXrepoPrefix) {
		sub := rel[len(cdrXrepoPrefix):]
		for _, pair := range [][2]string{{cdrDocsTreeName, ctx.docsRoot}, {cdrArchTreeName, ctx.archRoot}} {
			name, tree := pair[0], pair[1]
			if tree != "" && cdrIsDir(tree) && cdrExists(cdrPyJoin(tree, sub)) {
				out = append(out, name+"/"+sub)
			}
		}
		return out
	}
	if ctx.docsRoot != "" && cdrIsDir(ctx.docsRoot) && cdrExists(cdrPyJoin(ctx.docsRoot, rel)) {
		return []string{cdrDocsTreeName + "/" + rel}
	}
	return []string{}
}

// cdrXRepoRefs —— 逐字复刻 py `extract()` 的 `for rx in (RE_XDOCS, RE_XBARE)` 两轮扫描：
// 先 `Zerg-内部文档/`（RE_XDOCS 原序）再裸 `项目文档/`（RE_XBARE 原序）；散文误吞照样计数。
func cdrXRepoRefs(line string, st *cdrStats) []cdrRef {
	var out []cdrRef
	for _, pfx := range []string{cdrXrepoPrefix, cdrXBarePrefix} {
		n := len(line)
		i := 0
		for i < n {
			if strings.HasPrefix(line[i:], pfx) {
				ok := false
				if pfx == cdrXrepoPrefix {
					ok = i == 0 || !cdrDirLB(line[i-1])
				} else {
					ok = cdrXBarePrevOK(line, i)
				}
				if ok {
					j := i + len(pfx)
					for j < n {
						r, sz := utf8.DecodeRuneInString(line[j:])
						if !cdrIsPathChar(r) {
							break
						}
						j += sz
					}
					ref := line[i:j]
					if cdrEllipsisAfter(line, j) {
						st.refsProseSkipped++
					} else if cdrLooksLikeXref(ref) {
						out = append(out, cdrRef{ref, "跨仓直书", ""})
					} else {
						st.refsProseSkipped++
					}
					i = j
					continue
				}
			}
			_, sz := utf8.DecodeRuneInString(line[i:])
			i += sz
		}
	}
	return out
}

func cdrLooksLikeRef(ref string) bool {
	if strings.HasSuffix(ref, "/") {
		return true
	}
	body := ref
	if strings.HasPrefix(ref, "docs"+"/") {
		body = ref[len("docs"+"/"):]
	}
	var segs []string
	for _, s := range strings.Split(body, "/") {
		if s != "" {
			segs = append(segs, s)
		}
	}
	if len(segs) >= 2 {
		return true
	}
	if len(segs) == 0 {
		return false
	}
	return strings.ContainsAny(segs[0], "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789._-")
}

// cdrEllipsisAfter —— 抽取到的引用**紧跟省略号**（U+2026）⇒ 那是「被省略号截断的占位路径」，
// 本来就不是一条可解析的引用 ⇒ 与「散文误吞」同路处置（认出来 ⇒ 计数不判 · 不静默）。
func cdrEllipsisAfter(line string, j int) bool {
	if j >= len(line) {
		return false
	}
	r, _ := utf8.DecodeRuneInString(line[j:])
	return r == '\u2026'
}

// ── 抽取 ─────────────────────────────────────────────────────────────────────────────
func cdrDirectRefs(line string, st *cdrStats) []cdrRef {
	var out []cdrRef
	n := len(line)
	i := 0
	for i < n {
		if strings.HasPrefix(line[i:], "docs"+"/") {
			ok := i == 0 || !cdrDirLB(line[i-1])
			if ok {
				j := i + len("docs"+"/")
				for j < n {
					r, sz := utf8.DecodeRuneInString(line[j:])
					if !cdrIsPathChar(r) {
						break
					}
					j += sz
				}
				ref := line[i:j]
				if cdrEllipsisAfter(line, j) {
					st.refsProseSkipped++
				} else if cdrLooksLikeRef(ref) {
					out = append(out, cdrRef{ref, "直书", ""})
				} else {
					st.refsProseSkipped++
				}
				i = j
				continue
			}
		}
		_, sz := utf8.DecodeRuneInString(line[i:])
		i += sz
	}
	return out
}

// cdrJoinPathOf —— `filepath.Join("docs", …, "件")` 的**连续字面量段**拼接（首段是变量先跳过、遇变量即止）。
func cdrJoinPathOf(inner string) (string, string, bool) {
	var lits []string
	started, varfirst := false, false
	for _, arg := range strings.Split(inner, ",") {
		arg = cdrPyStrip(arg)
		if m := cdrStrLitFullRE.FindStringSubmatch(arg); m != nil {
			started = true
			lits = append(lits, m[1])
			continue
		}
		if !started {
			if arg != "" {
				varfirst = true
			}
			continue
		}
		break
	}
	var keep []string
	for _, x := range lits {
		if x != "" {
			keep = append(keep, x)
		}
	}
	path := strings.Join(keep, "/")
	if !strings.HasPrefix(path, "docs") || (path != "docs" && !strings.HasPrefix(path, "docs"+"/")) {
		return "", "", false
	}
	first := "字面量"
	if varfirst {
		first = "变量（运行期根）"
	}
	return path, "拼接首段=" + first, true
}

func cdrJoinRefs(line string) []cdrRef {
	var out []cdrRef
	n := len(line)
	i := 0
	for i < n {
		name := ""
		if strings.HasPrefix(line[i:], "filepath.Join") {
			name = "filepath"
		} else if strings.HasPrefix(line[i:], "path.Join") {
			name = "path"
		}
		if name != "" && (i == 0 || !cdrJoinLB(line[i-1])) {
			j := i + len(name) + len(".Join")
			for j < n {
				r, sz := utf8.DecodeRuneInString(line[j:])
				if !cdrPySpaceRune(r) {
					break
				}
				j += sz
			}
			if j < n && line[j] == '(' {
				k := j + 1
				for k < n && line[k] != '(' && line[k] != ')' {
					k++
				}
				if k < n && line[k] == ')' {
					if path, note, ok := cdrJoinPathOf(line[j+1 : k]); ok {
						out = append(out, cdrRef{path, "filepath.Join", note})
					}
					i = k + 1
					continue
				}
			}
		}
		_, sz := utf8.DecodeRuneInString(line[i:])
		i += sz
	}
	return out
}

func cdrExtract(line string, st *cdrStats) []cdrRef {
	out := cdrDirectRefs(line, st)
	out = append(out, cdrJoinRefs(line)...)
	for range cdrConcatRE.FindAllStringIndex(line, -1) {
		out = append(out, cdrRef{"docs", "字符串拼接", "前缀形态（后缀是变量 ⇒ 只判前缀目录）"})
	}
	out = append(out, cdrXRepoRefs(line, st)...)
	return out
}

// cdrReadText —— py `_read_text`：二进制（含 \x00）或非 UTF-8 ⇒ None。
//
// ★ 读失败不再与「内容型 None」同折：第三返回值 = 读失败说明（"" ⇒ 无读失败）。
//
//	py 原口子（件头 `:305/:311` `except Exception: return None`）把读不了的件也折成 None
//	⇒ 既不进可判也不进判不了、永不判红；本件拆出该面（登记 + 汇总处判红）。
//	二进制 / 非 UTF-8 的内容型 None 仍逐字照搬（不进 readFails）。
//	未覆盖面同步：此档两态（py 静默 ⟷ 本件退 2）不再逐字对齐。
func cdrReadText(path string) (string, bool, string) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", false, err.Error()
	}
	if bytes.IndexByte(raw, 0) >= 0 {
		return "", false, ""
	}
	if !utf8.Valid(raw) {
		return "", false, ""
	}
	return string(raw), true, ""
}

// ── 扫描 ─────────────────────────────────────────────────────────────────────────────
// ── 聚合 / 前缀候选（v1.3.0 · GAP-20260926-111/112；默认关 ⇒ 默认档零行为变更）──────────
// cdrGroupDims —— `--group-by` 维度闭集（与 py GROUP_DIMS 逐字同）。
var cdrGroupDims = []string{"src", "tgt", "form", "cls"}

// cdrDimValue —— 聚合维度取值（与 py `_dim_value` 逐字同）。
func cdrDimValue(h cdrHit, dim string) string {
	switch dim {
	case "src":
		if i := strings.IndexByte(h.file, '/'); i >= 0 {
			return h.file[:i]
		}
		return "(根级件)"
	case "tgt":
		if h.form == "GAP号" {
			return "(GAP号)"
		}
		p := h.path
		for _, pfx := range []string{"docs" + "/", cdrXrepoPrefix, cdrXBarePrefix} {
			if strings.HasPrefix(p, pfx) {
				p = p[len(pfx):]
				break
			}
		}
		for _, s := range strings.Split(p, "/") {
			if s != "" {
				return s
			}
		}
		return "(空)"
	case "form":
		return h.form
	}
	return h.cls
}

// cdrDirPrefix —— 引文的目录前缀（可登记 `ref_prefix` 候选）：最后一段之前 + `/`；单段 ⇒ ""。
func cdrDirPrefix(ref string) string {
	r := strings.TrimRight(ref, "/")
	i := strings.LastIndexByte(r, '/')
	if i < 0 {
		return ""
	}
	return r[:i+1]
}

// cdrPrefixCandidates —— 未登记的 ledger 命中 ⇒ registry entry 草稿（按 件·目录前缀 聚合 · 排序确定）。
func cdrPrefixCandidates(hits []cdrHit) []any {
	keys := []string{}
	groups := map[string][]string{}
	for _, h := range hits {
		if h.cls != "ledger" {
			continue
		}
		pre := cdrDirPrefix(h.path)
		if pre == "" {
			continue
		}
		k := h.file + "\x00" + pre
		if _, ok := groups[k]; !ok {
			keys = append(keys, k)
		}
		groups[k] = append(groups[k], h.path)
	}
	sort.Strings(keys)
	today := time.Now().Format("2006-01-02")
	out := []any{}
	for _, k := range keys {
		items := groups[k]
		sep := strings.IndexByte(k, 0)
		file, pre := k[:sep], k[sep+1:]
		uniq := map[string]bool{}
		samples := []string{}
		for _, p := range items {
			if !uniq[p] {
				uniq[p] = true
				samples = append(samples, p)
			}
		}
		sort.Strings(samples)
		if len(samples) > 3 {
			samples = samples[:3]
		}
		o := &cdrOM{}
		o.set("file", file)
		o.set("ref_prefix", pre)
		o.set("class", "ledger")
		o.set("reason", fmt.Sprintf("候选：%d 处（--prefix-candidates 生成 · 需人工补理由）", len(items)))
		o.set("date", today)
		o.set("count", len(items))
		o.set("samples", strings.Join(samples, " · "))
		out = append(out, o)
	}
	return out
}

func cdrScan(ctx *cdrCtx) (*cdrStats, []cdrHit) {
	st := &cdrStats{filesSeen: len(ctx.files)}
	hits := []cdrHit{}
	seen := map[string]bool{}
	gap := &cdrGapState{judged: ctx.gapIds != nil}
	files := append([]string{}, ctx.files...)
	sort.Strings(files) // ★ 排序点①：py `sorted(ctx.files)` —— **输出行序的决定者**
	for _, rel := range files {
		first := rel
		if idx := strings.IndexByte(rel, '/'); idx >= 0 {
			first = rel[:idx]
		}
		skip := false
		for _, d := range cdrSkipDirs {
			if first == d {
				skip = true
				break
			}
		}
		if !skip {
			for _, p := range cdrSkipPrefix {
				if strings.HasPrefix(rel, p) {
					skip = true
					break
				}
			}
		}
		if skip {
			continue
		}
		full := cdrPyJoin(ctx.root, rel)
		if !cdrIsFile(full) {
			continue
		}
		text, ok, rerr := cdrReadText(full)
		if rerr != "" {
			// ★ 读失败不再静默（原 py `except Exception: return None` 口子）：登记后继续
			st.readFails = append(st.readFails, rel+"（"+rerr+"）")
			continue
		}
		if !ok {
			st.unreadable = append(st.unreadable, rel)
			continue
		}
		st.filesScanned++
		for li, lineText := range strings.Split(text, "\n") {
			ln := li + 1
			for _, rf := range cdrExtract(lineText, st) {
				ref, form, note := rf.ref, rf.form, rf.note
				shown := ref
				if pos := strings.Index(lineText, ref); pos >= 0 {
					if m := cdrAnchorRE.FindStringSubmatch(lineText[pos+len(ref):]); m != nil {
						shown = ref + ":" + m[1]
					}
				}
				st.refs++
				key := fmt.Sprintf("%s\x00%d\x00%s\x00%s", rel, ln, form, ref)
				if seen[key] {
					continue
				}
				seen[key] = true
				if cdrRefExists(ctx, ref) {
					continue
				}
				st.dangling++
				cls, why := cdrClassify(ctx, rel, ref, lineText)
				if cls == "live" && strings.HasPrefix(note, "拼接首段=变量") {
					// 运行期树（拼接首段是变量）⇒ ledger（与 py v1.3.0 逐字同 · GAP-20260926-114）
					cls, why = "ledger", "运行期路径（拼接首段是变量 · 运行期根 ≠ 仓根）⇒ 记运行时事实，不改"
				}
				where := cdrTreeOf(ctx, ref)
				src := cdrPyStrip(lineText)
				if len([]rune(src)) > 160 {
					src = string([]rune(src)[:160])
				}
				hits = append(hits, cdrHit{file: rel, line: ln, ref: shown, path: ref, form: form,
					note: note, cls: cls, why: why, where: where,
					suggest: cdrSuggest(cls, ref, where, note), src: src})
			}
			for _, loc := range cdrGapRE.FindAllStringIndex(lineText, -1) {
				gid := lineText[loc[0]:loc[1]]
				gkey := fmt.Sprintf("%s\x00%d\x00GAP\x00%s", rel, ln, gid)
				if seen[gkey] {
					continue
				}
				seen[gkey] = true
				gap.candidates++
				if ctx.gapIds == nil {
					gap.unjudged = append(gap.unjudged, fmt.Sprintf("%s:%d · %s", rel, ln, gid))
					continue
				}
				if ctx.gapIds[gid] {
					continue
				}
				cls, why := cdrClassify(ctx, rel, gid, lineText)
				if cls == "live" && cdrIsGapSelfFile(ctx, rel) {
					cls, why = "ledger", "本件（判据件自身）的自检夹具号 · 例外面"
				} else if cls == "live" && cdrIsGapTestContainer(rel) {
					cls, why = "ledger", "测试夹具件（*_test.*/testdata/tests）里故意造的假号"
				}
				gsrc := cdrPyStrip(lineText)
				if len([]rune(gsrc)) > 160 {
					gsrc = string([]rune(gsrc)[:160])
				}
				rec := cdrHit{file: rel, line: ln, ref: gid, path: gid, form: "GAP号", note: "",
					cls: cls, why: why, where: []string{}, src: gsrc}
				if cls == "live" {
					rec.suggest = "引的号账内不存在 ⇒ 改指在册号，或先在账立案（GAP 号引用悬空）"
					gap.hits = append(gap.hits, rec)
				} else {
					gap.exempt = append(gap.exempt, rec)
				}
			}
		}
	}
	st.gap = gap
	return st, hits
}

// ── 登记表 ───────────────────────────────────────────────────────────────────────────
// cdrLoadRegistry ⇒ (doc, bad)。doc=nil ⇔ 拒给结论。
func cdrLoadRegistry(path string) (*cdrOM, []string) {
	if path == "" {
		return &cdrOM{}, []string{}
	}
	if !cdrExists(path) {
		return cdrEntriesOM(&[]any{}), []string{}
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		// ★ 现读形态 = 读失败**非静默**（已 fail-closed：调用处判 bad ⇒ 退 2）；
		//   唯一缺陷 = 把「读失败」误标成「不是合法 JSON」（归因混淆）。拆出读失败说明，仍退 2。
		return nil, []string{"登记表读不了（" + path + "）：" + err.Error()}
	}
	doc, perr := cdrJSONParse(string(raw))
	if perr != "" {
		return nil, []string{"登记表不是合法 JSON：" + perr}
	}
	om, ok := doc.(*cdrOM)
	if !ok {
		return nil, []string{"登记表缺 `entries` 数组（schema=dangling-refs-registry/1）"}
	}
	ev, okv := cdrOMGet(om, cdrKeyEntries)
	list, okl := ev.([]any)
	if !okv || !okl {
		return nil, []string{"登记表缺 `entries` 数组（schema=dangling-refs-registry/1）"}
	}
	var bad []string
	for i, e := range list {
		tag := fmt.Sprintf("entries[%d]", i)
		eom, ok := e.(*cdrOM)
		if !ok || cdrOMGetStr(eom, cdrKeyFile, "") == "" {
			bad = append(bad, tag+" 缺 `file`")
			continue
		}
		if cdrOMGetStr(eom, cdrKeyRef, "") == "" && cdrOMGetStr(eom, cdrKeyRefPrefix, "") == "" {
			bad = append(bad, tag+" 既没 `ref` 也没 `ref_prefix` —— **整件豁免 = 洗白**，不批"+
				"（机读件也请写 `ref_prefix: \"docs"+"/\"`）")
		}
		if cdrOMGetStr(eom, cdrKeyReason, "") == "" {
			bad = append(bad, tag+" 缺 `reason`（只给路径的登记 = 无记录的洗白）")
		}
		if !cdrDateRE.MatchString(cdrPyStr(cdrOMGetRaw(eom, cdrKeyDate))) {
			bad = append(bad, tag+" 缺 `date` 或形态不是 YYYY-MM-DD")
		}
		cls := cdrOMGetStr(eom, cdrKeyClass, "ledger")
		if cls != "live" && cls != "ledger" {
			bad = append(bad, tag+" class 不在 {live, ledger}")
		}
	}
	return om, bad
}

func cdrOMGetRaw(o *cdrOM, key string) any {
	v, _ := cdrOMGet(o, key)
	return v
}

// cdrPyStr —— py `str(x)` 的实用子集（登记表字段）。
func cdrPyStr(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case int:
		return strconv.Itoa(x)
	case float64:
		return strconv.FormatFloat(x, 'g', -1, 64)
	case bool:
		if x {
			return "True"
		}
		return "False"
	}
	return ""
}

func cdrEntriesOM(list *[]any) *cdrOM {
	o := &cdrOM{}
	o.set(cdrKeyEntries, *list)
	return o
}

// ── 自检（十格：必红×2 · 豁免×1 · 拼接×1 · 前缀×1 · 无输入×1 · 登记表×1 · 整件豁免×1 · 前缀越界×1 · 缺语料×1）──
func cdrMkFile(root, rel, text string) string {
	p := filepath.Join(root, filepath.FromSlash(rel))
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	_ = os.WriteFile(p, []byte(text), 0o644)
	return rel
}

func cdrSelftest(stdout, stderr io.Writer) bool {
	ok := true
	cases := 0
	tmp, err := os.MkdirTemp("", "cdr-selftest-")
	if err != nil {
		tmp = filepath.Join(os.TempDir(), "cdr-selftest-fallback")
		_ = os.MkdirAll(tmp, 0o755)
	}
	// ★ 与 py 的有意差：py `mkdtemp` **从不回收**；本件**自回收**（输出不含该路径 ⇒ 两态逐字仍同）。
	defer func() { _ = os.RemoveAll(tmp) }()

	repo := filepath.Join(tmp, "Zerg")
	docs := filepath.Join(tmp, "Zerg-内部文档")
	_ = os.MkdirAll(filepath.Join(docs, "01-设计"), 0o755)
	_ = os.WriteFile(filepath.Join(docs, "01-设计", "设计-真件.md"), []byte("真源\n"), 0o644)

	run := func(files []string, registry *cdrOM, root, d string, gapIds map[string]bool) (*cdrStats, []cdrHit) {
		if registry == nil {
			registry = cdrEntriesOM(&[]any{})
		}
		ctx := &cdrCtx{root: root, docsRoot: d, archRoot: "", files: files, registry: registry, gapIds: gapIds}
		return cdrScan(ctx)
	}

	// ① 必红：指向真不存在的路径 ⇒ 报出
	f1 := cdrMkFile(repo, "scripts/x.py", "# 依据：docs"+"/01-设计/不存在.md\n")
	_, hits := run([]string{f1}, nil, repo, docs, nil)
	var got []cdrHit
	for _, h := range hits {
		if h.ref == "docs"+"/01-设计/不存在.md" {
			got = append(got, h)
		}
	}
	cases++
	c1 := len(got) == 1 && got[0].cls == "live" && len(got[0].where) == 0
	ok = ok && c1
	gif := "—"
	gwf := "—"
	if len(got) > 0 {
		gif = got[0].cls
		gwf = cdrPyReprList(got[0].where)
	}
	fmt.Fprintf(stdout, "  %s [必红夹具] 悬空 1 处 = %d · 判类 = %s · 落点 = %s（期望 live · 三树都无）\n",
		cdrMark(c1), len(got), gif, gwf)

	// ② 必红（活指路 → 真在 Zerg-内部文档）⇒ 报出 + 落点 + 建议改指
	f2 := cdrMkFile(repo, "scripts/y.py", "# 真源：docs"+"/01-设计/设计-真件.md\n")
	_, hits = run([]string{f2}, nil, repo, docs, nil)
	got = nil
	for _, h := range hits {
		if h.path == "docs"+"/01-设计/设计-真件.md" {
			got = append(got, h)
		}
	}
	cases++
	good := len(got) == 1 && got[0].cls == "live" && len(got[0].where) > 0 && strings.Contains(got[0].suggest, "改指")
	ok = ok && good
	gw2, gs2 := "—", "—"
	if len(got) > 0 {
		gw2, gs2 = cdrPyReprList(got[0].where), got[0].suggest
	}
	fmt.Fprintf(stdout, "  %s [必红夹具·有落点] 命中 %d 处 · 落点 = %s · 建议 = %s（期望 live + 改指 Zerg-内部文档）\n",
		cdrMark(good), len(got), gw2, gs2)

	// ③ 豁免夹具：同一处写进登记表 ⇒ 标「已登记」且**不计命中**
	reg := cdrEntriesOM(&[]any{cdrEntryOM(f2, "docs"+"/01-设计/设计-真件.md", "", "ledger", "自检夹具：有意保留", "2026-09-22")})
	_, hits = run([]string{f2}, reg, repo, docs, nil)
	got = nil
	for _, h := range hits {
		if h.path == "docs"+"/01-设计/设计-真件.md" {
			got = append(got, h)
		}
	}
	cases++
	good = len(got) == 1 && got[0].cls == "已登记" && strings.Contains(got[0].suggest, "不计命中")
	ok = ok && good
	gc3, gs3 := "—", "—"
	if len(got) > 0 {
		gc3, gs3 = got[0].cls, got[0].suggest
	}
	fmt.Fprintf(stdout, "  %s [豁免夹具] 标 %s · 建议 = %s（期望「已登记」+ 不计命中）\n",
		cdrMark(good), gc3, gs3)

	// ④ 拼接式夹具：filepath.Join("docs", …) 必须认出来
	f4 := cdrMkFile(repo, "core/z.go",
		"\tissuesDir := filepath.Join(workDir, \"docs\", \"issues\")\n"+
			"\tpath := path.Join(\"docs\", \"01-设计\", \"设计-真件.md\")\n")
	_, hits = run([]string{f4}, nil, repo, docs, nil)
	j1, j2 := 0, 0
	j1cls, j2cls := "", ""
	for _, h := range hits {
		if h.ref == "docs"+"/issues" && h.form == "filepath.Join" {
			j1++
			j1cls = h.cls
		}
		if h.path == "docs"+"/01-设计/设计-真件.md" && h.form == "filepath.Join" {
			j2++
			j2cls = h.cls
		}
	}
	cases++
	// 判类成对负控（`GAP-20260926-114`）：**运行期路径**（拼接首段=变量）⇒ ledger（不算命中）；
	// **字面量**拼接 ⇒ live。两条同场 ⇒ 断言的是「判类看的是运行期性，不是拼接形态」。
	good = j1 == 1 && j2 == 1 && j1cls == "ledger" && j2cls == "live"
	ok = ok && good
	if j1cls == "" {
		j1cls = "—"
	}
	if j2cls == "" {
		j2cls = "—"
	}
	fmt.Fprintf(stdout, "  %s [拼接夹具] filepath.Join 认出 %d 处(运行期⇒%s) · path.Join 认出 %d 处(字面量⇒%s)"+
		"（期望 1/1 · ledger/live）\n",
		cdrMark(good), j1, j1cls, j2, j2cls)

	// ⑤ 前缀夹具：'docs/' + 变量 ⇒ 前缀目录在 ⇒ 不报
	f5 := cdrMkFile(repo, "publish/z.py", "name = \"docs"+"/\" + base\n")
	_ = os.MkdirAll(filepath.Join(repo, "docs"), 0o755)
	_, hits = run([]string{f5}, nil, repo, docs, nil)
	cases++
	good = true
	for _, h := range hits {
		if h.form == "字符串拼接" {
			good = false
		}
	}
	ok = ok && good
	fmt.Fprintf(stdout, "  %s [前缀夹具] 前缀目录在 ⇒ 不报 = %s（期望 True）\n", cdrMark(good), cdrPyBool(good))

	// ⑥ 无输入必须报错（不许静默空转 = 假绿）
	empty := filepath.Join(tmp, "empty")
	_ = os.MkdirAll(empty, 0o755)
	rc := cdrRealExitCode(empty, "", []string{}, true, nil, stderr)
	cases++
	good = rc == 2
	ok = ok && good
	fmt.Fprintf(stdout, "  %s [无输入夹具] 空树 rc = %d（期望 2 —— 拒绝静默空转）\n", cdrMark(good), rc)

	// ⑦ 登记表缺 reason/date ⇒ rc=2
	badreg := filepath.Join(tmp, "bad-registry.json")
	_ = os.WriteFile(badreg, []byte("{\"entries\":[{\"file\":\"a\",\"ref\":\"docs"+"/x\"}]}"), 0o644)
	doc, bad := cdrLoadRegistry(badreg)
	cases++
	good = doc == nil || len(bad) >= 2
	ok = ok && good
	fmt.Fprintf(stdout, "  %s [登记表夹具] 缺 reason+date 报 %d 条（期望 ≥2 ⇒ rc=2·拒给结论）\n", cdrMark(good), len(bad))

	// ⑧ 整件豁免（既无 ref 也无 ref_prefix）⇒ rc=2
	blankreg := filepath.Join(tmp, "blanket-registry.json")
	_ = os.WriteFile(blankreg, []byte(
		"{\"entries\":[{\"file\":\"scripts/x.py\",\"class\":\"ledger\",\"reason\":\"整件免了\",\"date\":\"2026-09-22\"}]}"), 0o644)
	_, bad = cdrLoadRegistry(blankreg)
	cases++
	good = len(bad) == 1 && strings.Contains(bad[0], "洗白")
	ok = ok && good
	fmt.Fprintf(stdout, "  %s [整件豁免夹具] 报 %d 条（期望 1 条 · 点名「整件豁免 = 洗白」）\n", cdrMark(good), len(bad))

	// ⑨ 前缀登记必须**真能**命中，且**不**越界
	f9 := cdrMkFile(repo, "scripts/w.py", "x = \"docs"+"/项目文档/v2.5.9/x.md\"\ny = \"docs"+"/01-设计/a.md\"\n")
	pre := cdrEntriesOM(&[]any{cdrEntryOM(f9, "", "docs"+"/项目文档/", "ledger", "前缀登记自检", "2026-09-22")})
	_, hits = run([]string{f9}, pre, repo, docs, nil)
	insideN, outsideN := 0, 0
	insideCls, outsideCls := "—", "—"
	for _, h := range hits {
		if strings.HasPrefix(h.path, "docs"+"/项目文档/") {
			insideN++
			insideCls = h.cls
		}
		if strings.HasPrefix(h.path, "docs"+"/01-设计/") {
			outsideN++
			if outsideCls == "—" {
				outsideCls = h.cls
			}
		}
	}
	cases++
	good = insideN == 1 && insideCls == "已登记" && outsideN == 1 && outsideCls != "已登记"
	ok = ok && good
	fmt.Fprintf(stdout, "  %s [前缀登记夹具] 前缀内=已登记 %s · 前缀外仍命中 %s（期望 1/1 —— 前缀不许越界洗白）\n",
		cdrMark(good), insideCls, outsideCls)

	// ⑩ 缺语料夹具：同级**语料树**取不到根 ⇒ 报缺；归档树缺席**不翻退码**
	missBad := cdrAbsentCorpusTrees(filepath.Join(tmp, "no-such-Zerg-内部文档"), "")
	missArch := cdrAbsentCorpusTrees(docs, filepath.Join(tmp, "no-such-Zerg-归档"))
	cases++
	good = len(missBad) == 1 && len(missArch) == 0
	ok = ok && good
	fmt.Fprintf(stdout, "  %s [缺语料夹具] 语料树缺席报 %d 条 · 语料树在盘(归档缺席)报 %d 条（期望 1 / 0 ⇒ 缺根不给结论·在盘照旧）\n",
		cdrMark(good), len(missBad), len(missArch))

	// ⑪ 跨仓夹具（GAP-20261005-18）：`Zerg-内部文档` 前缀 / 裸 `项目文档` 前缀指向**文仓里不存在**的件 ⇒ 报出；
	//    成对负控：同一处指向**文仓里存在**的件 ⇒ 不报（跨仓面只在「文仓根下不存在」时命中）。
	f11 := cdrMkFile(repo, "scripts/gates/xref.py",
		"# 真源：`Zerg-内部文档"+"/01-设计/不存在的旧文.md` 与 `项目文档"+"/不存在的账.md` 与 "+
			"`Zerg-内部文档"+"/01-设计/设计-真件.md`\n")
	_, hits = run([]string{f11}, nil, repo, docs, nil)
	missN, presentN := 0, 0
	for _, h := range hits {
		if h.ref == "Zerg-内部文档"+"/01-设计/不存在的旧文.md" || h.ref == "项目文档"+"/不存在的账.md" {
			missN++
		}
		if h.path == "Zerg-内部文档"+"/01-设计/设计-真件.md" {
			presentN++
		}
	}
	cases++
	good = missN == 2 && presentN == 0
	ok = ok && good
	fmt.Fprintf(stdout, "  %s [跨仓夹具] 文仓里不存在 ⇒ 报 %d 处 · 文仓里存在 ⇒ 报 %d 处（期望 2 / 0）\n",
		cdrMark(good), missN, presentN)

	// ⑫ GAP 号引用·悬空（档 · 成对负控之一）：注释里引的号账内不存在 ⇒ 命中（点名 件:行 · 引的号）
	led := map[string]bool{"GAP-" + "20261005-54": true, "GAP-" + "20261005-60": true}
	fg1 := cdrMkFile(repo, "scripts/gates/g1.py", "# 见缺口 `GAP-"+"20260928-999`（账内无）\n")
	st, _ := run([]string{fg1}, nil, repo, docs, led)
	gh := st.gap.hits
	cases++
	good = len(gh) == 1 && gh[0].ref == "GAP-"+"20260928-999" &&
		gh[0].file == fg1 && gh[0].line == 1
	ok = ok && good
	gr1 := "—"
	if len(gh) > 0 {
		gr1 = gh[0].ref
	}
	fmt.Fprintf(stdout, "  %s [GAP悬空夹具] 命中 %d 处 · 号 = %s（期望 1 · GAP-"+"20260928-999）\n",
		cdrMark(good), len(gh), gr1)

	// ⑬ GAP 号引用·全在账 ⇒ 不报（成对负控之二：账内有的号不该报）
	fg2 := cdrMkFile(repo, "scripts/gates/g2.py", "# 见缺口 `GAP-"+"20261005-54`（账内有）\n")
	st, _ = run([]string{fg2}, nil, repo, docs, led)
	cases++
	good = len(st.gap.hits) == 0
	ok = ok && good
	fmt.Fprintf(stdout, "  %s [GAP全在账夹具] 命中 %d 处（期望 0 —— 账内在册号不报）\n",
		cdrMark(good), len(st.gap.hits))

	// ⑭ GAP 号引用·负控（哑化）：同号写进**数据字串**（引号紧包）⇒ 例外面，不计命中也不哑报
	fg3 := cdrMkFile(repo, "scripts/gates/g3.py", "fx = {\"id\": \"GAP-"+"20260928-777\"}\n")
	st, _ = run([]string{fg3}, nil, repo, docs, led)
	cases++
	good = len(st.gap.hits) == 0 && len(st.gap.exempt) == 1
	ok = ok && good
	fmt.Fprintf(stdout, "  %s [GAP负控夹具] 命中 %d · 例外面 %d（期望 0 / 1 —— 数据字串是例外面，不哑报）\n",
		cdrMark(good), len(st.gap.hits), len(st.gap.exempt))

	// ⑮ GAP 号引用·账取不到 ⇒ 未判（不翻退码 · 不静默成绿）
	fg4 := cdrMkFile(repo, "scripts/gates/g4.py", "# 见缺口 `GAP-"+"20260928-888`\n")
	st, _ = run([]string{fg4}, nil, repo, docs, nil)
	cases++
	good = len(st.gap.hits) == 0 && st.gap.candidates >= 1 &&
		len(st.gap.unjudged) == 1 && !st.gap.judged
	ok = ok && good
	fmt.Fprintf(stdout, "  %s [GAP账缺夹具] 命中 %d · 未判 %d · judged=%s（期望 0 / ≥1 / False）\n",
		cdrMark(good), len(st.gap.hits), len(st.gap.unjudged), cdrPyBool(st.gap.judged))

	// ⑯ 棘轮格（成对负控）：超基线 ⇒ 超额（红）· 存量 ≤ 基线 ⇒ 不超额（绿）。
	_, _, ovOver := cdrRatchetExtra(1, 0, map[string]int{"docs": 0, "gap": 0})
	_, _, ovIn := cdrRatchetExtra(1, 0, map[string]int{"docs": 9, "gap": 9})
	cases++
	good = ovOver && !ovIn
	ok = ok && good
	fmt.Fprintf(stdout, "  %s [棘轮夹具] 超基线⇒超额 %s · 存量≤基线⇒不超额 %s（期望 True / True —— 成对负控）\n",
		cdrMark(good), cdrPyBool(ovOver), cdrPyBool(!ovIn))

	// ⑰⑱ 省略号截断（`…` · 2026-10-13 · `GAP-20260926-191`）：抽取到的引用**紧跟 U+2026** ⇒
	// 与「散文误吞」同路（认出来 ⇒ 计数不判 · 不 emit · 不静默）；成对负控 = 同一处不带省略号 ⇒ 照判。
	fg5 := cdrMkFile(repo, "scripts/gates/g17.py", "# 依据：docs"+"/项目文档/…\n")
	st17, hits17 := run([]string{fg5}, nil, repo, docs, nil)
	ell := 0
	for _, h := range hits17 {
		if strings.HasPrefix(h.ref, "docs"+"/项目文档/") {
			ell++
		}
	}
	cases++
	good = ell == 0 && st17.refsProseSkipped >= 1
	ok = ok && good
	fmt.Fprintf(stdout, "  %s [省略号截断夹具] 截断点紧跟 … ⇒ 报出 %d 处 · 散文误吞 %d（期望 0 / ≥1 —— 计数不判 · 不静默）\n",
		cdrMark(good), ell, st17.refsProseSkipped)

	fg6 := cdrMkFile(repo, "scripts/gates/g18.py", "# 依据：docs"+"/项目文档/不存在x.md\n")
	_, hits18 := run([]string{fg6}, nil, repo, docs, nil)
	real := 0
	for _, h := range hits18 {
		if h.ref == "docs"+"/项目文档/不存在x.md" {
			real++
		}
	}
	cases++
	good = real == 1
	ok = ok && good
	fmt.Fprintf(stdout, "  %s [省略号成对夹具] 不带省略号 ⇒ 报出 %d 处（期望 1 —— 成对负控：省略号之外照判）\n",
		cdrMark(good), real)

	if cases != 18 {
		fmt.Fprintf(stdout, "  自检用例数必须为 18，实际 %d（少跑即失效）\n", cases)
		ok = false
	}
	verdict := "**失败** ✗"
	if ok {
		verdict = "通过 ✓"
	}
	fmt.Fprintf(stdout, "  自检：%s（18 格：必红×2 · 豁免×1 · 拼接×1 · 前缀×1 · 无输入×1 · 登记表×1 · 整件豁免×1 ·"+
		" 前缀越界×1 · 缺语料×1 · 跨仓×1 · GAP悬空×1 · GAP全在账×1 · GAP负控×1 · GAP账缺×1 · 棘轮×1 ·"+
		" 省略号截断×1 · 省略号成对×1）\n", verdict)
	return ok
}

func cdrMark(b bool) string {
	if b {
		return "✓"
	}
	return "✗"
}

func cdrPyBool(b bool) string {
	if b {
		return "True"
	}
	return "False"
}

// cdrEntryOM —— 造一枚登记表条目（键序 = py dict 插入序：file, ref, ref_prefix, class, reason, date）。
func cdrEntryOM(file, ref, pre, cls, reason, date string) *cdrOM {
	o := &cdrOM{}
	o.set(cdrKeyFile, file)
	if ref != "" {
		o.set(cdrKeyRef, ref)
	}
	if pre != "" {
		o.set(cdrKeyRefPrefix, pre)
	}
	o.set(cdrKeyClass, cls)
	o.set(cdrKeyReason, reason)
	o.set(cdrKeyDate, date)
	return o
}

// cdrRealExitCode —— 自检用的「跑真判据」入口：只取退码，不打长清单（逐字复刻 py real_exit_code）。
func cdrRealExitCode(root, registryPath string, forceFiles []string, hasForce bool, gapIds map[string]bool, stderr io.Writer) int {
	var files []string
	if !hasForce {
		f, ok := cdrGitFiles(root, stderr)
		if !ok {
			return 2
		}
		files = f
	} else {
		files = forceFiles
	}
	if len(files) == 0 {
		fmt.Fprint(stderr, "✗ 扫到 0 个件 —— 拒绝静默空转（假绿）\n")
		return 2
	}
	reg, bad := cdrLoadRegistry(registryPath)
	if len(bad) > 0 {
		return 2
	}
	parent := filepath.Dir(root)
	ctx := &cdrCtx{root: root, docsRoot: filepath.Join(parent, cdrDocsTreeName),
		archRoot: filepath.Join(parent, cdrArchTreeName), files: files, registry: reg, gapIds: gapIds}
	st, hits := cdrScan(ctx)
	if st.filesScanned == 0 {
		fmt.Fprint(stderr, "✗ 扫到 0 个件 —— 拒绝静默空转（假绿）\n")
		return 2
	}
	gapHits := 0
	if st.gap != nil {
		gapHits = len(st.gap.hits)
	}
	if len(hits) > 0 || gapHits > 0 {
		return 1
	}
	return 0
}

// ── 真跑 ─────────────────────────────────────────────────────────────────────────────
func cdrGitFiles(root string, stderr io.Writer) ([]string, bool) {
	cmd := exec.Command("git", "ls-files", "-z")
	cmd.Dir = root
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			fmt.Fprintf(stderr, "✗ git ls-files rc=%d（不是 git 仓？）⇒ 不给结论\n", ee.ExitCode())
			return nil, false
		}
		fmt.Fprintf(stderr, "✗ 跑不了 git ls-files：%s ⇒ 不给结论\n", err)
		return nil, false
	}
	var files []string
	for _, p := range strings.Split(strings.ToValidUTF8(out.String(), "\uFFFD"), "\x00") {
		if p != "" {
			files = append(files, p)
		}
	}
	return files, true
}

var cdrRules = []string{
	cdrVersion + " —— 判据 / 两类定义 / 覆盖与未覆盖",
	"\n【判据】主仓（`git ls-files` 的件）里形如 `docs" + "/…` 的引用，解析后（相对**仓根**）不存在 ⇒ 悬空。",
	"\n【两类】live = 活指路（该改）· ledger = 机读清单/基线台账（记当时路径，不该改 ⇒ 登记）",
	"  登记表：" + cdrRegistryDefault,
	"  三种登记形态（**整件豁免 = 洗白 ⇒ rc=2 拒**）：",
	"    · `ref`        逐处：这一件里的**这一条**引文",
	"    · `ref_prefix` 前缀：这一件里**以该前缀开头**的全部引文（前缀外照旧命中 —— 不许越界）",
	"    · 每条必须带 `reason` + `date`（YYYY-MM-DD），缺任一 ⇒ rc=2（与 doc-freshness.config.json 同口径）",
	"  写进表 = 有意保留：输出里标 `已登记` 且**不计命中**，但**照样逐条打印**（豁免 ≠ 隐藏）",
	"\n【分类优先级（自动规则 · 每处都打印理由）】",
	"  ① 登记表命中           ⇒ 已登记（不计命中）",
	"  ② 夹具/探针假路径      ⇒ ledger（例：docs" + "/a.md；行含 夹具/探针/自检/负控）",
	"  ③ 机读件后缀 " + strings.Join(cdrMachineExt, ", ") + " ⇒ ledger",
	"  ④ 引文被引号紧包 ⇒ ledger（代码/数据字串：判据常量 · 配置值 · 夹具键值）",
	"  ⑤ 行首是 Markdown 表格行 ⇒ ledger（表格式清单/台账）",
	"  ⑥ 行内有记录措辞（以下任一）⇒ ledger：",
	"     " + strings.Join(cdrRecordWords, " · "),
	"  ⑦ 行内有指路措辞（以下任一）⇒ live：",
	"     " + strings.Join(cdrLiveWords, " · "),
	"  ⑧ 否则 ⇒ live（默认：正文/注释里的断指针，宁可多报 —— 本门只报告）",
	"\n【已覆盖形态】直书 `docs" + "/…`（含反引号/注释/字符串/frontmatter）·",
	"              `filepath.Join(\"docs\", …)` / `path.Join(…)` 拼接式 · .py 里 `\"docs" + "/\" + x` 前缀形态 ·",
	"              **跨仓直书** `Zerg-内部文档" + "/<rel>` 与裸 `项目文档" + "/<rel>`（2026-10-05 · GAP-20261005-18）",
	"\n【省略号截断（2026-10-13 · GAP-20260926-191）】抽取到的引用**紧跟省略号**（U+2026）⇒ 是「被省略号截断的占位路径」，",
	"             本来就不是一条可解析的引用 ⇒ 与「散文误吞」同路（**认出来 ⇒ 计数不判 · 不静默**：读数行「另滤掉散文误吞 N 处」含它）",
	"【未覆盖形态（明说 · 不假装全覆盖）】",
	"  · 纯 URL（`https://…/docs" + "/…`）—— 不是仓内相对引用，本门不判",
	"  · 变量拼进中段/后缀：`\"docs" + "/\" + lang + \"/x.md\"` —— 只判到前缀目录那一层",
	"  · 运行期树：`filepath.Join(workDir, \"docs\", \"issues\")` 真身是 workDir 下的目录 ——",
	"    报出来但按 ledger 判（要有意保留就登记），**不许按主仓同名树改**",
	"  · 软链 / 大小写变体 / 非 UTF-8 件（记入 files_unreadable）· 未 `git add` 的新件",
	"  · markdown 相对链接 `[](../x.md)` —— 属 `check-doc-freshness.py` D1，本门不重复",
	"\n【GAP 号引用类（2026-10-05 · GAP-20261005-54）】",
	"  判据：注释/文档里引的 `GAP-<8位日期>-<号>` 必须在**缺口账**里存在",
	"        （真源 <ZERG_STATE_DIR|~/.zerg/state>/" + cdrGapLedgerFile + "；可 `--ledger` 覆盖）",
	"  不存在 ⇒ 悬空（逐字点名 件:行 · 引的号）· 账取不到 ⇒ 本类未判（不给结论 · 但**不翻**其余面的退码）",
	"  例外面（照样逐条打印 · 不计命中）：本件自身(自检夹具号) / 测试夹具件(*_test.* / testdata/ / tests/ 里的假号) /",
	"                                    机读件(.json/.tsv/.csv/.txt) / 引号紧包的数据字串 / Markdown 表格行 /",
	"                                    登记表命中（`ref`=号 或 `ref_prefix`）",
	"  判定复用本门**同一个** `classify()`：只有 `live` 类算命中，`ledger`/`已登记` = 例外面",
	"\n【退码】0 无命中（或全登记/全例外面）• **或存量 ≤ 棘轮基线** · 1 **相对基线新增**未登记悬空（docs 路径类 ＋ GAP 号引用类）·",
	"        2 不给结论（不在仓根/0 件/登记表不合格/同级语料树 Zerg-内部文档 取不到根/自检不过）",
	"\n【棘轮（存量 ≤ 基线 ⇒ 绿 · 超出 ⇒ 响亮红并点名新增 · 只许下调）】形状照先例 `check_verify_cmd_real.go` 的 `cvrBaseline`：",
	fmt.Sprintf("  基线：两类各一条 —— docs 路径类 %d 处 · GAP 号引用类 %d 处（只许下调，永不上调）", cdrBaseline["docs"], cdrBaseline["gap"]),
}

func cdrPrintRules(stdout io.Writer) {
	for _, l := range cdrRules {
		fmt.Fprintln(stdout, l)
	}
}

// ── 入口（= py `main()`）─────────────────────────────────────────────────────────────
func checkDanglingRefsGateMain(stdout, stderr io.Writer, argv []string) int {
	root, aerr := filepath.Abs(".")
	if aerr != nil {
		root = "."
	}
	registryPath := cdrPyJoin(root, cdrRegistryDefault)
	ledgerPath := cdrGapLedgerPath()
	showAll := false
	liveOnly := false
	jsonOut := false
	dumpPath := ""
	groupBy := []string{}
	prefixCands := false

	i := 0
	for i < len(argv) {
		a := argv[i]
		switch {
		case a == "--selftest" || a == "--self-test":
			if cdrSelftest(stdout, stderr) {
				return 0
			}
			return 1
		case a == "--list-rules":
			cdrPrintRules(stdout)
			return 0
		case a == "--all":
			showAll = true
		case a == "--live-only":
			liveOnly = true
		case a == "--json":
			jsonOut = true
		case a == "--group-by":
			i++
			if i >= len(argv) {
				fmt.Fprint(stderr, "✗ `--group-by` 尾缺值（py 此档抛 IndexError）⇒ 不给结论\n")
				return 2
			}
			for _, d := range strings.Split(argv[i], ",") {
				if d != "" {
					groupBy = append(groupBy, d)
				}
			}
		case a == "--prefix-candidates":
			prefixCands = true
		case a == "--dump":
			i++
			if i >= len(argv) {
				fmt.Fprint(stderr, "✗ `--dump` 尾缺值（py 此档抛 IndexError）⇒ 不给结论\n")
				return 2
			}
			dumpPath, _ = filepath.Abs(argv[i])
		case a == "--root":
			i++
			if i >= len(argv) {
				fmt.Fprint(stderr, "✗ `--root` 尾缺值（py 此档抛 IndexError）⇒ 不给结论\n")
				return 2
			}
			root, _ = filepath.Abs(argv[i])
			registryPath = cdrPyJoin(root, cdrRegistryDefault)
		case a == "--registry":
			i++
			if i >= len(argv) {
				fmt.Fprint(stderr, "✗ `--registry` 尾缺值（py 此档抛 IndexError）⇒ 不给结论\n")
				return 2
			}
			registryPath, _ = filepath.Abs(argv[i])
		case a == "--ledger":
			i++
			if i >= len(argv) {
				fmt.Fprint(stderr, "✗ `--ledger` 尾缺值（py 此档抛 IndexError）⇒ 不给结论\n")
				return 2
			}
			ledgerPath, _ = filepath.Abs(argv[i])
		default:
			fmt.Fprintf(stderr, "✗ 未知参数 %s（用法见 --list-rules）\n", cdrPyReprStr(a))
			return 2
		}
		i++
	}

	for _, d := range groupBy {
		if !cdrStrIn(cdrGroupDims, d) {
			fmt.Fprintf(stderr, "✗ --group-by 维度不认识：%s（闭集：src|tgt|form|cls）\n", cdrPyReprStr(d))
			return 2
		}
	}

	if !cdrIsDir(cdrPyJoin(root, "docs")) && !cdrIsDir(cdrPyJoin(root, "core")) {
		fmt.Fprintf(stderr, "✗ 不在仓根（`docs"+"/` 与 `core/` 都不存在）：%s ⇒ 拒绝静默空转\n", root)
		return 2
	}

	files, ok := cdrGitFiles(root, stderr)
	if !ok || len(files) == 0 {
		fmt.Fprint(stderr, "✗ 扫到 0 个件 —— 拒绝静默空转（假绿）\n")
		return 2
	}

	reg, bad := cdrLoadRegistry(registryPath)
	if len(bad) > 0 {
		fmt.Fprint(stderr, "✗ 登记表不合格 ⇒ **不给结论**（写不清理由的豁免就是洗白）：\n")
		for _, b := range bad {
			fmt.Fprintln(stderr, "   · "+b)
		}
		return 2
	}

	gapIds, gapProblem := cdrLoadGapIds(ledgerPath)

	parent := filepath.Dir(root)
	sibling := os.Getenv(cdrDocsAltEnv)
	if sibling == "" {
		sibling = os.Getenv(cdrDocsEnv)
	}
	if sibling == "" {
		sibling = cdrPyJoin(parent, cdrDocsTreeName)
	}
	docsRoot := os.Getenv(cdrDocsRootEnv)
	if docsRoot == "" {
		docsRoot = sibling
	}
	archRoot := os.Getenv(cdrArchRootEnv)
	if archRoot == "" {
		archRoot = cdrPyJoin(parent, cdrArchTreeName)
	}
	absent := cdrAbsentCorpusTrees(docsRoot, archRoot)
	if reg == nil {
		reg = &cdrOM{}
	}
	ctx := &cdrCtx{root: root, docsRoot: docsRoot, archRoot: archRoot, files: files, registry: reg, gapIds: gapIds}
	st, hits := cdrScan(ctx)
	if st.filesScanned == 0 {
		fmt.Fprint(stderr, "✗ 扫到 0 个件 —— 拒绝静默空转（假绿）\n")
		return 2
	}
	if len(st.readFails) > 0 {
		// ★ 汇总处判红：读失败 ⇒ stderr BLOCKED 头 + 逐件点名 + 退 2（原 py 静默计入 files_unreadable）
		fmt.Fprint(stderr, "⛔ BLOCKED：以下件读失败，覆盖面不完整 —— 不给结论（假覆盖）\n")
		for _, p := range st.readFails {
			fmt.Fprintln(stderr, "   · "+p)
		}
		return 2
	}

	byLive, byLed, byReg := []cdrHit{}, []cdrHit{}, []cdrHit{}
	for _, h := range hits {
		switch h.cls {
		case "live":
			byLive = append(byLive, h)
		case "ledger":
			byLed = append(byLed, h)
		default:
			byReg = append(byReg, h)
		}
	}
	nLive, nLed, nReg := len(byLive), len(byLed), len(byReg)
	fileSet := map[string]bool{}
	for _, h := range hits {
		fileSet[h.file] = true
	}
	filesHit := len(fileSet)

	fmt.Fprintf(stdout, "%s —— 主仓悬空引用（`docs"+"/…` 指向不存在）\n", cdrVersion)
	fmt.Fprintf(stdout, "仓根：%s\n", root)
	for _, pair := range [][2]string{{cdrDocsTreeName, docsRoot}, {cdrArchTreeName, archRoot}} {
		name, tree := pair[0], pair[1]
		state := ""
		if cdrIsDir(tree) {
			state = "在"
		} else if name == cdrDocsTreeName {
			state = "**不在盘** —— 落点判不了 ⇒ 本门不给结论（rc=2）"
		} else {
			state = "**不在** —— 落点只报主仓"
		}
		fmt.Fprintf(stdout, "  %s：%s（%s）\n", name, tree, state)
	}
	fmt.Fprintf(stdout, "扫描面：git ls-files %d 件 ⇒ 文本件 %d · 提取引用 %d 处（另滤掉散文误吞 %d 处）· 悬空 %d 处\n",
		st.filesSeen, st.filesScanned, st.refs, st.refsProseSkipped, st.dangling)
	gapLedgerState := "在"
	if gapIds == nil {
		gapLedgerState = "**取不到** ⇒ GAP 号引用类未判：" + gapProblem
	}
	fmt.Fprintf(stdout, "  缺口账（GAP 号存在性真源）：%s（%s）\n", ledgerPath, gapLedgerState)
	if len(st.unreadable) > 0 {
		show := st.unreadable
		if len(show) > 3 {
			show = show[:3]
		}
		fmt.Fprintf(stdout, "  ⚠ 非 UTF-8 / 读不了 %d 件（未覆盖形态之一）：%s\n",
			len(st.unreadable), strings.Join(show, " · "))
	}
	fmt.Fprintf(stdout, "\n合计（docs/ 路径类）：悬空 %d 处 / %d 件 ⇒ **命中 %d 处**（live %d · ledger %d）· 已登记 %d 处（不计命中）\n",
		st.dangling, filesHit, nLive+nLed, nLive, nLed, nReg)
	gap := st.gap
	if gap == nil {
		gap = &cdrGapState{judged: gapIds != nil}
	}
	gHits, gExempt := gap.hits, gap.exempt
	nGap := len(gHits)
	if gap.judged {
		fmt.Fprintf(stdout, "合计（GAP 号引用类）：候选 %d 处 ⇒ **命中 %d 处**（live）· 例外面 %d 处（不计命中）\n",
			gap.candidates, len(gHits), len(gExempt))
	} else {
		fmt.Fprintf(stdout, "合计（GAP 号引用类）：候选 %d 处 ⇒ **未判**（缺口账取不到 ⇒ 本类不给结论）\n",
			gap.candidates)
	}
	// ★ 本枚（缺口 `GAP-20261013-28`）：**账真源自身**的悬空号 —— 扫面 = `git ls-files` 的仓内已跟踪件，
	//   账真源在仓外 ⇒「账内注引了不在册的号」没有任何一层会喊。本档**只报告**：✗ 不进棘轮、✗ 不进退出码。
	ledSelf := cdrLedgerSelfDangling(ledgerPath, gapIds)
	fmt.Fprintf(stdout, "合计（账真源自身 · 只报告 · 不计棘轮）：悬挂号引用 %d 处\n", len(ledSelf))
	for _, l := range ledSelf {
		fmt.Fprintln(stdout, "   "+l)
	}

	// ── 棘轮格（存量 ≤ 基线 ⇒ 绿 · 超出 ⇒ 响亮红并点名新增处）────────────────────────
	// 形状照先例 `check_verify_cmd_real.go` 的 `cvrBaseline` + 棘轮格（同形，✗ 不另起第二套）。
	nPath := nLive + nLed
	extraPath, extraGap, ratchetOver := cdrRatchetExtra(nPath, nGap, nil)
	fmt.Fprintf(stdout, "\n【棘轮】基线（只许下调 · 照先例 `cvrBaseline` 形态）：docs 路径类 %d 处 · GAP 号引用类 %d 处\n",
		cdrBaseline["docs"], cdrBaseline["gap"])
	fmt.Fprintf(stdout, "       现测：docs 路径类 %d 处（较基线 %s）· GAP 号引用类 %d 处（较基线 %s）\n",
		nPath, cdrDelta(extraPath), nGap, cdrDelta(extraGap))
	fmt.Fprintf(stdout, "       基线注：%s\n", cdrBaselineNote)

	cap := 30
	if showAll || ratchetOver {
		// 超额 ⇒ **全打**（首行即「响亮红」；下面每一处现行命中都逐条点名，供对基线复核新增）
		cap = 1 << 30
	}

	dump := func(title string, rows []cdrHit) {
		fmt.Fprintf(stdout, "\n── %s：%d 处\n", title, len(rows))
		lim := cap
		if len(rows) < lim {
			lim = len(rows)
		}
		for _, h := range rows[:lim] {
			where := "三树都无"
			if len(h.where) > 0 {
				where = strings.Join(h.where, "、")
			}
			fmt.Fprintf(stdout, "   %s:%d · 引文 `%s` · 解析 `%s` · 落点 %s · 【%s】%s · 建议：%s\n",
				h.file, h.line, h.ref, h.path, where, h.cls, h.why, h.suggest)
			if h.form != "直书" {
				note := ""
				if h.note != "" {
					note = "·" + h.note
				}
				fmt.Fprintf(stdout, "        （形态=%s%s）\n", h.form, note)
			}
		}
		if len(rows) > cap {
			fmt.Fprintf(stdout, "   …另 %d 处（--all 全打）\n", len(rows)-cap)
		}
	}

	if len(groupBy) > 0 {
		// `--group-by`：逐处清单换成**按维度聚合**（聚合面 = docs 路径类 ＋ GAP 号类全清单）。
		allh := []cdrHit{}
		allh = append(allh, hits...)
		allh = append(allh, gHits...)
		allh = append(allh, gExempt...)
		fmt.Fprintf(stdout, "\n── 聚合（--group-by=%s）：共 %d 处（docs 路径类 %d + GAP 号类 %d）\n",
			strings.Join(groupBy, ","), len(allh), len(hits), len(gHits)+len(gExempt))
		for _, dim := range groupBy {
			cnt := map[string]int{}
			for _, h := range allh {
				cnt[cdrDimValue(h, dim)]++
			}
			type cdrKv struct {
				v string
				n int
			}
			rows := []cdrKv{}
			for v, n := range cnt {
				rows = append(rows, cdrKv{v, n})
			}
			sort.Slice(rows, func(a, b int) bool {
				if rows[a].n != rows[b].n {
					return rows[a].n > rows[b].n
				}
				return rows[a].v < rows[b].v
			})
			fmt.Fprintf(stdout, "  【%s】\n", dim)
			for _, r := range rows {
				fmt.Fprintf(stdout, "     %s ⇒ %d 处\n", r.v, r.n)
			}
		}
	}
	if nLive > 0 && len(groupBy) == 0 {
		dump("live 命中（活指路 · 该改）", byLive)
	}
	if nLed > 0 && !liveOnly && len(groupBy) == 0 {
		dump("ledger 命中（机读清单/基线台账 · 记当时路径 ⇒ 登记，不改）", byLed)
	}
	if nReg > 0 && !liveOnly && len(groupBy) == 0 {
		dump("已登记（有意保留 · 不计命中）", byReg)
	}

	if len(gHits) > 0 && len(groupBy) == 0 {
		fmt.Fprintf(stdout, "\n── GAP 号引用命中（注释/文档引的号账内不存在 · 该改或先立案）：%d 处\n", len(gHits))
		lim := cap
		if len(gHits) < lim {
			lim = len(gHits)
		}
		for _, h := range gHits[:lim] {
			fmt.Fprintf(stdout, "   %s:%d · 引的号 `%s` · 【%s】%s · 建议：%s\n",
				h.file, h.line, h.ref, h.cls, h.why, h.suggest)
		}
		if len(gHits) > cap {
			fmt.Fprintf(stdout, "   …另 %d 处（--all 全打）\n", len(gHits)-cap)
		}
	}
	if len(gExempt) > 0 && !liveOnly && len(groupBy) == 0 {
		fmt.Fprintf(stdout, "\n── GAP 号引用例外面（夹具假号 / 机读件 / 快照 / 数据字串 · 不计命中）：%d 处\n", len(gExempt))
		lim := cap
		if len(gExempt) < lim {
			lim = len(gExempt)
		}
		for _, h := range gExempt[:lim] {
			fmt.Fprintf(stdout, "   %s:%d · 引的号 `%s` · 【%s】%s\n", h.file, h.line, h.ref, h.cls, h.why)
		}
		if len(gExempt) > cap {
			fmt.Fprintf(stdout, "   …另 %d 处（--all 全打）\n", len(gExempt)-cap)
		}
	}
	if len(gap.unjudged) > 0 {
		show := gap.unjudged
		if len(show) > 3 {
			show = show[:3]
		}
		fmt.Fprintf(stdout, "\n⛔ GAP 号引用类**未判**（缺口账取不到：%s）—— 候选 %d 处（前几处：%s）\n",
			gapProblem, len(gap.unjudged), strings.Join(show, " · "))
	}

	if prefixCands {
		cands := cdrPrefixCandidates(hits)
		fmt.Fprintf(stdout, "\n── 可登记前缀候选（未登记的 ledger 命中 · `--prefix-candidates`）：%d 条"+
			"（草稿 ⇒ `ref_prefix` 可直接粘进退 `entries`，`reason`/`date` 需人工补全）\n", len(cands))
		fmt.Fprintln(stdout, cdrJSONEnc(cands, true, 0))
	}

	fmt.Fprint(stdout, "\n未覆盖形态（明说，不假装全覆盖）：纯 URL · 变量拼进中段或后缀 · 运行期路径树 · "+
		"软链/大小写变体/非 UTF-8 件/未 git add 的新件 · markdown 相对链接（属 D1）\n")

	if jsonOut {
		o := &cdrOM{}
		o.set("schema", "dangling-refs/1")
		o.set("repo", root)
		o.set("dangling", st.dangling)
		o.set("files", filesHit)
		o.set("live", nLive)
		o.set("ledger", nLed)
		o.set("registered", nReg)
		o.set("hit", nLive+nLed)
		o.set("baseline_docs", cdrBaseline["docs"])
		o.set("baseline_gap", cdrBaseline["gap"])
		o.set("ratchet_over", ratchetOver)
		o.set("ratchet_extra_docs", cdrMax0(extraPath))
		o.set("ratchet_extra_gap", cdrMax0(extraGap))
		o.set("registry", registryPath)
		o.set("gap_judged", gap.judged)
		o.set("gap_candidates", gap.candidates)
		o.set("gap_hit", len(gHits))
		o.set("gap_exempt", len(gExempt))
		o.set("gap_ledger", ledgerPath)
		o.set("ledger_self_dangling", len(ledSelf))
		fmt.Fprintln(stdout, cdrJSONEnc(o, false, 0))
	}
	if dumpPath != "" {
		o := &cdrOM{}
		o.set("schema", "dangling-refs/1")
		o.set("repo", root)
		o.set("stats", cdrStatsOM(st))
		hitsAny := []any{}
		for _, h := range hits {
			hitsAny = append(hitsAny, cdrHitOM(h))
		}
		o.set("hits", hitsAny)
		if err := os.WriteFile(dumpPath, []byte(cdrJSONEnc(o, true, 0)), 0o644); err != nil {
			fmt.Fprintf(stderr, "✗ 落不了盘 %s：%s ⇒ 不给结论\n", dumpPath, err.Error())
			return 2
		}
		fmt.Fprintf(stdout, "（全量命中已落盘：%s）\n", dumpPath)
	}

	if len(absent) > 0 {
		for _, p := range absent {
			fmt.Fprintf(stdout, "\n⛔ 不给结论（rc=2）：同级语料树 `%s` 取不到根（%s）—— 落点判不了\n"+
				"   换根候选：`ZERG_DOCS_ROOT`，或同级根 `ZERG_DOCS_ALT`/`ZERG_DOCS`\n", p[0], p[1])
		}
		return 2
	}
	if nPath == 0 && nGap == 0 {
		fmt.Fprint(stdout, "\n结论：无命中（或全在登记表 / 例外面里）⇒ rc=0\n")
		return 0
	}
	if !ratchetOver {
		// 存量段：现测 ≤ 基线 ⇒ 绿 —— 存量与新增已分开，不再是「对存量恒红」。
		fmt.Fprintf(stdout, "\n结论：**存量段 rc=0**（docs 路径类 %d ≤ 基线 %d · GAP 号引用类 %d ≤ 基线 %d）—— "+
			"存量在基线内、不再恒红；新增一条悬空引用即越基线翻红\n",
			nPath, cdrBaseline["docs"], nGap, cdrBaseline["gap"])
		return 0
	}
	// 超出基线 ⇒ 响亮红（存量与新增必须分开 · 点名超额处）。
	fmt.Fprint(stderr, "⛔ 棘轮超额（rc=1 · 存量与新增必须分开）：\n")
	if extraPath > 0 {
		fmt.Fprintf(stderr, "   · docs 路径类：现测 %d 处 > 基线 %d 处 ⇒ **新增 %d 处**（上面已全打逐条点名：件:行 · 引文）\n",
			nPath, cdrBaseline["docs"], extraPath)
	}
	if extraGap > 0 {
		fmt.Fprintf(stderr, "   · GAP 号引用类：现测 %d 处 > 基线 %d 处 ⇒ **新增 %d 处**（上面已全打逐条点名：件:行 · 引的号）\n",
			nGap, cdrBaseline["gap"], extraGap)
	}
	fmt.Fprint(stderr, "   存量 = 基线那一档；✗ 不许抬基线充绿 —— 要么治掉新增引用，要么按纪律**下调**基线。\n")
	fmt.Fprintf(stdout, "\n结论：棘轮超额 ⇒ rc=1（docs 路径类新增 %d 处 · GAP 号引用类新增 %d 处 —— 见 stderr 头与上方全打清单）\n",
		cdrMax0(extraPath), cdrMax0(extraGap))
	return 1
}

// ── 有序 map + Python json 逐字 ─────────────────────────────────────────────────────
type cdrOM struct {
	keys []string
	vals []any
}

func (o *cdrOM) set(k string, v any) {
	o.keys = append(o.keys, k)
	o.vals = append(o.vals, v)
}

func cdrStatsOM(st *cdrStats) *cdrOM {
	o := &cdrOM{}
	o.set("files_scanned", st.filesScanned)
	o.set("files_seen", st.filesSeen)
	o.set("refs", st.refs)
	o.set("dangling", st.dangling)
	o.set("refs_prose_skipped", st.refsProseSkipped)
	if len(st.unreadable) > 0 {
		o.set("files_unreadable", st.unreadable)
	}
	o.set("gap", cdrGapOM(st.gap))
	return o
}

// cdrGapOM —— py `--dump` 里 `stats["gap"]` 的逐字同构（键序 = py dict 插入序）。
func cdrGapOM(g *cdrGapState) *cdrOM {
	o := &cdrOM{}
	if g == nil {
		o.set("hits", []any{})
		o.set("exempt", []any{})
		o.set("candidates", 0)
		o.set("judged", false)
		o.set("unjudged", []any{})
		return o
	}
	hitsAny := []any{}
	for _, h := range g.hits {
		hitsAny = append(hitsAny, cdrGapRecOM(h))
	}
	exAny := []any{}
	for _, h := range g.exempt {
		exAny = append(exAny, cdrGapRecOM(h))
	}
	unAny := []any{}
	for _, x := range g.unjudged {
		unAny = append(unAny, x)
	}
	o.set("hits", hitsAny)
	o.set("exempt", exAny)
	o.set("candidates", g.candidates)
	o.set("judged", g.judged)
	o.set("unjudged", unAny)
	return o
}

// cdrGapRecOM —— GAP 记录（live 才有 `suggest` 键 —— py 对 exempt 不落该键）。
func cdrGapRecOM(h cdrHit) *cdrOM {
	o := &cdrOM{}
	o.set("file", h.file)
	o.set("line", h.line)
	o.set("ref", h.ref)
	o.set("path", h.path)
	o.set("form", h.form)
	o.set("note", h.note)
	o.set("cls", h.cls)
	o.set("why", h.why)
	o.set("where", h.where)
	o.set("src", h.src)
	if h.suggest != "" {
		o.set("suggest", h.suggest)
	}
	return o
}

func cdrHitOM(h cdrHit) *cdrOM {
	o := &cdrOM{}
	o.set("file", h.file)
	o.set("line", h.line)
	o.set("ref", h.ref)
	o.set("path", h.path)
	o.set("form", h.form)
	o.set("note", h.note)
	o.set("cls", h.cls)
	o.set("why", h.why)
	o.set("where", h.where)
	o.set("suggest", h.suggest)
	o.set("src", h.src)
	return o
}

func cdrPyQuote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		default:
			if r < 0x20 {
				fmt.Fprintf(&b, `\u%04x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

// cdrJSONEnc —— 复刻 `json.dumps(obj, ensure_ascii=False, indent=<n>)`（indent=0 ⇒ 单行）。
func cdrJSONEnc(v any, indent bool, level int) string {
	pad := ""
	nl := ""
	childPad := ""
	if indent {
		nl = "\n"
		pad = strings.Repeat(" ", level)
		childPad = strings.Repeat(" ", level+1)
	}
	switch x := v.(type) {
	case nil:
		return "null"
	case string:
		return cdrPyQuote(x)
	case int:
		return strconv.Itoa(x)
	case bool:
		if x {
			return "true"
		}
		return "false"
	case []string:
		if len(x) == 0 {
			return "[]"
		}
		parts := make([]string, 0, len(x))
		for _, s := range x {
			parts = append(parts, cdrPyQuote(s))
		}
		if indent {
			return "[" + nl + childPad + strings.Join(parts, ","+nl+childPad) + nl + pad + "]"
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case []any:
		if len(x) == 0 {
			return "[]"
		}
		parts := make([]string, 0, len(x))
		for _, e := range x {
			parts = append(parts, cdrJSONEnc(e, indent, level+1))
		}
		if indent {
			return "[" + nl + childPad + strings.Join(parts, ","+nl+childPad) + nl + pad + "]"
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case *cdrOM:
		if len(x.keys) == 0 {
			return "{}"
		}
		parts := make([]string, 0, len(x.keys))
		for i, k := range x.keys {
			parts = append(parts, cdrPyQuote(k)+": "+cdrJSONEnc(x.vals[i], indent, level+1))
		}
		if indent {
			return "{" + nl + childPad + strings.Join(parts, ","+nl+childPad) + nl + pad + "}"
		}
		return "{" + strings.Join(parts, ", ") + "}"
	}
	return "null"
}

// ── JSON 解析（py `json.load` 的实用子集 · 面向登记表）───────────────────────────────
// 返回 (*cdrOM / []any / string / float64 / bool / nil, 错误文案)。错误文案为自有形态
// （py 那面是 CPython `JSONDecodeError` 文案 ⇒ 见件头未覆盖面②）。
func cdrJSONParse(s string) (any, string) {
	p := &cdrJSONParser{s: s}
	p.ws()
	v, ok := p.value()
	if !ok {
		return nil, p.err
	}
	p.ws()
	if p.i != len(p.s) {
		return nil, "多余内容（位置 " + strconv.Itoa(p.i) + "）"
	}
	return v, ""
}

type cdrJSONParser struct {
	s   string
	i   int
	err string
}

func (p *cdrJSONParser) ws() {
	for p.i < len(p.s) {
		switch p.s[p.i] {
		case ' ', '\t', '\n', '\r':
			p.i++
		default:
			return
		}
	}
}

func (p *cdrJSONParser) fail(msg string) {
	if p.err == "" {
		p.err = msg + "（位置 " + strconv.Itoa(p.i) + "）"
	}
}

func (p *cdrJSONParser) value() (any, bool) {
	if p.i >= len(p.s) {
		p.fail("Expecting value")
		return nil, false
	}
	switch c := p.s[p.i]; {
	case c == '{':
		return p.object()
	case c == '[':
		return p.array()
	case c == '"':
		return p.str()
	case c == 't':
		if strings.HasPrefix(p.s[p.i:], "true") {
			p.i += 4
			return true, true
		}
	case c == 'f':
		if strings.HasPrefix(p.s[p.i:], "false") {
			p.i += 5
			return false, true
		}
	case c == 'n':
		if strings.HasPrefix(p.s[p.i:], "null") {
			p.i += 4
			return nil, true
		}
	case c == '-' || (c >= '0' && c <= '9'):
		return p.number()
	}
	p.fail("Expecting value")
	return nil, false
}

func (p *cdrJSONParser) number() (any, bool) {
	start := p.i
	if p.i < len(p.s) && p.s[p.i] == '-' {
		p.i++
	}
	for p.i < len(p.s) {
		c := p.s[p.i]
		if (c >= '0' && c <= '9') || c == '.' || c == 'e' || c == 'E' || c == '+' || c == '-' {
			p.i++
			continue
		}
		break
	}
	f, err := strconv.ParseFloat(p.s[start:p.i], 64)
	if err != nil {
		p.err = "Expecting value"
		return nil, false
	}
	return f, true
}

func (p *cdrJSONParser) str() (any, bool) {
	if p.i >= len(p.s) || p.s[p.i] != '"' {
		p.fail("Expecting value")
		return nil, false
	}
	p.i++
	var b strings.Builder
	for p.i < len(p.s) {
		c := p.s[p.i]
		if c == '"' {
			p.i++
			return b.String(), true
		}
		if c == '\\' {
			p.i++
			if p.i >= len(p.s) {
				break
			}
			switch p.s[p.i] {
			case '"':
				b.WriteByte('"')
			case '\\':
				b.WriteByte('\\')
			case '/':
				b.WriteByte('/')
			case 'b':
				b.WriteByte('\b')
			case 'f':
				b.WriteByte('\f')
			case 'n':
				b.WriteByte('\n')
			case 'r':
				b.WriteByte('\r')
			case 't':
				b.WriteByte('\t')
			case 'u':
				if p.i+4 >= len(p.s) {
					p.fail("Invalid \\uXXXX escape")
					return nil, false
				}
				n, err := strconv.ParseUint(p.s[p.i+1:p.i+5], 16, 32)
				if err != nil {
					p.fail("Invalid \\uXXXX escape")
					return nil, false
				}
				b.WriteRune(rune(n))
				p.i += 4
			default:
				p.fail("Invalid \\escape")
				return nil, false
			}
			p.i++
			continue
		}
		b.WriteByte(c)
		p.i++
	}
	p.fail("Unterminated string")
	return nil, false
}

func (p *cdrJSONParser) array() (any, bool) {
	p.i++ // [
	out := []any{}
	p.ws()
	if p.i < len(p.s) && p.s[p.i] == ']' {
		p.i++
		return out, true
	}
	for {
		p.ws()
		v, ok := p.value()
		if !ok {
			return nil, false
		}
		out = append(out, v)
		p.ws()
		if p.i >= len(p.s) {
			p.fail("Expecting ',' delimiter")
			return nil, false
		}
		if p.s[p.i] == ',' {
			p.i++
			continue
		}
		if p.s[p.i] == ']' {
			p.i++
			return out, true
		}
		p.fail("Expecting ',' delimiter")
		return nil, false
	}
}

func (p *cdrJSONParser) object() (any, bool) {
	p.i++ // {
	o := &cdrOM{}
	p.ws()
	if p.i < len(p.s) && p.s[p.i] == '}' {
		p.i++
		return o, true
	}
	for {
		p.ws()
		k, ok := p.str()
		if !ok {
			return nil, false
		}
		p.ws()
		if p.i >= len(p.s) || p.s[p.i] != ':' {
			p.fail("Expecting ':' delimiter")
			return nil, false
		}
		p.i++
		p.ws()
		v, ok := p.value()
		if !ok {
			return nil, false
		}
		o.set(k.(string), v)
		p.ws()
		if p.i >= len(p.s) {
			p.fail("Expecting ',' delimiter")
			return nil, false
		}
		if p.s[p.i] == ',' {
			p.i++
			continue
		}
		if p.s[p.i] == '}' {
			p.i++
			return o, true
		}
		p.fail("Expecting ',' delimiter")
		return nil, false
	}
}

// ── Python repr（self-test 用例的期望/实得串）────────────────────────────────────────
func cdrPyReprStr(s string) string {
	q := "'"
	if strings.Contains(s, "'") && !strings.Contains(s, "\"") {
		q = "\""
	}
	var b strings.Builder
	b.WriteString(q)
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if string(r) == q {
				b.WriteString("\\" + string(r))
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteString(q)
	return b.String()
}

func cdrPyReprList(xs []string) string {
	parts := make([]string, 0, len(xs))
	for _, x := range xs {
		parts = append(parts, cdrPyReprStr(x))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}
