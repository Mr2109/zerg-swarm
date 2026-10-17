// family_chat_repl.go —— `zerg chat` 裸跑的**行式 REPL**（T9 落件 · `O-7` 已拍：Go × 非全屏行式）。
//
// ★ 它是什么（一句话）：裸 `zerg chat`（**有 TTY** 且无位置参数）进来以后，
// **读一行 ⇒ 当一轮 user 消息发出去 ⇒ 逐块打响应**。
//
// ★ 形态 = **非全屏行式**（✗ 全屏 TUI · ✗ 光标定位画面 · ✗ 接管 alt-screen）：
//
//	· stdout 只放**正文**（assistant 的流）—— 与 `chat send` 的流式档同一个出口；
//	· 提示符 / 回显 / 进度 / 错误一律 stderr —— 与 §5.5 ②「stdout 纯净」同口径；
//	· 骨架**手写**：✗ 不引 `bubbletea` / `golang.org/x/term`（本仓零新依赖；
//	  与 `family_approve.go` 的 `sttyEcho` 同一套办法 —— 借系统的 `stty`）。
//
// ★ 复用面（✗ 一条 HTTP / 一个事件读法都不重写）：
//
//	· 发一轮 = `chatPostTurn`（与 `chat send` **同一条**路：`POST /api/chat/sessions/{id}/send` ＋ SSE 读法）；
//	· 建会话 = `chatCreateSession`（与 `chat new` 同一条请求体）；
//	· 错误分档 = `chatReportErr` / `chatErrToCode`；会话 id 形状 = `chatNormalizeSessionRef`（`O-12`）。
//
// ★ 行为契约（逐条照 `family_chat.go` 的 `cmdChatRoot` / `chatEnterREPL` 注释）：
//
//	① 空行 / 全空白 ⇒ **不做事**，继续；
//	② `/exit` · `/quit` · Ctrl-D ⇒ 退出 `rc=0`；
//	③ Ctrl-C ⇒ **不杀进程**：有字就清当前行，空行就提示「再按一次退出」，**连按两次**才退（窗口 2s）；
//	④ 每行非命令文本 ⇒ 当一轮 user 消息发出去，**逐块**打 stdout（流式 · ✗ 攒完再打）；
//	⑤ 会话：`--session <id>` 续接 · `--new` 新建 · 都不给 ⇒ 自动建一个新会话（stderr 报 id ＋ 续接法）。
//	   ★ 现读差异（照实登记 · ✗ 不掩）：裸条 `chat` 的 usage 串里**没有**这两枚旗标 ⇒ 派发层的
//	   `usageDeclaresFlag` 把它们判成「未知旗标 2」（`zerg chat --session …` 现读 rc=2）；
//	   本件的两档**已就位**（一旦裸条 usage 补上即生效）。要放行得在 `main.go` 的裸条 `chat`
//	   usage 串里加 `[--session <id>] [--new]` —— 那属**命令树 / 用法面**、不在本单的改动面内。
//
// ★ Ctrl-C 的**真机制**（写死 · 照 `cmdChatRoot` 注释 ④ 最后一条）：REPL 期把终端切到 `-isig`
// ⇒ Ctrl-C **不再产生 SIGINT**，而是作为 `\x03` 字节从 stdin 到达 ⇒ 由本件的读循环自己处理
// （这也是「raw 档下 Ctrl-C 三格必须由 REPL 自己读 `\x03` 实现」那条口径的落点）。
// 另在入口 `signal.Notify(intr, os.Interrupt)` **显式接管**非终端来源的 SIGINT（`kill -INT`）：
// ✗ 不用 `signal.Ignore` —— 那会把「要退出」这一面也拦掉，还会把别族的退出语义一起吞。
//
// ★ 残留风险（照实登记 · ✗ 不掩）：`main()` 的全局 SIGINT/SIGTERM/SIGHUP 处理器持有**局部**通道
// （`main.go` 的 `sigs`）⇒ 本件拿不到句柄做 `signal.Stop`（把它提为包级属 `main.go` 信号面，
// 不在本单的改动面内）⇒ `kill -TERM` / 关窗仍会走 `os.Exit(130)` 而**不经过**本件的 `defer`；
// 兜底是「退出前必还原终端」＋ 还原失败时给人一句 `stty sane` 的提示。
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	// chatREPLPrompt —— 提示符（朴素：录屏 / 脚本里要能一眼认出「轮到我打字」）。
	chatREPLPrompt = "chat> "
	// chatREPLCtrlCWindow —— 「连按两次 Ctrl-C 退出」的窗口（超出即重新计数）。
	chatREPLCtrlCWindow = 2 * time.Second
	// chatREPLKeyBuf —— 键盘字节队列容量（读协程 → 主循环的缓冲）。
	chatREPLKeyBuf = 64
	// chatREPLETX —— Ctrl-C 的字节（`-isig` 档下它**就是**一个普通输入字节）。
	chatREPLETX = 0x03
	// chatREPLEOT —— Ctrl-D 的字节（`-icanon` 档下同上）。
	chatREPLEOT = 0x04
)

