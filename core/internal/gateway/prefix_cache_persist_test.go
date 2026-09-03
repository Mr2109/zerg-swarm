package gateway

// prefix_cache_persist_test.go — 丙批 C2 前缀命中率收尾单测（2026-09-11）。
//
// 覆盖：
//  ① 持久化往返：写入后新实例 load 得到同 ratio/基线/告警/未知统计
//  ② 损坏文件不崩：多种损坏形态 → 忽略并从空态开始，恢复后可正常落盘
//  ③ 节流写：窗口内不写；累计 10 个样本 / 超 5s / Flush 三种触发路径，且最后一批不丢
//  ④ 未知响应形态：计数 + 键名列表（不含值）+ 端点字段
//
// 运行：go test ./internal/gateway/ -run PrefixCache -v

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
)

// ---------------------------------------------------------------------------
// ① 持久化往返
// ---------------------------------------------------------------------------

func TestPrefixCachePersistRoundTrip(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "prefix_cache.json")

	bodyA := pvBody(`[{"type":"function","function":{"name":"t"}}]`, "提示A")
	bodyB := pvBody(`[{"type":"function","function":{"name":"t"}}]`, "提示B")

	fixed := time.UnixMilli(1_700_000_000_000)

	tr1 := newPrefixCache(50, time.Hour)
	tr1.now = func() time.Time { return fixed }
	tr1.enablePersistence(file)

	// 版本 A：3 次高命中（ratio 0.9）
	tr1.Record("m", bodyA, 9, 1)
	tr1.Record("m", bodyA, 9, 1)
	tr1.Record("m", bodyA, 9, 1)
	// 切到版本 B：固化基线（=0.9），窗口重置为 1 个样本 (1,9)
	tr1.Record("m", bodyB, 1, 9)
	// 未知形态统计也应随盘持久化
	tr1.markUnknown([]string{"timings", "usage", "usage.prompt_tokens"})
	tr1.Flush()

	want := tr1.Snapshot("m")

	// 落盘文件应存在且格式正确
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("落盘文件应存在: %v", err)
	}
	t.Logf("落盘样例(%s):\n%s", file, b)
	var pf persistedPrefixCache
	if err := json.Unmarshal(b, &pf); err != nil {
		t.Fatalf("落盘文件非合法 JSON: %v", err)
	}
	if pf.Version != persistedPrefixCacheVersion {
		t.Fatalf("落盘 version = %d, want %d", pf.Version, persistedPrefixCacheVersion)
	}
	pm := pf.Models["m"]
	if pm.BaselineRatio != 0.9 || !pm.HasBaseline {
		t.Fatalf("落盘基线 = %v (has=%v), want 0.9/true", pm.BaselineRatio, pm.HasBaseline)
	}
	if pm.Hits != 1 || pm.Misses != 9 {
		t.Fatalf("落盘窗口 hits/misses = %d/%d, want 1/9（版本 B 窗口）", pm.Hits, pm.Misses)
	}
	if len(pm.Samples) != 1 {
		t.Fatalf("落盘样本数 = %d, want 1", len(pm.Samples))
	}
	if pf.UnknownForms != 1 {
		t.Fatalf("落盘 unknown_forms = %d, want 1", pf.UnknownForms)
	}

	// 模拟重启：新实例 load
	tr2 := newPrefixCache(50, time.Hour)
	tr2.now = func() time.Time { return fixed }
	tr2.enablePersistence(file)

	got := tr2.Snapshot("m")
	if got.Hits != want.Hits || got.Misses != want.Misses || got.Ratio != want.Ratio {
		t.Fatalf("重启后 hits/misses/ratio = %d/%d/%v, want %d/%d/%v",
			got.Hits, got.Misses, got.Ratio, want.Hits, want.Misses, want.Ratio)
	}
	if got.BaselineRatio != want.BaselineRatio || got.HasBaseline != want.HasBaseline {
		t.Fatalf("重启后基线 = %v (has=%v), want %v (has=%v)",
			got.BaselineRatio, got.HasBaseline, want.BaselineRatio, want.HasBaseline)
	}
	if got.PromptVersion != want.PromptVersion {
		t.Fatalf("重启后 prompt_version = %s, want %s", got.PromptVersion, want.PromptVersion)
	}
	if got.UnknownForms != 1 || !reflect.DeepEqual(got.UnknownLastKeys, []string{"timings", "usage", "usage.prompt_tokens"}) {
		t.Fatalf("重启后未知统计 = %d/%v, want 1/[timings usage usage.prompt_tokens]",
			got.UnknownForms, got.UnknownLastKeys)
	}
	t.Logf("重启后快照: ratio=%v baseline=%v hits=%d misses=%d unknown=%d",
		got.Ratio, got.BaselineRatio, got.Hits, got.Misses, got.UnknownForms)
}

