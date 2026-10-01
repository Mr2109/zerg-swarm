// check_publish_face_sync.go —— 判据件 `scripts/gates/check-publish-face-sync.py`（550 行）的 **Go 真搬**
// （M2 · 发布面同步族 · 入口 `checkPublishFaceSyncGateMain` · 承接 GAP-20261001-13）。
//
// 验收判据 = 同 argv 跑两态（py 原件 ↔ 本入口），stdout / stderr / 退码**三面逐字相同**。
//
// 声明 (a) 默认根定位差异：py 从 `__file__` 上溯 3 层推 `REPO`（`scripts/gates/…` → 仓根）；Go 无 `__file__`
// 语境 ⇒ 只能从 **cwd 上溯**找「含 `scripts/gates/` 或 `publish/private-paths.txt` 的目录」当默认根。
// 两态在「脚本位置 == cwd 上溯根」时逐字同构（两环境对拍即按此布置）。★ 未传 `--repo` 时默认面**不是**
// 逐字承诺外的格：见交付⑤「未覆盖面」。
//
// 声明 (b) 自指：本件**不落在本判据自己的**判红面内（本门不读任何件的**内容**做判定，只读白名单/黑名单/
// EXCLUDES/标记表/替换规则表/镜像器自带表这**六张清单**）。★ 但本件落 `core/cmd/zerg/`，**落在「他门」的
// 扫描面内**：`scripts/gates/check-hardcoded-private-paths.py` 的 `SCAN_DIRS` 含 `core/` ⇒ 本件正文**零私有
// 卷/家目录绝对路径字面量**（一律不写）；夹具字面量只出现「仓相对路径」。四形态核查见交付④。
//
// 遍历/排序点（逐点交代 · 完整版见交付②）：
//
//	P1 `tracked_files`：`git -C repo -c core.quotepath=false ls-files` ⇒ 输出即 git 索引序（**已排序**，
//	   非 readdir 原序）；Go 侧 `exec.Command` 原样接住、`strings.Split("\n")` 保序 ⇒ 与 py 同序。
//	   **输出行序决定者**：候选集 `cand` 的行序 = 本序；判据①/②/⑥ 的明细行序 = `cand` 子序列 ⇒ 由本序决定。
//	P2 `no_set = set(no_marks)`：只做**成员判定**，从不迭代 ⇒ 无序遍历不进输出（零影响）。
//	P3 `sorted(missing)`：py 明写 sorted ⇒ Go `sort.Strings` 复现（唯一的显式排序点）。
//	P4 `marks` 表序 = 表文件行序（`mkcpLoadTable` 逐行追加）；`marks_cand` 取 `cand` 序 ∩ `no_set` ⇒ 输出表序无涉。
//	P5 清单语法 `list_syntax_problems`：遍历序 = `LIST_FILES + TSV_FILES` 声明序，逐件内逐行 ⇒ 无排序。
//
// Join / fnmatch / 正则 逐处交代（完整版见交付②）：
//
//	· `os.path.join` 出现在：`load_module`（均从 `REPO` 拼 `scripts/gates/…`）、`paths_for`、
//	  `list_syntax_problems`（`pubdir + name`）、`make_fixture`。三语义差（吃 `..` · 后段绝对丢前段 ·
//	  空段 `join('a','')=='a/'`）本件均**不触发**（各段皆非空、非绝对、无 `..`），但按本批口径**恒同构**：
//	  自写 `cpfsPyJoin`（照 CPython `posixpath.join` 逐段规则）＋ `cpfsPyNormpath`/`cpfsPyAbspath`
//	  （`os.path.abspath` = `normpath(join(getcwd(), p))`）；定向单测把三态各钉一格。
//	· `fnmatch` / `glob`：**本件不涉**（py 正文无 `glob`/`fnmatch` 调用）⇒ 无 `*` 跨 `/` 面。
//	· 正则：**本件 py 正文零 `re`**；唯一一处正则面在被复用的 `check-history-secrets.py::load_excludes`
//	  （`re.search(r"EXCLUDES=\((.*?)\n\)", text, re.S)`）⇒ 由**已落仓件** `check_marks_coverage_parity.go`
//	  的 `mkcpLoadExcludes`（纯字符串切分，同构）承接，本件不另写 RE2。见交付⑥。
//
// 复用面（单一真源 · ✗ 不另抄第二份）：
//
//	· 白名单/EXCLUDES/excluded/published_path/标记表/git 索引 —— `check_marks_coverage_parity.go`
//	  的 `mkcpLoadWhitelist`/`mkcpLoadExcludes`/`mkcpExcluded`/`mkcpPublishedPath`/`mkcpLoadTable`/
//	  `mkcpTableError`/`mkcpTrackedFiles`（后者 = `check-history-secrets.py` 的 `load_excludes` 等同构重写）
//	· 私有面 `load_rules`/`is_allowed` —— `family_publish_set.go` 的 `publishSetPrivateFaceRules`/
//	  `publishSetPrivateAllowed`（= `check-public-tree-private.py:47/:64` 的逐条镜像）
//	· 镜像器自带表 `DROP_EXACT`/`DROP_PREFIX` —— `mkcpPyStringSet`/`mkcpPyStringTuple`（Python 字面量现读）
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

const (
	cpfsWallEntry     = "wall/"
	cpfsMarksTableRel = "scripts/公开标记.tsv"
	cpfsUnclosedTail  = "；;、，,：:"
)

var cpfsListFiles = [][2]string{
	{"白名单", "whitelist.txt"},
	{"私有面黑名单", "private-paths.txt"},
	{"豁免清单", "mirror-allow.txt"},
}

var cpfsTsvFiles = [][2]string{
	{"替换规则表", "replace-rules.tsv"},
}

// ── os.path 语义（自写 · 照 CPython posixpath）──────────────────────────────────────────

