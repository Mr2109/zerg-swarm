package heartbeat

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Mr2109/zerg-swarm/agent/internal/backend"
	"github.com/Mr2109/zerg-swarm/agent/internal/monitor"
)

// ── 假实现（注入用） ──────────────────────────────────────────────

type fakeBackend struct {
	model    string
	names    []string
	state    string
	rssGb    float64
	healthy  bool
	resident []backend.ResidentDetail
}

func (f *fakeBackend) CurrentModel() string                     { return f.model }
func (f *fakeBackend) RegistryNames() []string                  { return f.names }
func (f *fakeBackend) State() string                            { return f.state }
func (f *fakeBackend) BackendRssGb() float64                    { return f.rssGb }
func (f *fakeBackend) IsHealthy() bool                          { return f.healthy }
func (f *fakeBackend) ResidentDetail() []backend.ResidentDetail { return f.resident }

type fakeVram struct {
	used, total float64
	known       bool
}

func (v fakeVram) VramUsedGb() (float64, bool)  { return v.used, v.known }
func (v fakeVram) VramTotalGb() (float64, bool) { return v.total, v.known }

type fakeActive struct{ n int }

func (a *fakeActive) ActiveRequests() int { return a.n }

// fakeEngineLive 引擎实时态假来源（注入用 · GAP-20260925-45）。
type fakeEngineLive struct{ states []backend.EngineLive }

func (f *fakeEngineLive) EngineLiveStates() []backend.EngineLive { return f.states }

// withEngineLive 给测试心跳器接上引擎实时态来源。
func withEngineLive(r *Runner, src engineLiveSource) *Runner {
	r.engineLive = src
	return r
}

// newTestRunner 组装一个只走注入源的心跳器（不触网、不起进程）。
// （第 7 参数 UnmanagedSource 已随 P4 退场清理删除——附录 C·C7。）
func newTestRunner(b *fakeBackend, v vramSource, a ActiveCounter) *Runner {
	return &Runner{
		machine:   "edge-node",
		backend:   b,
		sampler:   &monitor.Sampler{},
		vram:      v,
		active:    a,
		startedAt: time.Now(),
	}
}

func baseBackend() *fakeBackend {
	return &fakeBackend{model: "example-35b", names: []string{"example-35b"}, state: "ready", rssGb: 12.5, healthy: true}
}

// ── 反例优先：显存拿不到时绝不冒充 ────────────────────────────────

// TestBuildBody_VramUnknown_NoFakeGpuUsed —— 反例：显存未知时，
// 心跳里**不得出现** gpu_used_gb（更不得等于 RSS），只如实标 vram_known=false。
func TestBuildBody_VramUnknown_NoFakeGpuUsed(t *testing.T) {
	b := baseBackend()
	r := newTestRunner(b, fakeVram{known: false}, &fakeActive{})
	body := r.buildBody()

	if _, ok := body["gpu_used_gb"]; ok {
		t.Fatalf("显存未知时不得出现 gpu_used_gb（旧行为用 RSS 冒充）；实得 %v", body["gpu_used_gb"])
	}
	// 真 RSS 仍在 backend_rss_gb 里，二者绝不混同
	if got := body["backend_rss_gb"]; got != 12.5 {
		t.Fatalf("backend_rss_gb 应为真实 RSS 12.5，实得 %v", got)
	}
	if known, ok := body["vram_known"]; !ok || known != false {
		t.Fatalf("显存未知必须显式标 vram_known=false，实得 %v", body["vram_known"])
	}
	if _, ok := body["vram_used_gb"]; ok {
		t.Fatal("显存未知时不得出现 vram_used_gb")
	}
	if _, ok := body["vram_total_gb"]; ok {
		t.Fatal("显存未知时不得出现 vram_total_gb")
	}
}

