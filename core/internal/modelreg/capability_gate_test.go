package modelreg

// capability_gate_test.go — 待修补 #11（高）：能力断言的**引擎维度** + 按目标引擎判判定。
//
// 反例优先：每条用例都要能先失败（把"按目标引擎判"改回"全局判"→ 用例必须转红）。
// 四类：
//
//	① 快照在 llama.cpp 上 vision=true，目标引擎 vllm 无证据 → 不得放行，且给原因；
//	② 同一能力在两引擎取值不同（A true / B false）→ 各按各的判；
//	③ 旧快照（无引擎维度）→ fail-closed + unverifiable（兼容代价）；
//	④ 无任何能力证据 → fail-closed。
//
// 另含"不动记录正文"的守卫：引擎维度只进能力快照，绝不进记录正文（身份不破）。

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// snapWith 造一份最小能力快照（只填本文件用到的字段）。
func snapWith(caps []Capability, uv []Unverifiable) *CapabilitySnapshotArtifact {
	return &CapabilitySnapshotArtifact{
		Schema:       CapabilitySnapshotSchemaV1,
		Capabilities: caps,
		Unverifiable: uv,
	}
}

// ── 规范化：两个命名面（probe --engine / fleet backend）必须收敛到同一词 ──

func TestCanonicalEngine(t *testing.T) {
	cases := map[string]string{
		"llama.cpp":    "llama.cpp",
		"llama-server": "llama.cpp", // fleet.yaml 的 backend 写法
		"LLAMA-SERVER": "llama.cpp", // 大小写不敏感
		"vllm":         "vllm",
		"ds4-server":   "ds4",
		" ollama ":     "ollama",
		"":             "",
		// 未识别的引擎**原样返回**：不做映射猜测（猜错会把"没证据"变成"误判有证据"）。
		"weird-engine": "weird-engine",
	}
	for in, want := range cases {
		if got := CanonicalEngine(in); got != want {
			t.Errorf("CanonicalEngine(%q)=%q，want %q", in, got, want)
		}
	}
}

// ── 反例①：能力只在 llama.cpp 上被证过，目标引擎 vllm 不得放行 ──────────────

func TestCapabilityGate_RejectUnprovenTargetEngine(t *testing.T) {
	snap := snapWith([]Capability{
		{Name: "vision", Value: true, Source: "probed", Evidence: EvidenceVision, Engines: []string{"llama.cpp"}},
	}, nil)

	// 正例面：在 llama.cpp 上被证过 → 放行（否则本用例就只是"永远拒绝"的空跑）。
	if d := EvaluateCapabilityForEngine(snap, "llama-server", "vision"); !d.Allowed {
		t.Fatalf("目标引擎 llama-server（规范化=llama.cpp）应被证过 vision → 放行，实际：%+v", d)
	}

	// 反例面：目标是 vllm（快照里没有它的证据）→ 必须 fail-closed + 写明原因。
	d := EvaluateCapabilityForEngine(snap, "vllm", "vision")
	if d.Allowed {
		t.Fatalf("目标引擎 vllm 没有 vision 证据，绝不能放行（引擎维度被打穿）：%+v", d)
	}
	if d.Reason != CapReasonOtherEngineOnly {
		t.Fatalf("原因码应为 %s（只在别的引擎上被证过），实际 %s", CapReasonOtherEngineOnly, d.Reason)
	}
	if d.Engine != "vllm" {
		t.Fatalf("判定结果必须带目标引擎，实际 Engine=%q", d.Engine)
	}
	// 留痕必须写清"哪个引擎缺哪条能力证据"。
	for _, want := range []string{"vision", "vllm", "llama.cpp"} {
		if !strings.Contains(d.Trace, want) {
			t.Fatalf("留痕应包含 %q，实际：%s", want, d.Trace)
		}
	}
}

// ── 反例②：同一能力在两引擎取值不同 → 各按各的判 ──────────────────────────

