// check_wallclock_ratchet.go —— 门禁件 check-wallclock-ratchet.py（232 行 · sha16 d1c7b27345a3b2eb）的
// **Go 真搬**（M2 · 墙钟棘轮族 · 四档：日常 / 核章 / 中 / 盖章 · 只许下降）。
//
// 为什么有这件：M2 真搬 —— 把 scripts/gates 下的独立判据件落成 core/cmd/zerg 的 Go 入口，与 py 原件
// **两态逐字对拍**（rc / stdout / stderr 三面逐字节 + 写面产物字节）。本件已挂
// `gateCheckGoEntry["check-wallclock-ratchet"]`（`zerg gate check` 走 inproc）：
//
//	· ✗ 不在 main.go 注册任何命令（✗ 不进命令树 · `add_step` **已挂** `precommit-gates.sh:2830`（门77 · tri 档））；
//	· ✗ 不删 py 原件（退场规则：对拍通过后才退场 · 逐件点名）；
//
// 入口签名（与 py 的 main() 对齐）：
//
//	func checkWallclockRatchetMain(stdout, stderr io.Writer, argv []string) int
//	  · argv  —— 与 py sys.argv[1:] 同形（不含程序名）；
//	  · 退码  —— 0 通过 / 1 门禁红（棘轮被放宽 / 事后对账不符）/ 2 不给结论；与 py 三码逐字对齐；
//	  · 输出  —— stdout / stderr 与 py 逐字节对齐（见下对拍面 / 未覆盖面）。
//
// ★ 唯一真源（只读；仓外夹具对拍时指向副本）：
//
//	· 基线件 <HERE>/wallclock-baseline.json（schema wallclock-ratchet/1）；
//	· 台账件 <HERE>/<basename(基线件 audit 字段)>（append-only；只 --write 时追一行）。
//
// ★★ 墙钟面（本件名带 wallclock ⇒ 核心即时刻面）：
//
//	· 现测值来源 = **显式注入旋钮**：--measured <秒>（调用方直接给）或 --from-bench <json>（现跑输出）；
//	  两样都不给 ⇒ rc=2 不给结论（不许拿记忆充数）；
//	· 档阈值对比（将升 / 将降 / 阈值边界）全部由 --measured 注入**造出**（✗ 不靠现读常量）；
//	· 写入通道里的「当前时刻」：py 取 datetime.now(timezone.utc)（**无注入旋钮**）⇒ 本件加 Go-only 隐藏旋钮
//	  --now / env ZERG_NOW 钉死（✗ 不靠掩码）。
//
// ★ Go-only 注入旋钮（与 py 不可合同 ⇒ 单列点名，只在 Go 侧传）：
//
//	· --now <ISO> / env ZERG_NOW       —— 钉死写入通道的「当前时刻」（py 无此旗标 / env）；
//	· env ZERG_WALLCLOCK_HERE <目录>   —— 钉死 HERE（py 由 __file__ 推导；Go 无此语境 ⇒ 由 repoRoot() 推 <仓根>/scripts/gates）。
//
// 未覆盖面（逐条）：
//
//	① argparse -h 正文：逐字复刻 /usr/bin/python3 3.9.6 的 argparse 输出（段名 optional arguments:），换版本不跟；
//	② 默认 HERE 定位：py = dirname(abspath(__file__))；Go 无 __file__ ⇒ 由 repoRoot() 推 <仓根>/scripts/gates（✗ 不再落 cwd）。残差：可执行文件既不在仓内、cwd 也不在仓内时 repoRoot() 取不到 ⇒ 回落 cwd；
//	③ py 未捕异常（基线件坏 JSON / 非 UTF-8 / 台账行非对象 / 写盘 OSError）⇒ traceback rc=1；本件一律 fail-closed rc=2，正文不同；
//	④ 浮点 py repr 与 Go 在指数 ∈ [16,20] 的写法差（本件值域为秒 ⇒ 不触发，仍列）；
//	⑤ JSON 数字非规范写法（1e3 / 24.7630 形态）在 str() 与 dump 往返时的形态差（夹具用规范写法 ⇒ 不触发）；
//	⑥ json 收 NaN / Infinity / -Infinity 的角落（py 收、Go 拒）；
//	⑦ Go-only 旋钮本身（--now / ZERG_NOW / ZERG_WALLCLOCK_HERE）；--n / --no 前缀缩写因新增 --now 与 py 不同（其余前缀无碰撞）。
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	cwrProg     = "check-wallclock-ratchet.py"
	cwrBaseName = "wallclock-baseline.json"
	cwrDefAudit = "wallclock-audit.jsonl"
	// cwrNowLayout = py strftime("%Y-%m-%dT%H:%M:%SZ")（UTC · 尾 Z 为字面）。
	cwrNowLayout = "2006-01-02T15:04:05Z"
)

