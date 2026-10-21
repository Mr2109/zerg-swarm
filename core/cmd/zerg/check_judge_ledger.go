// check_judge_ledger.go —— 门禁件 `scripts/gates/check-judge-ledger.py` 的 **Go 真搬样板**（v2.5.14 · M2-0 · 路B续5）。
//
// 为什么有这件（不是「顺手 Go 化」）：
//
//	v2.5.14 定案（`设计-判据 CLI 化-v1.0-20260930.md` 第四节）把判据实现面分两段走 ——
//	**M1 = 先把 `scripts/` 下的独立件都收成 CLI 打得着的统一面（转发壳）**；
//	**M2 = 按价值**真搬 Go**（逐批判据搬 Go · 落点 `core/cmd/zerg/`）**。
//	本件 = M2-0 批（无外部树依赖 · 纯静态检查的判据）的一枚：
//	验收判据 = 对同一件跑两态（原件 ↔ 新入口），**stdout / stderr / 退码三面逐字相同**（4.2）。
//
// ⚠ 本单边界（照实）：本件**已接线（inproc 表分派）** ——
//
//	· **不**在 `core/cmd/zerg/main.go` 里注册任何命令（✗ 不进命令树 · `add_step` **已挂** `precommit-gates.sh:3712`（门85 · tri 档））；
//	· **不**删 `scripts/gates/check-judge-ledger.py` 原件（退场规则 4.3：**对拍通过后才**退场，且逐件显式点名）；
//	· **已接线**：入口 `judgeLedgerGateMain` 由 `gateCheckGoEntry["check-judge-ledger"]` 分派（`zerg gate check check-judge-ledger`）· 原文此处断言与现读相反（只改文案 · 未改行为）。
//
// 入口签名（与 py 的 `main()` 逐字对齐）：
//
//	func judgeLedgerGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv   —— 与 py `sys.argv[1:]` 同形（不含程序名）
//	  · 退码   —— 0 绿 / 1 红 / 2 不给结论（与 py 三码逐字对齐）
//	  · 输出   —— stdout/stderr 与 py 逐字节对齐（见下「已对拍面 / 未覆盖面」）
//
// ★★ 标准声明 (a)：**默认根定位差异 —— 有差异（且本单显式规避）**。
//
//	py `main()` 的 `--root` 缺省值 = `REPO = dirname(dirname(dirname(abspath(__file__))))`
//	= **脚本所在目录（`scripts/gates/`）上溯三层的仓根**。本件**无 `__file__` 语境**（未来接线后是
//	CLI 子命令）⇒ 缺省换成**当前工作目录**（`os.Getwd()` → `filepath.Abs`）——与 check-gotoolchain /
//	check-compat-manifest 两枚样板同一条口径。
//	落在**同一真仓、cwd = 仓根**时两者对 `--ledger`（相对 `scripts/gates/judge-run-ledger.jsonl`）的
//	解析**逐字相同**（已对拍）；「脚本位置 ≠ cwd」且都不给 `--root` 时路径推导面不同 ⇒ 这一面
//	**不属逐字对齐承诺**，对拍一律**显式传 `--root`**。
//
// 其余未覆盖面（✗ 不声称逐字对齐 · 接线前须逐面补齐或显式剔除）：
//
//	① **argparse 前缀缩写 + 用法/错误正文**：py 用 `argparse` ⇒ `--self-t` / `--che` 这类**唯一前缀**被接受、
//	   `-h/--help` 打 argparse 帮助正文、未知旗标打 `usage:` + `unrecognized arguments:` 正文（含
//	   `prog = check-judge-ledger.py`）—— 本件只认**精确名**，`-h`/`--help` 落自定 usage 行，未知旗标落
//	   自定错误行。**退码同（2）· 正文不同** ⇒ 不进逐字面。（对拍输入集已避开。）
//	② **异常路径**：台账**存在但读不到**（权限）时 py `isfile`=真 → `open` 抛 `OSError` → 文案
//	   `台账读不到：<py 的 errno 串>`；本件落 `台账读不到：<Go 的 err 串>` ⇒ **退码同（2）· 正文不同**。
//	   （台账是**目录**时两态同判 `台账不存在` —— 这条**可**逐字对齐。）
//	③ **坏行正文**：某行**非 JSON** 时 py 打 `json.JSONDecodeError` 的 `<str(e)>`；某行是**非对象** JSON 时 py 打
//	   `%r`（Python repr：`[...]` / `'x'` / `True` / `None`）—— 本件用 json.Number 保真 + 自实现 `jlgPyRepr`，
//	   对**常规标量/数组**逐字相同，对**嵌套 dict 的键序与转义**未做全枚举对拍。对拍输入集**不含**这两格。
//	④ py `--self-test` 的正文里有一处 `% ledger`（**临时目录随机名**）⇒ 该档 stdout **每次运行不同**、
//	   两态**天然不可逐字节对拍**；本件同功能（进程内复用入口取 rc · 逐格文案对齐），但**不列入对拍**。
//
// ★★ 标准声明 (b)：**自指四形态 —— 逐条真判**：
//
//	① 本件在**自己**的扫描面内吗？—— **否**。本门扫描面 = 调用方显式点名的 `--ledger` / `--baseline`
//	   两枚**数据件**（缺省 `scripts/gates/judge-run-ledger.jsonl`），**不扫任何源码树**：本件只有两个读点
//	   （`jlgReadLedger` 的 `os.Stat`+`os.ReadFile`、`jlgLoadLines` 的 `os.ReadFile`），**零 `WalkDir` /
//	   零 `os.ReadDir` / 零 `glob`**；py 原件同理（`open` / `os.path.isfile`）⇒ 本件源不进自身面。
//	② 本件落在**任何他门**的扫描面内吗？—— **是**（血证同 `check_private_path_forms.go`）。
//	   `scripts/gates/check-hardcoded-private-paths.py` 的 `SCAN_DIRS = ("core","agent","ui/src")` 含 `core/`
//	   ⇒ 本件 `core/cmd/zerg/check_judge_ledger.go` **在其扫描面内**（注释行豁免 · `*_test.go` 整体豁免）。
//	   该门 `NEEDLES` 判**三个私有卷/家目录绝对路径前缀**（逐字见该门源头的三枚字面量）。
//	   本件正文**零该三字面** —— 连注释也不复现原串（**本单第一发正是件头这句自己**把原串写进注释 ⇒
//	   `grep` 现读 1 行命中，已改成「不复现原串」的写法）⇒ 该门 0 命中。
//	   **证明**：落仓后 `grep -c <三字面>` 于本件 = **0 行**，且该门现读对本件 0 命中（本件非命中件）。
//	③ 本件**产出**回喂自身输入面（回环）吗？—— **否**。本件**只读**（py 件头「★ 本件只读」），
//	   stdout/stderr **不落任何文件**、不追加台账（写入方属壳侧）⇒ 无回环面。
//	④ 本件**测试件/夹具**里的标记字面被他门命中吗？—— **否（且被豁免）**。`check-hardcoded-private-paths.py`
//	   对 `*_test.go` **整体跳过**；且本件测试件夹具只用 `raw_rc` / `judge-run-ledger.jsonl` 等**通用名**与
//	   `t.TempDir()` 下的仓外路径，**零私有卷字面**。
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ── py 常量（**唯一真源**：与 py 逐字对齐 · 名字前缀 `jlg` 防包内同名）──────────────
const jlgLedgerDefault = "scripts/gates/judge-run-ledger.jsonl"

