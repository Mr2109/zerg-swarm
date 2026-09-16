package backend

// T12（进度证据）+ T14（阶段二：正文停滞看门狗）用例。
//
// 判据依据（2026-09-15 实测 + Mr2109 拍板"尽量不用时间限制"）：
//   · 有推进 ⇒ 活（合法的长 prefill 就是这样，实测 198,698 token / ≈30 min）；
//   · 无推进 + CPU 不动 ⇒ 立刻判死；
//   · 无推进 + CPU 在涨 ⇒ 累计到 N 个窗口判死；
//   · 进度不可读 ⇒ 只用 CPU 判（仍收敛）。

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func phase2Env(t *testing.T) {
	t.Setenv("ZERG_WATCHDOG", "1")
	t.Setenv("ZERG_WATCHDOG_WINDOW_S", "0.10")
	t.Setenv("ZERG_WATCHDOG_SAMPLE_S", "0.02")
	t.Setenv("ZERG_WATCHDOG_MAX_WINDOWS", "2")
	t.Setenv("ZERG_WATCHDOG_DEAD_FLAT", "2")
}

// sseEngine 假引擎：立刻回头（200 + SSE），随后按 contentFn 决定正文写什么。
// contentFn 返回 "" 表示"只发保活注释"（= 预填充期不出字的样子）。
func sseEngine(contentFn func(tick int) string) *httptest.Server {
	return sseEngineSlots(contentFn, `[{"id":0,"is_processing":true,"n_prompt_tokens":198698,"n_prompt_tokens_processed":196650}]`)
}

// sseEngineSlots：slotsBody 为空串 ⇒ `/slots` 不可读（用于"不可读不得判死"的回归）。
func sseEngineSlots(contentFn func(tick int) string, slotsBody string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/slots" {
			if slotsBody == "" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			_, _ = io.WriteString(w, slotsBody)
			return
		}
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fl, _ := w.(http.Flusher)
		if fl != nil {
			fl.Flush()
		}
		for tick := 0; tick < 60; tick++ {
			if s := contentFn(tick); s != "" {
				_, _ = io.WriteString(w, "data: "+s+"\n\n")
			} else {
				_, _ = io.WriteString(w, ":\n\n") // SSE 保活注释（不算内容）
			}
			if fl != nil {
				fl.Flush()
			}
			time.Sleep(10 * time.Millisecond)
		}
	}))
}

