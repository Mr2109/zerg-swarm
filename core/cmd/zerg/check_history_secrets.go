// check_history_secrets.go —— 门禁件 scripts/gates/check-history-secrets.py 的 **Go 真搬**（M2）。
//
// 本件 = 全历史敏感审计（B0 / G1）：逐提交扫描「真会被公开的文件集」（发布白名单内、非排除、非二进制），
// 逐 blob 套发布替换规则后再扫敏感模式，并扫提交信息本身；命中逐条打码打印，rc=1。
//
// 入口签名（与 py 的 main() 对齐）：
//
//	func checkHistorySecretsGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv —— 与 py sys.argv[1:] 同形（不含程序名）
//	  · 退码 —— 0 通过 / 1 有阻断级命中 / 2 判不了
//
// ★ 标准声明行 (a) —— **默认根定位差异**：py `REPO = dirname^3(abspath(__file__))`（脚本位置）；
// Go 无 `__file__` 语境 ⇒ 从 **cwd** 起上溯找带 `publish/replace-rules.tsv` 的目录当根。
// 同真仓 + cwd=仓根时两态读数逐字相同；「脚本位置 ≠ cwd」时不同（✗ 不声称逐字对齐）。
//
// ★ 标准声明行 (b) —— **自指检查**：本门扫描面 = 发布白名单（含 `core/`）⇒ 本件一旦被提交即入扫描面。
// ⇒ 正文**刻意不留**任何 BLOCK 命中形态：命中词/夹具字面一律**拼接构造**（见 chsecBlockSpecs、chsecFix*）。
// 量化依据：本门 11 条 BLOCK 正则对**本件源文本**现扫命中 0（测试件 NoSelfLeak 钉住）。
//
// 未覆盖面（✗ 不声称逐字对齐）：① argparse 帮助正文（`-h`，复刻 usage/options，非实现级同源）·
// ② 默认根定位（见 a）· ③ 非 UTF-8 文本 decode 粒度（py 按非法序列 / Go 按非法字节 run）·
// ④ 正则语义差（py `\s`/`\d`/`\w` Unicode ⟷ Go RE2 ASCII；RE2 无反向引用/环视，本件未用到）·
// ⑤ 并行档实现手段（py 起子进程 ⟷ Go 同进程 goroutine；结论面与 stdout 逐字一致）·
// ⑥ C 类兜底（py 裸 sys.exit ⟷ 本件 fail-closed rc=2）· ⑦ blob 遍历序走 git 输出序（两态同源）。
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ── 常量与真源（与 py 逐字同源）────────────────────────────────────────────────────────────

const (
	chsecProg     = "check-history-secrets.py"
	chsecIsoPrefx = "!夹具"
	chsecMaxBlob  = 2 * 1024 * 1024
	chsecWantCase = 14 // 自证用例条数（写死：少跑一格即断言失败）
)

type chsecPatSpec struct{ pat, why string }

// chsecBlockSpecs：阻断级（命中即不应公开）。★ 家目录项**拼接构造**（源码里无连成一串的私径）。
var chsecBlockSpecs = []chsecPatSpec{
	{`/Volumes/(AHZ|BXC)`, "私有卷绝对路径"},
	{`/Users/` + "ms" + "01", "用户家目录路径"},
	{`ghp_[A-Za-z0-9]{20,}|gho_[A-Za-z0-9]{20,}|github_pat_[A-Za-z0-9_]{20,}`, "GitHub 令牌"},
	{`sk-[A-Za-z0-9]{20,}`, "API 密钥（sk-）"},
	{`AKIA[0-9A-Z]{16}`, "AWS Access Key"},
	{`xox[baprs]-[A-Za-z0-9-]{10,}`, "Slack 令牌"},
	{"-----BEGIN " + `[A-Z ]*PRIVATE KEY-----`, "私钥块"},
	{`(?i)\b(password|passwd|secret|api[_-]?key|access[_-]?token)\s*[:=]\s*["']?[A-Za-z0-9._\-]{12,}`, "疑似明文口令/密钥赋值"},
	{`\bBearer\s+[A-Za-z0-9._\-]{20,}`, "Bearer 令牌"},
	{`192\.168\.\d{1,3}\.\d{1,3}`, "内网地址 192.168.x"},
	{`\b10\.0\.0\.\d{1,3}`, "内网地址 10.0.0.x"},
}

// chsecInfoSpecs：提示级（会被替换规则过滤掉，但仍值得知道）。★ 私有项目名/人物名**拼接构造**。
var chsecInfoSpecs = []chsecPatSpec{
	{`(?i)\.zerg/token|auth\.json`, "提到私有凭据文件路径（路径引用，非密钥值）"},
	{"千" + "章|边境" + "保镖", "私有项目名（替换规则会过滤）"},
	{`(?i)Mr` + "2109", "人物名（替换规则会过滤）"},
}

var chsecBinarySuffix = []string{
	".png", ".jpg", ".jpeg", ".gif", ".pdf", ".zip", ".gz", ".tgz", ".bundle",
	".dylib", ".so", ".a", ".o", ".ttf", ".ttc", ".otf", ".woff", ".woff2",
	".mp3", ".mp4", ".mov", ".webp", ".ico", ".icns", ".lock",
}

var chsecNoiseSuffix = []string{".bak", ".orig", ".rej"}
var chsecNoiseDirs = []string{"__pycache__"}

// ── 路径/字符串原语（自写同构）──────────────────────────────────────────────────────────────