var cwrTiers = []string{"daily", "seal", "medium", "stamp"}

// cwrUsageText —— argparse 出错时的 usage 段（逐字取自 /usr/bin/python3 3.9.6 · prog = 件基名）。
const cwrUsageText = "usage: check-wallclock-ratchet.py [-h] [--tier {daily,seal,medium,stamp}]\n" +
	"                                  [--measured MEASURED]\n" +
	"                                  [--from-bench FROM_BENCH] [--diff] [--check]\n" +
	"                                  [--write] [--reason REASON] [--allow-raise]\n" +
	"                                  [--json JSON]\n"

// cwrHelpText —— py -h/--help 的 stdout 逐字（Python 3.9 用 optional arguments:）。
const cwrHelpText = cwrUsageText + "\n" +
	"optional arguments:\n" +
	"  -h, --help            show this help message and exit\n" +
	"  --tier {daily,seal,medium,stamp}\n" +
	"  --measured MEASURED\n" +
	"  --from-bench FROM_BENCH\n" +
	"  --diff\n" +
	"  --check\n" +
	"  --write\n" +
	"  --reason REASON\n" +
	"  --allow-raise\n" +
	"  --json JSON\n"

// ── 保序 JSON 值（py json.load 保插入序；Go map 丢序 ⇒ 自建保序树）─────────────
//
// kind 取值：o 对象 / a 数组 / s 串 / n 数（保原字面）/ b 布尔 / z 空。
type cwrJ struct {
	kind  byte
	keys  []string
	vals  []*cwrJ
	elems []*cwrJ
	str   string
	num   string
	bl    bool
}

func cwrJStr(s string) *cwrJ { return &cwrJ{kind: 's', str: s} }
func cwrJNum(s string) *cwrJ { return &cwrJ{kind: 'n', num: s} }
func cwrJBool(b bool) *cwrJ  { return &cwrJ{kind: 'b', bl: b} }

func (o *cwrJ) get(k string) *cwrJ {
	if o == nil || o.kind != 'o' {
		return nil
	}
	for i, kk := range o.keys {
		if kk == k {
			return o.vals[i]
		}
	}
	return nil
}

func (o *cwrJ) put(k string, v *cwrJ) {
	o.keys = append(o.keys, k)
	o.vals = append(o.vals, v)
}

// set —— py dict 赋值语义：键在则**原位**替换（保序），不在则追加。
func (o *cwrJ) set(k string, v *cwrJ) {
	for i, kk := range o.keys {
		if kk == k {
			o.vals[i] = v
			return
		}
	}
	o.put(k, v)
}

// remove —— py dict.pop(k, None)：键在则删。
func (o *cwrJ) remove(k string) {
	for i, kk := range o.keys {
		if kk == k {
			o.keys = append(o.keys[:i], o.keys[i+1:]...)
			o.vals = append(o.vals[:i], o.vals[i+1:]...)
			return
		}
	}
}

func cwrParseJSON(raw string) (*cwrJ, error) {
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.UseNumber()
	v, err := cwrParseVal(dec)
	if err != nil {
		return nil, err
	}
	if _, err2 := dec.Token(); err2 != io.EOF {
		return nil, fmt.Errorf("trailing data")
	}
	return v, nil
}

func cwrParseVal(dec *json.Decoder) (*cwrJ, error) {
	t, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch tok := t.(type) {
	case json.Delim:
		switch tok {
		case '{':
			o := &cwrJ{kind: 'o'}
			for dec.More() {
				kt, err := dec.Token()
				if err != nil {
					return nil, err
				}
				ks, _ := kt.(string)
				v, err := cwrParseVal(dec)
				if err != nil {
					return nil, err
				}
				o.keys = append(o.keys, ks)
				o.vals = append(o.vals, v)
			}
			if _, err := dec.Token(); err != nil {
				return nil, err
			}
			return o, nil
		case '[':
			a := &cwrJ{kind: 'a'}
			for dec.More() {
				v, err := cwrParseVal(dec)
				if err != nil {
					return nil, err
				}
				a.elems = append(a.elems, v)
			}
			if _, err := dec.Token(); err != nil {
				return nil, err
			}
			return a, nil
		}
		return nil, fmt.Errorf("unexpected delimiter")
	case string:
		return &cwrJ{kind: 's', str: tok}, nil
	case json.Number:
		return &cwrJ{kind: 'n', num: tok.String()}, nil
	case bool:
		return &cwrJ{kind: 'b', bl: tok}, nil
	case nil:
		return &cwrJ{kind: 'z'}, nil
	}
	return nil, fmt.Errorf("unexpected token")
}

// ── 保序 JSON 编码（复刻 py json.dump / json.dumps）──────────────────────────

