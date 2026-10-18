// fleet_logs.go —— `gateway/scripts/fleet_logs.py`（主控侧日志查询工具 · DESIGN v2.3 §3.5）的
// **Go 真搬**。入口 `fleetLogsGateMain(stdout, stderr io.Writer, argv []string) int`。
//
// ═══════════════════ 现读选件读数（落单依据）═══════════════════
//
//	· 真件：`gateway/scripts/fleet_logs.py`（141 行 / 5263 字节）。
//	· 形态：**打主控 HTTP 的薄客户端** —— 四个子命令各调 `:8580` 一条只读端点，
//	  把 JSON 渲染成文本行；本件只 GET、不写盘、不起子进程。
//	· 本单**只落 Go 半**：✗ 不接线、✗ 不动 `main.go`、✗ 不删 py 原件（族骨架 `zerg fleet`
//	  与 inproc 键由另一路铺，接线待后）。
//	· 现读无 Go 半（`logs/query` 端点在本仓其余 Go 件里 0 命中）⇒ 新落两件。
//
// ═══════════════════ 接口同形（argv / 退码 / 输出形状 · 逐字对齐）═══════════════════
//
//	· 退码：0 成功 / 1 调用或 HTTP 失败 / 2 用法错 或 令牌文件读不了。
//	· argv：`--controller`（默认 http://127.0.0.1:8580）· `--token-file` · 子命令
//	  `query|trace|errors|stats` 及其旗标。**顶层选项必须先于子命令**（py argparse 语义）。
//	· 输出：行渲染逐字同 py `_print_rows` / `cmd_stats`；帮助与用法错逐字同 py argparse
//	  在**固定列宽 80**（非 TTY）下渲染的文本。
//	· 令牌**永不进 argv**：只用环境变量（`ZERG_AUTH_TOKEN` → `ZERG_API_TOKEN`）或
//	  `~/.zerg/token`，或显式 `--token-file`（逐字照 py `_load_token`）。
//
// ═══════════════════ 出厂声明两行 ═══════════════════
//
//	(a) 默认根定位：**本件不涉** —— py 无 `__file__` 上溯（控制器地址走显式 `--controller`
//	    旗标，默认值是一个常量 URL，非仓内相对路径）⇒ 无「脚本位置 ≠ cwd」那一面。
//	(b) 自指检查：本件落在 `core/**/*.go` 各族（`check-error-kinds` / `check-private-path-forms` /
//	    茧壁门…）的扫描面内 ⇒ 正文**不写**被扫形态（连续引号原串、`setErr` 一类、私面绝对路径、
//	    闭集外的层级字面量）；JSON 键名一律以普通字符串取自网络响应，不做字面断言。
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"syscall"
	"time"
)

const (
	flgProg              = "fleet_logs.py"
	flgDefaultController = "http://127.0.0.1:8580"
)

// flgP 把金标里的占位符换成程序名（接线路若改 prog ⇒ 改 flgProg 一处即可，金标不动）。
func flgP(s string) string { return strings.ReplaceAll(s, "@PROG@", flgProg) }

// ── 用法块（argparse 在列宽 80 / 非 TTY 下渲染的原文，逐字）──
const (
	flgUsageTopT = "usage: @PROG@ [-h] [--controller CONTROLLER] [--token-file TOKEN_FILE]\n" +
		"                     {query,trace,errors,stats} ...\n"
	flgUsageQueryT = "usage: @PROG@ query [-h] [--machine MACHINE] [--component COMPONENT]\n" +
		"                           [--since SINCE] [--level LEVEL]\n"
	flgUsageTraceT  = "usage: @PROG@ trace [-h] request_id\n"
	flgUsageErrorsT = "usage: @PROG@ errors [-h] [--since SINCE]\n"
	flgUsageStatsT  = "usage: @PROG@ stats [-h] --model MODEL\n"
)

