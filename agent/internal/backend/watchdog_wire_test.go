package backend

// watchdog_wire_test.go —— 批 B（T4/T5）接线判据：判死端到端 + 正常路径零采样。
//
// 场景：假引擎**永不回响应头**（模拟长 prefill / 卡死），看门狗按证据裁决：
//   · 本单元 CPU 工时不动 ⇒ stuck_cpu_stalled（提前）
//   · 工时一直涨但窗口用满 ⇒ stuck_no_progress
//   · 引擎正常立刻回头 ⇒ ok（且**一次读数都不发生**）
// 判死形态：返回 *BackendBusyError（服务端据此回 503 + Retry-After + watchdog 详情），
// 且卵被置 crashed（不再派新请求）、**不收卵**。

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// silentEngine 假引擎：/health 立刻 200（healthCheck 用），POST 永不回头。
//
// ⚠ 收尾顺序（本用例踩过）：必须先放行 handler 再 Close 服务 —— httptest.Server.Close()
// 会**等所有在飞的 handler 返回**，若 handler 还堵在 <-gate 上就先 Close，测试会永久挂住。
// 用 t.Cleanup 在闭包内固定顺序，免去各用例自己排 defer。
func silentEngine(t *testing.T) *httptest.Server {
	t.Helper()
	gate := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusOK)
			return
		}
		<-gate // 永不返回响应头（模拟长 prefill / 卡死）
	}))
	t.Cleanup(func() {
		close(gate) // 先放行
		srv.Close() // 再关（此时 handler 已能返回）
	})
	return srv
}

func wireTestManager(t *testing.T, port int, cpu func(*subproc) func() (uint64, error)) (*Manager, *subproc, func()) {
	mk := newEvictTestManager(1, map[string]*subproc{})
	sp := &subproc{model: "m", port: port, state: StateReady, lastUsed: time.Now(), entry: inflightEntry()}
	mk.procs = map[string]*subproc{"m": sp}
	mk.watchdogCPUFor = cpu
	t.Setenv("ZERG_WATCHDOG", "1")
	t.Setenv("ZERG_WATCHDOG_WINDOW_S", "0.05")
	t.Setenv("ZERG_WATCHDOG_SAMPLE_S", "0.01")
	t.Setenv("ZERG_WATCHDOG_MAX_WINDOWS", "2")
	return mk, sp, func() {}
}

// 判据 ①：工时不动 ⇒ 提前判死，错误是 Busy（可重试语义），卵被置 crashed。
func TestWatchdogWire_CPUStalledMarksCrashed(t *testing.T) {
	srv := silentEngine(t)
	m, _, done := wireTestManager(t, portOf17(t, srv.URL), func(*subproc) func() (uint64, error) {
		return func() (uint64, error) { return 5000, nil } // 一点不涨
	})
	defer done()

	_, err := m.InferForward(context.Background(), "m", "/v1/chat/completions", []byte(`{"model":"m"}`))
	if err == nil {
		t.Fatal("工时不动 ⇒ 必须判死并返回错误")
	}
	var bbe *BackendBusyError
	if !errors.As(err, &bbe) {
		t.Fatalf("判死错误应是 *BackendBusyError（服务端据此回 503+Retry-After），实得 %T：%v", err, err)
	}
	if bbe.Verdict != WatchdogStuckCPUStalled {
		t.Fatalf("判词应为 stuck_cpu_stalled，实得 %s", bbe.Verdict)
	}
	if !IsBackendBusy(err) {
		t.Fatal("判死必须被 IsBackendBusy 认（否则服务端会当 502 故障处理）")
	}
	if got := m.StateOf("m"); got != StateCrashed {
		t.Fatalf("判死后卵应置 crashed（不再派新请求；**不收卵**），实得 %s", got)
	}
	if _, still := m.procs["m"]; !still {
		t.Fatal("判死阶段不得收卵（是否收卵交给既有五档裁决）")
	}
	if bbe.Obs.Reason == "" || bbe.Obs.WindowIdx < 1 {
		t.Fatalf("观测面必须带理由与窗口号（供 failover/复盘）：%+v", bbe.Obs)
	}
}

// 判据 ②：工时一直涨但窗口用满 ⇒ stuck_no_progress。
func TestWatchdogWire_NoProgressAfterMaxWindows(t *testing.T) {
	srv := silentEngine(t)
	var n uint64
	m, _, done := wireTestManager(t, portOf17(t, srv.URL), func(*subproc) func() (uint64, error) {
		return func() (uint64, error) { n += 1000; return n, nil } // 一直涨
	})
	defer done()

	_, err := m.InferForward(context.Background(), "m", "/v1/chat/completions", []byte(`{"model":"m"}`))
	var bbe *BackendBusyError
	if !errors.As(err, &bbe) {
		t.Fatalf("窗口用满 ⇒ 应判死（*BackendBusyError），实得 %T：%v", err, err)
	}
	if bbe.Verdict != WatchdogStuckNoProgress {
		t.Fatalf("判词应为 stuck_no_progress，实得 %s", bbe.Verdict)
	}
	if bbe.Obs.Extensions != 2 {
		t.Fatalf("应重置满 2 次（环境变量压到 2），实得 %d", bbe.Obs.Extensions)
	}
}

// 判据 ③：引擎立刻回头 ⇒ ok，且**一次读数都不发生**（不得引入额外读盘）。
func TestWatchdogWire_FastEngineNeverSamples(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer srv.Close()
	calls := 0
	m, _, done := wireTestManager(t, portOf17(t, srv.URL), func(*subproc) func() (uint64, error) {
		return func() (uint64, error) { calls++; return 1, nil }
	})
	defer done()

	resp, err := m.InferForward(context.Background(), "m", "/v1/chat/completions", []byte(`{"model":"m"}`))
	if err != nil {
		t.Fatalf("正常路径不应报错: %v", err)
	}
	_ = resp.Body.Close()
	if calls != 0 {
		t.Fatalf("有输出时看门狗不得采样，实得 %d 次", calls)
	}
}
