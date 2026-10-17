// family_chat.go —— `zerg chat` 族（**人类正门** · `P-1` 乙案 · 设计稿 v0.3 §5 / §7 / §9.5）。
//
// ★ 它是什么（一句话）：把服务端**已注册**的 `/api/chat/*` 面做成**人能用的 CLI 正门**；
// 首版 **7 条**子命令（`F-1` 拍定）= `new` / `ls` / `send` / `show` / `abort` / `steer` / `rm`。
//
// ★ 三条硬边界（照 §0 第一原则与 §9.1 逐条落，本件只做这些）：
//
//	① **只做「打请求 + 渲染」**：会话 / 租约 / 压缩 / 工具循环 / 中断落库 / steer 挂点
//	   这些语义**全在服务端**（`core/internal/api/chat_handlers.go` · `core/internal/chat/**`）——
//	   本件一个字都不重写（✗ 不做会话状态机 · ✗ 不自建旁路端点 · ✗ 不直连模型端口）。
//	② **只经 `client`**（`statepath.CoreBaseURL()` + `X-Auth-Token`）：✗ 不走
//	   `/v1/chat/completions`、✗ 不 spool 文件互传 —— 与 `ui/` **同一面**（§9.1 第 1 条）。
//	③ **✗ 不动 `core/internal/**`**（首版命令面那一笔的硬边界）：本件只消费**现成**端点。
//
// ★ 落点与出处（逐条可核 · ✗ 不自由发挥）：
//
//	· 命令表 = 稿 §5.1 · 旗标口径 = §5.2 · `--json` 字段 = §5.4；
//	· 退码 = §5.3（**一律取自既有码 · ✗ 不新增码**；租约 409 ⇒ `14`）；
//	· 非交互 / 管道 / stdout 纪律 = §5.5 · 非 TTY 与 `--json` ⇒ **非流式** = §7.5；
//	· 事件全表（13 类）与分帧口径 = §7.1 / §7.2（按空行分帧 · 事件块内**全部** `data:` 行拼接）。
//
// ★ 已拍口径（本件逐条兑现）：`F-1` 七条 · `O-2` 只用 `--new`（✗ 不加 `--once`）·
// `O-3`＋`O-25` `--json` **强制非流式**且包封单 blob · `O-4` 接 `steer`（字段名 `steer_queued`）·
// `O-5` `regenerate`/`message_edit`/`search` **三条都不接**（✗ 不登记 ⇒ 真敲 = 未知命令 2）·
// `O-6` ✗ 不接 `send-tool` · `O-9` 默认纯文本 ＋ `--markdown` opt-in ·
// `O-12` 只认**完整**会话 id（✗ 不调 `session-resolve` 解析）· `O-16` `--last` ✗ 不默认按 cwd，
// 加 `--last-in-cwd` 收紧 · `O-14` 生成中回车默认 `queue`（**REPL 键位面 · 归 T9**，本件不落）·
// `O-7` Go × 非全屏行式（**REPL/TUI 骨架归 T9** —— 本件 = 命令壳，7 条子命令逐条可脚本化）。
//
// ★ **2026-10-17 新拍定（Mr2109）· 裸 `zerg chat` 按 TTY 分流**（它**作废**了「裸条归 T9 ⇒ 不登记」）：
//
//	· **有 TTY** ⇒ 裸跑**进 REPL**（骨架归 T9 · 入口落点 = 本件的 `chatEnterREPL`）；
//	· **无 TTY** ⇒ 裸跑**退 2** ＋ 提示写 **stderr**（「请加子命令，或进 TTY 使用交互模式」）；
//	· **`--help`** ⇒ 走派发层（`main.go` 的 `if inv.wantHelp` 那一格）⇒ **rc=0**，与 TTY 无关
//	  （裸条在册即自动成立 —— 本函数**不**碰这一态）；
//	· **防线**：裸条 `len(inv.args) > 0` **一律退 2**（✗ 否则 `zerg chat <未知名>` 会误进 REPL）；
//	· **命令树 path 数 248 → 256**（**+8** 条 = 7 条子命令 ＋ 裸条 `chat`）——
//	  ★ 别处若出现 **255**，那是 T9 勘定给的旧值、**已被本拍定作废**。
//
// ★ 两条**现读差异**（本件照现读落地；差异已逐条列进本路报告待裁）：
//
//	① `--resume <ref>` **本件不登记**：`--resume` 在 `main.go` 已是**全局布尔**
//	  （`zerg context ls --resume` 在用 · `parseInvocation` 专用 case），而 `valueFlagName` 是
//	  **取值**名字表 ⇒ 同一名字两态并存必撞门73 判据丙（「名字表按取值收 ⇒ 值/无值两态不一致」）。
//	  ⇒ 续接语义由 `--session <完整 id>` 承担，与 `O-12`「只认完整 id · ✗ 不走 `session-resolve`」同口径。
//	② `--timeout <时长>` **不写进本族用法串**：它是**全局**旗标（真消费点 = `wallclock.go` 的
//	  `wallclockFlag` · 派发层 · 全命令），对本族**照样生效**（到点退 `11`）；写进用法串会让门73
//	  面④ 新增一条「用法串有、实现件不读」⇒ rc=1，而豁免表在**禁碰**的 `check_*.go` 里。
//
// ★ **`--json` 语义（2026-10-17 Mr2109 拍定 · 本件逐条兑现）**：**单 blob**（一个包封）·
// **列表类**（`chat ls` / `chat show`）出 `items[] ＋ meta{count,total}`（`total` = **截尾前**条数 ·
// 见两处 `metaAddJSON("total", …)`）· **字段表外 ⇒ rc=2 ＋ stdout 零字节**（走 `requireFields` /
// `emitSelected` 的**唯一**出口）· **`--json` 强制非流式**（`O-3`＋`O-25`）。
//
// ★ 扁平化口径（`--json` · 照机制现读）：出口 `marshalObject` 只收 `map[string]string`
// ⇒ §5.4 的嵌套形态（`usage{…}` / `tool_calls[]{…}` / `messages[]{…}`）落成**扁平字段名**
// （`input_tokens` 一类**逐字取服务端载荷键**，✗ 不自造词汇；`tool_calls` 收**一个** JSON 文本格）。
package main

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
)

// ---- 常量（族名 / 会话面 / 落点）------------------------------------------------------------

const (
	// chatLastKey —— `global` 桶里那一格的键名（`chatLastFile.Global` 是 map ⇒ 键写死一处）。
	chatLastKey = "session_id"
	// chatFamily —— 族名（`zerg chat …`）。本族是**新族**（`P-1` 乙案：人类正门 = `zerg chat`）。
	chatFamily = "chat"
	// chatSourceCLI —— 本客户端建会话时显式传给服务端的 `source`（§9.2：终端 `cli`／GUI `desktop`）——
	// ★ **显式传**（✗ 不靠服务端缺省 `desktop`）：否则将来改缺省值会**静默改写历史标注**。
	chatSourceCLI = "cli"
	// chatLastFileName —— `--last` 的落点（§5.2「本机记住的最后会话」）：
	// `<状态目录>/chat-last.json`（状态目录真源 = `statepath.Dir()` · ✗ 不写死 `~/.zerg`）。
	chatLastFileName = "chat-last.json"
	// chatListLimitDefault —— `chat ls` 的默认列数（与现读服务端 `ListSessions(100)` 同值 · §5.1 的 `--limit`）。
	chatListLimitDefault = 100
	// chatWindowLimitDefault —— `chat show --window <mid>` 的窗口半径（服务端缺省 20 · 上限 100，见 `chat_window_handler.go`）。
	chatWindowLimitDefault = 20
)

// ---- 退出码的**名字**（码值本身一处不写：真源 = `exitcodes.go` 的表）------------------------

// 本件用到的码（逐条照 §5.3；✗ 不新增码 · 一个裸数字都不落）：
//
//	0 `exitOK` · 1 `exitFail` · 2 `exitUsage` · 4（未认证 · `codeOfKind` 投影）·
//	8 `exitBlocked` · 10 `exitInsufficientResource` · 11 `exitTimeout` · 12 `exitUnreachable` ·
//	14 `exitConflict` · 130 `exitInterrupted`（由 `main.go` 的信号处理器给）。

// ---- 会话语义：id 形状 / 短引用展开（一处真源：§5.2「展开规则要落一处真源」）-------------------

// chatSessionIDRe —— **完整**会话 id 的形状（`O-12` 只认完整 id；真源 = `internal/chat/chat_session.go:61`
// 的 `NewSessionID()` 逐字 `chat_YYYYMMDD_HHMMSS_6hex`）。
var chatSessionIDRe = regexp.MustCompile(`^chat_[0-9]{8}_[0-9]{6}_[0-9a-f]{6}$`)

