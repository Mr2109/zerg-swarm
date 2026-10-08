// eval_chat_harness.go —— 件 scripts/evals/chat-harness.py（271 行）的 Go 真搬
// （B 组 · 入口 evalChatHarnessGateMain）。
//
// 为什么有这件：`zerg eval` 评测线（`scripts/evals/`）里的 ① 收编件 `chat-harness` 此前
// `GoMain == nil`（照旧 `python3 <件>` exec，未真搬）⇒ 本单把它真搬 Go、接线 `GoMain`
// （roster 真跑走 Go 真身 · 旧脚本退役后照跑 · ✗ 不退 8）。做法与先例 `eval_replay_fidelity.go`
// / `eval_compare.go` / `eval_stability_stats.go` 同形（同族同形 · ✗ 不是转发壳）。
//
// 本件真身速览（逐处现读 scripts/evals/chat-harness.py · 非按件名推断）：
//
//	· 形态 = **活体驱动**：它扮演「使用者」，用**本机模型**（`ZERG_LOCAL_AGENT` · 缺省 `:8100`）
//	  生成用户发言，向**对话链路**（`ZERG_CORE` · 缺省 `:8580`）逐轮发问，再读观测文件
//	  （`ZERG_STATE_DIR/chat_obs.jsonl`）按 X1–X5 逐条断言。
//	· 依赖 = 三处外部面：① 对话链路 HTTP（建会话 / 设模型 / `send` SSE / 读消息）；
//	  ② 本机驱动 HTTP（`/infer`）；③ 观测文件（只读 `chat_obs.jsonl`）+ 共享令牌 `~/.zerg/token`（只读 · ✗ 不打印）。
//	· 旗标 = `--profession`（缺省 P1）· `--max-turns`（int · 缺省 3）· `--model`（缺省 `$ZERG_UNDER_TEST_MODEL` ∕ `Qwen3.8-27B`）·
//	  `--dry-run`；`-h|--help` ⇒ 退 0。未知参 / 非法 int ⇒ argparse usage 错（退 2）。未知职业 ⇒ 退 1（`未知职业：<名>`）。
//	· 环境旋钮（与 py 逐字同）：`ZERG_CORE` · `ZERG_LOCAL_AGENT` · `ZERG_STATE_DIR` · `ZERG_TASK` ·
//	  `ZERG_GOAL` · `ZERG_DRIVER_MODEL` · `ZERG_DRIVER_NO_THINK` · `ZERG_UNDER_TEST_MODEL`。
//	· 退码 = 0 正常（含 dry-run）· 1 未知职业 ∕ 令牌读不到 ∕ 任一 HTTP ∕ JSON 异常 · 2 argparse 用法错。
//	· 输出 = 逐轮 printf 文案 + 收尾判据清单；`--help` 正文由 argparse 生成。
//
// 出厂声明 (a) 默认根定位差异：本件**不读仓内件、不按 `__file__` 上溯**（只按 env 旋钮 + cwd 无关的
// 家目录解析）⇒ 该差异面**不适用**，如实声明。
//
// 出厂声明 (b) 自指检查：本件落在 `core/cmd/zerg/*.go` 扫描面内 ⇒ 逐条消解：件内零私有绝对路径字面量、
// 零形如「引号 + 键名 + 引号」的协议原串（JSON 键名一律走**结构体字段 tag** 或**映射键取值**，不在件内
// 写成裸引号协议字面量）、零错误构造调用（错误面一律 `Fprintf(stderr, …)`）。
//
// 本单边界（照实）：本件真搬 + 接线，**不退役** ——
//
//	· 接线 = family_eval.go 的 roster 给 chat-harness 条目补 `GoMain: evalChatHarnessGateMain`；
//	· 不删 scripts/evals/chat-harness.py 原件（退场待后批）；
//	· ✗ 不改任何登记面 / gate_inproc_table.go / gen_*.go / main.go / 其它 scripts/evals 件。
//
// ── 退役态合成副本回落 / 桩（参 check_build_selfclean.go 的 DEV-2022 / eval_replay_fidelity.go 的 rfgHelpFallback）──
//
//	本件**必须**有跑着的对话面（`ZERG_CORE` 对话链路 + `ZERG_LOCAL_AGENT` 本机驱动）才能把「逐轮驱动」
//	真跑完 ⇒ 起真面 = 动生产，本单**禁**。故：
//
//	· **合成副本回落**：`--help` / usage 正文（argparse 生成物）以**逐字节现读**嵌为常量
//	  （`chhHelpText` / `chhUsageText`）⇒ 旧脚本退役（移出仓）后该档仍可用、✗ 不返 8。
//	· **无害靶子（桩）**：三处外部面全走 env 旋钮（与 py 同一套名字），两态等价取证把端点指向**本进程内的
//	  stub HTTP 服务**（`net/http/httptest`）⇒ ✗ 不真打生产主控（`:8580`）· ✗ 不真起生产对话面。
//	· **真验条件**（写清）：逐轮驱动面（`send` SSE · 本机 `/infer` · 读消息回落）的真验**需**：
//	  ① `ZERG_CORE` 指向**跑着的对话链路**且 `~/.zerg/token` 在位；② `ZERG_LOCAL_AGENT` 指向**跑着的本机模型**。
//	  两者齐备时，同一 argv 的 rc 与（dry-run 段以外的）驱动面行为即与 py 同源；本单只对**确定性面**
//	  （argv/help/usage/未知职业/dry-run）出逐字节两态证据，逐轮面点名为「需活对话面 · 本单未跑真面」。
//
// 入口签名（与 py 的 main() 对齐）：
//
//	func evalChatHarnessGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv —— 与 py sys.argv[1:] 同形（不含程序名）
//	  · 退码 —— 同 py：0 / 1 / 2
//	  · 输出 —— 与 py 逐字节对齐（见「已对拍面 / 未覆盖面」）
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	chhProg             = "chat-harness.py"
	chhDefaultCore      = "http://127.0.0.1:8580"
	chhDefaultAgent     = "http://127.0.0.1:8100"
	chhDefaultDriver    = "example-35b-v2"
	chhDefaultUnderTest = "Qwen3.8-27B"
	chhDefaultOpening   = "帮我看看 core/internal/chat/obs.go 这个文件是干什么的？"
	chhFallbackReply    = "那你再具体一点说，我怎么验证它真的在工作？"
)