// TestBuildBody_VramUnknown_NeverEqualsRss —— 反例加固：即便有人把 RSS 当显存，
// 断言 gpu_used_gb 也绝不等同于 backend_rss_gb（缺席即不等同）。
func TestBuildBody_VramUnknown_NeverEqualsRss(t *testing.T) {
	b := baseBackend()
	r := newTestRunner(b, fakeVram{known: false}, &fakeActive{})
	body := r.buildBody()

	if v, ok := body["gpu_used_gb"]; ok && v == body["backend_rss_gb"] {
		t.Fatalf("gpu_used_gb 绝不能用 RSS 冒充：gpu=%v rss=%v", v, body["backend_rss_gb"])
	}
}

// TestBuildBody_VramKnown_EqualsInjected —— 显存可拿到时等于注入真值（且非 RSS）。
func TestBuildBody_VramKnown_EqualsInjected(t *testing.T) {
	b := baseBackend()
	r := newTestRunner(b, fakeVram{used: 7.5, total: 24, known: true}, &fakeActive{})
	body := r.buildBody()

	if got := body["gpu_used_gb"]; got != 7.5 {
		t.Fatalf("gpu_used_gb 应等于注入的真显存 7.5，实得 %v", got)
	}
	if got := body["gpu_used_gb"]; got == body["backend_rss_gb"] {
		t.Fatalf("gpu_used_gb 不应等于 RSS（rss=%v）", body["backend_rss_gb"])
	}
	if got := body["vram_used_gb"]; got != 7.5 {
		t.Fatalf("vram_used_gb 应为 7.5，实得 %v", got)
	}
	if got := body["vram_total_gb"]; got != 24.0 {
		t.Fatalf("vram_total_gb 应为 24，实得 %v", got)
	}
	if got := body["vram_free_gb"]; got != 16.5 {
		t.Fatalf("vram_free_gb 应为 16.5，实得 %v", got)
	}
	if body["vram_known"] != true {
		t.Fatalf("vram_known 应为 true，实得 %v", body["vram_known"])
	}
}

// ── active_requests 接真实在飞计数 ────────────────────────────────

// TestBuildBody_ActiveRequests_TracksCounter —— 随假计数器增减；无来源则缺席（不写死 0）。
func TestBuildBody_ActiveRequests_TracksCounter(t *testing.T) {
	b := baseBackend()
	counter := &fakeActive{n: 0}
	r := newTestRunner(b, fakeVram{known: false}, counter)

	if got := r.buildBody()["active_requests"]; got != 0 {
		t.Fatalf("真实 0 应如实报 0，实得 %v", got)
	}
	counter.n = 3
	if got := r.buildBody()["active_requests"]; got != 3 {
		t.Fatalf("随计数增到 3，实得 %v", got)
	}
	counter.n = 5
	if got := r.buildBody()["active_requests"]; got != 5 {
		t.Fatalf("随计数增到 5，实得 %v", got)
	}
	counter.n = 0
	if got := r.buildBody()["active_requests"]; got != 0 {
		t.Fatalf("随计数回落 0，实得 %v", got)
	}
}

// TestBuildBody_ActiveRequests_NoSourceAbsent —— 无计数来源时该字段缺席（绝不写死 0）。
func TestBuildBody_ActiveRequests_NoSourceAbsent(t *testing.T) {
	r := newTestRunner(baseBackend(), fakeVram{known: false}, nil)
	if _, ok := r.buildBody()["active_requests"]; ok {
		t.Fatal("无在飞计数来源时不得出现 active_requests（旧行为写死 0）")
	}
}

// ── 驻留明细 ──────────────────────────────────────────────────────

