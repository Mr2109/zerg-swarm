// check_gap_axis_reconcile.go —— 门75「缺口账两轴对账」只读常跑判据件
// `scripts/gates/check-gap-axis-reconcile.py`（759 行 · v1.1.0）的 **Go 真搬**（甲案 C 类第 6 枚）。
//
// 为什么有这件（不是「顺手 Go 化」）：
//
//	v2.5.14 定案把判据实现面分两段走 —— M1 先把 `scripts/` 下的独立件收成 CLI 打得着的统一面
//	（转发壳）；M2 按价值**真搬 Go**（逐批判据搬 Go · 落点 `core/cmd/zerg/`）。
//	本件 = M2「缺口账两轴对账」的一枚：验收判据 = 对同一件跑两态（py 原件 ↔ 本件入口），
//	stdout / stderr / 退码三面逐字相同（对拍输入集见回执 · 未覆盖面见下）。
//
// ⚠ 本单边界（照实）：本件**已接线（inproc 表分派）** ——
//
//	· **不**在 `core/cmd/zerg/main.go` 里注册任何命令（✗ 不进命令树 · `add_step` **已挂** `precommit-gates.sh:3630`（门75 · tri 档））；
//	· **不**删 `scripts/gates/check-gap-axis-reconcile.py` 原件（退场规则：对拍通过后才退场）；
//	· **不**碰任何登记面（judge-registry / gate-coverage / 公开标记.tsv 等）；
//	· **已接线**：入口 `checkGapAxisReconcileGateMain` 由 `gateCheckGoEntry["check-gap-axis-reconcile"]` 分派（`zerg gate check check-gap-axis-reconcile`）· 原文此处断言与现读相反（只改文案 · 未改行为）。
//
// 入口签名（与 py 的 `main()` 对齐）：
//
//	func checkGapAxisReconcileGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv   —— 与 py `sys.argv[1:]` 同形（不含程序名）
//	  · 退码   —— 0（甲/乙 等式全成立且戊/己 无新命中）/ 1（有不等·闭集外形·失效件·轴间不一致）/
//	             2（取数面拿不到 ⇒ BLOCKED，不绿）
//	  · 输出   —— stdout/stderr 与 py 逐字对齐（见下「已对拍面 / 未覆盖面」）
//
// ★★ 标准声明 (a)：**默认根/默认账定位差异 —— 有差异（本单对拍显式规避）**。
//
//	py 的 `resolve_repo` 无 `--repo`/`ZERG_REPO` 时，取 `dirname^3(abspath(__file__))`（= 脚本上溯三层）
//	再试 `os.getcwd()`；本件**无 `__file__` 语境**（未来接线后是 CLI 子命令）⇒ 该缺省档换成**当前工作
//	目录**（与同批已落件 `check_gap_ledger_face.go` 等同一条口径），保留「现跑 `git rev-parse
//	--show-toplevel`」这一步。落在**同一真仓、cwd = 仓根**且给 `--repo` 时两者逐字相同（对拍已用显式
//	`--repo`）；「脚本位置 ≠ cwd」且都不给 `--repo` 时**路径推导面不同** ⇒ 这一面**不属逐字对齐承诺**。
//	账本定位（`--ledger` > `--state-dir` > env `ZERG_STATE_DIR` > `~/.zerg/state`）**两态逐字同构**，
//	`~` 由 `os.UserHomeDir()` 展开（py 用 `os.path.expanduser`）。
//
// ★★ 标准声明 (b)：**自指六形态 —— 逐条真判**：
//
//	① 本件在**自己**的扫描面内吗？—— **否**。本门扫描面 = 三条**显式路径**上的三类真源
//	   （账本 jsonl · 全仓 `git ls-files` 现读集合 · 真源件 `core/cmd/zerg/family_gap.go`）。
//	   件面遍历面 = `git ls-files` 的**索引**输出（只列已入索引件）；本件落 `core/cmd/zerg/` 且未入索引
//	   ⇒ 不在集合内。**证明**：落仓前后 `git ls-files` 输出与门自身对真仓的读数逐字相同（见回执）。
//	② 本件落在**任何他门**的扫描面内吗？—— **是**（`core/` 域）。`check-hardcoded-private-paths.py` 的
//	   扫描面含 `core/`，本件正文**零私有卷/家目录绝对路径字面**（家目录一律 `os.UserHomeDir()` 现取）
//	   ⇒ 该门对本件 0 命中。**证明**：落仓前后该门对 `core/` 的命中数**逐字不变**（见回执「自指对照」）。
//	③ 本件**产出**回喂自身输入面（回环）吗？—— **否**。默认只读 + 打 stdout；仅 `--out <外部目录>` 时
//	   往仓外目录落两份，**不追加任何账 / 不写索引** ⇒ 无回环面。
//	④ 本件**测试件/夹具**里的标记字面被他门命中吗？—— 夹具只用 `t.TempDir()` 下的**仓外**路径与合成账，
//	   零私有卷字面；`*_test.go` 亦被 `check-hardcoded-private-paths.py` 整体豁免。
//	⑤ 本件里的 `git ls-files` 现跑**会扫到自己**吗？—— 否（只在**索引**内找件；本件未入索引）。
//	⑥ 本件把 `scripts/gates/check-gap-axis-reconcile.py` 的**完整原串**写进注释吗？—— 否（只以相对路径引用）。
//
// 其余未覆盖面（✗ 不声称逐字对齐 · 接线前须逐面补齐或显式剔除）：
//
//	① **argparse 前缀缩写 + 帮助/用法正文**：py 用 `argparse` ⇒ `--js` / `--state-d` 这类**唯一前缀**
//	   被接受、`-h/--help` 打 argparse 帮助正文（含 prog 与描述段）、未知旗标打 `usage:` +
//	   `unrecognized arguments:` 正文 —— 本件只认**精确名**，`-h`/`--help` 落自定 usage 行，未知旗标落自定
//	   错误行。**退码同（0 / 2）· 正文不同** ⇒ 不进逐字面（对拍输入集已避开）。
//	② **JSON 解析失败正文**：py `json.loads` 抛的异常文本（`Expecting value: line 1 column 1 (char 0)` 一族）
//	   与 Go `encoding/json` 的错误文本**不同** ⇒ `rc=2` 的 BLOCKED 正文在「有行解不开」这一格**不同**
//	   （退码同 2 · 仅 stderr/stdout 正文差异，对拍单列）。
//	③ **`\s` / `\b` 宽度**：`load_modrule` 用 py 正则（`\s` 含 Unicode 空白）；Go RE2 的 `\s` 仅 ASCII 空白。
//	   真源件现值用普通 ASCII 空白，对拍输入集不含该形态。
//	④ **行切分**：py 文本模式按 universal newlines 切行（`\r\n` / `\r` 亦为行界）；本件按 `\n` 切。
//	⑤ **stdout 文案里的 `%r`**：本件按 CPython `repr()` 口径自实现（引号选择 / 转义 / 浮点最短表示）。
//	   已被真账与合成账面覆盖；含非 BMP 代理对等极端形态不在承诺面内。
package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// ── 常量（逐字取自 py 件头 / 顶部）────────────────────────────────────────────

const gapAxisRecTool = "check-gap-axis-reconcile.py"
const gapAxisRecVersion = "v1.1.0" // 5b 补片：+判据戊（现读存在性）· +判据己（轴间一致性）
const gapAxisRecGateName = "门75 缺口账两轴对账：件面/模块面 桶和 == 账内 仍缺 总数（不等即红 · 阻断）"

// gapAxisRecLedgerName —— 账本名引用登记处 ledger_family.go（✗ 别在本件再写字面名 · GAP-20260927-323）。
var gapAxisRecLedgerName = ledgerFamilyNames["gaps"]

// 退码（三值 · 与 py `tri` 三档逐格同义）—— 具名常量，不用裸数字
const (
	gapAxisRecRCOK    = 0 // 甲/乙 全成立 且 丙 无闭集外形态 且 戊/己 无新命中（存量豁免不计）
	gapAxisRecRCFail  = 1 // 有不等 / 有丙档闭集外形 / 有失效件（非存量豁免）/ 有轴间不一致
	gapAxisRecRCBlock = 2 // 取数面拿不到（BLOCKED，不绿）
)

const gapAxisRecRemain = "仍缺" // 分子/分母都只认这个态
const gapAxisRecWU = "无件"     // 无件形态前缀

// 现读真源：同族门 check-gap-ledger-view.py 的 IMPACT_CLOSED（面闭集 · 六值）
var gapAxisRecImpactClosed = []string{"命令面", "门禁面", "文档面", "公开面", "换件面", "归档面"}

// 己 的真源件（**现读**口径来源 · 本件不另抄推导）
const gapAxisRecFamilyGapRel = "core/cmd/zerg/family_gap.go"

// 戊 的存量豁免表（**本件自己的棘轮面** · 逐条带 GAP-id 作指针）。
// 口径：**首次现读**若存在失效件（`unit` 指向仓内已不存在路径），逐条写进本表 ⇒ 只打印、**不进退码**；
// 表外的（=**新出现的**）失效件才 rc=1。**只增不删**。条目形制：(GAP-id, unit) —— 两键全等才算豁免。
// ★ 5b 首读现读（真账副本 · `git ls-files` 现跑）：失效件 **0 条** ⇒ 本表**空**（首读无存量）。
type gapAxisRecExemptPair struct {
	gid  string
	unit string
}