// ---------------------------------------------------------------------------
// ② 损坏文件不崩
// ---------------------------------------------------------------------------

func TestPrefixCachePersistCorruptFile(t *testing.T) {
	corrupts := map[string]string{
		"截断 JSON": `{"models": {`,
		"纯垃圾":     `not-json-at-all`,
		"空文件":     ``,
		"JSON 数组": `[1,2,3]`,
		"字段类型不符":  `{"models": 123}`,
	}
	body := pvBody(`[]`, "S")
	for name, content := range corrupts {
		t.Run(name, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "prefix_cache.json")
			if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
			// load 不得 panic
			tr := newPrefixCache(50, time.Hour)
			tr.enablePersistence(file)
			snap := tr.Snapshot("m")
			if snap.Status != "ok" || snap.Hits != 0 || snap.Misses != 0 {
				t.Fatalf("损坏文件应被忽略（空态），实际 status=%s hits=%d misses=%d",
					snap.Status, snap.Hits, snap.Misses)
			}
			// 损坏后仍能正常记录并重新落盘（合法 JSON）
			tr.Record("m", body, 5, 5)
			tr.Flush()
			b, err := os.ReadFile(file)
			if err != nil {
				t.Fatalf("损坏恢复后应可重新落盘: %v", err)
			}
			if !json.Valid(b) {
				t.Fatalf("重新落盘应为合法 JSON: %s", b)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ③ 节流写（注入时钟）—— 最后一批不丢
// ---------------------------------------------------------------------------

func TestPrefixCachePersistThrottleNoLoss(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "prefix_cache.json")
	body := pvBody(`[]`, "S")
	cur := time.UnixMilli(1_700_000_000_000)

	tr := newPrefixCache(50, time.Hour)
	tr.now = func() time.Time { return cur }
	tr.enablePersistence(file) // 节流基准 = cur

	// 同一时刻连续 5 个样本：未达 10 个阈值、未超 5s → 不写盘
	for i := 0; i < 5; i++ {
		tr.Record("m", body, 9, 1)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatalf("节流窗口内不应写盘，实际文件已存在（err=%v）", err)
	}
	// 推进 3s，再记 1 个样本：仍 <5s 且 <10 个 → 仍不写
	cur = cur.Add(3 * time.Second)
	tr.Record("m", body, 9, 1)
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatal("3s 内仍不应写盘（节流）")
	}
	// Flush 强制落盘 → 最后一批（6 个样本）不得丢
	tr.Flush()
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("Flush 后应落盘: %v", err)
	}
	var pf persistedPrefixCache
	if err := json.Unmarshal(b, &pf); err != nil {
		t.Fatalf("落盘非 JSON: %v", err)
	}
	if got := len(pf.Models["m"].Samples); got != 6 {
		t.Fatalf("Flush 后样本数 = %d, want 6（最后一批不得丢）", got)
	}
	if pf.Models["m"].Hits != 54 || pf.Models["m"].Misses != 6 {
		t.Fatalf("hits/misses = %d/%d, want 54/6", pf.Models["m"].Hits, pf.Models["m"].Misses)
	}

	// 触发路径二：累计 10 个样本自动写盘（无需 Flush）
	file2 := filepath.Join(dir, "prefix_cache2.json")
	tr2 := newPrefixCache(50, time.Hour)
	tr2.now = func() time.Time { return cur }
	tr2.enablePersistence(file2)
	for i := 0; i < 10; i++ {
		tr2.Record("m", body, 1, 0)
	}
	if _, err := os.Stat(file2); err != nil {
		t.Fatalf("累计 10 个样本应触发自动写盘: %v", err)
	}

	// 触发路径三：单个样本后推进 6s，再记 1 个 → 触发写盘
	file3 := filepath.Join(dir, "prefix_cache3.json")
	tr3 := newPrefixCache(50, time.Hour)
	t3 := cur
	tr3.now = func() time.Time { return t3 }
	tr3.enablePersistence(file3)
	tr3.Record("m", body, 1, 0)
	if _, err := os.Stat(file3); !os.IsNotExist(err) {
		t.Fatal("单样本（未超时）不应立即写盘")
	}
	t3 = t3.Add(6 * time.Second)
	tr3.Record("m", body, 1, 0)
	if _, err := os.Stat(file3); err != nil {
		t.Fatalf("超 5s 后的样本应触发写盘: %v", err)
	}
}

