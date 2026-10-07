// family_repo_diffstat.go —— `zerg repo diffstat --report <件>`（T20）。
//
// 病灶（逐字）：报行数**三格的机检面**没有正门 —— 今天判「一份 diffstat 是不是把
// **原始 diff / 增 / 减** 三格都报齐了」只能肉眼比；而**只给净数**（一个净差）在仓里
// 竟能一路当绿走过去。T20 的口径（逐字）：报数**一律三格**，**只给净数即判未报**。
//
// 三格（判据面逐字，缺一格即**判不了**）：
//
//	raw 原始 diff / add 增 / del 减
//
// ★ 退码（**判不了 ≠ 绿** —— 本命令的退码判的是「报告报齐没有」，不是「净数好不好看」）：
//
//	0  三格**逐字齐**（原始 diff / 增 / 减 都给了）—— 唯一写 stdout 的一档
//	1  输入为空（无读数 —— 也判不了）
//	2  缺格（**含「只给净数」** ⇒ 未报 ⇒ 判不了）· 用法错（缺 `--report` / 多余位置参数 / `--json` 字段表外）
//	8  **判不了**（`--report` 件读不到 ⇒ 不给结论）
//
// ★ 面流口径（照 T09/T11 那批的矩阵格：早判档 `want_stdout_bytes=0`）：
// **所有非 0 档一律只写 stderr · stdout 恒 0 字节**（判据件流在 stderr，机器面不受污染）；
// 只有三格齐（退 0）才写 stdout。
// ★ 只读：`os.ReadFile` 一枚件，**不写不删不改**任何件（跑完仓内件 sha 不变）。
// ★ 与 `repo status` 同族同守：**写旗标一律拒**（`repoWriteWords` · 形态无关）。
package main

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

// repoDiffstatCells —— 三格闭集（逐字；顺序即人面顺序）。
var repoDiffstatCells = []string{"raw", "add", "del"}

// repoDiffstatLabels —— 三格的中文名（报数面逐字）。
var repoDiffstatLabels = map[string]string{"raw": "原始 diff", "add": "增", "del": "减"}

// repoDiffstatFields —— `--json` 的字段表（每格一行四列：格名 / 中文名 / 值 / 命中否）。
var repoDiffstatFields = []string{"cell", "label", "value", "hit"}

// repoDiffstatCellPat —— 三格认法（点名格位 · 不抄行号）：
//
//	`原始 diff N` / 分隔符+`增 N` / 分隔符+`减 N`
//
// ★ 增/减 那两枚**要求前置分隔符**（空白 / `:` / `=` / `·` / `|` / `,` / 括号）——
// 这样 `净增 96`（净数措辞里的「增」）**不会被误当成 add 格**（「只给净数」必须真判成缺格）。
var repoDiffstatCellPat = map[string]*regexp.Regexp{
	"raw": regexp.MustCompile(`原始\s*diff\s*[:：=]?\s*(\d+)`),
	"add": regexp.MustCompile(`(?:^|[\s:：=·|,，、（(【])增\s*[:：=]?\s*(\d+)`),
	"del": regexp.MustCompile(`(?:^|[\s:：=·|,，、（(【])减\s*[:：=]?\s*(\d+)`),
}

// repoDiffstatNetOnly —— 「只给净数」的现读征兆（净 / net 字样）。
var repoDiffstatNetOnly = regexp.MustCompile(`(净|net)`)

// repoDiffstatFieldLegal —— `--json` 字段白名单（照同族口径）。
func repoDiffstatFieldLegal(f string) bool {
	for _, ok := range repoDiffstatFields {
		if f == ok {
			return true
		}
	}
	return false
}

// repoDiffstatScan —— 三格认读：返回（命中格 → 值）。缺的格**不补 0**（缺就是缺）。
func repoDiffstatScan(text string) map[string]string {
	out := map[string]string{}
	for k, re := range repoDiffstatCellPat {
		if m := re.FindStringSubmatch(text); m != nil {
			out[k] = m[1]
		}
	}
	return out
}