// chatNormalizeSessionRef —— 短引用 `#<id>` 的**唯一**展开处（§5.2：CLI 侧自行展开成完整 id；
// ✗ 不许两个客户端各写一套）。展开 = 去掉前导 `#`（**只去一个**）；形状仍按完整 id 判。
func chatNormalizeSessionRef(s string) (string, bool) {
	s = strings.TrimSpace(s)
	// 注释里出现 `#` 是为了让人面文案与 `--session` 口径一致：见 `chatUsageHints`。
	if strings.HasPrefix(s, "#") {
		s = strings.TrimPrefix(s, "#")
	}
	if !chatSessionIDRe.MatchString(s) {
		return "", false
	}
	return s, true
}

// ---- 裸 `zerg chat`（TTY 分流入口 · 2026-10-17 拍定）-------------------------------------

// cmdChatRoot —— **裸 `zerg chat`** 的入口（2026-10-17 Mr2109 拍定：**按 TTY 分流**）。
//
// 形态（逐条照拍定 · ✗ 不自由发挥）：
//
//	· **有 TTY** ⇒ 裸跑**进 REPL**（人机交互面 —— 骨架归 **T9**，落点 = `chatEnterREPL`）；
//	· **无 TTY** ⇒ 裸跑**退 2** ＋ 提示走 **stderr**（「请加子命令，或进 TTY 使用交互模式」）；
//	· **`--help`** ⇒ 派发层那一格（`inv.wantHelp` ⇒ `cmdUsageHelp`）⇒ **rc=0**，与 TTY 无关
//	  ⇒ 本函数**这条路径一个字节都不碰**（裸条在册即自动成立）。
//
// ★ 防线（拍定第 2 条）：**`len(inv.args) > 0` 一律退 2** —— 裸条会把未知名收成位置参数
// （同族先例 `help` 裸条：未知名由 `cmdHelp` 自己判）⇒ 不判的话 `zerg chat news` 会**误进 REPL**。
//
// ★★ **Ctrl-C 共存（2026-10-17 Mr2109 已拍 · 本路只登记现状与口径 · ✗ 不实现）**
//
// ① **现读全局面**（`main.go` · 一处三信号）：`sigs := make(chan os.Signal, 1)`（`:55`）⇒
// `signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)`（`:62`）⇒ 收信号即
// 打印一行判词 + `removeActiveMaterialized()` ⇒ `os.Exit(exitInterrupted)`（`:76` · **130**）。
// 语义逐字 = 「默认只退订、不取消」（`--cancel-on-interrupt` 才真取消）。
//
// ② **已拍口径 · ✗ 不改全局 `signal.Notify`**（它管**所有族**的 Ctrl-C 语义；为 `chat` 一家改 =
// 动别族的 Ctrl-C）—— 与 Hermes 经典 CLI（`full_screen=False` ＋ `patch_stdout`）**同样不改全局信号**；
// 裸跑**进 REPL（有 TTY 档）时由裸条入口显式接管 / 退出时恢复 SIGINT**；**非 TTY 档完全不动**
// （那一档裸跑本就退 2）。
//
// ③ **T9 的落点建议（件:行）**：
//
//	· ★ **前置一处**：`core/cmd/zerg/main.go:55` 的 `sigs` 是**`main()` 的局部变量** ⇒ 本件**拿不到句柄**
//	  做 `signal.Stop` ⇒ T9 必须先在 `main.go` 把这一枚**提为包级**（如 `var cliSigs = make(chan os.Signal, 1)`）
//	  或加一枚包级 `func stopCLISignalHandlers()`（★ 这一处是**唯一**需要动 `main.go` 信号面的地方）。
//	· **接管点** = `core/cmd/zerg/family_chat.go` 的 `chatEnterREPL`（本函数下一段）——
//	  进 raw mode **之前**：`signal.Stop(cliSigs)` ⇒（可选）自建 `signal.Notify(own, SIGTERM, SIGHUP)`
//	  的小处理面，收到即 **先 `term.Restore(fd, oldState)` 再 `os.Exit(exitInterrupted)`**（与全局**同码 130**）。
//	· **退出点**：`defer` 里 **先 `term.Restore` 再 `signal.Notify(cliSigs, SIGINT, SIGTERM, SIGHUP)`**（顺序见 ④）。
//
// ④ **raw-mode 与它的先后顺序（写死）**：`signal.Stop` ⇒ **在** `term.MakeRaw` **之前**；
// `term.Restore` ⇒ **在** `signal.Notify` 恢复**之前**。理由：反序会在窗口期留下「终端已 raw / 处理器已装」
// 或「终端未复原 / 处理器已卸」的中间态 —— 前者让 `SIGTERM` 走 `os.Exit(130)` 而**终端留在 raw**。
// ⑤ **残留风险（✗ 不掩）**：ⓐ `signal.Stop` 是**整枚通道**语义（当前只有 `main()` 用 ⇒ 安全，将来若别处复用
// 同一通道，REPL 期的 Stop 会把它们一起停掉）；ⓑ **只 Stop SIGINT 而不管 SIGTERM/SIGHUP** ⇒ 关窗/登出
// 仍 `os.Exit(130)` 且**不恢复终端**（残留 raw mode：shell 回显错乱）；ⓒ panic/异常退出路径不经过 `defer`
// ⇒ 需顶层 recover 兜；ⓓ raw mode 下 Ctrl-C **不再产生 SIGINT**（是 stdin 的 `\x03` 字节）⇒ §7.4 的
// 「Ctrl-C 三格」（草稿优先清空 / 2s 双按强退 / 生成中打的字搬回队列）**必须由 REPL 自己读 `\x03` 实现**，
// 与全局处理器**无关**（这也是本拍定「接管」的落点）。
func cmdChatRoot(inv *invocation, stdout, stderr io.Writer) int {
	if len(inv.args) > 0 {
		return chatReportErr(inv, stderr, "usage", "not_a_chat_action",
			fmt.Sprintf("`zerg chat %s` 不是本族的动作 —— 子命令只有 7 条：new · ls · send · show · abort · steer · rm（族级帮助：`%s help chat`）",
				inv.args[0], progName),
			exitCodeOf("usage"))
	}
	// ★ `inv.tty` 只看 **stdout**；REPL 还要**能读**：stdin 不是真终端（`zerg chat < /dev/null`）时
	// 进 REPL 只会立刻读到 EOF ⇒ 与「无 TTY」同一档收口（同 rc 2 · 同一句提示）。
	if !inv.tty || !isTTYFile(os.Stdin) {
		return chatReportErr(inv, stderr, "usage", "subcommand_required",
			"非交互（无 TTY）下裸跑 `zerg chat` 没有意义：请加子命令（`zerg chat new|ls|send|show|abort|steer|rm`），或进 TTY 使用交互模式",
			exitCodeOf("usage"))
	}
	return chatEnterREPL(inv, stdout, stderr)
}

// chatEnterREPL —— **REPL 入口**（T9 已落地 · 实现在 `family_chat_repl.go` 的 `chatREPLRun`）。
//
// ★ 形态（`O-7` 已拍）：**Go × 非全屏行式**（✗ 全屏 TUI · ✗ 引依赖：骨架手写，借系统的 `stty`）。
// 本函数只做一件薄事 —— 把进程的三个标准流交给 REPL（✗ 在这里塞行为：键位面 / 会话选择 /
// 流式渲染全在 `family_chat_repl.go`，一处实现一处可核）。
//
// ★ 退码：正常退出（`/exit` · `/quit` · Ctrl-D · 连按两次 Ctrl-C）一律 `0`；会话建不出来 /
// 续接 id 形状不对 ⇒ 那一档自己的码（`2` / `8` / `12` …，由 `chatReportErr` 给）。
func chatEnterREPL(inv *invocation, stdout, stderr io.Writer) int {
	return chatREPLRun(inv, os.Stdin, stdout, stderr)
}

// ---- 错误分类（照 `fetchInv` 同一口径 · 补 404/409/507/400 四档 —— §5.3）--------------------
// ---- 错误分类（照 `fetchInv` 同一口径 · 补 404/409/507/400 四档 —— §5.3）--------------------

// chatErrToCode —— 把「主控返回」翻译成 (kind, detail, rc)。**判据一处**（✗ 各命令不许各判）。
//
// 与 `readonly.go:fetchInv` 同源，只**按 §5.3 补四档**（本族特有语义）：
//
//	· 404 ⇒ `usage` / `session_not_found` / **2**（§5.3：「会话解析不到 ⇒ 不给结论」）；
//	· 409 ⇒ `conflict` / `turn_running` / **14**（会话租约：同会话并发轮次被拒 —— ✗ 不是 2）；
//	· 507 ⇒ `insufficient_resource` / `upstream_507` / **10**（子端无可用槽 · 原样转出）；
//	· 400 ⇒ `usage` / `bad_request` / **2**（请求体文法/取值不对）。
func chatErrToCode(status int) (string, string, int) {
	switch {
	case status == http.StatusUnauthorized:
		return "unauthenticated", fmt.Sprintf("http_%d", status), exitCodeOf("unauthenticated")
	case status == http.StatusForbidden:
		return "forbidden", fmt.Sprintf("http_%d", status), exitCodeOf("forbidden")
	case status == http.StatusNotFound:
		return "usage", "session_not_found", exitCodeOf("usage")
	case status == http.StatusConflict:
		return "conflict", "turn_running", exitCodeOf("conflict")
	case status == http.StatusInsufficientStorage || status == http.StatusTooManyRequests:
		return "insufficient_resource", "upstream_507", exitCodeOf("insufficient_resource")
	case status == http.StatusBadRequest:
		return "usage", "bad_request", exitCodeOf("usage")
	case status >= 500:
		return "upstream_error", fmt.Sprintf("http_%d", status), exitCodeOf("failed")
	}
	return "upstream_error", fmt.Sprintf("http_%d", status), exitCodeOf("failed")
}