// chatREPLAct —— 一次读行之后该做什么（四态 · 一处判、一处收口）。
type chatREPLAct int

const (
	chatREPLLine chatREPLAct = iota // 读到一行（可能为空行 ⇒ 由调用方判「不做事」）
	chatREPLIntr                    // Ctrl-C 只清了行 / 只提示 ⇒ 重新出提示符
	chatREPLEOF                     // Ctrl-D 或 stdin EOF ⇒ 退出 rc=0
	chatREPLBye                     // /exit · /quit · 连按两次 Ctrl-C ⇒ 退出 rc=0
)

// chatREPL —— 行式 REPL 的运行态（一个实例 = 一次交互会话）。
type chatREPL struct {
	inv *invocation
	in  *os.File  // stdin（键面）
	out io.Writer // stdout（**只放正文**）
	err io.Writer // stderr（提示 / 回显 / 进度 / 错误）
	sid string    // 当前会话 id

	tty   bool   // stdin 是不是**真终端**（`isTTYFile`：`/dev/null` 过不了它）
	saved string // `stty -g` 快照（非空 = 终端已切字符模式 ⇒ 退出必还原）

	ctrlCAt time.Time // 上一击 Ctrl-C 的时刻（**空行态**才计入「连按」）
}

// chatREPLRun —— REPL 的唯一实现入口（`family_chat.go` 的 `chatEnterREPL` 只是它的薄壳）。
func chatREPLRun(inv *invocation, in *os.File, stdout, stderr io.Writer) int {
	r := &chatREPL{inv: inv, in: in, out: stdout, err: stderr}
	r.tty = isTTYFile(in)
	if !r.tty {
		fmt.Fprintf(stderr, "%s: ⚠ stdin 不是终端 ⇒ 按**行式**读（✗ 无 Ctrl-C / Ctrl-D 键位面）\n", progName)
	}
	// ① 会话（`--session` 续接 / `--new` 新建 / 都不给 ⇒ 自动建）—— **在建终端模式之前**：
	//    建会话要打主控，失败就该干净退出（终端还没被动过）。
	if rc := r.pickSession(); rc != exitOK {
		return rc
	}
	// ② 终端切字符模式（Ctrl-C 才能**立刻**被读到，不必等回车）—— 失败即退化，不拦人。
	if r.tty {
		if err := r.enterCharMode(); err != nil {
			fmt.Fprintf(r.err, "%s: ⚠ 终端切字符模式失败（%v）⇒ 退化成逐行读（Ctrl-C 交给系统的信号面）\n", progName, err)
			r.tty = false
		}
	}
	// ③ 退出面：**先注册还原**（defer 后进先出 ⇒ 本行在 signal.Stop **之前**执行 ——
	//    与 `cmdChatRoot` 注释 ④「`term.Restore` 在恢复信号面之前」同序）。
	defer r.restoreTerm()
	// ④ 显式接管 SIGINT（✗ signal.Ignore）：终端来源的 Ctrl-C 走 `\x03` 字节，这一路只管非终端来源。
	intr := make(chan os.Signal, 1)
	signal.Notify(intr, os.Interrupt)
	defer signal.Stop(intr)

	// ⑤ 键盘：一个读协程逐字节喂队列（主循环同时看 `intr`，不必把 Read 做成可中断）。
	keys := make(chan byte, chatREPLKeyBuf)
	readErr := make(chan error, 1)
	go func() {
		for {
			var b [1]byte
			n, err := r.in.Read(b[:])
			if n == 1 {
				keys <- b[0]
			}
			if err != nil {
				readErr <- err
				return
			}
		}
	}()

	r.banner()
	for {
		if r.tty {
			fmt.Fprint(r.err, chatREPLPrompt)
		}
		line, act := r.readLine(keys, readErr, intr)
		switch act {
		case chatREPLIntr:
			continue
		case chatREPLEOF:
			fmt.Fprintf(r.err, "%s: 输入结束（Ctrl-D）⇒ 退出交互模式（会话 %s 保留）\n", progName, r.sid)
			return exitOK
		case chatREPLBye:
			return exitOK
		}
		text := strings.TrimSpace(line)
		if text == "" {
			continue // ① 空行 / 全空白 ⇒ 不做事，继续
		}
		switch text {
		case "/exit", "/quit":
			fmt.Fprintf(r.err, "%s: 退出交互模式（会话 %s 保留）\n", progName, r.sid)
			return exitOK
		case "/help":
			r.help()
			continue
		}
		r.sendTurn(text)
	}
}

