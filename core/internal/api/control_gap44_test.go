package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/config"
)

// zz_gap44_verify_test.go —— GAP-20260925-44 的两态机检（**只做 HTTP 级断言**，不引用本笔新增的
// Go 符号 ⇒ 改前/改后源码都能编译，改前必红、改后必绿）。
//
// 判据面：
//  正控 —— 受理面：`POST /api/control/load` 必须回 `ok:true` + `accepted:true` + 非空 `task_id`；
//          查询面：`GET /api/core/status` 的 `load_tasks` 必须含这枚 task_id。
//  负控 —— 未知模型 / 未知机器 / 子端同步拒装（507）/ 打不到子端 ⇒ **不得**回 `accepted`
//          （装载失败 = 未受理）。

func zzSplitHostPort(t *testing.T, raw string) (string, int) {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("解析假子端 URL 失败：%v", err)
	}
	p, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatalf("假子端端口不认：%q", u.Port())
	}
	return u.Hostname(), p
}

func zzSubend(t *testing.T, status int, body string) (string, int) {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/load", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	})
	s := httptest.NewServer(mux)
	t.Cleanup(s.Close)
	return zzSplitHostPort(t, s.URL)
}

func zzCfg(host string, port int) *config.FleetConfig {
	return &config.FleetConfig{
		Models: map[string][]config.ModelCandidate{"m1": {{Host: "fake"}}},
		Fleet:  map[string]config.FleetNode{"fake": {Host: host, Port: port}},
	}
}

func zzPost(h *ControlHandlers, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", "/api/control/load", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.LoadHandler(w, req)
	return w
}

func TestGAP44_LoadAcceptedSemantics(t *testing.T) {
	host, port := zzSubend(t, 200, `{"ok":true,"model":"m1","backend":"llama-server","port":9400}`)
	h := NewControlHandlers("tok", zzCfg(host, port))

	// ── 正控：受理面 ──
	w := zzPost(h, `{"machine":"fake","model":"m1"}`)
	body := w.Body.String()
	if w.Code != 200 {
		t.Fatalf("受理面该 200，得到 %d · body=%s", w.Code, body)
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(body), &m); err != nil {
		t.Fatalf("受理面不是合法 JSON：%v · body=%s", err, body)
	}
	if m["ok"] != true {
		t.Errorf("`ok` 必须保留为 true（只增不改）：body=%s", body)
	}
	if m["accepted"] != true {
		t.Fatalf("改前特征：受理面**没有** `accepted`（改后必须有）：body=%s", body)
	}
	tid, _ := m["task_id"].(string)
	if tid == "" {
		t.Fatalf("受理面**没有** `task_id`（改后必须有）：body=%s", body)
	}
	if s, _ := m["started_at"].(string); s == "" {
		t.Errorf("受理面缺 `started_at`：body=%s", body)
	}

	// ── 正控：查询面（复用既有 `GET /api/core/status`）──
	sreq := httptest.NewRequest("GET", "/api/core/status", nil)
	sw := httptest.NewRecorder()
	h.CoreStatusHandler(sw, sreq)
	sbody := sw.Body.String()
	if !strings.Contains(sbody, "load_tasks") {
		t.Fatalf("改前特征：查询面 `GET /api/core/status` **没有** `load_tasks`（改后必须有）：body=%s", sbody)
	}
	if !strings.Contains(sbody, tid) {
		t.Errorf("查询面 `load_tasks` 里查不到刚受理的 task_id=%s：body=%s", tid, sbody)
	}

	// ── 负控①：未知模型 ⇒ 404 · 不得回 accepted ──
	w = zzPost(h, `{"machine":"fake","model":"nope"}`)
	if w.Code != 404 || !strings.Contains(w.Body.String(), "MODEL_NOT_FOUND") {
		t.Errorf("未知模型该 404 MODEL_NOT_FOUND，得到 %d · body=%s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "accepted") {
		t.Errorf("未知模型（未受理）**不得**回 accepted：body=%s", w.Body.String())
	}

	// ── 负控②：未知机器 ⇒ 404 · 不得回 accepted ──
	w = zzPost(h, `{"machine":"ghost","model":"m1"}`)
	if w.Code != 404 || !strings.Contains(w.Body.String(), "MACHINE_NOT_FOUND") {
		t.Errorf("未知机器该 404 MACHINE_NOT_FOUND，得到 %d · body=%s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "accepted") {
		t.Errorf("未知机器（未受理）**不得**回 accepted：body=%s", w.Body.String())
	}
}

func TestGAP44_LoadFailureNoAccepted(t *testing.T) {
	// ── 负控③：子端**同步拒装**（507）⇒ 原样透传 · 不得回 accepted ──
	host, port := zzSubend(t, 507, `{"ok":false,"status":507,"error":"insufficient memory"}`)
	h := NewControlHandlers("tok", zzCfg(host, port))
	w := zzPost(h, `{"machine":"fake","model":"m1"}`)
	if w.Code != 507 {
		t.Errorf("子端 507 该原样透传，得到 %d · body=%s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "accepted") {
		t.Errorf("子端同步拒装（装载失败）**不得**回 accepted：body=%s", w.Body.String())
	}

	// ── 负控④：打不到子端 ⇒ 502 FORWARD_FAILED · 不得回 accepted ──
	// 127.0.0.1:1 上不会有监听 ⇒ 连接被拒。
	h2 := NewControlHandlers("tok", zzCfg("127.0.0.1", 1))
	w2 := zzPost(h2, `{"machine":"fake","model":"m1"}`)
	if w2.Code != 502 || !strings.Contains(w2.Body.String(), "FORWARD_FAILED") {
		t.Errorf("打不到子端该 502 FORWARD_FAILED，得到 %d · body=%s", w2.Code, w2.Body.String())
	}
	if strings.Contains(w2.Body.String(), "accepted") {
		t.Errorf("打不到子端（未受理）**不得**回 accepted：body=%s", w2.Body.String())
	}
}
