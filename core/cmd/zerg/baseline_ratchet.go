// baseline_ratchet.go —— M2 真搬：scripts/gates/baseline-ratchet.py
//
//	（`C2`：baseline **只许由一条命令改** · 只读 diff 先行 · 2026-09-23 · 波③）
//
// 来源件：scripts/gates/baseline-ratchet.py（py 真身 · 996 行 · sha16 5902c7f0d56da7f8）
// 本件是**逐字对拍**搬运：默认档（--view/--check）只读、逐字节对齐；写面（--write/--anchor）
//
//	一律只在**仓外夹具**上走，✗ 不写真实基线件、✗ 不碰真实台账。
//
// 出厂声明 (a) 默认根定位差异：py 从 `__file__` 上溯 N 层 ⟷ Go 无 `__file__` 语境，
//
//	只能从 cwd 上溯（`--root` 缺省 `.`）⇒「脚本位置 ≠ cwd」时路径推导面不同，✗ 不声称逐字对齐。
//
// 出厂声明 (b) 自指检查：本件落点 core/cmd/zerg/，而**本件自己的扫描面**是
//
//	CONDITIONS 里那四个「基线件」＋ 台账 scripts/gates/baseline-audit.jsonl ⇒ 本件**不在**其内（零自指）。
//	依据（量化）：本件正文只读点 = 4 个基线件 + 1 个台账件，全部不含 `core/cmd/zerg/`；
//	本件不含任何绝对私有路径字面量。
//	★ 但据「自指第四形态」，另门（扫 `core/` 的判据）可能扫到本件 ⇒ 落件前后做受控读数对照，见回执。
//
// 与 py 的**声明口径差**（逐字复刻不了、已具名）：
//
//	① argparse 帮助正文/用法退错正文：取自参考解释器 /usr/bin/python3 (3.9.6) 的**逐字**输出，
//	   随解释器版本会漂（3.10+ 为 `options:`）；本件按件头声明的解释器钉版。
//	② 现测命令真起子进程（subprocess）：`exec` 家族错误正文（FileNotFoundError/TimeoutExpired 的
//	   `%s` 文案）只对齐 rc/stdout，stderr 走声明文案 —— 见未覆盖面。
//	③ 正则语义差：py `\s`/`\d` 是 Unicode 语义，Go RE2 默认 ASCII ⇒ 非 ASCII 空白/数字输入未覆盖。
//	④ `int()` 认 Unicode 数字 ⟷ strconv 只认 ASCII（真仓读数全 ASCII）。
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const brProg = "baseline-ratchet.py"

// ── 台账件（append-only · 一件）───────────────────────────────────────────────
var brLedgerRel = brPyJoin("scripts", "gates", "baseline-audit.jsonl")

// ── 仓根标记（同门禁脚本的口径：入口 + 一棵构建树）────────────────────────────
var brRootMarkers = []string{
	brPyJoin("scripts", "gates", "precommit-gates.sh"),
	brPyJoin("core", "go.mod"),
}

// ── 条件表（声明式 · **不写死任何数字**：现值读件、现测现跑）────────────────────
type brCond struct {
	name      string
	gate      string
	file      string
	key       string
	policy    string
	unit      string
	measure   []string
	measureRe string
	// goMain —— ★ 退役态（2026-10-07 退役批）回落面：measure 的 py 件退役（不在盘）时，
	//   改用**同判据的 Go 半**（进程内跑 · 同一输出面 · 同一 measureRe 抽取）取同一读数
	//   ⇒ 退役态照常出结论（✗ 不 fail-closed）。✗ 不读 py。空 = 无 Go 半（仍 fail-closed）。
	goMain func(stdout, stderr io.Writer, argv []string) int
}

var brConditions = []brCond{
	{
		name:      "gate-coverage.baseline",
		gate:      "门①",
		file:      brPyJoin("scripts", "gates", "gate-coverage.config.json"),
		key:       "baseline",
		policy:    "down",
		unit:      "「闸文件里查无此名」的脚本条数",
		measure:   []string{"python3", brPyJoin("scripts", "gates", "check-gate-coverage.py"), "--baseline"},
		measureRe: `③\s*实测命中数[^=]*=\s*(\d+)`,
	},
	{
		name:      "cli-contract.blanks",
		gate:      "门⑪",
		file:      brPyJoin("scripts", "gates", "cli-contract-baseline.json"),
		key:       "blanks",
		policy:    "down",
		unit:      "命令面矩阵的空白格条数",
		measure:   []string{"python3", brPyJoin("scripts", "gates", "check-cli-contract.py")},
		measureRe: `空白\s*(\d+)\s*条`,
		goMain:    cliContractGateMain,
	},
	{
		name:      "doc-cmds.devdocs_report_hits",
		gate:      "门⑫",
		file:      brPyJoin("scripts", "gates", "doc-cmds-baseline.json"),
		key:       "devdocs_report_hits",
		policy:    "down",
		unit:      "开发文档面里手写命令示例的命中数",
		measure:   []string{"python3", brPyJoin("scripts", "gates", "check-doc-cmds.py"), "--scope", "devdocs"},
		measureRe: `命中\s*(\d+)\s*处`,
		goMain:    checkDocCmdsGateMain,
	},
	{
		name:      "i18n.allowed_count",
		gate:      "i18n 门禁",
		file:      brPyJoin("ui", "scripts", "i18n-baseline.json"),
		key:       "allowed_count",
		policy:    "down",
		unit:      "i18n 中文门禁基线白名单的**声明条数**（只许收不许放：白名单多一条 ⇒ 门 G4 当场红）",
		measure:   []string{"python3", brPyJoin("ui", "scripts", "check-i18n.py"), "--json"},
		measureRe: `"baseline_allowed"\s*:\s*(\d+)`,
	},
}

const brMeasureTimeout = 1800 // 秒（py MEASURE_TIMEOUT）

// ── py 路径/字符串原语（自写同构 · ✗ 不用 filepath.Join / strings.TrimSpace）──────