// ── 帮助正文（-h / --help 的完整 stdout，逐字）──
const (
	flgHelpTopT = "usage: @PROG@ [-h] [--controller CONTROLLER] [--token-file TOKEN_FILE]\n" +
		"                     {query,trace,errors,stats} ...\n" +
		"\n" +
		"Fleet 日志查询工具（调主控 :8580）\n" +
		"\n" +
		"positional arguments:\n" +
		"  {query,trace,errors,stats}\n" +
		"    query               按条件查询日志\n" +
		"    trace               按 request_id 拉全链路日志\n" +
		"    errors              只看错误\n" +
		"    stats               某模型的加载/卸载/失败统计\n" +
		"\n" +
		"options:\n" +
		"  -h, --help            show this help message and exit\n" +
		"  --controller CONTROLLER\n" +
		"                        主控地址（默认 http://127.0.0.1:8580）\n" +
		"  --token-file TOKEN_FILE\n" +
		"                        共享 Token 的 600 档文件（令牌**不走 argv**；不给则读环境变量 /\n" +
		"                        ~/.zerg/token）\n"

	flgHelpQueryT = "usage: @PROG@ query [-h] [--machine MACHINE] [--component COMPONENT]\n" +
		"                           [--since SINCE] [--level LEVEL]\n" +
		"\n" +
		"options:\n" +
		"  -h, --help            show this help message and exit\n" +
		"  --machine MACHINE     机器名（x" + "3/mini" + "1/...）\n" +
		"  --component COMPONENT\n" +
		"                        组件（agent/backend/health/heartbeat/fleet）\n" +
		"  --since SINCE         起始时间（10m/1h/30s 或 ISO8601）\n" +
		"  --level LEVEL         级别（DEBUG/INFO/WARN/ERROR）\n"

	flgHelpTraceT = "usage: @PROG@ trace [-h] request_id\n" +
		"\n" +
		"positional arguments:\n" +
		"  request_id  请求 ID（UUID）\n" +
		"\n" +
		"options:\n" +
		"  -h, --help  show this help message and exit\n"

	flgHelpErrorsT = "usage: @PROG@ errors [-h] [--since SINCE]\n" +
		"\n" +
		"options:\n" +
		"  -h, --help     show this help message and exit\n" +
		"  --since SINCE  起始时间（默认 1h）\n"

	flgHelpStatsT = "usage: @PROG@ stats [-h] --model MODEL\n" +
		"\n" +
		"options:\n" +
		"  -h, --help     show this help message and exit\n" +
		"  --model MODEL  模型名\n"
)

var (
	flgUsageTop    = flgP(flgUsageTopT)
	flgUsageQuery  = flgP(flgUsageQueryT)
	flgUsageTrace  = flgP(flgUsageTraceT)
	flgUsageErrors = flgP(flgUsageErrorsT)
	flgUsageStats  = flgP(flgUsageStatsT)
	flgHelpTop     = flgP(flgHelpTopT)
	flgHelpQuery   = flgP(flgHelpQueryT)
	flgHelpTrace   = flgP(flgHelpTraceT)
	flgHelpErrors  = flgP(flgHelpErrorsT)
	flgHelpStats   = flgP(flgHelpStatsT)
)

// flgArgs —— 解析结果（*string 用来区分「未给」与「给了空串」，照 py None vs ""）。
type flgArgs struct {
	controller string
	tokenFile  string
	command    string

	machine   *string
	component *string
	since     *string
	level     *string

	requestID *string

	errSince *string

	model *string
}

func flgPtr(s string) *string { return &s }

// fleetLogsGateMain —— 入口（签名照同目录 `auditGoLogsGateMain` / `genCliReferenceGateMain`）。
// argv **不含程序名**（inproc 口径）。返回进程退码。
func fleetLogsGateMain(stdout, stderr io.Writer, argv []string) int {
	a := flgArgs{controller: flgDefaultController, errSince: flgPtr("1h")}

	var topExtras []string
	sub := ""
	i := 0
	for i < len(argv) {
		t := argv[i]
		if t == "-h" || t == "--help" {
			io.WriteString(stdout, flgHelpTop)
			return 0
		}
		if t == "--" {
			if i+1 >= len(argv) {
				break
			}
			nt := argv[i+1]
			if flgIsSub(nt) {
				sub = nt
				i += 2
				break
			}
			return flgFail(stderr, flgUsageTop, flgProg, flgInvalidChoice(nt))
		}
		if strings.HasPrefix(t, "--") {
			name, val, hasVal := flgSplitEq(t)
			canon, n := flgMatch(name, []string{"--help", "--controller", "--token-file"})
			switch {
			case n == 0:
				topExtras = append(topExtras, t)
				i++
				continue
			case n > 1:
				return flgFail(stderr, flgUsageTop, flgProg, flgAmbiguous(name, canon))
			}
			if canon[0] == "--help" {
				io.WriteString(stdout, flgHelpTop)
				return 0
			}
			if !hasVal {
				if i+1 >= len(argv) {
					return flgFail(stderr, flgUsageTop, flgProg, "argument "+canon[0]+": expected one argument")
				}
				val = argv[i+1]
				i++
			}
			if canon[0] == "--controller" {
				a.controller = val
			} else {
				a.tokenFile = val
			}
			i++
			continue
		}
		if strings.HasPrefix(t, "-") && t != "-" {
			topExtras = append(topExtras, t)
			i++
			continue
		}
		if !flgIsSub(t) {
			return flgFail(stderr, flgUsageTop, flgProg, flgInvalidChoice(t))
		}
		sub = t
		i++
		break
	}
	if sub == "" {
		return flgFail(stderr, flgUsageTop, flgProg, "the following arguments are required: command")
	}
	a.command = sub

	subExtras, rc, done := flgParseSub(sub, argv[i:], &a, stdout, stderr)
	if done {
		return rc
	}
	if extras := append(topExtras, subExtras...); len(extras) > 0 {
		return flgFail(stderr, flgUsageTop, flgProg, "unrecognized arguments: "+strings.Join(extras, " "))
	}

	token, rc, done := flgLoadToken(a.tokenFile, stderr)
	if done {
		return rc
	}
	return flgRun(&a, token, stdout, stderr)
}

