// request_window_test.go —— 缺口 `GAP-20261010-16` 剩余面的子端判据。
//
// 判据面（改前必红 / 改后必绿，逐条可分辨）：
//
//	① 窗口计数**两态**：没发请求 ⇒ 0；真发一发 `/infer` ⇒ 1（✗ 不是「键在、值恒 0」）；
//	② `request_source`：无在飞 ⇒ **逐字「无」**（✗ 空串 / ✗ 缺键）；有在飞 ⇒ 来源逐字；
//	③ 不带 `?window_s=` 时两格**必须出现在 `/status` 回据里**（缺键 = 旧病复发）；
//	④ 窗口可覆盖：同一份记账换窗口读数就变（`?window_s=1` 过 2s ⇒ 0，默认 5min ⇒ 仍 1）；
//	⑤ 零回归：`active_requests` 在以上每一步都仍是它原来的执行期真值（本件不改它）。
//
// 本件不连真网、不起引擎：`/status` 与 `/infer` 都直接打 handler（与既有
// `load_reject_status_test.go` 同一手法）。注册表夹具复用同包的 `registryFromYAML`。
package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Mr2109/zerg-swarm/agent/internal/backend"
)

// gap16Server 造一台只有记账面的子端（不孵卵、不监听端口）。
func gap16Server(t *testing.T) *Server {
	t.Helper()
	reg := registryFromYAML(t, "egg-a:\n"+
		"  backend: llama-server\n"+
		"  file: /nonexistent/egg-a.gguf\n"+
		"  schema_version: 1\n"+
		"  idle_unload_s: 600\n"+
		"  cmd: /nonexistent/zerg-noop -m {file} --port {port}\n")
	return NewServer(NewAgent("node-a", "tok", reg, backend.NewManager(reg, "node-a"), ""))
}

// gap16Status 打一发 `/status`，返回 (HTTP 码, 回据对象)。
func gap16Status(t *testing.T, s *Server, path string) (int, map[string]interface{}) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("X-Auth-Token", "tok")
	rec := httptest.NewRecorder()
	s.handleStatus(rec, req)
	var obj map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &obj); err != nil {
		t.Fatalf("`%s` 回据不是 JSON 对象：%v · body=%s", path, err, rec.Body.String())
	}
	return rec.Code, obj
}

// TestRequestWindowCountsTwoStates —— ① 窗口计数两态（记账层，注入时钟 ⇒ 无 sleep）。
func TestRequestWindowCountsTwoStates(t *testing.T) {
	base := time.Unix(1800000000, 0)
	w := newRequestWindow()
	w.now = func() time.Time { return base }

	n0, src0 := w.snapshot(defaultRequestWindow)
	nSrc0 := len(src0)
	if n0 != 0 || nSrc0 != 0 {
		t.Fatalf("未发请求时应为「0 次 / 无来源」，实得 %d 次 · %d 个来源", n0, nSrc0)
	}

	w.note()
	n1, _ := w.snapshot(defaultRequestWindow)
	if n1 != 1 {
		t.Fatalf("真发 1 次后窗口计数该为 1，实得 %d（两态读数未变 ⇒ 这格没接线）", n1)
	}

	// ④ 同一份记账、只换窗口：2 秒后 1 秒窗口该归 0，5 分钟窗口仍为 1。
	w.now = func() time.Time { return base.Add(2 * time.Second) }
	nShort, _ := w.snapshot(time.Second)
	if nShort != 0 {
		t.Fatalf("`?window_s=1` 在 2s 后该为 0，实得 %d（窗口覆盖没生效）", nShort)
	}
	nLong, _ := w.snapshot(defaultRequestWindow)
	if nLong != 1 {
		t.Fatalf("默认 5 分钟窗口在同一时刻仍该为 1（两态须由窗口决定），实得 %d", nLong)
	}
}

