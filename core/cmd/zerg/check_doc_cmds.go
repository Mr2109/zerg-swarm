// check_doc_cmds.go —— 判据件 `scripts/gates/check-doc-cmds.py`（723 行）的 **Go 真搬**
// （v2.5.14 · M2 · **新口径靶：文仓/外部树依赖类** · 入口 `checkDocCmdsGateMain`）。
//
// 本件 = 门⑫「文档-命令一致性」（§九 M12 + §6.4 D3 · 开工单 T-51）的真搬：把**两棵语料**
// （正式面 `docs/**` = 阻断档；开发文档面 `Zerg-内部文档/项目文档/v2.5.13/` = 只报告档 + baseline 棘轮）
// 里手写的 `zerg …` 示例，对照**命令树源码**（`core/cmd/zerg/main.go` 的 `path: []string{…}` 字面量）
// 逐段收窄（最长前缀）判它拼不拼得到一条命令。验收判据 = 同 argv 跑两态（py 原件 ⟷ 本入口），
// stdout / stderr / 退码 **三面逐字相同**（除下列**显式声明的**新口径差异面）。
//
// 本件与新口径（外部树走显式入参旋钮）—— 逐条对应：
//
//	① 旋钮名**照 py 已有同名 env**：py 本就用 `ZERG_DEVDOCS_ROOT` / `ZERG_DOCS_ALT` / `ZERG_DOCS`
//	   （外加默认同级根 `<仓父>/Zerg-内部文档`）⇒ 本件**同名同序照搬、不另起名**
//	   （✗ 不造第三个互不认识的名字 —— 那正是 `GAP-20261001-01` 那个口子的来源）。py 对**仓根**
//	   本身无旗标、无 env（起于 `__file__`）⇒ 见 (a) 的默认根差异面。
//	② 缺省值 = py 默认推导**逐字同构**：`devdocs_root = env ZERG_DEVDOCS_ROOT or
//	   pyJoin(sibling_docs_base(root), "项目文档", "v2.5.13")`；`sibling_docs_base(root) =
//	   abspath(env ZERG_DOCS_ALT or env ZERG_DOCS or pyJoin(pyDir(abspath(root)), "Zerg-内部文档"))`。
//	   两串均**不含 `..` 段** ⇒ `filepath.Join` 的 normalizer 无机会吃掉东西，但本件仍走自实现
//	   `dcmdJoin`（posixpath.join 语义）以逐字对齐 py 的打印串。
//	③ ★ 退 2 边界：py 的**四条**语料缺件/空转分支（formal docs/ 不在 · formal 0 篇 · devdocs 0 篇 ·
//	   devdocs 根不在盘）**逐字照搬**退 2（这四条都是「这一格取不到 ⇒ 不给结论」，不是「任一外部树
//	   取不到就退 2」）。本件**未**把「任一取不到」扩成红：真仓两棵语料都在盘（现读：formal 0 命中 ·
//	   devdocs 454 命中），故本件与 py 在真仓上读数逐字同。✗ 不擅自加严、✗ 不静默绿。
//	④ 取不到一律**不给结论**（rc=2），✗ 不读成「0 件＝干净」；本件把 py 那四条 blocked 分支与
//	   「命令树读不出来 / baseline 解不动」两份 blocked 一并逐字保留。
//
// 本单边界（照实）：本件**只落码、不接线** ——
//
//	· **不**在 `core/cmd/zerg/main.go` 里注册命令（✗ 不进命令树 · ✗ 不改 `add_step`）；
//	· **不**删 `scripts/gates/check-doc-cmds.py` 原件（退场规则：**对拍通过后才**退场）；
//	· 入口 `checkDocCmdsGateMain` 目前**无调用点**（M2 的刻意状态）。
//
// ★ 两条标准声明行：
//
//	(a) **默认根定位差异**：py `HERE = dirname(abspath(__file__))` ·
//	    `DEFAULT_REPO = dirname(dirname(HERE))` ⇒ 默认根起于**脚本所在目录**（与 cwd 无关）；
//	    本件**无 `__file__` 语境** ⇒ 从 **cwd** 上溯 ≤8 层找含 `core/cmd/zerg/main.go` 的目录、
//	    找不到回落 `abspath(cwd)`。**旋钮缺省 vs py 默认推导逐字对比**：
//	      py  `root = os.path.abspath(DEFAULT_REPO)`（脚本位置上溯两层）
//	      Go  `dcmdDefaultRepo()`（cwd 上溯找 `core/cmd/zerg/main.go`；回落 abspath(cwd)）
//	      py  `sibling_docs_base(root) = abspath($ZERG_DOCS_ALT or $ZERG_DOCS or join(dirname(abspath(root)), "Zerg-内部文档"))`
//	      Go  `dcmdSiblingDocsBase(root)`：同名同序逐字（空串 env 视同未设，同 py 的 `or` 短路）
//	      py  `docs_root = $ZERG_DEVDOCS_ROOT or join(sibling_docs_base(root), "项目文档", "v2.5.13")`
//	      Go  `dcmdEnvOr("ZERG_DEVDOCS_ROOT", dcmdJoin(dcmdSiblingDocsBase(root), "项目文档", "v2.5.13"))`
//	    「脚本位置 == cwd 上溯根」时两者读数逐字相同（对拍甲/乙两环境已证）；否则**路径推导面不同**
//	    ⇒ 不属逐字对齐承诺。
//	(b) **自指检查（先真判 · 四形态逐条）**：
//	    ① **本件在自己扫描面内吗 ⇒ 否**。本门扫面 = 两棵语料树里的 **`.md` 件**，且只抽 `zerg …`
//	      形态（`collect_md` 只收 `fn.endswith(".md")`）。本件是 `.go`、落 `core/cmd/zerg/`。
//	      **量化依据**：`collect_md` 内 `endswith(".md")` 过滤 1 处；本件零 `.md` 路径字面量。
//	    ② **本件落在任何他门扫描面内吗 ⇒ 是**（`check-hardcoded-private-paths.py` 的 SCAN_DIRS 含
//	      `core/`；`check-error-kinds` 的 R3 扫 `core/cmd/zerg/*.go`）⇒ 按纪律处置：
//	      · 正文**零私有绝对路径字面量**（盘根绝对路径、家目录前缀一概不写：真仓根/外部树一律
//	        运行时取；需要举例时拆开拼串）；
//	      · **不写** `check-error-kinds` 的被扫错误构造调用原串；
//	      · **不写**反引号包裹的门扫描标记原串；
//	      · self-test 夹具里的魔法串（`doc-cmds-selftest-` 一族）只在**运行时建在 TMPDIR 下**的
//	        夹具件正文里，✗ 不落本件正文的连续被扫形态。
//	    ③ **证明处理有效**：落仓前后跑上述他门同一命令，命中数与输出 sha16 **逐字不变**（见交付读数）。
//	    ④ 本件**不接线** ⇒ 不进任何命令树/用法面 ⇒ 无「他门用法面」新增字面量。
//
// py 原件真口子（★ 不自行修 · 逐字保留 · **交回入账**）：
//
//	① `--self-test` 用 `tempfile.mkdtemp` 且**从不回收**（每跑一次自检，系统临时目录多一个
//	   `doc-cmds-selftest-*` 残留）—— 本件 Go 侧用 `defer os.RemoveAll` 回收（**输出不可见差异**：
//	   py 只打 `  <名> ⇒ rc=N` 与末行，均不含夹具路径 ⇒ 两态 stdout 逐字同）。
//	② `judge_md()` 声明的 `n_block` 计数**从不自增**（`n = n_block` 是死赋值）⇒ 该函数即使被调用
//	   也永远报 0 阻断；本件**不搬此死函数**（py 内无调用点，行为无影响，逐字对齐不受损）。
//	③ `judge()` 对**读不了的 `.md`** 只附一行详列、**不计命中也不阻断** ⇒ 一件读不了 = 静默少判
//	   （非 fail-closed）。**本件 Go 侧已修（B 类真口子 · 与 py 有意分叉）**：读失败 ⇒ 登记（相对路径 +
//	   err 正文）+ 入口 `checkDocCmdsGateMain` 补判红（stderr BLOCKED 头 + 逐件点名）⇒ rc=2（不再静默少判）。
//
// ★ 2026-10-04 同步（候选 · GAP-20261001-32③）：py 候件给 `judge()` / `judge_md()` 各增 `n_unread` 计数并
//
//	作第 4 返回值，两调用点非零即把「读不了 N 篇（读不了 ≠ 零命中）」并入 blocked ⇒ 进 rc 通道（rc=2）。
//	**本件 Go 侧已逐处同步**：`dcmdJudge` 加第 4 返回值 `nUnread`；两调用点并入同一段 blocked 文案。
//	既有 B 类登记面（stderr BLOCKED 头 + 逐件点名）**保留**（Go-only 分叉）；入口改为**先印 stdout 人读面、
//	再落 stderr BLOCKED** —— 否则那条新增的 blocked 文案会被 B 类早退整个吞掉（= 同步落空）。
//	`judge_md` 本件不搬（py 内死函数 · 无调用点 ⇒ 行为无影响）。
//
// ★ 2026-10-04 同步②（候选 · py⟷Go 全等追平）：py `bde233fb` 的**未建提案书写体例**（识别跳过**第三类**）逐处补入 ——
//
//	① 新增 `dcmdProposalMark` 闭集（15 判语 · 紧贴 `zerg` 之前才认 · 逐字复刻 py PROPOSAL_MARK）；
//	② `dcmdExtract` 行内分支改走 `FindAllStringSubmatchIndex`，命中判语 ⇒ 记 `proposal` 计数 + 丢该段；
//	③ 「识别跳过」行补第三类计数（原文串逐字：`三类都是`，✗ 不是旧 `此二类`）⇒ 每格 stdout 由差 30
//	   字符转为逐字同；④ 自检成对负控补 ⑳㉑㉒ 三组（自检网格 23 组 ⇒ 26 组，与 py 逐格对齐）。
//	⑤ 另补 py `devdocs_root_is_repo_root`（`GAP-20260927-83` · 原 Go 未搬）：devdocs 根**形态误用**
//	   同「根不在盘」档 rc=2。
//
// 未覆盖面（✗ 不声称逐字对齐）：
//
//	① argparse **长选项前缀缩写**（如 `--selft` ⇒ `--self-test`）本件不实现。
//	② `--scope` 后紧跟另一选项（如 `--scope --help`）的 argparse 特殊处置本件不复刻（本件把
//	   `--help` 当 scope 取值 ⇒ invalid choice）；`--scope` 缺值文案亦按本件自拟。
//	③ **默认根定位**：见 (a) —— 「脚本位置 ≠ cwd 上溯根」时不属逐字对齐承诺。
//	④ ★ **正则语义差**：py 的 `\s` / `\S` 是 **Unicode 语义**，Go RE2 默认 **ASCII**。涉及面 =
//	   `ZERG_INLINE` 的 `\s+` / `[^\s`]`、`IDENTITY_SEG` 的 `\s+`、围栏分割的 `\s*` 与 `(?<!\S)`。
//	   实测差：`zerg` 与命令词之间若夹 **U+00A0**（不换行空格）等 Unicode 空白 ⇒ py 认（`\s` 含
//	   U+00A0）、本件 RE2 **不认** ⇒ 该行**少抽一条**。真仓现读两棵语料该形态**零命中**（对拍逐字同）；
//	   凡会碰到 `\s`/`\S` 的语料本件**如实标为未覆盖**（✗ 不假装逐字对齐）。`\d`/`\w` 本件不涉
//	   （py 代码里用的是显式 `[0-9]` / `[0-9a-fA-F]`）。
//	⑤ 读文件失败正文：py 打 `读不了 <rel>：<OSError 串>` ⟷ 本件打 Go `err` 串（正文自定、不属对齐）。
//	⑥ `load_baseline` 遇 JSON 顶层非 dict（py 抛 `TypeError` 回溯 ⇒ 进程 rc=1）；本件归「解不动」
//	   ⇒ rc=2（fail-closed，有意差异）。
//
// `--evidence GAP-20261001-13`（在册 M2 承接 · 先已回读 · 语义对口：want_family=gate / want_action=check）。
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

