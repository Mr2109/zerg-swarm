// bench_summary.go —— 门禁件 `scripts/gates/bench-summary.py` 的 **Go 真搬**（v2.5.14 · M2 路 A 续2）。
//
// 为什么有这件（不是「顺手 Go 化」）：
//
//	v2.5.14 定案（`设计-判据 CLI 化-v1.0-20260930.md` 第四节）把判据实现面分两段走 ——
//	**M1 = 先把 `scripts/` 下的独立件收成 CLI 打得着的统一面（转发壳）**；
//	**M2 = 按价值真搬 Go（逐批判据搬 Go · 落点 `core/cmd/zerg/`）**。
//	本件 = M2 路 A 续2（首字母 a–e 区间）的第三枚：验收判据 = 对同一件跑两态
//	（原件 ↔ 新入口），**stdout / stderr / 退码三面逐字相同**（4.2）。
//
// 选它 / 不选它的理由（选件口径 = 无外部树依赖 · 输出确定有序 · 退码仅 0/1/2 · 纯静态无写面 ·
// 行数适中 · 未被已搬 14 枚覆盖）：
//
//	✓ 选：`scripts/gates/bench-summary.py`（152 行 · 纯 stdlib json/os/re/statistics/argparse/tempfile）
//	  · **无外部树依赖**（只读调用方点名的 --json 件 / --from-results 趟目录，不碰文档仓 / 公开树 / 生成物）
//	  · **输出确定有序**（零 os.walk / 零 glob / 零目录遍历；唯一排序点 `sorted(rows, key=-secs)` 为
//	    **稳定排序** ⇒ 同键保持输入序，逐字节可证）
//	  · **退码仅 0/1/2**（0 有表 / 1 自检不过 / 2 缺字段·空转·用法错）
//	  · **纯静态无写面**（默认档只读；写面仅 `--self-test` 落 `tempfile.mkdtemp` 临时目录，跑完即删）
//	  · **无人正在改**（`git status --porcelain` 现读不含它）· 不属已列重件 · 不在已搬 14 枚内。
//	✗ 不选 `check-anchor-uniqueness.py`（340 行）：**外部树依赖**（读仓外 `Zerg-内部文档/` 登记件 + 逐条现扫
//	  目标件）⇒ 不满足「无外部树依赖」。
//	✗ 不选 `check-contract-version-diff.py`（434 行）：依赖 **git 子进程**（`git log`/`rev-parse`/`show`）
//	  取基线 ⇒ 输入面是「仓的可变历史」而非仓内静态件 ⇒ 非纯静态、且两态不可逐字对拍（HEAD 会动）。
//	✗ 不选 `check-dryrun-semantics.py`（349 行）：靠 **`grep -r` 子进程**扫六个目录 ⇒ 命中序依赖
//	  目录遍历序（非确定）+ 外部 `grep` 在盘 ⇒ 不满足「输出确定有序」。
//	✗ 不选 `check-curl-timeout.py`（167 行）：扫描根靠 `os.walk` ⇒ 遍历序非确定（Go `WalkDir` 字典序
//	  ≠ py `scandir` inode 序）⇒ 对拍会假差。
//	✗ 不选 8 件 `doc-*` / `check-dangling-refs.py` / `check-writer-guard.py` 等：**另一路在改**（`git status`
//	  现读）+ 属共享件禁改面。
//
// ⚠ 本单边界（照实）：本件**只落码、不接线** ——
//
//	· **不**在 `core/cmd/zerg/main.go` 里注册任何命令（✗ 不进命令树 · ✗ 不改 `add_step`）；
//	· **不**删 `scripts/gates/bench-summary.py` 原件（退场规则 4.3：**对拍通过后才**退场，逐件显式点名）；
//	· 入口 `benchSummaryGateMain` 目前**无调用点** —— 这是 M2 的**刻意**状态：先证「搬得逐字相同」，
//	  接线与命令注册是后一单的事。
//
// 入口签名（与 py 的 `main()` 逐字对齐）：
//
//	func benchSummaryGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv   —— 与 py `sys.argv[1:]` 同形（不含程序名）
//	  · 退码   —— 0 有表 / 1 自检不过 / 2 缺字段·空转·不给结论（与 py 逐字对齐）
//	  · 输出   —— stdout/stderr 与 py 逐字节对齐（见下「已对拍面 / 未覆盖面」）
//
// 已对拍面（仓外夹具两态逐字 · ≥8 输入）：--json 全绿（含 `--tier` 前缀）· 多趟中位/最差格式 ·
// 各造 violation（缺 real_ms · 闭集外字段 `secs` · 非整数 real_ms）· 空转（items 空 / 无 items 键）⇒
// 空转 ≠ 绿 · 取不到输入（--json 件不在盘 · --from-results 目录缺 results.tsv）· 未知旗标 ⇒ 2 ·
// `-h`/`--help` 帮助正文 ⇒ 0 · `--json` 缺值 ⇒ 2 · 无参 ⇒ 2 · `--self-test` ⇒ 0 · 真仓现读（一趟真门跑
// 结果目录）。
// 未覆盖面（✗ 不声称逐字对齐）：
//
//	① **argparse 帮助正文已逐字对齐**（`-h`/`--help` 落到固定 prog=`bench-summary.py` 的用法/选项块，
//	   与 py 3.9 的 `optional arguments:` 口径一致）⇒ 此处不列。未逐一验证的只剩 argparse 的**极端内部形态**：
//	   短旗标簇（`-hx`）· `--` 分隔符语义 · 缩写**歧义**报话（`--t` 同时匹配 `--tier`/`--tsv`）·
//	   `--json=`（空值 ⇒ py 走「用法」档）—— 本件实现长旗标前缀匹配 + `=` 取值 + 缺值报话，上述极端形态
//	   只做到了「主形态对齐」，不属逐字对齐承诺。
//	② **异常路径**：`--json` 件内容**不是合法 JSON** / 顶层**不是对象** ⇒ py `json.loads`/`doc.get` 抛
//	   未捕获异常 ⇒ traceback + 进程退码 1；`results.tsv` **非 UTF-8** / **读不到**（权限）⇒ py 同样抛
//	   traceback 退 1。本件在这些格落「读不到 / 解析失败」⇒ 退码同为 1，但 **stderr 正文与 py 的 traceback
//	   不同** ⇒ 这一面**不在**对拍输入集内。
//	③ **数值/文本极端形态**：`real_ms` 为 `float`（`1.5` ⇒ py `int(1.5)`=1）· 超大浮点（`1e400` ⇒ py
//	   `OverflowError`）· 前导 `+`/`-`/下划线 `1_0` 的字符串整数 · Unicode 十进制数字（`\d` 在 py `re` 下
//	   认全角数字）—— 本件按 **ASCII** 十进制实现，上述极端形态未逐一验证。
//	④ **浮点呈现**：`%.3f` 对有限 IEEE-754 double，Go `fmt` 与 py `%` 同为「就近偶数」舍入 ⇒ 常规读数
//	   （`real_ms` 整除 1000 的千分位）逐字相同；极端二进制尾数未做全枚举对拍。
//
// ★★ 标准声明 (a)：**默认根定位差异 —— 无差异**。本件**没有**「默认仓根」概念（py 里也没有）：
// 两个入参（`--json` 件路径 · `--from-results` 趟目录）都由调用方**显式给出**，相对路径一律按
// **当前工作目录**解析（py `open(path)` ⟷ Go `os.Open(path)` / `filepath.Join`）⇒ 无 `__file__`→cwd
// 的根定位落差，无需「先显式传根」的对拍豁免。其余未覆盖面见上②③④。
//
// ★★ 标准声明 (b)：**自指三形态 —— 前提为假（本件不在自身扫描面内）**。查法：本件的「扫描面」= 它会
// 读的件集 = 调用方显式点名的 `--json` 件 + `--from-results <目录>/results.tsv` 两处，**零目录遍历、
// 零 glob、零仓内件扫描**（对照：本批被排除的 `check-curl-timeout.py` 才 `os.walk` 全树）。本件自己的
// 仓内路径（`core/cmd/zerg/bench_summary.go`）既不可能是 `--json` 输入件、也永远不在任何趟目录里 ⇒
// **本件绝不可能落进它自己的扫描面** ⇒ 件头/正文可安全写真实字面量，无需拼串/转义。
// 「假」的依据（可现读复核）：① 全件只用 `os.ReadFile`/`os.Stat` 两个读点，无 `filepath.WalkDir`、
// 无 `os.ReadDir`、无 `glob`；② py 原件同理（`open(fp)` / `os.path.isfile`），故即便未来接线，扫描面
// 也仍由调用方定，不含本件。
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ── py 常量（**唯一真源**：与 py 逐字对齐 · 名字前缀 `bs` 防包内同名）─────────────
const (
	bsProg     = "bench-summary.py"
	bsUsageAlt = "用法：bench-summary.py --json <件> | --from-results <趟目录>"
	// py `sorted(KNOWN)` 的逐字结果（接口闭集一条真源）。
	bsKnownJoined = "log,rc,real_ms,run,sys_ms,user_ms"
	// secsRE = ^(\d+)s$
	bsSecsSuffix = "s"
)

