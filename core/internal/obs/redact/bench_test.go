package redact

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

// ── 基准测试（任务表 T2.7 / D6：性能与限额）──────────────────────────────────
//
// 夹具与**原型同口径**：`sampleEvent(50)` 逐字段照抄 `~/zerg-redaction-probe`
// 的 `sampleEvent`（设计附件 §三「实测数据（macOS arm64 / Go 1.26.4 / 合成事件 50 行 row）」就是
// 在这个夹具上量的：仅 Marshal 15,008 ns · 正则包+预过滤 118,543 · 全量正则 338,624 · 本方案 327,442）。
// 口径一致才谈得上「同机同口径对照」——换一个夹具，数字就没法比。
//
// 三个**只活在测试里**的成本模型（benchTier1Prefilter / benchNaiveAllRules）对应原型的
// BenchmarkEventTier1WithPrefilter / BenchmarkNaiveAllRules，用来在本仓自己的门禁里复现那张对照表，
// 而不是引用一个别人本子上的数字（生产路径没有这两个函数）。
//
// 跑法：go test ./internal/obs/redact/ -run '^$' -bench . -benchmem -count=5

// sampleEvent 是合成事件：50 行 row，每行 3 个字段（note/code/line）+ 4 个顶层字段。
// note 含 `/`（进 Tier-1 管线）· code 是 ≥24 字符的代码行（进 Tier-0 的 b64 候选扫描）·
// line 是 int（类型门控的负例：非 string 不进管线）。
func sampleEvent(n int) map[string]any {
	rows := make([]any, 0, n)
	for i := 0; i < n; i++ {
		rows = append(rows, map[string]any{
			"note": "read /tmp/f" + strconv.Itoa(i) + " ok",
			"code": "for i := 0; i < n; i++ { x += i }",
			"line": i,
		})
	}
	return map[string]any{"tool": "fs.read", "model": "qwen3", "temperature": 0.7, "rows": rows}
}

// benchKeepEvent 是**全 A 类**事件（字段三分表 §五 的「原样保留」集合）：类型门控的受益场景。
// 形状与 sampleEvent 同构（50 行 row），但每行 6 个 A 类字段 ⇒ 302 个安全标量。
// 每个值都是低基数枚举/数值 ⇒ 按分级口径一个字节都不该进值管线（零扫描）。
func benchKeepEvent(n int) map[string]any {
	rows := make([]any, 0, n)
	for i := 0; i < n; i++ {
		rows = append(rows, map[string]any{
			"model":            "qwen3-30b-a3b",
			"tool_name":        "fs.read",
			"stop_reason":      "tool_use",
			"call_fingerprint": "9f2c11ab" + strconv.Itoa(i),
			"seq":              i,
			"duration_ms":      128,
		})
	}
	return map[string]any{"model": "qwen3-30b-a3b", "seq": 41, "rows": rows}
}

// benchKeepThroughPipeline 是「类型门控**关掉**」时的成本模型：递归 walk 与 walkValue 完全同形，
// 唯独 A 类字符串也照样进值管线。两个基准之差 = 门控省下的钱（用例断言门控生效，
// 这里给出纳秒数 —— 耗时断言在 CI 上会抖动，故只作报告数字，不作断言）。
func benchKeepThroughPipeline(v any) any {
	switch t := v.(type) {
	case string:
		return RedactValue(t)
	case map[string]any:
		m := make(map[string]any, len(t))
		for k, vv := range t {
			m[k] = benchKeepThroughPipeline(vv)
		}
		return m
	case []any:
		s := make([]any, len(t))
		for i := range t {
			s[i] = benchKeepThroughPipeline(t[i])
		}
		return s
	}
	return v
}

// ── 成本模型（生产路径没有它们；只为复现设计附件 §三 的两条基线）─────────────

// benchTier1Prefilter == 原型的 tier1WithPrefilter：预过滤 + 正则包，无 Tier-0 编码通道。
func benchTier1Prefilter(v any) any {
	switch t := v.(type) {
	case string:
		if !maybeSensitive(t) {
			return t
		}
		return asciiLine(foldWidth(t))
	case map[string]any:
		m := make(map[string]any, len(t))
		for k, vv := range t {
			m[k] = benchTier1Prefilter(vv)
		}
		return m
	case []any:
		s := make([]any, len(t))
		for i := range t {
			s[i] = benchTier1Prefilter(t[i])
		}
		return s
	}
	return v
}

// benchAlwaysAllRules == 原型的 alwaysAllRules：每条值跑全量正则，没有预过滤。
func benchAlwaysAllRules(s string) string {
	s = reSessTok.ReplaceAllString(s, PlaceholderRedacted)
	s = maskEncodedPayloads(s)
	s = reURLQuery.ReplaceAllString(s, PlaceholderRedacted)
	s = reBearer.ReplaceAllString(s, PlaceholderRedacted)
	s = reHomeDir.ReplaceAllString(s, PlaceholderPath)
	s = reEmail.ReplaceAllString(s, PlaceholderRedacted)
	s = reIPv4.ReplaceAllString(s, PlaceholderRedacted)
	return s
}