// jlgRequired —— py `REQUIRED`（六字段 · **顺序即报错顺序**）。
var jlgRequired = []string{"raw_rc", "detail", "sha256", "head", "judge_resolved", "dry_run"}

// jlgRCAliases —— py `RC_ALIASES`（「壳退码」别名 · 顺序即报错顺序）。
var jlgRCAliases = []string{"rc", "exit_code", "code", "shell_rc", "shell_exit_code"}

// 退码三档（与 py `RC_OK` / `RC_RED` / `RC_NO_VERDICT` 逐字对齐）。
const (
	jlgRCOK        = 0
	jlgRCRed       = 1
	jlgRCNoVerdict = 2
)

// jlgEntry —— 台账一行（对应 py `(lineno, obj)` 三元态：对象 / 解析错 / 非对象）。
type jlgEntry struct {
	lineno   int
	isObj    bool
	obj      map[string]interface{}
	parseErr string
	nonObj   string
}

// judgeLedgerGateMain —— 入口（= py `main()`）。
func judgeLedgerGateMain(stdout, stderr io.Writer, argv []string) int {
	root := ""
	ledger := jlgLedgerDefault
	baseline := ""
	doCheck := false
	doSelf := false
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "-h" || a == "--help":
			fmt.Fprintln(stdout, jlgUsage())
			return jlgRCOK
		case a == "--check":
			doCheck = true
		case a == "--self-test":
			doSelf = true
		case a == "--root":
			i++
			if i >= len(argv) {
				return jlgArgErr(stderr, "argument --root: expected one argument")
			}
			root = argv[i]
		case strings.HasPrefix(a, "--root="):
			root = a[len("--root="):]
		case a == "--ledger":
			i++
			if i >= len(argv) {
				return jlgArgErr(stderr, "argument --ledger: expected one argument")
			}
			ledger = argv[i]
		case strings.HasPrefix(a, "--ledger="):
			ledger = a[len("--ledger="):]
		case a == "--baseline":
			i++
			if i >= len(argv) {
				return jlgArgErr(stderr, "argument --baseline: expected one argument")
			}
			baseline = argv[i]
		case strings.HasPrefix(a, "--baseline="):
			baseline = a[len("--baseline="):]
		default:
			return jlgArgErr(stderr, "unrecognized arguments: "+a)
		}
	}

	if doSelf {
		return jlgSelfTest(stdout, stderr)
	}
	rootAbs := root
	if rootAbs == "" {
		rootAbs, _ = os.Getwd()
	}
	if abs, err := filepath.Abs(rootAbs); err == nil {
		rootAbs = abs
	}
	if doCheck {
		led := ledger
		if !filepath.IsAbs(led) {
			led = filepath.Join(rootAbs, led)
		}
		bl := baseline
		if bl != "" && !filepath.IsAbs(bl) {
			bl = filepath.Join(rootAbs, bl)
		}
		return jlgCmdCheck(stdout, stderr, led, bl)
	}
	fmt.Fprintln(stderr, "用法错：`--check` / `--self-test` 二选一必给一个（都不给 ⇒ rc=2）")
	return jlgRCNoVerdict
}

