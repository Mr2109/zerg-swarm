// check_cli_contract.go —— 门禁件 `scripts/gates/check-cli-contract.py` 的 Go 真搬
// （M2-0 · 甲案 C 类第 20 枚 · 本件 = CLI 契约面）。
//
// 为什么有这件：命令面是薄壳，它的病不是崩，是悄悄少一层（新命令忘配 must-fail、退码表被抄成
// 第二份、`--json` 包封改形状、矩阵被豁免洗白）。本件按 §九 M17 的六条条款逐条判（本门不重写
// 判据：T6 引用 M6 的族名闭合条款，判据本体在那两条里）。
//
// 入口签名（与 py 的 `main()` 对齐）：
//
//	func cliContractGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv —— 与 py `sys.argv[1:]` 同形（不含程序名）
//	  · 退码 —— 0 全过 / 1 有红 / 2 不给结论（缺件 · 空转 · 判不了 · 用法错）
//	  · 输出 —— stdout/stderr 与 py 逐字节对齐（未覆盖面见回执）
//
// ── 两行标准声明 ────────────────────────────────────────────────────────────────
//
//	(a) **默认根定位差异**：py 从 `__file__` 出发的 `find_root` 上溯 ≤8 层找含
//	    `core/cmd/zerg/main.go` 的根；Go 入口无 `__file__` 语境 ⇒ 从 **cwd / 传入位置参**
//	    上溯 ≤8 层找同一个标记件，找不到即「不给结论」。落在同一真仓且 cwd=仓根时两者
//	    路径推导无差异；「脚本位置 ≠ cwd」时不属逐字对齐承诺。
//	(b) **自指检查**：本件的扫描面（R7 族名闭合那一档）＝**全仓**（按排除表，含 `core/`）
//	    ⇒ 本件**在**自己的扫描面内（自指为真）。防三种自身扫描形态：
//	      ① 正则源串 `\b…\s+…\s+[A-Za-z]` 的两段之间是转义序列（非真空白），**不**自匹配；
//	      ② 判词/夹具里凡要出现被扫写法的，一律**拼串构造**（见 `clicSelfTest` 的 R7 负控）；
//	      ③ 报错文案用 `<动作>` 占位，不落全字面。有效性以「落件前后 py 侧 R7 命中数逐字相同」
//	        与「Go 侧 R7 命中数」两读数对照取证（回执给读数）。
//
// 本单边界（照实）：本件已接线（inproc 表分派）（不注册命令 · 不改 main.go · 不删 py 原件）。
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
	"time"
	"unicode"
)

// ── py 路径语义同构（posixpath）─────────────────────────────────────────────────
// 三类语义差（`..` 段 · 后段绝对 · 空段）逐处由自写同构覆盖，✗ 不用 filepath.Join。

func clicPyJoin(parts ...string) string {
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

func clicPyNormpath(path string) string {
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
	var newComps []string
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
	out := strings.Join(newComps, "/")
	if initialSlashes > 0 {
		out = strings.Repeat("/", initialSlashes) + out
	}
	if out == "" {
		return "."
	}
	return out
}

func clicPyDirname(p string) string {
	i := strings.LastIndex(p, "/") + 1
	head := p[:i]
	if head != "" && head != strings.Repeat("/", len(head)) {
		head = strings.TrimRight(head, "/")
	}
	return head
}

func clicPyAbspath(p string) string {
	cwd, _ := os.Getwd()
	if p == "" {
		p = "."
	}
	if strings.HasPrefix(p, "/") {
		return clicPyNormpath(p)
	}
	return clicPyNormpath(clicPyJoin(cwd, p))
}

func clicPyCommonPrefix(list1, list2 []string) []string {
	var res []string
	for i := 0; i < len(list1) && i < len(list2); i++ {
		if list1[i] != list2[i] {
			break
		}
		res = append(res, list1[i])
	}
	return res
}

func clicPyRelpath(path, start string) string {
	startAbs := clicPyAbspath(start)
	pathAbs := clicPyAbspath(path)
	var startList, pathList []string
	for _, x := range strings.Split(startAbs, "/") {
		if x != "" {
			startList = append(startList, x)
		}
	}
	for _, x := range strings.Split(pathAbs, "/") {
		if x != "" {
			pathList = append(pathList, x)
		}
	}
	i := len(clicPyCommonPrefix(startList, pathList))
	var relList []string
	for j := 0; j < len(startList)-i; j++ {
		relList = append(relList, "..")
	}
	relList = append(relList, pathList[i:]...)
	if len(relList) == 0 {
		return "."
	}
	return clicPyJoin(relList...)
}

func clicPyIsSpace(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', ' ', '\x1c', '\x1d', '\x1e', '\x1f', '\x85':
		return true
	}
	return unicode.Is(unicode.Zs, r)
}

func clicPyStrip(s string) string {
	return strings.TrimFunc(s, clicPyIsSpace)
}

// clicPySplit —— str.split()（无参）：按 py 空白连段切分、剥首尾。
func clicPySplit(s string) []string {
	r := []rune(s)
	n := len(r)
	var out []string
	i := 0
	for i < n {
		for i < n && clicPyIsSpace(r[i]) {
			i++
		}
		if i >= n {
			break
		}
		j := i
		for j < n && !clicPyIsSpace(r[j]) {
			j++
		}
		out = append(out, string(r[i:j]))
		i = j
	}
	return out
}