// chatReportErr —— 一处统一的错误出口（人面 + 机器面 + 退码）。
func chatReportErr(inv *invocation, stderr io.Writer, kind, detail, msg string, rc int) int {
	if inv != nil {
		inv.setErr(kind, detail, msg)
	}
	fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
	if inv != nil {
		fmt.Fprintf(stderr, "error.kind=%s · detail=%s · retryable=%t · remedy=%s（§九 M7：重试判定只读 kind）\n",
			kind, detail, retryableOf(kind), remedyOf(kind))
	}
	return rc
}

// chatCall —— 本族**唯一**HTTP 出口（`sendJSON` 之上只加三件事：退码分档 / 解析失败算 BLOCKED / 原样回显）。
//
// ★ 解析失败 ⇒ `8`（`blocked`）：§5.3 逐字「打到了主控但返回体解析不了 ⇒ 8」——
// **✗ 不许把「读不到真源」当健康**（拿空载荷当「零条」= 假绿）。
func chatCall(inv *invocation, c *client, method, path, body string, out *jsonObj, stderr io.Writer) int {
	status, respBody, err := sendJSON(c, method, path, body)
	if err != nil {
		var he *httpError
		if !asHTTP(err, &he) {
			return chatReportErr(inv, stderr,
				"unreachable", "dial_failed",
				fmt.Sprintf("打不到主控 %s%s（endpoint 来源: %s）：%v", c.base, path, baseFromBuiltin, err),
				exitCodeOf("unreachable"))
		}
		status, respBody = he.status, he.body
	}
	if status >= 400 {
		kind, detail, rc := chatErrToCode(status)
		// ★ 主控的原样响应**逐字带回来**（§4.1 K7「逐字透传」要能拿到原文）—— ✗ 不吞不翻译。
		return chatReportErr(inv, stderr, kind, detail,
			fmt.Sprintf("主控 %s%s 返回 http_%d · code=%s · 原样响应: %s",
				method, path, status, orDash(errorCodeOf(respBody)), firstLine(strings.TrimSpace(respBody))),
			rc)
	}
	if out != nil {
		var v jsonObj
		if err := json.Unmarshal([]byte(respBody), &v); err != nil {
			return chatReportErr(inv, stderr, "blocked", "response_unparsable",
				fmt.Sprintf("主控 %s 的响应不是合法 JSON ⇒ 不给结论（退码 8）：%v", path, err),
				exitCodeOf("blocked"))
		}
		*out = v
	}
	return exitOK
}

// ---- 会话解析（`--session` / 位置参数 / `--last` / `--last-in-cwd`）-------------------------

// chatLastEntry —— `chat-last.json` 的一条（`global` 与 `by_cwd` 两处同形）。
type chatLastEntry struct {
	SessionID string `json:"session_id"`
	At        string `json:"at"`
}

// chatLastFile —— 状态文件内容（`global` = 本机全局；`by_cwd` = 按工作区收紧的那一档 · `O-16`）。
type chatLastFile struct {
	Global map[string]chatLastEntry `json:"global,omitempty"`
	ByCwd  map[string]chatLastEntry `json:"by_cwd,omitempty"`
}

// chatWorkspaceKey —— 工作区键（`--last-in-cwd` 的分桶键）：git root 否则 cwd（与 Hermes 的
// `--resume latest` 工作区作用域同口径 · §5.5 ④）。✗ 不用整条路径当「全局」。
func chatWorkspaceKey() string {
	root := repoRoot()
	if root != "" {
		return root
	}
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	return ""
}

// chatLastRead —— 读 `--last` 记录；**读不到 ⇒ 不给结论**（`8`）——「状态目录读不到」是 §5.3 的 8 档。
func chatLastRead(inv *invocation, stderr io.Writer, inCwd bool) (string, int) {
	path := statepath.File(chatLastFileName)
	b, err := os.ReadFile(path)
	if err != nil {
		return "", chatReportErr(inv, stderr, "blocked", "last_unreadable",
			fmt.Sprintf("读不到 `--last` 落点 %s（先跑一次 `zerg chat new` 或 `zerg chat send --new`）⇒ 不给结论（退码 8）", path),
			exitCodeOf("blocked"))
	}
	var f chatLastFile
	if err := json.Unmarshal(b, &f); err != nil {
		return "", chatReportErr(inv, stderr, "blocked", "last_unparsable",
			fmt.Sprintf("`--last` 落点 %s 不是合法 JSON ⇒ 不给结论（退码 8）：%v", path, err),
			exitCodeOf("blocked"))
	}
	if inCwd {
		e, ok := f.ByCwd[chatWorkspaceKey()]
		if !ok || e.SessionID == "" {
			return "", chatReportErr(inv, stderr, "blocked", "last_absent_in_cwd",
				fmt.Sprintf("`--last-in-cwd` 在本工作区（%s）没有记录 ⇒ 不给结论（退码 8）", chatWorkspaceKey()),
				exitCodeOf("blocked"))
		}
		return e.SessionID, exitOK
	}
	e, ok := f.Global[chatLastKey]
	if !ok || e.SessionID == "" {
		return "", chatReportErr(inv, stderr, "blocked", "last_absent",
			"`--last` 记录里没有会话 id ⇒ 不给结论（退码 8）", exitCodeOf("blocked"))
	}
	return e.SessionID, exitOK
}

// chatLastWrite —— 记一笔「最后的会话」（`new` 与 `send` 成功之后各写一次）。
// ★ 写失败**不吞**：退回 stderr 一行告警，但**不改本命令的退码**（语义面已完成 · 记账面是附加物）。
func chatLastWrite(sessionID string, stderr io.Writer) {
	if sessionID == "" {
		return
	}
	path := statepath.File(chatLastFileName)
	var f chatLastFile
	if b, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(b, &f)
	}
	now := time.Now().Format(time.RFC3339)
	e := chatLastEntry{SessionID: sessionID, At: now}
	if f.Global == nil {
		f.Global = map[string]chatLastEntry{}
	}
	f.Global[chatLastKey] = e
	if k := chatWorkspaceKey(); k != "" {
		if f.ByCwd == nil {
			f.ByCwd = map[string]chatLastEntry{}
		}
		f.ByCwd[k] = e
	}
	b, err := json.MarshalIndent(f, "", "  ")
	if err == nil {
		err = os.MkdirAll(filepath.Dir(path), 0o755)
	}
	if err == nil {
		err = os.WriteFile(path, append(b, '\n'), 0o644)
	}
	if err != nil {
		fmt.Fprintf(stderr, "%s: ⚠ `--last` 记录写不进 %s（%v）—— 本命令的语义面已完成，只是下次 `--last` 找不到它\n",
			progName, path, err)
	}
}

// chatSessionFromInv —— 逐条解一个会话 id（`--session` / 位置参数 / `--last` / `--last-in-cwd`）。
//
// 次序（写死 · ✗ 不并列不猜）：`--session` > 位置参数 > `--last-in-cwd` > `--last`。
// 两枚 `--last*` 同给 ⇒ 用法错 2（互斥：一个说「全局」、一个说「本工作区」）。
func chatSessionFromInv(inv *invocation, stderr io.Writer, positional string) (string, int) {
	if inv.hasFlag("--session") {
		v := strings.TrimSpace(inv.flagVal("--session"))
		if v == "" {
			return "", chatReportErr(inv, stderr, "usage", "missing_flag_value",
				"`--session` 给了旗标却没给 id（要完整 id，如 `chat_20261017_010203_ab12cd`）", exitCodeOf("usage"))
		}
		id, ok := chatNormalizeSessionRef(v)
		if !ok {
			return "", chatReportErr(inv, stderr, "usage", "bad_session_id",
				fmt.Sprintf("会话 id %q 形状不合法（`O-12`：**只认完整 id** `chat_<8位日>_<6位时>_<6位hex>`；✗ 不做前缀/标题解析）", v),
				exitCodeOf("usage"))
		}
		return id, exitOK
	}
	if strings.TrimSpace(positional) != "" {
		id, ok := chatNormalizeSessionRef(positional)
		if !ok {
			return "", chatReportErr(inv, stderr, "usage", "bad_session_id",
				fmt.Sprintf("会话 id %q 形状不合法（`O-12`：**只认完整 id**；✗ 不做前缀/标题解析）", positional),
				exitCodeOf("usage"))
		}
		return id, exitOK
	}
	inCwd := inv.hasFlag("--last-in-cwd")
	if inCwd && inv.last {
		return "", chatReportErr(inv, stderr, "usage", "last_flags_conflict",
			"`--last` 与 `--last-in-cwd` 互斥（一个按本机全局、一个按本工作区 · `O-16`）", exitCodeOf("usage"))
	}
	if inCwd || inv.last {
		return chatLastRead(inv, stderr, inCwd)
	}
	return "", chatReportErr(inv, stderr, "usage", "missing_session",
		"要说给哪个会话：`--session <完整 id>` / `--last` / `--last-in-cwd` 三选一（要开新会话用 `--new`）",
		exitCodeOf("usage"))
}

