// check_glossary.go —— 判据件 `scripts/gates/check-glossary.py`（572 行）的 **Go 真搬**
// （v2.5.14 · M2 · 新口径靶：外部树走显式入参旋钮 · 入口 `checkGlossaryGateMain`）。
//
// 验收判据 = 同 argv 跑两态（py 原件 ⟷ 本入口），stdout / stderr / 退码 **三面逐字相同**
// （除下列**显式声明的**新口径差异面）。
//
// 本件与新口径（外部树走显式入参旋钮）—— 逐条对应：
//
//		① 旋钮名**照 py 已有同名旗标/env**：py 本就用 `--root` / `--scope devdocs` 外加
//		   `ZERG_DEVDOCS_ROOT` / `ZERG_DOCS_ALT` / `ZERG_DOCS`（同级根）+ `ZERG_GLOSSARY_TSV`
//		   （主表换表口）⇒ 本件**同名同序照搬、不另起名**（✗ 不造第三个互不认识的名字）。
//		② 缺省值 = py 默认推导**逐字同构**：
//		   py  `devdocs_root(repo_root) = abspath($ZERG_DEVDOCS_ROOT or
//		        join(abspath($ZERG_DOCS_ALT or $ZERG_DOCS or join(dirname(abspath(repo_root)),
//	       "Zerg-内部文档")), "项目文档", "v2.5.10"))`
//		   Go  `chglDevdocsRoot(root)` 同名同序逐字（空串 env 视同未设，同 py 的 `or` 短路）。
//		   两串均**不含 `..` 段** ⇒ `filepath.Join` 的 normalizer 无机会吃掉东西，但本件仍走自实现
//		   `chglJoin`（posixpath.join 语义）以逐字对齐 py 的打印串。
//		③ ★ 退 2 边界：py 的**三条**取不到输入分支（主表不在/表头不符/零数据行 · 语料 --paths 全取不到
//		   ⇒ 扫到 0 篇 · devdocs 根不在盘）**逐字照搬**退 2（都是「这一格取不到 ⇒ 不给结论」，
//		   ✗ 不是「任一外部树取不到就退 2」）。**读不了的件**亦逐字退 2（py 该档本就是 rc=2 ·
//		   不是本次抬档）。本件**未**把任一条扩成红 · ✗ 不静默绿。
//		④ 取不到一律**不给结论**（rc=2），✗ 不读成「0 件＝干净」。
//
// ★ 目录遍历序（本批最重要标准）：py `walk_files` 用 `os.walk` ⇒ 内部 `os.scandir` 的
// **readdir 原序**（未排序）。Go `os.ReadDir` / `filepath.WalkDir` / `filepath.Walk` **会排序** ✗
// ⇒ 本件一律用 `f.ReadDir(-1)`（保留原序）逐层递归，**逐字复刻 os.walk 的产出序**
// （先当前层文件（按 entry 序）· 再按 dn 序下钻子目录 · 名过滤 node_modules/.git ·
//
//	followlinks=False 不下钻符号链接）。整批命中明细的先后次序由它决定 ⇒ 差排会造成整批假失配。
//
// 本单边界（照实）：本件**只落码、不接线** ——
//
//	· **不**在 `core/cmd/zerg/main.go` 里注册命令（✗ 不进命令树 · ✗ 不改 `add_step`）；
//	· **不**删 `scripts/gates/check-glossary.py` 原件（退场规则：**对拍通过后才**退场）；
//	· 入口 `checkGlossaryGateMain` 目前**无调用点**（M2 的刻意状态）。
//
// ★ 两条标准声明行：
//
//	(a) **默认根定位差异**：py `ROOT = abspath(join(dirname(abspath(__file__)), "..", ".."))` ⇒
//	    默认根起于**脚本所在目录**（与 cwd 无关）；本件**无 `__file__` 语境** ⇒ 从 **cwd** 上溯 ≤8 层
//	    找含 `core/cmd/zerg/main.go` 的目录、找不到回落 `abspath(cwd)`。**旋钮缺省 vs py 默认推导逐字对比**：
//	      py  `ROOT`（脚本位置上溯两层）⟷ Go `chglDefaultRepo()`（cwd 上溯找 `core/cmd/zerg/main.go`；回落 abspath(cwd)）
//	      py  `MAIN = join(ROOT, "docs", "术语表.tsv")` ⟷ Go `chglJoin(chglRepoRoot(), "docs", "术语表.tsv")`
//	      py  `devdocs_root(ROOT)` ⟷ Go `chglDevdocsRoot(chglRepoRoot())`（同名同序逐字）
//	    「脚本位置 == cwd 上溯根」时两者读数逐字相同（对拍甲/乙两环境已证）；否则**路径推导面不同**
//	    ⇒ 不属逐字对齐承诺。
//	(b) **自指检查（先真判 · 四形态逐条）**：
//	    ① **本件在自己扫描面内吗 ⇒ 否**。本门扫面 = 两棵语料树里的 **`.md` 件**（`walk_files` 只收
//	       `f.endswith(".md")`）。本件是 `.go`、落 `core/cmd/zerg/`。
//	       **量化依据**：`walk_files` 内 `endswith(".md")` 过滤 1 处；本件零 `.md` 路径字面量。
//	    ② **本件落在任何他门扫描面内吗 ⇒ 是**（`check-hardcoded-private-paths.py` 的 SCAN_DIRS 含
//	       `core/`；`check-error-kinds` 的 R3 扫 `core/cmd/zerg/*.go`）⇒ 按纪律处置：
//	       · 正文**零私有绝对路径字面量**（盘根绝对路径、家目录前缀一概不写；真仓根/外部树一律
//	         运行时取；需要举例时拆开拼串）；
//	       · **不写** `check-error-kinds` 的被扫错误构造调用原串；
//	       · **不写**反引号包裹的门扫描标记原串（`check-keep-marks` 一类）；
//	       · self-test 夹具里的合成词条（`虫茧`/`虫巢` 一族）只在**运行时建在 TMPDIR 下**的
//	         夹具件正文里，✗ 不落本件正文的连续被扫形态。
//	    ③ **证明处理有效**：落仓前后跑上述他门同一命令，命中数与输出 sha16 **逐字不变**（见交付读数）。
//	    ④ 本件**不接线** ⇒ 不进任何命令树/用法面 ⇒ 无「他门用法面」新增字面量。
//
// py 原件真口子（★ 不自行修 · 逐字保留 · **交回入账**）：
//
//	① `--self-test` 用 `tempfile.mkdtemp` 且**从不回收**（每跑一次自检，系统临时目录多一个
//	   `glossary-selftest-*` 残留）—— 本件 Go 侧用 `defer os.RemoveAll` 回收（**输出不可见差异**：
//	   py 只打 `  <名> ⇒ rc=N` 与末行，均不含夹具路径 ⇒ 两态 stdout 逐字同）。
//	② `--self-test` 的子进程层用 `sys.executable SELF_PATH` **真起 py 子进程**；本件 Go 侧**无此语境**，
//	   改为**进程内**以同一 argv 调本入口 ⇒ 输出行文案与 rc 逐字同（见未覆盖面①）。
//	③ `main()` 最外层**无兜底**：py 的 argparse 由本件**手工复刻**（py 原件用的是裸 argv 扫描，
//	   不是 argparse；本件逐条照搬其手写扫描语义）。
//
// 未覆盖面（✗ 不声称逐字对齐）：
//
//	① `--self-test` 子进程层：py `subprocess.run([sys.executable, SELF_PATH, …])` ⟷ 本件进程内调用
//	   （`--self-test` 是**非确定面**：py 侧 mkdtemp 随机后缀 + 起子进程；本期以 py↔py 自证 +
//	   掩码后比取证，见交付读数）。
//	② ★ **正则语义差**：py 的 `\s` / `\S` 是 **Unicode 语义**（且含 `\v`），Go RE2 默认 **ASCII**
//	   （`\s` = `[\t\n\f\r ]`，**不含 `\v`**）。涉及面 = `RE_FM`(`\s*\n`) · `RE_ANCHOR`(`\s*`) ·
//	   `RE_HEADING`(`\s{0,3}`/`\s*`) · `RE_ALLOW`(`\s*`) 。实测差：这些位夹 **U+00A0** 或 **`\v`**
//	   时两态不同（py 认、Go 不认）⇒ 该行**少抽/多抽一条**。真仓现读两棵语料该形态**零命中**
//	   （对拍逐字同）。另 py `\b`（`RE_ALLOW` 的 `glossary-allow\b`）是 Unicode 词界，Go `\b` 是
//	   ASCII 词界 —— 仅当 `allow` 后紧跟非 ASCII 词字符时两态不同（真仓零命中）。
//	   ③ **py 非多行 `$` 匹配「串尾换行之前」· Go RE2 `$`（无 `(?m)`）只匹配串尾**：本件 `RE_HEADING`
//	   的 `$` 作用于**单行**（`body.split("\n")`）⇒ 该行不含 `\n` ⇒ 两态逐字同（不属差异面）。
//	④ py `str.strip()[:60]` 是按**码点**切片，本件 `chglRuneSlice` 同样按码点（对齐）。
//	⑤ 读文件失败正文：py 打 `✗ 读不了 <abs>：<OSError 串>` ⟷ 本件打 Go `err` 串（正文自定、不属对齐）。
//	⑥ **非 UTF-8 角落**：py `open(…, errors="replace")` 把非法字节换成 U+FFFD；本件 `string(b)` 保留原字节，
//	   RE2 按 rune 解码遇非法字节得 U+FFFD ⇒ 多数形态同，粒度角落可能不同（真仓语料全 UTF-8 · 零命中）。
//	⑦ `wb()` 的 py `\w`（Unicode 词字符）本件以 `unicode.IsLetter||IsDigit||IsNumber||'_'||Mn/Mc/200C/200D`
//	   近似 ⇒ 极少数 Unicode 类别（如 `\u00AA` 序数词）边界判定可能与 py 不同（真仓语料零命中）。
//
// `--evidence GAP-20261001-13`（在册 M2 承接 · 先已回读 · 语义对口：want_family=gate / want_action=check）。
package main

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

