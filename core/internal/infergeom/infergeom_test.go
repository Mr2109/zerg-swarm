// infergeom_test.go — T1.4：批量几何解析的守卫（正例 + 「上游不给则缺席」反例）。
//
// 为什么要有反例：缺字段的比对会**静默通过**（把缺席读成 0 = 处处相等 = 假绿）。
// 所以这里逐项断言 `Has* == false`（而不是"值为 0"）——缺席与 0 必须可分。
package infergeom

import "testing"

// 正例①：本仓 llama-server 实测形态（openai + llamacpp 两形态并存 + system_fingerprint）。
// 实测样本：{"usage":{"prompt_tokens":69,...,"prompt_tokens_details":{"cached_tokens":65}},
//
//	"timings":{"cache_n":65,"prompt_n":4,...},"system_fingerprint":"b10470-34af94cd9"}
func TestParseLlamaServerHitShape(t *testing.T) {
	body := []byte(`{"choices":[{"finish_reason":"stop"}],` +
		`"usage":{"prompt_tokens":69,"completion_tokens":1,"total_tokens":70,"prompt_tokens_details":{"cached_tokens":65}},` +
		`"timings":{"cache_n":65,"prompt_n":4,"predicted_n":1,"prompt_ms":641.6},` +
		`"system_fingerprint":"b10470-34af94cd9"}`)

	g, ok := Parse(body)
	if !ok {
		t.Fatalf("应认出可识别缓存计量（ok=true）")
	}
	// 聚合口径（与原网关解析器一致：llama-server 同时给两形态时优先 openai）
	if g.Form != FormOpenAI || g.CacheRead != 65 || g.CacheMiss != 4 {
		t.Fatalf("形态/聚合口径变了：form=%s read=%d miss=%d（want openai/65/4）", g.Form, g.CacheRead, g.CacheMiss)
	}
	// 几何字段：cache_n/prompt_n 来自 timings（**与形态判定无关**——它们才是决定 logits 的那一栏）
	if !g.HasCacheN || g.CacheN != 65 {
		t.Errorf("cache_n 应记下 65：has=%v v=%d", g.HasCacheN, g.CacheN)
	}
	if !g.HasPromptN || g.PromptN != 4 {
		t.Errorf("prompt_n 应记下 4：has=%v v=%d", g.HasPromptN, g.PromptN)
	}
	if !g.HasCachedTokens || g.CachedTokens != 65 {
		t.Errorf("cached_tokens 应记下 65：has=%v v=%d", g.HasCachedTokens, g.CachedTokens)
	}
	if g.SystemFingerprint != "b10470-34af94cd9" {
		t.Errorf("system_fingerprint 应抄下来，实际 %q", g.SystemFingerprint)
	}
	// 上游没给 ubatch/slot ⇒ 必须缺席（不编默认值）
	if g.HasUbatchN || g.HasSlotID {
		t.Errorf("上游没给 ubatch/slot ⇒ 必须缺席：ubatch=%v slot=%v", g.HasUbatchN, g.HasSlotID)
	}
	if !g.Any() || g.Raw == "" {
		t.Errorf("Any() 应为 true 且 raw 片段非空")
	}
}

// 正例②：冷启动（未命中）——`cache_n=0` / `cached_tokens=0` 是**有效测量值**，必须记下（不是缺席）。
func TestParseColdMissKeepsZero(t *testing.T) {
	body := []byte(`{"usage":{"prompt_tokens":12,"prompt_tokens_details":{"cached_tokens":0}},` +
		`"timings":{"cache_n":0,"prompt_n":12}}`)

	g, ok := Parse(body)
	if !ok {
		t.Fatalf("冷启动仍是有效样本（ok 应为 true）")
	}
	if !g.HasCacheN || g.CacheN != 0 {
		t.Errorf("cache_n=0 是测量值 ⇒ Has=true 且值 0，实际 has=%v v=%d", g.HasCacheN, g.CacheN)
	}
	if !g.HasPromptN || g.PromptN != 12 {
		t.Errorf("prompt_n 应为 12：has=%v v=%d", g.HasPromptN, g.PromptN)
	}
	if !g.HasCachedTokens || g.CachedTokens != 0 {
		t.Errorf("cached_tokens=0 是测量值 ⇒ Has=true，实际 has=%v", g.HasCachedTokens)
	}
	if g.CacheRead != 0 || g.CacheMiss != 12 {
		t.Errorf("聚合口径应为 0/12，实际 %d/%d", g.CacheRead, g.CacheMiss)
	}
	if g.SystemFingerprint != "" {
		t.Errorf("上游没给 fingerprint ⇒ 必须缺席（空），实际 %q", g.SystemFingerprint)
	}
}

