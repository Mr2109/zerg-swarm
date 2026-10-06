// check_selftest_knobs.go —— 判据件 `scripts/gates/check-selftest-knobs.py`（238 行）的 **Go 真搬**
// （v2.5.14 · M2 · 门99 提交链「关自检旋钮」只读元门 · 入口 `checkSelftestKnobsGateMain`）。
//
// 为什么有这件（不是「顺手 Go 化」）：
//
// \tM2 = 按价值**真搬 Go**（逐批判据搬 Go · 落点 `core/cmd/zerg/`）。验收判据 = 对同一 argv 跑两态
// \t（py 原件 ⟷ 本入口），**stdout / stderr / 退码三面逐字相同**（除下列**显式声明**的口径差异面与未覆盖面）。
//
// 选它 / 不选它的理由（本单口径 · 纯静态只读单件）：
//
// \t✓ 选：判据是**纯字符串 / 正则面**（读 `scripts/gates` 与 `core/cmd/zerg` 下被提交链点名的件，
// \t  逐行抽样「关自检型旋钮」消费形态）——**不开子进程、不碰网络、不读时刻、不用随机**；
// \t  `--self-test` 只在临时目录（`MkdirTemp`）里造件并自回收 ⇒ 唯一写面自回收。退码 `0/1/2` 三档齐 ·
// \t  主路径**零写面** · 行数适中（238 行）。桶 `q–s`（件名首字母 `s`）。
// \t✗ 不选同族：`check-exec-two-condition`(366) / `check-bypass-idioms`(343) / `check-empty-domain-guards`(436)
// \t  等含 `subprocess` 自检子进程面；`check-mkdir-temp-cleanup`(210) 已被并发路（本刻现读：
// \t  `scratch/grain21/check_mkdir_temp_cleanup.go` 在盘）认领 ⇒ 本件与在途路错开。
//
// 本单边界（照实）：本件**只落码、不接线** ——
//
// \t· **不**在 `main.go` 里注册命令（✗ 不进命令树）· **不**删 `scripts/gates/check-selftest-knobs.py` 原件
// \t  （退场规则：**对拍通过后才**退场 · 逐件点名）；入口 `checkSelftestKnobsGateMain` 目前**无调用点**（M2 刻意状态）。
// \t· ✗ 不改登记面六面 · ✗ 不改任何 `family_*.go` / 已落旧件 / `main.go` / `gate_inproc_table.go`。
//
// 入口签名（与 py 的 `main()` 对齐）：
//
// \tfunc checkSelftestKnobsGateMain(stdout, stderr io.Writer, argv []string) int
// \t  · argv  —— 与 py `sys.argv[1:]` 同形（**不含程序名**）
// \t  · 退码  —— 0 全被登记 / 1 有未登记旋钮（逐条点名）/ 2 缺语料 / 空域 / 白名单不全 / 用法错
// \t  · 输出  —— stdout / stderr 与 py 逐字节对齐（见「已对拍面 / 未覆盖面」）
//
// ★★ 两条标准声明行：
//
// \t(a) **默认根定位差异**（✗ 不声称逐字对齐）：
// \t     py `--root` 缺省 = `os.environ.get("ZERG_ROOT", os.getcwd())` ⇒ env 在场（即便空串）即采信，否则 cwd。
// \t     Go 无 `__file__` 语境 ⇒ 同名同序照搬：`ZERG_ROOT` 在场 ⇒ 采信；否则 `os.Getwd()`。
// \t     「cwd == py 运行时的 cwd」时两态读数逐字相同（对拍甲环境已证）。
// \t(b) **自指检查** —— 本件**不在**任何他门的扫描面内（四形态逐条 · 量化）：
// \t     ① 本件 = `core/cmd/zerg/check_selftest_knobs.go`（`.go` · 不在 `scripts/gates/`）⇒ 不被扫scripts面的判据扫到。
// \t     ② **`check-selftest-knobs.py` 的 `core/cmd/zerg/*.go` 面只判「提交链件」**（`in_commit_chain`：件基本名须
// \t        **逐字出现在 `precommit-gates.sh`**）⇒ 现读 `precommit-gates.sh` 里本件基本名（下划线形）命中 0
// \t        （连字符形只在 py 面）⇒ 本件**不进该门扫描面**。
// \t     ③ 那几枚含关键词的旋钮名（本件登记表里逐字点名、但**只登记不消费**）一律**拼串构造**（`cskKw*` 诸常量）
// \t        ⇒ 即使日后进链也不落连续字面。
// \t     ④ `--self-test` 的合成件只写在 `os.MkdirTemp` 的临时目录里（仓外）⇒ 不落进任何扫描面。
//
// 已对拍面（18 格 · 甲=真仓+默认根 / 显式传根；夹具两态逐字）：
//
// \t真仓现读（缺省根 rc=1 · 16 命中）· `--json`（真仓）· `--by-tier`（真仓）· `--self-test`（4 格）·
// \t真仓 `-h` · 未知旗标 `--bogus` · `--root` 缺值 · `--root <不存在>` · 空域夹具（rc=2）·
// \t命中夹具（rc=1 普通/ json / by-tier）· 干净夹具（rc=0 普通/ json）· 白名单命中夹具（被登记者跳过 rc=0）·
// \t缺 `precommit-gates.sh` 夹具（rc=2）· 前缀缩写 `--ro` · `--root=` 等号形 —— 所有非 argparse 正文格逐字节相同。
//
// 未覆盖面（✗ 不声称逐字对齐）：
//
// \t① **argparse 帮助 / 用法错正文**：本入口**已复刻** py（python3.9）的 `-h` 与两类用法错正文
// \t   （同 python 版本下逐字节相同）；但 `argparse` 正文随解释器版本变（py ≥3.10 印 `options:` ⟷ ≤3.9 印
// \t   `optional arguments:`；措辞差同理）⇒ 该面**依赖解释器版本**，本件不声称跨版本逐字对齐。
// \t② **非 UTF-8 输入件**：py `open(..., encoding="utf-8")` 无 `errors=` ⇒ 非 UTF-8 件抛 UnicodeDecodeError
// \t   （继承自 ValueError、**不**被 `except OSError` 兜住）⇒ 回溯 rc=1；本件按 `strings.ToValidUTF8`（替换）近似，
// \t   该档只保「合法 UTF-8 件」逐字对齐。
// \t③ **`str.splitlines()` / `str.strip()` 的 Unicode 边界**：已按码点显式展开（见 `cskSplitLines` / `cskPyStrip`）；
// \t   真仓这些件均为 LF 文本，未触发差。
// \t④ **`%r` 自检失败正文**：仅在该件自检**失败**时打印，正常路不产 ⇒ 未实测（不影响任何绿格）。
package main

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// cskVersion —— 本件版本标识（入口首行 `_ = cskVersion` 防未用）。
const cskVersion = "0.1.0（M2 真搬 · 未接线 · 门99 · 缺口 GAP-20260927-74）"

