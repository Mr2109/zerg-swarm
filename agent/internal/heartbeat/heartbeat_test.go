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

// newTestRunner 组装一个只走注入源的心跳器（不触网、不起进程）。
// （第 7 参数 UnmanagedSource 已随 P4 退场清理删除——附录 C·C7。）
func newTestRunner(b *fakeBackend, v vramSource, a ActiveCounter) *Runner {
	return &Runner{
		machine:   "x3",
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
	if body["machine"] != "x3" || body["backend_state"] != "ready" || body["model"] != "example-35b" {
		t.Fatalf("旧字段语义被改动: %v", body)
	}
}

// TestBuildBody_JSON_OldReaderIgnoresNewFields —— 旧读者（只认旧字段）能解析新心跳体。
func TestBuildBody_JSON_OldReaderIgnoresNewFields(t *testing.T) {
	b := baseBackend()
	b.resident = []backend.ResidentDetail{{Alias: "m1", State: "ready", Managed: true, Source: "managed"}}
	r := newTestRunner(b, fakeVram{used: 7.5, total: 24, known: true}, &fakeActive{n: 2})

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
	if legacy.Machine != "x3" || legacy.Model != "example-35b" || legacy.BackendRssGb != 12.5 {
		t.Fatalf("旧读者读到的旧字段语义不对: %+v", legacy)
	}
	if legacy.ActiveRequests != 2 {
		t.Fatalf("旧 reader 的 active_requests 应为真值 2，实得 %d", legacy.ActiveRequests)
	}
	if !legacy.Healthy {
		t.Fatal("healthy 应为 true")
	}
}