// brPyJoin = CPython posixpath.join 的逐段规则（含「后段绝对路径丢弃前段」「空段」两态）。
func brPyJoin(parts ...string) string {
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

// brPyNormpath = CPython posixpath.normpath（★ 保留恰两个前导斜杠）。
func brPyNormpath(path string) string {
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
	comps := strings.Split(path, "/")
	var newComps []string
	for _, comp := range comps {
		if comp == "" || comp == "." {
			continue
		}
		if comp != ".." || (initialSlashes == 0 && len(newComps) == 0) || (len(newComps) > 0 && newComps[len(newComps)-1] == "..") {
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

// brPyAbspath = CPython posixpath.abspath（纯词法：相对则 join(cwd,p)，再 normpath）。
func brPyAbspath(p string) string {
	if !strings.HasPrefix(p, "/") {
		cwd, err := os.Getwd()
		if err != nil {
			cwd = "."
		}
		p = brPyJoin(cwd, p)
	}
	return brPyNormpath(p)
}

// brPyDirname = CPython posixpath.dirname（= split 的头段 · 剥尾斜杠）。
func brPyDirname(p string) string {
	i := strings.LastIndex(p, "/") + 1
	head := p[:i]
	if head != "" && head != strings.Repeat("/", len(head)) {
		head = strings.TrimRight(head, "/")
	}
	return head
}

// brPyStrip = CPython str.strip()（★ 含 \x1c-\x1f · \x85 · \xa0 ⟷ Go unicode.IsSpace 不含 \x1c-\x1f）。
func brPyIsSpace(r rune) bool {
	if r >= 0x1c && r <= 0x1f {
		return true
	}
	return unicode.IsSpace(r)
}

func brPyStrip(s string) string {
	return strings.TrimFunc(s, brPyIsSpace)
}

func brPyLStrip(s string) string {
	return strings.TrimLeftFunc(s, brPyIsSpace)
}

// brPySplitLines = CPython str.splitlines()（在 \n \r \r\n \v \f \x1c-\x1e \x85 \u2028 \u2029 处切）。
func brPySplitLines(s string) []string {
	var out []string
	var cur strings.Builder
	rs := []rune(s)
	i := 0
	for i < len(rs) {
		r := rs[i]
		switch r {
		case '\n', '\v', '\f', 0x1c, 0x1d, 0x1e, 0x85, 0x2028, 0x2029:
			out = append(out, cur.String())
			cur.Reset()
			i++
		case '\r':
			out = append(out, cur.String())
			cur.Reset()
			if i+1 < len(rs) && rs[i+1] == '\n' {
				i += 2
			} else {
				i++
			}
		default:
			cur.WriteRune(r)
			i++
		}
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

// brPyRepr = 近似 CPython repr（只用于错误正文里的 %r；真仓路径上不触发）。
func brPyRepr(v any) string {
	switch x := v.(type) {
	case nil:
		return "None"
	case bool:
		if x {
			return "True"
		}
		return "False"
	case string:
		return "'" + strings.ReplaceAll(strings.ReplaceAll(x, "\\", "\\\\"), "'", "\\'") + "'"
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case json.Number:
		return x.String()
	case float64:
		return strconv.FormatFloat(x, 'g', -1, 64)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// ── 基础 ─────────────────────────────────────────────────────────────────────

func brIsFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func brIsDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

func brPathExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// brFindRoot = py find_root：从 start（abspath）上溯，两件标记全在 ⇒ 返根；否则 None。
func brFindRoot(start string) (string, bool) {
	cur := brPyAbspath(start)
	for {
		all := true
		for _, m := range brRootMarkers {
			if !brPathExists(brPyJoin(cur, m)) {
				all = false
				break
			}
		}
		if all {
			return cur, true
		}
		parent := brPyDirname(cur)
		if parent == cur {
			return "", false
		}
		cur = parent
	}
}

func brSha256File(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func brNowISO() string {
	return time.Now().Truncate(time.Second).Format("2006-01-02T15:04:05-07:00")
}

func brCondByName(name string, conds []brCond) *brCond {
	for i := range conds {
		if conds[i].name == name {
			return &conds[i]
		}
	}
	return nil
}

// ── JSON 取值（保号：用 json.Number 区分 int/float，与 py json.load 同形）───────

func brParseJSON(s string) (any, error) {
	dec := json.NewDecoder(strings.NewReader(s))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	// 尾随多余内容 ⇒ py json.load 报错
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("Extra data")
	}
	return v, nil
}

func brIsAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// ── 现值（读件）──────────────────────────────────────────────────────────────
// ⇒ (现值|nil, 件 sha256|nil, 错因|nil)。现值必须是**非负整数**（bool 不算）。
func brReadValue(root string, cond brCond) (*int, *string, *string) {
	path := brPyJoin(root, cond.file)
	if !brIsFile(path) {
		e := fmt.Sprintf("件不在：%s", cond.file)
		return nil, nil, &e
	}
	sha, err := brSha256File(path)
	if err != nil {
		e := fmt.Sprintf("件读不到：%s", cond.file)
		return nil, nil, &e
	}
	shaP := &sha
	b, rerr := os.ReadFile(path)
	if rerr != nil {
		e := fmt.Sprintf("件解不动（JSON）：%s", rerr)
		return nil, shaP, &e
	}
	docAny, jerr := brParseJSON(brPyUniversalNewlines(string(b)))
	if jerr != nil {
		e := fmt.Sprintf("件解不动（JSON）：%s", jerr)
		return nil, shaP, &e
	}
	doc, ok := docAny.(map[string]any)
	if !ok {
		e := fmt.Sprintf("件里没有键 `%s`", cond.key)
		return nil, shaP, &e
	}
	val, present := doc[cond.key]
	if !present {
		e := fmt.Sprintf("件里没有键 `%s`", cond.key)
		return nil, shaP, &e
	}
	n, ok := brNonNegInt(val)
	if !ok {
		e := fmt.Sprintf("键 `%s` 不是非负整数（现 %s）", cond.key, brPyRepr(val))
		return nil, shaP, &e
	}
	return &n, shaP, nil
}

// brNonNegInt：py 口径 —— bool 不算、int 且 >=0 才算。
func brNonNegInt(v any) (int, bool) {
	switch x := v.(type) {
	case json.Number:
		s := x.String()
		if brIsAllDigits(s) {
			n, err := strconv.Atoi(s)
			if err == nil {
				return n, true
			}
		}
		return 0, false
	default:
		return 0, false
	}
}

// brPyUniversalNewlines：open() 文本模式把 \r\n 与裸 \r 折成 \n。
func brPyUniversalNewlines(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return s
}

// ── 现测值（现跑该条件的门脚本 · **禁写常量**）───────────────────────────────
func brMeasureValue(root string, cond brCond, readings map[string]int, hasReadings bool) (*int, *string, *string) {
	if hasReadings {
		if v, ok := readings[cond.name]; ok {
			src := "注入读数（--readings）"
			vv := v
			return &vv, &src, nil
		}
	}
	// ★ 退役态（G19 · 2026-10-07 退役批）：现测命令 = ["python3", "<py>", args…]；该 py 件退役
	//   （不在盘）且本条件带 Go 半 ⇒ 进程内跑**同判据 Go 半**（gateCheckRunInproc 代管 cwd/env）
	//   取同一读数 ⇒ ✗ 不因缺 py 而「判不了 ⇒ 不给结论」（fail-closed）。
	//   py 件在盘时照旧真起子进程（两态逐字同 · 单一真源不变）。
	if cond.goMain != nil && len(cond.measure) >= 2 && cond.measure[0] == "python3" {
		if pyPath := brPyJoin(root, cond.measure[1]); !brIsFile(pyPath) {
			var so, se bytes.Buffer
			_ = gateCheckRunInproc(cond.goMain, root, &so, &se, cond.measure[2:])
			out := so.String() + "\n" + se.String()
			src := fmt.Sprintf("退役态回落（py 不在盘）：进程内跑 Go 半 · 原现测 `%s`", strings.Join(cond.measure, " "))
			return brMeasureExtract(out, cond, src)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), brMeasureTimeout*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, cond.measure[0], cond.measure[1:]...)
	cmd.Dir = root
	var so, se bytes.Buffer
	cmd.Stdout = &so
	cmd.Stderr = &se
	err := cmd.Run()
	joined := strings.Join(cond.measure, " ")
	if err != nil {
		if ee, ok := err.(*exec.Error); ok {
			e := fmt.Sprintf("现测命令起不来（%s）：%s", joined, ee.Err)
			return nil, nil, &e
		}
		if ctx.Err() == context.DeadlineExceeded {
			e := fmt.Sprintf("现测命令超时（%ds）：%s", brMeasureTimeout, joined)
			return nil, nil, &e
		}
	}
	rc := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			rc = ee.ExitCode()
		} else {
			e := fmt.Sprintf("现测命令起不来（%s）：%s", joined, err)
			return nil, nil, &e
		}
	}
	out := so.String() + "\n" + se.String()
	src := fmt.Sprintf("现跑 `%s`（rc=%d）", joined, rc)
	return brMeasureExtract(out, cond, src)
}

// brMeasureExtract —— 从现测输出里按 cond.measureRe 抽判据行（py 原 tail 逐字同）。
// 抽不到 ⇒ 判不了（✗ 不猜）；供「真起子进程」与「退役态 Go 半回落」两路共用同一抽取面。
func brMeasureExtract(out string, cond brCond, src string) (*int, *string, *string) {
	re, rerr := regexp.Compile(cond.measureRe)
	if rerr != nil {
		e := fmt.Sprintf("现测命令输出里找不到判据行（%s）⇒ **判不了**（不许猜）\n", cond.measureRe)
		return nil, &src, &e
	}
	m := re.FindStringSubmatch(out)
	if m == nil {
		lines := brPySplitLines(brPyStrip(out))
		if len(lines) > 6 {
			lines = lines[len(lines)-6:]
		}
		e := fmt.Sprintf("现测命令输出里找不到判据行（%s）⇒ **判不了**（不许猜）\n%s", cond.measureRe, strings.Join(lines, "\n"))
		return nil, &src, &e
	}
	n, cerr := strconv.Atoi(m[1])
	if cerr != nil {
		// py int() 认 Unicode 数字；ASCII 失败档（已声明未覆盖面）⇒ 不给结论
		e := fmt.Sprintf("现测命令输出里找不到判据行（%s）⇒ **判不了**（不许猜）\n%s", cond.measureRe, m[1])
		return nil, &src, &e
	}
	return &n, &src, nil
}

// ── 读数件 ───────────────────────────────────────────────────────────────────
func brReadReadings(path string, conds []brCond) (map[string]int, *string) {
	got := map[string]int{}
	var errs []string
	b, err := os.ReadFile(path)
	if err != nil {
		e := fmt.Sprintf("读数件读不到：%s", err)
		return nil, &e
	}
	lines := brPySplitLines(brPyUniversalNewlines(string(b)))
	for i, line := range lines {
		if strings.HasPrefix(brPyLStrip(line), "#") {
			continue
		}
		s := brPyStrip(line)
		if s == "" {
			continue
		}
		if !strings.Contains(s, "=") {
			errs = append(errs, fmt.Sprintf("第 %d 行缺 `=`（要 `条件名=值`）", i+1))
			continue
		}
		idx := strings.Index(s, "=")
		name := brPyStrip(s[:idx])
		val := brPyStrip(s[idx+1:])
		if brCondByName(name, conds) == nil {
			errs = append(errs, fmt.Sprintf("第 %d 行的条件名不认：%s", i+1, name))
			continue
		}
		if !brIsAllDigits(val) {
			errs = append(errs, fmt.Sprintf("第 %d 行的值不是非负整数：%s", i+1, brPyRepr(val)))
			continue
		}
		n, _ := strconv.Atoi(val)
		got[name] = n
	}
	if len(errs) > 0 {
		e := strings.Join(errs, "；")
		return nil, &e
	}
	return got, nil
}

// ── 台账 ─────────────────────────────────────────────────────────────────────
func brLedgerPath(root string) string {
	return brPyJoin(root, brLedgerRel)
}

// brLoadLedger ⇒ (entries, 错因|nil)。空/缺件 ⇒ ([], None)。
func brLoadLedger(root string) ([]map[string]any, *string) {
	path := brLedgerPath(root)
	if !brIsFile(path) {
		return []map[string]any{}, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		e := fmt.Sprintf("台账读不到：%s", err)
		return []map[string]any{}, &e
	}
	entries := []map[string]any{}
	var errs []string
	lines := brPySplitLines(brPyUniversalNewlines(string(b)))
	for i, line := range lines {
		s := brPyStrip(line)
		if s == "" {
			continue
		}
		v, jerr := brParseJSON(s)
		if jerr != nil {
			errs = append(errs, fmt.Sprintf("第 %d 行解不动：%s", i+1, jerr))
			continue
		}
		m, ok := v.(map[string]any)
		if !ok {
			errs = append(errs, fmt.Sprintf("第 %d 行不是一条台账记录（缺 seq/name）", i+1))
			continue
		}
		if _, ok1 := m["seq"]; !ok1 {
			errs = append(errs, fmt.Sprintf("第 %d 行不是一条台账记录（缺 seq/name）", i+1))
			continue
		}
		if _, ok2 := m["name"]; !ok2 {
			errs = append(errs, fmt.Sprintf("第 %d 行不是一条台账记录（缺 seq/name）", i+1))
			continue
		}
		entries = append(entries, m)
	}
	if len(errs) > 0 {
		e := strings.Join(errs, "；")
		return entries, &e
	}
	return entries, nil
}

// ── 台账两态错因码（缺口 Q-144 · 只加判因，✗ 不动退码）────────────────────────
const brLedgerAbsent = "ledger_absent"
const brLedgerNoRecord = "no_record"

var brLedgerReasonText = map[string]string{
	brLedgerAbsent:   "台账件**不在盘上**：" + brLedgerRel + " —— 真源缺件（「读不到」不许当绿）",
	brLedgerNoRecord: "台账件**在盘上、却一条记录都没有**：" + brLedgerRel + " —— 手改/新建都无处对账",
}

// brLedgerReason ⇒ ledger_absent / no_record / nil
func brLedgerReason(root string) *string {
	if !brIsFile(brLedgerPath(root)) {
		s := brLedgerAbsent
		return &s
	}
	entries, lerr := brLoadLedger(root)
	if lerr != nil {
		return nil
	}
	if len(entries) == 0 {
		s := brLedgerNoRecord
		return &s
	}
	h, _ := brLedgerShapeErrors(entries)
	if len(h) > 0 {
		return nil
	}
	return nil
}

// brLedgerShapeErrors ⇒ (hard, soft)
func brLedgerShapeErrors(entries []map[string]any) ([]string, []string) {
	hard := []string{}
	soft := []string{}
	for i, e := range entries {
		if !brNumEquals(e["seq"], i+1) {
			hard = append(hard, fmt.Sprintf("第 %d 条的 seq=%s（连号要求 = %d ⇒ 有删行/改写 ⇒ append-only 被破）",
				i+1, brPyRepr(e["seq"]), i+1))
		}
	}
	lastNew := map[string]any{}
	haveNew := map[string]bool{}
	lastSha := map[string]any{}
	haveSha := map[string]bool{}
	for _, e := range entries {
		nameRaw := e["name"]
		fRaw := e["file"]
		name := brPyRepr(nameRaw)
		f := brPyRepr(fRaw)
		if haveNew[name] && !brAnyEquals(e["old"], lastNew[name]) {
			soft = append(soft, fmt.Sprintf("`%s` 的值链断了：上一条 new=%s · 本条 old=%s",
				brStrVal(nameRaw), brPyRepr(lastNew[name]), brPyRepr(e["old"])))
		}
		if haveSha[f] && !brAnyEquals(e["sha256_before"], lastSha[f]) {
			soft = append(soft, fmt.Sprintf("件 `%s` 的件链断了：上一条 sha256_after=%s · 本条 sha256_before=%s（件被别的批动过）",
				brStrVal(fRaw), brPyRepr(lastSha[f]), brPyRepr(e["sha256_before"])))
		}
		lastNew[name] = e["new"]
		haveNew[name] = true
		lastSha[f] = e["sha256_after"]
		haveSha[f] = true
	}
	return hard, soft
}

func brStrVal(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return brPyRepr(v)
}

func brNumEquals(v any, n int) bool {
	switch x := v.(type) {
	case json.Number:
		f, err := strconv.ParseFloat(x.String(), 64)
		return err == nil && f == float64(n)
	default:
		return false
	}
}

func brAnyEquals(a, b any) bool {
	if a == nil && b == nil {
		return true
	}
	an, aok := a.(json.Number)
	bn, bok := b.(json.Number)
	if aok && bok {
		return an.String() == bn.String()
	}
	as, aok2 := a.(string)
	bs, bok2 := b.(string)
	if aok2 && bok2 {
		return as == bs
	}
	if a == nil || b == nil {
		return false
	}
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

// ── 手写 JSON（对齐 py json.dumps：缩进两空格 / 保键插入序 / ensure_ascii=False ──
//    以及 compact 档的 `, `/`: ` 分隔符）────────────────────────────────────────

type brObj struct {
	keys []string
	vals []any
}

func brNewObj(kv ...any) *brObj {
	o := &brObj{}
	for i := 0; i+1 < len(kv); i += 2 {
		o.keys = append(o.keys, kv[i].(string))
		o.vals = append(o.vals, kv[i+1])
	}
	return o
}

func (o *brObj) put(k string, v any) {
	o.keys = append(o.keys, k)
	o.vals = append(o.vals, v)
}

func brWriteJSONString(sb *strings.Builder, s string) {
	sb.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			sb.WriteString(`\"`)
		case '\\':
			sb.WriteString(`\\`)
		case '\n':
			sb.WriteString(`\n`)
		case '\r':
			sb.WriteString(`\r`)
		case '\t':
			sb.WriteString(`\t`)
		case '\b':
			sb.WriteString(`\b`)
		case '\f':
			sb.WriteString(`\f`)
		default:
			if r < 0x20 {
				sb.WriteString(fmt.Sprintf(`\u%04x`, r))
			} else {
				sb.WriteRune(r)
			}
		}
	}
	sb.WriteByte('"')
}

func brDumpJSON(sb *strings.Builder, v any, indent int) {
	switch x := v.(type) {
	case nil:
		sb.WriteString("null")
	case bool:
		if x {
			sb.WriteString("true")
		} else {
			sb.WriteString("false")
		}
	case int:
		sb.WriteString(strconv.Itoa(x))
	case int64:
		sb.WriteString(strconv.FormatInt(x, 10))
	case json.Number:
		sb.WriteString(x.String())
	case string:
		brWriteJSONString(sb, x)
	case *brObj:
		if len(x.keys) == 0 {
			sb.WriteString("{}")
			return
		}
		sb.WriteString("{\n")
		for i, k := range x.keys {
			sb.WriteString(strings.Repeat("  ", indent+1))
			brWriteJSONString(sb, k)
			sb.WriteString(": ")
			brDumpJSON(sb, x.vals[i], indent+1)
			if i < len(x.keys)-1 {
				sb.WriteString(",")
			}
			sb.WriteString("\n")
		}
		sb.WriteString(strings.Repeat("  ", indent))
		sb.WriteString("}")
	case []any:
		if len(x) == 0 {
			sb.WriteString("[]")
			return
		}
		sb.WriteString("[\n")
		for i, e := range x {
			sb.WriteString(strings.Repeat("  ", indent+1))
			brDumpJSON(sb, e, indent+1)
			if i < len(x)-1 {
				sb.WriteString(",")
			}
			sb.WriteString("\n")
		}
		sb.WriteString(strings.Repeat("  ", indent))
		sb.WriteString("]")
	case []string:
		if len(x) == 0 {
			sb.WriteString("[]")
			return
		}
		sb.WriteString("[\n")
		for i, e := range x {
			sb.WriteString(strings.Repeat("  ", indent+1))
			brWriteJSONString(sb, e)
			if i < len(x)-1 {
				sb.WriteString(",")
			}
			sb.WriteString("\n")
		}
		sb.WriteString(strings.Repeat("  ", indent))
		sb.WriteString("]")
	default:
		sb.WriteString("null")
	}
}

// brDumpCompact = py json.dumps(obj, ensure_ascii=False)（默认分隔符 `, `/`: `）—— 台账行用。
func brDumpCompact(sb *strings.Builder, v any) {
	switch x := v.(type) {
	case nil:
		sb.WriteString("null")
	case bool:
		if x {
			sb.WriteString("true")
		} else {
			sb.WriteString("false")
		}
	case int:
		sb.WriteString(strconv.Itoa(x))
	case int64:
		sb.WriteString(strconv.FormatInt(x, 10))
	case json.Number:
		sb.WriteString(x.String())
	case string:
		brWriteJSONString(sb, x)
	case *brObj:
		sb.WriteString("{")
		for i, k := range x.keys {
			if i > 0 {
				sb.WriteString(", ")
			}
			brWriteJSONString(sb, k)
			sb.WriteString(": ")
			brDumpCompact(sb, x.vals[i])
		}
		sb.WriteString("}")
	case []any:
		sb.WriteString("[")
		for i, e := range x {
			if i > 0 {
				sb.WriteString(", ")
			}
			brDumpCompact(sb, e)
		}
		sb.WriteString("]")
	default:
		sb.WriteString("null")
	}
}

// ── 台账追加 ─────────────────────────────────────────────────────────────────
func brAppendLedger(root string, entry *brObj) *string {
	var sb strings.Builder
	brDumpCompact(&sb, entry)
	line := sb.String()
	if strings.Contains(line, "\n") {
		e := "台账行里不许有换行"
		return &e
	}
	path := brLedgerPath(root)
	d := brPyDirname(path)
	if d != "" && !brIsDir(d) {
		e := fmt.Sprintf("台账目录不在：%s", d)
		return &e
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		e := fmt.Sprintf("台账追不上：%s", err)
		return &e
	}
	defer f.Close()
	if _, err := f.WriteString(line + "\n"); err != nil {
		e := fmt.Sprintf("台账追不上：%s", err)
		return &e
	}
	if err := f.Sync(); err != nil {
		e := fmt.Sprintf("台账追不上：%s", err)
		return &e
	}
	return nil
}

// ── 判 ───────────────────────────────────────────────────────────────────────
func brVerdictOf(cond brCond, curVal *int, curSha *string, entries []map[string]any) (string, string) {
	var es []map[string]any
	for _, e := range entries {
		if s, ok := e["name"].(string); ok && s == cond.name {
			es = append(es, e)
		}
	}
	if len(es) == 0 {
		return "red", "无记录：件里这个棘轮数字**一条台账记录都没有** ⇒ 判红（`R-03`）"
	}
	last := es[len(es)-1]
	if curVal == nil || !brNumEquals(last["new"], *curVal) {
		return "red", fmt.Sprintf("现值 %s ≠ 台账最后一条 new %s ⇒ 有一条**没记录的手改** ⇒ 判红（改动只许走 `--write`）",
			brStrOfInt(curVal), brPyRepr(last["new"]))
	}
	if sa, ok := last["sha256_after"].(string); ok && sa != "" && curSha != nil && sa != *curSha {
		return "warn", fmt.Sprintf("值有记录且对得上，但**件另有改动**（键值没动 ⇒ 不改判据宽度，只提示）：台账 %s… · 现件 %s…",
			brHead(sa, 12), brHead(*curSha, 12))
	}
	return "ok", fmt.Sprintf("台账最后一条：%s → %s（%s · %s）",
		brPyRepr(last["old"]), brPyRepr(last["new"]), brStrVal(last["ts"]), brStrVal(last["kind"]))
}

func brStrOfInt(p *int) string {
	if p == nil {
		return "None"
	}
	return strconv.Itoa(*p)
}

func brHead(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func brVerdictRC(rows []brRow, hard []string) int {
	if len(hard) > 0 {
		return 2
	}
	for _, r := range rows {
		if r.verdict == "red" {
			return 1
		}
	}
	return 0
}

type brRow struct {
	cond        brCond
	cur         *int
	sha         *string
	err         *string
	meas        *int
	measSrc     *string
	measErr     *string
	verdict     string
	verdictText string
}

func brCollect(root string, conds []brCond, readings map[string]int, hasReadings bool, only []string, withMeasure bool) ([]brRow, []string, []string) {
	entries, lerr := brLoadLedger(root)
	hard := []string{}
	soft := []string{}
	if lerr != nil {
		hard = append(hard, fmt.Sprintf("台账解不动：%s", *lerr))
	} else {
		h, s := brLedgerShapeErrors(entries)
		hard = append(hard, h...)
		soft = append(soft, s...)
	}
	rows := []brRow{}
	for _, cond := range conds {
		if len(only) > 0 && !brContains(only, cond.name) {
			continue
		}
		cur, sha, verr := brReadValue(root, cond)
		var mv *int
		var msrc, merr *string
		if withMeasure {
			mv, msrc, merr = brMeasureValue(root, cond, readings, hasReadings)
		}
		vd := ""
		vtxt := ""
		if verr != nil {
			vd = "n/a"
			vtxt = fmt.Sprintf("判不了：%s", *verr)
		} else {
			vd, vtxt = brVerdictOf(cond, cur, sha, entries)
		}
		rows = append(rows, brRow{cond: cond, cur: cur, sha: sha, err: verr,
			meas: mv, measSrc: msrc, measErr: merr, verdict: vd, verdictText: vtxt})
	}
	return rows, hard, soft
}

func brContains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

func brDirection(cur, meas *int) string {
	if meas == nil || cur == nil {
		return "—"
	}
	if *meas > *cur {
		return "将升"
	}
	if *meas < *cur {
		return "将降"
	}
	return "不变"
}

// ── 写面（唯一通道）──────────────────────────────────────────────────────────
func brRewriteKey(root string, cond brCond, newVal int) (*string, *string) {
	path := brPyJoin(root, cond.file)
	b, err := os.ReadFile(path)
	if err != nil {
		e := fmt.Sprintf("件读不到：%s", err)
		return nil, &e
	}
	raw := brPyUniversalNewlines(string(b))
	dq := string(rune(0x22))
	pat := regexp.MustCompile("(" + dq + regexp.QuoteMeta(cond.key) + dq + `\s*:\s*)(\d+)`)
	ms := pat.FindAllStringSubmatchIndex(raw, -1)
	if len(ms) != 1 {
		e := fmt.Sprintf("件里 `%s` 命中 %d 次（要**恰好 1 次**，多了少了都不许改）", cond.key, len(ms))
		return nil, &e
	}
	loc := ms[0]
	newRaw := raw[:loc[4]] + strconv.Itoa(newVal) + raw[loc[5]:]
	if newRaw == raw {
		e := "改写后与原文逐字相同（没有可改的东西）"
		return nil, &e
	}
	if err := os.WriteFile(path, []byte(newRaw), 0o644); err != nil {
		e := fmt.Sprintf("件写不进去：%s", err)
		return nil, &e
	}
	sha, err := brSha256File(path)
	if err != nil {
		e := fmt.Sprintf("件写不进去：%s", err)
		return nil, &e
	}
	return &sha, nil
}

func brWhoAmI(root string) string {
	for _, key := range []string{"user.name", "user.email"} {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		c := exec.CommandContext(ctx, "git", "config", key)
		c.Dir = root
		_ = c.Run()
		cancel()
	}
	name := ""
	email := ""
	for _, kg := range [][2]string{{"user.name", "name"}, {"user.email", "email"}} {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		c := exec.CommandContext(ctx, "git", "config", kg[0])
		c.Dir = root
		var so bytes.Buffer
		c.Stdout = &so
		_ = c.Run()
		cancel()
		val := brPyStrip(so.String())
		if kg[1] == "name" {
			name = val
		} else {
			email = val
		}
	}
	if name != "" && email != "" {
		return name + " <" + email + ">"
	}
	if name != "" {
		return name
	}
	if email != "" {
		return email
	}
	return "（未登记 · 本机 git 身份读不到）"
}

// ── 打印 ─────────────────────────────────────────────────────────────────────
func brPrintRows(w io.Writer, rows []brRow) {
	width := 10
	for _, r := range rows {
		if n := utf8.RuneCountInString(r.cond.name); n > width {
			width = n
		}
	}
	for _, r := range rows {
		c := r.cond
		cur := "—"
		if r.cur != nil {
			cur = strconv.Itoa(*r.cur)
		}
		fmt.Fprintf(w, "   %-*s  %-4s  %-46s 现值 %-5s  %s\n", width, c.name, c.gate, c.file, cur, r.verdictText)
		if r.meas != nil {
			fmt.Fprintf(w, "   %-*s        现测 %-5s（%s）⇒ %s\n", width, "", strconv.Itoa(*r.meas), brStrDeref(r.measSrc), brDirection(r.cur, r.meas))
		}
	}
}

func brStrDeref(p *string) string {
	if p == nil {
		return "None"
	}
	return *p
}

func brPrintDiff(w io.Writer, rows []brRow, hard []string, soft []string) {
	fmt.Fprint(w, "── baseline 差值（只读 · **不写任何件**）──\n")
	up, down, same := 0, 0, 0
	type um struct{ name, err string }
	var unmeasured []um
	for _, r := range rows {
		c := r.cond
		if r.measErr != nil {
			unmeasured = append(unmeasured, um{c.name, *r.measErr})
			fmt.Fprintf(w, "   %-32s 现值 %-5s 现测 **判不了** ⇒ 不给结论\n", c.name, brStrOfInt(r.cur))
			continue
		}
		d := brDirection(r.cur, r.meas)
		delta := 0
		if r.cur != nil && r.meas != nil {
			delta = *r.meas - *r.cur
		}
		switch d {
		case "将升":
			up++
		case "将降":
			down++
		default:
			same++
		}
		fmt.Fprintf(w, "   %-32s %-5s 现值 %-5s → 现测 %-5s  %s（%+d）· %s · %s\n",
			c.name, c.gate, brStrOfInt(r.cur), brStrOfInt(r.meas), d, delta, c.unit, brStrDeref(r.measSrc))
	}
	fmt.Fprintf(w, "   ⇒ **将升 %d 条 · 将降 %d 条 · 不变 %d 条**（逐条件名见上）\n", up, down, same)
	if up > 0 {
		fmt.Fprint(w, "   ★ 将升 = **放宽判据**：`R-03` 要求可升，但**必须留痕** ⇒ `--write --reason '…' --allow-raise`\n")
	}
	if down > 0 {
		fmt.Fprint(w, "   ★ 将降 = 顺棘轮（只许下调）：`--write --reason '…'`\n")
	}
	for _, u := range unmeasured {
		fmt.Fprintf(w, "   ⛔ `%s` 判不了：%s\n", u.name, u.err)
	}
	for _, e := range hard {
		fmt.Fprintf(w, "   ⛔ 台账有硬错：%s\n", e)
	}
	for _, e := range soft {
		fmt.Fprintf(w, "   ⚠ 台账提示（不改判）：%s\n", e)
	}
}

func brPrintView(w io.Writer, root string, rows []brRow, hard []string, soft []string) {
	fmt.Fprint(w, "── baseline 只读视图（唯一真源 = 本件；不写任何件）──\n")
	entries, _ := brLoadLedger(root)
	state := "**件还不存在**"
	if brIsFile(brLedgerPath(root)) {
		state = "件在"
	}
	fmt.Fprintf(w, "   台账：%s（%d 条记录 · %s）\n", brLedgerRel, len(entries), state)
	brPrintRows(w, rows)
	for _, e := range hard {
		fmt.Fprintf(w, "   ⛔ 台账硬错：%s\n", e)
	}
	for _, e := range soft {
		fmt.Fprintf(w, "   ⚠ 台账提示（不改判）：%s\n", e)
	}
}

func brWriteJSON(w io.Writer, payload *brObj) {
	var sb strings.Builder
	brDumpJSON(&sb, payload, 0)
	sb.WriteString("\n")
	fmt.Fprint(w, sb.String())
}

// ── 各档 ─────────────────────────────────────────────────────────────────────
func brIntOrNil(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}

func brStrOrNil(p *string) any {
	if p == nil {
		return nil
	}
	return *p
}

func brStrsAny(xs []string) []any {
	out := []any{}
	for _, x := range xs {
		out = append(out, x)
	}
	return out
}

func brSeqInt(v any) (int, bool) {
	if n, ok := v.(json.Number); ok {
		if i, err := strconv.Atoi(n.String()); err == nil {
			return i, true
		}
	}
	return 0, false
}

func brDoStatus(w io.Writer, root string, a *brArgs, conds []brCond, mode string) int {
	only := a.only
	if mode == "diff" {
		var readings map[string]int
		hasReadings := false
		if a.readings != "" {
			r, rerr := brReadReadings(a.readings, conds)
			if rerr != nil {
				fmt.Fprintf(w, "⛔ 读数件解不动：%s ⇒ 不给结论（rc=2）\n", *rerr)
				return 2
			}
			readings, hasReadings = r, true
		}
		rows, hard, soft := brCollect(root, conds, readings, hasReadings, only, true)
		if len(rows) == 0 {
			fmt.Fprint(w, "⛔ 一个条件都没选中 ⇒ 不给结论（rc=2）\n")
			return 2
		}
		if a.jsonOut {
			rowsArr := []any{}
			for _, r := range rows {
				o := brNewObj("cur", brIntOrNil(r.cur), "sha", brStrOrNil(r.sha), "meas", brIntOrNil(r.meas),
					"meas_src", brStrOrNil(r.measSrc), "meas_err", brStrOrNil(r.measErr))
				o.put("name", r.cond.name)
				o.put("gate", r.cond.gate)
				o.put("file", r.cond.file)
				o.put("direction", brDirection(r.cur, r.meas))
				rowsArr = append(rowsArr, o)
			}
			payload := brNewObj("mode", "diff", "rows", rowsArr,
				"ledger_hard_errors", brStrsAny(hard), "ledger_soft_notes", brStrsAny(soft))
			brWriteJSON(w, payload)
			for _, r := range rows {
				if r.measErr != nil {
					return 2
				}
			}
			return 0
		}
		brPrintDiff(w, rows, hard, soft)
		anyMeasErr := false
		for _, r := range rows {
			if r.measErr != nil {
				anyMeasErr = true
			}
		}
		if anyMeasErr {
			fmt.Fprint(w, "⇒ **不给结论（rc=2）**：有条件的现测值取不到（现跑命令失败/解析不出判据行）\n")
			return 2
		}
		fmt.Fprint(w, "⇒ 只报差跑成（rc=0）：差是事实，红是「没记录」——要留痕见 `--write`\n")
		return 0
	}

	rows, hard, soft := brCollect(root, conds, nil, false, only, false)
	if len(rows) == 0 {
		fmt.Fprint(w, "⛔ 一个条件都没选中 ⇒ 不给结论（rc=2）\n")
		return 2
	}
	reds := 0
	for _, r := range rows {
		if r.verdict == "red" {
			reds++
		}
	}
	reason := brLedgerReason(root)
	if a.jsonOut {
		rowsArr := []any{}
		for _, r := range rows {
			rowsArr = append(rowsArr, brNewObj("name", r.cond.name, "gate", r.cond.gate, "file", r.cond.file,
				"value", brIntOrNil(r.cur), "sha256", brStrOrNil(r.sha), "err", brStrOrNil(r.err),
				"verdict", r.verdict, "verdict_text", r.verdictText))
		}
		payload := brNewObj("mode", "view", "ledger", brLedgerRel, "reason", brStrOrNil(reason),
			"rows", rowsArr, "ledger_hard_errors", brStrsAny(hard), "ledger_soft_notes", brStrsAny(soft))
		brWriteJSON(w, payload)
	} else {
		brPrintView(w, root, rows, hard, soft)
	}
	if len(hard) > 0 {
		fmt.Fprint(w, "⇒ **不给结论（rc=2）**：台账本身解不动/形状坏了\n")
		return 2
	}
	rc := brVerdictRC(rows, hard)
	if rc == 1 {
		fmt.Fprintf(w, "⇒ **判红（rc=1）**：%d 个条件无记录或与台账不符（逐条见上）\n", reds)
		if reason != nil {
			fmt.Fprintf(w, "   ⛔ 错因（机器可判 · `Q-144`）：reason=%s —— %s\n", *reason, brLedgerReasonText[*reason])
		}
		fmt.Fprint(w, "   闭洞的唯一通道：`--write --reason '…'`（改数 + 留痕）或 `--anchor --reason '…'`（补现状锚，不改件）\n")
		return 1
	}
	fmt.Fprint(w, "⇒ 绿（rc=0）：每个棘轮数字都能在台账里找到它自己那条记录，且现值对得上\n")
	return rc
}

func brDoWrite(w io.Writer, root string, a *brArgs, conds []brCond) int {
	if brPyStrip(a.reason) == "" {
		fmt.Fprint(w, "⛔ `--write` 必须带 `--reason '…'`（`R-03` 逐字：**必须留「谁在何时为什么」**） ⇒ fail-closed，**不写**（rc=2）\n")
		return 2
	}
	var readings map[string]int
	hasReadings := false
	if a.readings != "" {
		r, rerr := brReadReadings(a.readings, conds)
		if rerr != nil {
			fmt.Fprintf(w, "⛔ 读数件解不动：%s ⇒ 不给结论（rc=2）\n", *rerr)
			return 2
		}
		readings, hasReadings = r, true
	}
	rows, hard, soft := brCollect(root, conds, readings, hasReadings, a.only, true)
	if len(hard) > 0 {
		fmt.Fprint(w, "⛔ 台账有硬错（append-only 的连号证据不过）⇒ 先把台账修对，不给结论（rc=2）：\n")
		for _, e := range hard {
			fmt.Fprintf(w, "   · %s\n", e)
		}
		return 2
	}
	for _, e := range soft {
		fmt.Fprintf(w, "   ⚠ 台账提示（不改判）：%s\n", e)
	}
	if len(rows) == 0 {
		fmt.Fprint(w, "⛔ 一个条件都没选中 ⇒ 不给结论（rc=2）\n")
		return 2
	}
	var blocked []brRow
	for _, r := range rows {
		if r.measErr != nil || r.err != nil {
			blocked = append(blocked, r)
		}
	}
	if len(blocked) > 0 {
		for _, r := range blocked {
			msg := ""
			if r.measErr != nil {
				msg = *r.measErr
			} else {
				msg = *r.err
			}
			fmt.Fprintf(w, "⛔ `%s` 取不到数：%s\n", r.cond.name, msg)
		}
		fmt.Fprint(w, "⇒ 有条件的数取不到 ⇒ **一个字节都不写**（不给结论 rc=2）\n")
		return 2
	}
	var raises []brRow
	for _, r := range rows {
		if r.meas != nil && r.cur != nil && *r.meas > *r.cur {
			raises = append(raises, r)
		}
	}
	if len(raises) > 0 && !a.allowRaise {
		for _, r := range raises {
			fmt.Fprintf(w, "⛔ `%s` 要**上调** %s → %s（= **放宽判据**）：`R-03` 允许升，但必须显式\n",
				r.cond.name, brStrOfInt(r.cur), brStrOfInt(r.meas))
		}
		fmt.Fprint(w, "⇒ 上调须带 `--allow-raise`（fail-closed：不加 ⇒ 不写 rc=2）\n")
		return 2
	}
	by := a.by
	if by == "" {
		by = brWhoAmI(root)
	}
	doneCount := 0
	overCount := 0
	for _, r := range rows {
		cond := r.cond
		old := r.cur
		newv := r.meas
		shaBefore := r.sha
		if old != nil && newv != nil && *newv == *old {
			fmt.Fprintf(w, "   · `%s` 现测 = 现值（%s）⇒ **不改件**；要留痕请用 `--anchor`\n", cond.name, brStrOfInt(old))
			continue
		}
		entries, _ := brLoadLedger(root)
		seq := 1
		if len(entries) > 0 {
			mx := 0
			for _, e := range entries {
				if n, ok := brSeqInt(e["seq"]); ok && n > mx {
					mx = n
				}
			}
			seq = mx + 1
		}
		shaAfter, werr := brRewriteKey(root, cond, *newv)
		if werr != nil {
			fmt.Fprintf(w, "⛔ `%s` 改不动：%s ⇒ 不给结论（rc=2）\n", cond.name, *werr)
			return 2
		}
		entry := brNewObj("seq", seq, "ts", brNowISO(), "kind", "write", "name", cond.name, "gate", cond.gate,
			"file", cond.file, "old", *old, "new", *newv, "reason", brPyStrip(a.reason), "by", by,
			"sha256_before", brStrOrNil(shaBefore), "sha256_after", brStrOrNil(shaAfter), "measure", brStrDeref(r.measSrc))
		lerr := brAppendLedger(root, entry)
		if lerr != nil {
			fmt.Fprintf(w, "⛔ 台账追不上（件已改 · 台账没落）：%s ⇒ 判红（rc=1）\n", *lerr)
			return 1
		}
		doneCount++
		if *newv > *old {
			overCount++
		}
		fmt.Fprintf(w, "   ✓ `%s`：%s → %s · 件 `%s` · 台账第 %d 条（%s）\n",
			cond.name, brStrOfInt(old), brStrOfInt(newv), cond.file, seq, by)
		fmt.Fprintf(w, "     理由：%s\n", brPyStrip(a.reason))
		fmt.Fprintf(w, "     sha256：%s… → %s…\n", brHead(brDerefOrNil(shaBefore), 12), brHead(brDerefOrNil(shaAfter), 12))
	}
	if doneCount == 0 {
		fmt.Fprint(w, "⇒ 没有任何条件的数变了 ⇒ 没写（rc=0）\n")
		return 0
	}
	fmt.Fprintf(w, "⇒ 已写 %d 条（**唯一通道** · 每条都带「件 · 旧值 → 新值 · 理由 · 时间」）\n", doneCount)
	if overCount > 0 {
		fmt.Fprintf(w, "   ★ 其中**上调** %d 条（放宽判据）—— 台账里留了痕，事后可审\n", overCount)
	}
	return 0
}

func brDerefOrNil(p *string) string {
	if p == nil {
		return "None"
	}
	return *p
}

func brDoAnchor(w io.Writer, root string, a *brArgs, conds []brCond) int {
	if brPyStrip(a.reason) == "" {
		fmt.Fprint(w, "⛔ `--anchor` 必须带 `--reason '…'`（锚也要「谁在何时为什么」）⇒ 不写（rc=2）\n")
		return 2
	}
	rows, hard, soft := brCollect(root, conds, nil, false, a.only, false)
	if len(hard) > 0 {
		fmt.Fprint(w, "⛔ 台账有硬错 ⇒ 不给结论（rc=2）\n")
		for _, e := range hard {
			fmt.Fprintf(w, "   · %s\n", e)
		}
		return 2
	}
	for _, e := range soft {
		fmt.Fprintf(w, "   ⚠ 台账提示（不改判）：%s\n", e)
	}
	var missing []brRow
	for _, r := range rows {
		if r.verdict == "red" && strings.HasPrefix(r.verdictText, "无记录") {
			missing = append(missing, r)
		}
	}
	for _, r := range rows {
		if r.err != nil {
			fmt.Fprintf(w, "⛔ `%s` 现值取不到：%s ⇒ 不写（rc=2）\n", r.cond.name, *r.err)
			return 2
		}
	}
	by := a.by
	if by == "" {
		by = brWhoAmI(root)
	}
	entries, _ := brLoadLedger(root)
	seq := 1
	if len(entries) > 0 {
		mx := 0
		for _, e := range entries {
			if n, ok := brSeqInt(e["seq"]); ok && n > mx {
				mx = n
			}
		}
		seq = mx + 1
	}
	n := 0
	for _, r := range missing {
		cond := r.cond
		entry := brNewObj("seq", seq+n, "ts", brNowISO(), "kind", "anchor", "name", cond.name, "gate", cond.gate,
			"file", cond.file, "old", brIntOrNil(r.cur), "new", brIntOrNil(r.cur), "reason", brPyStrip(a.reason), "by", by,
			"sha256_before", brStrOrNil(r.sha), "sha256_after", brStrOrNil(r.sha), "measure", "（锚：只记现状，不改件）")
		lerr := brAppendLedger(root, entry)
		if lerr != nil {
			fmt.Fprintf(w, "⛔ 台账追不上：%s ⇒ 判红（rc=1）\n", *lerr)
			return 1
		}
		n++
		fmt.Fprintf(w, "   ✓ 锚 `%s` = %s（**件一字未改** · 台账第 %d 条 · %s）\n", cond.name, brStrOfInt(r.cur), seq+n-1, by)
	}
	if n == 0 {
		fmt.Fprint(w, "⇒ 没有「无记录」的条件 ⇒ 没写（rc=0）\n")
		return 0
	}
	fmt.Fprintf(w, "⇒ 已补 %d 条现状锚（append-only · 此后这个数再动就必须走 `--write` 并留痕）\n", n)
	return 0
}

func brDoConditions(w io.Writer) int {
	condsArr := []any{}
	for _, c := range brConditions {
		condsArr = append(condsArr, brNewObj("name", c.name, "gate", c.gate, "file", c.file, "key", c.key,
			"policy", c.policy, "unit", c.unit, "measure", strings.Join(c.measure, " "), "measure_re", c.measureRe))
	}
	payload := brNewObj("id", "baseline-conditions.v1", "ledger", brLedgerRel,
		"note", "声明式条件表：本件不记任何数字（现值读件、现测现跑该条件的门脚本）", "conditions", condsArr)
	brWriteJSON(w, payload)
	return 0
}

// ── 自检（合成件 · 不碰真目标）───────────────────────────────────────────────
var brFXConds = []brCond{
	{name: "fx.alpha", gate: "门①", file: brPyJoin("scripts", "gates", "fx-alpha.json"),
		key: "baseline", policy: "down", unit: "合成条件甲",
		measure:   []string{"python3", brPyJoin("scripts", "gates", "fx-measure.py"), "12", "命中 12 处"},
		measureRe: `命中\s*(\d+)\s*处`},
	{name: "fx.beta", gate: "门⑪", file: brPyJoin("scripts", "gates", "fx-beta.json"),
		key: "blanks", policy: "down", unit: "合成条件乙",
		measure:   []string{"python3", brPyJoin("scripts", "gates", "fx-measure.py"), "0", "空白 0 条"},
		measureRe: `空白\s*(\d+)\s*条`},
}

func brDeepEqual(a, b any) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	switch av := a.(type) {
	case []any:
		bv, ok := b.([]any)
		if !ok || len(av) != len(bv) {
			return false
		}
		for i := range av {
			if !brDeepEqual(av[i], bv[i]) {
				return false
			}
		}
		return true
	case map[string]int:
		bv, ok := b.(map[string]int)
		if !ok || len(av) != len(bv) {
			return false
		}
		for k, v := range av {
			if bv[k] != v {
				return false
			}
		}
		return true
	}
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

func brReprAny(v any) string {
	switch x := v.(type) {
	case []any:
		parts := make([]string, 0, len(x))
		for _, e := range x {
			parts = append(parts, brReprAny(e))
		}
		if len(parts) == 1 {
			return "(" + parts[0] + ",)"
		}
		return "(" + strings.Join(parts, ", ") + ")"
	case []string:
		parts := make([]string, 0, len(x))
		for _, e := range x {
			parts = append(parts, brPyRepr(e))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case map[string]int:
		parts := make([]string, 0, len(x))
		for k, val := range x {
			parts = append(parts, brPyRepr(k)+": "+strconv.Itoa(val))
		}
		return "{" + strings.Join(parts, ", ") + "}"
	default:
		return brPyRepr(v)
	}
}

func brSelfTest(w io.Writer) int {
	fails := []string{}
	tmp, err := os.MkdirTemp("", "zerg-baseline-selftest-")
	if err != nil {
		fmt.Fprintf(w, "自检结论: %s（失败 %d 条）\n", "**有失败**", 1)
		return 2
	}
	defer os.RemoveAll(tmp) // ★ A 类：自检夹具自回收（py 走 finally shutil.rmtree）

	caseFn := func(name string, got, want any) {
		if brDeepEqual(got, want) {
			fmt.Fprintf(w, "   ✓ %s\n", name)
		} else {
			fmt.Fprintf(w, "   ✗ %s（got=%s want=%s）\n", name, brReprAny(got), brReprAny(want))
			fails = append(fails, name)
		}
	}
	caseTrue := func(name string, ok bool) { caseFn(name, ok, true) }

	root := brPyJoin(tmp, "repo")
	_ = os.MkdirAll(brPyJoin(root, "core"), 0o755)
	_ = os.MkdirAll(brPyJoin(root, "scripts", "gates"), 0o755)
	_ = os.WriteFile(brPyJoin(root, "core", "go.mod"), []byte{}, 0o644)
	_ = os.WriteFile(brPyJoin(root, "scripts", "gates", "precommit-gates.sh"), []byte{}, 0o644)
	_ = os.WriteFile(brPyJoin(root, "scripts", "gates", "fx-measure.py"),
		[]byte("import sys\nprint(sys.argv[2] if len(sys.argv) > 2 else 'nothing')\n"), 0o644)
	fxAlpha := brPyJoin(root, brFXConds[0].file)
	fxBeta := brPyJoin(root, brFXConds[1].file)
	_ = os.WriteFile(fxAlpha, []byte("{\n  \"id\": \"fx-alpha.v1\",\n  \"baseline\": 5,\n  \"note\": \"合成件\"\n}\n"), 0o644)
	_ = os.WriteFile(fxBeta, []byte("{\n  \"id\": \"fx-beta.v1\",\n  \"blanks\": 0\n}\n"), 0o644)
	lp := brLedgerPath(root)

	// ① 仓根上溯
	if got, ok := brFindRoot(brPyJoin(root, "scripts", "gates")); ok {
		caseFn("① 仓根上溯（从子目录找得到）", got, root)
	} else {
		caseFn("① 仓根上溯（从子目录找得到）", nil, root)
	}

	// ② 现值读得对（逐键）
	v, sha, rerr := brReadValue(root, brFXConds[0])
	caseFn("② 现值读件（fx.alpha.baseline = 5）", []any{brIntOrNil(v), brStrOrNil(rerr)}, []any{5, nil})
	shaStr := ""
	if sha != nil {
		shaStr = *sha
	}
	caseTrue("② 件 sha256 读得到（64 位十六进制）", regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(shaStr))

	// ③ 现测现跑（**不是常量**）
	mv, msrc, merr := brMeasureValue(root, brFXConds[0], nil, false)
	caseFn("③ 现测 = 现跑门脚本再解析（12）", []any{brIntOrNil(mv), brStrOrNil(merr)}, []any{12, nil})
	caseTrue("③ 来源串点名了现跑的命令", strings.Contains(brStrDeref(msrc), "现跑"))

	// ④ 差值方向
	p := func(n int) *int { return &n }
	caseFn("④ 方向：现值 5 · 现测 12 ⇒ 将升", brDirection(p(5), p(12)), "将升")
	caseFn("④ 方向：现值 5 · 现测 3 ⇒ 将降", brDirection(p(5), p(3)), "将降")
	caseFn("④ 方向：现值 5 · 现测 5 ⇒ 不变", brDirection(p(5), p(5)), "不变")

	// ⑤ 无记录 ⇒ 红
	entries, _ := brLoadLedger(root)
	shaBeta, _ := brSha256File(fxBeta)
	vd, vt := brVerdictOf(brFXConds[1], p(0), &shaBeta, entries)
	caseFn("⑤ 无记录 ⇒ 判红（rc 那档）", vd, "red")
	caseTrue("⑤ 判词里点明「无记录」", strings.Contains(vt, "无记录"))

	// ⑥ 锚之后 ⇒ 绿
	rc := brDoAnchor(w, root, &brArgs{reason: "自检：起步现状锚", by: "selftest"}, brFXConds)
	caseFn("⑥ --anchor 缺条件也无误 ⇒ rc=0", rc, 0)
	entries, lerr := brLoadLedger(root)
	caseFn("⑥ 锚记了 2 条（两个条件各一条）", len(entries), 2)
	caseTrue("⑥ 台账件真落了盘", brIsFile(lp))
	h6, s6 := brLedgerShapeErrors(entries)
	caseFn("⑥ 无形状硬错（连号）", h6, []any{})
	caseFn("⑥ 无形状提示（值链/件链）", s6, []any{})
	vd, _ = brVerdictOf(brFXConds[0], p(5), &shaStr, entries)
	_ = lerr
	caseFn("⑥ 锚之后 ⇒ 绿", vd, "ok")

	// ⑦ 手改一个字节、没记录 ⇒ 判红
	raw, _ := os.ReadFile(fxAlpha)
	_ = os.WriteFile(fxAlpha, []byte(strings.Replace(brokenReplace(raw), "\"baseline\": 5", "\"baseline\": 9", 1)), 0o644)
	v2, sha2, _ := brReadValue(root, brFXConds[0])
	vd, vt = brVerdictOf(brFXConds[0], v2, sha2, entries)
	caseFn("⑦ 手改值（5→9）无记录 ⇒ 判红", vd, "red")
	caseTrue("⑦ 判词点明「没记录的手改」", strings.Contains(vt, "没记录的手改"))

	// ⑧ 件被删 ⇒ 判不了
	_ = os.Remove(fxAlpha)
	_, _, verr := brReadValue(root, brFXConds[0])
	caseTrue("⑧ 件不在 ⇒ 有错因（判不了 ≠ 绿）", verr != nil)
	_ = os.WriteFile(fxAlpha, []byte("{\n  \"baseline\": 5\n}\n"), 0o644)

	// ⑨ 写面：缺 --reason ⇒ rc=2 且件一字不动
	shBefore, _ := brSha256File(fxAlpha)
	rc = brDoWrite(w, root, &brArgs{reason: "   ", only: []string{"fx.alpha"}, by: "selftest"}, brFXConds)
	caseFn("⑨ --write 缺理由 ⇒ rc=2（fail-closed）", rc, 2)
	shNow, _ := brSha256File(fxAlpha)
	caseFn("⑨ 缺理由时件 sha256 一字不变", shNow, shBefore)

	// ⑩ 写面：上调不带 --allow-raise ⇒ rc=2 且不写
	rc = brDoWrite(w, root, &brArgs{reason: "自检：试图上调", only: []string{"fx.alpha"}, by: "selftest"}, brFXConds)
	caseFn("⑩ 上调不带 --allow-raise ⇒ rc=2", rc, 2)
	shNow, _ = brSha256File(fxAlpha)
	caseFn("⑩ 上调被拦时件 sha256 一字不变", shNow, shBefore)

	// ⑪ 显式允许上调 ⇒ 件真变 + 台账真追一条
	beforeN := len(mustLoad(root))
	rc = brDoWrite(w, root, &brArgs{reason: "自检：上调留痕", only: []string{"fx.alpha"}, allowRaise: true, by: "selftest"}, brFXConds)
	caseFn("⑪ 显式允许上调 ⇒ rc=0", rc, 0)
	v3, sha3, _ := brReadValue(root, brFXConds[0])
	caseFn("⑪ 件里的数真变了（5 → 12）", brIntOrNil(v3), 12)
	entries, _ = brLoadLedger(root)
	caseFn("⑪ 台账真追了一条（+1）", len(entries), beforeN+1)
	last := entries[len(entries)-1]
	_, hasReason := last["reason"]
	caseFn("⑪ 那条四格齐（件/旧/新/理由+时间）",
		[]any{last["file"], last["old"], last["new"], hasReason, last["ts"] != nil, last["by"], last["kind"]},
		[]any{brFXConds[0].file, 5, 12, true, true, "selftest", "write"})
	h11, s11 := brLedgerShapeErrors(entries)
	caseFn("⑪ 写后形状仍无硬错", h11, []any{})
	brokenChain := false
	for _, s := range s11 {
		if strings.Contains(s, "件链断了") {
			brokenChain = true
		}
	}
	caseTrue("⑪ 件链断被点成**提示**（件被别的批动过 ≠ 判红）", brokenChain)
	vd, _ = brVerdictOf(brFXConds[0], v3, sha3, entries)
	caseFn("⑪ 写后 ⇒ 绿（现值 = 台账最后一条）", vd, "ok")

	// ⑫ 只读性：--diff / --view 函数级跑法前后件 sha 不变
	shaPre, _ := brSha256File(fxAlpha)
	rows, hard, soft := brCollect(root, brFXConds, nil, false, []string{"fx.alpha"}, true)
	brPrintDiff(w, rows, hard, soft)
	brPrintView(w, root, rows, hard, soft)
	shaPost, _ := brSha256File(fxAlpha)
	caseFn("⑫ 只读两档跑完：件 sha256 一字不变", shaPost, shaPre)

	// ⑬ 台账断号 ⇒ 硬错
	lb, _ := os.ReadFile(lp)
	llines := brPySplitLines(brPyUniversalNewlines(string(lb)))
	_ = os.WriteFile(lp, []byte(strings.Join(append([]string{llines[0]}, llines[2:]...), "\n")+"\n"), 0o644)
	entries, _ = brLoadLedger(root)
	h13, _ := brLedgerShapeErrors(entries)
	seqBroken := false
	for _, e := range h13 {
		if strings.Contains(e, "连号") {
			seqBroken = true
		}
	}
	caseTrue("⑬ 台账被删一条 ⇒ seq 断号被点成**硬错**", seqBroken)
	caseFn("⑬ 硬错时 --write 不给结论 ⇒ 一个字节不写（rc=2）",
		brDoWrite(w, root, &brArgs{reason: "自检：台账坏了", only: []string{"fx.alpha"}, allowRaise: true, by: "selftest"}, brFXConds), 2)

	// ⑭ 读数注入
	rp := brPyJoin(tmp, "bad-readings.tsv")
	_ = os.WriteFile(rp, []byte("fx.alpha = 3\n没人认识的条件 = 1\n"), 0o644)
	_, rrerr := brReadReadings(rp, brFXConds)
	caseTrue("⑭ 读数件里有不认的条件名 ⇒ 判不了", rrerr != nil)
	rp2 := brPyJoin(tmp, "good-readings.tsv")
	_ = os.WriteFile(rp2, []byte("# 复核用读数\nfx.alpha = 3\nfx.beta = 0\n"), 0o644)
	gotM, gerr := brReadReadings(rp2, brFXConds)
	caseFn("⑭ 好读数件解得出两格", []any{gotM, brStrOrNil(gerr)},
		[]any{map[string]int{"fx.alpha": 3, "fx.beta": 0}, nil})

	// ⑮ 只改那一个数字
	_ = os.Remove(lp)
	_ = os.WriteFile(fxAlpha, []byte("{\n  \"id\": \"fx-alpha.v1\",\n  \"baseline\": 5,\n  \"note\": \"合成件\"\n}\n"), 0o644)
	rp3 := brPyJoin(tmp, "down.tsv")
	_ = os.WriteFile(rp3, []byte("fx.alpha = 3\n"), 0o644)
	rc = brDoWrite(w, root, &brArgs{reason: "自检：下调留痕", only: []string{"fx.alpha"}, readings: rp3, by: "selftest"}, brFXConds)
	caseFn("⑮ 现值 5 · 现测 3 ⇒ 下调 rc=0", rc, 0)
	after, _ := os.ReadFile(fxAlpha)
	caseFn("⑮ 只改了那一个数字", string(after), "{\n  \"id\": \"fx-alpha.v1\",\n  \"baseline\": 3,\n  \"note\": \"合成件\"\n}\n")

	// ⑯ 台账两态可判（Q-144）
	viewPayload := func() (int, map[string]any) {
		var sb strings.Builder
		rcc := brDoStatus(&sb, root, &brArgs{jsonOut: true}, brFXConds, "view")
		txt := strings.Split(sb.String(), "\n⇒")[0]
		pm, _ := brParseJSON(txt)
		mm, _ := pm.(map[string]any)
		return rcc, mm
	}
	_ = os.Remove(lp)
	rowsA, hardA, _ := brCollect(root, brFXConds, nil, false, nil, false)
	reasonA := brLedgerReason(root)
	rcA, payA := viewPayload()
	caseFn("⑯ 件不在盘上 ⇒ reason=ledger_absent", brStrOrNil(reasonA), "ledger_absent")
	caseFn("⑯ 件不在盘上 ⇒ 仍 rc=1（同码不动）", []any{brVerdictRC(rowsA, hardA), rcA}, []any{1, 1})
	caseFn("⑯ 包封 `--json` 档带同一枚判因", payA["reason"], "ledger_absent")
	caseTrue("⑯ 包封只多这一枚键（旧键一字不改）", brKeyDiffIsReasonOnly(payA))
	_ = os.WriteFile(lp, []byte{}, 0o644)
	rowsB, hardB, _ := brCollect(root, brFXConds, nil, false, nil, false)
	reasonB := brLedgerReason(root)
	rcB, payB := viewPayload()
	caseFn("⑯ 件在但 0 条记录 ⇒ reason=no_record", brStrOrNil(reasonB), "no_record")
	caseFn("⑯ 件在但 0 条记录 ⇒ 仍 rc=1（同码不动）", []any{brVerdictRC(rowsB, hardB), rcB}, []any{1, 1})
	caseTrue("⑯ 两态同 rc=1、判因却分得开（机器面从此能问是哪一种）",
		brStrOrNil(reasonA) != brStrOrNil(reasonB) && payA["reason"] != payB["reason"])

	fmt.Fprintf(w, "自检结论: %s（失败 %d 条）\n", map[bool]string{true: "全过", false: "**有失败**"}[len(fails) == 0], len(fails))
	if len(fails) == 0 {
		return 0
	}
	return 2
}

func brokenReplace(b []byte) string { return string(b) }

func mustLoad(root string) []map[string]any {
	e, _ := brLoadLedger(root)
	return e
}

func brKeyDiffIsReasonOnly(m map[string]any) bool {
	base := map[string]bool{"mode": true, "ledger": true, "rows": true, "ledger_hard_errors": true, "ledger_soft_notes": true}
	diff := []string{}
	for k := range m {
		if !base[k] {
			diff = append(diff, k)
		}
	}
	return len(diff) == 1 && diff[0] == "reason"
}

// ── argparse（子集：够本件用；帮助正文/用法退错按参考解释器逐字钉版）─────────────
type brArgs struct {
	view, diff, check, conditions, write, anchor bool
	allowRaise, jsonOut, selfTest                bool
	help                                         bool
	reason, by, readings, root                   string
	only                                         []string
}

var brHelpText = "usage: baseline-ratchet.py [-h] [--view] [--diff] [--check] [--conditions]\n                           [--write] [--anchor] [--reason REASON] [--by BY]\n                           [--allow-raise] [--only 条件名] [--readings 件]\n                           [--root ROOT] [--json] [--self-test]\n\noptional arguments:\n  -h, --help       show this help message and exit\n  --view           只读视图（默认档）：现值 + 台账 + 判\n  --diff           **只报差**：将升/将降几条 + 逐条件名（现跑测量）\n  --check          同 --view（无记录 ⇒ 判红 rc=1；本档是判据那档）\n  --conditions     打印条件表（机器可读 JSON）\n  --write          **唯一写入口**：改数 + 台账留痕（须 --reason）\n  --anchor         补「现状锚」：只记台账、**不改任何件**（须 --reason）\n  --reason REASON  改动理由（写/锚必填；没理由 = 不写 rc=2）\n  --by BY          谁（默认读本机 git 身份）\n  --allow-raise    显式允许**上调**（放宽判据）\n  --only 条件名       只对指定条件生效（可重复）\n  --readings 件     注入现测读数（`条件名=值` 逐行）—— 只读数、不跑门\n  --root ROOT      仓根（默认从当前目录上溯）\n  --json           机器可读输出\n  --self-test      合成件成对负控（不碰真目标）\n" //BR_HELP_PLACEHOLDER
var brErrUsageText = "usage: baseline-ratchet.py [-h] [--view] [--diff] [--check] [--conditions]\n                           [--write] [--anchor] [--reason REASON] [--by BY]\n                           [--allow-raise] [--only 条件名] [--readings 件]\n                           [--root ROOT] [--json] [--self-test]"                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  //BR_USAGE_PLACEHOLDER

type brOpt struct {
	name     string
	takesVal bool
}

var brLongOpts = []brOpt{
	{"view", false}, {"diff", false}, {"check", false}, {"conditions", false},
	{"write", false}, {"anchor", false}, {"reason", true}, {"by", true},
	{"allow-raise", false}, {"only", true}, {"readings", true}, {"root", true},
	{"json", false}, {"self-test", false}, {"help", false},
}

func brFindOpt(name string) *brOpt {
	for i := range brLongOpts {
		if brLongOpts[i].name == name {
			return &brLongOpts[i]
		}
	}
	return nil
}

// brMatchLong：argparse 前缀消歧（精确优先；唯一前缀可用；多义返回 ambiguous）。
func brMatchLong(name string) (string, bool) {
	if brFindOpt(name) != nil {
		return name, false
	}
	var hits []string
	for _, o := range brLongOpts {
		if strings.HasPrefix(o.name, name) {
			hits = append(hits, o.name)
		}
	}
	if len(hits) == 1 {
		return hits[0], false
	}
	if len(hits) > 1 {
		return "", true
	}
	return "", false
}

func brArgErr(w io.Writer, msg string) int {
	fmt.Fprint(w, brErrUsageText+"\n")
	fmt.Fprintf(w, "%s: error: %s\n", brProg, msg)
	return 2
}

func brSetFlag(a *brArgs, name string) {
	switch name {
	case "view":
		a.view = true
	case "diff":
		a.diff = true
	case "check":
		a.check = true
	case "conditions":
		a.conditions = true
	case "write":
		a.write = true
	case "anchor":
		a.anchor = true
	case "allow-raise":
		a.allowRaise = true
	case "json":
		a.jsonOut = true
	case "self-test":
		a.selfTest = true
	case "help":
		a.help = true
	}
}

func brSetVal(a *brArgs, name, val string) {
	switch name {
	case "reason":
		a.reason = val
	case "by":
		a.by = val
	case "readings":
		a.readings = val
	case "root":
		a.root = val
	case "only":
		a.only = append(a.only, val)
	}
}

func brParseArgs(stdout, stderr io.Writer, args []string) (*brArgs, int, bool) {
	a := &brArgs{root: "."}
	var positionals []string
	dashdash := false
	i := 0
	for i < len(args) {
		tok := args[i]
		if dashdash {
			positionals = append(positionals, tok)
			i++
			continue
		}
		if tok == "--" {
			dashdash = true
			i++
			continue
		}
		if strings.HasPrefix(tok, "--") {
			name := tok[2:]
			val := ""
			hasVal := false
			if idx := strings.Index(name, "="); idx >= 0 {
				val = name[idx+1:]
				name = name[:idx]
				hasVal = true
			}
			matched, ambiguous := brMatchLong(name)
			if ambiguous {
				i++
				return nil, brArgErr(stderr, "ambiguous option: --"+name+" could match "+strings.Join(brPrefixHits(name), ", ")), true
			}
			if matched == "" {
				positionals = append(positionals, tok)
				i++
				continue
			}
			opt := brFindOpt(matched)
			if opt.takesVal {
				if !hasVal {
					if i+1 >= len(args) {
						return nil, brArgErr(stderr, "argument --"+matched+": expected one argument"), true
					}
					i++
					val = args[i]
				}
				brSetVal(a, matched, val)
			} else {
				if hasVal {
					return nil, brArgErr(stderr, "argument --"+matched+": ignored explicit argument "+brPyRepr(val)), true
				}
				brSetFlag(a, matched)
			}
			i++
			continue
		}
		if strings.HasPrefix(tok, "-") && tok != "-" {
			if tok == "-h" {
				brSetFlag(a, "help")
			} else {
				positionals = append(positionals, tok)
			}
			i++
			continue
		}
		positionals = append(positionals, tok)
		i++
	}
	if a.help {
		fmt.Fprint(stdout, brHelpText)
		return nil, 0, true
	}
	if len(positionals) > 0 {
		return nil, brArgErr(stderr, "unrecognized arguments: "+strings.Join(positionals, " ")), true
	}
	return a, 0, false
}

func brPrefixHits(name string) []string {
	var hits []string
	for _, o := range brLongOpts {
		if strings.HasPrefix(o.name, name) {
			hits = append(hits, "--"+o.name)
		}
	}
	return hits
}

// ── main ─────────────────────────────────────────────────────────────────────
func baselineRatchetGateMain(stdout, stderr io.Writer, argv []string) int {
	a, code, done := brParseArgs(stdout, stderr, argv)
	if done {
		return code
	}
	if a.selfTest {
		fmt.Fprint(stdout, "── baseline-ratchet 自检（合成件 · 不碰真目标）──\n")
		return brSelfTest(stdout)
	}
	if a.conditions {
		return brDoConditions(stdout)
	}
	chosen := []string{}
	for _, n := range []string{"view", "diff", "check", "write", "anchor"} {
		on := false
		switch n {
		case "view":
			on = a.view
		case "diff":
			on = a.diff
		case "check":
			on = a.check
		case "write":
			on = a.write
		case "anchor":
			on = a.anchor
		}
		if on {
			chosen = append(chosen, "--"+n)
		}
	}
	if len(chosen) > 1 {
		fmt.Fprintf(stdout, "⛔ 档位互斥（一次只做一件事）：%s ⇒ 用法错（rc=2）\n", strings.Join(chosen, ","))
		return 2
	}
	if len(a.only) > 0 {
		for _, n := range a.only {
			if brCondByName(n, brConditions) == nil {
				fmt.Fprintf(stdout, "⛔ 条件名不认：%s（列全部：--conditions）⇒ 不给结论（rc=2）\n", n)
				return 2
			}
		}
	}
	root, ok := brFindRoot(a.root)
	if !ok {
		fmt.Fprintf(stdout, "⛔ 找不到仓根（从 %s 上溯找不到 %s）⇒ 不给结论（rc=2）\n", a.root, strings.Join(brRootMarkers, " + "))
		return 2
	}
	if a.write {
		return brDoWrite(stdout, root, a, brConditions)
	}
	if a.anchor {
		return brDoAnchor(stdout, root, a, brConditions)
	}
	if a.diff {
		return brDoStatus(stdout, root, a, brConditions, "diff")
	}
	return brDoStatus(stdout, root, a, brConditions, "view")
}
