// eval_ab_campaigns.go —— 件 scripts/evals/ab-campaigns.py（155 行 · sha16 e305d68ffbb5bd67）的 Go 真搬
// （评测线 ② 内部实现件 · 入口 evalABCampaignsGateMain）。
//
// 为什么有这件 / 它到底干什么（逐处现读 scripts/evals/ab-campaigns.py · 非按件名推断）：
//
//	· 形态 = 实验编排器：对本机主控 HTTP API（env `ZERG_CORE`，缺省 `http://127.0.0.1:8580`）
//	  打一串 POST/GET，把「A/B 两个课题」的读数落成 JSON 档，并在 stdout 打进度与小节。
//	· 无 argparse：py 的 `main()` 不接参数、**整个 `sys.argv` 一字不读**（`if __name__=="__main__": main()`）
//	  ⇒ 任何旗标（`--help` / 未知参 / 位置参）一律**被忽略**、退码恒 0。故本件也**一字不读 argv**。
//	· 输入面：env `ZERG_CORE`（主控基址 · 原样拼接、不 strip）· env `ZERG_AB_RUNS`（每档次数 · 缺省 `"3"`）·
//	  `~/.zerg/token`（X-Auth-Token 真源 · import 期即读 ⇒ 缺它 py 未捕 ⇒ 回溯 rc=1）·
//	  `~/.zerg/state/chat_obs.jsonl`（turn/tool/queued 观测面的读取源）。
//	· 输出面：stdout（进度行 · 逐次读数行 · 「⇒ 小结」· 落档行 · 「总览」）；
//	  `~/.zerg/egg-profiles/ab-campaigns.json`（`json.dump(..., ensure_ascii=False, indent=2)` · **无尾换行**）。
//	· 退码：正常 0；import 期异常（token 读不到 / `ZERG_AB_RUNS` 非十进制整数）与收尾写档异常
//	  ⇒ py 顶层是裸 `main()`（无 `sys.exit` 兜底）⇒ **rc=1**（不是「不给结论 rc=2」）。逐次 run 内的异常
//	  被 py `try/except` 吞成一行「✗ 失败：…」并**继续下一档**（不影响最终 rc）。
//
// 主控客户端写法：照现役先例 `core/internal/agent/searx_client.go` —— 显式
// `&http.Client{Timeout: …}` + `http.NewRequestWithContext` + 认证头 `X-Auth-Token`；
// **协议（端点 / 方法 / 头 / 请求体）逐字照 py · ✗ 不自造、✗ 不改口径**（唯一例外：py 的
// `timeout=` 是 urlopen 的 socket 超时，本件按同一数值设 http.Client.Timeout，逐字对齐 1200s/60s；
// 且 `urlopen` 对 4xx/5xx **抛 HTTPError** ⇒ 本件同档返回错误）。
//
// 出厂声明 (a) 默认根定位差异：py 只用 `os.path.expanduser("~…")`（=`$HOME`）与 env，**不吃仓外树、
//
//	不按脚本位置上溯** ⇒ 「脚本位置 ≠ cwd」那类差异面**不适用**，如实声明。唯一细差：`HOME` 未设时
//	py 的 expanduser 回落 pwd 查表、Go `os.UserHomeDir()` 直接报错 ⇒ 本件 **fail-closed rc=1 并出声**。
//
// 出厂声明 (b) 自指检查：本件落在 `core/cmd/zerg/*.go` 扫描面内 ⇒ 逐条消解：件内**零仓外私有绝对
//
//	路径字面量**（只有 HOME 拼出的 `~` 相对段；✗ 不写任何私有根前缀 —— 连「我没有写它」这种消解声明
//	也按描述式写、✗ 不把该前缀本身当例子列出来）；命令树层级零字面量（**不进命令树** ——
//	登记 = 另一路独占，本单只落码）；件头一律**描述式**、不写连续协议原串（JSON 键名走代码位字符串，
//	件头只以「kind 字段」这种描述式提它 —— 规避 `check-error-kinds` R3 的「注释位连续原串」误伤）。
//
// 本单边界（照实 · 与同族 `eval_compare.go` / `eval_stability_stats.go` 同款）：
//
//	· 只落码、**不接线** —— 不在 `main.go` 注册命令、不改 `family_eval.go`（该件此刻正被另一路在改：
//	  `git status` 现读 `M`、mtime 与本次会话同刻）⇒ 入口 `evalABCampaignsGateMain` 暂**无调用点**（M2 的刻意状态）；
//	· **不删** `scripts/evals/ab-campaigns.py` 原件（退场：对拍通过後才逐件点名退场，本单不退役）；
//	· ✗ 不改任何登记面 / `family_*.go` / 已落旧件。
//
// 入口签名（与 py 的 `main()` 对齐）：
//
//	func evalABCampaignsGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv —— **一字不读**（py 同款：`main()` 不接参数）⇒ 无参 / `--help` / 未知参 **三档行为逐字同**
//	  · 退码 —— 同 py：0 正常 / 1 import 期或收尾写档异常
//	  · 输出 —— stdout + 落档件与 py 逐字对齐（非确定面 `run_at` / `wall_s` 见「未覆盖面」）
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	// eabcDefaultCore —— py `CORE = os.environ.get("ZERG_CORE", "http://127.0.0.1:8580")` 的缺省（逐字）。
	eabcDefaultCore = "http://127.0.0.1:8580"
	// eabcEnvCore —— 主控基址旋钮名（照 py 同名 env · ✗ 不另起名）。
	eabcEnvCore = "ZERG_CORE"
	// eabcEnvRuns —— 每档次数旋钮名（照 py 同名 env）。
	eabcEnvRuns = "ZERG_AB_RUNS"
	// eabcDefaultRuns —— py `RUNS = int(os.environ.get("ZERG_AB_RUNS", "3"))` 的缺省串（逐字）。
	eabcDefaultRuns = "3"
	// eabcTargetRel —— py `TARGET`（无害靶子 · 只读件 · 仓内相对路径 · 仅作任务文案里的字面）。
	eabcTargetRel = "core/internal/version/version.go"
	// eabcDefaultModel —— py `run_once(..., model="Qwen3.8-27B")` 的缺省（逐字）。
	eabcDefaultModel = "Qwen3.8-27B"
	// eabcPostTimeout / eabcGetTimeout —— py `post(timeout=1200)` / `get(timeout=60)`（逐字）。
	eabcPostTimeout = 1200 * time.Second
	eabcGetTimeout  = 60 * time.Second
)

