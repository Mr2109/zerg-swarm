package gateway

// capability_gate_boundaries_test.go — 待修补 #38（中）两条未接线边界的反例测试。
//
// 背景（#11 已实现「能力断言带引擎维度 + 路由硬门槛按目标引擎判、fail-closed」，但两条边界未接线）：
//
//	① 复合模型分支（zerg-baiyan）：在算必需能力之前就 return → 带图请求整条绕过门槛；
//	② pickFallbackRoute 换机：未透传 required → 首跳过门槛，换机目标引擎未验证就放行。
//
// 本文件的反例（每条都能先失败）：
//
//	①a 带图请求走复合模型分支 → 必须 fail-closed 400 capability_unavailable（不得绕过门槛/静默丢图）；
//	①b 纯文本请求走复合模型分支 → 行为不得因此改变（回归，不得误拦）；
//	②a 首跳引擎被证过 vision、换机目标未验证 → 换机后**必须**被拦（*CapabilityGateError/400）；
//	②b 无必需能力（required=nil）→ 换机照常（不得误拦，回归）；
//	②c 目标引擎也被证过 → 换机放行（门槛只拦未验证，不得过拦）；
//	②d forwardToBackend 内的换机 failover 端到端：required 确实穿透到 pickFallbackRoute，
//	    且门槛原因码原样上抛（不被 transport 错误盖成 502）。
//
// 全程不碰 ~/.zerg：编排器工作区用 ZERG_WORKSPACE 指向 t.TempDir；
// 能力快照用注入的假来源（fakeCapSource），不落真实记录。

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/config"
	"github.com/Mr2109/zerg-swarm/core/internal/contract"
	"github.com/Mr2109/zerg-swarm/core/internal/gateway/orchestrator"
	"github.com/Mr2109/zerg-swarm/core/internal/modelreg"
	"github.com/Mr2109/zerg-swarm/core/internal/store"
)

// recordingExecutor 是编排器 ModelExecutor 的假实现：只计数 + 回固定文本。
type recordingExecutor struct {
	mu    sync.Mutex
	calls int
	reply string
}

func (r *recordingExecutor) ExecuteWithResponse(ctx context.Context, model, prompt string, maxTokens int) (*orchestrator.ModelResponse, error) {
	r.mu.Lock()
	r.calls++
	r.mu.Unlock()
	return &orchestrator.ModelResponse{Content: r.reply, Tokens: 3}, nil
}

func (r *recordingExecutor) Execute(ctx context.Context, model, prompt string, maxTokens int) (string, error) {
	resp, err := r.ExecuteWithResponse(ctx, model, prompt, maxTokens)
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

func (r *recordingExecutor) callCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls
}

// compositeTestGateway 造一个挂编排器的网关。ZERG_WORKSPACE 指向临时目录——
// 编排器会把 harness 状态写到 <workspace>/.zerg/states，绝不落仓库或 ~/.zerg。
func compositeTestGateway(t *testing.T, exec orchestrator.ModelExecutor) *Gateway {
	t.Helper()
	t.Setenv("ZERG_WORKSPACE", t.TempDir())
	return &Gateway{
		config:       &config.FleetConfig{},
		roundRobin:   map[string]int{},
		store:        store.NewStore(),
		orchestrator: orchestrator.NewOrchestrator(orchestrator.DefaultOrchestratorConfig(), exec),
	}
}

func doHandle(g *Gateway, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	rec := httptest.NewRecorder()
	g.handleRequest(rec, req)
	return rec
}