// cmdRepoDiffstat —— `repo diffstat`（只读 · 不走脚本 · 不改任何件）。
func cmdRepoDiffstat(inv *invocation, stdout, stderr io.Writer) int {
	// 同族同守（照 `repo status` 那一口径）：写旗标一律拒。
	if hits := repoWriteHits(inv.orig); len(hits) > 0 {
		inv.setErr("usage", "write_flag_refused", "本命令是只读机检 · 写旗标一律拒")
		fmt.Fprintf(stderr, "%s: 拒（退码 2 · 不给结论）：命令行里出现写旗标 %v\n", progName, hits)
		fmt.Fprintf(stderr, "%s: `repo diffstat` **只读** · 要写请用 `%s repo commit`\n", progName, progName)
		return exitUsage
	}
	rep := strings.TrimSpace(inv.flagVal("--report"))
	if rep == "" {
		inv.setErr("usage", "missing_report", "缺 --report（报告件）")
		fmt.Fprintf(stderr, "%s: `repo diffstat` 要给一枚报告件：`--report <件>`\n", progName)
		fmt.Fprintf(stderr, "口径：报行数**一律三格**（原始 diff / 增 / 减）—— **只给净数即判未报**（不给结论）\n")
		return exitUsage
	}
	if len(inv.args) > 0 {
		inv.setErr("usage", "extra_args", "多余位置参数")
		fmt.Fprintf(stderr, "%s: `repo diffstat` 不收位置参数（报告件由 `--report` 指）—— 多给了 %q\n", progName, inv.args[0])
		return exitUsage
	}
	fields := inv.fields
	if len(fields) == 0 {
		fields = repoDiffstatFields
	}
	for _, f := range fields {
		if !repoDiffstatFieldLegal(f) {
			inv.setErr("usage", "json_field_unknown:"+f, fmt.Sprintf("未知字段 %q", f))
			return reportBadField(stderr, inv.path, f)
		}
	}

	b, err := os.ReadFile(rep)
	if err != nil {
		// 判不了 ⇒ 不给结论（8）：件读不到既不是「齐」也不是「缺」。
		inv.setErr("blocked", "report_unreadable", "报告件读不到")
		fmt.Fprintf(stderr, "%s: **判不了** ⇒ 不给结论（退码 8）\n", progName)
		fmt.Fprintf(stderr, "  --report %s：%v\n", rep, err)
		fmt.Fprintf(stderr, "口径：读不到既不是「三格齐」也不是「缺格」—— 不许当绿\n")
		return exitBlocked
	}
	text := string(b)
	if strings.TrimSpace(text) == "" {
		inv.setErr("failed", "report_empty", "报告件为空")
		fmt.Fprintf(stderr, "%s: 报告件：%s\n", progName, rep)
		fmt.Fprintf(stderr, "!! 输入为空 ⇒ 无读数 ⇒ 判不了（退 1）· 不许当绿\n")
		return exitFail
	}
	hits := repoDiffstatScan(text)
	missing := []string{}
	for _, c := range repoDiffstatCells {
		if _, ok := hits[c]; !ok {
			missing = append(missing, c)
		}
	}
	if len(missing) > 0 {
		inv.setErr("usage", "cells_missing:"+strings.Join(missing, ","), "三格缺格（未报）")
		names := []string{}
		for _, c := range missing {
			names = append(names, repoDiffstatLabels[c])
		}
		fmt.Fprintf(stderr, "%s: 报告件：%s\n", progName, rep)
		fmt.Fprintf(stderr, "✗ 未报：缺 %d 格（%s）—— 报行数一律三格（原始 diff / 增 / 减），只给净数视为未报\n",
			len(missing), strings.Join(names, " · "))
		if repoDiffstatNetOnly.MatchString(text) {
			fmt.Fprintf(stderr, "  （现读含「净」字样 ⇒ 只给了净数）\n")
		}
		fmt.Fprintf(stderr, "结论：判不了 ⇒ 不给结论（退 2）\n")
		return exitUsage
	}

	if inv.jsonGiven {
		if len(inv.fields) == 0 {
			inv.fields = fields
		}
		inv.metaAddStr("report", rep)
		inv.metaAddJSON("cells", fmt.Sprintf("%d", len(repoDiffstatCells)))
		items := make([]map[string]string, 0, len(repoDiffstatCells))
		for _, c := range repoDiffstatCells {
			items = append(items, map[string]string{
				"cell": c, "label": repoDiffstatLabels[c], "value": hits[c], "hit": "yes",
			})
		}
		return selectJSONList(stdout, stderr, inv, inv.path, fields, items)
	}

	fmt.Fprintf(stdout, "报告件：%s\n", rep)
	fmt.Fprintf(stdout, "✓ 三格齐：原始 diff=%s · 增=%s · 减=%s\n", hits["raw"], hits["add"], hits["del"])
	fmt.Fprintf(stdout, "结论：三格逐字齐（退 0）—— 报数面齐了\n")
	fmt.Fprintf(stdout, "（本命令只读 · 不改报告件 · 也不产报告件）\n")
	return exitOK
}