// cpfsPyJoin = `os.path.join`（posix）逐段规则。三语义差全在：
//   - 后段绝对 ⇒ 丢弃前面所有段（join("repo","/abs")=="/abs"）
//   - 空段：join("a","")=="a/"；join("","a")=="a"；join("a","","b")=="a/b"
//   - `..` **原样留串**（join("a","../b")=="a/../b" —— 不归一）
func cpfsPyJoin(parts ...string) string {
	if len(parts) == 0 {
		return ""
	}
	path := parts[0]
	for _, b := range parts[1:] {
		switch {
		case strings.HasPrefix(b, "/"):
			path = b
		case path == "" || strings.HasSuffix(path, "/"):
			path += b
		default:
			path += "/" + b
		}
	}
	return path
}

// cpfsPyNormpath = `os.path.normpath`（posix）逐字照 CPython。
func cpfsPyNormpath(path string) string {
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
	newComps := []string{}
	for _, comp := range strings.Split(path, "/") {
		if comp == "" || comp == "." {
			continue
		}
		if comp != ".." || (initialSlashes == 0 && len(newComps) == 0) ||
			(len(newComps) > 0 && newComps[len(newComps)-1] == "..") {
			newComps = append(newComps, comp)
		} else if len(newComps) > 0 {
			newComps = newComps[:len(newComps)-1]
		}
	}
	path = strings.Join(newComps, "/")
	if initialSlashes > 0 {
		path = strings.Repeat("/", initialSlashes) + path
	}
	if path == "" {
		return "."
	}
	return path
}

// cpfsPyAbspath = `os.path.abspath`（posix）：非绝对则 `join(getcwd(), p)` 再 `normpath`。
// ★ 注意与 Go `filepath.Abs` 的差：后者等价，但走 `Clean`；极端串（`//a`）语义不同 ⇒ 自写更稳。
func cpfsPyAbspath(p string) string {
	if !strings.HasPrefix(p, "/") {
		wd, err := os.Getwd()
		if err != nil {
			wd = "."
		}
		p = cpfsPyJoin(wd, p)
	}
	return cpfsPyNormpath(p)
}

// ── 基础判定（照 py 逐字）────────────────────────────────────────────────────────────

func cpfsIsFile(p string) bool { return mkcpIsFile(p) }

// cpfsExists = `os.path.exists`（跟随符号链接；断链 ⇒ false）。
func cpfsExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st != nil
}

// cpfsIsRepoRel = py `is_repo_rel`：非空 · 不以 `/` 或 `~` 开头 · 不含空白 · 无 `..` 段。
func cpfsIsRepoRel(rel string) bool {
	if rel == "" || strings.HasPrefix(rel, "/") || strings.HasPrefix(rel, "~") {
		return false
	}
	for _, r := range rel {
		if unicode.IsSpace(r) {
			return false
		}
	}
	return !mkcpHasDotDotSegment(rel)
}

// cpfsPyRepr 对应 py 对 str 的 `%r`（常见形态：无特殊字符 ⇒ 单引号包裹）。
func cpfsPyRepr(s string) string {
	if !strings.ContainsAny(s, "'\"\\\n\r\t") {
		return "'" + s + "'"
	}
	if strings.Contains(s, "'") && !strings.Contains(s, "\"") {
		return "\"" + s + "\""
	}
	return "'" + strings.ReplaceAll(strings.ReplaceAll(s, "\\", "\\\\"), "'", "\\'") + "'"
}

func cpfsCount(s, sub string) int { return strings.Count(s, sub) }

// cpfsDecodeUTF8Replace 对应 `raw.decode("utf-8","replace")`。★ 差异：Python 按「最长非法子序列」
// 逐段替换、Go `ToValidUTF8` 按「非法运行段」替换 ⇒ 连续多字节非法段的 U+FFFD 个数可能不同（未覆盖面）。
func cpfsDecodeUTF8Replace(b []byte) string { return strings.ToValidUTF8(string(b), "\uFFFD") }

// ── 清单语法（判据⑤）`list_syntax_problems` 逐字照搬 ──────────────────────────────────

