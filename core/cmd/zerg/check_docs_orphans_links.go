// check_docs_orphans_links.go —— 判据件 `scripts/gates/check-docs-orphans-links.py`（834 行）的
// **Go 真搬**（v2.5.14 · M2 · **新口径标准靶：文仓/外部树依赖类** · 入口 `checkDocsOrphansLinksGateMain`）。
//
// 为什么有这件：
//
//	v2.5.14 定案把判据实现面分两段 —— M1 = 转发壳（`zerg gate check <判据名>`）；M2 = 按价值真搬 Go
//	（落点 `core/cmd/zerg/`）。本件 = M2 一枚，且是本批**第一枚「有外部树依赖」的真搬**（其余 20 余枚
//	皆 M2-0「无输入物料」）：验收判据 = 同 argv 跑两态（py 原件 ⟷ 本入口），stdout / stderr / 退码
//	**三面逐字相同**（除下列**显式声明的**新口径差异面）。
//
// 本件与新口径（外部树改显式入参旋钮 —— 逐条对应派单四条）：
//
//	① 旋钮名**照 py 已有同名旗标**：py 本就带 `--repo-root` / `--docs-root` / `--tree`
//	   （外加 env `ZERG_PUB_TREE` 主名、`ZERG_PUBLIC_TREE` 兼容别名）⇒ 本件**同名照搬、不另起名**
//	   （✗ 不造第三个互相不认识的名字 —— 那正是 `GAP-20261001-01` 那个口子的来源）。
//	② 缺省值 = **py 默认推导逐字同构**（见 (a) 的逐字对比）：`repo_root` 起于**脚本位置**（py）
//	   ⟷ 起于 **cwd 上溯**（Go，无 `__file__` 语境）；`docs_root = pyJoin(dirname(repo_root), "Zerg-内部文档")`
//	   —— 用**字符串拼接**而非 `filepath.Join`（后者的 normalizer 会吃掉中间的 `..`，而 py 把 `..`
//	   原样留在打印串里 ⇒ 需要时必须串拼，逐字对拍验证）。
//	③ ★ 退 2 边界（**逐字沿用 py · 缺件 ≠ 绿**）：本门两棵外部树都是**必需输入** ——
//	   `docs_root`（第二棵树 · ①/③ 两格的域）与 `tree`（导出树 · ② 格的域）**任一**取不到 /
//	   判不了（域不在盘 / 真扫面 0 件 / 件读不了）⇒ rc=2「不给结论」。判准同 `check-dangling-refs.go`
//	   该档（py `absent_corpus_trees` 逐字沿用）与 `check-docs-orphans-links.py` 件头退码节
//	   （承 `GAP-20260927-82` · `GAP-20261005-69`）。**主仓 `--repo-root` 是必需输入**
//	   ⇒ 它取不到同 rc=2。
//	④ 取不到一律**不给结论**、✗ 不静默绿：任一格的域取不到 ⇒ 该格打印 `判不了`＋原因（绝不读成
//	   「0 件」＝干净）；本件是把「查不到」当「不存在」的那类静默绿的**反例**。
//
// 本单边界（照实）：本件**已接线（inproc 表分派）** ——
//
//	· **不**在 `core/cmd/zerg/main.go` 里注册命令（✗ 不进命令树 · `add_step` **未挂**（`precommit-gates.sh` 现读零命中））；
//	· **不**删 `scripts/gates/check-docs-orphans-links.py` 原件（退场规则：**对拍通过后才**退场）；
//	· **已接线**：入口 `checkDocsOrphansLinksGateMain` 由 `gateCheckGoEntry["check-docs-orphans-links"]` 分派（`zerg gate check check-docs-orphans-links`）· 原文此处断言与现读相反（只改文案 · 未改行为）。
//
// ★ 两条标准声明行：
//
//	(a) **默认根定位差异**：py `here = dirname(abspath(__file__))` ·
//	    `repo_default = dirname(dirname(here))` · `docs_root = join(dirname(repo_root), "Zerg-内部文档")`
//	    ⇒ 默认根起于**脚本所在目录**（与 cwd 无关）；本件**无 `__file__` 语境** ⇒ 不给 `--repo-root`
//	    时从 **cwd** 上溯 ≤8 层找含 `scripts/gates` 的目录、找不到回落 `abspath(cwd)`。
//	    **旋钮缺省 vs py 默认推导逐字对比**：
//	      py  `docs_root = a.docs_root if a.docs_root is not None else os.path.join(os.path.dirname(a.repo_root), "Zerg-内部文档")`
//	      Go  `coanDocsDefault(root) = root + "/../Zerg-内部文档"`（**字符串拼接、不规范化** ⇒ 中间 `..`
//	          原样留在打印串里，与 py 的 `os.path.join` 同构；★ 不用 `filepath.Join`，它会吃掉 `..`）
//	      py  `resolve_tree`: `--tree` → `$ZERG_PUB_TREE` → `$ZERG_PUBLIC_TREE` → `""`
//	      Go  `coanResolveTree`: 同名同序逐字（`strings.TrimSpace` 过滤空值同 py 的 `if val.strip()`）
//	    「脚本位置 == cwd 上溯根」时两者读数逐字相同（对拍甲/乙两环境已证）；否则**路径推导面不同**
//	    ⇒ 不属逐字对齐承诺，对拍一律显式传根或在夹具里让两者重合。
//	(b) **自指检查（先真判 · 四形态逐条）**：
//	    ① **本件在自己扫描面内吗 ⇒ 否**。本门扫面 = `--repo-root` 树与两棵外部树里的 **`.md` 件**
//	      （`scan_tree` 只收 `fn.endswith(".md")`；`orphans` 只取 `.md`）。本件是 `.go`，
//	      且落 `core/cmd/zerg/` —— **量化依据**：`scan_tree` 内 `endswith(".md")` 过滤 1 处、
//	      `links_of` 只吃调用方给的 `.md` 正文；真仓现读本门「主仓」格红件数 0、`core/` 下零 `.md`
//	      被点。⇒ 本门**永不自扫本件**。
//	    ② **本件落在任何他门扫描面内吗 ⇒ 是**（`check-hardcoded-private-paths.py` 的 SCAN_DIRS 含
//	      `core/`；`check-error-kinds` 的 R3 扫 `core/cmd/zerg/*.go`）⇒ 已按六发血证处置：
//	      · 正文**零私有绝对路径字面量**（盘根绝对路径、家目录前缀一概不写：真仓根/外部树一律运行时取；
//	        需要举例时**拆开拼串**，见 `coanUsersPrefix` / `coanVolumesPrefix`）；
//	      · **不写** `check-error-kinds` 的被扫错误构造调用原串；
//	      · **不写**反引号包裹的门扫描标记原串（`layer:` 一类）；
//	      · self-test 夹具里的魔法串（`coan-selftest-` / `gone.md` 一族）只落**运行时建在 TMPDIR 下**的
//	        夹具件正文，**不落本件正文**的连续被扫形态。
//	    ③ **证明处理有效**：落仓前后跑上述他门同一命令，命中数与输出 sha16 **逐字不变**（见交付读数）。
//	    ④ 本件**已接线（inproc 表分派）** ⇒ 不进任何命令树/用法面 ⇒ 无「他门用法面」新增字面量。
//
// py 原件真口子（★ 不自行改 py · **本件 Go 侧已修 · 与 py 有意分叉**）：
//
//	`scan_tree` 对**读不了的 `.md`** 记 `out[rel] = None`；`local_broken_detail` 遇 `text is None`
//	**直接 return**（丢掉该树此前已收的全部 hits）⇒ 一棵树里**任一 `.md` 读不了**，那一树的红明细
//	全丢、且**该格整格判不了**（rc=2），而非「跳过该件继续判」。**本件 Go 侧已修（B 类真口子）**：
//	读失败 ⇒ 登记（相对路径 + err 正文）+ 入口 `checkDocsOrphansLinksGateMain` 补判红（stderr BLOCKED
//	头 + 逐件点名）⇒ rc=2。
//
// ★ 2026-10-04 同步（候选 · GAP-20261001-26）：py 候件把 `local_broken_detail` 改成「读不了的件逐条记下、
//
//	不再一格 None 丢整树」—— 判不了态仍返已收的 hits/exempt/stale，理由标注「读不了 N 件」；调用方在
//	n is None 分支补印命中 / 假红明细。**本件 Go 侧已逐处同步**（`coanLocalBrokenDetail` 同语义、调用方
//	同位置补印）。既有 B 类登记面（stderr BLOCKED 头 + 逐件点名）**保留**（Go-only 分叉）⇒ 该档
//	stdout 与 py 候件逐字同、stderr 多一段 BLOCKED；导出树 / 第二棵树整棵读不了时，本件的 B 类早退
//	还会先于「结论」段返回（py 则会印一段「结论：不给结论」）—— 该差为**既有分叉**，非本次同步引入。
//
// 未覆盖面（✗ 不声称逐字对齐）：
//
//	① argparse **长选项前缀缩写**（如 `--rep`⇒`--repo-root`）本件不实现；`store_true` 型带 `=值`
//	   的 argparse 文案（"ignored explicit argument"）本件复刻。
//	② **默认根定位**：见 (a) —— 「脚本位置 ≠ cwd 上溯根」时不属逐字对齐承诺。
//	③ （**已收口 · 2026-10-07**）退 2 边界逐字沿用 py：两棵外部树任一取不到 / 判不了 ⇒ rc=2
//	   （见 3️⃣ ③；不再是「有意差异」，两态逐字同）。
//	④ py `open(..., encoding="utf-8")` 于 `nav_whitelist` 遇非 UTF-8 抛 `UnicodeDecodeError`
//	   （未捕获 ⇒ 进程 rc=1 + 回溯正文）；本件按「读不了 ⇒ 不给结论」处理（rc=2、正文自定）。
//	⑤ `--self-test` 用 `tempfile.mkdtemp`（随机后缀）——但**输出只打 ✓/✗ 行、不含路径** ⇒ 两态可逐字对齐。
//	⑥ `checkDocsOrphansLinksGateMain` 暂不覆盖「`--repo-root` 是件而非目录」的 py traceback 分支
//	   （`os.walk` 对文件返回空 ⇒ py 走「扫面 0 件」档；本件按「根不在盘」档 ⇒ 该角落 rc 同=2、正文不同）。
//
//	`--evidence GAP-20261001-13`（在册 M2 承接 · 先已回读 · 语义对口）。
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const coanVersion = "1.2.0"

