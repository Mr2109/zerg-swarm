package api

// resources_local_ledger_test.go —— 批 5：local 一行如实 + 显存取严在观测面的反例优先测试。
//
// 覆盖 #29（消费方不误判）、#30（local 账本有/无驻留都如实、vram 未知 vram_known=false）、
// 以及"显存未知 fail-closed / 统一内存不 fail-closed"在 /api/resources/ledger 的可见性。

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