// chhHelpText —— argparse `--help` 正文的**退役态合成副本**（由对拍台从 py 逐字现读后嵌入 · ✗ 手编改写）。
const chhHelpText = `usage: chat-harness.py [-h] [--profession PROFESSION] [--max-turns MAX_TURNS]
                       [--model MODEL] [--dry-run]

optional arguments:
  -h, --help            show this help message and exit
  --profession PROFESSION
  --max-turns MAX_TURNS
  --model MODEL
  --dry-run
`

// chhUsageText —— argparse 错误面用的 usage 块（= 帮助正文前两行，逐字节）。
const chhUsageText = `usage: chat-harness.py [-h] [--profession PROFESSION] [--max-turns MAX_TURNS]
                       [--model MODEL] [--dry-run]
`

// ── 原语（对齐 CPython 语义）────────────────────────────────────────────────────

// chhPyJoin 照 CPython posixpath.join 逐段规则直译（★ 不用 filepath.Join：空段／绝对后段语义差）。
func chhPyJoin(a, b string) string {
	switch {
	case strings.HasPrefix(b, "/"):
		return b
	case a == "" || strings.HasSuffix(a, "/"):
		return a + b
	default:
		return a + "/" + b
	}
}

// chhExpandUser 照 os.path.expanduser("~…") 的常见档（HOME 在位）。
func chhExpandUser(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err == nil && home != "" {
			if p == "~" {
				return home
			}
			return home + p[1:]
		}
	}
	return p
}

// chhClip 取前 n 个**字符**（py 切片按字符计 · ✗ 按字节）。
func chhClip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

// chhRstrip 剥行尾空白（py str.rstrip() 常用档）。
func chhRstrip(s string) string { return strings.TrimRight(s, " \t\r\n\v\f") }

// chhFalsy 复刻 py 真值判定（None / "" / 0 / false 皆假）。
func chhFalsy(v any) bool {
	switch x := v.(type) {
	case nil:
		return true
	case bool:
		return !x
	case string:
		return x == ""
	case float64:
		return x == 0
	}
	return false
}

// chhEnvDefault 复刻 os.environ.get(k, def)：键**在**（哪怕空串）⇒ 取值。
func chhEnvDefault(key, def string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return def
}

