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
	"testing"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/config"
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

// failoverGateGateway：模型 visionmodel 两候选——mini1=llama-server（vision 已证）/
// x3=vllm（vision 未证）。正是「首跳引擎被证过、换机目标未验证」的场景。
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
				{Host: "mini1", Backend: "llama-server", File: "/mini1/visionmodel.gguf"},
				{Host: "x3", Backend: "vllm", File: "/data/visionmodel.gguf"},
			},
		},
		Fleet: map[string]config.FleetNode{
			"mini1": {Host: "<worker-ip>", Port: 8100},
			"x3":    {Host: "<worker-ip>", Port: 8100},
		},
	}
	st := store.NewStore()
	injectSnap(st, "mini1", "visionmodel", 0, true, 0.1)
	injectSnap(st, "x3", "visionmodel", 0, true, 0.1)
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

// ②a 首跳引擎（mini1/llama.cpp）被证过 vision，换机目标只剩 x3（vllm，未验证）
// → 换机后**必须**被拦（fail-closed，*CapabilityGateError，分类 400 capability_unavailable）。
func TestPickFallbackRoute_ReGatesCapabilityAfterSwitch(t *testing.T) {
	g := failoverGateGateway(t)
	failed := &RouteResult{Host: "mini1"} // 首跳 = 被证过的引擎

	_, err := g.pickFallbackRoute(failed, "visionmodel", "backend mini1 forward failed: i/o timeout", []string{"vision"})
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
	route, err := g.pickFallbackRoute(&RouteResult{Host: "mini1"}, "visionmodel", "backend mini1 forward failed: i/o timeout", nil)
	if err != nil {
		t.Fatalf("无必需能力时换机不应被误拦：%v", err)
	}
	if route.Host != "x3" {
		t.Fatalf("应换到 x3，实际 %s", route.Host)
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
	route, err := g.pickFallbackRoute(&RouteResult{Host: "mini1"}, "visionmodel", "backend mini1 forward failed: i/o timeout", []string{"vision"})
	if err != nil {
		t.Fatalf("目标引擎也被证过 vision，换机不应被拦：%v", err)
	}
	if route.Host != "x3" {
		t.Fatalf("应换到 x3，实际 %s", route.Host)
	}
}

// ②d 端到端：forwardToBackend 内的换机 failover 必须真正拿到 required（而非只有直调 pickFallbackRoute 才生效），
// 且门槛错误原样上抛（不被 transport 错误盖成 502）。
func TestForwardToBackend_FailoverReGatesWithRequired(t *testing.T) {
	g := failoverGateGateway(t)
	g.client = &http.Client{Timeout: 5 * time.Second}

	// 首跳 URL 指向必然失败的本机端口（连接被拒）→ 触发内部换机 failover
	route := &RouteResult{Host: "mini1", Port: 1, URL: "http://127.0.0.1:1/infer"}
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