// readLine —— 读一行（**逐字节** · ✗ 不接管光标定位 / 不画全屏）。
func (r *chatREPL) readLine(keys <-chan byte, readErr <-chan error, intr <-chan os.Signal) (string, chatREPLAct) {
	line := make([]byte, 0, 256)
	escSkip := 0 // 转义序列（方向键 / Home / Delete）的吞字节计数：✗ 当正文发出去
	for {
		select {
		case b, ok := <-keys:
			if !ok {
				return string(line), chatREPLEOF
			}
			if escSkip > 0 {
				escSkip--
				continue
			}
			switch b {
			case 0x1b: // ESC：方向键一类 ⇒ 吞掉后续两字节（本版不做历史 / 光标移动）
				escSkip = 2
			case '\n', '\r':
				r.newline()
				return string(line), chatREPLLine
			case chatREPLEOT: // Ctrl-D ⇒ **退出**（契约 ②：Ctrl-D 与 /exit /quit 同档 ⇒ rc=0）
				// ✗ 不做「非空行就 flush 成一轮」那种隐式发送：Ctrl-D 是**退出键**，不是发送键
				// （否则手滑就白烧一轮；要发就回车）。
				r.newline()
				return "", chatREPLEOF
			case chatREPLETX: // Ctrl-C（`-isig` 档下作为字节到达）
				if r.ctrlC(line) {
					return "", chatREPLBye
				}
				return "", chatREPLIntr
			case 0x7f, 0x08: // 退格
				if len(line) > 0 {
					_, size := utf8.DecodeLastRune(line)
					line = line[:len(line)-size]
					r.redraw(line)
				}
			default:
				if b < 0x20 { // 其余控制符不收（当正文会污染提示符）
					continue
				}
				line = append(line, b)
				if r.tty {
					fmt.Fprint(r.err, string([]byte{b}))
				}
			}
		case <-readErr:
			return string(line), chatREPLEOF
		case <-intr:
			if r.ctrlC(line) {
				return "", chatREPLBye
			}
			return "", chatREPLIntr
		}
	}
}

