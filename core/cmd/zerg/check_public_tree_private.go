// check_public_tree_private.go —— 判据件
// `scripts/gates/check-public-tree-private.py`（430 行 · sha16 667d1bff503673de）的 **Go 真搬**
// （M2 批 · 甲案口径 · 公开面隐私件）。
//
// 本件做什么（= py `__main__` 的主逻辑）：
//
//	发布侧门禁：拿 `publish/private-paths.txt` 的**路径黑名单**（支持 `!` 豁免）与一棵「公开树」
//	逐条做包含判定 —— 凡黑名单路径出现在公开树里即逐条打印并 rc=1。第二道是**正向断言**（默认开）：
//	公开树 `docs/` 下每一条路径都必须落在「正式面 docs/zh|docs/en」或「overlay 映射目标」里，
//	否则 rc=1；`docs/` 空转或合法集读不出 ⇒ rc=2（不给结论，绝不静默放行）。
//
// 退码：0 = 通过；1 = 有命中；2 = 不给结论（无树 / 目录不存在 / 合法集读不出 / docs 空转 / 用法错）。
//
// ── 入口签名 ─────────────────────────────────────────────────────────────────
//
//	func checkPublicTreePrivateGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv —— 与 py `sys.argv[1:]` 同形（不含程序名）
//	  · 退码 0/1/2 与输出与 py 逐字节对齐（未覆盖面见下）
//
// ── 子进程面（一律现调 git，与 py 同名同参）──────────────────────────────────
//
//	git -C <repo> ls-files -z        （_published_sources 首选；失败 ⇒ 回落 overlay 目录递归）
//
//	自检面：**不起子进程** —— py 的 _run 会重起 `sys.executable <__file__>`，本件改为**同进程**
//	调用 `checkPublicTreePrivateGateMain`（复刻「真命令行 + 真退出码」，输出/退码逐字对齐）。
//
// ── 外部真源真身路径（全部 REPO 相对 · REPO = 仓根）───────────────────────────
//
//	publish/private-paths.txt                      黑名单（本件负向闸的判据真源）
//	publish/mirror-public-lib.py                   镜像器真身：map_path 的**算法**重落，
//	                                               DROP_EXACT/DROP_PREFIX/MAP_EXACT 三个**数据表现读**
//	publish/whitelist.txt                          发布白名单（published_path 的真源）
//	scripts/build/publish-public.sh                EXCLUDES=( ... ) 数组（load_excludes 的真源）
//	scripts/gates/check-history-secrets.py         发布侧谓词真身：published_path/load_whitelist/
//	                                               load_excludes/excluded 的**算法**重落
//	publish/docs/                                  overlay 源目录（git 不可用时的回落遍历面）
//
// ── 输出序的决定者（★ 逐点交代）──────────────────────────────────────────────
//
//	· 负向闸命中明细（`hits`）：外层 = **os.walk 产出序**（= 每层 readdir 原序；本件用
//	  `os.Open` + `ReadDir(-1)` 保原序，且**先出本层全部文件再按原序下钻** —— 复刻 os.walk
//	  的「yield 完当前层文件才下钻」），内层 = `dirs` 黑名单**声明序**（一条路径命中第一条即
//	  `break`）。明细只取前 40 项（py `hits[:40]`）。
//	· 正向断言非法明细（`illegal`）：`sorted(illegal)` → 码点序 ⟷ Go `sort.Strings`。
//	· `docs/allowlist.exact` 是**集合**（只剩 `len()` 进输出）⇒ 遍历序不影响输出行序。
//
// ── 标准声明 (a)：默认根定位差异 ──────────────────────────────────────────────
//
//	py 的 REPO = `dirname^3(abspath(__file__))`（恒 = 仓根，与 cwd 无关）；Go 无 `__file__`
//	语境 ⇒ 默认根从 **cwd 上溯**找仓根标记 `core/internal/version/version.go`，找不到则取 cwd。
//	⇒ 「脚本位置 ≠ cwd」时两层路径推导面不同。待检查的公开树是**位置参数**（argv），与根无关
//	⇒ 对拍一律显式传目标树。
//
// ── 标准声明 (b)：自指检查 ───────────────────────────────────────────────────
//
//	本门的**扫描面 = 目标公开树的路径名**（不是本件正文，也不是 Go 模块目录）⇒ 本件正文对
//	本门读数贡献恒 0（本件只在**目标树里含同名件**时才作为一个「路径」被扫，而那要求有人把
//	本件摆进待检树）。其余他门自指面逐条核：
//	  ① 本件零私有**绝对**路径字面量（路径一律由 argv / REPO 相对串传出）；
//	  ② 本件不打印任何闭集 kind 字面量；
//	  ③ 自检样例里的非法路径一律**拼串构造**（✗ 不在源码里留完整原串），本门私有字面量 0 处；
//	  ④ `--self-test` 是本门旗标所需的连续字面（与同批已落件同为既定面）。
//
// ── 未覆盖面（✗ 不声称逐字对齐）──────────────────────────────────────────────
//
//  1. `--json`：本门无机器读面（py `JSON_FIELDS = []`）⇒ 给了即逐字拒收 rc=2（与 py 同）。
//  2. argparse 的**用法/帮助正文**已逐字复刻（见 cptpUsage/cptpHelp）；未知旗标与缺值档亦复刻。
//  3. C 类真口子（最外层无兜底）：py `__main__` 裸跑 ⇒ 黑名单件读不了 = 回溯 + rc=1；本件改
//     为 fail-closed rc=2 + 具名文案（新口径）；该格只对齐 rc 之外不保。
//  4. `git ls-files` 输出**非 UTF-8** 时 py `text=True` 抛 UnicodeDecodeError ⇒ 回落；本件按
//     字节收，不做同档替换（真仓路径均合法 UTF-8 ⇒ 通常不触发）。
//  5. `os.getcwd()` ⟷ `os.Getwd()`：`$PWD` 为符号链接路径时两者可能不同（本件夹具一律绝对路径
//     + 非链接落点 ⇒ 不触发）。
//  6. `str.strip()` 系 py Unicode 语义（含 \x1c-\x1f · \x85 · \xa0），本件自写 `cptpPyStrip` 同构。
//  7. overlay 回落档（git 不可用）需 git 真缺席才可达 ⇒ 只声明口径、未实测。
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// ── py 语义基元（★ 一律自写，✗ 不用 filepath 家族）────────────────────────────