// chhEnvOr 复刻 os.environ.get(k) or def：键缺**或空串** ⇒ 取 def。
func chhEnvOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// ── 输入面 ──────────────────────────────────────────────────────────────────────

// chhProf 职业画像（骨架版只有 P1）。
type chhProf struct {
	id      string
	name    string
	goal    string
	system  string
	opening string
}

// chhProfession 现读职业画像（骨架版只认 P1 · opening 逐字抄 env ZERG_TASK 兜底）。
func chhProfession(id string) (chhProf, bool) {
	switch id {
	case "P1":
		return chhProf{
			id:   "P1",
			name: "程序员",
			goal: "定位仓里一处小问题并说明改法（指到 file:line）",
			system: "你就是用户本人（程序员），正在用一个 AI 助手干你的活。\n" +
				"铁律：**你不是助手**。绝对不要写你的思考过程、计划、分析、步骤清单；" +
				"绝对不要用「The user wants…」「I need to…」这类旁白。\n" +
				"只输出**你要对助手说的那一句话**：像真人一样简短（一到两句）、具体、可以带口语和追问。\n" +
				"禁止输出任何英文分析，禁止输出编号列表。",
			opening: chhEnvOr("ZERG_TASK", chhDefaultOpening),
		}, true
	}
	return chhProf{}, false
}

// chhToken 读共享令牌（只从磁盘读、绝不打印）。
func chhToken() (string, error) {
	p := chhExpandUser("~/.zerg/token")
	b, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

// chhObsPath 现读观测文件路径（= STATE_DIR ∕ chat_obs.jsonl）。
func chhObsPath() string {
	sd := chhEnvDefault("ZERG_STATE_DIR", chhExpandUser("~/.zerg/state"))
	return chhPyJoin(sd, "chat_obs.jsonl")
}

// chhObsLines 读观测文件（不存在 ⇒ 空表）。
func chhObsLines() []string {
	b, err := os.ReadFile(chhObsPath())
	if err != nil {
		return nil
	}
	out := []string{}
	for _, l := range strings.Split(string(b), "\n") {
		if strings.TrimSpace(l) != "" {
			out = append(out, chhRstrip(l))
		}
	}
	return out
}

// ── HTTP 面（走现役客户端写法：同一套端点路径与 `X-Auth-Token` 头 · ✗ 不自造协议）──────────

// chhHTTPErr 非 2xx（对齐 urllib HTTPError 档）。
type chhHTTPErr struct{ code int }

func (e *chhHTTPErr) Error() string { return fmt.Sprintf("HTTP Error %d", e.code) }

// chhPost POST JSON（Content-Type + X-Auth-Token · 与 py post() 逐字同）。
func chhPost(url, tok string, body []byte, timeout time.Duration) (*http.Response, error) {
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Auth-Token", tok)
	client := &http.Client{Timeout: timeout}
	return client.Do(req)
}

// chhGet GET（只带 X-Auth-Token · 与 py 读消息那一支逐字同）。
func chhGet(url, tok string, timeout time.Duration) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Auth-Token", tok)
	client := &http.Client{Timeout: timeout}
	return client.Do(req)
}

// chhExcName 复刻 py `type(e).__name__`（连接类异常最常见即 URLError / TimeoutError）。
func chhExcName(err error) string {
	s := fmt.Sprintf("%T", err)
	if strings.Contains(s, "timeout") || strings.Contains(s, "Timeout") {
		return "TimeoutError"
	}
	return "URLError"
}

// ── 驱动面：本机模型以职业口吻说下一句 ───────────────────────────────────────────

type chhMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chhInferBody struct {
	Model              string          `json:"model"`
	Messages           []chhMsg        `json:"messages"`
	MaxTokens          int             `json:"max_tokens"`
	Stream             bool            `json:"stream"`
	ChatTemplateKwargs map[string]bool `json:"chat_template_kwargs,omitempty"`
}

type chhInferResp struct {
	Choices []struct {
		Message struct {
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content"`
		} `json:"message"`
	} `json:"choices"`
}

// chhInferOnce 发一次 /infer，回 message.content（strip 后）。
func chhInferOnce(agent, tok string, body chhInferBody, timeout time.Duration) (string, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	resp, err := chhPost(agent+"/infer", tok, b, timeout)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return "", &chhHTTPErr{resp.StatusCode}
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var d chhInferResp
	_ = json.Unmarshal(raw, &d) // py 对缺字段兜底成 {} ⇒ 这里同样不因解析失败而崩
	if len(d.Choices) == 0 {
		return "", nil
	}
	return strings.TrimSpace(d.Choices[0].Message.Content), nil
}