func cwrIndent(sb *strings.Builder, n int) {
	for i := 0; i < n; i++ {
		sb.WriteByte(' ')
	}
}

func cwrQuote(sb *strings.Builder, s string) {
	sb.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			sb.WriteString("\\\"")
		case '\\':
			sb.WriteString("\\\\")
		case '\n':
			sb.WriteString("\\n")
		case '\r':
			sb.WriteString("\\r")
		case '\t':
			sb.WriteString("\\t")
		case '\b':
			sb.WriteString("\\b")
		case '\f':
			sb.WriteString("\\f")
		default:
			if r < 0x20 {
				sb.WriteString(fmt.Sprintf("\\u%04x", r))
			} else {
				sb.WriteRune(r)
			}
		}
	}
	sb.WriteByte('"')
}

// cwrDump —— 复刻 py json.dump(obj, f, ensure_ascii=False, indent=1)（每级 1 空格）。
func cwrDump(sb *strings.Builder, v *cwrJ, indent int) {
	switch v.kind {
	case 'o':
		if len(v.keys) == 0 {
			sb.WriteString("{}")
			return
		}
		sb.WriteString("{\n")
		for i, k := range v.keys {
			cwrIndent(sb, indent+1)
			cwrQuote(sb, k)
			sb.WriteString(": ")
			cwrDump(sb, v.vals[i], indent+1)
			if i < len(v.keys)-1 {
				sb.WriteString(",")
			}
			sb.WriteByte('\n')
		}
		cwrIndent(sb, indent)
		sb.WriteString("}")
	case 'a':
		if len(v.elems) == 0 {
			sb.WriteString("[]")
			return
		}
		sb.WriteString("[\n")
		for i, e := range v.elems {
			cwrIndent(sb, indent+1)
			cwrDump(sb, e, indent+1)
			if i < len(v.elems)-1 {
				sb.WriteString(",")
			}
			sb.WriteByte('\n')
		}
		cwrIndent(sb, indent)
		sb.WriteString("]")
	case 's':
		cwrQuote(sb, v.str)
	case 'n':
		sb.WriteString(v.num)
	case 'b':
		if v.bl {
			sb.WriteString("true")
		} else {
			sb.WriteString("false")
		}
	case 'z':
		sb.WriteString("null")
	}
}

// cwrDumpCompact —— 复刻 py json.dumps(obj, ensure_ascii=False)（默认分隔符 `, ` / `: `）。
func cwrDumpCompact(sb *strings.Builder, v *cwrJ) {
	switch v.kind {
	case 'o':
		sb.WriteString("{")
		for i, k := range v.keys {
			if i > 0 {
				sb.WriteString(", ")
			}
			cwrQuote(sb, k)
			sb.WriteString(": ")
			cwrDumpCompact(sb, v.vals[i])
		}
		sb.WriteString("}")
	case 'a':
		sb.WriteString("[")
		for i, e := range v.elems {
			if i > 0 {
				sb.WriteString(", ")
			}
			cwrDumpCompact(sb, e)
		}
		sb.WriteString("]")
	case 's':
		cwrQuote(sb, v.str)
	case 'n':
		sb.WriteString(v.num)
	case 'b':
		if v.bl {
			sb.WriteString("true")
		} else {
			sb.WriteString("false")
		}
	case 'z':
		sb.WriteString("null")
	}
}

// ── py 值形态原语（str / repr / bool / float）─────────────────────────────────

func cwrNumIsFloat(num string) bool { return strings.ContainsAny(num, ".eE") }

// cwrPyFloatRepr —— py float.__repr__ 的常见形态（最短往返；整值补 .0）。指数 ∈ [16,20] 一档见件头未覆盖面。
func cwrPyFloatRepr(f float64) string {
	if math.IsNaN(f) {
		return "nan"
	}
	if math.IsInf(f, 1) {
		return "inf"
	}
	if math.IsInf(f, -1) {
		return "-inf"
	}
	s := strconv.FormatFloat(f, 'g', -1, 64)
	if !strings.ContainsAny(s, ".eE") {
		s += ".0"
	}
	return s
}