// cptpPyIsSpace = py `str.isspace()` 的空白集：Unicode White_Space ∪ U+001C-U+001F。
// ★ Go `unicode.IsSpace` ✗ 不含 \x1c-\x1f（血证级差异）⇒ 单独补上。
func cptpPyIsSpace(r rune) bool {
	if r >= 0x1c && r <= 0x1f {
		return true
	}
	return unicode.IsSpace(r)
}

// cptpPyStrip = py `str.strip()`（无参版）。
func cptpPyStrip(s string) string { return strings.TrimFunc(s, cptpPyIsSpace) }

// cptpPyStripSet = py `str.strip(chars)`：剥两端落在 chars 集内的字符。
func cptpPyStripSet(s, chars string) string {
	return strings.TrimFunc(s, func(r rune) bool { return strings.ContainsRune(chars, r) })
}

// cptpPyJoin = CPython posixpath.join 逐段规则（★ 三语义差：中间 `..` 原样留 · 后段绝对丢前段 ·
// 空段保留尾 `/`）。✗ 不用 filepath.Join。
func cptpPyJoin(parts ...string) string {
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

// cptpPyNormpath = CPython posixpath.normpath（★ 保留恰两个前导斜杠 `//a` 不折成 `/a`）。
func cptpPyNormpath(path string) string {
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

// cptpPyAbspath = CPython posixpath.abspath（纯词法 · cwd 由调用方传入）。
func cptpPyAbspath(cwd, path string) string { return cptpPyNormpath(cptpPyJoin(cwd, path)) }

// cptpPyRelpath = CPython posixpath.relpath（path 为空 ⇒ 返回 ""，由调用处处置）。
func cptpPyRelpath(cwd, path, start string) string {
	if path == "" {
		return ""
	}
	split := func(p string) []string {
		var out []string
		for _, c := range strings.Split(p, "/") {
			if c != "" {
				out = append(out, c)
			}
		}
		return out
	}
	s, p := split(cptpPyAbspath(cwd, start)), split(cptpPyAbspath(cwd, path))
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

// cptpPyBasename = posixpath.basename（剥尾斜杠后取最后一段）。
func cptpPyBasename(p string) string {
	_, tail := cptpPySplit(p)
	return tail
}

// cptpPySplit = posixpath.split。
func cptpPySplit(p string) (string, string) {
	i := strings.LastIndex(p, "/") + 1
	head, tail := p[:i], p[i:]
	if head != "" && strings.Trim(head, "/") != "" {
		head = strings.TrimRight(head, "/")
	}
	return head, tail
}

// cptpPyTail = py `s[-n:]`（按**码点**取尾 n 个，✗ 不按字节切）。
func cptpPyTail(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[len(r)-n:])
}

// cptpNormalizeNewlines = py 文本模式 universal newlines（\r\n 与裸 \r 均折成 \n）。
func cptpNormalizeNewlines(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}

// ── 文件系统谓词（py os.path 语义：stat 跟随符号链接）─────────────────────────

func cptpExists(p string) bool { _, err := os.Stat(p); return err == nil }

func cptpIsDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

func cptpIsLink(p string) bool {
	fi, err := os.Lstat(p)
	return err == nil && fi.Mode()&os.ModeSymlink != 0
}

// ── 黑名单规则：load_rules / is_allowed / scan ───────────────────────────────

type cptpRules struct {
	dirs    []string
	files   []string
	allowed []string
}

// cptpLoadRules = py load_rules：逐行 strip；空行 / `#` 跳过；`!` 前缀 = 豁免；尾 `/` = 目录；
// 其余 = 文件。
//
// ★ 行口径**已抽到共享件** `private_face_rules.go` 的 `parsePrivateFaceRules`（缺口
// `GAP-20260927-414`：此前本件与 `family_publish_set.go` 的 `publishSetPrivateFaceRules` 各镜像
// 一份同型解析 ⇒ 真源一改两处各漂）。本件仍**自己**做 py 语义的读件 / 换行归一 / strip
// （`cptpPyStrip` —— 与 Go `strings.TrimSpace` 对 Unicode 空白覆盖面不同），只把「剥好之后怎么
// 分三档」交给共享件（strip 函数由本件传入 ⇒ py 语义不丢）。
func cptpLoadRules(path string) (cptpRules, error) {
	var r cptpRules
	b, err := os.ReadFile(path)
	if err != nil {
		return r, err
	}
	r.dirs, r.files, r.allowed = parsePrivateFaceRules(
		strings.Split(cptpNormalizeNewlines(string(b)), "\n"), cptpPyStrip)
	return r, nil
}

// cptpIsAllowed = py is_allowed。唯一实现已抽到共享件 `private_face_rules.go` 的
// `privateFaceAllowed`（缺口 `GAP-20260927-414`）。保留本名与签名 = 既有调用点（本件 `cptpScan`
// · `check_public_tree_private_test.go`）**一字不动**。
func cptpIsAllowed(rel string, allowed []string) bool {
	return privateFaceAllowed(rel, allowed)
}

type cptpHit struct {
	rel  string
	rule string
}

// cptpScan = py scan：os.walk 原序（每层 readdir 原序 · 先文件后按原序下钻 · 不下钻符号链接），
// 跳过含 `/.git` 的目录路径；命中取 `dirs` 声明序首条（break），否则整串精确命中 `files`。
func cptpScan(cwd, tree string, dirs, files, allowed []string) []cptpHit {
	var hits []cptpHit
	var walk func(dirpath string)
	walk = func(dirpath string) {
		if strings.Contains(dirpath, "/.git") || strings.HasSuffix(dirpath, "/.git") {
			return
		}
		fh, err := os.Open(dirpath)
		if err != nil {
			return
		}
		ents, err := fh.ReadDir(-1)
		_ = fh.Close()
		if err != nil {
			return
		}
		var subdirs []string
		for _, e := range ents {
			full := cptpPyJoin(dirpath, e.Name())
			if cptpIsDir(full) { // ★ stat 语义（跟随符号链接）—— py entry.is_dir() 同
				subdirs = append(subdirs, full)
				continue
			}
			rel := strings.ReplaceAll(cptpPyRelpath(cwd, full, tree), string(os.PathSeparator), "/")
			if cptpIsAllowed(rel, allowed) {
				continue
			}
			matched := false
			for _, d := range dirs {
				if strings.HasPrefix(rel, d) {
					hits = append(hits, cptpHit{rel: rel, rule: d})
					matched = true
					break
				}
			}
			if matched {
				continue
			}
			for _, x := range files {
				if rel == x {
					hits = append(hits, cptpHit{rel: rel, rule: rel})
					break
				}
			}
		}
		for _, sd := range subdirs {
			if cptpIsLink(sd) { // os.walk(followlinks=False)：符号链接目录不下钻
				continue
			}
			walk(sd)
		}
	}
	walk(tree)
	return hits
}

// ── 正向断言：docs/ 合法集现读 + 扫描 ─────────────────────────────────────────

var cptpFormalFacePrefixes = []string{"docs/zh/", "docs/en/"}

type cptpAllow struct {
	prefixes   []string
	exact      map[string]bool
	prov       map[string]string
	sourceMode string
	err        string
}

func cptpHasDef(src, name string) bool { return strings.Contains(src, "def "+name+"(") }

// cptpExtractBlock 取 `<open>` 之后、到首个 `\n<close>` 之前的块（复刻 py 常量表落点）。
func cptpExtractBlock(src, open, close string) (string, bool) {
	i := strings.Index(src, open)
	if i < 0 {
		return "", false
	}
	rest := src[i+len(open):]
	j := strings.Index(rest, close)
	if j < 0 {
		return "", false
	}
	return rest[:j], true
}

// cptpPyStringLiterals 从一段 py 源码块里抽出全部**双引号字符串字面量**（跳过 `#` 注释；
// 认 `\` 转义）。用于现读 DROP_EXACT / DROP_PREFIX 两个数据表。
func cptpPyStringLiterals(block string) []string {
	var out []string
	var cur strings.Builder
	inStr := false
	for i := 0; i < len(block); i++ {
		c := block[i]
		if inStr {
			if c == '\\' && i+1 < len(block) {
				cur.WriteByte(c)
				i++
				cur.WriteByte(block[i])
				continue
			}
			if c == '"' {
				inStr = false
				out = append(out, cur.String())
				cur.Reset()
				continue
			}
			cur.WriteByte(c)
			continue
		}
		if c == '"' {
			inStr = true
			continue
		}
		if c == '#' {
			for i < len(block) && block[i] != '\n' {
				i++
			}
		}
	}
	return out
}

// cptpParseStrSet 现读 `NAME = { ... }` 的字符串集合。
func cptpParseStrSet(src, name string) map[string]bool {
	set := map[string]bool{}
	blk, ok := cptpExtractBlock(src, name+" = {", "\n}")
	if !ok {
		return set
	}
	for _, s := range cptpPyStringLiterals(blk) {
		set[s] = true
	}
	return set
}

// cptpParseStrTuple 现读 `NAME = ( ... )` 的字符串元组（保声明序）。
func cptpParseStrTuple(src, name string) []string {
	blk, ok := cptpExtractBlock(src, name+" = (", "\n)")
	if !ok {
		return nil
	}
	return cptpPyStringLiterals(blk)
}

// cptpParseStrMap 现读 `NAME = { "k": "v", ... }`（逐行取对）。
func cptpParseStrMap(src, name string) map[string]string {
	m := map[string]string{}
	blk, ok := cptpExtractBlock(src, name+" = {", "\n}")
	if !ok {
		return m
	}
	re := regexp.MustCompile(`"([^"]*)"\s*:\s*"([^"]*)"`)
	for _, line := range strings.Split(blk, "\n") {
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = line[:i]
		}
		if mm := re.FindStringSubmatch(line); mm != nil {
			m[mm[1]] = mm[2]
		}
	}
	return m
}

// cptpLoadWhitelist = hsec.load_whitelist（逐行 strip；空 / `#` 跳过）。
func cptpLoadWhitelist(path string) []string {
	var out []string
	b, err := os.ReadFile(path)
	if err != nil {
		return out
	}
	for _, raw := range strings.Split(cptpNormalizeNewlines(string(b)), "\n") {
		line := cptpPyStrip(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	return out
}

// cptpLoadExcludes = hsec.load_excludes：解析 publish-public.sh 的 `EXCLUDES=( ... \n)` 数组。
func cptpLoadExcludes(path string) []string {
	var ex []string
	if !cptpExists(path) {
		return ex
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return ex
	}
	re := regexp.MustCompile(`(?s)EXCLUDES=\((.*?)\n\)`)
	m := re.FindStringSubmatch(string(b))
	if m == nil {
		return ex
	}
	for _, raw := range strings.Split(m[1], "\n") {
		line := cptpPyStrip(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = cptpPyStrip(strings.Split(line, "#")[0])
		line = cptpPyStripSet(line, `"`)
		line = cptpPyStripSet(line, `'`)
		if line != "" {
			ex = append(ex, line)
		}
	}
	return ex
}

// cptpPublishedPath = hsec.published_path。
func cptpPublishedPath(rel string, entries []string) bool {
	for _, e := range entries {
		if strings.HasSuffix(e, "/") {
			if strings.HasPrefix(rel, e) {
				return true
			}
		} else if rel == e {
			return true
		}
	}
	return false
}

// cptpExcluded = hsec.excluded（噪声后缀 · __pycache__ 段 · EXCLUDES 目录/精确项）。
func cptpExcluded(rel string, ex []string) bool {
	if strings.HasSuffix(rel, ".bak") || strings.HasSuffix(rel, ".orig") || strings.HasSuffix(rel, ".rej") {
		return true
	}
	for _, part := range strings.Split(rel, "/") {
		if part == "__pycache__" {
			return true
		}
	}
	for _, e := range ex {
		if rel == e || strings.HasPrefix(rel, strings.TrimRight(e, "/")+"/") {
			return true
		}
	}
	return false
}

func cptpHasPrefixAny(s string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

// cptpMapPath = 镜像器 map_path 的**算法**重落（数据表 DROP_*/MAP_EXACT 由上读现读）。"" = 不进公开面。
func cptpMapPath(rel string, dropExact map[string]bool, dropPrefix []string, mapExact map[string]string) string {
	if dropExact[rel] || cptpHasPrefixAny(rel, dropPrefix) {
		return ""
	}
	if v, ok := mapExact[rel]; ok {
		return v
	}
	if rel == "docs/skills/tool-upgrade.md" {
		return "docs/design/工具升级规范.md"
	}
	if strings.HasPrefix(rel, "publish/docs/design/") {
		return "docs/design/" + cptpPyBasename(rel)
	}
	if strings.HasPrefix(rel, "publish/docs/") {
		b := cptpPyBasename(rel)
		if b == "README.md" || b == "README.zh-CN.md" {
			return b
		}
		return "docs/" + b
	}
	if strings.HasPrefix(rel, "publish/ci/") {
		return ".github/workflows/" + cptpPyBasename(rel)
	}
	if strings.HasPrefix(rel, "publish/") {
		return ""
	}
	return rel
}

// cptpPublishedSources = py _published_sources：git ls-files -z 为源；失败回落 overlay 递归。
func cptpPublishedSources(cwd, repo, overlayDir string) ([]string, string) {
	if out, err := exec.Command("git", "-C", repo, "ls-files", "-z").Output(); err == nil {
		var rels []string
		for _, r := range strings.Split(string(out), "\x00") {
			if r != "" {
				rels = append(rels, r)
			}
		}
		if len(rels) > 0 {
			return rels, "git ls-files"
		}
	}
	var rels []string
	if cptpIsDir(overlayDir) {
		var walk func(dirpath string)
		walk = func(dirpath string) {
			fh, err := os.Open(dirpath)
			if err != nil {
				return
			}
			ents, err := fh.ReadDir(-1)
			_ = fh.Close()
			if err != nil {
				return
			}
			var subdirs []string
			for _, e := range ents {
				full := cptpPyJoin(dirpath, e.Name())
				if cptpIsDir(full) {
					if e.Name() == ".git" {
						continue
					}
					subdirs = append(subdirs, full)
					continue
				}
				rels = append(rels, strings.ReplaceAll(cptpPyRelpath(cwd, full, repo), string(os.PathSeparator), "/"))
			}
			for _, sd := range subdirs {
				if cptpIsLink(sd) {
					continue
				}
				walk(sd)
			}
		}
		walk(overlayDir)
	}
	sort.Strings(rels)
	return rels, "publish/docs 递归（git 不可用）"
}

// cptpDocsAllowlist = py docs_allowlist：现读「公开树 docs/ 下合法集」。
func cptpDocsAllowlist(cwd, repo string) cptpAllow {
	return cptpDocsAllowlistWith(cwd, repo, "")
}

// cptpDocsAllowlistWith = py `docs_allowlist()`（`publishShOverride` ≠ "" ⇒ 取代 publish-public.sh 路径 ·
//
//	等价 py 自检 ⑩ 里临时改全局 `PUBLISH_SH`）。
func cptpDocsAllowlistWith(cwd, repo, publishShOverride string) cptpAllow {
	base := cptpAllow{prefixes: cptpFormalFacePrefixes, exact: map[string]bool{},
		prov: map[string]string{}, sourceMode: "-"}
	mk := func(rel string) string { return cptpPyJoin(repo, rel) }
	mirrorLib := mk("publish/mirror-public-lib.py")
	whitelist := mk("publish/whitelist.txt")
	publishSh := mk("scripts/build/publish-public.sh")
	if publishShOverride != "" { // 自检 ⑩ 注入（py 改全局 PUBLISH_SH 的同构面）
		publishSh = publishShOverride
	}
	historySecrets := mk("scripts/gates/check-history-secrets.py")
	overlayDir := mk("publish/docs")
	for _, p := range []string{mirrorLib, whitelist, publishSh, historySecrets} {
		if !cptpExists(p) {
			base.err = "缺件：" + cptpPyRelpath(cwd, p, repo)
			return base
		}
	}
	mlibB, err1 := os.ReadFile(mirrorLib)
	hsecB, err2 := os.ReadFile(historySecrets)
	if err1 != nil || err2 != nil {
		base.err = "加载失败：镜像器或历史扫描器读不进"
		return base
	}
	ms, hs := string(mlibB), string(hsecB)
	if !cptpHasDef(ms, "map_path") {
		base.err = "镜像器映射真身读不出（无 map_path）"
		return base
	}
	for _, fn := range []string{"published_path", "load_whitelist", "load_excludes", "excluded"} {
		if !cptpHasDef(hs, fn) {
			base.err = "发布侧谓词读不出（无 " + fn + "）"
			return base
		}
	}
	entries := cptpLoadWhitelist(whitelist)
	excludes := cptpLoadExcludes(publishSh)
	if len(excludes) == 0 { // ★ 对齐 py `docs_allowlist` L158-161（GAP-20260927-192）：
		//   EXCLUDES 块读不出 / 缺件 ⇒ 0 条活跃 ⇒ **不给结论**（fail-closed · Go 半此前无此守卫）。
		base.err = "EXCLUDES 真源解析失配 / 0 条活跃（EXCLUDES=( … ) 块读不出或缺件）⇒ 不给结论 · GAP-20260927-192"
		return base
	}
	dropExact := cptpParseStrSet(ms, "DROP_EXACT")
	dropPrefix := cptpParseStrTuple(ms, "DROP_PREFIX")
	mapExact := cptpParseStrMap(ms, "MAP_EXACT")
	rels, mode := cptpPublishedSources(cwd, repo, overlayDir)
	exact := map[string]bool{}
	prov := map[string]string{}
	for _, rel := range rels {
		if !cptpPublishedPath(rel, entries) {
			continue
		}
		if cptpExcluded(rel, excludes) || dropExact[rel] || cptpHasPrefixAny(rel, dropPrefix) {
			continue
		}
		mp := cptpMapPath(rel, dropExact, dropPrefix, mapExact)
		if mp != "" && strings.HasPrefix(mp, "docs/") {
			exact[mp] = true
			prov[mp] = rel
		}
	}
	return cptpAllow{prefixes: cptpFormalFacePrefixes, exact: exact, prov: prov, sourceMode: mode}
}

// cptpDocsFaceScan = py docs_face_scan：docs/ 下全部文件 → (非法排序列表, 扫描计数)。
func cptpDocsFaceScan(cwd, tree string, allow cptpAllow) ([]string, int) {
	root := cptpPyJoin(tree, "docs")
	if !cptpIsDir(root) {
		return nil, 0
	}
	var illegal []string
	scanned := 0
	var walk func(dirpath string)
	walk = func(dirpath string) {
		fh, err := os.Open(dirpath)
		if err != nil {
			return
		}
		ents, err := fh.ReadDir(-1)
		_ = fh.Close()
		if err != nil {
			return
		}
		var subdirs []string
		for _, e := range ents {
			full := cptpPyJoin(dirpath, e.Name())
			if cptpIsDir(full) {
				if e.Name() == ".git" {
					continue
				}
				subdirs = append(subdirs, full)
				continue
			}
			rel := strings.ReplaceAll(cptpPyRelpath(cwd, full, tree), string(os.PathSeparator), "/")
			scanned++
			if allow.exact[rel] || cptpHasPrefixAny(rel, allow.prefixes) {
				continue
			}
			illegal = append(illegal, rel)
		}
		for _, sd := range subdirs {
			if cptpIsLink(sd) {
				continue
			}
			walk(sd)
		}
	}
	walk(root)
	sort.Strings(illegal)
	return illegal, scanned
}

// ── JSON 拒收（缺口 Q-176：本门无机器读面）────────────────────────────────────

var cptpJSONFields []string // 本门机器读面字段表（空 = 无）

func cptpRefuseJSON(argv []string, stderr io.Writer) (int, bool) {
	var hit []string
	for _, x := range argv {
		if x == "--json" || strings.HasPrefix(x, "--json=") {
			hit = append(hit, x)
		}
	}
	if len(hit) == 0 {
		return 0, false
	}
	qs := make([]string, 0, len(hit))
	for _, h := range hit {
		qs = append(qs, "`"+h+"`")
	}
	fmt.Fprintf(stderr, "⛔ 本门无机器读面：%s 在本门**未实现**（缺口 Q-176）\n", strings.Join(qs, " · "))
	if len(cptpJSONFields) > 0 {
		fs := make([]string, 0, len(cptpJSONFields))
		for _, f := range cptpJSONFields {
			fs = append(fs, "`"+f+"`")
		}
		fmt.Fprintf(stderr, "   字段表：%s\n", strings.Join(fs, " · "))
	} else {
		fmt.Fprint(stderr, "   字段表：无（本门只出人读面）\n")
	}
	fmt.Fprint(stderr, "⇒ 用法错（rc=2）：**机器面缺 ≠ 零命中** —— 两态不许同形\n")
	return 2, true
}

// ── argparse 用法/帮助正文（逐字复刻）────────────────────────────────────────

const cptpProg = "check-public-tree-private.py"

const cptpUsage = "usage: " + cptpProg + " [-h] [--rules RULES] [--docs-allowlist]\n" +
	"                                    [--no-docs-allowlist] [--self-test]\n" +
	"                                    [tree]\n"

const cptpHelp = cptpUsage + "\n" +
	"positional arguments:\n" +
	"  tree                 待检查的公开树目录\n" +
	"\n" +
	"optional arguments:\n" +
	"  -h, --help           show this help message and exit\n" +
	"  --rules RULES\n" +
	"  --docs-allowlist     正向断言：公开树 docs/ 下只允许正式面 + overlay 映射目标（**默认开**）\n" +
	"  --no-docs-allowlist  关掉正向断言，只跑负向黑名单闸（自检的成对负控用；发布链路不要关）\n" +
	"  --self-test          跑成对负控自检（合成夹具，不碰真仓、不判真目标）\n"

func cptpArgErr(stderr io.Writer, msg string) int {
	fmt.Fprint(stderr, cptpUsage)
	fmt.Fprintf(stderr, "%s: error: %s\n", cptpProg, msg)
	return 2
}

// ── 根定位 ───────────────────────────────────────────────────────────────────

// cptpRepoOverride —— 仅供自检面把「同进程的根」钉到 REPO（py 的 _run 用 cwd=REPO ＋ __file__ 推导）。
var cptpRepoOverride string

// cptpResolveRoot = 默认仓根：先 cwd 上溯找仓根标记，找不到取 cwd（声明 (a)）。
func cptpResolveRoot() string {
	if cptpRepoOverride != "" {
		return cptpRepoOverride
	}
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	dir := cwd
	for {
		if cptpExists(cptpPyJoin(dir, "core", "internal", "version", "version.go")) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return cwd
}

// cptpGlobPublicTrees = py glob.glob("/tmp/zerg-public-*") 按 mtime 降序（最新在前）。
func cptpGlobPublicTrees() []string {
	ents, err := os.ReadDir("/tmp")
	if err != nil {
		return nil
	}
	type cand struct {
		path  string
		mtime int64
	}
	var cs []cand
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), "zerg-public-") {
			p := cptpPyJoin("/tmp", e.Name())
			if fi, err := os.Stat(p); err == nil {
				cs = append(cs, cand{p, fi.ModTime().UnixNano()})
			}
		}
	}
	sort.SliceStable(cs, func(i, j int) bool { return cs[i].mtime > cs[j].mtime })
	out := make([]string, 0, len(cs))
	for _, c := range cs {
		out = append(out, c.path)
	}
	return out
}

// ── 主入口 ───────────────────────────────────────────────────────────────────

// checkPublicTreePrivateGateMain = 本判据的 Go 入口（argv 不含程序名）。
func checkPublicTreePrivateGateMain(stdout, stderr io.Writer, argv []string) int {
	if rc, hit := cptpRefuseJSON(argv, stderr); hit {
		return rc
	}
	root := cptpResolveRoot()
	cwd, _ := os.Getwd()

	tree := ""
	treeGiven := false
	rules := cptpPyJoin(root, "publish", "private-paths.txt")
	docsAllow := true
	selfTest := false
	i := 0
	for i < len(argv) {
		a := argv[i]
		val := func() (string, bool) {
			if eq := strings.IndexByte(a, '='); eq >= 0 {
				return a[eq+1:], true
			}
			if i+1 < len(argv) {
				i++
				return argv[i], true
			}
			return "", false
		}
		switch {
		case a == "-h" || a == "--help":
			fmt.Fprint(stdout, cptpHelp)
			return 0
		case a == "--rules" || strings.HasPrefix(a, "--rules="):
			v, ok := val()
			if !ok {
				return cptpArgErr(stderr, "argument --rules: expected one argument")
			}
			rules = v
		case a == "--docs-allowlist":
			docsAllow = true
		case a == "--no-docs-allowlist":
			docsAllow = false
		case a == "--self-test":
			selfTest = true
		case strings.HasPrefix(a, "-") && a != "-":
			return cptpArgErr(stderr, "unrecognized arguments: "+a)
		default:
			if treeGiven {
				return cptpArgErr(stderr, "unrecognized arguments: "+a)
			}
			tree = a
			treeGiven = true
		}
		i++
	}

	if selfTest {
		return cptpSelfTest(stdout, root)
	}

	if tree == "" {
		cands := cptpGlobPublicTrees()
		if len(cands) == 0 {
			fmt.Fprint(stdout, "✗ 没有指定公开树，也找不到 /tmp/zerg-public-*\n")
			return 2
		}
		tree = cands[0]
	}
	if !cptpIsDir(tree) {
		fmt.Fprintf(stdout, "✗ 目录不存在：%s\n", tree)
		return 2
	}

	r, err := cptpLoadRules(rules)
	if err != nil {
		// C 类真口子：py 裸回溯 rc=1 ⇒ 本件 fail-closed rc=2（新口径 · 见未覆盖面 3）。
		fmt.Fprintf(stderr, "⚠ 黑名单件读不了（%s）⇒ 不给结论（rc=2）\n", cptpPyRelpath(cwd, rules, root))
		return 2
	}
	hits := cptpScan(cwd, tree, r.dirs, r.files, r.allowed)

	fmt.Fprintf(stdout, "公开树：%s\n", tree)
	fmt.Fprintf(stdout, "黑名单：%d 目录 + %d 文件 + %d 豁免（%s）\n",
		len(r.dirs), len(r.files), len(r.allowed), cptpPyRelpath(cwd, rules, root))
	bad := 0
	if len(hits) > 0 {
		bad = 1
		fmt.Fprintf(stdout, "\n✗ 命中私有面 %d 处 —— 发布必须中止（一个字节都不推）：\n", len(hits))
		n := len(hits)
		if n > 40 {
			n = 40
		}
		for _, h := range hits[:n] {
			fmt.Fprintf(stdout, "    %-60s ← 规则 %s\n", h.rel, h.rule)
		}
		if len(hits) > 40 {
			fmt.Fprintf(stdout, "    …（其余 %d 处省略）\n", len(hits)-40)
		}
	}

	if !docsAllow {
		fmt.Fprint(stdout, "正向断言：已关闭（--no-docs-allowlist）⇒ 本次只判负向黑名单\n")
	} else {
		allow := cptpDocsAllowlist(cwd, root)
		if allow.err != "" {
			fmt.Fprintf(stdout, "\n⚠ 正向断言判不了（%s）⇒ **不给结论**（rc=2，不许当作通过）\n", allow.err)
			if bad != 0 {
				return 1
			}
			return 2
		}
		illegal, scanned := cptpDocsFaceScan(cwd, tree, allow)
		fmt.Fprintf(stdout, "正向断言：合法集 = 正式面 %s + overlay 映射目标 %d 条（现读：%s）\n",
			strings.Join(allow.prefixes, "·"), len(allow.exact), allow.sourceMode)
		if scanned == 0 {
			fmt.Fprint(stdout, "\n⚠ 空转：公开树 docs/ 下扫到 0 个文件（缺 docs/ 或目录为空）⇒ **不给结论**（rc=2，宁可拦下也不当作通过）\n")
			if bad != 0 {
				return 1
			}
			return 2
		}
		if len(illegal) > 0 {
			bad = 1
			fmt.Fprintf(stdout, "\n✗ docs/ 下 %d 处不在合法集内 —— 发布必须中止（正向断言，rc=1）：\n", len(illegal))
			n := len(illegal)
			if n > 40 {
				n = 40
			}
			for _, rel := range illegal[:n] {
				fmt.Fprintf(stdout, "    %s\n", rel)
			}
			if len(illegal) > 40 {
				fmt.Fprintf(stdout, "    …（其余 %d 处省略）\n", len(illegal)-40)
			}
			fmt.Fprint(stdout, "    （合法来源只有两个：正式面 docs/zh|docs/en；或 publish/docs/** 经映射落过来的那些）\n")
		}
	}

	if bad != 0 {
		return 1
	}
	if docsAllow {
		fmt.Fprint(stdout, "\n结果: 通过——公开树未出现任何私有面路径，且 docs/ 下全部来自合法来源\n")
	} else {
		fmt.Fprint(stdout, "\n结果: 通过——公开树未出现任何私有面路径（本次未判正向断言）\n")
	}
	return 0
}

// ── 成对负控自检（--self-test · 合成夹具 · 尊重 TMPDIR · 跑完即清）──────────────

// cptpPyDirname = posixpath.dirname。
func cptpPyDirname(p string) string { h, _ := cptpPySplit(p); return h }

// cptpPyBool = py `%s` 上的 True/False 字面（✗ Go `%v` 给 true/false）。
func cptpPyBool(b bool) string {
	if b {
		return "True"
	}
	return "False"
}

var cptpCleanSample = []string{"docs/zh/x.md", "docs/en/x.md"}

var cptpSampleBase = "自检样例.md"

// cptpSampleFor = py sample_for：目录规则 ⇒ 它下面一个文件；文件规则 ⇒ 它自己。
func cptpSampleFor(rule string) string {
	if strings.HasSuffix(rule, "/") {
		return rule + cptpSampleBase
	}
	return rule
}

// cptpWriteTree = py _write_tree（合成夹具内容，✗ 不含任何真内容）。
func cptpWriteTree(root string, rels []string) {
	for _, rel := range rels {
		p := cptpPyJoin(append([]string{root}, strings.Split(rel, "/")...)...)
		_ = os.MkdirAll(cptpPyDirname(p), 0o755)
		_ = os.WriteFile(p, []byte("自检夹具（合成，非真内容）\n"), 0o644)
	}
}

func cptpWriteRules(path string, rules []string) {
	_ = os.WriteFile(path, []byte(strings.Join(rules, "\n")+"\n"), 0o644)
}

// cptpTempBase = py _tmp_base ＋ tempfile.gettempdir：首选可用的 TMPDIR/TEMP/TMP，否则 /tmp。
func cptpTempBase() string {
	for _, k := range []string{"TMPDIR", "TEMP", "TMP"} {
		if v := os.Getenv(k); v != "" && cptpIsDir(v) {
			return v
		}
	}
	return "/tmp"
}

// cptpSelfTest = py self_test（夹具基目录尊重 TMPDIR · try/finally 清掉）。
func cptpSelfTest(stdout io.Writer, repo string) int {
	base, err := os.MkdirTemp(cptpTempBase(), "pubtree-private-selftest-")
	if err != nil {
		fmt.Fprintf(stdout, "自检：%d 过 / %d 红（%d 条用例）\n", 0, 1, 1)
		return 1
	}
	defer func() { _ = os.RemoveAll(base) }()
	prev := cptpRepoOverride
	cptpRepoOverride = repo
	defer func() { cptpRepoOverride = prev }()
	return cptpSelfTestBody(stdout, repo, base)
}

// cptpSelfTestBody = py _self_test_body。
func cptpSelfTestBody(stdout io.Writer, repo, base string) int {
	cwd, _ := os.Getwd()
	rulesPath := cptpPyJoin(repo, "publish", "private-paths.txt")
	r, _ := cptpLoadRules(rulesPath)
	var rules []string
	rules = append(rules, r.dirs...)
	rules = append(rules, r.files...)
	fmt.Fprintf(stdout, "自检工作区（尊重 TMPDIR · 跑完即清）：%s\n", base)
	fmt.Fprintf(stdout, "判据面：%d 目录 + %d 文件 = %d 条规则 ← %s\n",
		len(r.dirs), len(r.files), len(rules), cptpPyRelpath(cwd, rulesPath, repo))
	nOK, nBad := 0, 0
	check := func(name string, ok bool, detail string) {
		if ok {
			nOK++
			fmt.Fprintf(stdout, "  ✅ %s\n", name)
			return
		}
		nBad++
		if detail != "" {
			fmt.Fprintf(stdout, "  ❌ %s\n     %s\n", name, strings.ReplaceAll(detail, "\n", "\n     "))
			return
		}
		fmt.Fprintf(stdout, "  ❌ %s\n", name)
	}
	runGate := func(tree, rp string, extra ...string) (int, string) {
		argv := append([]string{tree, "--rules", rp}, extra...)
		var ob, eb strings.Builder
		rc := checkPublicTreePrivateGateMain(&ob, &eb, argv)
		return rc, cptpPyStrip(ob.String() + eb.String())
	}

	if len(rules) == 0 {
		check("① 判据面非空（解析出 ≥1 条规则）", false, "黑名单解析为 0 条 ⇒ 无法自检")
		fmt.Fprintf(stdout, "自检：%d 过 / %d 红（%d 条用例）\n", nOK, nBad, nOK+nBad)
		return 1
	}
	check("① 判据面非空（解析出 ≥1 条规则）", true, "")

	allRules := cptpPyJoin(base, "rules-all.txt")
	cptpWriteRules(allRules, rules)

	// ② 干净树（只放正式面样例）⇒ rc=0
	clean := cptpPyJoin(base, "clean")
	cptpWriteTree(clean, cptpCleanSample)
	rc, out := runGate(clean, allRules)
	check(fmt.Sprintf("② 干净树（%s）⇒ rc=0", strings.Join(cptpCleanSample, " · ")), rc == 0,
		fmt.Sprintf("rc=%d\n%s", rc, cptpPyTail(out, 400)))

	// ③④ 每条规则成对负控（★ 单跑负向判据：--no-docs-allowlist）
	noAllow := []string{"--no-docs-allowlist"}
	for idx, rule := range rules {
		i := idx + 1
		sample := cptpSampleFor(rule)
		tree := cptpPyJoin(base, "one", fmt.Sprintf("case%02d", i))
		cptpWriteTree(tree, []string{sample})
		rc, out := runGate(tree, allRules, noAllow...)
		named := strings.Contains(out, "规则 "+rule)
		check(fmt.Sprintf("③ %-40s ⇒ rc=1 且点名", rule), rc == 1 && named,
			fmt.Sprintf("rc=%d · 点名=%s\n%s", rc, cptpPyBool(named), cptpPyTail(out, 400)))
		stripped := cptpPyJoin(base, fmt.Sprintf("rules-stripped-%02d.txt", i))
		var rest []string
		for _, x := range rules {
			if x != rule {
				rest = append(rest, x)
			}
		}
		cptpWriteRules(stripped, rest)
		rc2, out2 := runGate(tree, stripped, noAllow...)
		check(fmt.Sprintf("④ %-40s ⇒ 抽掉后漏报（rc=0）", rule), rc2 == 0,
			fmt.Sprintf("rc=%d\n%s", rc2, cptpPyTail(out2, 400)))
	}

	// ⑤~⑨ 正向断言的成对探针
	allow := cptpDocsAllowlist(cwd, repo)
	ok5 := allow.err == "" && len(allow.exact) >= 1
	d5 := allow.err
	if d5 == "" {
		d5 = "overlay 目标为空：publish/docs 与白名单/映射对不上"
	}
	check(fmt.Sprintf("⑤ 合法集现读得出且非空（overlay 目标 %d 条 · 载入口径 %s）",
		len(allow.exact), allow.sourceMode), ok5, d5)

	if ok5 {
		var legal []string
		for k := range allow.exact {
			legal = append(legal, k)
		}
		sort.Strings(legal)
		legal = append(legal, "docs/zh/"+"正式样例.md", "docs/en/"+"formal-sample.md")
		illegal := []string{"docs/" + "设计" + "稿/a.md", "docs/" + "甲方" + "资料" + "/a.md"}

		tree := cptpPyJoin(base, "docs-legal")
		cptpWriteTree(tree, legal)
		rc, out := runGate(tree, allRules)
		check(fmt.Sprintf("⑥ 合法树（docs/zh + docs/en + %d 个 overlay 目标）⇒ rc=0", len(allow.exact)),
			rc == 0, fmt.Sprintf("rc=%d\n%s", rc, cptpPyTail(out, 500)))

		badTree := cptpPyJoin(base, "docs-illegal")
		cptpWriteTree(badTree, append(append([]string{}, legal...), illegal...))
		rc, out = runGate(badTree, allRules)
		named := true
		for _, p := range illegal {
			if !strings.Contains(out, p) {
				named = false
			}
		}
		check(fmt.Sprintf("⑦ 非法树（+%s）⇒ rc=1 且逐条点名", strings.Join(illegal, " + ")), rc == 1 && named,
			fmt.Sprintf("rc=%d · 逐条点名=%s\n%s", rc, cptpPyBool(named), cptpPyTail(out, 600)))

		rc2, out2 := runGate(badTree, allRules, noAllow...)
		check("⑧ 同一非法树抽掉正向断言 ⇒ rc=0（漏报）", rc2 == 0, fmt.Sprintf("rc=%d\n%s", rc2, cptpPyTail(out2, 400)))

		empty := cptpPyJoin(base, "docs-empty")
		cptpWriteTree(empty, []string{"README.md"})
		rc3, out3 := runGate(empty, allRules)
		check("⑨ 空转（树内无 docs/）⇒ rc=2 不给结论", rc3 == 2, fmt.Sprintf("rc=%d\n%s", rc3, cptpPyTail(out3, 400)))
	}

	// ⑩ EXCLUDES 失配守卫（`GAP-20260927-192` · 对齐 py 自检 ⑩）：真源在盘但解析不出活跃条目
	//   ⇒ `docs_allowlist` 必须报 error（成对：正控 = 真源正常时 ⑤ 已证 `exact ≥ 1` 且 error 空）。
	badPsh := cptpPyJoin(base, "publish-sh-broken.sh")
	_ = os.WriteFile(badPsh, []byte("# 没有 EXCLUDES=( … ) 块\nEXCLUDES_OK=1\n"), 0o644)
	a2 := cptpDocsAllowlistWith(cwd, repo, badPsh)
	check("⑩ EXCLUDES 真源解析失配（无 EXCLUDES 块）⇒ docs_allowlist 报 error ⇒ 调用方 rc=2",
		a2.err != "", fmt.Sprintf("error=%s", a2.err))

	extra := 1
	if ok5 {
		extra = 5
	}
	fmt.Fprintf(stdout, "自检：%d 过 / %d 红（%d 条用例 = 1 判据面 + 1 清洁 + 2×%d 规则 + %d 正向断言 + 1 EXCLUDES 守卫）\n",
		nOK, nBad, nOK+nBad, len(rules), extra)
	if nBad == 0 {
		return 0
	}
	return 1
}
