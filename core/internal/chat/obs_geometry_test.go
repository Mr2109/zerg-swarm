// obs_geometry_test.go — T1.4 验收：批量几何写进既有事件流（chat_obs.jsonl 同一套格式）。
//
// 走**真实路径**验证（不是直接调几何函数）：
//
//	httptest 假上游（llama-server SSE 形态）→ ChatInfer.InferStream（既有解析器 infergeom）
//	→ ObsTimer.SetGeometry → Finish → chat_obs.jsonl 里出现 geometry 块。
//
// 三条用例（缺一不可）：
// ① 正例·缓存命中：cache_n/prompt_n/cached_tokens/system_fingerprint 都要落盘且值正确；
// ② 正例·冷启动未命中：cache_n=0 / cached_tokens=0 是**有效测量值** ⇒ 必须落盘（与缺席可分）；
// ③ 反例·上游不给：字段必须**缺席**（不得写成 0）——只有 geometry_recorded:false 的结论。
//
// 另加：几何只挂「大模型调用」那一类（kind=turn）；工具/压缩记录**不得**带 geometry（要求 4）。
package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeGateway — 假上游：按 llama-server 的 SSE 形态回一块内容 + 可选末块（带 timings/usage/fingerprint）。
func fakeGateway(t *testing.T, finalChunk string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":\"好\"},\"finish_reason\":null}]}\n\n")
		if finalChunk != "" {
			fmt.Fprintf(w, "data: %s\n\n", finalChunk)
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	t.Cleanup(srv.Close)
	return srv
}