// jlgUsage —— 本件自定 usage（✗ 非 argparse 正文 · 见件头未覆盖面①）。
func jlgUsage() string {
	return "usage: check-judge-ledger.py [-h] [--root ROOT] [--check | --self-test] [--ledger LEDGER] [--baseline BASELINE]"
}

// jlgArgErr —— argparse 风格用法错（✗ 正文与 py 不同 · 退码同 2 · 见件头未覆盖面①）。
func jlgArgErr(stderr io.Writer, msg string) int {
	fmt.Fprintln(stderr, jlgUsage())
	fmt.Fprintf(stderr, "check-judge-ledger.py: error: %s\n", msg)
	return jlgRCNoVerdict
}

// jlgPyRepr —— Python `%r` 的最小实现（覆盖 json 解码出的常规值；嵌套 dict 键序未做全枚举对拍）。
func jlgPyRepr(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return "None"
	case bool:
		if t {
			return "True"
		}
		return "False"
	case json.Number:
		return t.String()
	case string:
		return "'" + strings.ReplaceAll(t, "'", "\\'") + "'"
	case []interface{}:
		parts := make([]string, len(t))
		for i, e := range t {
			parts[i] = jlgPyRepr(e)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	default:
		return fmt.Sprintf("%v", v)
	}
}