// TestRequestWindowInFlightSources —— ② 在飞来源：进 ⇒ 见、出 ⇒ 无；多路去重排序。
func TestRequestWindowInFlightSources(t *testing.T) {
	w := newRequestWindow()
	if _, srcs := w.snapshot(defaultRequestWindow); len(srcs) != 0 {
		t.Fatalf("无在飞时不该有来源，实得 %v", srcs)
	}

	id := w.enter("<cluster-ip>")
	_, srcs := w.snapshot(defaultRequestWindow)
	nSrc := len(srcs)
	if nSrc != 1 || srcs[0] != "<cluster-ip>" {
		t.Fatalf("在飞 1 路时来源该逐字 `<cluster-ip>`，实得 %d 个：%v", nSrc, srcs)
	}
	w.leave(id)
	_, srcs = w.snapshot(defaultRequestWindow)
	if len(srcs) != 0 {
		t.Fatalf("出飞之后来源该为空，实得 %v（leave 没接线 ⇒ 来源会永远挂着）", srcs)
	}

	// 两路同源 ⇒ 去重后 1 个；两路异源 ⇒ 排序（真源是 map，顺序必须稳定）。
	a := w.enter("<cluster-ip>")
	b := w.enter("<cluster-ip>")
	c := w.enter("<worker-ip>")
	_, srcs = w.snapshot(defaultRequestWindow)
	nSrc = len(srcs)
	if nSrc != 2 || srcs[0] != "<worker-ip>" || srcs[1] != "<cluster-ip>" {
		t.Fatalf("两路异源该去重排序为 [<worker-ip> <cluster-ip>]，实得 %d 个：%v", nSrc, srcs)
	}
	w.leave(a)
	w.leave(b)
	w.leave(c)
}

// TestHandleStatusReportsRequestColumns —— ②③⑤ 走真 handler：两格在、无在飞逐字「无」、
// 有在飞逐字来源，且 `active_requests` 全程不变（零回归）。
func TestHandleStatusReportsRequestColumns(t *testing.T) {
	s := gap16Server(t)

	code, st := gap16Status(t, s, "/status") // ★ 不带 `?window_s=`
	if code != http.StatusOK {
		t.Fatalf("`/status` 该 200，实得 %d", code)
	}
	if _, ok := st["recent_requests"]; !ok {
		t.Fatalf("不带 `?window_s=` 时 `recent_requests` 必须出现在回据里（缺键 = 旧病复发）· 键=%v", st)
	}
	if _, ok := st["request_source"]; !ok {
		t.Fatalf("不带 `?window_s=` 时 `request_source` 必须出现在回据里（缺键 = 旧病复发）· 键=%v", st)
	}
	if got, ok := st["recent_requests"].(float64); !ok || got != 0 {
		t.Fatalf("没发过请求时 `recent_requests` 该是数字 0，实得 %v（类型 %T）", st["recent_requests"], st["recent_requests"])
	}
	if got, _ := st["request_source"].(string); got != requestSourceNone {
		t.Fatalf("无在飞时 `request_source` 该逐字「%s」，实得 %q", requestSourceNone, got)
	}
	if got, ok := st["active_requests"].(float64); !ok || got != 0 {
		t.Fatalf("零回归：`active_requests` 该仍为 0，实得 %v", st["active_requests"])
	}

	// 在飞态：与产品路径同形 —— activeReqs 与 reqWin 同处增减。
	s.agent.mu.Lock()
	s.agent.activeReqs = 1
	s.agent.mu.Unlock()
	id := s.agent.reqWin.enter("<cluster-ip>")
	_, st = gap16Status(t, s, "/status")
	if got, _ := st["request_source"].(string); got != "<cluster-ip>" {
		t.Fatalf("在飞 1 路时 `request_source` 该逐字 `<cluster-ip>`，实得 %q", got)
	}
	if got, ok := st["active_requests"].(float64); !ok || got != 1 {
		t.Fatalf("零回归：在飞 1 路时 `active_requests` 该仍为 1，实得 %v", st["active_requests"])
	}

	s.agent.reqWin.leave(id)
	s.agent.mu.Lock()
	s.agent.activeReqs = 0
	s.agent.mu.Unlock()
	_, st = gap16Status(t, s, "/status")
	if got, _ := st["request_source"].(string); got != requestSourceNone {
		t.Fatalf("出飞之后该回到「%s」，实得 %q", requestSourceNone, got)
	}
}