func TestCapabilityGate_PerEngineValues(t *testing.T) {
	// 同一个 name 两条断言：llama.cpp=true（挂了 mmproj）、vllm=false（配方未挂 mmproj）。
	snap := snapWith([]Capability{
		{Name: "vision", Value: false, Source: "probed", Evidence: EvidenceVision + " (no_vision: http 400)", Engines: []string{"vllm"}},
		{Name: "vision", Value: true, Source: "probed", Evidence: EvidenceVision, Engines: []string{"llama.cpp"}},
	}, nil)

	dLlama := EvaluateCapabilityForEngine(snap, "llama-server", "vision")
	if !dLlama.Allowed || dLlama.Reason != CapReasonAllowed {
		t.Fatalf("llama.cpp 上 vision=true 应放行，实际：%+v", dLlama)
	}
	dVllm := EvaluateCapabilityForEngine(snap, "vllm", "vision")
	if dVllm.Allowed {
		t.Fatalf("vllm 上 vision=false（确定不支持）绝不放行，实际：%+v", dVllm)
	}
	if dVllm.Reason != CapReasonUnsupported {
		t.Fatalf("vllm 是**确定不支持**（不是不能判定），原因码应为 %s，实际 %s", CapReasonUnsupported, dVllm.Reason)
	}
	// 取值不同必须体现在判定上（若实现"取第一条"就会把两引擎判成同一个结果 → 本断言转红）。
	if dLlama.Allowed == dVllm.Allowed {
		t.Fatal("两引擎取值不同，判定却相同——说明没有按目标引擎取那一条")
	}
}

// ── 反例③：旧快照（无引擎维度）→ fail-closed + 明确降级路径（兼容代价）──────

func TestCapabilityGate_LegacySnapshotNoEngineDimension(t *testing.T) {
	// 旧快照：probe 未给 --engine → capabilities 存在但 engines 缺失。
	snap := snapWith([]Capability{
		{Name: "vision", Value: true, Source: "probed", Evidence: EvidenceVision},
	}, nil)
	if CapabilitySnapshotHasEngineDimension(snap) {
		t.Fatal("本用例的快照本应没有引擎维度（造错了）")
	}

	for _, engine := range []string{"llama-server", "vllm"} {
		d := EvaluateCapabilityForEngine(snap, engine, "vision")
		if d.Allowed {
			t.Fatalf("旧快照（无引擎维度）对 %s 绝不能放行（缺 = 未知，不等于可用）：%+v", engine, d)
		}
		if d.Reason != CapReasonEngineDimensionMissing {
			t.Fatalf("旧快照的原因码应为 %s，实际 %s", CapReasonEngineDimensionMissing, d.Reason)
		}
		// 降级路径必须显式标出兼容代价，供人判断"要不要重探补引擎维度"。
		if !strings.Contains(d.Trace, "legacy") || !strings.Contains(d.Trace, "compat cost") {
			t.Fatalf("旧快照的留痕应写明 legacy + 兼容代价，实际：%s", d.Trace)
		}
	}
}

// ── 反例④：无任何能力证据 → fail-closed ────────────────────────────────────

func TestCapabilityGate_NoEvidenceFailsClosed(t *testing.T) {
	// (a) 快照存在、但根本没有这条能力（也没有 unverifiable）→ 缺 = 未知。
	snap := snapWith([]Capability{
		{Name: "text", Value: true, Source: "probed", Evidence: EvidenceText, Engines: []string{"llama.cpp"}},
	}, nil)
	if d := EvaluateCapabilityForEngine(snap, "llama.cpp", "vision"); d.Allowed || d.Reason != CapReasonNoEvidence {
		t.Fatalf("无该能力证据必须 fail-closed 且原因 %s，实际：%+v", CapReasonNoEvidence, d)
	}

	// (b) 快照整个读不到（nil）→ 不能判定。
	if d := EvaluateCapabilityForEngine(nil, "llama.cpp", "vision"); d.Allowed || d.Reason != CapReasonNoSnapshot {
		t.Fatalf("快照缺失必须 fail-closed 且原因 %s，实际：%+v", CapReasonNoSnapshot, d)
	}

	// (c) 目标引擎未知（空）→ 不能判定（绝不退化成"全局放行"）。
	fullyCapable := snapWith([]Capability{
		{Name: "vision", Value: true, Source: "probed", Evidence: EvidenceVision, Engines: []string{"llama.cpp"}},
	}, nil)
	if d := EvaluateCapabilityForEngine(fullyCapable, "", "vision"); d.Allowed || d.Reason != CapReasonEngineUnknown {
		t.Fatalf("目标引擎未知必须 fail-closed 且原因 %s，实际：%+v", CapReasonEngineUnknown, d)
	}
}