func (m *Manager) waitVerdict(t *testing.T, model string, want WatchdogVerdict, d time.Duration) *WatchdogObservation {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		for _, o := range m.EggObservations() {
			if o.EggID == model && o.Watchdog != nil && o.Watchdog.Verdict == want {
				return o.Watchdog
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	return nil
}

// ① 正文阶段：只发保活、无推进，且 CPU 不动 ⇒ 判死（提前，不必等窗口用满）
func TestPhase2_NoProgressCPUFlat_JudgedStuck(t *testing.T) {
	phase2Env(t)
	srv := sseEngine(func(int) string { return "" }) // 永远只有保活注释
	defer srv.Close()

	m, _, done := wireTestManager(t, portOf17(t, srv.URL), func(*subproc) func() (uint64, error) {
		return func() (uint64, error) { return 12345, nil } // 工时不动
	})
	defer done()

	resp, err := m.InferForward(context.Background(), "m", "/v1/chat/completions", []byte(`{"model":"m","stream":true}`))
	if err != nil {
		t.Fatalf("阶段一（等头）不该失败：%v", err)
	}
	defer resp.Body.Close()

	obs := m.waitVerdict(t, "m", WatchdogStuckCPUStalled, 3*time.Second)
	if obs == nil {
		t.Fatal("正文阶段无推进 + CPU 不动 ⇒ 必须判死（这正是 A8 那类'看着在跑其实不动'）")
	}
	if obs.Reason == "" {
		t.Fatal("判词必须带理由（供主控 failover + 人复盘）")
	}
}

// ② 正文阶段：有推进（内容分片在出）⇒ **不得**判死（合法长生成要活）
//
// 夹具口径（2026-09-16 第十四轮重定，含一条把我第一版修法打回来的事实）：
//
//	· 病根是**夹具的时间比**，不是产品判据 —— 夹具 SSE 分片节奏 10ms/片，而判死门槛只有分片间隔的 2 倍，
//	  于是一次正常的投递迟延就能冒充「彻底停摆」（实测抓到的那次：判死时 content=288B，
//	  正文才出约 7 片、流还在跑）；
//	· **关键事实（第一版修法无效的原因）**：`wireTestManager`（共享夹具构造函数）**自己会覆写**
//	  `ZERG_WATCHDOG_WINDOW_S=0.05 / SAMPLE_S=0.01 / MAX_WINDOWS=2` —— 在它**之前**设的环境变量
//	  全被无声覆盖（我第一版把 Setenv 写在它前面 ⇒ 门槛算成了 250ms，实际生效的仍是
//	  `DEAD_FLAT 2 × SAMPLE 0.01 = **20ms**`）。**要改口径必须写在 wireTestManager 之后**。
//	· 现在的口径（写在之后，实测生效）：窗口 0.5s × 上限 5 ⇒ 阶段二"停滞窗口"口径 2.5s（远超本用例 0.6s
//	  的流长）· "提前判死"门槛 = DEAD_FLAT 10 × SAMPLE 0.02 = **200ms = 分片间隔的 20 倍**；
//	  阶段一（等头）窗口 0.5s，给掉调度迟延的余量。
//	· **断言一字未改**（内容在出 ⇒ 任何判死都不许出现）；变异验证证明它仍有牙齿（打坏正文计数 ⇒ 必红）。
func TestPhase2_Progressing_NotJudged(t *testing.T) {
	phase2Env(t)
	srv := sseEngine(func(tick int) string {
		return fmt.Sprintf(`{"choices":[{"delta":{"content":"字%d"}}]}`, tick)
	})
	defer srv.Close()

	m, _, done := wireTestManager(t, portOf17(t, srv.URL), func(*subproc) func() (uint64, error) {
		return func() (uint64, error) { return 999, nil } // 工时不动，但内容在出
	})
	defer done()

	// ⚠ **必须写在 wireTestManager 之后**：它内部会覆写 WINDOW/SAMPLE/MAX_WINDOWS（见用例头注释）。
	// 口径：停滞窗口 0.5s × 上限 5 = 2.5s（> 本用例 0.6s 流长）· 提前判死门槛 10 × 0.02 = 200ms。
	t.Setenv("ZERG_WATCHDOG_WINDOW_S", "0.5")
	t.Setenv("ZERG_WATCHDOG_MAX_WINDOWS", "5")
	t.Setenv("ZERG_WATCHDOG_SAMPLE_S", "0.02")
	t.Setenv("ZERG_WATCHDOG_DEAD_FLAT", "10")

	resp, err := m.InferForward(context.Background(), "m", "/v1/chat/completions", []byte(`{"model":"m","stream":true}`))
	if err != nil {
		t.Fatalf("阶段一不该失败：%v", err)
	}
	defer resp.Body.Close()
	go func() { _, _ = io.Copy(io.Discard, resp.Body) }() // 客户端正常读

	if obs := m.waitVerdict(t, "m", WatchdogStuckCPUStalled, 300*time.Millisecond); obs != nil {
		t.Fatalf("内容在出 ⇒ 绝不能判死：%+v", *obs)
	}
	if obs := m.waitVerdict(t, "m", WatchdogStuckNoProgress, 300*time.Millisecond); obs != nil {
		t.Fatalf("内容在出 ⇒ 绝不能判死：%+v", *obs)
	}
}

// ③ 正文阶段：无推进但 CPU 在涨（"占着 CPU 在磨"）⇒ 窗口用满后判死（兜住"瞎花力气"）
func TestPhase2_BusyNoProgress_AfterMaxWindows(t *testing.T) {
	phase2Env(t)
	srv := sseEngine(func(int) string { return "" })
	defer srv.Close()

	var n uint64 = 1000
	m, _, done := wireTestManager(t, portOf17(t, srv.URL), func(*subproc) func() (uint64, error) {
		return func() (uint64, error) { n += 100000; return n, nil } // 工时一直涨
	})
	defer done()

	resp, err := m.InferForward(context.Background(), "m", "/v1/chat/completions", []byte(`{"model":"m","stream":true}`))
	if err != nil {
		t.Fatalf("阶段一不该失败：%v", err)
	}
	defer resp.Body.Close()

	if obs := m.waitVerdict(t, "m", WatchdogStuckNoProgress, 3*time.Second); obs == nil {
		t.Fatal("CPU 在涨但始终无推进 ⇒ 窗口用满必须判死（否则'忙等'会永远循环）")
	}
}

// ④ T12：/slots 解析 —— 用真机实测到的字段与取值
func TestSlotsProgress_ParsesMeasuredShape(t *testing.T) {
	measured := `[{"id":0,"n_ctx":262144,"speculative":false,"is_processing":false},
	 {"id":2,"n_ctx":262144,"is_processing":true,"id_task":6,
	  "n_prompt_tokens":198698,"n_prompt_tokens_processed":196650,"n_prompt_tokens_cache":0}]`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/slots" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = io.WriteString(w, measured)
	}))
	defer srv.Close()

	m := &Manager{procs: map[string]*subproc{}}
	sp := &subproc{model: "m", port: portOf17(t, srv.URL)}
	got, err := m.eggProgressFunc(sp)()
	if err != nil {
		t.Fatalf("实测形态必须解析成功：%v", err)
	}
	if got != 196650 {
		t.Fatalf("进度读数错：want 196650 got %d（字段名按实测，不许猜）", got)
	}

	// 读到垃圾必须**明确报错**（调用侧据此退化为只用 CPU，不误判活）
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "not-json")
	}))
	defer bad.Close()
	sp2 := &subproc{model: "m", port: portOf17(t, bad.URL)}
	if _, err := m.eggProgressFunc(sp2)(); err == nil {
		t.Fatal("解析失败必须报错，不能静默返回 0（假绿）")
	}
}