// TestHandleInferNotesWindowCount —— ① 走真 `/infer`：打一发前 0、打一发后 1。
// 体里故意不给 `model`（handler 会 400）：这一格判的是「**收到过一发**」，不是「推理成功」。
func TestHandleInferNotesWindowCount(t *testing.T) {
	s := gap16Server(t)
	if _, st := gap16Status(t, s, "/status"); st["recent_requests"].(float64) != 0 {
		t.Fatalf("起始该为 0，实得 %v", st["recent_requests"])
	}

	req := httptest.NewRequest(http.MethodPost, "/infer", strings.NewReader(`{}`))
	req.Header.Set("X-Auth-Token", "tok")
	rec := httptest.NewRecorder()
	s.handleInfer(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("体里没有 model 该回 400，实得 %d（body=%s）", rec.Code, rec.Body.String())
	}

	_, st := gap16Status(t, s, "/status")
	if got, ok := st["recent_requests"].(float64); !ok || got != 1 {
		t.Fatalf("真打一发 `/infer` 后该为 1，实得 %v（前态 0 ⇒ 两态须不同）", st["recent_requests"])
	}

	// ④ 窗口覆盖走真 handler：先把时钟推过 2s，再要 1 秒窗口 ⇒ 0；默认窗口 ⇒ 仍 1。
	base := time.Now()
	s.agent.reqWin.now = func() time.Time { return base.Add(2 * time.Second) }
	if _, st := gap16Status(t, s, "/status?window_s=1"); st["recent_requests"].(float64) != 0 {
		t.Fatalf("`?window_s=1` 过 2s 后该为 0，实得 %v（窗口覆盖没生效）", st["recent_requests"])
	}
	if _, st := gap16Status(t, s, "/status"); st["recent_requests"].(float64) != 1 {
		t.Fatalf("默认窗口（5min）同一时刻仍该为 1，实得 %v", st["recent_requests"])
	}
}

// TestHandleStatusWindowParam —— ④ `?window_s=` 的接受面：缺省/正数收，其余**明报拒收**（400）。
func TestHandleStatusWindowParam(t *testing.T) {
	s := gap16Server(t)
	for _, tc := range []struct {
		q    string
		want int
	}{
		{"", http.StatusOK},
		{"?window_s=60", http.StatusOK},
		{"?window_s=86400", http.StatusOK}, // 上限（24h == CLI `--window-min 1440`）
		{"?window_s=0", http.StatusBadRequest},
		{"?window_s=-1", http.StatusBadRequest},
		{"?window_s=abc", http.StatusBadRequest},
		{"?window_s=86401", http.StatusBadRequest},
	} {
		req := httptest.NewRequest(http.MethodGet, "/status"+tc.q, nil)
		req.Header.Set("X-Auth-Token", "tok")
		rec := httptest.NewRecorder()
		s.handleStatus(rec, req)
		if rec.Code != tc.want {
			t.Fatalf("`/status%s` 该回 %d，实得 %d（body=%s）", tc.q, tc.want, rec.Code, rec.Body.String())
		}
		if tc.want == http.StatusBadRequest && !strings.Contains(rec.Body.String(), "window_s") {
			t.Fatalf("拒执理由该点名 `window_s`，实得 body=%s", rec.Body.String())
		}
	}
}

// TestRequestSourceOf —— 来源取值：X-Forwarded-For 首段优先，否则 RemoteAddr 的 IP。
func TestRequestSourceOf(t *testing.T) {
	for _, tc := range []struct {
		name string
		addr string
		xff  string
		want string
	}{
		{"回环带端口", "127.0.0.1:54321", "", "127.0.0.1"},
		{"裸 IP", "192.0.2.5", "", "192.0.2.5"},
		{"网关转发取首段", "127.0.0.1:54321", "203.0.113.7, <controller-ip>", "203.0.113.7"},
		{"XFF 单值", "127.0.0.1:54321", " 203.0.113.9 ", "203.0.113.9"},
	} {
		r := httptest.NewRequest(http.MethodPost, "/infer", nil)
		r.RemoteAddr = tc.addr
		if tc.xff != "" {
			r.Header.Set("X-Forwarded-For", tc.xff)
		}
		if got := requestSourceOf(r); got != tc.want {
			t.Fatalf("%s：来源该为 %q，实得 %q", tc.name, tc.want, got)
		}
	}
}
