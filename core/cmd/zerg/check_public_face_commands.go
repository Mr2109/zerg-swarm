// check_public_face_commands.go —— 判据件 `scripts/gates/check-public-face-commands.py`（598 行）的 **Go 真搬**
// （v2.5.14 · M2 · 公开面命令名族 · 入口 `checkPublicFaceCommandsGateMain` · 承接 GAP-20261001-13）。
//
// 验收判据 = 同 argv 跑两态（py 原件 ↔ 本入口），stdout / stderr / 退码**三面逐字相同**。
//
// 声明 (a) 默认根定位差异：py `HERE = dirname(abspath(__file__))` · `REPO_ROOT = dirname(dirname(HERE))`
// ⇒ 默认根起于**脚本所在目录**（与 cwd 无关）；Go 无 `__file__` 语境 ⇒ `cpfcRepoRootGo()` 从 **cwd**
// 上溯 ≤8 层找含 `core/cmd/zerg/main.go` 的目录、找不到回落 `pyAbspath(cwd)`。
// 两态在「脚本位置 == cwd 上溯根」时逐字同构（两环境对拍即按此布置：夹具根放 `fx/scripts/gates/` + cwd=fx）。
// 另：`--root` / `--zerg` 走**显式入参**，缺省 `zerg = pyJoin(root, "bin", "zerg")`（与 py 逐字同）。
//
// 声明 (b) ★自指四形态：
//
//	① 本件在自己扫描面内吗 ⇒ **否**。本门扫面 = `publish/docs/**/*.md`（只收 `.md`），本件是 `.go`、
//	   落 `core/cmd/zerg/`；量化：本件零 `.md` 路径字面量，SPAN 面（反引号包裹的 zerg 形态）只在**运行时**
//	   读到的页正文里出现。
//	② 本件落在**他门**扫描面内吗 ⇒ **是**（`check-hardcoded-private-paths.py` 的 SCAN_DIRS 含 `core/`；
//	   `check-error-kinds` 的 R3 扫 `core/cmd/zerg/*.go`）⇒ 处置：本件**零私有绝对路径字面量**（盘根 /
//	   家目录一律运行时取；无盘根前缀、无家目录前缀），且不写错误构造调用原串。
//	③ self-test 魔法串连续字面落地 ⇒ 本件夹具串只落在**运行时建在临时目录下**的夹具件正文里，✗ 不落本件正文。
//	④ 本件**不接线** ⇒ 不进命令树 / 用法面 ⇒ 无新增被扫字面量。
//
// py 原件边界（✗ 不自行改 · 交回入账）：
//
//	① 读失败面**两侧已亮**（与 py **有意分叉** · 不再「逐字照搬」）：件面（`os.ReadFile` 失败 ⇒ 进
//	   `blocks`）与**目录面**（`cpfcWalkFace` 里 `os.ReadDir` 失败的目录 ⇒ 进 `cpfcReadFails`）——
//	   原 py 两处分别 `except OSError: continue` / `os.walk(onerror=None)` **静默少判**，Go 侧一律响亮 ⇒ rc=2。
//	② 件非合法 UTF-8 时 `open(..., encoding="utf-8").read()` 裸抛（最外层无兜底 ⇒ 回溯 rc=1）—— 本件按
//	   Go 语义读字节（不抛）⇒ 该格**只保 rc 之外的正文**，列入未覆盖面。
//	③ 子进程被信号终止时 py `returncode` 为负信号号，Go `ExitCode()` 返 -1 ⇒ 未覆盖。
//
// 未覆盖面（✗ 不声称逐字对齐）：
//
//	① argparse **长选项前缀缩写**（`--sel` ⇒ `--self-test`）本件不实现（只认逐字全名）。
//	② `-h`/`--help` 帮助正文按 py 解释器 3.9 体例（段标题 `optional arguments:` · 80 列折行）逐字复刻；
//	   py 侧实际折行随 `COLUMNS` / 终端宽度变 ⇒ 非 80 列时该格不属逐字承诺。
//	③ 子进程输出非 UTF-8 / 子进程被信号终止 / 公开面件非合法 UTF-8：见上 ②③。
//	④ py `\d`（Unicode 数字）本件按 `[0-9]`（ASCII）：自报数用非 ASCII 数字时两态分叉。
//	⑤ 默认根定位差异：见 (a)。
package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ── 常量（= py 件头常量 · 逐字）────────────────────────────────────────────────
const (
	cpfcFaceDir = "publish/docs"
	cpfcSelfRel = "scripts/gates/check-public-face-commands.py"
)

var cpfcPages = []string{"publish/docs/CLI.zh-CN.md", "publish/docs/CLI.en.md"}

// cpfcWSClass = py 正则里 `\s` 的 Unicode 语义（Go RE2 的 `\s` 是 ASCII ⇒ 显式展开）。
const cpfcWSClass = `[\t\n\v\f\r \x1c-\x1f\x85\xa0\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}]`

var (
	cpfcCMDRow   = regexp.MustCompile(`^  zerg` + cpfcWSClass + `+(.*)$`)
	cpfcTwoSP    = regexp.MustCompile(cpfcWSClass + `{2,}`)
	cpfcFlagRE   = regexp.MustCompile(`--[A-Za-z][A-Za-z0-9-]*`)
	cpfcSpanRE   = regexp.MustCompile("\x60zerg" + cpfcWSClass + "+([^\x60]+)\x60")
	cpfcDangHead = regexp.MustCompile(`危险动作（已登记` + cpfcWSClass + `*([0-9]+)` + cpfcWSClass + `*条`)
	cpfcDangTail = regexp.MustCompile(`上表` + cpfcWSClass + `*([0-9]+)` + cpfcWSClass + `*条`)
	cpfcGenBegin = regexp.MustCompile(`<!--` + cpfcWSClass + `*=+` + cpfcWSClass + `*生成区 BEGIN[^>]*-->`)
	cpfcGenEnd   = regexp.MustCompile(`<!--` + cpfcWSClass + `*=+` + cpfcWSClass + `*生成区 END[^>]*-->`)
)

// ── py 字符串 / 路径原语（自写同构 · ✗ 不用 filepath.Join / TrimSpace）──────────