// ①a 带图请求走复合模型分支：不得绕过门槛（必须明确报 capability_unavailable/400，且不调编排器）。
func TestCompositeBranch_ImageRequestBlockedByCapabilityGate(t *testing.T) {
	exec := &recordingExecutor{reply: "不该被执行"}
	g := compositeTestGateway(t, exec)

	// 结构化 image_url 部件——正是 RequiredCapabilitiesFromRequest 认的「必要性升档」信号
	body := `{"model":"zerg-baiyan","messages":[{"role":"user","content":[{"type":"text","text":"描述这张图"},{"type":"image_url","image_url":{"url":"data:image/png;base64,AAAA"}}]}]}`
	rec := doHandle(g, body)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("带图请求走复合模型分支必须 fail-closed 400（不得绕过门槛），实际 %d，body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "capability_unavailable") {
		t.Fatalf("错误码应为 capability_unavailable（与 #11 同一套），实际 body=%s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "vision") {
		t.Fatalf("报因应写明缺哪条能力（vision），实际 body=%s", rec.Body.String())
	}
	if n := exec.callCount(); n != 0 {
		t.Fatalf("门槛拦下后不得再调编排器（否则仍是静默丢图降级）——实际调用 %d 次", n)
	}
}

// ①b 纯文本请求走复合模型分支：行为不得因本次改动改变（回归，不得误拦）。
func TestCompositeBranch_TextRequestUnaffected(t *testing.T) {
	exec := &recordingExecutor{reply: "综合回答"}
	g := compositeTestGateway(t, exec)

	rec := doHandle(g, `{"model":"zerg-baiyan","messages":[{"role":"user","content":"只问文字，没有图"}]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("纯文本请求行为不得改变（应 200），实际 %d body=%s", rec.Code, rec.Body.String())
	}
	if exec.callCount() == 0 {
		t.Fatal("纯文本请求应照常走编排器——实际未调用（改动的门槛误拦了正常路径）")
	}
	if !strings.Contains(rec.Body.String(), "综合回答") {
		t.Fatalf("应返回编排结果，实际 body=%s", rec.Body.String())
	}
}

// failoverGateGateway：模型 visionmodel 两候选——node-b=llama-server（vision 已证）/
// node-a=vllm（vision 未证）。正是「首跳引擎被证过、换机目标未验证」的场景。
func failoverGateGateway(t *testing.T) *Gateway {
	t.Helper()
	src := &fakeCapSource{snaps: map[string]*modelreg.CapabilitySnapshotArtifact{
		"visionmodel": {Schema: modelreg.CapabilitySnapshotSchemaV1, Capabilities: []modelreg.Capability{
			{Name: "vision", Value: true, Source: "probed", Evidence: modelreg.EvidenceVision, Engines: []string{"llama.cpp"}},
		}},
	}}
	cfg := &config.FleetConfig{
		Models: map[string][]config.ModelCandidate{
			"visionmodel": {
				{Host: "node-b", Backend: "llama-server", File: "/node-b/visionmodel.gguf"},
				{Host: "node-a", Backend: "vllm", File: "/data/visionmodel.gguf"},
			},
		},
		Fleet: map[string]config.FleetNode{
			"node-b": {Host: "<cluster-ip>", Port: 8100},
			"node-a": {Host: "<cluster-ip>", Port: 8100},
		},
	}
	st := store.NewStore()
	injectSnap(st, "node-b", "visionmodel", 0, true, 0.1)
	injectSnap(st, "node-a", "visionmodel", 0, true, 0.1)
	return &Gateway{
		config:     cfg,
		roundRobin: map[string]int{},
		store:      st,
		capSource:  src,
		failCounts: map[string]int{},
		failSince:  map[string]time.Time{},
		tripCounts: map[string]int{},
	}
}

// ②a 首跳引擎（node-b/llama.cpp）被证过 vision，换机目标只剩 node-a（vllm，未验证）
// → 换机后**必须**被拦（fail-closed，*CapabilityGateError，分类 400 capability_unavailable）。
func TestPickFallbackRoute_ReGatesCapabilityAfterSwitch(t *testing.T) {
	g := failoverGateGateway(t)
	failed := &RouteResult{Host: "node-b"} // 首跳 = 被证过的引擎

	_, err := g.pickFallbackRoute(failed, "visionmodel", "backend node-b forward failed: i/o timeout", []string{"vision"})
	if err == nil {
		t.Fatal("换机目标引擎（vllm）未被证过 vision 却放行——门槛被换机绕过（静默降级）")
	}
	var ge *CapabilityGateError
	if !errors.As(err, &ge) {
		t.Fatalf("应返回 *CapabilityGateError，实际 %T: %v", err, err)
	}
	if code, status := classifyRouteError(err); code != "capability_unavailable" || status != http.StatusBadRequest {
		t.Fatalf("分类应为 capability_unavailable/400，实际 %s/%d", code, status)
	}
}

// ②b 无必需能力（required=nil）→ 换机照常（不得误拦，回归）。
func TestPickFallbackRoute_WithoutRequiredStillSwitches(t *testing.T) {
	g := failoverGateGateway(t)
	route, err := g.pickFallbackRoute(&RouteResult{Host: "node-b"}, "visionmodel", "backend node-b forward failed: i/o timeout", nil)
	if err != nil {
		t.Fatalf("无必需能力时换机不应被误拦：%v", err)
	}
	if route.Host != "node-a" {
		t.Fatalf("应换到 node-a，实际 %s", route.Host)
	}
}

// ②c 目标引擎也被证过 vision → 换机放行（门槛只拦未验证，不得过拦）。
func TestPickFallbackRoute_RequiredProvenOnTargetStillSwitches(t *testing.T) {
	g := failoverGateGateway(t)
	g.capSource = &fakeCapSource{snaps: map[string]*modelreg.CapabilitySnapshotArtifact{
		"visionmodel": {Schema: modelreg.CapabilitySnapshotSchemaV1, Capabilities: []modelreg.Capability{
			{Name: "vision", Value: true, Source: "probed", Evidence: modelreg.EvidenceVision, Engines: []string{"llama.cpp", "vllm"}},
		}},
	}}
	route, err := g.pickFallbackRoute(&RouteResult{Host: "node-b"}, "visionmodel", "backend node-b forward failed: i/o timeout", []string{"vision"})
	if err != nil {
		t.Fatalf("目标引擎也被证过 vision，换机不应被拦：%v", err)
	}
	if route.Host != "node-a" {
		t.Fatalf("应换到 node-a，实际 %s", route.Host)
	}
}

// ②d 端到端：forwardToBackend 内的换机 failover 必须真正拿到 required（而非只有直调 pickFallbackRoute 才生效），
// 且门槛错误原样上抛（不被 transport 错误盖成 502）。
func TestForwardToBackend_FailoverReGatesWithRequired(t *testing.T) {
	g := failoverGateGateway(t)
	g.client = &http.Client{Timeout: 5 * time.Second}

	// 首跳 URL 指向必然失败的本机端口（连接被拒）→ 触发内部换机 failover
	route := &RouteResult{Host: "node-b", Port: 1, URL: "http://127.0.0.1:1/infer"}
	body := []byte(`{"model":"visionmodel","messages":[{"role":"user","content":"看图"}]}`)

	_, err := g.forwardToBackend(context.Background(), route, "/v1/chat/completions", body, http.Header{}, []string{"vision"})
	if err == nil {
		t.Fatal("换机目标引擎未证过 vision——forwardToBackend 内的 failover 必须被门槛拦下")
	}
	var ge *CapabilityGateError
	if !errors.As(err, &ge) {
		t.Fatalf("门槛原因应原样上抛（*CapabilityGateError），实际 %T: %v", err, err)
	}
	if code, status := classifyRouteError(err); code != "capability_unavailable" || status != http.StatusBadRequest {
		t.Fatalf("分类应为 capability_unavailable/400，实际 %s/%d", code, status)
	}
}

// ③（`GAP-20261014-62` 两态 · 波3 片10 · 端到端）带图请求走正门 `handleRequest`：
// 有**实测产物驱动**的合法快照（名册侧真记录 ＋ 探测产出的 `<记录>.capabilities.json`，
// vision=true · engines=[llama.cpp]）⇒ 正门**收**，且请求**真发到**引擎端点；
// 缺快照 / 目标引擎未证过 ⇒ 仍 400 `capability_unavailable`（⇐ 同一枚用例的负控，见文件末）。
//
// 关键在**目标名的两套命名**：路由表 id（`example-35b-v2`）⟷ 快照 id（`example-35b-v2-1-5-35b-q4-k-m`），
// 桥 = 在册真源 `contract.ModelIDMap()`（本用例的两侧取值就取自该真源，✗ 不写死、✗ 不造恒真快照）。
func TestImageRequestThroughGateway_FleetIDViaTruthSource(t *testing.T) {
	m, err := contract.ModelIDMap()
	if err != nil || len(m.Entries) == 0 {
		t.Fatalf("id 规范化真源不可用（%v）—— 本用例的前提", err)
	}
	e := m.Entries[0]
	registryID, fleetID := e.RegistryID, e.FleetIDs[0]
	if registryID == fleetID {
		t.Fatalf("真源首条的 registry_id 与 fleet_id 同名（%q）—— 本用例要的是两套命名不同的那一对", registryID)
	}

	// ── 名册侧：真记录（id = 快照 id）＋ 一次探测产出的能力快照兄弟文件 ──
	root := t.TempDir()
	rec := &modelreg.Record{
		Schema:     modelreg.SchemaV1,
		ID:         registryID,
		Digest:     "sha256:" + strings.Repeat("a", 64),
		Files:      []modelreg.File{{Role: "weights", Name: e.Weights, SHA256: strings.Repeat("b", 64), Size: 1}},
		License:    modelreg.License{SPDX: "Apache-2.0", Commercial: "yes"},
		State:      "known",
		Modalities: map[string][]string{"in": {"text", "image"}, "out": {"text"}},
	}
	st := modelreg.NewStore(root)
	if _, err := st.Put(rec, false); err != nil {
		t.Fatalf("落真实记录失败：%v", err)
	}
	if _, err := modelreg.WriteCapabilitySnapshot(st.PathFor(rec), rec, &modelreg.ProbeReport{
		Engine: "llama.cpp",
		Capabilities: []modelreg.Capability{
			{Name: "vision", Value: true, Source: "probed", Evidence: modelreg.EvidenceVision, Engines: []string{"llama.cpp"}},
		},
	}); err != nil {
		t.Fatalf("写能力快照失败：%v", err)
	}

	// ── 引擎侧：假上游端点（收到请求 = 这一次请求**真发出去**了）──
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chatcmpl_img","object":"chat.completion","created":1,"model":"vision",` +
			`"choices":[{"index":0,"message":{"role":"assistant","content":"我看到了图"},"finish_reason":"stop"}],` +
			`"usage":{"total_tokens":3}}`))
	}))
	defer srv.Close()
	host, port := hostPort(t, srv.URL)

	g := &Gateway{
		config: &config.FleetConfig{
			Models: map[string][]config.ModelCandidate{
				fleetID: {{Host: "node-a", Backend: "llama-server", File: "/m/" + e.Weights}},
			},
			Fleet: map[string]config.FleetNode{"node-a": {Host: host, Port: port}},
		},
		roundRobin:     map[string]int{},
		store:          store.NewStore(),
		modelsRootPath: root,
		client:         &http.Client{Timeout: 5 * time.Second},
	}

	// ── 正门：带图请求（结构化 image_url 部件 ⇒ 必需能力 vision）──
	body := `{"model":"` + fleetID + `","messages":[{"role":"user","content":[` +
		`{"type":"text","text":"描述这张图"},` +
		`{"type":"image_url","image_url":{"url":"data:image/png;base64,AAAA"}}]}]}`
	got := doHandle(g, body)

	if got.Code != http.StatusOK {
		t.Fatalf("有实测产物驱动的合法快照（%s ⟷ %s）时，带图请求必须走通正门（200），实际 %d，body=%s",
			fleetID, registryID, got.Code, got.Body.String())
	}
	if strings.Contains(got.Body.String(), "capability_unavailable") {
		t.Fatalf("不应再报能力闸拒绝，实际 body=%s", got.Body.String())
	}
	if atomic.LoadInt32(&hits) == 0 {
		t.Fatal("请求没有真发到引擎端点（正门放行却未转发 = 静默丢图）")
	}
	if !strings.Contains(got.Body.String(), "我看到了图") {
		t.Fatalf("应把引擎的回答原样带回，实际 body=%s", got.Body.String())
	}

	// ── 负控①（不许放宽）：目标引擎未证过 vision（vllm）⇒ 同一条带图请求仍 400 ──
	gOnly := &Gateway{
		config: &config.FleetConfig{
			Models: map[string][]config.ModelCandidate{
				fleetID: {{Host: "node-a", Backend: "vllm", File: "/m/" + e.Weights}},
			},
			Fleet: map[string]config.FleetNode{"node-a": {Host: host, Port: port}},
		},
		roundRobin:     map[string]int{},
		store:          store.NewStore(),
		modelsRootPath: root,
		client:         &http.Client{Timeout: 5 * time.Second},
	}
	before := atomic.LoadInt32(&hits)
	blocked := doHandle(gOnly, body)
	if blocked.Code != http.StatusBadRequest || !strings.Contains(blocked.Body.String(), "capability_unavailable") {
		t.Fatalf("目标引擎 vllm 未被证过 vision，仍须 400 capability_unavailable，实际 %d body=%s",
			blocked.Code, blocked.Body.String())
	}
	if atomic.LoadInt32(&hits) != before {
		t.Fatal("被门槛拦下的请求不得发到引擎端点")
	}
}
