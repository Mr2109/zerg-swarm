package main

// check-py-go-pairing-drift.py —— py⟷Go 双实现「两态对拍」防波门（**只读** · 棘轮只许降）· Go 真搬体。
//
// 本件是 `scripts/gates/check-py-go-pairing-drift.py`（778 行）的逐行真搬。口径与本仓「制品/子进程类判据」
// 一致：判据真起 `bin/zerg` 子进程，输入确定 ⇒ 输出逐字可对拍；`--zerg <制品>` 旋钮直接可用。
//
// 两条出厂声明（✗ 不声称逐字对齐的面）：
//  (a) 默认根定位（**已归一** · 2026-10-07）：py 从 `__file__` 上溯两层 = 仓根；Go 无 `__file__` 语境，
//      取**同族真搬件口径**（`check_partial_run_acceptance.go` 的 `praRepoRoot()` /
//      `check_fields_face_parity.go` 的 `ffpDefaultRoot()`）：env `ZERG_REPO_ROOT` 优先（转发壳
//      `gateCheckRunInproc` 在调 inproc 前必设它并把 cwd chdir 到仓根），否则 cwd 上溯找含 `scripts/gates`
//      的祖先；两路都定不到位 ⇒ fail-closed 退 2（✗ 不回落 cwd/`..` 猜）。⇒ cwd=仓根（或仓内任意层）·
//      或走 `zerg gate check <键>` 时，「无 --root」与「--root=<仓根>」两态读数逐字同（已对拍）。
//      残余未对齐面：仅当 cwd 在仓**外**且 env 未设时 Go fail-closed、而 py 仍按 `__file__` 得仓根。
//  (b) 自指检查：本件落在它自己扫描的面内？判「否」—— 被扫面只有 `core/cmd/zerg/gate_inproc_table.go`
//      （单件）、`scripts/gates/judge-registry.json`、`scripts/gates/py-go-pairing-drift.ratchet.json`、
//      以及回落档扫 `scripts/gates` 下的件；本件落在 `core/cmd/zerg/` 且扩展名非 `.py` ⇒ 不在任一被扫面内。
//
// 入口：`pgpdGateMain(stdout, stderr io.Writer, argv []string) int`（argv 不含程序名 · inproc 口径）。

import (
	"bytes"
	"context"
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
)

const (
	// pgpdVersion —— py `VERSION`（逐字）。
	pgpdVersion = "0.1.0（防波门 · 缺口承 zerg-gate-cli-pitfalls「双实现面只改半件＝假绿」）"
	// pgpdProg —— argparse 的 prog 名（`os.path.basename(sys.argv[0])` · 硬占位）。
	pgpdProg = "check-py-go-pairing-drift.py"

	pgpdInprocRel   = "core/cmd/zerg/gate_inproc_table.go"
	pgpdGatesRel    = pathScriptsGates
	pgpdRegistryRel = pathJudgeRegistry
	pgpdRatchetRel  = "scripts/gates/py-go-pairing-drift.ratchet.json"

	pgpdSchema       = "zerg/py-go-pairing-drift/v1"
	pgpdProbeTimeout = 120

	pgpdWriteClassHeadLines = 80
	pgpdWantCases           = 20
)

// pgpdHelpText —— argparse help（以 PATH `python3` = 3.9.x 为基准 · 见未覆盖面「解释器版本面」）。
// 3.10+ 会把小节标题 `optional arguments:` 印成 `options:` ⇒ 该字面属环境依赖面。
const pgpdHelpText = `usage: check-py-go-pairing-drift.py [-h] [--root ROOT] [--argv ARGV]
                                    [--zerg ZERG] [--python PYBIN] [--strict]
                                    [--json] [--dump-ratchet] [--only ONLY]
                                    [--self-test]

py⟷Go 双实现两态对拍防波门（只读 · 棘轮只许降）

optional arguments:
  -h, --help      show this help message and exit
  --root ROOT
  --argv ARGV     两态共用的 argv 串（按空格切；给空串 = 无参）
  --zerg ZERG     态① 启动器（缺省 <root>/bin/zerg）
  --python PYBIN  态② 解释器（缺省本进程 python）
  --strict        判红档：棘轮涨 / 表A 非空 ⇒ rc=1
  --json
  --dump-ratchet
  --only ONLY     只对拍指定键（可重复 · 调试用）
  --self-test
`

// pgpdUsageText —— argparse usage（报错时先打它 · 逐字）。
const pgpdUsageText = `usage: check-py-go-pairing-drift.py [-h] [--root ROOT] [--argv ARGV]
                                    [--zerg ZERG] [--python PYBIN] [--strict]
                                    [--json] [--dump-ratchet] [--only ONLY]
                                    [--self-test]
`

// inproc 表里的条目形态：`\t"<键>":   <Go 函数名>,`（★ 只认这一形态 · 表件里其余行不吃）
var pgpdKeyRe = regexp.MustCompile(`^\t"([^"]+)"\s*:\s*([A-Za-z_][A-Za-z0-9_]*)\s*,\s*$`)

// pgpdWriteClassRe —— 真身件头 `#zerg-class: write D0|D2|D3` 声明。
var pgpdWriteClassRe = regexp.MustCompile(`^#\s*zerg-class:\s*write\s+(?:D0|D2|D3)\s*$`)

// pgpdMaskRule —— 一条掩码规则。only != nil 时仅对满足谓词的匹配做替换（复刻 py 的零宽先行断言）。
type pgpdMaskRule struct {
	re   *regexp.Regexp
	rep  string
	only func(string) bool
}

// pgpdMaskRules —— 已声明的**非确定面**（掩码口径 · 只抹这些 · 其余逐字比 · 只增不改）。
// 顺序 = py `MASK_RULES` 逐条顺序。
var pgpdMaskRules = []pgpdMaskRule{
	{re: regexp.MustCompile(`\d{4}-\d{2}-\d{2}[ T]\d{2}:\d{2}:\d{2}(?:\.\d+)?`), rep: "<TS>"},
	// 毫秒墙钟（★ 排在 `s\b` 之前 · 旧表只认 `s`/`秒`）。
	{re: regexp.MustCompile(`\d+(?:\.\d+)?\s*ms\b`), rep: "<DUR>"},
	{re: regexp.MustCompile(`\d+(?:\.\d+)?\s*s\b`), rep: "<DUR>"},
	{re: regexp.MustCompile(`\d+(?:\.\d+)?\s*秒`), rep: "<DUR>"},
	{re: regexp.MustCompile(`/var/folders/[^\s"']+`), rep: "<TMP>"},
	{re: regexp.MustCompile(`/tmp/[^\s"']+`), rep: "<TMP>"},
	{re: regexp.MustCompile(`/private/[^\s"']+`), rep: "<TMP>"},
	{re: regexp.MustCompile(`pid[=:]\s*\d+`), rep: "pid=<PID>"},
	{re: regexp.MustCompile(`tempdir[=:]\s*\S+`), rep: "tempdir=<TMP>"},
	// 夹具目录 mkdtemp 随机后缀 `fx-<类>-<后缀>`：保留 `<类>` 段 · 只抹后缀。
	{re: regexp.MustCompile(`(fx-[a-z]+(?:-[a-z]+)*-)[A-Za-z0-9_]+`), rep: "${1}<FX>"},
	// 夹具 git 对象哈希：7–64 位十六进制且至少含一个 a–f（排除纯十进制计数）。
	// py 用零宽先行 `\b(?=[0-9a-f]*[a-f])[0-9a-f]{7,64}\b`；RE2 无先行 ⇒ 匹配后再按谓词过滤（等价）。
	{re: regexp.MustCompile(`\b[0-9a-f]{7,64}\b`), rep: "<SHA>", only: pgpdHasLowerHex},
	// py 裸回溯的帧行。
	{re: regexp.MustCompile(`(?m)^[ \t]*File "[^"]*", line \d+(?:, in [^\n]*)?[ \t]*$`), rep: "<TRACE_FRAME>"},
	// 回溯脚手架（头行 ＋ 其下缩进的帧/源码/caret 行）整块折叠。
	{re: regexp.MustCompile(`(?m)^Traceback \(most recent call last\):[ \t]*\r?\n(?:[ \t][^\n]*\r?\n|<TRACE_FRAME>[ \t]*\r?\n)*`), rep: ""},
	// Go 半回溯的具名文案尾注。
	{re: regexp.MustCompile(`（py 真身裸回溯[^\n]*）`), rep: ""},
	// CPython ≥3.12 编译源文件的 SyntaxWarning 行 ＋ 紧随的源码回显行。
	{re: regexp.MustCompile(`(?m)^[^\n]*: SyntaxWarning: [^\n]*\r?\n(?:[ \t][^\n]*\r?\n)?`), rep: ""},
}