// cpfsListSyntaxProblems → [(文件名, 行号(0=整件), 说明)]。只读，不落盘。
func cpfsListSyntaxProblems(paths cpfsPaths) [][3]string {
	pubdir := filepath.Dir(paths.whitelist)
	problems := [][3]string{}
	all := append(append([][2]string{}, cpfsListFiles...), cpfsTsvFiles...)
	for _, lf := range all {
		label, name := lf[0], lf[1]
		raw, err := os.ReadFile(cpfsPyJoin(pubdir, name))
		if err != nil {
			problems = append(problems, [3]string{name, "0",
				fmt.Sprintf("%s 读不到（缺件 ≠ 绿）", label)})
			continue
		}
		text := cpfsDecodeUTF8Replace(raw)
		if strings.TrimSpace(text) == "" {
			problems = append(problems, [3]string{name, "0",
				fmt.Sprintf("%s 是空的（空清单 = 什么都没配 ⇒ 不给结论）", label)})
			continue
		}
		if !strings.HasSuffix(string(raw), "\n") {
			problems = append(problems, [3]string{name, strconv.Itoa(cpfsCount(text, "\n") + 1),
				"末行没有结尾换行 ⇒ 末行可能被截断（未闭合）"})
		}
		type row struct {
			i  int
			ln string
		}
		rows := []row{}
		for i, ln := range strings.Split(text, "\n") {
			if strings.TrimSpace(ln) != "" {
				rows = append(rows, row{i + 1, ln})
			}
		}
		lastI, last := rows[len(rows)-1].i, rows[len(rows)-1].ln
		if name == "whitelist.txt" || name == "private-paths.txt" || name == "mirror-allow.txt" {
			trimmed := strings.TrimRight(last, " \t\n\r\v\f")
			tail := ""
			if trimmed != "" {
				rs := []rune(trimmed)
				tail = string(rs[len(rs)-1])
			}
			if tail != "" && strings.Contains(cpfsUnclosedTail, tail) {
				problems = append(problems, [3]string{name, strconv.Itoa(lastI),
					fmt.Sprintf("末条内容行以 %s 结尾（句子没写完 ⇒ 未闭合）", tail)})
			}
			if cpfsCount(last, "（") != cpfsCount(last, "）") {
				problems = append(problems, [3]string{name, strconv.Itoa(lastI),
					fmt.Sprintf("末条内容行全角括号不配平（%d 开 / %d 闭 ⇒ 未闭合）",
						cpfsCount(last, "（"), cpfsCount(last, "）"))})
			}
			if cpfsCount(text, "（") != cpfsCount(text, "）") {
				problems = append(problems, [3]string{name, "0",
					fmt.Sprintf("整件全角括号不配平（%d 开 / %d 闭 ⇒ 有注释没写完）",
						cpfsCount(text, "（"), cpfsCount(text, "）"))})
			}
		}
		for _, rw := range rows {
			ln := strings.TrimSpace(rw.ln)
			if strings.HasPrefix(ln, "#") {
				continue
			}
			switch name {
			case "mirror-allow.txt":
				parts := strings.SplitN(ln, "\t", 2)
				key := parts[0]
				why := ""
				if len(parts) == 2 {
					why = parts[1]
				}
				if strings.TrimSpace(why) == "" {
					problems = append(problems, [3]string{name, strconv.Itoa(rw.i),
						fmt.Sprintf("豁免条目不是『镜像路径<TAB>理由』（理由空 = 命中了也不说为什么）：%s",
							cpfsPyRepr(cpfsCut(ln, 60)))})
				} else if !cpfsIsRepoRel(strings.TrimSpace(key)) {
					problems = append(problems, [3]string{name, strconv.Itoa(rw.i),
						fmt.Sprintf("豁免条目的键不是仓相对路径：%s",
							cpfsPyRepr(cpfsCut(strings.TrimSpace(key), 60)))})
				}
			case "replace-rules.tsv":
				if len(strings.Split(ln, "\t")) < 2 {
					problems = append(problems, [3]string{name, strconv.Itoa(rw.i),
						fmt.Sprintf("规则行少于 2 个 TAB 字段（解析器会**静默跳过**这一条）：%s",
							cpfsPyRepr(cpfsCut(ln, 60)))})
				}
			default:
				body := ln
				if strings.HasPrefix(ln, "!") {
					body = strings.TrimSpace(ln[1:])
				}
				if body == "" {
					problems = append(problems, [3]string{name, strconv.Itoa(rw.i),
						"`!` 后面没有路径（空豁免）"})
				} else if !cpfsIsRepoRel(body) {
					problems = append(problems, [3]string{name, strconv.Itoa(rw.i),
						fmt.Sprintf("条目不是仓相对路径（解析器只当它不存在）：%s", cpfsPyRepr(cpfsCut(ln, 60)))})
				}
			}
		}
	}
	return problems
}

// cpfsCut = py 切片 `s[:n]`（按**字符**）。
func cpfsCut(s string, n int) string {
	rs := []rune(s)
	if len(rs) <= n {
		return s
	}
	return string(rs[:n])
}

// ── 真源路径 / 分类 ─────────────────────────────────────────────────────────────────

type cpfsPaths struct {
	whitelist    string
	privatePaths string
	publishSh    string
	mirrorLib    string
	marksTable   string
}

func cpfsPathsFor(repo string) cpfsPaths {
	return cpfsPaths{
		whitelist:    cpfsPyJoin(repo, "publish", "whitelist.txt"),
		privatePaths: cpfsPyJoin(repo, "publish", "private-paths.txt"),
		publishSh:    cpfsPyJoin(repo, "scripts", "build", "publish-public.sh"),
		mirrorLib:    cpfsPyJoin(repo, "publish", "mirror-public-lib.py"),
		marksTable:   cpfsPyJoin(repo, cpfsMarksTableRel),
	}
}

// cpfsOrdered 按 py dict 插入序返回 (键, 路径) 对（**零 map 迭代**）。
func (p cpfsPaths) cpfsOrdered() [][2]string {
	return [][2]string{
		{"whitelist", p.whitelist},
		{"private_paths", p.privatePaths},
		{"publish_sh", p.publishSh},
		{"mirror_lib", p.mirrorLib},
		{"marks_table", p.marksTable},
	}
}

func (p *cpfsPaths) cpfsSet(key, val string) {
	switch key {
	case "whitelist":
		p.whitelist = val
	case "private_paths":
		p.privatePaths = val
	case "publish_sh":
		p.publishSh = val
	case "mirror_lib":
		p.mirrorLib = val
	case "marks_table":
		p.marksTable = val
	}
}

// cpfsRequirePaths → 缺件的键列表（缺件一律 rc=2，绝不静默跳过）。
func cpfsRequirePaths(p cpfsPaths) []string {
	var miss []string
	for _, kv := range p.cpfsOrdered() {
		if !cpfsExists(kv[1]) {
			miss = append(miss, kv[0])
		}
	}
	return miss
}

type cpfsClass struct {
	cand, dropped, wall   []string
	hits                  [][2]string
	entries, excludes     []string
	pdirs, pfiles         []string
	marks                 []mkcpRow
	dropExact, dropPrefix []string
}

// cpfsMapPathNone = `map_path(rel) is None`（DROP_EXACT 全等 ∨ DROP_PREFIX 前缀）。
func cpfsMapPathNone(rel string, dropExact, dropPrefix []string) bool {
	if mkcpContains(dropExact, rel) {
		return true
	}
	for _, p := range dropPrefix {
		if strings.HasPrefix(rel, p) {
			return true
		}
	}
	return false
}