// TestBuildBody_Resident_ManagedOnly —— 托管项 managed=true 如实上报。
// （unmanaged[] 未托管探测已随 P4 退场清理删除——附录 C·C7。）
func TestBuildBody_Resident_ManagedOnly(t *testing.T) {
	b := baseBackend()
	b.resident = []backend.ResidentDetail{{
		Alias: "example-35b", File: "/data/models/example-35b-v2.gguf", State: "ready",
		ReqCount: 1, RssGb: 22.3, Managed: true, MemGb: 22, CtxWindow: 262144, Source: "managed",
	}}
	r := newTestRunner(b, fakeVram{known: false}, &fakeActive{})
	body := r.buildBody()

	res, ok := body["resident"].([]backend.ResidentDetail)
	if !ok {
		t.Fatalf("resident 应为明细列表，实得 %T", body["resident"])
	}
	if len(res) != 1 || !res[0].Managed || res[0].Source != "managed" {
		t.Fatalf("resident 应只有 1 个托管项，实得 %+v", res)
	}
	if _, exists := body["unmanaged"]; exists {
		t.Fatal("unmanaged[] 已随 P4 退场清理删除，不得再出现在心跳里（附录 C·C7）")
	}
}

// ── 向后兼容 ──────────────────────────────────────────────────────

// TestBuildBody_OldFieldsStillPresent —— 旧字段名与语义不变。
func TestBuildBody_OldFieldsStillPresent(t *testing.T) {
	r := newTestRunner(baseBackend(), fakeVram{used: 7.5, total: 24, known: true}, &fakeActive{n: 1})
	body := r.buildBody()

	for _, k := range []string{
		"machine", "backend_state", "model", "mem_available_gb", "mem_total_gb",
		"gpu_temp_c", "load", "models", "uptime", "backend_rss_gb", "healthy",
		"error", "cpu_pct", "gpu_pct", "code_version", "code_sha", "active_requests",
	} {
		if _, ok := body[k]; !ok {
			t.Fatalf("旧字段 %q 不应消失（向后兼容）", k)
		}
	}
	if body["machine"] != "edge-node" || body["backend_state"] != "ready" || body["model"] != "example-35b" {
		t.Fatalf("旧字段语义被改动: %v", body)
	}
}

// TestBuildBody_JSON_OldReaderIgnoresNewFields —— 旧读者（只认旧字段）能解析新心跳体。
func TestBuildBody_JSON_OldReaderIgnoresNewFields(t *testing.T) {
	b := baseBackend()
	b.resident = []backend.ResidentDetail{{Alias: "m1", State: "ready", Managed: true, Source: "managed"}}
	r := withEngineLive(newTestRunner(b, fakeVram{used: 7.5, total: 24, known: true}, &fakeActive{n: 2}),
		&fakeEngineLive{states: []backend.EngineLive{{EggID: "m1", State: "ready", Inflight: 2, Verdict: "ok"}}})

	raw, err := json.Marshal(r.buildBody())
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}

	// 旧读者结构（不含任何新增字段）
	var legacy struct {
		Machine        string  `json:"machine"`
		Model          string  `json:"model"`
		BackendState   string  `json:"backend_state"`
		MemAvailableGb float64 `json:"mem_available_gb"`
		BackendRssGb   float64 `json:"backend_rss_gb"`
		ActiveRequests int     `json:"active_requests"`
		Healthy        bool    `json:"healthy"`
		GpuUsedGb      float64 `json:"gpu_used_gb"`
	}
	if err := json.Unmarshal(raw, &legacy); err != nil {
		t.Fatalf("旧读者解析新心跳体失败（新增字段必须可选）: %v", err)
	}
	if legacy.Machine != "edge-node" || legacy.Model != "example-35b" || legacy.BackendRssGb != 12.5 {
		t.Fatalf("旧读者读到的旧字段语义不对: %+v", legacy)
	}
	if legacy.ActiveRequests != 2 {
		t.Fatalf("旧 reader 的 active_requests 应为真值 2，实得 %d", legacy.ActiveRequests)
	}
	if !legacy.Healthy {
		t.Fatal("healthy 应为 true")
	}
}