// ── M2 接线入口：`zerg gate check repo-diffstat-check`（进程内）────────────────────────────────
//
// 本件是 `zerg repo diffstat`（T20 正门一条）的**本体**；其判据件
// `scripts/gates/repo-diffstat-check.py`（55 行）是它的**薄壳**：只解析 `--file`（缺省读 stdin 落
// 临时件），把判定**整件委托**给正门 `zerg repo diffstat --report <件>`，逐字透传 rc（退码口径
// 3 格齐 0 / 空 1 / 缺格 2 / 件读不到 8 全在正门）。
// ⇒ 本入口 = 那条委托的**进程内等价物**：解析面照 py argparse 复刻，判定面直接调本件
// `cmdRepoDiffstat`（✗ 不起子进程 · ✗ 不改 `cmdRepoDiffstat` 一个字节 · 纯接口适配层）。
//
// 两条标准声明（照 M2 口径）：
//
//	(a) **默认根定位**：py 薄壳**无任何路径拼接 / 无 `__file__` 语境** —— 只把 `--file`（或临时件）
//	    路径**原样**交给正门；Go 侧同构（同样原样交给 `cmdRepoDiffstat`）⇒ 两侧与 cwd 无关、无推导差。
//	(b) **自指**：本件**不在**本门判据面内 —— 本门只读它被点名的那**一枚**报告件、不扫任何源码
//	    ⇒ 自指为假（改本件不影响本门对被扫件的读数）。
//
// 未覆盖面（如实列 · ✗ 不声称逐字）：
//
//	① `ZERG_BIN` 环境旋钮：py 用它选正门启动器；进程内**没有「另一个正门」可换** ⇒ 本入口不读它
//	   （该面只在「薄壳 exec 外部启动器」那一态下存在）。
//	② py `FileNotFoundError` 支（正门启动器不在 PATH）：进程内**恒不触发**（我们自己就是正门）；
//	   本入口对「临时件建不出/写失败」按 fail-closed 口径具名退 8（py 那一路是裸回溯 rc=1）。
//	③ argparse 极角落形态（短选项聚簇 `-hx` · 负号数值当值 …）只复刻被扫常见档。

const rdcProg = "repo-diffstat-check.py"

// rdcUsage —— py argparse 的用法串（prog 名 = `os.path.basename(sys.argv[0])` · 逐字）。
const rdcUsage = "usage: repo-diffstat-check.py [-h] [--file FILE]\n"

// rdcHelpText —— py argparse（py3.9 · 本机 PATH `python3` = 3.9.6）`-h/--help` 的**逐字**正文。
// ★ `optional arguments:` 这一小节标题是 **py<3.10** 形态（3.10+ 印 `options:` ⇒ 属解释器版本面）。
const rdcHelpText = rdcUsage +
	"\n" +
	"报数三格机检（**薄壳** → 正门 `zerg repo diffstat` · 只读 · 缺格即判不了）\n" +
	"\n" +
	"optional arguments:\n" +
	"  -h, --help   show this help message and exit\n" +
	"  --file FILE  diffstat 文本件（缺省读 stdin）\n"

// rdcLongOpts —— argparse 已注册的长选项串（按注册序 · 用于前缀缩写匹配）。
var rdcLongOpts = []string{"--help", "--file"}

