// check_git_path_outlet.go —— 门禁件 `scripts/gates/check-git-path-outlet.py` 的 Go 真搬（M2 · 甲案 C 类第 8 枚）。
//
// 本单边界（照实）：只落码、不接线、不注册命令、不删 py 原件。
//
//	· 入口 `gitPathOutletGateMain` 目前无调用点 —— M2 的刻意状态（先证逐字相同，接线另批）。
//	· 入口签名与 py `main()` 对齐：func gitPathOutletGateMain(stdout, stderr io.Writer, argv []string) int
//	  argv 与 py `sys.argv[1:]` 同形（不含程序名）；退码 0 判到 / 1 rc 档有命中 / 2 不给结论。
//
// 标准声明 (a) 默认根定位差异（不声称逐字对齐）：
//
//	py `repo_default = dirname(dirname(abspath(__file__)))` —— 由脚本位置（<根>/scripts/gates）上溯两层，与 cwd 无关；
//	Go 入口无 __file__ 语境 ⇒ 默认从 cwd 上溯 <=6 层找含 `scripts/gates/` 目录的那一级、找不到回落 cwd。
//	「脚本位置 != cwd 上溯根」时路径推导面不同 ⇒ 对拍一律显式传 `--repo`。
//
// 标准声明 (b) 自指检查（先真判）：
//
//	① 本件在**本门自己**的扫描面内（SCAN_ROOTS 含 `core`，SKIP_DIRS 不含 core/cmd/zerg）⇒ **是**。
//	   处置：与 py 同法，把全部件名 / 旗标 / 拼法键 / 夹具串一律**拼接构造**（`"ls" + "-" + "files"` 一类），
//	   使本件在扫描面上零命中。取证：落仓前后 py 门对真仓的读数逐字相同（见回执）。
//	② 件头/正文不写「真引号包裹的绝对路径原串」；本件只用相对路径字面。
//	③ `--self-test` 的夹具串同样拼接构造，不落可匹配原串。
//	④ 本件是否落在**任何他门**扫描面内：另测（见回执），未测项如实列入未覆盖面。
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const gpoVersion = "1.2.0"

const gpoSelfRel = "scripts/gates/check-git-path-outlet.py"

// ── 件名子命令名 / 旗标名：拼接构造 —— 本件自扫时不许自指命中 ────────────────────
var (
	gpoNLsFiles  = "ls" + "-" + "files"
	gpoNLsTree   = "ls" + "-" + "tree"
	gpoNDiff     = "diff"
	gpoNDiffTree = "diff" + "-" + "tree"
	gpoNShow     = "show"
	gpoNLog      = "log"
	gpoNStatus   = "status"
	gpoNGrep     = "grep"

	gpoFNameOnly  = "--" + "name-only"
	gpoFPorcelain = "--" + "porcelain"
	gpoFShort     = "--" + "short"
	gpoFStat      = "--" + "stat"
	gpoFZ         = "-" + "z"
	gpoFC         = "-" + "c"
	gpoKeyUpper   = "core." + "quote" + "Path"
	gpoKeyLower   = "core." + "quote" + "path"
	gpoGitTok     = "git"
)

type gpoRule struct{ ID, Desc string }

var gpoRules = []gpoRule{
	{"GPO-F1", "裸取件名：五类件名面，argv 里 -z / -c 都不带"},
	{"GPO-F2", "-c 后置在子命令之后（位置铁律违背）"},
	{"GPO-F3", "大写拼法 " + gpoKeyUpper + "（本单只认全小写 " + gpoKeyLower + "）"},
	{"GPO-F4", "会转义子命令（名单其余形态）argv 无免疫开关"},
	{"GPO-F5", "取件名输出直入匹配 / JSON / 拼路径（未经出口）"},
}

const gpoOutletPrefix = "core/internal/gitpaths/"

// gpoOutletAllow —— 白名单例外登记（现读零登记 · 与 py 同）。
type gpoAllowEntry struct{ Path, Rule, Reason, Replacement string }

var gpoOutletAllow = []gpoAllowEntry{}

// gpoTextAllowEntry —— 文字面（非 argv）例外登记：逐 (件, 规则, 锚) + reason + replacement。
type gpoTextAllowEntry struct{ Path, Rule, Anchor, Reason, Replacement string }

var gpoTextAllow = []gpoTextAllowEntry{
	{
		Path: "core/cmd/zerg/contract.go", Rule: "GPO-F1",
		Anchor:      "b.WriteString(\"  " + gpoGitTok + " " + gpoNGrep + " -l",
		Reason:      "帮助串：这一处是把「复跑判据」印给用户看的**文字**（`b.WriteString` 的常量），不是 argv —— 没有哪台机器会去跑它，故不产生转义名。",
		Replacement: "若要真给可复跑命令，交由「命令串产地」统一（另批）；本门不判文字面。",
	},
	{
		Path: "scripts/gates/check-publish-face-sync.py", Rule: "GPO-F1",
		Anchor:      "print(\"✗ " + gpoGitTok + " " + gpoNLsFiles + " 失败：%s（rc=%d）",
		Reason:      "消息串：`r.returncode != 0` 分支里的报错文本；真调用在**上一行**且已带 `-c " + gpoKeyLower + "=false`（同件 L102）。",
		Replacement: "无需改：真调用已合规。",
	},
	// ── 2026-10-02 · M2 py→Go 端口补齐（与 py 侧 TEXT_ALLOW 逐字同族同因）───────────
	//    这五条与上面 py 侧三条**同因**：命中行是**消息串**（非 argv），真调用已合规
	//    （`-z` 或 `-C`+`-c core.quotepath=false`+`-z`）。⑷⑸ 文案对现实现已陈旧（实走
	//    文件系统现走 ⇒ 不跑 git），只登记、不改文案。
	{
		Path: "core/cmd/zerg/check_affected_map.go", Rule: "GPO-F1",
		Anchor:      "fmt.Fprintf(stdout, \"[缺读数] " + gpoGitTok + " " + gpoNLsFiles + " 取不到：%s",
		Reason:      "消息串：`derr != nil` 分支里的报错文本；真取数走 `os.ReadDir` 文件系统现走（同函数 L944 注释逐字「✗ 不经 git」）⇒ 没有哪台机器会去跑这条 git 命令。该文案对现实现已陈旧，另批校正时本锚随之更新。",
		Replacement: "无需改：本行只是报错文本里的字面。",
	},
	{
		Path: "core/cmd/zerg/check_dangling_refs.go", Rule: "GPO-F1",
		Anchor:      "fmt.Fprintf(stderr, \"✗ " + gpoGitTok + " " + gpoNLsFiles + " rc=%d",
		Reason:      "消息串：Go 搬 py `check-dangling-refs.py` 的报错文本（py 侧同形已登记）；同函数真调用在 L1184 且已带 `-z`（随后按 NUL 逐字切分 stdout = 证据）。",
		Replacement: "无需改：真调用已合规；本行只是报错文本里的字面。",
	},
	{
		Path: "core/cmd/zerg/check_gap_axis_reconcile.go", Rule: "GPO-F1",
		Anchor:      "e = \"现读 " + gpoGitTok + " " + gpoNLsFiles + " 失败\"",
		Reason:      "消息串：Go 搬 py `check-gap-axis-reconcile.py` 的兜底文本（py 侧同形已登记）；同件真调用在 L212 且三锚点齐（`-C` + `" + gpoKeyLower + "=false` + `-z`）。",
		Replacement: "无需改：真调用已合规；本行只是报错文本里的字面。",
	},
	{
		Path: "core/cmd/zerg/who_references.go", Rule: "GPO-F1",
		Anchor:      "fmt.Fprintf(stderr, \"✗ " + gpoGitTok + " " + gpoNLsFiles + " rc=%d",
		Reason:      "消息串：Go 搬 py `who-references.py` 的报错文本（py 侧同形已登记）；同函数真调用在 L533 且已带 `-z`（随后按 NUL 切分 stdout = 证据）。",
		Replacement: "无需改：真调用已合规；本行只是报错文本里的字面。",
	},
	{
		Path: "scripts/gates/check-affected-map.py", Rule: "GPO-F1",
		Anchor:      "print('[缺读数] " + gpoGitTok + " " + gpoNLsFiles + " 取不到：%s",
		Reason:      "消息串：`except` 分支里的报错文本；真取数走 `disk_files` 的 `os.walk`（同函数 L351-352 注释逐字「✗ 不经 git」）⇒ 本行非 argv。该文案对现实现已陈旧，另批校正时本锚随之更新。",
		Replacement: "无需改：本行只是报错文本里的字面。",
	},
	// ── 2026-10-04 · 门⑦/门68 收口：`precommit-gates.sh` 的**步骤名文字面**（非 argv）──
	//    命中行是 `add_step … "<步骤名>" …` 里印给人看的**步骤名**；真取数走被点名的
	//    `check-public-marks-tracked-parity.py`（该件自带取件名现读对拍，见门94）。
	{
		Path: "scripts/gates/precommit-gates.sh", Rule: "GPO-F1",
		Anchor:      "门94 公开标记表时点对拍：理由列「未跟踪」↔ " + gpoGitTok + " " + gpoNLsFiles + " 现读",
		Reason:      "步骤名文字面：`add_step` 印给用户看的**步骤名**（引号内的常量），不是 argv —— 没有哪台机器会去跑它；真取数走被点名的 `check-public-marks-tracked-parity.py`（该件自带 " + gpoGitTok + " " + gpoNLsFiles + " 现读对拍）。",
		Replacement: "无需改：真调用在 check-public-marks-tracked-parity.py；本行只是步骤名里的字面。",
	},
}