// runTurnWithUpstream — 跑一轮（真流式解析 → 落盘），返回 chat_obs.jsonl 的**最后一行**。
func runTurnWithUpstream(t *testing.T, session string, round int, finalChunk string) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", dir)

	srv := fakeGateway(t, finalChunk)
	inf := NewChatInfer(srv.URL, "")

	timer := NewObsTimer(session, round, "gemma-4-26B")
	res, err := inf.InferStream(context.Background(), "gemma-4-26B", "sys",
		[]map[string]any{{"role": "user", "content": "hi"}}, nil)
	if err != nil {
		t.Fatalf("流式推理失败：%v", err)
	}
	timer.SetGeometry(res.Geometry) // ← 与 internal/api 的轮次适配器同一接线点
	timer.Finish("finish")

	b, err := os.ReadFile(filepath.Join(dir, obsFileName))
	if err != nil {
		t.Fatalf("观测文件未落盘：%v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	return lines[len(lines)-1]
}

// decodeTurn — 解一行 turn 记录（kind + geometry 原样）。
func decodeTurn(t *testing.T, line string) (string, map[string]any) {
	t.Helper()
	var rec struct {
		Kind     string         `json:"kind"`
		Geometry map[string]any `json:"geometry"`
	}
	if err := json.Unmarshal([]byte(line), &rec); err != nil {
		t.Fatalf("记录不是合法 JSON：%v\n%s", err, line)
	}
	return rec.Kind, rec.Geometry
}

// 正例①：缓存命中 —— 几何字段全部落盘且值正确。
func TestObsGeometryCacheHitLands(t *testing.T) {
	final := `{"choices":[{"delta":{},"finish_reason":"stop"}],` +
		`"usage":{"prompt_tokens":69,"completion_tokens":1,"total_tokens":70,"prompt_tokens_details":{"cached_tokens":65}},` +
		`"timings":{"cache_n":65,"prompt_n":4,"predicted_n":1},` +
		`"system_fingerprint":"b10470-34af94cd9"}`
	line := runTurnWithUpstream(t, "sess-G1", 1, final)

	kind, geom := decodeTurn(t, line)
	if kind != "turn" {
		t.Fatalf("几何应挂在**大模型调用**（kind=turn）记录上，实际 kind=%q：%s", kind, line)
	}
	want := map[string]float64{"cache_n": 65, "prompt_n": 4, "cached_tokens": 65}
	for k, v := range want {
		got, ok := geom[k]
		if !ok {
			t.Fatalf("缺少几何字段 %s：%s", k, line)
		}
		if f, _ := got.(float64); f != v {
			t.Errorf("%s = %v, want %v", k, got, v)
		}
	}
	if geom["system_fingerprint"] != "b10470-34af94cd9" {
		t.Errorf("system_fingerprint 未落盘或值错：%v", geom["system_fingerprint"])
	}
	if geom["geometry_recorded"] != true {
		t.Errorf("geometry_recorded 应为 true：%v", geom["geometry_recorded"])
	}
	// 上游没给 ubatch/slot ⇒ 必须缺席（不编造）
	for _, k := range []string{"ubatch_n", "slot_id"} {
		if _, ok := geom[k]; ok {
			t.Errorf("上游没给 %s ⇒ 必须缺席：%s", k, line)
		}
	}
}

// 正例②：冷启动（未命中）——0 是有效测量值，必须落盘（否则读侧无法区分"命不中"与"没数据"）。
func TestObsGeometryColdMissKeepsZero(t *testing.T) {
	final := `{"choices":[{"delta":{},"finish_reason":"stop"}],` +
		`"usage":{"prompt_tokens":12,"prompt_tokens_details":{"cached_tokens":0}},` +
		`"timings":{"cache_n":0,"prompt_n":12}}`
	line := runTurnWithUpstream(t, "sess-G2", 1, final)

	_, geom := decodeTurn(t, line)
	for _, k := range []string{"cache_n", "prompt_n", "cached_tokens"} {
		if _, ok := geom[k]; !ok {
			t.Fatalf("冷启动的 0 是有效测量值 ⇒ %s 必须落盘：%s", k, line)
		}
	}
	if geom["cache_n"] != float64(0) {
		t.Errorf("cache_n 应为 0（命不中），实际 %v", geom["cache_n"])
	}
	if geom["prompt_n"] != float64(12) {
		t.Errorf("prompt_n 应为 12（本次实评估），实际 %v", geom["prompt_n"])
	}
	if geom["geometry_recorded"] != true {
		t.Errorf("geometry_recorded 应为 true：%v", geom["geometry_recorded"])
	}
	if _, ok := geom["system_fingerprint"]; ok {
		t.Errorf("上游没给 fingerprint ⇒ 必须缺席：%s", line)
	}
}

// 反例③：上游**不给** timings/usage/system_fingerprint ⇒ 几何字段必须缺席（不得写成 0）。
// 这是本任务最关键的守卫：缺字段的回放比对会静默通过（字段全空 = 处处相等 = 假绿）。
func TestObsGeometryAbsentWhenUpstreamSilent(t *testing.T) {
	// 末块只有 choices（没有 timings/usage/fingerprint）
	line := runTurnWithUpstream(t, "sess-G3", 1, `{"choices":[{"delta":{},"finish_reason":"stop"}]}`)

	kind, geom := decodeTurn(t, line)
	if kind != "turn" {
		t.Fatalf("kind=%q（应仍是大模型调用记录）", kind)
	}
	for _, k := range []string{"cache_n", "prompt_n", "cached_tokens", "ubatch_n", "slot_id", "system_fingerprint"} {
		if v, ok := geom[k]; ok {
			t.Errorf("上游没给 %s ⇒ 必须**缺席**（不得写成 %v）：%s", k, v, line)
		}
	}
	if geom["geometry_recorded"] != false {
		t.Errorf("geometry_recorded 应为 false（问过了、上游什么都没给）：%v", geom["geometry_recorded"])
	}
	// 记录本身照常完整（观测不得因为缺几何而丢记录）
	if !strings.Contains(line, `"kind":"turn"`) || !strings.Contains(line, `"end_reason":"finish"`) {
		t.Errorf("缺几何不得影响记录本身：%s", line)
	}
}

// 要求④：几何只挂「大模型调用」（turn）；工具/压缩记录不得带 geometry。
func TestObsGeometryOnlyOnLLMCall(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", dir)

	NewObsTimer("sess-G4", 1, "m").Finish("finish")
	ObsTool("sess-G4", 1, "bash", "3ms", true, 1, MaxToolRounds)
	obsCompact("sess-G4", "threshold", 10, 2, 100, 0, "ok", "", false)

	b, err := os.ReadFile(filepath.Join(dir, obsFileName))
	if err != nil {
		t.Fatalf("观测文件未落盘：%v", err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		kind, _ := decodeTurn(t, line)
		hasGeom := strings.Contains(line, `"geometry"`)
		if kind == "turn" && !hasGeom {
			t.Errorf("turn 记录必须带 geometry 结论块：%s", line)
		}
		if kind != "turn" && hasGeom {
			t.Errorf("几何只挂大模型调用（kind=turn），%s 记录不得带：%s", kind, line)
		}
	}
}