const dcmdMainRel = "core/cmd/zerg/main.go"
const dcmdBaselineRel = "scripts/gates/doc-cmds-baseline.json"

const dcmdRCOK = 0
const dcmdRCFail = 1
const dcmdRCBlocked = 2

const dcmdMaxDepth = 3

const dcmdProg = "check-doc-cmds.py"

const dcmdUsageLine = "usage: check-doc-cmds.py [-h] [--scope {formal,devdocs}] [--self-test]"

// dcmdReadFails —— B 类读失败登记面（读不了的 `.md` / 命令树源件 · 逐件登记 相对路径 + err 正文）。
var dcmdReadFails []string

const dcmdHelp = `usage: check-doc-cmds.py [-h] [--scope {formal,devdocs}] [--self-test]

门⑫ · 文档-命令一致性（M12 + D3 · T-51）

optional arguments:
  -h, --help            show this help message and exit
  --scope {formal,devdocs}
                        语料域（可重复；缺省 = 两棵都判）
  --self-test
`

// dcmdJSONFields —— 本门机器读面字段表（唯一真源：印表与拒收同读这一处 · Q-176）。空 ⇒ 只出人读面。
var dcmdJSONFields = []string{}

// 排除表：与仓内其它门的扫描口径同一张（grep --exclude-dir 的同款），不留静默的洞。
var dcmdExcl = map[string]bool{
	"target": true, "node_modules": true, "dist": true, "bin": true, "vendor": true,
	"data": true, ".git": true, ".venv": true, "venv": true, ".build": true,
	"__pycache__": true, ".history": true, ".obsidian": true,
}

// ── 抽取正则（逐字复刻 py；★ `^` 锚定以复刻 py 的 `.match` 语义）──────────────────────────
var (
	dcmdZergInline  = regexp.MustCompile("`zerg(\\s+[^\\s`][^`]*)?`")
	dcmdFence       = regexp.MustCompile("^(```|~~~)")
	dcmdCmdWord     = regexp.MustCompile("^[A-Za-z][A-Za-z0-9_-]*")
	dcmdFlagWord    = regexp.MustCompile("^[\\[\\(]*-{1,2}[A-Za-z]")
	dcmdIdentitySeg = regexp.MustCompile("`?zerg\\s+(?:[0-9]+\\.[0-9]+[0-9A-Za-z.+\\-]*\\s+)?[0-9a-fA-F]{7,64}(?:\\+dirty)?\\s+[0-9]{4}-[0-9]{2}-[0-9]{2}[T ][0-9:]{5,8}Z?[^`]*`?")
	dcmdVersionTok  = regexp.MustCompile("^[0-9]+(?:\\.[0-9]+)+[0-9A-Za-z.+\\-]*$")
	// dcmdProposalMark —— 未建提案书写体例（2026-10-02 · 分诊批1 · bde233fb · 逐字复刻 py PROPOSAL_MARK）：
	//   只认**紧贴 `zerg` 之前、只隔空白的判语**（`建议收编 \`zerg X\`` 这种中间有别的词的**不认**）。
	dcmdProposalMark = regexp.MustCompile("(?:想要|建议|须|未建|待建|待拍|候选|规划|新面|不建|尚未建|已拍未建|拟建|应建|提出)\\s*$")
	dcmdFenceSeg     = regexp.MustCompile("^zerg(\\s+\\S.*)?$")
	dcmdPathLit      = regexp.MustCompile("path:\\s*\\[\\]string\\{([^}]*)\\}")
	dcmdQuoted       = regexp.MustCompile(`"([^"]+)"`)
	dcmdIDSkipRE     = regexp.MustCompile("身份行 / 版本串形态\\s*([0-9]+)\\s*段")
	dcmdPropSkipRE   = regexp.MustCompile("未建提案体例[^0-9]*([0-9]+)\\s*段")
)

// dcmdHit —— 一段抽出的 `zerg …`（行号 + 命令词）。
type dcmdHit struct {
	lineno int
	words  []string
}

// dcmdFile —— 语料件（相对串 + 绝对路径）。
type dcmdFile struct {
	rel, abs string
}