// ── 文字面掩蔽：行首即注释的行，整行不判 ────────────────────────────────────────
var gpoCommentMarkers = []string{"#", "//", "/*", "*", "<!--"}

// 扫面域（相对仓根）
var gpoScanRoots = []string{"scripts", "core", "agent", "publish", "tools", "ui"}

var gpoScanExts = []string{".py", ".sh", ".go", ".rs", ".bash", ".zsh"}

var gpoSkipDirs = map[string]bool{
	".git": true, "vendor": true, "node_modules": true, "target": true, "dist": true,
	"bin": true, "data": true, ".zerg": true, "__pycache__": true, "zerg-wt": true,
	".venv": true, "venv": true,
}

const (
	gpoMaxFileBytes   = 2 * 1024 * 1024
	gpoMaxWindowLines = 4
	gpoMaxWindowChars = 4000
)

// F5 的消费动作（同一逻辑行内出现 ⇒ 取件名输出未经出口就进了下游）
var gpoConsumers = []string{
	".match(", ".search(", ".startswith(", ".endswith(", ".splitlines()",
	"json.dump", "os.path.join", "filepath.Join", "path.Join",
	"strings.HasPrefix", "strings.HasSuffix", "strings.Contains",
	".MatchString(", "grep -", "for ", "$(",
}

// 未判件（点名登记 · 不静默）
var gpoNotJudged = [][2]string{
	{gpoSelfRel,
		"自指：本件就是判据的产地（规则表 / 夹具串 / 消费动作表都在本件里）⇒ 扫自己必自指命中。" +
			"**点名登记，不判**；本件自身的正确性靠 `--self-test` 的仓外夹具格保证。"},
}

var gpoStripChars = "\"'`(),[]{}:="

var gpoStopTokens = map[string]bool{
	"": true, "|": true, "||": true, "&&": true, ";": true, ")": true, "]": true,
	"}": true, ">": true, ">>": true, "<": true, "then": true, "do": true,
	"done": true, "fi": true, "else": true,
}

var gpoJSONFields = []string{"schema", "gate", "version", "tier", "scanned", "skipped", "hits", "exceptions",
	"top", "not_judged", "mode", "hits_all",
	"refused", "refuse_reason", "allowlist_bad"}

// ─────────────────────────── py 字符串 / 路径原语（自写同构）───────────────────────────

// gpoPyIsSpace —— Python str.isspace / re \s 的 Unicode 语义（含 \x1c-\x1f）。
func gpoPyIsSpace(r rune) bool {
	return unicode.IsSpace(r) || (r >= 0x1c && r <= 0x1f)
}

func gpoPyStrip(s string) string {
	return strings.TrimFunc(s, gpoPyIsSpace)
}

func gpoPyStripLeft(s string) string {
	return strings.TrimLeftFunc(s, gpoPyIsSpace)
}

// gpoPyTokens —— re.compile(r"[^\s]+").findall
func gpoPyTokens(s string) []string {
	out := []string{}
	var buf []rune
	for _, r := range s {
		if gpoPyIsSpace(r) {
			if len(buf) > 0 {
				out = append(out, string(buf))
				buf = buf[:0]
			}
			continue
		}
		buf = append(buf, r)
	}
	if len(buf) > 0 {
		out = append(out, string(buf))
	}
	return out
}

// gpoPySplitLines —— Python str.splitlines（\n \r \r\n \v \f \x1c-\x1e \x85 \u2028 \u2029）。
func gpoPySplitLines(s string) []string {
	var out []string
	var buf []rune
	r := []rune(s)
	for i := 0; i < len(r); i++ {
		c := r[i]
		if c == '\r' {
			if i+1 < len(r) && r[i+1] == '\n' {
				i++
			}
			out = append(out, string(buf))
			buf = buf[:0]
			continue
		}
		if c == '\n' || c == '\v' || c == '\f' || c == 0x1c || c == 0x1d || c == 0x1e ||
			c == 0x85 || c == 0x2028 || c == 0x2029 {
			out = append(out, string(buf))
			buf = buf[:0]
			continue
		}
		buf = append(buf, c)
	}
	if len(buf) > 0 {
		out = append(out, string(buf))
	}
	return out
}

