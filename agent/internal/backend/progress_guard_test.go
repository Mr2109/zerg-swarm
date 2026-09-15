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
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