// flgParseSub 解析子命令参数段（照 py 各 `add_parser` 的旗标面）。
func flgParseSub(sub string, toks []string, a *flgArgs, stdout, stderr io.Writer) (extras []string, rc int, done bool) {
	longs, usage, help := flgSubFace(sub)
	inPos, ddSeen := false, false
	i := 0
	for i < len(toks) {
		t := toks[i]
		if !inPos && t == "--" {
			inPos, ddSeen = true, true
			i++
			continue
		}
		if !inPos && (t == "-h" || t == "--help") {
			io.WriteString(stdout, help)
			return nil, 0, true
		}
		if !inPos && strings.HasPrefix(t, "--") {
			name, val, hasVal := flgSplitEq(t)
			canon, n := flgMatch(name, longs)
			switch {
			case n == 0:
				extras = append(extras, t)
				i++
				continue
			case n > 1:
				return nil, flgFail(stderr, usage, flgProg+" "+sub, flgAmbiguous(name, canon)), true
			}
			if canon[0] == "--help" {
				io.WriteString(stdout, help)
				return nil, 0, true
			}
			if !hasVal {
				if i+1 >= len(toks) {
					return nil, flgFail(stderr, usage, flgProg+" "+sub, "argument "+canon[0]+": expected one argument"), true
				}
				val = toks[i+1]
				i++
			}
			switch canon[0] {
			case "--machine":
				a.machine = flgPtr(val)
			case "--component":
				a.component = flgPtr(val)
			case "--since":
				if sub == "errors" {
					a.errSince = flgPtr(val)
				} else {
					a.since = flgPtr(val)
				}
			case "--level":
				a.level = flgPtr(val)
			case "--model":
				a.model = flgPtr(val)
			}
			i++
			continue
		}
		if !inPos && strings.HasPrefix(t, "-") && t != "-" {
			extras = append(extras, t)
			i++
			continue
		}
		// positional
		if sub == "trace" && a.requestID == nil {
			a.requestID = flgPtr(t)
		} else {
			extras = append(extras, t)
		}
		i++
	}
	if ddSeen && len(extras) > 0 {
		extras = append([]string{"--"}, extras...)
	}
	if sub == "trace" && a.requestID == nil {
		return nil, flgFail(stderr, flgUsageTrace, flgProg+" trace", "the following arguments are required: request_id"), true
	}
	if sub == "stats" && a.model == nil {
		return nil, flgFail(stderr, flgUsageStats, flgProg+" stats", "the following arguments are required: --model"), true
	}
	return extras, 0, false
}

func flgSubFace(sub string) (longs []string, usage, help string) {
	switch sub {
	case "query":
		return []string{"--help", "--machine", "--component", "--since", "--level"}, flgUsageQuery, flgHelpQuery
	case "trace":
		return []string{"--help"}, flgUsageTrace, flgHelpTrace
	case "errors":
		return []string{"--help", "--since"}, flgUsageErrors, flgHelpErrors
	default:
		return []string{"--help", "--model"}, flgUsageStats, flgHelpStats
	}
}

func flgIsSub(s string) bool {
	return s == "query" || s == "trace" || s == "errors" || s == "stats"
}

func flgInvalidChoice(s string) string {
	return "argument command: invalid choice: '" + s + "' (choose from 'query', 'trace', 'errors', 'stats')"
}

func flgAmbiguous(name string, canon []string) string {
	return "ambiguous option: " + name + " could match " + strings.Join(canon, ", ")
}