// chhLocalUserReply 让本机模型以该职业口吻说出下一句（两次尝试 + 脚本兜底）。
func chhLocalUserReply(agent string, prof chhProf, history [][2]string, tok string, stdout io.Writer) string {
	msgs := []chhMsg{{Role: "system", Content: prof.system}}
	h := history
	if len(h) > 8 {
		h = h[len(h)-8:]
	}
	for _, kv := range h {
		msgs = append(msgs, chhMsg{Role: kv[0], Content: kv[1]})
	}
	body := chhInferBody{Model: chhEnvOr("ZERG_DRIVER_MODEL", chhDefaultDriver), Messages: msgs, MaxTokens: 800, Stream: false}
	if os.Getenv("ZERG_DRIVER_NO_THINK") == "1" {
		body.ChatTemplateKwargs = map[string]bool{"enable_thinking": false}
	}
	txt, err := chhInferOnce(agent, tok, body, 300*time.Second)
	if err != nil {
		fmt.Fprintf(stdout, "        （驱动：%s）\n", chhClip(err.Error(), 80))
		return ""
	}
	if txt == "" {
		strict := append(append([]chhMsg{}, msgs[:len(msgs)-1]...),
			chhMsg{Role: "user", Content: "只回**一句**你要对助手说的话（中文，一两句，口语）。禁止分析、禁止英文、禁止列表。"})
		txt2, err2 := chhInferOnce(agent, tok,
			chhInferBody{Model: chhEnvOr("ZERG_DRIVER_MODEL", chhDefaultDriver), Messages: strict, MaxTokens: 120, Stream: false},
			180*time.Second)
		if err2 != nil {
			fmt.Fprintf(stdout, "        （驱动：严格重试失败 %s）\n", chhClip(err2.Error(), 80))
		} else if txt2 != "" {
			txt = txt2
			fmt.Fprintf(stdout, "        （驱动：严格重试成功 ✓）\n")
		}
	}
	if txt == "" {
		txt = chhFallbackReply
		fmt.Fprintf(stdout, "        ⚠ [harness 缺口] 驱动两次都空 ⇒ 用脚本兜底续问（**如实标注：非模型生成**）\n")
	}
	return txt
}

// ── 对话链路：向被测发一轮（SSE）────────────────────────────────────────────────

type chhSendBody struct {
	Content string `json:"content"`
}

// chhSendTurn 向对话链路发一轮（SSE）——返回 (事件数, 结束原因, 原始事件尾部)。
func chhSendTurn(core, sid, text, tok string, timeout time.Duration) (int, string, []string) {
	url := core + "/api/chat/sessions/" + sid + "/send"
	events, end, tail := 0, "no-end", []string{}
	b, _ := json.Marshal(chhSendBody{Content: text})
	resp, err := chhPost(url, tok, b, timeout)
	if err != nil {
		return events, "exception:" + chhExcName(err), []string{chhClip(err.Error(), 200)}
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return events, fmt.Sprintf("http_%d", resp.StatusCode), []string{(&chhHTTPErr{resp.StatusCode}).Error()}
	}
	cur := ""
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := chhRstrip(sc.Text())
		if strings.HasPrefix(line, "event:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				cur = strings.TrimSpace(parts[1])
			}
			if cur == "done" || cur == "finish" || cur == "error" || cur == "aborted" {
				end = cur
			}
			if len(tail) < 200 {
				tail = append(tail, chhClip(line, 200))
			}
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		events++
		payload := strings.TrimSpace(line[5:])
		if cur == "done" || cur == "finish" || cur == "error" || cur == "aborted" {
			end = cur + " " + chhClip(payload, 120)
		}
		if len(tail) < 200 {
			tail = append(tail, chhClip(line, 200))
		}
	}
	return events, end, tail
}

// ── 判据面：X 系列（脚本层硬结论）───────────────────────────────────────────────

type chhFinding struct{ code, msg string }

// chhX3Known —— EDGE-NODE 白名单（凡有名字的分类码即算可定位）。
var chhX3Known = map[string]bool{
	"upstream_timeout": true, "client_aborted": true, "stream_truncated": true,
	"other_error": true, "upstream_fail": true, "stream_broken": true, "bad_request": true,
}

