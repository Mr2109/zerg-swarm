package api

// resources_yield_visibility_test.go —— T1 / T4 / T6（《设计-资源管理器》§七）故障注入式验收。
//
// 观测面说明（如实）：设计稿 §3.4 里的 `GET /api/resources/fit` 与 `GET /api/resources/residency`
// 至今未实现；这两者的信息在**已实现**的 `GET /api/resources/ledger` 里承载——
// 每机 `resident[]`（= residency）与逐模型 `fit[]`（含 verdict/evict_plan/estimated/basis，= fit）。
// 本条据此在 ledger 上做故障注入式断言（不改产品接口形状）。
//
// 与既有用例的分工（不重复）：
//   - fit_test.go / ledger_test.go 覆盖纯计算（EstimateFit 的 verdict/evict_plan/estimated 表）；
//   - resources_ledger_test.go 覆盖鉴权/只读/estimated；
//   - 本文件补的是"**驱逐可见（T1 前后各取一次）**"与"**显存不足/缺关键输入在观测面的 fail-closed**"。

import (
	"net/http"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/config"
	"github.com/Mr2109/zerg-swarm/core/internal/resources"
	"github.com/Mr2109/zerg-swarm/core/internal/store"
)

// t1TargetConfig 造一台 x3 上的登记模型（权重 4G，GGUF 带 KV 真值）：
// need = 4G 权重 + 0.5G KV + 2G 开销 = 6.5G（用于把"装不装得下"卡在可控的区间里）。
func t1TargetConfig(gguf string) *config.FleetConfig {
	return &config.FleetConfig{
		Models: map[string][]config.ModelCandidate{
			"TestModel": {{Host: "x3", File: gguf, Architecture: "llama"}},
		},
		Fleet: map[string]config.FleetNode{},
	}
}

// T1：驻留 A → 请求 B（单槽装不下）→ 账本 resident 先出现 A 且 fit.evict_plan 指向 A；
// A 让位、B 驻留后再取一次：resident 出现 B、verdict=fit 且无 evict_plan（过程可查）。
func TestT1_SingleSlotYieldVisibleInLedger(t *testing.T) {
	h, dir := newLedgerHandlers(t)
	gguf := writeLedgerGGUF(t, dir, "target.gguf", true)
	writeLedgerRecord(t, dir, "targetmodel", "sha256-abc", "target.gguf", int64(4)<<30)
	h.Config = t1TargetConfig(gguf)
	h.KvCacheBytesPerElem = 2.0
	h.EngineOverheadGb = 2.0
	r := newResourcesTestRouter(h)

	// ── 前：驻留 A（2G、空闲可腾退）；可用内存 5G < need 6.5G → 单槽装不下 B ──
	h.Store.ReceiveHeartbeat(store.HeartbeatRequest{
		Machine: "x3", MemTotalGb: 128, MemAvailableGb: 5,
		VramKnown: true, VramTotalGb: 48, VramUsedGb: 8, VramFreeGb: 40,
		Resident: []resources.ResidentEntry{{
			Alias: "A", Digest: "sha256-A", State: resources.StateReady,
			MemGb: 2, Managed: true, LastUsedAgoS: 100,
		}},
	})
	before := machinesOf(t, decodeLedger(t, doResReq(t, r, http.MethodGet, "/api/resources/ledger", "test-token", "")))["x3"]

	arr, ok := before["resident"].([]interface{})
	if !ok || len(arr) != 1 {
		t.Fatalf("前：resident 必须先出现 A，实得 %v", before["resident"])
	}
	if a, _ := arr[0].(map[string]interface{}); a["alias"] != "A" {
		t.Fatalf("前：resident[0] 应为 A，实得 %v", a)
	}
	fit := firstFit(t, before)
	if fit["verdict"] != "evict" {
		t.Fatalf("前：单槽装不下应判 evict（驱逐后可跑），实得 %v basis=%v", fit["verdict"], fit["basis"])
	}
	plan, ok := fit["evict_plan"].([]interface{})
	if !ok || len(plan) != 1 || plan[0] != "A" {
		t.Fatalf("前：evict_plan 必须指向 A，实得 %v", fit["evict_plan"])
	}

	// ── 后：A 让位、B 驻留；再取一次账本 ──
	h.Store.ReceiveHeartbeat(store.HeartbeatRequest{
		Machine: "x3", MemTotalGb: 128, MemAvailableGb: 7,
		VramKnown: true, VramTotalGb: 48, VramUsedGb: 12, VramFreeGb: 36,
		Resident: []resources.ResidentEntry{{
			Alias: "B", Digest: "sha256-B", State: resources.StateReady,
			MemGb: 6.5, Managed: true,
		}},
	})
	after := machinesOf(t, decodeLedger(t, doResReq(t, r, http.MethodGet, "/api/resources/ledger", "test-token", "")))["x3"]

	arr2, ok := after["resident"].([]interface{})
	if !ok || len(arr2) != 1 {
		t.Fatalf("后：resident 应出现 B，实得 %v", after["resident"])
	}
	if b, _ := arr2[0].(map[string]interface{}); b["alias"] != "B" {
		t.Fatalf("后：resident[0] 应为 B（A 已让位），实得 %v", b)
	}
	fit2 := firstFit(t, after)
	if fit2["verdict"] != "fit" {
		t.Fatalf("后：A 让位后目标应可 fit，实得 %v basis=%v", fit2["verdict"], fit2["basis"])
	}
	if v, has := fit2["evict_plan"]; has {
		t.Fatalf("后：已判 fit 不得再带 evict_plan，实得 %v", v)
	}
}