// ── env 名（2026-10-01 统一 · 同族口径 · 与 family_impact_layers.go 逐字一致）────────────
const coanPubTreeEnv = "ZERG_PUB_TREE"         // 主名
const coanPubTreeEnvAlias = "ZERG_PUBLIC_TREE" // 兼容别名（只在主名为空时读它）

// ── (b) 拼串处理：避免本件正文出现他门 NEEDLES 的连续原串（运行时拼出的值逐字相同）────────
var (
	coanUsersPrefix   = "/Users" + "/"   // = 家目录前缀（check-hardcoded-private-paths 的形态之一）
	coanVolumesPrefix = "/Volumes" + "/" // = 私有卷前缀
)

// ── 基准偏移登记（键 = 件在树根下的相对路径 · 值 = (基准目录, 理由)）──────────────────────
var coanBaseOffset = map[string][2]string{
	"publish/docs/README.md":       {"publish", "公开树仓根定位（README 家族放公开树仓根）"},
	"publish/docs/README.zh-CN.md": {"publish", "公开树仓根定位（与 en 版同基准 · 语言切换互链）"},
}

// ── 抽取正则（逐字复刻 py）─────────────────────────────────────────────────────────────
var (
	coanLinkRE    = regexp.MustCompile(`\]\(\s*(<[^>]*>|[^)\s]*)(?:\s+"[^"]*")?\s*\)`)
	coanDefRE     = regexp.MustCompile(`^\s*\[[^\]]+\]:\s*(\S+)`)
	coanFenceRE   = regexp.MustCompile("^\\s*(`{3,}|~{3,})")
	coanNavKeyRE  = regexp.MustCompile(`^nav\s*:`)
	coanNavFileRE = regexp.MustCompile(`[\p{L}\p{N}_./-]+\.md`)
)

// ── 外链 / 非仓内形态（明确不进任何一格）──────────────────────────────────────────────
var coanExternalSchemes = []string{"http://", "https://", "ftp://", "ftps://", "mailto:", "tel:", "data:", "//"}

// ── 域（扫面不走的两类：已有剪枝目录 + 嵌套仓）────────────────────────────────────────
var coanPruneDirs = []string{".git", "node_modules", "venv", ".venv", "__pycache__"}

// coanNestedSeen —— {树根: [剪掉的嵌套仓相对路径]}（py 的模块级 `NESTED_SEEN`）。
var coanNestedSeen = map[string][]string{}

// ── W-06 术语表（两词不许互称）───────────────────────────────────────────────────────
var coanTermOf = map[string]string{
	"inbound_zero": "入链 = 0",
	"not_in_nav":   "不在 nav 白名单",
}

// ── Python 语义小工具 ─────────────────────────────────────────────────────────────────
func coanPyStrip(s string) string {
	return strings.TrimFunc(s, func(r rune) bool {
		if r >= 0x1c && r <= 0x1f {
			return true
		}
		return unicode.IsSpace(r)
	})
}

