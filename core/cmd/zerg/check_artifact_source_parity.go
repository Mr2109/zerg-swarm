// check_artifact_source_parity.go —— 判据件 `scripts/gates/check-artifact-source-parity.py`（476 行）的 **Go 真搬**
// （M2 · 甲案 C 类第 17 枚 · 入口 `checkArtifactSourceParityGateMain`）。
//
// 搬什么：门74「制品 ↔ 源码两代分离」只读判据 —— 盘上那件**制品**（缺省 `bin/zerg`）与**现读源码树**
//   是不是同一代。三档退码（与 `precommit-gates.sh` 的 tri 档逐格同义）：
//     0 = 同源 · 1 = 分离（逐条点名分在哪）· 2 = 不给结论（制品不在盘 / 跑不起来 / 自报面认不出 /
//     HEAD 取不到 / 输入面空转）。判据各只有一个定义处，都不写死时点读数。
//
// 三个面（与 py 逐字同构）：
//   面A 提交面：制品自报 commit ⟷ `git rev-parse HEAD`（真源 `scripts/build/build-all.sh:111`）。
//   面B 时点面：制品自报 build_time ⟷ 制品输入面最后一次改动 mtime。
//   面C 脏标记：只登记不判红（dirty 是建构常态）。
//   ★ 面A 的「sha 落后」本身不判红 —— 只有面B（建构之后输入面又改过）才是真分离。
//
// ★ 外部交互点（现读逐点列，全部锁死输入）：
//   ① 起子进程 `<制品> --version`（对应 py `subprocess.run`）：只取自报身份一行；env 剥掉
//      ZERG_STATE_DIR / ZERG_REPO / ZERG_PROPOSAL_DIR（与 py 同）；超时 15s。
//   ② 起子进程 `git -C <root> …`：rev-parse HEAD / rev-parse --short HEAD / status --porcelain /
//      cat-file -e / merge-base --is-ancestor / rev-list --count；超时 30s。
//   ③ 起子进程 `git`（自检里 init/add/commit，cwd = 夹具目录）。
//   ④ 起子进程 = 夹具里的假制品 shell 脚本（自检专用）。
//   ⇒ 本件属**制品/子进程类**：输入确定（锁定制品 sha256 + 锁定 git 摆位）时输出逐字可对拍。
//
// ★ 遍历面与输出序（逐点交代）：
//   ① 遍历 = 对 `core` / `shared` 两棵根做 `os.walk`（topdown · followlinks=False）。
//      Go 侧 `os.ReadDir` / `filepath.WalkDir` **会排序** ⇒ 本件用 `f.ReadDir(-1)` 保 readdir 原序，
//      且**先产完当前层文件、再按原序逐子目录下钻**（复刻 os.walk 的产出序）。
//   ② 目录分类用 `os.Stat`（跟随符号链接 = py `entry.is_dir()`）；**指向目录的符号链接不下钻**
//      （followlinks=False），但它会进 dirnames 参与剪枝。
//   ③ 剪枝：目录名 ∈ {.git, __pycache__, node_modules, target, testdata} 或以 `.` / `_` 开头 ⇒ 不进；
//      文件名以 `.` / `_` 开头或形如 `*_test.go` ⇒ 不进。
//   ④ ★ **输出行序的决定者 = py 第 151 行 `files = sorted(set(files) | set(embeds))`**
//      （显式排序，**不是**遍历序）⇒ 本件用 `sort.Strings`（UTF-8 字节序 == py 码点序）。
//      `lagging` 再按 mtime **降序**（py `sorted(..., key=mtime, reverse=True)` · 稳定 ⇒ 同刻保持 face 序）；
//      `newest_input` 取 `max(..., key=mtime)`（同刻取 face 序**最先**那件）。⇒ 本件**零 Go map 迭代进输出**。
//
// ★ 出厂声明行：
//   (a) **默认根定位差异**：py 从 `__file__` 上溯两层得仓根 ⟷ Go 无 `__file__` 语境、只能从 cwd 上溯
//       （取最近的含 `.git` 的祖先）⇒ 「脚本位置 ≠ cwd」时路径推导面不同，✗ 不声称逐字对齐。
//   (b) **自指检查**：本件落在**它自己的扫描面内**（`core/**/*.go` 是制品输入面的一部分）。
//       处理 = 全文**零物理行**以自门嵌入口径的指令原串起头（该口径一律描述式写，✗ 不落原串）⇒
//       自门现读本件时**不新增任何 embed 资产**（命中数 0）。落件前后自门对扫描面的读数逐字对照见回执。
//
// ★ 本件声明的口径差 / 未覆盖面见文件末「未覆盖面」注释块。

package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const aspTool = "check-artifact-source-parity.py"
const aspToolVersion = "v1.0.0"
const aspProg = "check-artifact-source-parity.py"

const aspRCOK, aspRCFail, aspRCBlocked = 0, 1, 2

var aspRCName = map[int]string{0: "OK（同源）", 1: "FAIL（两代分离）", 2: "BLOCKED（不给结论）"}

var aspInputRoots = []string{"core", "shared"}
var aspGoModFiles = []string{"go.mod", "go.sum"}
var aspSkipDirNames = map[string]bool{
	".git": true, "__pycache__": true, "node_modules": true, "target": true, "testdata": true,
}

const aspVersionTimeout = 15 // 秒（只跑 --version）
const aspGitTimeout = 30     // 秒（与 py 的 git 子进程超时同）

// 制品自报面（`version.Line`）：<件名> <版本> <commit>[+dirty] <build_time>
var aspIDRE = regexp.MustCompile("(?P<comp>\\S+)\\s+(?P<ver>\\S+)\\s+(?P<commit>[0-9a-fA-F]{7,40})(?P<dirty>\\+dirty)?\\s+(?P<build>\\d{4}-\\d{2}-\\d{2}T\\d{2}:\\d{2}:\\d{2}Z)\\s*$")
var aspEmbedRE = regexp.MustCompile("^\\s*//go:embed\\s+(?P<pat>.+?)\\s*$")

var aspEnvStrip = []string{"ZERG_STATE_DIR", "ZERG_REPO", "ZERG_PROPOSAL_DIR"}

const aspUsageText = "usage: check-artifact-source-parity.py [-h] [--root ROOT]\n" +
	"                                       [--artifact ARTIFACT] [--json]\n" +
	"                                       [--self-test] [--list-rules]\n"

const aspHelpText = aspUsageText + "\n" +
	"optional arguments:\n" +
	"  -h, --help           show this help message and exit\n" +
	"  --root ROOT\n" +
	"  --artifact ARTIFACT\n" +
	"  --json\n" +
	"  --self-test\n" +
	"  --list-rules\n"