// ---------------------------------------------------------------------------
// ④ 未知响应形态探测
// ---------------------------------------------------------------------------

func TestPrefixCacheUnknownForm(t *testing.T) {
	g := &Gateway{authToken: "tok", prefixCache: newPrefixCache(0, 0)}
	body := pvBody(`[]`, "S")

	// ① 可解析 JSON 但无 usage/timings → 计为未知
	g.recordPrefixCache("m", body, []byte(`{"choices":[{"index":0,"text":"x"}]}`))
	// ② 有 usage/timings 但字段不认识 → 计为未知，键名含 usage./timings.
	g.recordPrefixCache("m", body, []byte(`{"usage":{"prompt_tokens":12},"timings":{"predicted_n":3}}`))
	// ③ 损坏 JSON / 非对象 / 空 → 无法提取键名，不计数（不误判为形态）
	g.recordPrefixCache("m", body, []byte(`{bad json`))
	g.recordPrefixCache("m", body, []byte(`[1,2,3]`))
	g.recordPrefixCache("m", body, []byte(``))
	// ④ 已知形态不应计入未知
	g.recordPrefixCache("m", body, []byte(`{"usage":{"prompt_tokens":10,"prompt_tokens_details":{"cached_tokens":8}}}`))

	snap := g.prefixCache.Snapshot("")
	if snap.UnknownForms != 2 {
		t.Fatalf("unknown_forms = %d, want 2（仅①②计；损坏/非对象/已知形态不计）", snap.UnknownForms)
	}
	wantKeys := []string{"timings", "usage", "usage.prompt_tokens", "timings.predicted_n"}
	if !reflect.DeepEqual(snap.UnknownLastKeys, wantKeys) {
		t.Fatalf("unknown_last_keys = %v, want %v", snap.UnknownLastKeys, wantKeys)
	}
	// 已知形态确实被记录（hits/misses 不为零）
	if snap.Hits != 8 || snap.Misses != 2 {
		t.Fatalf("已知形态 hits/misses = %d/%d, want 8/2", snap.Hits, snap.Misses)
	}
	t.Logf("未知统计: count=%d lastKeys=%v", snap.UnknownForms, snap.UnknownLastKeys)
}

// TestPrefixCacheUnknownFormEndpoint 端点应多返回 unknown_forms + unknown_last_keys。
func TestPrefixCacheUnknownFormEndpoint(t *testing.T) {
	g := &Gateway{authToken: "tok-123", prefixCache: newPrefixCache(0, 0)}
	body := pvBody(`[]`, "S")
	g.recordPrefixCache("m", body, []byte(`{"choices":[{"index":0}]}`))
	g.recordPrefixCache("m", body, []byte(`{"object":"chat.completion","usage":{"prompt_tokens":5}}`))

	r := chi.NewRouter()
	g.RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodGet, "/api/metrics/prefix_cache", nil)
	req.Header.Set("X-Auth-Token", "tok-123")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("应 200，实际 %d：%s", rec.Code, rec.Body.String())
	}
	t.Logf("端点输出: %s", rec.Body.String())

	var got map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("响应非 JSON: %v", err)
	}
	if got["unknown_forms"].(float64) != 2 {
		t.Fatalf("unknown_forms = %v, want 2", got["unknown_forms"])
	}
	kl, ok := got["unknown_last_keys"].([]interface{})
	if !ok || len(kl) == 0 {
		t.Fatalf("unknown_last_keys 缺失：%s", rec.Body.String())
	}
	// 最近一条 = 第二条（object + usage.prompt_tokens）
	want := []string{"object", "usage", "usage.prompt_tokens"}
	if len(kl) != len(want) {
		t.Fatalf("unknown_last_keys = %v, want %v", kl, want)
	}
	for i := range want {
		if kl[i] != want[i] {
			t.Fatalf("unknown_last_keys = %v, want %v", kl, want)
		}
	}
}