// eabcTask —— py `TASK_A` / `TASK_B`（两串**逐字相同** ⇒ 收成一枚，✗ 不各写一份）。
func eabcTask() string {
	return "请读 " + eabcTargetRel + "，把其中 version 常量那一行**原文**贴出来，" +
		"并附上你读取它时那次工具调用的回执ID。只做这一步。"
}

// --------------------------------------------------------------------------- //
// py 原语同构（空白 / 行界 / 真假值 / 数值 / JSON 渲染）
// --------------------------------------------------------------------------- //

// eabcPyIsSpace —— py `str.isspace` 集（★ 含 \x1c-\x1f、\x85、\xa0；Go unicode.IsSpace 不认前三者）。
func eabcPyIsSpace(r rune) bool {
	switch {
	case r == ' ' || (r >= '\t' && r <= '\r'):
		return true
	case r >= 0x1c && r <= 0x1f:
		return true
	case r == 0x85 || r == 0xa0:
		return true
	case r == 0x1680 || (r >= 0x2000 && r <= 0x200a):
		return true
	case r == 0x2028 || r == 0x2029 || r == 0x202f || r == 0x205f || r == 0x3000:
		return true
	}
	return false
}

// eabcPyStrip —— py `str.strip()`（剥 py 的 Unicode 空白集）。
func eabcPyStrip(s string) string { return strings.TrimFunc(s, eabcPyIsSpace) }

// eabcPyTruthy —— py 真假值（None/False/0/0.0/""/[]/{} 皆假）。
func eabcPyTruthy(v interface{}) bool {
	switch t := v.(type) {
	case nil:
		return false
	case bool:
		return t
	case string:
		return t != ""
	case float64:
		return t != 0
	case int64:
		return t != 0
	case []interface{}:
		return len(t) > 0
	case map[string]interface{}:
		return len(t) > 0
	case *eabcObj:
		return len(t.keys) > 0
	}
	return true
}

