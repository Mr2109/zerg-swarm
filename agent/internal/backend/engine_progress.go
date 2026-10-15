package backend

// engine_progress.go —— T12 进度证据的**回落源**：解析引擎自己输出的进度行。
//
// 背景（2026-10-16 · 2.5.15「D 进度源」· 设计件《DS4进度源-设计-20261016》§4）：
// 卵侧看门狗要的「真实 chunk 计数」原先只有**一个**源 —— HTTP `GET /slots` 的
// `n_prompt_tokens_processed`（见 `watchdog_wire.go` 的 `eggProgressFunc`）；而 DS4
//（antirez DwarfStar）**没有 `/slots` 路由**（只读端点只有 /v1/models 一族）⇒ 该源恒报
//「进度不可读」⇒「progress 冻结 ≥ 预算 ⇒ 判卡」这条判据对它**从不生效**。
//
// 但 DS4 本来就在输出进度，走的是 stderr（→ journal，孵化路径）。实机逐字（journalctl -o cat）：
//
//	bwrap[1404525]: 1015 17:53:44 ds4-server: listening on http://127.0.0.1:9400
//	bwrap[1404525]: 1015 17:53:45 ds4-server: chat ctx=0..5:5 prompt start
//	bwrap[1404525]: 1015 17:53:49 ds4-server: chat ctx=5..6:1 gen=1 THINKING decoding chunk=3.84 t/s avg=3.84 t/s 0.260s
//	bwrap[1404525]: 1015 17:53:51 ds4-server: chat ctx=5..13:8 gen=8 THINKING decoding chunk=4.44 t/s avg=4.44 t/s 1.803s
//	bwrap[1404525]: 1015 17:53:51 ds4-server: chat ctx=0..14267:14267 TOOLS chunk 12288/14267 (86.1%) chunk=91.66 t/s
//
// 本件只做两件事：① 把这几行**纯函数**地解析成同口径的「已处理 token 计数」；
// ② 一个**只读**取数器（journal 尾巴 → 解析）。
//
// ★ 零引擎改动、零协议改动、零新配置：✗ 不改 `ds4_server.c`、✗ 不加新 env / 旗标 / 卵档案字段；
// 注入点（`manager.go` 的 `progFn`）与判据（`watchdog.go` / `progress_guard.go`）**一字不动**。

import (
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// engineProgressPromptStart 请求纪元边界标记：DS4 每个请求开头一行 `… prompt start`。
	engineProgressPromptStart = "prompt start"
	// engineProgressTailLines 取数窗口（journal 尾巴行数）：一个 prefill 量子（实测 4096 token）1 行
	// ＋ 解码每 50 token 1 行 ⇒ 40 行足以盖住「刚发生的推进」。
	engineProgressTailLines = 40
	// engineProgressPollEvery 节流：两次**真读**之间至少隔这么久（≤ 看门狗 Sample 默认 5 s）。
	engineProgressPollEvery = 3 * time.Second
)

// enginePrefillChunkRe 预填充进度行 `… chunk <已处理>/<总数> (<pct>%) …`。
//
// ★ 只认 `<数字>/<数字>` 这一形态：解码行里的 `chunk=4.44 t/s`（`=` 不是 `/`）**不会**被这里吃掉。
var enginePrefillChunkRe = regexp.MustCompile(`chunk[ \t]+([0-9]+)/([0-9]+)`)

// engineGenRe 解码进度行 `… gen=<N> … decoding …`（DS4 每 50 个生成 token 一行 + 收尾必出一行）。
var engineGenRe = regexp.MustCompile(`\bgen=([0-9]+)`)

// parseEngineProgress 从**引擎输出行的尾巴**里算出「本请求已处理 token 计数」。
//
// 纯函数：同一窗口重复读，结果逐字相同（5 s 级轮询会反复覆盖同一段 ⇒ **必须幂等**）。
//
//	· `… prompt start`             ⇒ 新请求纪元：prefillMax=0, genMax=0（**旧纪元的账不留**）
//	· `… chunk <cur>/<total> (…)`  ⇒ prefillMax = max(prefillMax, cur)   // 预填充（DS4 stderr 行）
//	· `… gen=<N> … decoding …`     ⇒ genMax    = max(genMax, N)          // 解码（DS4 stderr 行）
//
// 返回 counter = prefillMax + genMax —— 同一请求内**单调不减**：预填充阶段只 prefillMax 涨；
// 解码阶段 prefillMax 冻结、genMax 涨 ⇒ **跨相也不回退**。
//
// 一行都没匹配 ⇒ `(0, false)`：**读不到**（✗ 不许回落成「0 ＝ 没推进」—— 那是把「读不到」
// 误读成「冻结」，会误杀；调用侧只按「读不到」处理，见 `progress_guard.go` 的 fail-open 口径）。
//
// ★ 已知边界（设计 §5.6）：计数器只跟**最新纪元**（窗口里最后一次 `prompt start`）；
// 同一枚卵上真并发两个请求时，非最新那个的推进**看不见**。
func parseEngineProgress(tail string) (uint64, bool) {
	var prefillMax, genMax uint64
	matched := false
	for _, line := range strings.Split(tail, "\n") {
		if strings.Contains(line, engineProgressPromptStart) {
			// ★ 纪元重置：新请求开始 ⇒ 旧请求的 chunk / gen 账**一律作废**
			// （防把上个请求的推进记到本请求头上，也防跨请求读数被读成「一直在推进」）。
			prefillMax, genMax = 0, 0
			matched = true
			continue
		}
		if m := enginePrefillChunkRe.FindStringSubmatch(line); m != nil {
			if cur, err := strconv.ParseUint(m[1], 10, 64); err == nil && cur > prefillMax {
				prefillMax = cur
			}
			matched = true
			continue
		}
		if m := engineGenRe.FindStringSubmatch(line); m != nil {
			if gen, err := strconv.ParseUint(m[1], 10, 64); err == nil && gen > genMax {
				genMax = gen
			}
			matched = true
		}
	}
	if !matched {
		return 0, false
	}
	return prefillMax + genMax, true
}

