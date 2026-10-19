// check_ai_direct_edits.go —— 门禁件 `scripts/gates/check-ai-direct-edits.py` 的 **Go 真搬样板**（v2.5.14 · M2-0 · 路 A · 首字母 a–e 批）。
//
// 为什么有这件（不是「顺手 Go 化」）：
//
//	v2.5.14 定案（`设计-判据 CLI 化-v1.0-20260930.md` 第四节）把判据实现面分两段走 ——
//	M1 = 先把 `scripts/` 下的独立件都收成 CLI 打得着的统一面（转发壳）；
//	M2 = 按价值**真搬 Go**（逐批判据搬 Go · 落点 `core/cmd/zerg/`）。
//	本件 = M2-0 批（无输入物料、纯静态检查的判据）的一枚样板：
//	验收判据 = 对同一件跑两态（原件 ↔ 新入口），**stdout / stderr / 退码三面逐字相同**（4.2）。
//
// 选它的理由（M2-0 = 「无外部树依赖 · 输出确定有序 · 退码 0/1/2 · 纯静态无写面 · 行数适中」）：
//
//	✓ 选：`check-ai-direct-edits.py`（258 行）——判据面只读**仓内**一件
//	  `core/cmd/zerg/testdata/cli-matrix.json`（门⑪ 的同一份投影）＋ **一个回执目录**
//	  （默认 `$ZERG_RECEIPT_DIR` / `$ZERG_STATE_DIR/receipts` / `~/.zerg/state/receipts`，可用 `--dir` 指）——
//	  **无外部树依赖**（不碰文档仓 / 公开产物树 / `bin/`）。
//	  · 输出**定序**：回执页 `sorted(os.listdir(...))` + 逐页判定序固定（REQ 序 R2→R3→R1→R4），
//	    无 `os.walk` / 无「按 inode 序」读取 ⇒ 逐字节对拍可证。
//	  · 退码 `0/1/2` 三档齐：0 全绿 / 1 门禁红（R1–R4 任一）/ 2 不给结论（空转 · 目录不存在 · 未知旗标 · `--json` 无机器面）。
//	  · **纯静态、无写面**（默认档不写任何件；仅 `--self-test` 在临时目录造夹具 —— 与 py 同）。
//	  · `git status` 现读**不含它**（无人正在改）。
//	✗ 不选 `check-ci-package-parity.py` / `check-contract-registry.py` / `check-error-kinds.py`：**已落仓**（同批前单已搬，`git status` 见 `?? core/cmd/zerg/check_*.go`）。
//	✗ 不选 `check-curl-timeout.py`（167 行）：扫描根靠 `os.walk` ⇒ **遍历序非确定**（Go `WalkDir` 字典序 ≠ py `scandir` inode 序）⇒ 不满足「输出有序」。
//	✗ 不选 `check-doc-section-numbers.py`（189 行）等 8 件 `doc-*`：**另一路在改**（`git status` 现读 M）⇒ 违反 M2 并行路铁律②（共享件）。
//	✗ 不选 `check-anchor-uniqueness.py`（340 行）/ `check-compat-manifest.py`（449 行）/ `check-affected-map.py`（523 行）/ `check-exit-codes.py`（649 行）/ `check-build-tags.py`（863 行）：判据面**依赖同级外部树或输入物料**（案卷/清单 JSON 由调用方指定 · 可指向仓外）⇒ 不属 M2-0「无输入物料」，且部分**行数偏大**。
//
// ── ★ 本批新增 R5（缺口 `GAP-20260927-81` · 门⑬「既漏又假」的两半）──────────────────────
//
//	旧行为的两半：**假** = R1 只要 `evidence` 里出现**任意** `zerg <词>`（含**只读**调用）就放行
//	⇒ 一次「手改后补一条含任意 zerg 调用的回执」即可绕绿（报绿而 H1 根本没判）；
//	**漏** = 本门只扫**回执目录**一层 `.json`，**不读件级账**（`edit_audit.jsonl`）⇒ 回执账一旦断供
//	（轮级账停在某天、件级账里主仓写轮仍天天在涨），本门照报「零直改纪律全过」—— 那个绿与今天的改动无关。
//
//	修法（两半同批 · 与 py 逐条对齐）= 把**件级账的主仓写轮行**（`event == "edit"` 且非 `sibling_repo`）
//	当第二个判据源，只做**轮级/覆盖**对账（**不**做逐件匹配 —— 两账无公共键：件级 vs 轮级）：
//
//	· **R5a（假 · 写轮对账 · 代拍口径 GAP-20261015-05）**：每一「说动过仓」的页，其**轮窗**
//	  （前页回执之后、本页 `created_at` 之前）内件级账必须有 ≥1 主仓写轮行（**轮窗腿** · 旧断言原样）；
//	  轮窗空 ⇒ 再逐件看该页 `--file` 清单里的**源码面**件（**件级腿**）：该件有 ≤ 页时刻的 `dev edit` 行
//	  （即「上一回执以来 ∪ 该件最近一次 dev edit」）⇒ 放行，无 ⇒ 红。
//	  · **生成面**（门/器产出的派生件：`docs/**`·`publish/docs/**`·`项目文档/**`·契约/兼容/门清单等
//	    注册表·矩阵·台账·`Cargo.lock`·词表·探针物料）与 **删除面**（件**已不在 `git ls-files`**，
//	    删除走 `git rm`）**只报告 · 不进 rc** —— 两类结构上永远没有 `dev edit` 行。
//	  · 页证据里**没有可解析的 `--file` 清单** ⇒ 件面判不了 ⇒ 按旧口径落红（旧断言不放宽）。
//	  · **署名面分流（L1 · 用户已拍）**：本门 want = 「**AI** 绕正门直写」—— 该轮提交信息带
//	    `由:` trailer 且值**非 AI 面**（人署名）⇒ **只报告、不进 rc**；带 AI 面署名（`Hermes`/`父代理`）
//	    或**取不到署名**（无 trailer / 未声明 / git 取不到）⇒ **仍红**（fail-closed，✗ 默认放行）。
//	    ★ git 的 `%an`/`%cn` **不是**判据面（全仓一律 `Mr2109` ⇒ 零鉴别力）。
//	  ★ 页 `created_at` **早于件级账最早一行** ⇒ 本腿**不上判**（件级账判不了它 —— 不许拿缺账当红）。