// ctrlC —— Ctrl-C 的唯一处理处（`\x03` 字节与 SIGINT 两条来源**同走这里**）。
//
// 返回 quit：true = 连按两次（空行态 ＋ 窗口内）⇒ 退出 `rc=0`；false = 只清行 / 只提示。
func (r *chatREPL) ctrlC(line []byte) bool {
	if len(line) > 0 {
		// 清行那一击**不计入**「连按」：否则「清一下行、顺手再按一下」就会退掉。
		r.ctrlCAt = time.Time{}
		fmt.Fprintf(r.err, "\r\x1b[K（Ctrl-C：已清当前行 · 再按一次（空行）退出，或输入 /exit）\n")
		return false
	}
	if !r.ctrlCAt.IsZero() && time.Since(r.ctrlCAt) < chatREPLCtrlCWindow {
		r.ctrlCAt = time.Time{}
		fmt.Fprintf(r.err, "\r\x1b[K%s: 连按两次 Ctrl-C ⇒ 退出交互模式（会话 %s 保留）\n", progName, r.sid)
		return true
	}
	r.ctrlCAt = time.Now()
	fmt.Fprintf(r.err, "\r\x1b[K（Ctrl-C：再按一次退出 · 或输入 /exit）\n")
	return false
}

// sendTurn —— 把一行文本当**一轮 user 消息**发出去（与 `chat send` 同一条路），流式逐块打 stdout。
func (r *chatREPL) sendTurn(text string) {
	body, err := json.Marshal(map[string]any{"content": text})
	if err != nil {
		fmt.Fprintf(r.err, "%s: 请求体序列化失败（%v）—— 这一轮没发出去\n", progName, err)
		return
	}
	markdown := r.inv.hasFlag("--markdown") && !r.inv.jsonGiven
	turn, rc := chatPostTurn(r.inv, newClient(), r.sid, string(body), true, markdown, r.out, r.err)
	// 正文尾巴补一个换行（流式档的 `delta` 不带行尾 ⇒ 不补会让下一轮提示粘在正文后面）。
	fmt.Fprintln(r.out)
	if turn == nil {
		fmt.Fprintf(r.err, "%s: 这一轮没成（rc=%d）—— 会话 %s 仍在；可重发，或 /exit 退出\n", progName, rc, r.sid)
		return
	}
	chatLastWrite(r.sid, r.err)
	if rc != exitOK {
		fmt.Fprintf(r.err, "%s: 这一轮按下档收尾（rc=%d · interrupted=%t）—— 会话 %s 仍在\n",
			progName, rc, turn.Interrupted, r.sid)
	}
	fmt.Fprintf(r.err, "#%s · msg %s · in %s / out %s / think %s\n", r.sid,
		orDash(turn.MessageID), orDash(turn.InputTokens), orDash(turn.OutputTokens), orDash(turn.ReasonTokens))
}

// banner —— 进 REPL 的三行说明（stderr：人面）。
func (r *chatREPL) banner() {
	fmt.Fprintf(r.err, "%s: 交互模式（行式 REPL · 会话 %s）\n", progName, r.sid)
	fmt.Fprintf(r.err, "  直接输入 = 发一轮 · 空行 = 不发 · /exit 或 /quit 或 Ctrl-D = 退出（rc=0）\n")
	fmt.Fprintf(r.err, "  Ctrl-C = 清当前行（连按两次退出）· /help = 这三行\n")
}

// help —— REPL 内的 `/help`（✗ 不落到全局 `zerg help`：那是命令树的面，这里是键位面）。
func (r *chatREPL) help() {
	fmt.Fprintf(r.err, "  直接输入一行文本 = 当一轮 user 消息发出去（响应逐块流式打出来）\n")
	fmt.Fprintf(r.err, "  /exit · /quit = 退出（rc=0）· Ctrl-D = 退出 · Ctrl-C = 清当前行（连按两次退出）\n")
	fmt.Fprintf(r.err, "  /help = 本三行 · 当前会话 %s\n", r.sid)
}

