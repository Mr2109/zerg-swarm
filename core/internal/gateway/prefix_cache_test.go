package gateway

// prefix_cache_test.go — 丙批 N4 前缀命中率闭环单测（2026-09-10）。
//
// 覆盖：
//  ① 三种（+Anthropic）响应形态的解析——含缺失字段/损坏 JSON 不 panic
//  ② 滑动窗口 ratio 正确（样本数上限 + 时间上限）
//  ③ prompt 版本变更 → 窗口重置 + 基线固化
//  ④ 版本级告警：触发 / 不触发两种情形
//  ⑤ GET /api/metrics/prefix_cache 端点（含认证与 ?model= 聚焦）
//
// 运行：go test ./internal/gateway/ -run PrefixCache -v

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
)

// pvBody 构造带 tools + system 的请求体（控制 prompt 版本）。
func pvBody(tools, system string) []byte {
	return []byte(fmt.Sprintf(
		`{"model":"m","tools":%s,"messages":[{"role":"system","content":%q},{"role":"user","content":"hi"}]}`,
		tools, system))
}

// ---------------------------------------------------------------------------
// ① 解析多形态
// ---------------------------------------------------------------------------

func TestPrefixCacheParseForms(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		wantRead int
		wantMiss int
		wantForm string
		wantOK   bool
	}{
		{
			name:     "openai 式（cached_tokens + prompt_tokens）",
			body:     `{"usage":{"prompt_tokens":69,"completion_tokens":1,"total_tokens":70,"prompt_tokens_details":{"cached_tokens":65}}}`,
			wantRead: 65, wantMiss: 4, wantForm: "openai", wantOK: true,
		},
		{
			// 本仓 llama-server 实测响应：同时给 openai + llamacpp 两形态（优先 openai）
			name:     "llama-server 真实形态（openai+timings 并存）",
			body:     `{"usage":{"prompt_tokens":69,"total_tokens":70,"prompt_tokens_details":{"cached_tokens":65}},"timings":{"cache_n":65,"prompt_n":4,"prompt_ms":641.6}}`,
			wantRead: 65, wantMiss: 4, wantForm: "openai", wantOK: true,
		},
		{
			name:     "llama.cpp 式（仅 timings）",
			body:     `{"timings":{"cache_n":30,"prompt_n":70,"predicted_n":1}}`,
			wantRead: 30, wantMiss: 70, wantForm: "llamacpp", wantOK: true,
		},
		{
			name:     "llama.cpp 式（缺 prompt_n——miss 记 0）",
			body:     `{"timings":{"cache_n":5}}`,
			wantRead: 5, wantMiss: 0, wantForm: "llamacpp", wantOK: true,
		},
		{
			name:     "deepseek 式（hit/miss 两字段）",
			body:     `{"usage":{"prompt_cache_hit_tokens":100,"prompt_cache_miss_tokens":20,"prompt_tokens":120}}`,
			wantRead: 100, wantMiss: 20, wantForm: "deepseek", wantOK: true,
		},
		{
			name:     "anthropic 式（cache_read/cache_creation）",
			body:     `{"usage":{"cache_read_input_tokens":1000,"cache_creation_input_tokens":200,"input_tokens":1200}}`,
			wantRead: 1000, wantMiss: 200, wantForm: "anthropic", wantOK: true,
		},
		{
			name:     "openai 式（缺 prompt_tokens——miss 记 0）",
			body:     `{"usage":{"prompt_tokens_details":{"cached_tokens":12}}}`,
			wantRead: 12, wantMiss: 0, wantForm: "openai", wantOK: true,
		},
		{
			name:     "冷启动（cached=0——仍为有效样本，全 miss）",
			body:     `{"usage":{"prompt_tokens":11,"prompt_tokens_details":{"cached_tokens":0}}}`,
			wantRead: 0, wantMiss: 11, wantForm: "openai", wantOK: true,
		},
		// —— 缺失字段 / 损坏形态：一律 ok=false，且不得 panic ——
		{name: "空 body", body: ``, wantOK: false},
		{name: "损坏 JSON", body: `{bad json`, wantOK: false},
		{name: "无 usage/timings", body: `{"choices":[{"index":0}]}`, wantOK: false},
		{name: "空 usage 与空 timings", body: `{"usage":{},"timings":{}}`, wantOK: false},
		{name: "prompt_tokens_details 空对象", body: `{"usage":{"prompt_tokens_details":{}}}`, wantOK: false},
		{name: "字段类型不符（字符串）", body: `{"usage":{"prompt_cache_hit_tokens":"x"}}`, wantOK: false},
		{name: "usage 不是对象", body: `{"usage":"oops","timings":123}`, wantOK: false},
		{name: "timings 无相关字段", body: `{"timings":{"predicted_n":3,"predicted_ms":1.0}}`, wantOK: false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// 缺失字段不 panic：直接调用（若有越界会 panic 使测试失败）
			read, miss, form, raw, ok := parsePrefixCacheUsage([]byte(c.body))
			if ok != c.wantOK {
				t.Fatalf("ok = %v, want %v（read=%d miss=%d form=%s）", ok, c.wantOK, read, miss, form)
			}
			if !c.wantOK {
				return
			}
			if read != c.wantRead || miss != c.wantMiss {
				t.Fatalf("read/miss = %d/%d, want %d/%d", read, miss, c.wantRead, c.wantMiss)
			}
			if form != c.wantForm {
				t.Fatalf("form = %s, want %s", form, c.wantForm)
			}
			if raw == "" {
				t.Fatalf("raw 片段不应为空（DEBUG 日志用）")
			}
			if len(raw) > 700 {
				t.Fatalf("raw 片段过长（应截断到 600 字符内）：%d", len(raw))
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ② 滑动窗口 ratio
// ---------------------------------------------------------------------------

func TestPrefixCacheWindowRatio(t *testing.T) {
	tr := newPrefixCache(3, time.Hour) // 窗口=最近 3 次
	body := pvBody(`[{"type":"function","function":{"name":"t"}}]`, "S")

	tr.Record("m", body, 8, 2) // 窗口: (8,2)              → 0.80
	tr.Record("m", body, 9, 1) // 窗口: (8,2)(9,1)         → 0.85
	tr.Record("m", body, 5, 5) // 窗口: 三个               → 22/30 = 0.7333
	snap := tr.Snapshot("m")
	if snap.Hits != 22 || snap.Misses != 8 {
		t.Fatalf("三样本 hits/misses = %d/%d, want 22/8", snap.Hits, snap.Misses)
	}
	if snap.Window.Samples != 3 {
		t.Fatalf("窗口样本数 = %d, want 3", snap.Window.Samples)
	}

	// 第 4 次——应挤出最旧样本 (8,2)，保留 (9,1)(5,5)(0,10)
	tr.Record("m", body, 0, 10)
	snap = tr.Snapshot("m")
	if snap.Hits != 14 || snap.Misses != 16 {
		t.Fatalf("滚动后 hits/misses = %d/%d, want 14/16", snap.Hits, snap.Misses)
	}
	if snap.Window.Samples != 3 {
		t.Fatalf("滚动后样本数 = %d, want 3（窗口上限）", snap.Window.Samples)
	}
	if want := 14.0 / 30.0; snap.Ratio-want > 1e-3 || want-snap.Ratio > 1e-3 {
		t.Fatalf("ratio = %v, want ≈%v（snapshot 保留 4 位小数）", snap.Ratio, want)
	}
}

// TestPrefixCacheWindowByTime 时间上限：超 10 分钟（注入时钟）的旧样本被剔除。
func TestPrefixCacheWindowByTime(t *testing.T) {
	tr := newPrefixCache(50, 10*time.Minute)
	cur := time.Now()
	tr.now = func() time.Time { return cur }
	body := pvBody(`[]`, "S")

	tr.Record("m", body, 9, 1) // 旧样本
	cur = cur.Add(11 * time.Minute)
	tr.Record("m", body, 1, 9) // 新样本——旧样本应超龄被剔除

	snap := tr.Snapshot("m")
	if snap.Window.Samples != 1 {
		t.Fatalf("超龄样本未剔除：样本数 = %d, want 1", snap.Window.Samples)
	}
	if snap.Hits != 1 || snap.Misses != 9 {
		t.Fatalf("hits/misses = %d/%d, want 1/9", snap.Hits, snap.Misses)
	}
}

// ---------------------------------------------------------------------------
// ③ 版本变更 → 窗口重置 + 基线固化
// ---------------------------------------------------------------------------

func TestPrefixCacheVersionReset(t *testing.T) {
	tr := newPrefixCache(50, time.Hour)
	bodyA := pvBody(`[{"x":1}]`, "系统提示-A")
	bodyB := pvBody(`[{"x":1}]`, "系统提示-B") // 仅系统提示变化

	if promptVersionOf(bodyA) == promptVersionOf(bodyB) {
		t.Fatal("系统提示变化应产生不同版本号")
	}

	// 版本 A：3 次高命中
	tr.Record("m", bodyA, 9, 1)
	tr.Record("m", bodyA, 9, 1)
	tr.Record("m", bodyA, 9, 1)

	// 切到版本 B：窗口应重置（基线 = 版本 A 命中率 0.9）
	version, changed, _ := tr.Record("m", bodyB, 1, 9)
	if !changed {
		t.Fatal("版本变更应报告 changed=true")
	}
	snap := tr.Snapshot("m")
	if snap.PromptVersion != version {
		t.Fatalf("prompt_version = %s, want %s", snap.PromptVersion, version)
	}
	if snap.Window.Samples != 1 {
		t.Fatalf("版本切换后窗口应重置为 1 个样本，实际 %d", snap.Window.Samples)
	}
	if !snap.HasBaseline || snap.BaselineRatio != 0.9 {
		t.Fatalf("基线应固化为上一版本 0.9，实际 has=%v baseline=%v", snap.HasBaseline, snap.BaselineRatio)
	}
}

// TestPrefixCacheVersionGranularity 工具 schema 一变，版本即变。
func TestPrefixCacheVersionGranularity(t *testing.T) {
	tr := newPrefixCache(50, time.Hour)
	withToolX := pvBody(`[{"type":"function","function":{"name":"x"}}]`, "S")
	withToolY := pvBody(`[{"type":"function","function":{"name":"y"}}]`, "S")
	noTools := pvBody(`[]`, "S")

	// 同一 body 版本稳定
	if promptVersionOf(withToolX) != promptVersionOf(withToolX) {
		t.Fatal("同一请求体版本号应稳定")
	}
	// 工具名变化 → 版本变化
	if promptVersionOf(withToolX) == promptVersionOf(withToolY) {
		t.Fatal("工具名变化应产生不同版本号")
	}
	// 增删工具 → 版本变化
	if promptVersionOf(withToolX) == promptVersionOf(noTools) {
		t.Fatal("工具集合变化应产生不同版本号")
	}
	// 空 body → 空版本
	if promptVersionOf(nil) != "" || promptVersionOf([]byte("{bad")) != "" {
		t.Fatal("空/损坏 body 应返回空版本号")
	}
	_ = tr
}

// ---------------------------------------------------------------------------
// ④ 告警：触发 / 不触发
// ---------------------------------------------------------------------------

func TestPrefixCacheAlertTriggered(t *testing.T) {
	tr := newPrefixCache(50, time.Hour)
	bodyA := pvBody(`[{"x":1}]`, "旧提示")
	bodyB := pvBody(`[{"x":1}]`, "新提示") // 版本变化（模拟改工具/提示）

	// 版本 A：命中率 0.9
	tr.Record("m", bodyA, 9, 1)
	tr.Record("m", bodyA, 9, 1)
	tr.Record("m", bodyA, 9, 1)

	// 版本 B：命中率 0.1——绝对下降 0.8 ≥ 0.3 → 应告警
	var got *PrefixCacheAlert
	for i := 0; i < 3; i++ {
		_, _, a := tr.Record("m", bodyB, 1, 9)
		if a != nil {
			got = a
		}
	}
	if got == nil {
		t.Fatal("命中率显著下降（0.9→0.1）应触发告警")
	}
	if got.BaselineRatio != 0.9 || got.Ratio != 0.1 {
		t.Fatalf("告警 ratio/baseline = %v/%v, want 0.1/0.9", got.Ratio, got.BaselineRatio)
	}
	if got.Reason != "绝对下降 ≥ 30 个百分点" {
		t.Fatalf("触发原因 = %q", got.Reason)
	}
	snap := tr.Snapshot("m")
	if len(snap.Alerts) != 1 {
		t.Fatalf("告警列表应含 1 条，实际 %d", len(snap.Alerts))
	}
	if snap.Alerts[0].Message == "" || snap.Alerts[0].PromptVersion == "" {
		t.Fatal("告警消息/版本号不应为空")
	}
	if b, err := json.Marshal(snap.Alerts[0]); err == nil {
		t.Logf("告警样例: %s", b)
	}
	if b, err := json.Marshal(snap); err == nil {
		t.Logf("端点（含告警）样例: %s", b)
	}
}

func TestPrefixCacheAlertNotTriggered(t *testing.T) {
	// 情形一：版本变化但命中率相当（0.5 → 0.5）——不告警
	tr := newPrefixCache(50, time.Hour)
	bodyA := pvBody(`[{"x":1}]`, "提示A")
	bodyB := pvBody(`[{"x":1}]`, "提示B")
	for i := 0; i < 3; i++ {
		tr.Record("m", bodyA, 5, 5)
	}
	for i := 0; i < 3; i++ {
		if _, _, a := tr.Record("m", bodyB, 5, 5); a != nil {
			t.Fatalf("命中率未下降（0.5→0.5）不应告警，却触发：%s", a.Reason)
		}
	}
	if got := len(tr.Snapshot("m").Alerts); got != 0 {
		t.Fatalf("不应有告警，实际 %d 条", got)
	}

	// 情形二：轻微下降（0.5 → 0.4）——两阈值均未达——不告警
	tr2 := newPrefixCache(50, time.Hour)
	bodyC := pvBody(`[{"x":1}]`, "提示C")
	bodyD := pvBody(`[{"x":1}]`, "提示D")
	for i := 0; i < 3; i++ {
		tr2.Record("m", bodyC, 5, 5)
	}
	for i := 0; i < 3; i++ {
		if _, _, a := tr2.Record("m", bodyD, 4, 6); a != nil {
			t.Fatalf("轻微下降（0.5→0.4）不应告警，却触发：%s", a.Reason)
		}
	}

	// 情形三：首版本无基线——即使低命中也不告警
	tr3 := newPrefixCache(50, time.Hour)
	b := pvBody(`[{"x":1}]`, "首版")
	for i := 0; i < 5; i++ {
		if _, _, a := tr3.Record("m", b, 0, 10); a != nil {
			t.Fatalf("首版本无基线不应告警，却触发：%s", a.Reason)
		}
	}
}

// ---------------------------------------------------------------------------
// ⑤ 端点 GET /api/metrics/prefix_cache
// ---------------------------------------------------------------------------

func TestPrefixCacheEndpoint(t *testing.T) {
	g := &Gateway{
		authToken: "tok-123",
		// 生产默认窗口（50 次 / 10 分钟）——newPrefixCache(0,0) 走默认值
		prefixCache: newPrefixCache(0, 0),
	}
	body := pvBody(`[{"type":"function","function":{"name":"t"}}]`, "端到端提示")
	resp := `{"usage":{"prompt_tokens":100,"prompt_tokens_details":{"cached_tokens":80}},"timings":{"cache_n":80,"prompt_n":20}}`
	g.recordPrefixCache("m", body, []byte(resp))
	g.recordPrefixCache("m", body, []byte(resp))

	r := chi.NewRouter()
	g.RegisterRoutes(r)

	// 无 token → 401（认证中间件生效）
	req := httptest.NewRequest(http.MethodGet, "/api/metrics/prefix_cache", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("无 token 应 401，实际 %d", rec.Code)
	}

	// 带 token → 200 + 规定字段
	req = httptest.NewRequest(http.MethodGet, "/api/metrics/prefix_cache", nil)
	req.Header.Set("X-Auth-Token", "tok-123")
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("带 token 应 200，实际 %d：%s", rec.Code, rec.Body.String())
	}
	t.Logf("端点输出: %s", rec.Body.String())

	var got map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("响应非 JSON: %v", err)
	}
	for _, k := range []string{"prompt_version", "window", "hits", "misses", "ratio", "baseline_ratio", "alerts"} {
		if _, ok := got[k]; !ok {
			t.Fatalf("响应缺少字段 %q：%s", k, rec.Body.String())
		}
	}
	if got["hits"].(float64) != 160 || got["misses"].(float64) != 40 {
		t.Fatalf("hits/misses = %v/%v, want 160/40", got["hits"], got["misses"])
	}
	if got["ratio"].(float64) != 0.8 {
		t.Fatalf("ratio = %v, want 0.8", got["ratio"])
	}
	if got["prompt_version"].(string) == "" {
		t.Fatal("prompt_version 不应为空")
	}
	if _, ok := got["models"].(map[string]interface{}); !ok {
		t.Fatalf("无 ?model= 时应含 models 明细：%s", rec.Body.String())
	}

	// ?model=m 聚焦
	req = httptest.NewRequest(http.MethodGet, "/api/metrics/prefix_cache?model=m", nil)
	req.Header.Set("X-Auth-Token", "tok-123")
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("?model=m 应 200，实际 %d", rec.Code)
	}
	var got2 map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &got2); err != nil {
		t.Fatalf("聚焦响应非 JSON: %v", err)
	}
	if got2["hits"].(float64) != 160 {
		t.Fatalf("聚焦 hits = %v, want 160", got2["hits"])
	}
	if _, present := got2["models"]; present {
		t.Fatal("?model= 聚焦时不应返回 models 汇总")
	}
}

// TestPrefixCacheEndpointNilTracker nil 追踪器（未初始化）不应 panic，返回 disabled。
func TestPrefixCacheEndpointNilTracker(t *testing.T) {
	g := &Gateway{}
	req := httptest.NewRequest(http.MethodGet, "/api/metrics/prefix_cache", nil)
	rec := httptest.NewRecorder()
	g.handlePrefixCacheMetrics(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("nil tracker 应 200，实际 %d", rec.Code)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("响应非 JSON: %v", err)
	}
	if got["status"] != "disabled" {
		t.Fatalf("status = %v, want disabled", got["status"])
	}
}