//	· **R5b（漏 · 覆盖自证）**：件级账里最新主仓写轮**晚于**最新回执 ⇒ **rc=2 判不了**（不得报绿）。
//	  ★ F35（拍定 A1）：回执页**无任何 `created_at`** ⇒ 无时间基准可比 ⇒ **亦判 rc=2**
//	    —— ✗ 不得静默跳过该支报绿（旧行为：rc=0 且印不实的「双向对账 ✓」）。
//
//	对拍面（本次新增 · 仓外夹具两态逐字）：R5a 红格（单页早于件级账起点 ⇒ 不上判 · 两页且后页该窗零写轮 ⇒ 红）·
//	R5b 空窗格（件级账最新写轮晚于最新回执 ⇒ 2）· sibling 行不上判格 · 空件级账格。
//
// ⚠ 本单边界（照实）：本件**只落码、不接线** ——
//
//	· **不**在 `core/cmd/zerg/main.go` 里注册任何命令（✗ 不进命令树 · ✗ 不改任何 `add_step`）；
//	· **不**删 `scripts/gates/check-ai-direct-edits.py` 原件（退场规则 4.3：**对拍通过后才**退场，且逐件显式点名）；
//	· 入口 `aidirecteditsGateMain` 目前**无调用点** —— 这是 M2-0 的**刻意**状态：
//	  先证明「搬得逐字相同」，接线与命令注册是后一单的事。
//
// 入口签名（与 py 的 `main()` 逐字对齐）：
//
//	func aidirecteditsGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv  —— 与 py `sys.argv[1:]` 同形（不含程序名）
//	  · 退码  —— 0 通过 / 1 门禁红 / 2 不给结论（与 py 三码逐字对齐）
//	  · 输出  —— stdout/stderr 与 py 逐字节对齐（见下「已对拍面 / 未覆盖面」）
//
// ── ★ 两条标准声明行（M2 样板必备）──────────────────────────────────────────────
//
//	(a) **默认根定位差异**：py `DEFAULT_ROOT = dirname(dirname(HERE))`，`HERE = dirname(abspath(__file__))`
//	    ⇒ 由**脚本位置**（`scripts/gates` 上溯两层）**固定**指向仓根，与 cwd 无关；Go 入口**无 `__file__` 语境**
//	    ⇒ 本件默认根取 **cwd**（`os.Getwd()`）。同真仓 + cwd=仓根 时两者读数逐字相同（已对拍 C14）；
//	    「脚本位置 ≠ cwd」时路径推导面不同 ⇒ 对拍一律显式传 `--root`。
//	(b) **自指检查**：本件扫描面 = **一个回执目录**（state 树，默认 `~/.zerg/state/receipts`）＋ **仓内 `cli-matrix.json`**，
//	    **不读** `scripts/gates/` · **不读** `core/cmd/zerg/*.go` ⇒ **本件不在自己扫描面内**。
//	    故夹具 / 自检 / 件头里的标记字面（`zerg gate run` `/ §20.3` / `TR-…`）**无需**回避反引号原串直写 ——
//	    即**不存在** `check_wall_layer.go` 血证（曾把门⑥ 茧壁凭空判红）里那种「本件在自己扫描面内」的风险。
//
// 已对拍面（仓外夹具两态逐字 · ≥14 格）：
//
//	全绿 · 各造 violation（R1 AI 直改 / R2 缺格 / R2 trace 形态 / R3 件名 ≠ round_id / R4 幽灵调用）·
//	成对负控（动过仓 ↔ 没动仓）· 多页定序 · 空转（回执目录 0 页）⇒ 2 · 取不到输入（回执目录不存在）⇒ 2 ·
//	未知旗标 ⇒ 2 · 位置参数 ⇒ 2 · `--json` / `--json=stdout` 无机器面拒收 ⇒ 2 · `-h` / `--help` ⇒ 0 ·
//	`--self-test` 三档 · 真仓现读（显式根 / 缺省根 · cwd=仓根）。
//
// 未覆盖面（✗ 不声称逐字对齐 · 逐条已列）：
//
//	① **回执坏 JSON 的异常正文**：py `json.load` 抛 `json.decoder.JSONDecodeError`，错误行 `R2 回执不是合法 JSON：%s（%s）`
//	   的第②个 `%s` = **CPython 的 JSON 扫描器文案**（`Expecting property name enclosed in double quotes: line 1 column 2 (char 1)` 一类）；
//	   本件落 Go `encoding/json` 的文案 ⇒ **退码同为 1，但该行正文不同**。故对拍表**不收**「坏 JSON」格。
//	② **`cli-matrix.json` 读不出来的失败行**：py `open/json` 抛的 `FileNotFoundError str` 进 `命令树投影读不出来（…）：<e>`
//	   的 `<e>` 位；Go 文案不同 ⇒ 同样**退码 2、正文不同**。故对拍表**不收**「根下无矩阵」格（「取不到输入」改用**回执目录不存在**那一格，该格 py 文案与 Go 逐字同）。
//	③ **回执字段非字符串**（`round_id`/`trace_id`/`what`/`next` 为 `null`/数/数组）：py 走 `str(...)` / `%r` 的 **Python repr**；
//	   本件对**标量**（串/布尔/数/null）逐字复刻，**嵌套容器**（数组/对象）的 `str()` repr 未复刻 ⇒ 不属逐字对齐承诺。
//	④ **argparse 前缀缩写**（`--ro`/`--di`）与 `--help=…` 等写法：py argparse **收**缩写；本件只认全称 ⇒ 不属承诺。
//	⑤ **EPIPE / 写端关闭**：py 的 `print` 抛 `BrokenPipeError`（进程退 1）；Go `fmt.Fprintf` 吞错返回 ⇒ 不属承诺。
//	⑥ py `--self-test` 用 `subprocess` **真跑自身进程**取真退出码；本件 `--self-test` **进程内**复用同一入口取 rc
//	   （打印文案逐字对齐；差在实现手段，不在读数）。若 `sys.executable` 缺失/py 件被移走，py 自检会报错而本件不受影响 ⇒ 这一档不属承诺。
//
//	—— ①②③④⑤⑥ 均**不在**本单对拍输入集内；接线前须逐面补齐或显式剔除。
package main

import (
	"bytes"
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
)

// ── 与 py 逐条对齐的常量 / 正则（名字前缀 `ade` 防包内同名）────────────────────────

// adeMatrixRel —— py `MATRIX_REL`（命令树投影相对仓根的路径）。
const adeMatrixRel = pathCliMatrixJSON

// adeLedgerRel —— py `LEDGER_REL`（件级账相对**状态目录**的件名）。
const adeLedgerRel = "edit_audit.jsonl"

// adeProg —— py argparse 的 prog（= 原件基名，帮助/报错正文里逐字出现）。
const adeProg = "check-ai-direct-edits.py"

// adeUsageLine —— argparse 报错首行（含 `usage:`，逐字）。
const adeUsageLine = "usage: check-ai-direct-edits.py [-h] [--root ROOT] [--dir DIR] [--ledger LEDGER] [--self-test]\n"

// adeHelp —— `-h/--help` 正文（现读复刻 · 现读 py = `/usr/bin/python3` 3.9.6 ⇒ argparse 段标题 `optional arguments:`；
// 换 Python 版本（≥3.10 段标题作 `options:` / 措辞）即不同 ⇒ 见件头未覆盖面④延伸）。
const adeHelp = "usage: check-ai-direct-edits.py [-h] [--root ROOT] [--dir DIR] [--ledger LEDGER] [--self-test]\n" +
	"\n" +
	"optional arguments:\n" +
	"  -h, --help   show this help message and exit\n" +
	"  --root ROOT\n" +
	"  --dir DIR\n" +
	"  --ledger LEDGER\n" +
	"  --self-test\n"

// adeREQ —— py `REQ`（R2 五格中的「必填四格」）。
var adeREQ = []string{"round_id", "trace_id", "what", "next"}

// adeTraceRE —— py `TRACE_RE = ^TR-[0-9a-f]{12}$`。
var adeTraceRE = regexp.MustCompile(`^TR-[0-9a-f]{12}$`)

// adeZergRE —— py `ZERG_RE = (?:^|[\s（(])zerg\s+([A-Za-z][A-Za-z0-9_-]*)`。
var adeZergRE = regexp.MustCompile(`(?:^|[\s（(])zerg\s+([A-Za-z][A-Za-z0-9_-]*)`)

// adeNoncmdRE —— py `ZERG_NONCMD_RE`（缺口 `GAP-20260927-59`：`zerg --help` / `-h` / 版本号一类非子命令调用）。
var adeNoncmdRE = regexp.MustCompile(`(?:^|[\s（(])zerg\s+(?:--?[A-Za-z]|[0-9])`)

// adeJSONFields —— py `JSON_FIELDS`（本门无机器读面 ⇒ 空 · **唯一真源**）。
var adeJSONFields = []string{}

// ── py `default_dir()` ──────────────────────────────────────────────────────

func adeDefaultDir() string {
	if d := strings.TrimSpace(os.Getenv("ZERG_RECEIPT_DIR")); d != "" {
		return d
	}
	if s := strings.TrimSpace(os.Getenv("ZERG_STATE_DIR")); s != "" {
		return filepath.Join(s, "receipts")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}
	return filepath.Join(home, ".zerg", "state", "receipts")
}

// ── py `default_ledger()`（件级账默认落点：状态目录下一件 · 与回执账同根）──────

func adeDefaultLedger() string {
	if s := strings.TrimSpace(os.Getenv("ZERG_STATE_DIR")); s != "" {
		return filepath.Join(s, adeLedgerRel)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}
	return filepath.Join(home, ".zerg", "state", adeLedgerRel)
}

// ── py `default root`（差异见件头声明行 (a)）────────────────────────────────
//
// py：DEFAULT_ROOT = dirname(dirname(HERE))（HERE = 脚本所在 `scripts/gates`）⇒ 与 cwd 无关。
// Go：无 `__file__` ⇒ 取 cwd（同真仓 + cwd=仓根 时逐字相同）。
func adeDefaultRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}

// ── py `load_command_tree(root)` ────────────────────────────────────────────

