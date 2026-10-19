// check_public_marks.go —— 判据件 `scripts/gates/check-public-marks.py`（898 行）的 **Go 真搬**
// （M2 一枚 · 公开标记族 · 入口 `checkPublicMarksGateMain` · 承接 GAP-20261001-13）。
//
// 验收判据 = 同 argv 跑两态（py 原件 ↔ 本入口），stdout / stderr / 退码 **三面逐字相同**。
//
// 声明 (a) 默认根定位差异：py 从 `__file__` 上溯推 ROOT（`dirname(dirname(HERE))`，HERE=`scripts/gates`）；
// 本件无 `__file__` 语境 ⇒ 只能从 **cwd 上溯** 找「含 `scripts/公开标记.tsv` 的目录」当默认根，
// 找不到则回落到 cwd。⇒ 两态在「脚本位置 == cwd 上溯根」时逐字同构；「脚本位置 ≠ cwd」时路径推导面不同
// （✗ 不声称逐字对齐）—— 对拍甲档显式传 `--root`、乙档令 cwd == 夹具根即可对齐。
//
// 声明 (b) 自指：本判据扫的是 `scripts/<域>/…` 面（含 `scripts/` 顶层 `*.sh|*.py`），本件落
// `core/cmd/zerg/` ⇒ **不落在本判据自己的扫描面内**。四形态逐条核查见交付④；本件正文里凡
// 「被判红的私有面词面」一律 **拆串拼**（`/Volumes/`+`AHZ` · `ms`+`01` · `A72`+`109` 一类），
// 以免本件被 **他门**（`check-hardcoded-private-paths.py` 的 SCAN_DIRS 含 `core/`）扫成新命中；
// 且本件 **零** 文档树悬空字面（一旦被跟踪前瞻消解见交付④）。
//
// 遍历/排序点（逐处交代，见交付①）：
//
//	P1 `face_domains` 盘侧 `os.listdir`（原序）→ `sorted(doms)`：输出 = 排序后域集，**决定 B 档表头域序**。
//	P2 `scan_face` 域递归（`os.walk` 原序 + `dns[:]=sorted` + `fns sorted`）→ 末尾 `sorted(out)`：
//	   **输出行序决定者 = 末尾全局 `sorted(out)`**（遍历序只为控制流逐字同构，不进输出）。
//	P3 `scan_face` 顶层档 `sorted(os.listdir)`：决定顶层件行序。
//	P4 `run` 的 `for rel in sorted(yes)`：决定 B3/B3e 明细件序；`dangling`/`missing` 各按表序/面序。
//	P5 `scan_tree` 产出树递归 → 末尾 `sorted`：决定 D 档明细件序。
//	P6 `scan_encoded` 末尾 `sorted(hits, key=行号)`（稳定）：决定 B3e 行序。
//	⇒ 全程 **零 Go map 迭代进输出**（域集/未登记集一律「map 收集 → 排序进 slice」）。
package main