// eabcPyStr —— py `str(v)`（进 URL / 文案面用）。
func eabcPyStr(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return "None"
	case bool:
		if t {
			return "True"
		}
		return "False"
	case string:
		return t
	case float64:
		if t == math.Trunc(t) && !math.IsInf(t, 0) {
			return strconv.FormatFloat(t, 'f', -1, 64)
		}
		return eabcPyFloat(t)
	case int64:
		return strconv.FormatInt(t, 10)
	}
	return fmt.Sprint(v)
}

// eabcPyStrOrEmpty —— py `(v or "")`：假值 ⇒ ""，否则 `str(v)`。
func eabcPyStrOrEmpty(v interface{}) string {
	if !eabcPyTruthy(v) {
		return ""
	}
	return eabcPyStr(v)
}

// eabcUniversalNewlines —— py 文本模式 `open()` 的通用换行（\r\n / 裸 \r 一律折成 \n）。
func eabcUniversalNewlines(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}

// eabcFileLines —— py `for line in f`：按 \n 切、**去尾 \n**，且**文件以 \n 收尾不额外产空尾项**。
func eabcFileLines(content string) []string {
	if content == "" {
		return nil
	}
	parts := strings.Split(content, "\n")
	if strings.HasSuffix(content, "\n") {
		parts = parts[:len(parts)-1]
	}
	return parts
}

// eabcPyRound —— py `round(x, nd)`：对**真二进制值**正确舍入到 nd 位小数（非 tie 档与 py 逐字同）。
func eabcPyRound(x float64, nd int) float64 {
	if math.IsNaN(x) || math.IsInf(x, 0) {
		return x
	}
	f, err := strconv.ParseFloat(strconv.FormatFloat(x, 'f', nd, 64), 64)
	if err != nil {
		return x
	}
	return f
}

// eabcPyFloat —— py `repr(float)`（最短往返表示；整数型浮点补 `.0`；Inf/NaN 走 py 字面）。
func eabcPyFloat(f float64) string {
	if math.IsInf(f, 1) {
		return "Infinity"
	}
	if math.IsInf(f, -1) {
		return "-Infinity"
	}
	if math.IsNaN(f) {
		return "NaN"
	}
	s := strconv.FormatFloat(f, 'g', -1, 64)
	if !strings.ContainsAny(s, ".eE") {
		s += ".0"
	}
	return s
}

