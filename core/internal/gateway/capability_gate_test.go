package gateway

// capability_gate_test.go — 待修补 #11（高）在**网关路由硬门槛**一侧的反例测试。
//
// 规则：能力断言带引擎维度后，路由硬门槛按**目标引擎**判定——只有目标引擎被证过
// 支持该必需能力才放行；否则 unverifiable + fail-closed（返回明确原因），绝不静默降级。
//
// 反例优先（四类，每条都能先失败）：
//
//	① 快照在 llama.cpp 上 vision=true，目标引擎 vllm 无证据 → 不得放行（且不得退到未验证引擎）；
//	② 同一能力两引擎取值不同（A true / B false）→ 各按各的判；
//	③ 旧快照（无引擎维度）→ fail-closed + unverifiable（兼容代价）；
//	④ 无任何能力证据 → fail-closed。
//
// 写盘一律不碰 ~/.zerg：本文件用**注入的假来源**（capSource），连 t.TempDir 都不需要
// 落真实记录；另有一条用真实文件来源 + t.TempDir 的用例覆盖解析路径。

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/config"
	"github.com/Mr2109/zerg-swarm/core/internal/modelreg"
	"github.com/Mr2109/zerg-swarm/core/internal/store"
)

// fakeCapSource 是注入的假能力快照来源（不读磁盘，不写 ~/.zerg）。
type fakeCapSource struct {
	snaps map[string]*modelreg.CapabilitySnapshotArtifact
	errs  map[string]error
}

func (f *fakeCapSource) CapabilitySnapshot(model string) (*modelreg.CapabilitySnapshotArtifact, error) {
	if err, ok := f.errs[model]; ok {
		return nil, err
	}
	return f.snaps[model], nil
}

// gateTestGateway 造一个带假能力来源的网关：模型 model 有两个候选，
// local 的 backend=llama-server、x3 的 backend=vllm（两种引擎，正是打穿场景）。
func gateTestGateway(t *testing.T, src CapabilitySnapshotSource, candidates []config.ModelCandidate) *Gateway {
	t.Helper()
	fleet := map[string]config.FleetNode{"x3": {Host: "<worker-ip>", Port: 8100}}
	return &Gateway{
		config:     &config.FleetConfig{Models: map[string][]config.ModelCandidate{"visionmodel": candidates}, Fleet: fleet},
		roundRobin: map[string]int{},
		store:      store.NewStore(),
		capSource:  src,
	}
}

// twoEngineCandidates：local(llama-server) + x3(vllm)。
func twoEngineCandidates() []config.ModelCandidate {
	return []config.ModelCandidate{
		{Host: "local", Backend: "llama-server", File: "~/visionmodel.gguf"},
		{Host: "x3", Backend: "vllm", File: "/data/visionmodel.gguf"},
	}
}

// ① 目标引擎没被证过支持 → 不得放行到该引擎；有被证过的引擎则选它。
func TestRouteGate_RejectUnprovenTargetEngine(t *testing.T) {
	src := &fakeCapSource{snaps: map[string]*modelreg.CapabilitySnapshotArtifact{
		"visionmodel": {Schema: modelreg.CapabilitySnapshotSchemaV1, Capabilities: []modelreg.Capability{
			{Name: "vision", Value: true, Source: "probed", Evidence: modelreg.EvidenceVision, Engines: []string{"llama.cpp"}},
		}},
	}}
	g := gateTestGateway(t, src, twoEngineCandidates())

	// 选中的引擎必须是被证过 vision 的那一个（llama.cpp），而不是 vllm。
	route, err := g.pickRoute("visionmodel", "", "", "vision")
	if err != nil {
		t.Fatalf("有被证过的引擎（llama.cpp）时不应失败：%v", err)
	}
	if route.Host != "local" {
		t.Fatalf("应选被证过 vision 的 llama.cpp 候选（local），实际 %s——引擎维度被打穿", route.Host)
	}

	// 只有 vllm 候选时 → fail-closed，且错误写明哪个引擎缺哪条能力证据。
	gOnly := gateTestGateway(t, src, []config.ModelCandidate{
		{Host: "x3", Backend: "vllm", File: "/data/visionmodel.gguf"},
	})
	_, err = gOnly.pickRoute("visionmodel", "", "", "vision")
	if err == nil {
		t.Fatal("目标引擎 vllm 没有 vision 证据却放行了——静默降级（绝不能发生）")
	}
	var ge *CapabilityGateError
	if !errors.As(err, &ge) {
		t.Fatalf("应返回 *CapabilityGateError，实际 %T: %v", err, err)
	}
	if ge.Engine != "vllm" {
		t.Fatalf("错误应指明目标引擎 vllm，实际 %q", ge.Engine)
	}
	for _, want := range []string{"vision", "vllm", "llama.cpp"} {
		if !strings.Contains(ge.Error(), want) {
			t.Fatalf("错误文本应含 %q（写清哪个引擎缺哪条证据），实际：%s", want, ge.Error())
		}
	}
	// 明确映射到 400 capability_unavailable（不静默降级成 502/别的模型）。
	if code, status := classifyRouteError(err); code != "capability_unavailable" || status != http.StatusBadRequest {
		t.Fatalf("分类应为 capability_unavailable/400，实际 %s/%d", code, status)
	}
}