// 反例①（本任务的核心反例）：上游**不给 timings/usage** ⇒ 所有几何字段缺席、Any()=false。
// 纪律：**不得**把缺席写成 0（那些 0 会让回放比对静默通过）。
func TestParseNoUpstreamGeometryIsAbsent(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"只有 choices（上游不给任何计量）", `{"choices":[{"finish_reason":"stop"}]}`},
		{"timings 里没有几何键", `{"timings":{"predicted_n":3,"predicted_ms":1.0}}`},
		{"usage 空对象 + timings 空对象", `{"usage":{},"timings":{}}`},
		{"空 body", ``},
		{"损坏 JSON", `{bad json`},
		{"用量字段类型不符（字符串）", `{"timings":{"cache_n":"65","prompt_n":"4"}}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			g, ok := Parse([]byte(c.body))
			if ok {
				t.Fatalf("不识别为缓存计量样本（ok 应为 false）")
			}
			if g.Any() {
				t.Fatalf("拿不到几何 ⇒ Any() 必须 false（不许编造）")
			}
			if g.HasCacheN || g.HasPromptN || g.HasCachedTokens || g.HasUbatchN || g.HasSlotID {
				t.Fatalf("几何字段必须**缺席**（Has*=false），实际 %+v", g)
			}
			if g.SystemFingerprint != "" {
				t.Fatalf("上游没给 fingerprint ⇒ 必须缺席，实际 %q", g.SystemFingerprint)
			}
		})
	}
}

// 反例②：只给 system_fingerprint（没有 timings）——ok=false（不参与命中率统计），
// 但 Any()=true：**上游真给的证据还是要记**（它不是 cache 计量，不等于"什么都没有"）。
func TestParseFingerprintOnlyStillRecorded(t *testing.T) {
	g, ok := Parse([]byte(`{"choices":[],"system_fingerprint":"b10470-34af94cd9"}`))
	if ok {
		t.Fatalf("没有缓存计量 ⇒ ok=false（命中率统计不该被喂样本）")
	}
	if !g.Any() {
		t.Fatalf("fingerprint 是上游真给的证据 ⇒ Any() 应为 true")
	}
	if g.SystemFingerprint != "b10470-34af94cd9" {
		t.Fatalf("fingerprint 应抄下来，实际 %q", g.SystemFingerprint)
	}
	if g.HasCacheN || g.HasPromptN || g.HasCachedTokens {
		t.Fatalf("cache 计量缺席就必须 Has*=false")
	}
}

// ubatch_n / slot_id：候选键命中才记（llama.cpp 的 chat 接口通常不给 ⇒ 缺席）。
func TestParseUbatchAndSlotCandidates(t *testing.T) {
	g, _ := Parse([]byte(`{"timings":{"cache_n":8,"prompt_n":2,"ubatch_n":1024},"slot_id":3}`))
	if !g.HasUbatchN || g.UbatchN != 1024 {
		t.Errorf("timings.ubatch_n 应记下：has=%v v=%d", g.HasUbatchN, g.UbatchN)
	}
	if !g.HasSlotID || g.SlotID != 3 {
		t.Errorf("顶层 slot_id 应记下：has=%v v=%d", g.HasSlotID, g.SlotID)
	}

	g2, _ := Parse([]byte(`{"timings":{"cache_n":8,"prompt_n":2},"id_slot":7}`))
	if !g2.HasSlotID || g2.SlotID != 7 {
		t.Errorf("id_slot 候选应记下：has=%v v=%d", g2.HasSlotID, g2.SlotID)
	}

	// 都没有 ⇒ 缺席（不编 0）
	g3, _ := Parse([]byte(`{"timings":{"cache_n":8,"prompt_n":2}}`))
	if g3.HasUbatchN || g3.HasSlotID {
		t.Errorf("上游没给 ⇒ ubatch/slot 必须缺席")
	}
}

// CarriesGeometry：流式筛块的粗筛（只看键名，不解析）——不得漏掉末块，也不得误报纯 delta 块。
func TestCarriesGeometry(t *testing.T) {
	cases := []struct {
		payload string
		want    bool
	}{
		{`{"choices":[{"delta":{"content":"hi"}}]}`, false},
		{`{"choices":[{"delta":{}}],"timings":{"cache_n":0,"prompt_n":12}}`, true},
		{`{"choices":[{"delta":{}}],"usage":{"prompt_tokens":12}}`, true},
		{`{"choices":[{"delta":{}}],"system_fingerprint":"b10470-34af94cd9"}`, true},
		{`{"choices":[{"delta":{"content":"usages of stuff"}}]}`, false}, // 正文里出现 usage 字样不算
		{``, false},
	}
	for _, c := range cases {
		if got := CarriesGeometry([]byte(c.payload)); got != c.want {
			t.Errorf("CarriesGeometry(%s) = %v, want %v", c.payload, got, c.want)
		}
	}
}