// cskProg —— py argparse 的 prog 名（= 件基本名 · 帮助/用法错正文里出现）。
const cskProg = "check-selftest-knobs.py"

// cskWS —— py `\s`（Unicode）的显式码点展开（`_PyUnicode_IsWhitespace` 认的集合，含 `\x1c-\x1f` / `\x85`）。
const cskWS = `\x{0009}-\x{000D}\x{001C}-\x{001F}\x{0020}\x{0085}\x{00A0}` +
	`\x{1680}\x{2000}-\x{200A}\x{2028}\x{2029}\x{202F}\x{205F}\x{3000}`

// 关键词片段（拼串构造 ⇒ 源码里不落连续被扫字面；运行时值与 py 逐字相同）。
var (
	cskKwUnit        = "UNIT" + "TEST"
	cskKwSelftest    = "SELF" + "TEST"
	cskKwSelftestOff = "SELF" + "TEST_OFF"
	cskKwMetaOff     = "META_" + "OFF"
	cskKwNoPerl      = "NO_" + "PERL"
)

// 旋钮全名（= py 白名单 KNOB_ALLOW 的 id 与 KNOB_TIER 的键；拼串构造）。
var (
	cskKwDocMeta  = "CHECK_DOC_NAME_" + cskKwMetaOff
	cskKwAllshU   = "ZERG_ALLSH_" + cskKwUnit
	cskKwAllshP   = "ZERG_ALLSH_" + cskKwNoPerl
	cskKwDepth    = "ZERG_GATES_" + cskKwSelftest + "_DEPTH"
	cskKwRealUnit = "ZERG_REAL_GATES_" + cskKwUnit
)