// ---- 图片（`--images <路径>`：读盘 → data URL —— 服务端只收 data URL 基64）-------------------

// chatImageMime —— 扩展名 → MIME（**认不出就不猜**：退用法错 2，✗ 不许把二进制当 png 喂过去）。
func chatImageMime(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	}
	return ""
}

// chatImagesToDataURLs —— 逐张读盘并转 data URL（`--images` 的取值域；✗ 不做网络取图）。
func chatImagesToDataURLs(inv *invocation, mimes []string, stderr io.Writer) ([]string, int) {
	out := []string{}
	for _, p := range mimes {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		mime := chatImageMime(p)
		if mime == "" {
			return nil, chatReportErr(inv, stderr, "usage", "image_kind_unknown",
				fmt.Sprintf("`--images %s` 的扩展名认不出（本版只收 .png/.jpg/.jpeg/.gif/.webp）", p), exitCodeOf("usage"))
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, chatReportErr(inv, stderr, "usage", "content_unreadable",
				fmt.Sprintf("`--images %s` 读不到：%v", p, err), exitCodeOf("usage"))
		}
		out = append(out, "data:"+mime+";base64,"+base64.StdEncoding.EncodeToString(b))
	}
	return out, exitOK
}

// ---- `chat new` --------------------------------------------------------------------------

// chatNewFields —— `--json` 可取字段（§5.4 · 字段名**逐字取服务端载荷键**）。
var chatNewFields = []string{"session_id", "title", "model", "source", "created_at"}

func cmdChatNew(inv *invocation, stdout, stderr io.Writer) int {
	if inv.jsonGiven && len(inv.fields) == 0 {
		return requireFields(inv, stderr) // K2 甲档：给了 --json 不给字段 ⇒ 取自退码表（usage 2）
	}
	body := map[string]any{"source": chatSourceCLI}
	if v := strings.TrimSpace(inv.modelWant); v != "" {
		body["model"] = v
	}
	if v := strings.TrimSpace(inv.flagVal("--title")); v != "" {
		body["title"] = v
	}
	if v := strings.TrimSpace(inv.flagVal("--source")); v != "" {
		body["source"] = v // 显式覆盖缺省 `cli`（✗ 不静默：`--source` 是 §5.2 的取值旗标）
	}
	b, _ := json.Marshal(body)
	var resp jsonObj
	if rc := chatCall(inv, newClient(), http.MethodPost, "/api/chat/sessions", string(b), &resp, stderr); rc != exitOK {
		return rc
	}
	se := asObj(resp["session"])
	if se == nil {
		se = resp
	}
	row := map[string]string{
		"session_id": cell(se["id"]),
		"title":      cell(se["title"]),
		"model":      cell(se["model"]),
		"source":     cell(se["source"]),
		"created_at": chatTimeCell(se["created_at"]),
	}
	if row["session_id"] == "" {
		return chatReportErr(inv, stderr, "blocked", "response_shape_unexpected",
			"主控建会话的响应里没有会话 id ⇒ 不给结论（退码 8）", exitCodeOf("blocked"))
	}
	chatLastWrite(row["session_id"], stderr)
	if inv.jsonGiven {
		return selectJSON(stdout, stderr, inv, inv.path, inv.fields, row)
	}
	fmt.Fprintf(stdout, "%s\n", row["session_id"])
	fmt.Fprintf(stderr, "%s: 建会话成功（source=%s · model=%s）—— 续接：`%s chat send --session %s \"…\"`\n",
		progName, row["source"], row["model"], progName, row["session_id"])
	return exitOK
}

// ---- `chat ls` ---------------------------------------------------------------------------

// chatLsFields —— `--json` 可取字段（§5.4）。
var chatLsFields = []string{"session_id", "title", "model", "source", "created_at",
	"last_activity_at", "message_count", "input_tokens", "output_tokens", "reasoning_tokens",
	"pinned", "archived"}

func cmdChatLs(inv *invocation, stdout, stderr io.Writer) int {
	if inv.jsonGiven && len(inv.fields) == 0 {
		return requireFields(inv, stderr)
	}
	limit := chatListLimitDefault
	if inv.hasFlag("--limit") {
		n, err := strconv.Atoi(strings.TrimSpace(inv.flagVal("--limit")))
		if err != nil || n <= 0 {
			return chatReportErr(inv, stderr, "usage", "bad_limit",
				fmt.Sprintf("`--limit %s` 不是正整数", inv.flagVal("--limit")), exitCodeOf("usage"))
		}
		limit = n
	}
	src := strings.TrimSpace(inv.flagVal("--source"))
	c := newClient()
	var resp jsonObj
	if rc := chatCall(inv, c, http.MethodGet, "/api/chat/sessions", "", &resp, stderr); rc != exitOK {
		return rc
	}
	rows := []map[string]string{}
	for _, it := range asList(resp["sessions"]) {
		o := asObj(it)
		if o == nil {
			continue
		}
		row := map[string]string{
			"session_id":       cell(o["id"]),
			"title":            cell(o["title"]),
			"model":            cell(o["model"]),
			"source":           cell(o["source"]),
			"created_at":       chatTimeCell(o["created_at"]),
			"last_activity_at": chatTimeCell(o["last_activity_at"]),
			"message_count":    cell(o["message_count"]),
			"input_tokens":     cell(o["input_tokens"]),
			"output_tokens":    cell(o["output_tokens"]),
			"reasoning_tokens": cell(o["reasoning_tokens"]),
			"pinned":           cell(o["pinned"]),
			"archived":         cell(o["archived"]),
		}
		// ★ 两处**收窄**在本壳里做（服务端现读 `ListSessions(100)` **不收任何查询参数** ——
		//   `chat_handlers.go:222-230` 现读实据）⇒ 不做就只是「解析了没用」= 静默吞（硬约束⑤）。
		//   ① `--archived`：**不给** ⇒ 隐藏归档（`archived=1` 列表隐藏，与 `SetArchive` 的注释同口径）；
		//   ② `--source <名>`：只留该来源。
		if !inv.hasFlag("--archived") && chatTruthy(row["archived"]) {
			continue
		}
		if src != "" && row["source"] != src {
			continue
		}
		rows = append(rows, row)
	}
	// ★ 列表类机器面的 `meta.total`（`--json` 拍定）= **截尾前**条数（`--limit` 之前 · 过滤之后）
	//   ⇒ `meta.count`（框架自动）与它一比就知道「本地收窄吃掉了多少条」。
	inv.metaAddJSON("total", strconv.Itoa(len(rows)))
	if len(rows) > limit {
		rows = rows[:limit]
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i]["session_id"] > rows[j]["session_id"] })
	srcWord := "全部"
	if src != "" {
		srcWord = src
	}
	fmt.Fprintf(stderr, "%s: 会话 %d 条（服务端现读上限 100 · 本地 `--limit %d` · 归档%s · 来源=%s）\n",
		progName, len(rows), limit, chatOnOff(inv.hasFlag("--archived")), srcWord)
	if inv.jsonGiven {
		return selectJSONList(stdout, stderr, inv, inv.path, inv.fields, rows)
	}
	display := []string{"session_id", "title", "model", "source", "message_count"}
	table := make([][]string, 0, len(rows))
	for _, r := range rows {
		table = append(table, []string{r["session_id"], r["title"], r["model"], r["source"], r["message_count"]})
	}
	renderRows(stdout, stderr, inv.tty && !inv.plain, display, table)
	return exitOK
}

// ---- `chat send`（本族主写面 · SSE · `--json`/非 TTY/`--no-stream` ⇒ 非流式）-------------------

// chatSendFields —— `--json` 可取字段（§5.4 扁平化；token 三格逐字取 `done` 载荷键）。
var chatSendFields = []string{"session_id", "message_id", "role", "content", "reasoning",
	"tool_calls", "title", "interrupted", "steer_undrained",
	"input_tokens", "output_tokens", "reasoning_tokens"}

// chatTurn —— 一轮的收束面（服务端事件流的**归并结果**；✗ 壳里不产生任何语义）。
type chatTurn struct {
	MessageID      string
	Title          string
	Output         strings.Builder
	Reasoning      strings.Builder
	Tools          []string
	SteerUndrained []string
	InputTokens    string
	OutputTokens   string
	ReasonTokens   string
	Interrupted    bool
	Err            string
	Ended          bool
}