// cpfsClassify → 分类结果；表不合规 ⇒ 返 `mkcpTableError`（调用方按 rc=2 处理）。
func cpfsClassify(repo string, p cpfsPaths) (*cpfsClass, error) {
	c := &cpfsClass{}
	c.entries = mkcpLoadWhitelist(p.whitelist)
	c.excludes = mkcpLoadExcludes(p.publishSh)
	plines, err := publishSetReadLines(p.privatePaths)
	if err != nil {
		return nil, mkcpUnsupported{"私有面黑名单读不到：" + p.privatePaths}
	}
	dirs, files, allowed := publishSetPrivateFaceRules(plines)
	c.pdirs, c.pfiles = dirs, files

	if de, ok := mkcpPyStringSet(p.mirrorLib, "DROP_EXACT"); ok {
		c.dropExact = de
	}
	if dp, ok := mkcpPyStringTuple(p.mirrorLib, "DROP_PREFIX"); ok {
		c.dropPrefix = dp
	}

	rows, err := mkcpLoadTable(p.marksTable)
	if err != nil {
		return nil, err
	}
	c.marks = rows

	tracked, err := mkcpTrackedFiles(repo)
	if err != nil {
		return nil, err
	}
	for _, rel := range tracked {
		if !mkcpPublishedPath(rel, c.entries) {
			continue
		}
		if strings.HasPrefix(rel, "publish/") {
			continue
		}
		c.cand = append(c.cand, rel)
		if strings.HasPrefix(rel, cpfsWallEntry) {
			c.wall = append(c.wall, rel)
		}
		var m string
		if !publishSetPrivateAllowed(rel, allowed) {
			for _, d := range c.pdirs {
				if strings.HasPrefix(rel, d) {
					m = d
					break
				}
			}
			if m == "" && mkcpContains(c.pfiles, rel) {
				m = rel
			}
		}
		if m != "" {
			c.hits = append(c.hits, [2]string{rel, m})
		}
		if cpfsMapPathNone(rel, c.dropExact, c.dropPrefix) {
			c.dropped = append(c.dropped, rel)
		}
	}
	return c, nil
}

// ── 判定 `evaluate` 逐字照搬 ─────────────────────────────────────────────────────────