const chglMainRel = "core/cmd/zerg/main.go"
const chglGlossaryRel = "docs/术语表.tsv"

const chglRCBlocked = 2

// chglCols —— 表头唯一真源（与 py COLS 逐字同）。
var chglCols = []string{"zh", "en", "id", "do_not_translate", "forbid_zh", "forbid_en", "owner", "note"}

// chglWhitelist —— 词表文件自身与两个脚本（否则规则文本自己就红）。
var chglWhitelist = []string{
	"docs/术语表.tsv", "docs/术语表.md", "docs/术语表.jieba.txt", "docs/术语表.forbid.txt",
	"docs/术语表-说明.md", "scripts/gates/check-glossary.py", "scripts/docs/gen-glossary-exports.py",
}

// chglPrivateExact —— 「术语」（标题文本恰为此值即命中 W5）。
const chglPrivateExact = "术语"

// ── 抽取正则（逐字复刻 py；★ ASCII/Unicode 语义差见件头未覆盖面②）───────────────────────────
var (
	chglREAllow           = regexp.MustCompile(`<!--\s*glossary-allow\b`)
	chglREComment         = regexp.MustCompile(`(?s)<!--.*?-->`)
	chglREFence           = regexp.MustCompile("(?s)(?m)^```.*?^```")
	chglRECode            = regexp.MustCompile("`([^`\n]+)`")
	chglREFM              = regexp.MustCompile(`(?s)^---\s*\n(.*?)\n---\s*\n`)
	chglCJK               = regexp.MustCompile(`[\x{3400}-\x{9fff}]`)
	chglREAnchor          = regexp.MustCompile(`\{:\s*#[^}]*\}`)
	chglREHeading         = regexp.MustCompile(`^\s{0,3}(#{1,4})\s*(.+?)\s*#*\s*$`)
	chglREPrivateGlossary = regexp.MustCompile(`术语表`)
)

// chglUsage —— py docstring「用法」段逐字（唯一真源 = 本件此串；⟷ py usage_help()）。
// 含反引号（“devdocs_root()`；“ 那个），故用解释串（\n 转义）而非原始串。
var chglUsage = "用法：\n  python3 scripts/gates/check-glossary.py [--lang zh,en] [--paths P …] [--enforce] [--no-identifiers] [--selftest]\n  python3 scripts/gates/check-glossary.py --scope devdocs [--root <目录>]   # D3①：扫开发文档面（Zerg-内部文档）\n     --enforce        把 T2 术语档升为阻塞（收敛后的口径）\n     --no-identifiers 关掉 B5/W4（大型树上想只看词表命中时用）\n     --scope devdocs  换到**开发文档面**（D3 第一步 · 默认根见 devdocs_root()`；语料一律按**中文面**判）\n     --root <目录>    换扫描根（缺省 = 本脚本的上一级 = 主仓根；**默认行为一字不变**）"