var gapAxisRecExemptExistence = []gapAxisRecExemptPair{
	{gid: "GAP-20261001-11", unit: "scripts/gates/check-archive-mount.py"},
	{gid: "GAP-20261005-18", unit: "scripts/gates/check-pending-decisions.py"},
	{gid: "GAP-20260928-137", unit: "scripts/gates/check-newfile-registry.py"},
	{gid: "GAP-20260928-51", unit: "scripts/gates/check-trustroot-witness.py"},
	{gid: "GAP-20260927-70", unit: "scripts/gates/check-approve-backfill.py"},
	// 段2 捞回4件批（2026-10-07）：check-public-face-pairing.py 已登记退场（`zerg script retire`）⇒ 该仍缺行 unit 失指，入存量豁免.
	{gid: "GAP-20260926-39", unit: "scripts/gates/check-public-face-pairing.py"},
	// G19/G20 退役批（2026-10-07 · 35d270c0）：check-build-tags.py 已退役（Go 半 core/cmd/zerg/check_build_tags.go 在盘）⇒ 该仍缺行 unit 失指，入存量豁免.
	{gid: "GAP-20261007-18", unit: "scripts/gates/check-build-tags.py"},
	// 段2 续退批（2026-10-07 · 本单）：check-service-declaration.py 已登记退场（`zerg script retire`）⇒ 该仍缺行（GAP-20260927-101 · 门⑧/门㉒ 自检夹具族）unit 失指，入存量豁免.
	{gid: "GAP-20260927-101", unit: "scripts/gates/check-service-declaration.py"},
	// 段2 续退批（2026-10-07 · 本单）：check-anchor-uniqueness.py 已登记退场（Go 半 core/cmd/zerg/check_anchor_uniqueness.go 在盘）⇒ 该件无仍缺 unit 行，登记留痕.
	// 乙组退役批（2026-10-07 · 本单）：check-stale-frozen.py 已登记退场（`zerg script retire` · Go 半 core/cmd/zerg/check_stale_frozen.go 在盘）⇒ 该仍缺行 unit 失指，入存量豁免。
	{gid: "GAP-20261005-64", unit: "scripts/gates/check-stale-frozen.py"},
	// 乙组退役批（2026-10-07 · 本单）：check-registry-key-unique.py 已登记退场（`zerg script retire` · Go 半 core/cmd/zerg/check_registry_key_unique.go 在盘）⇒ 该仍缺行 unit 失指，入存量豁免。
	{gid: "GAP-20261007-09", unit: "scripts/gates/check-registry-key-unique.py"},
	// 公开面退 10 件批（2026-10-07 · 本单）：check-write-face-gate.py 已登记退场（`zerg script retire` · Go 半 core/cmd/zerg/check_write_face_gate.go 在盘）⇒ 该仍缺行（GAP-20261007-08）unit 失指，入存量豁免.
	{gid: "GAP-20261007-08", unit: "scripts/gates/check-write-face-gate.py"},
	// 甲组 G01 第5件退场随动（2026-10-08 · 本单）：check-ghost-decl-diff.py 退场后（Go 半 core/cmd/zerg/check_ghost_decl_diff.go 在盘 · 其读取面 :844 源码级牙已 by-guard 容忍 · 自检 ④b/④c 已 by-else 容忍）⇒ 该仍缺行（GAP-20260927-62 · 门㉒ 侧① 现值面）unit 失指，入存量豁免（本表只增不删）。py 在盘时本对不生效（`ls-files` 命中即 continue）。
	{gid: "GAP-20260927-62", unit: "scripts/gates/check-ghost-decl-diff.py"},
	// 第二批收口·退役随动（2026-10-09 · 本单）：compare-wall-argv.py 已登记退场（`retire_ledger` 在账 · Go 半 core/cmd/zerg/eval_wall_argv.go 在盘）⇒ 该仍缺行（GAP-20261009-03 · 门禁面 茧壁迁移桥临时件回收）unit 失指，入存量豁免（本表只增不删）。py 在盘时本对不生效（`ls-files` 命中即 continue）。
	{gid: "GAP-20261009-03", unit: "scripts/evals/compare-wall-argv.py"},
	// 第二批收口·退役随动（2026-10-09 · 本单）：chat-harness.py 已登记退场（`retire_ledger` 在账 · Go 半 core/cmd/zerg/eval_chat_harness.go 在盘且入口已接线）⇒ 该仍缺行（GAP-20261009-04 · 命令面 eval run chat-harness）unit 失指，入存量豁免（本表只增不删）。py 在盘时本对不生效（`ls-files` 命中即 continue）。
	{gid: "GAP-20261009-04", unit: "scripts/evals/chat-harness.py"},
	// py 退场随动（2026-10-12 · 本单）：check-doc-name.py 已退役（Go 半 core/cmd/zerg/check_doc_name.go 在盘）⇒ 该仍缺行（GAP-20260927-63 · 经用户拍板由「已解」改回「仍缺」）unit 失指，入存量豁免（只打印 · 不进退码）。
	{gid: "GAP-20260927-63", unit: "scripts/gates/check-doc-name.py"},
}

// gapAxisRecWUClosed —— 无件桶名闭集 = 裸 `无件` + `无件(<面>)`（面 ∈ 面闭集）
func gapAxisRecWUClosed() []string {
	out := []string{gapAxisRecWU}
	for _, f := range gapAxisRecImpactClosed {
		out = append(out, gapAxisRecWU+"("+f+")")
	}
	return out
}

// gapAxisRecModClosed —— 模块面的无件桶名闭集（真账里无件行的 module 就是裸 `无件`）
func gapAxisRecModClosed() []string { return []string{gapAxisRecWU} }

// ── 取数面 ───────────────────────────────────────────────────────────────────

func gapAxisRecExpandUser(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			if p == "~" {
				return home
			}
			return filepath.Join(home, p[2:])
		}
	}
	return p
}

func gapAxisRecAbsExpand(p string) string {
	abs, err := filepath.Abs(gapAxisRecExpandUser(p))
	if err != nil {
		return gapAxisRecExpandUser(p)
	}
	return abs
}

func gapAxisRecDefaultStateDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(".zerg", "state")
	}
	return filepath.Join(home, ".zerg", "state")
}

// gapAxisRecResolveLedger —— `--ledger` > `--state-dir` > env `ZERG_STATE_DIR` > `~/.zerg/state`，
// 账本名写死 `zerg-cli-gaps.jsonl`。
func gapAxisRecResolveLedger(ledger, stateDir string) string {
	if ledger != "" {
		return gapAxisRecAbsExpand(ledger)
	}
	base := stateDir
	if base == "" {
		base = os.Getenv("ZERG_STATE_DIR")
	}
	if base == "" {
		base = gapAxisRecDefaultStateDir()
	}
	return filepath.Join(gapAxisRecAbsExpand(base), gapAxisRecLedgerName)
}

// gapAxisRecResolveRepo —— 仓根：`--repo` > env `ZERG_REPO` > cwd 现跑 `git rev-parse --show-toplevel`。
// （py 缺省第一支是脚本位置上溯三层 —— 见件头标准声明 (a)）
func gapAxisRecResolveRepo(repo string) string {
	if repo != "" {
		return gapAxisRecAbsExpand(repo)
	}
	if env := os.Getenv("ZERG_REPO"); env != "" {
		return gapAxisRecAbsExpand(env)
	}
	here, err := os.Getwd()
	if err != nil {
		return ""
	}
	for _, cand := range []string{here} {
		cmd := exec.Command("git", "-C", cand, "rev-parse", "--show-toplevel")
		out, err := cmd.Output()
		if err == nil {
			s := strings.TrimSpace(string(out))
			if s != "" {
				return s
			}
		}
	}
	return ""
}

// gapAxisRecGitTracked —— 戊 的唯一取数口 —— **现读** `git ls-files`（真源 `gapTrackedFiles` 同形：`ls-files -z`）。
// ⇒ (set|nil, err)。拿不到（不在 git 仓 / git 起不来）⇒ nil —— 调用方按 rc=2 **不给结论**。
func gapAxisRecGitTracked(root string) (map[string]bool, string) {
	if root == "" {
		return nil, "仓根没解析出来（--repo / ZERG_REPO / git rev-parse 三面都没给）"
	}
	cmd := exec.Command("git", "-C", root, "-c", "core.quotepath=false", "ls-files", "-z")
	out, err := cmd.Output()
	if err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			code := err.(*exec.ExitError).ExitCode()
			return nil, fmt.Sprintf("git ls-files 失败（rc=%d）：%s", code, root)
		}
		return nil, "git 起不来：" + err.Error()
	}
	set := map[string]bool{}
	for _, x := range strings.Split(string(out), "\x00") {
		if x != "" {
			set[x] = true
		}
	}
	return set, ""
}

// ── 己 的真源口径（现读 family_gap.go）──────────────────────────────────────

type gapAxisRecRule struct {
	src        string
	sha16      string
	noFile     string
	noneMod    string
	rootMod    string
	plannedPfx string
	rootDirLit string
}

// gapAxisRecLoadModRule —— 己 的唯一口径口 —— **现读**真源件 `core/cmd/zerg/family_gap.go` 的
// `gapModuleOfUnit`。锚点（三常量 + `filepath.Dir` + 手写占位前缀）缺一 ⇒ nil ⇒ rc=2（不另抄推导）。
func gapAxisRecLoadModRule(root string) (*gapAxisRecRule, string) {
	if root == "" {
		return nil, "仓根没解析出来（己 的真源件读不到）"
	}
	path := filepath.Join(root, filepath.FromSlash(gapAxisRecFamilyGapRel))
	st, err := os.Stat(path)
	if err != nil || st.IsDir() {
		return nil, "真源件不在盘：" + path
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, "真源件读不了：" + err.Error()
	}
	txt := string(raw)

	reConst := func(name string) (string, bool) {
		re := regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\s*=\s*"([^"]*)"`)
		m := re.FindStringSubmatch(txt)
		if m == nil {
			return "", false
		}
		return m[1], true
	}
	noFile, okNF := reConst("gapUnitNoFile")
	noneMod, okNM := reConst("gapUnitNoneMod")
	rootMod, okRM := reConst("gapUnitRootMod")

	reBody := regexp.MustCompile(`(?s)func gapModuleOfUnit\(unit string\) string \{(.*?)\n\}`)
	bodyM := reBody.FindStringSubmatch(txt)

	var miss []string
	if !okNF {
		miss = append(miss, "gapUnitNoFile")
	}
	if !okNM {
		miss = append(miss, "gapUnitNoneMod")
	}
	if !okRM {
		miss = append(miss, "gapUnitRootMod")
	}
	if bodyM == nil {
		miss = append(miss, "gapModuleOfUnit 函数体")
	}
	if len(miss) > 0 {
		return nil, "真源口径锚点没命中：" + strings.Join(miss, " · ")
	}
	b := bodyM[1]
	if !strings.Contains(b, "filepath.Dir") {
		return nil, "真源口径锚点没命中：filepath.Dir（目录前缀语义）"
	}
	rePm := regexp.MustCompile(`HasPrefix\(unit,\s*"([^"]*)"\)`)
	pm := rePm.FindStringSubmatch(b)
	if pm == nil {
		return nil, "真源口径锚点没命中：手写占位前缀（HasPrefix(unit, \"…\")）"
	}
	reDr := regexp.MustCompile(`dir\s*==\s*"([^"]*)"`)
	dr := reDr.FindStringSubmatch(b)
	rootLit := "."
	if dr != nil {
		rootLit = dr[1]
	}
	return &gapAxisRecRule{
		src: path, sha16: gapAxisRecSHA16(path), noFile: noFile, noneMod: noneMod,
		rootMod: rootMod, plannedPfx: pm[1], rootDirLit: rootLit,
	}, ""
}