// chsecPyJoin：CPython posixpath.join 逐段规则（✗ 不用 filepath.Join）。
func chsecPyJoin(parts ...string) string {
	if len(parts) == 0 {
		return ""
	}
	path := parts[0]
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

// chsecPyNormpath：CPython posixpath.normpath（保留恰两个前导斜杠）。
func chsecPyNormpath(p string) string {
	if p == "" {
		return "."
	}
	initialSlashes := 0
	for initialSlashes < len(p) && p[initialSlashes] == '/' {
		initialSlashes++
	}
	if initialSlashes > 2 {
		initialSlashes = 2
	}
	comps := []string{}
	for _, c := range strings.Split(p, "/") {
		if c == "" || c == "." {
			continue
		}
		if c == ".." {
			if len(comps) > 0 && comps[len(comps)-1] != ".." {
				comps = comps[:len(comps)-1]
			} else if initialSlashes == 0 {
				comps = append(comps, "..")
			}
			continue
		}
		comps = append(comps, c)
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

// chsecPyAbspath：CPython os.path.abspath（词法 join(cwd,p) + normpath；✗ 不做 realpath）。
func chsecPyAbspath(p string) string {
	if !strings.HasPrefix(p, "/") {
		cwd, err := os.Getwd()
		if err != nil {
			cwd = "."
		}
		p = chsecPyJoin(cwd, p)
	}
	return chsecPyNormpath(p)
}

// chsecPyDirname：CPython posixpath.dirname（= split[0]）。
func chsecPyDirname(p string) string {
	i := strings.LastIndex(p, "/")
	if i < 0 {
		return ""
	}
	if i == 0 {
		return "/"
	}
	return p[:i]
}

// chsecPySpace：py str.isspace()（含 \x1c-\x1f）⟷ Go unicode.IsSpace 不含 \x1c-\x1f。
func chsecPySpace(r rune) bool {
	if r >= 0x1c && r <= 0x1f {
		return true
	}
	return unicode.IsSpace(r)
}

// chsecPyStrip：py str.strip() 同构（✗ 不用 strings.TrimSpace）。
func chsecPyStrip(s string) string { return strings.TrimFunc(s, chsecPySpace) }

// chsecPyStripChars：py str.strip(chars)。
func chsecPyStripChars(s, chars string) string {
	return strings.TrimFunc(s, func(r rune) bool { return strings.ContainsRune(chars, r) })
}

// chsecTruncRunes：py 切片 s[:n] 按**码点**。
func chsecTruncRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

func chsecRuneLen(s string) int { return utf8.RuneCountInString(s) }

// chsecSplitWS：py str.split(None, maxsplit) 同构。
func chsecSplitWS(s string, maxsplit int) []string {
	var out []string
	r := []rune(s)
	i, n := 0, len(r)
	for {
		for i < n && chsecPySpace(r[i]) {
			i++
		}
		if i >= n {
			break
		}
		if maxsplit >= 0 && len(out) == maxsplit {
			out = append(out, string(r[i:]))
			break
		}
		j := i
		for j < n && !chsecPySpace(r[j]) {
			j++
		}
		out = append(out, string(r[i:j]))
		i = j
	}
	return out
}

// chsecFnMatch：py fnmatch.fnmatch 同构（`*` **跨 `/`** ⟷ Go path.Match 不跨）。
func chsecFnMatch(name, pat string) bool {
	var b strings.Builder
	b.WriteString("^")
	r := []rune(pat)
	for i := 0; i < len(r); i++ {
		c := r[i]
		switch c {
		case '*':
			b.WriteString(".*")
		case '?':
			b.WriteString(".")
		case '[':
			j := i + 1
			if j < len(r) && (r[j] == '!' || r[j] == '^') {
				j++
			}
			if j < len(r) && r[j] == ']' {
				j++
			}
			for j < len(r) && r[j] != ']' {
				j++
			}
			if j >= len(r) {
				b.WriteString(regexp.QuoteMeta(string(c)))
			} else {
				inner := string(r[i+1 : j])
				if strings.HasPrefix(inner, "!") {
					inner = "^" + inner[1:]
				}
				b.WriteString("[" + inner + "]")
				i = j
			}
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	b.WriteString("$")
	rx, err := regexp.Compile(b.String())
	if err != nil {
		return false
	}
	return rx.MatchString(name)
}

func chsecIsDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

func chsecIsFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// chsecRepoRoot：默认根（Go 无 __file__ ⇒ 从 cwd 上溯找带 publish/replace-rules.tsv 的目录）。
func chsecRepoRoot() string {
	d, err := os.Getwd()
	if err != nil {
		return "."
	}
	for i := 0; i < 8; i++ {
		if chsecIsFile(chsecPyJoin(d, "publish", "replace-rules.tsv")) {
			return d
		}
		parent := chsecPyDirname(d)
		if parent == "" || parent == d {
			break
		}
		d = parent
	}
	if cwd, err := os.Getwd(); err == nil {
		return cwd
	}
	return "."
}

// chsecRulesPath / chsecPublishSh：外部真源真身（真仓内相对落点）。
func chsecRulesPath(root string) string {
	return chsecPyJoin(root, "publish", "replace-rules.tsv")
}
func chsecPublishShPath(root string) string {
	return chsecPyJoin(root, "scripts", "build", "publish-public.sh")
}
func chsecWhitelistPath(root string) string {
	return chsecPyJoin(root, "publish", "whitelist.txt")
}

// ── 子进程（git 面）──────────────────────────────────────────────────────────────────────

// chsecShText：py `sh(..., binary=False)` —— 取 stdout、**忽略退码**、按 utf-8 宽松解码。
func chsecShText(args []string, cwd string) string {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = cwd
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = io.Discard
	_ = cmd.Run()
	return strings.ToValidUTF8(out.String(), "\uFFFD")
}

// chsecShBytes：py `sh(..., binary=True)` —— 取原始 stdout 字节。
func chsecShBytes(args []string, cwd string) []byte {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = cwd
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = io.Discard
	_ = cmd.Run()
	return out.Bytes()
}

// ── 读面：白名单 / 排除项 / 替换规则 ───────────────────────────────────────────────────────

func chsecLoadWhitelist(path string) []string {
	entries := []string{}
	b, err := os.ReadFile(path)
	if err != nil {
		return entries
	}
	for _, raw := range strings.Split(string(b), "\n") {
		line := chsecPyStrip(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		entries = append(entries, line)
	}
	return entries
}

var chsecExcludesRe = regexp.MustCompile(`(?s)EXCLUDES=\((.*?)\n\)`)

// chsecLoadExcludes：从 publish-public.sh 的 EXCLUDES=( … ) 数组解析（单一真源）。
func chsecLoadExcludes(path string) []string {
	ex := []string{}
	b, err := os.ReadFile(path)
	if err != nil {
		return ex
	}
	m := chsecExcludesRe.FindStringSubmatch(string(b))
	if m == nil {
		return ex
	}
	for _, raw := range strings.Split(m[1], "\n") {
		line := chsecPyStrip(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = chsecPyStrip(strings.SplitN(line, "#", 2)[0])
		line = chsecPyStripChars(line, `"`)
		line = chsecPyStripChars(line, `'`)
		if line != "" {
			ex = append(ex, line)
		}
	}
	return ex
}

func chsecExcluded(rel string, ex []string) bool {
	for _, s := range chsecNoiseSuffix {
		if strings.HasSuffix(rel, s) {
			return true
		}
	}
	for _, part := range strings.Split(rel, "/") {
		for _, d := range chsecNoiseDirs {
			if part == d {
				return true
			}
		}
	}
	for _, e := range ex {
		if rel == e || strings.HasPrefix(rel, strings.TrimRight(e, "/")+"/") {
			return true
		}
	}
	return false
}

// chsecIsIsolated：py is_isolated（fnmatch，`*` 跨 `/`）。
func chsecIsIsolated(rel string, isolated []string) bool {
	if rel == "" {
		return false
	}
	for _, pat := range isolated {
		if chsecFnMatch(rel, pat) {
			return true
		}
	}
	return false
}

// chsecPublishedPath：py published_path（目录以 `/` 结尾 = 递归）。
func chsecPublishedPath(rel string, entries []string) bool {
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

type chsecLit struct{ lit, rep string }
type chsecRx struct {
	rx  *regexp.Regexp
	rep string
}

// chsecFixU：把 py 正则里的 `\uXXXX` 归一为 RE2 认的 `\x{XXXX}`。
var chsecURe = regexp.MustCompile(`\\u([0-9a-fA-F]{4})`)

func chsecFixU(pat string) string { return chsecURe.ReplaceAllString(pat, `\x{$1}`) }

// chsecLoadRules：发布替换规则（字面 / regex: / !夹具 隔离 glob）。
func chsecLoadRules(path string) ([]chsecLit, []chsecRx, []string) {
	lits := []chsecLit{}
	rxlist := []chsecRx{}
	isolated := []string{}
	b, err := os.ReadFile(path)
	if err != nil {
		return lits, rxlist, isolated
	}
	for _, raw := range strings.Split(string(b), "\n") {
		line := strings.TrimSuffix(raw, "\n")
		if chsecPyStrip(line) == "" || strings.HasPrefix(strings.TrimLeftFunc(line, unicode.IsSpace), "#") {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 2 {
			continue
		}
		pat, rep := parts[0], parts[1]
		if pat == chsecIsoPrefx {
			isolated = append(isolated, chsecPyStrip(rep))
		} else if strings.HasPrefix(pat, "regex:") {
			rx, err := regexp.Compile(chsecFixU(pat[6:]))
			if err != nil {
				continue
			}
			rxlist = append(rxlist, chsecRx{rx, rep})
		} else {
			lits = append(lits, chsecLit{pat, rep})
		}
	}
	return lits, rxlist, isolated
}

// chsecApplyRules：套替换规则；`rel` 命中夹具隔离 ⇒ 原样返回（rel=="" 不适用隔离）。
func chsecApplyRules(text string, lits []chsecLit, regexes []chsecRx, isolated []string, rel string) string {
	if rel != "" && chsecIsIsolated(rel, isolated) {
		return text
	}
	for _, l := range lits {
		if strings.Contains(text, l.lit) {
			text = strings.ReplaceAll(text, l.lit, l.rep)
		}
	}
	for _, r := range regexes {
		text = r.rx.ReplaceAllString(text, r.rep)
	}
	return text
}

// chsecMask：py mask（先 strip；码点 >8 则取前 6 码点 + … + （N 字符））。
func chsecMask(s string) string {
	s = chsecPyStrip(s)
	if chsecRuneLen(s) > 8 {
		return chsecTruncRunes(s, 6) + "…" + fmt.Sprintf("（%d 字符）", chsecRuneLen(s))
	}
	return s
}

// ── 扫描核（串行档与并行片共用同一份判据）────────────────────────────────────────────────

type chsecCompiled struct {
	rx  *regexp.Regexp
	why string
}

type chsecVerdict struct {
	kind   string // clean / block / info / skip
	why    string
	sample string
}

type chsecCount struct {
	idx   int
	bytes int
}

type chsecHit struct {
	Commit string `json:"commit"`
	Path   string `json:"path"`
	Why    string `json:"why"`
	Match  string `json:"match"`
}

type chsecInfoHit struct {
	Commit string `json:"commit"`
	Path   string `json:"path"`
	Why    string `json:"why"`
}

func chsecKey(osha string, iso bool) string {
	if iso {
		return osha + "|True"
	}
	return osha + "|False"
}

// 防空绿档要换的真源（默认走真实现）。
var (
	chsecPublishedPathFn = chsecPublishedPath
	chsecLoadRulesFn     = chsecLoadRules
)

func chsecMin(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func chsecSHA10(sha string) string {
	if len(sha) > 10 {
		return sha[:10]
	}
	return sha
}

func chsecSHA12(s string) string {
	if len(s) > 12 {
		return s[:12]
	}
	return s
}

func chsecHasSuffixFold(s string, suffixes []string) bool {
	ls := strings.ToLower(s)
	for _, suf := range suffixes {
		if strings.HasSuffix(ls, suf) {
			return true
		}
	}
	return false
}

func chsecCountedBytes(counted map[string]chsecCount) int {
	s := 0
	for _, v := range counted {
		s += v.bytes
	}
	return s
}

// chsecScanCommits：按 revs 顺序扫描一段提交（**串行档与并行片共用同一份判据**）。
func chsecScanCommits(revs []string, repo string, lits []chsecLit, regexes []chsecRx, isolated []string,
	entries, exList []string, blk, inf []chsecCompiled, baseIdx int, emitProgress bool,
	cache map[string]chsecVerdict, counted map[string]chsecCount, stdout io.Writer) ([]chsecHit, []chsecInfoHit, map[string]chsecCount) {

	hits := []chsecHit{}
	infoHits := []chsecInfoHit{}
	if cache == nil {
		cache = map[string]chsecVerdict{}
	}
	if counted == nil {
		counted = map[string]chsecCount{}
	}
	total := len(revs)
	for local, sha := range revs {
		idx := baseIdx + local + 1
		// ① 提交信息本身也会被镜像 ⇒ 一起审
		msg := chsecApplyRules(chsecShText([]string{"git", "log", "-1", "--format=%B", sha}, repo), lits, regexes, nil, "")
		for _, c := range blk {
			if m := c.rx.FindString(msg); m != "" {
				hits = append(hits, chsecHit{chsecSHA10(sha), "<commit-message>", c.why, chsecMask(m)})
			}
		}
		// ② 该提交里「会被公开」的文件（ls-tree 带 blob sha ⇒ 同一内容只扫一次）
		listing := chsecShText([]string{"git", "-c", "core.quotepath=false", "ls-tree", "-r", sha}, repo)
		for _, line := range strings.Split(listing, "\n") {
			parts := chsecSplitWS(line, 3)
			if len(parts) != 4 {
				continue
			}
			otype, osha, rel := parts[1], parts[2], parts[3]
			if otype != "blob" || !chsecPublishedPathFn(rel, entries) || chsecExcluded(rel, exList) {
				continue
			}
			if chsecHasSuffixFold(rel, chsecBinarySuffix) {
				continue
			}
			iso := chsecIsIsolated(rel, isolated)
			key := chsecKey(osha, iso)
			verdict, ok := cache[key]
			if !ok {
				blob := chsecShBytes([]string{"git", "cat-file", "blob", osha}, repo)
				nul := -1
				if len(blob) > 0 {
					nul = bytes.IndexByte(blob[:chsecMin(8192, len(blob))], 0)
				}
				if len(blob) > chsecMaxBlob || nul >= 0 {
					cache[key] = chsecVerdict{kind: "skip"}
					continue
				}
				if _, seen := counted[key]; !seen {
					counted[key] = chsecCount{idx, len(blob)}
				}
				if !utf8.Valid(blob) {
					cache[key] = chsecVerdict{kind: "skip"}
					continue
				}
				text := chsecApplyRules(string(blob), lits, regexes, isolated, rel)
				verdict = chsecVerdict{kind: "clean"}
				blocked := false
				for _, c := range blk {
					if m := c.rx.FindString(text); m != "" {
						verdict = chsecVerdict{kind: "block", why: c.why, sample: chsecMask(m)}
						blocked = true
						break
					}
				}
				if !blocked {
					for _, c := range inf {
						if c.rx.FindString(text) != "" {
							verdict = chsecVerdict{kind: "info", why: c.why}
							break
						}
					}
				}
				cache[key] = verdict
			}
			if verdict.kind == "block" {
				hits = append(hits, chsecHit{chsecSHA10(sha), rel, verdict.why, verdict.sample})
			} else if verdict.kind == "info" {
				infoHits = append(infoHits, chsecInfoHit{chsecSHA10(sha), rel, verdict.why})
			}
		}
		if emitProgress && idx%200 == 0 {
			fmt.Fprintf(stdout, "  …已审 %d/%d 笔提交（文件 %d，%.1f MB）\n",
				idx, total, len(counted), float64(chsecCountedBytes(counted))/1048576)
		}
	}
	return hits, infoHits, counted
}

// ── 并行档：提交区间分片（同进程 goroutine · stdout 与串行逐字一致）────────────────────────────

func chsecCPUCount() int {
	n := runtime.NumCPU()
	if n < 1 {
		return 4
	}
	return n
}

// chsecResolveJobs：并发数闭集：0/缺省 = 自动 min(8,max(1,核数//2)) · 1..32 直给 · 其余 ⇒ -1。
func chsecResolveJobs(v int) int {
	if v == 0 {
		j := chsecCPUCount() / 2
		if j < 1 {
			j = 1
		}
		if j > 8 {
			j = 8
		}
		return j
	}
	if v >= 1 && v <= 32 {
		return v
	}
	return -1
}

// chsecSplitBounds：把 total 笔切成 n 段连续区间（前 r 段各多一笔）。
func chsecSplitBounds(total, n int) [][2]int {
	q, r := total/n, total%n
	out := make([][2]int, 0, n)
	s := 0
	for i := 0; i < n; i++ {
		e := s + q
		if i < r {
			e++
		}
		out = append(out, [2]int{s, e})
		s = e
	}
	return out
}

func chsecCumulativeAt(merged map[string]chsecCount, idx int) (int, int) {
	f, b := 0, 0
	for _, v := range merged {
		if v.idx <= idx {
			f++
			b += v.bytes
		}
	}
	return f, b
}

// chsecRunParallel：按提交区间并发跑分片、按最小序号归并 ⇒ (hits, info, merged, n, ok)。
func chsecRunParallel(revs []string, repo string, jobs int, lits []chsecLit, regexes []chsecRx, isolated []string,
	entries, exList []string, blk, inf []chsecCompiled, stderr io.Writer) ([]chsecHit, []chsecInfoHit, map[string]chsecCount, int, bool) {

	n := jobs
	if len(revs) < n {
		n = len(revs)
	}
	bounds := chsecSplitBounds(len(revs), n)
	for i, b := range bounds {
		if b[1] <= b[0] {
			fmt.Fprintf(stderr, "✗ 分片 %d 为空片 ⇒ 判不了(2)（不许静默少审）\n", i)
			return nil, nil, nil, 0, false
		}
	}
	type shardOut struct {
		hits   []chsecHit
		info   []chsecInfoHit
		counts map[string]chsecCount
	}
	outs := make([]shardOut, n)
	done := make(chan int, n)
	for i, b := range bounds {
		go func(i, a, e int) {
			h, ih, c := chsecScanCommits(revs[a:e], repo, lits, regexes, isolated, entries, exList, blk, inf,
				a, false, map[string]chsecVerdict{}, map[string]chsecCount{}, io.Discard)
			outs[i] = shardOut{h, ih, c}
			done <- i
		}(i, b[0], b[1])
	}
	for k := 0; k < n; k++ {
		<-done
	}
	fmt.Fprintf(stderr, "并行档：%d 片（提交区间连续切分 · 结果面按片序拼接 + 首见序号归并 ⇒ 与串行逐字一致）\n", n)
	hits := []chsecHit{}
	info := []chsecInfoHit{}
	merged := map[string]chsecCount{}
	for _, o := range outs {
		hits = append(hits, o.hits...)
		info = append(info, o.info...)
		for key, v := range o.counts {
			cur, ok := merged[key]
			if !ok || v.idx < cur.idx {
				merged[key] = v
			}
		}
	}
	return hits, info, merged, n, true
}

// ── 参数解析（argparse 面复刻）────────────────────────────────────────────────────────────

type chsecArgs struct {
	repo          string
	rng           string
	jsonOut       string
	maxHits       int
	jobs          int
	shardSpec     string
	shardOut      string
	shardTotal    int
	shardTotalSet bool
	shardFirst    string
	shardFirstSet bool
	selftest      bool
	help          bool
}

const chsecUsage = "usage: " + chsecProg + " [-h] [--repo REPO] [--range RNG]\n" +
	"                                [--json JSON_OUT] [--max-hits MAX_HITS]\n" +
	"                                [--jobs JOBS] [--self-test]\n"

const chsecHelpBody = chsecUsage + "\n" +
	"optional arguments:\n" +
	"  -h, --help            show this help message and exit\n" +
	"  --repo REPO           仓库路径（默认本仓；自测/B1 复用时可指向别处）\n" +
	"  --range RNG           提交区间，如 A..B；默认 --all\n" +
	"  --json JSON_OUT\n" +
	"  --max-hits MAX_HITS\n" +
	"  --jobs JOBS           并行分片数（缺省 1=串行·2..32 直给·0=自动 min(8, 核数//2)）\n" +
	"  --self-test, --selftest\n" +
	"                        只跑成对负控自检（合成夹具 + 现读真仓结构 · 不碰真目标 · 不跑全历史）\n"

// chsecParseArgs：复刻 py argparse（未知旗标/缺参/非法 int ⇒ 用法错 rc=2）。
func chsecParseArgs(argv []string, stderr io.Writer) (*chsecArgs, bool) {
	a := &chsecArgs{repo: chsecRepoRoot(), maxHits: 200, jobs: 1}
	unrecognized := []string{}
	needVal := ""
	fail := func(msg string) {
		fmt.Fprint(stderr, chsecUsage)
		fmt.Fprintf(stderr, "%s: error: %s\n", chsecProg, msg)
	}
	i := 0
	for i < len(argv) {
		tok := argv[i]
		if needVal != "" {
			if err := a.setOpt(needVal, tok); err != nil {
				fail(err.Error())
				return nil, false
			}
			needVal = ""
			i++
			continue
		}
		if strings.HasPrefix(tok, "--") {
			name, val, hasVal := tok, "", false
			if eq := strings.Index(tok, "="); eq >= 0 {
				name, val, hasVal = tok[:eq], tok[eq+1:], true
			}
			canon, takesVal, ok := chsecCanonFlag(name)
			if !ok {
				unrecognized = append(unrecognized, tok)
				i++
				continue
			}
			if takesVal {
				if hasVal {
					if err := a.setOpt(canon, val); err != nil {
						fail(err.Error())
						return nil, false
					}
				} else {
					needVal = canon
				}
			} else {
				if hasVal {
					fail(fmt.Sprintf("argument %s: ignored explicit argument %q", name, val))
					return nil, false
				}
				a.setFlag(canon)
			}
			i++
			continue
		}
		if tok == "-h" {
			a.help = true
			i++
			continue
		}
		unrecognized = append(unrecognized, tok)
		i++
	}
	if needVal != "" {
		fail(fmt.Sprintf("argument %s: expected one argument", needVal))
		return nil, false
	}
	if len(unrecognized) > 0 {
		fail("unrecognized arguments: " + strings.Join(unrecognized, " "))
		return nil, false
	}
	return a, true
}

func (a *chsecArgs) setFlag(canon string) {
	switch canon {
	case "--self-test":
		a.selftest = true
	case "--help":
		a.help = true
	}
}

func (a *chsecArgs) setOpt(canon, val string) error {
	switch canon {
	case "--repo":
		a.repo = val
	case "--range":
		a.rng = val
	case "--json":
		a.jsonOut = val
	case "--max-hits":
		n, err := strconv.Atoi(val)
		if err != nil {
			return fmt.Errorf("argument --max-hits: invalid int value: '%s'", val)
		}
		a.maxHits = n
	case "--jobs":
		n, err := strconv.Atoi(val)
		if err != nil {
			return fmt.Errorf("argument --jobs: invalid int value: '%s'", val)
		}
		a.jobs = n
	case "--_shard":
		a.shardSpec = val
	case "--_shard-out":
		a.shardOut = val
	case "--_shard-total":
		n, err := strconv.Atoi(val)
		if err != nil {
			return fmt.Errorf("argument --_shard-total: invalid int value: '%s'", val)
		}
		a.shardTotal = n
		a.shardTotalSet = true
	case "--_shard-first":
		a.shardFirst = val
		a.shardFirstSet = true
	}
	return nil
}

// chsecCanonFlag：长旗标 ⇒ (正名, 是否带值, 是否认得)；支持 argparse 的**无歧义前缀**。
func chsecCanonFlag(name string) (string, bool, bool) {
	type ent struct {
		name     string
		takesVal bool
	}
	table := []ent{
		{"--repo", true}, {"--range", true}, {"--json", true}, {"--max-hits", true},
		{"--jobs", true}, {"--_shard", true}, {"--_shard-out", true}, {"--_shard-total", true},
		{"--_shard-first", true}, {"--self-test", false}, {"--selftest", false}, {"--help", false},
	}
	canon := func(s string) string {
		if s == "--selftest" {
			return "--self-test"
		}
		return s
	}
	for _, t := range table {
		if t.name == name {
			return canon(t.name), t.takesVal, true
		}
	}
	var matches []ent
	for _, t := range table {
		if strings.HasPrefix(t.name, name) {
			matches = append(matches, t)
		}
	}
	if len(matches) == 0 {
		return "", false, false
	}
	uniq := map[string]bool{}
	for _, m := range matches {
		uniq[canon(m.name)] = true
	}
	if len(uniq) == 1 {
		return canon(matches[0].name), matches[0].takesVal, true
	}
	return "", false, false
}

// ── JSON 产出 ──────────────────────────────────────────────────────────────────────────

// chsecJSONEncoder：等价 py json.dump(..., ensure_ascii=False, indent=2)（不转义 < > &）。
func chsecJSONEncoder(w io.Writer) *json.Encoder {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc
}

func chsecEncodeNoTrailNL(v any, w io.Writer) error {
	var buf bytes.Buffer
	if err := chsecJSONEncoder(&buf).Encode(v); err != nil {
		return err
	}
	_, err := w.Write(bytes.TrimSuffix(buf.Bytes(), []byte("\n")))
	return err
}

// ── 入口 ────────────────────────────────────────────────────────────────────────────────

func checkHistorySecretsGateMain(stdout, stderr io.Writer, argv []string) (rc int) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(stderr, "✗ 内部错误：%v ⇒ 判不了(2)\n", r)
			rc = 2
		}
	}()

	a, ok := chsecParseArgs(argv, stderr)
	if !ok {
		return 2
	}
	if a.help {
		fmt.Fprint(stdout, chsecHelpBody)
		return 0
	}

	if a.selftest {
		fmt.Fprint(stdout, "── "+chsecProg+" 自证（成对正负控 + 防空绿档 · 仓外夹具 · 真退出码）\n")
		return chsecSelfTest(stdout, stderr)
	}

	if a.shardSpec != "" {
		return chsecShardWorker(a, stdout, stderr)
	}

	root := chsecRepoRoot()
	repo := chsecPyAbspath(a.repo)
	if !chsecIsDir(chsecPyJoin(repo, ".git")) {
		fmt.Fprintf(stdout, "✗ 不是 git 仓库：%s\n", repo)
		return 2
	}

	jobs := chsecResolveJobs(a.jobs)
	if jobs < 0 {
		fmt.Fprintf(stdout, "✗ --jobs 只认 1..32（0/缺省 = 自动 min(8, max(1, 核数//2))）：%v ⇒ 判不了(2)\n", a.jobs)
		return 2
	}

	lits, regexes, isolated := chsecLoadRulesFn(chsecRulesPath(root))
	exList := chsecLoadExcludes(chsecPublishShPath(root))
	fmt.Fprintf(stdout, "排除项: %d 条（从 publish-public.sh 的 EXCLUDES 解析）+ 噪声后缀 %s\n", len(exList), strings.Join(chsecNoiseSuffix, "/"))
	fmt.Fprintf(stdout, "替换规则: %d 条字面 + %d 条正则（扫描前先套用——审计的是『真会公开的形状』）\n", len(lits), len(regexes))
	isoSuffix := ""
	if len(isolated) > 0 {
		isoSuffix = "：" + strings.Join(isolated, " · ")
	}
	fmt.Fprintf(stdout, "夹具隔离: %d 条 glob（命中件整段跳过替换 · §五 第三批）%s\n", len(isolated), isoSuffix)

	wl := chsecWhitelistPath(repo)
	if !chsecIsFile(wl) {
		wl = chsecWhitelistPath(root)
	}
	entries := chsecLoadWhitelist(wl)

	var revs []string
	scope := ""
	if a.rng != "" {
		revs = chsecFields(chsecShText([]string{"git", "rev-list", a.rng}, repo))
		scope = a.rng
	} else {
		revs = chsecFields(chsecShText([]string{"git", "rev-list", "--all"}, repo))
		scope = "--all"
	}
	fmt.Fprintf(stdout, "审计范围: %s（%d 笔提交）\n", scope, len(revs))
	fmt.Fprintf(stdout, "白名单条目: %d（只审『真会被公开』的那些路径）\n", len(entries))

	blk := chsecCompileSpecs(chsecBlockSpecs)
	inf := chsecCompileSpecs(chsecInfoSpecs)

	var hits []chsecHit
	var infoHits []chsecInfoHit
	var scannedFiles, scannedBytes int

	if jobs > 1 && len(revs) > 1 {
		h, ih, merged, _, ok2 := chsecRunParallel(revs, repo, jobs, lits, regexes, isolated, entries, exList, blk, inf, stderr)
		if !ok2 {
			return 2
		}
		hits, infoHits = h, ih
		for idx := 200; idx <= len(revs); idx += 200 {
			f, b := chsecCumulativeAt(merged, idx)
			fmt.Fprintf(stdout, "  …已审 %d/%d 笔提交（文件 %d，%.1f MB）\n", idx, len(revs), f, float64(b)/1048576)
		}
		scannedFiles = len(merged)
		scannedBytes = chsecCountedBytes(merged)
	} else {
		blobCache := map[string]chsecVerdict{}
		counted := map[string]chsecCount{}
		hits, infoHits, counted = chsecScanCommits(revs, repo, lits, regexes, isolated, entries, exList, blk, inf,
			0, true, blobCache, counted, stdout)
		scannedFiles = len(counted)
		scannedBytes = chsecCountedBytes(counted)
	}

	fmt.Fprintf(stdout, "\n扫描完成：提交 %d 笔 · 文件版本 %d 个 · %.1f MB 文本\n", len(revs), scannedFiles, float64(scannedBytes)/1048576)

	if len(infoHits) > 0 {
		fmt.Fprintf(stdout, "\n· 提示级（%d 处，替换规则会过滤，不影响公开）:\n", len(infoHits))
		for i, h := range infoHits {
			if i >= 10 {
				break
			}
			fmt.Fprintf(stdout, "    %s  %-52s %s\n", h.Commit, chsecTruncRunes(h.Path, 52), h.Why)
		}
	}
	if len(hits) > 0 {
		type kp struct{ path, why string }
		order := []kp{}
		idx := map[kp]int{}
		counts := []int{}
		firsts := []string{}
		samples := []string{}
		for _, h := range hits {
			k := kp{h.Path, h.Why}
			if j, ok3 := idx[k]; ok3 {
				counts[j]++
			} else {
				idx[k] = len(order)
				order = append(order, k)
				counts = append(counts, 1)
				firsts = append(firsts, h.Commit)
				samples = append(samples, h.Match)
			}
		}
		perm := make([]int, len(order))
		for i := range perm {
			perm[i] = i
		}
		sort.SliceStable(perm, func(a, b int) bool { return counts[perm[a]] > counts[perm[b]] })
		fmt.Fprintf(stdout, "\n✗ 阻断级命中 %d 处，涉及 %d 个文件（按文件归并）：\n", len(hits), len(order))
		for n, j := range perm {
			if n >= 30 {
				break
			}
			fmt.Fprintf(stdout, "    %-56s %-20s ×%-4d 首见 %s  %s\n",
				chsecTruncRunes(order[j].path, 56), order[j].why, counts[j], firsts[j], samples[j])
		}
		fmt.Fprintf(stdout, "\n  逐条明细（前 %d）:\n", chsecMin(a.maxHits, len(hits)))
		for i, h := range hits {
			if i >= a.maxHits {
				break
			}
			fmt.Fprintf(stdout, "    %s  %-52s %-22s %s\n", h.Commit, chsecTruncRunes(h.Path, 52), h.Why, h.Match)
		}
		if len(hits) > a.maxHits {
			fmt.Fprintf(stdout, "    …（其余 %d 处省略）\n", len(hits)-a.maxHits)
		}
	} else {
		fmt.Fprint(stdout, "\n结果: 通过——全历史未发现阻断级敏感内容 ✓\n")
	}

	if a.jsonOut != "" {
		if hits == nil {
			hits = []chsecHit{}
		}
		if infoHits == nil {
			infoHits = []chsecInfoHit{}
		}
		report := struct {
			Scope     string         `json:"scope"`
			Commits   int            `json:"commits"`
			Files     int            `json:"files"`
			Bytes     int            `json:"bytes"`
			BlockHits []chsecHit     `json:"block_hits"`
			InfoHits  []chsecInfoHit `json:"info_hits"`
		}{scope, len(revs), scannedFiles, scannedBytes, hits, infoHits}
		fh, err := os.Create(a.jsonOut)
		if err != nil {
			fmt.Fprintf(stderr, "✗ 写 JSON 失败：%v ⇒ 判不了(2)\n", err)
			return 2
		}
		err = chsecEncodeNoTrailNL(report, fh)
		_ = fh.Close()
		if err != nil {
			fmt.Fprintf(stderr, "✗ 写 JSON 失败：%v ⇒ 判不了(2)\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "  JSON 报告: %s\n", a.jsonOut)
	}
	if len(hits) > 0 {
		return 1
	}
	return 0
}

func chsecFields(s string) []string {
	f := strings.Fields(s)
	if f == nil {
		return []string{}
	}
	return f
}

func chsecCompileSpecs(specs []chsecPatSpec) []chsecCompiled {
	out := make([]chsecCompiled, 0, len(specs))
	for _, s := range specs {
		out = append(out, chsecCompiled{regexp.MustCompile(chsecFixU(s.pat)), s.why})
	}
	return out
}

// chsecShardWorker：内部档（`--_shard` 分片工作进程；不自己印报告）。
func chsecShardWorker(a *chsecArgs, stdout, stderr io.Writer) int {
	root := chsecRepoRoot()
	repo := chsecPyAbspath(a.repo)
	if !chsecIsDir(chsecPyJoin(repo, ".git")) {
		fmt.Fprintf(stderr, "✗ 不是 git 仓库：%s\n", repo)
		return 2
	}
	parts := strings.SplitN(a.shardSpec, "/", 2)
	k, n := -1, -1
	if len(parts) == 2 {
		k, _ = strconv.Atoi(parts[0])
		n, _ = strconv.Atoi(parts[1])
	}
	if len(parts) != 2 || n <= 0 {
		fmt.Fprintf(stderr, "✗ 分片参数读不出：%q\n", a.shardSpec)
		return 2
	}
	lits, regexes, isolated := chsecLoadRulesFn(chsecRulesPath(root))
	exList := chsecLoadExcludes(chsecPublishShPath(root))
	wl := chsecWhitelistPath(repo)
	if !chsecIsFile(wl) {
		wl = chsecWhitelistPath(root)
	}
	entries := chsecLoadWhitelist(wl)
	var revs []string
	if a.rng != "" {
		revs = chsecFields(chsecShText([]string{"git", "rev-list", a.rng}, repo))
	} else {
		revs = chsecFields(chsecShText([]string{"git", "rev-list", "--all"}, repo))
	}
	if a.shardTotalSet && len(revs) != a.shardTotal {
		fmt.Fprintf(stderr, "✗ 分片对齐失败：父见 %d 笔 · 本片现读 %d 笔 ⇒ 判不了(2)\n", a.shardTotal, len(revs))
		return 2
	}
	bounds := chsecSplitBounds(len(revs), n)
	if k < 0 || k >= len(bounds) {
		fmt.Fprintf(stderr, "✗ 分片参数读不出：%q\n", a.shardSpec)
		return 2
	}
	lo, hi := bounds[k][0], bounds[k][1]
	if a.shardFirstSet && hi > lo && revs[lo] != a.shardFirst {
		fmt.Fprintf(stderr, "✗ 分片对齐失败：本片首笔 %s ≠ 父见 %s ⇒ 判不了(2)\n", chsecSHA12(revs[lo]), chsecSHA12(a.shardFirst))
		return 2
	}
	blk := chsecCompileSpecs(chsecBlockSpecs)
	inf := chsecCompileSpecs(chsecInfoSpecs)
	hits, infoHits, counted := chsecScanCommits(revs[lo:hi], repo, lits, regexes, isolated, entries, exList, blk, inf,
		lo, false, map[string]chsecVerdict{}, map[string]chsecCount{}, io.Discard)
	cj := map[string][]int{}
	for key, v := range counted {
		cj[key] = []int{v.idx, v.bytes}
	}
	payload := struct {
		Hits     []chsecHit       `json:"hits"`
		InfoHits []chsecInfoHit   `json:"info_hits"`
		Counted  map[string][]int `json:"counted"`
	}{hits, infoHits, cj}
	_ = stdout
	if a.shardOut != "" {
		fh, err := os.Create(a.shardOut)
		if err != nil {
			return 2
		}
		err = chsecEncodeNoTrailNL(payload, fh)
		_ = fh.Close()
		if err != nil {
			return 2
		}
	}
	return 0
}

// ── 自检面（`--self-test`）：成对正负控 + 三格防空绿 · 仓外夹具 · 不碰真目标 ─────────────────────

var chsecTmpDirs []string

var chsecOctRe = regexp.MustCompile(`\\[0-7]{3}`)

// 夹具里的敏感文本一律**拼装写**（源码里不留完整形态）。
var (
	chsecFixAWS   = "AKIA" + strings.Repeat("0", 16)
	chsecFixPEM   = "-----BEGIN " + "RSA PRIVATE KEY-----"
	chsecFixSlack = "xoxb-" + strings.Repeat("0", 12)
	chsecFixHome  = "/Users/" + "ms" + "01"
)

var (
	chsecFixDirty    = "# 夹具：一段不该公开的文本（拼装写 · 本件源码里不留完整形态）\n" + `k1 = "` + chsecFixAWS + `"` + "\n"
	chsecFixPEMFile  = `k2 = "` + chsecFixPEM + `"` + "\n"
	chsecFixHomeFile = `HOME_HINT = "` + chsecFixHome + `"` + "\n"
	chsecFixGood     = "def main():\n    return 0\n"
	chsecFixWL       = "src/\n"
)

type chsecFixtureFile struct{ rel, body string }

func chsecWriteFile(path, text string) error {
	dir := chsecPyDirname(path)
	if dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(path, []byte(text), 0o644)
}

func chsecRunQuiet(cwd string, args ...string) {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = cwd
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	_ = cmd.Run()
}

// chsecMkFixtureRepo：在**仓外**临时目录里现搭一个最小 git 仓（真 git init + 一笔提交）。
func chsecMkFixtureRepo(base, name string, files []chsecFixtureFile, msg, whitelist string) string {
	root, err := os.MkdirTemp(base, name+"-")
	if err != nil {
		return ""
	}
	chsecTmpDirs = append(chsecTmpDirs, root)
	_ = chsecWriteFile(chsecPyJoin(root, "publish", "whitelist.txt"), whitelist)
	for _, f := range files {
		_ = chsecWriteFile(chsecPyJoin(root, f.rel), f.body)
	}
	ident := []string{"-c", "user.name=selftest", "-c", "user.email=selftest@example.invalid"}
	chsecRunQuiet(root, "git", "init", "-q")
	chsecRunQuiet(root, append(append([]string{"git"}, ident...), "add", "-A")...)
	chsecRunQuiet(root, append(append([]string{"git"}, ident...), "commit", "-q", "-m", msg)...)
	return root
}

func chsecMkTempDir(base, prefix string) string {
	d, err := os.MkdirTemp(base, prefix)
	if err != nil {
		return ""
	}
	chsecTmpDirs = append(chsecTmpDirs, d)
	return d
}

// chsecRunCLI：跑**真入口**（同进程 · 真返回值）⇒ (rc, 两路输出合并)。
func chsecRunCLI(repo string) (int, string) {
	var o, e bytes.Buffer
	rc := checkHistorySecretsGateMain(&o, &e, []string{"--repo", repo})
	return rc, o.String() + e.String()
}

func chsecCleanup() {
	for _, d := range chsecTmpDirs {
		_ = os.RemoveAll(d)
	}
	chsecTmpDirs = nil
}

func chsecYesNo(b bool) string {
	if b {
		return "是"
	}
	return "否"
}

// chsecHasNo：py 里的「有」/「无」二值（如空转读数那一格）。
func chsecHasNo(b bool) string {
	if b {
		return "有"
	}
	return "无"
}

func chsecSelfTest(stdout, stderr io.Writer) int {
	d := chsecMkTempDir("", "chsec-selftest-")
	defer chsecCleanup()
	bad, total := 0, 0
	caseFn := func(tag, desc string, ok bool, detail string) {
		total++
		mark := "✓"
		if !ok {
			mark = "✗"
			bad++
		}
		fmt.Fprintf(stdout, "  %s %s %s\n       %s\n", mark, tag, desc, detail)
	}

	fmt.Fprint(stdout, "── "+chsecProg+" 自检（成对正负控 · 防空绿档 · 仓外夹具 · 不碰真目标）──\n")

	fmt.Fprint(stdout, "[负控] 白名单内的件里放真形状敏感文本（判据 `BLOCK`）⇒ 必判红 rc=1 且逐档点名\n")
	fmt.Fprint(stdout, "       ★ 两条判据摆**两个件**：一个 blob 只出一个 verdict（首个命中的模式）\n")
	dirty := chsecMkFixtureRepo(d, "dirty", []chsecFixtureFile{{"src/app.py", chsecFixDirty}, {"src/priv.py", chsecFixPEMFile}}, "夹具：初始提交", chsecFixWL)
	rc, text := chsecRunCLI(dirty)
	caseFn("①负控", "白名单内两件（AWS 形 / 私钥块）⇒ rc=1 且两档都点名",
		rc == 1 && strings.Contains(text, "AWS Access Key") && strings.Contains(text, "私钥块") && strings.Contains(text, "阻断级命中"),
		fmt.Sprintf("rc=%d（期望 1）· 命中文里两档都在：%s", rc,
			chsecYesNo(strings.Contains(text, "AWS Access Key") && strings.Contains(text, "私钥块"))))

	fmt.Fprint(stdout, "[负控] 提交信息本身也会被镜像 ⇒ 一起审（`<commit-message>`）\n")
	repoMsg := chsecMkFixtureRepo(d, "commit-msg", []chsecFixtureFile{{"src/app.py", chsecFixGood}}, "夹具：令牌 "+chsecFixSlack, chsecFixWL)
	rc, text = chsecRunCLI(repoMsg)
	caseFn("②负控", "提交信息含 Slack 令牌形 ⇒ rc=1 且点名 `<commit-message>`",
		rc == 1 && strings.Contains(text, "Slack 令牌") && strings.Contains(text, "<commit-message>"),
		fmt.Sprintf("rc=%d（期望 1）", rc))

	fmt.Fprint(stdout, "[正控] 与 ② 成对：同一棵树、只把提交信息里那一形去掉 ⇒ 回绿 rc=0\n")
	repoMsgOk := chsecMkFixtureRepo(d, "commit-msg-ok", []chsecFixtureFile{{"src/app.py", chsecFixGood}}, "夹具：干净的提交信息", chsecFixWL)
	rc, text = chsecRunCLI(repoMsgOk)
	caseFn("③正控", "复原（提交信息无令牌形）⇒ rc=0 且输出「结果: 通过」",
		rc == 0 && strings.Contains(text, "结果: 通过"), fmt.Sprintf("rc=%d（期望 0）", rc))

	fmt.Fprint(stdout, "[正控] 干净夹具仓 ⇒ rc=0（不误报）\n")
	clean := chsecMkFixtureRepo(d, "clean", []chsecFixtureFile{{"src/app.py", chsecFixGood}}, "夹具：初始提交", chsecFixWL)
	rc, text = chsecRunCLI(clean)
	caseFn("④正控", "干净件 ⇒ rc=0 且输出「结果: 通过」", rc == 0 && strings.Contains(text, "结果: 通过"),
		fmt.Sprintf("rc=%d（期望 0）", rc))

	fmt.Fprint(stdout, "[区分度负控] 同一段文本放进**白名单外**的件 ⇒ 必放绿（只审真会被公开的路径）\n")
	outside := chsecMkFixtureRepo(d, "outside", []chsecFixtureFile{{"notes/secret.md", chsecFixDirty}}, "夹具：初始提交", chsecFixWL)
	rc, text = chsecRunCLI(outside)
	caseFn("⑤负控", "白名单外同一段文本 ⇒ rc=0（白名单判据不许被绕过）",
		rc == 0 && !strings.Contains(text, "阻断级命中"), fmt.Sprintf("rc=%d（期望 0）", rc))

	fmt.Fprint(stdout, "[正控] 白名单内但命中噪声后缀（`.bak`）⇒ 不审；且**空转读数必须逐字可见**（不许静默）\n")
	noise := chsecMkFixtureRepo(d, "noise", []chsecFixtureFile{{"src/app.py.bak", chsecFixDirty}}, "夹具：初始提交", chsecFixWL)
	rc, text = chsecRunCLI(noise)
	caseFn("⑥正控", "噪声后缀件不审 ⇒ rc=0，且打印「文件版本 0 个」（空转可见）",
		rc == 0 && strings.Contains(text, "文件版本 0 个"),
		fmt.Sprintf("rc=%d · 空转读数 %s", rc, chsecHasNo(strings.Contains(text, "文件版本 0 个"))))

	fmt.Fprint(stdout, "[退码钳] `--repo` 不是 git 仓 ⇒ rc=2（判不了 ≠ 绿）\n")
	nogit := chsecMkTempDir(d, "chsec-nogit-")
	_ = chsecWriteFile(chsecPyJoin(nogit, "publish", "whitelist.txt"), chsecFixWL)
	rc, text = chsecRunCLI(nogit)
	caseFn("⑦退码钳", "非 git 仓 ⇒ rc=2 且逐字说明「不是 git 仓库」",
		rc == 2 && strings.Contains(text, "不是 git 仓库"), fmt.Sprintf("rc=%d（期望 2）", rc))

	fmt.Fprint(stdout, "[正控] 替换规则**先变换、再扫描**：白名单内的家目录形会被改写成 `~` ⇒ 不判红\n")
	home := chsecMkFixtureRepo(d, "home", []chsecFixtureFile{{"src/app.py", chsecFixHomeFile}}, "夹具：初始提交", chsecFixWL)
	rc, text = chsecRunCLI(home)
	caseFn("⑧正控", "规则会改写的形状 ⇒ rc=0（先变换再扫描 —— 否则第一版误报的硬伤会回来）",
		rc == 0, fmt.Sprintf("rc=%d（期望 0）", rc))

	fmt.Fprint(stdout, "[防空绿档] 把判据真源弄哑 / 换成恒真 ⇒ 读数必须跟着变（不变 = 那条真源本来就是死的）\n")
	savedBlocks := chsecBlockSpecs
	rcMute, _ := func() (int, string) {
		chsecBlockSpecs = nil
		defer func() { chsecBlockSpecs = savedBlocks }()
		return chsecRunCLI(dirty)
	}()
	caseFn("⑨防空绿", "清空 `BLOCK` 模式表 ⇒ ① 那棵**必红**的树退回 rc=0（判红侧不是恒红）",
		rcMute == 0, fmt.Sprintf("rc=%d（期望 0）", rcMute))

	savedPub := chsecPublishedPathFn
	rcConst, _ := func() (int, string) {
		chsecPublishedPathFn = func(rel string, entries []string) bool { return true }
		defer func() { chsecPublishedPathFn = savedPub }()
		return chsecRunCLI(outside)
	}()
	caseFn("⑩防空绿", "`published_path` 换成恒真 ⇒ ⑤ 那棵**放绿**的树变红（放行侧不是恒绿）",
		rcConst == 1, fmt.Sprintf("rc=%d（期望 1）", rcConst))

	savedRules := chsecLoadRulesFn
	rcNoRule, _ := func() (int, string) {
		chsecLoadRulesFn = func(string) ([]chsecLit, []chsecRx, []string) { return []chsecLit{}, []chsecRx{}, []string{} }
		defer func() { chsecLoadRulesFn = savedRules }()
		return chsecRunCLI(home)
	}()
	caseFn("⑪防空绿", "`load_rules` 换成空规则 ⇒ ⑧ 那棵放绿的树变红（「先变换再扫描」是活的）",
		rcNoRule == 1, fmt.Sprintf("rc=%d（期望 1）", rcNoRule))

	fmt.Fprint(stdout, "[判红] 白名单内**中文名件**带真形状敏感文本 ⇒ rc=1 且**原样**点名（不是转义形态）\n")
	cjkRel := "src/" + "中文名件.py"
	cjkRepo := chsecMkFixtureRepo(d, "cjk", []chsecFixtureFile{{cjkRel, chsecFixDirty}}, "夹具：初始提交", chsecFixWL)
	rc, text = chsecRunCLI(cjkRepo)
	octDisp := "是"
	if chsecOctRe.MatchString(text) {
		octDisp = "否（出了 \\350 这类形态）"
	}
	caseFn("⑬中文名件", "取清单能**原样拿到真名** ⇒ 该件被扫到、判红、逐字点名",
		rc == 1 && strings.Contains(text, cjkRel) && !strings.Contains(text, "\"src/") && !chsecOctRe.MatchString(text),
		fmt.Sprintf("rc=%d（期望 1）· 原样点名 %s · 无八进制转义 %s", rc, chsecYesNo(strings.Contains(text, cjkRel)), octDisp))

	fmt.Fprint(stdout, "[牙齿] 同仓同件：默认 quotePath 下 git **确实**把它转义出去（⇒ 丢那个 `-c` 后 ⑬ 必红）\n")
	naive := chsecShText([]string{"git", "-c", "core.quotepath=true", "ls-tree", "-r", "HEAD", "--name-only"}, cjkRepo)
	naiveEsc := chsecOctRe.MatchString(naive) && strings.Contains(naive, "\"src/")
	inDisp := func(b bool) string {
		if b {
			return "在"
		}
		return "不在"
	}
	caseFn("⑭牙齿", "默认 quotePath 下列表里是转义形态、原样名不在（不是恒绿摆设）",
		naiveEsc && !strings.Contains(naive, cjkRel),
		fmt.Sprintf("转义形态 %s · 原样名 %s（期望：有转义 / 无原样）", inDisp(naiveEsc), inDisp(strings.Contains(naive, cjkRel))))

	fmt.Fprint(stdout, "[真目标] 现读真仓**结构**（**不跑全历史** —— 实测 3m55s，见文件头）⇒ 结构断言 ＋ 计数照实报\n")
	root := chsecRepoRoot()
	revsN := len(chsecFields(chsecShText([]string{"git", "rev-list", "--all"}, root)))
	ents := len(chsecLoadWhitelist(chsecWhitelistPath(root)))
	lits, regexes, _ := chsecLoadRules(chsecRulesPath(root))
	ex := len(chsecLoadExcludes(chsecPublishShPath(root)))
	okReal := chsecIsDir(chsecPyJoin(root, ".git")) && revsN > 0 && ents > 0 && (len(lits)+len(regexes)) > 0 && ex > 0
	caseFn("⑫真目标", "真仓：git 仓成立 · 提交数 > 0 · 白名单 / 规则 / 排除项条数 > 0", okReal,
		fmt.Sprintf("%s ⇒ 提交 %d 笔 · 白名单 %d 条 · 规则 %d 条（字面 %d + 正则 %d）· 排除项 %d 条",
			root, revsN, ents, len(lits)+len(regexes), len(lits), len(regexes), ex))
	fmt.Fprint(stdout, "  ⚠ 照实记（只报 · 不当绿）：本件真跑现读 **rc=1**（全历史有阻断级命中 · 4 分钟档）——\n")
	fmt.Fprint(stdout, "     它的两个挂点是提交闸 `tri-report`（只报告 · 不计退码）与 `real-gates.sh --full`；\n")
	fmt.Fprint(stdout, "     本批只补自检面，判据与真跑退码一字未改。\n")

	fmt.Fprintf(stdout, "  自证用例：%d 条 · 断言条数 %d（少跑一格即失效）\n", total, chsecWantCase)
	if total != chsecWantCase {
		fmt.Fprintf(stderr, "✗ 判不了：自检用例数对不上（应 %d，实跑 %d）⇒ rc=2（不许当绿）\n", chsecWantCase, total)
		return 2
	}
	if bad == 0 {
		fmt.Fprint(stdout, "⇒ 自检 全过（rc=0）\n")
		return 0
	}
	fmt.Fprintf(stdout, "⇒ 自检 有 %d 条断言不过（rc=1）\n", bad)
	return 1
}