// eabcJSONString —— py `json.dumps(..., ensure_ascii=False)` 的字符串转义（控制符转义、非 ASCII 原样留）。
func eabcJSONString(s string) string {
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
		case '\t':
			b.WriteString("\\t")
		case '\n':
			b.WriteString("\\n")
		case '\f':
			b.WriteString("\\f")
		case '\r':
			b.WriteString("\\r")
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

// eabcObj —— 保**插入序**的 JSON 对象（✗ 用 Go map ⇒ 迭代序随机 ⇒ 假失配）。
type eabcObj struct {
	keys []string
	vals []interface{}
}

func eabcNewObj() *eabcObj { return &eabcObj{} }

func (o *eabcObj) set(k string, v interface{}) *eabcObj {
	o.keys = append(o.keys, k)
	o.vals = append(o.vals, v)
	return o
}

func (o *eabcObj) get(k string) (interface{}, bool) {
	for i, kk := range o.keys {
		if kk == k {
			return o.vals[i], true
		}
	}
	return nil, false
}

// eabcEmitJSON —— 复刻 py json：非缩进档分隔 `, ` / `: `；缩进档分隔 `,` / `: ` + 逐层两空格缩进。
func eabcEmitJSON(v interface{}, indent string, pretty bool, b *strings.Builder) {
	sep, keySep := ", ", ": "
	if pretty {
		sep = ","
	}
	switch t := v.(type) {
	case *eabcObj:
		if len(t.keys) == 0 {
			b.WriteString("{}")
			return
		}
		if pretty {
			b.WriteString("{\n")
		} else {
			b.WriteString("{")
		}
		for i, k := range t.keys {
			if pretty {
				b.WriteString(indent + "  ")
			}
			b.WriteString(eabcJSONString(k))
			b.WriteString(keySep)
			eabcEmitJSON(t.vals[i], indent+"  ", pretty, b)
			if i < len(t.keys)-1 {
				b.WriteString(sep)
			}
			if pretty {
				b.WriteString("\n")
			}
		}
		if pretty {
			b.WriteString(indent + "}")
		} else {
			b.WriteString("}")
		}
	case []interface{}:
		if len(t) == 0 {
			b.WriteString("[]")
			return
		}
		if pretty {
			b.WriteString("[\n")
		} else {
			b.WriteString("[")
		}
		for i, e := range t {
			if pretty {
				b.WriteString(indent + "  ")
			}
			eabcEmitJSON(e, indent+"  ", pretty, b)
			if i < len(t)-1 {
				b.WriteString(sep)
			}
			if pretty {
				b.WriteString("\n")
			}
		}
		if pretty {
			b.WriteString(indent + "]")
		} else {
			b.WriteString("]")
		}
	case string:
		b.WriteString(eabcJSONString(t))
	case bool:
		if t {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	case int:
		b.WriteString(strconv.Itoa(t))
	case int64:
		b.WriteString(strconv.FormatInt(t, 10))
	case float64:
		b.WriteString(eabcPyFloat(t))
	case nil:
		b.WriteString("null")
	default:
		b.WriteString("null")
	}
}

// eabcDumps —— `json.dumps(v, ensure_ascii=False)`（非缩进）。
func eabcDumps(v interface{}) string {
	var b strings.Builder
	eabcEmitJSON(v, "", false, &b)
	return b.String()
}

// eabcDumpIndent —— `json.dump(v, ensure_ascii=False, indent=2)`（缩进 2）。
func eabcDumpIndent(v interface{}) string {
	var b strings.Builder
	eabcEmitJSON(v, "", true, &b)
	return b.String()
}

// --------------------------------------------------------------------------- //
// 观测面（chat_obs.jsonl）
// --------------------------------------------------------------------------- //

// eabcObsEntry —— 一行观测：合法 JSON 对象入 obj；JSON 合法但**非对象**（py 里合法但后续 .get 会崩）标 bad。
type eabcObsEntry struct {
	obj map[string]interface{}
	bad bool
}

// eabcReadStateRaw —— 读状态件（通用换行已折）；缺件或读不了 ⇒ err（py `open` 会抛 ⇒ 整轮失败）。
func eabcReadStateRaw(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return eabcUniversalNewlines(string(b)), nil
}

// eabcObsSince —— py `obs_since(offset)`：返回「第 offset+1 条起的非空行」解析结果 + 非空行总数。
func eabcObsSince(path string, offset int) ([]eabcObsEntry, int, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, offset, nil // py `os.path.exists` 否 ⇒ ([], offset)
	}
	content, err := eabcReadStateRaw(path)
	if err != nil {
		return nil, offset, err
	}
	out := []eabcObsEntry{}
	seen := 0
	for _, line := range eabcFileLines(content) {
		line = eabcPyStrip(line)
		if line == "" {
			continue
		}
		seen++
		if seen > offset {
			var obj map[string]interface{}
			if err := json.Unmarshal([]byte(line), &obj); err == nil && obj != nil {
				out = append(out, eabcObsEntry{obj: obj})
			} else if err == nil {
				out = append(out, eabcObsEntry{bad: true})
			}
			// err != nil ⇒ py `json.loads` 抛 ⇒ 被内层 except 吞掉（跳过该行）
		}
	}
	return out, seen, nil
}

// eabcCountObs —— py `count_obs()`：状态件里非空行数（缺件 ⇒ 0；读不了 ⇒ err）。
func eabcCountObs(path string) (int, error) {
	if _, err := os.Stat(path); err != nil {
		return 0, nil
	}
	content, err := eabcReadStateRaw(path)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, line := range eabcFileLines(content) {
		if eabcPyStrip(line) != "" {
			n++
		}
	}
	return n, nil
}

// eabcPyCountLines —— py `sum(1 for _ in resp)`：按 readline 语义数行（尾无 \n 的残段仍算一行）。
func eabcPyCountLines(b []byte) int {
	if len(b) == 0 {
		return 0
	}
	s := string(b)
	n := strings.Count(s, "\n")
	if !strings.HasSuffix(s, "\n") {
		n++
	}
	return n
}

// --------------------------------------------------------------------------- //
// 主控 HTTP 客户端（照先例 searx_client.go 的写形态）
// --------------------------------------------------------------------------- //

type eabcClient struct {
	base     string
	tok      string
	postHTTP *http.Client
	getHTTP  *http.Client
}

func eabcNewClient(base, tok string) *eabcClient {
	return &eabcClient{
		base:     base,
		tok:      tok,
		postHTTP: &http.Client{Timeout: eabcPostTimeout},
		getHTTP:  &http.Client{Timeout: eabcGetTimeout},
	}
}

// eabcHTTPStatusError —— py `urlopen` 对 4xx/5xx 抛 HTTPError；本件同档返回错误（✗ 不静默读错体当正常）。
func eabcHTTPStatusError(resp *http.Response) error {
	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	}
	return nil
}