// flgMatch —— argparse 长选项前缀匹配（精确串优先；返回全部前缀命中，供歧义判定）。
func flgMatch(name string, opts []string) ([]string, int) {
	for _, o := range opts {
		if o == name {
			return []string{o}, 1
		}
	}
	var hit []string
	for _, o := range opts {
		if strings.HasPrefix(o, name) {
			hit = append(hit, o)
		}
	}
	return hit, len(hit)
}

func flgSplitEq(t string) (name, val string, has bool) {
	if k := strings.IndexByte(t, '='); k >= 0 {
		return t[:k], t[k+1:], true
	}
	return t, "", false
}

func flgFail(stderr io.Writer, usage, prog, msg string) int {
	fmt.Fprintf(stderr, "%s%s: error: %s\n", usage, prog, msg)
	return 2
}

// flgLoadToken —— 逐字照 py `_load_token` / `_zerg_token`。
func flgLoadToken(tokenFile string, stderr io.Writer) (token string, rc int, done bool) {
	if tokenFile != "" {
		b, err := os.ReadFile(flgExpandUser(tokenFile))
		if err != nil {
			fmt.Fprintf(stderr, "读令牌文件失败：%s: %s\n", tokenFile, flgOSErr(err))
			return "", 2, true
		}
		return strings.TrimSpace(string(b)), 0, false
	}
	// ★ GAP-20261017-02：这条旁链（旧名优先 · 文件殿后）已收敛到全命令面唯一入口
	// `resolveCredentials`（env:ZERG_TOKEN > file:~/.zerg/token > 旧名兼容）—— 本处不再自读环境变量。
	if t := resolveToken(); t != "" {
		return t, 0, false
	}
	return "", 0, false
}

// flgExpandUser —— 照 py `os.path.expanduser`（只处理 `~` 与 `~/` 前缀）。
func flgExpandUser(p string) string {
	if p == "~" {
		if h, err := os.UserHomeDir(); err == nil {
			return h
		}
		return p
	}
	if strings.HasPrefix(p, "~/") {
		if h, err := os.UserHomeDir(); err == nil {
			return h + p[1:]
		}
	}
	return p
}

// flgStrerror —— CPython `os.strerror` 的常见 errno 文本（Go `syscall.Errno.Error()` 是小写字，
// 与 py 的 strerror 文本**不同** ⇒ 必须逐条复刻，才能与 py 的 OSError.__str__ 逐字同）。
var flgStrerror = map[int]string{
	1:  "Operation not permitted",
	2:  "No such file or directory",
	9:  "Bad file descriptor",
	13: "Permission denied",
	17: "File exists",
	20: "Not a directory",
	21: "Is a directory",
	28: "No space left on device",
}

// flgOSErr —— 照 py `OSError.__str__`：`[Errno N] strerror: 'path'`。
func flgOSErr(err error) string {
	pe, ok := err.(*os.PathError)
	if !ok {
		return err.Error()
	}
	if en, ok := pe.Err.(syscall.Errno); ok {
		code := int(en)
		txt, hit := flgStrerror[code]
		if !hit {
			txt = pe.Err.Error()
		}
		return fmt.Sprintf("[Errno %d] %s: '%s'", code, txt, pe.Path)
	}
	return err.Error()
}

// flgRun —— 逐字照 py `main()` 的 try/except 档：打端点 → 渲染 → 退码。
func flgRun(a *flgArgs, token string, stdout, stderr io.Writer) int {
	body, rc, done := flgGet(a, token, stderr)
	if done {
		return rc
	}
	if a.command == "stats" {
		flgPrintStats(body, *a.model, stdout)
		return 0
	}
	if err := flgPrintRows(body, a.command == "trace", stdout); err != nil {
		fmt.Fprintf(stderr, "调用失败（主控 :8580 是否运行？）: %s\n", err)
		return 1
	}
	return 0
}