func cmdChatSend(inv *invocation, stdout, stderr io.Writer) int {
	if inv.jsonGiven && len(inv.fields) == 0 {
		return requireFields(inv, stderr)
	}
	prompt, rc := chatPromptFromInv(inv, stderr)
	if rc != exitOK {
		return rc
	}
	// 会话来自哪一枚（✗ 三枚都不给 ⇒ 用法错 2：命令面无常驻 REPL ⇒ 必须点名）：
	sid := ""
	if inv.hasFlag("--new") {
		if inv.hasFlag("--session") || inv.last || inv.hasFlag("--last-in-cwd") {
			return chatReportErr(inv, stderr, "usage", "new_and_session_conflict",
				"`--new`（新开一个会话）与 `--session`/`--last`/`--last-in-cwd` 不许同给", exitCodeOf("usage"))
		}
		id, rc := chatCreateSession(inv, stderr)
		if rc != exitOK {
			return rc
		}
		sid = id
	} else {
		id, rc := chatSessionFromInv(inv, stderr, "")
		if rc != exitOK {
			return rc
		}
		sid = id
	}
	imgs, rc := chatImagesToDataURLs(inv, inv.flagVals("--images"), stderr)
	if rc != exitOK {
		return rc
	}
	// ★ 流式 / 非流式**唯一**的分档（§7.5 + §5.5）：
	//   · `--json` **强制**非流式（`O-3`＋`O-25`：包封单 blob · 门㉖：TTY/非 TTY `--json` 逐字相同）；
	//   · `--no-stream` 显式非流式；
	//   · **非 TTY**（stdout 不是终端）⇒ 非流式（可管道 · §5.5 ①）。
	streaming := !inv.jsonGiven && !inv.hasFlag("--no-stream") && inv.tty
	markdown := inv.hasFlag("--markdown") && !inv.jsonGiven && inv.tty

	body := map[string]any{"content": prompt}
	if len(imgs) > 0 {
		body["images"] = imgs
	}
	bb, _ := json.Marshal(body)

	c := newClient()
	turn, rc := chatPostTurn(inv, c, sid, string(bb), streaming, markdown, stdout, stderr)
	// ★ `O-17`：`turn != nil` 而 `rc != exitOK` ⇒ **服务端报 `done{interrupted:true}` 而本端没按过停**那一档
	//   （见 `chatPostTurn` 里的 `O-17` 分支）：结果面照出（包封 / 正文 ＋ `interrupted:"true"`），退码按它收
	//   —— ✗ 不当成功档（`rc` 一直带到本函数末尾那一处 `return`）。
	if rc != exitOK && turn == nil {
		return rc
	}
	chatLastWrite(sid, stderr)
	row := map[string]string{
		"session_id":       sid,
		"message_id":       turn.MessageID,
		"role":             "assistant",
		"content":          turn.Output.String(),
		"reasoning":        turn.Reasoning.String(),
		"tool_calls":       chatJoinNonEmpty(turn.Tools, " | "),
		"title":            turn.Title,
		"interrupted":      strconv.FormatBool(turn.Interrupted),
		"steer_undrained":  chatJoinNonEmpty(turn.SteerUndrained, "\n"),
		"input_tokens":     turn.InputTokens,
		"output_tokens":    turn.OutputTokens,
		"reasoning_tokens": turn.ReasonTokens,
	}
	if inv.jsonGiven {
		if r := selectJSON(stdout, stderr, inv, inv.path, inv.fields, row); r != exitOK {
			return r
		}
		return rc
	}
	if streaming {
		fmt.Fprintf(stderr, "\n#%s · msg %s · in %s / out %s / think %s\n",
			sid, orDash(turn.MessageID), orDash(turn.InputTokens), orDash(turn.OutputTokens), orDash(turn.ReasonTokens))
	} else {
		out := row["content"]
		if markdown {
			out = chatMarkdownRender(out)
		}
		fmt.Fprint(stdout, out)
		if !strings.HasSuffix(out, "\n") {
			fmt.Fprintln(stdout)
		}
		fmt.Fprintf(stderr, "#%s · msg %s\n", sid, orDash(turn.MessageID))
	}
	return rc
}

// chatPostTurn —— 打一轮并**按同一套事件读法**收尾（流式逐字上 stdout / 非流式攒着一次出）。
//
// ★ 一条路径，两种渲染（§7.5 逐字「这**不是**在造第二条路径」）。
func chatPostTurn(inv *invocation, c *client, sid, body string, streaming, markdown bool, stdout, stderr io.Writer) (*chatTurn, int) {
	// `--wait`（§5.2）：撞租约（409）时**排队重试**而不是立刻退 14。默认档**立刻**报 14（✗ 不隐含排队）。
	deadline := time.Now().Add(30 * time.Minute)
	for attempt := 0; ; attempt++ {
		req, err := http.NewRequest(http.MethodPost, c.base+"/api/chat/sessions/"+sid+"/send", strings.NewReader(body))
		if err != nil {
			return nil, chatReportErr(inv, stderr, "usage", "bad_request", err.Error(), exitCodeOf("usage"))
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "text/event-stream")
		req.Header.Set("X-Auth-Token", c.token)
		// ★ 流式请求**不设** `http.Client.Timeout`（`Timeout` 是**整个请求**的钟 —— 会把长轮次腰斩）：
		//   上界走 §九 M8 的全局墙钟（`--timeout` ⇒ 派发层 `wallclock.go` ⇒ 到点退 11）＋ SIGINT（130）。
		resp, err := chatStreamClient.Do(req)
		if err != nil {
			return nil, chatReportErr(inv, stderr, "unreachable", "dial_failed",
				fmt.Sprintf("打不到主控 %s/send：%v", c.base, err), exitCodeOf("unreachable"))
		}
		if resp.StatusCode == http.StatusConflict && inv.wait && time.Now().Before(deadline) {
			resp.Body.Close()
			fmt.Fprintf(stderr, "%s: 会话 %s 已有轮次在跑（409）—— 按 `--wait` **排队重试**（第 %d 次）\n",
				progName, sid, attempt+1)
			time.Sleep(2 * time.Second)
			continue
		}
		if resp.StatusCode >= 400 {
			raw, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			kind, detail, rc := chatErrToCode(resp.StatusCode)
			extra := ""
			if resp.StatusCode == http.StatusConflict {
				extra = "（§5.3：会话租约 409 ⇒ 14；要排队就给 `--wait`）"
			}
			return nil, chatReportErr(inv, stderr, kind, detail,
				fmt.Sprintf("主控 /send 返回 http_%d%s · 原样响应: %s",
					resp.StatusCode, extra, firstLine(strings.TrimSpace(string(raw)))), rc)
		}
		defer resp.Body.Close()
		turn, err := chatReadSSE(resp.Body, streaming, markdown, stdout, stderr)
		if err != nil {
			return nil, chatReportErr(inv, stderr, "blocked", "stream_truncated",
				fmt.Sprintf("读 /send 的事件流中断：%v ⇒ 不给结论（退码 8）", err), exitCodeOf("blocked"))
		}
		if turn.Err != "" {
			// `error` 事件 = 服务端把这一轮判成失败（或中断且落库也失败）——
			// ★ `O-17`：**本端没按过停**而服务端报 interrupted ⇒ 归 `1`（130 只留给「本端用户按的停」）。
			rc := exitCodeOf("failed")
			if turn.Interrupted {
				rc = exitCodeOf("failed")
			}
			return nil, chatReportErr(inv, stderr, "failed", "turn_error",
				fmt.Sprintf("服务端把这一轮判成失败：%s", turn.Err), rc)
		}
		// ★ `O-17`（2026-10-17 已拍）：服务端**只发** `done{interrupted:true}`（`turn.Err == ""`）而
		//   **本端没按过停** ⇒ 归 `1` —— ✗ 不许落回成功档 `0`（那正是本条缺口的现读形态）。
		//   「本端按过停」那条路**不在这里**：它是 `main.go` 的全局信号处理器（`os.Exit(exitInterrupted)` ⇒ 130）
		//   ⇒ 能走到本行即证明本端没按过 ⇒ 这一档一律归 `1`；`130` 只留给「本端用户按的停」。
		//   ★ 这一档**仍把 `turn` 交回调用方**（结果面要照出 `interrupted:"true"` ＋ 已收到的正文），
		//   由调用方出完结果面、再按本退码收尾（✗ 不在这里把结果面吞掉）。
		if turn.Interrupted {
			rc := exitCodeOf("failed")
			// 人面说明 ＋ 机器面 `error.kind`（族内唯一口 = `chatReportErr`）；★ 它的返回值就是本分支的码，
			// 本分支自己收尾（✗ 不采用它的返回值去退 —— 那会把结果面提前退掉）。
			_ = chatReportErr(inv, stderr, "failed", "turn_interrupted",
				"这一轮被中断（interrupted:true · ✗ 本端没按过停 ⇒ 被其它客户端 / 服务端中断）⇒ 照 `O-17` 归 `1`（130 只留给本端用户按的停）", rc)
			return turn, rc
		}
		return turn, exitOK
	}
}