// eabcPost —— py `post(path, body)`：POST JSON（`json.dumps(body).encode()`）+ `Content-Type` + `X-Auth-Token`。
func (c *eabcClient) eabcPost(path string, body interface{}) (*http.Response, error) {
	ctx, cancel := context.WithTimeout(context.Background(), eabcPostTimeout)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+path, strings.NewReader(eabcDumps(body)))
	if err != nil {
		cancel()
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Auth-Token", c.tok)
	resp, err := c.postHTTP.Do(req)
	if err != nil {
		cancel()
		return nil, err
	}
	resp.Body = &eabcCancelCloser{rc: resp.Body, cancel: cancel}
	if err := eabcHTTPStatusError(resp); err != nil {
		resp.Body.Close()
		return nil, err
	}
	return resp, nil
}

type eabcCancelCloser struct {
	rc     io.ReadCloser
	cancel context.CancelFunc
}

func (c *eabcCancelCloser) Read(p []byte) (int, error) { return c.rc.Read(p) }
func (c *eabcCancelCloser) Close() error {
	err := c.rc.Close()
	c.cancel()
	return err
}

// eabcGetJSON —— py `get(path)`：GET ⇒ decode ⇒ json.loads；**transport / HTTP 错 ⇒ 返回错误**、
// **JSON 坏 ⇒ (nil, nil)**（py 的 try/except 只罩 json.loads）。
func (c *eabcClient) eabcGetJSON(path string) (interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), eabcGetTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Auth-Token", c.tok)
	resp, err := c.getHTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := eabcHTTPStatusError(resp); err != nil {
		return nil, err
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	// py `.read().decode()` 在 try 之外 ⇒ 非 UTF-8 会抛。Go 的 string(b) 恒成功，
	// 此面差异写进未覆盖面（真仓/token 面皆合法 UTF-8 ⇒ 不触发）。
	var v interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		return nil, nil // py 内层 except ⇒ None
	}
	return v, nil
}

// --------------------------------------------------------------------------- //
// 数值聚合（py int/float 两态）
// --------------------------------------------------------------------------- //

// eabcNum —— py `x or 0` 后的候选：整数源 ⇒ 输出 int；小数源 ⇒ 输出 float。
type eabcNum struct {
	f     float64
	isInt bool
}

func eabcNumOf(v interface{}) eabcNum {
	switch t := v.(type) {
	case int64:
		return eabcNum{float64(t), true}
	case int:
		return eabcNum{float64(t), true}
	case float64:
		if t == math.Trunc(t) && !math.IsInf(t, 0) {
			return eabcNum{t, true}
		}
		return eabcNum{t, false}
	}
	return eabcNum{0, true} // py `or 0`：缺 / None / False / 0 ⇒ 0（int）
}

// eabcMaxNums —— py `max([v or 0 …] or [0])`：空 ⇒ int 0；否则取最大者（保其 int/float 型别）。
func eabcMaxNums(xs []eabcNum) interface{} {
	if len(xs) == 0 {
		return int64(0)
	}
	best := xs[0]
	for _, x := range xs[1:] {
		if x.f > best.f {
			best = x
		}
	}
	if best.isInt {
		return int64(best.f)
	}
	return best.f
}

func eabcAsInt(v interface{}) (int64, bool) {
	switch t := v.(type) {
	case int64:
		return t, true
	case int:
		return int64(t), true
	case float64:
		if t == math.Trunc(t) && !math.IsInf(t, 0) {
			return int64(t), true
		}
	}
	return 0, false
}

// --------------------------------------------------------------------------- //
// 一轮 run
// --------------------------------------------------------------------------- //

func eabcAsStr(v interface{}) (string, bool) {
	s, ok := v.(string)
	return s, ok
}