func adeLoadCommandTree(root string) (map[string]bool, string) {
	p := filepath.Join(root, adeMatrixRel)
	b, err := os.ReadFile(p)
	if err != nil {
		// py 的异常正文（FileNotFoundError str）与 Go 不同 ⇒ 见件头未覆盖面②。
		return nil, fmt.Sprintf("命令树投影读不出来（%s）：%v", adeMatrixRel, err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Sprintf("命令树投影读不出来（%s）：%v", adeMatrixRel, err)
	}
	names := map[string]bool{}
	if cases, ok := m["cases"].([]any); ok {
		for _, c := range cases {
			cm, ok := c.(map[string]any)
			if !ok {
				continue
			}
			if s, ok := cm["command"].(string); ok && s != "" {
				f := strings.Fields(s)
				if len(f) > 0 {
					names[f[0]] = true
				}
			}
		}
	}
	return names, ""
}

// ── R5a 的件面判定（代拍口径 · GAP-20261015-05）────────────────────────────────
//
// 件分三面：**源码面**（进 R5a 判决）· **生成面**（只报告）· **删除面**（只报告）。两面的「为什么」：
//   · 生成面 = 门/器/导出器产出的派生件 —— 文档面（`docs/**`·`publish/docs/**`·`项目文档/**`）、
//     契约/兼容/门清单等注册表·矩阵·台账（`core/internal/contract/**` · `cli-matrix.json` 一类
//     `*.json|*.jsonl`）、词表（`styles/config/vocabularies/**`）、探针物料（`scripts/sandbox-probes/**`）、
//     `Cargo.lock`、缺失总账/公开标记。它们由正门命令**生成**、不经 `dev edit` ⇒ 通篇零 `edit` 行；
//     判它必假阳（旧口径 81 条里 153 件次即此类）。
//   · 删除面 = 该件**已不在 `git ls-files`**（退役删除）—— `dev edit` 没有删件模式，删除走 `git rm`
//     ⇒ 结构上也不产生 `edit` 行（旧口径 81 条里 20 件次即此类）。
//   · 非 git 面（仓外夹具 / `git` 不可用）⇒ 退「件不在盘」这一档（仍只报告，✗ 不吃判决）。

// adePageLedger —— R5 两腿的读数面：轮级（`ats`）＋ 件级（`file` → 该件的主仓写轮行时刻）＋ 件面判定用的仓根。
type adePageLedger struct {
	ats  []string            // 轮级面：全部主仓写轮行时刻（升序）
	file map[string][]string // 件级面：`file` → 该件全部主仓写轮行时刻（升序）
	root string              // 件面判定用：`git -C root ls-files`
}

// adeGenFaceRE —— **生成面**判据（见上三条「为什么」）。
var adeGenFaceRE = regexp.MustCompile(
	`^(docs|publish/docs|项目文档)/` +
		`|^core/internal/contract/` +
		`|^styles/config/vocabularies/` +
		`|^scripts/sandbox-probes/` +
		`|(^|/)(cli-matrix|judge-registry|judge-run-ledger|affected-map|lang-freeze-baseline` +
		`|cli-contract-baseline|dangling-refs-registry|doc-cmds-baseline|newcmd-paths-baseline` +
		`|judge-adoption|keep-marks-registry|gate-coverage\.config|gate-check-write-tools|compat)` +
		`\.(json|jsonl)$` +
		`|Cargo\.lock$` +
		`|(^|/)(缺口总账|公开标记)[^/]*\.(md|tsv)$`)

// adeTrackedSet —— `git -C root ls-files`（仓内件全集 · 件面判定真源）。`ok=false` ⇒ 非 git 面 ⇒ 退盘上存在性。
func adeTrackedSet(root string) (map[string]bool, bool) {
	if root == "" {
		return nil, false
	}
	out, err := exec.Command("git", "-C", root, "ls-files", "-z", "--full-name").Output()
	if err != nil {
		return nil, false
	}
	s := map[string]bool{}
	for _, p := range strings.Split(string(out), "\x00") {
		if p != "" {
			s[p] = true
		}
	}
	return s, true
}

// adeFileFace —— 单件分面：`del`（已不在仓）· `gen`（生成面）· `src`（源码面 · 只有它进 R5a 判决）。
func adeFileFace(root, p string, tracked map[string]bool, useGit bool) string {
	if useGit {
		if !tracked[p] {
			return "del"
		}
	} else if _, err := os.Stat(filepath.Join(root, p)); err != nil {
		return "del"
	}
	if adeGenFaceRE.MatchString(p) {
		return "gen"
	}
	return "src"
}

// ── 署名面（★ L1 · GAP-20261015-05 追加 · 用户已拍口径）：本门 want = 「**AI** 绕正门直写」——
// **非 AI（人）的直改不在 want 内** ⇒ 只报告、不进 rc。判据面 = 该轮提交信息里的 `由:` trailer
// （`zerg repo commit` 按 `--by` 落 —— 真源 `family_repo_commit.go` 的 `commitMessageTemplate`）。
//
//	· **git 的 author/committer ✗ 不是判据面**：本仓「AI 改动由人提交」（AI 无提交权），
//	  现读全仓 AI/人提交的 `%an`/`%cn` 一律 `Mr2109` ⇒ 零鉴别力（✗ 别拿它当署名）。
//	· **取值集合**（现读：全部 113 笔带 trailer 的提交）: **AI 面** = 值以 `Hermes`（如
//	  `Hermes (AI)` / `Hermes(…)`）或 `父代理` 开头；**非 AI 面** = 其它显式署名（`Mr2109` / `Mr2109` …）。
//	· **取不到署名**（无 `由:` trailer｜值为未声明占位｜git 取不到）⇒ **fail-closed 判红**（✗ 默认放行）。
var adeAISigRE = regexp.MustCompile(`^(Hermes|父代理|egg)`)

// adeSigUndeclared —— `zerg repo commit` 在 `--only/--removed` 档缺 `--by` 时落的占位（= 未声明）。
const adeSigUndeclared = "（未声明）"

// adeSigIsAI —— 署名值是否 AI 面（前缀精确匹配 · 取值集合见上）。
func adeSigIsAI(v string) bool { return adeAISigRE.MatchString(strings.TrimSpace(v)) }

// adeCommitSig —— 读一笔提交的署名（提交信息里的 `由:` trailer 值）。ok=false ⇒ 取不到。
func adeCommitSig(root, sha string) (string, bool) {
	if root == "" || sha == "" {
		return "", false
	}
	out, err := exec.Command("git", "-C", root, "log", "-1", "--format=%B", sha).Output()
	if err != nil {
		return "", false
	}
	for _, ln := range strings.Split(string(out), "\n") {
		ln = strings.TrimRight(ln, "\r")
		if !strings.HasPrefix(ln, "由:") {
			continue
		}
		v := strings.TrimSpace(ln[len("由:"):])
		if v == "" || v == adeSigUndeclared {
			return "", false // 未声明 ⇒ 取不到署名（fail-closed）
		}
		return v, true
	}
	return "", false
}

// adeRoundSig —— 该轮的署名（逐笔提交取第一枚带 `由:` trailer 的值）。ok=false ⇒ 取不到。
func adeRoundSig(root string, shas []string) (string, bool) {
	for _, s := range shas {
		if v, ok := adeCommitSig(root, s); ok {
			return v, true
		}
	}
	return "", false
}

// adePageFiles —— 页证据里的 `--file` 件清单（`zerg repo commit --file a b c …`）。
//
// 取 `--file` 之后、下一个 `--旗标` 之前的词；词须是路径形（Unicode 字母/数字/`_./-` 且含 `/` 或 `.`）
// —— 中文路径照收（`docs/zh/参考-命令行.md`），中文夹注（含全角括号一类）自然落选。★ ✗ 不做前缀/通配匹配。
var (
	adeFileFlagRE = regexp.MustCompile(`--file\s+`)
	adePathTokRE  = regexp.MustCompile(`^[\p{L}\p{N}_./\\-]+$`)
)

func adePageFiles(ev string) []string {
	var out []string
	seen := map[string]bool{}
	for _, loc := range adeFileFlagRE.FindAllStringIndex(ev, -1) {
		for _, t := range strings.Fields(ev[loc[1]:]) {
			if strings.HasPrefix(t, "--") {
				break
			}
			if !adePathTokRE.MatchString(t) {
				continue
			}
			if !strings.Contains(t, "/") && !strings.Contains(t, ".") {
				continue
			}
			if !seen[t] {
				seen[t] = true
				out = append(out, t)
			}
		}
	}
	return out
}

// adeNames —— 只报告行里的件名枚举（空 ⇒ 空串；多则前 6 件 ＋ 「… 另 N 件」防报话爆行）。
func adeNames(xs []string) string {
	if len(xs) == 0 {
		return ""
	}
	ys := append([]string(nil), xs...)
	sort.Strings(ys)
	const adeNameCap = 6
	if len(ys) > adeNameCap {
		return "（" + strings.Join(ys[:adeNameCap], " · ") + fmt.Sprintf(" · … 另 %d 件）", len(ys)-adeNameCap)
	}
	return "（" + strings.Join(ys, " · ") + "）"
}

// ── py `check_dir(d, tree)` → (rc, lines) ───────────────────────────────────

// adeLoadLedger —— py `load_ledger(path)` 的读面（扩展）：主仓写轮行时刻（轮级 · 升序）
// ＋ 逐件写轮行时刻（件级 · `file` → 该件全部主仓写轮行时刻升序）+ 读不出来的错误串。
//
// 真源 = `edit_audit.jsonl` 里 `event == "edit"` 且**非** `sibling_repo`（= 落主仓相对路径件）的行；
// 只需 `at`（ISO-8601 · 同形 ⇒ 字典序即时序）与 `file`（件级腿用）。`at` 缺失的行跳过。
// err 非空 ⇒ 调用方按 rc=2 落（缺账不当绿）。
func adeLoadLedger(path string) ([]string, map[string][]string, string) {
	fi, err := os.Stat(path)
	if err != nil || fi.IsDir() {
		return nil, nil, fmt.Sprintf("件级账读不出来（%s）：件不存在", path)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Sprintf("件级账读不出来（%s）：%v", path, err)
	}
	var ats []string
	byFile := map[string][]string{}
	for _, ln := range strings.Split(string(b), "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		var o map[string]any
		if json.Unmarshal([]byte(ln), &o) != nil || o == nil {
			continue
		}
		if adeGetStr(o, "event") != "edit" {
			continue
		}
		if _, sib := o["sibling_repo"]; sib && adeTruthy(o, "sibling_repo") {
			continue
		}
		a := strings.TrimSpace(adeGetStr(o, "at"))
		if a != "" {
			ats = append(ats, a)
			if f := strings.TrimSpace(adeGetStr(o, "file")); f != "" {
				byFile[f] = append(byFile[f], a)
			}
		}
	}
	sort.Strings(ats)
	for _, v := range byFile {
		sort.Strings(v)
	}
	return ats, byFile, ""
}

func adeCheckDir(d string, tree map[string]bool, lg *adePageLedger) (int, []string) {
	fi, err := os.Stat(d)
	if err != nil || !fi.IsDir() {
		return 2, []string{fmt.Sprintf("⛔ 不给结论：回执目录不存在（%s）—— 没有回执 ≠ 绿（§20.3 H3）", d)}
	}
	ents := []string{}
	des, err := os.ReadDir(d)
	if err == nil {
		for _, e := range des {
			if strings.HasSuffix(e.Name(), ".json") {
				ents = append(ents, e.Name())
			}
		}
	}
	sort.Strings(ents)
	if len(ents) == 0 {
		return 2, []string{fmt.Sprintf("⛔ 不给结论：回执目录里 0 页（%s）—— 没有回执 ≠ 绿（§20.3 H3）", d)}
	}
	var errs []string
	var report []string
	type adePage struct {
		name string
		ca   string
		ncom int
		ev   string
		shas []string
	}
	var pages []adePage
	for _, name := range ents {
		p := filepath.Join(d, name)
		raw, rerr := os.ReadFile(p)
		var r map[string]any
		if rerr != nil {
			errs = append(errs, fmt.Sprintf("R2 回执不是合法 JSON：%s（%v）", name, rerr))
			continue
		}
		if jerr := json.Unmarshal(raw, &r); jerr != nil {
			// py 的 JSONDecodeError 文案与 Go 不同 ⇒ 见件头未覆盖面①（退码同、正文异）。
			errs = append(errs, fmt.Sprintf("R2 回执不是合法 JSON：%s（%v）", name, jerr))
			continue
		}
		if r == nil {
			errs = append(errs, fmt.Sprintf("R2 回执不是合法 JSON：%s（裸 null）", name))
			continue
		}
		rid := adeGetStr(r, "round_id")
		// R2 五格 + trace_id 形态
		for _, k := range adeREQ {
			if !adeTruthy(r, k) {
				errs = append(errs, fmt.Sprintf("R2 回执缺格 %s：%s", k, name))
			}
		}
		if rid != "" {
			if !adeTraceRE.MatchString(adeGetStr(r, "trace_id")) {
				errs = append(errs, fmt.Sprintf("R2 trace_id 形态不对（要 TR- + 12 位十六进制）：%s ⇒ %s",
					name, adePyReprValue(r, "trace_id")))
			}
		}
		// R3 件名 ↔ round_id
		if rid != "" && name != rid+".json" {
			errs = append(errs, fmt.Sprintf("R3 件名与 round_id 对不上：文件 %s · round_id %s", name, adePyRepr(rid)))
		}
		evtext := adeEvidenceText(r)
		commits := adeListLen(r, "commits")
		// R1 H1：动过仓 ⇒ 必须有一条命令面调用
		calls := adeZergRE.FindAllStringSubmatch(evtext, -1)
		var callNames []string
		for _, m := range calls {
			callNames = append(callNames, m[1])
		}
		if commits > 0 && len(callNames) == 0 && len(adeNoncmdRE.FindAllString(evtext, -1)) == 0 {
			errs = append(errs, fmt.Sprintf("R1 **AI 直改**：%s 说动过仓（commits=%d 条）而证据里**没有任何 `zerg …` 调用** "+
				"⇒ 门红（§20.3 H1：执行只能走命令面）", name, commits))
		}
		// R4 点名的命令面调用必须真在命令树里
		if tree != nil {
			for _, c := range callNames {
				if !tree[c] {
					errs = append(errs, fmt.Sprintf("R4 证据里点名的命令面调用 `zerg %s` 在命令树里**解析不到**（幽灵调用）：%s", c, name))
				}
			}
		}
		pages = append(pages, adePage{name: name, ca: strings.TrimSpace(adeGetStr(r, "created_at")), ncom: commits, ev: evtext, shas: adeListStrings(r, "commits")})
	}
	// ── R5：回执账（轮级）⟷ 件级账（件级）双向对账（GAP-20260927-81 的两半）────────────
	if lg != nil && len(lg.ats) > 0 {
		firstAt := lg.ats[0]
		stampSet := map[string]bool{}
		for _, pg := range pages {
			if pg.ca != "" {
				stampSet[pg.ca] = true
			}
		}
		var stamps []string
		for s := range stampSet {
			stamps = append(stamps, s)
		}
		sort.Strings(stamps)
		tracked, useGit := adeTrackedSet(lg.root)
		// R5a（假 · 轮窗腿 ＋ 件级腿 · 代拍口径 GAP-20261015-05）：说动过仓的页，先在「轮窗」内要件级账有
		// ≥1 主仓写轮行（旧断言原样）；轮窗空 ⇒ 再逐件看 `--file` 清单里的**源码面**件有没有 ≤ 页时刻的
		// `dev edit` 行。生成面/删除面**只报告 · 不进 rc**（两类结构上没有 `dev edit` 行）。
		for _, pg := range pages {
			if pg.ncom <= 0 || pg.ca == "" {
				continue
			}
			if pg.ca < firstAt {
				continue // 该页早于件级账最早一行 ⇒ 件级账判不了它（不上判 · 不许拿缺账当红）
			}
			lo := ""
			for _, k := range stamps {
				if k < pg.ca {
					lo = k
				}
			}
			win := 0
			for _, a := range lg.ats {
				if (lo == "" || a > lo) && a <= pg.ca {
					win++
				}
			}
			loDisp := lo
			if loDisp == "" {
				loDisp = "件级账起点"
			}
			if win > 0 {
				continue // 轮窗腿放行（旧断言原样保留）
			}
			files := adePageFiles(pg.ev)
			if len(files) == 0 {
				// 页证据里没有可解析的 `--file` 清单 ⇒ 件面判不了 ⇒ 按旧口径（轮窗）落红 · 旧断言不放宽。
				errs = append(errs, fmt.Sprintf("R5a **假绿绕行**：%s 说动过仓（commits=%d 条）而件级账在这一轮"+
					"（%s 之后、%s 之前）**没有任何主仓写轮行** ⇒ 不是走命令面写的"+
					"（§20.3 H1：执行只能走命令面）", pg.name, pg.ncom, loDisp, pg.ca))
				continue
			}
			var src, gen, dele []string
			for _, f := range files {
				switch adeFileFace(lg.root, f, tracked, useGit) {
				case "gen":
					gen = append(gen, f)
				case "del":
					dele = append(dele, f)
				default:
					src = append(src, f)
				}
			}
			if len(src) == 0 {
				report = append(report, fmt.Sprintf("R5a·只报告（生成面/删除面 · ✗ 不进 rc）：%s 动过仓（commits=%d 条 · "+
					"轮窗 %s → %s 内零主仓写轮行）· 生成面 %d 件%s · 删除面 %d 件%s —— 两类结构上没有 `dev edit` 行"+
					"（生成面 = 门/器产出的派生件；删除面 = 件已不在 `git ls-files`）",
					pg.name, pg.ncom, loDisp, pg.ca, len(gen), adeNames(gen), len(dele), adeNames(dele)))
				continue
			}
			var miss []string
			for _, f := range src {
				covered := false
				for _, a := range lg.file[f] {
					if a <= pg.ca {
						covered = true
						break
					}
				}
				if !covered {
					miss = append(miss, f)
				}
			}
			if len(miss) > 0 {
				sort.Strings(miss)
				missTxt := strings.Join(miss, " · ")
				// ★ 署名面分流（L1）：want = 「**AI** 绕正门直写」—— 该轮署名**非 AI**（人）⇒ 只报告、不进 rc。
				if sig, ok := adeRoundSig(lg.root, pg.shas); ok && !adeSigIsAI(sig) {
					report = append(report, fmt.Sprintf("R5a·只报告（非 AI 署名 · ✗ 不进 rc）：%s 动过仓（commits=%d 条 · "+
						"源码面件在轮窗（%s 之后、%s 之前）零主仓写轮行、也**没有 ≤ 页时刻的 `dev edit` 行**）：%s —— "+
						"该轮署名为「%s」（**非 AI 面** · 本门 want = 「AI 绕正门直写」，人直改不在 want 内）",
						pg.name, pg.ncom, loDisp, pg.ca, missTxt, sig))
				} else {
					sigTxt := "该轮**取不到署名**（无 `由:` trailer ⇒ fail-closed）"
					if ok && adeSigIsAI(sig) {
						sigTxt = fmt.Sprintf("该轮署名为「%s」（**AI 面**）", sig)
					}
					errs = append(errs, fmt.Sprintf("R5a **假绿绕行**：%s 说动过仓（commits=%d 条）而**源码面**件在轮窗"+
						"（%s 之后、%s 之前）零主仓写轮行、件级账里也**没有 ≤ 页时刻的 `dev edit` 行**："+
						"%s ⇒ 不是走命令面写的（§20.3 H1：执行只能走命令面）；%s",
						pg.name, pg.ncom, loDisp, pg.ca, missTxt, sigTxt))
				}
			}
		}
	}
	if len(errs) > 0 {
		return 1, append(errs, report...)
	}
	// R5b（漏）：件级账里最新主仓写轮晚于最新回执 ⇒ 回执账没覆盖 ⇒ 判不了（不得报绿）。
	if lg != nil && len(lg.ats) > 0 {
		newestAt := lg.ats[len(lg.ats)-1]
		stampSet := map[string]bool{}
		for _, pg := range pages {
			if pg.ca != "" {
				stampSet[pg.ca] = true
			}
		}
		var stamps []string
		for s := range stampSet {
			stamps = append(stamps, s)
		}
		sort.Strings(stamps)
		// ★ F35（2026-10-15 真修 · 拍定 A1 · 最小幅）：回执页**无任何 `created_at`** ⇒ 没有时间基准可比 ⇒
		// 覆盖自证不可用 ⇒ **亦判 rc=2（不给结论）**。旧实现 `len(stamps) > 0 && …` 在该形态下整支跳过 ⇒
		// rc=0 且印出一句**不实**的「R5 轮级账 ⟷ 件级账双向对账 ✓（… ≤ 最新回执）」（根本没有回执时间可比）。
		// ✗ 不得折绿、✗ 不得折 rc=1 —— 与本门「取不到 ⇒ 判不了」同向（fail-closed）。
		if len(stamps) == 0 {
			return 2, append([]string{fmt.Sprintf("⛔ 不给结论（R5b 覆盖自证）：回执 %d 页**无任何 `created_at`** ⇒ "+
				"没有时间基准可比，而件级账里 %d 个主仓写轮无从自证 ⇒ 覆盖自证不可用，本门判不了"+
				"（旧行为在这报绿＝假绿 · F35 拍定 A1）", len(pages), len(lg.ats))}, report...)
		}
		if newestAt > stamps[len(stamps)-1] {
			late := 0
			for _, a := range lg.ats {
				if a > stamps[len(stamps)-1] {
					late++
				}
			}
			// ★ F23（2026-10-15 真修 · 最小幅）：判不了（rc=2）**也要**把已经算出来的 R5a 观测印出来 ——
			// 旧实现把 report 丢在这一支（现读面只剩这一行「不给结论」），谁都判不出「R5a 还剩几条红 /
			// 几条只报告」（收口批实测：真仓现读 1 行 ⟷ 把件级账截到最新回执那一刻之后 55 行 ＋ rc=0）。
			// ✗ 不动判据：rc 仍 2 · R5a/R5b 的判决与红项一字未改，只把已算出的只报告行一并印出。
			return 2, append([]string{fmt.Sprintf("⛔ 不给结论（R5b 覆盖自证）：件级账里最新主仓写轮 %s **晚于**最新回执 %s —— "+
				"有 %d 个主仓写轮没有任何回执覆盖 ⇒ 回执账断供，本门判不了"+
				"（旧行为在这报绿＝假绿 · GAP-20260927-81）", newestAt, stamps[len(stamps)-1], late)}, report...)
		}
	}
	// ★ F41（2026-10-15 · 扩 F35／拍定 A1 到邻接两处 fail-open 面 · 最小幅）：**任一回执页无 `created_at`** ⇒
	// 该页没有时间基准可比 ⇒ R5a 的轮窗腿／件级腿与 R5b 的覆盖自证都**判不了它**。
	// 旧实现只在「**全部**页无戳」（`len(stamps) == 0` · 上面 F35 那一支）时出声；下面两形态都够不着：
	//   ① **混有戳页**（≥1 页带 `created_at` ⇒ `len(stamps) > 0`，F35 支不触发）⇒ 无戳页在 R5a 被
	//      `pg.ca == ""` **整页跳过**；② **件级账空**（`len(lg.ats) == 0` ⇒ 上面两个
	//      `lg != nil && len(lg.ats) > 0` 块整块不进）⇒ 连 F35 支都够不着。⇒ 该无戳页被**静默放过**，
	// 门可报 rc=0「零直改纪律全过」＝假绿。本例**亦判 rc=2（不给结论）且出声**，✗ 不得折绿、
	// ✗ 不得折 rc=1 —— 与 F35 同向（取不到时间基准 ⇒ 判不了 · fail-closed）。
	// ✗ 不动 R5a／R5b 既有判据与报话；✗ 不碰「**空件级账 ＋ 全带戳页** ⇒ rc=0」那一支（自检⑮ 立的成文口径）。
	{
		noStampPages := 0
		for _, pg := range pages {
			if pg.ca == "" {
				noStampPages++
			}
		}
		if noStampPages > 0 {
			return 2, append([]string{fmt.Sprintf("⛔ 不给结论（R5b 覆盖自证）：回执 %d 页里有 %d 页**无 `created_at`** ⇒ "+
				"没有时间基准可比 ⇒ 覆盖自证不可用，本门判不了（旧行为在这报绿＝假绿 · F41 扩 F35 拍定 A1）",
				len(pages), noStampPages)}, report...)
		}
	}
	lines := []string{fmt.Sprintf("回执 %d 页逐页过：H1 零直改 ✓ · H3 五格 ✓ · 件名 ✓ · 命令面调用可解析 ✓", len(ents))}
	lines = append(lines, report...)
	if lg != nil && len(lg.ats) > 0 {
		lines = append(lines, fmt.Sprintf("R5 轮级账 ⟷ 件级账双向对账 ✓（主仓写轮 %d 行 · 最新 %s ≤ 最新回执）",
			len(lg.ats), lg.ats[len(lg.ats)-1]))
	}
	return 0, lines
}

// adeGetStr —— py `str(r.get(k, ""))`（缺格 ⇒ ""；标量逐字复刻；容器见未覆盖面③）。
func adeGetStr(r map[string]any, k string) string {
	v, ok := r[k]
	if !ok {
		return ""
	}
	return adePyStr(v)
}

// adeTruthy —— py `str(r.get(k, "")).strip()` 的真假（缺格 ⇒ "" ⇒ 假）。
func adeTruthy(r map[string]any, k string) bool {
	v, ok := r[k]
	if !ok {
		return false
	}
	return strings.TrimSpace(adePyStr(v)) != ""
}

// adePyStr —— py `str(x)`（串/布尔/数/null 逐字；容器见未覆盖面③）。
func adePyStr(v any) string {
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
		return adePyNum(t)
	case json.Number:
		return t.String()
	}
	return ""
}