// jlgIsPyInt —— `isinstance(x, int)` 的近似：JSON 整数（无 `.` `e` `E`）⇒ 真；bool/float/串 ⇒ 假。
func jlgIsPyInt(v interface{}) bool {
	n, ok := v.(json.Number)
	if !ok {
		return false
	}
	return !strings.ContainsAny(n.String(), ".eE")
}

// jlgReadLedger —— py `read_ledger()`。err 非空 = 读不到（不给结论）。
func jlgReadLedger(path string) ([]jlgEntry, string) {
	fi, err := os.Stat(path)
	if err != nil || fi.IsDir() {
		return nil, "台账不存在：" + path
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "台账读不到：" + err.Error()
	}
	var entries []jlgEntry
	for i, line := range strings.Split(string(data), "\n") {
		s := strings.TrimSpace(line)
		if s == "" {
			continue
		}
		e := jlgEntry{lineno: i + 1}
		var v interface{}
		dec := json.NewDecoder(strings.NewReader(s))
		dec.UseNumber()
		if derr := dec.Decode(&v); derr != nil {
			e.parseErr = derr.Error()
		} else if m, ok := v.(map[string]interface{}); ok {
			e.isObj = true
			e.obj = m
		} else {
			e.nonObj = jlgPyRepr(v)
		}
		entries = append(entries, e)
	}
	return entries, ""
}

// jlgLoadLines —— py `load_lines()`：逐行 strip、丢空行。
func jlgLoadLines(path string) ([]string, string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "基线读不到：" + err.Error()
	}
	var out []string
	for _, l := range strings.Split(string(data), "\n") {
		t := strings.TrimSpace(l)
		if t != "" {
			out = append(out, t)
		}
	}
	return out, ""
}

// jlgProblemList —— py `problem_list()`：逐行列面 + 顶替 + append-only ⇒ 问题清单（空 = 绿）。
func jlgProblemList(entries []jlgEntry, rawLines, baselineLines []string, hasBaseline bool) []string {
	var problems []string
	for _, e := range entries {
		if e.parseErr != "" {
			problems = append(problems, fmt.Sprintf("第 %d 行 JSON 解析失败：%s", e.lineno, e.parseErr))
			continue
		}
		if !e.isObj {
			problems = append(problems, fmt.Sprintf("第 %d 行不是 JSON 对象：%s", e.lineno, e.nonObj))
			continue
		}
		obj := e.obj
		var missing []string
		for _, k := range jlgRequired {
			if _, ok := obj[k]; !ok {
				missing = append(missing, k)
			}
		}
		rawMissing := false
		for _, k := range missing {
			if k == "raw_rc" {
				rawMissing = true
			}
		}
		if rawMissing {
			var aliases []string
			for _, a := range jlgRCAliases {
				if _, ok := obj[a]; ok {
					aliases = append(aliases, a)
				}
			}
			if len(aliases) > 0 {
				problems = append(problems, fmt.Sprintf("第 %d 行 **`raw_rc` 被顶替**（硬边 · `raw_rc` 与壳退码须分列，不许同形）：用 %s 顶格", e.lineno, strings.Join(aliases, " / ")))
			} else {
				problems = append(problems, fmt.Sprintf("第 %d 行缺必填字段 `raw_rc`", e.lineno))
			}
		}
		for _, k := range missing {
			if k != "raw_rc" {
				problems = append(problems, fmt.Sprintf("第 %d 行缺必填字段 `%s`", e.lineno, k))
			}
		}
		if rv, ok := obj["raw_rc"]; ok && !jlgIsPyInt(rv) {
			problems = append(problems, fmt.Sprintf("第 %d 行 `raw_rc` 不是整数：%s", e.lineno, jlgPyRepr(rv)))
		}
		if jv, ok := obj["judge_resolved"]; ok {
			if _, isBool := jv.(bool); !isBool {
				problems = append(problems, fmt.Sprintf("第 %d 行 `judge_resolved` 不是布尔：%s", e.lineno, jlgPyRepr(jv)))
			}
		}
		if dv, ok := obj["dry_run"]; ok {
			if _, isBool := dv.(bool); !isBool {
				problems = append(problems, fmt.Sprintf("第 %d 行 `dry_run` 不是布尔：%s", e.lineno, jlgPyRepr(dv)))
			}
		}
	}

	// append-only：基线里的每一行都必须在现读台账里出现。
	if hasBaseline {
		have := map[string]bool{}
		for _, line := range rawLines {
			have[line] = true
		}
		for i, bl := range baselineLines {
			if !have[bl] {
				problems = append(problems, fmt.Sprintf("append-only 破（基线第 %d 行在现读台账里找不到 ⇒ 旧行被改/被删）", i+1))
			}
		}
	}
	return problems
}