// chatStreamClient —— 流式面专用客户端（✗ 不设 `Timeout` —— 见 `chatPostTurn` 里的那条口径）。
var chatStreamClient = &http.Client{}

// chatReadSSE —— 读一条 SSE 流（§7.1 分帧 + §7.2 事件全表）。
//
// ★ 分帧口径写死（照 SSE 规范与 §7.1）：**空行结束一帧**；帧内**全部** `data:` 行按 `\n` 拼接
// （服务端现读每帧只写一行，但客户端**必须**按规范收全 —— ✗ 只取第一行）。
func chatReadSSE(r io.Reader, streaming, markdown bool, stdout, stderr io.Writer) (*chatTurn, error) {
	turn := &chatTurn{}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	evName := ""
	var dataLines []string
	frameFlush := func() {
		if len(dataLines) > 0 {
			chatHandleEvent(turn, evName, strings.Join(dataLines, "\n"), streaming, markdown, stdout, stderr)
		}
		evName, dataLines = "", nil
	}
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		if line == "" {
			frameFlush()
			continue
		}
		switch {
		case strings.HasPrefix(line, "event:"):
			evName = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			dataLines = append(dataLines, strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		}
	}
	frameFlush()
	return turn, sc.Err()
}

// chatHandleEvent —— 逐类事件的处理（§7.2 的 13 类全在册；✗ 未登记的类不静默 —— 见 default 支）。
func chatHandleEvent(turn *chatTurn, evName, data string, streaming, markdown bool, stdout, stderr io.Writer) {
	var o jsonObj
	if strings.TrimSpace(data) != "" {
		_ = json.Unmarshal([]byte(data), &o)
	}
	switch evName {
	case "delta":
		// `{type: "reasoning"|"output", text}`（`type` 闭集见 §7.2 第 1 行）。
		typ, text := cell(o["type"]), cell(o["text"])
		if typ == "output" {
			turn.Output.WriteString(text)
			if streaming {
				if markdown {
					fmt.Fprint(stdout, chatMarkdownRenderChunk(text))
				} else {
					fmt.Fprint(stdout, text)
				}
			}
		} else {
			// ★ `O-9`/§7.3：思考**不丢**（进机器面），默认**不显**（本版 ✗ 不加 `--show-reasoning`）。
			turn.Reasoning.WriteString(text)
		}
	case "done":
		turn.Ended = true
		turn.MessageID = cell(o["message_id"])
		turn.Title = cell(o["title"])
		turn.InputTokens = cell(o["input_tokens"])
		turn.OutputTokens = cell(o["output_tokens"])
		turn.ReasonTokens = cell(o["reasoning_tokens"])
		if v, ok := o["interrupted"]; ok && chatTruthy(cell(v)) {
			turn.Interrupted = true
		}
		for _, s := range asList(o["steer_undrained"]) {
			turn.SteerUndrained = append(turn.SteerUndrained, cell(s))
		}
	case "error":
		turn.Err = cell(o["error"])
		if v, ok := o["interrupted"]; ok && chatTruthy(cell(v)) {
			turn.Interrupted = true
		}
	case "tool_start":
		turn.Tools = append(turn.Tools, `{"name":`+jstr(cell(o["name"]))+`,"status":"start"}`)
	case "tool":
		turn.Tools = append(turn.Tools, chatToolCallJSON(o))
	case "tool_ping", "compacting", "compact_done", "retry", "retry_done", "loop_hint", "resume", "checkpoint_failed":
		// ★ 这九类**首版只在人面报一行进度**（§7.3/§8.2 的进度面）：正文只由 `delta{output}` 组装。
		//   ★ 与「未消费事件必须显式登记」（§7.10 第 4 条）同一方向：**列名在此**，✗ 不落 `_ => {}` 静默。
		if msg := chatProgressLine(evName, o); msg != "" {
			// ★ stdout **只放正文/协议帧**（§5.5 ②：stdout 纯净）—— 进度一律走 stderr。
			fmt.Fprintf(stderr, "%s\n", msg)
		}
	default:
		// 契约定源件（`G-B4` · §7.10）尚未落件 ⇒ 本版把「本壳不认的事件名」**点名**（✗ 不静默吞）：
		// 落 `turn.Tools` 里一格（机器面可见），人面不打印。
		turn.Tools = append(turn.Tools, `{"event":`+jstr(evName)+`,"unregistered":true}`)
	}
}

// chatProgressLine —— 九类进度事件的人面一句话（`§7.3`；空串 = 本版不显）。
func chatProgressLine(evName string, o jsonObj) string {
	switch evName {
	case "tool_ping":
		return fmt.Sprintf("… %s 跑了 %ss", cell(o["name"]), cell(o["elapsed"]))
	case "compacting":
		return "… 正在压缩历史"
	case "compact_done":
		return "… 压缩完成"
	case "retry":
		return fmt.Sprintf("… 第 %s 次重试（%s）", cell(o["attempt"]), cell(o["reason"]))
	case "retry_done":
		return "… 重试成功"
	case "loop_hint":
		return fmt.Sprintf("⚠ 环提示 %s", cell(o["kind"]))
	case "resume":
		return fmt.Sprintf("… 从第 %s 步续跑", cell(o["from_step"]))
	case "checkpoint_failed":
		return fmt.Sprintf("⚠ 检查点失败 %s", cell(o["step"]))
	}
	return ""
}

// chatToolCallJSON —— `tool` 事件的一格（`{name,args,result}` · 逐字取载荷键 · ✗ 不重排内容）。
func chatToolCallJSON(o jsonObj) string {
	args := cell(o["args"])
	if args != "" && !json.Valid([]byte(args)) {
		args = jstr(args)
	}
	if args == "" {
		args = `null`
	}
	return `{"name":` + jstr(cell(o["name"])) + `,"args":` + args + `,"result":` + jstr(cell(o["result"])) + `}`
}

// ---- `chat show` -------------------------------------------------------------------------

// chatShowFields —— `--json` 可取字段（§5.4 扁平化：逐条消息一行 + 会话面重复列）。
var chatShowFields = []string{"session_id", "title", "model", "source", "message_count",
	"message_id", "role", "content", "reasoning", "active", "timestamp", "tool_calls",
	"text", "around_id", "limit"}

func cmdChatShow(inv *invocation, stdout, stderr io.Writer) int {
	if inv.jsonGiven && len(inv.fields) == 0 {
		return requireFields(inv, stderr)
	}
	pos := ""
	if len(inv.args) > 0 {
		pos = inv.args[0]
	}
	sid, rc := chatSessionFromInv(inv, stderr, pos)
	if rc != exitOK {
		return rc
	}
	limit := chatWindowLimitDefault
	if inv.hasFlag("--limit") {
		n, err := strconv.Atoi(strings.TrimSpace(inv.flagVal("--limit")))
		if err != nil || n <= 0 {
			return chatReportErr(inv, stderr, "usage", "bad_limit",
				fmt.Sprintf("`--limit %s` 不是正整数", inv.flagVal("--limit")), exitCodeOf("usage"))
		}
		limit = n
	}
	// ★ `--window <mid>`：走服务端 `GET /{id}/window?around_id=…&limit=…`（召回指针回到原文 ·
	//   §5.1 第 4 行）——**✗ 不在壳里重算窗口**（逻辑不重写 · §9.1 第 3 条）。
	if inv.hasFlag("--window") {
		mid := strings.TrimSpace(inv.flagVal("--window"))
		if _, err := strconv.ParseInt(mid, 10, 64); err != nil {
			return chatReportErr(inv, stderr, "usage", "bad_window",
				fmt.Sprintf("`--window %s` 应为消息序号（`around_id`）", mid), exitCodeOf("usage"))
		}
		path := fmt.Sprintf("/api/chat/sessions/%s/window?around_id=%s&limit=%d", sid, mid, limit)
		var resp jsonObj
		if rc := chatCall(inv, newClient(), http.MethodGet, path, "", &resp, stderr); rc != exitOK {
			return rc
		}
		row := map[string]string{
			"session_id": sid,
			"text":       cell(resp["text"]),
			"around_id":  cell(resp["around_id"]),
			"limit":      cell(resp["limit"]),
		}
		if inv.jsonGiven {
			return selectJSON(stdout, stderr, inv, inv.path, inv.fields, row)
		}
		fmt.Fprintln(stdout, row["text"])
		return exitOK
	}
	var resp jsonObj
	if rc := chatCall(inv, newClient(), http.MethodGet, "/api/chat/sessions/"+sid, "", &resp, stderr); rc != exitOK {
		return rc
	}
	se := asObj(resp["session"])
	if se == nil {
		se = resp
	}
	msgs := asList(resp["messages"])
	rows := []map[string]string{}
	for _, it := range msgs {
		o := asObj(it)
		if o == nil {
			continue
		}
		rows = append(rows, map[string]string{
			"session_id": cell(se["id"]),
			"title":      cell(se["title"]),
			"model":      cell(o["model"]),
			"source":     cell(se["source"]),
			"message_id": cell(o["id"]),
			"role":       cell(o["role"]),
			"content":    cell(o["content"]),
			"reasoning":  cell(o["reasoning"]),
			"active":     cell(o["active"]),
			"timestamp":  chatTimeCell(o["timestamp"]),
			"tool_calls": cell(o["tool_calls"]),
		})
	}
	// ★ `message_count` = **该会话的消息总条数**（截尾前）—— ✗ 不是「本页行数」（修前口径写的是截尾后，
	//   那会让 `message_count` 随 `--limit` 变，与 §5.4 的「会话面重复列」语义相左）。
	total := len(rows)
	inv.metaAddJSON("total", strconv.Itoa(total))
	if len(rows) > limit {
		rows = rows[len(rows)-limit:]
	}
	for _, r := range rows {
		r["message_count"] = strconv.Itoa(total)
	}
	if inv.jsonGiven {
		return selectJSONList(stdout, stderr, inv, inv.path, inv.fields, rows)
	}
	display := []string{"message_id", "role", "timestamp", "content"}
	table := make([][]string, 0, len(rows))
	for _, r := range rows {
		table = append(table, []string{r["message_id"], r["role"], r["timestamp"], r["content"]})
	}
	renderRows(stdout, stderr, inv.tty && !inv.plain, display, table)
	fmt.Fprintf(stderr, "%s: 会话 %s · 消息 %d 条（`--limit` 本地截尾为 %d）\n", progName, sid, len(msgs), limit)
	return exitOK
}