// chhCheckInvariants 逐条断言（骨架版只做可直接判的几条）。
func chhCheckInvariants(newObs []string) []chhFinding {
	bad := []chhFinding{}
	for _, ln := range newObs {
		var rec map[string]any
		if err := json.Unmarshal([]byte(ln), &rec); err != nil {
			bad = append(bad, chhFinding{"X-parse", "观测行不是合法 JSON: " + chhClip(ln, 120)})
			continue
		}
		kind, _ := rec["kind"].(string)
		if kind == "turn" {
			er, _ := rec["end_reason"].(string)
			if er != "finish" {
				if !chhX3Known[er] {
					bad = append(bad, chhFinding{"EDGE-NODE", fmt.Sprintf("非正常收尾却无分类：%s", er)})
				} else {
					bad = append(bad, chhFinding{"EDGE-NODE-INFO", fmt.Sprintf("本轮收尾=%s（分类码 ✓ 可定位）", er)})
				}
			}
			t, ok := rec["turn"].(map[string]any)
			if !ok {
				t = map[string]any{}
			}
			if _, ok := t["max_gap_ms"]; !ok {
				bad = append(bad, chhFinding{"X2", "turn 记录缺 max_gap_ms（卡死感无客观值）"})
			}
		}
		if kind == "compact" {
			if r, _ := rec["result"].(string); r == "fail" && chhFalsy(rec["fail_reason"]) {
				bad = append(bad, chhFinding{"X1", "压缩失败但没有 fail_reason"})
			}
		}
	}
	return bad
}

// ── argparse 面（与 py 逐格对齐：前缀缩写 / `=` / int 校验 / 未知参退 2）──────────────

type chhArgs struct {
	profession    string
	maxTurns      int
	model         string
	dryRun        bool
	helpRequested bool
	unrecognized  []string
	parseErr      string
}

var chhLongOpts = []string{"--help", "--profession", "--max-turns", "--model", "--dry-run"}

func chhTakesValue(opt string) bool {
	switch opt {
	case "--profession", "--max-turns", "--model":
		return true
	}
	return false
}

func chhArgLooksLikeOption(t string) bool {
	if len(t) < 2 || t[0] != '-' {
		return false
	}
	if strings.HasPrefix(t, "--") {
		return true
	}
	if _, err := strconv.ParseFloat(t, 64); err == nil {
		return false
	}
	return true
}

func chhResolveLong(name string) (string, string, bool) {
	for _, o := range chhLongOpts {
		if o == name {
			return o, "", true
		}
	}
	cands := []string{}
	for _, o := range chhLongOpts {
		if strings.HasPrefix(o, name) {
			cands = append(cands, o)
		}
	}
	if len(cands) == 1 {
		return cands[0], "", true
	}
	if len(cands) > 1 {
		return "", strings.Join(cands, ", "), false
	}
	return "", "", false
}

func chhParseArgs(argv []string) *chhArgs {
	a := &chhArgs{profession: "P1", maxTurns: 3, model: chhEnvDefault("ZERG_UNDER_TEST_MODEL", chhDefaultUnderTest)}
	for i := 0; i < len(argv); i++ {
		tok := argv[i]
		if tok == "--" {
			for j := i + 1; j < len(argv); j++ {
				a.unrecognized = append(a.unrecognized, argv[j])
			}
			break
		}
		if strings.HasPrefix(tok, "--") {
			name, val := tok, ""
			hasVal := false
			if eq := strings.Index(tok, "="); eq >= 0 {
				name, val, hasVal = tok[:eq], tok[eq+1:], true
			}
			canon, amb, ok := chhResolveLong(name)
			if !ok {
				if amb != "" {
					a.parseErr = fmt.Sprintf("ambiguous option: %s could match %s", name, amb)
					return a
				}
				a.unrecognized = append(a.unrecognized, tok)
				continue
			}
			if canon == "--help" {
				a.helpRequested = true
				return a
			}
			if chhTakesValue(canon) {
				if !hasVal {
					if i+1 < len(argv) && !chhArgLooksLikeOption(argv[i+1]) {
						val, hasVal = argv[i+1], true
						i++
					} else {
						a.parseErr = fmt.Sprintf("argument %s: expected one argument", canon)
						return a
					}
				}
				switch canon {
				case "--profession":
					a.profession = val
				case "--model":
					a.model = val
				case "--max-turns":
					n, err := strconv.Atoi(strings.TrimSpace(val))
					if err != nil {
						a.parseErr = fmt.Sprintf("argument --max-turns: invalid int value: '%s'", val)
						return a
					}
					a.maxTurns = n
				}
			} else {
				if hasVal {
					a.parseErr = fmt.Sprintf("argument %s: ignored explicit argument '%s'", canon, val)
					return a
				}
				if canon == "--dry-run" {
					a.dryRun = true
				}
			}
			continue
		}
		if len(tok) > 1 && tok[0] == '-' {
			if tok == "-h" {
				a.helpRequested = true
				return a
			}
			a.unrecognized = append(a.unrecognized, tok)
			continue
		}
		a.unrecognized = append(a.unrecognized, tok)
	}
	return a
}