var bsBanned = map[string]bool{"secs": true, "seconds": true, "real_s": true}

// bsUsageLine / bsHelpBody —— py argparse 的用法与帮助块（prog 固定 `bench-summary.py`），逐字。
const bsUsageLine = "usage: bench-summary.py [-h] [--json JSON] [--tier TIER]\n" +
	"                        [--from-results FROM_RESULTS] [--tsv] [--self-test]\n"

const bsHelpBody = bsUsageLine + "\n" +
	"optional arguments:\n" +
	"  -h, --help            show this help message and exit\n" +
	"  --json JSON\n" +
	"  --tier TIER\n" +
	"  --from-results FROM_RESULTS\n" +
	"  --tsv\n" +
	"  --self-test\n"

// ── 有序 JSON 对象（py dict 保插入序 ⇒ 逐键遍历序要复刻）────────────────────
type bsVal struct {
	kind string // "str" / "num" / "bool" / "null" / "other"
	s    string
	raw  string
}

type bsObj struct {
	keys []string
	vals map[string]bsVal
}

func (o *bsObj) get(k string) (bsVal, bool) {
	v, ok := o.vals[k]
	return v, ok
}

func bsValFromRaw(raw []byte) bsVal {
	t := strings.TrimSpace(string(raw))
	switch {
	case t == "null":
		return bsVal{kind: "null", raw: t}
	case t == "true" || t == "false":
		return bsVal{kind: "bool", raw: t}
	case strings.HasPrefix(t, "\""):
		var s string
		if err := json.Unmarshal([]byte(t), &s); err == nil {
			return bsVal{kind: "str", s: s, raw: t}
		}
		return bsVal{kind: "other", raw: t}
	case t != "":
		return bsVal{kind: "num", raw: t}
	}
	return bsVal{kind: "other", raw: t}
}