// ---- `chat abort` ------------------------------------------------------------------------

// chatAbortFields —— `--json` 可取字段（§5.4）。
var chatAbortFields = []string{"session_id", "aborted", "reason"}

func cmdChatAbort(inv *invocation, stdout, stderr io.Writer) int {
	if inv.jsonGiven && len(inv.fields) == 0 {
		return requireFields(inv, stderr)
	}
	if len(inv.args) == 0 {
		return chatReportErr(inv, stderr, "usage", "missing_id",
			"`zerg chat abort <id>` 缺会话 id", exitCodeOf("usage"))
	}
	sid, ok := chatNormalizeSessionRef(inv.args[0])
	if !ok {
		return chatReportErr(inv, stderr, "usage", "bad_session_id",
			fmt.Sprintf("会话 id %q 形状不合法（只认完整 id）", inv.args[0]), exitCodeOf("usage"))
	}
	var resp jsonObj
	if rc := chatCall(inv, newClient(), http.MethodPost, "/api/chat/sessions/"+sid+"/abort", "{}", &resp, stderr); rc != exitOK {
		return rc
	}
	row := map[string]string{
		"session_id": sid,
		"aborted":    cell(resp["aborted"]),
		"reason":     cell(resp["reason"]),
	}
	if inv.jsonGiven {
		return selectJSON(stdout, stderr, inv, inv.path, inv.fields, row)
	}
	fmt.Fprintf(stdout, "%s\n", sid)
	fmt.Fprintf(stderr, "%s: abort 已发出（aborted=%s%s）—— 已流出的部分内容由服务端落库（附「（已中断）」）\n",
		progName, orDash(row["aborted"]), chatDashOr(row["reason"]))
	return exitOK
}

// ---- `chat steer` ------------------------------------------------------------------------

// chatSteerFields —— `--json` 可取字段（§5.4 · `O-4` 订正后的字段名 = `steer_queued`）。
var chatSteerFields = []string{"session_id", "steer_queued", "turn_running"}

func cmdChatSteer(inv *invocation, stdout, stderr io.Writer) int {
	if inv.jsonGiven && len(inv.fields) == 0 {
		return requireFields(inv, stderr)
	}
	if len(inv.args) < 2 {
		return chatReportErr(inv, stderr, "usage", "missing_steer_args",
			"`zerg chat steer <id> <文字>` 缺会话 id 或插话文字", exitCodeOf("usage"))
	}
	sid, ok := chatNormalizeSessionRef(inv.args[0])
	if !ok {
		return chatReportErr(inv, stderr, "usage", "bad_session_id",
			fmt.Sprintf("会话 id %q 形状不合法（只认完整 id）", inv.args[0]), exitCodeOf("usage"))
	}
	text := strings.TrimSpace(strings.Join(inv.args[1:], " "))
	if text == "" {
		return chatReportErr(inv, stderr, "usage", "missing_steer_text",
			"插话文字是空的（服务端会拒 `content` 为空 · 见 `chat_handlers.go:130`）", exitCodeOf("usage"))
	}
	bb, _ := json.Marshal(map[string]any{"content": text})
	var resp jsonObj
	if rc := chatCall(inv, newClient(), http.MethodPost, "/api/chat/sessions/"+sid+"/steer", string(bb), &resp, stderr); rc != exitOK {
		return rc
	}
	row := map[string]string{
		"session_id":   sid,
		"steer_queued": cell(resp["steer_queued"]),
		"turn_running": cell(resp["turn_running"]),
	}
	if inv.jsonGiven {
		return selectJSON(stdout, stderr, inv, inv.path, inv.fields, row)
	}
	fmt.Fprintf(stdout, "%s\n", sid)
	// ★ 插话**必须有回执**（§7.9 末条）：`turn_running=false` ⇒ 服务端**不入库**（避免误挂到未来轮次）
	//   ⇒ 人面**如实说**「没挂上」，✗ 不许假装插进去了。
	if chatTruthy(row["turn_running"]) {
		fmt.Fprintf(stderr, "%s: 插话已挂（steer_queued=%s）—— 挂**下一次工具结果**\n", progName, orDash(row["steer_queued"]))
	} else {
		fmt.Fprintf(stderr, "%s: ⚠ 该会话此刻**没有运行中的轮次** ⇒ 服务端**没入库**（steer_queued=0）—— 想说的话请走 `chat send`（生成中按回车默认排队见 §7.9 · `O-14`）\n", progName)
	}
	return exitOK
}

// ---- `chat rm`（写面 ⇒ fail-closed：`--dry-run` 出计划件 / 缺 `--yes` ⇒ 2）-------------------

// chatRmFields —— `--json` 可取字段（§5.4）。`dry_run` 登表：干跑档恒有（`"true"`）、真写档见 `cmdChatRm` 该档 row。
var chatRmFields = []string{"session_id", "deleted", "dry_run"}