func cpfsEvaluate(repo string, paths cpfsPaths) (int, []string) {
	missing := cpfsRequirePaths(paths)
	if len(missing) > 0 {
		sort.Strings(missing)
		return 2, []string{"  ✗ 缺件（" + strings.Join(missing, "、") + "）⇒ 拒绝给结论（rc=2）"}
	}
	cl, err := cpfsClassify(repo, paths)
	if err != nil {
		if te, ok := err.(mkcpTableError); ok {
			return 2, []string{"  ✗ 标记表不合规（" + te.msg +
				"）⇒ 拒绝给结论（rc=2）—— 用坏表判出的绿没有意义（A 档口径）"}
		}
		return 2, []string{"  ✗ " + err.Error() + "（rc=2）"}
	}
	var noMarks []string
	for _, r := range cl.marks {
		if r.pub == "no" {
			noMarks = append(noMarks, r.rel)
		}
	}
	lines := []string{}
	lines = append(lines, fmt.Sprintf("发布面同步门禁 —— 候选 %d 条（白名单 %d 条 · 黑名单 %d 目录+%d 文件 · "+
		"EXCLUDES %d 条 · 标记表 %d 行/`no` %d 条）",
		len(cl.cand), len(cl.entries), len(cl.pdirs), len(cl.pfiles), len(cl.excludes),
		len(cl.marks), len(noMarks)))
	lines = append(lines, fmt.Sprintf("  候选里的 wall/ 路径：%d 条", len(cl.wall)))
	lines = append(lines, fmt.Sprintf("  镜像器自带清单丢弃（map_path=None）：%d 条", len(cl.dropped)))
	lines = append(lines, fmt.Sprintf("  命中私有面黑名单：%d 条", len(cl.hits)))

	if len(cl.cand) == 0 {
		return 2, append(lines, "  ✗ 守卫：候选集为空 ⇒ 拒绝给结论（rc=2）")
	}
	if len(cl.entries) == 0 {
		return 2, append(lines, "  ✗ 守卫：白名单解析为空 ⇒ 拒绝给结论（rc=2）")
	}
	if len(cl.excludes) == 0 {
		return 2, append(lines, "  ✗ 守卫：EXCLUDES 一条都解析不出来 ⇒ 拒绝给结论（rc=2）")
	}
	if len(cl.pdirs) == 0 && len(cl.pfiles) == 0 {
		return 2, append(lines, "  ✗ 守卫：私有面黑名单解析为空 ⇒ 拒绝给结论（rc=2）")
	}
	wallInEntries := mkcpContains(cl.entries, cpfsWallEntry)
	if !wallInEntries {
		wallKey := strings.TrimRight(cpfsWallEntry, "/")
		for _, e := range cl.entries {
			if strings.TrimRight(e, "/") == wallKey {
				wallInEntries = true
				break
			}
		}
	}
	if !wallInEntries {
		return 2, append(lines, fmt.Sprintf("  ✗ 守卫：白名单里没有 %s ⇒ 拒绝给结论（rc=2）", cpfsWallEntry))
	}
	if len(cl.wall) == 0 {
		return 2, append(lines, fmt.Sprintf("  ✗ 守卫：候选集里没有一条 %s 路径 ⇒ 拒绝给结论（rc=2）", cpfsWallEntry))
	}
	if len(noMarks) == 0 {
		return 2, append(lines, "  ✗ 守卫：标记表里一条 `public=no` 都没有 ⇒ 判据⑥ 无区分度（空转 = 假覆盖）"+
			"⇒ 拒绝给结论（rc=2）")
	}

	syn := cpfsListSyntaxProblems(paths)
	lines = append(lines, fmt.Sprintf("  清单语法（白名单/私有面黑名单/豁免清单/替换规则表 四件）：%d 处不合格", len(syn)))
	if len(syn) > 0 {
		lim := len(syn)
		if lim > cpfsDetailCap {
			lim = cpfsDetailCap
		}
		for _, s := range syn[:lim] {
			ln := s[1]
			if ln == "0" {
				ln = "整件"
			}
			lines = append(lines, fmt.Sprintf("  ✗ 判据⑤ %s:%s —— %s", s[0], ln, s[2]))
		}
		lines = append(lines, "  ⇒ 真源读不全（不合格行会被解析器**静默跳过** ⇒ 门禁假绿）⇒ 不给结论（rc=2）")
		return 2, lines
	}

	bad := 0
	var drift []string
	for _, p := range cl.dropped {
		if !mkcpExcluded(p, cl.excludes) {
			drift = append(drift, p)
		}
	}
	if len(drift) > 0 {
		bad = 1
		lines = append(lines, fmt.Sprintf("  ✗ 判据① 漂移 %d 条 —— 镜像器丢、旧器不排（旧器真跑会导出 ⇒ "+
			"被私有面门禁抓住而整批自中止）：", len(drift)))
		lim := cpfsLim(len(drift))
		for _, p := range drift[:lim] {
			lines = append(lines, "      "+p)
		}
	} else {
		lines = append(lines, "  ✅ 判据① 漂移：0 条（镜像器丢弃的路径旧器全都排除了）")
	}

	var leak [][2]string
	for _, hm := range cl.hits {
		p := hm[0]
		if !mkcpExcluded(p, cl.excludes) && !mkcpContains(cl.dropped, p) {
			leak = append(leak, hm)
		}
	}
	if len(leak) > 0 {
		bad = 1
		lines = append(lines, fmt.Sprintf("  ✗ 判据② 两边都漏 %d 条 —— 命中私有面黑名单且两器都没处理"+
			"（两器都会把私有面路径带进公开面）：", len(leak)))
		lim := cpfsLim(len(leak))
		for _, hm := range leak[:lim] {
			lines = append(lines, "      "+cpfsPad(hm[0], 60)+" ← 规则 "+hm[1])
		}
	} else {
		lines = append(lines, "  ✅ 判据② 两边都漏：0 条（命中黑名单的路径两器都已处理）")
	}

	noSet := map[string]bool{}
	for _, r := range noMarks {
		noSet[r] = true
	}
	var marksCand []string
	for _, p := range cl.cand {
		if noSet[p] {
			marksCand = append(marksCand, p)
		}
	}
	var unhandled []string
	for _, p := range marksCand {
		if !mkcpExcluded(p, cl.excludes) {
			unhandled = append(unhandled, p)
		}
	}
	if len(unhandled) > 0 {
		bad = 1
		nMirror := 0
		for _, p := range unhandled {
			if !cpfsMapPathNone(p, cl.dropExact, cl.dropPrefix) {
				nMirror++
			}
		}
		lines = append(lines, fmt.Sprintf("  ✗ 判据⑥ 表说 no、链上却仍会发 %d 条（其中镜像器仍发 %d 条 ⇒ 会进产出树）：",
			len(unhandled), nMirror))
		lim := cpfsLim(len(unhandled))
		for _, p := range unhandled[:lim] {
			tail := "旧器映射层丢，但 EXCLUDES 没收 ⇒ 快照通道会带上"
			if !cpfsMapPathNone(p, cl.dropExact, cl.dropPrefix) {
				tail = "镜像器仍发 ⇒ 会进产出树"
			}
			lines = append(lines, fmt.Sprintf("      %s（%s）", p, tail))
		}
		lines = append(lines, "      ⇒ 处置只有两条（照设计稿 §3.3 纪律 3）：收进 EXCLUDES + DROP_EXACT，或**改表**；"+
			"**不许**放宽本判据来凑绿 ✗")
	} else {
		lines = append(lines, fmt.Sprintf("  ✅ 判据⑥ 表说 no 的件两器都已排除：%d 条（表 `no` %d 行 · 候选集内 %d 条）",
			len(marksCand), len(noMarks), len(marksCand)))
	}
	return bad, lines
}

// cpfsDetailCap —— 明细行/语法问题行的打印上限（py 的 `[:40]` · 纯判据参数，**不是**退出码）。
// ★ 声成具名常量而非裸数字：门⑪ `T3`（`check-cli-contract.py::bare_numeric_returns`）的扫描面口径很宽 ——
//
//	它把命令面源码里任何 `return <数字>` 都当成退出码去比退码真源表（见 `family_gap.go` 的同条记录）⇒
//	裸写 40 会被判「不在真源表里」。
const cpfsDetailCap = 40

func cpfsLim(n int) int {
	if n > cpfsDetailCap {
		return cpfsDetailCap
	}
	return n
}

// cpfsPad 对应 py 的 `%-60s`（按**字符**左对齐补空格 —— py str 格式化按字符）。
func cpfsPad(s string, w int) string {
	n := len([]rune(s))
	if n >= w {
		return s
	}
	return s + strings.Repeat(" ", w-n)
}

// ── 自检夹具（16 条 · 逐字照 py FIXTURES/self_test）──────────────────────────────────

const cpfsFixtureLibTmpl = `# 夹具用的最小 mirror lib（只提供本门禁读的三个名字）
DROP_PREFIX = ()
DROP_EXACT = {%s}

def map_path(rel):
    if rel in DROP_EXACT or any(rel.startswith(p) for p in DROP_PREFIX):
        return None
    return rel
`

var cpfsFixtureTracked = []string{"scripts/mech.py", "scripts/keep.py", "wall/src/lib.rs", "wall/Cargo.toml"}