// ── 账本读取 ─────────────────────────────────────────────────────────────────

type gapAxisRecRow struct {
	lineno int
	obj    map[string]interface{}
}

type gapAxisRecBadLine struct {
	lineno string
	why    string
}

func gapAxisRecIsFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// gapAxisRecReadLedger ⇒ (rows, lines, unparsed)
//
//	rows    : [ (lineno, obj) ] 逐行解析成功且是 JSON 对象
//	lines   : 非空行总条数
//	unparsed: [ (lineno, 原因) ] JSON 解不开 / 顶层不是对象
func gapAxisRecReadLedger(path string) ([]gapAxisRecRow, int, []gapAxisRecBadLine) {
	if !gapAxisRecIsFile(path) {
		return nil, 0, []gapAxisRecBadLine{{"—", "账本不在盘：" + path}}
	}
	fh, err := os.Open(path)
	if err != nil {
		return nil, 0, []gapAxisRecBadLine{{"—", "账本读不了：" + err.Error()}}
	}
	defer fh.Close()
	rows := []gapAxisRecRow{}
	var unparsed []gapAxisRecBadLine
	lines := 0
	sc := bufio.NewScanner(fh)
	sc.Buffer(make([]byte, 0, 1<<20), 1<<26)
	i := 0
	for sc.Scan() {
		i++
		s := strings.TrimSpace(sc.Text())
		if s == "" {
			continue
		}
		lines++
		var obj interface{}
		if err := json.Unmarshal([]byte(s), &obj); err != nil {
			unparsed = append(unparsed, gapAxisRecBadLine{strconv.Itoa(i), "JSON 解不开：" + err.Error()})
			continue
		}
		m, ok := obj.(map[string]interface{})
		if !ok {
			unparsed = append(unparsed, gapAxisRecBadLine{strconv.Itoa(i), "顶层不是 JSON 对象：" + gapAxisRecTypeName(obj)})
			continue
		}
		rows = append(rows, gapAxisRecRow{i, m})
	}
	return rows, lines, unparsed
}

func gapAxisRecTypeName(v interface{}) string {
	switch v.(type) {
	case nil:
		return "NoneType"
	case bool:
		return "bool"
	case float64:
		return "float"
	case string:
		return "str"
	case []interface{}:
		return "list"
	case map[string]interface{}:
		return "dict"
	}
	return fmt.Sprintf("%T", v)
}

func gapAxisRecSHA16(path string) string {
	fh, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer fh.Close()
	h := sha256.New()
	_, _ = io.Copy(h, fh)
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// gapAxisRecNowISO —— py: datetime.now().astimezone().isoformat(timespec="seconds")
func gapAxisRecNowISO() string {
	return time.Now().Format("2006-01-02T15:04:05-07:00")
}

// ── 分桶（两轴共用一把尺）────────────────────────────────────────────────────

// gapAxisRecClassify ⇒ (kind, bucket)
//
//	kind = "you" 有件 · "wu" 无件 · "anom" 键面缺失（不入桶）
func gapAxisRecClassify(obj map[string]interface{}, key string) (string, string) {
	raw, ok := obj[key]
	if !ok { // 键面缺失 ⇒ 不入任一桶（甲/乙 的差就在这类）
		return "anom", ""
	}
	v, ok := raw.(string)
	if !ok { // 值不是字符串 ⇒ 同样不入桶
		return "anom", ""
	}
	if v == "" || strings.HasPrefix(v, gapAxisRecWU) {
		if v == "" {
			return "wu", gapAxisRecWU // 空串归裸 `无件` 桶（"空"亦属无件形态）
		}
		return "wu", v
	}
	return "you", v
}

type gapAxisRecAnom struct {
	lineno int
	gid    string
}

type gapAxisRecAxis struct {
	key        string
	bucketsYou int
	bucketsWU  int
	sumYou     int
	sumWU      int
	sumAll     int
	total      int
	diff       int
	anom       []gapAxisRecAnom
	wuNames    []string
	outside    []string
}

func gapAxisRecAxisReport(rows []gapAxisRecRow, key string, closed []string, remTotal int) gapAxisRecAxis {
	you := map[string]int{}
	wu := map[string]int{}
	var anom []gapAxisRecAnom
	for _, r := range rows {
		kind, bucket := gapAxisRecClassify(r.obj, key)
		switch kind {
		case "you":
			you[bucket]++
		case "wu":
			wu[bucket]++
		default:
			anom = append(anom, gapAxisRecAnom{r.lineno, gapAxisRecGID(r.obj)})
		}
	}
	sYou, sWU := 0, 0
	for _, c := range you {
		sYou += c
	}
	for _, c := range wu {
		sWU += c
	}
	var outside []string
	for k := range wu {
		if !gapAxisRecIn(closed, k) {
			outside = append(outside, k)
		}
	}
	sort.Strings(outside)
	names := make([]string, 0, len(wu))
	for k := range wu {
		names = append(names, k)
	}
	sort.Strings(names)
	return gapAxisRecAxis{
		key: key, bucketsYou: len(you), bucketsWU: len(wu),
		sumYou: sYou, sumWU: sWU, sumAll: sYou + sWU, total: remTotal,
		diff: remTotal - (sYou + sWU), anom: anom, wuNames: names, outside: outside,
	}
}

// ── 戊 / 己（5b 补片 · 两条新判据）──────────────────────────────────────────

// gapAxisRecModuleOfUnit —— 按**现读**真源口径推 `module`（= `gapModuleOfUnit` 的逐字同义实现）。
// 口径三支：① 空串 / `gapUnitNoFile` / 手写占位前缀 ⇒ `gapUnitNoneMod`；
// ② `filepath.Dir` = 仓根（`.` / 空 / 分隔符）⇒ `gapUnitRootMod`；③ 其余 ⇒ 目录前缀。
func gapAxisRecModuleOfUnit(unit string, rule *gapAxisRecRule) string {
	if unit == "" || unit == rule.noFile || strings.HasPrefix(unit, rule.plannedPfx) {
		return rule.noneMod
	}
	d := filepath.Dir(unit)
	if d == "" || d == "." || d == string(os.PathSeparator) || d == "/" {
		return rule.rootMod
	}
	return d
}

type gapAxisRecExistHit struct {
	lineno     int
	gid        string
	unit       string
	unitSource interface{}
}

type gapAxisRecExempt struct {
	lineno int
	gid    string
	unit   string
}

// gapAxisRecExistenceHits ⇒ (hits, exempted)
//
//	hits    : [(行号, GAP-id, unit, unit_source)] —— **新**失效件（⇒ rc=1）
//	exempted: [(行号, GAP-id, unit)] —— 存量豁免命中（逐条打印 · **不进退码**）
//	域 = 件面判成「有件」的行；`unit` 键面缺失 / 空串 / `无件*` 一律不进戊。
func gapAxisRecExistenceHits(rem []gapAxisRecRow, tracked map[string]bool, exempt []gapAxisRecExemptPair) ([]gapAxisRecExistHit, []gapAxisRecExempt) {
	var hits []gapAxisRecExistHit
	var exempted []gapAxisRecExempt
	for _, r := range rem {
		kind, _ := gapAxisRecClassify(r.obj, "unit")
		if kind != "you" {
			continue
		}
		u := r.obj["unit"].(string)
		if tracked[u] { // 现读 ls-files 逐字命中 ⇒ 件在盘上
			continue
		}
		gid := gapAxisRecGID(r.obj)
		isExempt := false
		for _, e := range exempt {
			if e.gid == gid && e.unit == u {
				isExempt = true
				break
			}
		}
		if isExempt {
			exempted = append(exempted, gapAxisRecExempt{r.lineno, gid, u})
			continue
		}
		hits = append(hits, gapAxisRecExistHit{r.lineno, gid, u, r.obj["unit_source"]})
	}
	return hits, exempted
}

type gapAxisRecConsHit struct {
	lineno int
	gid    string
	unit   string
	got    interface{}
	want   string
	note   string
}

// gapAxisRecConsistencyHits ⇒ [(行号, GAP-id, unit, 账里 module, 口径推导值, 说明)]（差数进 rc=1）
//
// 域 = **每条有 `unit`** 的行（键面存在且值为字符串 —— 含无件形态）。
// ★ 现读真源的**字面**边界：`gapModuleOfUnit` 只认 `gapUnitNoFile`（= `无件(命令面)`）**这一个字面**
// + 手写占位前缀 ⇒ `无件`；闭集里的**其他**无件形态在真源里会落到 `filepath.Dir` 那一支 ⇒ **`仓根`**。
// 这类命中**单独加说明**（不静默）。
func gapAxisRecConsistencyHits(rem []gapAxisRecRow, rule *gapAxisRecRule) []gapAxisRecConsHit {
	var out []gapAxisRecConsHit
	for _, r := range rem {
		kind, _ := gapAxisRecClassify(r.obj, "unit")
		if kind == "anom" { // 无 `unit` 键 ⇒ 轴间无从对起（那是甲的面）
			continue
		}
		u := r.obj["unit"].(string)
		want := gapAxisRecModuleOfUnit(u, rule)
		got := r.obj["module"]
		if got != interface{}(want) {
			note := ""
			if strings.HasPrefix(u, gapAxisRecWU) && u != rule.noFile {
				note = fmt.Sprintf("真源口径边界：`gapModuleOfUnit` 只认 `%s` 字面 ⇒ `%s` 这支落到 "+
					"`filepath.Dir` ⇒ `%s`（闭集内其他无件形态不在真源这一支里）",
					rule.noFile, "dir 分支", rule.rootMod)
			}
			out = append(out, gapAxisRecConsHit{r.lineno, gapAxisRecGID(r.obj), u, got, want, note})
		}
	}
	return out
}

func gapAxisRecGID(obj map[string]interface{}) string {
	if v, ok := obj["id"]; ok {
		return gapAxisRecPyStrAny(v)
	}
	return "(无 id)"
}

func gapAxisRecIn(xs []string, v string) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

// ── CPython 口径的小工具（`%r` / `str()` / 浮点最短表示 / JSON 序列化）────────

// gapAxisRecPyStrAny —— py `str(v)`（None ⇒ "None" · True ⇒ "True" …）
func gapAxisRecPyStrAny(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return "None"
	case string:
		return t
	case bool:
		if t {
			return "True"
		}
		return "False"
	case float64:
		return gapAxisRecPyFloat(t)
	case int:
		return strconv.Itoa(t)
	}
	return fmt.Sprintf("%v", v)
}

