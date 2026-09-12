package api

// resources_local_ledger_test.go —— 批 5：local 一行如实 + 显存取严在观测面的反例优先测试。
//
// 覆盖 #29（消费方不误判）、#30（local 账本有/无驻留都如实、vram 未知 vram_known=false）、
// 以及"显存未知 fail-closed / 统一内存不 fail-closed"在 /api/resources/ledger 的可见性。

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/config"
	"github.com/Mr2109/zerg-swarm/core/internal/localback"
	"github.com/Mr2109/zerg-swarm/core/internal/resources"
	"github.com/Mr2109/zerg-swarm/core/internal/store"
)

// #29 消费方（GET /api/fleet/status）：本机走 LocalBack 快照——不得因显存缺席而崩/误判。
func TestStatusHandler_LocalRowDoesNotFakeVram(t *testing.T) {
	h := newTestHandlers()
	h.LocalBack = localback.NewLocalBackend("")

	req := httptest.NewRequest(http.MethodGet, "/api/fleet/status", nil)
	w := httptest.NewRecorder()
	h.StatusHandler(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("状态码应为 200，实得 %d body=%s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("响应不是 JSON: %v", err)
	}
	machines, _ := resp["machines"].(map[string]interface{})
	local, ok := machines["local"].(map[string]interface{})
	if !ok {
		t.Fatalf("local 一行必须存在（本机不经心跳，由主控自填），实得 %v", machines)
	}
	// 关键：不得拿内存量冒充显存——未加载模型时显存就是 0/未知
	if gpu, _ := local["gpu_used_gb"].(float64); gpu != 0 {
		t.Fatalf("本机 gpu_used_gb 不得是假值（应为 0/未知），实得 %v", gpu)
	}
	if v, has := local["vram_known"]; has && v == true {
		t.Fatalf("本机拿不到独立显存，vram_known 不得为 true，实得 %v", v)
	}
	if tot, _ := resp["total_gpu_used_gb"].(float64); tot != 0 {
		t.Fatalf("总显存占用不得含本机假值，实得 %v", tot)
	}
}

// #30：local 一行**有驻留**时 resident[] 非空且字段正确；显存未知 vram_known=false（不出现假值键）。
func TestResourceLedger_LocalRowCarriesResidentAndHonestVram(t *testing.T) {
	h := newTestHandlers() // Store 已就绪
	m := "example-35b-v2"
	h.Store.SetLocalSnapshot(store.LocalSnapshotData{
		Machine: "local", Model: &m, Models: []string{m},
		Healthy: true, State: "ready", MemAvailableGb: 20, MemTotalGb: 64,
		Resident: []resources.ResidentEntry{{
			Alias: "example-35b-v2", File: "/models/example-35b-v2.gguf",
			State: resources.StateReady, MemGb: 22, Managed: true, Source: "localback",
		}},
		VramUnified: true, // Apple Silicon：显存即内存
	})
	r := newResourcesTestRouter(h)
	w := doResReq(t, r, http.MethodGet, "/api/resources/ledger", "test-token", "")
	if w.Code != http.StatusOK {
		t.Fatalf("状态码应为 200，实得 %d body=%s", w.Code, w.Body.String())
	}
	local := machinesOf(t, decodeLedger(t, w))["local"]
	if local["vram_known"] != false {
		t.Fatalf("本机无独立显存，vram_known 应为 false，实得 %v", local["vram_known"])
	}
	if local["vram_unified"] != true {
		t.Fatalf("统一内存平台应标 vram_unified=true，实得 %v", local["vram_unified"])
	}
	for _, k := range []string{"vram_total_gb", "vram_used_gb", "vram_free_gb"} {
		if v, has := local[k]; has {
			t.Fatalf("显存未知时 %s 必须整键不出现（不造值），实得 %v", k, v)
		}
	}
	arr, ok := local["resident"].([]interface{})
	if !ok || len(arr) != 1 {
		t.Fatalf("local 有驻留时 resident[] 应为 1 项，实得 %v", local["resident"])
	}
	entry := arr[0].(map[string]interface{})
	if entry["alias"] != "example-35b-v2" || entry["state"] != "ready" || entry["managed"] != true {
		t.Fatalf("驻留项字段不符: %v", entry)
	}
	if entry["mem_gb"] != float64(22) {
		t.Fatalf("驻留项 mem_gb 应为 22，实得 %v", entry["mem_gb"])
	}
}

// #30：local 一行**无驻留**时不为空也不报错（resident 整键不出现）。
func TestResourceLedger_LocalRowNoResidentIsHonest(t *testing.T) {
	h := newTestHandlers()
	// 本机未加载任何模型：resident 为空、显存未知
	h.Store.SetLocalSnapshot(store.LocalSnapshotData{
		Machine: "local", Healthy: false, State: "idle", MemAvailableGb: 40, MemTotalGb: 64, VramUnified: true,
	})
	r := newResourcesTestRouter(h)
	w := doResReq(t, r, http.MethodGet, "/api/resources/ledger", "test-token", "")
	if w.Code != http.StatusOK {
		t.Fatalf("无驻留也必须 200（不许报错），实得 %d body=%s", w.Code, w.Body.String())
	}
	local := machinesOf(t, decodeLedger(t, w))["local"]
	if _, has := local["resident"]; has {
		t.Fatalf("无驻留时 resident 必须整键不出现，实得 %v", local["resident"])
	}
	if local["vram_known"] != false {
		t.Fatalf("vram_known 应为 false，实得 %v", local["vram_known"])
	}
	if local["mem_known"] != true {
		t.Fatalf("有内存数据时 mem_known 应为 true，实得 %v", local["mem_known"])
	}
}

// 取严可见性：显存未知且非统一内存 → 账本里的 fit 判 no_fit + estimated=true（fail-closed 可见）；
// 统一内存（显存即内存）→ 按内存口径判 fit，不 fail-closed。
func TestResourceLedger_VramUnknownFailsClosedUnifiedDoesNot(t *testing.T) {
	h, dir := newLedgerHandlers(t)
	gguf := writeLedgerGGUF(t, dir, "kvtruth.gguf", true)
	writeLedgerRecord(t, dir, "testmodel", "sha256-abc", "kvtruth.gguf", int64(4)<<30)
	h.Config = &config.FleetConfig{
		Models: map[string][]config.ModelCandidate{
			"TestModel": {{Host: "local", File: gguf, Architecture: "llama"}},
		},
		Fleet: map[string]config.FleetNode{},
	}
	h.KvCacheBytesPerElem = 2.0
	h.EngineOverheadGb = 2.0
	r := newResourcesTestRouter(h)

	// A: 显存未知且非统一内存（机器像 GPU 机但读不到显存）→ fail-closed
	h.Store.ReceiveHeartbeat(store.HeartbeatRequest{Machine: "x3", MemTotalGb: 128, MemAvailableGb: 120})
	w := doResReq(t, r, http.MethodGet, "/api/resources/ledger", "test-token", "")
	if w.Code != http.StatusOK {
		t.Fatalf("状态码应为 200，实得 %d", w.Code)
	}
	fit := firstFit(t, machinesOf(t, decodeLedger(t, w))["x3"])
	if fit["verdict"] != "no_fit" {
		t.Fatalf("显存未知（非统一内存）必须 fail-closed 判 no_fit，实得 %v basis=%v", fit["verdict"], fit["basis"])
	}
	if fit["estimated"] != true {
		t.Fatalf("fail-closed 结论必须标 estimated=true，实得 %v", fit["estimated"])
	}
	if b, _ := fit["basis"].(string); !strings.Contains(b, "显存未知") {
		t.Fatalf("basis 应写明显存未知，实得 %q", b)
	}

	// B: 统一内存（显存即内存）→ 不得 fail-closed
	h.Store.ReceiveHeartbeat(store.HeartbeatRequest{Machine: "local", MemTotalGb: 128, MemAvailableGb: 120, VramUnified: true})
	w2 := doResReq(t, r, http.MethodGet, "/api/resources/ledger", "test-token", "")
	fitLocal := firstFit(t, machinesOf(t, decodeLedger(t, w2))["local"])
	if fitLocal["verdict"] == "no_fit" {
		t.Fatalf("统一内存（显存即内存）不得 fail-closed，实得 basis=%v", fitLocal["basis"])
	}
}

// ── 本任务：账本里 local 缺席的缺口（机器列表来源与 /api/fleet/status 不同）──────────

// fakeLocalSnap 是 LocalSnapshotProvider 的测试替身：只回放固定快照，不做任何 IO。
// 为什么需要替身：localback.LocalBackend 的 state/file/memGB 在包外不可构造，
// api 侧无法真实造出"本机有驻留"的快照（真实构造见 localback/local_ledger_test.go）。
type fakeLocalSnap struct{ snap *localback.LocalSnapshot }

func (f fakeLocalSnap) Snapshot() *localback.LocalSnapshot { return f.snap }

// 反例优先（缺口本体）：store 里还没有 local 行（刚启动/刚部署，30s 周期任务尚未首次落盘）时，
// 账本仍必须含 local——来源应与同刻的 /api/fleet/status 一致；且不得因此吞掉 x3 等远程行。
func TestResourceLedger_LocalPresentBeforeTickerWrite(t *testing.T) {
	h := newTestHandlers()
	h.LocalBack = localback.NewLocalBackend("") // 无任何 store 写入 = 周期任务尚未落盘
	h.Store.ReceiveHeartbeat(store.HeartbeatRequest{Machine: "x3", MemTotalGb: 128, MemAvailableGb: 120})
	r := newResourcesTestRouter(h)

	w := doResReq(t, r, http.MethodGet, "/api/resources/ledger", "test-token", "")
	if w.Code != http.StatusOK {
		t.Fatalf("状态码应为 200，实得 %d body=%s", w.Code, w.Body.String())
	}
	resp := decodeLedger(t, w)
	machines := machinesOf(t, resp)
	if _, ok := machines["x3"]; !ok {
		t.Fatalf("远程行 x3 不得因加 local 而消失，实得 %v", resp["machines"])
	}
	local, ok := machines["local"]
	if !ok {
		t.Fatalf("周期任务未落盘时账本也必须含 local（来源应与 /api/fleet/status 同一套），实得 %v", resp["machines"])
	}
	if n, _ := resp["count"].(float64); int(n) != 2 {
		t.Fatalf("local 与 x3 同在时 count 应为 2，实得 %v", resp["count"])
	}
	if local["mem_known"] != true {
		t.Fatalf("本机内存可采样 → mem_known 应为 true，实得 %v", local)
	}
}

// local 无驻留/无显存 → 字段缺席（不造假值）、仍 200；统一内存如实标注。
func TestResourceLedger_LocalHonestWhenNothingLoaded(t *testing.T) {
	h := newTestHandlers()
	h.LocalBack = localback.NewLocalBackend("") // idle：无模型、拿不到独立显存
	r := newResourcesTestRouter(h)

	w := doResReq(t, r, http.MethodGet, "/api/resources/ledger", "test-token", "")
	if w.Code != http.StatusOK {
		t.Fatalf("无驻留/无显存也必须 200（不许报错），实得 %d body=%s", w.Code, w.Body.String())
	}
	local := machinesOf(t, decodeLedger(t, w))["local"]
	if local == nil {
		t.Fatalf("local 一行必须在列")
	}
	if v, has := local["resident"]; has {
		t.Fatalf("无驻留时 resident 必须整键不出现，实得 %v", v)
	}
	if local["vram_known"] != false {
		t.Fatalf("本机拿不到独立显存，vram_known 必须为 false（不冒充），实得 %v", local["vram_known"])
	}
	for _, k := range []string{"vram_total_gb", "vram_used_gb", "vram_free_gb"} {
		if v, has := local[k]; has {
			t.Fatalf("显存未知时 %s 必须整键不出现（不造值），实得 %v", k, v)
		}
	}
	// 统一内存平台（显存即内存）：与 localback 同一口径，不因接口而异
	if runtime.GOOS == "darwin" {
		if local["vram_unified"] != true {
			t.Fatalf("Apple Silicon 应标 vram_unified=true（显存即内存），实得 %v", local["vram_unified"])
		}
	} else if v, has := local["vram_unified"]; has {
		t.Fatalf("非统一内存平台不得标 vram_unified，实得 %v", v)
	}
}

// local 有驻留时 resident[] 来自本机实时快照，字段正确（managed=true / source=localback）；
// 同一请求里远程行 x3 的形状不得改变（有显存 → 三值齐全 + resident 照旧）。
func TestResourceLedger_LocalResidentFromLiveSnapshot(t *testing.T) {
	h := newTestHandlers()
	h.LocalSnapProvider = fakeLocalSnap{snap: &localback.LocalSnapshot{
		Machine: "local", BackendState: "ready", Healthy: true,
		MemTotalGb: 64, MemAvailableGb: 20, Models: []string{"example-35b-v2"},
		VramUnified: true, // Apple Silicon：显存即内存，vram_known 仍为 false
		Resident: []resources.ResidentEntry{{
			Alias: "example-35b-v2", File: "/models/example-35b-v2.gguf",
			State: resources.StateReady, MemGb: 22, Managed: true, Source: "localback",
		}},
	}}
	h.Store.ReceiveHeartbeat(store.HeartbeatRequest{
		Machine: "x3", MemTotalGb: 128, MemAvailableGb: 120,
		VramKnown: true, VramTotalGb: 48, VramUsedGb: 10, VramFreeGb: 38,
		Resident: []resources.ResidentEntry{{Alias: "x3-model", Managed: true, Source: "agent"}},
	})
	r := newResourcesTestRouter(h)

	w := doResReq(t, r, http.MethodGet, "/api/resources/ledger", "test-token", "")
	if w.Code != http.StatusOK {
		t.Fatalf("状态码应为 200，实得 %d body=%s", w.Code, w.Body.String())
	}
	machines := machinesOf(t, decodeLedger(t, w))
	local := machines["local"]
	if local == nil {
		t.Fatalf("local 一行必须在列")
	}
	arr, ok := local["resident"].([]interface{})
	if !ok || len(arr) != 1 {
		t.Fatalf("local 有驻留时 resident[] 应为 1 项，实得 %v", local["resident"])
	}
	entry := arr[0].(map[string]interface{})
	if entry["alias"] != "example-35b-v2" || entry["state"] != "ready" {
		t.Fatalf("驻留项 alias/state 不符: %v", entry)
	}
	if entry["managed"] != true || entry["source"] != "localback" {
		t.Fatalf("本机驻留项必须 managed=true 且 source=localback，实得 %v", entry)
	}
	if entry["mem_gb"] != float64(22) || entry["file"] != "/models/example-35b-v2.gguf" {
		t.Fatalf("驻留项 mem_gb/file 不符: %v", entry)
	}
	if local["vram_known"] != false {
		t.Fatalf("本机拿不到独立显存，vram_known 必须为 false，实得 %v", local["vram_known"])
	}
	// 远程行形状不变（回归：加 local 不得影响 x3）
	x3 := machines["x3"]
	if x3["vram_known"] != true || x3["vram_total_gb"] != float64(48) {
		t.Fatalf("x3 显存形状被改变: %v", x3)
	}
	if _, has := x3["resident"]; !has {
		t.Fatalf("x3 的 resident 不应消失: %v", x3)
	}
}

// 反例：实时来源不可得（provider 回放 nil）且 store 无 local 行 → 不得凭空造 local，也不 500。
func TestResourceLedger_NoLocalWhenSourceUnavailable(t *testing.T) {
	h := newTestHandlers()
	h.LocalSnapProvider = fakeLocalSnap{snap: nil}
	h.Store.ReceiveHeartbeat(store.HeartbeatRequest{Machine: "x3", MemTotalGb: 128, MemAvailableGb: 120})
	r := newResourcesTestRouter(h)

	w := doResReq(t, r, http.MethodGet, "/api/resources/ledger", "test-token", "")
	if w.Code != http.StatusOK {
		t.Fatalf("来源不可得也必须 200（不许 500），实得 %d body=%s", w.Code, w.Body.String())
	}
	m := machinesOf(t, decodeLedger(t, w))
	if row, has := m["local"]; has {
		t.Fatalf("拿不到本机实时快照且 store 无 local 时不得造 local 行，实得 %v", row)
	}
	if _, has := m["x3"]; !has {
		t.Fatalf("远程行应照常返回，实得 %v", m)
	}
}
