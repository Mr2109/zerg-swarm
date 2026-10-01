package main

// check_gap_ledger_view.go —— `scripts/gates/check-gap-ledger-view.py`（1030 行 · sha16 6818c0b2414a562c）的 Go 搬运件。
//
// 判据（R1…R6 · 逐字照搬 py 原件现读件体）：
//   R1 三方对拍（判红）：真源（jsonl 逐行 id+state）↔ `zerg gap export` 现跑 ↔ 当期结转账件（件族：索引 + 页件）。
//   R2 体积（判红 · 逐件）：≤ 300 行 · ≤ 60KB。
//   R3 单行（判红 · 逐件）：最长行 ≤ 200 字符（打行号 + 长度 + 前 60 字）。
//   R4 写保护（有指纹则判红 / 无则只报告）：导出产出指纹 ⟷ 生成件指纹 逐字比。
//   R5 缺件/空转（rc=2 · 不许当绿）：真源不可读/0 条 · export 跑不动 · 产出解析不出 · 文档仓无当期索引件。
//   R6 旧版账（只报告）：非当期 `项目文档/<版>/缺口总账-<日期>.md` ⇒ 列行数/字节/最长行，不入退码。
//
// 出厂声明：
//   (a) 默认根定位差异：py 从 `__file__` 上溯两层得 DEFAULT_ROOT；Go 无 `__file__` 语境 ⇒ 从 cwd 上溯找
//       含 `scripts/gates` 的目录当仓根。二者在「脚本位置 == cwd 的上溯根」时同构，否则路径推导面不同（✗ 不声称逐字对齐）。
//   (b) 自指检查：本件落在「扫 `core/cmd/zerg/*.go`」的门的扫描面内（如门⑪ T3 退码表、check-hardcoded-private-paths 等）。
//       本件全篇**零绝对路径字面量**、**零私仓路径原串**、**零反引号包裹的仓内路径串**；路径一律由 cwd / 状态目录推导，
//       或走显式入参旋钮（--root / --docs-root / --source）。故不引入新的自指命中（见回执自指对照）。
//
// 子进程面：`EXPORT_BIN = <仓根>/bin/zerg gap export`（只认本门隔壁那一个 · 不随 --root 换）· 90s 超时 · 只读。
// 写面：默认档**零写盘**（纯只读）· `--self-test` 写仓外临时夹具（Go 侧 `defer os.RemoveAll` 回收，顺带止血 py 的 A 类 tempdir 泄漏）。
// 外部真源真身：`<状态目录>/zerg-cli-gaps.jsonl`，状态目录缺省 = `~/.zerg/state`（显式旋钮 `--source` 可换）。

import (
	"encoding/json"
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
	"unicode/utf8"
)

const (
	glvVersion         = "check-gap-ledger-view.py v1.0.0"
	glvExportTimeout   = 90
	glvSourceName      = "zerg-cli-gaps.jsonl"
	glvLedgerDir       = "项目文档"
	glvGeneratedMarker = "> 生成件：由 `zerg gap export`"
	glvMaxLines        = 300
	glvMaxBytes        = 60 * 1024
	glvMaxLineChars    = 200
	glvNameLimit       = 5
)

var (
	glvExportArgs       = []string{"gap", "export"}
	glvOpenStates       = []string{"仍缺", "已派", "已立项", "回归"}
	glvClosedStates     = []string{"已解", "不做"}
	glvStateClosed      = []string{"仍缺", "已派", "已立项", "回归", "已解", "不做"}
	glvImpactClosed     = []string{"命令面", "门禁面", "文档面", "公开面", "换件面", "归档面"}
	glvPrioClosed       = []string{"P0", "P1", "P2"}
	glvExemptPathSegs   = []string{"承接自"}
	glvGeneratedMarkers = []string{glvGeneratedMarker, strings.TrimRight(glvGeneratedMarker, "`")}
)

var (
	glvLedgerNameRE     = regexp.MustCompile(`^缺口总账-(\d{8})\.md$`)
	glvLedgerDatePat    = `(\d{8}|\d{4}-\d{2}-\d{2})`
	glvIndexNameRE      = regexp.MustCompile(`^缺口总账-` + glvLedgerDatePat + `\.md$`)
	glvPageNameRE       = regexp.MustCompile(`^缺口总账-` + glvLedgerDatePat + `-([^-]+)-p(\d{2})\.md$`)
	glvHdrRE            = regexp.MustCompile(`真源条数 \*\*(\d+)\*\* · 未闭 \*\*(\d+)\*\* · 已闭 \*\*(\d+)\*\*（其中「不做」\*\*(\d+)\*\*）`)
	glvFaceRE           = regexp.MustCompile(`(?m)^> 按面：(.+)$`)
	glvBulletRE         = regexp.MustCompile("^- `([^`]+)` (\\S+) · \\*\\*([^*]+)\\*\\* · ")
	glvFootRE           = regexp.MustCompile(`未闭 (\d+) 条 · 单行上限 (\d+) 字符`)
	glvItemMarkBulletRE = regexp.MustCompile("^- `(GAP-\\d{8}-\\d+|Q-\\d{3})`(?:\\s|$)")
	glvItemMarkTableRE  = regexp.MustCompile("^\\|\\s*`?(GAP-\\d{8}-\\d+|Q-\\d{3})`?(?:\\s*\\||\\s*$)")
)

var glvFingerprintCarriers = []*regexp.Regexp{
	regexp.MustCompile(`指纹[^\n]{0,40}?([0-9a-fA-F]{8,64})`),
	regexp.MustCompile(`\bfp\s*[=:]\s*([0-9a-fA-F]{8,64})`),
	regexp.MustCompile(`(?i)\bsha256\s*[=:]\s*([0-9a-fA-F]{8,64})`),
}

// glvFingerprintPatterns —— `--list-rules` 打的是 py `rx.pattern`（**不含** re.I 内联旗标）。
var glvFingerprintPatterns = []string{
	`指纹[^\n]{0,40}?([0-9a-fA-F]{8,64})`,
	`\bfp\s*[=:]\s*([0-9a-fA-F]{8,64})`,
	`\bsha256\s*[=:]\s*([0-9a-fA-F]{8,64})`,
}

// ═════════════════════════════════════ py 路径 / 字符串原语同构 ═════════════════════════════════════

func glvPyIsSpace(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', ' ', 0x1c, 0x1d, 0x1e, 0x1f, 0x85, 0xa0:
		return true
	}
	return unicode.IsSpace(r)
}

// glvPyStrip —— Python str.strip() 同构（剥 Unicode 空白，含 0x1c-0x1f/0x85/0xa0）。
func glvPyStrip(s string) string { return strings.TrimFunc(s, glvPyIsSpace) }