// eabcMessages —— py `messages(sid)`：list ⇒ 原样；dict ⇒ `.get("messages") or []`；其余 ⇒ []。
func (c *eabcClient) eabcMessages(sidStr string) ([]interface{}, error) {
	v, err := c.eabcGetJSON("/api/chat/sessions/" + sidStr + "/messages")
	if err != nil {
		return nil, err
	}
	switch t := v.(type) {
	case []interface{}:
		return t, nil
	case map[string]interface{}:
		if m, ok := t["messages"]; ok {
			if lst, ok := m.([]interface{}); ok {
				return lst, nil
			}
		}
		return []interface{}{}, nil
	}
	return []interface{}{}, nil
}

// eabcRunOnce —— py `run_once(group, idx, task, model)` 的逐字搬运。
func (c *eabcClient) eabcRunOnce(group string, idx int, task, state string) (*eabcObj, error) {
	resp, err := c.eabcPost("/api/chat/sessions",
		eabcNewObj().set("title", fmt.Sprintf("[ab] %s-%d", group, idx)).set("source", "ab"))
	if err != nil {
		return nil, err
	}
	d, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, err
	}
	var j map[string]interface{}
	if err := json.Unmarshal(d, &j); err != nil {
		return nil, err
	}
	// py `sid = j.get("session_id") or j.get("id")`：保**真假值**语义 ⇒ 取原始值，
	// 进 URL 用 `str(sid)`（None ⇒ 字面 "None"，与 py f-string 逐字同）。
	var sidRaw interface{}
	if v, ok := j["session_id"]; ok && eabcPyTruthy(v) {
		sidRaw = v
	} else if v, ok := j["id"]; ok && eabcPyTruthy(v) {
		sidRaw = v
	}
	sidStr := eabcPyStr(sidRaw)

	mresp, err := c.eabcPost("/api/chat/sessions/"+sidStr+"/model",
		eabcNewObj().set("model", eabcDefaultModel))
	if err != nil {
		return nil, err
	}
	io.Copy(io.Discard, mresp.Body)
	mresp.Body.Close()

	off, err := eabcCountObs(state)
	if err != nil {
		return nil, err
	}
	t0 := time.Now()
	sresp, err := c.eabcPost("/api/chat/sessions/"+sidStr+"/send",
		eabcNewObj().set("content", task))
	if err != nil {
		return nil, err
	}
	sb, err := io.ReadAll(sresp.Body)
	sresp.Body.Close()
	if err != nil {
		return nil, err
	}
	events := eabcPyCountLines(sb)
	wall := time.Since(t0).Seconds()

	newEntries, _, err := eabcObsSince(state, off)
	if err != nil {
		return nil, err
	}

	turns := []map[string]interface{}{}
	tools := []map[string]interface{}{}
	queued := []eabcNum{}
	for _, e := range newEntries {
		if e.bad {
			// py 里非对象观测 ⇒ 后续 `.get` 崩 ⇒ 整轮失败（照实成错，✗ 静默吞）。
			return nil, fmt.Errorf("观测行不是 JSON 对象（py 于该档会崩）")
		}
		kind, _ := eabcAsStr(e.obj["kind"])
		switch kind {
		case "turn":
			turns = append(turns, e.obj)
		case "tool":
			tools = append(tools, e.obj)
		case "queued":
			queued = append(queued, eabcNumOf(e.obj["queued_ms"]))
		}
	}

	msgs, err := c.eabcMessages(sidStr)
	if err != nil {
		return nil, err
	}
	var bodyB strings.Builder
	for _, m := range msgs {
		mm, ok := m.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("messages 项不是 JSON 对象（py 于该档会崩）")
		}
		if role, _ := eabcAsStr(mm["role"]); role != "assistant" {
			continue
		}
		cv, ok := mm["content"]
		if !ok || cv == nil {
			continue
		}
		cs, ok := cv.(string)
		if !ok {
			return nil, fmt.Errorf("assistant content 非字符串（py 于该档会崩）")
		}
		bodyB.WriteString(cs)
	}
	assistantBody := bodyB.String()

	hit := strings.Contains(assistantBody, "Version") ||
		(strings.Contains(assistantBody, "version") && strings.Contains(assistantBody, "2.5.9"))

	stepNums := []eabcNum{}
	for _, t := range turns {
		stepNums = append(stepNums, eabcNumOf(t["round"]))
	}
	endSet := map[string]bool{}
	for _, t := range turns {
		endSet[eabcPyStrOrEmpty(t["end_reason"])] = true
	}
	verdictSet := map[string]bool{}
	for _, t := range turns {
		verdict := ""
		switch tv := t["turn"].(type) {
		case map[string]interface{}:
			verdict = eabcPyStrOrEmpty(tv["verdict"])
		case nil:
			verdict = ""
		default:
			return nil, fmt.Errorf("turn 字段非对象（py 于该档会崩）")
		}
		verdictSet[verdict] = true
	}
	wroteAnything := false
	for _, t := range tools {
		if tool, _ := eabcAsStr(t["tool"]); tool == "write" || tool == "edit" {
			wroteAnything = true
		}
	}

	o := eabcNewObj()
	o.set("group", group)
	o.set("run", idx)
	o.set("session", sidRaw) // py 存的是 `sid` 原值（None ⇒ null）
	o.set("steps", eabcMaxNums(stepNums))
	o.set("tool_calls", len(tools))
	o.set("wall_s", eabcPyRound(wall, 1))
	o.set("events", events)
	o.set("assistant_chars", len([]rune(assistantBody)))
	o.set("citation_hit", hit)
	o.set("end_reasons", eabcSortedSet(endSet))
	o.set("verdicts", eabcSortedSet(verdictSet))
	o.set("queued_max_ms", eabcMaxNums(queued))
	o.set("wrote_anything", wroteAnything)
	return o, nil
}