// pickSession —— 会话选择（`--session` 续接 · `--new` 新建 · 都不给 ⇒ 自动建一个新会话）。
//
// ★ 报 id ＋ 续接法照 `chat new` 已有的报法（stderr 一行 · stdout 留给正文）。
func (r *chatREPL) pickSession() int {
	if r.inv.hasFlag("--session") {
		if r.inv.hasFlag("--new") {
			return chatReportErr(r.inv, r.err, "usage", "new_and_session_conflict",
				"`--new`（新开一个会话）与 `--session` 不许同给（与 `chat send` 同口径）", exitCodeOf("usage"))
		}
		v := strings.TrimSpace(r.inv.flagVal("--session"))
		if v == "" {
			return chatReportErr(r.inv, r.err, "usage", "missing_flag_value",
				"`--session` 给了旗标却没给 id（要完整 id，如 `chat_20261017_010203_ab12cd`）", exitCodeOf("usage"))
		}
		id, ok := chatNormalizeSessionRef(v)
		if !ok {
			return chatReportErr(r.inv, r.err, "usage", "bad_session_id",
				fmt.Sprintf("会话 id %q 形状不合法（`O-12`：只认完整 id；✗ 不做前缀 / 标题解析）", v), exitCodeOf("usage"))
		}
		r.sid = id
		fmt.Fprintf(r.err, "%s: 续接会话 %s\n", progName, id)
		return exitOK
	}
	id, rc := chatCreateSession(r.inv, r.err)
	if rc != exitOK {
		return rc
	}
	r.sid = id
	chatLastWrite(id, r.err)
	fmt.Fprintf(r.err, "%s: 交互会话 %s 就绪 —— 续接单发一轮：`%s chat send --session %s \"…\"`\n",
		progName, id, progName, id)
	return exitOK
}

// chatREPLStty 跑一次 `stty …`（stdin = 本端 in）。
//
// ★ 与 `family_approve.go` 的 `sttyEcho` **同一套办法**：借系统的 `stty`，✗ 不为它引 `x/term` 依赖。
// 返回 stdout 去空白（`stty -g` 的快照要整串拿回来）。
func chatREPLStty(in *os.File, args ...string) (string, error) {
	cmd := exec.Command("stty", args...)
	cmd.Stdin = in
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

// enterCharMode —— 切「逐字节 ＋ 无回显 ＋ 关 ISIG」。
func (r *chatREPL) enterCharMode() error {
	saved, err := chatREPLStty(r.in, "-g")
	if err != nil {
		return fmt.Errorf("`stty -g` 快照失败：%v", err)
	}
	if saved == "" {
		return fmt.Errorf("`stty -g` 没给出快照（无法保证退出能还原）")
	}
	r.saved = saved
	// `-isig`：Ctrl-C **不进信号面**（作为 `\x03` 字节到达）—— 本档「Ctrl-C 不杀进程」的真机制；
	// `-icanon min 1 time 0`：逐字节交付（Ctrl-C 立刻可读，不必等回车）；
	// `-echo`：回显由本件自己做（整行重画 ⇒ 中文退格才对得上列宽）。
	if _, err := chatREPLStty(r.in, "-isig", "-icanon", "-echo", "min", "1", "time", "0"); err != nil {
		return fmt.Errorf("切字符模式失败：%v", err)
	}
	return nil
}

// restoreTerm —— 还原终端（`stty <快照>`）。幂等：快照被清空后不再动。
func (r *chatREPL) restoreTerm() {
	if r.saved == "" {
		return
	}
	saved := r.saved
	r.saved = ""
	if _, err := chatREPLStty(r.in, saved); err != nil {
		fmt.Fprintf(r.err, "%s: ⚠ 终端状态没还原回（%v）—— shell 若回显异常，手动跑一次 `stty sane`\n", progName, err)
	}
}

// newline —— 行尾（只在真终端上补 `\r\n`：管道档不加多余字节）。
func (r *chatREPL) newline() {
	if r.tty {
		fmt.Fprint(r.err, "\r\n")
	}
}

// redraw —— 整行重画（退格 / 清行后）。用 `\r` ＋ `\x1b[K` 而不是逐字节 `\b \b`：
// 中文占两列，逐字节退格会错位（✗ 全屏定位，只清本行）。
func (r *chatREPL) redraw(line []byte) {
	if !r.tty {
		return
	}
	fmt.Fprintf(r.err, "\r\x1b[K%s%s", chatREPLPrompt, string(line))
}