// rdcArgErr —— 用法错（py argparse）：stderr 印 usage ＋ `prog: error: …`，退码 2。
func rdcArgErr(stderr io.Writer, msg string) int {
	fmt.Fprint(stderr, rdcUsage)
	fmt.Fprintf(stderr, "%s: error: %s\n", rdcProg, msg)
	return 2
}

// repoDiffstatGateMain —— 判据件 `scripts/gates/repo-diffstat-check.py` 的进程内入口
// （argv **不含程序名** · inproc 口径 · 照 M2 固定签名）。
func repoDiffstatGateMain(stdout, stderr io.Writer, argv []string) int {
	fileVal := ""
	var extras []string
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		if a == "--" {
			// py argparse：`--` 与其后全部**原样**进 extras（无位置参数 ⇒ 报「unrecognized arguments」）。
			extras = append(extras, argv[i:]...)
			break
		}
		if a == "-h" {
			fmt.Fprint(stdout, rdcHelpText)
			return 0
		}
		if a == "-" || !strings.HasPrefix(a, "-") {
			extras = append(extras, a)
			continue
		}
		if !strings.HasPrefix(a, "--") {
			extras = append(extras, a) // 非 -h 的短选项（py argparse 不认 ⇒ unrecognized）
			continue
		}
		name, explicit, hasExplicit := a, "", false
		if idx := strings.Index(a, "="); idx >= 0 {
			name, explicit, hasExplicit = a[:idx], a[idx+1:], true
		}
		var matches []string
		for _, o := range rdcLongOpts {
			if strings.HasPrefix(o, name) {
				matches = append(matches, o)
			}
		}
		if len(matches) == 0 {
			extras = append(extras, a)
			continue
		}
		if len(matches) > 1 {
			return rdcArgErr(stderr, "ambiguous option: "+name+" could match "+strings.Join(matches, ", "))
		}
		switch matches[0] {
		case "--help":
			if hasExplicit {
				return rdcArgErr(stderr, "argument -h/--help: ignored explicit argument '"+explicit+"'")
			}
			fmt.Fprint(stdout, rdcHelpText)
			return 0
		case "--file":
			if hasExplicit {
				fileVal = explicit
			} else if i+1 < len(argv) && (argv[i+1] == "-" || !strings.HasPrefix(argv[i+1], "-")) {
				i++
				fileVal = argv[i]
			} else {
				return rdcArgErr(stderr, "argument --file: expected one argument")
			}
		}
	}
	if len(extras) > 0 {
		return rdcArgErr(stderr, "unrecognized arguments: "+strings.Join(extras, " "))
	}

	// 判定面：py「`--file` 有值 ⇒ 用它；否则读 stdin 落临时件」+「交给正门 `--report <target>`」
	// ⇒ 此处**进程内**同构。★ py 判的是 `if a.file:`（假值即走 stdin）⇒ 空串同「没给」。
	target := fileVal
	if target == "" {
		text, _ := io.ReadAll(os.Stdin)
		f, err := os.CreateTemp("", "repo-diffstat-*.txt")
		if err != nil {
			fmt.Fprintf(stderr, "%s: 临时件建不出（%v）⇒ 判不了（退码 %d）\n", progName, err, exitBlocked)
			return exitBlocked
		}
		tmp := f.Name()
		defer func() { _ = os.Remove(tmp) }() // 自回收（py `finally: os.unlink(tmp)` 同构）
		if _, werr := f.Write(text); werr != nil {
			_ = f.Close()
			fmt.Fprintf(stderr, "%s: 临时件写失败（%v）⇒ 判不了（退码 %d）\n", progName, werr, exitBlocked)
			return exitBlocked
		}
		_ = f.Close()
		target = tmp
	}

	// 正门调用面：argv 形态照 CLI `repo diffstat --report <件>` 逐字（`inv.orig` 供写旗标守卫 · `inv.path` 供报错）。
	inv, err := parseInvocation([]string{"repo", "diffstat", "--report", target})
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", progName, err)
		return exitUsage
	}
	return cmdRepoDiffstat(inv, stdout, stderr)
}