// ② 同一能力两引擎取值不同 → 各按各的判（选被证 true 的那个引擎）。
func TestRouteGate_PerEngineValues(t *testing.T) {
	src := &fakeCapSource{snaps: map[string]*modelreg.CapabilitySnapshotArtifact{
		"visionmodel": {Schema: modelreg.CapabilitySnapshotSchemaV1, Capabilities: []modelreg.Capability{
			{Name: "vision", Value: false, Source: "probed", Evidence: modelreg.EvidenceVision + " (no_vision: http 400)", Engines: []string{"llama.cpp"}},
			{Name: "vision", Value: true, Source: "probed", Evidence: modelreg.EvidenceVision, Engines: []string{"vllm"}},
		}},
	}}
	g := gateTestGateway(t, src, twoEngineCandidates())

	route, err := g.pickRoute("visionmodel", "", "", "vision")
	if err != nil {
		t.Fatalf("vllm 上 vision=true 应可路由：%v", err)
	}
	if route.Host != "x3" {
		t.Fatalf("应选被证 vision=true 的 vllm 候选（x3），实际 %s", route.Host)
	}
	// 反向：被证 false 的引擎不得被选中。
	if route.Host == "local" {
		t.Fatal("llama.cpp 上 vision=false（确定不支持），却被选中——按引擎取值失效")
	}
}

// ③ 旧快照（无引擎维度）→ fail-closed + unverifiable（兼容代价）。
func TestRouteGate_LegacySnapshotFailsClosed(t *testing.T) {
	src := &fakeCapSource{snaps: map[string]*modelreg.CapabilitySnapshotArtifact{
		"visionmodel": {Schema: modelreg.CapabilitySnapshotSchemaV1, Capabilities: []modelreg.Capability{
			{Name: "vision", Value: true, Source: "probed", Evidence: modelreg.EvidenceVision}, // 无 engines
		}},
	}}
	g := gateTestGateway(t, src, twoEngineCandidates())

	_, err := g.pickRoute("visionmodel", "", "", "vision")
	if err == nil {
		t.Fatal("旧快照（无引擎维度）不得放行——缺 = 未知，绝不当作全局可用")
	}
	var ge *CapabilityGateError
	if !errors.As(err, &ge) {
		t.Fatalf("应返回 *CapabilityGateError，实际 %T: %v", err, err)
	}
	if ge.Reason != modelreg.CapReasonEngineDimensionMissing {
		t.Fatalf("原因码应为 %s（旧快照兼容代价），实际 %s", modelreg.CapReasonEngineDimensionMissing, ge.Reason)
	}
	if !strings.Contains(ge.Trace, "compat cost") {
		t.Fatalf("留痕应写明兼容代价，实际：%s", ge.Trace)
	}
}

// ④ 无任何能力证据（连快照都没有）→ fail-closed。
func TestRouteGate_NoSnapshotFailsClosed(t *testing.T) {
	src := &fakeCapSource{snaps: map[string]*modelreg.CapabilitySnapshotArtifact{}} // 没有 visionmodel 的快照
	g := gateTestGateway(t, src, twoEngineCandidates())

	_, err := g.pickRoute("visionmodel", "", "", "vision")
	if err == nil {
		t.Fatal("无任何能力证据不得放行（fail-closed）")
	}
	var ge *CapabilityGateError
	if !errors.As(err, &ge) {
		t.Fatalf("应返回 *CapabilityGateError，实际 %T: %v", err, err)
	}
	if ge.Reason != modelreg.CapReasonNoSnapshot {
		t.Fatalf("原因码应为 %s，实际 %s", modelreg.CapReasonNoSnapshot, ge.Reason)
	}
	if code, status := classifyRouteError(err); code != "capability_unavailable" || status != http.StatusBadRequest {
		t.Fatalf("分类应为 capability_unavailable/400，实际 %s/%d", code, status)
	}
}

// 读快照报错（权限/坏文件）→ 同样 fail-closed（不因"读不到"而放行）。
func TestRouteGate_SnapshotReadErrorFailsClosed(t *testing.T) {
	src := &fakeCapSource{
		snaps: map[string]*modelreg.CapabilitySnapshotArtifact{},
		errs:  map[string]error{"visionmodel": errors.New("boom")},
	}
	g := gateTestGateway(t, src, twoEngineCandidates())
	_, err := g.pickRoute("visionmodel", "", "", "vision")
	var ge *CapabilityGateError
	if err == nil || !errors.As(err, &ge) || ge.Reason != modelreg.CapReasonNoSnapshot {
		t.Fatalf("快照读失败必须 fail-closed 且原因 %s，实际 %v", modelreg.CapReasonNoSnapshot, err)
	}
}