// pgpdHasLowerHex —— 匹配串是否至少含一个 a–f（复刻 py 先行断言的过滤面）。
func pgpdHasLowerHex(s string) bool {
	for _, r := range s {
		if r >= 'a' && r <= 'f' {
			return true
		}
	}
	return false
}

// pgpdMask —— 逐条应用掩码规则。
func pgpdMask(text string) string {
	for _, r := range pgpdMaskRules {
		if r.only != nil {
			text = r.re.ReplaceAllStringFunc(text, func(m string) string {
				if r.only(m) {
					return r.re.ReplaceAllString(m, r.rep)
				}
				return m
			})
		} else {
			text = r.re.ReplaceAllString(text, r.rep)
		}
	}
	return text
}

// ────────────────────────── py 语义小原语 ──────────────────────────

// pgpdPyJoin —— CPython posixpath.join 的逐段规则直译（✗ 不用 filepath.Join）。
func pgpdPyJoin(a string, parts ...string) string {
	for _, b := range parts {
		if strings.HasPrefix(b, "/") {
			a = b
		} else if a == "" || strings.HasSuffix(a, "/") {
			a += b
		} else {
			a = a + "/" + b
		}
	}
	return a
}

// pgpdSplitLines —— 复刻 py str.splitlines()（含 \r\n/\r/竖表/换页/记录分隔/下一行/NEL/LS/PS）。
func pgpdSplitLines(s string) []string {
	var out []string
	var cur strings.Builder
	rs := []rune(s)
	for i := 0; i < len(rs); i++ {
		r := rs[i]
		switch {
		case r == '\r':
			if i+1 < len(rs) && rs[i+1] == '\n' {
				i++
			}
			out = append(out, cur.String())
			cur.Reset()
		case r == '\n' || r == '\v' || r == '\f' || r == '\x1c' || r == '\x1d' || r == '\x1e' || r == '\x85' || r == '\u2028' || r == '\u2029':
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

// pgpdPyReprStr —— py repr() 对 str（单引号 · 转义反斜杠与单引号 · 非 ASCII 原样）。
func pgpdPyReprStr(s string) string {
	var b strings.Builder
	b.WriteByte('\'')
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '\'':
			b.WriteString(`\'`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('\'')
	return b.String()
}

// pgpdPyReprList —— py repr() 对 list[str]。
func pgpdPyReprList(items []string) string {
	parts := make([]string, 0, len(items))
	for _, s := range items {
		parts = append(parts, pgpdPyReprStr(s))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// pgpdPyReprAny —— py repr() 对 None / list[str] / str（本门只用这三型）。
func pgpdPyReprAny(v any) string {
	switch x := v.(type) {
	case nil:
		return "None"
	case []string:
		return pgpdPyReprList(x)
	case string:
		return pgpdPyReprStr(x)
	}
	return fmt.Sprintf("%v", v)
}

// pgpdUTF8Replace —— py `decode("utf-8","replace")` 的近似（Go 按非法字节各替换一个 U+FFFD）。
func pgpdUTF8Replace(s string) string {
	return strings.ToValidUTF8(s, "\uFFFD")
}

// ────────────────────────── 有序 JSON（复刻 py json.dumps(indent=2, ensure_ascii=False)） ──────────────────────────

type pgpdKV struct {
	key string
	val any
}

// pgpdJSONStr —— py json.dumps 对 str（ensure_ascii=False）。
func pgpdJSONStr(s string) string {
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
				b.WriteString(fmt.Sprintf(`\u%04x`, r))
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

// pgpdJSONValue —— 递归渲染：str / int / []string。
func pgpdJSONValue(v any, indent int) string {
	switch x := v.(type) {
	case string:
		return pgpdJSONStr(x)
	case int:
		return strconv.Itoa(x)
	case []string:
		if len(x) == 0 {
			return "[]"
		}
		inner := strings.Repeat(" ", indent+2)
		pad := strings.Repeat(" ", indent)
		lines := make([]string, 0, len(x))
		for _, s := range x {
			lines = append(lines, inner+pgpdJSONStr(s))
		}
		return "[\n" + strings.Join(lines, ",\n") + "\n" + pad + "]"
	}
	return "null"
}

// pgpdOrderedJSON —— py json.dumps(dict, ensure_ascii=False, indent=2) 的键序保持复刻。
func pgpdOrderedJSON(kvs []pgpdKV) string {
	if len(kvs) == 0 {
		return "{}"
	}
	inner := strings.Repeat(" ", 2)
	lines := make([]string, 0, len(kvs))
	for _, kv := range kvs {
		lines = append(lines, inner+pgpdJSONStr(kv.key)+": "+pgpdJSONValue(kv.val, 2))
	}
	return "{\n" + strings.Join(lines, ",\n") + "\n}"
}

// ────────────────────────── 读面 ──────────────────────────

// pgpdIsWriteClass —— 真身件头是否声明 `#zerg-class: write`（取不到 ⇒ 当非写类）。
func pgpdIsWriteClass(root, rel string) bool {
	p := pgpdPyJoin(root, rel)
	data, err := os.ReadFile(p)
	if err != nil {
		return false
	}
	s := strings.ReplaceAll(string(data), "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	lines := strings.Split(s, "\n")
	for i, ln := range lines {
		if i >= pgpdWriteClassHeadLines {
			break
		}
		if pgpdWriteClassRe.MatchString(strings.TrimRight(strings.TrimRight(ln, "\n"), "\r")) {
			return true
		}
	}
	return false
}

// pgpdReadInprocKeys ⇒ ([(键, Go 函数名)], "")；取不到 ⇒ (nil, 说明)。
func pgpdReadInprocKeys(root string) ([][2]string, string) {
	p := pgpdPyJoin(root, pgpdInprocRel)
	if !pgpdIsFile(p) {
		return nil, "inproc 表读不到：" + pgpdInprocRel
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, "inproc 表读不出：" + err.Error()
	}
	var out [][2]string
	for _, ln := range pgpdSplitLines(string(data)) {
		m := pgpdKeyRe.FindStringSubmatch(ln)
		if m != nil {
			out = append(out, [2]string{m[1], m[2]})
		}
	}
	if len(out) == 0 {
		return nil, "inproc 表解析出 0 键（取不到输入 ⇒ 空域 ≠ 全绿）"
	}
	return out, ""
}

// pgpdLoadRegistry ⇒ ({名: (相对件路径, 解释器)}, "")；注册表取不到 ⇒ (nil, 说明)。
func pgpdLoadRegistry(root string) (map[string][2]string, string) {
	p := pgpdPyJoin(root, pgpdRegistryRel)
	if !pgpdIsFile(p) {
		return nil, "判据注册表不在盘：" + pgpdRegistryRel
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, "判据注册表解析不上：" + err.Error()
	}
	var d map[string]any
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, "判据注册表解析不上：" + err.Error()
	}
	entriesAny, ok := d["entries"]
	if !ok {
		return nil, "判据注册表缺 entries 列表"
	}
	entries, ok := entriesAny.([]any)
	if !ok {
		return nil, "判据注册表缺 entries 列表"
	}
	out := map[string][2]string{}
	for _, ea := range entries {
		e, ok := ea.(map[string]any)
		if !ok {
			continue
		}
		name := pgpdOptStr(e, "name")
		if name == "" {
			name = pgpdOptStr(e, "id")
		}
		rel := pgpdOptStr(e, "path")
		if name == "" || rel == "" {
			continue
		}
		interp := pgpdOptStr(e, "interp")
		if interp == "" {
			if strings.HasSuffix(rel, ".sh") {
				interp = "bash"
			} else {
				interp = "python3"
			}
		}
		out[name] = [2]string{rel, interp}
	}
	if len(out) == 0 {
		return nil, "判据注册表解析出 0 条（空域 ≠ 全绿）"
	}
	return out, ""
}

// pgpdScanGatesFallback —— 注册表取不到时的回落：只扫 scripts/gates 的 .py。
func pgpdScanGatesFallback(root string) (map[string][2]string, string) {
	d := pgpdPyJoin(root, pgpdGatesRel)
	if !pgpdIsDir(d) {
		return nil, "门件目录读不到：" + pgpdGatesRel
	}
	ents, err := os.ReadDir(d)
	if err != nil {
		return nil, "门件目录读不到：" + pgpdGatesRel
	}
	out := map[string][2]string{}
	for _, e := range ents {
		name := e.Name()
		if strings.HasSuffix(name, ".py") {
			out[strings.TrimSuffix(name, ".py")] = [2]string{pgpdPyJoin("scripts", "gates", name), "python3"}
		}
	}
	return out, ""
}

// pgpdResolveRealbodies —— B 侧真身 ⇒ (map, 来源)。来源 ∈ {"registry","gates"}。
func pgpdResolveRealbodies(root string) (map[string][2]string, string) {
	rb, err := pgpdLoadRegistry(root)
	if rb != nil {
		return rb, "registry"
	}
	rb2, err2 := pgpdScanGatesFallback(root)
	if rb2 == nil {
		return nil, "realbody 取不到：注册表(" + err + ") · 目录扫描(" + err2 + ")"
	}
	return rb2, "gates"
}

// pgpdLoadRatchet ⇒ (known set, "")。
func pgpdLoadRatchet(root string, argvList []string) (map[string]bool, string) {
	p := pgpdPyJoin(root, pgpdRatchetRel)
	if !pgpdIsFile(p) {
		return nil, "棘轮不在盘：" + pgpdRatchetRel + "（先据 --dump-ratchet 落它）"
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, "棘轮解析不上：" + err.Error()
	}
	var d map[string]any
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, "棘轮解析不上：" + err.Error()
	}
	raAny, ok := d["probe_argv"]
	var ra []string
	var raRepr string
	if !ok || raAny == nil {
		raRepr = "None"
	} else if list, ok := pgpdStringList(raAny); ok {
		ra = list
		raRepr = pgpdPyReprList(list)
	} else if s, ok := raAny.(string); ok {
		raRepr = pgpdPyReprStr(s)
	} else {
		raRepr = fmt.Sprintf("%v", raAny)
	}
	if !pgpdStrSliceEq(ra, argvList) {
		return nil, "棘轮 probe_argv=" + raRepr + " ≠ 本次 argv=" + pgpdPyReprList(argvList) + " ⇒ 不可比（不给结论）"
	}
	knownAny, ok := d["known"]
	if !ok {
		return nil, "棘轮缺 known 列表"
	}
	knownList, ok := pgpdStringList(knownAny)
	if !ok {
		return nil, "棘轮缺 known 列表"
	}
	known := map[string]bool{}
	for _, k := range knownList {
		known[k] = true
	}
	return known, ""
}

func pgpdOptStr(e map[string]any, k string) string {
	v, ok := e[k]
	if !ok || v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}

func pgpdStringList(v any) ([]string, bool) {
	arr, ok := v.([]any)
	if !ok {
		return nil, false
	}
	out := make([]string, 0, len(arr))
	for _, x := range arr {
		s, ok := x.(string)
		if !ok {
			return nil, false
		}
		out = append(out, s)
	}
	return out, true
}

func pgpdStrSliceEq(a, b []string) bool {
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

func pgpdIsFile(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.Mode().IsRegular()
}

func pgpdIsDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

// ────────────────────────── 跑子进程 / 对拍 ──────────────────────────

// pgpdRunSideFn —— 单侧跑一条 argv ⇒ (rc|nil, out, err, 超时/起不来 标记)。可注入（自检 ⑲/⑳）。
type pgpdRunSideFn func(argv []string, cwd string, timeout int) (*int, string, string, bool)

var pgpdRunSide pgpdRunSideFn = pgpdRunSideImpl

func pgpdRunSideImpl(argv []string, cwd string, timeout int) (*int, string, string, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = cwd
	var ob, eb bytes.Buffer
	cmd.Stdout = &ob
	cmd.Stderr = &eb
	err := cmd.Run()
	out := pgpdUTF8Replace(ob.String())
	errt := pgpdUTF8Replace(eb.String())
	if ctx.Err() == context.DeadlineExceeded {
		return nil, out, errt, true
	}
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			rc := ee.ExitCode()
			return &rc, out, errt, false
		}
		return nil, "", err.Error(), true
	}
	rc := 0
	return &rc, out, errt, false
}

// pgpdProbeResult —— 一条键的对拍读数。
type pgpdProbeResult struct {
	kind    string // same/diff/unresolved
	aRC     *int
	bRC     *int
	aTo     bool
	bTo     bool
	outDiff bool
	errDiff bool
	rcDiff  bool
}

// pgpdProbe —— 两态对拍一条键。
func pgpdProbe(root, key string, argvList []string, zergBin, pyBin, rel, interp string) pgpdProbeResult {
	sideA := []string{zergBin, "gate", "check", key}
	if pgpdIsWriteClass(root, rel) {
		sideA = append(sideA, "--yes") // ★ 紧贴键名（壳在 exec 前消费 · ✗ 不透传子件）
	}
	sideA = append(sideA, argvList...)
	interpBin := pyBin
	if interp != "python3" {
		interpBin = interp
	}
	sideB := append([]string{interpBin, pgpdPyJoin(root, rel)}, argvList...)
	rca, outa, erra, toa := pgpdRunSide(sideA, root, pgpdProbeTimeout)
	rcb, outb, errb, tob := pgpdRunSide(sideB, root, pgpdProbeTimeout)
	if toa || tob || rca == nil || rcb == nil {
		return pgpdProbeResult{kind: "unresolved", aRC: rca, bRC: rcb, aTo: toa, bTo: tob}
	}
	ma, mb := pgpdMask(outa), pgpdMask(outb)
	ea, eb := pgpdMask(erra), pgpdMask(errb)
	if *rca == *rcb && ma == mb && ea == eb {
		return pgpdProbeResult{kind: "same", aRC: rca, bRC: rcb}
	}
	return pgpdProbeResult{kind: "diff", aRC: rca, bRC: rcb,
		outDiff: ma != mb, errDiff: ea != eb, rcDiff: *rca != *rcb}
}

// ────────────────────────── 读数模型 ──────────────────────────

type pgpdRep struct {
	Root           string
	ProbeArgv      []string
	KeysTotal      int
	PyStemsTotal   int
	RealbodySource string
	Probed         int
	Same           []string
	Diff           []string
	Unresolved     []string
	TableA         []string
	TableB         []string
	RatchetKnown   []string
	RatchetGrew    []string
	RatchetShrank  []string
	Results        map[string]pgpdProbeResult
}

// pgpdEvaluate ⇒ (rep, "")。
func pgpdEvaluate(root string, argvList []string, zergBin, pyBin string, timeout int, requireRatchet bool, only []string) (*pgpdRep, string) {
	keys, err := pgpdReadInprocKeys(root)
	if keys == nil {
		return nil, err
	}
	realbodies, rbSrc := pgpdResolveRealbodies(root)
	if realbodies == nil {
		return nil, rbSrc
	}
	known := map[string]bool{}
	if requireRatchet {
		known, err = pgpdLoadRatchet(root, argvList)
		if known == nil {
			return nil, err
		}
	}
	if zergBin == "" {
		zergBin = pgpdPyJoin(root, "bin", "zerg")
	}
	if !pgpdIsFile(zergBin) {
		return nil, "bin/zerg 不在盘：" + zergBin + "（态① 跑不了）"
	}
	if pyBin == "" {
		pyBin = "python3"
	}
	keyNames := make([]string, 0, len(keys))
	for _, kv := range keys {
		keyNames = append(keyNames, kv[0])
	}
	if only != nil {
		onlySet := map[string]bool{}
		for _, o := range only {
			onlySet[o] = true
		}
		filtered := make([]string, 0, len(keyNames))
		for _, k := range keyNames {
			if onlySet[k] {
				filtered = append(filtered, k)
			}
		}
		keyNames = filtered
	}
	keySet := map[string]bool{}
	for _, kv := range keys {
		keySet[kv[0]] = true
	}
	// 表A：A 有键 · B 无真身
	tableA := []string{}
	for _, k := range keyNames {
		if _, ok := realbodies[k]; !ok {
			tableA = append(tableA, k)
		}
	}
	// 表B：B 有真身 · A 无键
	tableB := []string{}
	for s := range realbodies {
		if !keySet[s] {
			tableB = append(tableB, s)
		}
	}
	sort.Strings(tableB)
	var same, diff, unresolved []string
	results := map[string]pgpdProbeResult{}
	for _, k := range keyNames {
		rb, ok := realbodies[k]
		if !ok {
			continue // 表A 已点名 · 无件可对拍
		}
		r := pgpdProbe(root, k, argvList, zergBin, pyBin, rb[0], rb[1])
		results[k] = r
		switch r.kind {
		case "same":
			same = append(same, k)
		case "diff":
			diff = append(diff, k)
		default:
			unresolved = append(unresolved, k)
		}
	}
	_ = timeout
	diffSet := map[string]bool{}
	for _, k := range diff {
		diffSet[k] = true
	}
	grew, shrank := []string{}, []string{}
	for k := range diffSet {
		if !known[k] {
			grew = append(grew, k)
		}
	}
	for k := range known {
		if !diffSet[k] {
			shrank = append(shrank, k)
		}
	}
	knownList := make([]string, 0, len(known))
	for k := range known {
		knownList = append(knownList, k)
	}
	sort.Strings(same)
	sort.Strings(diff)
	sort.Strings(unresolved)
	sort.Strings(grew)
	sort.Strings(shrank)
	sort.Strings(knownList)
	rep := &pgpdRep{
		Root:           root,
		ProbeArgv:      append([]string{}, argvList...),
		KeysTotal:      len(keyNames),
		PyStemsTotal:   len(realbodies),
		RealbodySource: rbSrc,
		Probed:         len(same) + len(diff) + len(unresolved),
		Same:           same,
		Diff:           diff,
		Unresolved:     unresolved,
		TableA:         tableA,
		TableB:         tableB,
		RatchetKnown:   knownList,
		RatchetGrew:    grew,
		RatchetShrank:  shrank,
		Results:        results,
	}
	if rep.Same == nil {
		rep.Same = []string{}
	}
	if rep.Diff == nil {
		rep.Diff = []string{}
	}
	if rep.Unresolved == nil {
		rep.Unresolved = []string{}
	}
	if rep.TableA == nil {
		rep.TableA = []string{}
	}
	if rep.TableB == nil {
		rep.TableB = []string{}
	}
	return rep, ""
}

// ────────────────────────── 渲染 ──────────────────────────

func pgpdRender(rep *pgpdRep, strict bool) string {
	out := []string{}
	out = append(out, "── py⟷Go 双实现两态对拍（inproc 键 ⇕ 判据注册表真身路径）──")
	argvStr := strings.Join(rep.ProbeArgv, " ")
	if argvStr == "" {
		argvStr = "（空）"
	}
	out = append(out, "  两态 argv："+argvStr)
	out = append(out, fmt.Sprintf("  inproc 键 %d 条 · py 真身 %d 条 · 已对拍 %d 条（同 %d · 差 %d · 不判 %d）",
		rep.KeysTotal, rep.PyStemsTotal, rep.Probed, len(rep.Same), len(rep.Diff), len(rep.Unresolved)))
	out = append(out, fmt.Sprintf("  棘轮：已知差 %d · 本次涨 %d · 本次降 %d",
		len(rep.RatchetKnown), len(rep.RatchetGrew), len(rep.RatchetShrank)))
	if len(rep.Diff) > 0 {
		out = append(out, "  ✗ 差集（两态 stdout/stderr/退码 未逐字同）：")
		for _, k := range rep.Diff {
			r := rep.Results[k]
			tags := []string{}
			if r.rcDiff {
				tags = append(tags, fmt.Sprintf("rc %v≠%v", pgpdRCStr(r.aRC), pgpdRCStr(r.bRC)))
			}
			if r.outDiff {
				tags = append(tags, "stdout 异")
			}
			if r.errDiff {
				tags = append(tags, "stderr 异")
			}
			out = append(out, fmt.Sprintf("    · %s（%s）", k, strings.Join(tags, " · ")))
		}
	}
	if len(rep.Unresolved) > 0 {
		out = append(out, "  ⚠ 不判（超时/起不来 · 不进棘轮）："+strings.Join(rep.Unresolved, " "))
	}
	if len(rep.RatchetGrew) > 0 {
		out = append(out, "  ✗ 棘轮涨了（现读差集里棘轮没有的键）："+strings.Join(rep.RatchetGrew, " "))
	}
	if len(rep.RatchetShrank) > 0 {
		out = append(out, "  ✓ 棘轮降了（棘轮里有、现读已不合）："+strings.Join(rep.RatchetShrank, " "))
	}
	out = append(out, fmt.Sprintf("  表A「A 有键 · B 无件」%d 条：", len(rep.TableA)))
	for _, k := range rep.TableA {
		out = append(out, "    ✗ "+k+"（inproc 在册 · 判据注册表里无真身路径）")
	}
	if len(rep.TableA) == 0 {
		out = append(out, "    （空 · 每枚 inproc 键都在注册表里有真身）")
	}
	out = append(out, fmt.Sprintf("  表B「B 有件 · A 无键」%d 条（半接线/未搬 · 只报告）：", len(rep.TableB)))
	for _, s := range rep.TableB {
		out = append(out, "    · "+s)
	}
	verdict := "报告档"
	if strict {
		verdict = "判红档"
	}
	greens := len(rep.TableA) == 0 && len(rep.RatchetGrew) == 0
	verdictBody := "全绿"
	if !greens {
		verdictBody = fmt.Sprintf("有 %d 项需处理（表A %d · 棘轮涨 %d）",
			len(rep.TableA)+len(rep.RatchetGrew), len(rep.TableA), len(rep.RatchetGrew))
	}
	out = append(out, fmt.Sprintf("  ⇒ %s：%s", verdict, verdictBody))
	return strings.Join(out, "\n")
}

func pgpdRCStr(p *int) string {
	if p == nil {
		return "None"
	}
	return strconv.Itoa(*p)
}

// pgpdJSONBodyKVs —— --json 档（排除 results）。
func pgpdJSONBodyKVs(rep *pgpdRep) []pgpdKV {
	return []pgpdKV{
		{"schema", pgpdSchema},
		{"root", rep.Root},
		{"probe_argv", rep.ProbeArgv},
		{"keys_total", rep.KeysTotal},
		{"py_stems_total", rep.PyStemsTotal},
		{"realbody_source", rep.RealbodySource},
		{"probed", rep.Probed},
		{"same", rep.Same},
		{"diff", rep.Diff},
		{"unresolved", rep.Unresolved},
		{"table_a_key_no_file", rep.TableA},
		{"table_b_file_no_key", rep.TableB},
		{"ratchet_known", rep.RatchetKnown},
		{"ratchet_grew", rep.RatchetGrew},
		{"ratchet_shrank", rep.RatchetShrank},
	}
}

// pgpdDumpRatchet —— 现读差集打到 stdout（不写盘）。
func pgpdDumpRatchet(out io.Writer, rep *pgpdRep) {
	body := []pgpdKV{
		{"schema", pgpdSchema},
		{"_口径", "只许下降：本集是「已声明可分叉」的键；现读差集必须 ⊆ 本集。新增键 ⇒ --strict 判红。"},
		{"probe_argv", rep.ProbeArgv},
		{"known", rep.Diff},
	}
	fmt.Fprintln(out, pgpdOrderedJSON(body))
}

// ────────────────────────── argparse 子集 ──────────────────────────

type pgpdArgs struct {
	root        string
	argv        string
	zerg        string
	pybin       string
	strict      bool
	jsonOut     bool
	dumpRatchet bool
	only        []string
	selfTest    bool
}

var pgpdLongOpts = []string{"--root", "--argv", "--zerg", "--python", "--strict", "--json", "--dump-ratchet", "--only", "--self-test"}

func pgpdTakesValue(opt string) bool {
	switch opt {
	case "--root", "--argv", "--zerg", "--python", "--only":
		return true
	}
	return false
}

func pgpdLooksLikeOption(s string) bool {
	if len(s) < 2 || s[0] != '-' {
		return false
	}
	// 负数不算 option。
	if _, err := strconv.Atoi(s); err == nil {
		return false
	}
	return true
}

// pgpdParseArgs ⇒ (args, action, errMsg)。action ∈ {"run","help","error"}。
func pgpdParseArgs(argv []string) (pgpdArgs, string, string) {
	a := pgpdArgs{argv: "--self-test"}
	var extras []string
	i := 0
	for i < len(argv) {
		arg := argv[i]
		i++
		if arg == "--" {
			extras = append(extras, argv[i-1:]...)
			break
		}
		if strings.HasPrefix(arg, "--") {
			opt, val := arg, ""
			hasVal := false
			if k := strings.Index(arg, "="); k >= 0 {
				opt = arg[:k]
				val = arg[k+1:]
				hasVal = true
			}
			// 精确匹配优先，否则唯一前缀。
			matched := ""
			var cands []string
			for _, o := range pgpdLongOpts {
				if o == opt {
					matched = o
					cands = []string{o}
					break
				}
				if strings.HasPrefix(o, opt) {
					cands = append(cands, o)
				}
			}
			if matched == "" {
				if len(cands) > 1 {
					return a, "error", "ambiguous option: " + opt + " could match " + strings.Join(cands, ", ")
				}
				if len(cands) == 1 {
					matched = cands[0]
				}
			}
			if matched == "" {
				extras = append(extras, arg)
				continue
			}
			if pgpdTakesValue(matched) {
				if !hasVal {
					if i >= len(argv) || pgpdLooksLikeOption(argv[i]) {
						return a, "error", "argument " + matched + ": expected one argument"
					}
					val = argv[i]
					i++
				}
				switch matched {
				case "--root":
					a.root = val
				case "--argv":
					a.argv = val
				case "--zerg":
					a.zerg = val
				case "--python":
					a.pybin = val
				case "--only":
					a.only = append(a.only, val)
				}
			} else {
				switch matched {
				case "--strict":
					a.strict = true
				case "--json":
					a.jsonOut = true
				case "--dump-ratchet":
					a.dumpRatchet = true
				case "--self-test":
					a.selfTest = true
				}
			}
			continue
		}
		if strings.HasPrefix(arg, "-") && arg != "-" {
			if arg == "-h" {
				return a, "help", ""
			}
			extras = append(extras, arg)
			continue
		}
		extras = append(extras, arg)
	}
	if len(extras) > 0 {
		return a, "error", "unrecognized arguments: " + strings.Join(extras, " ")
	}
	return a, "run", ""
}

// ────────────────────────── 主入口 ──────────────────────────

// pgpdGateMain —— 真搬入口（argv 不含程序名）。
func pgpdGateMain(stdout, stderr io.Writer, argv []string) int {
	return pgpdMain(argv, stdout, stderr)
}

func pgpdMain(argv []string, stdout, stderr io.Writer) int {
	a, action, errMsg := pgpdParseArgs(argv)
	switch action {
	case "help":
		io.WriteString(stdout, pgpdHelpText)
		return 0
	case "error":
		io.WriteString(stderr, pgpdUsageText)
		io.WriteString(stderr, pgpdProg+": error: "+errMsg+"\n")
		return 2
	}
	if a.selfTest {
		return pgpdSelfTest(stdout, stderr)
	}
	root := ""
	if a.root != "" {
		root = pgpdAbspath(a.root)
	} else {
		root = pgpdDefaultRoot()
		if root == "" {
			fmt.Fprintf(stderr, "%s: 仓根定位失败（无 --root · env ZERG_REPO_ROOT 未设 · cwd 上溯找不到含 %s 的祖先）⇒ 不给结论（退码 2）\n", pgpdVersion, pgpdGatesRel)
			return 2
		}
	}
	var argvList []string
	if a.argv != "" {
		argvList = strings.Fields(a.argv)
	} else {
		argvList = []string{}
	}
	zergBin := a.zerg
	if zergBin == "" {
		zergBin = pgpdPyJoin(root, "bin", "zerg")
	}
	var pyBin string
	if a.pybin != "" {
		pyBin = a.pybin
	}
	rep, err := pgpdEvaluate(root, argvList, zergBin, pyBin, pgpdProbeTimeout, !a.dumpRatchet, a.only)
	if rep == nil {
		fmt.Fprintf(stderr, "%s: %s ⇒ 不给结论（退码 2）\n", pgpdVersion, err)
		return 2
	}
	if a.dumpRatchet {
		pgpdDumpRatchet(stdout, rep)
		return 0
	}
	if a.jsonOut {
		fmt.Fprintln(stdout, pgpdOrderedJSON(pgpdJSONBodyKVs(rep)))
	} else {
		fmt.Fprintln(stdout, pgpdRender(rep, a.strict))
	}
	if a.strict && (len(rep.TableA) > 0 || len(rep.RatchetGrew) > 0) {
		return 1
	}
	return 0
}

// pgpdAbspath —— py os.path.abspath 的近似（join with cwd 后 Clean）。
func pgpdAbspath(p string) string {
	if !strings.HasPrefix(p, "/") {
		wd, err := os.Getwd()
		if err != nil {
			wd = ""
		}
		p = pgpdPyJoin(wd, p)
	}
	return pgpdNormpath(p)
}

// pgpdDefaultRoot —— 缺省根（见件头 (a)）：与**同族真搬件**同口径 ——
// `check_partial_run_acceptance.go` 的 `praRepoRoot()` / `check_fields_face_parity.go` 的
// `ffpDefaultRoot()` 都是「env `ZERG_REPO_ROOT` 优先，否则 cwd 上溯找含 `scripts/gates` 的祖先」。
// env 优先的理由：转发壳 `gateCheckRunInproc`（`family_gate_check.go`）在调 inproc 之前**必设**
// `ZERG_REPO_ROOT=root` 并把 cwd chdir 到 root ⇒ 走 `zerg gate check <键>` 时 env 那一路必命中。
// 两路都定不到位 ⇒ 返回 ""（调用方 fail-closed 退 2，✗ 不回落 cwd/`..` 猜一个）。
func pgpdDefaultRoot() string {
	if v := os.Getenv("ZERG_REPO_ROOT"); v != "" {
		return pgpdAbspath(v)
	}
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	d := pgpdAbspath(wd)
	for i := 0; i < 64; i++ {
		if st, e := os.Stat(pgpdPyJoin(d, pgpdGatesRel)); e == nil && st.IsDir() {
			return d
		}
		parent := pgpdNormpath(pgpdPyJoin(d, ".."))
		if parent == "" || parent == d {
			break
		}
		d = parent
	}
	return ""
}

// pgpdNormpath —— 纯词法归一（复刻 posixpath.normpath 的常见形态 · 保留恰两个前导斜杠）。
func pgpdNormpath(p string) string {
	if p == "" {
		return "."
	}
	doubleSlash := strings.HasPrefix(p, "//") && !strings.HasPrefix(p, "///")
	abs := strings.HasPrefix(p, "/")
	parts := strings.Split(p, "/")
	var out []string
	for _, seg := range parts {
		if seg == "" || seg == "." {
			continue
		}
		if seg == ".." {
			if len(out) > 0 && out[len(out)-1] != ".." {
				out = out[:len(out)-1]
			} else if !abs {
				out = append(out, "..")
			}
			continue
		}
		out = append(out, seg)
	}
	res := strings.Join(out, "/")
	if abs {
		res = "/" + res
	}
	if doubleSlash {
		res = "/" + res
	}
	if res == "" {
		res = "."
	}
	return res
}

// ────────────────────────── 自检面（--self-test） ──────────────────────────

var pgpdTmpDirs []string

var pgpdDefaultKeys = []string{"k-alpha", "k-beta"}

func pgpdTmp(base, prefix string) string {
	d, err := os.MkdirTemp(base, prefix)
	if err != nil {
		return ""
	}
	pgpdTmpDirs = append(pgpdTmpDirs, d)
	return d
}

func pgpdCleanup() {
	for _, x := range pgpdTmpDirs {
		os.RemoveAll(x)
	}
}

func pgpdWrite(path, body string) error {
	return os.WriteFile(path, []byte(body), 0o644)
}

// pgpdMkFixture —— 现搭仓外最小仓树。
func pgpdMkFixture(base string, keys, pyExtra, argv, ratchetKnown, pyMissing []string) string {
	fix := pgpdTmp(base, "pgpd-")
	if fix == "" {
		return ""
	}
	if keys == nil {
		keys = pgpdDefaultKeys
	}
	if argv == nil {
		argv = []string{"--self-test"}
	}
	os.MkdirAll(pgpdPyJoin(fix, "core", "cmd", "zerg"), 0o755)
	os.MkdirAll(pgpdPyJoin(fix, "scripts", "gates"), 0o755)
	os.MkdirAll(pgpdPyJoin(fix, "bin"), 0o755)
	os.MkdirAll(pgpdPyJoin(fix, "go"), 0o755)
	// inproc 表
	lines := []string{"package main", "", "var gateCheckGoEntry = map[string]func(){"}
	for _, k := range keys {
		lines = append(lines, "\t\""+k+"\":   "+strings.ReplaceAll(k, "-", "_")+"GateMain,")
	}
	lines = append(lines, "}")
	pgpdWrite(pgpdPyJoin(fix, pgpdInprocRel), strings.Join(lines, "\n")+"\n")
	// bin/zerg 桩
	stub := "#!/bin/sh\n" +
		"here=\"$(CDPATH= cd -- \"$(dirname -- \"$0\")\" && pwd)\"\n" +
		"[ \"$1\" = \"gate\" ] && shift\n" +
		"[ \"$1\" = \"check\" ] && shift\n" +
		"k=\"$1\"; shift\n" +
		"exec python3 \"$here/../go/$k.py\" \"$@\"\n"
	zb := pgpdPyJoin(fix, "bin", "zerg")
	pgpdWrite(zb, stub)
	os.Chmod(zb, 0o755)
	// 两侧真件
	joined := strings.Join(argv, " ")
	missing := map[string]bool{}
	for _, m := range pyMissing {
		missing[m] = true
	}
	for _, k := range keys {
		body := "import sys\nif \"--self-test\" in sys.argv:\n    print(\"OUT-" + k + "\")\nelse:\n    print(\"arg:" + joined + "\")\n"
		pgpdWrite(pgpdPyJoin(fix, "go", k+".py"), body)
		if !missing[k] {
			pgpdWrite(pgpdPyJoin(fix, pgpdGatesRel, k+".py"), body)
		}
	}
	for _, k := range pyExtra {
		pgpdWrite(pgpdPyJoin(fix, pgpdGatesRel, k+".py"), "# orphan\n")
	}
	// 棘轮
	sort.Strings(ratchetKnown)
	kr := make([]string, len(ratchetKnown))
	for i, s := range ratchetKnown {
		kr[i] = pgpdJSONStr(s)
	}
	knownJSON := "[" + strings.Join(kr, ", ") + "]"
	av := make([]string, len(argv))
	for i, s := range argv {
		av[i] = pgpdJSONStr(s)
	}
	ratchet := "{\"schema\": " + pgpdJSONStr(pgpdSchema) + ", \"probe_argv\": [" + strings.Join(av, ", ") + "], \"known\": " + knownJSON + "}"
	pgpdWrite(pgpdPyJoin(fix, pgpdRatchetRel), ratchet)
	return fix
}

// pgpdBumpSide —— 把某件的一侧改一个字节：`OUT-k-alpha` → `OUT-<to>`。
func pgpdBumpSide(fix, side, key, to string) {
	rel := pgpdPyJoin("go", key+".py")
	if side != "go" {
		rel = pgpdPyJoin(pgpdGatesRel, key+".py")
	}
	p := pgpdPyJoin(fix, rel)
	data, _ := os.ReadFile(p)
	re := regexp.MustCompile(`OUT-[A-Za-z0-9-]+`)
	n := len(re.FindAllString(string(data), -1))
	if n != 1 {
		panic(fmt.Sprintf("未改到（%s · 命中 %d 次）", rel, n))
	}
	pgpdWrite(p, re.ReplaceAllString(string(data), "OUT-"+to))
}

func pgpdWriteSide(fix, side, key, body string) {
	rel := pgpdPyJoin("go", key+".py")
	if side != "go" {
		rel = pgpdPyJoin(pgpdGatesRel, key+".py")
	}
	pgpdWrite(pgpdPyJoin(fix, rel), body)
}

// pgpdWriteRegistry —— 写一枚仓外夹具注册表（B 侧真身单一真源）。
func pgpdWriteRegistry(fix string, entries [][3]string) {
	var es []string
	for _, e := range entries {
		n, pp, i := e[0], e[1], e[2]
		es = append(es, "{\"id\": "+pgpdJSONStr(n)+", \"name\": "+pgpdJSONStr(n)+", \"path\": "+pgpdJSONStr(pp)+", \"interp\": "+pgpdJSONStr(i)+"}")
	}
	body := "{\"schema\": \"judge-registry/1\", \"roster_dir\": \"scripts\", \"entries\": [" + strings.Join(es, ", ") + "]}"
	pgpdWrite(pgpdPyJoin(fix, pgpdRegistryRel), body)
}

// pgpdRunInproc —— 进程内跑 pgpdMain（捕获 stdout · stderr 走真 stderr）。
func pgpdRunInproc(argv []string, stderr io.Writer) (int, string) {
	var buf bytes.Buffer
	rc := pgpdMain(argv, &buf, stderr)
	return rc, buf.String()
}

// pgpdSelfTest —— 成对正负控（20 格）。
func pgpdSelfTest(stdout, stderr io.Writer) int {
	pgpdTmpDirs = nil // ★ 包级登记面自清（✗ 跨调用残留）
	defer pgpdCleanup()
	d := pgpdTmp("", "pgpd-selftest-")
	bad, total := 0, 0
	casef := func(tag, desc string, ok bool, detail string) {
		total++
		if !ok {
			bad++
		}
		mark := "✓"
		if !ok {
			mark = "✗"
		}
		fmt.Fprintf(stdout, "  %s %s %s\n       %s\n", mark, tag, desc, detail)
	}
	fmt.Fprintln(stdout, "── check-py-go-pairing-drift.py 自检（成对正负控 · 仓外夹具 · 不碰真目标）──")

	// ①正控
	f := pgpdMkFixture(d, nil, nil, nil, nil, nil)
	rep, err := pgpdEvaluate(f, []string{"--self-test"}, "", "", pgpdProbeTimeout, true, nil)
	casef("①正控", "健康夹具（两侧同）⇒ 差集空 · 表A 空 · rc=0",
		err == "" && rep != nil && len(rep.Diff) == 0 && len(rep.TableA) == 0,
		fmt.Sprintf("err=%s diff=%s 表A=%s", pgpdPyReprStr(err), pgpdPyReprList(pgpdListOrNil(rep, func(r *pgpdRep) []string { return r.Diff })), pgpdPyReprList(pgpdListOrNil(rep, func(r *pgpdRep) []string { return r.TableA }))))

	// ②负控
	pgpdBumpSide(f, "go", "k-alpha", "k-alphb")
	rep, err = pgpdEvaluate(f, []string{"--self-test"}, "", "", pgpdProbeTimeout, true, nil)
	casef("②负控", "改**态①(Go 侧)**一字节 ⇒ 差集恰 {k-alpha}（点名该键）",
		err == "" && pgpdListEq(pgpdListOrNil(rep, func(r *pgpdRep) []string { return r.Diff }), []string{"k-alpha"}),
		"diff="+pgpdPyReprList(pgpdListOrNil(rep, func(r *pgpdRep) []string { return r.Diff })))

	// ③正控
	pgpdBumpSide(f, "go", "k-alpha", "k-alpha")
	rep, err = pgpdEvaluate(f, []string{"--self-test"}, "", "", pgpdProbeTimeout, true, nil)
	casef("③正控", "与②成对：复原 Go 侧 ⇒ 差集回空",
		err == "" && pgpdListEq(pgpdListOrNil(rep, func(r *pgpdRep) []string { return r.Diff }), []string{}),
		"diff="+pgpdPyReprList(pgpdListOrNil(rep, func(r *pgpdRep) []string { return r.Diff })))

	// ④负控
	pgpdBumpSide(f, "py", "k-beta", "k-betb")
	rep, err = pgpdEvaluate(f, []string{"--self-test"}, "", "", pgpdProbeTimeout, true, nil)
	casef("④负控", "改**态②(py 侧)**一字节 ⇒ 差集恰 {k-beta}",
		err == "" && pgpdListEq(pgpdListOrNil(rep, func(r *pgpdRep) []string { return r.Diff }), []string{"k-beta"}),
		"diff="+pgpdPyReprList(pgpdListOrNil(rep, func(r *pgpdRep) []string { return r.Diff })))
	pgpdBumpSide(f, "py", "k-beta", "k-beta")

	// ⑤负控
	pgpdWriteSide(f, "py", "k-alpha",
		"import sys\nif \"--self-test\" in sys.argv:\n    print(\"OUT-k-alpha\")\nsys.exit(3)\n")
	rep, err = pgpdEvaluate(f, []string{"--self-test"}, "", "", pgpdProbeTimeout, true, nil)
	casef("⑤负控", "态② 退码改成 3（stdout 相同）⇒ 仍判异（退码面也算）",
		err == "" && pgpdListEq(pgpdListOrNil(rep, func(r *pgpdRep) []string { return r.Diff }), []string{"k-alpha"}),
		"diff="+pgpdPyReprList(pgpdListOrNil(rep, func(r *pgpdRep) []string { return r.Diff })))
	pgpdWriteSide(f, "py", "k-alpha",
		"import sys\nif \"--self-test\" in sys.argv:\n    print(\"OUT-k-alpha\")\nelse:\n    print(\"arg:--self-test\")\n")

	// ⑥负控
	f2 := pgpdMkFixture(d, nil, nil, nil, []string{}, nil)
	pgpdBumpSide(f2, "go", "k-alpha", "k-alphb")
	rc, out := pgpdRunInproc([]string{"--root", f2, "--strict"}, stderr)
	casef("⑥负控", "棘轮 known 空 + 出现新差 ⇒ --strict rc=1（棘轮只许降）",
		rc == 1 && strings.Contains(out, "棘轮涨了") && strings.Contains(out, "k-alpha"),
		fmt.Sprintf("rc=%d", rc))

	// ⑦正控
	f3 := pgpdMkFixture(d, nil, nil, nil, []string{"k-alpha"}, nil)
	pgpdBumpSide(f3, "go", "k-alpha", "k-alphb")
	rc, _ = pgpdRunInproc([]string{"--root", f3, "--strict"}, stderr)
	casef("⑦正控", "与⑥成对：差集已在棘轮里 ⇒ --strict rc=0", rc == 0, fmt.Sprintf("rc=%d", rc))

	// ⑧负控
	f4 := pgpdMkFixture(d, []string{"k-alpha", "k-beta", "k-gamma"}, nil, nil, nil, []string{"k-gamma"})
	rep, err = pgpdEvaluate(f4, []string{"--self-test"}, "", "", pgpdProbeTimeout, true, nil)
	rc, out = pgpdRunInproc([]string{"--root", f4, "--strict"}, stderr)
	casef("⑧负控", "表A：inproc 有 k-gamma 而 scripts/gates/k-gamma.py 不在盘 ⇒ 点名 · strict rc=1",
		err == "" && pgpdListEq(pgpdListOrNil(rep, func(r *pgpdRep) []string { return r.TableA }), []string{"k-gamma"}) && rc == 1 && strings.Contains(out, "k-gamma"),
		fmt.Sprintf("表A=%s rc=%d", pgpdPyReprList(pgpdListOrNil(rep, func(r *pgpdRep) []string { return r.TableA })), rc))

	// ⑨正控
	f5 := pgpdMkFixture(d, nil, []string{"k-orphan"}, nil, nil, nil)
	rep, err = pgpdEvaluate(f5, []string{"--self-test"}, "", "", pgpdProbeTimeout, true, nil)
	rc, _ = pgpdRunInproc([]string{"--root", f5, "--strict"}, stderr)
	casef("⑨正控", "表B：scripts/gates/k-orphan.py 在盘而 inproc 无键 ⇒ 表B 点名 · **不**判红（rc=0）",
		err == "" && pgpdListEq(pgpdListOrNil(rep, func(r *pgpdRep) []string { return r.TableB }), []string{"k-orphan"}) && rc == 0,
		fmt.Sprintf("表B=%s rc=%d", pgpdPyReprList(pgpdListOrNil(rep, func(r *pgpdRep) []string { return r.TableB })), rc))

	// ⑩掩码
	f6 := pgpdMkFixture(d, []string{"k-mask"}, nil, nil, nil, nil)
	pgpdWrite(pgpdPyJoin(f6, "go", "k-mask.py"), "print(\"done 2026-10-05 12:00:00 in 0.31s at /var/folders/ab/T/xyz\")\n")
	pgpdWrite(pgpdPyJoin(f6, pgpdGatesRel, "k-mask.py"), "print(\"done 2026-10-06 01:02:03 in 9.99s at /var/folders/zz/T/qwerty\")\n")
	rep, err = pgpdEvaluate(f6, []string{"--self-test"}, "", "", pgpdProbeTimeout, true, nil)
	casef("⑩掩码", "两侧只差时间戳/耗时/临时目录（已声明非确定面）⇒ 掩码后判**同**",
		err == "" && pgpdListEq(pgpdListOrNil(rep, func(r *pgpdRep) []string { return r.Diff }), []string{}),
		"diff="+pgpdPyReprList(pgpdListOrNil(rep, func(r *pgpdRep) []string { return r.Diff })))

	// ⑪退码钳
	f7 := pgpdTmp(d, "pgpd-notable-")
	os.MkdirAll(pgpdPyJoin(f7, "scripts", "gates"), 0o755)
	os.MkdirAll(pgpdPyJoin(f7, "bin"), 0o755)
	rc, _ = pgpdRunInproc([]string{"--root", f7}, stderr)
	casef("⑪退码钳", "inproc 表读不到 ⇒ rc=2（不给结论 · 空域 ≠ 全绿）", rc == 2, fmt.Sprintf("rc=%d", rc))

	// ⑫退码钳
	f8 := pgpdMkFixture(d, nil, nil, []string{"--self-test"}, nil, nil)
	rc, _ = pgpdRunInproc([]string{"--root", f8, "--argv="}, stderr)
	casef("⑫退码钳", "棘轮 probe_argv(--self-test) ≠ 本次 argv(空) ⇒ rc=2（不可比）", rc == 2, fmt.Sprintf("rc=%d", rc))

	// ⑬正控（决-10）
	f9 := pgpdMkFixture(d, []string{"k-reg"}, nil, nil, nil, []string{"k-reg"})
	os.MkdirAll(pgpdPyJoin(f9, "scripts", "evals"), 0o755)
	cp(pgpdPyJoin(f9, "go", "k-reg.py"), pgpdPyJoin(f9, "scripts", "evals", "k-reg.py"))
	pgpdWriteRegistry(f9, [][3]string{{"k-reg", "scripts/evals/k-reg.py", "python3"}})
	rep, err = pgpdEvaluate(f9, []string{"--self-test"}, "", "", pgpdProbeTimeout, true, nil)
	rc, _ = pgpdRunInproc([]string{"--root", f9, "--strict"}, stderr)
	casef("⑬正控", "决-10：真身在 scripts/evals（不在 gates 面）· 注册表给路径 ⇒ 表A 空 · 两态对拍走注册表路径 rc=0",
		err == "" && rep != nil && len(rep.TableA) == 0 && len(rep.Diff) == 0 && rc == 0,
		fmt.Sprintf("表A=%s diff=%s rc=%d", pgpdPyReprList(pgpdListOrNil(rep, func(r *pgpdRep) []string { return r.TableA })), pgpdPyReprList(pgpdListOrNil(rep, func(r *pgpdRep) []string { return r.Diff })), rc))

	// ⑭负控（决-10）
	f10 := pgpdMkFixture(d, []string{"k-reg2"}, nil, nil, nil, []string{"k-reg2"})
	os.MkdirAll(pgpdPyJoin(f10, "scripts", "evals"), 0o755)
	pgpdWriteRegistry(f10, [][3]string{{"k-other", "scripts/evals/k-other.py", "python3"}})
	rep, err = pgpdEvaluate(f10, []string{"--self-test"}, "", "", pgpdProbeTimeout, true, nil)
	rc, out = pgpdRunInproc([]string{"--root", f10, "--strict"}, stderr)
	casef("⑭负控", "决-10：注册表在盘但无 k-reg2 ⇒ 表A 点名 · strict rc=1",
		err == "" && pgpdListEq(pgpdListOrNil(rep, func(r *pgpdRep) []string { return r.TableA }), []string{"k-reg2"}) && rc == 1 && strings.Contains(out, "k-reg2"),
		fmt.Sprintf("表A=%s rc=%d", pgpdPyReprList(pgpdListOrNil(rep, func(r *pgpdRep) []string { return r.TableA })), rc))

	// ⑮正控（回溯帧面）
	f11 := pgpdMkFixture(d, []string{"k-tb"}, nil, nil, nil, nil)
	goTB := "import sys\nsys.stderr.write(\"KeyError: 'K'（py 真身裸回溯 rc=1 · 回溯帧落在解释器面 ⇒ 正文不复刻）\\n\")\nsys.exit(1)\n"
	pgpdWrite(pgpdPyJoin(f11, "go", "k-tb.py"), goTB)
	pyTB := "import sys\nsys.stderr.write(\"Traceback (most recent call last):\\n  File \\\"/x/os.py\\\", line 679, in __getitem__\\n    w = os.environ[\\\"PROBE_DIR\\\"]\\n        ~~~~~~~~~~^^^^^^^^^^^^^\\n  File \\\"<frozen os>\\\", line 709, in __getitem__\\nKeyError: 'K'\\n\")\nsys.exit(1)\n"
	pgpdWrite(pgpdPyJoin(f11, pgpdGatesRel, "k-tb.py"), pyTB)
	rep, err = pgpdEvaluate(f11, []string{"--self-test"}, "", "", pgpdProbeTimeout, true, nil)
	casef("⑮正控", "回溯帧面：py 整段裸回溯 ⟷ Go 一行具名文案 ⇒ 掩码后判**同**",
		err == "" && pgpdListEq(pgpdListOrNil(rep, func(r *pgpdRep) []string { return r.Diff }), []string{}),
		"diff="+pgpdPyReprList(pgpdListOrNil(rep, func(r *pgpdRep) []string { return r.Diff })))

	// ⑯负控（成对）
	pgpdWrite(pgpdPyJoin(f11, "go", "k-tb.py"), strings.ReplaceAll(goTB, "'K'", "'M'"))
	rep, err = pgpdEvaluate(f11, []string{"--self-test"}, "", "", pgpdProbeTimeout, true, nil)
	casef("⑯负控", "与⑮成对：异常正文 K→M ⇒ 仍判异（真差不得被掩）",
		err == "" && pgpdListEq(pgpdListOrNil(rep, func(r *pgpdRep) []string { return r.Diff }), []string{"k-tb"}),
		"diff="+pgpdPyReprList(pgpdListOrNil(rep, func(r *pgpdRep) []string { return r.Diff })))

	// ⑰正控（解释器告警面）
	f12 := pgpdMkFixture(d, []string{"k-warn"}, nil, nil, nil, nil)
	pgpdWrite(pgpdPyJoin(f12, "go", "k-warn.py"), "import sys\nsys.exit(0)\n")
	warnOK := "import sys\nsys.stderr.write('/x/gen-gap-ledger.py:29: SyntaxWarning: \"\\\\d\" is an invalid escape sequence.\\n  GN_ID_RE = re.compile(\"^Q-\\\\d+$\")\\n')\nsys.exit(0)\n"
	pgpdWrite(pgpdPyJoin(f12, pgpdGatesRel, "k-warn.py"), warnOK)
	rep, err = pgpdEvaluate(f12, []string{"--self-test"}, "", "", pgpdProbeTimeout, true, nil)
	casef("⑰正控", "解释器告警面：py SyntaxWarning 行＋源回显行 ⟷ Go 空 ⇒ 掩码后判**同**",
		err == "" && pgpdListEq(pgpdListOrNil(rep, func(r *pgpdRep) []string { return r.Diff }), []string{}),
		"diff="+pgpdPyReprList(pgpdListOrNil(rep, func(r *pgpdRep) []string { return r.Diff })))

	// ⑱负控（成对）
	pgpdWrite(pgpdPyJoin(f12, pgpdGatesRel, "k-warn.py"),
		"import sys\nsys.stderr.write('/x/gen-gap-ledger.py:29: SyntaxWarning: bad\\n  src line\\nREAL-DIFF-LINE\\n')\nsys.exit(0)\n")
	rep, err = pgpdEvaluate(f12, []string{"--self-test"}, "", "", pgpdProbeTimeout, true, nil)
	casef("⑱负控", "与⑰成对：告警行外另加一行真差 ⇒ 仍判异（掩码 ✗ 不吞真差）",
		err == "" && pgpdListEq(pgpdListOrNil(rep, func(r *pgpdRep) []string { return r.Diff }), []string{"k-warn"}),
		"diff="+pgpdPyReprList(pgpdListOrNil(rep, func(r *pgpdRep) []string { return r.Diff })))

	// ⑲正控（写工具档）
	f13 := pgpdMkFixture(d, []string{"k-w"}, nil, nil, nil, nil)
	pgpdWrite(pgpdPyJoin(f13, pgpdGatesRel, "k-w.sh"), "#!/bin/sh\n#zerg-class: write D2\necho OUT-k-w\n")
	var seen [][]string
	origRS := pgpdRunSide
	pgpdRunSide = func(argv []string, cwd string, timeout int) (*int, string, string, bool) {
		seen = append(seen, append([]string{}, argv...))
		z := 0
		return &z, "", "", false
	}
	pgpdProbe(f13, "k-w", []string{"--self-test"}, "/bin/zerg", "python3", "scripts/gates/k-w.sh", "bash")
	pgpdRunSide = origRS
	casef("⑲正控", "写工具档：件头 `#zerg-class: write D2` ⇒ is_write_class True · side_a 键名后带 --yes",
		pgpdIsWriteClass(f13, "scripts/gates/k-w.sh") && len(seen) > 0 && pgpdListEq(seen[0][:5], []string{"/bin/zerg", "gate", "check", "k-w", "--yes"}),
		fmt.Sprintf("is_write_class=%v side_a=%s", pgpdPyReprBool(pgpdIsWriteClass(f13, "scripts/gates/k-w.sh")), pgpdPyReprList(pgpdFirstRow(seen))))

	// ⑳负控（成对）
	pgpdWrite(pgpdPyJoin(f13, pgpdGatesRel, "k-plain.sh"), "#!/bin/sh\necho hi\n")
	seen = nil
	pgpdRunSide = func(argv []string, cwd string, timeout int) (*int, string, string, bool) {
		seen = append(seen, append([]string{}, argv...))
		z := 0
		return &z, "", "", false
	}
	pgpdProbe(f13, "k-plain", []string{"--self-test"}, "/bin/zerg", "python3", "scripts/gates/k-plain.sh", "bash")
	pgpdRunSide = origRS
	casef("⑳负控", "与⑲成对：非写类件 ⇒ is_write_class False · side_a ✗ 不带 --yes",
		!pgpdIsWriteClass(f13, "scripts/gates/k-plain.sh") && len(seen) > 0 && !pgpdContains(seen[0], "--yes"),
		fmt.Sprintf("is_write_class=%v side_a=%s", pgpdPyReprBool(pgpdIsWriteClass(f13, "scripts/gates/k-plain.sh")), pgpdPyReprList(pgpdFirstRow(seen))))

	fmt.Fprintf(stdout, "  自证用例：%d 条 · 断言条数写死 %d（少跑一格即失效）\n", total, pgpdWantCases)
	if total != pgpdWantCases {
		fmt.Fprintf(stderr, "✗ 判不了：自检用例数对不上（应 %d，实跑 %d）⇒ rc=2\n", pgpdWantCases, total)
		return 2
	}
	verdict := "全过（rc=0）"
	if bad != 0 {
		verdict = fmt.Sprintf("有 %d 条断言不过（rc=1）", bad)
	}
	fmt.Fprintf(stdout, "⇒ 自检 %s\n", verdict)
	if bad == 0 {
		return 0
	}
	return 1
}

// pgpdListOrNil —— 安全取 rep 字段（rep 为 nil 时返回 nil）。
func pgpdListOrNil(rep *pgpdRep, f func(*pgpdRep) []string) []string {
	if rep == nil {
		return nil
	}
	return f(rep)
}

func pgpdListEq(a, b []string) bool {
	if a == nil {
		a = []string{}
	}
	if b == nil {
		b = []string{}
	}
	return pgpdStrSliceEq(a, b)
}

func pgpdPyReprBool(b bool) string {
	if b {
		return "True"
	}
	return "False"
}

func pgpdContains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

func pgpdFirstRow(xs [][]string) []string {
	if len(xs) == 0 {
		return nil
	}
	return xs[0]
}

func cp(src, dst string) {
	data, err := os.ReadFile(src)
	if err == nil {
		os.WriteFile(dst, data, 0o644)
	}
}