import (
	"crypto/rand"
	"encoding/base64"
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

const (
	cpmTableRel  = pathMarksTable
	cpmTop       = "顶层"
	cpmProg      = "check-public-marks.py"
	cpmTableCols = "文件\tpublic\t理由"
)

var cpmCols = []string{"文件", "public", "理由"}

// OFF_FACE_DOMAINS：面外域（分域前的工作/实验目录）**不进面**（逐字照 py L97）。
var cpmOffFaceDomains = []string{"exportnames", "sandbox-probes", "plugins"}

var cpmSkipDirs = map[string]bool{
	"__pycache__": true, ".git": true, ".history": true, ".mypy_cache": true,
	".pytest_cache": true, "node_modules": true, "target": true, "dist": true,
	"venv": true, ".venv": true, "build": true,
}

var cpmSkipSuffix = []string{"~", ".bak", ".orig", ".rej", ".swp", ".pyc", ".DS_Store"}

var cpmTreeSkipDirs = map[string]bool{
	".git": true, ".history": true, "__pycache__": true, ".mypy_cache": true,
	".pytest_cache": true, "node_modules": true,
}

// ── py 语义工具（posixpath / 词边界 同构）────────────────────────────────────────────

// cpmPyJoin = CPython posixpath.join 逐段规则（★ 三语义差：中间 `..` 原样留 · 后段绝对丢前段 ·
// 空段保留尾 `/`）。模板见 skill `templates/pyjoin.go`，按件名缩写定名 `cpmPyJoin`。
func cpmPyJoin(parts ...string) string {
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

func cpmPyNormpath(path string) string {
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

func cpmPyAbspath(p string) string {
	if !strings.HasPrefix(p, "/") {
		wd, err := os.Getwd()
		if err != nil {
			wd = ""
		}
		p = cpmPyJoin(wd, p)
	}
	return cpmPyNormpath(p)
}

func cpmSplitComps(p string) []string {
	var out []string
	for _, c := range strings.Split(p, "/") {
		if c != "" {
			out = append(out, c)
		}
	}
	return out
}

func cpmPyRelpath(path, start string) string {
	sl := cpmSplitComps(cpmPyAbspath(start))
	pl := cpmSplitComps(cpmPyAbspath(path))
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
	return cpmPyJoin(rel...)
}

// cpmIsWord = py `\w`（Unicode 词字符：字母 | 数字 | `_`）。
func cpmIsWord(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsNumber(r)
}

// cpmIsSpace = py `\s`（Unicode 空白）。
func cpmIsSpace(r rune) bool { return unicode.IsSpace(r) }

// cpmBadDotDigit = py `(?<![0-9.])` 的「前一字 ∈ [0-9.]」判定。
func cpmBadDotDigit(r rune) bool { return (r >= '0' && r <= '9') || r == '.' }

// cpmBadHostChar = py `[A-Za-z0-9_.-]` 单字符判定（机型 host 形态用）。
func cpmBadHostChar(r rune) bool {
	return (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') ||
		(r >= '0' && r <= '9') || r == '_' || r == '.' || r == '-'
}

// cpmClassChar = `[A-Za-z0-9._\-]` 单字节判定（口令/Bearer 值类用）。
func cpmClassChar(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') ||
		(c >= '0' && c <= '9') || c == '.' || c == '_' || c == '-'
}

func cpmIsDir(p string) bool { // os.path.isdir（跟随符号链接）
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

func cpmIsFile(p string) bool { // os.path.isfile（跟随符号链接）
	fi, err := os.Stat(p)
	return err == nil && fi.Mode().IsRegular()
}

func cpmIsSymlink(p string) bool {
	fi, err := os.Lstat(p)
	return err == nil && fi.Mode()&os.ModeSymlink != 0
}

func cpmHasSuffixAny(s string, suffixes []string) bool {
	for _, suf := range suffixes {
		if strings.HasSuffix(s, suf) {
			return true
		}
	}
	return false
}

// cpmListdir = os.listdir（readdir 原序；调用方自己决定是否排序）。
func cpmListdir(dir string) []string {
	f, err := os.Open(dir)
	if err != nil {
		return nil
	}
	ents, err := f.ReadDir(-1)
	f.Close()
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(ents))
	for _, e := range ents {
		out = append(out, e.Name())
	}
	return out
}

// cpmWalk 复刻 os.walk(topdown=True, followlinks=False) 的产出序：先本层文件（排序后经 emit），
// 再按 `sorted(dns 过滤后)` 逐个下钻（符号链接到目录者只进 dns、不递归）。
func cpmWalk(top string, skip map[string]bool, emit func(dp string, fns []string)) {
	f, err := os.Open(top)
	if err != nil {
		return
	}
	ents, err := f.ReadDir(-1)
	f.Close()
	if err != nil {
		return
	}
	var dirs, nondirs []string
	for _, e := range ents {
		full := cpmPyJoin(top, e.Name())
		if cpmIsDir(full) { // py entry.is_dir() 跟随符号链接
			dirs = append(dirs, e.Name())
		} else {
			nondirs = append(nondirs, e.Name())
		}
	}
	sort.Strings(nondirs)
	emit(top, nondirs)
	sort.Strings(dirs)
	for _, d := range dirs {
		if strings.HasPrefix(d, ".") || skip[d] {
			continue
		}
		full := cpmPyJoin(top, d)
		if cpmIsSymlink(full) { // followlinks=False
			continue
		}
		cpmWalk(full, skip, emit)
	}
}

func cpmRuneSlice(s string, n int) string {
	if n <= 0 {
		return ""
	}
	cnt := 0
	for i := range s {
		if cnt == n {
			return s[:i]
		}
		cnt++
	}
	return s
}

// cpmPyRepr = py `repr(str)`（本判据只用得到简单 ASCII，含引号切换的常规档）。
func cpmPyRepr(s string) string {
	if !strings.Contains(s, "'") {
		return "'" + strings.ReplaceAll(s, "\\", "\\\\") + "'"
	}
	if !strings.Contains(s, "\"") {
		return "\"" + strings.ReplaceAll(s, "\\", "\\\\") + "\""
	}
	return "'" + strings.ReplaceAll(strings.ReplaceAll(s, "\\", "\\\\"), "'", "\\'") + "'"
}

func cpmPyListRepr(items []string) string {
	parts := make([]string, 0, len(items))
	for _, it := range items {
		parts = append(parts, cpmPyRepr(it))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// cpmPyOSError 手写对齐 py 的 OSError 串 `[Errno N] <strerror>: '<path>'`。
func cpmPyOSError(err error, path string) string {
	var eno syscall.Errno
	if errors.As(err, &eno) {
		msg := ""
		switch eno {
		case syscall.ENOENT:
			msg = "No such file or directory"
		case syscall.EACCES:
			msg = "Permission denied"
		case syscall.EISDIR:
			msg = "Is a directory"
		default:
			msg = eno.Error()
		}
		return fmt.Sprintf("[Errno %d] %s: '%s'", int(eno), msg, path)
	}
	return err.Error()
}

// cpmReadText = `open(path, encoding="utf-8", errors="replace").read()`。
func cpmReadText(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.ToValidUTF8(string(b), "\uFFFD"), nil
}

// ── 私有面模式表（逐字照 py PRIVATE_PATTERNS；★ 正则不硬译：无后行断言的 4 条 + 2 条词界条手写）──
type cpmPat struct {
	why  string
	find func(string) [][2]int
}

func cpmReFind(re *regexp.Regexp, startBad, endBad func(rune) bool) func(string) [][2]int {
	return func(s string) [][2]int {
		raw := re.FindAllStringIndex(s, -1)
		spans := make([][2]int, 0, len(raw))
		for _, sp := range raw {
			spans = append(spans, [2]int{sp[0], sp[1]})
		}
		if startBad == nil && endBad == nil {
			return spans
		}
		out := [][2]int{}
		for _, sp := range spans {
			ok := true
			if startBad != nil && sp[0] > 0 {
				r, _ := utf8.DecodeLastRuneInString(s[:sp[0]])
				if startBad(r) {
					ok = false
				}
			}
			if ok && endBad != nil && sp[1] < len(s) {
				r, _ := utf8.DecodeRuneInString(s[sp[1]:])
				if endBad(r) {
					ok = false
				}
			}
			if ok {
				out = append(out, sp)
			}
		}
		return out
	}
}

// cpmScanLits 逐字面量扫描 + 可选词界判定（左/右）；多字面量按起点合并、去重叠（左优先）。
func cpmScanLits(s string, lits []string, startBad, endBad func(rune) bool) [][2]int {
	type sp struct{ a, b int }
	var all []sp
	for _, lit := range lits {
		off := 0
		for {
			i := strings.Index(s[off:], lit)
			if i < 0 {
				break
			}
			st := off + i
			en := st + len(lit)
			ok := true
			if startBad != nil && st > 0 {
				r, _ := utf8.DecodeLastRuneInString(s[:st])
				if startBad(r) {
					ok = false
				}
			}
			if ok && endBad != nil && en < len(s) {
				r, _ := utf8.DecodeRuneInString(s[en:])
				if endBad(r) {
					ok = false
				}
			}
			if ok {
				all = append(all, sp{st, en})
			}
			off = en
		}
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].a < all[j].a })
	out := [][2]int{}
	lastEnd := -1
	for _, x := range all {
		if x.a >= lastEnd {
			out = append(out, [2]int{x.a, x.b})
			lastEnd = x.b
		}
	}
	return out
}

// cpmLitFinder 把字面量扫描包成 `cpmPat.find`。
func cpmLitFinder(lits []string, startBad, endBad func(rune) bool) func(string) [][2]int {
	return func(s string) [][2]int { return cpmScanLits(s, lits, startBad, endBad) }
}

// cpmFindBearer = 手写 `\bBearer\s+[A-Za-z0-9._\-]{20,}`（`\b`/`\s` 按 py Unicode 语义）。
func cpmFindBearer(s string) [][2]int {
	out := [][2]int{}
	lit := "Bearer"
	off := 0
	for {
		i := strings.Index(s[off:], lit)
		if i < 0 {
			break
		}
		st := off + i
		end0 := st + len(lit)
		ok := true
		if st > 0 {
			r, _ := utf8.DecodeLastRuneInString(s[:st])
			if cpmIsWord(r) {
				ok = false
			}
		}
		if !ok {
			off = end0
			continue
		}
		j := end0
		cnt := 0
		for j < len(s) {
			r, sz := utf8.DecodeRuneInString(s[j:])
			if cpmIsSpace(r) {
				j += sz
				cnt++
			} else {
				break
			}
		}
		if cnt == 0 {
			off = end0
			continue
		}
		k := j
		n := 0
		for k < len(s) && cpmClassChar(s[k]) {
			k++
			n++
		}
		if n < 20 {
			off = end0
			continue
		}
		out = append(out, [2]int{st, k})
		off = k
	}
	return out
}

// cpmFindAssigned = 手写 `(?i)\b(password|passwd|secret|api[_-]?key|access[_-]?token)\s*[:=]\s*["']?[A-Za-z0-9._\-]{12,}`。
func cpmFindAssigned(s string) [][2]int {
	out := [][2]int{}
	kws := []string{"password", "passwd", "secret", "api_key", "apikey", "api-key",
		"access_token", "accesstoken", "access-token"}
	i := 0
	for i < len(s) {
		r, sz := utf8.DecodeRuneInString(s[i:])
		if !cpmIsWord(r) {
			i += sz
			continue
		}
		if i > 0 {
			pr, _ := utf8.DecodeLastRuneInString(s[:i])
			if cpmIsWord(pr) {
				i += sz
				continue
			}
		}
		matched := false
		for _, kw := range kws {
			if len(s)-i < len(kw) || !strings.EqualFold(s[i:i+len(kw)], kw) {
				continue
			}
			j := i + len(kw)
			for j < len(s) {
				r2, sz2 := utf8.DecodeRuneInString(s[j:])
				if cpmIsSpace(r2) {
					j += sz2
				} else {
					break
				}
			}
			if j >= len(s) || (s[j] != ':' && s[j] != '=') {
				continue
			}
			j++
			for j < len(s) {
				r2, sz2 := utf8.DecodeRuneInString(s[j:])
				if cpmIsSpace(r2) {
					j += sz2
				} else {
					break
				}
			}
			if j < len(s) && (s[j] == '"' || s[j] == '\'') {
				j++
			}
			k := j
			n := 0
			for k < len(s) && cpmClassChar(s[k]) {
				k++
				n++
			}
			if n >= 12 {
				out = append(out, [2]int{i, k})
				i = k
				matched = true
				break
			}
		}
		if matched {
			continue
		}
		i += sz
	}
	return out
}

var cpmB64Std = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"

// cpmPrivatePatterns 是**可变**的：自检 ⑯ 会把它整体置空做反向控制（同 py 的 global 手法）。
var cpmPrivatePatterns = cpmBuildPatterns()

func cpmBuildPatterns() []cpmPat {
	volLits := []string{"/Volumes/" + "AHZ", "/Volumes/" + "BXC"}
	reUsers := regexp.MustCompile("/Users/[A-Za-z0-9_.-]+/")
	reHome := regexp.MustCompile("/home/[A-Za-z0-9_.-]+/")
	reIP192 := regexp.MustCompile(`192\.168\.\p{Nd}{1,3}\.\p{Nd}{1,3}`)
	reIP10 := regexp.MustCompile(`10\.\p{Nd}{1,3}\.\p{Nd}{1,3}\.\p{Nd}{1,3}`)
	reIP172 := regexp.MustCompile(`172\.(?:1[6-9]|2\p{Nd}|3[01])\.\p{Nd}{1,3}\.\p{Nd}{1,3}`)
	reProj := regexp.MustCompile("千" + "章|边" + "境保镖")
	reGH := regexp.MustCompile("ghp_[A-Za-z0-9]{20,}|gho_[A-Za-z0-9]{20,}|github_pat_[A-Za-z0-9_]{20,}")
	reSk := regexp.MustCompile("sk-[A-Za-z0-9]{20,}")
	reAKIA := regexp.MustCompile("AKIA[0-9A-Z]{16}")
	reSlack := regexp.MustCompile("xox[baprs]-[A-Za-z0-9-]{10,}")
	reKey := regexp.MustCompile("-----BEGIN [A-Z ]*PRIVATE KEY-----")
	return []cpmPat{
		{"私有卷绝对路径", cpmLitFinder(volLits, nil, cpmIsWord)},
		{"用户家目录绝对路径", cpmReFind(reUsers, nil, nil)},
		{"Linux 家目录绝对路径", cpmReFind(reHome, nil, nil)},
		{"内网地址 192.168.x", cpmReFind(reIP192, cpmBadDotDigit, nil)},
		{"内网地址 10.x", cpmReFind(reIP10, cpmBadDotDigit, nil)},
		{"内网地址 172.16–31.x", cpmReFind(reIP172, cpmBadDotDigit, nil)},
		{"账户：Gitee 镜像账号", cpmLitFinder([]string{"A72" + "109"}, cpmIsWord, cpmIsWord)},
		{"机群机型名 Mr2109", cpmLitFinder([]string{"ms" + "01"}, cpmIsWord, cpmIsWord)},
		{"机群机型名 小端机型", cpmLitFinder([]string{"x" + "3"}, cpmBadHostChar, cpmBadHostChar)},
		{"机群机型名 小端机型（大写）", cpmLitFinder([]string{"X" + "3"}, cpmBadHostChar, cpmBadHostChar)},
		{"机群机型名 小端机型-（后接连字符的 host 形态）", cpmLitFinder([]string{"x" + "3-", "X" + "3-"}, cpmBadHostChar, nil)},
		{"机群机型名 miniN", cpmLitFinder([]string{"mini" + "1", "mini" + "2", "mini" + "3"}, cpmIsWord, cpmIsWord)},
		{"私有项目名", cpmReFind(reProj, nil, nil)},
		{"GitHub 令牌", cpmReFind(reGH, nil, nil)},
		{"API 密钥（sk-）", cpmReFind(reSk, nil, nil)},
		{"AWS Access Key", cpmReFind(reAKIA, nil, nil)},
		{"Slack 令牌", cpmReFind(reSlack, nil, nil)},
		{"私钥块", cpmReFind(reKey, nil, nil)},
		{"明文口令/密钥赋值", cpmFindAssigned},
		{"Bearer 令牌", cpmFindBearer},
	}
}

// cpmEncodedMarkOn / cpmTreeJudgeOn 是**可变**的（自检 ㉝/㉓ 的反向控制）。
var (
	cpmEncodedMarkOn = true
	cpmTreeJudgeOn   = true
)

// cpmJSONFields —— 本门机器面字段表（**唯一真源**：`--json <字段>` 的装包与拒收同读这一处）。
// 治缺口 GAP-20260926-50：本门此前无机读面（`--json` 按 Q-176 一律拒收）⇒ 现给机读出口。
// 两档各占一部分键：主判面（A/B/C）13 键 ＋ D 档产出树面；`--json <字段>` 只出所选键，未知 ⇒ rc=2。
// ★ 只加机读出口：判定面 / 人面文案 / 退码一字不动（新旗标走同族先例的可选字段层 · ✗ 不进冻结帮助表）。
var cpmJSONFields = []string{
	"root", "table", "mode", "rc",
	"rows", "public_yes", "public_no", "face", "face_domains",
	"dangling", "missing", "private", "encoded",
	"tree", "no_in", "no_absent", "yes_in", "yes_absent", "unregistered", "self_in",
}

var (
	cpmB64Cand    = regexp.MustCompile("[A-Za-z0-9+/]{12,}={0,2}")
	cpmB64URLCand = regexp.MustCompile("[A-Za-z0-9_\\-]{12,}")
	cpmPctEsc     = regexp.MustCompile("%[0-9A-Fa-f]{2}")
)

// ── 表 ───────────────────────────────────────────────────────────────────────────

type cpmRow struct {
	ln            int
	rel, pub, why string
}

type cpmTableError struct{ msg string }

func (e cpmTableError) Error() string { return e.msg }

func cpmLoadTable(path string) ([]cpmRow, error) {
	if !cpmIsFile(path) {
		return nil, cpmTableError{"标记表不存在：" + path}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		// py 此处 `open().read()` 失败会抛 OSError（未被 TableError 捕获）⇒ 其上溯成 rc=1 回溯。
		// 本件按新口径 fail-closed 退 rc=2（本门本门无 rc=1 回溯面）—— 口径差见交付⑤未覆盖面。
		return nil, cpmTableError{"标记表读不了：" + cpmPyOSError(err, path)}
	}
	text := string(data)
	lines := strings.Split(text, "\n")
	var rows []cpmRow
	dup := map[string]int{}
	for idx, line := range lines {
		i := idx + 1
		if i == 1 {
			header := strings.Split(strings.TrimRight(line, "\r"), "\t")
			if !cpmStringSliceEq(header, cpmCols) {
				return nil, cpmTableError{fmt.Sprintf("表头不符：%s（应为 %s）",
					cpmPyListRepr(header), strings.Join(cpmCols, "\t"))}
			}
			continue
		}
		if strings.TrimSpace(line) == "" {
			continue
		}
		f := strings.Split(strings.TrimRight(line, "\r"), "\t")
		if len(f) != len(cpmCols) {
			return nil, cpmTableError{fmt.Sprintf("第 %d 行字段数 %d ≠ %d（列口径：%s）",
				i, len(f), len(cpmCols), strings.Join(cpmCols, "\t"))}
		}
		rel := strings.TrimSpace(f[0])
		pub := strings.TrimSpace(f[1])
		why := strings.TrimSpace(f[2])
		if rel == "" {
			return nil, cpmTableError{fmt.Sprintf("第 %d 行「文件」列为空", i)}
		}
		bad := strings.HasPrefix(rel, "/") || strings.HasPrefix(rel, "./") || cpmHasDotDotSeg(rel)
		if bad {
			return nil, cpmTableError{fmt.Sprintf("第 %d 行的路径不是仓相对路径（禁用绝对/`./`/`..`）：%s", i, rel)}
		}
		if pub != "yes" && pub != "no" {
			return nil, cpmTableError{fmt.Sprintf("第 %d 行 public 不是 yes|no：%s", i, cpmPyRepr(pub))}
		}
		if why == "" {
			return nil, cpmTableError{fmt.Sprintf("第 %d 行「理由」为空（登记必须说清为什么 —— 写不清理由的标记就是洗白）", i)}
		}
		if prev, ok := dup[rel]; ok {
			return nil, cpmTableError{fmt.Sprintf("同一件重复登记：%s（第 %d 行与第 %d 行）", rel, prev, i)}
		}
		dup[rel] = i
		rows = append(rows, cpmRow{i, rel, pub, why})
	}
	if len(rows) == 0 {
		return nil, cpmTableError{"表零数据行（只有表头）—— 空表不许当绿"}
	}
	return rows, nil
}

func cpmStringSliceEq(a, b []string) bool {
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

func cpmHasDotDotSeg(rel string) bool {
	for _, seg := range strings.Split(rel, "/") {
		if seg == ".." {
			return true
		}
	}
	return false
}

// cpmFaceDomains = 现读派生域面（含 `TOP` 一档）；面外域一律剔除；`hasRoot=false` 时只看表侧。
func cpmFaceDomains(rows []cpmRow, root string, hasRoot bool) []string {
	doms := map[string]bool{}
	for _, r := range rows {
		p := strings.Split(r.rel, "/")
		if len(p) >= 2 && p[0] == "scripts" {
			if len(p) >= 3 {
				doms[p[1]] = true
			} else {
				doms[cpmTop] = true
			}
		}
	}
	if hasRoot {
		sdir := cpmPyJoin(root, "scripts")
		if cpmIsDir(sdir) {
			for _, name := range cpmListdir(sdir) {
				if strings.HasPrefix(name, ".") || cpmSkipDirs[name] || cpmHasSuffixAny(name, cpmSkipSuffix) {
					continue
				}
				if cpmIsDir(cpmPyJoin(sdir, name)) {
					doms[name] = true
				} else if strings.HasSuffix(name, ".sh") || strings.HasSuffix(name, ".py") {
					doms[cpmTop] = true
				}
			}
		}
	}
	for _, d := range cpmOffFaceDomains {
		delete(doms, d)
	}
	out := make([]string, 0, len(doms))
	for d := range doms {
		out = append(out, d)
	}
	sort.Strings(out)
	return out
}

// cpmScanFace = 面里的件（仓相对路径 · 字典序）；0 件 ⇒ 调用方判空转。
func cpmScanFace(root string, face []string) []string {
	out := []string{}
	for _, d := range face {
		if d == cpmTop {
			sdir := cpmPyJoin(root, "scripts")
			if !cpmIsDir(sdir) {
				continue
			}
			names := cpmListdir(sdir)
			sort.Strings(names)
			for _, fn := range names {
				if strings.HasPrefix(fn, ".") || cpmHasSuffixAny(fn, cpmSkipSuffix) ||
					!(strings.HasSuffix(fn, ".sh") || strings.HasSuffix(fn, ".py")) {
					continue
				}
				if cpmIsFile(cpmPyJoin(sdir, fn)) {
					out = append(out, "scripts/"+fn)
				}
			}
			continue
		}
		base := cpmPyJoin(root, "scripts", d)
		if !cpmIsDir(base) {
			continue
		}
		cpmWalk(base, cpmSkipDirs, func(dp string, fns []string) {
			for _, fn := range fns {
				if strings.HasPrefix(fn, ".") || cpmHasSuffixAny(fn, cpmSkipSuffix) {
					continue
				}
				rel := cpmPyRelpath(cpmPyJoin(dp, fn), root)
				out = append(out, strings.ReplaceAll(rel, string(os.PathSeparator), "/"))
			}
		})
	}
	sort.Strings(out)
	return out
}

type cpmHit struct {
	why    string
	ln     int
	sample string
}

// cpmScanPrivate = 逐模式扫整件字节 —— **纯注释行豁免**（口径与兄弟门 `check-hardcoded-private-paths`
// 逐字同源：同件 `hcppIsComment` / `hcppCommentPrefixes`）。
//
// ★ 本枚（缺口 `GAP-20261013-24`）：此处此前**不给注释豁免**，而兄弟门给（它的成对正控逐字
// 「同一路径出现在注释里 ⇒ **不许红**（防假红）」）⇒ 同一处私有面只出现在 Go 注释里时
// 「一门放过、一门判红」。两门口径对齐 = **都跳纯注释行**（注释不编译进二进制、也不进公开面件正文）。
// ✗ 只跳**整行**注释（行尾注释仍算命中）；✗ 不改 B3/B3e 其余判法、✗ 不动任何既有格的计数口径。
func cpmScanPrivate(path string) []cpmHit {
	txt, err := cpmReadText(path)
	if err != nil {
		return []cpmHit{{"读不了文件：" + cpmPyOSError(err, path), 0, ""}}
	}
	var hits []cpmHit
	for _, p := range cpmPrivatePatterns {
		for _, sp := range p.find(txt) {
			ln := strings.Count(txt[:sp[0]], "\n") + 1
			if hcppIsComment(cpmLineAt(txt, sp[0])) {
				continue
			}
			hits = append(hits, cpmHit{p.why, ln, cpmRuneSlice(txt[sp[0]:sp[1]], 60)})
		}
	}
	return hits
}

// cpmLineAt —— 取字节偏移 `off` 所在的**整行**（行首到该行行尾；供注释豁免判据用）。
func cpmLineAt(txt string, off int) string {
	if off < 0 {
		off = 0
	}
	if off > len(txt) {
		off = len(txt)
	}
	start := strings.LastIndex(txt[:off], "\n") + 1
	if end := strings.Index(txt[off:], "\n"); end >= 0 {
		return txt[start : off+end]
	}
	return txt[start:]
}

// cpmPlainMatch = 明文命中真源：解码后一律回 `cpmPrivatePatterns` 判，不另立白名单。
func cpmPlainMatch(text string) (string, string) {
	for _, p := range cpmPrivatePatterns {
		sp := p.find(text)
		if len(sp) > 0 {
			return p.why, cpmRuneSlice(text[sp[0][0]:sp[0][1]], 50)
		}
	}
	return "", ""
}

// cpmDecodeB64 = **严格**解码（非法/填充补不上/字节非 utf-8 ⇒ ok=false）。
func cpmDecodeB64(tok string, url bool) (string, bool) {
	s := tok
	if url {
		s = strings.NewReplacer("-", "+", "_", "/").Replace(s)
	}
	if len(s)%4 == 1 {
		return "", false
	}
	if len(s)%4 != 0 {
		s += strings.Repeat("=", 4-len(s)%4)
	}
	var out []byte
	n := len(s)
	for i := 0; i < n; i += 4 {
		var v [4]int
		pad := 0
		for j := 0; j < 4; j++ {
			c := s[i+j]
			if c == '=' {
				if i+4 != n {
					return "", false
				}
				v[j] = -1
				pad++
				continue
			}
			if pad > 0 {
				return "", false
			}
			idx := strings.IndexByte(cpmB64Std, c)
			if idx < 0 {
				return "", false
			}
			v[j] = idx
		}
		if pad > 2 {
			return "", false
		}
		switch pad {
		case 0:
			x := v[0]<<18 | v[1]<<12 | v[2]<<6 | v[3]
			out = append(out, byte(x>>16), byte(x>>8), byte(x))
		case 1:
			x := v[0]<<18 | v[1]<<12 | v[2]<<6
			out = append(out, byte(x>>16), byte(x>>8))
		default:
			x := v[0]<<18 | v[1]<<12
			out = append(out, byte(x>>16))
		}
	}
	if !utf8.Valid(out) {
		return "", false
	}
	return string(out), true
}

// cpmUnquote = urllib.parse.unquote（`%XX` 逐字节还原后按 utf-8 解码、errors=replace；`+` 不还原）。
func cpmUnquote(s string) string {
	var buf []byte
	i := 0
	for i < len(s) {
		if s[i] == '%' && i+2 < len(s) && cpmIsHex(s[i+1]) && cpmIsHex(s[i+2]) {
			buf = append(buf, byte(cpmHexVal(s[i+1])<<4|cpmHexVal(s[i+2])))
			i += 3
			continue
		}
		buf = append(buf, s[i])
		i++
	}
	return strings.ToValidUTF8(string(buf), "\uFFFD")
}

func cpmIsHex(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'A' && c <= 'F') || (c >= 'a' && c <= 'f')
}

func cpmHexVal(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	default:
		return int(c-'a') + 10
	}
}

// cpmScanEncoded = 编码变形扫描（只对 public=yes 面调用）。
func cpmScanEncoded(path string) []cpmHit {
	txt, err := cpmReadText(path)
	if err != nil {
		return []cpmHit{{"读不了文件：" + cpmPyOSError(err, path), 0, ""}}
	}
	hits := []cpmHit{}
	var urlSpans [][2]int
	for _, m := range cpmB64URLCand.FindAllStringIndex(txt, -1) {
		tok := txt[m[0]:m[1]]
		if !strings.ContainsAny(tok, "-_") {
			continue
		}
		urlSpans = append(urlSpans, [2]int{m[0], m[1]})
		if dec, ok := cpmDecodeB64(tok, true); ok {
			if why, frag := cpmPlainMatch(dec); why != "" {
				hits = append(hits, cpmHit{
					fmt.Sprintf("编码变形 base64url（解开后命中「%s」）", why),
					strings.Count(txt[:m[0]], "\n") + 1,
					fmt.Sprintf("%s… ⇒ %s", cpmRuneSlice(tok, 40), frag)})
			}
		}
	}
	for _, m := range cpmB64Cand.FindAllStringIndex(txt, -1) {
		skip := false
		for _, b := range urlSpans {
			if b[0] <= m[0] && m[1] <= b[1] {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		tok := txt[m[0]:m[1]]
		if dec, ok := cpmDecodeB64(tok, false); ok {
			if why, frag := cpmPlainMatch(dec); why != "" {
				hits = append(hits, cpmHit{
					fmt.Sprintf("编码变形 base64（解开后命中「%s」）", why),
					strings.Count(txt[:m[0]], "\n") + 1,
					fmt.Sprintf("%s… ⇒ %s", cpmRuneSlice(tok, 40), frag)})
			}
		}
	}
	if cpmPctEsc.MatchString(txt) {
		for i, line := range strings.Split(txt, "\n") {
			if !cpmPctEsc.MatchString(line) {
				continue
			}
			dec := cpmUnquote(line)
			if dec == line {
				continue
			}
			why, frag := cpmPlainMatch(dec)
			if why != "" {
				if w2, _ := cpmPlainMatch(line); w2 == "" {
					hits = append(hits, cpmHit{
						fmt.Sprintf("编码变形 URL（百分号转义 · 解开后命中「%s」）", why), i + 1,
						fmt.Sprintf("%s… ⇒ %s", cpmRuneSlice(strings.TrimSpace(line), 40), frag)})
				}
			}
		}
	}
	sort.SliceStable(hits, func(a, b int) bool { return hits[a].ln < hits[b].ln })
	return hits
}

func cpmCtxDump(out io.Writer, title string, items []string, maxExamples int) {
	fmt.Fprintf(out, "\n── %s：%d 处\n", title, len(items))
	for i, x := range items {
		if i >= maxExamples {
			break
		}
		fmt.Fprintf(out, "   %s\n", x)
	}
	if len(items) > maxExamples {
		fmt.Fprintf(out, "   …另 %d 处\n", len(items)-maxExamples)
	}
}

// cpmRun = 判一遍 ⇒ 退出码（0 全绿 · 1 违规 · 2 表不合规/空转）。
func cpmRun(root string, table *string, maxExamples int, out io.Writer, jr *cpmJSONReq) int {
	t := ""
	if table == nil {
		t = cpmPyJoin(root, cpmTableRel)
	} else {
		t = *table
	}
	rows, err := cpmLoadTable(t)
	if err != nil {
		fmt.Fprintf(out, "✗ %s\n", err.Error())
		fmt.Fprint(out, "⇒ 不给结论（rc=2）：表不合规时**不判**真目标 —— 用坏表判出的绿没有意义\n")
		return 2
	}
	faceDoms := cpmFaceDomains(rows, root, root != "")
	if len(faceDoms) == 0 {
		fmt.Fprint(out, "✗ 表里没有一条 `scripts/<域>/…` 行，且盘上 `scripts/` 下无域目录 ⇒ 域面派生为空，拒绝给结论（rc=2）\n")
		return 2
	}
	face := cpmScanFace(root, faceDoms)
	if len(face) == 0 {
		fmt.Fprintf(out, "✗ scripts/<域>/ 面里 0 件（域 = %s · **现读派生**）⇒ 空转，拒绝给结论（rc=2）\n",
			strings.Join(faceDoms, " · "))
		return 2
	}
	byRel := map[string]cpmRow{}
	for _, r := range rows {
		byRel[r.rel] = r
	}
	var yes []string
	var dangling []string
	for _, r := range rows {
		if r.pub == "yes" {
			yes = append(yes, r.rel)
		}
		if !cpmIsFile(cpmPyJoin(root, r.rel)) {
			dangling = append(dangling, r.rel)
		}
	}
	var missing []string
	for _, rel := range face {
		if _, ok := byRel[rel]; !ok {
			missing = append(missing, rel)
		}
	}
	var private, encoded []cpmPrivHit
	sort.Strings(yes)
	for _, rel := range yes {
		p := cpmPyJoin(root, rel)
		for _, h := range cpmScanPrivate(p) {
			private = append(private, cpmPrivHit{rel, h.why, h.sample, h.ln})
		}
		if cpmEncodedMarkOn {
			for _, h := range cpmScanEncoded(p) {
				encoded = append(encoded, cpmPrivHit{rel, h.why, h.sample, h.ln})
			}
		}
	}
	fmt.Fprintf(out, "表：%s（%d 行 · public=yes %d · public=no %d）· 面：scripts/<域>/ %d 件（域 %d 个 · %s）\n",
		t, len(rows), len(yes), len(rows)-len(yes), len(face), len(faceDoms), strings.Join(faceDoms, " · "))

	b1 := []string{}
	for _, r := range dangling {
		b1 = append(b1, fmt.Sprintf("%s（表第 %d 行）", r, byRel[r].ln))
	}
	cpmCtxDump(out, "B1 悬空登记（表里点名、盘上无此件）", b1, maxExamples)
	cpmCtxDump(out, "B2 漏登记（盘上有、表里没有）", missing, maxExamples)
	b3 := []string{}
	for _, h := range private {
		b3 = append(b3, fmt.Sprintf("%s:%d  [%s]  %s", h.rel, h.ln, h.why, h.sample))
	}
	cpmCtxDump(out, "B3 public=yes 面里的私有面", b3, maxExamples)
	b3e := []string{}
	for _, h := range encoded {
		b3e = append(b3e, fmt.Sprintf("%s:%d  [%s]  %s", h.rel, h.ln, h.why, h.sample))
	}
	cpmCtxDump(out, "B3e public=yes 面里的**编码变形**（解码成功且解出命中上面同一张 `PRIVATE_PATTERNS`）", b3e, maxExamples)

	lineOf := map[string]int{}
	for _, r := range rows {
		lineOf[r.rel] = r.ln
	}
	bad := len(dangling) + len(missing) + len(private) + len(encoded)
	fmt.Fprintf(out, "\n合计：悬空 %d · 漏登记 %d · 私有面 %d · 编码变形 %d\n",
		len(dangling), len(missing), len(private), len(encoded))
	if bad != 0 {
		fmt.Fprintf(out, "⇒ 违规 %d 处（rc=1）：修表或修件 —— ① 悬空 ⇒ 改表；② 漏登记 ⇒ 补表（附理由）；"+
			"③ 私有面 ⇒ 把该件标 public=no，或去掉私有面（**不许**放宽本门的模式表来凑绿）；"+
			"④ 编码变形 ⇒ 同上，**不许**给编码形态加豁免白名单来凑绿\n", bad)
		if jr != nil && jr.given {
			fmt.Fprintln(out, cpmJSONMain(root, t, len(rows), len(yes), face, faceDoms, dangling, missing,
				private, encoded, lineOf, 1, jr.val))
		}
		return 1
	}
	fmt.Fprintf(out, "⇒ 全绿（rc=0）：%d 件已登记 · public=yes 面 %d 件未发现私有面 · 编码变形 0 处\n",
		len(rows), len(yes))
	if jr != nil && jr.given {
		fmt.Fprintln(out, cpmJSONMain(root, t, len(rows), len(yes), face, faceDoms, dangling, missing,
			private, encoded, lineOf, 0, jr.val))
	}
	return 0
}

// ── D 档：产出树 ⨯ 标记表（只报告）────────────────────────────────────────────────

func cpmScanTree(tree string) ([]string, []string) {
	allf := []string{}
	scripts := []string{}
	cpmWalk(tree, cpmTreeSkipDirs, func(dp string, fns []string) {
		for _, fn := range fns {
			if strings.HasPrefix(fn, ".") || cpmHasSuffixAny(fn, cpmSkipSuffix) {
				continue
			}
			rel := cpmPyRelpath(cpmPyJoin(dp, fn), tree)
			rel = strings.ReplaceAll(rel, string(os.PathSeparator), "/")
			allf = append(allf, rel)
			if strings.HasPrefix(rel, "scripts/") {
				scripts = append(scripts, rel)
			}
		}
	})
	sort.Strings(allf)
	sort.Strings(scripts)
	return allf, scripts
}

func cpmInDomains(rel string, face []string) bool {
	p := strings.Split(rel, "/")
	if len(p) < 3 || p[0] != "scripts" {
		return false
	}
	for _, d := range face {
		if d == p[1] {
			return true
		}
	}
	return false
}

func cpmRunTree(tree string, table *string, maxExamples int, out io.Writer, defaultRoot string, jr *cpmJSONReq) int {
	t := ""
	if table == nil {
		t = cpmPyJoin(defaultRoot, cpmTableRel)
	} else {
		t = *table
	}
	if tree == "" || !cpmIsDir(tree) {
		shown := tree
		if tree == "" {
			shown = "（没给 --tree）"
		}
		fmt.Fprintf(out, "✗ 产出树不存在或不是目录：%s（缺件 ≠ 绿）\n", shown)
		fmt.Fprint(out, "⇒ 不给结论（rc=2）：判不了产出树时**不判** —— 这类判据在发布档一律按「拦」处理\n")
		return 2
	}
	rows, err := cpmLoadTable(t)
	if err != nil {
		fmt.Fprintf(out, "✗ %s\n", err.Error())
		fmt.Fprint(out, "⇒ 不给结论（rc=2）：表不合规时**不判**产出树 —— 用坏表判出的绿没有意义\n")
		return 2
	}
	treeAll, treeScripts := cpmScanTree(tree)
	face := cpmFaceDomains(rows, "", false)
	if len(treeScripts) == 0 {
		fmt.Fprintf(out, "✗ 产出树 %s 的 scripts/ 面里 0 件 ⇒ 空转，拒绝给结论（rc=2）\n", tree)
		fmt.Fprintf(out, "   （这类树判不了「表 ⨯ scripts/」：整树 %d 件，但一件脚本都没在里面 —— 不要拿它当绿）\n",
			len(treeAll))
		return 2
	}
	inTree := map[string]bool{}
	for _, f := range treeAll {
		inTree[f] = true
	}
	var noIn, noAbsent, yesIn, yesAbsent []cpmRowRef
	for _, r := range rows {
		inside := inTree[r.rel]
		if r.pub == "no" {
			if inside {
				noIn = append(noIn, cpmRowRef{r.ln, r.rel, r.why})
			} else {
				noAbsent = append(noAbsent, cpmRowRef{r.ln, r.rel, r.why})
			}
		} else {
			if inside {
				yesIn = append(yesIn, cpmRowRef{r.ln, r.rel, r.why})
			} else {
				yesAbsent = append(yesAbsent, cpmRowRef{r.ln, r.rel, r.why})
			}
		}
	}
	selfIn := false
	for _, f := range treeScripts {
		if f == cpmTableRel {
			selfIn = true
			break
		}
	}
	registered := map[string]bool{}
	for _, r := range rows {
		registered[r.rel] = true
	}
	unregistered := []string{}
	for _, p := range treeScripts {
		if !registered[p] {
			unregistered = append(unregistered, p)
		}
	}
	sort.Strings(unregistered)
	if !cpmTreeJudgeOn {
		noIn, yesAbsent, selfIn = nil, nil, false
	}
	fmt.Fprintf(out, "产出树：%s · 表：%s（%d 行 · public=yes %d · public=no %d）\n",
		tree, t, len(rows), len(yesIn)+len(yesAbsent), len(noIn)+len(noAbsent))
	fmt.Fprintf(out, "基线（现跑 · 产出树 ⨯ 标记表）：表 no 件进产出树 = %d · 表 no 件缺产出树 = %d · 树内未登记件 = %d\n",
		len(noIn), len(noAbsent), len(unregistered))
	fmt.Fprintf(out, "反向（现跑）：表 yes 件在产出树 = %d · 表 yes 件缺产出树 = %d\n",
		len(yesIn), len(yesAbsent))
	d1 := []string{}
	for _, x := range noIn {
		extra := ""
		if !cpmInDomains(x.rel, face) {
			extra = "  ★面外件（不在派生的域面内 · 设计稿 §六 P6 待拍）"
		}
		d1 = append(d1, fmt.Sprintf("%s（表第 %d 行）→ 产出树 %s%s\n       ← 表判 public=no：%s",
			x.rel, x.ln, x.rel, extra, x.why))
	}
	cpmCtxDump(out, "D1 表说 no、却进了产出树（只报告：本档不改退出码）", d1, maxExamples)
	d2 := []string{}
	for _, x := range yesAbsent {
		d2 = append(d2, fmt.Sprintf("%s（表第 %d 行）← 表判 public=yes：%s", x.rel, x.ln, x.why))
	}
	cpmCtxDump(out, "D2 表说 yes、却不在产出树（反向冲突）", d2, maxExamples)
	d3 := []string{}
	if selfIn {
		d3 = append(d3, fmt.Sprintf("%s → 产出树 %s（表自己不在表内 ⇒ 也是下面 D4 未登记件之一）",
			cpmTableRel, cpmTableRel))
	}
	cpmCtxDump(out, "D3 表自身在产出树里（判据的真源跟着出公开面）", d3, maxExamples)
	cpmCtxDump(out, "D4 树内未登记件（只报数 · **不进命中** —— 处置建议见设计稿 §六 P9：先不扩，只登记为缺口，由人拍）",
		unregistered, maxExamples)
	hits := len(noIn) + len(yesAbsent)
	if selfIn {
		hits++
	}
	fmt.Fprintf(out, "\n合计：D1 %d 处 · D2 %d 处 · D3 %d 处 · D4 未登记 %d 件（只报数）\n",
		len(noIn), len(yesAbsent), cpmBoolToInt(selfIn), len(unregistered))
	if hits != 0 {
		fmt.Fprintf(out, "⇒ 命中 %d 处（rc=1 · **本档只报告**：挂它的那一步申报 `tri-report` ⇒ 落 REPORT，\n"+
			"   不计失败项、不改退出码；逐域收敛到 0 之后再**单独一笔**升 `tri`）\n"+
			"   命中之后只有两条路：① 改表（承认它该发）② 改处置（收进排除清单 / EXCLUDES）\n"+
			"   —— **不许**放宽本判据来凑绿 ✗（照设计稿 §3.3 纪律 3）\n", hits)
		if jr != nil && jr.given {
			fmt.Fprintln(out, cpmJSONTree(defaultRoot, t, tree, len(rows), len(yesIn)+len(yesAbsent),
				len(noIn)+len(noAbsent), face, noIn, noAbsent, yesIn, yesAbsent, unregistered, selfIn, 1, jr.val))
		}
		return 1
	}
	fmt.Fprint(out, "⇒ 全绿（rc=0）：表说 no 的件一件都不在产出树里 · 表说 yes 的件全在 · 表自身不在产出树里\n")
	if jr != nil && jr.given {
		fmt.Fprintln(out, cpmJSONTree(defaultRoot, t, tree, len(rows), len(yesIn)+len(yesAbsent),
			len(noIn)+len(noAbsent), face, noIn, noAbsent, yesIn, yesAbsent, unregistered, selfIn, 0, jr.val))
	}
	return 0
}

func cpmBoolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// ── 自检：成对负控（含 must-fail；合成夹具 · **自回收** TMPDIR 前后 delta=0）──────────────

func cpmWriteFiles(root string, files [][2]string) {
	for _, kv := range files {
		p := cpmPyJoin(root, kv[0])
		_ = os.MkdirAll(cpmDirname(p), 0o755)
		_ = os.WriteFile(p, []byte(kv[1]), 0o644)
	}
}

func cpmDirname(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' {
			return p[:i]
		}
	}
	return ""
}

func cpmFixture(tmp, tableText string, files [][2]string) (string, string) {
	root, err := os.MkdirTemp(tmp, "")
	if err != nil {
		root = cpmPyJoin(tmp, "fx")
		_ = os.MkdirAll(root, 0o755)
	}
	cpmWriteFiles(root, files)
	tpath := cpmPyJoin(root, cpmTableRel)
	_ = os.MkdirAll(cpmDirname(tpath), 0o755)
	_ = os.WriteFile(tpath, []byte(tableText), 0o644)
	return root, tpath
}

func cpmFixtureTree(tmp, tableText string, treeFiles [][2]string) (string, string, string) {
	root, tpath := cpmFixture(tmp, tableText, nil)
	tree, err := os.MkdirTemp(tmp, "")
	if err != nil {
		tree = cpmPyJoin(tmp, "tr")
		_ = os.MkdirAll(tree, 0o755)
	}
	cpmWriteFiles(tree, treeFiles)
	return root, tpath, tree
}

func cpmRunQuiet(root string, table string) (int, string) {
	var buf strings.Builder
	rc := cpmRun(root, &table, 20, &buf, nil)
	return rc, buf.String()
}

func cpmRunTreeQuiet(tree string, table string) (int, string) {
	var buf strings.Builder
	rc := cpmRunTree(tree, &table, 20, &buf, cpmDefaultRoot(), nil)
	return rc, buf.String()
}

// cpmSelftest 逐字照 py selftest（33 条）；夹具在**本函数内自回收**（defer RemoveAll）。
func cpmSelftest(out io.Writer) int {
	ok, total := 0, 0
	caseN := func(name string, wantRC int, wantText []string, mk func() (int, string)) {
		total++
		rc, text := mk()
		good := rc == wantRC
		if good {
			for _, s := range wantText {
				if !strings.Contains(text, s) {
					good = false
					break
				}
			}
		}
		if good {
			ok++
		}
		extra := ""
		if !good {
			var miss []string
			for _, s := range wantText {
				if !strings.Contains(text, s) {
					miss = append(miss, s)
				}
			}
			extra = fmt.Sprintf("；缺字样 %s", cpmPyListRepr(miss))
		}
		fmt.Fprintf(out, "  %s %s（期望 rc=%d / 实测 rc=%d%s）\n",
			map[bool]string{true: "✓", false: "✗"}[good], name, wantRC, rc, extra)
	}

	good := "文件\tpublic\t理由\n" +
		"scripts/build/good.sh\tyes\tA 类：构建/启动链（合成夹具）\n" +
		"scripts/gates/gate.sh\tno\tB 类：门禁（合成夹具）\n"
	dirtyNo := "#!/bin/sh\n# 私有面样例（public=no 面 —— 本门**不判**）\n" +
		"# <worker-ip> /Users/" + "Mr2109/x ghp_" + "AAAAAAAAAAAAAAAAAAAAAAAA" + "\n"

	tmp, err := os.MkdirTemp("", "pubmarks-selftest-")
	if err != nil {
		tmp = os.TempDir()
	}
	// ★★ 自回收：py 的 `mkdtemp(prefix="pubmarks-selftest-")` 从不回收（A 类）⇒ Go 件 defer 清干净。
	defer os.RemoveAll(tmp)

	base := func(extra [][2]string, tableText *string, tableOverride *string) (string, string) {
		files := [][2]string{
			{"scripts/build/good.sh", "#!/bin/sh\necho ok\n"},
			{"scripts/gates/gate.sh", dirtyNo},
		}
		files = append(files, extra...)
		tt := good
		if tableText != nil {
			tt = *tableText
		}
		r, t := cpmFixture(tmp, tt, files)
		if tableOverride != nil {
			t = *tableOverride
		}
		return r, t
	}
	mk := func(root, table string) func() (int, string) {
		return func() (int, string) { return cpmRunQuiet(root, table) }
	}

	// ① 正控：yes 面干净 + no 面带私有面 ⇒ 绿（同时钉「不判 no 面」）
	r1, t1 := base(nil, nil, nil)
	caseN("① 正控：yes 面干净、no 面带私有面 ⇒ 绿（不判 no 面）", 0, []string{"全绿", "public=yes 1"}, mk(r1, t1))

	// ② must-fail：public=yes 面塞内网 IP ⇒ 必红
	r2, t2 := base([][2]string{{"scripts/build/good.sh", "#!/bin/sh\ncurl http://<worker-ip>:8100/api\n"}}, nil, nil)
	caseN("② must-fail：public=yes 面塞内网 IP ⇒ 红且点名模式与行号", 1,
		[]string{"私有面 1", "<worker-ip>", "内网地址 192.168.x", "good.sh:2"}, mk(r2, t2))

	// ③ must-fail：public=yes 面塞私有绝对路径 ⇒ 必红
	r3, t3 := base([][2]string{{"scripts/build/good.sh", "#!/bin/sh\ncd /Volumes/" + "AHZ/00 项目/模型类/Zerg\n"}}, nil, nil)
	caseN("③ must-fail：public=yes 面塞私有卷绝对路径 ⇒ 红", 1,
		[]string{"私有卷绝对路径", "/Volumes/" + "AHZ"}, mk(r3, t3))

	// ④ must-fail：public=yes 面塞账户与机型名 ⇒ 必红两条
	r4, t4 := base([][2]string{{"scripts/build/good.sh",
		"#!/bin/sh\ngit clone https://gitee.com/A72" + "109/zerg-swarm\nssh ms" + "01 'true'\n"}}, nil, nil)
	caseN("④ must-fail：public=yes 面塞账户（A72109）与机型名（Mr2109）⇒ 红两条", 1,
		[]string{"Gitee 镜像账号", "机群机型名 Mr2109"}, mk(r4, t4))

	// ⑤ must-fail：public=yes 面塞令牌 ⇒ 必红
	r5, t5 := base([][2]string{{"scripts/build/good.sh", "#!/bin/sh\nexport T=ghp_" + "AAAAAAAAAAAAAAAAAAAAAAAA" + "\n"}}, nil, nil)
	caseN("⑤ must-fail：public=yes 面塞 GitHub 令牌 ⇒ 红", 1, []string{"GitHub 令牌", "ghp_"}, mk(r5, t5))

	// ⑥ 悬空登记 ⇒ 红
	tab6 := good + "scripts/gates/不存在.sh\tno\tB 类：合成夹具\n"
	r6, t6 := base(nil, &tab6, nil)
	caseN("⑥ 悬空登记（表里点名、盘上无此件）⇒ 红并点名", 1, []string{"悬空登记", "不存在.sh"}, mk(r6, t6))

	// ⑦ 漏登记 ⇒ 红
	r7, t7 := base([][2]string{{"scripts/build/newbie.sh", "#!/bin/sh\ntrue\n"}}, nil, nil)
	caseN("⑦ 漏登记（盘上有、表里没有）⇒ 红并点名", 1, []string{"漏登记", "newbie.sh"}, mk(r7, t7))

	// ⑧ 表头不符 ⇒ rc=2
	tab8 := "文件\tpublic\t理由\t备注\nscripts/build/good.sh\tyes\tA\n"
	r8, t8 := base(nil, &tab8, nil)
	caseN("⑧ 表头不符 ⇒ rc=2（不给结论，不判真目标）", 2, []string{"表头不符"}, mk(r8, t8))

	// ⑨ public 值非法 ⇒ rc=2
	tab9 := "文件\tpublic\t理由\nscripts/build/good.sh\tYES\tA\n"
	r9, t9 := base(nil, &tab9, nil)
	caseN("⑨ public 值非法（YES）⇒ rc=2", 2, []string{"public 不是 yes|no"}, mk(r9, t9))

	// ⑩ 理由空 ⇒ rc=2
	tab10 := "文件\tpublic\t理由\nscripts/build/good.sh\tyes\t\n"
	r10, t10 := base(nil, &tab10, nil)
	caseN("⑩ 理由空 ⇒ rc=2（写不清理由的标记 = 洗白）", 2, []string{"理由」为空"}, mk(r10, t10))

	// ⑪ 重复登记 ⇒ rc=2
	tab11 := good + "scripts/build/good.sh\tno\t重复\n"
	r11, t11 := base(nil, &tab11, nil)
	caseN("⑪ 同一件重复登记 ⇒ rc=2", 2, []string{"重复登记"}, mk(r11, t11))

	// ⑫ 表缺件 ⇒ rc=2
	r12, _ := base(nil, nil, nil)
	missTab := cpmPyJoin(r12, "scripts/没有这个表.tsv")
	caseN("⑫ 表缺件 ⇒ rc=2", 2, []string{"标记表不存在"}, mk(r12, missTab))

	// ⑬ 空表（只有表头）⇒ rc=2
	tab13 := "文件\tpublic\t理由\n"
	r13, t13 := base(nil, &tab13, nil)
	caseN("⑬ 空表（只有表头）⇒ rc=2（空表不许当绿）", 2, []string{"零数据行"}, mk(r13, t13))

	// ⑭ 空转（面里 0 件）⇒ rc=2
	r14, t14 := cpmFixture(tmp, good, [][2]string{{"scripts/calib/占位.txt", "x\n"}})
	_ = os.RemoveAll(cpmPyJoin(r14, "scripts", "calib"))
	caseN("⑭ 面里 0 件 ⇒ rc=2（空转 = 假覆盖，拒绝给结论）", 2, []string{"空转"}, mk(r14, t14))

	// ⑮ 绝对路径写法 ⇒ rc=2
	tab15 := "文件\tpublic\t理由\n/abs/good.sh\tyes\tA\n"
	r15, t15 := base(nil, &tab15, nil)
	caseN("⑮ 表里写绝对路径 ⇒ rc=2", 2, []string{"不是仓相对路径"}, mk(r15, t15))

	// ⑯ 反向控制：关掉私有面扫描 ⇒ ② 那棵树必须退回绿
	savedPats := cpmPrivatePatterns
	cpmPrivatePatterns = nil
	rc16, text16 := cpmRunQuiet(r2, t2)
	cpmPrivatePatterns = savedPats
	total++
	g16 := rc16 == 0 && strings.Contains(text16, "全绿")
	if g16 {
		ok++
	}
	fmt.Fprintf(out, "  %s ⑯ 反向控制：关掉私有面模式表 ⇒ ② 那棵树退回绿（rc=%d）—— 证明 ② 不是恒红也不是恒绿\n",
		map[bool]string{true: "✓", false: "✗"}[g16], rc16)

	// ⑰–㉓：D 档
	goodTree := [][2]string{{"scripts/build/good.sh", "#!/bin/sh\necho ok\n"}}
	dirtyTree := append(append([][2]string{}, goodTree...), [2]string{"scripts/gates/gate.sh", "#!/bin/sh\ntrue\n"})

	_, t17, tr17 := cpmFixtureTree(tmp, good,
		append(append([][2]string{}, goodTree...), [2]string{"scripts/sandbox-probes/x.sb", "x\n"}))
	caseN("⑰ 正控 D：产出树里只有 yes 件（另有一件未登记）⇒ 绿（未登记只报数、不进命中）", 0,
		[]string{"全绿", "表 no 件进产出树 = 0", "树内未登记件 = 1"}, func() (int, string) { return cpmRunTreeQuiet(tr17, t17) })

	_, t18, tr18 := cpmFixtureTree(tmp, good, dirtyTree)
	caseN("⑱ must-fail D1：表说 no 的件进了产出树 ⇒ 红且点名（件:行 + 理由原样）", 1,
		[]string{"表 no 件进产出树 = 1", "scripts/gates/gate.sh（表第 3 行）", "B 类：门禁（合成夹具）"},
		func() (int, string) { return cpmRunTreeQuiet(tr18, t18) })

	_, t19, tr19 := cpmFixtureTree(tmp, good, [][2]string{{"scripts/gates/gate.sh", "#!/bin/sh\ntrue\n"}})
	caseN("⑲ must-fail D2：表说 yes 的件不在产出树 ⇒ 红且点名（反向冲突）", 1,
		[]string{"表 yes 件缺产出树 = 1", "scripts/build/good.sh（表第 2 行）"},
		func() (int, string) { return cpmRunTreeQuiet(tr19, t19) })

	_, t20, tr20 := cpmFixtureTree(tmp, good,
		append(append([][2]string{}, goodTree...), [2]string{"scripts/公开标记.tsv", "文件\tpublic\t理由\n"}))
	caseN("⑳ must-fail D3：表自身在产出树里 ⇒ 单独一条（判据的真源跟着出公开面）", 1,
		[]string{"D3 表自身在产出树里", "scripts/公开标记.tsv → 产出树"},
		func() (int, string) { return cpmRunTreeQuiet(tr20, t20) })

	r21, t21, _ := cpmFixtureTree(tmp, good, goodTree)
	caseN("㉑ must-fail D：产出树不存在 ⇒ rc=2（缺件 ≠ 绿 · 判不了 ≠ 绿）", 2,
		[]string{"缺件 ≠ 绿", "不给结论"},
		func() (int, string) { return cpmRunTreeQuiet(cpmPyJoin(r21, "没有这棵树"), t21) })

	_, t22, tr22 := cpmFixtureTree(tmp, good, [][2]string{{"docs" + "/a.md", "x\n"}})
	caseN("㉒ must-fail D：产出树 scripts/ 面 0 件 ⇒ rc=2（空转 = 假覆盖，拒绝给结论）", 2,
		[]string{"空转"}, func() (int, string) { return cpmRunTreeQuiet(tr22, t22) })

	savedTree := cpmTreeJudgeOn
	cpmTreeJudgeOn = false
	rc23, text23 := cpmRunTreeQuiet(tr18, t18)
	cpmTreeJudgeOn = savedTree
	total++
	g23 := rc23 == 0 && strings.Contains(text23, "全绿")
	if g23 {
		ok++
	}
	fmt.Fprintf(out, "  %s ㉓ 反向控制：关掉 D 档命中判定 ⇒ ⑱ 那棵树退回绿（rc=%d）—— 证明 ⑱ 不是恒红也不是恒绿\n",
		map[bool]string{true: "✓", false: "✗"}[g23], rc23)

	// ㉔–㉘：域面派生档
	fmt.Fprint(out, "[域面派生档] 表 ∪ 盘派生域面（含「顶层」一档）· 面外域不进面\n")
	r24, t24 := base([][2]string{{"scripts/zzz/lone.py", "#!/bin/sh\ntrue\n"}}, nil, nil)
	caseN("㉔ must-fail · 域面派生：盘上新域 `scripts/zzz/`（表里一行都没有）⇒ 进面且点名", 1,
		[]string{"漏登记", "scripts/zzz/lone.py"}, mk(r24, t24))

	tab25 := good + "scripts/zzz/lone.py\tno\tB 类：合成夹具（新域）\n"
	r25, t25 := base([][2]string{{"scripts/zzz/lone.py", "#!/bin/sh\ntrue\n"}}, &tab25, nil)
	caseN("㉕ 成对：同一棵树把该件补进表 ⇒ 回绿（与 ㉔ 成对）", 0, []string{"全绿"}, mk(r25, t25))

	tab26 := good + "scripts/dev/isolate-ro.sh\tno\tB 类·开发工具（合成夹具 · 缺口 GAP-20260926-20）\n"
	r26, t26 := base([][2]string{{"scripts/dev/isolate-ro.sh", "#!/bin/sh\ntrue\n"},
		{"scripts/dev/lone.py", "#!/bin/sh\ntrue\n"}}, &tab26, nil)
	caseN("㉖ must-fail · dev 口径纳入：表里有 dev 行 ⇒ 同域未登记件被点名（旧 9 域常量对 dev 全瞎）", 1,
		[]string{"漏登记", "scripts/dev/lone.py"}, mk(r26, t26))

	r28, t28 := base([][2]string{{"scripts/top.sh", "#!/bin/sh\ntrue\n"}}, nil, nil)
	caseN("㉘ must-fail · 顶层一档：`scripts/top.sh` 未登记 ⇒ 点名（口径同门侧 `scripts/*.sh`）", 1,
		[]string{"漏登记", "scripts/top.sh"}, mk(r28, t28))

	r27, t27 := base([][2]string{{"scripts/sandbox-probes/x.sb", "x\n"}}, nil, nil)
	rc27, text27 := cpmRunQuiet(r27, t27)
	total++
	g27 := rc27 == 0 && strings.Contains(text27, "全绿") && !strings.Contains(text27, "sandbox-probes")
	if g27 {
		ok++
	}
	fmt.Fprintf(out, "  %s ㉗ 反向控制：面外域（`scripts/sandbox-probes/`）有未登记件 ⇒ 放绿且不点名（rc=%d）"+
		"—— 证明派生**没有**把不可比的面拉进来\n", map[bool]string{true: "✓", false: "✗"}[g27], rc27)

	// ㉙–㉝：编码变形档
	fmt.Fprint(out, "[编码变形档] 只收「**能解码**且**解出来命中 `PRIVATE_PATTERNS`**」的串（复用同一张表 · 不另写白名单）\n")
	b64Home := "L1VzZXJzL2Z1enowMS8uY29uZmlnL3plcmcvdG9rZW4="
	r29, t29 := base([][2]string{{"scripts/build/good.sh",
		"#!/bin/sh\nTOKEN_FILE=$(echo " + b64Home + " | base64 -d)\n"}}, nil, nil)
	caseN("㉙ must-fail · 编码变形：public=yes 面塞 base64 的真家目录路径 ⇒ 红且点名（件:行 + 解出片段）", 1,
		[]string{"编码变形 1", "good.sh:2", "编码变形 base64", "用户家目录绝对路径"}, mk(r29, t29))

	b64OK := base64.StdEncoding.EncodeToString([]byte("hello zerg swarm public fixture ok"))
	r30, t30 := base([][2]string{{"scripts/build/good.sh", "#!/bin/sh\necho " + b64OK + "\n"}}, nil, nil)
	rc30, text30 := cpmRunQuiet(r30, t30)
	total++
	g30 := rc30 == 0 && strings.Contains(text30, "全绿") && strings.Contains(text30, "编码变形 0")
	if g30 {
		ok++
	}
	fmt.Fprintf(out, "  %s ㉚ 成对 · 编码变形：无害 base64（解开 = 普通英文、不命中形态表）⇒ 放绿（rc=%d）"+
		"—— 证明这一格**不是**「看到长串就报」\n", map[bool]string{true: "✓", false: "✗"}[g30], rc30)

	junkRaw := make([]byte, 26)
	_, _ = rand.Read(junkRaw[:24])
	junkRaw[24] = 0xff
	junkRaw[25] = 0xfe
	b64Junk := base64.StdEncoding.EncodeToString(junkRaw)
	r31, t31 := base([][2]string{{"scripts/build/good.sh", "#!/bin/sh\necho " + b64Junk + "\n"}}, nil, nil)
	rc31, text31 := cpmRunQuiet(r31, t31)
	total++
	g31 := rc31 == 0 && strings.Contains(text31, "全绿") && strings.Contains(text31, "编码变形 0")
	if g31 {
		ok++
	}
	fmt.Fprintf(out, "  %s ㉛ 成对 · 编码变形：随机长串（解开不是 utf-8 = 乱码）⇒ 放绿（rc=%d）"+
		"—— 证明「解不出 ⇒ 不报」\n", map[bool]string{true: "✓", false: "✗"}[g31], rc31)

	r32, t32 := base([][2]string{{"scripts/build/good.sh",
		"#!/bin/sh\nP=%2FUsers%2FMr2109%2F.config%2Fzerg%2Ftoken\n"}}, nil, nil)
	caseN("㉜ must-fail · 编码变形：public=yes 面塞 URL 百分号转义的真家目录路径 ⇒ 红且点名", 1,
		[]string{"编码变形 1", "good.sh:2", "编码变形 URL", "用户家目录绝对路径"}, mk(r32, t32))

	savedEnc := cpmEncodedMarkOn
	cpmEncodedMarkOn = false
	rc33, text33 := cpmRunQuiet(r29, t29)
	cpmEncodedMarkOn = savedEnc
	total++
	g33 := rc33 == 0 && strings.Contains(text33, "全绿")
	if g33 {
		ok++
	}
	fmt.Fprintf(out, "  %s ㉝ 反向控制：关掉编码变形格 ⇒ ㉙ 那棵树退回绿（rc=%d）—— 证明 ㉙ 不是恒红也不是恒绿\n",
		map[bool]string{true: "✓", false: "✗"}[g33], rc33)

	// ㉞–㊱：机读面（缺口 GAP-20260926-50 · `--json <字段>` 子集选择 ＋ 未知字段 ⇒ rc=2）
	fmt.Fprint(out, "[机读面档] `--json <字段>` 子集选择 · 未知字段 ⇒ rc=2 印可选字段表（Q-176 形态）\n")
	captureE := func(argv []string) (int, string, string) {
		var ob, eb strings.Builder
		rc := checkPublicMarksGateMain(&ob, &eb, argv)
		return rc, ob.String(), eb.String()
	}
	rc34, out34, _ := captureE([]string{"--root", r1, "--table", t1, "--json"})
	total++
	g34 := rc34 == 0 && strings.Contains(out34, "\"mode\": \"judge\"") &&
		strings.Contains(out34, "\"missing\": [") && strings.Contains(out34, "\"rc\": 0")
	if g34 {
		ok++
	}
	fmt.Fprintf(out, "  %s ㉞ 机读面 · 正控：`--json` ⇒ rc=0 ＋ payload（mode=judge · rc=0 · missing 在场）（实测 rc=%d）\n",
		map[bool]string{true: "✓", false: "✗"}[g34], rc34)

	rc35, out35, _ := captureE([]string{"--root", r1, "--table", t1, "--json", "root"})
	total++
	g35 := rc35 == 0 && strings.Contains(out35, "\"root\":") && !strings.Contains(out35, "\"missing\":")
	if g35 {
		ok++
	}
	fmt.Fprintf(out, "  %s ㉟ 机读面 · 字段选择：`--json root` ⇒ 只出 root 键（真收字段）（实测 rc=%d）\n",
		map[bool]string{true: "✓", false: "✗"}[g35], rc35)

	rc36, _, err36 := captureE([]string{"--root", r1, "--table", t1, "--json", "nope,root"})
	total++
	g36 := rc36 == 2 && strings.Contains(err36, "不认的字段名")
	if g36 {
		ok++
	}
	fmt.Fprintf(out, "  %s ㊱ 机读面 · must-fail：`--json nope,root` ⇒ rc=2 ＋「不认的字段名」（实测 rc=%d）\n",
		map[bool]string{true: "✓", false: "✗"}[g36], rc36)

	fmt.Fprintf(out, "  自证用例：%d 条 · 通过 %d 条（断言用例条数 %d，少跑即失效）\n", total, ok, 36)
	if ok != total || total != 36 {
		return 1
	}
	return 0
}

// ── 缺口 GAP-20260926-50（原 Q-176 拒收面）：给本门机读出口 `--json <字段>` ──────────────────
//
// 口径逐字照同族正解 `check_stale_frozen.go` 的 `csfRefuseUnknownJSONFields`：
//   · `--json`（无值 / 后随 `-`）⇒ 出全量 payload，不拒；
//   · `--json <字段表>`（逗号分隔）⇒ 只出所选键（真收字段）；
//   · 含**不认的名** ⇒ rc=2 ＋ 印可选字段表（**字段名打错 ≠ 零命中** —— 两态在机器面上必须分得开）。
// ★ 只加机读出口：本门判定面 / 人面文案 / 退码一字不动（新旗标 ✗ 不进冻结帮助表）。

// cpmJSONReq —— `--json` 的调用意图（given=false ⇒ 人面档，不出 payload）。
type cpmJSONReq struct {
	given bool
	val   string
}

// cpmJSONKV —— 机器面一格（k=键 · v=已序列化好的 JSON 字面）。
type cpmJSONKV struct{ k, v string }

// cpmJSONStr —— JSON 字符串字面（ensure_ascii=False 的转义面）。
func cpmJSONStr(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, c := range s {
		switch c {
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
			if c < 0x20 {
				b.WriteString(fmt.Sprintf("\\u%04x", c))
			} else {
				b.WriteRune(c)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

func cpmJSONInt(n int) string { return strconv.Itoa(n) }

func cpmJSONBool(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func cpmJSONStrArr(xs []string) string {
	parts := make([]string, len(xs))
	for i, x := range xs {
		parts[i] = cpmJSONStr(x)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// cpmJSONAssemble —— 全量键序 = all 的给定序（= cpmJSONFields 里该档在场的键序）；`fields` 非空
// ⇒ 只出所选键（按用户给的字段序）；未知名已由 cpmRefuseUnknownJSONFields 先拦（此处防御性跳过）。
func cpmJSONAssemble(all []cpmJSONKV, fields string) string {
	byKey := map[string]string{}
	for _, kv := range all {
		byKey[kv.k] = kv.v
	}
	sel := all
	if fields != "" {
		sel = nil
		for _, f := range strings.Split(fields, ",") {
			k := strings.TrimSpace(f)
			if k == "" {
				continue
			}
			if v, ok := byKey[k]; ok {
				sel = append(sel, cpmJSONKV{k, v})
			}
		}
	}
	var b strings.Builder
	b.WriteString("{")
	for i, kv := range sel {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString("\"" + kv.k + "\": " + kv.v)
	}
	b.WriteString("}")
	return b.String()
}

// cpmPrivHit —— public=yes 面命中的一格（私有面 / 编码变形共用）。
type cpmPrivHit struct {
	rel, why, sample string
	ln               int
}

// cpmJSONPrivHits —— 命中列表装包（结构化，✗ 不是人面那行字符串 ⇒ 机器可直接抠）。
func cpmJSONPrivHits(hs []cpmPrivHit) string {
	out := make([]string, 0, len(hs))
	for _, h := range hs {
		out = append(out, "{\"rel\": "+cpmJSONStr(h.rel)+", \"line\": "+cpmJSONInt(h.ln)+
			", \"why\": "+cpmJSONStr(h.why)+", \"sample\": "+cpmJSONStr(h.sample)+"}")
	}
	return "[" + strings.Join(out, ", ") + "]"
}

// cpmJSONDangling —— B1 悬空登记装包（rel ＋ 表行号）。
func cpmJSONDangling(rels []string, lineOf map[string]int) string {
	out := make([]string, 0, len(rels))
	for _, r := range rels {
		out = append(out, "{\"rel\": "+cpmJSONStr(r)+", \"table_line\": "+cpmJSONInt(lineOf[r])+"}")
	}
	return "[" + strings.Join(out, ", ") + "]"
}

// cpmRowRef —— D 档行引用（rel ＋ 表行号 ＋ 理由）。
type cpmRowRef struct {
	ln       int
	rel, why string
}

// cpmJSONRowRefs —— D 档行引用装包；`withOff` ⇒ 附「面外件」标记（D1 档用）。
func cpmJSONRowRefs(rows []cpmRowRef, withOff bool, face []string) string {
	out := make([]string, 0, len(rows))
	for _, x := range rows {
		obj := "{\"rel\": " + cpmJSONStr(x.rel) + ", \"table_line\": " + cpmJSONInt(x.ln) + ", \"why\": " + cpmJSONStr(x.why)
		if withOff {
			obj += ", \"off_domain\": " + cpmJSONBool(!cpmInDomains(x.rel, face))
		}
		obj += "}"
		out = append(out, obj)
	}
	return "[" + strings.Join(out, ", ") + "]"
}

// cpmJSONMain —— 主判面 payload（键序 = cpmJSONFields 前 13 键）。
func cpmJSONMain(root, table string, nRows, nYes int, face []string, faceDoms, dangling, missing []string,
	private, encoded []cpmPrivHit, lineOf map[string]int, rc int, fields string) string {
	all := []cpmJSONKV{
		{"root", cpmJSONStr(root)},
		{"table", cpmJSONStr(table)},
		{"mode", cpmJSONStr("judge")},
		{"rc", cpmJSONInt(rc)},
		{"rows", cpmJSONInt(nRows)},
		{"public_yes", cpmJSONInt(nYes)},
		{"public_no", cpmJSONInt(nRows - nYes)},
		{"face", cpmJSONInt(len(face))},
		{"face_domains", cpmJSONStrArr(faceDoms)},
		{"dangling", cpmJSONDangling(dangling, lineOf)},
		{"missing", cpmJSONStrArr(missing)},
		{"private", cpmJSONPrivHits(private)},
		{"encoded", cpmJSONPrivHits(encoded)},
	}
	return cpmJSONAssemble(all, fields)
}

// cpmJSONTree —— D 档产出树面 payload（键序 = 公共 4 键 ＋ cpmJSONFields 后段 7 键）。
func cpmJSONTree(root, table, tree string, nRows, nYes, nNo int, face []string,
	noIn, noAbsent, yesIn, yesAbsent []cpmRowRef, unregistered []string, selfIn bool, rc int, fields string) string {
	all := []cpmJSONKV{
		{"root", cpmJSONStr(root)},
		{"table", cpmJSONStr(table)},
		{"mode", cpmJSONStr("judge-tree")},
		{"rc", cpmJSONInt(rc)},
		{"tree", cpmJSONStr(tree)},
		{"rows", cpmJSONInt(nRows)},
		{"public_yes", cpmJSONInt(nYes)},
		{"public_no", cpmJSONInt(nNo)},
		{"no_in", cpmJSONRowRefs(noIn, true, face)},
		{"no_absent", cpmJSONRowRefs(noAbsent, false, nil)},
		{"yes_in", cpmJSONRowRefs(yesIn, false, nil)},
		{"yes_absent", cpmJSONRowRefs(yesAbsent, false, nil)},
		{"unregistered", cpmJSONStrArr(unregistered)},
		{"self_in", cpmJSONBool(selfIn)},
	}
	return cpmJSONAssemble(all, fields)
}

// cpmRefuseUnknownJSONFields —— `--json <字段>` 里有不认的名 ⇒ rc=2 ＋ 印可选字段表（Q-176 口径）。
// 与同族判据件同形：字段名打错 ≠ 零命中。★ 先于 argparse 判；`--json`（无值 / 后随 `-`）⇒ 收全量，不拒。
func cpmRefuseUnknownJSONFields(stderr io.Writer, argv []string) *int {
	var want []string
	for i, x := range argv {
		if x == "--json" {
			v := ""
			if i+1 < len(argv) && !strings.HasPrefix(argv[i+1], "-") {
				v = argv[i+1]
			}
			want = append(want, strings.Split(v, ",")...)
		} else if strings.HasPrefix(x, "--json=") {
			want = append(want, strings.Split(strings.SplitN(x, "=", 2)[1], ",")...)
		}
	}
	var fields []string
	for _, f := range want {
		if fs := strings.TrimSpace(f); fs != "" {
			fields = append(fields, fs)
		}
	}
	known := map[string]bool{}
	for _, f := range cpmJSONFields {
		known[f] = true
	}
	var bad []string
	for _, f := range fields {
		if !known[f] {
			bad = append(bad, f)
		}
	}
	if len(bad) == 0 {
		return nil
	}
	quoted := func(xs []string) string {
		out := make([]string, 0, len(xs))
		for _, x := range xs {
			out = append(out, "`"+x+"`")
		}
		return strings.Join(out, " · ")
	}
	fmt.Fprintf(stderr, "⛔ `--json` 不认的字段名：%s\n", quoted(bad))
	fmt.Fprintf(stderr, "   可选字段表：%s\n", quoted(cpmJSONFields))
	fmt.Fprint(stderr, "⇒ 用法错（rc=2）：**字段名打错 ≠ 零命中** —— 两态在机器面上必须分得开\n")
	rc := 2
	return &rc
}

// ── 默认根推导（声明 (a)：无 `__file__` ⇒ 从 cwd 上溯找含标记表的目录）────────────────────

func cpmDefaultRoot() string {
	wd, err := os.Getwd()
	if err != nil || wd == "" {
		return wd
	}
	d := wd
	for {
		if cpmIsFile(cpmPyJoin(d, cpmTableRel)) {
			return d
		}
		parent := cpmDirname(d)
		if parent == "" || parent == d {
			return wd
		}
		d = parent
	}
}

const cpmUsage = "usage: check-public-marks.py [-h] [--root ROOT] [--table TABLE] [--tree TREE]\n" +
	"                             [--max-examples MAX_EXAMPLES] [--self-test]\n"

const cpmHelpBody = "\noptional arguments:\n" +
	"  -h, --help            show this help message and exit\n" +
	"  --root ROOT           树根（默认 = 仓根）\n" +
	"  --table TABLE         标记表路径（默认 = <root>/scripts/公开标记.tsv）\n" +
	"  --tree TREE           **D 档**：产出树根 ⇒ 判「产出树 ⨯ 标记表」（只报告 · 命中 rc=1）\n" +
	"  --max-examples MAX_EXAMPLES\n" +
	"  --self-test, --selftest\n" +
	"                        只跑成对负控自检（合成夹具，不碰真目标）\n"

// checkPublicMarksGateMain = 本判据的 Go 入口（argv 不含程序名；stdout/stderr 由调用方给）。
func checkPublicMarksGateMain(stdout, stderr io.Writer, argv []string) int {
	if rc := cpmRefuseUnknownJSONFields(stderr, argv); rc != nil {
		return *rc
	}
	root := cpmDefaultRoot()
	var table, tree *string
	maxExamples := 20
	selftest := false
	jsonGiven := false
	jsonVal := ""
	i := 0
	errf := func(msg string) int {
		fmt.Fprint(stderr, cpmUsage)
		fmt.Fprintf(stderr, "%s: error: %s\n", cpmProg, msg)
		return 2
	}
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
			fmt.Fprint(stdout, cpmUsage)
			fmt.Fprint(stdout, cpmHelpBody)
			return 0
		case a == "--root" || strings.HasPrefix(a, "--root="):
			v, okv := val()
			if !okv {
				return errf("argument --root: expected one argument")
			}
			root = v
		case a == "--table" || strings.HasPrefix(a, "--table="):
			v, okv := val()
			if !okv {
				return errf("argument --table: expected one argument")
			}
			table = &v
		case a == "--tree" || strings.HasPrefix(a, "--tree="):
			v, okv := val()
			if !okv {
				return errf("argument --tree: expected one argument")
			}
			tree = &v
		case a == "--max-examples" || strings.HasPrefix(a, "--max-examples="):
			v, okv := val()
			if !okv {
				return errf("argument --max-examples: expected one argument")
			}
			n, cerr := strconv.Atoi(v)
			if cerr != nil {
				return errf(fmt.Sprintf("argument --max-examples: invalid int value: %s", cpmPyRepr(v)))
			}
			maxExamples = n
		case a == "--json" || strings.HasPrefix(a, "--json="):
			jsonGiven = true
			if eq := strings.IndexByte(a, '='); eq >= 0 {
				jsonVal = a[eq+1:]
			} else if i+1 < len(argv) && !strings.HasPrefix(argv[i+1], "-") {
				i++
				jsonVal = argv[i]
			} else {
				jsonVal = ""
			}
		case a == "--self-test" || a == "--selftest":
			selftest = true
		default:
			return errf("unrecognized arguments: " + a)
		}
		i++
	}
	if selftest {
		fmt.Fprint(stdout, "── check-public-marks.py 自证（含 must-fail 负控 · 合成夹具）\n")
		return cpmSelftest(stdout)
	}
	var jr *cpmJSONReq
	if jsonGiven {
		jr = &cpmJSONReq{given: true, val: jsonVal}
	}
	if tree != nil {
		fmt.Fprint(stdout, "── check-public-marks.py D 档（产出树 ⨯ 标记表 · **只报告**：命中 rc=1 不改退出码）\n")
		return cpmRunTree(*tree, table, maxExamples, stdout, root, jr)
	}
	return cpmRun(root, table, maxExamples, stdout, jr)
}