// cpfcPyJoin = CPython posixpath.join 逐段规则（`..` 原样留 · 后段绝对丢前段 · 空段保尾 `/`）。
func cpfcPyJoin(parts ...string) string {
	path := ""
	if len(parts) > 0 {
		path = parts[0]
	}
	for _, b := range parts[1:] {
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

func cpfcPyNormpath(path string) string {
	if path == "" {
		return "."
	}
	initial := 0
	if strings.HasPrefix(path, "/") {
		initial = 1
	}
	if strings.HasPrefix(path, "//") && !strings.HasPrefix(path, "///") {
		initial = 2
	}
	var newComps []string
	for _, comp := range strings.Split(path, "/") {
		if comp == "" || comp == "." {
			continue
		}
		if comp != ".." || initial == 0 && len(newComps) == 0 ||
			len(newComps) > 0 && newComps[len(newComps)-1] == ".." {
			newComps = append(newComps, comp)
		} else if len(newComps) > 0 {
			newComps = newComps[:len(newComps)-1]
		}
	}
	res := strings.Join(newComps, "/")
	if initial > 0 {
		res = strings.Repeat("/", initial) + res
	}
	if res == "" {
		return "."
	}
	return res
}

func cpfcPyAbspath(p string) string {
	if !strings.HasPrefix(p, "/") {
		wd, err := os.Getwd()
		if err != nil {
			wd = ""
		}
		p = cpfcPyJoin(wd, p)
	}
	return cpfcPyNormpath(p)
}

func cpfcSplitComps(p string) []string {
	var out []string
	for _, c := range strings.Split(p, "/") {
		if c != "" {
			out = append(out, c)
		}
	}
	return out
}

func cpfcPyRelpath(path, start string) string {
	startAbs := cpfcPyAbspath(start)
	pathAbs := cpfcPyAbspath(path)
	sl := cpfcSplitComps(startAbs)
	pl := cpfcSplitComps(pathAbs)
	i := 0
	for i < len(sl) && i < len(pl) && sl[i] == pl[i] {
		i++
	}
	var rel []string
	for j := 0; j < len(sl)-i; j++ {
		rel = append(rel, "..")
	}
	rel = append(rel, pl[i:]...)
	if len(rel) == 0 {
		return "."
	}
	return cpfcPyJoin(rel...)
}

func cpfcPySplit(p string) (string, string) {
	i := strings.LastIndex(p, "/") + 1
	head, tail := p[:i], p[i:]
	if head != "" {
		if trimmed := strings.TrimRight(head, "/"); trimmed != "" {
			head = trimmed
		}
	}
	return head, tail
}

func cpfcPyDirname(p string) string {
	d, _ := cpfcPySplit(p)
	return d
}

func cpfcPyBase(p string) string {
	_, t := cpfcPySplit(p)
	return t
}

// cpfcPyIsSpace = CPython str.isspace()（★ 含 \x1c-\x1f ⇒ ✗ 不能用 unicode.IsSpace 单独了事）。
func cpfcPyIsSpace(r rune) bool {
	if r >= 0x1c && r <= 0x1f {
		return true
	}
	return unicode.IsSpace(r)
}

// cpfcPyStrip = CPython str.strip()。
func cpfcPyStrip(s string) string {
	return strings.TrimFunc(s, cpfcPyIsSpace)
}

// cpfcPyRStrip = CPython str.rstrip()。
func cpfcPyRStrip(s string) string {
	return strings.TrimRightFunc(s, cpfcPyIsSpace)
}

// cpfcSplitWS = CPython str.split()（无参 · 按 Unicode 空白切 · 丢弃空段）。
func cpfcSplitWS(s string) []string {
	return strings.FieldsFunc(s, cpfcPyIsSpace)
}

// cpfcNormNL = py `subprocess(text=True)` 的 universal newlines（`\r\n` 与裸 `\r` 都折成 `\n`）。
func cpfcNormNL(s string) string {
	if strings.IndexByte(s, '\r') < 0 {
		return s
	}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}

func cpfcIsFile(p string) bool {
	st, err := os.Stat(p)
	if err != nil {
		return false
	}
	return !st.IsDir()
}

func cpfcFirstN(s []string, n int) []string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func cpfcSortedKeysBool(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func cpfcSortedKeysStr(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func cpfcSortedKeysInt(m map[string]int) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// cpfcRepoRootGo = (a) 的 Go 默认根：cwd 上溯 ≤8 层找含 `core/cmd/zerg/main.go` 的目录，回落 pyAbspath(cwd)。
func cpfcRepoRootGo() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	d := wd
	for i := 0; i < 8; i++ {
		if cpfcIsFile(cpfcPyJoin(d, "core", "cmd", "zerg", "main.go")) {
			return d
		}
		nd := cpfcPyDirname(d)
		if nd == d {
			break
		}
		d = nd
	}
	return cpfcPyAbspath(wd)
}

// ── 子进程面（直调二进制取 rc / stdout / stderr · 不接管道 · 不见 shell）─────────

func cpfcRun(zerg string, args ...string) (int, string, string) {
	cmd := exec.Command(zerg, args...)
	var so, se bytes.Buffer
	cmd.Stdout = &so
	cmd.Stderr = &se
	err := cmd.Run()
	if cmd.ProcessState == nil {
		return exitBlocked, "", fmt.Sprintf("cannot execute %s: %s", zerg, err) // ★ 子进程**起不来**（非 ExitError）⇒ 读不到 ⇒ 归 `exitBlocked`(8「有 BLOCKED · 读不到不许当健康」)；
		// 原先裸写 127（shell 惯例）不在命令面真源表里 ⇒ 门⑪ T3 报（GAP-20261002-08）。
	}
	return cmd.ProcessState.ExitCode(), cpfcNormNL(so.String()), cpfcNormNL(se.String())
}

// ── help 面解析（= py `_path_of` / `parse_tree`）───────────────────────────────

// cpfcPathOf 返回 (path, pOK, usage, uOK)：py 的三态 (None,None) / (None,usage) / (path,usage)。
func cpfcPathOf(line string) (string, bool, string, bool) {
	m := cpfcCMDRow.FindStringSubmatch(line)
	if m == nil {
		return "", false, "", false
	}
	usage := cpfcPyStrip(cpfcTwoSP.Split(m[1], -1)[0])
	toks := cpfcSplitWS(usage)
	if len(toks) == 0 {
		return "", false, "", true
	}
	if toks[0][0] == '<' {
		return "", false, usage, true
	}
	var segs []string
	for _, t := range toks {
		if t[0] == '-' || t[0] == '<' || t[0] == '[' {
			break
		}
		segs = append(segs, t)
	}
	p := strings.Join(segs, " ")
	if p == "" {
		return "", false, usage, true
	}
	return p, true, usage, true
}

type cpfcNote struct {
	helpRC        int
	dangerRC      int
	generic       []string
	usageN        int
	dangerN       int
	claimedDanger int // -1 = None
	guard         []string
}

func cpfcPyNoneInt(i int) string {
	if i < 0 {
		return "None"
	}
	return strconv.Itoa(i)
}

func cpfcParseTree(zerg string) (map[string]string, map[string]string, *cpfcNote) {
	note := &cpfcNote{claimedDanger: -1}
	rc, out, _ := cpfcRun(zerg, "help")
	note.helpRC = rc
	if rc != 0 || cpfcPyStrip(out) == "" {
		note.guard = append(note.guard, fmt.Sprintf(
			"`zerg help` 现跑 rc=%d（stdout %d 字节）⇒ 树解析不上", rc, utf8.RuneCountInString(out)))
		return map[string]string{}, map[string]string{}, note
	}
	lines := strings.Split(out, "\n")
	iUse, iCmd, iDan := -1, -1, -1
	for i, l := range lines {
		if iUse < 0 && strings.HasPrefix(l, "用法") && strings.HasSuffix(cpfcPyRStrip(l), ":") {
			iUse = i
		}
		if iCmd < 0 && strings.HasPrefix(l, "命令（本版已开放") {
			iCmd = i
		}
		if iDan < 0 && cpfcDangHead.MatchString(l) {
			iDan = i
		}
	}
	if iUse < 0 || iCmd < 0 || iDan < 0 || !(iUse < iCmd && iCmd < iDan) {
		note.guard = append(note.guard, "帮助面结构对不上：找不到按序的「用法块 → 命令块 → 危险块」三块"+
			"（现读 `用法:`="+cpfcPyNoneInt(iUse)+" · 命令块="+cpfcPyNoneInt(iCmd)+
			" · 危险块="+cpfcPyNoneInt(iDan)+"）")
		return map[string]string{}, map[string]string{}, note
	}
	usage := map[string]string{}
	var generic []string
	block := append(append([]string{}, lines[iUse+1:iCmd]...), lines[iCmd+1:iDan]...)
	for _, l := range block {
		p, pOK, u, uOK := cpfcPathOf(l)
		if !pOK {
			if uOK {
				toks := cpfcSplitWS(u)
				if len(toks) > 0 && toks[0][0] == '<' {
					generic = append(generic, u)
				}
			}
			continue
		}
		usage[p] = u
	}
	note.generic = generic
	note.usageN = len(usage)

	rc2, out2, _ := cpfcRun(zerg, "help", "dangerous")
	note.dangerRC = rc2
	if rc2 != 0 || cpfcPyStrip(out2) == "" {
		note.guard = append(note.guard, fmt.Sprintf("`zerg help dangerous` 现跑 rc=%d ⇒ 危险块解析不上", rc2))
		return map[string]string{}, map[string]string{}, note
	}
	danger := map[string]string{}
	for _, l := range strings.Split(out2, "\n") {
		p, pOK, _, _ := cpfcPathOf(l)
		if !pOK {
			continue
		}
		danger[p] = cpfcPyStrip(l)
	}
	note.dangerN = len(danger)

	if len(usage) == 0 || len(danger) == 0 {
		note.guard = append(note.guard, fmt.Sprintf("树为空转：用法/命令块 %d 条 · 危险块 %d 条", len(usage), len(danger)))
	}
	claimed := -1
	for _, l := range lines {
		if m := cpfcDangHead.FindStringSubmatch(l); m != nil {
			claimed = cpfcAtoi(m[1])
			break
		}
	}
	for _, l := range strings.Split(out2, "\n") {
		if m := cpfcDangTail.FindStringSubmatch(l); m != nil {
			if claimed < 0 {
				claimed = cpfcAtoi(m[1])
			}
			tail := cpfcAtoi(m[1])
			if tail != len(danger) {
				note.guard = append(note.guard, fmt.Sprintf(
					"危险块**自报数**（`上表 %d 条`）≠ 解析出的 %d 条 ⇒ 判不了", tail, len(danger)))
			}
			break
		}
	}
	if claimed >= 0 && claimed != len(danger) {
		note.guard = append(note.guard, fmt.Sprintf(
			"危险块表头自报「已登记 %d 条」≠ 解析出的 %d 条 ⇒ 判不了", claimed, len(danger)))
	}
	var both []string
	for k := range usage {
		if _, ok := danger[k]; ok {
			both = append(both, k)
		}
	}
	sort.Strings(both)
	if len(both) > 0 {
		note.guard = append(note.guard, fmt.Sprintf("命令块 ∩ 危险块 = %d 条（%s）⇒ 两集合本应不相交（现读 0）",
			len(both), strings.Join(cpfcFirstN(both, 4), " · ")))
	}
	note.claimedDanger = claimed
	return usage, danger, note
}

func cpfcAtoi(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}

// cpfcGenRegion = py `gen_region`。
func cpfcGenRegion(text string) (string, bool) {
	mb := cpfcGenBegin.FindStringIndex(text)
	me := cpfcGenEnd.FindStringIndex(text)
	if mb == nil || me == nil || me[0] < mb[1] {
		return "", false
	}
	return text[mb[1]:me[0]], true
}

// cpfcDecodeText = py `open(..., encoding="utf-8").read()` 的可移植面（universal newlines；解码差异见未覆盖面②）。
func cpfcDecodeText(b []byte) string {
	return cpfcNormNL(string(b))
}

// cpfcReadFails —— B 类读失败登记面（读不了的公开面**目录** ⇒ 逐处登记 相对路径 + err 正文）。
// ★ 同族先例 `check_doc_cmds.go` 的 `dcmdReadFails` · `check_bypass_idioms.go` 的 `cbiReadFails`
// （件面 + 目录面）· `check_selftest_knobs.go` 的 `cskReadFails`。本件件面（`os.ReadFile` 失败）
// 早已亮（⇒ `blocks` ⇒ rc=2）；本面收的是**遍历面残留**：`os.ReadDir` 失败的目录。
// 读失败 ⇒ 登记 + 入口判红（stderr BLOCKED 头 + 逐处点名）⇒ rc=2（✗ 不再「读不了的目录静默跳过」）。
// 自清点：每次 `cpfcJudge` 起手清空 ⇒ 不许跨调用残留（残留会让下一次一进门就被判红）。
var cpfcReadFails []string

// ── 公开面遍历（复刻 os.walk 产出序：当前层全部文件先出，再按目录名序下钻）─────────

func cpfcWalkFace(root, face string) []string {
	var files []string
	var rec func(dp string)
	rec = func(dp string) {
		entries, err := os.ReadDir(dp) // 按文件名排序（= py 的 sorted(fns) / sorted(dns)）
		if err != nil {
			// ★ B 类真口子修（目录面 · 与 py `os.walk(onerror=None)` 有意分叉）：
			//   读不了的目录 ⇒ 登记 + 入口判红 + rc=2（✗ 不再静默跳过 ⇒ 少判）。
			cpfcReadFails = append(cpfcReadFails, fmt.Sprintf("%s（%v）", cpfcPyRelpath(dp, root), err))
			return
		}
		var fns, dns []string
		for _, e := range entries {
			name := e.Name()
			full := cpfcPyJoin(dp, name)
			st, serr := os.Stat(full) // 跟随符号链接（= py entry.is_dir()）
			if serr == nil && st.IsDir() {
				if !strings.HasPrefix(name, ".") && e.Type()&os.ModeSymlink == 0 {
					dns = append(dns, name) // followlinks=False ⇒ 指向目录的链接不下钻
				}
				continue
			}
			fns = append(fns, name)
		}
		for _, fn := range fns {
			if strings.HasPrefix(fn, ".") || !strings.HasSuffix(fn, ".md") {
				continue
			}
			files = append(files, cpfcPyJoin(dp, fn))
		}
		for _, dn := range dns {
			rec(cpfcPyJoin(dp, dn))
		}
	}
	rec(face)
	return files
}

// ── 判据体（= py `judge`）──────────────────────────────────────────────────────

type cpfcHit struct {
	Rel  string
	Code string
	Ref  string
	Why  string
}

type cpfcData struct {
	root       string
	zerg       string
	tree       int
	danger     int
	spans      int
	generic    int
	groupRef   int
	files      int
	treeTotal  int
	blocks     []string
	hits       []cpfcHit
	topics     map[string]int
	helpUsage  map[string]string
	unjudged   []string
	unparsed   []string
	hasTreeTot bool
}

func cpfcBlockedLines(blocks []string, tail string) []string {
	out := make([]string, 0, len(blocks)+1)
	for _, b := range blocks {
		out = append(out, "⛔ "+b)
	}
	out = append(out, tail)
	return out
}

func cpfcJudge(root, zerg string, selfExempt bool) (int, []string, *cpfcData) {
	root = cpfcPyAbspath(root)
	if zerg == "" {
		zerg = cpfcPyJoin(root, "bin", "zerg")
	}
	face := cpfcPyJoin(root, cpfcFaceDir)
	var lines []string
	var blocks []string
	var hits []cpfcHit
	data := &cpfcData{root: root, zerg: zerg, hits: []cpfcHit{}}
	cpfcReadFails = nil // ★ 自清：包级登记面不许跨调用残留（残留会让下一次一进门就被判红）

	st, err := os.Stat(face)
	if err != nil || !st.IsDir() {
		return 2, []string{"⛔ 缺件：" + cpfcFaceDir + "/ 不在（缺件 ≠ 绿 ⇒ 不给结论）"}, data
	}
	if !cpfcIsFile(zerg) {
		alt := cpfcPyJoin(cpfcRepoRootGo(), "bin", "zerg")
		if cpfcIsFile(alt) {
			lines = append(lines, "⚠ 真源二进制改指：`"+zerg+"` 不在 ⇒ 回落 `"+alt+"`（夹具/负控专用 · "+
				"与 `ZERG_PUBFACE_ROOT` 同族）")
			zerg = alt
			data.zerg = zerg
		} else {
			return 2, []string{"⛔ 缺件：真源二进制 " + zerg + " 不在（缺件 ≠ 绿 ⇒ 不给结论）"}, data
		}
	}
	if !cpfcIsFile(zerg) {
		return 2, []string{"⛔ 缺件：真源二进制 " + zerg + " 不在（缺件 ≠ 绿 ⇒ 不给结论）"}, data
	}

	usage, danger, note := cpfcParseTree(zerg)
	data.tree, data.danger = len(usage), len(danger)
	data.blocks = append([]string{}, note.generic...)
	data.generic = len(data.blocks)
	blocks = append(blocks, note.guard...)
	if len(blocks) > 0 {
		return 2, cpfcBlockedLines(blocks, "⛔ 树解析判不了 ⇒ **不给结论（rc=2）**"), data
	}

	treeSet := map[string]bool{}
	for k := range usage {
		treeSet[k] = true
	}
	for k := range danger {
		treeSet[k] = true
	}
	tree := cpfcSortedKeysBool(treeSet)
	data.treeTotal = len(tree)
	lines = append(lines, fmt.Sprintf("真源：`%s` 现读 —— 命令块 **%d** 条 + 危险块 **%d** 条 = **%d** 条",
		cpfcPyBase(zerg), len(usage), len(danger), len(tree)))

	files := cpfcWalkFace(root, face)
	faceTexts := map[string]string{}
	for _, fp := range files {
		rel := cpfcPyRelpath(fp, root)
		if selfExempt && rel == cpfcSelfRel {
			continue
		}
		b, rerr := os.ReadFile(fp)
		if rerr != nil {
			blocks = append(blocks, fmt.Sprintf("件读不了：%s（%s）", rel, rerr))
			continue
		}
		faceTexts[rel] = cpfcDecodeText(b)
	}
	var spans [][2]string
	for _, rel := range cpfcSortedKeysStr(faceTexts) {
		for _, m := range cpfcSpanRE.FindAllStringSubmatch(faceTexts[rel], -1) {
			spans = append(spans, [2]string{rel, cpfcPyStrip(m[1])})
		}
	}
	data.spans = len(spans)
	data.files = len(faceTexts)
	if len(blocks) > 0 {
		return 2, cpfcBlockedLines(blocks, "⛔ 缺件/判不了 ⇒ **不给结论（rc=2）**（缺件 ≠ 绿）"), data
	}
	if len(spans) == 0 {
		return 2, []string{fmt.Sprintf("⛔ 空转：公开面 **%d** 件里 **0** 个 `` `zerg …` `` 串 ⇒ 判据无区分度"+
			"（不给结论 · 空转 = 假覆盖）", len(faceTexts))}, data
	}

	// ── 方向 A（树 → 页）──────────────────────────────────────────────────────
	lines = append(lines, fmt.Sprintf("── 方向 A（`undocumented-command`）树 **%d** 条逐条须在页的生成区里有格 ──", len(tree)))
	for _, rel := range cpfcPages {
		fp := cpfcPyJoin(root, rel)
		if !cpfcIsFile(fp) {
			blocks = append(blocks, fmt.Sprintf("页缺件：%s 不在（方向 A 判不了）", rel))
			continue
		}
		b, rerr := os.ReadFile(fp)
		if rerr != nil {
			blocks = append(blocks, fmt.Sprintf("页读不了：%s（%s）", rel, rerr))
			continue
		}
		body, ok := cpfcGenRegion(cpfcDecodeText(b))
		if !ok {
			blocks = append(blocks, fmt.Sprintf("页 %s 的生成区标记找不到（`生成区 BEGIN`/`END`）⇒ 判不了", rel))
			continue
		}
		var miss []string
		for _, c := range tree {
			if !strings.Contains(body, "zerg "+c) {
				miss = append(miss, c)
			}
		}
		if len(miss) > 0 {
			for _, c := range cpfcFirstN(miss, 12) {
				hits = append(hits, cpfcHit{Rel: rel, Code: "undocumented-command", Ref: "zerg " + c,
					Why: "树里有这条命令、页的生成区里找不到它 ⇒ 树里新了、页没跟上"})
			}
			lines = append(lines, fmt.Sprintf("   ✗ %s：**%d 条**树命令在生成区里没有格（%s）",
				rel, len(miss), strings.Join(cpfcFirstN(miss, 6), " · ")))
		} else {
			lines = append(lines, fmt.Sprintf("   ✓ %s：生成区 **%d/%d** 条逐条有格", rel, len(tree), len(tree)))
		}
	}
	if len(blocks) > 0 {
		return 2, cpfcBlockedLines(blocks, "⛔ 缺件/判不了 ⇒ **不给结论（rc=2）**（缺件 ≠ 绿）"), data
	}

	// ── 方向 B（页 → 树）──────────────────────────────────────────────────────
	lines = append(lines, fmt.Sprintf("── 方向 B（`nonexistent-command-documented` / `nonexistent-parameter-documented`）"+
		"公开面 **%d** 件里的 `` `zerg …` `` 串逐条判 ──", len(faceTexts)))
	topics := map[string]int{}
	helpUsage := map[string]string{}
	var unjudged []string
	var unparsed []string // ★ GAP-20260927-86：抽不出命令路径的串（首词 `-`/`[`）

	flagFace := func(path string) (string, bool) {
		if u, ok := usage[path]; ok {
			return u, true
		}
		if _, ok := danger[path]; ok {
			if _, seen := helpUsage[path]; !seen {
				rc3, out3, _ := cpfcRun(zerg, append(cpfcSplitWS(path), "--help")...)
				first := cpfcPyStrip(strings.Split(out3, "\n")[0])
				if rc3 == 0 && strings.HasPrefix(first, "zerg ") {
					first = first[len("zerg "):]
				} else {
					first = ""
				}
				helpUsage[path] = first
			}
			if v := helpUsage[path]; v != "" {
				return v, true
			}
			return "", false
		}
		return "", false
	}

	for _, sp := range spans {
		rel, body := sp[0], sp[1]
		toks := cpfcSplitWS(body)
		if len(toks) == 0 {
			continue
		}
		if toks[0][0] == '<' {
			data.generic++
			continue
		}
		var segs []string
		for _, t := range toks {
			if t[0] == '-' || t[0] == '<' || t[0] == '[' {
				break
			}
			segs = append(segs, t)
		}
		path := strings.Join(segs, " ")
		// ★ GAP-20260927-86：首词是 `-`/`[` 的串（R1 守卫只认 `<`）⇒ segs 为空 ⇒ path 得空串（非 None）。
		//   登记为「判不了」并 continue（照本门 flagFace 的 `or None` 归口 + 「判不了 ⇒ rc=2」先例）。
		if path == "" {
			unparsed = append(unparsed, fmt.Sprintf("%s `zerg %s`", rel, body))
			continue
		}
		usedSet := map[string]bool{}
		for _, f := range cpfcFlagRE.FindAllString(body, -1) {
			usedSet[strings.SplitN(f, "=", 2)[0]] = true
		}
		used := cpfcSortedKeysBool(usedSet)
		topic := ""
		hasTopic := false
		if !treeSet[path] {
			parts := strings.Split(path, " ")
			if len(parts) > 0 && parts[0] == "help" && len(parts) > 1 &&
				!strings.HasPrefix(parts[1], "-") && !strings.HasPrefix(parts[1], "<") {
				topic, path, hasTopic = parts[1], "help", true
			}
		}
		if treeSet[path] {
			if hasTopic {
				if _, ok := topics[topic]; !ok {
					rc, _, _ := cpfcRun(zerg, "help", topic)
					topics[topic] = rc
				}
				if topics[topic] != 0 {
					hits = append(hits, cpfcHit{Rel: rel, Code: "nonexistent-help-topic",
						Ref: fmt.Sprintf("`zerg %s`", body),
						Why: fmt.Sprintf("主题 `%s` 真跑 `zerg help %s` ⇒ rc=%s（树面里没有这个主题）",
							topic, topic, strconv.Itoa(topics[topic]))})
				}
				continue
			}
			if len(used) == 0 {
				continue
			}
			uface, ok := flagFace(path)
			if !ok {
				unjudged = append(unjudged, fmt.Sprintf("%s `zerg %s`（%s）", rel, body, path))
				continue
			}
			uflags := map[string]bool{}
			for _, f := range cpfcFlagRE.FindAllString(uface, -1) {
				uflags[f] = true
			}
			badSet := map[string]bool{}
			for _, f := range used {
				if !uflags[f] {
					badSet[f] = true
				}
			}
			if len(badSet) > 0 {
				for _, f := range cpfcSortedKeysBool(badSet) {
					hits = append(hits, cpfcHit{Rel: rel, Code: "nonexistent-parameter-documented",
						Ref: fmt.Sprintf("`zerg %s`", body),
						Why: fmt.Sprintf("旗标 %s 不在 `%s` 的用法串里（现读用法串 = `zerg %s` ⇒ "+
							"要么页写错、要么该命令的用法串漏声明）", f, path, uface)})
				}
				continue
			}
		} else {
			pref := false
			for c := range treeSet {
				if strings.HasPrefix(c, path+" ") {
					pref = true
					break
				}
			}
			if pref {
				data.groupRef++
				continue
			}
			hits = append(hits, cpfcHit{Rel: rel, Code: "nonexistent-command-documented",
				Ref: fmt.Sprintf("`zerg %s`", body),
				Why: fmt.Sprintf("命令路径 `%s` 既不在树里、也不是任何树命令的前缀", path)})
			continue
		}
	}
	data.topics = topics
	data.helpUsage = helpUsage
	data.unjudged = unjudged
	data.unparsed = unparsed
	lines = append(lines, fmt.Sprintf("   · 扫到 `` `zerg …` `` 串 **%d** 个（其中通用式行 %d · 族/子族引用 %d · 主题面 %d 个主题）",
		data.spans, data.generic, data.groupRef, len(topics)))
	if len(helpUsage) > 0 {
		lines = append(lines, fmt.Sprintf("   · 危险命令的旗标真源：现跑 `zerg <命令> --help` 取用法串 **%d** 条（%s）",
			len(helpUsage), strings.Join(cpfcFirstN(cpfcSortedKeysStr(helpUsage), 6), " · ")))
	}
	if len(unjudged) > 0 {
		lines = append(lines, fmt.Sprintf("   · **未判**旗标 **%d** 处（该命令的用法串两处都取不到 ⇒ 照实登记、不当绿）"+
			"：%s", len(unjudged), strings.Join(cpfcFirstN(unjudged, 4), " · ")))
		// ★ GAP-20260927-95：这一档原只打印「不当绿」、退码仍落 `if hits` 之后的 0 ⇒
		//   同屏「不当绿」紧跟「全绿 rc=0」。「未判」= 取不到真源 = 判不了 ⇒ 按三档 2 收口（2 > 1 > 0）。
		lines = append(lines, fmt.Sprintf("⇒ **不给结论（rc=2）**：%d 处旗标**未判**（用法串两处都取不到）——"+
			"「取不到」≠ 全绿", len(unjudged)))
		return 2, lines, data
	}
	// ★ GAP-20260927-86：抽不出命令路径的串（首词 `-`/`[`）= 判不了 ⇒ 按三档 2 收口（2 > 1 > 0）。
	if len(unparsed) > 0 {
		lines = append(lines, fmt.Sprintf("   · **判不了**（抽不出命令路径的串）**%d** 处：%s",
			len(unparsed), strings.Join(cpfcFirstN(unparsed, 4), " · ")))
		lines = append(lines, fmt.Sprintf("⇒ **不给结论（rc=2）**：%d 处 `zerg …` 串抽不出路径（首词 `-`/`[` · 判不了优先于有红）",
			len(unparsed)))
		return 2, lines, data
	}
	data.hits = hits
	if len(hits) > 0 {
		for _, h := range hits {
			lines = append(lines, fmt.Sprintf("   ✗ %s [%s] %s —— %s", h.Rel, h.Code, h.Ref, h.Why))
		}
		lines = append(lines, fmt.Sprintf("⇒ **有红（rc=1）**：方向 A/B 合计 **%d** 处例外（例外须 0 · 逐条在案、不静默过滤）", len(hits)))
		return 1, lines, data
	}
	lines = append(lines, fmt.Sprintf("⇒ 全绿（rc=0）：方向 A **%d** 条逐条有格 · 方向 B **%d** 串零例外",
		data.treeTotal, data.spans))
	return 0, lines, data
}

// ── 合成夹具自检（成对负控 · 不碰真目标）────────────────────────────────────────

func cpfcMakeFixture(base string, dropRows, extra []string, cleanExceptions, stub bool, pages map[string]string) (string, bool) {
	root, err := os.MkdirTemp(base, "pfcmds-")
	if err != nil {
		return "", false
	}
	_ = os.MkdirAll(cpfcPyJoin(root, cpfcFaceDir), 0o755)
	for _, rel := range cpfcPages {
		pg := cpfcPyJoin(root, rel)
		if stub {
			_ = os.WriteFile(pg, []byte("<!-- ===== 生成区 BEGIN · 夹具 ===== -->\n夹具：无命令引用\n"+
				"<!-- ===== 生成区 END ===== -->\n"), 0o644)
			continue
		}
		b, rerr := os.ReadFile(cpfcPyJoin(cpfcRepoRootGo(), rel))
		if rerr != nil {
			return root, false
		}
		text := cpfcDecodeText(b)
		if cleanExceptions {
			text = strings.ReplaceAll(text, "\x60zerg context ls --resume\x60", "\x60zerg context ls\x60")
		}
		if len(dropRows) > 0 {
			var keep []string
			for _, l := range strings.Split(text, "\n") {
				skip := false
				for _, d := range dropRows {
					if strings.Contains(l, d+"\x60") && strings.Contains(l, "| \x60"+d) {
						skip = true
						break
					}
				}
				if skip {
					continue
				}
				keep = append(keep, l)
			}
			text = strings.Join(keep, "\n")
		}
		if len(extra) > 0 {
			text += "\n" + strings.Join(extra, "\n") + "\n"
		}
		_ = os.WriteFile(pg, []byte(text), 0o644)
	}
	for name, body := range pages {
		_ = os.WriteFile(cpfcPyJoin(root, cpfcFaceDir, name), []byte(body), 0o644)
	}
	return root, true
}

func cpfcSelfTest() (int, []string) {
	var lines []string
	var ok []bool
	check := func(cond bool, desc string) {
		ok = append(ok, cond)
		mark := "✗"
		if cond {
			mark = "✓"
		}
		lines = append(lines, "   "+mark+" "+desc)
	}
	real := cpfcPyJoin(cpfcRepoRootGo(), "bin", "zerg")
	if !cpfcIsFile(real) {
		lines = append(lines, fmt.Sprintf("   ✗ 前置缺件：%s 不在 ⇒ 自检不给结论", real))
		return 2, lines
	}
	base, err := os.MkdirTemp("", "pfcmds-st-")
	if err != nil {
		lines = append(lines, "   ✗ 前置缺件：临时基目录建不起来 ⇒ 自检不给结论")
		return 2, lines
	}
	defer os.RemoveAll(base)

	// 格 1 正控（`GAP-20260927-11`）：**合成页** —— 绑「机制」不绑「世界」。
	//   旧形态拿**真页副本**当输入、只靠一处写死的例外串（py `:4xx` 的 `replace`）清洗 ⇒
	//   真世界一旦冒出**任何新例外**（页里多一条 `zerg <命令> <不存在的旗标>`），格 1 必红
	//   ⇒ 自检不过 ⇒ 门**拒绝进真目标判别**（fail-closed 自锁）⇒「门红」与「真目标到底
	//   几处例外」**脱钩**，人看不到真读数。
	//   新形态：页内容由**现读命令树**生成（逐条 `zerg <path>`、不含任何旗标引用）⇒
	//   方向 A 逐条有格、方向 B 零例外 —— rc 恒 0，与世界漂移无关（drift-immune）。
	//   真页的实判仍由格 8（`rc in (0,1)` · 漂移免疫形态）承担，本格不再重复它的面。
	//   （`cleanExceptions` 一处例外串清洗保留：格 2–5 仍在用 ⇒ 别顺手删。）
	u1, d1, _ := cpfcParseTree(real)
	set1 := map[string]struct{}{}
	for k := range u1 {
		set1[k] = struct{}{}
	}
	for k := range d1 {
		set1[k] = struct{}{}
	}
	var tree1 []string
	for k := range set1 {
		tree1 = append(tree1, k)
	}
	sort.Strings(tree1)
	if len(tree1) == 0 {
		check(false, "格 1 正控前置：现读命令树解析为空 ⇒ 建不起合成页（不给结论）")
	} else {
		var body strings.Builder
		body.WriteString("<!-- ===== 生成区 BEGIN · 合成夹具（GAP-20260927-11）===== -->\n")
		for i, c := range tree1 {
			if i > 0 {
				body.WriteString("\n")
			}
			body.WriteString("命令 `zerg " + c + "`。")
		}
		body.WriteString("\n<!-- ===== 生成区 END ===== -->\n")
		pages := map[string]string{}
		for _, p := range cpfcPages {
			pages[cpfcPyBase(p)] = body.String()
		}
		r1, _ := cpfcMakeFixture(base, nil, nil, false, true, pages)
		rc, _, _ := cpfcJudge(r1, real, true)
		check(rc == 0, fmt.Sprintf("格 1 正控（合成页：命令树 %d 条逐条有格 + 零例外）⇒ rc=0（实测 rc=%d）",
			len(tree1), rc))
	}

	r2, _ := cpfcMakeFixture(base, []string{"zerg version"}, nil, true, false, nil)
	rc, _, d2 := cpfcJudge(r2, real, true)
	check(rc == 1 && cpfcHasCode(d2, "undocumented-command"),
		fmt.Sprintf("格 2 负控（页里删掉 `zerg version` 那一格）⇒ rc=1 + `undocumented-command`（实测 rc=%d）", rc))

	r3, _ := cpfcMakeFixture(base, nil, []string{"见 `zerg frobnicate --xyz`。"}, true, false, nil)
	rc, _, d3 := cpfcJudge(r3, real, true)
	check(rc == 1 && cpfcHasCode(d3, "nonexistent-command-documented"),
		fmt.Sprintf("格 3 负控（页里塞 `zerg frobnicate`）⇒ rc=1 + `nonexistent-command-documented`（实测 rc=%d）", rc))

	r4, _ := cpfcMakeFixture(base, nil, []string{"见 `zerg version --frob`。"}, true, false, nil)
	rc, _, d4 := cpfcJudge(r4, real, true)
	check(rc == 1 && cpfcHasCode(d4, "nonexistent-parameter-documented"),
		fmt.Sprintf("格 4 负控（真命令 + 假旗标 `--frob`）⇒ rc=1 + `nonexistent-parameter-documented`（实测 rc=%d）", rc))

	r5, _ := cpfcMakeFixture(base, nil, []string{"见 `zerg help frobnicate`。"}, true, false, nil)
	rc, _, d5 := cpfcJudge(r5, real, true)
	check(rc == 1 && cpfcHasCode(d5, "nonexistent-help-topic"),
		fmt.Sprintf("格 5 负控（假主题 `zerg help frobnicate`）⇒ rc=1 + `nonexistent-help-topic`（实测 rc=%d）", rc))

	r6, _ := cpfcMakeFixture(base, nil, nil, false, true, nil)
	rc, _, _ = cpfcJudge(r6, real, true)
	check(rc == 2, fmt.Sprintf("格 6 空转负控（公开面 0 个 `zerg …` 串）⇒ rc=2（实测 rc=%d）", rc))

	var r7 string
	if d, derr := os.MkdirTemp(base, "pfcmds-no-"); derr == nil {
		r7 = d
		_ = os.MkdirAll(cpfcPyJoin(r7, cpfcFaceDir, "design"), 0o755)
	}
	rc, _, _ = cpfcJudge(r7, real, true)
	check(rc == 2, fmt.Sprintf("格 7 缺件负控（页不在 ⇒ 方向 A 判不了）⇒ rc=2（实测 rc=%d）", rc))

	rc, _, d8 := cpfcJudge(cpfcRepoRootGo(), real, true)
	check((rc == 0 || rc == 1) && d8.spans > 0,
		fmt.Sprintf("格 8 真树现跑：rc=%d · 扫到 %d 串 · 例外 %d 处（照实：本格只证「判得动」，不要求 0）",
			rc, d8.spans, len(d8.hits)))

	// 格 9 判不了负控（GAP-20260927-86）：页里塞首词为 `-`/`[` 的串 ⇒ 旧版 IndexError 崩（与有红同退码 rc=1）。
	//   改后必须 rc=2 且 data 里记 unparsed（判不了优先于有红），不崩。
	r9, _ := cpfcMakeFixture(base, nil, []string{"见 `zerg -x 形态`。", "见 `zerg [对象] ls`。"}, true, false, nil)
	rc, _, d9 := cpfcJudge(r9, real, true)
	check(rc == 2 && len(d9.unparsed) >= 1,
		fmt.Sprintf("格 9 判不了负控（串首词 `-`/`[` ⇒ 抽不出路径）⇒ rc=2 + data.unparsed 非空（实测 rc=%d · unparsed=%d）",
			rc, len(d9.unparsed)))

	if cpfcAllTrue(ok) {
		lines = append(lines, fmt.Sprintf("   ⇒ 自检全过（%d 格）", len(ok)))
		return 0, lines
	}
	lines = append(lines, fmt.Sprintf("   ⇒ 自检不过（%d/%d 格）", cpfcCountTrue(ok), len(ok)))
	return 2, lines
}

func cpfcHasCode(d *cpfcData, code string) bool {
	if d == nil {
		return false
	}
	for _, h := range d.hits {
		if h.Code == code {
			return true
		}
	}
	return false
}

func cpfcAllTrue(ok []bool) bool {
	for _, b := range ok {
		if !b {
			return false
		}
	}
	return true
}

func cpfcCountTrue(ok []bool) int {
	n := 0
	for _, b := range ok {
		if b {
			n++
		}
	}
	return n
}

// ── `--json <字段>` 字段名打错 ⇒ 用法错 rc=2 ＋ 印可选字段表（缺口 `Q-176`）──────

// cpfcJSONFields 本门机器读面字段表（唯一真源：印表与拒收同读这一处）。
var cpfcJSONFields = []string{"counts", "hits", "topics"}

func cpfcRefuseUnknownJSONFields(stderr io.Writer, argv []string) (int, bool) {
	var want []string
	for i, x := range argv {
		if x == "--json" {
			v := ""
			if i+1 < len(argv) {
				v = argv[i+1]
			}
			want = append(want, strings.Split(v, ",")...)
		} else if strings.HasPrefix(x, "--json=") {
			want = append(want, strings.Split(strings.SplitN(x, "=", 2)[1], ",")...)
		}
	}
	var clean []string
	for _, f := range want {
		if cpfcPyStrip(f) != "" {
			clean = append(clean, cpfcPyStrip(f))
		}
	}
	var bad []string
	for _, f := range clean {
		found := false
		for _, g := range cpfcJSONFields {
			if f == g {
				found = true
				break
			}
		}
		if !found {
			bad = append(bad, f)
		}
	}
	if len(bad) == 0 {
		return 0, false
	}
	quoted := make([]string, 0, len(bad))
	for _, b := range bad {
		quoted = append(quoted, "\x60"+b+"\x60")
	}
	var fquoted []string
	for _, f := range cpfcJSONFields {
		fquoted = append(fquoted, "\x60"+f+"\x60")
	}
	fmt.Fprintln(stderr, "⛔ `--json` 不认的字段名："+strings.Join(quoted, " · "))
	fmt.Fprintln(stderr, "   可选字段表："+strings.Join(fquoted, " · "))
	fmt.Fprintln(stderr, "⇒ 用法错（rc=2）：**字段名打错 ≠ 零命中** —— 两态在机器面上必须分得开")
	return 2, true
}

// ── argparse 面（子集 · 逐字复刻 3.9 体例；前缀缩写见未覆盖面①）─────────────────

const cpfcProg = "check-public-face-commands.py"

const cpfcUsageLine = "usage: " + cpfcProg + " [-h] [--root ROOT] [--zerg ZERG]\n" +
	"                                     [--json JSON] [--self-test]\n" +
	"                                     [--no-self-test] [--read-only-count]\n"

const cpfcHelpText = cpfcUsageLine +
	"\n" +
	"公开面命令名抽取器 + 反向探针（`T-14` · `V10`）\n" +
	"\n" +
	"optional arguments:\n" +
	"  -h, --help         show this help message and exit\n" +
	"  --root ROOT        判据树根（默认仓根）\n" +
	"  --zerg ZERG        真源二进制（默认 <root>/bin/zerg）\n" +
	"  --json JSON        机器读面：逗号分隔 counts,hits,topics\n" +
	"  --self-test        只跑自检（合成夹具 · 不碰真目标）\n" +
	"  --no-self-test     跳过开跑前的自检\n" +
	"  --read-only-count  自检不过仍打印只读计数（**不给结论**：rc 仍退 2，只多一份机器可读计数；GAP-20260926-147）\n"

type cpfcArgs struct {
	root          string
	zerg          string
	json          string
	selfTest      bool
	noSelfTest    bool
	readOnlyCount bool
}

func cpfcArgError(stderr io.Writer, msg string) int {
	fmt.Fprint(stderr, cpfcUsageLine)
	fmt.Fprintf(stderr, "%s: error: %s\n", cpfcProg, msg)
	return 2
}

func cpfcLooksLikeOption(s string) bool {
	return len(s) > 1 && strings.HasPrefix(s, "-")
}

func cpfcParseArgs(stdout, stderr io.Writer, argv []string) (cpfcArgs, int, bool) {
	a := cpfcArgs{root: cpfcRepoRootGo()}
	var extras []string
	for i := 0; i < len(argv); i++ {
		x := argv[i]
		if x == "--" {
			extras = append(extras, argv[i+1:]...)
			break
		}
		if x == "-h" || x == "--help" {
			fmt.Fprint(stdout, cpfcHelpText)
			return a, 0, true
		}
		if strings.HasPrefix(x, "--") {
			name, val := x, ""
			hasVal := false
			if eq := strings.Index(x, "="); eq >= 0 {
				name, val, hasVal = x[:eq], x[eq+1:], true
			}
			switch name {
			case "--root", "--zerg", "--json":
				if !hasVal {
					if i+1 >= len(argv) || cpfcLooksLikeOption(argv[i+1]) {
						return a, cpfcArgError(stderr, "argument "+name+": expected one argument"), true
					}
					i++
					val = argv[i]
				}
				switch name {
				case "--root":
					a.root = val
				case "--zerg":
					a.zerg = val
				case "--json":
					a.json = val
				}
			case "--self-test", "--no-self-test", "--read-only-count":
				if hasVal {
					return a, cpfcArgError(stderr, "argument "+name+": ignored explicit argument '"+val+"'"), true
				}
				switch name {
				case "--self-test":
					a.selfTest = true
				case "--no-self-test":
					a.noSelfTest = true
				default:
					a.readOnlyCount = true
				}
			default:
				extras = append(extras, x)
			}
			continue
		}
		if cpfcLooksLikeOption(x) {
			extras = append(extras, x)
			continue
		}
		extras = append(extras, x)
	}
	if len(extras) > 0 {
		return a, cpfcArgError(stderr, "unrecognized arguments: "+strings.Join(extras, " ")), true
	}
	return a, 0, false
}

// ── `--json` 输出（复刻 json.dumps(ensure_ascii=False, sort_keys=True) 的形态）──

func cpfcJSONStr(s string) string {
	var b strings.Builder
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
				fmt.Fprintf(&b, "\\u%04x", r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

func cpfcJSONLine(want []string, d *cpfcData) string {
	seen := map[string]bool{}
	var keys []string
	for _, k := range want {
		if !seen[k] {
			seen[k] = true
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		switch k {
		case "counts":
			inner := []string{
				"\"commands\": " + strconv.Itoa(d.tree),
				"\"dangerous\": " + strconv.Itoa(d.danger),
				"\"generic\": " + strconv.Itoa(d.generic),
				"\"group_ref\": " + strconv.Itoa(d.groupRef),
				"\"spans\": " + strconv.Itoa(d.spans),
				"\"tree_total\": " + strconv.Itoa(d.treeTotal),
			}
			parts = append(parts, "\"counts\": {"+strings.Join(inner, ", ")+"}")
		case "hits":
			var hs []string
			for _, h := range d.hits {
				hs = append(hs, "{\"code\": "+cpfcJSONStr(h.Code)+", \"ref\": "+cpfcJSONStr(h.Ref)+
					", \"rel\": "+cpfcJSONStr(h.Rel)+", \"why\": "+cpfcJSONStr(h.Why)+"}")
			}
			parts = append(parts, "\"hits\": ["+strings.Join(hs, ", ")+"]")
		case "topics":
			var tks []string
			for _, tk := range cpfcSortedKeysInt(d.topics) {
				tks = append(tks, cpfcJSONStr(tk)+": "+strconv.Itoa(d.topics[tk]))
			}
			parts = append(parts, "\"topics\": {"+strings.Join(tks, ", ")+"}")
		}
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

// ── 入口（= py `main()`）· ✗ 不接线 / ✗ 不注册命令 ─────────────────────────────

func checkPublicFaceCommandsGateMain(stdout, stderr io.Writer, argv []string) int {
	if rc, bad := cpfcRefuseUnknownJSONFields(stderr, argv); bad {
		return rc
	}
	a, rc, done := cpfcParseArgs(stdout, stderr, argv)
	if done {
		return rc
	}
	selftestFailed := false
	if !a.selfTest && !a.noSelfTest {
		src, out := cpfcSelfTest()
		fmt.Fprintln(stdout, "── 开跑前自检（合成夹具 · 不碰真目标）──")
		for _, l := range out {
			fmt.Fprintln(stdout, l)
		}
		if src != 0 {
			if !a.readOnlyCount {
				fmt.Fprintln(stdout, "⛔ 自检不过 ⇒ 拒绝跑真目标（门禁自己先能被证明「会红」）")
				return 2
			}
			selftestFailed = true
			fmt.Fprintln(stdout, "⚠ 自检不过 ⇒ --read-only-count：只印只读计数，**不给结论**（rc 仍 = 2）")
		}
	} else if a.selfTest {
		src, out := cpfcSelfTest()
		fmt.Fprintln(stdout, "── 自检（合成夹具 · 不碰真目标）──")
		for _, l := range out {
			fmt.Fprintln(stdout, l)
		}
		return src
	}

	r, lines, data := cpfcJudge(a.root, a.zerg, true)
	fmt.Fprintf(stdout, "门 · 公开面命令名（`T-14` · 判据 `V10` · 漏点 `E-4`）· 树根 %s\n", data.root)
	for _, l := range lines {
		fmt.Fprintln(stdout, l)
	}
	if a.json != "" {
		var keys []string
		for _, k := range strings.Split(a.json, ",") {
			if cpfcPyStrip(k) != "" {
				keys = append(keys, cpfcPyStrip(k))
			}
		}
		fmt.Fprintln(stdout, cpfcJSONLine(keys, data))
	}
	if r == 2 {
		fmt.Fprintln(stdout, "⛔ 缺件/判不了 ⇒ **不给结论（rc=2）**（缺件 ≠ 绿 · 不许当绿）")
	} else if r == 1 {
		fmt.Fprintln(stdout, "✗ 有红（rc=1）：例外须 0 · 逐条见上（处置只有两条：改页，或改命令面的用法串 —— "+
			"不许放宽判据来凑绿 ✗）")
	}
	// ★ B 类真口子修（目录面）：遍历读失败 ⇒ 响亮 BLOCKED rc=2 + 逐处点名（✗ 不再静默少判）。
	//   同族先例 `check_doc_cmds.go` 的 `dcmdReadFails` / `check_bypass_idioms.go` 的 `cbiReadFails`。
	if len(cpfcReadFails) > 0 {
		fmt.Fprintf(stderr, "BLOCKED —— 有 %d 处读不到（B 类读失败 ⇒ 登记 + 判红 · rc=2 · 不给结论）：\n", len(cpfcReadFails))
		for _, u := range cpfcReadFails {
			fmt.Fprintf(stderr, "  [读失败] %s\n", u)
		}
		return 2
	}
	if selftestFailed {
		return 2
	}
	return r
}