// T4：内存够、但 vram_free_gb 不足 → ledger 的 fit 判 no_fit（§3.3d 两种约束取严，显存独立生效）。
func TestT4_MemEnoughVramShort_NoFit(t *testing.T) {
	h, dir := newLedgerHandlers(t)
	gguf := writeLedgerGGUF(t, dir, "target.gguf", true)
	writeLedgerRecord(t, dir, "targetmodel", "sha256-abc", "target.gguf", int64(4)<<30)
	h.Config = t1TargetConfig(gguf)
	h.KvCacheBytesPerElem = 2.0
	h.EngineOverheadGb = 2.0
	// 内存充裕（100G），显存只剩 1G（need 6.5G）
	h.Store.ReceiveHeartbeat(store.HeartbeatRequest{
		Machine: "x3", MemTotalGb: 128, MemAvailableGb: 100,
		VramKnown: true, VramTotalGb: 24, VramUsedGb: 23, VramFreeGb: 1,
	})
	r := newResourcesTestRouter(h)
	w := doResReq(t, r, http.MethodGet, "/api/resources/ledger", "test-token", "")
	if w.Code != http.StatusOK {
		t.Fatalf("状态码应为 200，实得 %d body=%s", w.Code, w.Body.String())
	}
	x3 := machinesOf(t, decodeLedger(t, w))["x3"]
	if x3["vram_known"] != true {
		t.Fatalf("前置：x3 应报有显存，实得 vram_known=%v", x3["vram_known"])
	}
	if x3["vram_free_gb"] != float64(1) {
		t.Fatalf("前置：账本应如实报 vram_free_gb=1，实得 %v", x3["vram_free_gb"])
	}
	fit := firstFit(t, x3)
	if fit["verdict"] != "no_fit" {
		t.Fatalf("内存够但显存不足必须判 no_fit（二者取严），实得 %v basis=%v", fit["verdict"], fit["basis"])
	}
	basis, _ := fit["basis"].(string)
	if strings.TrimSpace(basis) == "" {
		t.Fatal("no_fit 必须给出可解释的 basis")
	}
	// 判别力所在：必须是"显存真报了、且真不够"导致的 no_fit——
	// 而不是"显存未知 fail-closed"（那也会 no_fit 且 basis 里含"显存"，不足以区分）。
	if !strings.Contains(basis, "显存：空闲=1.00") {
		t.Fatalf("basis 必须写明显存真值不足（空闲=1.00），实得 %q", basis)
	}
	if strings.Contains(basis, "显存未知") {
		t.Fatalf("不得退化成「显存未知 fail-closed」，实得 %q", basis)
	}
}

// T6：估不出（缺关键输入：权重字节）→ 观测面 verdict=no_fit（fail-closed，不许默认放行），
// 且 estimated=true、basis 非空并写明原因。
func TestT6_MissingKeyInput_FailsClosedNoFitWithBasis(t *testing.T) {
	h, dir := newLedgerHandlers(t)
	gguf := writeLedgerGGUF(t, dir, "target.gguf", true)
	// 登记记录的权重字节缺失（size=0）——估不出的关键输入。
	writeLedgerRecord(t, dir, "targetmodel", "sha256-abc", "target.gguf", 0)
	h.Config = t1TargetConfig(gguf)
	h.KvCacheBytesPerElem = 2.0
	h.EngineOverheadGb = 2.0
	// 内存/显存都充裕（统一内存）——若估算"默认放行"，就会误判 fit。
	h.Store.ReceiveHeartbeat(store.HeartbeatRequest{
		Machine: "local", MemTotalGb: 128, MemAvailableGb: 120, VramUnified: true,
	})
	r := newResourcesTestRouter(h)
	w := doResReq(t, r, http.MethodGet, "/api/resources/ledger", "test-token", "")
	fit := firstFit(t, machinesOf(t, decodeLedger(t, w))["local"])
	if fit["verdict"] != "no_fit" {
		t.Fatalf("缺关键输入（权重字节）必须 fail-closed 判 no_fit，实得 %v basis=%v", fit["verdict"], fit["basis"])
	}
	if fit["estimated"] != true {
		t.Fatalf("fail-closed 结论必须标 estimated=true，实得 %v", fit["estimated"])
	}
	basis, _ := fit["basis"].(string)
	if strings.TrimSpace(basis) == "" {
		t.Fatal("basis 必须非空（可解释）")
	}
	if !strings.Contains(basis, "权重") {
		t.Fatalf("basis 应写明缺的是权重字节，实得 %q", basis)
	}
}
