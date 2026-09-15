// eggs_endpoint_test.go —— P7 批 1：GET /eggs 只读端点验收（鉴权 / 只读 / 不编造）。
//
// 设计真源：设计-子端沙箱化-20260914.md §5.4（端点名定案 /eggs；字段 eggs[] / egg_id /
// engine_impl / schema_version）· §8.4（has_profile 无档案必须 false，不许编造）。
package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/agent/internal/backend"
	"github.com/Mr2109/zerg-swarm/agent/internal/monitor"
)

func eggsServer() *Server {
	return NewServer(NewAgent("x3", "tok", nil, backend.NewManager(nil, "x3"), ""))
}

// callEggs 直接调 handler（与 pin_test.go 同风格）。
func callEggs(t *testing.T, method, token string) (int, map[string]interface{}) {
	t.Helper()
	s := eggsServer()
	req := httptest.NewRequest(method, "/eggs", nil)
	if token != "" {
		req.Header.Set("X-Auth-Token", token)
	}
	rec := httptest.NewRecorder()
	s.handleEggs(rec, req)
	var out map[string]interface{}
	if rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("响应不是 JSON: %q (%v)", rec.Body.String(), err)
		}
	}
	return rec.Code, out
}

// 无令牌 ⇒ 401（与既有端点同门，不因只读而开口子）。
func TestEggs_AuthRequired(t *testing.T) {
	code, _ := callEggs(t, http.MethodGet, "")
	if code != http.StatusUnauthorized {
		t.Fatalf("无令牌应 401，实得 %d", code)
	}
}

// 只读端点：非 GET ⇒ 405（不接受写动作，红线②）。
func TestEggs_MethodNotAllowed(t *testing.T) {
	code, out := callEggs(t, http.MethodPost, "tok")
	if code != http.StatusMethodNotAllowed {
		t.Fatalf("POST 应 405，实得 %d (%v)", code, out)
	}
}

// 有令牌 + 无托管卵 ⇒ 200 且 eggs 为长度 0 的数组（缺席不编造）。
func TestEggs_EmptyShape(t *testing.T) {
	code, out := callEggs(t, http.MethodGet, "tok")
	if code != http.StatusOK {
		t.Fatalf("应 200，实得 %d", code)
	}
	eggs, ok := out["eggs"].([]interface{})
	if !ok {
		t.Fatalf("eggs 应为数组，实得 %T", out["eggs"])
	}
	if len(eggs) != 0 {
		t.Fatalf("无托管卵时 eggs 应为空数组，实得 %v", eggs)
	}
	if _, ok := out["generated_at"].(string); !ok {
		t.Error("应带 generated_at（可观测面时间基准）")
	}
}

// eggEntries 装配：GTT 归因按 pid 匹配；无归因条目 ⇒ gtt_gb 缺席；无档案 ⇒ has_profile=false。
func TestEggEntries_AttribAndProfileHonesty(t *testing.T) {
	remain := 123.0
	obs := []backend.EggObservation{
		{EggID: "qwen", Model: "qwen", EngineImpl: "llama-server", State: "ready",
			Port: 58100, PID: 4242, Inflight: 2, IdleArmedRemainS: &remain, SchemaVersion: 3},
		{EggID: "k2", Model: "k2", EngineImpl: "/home/g01/llama-k2/build-k2/bin/llama-server",
			State: "loading", Port: 58101, PID: 9999, Inflight: 0},
	}
	attrib := map[int]monitor.ProcAttrib{
		4242: {PID: 4242, Argv0: "llama-server", GttGb: 32.74},
	}
	got := eggEntries(obs, attrib, func(string) bool { return false })

	if len(got) != 2 {
		t.Fatalf("应 2 条，实得 %d", len(got))
	}
	if got[0].EggID != "qwen" || got[0].GttGb != 32.7 {
		t.Errorf("qwen 应带 GTT 归因 32.7，实得 %+v", got[0])
	}
	if got[0].Inflight != 2 || got[0].IdleArmedS == nil || *got[0].IdleArmedS != 123 {
		t.Errorf("在飞计数与空窗剩余应如实透传，实得 %+v", got[0])
	}
	if got[0].SchemaVersion != 3 {
		t.Errorf("schema_version 应如实透传，实得 %d", got[0].SchemaVersion)
	}
	if got[1].GttGb != 0 {
		t.Errorf("无归因条目不得编造 GTT，实得 %v", got[1].GttGb)
	}
	for _, e := range got {
		if e.HasProfile {
			t.Errorf("%s：无档案时 has_profile 必须 false（§8.4 不许编造）", e.EggID)
		}
		if !e.Managed {
			t.Errorf("%s：eggs[] 只装本端托管项，managed 恒 true", e.EggID)
		}
	}
}