// ── 引擎实时态（GAP-20260925-45）──────────────────────────────────
// 为什么这四例：心跳要能带出「此刻在不在算 / 有没有输出 / 卡没卡」，且遵守
// 「没有来源就不出现」的诚实纪律（空集合 ≠ 空数组撑场）。

// TestBuildBody_EngineLive_CarriesStates —— 有来源 ⇒ engines[] 原样带上每枚引擎的实时事实。
func TestBuildBody_EngineLive_CarriesStates(t *testing.T) {
	src := &fakeEngineLive{states: []backend.EngineLive{
		{EggID: "example-35b", State: "ready", PID: 4242, Inflight: 2, FailN: 1,
			Verdict: "extended", LastOutputAgoS: 12.5, CPUUsecDelta: 900000, Reason: "窗口无输出但工时在涨"},
		{EggID: "gemma-4-12B", State: "ready", PID: 4243, Inflight: 0, Verdict: "ok"},
	}}
	r := withEngineLive(newTestRunner(baseBackend(), fakeVram{known: false}, &fakeActive{}), src)
	body := r.buildBody()

	got, ok := body["engines"].([]backend.EngineLive)
	if !ok {
		t.Fatalf("有来源时 engines 应为 []backend.EngineLive，实得 %T", body["engines"])
	}
	if len(got) != 2 {
		t.Fatalf("engines 应有 2 枚，实得 %d", len(got))
	}
	if got[0].Inflight != 2 || got[0].Verdict != "extended" || got[0].LastOutputAgoS != 12.5 || got[0].CPUUsecDelta != 900000 {
		t.Fatalf("engines[0] 实时事实未如实带上: %+v", got[0])
	}
	if got[1].EggID != "gemma-4-12B" || got[1].Verdict != "ok" {
		t.Fatalf("engines[1] 实时事实未如实带上: %+v", got[1])
	}
}

// TestBuildBody_EngineLive_NoSourceAbsent —— 无来源 ⇒ engines 缺席（绝不写死空数组）。
func TestBuildBody_EngineLive_NoSourceAbsent(t *testing.T) {
	r := newTestRunner(baseBackend(), fakeVram{known: false}, &fakeActive{})
	if _, ok := r.buildBody()["engines"]; ok {
		t.Fatal("无引擎实时态来源时不得出现 engines（与 active_requests 同纪律）")
	}
}

// TestBuildBody_EngineLive_EmptyAbsent —— 有来源但本端无托管引擎 ⇒ 仍缺席（空 ≠ 撑一个空数组）。
func TestBuildBody_EngineLive_EmptyAbsent(t *testing.T) {
	r := withEngineLive(newTestRunner(baseBackend(), fakeVram{known: false}, &fakeActive{}), &fakeEngineLive{})
	if v, ok := r.buildBody()["engines"]; ok {
		t.Fatalf("无托管引擎时 engines 应缺席（空集合 ≠ 空数组撑场），实得 %v", v)
	}
}

// TestBuildBody_EngineLive_JSONRoundTrip —— engines[] 能序列化并原样解回（主控消费面同一形状）。
func TestBuildBody_EngineLive_JSONRoundTrip(t *testing.T) {
	src := &fakeEngineLive{states: []backend.EngineLive{
		{EggID: "e1", State: "ready", PID: 7, Inflight: 1, Verdict: "stuck_cpu_stalled",
			LastOutputAgoS: 91, CPUUsecDelta: 0, Reason: "工时一点不动"},
	}}
	r := withEngineLive(newTestRunner(baseBackend(), fakeVram{known: false}, &fakeActive{}), src)
	raw, err := json.Marshal(r.buildBody())
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	var back struct {
		Engines []backend.EngineLive `json:"engines"`
	}
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("engines 解析失败: %v", err)
	}
	if len(back.Engines) != 1 || back.Engines[0].EggID != "e1" || back.Engines[0].Verdict != "stuck_cpu_stalled" {
		t.Fatalf("engines JSON 往返不全: %+v", back.Engines)
	}
}