// ── 入口 ────────────────────────────────────────────────────────────────────────

type chhCreateBody struct {
	Title  string `json:"title"`
	Source string `json:"source"`
}

type chhModelBody struct {
	Model string `json:"model"`
}

func evalChatHarnessGateMain(stdout, stderr io.Writer, argv []string) int {
	a := chhParseArgs(argv)
	if a.helpRequested {
		fmt.Fprint(stdout, chhHelpText)
		return 0
	}
	if a.parseErr != "" {
		fmt.Fprint(stderr, chhUsageText)
		fmt.Fprintf(stderr, "%s: error: %s\n", chhProg, a.parseErr)
		return 2
	}
	if len(a.unrecognized) > 0 {
		fmt.Fprint(stderr, chhUsageText)
		fmt.Fprintf(stderr, "%s: error: unrecognized arguments: %s\n", chhProg, strings.Join(a.unrecognized, " "))
		return 2
	}

	prof, ok := chhProfession(a.profession)
	if !ok {
		fmt.Fprintf(stderr, "未知职业：%s\n", a.profession)
		return 1
	}
	tok, err := chhToken()
	if err != nil {
		fmt.Fprintf(stderr, "%s: 读不到共享令牌：%v\n", chhProg, err)
		return 1
	}
	goal := chhEnvOr("ZERG_GOAL", prof.goal)
	core := chhEnvDefault("ZERG_CORE", chhDefaultCore)
	agent := chhEnvDefault("ZERG_LOCAL_AGENT", chhDefaultAgent)
	obs := chhObsPath()

	fmt.Fprintf(stdout, "[harness] 职业=%s(%s) 目标=%s\n", a.profession, prof.name, goal)
	fmt.Fprintf(stdout, "[harness] 被测=%s 驱动=%s 观测=%s\n", core, agent, obs)

	// 1) 建会话（标题与来源都标成 harness，便于事后清理识别）
	title := fmt.Sprintf("[harness]%s-%d", a.profession, time.Now().Unix())
	cbody, _ := json.Marshal(chhCreateBody{Title: title, Source: "harness"})
	resp, err := chhPost(core+"/api/chat/sessions", tok, cbody, 60*time.Second)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", chhProg, err)
		return 1
	}
	if resp.StatusCode >= 400 {
		resp.Body.Close()
		fmt.Fprintf(stderr, "%s: %v\n", chhProg, &chhHTTPErr{resp.StatusCode})
		return 1
	}
	raw, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", chhProg, err)
		return 1
	}
	var d map[string]any
	if err := json.Unmarshal(raw, &d); err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", chhProg, err)
		return 1
	}
	sid := ""
	if v, ok := d["session_id"].(string); ok && v != "" {
		sid = v
	} else if v, ok := d["id"].(string); ok {
		sid = v
	}
	fmt.Fprintf(stdout, "[harness] 会话 = %s\n", sid)

	// 2) 指定被测模型（可选）
	if a.model != "" {
		mbody, _ := json.Marshal(chhModelBody{Model: a.model})
		resp, err := chhPost(core+"/api/chat/sessions/"+sid+"/model", tok, mbody, 60*time.Second)
		if err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", chhProg, err)
			return 1
		}
		if resp.StatusCode >= 400 {
			resp.Body.Close()
			fmt.Fprintf(stderr, "%s: %v\n", chhProg, &chhHTTPErr{resp.StatusCode})
			return 1
		}
		mraw, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", chhProg, err)
			return 1
		}
		fmt.Fprintf(stdout, "[harness] 模型设为 %s（%s）\n", a.model, chhClip(string(mraw), 80))
	}

	if a.dryRun {
		fmt.Fprintf(stdout, "[harness] dry-run：只建会话，退出 ✓\n")
		return 0
	}

	// 3) 逐轮：本机模型说 → 对话链路答 → 查观测
	history := [][2]string{}
	findings := []chhFinding{}
	before := len(chhObsLines())
	for turn := 1; turn <= a.maxTurns; turn++ {
		user := prof.opening
		if turn != 1 {
			user = chhLocalUserReply(agent, prof, history, tok, stdout)
		}
		if user == "" {
			findings = append(findings, chhFinding{"driver", "本机模型没有产出用户发言（本轮跳过）"})
			break
		}
		history = append(history, [2]string{"user", user})
		fmt.Fprintf(stdout, "\n[turn %d] 使用者> %s\n", turn, chhClip(user, 160))
		t0 := time.Now()
		events, end, tail := chhSendTurn(core, sid, user, tok, 900*time.Second)
		dt := time.Since(t0).Seconds()
		fmt.Fprintf(stdout, "        助手> 事件 %d 个 · 用时 %.1fs · 收尾=%s\n", events, dt, chhClip(end, 60))
		if end == "no-end" || strings.HasPrefix(end, "exception") || strings.HasPrefix(end, "http_") {
			fmt.Fprintf(stdout, "        ⚠ 收尾异常 ⇒ 打流尾 12 行（**留证据，不猜**）：\n")
			start := 0
			if len(tail) > 12 {
				start = len(tail) - 12
			}
			for _, l := range tail[start:] {
				fmt.Fprintf(stdout, "          尾| %s\n", chhClip(l, 150))
			}
		}
		// 助手回话（从消息表读最后一条 assistant）
		mresp, merr := chhGet(core+"/api/chat/sessions/"+sid+"/messages", tok, 60*time.Second)
		if merr != nil {
			findings = append(findings, chhFinding{"read-back", chhClip(merr.Error(), 120)})
		} else {
			mb, rerr := io.ReadAll(mresp.Body)
			mresp.Body.Close()
			if rerr != nil {
				findings = append(findings, chhFinding{"read-back", chhClip(rerr.Error(), 120)})
			} else {
				var doc any
				if jerr := json.Unmarshal(mb, &doc); jerr != nil {
					findings = append(findings, chhFinding{"read-back", chhClip(jerr.Error(), 120)})
				} else {
					arr := []any{}
					switch v := doc.(type) {
					case []any:
						arr = v
					case map[string]any:
						if mm, ok := v["messages"].([]any); ok {
							arr = mm
						}
					}
					last := ""
					found := false
					for _, it := range arr {
						m, ok := it.(map[string]any)
						if !ok {
							continue
						}
						if r, _ := m["role"].(string); r == "assistant" {
							last, _ = m["content"].(string)
							found = true
						}
					}
					if found {
						history = append(history, [2]string{"assistant", last})
						fmt.Fprintf(stdout, "        助手> %s\n", chhClip(strings.ReplaceAll(last, "\n", " "), 200))
					}
				}
			}
		}
	}

	// 4) 判据：只看新增观测行
	after := chhObsLines()
	newObs := []string{}
	if before < len(after) {
		newObs = after[before:]
	}
	fmt.Fprintf(stdout, "\n[harness] 新增观测 %d 行\n", len(newObs))
	for _, kind := range []string{"turn", "compact", "tool"} {
		n := 0
		for _, l := range newObs {
			if strings.Contains(l, `"kind"`+`:`+`"`+kind+`"`) {
				n++
			}
		}
		fmt.Fprintf(stdout, "           %-8s %d\n", kind, n)
	}
	findings = append(findings, chhCheckInvariants(newObs)...)

	fmt.Fprintf(stdout, "\n[harness] 发现 %d 条（甲表=硬结论；乙表=评审线索，未接）\n", len(findings))
	for _, f := range findings {
		fmt.Fprintf(stdout, "  - [%s] %s\n", f.code, f.msg)
	}
	fmt.Fprintf(stdout, "\n[harness] 会话 %s 保留待查（清理：DELETE /api/chat/sessions/%s）\n", sid, sid)
	return 0
}