// eabcSortedSet —— py `sorted({...})`（码点序 ⇒ Go 字节序对 UTF-8 同序）。
func eabcSortedSet(set map[string]bool) []interface{} {
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]interface{}, 0, len(keys))
	for _, k := range keys {
		out = append(out, k)
	}
	return out
}

// --------------------------------------------------------------------------- //
// 小结
// --------------------------------------------------------------------------- //

// eabcMedianInts —— py `statistics.median(list[int])`：奇数取中位（int）、偶数取两中位均值（float）。
func eabcMedianInts(xs []int64) interface{} {
	if len(xs) == 0 {
		return 0
	}
	s := append([]int64(nil), xs...)
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	n := len(s)
	if n%2 == 1 {
		return s[n/2]
	}
	return float64(s[n/2-1]+s[n/2]) / 2.0
}

// eabcMedianFloats —— py `statistics.median(list[float])`：恒 float。
func eabcMedianFloats(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	s := append([]float64(nil), xs...)
	sort.Float64s(s)
	n := len(s)
	if n%2 == 1 {
		return s[n/2]
	}
	return (s[n/2-1] + s[n/2]) / 2.0
}

// eabcSummarize —— py `summarize(rows)`（空 rows ⇒ `{}`）。
func eabcSummarize(rows []*eabcObj) *eabcObj {
	if len(rows) == 0 {
		return eabcNewObj()
	}
	n := len(rows)
	hits := 0
	walls := []float64{}
	tools := []int64{}
	queuedMax := int64(0)
	endSet := map[string]bool{}
	for _, r := range rows {
		if v, _ := r.get("citation_hit"); v == true {
			hits++
		}
		if v, ok := r.get("wall_s"); ok {
			if f, ok := v.(float64); ok {
				walls = append(walls, f)
			}
		}
		if v, ok := r.get("tool_calls"); ok {
			if iv, ok := eabcAsInt(v); ok {
				tools = append(tools, iv)
			}
		}
		if v, ok := r.get("queued_max_ms"); ok {
			if iv, ok := eabcAsInt(v); ok && iv > queuedMax {
				queuedMax = iv
			}
		}
		if v, ok := r.get("end_reasons"); ok {
			if lst, ok := v.([]interface{}); ok {
				for _, e := range lst {
					if s, ok := e.(string); ok {
						endSet[s] = true
					}
				}
			}
		}
	}
	o := eabcNewObj()
	o.set("n", n)
	o.set("citation_hit_rate", eabcPyRound(float64(hits)/float64(n), 2))
	o.set("wall_s_median", eabcPyRound(eabcMedianFloats(walls), 1))
	o.set("tool_calls_median", eabcMedianInts(tools))
	o.set("queued_max_ms", queuedMax)
	o.set("end_reasons", eabcSortedSet(endSet))
	return o
}

// eabcPyInt —— py `int(s)`（十进制）：剥 py 空白后走 Atoi（★ 未复刻 py 的下划线与 Unicode 数字 ⇒ 未覆盖面）。
func eabcPyInt(s string) (int, error) {
	return strconv.Atoi(eabcPyStrip(s))
}