// benchNaiveAllRules == 原型的 naiveRedact：类型分派有、预过滤没有。
func benchNaiveAllRules(v any) any {
	switch t := v.(type) {
	case string:
		return benchAlwaysAllRules(t)
	case map[string]any:
		m := make(map[string]any, len(t))
		for k, vv := range t {
			m[k] = benchNaiveAllRules(vv)
		}
		return m
	case []any:
		s := make([]any, len(t))
		for i := range t {
			s[i] = benchNaiveAllRules(t[i])
		}
		return s
	}
	return v
}

// ── 事件级基准（每次完整事件）───────────────────────────────────────────────

// BenchmarkMarshalOnly 是地板：不脱敏时写者本身的开销（设计附件 §三 第一行）。
func BenchmarkMarshalOnly(b *testing.B) {
	ev := sampleEvent(50)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := json.Marshal(ev); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkEventTier1Prefilter_Baseline：基线成本模型（设计附件 118,543 ns）。
func BenchmarkEventTier1Prefilter_Baseline(b *testing.B) {
	ev := sampleEvent(50)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = benchTier1Prefilter(ev)
	}
}

// BenchmarkEventNaiveAllRules_Baseline：基线成本模型（设计附件 338,624 ns）。
func BenchmarkEventNaiveAllRules_Baseline(b *testing.B) {
	ev := sampleEvent(50)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = benchNaiveAllRules(ev)
	}
}

// BenchmarkRedactEvent：本方案的类型优先 + Tier-0 + 预过滤（设计附件 327,442 ns 的口径）。
// T2.7 的验收就是拿它与上面三条同机比较。
func BenchmarkRedactEvent(b *testing.B) {
	ev := sampleEvent(50)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = RedactEvent(ev)
	}
}

// BenchmarkMarshalRedacted：**真实写路径**（脱敏 + 预算复核 + 编码）——调用方实际要付的成本，
// 也是限流/核数估算该用的数字（设计附件结论③「≈20 倍 Marshal，1000 事件/s ≈ 0.33 核」）。
func BenchmarkMarshalRedacted(b *testing.B) {
	ev := sampleEvent(50)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = MarshalRedacted(ev)
	}
}

// BenchmarkRedactEvent_KeepFieldsOnly：全 A 类事件 —— 类型门控的收益（零扫描）。
func BenchmarkRedactEvent_KeepFieldsOnly(b *testing.B) {
	ev := benchKeepEvent(50)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = RedactEvent(ev)
	}
}

// BenchmarkRedactEvent_KeepFieldsThroughPipeline：门控**关掉**时的同一事件成本模型
// （302 个 A 类标量照样进值管线）。两个数字之差 = 类型门控省下的钱。
func BenchmarkRedactEvent_KeepFieldsThroughPipeline(b *testing.B) {
	ev := benchKeepEvent(50)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = benchKeepThroughPipeline(ev)
	}
}

// ── 单值微基准（每个值的成本；写路径上真正的计价单位）───────────────────────

// benchValues 与原型同口径（设计附件 §三 的单值成本：短词 0.49µs · email 4.6µs ·
// 路径 6.1µs · 代码行 4.4µs · base64 3.1µs · 1.8KB 长文本 53.4µs）。
var benchValues = map[string]string{
	"short_word":    "tool_call",
	"path":          "~/projects/zerg/internal/controller/pipeline.go",
	"code_line":     "for i := 0; i < n; i++ { x += i }",
	"email":         "contact ops@zerg.local for help",
	"b64_blob":      "cfg=L1VzZXJzL2Z1enowMS8uY29uZmlnL3plcmcvdG9rZW4=",
	"b64_in_code":   "token := base64.StdEncoding.EncodeToString([]byte(path))",
	"long_text":     strings.Repeat("the quick brown fox jumps over the lazy dog ", 40), // 1.8KB
	"prefilter_off": "plainwordnohint",                                                  // 必要条件全不满足 ⇒ 预过滤直接跳过（最便宜的一档）
}

func BenchmarkValue(b *testing.B) {
	for name, v := range benchValues {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = RedactValue(v)
			}
		})
	}
}

// ── 大值（值长上限存在的理由：成本 ≈ 29 ns/byte，值无限长 ⇒ 成本无限）────────

func benchLargeValues() map[string]string {
	body := strings.Repeat("read /tmp/f0 ok; dur=12ms; retry=0; ", 1600) // ≈ 64KB 的自由文本
	return map[string]string{
		"large_64kb_loglike":   body, // 含分隔符 ⇒ 全量正则（贵的极端）
		"large_64kb_plaintext": strings.Repeat("the quick brown fox jumps over the lazy dog ", 1400),
		// 尾部藏一个真秘密的 base64 载荷（编码通道在大值上同样不能被预过滤跳过）
		"large_64kb_with_b64_secret": body + " blob=L1VzZXJzL2Z1enowMS8uY29uZmlnL3plcmcvdG9rZW4=",
	}
}

func BenchmarkValueLarge(b *testing.B) {
	for name, v := range benchLargeValues() {
		b.Run(name, func(b *testing.B) {
			b.SetBytes(int64(len(v)))
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = RedactValue(v)
			}
		})
	}
}