const cpfsDefaultMarks = "文件\tpublic\t理由\n" +
	"scripts/mech.py\tno\t夹具：B 类（合成 · 判据⑥ 的被判对象）\n" +
	"scripts/keep.py\tyes\t夹具：A 类（合成）\n"

type cpfsFx struct {
	name            string
	wl, priv        []string
	excl, drop      []string
	want            int
	noWallFiles     bool
	deleteWhitelist bool
	deleteMarks     bool
	allowText       string
	marksText       string
	trackedExtra    []string
}

var cpfsFixtures = []cpfsFx{
	{"ok-both-dropped", []string{"scripts/", "wall/"}, []string{"scripts/mech.py"},
		[]string{"scripts/mech.py"}, []string{"scripts/mech.py"}, 0, false, false, false, "", "", nil},
	{"drift-mirror-only", []string{"scripts/", "wall/"}, []string{"scripts/mech.py"},
		[]string{"scripts/unrelated-but-real.py"}, []string{"scripts/mech.py"}, 1, false, false, false, "", "", nil},
	{"both-leak", []string{"scripts/", "wall/"}, []string{"scripts/mech.py"},
		[]string{"scripts/unrelated-but-real.py"}, []string{}, 1, false, false, false, "", "", nil},
	{"no-private-rules", []string{"scripts/", "wall/"}, []string{},
		[]string{"scripts/mech.py"}, []string{"scripts/mech.py"}, 2, false, false, false, "", "", nil},
	{"whitelist-without-wall", []string{"scripts/"}, []string{"scripts/mech.py"},
		[]string{"scripts/mech.py"}, []string{"scripts/mech.py"}, 2, false, false, false, "", "", nil},
	{"candidate-without-wall", []string{"scripts/", "wall/"}, []string{"scripts/mech.py"},
		[]string{"scripts/mech.py"}, []string{"scripts/mech.py"}, 2, true, false, false, "", "", nil},
	{"missing-whitelist", []string{"scripts/", "wall/"}, []string{"scripts/mech.py"},
		[]string{"scripts/mech.py"}, []string{"scripts/mech.py"}, 2, false, true, false, "", "", nil},
	{"list-lastline-unclosed", []string{"scripts/", "wall/", "# v2.5.9 残件：某共享模块（裁决逻辑；"},
		[]string{"scripts/mech.py"}, []string{"scripts/mech.py"}, []string{"scripts/mech.py"}, 2, false, false, false, "", "", nil},
	{"list-bad-entry", []string{"scripts/", "wall/", "/abs/path"},
		[]string{"scripts/mech.py"}, []string{"scripts/mech.py"}, []string{"scripts/mech.py"}, 2, false, false, false, "", "", nil},
	{"list-allow-no-reason", []string{"scripts/", "wall/"}, []string{"scripts/mech.py"},
		[]string{"scripts/mech.py"}, []string{"scripts/mech.py"}, 2, false, false, false,
		"# 夹具豁免清单\nscripts/mech.py 夹具理由（没有 TAB）\n", "", nil},
	{"marks-no-both-handled", []string{"scripts/", "wall/"}, []string{"scripts/mech.py"},
		[]string{"scripts/mech.py"}, []string{"scripts/mech.py"}, 0, false, false, false, "", "", nil},
	{"marks-no-excludes-covers-mirror-layer", []string{"scripts/", "wall/"}, []string{"gateway/fleet.yaml"},
		[]string{"scripts/mech.py"}, []string{}, 0, false, false, false, "", "", nil},
	{"marks-no-both-send", []string{"scripts/", "wall/"}, []string{"gateway/fleet.yaml"},
		[]string{"scripts/unrelated-but-real.py"}, []string{}, 1, false, false, false, "", "", nil},
	{"marks-table-missing", []string{"scripts/", "wall/"}, []string{"scripts/mech.py"},
		[]string{"scripts/mech.py"}, []string{"scripts/mech.py"}, 2, false, false, true, "", "", nil},
	{"marks-no-rows-empty", []string{"scripts/", "wall/"}, []string{"scripts/mech.py"},
		[]string{"scripts/mech.py"}, []string{"scripts/mech.py"}, 2, false, false, false, "",
		"文件\tpublic\t理由\nscripts/keep.py\tyes\t夹具：只有 yes 行\n", nil},
	{"marks-no-nonascii-file-sent", []string{"scripts/", "wall/"}, []string{"scripts/mech.py"},
		[]string{"scripts/mech.py"}, []string{"scripts/mech.py"}, 1, false, false, false, "",
		"文件\tpublic\t理由\n" +
			"scripts/mech.py\tno\t夹具：B 类（合成 · 判据⑥ 的被判对象）\n" +
			"scripts/公开核对.tsv\tno\t夹具：非 ASCII 名 + 表说 no + 两器都发（缺口 ① 的那一格）\n" +
			"scripts/keep.py\tyes\t夹具：A 类（合成）\n",
		[]string{"scripts/公开核对.tsv"}},
}

func cpfsMkdirAll(p string) { _ = os.MkdirAll(p, 0o755) }

func cpfsWrite(p, content string) {
	cpfsMkdirAll(filepath.Dir(p))
	_ = os.WriteFile(p, []byte(content), 0o644)
}