// 无必需能力时门槛是零改动路径（向后兼容：既有路由行为一字不变）。
func TestRouteGate_NoRequiredCapabilityIsNoop(t *testing.T) {
	// 故意给一份"什么都没有"的来源——没有必需能力时它不该被调用/影响路由。
	src := &fakeCapSource{snaps: map[string]*modelreg.CapabilitySnapshotArtifact{}}
	g := gateTestGateway(t, src, twoEngineCandidates())
	route, err := g.pickRoute("visionmodel", "", "") // 不带 required
	if err != nil {
		t.Fatalf("无必需能力时不应因门槛失败：%v", err)
	}
	if route.Host == "" {
		t.Fatal("无必需能力时应照常路由")
	}
}

// RequiredCapabilitiesFromRequest：只认结构化图像部件，不做模糊匹配。
func TestRequiredCapabilitiesFromRequest(t *testing.T) {
	cases := []struct {
		name string
		body string
		want []string
	}{
		{"openai_image_url_obj", `{"model":"m","messages":[{"role":"user","content":[{"type":"text","text":"看图"},{"type":"image_url","image_url":{"url":"data:image/png;base64,AAAA"}}]}]}`, []string{"vision"}},
		{"openai_image_url_str", `{"messages":[{"content":[{"image_url":"https://x/a.png"}]}]}`, []string{"vision"}},
		{"responses_input_image", `{"input":[{"type":"input_image","image_url":"data:image/png;base64,AAAA"}]}`, []string{"vision"}},
		{"bare_type_image", `{"messages":[{"content":[{"type":"image","source":{"data":"AAAA"}}]}]}`, []string{"vision"}},
		{"text_only", `{"model":"m","messages":[{"role":"user","content":"你好"}]}`, nil},
		{"word_image_but_no_part", `{"messages":[{"role":"user","content":"请描述 image 这个词"}]}`, nil},
		{"empty", ``, nil},
		{"not_json", `not json at all`, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := RequiredCapabilitiesFromRequest([]byte(c.body))
			if len(got) != len(c.want) {
				t.Fatalf("got=%v want=%v", got, c.want)
			}
			for i := range c.want {
				if got[i] != c.want[i] {
					t.Fatalf("got=%v want=%v", got, c.want)
				}
			}
		})
	}
}

// 真实文件来源（t.TempDir）：模型目录里没有该 id → 无快照 → fail-closed；
// 有记录 + 快照（带引擎维度）→ 按目标引擎放行。全程不碰 ~/.zerg。
func TestFileCapabilitySource_TempDir(t *testing.T) {
	root := t.TempDir()
	rec := &modelreg.Record{
		Schema:     modelreg.SchemaV1,
		ID:         "visionmodel",
		Digest:     "sha256:" + strings.Repeat("a", 64),
		Files:      []modelreg.File{{Role: "weights", Name: "w.gguf", SHA256: strings.Repeat("b", 64), Size: 1}},
		License:    modelreg.License{SPDX: "Apache-2.0", Commercial: "yes"},
		State:      "known",
		Modalities: map[string][]string{"in": {"text"}, "out": {"text"}},
	}
	st := modelreg.NewStore(root)
	if _, err := st.Put(rec, false); err != nil {
		t.Fatalf("落真实记录失败：%v", err)
	}
	recPath := st.PathFor(rec)
	rep := &modelreg.ProbeReport{
		Engine: "llama.cpp",
		Capabilities: []modelreg.Capability{
			{Name: "vision", Value: true, Source: "probed", Evidence: modelreg.EvidenceVision},
		},
	}
	if _, err := modelreg.WriteCapabilitySnapshot(recPath, rec, rep); err != nil {
		t.Fatalf("写能力快照失败：%v", err)
	}

	g := gateTestGateway(t, newFileCapabilitySource(root), twoEngineCandidates())

	// 目标引擎 vllm 无证据 → 不放行（但 local/llama.cpp 被证过 → 选中它）。
	route, err := g.pickRoute("visionmodel", "", "", "vision")
	if err != nil {
		t.Fatalf("llama.cpp 被证过 vision，应可选：%v", err)
	}
	if route.Host != "local" {
		t.Fatalf("应选 llama.cpp 候选，实际 %s", route.Host)
	}

	// 目录里没有该能力证据（此快照只有 vision，没有 tools）→ fail-closed。
	_, err = g.pickRoute("visionmodel", "", "", "tools")
	if err == nil {
		t.Fatal("快照里没有 tools 证据应 fail-closed（没有证据不等于可用）")
	}
	var ge *CapabilityGateError
	if !errors.As(err, &ge) || ge.Reason != modelreg.CapReasonNoEvidence {
		t.Fatalf("应 fail-closed 且原因 %s，实际 %v", modelreg.CapReasonNoEvidence, err)
	}
}