func adePyNum(f float64) string {
	if math.IsInf(f, 0) || math.IsNaN(f) {
		return "nan"
	}
	if f == math.Trunc(f) && math.Abs(f) < 1e15 {
		return strconv.FormatInt(int64(f), 10)
	}
	return strconv.FormatFloat(f, 'g', -1, 64)
}

// adePyReprValue —— py `%r` 于 `r.get(k)`（缺格 ⇒ 空串的 repr，即两个单引号）。
func adePyReprValue(r map[string]any, k string) string {
	v, ok := r[k]
	if !ok {
		return "''"
	}
	if v == nil {
		return "None"
	}
	if s, ok := v.(string); ok {
		return adePyRepr(s)
	}
	return adePyStr(v)
}

// adePyRepr —— py `repr(str)`（常用档：无引号冲突走单引号；含 `'` 不含 `"` 走双引号）。
func adePyRepr(s string) string {
	hasS := strings.ContainsRune(s, '\'')
	hasD := strings.ContainsRune(s, '"')
	if hasS && !hasD {
		return "\"" + s + "\""
	}
	if hasS && hasD {
		return "\"" + strings.ReplaceAll(s, "\"", "\\\"") + "\""
	}
	return "'" + s + "'"
}

// adeEvidenceText —— py：`ev = r.get("evidence") or []`；串 ⇒ 包一层；`" · ".join(str(x) …)`。
func adeEvidenceText(r map[string]any) string {
	v, ok := r["evidence"]
	if !ok || v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	if arr, ok := v.([]any); ok {
		parts := make([]string, len(arr))
		for i, x := range arr {
			parts[i] = adePyStr(x)
		}
		return strings.Join(parts, " · ")
	}
	return "" // 容器/怪异形态见未覆盖面③
}