// gpoPyRepr —— Python repr(str)。
func gpoPyRepr(s string) string {
	q := byte('\'')
	if strings.ContainsRune(s, '\'') && !strings.ContainsRune(s, '"') {
		q = '"'
	}
	var b strings.Builder
	b.WriteByte(q)
	for _, r := range s {
		switch {
		case r == '\\':
			b.WriteString("\\\\")
		case r == rune(q):
			b.WriteByte('\\')
			b.WriteRune(r)
		case r == '\n':
			b.WriteString("\\n")
		case r == '\r':
			b.WriteString("\\r")
		case r == '\t':
			b.WriteString("\\t")
		case r < 0x20 || r == 0x7f:
			b.WriteString(fmt.Sprintf("\\x%02x", r))
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte(q)
	return b.String()
}

func gpoPyStrList(xs []string) string {
	if len(xs) == 0 {
		return "[]"
	}
	parts := make([]string, len(xs))
	for i, x := range xs {
		parts[i] = gpoPyRepr(x)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func gpoPyStrTuple(vals ...string) string {
	return "(" + strings.Join(vals, ", ") + ")"
}

func gpoPyStrInt(n int) string { return strconv.Itoa(n) }

func gpoPyStrBool(b bool) string {
	if b {
		return "True"
	}
	return "False"
}

func gpoTruncRunes(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		r = r[:n]
	}
	return string(r)
}

// gpoPyJoin —— posixpath.join 逐段同构（含空段 / 后段绝对路径 / 不归一 .. 三类语义）。
func gpoPyJoin(a string, rest ...string) string {
	path := a
	for _, b := range rest {
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

// gpoPyNormpath —— posixpath.normpath（含「保留恰两个前导斜杠」规则）。
func gpoPyNormpath(path string) string {
	if path == "" {
		return "."
	}
	initialSlashes := strings.HasPrefix(path, "/")
	if initialSlashes && strings.HasPrefix(path, "//") && !strings.HasPrefix(path, "///") {
		initialSlashes = true
		// 保留恰两个前导斜杠
	}
	twoSlashes := strings.HasPrefix(path, "//") && !strings.HasPrefix(path, "///")
	comps := strings.Split(path, "/")
	newComps := []string{}
	for _, comp := range comps {
		if comp == "" || comp == "." {
			continue
		}
		if comp != ".." || (!initialSlashes && len(newComps) == 0) || (len(newComps) > 0 && newComps[len(newComps)-1] == "..") {
			newComps = append(newComps, comp)
		} else if len(newComps) > 0 {
			newComps = newComps[:len(newComps)-1]
		}
	}
	out := strings.Join(newComps, "/")
	if initialSlashes {
		if twoSlashes {
			out = "//" + out
		} else {
			out = "/" + out
		}
	}
	if out == "" {
		return "."
	}
	return out
}

func gpoPyAbspath(path string) string {
	if !strings.HasPrefix(path, "/") {
		cwd, err := os.Getwd()
		if err == nil {
			path = gpoPyJoin(cwd, path)
		}
	}
	return gpoPyNormpath(path)
}

func gpoNonEmptySplit(p string) []string {
	out := []string{}
	for _, x := range strings.Split(p, "/") {
		if x != "" {
			out = append(out, x)
		}
	}
	return out
}

// gpoPyRelpath —— posixpath.relpath（公共前缀 + 「..」补位）。
func gpoPyRelpath(path, start string) string {
	if path == "" {
		return ""
	}
	sl := gpoNonEmptySplit(gpoPyAbspath(start))
	pl := gpoNonEmptySplit(gpoPyAbspath(path))
	i := 0
	for i < len(sl) && i < len(pl) && sl[i] == pl[i] {
		i++
	}
	ups := len(sl) - i
	if ups > 0 {
		parts := make([]string, 0, ups+len(pl)-i)
		for j := 0; j < ups; j++ {
			parts = append(parts, "..")
		}
		parts = append(parts, pl[i:]...)
		return strings.Join(parts, "/")
	}
	if i >= len(pl) {
		return "."
	}
	return strings.Join(pl[i:], "/")
}

// gpoPyDirname / gpoPyBasename —— posixpath.split 语义（剥尾斜杠）。
func gpoPySplit(p string) (string, string) {
	i := strings.LastIndex(p, "/") + 1
	head, tail := p[:i], p[i:]
	if head != "" && head != strings.Repeat("/", len(head)) {
		head = strings.TrimRight(head, "/")
	}
	return head, tail
}

func gpoPyDirname(p string) string  { h, _ := gpoPySplit(p); return h }
func gpoPyBasename(p string) string { _, t := gpoPySplit(p); return t }

// ─────────────────────────── 工具小件 ───────────────────────────

func gpoInSlice(xs []string, v string) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

func gpoEndsWithAny(s string, exts []string) bool {
	for _, e := range exts {
		if strings.HasSuffix(s, e) {
			return true
		}
	}
	return false
}

func gpoMin(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func gpoErrName(err error) string {
	if err == nil {
		return "OSError"
	}
	if os.IsNotExist(err) {
		return "FileNotFoundError"
	}
	if os.IsPermission(err) {
		return "PermissionError"
	}
	return "OSError"
}

func gpoIndexOfRunes(hay, needle []rune) int {
	if len(needle) == 0 {
		return 0
	}
	for i := 0; i+len(needle) <= len(hay); i++ {
		match := true
		for j := range needle {
			if hay[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

// gpoSnippet —— py `_snippet`：找不到 ⇒ strip 后前 120 码点。
func gpoSnippet(text, needle string) string {
	r := []rune(text)
	nr := []rune(needle)
	i := gpoIndexOfRunes(r, nr)
	if i < 0 {
		if len(r) > 120 {
			r = r[:120]
		}
		return gpoPyStrip(string(r))
	}
	lo := i - 40
	if lo < 0 {
		lo = 0
	}
	hi := i + len(nr) + 40
	if hi > len(r) {
		hi = len(r)
	}
	return gpoPyStrip(string(r[lo:hi]))
}

// gpoMaskLine —— 行首是注释标记 ⇒ 返回空串（该行无 argv 可判）。
func gpoMaskLine(line string) string {
	s := gpoPyStripLeft(line)
	for _, m := range gpoCommentMarkers {
		if strings.HasPrefix(s, m) {
			if m == "*" {
				rs := []rune(s)
				if len(rs) > 1 && !gpoPyIsSpace(rs[1]) {
					continue
				}
			}
			return ""
		}
	}
	return line
}

func gpoAnchorHit(e gpoTextAllowEntry, firstLine string) bool {
	return e.Anchor != "" && strings.Contains(firstLine, e.Anchor)
}

func gpoAnyAnchorHit(es []gpoTextAllowEntry, firstLine string) bool {
	for _, e := range es {
		if gpoAnchorHit(e, firstLine) {
			return true
		}
	}
	return false
}

func gpoFirstAnchorHit(es []gpoTextAllowEntry, firstLine string) gpoTextAllowEntry {
	for _, e := range es {
		if gpoAnchorHit(e, firstLine) {
			return e
		}
	}
	return gpoTextAllowEntry{}
}

// ─────────────────────────── 登记校验 ───────────────────────────

// gpoValidateTextAllow —— 文字面登记校验（可被 --self-test 换桩）。
var gpoValidateTextAllow = func(root string, checkExists bool) []string {
	bad := []string{}
	for _, e := range gpoTextAllow {
		p, r, a := e.Path, e.Rule, e.Anchor
		if gpoPyStrip(a) == "" {
			bad = append(bad, fmt.Sprintf("文字面登记 %s · %s 缺 anchor（无锚 = 整件豁免 = 不许）", p, r))
		}
		if gpoPyStrip(e.Reason) == "" {
			bad = append(bad, fmt.Sprintf("文字面登记 %s · %s 缺 reason", p, r))
		}
		if gpoPyStrip(e.Replacement) == "" {
			bad = append(bad, fmt.Sprintf("文字面登记 %s · %s 缺 replacement", p, r))
		}
		if !gpoInSlice(gpoRuleIDs(), r) {
			bad = append(bad, fmt.Sprintf("文字面登记 %s 的规则号 %s 不在规则表里", p, gpoPyRepr(r)))
		}
		if !checkExists {
			continue
		}
		fp := gpoPyJoin(root, p)
		st, err := os.Stat(fp)
		if p == "" || err != nil || st.IsDir() {
			bad = append(bad, fmt.Sprintf("文字面登记的件 %s 在盘上不存在（悬空登记）", p))
			continue
		}
		raw, err := os.ReadFile(fp)
		if err != nil || !utf8.Valid(raw) {
			gpoReadFails = append(gpoReadFails, gpoPyRepr(p)+"（文字面登记件读不动）")
			bad = append(bad, fmt.Sprintf("文字面登记的件 %s 读不动", p))
			continue
		}
		if a != "" && !strings.Contains(string(raw), a) {
			bad = append(bad, fmt.Sprintf("文字面登记 %s · %s 的锚在件里找不到（锚失效 ⇒ 该处又报红）", p, r))
		}
	}
	return bad
}

// gpoValidateAllowlist —— 例外登记三条校验（可被 --self-test 换桩）。
var gpoValidateAllowlist = func(root string, checkExists bool) []string {
	bad := []string{}
	for _, e := range gpoOutletAllow {
		p := e.Path
		if !strings.HasPrefix(p, gpoOutletPrefix) {
			bad = append(bad, fmt.Sprintf("例外件 %s 不在出口域 %s 之下（white-list 只许出口实现件本身）", gpoPyRepr(p), gpoOutletPrefix))
		}
		if gpoPyStrip(e.Reason) == "" {
			bad = append(bad, fmt.Sprintf("例外 %s · %s 缺 reason", p, e.Rule))
		}
		if gpoPyStrip(e.Replacement) == "" {
			bad = append(bad, fmt.Sprintf("例外 %s · %s 缺 replacement", p, e.Rule))
		}
		if checkExists && p != "" {
			if _, err := os.Stat(gpoPyJoin(root, p)); err != nil {
				bad = append(bad, fmt.Sprintf("例外件 %s 在盘上不存在（悬空例外）", p))
			}
		}
		if !gpoInSlice(gpoRuleIDs(), e.Rule) {
			bad = append(bad, fmt.Sprintf("例外 %s 的规则号 %s 不在规则表里", p, gpoPyRepr(e.Rule)))
		}
	}
	return bad
}

func gpoRuleIDs() []string {
	out := make([]string, 0, len(gpoRules))
	for _, r := range gpoRules {
		out = append(out, r.ID)
	}
	return out
}

// ─────────────────────────── 判据核心 ───────────────────────────

func gpoNorm(tok string) string {
	return strings.TrimFunc(tok, func(r rune) bool { return strings.ContainsRune(gpoStripChars, r) })
}

func gpoHasZ(toks []string) bool {
	for _, t := range toks {
		if t == gpoFZ {
			return true
		}
		rs := []rune(t)
		if strings.HasPrefix(t, "-") && !strings.HasPrefix(t, "--") && len(rs) > 1 && strings.Contains(string(rs[1:]), "z") {
			return true
		}
	}
	return false
}

// gpoQuoteKeyIn —— argv 里是否带 `-c <含拼法键的值>`（合写 `-ccore…` 也算）。
func gpoQuoteKeyIn(toks []string) bool {
	for i, t := range toks {
		if t == gpoFC && i+1 < len(toks) && strings.Contains(toks[i+1], "quote") {
			return true
		}
		if strings.HasPrefix(t, gpoFC) && len([]rune(t)) > 2 && strings.Contains(string([]rune(t)[2:]), "quote") {
			return true
		}
	}
	return false
}

// gpoFace ⇒ (面名, 是否五类面, 是否取件名调用)。
func gpoFace(sub string, args []string) (string, bool, bool) {
	has := func(x string) bool { return gpoInSlice(args, x) }
	switch sub {
	case gpoNLsFiles:
		return "ls-files", true, true
	case gpoNLsTree:
		if has("-r") || has(gpoFNameOnly) {
			return "ls-tree", true, true
		}
		return "", false, false
	case gpoNDiff:
		if has(gpoFNameOnly) {
			return "diff --name-only", true, true
		}
		if has(gpoFStat) {
			return "diff --stat", false, true
		}
		return "", false, false
	case gpoNStatus:
		if has(gpoFPorcelain) || has(gpoFShort) {
			return "status --porcelain/--short", true, true
		}
		return "", false, false
	case gpoNGrep:
		return "grep", true, true
	case gpoNDiffTree:
		if has("-r") || has(gpoFNameOnly) {
			return "diff-tree", false, true
		}
		return "", false, false
	case gpoNShow:
		if has(gpoFNameOnly) {
			return "show --name-only", false, true
		}
		return "", false, false
	case gpoNLog:
		if has(gpoFNameOnly) {
			return "log --name-only", false, true
		}
		return "", false, false
	}
	return "", false, false
}

type gpoInv struct {
	Sub       string
	Args      []string
	CIndex    int // -1 = None
	SubIndex  int
	RegionEnd int
	GitIndex  int
}

// gpoParseInvocations —— 从 token 序列抽调用（只认「git 之后紧跟一个已知子命令」这一形态）。
func gpoParseInvocations(tokens []string) []gpoInv {
	out := []gpoInv{}
	known := map[string]bool{
		gpoNLsFiles: true, gpoNLsTree: true, gpoNDiff: true, gpoNDiffTree: true,
		gpoNShow: true, gpoNLog: true, gpoNStatus: true, gpoNGrep: true,
	}
	i := 0
	for i < len(tokens) {
		if tokens[i] != gpoGitTok {
			i++
			continue
		}
		j := i + 1
		cIndex := -1
		for j < len(tokens) {
			t := tokens[j]
			if t == gpoFC {
				cIndex = j
				j += 2
				continue
			}
			if t == "-C" {
				j += 2
				continue
			}
			if strings.HasPrefix(t, "--git-dir") || strings.HasPrefix(t, "--work-tree") || t == "--no-pager" {
				j++
				continue
			}
			if strings.HasPrefix(t, "-") {
				j++
				continue
			}
			break
		}
		if j >= len(tokens) {
			i++
			continue
		}
		sub := tokens[j]
		if !known[sub] {
			i++
			continue
		}
		args := []string{}
		k := j + 1
		for k < len(tokens) && !gpoStopTokens[gpoNorm(tokens[k])] {
			args = append(args, gpoNorm(tokens[k]))
			k++
		}
		if cIndex < 0 {
			for d, av := range args {
				if av == gpoFC {
					cIndex = j + 1 + d
					break
				}
			}
		}
		out = append(out, gpoInv{Sub: sub, Args: args, CIndex: cIndex, SubIndex: j, RegionEnd: k, GitIndex: i})
		i = k
	}
	return out
}

type gpoJudged struct{ Rule, Why, Snippet string }

// gpoJudgeWindow —— 判一条逻辑窗口。
func gpoJudgeWindow(text string, skipRules []string, headN int, firstLine string) []gpoJudged {
	hits := []gpoJudged{}
	toks := gpoPyTokens(text)
	lineText := firstLine
	// GPO-F3：大写拼法（逐行文本面，含注释/常量/文档串）
	if !gpoInSlice(skipRules, "GPO-F3") && strings.Contains(lineText, gpoKeyUpper) {
		hits = append(hits, gpoJudged{"GPO-F3", "大写拼法 " + gpoKeyUpper, gpoSnippet(lineText, gpoKeyUpper)})
	}
	for _, inv := range gpoParseInvocations(toks) {
		if inv.GitIndex >= headN {
			continue
		}
		face, canon, ok := gpoFace(inv.Sub, inv.Args)
		if !ok {
			continue
		}
		argtoks := toks[inv.GitIndex:inv.RegionEnd]
		immune := gpoHasZ(argtoks) || gpoQuoteKeyIn(argtoks)
		// GPO-F2：`-c` 后置
		if !gpoInSlice(skipRules, "GPO-F2") && inv.CIndex >= 0 && inv.CIndex > inv.SubIndex {
			hits = append(hits, gpoJudged{"GPO-F2", fmt.Sprintf("`-c` 落在子命令 %s 之后", inv.Sub), gpoSnippet(text, gpoFC)})
		}
		// GPO-F1 / GPO-F4：会转义子命令而 argv 无免疫开关
		if !immune {
			rid := "GPO-F4"
			if canon {
				rid = "GPO-F1"
			}
			if !gpoInSlice(skipRules, rid) {
				hits = append(hits, gpoJudged{rid, fmt.Sprintf("子命令 %s 无 `-z` 也无 `-c`（会转义面）", inv.Sub), gpoSnippet(text, inv.Sub)})
			}
		}
		// GPO-F5：取件名输出直入消费动作（未经出口）
		if !gpoInSlice(skipRules, "GPO-F5") && !immune {
			tail := ""
			if inv.RegionEnd < len(toks) {
				tail = strings.Join(toks[inv.RegionEnd:], " ")
			}
			for _, c := range gpoConsumers {
				if strings.Contains(tail, c) || (c == "$(" && strings.Contains(text, "$("+gpoGitTok)) {
					hits = append(hits, gpoJudged{"GPO-F5", fmt.Sprintf("%s 面输出直入 %s（未经出口）", face, gpoPyRepr(c)), gpoSnippet(text, c)})
					break
				}
			}
		}
	}
	return hits
}

type gpoLogWindow struct {
	Line      int
	Text      string
	HeadN     int
	FirstLine string
}

// gpoLogicalWindows —— 窗口 = 本行 + 后 3 行（不跨语句结构合并）。
func gpoLogicalWindows(lines []string) []gpoLogWindow {
	n := len(lines)
	masked := make([]string, n)
	for i, x := range lines {
		masked[i] = gpoMaskLine(x)
	}
	out := make([]gpoLogWindow, 0, n)
	for i := 0; i < n; i++ {
		buf := ""
		head := 0
		headSet := false
		end := i + gpoMaxWindowLines
		if end > n {
			end = n
		}
		for k := i; k < end; k++ {
			if buf != "" {
				buf = buf + " " + masked[k]
			} else {
				buf = masked[k]
			}
			if !headSet {
				head = len(gpoPyTokens(buf))
				headSet = true
			}
			if utf8.RuneCountInString(buf) > gpoMaxWindowChars {
				break
			}
		}
		out = append(out, gpoLogWindow{Line: i + 1, Text: buf, HeadN: head, FirstLine: masked[i]})
	}
	return out
}

// ─────────────────────────── 列件 / 扫面 ───────────────────────────

type gpoWalkDir struct {
	Dirpath string
	Dirs    []string
	Files   []string
}

// gpoReadFails 登记「判据件读失败」的口子（B 类：py `except (OSError, UnicodeDecodeError): continue` 的静默面）。
// 覆盖 os.ReadFile 三处失败分支（文字面登记件 / 列件 / 扫面重读）。
// 汇总处（gitPathOutletGateMain）非空即 stderr 打 BLOCKED 头 ＋ 逐件点名 ＋ rc=2。
// 未覆盖面：非 UTF-8（!utf8.Valid）走同支合并登记；os.Open（gpoOsWalk）为另一支，本单未纳。
var gpoReadFails []string

func gpoIsSymlink(p string) bool {
	st, err := os.Lstat(p)
	if err != nil {
		return false
	}
	return st.Mode()&os.ModeSymlink != 0
}

// gpoOsWalk —— os.walk(top, followlinks=False) 的产出序（文件在前、按 readdir 原序下钻）。
func gpoOsWalk(top string, skipDirs map[string]bool) []gpoWalkDir {
	out := []gpoWalkDir{}
	var walk func(dir string)
	walk = func(dir string) {
		f, err := os.Open(dir)
		if err != nil {
			return
		}
		entries, err := f.ReadDir(-1)
		f.Close()
		if err != nil {
			return
		}
		var dirs, nondirs []string
		for _, e := range entries {
			name := e.Name()
			full := gpoPyJoin(dir, name)
			isDir := false
			if st, err := os.Stat(full); err == nil {
				isDir = st.IsDir()
			}
			if isDir {
				if gpoIsSymlink(full) {
					nondirs = append(nondirs, name)
				} else {
					dirs = append(dirs, name)
				}
			} else {
				nondirs = append(nondirs, name)
			}
		}
		kept := make([]string, 0, len(dirs))
		for _, d := range dirs {
			if !skipDirs[d] {
				kept = append(kept, d)
			}
		}
		out = append(out, gpoWalkDir{Dirpath: dir, Dirs: kept, Files: nondirs})
		for _, d := range kept {
			walk(gpoPyJoin(dir, d))
		}
	}
	walk(top)
	return out
}

// gpoIterFiles —— 列件（走 os.walk，不经 git ⇒ 对 CJK 名天生不瞎）。
func gpoIterFiles(root string, scanRoots, exts []string, skipDirs map[string]bool) ([]string, [][2]string) {
	files := []string{}
	skipped := [][2]string{}
	seen := map[string]bool{}
	for _, sub := range scanRoots {
		base := gpoPyJoin(root, sub)
		var cand []gpoWalkDir
		if st, err := os.Stat(base); err == nil && !st.IsDir() {
			cand = []gpoWalkDir{{Dirpath: gpoPyDirname(base), Dirs: nil, Files: []string{gpoPyBasename(base)}}}
		} else if st, err := os.Stat(base); err == nil && st.IsDir() {
			cand = gpoOsWalk(base, skipDirs)
		} else {
			continue
		}
		for _, wd := range cand {
			fns := append([]string(nil), wd.Files...)
			sort.Strings(fns)
			for _, fn := range fns {
				if !gpoEndsWithAny(fn, exts) {
					continue
				}
				p := gpoPyJoin(wd.Dirpath, fn)
				rel := gpoPyRelpath(p, root)
				if seen[rel] {
					continue
				}
				seen[rel] = true
				st, err := os.Stat(p)
				if err != nil {
					skipped = append(skipped, [2]string{rel, "读不动/非 UTF-8：" + gpoErrName(err)})
					continue
				}
				if st.Size() > gpoMaxFileBytes {
					skipped = append(skipped, [2]string{rel, fmt.Sprintf("件大于 %d 字节", gpoMaxFileBytes)})
					continue
				}
				raw, err := os.ReadFile(p)
				if err != nil {
					gpoReadFails = append(gpoReadFails, rel+"（列件读不动："+gpoErrName(err)+"）")
					skipped = append(skipped, [2]string{rel, "读不动/非 UTF-8：" + gpoErrName(err)})
					continue
				}
				if !utf8.Valid(raw) {
					skipped = append(skipped, [2]string{rel, "读不动/非 UTF-8：UnicodeDecodeError"})
					continue
				}
				files = append(files, rel)
			}
		}
	}
	sort.Strings(files)
	return files, skipped
}

type gpoHitRecord struct {
	File    string
	Line    int
	Rule    string
	Why     string
	Snippet string
}

type gpoExceptionRecord struct {
	File   string
	Line   int
	Rule   string
	Reason string
	IsText bool
}

type gpoScanResult struct {
	Scanned    []string
	Skipped    [][2]string
	Hits       []gpoHitRecord
	Exceptions []gpoExceptionRecord
}

// gpoScan —— 扫面 ⇒ 结果（只读）。
func gpoScan(root string, skipRules, scanRoots, exts []string, skipDirs map[string]bool) gpoScanResult {
	files, skipped := gpoIterFiles(root, scanRoots, exts, skipDirs)
	allow := map[string]gpoAllowEntry{}
	for _, e := range gpoOutletAllow {
		allow[e.Path+"\x00"+e.Rule] = e
	}
	textAllow := map[string][]gpoTextAllowEntry{}
	for _, e := range gpoTextAllow {
		k := e.Path + "\x00" + e.Rule
		textAllow[k] = append(textAllow[k], e)
	}
	hits := []gpoHitRecord{}
	exceptions := []gpoExceptionRecord{}
	for _, rel := range files {
		p := gpoPyJoin(root, rel)
		raw, err := os.ReadFile(p)
		if err != nil || !utf8.Valid(raw) {
			gpoReadFails = append(gpoReadFails, rel+"（扫面重读失败）")
			continue
		}
		text := string(raw)
		lines := gpoPySplitLines(text)
		for _, lw := range gpoLogicalWindows(lines) {
			for _, jd := range gpoJudgeWindow(lw.Text, skipRules, lw.HeadN, lw.FirstLine) {
				key := rel + "\x00" + jd.Rule
				if e, ok := allow[key]; ok {
					exceptions = append(exceptions, gpoExceptionRecord{File: rel, Line: lw.Line, Rule: jd.Rule, Reason: e.Reason})
				} else if lst, ok := textAllow[key]; ok && gpoAnyAnchorHit(lst, lw.FirstLine) {
					e := gpoFirstAnchorHit(lst, lw.FirstLine)
					exceptions = append(exceptions, gpoExceptionRecord{File: rel, Line: lw.Line, Rule: jd.Rule, Reason: e.Reason, IsText: true})
				} else {
					hits = append(hits, gpoHitRecord{File: rel, Line: lw.Line, Rule: jd.Rule, Why: jd.Why, Snippet: jd.Snippet})
				}
			}
		}
	}
	return gpoScanResult{Scanned: files, Skipped: skipped, Hits: hits, Exceptions: exceptions}
}

// ─────────────────────────── 自带回归格（B2 · 仓外夹具）───────────────────────────

// gpoGit —— 真起 git 子进程（清掉 GIT_* 与 ZERG_STATE_DIR）。
func gpoGit(cwd string, args []string) (int, string, string) {
	env := []string{}
	for _, e := range os.Environ() {
		k := e
		if idx := strings.IndexByte(e, '='); idx >= 0 {
			k = e[:idx]
		}
		switch k {
		case "GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_CONFIG",
			"GIT_CONFIG_PARAMETERS", "ZERG_STATE_DIR":
			continue
		}
		env = append(env, e)
	}
	cmd := exec.Command(gpoGitTok, args...)
	cmd.Dir = cwd
	cmd.Env = env
	cmd.Stdin = nil
	var so, se bytes.Buffer
	cmd.Stdout = &so
	cmd.Stderr = &se
	err := cmd.Run()
	rc := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			rc = ee.ExitCode()
		} else {
			rc = -1
		}
	}
	return rc, so.String(), se.String()
}

// gpoFixtureRepo —— 真造仓 + 真造中文名件（行为型靶子）。
func gpoFixtureRepo(base, name string) string {
	d := gpoPyJoin(base, name)
	if err := os.MkdirAll(d, 0o755); err != nil {
		panic("合成仓建不起来")
	}
	rc, _, _ := gpoGit(d, []string{"init", "-q", "."})
	if rc != 0 {
		panic("合成仓建不起来")
	}
	for _, fn := range []string{"a.txt", "中文名件.md"} {
		if err := os.WriteFile(gpoPyJoin(d, fn), []byte("x\n"), 0o644); err != nil {
			panic("合成仓建不起来")
		}
	}
	gpoGit(d, []string{"add", "--", "a.txt", "中文名件.md"})
	gpoGit(d, []string{"-c", "user.email=t@zerg.local", "-c", "user.name=t", "commit", "-qm", "init"})
	return d
}

func gpoRulesOf(hits []gpoHitRecord) []string {
	out := []string{}
	for _, h := range hits {
		out = append(out, h.Rule)
	}
	return out
}

func gpoCountRule(hits []gpoHitRecord, rule string) int {
	n := 0
	for _, h := range hits {
		if h.Rule == rule {
			n++
		}
	}
	return n
}

func gpoHasRule(hits []gpoHitRecord, rule string) bool { return gpoCountRule(hits, rule) > 0 }

func gpoSelftest(out io.Writer) bool {
	total := [2]int{}
	gpoReadFails = nil // ★ 自检自清：包级登记面不许被上一次调用（或上一个测试）的残留污染。
	caseFn := func(name, want, got string) bool {
		total[0]++
		ok := want == got
		if !ok {
			total[1]++
		}
		mark := "✓"
		if !ok {
			mark = "✗"
		}
		fmt.Fprintf(out, "  %s %s（期望 %s · 实得 %s）\n", mark, name, want, got)
		return ok
	}

	base, err := os.MkdirTemp("", "gpo-selftest-")
	if err != nil {
		fmt.Fprintf(out, "  自检 %d 格 · 失败 %d 格\n", 1, 1)
		return false
	}
	defer os.RemoveAll(base)

	// ① 行为靶子：真仓 + 真跑 CLI
	repo := gpoFixtureRepo(base, "probe")
	_, bareOut, _ := gpoGit(repo, []string{gpoNLsFiles})
	_, verbatimOut, _ := gpoGit(repo, []string{gpoNLsFiles, gpoFZ})
	bareLines := []string{}
	for _, l := range gpoPySplitLines(bareOut) {
		if gpoPyStrip(l) != "" {
			bareLines = append(bareLines, l)
		}
	}
	bareEscaped := 0
	for _, l := range bareLines {
		if strings.HasPrefix(l, "\"") && strings.HasSuffix(l, "\"") {
			bareEscaped++
		}
	}
	zNames := []string{}
	for _, p := range strings.Split(verbatimOut, "\x00") {
		if p != "" {
			zNames = append(zNames, p)
		}
	}
	caseFn("行为靶子：裸调真产出转义形态（首尾带引号）件 ≥1", gpoPyStrBool(true), gpoPyStrBool(bareEscaped >= 1))
	caseFn("行为靶子：合规写法真产出真 UTF-8 名（中文名件在内）", gpoPyStrBool(true), gpoPyStrBool(gpoInSlice(zNames, "中文名件.md")))
	caseFn("行为靶子：同一仓两条路件数一致", gpoPyStrInt(len(bareLines)), gpoPyStrInt(len(zNames)))

	// ⑤ 扫面自身对 CJK 名不瞎（行为型）
	files, _ := gpoIterFiles(repo, []string{"."}, []string{".txt", ".md"}, map[string]bool{})
	caseFn("⑤ 扫面清单含中文名件（os.walk 天生不转义）", gpoPyStrBool(true), gpoPyStrBool(gpoInSlice(files, "中文名件.md")))

	// 夹具：四格文本（拼接构造，避免本件自指）
	okLine := gpoGitTok + " " + gpoNLsFiles + " " + gpoFZ
	bareLine := gpoGitTok + " " + gpoNLsFiles
	lateC := gpoGitTok + " " + gpoNStatus + " " + gpoFPorcelain + " " + gpoFC + " " + gpoKeyLower + "=false"
	upper := "-c " + gpoKeyUpper + "=false " + gpoGitTok + " " + gpoNLsFiles + " " + gpoFZ

	runFixture := func(text, name string, skipRules []string) gpoScanResult {
		d := gpoPyJoin(base, name)
		_ = os.MkdirAll(d, 0o755)
		_ = os.WriteFile(gpoPyJoin(d, "case.sh"), []byte(text+"\n"), 0o644)
		return gpoScan(d, skipRules, []string{"."}, []string{".sh"}, map[string]bool{})
	}

	// ① 合规 ⇒ 全绿
	r1 := runFixture(okLine, "c1_ok", nil)
	caseFn("① 合规写法（带 -z）⇒ 命中 0", "0", gpoPyStrInt(len(r1.Hits)))

	// ② 裸调 ⇒ 红（GPO-F1）
	r2 := runFixture(bareLine, "c2_bare", nil)
	caseFn("② 裸调（无开关）⇒ 报红 · 规则 GPO-F1", gpoPyStrList([]string{"GPO-F1"}), gpoPyStrList(gpoRulesOf(r2.Hits)))

	// ③ `-c` 后置 ⇒ 红（GPO-F2）
	r3 := runFixture(lateC, "c3_late_c", nil)
	caseFn("③ `-c` 后置 ⇒ 报红 · 含 GPO-F2", gpoPyStrBool(true), gpoPyStrBool(gpoHasRule(r3.Hits, "GPO-F2")))

	// ④ 大写拼法 ⇒ 红（GPO-F3）
	r4 := runFixture(upper, "c4_upper", nil)
	caseFn("④ 大写拼法 ⇒ 报红 · 含 GPO-F3", gpoPyStrBool(true), gpoPyStrBool(gpoHasRule(r4.Hits, "GPO-F3")))

	// ⑥ 牙齿·拿掉 `-z`
	beforeZ := strings.Count(okLine, gpoFZ)
	mutated := strings.ReplaceAll(okLine, " "+gpoFZ, "")
	afterZ := strings.Count(mutated, gpoFZ)
	fmt.Fprintf(out, "    ⑥ 负控前置：`%s` 出现次数 替换前=%d · 替换后=%d\n", gpoFZ, beforeZ, afterZ)
	caseFn("⑥ 负控前置断言：拿掉 `-z` 后次数差 == 1", "1", gpoPyStrInt(beforeZ-afterZ))
	caseFn("⑥ 负控前置断言：替换真的改了文本", gpoPyStrBool(true), gpoPyStrBool(mutated != okLine))
	r6 := runFixture(mutated, "c6_noz", nil)
	caseFn("⑥ 牙齿：拿掉 `-z` 再跑 ⇒ 变红（GPO-F1）", gpoPyStrBool(true), gpoPyStrBool(gpoHasRule(r6.Hits, "GPO-F1")))

	// ⑦ 牙齿·拿掉判据
	beforeR := gpoCountRule(r2.Hits, "GPO-F1")
	r7 := runFixture(bareLine, "c7_ruledrop", []string{"GPO-F1", "GPO-F4"})
	afterR := gpoCountRule(r7.Hits, "GPO-F1")
	fmt.Fprintf(out, "    ⑦ 负控前置：`GPO-F1` 命中数 摘掉判据前=%d · 摘掉后=%d\n", beforeR, afterR)
	caseFn("⑦ 负控前置断言：摘掉判据后 GPO-F1 命中差 == 1", "1", gpoPyStrInt(beforeR-afterR))
	caseFn("⑦ 负控：摘掉抓裸调的那颗判据 ⇒ 同一夹具不再报该条", "[]", gpoPyStrList(gpoRulesOf(r7.Hits)))

	// ⑧ 例外登记三条校验
	caseFn("⑧ 例外登记：现表通过三条校验（件存在性另由真仓跑覆盖）", "[]", gpoPyStrList(gpoValidateAllowlist(base, false)))
	saved := gpoOutletAllow
	gpoOutletAllow = []gpoAllowEntry{{Path: gpoOutletPrefix + "x.go", Rule: "GPO-F1", Reason: "", Replacement: "y"}}
	caseFn("⑧ 例外负控：缺 reason ⇒ 校验报出", gpoPyStrBool(true), gpoPyStrBool(len(gpoValidateAllowlist(base, false)) >= 1))
	gpoOutletAllow = saved

	// ⑨ 注释里提到裸调字面 ⇒ 不得报红
	r9 := runFixture("# "+bareLine+"   ← 注释里提到裸调字面（旧口径）", "c9_comment", nil)
	caseFn("⑨ 注释里提到裸调字面 ⇒ 命中 0（文字面掩蔽 · 散文不是 argv）", "0", gpoPyStrInt(len(r9.Hits)))

	// ⑩ 文字面登记 = 锚定，不是整件豁免
	r10text := "# 注释里的旧口径：" + bareLine + "\n" + bareLine + "\n"
	savedT := gpoTextAllow
	gpoTextAllow = []gpoTextAllowEntry{{
		Path: "case.sh", Rule: "GPO-F1", Anchor: "# 注释里的旧口径：",
		Reason: "夹具：证明登记锚定的是那一处文字", Replacement: "夹具（无需改）",
	}}
	r10 := runFixture(r10text, "c10_anchor", nil)
	caseFn("⑩ 锚只对那一处文字生效 ⇒ 同件**真裸调行仍红**（恰 1 条 GPO-F1）", gpoPyStrList([]string{"GPO-F1"}), gpoPyStrList(gpoRulesOf(r10.Hits)))
	d10 := gpoPyJoin(base, "c10_anchor")
	caseFn("⑩ 登记负控：锚真在件里 ⇒ 校验通过", "[]", gpoPyStrList(gpoValidateTextAllow(d10, true)))
	gpoTextAllow = []gpoTextAllowEntry{{
		Path: "case.sh", Rule: "GPO-F1", Anchor: "锚不在件里-XYZ",
		Reason: "夹具：证明登记锚定的是那一处文字", Replacement: "夹具（无需改）",
	}}
	caseFn("⑩ 登记负控：锚不在件里 ⇒ 校验报出（锚失效不许静默变绿）", gpoPyStrBool(true), gpoPyStrBool(len(gpoValidateTextAllow(d10, true)) >= 1))
	gpoTextAllow = []gpoTextAllowEntry{{
		Path: "case.sh", Rule: "GPO-F1", Anchor: "",
		Reason: "夹具：证明登记锚定的是那一处文字", Replacement: "夹具（无需改）",
	}}
	caseFn("⑩ 登记负控：缺锚 ⇒ 校验报出（无锚=整件豁免=不许）", gpoPyStrBool(true), gpoPyStrBool(len(gpoValidateTextAllow(d10, true)) >= 1))
	gpoTextAllow = savedT

	// ⑪ 全量字段 `hits_all`
	d11 := gpoPyJoin(gpoPyJoin(base, "c11_all"), "scripts")
	_ = os.MkdirAll(d11, 0o755)
	var sb strings.Builder
	for i := 0; i < 12; i++ {
		sb.WriteString(bareLine)
		sb.WriteString("\n")
	}
	_ = os.WriteFile(gpoPyJoin(d11, "case.sh"), []byte(sb.String()), 0o644)
	sva, svt := gpoValidateAllowlist, gpoValidateTextAllow
	gpoValidateAllowlist = func(string, bool) []string { return nil }
	gpoValidateTextAllow = func(string, bool) []string { return nil }
	var buf11 bytes.Buffer
	_ = gitPathOutletGateMain(&buf11, io.Discard, []string{"--repo", gpoPyJoin(base, "c11_all"), "--json"})
	gpoValidateAllowlist, gpoValidateTextAllow = sva, svt
	j11 := gpoLastJSON(buf11.String())
	caseFn("⑪ `--json` 全量字段 `hits_all`：字段在 · 条数 == hits · `top` 仍只前 10 条",
		gpoPyStrTuple(gpoPyStrBool(true), "12", "10"),
		gpoPyStrTuple(gpoPyStrBool(gpoJSONHas(j11, "hits_all")), gpoPyStrInt(gpoJSONArrLen(j11, "hits_all")), gpoPyStrInt(gpoJSONArrLen(j11, "top"))))

	// ⑫ 扫面面空 / 根不存在 ⇒ rc=2 且点名（成对负控）
	sva2, svt2 := gpoValidateAllowlist, gpoValidateTextAllow
	gpoValidateAllowlist = func(string, bool) []string { return nil }
	gpoValidateTextAllow = func(string, bool) []string { return nil }
	runMain := func(repoArg string) (int, string) {
		var b bytes.Buffer
		rc := gitPathOutletGateMain(&b, io.Discard, []string{"--repo", repoArg, "--json"})
		return rc, b.String()
	}
	d12 := gpoPyJoin(gpoPyJoin(base, "c12_scan_real"), "scripts")
	_ = os.MkdirAll(d12, 0o755)
	_ = os.WriteFile(gpoPyJoin(d12, "case.sh"), []byte("x\n"), 0o644)
	rcP, outP := runMain(gpoPyDirname(d12))
	caseFn("⑫ 正控：根里真有可扫件 ⇒ 判到（rc=0 · 扫面 1 件）",
		gpoPyStrTuple("0", gpoPyStrBool(true)),
		gpoPyStrTuple(gpoPyStrInt(rcP), gpoPyStrBool(strings.Contains(outP, "扫面 1 件"))))
	d12e := gpoPyJoin(base, "c12_scan_empty")
	_ = os.MkdirAll(d12e, 0o755)
	rcA, outA := runMain(gpoPyJoin(base, "c12_根不存在-XYZ"))
	caseFn("⑫ 负控 A：根不存在 ⇒ rc=2（不给结论）", "2", gpoPyStrInt(rcA))
	caseFn("⑫ 负控 A 且逐字点名「扫面 0 件」", gpoPyStrBool(true), gpoPyStrBool(strings.Contains(outA, "扫面 0 件")))
	rcB, outB := runMain(d12e)
	caseFn("⑫ 负控 B：空目录（存在但一件没有）⇒ rc=2 —— 与正控同一条命令，只换根", "2", gpoPyStrInt(rcB))
	caseFn("⑫ 负控 B 且逐字点名「扫面 0 件」", gpoPyStrBool(true), gpoPyStrBool(strings.Contains(outB, "扫面 0 件")))
	j12 := gpoLastJSON(outB)
	caseFn("⑫ 负控 A/B 的机器面：refused=True · refuse_reason=empty_scan · scanned=0",
		gpoPyStrTuple(gpoPyStrBool(true), gpoPyRepr("empty_scan"), "0"),
		gpoPyStrTuple(gpoPyStrBool(gpoJSONBool(j12, "refused")), gpoPyRepr(gpoJSONStr(j12, "refuse_reason")), gpoPyStrInt(gpoJSONInt(j12, "scanned"))))
	gpoValidateAllowlist, gpoValidateTextAllow = sva2, svt2

	fmt.Fprintf(out, "  自检 %d 格 · 失败 %d 格\n", total[0], total[1])
	return total[1] == 0
}

func gpoLastJSON(s string) map[string]any {
	lines := gpoPySplitLines(gpoPyStrip(s))
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.HasPrefix(lines[i], "{") {
			var m map[string]any
			if err := json.Unmarshal([]byte(lines[i]), &m); err == nil {
				return m
			}
		}
	}
	return map[string]any{}
}

func gpoJSONHas(m map[string]any, k string) bool {
	_, ok := m[k]
	return ok
}

func gpoJSONArrLen(m map[string]any, k string) int {
	if v, ok := m[k].([]any); ok {
		return len(v)
	}
	return 0
}

func gpoJSONBool(m map[string]any, k string) bool {
	if v, ok := m[k].(bool); ok {
		return v
	}
	return false
}

func gpoJSONStr(m map[string]any, k string) string {
	if v, ok := m[k].(string); ok {
		return v
	}
	return ""
}

func gpoJSONInt(m map[string]any, k string) int {
	if v, ok := m[k].(float64); ok {
		return int(v)
	}
	return 0
}

// ─────────────────────────── 机器面（JSON）───────────────────────────

type gpoJF struct{ K, V string }

func gpoPyJSONStr(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString("\\\"")
		case '\\':
			b.WriteString("\\\\")
		case '\b':
			b.WriteString("\\b")
		case '\f':
			b.WriteString("\\f")
		case '\n':
			b.WriteString("\\n")
		case '\r':
			b.WriteString("\\r")
		case '\t':
			b.WriteString("\\t")
		default:
			if r < 0x20 {
				b.WriteString(fmt.Sprintf("\\u%04x", r))
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

func gpoJStr(s string) string { return gpoPyJSONStr(s) }

func gpoJObj(fields ...gpoJF) string {
	parts := make([]string, 0, len(fields))
	for _, f := range fields {
		parts = append(parts, gpoJStr(f.K)+": "+f.V)
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

func gpoJArr(vals []string) string { return "[" + strings.Join(vals, ", ") + "]" }

func gpoTopArr(hits []gpoHitRecord) string {
	n := gpoMin(10, len(hits))
	parts := make([]string, 0, n)
	for _, h := range hits[:n] {
		parts = append(parts, gpoJObj(
			gpoJF{"file", gpoJStr(h.File)},
			gpoJF{"line", strconv.Itoa(h.Line)},
			gpoJF{"rule", gpoJStr(h.Rule)},
		))
	}
	return gpoJArr(parts)
}

func gpoHitsAllArr(hits []gpoHitRecord) string {
	parts := make([]string, 0, len(hits))
	for _, h := range hits {
		parts = append(parts, gpoJObj(
			gpoJF{"file", gpoJStr(h.File)},
			gpoJF{"line", strconv.Itoa(h.Line)},
			gpoJF{"rule", gpoJStr(h.Rule)},
			gpoJF{"why", gpoJStr(h.Why)},
		))
	}
	return gpoJArr(parts)
}

func gpoJSONCommonHead(res gpoScanResult, tier string) []gpoJF {
	return []gpoJF{
		{"schema", gpoJStr("zerg-gate/v1")},
		{"gate", gpoJStr("check-git-path-outlet")},
		{"version", gpoJStr(gpoVersion)},
		{"tier", gpoJStr(tier)},
		{"scanned", strconv.Itoa(len(res.Scanned))},
		{"skipped", strconv.Itoa(len(res.Skipped))},
		{"hits", strconv.Itoa(len(res.Hits))},
		{"exceptions", strconv.Itoa(len(res.Exceptions))},
		{"top", gpoTopArr(res.Hits)},
		{"hits_all", gpoHitsAllArr(res.Hits)},
		{"not_judged", gpoJArr([]string{gpoJStr(gpoSelfRel)})},
	}
}

func gpoJSONNormal(res gpoScanResult, tier string) string {
	fields := gpoJSONCommonHead(res, tier)
	fields = append(fields, gpoJF{"mode", gpoJStr(tier)})
	return gpoJObj(fields...)
}

func gpoJSONRefuse(res gpoScanResult, tier, reason string, bad []string) string {
	fields := gpoJSONCommonHead(res, tier)
	fields = append(fields,
		gpoJF{"mode", gpoJStr("refuse")},
		gpoJF{"refused", "true"},
		gpoJF{"refuse_reason", gpoJStr(reason)},
		gpoJF{"allowlist_bad", gpoJArr(stringsToJSONStrs(bad))},
	)
	return gpoJObj(fields...)
}

func stringsToJSONStrs(xs []string) []string {
	out := make([]string, 0, len(xs))
	for _, x := range xs {
		out = append(out, gpoJStr(x))
	}
	return out
}

// ─────────────────────────── 主面 ───────────────────────────

const gpoListRules = "判据（C-6 · 黑名单五类）\n" +
	"--------------------------\n" +
	"  GPO-F1  裸取件名：ls-files / ls-tree -r|--name-only / diff --name-only /\n" +
	"          status --porcelain|--short / grep —— argv 里 `-z` 与 `-c` 都不带\n" +
	"  GPO-F2  `-c` 后置在子命令之后（位置铁律；`ls-files … -c …` 是 rc=0 的静默错）\n" +
	"  GPO-F3  大写拼法那一形（本单只认全小写）\n" +
	"  GPO-F4  其余会转义子命令（diff-tree / show --name-only / log --name-only / diff --stat）无免疫开关\n" +
	"  GPO-F5  取件名输出直入匹配 / JSON / 拼路径（未经出口）\n" +
	"★ GPO-F5 只在调用**不带**免疫开关时判（「未经出口」的字面义）\n" +
	"白名单例外：**仅** core/internal/gitpaths/ 的出口实现件本身；每条必带 reason + replacement\n" +
	"文字面（非 argv）例外：逐 **(件, 规则, 锚)** 登记 + reason + replacement —— 注释/帮助串/消息串/文档散文\n" +
	"档位：--tier report（默认·只报告 rc=0）· --tier rc（命中 ⇒ rc=1）\n" +
	"退码：0 判到 · 1 rc 档有命中 · 2 不给结论（扫面空 / 例外登记不合格 / 自检不过 / 用法错）\n"

const gpoUsageText = "usage: check-git-path-outlet.py [-h] [--repo REPO] [--tier {report,rc}]\n" +
	"                                [--strict] [--json [JSON]] [--list-rules]\n" +
	"                                [--self-test]\n"

const gpoHelpText = gpoUsageText + "\n" +
	"件名统一出口的调用点判据门（C-6 · GAP-20260927-216 ③）\n" +
	"\n" +
	"optional arguments:\n" +
	"  -h, --help          show this help message and exit\n" +
	"  --repo REPO\n" +
	"  --tier {report,rc}\n" +
	"  --strict            = --tier rc\n" +
	"  --json [JSON]\n" +
	"  --list-rules\n" +
	"  --self-test\n"

func gpoArgparseError(stderr io.Writer, msg string) {
	io.WriteString(stderr, gpoUsageText+"check-git-path-outlet.py: error: "+msg+"\n")
}

// gpoRefuseUnknownJSONFields —— `--json` 的字段名白名单前置校验（先于 argparse）。
func gpoRefuseUnknownJSONFields(argv []string, stderr io.Writer) *int {
	want := []string{}
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
	filtered := []string{}
	for _, f := range want {
		s := gpoPyStrip(f)
		if s != "" {
			filtered = append(filtered, s)
		}
	}
	bad := []string{}
	for _, f := range filtered {
		if !gpoInSlice(gpoJSONFields, f) {
			bad = append(bad, f)
		}
	}
	if len(bad) == 0 {
		return nil
	}
	parts := make([]string, len(bad))
	for i, b := range bad {
		parts[i] = "`" + b + "`"
	}
	fmt.Fprintf(stderr, "⛔ `--json` 不认的字段名：%s\n", strings.Join(parts, " · "))
	opts := make([]string, len(gpoJSONFields))
	for i, f := range gpoJSONFields {
		opts[i] = "`" + f + "`"
	}
	fmt.Fprintf(stderr, "   可选字段表：%s\n", strings.Join(opts, " · "))
	rc := 2
	return &rc
}

func gpoRefuseMachineFace(stdout io.Writer, js string, res gpoScanResult, tier, reason string, bad []string) {
	if js == "" {
		return
	}
	io.WriteString(stdout, gpoJSONRefuse(res, tier, reason, bad)+"\n")
}

// gpoDefaultRepo —— 从 cwd 上溯 <=6 层找含 `scripts/gates/` 的那一级、找不到回落 cwd。
func gpoDefaultRepo() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	cur := cwd
	for i := 0; i < 6; i++ {
		if st, err := os.Stat(gpoPyJoin(cur, "scripts", "gates")); err == nil && st.IsDir() {
			return cur
		}
		parent := gpoPyDirname(cur)
		if parent == cur || parent == "" {
			break
		}
		cur = parent
	}
	return cwd
}

type gpoArgs struct {
	Repo, Tier string
	Strict     bool
	JSON       string
	ListRules  bool
	SelfTest   bool
}

func gpoParseArgs(argv []string, stdout, stderr io.Writer) (gpoArgs, int, bool) {
	a := gpoArgs{Repo: gpoDefaultRepo(), Tier: "report"}
	i := 0
	for i < len(argv) {
		x := argv[i]
		switch {
		case x == "-h" || x == "--help":
			io.WriteString(stdout, gpoHelpText)
			return a, 0, true
		case x == "--strict":
			a.Strict = true
			i++
		case x == "--list-rules":
			a.ListRules = true
			i++
		case x == "--self-test":
			a.SelfTest = true
			i++
		case x == "--repo":
			if i+1 >= len(argv) {
				gpoArgparseError(stderr, "argument --repo: expected one argument")
				return a, 2, true
			}
			a.Repo = argv[i+1]
			i += 2
		case strings.HasPrefix(x, "--repo="):
			a.Repo = strings.TrimPrefix(x, "--repo=")
			i++
		case x == "--tier":
			if i+1 >= len(argv) {
				gpoArgparseError(stderr, "argument --tier: expected one argument")
				return a, 2, true
			}
			v := argv[i+1]
			if v != "report" && v != "rc" {
				gpoArgparseError(stderr, fmt.Sprintf("argument --tier: invalid choice: %s (choose from 'report', 'rc')", gpoPyRepr(v)))
				return a, 2, true
			}
			a.Tier = v
			i += 2
		case strings.HasPrefix(x, "--tier="):
			v := strings.TrimPrefix(x, "--tier=")
			if v != "report" && v != "rc" {
				gpoArgparseError(stderr, fmt.Sprintf("argument --tier: invalid choice: %s (choose from 'report', 'rc')", gpoPyRepr(v)))
				return a, 2, true
			}
			a.Tier = v
			i++
		case x == "--json":
			if i+1 < len(argv) && !strings.HasPrefix(argv[i+1], "-") {
				a.JSON = argv[i+1]
				i += 2
			} else {
				a.JSON = "full"
				i++
			}
		case strings.HasPrefix(x, "--json="):
			a.JSON = strings.TrimPrefix(x, "--json=")
			i++
		default:
			gpoArgparseError(stderr, "unrecognized arguments: "+x)
			return a, 2, true
		}
	}
	return a, 0, false
}

// gitPathOutletGateMain —— 门入口（与 py main() 逐字对齐）。
func gitPathOutletGateMain(stdout, stderr io.Writer, argv []string) int {
	if rc := gpoRefuseUnknownJSONFields(argv, stderr); rc != nil {
		return *rc
	}
	a, rcParse, done := gpoParseArgs(argv, stdout, stderr)
	if done {
		return rcParse
	}
	if a.ListRules {
		io.WriteString(stdout, gpoListRules)
		return 0
	}
	if a.SelfTest {
		io.WriteString(stdout, "── 自带回归格（仓外夹具 · 真造仓 + 真跑 CLI）──\n")
		if gpoSelftest(stdout) {
			return 0
		}
		return 2
	}

	tier := "report"
	if a.Strict || a.Tier == "rc" {
		tier = "rc"
	}
	bad := append(gpoValidateAllowlist(a.Repo, true), gpoValidateTextAllow(a.Repo, true)...)
	res := gpoScan(a.Repo, nil, gpoScanRoots, gpoScanExts, gpoSkipDirs)
	if len(gpoReadFails) > 0 {
		fmt.Fprintf(stderr, "⛔ BLOCKED：判据件读失败 %d 件（读失败静默 ⇒ 不给结论 · rc=2）\n", len(gpoReadFails))
		for _, f := range gpoReadFails {
			fmt.Fprintf(stderr, "    · %s\n", f)
		}
		return 2
	}

	if len(bad) > 0 {
		gpoRefuseMachineFace(stdout, a.JSON, res, tier, "allowlist_invalid", bad)
		fmt.Fprintf(stdout, "⛔ 不给结论（rc=2）：白名单例外登记不合格 —— %d 条：\n", len(bad))
		for _, b := range bad {
			fmt.Fprintf(stdout, "    · %s\n", b)
		}
		fmt.Fprintf(stdout, "  扫面 %d 件 · 命中 **%d** 条 · 白名单例外命中 **%d** 条\n",
			len(res.Scanned), len(res.Hits), len(res.Exceptions))
		return 2
	}

	if len(res.Scanned) == 0 {
		gpoRefuseMachineFace(stdout, a.JSON, res, tier, "empty_scan", nil)
		io.WriteString(stdout, "⛔ 不给结论（rc=2）：扫面 0 件（抽取器被改坏 / 形态变了 ⇒ 空转 = 假绿）\n")
		return 2
	}

	hits, exc := res.Hits, res.Exceptions
	fmt.Fprintf(stdout, "件名统一出口的调用点判据（C-6）· v%s · 档位 **%s**\n", gpoVersion, tier)
	fmt.Fprintf(stdout, "  扫面 %d 件（os.walk 列件 · 不经 git ⇒ 对 CJK 名天生不瞎）· 跳过 %d 件\n",
		len(res.Scanned), len(res.Skipped))
	if len(res.Skipped) > 0 {
		n := gpoMin(5, len(res.Skipped))
		for _, s := range res.Skipped[:n] {
			fmt.Fprintf(stdout, "      · 跳过 %s（%s）\n", s[0], s[1])
		}
	}
	fmt.Fprintf(stdout, "  命中 **%d** 条 · 白名单例外命中 **%d** 条（例外域：%s）\n",
		len(hits), len(exc), gpoOutletPrefix)
	n := gpoMin(10, len(hits))
	for _, h := range hits[:n] {
		fmt.Fprintf(stdout, "    ✗ %s:%d  [%s] %s\n", h.File, h.Line, h.Rule, h.Why)
		fmt.Fprintf(stdout, "        %s\n", h.Snippet)
	}
	if len(hits) > 10 {
		fmt.Fprintf(stdout, "    …（还有 %d 条，见 --json full）\n", len(hits)-10)
	}
	ne := gpoMin(10, len(exc))
	for _, e := range exc[:ne] {
		fmt.Fprintf(stdout, "    ○ 例外 %s:%d  [%s] reason=%s\n", e.File, e.Line, e.Rule, gpoTruncRunes(e.Reason, 60))
	}
	for _, nj := range gpoNotJudged {
		fmt.Fprintf(stdout, "    ○ 未判（点名登记 · 不判）：%s —— %s\n", nj[0], nj[1])
	}

	if a.JSON != "" {
		io.WriteString(stdout, gpoJSONNormal(res, tier)+"\n")
	}

	if len(hits) > 0 && tier == "rc" {
		fmt.Fprintf(stdout, "⇒ 判红（rc=1 · rc 档）：%d 条裸取件名调用点 ⇒ 转义形态会静默进判定\n", len(hits))
		return 1
	}
	if len(hits) > 0 {
		fmt.Fprintf(stdout, "⇒ 判到了（rc=0 · **report 档**）：%d 条 —— 不阻断；升档用 `--tier rc`\n", len(hits))
		return 0
	}
	fmt.Fprintf(stdout, "⇒ 全绿（rc=0）：%d 件里没有裸取件名调用点\n", len(res.Scanned))
	return 0
}