// ══ posixpath 语义小工具（逐字复刻 py 的 os.path.*）══════════════════════════════════════

func chglSplitPath(p string) (string, string) {
	i := strings.LastIndex(p, "/") + 1
	return p[:i], p[i:]
}

func chglDir(p string) string {
	head, _ := chglSplitPath(p)
	if head == "" {
		return ""
	}
	trimmed := strings.TrimRight(head, "/")
	if trimmed == "" {
		return "/"
	}
	return trimmed
}

func chglJoin(a string, parts ...string) string {
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

func chglNormpath(p string) string {
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

func chglAbs(p string) string {
	if strings.HasPrefix(p, "/") {
		return chglNormpath(p)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return chglNormpath(p)
	}
	return chglNormpath(chglJoin(cwd, p))
}

func chglRelpath(path, start string) string {
	startAbs := chglAbs(start)
	pathAbs := chglAbs(path)
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

func chglIsFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func chglIsDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

// ══ py str 语义小工具 ════════════════════════════════════════════════════════════════════

// chglRuneSlice —— py `s[:n]` 按码点切片。
func chglRuneSlice(s string, n int) string {
	rs := []rune(s)
	if len(rs) > n {
		rs = rs[:n]
	}
	return string(rs)
}

// chglRuneRStrip —— py `s.rstrip(cutset)` 按码点去尾。
func chglRuneRStrip(s, cutset string) string {
	rs := []rune(s)
	for len(rs) > 0 && strings.ContainsRune(cutset, rs[len(rs)-1]) {
		rs = rs[:len(rs)-1]
	}
	return string(rs)
}

// chglLStrip —— py `s.lstrip()`（Unicode 空白）。
func chglLStrip(s string) string {
	return strings.TrimLeftFunc(s, unicode.IsSpace)
}

func chglIsASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

// chglPyIsLower —— py `str.islower()`：至少一个 cased 字符且全为小写。
func chglPyIsLower(s string) bool {
	hasCased := false
	for _, r := range s {
		if unicode.IsUpper(r) || unicode.IsTitle(r) {
			return false
		}
		if unicode.IsLower(r) {
			hasCased = true
		}
	}
	return hasCased
}

// chglPyIsAlpha —— py `str.isalpha()`：非空且全为字母。
func chglPyIsAlpha(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !unicode.IsLetter(r) {
			return false
		}
	}
	return true
}

// chglPyCapitalize —— py `str.capitalize()`：首字符大写、其余小写。
func chglPyCapitalize(s string) string {
	rs := []rune(s)
	if len(rs) == 0 {
		return s
	}
	out := []rune{unicode.ToUpper(rs[0])}
	for _, r := range rs[1:] {
		out = append(out, unicode.ToLower(r))
	}
	return string(out)
}

// chglPyRepr —— py `%r`（str 形 · 单引号；含单引号则双引号）。
func chglPyRepr(s string) string {
	if strings.Contains(s, "'") && !strings.Contains(s, `"`) {
		return `"` + s + `"`
	}
	return "'" + strings.ReplaceAll(s, "'", `\'`) + "'"
}

// ══ 词界匹配（逐字复刻 py wb()）══════════════════════════════════════════════════════════

// chglPyWord —— py `\w` 的近似（Unicode 词字符）。
func chglPyWord(r rune) bool {
	if r == '_' || r == 0x200C || r == 0x200D {
		return true
	}
	return unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsNumber(r) ||
		unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Mc, r)
}

// chglLeftOK —— 复刻 `(?<![\w\-/.])`：前一个字符不是词字符，也不是 `-` `/` `.`。
func chglLeftOK(s string, pos int) bool {
	if pos == 0 {
		return true
	}
	r, _ := utf8.DecodeLastRuneInString(s[:pos])
	if r == '-' || r == '/' || r == '.' {
		return false
	}
	return !chglPyWord(r)
}

// chglRightOK —— 复刻 `(?![A-Za-z0-9_\-])`（ASCII 类）：后一个字符不在类内。
func chglRightOK(s string, end int) bool {
	if end >= len(s) {
		return true
	}
	b := s[end]
	if b >= 0x80 {
		return true
	}
	return !((b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_' || b == '-')
}

// chglWordHits —— py `wb(word).search(line)` 的真值：CJK 词 ⇒ 纯子串；ASCII/其它词 ⇒ 手工词界。
func chglWordHits(line, word string) bool {
	if word == "" {
		return false
	}
	if chglCJK.MatchString(word) {
		return strings.Contains(line, word)
	}
	idx := 0
	for idx <= len(line) {
		j := strings.Index(line[idx:], word)
		if j < 0 {
			return false
		}
		pos := idx + j
		if chglLeftOK(line, pos) && chglRightOK(line, pos+len(word)) {
			return true
		}
		idx = pos + 1
	}
	return false
}

// ══ 清洗 ═════════════════════════════════════════════════════════════════════════════════

func chglNLRep(m string) string { return strings.Repeat("\n", strings.Count(m, "\n")) }

func chglStripComments(text string) string {
	return chglREComment.ReplaceAllStringFunc(text, chglNLRep)
}

func chglStripFences(text string) string {
	return chglREFence.ReplaceAllStringFunc(text, chglNLRep)
}

// chglParseFM —— py parse_frontmatter（RE_FM.match ⇒ 锚在串首）。
func chglParseFM(text string) map[string]string {
	fm := map[string]string{}
	m := chglREFM.FindStringSubmatch(text)
	if m == nil {
		return fm
	}
	for _, l := range strings.Split(m[1], "\n") {
		if strings.Contains(l, ":") && !strings.HasPrefix(chglLStrip(l), "#") {
			k, v, _ := strings.Cut(l, ":")
			v = strings.TrimSpace(v)
			v = strings.Trim(v, `"`)
			v = strings.Trim(v, "'")
			fm[strings.TrimSpace(k)] = v
		}
	}
	return fm
}

// chglSplitMulti —— py split_multi：`|` 分隔 · strip · 保序去重。
func chglSplitMulti(v string) []string {
	out := []string{}
	for _, x := range strings.Split(v, "|") {
		x = strings.TrimSpace(x)
		if x != "" && !chglIn(out, x) {
			out = append(out, x)
		}
	}
	return out
}

func chglIn(xs []string, x string) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}

func chglHead(xs []string, n int) []string {
	if len(xs) > n {
		return xs[:n]
	}
	return xs
}

func chglSortedKeys(m map[string]bool) []string {
	out := []string{}
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ══ 词表 ═════════════════════════════════════════════════════════════════════════════════

// chglLoadGlossary —— py load_glossary（唯一换表口 = `ZERG_GLOSSARY_TSV`；否则 MAIN = ROOT/docs/术语表.tsv）。
func chglLoadGlossary() ([]map[string]string, error) {
	path := os.Getenv("ZERG_GLOSSARY_TSV")
	if path == "" {
		path = chglJoin(chglRepoRoot(), chglGlossaryRel)
	}
	if !chglIsFile(path) {
		return nil, fmt.Errorf("术语表主表不存在：%s", path)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	rows := []map[string]string{}
	header := []string(nil)
	for i, line := range strings.Split(string(b), "\n") {
		n := i + 1
		if n == 1 {
			header = strings.Split(strings.TrimRight(line, "\r"), "\t")
			if !chglEqStrs(header, chglCols) {
				return nil, fmt.Errorf("表头不符：%s", chglPyListRepr(header))
			}
			continue
		}
		if strings.TrimSpace(line) == "" {
			continue
		}
		f := strings.Split(strings.TrimRight(line, "\r"), "\t")
		if len(f) != len(chglCols) || f[0] == "" || strings.TrimSpace(f[0]) == "" {
			return nil, fmt.Errorf("第 %d 行不合法（字段数 %d / zh 空）", n, len(f))
		}
		row := map[string]string{}
		for k, c := range chglCols {
			row[c] = f[k]
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("主表零数据行")
	}
	return rows, nil
}

func chglEqStrs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// chglPyListRepr —— py list[str] 的 `%s`（repr）形：['a', 'b']。
func chglPyListRepr(xs []string) string {
	parts := make([]string, 0, len(xs))
	for _, x := range xs {
		parts = append(parts, chglPyRepr(x))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// ══ 单页判定 ═════════════════════════════════════════════════════════════════════════════

type chglPair struct{ w, canon string }

type chglFile struct{ rel, abs string }

// chglScanPage —— py scan_page 的真值复刻。返回 (blocking=T1, terms=T2, warnings=T3, exempted)。
func chglScanPage(rel, text string, gloss []map[string]string, lang string,
	identifiers bool, exists func(string) bool) ([]string, []string, []string, int) {

	blocking, terms, warnings := []string{}, []string{}, []string{}
	lines := strings.Split(text, "\n")
	var scanLines []string
	if lang == "en" {
		scanLines = strings.Split(chglStripComments(text), "\n")
	} else {
		scanLines = lines
	}
	stripped := chglStripComments(text)
	body := chglStripFences(stripped)
	fm := chglParseFM(text)

	pairsZh, pairsEn := []chglPair{}, []chglPair{}
	for _, r := range gloss {
		for _, w := range chglSplitMulti(r["forbid_zh"]) {
			if !chglPairHas(pairsZh, w) {
				pairsZh = append(pairsZh, chglPair{w, r["zh"]})
			}
		}
		for _, w := range chglSplitMulti(r["forbid_en"]) {
			if !chglPairHas(pairsEn, w) {
				pairsEn = append(pairsEn, chglPair{w, r["zh"]})
			}
		}
	}
	enEmpty := []map[string]string{}
	for _, r := range gloss {
		if strings.TrimSpace(r["en"]) == "" {
			enEmpty = append(enEmpty, r)
		}
	}

	hits := func(pairs []chglPair, target *[]string, tag string) int {
		n := 0
		for i, l := range lines {
			targetLine := ""
			if i < len(scanLines) {
				targetLine = scanLines[i]
			}
			for _, pr := range pairs {
				if chglWordHits(targetLine, pr.w) {
					if chglREAllow.MatchString(l) {
						n++
						continue
					}
					*target = append(*target, fmt.Sprintf("%s:%d  [%s]  %s  ← 正名「%s」",
						rel, i+1, tag, chglRuneSlice(strings.TrimSpace(targetLine), 60), pr.canon))
				}
			}
		}
		return n
	}

	if lang == "zh" {
		hits(pairsZh, &terms, "T2 禁用中文写法")
		hits(pairsEn, &terms, "T2 中英混写")
	} else {
		hits(pairsEn, &terms, "T2 禁用英文写法")
		for _, r := range enEmpty {
			if chglWordHits(body, r["zh"]) {
				blocking = append(blocking, fmt.Sprintf("%s  [T1 B3 en 列为空不得进译稿]  「%s」  ← %s",
					rel, r["zh"], chglRuneSlice(r["note"], 40)))
			}
		}
		isTranslation := !strings.HasSuffix(rel, "README.md")
		if isTranslation {
			miss := []string{}
			for _, k := range []string{"translation_of", "ai_assisted", "translation_status"} {
				if fm[k] == "" {
					miss = append(miss, k)
				}
			}
			if len(miss) > 0 {
				blocking = append(blocking, fmt.Sprintf("%s  [T1 B4 译页三必填缺 %s]", rel, strings.Join(miss, "/")))
			}
			if fm["translation_of"] != "" && !exists(fm["translation_of"]) {
				warnings = append(warnings, fmt.Sprintf("%s  [T3 W3 translation_of 指向不存在的源页]  %s",
					rel, fm["translation_of"]))
			}
		}
		if identifiers && isTranslation {
			bodyIDs := chglSetOfCode(chglStripFences(stripped))
			srcIDs := map[string]bool{}
			for _, mm := range chglREComment.FindAllString(text, -1) {
				for _, id := range chglCodeFindAll(mm) {
					srcIDs[id] = true
				}
			}
			if len(srcIDs) == 0 {
				warnings = append(warnings, fmt.Sprintf("%s  [T3 W4 无源文注释，零改动无法机械判]", rel))
			} else if !chglSameSet(bodyIDs, srcIDs) {
				missing := chglSortedDiff(srcIDs, bodyIDs)
				extra := chglSortedDiff(bodyIDs, srcIDs)
				if len(missing) > 0 {
					blocking = append(blocking, fmt.Sprintf("%s  [T1 B5 标识符被改动/丢失 %d 个]  %s",
						rel, len(missing), strings.Join(chglHead(missing, 5), "、")))
				}
				if len(extra) > 0 {
					blocking = append(blocking, fmt.Sprintf("%s  [T1 B5 正文多出源页没有的代码标识符 %d 个]  %s",
						rel, len(extra), strings.Join(chglHead(extra, 5), "、")))
				}
			}
		}
		for _, r := range gloss {
			e := strings.TrimSpace(r["en"])
			if e != "" && chglIsASCII(e) && chglPyIsLower(e) && !strings.Contains(e, " ") && chglPyIsAlpha(e) {
				if chglWordHits(body, chglPyCapitalize(e)) && !chglWordHits(body, e) {
					warnings = append(warnings, fmt.Sprintf("%s  [T3 W2 只有大小写变体 %s，未见正名 %s]",
						rel, chglPyCapitalize(e), e))
				}
			}
		}
	}
	for i, l := range strings.Split(body, "\n") {
		m := chglREHeading.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		title := chglREAnchor.ReplaceAllString(m[2], "")
		title = strings.TrimSpace(title)
		title = strings.Trim(title, "*_")
		title = strings.TrimSpace(title)
		title = chglRuneRStrip(title, "：:。.、，,")
		if !(chglREPrivateGlossary.MatchString(title) || title == chglPrivateExact) {
			continue
		}
		allowLine := ""
		if i < len(lines) {
			allowLine = lines[i]
		}
		if chglREAllow.MatchString(allowLine) {
			continue
		}
		warnings = append(warnings, fmt.Sprintf("%s:%d  [T3 W5 正文私建术语表]  %s  ← 全局术语只有一份（docs/术语表.tsv）：全局 ⇒ 进主表；本文专有 ⇒ 改名（如「本文概念/字段说明」）",
			rel, i+1, chglRuneSlice(strings.TrimSpace(chglREAnchor.ReplaceAllString(l, "")), 60)))
	}
	exempted := 0
	for _, l := range lines {
		if chglREAllow.MatchString(l) {
			exempted++
		}
	}
	return blocking, terms, warnings, exempted
}

func chglPairHas(ps []chglPair, w string) bool {
	for _, p := range ps {
		if p.w == w {
			return true
		}
	}
	return false
}

func chglCodeFindAll(s string) []string {
	out := []string{}
	for _, m := range chglRECode.FindAllStringSubmatch(s, -1) {
		out = append(out, m[1])
	}
	return out
}

func chglSetOfCode(s string) map[string]bool {
	out := map[string]bool{}
	for _, id := range chglCodeFindAll(s) {
		out[id] = true
	}
	return out
}

func chglSameSet(a, b map[string]bool) bool {
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

// chglSortedDiff —— py `sorted(a - b)`。
func chglSortedDiff(a, b map[string]bool) []string {
	out := []string{}
	for k := range a {
		if !b[k] {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// ══ 语料遍历（★ 复刻 os.walk：readdir 原序 · 深度优先 · 不下钻符号链接）════════════════════

func chglLangOf(rel, scope string) string {
	if scope == "devdocs" {
		return "zh"
	}
	parts := strings.Split(rel, "/")
	if len(parts) >= 2 && parts[1] == "zh" {
		return "zh"
	}
	return "en"
}

func chglWalkFiles(paths []string, root string, langs map[string]bool, scope string) []chglFile {
	out := []chglFile{}
	for _, p := range paths {
		ap := p
		if !strings.HasPrefix(p, "/") {
			ap = chglJoin(root, p)
		}
		if chglIsFile(ap) {
			out = append(out, chglFile{chglRelpath(ap, root), ap})
			continue
		}
		if !chglIsDir(ap) {
			continue
		}
		chglWalkDir(ap, root, langs, scope, &out)
	}
	return out
}

// chglWalkDir —— ★ 保留原序读法：`f.ReadDir(-1)`（**不用**会排序的 os.ReadDir/WalkDir）。
func chglWalkDir(dir, root string, langs map[string]bool, scope string, out *[]chglFile) {
	f, err := os.Open(dir)
	if err != nil {
		return
	}
	entries, err := f.ReadDir(-1) // 未排序：与 py os.scandir/readdir 同序
	f.Close()
	if err != nil {
		return
	}
	dirs := []string{}
	for _, e := range entries {
		p := chglJoin(dir, e.Name())
		st, serr := os.Stat(p)
		if serr == nil && st.IsDir() {
			if e.Name() == "node_modules" || e.Name() == ".git" {
				continue
			}
			if lst, lerr := os.Lstat(p); lerr == nil && lst.Mode()&os.ModeSymlink != 0 {
				continue // os.walk(followlinks=False)：不下钻符号链接
			}
			dirs = append(dirs, p)
		} else if strings.HasSuffix(e.Name(), ".md") {
			rel := chglRelpath(p, root)
			if langs[chglLangOf(rel, scope)] {
				*out = append(*out, chglFile{rel, p})
			}
		}
	}
	for _, d := range dirs {
		chglWalkDir(d, root, langs, scope, out)
	}
}

// ══ 默认根 ═══════════════════════════════════════════════════════════════════════════════

// chglRepoRoot —— Go 侧无 `__file__` 语境：从 cwd 上溯找含 core/cmd/zerg/main.go 的目录（见 (a)）。
func chglRepoRoot() string {
	cwd, err := os.Getwd()
	if err != nil {
		return chglNormpath(".")
	}
	d := cwd
	for i := 0; i < 8; i++ {
		if chglIsFile(chglJoin(d, chglMainRel)) {
			return chglNormpath(d)
		}
		parent := chglDir(d)
		if parent == d || parent == "" {
			break
		}
		d = parent
	}
	return chglNormpath(cwd)
}

// chglDevdocsRoot —— 开发文档面根（与 py devdocs_root 同名同序逐字）。
func chglDevdocsRoot(repoRoot string) string {
	if v := os.Getenv("ZERG_DEVDOCS_ROOT"); v != "" {
		return chglAbs(v)
	}
	sibling := os.Getenv("ZERG_DOCS_ALT")
	if sibling == "" {
		sibling = os.Getenv("ZERG_DOCS")
	}
	if sibling == "" {
		sibling = chglJoin(chglDir(chglAbs(repoRoot)), "Zerg-内部文档")
	}
	return chglAbs(chglJoin(sibling, "项目文档", "v2.5.10"))
}

func chglPyBool(b bool) string {
	if b {
		return "True"
	}
	return "False"
}

// ══ 入口 ═════════════════════════════════════════════════════════════════════════════════

func chglHas(argv []string, x string) bool {
	for _, a := range argv {
		if a == x {
			return true
		}
	}
	return false
}

// chglRun —— 真跑（不含 Q-176：--self-test 的档③′ 需要它照常拦）。
func chglRun(stdout, stderr io.Writer, argv []string) int {
	known := map[string]bool{
		"--root": true, "--scope": true, "--lang": true, "--paths": true,
		"--self-test": true, "--selftest": true, "--enforce": true, "--json": true,
		"--no-identifiers": true, "--help": true, "-h": true,
	}
	unknown := []string{}
	for _, a := range argv {
		if strings.HasPrefix(a, "-") && !known[a] {
			unknown = append(unknown, a)
		}
	}
	if len(unknown) > 0 {
		fmt.Fprintf(stderr, "✗ 未知参数: %s\n", strings.Join(unknown, " "))
		return chglRCBlocked
	}
	if chglHas(argv, "--help") || chglHas(argv, "-h") {
		fmt.Fprint(stdout, chglUsage+"\n")
		return 0
	}
	if chglHas(argv, "--self-test") || chglHas(argv, "--selftest") {
		return chglSelfTest(stdout)
	}
	root := chglRepoRoot()
	scope := ""
	langs := map[string]bool{}
	paths := []string{}
	i := 0
	for i < len(argv) {
		if argv[i] == "--root" && i+1 < len(argv) {
			root = chglAbs(argv[i+1])
			i += 2
			continue
		}
		if argv[i] == "--scope" && i+1 < len(argv) {
			scope = argv[i+1]
			if scope != "docs" && scope != "devdocs" {
				fmt.Fprintf(stderr, "✗ 未知 --scope %s（表内只有 docs / devdocs）⇒ 不给结论\n", chglPyRepr(scope))
				return chglRCBlocked
			}
			i += 2
			continue
		}
		if argv[i] == "--lang" && i+1 < len(argv) {
			for _, x := range strings.Split(argv[i+1], ",") {
				if x == "zh" || x == "en" {
					langs[x] = true
				}
			}
			i += 2
			continue
		}
		if argv[i] == "--paths" {
			i++
			for i < len(argv) && !strings.HasPrefix(argv[i], "--") {
				paths = append(paths, argv[i])
				i++
			}
			continue
		}
		i++
	}
	if scope == "devdocs" && !chglHas(argv, "--root") {
		root = chglDevdocsRoot(chglRepoRoot())
		if !chglIsDir(root) {
			fmt.Fprintf(stderr, "✗ 开发文档面根不在（%s）⇒ 不给结论（rc=2 · 仓外不可达不许静默当绿）\n", root)
			return chglRCBlocked
		}
		langs = map[string]bool{"zh": true}
		if len(paths) == 0 {
			paths = []string{root}
		}
	}
	if len(langs) == 0 {
		langs = map[string]bool{"zh": true, "en": true}
	}
	if len(paths) == 0 {
		for _, l := range chglSortedKeys(langs) {
			paths = append(paths, chglJoin("docs", l))
		}
	}

	gloss, err := chglLoadGlossary()
	if err != nil {
		fmt.Fprintf(stderr, "✗ %s\n", err)
		return chglRCBlocked
	}
	files := chglWalkFiles(paths, root, langs, scope)
	if len(files) == 0 {
		fmt.Fprint(stderr, "✗ 扫到 0 个 .md —— 拒绝静默空转（假绿）。检查 --paths 与 docs/zh|docs/en 是否存在\n")
		return chglRCBlocked
	}

	blocking, terms, warnings := []string{}, []string{}, []string{}
	exempt := 0
	identifiers := !chglHas(argv, "--no-identifiers")
	enforce := chglHas(argv, "--enforce")
	for _, f := range files {
		if chglIn(chglWhitelist, f.rel) {
			continue
		}
		b, err := os.ReadFile(f.abs)
		if err != nil {
			fmt.Fprintf(stderr, "✗ 读不了 %s：%s\n", f.abs, err)
			return chglRCBlocked
		}
		lang := chglLangOf(f.rel, scope)
		bb, tt, ww, x := chglScanPage(f.rel, string(b), gloss, lang, identifiers,
			func(p string) bool { return chglIsFile(chglJoin(root, p)) })
		blocking = append(blocking, bb...)
		terms = append(terms, tt...)
		warnings = append(warnings, ww...)
		exempt += x
	}

	enEmpty := 0
	for _, r := range gloss {
		if strings.TrimSpace(r["en"]) == "" {
			enEmpty++
		}
	}
	fmt.Fprintf(stdout, "术语表 %d 条（en 空 %d 条）· 扫描 %d 篇（%s；白名单 %d 篇）· enforce=%s\n",
		len(gloss), enEmpty, len(files), strings.Join(chglSortedKeys(langs), "+"), len(chglWhitelist), chglPyBool(enforce))
	bags := []struct {
		title string
		items []string
	}{
		{"T1 阻塞档（rc=2）", blocking},
		{"T2 术语档（非阻塞 · --enforce 时 rc=2）", terms},
		{"T3 告警档（非阻塞）", warnings},
	}
	for _, bag := range bags {
		fmt.Fprintf(stdout, "\n── %s：%d 处\n", bag.title, len(bag.items))
		for _, x := range chglHead(bag.items, 20) {
			fmt.Fprintf(stdout, "    %s\n", x)
		}
		if len(bag.items) > 20 {
			fmt.Fprintf(stdout, "    …另 %d 处\n", len(bag.items)-20)
		}
	}
	fmt.Fprintf(stdout, "\n合计：T1 %d · T2 %d · T3 %d · 单条豁免 %d\n",
		len(blocking), len(terms), len(warnings), exempt)
	if len(blocking) > 0 || (enforce && len(terms) > 0) {
		return 2
	}
	if len(terms) > 0 || len(warnings) > 0 {
		return 1
	}
	return 0
}

// checkGlossaryGateMain —— 真搬入口（M2 刻意无调用点）。
func checkGlossaryGateMain(stdout, stderr io.Writer, argv []string) int {
	// ── Q-176：本门无机器读面 ⇒ 给了 --json 不许沉默（先于一切）────────────────────────
	jsonHits := []string{}
	for _, x := range argv {
		if x == "--json" || strings.HasPrefix(x, "--json=") {
			jsonHits = append(jsonHits, "`"+x+"`")
		}
	}
	if len(jsonHits) > 0 {
		fmt.Fprintf(stderr, "⛔ 本门无机器读面：%s 在本门**未实现**（缺口 Q-176）\n", strings.Join(jsonHits, " · "))
		fmt.Fprint(stderr, "   字段表：无（本门只出人读面）\n")
		fmt.Fprint(stderr, "⇒ 用法错（rc=2）：**机器面缺 ≠ 零命中** —— 两态不许同形\n")
		return chglRCBlocked
	}
	return chglRun(stdout, stderr, argv)
}

// ══ 自证 ═════════════════════════════════════════════════════════════════════════════════

func chglRow(v ...string) map[string]string {
	row := map[string]string{}
	for i, c := range chglCols {
		if i < len(v) {
			row[c] = v[i]
		} else {
			row[c] = ""
		}
	}
	return row
}

func chglSelfGloss() []map[string]string {
	return []map[string]string{
		chglRow("虫茧", "cocoon", "cocoon", "cocoon", "虫巢|集装箱", "pod", "Mr2109", "只此一译"),
		chglRow("承接项", "", "", "", "", "", "Mr2109", "英文待拍"),
		chglRow("虫卵", "egg", "", "", "沙箱|容器", "sandbox|container", "Mr2109", ""),
		chglRow("主控", "core", "core", "core", "", "", "Mr2109", ""),
	}
}

const chglFmOK = "---\ntitle: T\ntranslation_of: docs/x.md\nai_assisted: ai\ntranslation_status: machine_draft\n---\n"

type chglCase struct {
	name string
	fn   func() int
	want int
}

// chglSelfPure —— py selftest()：29 条纯函数成对正负控。
func chglSelfPure(stdout io.Writer) int {
	gloss := chglSelfGloss()
	terms := func(rel, text, lang string) int {
		_, t, _, _ := chglScanPage(rel, text, gloss, lang, true, func(string) bool { return true })
		return len(t)
	}
	block := func(rel, text, lang string, ids bool, ex func(string) bool) int {
		b, _, _, _ := chglScanPage(rel, text, gloss, lang, ids, ex)
		return len(b)
	}
	warn := func(rel, text, lang string, ids bool, ex func(string) bool) int {
		_, _, w, _ := chglScanPage(rel, text, gloss, lang, ids, ex)
		return len(w)
	}
	cases := []chglCase{
		{"T2 中文页禁写 虫巢", func() int { return terms("docs/zh/a.md", "这里叫虫巢。", "zh") }, 1},
		{"T2 正名不命中", func() int { return terms("docs/zh/a.md", "这里叫虫茧。", "zh") }, 0},
		{"T2 单条豁免生效", func() int { return terms("docs/zh/a.md", "旧称虫巢。<!-- glossary-allow -->", "zh") }, 0},
		{"T2 英文页禁写 pod", func() int { return terms("docs/en/a.md", chglFmOK+"the pod runs\n", "en") }, 1},
		{"T2 英文页注释里的 pod 不算", func() int { return terms("docs/en/a.md", chglFmOK+"x\n<!-- the pod runs -->\n", "en") }, 0},
		{"T2 中文页注释里的 pod 仍算", func() int { return terms("docs/zh/a.md", "x\n<!-- the pod runs -->\n", "zh") }, 1},
		{"T2 同词多行登记只报一次", func() int { return terms("docs/en/a.md", chglFmOK+"container leak\n", "en") }, 1},
		{"T2 词界防误伤 podcast", func() int { return terms("docs/en/a.md", chglFmOK+"a podcast\n", "en") }, 0},
		{"T2 路径里的同名段不算（/pod/x）", func() int { return terms("docs/zh/a.md", "路径 `/pod/x`", "zh") }, 0},
		{"T2 中文页混写 pod", func() int { return terms("docs/zh/a.md", "一个 pod", "zh") }, 1},
		{"T1 B3 en 空词条进译稿", func() int { return block("docs/en/a.md", chglFmOK+"承接项 list\n", "en", true, chglYes) }, 1},
		{"T1 B3 注释里的中文不算", func() int { return block("docs/en/a.md", chglFmOK+"x\n<!-- 承接项 -->\n", "en", true, chglYes) }, 0},
		{"T1 B4 译页三必填", func() int { return block("docs/en/a.md", "---\ntitle: T\n---\nbody\n", "en", true, chglYes) }, 1},
		{"T1 B4 README 豁免", func() int { return block("docs/en/README.md", "body\n", "en", true, chglYes) }, 0},
		{"T1 B5 标识符集合相等", func() int {
			return block("docs/en/a.md", chglFmOK+"run `doc_search`\n<!-- 跑 `doc_search` -->\n", "en", true, chglYes)
		}, 0},
		{"T1 B5 标识符被译（丢 1 多 1）", func() int {
			return block("docs/en/a.md", chglFmOK+"run `docSearch`\n<!-- 跑 `doc_search` -->\n", "en", true, chglYes)
		}, 2},
		{"T3 W2 只有大小写变体", func() int { return warn("docs/en/a.md", chglFmOK+"Cocoon is here\n", "en", false, chglYes) }, 1},
		{"T3 W3 translation_of 悬空", func() int { return warn("docs/en/a.md", chglFmOK+"x\n", "en", false, chglNo) }, 1},
		{"T3 W4 无源文注释", func() int { return warn("docs/en/a.md", chglFmOK+"x\n", "en", true, chglYes) }, 1},
		{"T3 W5 正文私建术语表（## 术语表）",
			func() int { return warn("docs/zh/a.md", "## 术语表\n", "zh", true, chglYes) }, 1},
		{"T3 W5 现成落点 ## 术语 {: #术语 }（改名前的形态）",
			func() int { return warn("docs/zh/a.md", "## 术语 {: #术语 }\n", "zh", true, chglYes) }, 1},
		{"T3 W5 带前缀的术语表标题（### 中英术语表）",
			func() int { return warn("docs/zh/a.md", "### 中英术语表\n", "zh", true, chglYes) }, 1},
		{"T3 W5 改名后不命中（## 本文概念/字段说明 {: #术语 }）",
			func() int {
				return warn("docs/zh/a.md", "## 本文概念/字段说明 {: #术语 }\n", "zh", true, chglYes)
			}, 0},
		{"T3 W5 锚点里的「术语表」不算（引用，不是私建）",
			func() int {
				return warn("docs/en/a.md", chglFmOK+"# Glossary: one master table {: #术语表：一份主表 }\n", "en", false, chglYes)
			}, 0},
		{"T3 W5 围栏代码块里的 ## 术语表 不算（示例）",
			func() int { return warn("docs/zh/a.md", "```\n## 术语表\n```\n", "zh", true, chglYes) }, 0},
		{"T3 W5 单条豁免生效（<!-- glossary-allow -->）",
			func() int { return warn("docs/zh/a.md", "## 术语表 <!-- glossary-allow -->\n", "zh", true, chglYes) }, 0},
		{"T3 W5 含「术语」但不恰为（## 术语与字段说明）不命中",
			func() int { return warn("docs/zh/a.md", "## 术语与字段说明\n", "zh", true, chglYes) }, 0},
		{"D3① devdocs：`01-设计/x.md` 按中文面判（禁写中文写法要命中）",
			func() int {
				if chglLangOf("01-设计/x.md", "devdocs") == "zh" {
					return 1
				}
				return 0
			}, 1},
		{"D3① 默认口径：同路径第二段不是 zh ⇒ 英文面（**这是那 93 处假红的来源**）",
			func() int {
				if chglLangOf("01-设计/x.md", "") == "en" {
					return 1
				}
				return 0
			}, 1},
	}
	fmt.Fprintf(stdout, "── check-glossary.py 自证（29 条）\n")
	ok := true
	for _, c := range cases {
		got, gotOK := chglSafeLen(c.fn)
		good := gotOK && got == c.want
		ok = ok && good
		mark := "✓"
		if !good {
			mark = "✗"
		}
		disp := fmt.Sprintf("%d", got)
		if !gotOK {
			disp = "异常"
		}
		fmt.Fprintf(stdout, "  %s %s（期望 %d 实际 %s）\n", mark, c.name, c.want, disp)
	}
	if ok {
		fmt.Fprintf(stdout, "  自证：通过 ✓\n")
		return 0
	}
	fmt.Fprintf(stdout, "  自证：**失败** ✗\n")
	return 1
}

func chglYes(string) bool { return true }
func chglNo(string) bool  { return false }

func chglSafeLen(fn func() int) (n int, ok bool) {
	ok = true
	defer func() {
		if r := recover(); r != nil {
			ok = false
		}
	}()
	n = fn()
	return n, ok
}

// chglSelfTest —— py self_test()：三档自检（正控 rc=0 · 负控 rc=1 · 用法错 rc=2）。
func chglSelfTest(stdout io.Writer) int {
	bad := 0
	chk := func(label string, want, rc int, out string) {
		good := rc == want
		if !good {
			bad++
		}
		tail := ""
		if !good {
			lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
			last := ""
			if len(lines) > 0 {
				last = lines[len(lines)-1]
			}
			tail = " ｜ " + chglRuneSlice(last, 90)
		}
		mark := "✓"
		if !good {
			mark = "✗"
		}
		fmt.Fprintf(stdout, "  %s %s（期望 rc=%d 实际 rc=%d）%s\n", mark, label, want, rc, tail)
	}
	fmt.Fprintf(stdout, "── check-glossary.py --self-test 三档（正控 0 · 负控 1 · 用法错 2）\n")
	fmt.Fprintf(stdout, "   ＜层① 纯函数 29 例（判据成对正负控 · 不碰任何目录）＞\n")
	if chglSelfPure(stdout) != 0 {
		bad++
	}
	tmp, err := os.MkdirTemp("", "glossary-selftest-")
	if err != nil {
		fmt.Fprintf(stdout, "── 自检：**1 档不过** ✗\n")
		return 1
	}
	defer os.RemoveAll(tmp) // py 不回收；本件回收（输出不可见差异）

	tsv := chglJoin(tmp, "术语表.tsv")
	content := strings.Join(chglCols, "\t") + "\n" +
		"虫茧\tcocoon\tcocoon\tcocoon\t\t\tMr2109\t\n" +
		"虫巢\t\t\t\t虫巢\t\tMr2109\t英文待拍\n"
	if err := os.WriteFile(tsv, []byte(content), 0o644); err != nil {
		fmt.Fprintf(stdout, "── 自检：**1 档不过** ✗\n")
		return 1
	}
	docs := chglJoin(tmp, "docs", "zh")
	os.MkdirAll(docs, 0o755)
	clean := chglJoin(docs, "clean.md")
	os.WriteFile(clean, []byte("本文只写正名：这里叫虫茧。\n"), 0o644)
	dirty := chglJoin(docs, "dirty.md")
	os.WriteFile(dirty, []byte("旧称：这里叫虫巢。\n"), 0o644)

	old, had := os.LookupEnv("ZERG_GLOSSARY_TSV")
	os.Setenv("ZERG_GLOSSARY_TSV", tsv)
	defer func() {
		if had {
			os.Setenv("ZERG_GLOSSARY_TSV", old)
		} else {
			os.Unsetenv("ZERG_GLOSSARY_TSV")
		}
	}()

	run := func(a ...string) (int, string) {
		var out, errb strings.Builder
		argv := append([]string{"--root", tmp, "--lang", "zh"}, a...)
		rc := checkGlossaryGateMain(&out, &errb, argv)
		return rc, out.String() + errb.String()
	}

	fmt.Fprintf(stdout, "   ＜层② 子进程真跑本件（合成根 + 合成主表 · 临时目录）＞\n")
	rc, out := run("--paths", clean)
	chk("档① 正控：中文页只写正名 ⇒ rc=0", 0, rc, out)
	rc, out = run("--paths", dirty)
	chk("档② 负控：中文页出现 `forbid_zh` 词（虫巢）⇒ T2 非阻塞 rc=1", 1, rc, out)
	rc, out = run("--scope", "no-such-scope", "--paths", clean)
	chk("档③ 用法错：未知 `--scope` ⇒ rc=2", 2, rc, out)
	rc, out = run("--paths", clean, "--json")
	chk("档③′ 用法错：`--json` 而本门无机器读面（`Q-176` 统一口）", 2, rc, out)
	if bad == 0 {
		fmt.Fprintf(stdout, "── 自检：全过 ✓\n")
		return 0
	}
	fmt.Fprintf(stdout, "── 自检：**%d 档不过** ✗\n", bad)
	return 1
}