// --------------------------------------------------------------------------- //
// 入口
// --------------------------------------------------------------------------- //

// eabcPlanItem —— py `plan` 一行（顺序即输出顺序 · 逐字照抄 label/key/task）。
type eabcPlanItem struct {
	label string
	key   string
	task  string
}

func eabcPlan(task string) []eabcPlanItem {
	return []eabcPlanItem{
		{"课题一A-不打回", "A_no_sendback", task},
		{"课题一B-打回重做", "B_sendback", task},
		{"课题二甲-指派", "C_assign", task},
		{"课题二乙-广播自愿", "D_broadcast", task},
	}
}

func evalABCampaignsGateMain(stdout, stderr io.Writer, argv []string) int {
	_ = argv // py `main()` 不读 sys.argv ⇒ 本件一字不读（无参/--help/未知参三档行为逐字同）

	// ① 模块级面（py import 期即执行 · 顺序：CORE → TOK → STATE → OUT → RUNS）。
	core := eabcDefaultCore
	if v, ok := os.LookupEnv(eabcEnvCore); ok {
		core = v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(stderr, "ab-campaigns: 取不到 HOME（py expanduser 会回落 pwd）=> 不给结论（退码 1）：%v\n", err)
		return 1
	}
	tokenBytes, err := os.ReadFile(filepath.Join(home, ".zerg", "token"))
	if err != nil {
		fmt.Fprintf(stderr, "ab-campaigns: 读 token 失败 => 不给结论（退码 1）：%v\n", err)
		return 1
	}
	tok := strings.TrimSpace(string(tokenBytes)) // py `.read().strip()`
	state := filepath.Join(home, ".zerg", "state", "chat_obs.jsonl")
	outPath := filepath.Join(home, ".zerg", "egg-profiles", "ab-campaigns.json")
	runsStr := eabcDefaultRuns
	if v, ok := os.LookupEnv(eabcEnvRuns); ok {
		runsStr = v
	}
	runs, err := eabcPyInt(runsStr)
	if err != nil {
		fmt.Fprintf(stderr, "ab-campaigns: ZERG_AB_RUNS 不是十进制整数 => 不给结论（退码 1）：%q\n", runsStr)
		return 1
	}

	task := eabcTask()
	client := eabcNewClient(core, tok)

	results := eabcNewObj()
	results.set("run_at", time.Now().UTC().Format("2006-01-02T15:04:05Z"))
	results.set("runs_per_arm", runs)
	arms := eabcNewObj()
	results.set("arms", arms)

	for _, it := range eabcPlan(task) {
		rows := []*eabcObj{}
		for i := 1; i <= runs; i++ {
			fmt.Fprintf(stdout, "[ab] %s 第 %d/%d 次 …\n", it.label, i, runs)
			row, err := client.eabcRunOnce(it.key, i, it.task, state)
			if err != nil {
				fmt.Fprintf(stdout, "   ✗ 失败：%v\n", err)
				continue
			}
			rows = append(rows, row)
			fmt.Fprintf(stdout, "   %s\n", eabcDumps(row))
		}
		quoted := make([]interface{}, 0, len(rows))
		for _, r := range rows {
			quoted = append(quoted, r)
		}
		arm := eabcNewObj()
		arm.set("rows", quoted)
		summary := eabcSummarize(rows)
		arm.set("summary", summary)
		arms.set(it.label, arm)
		fmt.Fprintf(stdout, "   ⇒ 小结：%s\n", eabcDumps(summary))
	}

	data := []byte(eabcDumpIndent(results))
	if err := os.WriteFile(outPath, data, 0o644); err != nil {
		fmt.Fprintf(stderr, "ab-campaigns: 落档失败 => 不给结论（退码 1）：%v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "[ab] ✓ 已写：%s\n", outPath)
	fmt.Fprint(stdout, "═══ 总览 ═══\n")
	// py 末段：`for label, v in results["arms"].items(): print(f"  {label}: {json.dumps(v['summary'])}")`
	for i, k := range arms.keys {
		arm, _ := arms.vals[i].(*eabcObj)
		summary, _ := arm.get("summary")
		fmt.Fprintf(stdout, "  %s: %s\n", k, eabcDumps(summary))
	}
	return 0
}