// jlgCmdCheck —— py `cmd_check()`。
func jlgCmdCheck(stdout, stderr io.Writer, ledger, baseline string) int {
	entries, err := jlgReadLedger(ledger)
	if err != "" {
		fmt.Fprintf(stderr, "⛔ %s ⇒ **不给结论（rc=2）**\n", err)
		return jlgRCNoVerdict
	}
	if len(entries) == 0 {
		fmt.Fprintf(stderr, "⛔ 台账**空件/无有效行**（%s）⇒ **不给结论（rc=2）** ✗ 不当绿\n", ledger)
		return jlgRCNoVerdict
	}

	rawLines, err2 := jlgLoadLines(ledger)
	if err2 != "" {
		fmt.Fprintf(stderr, "⛔ %s ⇒ 不给结论（rc=2）\n", err2)
		return jlgRCNoVerdict
	}

	hasBaseline := baseline != ""
	var baselineLines []string
	if hasBaseline {
		bl, berr := jlgLoadLines(baseline)
		if berr != "" {
			fmt.Fprintf(stderr, "⛔ %s ⇒ 不给结论（rc=2）\n", berr)
			return jlgRCNoVerdict
		}
		baselineLines = bl
	}

	problems := jlgProblemList(entries, rawLines, baselineLines, hasBaseline)
	if len(problems) > 0 {
		fmt.Fprintf(stderr, "✗ 判据运行台账对拍**红**（%d 条）：\n", len(problems))
		for _, p := range problems {
			fmt.Fprintln(stderr, "  · "+p)
		}
		return jlgRCRed
	}
	suffix := ""
	if baseline != "" {
		suffix = fmt.Sprintf("（基线 %s）", baseline)
	}
	fmt.Fprintf(stdout, "✓ 判据运行台账对拍绿（rc=0）：行 %d · 六字段齐 · append-only 成立%s\n", len(entries), suffix)
	return jlgRCOK
}