func cmdChatRm(inv *invocation, stdout, stderr io.Writer) int {
	if inv.jsonGiven && len(inv.fields) == 0 {
		return requireFields(inv, stderr)
	}
	// ★ 两道确认档自相矛盾 ⇒ 共享判定（与 `guard.go` 同一处 · ✗ 不另写一句判词）。
	if rc := dryRunYesConflict(inv, stderr); rc != exitOK {
		return rc
	}
	if len(inv.args) == 0 {
		return chatReportErr(inv, stderr, "usage", "missing_id",
			"`zerg chat rm <id>` 缺会话 id", exitCodeOf("usage"))
	}
	sid, ok := chatNormalizeSessionRef(inv.args[0])
	if !ok {
		return chatReportErr(inv, stderr, "usage", "bad_session_id",
			fmt.Sprintf("会话 id %q 形状不合法（只认完整 id）", inv.args[0]), exitCodeOf("usage"))
	}
	// ① `--dry-run`：只出**计划件**（零副作用 —— ✗ 不碰主控、✗ 不删任何东西）。
	if inv.dryRun {
		// ★ `J-5`（2026-10-17 已拍）：给了 `--json` ⇒ 计划件走 **stderr**、stdout 只留六键包封
		//   （stdout 是结果面 —— 混进人面计划件会让消费侧解析不到包封 · 门㉖ 两态也因此可比）。
		//   同族先例 = `script inventory sync` 的 `planOut := stdout; if inv.jsonGiven { planOut = stderr }`。
		planOut := stdout
		if inv.jsonGiven {
			planOut = stderr
		}
		fmt.Fprintln(planOut, "计划件（--dry-run · 零副作用 —— 未执行、未改任何状态）")
		fmt.Fprintf(planOut, "  动作     : %s chat rm\n", progName)
		fmt.Fprintf(planOut, "  危险档   : D2（硬删一个会话 —— **不可逆**：消息随之消失）\n")
		fmt.Fprintf(planOut, "  目标     : %s\n", sid)
		fmt.Fprintf(planOut, "  请求     : DELETE %s/api/chat/sessions/%s\n", newClient().base, sid)
		fmt.Fprintf(planOut, "  它会动   : 服务端**硬删**该会话与其消息（`DeleteSession` → `deleted:true`）\n")
		fmt.Fprintf(planOut, "  执行要   : --yes\n")
		fmt.Fprintf(planOut, "  来源     : 设计稿 §5.1 第 7 行 · §5.2（写面 fail-closed）· §5.3（退码取既有表）\n")
		fmt.Fprintln(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未执行、未改任何状态）")
		// ★ `meta.changed` = **本次调用**是否真改变了状态（`idemSpec.Changed` · §九 M4）⇒ 干跑零副作用
		//   必须逐字 `false`；✗ 不许靠命令表派生（派生支 = `cmd.idem.Changed && cmd.danger != nil` ⇒
		//   危险命令恒 `true` · `main.go:5943-5944`）。
		//   同族先例：`agent registry --add --dry-run`（`family_agent_ops.go:735-736`）· `agent reload --dry-run`（`:314-315`）。
		changed := false
		inv.changed = &changed
		if inv.jsonGiven {
			// ★ 机器面：干跑也走 `selectJSON`（与真写档**同一处出口** ⇒ 形状一致）。
			//   `deleted:"false"` = 干跑**什么都没删**；`dry_run:"true"` = 这一档的标识。
			//   ★ 字段表仍以 `chatRmFields` 为准 —— 没登进表里的键**不会**出现在 `items` 里（✗ 不靠行里多塞键绕字段表）。
			return selectJSON(stdout, stderr, inv, inv.path, inv.fields,
				map[string]string{"session_id": sid, "deleted": "false", "dry_run": "true"})
		}
		return exitOK
	}
	// ② 确认档 fail-closed（缺 `--yes` ⇒ 2 · **零副作用** · 从不提问 · 无 TTY 也一样）。
	if !inv.yes {
		inv.setErr("usage", "yes_required", "D2 档缺 --yes ⇒ 不执行")
		guardMarkDecision(inv, guardDecisionApproval)
		fmt.Fprintf(stderr, "%s: `chat rm` 是 D2 档 —— **缺 --yes ⇒ 不执行**（§九 M3 C2 fail-closed）\n", progName)
		fmt.Fprintf(stderr, "先看计划件：%s chat rm %s --dry-run\n", progName, sid)
		fmt.Fprintf(stderr, "error.kind=usage · detail=yes_required · retryable=false · remedy=fix_usage\n")
		return exitCodeOf("usage")
	}
	var resp jsonObj
	if rc := chatCall(inv, newClient(), http.MethodDelete, "/api/chat/sessions/"+sid, "", &resp, stderr); rc != exitOK {
		return rc
	}
	row := map[string]string{"session_id": sid, "deleted": cell(resp["deleted"]), "dry_run": "false"}
	if inv.jsonGiven {
		return selectJSON(stdout, stderr, inv, inv.path, inv.fields, row)
	}
	fmt.Fprintf(stdout, "%s\n", sid)
	fmt.Fprintf(stderr, "%s: 会话 %s 已删（deleted=%s）—— 硬删，不可逆\n", progName, sid, orDash(row["deleted"]))
	return exitOK
}

// ---- 小工具（本件内共享；✗ 不引第三方库）----------------------------------------------------

// chatCreateSession —— 建会话（`chat send --new` 用；`chat new` 走 `cmdChatNew` —— **同一处**请求体）。
func chatCreateSession(inv *invocation, stderr io.Writer) (string, int) {
	body := map[string]any{"source": chatSourceCLI}
	if v := strings.TrimSpace(inv.modelWant); v != "" {
		body["model"] = v
	}
	b, _ := json.Marshal(body)
	var resp jsonObj
	if rc := chatCall(inv, newClient(), http.MethodPost, "/api/chat/sessions", string(b), &resp, stderr); rc != exitOK {
		return "", rc
	}
	se := asObj(resp["session"])
	if se == nil {
		se = resp
	}
	id := cell(se["id"])
	if id == "" {
		return "", chatReportErr(inv, stderr, "blocked", "response_shape_unexpected",
			"主控建会话的响应里没有会话 id ⇒ 不给结论（退码 8）", exitCodeOf("blocked"))
	}
	fmt.Fprintf(stderr, "%s: `--new` 已建会话 %s（source=%s）\n", progName, id, chatSourceCLI)
	return id, exitOK
}

// chatPromptFromInv —— 取提示（§5.5 ③：`-` = 从 stdin 读，**零 shell 解释**）。
func chatPromptFromInv(inv *invocation, stderr io.Writer) (string, int) {
	if len(inv.args) == 0 {
		return "", chatReportErr(inv, stderr, "usage", "missing_prompt",
			"`zerg chat send <提示>` 缺提示（管道档：`echo \"…\" | zerg chat send -`）", exitCodeOf("usage"))
	}
	if inv.args[0] == "-" {
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", chatReportErr(inv, stderr, "usage", "content_unreadable",
				fmt.Sprintf("`-` 档从 stdin 读提示失败：%v", err), exitCodeOf("usage"))
		}
		s := strings.TrimRight(string(b), "\n")
		if strings.TrimSpace(s) == "" {
			return "", chatReportErr(inv, stderr, "usage", "missing_prompt",
				"`-` 档从 stdin 读到的提示是空的", exitCodeOf("usage"))
		}
		// ★ **零 shell 解释**（§5.5 ③ 逐字）：引号 / `$(...)` / 反引号**原样保留** —— 本函数不做任何转义处理。
		return s, exitOK
	}
	joined := strings.TrimSpace(strings.Join(inv.args, " "))
	if joined == "" {
		return "", chatReportErr(inv, stderr, "usage", "missing_prompt",
			"提示是空的（§5.5 ③：管道档用 `-`）", exitCodeOf("usage"))
	}
	return joined, exitOK
}

// chatMarkdownRender —— `--markdown` 的**最小渲染档**（`O-9`：默认纯文本 ＋ 本旗标 opt-in）。
//
// 本版只渲四类（✗ 不做表格/链接/嵌套/图片）：标题 → 粗体；行内 `**粗**` / 反引号 → ANSI；
// 有序/无序列表 → 原样保留缩进；围栏代码块 → 四空格缩进 + 不解释行内标记。
// ★ 只在 **TTY ＋ 非 `--json`** 那一档生效（`--json` 面恒为包封，门㉖ 两态逐字相同）。
func chatMarkdownRender(s string) string {
	if s == "" {
		return s
	}
	var b strings.Builder
	inFence := false
	for _, ln := range strings.Split(s, "\n") {
		trimmed := strings.TrimSpace(ln)
		if strings.HasPrefix(trimmed, "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			b.WriteString("    " + ln + "\n")
			continue
		}
		if strings.HasPrefix(ln, "#") {
			h := strings.TrimLeft(ln, "#")
			if strings.HasPrefix(h, " ") {
				b.WriteString("\x1b[1m" + chatInlineFmt(strings.TrimSpace(h)) + "\x1b[0m\n")
				continue
			}
		}
		b.WriteString(chatInlineFmt(ln) + "\n")
	}
	out := b.String()
	return strings.TrimSuffix(out, "\n")
}

// chatMarkdownRenderChunk —— 流式档的**逐块**渲染：只做行内标记（✗ 不跨块判围栏 —— 块边界不可知）。
func chatMarkdownRenderChunk(s string) string { return chatInlineFmt(s) }

// chatInlineFmt —— 行内两枚标记（`**粗**` → ANSI 粗体 · 反引号 → 反显）。✗ 不解释别的语法。
func chatInlineFmt(s string) string {
	for {
		i := strings.Index(s, "**")
		if i < 0 {
			break
		}
		j := strings.Index(s[i+2:], "**")
		if j < 0 {
			break
		}
		s = s[:i] + "\x1b[1m" + s[i+2:i+2+j] + "\x1b[0m" + s[i+2+j+2:]
	}
	for {
		i := strings.Index(s, "`")
		if i < 0 {
			break
		}
		j := strings.Index(s[i+1:], "`")
		if j < 0 {
			break
		}
		s = s[:i] + "\x1b[7m" + s[i+1:i+1+j] + "\x1b[0m" + s[i+1+j+1:]
	}
	return s
}

// chatTruthy —— 载荷里真值的三态（`true` / `1` / `"true"` 都算真；**取不到 ⇒ 假**，✗ 不当真）。
func chatTruthy(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "true", "1", "yes":
		return true
	}
	return false
}

// chatTimeCell —— 时间格（服务端有 `RFC3339` 串与整秒两种形态；✗ 不猜时区、认不出就原样给）。
func chatTimeCell(v any) string {
	s := cell(v)
	if s == "" {
		return ""
	}
	if n, err := strconv.ParseInt(s, 10, 64); err == nil && n > 0 {
		return time.Unix(n, 0).Format("2006-01-02 15:04:05")
	}
	return s
}

// chatDashOr —— 空 ⇒ `（无）`（人面读法；✗ 不落空串）。
func chatDashOr(s string) string {
	if strings.TrimSpace(s) == "" {
		return ""
	}
	return " · " + s
}

// chatOnOff —— 布尔档的人面词。
func chatOnOff(b bool) string {
	if b {
		return "含"
	}
	return "隐藏"
}

// chatJoinNonEmpty —— 只拼非空项（空 ⇒ 空串 —— 与「在册缺值 ⇒ 整键略去」同一条铁律）。
func chatJoinNonEmpty(xs []string, sep string) string {
	out := []string{}
	for _, x := range xs {
		if strings.TrimSpace(x) != "" {
			out = append(out, x)
		}
	}
	return strings.Join(out, sep)
}