// engineJournalRunner 取数执行器（✗ 不另造 runner、✗ 不自己拼 env）：生产用 `unit_probe.go` 的
// `unitCmdRunner` —— 它已补 `hatch.UserScopeEnv()`（子端是**系统服务**，环境里没有
// `XDG_RUNTIME_DIR` ⇒ 一切 `journalctl --user` 都会失败；读不到 ⇒ 回落「读不到」，
// 与今天**等价** ⇒ 零回归、零误杀）。抽成变量只为单测注入假输出（nil ＝ 生产）。
var engineJournalRunner unitCmdRunnerFunc

// engineProgressState 引擎输出行进度源的小状态（挂在 `subproc` 上：孵化路径 `sp.proc` 恒 nil，
// 故必须挂在 sp 上）。**一个字段装两件事**：
//
//	· notSlots —— 「这一枚引擎没有 `/slots`」缓存（首次非 200 置真）⇒ 之后不再每次采样白打一次 404；
//	· 3 s 节流 ＋ 上一读数缓存 —— 同一采样窗内不重复 fork journalctl。
//
// 零值即可用（无需在构造点初始化）；读写一律在自带互斥下（同一卵上的并发请求共享它）。
type engineProgressState struct {
	mu         sync.Mutex
	lastPollAt time.Time
	last       uint64
	ok         bool
	notSlots   bool
}

// slotsAbsent 该引擎是否已被判定「没有 `/slots`」（缓存读）。
func (s *engineProgressState) slotsAbsent() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.notSlots
}

// noteSlotsAbsent 记下「这一枚引擎没有 `/slots`」（首次非 200 时置真）。
func (s *engineProgressState) noteSlotsAbsent() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.notSlots = true
}

// read 带 3 s 节流地取一次读数：节流窗内直接回**上次真读到**的读数
// （读不到 ⇒ 不缓存，下次仍真读 —— 读不到不该被冻成 3 s 的假值）。
func (s *engineProgressState) read(now time.Time, fetch func() (uint64, bool)) (uint64, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ok && !s.lastPollAt.IsZero() && now.Sub(s.lastPollAt) < engineProgressPollEvery {
		return s.last, true
	}
	v, ok := fetch()
	s.lastPollAt, s.last, s.ok = now, v, ok
	return v, ok
}

// unitEngineProgress 读该卵单元的 journal 尾巴 → parseEngineProgress。
//
// 行参逐字照 `unit_probe.go` 的 `unitJournalTail`（只是窗口更长：40 行），
// ★ runner 走同一个 `unitCmdRunner`（它处理 `XDG_RUNTIME_DIR`，见 `unit_probe.go` 的 `unitCmdRunner`）。
// 命令失败 / 单元名为空 / 一行都没匹配 ⇒ `(0, false)` ⇒ 调用侧照旧按「读不到」处理（fail-open）。
func (m *Manager) unitEngineProgress(sp *subproc) (uint64, bool) {
	if sp == nil || strings.TrimSpace(sp.Unit) == "" {
		return 0, false
	}
	run := engineJournalRunner
	if run == nil {
		run = unitCmdRunner
	}
	out, err := run(5*time.Second, "journalctl", "--user", "-u", sp.Unit,
		"-n", strconv.Itoa(engineProgressTailLines), "--no-pager", "-o", "cat")
	if err != nil && strings.TrimSpace(out) == "" {
		return 0, false
	}
	return parseEngineProgress(out)
}

// engineProgressFromEngine 取一次「引擎输出行进度」（带 3 s 节流 ＋ 上一读数缓存）。
// 只在 `/slots` 读不到时由 `eggProgressFunc` 回落调用（见 `watchdog_wire.go`）。
func (m *Manager) engineProgressFromEngine(sp *subproc) (uint64, bool) {
	if sp == nil {
		return 0, false
	}
	return sp.unitProgress.read(time.Now(), func() (uint64, bool) {
		return m.unitEngineProgress(sp)
	})
}