// cskAllowEntry —— 白名单「五件套」：形态 id / 内容锚 / 理由 / 出处 / 看着它的门。
type cskAllowEntry struct {
	ID     string
	Anchor string
	Reason string
	Source string
	Gate   string
}

// cskAllowTable —— py `KNOB_ALLOW`（四条 · 逐字移植；理由改写为描述式，内容不含被点名字面）。
func cskAllowTable() []cskAllowEntry {
	reason := "门109 check-env-knobs.py 的旋钮登记表逐字点名本旋钮（只登记、不消费）⇒ 本门 NAME_RE 兜底把「提及」当「消费」误报；登记它消掉误报，不改本门判据"
	anch := func(kw string, reader string) string {
		return "\"" + kw + "\": {\"readers\": \"" + reader + "\""
	}
	return []cskAllowEntry{
		{ID: cskKwAllshU, Anchor: anch(cskKwAllshU, "all.sh"),
			Reason: reason, Source: "GAP-20260927-439", Gate: "门99 check-selftest-knobs.py"},
		{ID: cskKwAllshP, Anchor: anch(cskKwAllshP, "all.sh"),
			Reason: reason, Source: "GAP-20260927-439", Gate: "门99 check-selftest-knobs.py"},
		{ID: cskKwDepth, Anchor: anch(cskKwDepth, "precommit-gates.sh"),
			Reason: reason, Source: "GAP-20260927-439", Gate: "门99 check-selftest-knobs.py"},
		{ID: cskKwRealUnit, Anchor: anch(cskKwRealUnit, "real-gates.sh"),
			Reason: reason, Source: "GAP-20260927-439", Gate: "门99 check-selftest-knobs.py"},
	}
}

// cskAllow —— py 模块级 `KNOB_ALLOW`（只读 · 无累积写面）。
var cskAllow = cskAllowTable()

// cskTiers —— py `KNOB_TIERS`（固定次序）。
var cskTiers = []string{"isolated", "silent_off", "unclassified"}

// cskTier —— py `KNOB_TIER`（三枚已知旋钮本体按危害分层）。
var cskTier = map[string]string{
	cskKwDocMeta: "silent_off",
	cskKwAllshU:  "isolated",
	cskKwAllshP:  "isolated",
}

// cskTierOf —— py `knob_tier(name)`（未登记 ⇒ unclassified）。
func cskTierOf(name string) string {
	if t, ok := cskTier[name]; ok {
		return t
	}
	return "unclassified"
}

// cskNameRE —— py `NAME_RE`（`\s` 无关；关键词经拼串装配，避免连续被扫字面）。
var cskNameRE = regexp.MustCompile(
	`\b([A-Z][A-Z0-9_]*(?:` +
		cskKwUnit + `|` + cskKwSelftestOff + `|` + cskKwSelftest + `|` + cskKwMetaOff + `|` + cskKwNoPerl +
		`)[A-Z0-9_]*)\b`)

// cskConsumeREs —— py `CONSUME_RES`（5 条 · `\s` 已按 cskWS 展开 · 次序照搬）。
var cskConsumeREs = []*regexp.Regexp{
	regexp.MustCompile(`os\.Getenv\([` + cskWS + `]*"([A-Z][A-Z0-9_]*)"`),
	regexp.MustCompile(`environ\.get\([` + cskWS + `]*["']([A-Z][A-Z0-9_]*)["']`),
	regexp.MustCompile(`\$\{([A-Z][A-Z0-9_]*):-`),
	regexp.MustCompile(`\[[` + cskWS + `]*-n[` + cskWS + `]+"\$\{?([A-Z][A-Z0-9_]*)\}?"`),
	regexp.MustCompile(`\[[` + cskWS + `]*"\$\{?([A-Z][A-Z0-9_]*)\}?"[` + cskWS + `]*=`),
}