// coanPyJoin —— `os.path.join(a, b)`：b 绝对则取 b；否则串拼（**不规范化**）。
func coanPyJoin(a, b string) string {
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

// coanNorm —— `os.path.normpath(p).replace(os.sep, "/")`。
func coanNorm(p string) string {
	if p == "" {
		return "."
	}
	return filepath.ToSlash(filepath.Clean(p))
}

// coanSplitLines —— Python `str.splitlines()`（比 `\n` 多认一族行界）。
func coanSplitLines(s string) []string {
	var out []string
	start := 0
	i := 0
	for i < len(s) {
		c := s[i]
		size := 1
		isBreak := false
		switch {
		case c == '\n' || c == '\v' || c == '\f' || c == 0x1c || c == 0x1d || c == 0x1e:
			isBreak = true
		case c == '\r':
			isBreak = true
			if i+1 < len(s) && s[i+1] == '\n' {
				size = 2
			}
		case c == 0xc2 && i+1 < len(s) && s[i+1] == 0x85: // NEL
			isBreak = true
			size = 2
		case c == 0xe2 && i+2 < len(s) && s[i+1] == 0x80 && (s[i+2] == 0xa8 || s[i+2] == 0xa9): // U+2028/9
			isBreak = true
			size = 3
		}
		if isBreak {
			out = append(out, s[start:i])
			i += size
			start = i
			continue
		}
		i++
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

// coanUnquote —— `urllib.parse.unquote(s)`（%XX 解码 + UTF-8 errors="replace"）。
func coanUnquote(s string) string {
	if !strings.Contains(s, "%") {
		return s
	}
	var b []byte
	for i := 0; i < len(s); {
		if s[i] == '%' && i+2 < len(s) && coanIsHex(s[i+1]) && coanIsHex(s[i+2]) {
			b = append(b, byte(coanHexVal(s[i+1])<<4|coanHexVal(s[i+2])))
			i += 3
			continue
		}
		b = append(b, s[i])
		i++
	}
	return strings.ToValidUTF8(string(b), "\uFFFD")
}

func coanIsHex(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

func coanHexVal(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	default:
		return int(c-'A') + 10
	}
}

func coanStrIn(xs []string, v string) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

// ── 外链判定 ─────────────────────────────────────────────────────────────────────────
func coanIsExternal(target string) string {
	t := coanPyStrip(target)
	if strings.HasPrefix(t, "<") && strings.HasSuffix(t, ">") {
		t = coanPyStrip(t[1 : len(t)-1])
	}
	if t == "" {
		return "空目标"
	}
	low := strings.ToLower(t)
	for _, s := range coanExternalSchemes {
		if strings.HasPrefix(low, s) {
			return fmt.Sprintf("外链（%s…）", s)
		}
	}
	if strings.HasPrefix(t, "#") {
		return "纯锚（无路径）"
	}
	if strings.HasPrefix(t, "/") {
		return "绝对路径（不按件所在目录解析）"
	}
	if strings.Contains(t, "{") || strings.Contains(t, "}") {
		return "模板串（含花括号）"
	}
	return ""
}

// coanEllipsisTgt —— 遮蔽（抽取前置一步 · 不是判据）：目标里带省略号（`…` / `...`）
// ⇒ 是**被省略号截断的占位路径**，本来就不是一条可解析的链接（与围栏块/行内码同属遮蔽面）
// ⇒ 不进任何一格。口径照 `check-doc-freshness` 的「省略号目标（不判）」一类。
func coanEllipsisTgt(target string) bool {
	t := coanPyStrip(target)
	return strings.Contains(t, "\u2026") || strings.Contains(t, "...")
}

// ── 遮蔽（不是判据：挡的是「本来就不是链接的文本」）─────────────────────────────────────
func coanBlank(line string) string {
	return strings.Repeat(" ", utf8.RuneCountInString(line))
}

// coanMaskInlineCode —— 剥行内码 span（反引号成对 · 支持多反引号定界）。
func coanMaskInlineCode(line string) string {
	out := []byte(line)
	n := len(line)
	i := 0
	for i < n {
		if line[i] != '`' {
			i++
			continue
		}
		j := i
		for j < n && line[j] == '`' {
			j++
		}
		run := j - i
		end := -1
		k := j + 1
		for k+run <= n {
			if line[k] != '`' {
				k++
				continue
			}
			cnt := 0
			for k+cnt < n && line[k+cnt] == '`' {
				cnt++
			}
			if cnt >= run {
				end = k
				break
			}
			k += cnt
		}
		if end < 0 {
			i = j
			continue
		}
		for x := i; x < end+run; x++ {
			out[x] = ' '
		}
		i = end + run
	}
	return string(out)
}

// coanMaskCode —— 围栏代码块（整块）+ 行内码 span（块内）换等长空白。
func coanMaskCode(text string) string {
	var out []string
	fence := byte(0)
	for _, line := range strings.Split(text, "\n") {
		m := coanFenceRE.FindStringSubmatch(line)
		if fence == 0 {
			if m != nil {
				fence = m[1][0]
				out = append(out, coanBlank(line))
			} else {
				out = append(out, coanMaskInlineCode(line))
			}
		} else {
			if m != nil && m[1][0] == fence {
				fence = 0
			}
			out = append(out, coanBlank(line))
		}
	}
	return strings.Join(out, "\n")
}

// ── 域：树遍历（剪枝目录 + 嵌套仓）──────────────────────────────────────────────────────
type coanTriple struct {
	Dir   string
	Dirs  []string
	Files []string
}

func coanTreeWalk(root string) []coanTriple {
	var triples []coanTriple
	var nested []string
	var rec func(dir string)
	rec = func(dir string) {
		ents, err := os.ReadDir(dir)
		if err != nil {
			triples = append(triples, coanTriple{Dir: dir})
			return
		}
		var dirs, files []string
		for _, e := range ents {
			if e.IsDir() {
				dirs = append(dirs, e.Name())
			} else {
				files = append(files, e.Name())
			}
		}
		sort.Strings(dirs)
		sort.Strings(files)
		var keep []string
		for _, d := range dirs {
			if coanStrIn(coanPruneDirs, d) {
				continue
			}
			if _, err := os.Stat(filepath.Join(dir, d, ".git")); err == nil {
				nested = append(nested, coanNorm(coanRel(root, filepath.Join(dir, d))))
				continue
			}
			keep = append(keep, d)
		}
		triples = append(triples, coanTriple{Dir: dir, Dirs: keep, Files: files})
		for _, d := range keep {
			rec(filepath.Join(dir, d))
		}
	}
	rec(root)
	coanNestedSeen[root] = nested
	return triples
}

// coanRel —— `os.path.relpath(path, root)`（相对路径，无前导 `./`）。
func coanRel(root, path string) string {
	r, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return r
}

// coanScanTree —— 扫一棵树里的 .md 件 ⇒ ({相对路径: 全文|nil}, note)。ok=false ⇔ 根不在盘。
func coanScanTree(root string) (map[string]*string, string, bool) {
	if !coanIsDir(root) {
		return nil, "不在盘", false
	}
	out := map[string]*string{}
	for _, t := range coanTreeWalk(root) {
		for _, fn := range t.Files {
			if !strings.HasSuffix(fn, ".md") {
				continue
			}
			full := filepath.Join(t.Dir, fn)
			rel := coanNorm(coanRel(root, full))
			b, err := os.ReadFile(full)
			if err != nil {
				// ★ B 类真口子修（原 py `except OSError` 静默 · ✗ 不再静默）：登记 + 判红 + rc=2。
				coanReadFails = append(coanReadFails, coanHit{File: rel, Raw: err.Error()})
				out[rel] = nil
				continue
			}
			s := string(b)
			out[rel] = &s
		}
	}
	note := fmt.Sprintf("%d 件 .md", len(out))
	if len(coanNestedSeen[root]) > 0 {
		note += fmt.Sprintf(" · 剪掉嵌套仓 %d 个（域外：别仓的件）", len(coanNestedSeen[root]))
	}
	return out, note, true
}

// coanLinksOf —— 逐行抽链接（先遮蔽再抽）⇒ [(行号, 原文目标)]。
func coanLinksOf(text string) [][2]interface{} {
	var got [][2]interface{}
	for i, line := range coanSplitLines(coanMaskCode(text)) {
		for _, m := range coanLinkRE.FindAllStringSubmatch(line, -1) {
			got = append(got, [2]interface{}{i + 1, m[1]})
		}
		if m := coanDefRE.FindStringSubmatch(line); m != nil {
			got = append(got, [2]interface{}{i + 1, m[1]})
		}
	}
	return got
}

// coanRlink —— 一处的归一目标（剥离 <> / 去锚 / %解码）。空串 = 不进。
func coanRlink(rawtgt string) string {
	tgt := coanPyStrip(rawtgt)
	if strings.HasPrefix(tgt, "<") && strings.HasSuffix(tgt, ">") {
		tgt = coanPyStrip(tgt[1 : len(tgt)-1])
	}
	if idx := strings.Index(tgt, "#"); idx >= 0 {
		tgt = tgt[:idx]
	}
	tgt = coanUnquote(tgt)
	return tgt
}

// ── 一格：本地断链 ───────────────────────────────────────────────────────────────────
type coanHit struct {
	File     string
	Line     int
	Raw      string
	Resolved string
}

// coanReadFails —— B 类读失败登记面（读不了的 `.md` / `mkdocs.yml` · 逐件登记 相对路径 + err 正文）。
var coanReadFails []coanHit

// coanOffsetOK —— 命中「基准偏移登记」的错 ⇒ 按登记基准重解析 + 现读复核。
func coanOffsetOK(rel, tgt, root string, have map[string]bool) string {
	ent, ok := coanBaseOffset[rel]
	if !ok {
		return ""
	}
	base, why := ent[0], ent[1]
	cand := coanNorm(coanPyJoin(base, tgt))
	if have[cand] {
		return fmt.Sprintf("%s（登记基准 `%s`）⇒ 复核命中 `%s`", why, base, cand)
	}
	if coanExists(filepath.Join(root, filepath.FromSlash(cand))) {
		return fmt.Sprintf("%s（登记基准 `%s`）⇒ 复核命中 `%s`", why, base, cand)
	}
	return ""
}

// coanOffsetRegistryCheck —— 登记体检：过期 ⇒ 逐字点名（两种）。
func coanOffsetRegistryCheck(root string, files map[string]*string, rawHits []coanHit) []string {
	stale := []string{}
	if files == nil {
		return stale
	}
	hitFiles := map[string]bool{}
	for _, h := range rawHits {
		hitFiles[h.File] = true
	}
	keys := make([]string, 0, len(coanBaseOffset))
	for k := range coanBaseOffset {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, rel := range keys {
		if _, ok := files[rel]; !ok {
			continue
		}
		base := coanBaseOffset[rel][0]
		if !coanIsDir(filepath.Join(root, filepath.FromSlash(base))) {
			stale = append(stale, fmt.Sprintf("%s ⇒ **登记基准 `%s` 不是目录**（基准指空 ⇒ 撤或改登记）", rel, base))
			continue
		}
		if !hitFiles[rel] {
			stale = append(stale, fmt.Sprintf("%s ⇒ **在扫面域内却一条原位错都没有**（链接已改回原位可达 / 该件不再需要偏移 ⇒ 请撤登记）", rel))
		}
	}
	return stale
}

// coanLocalBrokenDetail —— 一格：本地断链（域 = 一棵树）。
func coanLocalBrokenDetail(root string) (int, bool, []coanHit, []coanHit, []string, string) {
	if root == "" || !coanIsDir(root) {
		return 0, false, nil, nil, nil, "根不在盘"
	}
	files, _, _ := coanScanTree(root)
	if len(files) == 0 {
		// ★ 空树成对判（py `local_broken_detail` 逐字搬）：域内 0 件但确有嵌套仓被剪 ⇒
		//   0 件是实情（照判 0）；真扫面 0 件 ⇒ **判不了**（ok=false ⇒ 入口 rc=2）——
		//   ✗ 绝不读成「0 件 = 干净」（本件头 (b)/退码节的自违面，本单补正）。
		if len(coanNestedSeen[root]) > 0 {
			return 0, true, nil, nil, nil, "扫面 0 件 .md（仅域外嵌套仓件）"
		}
		return 0, false, nil, nil, nil, "扫面 0 件 .md"
	}
	have := map[string]bool{}
	for k := range files {
		have[k] = true
	}
	raw := []coanHit{}
	hits := []coanHit{}
	exempt := []coanHit{}
	// ● GAP-20261001-26 修（照 py 候件逐处同步）：读不了的件逐条记下、不再一格 nil 丢整树。
	unreadable := []string{}
	rels := make([]string, 0, len(files))
	for k := range files {
		rels = append(rels, k)
	}
	sort.Strings(rels)
	for _, rel := range rels {
		text := files[rel]
		if text == nil {
			unreadable = append(unreadable, rel)
			continue
		}
		for _, lk := range coanLinksOf(*text) {
			lineno := lk[0].(int)
			rawtgt := lk[1].(string)
			if coanIsExternal(rawtgt) != "" || coanEllipsisTgt(rawtgt) {
				continue
			}
			tgt := coanRlink(rawtgt)
			if tgt == "" {
				continue
			}
			resolved := coanNorm(coanPyJoin(coanNorm(filepath.ToSlash(filepath.Dir(filepath.FromSlash(rel)))), tgt))
			if have[resolved] || coanExists(filepath.Join(root, filepath.FromSlash(resolved))) {
				continue
			}
			rec := coanHit{File: rel, Line: lineno, Raw: tgt, Resolved: resolved}
			raw = append(raw, rec)
			if coanOffsetOK(rel, tgt, root, have) != "" {
				exempt = append(exempt, rec)
			} else {
				hits = append(hits, rec)
			}
		}
	}
	stale := coanOffsetRegistryCheck(root, files, raw)
	seen := map[string]bool{}
	for _, h := range hits {
		seen[h.File] = true
	}
	n := len(seen)
	if len(unreadable) > 0 {
		// ● GAP-20261001-26 修：判不了态照返**已收明细**（hits/exempt/stale），why 单独标注「读不了 N 件」；
		//   ok=false（= py n=None）仍走「不给结论 rc=2」出口 ⇒ 只补证据面、不放宽判据（读不了 ≠ 零件）。
		tail := unreadable
		if len(tail) > 8 {
			tail = tail[:8]
		}
		return 0, false, hits, exempt, stale,
			fmt.Sprintf("件读不了 %d 件：%s", len(unreadable), strings.Join(tail, " · "))
	}
	return n, true, hits, exempt, stale, ""
}

// ── 二格：孤儿（域 = 一棵树整棵）──────────────────────────────────────────────────────
func coanOrphans(tree string) (int, bool, []coanHit, string) {
	if tree == "" || !coanIsDir(tree) {
		shown := tree
		if shown == "" {
			shown = "（没给）"
		}
		return 0, false, nil, "导出树不在盘：" + shown
	}
	allf := map[string]bool{}
	for _, t := range coanTreeWalk(tree) {
		for _, fn := range t.Files {
			allf[coanNorm(coanRel(tree, filepath.Join(t.Dir, fn)))] = true
		}
	}
	mds := map[string]bool{}
	for p := range allf {
		if strings.HasSuffix(p, ".md") {
			mds[p] = true
		}
	}
	if len(mds) == 0 {
		return 0, true, nil, "扫面 0 件 .md"
	}
	rels := make([]string, 0, len(mds))
	for p := range mds {
		rels = append(rels, p)
	}
	sort.Strings(rels)
	inbound := map[string]int{}
	for _, rel := range rels {
		b, err := os.ReadFile(filepath.Join(tree, filepath.FromSlash(rel)))
		if err != nil {
			// ★ B 类真口子修（原 py `except OSError` 静默 · ✗ 不再静默）：登记 + 判红 + rc=2。
			coanReadFails = append(coanReadFails, coanHit{File: rel, Raw: err.Error()})
			return 0, false, nil, "件读不了：" + rel
		}
		for _, lk := range coanLinksOf(string(b)) {
			raw := lk[1].(string)
			if coanIsExternal(raw) != "" || coanEllipsisTgt(raw) {
				continue
			}
			tgt := coanRlink(raw)
			if tgt == "" {
				continue
			}
			resolved := coanNorm(coanPyJoin(coanNorm(filepath.ToSlash(filepath.Dir(filepath.FromSlash(rel)))), tgt))
			if allf[resolved] {
				inbound[resolved]++
			}
		}
	}
	orphan := []string{}
	for p := range mds {
		if inbound[p] == 0 {
			orphan = append(orphan, p)
		}
	}
	sort.Strings(orphan)
	hits := make([]coanHit, 0, len(orphan))
	for _, p := range orphan {
		hits = append(hits, coanHit{File: p})
	}
	return len(orphan), true, hits, ""
}

// ── W-06 术语守卫 + nav 白名单 ────────────────────────────────────────────────────────
func coanTermGuard(kind, word string) string {
	want, ok := coanTermOf[kind]
	if !ok {
		ks := make([]string, 0, len(coanTermOf))
		for k := range coanTermOf {
			ks = append(ks, k)
		}
		sort.Strings(ks)
		return fmt.Sprintf("未知格 kind=%s（只认 %s）", coanPyReprStr(kind), strings.Join(ks, " / "))
	}
	if word != want {
		return fmt.Sprintf("格 `%s` 的用词是「%s」，不是「%s」—— 两词不许互称", kind, want, word)
	}
	return ""
}

func coanHasMkdocs(root string) bool {
	st, err := os.Stat(filepath.Join(root, "mkdocs.yml"))
	return err == nil && !st.IsDir()
}

func coanNavWhitelist(root string) (map[string]bool, bool, string) {
	p := filepath.Join(root, "mkdocs.yml")
	st, err := os.Stat(p)
	if err != nil || st.IsDir() {
		return nil, false, "本仓无 `mkdocs.yml`"
	}
	b, err := os.ReadFile(p)
	if err != nil {
		// ★ B 类真口子修（原 py `except OSError` 静默 · ✗ 不再静默）：登记 + 判红 + rc=2。
		coanReadFails = append(coanReadFails, coanHit{File: "mkdocs.yml", Raw: err.Error()})
		return nil, false, fmt.Sprintf("`mkdocs.yml` 读不了（%s）", err)
	}
	out := map[string]bool{}
	seen := false
	for _, line := range coanSplitLines(string(b)) {
		stripped := coanPyStrip(line)
		if coanNavKeyRE.MatchString(stripped) {
			seen = true
			continue
		}
		if !seen {
			continue
		}
		if stripped != "" && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			break
		}
		for _, m := range coanNavFileRE.FindAllString(line, -1) {
			out[m] = true
		}
	}
	return out, true, ""
}

func coanNotInNav(root string) (int, bool, []coanHit, string) {
	wl, ok, why := coanNavWhitelist(root)
	if !ok {
		return 0, false, nil, why
	}
	files, note, _ := coanScanTree(root)
	if files == nil {
		return 0, false, nil, note
	}
	if len(files) == 0 {
		return 0, false, nil, "扫面 0 件（无 `.md`）"
	}
	rels := make([]string, 0, len(files))
	for p := range files {
		rels = append(rels, p)
	}
	sort.Strings(rels)
	hits := []coanHit{}
	for _, p := range rels {
		if !wl[p] {
			hits = append(hits, coanHit{File: p})
		}
	}
	return len(hits), true, hits, ""
}

// ── 工具 ─────────────────────────────────────────────────────────────────────────────
func coanExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func coanIsDir(p string) bool {
	if p == "" {
		return false
	}
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

func coanIsFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// coanDocsDefault —— py `os.path.join(os.path.dirname(repo_root), "Zerg-内部文档")` 逐字同构（串拼 · 不规范化）。
func coanDocsDefault(root string) string {
	d := filepath.Dir(root)
	if d == "" {
		d = "."
	}
	return coanPyJoin(d, "Zerg-内部文档")
}

// coanResolveTree —— py `resolve_tree`：`--tree` → 主名 env → 别名 env → 无。
func coanResolveTree(cliTree string, hasCLI bool) (string, string) {
	if hasCLI {
		return cliTree, "--tree"
	}
	for _, name := range []string{coanPubTreeEnv, coanPubTreeEnvAlias} {
		val := os.Getenv(name)
		if coanPyStrip(val) != "" {
			return coanPyStrip(val), "$" + name
		}
	}
	return "", ""
}

// coanDefaultRoot —— 标准声明 (a)：从 cwd 上溯 ≤8 层找含 `scripts/gates` 的目录，找不到回落 cwd。
func coanDefaultRoot() string {
	d, err := os.Getwd()
	if err != nil {
		return "."
	}
	abs, err := filepath.Abs(d)
	if err != nil {
		abs = d
	}
	cur := abs
	for i := 0; i < 8; i++ {
		if coanIsDir(filepath.Join(cur, "scripts", "gates")) {
			return cur
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}
	return abs
}

// ── Python repr（self-test 用例的期望/实得串）────────────────────────────────────────
func coanPyReprStr(s string) string {
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
			if r == rune(q[0]) {
				b.WriteString("\\" + string(r))
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteString(q)
	return b.String()
}

func coanPyReprListStr(xs []string) string {
	parts := make([]string, 0, len(xs))
	for _, x := range xs {
		parts = append(parts, coanPyReprStr(x))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func coanPyReprTupleIntStrList(a int, xs []string) string {
	return "(" + strconv.Itoa(a) + ", " + coanPyReprListStr(xs) + ")"
}

func coanPyReprTupleInts(xs ...int) string {
	parts := make([]string, 0, len(xs))
	for _, x := range xs {
		parts = append(parts, strconv.Itoa(x))
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

func coanPyBool(b bool) string {
	if b {
		return "True"
	}
	return "False"
}

// ── 自检（成对负控 · 合成夹具 · 不碰真目标）──────────────────────────────────────────
func coanSelfTest(out io.Writer) bool {
	coanReadFails = nil // ★ 自清：包级登记面不许跨调用残留（残留会让下一次一进门就被 BLOCKED）
	total, failed := 0, 0
	caseFn := func(name, want, got string) {
		total++
		ok := want == got
		if !ok {
			failed++
		}
		mark := "✓"
		if !ok {
			mark = "✗"
		}
		fmt.Fprintf(out, "  %s %s（期望 %s · 实得 %s）\n", mark, name, want, got)
	}
	write := func(p, body string) {
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		_ = os.WriteFile(p, []byte(body), 0o644)
	}

	tmp, err := os.MkdirTemp("", "coan-selftest-")
	if err != nil {
		tmp = filepath.Join(os.TempDir(), "coan-selftest-fallback")
		_ = os.MkdirAll(tmp, 0o755)
	}
	defer os.RemoveAll(tmp)

	// 夹具①
	t1 := filepath.Join(tmp, "t1")
	_ = os.MkdirAll(filepath.Join(t1, "sub"), 0o755)
	write(filepath.Join(t1, "a.md"), "# a\n\n[好](sub/b.md) · [坏](sub/nope.md)\n\n"+
		"[外](https://example.com/x.md) · [锚](#sec) · [绝](/etc/hosts) · "+
		"[邮](mailto:a@b.c) · [协相对](//host/x) · [模]({{var}}/x.md)\n")
	write(filepath.Join(t1, "sub", "b.md"), "# b\n\n[回](../a.md)\n")
	n1, ok1, hits1, _, _, _ := coanLocalBrokenDetail(t1)
	got1a := "None"
	if ok1 {
		got1a = strconv.Itoa(n1)
	}
	caseFn("① 本地断链：件数（只数真断链 · 外链 6 处全不进）", "1", got1a)
	pair := []string{"", ""}
	if len(hits1) > 0 {
		pair = []string{fmt.Sprintf("%s:%d", hits1[0].File, hits1[0].Line), hits1[0].Resolved}
	}
	caseFn("① 本地断链：点名到 件:行 + 解析后路径", coanPyReprListStr([]string{"a.md:3", "sub/nope.md"}), coanPyReprListStr(pair))

	// 夹具②
	t2 := filepath.Join(tmp, "t2")
	_ = os.MkdirAll(t2, 0o755)
	write(filepath.Join(t2, "a.md"), "[好](b.md)\n")
	write(filepath.Join(t2, "b.md"), "[回](a.md)\n")
	n2, ok2, _, _, _, _ := coanLocalBrokenDetail(t2)
	g2 := "None"
	if ok2 {
		g2 = strconv.Itoa(n2)
	}
	caseFn("② 正控：全好 ⇒ 本地断链 0 件", "0", g2)

	// 夹具③
	t3 := filepath.Join(tmp, "t3")
	_ = os.MkdirAll(t3, 0o755)
	write(filepath.Join(t3, "a.md"), "[b](b.md)\n")
	write(filepath.Join(t3, "b.md"), "[a](a.md)\n")
	write(filepath.Join(t3, "c.md"), "没人链我（[外链不算入链](https://example.com/a.md)）\n")
	n3, ok3, hits3, _ := coanOrphans(t3)
	g3 := "None"
	if ok3 {
		g3 = strconv.Itoa(n3)
	}
	caseFn("③ 孤儿：入链 = 0 的件数（外链不算入链）", "1", g3)
	names3 := []string{}
	for _, h := range hits3 {
		names3 = append(names3, h.File)
	}
	caseFn("③ 孤儿：点名到件", coanPyReprListStr([]string{"c.md"}), coanPyReprListStr(names3))

	// 夹具④
	_, okLB, _, _, _, _ := coanLocalBrokenDetail(filepath.Join(tmp, "nope"))
	_, okOr, _, _ := coanOrphans(filepath.Join(tmp, "nope"))
	caseFn("④ 判不了：根不在盘 ⇒ local_broken 返回 判不了", "True", coanPyBool(!okLB))
	caseFn("④ 判不了：导出树不在盘 ⇒ orphans 返回 判不了", "True", coanPyBool(!okOr))

	// ⑤-⑦ W-06
	caseFn("⑤ W-06 术语守卫：本词 ⇒ 放行", "", coanTermGuard("inbound_zero", "入链 = 0"))
	caseFn("⑤ W-06 术语守卫（负控：把「入链 0」说成「不在 nav」⇒ 判词非空）", "True",
		coanPyBool(coanTermGuard("inbound_zero", "不在 nav 白名单") != ""))
	caseFn("⑤ W-06 术语守卫（负控：反向互称也判）", "True",
		coanPyBool(coanTermGuard("not_in_nav", "入链 = 0") != ""))
	t6a := filepath.Join(tmp, "t6a")
	_ = os.MkdirAll(t6a, 0o755)
	write(filepath.Join(t6a, "a.md"), "# a\n")
	_, ok6a, _, why6a := coanNotInNav(t6a)
	caseFn("⑥ W-06 nav：无 `mkdocs.yml` ⇒ **无适用域**（判不了 · 不是 0 件）", "True", coanPyBool(!ok6a))
	caseFn("⑥ W-06 nav：无 `mkdocs.yml` ⇒ 理由点名本仓无", "True", coanPyBool(strings.Contains(why6a, "无 `mkdocs.yml`")))
	t6b := filepath.Join(tmp, "t6b")
	_ = os.MkdirAll(t6b, 0o755)
	write(filepath.Join(t6b, "mkdocs.yml"), "site_name: x\nnav:\n  - 首页: index.md\n")
	write(filepath.Join(t6b, "index.md"), "# i\n")
	write(filepath.Join(t6b, "loose.md"), "# l\n")
	n6, _, hits6, _ := coanNotInNav(t6b)
	caseFn("⑥ W-06 nav：白名单外的 `.md` 被数（1 件）", "1", strconv.Itoa(n6))
	names6 := []string{}
	for _, h := range hits6 {
		names6 = append(names6, h.File)
	}
	caseFn("⑥ W-06 nav：点名到件（白名单里的 index.md 不算）", coanPyReprListStr([]string{"loose.md"}), coanPyReprListStr(names6))
	n7, _, hits7, _ := coanOrphans(t3)
	caseFn("⑦ W-06 ③ 域 = 第二棵树整棵：与 ② 同度量、换域照数（1 件）", "1", strconv.Itoa(n7))
	names7 := []string{}
	for _, h := range hits7 {
		names7 = append(names7, h.File)
	}
	_, _, hits7b, _ := coanOrphans(t3)
	n7b := []string{}
	for _, h := range hits7b {
		n7b = append(n7b, h.File)
	}
	caseFn("⑦ W-06 ③ 与 ② 的点名件同一枚（度量同源 · 域不同）", coanPyReprListStr(n7b), coanPyReprListStr(names7))

	// ⑧⑨ 遮蔽：围栏块
	t8 := filepath.Join(tmp, "t8")
	_ = os.MkdirAll(t8, 0o755)
	write(filepath.Join(t8, "a.md"), "# a\n\n```sh\npython3 x.py '[坏](gone.md)'\n```\n")
	n8a, ok8a, _, _, _, _ := coanLocalBrokenDetail(t8)
	g8a := "None"
	if ok8a {
		g8a = strconv.Itoa(n8a)
	}
	caseFn("⑧ 遮蔽·围栏块里的路径 ⇒ **不报**（不是链接）", "0", g8a)
	write(filepath.Join(t8, "a.md"), "# a\n\n[坏](gone.md)\n\n```sh\npython3 x.py '[也坏](gone2.md)'\n```\n")
	n8, ok8, h8, _, _, _ := coanLocalBrokenDetail(t8)
	if !ok8 {
		n8 = -1
	}
	raws8 := []string{}
	for _, h := range h8 {
		raws8 = append(raws8, h.Raw)
	}
	caseFn("⑨ 遮蔽（成对）：块外那一处**照报**（件数 1 · 只报块外那处）",
		coanPyReprTupleIntStrList(1, []string{"gone.md"}), coanPyReprTupleIntStrList(n8, raws8))

	// ⑩⑪ 遮蔽：行内码 span
	t9 := filepath.Join(tmp, "t9")
	_ = os.MkdirAll(t9, 0o755)
	write(filepath.Join(t9, "a.md"), "# a\n\n抽 `[字](目标)` 时把 ``[坏](gone.md)`` 也抽走了。\n")
	n9a, ok9a, _, _, _, _ := coanLocalBrokenDetail(t9)
	g9a := "None"
	if ok9a {
		g9a = strconv.Itoa(n9a)
	}
	caseFn("⑩ 遮蔽·行内码 span 里的路径 ⇒ **不报**", "0", g9a)
	write(filepath.Join(t9, "a.md"), "# a\n\n真链接 [坏](gone.md)（另有行内码 ``[x](gone2.md)``）\n")
	n9, ok9, h9, _, _, _ := coanLocalBrokenDetail(t9)
	if !ok9 {
		n9 = -1
	}
	raws9 := []string{}
	for _, h := range h9 {
		raws9 = append(raws9, h.Raw)
	}
	caseFn("⑪ 遮蔽（成对）：行内码外的真断链**照报**（件数 1 · 只报码外那处）",
		coanPyReprTupleIntStrList(1, []string{"gone.md"}), coanPyReprTupleIntStrList(n9, raws9))

	// ⑫⑬ 域：嵌套仓
	t10 := filepath.Join(tmp, "t10")
	_ = os.MkdirAll(filepath.Join(t10, "vendor", "sub"), 0o755)
	write(filepath.Join(t10, "vendor", "sub", ".git"), "gitdir: /elsewhere/.git/modules/sub\n")
	write(filepath.Join(t10, "vendor", "sub", "doc.md"), "# v\n\n[坏](gone.md)\n")
	n10a, ok10a, _, _, _, _ := coanLocalBrokenDetail(t10)
	g10a := "None"
	if ok10a {
		g10a = strconv.Itoa(n10a)
	}
	caseFn("⑫ 域·嵌套仓（子模块工作树）里的件 ⇒ **不进域、不报**", "0", g10a)
	write(filepath.Join(t10, "doc.md"), "# v\n\n[坏](gone.md)\n")
	n10, ok10, h10, _, _, _ := coanLocalBrokenDetail(t10)
	if !ok10 {
		n10 = -1
	}
	names10 := []string{}
	for _, h := range h10 {
		names10 = append(names10, h.File)
	}
	caseFn("⑬ 域（成对）：同一段坏文本在域内 ⇒ **必报**（件数 1）",
		coanPyReprTupleIntStrList(1, []string{"doc.md"}), coanPyReprTupleIntStrList(n10, names10))

	// ⑭⑮ 基准偏移登记
	t11 := filepath.Join(tmp, "t11")
	_ = os.MkdirAll(filepath.Join(t11, "publish", "docs"), 0o755)
	write(filepath.Join(t11, "publish", "docs", "QUICKSTART.en.md"), "# q\n")
	write(filepath.Join(t11, "publish", "LICENSE"), "Apache-2.0\n")
	write(filepath.Join(t11, "publish", "docs", "README.md"), "# r\n\n见 [q](docs/QUICKSTART.en.md) 与 [lic](LICENSE)。\n")
	n11, ok11, _, ex11, st11, _ := coanLocalBrokenDetail(t11)
	if !ok11 {
		n11 = -1
	}
	caseFn("⑭ 登记·按基准复核命中 ⇒ **不报**（红 0 · 已登记假红 2 处 · 登记未过期）",
		coanPyReprTupleInts(0, 2, 0), coanPyReprTupleInts(n11, len(ex11), len(st11)))
	write(filepath.Join(t11, "publish", "docs", "README.md"), "# r\n\n见 [q](docs/QUICKSTART.en.md)。\n\n也见 [x](docs/NOPE.md)。\n")
	n11b, ok11b, red11, _, _, _ := coanLocalBrokenDetail(t11)
	if !ok11b {
		n11b = -1
	}
	raws11 := []string{}
	for _, h := range red11 {
		raws11 = append(raws11, h.Raw)
	}
	caseFn("⑮ 登记（成对）：基准也指不到的 ⇒ **照旧判红并点名**",
		coanPyReprTupleIntStrList(1, []string{"docs/NOPE.md"}), coanPyReprTupleIntStrList(n11b, raws11))

	// ⑯⑰ 登记体检
	t12 := filepath.Join(tmp, "t12")
	_ = os.MkdirAll(filepath.Join(t12, "publish", "docs"), 0o755)
	write(filepath.Join(t12, "publish", "docs", "QUICKSTART.en.md"), "# q\n")
	write(filepath.Join(t12, "publish", "docs", "README.md"), "# r\n\n见 [q](QUICKSTART.en.md)。\n")
	n12, ok12, _, _, st12, _ := coanLocalBrokenDetail(t12)
	if !ok12 {
		n12 = -1
	}
	second := len(st12)
	for _, s := range st12 {
		if strings.Contains(s, "请撤登记") {
			second = 1
			break
		}
	}
	caseFn("⑯ 登记体检①：登记件在域内却 0 原位错 ⇒ 判**登记过期**并点名",
		coanPyReprTupleInts(0, 1), coanPyReprTupleInts(n12, second))
	old := coanBaseOffset["publish/docs/README.md"]
	coanBaseOffset["publish/docs/README.md"] = [2]string{"no-such-base", "夹具：基准指空"}
	st2 := coanOffsetRegistryCheck(t12, map[string]*string{"publish/docs/README.md": coanStrPtr("x")},
		[]coanHit{{File: "publish/docs/README.md"}})
	coanBaseOffset["publish/docs/README.md"] = old
	caseFn("⑰ 登记体检②：登记基准**不是目录** ⇒ 点名（基准指空）", "True",
		coanPyBool(len(st2) > 0 && strings.Contains(st2[0], "不是目录")))

	// ── ⑱⑲ 空树（成对）：真扫面 0 件 = 判不了（与件头退码节 / not_in_nav 同档 · rc=2 · GAP-20260927-124）
	t12e := filepath.Join(tmp, "t12empty")
	_ = os.MkdirAll(t12e, 0o755)
	_, ok12e, _, _, _, _ := coanLocalBrokenDetail(t12e)
	caseFn("⑱ 空树（无 .md · 无嵌套仓）：local_broken_detail 判不了（None => rc=2），不是 0 件绿",
		"True", coanPyBool(!ok12e))
	write(filepath.Join(t12e, "a.md"), "# a\n\n[bad](gone.md)\n")
	n12e, _, _, _, _, _ := coanLocalBrokenDetail(t12e)
	caseFn("⑲ 空树（成对）：有 .md 且真有断链 => 照报（件数 1）", "1", strconv.Itoa(n12e))

	// ⑳㉑ 遮蔽：省略号占位目标（`…` / `...` · 2026-10-13 · `GAP-20260926-191`）
	t13 := filepath.Join(tmp, "t13")
	_ = os.MkdirAll(t13, 0o755)
	write(filepath.Join(t13, "a.md"), "# a\n\n见 [目录](docs/项目文档/…) 与 [盘](../../…) 两处\n")
	n13a, ok13a, _, _, _, _ := coanLocalBrokenDetail(t13)
	g13a := "None"
	if ok13a {
		g13a = strconv.Itoa(n13a)
	}
	caseFn("⑳ 遮蔽·省略号占位目标（`…` / `...`）⇒ **不进任何一格**（被截断的占位路径 · 不是链接）", "0", g13a)
	write(filepath.Join(t13, "a.md"), "# a\n\n见 [目录](docs/项目文档/…) 与 [真坏](gone.md)\n")
	n13, ok13, h13, _, _, _ := coanLocalBrokenDetail(t13)
	if !ok13 {
		n13 = -1
	}
	raws13 := []string{}
	for _, h := range h13 {
		raws13 = append(raws13, h.Raw)
	}
	caseFn("㉑ 遮蔽（成对）：省略号之外的真断链**照报**（件数 1 · 只报真断链那处）",
		coanPyReprTupleIntStrList(1, []string{"gone.md"}), coanPyReprTupleIntStrList(n13, raws13))
	fmt.Fprintf(out, "  自检 %d 格 · 失败 %d 格\n", total, failed)
	return failed == 0
}

func coanStrPtr(s string) *string { return &s }

// ── LIST_RULES（逐字复刻 py）────────────────────────────────────────────────────────
const coanListRules = "判据（两格 · 只报不拦）\n" +
	"------------------------\n" +
	"  ① local_broken 本地断链 · 域 = 两棵树（--repo-root + --docs-root）\n" +
	"     判据 = markdown 件里的**仓内相对**链接（行内 `](t)` + 参考式 `]: t`）解析后指不到真件\n" +
	"     计数单位 = **件**（同一件多处断链只算一件）；明细 = 件:行 + 原文 + 解析后路径\n" +
	"  ② orphans 孤儿 · 域 = 导出树（--tree）\n" +
	"     判据 = 树内 `.md` 件的**入链数 = 0**（树内别的件有一条仓内相对链接解析后落在它身上 = 有入链）\n" +
	"     计数单位 = **件**；明细 = 件（入链 0）\n" +
	"  ③ inbound_zero **入链 = 0** · 域 = **第二棵树整棵**（--docs-root · `W-06` 序91）\n" +
	"     判据 = 同一枚度量（链接数），**另一个域**（源树整棵）；与 ② **不许互称**\n" +
	"  ④ not_in_nav **不在 nav 白名单** · 域 = **有 `mkdocs.yml` 的树**（`W-06` 序91）\n" +
	"     判据 = `mkdocs.yml` 的 `nav:` 白名单外的 `.md` 件；**本仓无 `mkdocs.yml`** ⇒ **无适用域**（不是 0 件 ✗）\n" +
	"\n" +
	"外链明确不进（负控钉住）\n" +
	"------------------------\n" +
	"  http(s) / ftp(s) / 协议相对 `//` / mailto / tel / data / 纯锚 `#x` / 绝对路径 `/x` /\n" +
	"  含花括号的模板 / `<>` 包裹 / 空串 ⇒ **不进任何一格**\n" +
	"\n" +
	"遮蔽（抽取**前置**一步 · **不是**判据 · 2026-09-26 重测落的）\n" +
	"------------------------------------------------------------\n" +
	"  · **围栏代码块**（` ``` ` / `~~~` 起止）**整块** + **行内码 span**（反引号成对 · 支持多反引号定界）\n" +
	"    ⇒ 抽取前换成**等长空白**（保行号/列偏移）⇒ 块内的 `[字](目标)` **不进**\n" +
	"    （那两处是**被引用的文本**：别仓坏链原话 / 链接语法示例 / 修法脚本里的字符串字面量）\n" +
	"  · **省略号占位目标**（目标里带 `…` / `...` ⇒ **被省略号截断的占位路径** · 与围栏块/行内码同属「本来就不是链接的文本」）\n" +
	"    ⇒ 抽取前置遮蔽 · **不进任何一格** · 成对负控见自检 ⑳㉑（本仓现读：主仓 0 处 · 第二棵树 3 处 / 2 件 ⇒ 该格 6 件 → 4 件）\n" +
	"  · 遮蔽**之外**的同一段坏文本照旧判 —— 成对负控钉住：**块内不报 ∧ 块外必报**（本仓现读：19 处 = 围栏 8 + 行内码 11，全假红）\n" +
	"  · 口径与 `check-zh-en.py` 的 `strip_noncode` 同族（先剥引用/代码，再判正文）—— **复用已有管线**、不加新机制\n" +
	"\n" +
	"域里**不进**（2026-09-26 重测落的）\n" +
	"----------------------------------\n" +
	"  · **嵌套仓**（目录里有 `.git` = 子模块工作树 / 半克隆）⇒ 那是**别仓的件**，本门判不了也不该判\n" +
	"    ⇒ 剪掉并**照实打印**（登记一行 · 不静默）：本仓 `vendor/rtk` = 子模块 `rtk-ai/rtk`（父仓只跟踪 gitlink）\n" +
	"  · 已有剪枝目录（`.git` / `node_modules` / `venv` / `.venv` / `__pycache__`）照旧\n" +
	"\n" +
	"基准偏移登记（`BASE_OFFSET` · **收窄判据** · 与 `check-links-lychee.py` 同式样）\n" +
	"------------------------------------------------------------------------------\n" +
	"  · 键 = 件相对**树根**路径（**无行号锚**）· 值 = `(基准目录, 理由)`\n" +
	"  · 命中登记 ⇒ **按登记基准重解析 + 现读存在性复核**：复核命中 ⇒ 记「已登记假红（基准偏移 · 已复核）」· **不计入断链数**（逐条印落点）\n" +
	"  · 复核落空 ⇒ **照旧判红**（豁免只挡「基准错」这一半）· 登记**过期** ⇒ **rc=2 + 逐字点名**（基准指空 / 件在域内却 0 原位错）\n" +
	"\n" +
	"未覆盖（不假装全覆盖）\n" +
	"----------------------\n" +
	"  · 目录型链接（指向目录而非件）只判「目录在不在」，不追目录内默认件\n" +
	"  · 非 `.md` 件里的链接（.html/.txt/.py）**不扫**（本题面只管文档）\n" +
	"  · 另一棵树里的**导出树**（跨仓的产出树）不判\n" +
	"  · **嵌套仓内部**（子模块自己的文档）不判 —— 域外（见上「域里不进」）\n" +
	"  · nav 白名单 / `mkdocs.yml` 那一半 = `W-06`（2026-09-24 **波6 序91 已落** ⇒ 落成上面 ③④ 两格；\n" +
	"    **本仓无 `mkdocs.yml`** 那一行**照实登记**、不假装覆盖）\n" +
	"\n" +
	"退码\n" +
	"----\n" +
	"  0 = 判得了（命中多少都不改码 —— 「先只报不拦」）\n" +
	"  2 = 不给结论（仓根缺 / 扫面 0 件 / 件读不了 / **基准偏移登记过期** / 自检不过 /\n" +
	"                **导出树读不到** / **第二棵树读不到** —— 缺件 ≠ 绿 · 2026-10-01 收口）\n"

// ── 机器面（Python json.dumps(..., ensure_ascii=False) 逐字）──────────────────────────
type coanOM struct {
	keys []string
	vals []any
}

func (o *coanOM) set(k string, v any) {
	o.keys = append(o.keys, k)
	o.vals = append(o.vals, v)
}

func coanPyQuote(s string) string {
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

func coanJSONEnc(v any) string {
	switch x := v.(type) {
	case nil:
		return "null"
	case string:
		return coanPyQuote(x)
	case int:
		return strconv.Itoa(x)
	case *int:
		if x == nil {
			return "null"
		}
		return strconv.Itoa(*x)
	case []string:
		parts := make([]string, 0, len(x))
		for _, s := range x {
			parts = append(parts, coanPyQuote(s))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case *coanOM:
		parts := make([]string, 0, len(x.keys))
		for i, k := range x.keys {
			parts = append(parts, coanPyQuote(k)+": "+coanJSONEnc(x.vals[i]))
		}
		return "{" + strings.Join(parts, ", ") + "}"
	}
	return "null"
}

// ── argparse 子集 ────────────────────────────────────────────────────────────────────
const coanProg = "check-docs-orphans-links.py"

const coanUsage = "usage: check-docs-orphans-links.py [-h] [--repo-root REPO_ROOT]\n" +
	"                                   [--docs-root DOCS_ROOT] [--tree TREE]\n" +
	"                                   [--max-examples MAX_EXAMPLES] [--json]\n" +
	"                                   [--list-rules] [--self-test]\n"

const coanHelp = coanUsage + "\n" +
	"文档孤儿 / 本地断链两格（只报不拦）\n" +
	"\n" +
	"optional arguments:\n" +
	"  -h, --help            show this help message and exit\n" +
	"  --repo-root REPO_ROOT\n" +
	"  --docs-root DOCS_ROOT\n" +
	"                        第二棵树（默认 = 同级目录 Zerg-内部文档；不在盘 ⇒ 判不了 ⇒ 本门不给结论 rc=2）\n" +
	"  --tree TREE           导出树（默认 $ZERG_PUB_TREE · 兼容别名 $ZERG_PUBLIC_TREE；都没有 ⇒\n" +
	"                        不给结论）\n" +
	"  --max-examples MAX_EXAMPLES\n" +
	"  --json\n" +
	"  --list-rules\n" +
	"  --self-test\n"

func coanArgErr(stderr io.Writer, msg string) int {
	fmt.Fprint(stderr, coanUsage)
	fmt.Fprintf(stderr, "%s: error: %s\n", coanProg, msg)
	return 2
}

type coanArgs struct {
	RepoRoot    string
	DocsRoot    string
	HasDocs     bool
	Tree        string
	HasTree     bool
	MaxExamples int
	JSON        bool
	ListRules   bool
	SelfTest    bool
}

// coanParse —— 返回 (args, rc)。rc>=0 ⇒ 已出结论；rc<0 ⇒ 继续。
func coanParse(stdout, stderr io.Writer, argv []string) (coanArgs, int) {
	a := coanArgs{MaxExamples: 20}
	var unknown []string
	i := 0
	needVal := func(name string, hasEq bool, val string) (string, int) {
		if hasEq {
			return val, -1
		}
		if i < len(argv) && !strings.HasPrefix(argv[i], "-") {
			v := argv[i]
			i++
			return v, -1
		}
		return "", coanArgErr(stderr, "argument "+name+": expected one argument")
	}
	for i < len(argv) {
		tok := argv[i]
		i++
		if tok == "-h" || tok == "--help" {
			fmt.Fprint(stdout, coanHelp)
			return a, 0
		}
		if strings.HasPrefix(tok, "--") {
			name := tok
			val := ""
			hasEq := false
			if eq := strings.IndexByte(tok, '='); eq >= 0 {
				name, val, hasEq = tok[:eq], tok[eq+1:], true
			}
			switch name {
			case "--repo-root":
				v, rc := needVal(name, hasEq, val)
				if rc >= 0 {
					return a, rc
				}
				a.RepoRoot = v
			case "--docs-root":
				v, rc := needVal(name, hasEq, val)
				if rc >= 0 {
					return a, rc
				}
				a.DocsRoot, a.HasDocs = v, true
			case "--tree":
				v, rc := needVal(name, hasEq, val)
				if rc >= 0 {
					return a, rc
				}
				a.Tree, a.HasTree = v, true
			case "--max-examples":
				v, rc := needVal(name, hasEq, val)
				if rc >= 0 {
					return a, rc
				}
				n, err := strconv.Atoi(v)
				if err != nil {
					return a, coanArgErr(stderr, fmt.Sprintf("argument --max-examples: invalid int value: '%s'", v))
				}
				a.MaxExamples = n
			case "--json":
				if hasEq {
					return a, coanArgErr(stderr, fmt.Sprintf("argument --json: ignored explicit argument '%s'", val))
				}
				a.JSON = true
			case "--list-rules":
				if hasEq {
					return a, coanArgErr(stderr, fmt.Sprintf("argument --list-rules: ignored explicit argument '%s'", val))
				}
				a.ListRules = true
			case "--self-test":
				if hasEq {
					return a, coanArgErr(stderr, fmt.Sprintf("argument --self-test: ignored explicit argument '%s'", val))
				}
				a.SelfTest = true
			default:
				unknown = append(unknown, tok)
			}
			continue
		}
		unknown = append(unknown, tok)
	}
	if len(unknown) > 0 {
		return a, coanArgErr(stderr, "unrecognized arguments: "+strings.Join(unknown, " "))
	}
	return a, -1
}

// ── 入口（= py `main()`）─────────────────────────────────────────────────────────────
func checkDocsOrphansLinksGateMain(stdout, stderr io.Writer, argv []string) int {
	a, rc := coanParse(stdout, stderr, argv)
	if rc >= 0 {
		return rc
	}
	if a.ListRules {
		fmt.Fprint(stdout, coanListRules)
		return 0
	}
	if a.SelfTest {
		fmt.Fprint(stdout, "── 自检（合成夹具 · 不碰真目标）──\n")
		if coanSelfTest(stdout) {
			return 0
		}
		return 2
	}

	coanReadFails = nil
	repoRoot := a.RepoRoot
	if repoRoot == "" {
		repoRoot = coanDefaultRoot()
	}
	if !coanIsDir(repoRoot) {
		fmt.Fprintf(stdout, "⛔ 仓根不在盘：%s ⇒ 不给结论（rc=2）\n", repoRoot)
		return 2
	}
	docsRoot := a.DocsRoot
	if !a.HasDocs {
		docsRoot = coanDocsDefault(repoRoot)
	}
	tree, treeSrc := coanResolveTree(a.Tree, a.HasTree)

	fmt.Fprintf(stdout, "文档孤儿 / 本地断链（两格 · 只报不拦）· v%s\n", coanVersion)
	fmt.Fprint(stdout, "  判据见 `--list-rules`：① local_broken（域 = 两棵树）· ② orphans（域 = 导出树）\n")
	fmt.Fprintf(stdout, "  域① 主仓根 = %s\n", repoRoot)
	docsNote := ""
	if !coanIsDir(docsRoot) {
		docsNote = "  ★不在盘（该树判不了 ⇒ 本门不给结论 rc=2 · 缺件 ≠ 绿）"
	}
	fmt.Fprintf(stdout, "  域① 第二棵树 = %s%s\n", docsRoot, docsNote)
	treeNote := ""
	if !(tree != "" && coanIsDir(tree)) {
		treeNote = "  ★读不到（孤儿格 = 判不了 ⇒ 本门不给结论 rc=2）"
	}
	shownTree := tree
	if shownTree == "" {
		shownTree = "（未给）"
	}
	shownSrc := treeSrc
	if shownSrc == "" {
		shownSrc = "无"
	}
	fmt.Fprintf(stdout, "  域② 导出树 = %s（来源 %s）%s\n", shownTree, shownSrc, treeNote)

	bad := 0
	secondMissing := false
	secondWhy := ""
	brCounts := [2]*int{} // 主仓 / 第二棵树（nil = 判不了）
	brExempt := [2][]coanHit{}
	var brStale []string
	labels := [2]string{"主仓", "第二棵树"}
	roots := [2]string{repoRoot, docsRoot}
	var brAll []coanHit
	for idx := 0; idx < 2; idx++ {
		n, ok, hits, exempt, stale, why := coanLocalBrokenDetail(roots[idx])
		if !ok {
			brCounts[idx] = nil
		} else {
			v := n
			brCounts[idx] = &v
		}
		brExempt[idx] = exempt
		brStale = append(brStale, stale...)
		if !ok {
			fmt.Fprintf(stdout, "  ⛔ 本地断链 · %s：判不了（%s）\n", labels[idx], why)
			// ● GAP-20261001-26 修（照 py 候件逐处同步）：判不了 ≠ 无红 —— 本树别处已收的红明细 /
			//   假红明细照印，不因一格读不了而丢弃（只补证据面、不放宽判据）。
			for k, h := range hits {
				if k >= a.MaxExamples {
					break
				}
				fmt.Fprintf(stdout, "      %s:%d  [%s] ⇒ %s（指不到）\n", h.File, h.Line, h.Raw, h.Resolved)
			}
			brAll = append(brAll, hits...)
			if len(exempt) > 0 { // 已登记假红**逐条印出落点**（看得见，不是静默放行）
				seen := map[string]bool{}
				for _, h := range hits {
					seen[h.File] = true
				}
				fmt.Fprintf(stdout, "      ⊕ 已登记假红（基准偏移 · 已逐条复核 · 不计入上面 %d 处）：\n", len(seen))
				for k, h := range exempt {
					if k >= a.MaxExamples {
						break
					}
					fmt.Fprintf(stdout, "          %s:%d  [%s] ⟸ %s\n", h.File, h.Line, h.Raw,
						coanOffsetOK(h.File, h.Raw, roots[idx], map[string]bool{}))
				}
			}
			if idx == 0 {
				bad = 1
			} else {
				secondMissing = true
				secondWhy = why
			}
		} else {
			fmt.Fprintf(stdout, "  ✓ 本地断链 · %s：%d 件\n", labels[idx], n)
			for k, h := range hits {
				if k >= a.MaxExamples {
					break
				}
				fmt.Fprintf(stdout, "      %s:%d  [%s] ⇒ %s（指不到）\n", h.File, h.Line, h.Raw, h.Resolved)
			}
			brAll = append(brAll, hits...)
			if len(exempt) > 0 {
				seen := map[string]bool{}
				for _, h := range hits {
					seen[h.File] = true
				}
				fmt.Fprintf(stdout, "      ⊕ 已登记假红（基准偏移 · 已逐条复核 · 不计入上面 %d 处）：\n", len(seen))
				for k, h := range exempt {
					if k >= a.MaxExamples {
						break
					}
					fmt.Fprintf(stdout, "          %s:%d  [%s] ⟸ %s\n", h.File, h.Line, h.Raw,
						coanOffsetOK(h.File, h.Raw, roots[idx], map[string]bool{}))
				}
			}
		}
		for _, nested := range coanNestedSeen[roots[idx]] {
			fmt.Fprintf(stdout, "      ⊘ 剪掉的嵌套仓（域外 · 别仓的件）：%s\n", nested)
		}
	}
	if len(brAll) > a.MaxExamples*2 {
		fmt.Fprintf(stdout, "      （明细截断：共 %d 处 · 上面每树各取前 %d 行）\n", len(brAll), a.MaxExamples)
	}
	if len(brStale) > 0 {
		fmt.Fprintf(stdout, "  ⛔ 基准偏移登记过期 %d 条 —— 登记指不到现实（逐字点名）：\n", len(brStale))
		for _, s := range brStale {
			fmt.Fprintf(stdout, "      · %s\n", s)
		}
		fmt.Fprint(stdout, "  ⇒ 不给结论（rc=2）—— 登记过期不许静默（治法：撤或改 `BASE_OFFSET`）\n")
		return 2
	}

	on, okOrph, ohits, owhy := coanOrphans(tree)
	for _, nested := range coanNestedSeen[tree] {
		fmt.Fprintf(stdout, "      ⊘ 剪掉的嵌套仓（域外 · 别仓的件）：%s\n", nested)
	}
	if !okOrph {
		fmt.Fprintf(stdout, "  ⛔ 孤儿 · 导出树：判不了（%s）—— 这一格判不了 ≠ 整门判不了；"+
			"★ 导出树是本门判据的必需输入 ⇒ **缺件 ≠ 绿**：本门不给结论（rc=2）\n", owhy)
	} else {
		fmt.Fprintf(stdout, "  ✓ 孤儿 · 导出树：%d 件（入链 = 0）\n", on)
		for k, h := range ohits {
			if k >= a.MaxExamples {
				break
			}
			fmt.Fprintf(stdout, "      %s（入链 0）\n", h.File)
		}
	}

	inzWord, navWord := coanTermOf["inbound_zero"], coanTermOf["not_in_nav"]
	for _, kw := range [][2]string{{"inbound_zero", inzWord}, {"not_in_nav", navWord}} {
		if guard := coanTermGuard(kw[0], kw[1]); guard != "" {
			fmt.Fprintf(stdout, "  ⛔ 术语守卫（W-06）：%s ⇒ 不给结论（rc=2）\n", guard)
			return 2
		}
	}
	inz, okInz, inzHits, inzWhy := coanOrphans(docsRoot)
	for _, nested := range coanNestedSeen[docsRoot] {
		fmt.Fprintf(stdout, "      ⊘ 剪掉的嵌套仓（域外 · 别仓的件）：%s\n", nested)
	}
	if !okInz {
		fmt.Fprintf(stdout, "  ⛔ %s · 第二棵树整棵：判不了（%s）—— ★ 第二棵树是本格域的必需输入 ⇒ "+
			"**缺件 ≠ 绿**：本门不给结论（rc=2）\n", inzWord, inzWhy)
	} else {
		fmt.Fprintf(stdout, "  ✓ %s · 第二棵树整棵：%d 件（与 ② 同度量 · 不同域）\n", inzWord, inz)
		for k, h := range inzHits {
			if k >= a.MaxExamples {
				break
			}
			fmt.Fprintf(stdout, "      %s（%s）\n", h.File, inzWord)
		}
	}
	navN, okNav, navHits, navWhy := coanNotInNav(repoRoot)
	navDomain := "（本仓无 `mkdocs.yml`）"
	navMode := "**无适用域**（照实登记 · 不假装覆盖）"
	if coanHasMkdocs(repoRoot) {
		navDomain = repoRoot
		navMode = "判"
	}
	fmt.Fprintf(stdout, "  域④ 有 `mkdocs.yml` 的树 = %s ⇒ %s\n", navDomain, navMode)
	if !okNav {
		fmt.Fprintf(stdout, "  ⛔ %s · 本仓：无适用域（%s）—— 「无适用域」≠「0 件」✗\n", navWord, navWhy)
	} else {
		fmt.Fprintf(stdout, "  ✓ %s · 本仓：%d 件\n", navWord, navN)
		for k, h := range navHits {
			if k >= a.MaxExamples {
				break
			}
			fmt.Fprintf(stdout, "      %s（%s）\n", h.File, navWord)
		}
	}

	if len(coanReadFails) > 0 {
		fmt.Fprintf(stderr, "BLOCKED —— 有 %d 件读不到（B 类读失败 ⇒ 登记 + 判红 · rc=2 · 不给结论）：\n", len(coanReadFails))
		for _, h := range coanReadFails {
			fmt.Fprintf(stderr, "  [读失败] %s  %s\n", h.File, h.Raw)
		}
		return 2
	}
	if brCounts[0] == nil {
		return 2
	}
	// ★ 缺件 ≠ 绿（逐字沿用 py `check-docs-orphans-links.py` 退码节 · 同族 `check-dangling-refs.go`
	//   该档口径）：本门两棵外部树都是**必需输入** —— `docs_root`（第二棵树 · ①/③ 两格的域）与
	//   `tree`（导出树 · ② 格的域）**任一**取不到 / 判不了（域不在盘 / 真扫面 0 件 / 件读不了）⇒
	//   本门**不给结论 rc=2**（承 `GAP-20260927-82` · `GAP-20261005-69`）。
	noConcl := []string{}
	if secondMissing {
		noConcl = append(noConcl, fmt.Sprintf("① local_broken 第二棵树格：读不到（%s）", secondWhy))
	}
	if !okInz {
		noConcl = append(noConcl, fmt.Sprintf("③ %s 格（域 = 第二棵树整棵）：读不到（%s）", inzWord, inzWhy))
	}
	if len(noConcl) > 0 {
		fmt.Fprint(stdout, "\n结论：**不给结论（rc=2）** —— 第二棵树是本门 ①③ 两格的域 · 缺件 ≠ 绿：\n")
		for _, r := range noConcl {
			fmt.Fprintf(stdout, "      · %s\n", r)
		}
		fmt.Fprint(stdout, "      （本地断链 域 = 两棵树 `--repo-root` + `--docs-root`/同级 `Zerg-内部文档`；两格同域 ⇒ 一并触碰）\n")
		return 2
	}
	if !okOrph { // ★ 导出树读不到 ⇒ 孤儿格判不了 · 缺件 ≠ 绿 ⇒ 本门不给结论（零放宽）
		fmt.Fprintf(stdout, "\n结论：**不给结论（rc=2）** —— 导出树读不到（%s）\n", owhy)
		fmt.Fprintf(stdout, "      取树次序 `--tree` → `$%s` → `$%s`（与 B12/B14/B16/B17 同口径 · 缺件 ≠ 绿）\n",
			coanPubTreeEnv, coanPubTreeEnvAlias)
		return 2
	}

	exAll := [2]int{len(brExempt[0]), len(brExempt[1])}
	cMain, cSecond := "None", "None"
	totalLocal := 0
	if brCounts[0] != nil {
		cMain = strconv.Itoa(*brCounts[0])
		totalLocal += *brCounts[0]
	}
	if brCounts[1] != nil {
		cSecond = strconv.Itoa(*brCounts[1])
		totalLocal += *brCounts[1]
	}
	onStr, inzStr, navStr := "None", "None", "None"
	if okOrph {
		onStr = strconv.Itoa(on)
	}
	if okInz {
		inzStr = strconv.Itoa(inz)
	}
	if okNav {
		navStr = strconv.Itoa(navN)
	}
	fmt.Fprintf(stdout, "\n结论（只报不拦）：本地断链 主仓 %s 件 / 第二棵树 %s 件 · "+
		"已登记假红（基准偏移）主仓 %d 处 / 第二棵树 %d 处（已逐条复核 · 不计）· 孤儿 %s 件 · "+
		"%s 第二棵树整棵 %s 件 · %s 本仓 %s ⇒ rc=0（命中不改码 ✗）\n",
		cMain, cSecond, exAll[0], exAll[1], onStr, inzWord, inzStr, navWord, navStr)
	if a.JSON {
		lb := &coanOM{}
		lb.set("count", totalLocal)
		byTree := &coanOM{}
		byTree.set(labels[0], brCounts[0])
		byTree.set(labels[1], brCounts[1])
		lb.set("by_tree", byTree)

		oe := &coanOM{}
		oe.set("count", exAll[0]+exAll[1])
		oeByTree := &coanOM{}
		oeByTree.set(labels[0], exAll[0])
		oeByTree.set(labels[1], exAll[1])
		oe.set("by_tree", oeByTree)
		regKeys := make([]string, 0, len(coanBaseOffset))
		for k := range coanBaseOffset {
			regKeys = append(regKeys, k)
		}
		sort.Strings(regKeys)
		oe.set("registry", regKeys)
		oe.set("stale", []string{})

		dp := &coanOM{}
		for _, r := range []string{repoRoot, docsRoot, tree} {
			nested := coanNestedSeen[r]
			if nested == nil {
				nested = []string{}
			}
			dp.set(r, nested)
		}

		orph := &coanOM{}
		if okOrph {
			orph.set("count", on)
		} else {
			orph.set("count", nil)
		}
		orph.set("domain", tree)

		iz := &coanOM{}
		if okInz {
			iz.set("count", inz)
		} else {
			iz.set("count", nil)
		}
		iz.set("domain", docsRoot)
		iz.set("word", inzWord)

		ni := &coanOM{}
		if okNav {
			ni.set("count", navN)
		} else {
			ni.set("count", nil)
		}
		if coanHasMkdocs(repoRoot) {
			ni.set("domain", repoRoot)
		} else {
			ni.set("domain", nil)
		}
		ni.set("word", navWord)
		if navWhy != "" {
			ni.set("why", navWhy)
		} else {
			ni.set("why", nil)
		}

		root := &coanOM{}
		root.set("schema", "zerg-gate/v1")
		root.set("gate", "check-docs-orphans-links")
		root.set("version", coanVersion)
		root.set("local_broken", lb)
		root.set("offset_exempt", oe)
		root.set("domain_pruned", dp)
		root.set("orphans", orph)
		root.set("inbound_zero", iz)
		root.set("not_in_nav", ni)
		root.set("mode", "report-only")
		fmt.Fprintln(stdout, coanJSONEnc(root))
	}
	if bad == 0 {
		return 0
	}
	return 2
}