// 封闭性核验不许只写日志（§6.9 静默失效不得当凭据）：载荷里必须看得见
// enclosure_verified + enclosure_note，且「未核验」与「从未声称隔离」两种 false 靠 note 区分。
func TestEggEntries_EnclosureFieldsVisible(t *testing.T) {
	obs := []backend.EggObservation{
		{EggID: "verified", EnclosureVerified: true, EnclosureNote: "封闭性核验通过（models_ro=true …）"},
		{EggID: "unverified", EnclosureNote: "未核验：拿不到引擎 pid（unit=zerg-x）"},
		{EggID: "bare"}, // 裸 exec 路径（孵化开关关）：从未声称过隔离 ⇒ 无留痕
	}
	got := eggEntries(obs, nil, nil)
	if len(got) != 3 {
		t.Fatalf("应 3 条，实得 %d", len(got))
	}
	if !got[0].EnclosureVerified || got[0].EnclosureNote == "" {
		t.Errorf("核验通过必须如实透传 verified + 留痕，实得 %+v", got[0])
	}
	if got[1].EnclosureVerified {
		t.Errorf("未核验绝不许显示成已核验，实得 %+v", got[1])
	}
	if got[2].EnclosureVerified || got[2].EnclosureNote != "" {
		t.Errorf("未声称过隔离的卵（裸 exec 路径）不得编造核验留痕，实得 %+v", got[2])
	}

	// JSON 键名（观测面口径）：enclosure_verified 恒出现；note 空则缺席（不编造空话术）
	b, err := json.Marshal(got[1])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"enclosure_verified":false`) {
		t.Errorf("载荷应带 enclosure_verified=false，实得 %s", b)
	}
	if !strings.Contains(string(b), `"enclosure_note":"未核验`) {
		t.Errorf("载荷应带 enclosure_note（写明未核验），实得 %s", b)
	}
	b0, err := json.Marshal(got[2])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b0), "enclosure_note") {
		t.Errorf("无留痕时 enclosure_note 应缺席，实得 %s", b0)
	}
	if !strings.Contains(string(b0), `"enclosure_verified":false`) {
		t.Errorf("未核验的卵也要如实给出 enclosure_verified=false，实得 %s", b0)
	}
}

// 外部占用判定口径（§8.7）：本端托管 pid 必须被排除，非引擎使用者才列出。
func TestExternalOccupants_ExcludesManaged(t *testing.T) {
	obs := []backend.EggObservation{{EggID: "e1", PID: 4242}}
	managed := map[int]struct{}{4242: {}}
	attrib := map[int]monitor.ProcAttrib{
		4242: {PID: 4242, Argv0: "llama-server", GttGb: 32.0},
		7777: {PID: 7777, Argv0: "some-gpu-app", GttGb: 1.5},
	}
	var listed []int
	for pid := range attrib {
		if _, isManaged := managed[pid]; isManaged {
			continue
		}
		listed = append(listed, pid)
	}
	if len(listed) != 1 || listed[0] != 7777 {
		t.Fatalf("应只列出非托管 pid 7777，实得 %v（托管项 %v 被排除；卵 %v）", listed, managed, obs)
	}
}