func cpfsGit(args ...string) {
	cmd := exec.Command("git", args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	_ = cmd.Run()
}

func cpfsMakeFixture(root string, fx cpfsFx) {
	cpfsMkdirAll(cpfsPyJoin(root, "publish"))
	cpfsMkdirAll(cpfsPyJoin(root, "scripts", "build"))
	cpfsMkdirAll(cpfsPyJoin(root, "wall", "src"))
	cpfsWrite(cpfsPyJoin(root, "publish", "whitelist.txt"),
		"# 夹具白名单\n"+strings.Join(fx.wl, "\n")+"\n")
	cpfsWrite(cpfsPyJoin(root, "publish", "private-paths.txt"),
		"# 夹具黑名单\n"+strings.Join(fx.priv, "\n")+"\n")
	var exb []string
	for _, e := range fx.excl {
		exb = append(exb, "  \""+e+"\"")
	}
	cpfsWrite(cpfsPyJoin(root, "scripts", "build", "publish-public.sh"),
		"#!/usr/bin/env bash\nEXCLUDES=(\n"+strings.Join(exb, "\n")+"\n)\n")
	var drb []string
	for _, d := range fx.drop {
		drb = append(drb, "\""+d+"\"")
	}
	cpfsWrite(cpfsPyJoin(root, "publish", "mirror-public-lib.py"),
		fmt.Sprintf(cpfsFixtureLibTmpl, strings.Join(drb, ", ")))
	cpfsWrite(cpfsPyJoin(root, "publish", "replace-rules.tsv"), "# 夹具替换规则表\nfoo\tbar\t夹具\n")
	allow := fx.allowText
	if allow == "" {
		allow = "# 夹具豁免清单\nscripts/mech.py\t夹具理由\n"
	}
	cpfsWrite(cpfsPyJoin(root, "publish", "mirror-allow.txt"), allow)
	marksPath := cpfsPyJoin(root, cpfsMarksTableRel)
	marksText := fx.marksText
	if marksText == "" {
		marksText = cpfsDefaultMarks
	}
	cpfsWrite(marksPath, marksText)

	tracked := append(append([]string{}, cpfsFixtureTracked...), fx.trackedExtra...)
	if fx.noWallFiles {
		kept := []string{}
		for _, t := range tracked {
			if !strings.HasPrefix(t, "wall/") {
				kept = append(kept, t)
			}
		}
		tracked = kept
	}
	for _, rel := range tracked {
		cpfsWrite(cpfsPyJoin(root, rel), "x\n")
	}
	cpfsGit("init", "-q", "-b", "main", root)
	addCmd := exec.Command("git", "-C", root, "add", "-A")
	addCmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@e",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@e")
	addCmd.Stdout = io.Discard
	addCmd.Stderr = io.Discard
	_ = addCmd.Run()
	if fx.deleteWhitelist {
		_ = os.Remove(cpfsPyJoin(root, "publish", "whitelist.txt"))
	}
	if fx.deleteMarks {
		_ = os.Remove(marksPath)
	}
}

func cpfsSelfTest(stdout io.Writer) int {
	base, err := os.MkdirTemp("", "face-sync-selftest-")
	if err != nil {
		fmt.Fprintln(stdout, "✗ 自检工作区建不出："+err.Error())
		return 1
	}
	defer func() { _ = os.RemoveAll(base) }()
	fmt.Fprintf(stdout, "自检工作区：%s\n", base)
	passN, failN := 0, 0
	for _, fx := range cpfsFixtures {
		root := cpfsPyJoin(base, fx.name)
		cpfsMkdirAll(root)
		cpfsMakeFixture(root, fx)
		rc, _ := cpfsEvaluate(root, cpfsPathsFor(root))
		if rc == fx.want {
			passN++
			fmt.Fprintf(stdout, "  ✅ %s rc=%d（期望 %d）\n", cpfsPad(fx.name, 24), rc, fx.want)
		} else {
			failN++
			fmt.Fprintf(stdout, "  ❌ %s rc=%d（期望 %d）\n", cpfsPad(fx.name, 24), rc, fx.want)
		}
	}
	fmt.Fprintf(stdout, "自检：%d 过 / %d 红（%d 条用例）\n", passN, failN, passN+failN)
	if failN == 0 {
		return 0
	}
	return 1
}

// ── 缺口 `Q-176`：本门无机器读面 ⇒ 给了 `--json` 不许沉默 ──────────────────────────────
var cpfsJSONFields = []string{} // 本门机器读面字段表（唯一真源：印表与拒收同读这一处）

func cpfsRefuseJSON(stderr io.Writer, argv []string) (int, bool) {
	var hit []string
	for _, x := range argv {
		if x == "--json" || strings.HasPrefix(x, "--json=") {
			hit = append(hit, x)
		}
	}
	if len(hit) == 0 {
		return 0, false
	}
	quoted := make([]string, len(hit))
	for i, h := range hit {
		quoted[i] = "`" + h + "`"
	}
	fmt.Fprintf(stderr, "⛔ 本门无机器读面：%s 在本门**未实现**（缺口 Q-176）\n", strings.Join(quoted, " · "))
	fieldTable := "无（本门只出人读面）"
	if len(cpfsJSONFields) > 0 {
		q := make([]string, len(cpfsJSONFields))
		for i, f := range cpfsJSONFields {
			q[i] = "`" + f + "`"
		}
		fieldTable = strings.Join(q, " · ")
	}
	fmt.Fprintf(stderr, "   字段表：%s\n", fieldTable)
	fmt.Fprintln(stderr, "⇒ 用法错（rc=2）：**机器面缺 ≠ 零命中** —— 两态不许同形")
	return 2, true
}

// ── 默认根推导（★ 声明 (a)：无 `__file__` ⇒ 从 cwd 上溯）────────────────────────────
func cpfsRepoRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	d := wd
	for {
		if cpfsIsDirLike(cpfsPyJoin(d, "scripts", "gates")) ||
			cpfsExists(cpfsPyJoin(d, "publish", "private-paths.txt")) {
			return d
		}
		parent := filepath.Dir(d)
		if parent == d {
			return wd
		}
		d = parent
	}
}

func cpfsIsDirLike(p string) bool { return mkcpIsDir(p) }

// ── 入口 ─────────────────────────────────────────────────────────────────────────
type cpfsArgs struct {
	prog      string
	repo      string
	selftest  bool
	help      bool
	whitelist string
	private   string
	publishSh string
	mirrorLib string
}