// ── 自检（成对负控 · 合成台账全在临时目录 · 不碰真台账）────────────────────────────
// py `cmd_self_test()` 的 Go 同功能搬：进程内复用 `jlgCmdCheck` 取 rc（✗ 不猜退码）。
func jlgSelfTest(stdout, stderr io.Writer) int {
	ok := true
	d, err := os.MkdirTemp("", "judge-ledger-self-")
	if err != nil {
		fmt.Fprintf(stderr, "✗ 自检：建临时目录失败：%v\n", err)
		return jlgRCNoVerdict
	}
	defer os.RemoveAll(d)
	led := filepath.Join(d, "judge-run-ledger.jsonl")

	base := func() map[string]interface{} {
		return map[string]interface{}{
			"raw_rc": json.Number("0"), "detail": "ok",
			"sha256": strings.Repeat("a", 64), "head": "6aa1120e",
			"judge_resolved": true, "dry_run": false,
		}
	}
	write := func(obj map[string]interface{}) {
		b, _ := json.Marshal(obj)
		_ = os.WriteFile(led, append(b, '\n'), 0o644)
	}
	writeRaw := func(s string) { _ = os.WriteFile(led, []byte(s), 0o644) }
	run := func() int { return jlgCmdCheck(stdout, stderr, led, "") }

	// ── 正控：两行合法（六字段齐）⇒ rc=0 ─────────────────────────────
	{
		m1 := base()
		m2 := base()
		m2["raw_rc"] = json.Number("1")
		m2["detail"] = "judge_red"
		b1, _ := json.Marshal(m1)
		b2, _ := json.Marshal(m2)
		writeRaw(string(b1) + "\n" + string(b2) + "\n")
	}
	if run() == jlgRCOK {
		fmt.Fprintln(stdout, "✓ 正控：两行合法 ⇒ rc=0")
	} else {
		fmt.Fprintf(stdout, "✗ 正控：合法台账被判 rc=%d\n", run())
		ok = false
	}

	// ── 负控①：缺字段（抹 detail）⇒ rc=1 ────────────────────────────
	m := base()
	delete(m, "detail")
	write(m)
	if rc := run(); rc == jlgRCRed {
		fmt.Fprintln(stdout, "✓ 负控①（缺 `detail` 字段）：现 rc=1（红）")
	} else {
		fmt.Fprintf(stdout, "✗ 负控①（缺字段）：该红没红 ⇒ rc=%d\n", rc)
		ok = false
	}

	// ── 负控②：`raw_rc` 被 `rc` 顶替（硬边）⇒ rc=1 ─────────────────
	m = base()
	rcv := m["raw_rc"]
	delete(m, "raw_rc")
	m["rc"] = rcv
	write(m)
	if rc := run(); rc == jlgRCRed {
		fmt.Fprintln(stdout, "✓ 负控②（`raw_rc` 被 `rc` 顶替）：现 rc=1（红 · 硬边成立）")
	} else {
		fmt.Fprintf(stdout, "✗ 负控②（顶替退码）：该红没红 ⇒ rc=%d\n", rc)
		ok = false
	}

	// ── 负控③：空台账 ⇒ rc=2（不给结论 ✗ 不当绿）────────────────────
	writeRaw("")
	if rc := run(); rc == jlgRCNoVerdict {
		fmt.Fprintln(stdout, "✓ 负控③（空台账）：现 rc=2（不给结论 · ✗ 不当绿）")
	} else {
		fmt.Fprintf(stdout, "✗ 负控③（空台账）：该不给结论却没给 ⇒ rc=%d\n", rc)
		ok = false
	}

	// ── 负控④：台账不存在 ⇒ rc=2 ────────────────────────────────────
	if rc := jlgCmdCheck(stdout, stderr, filepath.Join(d, "absent.jsonl"), ""); rc == jlgRCNoVerdict {
		fmt.Fprintln(stdout, "✓ 负控④（台账不存在）：现 rc=2（不给结论）")
	} else {
		fmt.Fprintf(stdout, "✗ 负控④（台账不存在）：该不给结论却没给 ⇒ rc=%d\n", rc)
		ok = false
	}

	// ── 负控⑤：旧行被改 / 被删（对基线）⇒ rc=1 ──────────────────────
	baseline := filepath.Join(d, "baseline.jsonl")
	b := base()
	b["sha256"] = strings.Repeat("b", 64)
	bb, _ := json.Marshal(b)
	_ = os.WriteFile(baseline, append(bb, '\n'), 0o644)
	c := base()
	c["sha256"] = strings.Repeat("c", 64)
	cb, _ := json.Marshal(c)
	_ = os.WriteFile(led, append(cb, '\n'), 0o644)
	if rc := jlgCmdCheck(stdout, stderr, led, baseline); rc == jlgRCRed {
		fmt.Fprintln(stdout, "✓ 负控⑤（对基线：旧行被改/被删）：现 rc=1（append-only 破）")
	} else {
		fmt.Fprintf(stdout, "✗ 负控⑤（旧行被改删）：该红没红 ⇒ rc=%d\n", rc)
		ok = false
	}

	if ok {
		return jlgRCOK
	}
	return jlgRCNoVerdict
}