// glvPyJoin —— 照 CPython posixpath.join 逐段规则直译。
func glvPyJoin(a string, parts ...string) string {
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

// glvPyNormpath —— 照 CPython posixpath.normpath（保留恰两个前导斜杠）。
func glvPyNormpath(path string) string {
	if path == "" {
		return "."
	}
	initial := 0
	if strings.HasPrefix(path, "/") {
		initial = 1
		if strings.HasPrefix(path, "//") && !strings.HasPrefix(path, "///") {
			initial = 2
		}
	}
	var newComps []string
	for _, comp := range strings.Split(path, "/") {
		if comp == "" || comp == "." {
			continue
		}
		if comp != ".." || (initial == 0 && len(newComps) == 0) || (len(newComps) > 0 && newComps[len(newComps)-1] == "..") {
			newComps = append(newComps, comp)
		} else if len(newComps) > 0 {
			newComps = newComps[:len(newComps)-1]
		}
	}
	out := strings.Join(newComps, "/")
	if initial > 0 {
		out = strings.Repeat("/", initial) + out
	}
	if out == "" {
		return "."
	}
	return out
}

func glvPyIsAbs(p string) bool { return strings.HasPrefix(p, "/") }

func glvPyAbspath(path string) string {
	if !glvPyIsAbs(path) {
		cwd, _ := os.Getwd()
		path = glvPyJoin(cwd, path)
	}
	return glvPyNormpath(path)
}

// glvPyDirname —— 照 CPython posixpath.split 的 head 语义（剥尾斜杠，根除外）。
func glvPyDirname(p string) string {
	i := strings.LastIndex(p, "/") + 1
	head, tail := p[:i], p[i:]
	_ = tail
	if head != "" && head != strings.Repeat("/", len(head)) {
		head = strings.TrimRight(head, "/")
	}
	return head
}

// glvPyRelpath —— 照 CPython posixpath.relpath。
func glvPyRelpath(path, start string) string {
	start = glvPyAbspath(start)
	path = glvPyAbspath(path)
	if path == start {
		return "."
	}
	var startList, pathList []string
	for _, x := range strings.Split(start, "/") {
		if x != "" {
			startList = append(startList, x)
		}
	}
	for _, x := range strings.Split(path, "/") {
		if x != "" {
			pathList = append(pathList, x)
		}
	}
	i := 0
	for i < len(startList) && i < len(pathList) && startList[i] == pathList[i] {
		i++
	}
	var rel []string
	for j := i; j < len(startList); j++ {
		rel = append(rel, "..")
	}
	rel = append(rel, pathList[i:]...)
	if len(rel) == 0 {
		return "."
	}
	return strings.Join(rel, "/")
}

// glvSplitlines —— Python str.splitlines 同构（含 \v \f 0x1c-0x1e \x85 \u2028 \u2029）。
func glvSplitlines(s string) []string {
	var out []string
	var cur strings.Builder
	rs := []rune(s)
	for i := 0; i < len(rs); i++ {
		r := rs[i]
		switch r {
		case '\n', '\v', '\f', 0x1c, 0x1d, 0x1e, 0x85, 0x2028, 0x2029:
			out = append(out, cur.String())
			cur.Reset()
		case '\r':
			if i+1 < len(rs) && rs[i+1] == '\n' {
				i++
			}
			out = append(out, cur.String())
			cur.Reset()
		default:
			cur.WriteRune(r)
		}
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

func glvDash(txt string, n int) string {
	s := strings.ReplaceAll(glvPyStrip(txt), "\n", " ")
	rn := []rune(s)
	if len(rn) <= n {
		return s
	}
	return string(rn[:n]) + "…"
}

// ═════════════════════════════════════ 有序 JSON（保 py dict 插入序 + json.dumps 分隔符）═════════════════

type glvJObj struct {
	keys []string
	vals []any
}

func glvObjNew() *glvJObj { return &glvJObj{} }

func (o *glvJObj) set(k string, v any) *glvJObj {
	o.keys = append(o.keys, k)
	o.vals = append(o.vals, v)
	return o
}

// glvKV —— 速建有对象：glvKV("a", 1, "b", "x")。
func glvKV(kv ...any) *glvJObj {
	o := glvObjNew()
	for i := 0; i+1 < len(kv); i += 2 {
		o.set(kv[i].(string), kv[i+1])
	}
	return o
}

func glvJSONEsc(s string) string {
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

func glvJSONWrite(b *strings.Builder, v any) {
	switch x := v.(type) {
	case nil:
		b.WriteString("null")
	case bool:
		if x {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	case int:
		b.WriteString(strconv.Itoa(x))
	case string:
		b.WriteString(glvJSONEsc(x))
	case *glvJObj:
		b.WriteString("{")
		for i, k := range x.keys {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(glvJSONEsc(k))
			b.WriteString(": ")
			glvJSONWrite(b, x.vals[i])
		}
		b.WriteString("}")
	case []any:
		b.WriteString("[")
		for i, e := range x {
			if i > 0 {
				b.WriteString(", ")
			}
			glvJSONWrite(b, e)
		}
		b.WriteString("]")
	case []string:
		b.WriteString("[")
		for i, e := range x {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(glvJSONEsc(e))
		}
		b.WriteString("]")
	default:
		b.WriteString("null")
	}
}

func glvJSONDump(v any) string {
	var b strings.Builder
	glvJSONWrite(&b, v)
	return b.String()
}

// ═════════════════════════════════════ 读盘面 ═════════════════════════════════════

type glvRec struct {
	Line                    int
	ID, State, Impact, Prio string
}

type glvBadLine struct {
	Line int
	Msg  string
}

type glvOutOfSet struct {
	Line  int
	ID    string
	Field string
	Value string
}

type glvSrcRead struct {
	Status   string
	Path     string
	Msg      string
	Lines    int
	Bytes    int
	Recs     []glvRec
	Bad      []glvBadLine
	IDs      []string
	DupIDs   []string
	Open     []string
	Closed   []string
	StateOf  map[string]string
	OutOfSet []glvOutOfSet
}

func (r *glvSrcRead) toJSON() *glvJObj {
	switch r.Status {
	case "absent":
		return glvKV("path", r.Path)
	case "unreadable":
		return glvKV("path", r.Path, "msg", r.Msg)
	case "badline":
		bad := []any{}
		for _, b := range r.Bad {
			bad = append(bad, glvKV("line", b.Line, "msg", b.Msg))
		}
		recs := []any{}
		for _, x := range r.Recs {
			recs = append(recs, glvKV("line", x.Line, "id", x.ID, "state", x.State, "impact", x.Impact, "prio", x.Prio))
		}
		return glvKV("path", r.Path, "bad", bad, "recs", recs)
	case "empty":
		return glvKV("path", r.Path, "lines", r.Lines, "bytes", r.Bytes)
	}
	// ok
	ids := r.IDs
	if ids == nil {
		ids = []string{}
	}
	dup := r.DupIDs
	if dup == nil {
		dup = []string{}
	}
	oos := []any{}
	for _, x := range r.OutOfSet {
		oos = append(oos, glvKV("line", x.Line, "id", x.ID, "field", x.Field, "value", x.Value))
	}
	open := r.Open
	if open == nil {
		open = []string{}
	}
	closed := r.Closed
	if closed == nil {
		closed = []string{}
	}
	so := glvObjNew()
	for _, id := range ids {
		so.set(id, r.StateOf[id])
	}
	return glvKV("path", r.Path, "lines", r.Lines, "bytes", r.Bytes, "recs", len(r.Recs),
		"ids", ids, "dup_ids", dup, "states", glvObjNew(), "open", open, "closed", closed,
		"state_of", so, "out_of_set", oos)
}

func glvReadSource(path string) *glvSrcRead {
	st, err := os.Stat(path)
	if err != nil || st.IsDir() {
		return &glvSrcRead{Status: "absent", Path: path}
	}
	rawB, err := os.ReadFile(path)
	if err != nil {
		return &glvSrcRead{Status: "unreadable", Path: path, Msg: err.Error()}
	}
	raw := string(rawB)
	var recs []glvRec
	var bad []glvBadLine
	for i, ln := range glvSplitlines(raw) {
		s := glvPyStrip(ln)
		if s == "" {
			continue
		}
		o, jerr := glvParseJSONObject(s)
		if jerr != "" {
			bad = append(bad, glvBadLine{Line: i + 1, Msg: jerr})
			continue
		}
		recs = append(recs, glvRec{Line: i + 1, ID: o.str("id"), State: o.str("state"),
			Impact: o.str("impact"), Prio: o.str("prio")})
	}
	if len(bad) > 0 {
		return &glvSrcRead{Status: "badline", Path: path, Bad: bad, Recs: recs}
	}
	if len(recs) == 0 {
		return &glvSrcRead{Status: "empty", Path: path, Lines: len(glvSplitlines(raw)), Bytes: len(rawB)}
	}
	ids := make([]string, 0, len(recs))
	cnt := map[string]int{}
	for _, r := range recs {
		ids = append(ids, r.ID)
		cnt[r.ID]++
	}
	var dup []string
	for k, v := range cnt {
		if v > 1 {
			dup = append(dup, k)
		}
	}
	sort.Strings(dup)
	var open, closed []string
	stateOf := map[string]string{}
	for _, r := range recs {
		stateOf[r.ID] = r.State
		if glvIn(r.State, glvOpenStates) {
			open = append(open, r.ID)
		}
		if glvIn(r.State, glvClosedStates) {
			closed = append(closed, r.ID)
		}
	}
	sort.Strings(open)
	sort.Strings(closed)
	var oos []glvOutOfSet
	for _, r := range recs {
		if !glvIn(r.State, glvStateClosed) {
			oos = append(oos, glvOutOfSet{Line: r.Line, ID: r.ID, Field: "state", Value: r.State})
		}
		if !glvIn(r.Impact, glvImpactClosed) {
			oos = append(oos, glvOutOfSet{Line: r.Line, ID: r.ID, Field: "impact", Value: r.Impact})
		}
		if r.Prio != "" && !glvIn(r.Prio, glvPrioClosed) {
			oos = append(oos, glvOutOfSet{Line: r.Line, ID: r.ID, Field: "prio", Value: r.Prio})
		}
	}
	return &glvSrcRead{Status: "ok", Path: path, Lines: len(glvSplitlines(raw)), Bytes: len(rawB),
		Recs: recs, IDs: ids, DupIDs: dup, Open: open, Closed: closed, StateOf: stateOf, OutOfSet: oos}
}

func glvIn(s string, set []string) bool {
	for _, x := range set {
		if x == s {
			return true
		}
	}
	return false
}

// glvParseJSONObject —— 最小 JSON 对象解析（只取 id/state/impact/prio 四个字符串值）。
// 缺键 / 显式 null 都回落空串（与 py `str(o.get("id",""))` 在 `status=str(None)` 上一致）。
type glvJSONObj map[string]any

// glvParseJSONObject —— 最小 JSON 对象解析（只取 id/state/impact/prio 四个值）。
func glvParseJSONObject(s string) (glvJSONObj, string) {
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return nil, err.Error()
	}
	return glvJSONObj(m), ""
}

func (o glvJSONObj) str(k string) string {
	v, ok := o[k]
	if !ok || v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}

// ═════════════════════════════════════ 导出面 ═════════════════════════════════════

type glvExportParsed struct {
	Total       int
	Open        int
	Closed      int
	NoDecide    int
	FootOpen    int
	FootLineMax int
	Bullets     []glvBullet
	Fingerprint string
}

type glvBullet struct {
	ID, Prio, State string
}

func glvParseExport(out string) *glvExportParsed {
	h := glvHdrRE.FindStringSubmatch(out)
	f := glvFootRE.FindStringSubmatch(out)
	if h == nil || f == nil {
		return nil
	}
	var bullets []glvBullet
	for _, l := range glvSplitlines(out) {
		m := glvBulletRE.FindStringSubmatch(l)
		if m != nil {
			bullets = append(bullets, glvBullet{ID: m[1], Prio: m[2], State: m[3]})
		}
	}
	return &glvExportParsed{
		Total: glvAtoi(h[1]), Open: glvAtoi(h[2]), Closed: glvAtoi(h[3]), NoDecide: glvAtoi(h[4]),
		FootOpen: glvAtoi(f[1]), FootLineMax: glvAtoi(f[2]), Bullets: bullets,
		Fingerprint: glvFindFingerprint(out),
	}
}

func glvFindFingerprint(text string) string {
	for _, rx := range glvFingerprintCarriers {
		if m := rx.FindStringSubmatch(text); m != nil {
			return m[1]
		}
	}
	return ""
}

func glvAtoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

// ═════════════════════════════════════ 账件面 ═════════════════════════════════════

type glvLedgerStats struct {
	Path, Rel   string
	Kind        string
	Lines       int
	Bytes       int
	MaxLineNo   int
	MaxLineLen  int
	MaxLineHead string
	Generated   bool
	Fingerprint string
	IDs         []string
	Header      *glvHeader
	Text        string
}

type glvHeader struct {
	Total, Open, Closed int
}

func glvLedgerIDs(txt string) []string {
	var ids []string
	for _, ln := range glvSplitlines(txt) {
		if m := glvItemMarkBulletRE.FindStringSubmatch(ln); m != nil {
			ids = append(ids, m[1])
			continue
		}
		if m := glvItemMarkTableRE.FindStringSubmatch(ln); m != nil {
			ids = append(ids, m[1])
		}
	}
	return glvSortedUnique(ids)
}

func glvLedgerStatsOf(path string) *glvLedgerStats {
	rawB, err := os.ReadFile(path)
	if err != nil {
		return &glvLedgerStats{Path: path}
	}
	txt := string(rawB)
	lines := glvSplitlines(txt)
	mx := -1
	best := -1
	for i, l := range lines {
		if n := utf8.RuneCountInString(l); n > best {
			best = n
			mx = i
		}
	}
	s := &glvLedgerStats{Path: path, Lines: len(lines), Bytes: len(rawB)}
	if mx >= 0 {
		s.MaxLineNo = mx + 1
		s.MaxLineLen = utf8.RuneCountInString(lines[mx])
		s.MaxLineHead = glvDash(lines[mx], 60)
	}
	s.Generated = strings.Contains(txt, glvGeneratedMarker)
	s.Fingerprint = glvFindFingerprint(txt)
	s.IDs = glvLedgerIDs(txt)
	if h := glvHdrRE.FindStringSubmatch(txt); h != nil {
		s.Header = &glvHeader{Total: glvAtoi(h[1]), Open: glvAtoi(h[2]), Closed: glvAtoi(h[3])}
	}
	s.Text = txt
	return s
}

// ═════════════════════════════════════ 遍历面（os.walk 同构 · 原序）═════════════════════════════════

// glvScandirSplit —— readdir 原序；目录判定用 os.Stat（跟随符号链接 · 同 py entry.is_dir()）。
func glvScandirSplit(dir string) (dirs, nondirs []string) {
	f, err := os.Open(dir)
	if err != nil {
		return nil, nil // os.walk onerror=None ⇒ 静默跳过
	}
	entries, _ := f.ReadDir(-1)
	f.Close()
	for _, e := range entries {
		p := glvPyJoin(dir, e.Name())
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			dirs = append(dirs, e.Name())
		} else {
			nondirs = append(nondirs, e.Name())
		}
	}
	return dirs, nondirs
}

// glvWalkPy —— os.walk(top, followlinks=False) 同构：先 yield 当前层文件，再按 readdir 原序下钻。
func glvWalkPy(top string, fn func(root string, files []string)) {
	dirs, nondirs := glvScandirSplit(top)
	fn(top, nondirs)
	for _, d := range dirs {
		p := glvPyJoin(top, d)
		if fi, err := os.Lstat(p); err == nil && fi.Mode()&os.ModeSymlink != 0 {
			continue
		}
		glvWalkPy(p, fn)
	}
}

func glvSortedUnique(ids []string) []string {
	if len(ids) == 0 {
		return nil
	}
	m := map[string]bool{}
	for _, x := range ids {
		m[x] = true
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ═════════════════════════════════════ 选件（件族）═════════════════════════════════

type glvFound struct {
	Path, Rel, Ver, Dir, Kind, Date, DateKey, Face, Page string
	Exempt                                               bool
}

type glvFamily struct {
	Index       *glvFound
	Pages       []*glvFound
	Family      []*glvFound
	History     []*glvFound
	Exempt      []*glvFound
	OrphanPages []*glvFound
}

func glvVkey(ver string) ([]int, string) {
	var nums []int
	cur := ""
	for _, r := range ver {
		if r >= '0' && r <= '9' {
			cur += string(r)
		} else if cur != "" {
			n, _ := strconv.Atoi(cur)
			nums = append(nums, n)
			cur = ""
		}
	}
	if cur != "" {
		n, _ := strconv.Atoi(cur)
		nums = append(nums, n)
	}
	return nums, ver
}

func glvVkeyLess(a, b []int) bool {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return len(a) < len(b)
}

func glvCollectFamily(docs string) *glvFamily {
	base := glvPyJoin(docs, glvLedgerDir)
	var found []*glvFound
	if st, err := os.Stat(base); err == nil && st.IsDir() {
		verEntries, _ := os.ReadDir(base)
		var vers []string
		for _, e := range verEntries {
			vers = append(vers, e.Name())
		}
		sort.Strings(vers)
		for _, ver := range vers {
			vdir := glvPyJoin(base, ver)
			if st, err := os.Stat(vdir); err != nil || !st.IsDir() {
				continue
			}
			glvWalkPy(vdir, func(root string, files []string) {
				sf := append([]string{}, files...)
				sort.Strings(sf)
				for _, fn := range sf {
					pm := glvPageNameRE.FindStringSubmatch(fn)
					m := glvIndexNameRE.FindStringSubmatch(fn)
					if pm == nil && m == nil {
						continue
					}
					p := glvPyJoin(root, fn)
					rel := glvPyRelpath(p, docs)
					g := pm
					if g == nil {
						g = m
					}
					kind := "index"
					face, page := "", ""
					if pm != nil {
						kind = "page"
						face = pm[2]
						page = pm[3]
					}
					found = append(found, &glvFound{Path: p, Rel: rel, Ver: ver, Dir: glvPyDirname(p),
						Kind: kind, Date: g[1], DateKey: strings.ReplaceAll(g[1], "-", ""),
						Face: face, Page: page, Exempt: glvExempt(rel)})
				}
			})
		}
	}
	var live []*glvFound
	for _, x := range found {
		if !x.Exempt {
			live = append(live, x)
		}
	}
	var idxPool []*glvFound
	for _, x := range live {
		if x.Kind == "index" {
			idxPool = append(idxPool, x)
		}
	}
	sort.SliceStable(idxPool, func(i, j int) bool {
		ai, av := glvVkey(idxPool[i].Ver)
		bi, bv := glvVkey(idxPool[j].Ver)
		if glvVkeyLess(ai, bi) {
			return true
		}
		if glvVkeyLess(bi, ai) {
			return false
		}
		if av != bv {
			return av < bv
		}
		if idxPool[i].DateKey != idxPool[j].DateKey {
			return idxPool[i].DateKey < idxPool[j].DateKey
		}
		return idxPool[i].Path < idxPool[j].Path
	})
	var index *glvFound
	if len(idxPool) > 0 {
		index = idxPool[len(idxPool)-1]
	}
	var pages, orphans []*glvFound
	if index != nil {
		for _, x := range live {
			if x.Kind != "page" {
				continue
			}
			ok := glvIn(x.Face, glvImpactClosed) && x.DateKey == index.DateKey && x.Dir == index.Dir
			if ok {
				pages = append(pages, x)
			} else {
				orphans = append(orphans, x)
			}
		}
		sort.SliceStable(pages, func(i, j int) bool {
			if pages[i].Face != pages[j].Face {
				return pages[i].Face < pages[j].Face
			}
			if pages[i].Page != pages[j].Page {
				return pages[i].Page < pages[j].Page
			}
			return pages[i].Path < pages[j].Path
		})
	} else {
		for _, x := range live {
			if x.Kind == "page" {
				orphans = append(orphans, x)
			}
		}
	}
	fam := &glvFamily{Index: index, Pages: pages, OrphanPages: orphans}
	if index != nil {
		fam.Family = append(fam.Family, index)
	}
	fam.Family = append(fam.Family, pages...)
	famPaths := map[string]bool{}
	for _, x := range fam.Family {
		famPaths[x.Path] = true
	}
	for _, x := range found {
		if !famPaths[x.Path] {
			fam.History = append(fam.History, x)
		}
	}
	for _, x := range found {
		if x.Exempt {
			fam.Exempt = append(fam.Exempt, x)
		}
	}
	return fam
}

func glvExempt(rel string) bool {
	for _, seg := range glvExemptPathSegs {
		if strings.Contains(rel, seg) {
			return true
		}
	}
	return false
}

// ═════════════════════════════════════ 判据 ═════════════════════════════════════

type glvNote struct {
	Rule, What string
	Read       any
	HasName    bool
	Name       []string
}

func (n *glvNote) toJSON() *glvJObj {
	o := glvObjNew()
	o.set("rule", n.Rule).set("what", n.What).set("read", n.Read)
	if n.HasName {
		nm := n.Name
		if nm == nil {
			nm = []string{}
		}
		o.set("name", nm)
	}
	return o
}

func glvSplitDiff(want, got []string) ([]string, []string) {
	w := map[string]bool{}
	g := map[string]bool{}
	for _, x := range want {
		w[x] = true
	}
	for _, x := range got {
		g[x] = true
	}
	var miss, extra []string
	for x := range w {
		if !g[x] {
			miss = append(miss, x)
		}
	}
	for x := range g {
		if !w[x] {
			extra = append(extra, x)
		}
	}
	sort.Strings(miss)
	sort.Strings(extra)
	return miss, extra
}

func glvNameList(ids []string, what string) []string {
	if len(ids) == 0 {
		return nil
	}
	var head []string
	for i, id := range ids {
		if i >= glvNameLimit {
			break
		}
		head = append(head, "`"+id+"`")
	}
	h := strings.Join(head, "、")
	if len(ids) > glvNameLimit {
		h += fmt.Sprintf("（共 %d 条 · 此处只点前 %d 条）", len(ids), glvNameLimit)
	}
	return []string{fmt.Sprintf("%s %d 条：%s", what, len(ids), h)}
}

func glvResolveRoot() string {
	cwd, _ := os.Getwd()
	d := cwd
	for {
		if st, err := os.Stat(glvPyJoin(d, "scripts", "gates")); err == nil && st.IsDir() {
			return d
		}
		parent := glvDirOf(d)
		if parent == d {
			break
		}
		d = parent
	}
	return cwd
}

func glvDirOf(p string) string {
	if i := strings.LastIndex(p, "/"); i > 0 {
		return p[:i]
	}
	if strings.HasPrefix(p, "/") {
		return "/"
	}
	return p
}

func glvStateDirDefault() string {
	home, _ := os.UserHomeDir()
	return glvPyJoin(home, ".zerg", "state")
}

func glvRun(root, docsRoot, source string, jsonOut bool, exportCmd []string, defaultRoot, exportBin string, stdout io.Writer) int {
	root = glvPyAbspath(root)
	var problems, reds, reports []*glvNote

	if st, err := os.Stat(glvPyJoin(root, "scripts", "gates")); err != nil || !st.IsDir() {
		fmt.Fprintf(stdout, "⛔ 不给结论（rc=2）：`%s` 下没有 `scripts/gates/` ⇒ 判不了根\n", root)
		return 2
	}
	if st, err := os.Stat(exportBin); err != nil || st.IsDir() {
		fmt.Fprintf(stdout, "⛔ 不给结论（rc=2）：导出器不在盘：`%s`（读不到导出面 ⇒ 判不了三方）\n", exportBin)
		return 2
	}
	docs := docsRoot
	if docs == "" {
		docs = glvPyJoin(glvPyDirname(root), "Zerg-内部文档")
	}
	docs = glvPyAbspath(docs)
	if st, err := os.Stat(docs); err != nil || !st.IsDir() {
		fmt.Fprintf(stdout, "⛔ 不给结论（rc=2）：文档仓根不在盘：`%s`（不拿「读不到」当「没问题」）\n", docs)
		return 2
	}
	src := source
	if src == "" {
		src = glvPyJoin(glvStateDirDefault(), glvSourceName)
	}
	src = glvPyAbspath(src)

	// ── R5：真源现读 ──
	sread := glvReadSource(src)
	if sread.Status != "ok" {
		what := map[string]string{"absent": "不在盘", "unreadable": "读不到",
			"badline": "有非 JSON 行", "empty": "0 条（空转）"}[sread.Status]
		problems = append(problems, &glvNote{Rule: "R5", What: "真源" + what, Read: sread.toJSON()})
	}

	// ── R5：导出器现跑 ──
	var exp *glvExportParsed
	if len(problems) == 0 {
		cmd := exportCmd
		if cmd == nil {
			cmd = append([]string{exportBin}, glvExportArgs...)
		}
		exst, erc, emsg, eout := glvRunExport(cmd, glvPyDirname(src), defaultRoot)
		if exst != "ok" {
			problems = append(problems, &glvNote{Rule: "R5",
				What: fmt.Sprintf("`zerg gap export` 跑不动（rc=%d）", erc),
				Read: glvKV("cmd", strings.Join(cmd, " "), "msg", emsg)})
		} else {
			exp = glvParseExport(eout)
			if exp == nil {
				problems = append(problems, &glvNote{Rule: "R5", What: "导出产出解析不出（件头/件尾对不上）",
					Read: glvKV("bytes", len(eout))})
			}
		}
	}

	// ── 当期结转账件 + 历史件（R6 只报告）──
	fam := glvCollectFamily(docs)
	current, pages, history, exempt := fam.Index, fam.Pages, fam.History, fam.Exempt
	var led *glvLedgerStats
	if current == nil {
		problems = append(problems, &glvNote{Rule: "R5",
			What: "文档仓里一件当期**索引件** `缺口总账-<日期>.md` 都没有（空转 ⇒ 判不了三方）",
			Read: glvKV("glob", fmt.Sprintf("%s/<版>/缺口总账-<日期>.md", glvLedgerDir))})
	} else {
		led = glvLedgerStatsOf(current.Path)
		led.Rel = current.Rel
	}
	type famLed struct {
		X *glvFound
		S *glvLedgerStats
	}
	var famLedList []famLed
	for _, x := range fam.Family {
		s := glvLedgerStatsOf(x.Path)
		s.Rel, s.Kind = x.Rel, x.Kind
		famLedList = append(famLedList, famLed{X: x, S: s})
	}
	for _, x := range history {
		s := glvLedgerStatsOf(x.Path)
		reports = append(reports, &glvNote{Rule: "R6",
			What: fmt.Sprintf("历史账（只报告 · 不入退码）%s", x.Rel),
			Read: glvKV("lines", s.Lines, "bytes", s.Bytes, "maxline_no", s.MaxLineNo,
				"maxline_len", s.MaxLineLen, "exempt", x.Exempt)})
	}

	// ── R1 三方对拍 ──
	r1Src := glvKV("recs", len(sread.Recs), "open", len(sread.Open), "closed", len(sread.Closed))
	var r1Export, r1Ledger any
	var famIDs []string
	if len(problems) == 0 {
		r1Export = glvKV("total", exp.Total, "open", exp.Open, "closed", exp.Closed,
			"no_decide", exp.NoDecide, "bullets", len(exp.Bullets), "foot_open", exp.FootOpen)
		if exp.Total != len(sread.Recs) {
			reds = append(reds, &glvNote{Rule: "R1a",
				What: fmt.Sprintf("条数不一致：真源 %d 条 vs 导出件头「真源条数 %d」", len(sread.Recs), exp.Total),
				Read: glvObjNew()})
		}
		if exp.Open != exp.FootOpen {
			reds = append(reds, &glvNote{Rule: "R1a",
				What: fmt.Sprintf("导出件内自相矛盾：件头「未闭 %d」vs 件尾「未闭 %d 条」", exp.Open, exp.FootOpen),
				Read: glvObjNew()})
		}
		if exp.Open+exp.Closed != exp.Total {
			reds = append(reds, &glvNote{Rule: "R1a",
				What: fmt.Sprintf("导出件头算术不闭合：未闭 %d + 已闭 %d ≠ 条数 %d", exp.Open, exp.Closed, exp.Total),
				Read: glvObjNew()})
		}
		var expIDs []string
		for _, b := range exp.Bullets {
			expIDs = append(expIDs, b.ID)
		}
		miss, extra := glvSplitDiff(sread.Open, expIDs)
		if len(miss) > 0 {
			n := &glvNote{Rule: "R1a", What: "真源未闭里有、导出正文里**缺**", Read: glvObjNew(),
				HasName: true, Name: glvNameList(miss, "缺")}
			reds = append(reds, n)
		}
		if len(extra) > 0 {
			n := &glvNote{Rule: "R1a", What: "导出正文里有、真源未闭里**多**", Read: glvObjNew(),
				HasName: true, Name: glvNameList(extra, "多")}
			reds = append(reds, n)
		}
		for _, b := range exp.Bullets {
			want, ok := sread.StateOf[b.ID]
			if ok && want != b.State {
				reds = append(reds, &glvNote{Rule: "R1a",
					What: fmt.Sprintf("状态不一致 `%s`：真源「%s」vs 导出「%s」", b.ID, want, b.State),
					Read: glvObjNew()})
			}
		}
		if len(sread.DupIDs) > 0 {
			reds = append(reds, &glvNote{Rule: "R1a", What: "真源 id 重复（主键必须唯一）", Read: glvObjNew(),
				HasName: true, Name: glvNameList(sread.DupIDs, "重号")})
		}
		if exp.NoDecide > exp.Closed {
			reds = append(reds, &glvNote{Rule: "R1a",
				What: fmt.Sprintf("「不做」%d > 已闭 %d（不可能）", exp.NoDecide, exp.Closed), Read: glvObjNew()})
		}
		// R1b：真源 ↔ 当期结转账件（第三方腿）
		var allIDs []string
		for _, fl := range famLedList {
			allIDs = append(allIDs, fl.S.IDs...)
		}
		famIDs = glvSortedUnique(allIDs)
		type pageIDs struct {
			Rel string
			IDs []string
		}
		var pageIDList []pageIDs
		for _, fl := range famLedList {
			if fl.X.Kind == "page" {
				pageIDList = append(pageIDList, pageIDs{Rel: fl.S.Rel, IDs: fl.S.IDs})
			}
		}
		ldiffMiss, ldiffExtra := glvSplitDiff(sread.Open, famIDs)
		var hdr any
		if led.Header != nil {
			hdr = glvKV("total", led.Header.Total, "open", led.Header.Open, "closed", led.Header.Closed)
		}
		r1Ledger = glvKV("rel", led.Rel, "lines", led.Lines, "bytes", led.Bytes,
			"generated", led.Generated, "ids", len(led.IDs), "header", hdr,
			"pages", len(pages), "family_ids", len(famIDs))
		anyMarker := false
		for _, mk := range glvGeneratedMarkers {
			if strings.Contains(led.Text, mk) {
				anyMarker = true
			}
		}
		if !anyMarker {
			reds = append(reds, &glvNote{Rule: "R1b",
				What: fmt.Sprintf("**生成件未接管**：当期结转账件 `%s` 里没有生成标记逐字句「%s」（或**同一句去掉收尾反引号**的前缀形态）⇒ 它仍是手写账（第三方腿不是真源视图）", led.Rel, glvGeneratedMarker),
				Read: glvObjNew()})
		} else if led.Header != nil {
			if led.Header.Open != exp.Open {
				reds = append(reds, &glvNote{Rule: "R1b",
					What: fmt.Sprintf("账件件头「未闭 %d」vs 导出现跑「未闭 %d」不一致", led.Header.Open, exp.Open),
					Read: glvObjNew()})
			}
			if led.Header.Total != len(sread.Recs) {
				reds = append(reds, &glvNote{Rule: "R1b",
					What: fmt.Sprintf("账件件头「真源条数 %d」vs 真源现读 %d 条不一致", led.Header.Total, len(sread.Recs)),
					Read: glvObjNew()})
			}
		}
		if len(ldiffMiss) > 0 {
			reds = append(reds, &glvNote{Rule: "R1b", What: "真源未闭里有、账件里**缺**", Read: glvObjNew(),
				HasName: true, Name: glvNameList(ldiffMiss, "缺")})
		}
		if len(ldiffExtra) > 0 {
			reds = append(reds, &glvNote{Rule: "R1b", What: "账件里有、真源未闭里**多**（含已闭/历史号）", Read: glvObjNew(),
				HasName: true, Name: glvNameList(ldiffExtra, "多")})
		}
		// R1c：页件两两 id 不交
		seen := map[string]string{}
		var dupPairs [][3]string
		for _, p := range pageIDList {
			for _, i := range p.IDs {
				if prev, ok := seen[i]; ok {
					dupPairs = append(dupPairs, [3]string{i, prev, p.Rel})
				}
				if _, ok := seen[i]; !ok {
					seen[i] = p.Rel
				}
			}
		}
		if len(dupPairs) > 0 {
			first := []any{}
			n := len(dupPairs)
			if n > 3 {
				n = 3
			}
			for _, x := range dupPairs[:n] {
				first = append(first, []any{x[0], x[1], x[2]})
			}
			var ds []string
			for _, x := range dupPairs {
				ds = append(ds, x[0])
			}
			reds = append(reds, &glvNote{Rule: "R1c",
				What: fmt.Sprintf("页件两两 id 有交集（同一号在多页重复 ⇒ 并集会把重页完全掩盖）：%d 处", len(dupPairs)),
				Read: glvKV("first", first), HasName: true, Name: glvNameList(glvSortedUnique(ds), "重号")})
		}
		// R1d：Σ页条目 == 索引件头「未闭」
		if len(pages) > 0 && led.Header != nil {
			ptot := 0
			for _, p := range pageIDList {
				ptot += len(p.IDs)
			}
			if ptot != led.Header.Open {
				reds = append(reds, &glvNote{Rule: "R1d",
					What: fmt.Sprintf("Σ页条目 %d ≠ 索引件头「未闭 %d」（漏页 / 重页 / 多算）", ptot, led.Header.Open),
					Read: glvKV("page_entries", ptot, "index_open", led.Header.Open)})
			}
		}
	}

	// ── R2 体积 ──
	for _, fl := range famLedList {
		var over []string
		if fl.S.Lines > glvMaxLines {
			over = append(over, fmt.Sprintf("行数 %d > 上限 %d", fl.S.Lines, glvMaxLines))
		}
		if fl.S.Bytes > glvMaxBytes {
			over = append(over, fmt.Sprintf("字节 %d > 上限 %d", fl.S.Bytes, glvMaxBytes))
		}
		if len(over) > 0 {
			reds = append(reds, &glvNote{Rule: "R2",
				What: fmt.Sprintf("结转账件体积超限：%s（件 `%s`）", strings.Join(over, " · "), fl.S.Rel),
				Read: glvKV("lines", fl.S.Lines, "bytes", fl.S.Bytes)})
		}
	}
	// ── R3 单行 ──
	for _, fl := range famLedList {
		if fl.S.MaxLineLen > glvMaxLineChars {
			reds = append(reds, &glvNote{Rule: "R3",
				What: fmt.Sprintf("结转账件最长行 %d 字符 > 上限 %d（第 %d 行 · 前 60 字：%s）（件 `%s`）",
					fl.S.MaxLineLen, glvMaxLineChars, fl.S.MaxLineNo, fl.S.MaxLineHead, fl.S.Rel),
				Read: glvKV("line", fl.S.MaxLineNo, "len", fl.S.MaxLineLen)})
		}
	}

	// ── R4 写保护 ──
	var expFP, ledFP string
	ledGenerated := false
	if led != nil {
		ledFP = led.Fingerprint
		ledGenerated = led.Generated
	}
	if exp != nil {
		expFP = exp.Fingerprint
	}
	verdict := ""
	if led != nil && exp != nil {
		switch {
		case expFP == "" && ledFP == "":
			verdict = "无指纹 ⇒ 无法判手改（R-05 未落地 · **只报告**）：`gap export` 今天的人面产出里**没有**可对拍指纹，本门不发明格式 ⇒ 不入退码"
		case expFP != "" && ledFP == "" && led.Generated:
			verdict = "生成件缺指纹（导出器有指纹而账件里没有）⇒ 判红"
			reds = append(reds, &glvNote{Rule: "R4",
				What: fmt.Sprintf("生成件缺指纹（导出器现跑带指纹 `%s`）", glvFirst16(expFP)), Read: glvObjNew()})
		case expFP != "" && ledFP == "":
			verdict = "账件不是生成件 ⇒ 无指纹可判（只报告）"
		case expFP != ledFP:
			verdict = "指纹不符 ⇒ 判红（手改嫌疑）"
			reds = append(reds, &glvNote{Rule: "R4",
				What: fmt.Sprintf("指纹不符：导出现跑 `%s` vs 账件 `%s`", glvFirst16(expFP), glvFirst16(ledFP)),
				Read: glvObjNew()})
		default:
			verdict = fmt.Sprintf("指纹逐字一致（`%s…`）", glvFirst16(expFP))
		}
	}
	r4verdict := verdict
	if r4verdict == "" {
		r4verdict = "判不了（缺件）"
	}
	reports = append(reports, &glvNote{Rule: "R4", What: "写保护现读：" + r4verdict,
		Read: glvKV("export_fp", glvNilIfEmpty(expFP), "ledger_fp", glvNilIfEmpty(ledFP))})
	if len(sread.OutOfSet) > 0 {
		first := []any{}
		n := len(sread.OutOfSet)
		if n > 3 {
			n = 3
		}
		for _, x := range sread.OutOfSet[:n] {
			first = append(first, glvKV("line", x.Line, "id", x.ID, "field", x.Field, "value", x.Value))
		}
		reports = append(reports, &glvNote{Rule: "-",
			What: "真源里有值不在闭集（导出器会自行判红；本门照实数报告）",
			Read: glvKV("n", len(sread.OutOfSet), "first", first)})
	}

	// ── 输出 ──
	if jsonOut {
		sortedRules := func(notes []*glvNote) []string {
			m := map[string]bool{}
			for _, n := range notes {
				m[n.Rule] = true
			}
			var out []string
			for k := range m {
				out = append(out, k)
			}
			sort.Strings(out)
			return out
		}
		items := []any{}
		for _, r := range sortedRules(reds) {
			items = append(items, glvKV("rule", r, "state", "red"))
		}
		for _, r := range sortedRules(problems) {
			items = append(items, glvKV("rule", r, "state", "problem"))
		}
		meta := glvObjNew()
		meta.set("source", src).set("docs_root", docs).set("root", root).set("export_bin", exportBin)
		meta.set("limits", glvKV("max_lines", glvMaxLines, "max_bytes", glvMaxBytes, "max_line_chars", glvMaxLineChars))
		meta.set("export", r1Export)
		meta.set("ledger", r1Ledger)
		meta.set("history_ledgers", len(history)).set("exempt_ledgers", len(exempt))
		var indexRel any
		if led != nil {
			indexRel = led.Rel
		}
		var familyIDsVal any
		if len(problems) == 0 {
			familyIDsVal = len(famIDs)
		}
		meta.set("ledger_family", glvKV("index", indexRel, "pages", len(pages),
			"family_ids", familyIDsVal, "orphan_pages", len(fam.OrphanPages)))
		meta.set("fingerprint", glvKV("export_fp", glvNilIfEmpty(expFP), "ledger_fp", glvNilIfEmpty(ledFP),
			"ledger_generated", ledGenerated, "verdict", verdict))
		meta.set("generated_taken_over", ledGenerated)
		doc := glvObjNew()
		doc.set("schema", "zerg/v1").set("kind", "GapLedgerView").set("tool", glvVersion)
		doc.set("items", items)
		doc.set("meta", meta)
		doc.set("reds", glvNotesArr(reds)).set("problems", glvNotesArr(problems)).set("reports", glvNotesArr(reports))
		doc.set("warnings", []any{}).set("truncated", false)
		_ = r1Src
		fmt.Fprintln(stdout, glvJSONDump(doc))
		if len(problems) > 0 {
			return 2
		}
		if len(reds) > 0 {
			return 1
		}
		return 0
	}

	fmt.Fprintf(stdout, "── 缺口账「视图分离」三方对拍 + 体积/单行 + 指纹写保护（%s · 设计稿 v1.3 §0 ①）──\n", glvVersion)
	fmt.Fprintf(stdout, "  真源    : %s\n", src)
	fmt.Fprintf(stdout, "  导出器  : %s %s（**现跑**）\n", exportBin, strings.Join(glvExportArgs, " "))
	fmt.Fprintf(stdout, "  文档仓  : %s\n", docs)
	curRel := "（一件都没有）"
	if led != nil {
		curRel = led.Rel
	}
	fmt.Fprintf(stdout, "  当期索引件: %s\n", curRel)
	fmt.Fprintf(stdout, "  页件    : %d 件（历史件 %d · 豁免件 %d · 非族页件 %d）\n",
		len(pages), len(history), len(exempt), len(fam.OrphanPages))
	if len(problems) == 0 {
		fmt.Fprintf(stdout, "  ── R1 三方对拍 ──\n")
		fmt.Fprintf(stdout, "     真源    : %d 条 · 未闭 %d · 已闭 %d\n", len(sread.Recs), len(sread.Open), len(sread.Closed))
		fmt.Fprintf(stdout, "     导出现跑: 件头 %d 条 · 未闭 %d · 已闭 %d（不做 %d）· 正文 %d 条 · 件尾未闭 %d\n",
			exp.Total, exp.Open, exp.Closed, exp.NoDecide, len(exp.Bullets), exp.FootOpen)
		genFlag := "**否**✗"
		for _, mk := range glvGeneratedMarkers {
			if strings.Contains(led.Text, mk) {
				genFlag = "是"
			}
		}
		fmt.Fprintf(stdout, "     账件    : %s（生成件=%s）· 行 %d · 字节 %d · id %d 个\n",
			led.Rel, genFlag, led.Lines, led.Bytes, len(led.IDs))
		sigma := 0
		for _, fl := range famLedList {
			if fl.X.Kind == "page" {
				sigma += len(fl.S.IDs)
			}
		}
		fmt.Fprintf(stdout, "     件族    : 索引 1 + 页 %d ⇒ 族内 id 并集 %d 个 · Σ页条目 %d\n",
			len(pages), len(famIDs), sigma)
		fmt.Fprintf(stdout, "  ── R2 体积（逐件）──\n")
		for _, fl := range famLedList {
			lc, bc := "✓", "✓"
			if fl.S.Lines > glvMaxLines {
				lc = "**超**✗"
			}
			if fl.S.Bytes > glvMaxBytes {
				bc = "**超**✗"
			}
			fmt.Fprintf(stdout, "     [%s] 行数 %d / 上限 %d %s · 字节 %d / 上限 %d %s\n",
				fl.S.Rel, fl.S.Lines, glvMaxLines, lc, fl.S.Bytes, glvMaxBytes, bc)
		}
		fmt.Fprintf(stdout, "  ── R3 单行（逐件）──\n")
		for _, fl := range famLedList {
			lc := "✓"
			if fl.S.MaxLineLen > glvMaxLineChars {
				lc = "**超**✗"
			}
			fmt.Fprintf(stdout, "     [%s] 最长行 %d 字符（第 %d 行）/ 上限 %d %s\n",
				fl.S.Rel, fl.S.MaxLineLen, fl.S.MaxLineNo, glvMaxLineChars, lc)
		}
		fmt.Fprintf(stdout, "  ── R4 写保护 ──\n")
		efp, lfp := "**无**", "**无**"
		if expFP != "" {
			efp = expFP
		}
		if ledFP != "" {
			lfp = ledFP
		}
		fmt.Fprintf(stdout, "     导出产出指纹: %s · 账件指纹: %s\n", efp, lfp)
		fmt.Fprintf(stdout, "     结论: %s\n", verdict)
	}
	if len(reports) > 0 {
		fmt.Fprintf(stdout, "  ── 只报告（**不入退码** ✗）──\n")
		for _, r := range reports {
			fmt.Fprintf(stdout, "     · [%s] %s\n", r.Rule, r.What)
		}
	}
	if len(problems) > 0 {
		fmt.Fprintf(stdout, "⛔ 不给结论（rc=2）：\n")
		for _, p := range problems {
			fmt.Fprintf(stdout, "   · [%s] %s\n", p.Rule, p.What)
			if rd, ok := p.Read.(*glvJObj); ok {
				for i, k := range rd.keys {
					if k == "path" {
						fmt.Fprintf(stdout, "       盘面：%s\n", rd.vals[i])
					}
					if k == "msg" {
						if s, ok := rd.vals[i].(string); ok {
							fmt.Fprintf(stdout, "       原样：%s\n", glvDash(s, 120))
						}
					}
				}
			}
		}
		return 2
	}
	if len(reds) > 0 {
		fmt.Fprintf(stdout, "✗ 判红 %d 处（rc=1 · 逐条点名）：\n", len(reds))
		for _, r := range reds {
			fmt.Fprintf(stdout, "   · [%s] %s\n", r.Rule, r.What)
			for _, n := range r.Name {
				fmt.Fprintf(stdout, "       %s\n", n)
			}
		}
		fmt.Fprintf(stdout, "⇒ 处置（**不是**改本门）：① 三方对拍红 ⇒ 那是 ③（`zerg gap export` 接管结账单生成）的活；② 体积/单行红 ⇒ 生成件由 export 产出（它已自带 ≤200 单行纪律）\n")
		return 1
	}
	ledRel := "-"
	if led != nil {
		ledRel = led.Rel
	}
	fmt.Fprintf(stdout, "⇒ 三方一致 + 体积/单行合规（rc=0）· 件 `%s`\n", ledRel)
	fmt.Fprintf(stdout, "   ★ 本件只**读**：真源、账件一个字未改（写它们 = 别的笔的活）\n")
	return 0
}

func glvNotesArr(notes []*glvNote) []any {
	out := []any{}
	for _, n := range notes {
		out = append(out, n.toJSON())
	}
	return out
}

func glvNilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func glvFirst16(s string) string {
	if len(s) > 16 {
		return s[:16]
	}
	return s
}

// glvRunExport —— 现跑导出器：环境里把 ZERG_STATE_DIR 钉到真源所在目录。
func glvRunExport(cmd []string, stateDir, cwd string) (string, int, string, string) {
	c := exec.Command(cmd[0], cmd[1:]...)
	c.Dir = cwd
	env := os.Environ()
	env = append(env, "ZERG_STATE_DIR="+stateDir)
	c.Env = env
	var out, errb strings.Builder
	c.Stdout = &out
	c.Stderr = &errb
	done := make(chan error, 1)
	if err := c.Start(); err != nil {
		return "no_bin", 0, err.Error(), ""
	}
	go func() { done <- c.Wait() }()
	select {
	case err := <-done:
		rc := 0
		if err != nil {
			if ee, ok := err.(*exec.ExitError); ok {
				rc = ee.ExitCode()
			} else {
				return "no_bin", 0, err.Error(), ""
			}
		}
		if rc == 0 {
			return "ok", 0, "", out.String()
		}
		se := errb.String()
		if len(se) > 400 {
			se = se[len(se)-400:]
		}
		return "rc", rc, se, out.String()
	case <-time.After(glvExportTimeout * time.Second):
		_ = c.Process.Kill()
		return "timeout", 0, "", ""
	}
}

func glvListRules(stdout io.Writer, exportBin string) int {
	fmt.Fprintf(stdout, "── %s 判据 / 登记面 / 豁免 / 上限 ──\n", glvVersion)
	fmt.Fprintf(stdout, "  R1 三方对拍（判红）：真源(jsonl 逐行 id+state) ↔ `zerg gap export` 现跑 ↔ 当期结转账件\n")
	fmt.Fprintf(stdout, "  R2 体积（判红）：当期账件 ≤ %d 行 · ≤ %d 字节\n", glvMaxLines, glvMaxBytes)
	fmt.Fprintf(stdout, "  R3 单行（判红）：当期账件最长行 ≤ %d 字符（打行号+长度）\n", glvMaxLineChars)
	fmt.Fprintf(stdout, "  R4 写保护（有指纹则判红 · 无则只报告）：导出产出指纹 ⟷ 生成件指纹 逐字比\n")
	fmt.Fprintf(stdout, "  R5 缺件/空转（rc=2 · 不许当绿）：真源不可读/0 条 · export 跑不动 · 产出一件账都没有\n")
	fmt.Fprintf(stdout, "  R6 旧版账（只报告）：非当期 `%s/<版>/%s` ⇒ 列行数/字节/最长行，不入退码\n",
		glvLedgerDir, "缺口总账-<日期>.md")
	fmt.Fprintf(stdout, "  登记面：真源 `%s`（状态目录缺省 `%s`）\n", glvSourceName, glvStateDirDefault())
	fmt.Fprintf(stdout, "     态闭集: 未闭 %s ｜ 已闭 %s\n", strings.Join(glvOpenStates, "/"), strings.Join(glvClosedStates, "/"))
	fmt.Fprintf(stdout, "     面闭集: %s\n", strings.Join(glvImpactClosed, " / "))
	fmt.Fprintf(stdout, "     prio  : %s\n", strings.Join(glvPrioClosed, " / "))
	fmt.Fprintf(stdout, "     生成标记（逐字句 · 现读 export 人面第 3 行）: %s\n", glvGeneratedMarker)
	fmt.Fprintf(stdout, "  豁免（**只此一条**）：路径段命中 %s ⇒ 历史件只报告\n", strings.Join(glvExemptPathSegs, " / "))
	var pats []string
	pats = append(pats, glvFingerprintPatterns...)
	fmt.Fprintf(stdout, "  指纹载体（现读，未发明格式）：%s\n", strings.Join(pats, " ｜ "))
	fmt.Fprintf(stdout, "  导出器（只认本门隔壁那一个）：%s %s\n", exportBin, strings.Join(glvExportArgs, " "))
	fmt.Fprintf(stdout, "  退码：0 三方一致且合规 · 1 不一致/超限/指纹不符 · 2 不给结论（缺件/空转/导出跑不动/自检不过）\n")
	return 0
}

// ═════════════════════════════════════ 自检（成对负控 · 合成夹具 · 不碰真源/真账）══════════════════

const glvFixtureExportPy = `# -*- coding: utf-8 -*-
"""合成导出器（自检专用 · 读 <ZERG_STATE_DIR>/export-fixture.md 并照退码文件退出）。"""
import os, sys
d = os.environ.get("ZERG_STATE_DIR", "")
rcf = os.path.join(d, "export-rc")
rc = int(open(rcf).read().strip()) if os.path.exists(rcf) else 0
p = os.path.join(d, "export-fixture.md")
if os.path.exists(p):
    sys.stdout.write(open(p, encoding="utf-8").read())
if rc:
    sys.stderr.write("fixture export 故意失败 rc=%d\n" % rc)
sys.exit(rc)
`

func glvMkExportMd(items []glvFixtureRec, total, closed, noDecide, lineMax int) string {
	per := map[string]int{}
	for _, i := range items {
		per[i.Impact]++
	}
	var out []string
	out = append(out, "## 缺口真源导出（**只列未闭**） · 2026-09-26", "",
		"> 生成件：由 `zerg gap export` 从真源 `/x/zerg-cli-gaps.jsonl` 生成 —— **不许手改** ✗（手改会被门禁判红）。",
		fmt.Sprintf("> 真源条数 **%d** · 未闭 **%d** · 已闭 **%d**（其中「不做」**%d**）", total, len(items), closed, noDecide))
	var faceParts []string
	for _, f := range glvImpactClosed {
		faceParts = append(faceParts, fmt.Sprintf("%s %d", f, per[f]))
	}
	out = append(out, "> 按面："+strings.Join(faceParts, " · "), "")
	for _, f := range glvImpactClosed {
		var grp []glvFixtureRec
		for _, i := range items {
			if i.Impact == f {
				grp = append(grp, i)
			}
		}
		if len(grp) == 0 {
			continue
		}
		out = append(out, fmt.Sprintf("### %s（%d 条）", f, len(grp)), "")
		for _, i := range grp {
			ln := fmt.Sprintf("- `%s` %s · **%s** · %s · 想要 `w` · 判据 `zerg task ls`", i.ID, i.Prio, i.State, i.Symptom)
			if utf8.RuneCountInString(ln) <= lineMax {
				out = append(out, ln)
			} else {
				rn := []rune(ln)
				out = append(out, string(rn[:lineMax-1])+"…")
			}
		}
		out = append(out, "")
	}
	out = append(out, fmt.Sprintf("（本片段由 CLI 生成 · 未闭 %d 条 · 单行上限 %d 字符）", len(items), lineMax))
	return strings.Join(out, "\n") + "\n"
}

type glvFixtureRec struct {
	ID, Impact, Prio, State, Symptom string
}

func glvMkRec(id string) glvFixtureRec {
	return glvFixtureRec{ID: id, Impact: "命令面", Prio: "P0", State: "仍缺", Symptom: "s"}
}

func glvFixtureSrcLine(r glvFixtureRec) string {
	o := glvObjNew()
	o.set("id", r.ID).set("fp", "fp"+glvTail2(r.ID)).set("handmade", "").set("impact", r.Impact)
	o.set("prio", r.Prio).set("state", r.State).set("symptom", r.Symptom)
	o.set("want_action", "w").set("want_family", "w").set("verify_cmd", "zerg task ls")
	o.set("repro_cmd", "").set("found_at", "2026-09-26T00:00:00+08:00")
	o.set("last_verified_at", "").set("solved_at", "").set("solved_evidence", "")
	return glvJSONDump(o)
}

func glvTail2(s string) string {
	rn := []rune(s)
	if len(rn) < 2 {
		return s
	}
	return string(rn[len(rn)-2:])
}

type glvSelfTest struct {
	tmp     string
	fixture string
	stdout  io.Writer
	ok      bool
	nCase   int
	python  string
}

func (t *glvSelfTest) mk(srcRecs []glvFixtureRec, expMd string, hasExp bool, ledLines []string, hasLed bool, history [][2]any, exportRC int, ledName, defaultRoot, exportBin string) (string, string, string) {
	t.nCase++
	base := glvPyJoin(t.tmp, fmt.Sprintf("case-%02d", t.nCase))
	repo := glvPyJoin(base, "repo")
	_ = os.MkdirAll(glvPyJoin(repo, "scripts", "gates"), 0o755)
	state := glvPyJoin(base, "state")
	_ = os.MkdirAll(state, 0o755)
	var srcLines []string
	for _, r := range srcRecs {
		srcLines = append(srcLines, glvFixtureSrcLine(r))
	}
	_ = os.WriteFile(glvPyJoin(state, glvSourceName), []byte(strings.Join(srcLines, "\n")+"\n"), 0o644)
	if len(srcRecs) == 0 {
		_ = os.WriteFile(glvPyJoin(state, glvSourceName), []byte(""), 0o644)
	}
	_ = os.WriteFile(glvPyJoin(state, "export-rc"), []byte(strconv.Itoa(exportRC)), 0o644)
	if hasExp {
		_ = os.WriteFile(glvPyJoin(state, "export-fixture.md"), []byte(expMd), 0o644)
	}
	docs := glvPyJoin(base, "Zerg-内部文档")
	vdir := glvPyJoin(docs, glvLedgerDir, "vX")
	_ = os.MkdirAll(vdir, 0o755)
	if hasLed {
		_ = os.WriteFile(glvPyJoin(vdir, ledName), []byte(strings.Join(ledLines, "\n")+"\n"), 0o644)
	}
	for _, h := range history {
		rel := h[0].(string)
		lines := h[1].([]string)
		hp := glvPyJoin(docs, glvLedgerDir, rel)
		_ = os.MkdirAll(glvPyDirname(hp), 0o755)
		_ = os.WriteFile(hp, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
	}
	return repo, docs, glvPyJoin(state, glvSourceName)
}

func (t *glvSelfTest) cap(r, d, s string, defaultRoot, exportBin string) (int, string) {
	var buf strings.Builder
	rc := glvRun(r, d, s, false, []string{t.python, t.fixture}, defaultRoot, exportBin, &buf)
	return rc, buf.String()
}

func (t *glvSelfTest) kase(name string, wantRC, gotRC int, extraOK bool, note string) {
	good := (wantRC == gotRC) && extraOK
	t.ok = t.ok && good
	mark := "✓"
	if !good {
		mark = "✗"
	}
	fmt.Fprintf(t.stdout, "  %s %s（rc=%d · 期望 %d）%s\n", mark, name, gotRC, wantRC, note)
}

// kaseB —— py `case(name, want_rc=True, got_rc=<bool expr>)` 的布尔形态（逐字保留 `%s` 打 True/False）。
func (t *glvSelfTest) kaseB(name string, want, got bool) {
	good := want == got
	t.ok = t.ok && good
	mark := "✓"
	if !good {
		mark = "✗"
	}
	pyb := func(b bool) string {
		if b {
			return "True"
		}
		return "False"
	}
	fmt.Fprintf(t.stdout, "  %s %s（rc=%s · 期望 %s）\n", mark, name, pyb(got), pyb(want))
}

func glvLedOf(items []glvFixtureRec, marker bool, extra []string) []string {
	body := glvSplitlines(glvMkExportMd(items, len(items)+1, 1, 0, 200))
	line2 := "> 生成件：由 `zerg gap export` 从真源 `/x/zerg-cli-gaps.jsonl` 生成 —— **不许手改** ✗（手改会被门禁判红）。"
	if marker {
		body[2] = line2
	} else {
		body[2] = "> 说明：本件手写。"
	}
	return append(body, extra...)
}

func glvIndexMd(items []glvFixtureRec, pages [][2]any, marker bool) []string {
	per := map[string]int{}
	for _, i := range items {
		per[i.Impact]++
	}
	var head []string
	if marker {
		head = []string{"> 生成件：由 `zerg gap export` 从真源 `/x/zerg-cli-gaps.jsonl` 生成 —— **不许手改** ✗（手改会被门禁判红）。"}
	} else {
		head = []string{"> 说明：本件手写。"}
	}
	out := []string{"## 缺口真源导出（**只列未闭**） · 2026-09-27", ""}
	out = append(out, head...)
	out = append(out,
		fmt.Sprintf("> 真源条数 **%d** · 未闭 **%d** · 已闭 **%d**（其中「不做」**%d**）", len(items)+1, len(items), 1, 0))
	var faceParts []string
	for _, f := range glvImpactClosed {
		faceParts = append(faceParts, fmt.Sprintf("%s %d", f, per[f]))
	}
	out = append(out, "> 按面："+strings.Join(faceParts, " · "),
		"> 拆件：一级 = 面 · 二级 = 每页 120 条 · 页件名 `缺口总账-<日期>-<面>-pNN.md`", "")
	for _, p := range pages {
		face := p[0].(string)
		cnt := p[1].(int)
		out = append(out, fmt.Sprintf("### %s（%d 条）", face, cnt), "",
			fmt.Sprintf("- 第 1 页：`缺口总账-2026-09-27-%s-p01.md` · 面内 1–%d 条", face, cnt))
	}
	return out
}

func glvPageMd(items []glvFixtureRec, face string, pno int, total int) []string {
	out := []string{fmt.Sprintf("# %s · 缺口总账 2026-09-27", face), "",
		fmt.Sprintf("> 第 %d 页·本页 %d 条·面内 1–%d 条·本面共 %d 条", pno, len(items), len(items), total),
		"> 真源 = `/x/zerg-cli-gaps.jsonl` · 面内顺序 = `(prioRank, id)` 稳定序 · 单行上限 200 字符 · 生成件**不许手改** ✗", ""}
	for _, i := range items {
		out = append(out, fmt.Sprintf("- `%s` %s · **%s** · %s", i.ID, i.Prio, i.State, i.Symptom))
	}
	return out
}

func glvMkFamily(t *glvSelfTest, srcRecs []glvFixtureRec, expMd string, indexLines []string, pageFiles [][2]any, exportRC int) (string, string, string) {
	t.nCase++
	base := glvPyJoin(t.tmp, fmt.Sprintf("case-%02d", t.nCase))
	repo := glvPyJoin(base, "repo")
	_ = os.MkdirAll(glvPyJoin(repo, "scripts", "gates"), 0o755)
	state := glvPyJoin(base, "state")
	_ = os.MkdirAll(state, 0o755)
	var srcLines []string
	for _, r := range srcRecs {
		srcLines = append(srcLines, glvFixtureSrcLine(r))
	}
	_ = os.WriteFile(glvPyJoin(state, glvSourceName), []byte(strings.Join(srcLines, "\n")+"\n"), 0o644)
	_ = os.WriteFile(glvPyJoin(state, "export-rc"), []byte(strconv.Itoa(exportRC)), 0o644)
	_ = os.WriteFile(glvPyJoin(state, "export-fixture.md"), []byte(expMd), 0o644)
	docs := glvPyJoin(base, "Zerg-内部文档")
	vdir := glvPyJoin(docs, glvLedgerDir, "vX")
	_ = os.MkdirAll(vdir, 0o755)
	_ = os.WriteFile(glvPyJoin(vdir, "缺口总账-2026-09-27.md"), []byte(strings.Join(indexLines, "\n")+"\n"), 0o644)
	for _, pf := range pageFiles {
		fn := pf[0].(string)
		lines := pf[1].([]string)
		_ = os.WriteFile(glvPyJoin(vdir, fn), []byte(strings.Join(lines, "\n")+"\n"), 0o644)
	}
	return repo, docs, glvPyJoin(state, glvSourceName)
}

// glvSelfTestRun —— 与 py `self_test()` 逐格同构（17 格）。defaultRoot/exportBin 由调用者给（Go 无 __file__）。
func glvSelfTestRun(stdout io.Writer, python, defaultRoot, exportBin string) int {
	t := &glvSelfTest{stdout: stdout, ok: true, python: python}
	tmp, err := os.MkdirTemp("", "glv-selftest-")
	if err != nil {
		fmt.Fprintf(stdout, "自检夹具建不了：%s\n", err)
		return 1
	}
	defer os.RemoveAll(tmp) // ★ 止血 py 的 A 类 tempdir 泄漏
	t.tmp = tmp
	t.fixture = glvPyJoin(tmp, "fixture-export.py")
	_ = os.WriteFile(t.fixture, []byte(glvFixtureExportPy), 0o644)

	A := glvMkRec("GAP-20260926-01")
	A.Symptom = "甲"
	B := glvMkRec("GAP-20260926-02")
	B.State, B.Symptom = "已解", "乙"
	expAb := glvMkExportMd([]glvFixtureRec{A}, 2, 1, 0, 200)

	// ① 正控
	r, d, s := t.mk([]glvFixtureRec{A, B}, expAb, true, glvLedOf([]glvFixtureRec{A}, true, nil), true, nil, 0, "缺口总账-20260926.md", defaultRoot, exportBin)
	rc, out := t.cap(r, d, s, defaultRoot, exportBin)
	t.kase("① 正控：三方一致（生成件接管）⇒ rc=0", 0, rc, strings.Contains(out, "三方一致"), "")
	t.kaseB("⑩ R4 只报告：无指纹 ⇒ 不入退码", true, strings.Contains(out, "无指纹 ⇒ 无法判手改") && rc == 0)

	// ② 负控 R1a
	C := glvMkRec("GAP-20260926-03")
	C.Symptom = "丙"
	r2, d2, s2 := t.mk([]glvFixtureRec{A, C, B}, expAb, true, glvLedOf([]glvFixtureRec{A}, true, nil), true, nil, 0, "缺口总账-20260926.md", defaultRoot, exportBin)
	rc, out = t.cap(r2, d2, s2, defaultRoot, exportBin)
	t.kase("② 负控 R1a：真源未闭 2 条 vs 导出 1 条 ⇒ rc=1", 1, rc, true, "")
	t.kaseB("②b 点名到「缺哪个」（`GAP-20260926-03`）", true, strings.Contains(out, "`GAP-20260926-03`") && strings.Contains(out, "[R1a]"))
	// ②c 真话负控
	expAbc := glvMkExportMd([]glvFixtureRec{A, C}, 3, 1, 0, 200)
	r2c, d2c, s2c := t.mk([]glvFixtureRec{A, C, B}, expAbc, true, glvLedOf([]glvFixtureRec{A, C}, true, nil), true, nil, 0, "缺口总账-20260926.md", defaultRoot, exportBin)
	rc, out = t.cap(r2c, d2c, s2c, defaultRoot, exportBin)
	t.kase("②c 真话负控（只把导出补齐）⇒ 回绿", 0, rc, strings.Contains(out, "三方一致"), "")
	// ③
	r3, d3, s3 := t.mk([]glvFixtureRec{A, B}, glvMkExportMd([]glvFixtureRec{A, C}, 3, 1, 0, 200), true, glvLedOf([]glvFixtureRec{A, C}, true, nil), true, nil, 0, "缺口总账-20260926.md", defaultRoot, exportBin)
	rc, out = t.cap(r3, d3, s3, defaultRoot, exportBin)
	t.kase("③ 负控 R1a：导出正文多一条 ⇒ rc=1 点名「多」", 1, rc, strings.Contains(out, "多") && strings.Contains(out, "`GAP-20260926-03`"), "")
	// ④
	A2 := A
	A2.State = "已派"
	expSt := glvMkExportMd([]glvFixtureRec{A2}, 2, 1, 0, 200)
	A3 := A
	A3.State = "已派"
	r4, d4, s4 := t.mk([]glvFixtureRec{A, B}, expSt, true, glvLedOf([]glvFixtureRec{A3}, true, nil), true, nil, 0, "缺口总账-20260926.md", defaultRoot, exportBin)
	rc, out = t.cap(r4, d4, s4, defaultRoot, exportBin)
	t.kase("④ 负控 R1a：状态不一致（仍缺 vs 已派）⇒ rc=1", 1, rc, strings.Contains(out, "状态不一致"), "")
	// ⑤
	r5, d5, s5 := t.mk([]glvFixtureRec{A, B}, expAb, true, glvLedOf([]glvFixtureRec{A}, false, nil), true, nil, 0, "缺口总账-20260926.md", defaultRoot, exportBin)
	rc, out = t.cap(r5, d5, s5, defaultRoot, exportBin)
	t.kase("⑤ 负控 R1b：账件缺生成标记 ⇒ rc=1「生成件未接管」", 1, rc, strings.Contains(out, "生成件未接管"), "")
	// ⑥
	longLed := append(glvLedOf([]glvFixtureRec{A}, true, nil), func() []string {
		var z []string
		for i := 0; i < 305; i++ {
			z = append(z, fmt.Sprintf("<!-- pad %d -->", i))
		}
		return z
	}()...)
	r6, d6, s6 := t.mk([]glvFixtureRec{A, B}, expAb, true, longLed, true, nil, 0, "缺口总账-20260926.md", defaultRoot, exportBin)
	rc, out = t.cap(r6, d6, s6, defaultRoot, exportBin)
	t.kase("⑥ 负控 R2：行数 300+ ⇒ rc=1（逐字打实际值）", 1, rc, strings.Contains(out, "[R2]") && strings.Contains(out, "行数") && strings.Contains(out, "上限 300"), "")
	// ⑥b
	fat := append(glvLedOf([]glvFixtureRec{A}, true, nil), func() []string {
		var z []string
		for i := 0; i < 520; i++ {
			z = append(z, "<!-- "+strings.Repeat("x", 120)+" -->")
		}
		return z
	}()...)
	r6b, d6b, s6b := t.mk([]glvFixtureRec{A, B}, expAb, true, fat, true, nil, 0, "缺口总账-20260926.md", defaultRoot, exportBin)
	rc, out = t.cap(r6b, d6b, s6b, defaultRoot, exportBin)
	t.kase("⑥b 负控 R2：字节 60KB+ ⇒ rc=1", 1, rc, strings.Contains(out, fmt.Sprintf("上限 %d", glvMaxBytes)), "")
	// ⑦
	r3l := append(glvLedOf([]glvFixtureRec{A}, true, nil), "<!-- "+strings.Repeat("y", 201)+" -->")
	r7, d7, s7 := t.mk([]glvFixtureRec{A, B}, expAb, true, r3l, true, nil, 0, "缺口总账-20260926.md", defaultRoot, exportBin)
	rc, out = t.cap(r7, d7, s7, defaultRoot, exportBin)
	t.kase("⑦ 负控 R3：最长行 210 字符 ⇒ rc=1（行号+长度）", 1, rc, strings.Contains(out, "[R3]") && strings.Contains(out, "最长行 210 字符") && strings.Contains(out, "第 12 行"), "")
	// ⑧
	expFp := expAb + "\n> 生成指纹 sha256:deadbeefdeadbeef\n"
	r8, d8, s8 := t.mk([]glvFixtureRec{A, B}, expFp, true, glvLedOf([]glvFixtureRec{A}, true, nil), true, nil, 0, "缺口总账-20260926.md", defaultRoot, exportBin)
	rc, out = t.cap(r8, d8, s8, defaultRoot, exportBin)
	t.kase("⑧ 负控 R4：导出带指纹而生成件缺 ⇒ rc=1", 1, rc, strings.Contains(out, "[R4]"), "")
	r8b, d8b, s8b := t.mk([]glvFixtureRec{A, B}, expFp, true, glvLedOf([]glvFixtureRec{A}, true, []string{"> 生成指纹 sha256:cafebabe"}), true, nil, 0, "缺口总账-20260926.md", defaultRoot, exportBin)
	rc, out = t.cap(r8b, d8b, s8b, defaultRoot, exportBin)
	t.kase("⑧b 负控 R4：指纹不符 ⇒ rc=1", 1, rc, strings.Contains(out, "指纹不符"), "")
	r8c, d8c, s8c := t.mk([]glvFixtureRec{A, B}, expFp, true, glvLedOf([]glvFixtureRec{A}, true, []string{"> 生成指纹 sha256:deadbeefdeadbeef"}), true, nil, 0, "缺口总账-20260926.md", defaultRoot, exportBin)
	rc, out = t.cap(r8c, d8c, s8c, defaultRoot, exportBin)
	t.kase("⑧c 正控 R4：指纹逐字一致 ⇒ rc=0", 0, rc, strings.Contains(out, "指纹逐字一致"), "")
	// ⑨
	r9, d9, s9 := t.mk([]glvFixtureRec{A}, expAb, true, glvLedOf([]glvFixtureRec{A}, true, nil), true, nil, 0, "缺口总账-20260926.md", defaultRoot, exportBin)
	_ = os.Remove(s9)
	rc, out = t.cap(r9, d9, s9, defaultRoot, exportBin)
	t.kase("⑨ 负控 R5：真源不在盘 ⇒ rc=2（不拿读不到当没问题）", 2, rc, strings.Contains(out, "不给结论"), "")
	r9b, d9b, s9b := t.mk([]glvFixtureRec{}, expAb, true, glvLedOf([]glvFixtureRec{A}, true, nil), true, nil, 0, "缺口总账-20260926.md", defaultRoot, exportBin)
	rc, out = t.cap(r9b, d9b, s9b, defaultRoot, exportBin)
	t.kase("⑨b 负控 R5：真源 0 条（空转）⇒ rc=2", 2, rc, strings.Contains(out, "不给结论"), "")
	r9c, d9c, s9c := t.mk([]glvFixtureRec{A}, expAb, true, glvLedOf([]glvFixtureRec{A}, true, nil), true, nil, 3, "缺口总账-20260926.md", defaultRoot, exportBin)
	rc, out = t.cap(r9c, d9c, s9c, defaultRoot, exportBin)
	t.kase("⑨c 负控 R5：导出跑不动（rc=3）⇒ rc=2（不许当绿）", 2, rc, strings.Contains(out, "不给结论"), "")
	r9d, d9d, s9d := t.mk([]glvFixtureRec{A}, expAb, true, nil, false, nil, 0, "缺口总账-20260926.md", defaultRoot, exportBin)
	rc, out = t.cap(r9d, d9d, s9d, defaultRoot, exportBin)
	t.kase("⑨d 负控 R5：文档仓一件账都没有 ⇒ rc=2", 2, rc, strings.Contains(out, "不给结论"), "")
	r9e, d9e, s9e := t.mk([]glvFixtureRec{A}, "这不是导出片段", true, glvLedOf([]glvFixtureRec{A}, true, nil), true, nil, 0, "缺口总账-20260926.md", defaultRoot, exportBin)
	rc, out = t.cap(r9e, d9e, s9e, defaultRoot, exportBin)
	t.kase("⑨e 负控 R5：产出解析不出 ⇒ rc=2", 2, rc, strings.Contains(out, "解析不出"), "")
	// ⑪
	hist := [][2]any{{"vOld/缺口总账-20260101.md", func() []string {
		z := []string{"# 旧账"}
		for i := 0; i < 500; i++ {
			z = append(z, fmt.Sprintf("| %d |", i))
		}
		return z
	}()}}
	r11, d11, s11 := t.mk([]glvFixtureRec{A, B}, expAb, true, glvLedOf([]glvFixtureRec{A}, true, nil), true, hist, 0, "缺口总账-20260926.md", defaultRoot, exportBin)
	rc, out = t.cap(r11, d11, s11, defaultRoot, exportBin)
	t.kase("⑪ 只报告 R6：历史账 500 行 ⇒ 仍 rc=0（不入退码）", 0, rc, strings.Contains(out, "[R6]"), "")
	// ⑫
	proseLed := glvLedOf([]glvFixtureRec{A}, true, []string{fmt.Sprintf("- `%s` P1 · **仍缺** · 与 `Q-999` 同根（散文引用 · 非条目）", A.ID)})
	r12, d12, s12 := t.mk([]glvFixtureRec{A, B}, expAb, true, proseLed, true, nil, 0, "缺口总账-20260926.md", defaultRoot, exportBin)
	rc, out = t.cap(r12, d12, s12, defaultRoot, exportBin)
	t.kase("⑫ 正控 R1b：散文引用的 `Q-999` 不算条目 ⇒ rc=0", 0, rc, strings.Contains(out, "三方一致") && !strings.Contains(out, "Q-999"), "")
	// ⑬
	markLed := glvLedOf([]glvFixtureRec{A}, true, []string{"- `Q-999` P1 · **仍缺** · 作为条目标记出现（真源无此号）"})
	r13, d13, s13 := t.mk([]glvFixtureRec{A, B}, expAb, true, markLed, true, nil, 0, "缺口总账-20260926.md", defaultRoot, exportBin)
	rc, out = t.cap(r13, d13, s13, defaultRoot, exportBin)
	t.kase("⑬ 负控 R1b：条目标记形式的 `Q-999` ⇒ rc=1 点名「多」", 1, rc, strings.Contains(out, "[R1b]") && strings.Contains(out, "`Q-999`") && strings.Contains(out, "多"), "")

	// ── 件族格 ──
	E := glvMkRec("GAP-20260927-01")
	E.Symptom = "戊"
	F := glvMkRec("GAP-20260927-02")
	F.Symptom = "己"
	G := glvMkRec("GAP-20260927-03")
	G.Impact, G.Symptom = "门禁面", "庚"
	famItems := []glvFixtureRec{E, F, G}
	expEfg := glvMkExportMd(famItems, 4, 1, 0, 200)
	idxLines := glvIndexMd(famItems, [][2]any{{"命令面", 2}, {"门禁面", 1}}, true)
	pCmd := glvPageMd([]glvFixtureRec{E, F}, "命令面", 1, 2)
	pDoc := glvPageMd([]glvFixtureRec{G}, "门禁面", 1, 1)
	famSrc := append(append([]glvFixtureRec{}, famItems...), B)
	// ⑭
	r14, d14, s14 := glvMkFamily(t, famSrc, expEfg, idxLines, [][2]any{
		{"缺口总账-2026-09-27-命令面-p01.md", pCmd}, {"缺口总账-2026-09-27-门禁面-p01.md", pDoc}}, 0)
	rc, out = t.cap(r14, d14, s14, defaultRoot, exportBin)
	t.kase("⑭ 正控 · 件族（索引+2 页 · 并集一致）⇒ rc=0", 0, rc, strings.Contains(out, "三方一致"), "")
	// ⑮
	r15, d15, s15 := glvMkFamily(t, famSrc, expEfg, idxLines, [][2]any{
		{"缺口总账-2026-09-27-命令面-p01.md", glvPageMd([]glvFixtureRec{E}, "命令面", 1, 2)},
		{"缺口总账-2026-09-27-门禁面-p01.md", pDoc}}, 0)
	rc, out = t.cap(r15, d15, s15, defaultRoot, exportBin)
	t.kase("⑮ 负控 · 漏页（Σ页 2 ≠ 索引件头未闭 3）⇒ rc=1 + [R1d]", 1, rc, strings.Contains(out, "[R1d]") && strings.Contains(out, "[R1b]"), "")
	// ⑯
	r16, d16, s16 := glvMkFamily(t, famSrc, expEfg, idxLines, [][2]any{
		{"缺口总账-2026-09-27-命令面-p01.md", pCmd},
		{"缺口总账-2026-09-27-命令面-p02.md", glvPageMd([]glvFixtureRec{F, G}, "命令面", 2, 3)}}, 0)
	rc, out = t.cap(r16, d16, s16, defaultRoot, exportBin)
	t.kase("⑯ 负控 · 重页（并集仍一致 ⇒ 靠 R1c 抓）⇒ rc=1 + [R1c]", 1, rc,
		strings.Contains(out, "[R1c]") && !strings.Contains(out, "[R1b]") && strings.Contains(out, "[R1d]"), "")
	// ⑰
	r17, d17, s17 := glvMkFamily(t, famSrc, expEfg, idxLines, [][2]any{
		{"缺口总账-2026-09-27-命令面-p01.md", pCmd},
		{"缺口总账-2026-09-27-门禁面-p01.md", pDoc},
		{"缺口总账-2026-09-26-命令面-p01.md", glvPageMd([]glvFixtureRec{E, F}, "命令面", 1, 2)}}, 0)
	rc, out = t.cap(r17, d17, s17, defaultRoot, exportBin)
	t.kase("⑰ 三锁（日期锁）：异日期页件不入族 · 只报告 ⇒ rc=0 + [R6]", 0, rc, strings.Contains(out, "[R6]") && strings.Contains(out, "三方一致"), "")

	fmt.Fprintf(stdout, "自检格数: %d\n", t.nCase)
	if t.ok {
		fmt.Fprintf(stdout, "自检结论: 全过\n")
		return 0
	}
	fmt.Fprintf(stdout, "自检结论: **有断言失败**\n")
	return 1
}

// ═════════════════════════════════════ 入口（argparse 同构）════════════════════════════════════

const glvProg = "check-gap-ledger-view.py"

func glvUsage() string {
	return "usage: " + glvProg + " [-h] [--root ROOT] [--docs-root DOCS_ROOT]\n" +
		"                                [--source SOURCE] [--json] [--list-rules]\n" +
		"                                [--self-test] [--version]\n"
}

func glvHelp() string {
	return glvUsage() + "\n" +
		"缺口账视图分离：三方对拍 + 体积/单行纪律 + 指纹写保护（设计稿 v1.3 §0 ①）\n" +
		"\n" +
		"optional arguments:\n" +
		"  -h, --help            show this help message and exit\n" +
		"  --root ROOT           主仓仓根（默认真源：本脚本所在仓 · 只用来定位兄弟目录 Zerg-内部文档/）\n" +
		"  --docs-root DOCS_ROOT\n" +
		"                        文档仓根（默认：主仓的兄弟目录 Zerg-内部文档/）\n" +
		"  --source SOURCE       真源 jsonl（缺省 = <状态目录>/zerg-cli-gaps.jsonl）\n" +
		"  --json                机读输出（zerg/v1 形状）\n" +
		"  --list-rules\n" +
		"  --self-test\n" +
		"  --version             show program's version number and exit\n"
}

// gapLedgerViewGateMain —— 已落仓件的入口（stdout/stderr 可注入）。
func gapLedgerViewGateMain(stdout, stderr io.Writer, argv []string) int {
	for _, a := range argv {
		if a == "-h" || a == "--help" {
			fmt.Fprint(stdout, glvHelp())
			return 0
		}
	}
	for _, a := range argv {
		if a == "--version" {
			fmt.Fprintln(stdout, glvVersion)
			return 0
		}
	}
	var root, docsRoot, source string
	var jsonOut, listRules, selfTest bool
	var unknown []string
	i := 0
	for i < len(argv) {
		a := argv[i]
		var inline string
		var hasInline bool
		if strings.HasPrefix(a, "--") {
			if eq := strings.Index(a, "="); eq >= 0 {
				inline = a[eq+1:]
				hasInline = true
				a = a[:eq]
			}
		}
		switch a {
		case "--root", "--docs-root", "--source":
			var val string
			if hasInline {
				val = inline
			} else if i+1 < len(argv) {
				val = argv[i+1]
				i++
			} else {
				fmt.Fprint(stderr, glvUsage())
				fmt.Fprintf(stderr, "%s: error: argument %s: expected one argument\n", glvProg, a)
				return 2
			}
			switch a {
			case "--root":
				root = val
			case "--docs-root":
				docsRoot = val
			case "--source":
				source = val
			}
		case "--json":
			jsonOut = true
		case "--list-rules":
			listRules = true
		case "--self-test":
			selfTest = true
		default:
			unknown = append(unknown, argv[i])
		}
		i++
	}
	if len(unknown) > 0 {
		fmt.Fprint(stderr, glvUsage())
		fmt.Fprintf(stderr, "%s: error: unrecognized arguments: %s\n", glvProg, strings.Join(unknown, " "))
		return 2
	}

	defaultRoot := glvResolveRoot()
	exportBin := glvPyJoin(defaultRoot, "bin", "zerg")
	if selfTest {
		python := os.Getenv("GLV_PYTHON")
		if python == "" {
			python = "/usr/bin/python3"
		}
		return glvSelfTestRun(stdout, python, defaultRoot, exportBin)
	}
	if listRules {
		return glvListRules(stdout, exportBin)
	}
	if root == "" {
		root = defaultRoot
	}
	return glvRun(root, docsRoot, source, jsonOut, nil, defaultRoot, exportBin, stdout)
}