// bsObjFromRaw —— 用 json.Decoder.Token() 逐键取**文档序**（py dict 插入序）。
func bsObjFromRaw(raw []byte) (*bsObj, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return nil, fmt.Errorf("not a JSON object")
	}
	o := &bsObj{vals: map[string]bsVal{}}
	for dec.More() {
		kt, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, _ := kt.(string)
		var v json.RawMessage
		if err := dec.Decode(&v); err != nil {
			return nil, err
		}
		o.keys = append(o.keys, key)
		if _, dup := o.vals[key]; !dup {
			o.vals[key] = bsValFromRaw(v)
		}
	}
	if _, err := dec.Token(); err != nil { // 收尾 `}`
		return nil, err
	}
	return o, nil
}

// ── 入口（= py `main()`）────────────────────────────────────────────────
func benchSummaryGateMain(stdout, stderr io.Writer, argv []string) int {
	ns, perr := bsParseArgs(argv)
	if perr != nil {
		fmt.Fprint(stderr, bsUsageLine)
		fmt.Fprintf(stderr, "%s: error: %s\n", bsProg, perr.Error())
		return 2
	}
	if ns.help {
		fmt.Fprint(stdout, bsHelpBody)
		return 0
	}
	if ns.selfTest {
		return bsSelfTest(stdout)
	}
	var rc int
	var lines []string
	switch {
	case ns.fromResults != "":
		rc, lines = bsFromResults(ns.fromResults)
	case ns.json != "":
		if !bsIsFile(ns.json) {
			fmt.Fprintf(stderr, "✗ 缺件 %s ⇒ rc=2\n", ns.json)
			return 2
		}
		b, err := os.ReadFile(ns.json)
		if err != nil {
			// py `json.loads(open(...).read())` 抛未捕获异常 ⇒ traceback 退 1（见件头未覆盖面②）。
			fmt.Fprintf(stderr, "✗ 读不到 %s：%v\n", ns.json, err)
			return 1
		}
		items, err := bsLoadItems(b)
		if err != nil {
			fmt.Fprintf(stderr, "✗ 解析不了 %s：%v\n", ns.json, err)
			return 1
		}
		rc, lines = bsSummariseItems(items)
		if ns.tier != "" {
			lines = append([]string{"档位=" + ns.tier}, lines...)
		}
	default:
		fmt.Fprintln(stderr, bsUsageAlt)
		return 2
	}
	for _, x := range lines {
		fmt.Fprintln(stdout, x)
	}
	return rc
}