// ══ posixpath 语义小工具（逐字复刻 py 的 os.path.*）══════════════════════════════════════

// dcmdSplitPath —— posixpath.split 语义：返回 (head, tail)。
func dcmdSplitPath(p string) (string, string) {
	i := strings.LastIndex(p, "/") + 1
	return p[:i], p[i:]
}

// dcmdDir —— posixpath.dirname 语义。
func dcmdDir(p string) string {
	head, _ := dcmdSplitPath(p)
	if head == "" {
		return ""
	}
	trimmed := strings.TrimRight(head, "/")
	if trimmed == "" {
		return "/"
	}
	return trimmed
}

// dcmdBase —— posixpath.basename 语义（尾随斜杠 ⇒ ""）。
func dcmdBase(p string) string {
	_, tail := dcmdSplitPath(p)
	return tail
}

// dcmdJoin —— posixpath.join 语义（b 为绝对则整个替换；a 以 "/" 结尾则直连）。
func dcmdJoin(a string, parts ...string) string {
	path := a
	for _, b := range parts {
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

// dcmdNormpath —— posixpath.normpath 语义（简化 · 覆盖真仓/夹具形态）。
func dcmdNormpath(p string) string {
	if p == "" {
		return "."
	}
	abs := strings.HasPrefix(p, "/")
	parts := strings.Split(p, "/")
	out := []string{}
	for _, seg := range parts {
		switch seg {
		case "", ".":
			continue
		case "..":
			if len(out) > 0 && out[len(out)-1] != ".." {
				out = out[:len(out)-1]
			} else if !abs {
				out = append(out, "..")
			}
		default:
			out = append(out, seg)
		}
	}
	res := strings.Join(out, "/")
	if abs {
		res = "/" + res
	}
	if res == "" {
		if abs {
			return "/"
		}
		return "."
	}
	return res
}

// dcmdAbs —— os.path.abspath 语义。
func dcmdAbs(p string) string {
	if strings.HasPrefix(p, "/") {
		return dcmdNormpath(p)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return dcmdNormpath(p)
	}
	return dcmdNormpath(dcmdJoin(cwd, p))
}

// dcmdRelpath —— posixpath.relpath 语义。
func dcmdRelpath(path, start string) string {
	startAbs := dcmdAbs(start)
	pathAbs := dcmdAbs(path)
	split := func(s string) []string {
		out := []string{}
		for _, x := range strings.Split(s, "/") {
			if x != "" {
				out = append(out, x)
			}
		}
		return out
	}
	sl := split(startAbs)
	pl := split(pathAbs)
	i := 0
	for i < len(sl) && i < len(pl) && sl[i] == pl[i] {
		i++
	}
	rel := []string{}
	for k := i; k < len(sl); k++ {
		rel = append(rel, "..")
	}
	rel = append(rel, pl[i:]...)
	if len(rel) == 0 {
		return "."
	}
	return strings.Join(rel, "/")
}

// dcmdIsFile —— os.path.isfile 语义。
func dcmdIsFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// dcmdEnvOr —— `env.get(K) or default`：空串视同未设。
func dcmdEnvOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// ══ 抽取 / 清洗 ══════════════════════════════════════════════════════════════════════════

// dcmdSplitLines —— str.splitlines 语义（\n \r \r\n \v \f \x1c-\x1e \x85 \u2028 \u2029；尾空行丢弃）。
func dcmdSplitLines(s string) []string {
	out := []string{}
	var cur strings.Builder
	rs := []rune(s)
	for i := 0; i < len(rs); i++ {
		r := rs[i]
		brk := r == '\n' || r == '\r' || r == '\v' || r == '\f' || r == 0x1c || r == 0x1d ||
			r == 0x1e || r == 0x85 || r == 0x2028 || r == 0x2029
		if brk {
			if r == '\r' && i+1 < len(rs) && rs[i+1] == '\n' {
				i++
			}
			out = append(out, cur.String())
			cur.Reset()
			continue
		}
		cur.WriteRune(r)
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

// dcmdSegmentWords —— 一段 `zerg …` 之后的行文 → (类别, 命令词)；类别 ∈ {cmd,identity,noise}。
func dcmdSegmentWords(seg string) (string, []string) {
	toks := strings.Fields(seg)
	first := ""
	if len(toks) > 0 {
		first = toks[0]
	}
	if first != "" && !dcmdCmdWord.MatchString(first) && !dcmdFlagWord.MatchString(first) {
		if dcmdVersionTok.MatchString(first) {
			return "identity", nil
		}
		return "noise", nil
	}
	return "cmd", dcmdCommandWords(seg)
}

// dcmdCountSkip —— 两类「认出来才丢」的段数分别计数（stats 为 nil 时只当没传）。
func dcmdCountSkip(stats map[string]int, kind string) {
	if stats == nil || kind == "cmd" {
		return
	}
	stats[kind] = stats[kind] + 1
}

// dcmdCommandWords —— 一段 `zerg …` 之后的行文 → 命令词（剥掉旗标与**紧跟旗标的取值**）。
func dcmdCommandWords(seg string) []string {
	out := []string{}
	skip := false
	for _, tok := range strings.Fields(seg) {
		if dcmdFlagWord.MatchString(tok) {
			skip = true
			continue
		}
		if skip {
			skip = false
			continue
		}
		if m := dcmdCmdWord.FindString(tok); m != "" {
			out = append(out, m)
		}
	}
	return out
}

// dcmdSkipSpace —— 从 i 起吞掉 Unicode 空白（复刻 py `\s*` 的 Unicode 语义）。
func dcmdSkipSpace(s string, i int) int {
	for i < len(s) {
		r, sz := decodeRune(s, i)
		if !unicode.IsSpace(r) {
			break
		}
		i += sz
	}
	return i
}

func decodeRune(s string, i int) (rune, int) {
	for _, r := range s[i:] {
		return r, len(string(r))
	}
	return 0, 1
}

// dcmdPrevIsSpace —— 复刻 `(?<!\S)`：i==0 或前一个字符是空白（ASCII 判定 · 见 (b)/未覆盖④）。
func dcmdPrevIsSpace(s string, i int) bool {
	if i == 0 {
		return true
	}
	b := s[i-1]
	if b >= 0x80 {
		return false // 多字节字符的尾字节：按「非 ASCII 空白」处理（未覆盖④）
	}
	return b == ' ' || b == '\t' || b == '\n' || b == '\r' || b == '\v' || b == '\f'
}

// dcmdMatchPromptSep —— 复刻 `(?:\$|PS\s*[A-Za-z]>|>)\s*` 在 i 处的匹配；返回结束位置。
func dcmdMatchPromptSep(s string, i int) (int, bool) {
	if i < len(s) && s[i] == '$' {
		return dcmdSkipSpace(s, i+1), true
	}
	if strings.HasPrefix(s[i:], "PS") {
		j := dcmdSkipSpace(s, i+2)
		if j < len(s) && isASCIILetter(s[j]) && j+1 < len(s) && s[j+1] == '>' {
			return dcmdSkipSpace(s, j+2), true
		}
	}
	if i < len(s) && s[i] == '>' {
		return dcmdSkipSpace(s, i+1), true
	}
	return i, false
}

func isASCIILetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

// dcmdSplitPrompt —— 复刻 `re.split(r'(?<!\S)(?:\$|PS\s*[A-Za-z]>|>)\s*', line)`。
func dcmdSplitPrompt(s string) []string {
	out := []string{}
	last := 0
	i := 0
	for i < len(s) {
		if dcmdPrevIsSpace(s, i) {
			if j, ok := dcmdMatchPromptSep(s, i); ok {
				out = append(out, s[last:i])
				last = j
				i = j
				continue
			}
		}
		i++
	}
	out = append(out, s[last:])
	return out
}

// dcmdExtract —— 一段 markdown → 抽出的 `zerg …` 段（围栏内 + 行内都抽；只抽 zerg 开头）。
func dcmdExtract(text string, stats map[string]int) []dcmdHit {
	hits := []dcmdHit{}
	inFence := false
	for idx, line := range dcmdSplitLines(text) {
		lineno := idx + 1
		nID := len(dcmdIdentitySeg.FindAllStringIndex(line, -1))
		if nID > 0 {
			line = dcmdIdentitySeg.ReplaceAllString(line, "")
			if stats != nil {
				stats["identity_lines"] = stats["identity_lines"] + nID
			}
		}
		stripped := strings.TrimSpace(line)
		if dcmdFence.MatchString(stripped) {
			inFence = !inFence
			continue
		}
		if inFence {
			for _, seg := range dcmdSplitPrompt(stripped) {
				if m := dcmdFenceSeg.FindStringSubmatch(strings.TrimSpace(seg)); m != nil {
					kind, words := dcmdSegmentWords(m[1])
					dcmdCountSkip(stats, kind)
					hits = append(hits, dcmdHit{lineno, words})
				}
			}
		} else {
			for _, m := range dcmdZergInline.FindAllStringSubmatchIndex(line, -1) {
				// 未建提案书写体例：`zerg <名>` 紧跟在判语之后 ⇒ 那是**账体例里的提案名**、
				//   不是命令示例 ⇒ 不计（见件头「未建提案书写体例」）。与身份行同法：**认出来才丢**。
				if dcmdProposalMark.MatchString(line[:m[0]]) {
					dcmdCountSkip(stats, "proposal")
					hits = append(hits, dcmdHit{lineno, nil})
					continue
				}
				seg := ""
				if m[2] >= 0 {
					seg = line[m[2]:m[3]]
				}
				kind, words := dcmdSegmentWords(seg)
				dcmdCountSkip(stats, kind)
				hits = append(hits, dcmdHit{lineno, words})
			}
		}
	}
	return hits
}

// ── 语料树遍历（复刻 os.walk：未排序 readdir 序 + 深度优先 + 名过滤 + 不下钻符号链接）─────────
func dcmdCollectMD(root, relDir string) ([]dcmdFile, bool) {
	base := dcmdJoin(root, relDir)
	st, err := os.Stat(base)
	if err != nil || !st.IsDir() {
		return nil, false
	}
	out := []dcmdFile{}
	var walk func(dir string)
	walk = func(dir string) {
		f, err := os.Open(dir)
		if err != nil {
			// ★ B 类真口子修（原 py `except OSError` 静默 · ✗ 不再静默）：登记 + 判红 + rc=2。
			dcmdReadFails = append(dcmdReadFails, fmt.Sprintf("%s（%v）", dcmdRelpath(dir, root), err))
			return
		}
		entries, err := f.ReadDir(-1) // 未排序：与 py os.scandir/readdir 同序（实测逐位相同）
		f.Close()
		if err != nil {
			return
		}
		dirs := []string{}
		for _, e := range entries {
			p := dcmdJoin(dir, e.Name())
			esz, serr := os.Stat(p)
			if serr == nil && esz.IsDir() {
				lst, lerr := os.Lstat(p)
				isLink := lerr == nil && lst.Mode()&os.ModeSymlink != 0
				if !dcmdExcl[e.Name()] && !isLink { // os.walk(followlinks=False)：不下钻符号链接
					dirs = append(dirs, p)
				}
			} else if strings.HasSuffix(e.Name(), ".md") {
				out = append(out, dcmdFile{rel: dcmdRelpath(p, root), abs: p})
			}
		}
		for _, d := range dirs {
			walk(d)
		}
	}
	walk(base)
	return out, true
}

// ── 命令树 ══════════════════════════════════════════════════════════════════════════════

func dcmdKey(parts []string) string {
	return strings.Join(parts, "\x00")
}

// dcmdParseTree —— 命令树 → {key: 是不是一条完整命令}（L1/L2 也登记，值 False）。
func dcmdParseTree(root string) map[string]bool {
	b, err := os.ReadFile(dcmdJoin(root, dcmdMainRel))
	if err != nil {
		// ★ B 类真口子修（原 py `except OSError` 静默 · ✗ 不再静默）：登记 + 判红 + rc=2。
		dcmdReadFails = append(dcmdReadFails, fmt.Sprintf("%s（%v）", dcmdMainRel, err))
		return nil
	}
	src := string(b)
	start := strings.Index(src, "commands = []*command{")
	if start < 0 {
		return nil
	}
	end := strings.Index(src[start:], "\n\t}\n")
	if end < 0 {
		return nil
	}
	seg := src[start : start+end]
	out := map[string]bool{}
	for _, m := range dcmdPathLit.FindAllStringSubmatch(seg, -1) {
		parts := []string{}
		for _, q := range dcmdQuoted.FindAllStringSubmatch(m[1], -1) {
			parts = append(parts, q[1])
		}
		if len(parts) == 0 {
			continue
		}
		out[dcmdKey(parts)] = true
		for _, k := range []int{1, 2} {
			if k < len(parts) {
				if _, ok := out[dcmdKey(parts[:k])]; !ok {
					out[dcmdKey(parts[:k])] = false
				}
			}
		}
	}
	return out
}

// dcmdResolveCmd —— 命令树逐段收窄（最长前缀）⇒ 命中的命令路径长度 k（0 = 拼不出）。
func dcmdResolveCmd(words []string, tree map[string]bool) int {
	k := 0
	limit := len(words)
	if dcmdMaxDepth < limit {
		limit = dcmdMaxDepth
	}
	for n := 1; n <= limit; n++ {
		if _, ok := tree[dcmdKey(words[:n])]; ok {
			k = n
		}
	}
	if k == 0 {
		return 0
	}
	if v, ok := tree[dcmdKey(words[:k])]; ok && v {
		return k
	}
	if k == 2 {
		return k
	}
	if k == len(words) {
		return k
	}
	return 0
}

// ══ 判据主体 ═════════════════════════════════════════════════════════════════════════════

func dcmdJudge(files []dcmdFile, tree map[string]bool, blocking bool, stats map[string]int) (int, int, []string, int) {
	n := 0
	nUnread := 0 // ● GAP-20261001-32③ 修（照 py 候件逐处同步）：读不了的件计数 ⇒ 调用方归 blocked（进 rc 通道）
	details := []string{}
	for _, f := range files {
		b, err := os.ReadFile(f.abs)
		if err != nil {
			// ★ B 类真口子修（原 py `except OSError` 静默 · ✗ 不再静默）：登记 + 判红 + rc=2。
			dcmdReadFails = append(dcmdReadFails, fmt.Sprintf("%s（%v）", f.rel, err))
			nUnread++
			details = append(details, fmt.Sprintf("读不了 %s：%s", f.rel, err))
			continue
		}
		for _, h := range dcmdExtract(string(b), stats) {
			if len(h.words) == 0 {
				continue
			}
			if dcmdResolveCmd(h.words, tree) != 0 {
				continue
			}
			n++
			w := h.words
			if len(w) > 2 {
				w = w[:2]
			}
			details = append(details, fmt.Sprintf("%s:%d → `zerg %s` 不在命令树（最长前缀判据）",
				f.rel, h.lineno, strings.Join(w, " ")))
		}
	}
	if blocking {
		return n, 0, details, nUnread
	}
	return 0, n, details, nUnread
}

func dcmdLoadBaseline(root string) (int, bool) {
	b, err := os.ReadFile(dcmdJoin(root, dcmdBaselineRel))
	if err != nil {
		// ★ 基线件读不了走它自己的 rc=2 路（调用处判 false ⇒ 打「基线缺件」文案）；
		// ✗ 不并入 dcmdReadFails —— 那是「扫描面判据件读不了 ⇒ 覆盖不完整」的登记面，两者语义不同。
		return 0, false
	}
	var d map[string]interface{}
	if err := json.Unmarshal(b, &d); err != nil {
		return 0, false
	}
	v, ok := d["devdocs_report_hits"]
	if !ok {
		return 0, false
	}
	switch t := v.(type) {
	case float64:
		return int(t), true
	case string:
		i, err := strconv.Atoi(t)
		if err != nil {
			return 0, false
		}
		return i, true
	}
	return 0, false
}

// dcmdSiblingDocsBase —— 同级开发文档仓根（与同族件同一取根规则）。
func dcmdSiblingDocsBase(root string) string {
	if v := os.Getenv("ZERG_DOCS_ALT"); v != "" {
		return dcmdAbs(v)
	}
	if v := os.Getenv("ZERG_DOCS"); v != "" {
		return dcmdAbs(v)
	}
	return dcmdJoin(dcmdDir(dcmdAbs(root)), "Zerg-内部文档")
}

// dcmdDevdocsRootIsRepoRoot —— 根形态误用探测器（GAP-20260927-83 · 照 py `devdocs_root_is_repo_root` 逐字）：
// 把**文档仓仓根**当版本面根传进来时，它下面一定有 `项目文档/`（版本目录的父）⇒ 判为形态误用。
func dcmdDevdocsRootIsRepoRoot(p string) bool {
	st, err := os.Stat(dcmdJoin(p, "项目文档"))
	return err == nil && st.IsDir()
}

func dcmdHead(xs []string, n int) []string {
	if len(xs) > n {
		return xs[:n]
	}
	return xs
}

func dcmdCheck(root string, scopes []string) (int, []string) {
	dcmdReadFails = nil
	blockedMsg := fmt.Sprintf("⛔ 不给结论：命令树读不出来（%s 缺件/空转）—— 判据不可判 ⇒ 不给结论", dcmdMainRel)
	if !dcmdIsFile(dcmdJoin(root, dcmdMainRel)) {
		return dcmdRCBlocked, []string{blockedMsg}
	}
	tree := dcmdParseTree(root)
	if tree == nil || len(tree) == 0 {
		return dcmdRCBlocked, []string{blockedMsg}
	}
	baseline, ok := dcmdLoadBaseline(root)
	if !ok {
		return dcmdRCBlocked, []string{fmt.Sprintf("⛔ 不给结论：baseline 缺件或解不动（%s）—— 棘轮没法判 ⇒ 不给结论", dcmdBaselineRel)}
	}
	errs := []string{}
	lines := []string{}
	blocked := []string{}
	detailAll := []string{}
	devdocsHits, formalHits := 0, 0
	stats := map[string]int{}
	for _, scope := range scopes {
		if scope == "formal" {
			files, ok := dcmdCollectMD(root, "docs")
			if !ok {
				blocked = append(blocked, "⛔ 不给结论：formal 语料不在（docs/ 整棵缺席 · 语料域缺件）—— 判据不可判")
				continue
			}
			if len(files) == 0 {
				blocked = append(blocked, "⛔ 不给结论：formal 扫到 0 篇（空转 = 假绿）—— 判据不可判")
				continue
			}
			n, _, details, nUnread := dcmdJudge(files, tree, true, stats)
			if nUnread > 0 {
				// ● GAP-20261001-32③ 修（照 py 候件逐处同步）：读不了 ≠ 零命中 ⇒ 归 blocked（rc=2）。
				blocked = append(blocked, fmt.Sprintf("⛔ 不给结论：formal 语料有 %d 篇读不了（读不了 ≠ 零命中）—— 判据不可判", nUnread))
			}
			formalHits += n
			if n > 0 {
				errs = append(errs, fmt.Sprintf("formal（正式面 · **阻断**）：%d 处手写 `zerg …` 示例拼不到命令树（§九 M12）", n))
			}
			detailAll = append(detailAll, dcmdHead(details, 20)...)
		} else {
			docsRoot := dcmdEnvOr("ZERG_DEVDOCS_ROOT", dcmdJoin(dcmdSiblingDocsBase(root), "项目文档", "v2.5.13"))
			if dcmdDevdocsRootIsRepoRoot(docsRoot) {
				// ★ 照 py 候件逐字补：根**形态误用**（把文档仓仓根当版本面根）⇒ 与「根不在盘」同档 rc=2。
				blocked = append(blocked, fmt.Sprintf("⛔ 不给结论：devdocs 根**形态误用**（%s 下面还有 `项目文档/` ⇒ 这是"+
					"**文档仓仓根**、不是版本面根）—— 拿整仓根当面扫会把整棵树误判 ⇒ 判据不可判"+
					"（`GAP-20260927-83`）：请设 `ZERG_DEVDOCS_ROOT=<...>/项目文档/vX.Y.Z`", docsRoot))
				continue
			}
			files, ok := dcmdCollectMD(dcmdDir(docsRoot), dcmdBase(docsRoot))
			if !ok {
				blocked = append(blocked, fmt.Sprintf("⛔ 不给结论：devdocs 根整棵不在盘（%s · 语料域缺件）—— 判据不可判（换根请设 `ZERG_DEVDOCS_ROOT` 或同级根 `ZERG_DOCS_ALT`/`ZERG_DOCS`）", docsRoot))
				continue
			}
			if len(files) == 0 {
				blocked = append(blocked, "⛔ 不给结论：devdocs 扫到 0 篇（空转 = 假绿）—— 判据不可判")
				continue
			}
			_, r, details, nUnread := dcmdJudge(files, tree, false, stats)
			if nUnread > 0 {
				// ● GAP-20261001-32③ 修（照 py 候件逐处同步）：读不了 ≠ 零命中 ⇒ 归 blocked（rc=2）。
				blocked = append(blocked, fmt.Sprintf("⛔ 不给结论：devdocs 语料有 %d 篇读不了（读不了 ≠ 零命中）—— 判据不可判", nUnread))
			}
			devdocsHits += r
			lines = append(lines, fmt.Sprintf("devdocs（只报告档 · §6.3 S6 起步）：命中 %d 处（baseline %d）", r, baseline))
			detailAll = append(detailAll, dcmdHead(details, 20)...)
		}
	}
	if devdocsHits > baseline {
		errs = append(errs, fmt.Sprintf("T-51 棘轮：devdocs 命中 %d > baseline %d（只许**下调**）", devdocsHits, baseline))
	}
	lines = append(lines, fmt.Sprintf("命令树 %d 条（L1+L2）· formal 命中 %d（阻断档）· devdocs 命中 %d（只报告档）",
		len(tree), formalHits, devdocsHits))
	lines = append(lines, fmt.Sprintf("识别跳过（**不计入**上面两个数）：身份行 / 版本串形态 %d 段 · 非命令噪声 %d 段"+
		" · 未建提案体例（判语引出的 `zerg <名>`）%d 段"+
		" —— ★ 三类都是**认出来了**才丢（≠ 修前那种「抽不出词 ⇒ 静默」）",
		stats["identity_lines"]+stats["identity"], stats["noise"], stats["proposal"]))
	for _, d := range detailAll {
		lines = append(lines, "  "+d)
	}
	lines = append(lines, blocked...)
	for _, e := range errs {
		lines = append(lines, "✗ "+e)
	}
	if len(blocked) > 0 {
		return dcmdRCBlocked, lines
	}
	if len(errs) > 0 {
		return dcmdRCFail, lines
	}
	return dcmdRCOK, lines
}

// ══ 入口 ════════════════════════════════════════════════════════════════════════════════

// dcmdDefaultRepo —— Go 侧无 `__file__` 语境：从 cwd 上溯找含 core/cmd/zerg/main.go 的目录（见 (a)）。
func dcmdDefaultRepo() string {
	cwd, err := os.Getwd()
	if err != nil {
		return dcmdNormpath(".")
	}
	d := cwd
	for i := 0; i < 8; i++ {
		if dcmdIsFile(dcmdJoin(d, dcmdMainRel)) {
			return dcmdNormpath(d)
		}
		parent := dcmdDir(d)
		if parent == d || parent == "" {
			break
		}
		d = parent
	}
	return dcmdNormpath(cwd)
}

func dcmdArgErr(stderr io.Writer, msg string) {
	fmt.Fprintf(stderr, "%s\n", dcmdUsageLine)
	fmt.Fprintf(stderr, "%s: error: %s\n", dcmdProg, msg)
}

func dcmdValidScope(v string) bool {
	return v == "formal" || v == "devdocs"
}

// dcmdParseArgs —— 复刻 py 的 argparse（`--scope` append + `--self-test` + `-h/--help`）。
// 返回 (scopes, selfTest, rc, done)：done=true 表示参数面已给出终局（帮助/错误）。
func dcmdParseArgs(argv []string, stdout, stderr io.Writer) ([]string, bool, int, bool) {
	scopes := []string{}
	selfTest := false
	unknown := []string{}
	i := 0
	for i < len(argv) {
		a := argv[i]
		if a == "--" {
			unknown = append(unknown, argv[i+1:]...)
			break
		}
		switch {
		case a == "-h" || a == "--help":
			fmt.Fprint(stdout, dcmdHelp)
			return nil, false, dcmdRCOK, true
		case a == "--self-test":
			selfTest = true
			i++
		case a == "--scope":
			if i+1 >= len(argv) {
				dcmdArgErr(stderr, "argument --scope: expected one argument")
				return nil, false, dcmdRCBlocked, true
			}
			v := argv[i+1]
			if !dcmdValidScope(v) {
				dcmdArgErr(stderr, fmt.Sprintf("argument --scope: invalid choice: '%s' (choose from 'formal', 'devdocs')", v))
				return nil, false, dcmdRCBlocked, true
			}
			scopes = append(scopes, v)
			i += 2
		case strings.HasPrefix(a, "--scope="):
			v := a[len("--scope="):]
			if !dcmdValidScope(v) {
				dcmdArgErr(stderr, fmt.Sprintf("argument --scope: invalid choice: '%s' (choose from 'formal', 'devdocs')", v))
				return nil, false, dcmdRCBlocked, true
			}
			scopes = append(scopes, v)
			i++
		case strings.HasPrefix(a, "-") && a != "-":
			unknown = append(unknown, a)
			i++
		default:
			unknown = append(unknown, a)
			i++
		}
	}
	if len(unknown) > 0 {
		dcmdArgErr(stderr, "unrecognized arguments: "+strings.Join(unknown, " "))
		return nil, false, dcmdRCBlocked, true
	}
	return scopes, selfTest, 0, false
}

// checkDocCmdsGateMain —— 真搬入口（M2 刻意无调用点）。
func checkDocCmdsGateMain(stdout, stderr io.Writer, argv []string) int {
	dcmdReadFails = nil // ★ 自清：包级登记面不许跨调用残留（残留会让下一次一进门就被 BLOCKED）
	// ── Q-176：本门无机器读面 ⇒ 给了 --json 不许沉默（先于 argparse）────────────────────
	jsonHits := []string{}
	for _, x := range argv {
		if x == "--json" || strings.HasPrefix(x, "--json=") {
			jsonHits = append(jsonHits, "`"+x+"`")
		}
	}
	if len(jsonHits) > 0 {
		fmt.Fprintf(stderr, "⛔ 本门无机器读面：%s 在本门**未实现**（缺口 Q-176）\n", strings.Join(jsonHits, " · "))
		if len(dcmdJSONFields) == 0 {
			fmt.Fprintf(stderr, "   字段表：%s\n", "无（本门只出人读面）")
		} else {
			quoted := make([]string, len(dcmdJSONFields))
			for i, f := range dcmdJSONFields {
				quoted[i] = "`" + f + "`"
			}
			fmt.Fprintf(stderr, "   字段表：%s\n", strings.Join(quoted, " · "))
		}
		fmt.Fprint(stderr, "⇒ 用法错（rc=2）：**机器面缺 ≠ 零命中** —— 两态不许同形\n")
		return dcmdRCBlocked
	}
	scopes, selfTest, rc, done := dcmdParseArgs(argv, stdout, stderr)
	if done {
		return rc
	}
	if selfTest {
		return dcmdSelfTest(stdout)
	}
	root := dcmdDefaultRepo()
	if !dcmdIsFile(dcmdJoin(root, dcmdMainRel)) {
		fmt.Fprintf(stdout, "⛔ 不给结论：找不到仓根（%s 不在）\n", dcmdMainRel)
		return dcmdRCBlocked
	}
	if len(scopes) == 0 {
		scopes = []string{"formal", "devdocs"}
	}
	jc, lines := dcmdCheck(root, scopes)
	// ★ 2026-10-04 同步（候选 · GAP-20261001-32③）：先把 stdout 人读面印全（含 py 新增的「读不了 ⇒ blocked」
	//   文案），再落 B 类登记面的 stderr BLOCKED（Go-only 分叉）—— 改前这里在印 stdout 之前就早退，
	//   会把 py 侧新增的那条 blocked 文案整个吞掉（= 同步落空）。
	fmt.Fprintf(stdout, "门⑫ 文档-命令一致性（§九 M12 · §6.4 D3 · 开工单 T-51）· 仓根 %s\n", root)
	for _, l := range lines {
		fmt.Fprintf(stdout, "  %s\n", l)
	}
	verdict := jc
	if len(dcmdReadFails) > 0 {
		// B 类登记面响 ⇒ 出口一律 rc=2（与下面 return dcmdRCBlocked 同源），stdout 判词同步，✗ 不许
		// 出现「stdout 印 rc=0 而进程 rc=2」的自相矛盾（dir 级读失败与 judge 级 nUnread 可不同源）。
		verdict = dcmdRCBlocked
	}
	switch verdict {
	case dcmdRCOK:
		fmt.Fprint(stdout, "⇒ 全过（rc=0）\n")
	case dcmdRCFail:
		fmt.Fprint(stdout, "⇒ 判红（rc=1）：逐条看上面的命中\n")
	default:
		fmt.Fprint(stdout, "⇒ 不给结论（rc=2）：缺件/空转 —— **不许当绿**\n")
	}
	if len(dcmdReadFails) > 0 {
		fmt.Fprintf(stderr, "BLOCKED —— 有 %d 件读不到（B 类读失败 ⇒ 登记 + 判红 · rc=2 · 不给结论）：\n", len(dcmdReadFails))
		for _, u := range dcmdReadFails {
			fmt.Fprintf(stderr, "  [读失败] %s\n", u)
		}
		return dcmdRCBlocked
	}
	return jc
}

// ══ 成对负控自检 ═══════════════════════════════════════════════════════════════════════════

func dcmdSelfTest(stdout io.Writer) int {
	dcmdReadFails = nil // ★ 自清：包级登记面不许跨调用残留（残留会让下一次一进门就被 BLOCKED）
	bad := []string{}
	ran := []string{}
	tmp, err := os.MkdirTemp("", "doc-cmds-selftest-")
	if err != nil {
		fmt.Fprintf(stdout, "自检失败：\n  ✗ 夹具建不起来：%s\n", err)
		return dcmdRCBlocked
	}
	defer os.RemoveAll(tmp) // py 不回收；本件回收（输出不可见差异）

	os.MkdirAll(dcmdJoin(tmp, "core", "cmd", "zerg"), 0o755)
	os.MkdirAll(dcmdJoin(tmp, "scripts", "gates"), 0o755)
	treeSrc := "var commands []*command\n" +
		"func init() {\n\tcommands = []*command{\n" +
		"\t\t{path: []string{\"version\"}, run: cmdVersion},\n" +
		"\t\t{path: []string{\"dev\", \"proposal\"}, run: cmdDevProposal},\n" +
		"\t\t{path: []string{\"help\"}, run: cmdHelp},\n" +
		"\t\t{path: []string{\"doctor\"}, run: cmdDoctor},\n" +
		"\t\t{path: []string{\"update\"}, run: cmdUpdate},\n" +
		"\t\t{path: []string{\"agent\", \"logs\"}, run: cmdAgentLogs},\n" +
		"\t\t{path: []string{\"task\", \"ls\"}, run: cmdTaskLs},\n" +
		"\t\t{path: []string{\"isolate-ro\"}, run: cmdIsolateRO},\n" +
		"\t}\n}\n"
	writeFile := func(p, data string) {
		if err := os.WriteFile(p, []byte(data), 0o644); err != nil {
			bad = append(bad, "夹具写不了 "+p+"："+err.Error())
		}
	}

	writeFile(dcmdJoin(tmp, dcmdMainRel), treeSrc)
	writeFile(dcmdJoin(tmp, dcmdBaselineRel), "{\"devdocs_report_hits\": 0, \"note\": \"selftest\"}")

	caseFn := func(name string, want, got int) {
		ran = append(ran, name)
		if want != got {
			bad = append(bad, fmt.Sprintf("%s：want rc=%d got rc=%d", name, want, got))
		}
		fmt.Fprintf(stdout, "  %s ⇒ rc=%d\n", name, got)
	}
	idSkips := func(lines []string) int {
		for _, l := range lines {
			if m := dcmdIDSkipRE.FindStringSubmatch(l); m != nil {
				n, _ := strconv.Atoi(m[1])
				return n
			}
		}
		return -1
	}
	// propSkips —— 读「未建提案体例 … N 段」那行的计数（认出来了才丢的**证据** ⇒ ≥1 才算认过）。
	propSkips := func(lines []string) int {
		for _, l := range lines {
			if m := dcmdPropSkipRE.FindStringSubmatch(l); m != nil {
				n, _ := strconv.Atoi(m[1])
				return n
			}
		}
		return -1
	}
	has := func(lines []string, sub string) bool {
		for _, l := range lines {
			if strings.Contains(l, sub) {
				return true
			}
		}
		return false
	}

	// ① 正控：好件 ⇒ 0
	docs := dcmdJoin(tmp, "docs")
	os.MkdirAll(docs, 0o755)
	writeFile(dcmdJoin(docs, "good.md"), "跑这个：\n```sh\nzerg dev proposal list\n```\n行内 `zerg version` 也过。\n")
	rc, _ := dcmdCheck(tmp, []string{"formal"})
	caseFn("① 正控（好件 ⇒ 0）", 0, rc)

	// ② 负控：坏件 ⇒ 1
	writeFile(dcmdJoin(docs, "bad.md"), "```sh\nzerg dev propozal list\n```\n")
	rc, lines := dcmdCheck(tmp, []string{"formal"})
	caseFn("② 负控（坏件 ⇒ 1）", 1, rc)
	if !has(lines, "propozal") {
		bad = append(bad, "② 负控：判词里没有坏命令名")
	}

	// ③ 空转：命令树读不出来 ⇒ 2
	os.Rename(dcmdJoin(tmp, dcmdMainRel), dcmdJoin(tmp, dcmdMainRel)+".bak")
	rc, _ = dcmdCheck(tmp, []string{"devdocs"})
	caseFn("③ 空转（缺命令树 ⇒ 2）", 2, rc)
	os.Rename(dcmdJoin(tmp, dcmdMainRel)+".bak", dcmdJoin(tmp, dcmdMainRel))

	// ④ baseline 缺件 ⇒ 2
	os.Rename(dcmdJoin(tmp, dcmdBaselineRel), dcmdJoin(tmp, dcmdBaselineRel)+".bak")
	rc, _ = dcmdCheck(tmp, []string{"devdocs"})
	caseFn("④ 负控（baseline 缺 ⇒ 2）", 2, rc)
	os.Rename(dcmdJoin(tmp, dcmdBaselineRel)+".bak", dcmdJoin(tmp, dcmdBaselineRel))

	// ⑤ devdocs 命中超 baseline ⇒ 棘轮红（1）
	devdocs := dcmdJoin(tmp, "devdocs")
	os.MkdirAll(devdocs, 0o755)
	writeFile(dcmdJoin(devdocs, "bad.md"), "```sh\nzerg dev propozal list\n```\n")
	os.Setenv("ZERG_DEVDOCS_ROOT", devdocs)
	rc, _ = dcmdCheck(tmp, []string{"devdocs"})
	os.Unsetenv("ZERG_DEVDOCS_ROOT")
	caseFn("⑤ devdocs 命中超 baseline ⇒ 棘轮红（1）", 1, rc)

	// ⑥ 棘轮上调放行 ⇒ 回绿
	writeFile(dcmdJoin(tmp, dcmdBaselineRel), "{\"devdocs_report_hits\": 1, \"note\": \"selftest\"}")
	os.Setenv("ZERG_DEVDOCS_ROOT", devdocs)
	rc, _ = dcmdCheck(tmp, []string{"devdocs"})
	os.Unsetenv("ZERG_DEVDOCS_ROOT")
	caseFn("⑥ devdocs 命中 ≤ baseline ⇒ 0（棘轮回绿）", 0, rc)

	// ⑦ 正控（修前的假红）：真命令带旗标与旗标取值 ⇒ 必须绿
	os.Remove(dcmdJoin(docs, "bad.md"))
	writeFile(dcmdJoin(docs, "good.md"), "```sh\nzerg version --json name\n$ zerg doctor --quick --json name,verdict\n"+
		"zerg update <主机名> --confirm=<主机名>\nzerg agent logs <机器名> --json machine\n"+
		"zerg doctor [--min-ctx n]\n```\n"+
		"行内 `zerg task ls --json id,status` 也过。\n")
	rc, lines = dcmdCheck(tmp, []string{"formal"})
	caseFn("⑦ 正控（真命令带旗标/旗标取值 ⇒ 0；修前是假红）", 0, rc)
	if rc != 0 {
		bad = append(bad, "⑦ 正控：旗标与旗标取值仍被当成命令词")
	}

	// ⑧ 负控：剥旗标不许把区分度剥没了
	writeFile(dcmdJoin(docs, "good.md"), "```sh\nzerg dev propozal --json name\n```\n")
	rc, lines = dcmdCheck(tmp, []string{"formal"})
	caseFn("⑧ 负控（坏命令名 + 旗标 ⇒ 1）", 1, rc)
	if !has(lines, "propozal") {
		bad = append(bad, "⑧ 负控：判词里没有坏命令名")
	}

	// ⑨ 正控：`help <主题>` ⇒ 0
	writeFile(dcmdJoin(docs, "good.md"), "退码表见 `zerg help exit-codes`，输出契约见 `zerg help contract`。\n")
	rc, lines = dcmdCheck(tmp, []string{"formal"})
	caseFn("⑨ 正控（`help <主题>` ⇒ 0；修前是假红）", 0, rc)
	if rc != 0 {
		bad = append(bad, "⑨ 正控：`help <主题>` 仍被判红")
	}

	// ⑩ 身份行 · 行内反引号 · sha 首字母 ⇒ 0
	writeFile(dcmdJoin(docs, "good.md"), "> 身份 / identity：`zerg 2.5.11 ab1ac075+dirty 2026-09-23T17:53:17Z`\n")
	rc, lines = dcmdCheck(tmp, []string{"formal"})
	caseFn("⑩ 正控（身份行 · 行内反引号 · sha 首字母 ⇒ 0；修前是假红）", 0, rc)
	if idSkips(lines) < 1 {
		bad = append(bad, "⑩ 正控：身份行没被**认出来**（识别计数 = 0）—— 换个 sha 值就会两态摇摆")
	}

	// ⑪ sha 首数字 ⇒ 0（且必须认出来）
	writeFile(dcmdJoin(docs, "good.md"), "> 身份 / identity：`zerg 2.5.11 54a3dc51+dirty 2026-09-23T17:53:17Z`\n")
	rc, lines = dcmdCheck(tmp, []string{"formal"})
	caseFn("⑪ 正控（身份行 · sha 首数字 ⇒ 0）", 0, rc)
	if idSkips(lines) < 1 {
		bad = append(bad, "⑪ 正控：sha 首数字那一格仍是**静默**的（识别计数 = 0）⇒ 潜伏口没堵上")
	}

	// ⑫ 版本号 + sha · 无时刻 ⇒ 0（治法② 独立成立）
	writeFile(dcmdJoin(docs, "good.md"), "> 版本 / version：`zerg 2.5.11 ab1ac075+dirty`\n")
	rc, lines = dcmdCheck(tmp, []string{"formal"})
	caseFn("⑫ 正控（版本号 + sha · 无时刻 ⇒ 0 · 治法② 独立成立）", 0, rc)
	if idSkips(lines) < 1 {
		bad = append(bad, "⑫ 正控：治法② 那一格没认出来（识别计数 = 0）")
	}

	// ⑬ 身份行 + 同一行上的真错误 ⇒ 1
	writeFile(dcmdJoin(docs, "good.md"), "> 身份 / identity：`zerg 2.5.11 ab1ac075+dirty 2026-09-23T17:53:17Z`"+
		" · 另见 `zerg dev propozal list`\n")
	rc, lines = dcmdCheck(tmp, []string{"formal"})
	caseFn("⑬ 负控（身份行同一行上的真错误 ⇒ 1 · 治法① 不整行跳过）", 1, rc)
	if !has(lines, "propozal") {
		bad = append(bad, "⑬ 负控：判词里没有坏命令名")
	}

	// ⑭ 身份行与真错误同篇 ⇒ 1
	writeFile(dcmdJoin(docs, "good.md"), "> 身份 / identity：`zerg 2.5.11 ab1ac075+dirty 2026-09-23T17:53:17Z`\n\n"+
		"```sh\nzerg dev propozal list\n```\n")
	rc, lines = dcmdCheck(tmp, []string{"formal"})
	caseFn("⑭ 负控（身份行 + 同篇真错误 ⇒ 1）", 1, rc)
	if idSkips(lines) < 1 {
		bad = append(bad, "⑭ 负控：身份行没被认出来")
	}
	if !has(lines, "propozal") {
		bad = append(bad, "⑭ 负控：判词里没有坏命令名")
	}

	// ── 未建提案书写体例（2026-10-02 · 分诊批1 · bde233fb · 口径治本）──────────────────────────
	//  缺口账/设计稿用「判语 + `zerg <名>`」记录**尚未建**的面（`想要 `zerg <slug>`` 一类）——
	//  那个 `<名>` 是**账里的提案名**，不是**命令示例** ⇒ **不计**（否则门把「记录提案」判成
	//  「引用了不存在的命令」）。三条成对负控（缺一不可）：① 判语引出 ⇒ **不计**；
	//  ② **同一条命令**写成普通示例（无判语）⇒ **必计**；③ 真命令示例 + 拼错 ⇒ **必计并点名**。
	// ⑳ 负控：判语（想要）引出的 `zerg <名>` ⇒ 不计 ⇒ rc=0，且**认出来了**（提案体例计数 ≥1）
	writeFile(dcmdJoin(docs, "good.md"), "未建提案：想要 `zerg pipelinedefect` 收成正门。\n")
	rc, lines = dcmdCheck(tmp, []string{"formal"})
	caseFn("⑳ 负控（判语引出的 `zerg <名>` ⇒ 不计 · rc=0）", 0, rc)
	if propSkips(lines) < 1 {
		bad = append(bad, "⑳ 负控：提案体例没被**认出来**（计数 = 0）—— 换个词就会两态摇摆")
	}

	// ㉑ 正控（成对）：**同一条命令**（`pipelinedefect`）写成**普通示例**（无判语）⇒ **必计** ⇒ rc=1
	writeFile(dcmdJoin(docs, "good.md"), "用法：`zerg pipelinedefect` 可直接跑。\n")
	rc, lines = dcmdCheck(tmp, []string{"formal"})
	caseFn("㉑ 正控（同一条命令无判语 ⇒ 必计 · rc=1）", 1, rc)
	if !has(lines, "pipelinedefect") {
		bad = append(bad, "㉑ 正控：判词里没有该命令名（判据被放宽了）")
	}

	// ㉒ 负控（成对）：真命令示例**拼错**（`dev propozal`，真命令是 `dev proposal`）⇒ **必计并点名**
	writeFile(dcmdJoin(docs, "good.md"), "```sh\nzerg dev propozal list\n```\n")
	rc, lines = dcmdCheck(tmp, []string{"formal"})
	caseFn("㉒ 负控（真命令拼错 ⇒ 必计并点名 · rc=1）", 1, rc)
	if !has(lines, "propozal") {
		bad = append(bad, "㉒ 负控：判词里没有拼错的真命令名")
	}

	// ⑮b 成对负控：formal 语料在且好件 ⇒ 0
	writeFile(dcmdJoin(docs, "good.md"), "行内 `zerg version` 一行好件。\n")
	rc, _ = dcmdCheck(tmp, []string{"formal"})
	caseFn("⑮b 成对负控（formal · 语料在且好件 ⇒ 0）", 0, rc)

	// ⑮ formal · docs/ 整棵不在 ⇒ 2
	os.Rename(docs, docs+".bak")
	rc, lines = dcmdCheck(tmp, []string{"formal"})
	caseFn("⑮ 缺件（formal · docs/ 不在 ⇒ 2；改前 rc=1）", 2, rc)
	if !has(lines, "docs/ 整棵缺席") {
		bad = append(bad, "⑮ 缺件：判词里没说清「docs/ 整棵缺席」")
	}
	os.Rename(docs+".bak", docs)

	// ⑯ formal · docs/ 在但 0 篇 ⇒ 2；⑯b 塞回坏件 ⇒ 照旧 1
	holder := dcmdJoin(tmp, "good.md.hold")
	os.Rename(dcmdJoin(docs, "good.md"), holder)
	rc, _ = dcmdCheck(tmp, []string{"formal"})
	caseFn("⑯ 空转（formal · docs/ 在但 0 篇 ⇒ 2；改前 rc=1）", 2, rc)
	writeFile(dcmdJoin(docs, "bad.md"), "```sh\nzerg dev propozal list\n```\n")
	rc, _ = dcmdCheck(tmp, []string{"formal"})
	caseFn("⑯b 成对负控（0 篇那棵塞回**坏件** ⇒ 照旧 1）", 1, rc)
	os.Remove(dcmdJoin(docs, "bad.md"))
	os.Rename(holder, dcmdJoin(docs, "good.md"))

	// ⑰ devdocs · 扫到 0 篇 ⇒ 2；⑰b 塞回好件 ⇒ 0
	emptyDev := dcmdJoin(tmp, "devdocs-zero")
	os.MkdirAll(emptyDev, 0o755)
	os.Setenv("ZERG_DEVDOCS_ROOT", emptyDev)
	rc, _ = dcmdCheck(tmp, []string{"devdocs"})
	caseFn("⑰ 空转（devdocs · 0 篇 ⇒ 2；改前 rc=1）", 2, rc)
	writeFile(dcmdJoin(emptyDev, "good.md"), "行内 `zerg version` 一行好件。\n")
	rc, _ = dcmdCheck(tmp, []string{"devdocs"})
	caseFn("⑰b 成对负控（devdocs · 0 篇那根塞回好件 ⇒ 0）", 0, rc)
	os.Unsetenv("ZERG_DEVDOCS_ROOT")

	// ⑰c devdocs · 根整棵不在盘 ⇒ 2
	os.Setenv("ZERG_DEVDOCS_ROOT", dcmdJoin(tmp, "no-such-Zerg-内部文档"))
	rc, lines = dcmdCheck(tmp, []string{"devdocs"})
	caseFn("⑰c 缺件（devdocs · 根整棵不在盘 ⇒ 2；改前 0/1 静默绿）", 2, rc)
	if !has(lines, "整棵不在盘") {
		bad = append(bad, "⑰c 缺件：判词里没说清「devdocs 根整棵不在盘」")
	}
	os.Unsetenv("ZERG_DEVDOCS_ROOT")

	// ⑱ 顶层命令 + 子动作 ⇒ 0
	writeFile(dcmdJoin(docs, "good.md"), "```sh\nzerg isolate-ro new <真仓>\n```\n")
	rc, lines = dcmdCheck(tmp, []string{"formal"})
	caseFn("⑱ 正控（顶层命令 + 子动作 ⇒ 0；修前是假红）", 0, rc)
	if rc != 0 {
		bad = append(bad, "⑱ 正控：`isolate-ro new` 仍被判红")
	}

	// ⑲ 负控：族名不许当前缀停靠
	writeFile(dcmdJoin(docs, "good.md"), "```sh\nzerg dev propozal\n```\n")
	rc, lines = dcmdCheck(tmp, []string{"formal"})
	caseFn("⑲ 负控（族名不许当前缀停靠 · 坏动作 ⇒ 1）", 1, rc)
	if !has(lines, "propozal") {
		bad = append(bad, "⑲ 负控：判词里没有坏命令名")
	}

	if len(bad) > 0 {
		fmt.Fprint(stdout, "自检失败：\n")
		for _, b := range bad {
			fmt.Fprintf(stdout, "  ✗ %s\n", b)
		}
		return dcmdRCBlocked
	}
	fmt.Fprintf(stdout, "门⑫ 自检全过（%d 组成对/正控/空转/语料缺件三条分支/身份行噪声负控）\n", len(ran))
	return dcmdRCOK
}