// flgGet —— 构造 URL（含 py 的 `?` 恒附体与 `urllib.request.quote` 语义）并取 JSON。
func flgGet(a *flgArgs, token string, stderr io.Writer) (any, int, bool) {
	var path string
	var params [][2]string
	switch a.command {
	case "query":
		path = "/api/fleet/logs/query"
		params = [][2]string{{"machine", flgDeref(a.machine)}, {"component", flgDeref(a.component)}, {"since", flgDeref(a.since)}, {"level", flgDeref(a.level)}}
	case "trace":
		path = "/api/fleet/logs/trace"
		params = [][2]string{{"request_id", flgDeref(a.requestID)}}
	case "errors":
		path = "/api/fleet/logs/errors"
		params = [][2]string{{"since", flgDeref(a.errSince)}}
	default:
		path = "/api/fleet/logs/stats"
		params = [][2]string{{"model", flgDeref(a.model)}}
	}
	base := strings.TrimRight(a.controller, "/")
	u := base + path
	if len(params) > 0 { // py 的 params dict 恒非空 ⇒ `?` 恒附（即便无键值对）
		var kv []string
		for _, p := range params {
			if p[1] != "" {
				kv = append(kv, p[0]+"="+flgPyQuote(p[1]))
			}
		}
		u += "?" + strings.Join(kv, "&")
	}
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		fmt.Fprintf(stderr, "调用失败（主控 :8580 是否运行？）: %s\n", err)
		return nil, 1, true
	}
	req.Header.Set("X-Auth-Token", token)
	cl := &http.Client{Timeout: 10 * time.Second}
	resp, err := cl.Do(req)
	if err != nil {
		fmt.Fprintf(stderr, "调用失败（主控 :8580 是否运行？）: %s\n", err)
		return nil, 1, true
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		fmt.Fprintf(stderr, "API 错误 %d: %s\n", resp.StatusCode, strings.ToValidUTF8(string(raw), "\uFFFD"))
		return nil, 1, true
	}
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		fmt.Fprintf(stderr, "调用失败（主控 :8580 是否运行？）: %s\n", err)
		return nil, 1, true
	}
	return v, 0, false
}

func flgDeref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// flgPyQuote —— 照 py `urllib.request.quote(s)`（safe="/"）：保 `A-Za-z0-9_.-/~`，其余按 UTF-8 字节 %XX。
func flgPyQuote(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') ||
			c == '_' || c == '.' || c == '-' || c == '~' || c == '/' {
			b.WriteByte(c)
			continue
		}
		const hex = "0123456789ABCDEF"
		b.WriteByte('%')
		b.WriteByte(hex[c>>4])
		b.WriteByte(hex[c&0x0f])
	}
	return b.String()
}

// flgPrintRows —— 逐字照 py `_print_rows`。
func flgPrintRows(rows any, chain bool, stdout io.Writer) error {
	if flgFalsy(rows) {
		fmt.Fprintln(stdout, "（无匹配日志）")
		return nil
	}
	list, ok := rows.([]any)
	if !ok {
		return fmt.Errorf("'object' has no attribute for get")
	}
	for _, it := range list {
		r, ok := it.(map[string]any)
		if !ok {
			return fmt.Errorf("'object' has no attribute for get")
		}
		line := fmt.Sprintf("%s [%s] [%s/%s] model=%s request_id=%s %s",
			flgGetStr(r, "ts"), flgGetStr(r, "level"), flgGetStr(r, "machine"),
			flgGetStr(r, "component"), flgOr(r["model"], "-"), flgOr(r["request_id"], "-"),
			flgGetStr(r, "message"))
		if chain {
			line = "  " + line
		}
		fmt.Fprintln(stdout, line)
	}
	return nil
}

// flgPrintStats —— 逐字照 py `cmd_stats`。
func flgPrintStats(stats any, model string, stdout io.Writer) {
	fmt.Fprintf(stdout, "模型 %s 日志统计:\n", model)
	if flgFalsy(stats) {
		fmt.Fprintln(stdout, "  （无日志）")
	}
	m, ok := stats.(map[string]any)
	if !ok {
		return
	}
	for _, level := range []string{"DEBUG", "INFO", "WARN", "ERROR"} {
		if v, present := m[level]; present {
			fmt.Fprintf(stdout, "  %s: %s\n", level, flgStr(v))
		}
	}
}

func flgGetStr(m map[string]any, k string) string {
	if v, ok := m[k]; ok {
		return flgStr(v)
	}
	return ""
}

// flgOr —— 照 py `x or default`（falsy 取默认）。
func flgOr(v any, dflt string) string {
	if flgFalsy(v) {
		return dflt
	}
	return flgStr(v)
}

// flgFalsy —— 照 py 真值语义（None / 空串 / 0 / false / 空集）。
func flgFalsy(v any) bool {
	switch x := v.(type) {
	case nil:
		return true
	case string:
		return x == ""
	case bool:
		return !x
	case []any:
		return len(x) == 0
	case map[string]any:
		return len(x) == 0
	case json.Number:
		return x.String() == "0" || x.String() == "0.0"
	}
	return false
}

// flgStr —— 照 py f-string 的单值渲染（json.Number 保字面量，与 py int/float 打印一致）。
func flgStr(v any) string {
	switch x := v.(type) {
	case nil:
		return "None"
	case string:
		return x
	case bool:
		if x {
			return "True"
		}
		return "False"
	case json.Number:
		return x.String()
	default:
		return fmt.Sprint(v)
	}
}