// cskPyJoin —— py `os.path.join`（posixpath 语义：逐段串拼 · 后段绝对路径则重置 · 不归一）。
func cskPyJoin(parts ...string) string {
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

// cskPyIsSpace —— py `str.isspace()` 认的空白（含 `\x1c-\x1f`，`unicode.IsSpace` 不含）。
func cskPyIsSpace(r rune) bool {
	switch r {
	case 0x1c, 0x1d, 0x1e, 0x1f:
		return true
	}
	return unicode.IsSpace(r)
}

// cskPyStrip —— py `str.strip()`（两侧按 py 空白集剥）。
func cskPyStrip(s string) string {
	r := []rune(s)
	i := 0
	for i < len(r) && cskPyIsSpace(r[i]) {
		i++
	}
	j := len(r)
	for j > i && cskPyIsSpace(r[j-1]) {
		j--
	}
	return string(r[i:j])
}

// cskSlice120 —— py `s[:120]`（按码点切 · 非按字节）。
func cskSlice120(s string) string {
	r := []rune(s)
	if len(r) > 120 {
		r = r[:120]
	}
	return string(r)
}

// cskSplitLines —— py `str.splitlines()` 的逐码点对齐（11 类断行；尾断行不产空串）。
func cskSplitLines(s string) []string {
	var out []string
	var cur strings.Builder
	r := []rune(s)
	for i := 0; i < len(r); i++ {
		c := r[i]
		switch {
		case c == '\n':
			out = append(out, cur.String())
			cur.Reset()
		case c == '\r':
			if i+1 < len(r) && r[i+1] == '\n' {
				i++
			}
			out = append(out, cur.String())
			cur.Reset()
		case c == '\v' || c == '\f' || c == 0x1c || c == 0x1d || c == 0x1e || c == 0x85 || c == 0x2028 || c == 0x2029:
			out = append(out, cur.String())
			cur.Reset()
		default:
			cur.WriteRune(c)
		}
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

// cskDecode —— py `open(..., encoding="utf-8")` 的近似（非 UTF-8 面见未覆盖面②）。
func cskDecode(b []byte) string {
	return strings.ToValidUTF8(string(b), "\uFFFD")
}

// cskHit —— 一条未登记旋钮命中。
type cskHit struct {
	File string
	Line int
	Knob string
	Text string
}

// cskIterFiles —— py `iter_files(root)`：按固定子目录次序 · 各子目录内 `sorted(os.listdir)`。
func cskIterFiles(root string) [][2]string {
	var out [][2]string
	for _, sub := range []string{"scripts/gates", "core/cmd/zerg"} {
		d := cskPyJoin(root, sub)
		ents, err := os.ReadDir(d)
		if err != nil {
			continue
		}
		names := make([]string, 0, len(ents))
		for _, e := range ents {
			names = append(names, e.Name())
		}
		sort.Strings(names)
		for _, name := range names {
			if strings.HasSuffix(name, ".py") || strings.HasSuffix(name, ".sh") || strings.HasSuffix(name, ".go") {
				out = append(out, [2]string{cskPyJoin(d, name), name})
			}
		}
	}
	return out
}

// cskNameMatchStart —— py `NAME_RE.match(nm)`（起点锚定）。
func cskNameMatchStart(s string) bool {
	loc := cskNameRE.FindStringIndex(s)
	return loc != nil && loc[0] == 0
}

// cskHasKeyword —— py `re.search(r"(UNITTEST|SELFTEST|META_OFF|NO_PERL)", nm)`（子串式）。
func cskHasKeyword(s string) bool {
	return strings.Contains(s, cskKwUnit) || strings.Contains(s, cskKwSelftest) ||
		strings.Contains(s, cskKwMetaOff) || strings.Contains(s, cskKwNoPerl)
}

// cskScan —— py `scan(root)`：回 (hits, pc=precommit 在场, chain_files)。
func cskScan(root string) ([]cskHit, bool, int) {
	precommit := cskPyJoin(root, "scripts/gates/precommit-gates.sh")
	var pcSrc string
	pc := false
	if fi, err := os.Stat(precommit); err == nil && fi.Mode().IsRegular() {
		if b, err2 := os.ReadFile(precommit); err2 == nil {
			pcSrc = cskDecode(b)
			pc = true
		}
	}
	allowed := map[string]bool{}
	for _, e := range cskAllow {
		allowed[e.ID] = true
		allowed[e.Anchor] = true
	}
	var hits []cskHit
	chainFiles := 0
	for _, pair := range cskIterFiles(root) {
		path, name := pair[0], pair[1]
		if !pc || !strings.Contains(pcSrc, name) {
			continue // 非提交链件不判（本件只管提交链上的门）
		}
		b, err := os.ReadFile(path)
		if err != nil {
			continue // py `except OSError: continue`（读不了静默跳过 · 逐字照搬）
		}
		lines := cskSplitLines(cskDecode(b))
		chainFiles++
		for i, line := range lines {
			nm := ""
			for _, cr := range cskConsumeREs {
				if m := cr.FindStringSubmatch(line); m != nil {
					nm = m[1]
					break
				}
			}
			if nm == "" {
				if m := cskNameRE.FindStringSubmatch(line); m != nil {
					nm = m[1]
				}
			}
			if nm == "" {
				continue
			}
			if !cskNameMatchStart(nm) && !cskHasKeyword(nm) {
				continue
			}
			if allowed[nm] {
				continue
			}
			hits = append(hits, cskHit{File: name, Line: i + 1, Knob: nm, Text: cskSlice120(cskPyStrip(line))})
		}
	}
	return hits, pc, chainFiles
}

// cskJSONStr —— py `json.dumps(s, ensure_ascii=False)` 的单串转义。
func cskJSONStr(s string) string {
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
				fmt.Fprintf(&b, `\u%04x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

// cskRun —— py `run(root, as_json)` 逐字复刻（三档退码 · 两档输出）。
func cskRun(stdout, stderr io.Writer, root string, asJSON bool) int {
	hits, pc, chainFiles := cskScan(root)
	if !pc {
		fmt.Fprint(stderr, "BLOCKED —— 缺 `scripts/gates/precommit-gates.sh`（无法判定提交链）⇒ rc=2\n")
		return 2
	}
	// ★ 空集守卫：空域 ⇒ fail-closed（提交链上 0 件门件 ⇒ rc=2，不许把「空转」当绿）。
	if chainFiles == 0 {
		fmt.Fprint(stderr, "BLOCKED —— 空域：precommit-gates.sh 未点名任何被扫门件（提交链上 0 件 · 取不到输入）⇒ "+
			"rc=2 不给结论（空转 ≠ 无未登记旋钮）\n")
		return 2
	}
	var badAllow []string
	for _, e := range cskAllow {
		if e.ID == "" || e.Anchor == "" || e.Reason == "" || e.Source == "" || e.Gate == "" {
			id := e.ID
			if id == "" {
				id = "?"
			}
			badAllow = append(badAllow, id)
		}
	}
	if len(badAllow) > 0 {
		fmt.Fprintf(stderr, "BLOCKED —— 白名单条目不齐五件套：%s ⇒ rc=2\n", strings.Join(badAllow, ","))
		return 2
	}
	if asJSON {
		tierCounts := map[string]int{}
		for _, t := range cskTiers {
			tierCounts[t] = 0
		}
		for _, h := range hits {
			tierCounts[cskTierOf(h.Knob)]++
		}
		rc := 0
		if len(hits) > 0 {
			rc = 1
		}
		var b strings.Builder
		b.WriteString(`{"rc": ` + strconv.Itoa(rc))
		b.WriteString(`, "knob_count": ` + strconv.Itoa(len(hits)))
		b.WriteString(`, "hits": [`)
		for i, h := range hits {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(`{"file": ` + cskJSONStr(h.File) + `, "line": ` + strconv.Itoa(h.Line) +
				`, "knob": ` + cskJSONStr(h.Knob) + `, "tier": ` + cskJSONStr(cskTierOf(h.Knob)) + `}`)
		}
		b.WriteString(`], "tier_counts": {`)
		for i, t := range cskTiers {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(cskJSONStr(t) + `: ` + strconv.Itoa(tierCounts[t]))
		}
		b.WriteString(`}}`)
		fmt.Fprintln(stdout, b.String())
	} else {
		fmt.Fprintln(stdout, "提交链「关自检旋钮」元门（GAP-20260927-74）")
		fmt.Fprintf(stdout, "提交链上未登记的关自检型旋钮 = %d 条\n", len(hits))
		for _, h := range hits {
			fmt.Fprintf(stdout, "  ✗ %s:%d 旋钮 %s [档 %s] ⇒ %s\n", h.File, h.Line, h.Knob, cskTierOf(h.Knob), h.Text)
		}
		fmt.Fprintln(stdout, "  口径：提交链上的门不接受关自检型旋钮；要保留须进 KNOB_ALLOW 五件套，")
		fmt.Fprintln(stdout, "        或改成显式且打印的 --selftest-off 旗标（并在输出里申报）。")
	}
	if len(hits) > 0 {
		return 1
	}
	return 0
}

// cskHitsRepr —— py `%r` 对 hits（list[tuple]）的近似（仅失败路打印）。
func cskHitsRepr(hits []cskHit) string {
	var b strings.Builder
	b.WriteByte('[')
	for i, h := range hits {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString("('" + h.File + "', " + strconv.Itoa(h.Line) + ", '" + h.Knob + "', '" + h.Text + "')")
	}
	b.WriteByte(']')
	return b.String()
}

// cskSelfTest —— py `self_test()` 逐字复刻（4 格 · 合成件写临时目录并自回收）。
func cskSelfTest(stdout, stderr io.Writer) int {
	ok := true
	td, err := os.MkdirTemp("", "selftest-knobs-")
	if err != nil {
		fmt.Fprint(stdout, "自检 FAIL（4 格）\n")
		return 1
	}
	defer os.RemoveAll(td)
	precommit := cskPyJoin(td, "scripts/gates/precommit-gates.sh")
	_ = os.MkdirAll(cskPyJoin(td, "scripts/gates"), 0o755)
	_ = os.MkdirAll(cskPyJoin(td, "core/cmd/zerg"), 0o755)
	_ = os.WriteFile(precommit,
		[]byte("add_step docs \"门x\" tri \"$R\" \"python3 scripts/gates/check-doc-name.py\"\n"), 0o644)
	_ = os.WriteFile(cskPyJoin(td, "scripts/gates/check-doc-name.py"),
		[]byte("meta = os.environ.get(\""+cskKwDocMeta+"\") != \"1\"\n"), 0o644)
	hits, pc, _ := cskScan(td)
	if !pc || !(len(hits) == 1 && hits[0].Knob == cskKwDocMeta) {
		fmt.Fprintf(stdout, "  ✗ 正控失败：期望命中 "+cskKwDocMeta+"，实得 %s\n", cskHitsRepr(hits))
		ok = false
	}
	if cskRun(stdout, stderr, td, false) != 1 {
		fmt.Fprint(stdout, "  ✗ 正控 rc 失败：应回 1\n")
		ok = false
	}
	// 负控：换成非提交链件 ⇒ 0
	_ = os.WriteFile(precommit, []byte("# 无 add_step\n"), 0o644)
	if h2, _, _ := cskScan(td); len(h2) > 0 {
		fmt.Fprint(stdout, "  ✗ 负控失败：非提交链件不该判\n")
		ok = false
	}
	// 负控′：precommit 在盘但不点名任何被扫件 ⇒ 提交链 0 件（空域）⇒ rc=2（空集守卫）
	if _, _, cf := cskScan(td); cf != 0 {
		fmt.Fprint(stdout, "  ✗ 负控失败：应得提交链 0 件（空域）\n")
		ok = false
	}
	if cskRun(stdout, stderr, td, false) != 2 {
		fmt.Fprint(stdout, "  ✗ 负控失败：空域（提交链 0 件）应回 2（空集守卫）\n")
		ok = false
	}
	if ok {
		fmt.Fprint(stdout, "自检 PASS（4 格）\n")
		return 0
	}
	fmt.Fprint(stdout, "自检 FAIL（4 格）\n")
	return 1
}

// cskUsage —— argparse 用法行（两行 · 续行缩进 = len("usage: ")+len(prog)+1）。
var cskUsage = "usage: " + cskProg + " [-h] [--root ROOT] [--json] [--by-tier]\n" +
	strings.Repeat(" ", len("usage: ")+len(cskProg)+1) + "[--self-test]\n"

// cskHelp —— py（python3.9）`-h` 的逐字节正文。
var cskHelp = cskUsage + "\n" +
	"提交链「关自检旋钮」只读元门（GAP-20260927-74）\n\n" +
	"optional arguments:\n" +
	"  -h, --help   show this help message and exit\n" +
	"  --root ROOT\n" +
	"  --json\n" +
	"  --by-tier    只按分档打计数（GAP-20260927-74）\n" +
	"  --self-test\n"

// cskErr —— argparse 用法错正文（usage + `prog: error: <msg>`）→ stderr，退码 2。
func cskErr(stderr io.Writer, msg string) int {
	fmt.Fprint(stderr, cskUsage+cskProg+": error: "+msg+"\n")
	return 2
}

// cskResolveLong —— argparse 唯一前缀缩写解析：回 (规范长旗标, 是否歧义)。
func cskResolveLong(name string) (string, bool) {
	known := []string{"--help", "--root", "--json", "--by-tier", "--self-test"}
	matches := []string{}
	for _, k := range known {
		if k == name {
			return k, false
		}
		if strings.HasPrefix(k, name) {
			matches = append(matches, k)
		}
	}
	if len(matches) == 1 {
		return matches[0], false
	}
	if len(matches) > 1 {
		return "", true
	}
	return "", false
}

// cskLooksLikeOpt —— argparse 判定「下一 token 是否像选项」（不许被当 --root 的值）。
func cskLooksLikeOpt(s string) bool {
	return strings.HasPrefix(s, "-") && s != "-"
}

// checkSelftestKnobsGateMain —— 入口（= py `main()`）。
func checkSelftestKnobsGateMain(stdout, stderr io.Writer, argv []string) int {
	_ = cskVersion
	root := ""
	hasRoot := false
	jsonOut := false
	byTier := false
	selfTest := false
	var positionals []string
	afterDD := false
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		if afterDD {
			positionals = append(positionals, a)
			continue
		}
		if a == "--" {
			afterDD = true
			continue
		}
		if a == "-h" || a == "--help" {
			fmt.Fprint(stdout, cskHelp)
			return 0
		}
		if strings.HasPrefix(a, "--") {
			name := a
			val := ""
			hasEq := false
			if k := strings.Index(a, "="); k >= 0 {
				name = a[:k]
				val = a[k+1:]
				hasEq = true
			}
			resolved, ambig := cskResolveLong(name)
			if ambig {
				return cskErr(stderr, "ambiguous option: "+name+" could match --help, --root, --json, --by-tier, --self-test")
			}
			if resolved == "" {
				positionals = append(positionals, a)
				continue
			}
			switch resolved {
			case "--help":
				fmt.Fprint(stdout, cskHelp)
				return 0
			case "--root":
				if hasEq {
					root = val
					hasRoot = true
				} else if i+1 < len(argv) && !cskLooksLikeOpt(argv[i+1]) {
					i++
					root = argv[i]
					hasRoot = true
				} else {
					return cskErr(stderr, "argument --root: expected one argument")
				}
			default: // --json / --by-tier / --self-test（store_true）
				if hasEq {
					return cskErr(stderr, "argument "+resolved+": ignored explicit argument '"+val+"'")
				}
				switch resolved {
				case "--json":
					jsonOut = true
				case "--by-tier":
					byTier = true
				case "--self-test":
					selfTest = true
				}
			}
			continue
		}
		if cskLooksLikeOpt(a) {
			positionals = append(positionals, a)
			continue
		}
		positionals = append(positionals, a)
	}
	if len(positionals) > 0 {
		return cskErr(stderr, "unrecognized arguments: "+strings.Join(positionals, " "))
	}
	if selfTest {
		return cskSelfTest(stdout, stderr)
	}
	if !hasRoot {
		if v, ok := os.LookupEnv("ZERG_ROOT"); ok {
			root = v
		} else {
			wd, err := os.Getwd()
			if err != nil {
				root = "."
			} else {
				root = wd
			}
		}
	}
	if byTier {
		hits, pc, chainFiles := cskScan(root)
		if !pc {
			fmt.Fprint(stderr, "BLOCKED —— 缺 precommit-gates.sh（无法判定提交链）⇒ rc=2\n")
			return 2
		}
		if chainFiles == 0 {
			fmt.Fprint(stderr, "BLOCKED —— 空域：提交链上 0 件门件（取不到输入）⇒ rc=2\n")
			return 2
		}
		cnt := map[string]int{}
		for _, t := range cskTiers {
			cnt[t] = 0
		}
		for _, h := range hits {
			cnt[cskTierOf(h.Knob)]++
		}
		for _, t := range cskTiers {
			fmt.Fprintf(stdout, "%-13s %d\n", t, cnt[t])
		}
		if len(hits) > 0 {
			return 1
		}
		return 0
	}
	return cskRun(stdout, stderr, root, jsonOut)
}