// ── 反例（承 #27）：不能判定 ≠ 不支持，且都不放行 ─────────────────────────

func TestCapabilityGate_UnverifiableIsNotUnsupported(t *testing.T) {
	snap := snapWith(nil, []Unverifiable{
		{Name: "vision", Reason: FailBudgetExhausted, Evidence: EvidenceVision + " budget=2048"},
	})
	d := EvaluateCapabilityForEngine(snap, "llama.cpp", "vision")
	if d.Allowed {
		t.Fatalf("unverifiable 的能力绝不能放行：%+v", d)
	}
	if d.Reason != CapReasonUnverifiable {
		t.Fatalf("原因码应为 %s（不能判定），实际 %s——不得与「确定不支持」混为一谈", CapReasonUnverifiable, d.Reason)
	}
}

// EnginesOf：只读地取某能力声明的引擎（规范化、去重）。
func TestEnginesOf(t *testing.T) {
	snap := snapWith([]Capability{
		{Name: "vision", Value: true, Engines: []string{"llama-server", "llama.cpp", "vllm"}},
	}, nil)
	got := EnginesOf(snap, "vision")
	want := []string{"llama.cpp", "vllm"} // 规范化 + 去重，保持出现顺序
	if len(got) != len(want) {
		t.Fatalf("EnginesOf 长度不对：got=%v want=%v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("EnginesOf[%d]=%q，want %q（全量 %v）", i, got[i], want[i], got)
		}
	}
	if EnginesOf(snap, "tools") != nil {
		t.Fatal("没有的能力条应返回 nil（缺 = 未知）")
	}
}

// ── 引擎维度只进快照、绝不进记录正文（身份不破）──────────────────────────────

func TestProbeEngineDimensionEntersSnapshotNotRecordBody(t *testing.T) {
	srv := fakeEngine(t, "image input is not supported by this model")
	gguf := writeTestGGUF(t, t.TempDir(), "eng-dims.gguf")

	// 给引擎 → 能力断言应带 engines[]（且规范化 fleet 写法 llama-server → llama.cpp）。
	rec, rep, err := Probe(ProbeOptions{
		Target:   gguf,
		Endpoint: srv.URL,
		Engine:   "llama-server",
		Timeout:  5 * time.Second,
	})
	if err != nil {
		t.Fatalf("probe 失败：%v", err)
	}
	snap := NewCapabilitySnapshot(rec, rep)
	if len(snap.Capabilities) == 0 {
		t.Fatal("快照应至少有一条能力断言（本用例造错了）")
	}
	for _, c := range snap.Capabilities {
		if len(c.Engines) != 1 || c.Engines[0] != "llama.cpp" {
			t.Fatalf("能力断言 %q 应带 engines=[llama.cpp]，实际 %v", c.Name, c.Engines)
		}
	}
	// 记录**正文**（内容寻址的锚）绝不能出现 engines —— 引擎属"现状"，进快照即可。
	if strings.Contains(string(recordBody(t, rec)), "engines") {
		t.Fatal("记录正文出现了 engines——引擎维度只允许进能力快照，不得破身份")
	}
	// 快照序列化后确实承载 engines（否则上面读的是内存里的对象，落盘可能丢）。
	sb, _ := json.Marshal(snap)
	if !strings.Contains(string(sb), `"engines":["llama.cpp"]`) {
		t.Fatalf("快照 JSON 未承载 engines：%s", string(sb))
	}

	// 不给引擎 → 不附 engines（缺 = 未知，绝不默认成某个引擎）。
	rec2, rep2, err := Probe(ProbeOptions{Target: gguf, Endpoint: srv.URL, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("probe（不给引擎）失败：%v", err)
	}
	snap2 := NewCapabilitySnapshot(rec2, rep2)
	for _, c := range snap2.Capabilities {
		if len(c.Engines) != 0 {
			t.Fatalf("未给 --engine 的能力断言不得带 engines（缺=未知），实际 %q=%v", c.Name, c.Engines)
		}
	}
	if CapabilitySnapshotHasEngineDimension(&snap2) {
		t.Fatal("未给 --engine 的快照应被视为「无引擎维度」（旧形态）")
	}
}