var _ = json.Marshal

// ⑤ 回归（沙箱批 D 抓到的假阳性）：**进度读数不可读时，绝不能按窗口判死**。
// 读不到 ≠ 没推进；否则一次 /slots 读取失败就会把合法的长预填充（实测 ≈30 min 那种）杀掉。
func TestPhase2_ProgressUnreadable_MustNotJudge(t *testing.T) {
	phase2Env(t)
	srv := sseEngineSlots(func(int) string { return "" }, "") // /slots 不可读，正文只有保活
	defer srv.Close()

	var n uint64 = 500
	m, _, done := wireTestManager(t, portOf17(t, srv.URL), func(*subproc) func() (uint64, error) {
		return func() (uint64, error) { n += 100000; return n, nil } // CPU 一直涨（"看着在忙"）
	})
	defer done()

	resp, err := m.InferForward(context.Background(), "m", "/v1/chat/completions", []byte(`{"model":"m","stream":true}`))
	if err != nil {
		t.Fatalf("阶段一不该失败：%v", err)
	}
	defer resp.Body.Close()

	// 跑满远超"窗口数"的时间（0.10s × 2 窗的 3 倍以上）
	if obs := m.waitVerdict(t, "m", WatchdogStuckNoProgress, 700*time.Millisecond); obs != nil {
		t.Fatalf("进度不可读时按窗口判死 = 误杀合法长预填充：%+v", *obs)
	}
}

// ⑥ 回归（2026-09-16 第十四轮 -race 实测抓到的**偶发假判死**）：**正文读完（EOF）之后不得再判死**。
//
// 病根：守卫只认「有没有推进」，而**流结束之后「无推进」是必然的**（引擎收工 ⇒ 内容不再来、
// 本单元 CPU 也不再涨）⇒ 不在 EOF 退场，就会在 EOF 之后的第 DeadFlat 次采样把
// **已经正常完成**的请求判成 stuck_cpu_stalled、置 crashed、掐掉请求。
// 夹具按真引擎的行为给工时：**生成期在涨、流读完之后闲下来不动**（真引擎就是这两段）。
// 本用例把这一刻**钉成确定性**：客户端把正文读干净（走到 EOF）之后**再等 1 s**
// （生效门槛 = `wireTestManager` 覆写的 DEAD_FLAT 2 × SAMPLE 0.01 = 20ms ⇒ 余量 50 倍），
// 期间不得出现任何判死。
func TestPhase2_StreamFinished_NotJudgedAfterEOF(t *testing.T) {
	phase2Env(t)
	srv := sseEngine(func(tick int) string {
		return fmt.Sprintf(`{"choices":[{"delta":{"content":"字%d"}}]}`, tick)
	})
	defer srv.Close()

	var cpuUsec uint64 = 999
	var eofReached atomic.Bool
	m, _, done := wireTestManager(t, portOf17(t, srv.URL), func(*subproc) func() (uint64, error) {
		return func() (uint64, error) {
			if eofReached.Load() {
				return cpuUsec, nil // 流读完之后：引擎本就该闲下来（这才是"必然无推进"的那一刻）
			}
			cpuUsec += 5000 // 生成期在花力气（真机实测量级：5 s 涨 5,008,555 µs）
			return cpuUsec, nil
		}
	})
	defer done()

	// ⚠ **必须写在 wireTestManager 之后**（它内部会覆写 WINDOW/SAMPLE/MAX_WINDOWS）。
	// 口径：阶段一（等头）窗口 0.5s 给掉调度迟延；本例判定对象是"EOF 之后不得再判死"。
	t.Setenv("ZERG_WATCHDOG_WINDOW_S", "0.5")
	t.Setenv("ZERG_WATCHDOG_MAX_WINDOWS", "5")

	resp, err := m.InferForward(context.Background(), "m", "/v1/chat/completions", []byte(`{"model":"m","stream":true}`))
	if err != nil {
		t.Fatalf("阶段一不该失败：%v", err)
	}
	defer resp.Body.Close()

	// 读干净 = 走到 EOF = 推理确实结束了（这一步是本用例的前提，必须先成立再谈"之后"）
	n, err := io.Copy(io.Discard, resp.Body)
	if err != nil {
		t.Fatalf("读正文出错：%v", err)
	}
	if n == 0 {
		t.Fatal("正文应有内容（本用例的前提是内容在出）")
	}
	eofReached.Store(true) // 此后引擎闲下来：内容与工时双不动 —— 正是最容易被误判成"停摆"的形态

	if obs := m.waitVerdict(t, "m", WatchdogStuckCPUStalled, time.Second); obs != nil {
		t.Fatalf("正文已读完（EOF）⇒ 绝不能判死（此后无推进是必然的）：%+v", *obs)
	}
	if obs := m.waitVerdict(t, "m", WatchdogStuckNoProgress, time.Second); obs != nil {
		t.Fatalf("正文已读完（EOF）⇒ 绝不能判死：%+v", *obs)
	}
}