const aspRulesText = "门74 制品↔源码两代分离 · 规则表（check-artifact-source-parity.py v1.0.0）\n" +
	"判据面A 提交面：制品 `--version` 自报 commit ⟷ `git rev-parse HEAD`\n" +
	"  · 真源 scripts/build/build-all.sh:111（SHA=$(git rev-parse --short HEAD)）\n" +
	"判据面B 时点面：制品自报 build_time ⟷ 制品输入面最后一次改动 mtime\n" +
	"  输入面 = core/shared 下非测试 *.go + go.mod/go.sum + 源码 //go:embed 点到的资产（现读，不写死）\n" +
	"判据面C 脏标记：只登记不判红（dirty 是常态）\n" +
	"退码：0 同源 · 1 分离（逐条点名）· 2 不给结论（制品不在盘/跑不起来/自报面认不出/HEAD 取不到/输入面空转）\n" +
	"只读：不写任何件 · 不重编 · 不跑 go build / go test · 不读状态目录\n"

// ═══════════════════════ py 路径语义家族（自写同构 · 见 references/porting-standards） ═══════════════════════

// aspPyJoin 复刻 posixpath.join —— 逐段规则直译（✗ 不用 filepath.Join：`..`/绝对段/空段三态会差）。
func aspPyJoin(parts ...string) string {
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

// aspPyNormpath 复刻 posixpath.normpath（保留恰两个前导斜杠的特例）。
func aspPyNormpath(path string) string {
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

// aspPySplit 复刻 posixpath.split（head 除纯斜杠外剥尾斜杠）。
func aspPySplit(p string) (string, string) {
	i := strings.LastIndex(p, "/") + 1
	head, tail := p[:i], p[i:]
	if head != "" && head != strings.Repeat("/", len(head)) {
		head = strings.TrimRight(head, "/")
	}
	return head, tail
}

// aspPyDirname 复刻 posixpath.dirname。
func aspPyDirname(p string) string {
	h, _ := aspPySplit(p)
	return h
}

// aspPyAbspath 复刻 posixpath.abspath：纯词法（cwd 由调用方传入 · ✗ 不在函数里 os.Getwd）。
func aspPyAbspath(cwd, path string) string {
	return aspPyNormpath(aspPyJoin(cwd, path))
}

// aspPyRelpath 复刻 posixpath.relpath（start 传 cwd）。
func aspPyRelpath(cwd, path, start string) string {
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
	s, p := split(aspPyAbspath(cwd, start)), split(aspPyAbspath(cwd, path))
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

// aspFnmatchCase 复刻 fnmatch.fnmatchcase：`*` / `?` **跨 /`** · 大小写敏感（POSIX）。
// ★ py 还认 `[seq]` / `[!seq]`：本件**一并实现**（真仓现读 embed pattern 全为字面件名 ⇒ 不触发，见未覆盖面）。
func aspFnmatchCase(name, pat string) bool {
	return aspFnmatch([]rune(name), []rune(pat))
}

func aspFnmatch(n, p []rune) bool {
	for len(p) > 0 {
		switch p[0] {
		case '*':
			for len(p) > 0 && p[0] == '*' {
				p = p[1:]
			}
			if len(p) == 0 {
				return true
			}
			for i := 0; i <= len(n); i++ {
				if aspFnmatch(n[i:], p) {
					return true
				}
			}
			return false
		case '?':
			if len(n) == 0 {
				return false
			}
			n, p = n[1:], p[1:]
		case '[':
			if len(n) == 0 {
				return false
			}
			matched, rest, ok := aspMatchClass(n[0], p)
			if !ok { // 未闭合的 `[` ⇒ 按字面
				if n[0] != '[' {
					return false
				}
				n, p = n[1:], p[1:]
				continue
			}
			if !matched {
				return false
			}
			n, p = n[1:], rest
		default:
			if len(n) == 0 || n[0] != p[0] {
				return false
			}
			n, p = n[1:], p[1:]
		}
	}
	return len(n) == 0
}

// aspMatchClass 解析 `[...]`（支持 `!`/`^` 取反与 a-z 区间）。ok=false 表示未闭合。
func aspMatchClass(c rune, p []rune) (bool, []rune, bool) {
	i := 1
	neg := false
	if i < len(p) && (p[i] == '!' || p[i] == '^') {
		neg = true
		i++
	}
	matched := false
	if i < len(p) && p[i] == ']' { // `]` 紧邻开头 ⇒ 字面成员
		if c == ']' {
			matched = true
		}
		i++
	}
	for i < len(p) && p[i] != ']' {
		lo := p[i]
		if i+2 < len(p) && p[i+1] == '-' && p[i+2] != ']' {
			hi := p[i+2]
			if c >= lo && c <= hi {
				matched = true
			}
			i += 3
			continue
		}
		if c == lo {
			matched = true
		}
		i++
	}
	if i >= len(p) {
		return false, nil, false
	}
	return matched != neg, p[i+1:], true
}

// aspHasMagic 复刻 glob 的 magic 判定。
func aspHasMagic(s string) bool {
	return strings.ContainsAny(s, "*?[")
}

// aspScandirNames 保 readdir 原序取目录项名（读不了 ⇒ 空，对应 py glob 的 OSError⇒[]）。
func aspScandirNames(dir string) []string {
	target := dir
	if target == "" {
		target = "."
	}
	f, err := os.Open(target)
	if err != nil {
		return nil
	}
	ents, err := f.ReadDir(-1)
	_ = f.Close()
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(ents))
	for _, e := range ents {
		names = append(names, e.Name())
	}
	return names
}

// aspGlob 复刻 glob.glob（逐段展开 · 非递归 · 隐藏项仅在 pattern 段以 `.` 开头时参与）。
func aspGlob(pattern string) []string {
	if !aspHasMagic(pattern) {
		if _, err := os.Lstat(pattern); err == nil {
			return []string{pattern}
		}
		return nil
	}
	abs := strings.HasPrefix(pattern, "/")
	segs := strings.Split(pattern, "/")
	if abs {
		segs = segs[1:]
	}
	dirs := []string{""}
	if abs {
		dirs = []string{"/"}
	}
	for _, seg := range segs {
		var next []string
		for _, d := range dirs {
			if !aspHasMagic(seg) {
				p := aspPyJoin(d, seg)
				if _, err := os.Lstat(p); err == nil {
					next = append(next, p)
				}
				continue
			}
			hiddenPat := strings.HasPrefix(seg, ".")
			for _, nm := range aspScandirNames(d) {
				if !hiddenPat && strings.HasPrefix(nm, ".") {
					continue
				}
				if aspFnmatchCase(nm, seg) {
					next = append(next, aspPyJoin(d, nm))
				}
			}
		}
		dirs = next
	}
	return dirs
}

// ═══════════════════════ py 字符串原语（逐处现读） ═══════════════════════

// aspIsPySpace 复刻 py `str.strip()` 的空白集（含 \x1c-\x1f；Go unicode.IsSpace 不含这四个）。
func aspIsPySpace(r rune) bool {
	switch r {
	case 0x1c, 0x1d, 0x1e, 0x1f:
		return true
	}
	return unicode.IsSpace(r)
}

// aspPyStrip 复刻 py `str.strip()`。
func aspPyStrip(s string) string {
	return strings.TrimFunc(s, aspIsPySpace)
}

func aspIsPyDigit(r rune) bool { return unicode.IsDigit(r) }

// aspSplitLines 复刻 py `str.splitlines()`（含 \v \f \x1c-\x1e \x85 \u2028 \u2029 边界）。
func aspSplitLines(s string) []string {
	var out []string
	rs := []rune(s)
	start := 0
	i := 0
	for i < len(rs) {
		c := rs[i]
		boundary := false
		switch c {
		case '\n', '\v', '\f', 0x1c, 0x1d, 0x1e, 0x85, 0x2028, 0x2029:
			boundary = true
		case '\r':
			boundary = true
			if i+1 < len(rs) && rs[i+1] == '\n' {
				i++
			}
		}
		if boundary {
			out = append(out, string(rs[start:i]))
			i++
			start = i
			continue
		}
		i++
	}
	if start < len(rs) {
		out = append(out, string(rs[start:]))
	}
	return out
}

// aspIterLines 复刻 py 文本模式逐行迭代（universal newlines：\n / \r / \r\n；不切 \v \f 等）。
func aspIterLines(s string) []string {
	var out []string
	start := 0
	i := 0
	for i < len(s) {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			i++
			start = i
		} else if s[i] == '\r' {
			out = append(out, s[start:i])
			i++
			if i < len(s) && s[i] == '\n' {
				i++
			}
			start = i
		} else {
			i++
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

func aspCountContains(lines []string, sub string) int {
	n := 0
	for _, ln := range lines {
		if strings.Contains(ln, sub) {
			n++
		}
	}
	return n
}

func aspPyListJoin(items []string, sep string) string { return strings.Join(items, sep) }

// ═══════════════════════ 有序 JSON（复刻 `json.dumps(data, ensure_ascii=False, indent=2)`） ═══════════════════════

type aspKV struct {
	K string
	V any
}
type aspDict []aspKV

func aspJSONDump(d aspDict) string {
	var b strings.Builder
	aspJSONWrite(&b, d, 0)
	return b.String()
}

func aspJSONWrite(b *strings.Builder, v any, lvl int) {
	switch t := v.(type) {
	case nil:
		b.WriteString("null")
	case bool:
		if t {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	case int:
		b.WriteString(strconv.Itoa(t))
	case string:
		b.WriteString(aspJSONStr(t))
	case aspDict:
		aspJSONWriteDict(b, t, lvl)
	case []any:
		aspJSONWriteList(b, t, lvl)
	default:
		b.WriteString("null")
	}
}

func aspJSONWriteDict(b *strings.Builder, d aspDict, lvl int) {
	if len(d) == 0 {
		b.WriteString("{}")
		return
	}
	ind := strings.Repeat("  ", lvl+1)
	closeInd := strings.Repeat("  ", lvl)
	b.WriteString("{\n")
	for i, kv := range d {
		b.WriteString(ind)
		b.WriteString(aspJSONStr(kv.K))
		b.WriteString(": ")
		aspJSONWrite(b, kv.V, lvl+1)
		if i < len(d)-1 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	b.WriteString(closeInd + "}")
}

func aspJSONWriteList(b *strings.Builder, l []any, lvl int) {
	if len(l) == 0 {
		b.WriteString("[]")
		return
	}
	ind := strings.Repeat("  ", lvl+1)
	closeInd := strings.Repeat("  ", lvl)
	b.WriteString("[\n")
	for i, e := range l {
		b.WriteString(ind)
		aspJSONWrite(b, e, lvl+1)
		if i < len(l)-1 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	b.WriteString(closeInd + "]")
}

func aspJSONStr(s string) string {
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
				b.WriteString(fmt.Sprintf("\\u%04x", r))
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

// ═══════════════════════ 取数面 ═══════════════════════

type aspIdent struct {
	commit    string
	commitRaw string
	dirty     bool
	version   string
	component string
	buildSec  int64
	buildTime string
}

func aspIsExecutable(p string) bool {
	fi, err := os.Stat(p)
	if err != nil {
		return false
	}
	if fi.IsDir() { // py os.access(dir, X_OK) 通常为真
		return true
	}
	return fi.Mode().Perm()&0o111 != 0
}

func aspStderrSuffix(se string) string {
	if aspPyStrip(se) != "" {
		return "\n" + se
	}
	return ""
}

func aspStrippedEnv() []string {
	var out []string
	for _, kv := range os.Environ() {
		k := kv
		if i := strings.IndexByte(kv, '='); i >= 0 {
			k = kv[:i]
		}
		strip := false
		for _, s := range aspEnvStrip {
			if k == s {
				strip = true
				break
			}
		}
		if !strip {
			out = append(out, kv)
		}
	}
	return out
}

// aspReadArtifactIdentity 对应 py `read_artifact_identity` ⇒ (ident, 失败原因, 原始输出)。
func aspReadArtifactIdentity(art string) (*aspIdent, string, string) {
	if _, err := os.Stat(art); err != nil {
		return nil, fmt.Sprintf("制品不在盘：%s", art), ""
	}
	if !aspIsExecutable(art) {
		return nil, fmt.Sprintf("制品不可执行（不是可跑的一件）：%s", art), ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), aspVersionTimeout*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, art, "--version")
	cmd.Env = aspStrippedEnv()
	var so, se bytes.Buffer
	cmd.Stdout = &so
	cmd.Stderr = &se
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return nil, fmt.Sprintf("跑 `--version` 超时（>%ds）", aspVersionTimeout), ""
	}
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			out := so.String() + aspStderrSuffix(se.String())
			return nil, fmt.Sprintf("`--version` rc=%d", ee.ExitCode()), aspPyStrip(out)
		}
		return nil, fmt.Sprintf("跑不起来：%s", err), ""
	}
	out := so.String() + aspStderrSuffix(se.String())
	for _, line := range aspSplitLines(so.String()) {
		m := aspIDRE.FindStringSubmatch(aspPyStrip(line))
		if m == nil {
			continue
		}
		bt, perr := time.Parse("2006-01-02T15:04:05Z", m[5])
		if perr != nil {
			// py 此处裸 strptime 抛 ValueError ⇒ 回溯 rc=1；本件按 C 类口径 fail-closed rc=2（见未覆盖面）。
			return nil, "自报面时刻不可解析（py 该档裸回溯 rc=1 · 本件 fail-closed）", aspPyStrip(out)
		}
		return &aspIdent{
			commit:    strings.ToLower(m[3]),
			commitRaw: m[3],
			dirty:     m[4] != "",
			version:   m[2],
			component: m[1],
			buildSec:  bt.Unix(),
			buildTime: m[5],
		}, "", aspPyStrip(out)
	}
	return nil, "自报面认不出（没有 `<件名> <版本> <commit>[+dirty] <ISO8601Z>` 那样一行）", aspPyStrip(out)
}

// aspGit 对应 py `git(root, *args)`（rc≠0 / OSError / 超时 ⇒ (\"\", false)）。
func aspGit(root string, args ...string) (string, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), aspGitTimeout*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", root}, args...)...)
	var so bytes.Buffer
	cmd.Stdout = &so
	cmd.Stderr = &bytes.Buffer{}
	if err := cmd.Run(); err != nil {
		return "", false
	}
	return strings.TrimSpace(so.String()), true
}

type aspFileMT struct {
	path string
	mt   float64
}

// aspWalkPy 复刻 os.walk(topdown=True, followlinks=False) 的产出序 + py 的剪枝。
func aspWalkPy(top string, visit func(string, []string)) {
	f, err := os.Open(top)
	if err != nil {
		return // os.walk onerror=None ⇒ 读不了的目录静默跳过（B 类目录面 · 逐字照搬）
	}
	ents, err := f.ReadDir(-1)
	_ = f.Close()
	if err != nil {
		return
	}
	var fileNames []string
	var descend []string
	for _, e := range ents {
		p := aspPyJoin(top, e.Name())
		fi, serr := os.Stat(p) // 跟随符号链接 = py entry.is_dir()
		isDir := serr == nil && fi.IsDir()
		if isDir {
			if li, lerr := os.Lstat(p); lerr != nil || li.Mode()&os.ModeSymlink == 0 {
				descend = append(descend, e.Name()) // followlinks=False ⇒ 符号链接目录不下钻
			}
			continue
		}
		fileNames = append(fileNames, e.Name())
	}
	visit(top, fileNames)
	for _, d := range descend {
		if aspSkipDirNames[d] {
			continue
		}
		if strings.HasPrefix(d, ".") || strings.HasPrefix(d, "_") {
			continue
		}
		aspWalkPy(aspPyJoin(top, d), visit)
	}
}

// aspCollectInputFace 对应 py `collect_input_face` ⇒ (face, embed 件数, 扫描面件数)。
func aspCollectInputFace(root string) ([]aspFileMT, int, int) {
	var files []string
	var embeds []string
	for _, rel := range aspInputRoots {
		base := aspPyJoin(root, rel)
		fi, err := os.Stat(base)
		if err != nil || !fi.IsDir() {
			continue
		}
		aspWalkPy(base, func(dirpath string, fnames []string) {
			for _, fn := range fnames {
				if strings.HasPrefix(fn, ".") || strings.HasPrefix(fn, "_") {
					continue
				}
				if strings.HasSuffix(fn, "_test.go") {
					continue
				}
				p := aspPyJoin(dirpath, fn)
				if strings.HasSuffix(fn, ".go") {
					files = append(files, p)
					data, rerr := os.ReadFile(p)
					if rerr != nil {
						continue // B 类：读失败静默（py `except OSError: pass` · 逐字照搬）
					}
					for _, line := range aspIterLines(string(data)) {
						m := aspEmbedRE.FindStringSubmatch(line)
						if m == nil {
							continue
						}
						for _, tok := range strings.Fields(m[1]) {
							for _, hit := range aspGlob(aspPyJoin(dirpath, tok)) {
								if st, serr2 := os.Stat(hit); serr2 == nil && !st.IsDir() {
									embeds = append(embeds, hit)
								}
							}
						}
					}
				} else if fn == "go.mod" || fn == "go.sum" {
					files = append(files, p)
				}
			}
		})
	}
	uniq := map[string]bool{}
	for _, p := range files {
		uniq[p] = true
	}
	for _, p := range embeds {
		uniq[p] = true
	}
	paths := make([]string, 0, len(uniq))
	for p := range uniq {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	var face []aspFileMT
	for _, p := range paths {
		if fi, err := os.Stat(p); err == nil { // B 类：os.stat 失败静默跳过
			face = append(face, aspFileMT{path: p, mt: aspMtimeFloat(fi)})
		}
	}
	emb := map[string]bool{}
	for _, p := range embeds {
		emb[p] = true
	}
	return face, len(emb), len(paths)
}

func aspMtimeFloat(fi os.FileInfo) float64 {
	mt := fi.ModTime()
	return float64(mt.Unix()) + float64(mt.Nanosecond())/1e9
}

func aspISO(epoch float64) string {
	sec := int64(epoch)
	nsec := int64((epoch - float64(sec)) * 1e9)
	return time.Unix(sec, nsec).UTC().Format("2006-01-02T15:04:05Z")
}

func aspLocal(epoch int64) string {
	return time.Unix(epoch, 0).Local().Format("2006-01-02 15:04:05")
}

// ═══════════════════════ 主判据 ═══════════════════════

// aspJudge 对应 py `judge`（不打印；⇒ rc / lines / data）。data 保 py 的插入序。
func aspJudge(root, art string) (int, []string, aspDict) {
	cwd, _ := os.Getwd()
	root = aspPyAbspath(cwd, root)
	art = aspPyAbspath(cwd, art)
	var lines []string
	data := aspDict{{K: "root", V: root}, {K: "artifact", V: art}}
	add := func(k string, v any) { data = append(data, aspKV{K: k, V: v}) }

	lines = append(lines, "门74 制品↔源码两代分离（只读 · 现读对拍）")
	lines = append(lines, fmt.Sprintf("  制品侧 %s", art))

	ident, errMsg, raw := aspReadArtifactIdentity(art)
	if ident == nil {
		lines = append(lines, fmt.Sprintf("  ✗ 2 不给结论：%s", errMsg))
		if raw != "" {
			lines = append(lines, fmt.Sprintf("    制品自报原文：%s", strings.ReplaceAll(raw, "\n", " ⏎ ")))
		}
		lines = append(lines, "    ★ 「取不到数」不当绿 —— 无制品 / 认不出 ⇒ BLOCKED")
		add("reason", errMsg)
		return aspRCBlocked, lines, data
	}

	add("artifact_commit", ident.commit)
	add("artifact_dirty", ident.dirty)
	add("artifact_version", ident.version)
	add("artifact_build", ident.buildTime)
	dirtyMark := ""
	if ident.dirty {
		dirtyMark = "+dirty"
	}
	lines = append(lines, fmt.Sprintf("  制品侧：版本 %s · 代码 %s%s · 构建 %s（现跑 `--version` 取）",
		ident.version, ident.commitRaw, dirtyMark, ident.buildTime))

	headFull, okHead := aspGit(root, "rev-parse", "HEAD")
	if !okHead || headFull == "" {
		lines = append(lines, fmt.Sprintf("  ✗ 2 不给结论：源码侧 HEAD 取不到（%s 不是 git 仓 / git 不可用）", root))
		add("reason", "HEAD unavailable")
		return aspRCBlocked, lines, data
	}
	headShortRaw, okShort := aspGit(root, "rev-parse", "--short", "HEAD")
	headShort := headShortRaw
	if !okShort || headShort == "" {
		if len(headFull) >= 7 {
			headShort = headFull[:7]
		} else {
			headShort = headFull
		}
	}
	status, _ := aspGit(root, "status", "--porcelain")
	dirtyCount := 0
	for _, x := range aspSplitLines(status) {
		if aspPyStrip(x) != "" {
			dirtyCount++
		}
	}
	add("head_full", headFull)
	add("head_short", headShort)
	add("tree_dirty", dirtyCount)
	lines = append(lines, fmt.Sprintf("  源码侧 %s：HEAD %s（短 %s）· 工作树未提交 %d 件",
		root, headFull, headShort, dirtyCount))

	face, embedN, _ := aspCollectInputFace(root)
	if len(face) == 0 {
		lines = append(lines, fmt.Sprintf("  ✗ 2 不给结论：制品输入面空转（%s 下 0 件可读 —— 空转不许当绿）",
			aspPyListJoin(aspInputRoots, " / ")))
		add("reason", "input face empty")
		return aspRCBlocked, lines, data
	}
	newest := face[0]
	for _, fm := range face[1:] {
		if fm.mt > newest.mt {
			newest = fm
		}
	}
	add("input_files", len(face))
	add("embed_files", embedN)
	add("newest_input", aspPyRelpath("", newest.path, root))
	add("newest_input_mtime", aspISO(newest.mt))
	lines = append(lines, fmt.Sprintf("  源码侧制品输入面：%d 件（含 go:embed 资产 %d 件）· 最后一次改动 %s  %s",
		len(face), embedN, aspISO(newest.mt), aspPyRelpath("", newest.path, root)))

	// ── 面A 提交面 ──
	artCommit := ident.commit
	same := strings.HasPrefix(headFull, artCommit) || strings.HasPrefix(headShort, artCommit) ||
		strings.HasPrefix(artCommit, headShort)
	_, known := aspGit(root, "cat-file", "-e", artCommit)
	var behind *int
	var faceA string
	if same {
		faceA = "同一笔（制品自报 sha 就是现读 HEAD）"
	} else if known {
		if _, anc := aspGit(root, "merge-base", "--is-ancestor", artCommit, "HEAD"); anc {
			cntS, _ := aspGit(root, "rev-list", "--count", artCommit+"..HEAD")
			if cntS != "" && aspAllASCIIDigits(cntS) {
				if n, cerr := strconv.Atoi(cntS); cerr == nil {
					behind = &n
				}
			}
			bStr := "?"
			if behind != nil {
				bStr = strconv.Itoa(*behind)
			}
			faceA = fmt.Sprintf("制品那一代是 HEAD 的祖先 · 落后 %s 笔（建构之后又落了提交）", bStr)
		} else {
			faceA = "**分叉**：制品那一代不是现读 HEAD 的祖先（两条线）"
		}
	} else {
		faceA = "**制品那一代不在本仓对象库**（sha 不是本仓任何一代）"
	}
	add("face_a", faceA)
	if behind != nil {
		add("behind", *behind)
	} else {
		add("behind", nil)
	}
	add("ancestor_known", known)

	// ── 面B 时点面 ──
	buildEpoch := float64(ident.buildSec)
	var lagging []aspFileMT
	for _, fm := range face {
		if fm.mt > buildEpoch {
			lagging = append(lagging, fm)
		}
	}
	sort.SliceStable(lagging, func(i, j int) bool { return lagging[i].mt > lagging[j].mt })
	add("lagging_count", len(lagging))
	lagList := []any{}
	for i, fm := range lagging {
		if i >= 10 {
			break
		}
		lagList = append(lagList, aspDict{
			{K: "file", V: aspPyRelpath("", fm.path, root)},
			{K: "mtime", V: aspISO(fm.mt)},
		})
	}
	add("lagging", lagList)
	faceBLag := len(lagging) > 0

	lines = append(lines, fmt.Sprintf("  面A 提交面：%s", faceA))
	early := "不早于"
	concl := "制品已包含现读输入面"
	if faceBLag {
		early = "早于"
		concl = "**落后**"
	}
	lines = append(lines, fmt.Sprintf("  面B 时点面：制品建构 %s（%s）%s 输入面最后一次改动 %s  ⇒ %s",
		ident.buildTime, aspLocal(ident.buildSec), early, aspISO(newest.mt), concl))
	lines = append(lines, fmt.Sprintf("  面C 脏标记：制品自报 %s —— dirty 是常态（建构时工作树有未提交改动），"+
		"**本件不因 dirty 判红**", func() string {
		if ident.dirty {
			return "+dirty"
		}
		return "无 dirty"
	}()))

	rc := aspRCOK
	if faceBLag {
		rc = aspRCFail
		lines = append(lines, "  ✗ 1 **分离（落后）**：制品建构之后，下列制品输入面又改过 —— 这些改动没进制品：")
		for i, fm := range lagging {
			if i >= 10 {
				break
			}
			lines = append(lines, fmt.Sprintf("    · %s（源码侧 %s ＞ 制品侧 %s）",
				aspPyRelpath("", fm.path, root), aspISO(fm.mt), ident.buildTime))
		}
		if len(lagging) > 10 {
			lines = append(lines, fmt.Sprintf("    · …另 %d 件（同一判据，此处只列前 10）", len(lagging)-10))
		}
		lines = append(lines, "    ⇒ 治法：`bash scripts/build/build-all.sh`（**全量档**）重编（旧件自动留档入 bin/_history/）")
	} else if !known {
		rc = aspRCBlocked
		lines = append(lines, "  ✗ 2 不给结论：面A 说「制品那一代不在本仓对象库」⇒ 两代关系判不了（不猜、不当绿）")
	} else if behind != nil && *behind > 0 {
		lines = append(lines, fmt.Sprintf("  ✓ 0 同源（提交面落后 %d 笔是**常态**：先改 → 重编 → 再提交，制品已含 HEAD 的内容；"+
			"判据落在面B）", *behind))
	} else {
		lines = append(lines, "  ✓ 0 同源：面A 同笔 · 面B 未落后")
	}
	if rc == aspRCOK && behind == nil && !same {
		lines = append(lines, "  ✓ 0 同源：面B 未落后（判据落在面B）")
	}
	add("rc", rc)
	add("rc_name", aspRCName[rc])
	return rc, lines, data
}

func aspAllASCIIDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// ═══════════════════════ 自检（合成夹具 · 一律落临时树） ═══════════════════════

func aspRunGitIn(dir string, args ...string) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Stdout = &bytes.Buffer{}
	cmd.Stderr = &bytes.Buffer{}
	_ = cmd.Run()
}

// aspMkRepo 对应 py `_mk_repo`：合成一个真 git 仓（一筆提交 + 一个 core/*.go 输入件）。
func aspMkRepo(tmp, tag, shaBody string) string {
	r := aspPyJoin(tmp, tag)
	_ = os.MkdirAll(aspPyJoin(r, "core"), 0o755)
	_ = os.WriteFile(aspPyJoin(r, "core", "a.go"), []byte("package main\n"+shaBody+"\n"), 0o644)
	aspRunGitIn(r, "init", "-q")
	aspRunGitIn(r, "-c", "user.email=s@x", "-c", "user.name=s", "add", "-A")
	aspRunGitIn(r, "-c", "user.email=s@x", "-c", "user.name=s", "commit", "-qm", "x")
	return r
}

// aspFakeArtifact 对应 py `_fake_artifact`：写一个自报身份的假制品 shell 脚本。
func aspFakeArtifact(root, name, commit, buildISO string, dirty bool) string {
	p := aspPyJoin(root, name)
	d := ""
	if dirty {
		d = "+dirty"
	}
	content := "#!/bin/sh\necho \"zerg 2.5.12 " + commit + d + " " + buildISO + "\"\n"
	_ = os.WriteFile(p, []byte(content), 0o755)
	return p
}

// aspSetMtime 对应 py `_set_mtime`：把件 mtime 钉到指定 ISO 时刻。
func aspSetMtime(path, iso string) {
	t, err := time.Parse("2006-01-02T15:04:05Z", iso)
	if err != nil {
		return
	}
	_ = os.Chtimes(path, t, t)
}

func aspFirst16(s string) string {
	if len(s) > 16 {
		return s[:16]
	}
	return s
}

func aspDictGetInt(d aspDict, k string) int {
	for _, kv := range d {
		if kv.K == k {
			if n, ok := kv.V.(int); ok {
				return n
			}
		}
	}
	return -999999
}

func aspDictHas(d aspDict, k string) bool {
	for _, kv := range d {
		if kv.K == k {
			return true
		}
	}
	return false
}

func aspRepr(v any) string {
	switch t := v.(type) {
	case nil:
		return "None"
	case bool:
		if t {
			return "True"
		}
		return "False"
	case int:
		return strconv.Itoa(t)
	case string:
		return "'" + strings.ReplaceAll(strings.ReplaceAll(t, "\\", "\\\\"), "'", "\\'") + "'"
	}
	return fmt.Sprintf("%v", v)
}

func aspAnyEq(a, b any) bool {
	switch x := a.(type) {
	case int:
		y, ok := b.(int)
		return ok && x == y
	case bool:
		y, ok := b.(bool)
		return ok && x == y
	case string:
		y, ok := b.(string)
		return ok && x == y
	case nil:
		return b == nil
	}
	return false
}

// aspSelfTest 对应 py `run_self_test`（合成夹具 · 真起 git 子进程 · 打印逐字对齐）。
func aspSelfTest(stdout, stderr io.Writer) int {
	total, bad := 0, 0
	chk := func(name string, got, want any) {
		total++
		if !aspAnyEq(got, want) {
			bad++
			fmt.Fprintf(stdout, "  ✗ 断言失败：%s —— 得到 %s，期望 %s\n", name, aspRepr(got), aspRepr(want))
		}
	}
	fmt.Fprint(stdout, "门74 制品↔源码两代分离 · 自检（合成夹具 · 临时树）\n")
	tmp, err := os.MkdirTemp("", "g74-selftest-")
	if err != nil {
		fmt.Fprint(stdout, "自检跑不了：造不出临时目录\n")
		return 1
	}
	defer func() { _ = os.RemoveAll(tmp) }()

	// ① 正控：制品 sha = HEAD 短 sha，且建构晚于输入面 ⇒ rc=0
	r1 := aspMkRepo(tmp, "same", "// v1")
	h1, _ := aspGit(r1, "rev-parse", "HEAD")
	a1 := aspFakeArtifact(r1, "art1", aspFirst16(h1), "2026-09-28T10:00:00Z", false)
	aspSetMtime(aspPyJoin(r1, "core", "a.go"), "2026-09-28T09:00:00Z")
	rc1, _, _ := aspJudge(r1, a1)
	chk("① 制品 sha = HEAD 且建构不早于输入面 ⇒ rc=0", rc1, 0)

	// ② 分离（落后）：制品建构早于输入面最后一次改动 ⇒ rc=1 且点名那一件
	r2 := aspMkRepo(tmp, "lag", "// v1")
	h2, _ := aspGit(r2, "rev-parse", "HEAD")
	a2 := aspFakeArtifact(r2, "art2", aspFirst16(h2), "2026-09-27T00:00:00Z", false)
	aspSetMtime(aspPyJoin(r2, "core", "a.go"), "2026-09-28T09:00:00Z")
	rc2, lines2, _ := aspJudge(r2, a2)
	chk("② 制品建构早于输入面改动 ⇒ rc=1（真落后）", rc2, 1)
	chk("② 逐条点名分在哪（报的是 core/a.go）", aspCountContains(lines2, "· core/a.go（源码侧"), 1)
	chk("② 报告里同时印出两侧值（制品侧建构时刻）", aspCountContains(lines2, "2026-09-27T00:00:00Z") >= 1, true)

	// ③ 负控（换回）：同一夹具把制品换回「建构晚于输入面」⇒ 回 0
	r3 := aspMkRepo(tmp, "back", "// v1")
	h3, _ := aspGit(r3, "rev-parse", "HEAD")
	a3 := aspFakeArtifact(r3, "art3", aspFirst16(h3), "2026-09-28T10:00:00Z", false)
	aspSetMtime(aspPyJoin(r3, "core", "a.go"), "2026-09-28T09:00:00Z")
	rc3, _, _ := aspJudge(r3, a3)
	chk("③ 负控（换回）⇒ rc=0", rc3, 0)

	// ④ ★ 反误报：dirty 常态 + 提交面落后 ⇒ 必须 rc=0
	r4 := aspMkRepo(tmp, "dirty", "// v1")
	aspRunGitIn(r4, "-c", "user.email=s@x", "-c", "user.name=s", "commit", "-q", "--allow-empty", "-m", "y")
	h4old, _ := aspGit(r4, "rev-list", "--max-count=1", "HEAD~1")
	a4 := aspFakeArtifact(r4, "art4", aspFirst16(h4old), "2026-09-28T10:00:00Z", true)
	aspSetMtime(aspPyJoin(r4, "core", "a.go"), "2026-09-28T09:00:00Z")
	rc4, lines4, d4 := aspJudge(r4, a4)
	chk("④ dirty + 提交面落后 1 笔 + 建构不早于输入面 ⇒ rc=0（禁把常态判红）", rc4, 0)
	chk("④ 面C 如实登记了 dirty", aspCountContains(lines4, "面C 脏标记：制品自报 +dirty"), 1)
	chk("④ 面A 如实说明落后笔数", aspDictGetInt(d4, "behind"), 1)

	// ⑤ 取数面拿不到（制品不在盘）⇒ rc=2
	rc5, lines5, _ := aspJudge(r3, aspPyJoin(tmp, "没有这一件", "zerg"))
	chk("⑤ 制品不在盘 ⇒ rc=2（不给结论）", rc5, 2)
	chk("⑤ 报的是「制品不在盘」", aspCountContains(lines5, "制品不在盘"), 1)

	// ⑥ 制品自报面认不出 ⇒ rc=2
	badp := aspPyJoin(r3, "art-bad")
	_ = os.WriteFile(badp, []byte("#!/bin/sh\necho 我不知道\n"), 0o755)
	rc6, lines6, _ := aspJudge(r3, badp)
	chk("⑥ 自报面认不出 ⇒ rc=2", rc6, 2)
	chk("⑥ 报的是「自报面认不出」", aspCountContains(lines6, "自报面认不出"), 1)

	// ⑦ 制品那一代不在本仓对象库 ⇒ rc=2
	a7 := aspFakeArtifact(r3, "art7", "deadbee1234", "2026-09-28T10:00:00Z", false)
	rc7, _, _ := aspJudge(r3, a7)
	chk("⑦ 制品 sha 不是本仓对象 ⇒ rc=2（判不了）", rc7, 2)

	// ⑧ 输入面空转 ⇒ rc=2
	r8 := aspPyJoin(tmp, "empty")
	_ = os.MkdirAll(aspPyJoin(r8, "core"), 0o755)
	aspRunGitIn(r8, "init", "-q")
	a8 := aspFakeArtifact(r8, "art8", strings.Repeat("0", 16), "2026-09-28T10:00:00Z", false)
	rc8, _, _ := aspJudge(r8, a8)
	chk("⑧ 输入面空转 ⇒ rc=2（空转不许当绿）", rc8, 2)

	// ⑨ 测试件不算制品输入面（*_test.go 改了不许判红）
	r9 := aspMkRepo(tmp, "testonly", "// v1")
	h9, _ := aspGit(r9, "rev-parse", "HEAD")
	a9 := aspFakeArtifact(r9, "art9", aspFirst16(h9), "2026-09-28T10:00:00Z", false)
	aspSetMtime(aspPyJoin(r9, "core", "a.go"), "2026-09-28T09:00:00Z")
	tp := aspPyJoin(r9, "core", "a_test.go")
	_ = os.WriteFile(tp, []byte("package main\n"), 0o644)
	aspSetMtime(tp, "2026-09-28T11:00:00Z")
	rc9, _, _ := aspJudge(r9, a9)
	chk("⑨ 只改了 *_test.go ⇒ rc=0（测试件不进制品）", rc9, 0)

	// ⑩ 自门嵌入口径点到的资产改了要判红（资产是编译进制品的）
	r10 := aspMkRepo(tmp, "embed", "// v1")
	_ = os.WriteFile(aspPyJoin(r10, "core", "a.go"),
		[]byte("package main\n\n//go:embed asset.json\nvar raw string\n"), 0o644)
	_ = os.WriteFile(aspPyJoin(r10, "core", "asset.json"), []byte("{}\n"), 0o644)
	aspRunGitIn(r10, "-c", "user.email=s@x", "-c", "user.name=s", "add", "-A")
	aspRunGitIn(r10, "-c", "user.email=s@x", "-c", "user.name=s", "commit", "-qm", "e")
	h10, _ := aspGit(r10, "rev-parse", "HEAD")
	a10 := aspFakeArtifact(r10, "art10", aspFirst16(h10), "2026-09-28T10:00:00Z", false)
	aspSetMtime(aspPyJoin(r10, "core", "a.go"), "2026-09-28T09:00:00Z")
	aspSetMtime(aspPyJoin(r10, "core", "asset.json"), "2026-09-28T11:00:00Z")
	rc10, lines10, d10 := aspJudge(r10, a10)
	chk("⑩ go:embed 资产被现读认了出来（1 件）", aspDictGetInt(d10, "embed_files"), 1)
	chk("⑩ 资产改了（建构之后）⇒ rc=1", rc10, 1)
	chk("⑩ 点名的是 core/asset.json", aspCountContains(lines10, "· core/asset.json（源码侧"), 1)

	// ⑪ --json 面可解析且字段齐
	_, _, dj := aspJudge(r1, a1)
	chk("⑪ --json 面字段齐（rc / 制品侧 / 源码侧三格）",
		aspDictHas(dj, "rc") && aspDictHas(dj, "artifact_commit") && aspDictHas(dj, "head_full"), true)

	fmt.Fprintf(stdout, "── 自检结论：断言 %d 条 · 失败 %d 条 ──\n", total, bad)
	if bad == 0 {
		return 0
	}
	return 1
}

// ═══════════════════════ 入口 ═══════════════════════

func aspArgErr(stderr io.Writer, detail string) int {
	fmt.Fprint(stderr, aspUsageText)
	fmt.Fprintf(stderr, "%s: error: %s\n", aspProg, detail)
	return aspRCBlocked
}

// aspRepoRoot —— py 从 `__file__` 上溯两层；Go 无 __file__ ⇒ 从 cwd 上溯找最近的含 `.git` 的祖先
// （见声明行 (a) · 判据本质需要一棵 git 仓）。
func aspRepoRoot(cwd string) string {
	d := cwd
	for i := 0; i < 12; i++ {
		if _, err := os.Stat(aspPyJoin(d, ".git")); err == nil {
			return d
		}
		parent := aspPyDirname(d)
		if parent == d || parent == "" {
			break
		}
		d = parent
	}
	return cwd
}

// checkArtifactSourceParityGateMain —— 入口（= py `main()`）。
func checkArtifactSourceParityGateMain(stdout, stderr io.Writer, argv []string) int {
	root, artifact := "", ""
	haveRoot, haveArtifact := false, false
	asJSON, selfTest, listRules, wantHelp := false, false, false, false
	i := 0
	for i < len(argv) {
		a := argv[i]
		switch {
		case a == "-h" || a == "--help":
			wantHelp = true
		case a == "--root":
			if i+1 >= len(argv) {
				return aspArgErr(stderr, "argument --root: expected one argument")
			}
			i++
			root, haveRoot = argv[i], true
		case strings.HasPrefix(a, "--root="):
			root, haveRoot = a[len("--root="):], true
		case a == "--artifact":
			if i+1 >= len(argv) {
				return aspArgErr(stderr, "argument --artifact: expected one argument")
			}
			i++
			artifact, haveArtifact = argv[i], true
		case strings.HasPrefix(a, "--artifact="):
			artifact, haveArtifact = a[len("--artifact="):], true
		case a == "--json":
			asJSON = true
		case a == "--self-test":
			selfTest = true
		case a == "--list-rules":
			listRules = true
		default:
			return aspArgErr(stderr, "unrecognized arguments: "+a)
		}
		i++
	}
	if wantHelp {
		fmt.Fprint(stdout, aspHelpText)
		return aspRCOK
	}
	if listRules {
		fmt.Fprint(stdout, aspRulesText)
		return aspRCOK
	}
	if selfTest {
		return aspSelfTest(stdout, stderr)
	}

	cwd, _ := os.Getwd()
	r := root
	if !haveRoot {
		r = aspRepoRoot(cwd)
	}
	rt := aspPyAbspath(cwd, r)
	at := artifact
	if !haveArtifact {
		at = aspPyJoin(rt, "bin/zerg")
	}
	at = aspPyAbspath(cwd, at)
	rc, lines, data := aspJudge(rt, at)
	if asJSON {
		fmt.Fprintln(stdout, aspJSONDump(data))
	} else {
		for _, ln := range lines {
			fmt.Fprintln(stdout, ln)
		}
	}
	return rc
}

// ═══════════════════════ 未覆盖面（照实声明 · ✗ 不声称逐字对齐） ═══════════════════════
//
// ① argparse 帮助/用法正文：`-h` / `--help` 与用法错的**正文**按本机默认宽度逐字抄入；COLUMNS 变宽/变窄时
//    argparse 会换行不同 ⇒ 该档只在默认宽度下逐字同。用法错（未知旗标 / 缺值）的 error 行正文与 argparse 逐字对齐，
//    但 argparse 支持的**前缀缩写**（`--sel` ⇒ `--self-test`）本件**未实现** ⇒ 该形态本件 rc=2。
// ② 制品「跑不起来」档：py 打 `跑不起来：[Errno 13] Permission denied: '<p>'` ⟷ Go 打 `跑不起来：fork/exec <p>: permission denied`
//    ⇒ 该格**只对齐 rc=2 与首句**，OS 错误正文不复刻（含本机 errno 文本）。
// ③ 制品自报 build 字段越界（如 `2026-99-99T99:99:99Z` 过正则但 strptime 失败）：py 裸回溯 rc=1（C 类）⇒
//    本件 fail-closed 改 rc=2 ＋ 具名文案 ⇒ 与 py 该档 rc 不同（py 该档无正文可比）。
// ④ 正则 Unicode 语义差：py `\s`/`\S`/`\d` 默认 Unicode ⟷ Go RE2 ASCII ⇒ 制品自报行含 U+00A0 一类空白时可能分叉
//    （现役 `bin/zerg --version` 实测纯 ASCII · 不触发）；`\d{4}` 同理。
// ⑤ `str.isdigit()` 认 Unicode 数字 ⟷ 本件 `aspAllASCIIDigits` 只认 ASCII ⇒ git 返回非 ASCII 数字时该档不同
//    （git 恒出 ASCII ⇒ 不触发）。
// ⑥ 读文本时的非法 UTF-8：py `errors='replace'` 按非法**序列**替换一个 U+FFFD ⟷ Go 保留原字节 ⇒ 只在
//    非法 UTF-8 的 `.go` 件上分叉（真仓恒合法 UTF-8 ⇒ 不触发）。
// ⑦ 文件遍历的 `\v`/`\f`/`\x1c` 一类「行边界」：py 文件迭代按 universal newlines（仅 \n \r \r\n）⟷ 本件
//    `aspIterLines` 同 ⇒ 该面**已对齐**；但 `str.splitlines()` 面（制品 stdout 解析）本件用 `aspSplitLines` 全对齐。
// ⑧ 外部工具链面：本件依赖 `git`（自检 + 判据）与系统 exec ⇒ 版本面差列入未覆盖面；回执报工具链指纹。
// ⑨ B 类真口子（逐字照搬 · ✗ 不擅自改成红）：`os.walk` 读不了的**目录**静默跳过（onerror=None）·
//    读不了的 `.go` 件静默不做 embed 扫描（`except OSError: pass`）· `os.stat` 失败的件静默移出输入面
//    （`except OSError: pass`）⇒ 被跳过的件既不进「可判」也不进「判不了」。经 `check_artifact_source_parity_test.go`
//    的专测子用例把这个现状钉住（日后谁修了口子，用例立刻红）。
// ⑩ 自指面：本件落在自门扫描面（`core/**/*.go`）内 ⇒ 处理 = 全文零物理行以自门嵌入口径原串起头（描述式写）⇒
//    自门现读本件**新增 embed 资产 0**；但**新增 1 个输入面件**（这是件存在的必然，非缺陷）⇒ input_files +1。
// ⑪ 只读面：本件不写仓内仓外任何件（自检夹具落 `os.MkdirTemp` 临时树并 `defer os.RemoveAll` 回收，
//    与 py 的 `shutil.rmtree` 对应）· 不重编 · 不跑 go build/go test · 不读状态目录。