// ── argparse 复刻（`-h/--help` · 长旗标前缀匹配 · `=` 取值 · 缺值报话 · 未知旗标）──
type bsNS struct {
	json        string
	tier        string
	fromResults string
	tsv         bool
	selfTest    bool
	help        bool
}

var bsLongOpts = []string{"help", "json", "tier", "from-results", "tsv", "self-test"}

func bsParseArgs(argv []string) (*bsNS, error) {
	ns := &bsNS{}
	var unknown []string
	i := 0
	for ; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "--":
			unknown = append(unknown, argv[i+1:]...)
			i = len(argv)
		case strings.HasPrefix(a, "--"):
			body := a[2:]
			val := ""
			hasVal := false
			if eq := strings.Index(body, "="); eq >= 0 {
				val, hasVal = body[eq+1:], true
				body = body[:eq]
			}
			canon, ambig := bsResolveLong(body)
			if ambig != "" {
				return nil, fmt.Errorf("ambiguous option: --%s could match %s", body, ambig)
			}
			if canon == "" {
				unknown = append(unknown, a)
				continue
			}
			switch canon {
			case "help":
				ns.help = true
			case "tsv":
				ns.tsv = true
			case "self-test":
				ns.selfTest = true
			default:
				if !hasVal {
					if i+1 >= len(argv) {
						return nil, fmt.Errorf("argument --%s: expected one argument", body)
					}
					i++
					val = argv[i]
				}
				switch canon {
				case "json":
					ns.json = val
				case "tier":
					ns.tier = val
				case "from-results":
					ns.fromResults = val
				}
			}
		case strings.HasPrefix(a, "-") && a != "-":
			// 短旗标：py argparse 只认 `-h`；其余（含簇）落未知旗标。
			if a == "-h" {
				ns.help = true
			} else {
				unknown = append(unknown, a)
			}
		default:
			unknown = append(unknown, a)
		}
	}
	if len(unknown) > 0 {
		return nil, fmt.Errorf("unrecognized arguments: %s", strings.Join(unknown, " "))
	}
	return ns, nil
}

// bsResolveLong —— py argparse 的长旗标**前缀匹配**；唯一命中 ⇒ 规范名；多命中 ⇒ 歧义报话。
func bsResolveLong(body string) (string, string) {
	if body == "" {
		return "", ""
	}
	for _, o := range bsLongOpts {
		if o == body {
			return o, ""
		}
	}
	var hits []string
	for _, o := range bsLongOpts {
		if strings.HasPrefix(o, body) {
			hits = append(hits, "--"+o)
		}
	}
	if len(hits) == 1 {
		return strings.TrimPrefix(hits[0], "--"), ""
	}
	if len(hits) > 1 {
		return "", strings.Join(hits, ", ")
	}
	return "", ""
}

