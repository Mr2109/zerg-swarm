package api

// handlers_verified_output_test.go — GAP-20261003-21（设计稿 §五 步4 · 加性出参）的回归牙。
//
// 目标：对外面（ModelDetailHandler · GET /api/models/{name}）**也出** verified_state / verified_where，
// 而旧 `"verified": bool` **一字不改**（加性、零破坏）。透传字段、不判不推导（判据 G 在 check_nodes_roster.go）。
//
// ★ 哑化加性（去掉那两枚出参 key）⇒ 本用例红（走 -overlay 负控可复现）。
//
// 写法与 models_registry_test.go 一致：httptest + 真实 chi 路由（取 {name} 路径参数）。

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/config"
	"github.com/Mr2109/zerg-swarm/core/internal/store"
	"github.com/go-chi/chi/v5"
)

func newVerifiedOutHandlers() *Handlers {
	return &Handlers{
		Config: &config.FleetConfig{
			Models: map[string][]config.ModelCandidate{
				// 旧态：只有 verified bool，未用加性键（state 空 = 旧读者按 verified 回落）。
				"M-Old-True": {{Host: "node-a", Backend: "ds4-server", File: "/x.gguf", MemGb: 1, Verified: true}},
				// 第三态：走加性键 reported + 自报机名。
				"M-Reported": {{Host: "node-b", Backend: "ds4-server", File: "/z.gguf", MemGb: 1, VerifiedState: "reported", VerifiedWhere: "edge-node"}},
			},
			Fleet: map[string]config.FleetNode{},
		},
		Store: store.NewStore(),
	}
}

func TestModelDetailHandlerVerifiedStateAdditive(t *testing.T) {
	h := newVerifiedOutHandlers()
	r := chi.NewRouter()
	r.Get("/api/models/{name}", h.ModelDetailHandler)

	getRaw := func(name string) map[string]json.RawMessage {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/api/models/"+name, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("GET /api/models/%s 应 200，实得 %d body=%s", name, w.Code, w.Body.String())
		}
		var m map[string]json.RawMessage
		if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
			t.Fatalf("响应 JSON 解析失败: %v body=%s", err, w.Body.String())
		}
		return m
	}

	// 旧态零破坏：verified 仍是 bool，且旧值逐字保留（true/false 与输入一致）。
	old := getRaw("M-Old-True")
	if string(old["verified"]) != "true" {
		t.Fatalf("旧出参破：M-Old-True 的 \"verified\" 应为 bool true，实得 %s", old["verified"])
	}
	// 加性新键必须在（哑化即在此红）——旧数据 state 为空，透传空串（旧读者按 verified 回落）。
	if _, ok := old["verified_state"]; !ok {
		t.Fatalf("加性出参缺：响应无 \"verified_state\" 键（哑化加性？）全文=%v", old)
	}
	if string(old["verified_state"]) != `""` {
		t.Fatalf(`verified_state 应透传空串（旧读者按 verified 回落），实得 %s`, old["verified_state"])
	}
	if _, ok := old["verified_where"]; !ok {
		t.Fatalf("加性出参缺：响应无 \"verified_where\" 键（哑化加性？）全文=%v", old)
	}

	// 新态：reported + verified_where 必须由出参承载（哑化即在此红）。
	rep := getRaw("M-Reported")
	if string(rep["verified"]) != "false" {
		t.Fatalf("旧出参破：M-Reported 的 \"verified\" 应为 bool false，实得 %s", rep["verified"])
	}
	if string(rep["verified_state"]) != `"reported"` {
		t.Fatalf(`加性出参未承载：verified_state 要 "reported"，实得 %s`, rep["verified_state"])
	}
	if string(rep["verified_where"]) != `"edge-node"` {
		t.Fatalf(`加性出参未承载：verified_where 要 "edge-node"，实得 %s`, rep["verified_where"])
	}
}