// cwrPyStr —— py str(值)：数按「含 . 或 e ⇒ 浮点」，其余整字面。
func cwrPyStr(v *cwrJ) string {
	if v == nil {
		return "None"
	}
	switch v.kind {
	case 's':
		return v.str
	case 'n':
		if cwrNumIsFloat(v.num) {
			f, _ := strconv.ParseFloat(v.num, 64)
			return cwrPyFloatRepr(f)
		}
		return v.num
	case 'b':
		if v.bl {
			return "True"
		}
		return "False"
	case 'a':
		var parts []string
		for _, e := range v.elems {
			parts = append(parts, cwrPyRepr(e))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case 'o':
		var parts []string
		for i, k := range v.keys {
			parts = append(parts, cwrPyStrQuote(k)+": "+cwrPyRepr(v.vals[i]))
		}
		return "{" + strings.Join(parts, ", ") + "}"
	}
	return "None"
}

// cwrPyStrQuote —— py repr(str) 的常用形态（单引号 · 转义反斜杠与单引号）。
func cwrPyStrQuote(s string) string {
	var b strings.Builder
	b.WriteByte('\'')
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString("\\\\")
		case '\'':
			b.WriteString("\\'")
		case '\n':
			b.WriteString("\\n")
		case '\r':
			b.WriteString("\\r")
		case '\t':
			b.WriteString("\\t")
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('\'')
	return b.String()
}

// cwrPyRepr —— py repr(值)（供 die 文案里的 !r 用）。
func cwrPyRepr(v *cwrJ) string {
	if v == nil {
		return "None"
	}
	switch v.kind {
	case 's':
		return cwrPyStrQuote(v.str)
	case 'n':
		return v.num
	case 'b':
		if v.bl {
			return "True"
		}
		return "False"
	case 'z':
		return "None"
	case 'a':
		var parts []string
		for _, e := range v.elems {
			parts = append(parts, cwrPyRepr(e))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case 'o':
		var parts []string
		for i, k := range v.keys {
			parts = append(parts, cwrPyStrQuote(k)+": "+cwrPyRepr(v.vals[i]))
		}
		return "{" + strings.Join(parts, ", ") + "}"
	}
	return "None"
}

// cwrTruthy —— py 真假值语义。
func cwrTruthy(v *cwrJ) bool {
	if v == nil {
		return false
	}
	switch v.kind {
	case 'z':
		return false
	case 'b':
		return v.bl
	case 'n':
		f, err := strconv.ParseFloat(v.num, 64)
		if err != nil {
			return true
		}
		return f != 0
	case 's':
		return v.str != ""
	case 'a':
		return len(v.elems) > 0
	case 'o':
		return len(v.keys) > 0
	}
	return false
}

func cwrBoolStr(b bool) string {
	if b {
		return "True"
	}
	return "False"
}

// cwrFloat —— py float(值) 的常用形态。
func cwrFloat(v *cwrJ) float64 {
	if v == nil {
		return 0
	}
	switch v.kind {
	case 'n':
		f, _ := strconv.ParseFloat(v.num, 64)
		return f
	case 'b':
		if v.bl {
			return 1
		}
		return 0
	case 's':
		f, _ := strconv.ParseFloat(strings.TrimSpace(v.str), 64)
		return f
	}
	return 0
}

// cwrRound3 —— py round(x, 3) 的常用形态（正确舍入到 3 位小数）。
func cwrRound3(f float64) float64 {
	s := strconv.FormatFloat(f, 'f', 3, 64)
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return f
	}
	return v
}

// cwrSign2 —— py f"{x:+.2f}"（带符号两位小数）。
func cwrSign2(f float64) string { return fmt.Sprintf("%+.2f", f) }

// ── 路径原语（posixpath 口径；✗ 不用 filepath 的 Clean/Sep 语义）───────────────

func cwrJoin(a, b string) string {
	if strings.HasPrefix(b, "/") {
		return b
	}
	if a == "" || strings.HasSuffix(a, "/") {
		return a + b
	}
	return a + "/" + b
}

func cwrPyBasename(p string) string {
	p = strings.TrimRight(p, "/")
	if p == "" {
		return ""
	}
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return p
}

func cwrIsFile(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

// cwrOSErrText —— py OSError 的 str 形态（errno + strerror + 引号文件名）；未列的 errno 退 Go 文案。
func cwrOSErrText(err error, path string) string {
	if os.IsNotExist(err) {
		return "[Errno 2] No such file or directory: " + cwrPyStrQuote(path)
	}
	if os.IsPermission(err) {
		return "[Errno 13] Permission denied: " + cwrPyStrQuote(path)
	}
	return err.Error()
}

// cwrHere —— 资源根。py = dirname(abspath(__file__)) = `<仓根>/scripts/gates`。
// Go 无 __file__ ⇒ 由 `repoRoot()`（ZERG_REPO → 可执行文件目录上溯 → cwd 上溯，锚 = core/internal/version/version.go）
// 推 `<仓根>/scripts/gates` —— ✗ 不依赖 cwd（inproc 档 cwd = 仓根 ⇒ 旧口径会落仓根、读不到 scripts/gates/ 下的基线件）。
// env ZERG_WALLCLOCK_HERE 仍在**最前**（可选覆盖，✗ 非必需项）。
func cwrHere() string {
	if v := os.Getenv("ZERG_WALLCLOCK_HERE"); v != "" {
		return v
	}
	if r := repoRoot(); r != "" {
		return cwrJoin(r, "scripts/gates")
	}
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}

func cwrNowISO(knob string) string {
	s := knob
	if s == "" {
		s = os.Getenv("ZERG_NOW")
	}
	if s != "" {
		return s
	}
	return time.Now().UTC().Format(cwrNowLayout)
}

// ── 基线件 / 台账件读取 ───────────────────────────────────────────────────────

func cwrDie(stderr io.Writer, msg string) {
	fmt.Fprintf(stderr, "%s\n", msg)
}

func cwrLoad(base string, stderr io.Writer) (*cwrJ, bool) {
	if !cwrIsFile(base) {
		cwrDie(stderr, "✗ 缺基线件 "+base+" ⇒ 不给结论(rc=2)")
		return nil, false
	}
	b, err := os.ReadFile(base)
	if err != nil {
		cwrDie(stderr, "✗ 缺基线件 "+base+" ⇒ 不给结论(rc=2)")
		return nil, false
	}
	v, err := cwrParseJSON(string(b))
	if err != nil {
		cwrDie(stderr, "✗ 基线件读不出 JSON："+base+" ⇒ 不给结论(rc=2)")
		return nil, false
	}
	return v, true
}

func cwrAuditField(cfg *cwrJ) string {
	v := cfg.get("audit")
	if v == nil || v.kind != 's' {
		return cwrDefAudit
	}
	return v.str
}

func cwrSplitLines(s string) []string { return strings.Split(s, "\n") }

// cwrLastAuditForTier —— 台账里**该档**的最后一条（按档筛选，✗ 不取整份台账最后一行）。
func cwrLastAuditForTier(path, tier string) *cwrJ {
	if !cwrIsFile(path) {
		return nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var last *cwrJ
	for _, ln := range cwrSplitLines(string(b)) {
		s := strings.TrimSpace(ln)
		if s == "" {
			continue
		}
		rec, err := cwrParseJSON(s)
		if err != nil {
			continue
		}
		if rec.kind != 'o' {
			continue
		}
		tv := rec.get("tier")
		if tv != nil && tv.kind == 's' && tv.str == tier {
			last = rec
		}
	}
	return last
}

// ── 现测值（--measured / --from-bench）───────────────────────────────────────

func cwrNoneOrEmptyStr(v *cwrJ) bool {
	if v == nil || v.kind == 'z' {
		return true
	}
	return v.kind == 's' && v.str == ""
}

// cwrToFloat —— py float(JSON 值) 的常用形态；ok=false 表 py 会抛（TypeError/ValueError）。
func cwrToFloat(v *cwrJ) (float64, bool) {
	if v == nil {
		return 0, false
	}
	switch v.kind {
	case 'n':
		f, err := strconv.ParseFloat(v.num, 64)
		return f, err == nil
	case 'b':
		if v.bl {
			return 1, true
		}
		return 0, true
	case 's':
		f, err := strconv.ParseFloat(strings.TrimSpace(v.str), 64)
		return f, err == nil
	}
	return 0, false
}

func cwrMeasuredSecs(a *cwrArgs, stderr io.Writer) (float64, bool) {
	if a.hasMeasured {
		return a.measured, true
	}
	if a.fromBench != "" {
		rawb, err := os.ReadFile(a.fromBench)
		if err != nil {
			cwrDie(stderr, "✗ 读不了 --from-bench "+a.fromBench+": "+cwrOSErrText(err, a.fromBench)+" ⇒ 不给结论(2)")
			return 0, false
		}
		raw := string(rawb)
		data, perr := cwrParseJSON(raw)
		if perr != nil {
			f, ferr := strconv.ParseFloat(strings.TrimSpace(raw), 64)
			if ferr != nil {
				cwrDie(stderr, "✗ 读不了 --from-bench "+a.fromBench+": could not convert string to float: "+
					cwrPyStrQuote(strings.TrimSpace(raw))+" ⇒ 不给结论(2)")
				return 0, false
			}
			return f, true
		}
		if data.kind == 'o' {
			items := data.get("items")
			if items != nil && items.kind == 'a' && len(items.elems) > 0 {
				var ms []float64
				for _, it := range items.elems {
					if it.kind != 'o' {
						continue
					}
					rv := it.get("real_ms")
					if cwrNoneOrEmptyStr(rv) {
						continue
					}
					f, ok := cwrToFloat(rv)
					if !ok {
						cwrDie(stderr, "✗ --from-bench 的 items[].real_ms 读不出数："+cwrPyRepr(rv)+" ⇒ 不给结论(2)")
						return 0, false
					}
					ms = append(ms, f)
				}
				if len(ms) == 0 {
					cwrDie(stderr, "✗ --from-bench 的 items 里一条可用 real_ms 都没有 ⇒ 不给结论(2)")
					return 0, false
				}
				best := ms[0]
				for _, m := range ms[1:] {
					if m > best {
						best = m
					}
				}
				return best / 1000.0, true
			}
			rv := data.get("real_ms")
			if !cwrNoneOrEmptyStr(rv) {
				f, ok := cwrToFloat(rv)
				if !ok {
					cwrDie(stderr, "✗ --from-bench 的 real_ms 读不出数："+cwrPyRepr(rv)+" ⇒ 不给结论(2)")
					return 0, false
				}
				return f / 1000.0, true
			}
			for _, k := range []string{"secs", "seconds", "wall"} {
				kv := data.get(k)
				if kv == nil {
					continue
				}
				f, ok := cwrToFloat(kv)
				if !ok {
					cwrDie(stderr, "✗ 读不了 --from-bench "+a.fromBench+": float() argument must be a string or a real number ⇒ 不给结论(2)")
					return 0, false
				}
				return f, true
			}
			st := data.get("steps")
			if st != nil {
				if st.kind != 'a' {
					cwrDie(stderr, "✗ 读不了 --from-bench "+a.fromBench+": object is not iterable ⇒ 不给结论(2)")
					return 0, false
				}
				sum := 0.0
				for _, s := range st.elems {
					sv := s.get("secs")
					if sv == nil {
						continue
					}
					f, ok := cwrToFloat(sv)
					if !ok {
						cwrDie(stderr, "✗ 读不了 --from-bench "+a.fromBench+": float() argument must be a string or a real number ⇒ 不给结论(2)")
						return 0, false
					}
					sum += f
				}
				return sum, true
			}
			cwrDie(stderr, "✗ --from-bench 的 json 里找不到 secs/seconds/wall/steps ⇒ 不给结论(2)")
			return 0, false
		}
		f, ok := cwrToFloat(data)
		if !ok {
			cwrDie(stderr, "✗ 读不了 --from-bench "+a.fromBench+": 形态读不出数 ⇒ 不给结论(2)")
			return 0, false
		}
		return f, true
	}
	cwrDie(stderr, "✗ 既没给 --measured 也没给 --from-bench ⇒ 不给结论(2)（不许拿记忆充数）")
	return 0, false
}

// ── 参数面（复刻 argparse 子集）─────────────────────────────────────────────

type cwrOpt struct {
	name     string
	takesVal bool
}

// cwrOpts —— 长选项表。末项 now 为 Go-only 隐藏旋钮（py 无）。
var cwrOpts = []cwrOpt{
	{"help", false},
	{"tier", true},
	{"measured", true},
	{"from-bench", true},
	{"diff", false},
	{"check", false},
	{"write", false},
	{"reason", true},
	{"allow-raise", false},
	{"json", true},
	{"now", true},
}

type cwrArgs struct {
	help        bool
	diff        bool
	check       bool
	write       bool
	allowRaise  bool
	hasMeasured bool
	measured    float64
	tier        string
	fromBench   string
	reason      string
	jsonOut     string
	now         string
}

func cwrOptByName(name string) cwrOpt {
	for _, o := range cwrOpts {
		if o.name == name {
			return o
		}
	}
	return cwrOpt{}
}

func cwrMatchLong(name string) (string, []string) {
	var hits []string
	for _, o := range cwrOpts {
		if strings.HasPrefix(o.name, name) {
			hits = append(hits, o.name)
		}
	}
	for _, h := range hits {
		if h == name {
			return name, hits
		}
	}
	if len(hits) == 1 {
		return hits[0], hits
	}
	return "", hits
}

func cwrArgErr(stderr io.Writer, msg string) int {
	fmt.Fprint(stderr, cwrUsageText)
	fmt.Fprintf(stderr, "%s: error: %s\n", cwrProg, msg)
	return 2
}

func cwrInTiers(v string) bool {
	for _, t := range cwrTiers {
		if t == v {
			return true
		}
	}
	return false
}

func cwrLooksOpt(s string) bool {
	if !strings.HasPrefix(s, "-") || s == "-" {
		return false
	}
	if _, err := strconv.ParseFloat(s, 64); err == nil {
		return false
	}
	return true
}

func cwrSetVal(a *cwrArgs, stderr io.Writer, name, val string) (int, bool) {
	switch name {
	case "tier":
		if !cwrInTiers(val) {
			return cwrArgErr(stderr, "argument --tier: invalid choice: "+cwrPyStrQuote(val)+
				" (choose from 'daily', 'seal', 'medium', 'stamp')"), true
		}
		a.tier = val
	case "measured":
		f, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return cwrArgErr(stderr, "argument --measured: invalid float value: "+cwrPyStrQuote(val)), true
		}
		a.measured = f
		a.hasMeasured = true
	case "from-bench":
		a.fromBench = val
	case "reason":
		a.reason = val
	case "json":
		a.jsonOut = val
	case "now":
		a.now = val
	}
	return 0, false
}

func cwrSetFlag(a *cwrArgs, name string) {
	switch name {
	case "diff":
		a.diff = true
	case "check":
		a.check = true
	case "write":
		a.write = true
	case "allow-raise":
		a.allowRaise = true
	case "help":
		a.help = true
	}
}

func cwrParseArgs(stdout, stderr io.Writer, argv []string) (*cwrArgs, int, bool) {
	a := &cwrArgs{tier: "daily"}
	var extras []string
	onlyPositional := false
	i := 0
	for i < len(argv) {
		tok := argv[i]
		if onlyPositional {
			extras = append(extras, tok)
			i++
			continue
		}
		if tok == "--" {
			onlyPositional = true
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
			matched, cands := cwrMatchLong(name)
			if matched == "" {
				if len(cands) > 1 {
					prefixed := make([]string, len(cands))
					for j, c := range cands {
						prefixed[j] = "--" + c
					}
					return nil, cwrArgErr(stderr, "ambiguous option: --"+name+" could match "+
						strings.Join(prefixed, ", ")), true
				}
				extras = append(extras, tok)
				i++
				continue
			}
			opt := cwrOptByName(matched)
			if opt.takesVal {
				if !hasVal {
					if i+1 >= len(argv) || cwrLooksOpt(argv[i+1]) {
						return nil, cwrArgErr(stderr, "argument --"+matched+": expected one argument"), true
					}
					i++
					val = argv[i]
				}
				if code, done := cwrSetVal(a, stderr, matched, val); done {
					return nil, code, true
				}
			} else {
				if hasVal {
					return nil, cwrArgErr(stderr, "argument --"+matched+": ignored explicit argument "+
						cwrPyStrQuote(val)), true
				}
				if matched == "help" {
					fmt.Fprint(stdout, cwrHelpText)
					return nil, 0, true
				}
				cwrSetFlag(a, matched)
			}
			i++
			continue
		}
		if strings.HasPrefix(tok, "-") && tok != "-" {
			if tok == "-h" {
				fmt.Fprint(stdout, cwrHelpText)
				return nil, 0, true
			}
			extras = append(extras, tok)
			i++
			continue
		}
		extras = append(extras, tok)
		i++
	}
	if len(extras) > 0 {
		return nil, cwrArgErr(stderr, "unrecognized arguments: "+strings.Join(extras, " ")), true
	}
	return a, 0, false
}

// ── 入口 ─────────────────────────────────────────────────────────────────────

func checkWallclockRatchetMain(stdout, stderr io.Writer, argv []string) int {
	a, code, done := cwrParseArgs(stdout, stderr, argv)
	if done {
		return code
	}

	here := cwrHere()
	base := cwrJoin(here, cwrBaseName)

	cfg, ok := cwrLoad(base, stderr)
	if !ok {
		return 2
	}

	tiers := cfg.get("tiers")
	var t *cwrJ
	if tiers != nil && tiers.kind == 'o' {
		t = tiers.get(a.tier)
	}
	if t == nil {
		fmt.Fprintf(stderr, "✗ 基线件里没有 `%s` 档 ⇒ 不给结论(2)\n", a.tier)
		return 2
	}
	cur := t.get("secs")
	unmeasured := cwrTruthy(t.get("_unmeasured")) || cur == nil
	if unmeasured {
		if cwrTruthy(t.get("_not_a_wallclock")) {
			fmt.Fprintf(stderr, "✗ `%s` 档**不计墙钟**（`_not_a_wallclock=true`：本档不真跑任何步，"+
				"无「跑门耗时」这一项） ⇒ 不给结论(rc=2)：核章档的判据是章链覆盖与新鲜度，"+
				"走 `scripts/gates/seal-ledger.py --verify`\n", a.tier)
			return 2
		}
		fmt.Fprintf(stderr, "✗ `%s` 档**未标定**（secs=%s · _unmeasured=%s） ⇒ 不给结论(rc=2)："+
			"先在有门跑窗口实测并 `--write` 标定，不许静默当绿\n",
			a.tier, cwrPyStr(cur), cwrBoolStr(cwrTruthy(t.get("_unmeasured"))))
		return 2
	}

	audit := cwrJoin(here, cwrPyBasename(cwrAuditField(cfg)))

	if a.check {
		lastT := cwrLastAuditForTier(audit, a.tier)
		if lastT == nil {
			fmt.Fprintf(stderr, "✗ --check：台账 %s 里**没有 `%s` 档**的记录，而基线件有值 ⇒ 判红(1)\n",
				audit, a.tier)
			return 1
		}
		if cwrFloat(lastT.get("new")) != cwrFloat(cur) {
			fmt.Fprintf(stderr, "✗ --check：现值 %s ≠ 台账里 `%s` 档最新一条 new %s ⇒ **有人手改** ⇒ 判红(1)\n",
				cwrPyStr(cur), a.tier, cwrPyStr(lastT.get("new")))
			return 1
		}
		fmt.Fprintf(stdout, "✓ --check 过：%s 现值 %s == 台账里 `%s` 档最新一条 new\n",
			a.tier, cwrPyStr(cur), a.tier)
		return 0
	}

	ms, ok2 := cwrMeasuredSecs(a, stderr)
	if !ok2 {
		return 2
	}
	fmt.Fprintf(stdout, "[%s] 现值 %ss · 现测 %ss\n", a.tier, cwrPyStr(cur), cwrPyFloatRepr(ms))

	if a.diff {
		d := ms - cwrFloat(cur)
		cond := "将降/持平（棘轮允许）"
		if d > 0 {
			cond = "将升（放宽判据，需 --allow-raise）"
		}
		fmt.Fprintf(stdout, "差 %ss —— %s\n", cwrSign2(d), cond)
		if a.jsonOut != "" {
			o := &cwrJ{kind: 'o'}
			o.put("tier", cwrJStr(a.tier))
			o.put("cur", cur)
			o.put("measured", cwrJNum(cwrPyFloatRepr(ms)))
			o.put("delta", cwrJNum(cwrPyFloatRepr(d)))
			var sb strings.Builder
			cwrDumpCompact(&sb, o)
			fmt.Fprintf(stdout, "%s\n", sb.String())
		}
		return 0
	}

	curF := cwrFloat(cur)
	raising := ms > curF
	if raising && !a.write {
		fmt.Fprintf(stderr, "✗ 现测 %ss > 现值 %ss ⇒ **棘轮只许下降**（rc=1）\n",
			cwrPyFloatRepr(ms), cwrPyStr(cur))
		return 1
	}
	if raising && !a.allowRaise {
		fmt.Fprint(stderr, "✗ 上调 = 放宽判据：给了 --write 还要 --allow-raise 才写（rc=2 · fail-closed，不写）\n")
		return 2
	}
	if raising && a.reason == "" {
		fmt.Fprint(stderr, "✗ 上调必须带 --reason（留痕）⇒ 不给结论(rc=2)\n")
		return 2
	}
	if !a.write {
		if ms <= curF {
			fmt.Fprintf(stdout, "✓ 现测 %ss ≤ 现值 %ss（棘轮 OK）\n", cwrPyFloatRepr(ms), cwrPyStr(cur))
		}
		return 0
	}

	// ── 写通道：改数 + append-only 台账 ─────────────────────────────────────
	if a.reason == "" && !raising {
		fmt.Fprint(stderr, "✗ --write 必须带 --reason（唯一写通道要留痕）⇒ 不给结论(rc=2)\n")
		return 2
	}
	newv := cwrRound3(ms)
	t.set("secs", cwrJNum(cwrPyFloatRepr(newv)))
	t.remove("_unmeasured")
	now := cwrNowISO(a.now)
	t.set("updated", cwrJStr(now))

	var sb strings.Builder
	cwrDump(&sb, cfg, 0)
	sb.WriteByte('\n')
	if err := os.WriteFile(base, []byte(sb.String()), 0o644); err != nil {
		cwrDie(stderr, "✗ 写不了基线件 "+base+": "+cwrOSErrText(err, base)+" ⇒ 不给结论(2)")
		return 2
	}

	rec := &cwrJ{kind: 'o'}
	rec.put("t", cwrJStr(now))
	rec.put("tier", cwrJStr(a.tier))
	rec.put("old", cur)
	rec.put("new", t.get("secs"))
	rec.put("raise", cwrJBool(raising))
	rec.put("reason", cwrJStr(a.reason))
	user := os.Getenv("USER")
	if user == "" {
		user = "?"
	}
	rec.put("by", cwrJStr(user))
	var rb strings.Builder
	cwrDumpCompact(&rb, rec)
	rb.WriteByte('\n')
	f, err := os.OpenFile(audit, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		cwrDie(stderr, "✗ 写不了台账 "+audit+": "+cwrOSErrText(err, audit)+" ⇒ 不给结论(2)")
		return 2
	}
	if _, err := f.WriteString(rb.String()); err != nil {
		_ = f.Close()
		cwrDie(stderr, "✗ 写不了台账 "+audit+": "+cwrOSErrText(err, audit)+" ⇒ 不给结论(2)")
		return 2
	}
	_ = f.Close()

	fmt.Fprintf(stdout, "✓ 已写 %s: %s → %s · 台账 +1 行（%s）\n",
		a.tier, cwrPyStr(cur), cwrPyStr(t.get("secs")), audit)
	return 0
}