// adeListLen —— py `len(commits)`，其中 `commits = r.get("commits") or []`，串 ⇒ 包一层。
func adeListLen(r map[string]any, k string) int {
	v, ok := r[k]
	if !ok || v == nil {
		return 0
	}
	switch t := v.(type) {
	case string:
		if t == "" {
			return 0
		}
		return 1
	case []any:
		return len(t)
	}
	return 0
}

// adeListStrings —— `commits` 里的 sha 串（`adeListLen` 的字符串投影 · 署名面逐笔取 trailer 用）。
func adeListStrings(r map[string]any, k string) []string {
	v, ok := r[k]
	if !ok || v == nil {
		return nil
	}
	switch t := v.(type) {
	case string:
		if t == "" {
			return nil
		}
		return []string{t}
	case []any:
		var out []string
		for _, x := range t {
			if s, ok := x.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// ── py `refuse_json_without_machine_face(argv)` ─────────────────────────────

func adeRefuseJSON(stderr io.Writer, argv []string) (int, bool) {
	var hit []string
	for _, a := range argv {
		if a == "--json" || strings.HasPrefix(a, "--json=") {
			hit = append(hit, a)
		}
	}
	if len(hit) == 0 {
		return 0, false
	}
	parts := make([]string, len(hit))
	for i, h := range hit {
		parts[i] = "`" + h + "`"
	}
	fmt.Fprintf(stderr, "⛔ 本门无机器读面：%s 在本门**未实现**（缺口 Q-176）\n", strings.Join(parts, " · "))
	if len(adeJSONFields) == 0 {
		fmt.Fprintln(stderr, "   字段表：无（本门只出人读面）")
	} else {
		fs := make([]string, len(adeJSONFields))
		for i, f := range adeJSONFields {
			fs[i] = "`" + f + "`"
		}
		fmt.Fprintf(stderr, "   字段表：%s\n", strings.Join(fs, " · "))
	}
	fmt.Fprintln(stderr, "⇒ 用法错（rc=2）：**机器面缺 ≠ 零命中** —— 两态不许同形")
	return 2, true
}

// ── py `self_test(root)`：三档之外的全档（1 正控 + 5 负控 + 2 空转）────────────

func adeSelfTest(stdout io.Writer) int {
	var fails []string
	tmp, err := os.MkdirTemp("", "zerg-gate13-")
	if err != nil {
		tmp = os.TempDir()
	}
	defer os.RemoveAll(tmp)

	// py `_mk`：把回执对象写成 `<dir>/<name>`（ensure_ascii=False ⇒ 中文原样）。
	mk := func(d, name string, obj map[string]any) {
		b, _ := json.Marshal(obj)
		_ = os.WriteFile(filepath.Join(d, name), b, 0o644)
	}
	// py `_good`：合规回执模板。
	good := func(rid string, kw map[string]any) map[string]any {
		r := map[string]any{
			"round_id":   rid,
			"trace_id":   "TR-0123456789ab",
			"what":       "做了事",
			"next":       "下一步",
			"blockers":   []any{},
			"evidence":   []any{"zerg gate run --fast"},
			"commits":    []any{"deadbee"},
			"created_at": "2026-09-20T00:00:00+08:00",
		}
		for k, v := range kw {
			r[k] = v
		}
		return r
	}
	mkdir := func(sub string) string {
		p := filepath.Join(tmp, sub)
		_ = os.MkdirAll(p, 0o755)
		return p
	}

	// ① 空转：目录不存在 ⇒ 2
	if rc, _ := adeCheckDir(filepath.Join(tmp, "nope"), nil, nil); rc != 2 {
		fails = append(fails, fmt.Sprintf("自检① 目录不存在应 rc=2，实测 %d", rc))
	}
	// ② 空转：目录在但 0 页 ⇒ 2
	if rc, _ := adeCheckDir(mkdir("empty"), nil, nil); rc != 2 {
		fails = append(fails, fmt.Sprintf("自检② 0 页应 rc=2，实测 %d", rc))
	}
	// ③ 正控：合规 ⇒ 0
	okDir := mkdir("ok")
	mk(okDir, "R-20260920T000001.json", good("R-20260920T000001", nil))
	if rc, _ := adeCheckDir(okDir, map[string]bool{"gate": true}, nil); rc != 0 {
		fails = append(fails, fmt.Sprintf("自检③ 合规回执应 rc=0，实测 %d", rc))
	}
	// ④ 负控 R1：动过仓但证据里没有命令面调用 ⇒ 1
	n1 := mkdir("n1")
	mk(n1, "R-20260920T000002.json", good("R-20260920T000002", map[string]any{"evidence": []any{"vim core/x.go"}}))
	if rc, _ := adeCheckDir(n1, map[string]bool{"gate": true}, nil); rc != 1 {
		fails = append(fails, fmt.Sprintf("自检④ R1 应 rc=1（AI 直改），实测 %d", rc))
	}
	// ⑤ 负控 R1 不误判：没动仓（commits 空）⇒ 0
	n2 := mkdir("n2")
	mk(n2, "R-20260920T000003.json", good("R-20260920T000003", map[string]any{"commits": []any{}, "evidence": []any{"只读 grep"}}))
	if rc, _ := adeCheckDir(n2, map[string]bool{"gate": true}, nil); rc != 0 {
		fails = append(fails, fmt.Sprintf("自检⑤ 没动仓应 rc=0（不误判），实测 %d", rc))
	}
	// ⑥ 负控 R2：trace_id 形态坏 ⇒ 1
	n3 := mkdir("n3")
	mk(n3, "R-20260920T000004.json", good("R-20260920T000004", map[string]any{"trace_id": "trace-bad"}))
	if rc, _ := adeCheckDir(n3, map[string]bool{"gate": true}, nil); rc != 1 {
		fails = append(fails, fmt.Sprintf("自检⑥ R2 应 rc=1，实测 %d", rc))
	}
	// ⑦ 负控 R3：件名 ≠ round_id ⇒ 1
	n4 := mkdir("n4")
	mk(n4, "R-错的.json", good("R-20260920T000005", nil))
	if rc, _ := adeCheckDir(n4, map[string]bool{"gate": true}, nil); rc != 1 {
		fails = append(fails, fmt.Sprintf("自检⑦ R3 应 rc=1，实测 %d", rc))
	}
	// ⑧ 负控 R4：点名的命令面调用在命令树里解析不到 ⇒ 1
	n5 := mkdir("n5")
	mk(n5, "R-20260920T000006.json", good("R-20260920T000006", map[string]any{"evidence": []any{"zerg 绝无此命令"}}))
	if rc, _ := adeCheckDir(n5, map[string]bool{"gate": true}, nil); rc != 1 {
		fails = append(fails, fmt.Sprintf("自检⑧ R4 应 rc=1（幽灵调用），实测 %d", rc))
	}
	// ⑨ 负控 R1 不误判：`zerg --help`（非子命令调用）⇒ 0（GAP-20260927-59）
	n6 := mkdir("n6")
	mk(n6, "R-20260920T000007.json", good("R-20260920T000007", map[string]any{"evidence": []any{"zerg --help"}}))
	if rc, _ := adeCheckDir(n6, map[string]bool{"gate": true}, nil); rc != 0 {
		fails = append(fails, fmt.Sprintf("自检⑨ zerg --help 应 rc=0（非子命令调用 · 不误判），实测 %d", rc))
	}
	// ⑩ 负控 R1 不误判：`zerg 2.5.12+dirty`（版本号形态）⇒ 0（GAP-20260927-59）
	n7 := mkdir("n7")
	mk(n7, "R-20260920T000008.json", good("R-20260920T000008", map[string]any{"evidence": []any{"zerg 2.5.12+dirty"}}))
	if rc, _ := adeCheckDir(n7, map[string]bool{"gate": true}, nil); rc != 0 {
		fails = append(fails, fmt.Sprintf("自检⑩ zerg 版本号 应 rc=0（版本号形态 · 不误判），实测 %d", rc))
	}
	// 件级账夹具：`ats` 各写一条 `event=edit` 主仓写轮行；`sib` 非空写一条 sibling 行（不上判）。
	led := func(path string, ats []string, sib string) string {
		var b strings.Builder
		for _, a := range ats {
			b.WriteString(fmt.Sprintf("{\"at\":%q,\"event\":\"edit\",\"file\":\"core/x.go\",\"mode\":\"replace\"}\n", a))
		}
		if sib != "" {
			b.WriteString(fmt.Sprintf("{\"at\":%q,\"event\":\"edit\",\"file\":\"docs/a.md\",\"sibling_repo\":\"Zerg-内部文档\"}\n", sib))
		}
		_ = os.WriteFile(path, []byte(b.String()), 0o644)
		return path
	}
	ledAt := func(path string) *adePageLedger {
		ats, byFile, err := adeLoadLedger(path)
		if err != "" {
			fails = append(fails, "自检 件级账夹具读不出来："+err)
		}
		return &adePageLedger{ats: ats, file: byFile, root: tmp}
	}
	// ⑪ 负控 R5a（假绿绕行）：手改后补一条含**任意** zerg 调用的回执 ⇒ 1（GAP-20260927-81）
	n8 := mkdir("n8")
	mk(n8, "R-20260920T100000.json", good("R-20260920T100000", map[string]any{
		"created_at": "2026-09-20T10:00:00+08:00", "evidence": []any{"zerg dev edit --proposal DEV-1"}}))
	mk(n8, "R-20260920T120000.json", good("R-20260920T120000", map[string]any{
		"created_at": "2026-09-20T12:00:00+08:00", "evidence": []any{"zerg gate ls"}}))
	led8 := led(filepath.Join(tmp, "n8-ledger.jsonl"), []string{"2026-09-20T09:00:00+08:00"}, "")
	if rc, _ := adeCheckDir(n8, map[string]bool{"gate": true, "dev": true}, ledAt(led8)); rc != 1 {
		fails = append(fails, fmt.Sprintf("自检⑪ R5a 应 rc=1（手改补回执绕绿 · 假绿），实测 %d", rc))
	}
	// ⑫ 负控 R5b（覆盖自证）：件级账最新主仓写轮晚于最新回执 ⇒ 2（不是绿）
	n9 := mkdir("n9")
	mk(n9, "R-20260920T100100.json", good("R-20260920T100100", map[string]any{
		"created_at": "2026-09-20T10:00:00+08:00", "evidence": []any{"zerg dev edit --proposal DEV-1"}}))
	led9 := led(filepath.Join(tmp, "n9-ledger.jsonl"),
		[]string{"2026-09-20T09:00:00+08:00", "2026-09-20T12:00:00+08:00"}, "")
	if rc, _ := adeCheckDir(n9, map[string]bool{"gate": true, "dev": true}, ledAt(led9)); rc != 2 {
		fails = append(fails, fmt.Sprintf("自检⑫ R5b 应 rc=2（覆盖自证 · 判不了），实测 %d", rc))
	}
	// ⑬ 正控 R5：写轮在轮窗内、且没有更晚的写轮 ⇒ 0
	n10 := mkdir("n10")
	mk(n10, "R-20260920T100200.json", good("R-20260920T100200", map[string]any{
		"created_at": "2026-09-20T10:00:00+08:00", "evidence": []any{"zerg dev edit --proposal DEV-1"}}))
	led10 := led(filepath.Join(tmp, "n10-ledger.jsonl"),
		[]string{"2026-09-20T09:00:00+08:00"}, "2026-09-21T00:00:00+08:00")
	if rc, _ := adeCheckDir(n10, map[string]bool{"gate": true, "dev": true}, ledAt(led10)); rc != 0 {
		fails = append(fails, fmt.Sprintf("自检⑬ R5 正控应 rc=0（写轮在轮窗内 · sibling 行不上判），实测 %d", rc))
	}
	// ⑭ 负控 R5：回执早于件级账最早一行 ⇒ R5a 不上判、但 R5b 仍应 rc=2
	n11 := mkdir("n11")
	mk(n11, "R-20260919T235900.json", good("R-20260919T235900", map[string]any{
		"created_at": "2026-09-19T23:59:00+08:00"}))
	led11 := led(filepath.Join(tmp, "n11-ledger.jsonl"), []string{"2026-09-20T09:00:00+08:00"}, "")
	if rc, _ := adeCheckDir(n11, map[string]bool{"gate": true}, ledAt(led11)); rc != 2 {
		fails = append(fails, fmt.Sprintf("自检⑭ 回执早于件级账最早一行 ⇒ R5a 不上判、R5b 仍应 rc=2，实测 %d", rc))
	}
	// ⑮ 正控 R5：空件级账 + 页早于其起点 ⇒ 不上判 ⇒ 0
	n12 := mkdir("n12")
	mk(n12, "R-20260919T230000.json", good("R-20260919T230000", map[string]any{
		"created_at": "2026-09-19T23:00:00+08:00"}))
	led12 := led(filepath.Join(tmp, "n12-ledger.jsonl"), nil, "")
	if rc, _ := adeCheckDir(n12, map[string]bool{"gate": true}, ledAt(led12)); rc != 0 {
		fails = append(fails, fmt.Sprintf("自检⑮ 空件级账 + 页早于其起点 ⇒ 应 rc=0（不上判），实测 %d", rc))
	}

	// ⑯ 负控 R5b（覆盖自证 · **无戳页** · F35 拍定 A1）：回执页**无任何 `created_at`** ⇒ 没有时间基准可比
	//     ⇒ **亦判 rc=2（不给结论）**，✗ 不得静默跳过该支报绿（旧行为：rc=0 且印不实的「双向对账 ✓」）。
	n13 := mkdir("n13")
	mk(n13, "R-20260920T000009.json", good("R-20260920T000009", map[string]any{"created_at": ""}))
	led13 := led(filepath.Join(tmp, "n13-ledger.jsonl"), []string{"2026-09-21T09:00:00+08:00"}, "")
	if rc, _ := adeCheckDir(n13, map[string]bool{"gate": true}, ledAt(led13)); rc != 2 {
		fails = append(fails, fmt.Sprintf("自检⑯ 无戳页应 rc=2（覆盖自证 · 无时间基准 · 判不了），实测 %d", rc))
	}
	// ⑰ 负控 R5a／覆盖自证（F41 · 邻接 fail-open 面①）：**混有无戳页**（≥1 页带 created_at、另有页无）
	//     ⇒ 无戳页在 R5a 被 `pg.ca == ""` **整页跳过** ⇒ 旧行为 rc=0＝假绿；本例**亦判 rc=2（判不了）且出声**。
	n14 := mkdir("n14")
	mk(n14, "R-20260920T100000.json", good("R-20260920T100000", map[string]any{
		"created_at": "2026-09-20T10:00:00+08:00", "evidence": []any{"zerg dev edit --proposal DEV-1"}}))
	mk(n14, "R-20260920T110000.json", good("R-20260920T110000", map[string]any{"created_at": ""}))
	led14 := led(filepath.Join(tmp, "n14-ledger.jsonl"), []string{"2026-09-20T09:00:00+08:00"}, "")
	if rc, _ := adeCheckDir(n14, map[string]bool{"gate": true, "dev": true}, ledAt(led14)); rc != 2 {
		fails = append(fails, fmt.Sprintf("自检⑰ 混有无戳页应 rc=2（判不了 · R5a 整页跳过＝假绿），实测 %d", rc))
	}
	// ⑱ 负控 覆盖自证（F41 · 邻接 fail-open 面②）：**件级账空 ＋ 无戳页** ⇒ R5a／R5b 两块整块不进 ⇒ 旧行为 rc=0；
	//     本例**亦判 rc=2（判不了）且出声**（✗ 不碰「空账 ＋ 全带戳页 ⇒ rc=0」那一支 · 自检⑮）。
	n15 := mkdir("n15")
	mk(n15, "R-20260920T100000.json", good("R-20260920T100000", map[string]any{"created_at": ""}))
	led15 := led(filepath.Join(tmp, "n15-ledger.jsonl"), nil, "")
	if rc, _ := adeCheckDir(n15, map[string]bool{"gate": true}, ledAt(led15)); rc != 2 {
		fails = append(fails, fmt.Sprintf("自检⑱ 空件级账 + 无戳页应 rc=2（判不了），实测 %d", rc))
	}
	if len(fails) > 0 {
		fmt.Fprintf(stdout, "门⑬ 自检失败 %d 条：\n", len(fails))
		for _, f := range fails {
			fmt.Fprintln(stdout, "  ✗ "+f)
		}
		return 1
	}
	fmt.Fprintln(stdout, "门⑬ 自检全过：3 正控 + 13 负控 + 2 空转（成对负控）")
	return 0
}

// ── 入口（= py `main()`）────────────────────────────────────────────────────

func aidirecteditsGateMain(stdout, stderr io.Writer, argv []string) int {
	// ① Q-176：给了 `--json` 而本门**无机器读面** ⇒ 逐字说明后 rc=2（先于一切，含 argparse/--self-test）。
	if rc, ok := adeRefuseJSON(stderr, argv); ok {
		return rc
	}
	// ② argparse：`-h/--help` 在任何位置出现即出帮助并 rc=0（含「未知旗标在前」也照样先出帮助 —— 现读复刻）。
	for _, a := range argv {
		if a == "-h" || a == "--help" {
			fmt.Fprint(stdout, adeHelp)
			return 0
		}
	}
	// ③ argparse：选项解析（`--root` / `--dir` / `--self-test`），未识别项收集后报错（rc=2）。
	root := adeDefaultRoot()
	dirArg := ""
	ledgerArg := ""
	selfTest := false
	var unrecognized []string
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "--root":
			if i+1 >= len(argv) {
				fmt.Fprint(stderr, adeUsageLine)
				fmt.Fprintf(stderr, "%s: error: argument --root: expected one argument\n", adeProg)
				return 2
			}
			i++
			root = argv[i]
		case strings.HasPrefix(a, "--root="):
			root = a[len("--root="):]
		case a == "--dir":
			if i+1 >= len(argv) {
				fmt.Fprint(stderr, adeUsageLine)
				fmt.Fprintf(stderr, "%s: error: argument --dir: expected one argument\n", adeProg)
				return 2
			}
			i++
			dirArg = argv[i]
		case strings.HasPrefix(a, "--dir="):
			dirArg = a[len("--dir="):]
		case a == "--ledger":
			if i+1 >= len(argv) {
				fmt.Fprint(stderr, adeUsageLine)
				fmt.Fprintf(stderr, "%s: error: argument --ledger: expected one argument\n", adeProg)
				return 2
			}
			i++
			ledgerArg = argv[i]
		case strings.HasPrefix(a, "--ledger="):
			ledgerArg = a[len("--ledger="):]
		case a == "--self-test":
			selfTest = true
		default:
			unrecognized = append(unrecognized, a)
		}
	}
	if len(unrecognized) > 0 {
		fmt.Fprint(stderr, adeUsageLine)
		fmt.Fprintf(stderr, "%s: error: unrecognized arguments: %s\n", adeProg, strings.Join(unrecognized, " "))
		return 2
	}
	// ④ `--self-test` 存在即走自检（py：`if a.self_test: return self_test(a.root)`）。
	if selfTest {
		return adeSelfTest(stdout)
	}
	d := dirArg
	if d == "" {
		d = adeDefaultDir()
	}
	tree, terr := adeLoadCommandTree(root)
	if terr != "" {
		fmt.Fprintf(stdout, "⛔ 不给结论：%s\n", terr)
		return 2
	}
	lpath := ledgerArg
	if lpath == "" {
		lpath = adeDefaultLedger()
	}
	ats, byFile, lerr := adeLoadLedger(lpath)
	if lerr != "" {
		fmt.Fprintf(stdout, "⛔ 不给结论：%s\n", lerr)
		return 2
	}
	rc, lines := adeCheckDir(d, tree, &adePageLedger{ats: ats, file: byFile, root: root})
	fmt.Fprintf(stdout, "门⑬ AI 直改纪律（§20.3 H1 · 回执 %s）· 仓根 %s\n", d, root)
	// ★ 缺口 `GAP-20260927-71`：件级账（`edit_audit.jsonl`）是**活文件** —— 取证期间会被别的进程追加。
	//   可变面必须**自报读数时点与件长**（本门取 want 的第二形态：不落冻结快照，改在输出里自报），
	//   否则「门绿」只是那一刻的快照、可在无人改判据时自行变绿/变红。
	if b, rerr := os.ReadFile(lpath); rerr == nil {
		mt := ""
		if fi, serr := os.Stat(lpath); serr == nil {
			mt = fi.ModTime().Format("2006-01-02T15:04:05Z07:00")
		}
		sum := sha256.Sum256(b)
		fmt.Fprintf(stdout, "  [自报·可变面 GAP-20260927-71] 件级账 %s · 读数时点 %s · 件长 %d B · sha16 %s\n",
			lpath, mt, len(b), hex.EncodeToString(sum[:])[:16])
	}
	for _, ln := range lines {
		fmt.Fprintf(stdout, "  %s\n", ln)
	}
	switch rc {
	case 0:
		fmt.Fprint(stdout, "⇒ 零直改纪律全过（rc=0）\n")
	case 1:
		fmt.Fprint(stdout, "⇒ 有红项（rc=1）：上面逐条点名\n")
	default:
		fmt.Fprint(stdout, "⇒ 不给结论（rc=2）\n")
	}
	return rc
}

// adeRunForTest —— 进程内跑入口取 (rc, stdout+stderr)（`--self-test` 复刻 py 的 subprocess 汇总口径）。
// 仅供同批测试件复用；py 侧合流 = `(p.stdout or "") + (p.stderr or "")`。
func adeRunForTest(argv []string) (int, string) {
	var o, e bytes.Buffer
	rc := aidirecteditsGateMain(&o, &e, argv)
	return rc, o.String() + e.String()
}