// ── py `summarise_items()` ─────────────────────────────────────────────
func bsSummariseItems(items []*bsObj) (int, []string) {
	if len(items) == 0 {
		return 2, []string{"[不给结论] bench items 为空 ⇒ 空转不许当绿 ⇒ rc=2"}
	}
	secs := make([]float64, 0, len(items))
	for i, it := range items {
		for _, k := range it.keys {
			if bsBanned[k] {
				return 2, []string{fmt.Sprintf("[不给结论] C1 未知字段 `%s`（接口闭集 %s）⇒ rc=2", k, bsKnownJoined)}
			}
		}
		v, ok := it.get("real_ms")
		if !ok || v.kind == "null" || (v.kind == "str" && v.s == "") {
			return 2, []string{fmt.Sprintf("[不给结论] C0 第 %d 趟缺 real_ms ⇒ rc=2", i)}
		}
		n, err := bsPyInt(v)
		if err != nil {
			return 2, []string{fmt.Sprintf("[不给结论] C0 real_ms 不是整数：%s ⇒ rc=2", bsPyRepr(v))}
		}
		secs = append(secs, float64(n)/1000.0)
	}
	lines := []string{fmt.Sprintf("┌─ 档位汇总（%d 趟）", len(secs))}
	lines = append(lines, fmt.Sprintf("│ 中位 real = %.3fs", bsMedian(secs)))
	lines = append(lines, fmt.Sprintf("│ 最差 real = %.3fs  ← 棘轮取数（max）", bsMax(secs)))
	parts := make([]string, len(secs))
	for i, s := range secs {
		parts[i] = fmt.Sprintf("%.3fs", s)
	}
	lines = append(lines, "└ 逐趟 real = "+strings.Join(parts, ", "))
	return 0, lines
}

// ── py `from_results()` ────────────────────────────────────────────────
type bsRow struct {
	name string
	rc   string
	s    int
	hasS bool
	raw  string
}

func bsFromResults(d string) (int, []string) {
	fp := filepath.Join(d, "results.tsv")
	if !bsIsFile(fp) {
		return 2, []string{fmt.Sprintf("[不给结论] 缺 %s ⇒ rc=2", fp)}
	}
	b, err := os.ReadFile(fp)
	if err != nil {
		// py `open(fp)` 抛未捕获异常 ⇒ traceback 退 1（见件头未覆盖面②）。
		return 2, []string{fmt.Sprintf("[不给结论] 读不到 %s：%v ⇒ rc=2", fp, err)}
	}
	txt := bsUniversalNewlines(string(b))
	linesRaw := strings.Split(txt, "\n")
	if n := len(linesRaw); n > 0 && linesRaw[n-1] == "" && strings.HasSuffix(txt, "\n") {
		linesRaw = linesRaw[:n-1]
	}
	var rows []bsRow
	for _, line := range linesRaw {
		c := strings.Split(strings.TrimSuffix(line, "\n"), "\t")
		if len(c) < 4 {
			continue
		}
		r := bsRow{name: c[1], rc: c[2], raw: bsPyStrip(c[3])}
		if n, ok := bsSecsParse(r.raw); ok {
			r.s, r.hasS = n, true
		}
		rows = append(rows, r)
	}
	if len(rows) == 0 {
		return 2, []string{"[不给结论] results.tsv 无步行 ⇒ rc=2"}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		return bsSortKey(rows[i]) < bsSortKey(rows[j])
	})
	lines := []string{
		fmt.Sprintf("┌─ 逐 step 秒数（%d 步 · 源 %s）", len(rows), fp),
		"│ " + bsPadRune("秒", 4) + " " + bsPadRune("rc", 6) + " 步名",
	}
	tot := 0
	for _, r := range rows {
		if r.hasS {
			tot += r.s
		}
		lines = append(lines, "│ "+bsPadRune(r.raw, 4)+" "+bsPadRune(r.rc, 6)+" "+r.name)
	}
	lines = append(lines, fmt.Sprintf("└ 合计 %ds（%d 步）", tot, len(rows)))
	return 0, lines
}