// clicPyReprStr —— py repr(str) 的一档近似（单/双引号选择 + 常见转义）。
func clicPyReprStr(s string) string {
	hasSingle := strings.Contains(s, "'")
	hasDouble := strings.Contains(s, "\"")
	quote := byte('\'')
	if hasSingle && !hasDouble {
		quote = '"'
	}
	var b strings.Builder
	b.WriteByte(quote)
	for _, r := range s {
		switch {
		case r == '\\':
			b.WriteString("\\\\")
		case r == rune(quote):
			b.WriteByte('\\')
			b.WriteRune(r)
		case r == '\n':
			b.WriteString("\\n")
		case r == '\r':
			b.WriteString("\\r")
		case r == '\t':
			b.WriteString("\\t")
		default:
			if r < 0x20 || r == 0x7f {
				fmt.Fprintf(&b, "\\x%02x", r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte(quote)
	return b.String()
}

func clicPyStr(v any) string {
	switch t := v.(type) {
	case nil:
		return "None"
	case string:
		return t
	case bool:
		if t {
			return "True"
		}
		return "False"
	case float64:
		return strconv.FormatFloat(t, 'g', -1, 64)
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func clicPyRepr(v any) string {
	if s, ok := v.(string); ok {
		return clicPyReprStr(s)
	}
	return clicPyStr(v)
}

func clicPyStrListRepr(xs []string) string {
	parts := make([]string, 0, len(xs))
	for _, x := range xs {
		parts = append(parts, clicPyReprStr(x))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func clicPyIntSliceRepr(xs []int) string {
	parts := make([]string, 0, len(xs))
	for _, x := range xs {
		parts = append(parts, strconv.Itoa(x))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func clicPyTupleRepr(items []string) string {
	if len(items) == 1 {
		return "(" + clicPyReprStr(items[0]) + ",)"
	}
	return "(" + strings.Join(items, ", ") + ")"
}

// clicPyInt —— py int() 的一档近似（int/float 截零 / 数字串 / bool）。
func clicPyInt(v any) (int, bool) {
	switch t := v.(type) {
	case float64:
		return int(t), true
	case int:
		return t, true
	case int64:
		return int(t), true
	case bool:
		if t {
			return 1, true
		}
		return 0, true
	case string:
		s := strings.TrimSpace(t)
		if n, err := strconv.Atoi(s); err == nil {
			return n, true
		}
		return 0, false
	}
	return 0, false
}

// ── 常量 / 正则 ────────────────────────────────────────────────────────────────

var (
	clicMatrixRel    = clicPyJoin("core", "cmd", "zerg", "testdata", "cli-matrix.json")
	clicBaselineRel  = clicPyJoin("scripts", "gates", "cli-contract-baseline.json")
	clicExitcodesRel = clicPyJoin("core", "cmd", "zerg", "exitcodes.go")
	clicMainRel      = clicPyJoin("core", "cmd", "zerg", "main.go")
	clicRegistryRel  = clicPyJoin("core", "internal", "contract", "registry.json")
	// clicSelfRel = 本件在仓根下的相对路径。正式制品用 `-trimpath` 构建 ⇒
	// runtime.Caller(0) 给的是模块路径形态的相对路径（os.ReadFile 解不出）⇒ 靠它退化。
	clicSelfRel = clicPyJoin("core", "cmd", "zerg", "check_cli_contract.go")
)

var clicNodeFamilyRE = regexp.MustCompile(`\bzerg\s+node\s+[A-Za-z]`)

var clicScanExcludeDirs = map[string]bool{
	"target": true, "node_modules": true, "dist": true, "bin": true, "vendor": true,
	"data": true, ".git": true, ".venv": true, "venv": true, ".build": true,
}

var clicScanExts = []string{".go", ".sh", ".py", ".rs", ".md", ".json", ".yaml", ".yml", ".ts", ".tsx", ".js", ".toml"}

var (
	clicCmdRE         = regexp.MustCompile(`path:\s*\[\]string\{([^}]*)\}`)
	clicPathSegRE     = regexp.MustCompile(`"([^"]+)"`)
	clicDangerRE      = regexp.MustCompile(`danger:\s*&dangerSpec\{(dangerD[23])`)
	clicFieldsRE      = regexp.MustCompile(`fields:\s*\[\]string\{([^}]*)\}`)
	clicEndpointRE    = regexp.MustCompile(`endpoint:\s*"([^"]*)"`)
	clicPassthroughRE = regexp.MustCompile(`passthrough:\s*true`)
	clicRowRE         = regexp.MustCompile(`\{\s*(\d+)\s*,\s*"([^"]+)"\s*,\s*"([^"]*)"`)
	clicConstRE       = regexp.MustCompile(`exit([A-Z]\w*)\s*=\s*(\d+)\b`)
	clicBareRetRE     = regexp.MustCompile(`\b(?:(?:return|os\.Exit)\s+|rc[A-Za-z_]*\s*=\s*)\(?\s*(\d+)\b`)
)

// clicBlockHasFields —— 一条命令块是否**有字段表**（缺口 GAP-20260928-159）。
//
//	① 字面量形态 `fields: []string{…}`；
//	② 标识符形态 `fields: <ident>,` —— 含 `append(append([]string{}, A...), B...)` **两片拼接** 与
//	   裸标识符常量 —— 走 `ffpIdentDefs`/`ffpResolveIdentNames`（与字段面对拍门 `check_fields_face_parity.go`
//	   **同源**）。解析不到名字（没定义 / 展开为空）⇒ **不算**「有字段表」（与 `ffpDeclaredFaces` 同判）。
//	⇒ 契约矩阵侧与字段面对拍侧读**同一张**字段表，消除「同一条命令两面口径相左」。
func clicBlockHasFields(block string, lit, refs map[string][][]string, memo map[string]map[string]bool) bool {
	if clicFieldsRE.MatchString(block) {
		return true
	}
	im := ffpReFieldsIdent.FindStringSubmatch(block)
	if im == nil {
		return false
	}
	return len(ffpResolveIdentNames(lit, refs, im[1], memo, 0)) > 0
}

const clicNoConclusionRC = 8

var clicPlaceholderRE = regexp.MustCompile(`^<[^<>]+>$`)

var (
	clicWhyR1BytesRE = regexp.MustCompile(`(\d+)\s*字节`)
	clicWhyR2CountRE = regexp.MustCompile(`(\d+)\s*(?:条|行|格|处|项|列|个|件|层|位)`)
	clicWhyR3RCRE    = regexp.MustCompile(`退\s*(\d+)`)
)

var clicWhyE2Marks = []string{"≤", "≥", "<", ">", "上限", "门槛", "最多", "至少"}

const clicWhyE2Window = 6

var clicWhyE3WideMarks = []string{"前身", "原本", "已删", "已移走", "原值"}

const clicWhyE3WideWindow = 200

var clicWhyE3TightMarks = []string{"改前", "旧值", "曾"}

var clicWhyE3ClauseBreakRE = regexp.MustCompile(`[；;。！!？?\n）)]`)

var clicWhyE4Marks = []string{"不是", "不为", "不记", "不用", "非"}

const clicWhyE4Window = 8

// clicMatrixNote —— 与 py 逐字同（矩阵自述）。
const clicMatrixNote = "命令面自身的 must-fail 矩阵（§九 M17 `T2` · 开工单 T-42 判据①）。\n" +
	"口径：这不是「猜的期望值」，是**冻结的现行为（棘轮）** —— 每条口令逐条现跑两遍、" +
	"退码一致、且与判据语义相符才入矩阵；改行为就必须改这一格（改不动就是回归）。\n" +
	"候选口令按命令形状给（危险档 / 字段面 / 说明面 / 透传型），执行前判的那两条（未知旗标 · " +
	"不认的主号）**每条命令都有**；透传型只留执行前判那条 —— 别的会把被包的脚本真跑一遍。\n" +
	"重生成：`zerg gate check check-cli-contract --emit-matrix`" +
	"（先用 `zerg build all --only cli` 出**新**制品）。\n" +
	"★ 生成是**增量合并**（不是整表重写）：只刷新机器可派生列" +
	"（`want_rc` / `want_stdout_bytes` = **现跑字节数** `len(stdout.encode())` —— 不是字符数），" +
	"既有格的 id/command/argv/why 与顺序逐字保留（手写格不会被重写吃掉），" +
	"只有「一格都没有的新命令」才追加候选格。\n" +
	"覆盖与空白由 T1/T5 判（空白受 baseline 棘轮约束）。"

// ── 数据结构 ──────────────────────────────────────────────────────────────────

type clicShape struct {
	danger      string
	fields      bool
	net         bool
	passthrough bool
}

type clicCmd struct {
	path  string
	shape clicShape
}

type clicRowT struct {
	code    int
	name    string
	meaning string
}

type clicBare struct {
	file string
	code int
}

type clicCandidate struct {
	name string
	argv []string
	want int
	why  string
}

func clicSelfFile() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return ""
	}
	return file
}

func clicReadFile(path string) (string, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	return string(b), true
}

// clicSelfSource 取本件源码文本 + 用于 R7 排除自身的盘上路径。
// 先走 runtime.Caller(0)（开发/测试制品给绝对路径，可读）；正式制品 `-trimpath`
// ⇒ 给的是 `github.com/Mr2109/zerg-swarm/core/cmd/zerg/check_cli_contract.go`
// 这种模块路径形态的相对路径，os.ReadFile 解不出 ⇒ 退化为「仓根 + 固定相对路径」。
// 两路都判不了才返回 ok=false（照样判红，不悄悄放行）。
func clicSelfSource(root string) (src, abs string, ok bool) {
	if p := clicSelfFile(); p != "" {
		if s, r := clicReadFile(p); r {
			return s, p, true
		}
	}
	// 退化：按「仓根 + 固定相对路径」找。传入 root 在主路是真仓根；自检夹具树的
	// root 是临时目录 ⇒ 再补一路与传入 root 无关的真仓根兜底。
	roots := []string{root}
	if er := clicSelfRepoRoot(); er != "" && er != root {
		roots = append(roots, er)
	}
	for _, rt := range roots {
		if rt == "" {
			continue
		}
		p := clicPyJoin(rt, clicSelfRel)
		if s, r := clicReadFile(p); r {
			return s, p, true
		}
	}
	return "", clicPyJoin(root, clicSelfRel), false
}

// clicSelfRepoRoot：与传入 root 无关的仓根兜底（自检夹具树的 root 不是真仓根）。
// ① os.Executable() 上溯（正式制品住 <root>/bin/，`-trimpath` 不影响它）；
// ② cwd 上溯。两路都找不到才回空串。
func clicSelfRepoRoot() string {
	if exe, err := os.Executable(); err == nil {
		if r := clicFindRoot(clicPyDirname(exe)); r != "" {
			return r
		}
	}
	if wd, err := os.Getwd(); err == nil {
		if r := clicFindRoot(wd); r != "" {
			return r
		}
	}
	return ""
}

func clicFindRoot(start string) string {
	d := clicPyAbspath(start)
	for i := 0; i < 8; i++ {
		if fi, err := os.Stat(clicPyJoin(d, clicMainRel)); err == nil && !fi.IsDir() {
			return d
		}
		parent := clicPyDirname(d)
		if parent == d {
			break
		}
		d = parent
	}
	return ""
}

func clicParseTree(root string) []clicCmd {
	src, ok := clicReadFile(clicPyJoin(root, clicMainRel))
	if !ok {
		return nil
	}
	start := strings.Index(src, "commands = []*command{")
	if start < 0 {
		return nil
	}
	rel := strings.Index(src[start:], "\n\t}\n")
	end := len(src)
	if rel >= 0 {
		end = start + rel
	}
	seg := src[start:end]
	// ★ 缺口 GAP-20260928-159（「两片拼接治法」**同源**）：字段表**不只认 `[]string{…}` 字面量** ——
	//   `fields: devEditFields,` 一类**拼接变量 / 裸标识符**也要读成「有字段表」。解析**直接复用**
	//   字段面对拍门（`check_fields_face_parity.go`）的标识符解析（`ffpIdentDefs`/`ffpResolveIdentNames`）
	//   ⇒ 契约矩阵侧与字段面对拍侧**同一取数面**，不再把同一张字段表在契约那侧读成「没有字段表」。
	fldLit, fldRefs := ffpIdentDefs(ffpPkgSources(root))
	fldMemo := map[string]map[string]bool{}
	locs := clicCmdRE.FindAllStringSubmatchIndex(seg, -1)
	var out []clicCmd
	for i, m := range locs {
		group1 := seg[m[2]:m[3]]
		parts := clicPathSegRE.FindAllStringSubmatch(group1, -1)
		var names []string
		for _, p := range parts {
			names = append(names, p[1])
		}
		if len(names) == 0 {
			continue
		}
		nxt := len(seg)
		if i+1 < len(locs) {
			nxt = locs[i+1][0]
		}
		block := seg[m[1]:nxt]
		danger := ""
		if dm := clicDangerRE.FindStringSubmatch(block); dm != nil {
			danger = dm[1]
		}
		ep := clicEndpointRE.FindStringSubmatch(block)
		net := ep != nil && clicPyStrip(ep[1]) != ""
		out = append(out, clicCmd{
			path: strings.Join(names, " "),
			shape: clicShape{
				danger:      danger,
				fields:      clicBlockHasFields(block, fldLit, fldRefs, fldMemo),
				net:         net,
				passthrough: clicPassthroughRE.MatchString(block),
			},
		})
	}
	return out
}

func clicExitcodeTable(root string) []clicRowT {
	src, ok := clicReadFile(clicPyJoin(root, clicExitcodesRel))
	if !ok {
		return nil
	}
	var rows []clicRowT
	for _, v := range []string{"exitcodeTable", "exitcodeReserved"} {
		i := strings.Index(src, "var "+v+" = []exitcodeRow{")
		if i < 0 {
			continue
		}
		j := strings.Index(src[i:], "\n}")
		if j < 0 {
			continue
		}
		j += i
		for _, m := range clicRowRE.FindAllStringSubmatch(src[i:j], -1) {
			c, err := strconv.Atoi(m[1])
			if err != nil {
				continue
			}
			rows = append(rows, clicRowT{code: c, name: m[2], meaning: m[3]})
		}
	}
	return rows
}

func clicNamedConsts(root string) map[string]int {
	src, ok := clicReadFile(clicPyJoin(root, clicMainRel))
	if !ok {
		return map[string]int{}
	}
	i := strings.Index(src, "const (")
	if i < 0 {
		return map[string]int{}
	}
	rel := strings.Index(src[i:], "\n)")
	if rel < 0 {
		return map[string]int{}
	}
	block := src[i : i+rel]
	if !strings.Contains(block, "exitOK") && !strings.Contains(block, "exitFail") {
		return map[string]int{}
	}
	out := map[string]int{}
	for _, m := range clicConstRE.FindAllStringSubmatch(block, -1) {
		n, err := strconv.Atoi(m[2])
		if err != nil {
			continue
		}
		out[m[1]] = n
	}
	return out
}

// clicStripGoCode — 去掉 Go 源里的**字符串/字符字面量与注释**（换行原样保留、其余字符以空格顶替，
// 长度与偏移不变），使 T3 的裸数字判据只看**真源码位置**：夹具字符串里的 `func … { return 3 }`、
// 注释里的 `return 9` 都**不算**退码（缺口 GAP-20261002-12：此前全文扫 ⇒ 三处假红）。
// ★ 真源码里的 `return 127` / 赋值形 `rc = 127`（DEV-2144/DEV-2148 两半）原样留在结果里 ⇒ **真缺口照红**（负控面不缩水）。
func clicStripGoCode(s string) string {
	out := []byte(s)
	const (
		stcCode = iota
		stcLine
		stcBlock
		stcStr
		stcRaw
		stcRune
	)
	st := stcCode
	for i := 0; i < len(s); i++ {
		c := s[i]
		var nx byte
		if i+1 < len(s) {
			nx = s[i+1]
		}
		switch st {
		case stcCode:
			switch {
			case c == '/' && nx == '/':
				out[i], out[i+1] = ' ', ' '
				i++
				st = stcLine
			case c == '/' && nx == '*':
				out[i], out[i+1] = ' ', ' '
				i++
				st = stcBlock
			case c == '"':
				out[i] = ' '
				st = stcStr
			case c == '`':
				out[i] = ' '
				st = stcRaw
			case c == '\'':
				out[i] = ' '
				st = stcRune
			}
		case stcLine:
			if c == '\n' {
				st = stcCode
			} else {
				out[i] = ' '
			}
		case stcBlock:
			switch {
			case c == '*' && nx == '/':
				out[i], out[i+1] = ' ', ' '
				i++
				st = stcCode
			case c != '\n':
				out[i] = ' '
			}
		case stcStr:
			switch {
			case c == '\\' && i+1 < len(s):
				out[i], out[i+1] = ' ', ' '
				i++
			case c == '"':
				out[i] = ' '
				st = stcCode
			case c != '\n':
				out[i] = ' '
			}
		case stcRaw:
			switch {
			case c == '`':
				out[i] = ' '
				st = stcCode
			case c != '\n':
				out[i] = ' '
			}
		case stcRune:
			switch {
			case c == '\\' && i+1 < len(s):
				out[i], out[i+1] = ' ', ' '
				i++
			case c == '\'':
				out[i] = ' '
				st = stcCode
			case c != '\n':
				out[i] = ' '
			}
		}
	}
	return string(out)
}

func clicBareNumericReturns(root string) []clicBare {
	d := clicPyJoin(root, "core", "cmd", "zerg")
	entries, err := os.ReadDir(d)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	var out []clicBare
	for _, name := range names {
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		txt, ok := clicReadFile(clicPyJoin(d, name))
		if !ok {
			continue
		}
		for _, m := range clicBareRetRE.FindAllStringSubmatch(clicStripGoCode(txt), -1) {
			n, err := strconv.Atoi(m[1])
			if err != nil {
				continue
			}
			out = append(out, clicBare{file: name, code: n})
		}
	}
	return out
}

func clicLoadJSONMap(path string) map[string]any {
	txt, ok := clicReadFile(path)
	if !ok {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(txt), &m); err != nil {
		return nil
	}
	return m
}

func clicLoadMatrix(root string) map[string]any {
	p := clicPyJoin(root, clicMatrixRel)
	if fi, err := os.Stat(p); err != nil || fi.IsDir() {
		return nil
	}
	return clicLoadJSONMap(p)
}

func clicLoadBaseline(root string) map[string]any {
	p := clicPyJoin(root, clicBaselineRel)
	if fi, err := os.Stat(p); err != nil || fi.IsDir() {
		return nil
	}
	return clicLoadJSONMap(p)
}

func clicRegistryGateNames(root string) ([]string, bool) {
	p := clicPyJoin(root, clicRegistryRel)
	if fi, err := os.Stat(p); err != nil || fi.IsDir() {
		return nil, false
	}
	m := clicLoadJSONMap(p)
	if m == nil {
		return nil, false
	}
	var out []string
	if ents, ok := m["entries"].([]any); ok {
		for _, e := range ents {
			if em, ok := e.(map[string]any); ok {
				out = append(out, clicPyStr(clicMapGetOr(em, "gate", "")))
			} else {
				out = append(out, "")
			}
		}
	}
	return out, true
}

// ── map 取值助手（区分缺键 / 显式 null 由调用点自行处置）─────────────────────────

func clicMapGet(m map[string]any, key string) (any, bool) {
	if m == nil {
		return nil, false
	}
	v, ok := m[key]
	return v, ok
}

func clicMapGetOr(m map[string]any, key string, def any) any {
	if v, ok := clicMapGet(m, key); ok {
		return v
	}
	return def
}

func clicMapHas(m map[string]any, key string) bool {
	_, ok := clicMapGet(m, key)
	return ok
}

func clicMapStr(m map[string]any, key, def string) string {
	if v, ok := clicMapGet(m, key); ok {
		return clicPyStr(v)
	}
	return def
}

func clicMapList(m map[string]any, key string) []any {
	if v, ok := clicMapGet(m, key); ok {
		if l, ok := v.([]any); ok {
			return l
		}
	}
	return nil
}

// ── R7 族名闭合：全仓（按排除表）扫被禁写法 ────────────────────────────────────

func clicNodeFamilyHits(root string, selfPath string) []string {
	me := clicPyAbspath(selfPath)
	hits := []string{}
	clicWalkCollect(root, root, me, &hits)
	return hits
}

func clicWalkCollect(root, dir, me string, hits *[]string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	var files, dirs []string
	for _, name := range names {
		full := clicPyJoin(dir, name)
		fi, err := os.Stat(full)
		if err != nil {
			continue // 悬空链接 / 读不了：py 视作非目录 ⇒ 归「文件」但读时跳过
		}
		if fi.IsDir() {
			dirs = append(dirs, name)
		} else {
			files = append(files, name)
		}
	}
	for _, fn := range files {
		matched := false
		for _, ext := range clicScanExts {
			if strings.HasSuffix(fn, ext) {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		full := clicPyJoin(dir, fn)
		if clicPyAbspath(full) == me {
			continue
		}
		txt, ok := clicReadFile(full)
		if !ok {
			continue
		}
		for i, line := range clicSplitLines(txt) {
			if clicNodeFamilyRE.MatchString(line) {
				rel := clicPyRelpath(full, root)
				*hits = append(*hits, fmt.Sprintf("%s:%d: %s", rel, i+1, clicFirstRunes(clicPyStrip(line), 140)))
			}
		}
	}
	for _, d := range dirs {
		if clicScanExcludeDirs[d] {
			continue
		}
		full := clicPyJoin(dir, d)
		fi, err := os.Lstat(full)
		if err == nil && fi.Mode()&os.ModeSymlink != 0 {
			continue // followlinks=False：符号链接目录不下钻
		}
		clicWalkCollect(root, full, me, hits)
	}
}

// clicSplitLines —— py str.splitlines()（认 \n \r \r\n \v \f \x1c-\x1e \x85 \u2028 \u2029）。
func clicSplitLines(s string) []string {
	var out []string
	var cur strings.Builder
	rs := []rune(s)
	for i := 0; i < len(rs); i++ {
		r := rs[i]
		switch r {
		case '\n', '\x0b', '\x0c', '\x1c', '\x1d', '\x1e', '\x85', '\u2028', '\u2029':
			out = append(out, cur.String())
			cur.Reset()
			continue
		case '\r':
			if i+1 < len(rs) && rs[i+1] == '\n' {
				i++
			}
			out = append(out, cur.String())
			cur.Reset()
			continue
		}
		cur.WriteRune(r)
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

func clicFirstRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

// ── 候选面 / 现跑派生 ────────────────────────────────────────────────────────

func clicCandidateCases(path string, shape clicShape, usageCode int) []clicCandidate {
	pre := func(tail []string) []string {
		return append(append([]string{}, clicPySplit(path)...), tail...)
	}
	out := []clicCandidate{
		{"未知旗标", pre([]string{"--nosuchflag-zz"}), 2,
			"未知旗标 = 用法错（§4.1 K14 四件套）· 执行前判，不碰任何目标"},
		{"不认的契约主号", pre([]string{"--schema", "zerg/v9"}), 2,
			"消费侧拒绝不认的 schema 主号（§十二 P-026）· 执行前判"},
	}
	if shape.passthrough {
		var kept []clicCandidate
		for _, c := range out {
			if c.name == "不认的契约主号" {
				kept = append(kept, c)
			}
		}
		return kept
	}
	if shape.danger == "dangerD3" {
		out = append(out,
			clicCandidate{"缺 --confirm（D3）", pre([]string{"x"}), 2,
				"D3 不可逆：缺确认 ⇒ fail-closed 2（§九 M3 C2 · §十二 P-014）"},
			clicCandidate{"--confirm 值不匹配", pre([]string{"x", "--confirm=不匹配-zz", "--yes"}), 2,
				"确认值必须与目标逐字相同 —— 给错值一律拒绝（§4.1 K7）"})
	}
	if shape.danger == "dangerD2" {
		out = append(out, clicCandidate{"缺 --yes（D2）", pre([]string{"x"}), 2,
			"D2 缺 --yes ⇒ fail-closed 2（§九 M3 C2）"})
	}
	if shape.danger == "" && shape.fields && !shape.net && path != "help export" {
		out = append(out, clicCandidate{"--json 不给字段", pre([]string{"--json"}), usageCode,
			fmt.Sprintf("给了 --json 不给字段 ⇒ %d + stdout 0 字节（§4.1 K2 甲档 · 码**取自退码表** `usage`）", usageCode)})
	}
	if shape.danger == "" && !shape.fields && !shape.net {
		out = append(out, clicCandidate{"说明面不认 --json", pre([]string{"--json", "name"}), 2,
			fmt.Sprintf("没有字段表的说明面 ⇒ 2（与 K2 甲档归一后的 %d 同码、**错因不同**：这里根本没有字段表）", usageCode)})
	}
	return out
}

func clicRunOnce(binz string, argv []string) (int, int) {
	cmd := exec.Command(binz, argv...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stdin = nil
	err := cmd.Run()
	rc := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			rc = ee.ExitCode()
		} else {
			rc = -1
		}
	}
	return rc, out.Len()
}

func clicDeriveColumns(binz string, argv []string) (map[string]any, string) {
	var rcs, outs []int
	for i := 0; i < 2; i++ {
		rc, n := clicRunOnce(binz, argv)
		rcs = append(rcs, rc)
		outs = append(outs, n)
	}
	cmd := exec.Command(binz, argv...)
	var ob bytes.Buffer
	cmd.Stdout = &ob
	_ = cmd.Run()
	if strings.Contains(ob.String(), "未知命令") {
		return nil, "幽灵命令（制品里没有它 ⇒ 制品不是新的）"
	}
	if rcs[0] != rcs[1] {
		return nil, fmt.Sprintf("两次退码不等 %s", clicPyIntSliceRepr(rcs))
	}
	if rcs[0] == 0 {
		return nil, "退码 0 —— must-fail 矩阵不许有「该绿」的格"
	}
	return map[string]any{"want_rc": rcs[0], "want_stdout_bytes": outs[0]}, ""
}

// ── live_verify（§六 R-6）────────────────────────────────────────────────────

// clicResolveBin —— py `resolve_bin(root, bin_path)`（缺口 `GAP-20260926-12`）：
//
//	不给（""）⇒ 老口径 `<root>/bin/zerg`（逐字不变）；给了 ⇒ 用**绝对化后**的该路径
//	（于是「仓外临时仓根 ＋ 本机真制品」两态可用，✗ 不许碰真仓 `bin/`）。
func clicResolveBin(root, binPath string) string {
	if binPath != "" {
		if strings.HasPrefix(binPath, "/") {
			return binPath
		}
		wd, err := os.Getwd()
		if err != nil {
			return binPath
		}
		return clicPyJoin(wd, binPath)
	}
	return clicPyJoin(root, "bin", "zerg")
}

// clicLiveVerify —— 保原签名（测试件/既有调用点）· 转发 `clicLiveVerifyBin`（binPath=""）。
func clicLiveVerify(root string, matrixPath *string) (int, []string) {
	return clicLiveVerifyBin(root, matrixPath, "")
}

func clicLiveVerifyBin(root string, matrixPath *string, binPath string) (int, []string) {
	binz := clicResolveBin(root, binPath)
	if fi, err := os.Stat(binz); err != nil || fi.IsDir() {
		return 2, []string{"⛔ 不给结论：找不到 bin/zerg —— 先用 `zerg build all --only cli` 出新制品"}
	}
	mpath := clicPyJoin(root, clicMatrixRel)
	if matrixPath != nil {
		mpath = *matrixPath
	}
	if fi, err := os.Stat(mpath); err != nil || fi.IsDir() {
		return 2, []string{fmt.Sprintf("⛔ 不给结论：矩阵不在盘（%s）", mpath)}
	}
	m := clicLoadJSONMap(mpath)
	if m == nil {
		return 2, []string{"⛔ 不给结论：矩阵读不回来（解析失败）"}
	}
	var cases []map[string]any
	for _, c := range clicMapList(m, "cases") {
		cm, ok := c.(map[string]any)
		if !ok {
			continue
		}
		if len(clicMapList(cm, "argv")) == 0 {
			continue
		}
		cases = append(cases, cm)
	}
	if len(cases) == 0 {
		return 2, []string{"⛔ 不给结论：矩阵里没有带 argv 的格（零格 ⇒ 不许当绿）"}
	}
	matched := 0
	var badLines []string
	for _, c := range cases {
		argvv := clicStrSlice(clicMapList(c, "argv"))
		rc1, b1 := clicRunOnce(binz, argvv)
		rc2, b2 := clicRunOnce(binz, argvv)
		wantRcV, _ := clicMapGet(c, "want_rc")
		wantBytesV, _ := clicMapGet(c, "want_stdout_bytes")
		wantRc, _ := clicPyInt(wantRcV)
		wantBytes, _ := clicPyInt(wantBytesV)
		// ★ 2026-10-12（缺口 `GAP-20261012-05`）：与判定口（`cli_matrix_test.go` 的 `judgeCase`）**同规** ——
		//   标了 `stdout_bytes_unpinned` 的格**只钉 rc、不钉字节**：时点变的 stdout 既冻不成常量，
		//   同一 argv **两次现跑本就不同** ⇒ 字节两比（`b1==b2` 与 `b1==wantBytes`）**都要跳**（只留 rc）。
		unpinned := clicTruthy(clicMapGetOr(c, "stdout_bytes_unpinned", nil))
		if rc1 == rc2 && wantRcV != nil && rc1 == wantRc && (unpinned || (wantBytesV != nil && b1 == b2 && b1 == wantBytes)) {
			matched++
			continue
		}
		var how []string
		if rc1 != rc2 || (!unpinned && b1 != b2) {
			how = append(how, fmt.Sprintf("两次现跑不等（%d/%d 字节 vs %d/%d 字节）", rc1, b1, rc2, b2))
		}
		if rc1 != wantRc {
			how = append(how, fmt.Sprintf("退码 %d ≠ 账上 %s", rc1, clicPyRepr(wantRcV)))
		}
		if !unpinned && b1 != wantBytes {
			how = append(how, fmt.Sprintf("stdout %d 字节 ≠ 账上 %s 字节", b1, clicPyRepr(wantBytesV)))
		}
		badLines = append(badLines, fmt.Sprintf("R-6 幽灵格/多余格：%s ⇒ argv=%s · %s",
			clicPyStr(clicMapGetOr(c, "id", nil)), clicPyStrListRepr(argvv), strings.Join(how, " · ")))
	}
	rc := 0
	if len(badLines) > 0 {
		rc = 1
	}
	lines := []string{fmt.Sprintf("R-6 逐格现跑（每格两遍 · 核退码与 stdout 字节数）：合 %d / %d 格 · mismatch %d 格",
		matched, len(cases), len(badLines))}
	return rc, append(lines, badLines...)
}

func clicStrSlice(l []any) []string {
	var out []string
	for _, x := range l {
		out = append(out, clicPyStr(x))
	}
	return out
}

// ── T7 绿格现跑（`tiers.*.cells` 的 `want_rc`）────────────────────────────────

// ordered JSON（保 py dict 插入序 —— tiers 递归按文档序）

type clicJKV struct {
	key string
	val *clicJVal
}

type clicJVal struct {
	kind byte // 'n' 'b' 'd' 's' 'a' 'o'
	obj  []clicJKV
	arr  []*clicJVal
	s    string
	b    bool
	num  float64
}

func clicJSONOrdered(data []byte) (*clicJVal, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	return clicDecToken(dec)
}

func clicDecToken(dec *json.Decoder) (*clicJVal, error) {
	t, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch tv := t.(type) {
	case json.Delim:
		switch tv {
		case '{':
			o := &clicJVal{kind: 'o'}
			for dec.More() {
				kt, err := dec.Token()
				if err != nil {
					return nil, err
				}
				key, _ := kt.(string)
				vv, err := clicDecToken(dec)
				if err != nil {
					return nil, err
				}
				o.obj = append(o.obj, clicJKV{key, vv})
			}
			if _, err := dec.Token(); err != nil {
				return nil, err
			}
			return o, nil
		case '[':
			a := &clicJVal{kind: 'a'}
			for dec.More() {
				vv, err := clicDecToken(dec)
				if err != nil {
					return nil, err
				}
				a.arr = append(a.arr, vv)
			}
			if _, err := dec.Token(); err != nil {
				return nil, err
			}
			return a, nil
		}
	case string:
		return &clicJVal{kind: 's', s: tv}, nil
	case float64:
		return &clicJVal{kind: 'd', num: tv}, nil
	case bool:
		return &clicJVal{kind: 'b', b: tv}, nil
	case nil:
		return &clicJVal{kind: 'n'}, nil
	}
	return nil, fmt.Errorf("unexpected token")
}

func clicDictGet(v *clicJVal, key string) (*clicJVal, bool) {
	if v == nil || v.kind != 'o' {
		return nil, false
	}
	for _, kv := range v.obj {
		if kv.key == key {
			return kv.val, true
		}
	}
	return nil, false
}

func clicValStr(v *clicJVal) string {
	if v == nil {
		return "None"
	}
	switch v.kind {
	case 's':
		return v.s
	case 'd':
		return strconv.FormatFloat(v.num, 'g', -1, 64)
	case 'b':
		if v.b {
			return "True"
		}
		return "False"
	}
	return "None"
}

type clicTierCell struct {
	path string
	c    *clicJVal
}

func clicTierCells(v *clicJVal, path string) []clicTierCell {
	out := []clicTierCell{}
	if v == nil || v.kind != 'o' {
		return out
	}
	for _, kv := range v.obj {
		if kv.val == nil || kv.val.kind != 'o' {
			continue
		}
		cellsV, has := clicDictGet(kv.val, "cells")
		if has && cellsV.kind == 'a' {
			for i, c := range cellsV.arr {
				if c != nil && c.kind == 'o' {
					out = append(out, clicTierCell{path: fmt.Sprintf("%s.%s.cells[%d]", path, kv.key, i), c: c})
				}
			}
		} else {
			out = append(out, clicTierCells(kv.val, path+"."+kv.key)...)
		}
	}
	return out
}

func clicStepNameCandidates(binz string) ([]string, string) {
	cmd := exec.Command(binz, "gate", "run", "--list")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stdin = nil
	err := cmd.Run()
	rc := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			rc = ee.ExitCode()
		} else {
			rc = -1
		}
	}
	if rc != 0 {
		return nil, fmt.Sprintf("`gate run --list` 退出 %d", rc)
	}
	var names []string
	for _, ln := range clicSplitLines(out.String()) {
		if strings.Contains(ln, "──") || strings.Contains(ln, "清单") {
			continue
		}
		parts := strings.SplitN(ln, " ", 3)
		// py: ln.split(None, 2) —— 按空白连段切，最多 3 段
		fields := clicPythonSplitN(ln, 2)
		_ = parts
		if len(fields) == 3 && clicPyStrip(fields[2]) != "" {
			names = append(names, clicPyStrip(fields[2]))
		}
	}
	if len(names) == 0 {
		return nil, "步名表里一行都没读到"
	}
	return names, ""
}

// clicPythonSplitN —— str.split(None, maxsplit)：按 py 空白连段切，最多 maxsplit 次。
func clicPythonSplitN(s string, maxsplit int) []string {
	r := []rune(s)
	n := len(r)
	var out []string
	i := 0
	for {
		for i < n && clicPyIsSpace(r[i]) {
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
		for j < n && !clicPyIsSpace(r[j]) {
			j++
		}
		out = append(out, string(r[i:j]))
		i = j
	}
	return out
}

func clicTierCellLive(root string, matrixRaw []byte, binPath string) (int, []string, []string, map[string]any) {
	binz := clicResolveBin(root, binPath)
	if fi, err := os.Stat(binz); err != nil || fi.IsDir() {
		return 2, []string{"⛔ 不给结论：找不到 bin/zerg —— T7 绿格现跑跑不起来（先用 `zerg build all --only cli` 出**新**制品）"}, nil, map[string]any{}
	}
	root0, err := clicJSONOrdered(matrixRaw)
	if err != nil {
		return 2, []string{"⛔ 不给结论：矩阵读不回来（解析失败）"}, nil, map[string]any{}
	}
	tiers, _ := clicDictGet(root0, "tiers")
	cells := clicTierCells(tiers, "tiers")
	if len(cells) == 0 {
		return 2, []string{"⛔ 不给结论：矩阵 `tiers.*.cells` **一格都没有**（声明面被清空 = 空转）—— T7 判不了，不许当绿"}, nil, map[string]any{"cells": 0}
	}
	var names []string
	errMsg := ""
	need := false
	for _, tc := range cells {
		av, _ := clicDictGet(tc.c, "argv")
		if av != nil && av.kind == 'a' {
			for _, t := range av.arr {
				if clicPlaceholderRE.MatchString(clicValStr(t)) {
					need = true
				}
			}
		}
	}
	if need {
		cand, e := clicStepNameCandidates(binz)
		names = cand
		errMsg = e
	}
	if errMsg != "" {
		return 2, []string{fmt.Sprintf("⛔ 不给结论：占位符（`<步名>`）判不了 —— %s", errMsg)}, nil, map[string]any{"cells": len(cells)}
	}
	var bad, infos, blockedNotes []string
	nOK, nBlocked := 0, 0
	for _, tc := range cells {
		cid := clicValStr(mustVal(clicDictGet(tc.c, "id")))
		if v, ok := clicDictGet(tc.c, "id"); !ok || v == nil {
			cid = "?"
		}
		argv := clicArgvStrings(clicDictGet2(tc.c, "argv"))
		if len(argv) == 0 {
			bad = append(bad, fmt.Sprintf("T7 绿格现跑：**格 %s**（登记在 %s）没有 argv ⇒ 声明面没有判据面可跑", cid, tc.path))
			continue
		}
		ran := make([]string, 0, len(argv))
		for _, t := range argv {
			if clicPlaceholderRE.MatchString(t) && len(names) > 0 {
				ran = append(ran, names[0])
			} else {
				ran = append(ran, t)
			}
		}
		rc1, b1 := clicRunOnce(binz, ran)
		rc2, _ := clicRunOnce(binz, ran)
		wantV, hasWant := clicDictGet(tc.c, "want_rc")
		wantRc, _ := clicPyInt(clicValToAny(wantV))
		_ = hasWant
		if rc1 == rc2 && rc1 == wantRc {
			nOK++
			infos = append(infos, fmt.Sprintf("  合：格 %s（%s）⇒ 现跑 argv=%s rc=%d（两遍一致）· stdout %d 字节",
				cid, tc.path, clicPyStrListRepr(ran), rc1, b1))
			continue
		}
		sdV, _ := clicDictGet(tc.c, "state_dependent")
		sd := ""
		isSDstr := sdV != nil && sdV.kind == 's'
		if isSDstr {
			sd = sdV.s
		}
		if wantRc == 0 && rc1 == rc2 && rc1 == clicNoConclusionRC && isSDstr && clicPyStrip(sd) != "" {
			nBlocked++
			blockedNotes = append(blockedNotes, fmt.Sprintf("  判不了：格 %s（登记在 %s）现跑 argv=%s ⇒ 退码 %d（CLI 自陈**不给结论**· 上下文缺那一趟门跑的结果台账）—— 该格声明为 `state_dependent`：%s",
				cid, tc.path, clicPyStrListRepr(ran), clicNoConclusionRC, sd))
			continue
		}
		var how []string
		if rc1 != rc2 {
			how = append(how, fmt.Sprintf("两次现跑退码不等（%d vs %d）—— 格不稳定", rc1, rc2))
		}
		if rc1 != wantRc {
			how = append(how, fmt.Sprintf("退码 %d ≠ 账上 want_rc=%d", rc1, wantRc))
		}
		bad = append(bad, fmt.Sprintf("T7 绿格现跑：**格 %s**（登记在 %s）现跑 argv=%s ⇒ %s",
			cid, tc.path, clicPyStrListRepr(ran), strings.Join(how, " · ")))
	}
	head := []string{fmt.Sprintf("绿格现跑（`tiers.*.cells[]` 逐格核 want_rc · 每格两遍）：合 %d / %d 格 · 判不了 %d 格 · mismatch %d 格",
		nOK, len(cells), nBlocked, len(bad))}
	stat := map[string]any{"cells": len(cells), "ok": nOK, "bad": len(bad), "blocked": nBlocked,
		"info": infos, "blocked_notes": blockedNotes}
	if nOK == 0 && len(bad) == 0 {
		return 2, append([]string{fmt.Sprintf("⛔ 不给结论：T7 绿格现跑**一格都没判合**（判不了 %d 格 = 判据面空转）—— `state_dependent` 是给状态依赖格判「判不了」的口径，不是让整块空转的口子", nBlocked)}, blockedNotes...), nil, stat
	}
	rc := 0
	if len(bad) > 0 {
		rc = 1
	}
	return rc, head, bad, stat
}

func mustVal(v *clicJVal, ok bool) *clicJVal {
	return v
}

func clicDictGet2(v *clicJVal, key string) (*clicJVal, bool) {
	return clicDictGet(v, key)
}

func clicValToAny(v *clicJVal) any {
	if v == nil {
		return nil
	}
	switch v.kind {
	case 's':
		return v.s
	case 'd':
		return v.num
	case 'b':
		return v.b
	}
	return nil
}

func clicArgvStrings(v *clicJVal, ok bool) []string {
	if v == nil || v.kind != 'a' {
		return nil
	}
	var out []string
	for _, x := range v.arr {
		out = append(out, clicValStr(x))
	}
	return out
}

// ── T5d：why ⟷ 值 同源 ───────────────────────────────────────────────────────

func clicWhyClauseStart(why string, i int) int {
	pre := why[:i]
	locs := clicWhyE3ClauseBreakRE.FindAllStringIndex(pre, -1)
	start := 0
	for _, l := range locs {
		start = l[1]
	}
	return start
}

func clicWhyClaimExempt(why string, mt []int) string {
	pre := why[:mt[2]]
	if strings.HasSuffix(clicPyStripRight(pre), "第") {
		return "E1 出处/编号"
	}
	tail2 := clicLastRunes(pre, clicWhyE2Window)
	for _, m := range clicWhyE2Marks {
		if strings.Contains(tail2, m) {
			return "E2 门槛/上限"
		}
	}
	tail4 := clicLastRunes(pre, clicWhyE4Window)
	tail4 = strings.ReplaceAll(tail4, "*", "")
	for _, m := range clicWhyE4Marks {
		if strings.Contains(tail4, m) {
			return "E4 否定式"
		}
	}
	tail3w := clicLastRunes(pre, clicWhyE3WideWindow)
	for _, m := range clicWhyE3WideMarks {
		if strings.Contains(tail3w, m) {
			return "E3 历史/他格"
		}
	}
	tail3t := why[clicWhyClauseStart(why, mt[2]):]
	for _, m := range clicWhyE3TightMarks {
		if strings.Contains(tail3t, m) {
			return "E3 历史/他格"
		}
	}
	return ""
}

func clicPyStripRight(s string) string {
	return strings.TrimRightFunc(s, clicPyIsSpace)
}

func clicLastRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[len(r)-n:])
}

type clicHit struct {
	start int
	kind  string
	m     []int // submatch indices (with group 1)
}

func clicWhyValueClaims(cases []map[string]any) ([]string, map[string]any) {
	pub := 0
	for _, c := range cases {
		rp, okp := clicMapGet(c, "want_rc_public")
		sp, oks := clicMapGet(c, "want_stdout_bytes_public")
		if (okp && rp != nil) || (oks && sp != nil) {
			pub++
		}
	}
	var errs []string
	exempt := map[string]int{}
	judged := 0
	badCells := map[string]bool{}
	for _, c := range cases {
		whyV, ok := clicMapGet(c, "why")
		why, isStr := whyV.(string)
		if !ok || !isStr {
			continue
		}
		cid := clicPyStr(clicMapGetOr(c, "id", "?"))
		var hits []clicHit
		for _, kr := range []struct {
			kind string
			re   *regexp.Regexp
		}{{"R1", clicWhyR1BytesRE}, {"R2", clicWhyR2CountRE}, {"R3", clicWhyR3RCRE}} {
			for _, m := range kr.re.FindAllStringSubmatchIndex(why, -1) {
				hits = append(hits, clicHit{start: m[0], kind: kr.kind, m: m})
			}
		}
		sort.SliceStable(hits, func(i, j int) bool { return hits[i].start < hits[j].start })
		for _, h := range hits {
			ex := clicWhyClaimExempt(why, h.m)
			if ex != "" {
				exempt[ex]++
				continue
			}
			judged++
			num, _ := strconv.Atoi(why[h.m[2]:h.m[3]])
			var val any
			var col string
			hasVal := false
			switch h.kind {
			case "R1":
				val, hasVal = clicMapGet(c, "want_stdout_bytes")
				col = "want_stdout_bytes"
			case "R3":
				val, hasVal = clicMapGet(c, "want_rc")
				col = "want_rc"
			default:
				val = pub
				hasVal = true
				col = "带 `*_public` 列的格数"
			}
			if !hasVal || val == nil {
				errs = append(errs, fmt.Sprintf("T5d why⟷值：%s 的 why 有一处%s读数（%s）而本格**没有 %s** ⇒ 判不了",
					cid, h.kind, clicPyRepr(why[h.m[0]:h.m[1]]), col))
				badCells[cid] = true
				continue
			}
			vi, okInt := clicPyInt(val)
			if okInt && num == vi {
				continue
			}
			ms, me := h.m[2], h.m[3]
			qs := ms - 12
			if qs < 0 {
				qs = 0
			}
			qe := me + 12
			if qe > len(why) {
				qe = len(why)
			}
			quote := strings.ReplaceAll(why[qs:qe], "\n", "⏎")
			errs = append(errs, fmt.Sprintf("T5d why⟷值：%s ｜ why 里「%s」= %d ≠ %s = %s（原句「…%s…」）",
				cid, clicPyStrip(why[h.m[0]:h.m[1]]), num, col, clicPyStr(val), quote))
			badCells[cid] = true
		}
	}
	stat := map[string]any{"judged": judged, "bad_cells": len(badCells), "bad": len(errs), "exempt": exempt}
	return errs, stat
}

// ── emit_matrix（增量合并）──────────────────────────────────────────────────

func clicEmitMatrix(root string, outPath *string) (int, []string) {
	return clicEmitMatrixOpt(root, outPath, false, "")
}

// clicEmitMatrixOpt —— py `emit_matrix(root, out_path, dry_run, bin_path)`：
// dryRun=true ⇒ 只报计划、**不写**矩阵件（缺口 `GAP-20260927-372`）；binPath ⇒ 现跑取数口用的制品。
func clicEmitMatrixOpt(root string, outPath *string, dryRun bool, binPath string) (int, []string) {
	binz := clicResolveBin(root, binPath)
	if fi, err := os.Stat(binz); err != nil || fi.IsDir() {
		return 2, []string{"⛔ 不给结论：找不到 bin/zerg —— 先用 zerg build all --only cli 出新制品"}
	}
	tree := clicParseTree(root)
	if len(tree) == 0 {
		return 2, []string{"⛔ 不给结论：命令树读不出来（空转）"}
	}
	nameToCode := map[string]int{}
	for _, r := range clicExitcodeTable(root) {
		nameToCode[r.name] = r.code
	}
	usageCode := 2
	if v, ok := nameToCode["usage"]; ok {
		usageCode = v
	}
	old := clicLoadMatrix(root)
	if old == nil {
		old = map[string]any{}
	}
	oldCases := clicCasesOf(old)
	// ★ 内层 case 键序（缺口 `GAP-20261005-07` 残余 · 对齐 py `emit_matrix`）：
	//   py 侧既有格走 `hit = dict(c)`（`json.load` **保件内插入序**）、新增格用**字面插入序**
	//   （`id·command·argv·want_rc·want_stdout_bytes·why`）；Go 端若把每格写成 `map[string]any`，
	//   `encoding/json` 会**按字典序排键** ⇒ 真源件在两序间翻烧饼（同内容不同字节、每次 emit 无故改字节）。
	//   落法：既有格取**原始 JSON 原文**（键序 + 值逐字保留 · 只把两个派生列的数字就地换掉），
	//   新增格按 py 字面序造原文 —— 两半序列化方式对齐。
	var oldRawCases []string
	if rawBytes, rerr := os.ReadFile(clicPyJoin(root, clicMatrixRel)); rerr == nil {
		oldRawCases = clicRawCaseTexts(rawBytes)
	}
	var treePaths []string
	for _, c := range tree {
		treePaths = append(treePaths, c.path)
	}
	var cases []map[string]any
	caseRaws := []json.RawMessage{}
	refreshed, kept := 0, 0
	var warned []string
	for i, c := range oldCases {
		hit := clicCopyMap(c)
		rawCase := ""
		if i < len(oldRawCases) {
			rawCase = oldRawCases[i]
		}
		argv := clicStrSlice(clicMapList(c, "argv"))
		if len(argv) == 0 {
			warned = append(warned, fmt.Sprintf("保留原值（口令为空 ⇒ 跑不了）：%s", clicPyStr(clicMapGetOr(c, "id", "?"))))
		} else {
			d, why := clicDeriveColumns(binz, argv)
			if d == nil {
				warned = append(warned, fmt.Sprintf("保留原值（现跑不成立）：%s —— %s", clicPyStr(clicMapGetOr(c, "id", "?")), why))
			} else {
				oldRc, _ := clicPyInt(clicMapGetOr(c, "want_rc", nil))
				oldB, _ := clicPyInt(clicMapGetOr(c, "want_stdout_bytes", nil))
				newRc, _ := clicPyInt(d["want_rc"])
				newB, _ := clicPyInt(d["want_stdout_bytes"])
				if oldRc != newRc || oldB != newB {
					refreshed++
				}
				hit["want_rc"] = d["want_rc"]
				hit["want_stdout_bytes"] = d["want_stdout_bytes"]
				if rawCase != "" {
					rawCase = clicPatchCaseNum(rawCase, "want_rc", d["want_rc"])
					rawCase = clicPatchCaseNum(rawCase, "want_stdout_bytes", d["want_stdout_bytes"])
				}
			}
		}
		if rawCase == "" {
			rawCase = clicCaseFromMap(hit)
		}
		caseRaws = append(caseRaws, json.RawMessage(rawCase))
		cases = append(cases, hit)
		kept++
	}
	covered := map[string]bool{}
	for _, c := range oldCases {
		covered[clicMapStr(c, "command", "")] = true
	}
	added := 0
	var dropped []string
	for _, cmd := range tree {
		if covered[cmd.path] {
			continue
		}
		for _, cand := range clicCandidateCases(cmd.path, cmd.shape, usageCode) {
			d, w := clicDeriveColumns(binz, cand.argv)
			if d == nil {
				dropped = append(dropped, clicPyTupleRepr([]string{cmd.path, cand.name, w}))
				continue
			}
			gotRc, _ := clicPyInt(d["want_rc"])
			if gotRc != cand.want {
				dropped = append(dropped, clicPyTupleRepr([]string{cmd.path, cand.name,
					fmt.Sprintf("退码 %d ≠ 判据语义要的 %d", gotRc, cand.want)}))
				continue
			}
			id := strings.ReplaceAll(cmd.path, " ", "/") + "#" + cand.name
			cases = append(cases, map[string]any{
				"id": id, "command": cmd.path, "argv": cand.argv,
				"want_rc": d["want_rc"], "want_stdout_bytes": d["want_stdout_bytes"], "why": cand.why,
			})
			caseRaws = append(caseRaws, json.RawMessage(clicNewCaseRaw(id, cmd.path, cand.argv,
				d["want_rc"], d["want_stdout_bytes"], cand.why)))
			added++
		}
	}
	// ★ 顶层键序 = py `emit_matrix` 的 doc 字面量序（`id · note · exemptions · cases · tiers`）。
	//   py 用 `json.dump(doc, …)`（**保插入序** · 非 sort_keys）⇒ Go 端**不许**用 `map[string]any`
	//   （`encoding/json` 对 map **按字典序排键** ⇒ 产物顶层键序与真源分歧、每次 emit 都无故改字节）。
	//   ★ 与仓内各「Go 真搬」同规（`check_affected_map.go`/`bench_summary.go` 等均复刻 py 插入序）。
	//   `tiers` 只在真源有时带上（py：`if old.get('tiers')`），且**排在末位**。
	doc := clicMatrixDoc{
		ID:         clicMapGetOr(old, "id", nil),
		Note:       clicMatrixNote,
		Exemptions: clicMapGetOr(old, "exemptions", nil),
		Cases:      caseRaws,
	}
	if doc.ID == nil {
		doc.ID = "cli-contract.v1"
	}
	if doc.Exemptions == nil {
		doc.Exemptions = []any{}
	}
	if t, ok := clicMapGet(old, "tiers"); ok && clicTruthy(t) {
		doc.Tiers = t
	}
	dst := clicPyJoin(root, clicMatrixRel)
	if outPath != nil {
		dst = *outPath
	}
	if !dryRun {
		if err := clicWriteJSON(dst, doc); err != nil {
			return 2, []string{fmt.Sprintf("⛔ 不给结论：矩阵写不出去（%s）", err)}
		}
	}
	have := map[string]bool{}
	for _, c := range cases {
		have[clicMapStr(c, "command", "")] = true
	}
	treeSet := map[string]bool{}
	for _, p := range treePaths {
		treeSet[p] = true
	}
	var ghosts []string
	for cm := range have {
		if !treeSet[cm] {
			ghosts = append(ghosts, cm)
		}
	}
	sort.Strings(ghosts)
	nBlank := 0
	for _, p := range treePaths {
		if !have[p] {
			nBlank++
		}
	}
	mline := fmt.Sprintf("矩阵已**增量合并**（不是整表重写）：%s", dst)
	if dryRun {
		mline = fmt.Sprintf("矩阵**干跑 · 未落盘**（--dry-run · 缺口 GAP-20260927-372）：计划写 %s", dst)
	}
	lines := []string{
		mline,
		fmt.Sprintf("命令树 %d 条 · 格 %d → %d（既有 %d 格逐字保留 · 其中派生列刷新 %d 格 · 新命令追加 %d 格）",
			len(treePaths), len(oldCases), len(cases), kept, refreshed, added),
		fmt.Sprintf("有格的命令 %d 条 · 空白 %d 条 · 豁免 %d 条", len(have), nBlank, len(clicExemptionsOf(old))),
	}
	for _, g := range ghosts {
		lines = append(lines, fmt.Sprintf("⚠ 矩阵里的 %s 指向命令树里没有的命令（T1 会判幽灵）—— 增量合并不静默删格，需人工定夺", clicPyReprStr(g)))
	}
	for _, d := range dropped {
		lines = append(lines, fmt.Sprintf("丢弃（新命令的候选格·退码不符/幽灵）：%s", d))
	}
	for _, w := range warned {
		lines = append(lines, "⚠ "+w)
	}
	if len(oldCases) == 0 {
		lines = append(lines, "⚠ 既有矩阵为空 ⇒ 本次是按候选面**新建**（不是合并）：手写格无处可保，且命令树里没有 must-fail 新面的命令会成空白（T1/T5 会判）")
	}
	if outPath == nil && !dryRun {
		vrc, vlines := clicLiveVerifyBin(root, nil, binPath)
		lines = append(lines, vlines...)
		if vrc != 0 {
			lines = append(lines, fmt.Sprintf("⚠ 重冻后的**新账**与现跑对不上（见上）—— 账与盘不一致就是「幽灵格」；要判红请单跑 `--verify-live`（本次 rc=%d）", vrc))
		}
	}
	return 0, lines
}

func clicTruthy(v any) bool {
	switch t := v.(type) {
	case nil:
		return false
	case bool:
		return t
	case string:
		return t != ""
	case []any:
		return len(t) > 0
	case map[string]any:
		return len(t) > 0
	}
	return true
}

// clicMatrixDoc —— 矩阵顶层对象的**固定键序**（复刻 py `emit_matrix` 的 doc 字面量序）：
//
//	`id · note · exemptions · cases · tiers`。★ 用结构体而非 `map[string]any` 把序**定死** ——
//	Go `encoding/json` 对 map **按字典序排键**，与 py `json.dump(…, sort_keys=False)` 的插入序分歧
//	（分歧的后果：同内容不同字节 ⇒ 门报字节差、每次 emit 无故改字节的噪声）。
type clicMatrixDoc struct {
	ID         any `json:"id"`
	Note       any `json:"note"`
	Exemptions any `json:"exemptions"`
	Cases      any `json:"cases"`
	Tiers      any `json:"tiers,omitempty"`
}

func clicWriteJSON(path string, doc any) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", " ")
	if err := enc.Encode(doc); err != nil {
		return err
	}
	// ★ 缺口 `GAP-20260927-303`：**临时件 + `rename` 原子替换**，✗ 不再裸 `os.WriteFile` 直接截断写。
	//   过程：同目录开临时件 → 整份写 → `Sync` 落盘 → `Chmod 0644` → `os.Rename` 顶上真源件。
	//   同目录 ⇒ 同文件系统 ⇒ `rename` 原子；任何一步失败 ⇒ 删临时件、**原目标一个字节不动**。
	//   （改前裸截断写：编码/落盘中途失败会把真源矩阵毁成半截 —— 这一版把「失败不伤原件」补上。）
	dir := clicPyDirname(path)
	if dir == "" {
		dir = "."
	}
	tmp, err := os.CreateTemp(dir, ".clic-matrix-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(buf.Bytes()); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		os.Remove(tmpName)
		return err
	}
	// ★ 缺口 `GAP-20260927-303`（残余半 · 退点）：目标件已存在 ⇒ 先落一份逐字节**退点**
	//   `<目标>.bak-<时间戳>`（同目录 · 供误合并/误写后回退）；退点写不成 ⇒ 删临时件、原目标不动。
	//   （原子替换本身已由上面的临时件 + rename 保证「失败不伤原件」；这里补的是「改坏了能从退点回退」。）
	if fi, serr := os.Stat(path); serr == nil && !fi.IsDir() {
		bak := path + ".bak-" + time.Now().Format("20060102-150405")
		if oldB, oerr := os.ReadFile(path); oerr == nil {
			if werr := os.WriteFile(bak, oldB, 0o644); werr != nil {
				os.Remove(tmpName)
				return werr
			}
		}
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}

func clicCasesOf(m map[string]any) []map[string]any {
	var out []map[string]any
	for _, c := range clicMapList(m, "cases") {
		if cm, ok := c.(map[string]any); ok {
			out = append(out, cm)
		}
	}
	return out
}

func clicCasesToAny(cs []map[string]any) []any {
	out := make([]any, 0, len(cs))
	for _, c := range cs {
		out = append(out, c)
	}
	return out
}

// clicRawCaseTexts —— 从矩阵件**原始字节**里按序取出顶层 `cases[]` 每格的 JSON 原文
// （键序 + 值逐字保留）。py 侧 `hit = dict(c)` 的等价物：既有格序列化序 = 件内现序。
func clicRawCaseTexts(raw []byte) []string {
	dec := json.NewDecoder(bytes.NewReader(raw))
	t, err := dec.Token()
	if err != nil {
		return nil
	}
	if d, ok := t.(json.Delim); !ok || d != '{' {
		return nil
	}
	for dec.More() {
		kt, err := dec.Token()
		if err != nil {
			return nil
		}
		key, _ := kt.(string)
		if key == "cases" {
			at, err := dec.Token()
			if err != nil {
				return nil
			}
			if d, ok := at.(json.Delim); !ok || d != '[' {
				return nil
			}
			var out []string
			for dec.More() {
				var rm json.RawMessage
				if err := dec.Decode(&rm); err != nil {
					return out
				}
				out = append(out, string(rm))
			}
			return out
		}
		var skip json.RawMessage
		if err := dec.Decode(&skip); err != nil {
			return nil
		}
	}
	return nil
}

// clicPatchCaseNum —— 把一格 case 原文里 `"<key>":` 后面的数字就地换成 val（保键序、保其余值逐字）。
// 键必须是**对象键**（后随 `:`）⇒ 与 `want_rc_public` 一类后缀键、以及字符串值里的同名字样都不冲突。
func clicPatchCaseNum(raw, key string, val any) string {
	n, ok := clicPyInt(val)
	if !ok {
		return raw
	}
	needle := "\"" + key + "\""
	idx := strings.Index(raw, needle)
	if idx < 0 {
		return raw
	}
	j := idx + len(needle)
	for j < len(raw) && (raw[j] == ' ' || raw[j] == '\t' || raw[j] == '\n' || raw[j] == '\r') {
		j++
	}
	if j >= len(raw) || raw[j] != ':' {
		return raw
	}
	j++
	for j < len(raw) && (raw[j] == ' ' || raw[j] == '\t' || raw[j] == '\n' || raw[j] == '\r') {
		j++
	}
	k := j
	if k < len(raw) && (raw[k] == '-' || raw[k] == '+') {
		k++
	}
	for k < len(raw) && raw[k] >= '0' && raw[k] <= '9' {
		k++
	}
	if k == j {
		return raw
	}
	return raw[:j] + strconv.Itoa(n) + raw[k:]
}

// clicCaseFromMap —— 兜底：原文取不到时按 map 序列化（既有行为 · 键序会排字典序）。
func clicCaseFromMap(m map[string]any) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(m)
	return strings.TrimRight(buf.String(), "\n")
}

// clicNewCaseRow —— 新增格的**字面插入序**（= py `emit_matrix` 那一句的字面序）。
type clicNewCaseRow struct {
	ID              string   `json:"id"`
	Command         string   `json:"command"`
	Argv            []string `json:"argv"`
	WantRC          any      `json:"want_rc"`
	WantStdoutBytes any      `json:"want_stdout_bytes"`
	Why             string   `json:"why"`
}

// clicNewCaseRaw —— 按 py 字面序造新增格原文（`ensure_ascii=False` ⇒ 不转义 HTML / 非 ASCII）。
func clicNewCaseRaw(id, command string, argv []string, rc, wb any, why string) string {
	if argv == nil {
		argv = []string{}
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(clicNewCaseRow{ID: id, Command: command, Argv: argv, WantRC: rc, WantStdoutBytes: wb, Why: why})
	return strings.TrimRight(buf.String(), "\n")
}

func clicExemptionsOf(m map[string]any) []map[string]any {
	var out []map[string]any
	for _, e := range clicMapList(m, "exemptions") {
		if em, ok := e.(map[string]any); ok {
			out = append(out, em)
		}
	}
	return out
}

func clicCopyMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// ── check（主体）─────────────────────────────────────────────────────────────

func clicCheck(root string, noRuntime bool) (int, []string) {
	return clicCheckBin(root, noRuntime, "")
}

// clicCheckBin —— binPath ≠ "" ⇒ 现跑取数口用该制品（py `check(root, no_runtime, bin_path)`）。
func clicCheckBin(root string, noRuntime bool, binPath string) (int, []string) {
	var errs []string

	tree := clicParseTree(root)
	matrix := clicLoadMatrix(root)
	table := clicExitcodeTable(root)
	consts := clicNamedConsts(root)
	baseline := clicLoadBaseline(root)

	if len(tree) == 0 {
		return 2, []string{"⛔ 不给结论：命令树一条命令都没读到（T1 空转 = 假覆盖）"}
	}
	if matrix == nil || len(clicCasesOf(matrix)) == 0 {
		return 2, []string{fmt.Sprintf("⛔ 不给结论：must-fail 矩阵缺失或为空（%s）（T2 空转 = 假覆盖）", clicMatrixRel)}
	}
	if len(table) == 0 || len(consts) == 0 {
		return 2, []string{"⛔ 不给结论：退码表或退码常量没读到（T3 空转 = 假覆盖）"}
	}
	if baseline == nil || !clicMapHas(baseline, "blanks") {
		return 2, []string{fmt.Sprintf("⛔ 不给结论：baseline 棘轮文件缺失/不可解析（%s）—— 「清到 0 再升阻断」的锚点没了", clicBaselineRel)}
	}

	cases := clicCasesOf(matrix)
	exemptions := clicExemptionsOf(matrix)
	for _, e := range exemptions {
		r := ""
		if v, ok := clicMapGet(e, "reason"); ok {
			r = clicPyStr(v)
		}
		if clicPyStrip(r) == "" {
			return 2, []string{fmt.Sprintf("⛔ 不给结论：豁免 %s **没有 reason** —— 写不清理由的豁免就是洗白（§九 M17 T5）", clicPyRepr(clicMapGetOr(e, "command", "?")))}
		}
	}

	var treePaths []string
	for _, c := range tree {
		treePaths = append(treePaths, c.path)
	}
	byCmd := map[string][]map[string]any{}
	for _, c := range cases {
		byCmd[clicMapStr(c, "command", "")] = append(byCmd[clicMapStr(c, "command", "")], c)
	}

	// ---- T1 覆盖 ----
	exemptCmds := map[string]bool{}
	for _, e := range exemptions {
		exemptCmds[clicMapStr(e, "command", "")] = true
	}
	var blanks []string
	for _, p := range treePaths {
		if _, ok := byCmd[p]; !ok && !exemptCmds[p] {
			blanks = append(blanks, p)
		}
	}
	sort.Strings(blanks)
	for _, p := range blanks {
		errs = append(errs, fmt.Sprintf("T1 覆盖：命令 %s 既没有 case 也没有带 reason 的豁免", clicPyReprStr(p)))
	}
	treeSet := map[string]bool{}
	for _, p := range treePaths {
		treeSet[p] = true
	}
	var ghosts []string
	for cm := range byCmd {
		if !treeSet[cm] {
			ghosts = append(ghosts, cm)
		}
	}
	sort.Strings(ghosts)
	for _, g := range ghosts {
		errs = append(errs, fmt.Sprintf("T1 覆盖：矩阵里的 %s 是**幽灵条目**（命令树里没有这条命令）", clicPyReprStr(g)))
	}

	// ---- T2 must-fail ----
	nMustFail := 0
	for _, c := range cases {
		rcv, _ := clicMapGet(c, "want_rc")
		rc, ok := clicPyInt(rcv)
		if !ok {
			errs = append(errs, fmt.Sprintf("T2 must-fail：case %s 的 want_rc 不是整数", clicPyRepr(clicMapGetOr(c, "id", "?"))))
			continue
		}
		if rc == 0 {
			errs = append(errs, fmt.Sprintf("T2 must-fail：case %s 的 want_rc = 0 —— must-fail 矩阵不许有「该绿」的条目", clicPyRepr(clicMapGetOr(c, "id", "?"))))
			continue
		}
		nMustFail++
	}
	for _, p := range treePaths {
		cs, ok := byCmd[p]
		if !ok {
			continue
		}
		anyMustFail := false
		for _, c := range cs {
			v, _ := clicPyInt(clicMapGetOr(c, "want_rc", 0))
			if v != 0 {
				anyMustFail = true
			}
		}
		if !anyMustFail {
			errs = append(errs, fmt.Sprintf("T2 must-fail：命令 %s 一条 must-fail 都没有", clicPyReprStr(p)))
		}
	}

	// ---- T5b 格级棘轮 ----
	baseMf, mfOK := clicPyInt(clicMapGetOr(baseline, "mustfail_cases", nil))
	if !mfOK {
		return 2, []string{fmt.Sprintf("⛔ 不给结论：baseline 棘轮缺 `mustfail_cases`（格级锚点）—— 格级掉格会静默（%s）", clicBaselineRel)}
	}
	if nMustFail < baseMf {
		errs = append(errs, fmt.Sprintf("T5b 格级棘轮：must-fail 格 %d 条 < baseline %d 条（格级只许**上调**；确需下调须显式改 %s 并写明理由 —— 防「同命令内部掉一格」静默）",
			nMustFail, baseMf, clicBaselineRel))
	}

	// ---- T5c 树条数相等判 ----
	baseCmds, cOK := clicPyInt(clicMapGetOr(baseline, "commands", nil))
	if !cOK {
		return 2, []string{fmt.Sprintf("⛔ 不给结论：baseline 棘轮缺 `commands`（树条数锚点）—— 树条数漂了会静默（%s）", clicBaselineRel)}
	}
	if len(tree) != baseCmds {
		errs = append(errs, fmt.Sprintf("T5c 树条数相等判：现算命令树 %d 条 ≠ baseline 记的 %d 条（差 %+d）—— 两处真源开始漂了；确需变动须显式改 %s 并写明理由",
			len(tree), baseCmds, len(tree)-baseCmds, clicBaselineRel))
	}

	// ---- T3 退码表唯一 ----
	var codes []int
	seen := map[int]string{}
	for _, r := range table {
		if prev, ok := seen[r.code]; ok {
			errs = append(errs, fmt.Sprintf("T3 退码表：**同数字两义** —— %d 同时是 %s 与 %s（§十二 P-013 占号纪律）", r.code, clicPyReprStr(prev), clicPyReprStr(r.name)))
		}
		if _, ok := seen[r.code]; !ok {
			seen[r.code] = r.name
		}
		codes = append(codes, r.code)
	}
	legal := map[int]bool{}
	for _, c := range codes {
		legal[c] = true
	}
	var constNames []string
	for n := range consts {
		constNames = append(constNames, n)
	}
	sort.Slice(constNames, func(i, j int) bool {
		if consts[constNames[i]] != consts[constNames[j]] {
			return consts[constNames[i]] < consts[constNames[j]]
		}
		return constNames[i] < constNames[j]
	})
	for _, nm := range constNames {
		val := consts[nm]
		if !legal[val] {
			var legalSorted []int
			for c := range legal {
				legalSorted = append(legalSorted, c)
			}
			sort.Ints(legalSorted)
			errs = append(errs, fmt.Sprintf("T3 退码表：常量 exit%s = %d **不在真源表里**（表里只有 %s）", nm, val, clicPyIntSliceRepr(legalSorted)))
		}
	}
	constVals := map[int]bool{}
	for _, v := range consts {
		constVals[v] = true
	}
	if !clicIntSetEqual(constVals, legal) {
		var a, b []int
		for v := range constVals {
			a = append(a, v)
		}
		for v := range legal {
			b = append(b, v)
		}
		sort.Ints(a)
		sort.Ints(b)
		errs = append(errs, fmt.Sprintf("T3 退码表：常量集 %s ≠ 表内数字集 %s（两处真源开始漂了）", clicPyIntSliceRepr(a), clicPyIntSliceRepr(b)))
	}
	bare := clicBareNumericReturns(root)
	for _, b := range bare {
		if !legal[b.code] {
			errs = append(errs, fmt.Sprintf("T3 退码表：%s 里出现**裸数字**退出码 %d —— 它不在真源表里（该走具名常量）", b.file, b.code))
		}
	}

	// ---- T5 空白 + baseline 棘轮 ----
	baseBlanks, _ := clicPyInt(clicMapGetOr(baseline, "blanks", 0))
	if len(blanks) > baseBlanks {
		errs = append(errs, fmt.Sprintf("T5 棘轮：空白 %d 条 > baseline %d 条（棘轮只许**下调**；新增命令必须补 must-fail）", len(blanks), baseBlanks))
	}

	// ---- T5d why ⟷ 值 同源 ----
	t5dErrs, t5dStat := clicWhyValueClaims(cases)
	if t5dStat == nil {
		return 2, []string{"⛔ 不给结论：why ⟷ 值的判据面拿不到（`cases` 不是表）—— 判不了不许当绿"}
	}
	errs = append(errs, t5dErrs...)

	// ---- T6 引用 M6 R7 ----
	gates, gatesOK := clicRegistryGateNames(root)
	if !gatesOK {
		return 2, []string{fmt.Sprintf("⛔ 不给结论：契约登记表读不出来（%s）—— T6 的引用面无法判", clicRegistryRel)}
	}
	gatesNamed := false
	for _, g := range gates {
		if strings.Contains(g, "check_cli_contract.go") || strings.Contains(g, "check-cli-contract.py") {
			gatesNamed = true
		}
	}
	if !gatesNamed {
		errs = append(errs, "T6 引用：契约登记表里**没有**任何条目的 gate 字段点名本门（Go 半 check_cli_contract.go / py 原件 check-cli-contract.py）（引用面 = 0 ⇒ 本门挂在空气上）")
	}
	selfSrc, selfPath, selfOK := clicSelfSource(root)
	if !selfOK || !strings.Contains(selfSrc, "M6 R7") {
		errs = append(errs, "T6 引用：本门头部没有引用 §九 M6 R7（本门不重写判据，只引用它）")
	}

	// ---- R7 族名闭合 ----
	for _, c := range tree {
		fam := clicPySplit(c.path)
		if len(fam) > 0 && fam[0] == "node" {
			errs = append(errs, "R7 族名：命令树里出现 `node` 族 —— 族名只有一个 `agent`（§十二 P-038）")
			break
		}
	}
	hits := clicNodeFamilyHits(root, selfPath)
	for i, h := range hits {
		if i >= 20 {
			break
		}
		errs = append(errs, fmt.Sprintf("R7 族名：全仓还有 `zerg node <动作>` 的写法 ⇒ %s", h))
	}
	if len(hits) > 20 {
		errs = append(errs, fmt.Sprintf("R7 族名：另有 %d 处（上面只列前 20）", len(hits)-20))
	}

	// ---- T4 --json 形状守卫（运行时只读）----
	t4 := "（--no-runtime：夹具模式跳过）"
	if !noRuntime {
		r := exec.Command("go", "test", "./cmd/zerg/", "-run", "TestCLIContractJSONShape", "-count=1")
		r.Dir = clicPyJoin(root, "core")
		var so, se bytes.Buffer
		r.Stdout = &so
		r.Stderr = &se
		err := r.Run()
		if err != nil {
			if _, ok := err.(*exec.ExitError); !ok {
				return 2, []string{"⛔ 不给结论：T4 的运行时守卫跑不起来（机器上没有 go）—— 「跑不起来」不许当绿"}
			}
			seStr := se.String()
			if clicContainsAny(seStr, []string{"go.mod file not found", "cannot find package", "no such file or directory", "is not in std", "executable file not found", "no Go files"}) {
				first := "go test 跑不起来"
				if t := clicPyStrip(seStr); t != "" {
					first = clicSplitLines(t)[0]
				}
				return 2, []string{fmt.Sprintf("⛔ 不给结论：T4 的运行时守卫在这棵树上跑不起来（%s）—— 「跑不起来」不许当绿", first)}
			}
			tail := clicTailRunes(so.String(), 1500) + clicTailRunes(seStr, 500)
			errs = append(errs, "T4 `--json` 形状守卫：go test 失败 —— 包封形状被改了（§九 M6 I1/I2/I3 · R7）\n"+tail)
		}
		t4 = "go test -run TestCLIContractJSONShape ⇒ rc=0"
	}

	// ---- T7 绿格现跑 ----
	t7 := "（--no-runtime：夹具模式跳过）"
	var t7Info []string
	if !noRuntime {
		raw, _ := clicReadFile(clicPyJoin(root, clicMatrixRel))
		rc7, head7, bad7, stat7 := clicTierCellLive(root, []byte(raw), binPath)
		if rc7 == 2 {
			return 2, head7
		}
		t7 = head7[0]
		if inf, ok := stat7["info"].([]string); ok {
			t7Info = append(t7Info, inf...)
		}
		if bn, ok := stat7["blocked_notes"].([]string); ok {
			t7Info = append(t7Info, bn...)
		}
		errs = append(errs, bad7...)
	}

	lines := []string{
		fmt.Sprintf("命令树 %d 条 · 矩阵 case %d 条（must-fail %d）· 空白 %d 条 · 豁免 %d 条",
			len(treePaths), len(cases), nMustFail, len(blanks), len(exemptions)),
		fmt.Sprintf("退码表 %d 格（数字唯一：%s）· 常量 %d 个 · 裸数字退出码 %d 处",
			len(table), clicYesNo(len(clicDedupInts(codes)) == len(codes)), len(consts), len(bare)),
		"T4 " + t4,
		"T7 " + t7,
	}
	lines = append(lines, t7Info...)
	// ★ 缺口 `GAP-20260927-289`：把 baseline 的**计数锚**（声明值）与**实测值**在此一览自报 ——
	//   让「note/计数锚过期」当场可读（T5b/T5c/T5 已在漂时判红；本行把它们摆在一起，✗ 不再只能从账外看出来）。
	blOK := len(tree) == baseCmds && nMustFail >= baseMf && len(blanks) <= baseBlanks
	blWord := "漂（见 ✗ T5/T5b/T5c）"
	if blOK {
		blWord = "齐"
	}
	lines = append(lines, fmt.Sprintf("baseline 计数锚 声明⟷实测（缺口 GAP-20260927-289）：命令树 %d/%d · must-fail %d/%d · 空白 %d/%d ⇒ %s",
		len(tree), baseCmds, nMustFail, baseMf, len(blanks), baseBlanks, blWord))
	lines = append(lines, fmt.Sprintf("T6 契约登记表 gate 字段点名本脚本：%s", clicYesNo(gatesNamed)))
	lines = append(lines, fmt.Sprintf("R7 族名闭合（`agent` 唯一 · 全仓 `zerg node <动作>` 命中）：%d 处", len(hits)))
	var exemptParts []string
	for _, k := range clicSortedKeys(t5dStat["exempt"].(map[string]int)) {
		exemptParts = append(exemptParts, fmt.Sprintf("%s %d", k, t5dStat["exempt"].(map[string]int)[k]))
	}
	exemptStr := "无"
	if len(exemptParts) > 0 {
		exemptStr = strings.Join(exemptParts, " · ")
	}
	sumExempt := 0
	for _, v := range t5dStat["exempt"].(map[string]int) {
		sumExempt += v
	}
	lines = append(lines, fmt.Sprintf("T5d why ⟷ 值 同源（三 regex：字节/条数/退码）：判 %d 处 · 不一致 %d 格 %d 处 · 免于判 %d 处（%s）",
		t5dStat["judged"], t5dStat["bad_cells"], t5dStat["bad"], sumExempt, exemptStr))
	for _, e := range errs {
		lines = append(lines, "✗ "+e)
	}
	if len(errs) > 0 {
		return 1, lines
	}
	return 0, lines
}

func clicYesNo(b bool) string {
	if b {
		return "是"
	}
	return "否"
}

func clicDedupInts(xs []int) []int {
	seen := map[int]bool{}
	var out []int
	for _, x := range xs {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}

func clicIntSetEqual(a, b map[int]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}

func clicSortedKeys(m map[string]int) []string {
	var ks []string
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

func clicContainsAny(s string, subs []string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func clicTailRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[len(r)-n:])
}

// ── 自检（成对负控）──────────────────────────────────────────────────────────

const clicGoodMain = `package main

const (
	exitOK          = 0
	exitFail        = 1
	exitUsage       = 2
	exitInterrupted = 130
)

var commands []*command

func init() {
	commands = []*command{
		{
			path:    []string{"version"},
			fields:  []string{"name"},
			endpoint: "",
			run:     cmdVersion,
		},
		{
			path:   []string{"core", "stop"},
			danger: &dangerSpec{dangerD3, "主机名", "停主控", "§三 C 族"},
			run:    cmdGuarded,
		},
	}
}
`

const clicGoodExitcodes = `package main

type exitcodeRow struct {
	Code      int
	Name      string
	Meaning   string
	Retryable string
}

var exitcodeTable = []exitcodeRow{
	{0, "ok", "成功", "—"},
	{1, "failed", "失败", "可重试"},
	{2, "usage", "用法错", "—"},
}

var exitcodeReserved = []exitcodeRow{
	{130, "interrupted", "人打断", "—"},
}
`

func clicGoodMatrix() map[string]any {
	return map[string]any{
		"id":         "cli-contract.v1",
		"exemptions": []any{},
		"cases": []any{
			map[string]any{"id": "version#未知旗标", "command": "version", "argv": []any{"version", "--x"},
				"want_rc": 2.0, "want_stdout_bytes": 0.0, "why": "未知旗标 = 用法错"},
			map[string]any{"id": "core/stop#缺 --confirm", "command": "core stop", "argv": []any{"core", "stop", "x"},
				"want_rc": 2.0, "want_stdout_bytes": 0.0, "why": "D3 缺确认 ⇒ 2"},
		},
	}
}

func clicGoodBaseline() map[string]any {
	return map[string]any{"id": "cli-contract-baseline.v1", "blanks": 0.0, "mustfail_cases": 2.0,
		"commands": 2.0,
		"note":     "只许下调：本门落地时空白已清零 ⇒ 从此任何空白即红；`commands` 是 T5c 的**相等判**锚点（夹具树 = version + core stop 两条）"}
}

func clicGoodRegistry() map[string]any {
	return map[string]any{"schema": "zerg-contract-registry/1", "entries": []any{
		map[string]any{"id": "S-g", "name": "命令面契约（本件）",
			"gate": "scripts/gates/check-error-kinds.py · scripts/gates/check-cli-contract.py"},
	}}
}

func clicFixture(root string, main, exitcodes string, matrix, baseline, registry map[string]any, extraCmd string) {
	write := func(rel, body string) {
		p := clicPyJoin(root, rel)
		_ = os.MkdirAll(clicPyDirname(p), 0o755)
		_ = os.WriteFile(p, []byte(body), 0o644)
	}
	write(clicMainRel, main)
	write(clicExitcodesRel, exitcodes)
	jb, _ := json.Marshal(matrix)
	write(clicMatrixRel, string(jb))
	bb, _ := json.Marshal(baseline)
	write(clicBaselineRel, string(bb))
	rb, _ := json.Marshal(registry)
	write(clicRegistryRel, string(rb))
	if extraCmd != "" {
		p := clicPyJoin(root, clicMainRel)
		f, _ := os.OpenFile(p, os.O_APPEND|os.O_WRONLY, 0o644)
		if f != nil {
			_, _ = f.WriteString(extraCmd)
			_ = f.Close()
		}
	}
}

func clicDeepCopy(m map[string]any) map[string]any {
	b, _ := json.Marshal(m)
	var out map[string]any
	_ = json.Unmarshal(b, &out)
	return out
}

func clicSelfTest(w io.Writer) int {
	var bad []string
	caseFn := func(name string, want, got int, extra string) {
		if want != got {
			bad = append(bad, fmt.Sprintf("%s：要 rc=%d，实际 rc=%d %s", name, want, got, extra))
		}
	}
	tmp, err := os.MkdirTemp("", "clicontract-")
	if err != nil {
		fmt.Fprintf(w, "自检结论: 不过（失败 %d 条）\n", 1)
		fmt.Fprintln(w, "✗ 建临时目录失败："+err.Error())
		return 2
	}
	defer os.RemoveAll(tmp)

	// ① 正控
	r := clicPyJoin(tmp, "good")
	clicFixture(r, clicGoodMain, clicGoodExitcodes, clicGoodMatrix(), clicGoodBaseline(), clicGoodRegistry(), "")
	rc, lines := clicCheck(r, true)
	caseFn("① 正控（好夹具 ⇒ 0）", 0, rc, strings.Join(lines, "\n"))

	// ② T1 负控
	r = clicPyJoin(tmp, "t1")
	clicFixture(r, clicGoodMain, clicGoodExitcodes, clicGoodMatrix(), clicGoodBaseline(), clicGoodRegistry(),
		"\n// 追加：矩阵里没有它的 must-fail\nvar extra = []*command{}\n")
	body, _ := clicReadFile(clicPyJoin(r, clicMainRel))
	body = strings.ReplaceAll(body, "\t}\n}\n", "\t\t{path: []string{\"agent\", \"ls\"}, fields: []string{\"machine\"}, run: cmdAgentLs},\n\t}\n}\n")
	_ = os.WriteFile(clicPyJoin(r, clicMainRel), []byte(body), 0o644)
	rc, lines = clicCheck(r, true)
	caseFn("② T1 负控（树里有命令、矩阵没 case ⇒ 1）", 1, rc, "")
	if !clicAnyLine(lines, "T1 覆盖", "agent ls") {
		bad = append(bad, "② T1 负控：判词没有点名缺 case 的那条命令")
	}

	// ③ T2 负控
	m := clicDeepCopy(clicGoodMatrix())
	m["cases"].([]any)[0].(map[string]any)["want_rc"] = 0.0
	rc, lines = clicCheck(clicFixturePath(tmp, "t2", m), true)
	caseFn("③ T2 负控（want_rc=0 ⇒ 1）", 1, rc, "")
	if !clicAnyLine(lines, "T2 must-fail") {
		bad = append(bad, "③ T2 负控：判词里没有 T2")
	}

	// ④ T1 负控b（幽灵）
	m = clicDeepCopy(clicGoodMatrix())
	m["cases"] = append(m["cases"].([]any), map[string]any{"id": "ghost#x", "command": "no such",
		"argv": []any{"no", "such"}, "want_rc": 2.0, "want_stdout_bytes": 0.0, "why": "幽灵"})
	rc, lines = clicCheck(clicFixturePath(tmp, "t1b", m), true)
	caseFn("④ T1 负控（幽灵条目 ⇒ 1）", 1, rc, "")
	if !clicAnyLine(lines, "幽灵") {
		bad = append(bad, "④ T1 负控：判词里没有「幽灵」")
	}

	// ⑤ T3 负控（同数字两义）
	ec := strings.Replace(clicGoodExitcodes, "{2, \"usage\", \"用法错\", \"—\"},", "{0, \"usage\", \"用法错\", \"—\"},", 1)
	r = clicPyJoin(tmp, "t3")
	clicFixture(r, clicGoodMain, ec, clicGoodMatrix(), clicGoodBaseline(), clicGoodRegistry(), "")
	rc, lines = clicCheck(r, true)
	caseFn("⑤ T3 负控（同数字两义 ⇒ 1）", 1, rc, "")
	if !clicAnyLine(lines, "同数字两义") {
		bad = append(bad, "⑤ T3 负控：判词里没有「同数字两义」")
	}

	// ⑥ T3 负控b（裸数字不在表里）
	r = clicPyJoin(tmp, "t3b")
	clicFixture(r, clicGoodMain, clicGoodExitcodes, clicGoodMatrix(), clicGoodBaseline(), clicGoodRegistry(), "")
	p := clicPyJoin(r, clicMainRel)
	f, _ := os.OpenFile(p, os.O_APPEND|os.O_WRONLY, 0o644)
	if f != nil {
		_, _ = f.WriteString("\nfunc bogus() int { return " + "77 }\n")
		_ = f.Close()
	}
	rc, lines = clicCheck(r, true)
	caseFn("⑥ T3 负控（裸数字 77 ⇒ 1）", 1, rc, "")
	if !clicAnyLine(lines, "裸数字") {
		bad = append(bad, "⑥ T3 负控：判词里没有「裸数字」")
	}

	// ⑥b T3 负控c（赋值形裸数字不在表里 —— 闭合 DEV-2144/DEV-2148 的赋值支）
	r = clicPyJoin(tmp, "t3c")
	clicFixture(r, clicGoodMain, clicGoodExitcodes, clicGoodMatrix(), clicGoodBaseline(), clicGoodRegistry(), "")
	p = clicPyJoin(r, clicMainRel)
	f, _ = os.OpenFile(p, os.O_APPEND|os.O_WRONLY, 0o644)
	if f != nil {
		_, _ = f.WriteString("\nfunc bogusAssign() int { rc := 0; rc = " + "127; return rc }\n")
		_ = f.Close()
	}
	rc, lines = clicCheck(r, true)
	caseFn("⑥b T3 负控（赋值形裸数字 rc=127 ⇒ 1）", 1, rc, "")
	if !clicAnyLine(lines, "裸数字") {
		bad = append(bad, "⑥b T3 负控：判词里没有「裸数字」")
	}

	// ⑦ T5 负控（豁免无 reason ⇒ 2）
	m = clicDeepCopy(clicGoodMatrix())
	m["exemptions"] = []any{map[string]any{"command": "core stop", "reason": ""}}
	rc, lines = clicCheck(clicFixturePath(tmp, "t5", m), true)
	caseFn("⑦ T5 负控（豁免无 reason ⇒ 2）", 2, rc, "")
	if !clicAnyLine(lines, "没有 reason") {
		bad = append(bad, "⑦ T5 负控：判词里没有「没有 reason」")
	}

	// ⑧ T5 负控b（baseline 缺失 ⇒ 2）
	r = clicPyJoin(tmp, "t5b")
	clicFixture(r, clicGoodMain, clicGoodExitcodes, clicGoodMatrix(), clicGoodBaseline(), clicGoodRegistry(), "")
	_ = os.Remove(clicPyJoin(r, clicBaselineRel))
	rc, lines = clicCheck(r, true)
	caseFn("⑧ T5 负控（baseline 缺失 ⇒ 2）", 2, rc, "")

	// ⑨ T4 负控（运行时守卫跑不起来 ⇒ 2）
	r = clicPyJoin(tmp, "t4")
	clicFixture(r, clicGoodMain, clicGoodExitcodes, clicGoodMatrix(), clicGoodBaseline(), clicGoodRegistry(), "")
	rc, lines = clicCheck(r, false)
	caseFn("⑨ T4 负控（运行时守卫跑不起来 ⇒ 2）", 2, rc, "")
	if !clicAnyLine(lines, "T4") {
		bad = append(bad, "⑨ T4 负控：判词里没有 T4")
	}

	// ⑩ T6 负控（引用面 = 0 ⇒ 1）
	reg := clicDeepCopy(clicGoodRegistry())
	reg["entries"].([]any)[0].(map[string]any)["gate"] = "scripts/gates/check-error-kinds.py"
	rc, lines = clicCheck(clicFixturePath2(tmp, "t6", reg), true)
	caseFn("⑩ T6 负控（引用面 = 0 ⇒ 1）", 1, rc, "")
	if !clicAnyLine(lines, "T6 引用") {
		bad = append(bad, "⑩ T6 负控：判词里没有 T6")
	}

	// ⑪ 空转负控（矩阵空 ⇒ 2）
	m = clicDeepCopy(clicGoodMatrix())
	m["cases"] = []any{}
	rc, lines = clicCheck(clicFixturePath(tmp, "empty", m), true)
	caseFn("⑪ 空转负控（矩阵空 ⇒ 2）", 2, rc, "")

	// ⑫ R7 负控（全仓另有被禁写法 ⇒ 1）—— 夹具里那处写法用拼接构造，不落字面量
	r = clicPyJoin(tmp, "r7")
	clicFixture(r, clicGoodMain, clicGoodExitcodes, clicGoodMatrix(), clicGoodBaseline(), clicGoodRegistry(), "")
	_ = os.WriteFile(clicPyJoin(r, "docs-命令说明.md"), []byte("跑 `zerg "+"node reap` 收残留\n"), 0o644)
	rc, lines = clicCheck(r, true)
	caseFn("⑫ R7 负控（另有被禁写法 ⇒ 1）", 1, rc, "")
	if !clicAnyLine(lines, "R7 族名") {
		bad = append(bad, "⑫ R7 负控：判词里没有 R7 族名")
	}

	// ⑬ R7 正控（去掉那处写法 ⇒ 0）
	_ = os.Remove(clicPyJoin(r, "docs-命令说明.md"))
	rc, lines = clicCheck(r, true)
	caseFn("⑬ R7 正控（去掉那处写法 ⇒ 0）", 0, rc, "")

	// ⑭ emit-matrix 空转负控（没有 bin/zerg ⇒ 2）
	rc, lines = clicEmitMatrix(clicPyJoin(tmp, "good"), nil)
	caseFn("⑭ emit-matrix 负控（没有 bin/zerg ⇒ 2）", 2, rc, "")
	if !clicAnyLine(lines, "找不到 bin/zerg") {
		bad = append(bad, "⑭ emit-matrix 负控：判词里没点名缺制品")
	}

	// ⑮ emit-matrix 字节数 + 增量合并
	r = clicPyJoin(tmp, "emit")
	seed := clicDeepCopy(clicGoodMatrix())
	seed["cases"] = []any{
		map[string]any{"id": "version#手写格", "command": "version", "argv": []any{"version"},
			"want_rc": 2.0, "want_stdout_bytes": 0.0, "why": "手写：版本号后面跟东西 ⇒ 2"},
		map[string]any{"id": "version#未知旗标", "command": "version", "argv": []any{"version", "--x"},
			"want_rc": 9.0, "want_stdout_bytes": 0.0, "why": "旧 why（候选面不许覆盖它）"},
	}
	seed["exemptions"] = []any{map[string]any{"command": "core stop", "reason": "夹具豁免（合并不许丢）"}}
	clicFixture(r, clicGoodMain, clicGoodExitcodes, seed, clicGoodBaseline(), clicGoodRegistry(), "")
	fakeOut := "用法错：中文输出（字节数 ≠ 字符数）\n"
	_ = os.MkdirAll(clicPyJoin(r, "bin"), 0o755)
	script := "#!/bin/sh\nprintf '%s\\n' '用法错：中文输出（字节数 ≠ 字符数）'\nexit 2\n"
	_ = os.WriteFile(clicPyJoin(r, "bin", "zerg"), []byte(script), 0o755)
	dst := clicPyJoin(tmp, "emit-out.json")
	rc, lines = clicEmitMatrix(r, &dst)
	caseFn("⑮ emit-matrix（增量合并 ⇒ 0）", 0, rc, "")
	var mout map[string]any
	if b, err := os.ReadFile(dst); err == nil {
		if e := json.Unmarshal(b, &mout); e != nil {
			mout = map[string]any{}
			bad = append(bad, "⑮ emit-matrix：产物读不回来（"+e.Error()+"）")
		}
	} else {
		mout = map[string]any{}
		bad = append(bad, "⑮ emit-matrix：产物读不回来（"+err.Error()+"）")
	}
	casesOut := clicCasesOf(mout)
	by := map[string]map[string]any{}
	for _, c := range casesOut {
		by[clicMapStr(c, "id", "")] = c
	}
	wantBytes := len([]byte(fakeOut))
	wantChars := len([]rune(fakeOut))
	if wantBytes == wantChars {
		bad = append(bad, "⑮ 夹具不具区分度：中文输出的字节数与字符数相等")
	}
	var first map[string]any
	if len(casesOut) > 0 {
		first = casesOut[0]
	} else {
		first = map[string]any{}
	}
	seed0 := seed["cases"].([]any)[0].(map[string]any)
	for _, k := range []string{"id", "command", "argv", "why"} {
		if clicPyStr(clicMapGetOr(first, k, nil)) != clicPyStr(clicMapGetOr(seed0, k, nil)) ||
			clicPyStrListRepr(clicStrSlice(clicMapList(first, k))) != clicPyStrListRepr(clicStrSlice(clicMapList(seed0, k))) {
			if !clicSameVal(clicMapGetOr(first, k, nil), clicMapGetOr(seed0, k, nil)) {
				bad = append(bad, fmt.Sprintf("⑮ 手写格的 %s 没逐字保留（第 1 格 = %s）—— 整表重写的老病", k, clicPyStr(first)))
			}
		}
	}
	if !clicSameVal(clicMapGetOr(first, "want_rc", nil), 2.0) || !clicSameVal(clicMapGetOr(first, "want_stdout_bytes", nil), float64(wantBytes)) {
		bad = append(bad, fmt.Sprintf("⑮ 手写格的派生列没刷对：%s（要 rc=2 / %d 字节）", clicPyStr(first), wantBytes))
	}
	oldAuto := by["version#未知旗标"]
	oldAutoArgv := clicPyStrListRepr(clicStrSlice(clicMapList(oldAuto, "argv")))
	if oldAuto == nil || oldAutoArgv != clicPyStrListRepr([]string{"version", "--x"}) || clicMapStr(oldAuto, "why", "") != "旧 why（候选面不许覆盖它）" {
		bad = append(bad, fmt.Sprintf("⑮ 既有格的 argv/why 被候选面覆盖了：%s", clicPyStr(oldAuto)))
	}
	if oldAuto != nil {
		gotB, _ := clicPyInt(clicMapGetOr(oldAuto, "want_stdout_bytes", nil))
		gotRc, _ := clicPyInt(clicMapGetOr(oldAuto, "want_rc", nil))
		if gotRc != 2 || gotB != wantBytes {
			bad = append(bad, fmt.Sprintf("⑮ 既有格派生列没刷对：want_rc=%d want_stdout_bytes=%d（要 2 / %d 字节；记成 %d 就是**字符数** —— bug ①）", gotRc, gotB, wantBytes, wantChars))
		}
	}
	if _, ok := by["core/stop#缺 --confirm（D3）"]; !ok {
		bad = append(bad, "⑮ 命令树里有、矩阵里一格都没有的命令**没有**追加候选格（新命令会成空白）")
	}
	if !clicSameVal(clicMapGetOr(mout, "exemptions", nil), seed["exemptions"]) {
		bad = append(bad, fmt.Sprintf("⑮ 既有 exemptions 被丢了：%s", clicPyStr(clicMapGetOr(mout, "exemptions", nil))))
	}

	// ⑯ emit-matrix 幂等正控
	dst2 := clicPyJoin(tmp, "emit-out2.json")
	rc2, _ := clicEmitMatrix(r, &dst2)
	same := false
	if b1, e1 := os.ReadFile(dst); e1 == nil {
		if b2, e2 := os.ReadFile(dst2); e2 == nil {
			same = bytes.Equal(b1, b2)
		}
	}
	caseFn("⑯ emit-matrix 幂等（连跑两遍逐字节相同 ⇒ 0）", 0, rc2, "")
	if !same {
		bad = append(bad, "⑯ emit-matrix 不是幂等的：同一棵树上连跑两遍的产物不同")
	}

	// ⑰ R-6 正控
	r6 := clicPyJoin(tmp, "r6")
	clicFixture(r6, clicGoodMain, clicGoodExitcodes, clicGoodMatrix(), clicGoodBaseline(), clicGoodRegistry(), "")
	_ = os.MkdirAll(clicPyJoin(r6, "bin"), 0o755)
	_ = os.WriteFile(clicPyJoin(r6, "bin", "zerg"), []byte("#!/bin/sh\nexit 2\n"), 0o755)
	rc, lines = clicLiveVerify(r6, nil)
	caseFn("⑰ R-6 正控（账与盘逐格对得上 ⇒ 0）", 0, rc, strings.Join(lines, "\n"))
	if !clicAnyLine(lines, "合 2 / 2 格", "mismatch 0 格") {
		bad = append(bad, fmt.Sprintf("⑰ R-6 正控：判词没有「合 2 / 2 格 · mismatch 0 格」—— %s", clicPyStr(lines)))
	}
	rcChk, linesChk := clicCheck(r6, true)
	caseFn("⑰ R-6 正控（不动 ⇒ 该器仍 0 且四数不变）", 0, rcChk, strings.Join(linesChk, "\n"))

	// ⑱ R-6 负控（幽灵格）
	m4 := clicDeepCopy(clicGoodMatrix())
	m4["cases"].([]any)[1].(map[string]any)["want_rc"] = 0.0
	r6b := clicFixturePath(tmp, "r6b", m4)
	_ = os.MkdirAll(clicPyJoin(r6b, "bin"), 0o755)
	_ = os.WriteFile(clicPyJoin(r6b, "bin", "zerg"), []byte("#!/bin/sh\nexit 2\n"), 0o755)
	rc, lines = clicLiveVerify(r6b, nil)
	caseFn("⑱ R-6 负控（幽灵格：账 rc=0 盘 rc=2 ⇒ 1）", 1, rc, "")
	if !clicAnyLine(lines, "幽灵格", "core/stop") {
		bad = append(bad, fmt.Sprintf("⑱ R-6 负控：判词没有点名那一格 —— %s", clicPyStr(lines)))
	}

	// ⑲ R-6 负控（删一格）
	m4c := clicDeepCopy(clicGoodMatrix())
	var keptCases []any
	for _, c := range m4c["cases"].([]any) {
		if clicMapStr(c.(map[string]any), "command", "") != "core stop" {
			keptCases = append(keptCases, c)
		}
	}
	m4c["cases"] = keptCases
	rc, lines = clicCheck(clicFixturePath(tmp, "r6c", m4c), true)
	if rc == 0 {
		bad = append(bad, "⑲ R-6 负控：**删一格**之后该器仍 rc=0（命令没格也不红 ⇒ 账可以漏格）")
	}
	if !clicAnyLine(lines, "T1 覆盖", "core stop") {
		bad = append(bad, fmt.Sprintf("⑲ R-6 负控：删一格后判词没有点名 `core stop` —— %s", clicPyStr(lines)))
	}

	// ⑳ R-6 负控（没有制品 ⇒ 2）
	r6d := clicPyJoin(tmp, "r6d")
	clicFixture(r6d, clicGoodMain, clicGoodExitcodes, clicGoodMatrix(), clicGoodBaseline(), clicGoodRegistry(), "")
	rc, lines = clicLiveVerify(r6d, nil)
	caseFn("⑳ R-6 负控（没有制品 ⇒ 2 不给结论）", 2, rc, "")
	if !clicAnyLine(lines, "找不到 bin/zerg") {
		bad = append(bad, "⑳ R-6 负控：判词没有点名缺制品")
	}

	// ㉑ 字段表标识符/拼接解析（缺口 GAP-20260928-159：契约矩阵侧与字段面对拍侧「同源」）——
	//    `fields: zzFields,`（`append(append([]string{}, A...), B...)` 两片拼接变量）必须读成
	//    **有字段表**；旧「只认单行 []string{…}」的解析器在这一格必 FAIL（把同一张字段表读成「没有字段表」）。
	//    负控见 `check_cli_contract_test.go`（标识符未定义 ⇒ 仍不算有字段表）。
	r = clicPyJoin(tmp, "fldident")
	clicFixture(r, clicGoodMain, clicGoodExitcodes, clicGoodMatrix(), clicGoodBaseline(), clicGoodRegistry(), "")
	fp := clicPyJoin(r, clicMainRel)
	fb, _ := clicReadFile(fp)
	fb = strings.ReplaceAll(fb, "fields:  []string{\"name\"},", "fields:  zzFields,")
	fb += "\nvar zzShare = []string{\"probe\", \"plan\"}\nvar zzExtra = []string{\"apply\"}\nvar zzFields = append(append([]string{}, zzShare...), zzExtra...)\n"
	_ = os.WriteFile(fp, []byte(fb), 0o644)
	hasIdent := false
	for _, c := range clicParseTree(r) {
		if c.path == "version" {
			hasIdent = c.shape.fields
		}
	}
	if !hasIdent {
		bad = append(bad, "㉑ 字段表标识符/拼接解析：`fields: zzFields,`（append 两片拼接变量）应读成有字段表，实得 false（契约矩阵侧读成了「没有字段表」）")
	}

	verdict := "全过"
	if len(bad) > 0 {
		verdict = "不过"
	}
	fmt.Fprintf(w, "自检结论: %s（失败 %d 条）\n", verdict, len(bad))
	for _, b := range bad {
		fmt.Fprintln(w, "✗ "+b)
	}
	if len(bad) > 0 {
		return 2
	}
	return 0
}

func clicFixturePath(tmp, name string, matrix map[string]any) string {
	r := clicPyJoin(tmp, name)
	clicFixture(r, clicGoodMain, clicGoodExitcodes, matrix, clicGoodBaseline(), clicGoodRegistry(), "")
	return r
}

func clicFixturePath2(tmp, name string, registry map[string]any) string {
	r := clicPyJoin(tmp, name)
	clicFixture(r, clicGoodMain, clicGoodExitcodes, clicGoodMatrix(), clicGoodBaseline(), registry, "")
	return r
}

func clicAnyLine(lines []string, subs ...string) bool {
	for _, l := range lines {
		all := true
		for _, s := range subs {
			if !strings.Contains(l, s) {
				all = false
				break
			}
		}
		if all {
			return true
		}
	}
	return false
}

func clicSameVal(a, b any) bool {
	ab, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	return bytes.Equal(ab, bb)
}

// ── Q-176 机器读面拒收 ───────────────────────────────────────────────────────

var clicJSONFields = []string{}

func clicRefuseJSON(argv []string, stderr io.Writer) *int {
	if argv == nil {
		argv = []string{}
	}
	var hit []string
	for _, x := range argv {
		if x == "--json" || strings.HasPrefix(x, "--json=") {
			hit = append(hit, "`"+x+"`")
		}
	}
	if len(hit) == 0 {
		return nil
	}
	rc := 2
	fmt.Fprintf(stderr, "⛔ 本门无机器读面：%s 在本门**未实现**（缺口 Q-176）\n", strings.Join(hit, " · "))
	fields := "无（本门只出人读面）"
	if len(clicJSONFields) > 0 {
		var fs []string
		for _, f := range clicJSONFields {
			fs = append(fs, "`"+f+"`")
		}
		fields = strings.Join(fs, " · ")
	}
	fmt.Fprintf(stderr, "   字段表：%s\n", fields)
	fmt.Fprintf(stderr, "⇒ 用法错（rc=2）：**机器面缺 ≠ 零命中** —— 两态不许同形\n")
	return &rc
}

// ── 参数解析（argparse 形状）──────────────────────────────────────────────────

type clicArgs struct {
	root       string
	bin        string
	binSet     bool
	selfTest   bool
	noRuntime  bool
	emitMatrix *string
	dryRun     bool
	verifyLive bool
}

const clicProg = "check-cli-contract.py"

func clicUsage() string {
	return "usage: " + clicProg + " [-h] [--bin PATH] [--self-test] [--no-runtime]\n" +
		"                             [--emit-matrix [PATH] | --verify-live] [--dry-run]\n" +
		"                             [root]\n"
}

func clicHelp() string {
	return clicUsage() + "\n" +
		"positional arguments:\n" +
		"  root\n" +
		"\n" +
		"options:\n" +
		"  -h, --help            show this help message and exit\n" +
		"  --bin PATH            缺口 GAP-20260926-12：指定制品路径。不给 = <root>/bin/zerg；给了 = 用该路径（绝对化后）—— 仓外临时仓根复用本机真制品，免造 bin/\n" +
		"  --self-test\n" +
		"  --no-runtime          **只给夹具用**：跳过 T4 的 go test（真目标不许带这个开关）\n" +
		"  --emit-matrix [PATH]  按命令树 + 现跑 bin/zerg **重生成** must-fail 矩阵（先用 `zerg build all --only cli` 出新制品）\n" +
		"  --dry-run             **只判定 · 不落任何产物**（缺口 GAP-20260927-372）：与 --emit-matrix 同给时只报将要合并的增量、**不写**矩阵件；不给 --emit-matrix 时 = 只读判定（同裸跑）\n" +
		"  --verify-live         §六 R-6：拿冻结的矩阵逐格现跑同一 argv（每格两遍）、核 rc 与 stdout 字节数 —— 抓「多余格 / 幽灵格」（0 全合 / 1 mismatch / 2 不给结论）；★ 与 --emit-matrix **互斥**（同给 ⇒ 用法错 rc=2）\n"
}

func clicParseArgs(argv []string, stdout, stderr io.Writer) (clicArgs, *int) {
	a := clicArgs{root: "."}
	positional := 0
	i := 0
	fail := func(msg string) (clicArgs, *int) {
		rc := 2
		fmt.Fprint(stderr, clicUsage())
		fmt.Fprintf(stderr, "%s: error: %s\n", clicProg, msg)
		return a, &rc
	}
	for i < len(argv) {
		x := argv[i]
		switch {
		case x == "-h" || x == "--help":
			fmt.Fprint(stdout, clicHelp())
			rc := 0
			return a, &rc
		case x == "--self-test":
			a.selfTest = true
		case x == "--no-runtime":
			a.noRuntime = true
		case x == "--dry-run":
			a.dryRun = true
		case x == "--bin":
			if i+1 < len(argv) && !strings.HasPrefix(argv[i+1], "-") {
				a.bin = argv[i+1]
				a.binSet = true
				i++
			} else {
				return fail("argument --bin: expected one argument")
			}
		case strings.HasPrefix(x, "--bin="):
			a.bin = strings.TrimPrefix(x, "--bin=")
			a.binSet = true
		case x == "--verify-live":
			a.verifyLive = true
		case x == "--emit-matrix":
			if i+1 < len(argv) && !strings.HasPrefix(argv[i+1], "-") {
				v := argv[i+1]
				a.emitMatrix = &v
				i++
			} else {
				v := ""
				a.emitMatrix = &v
			}
		case strings.HasPrefix(x, "--emit-matrix="):
			v := strings.TrimPrefix(x, "--emit-matrix=")
			a.emitMatrix = &v
		case strings.HasPrefix(x, "-") && x != "-":
			return fail("unrecognized arguments: " + x)
		default:
			positional++
			if positional > 1 {
				return fail("unrecognized arguments: " + x)
			}
			a.root = x
		}
		i++
	}
	// ★ 缺口 `GAP-20260927-304`：`--emit-matrix` 与 `--verify-live` 两档**互斥**（按 argparse
	//   互斥组口径：同给 ⇒ 用法错 rc=2 · ✗ 不静默挑一个）。缘由：入口分派里 `if a.verifyLive`
	//   在 `if a.emitMatrix != nil` 之前 ⇒ 同给时**写矩阵那档被静默忽略**、只剩逐格回放；而 emit
	//   档自带的那次现读复核**只报不判**（`clicEmitMatrixOpt` 恒退 0）⇒ 同给时极易把「emit 的只报
	//   复核」误当验收证据。两样都要 ⇒ 分两次单跑，一次只做一件事。
	if a.emitMatrix != nil && a.verifyLive {
		return fail("argument --verify-live: not allowed with argument --emit-matrix")
	}
	return a, nil
}

// ── 入口 ─────────────────────────────────────────────────────────────────────

func cliContractGateMain(stdout, stderr io.Writer, argv []string) int {
	if rc := clicRefuseJSON(argv, stderr); rc != nil {
		return *rc
	}
	a, immediate := clicParseArgs(argv, stdout, stderr)
	if immediate != nil {
		return *immediate
	}
	if a.selfTest {
		return clicSelfTest(stdout)
	}
	root := clicFindRoot(a.root)
	if root == "" {
		fmt.Fprintf(stdout, "⛔ 不给结论：找不到仓根（从 %s 上溯找不到 %s）\n", a.root, clicMainRel)
		return 2
	}
	if a.verifyLive {
		rc, lines := clicLiveVerifyBin(root, nil, a.bin)
		fmt.Fprintf(stdout, "门⑪ · §六 `R-6` 逐格回放（仓根 %s）\n", root)
		for _, l := range lines {
			fmt.Fprintf(stdout, "  %s\n", l)
		}
		verdict := ""
		switch rc {
		case 0:
			verdict = "全合（rc=0）"
		case 1:
			verdict = "判红（rc=1）：账与盘对不上 —— 逐格看上面的判词"
		default:
			verdict = "不给结论（rc=2）：缺件/零格 —— 不许当绿"
		}
		fmt.Fprintf(stdout, "⇒ R-6 %s\n", verdict)
		return rc
	}
	if a.emitMatrix != nil {
		var op *string
		if *a.emitMatrix != "" {
			v := *a.emitMatrix
			op = &v
		}
		rc, lines := clicEmitMatrixOpt(root, op, a.dryRun, a.bin)
		for _, l := range lines {
			fmt.Fprintf(stdout, "  %s\n", l)
		}
		return rc
	}
	rc, lines := clicCheckBin(root, a.noRuntime, a.bin)
	fmt.Fprintf(stdout, "门⑪ 命令面契约（§九 M17 T1–T6）· 仓根 %s\n", root)
	for _, l := range lines {
		fmt.Fprintf(stdout, "  %s\n", l)
	}
	switch rc {
	case 0:
		fmt.Fprintf(stdout, "⇒ 命令面契约全过（rc=0）\n")
	case 1:
		fmt.Fprintf(stdout, "⇒ 命令面契约判红（rc=1）：逐条看上面的 T-编号\n")
	default:
		fmt.Fprintf(stdout, "⇒ 不给结论（rc=2）：缺件/空转/判不了 —— **不许当绿**\n")
	}
	return rc
}