func cpfsUsage(prog string) string {
	return fmt.Sprintf("usage: %s [-h] [--repo REPO] [--self-test]\n"+
		"                                  [--whitelist WHITELIST]\n"+
		"                                  [--private-paths PRIVATE_PATHS]\n"+
		"                                  [--publish-sh PUBLISH_SH]\n"+
		"                                  [--mirror-lib MIRROR_LIB]\n", prog)
}

func cpfsPrintHelp(w io.Writer, prog string) {
	fmt.Fprint(w, cpfsUsage(prog))
	fmt.Fprint(w, "\noptional arguments:\n")
	fmt.Fprint(w, "  -h, --help            show this help message and exit\n")
	fmt.Fprint(w, "  --repo REPO\n")
	fmt.Fprint(w, "  --self-test\n")
	fmt.Fprint(w, "  --whitelist WHITELIST\n")
	fmt.Fprint(w, "  --private-paths PRIVATE_PATHS\n")
	fmt.Fprint(w, "  --publish-sh PUBLISH_SH\n")
	fmt.Fprint(w, "  --mirror-lib MIRROR_LIB\n")
}

func cpfsParseArgs(stderr io.Writer, argv []string) (cpfsArgs, int) {
	a := cpfsArgs{repo: cpfsRepoRoot()}
	prog := "check-publish-face-sync.py"
	i := 0
	if len(argv) > 0 {
		prog = filepath.Base(argv[0])
		i = 1
	}
	a.prog = prog
	var unknown []string
	need := func(name string, j int) (string, bool) {
		if j+1 >= len(argv) {
			fmt.Fprint(stderr, cpfsUsage(prog))
			fmt.Fprintf(stderr, "%s: error: argument %s: expected one argument\n", prog, name)
			return "", false
		}
		return argv[j+1], true
	}
	for ; i < len(argv); i++ {
		x := argv[i]
		switch {
		case x == "-h" || x == "--help":
			a.help = true
		case x == "--self-test":
			a.selftest = true
		case x == "--repo":
			v, ok := need("--repo", i)
			if !ok {
				return a, 2
			}
			a.repo = v
			i++
		case strings.HasPrefix(x, "--repo="):
			a.repo = x[len("--repo="):]
		case x == "--whitelist":
			v, ok := need("--whitelist", i)
			if !ok {
				return a, 2
			}
			a.whitelist = v
			i++
		case strings.HasPrefix(x, "--whitelist="):
			a.whitelist = x[len("--whitelist="):]
		case x == "--private-paths":
			v, ok := need("--private-paths", i)
			if !ok {
				return a, 2
			}
			a.private = v
			i++
		case strings.HasPrefix(x, "--private-paths="):
			a.private = x[len("--private-paths="):]
		case x == "--publish-sh":
			v, ok := need("--publish-sh", i)
			if !ok {
				return a, 2
			}
			a.publishSh = v
			i++
		case strings.HasPrefix(x, "--publish-sh="):
			a.publishSh = x[len("--publish-sh="):]
		case x == "--mirror-lib":
			v, ok := need("--mirror-lib", i)
			if !ok {
				return a, 2
			}
			a.mirrorLib = v
			i++
		case strings.HasPrefix(x, "--mirror-lib="):
			a.mirrorLib = x[len("--mirror-lib="):]
		default:
			unknown = append(unknown, x)
		}
	}
	if len(unknown) > 0 {
		fmt.Fprint(stderr, cpfsUsage(prog))
		fmt.Fprintf(stderr, "%s: error: unrecognized arguments: %s\n", prog, strings.Join(unknown, " "))
		return a, 2
	}
	return a, 0
}

func checkPublishFaceSyncGateMain(stdout, stderr io.Writer, argv []string) int {
	if rc, hit := cpfsRefuseJSON(stderr, argv); hit {
		return rc
	}
	args, rc := cpfsParseArgs(stderr, argv)
	if rc != 0 {
		return rc
	}
	if args.help {
		cpfsPrintHelp(stdout, args.prog)
		return 0
	}
	if args.selftest {
		return cpfsSelfTest(stdout)
	}
	if cpfsSelfTest(stdout) != 0 {
		fmt.Fprint(stdout, "\n✗ 自检未过 ⇒ 拒绝扫真目标（rc=2）\n")
		return 2
	}
	repo := cpfsPyAbspath(args.repo)
	paths := cpfsPathsFor(repo)
	for _, kv := range [][2]string{
		{"whitelist", args.whitelist}, {"private_paths", args.private},
		{"publish_sh", args.publishSh}, {"mirror_lib", args.mirrorLib},
	} {
		if kv[1] != "" {
			paths.cpfsSet(kv[0], cpfsPyAbspath(kv[1]))
		}
	}
	for _, kv := range paths.cpfsOrdered() {
		if !cpfsExists(kv[1]) {
			fmt.Fprintf(stdout, "✗ 缺件（%s）：%s ⇒ 拒绝给结论（rc=2）\n", kv[0], kv[1])
			return 2
		}
	}
	fmt.Fprintf(stdout, "仓：%s\n", repo)
	fmt.Fprint(stdout, "口径：候选集排除 `publish/`（旧器对它是结构性处置，不由 EXCLUDES 驱动）\n\n")
	code, lines := cpfsEvaluate(repo, paths)
	for _, ln := range lines {
		fmt.Fprintln(stdout, ln)
	}
	switch code {
	case 0:
		fmt.Fprint(stdout, "\n结果: 通过 —— 两器排除口径一致，私有面命中两器都已处理\n")
	case 1:
		fmt.Fprint(stdout, "\n结果: ✗ 发布面不同步（详见上面 ✗ 行）—— 真跑发布器会自中止/或两器漂移\n")
	default:
		fmt.Fprint(stdout, "\n结果: ✗ 拒绝给结论（无区分度或用法问题，rc=2）\n")
	}
	return code
}