// gapAxisRecPyRepr —— py `repr(str)`：引号选择 + 转义（≙ 本件口径面所用的全部形态）
func gapAxisRecPyRepr(s string) string {
	quote := byte('\'')
	if strings.ContainsRune(s, '\'') && !strings.ContainsRune(s, '"') {
		quote = '"'
	}
	var b strings.Builder
	b.WriteByte(quote)
	for _, r := range s {
		switch {
		case byte(r) == quote && r < 128:
			b.WriteByte('\\')
			b.WriteRune(r)
		case r == '\\':
			b.WriteString(`\\`)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r == '	':
			b.WriteString(`	`)
		case r < 0x20 || r == 0x7f:
			b.WriteString(fmt.Sprintf(`\x%02x`, r))
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte(quote)
	return b.String()
}

// gapAxisRecPyReprAny —— py `%r` 的取值面（nil ⇒ None · str ⇒ repr · 其余 ⇒ str）
func gapAxisRecPyReprAny(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return "None"
	case string:
		return gapAxisRecPyRepr(t)
	case bool:
		if t {
			return "True"
		}
		return "False"
	case float64:
		return gapAxisRecPyFloat(t)
	}
	return fmt.Sprintf("%v", v)
}

// gapAxisRecPyFloat —— CPython repr(float) 口径（阈值 decpt<=-4 或 >16 走指数形）
func gapAxisRecPyFloat(f float64) string {
	if math.IsNaN(f) {
		return "NaN"
	}
	if math.IsInf(f, 1) {
		return "Infinity"
	}
	if math.IsInf(f, -1) {
		return "-Infinity"
	}
	if f == 0 {
		if math.Signbit(f) {
			return "-0.0"
		}
		return "0.0"
	}
	s := strconv.FormatFloat(f, 'e', -1, 64)
	i := strings.IndexByte(s, 'e')
	mant, expStr := s[:i], s[i+1:]
	exp, _ := strconv.Atoi(expStr)
	neg := strings.HasPrefix(mant, "-")
	mant = strings.TrimPrefix(mant, "-")
	digits := strings.Replace(mant, ".", "", 1)
	decpt := exp + 1
	var out string
	switch {
	case decpt <= -4 || decpt > 16:
		m := digits
		if len(digits) > 1 {
			m = digits[:1] + "." + digits[1:]
		}
		e := decpt - 1
		sign := "+"
		if e < 0 {
			sign = "-"
			e = -e
		}
		out = m + "e" + sign + fmt.Sprintf("%02d", e)
	case decpt <= 0:
		out = "0." + strings.Repeat("0", -decpt) + digits
	case decpt >= len(digits):
		out = digits + strings.Repeat("0", decpt-len(digits)) + ".0"
	default:
		out = digits[:decpt] + "." + digits[decpt:]
	}
	if neg {
		out = "-" + out
	}
	return out
}

// gapAxisRecPyRound4 —— py `round(x, 4)`（十进制正确舍入 · 半偶）
func gapAxisRecPyRound4(f float64) float64 {
	v, err := strconv.ParseFloat(strconv.FormatFloat(f, 'f', 4, 64), 64)
	if err != nil {
		return f
	}
	return v
}

// ── Python 兼容 JSON 序列化（indent<0 ⇒ 紧凑 `", "` / `": "`；否则 indent 空格缩进）──

type gapAxisRecKV struct {
	k string
	v interface{}
}

type gapAxisRecObj []gapAxisRecKV

func gapAxisRecPyStrJSON(s string) string {
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
		case '	':
			b.WriteString(`	`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		default:
			if r < 0x20 {
				b.WriteString(fmt.Sprintf(`\u%04x`, r))
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

func gapAxisRecPyJSON(v interface{}, indent, level int) string {
	compact := indent < 0
	nl, pad, padIn, isep := "", "", "", ", "
	if !compact {
		nl = "\n"
		pad = strings.Repeat(" ", indent*level)
		padIn = strings.Repeat(" ", indent*(level+1))
		isep = "," + nl + padIn
	}
	join := func(parts []string, open, close string) string {
		if len(parts) == 0 {
			return open + close
		}
		return open + nl + padIn + strings.Join(parts, isep) + nl + pad + close
	}
	switch t := v.(type) {
	case nil:
		return "null"
	case bool:
		if t {
			return "true"
		}
		return "false"
	case string:
		return gapAxisRecPyStrJSON(t)
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case float64:
		return gapAxisRecPyFloat(t)
	case gapAxisRecObj:
		var parts []string
		for _, kv := range t {
			parts = append(parts, gapAxisRecPyStrJSON(kv.k)+": "+gapAxisRecPyJSON(kv.v, indent, level+1))
		}
		if compact {
			return "{" + strings.Join(parts, isep) + "}"
		}
		return join(parts, "{", "}")
	case []string:
		parts := make([]string, len(t))
		for i, e := range t {
			parts[i] = gapAxisRecPyStrJSON(e)
		}
		return join(parts, "[", "]")
	case []gapAxisRecAnom:
		parts := make([]string, len(t))
		for i, e := range t {
			parts[i] = gapAxisRecPyJSON([]interface{}{e.lineno, e.gid}, indent, level+1)
		}
		return join(parts, "[", "]")
	case []gapAxisRecExistHit:
		parts := make([]string, len(t))
		for i, e := range t {
			parts[i] = gapAxisRecPyJSON([]interface{}{e.lineno, e.gid, e.unit, e.unitSource}, indent, level+1)
		}
		return join(parts, "[", "]")
	case []gapAxisRecExempt:
		parts := make([]string, len(t))
		for i, e := range t {
			parts[i] = gapAxisRecPyJSON([]interface{}{e.lineno, e.gid, e.unit}, indent, level+1)
		}
		return join(parts, "[", "]")
	case []gapAxisRecConsHit:
		parts := make([]string, len(t))
		for i, e := range t {
			parts[i] = gapAxisRecPyJSON([]interface{}{e.lineno, e.gid, e.unit, e.got, e.want, e.note}, indent, level+1)
		}
		return join(parts, "[", "]")
	case []gapAxisRecTop:
		parts := make([]string, len(t))
		for i, e := range t {
			parts[i] = gapAxisRecPyJSON([]interface{}{e.mod, e.n}, indent, level+1)
		}
		return join(parts, "[", "]")
	case []interface{}:
		parts := make([]string, len(t))
		for i, e := range t {
			parts[i] = gapAxisRecPyJSON(e, indent, level+1)
		}
		return join(parts, "[", "]")
	}
	return "null"
}

// ── 判定 ─────────────────────────────────────────────────────────────────────

type gapAxisRecTop struct {
	mod string
	n   int
}

type gapAxisRecCoverage struct {
	wu    int
	you   int
	ratio float64
	top   []gapAxisRecTop
}

type gapAxisRecExistence struct {
	checked  int
	hits     []gapAxisRecExistHit
	exempted []gapAxisRecExempt
	trackedN int
	src      string
}

type gapAxisRecConsistency struct {
	checked  int
	hits     []gapAxisRecConsHit
	src      string
	srcSha16 string
}

type gapAxisRecReport struct {
	ledger      string
	sha16       string
	readAt      string
	lines       int
	rowsN       int
	remTotal    int
	unit        gapAxisRecAxis
	mod         gapAxisRecAxis
	coverage    gapAxisRecCoverage
	existence   gapAxisRecExistence
	consistency gapAxisRecConsistency
	exemptN     int
	rc          int
}

type gapAxisRecCtx struct {
	tracked    map[string]bool
	rule       *gapAxisRecRule
	exempt     []gapAxisRecExemptPair
	trackedErr string
	ruleErr    string
}

type gapAxisRecJudgeOut struct {
	blocked  string
	unparsed []gapAxisRecBadLine
	lines    int
	rep      *gapAxisRecReport
}

// gapAxisRecJudge ⇒ (rc, out)。ctx.tracked/rule 为 nil = 取数面拿不到。
func gapAxisRecJudge(rows []gapAxisRecRow, lines int, unparsed []gapAxisRecBadLine, ledger string, ctx *gapAxisRecCtx) (int, *gapAxisRecJudgeOut) {
	if rows == nil {
		msg := "取数面拿不到"
		if len(unparsed) > 0 {
			msg = unparsed[0].why
		}
		return gapAxisRecRCBlock, &gapAxisRecJudgeOut{blocked: msg}
	}
	if len(unparsed) > 0 || lines == 0 || len(rows) == 0 {
		why := "账本空转（0 行）"
		if len(unparsed) > 0 {
			why = fmt.Sprintf("有 %d 行解不开", len(unparsed))
		}
		u := unparsed
		if len(u) > 20 {
			u = u[:20]
		}
		return gapAxisRecRCBlock, &gapAxisRecJudgeOut{blocked: why, unparsed: u, lines: lines}
	}

	var rem []gapAxisRecRow
	for _, r := range rows {
		if s, ok := r.obj["state"].(string); ok && s == gapAxisRecRemain {
			rem = append(rem, r)
		}
	}
	remTotal := len(rem)
	// ★ 2026-10-23（Mr2109 拍板 · 缺口 `GAP-20260926-18` 实施中现读到的**提交焊死**）：
	//   原口径「`仍缺` 0 条 ⇒ BLOCKED（空转不给结论）」在**账面全清**时会把提交闸焊死 ——
	//   而账面 0 未闭正是**目标态**。改：只有**账本 0 行**（真·取数面拿不到）才 BLOCKED；
	//   账本非空 + `仍缺` 0 条 ⇒ 走正常计算，得 rc=0（人面会印「账内读数 … 仍缺 0」）。
	if remTotal == 0 && len(rows) == 0 {
		return gapAxisRecRCBlock, &gapAxisRecJudgeOut{blocked: "账本 0 行（真空转 · 取数面拿不到）", lines: lines}
	}

	unit := gapAxisRecAxisReport(rem, "unit", gapAxisRecWUClosed(), remTotal)
	mod := gapAxisRecAxisReport(rem, "module", gapAxisRecModClosed(), remTotal)

	// 判据丁：覆盖率（只报告 · 不判红）
	modCounts := map[string]int{}
	for _, r := range rem {
		k := "(无)"
		if s, ok := r.obj["module"].(string); ok {
			k = s
		}
		modCounts[k]++
	}
	topItems := make([]gapAxisRecTop, 0, len(modCounts))
	for k, v := range modCounts {
		topItems = append(topItems, gapAxisRecTop{k, v})
	}
	sort.Slice(topItems, func(i, j int) bool {
		if topItems[i].n != topItems[j].n {
			return topItems[i].n > topItems[j].n
		}
		return topItems[i].mod < topItems[j].mod
	})
	if len(topItems) > 3 {
		topItems = topItems[:3]
	}
	ratio := 0.0
	if remTotal != 0 {
		ratio = float64(unit.sumWU) / float64(remTotal)
	}
	cover := gapAxisRecCoverage{wu: unit.sumWU, you: unit.sumYou, ratio: ratio, top: topItems}

	// 判据戊 / 己 的取数面（**拿不到就不给结论** · 不拿旧口径硬判）
	if ctx.tracked == nil {
		e := ctx.trackedErr
		if e == "" {
			e = "现读 git ls-files 失败"
		}
		return gapAxisRecRCBlock, &gapAxisRecJudgeOut{blocked: "戊 取数面拿不到：" + e}
	}
	if ctx.rule == nil {
		e := ctx.ruleErr
		if e == "" {
			e = "真源口径没现读出来"
		}
		return gapAxisRecRCBlock, &gapAxisRecJudgeOut{blocked: "己 取数面拿不到：" + e}
	}

	existHits, existKeep := gapAxisRecExistenceHits(rem, ctx.tracked, ctx.exempt)
	consist := gapAxisRecConsistencyHits(rem, ctx.rule)
	checkedYou, checkedUnit := 0, 0
	for _, r := range rem {
		k, _ := gapAxisRecClassify(r.obj, "unit")
		if k == "you" {
			checkedYou++
		}
		if k != "anom" {
			checkedUnit++
		}
	}
	exist := gapAxisRecExistence{checked: checkedYou, hits: existHits, exempted: existKeep,
		trackedN: len(ctx.tracked), src: "git -C <仓> ls-files -z（现读 · 无缓存）"}
	consistRep := gapAxisRecConsistency{checked: checkedUnit, hits: consist, src: ctx.rule.src, srcSha16: ctx.rule.sha16}

	bad := unit.diff != 0 || mod.diff != 0 || len(unit.anom) > 0 || len(mod.anom) > 0 ||
		len(unit.outside) > 0 || len(mod.outside) > 0 || len(existHits) > 0 || len(consist) > 0
	rc := gapAxisRecRCOK
	if bad {
		rc = gapAxisRecRCFail
	}
	rep := &gapAxisRecReport{
		ledger: ledger, sha16: gapAxisRecSHA16(ledger), readAt: gapAxisRecNowISO(),
		lines: lines, rowsN: len(rows), remTotal: remTotal, unit: unit, mod: mod, coverage: cover,
		existence: exist, consistency: consistRep, exemptN: len(ctx.exempt), rc: rc,
	}
	return rc, &gapAxisRecJudgeOut{rep: rep}
}

// ── 人面渲染（逐字对齐 py `render`）──────────────────────────────────────────

func gapAxisRecJoinRepr(xs []string) string {
	parts := make([]string, len(xs))
	for i, k := range xs {
		parts[i] = gapAxisRecPyRepr(k)
	}
	return strings.Join(parts, " / ")
}

func gapAxisRecRender(rep *gapAxisRecReport) string {
	var out []string
	out = append(out, "── "+gapAxisRecGateName+" ──")
	out = append(out, "取数面 : "+rep.ledger)
	out = append(out, fmt.Sprintf("现读时刻: %s · 账本 sha256[0:16] = %s", rep.readAt, rep.sha16))
	out = append(out, fmt.Sprintf("账内读数: 行 %d · 对象 %d · 仍缺 %d（分子/分母唯一来源）",
		rep.lines, rep.rowsN, rep.remTotal))
	axes := []struct {
		label  string
		a      gapAxisRecAxis
		closed []string
	}{
		{"甲 件面", rep.unit, gapAxisRecWUClosed()},
		{"乙 模块面", rep.mod, gapAxisRecModClosed()},
	}
	for _, ax := range axes {
		a := ax.a
		mark := "✗"
		if a.diff == 0 && len(a.anom) == 0 {
			mark = "✓"
		}
		out = append(out, fmt.Sprintf("%s : 有件桶 %d 个 Σ=%d ｜ 无件桶 %d 个 Σ=%d ｜ 桶和 %d ｜ 差 = %d %s",
			ax.label, a.bucketsYou, a.sumYou, a.bucketsWU, a.sumWU, a.sumAll, a.diff, mark))
		if len(a.anom) > 0 {
			out = append(out, fmt.Sprintf("   ✗ 键面缺失（不入桶 ⇒ 等式差就在这几行）· %d 条（GAP-id 作主指针）：", len(a.anom)))
			n := len(a.anom)
			if n > 20 {
				n = 20
			}
			for _, an := range a.anom[:n] {
				out = append(out, fmt.Sprintf("      · %s（行 %d）· 缺 `%s` 键", an.gid, an.lineno, a.key))
			}
			if len(a.anom) > 20 {
				out = append(out, fmt.Sprintf("      · …（只印前 20 条 · 共 %d 条）", len(a.anom)))
			}
		}
		if len(a.outside) > 0 {
			out = append(out, fmt.Sprintf("   ✗ 闭集外形 · %d 个桶名不在闭集内：", len(a.outside)))
			n := len(a.outside)
			if n > 20 {
				n = 20
			}
			for _, k := range a.outside[:n] {
				out = append(out, "      · "+gapAxisRecPyRepr(k))
			}
		}
		names := "(无)"
		if len(a.wuNames) > 0 {
			names = gapAxisRecJoinRepr(a.wuNames)
		}
		out = append(out, "   现读桶名（无件面）: "+names)
		out = append(out, "   闭集            : "+gapAxisRecJoinRepr(ax.closed))
	}
	c := rep.coverage
	topParts := make([]string, len(c.top))
	for i, t := range c.top {
		topParts[i] = fmt.Sprintf("%s %d", t.mod, t.n)
	}
	out = append(out, fmt.Sprintf("丁 覆盖率（只报告）: 无件 %d / 仍缺 %d = %.1f%% ｜ 前三模块: %s",
		c.wu, rep.remTotal, c.ratio*100, strings.Join(topParts, " · ")))

	e := rep.existence
	out = append(out, fmt.Sprintf("戊 件面现读存在性 : 受检 %d 条（有件行）· 失效件 %d 条 · 存量豁免 %d 条 · 取数口 %s",
		e.checked, len(e.hits), len(e.exempted), e.src))
	out = append(out, fmt.Sprintf("   现读清单        : %d 条（`git ls-files` 现跑 · 无缓存）", e.trackedN))
	for i, h := range e.hits {
		if i >= 40 {
			break
		}
		out = append(out, fmt.Sprintf("   ✗ %s（行 %d）· 失效件 `%s`（unit_source=%s —— 现读清单里没有这条路径）",
			h.gid, h.lineno, h.unit, gapAxisRecPyStrAny(h.unitSource)))
	}
	if len(e.hits) > 40 {
		out = append(out, fmt.Sprintf("   · …（只印前 40 条 · 共 %d 条）", len(e.hits)))
	}
	for i, x := range e.exempted {
		if i >= 40 {
			break
		}
		out = append(out, fmt.Sprintf("   ○ %s（行 %d）· 存量豁免（**不进退码**）· `%s`", x.gid, x.lineno, x.unit))
	}
	out = append(out, fmt.Sprintf("   存量豁免表      : %d 条（本表只增不删 · 逐条带 GAP-id 指针 · 表外才算新失效件）", rep.exemptN))

	k := rep.consistency
	out = append(out, fmt.Sprintf("己 轴间一致性     : 受检 %d 条（有 unit 的行）· 不一致 %d 条 · 口径真源 %s（sha256[0:16]=%s）",
		k.checked, len(k.hits), filepath.Base(k.src), k.srcSha16))
	for i, h := range k.hits {
		if i >= 40 {
			break
		}
		out = append(out, fmt.Sprintf("   ✗ %s（行 %d）· unit=`%s` 账里 module=%s ≠ 口径推导 %s",
			h.gid, h.lineno, h.unit, gapAxisRecPyReprAny(h.got), gapAxisRecPyRepr(h.want)))
		if h.note != "" {
			out = append(out, "      ↳ "+h.note)
		}
	}
	if len(k.hits) > 40 {
		out = append(out, fmt.Sprintf("   · …（只印前 40 条 · 共 %d 条）", len(k.hits)))
	}
	verdict := ""
	switch rep.rc {
	case gapAxisRecRCOK:
		verdict = "PASS（甲/乙 两条等式全成立 · 戊/己 无新命中）"
	case gapAxisRecRCFail:
		verdict = "FAIL（有不等 / 有闭集外形 / 有失效件 / 有轴间不一致）"
	}
	out = append(out, fmt.Sprintf("⇒ rc=%d %s", rep.rc, verdict))
	return strings.Join(out, "\n")
}

// gapAxisRecReportJSON —— `--out` 的机器面（py: json.dump(rep, ensure_ascii=False, indent=1, sort_keys=True)）
func gapAxisRecReportJSON(rep *gapAxisRecReport) gapAxisRecObj {
	axObj := func(a gapAxisRecAxis) gapAxisRecObj {
		return gapAxisRecObj{
			{"anom", a.anom}, {"buckets_wu", a.bucketsWU}, {"buckets_you", a.bucketsYou},
			{"diff", a.diff}, {"key", a.key}, {"outside", a.outside},
			{"sum_all", a.sumAll}, {"sum_wu", a.sumWU}, {"sum_you", a.sumYou},
			{"total", a.total}, {"wu_names", a.wuNames},
		}
	}
	c := rep.coverage
	return gapAxisRecObj{
		{"consistency", gapAxisRecObj{
			{"checked", rep.consistency.checked}, {"hits", rep.consistency.hits},
			{"src", rep.consistency.src}, {"src_sha16", rep.consistency.srcSha16},
		}},
		{"coverage", gapAxisRecObj{
			{"ratio", c.ratio}, {"top_modules", c.top}, {"wu", c.wu}, {"you", c.you},
		}},
		{"exempt_n", rep.exemptN},
		{"existence", gapAxisRecObj{
			{"checked", rep.existence.checked}, {"exempted", rep.existence.exempted},
			{"hits", rep.existence.hits}, {"src", rep.existence.src}, {"tracked_n", rep.existence.trackedN},
		}},
		{"ledger", rep.ledger},
		{"lines", rep.lines},
		{"module", axObj(rep.mod)},
		{"rc", rep.rc},
		{"read_at", rep.readAt},
		{"rem_total", rep.remTotal},
		{"rows", rep.rowsN},
		{"sha16", rep.sha16},
		{"unit", axObj(rep.unit)},
	}
}

// ── list-rules ───────────────────────────────────────────────────────────────

func gapAxisRecListRules(stdout io.Writer) {
	lines := []string{
		fmt.Sprintf("── %s %s · 判据/闭集/取数口径 ──", gapAxisRecTool, gapAxisRecVersion),
		"判据甲  件面等式：按 `unit` 分桶，Σ(有件桶)+Σ(无件桶) == 账内 仍缺 总数",
		"判据乙  模块面等式：同一把尺换 `module`",
		"判据丙  桶名形制：无件桶名 ∈ 闭集；闭集外 ⇒ 逐条点名 rc=1",
		"判据丁  覆盖率：无件 占 仍缺 比例 + 前三模块（**只报告 · 不判红**）",
		"判据戊  件面现读存在性（5b）：有件行的 `unit` 必须在**现读** `git ls-files`（仓根现跑 · 无缓存）里；",
		"        失效件逐条点名（GAP-id 主指针）⇒ rc=1；存量豁免表 `EXEMPT_EXISTENCE` 只打印不进退码（棘轮）",
		"判据己  轴间一致性（5b）：有 `unit` 的行其 `module` 必须 == 由 `unit` 按 `gapModuleOfUnit` 口径推出的值；",
		"        口径**现读**自真源 `core/cmd/zerg/family_gap.go`（三常量 + 手写占位前缀 + filepath.Dir 语义）",
		"桶二分  有件 = 键面存在且值非空、不以 `无件` 开头 ； 无件 = 以 `无件` 开头 或 空串",
		"        **键面缺失**（整个键不在行里）⇒ 不入任一桶 ⇒ 等式差 = 这类条数（逐条点名）",
		"无件闭集: " + gapAxisRecJoinRepr(gapAxisRecWUClosed()),
		"闭集真源: check-gap-ledger-view.py 的 IMPACT_CLOSED = " + strings.Join(gapAxisRecImpactClosed, " / "),
		fmt.Sprintf("己 真源 : %s（现读 · 锚点缺一 ⇒ rc=2 不给结论）", gapAxisRecFamilyGapRel),
		fmt.Sprintf("戊 存量豁免表: %d 条（只增不删 · 逐条带 GAP-id 指针 · 表外才算新）· 首读现读失效件 0 条", len(gapAxisRecExemptExistence)),
		"退码   : 0 甲/乙 全成立且戊/己 无新命中 · 1 有不等/闭集外形/失效件/轴间不一致 · 2 取数面拿不到（BLOCKED，不绿）",
		fmt.Sprintf("取数面 : 账本 --ledger > --state-dir > env ZERG_STATE_DIR > ~/.zerg/state，账本名 = %s", gapAxisRecLedgerName),
		"         仓根 --repo > env ZERG_REPO > 本件位置/现目录 `git rev-parse --show-toplevel`",
		"点名口径: 一律以 **GAP-id** 作主指针（行号只作副信息 —— 账里行号会腐烂）",
		"只读面 : 默认不写任何件；仅 `--out <外部目录>` 时落 json/md 两份",
		"禁项   : 不把整份账打进 stdout（只汇总数 + 点名行）；代码无期望条数常量",
	}
	for _, l := range lines {
		fmt.Fprintln(stdout, l)
	}
}

// ── self-test（成对负控 · 合成账 · 不碰真目标 / 不碰 git）────────────────────

func gapAxisRecPS(s string) *string { return &s }

func gapAxisRecWriteLedger(path string, rows []gapAxisRecObj) error {
	fh, err := os.Create(path)
	if err != nil {
		return err
	}
	defer fh.Close()
	for _, r := range rows {
		if _, err := io.WriteString(fh, gapAxisRecPyJSON(r, -1, 0)+"\n"); err != nil {
			return err
		}
	}
	return nil
}

func gapAxisRecMakeRow(rid, state string, unit, module *string, dropUnit, dropModule bool, src *string) gapAxisRecObj {
	o := gapAxisRecObj{
		{"id", rid}, {"state", state}, {"symptom", "s"}, {"prio", "P1"}, {"impact", "命令面"},
	}
	if unit != nil && !dropUnit {
		o = append(o, gapAxisRecKV{"unit", *unit})
	}
	if module != nil && !dropModule {
		o = append(o, gapAxisRecKV{"module", *module})
	}
	if src != nil {
		o = append(o, gapAxisRecKV{"unit_source", *src})
	}
	return o
}

func gapAxisRecCopyRow(r gapAxisRecObj) gapAxisRecObj {
	out := make(gapAxisRecObj, len(r))
	copy(out, r)
	return out
}

func gapAxisRecDropKey(r gapAxisRecObj, k string) gapAxisRecObj {
	out := make(gapAxisRecObj, 0, len(r))
	for _, kv := range r {
		if kv.k != k {
			out = append(out, kv)
		}
	}
	return out
}

// gapAxisRecSplitLines —— py `str.splitlines()` 的常用面（空串 ⇒ 零行）
func gapAxisRecSplitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

func gapAxisRecRunProbe(stdout io.Writer, ledgerPath, label string, expectRC int, mustContain []string, ctx *gapAxisRecCtx) (bool, int, *gapAxisRecJudgeOut, string) {
	rows, lines, unparsed := gapAxisRecReadLedger(ledgerPath)
	rc, out := gapAxisRecJudge(rows, lines, unparsed, ledgerPath, ctx)
	text := ""
	if rc == gapAxisRecRCBlock {
		text = out.blocked
	} else {
		text = gapAxisRecRender(out.rep)
	}
	ok := rc == expectRC
	if ok {
		for _, m := range mustContain {
			if !strings.Contains(text, m) {
				ok = false
				break
			}
		}
	}
	mark := "✗"
	if ok {
		mark = "✓"
	}
	pad := 58 - utf8.RuneCountInString(label)
	if pad < 0 {
		pad = 0
	}
	fmt.Fprintf(stdout, "  %s%s rc=%d 期望 %d  %s\n", label, strings.Repeat(" ", pad), rc, expectRC, mark)
	if !ok {
		fmt.Fprintln(stdout, "      ---- 实测输出 ----")
		ls := gapAxisRecSplitLines(text)
		if len(ls) > 24 {
			ls = ls[:24]
		}
		for _, ln := range ls {
			fmt.Fprintln(stdout, "      "+ln)
		}
		for _, m := range mustContain {
			if !strings.Contains(text, m) {
				fmt.Fprintf(stdout, "      缺片段: %s\n", gapAxisRecPyRepr(m))
			}
		}
	}
	return ok, rc, out, text
}

const gapAxisRecSelftestN = 17 // 旧有 9 条 + 5b 新增 8 条

func gapAxisRecSelfTest(stdout io.Writer) int {
	tmp, err := os.MkdirTemp("", "gap-axis-selftest-")
	if err != nil {
		tmp = filepath.Join(os.TempDir(), "gap-axis-selftest")
		_ = os.MkdirAll(tmp, 0o700)
	}
	defer os.RemoveAll(tmp)
	fmt.Fprintf(stdout, "── %s --self-test（合成账 · 仓外临时目录 %s）──\n", gapAxisRecTool, tmp)
	ok := true

	fakeRule := &gapAxisRecRule{src: "(合成 · 自检用)", sha16: strings.Repeat("0", 16),
		noFile: "无件(命令面)", noneMod: "无件", rootMod: "仓根", plannedPfx: "拟(", rootDirLit: "."}
	fakeTracked := map[string]bool{"core/a.go": true}
	defCtx := func() *gapAxisRecCtx {
		return &gapAxisRecCtx{tracked: fakeTracked, rule: fakeRule, exempt: []gapAxisRecExemptPair{}}
	}
	emptyExempt := []gapAxisRecExemptPair{}

	good := []gapAxisRecObj{}
	for i := 1; i <= 6; i++ {
		good = append(good, gapAxisRecMakeRow(fmt.Sprintf("P%02d", i), gapAxisRecRemain,
			gapAxisRecPS("core/a.go"), gapAxisRecPS("core"), false, false, nil))
	}
	for i := 1; i <= 4; i++ {
		good = append(good, gapAxisRecMakeRow(fmt.Sprintf("W%02d", i), gapAxisRecRemain,
			gapAxisRecPS("无件(命令面)"), gapAxisRecPS("无件"), false, false, nil))
	}
	withGood := func(extra ...gapAxisRecObj) []gapAxisRecObj {
		out := append([]gapAxisRecObj{}, good...)
		return append(out, extra...)
	}

	// ① 正控：10 条 仍缺（有件 6 · 无件 4）⇒ 两条等式全成立 · 戊/己 无新命中 ⇒ rc=0
	{
		p := filepath.Join(tmp, "good.jsonl")
		_ = gapAxisRecWriteLedger(p, good)
		r, _, _, _ := gapAxisRecRunProbe(stdout, p, "① 正控：Σ有件 6 + Σ无件 4 == 总数 10 ⇒ rc=0", gapAxisRecRCOK,
			[]string{"Σ=6", "Σ=4", "差 = 0", "戊 件面现读存在性 : 受检 6 条", "失效件 0 条"}, defCtx())
		ok = ok && r
	}

	// ② 负控（**坏账**）：把「有件桶」里 3 条的 `unit` 键整个删掉 ⇒ 3 条不入任一桶 ⇒ 桶和 7 ≠ 10
	{
		bad := make([]gapAxisRecObj, len(good))
		for i, x := range good {
			bad[i] = gapAxisRecCopyRow(x)
		}
		for i := 0; i < 3; i++ {
			bad[i] = gapAxisRecDropKey(bad[i], "unit")
		}
		p := filepath.Join(tmp, "bad.jsonl")
		_ = gapAxisRecWriteLedger(p, bad)
		r, _, rep, _ := gapAxisRecRunProbe(stdout, p, "② 负控：删掉 3 条的 `unit` 键 ⇒ Σ 7 ≠ 10 ⇒ rc=1 + 点名", gapAxisRecRCFail,
			[]string{"差 = 3", "键面缺失", "P01", "P02", "P03"}, defCtx())
		ok = ok && r && rep.rep.unit.diff == 3
	}

	// ③ 负控：闭集外无件形态 ⇒ rc=1 逐条点名
	{
		p := filepath.Join(tmp, "outside.jsonl")
		_ = gapAxisRecWriteLedger(p, withGood(gapAxisRecMakeRow("X01", gapAxisRecRemain,
			gapAxisRecPS("无件(乱写)"), gapAxisRecPS("无件"), false, false, nil)))
		r, _, _, _ := gapAxisRecRunProbe(stdout, p, "③ 负控：无件桶名 `无件(乱写)` 在闭集外 ⇒ rc=1 点名", gapAxisRecRCFail,
			[]string{"闭集外形", "无件(乱写)"}, defCtx())
		ok = ok && r
	}

	// ④ 正控：裸 `无件`（旧形态）+ 空串（"空"亦属无件形态）都落在闭集内 ⇒ rc=0
	{
		p := filepath.Join(tmp, "bare.jsonl")
		_ = gapAxisRecWriteLedger(p, withGood(
			gapAxisRecMakeRow("B01", gapAxisRecRemain, gapAxisRecPS("无件"), gapAxisRecPS("仓根"), false, false, nil),
			gapAxisRecMakeRow("B02", gapAxisRecRemain, gapAxisRecPS(""), gapAxisRecPS("无件"), false, false, nil)))
		r, _, rep, _ := gapAxisRecRunProbe(stdout, p, "④ 正控：裸 `无件` / 空串 皆落无件桶且在闭集内 ⇒ rc=0", gapAxisRecRCOK,
			[]string{"差 = 0", "'无件'"}, defCtx())
		ok = ok && r && rep.rep.unit.sumWU == 6
	}

	// ⑤ 正控：`state` 不是 仍缺 的行 —— 不进分子亦不进分母 ⇒ 读数不变
	{
		p := filepath.Join(tmp, "otherstate.jsonl")
		_ = gapAxisRecWriteLedger(p, withGood(
			gapAxisRecMakeRow("D01", "已解", gapAxisRecPS("core/z.go"), gapAxisRecPS("core"), false, false, nil),
			gapAxisRecMakeRow("D02", "不做", gapAxisRecPS("无件(命令面)"), gapAxisRecPS("无件"), false, false, nil)))
		r, _, rep, _ := gapAxisRecRunProbe(stdout, p, "⑤ 正控：state 非 仍缺 的行 不进分子亦不进分母 ⇒ rc=0", gapAxisRecRCOK,
			[]string{"仍缺 10", "差 = 0"}, defCtx())
		ok = ok && r && rep.rep.remTotal == 10
	}

	// ⑥ 负控（取数面）：整行 JSON 坏 ⇒ 必须 rc=2 BLOCKED（**不当绿**）
	{
		p := filepath.Join(tmp, "broken.jsonl")
		_ = os.WriteFile(p, []byte("{\"id\":\"Q1\",\"state\":\"仍缺\",\"unit\":\"core/a.go\",\"module\":\"core\"}\n}{ 这不是 json\n"), 0o644)
		r, _, _, _ := gapAxisRecRunProbe(stdout, p, "⑥ 负控：有行解不开 ⇒ rc=2 BLOCKED（不绿）", gapAxisRecRCBlock, nil, defCtx())
		ok = ok && r
	}

	// ⑦ 负控（取数面）：账本不在盘 ⇒ rc=2
	{
		r, _, _, _ := gapAxisRecRunProbe(stdout, filepath.Join(tmp, "nope.jsonl"),
			"⑦ 负控：账本不在盘 ⇒ rc=2 BLOCKED", gapAxisRecRCBlock, nil, defCtx())
		ok = ok && r
	}

	// ⑧ 负控（取数面）：空账 ⇒ rc=2（空转 = 假绿，不给结论）
	{
		p := filepath.Join(tmp, "empty.jsonl")
		_ = gapAxisRecWriteLedger(p, []gapAxisRecObj{})
		r, _, _, _ := gapAxisRecRunProbe(stdout, p, "⑧ 负控：0 行 ⇒ rc=2（空转不给结论）", gapAxisRecRCBlock, nil, defCtx())
		ok = ok && r
	}

	// ⑨ 负控：模块面等式 —— 删掉 `module` 键 ⇒ 乙 差 = 3
	{
		bad2 := make([]gapAxisRecObj, len(good))
		for i, x := range good {
			bad2[i] = gapAxisRecCopyRow(x)
		}
		for i := 0; i < 3; i++ {
			bad2[i] = gapAxisRecDropKey(bad2[i], "module")
		}
		p := filepath.Join(tmp, "badmod.jsonl")
		_ = gapAxisRecWriteLedger(p, bad2)
		r, _, rep, _ := gapAxisRecRunProbe(stdout, p, "⑨ 负控：删掉 3 条的 `module` 键 ⇒ 乙 差 = 3 ⇒ rc=1", gapAxisRecRCFail,
			[]string{"差 = 3", "缺 `module` 键"}, defCtx())
		ok = ok && r && rep.rep.mod.diff == 3
	}

	// ⑩ 戊 负控（**5b 新**）：合成一条 `unit` 指向**盘上不存在**的件 ⇒ rc=1 且点名那条 GAP-id
	{
		p := filepath.Join(tmp, "exist-hit.jsonl")
		_ = gapAxisRecWriteLedger(p, withGood(gapAxisRecMakeRow("GAP-20260928-9001", gapAxisRecRemain,
			gapAxisRecPS("scripts/gates/不存在的件.py"), gapAxisRecPS("scripts/gates"), false, false, gapAxisRecPS("auto"))))
		r, _, rep, _ := gapAxisRecRunProbe(stdout, p, "⑩ 戊 负控：unit 指向不存在件 ⇒ rc=1 且点名 GAP-id", gapAxisRecRCFail,
			[]string{"GAP-20260928-9001", "失效件 1 条", "失效件 `scripts/gates/不存在的件.py`"}, defCtx())
		ok = ok && r && len(rep.rep.existence.hits) == 1
	}

	// ⑪ 戊 正控（**5b 新**）：`unit` 为**无件闭集内**值 ⇒ **不进戊**（受检 0 条 · 不判红）
	{
		p := filepath.Join(tmp, "exist-wu.jsonl")
		_ = gapAxisRecWriteLedger(p, []gapAxisRecObj{gapAxisRecMakeRow("GAP-20260928-9002", gapAxisRecRemain,
			gapAxisRecPS("无件(命令面)"), gapAxisRecPS("无件"), false, false, gapAxisRecPS("hand"))})
		r, _, rep, _ := gapAxisRecRunProbe(stdout, p, "⑪ 戊 正控：unit 为无件闭集内值 ⇒ 不进戊（受检 0）⇒ rc=0", gapAxisRecRCOK,
			[]string{"受检 0 条（有件行）", "失效件 0 条"}, defCtx())
		ok = ok && r && rep.rep.existence.checked == 0
	}

	// ⑫ 己 负控（**5b 新**）：合成一条 `module` 与 `unit` 矛盾 ⇒ rc=1 且点名那条 GAP-id
	{
		p := filepath.Join(tmp, "consist-hit.jsonl")
		_ = gapAxisRecWriteLedger(p, withGood(gapAxisRecMakeRow("GAP-20260928-9003", gapAxisRecRemain,
			gapAxisRecPS("core/a.go"), gapAxisRecPS("scripts"), false, false, gapAxisRecPS("auto"))))
		r, _, rep, _ := gapAxisRecRunProbe(stdout, p, "⑫ 己 负控：module 与 unit 矛盾 ⇒ rc=1 且点名", gapAxisRecRCFail,
			[]string{"GAP-20260928-9003", "不一致 1 条", "账里 module='scripts' ≠ 口径推导 'core'"}, defCtx())
		ok = ok && r && len(rep.rep.consistency.hits) == 1
	}

	// ⑬ 己 正控（**5b 新**）：一致行（含仓根件 `仓根` 那一支 + 无件行 `无件` 那一支）⇒ rc=0
	{
		p := filepath.Join(tmp, "consist-ok.jsonl")
		_ = gapAxisRecWriteLedger(p, withGood(
			gapAxisRecMakeRow("GAP-20260928-9004", gapAxisRecRemain, gapAxisRecPS("README.md"), gapAxisRecPS("仓根"), false, false, gapAxisRecPS("auto")),
			gapAxisRecMakeRow("GAP-20260928-9005", gapAxisRecRemain, gapAxisRecPS("无件(命令面)"), gapAxisRecPS("无件"), false, false, gapAxisRecPS("hand"))))
		r, _, rep, _ := gapAxisRecRunProbe(stdout, p, "⑬ 己 正控：module 与口径推导一致（含 仓根/无件 两支）⇒ rc=0", gapAxisRecRCOK,
			[]string{"不一致 0 条", "受检 12 条（有 unit 的行）"},
			&gapAxisRecCtx{tracked: map[string]bool{"core/a.go": true, "README.md": true}, rule: fakeRule, exempt: emptyExempt})
		ok = ok && r && len(rep.rep.consistency.hits) == 0
	}

	// ⑭ 戊 存量豁免（**5b 新**）：同一条失效件，进了豁免表 ⇒ 只打印、**不进退码**（rc=0）
	p14 := filepath.Join(tmp, "exist-exempt.jsonl")
	{
		_ = gapAxisRecWriteLedger(p14, withGood(gapAxisRecMakeRow("GAP-20260928-9006", gapAxisRecRemain,
			gapAxisRecPS("scripts/gates/存量差件.py"), gapAxisRecPS("scripts/gates"), false, false, gapAxisRecPS("auto"))))
		r, _, rep, _ := gapAxisRecRunProbe(stdout, p14, "⑭ 戊 存量豁免：失效件进表 ⇒ 打印但不进退码 ⇒ rc=0", gapAxisRecRCOK,
			[]string{"失效件 0 条", "存量豁免 1 条", "○ GAP-20260928-9006", "不进退码"},
			&gapAxisRecCtx{tracked: fakeTracked, rule: fakeRule,
				exempt: []gapAxisRecExemptPair{{"GAP-20260928-9006", "scripts/gates/存量差件.py"}}})
		ok = ok && r && len(rep.rep.existence.exempted) == 1 && len(rep.rep.existence.hits) == 0
	}

	// ⑮ 取数面（**5b 新**）：现读清单拿不到 ⇒ rc=2（**不给绿也不给红**）
	{
		r, _, _, _ := gapAxisRecRunProbe(stdout, p14, "⑮ 负控：戊 取数面（ls-files）拿不到 ⇒ rc=2 不给结论", gapAxisRecRCBlock, nil,
			&gapAxisRecCtx{rule: fakeRule, exempt: emptyExempt, trackedErr: "合成：git ls-files 拿不到"})
		ok = ok && r
	}

	// ⑯ 取数面（**5b 新**）：真源口径锚点没命中 ⇒ rc=2
	{
		r, _, _, _ := gapAxisRecRunProbe(stdout, p14, "⑯ 负控：己 真源口径没现读出来 ⇒ rc=2 不给结论", gapAxisRecRCBlock, nil,
			&gapAxisRecCtx{tracked: fakeTracked, exempt: emptyExempt, ruleErr: "合成：真源口径锚点缺"})
		ok = ok && r
	}

	// ⑰ 己 真源口径边界（**5b 新**）：闭集里的**其他**无件形态在真源 `gapModuleOfUnit` 里**不在**
	// `无件` 那一支 ⇒ 推成 `仓根`；账里写 `无件` ⇒ 命中。命中行会**带说明**（不静默）。
	{
		p := filepath.Join(tmp, "consist-nuance.jsonl")
		_ = gapAxisRecWriteLedger(p, withGood(gapAxisRecMakeRow("GAP-20260928-9007", gapAxisRecRemain,
			gapAxisRecPS("无件(门禁面)"), gapAxisRecPS("无件"), false, false, gapAxisRecPS("hand"))))
		r, _, rep, _ := gapAxisRecRunProbe(stdout, p, "⑰ 己 真源口径边界：`无件(门禁面)`+module`无件` ⇒ rc=1 带说明", gapAxisRecRCFail,
			[]string{"GAP-20260928-9007", "不一致 1 条", "真源口径边界", "⇒ `仓根`", "闭集内其他无件形态不在真源这一支里"}, defCtx())
		ok = ok && r && len(rep.rep.consistency.hits) == 1
	}

	verdict := "FAIL"
	if ok {
		verdict = "PASS"
	}
	fmt.Fprintf(stdout, "⇒ 自检 %s（%d 条断言 · 旧有 9 条 + 5b 新增 8 条）\n", verdict, gapAxisRecSelftestN)
	if ok {
		return gapAxisRecRCOK
	}
	return gapAxisRecRCFail
}

// ── 入口（= py `main()`）────────────────────────────────────────────────────

// gapAxisRecUsageBlock —— argparse 的用法块（宽 80 的换行形态 · 见件头未覆盖面①）
const gapAxisRecUsageBlock = `usage: check-gap-axis-reconcile.py [-h] [--ledger LEDGER]
                                   [--state-dir STATE_DIR] [--repo REPO]
                                   [--out OUT] [--json] [--list-rules]
                                   [--self-test]`

// gapAxisRecHelpText —— argparse 的 `-h/--help` 正文（现读自 py 3.9 argparse · 宽 80 的换行形态）
const gapAxisRecHelpText = gapAxisRecUsageBlock + `

门75 缺口账两轴对账：件面/模块面 桶和 == 账内 仍缺 总数（不等即红 · 阻断）

optional arguments:
  -h, --help            show this help message and exit
  --ledger LEDGER       账本件（默认按取数面推）
  --state-dir STATE_DIR
                        状态目录（默认 env ZERG_STATE_DIR / ~/.zerg/state）
  --repo REPO           仓根（默认 env ZERG_REPO / 现跑 git rev-parse）—— 戊/己 的取数面
  --out OUT             **外部**输出目录（给了才落 json/md 两份）
  --json                末尾追加机器面一行
  --list-rules
  --self-test`

// gapAxisRecArgErr —— argparse 风格用法错（用法块 + error 行 · 退码同 2）。
// ✗ 唯一前缀缩写（`--js` 一类）与 COLUMNS≠80 的换行形态仍在未覆盖面①。
func gapAxisRecArgErr(stderr io.Writer, msg string) int {
	fmt.Fprintln(stderr, gapAxisRecUsageBlock)
	fmt.Fprintf(stderr, "check-gap-axis-reconcile.py: error: %s\n", msg)
	return gapAxisRecRCBlock
}

func checkGapAxisReconcileGateMain(stdout, stderr io.Writer, argv []string) int {
	ledgerArg, stateDir, repo, outDir := "", "", "", ""
	jsonOut, listRules, selfTest := false, false, false
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "-h" || a == "--help":
			fmt.Fprintln(stdout, gapAxisRecHelpText)
			return gapAxisRecRCOK
		case a == "--json":
			jsonOut = true
		case a == "--list-rules":
			listRules = true
		case a == "--self-test":
			selfTest = true
		case a == "--ledger":
			i++
			if i >= len(argv) {
				return gapAxisRecArgErr(stderr, "argument --ledger: expected one argument")
			}
			ledgerArg = argv[i]
		case strings.HasPrefix(a, "--ledger="):
			ledgerArg = a[len("--ledger="):]
		case a == "--state-dir":
			i++
			if i >= len(argv) {
				return gapAxisRecArgErr(stderr, "argument --state-dir: expected one argument")
			}
			stateDir = argv[i]
		case strings.HasPrefix(a, "--state-dir="):
			stateDir = a[len("--state-dir="):]
		case a == "--repo":
			i++
			if i >= len(argv) {
				return gapAxisRecArgErr(stderr, "argument --repo: expected one argument")
			}
			repo = argv[i]
		case strings.HasPrefix(a, "--repo="):
			repo = a[len("--repo="):]
		case a == "--out":
			i++
			if i >= len(argv) {
				return gapAxisRecArgErr(stderr, "argument --out: expected one argument")
			}
			outDir = argv[i]
		case strings.HasPrefix(a, "--out="):
			outDir = a[len("--out="):]
		default:
			return gapAxisRecArgErr(stderr, "unrecognized arguments: "+a)
		}
	}
	if listRules {
		gapAxisRecListRules(stdout)
		return gapAxisRecRCOK
	}
	if selfTest {
		return gapAxisRecSelfTest(stdout)
	}

	ledger := gapAxisRecResolveLedger(ledgerArg, stateDir)
	root := gapAxisRecResolveRepo(repo)
	tracked, trackedErr := gapAxisRecGitTracked(root)
	rule, ruleErr := gapAxisRecLoadModRule(root)
	ctx := &gapAxisRecCtx{tracked: tracked, rule: rule, exempt: gapAxisRecExemptExistence,
		trackedErr: trackedErr, ruleErr: ruleErr}
	rows, lines, unparsed := gapAxisRecReadLedger(ledger)
	rc, jo := gapAxisRecJudge(rows, lines, unparsed, ledger, ctx)
	if rc == gapAxisRecRCBlock {
		fmt.Fprintf(stdout, "── %s ──\n", gapAxisRecGateName)
		fmt.Fprintf(stdout, "取数面 : %s\n", ledger)
		fmt.Fprintf(stdout, "现读时刻: %s\n", gapAxisRecNowISO())
		fmt.Fprintf(stdout, "⇒ rc=2 BLOCKED（取数面拿不到）：%s\n", jo.blocked)
		u := jo.unparsed
		if len(u) > 20 {
			u = u[:20]
		}
		for _, b := range u {
			fmt.Fprintf(stdout, "   · 行 %s · %s\n", b.lineno, b.why)
		}
		if jsonOut {
			fmt.Fprintln(stdout, gapAxisRecPyJSON(gapAxisRecObj{
				{"tool", gapAxisRecTool}, {"rc", 2}, {"blocked", jo.blocked},
				{"ledger", ledger}, {"read_at", gapAxisRecNowISO()},
			}, -1, 0))
		}
		return gapAxisRecRCBlock
	}

	fmt.Fprintln(stdout, gapAxisRecRender(jo.rep))
	if outDir != "" {
		_ = os.MkdirAll(outDir, 0o755)
		if fh, err := os.Create(filepath.Join(outDir, "gap-axis-reconcile.json")); err == nil {
			_, _ = io.WriteString(fh, gapAxisRecPyJSON(gapAxisRecReportJSON(jo.rep), 1, 0)+"\n")
			_ = fh.Close()
		}
		if fh, err := os.Create(filepath.Join(outDir, "gap-axis-reconcile.md")); err == nil {
			_, _ = io.WriteString(fh, gapAxisRecRender(jo.rep)+"\n")
			_ = fh.Close()
		}
		fmt.Fprintf(stdout, "外部落盘: %s/{gap-axis-reconcile.json,gap-axis-reconcile.md}\n", outDir)
	}
	if jsonOut {
		rep := jo.rep
		axisJSON := func(a gapAxisRecAxis) gapAxisRecObj {
			return gapAxisRecObj{
				{"buckets_you", a.bucketsYou}, {"buckets_wu", a.bucketsWU},
				{"sum_you", a.sumYou}, {"sum_wu", a.sumWU}, {"diff", a.diff},
			}
		}
		fmt.Fprintln(stdout, gapAxisRecPyJSON(gapAxisRecObj{
			{"sha16", rep.sha16}, {"read_at", rep.readAt}, {"lines", rep.lines},
			{"rows", rep.rowsN}, {"rem_total", rep.remTotal}, {"rc", rep.rc},
			{"tool", gapAxisRecTool},
			{"unit", axisJSON(rep.unit)}, {"module", axisJSON(rep.mod)},
			{"coverage_wu_ratio", gapAxisRecPyRound4(rep.coverage.ratio)},
			{"existence_checked", rep.existence.checked},
			{"existence_hits", len(rep.existence.hits)},
			{"existence_exempted", len(rep.existence.exempted)},
			{"consistency_checked", rep.consistency.checked},
			{"consistency_hits", len(rep.consistency.hits)},
			{"modrule_src_sha16", rep.consistency.srcSha16},
		}, -1, 0))
	}
	return rc
}