// bsSortKey —— py `-(r[2] or 0)`。
func bsSortKey(r bsRow) int {
	if r.hasS {
		return -r.s
	}
	return 0
}

// bsSecsParse —— py `^(\d+)s$` + `int(group)`（ASCII 十进制；见件头未覆盖面③）。
func bsSecsParse(s string) (int, bool) {
	if !strings.HasSuffix(s, bsSecsSuffix) {
		return 0, false
	}
	digits := s[:len(s)-1]
	if digits == "" {
		return 0, false
	}
	for _, r := range digits {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	n, err := strconv.Atoi(digits)
	if err != nil {
		return 0, false
	}
	return n, true
}

// ── py `--self-test`（5 控 · 打印文案逐字）──────────────────────────────
func bsSelfTest(stdout io.Writer) int {
	var fails []string
	if rc, lines := bsSummariseItems([]*bsObj{
		{keys: []string{"real_ms"}, vals: map[string]bsVal{"real_ms": {kind: "str", s: "24763"}}},
		{keys: []string{"real_ms"}, vals: map[string]bsVal{"real_ms": {kind: "str", s: "24763"}}},
	}); rc != 0 || !bsAnyContains(lines, "24.763s") {
		fails = append(fails, fmt.Sprintf("① 正控期望 rc=0 且 24.763s，实得 rc=%d", rc))
	}
	if rc, _ := bsSummariseItems([]*bsObj{
		{keys: []string{"run"}, vals: map[string]bsVal{"run": {kind: "str", s: "x"}}},
	}); rc != 2 {
		fails = append(fails, fmt.Sprintf("② 缺 real_ms 期望 rc=2，实得 %d", rc))
	}
	if rc, lines := bsSummariseItems([]*bsObj{
		{keys: []string{"real_ms", "secs"}, vals: map[string]bsVal{
			"real_ms": {kind: "str", s: "1"}, "secs": {kind: "str", s: "2"}}},
	}); rc != 2 || !bsAnyContains(lines, "未知字段") {
		fails = append(fails, fmt.Sprintf("③ 未知字段期望 rc=2，实得 rc=%d", rc))
	}
	if rc, _ := bsSummariseItems(nil); rc != 2 {
		fails = append(fails, fmt.Sprintf("④ 空集期望 rc=2，实得 %d", rc))
	}
	tmp, err := os.MkdirTemp("", "bench-summ-")
	if err == nil {
		defer func() { _ = os.RemoveAll(tmp) }()
		if err := os.WriteFile(filepath.Join(tmp, "results.tsv"),
			[]byte("PASS\ta-step\t0\t3s\t/x.log\nFAIL\tb-step\t1\t12s\t/y.log\n"), 0o644); err == nil {
			if rc, lines := bsFromResults(tmp); rc != 0 || !bsAnyContains(lines, "合计 15s") {
				tail := ""
				if len(lines) > 0 {
					tail = lines[len(lines)-1]
				}
				fails = append(fails, fmt.Sprintf("⑤ results 解析期望 rc=0/合计 15s，实得 rc=%d [%s]", rc, bsPyRepr(bsVal{kind: "str", s: tail})))
			}
		}
	}
	if len(fails) > 0 {
		fmt.Fprintln(stdout, "自检不过：")
		for _, x := range fails {
			fmt.Fprintln(stdout, "  ✗ "+x)
		}
		return 1
	}
	fmt.Fprintln(stdout, "自检通过：5 控（正控 2 + 负控 3）逐条对拍 ✓")
	return 0
}

// ── 小工具 ─────────────────────────────────────────────────────────────
func bsLoadItems(b []byte) ([]*bsObj, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(b, &top); err != nil {
		return nil, err
	}
	rawItems, ok := top["items"]
	if !ok {
		return nil, nil
	}
	var raws []json.RawMessage
	if err := json.Unmarshal(rawItems, &raws); err != nil {
		return nil, err
	}
	out := make([]*bsObj, 0, len(raws))
	for _, r := range raws {
		o, err := bsObjFromRaw(r)
		if err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, nil
}

func bsMedian(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	c := append([]float64(nil), xs...)
	sort.Float64s(c)
	n := len(c)
	if n%2 == 1 {
		return c[n/2]
	}
	return (c[n/2-1] + c[n/2]) / 2
}

func bsMax(xs []float64) float64 {
	m := xs[0]
	for _, x := range xs[1:] {
		if x > m {
			m = x
		}
	}
	return m
}

// bsPyInt —— py `int(v)`：str 走十进制（可带空白/正负号）· 数字截断取整 · bool 0/1。
func bsPyInt(v bsVal) (int, error) {
	switch v.kind {
	case "bool":
		if v.raw == "true" {
			return 1, nil
		}
		return 0, nil
	case "num":
		t := v.raw
		if strings.ContainsAny(t, ".eE") {
			f, err := strconv.ParseFloat(t, 64)
			if err != nil {
				return 0, err
			}
			if f > 9.2e18 || f < -9.2e18 {
				return 0, fmt.Errorf("overflow")
			}
			return int(f), nil // Go 截断向零 = py `int(float)`
		}
		n, err := strconv.Atoi(t)
		if err != nil {
			return 0, err
		}
		return n, nil
	case "str":
		return bsPyIntStr(v.s)
	}
	return 0, fmt.Errorf("not an integer")
}

func bsPyIntStr(s string) (int, error) {
	t := strings.TrimFunc(s, unicode.IsSpace)
	if t == "" {
		return 0, fmt.Errorf("invalid literal")
	}
	sign := 1
	if t[0] == '+' || t[0] == '-' {
		if t[0] == '-' {
			sign = -1
		}
		t = t[1:]
	}
	if t == "" {
		return 0, fmt.Errorf("invalid literal")
	}
	digits := make([]byte, 0, len(t))
	for i := 0; i < len(t); i++ {
		c := t[i]
		if c == '_' {
			if i == 0 || i == len(t)-1 || t[i-1] == '_' || t[i+1] == '_' {
				return 0, fmt.Errorf("invalid literal")
			}
			continue
		}
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("invalid literal")
		}
		digits = append(digits, c)
	}
	if len(digits) > 18 {
		n, err := strconv.ParseInt(string(digits), 10, 64)
		if err != nil {
			return 0, err
		}
		return int(n) * sign, nil
	}
	n, err := strconv.Atoi(string(digits))
	if err != nil {
		return 0, err
	}
	return n * sign, nil
}

// bsPyRepr —— py `%r`（本门取值面只出字符串/整数/布尔/null，见件头未覆盖面③）。
func bsPyRepr(v bsVal) string {
	switch v.kind {
	case "str":
		q := "'"
		if strings.Contains(v.s, "'") && !strings.Contains(v.s, "\"") {
			q = "\""
		}
		body := strings.ReplaceAll(v.s, "\\", "\\\\")
		body = strings.ReplaceAll(body, q, "\\"+q)
		return q + body + q
	case "bool":
		if v.raw == "true" {
			return "True"
		}
		return "False"
	case "null":
		return "None"
	case "num":
		return v.raw
	}
	return v.raw
}

// bsPadRune —— py `%-Ns`：按**字符**（rune）左对齐填充（Go `%-Ns` 按字节 ⇒ 中文会少填）。
func bsPadRune(s string, w int) string {
	n := utf8.RuneCountInString(s)
	if n >= w {
		return s
	}
	return s + strings.Repeat(" ", w-n)
}

// bsPyStrip —— py `str.strip()`（去两端 unicode 空白）。
func bsPyStrip(s string) string { return strings.TrimFunc(s, unicode.IsSpace) }

// bsUniversalNewlines —— py 文本模式通用换行翻译（`\r\n`/`\r` → `\n`）。
func bsUniversalNewlines(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}

func bsIsFile(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

func bsAnyContains(lines []string, sub string) bool {
	for _, l := range lines {
		if strings.Contains(l, sub) {
			return true
		}
	}
	return false
}
