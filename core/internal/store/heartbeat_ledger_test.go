package store

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/resources"
)

// TestHeartbeat_DecodeLedgerFields —— 子端新心跳体（含显存/驻留/未托管）能落库。
func TestHeartbeat_DecodeLedgerFields(t *testing.T) {
	// 形状对齐 agent 侧 heartbeat.buildBody 的真实输出
	payload := []byte(`{
		"machine":"x3","backend_state":"ready","model":"example-35b",
		"mem_available_gb":40.5,"mem_total_gb":128,"backend_rss_gb":22.3,
		"active_requests":3,"healthy":true,
		"vram_known":true,"gpu_used_gb":20.4,"vram_used_gb":20.4,"vram_total_gb":24,"vram_free_gb":3.6,
		"resident":[
			{"alias":"example-35b","file":"/data/models/ornith.gguf","state":"ready","last_used_ago_s":3.2,"req_count":2,"rss_gb":22.3,"managed":true,"mem_gb":22,"ctx_window":262144,"source":"managed"},
			{"alias":"unmanaged@127.0.0.1:9001","state":"ready","managed":false,"source":"manual"}
		],
		"unmanaged":[{"port":9001,"managed":false,"note":"listening on 127.0.0.1, not managed by agent (managed=false); read-only report, never taken over"}],
		"future_unknown_field":{"x":1}
	}`)

	var req HeartbeatRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		t.Fatalf("新心跳体解析失败: %v", err)
	}
	if req.VramKnown != true || req.VramUsedGb != 20.4 || req.VramTotalGb != 24 || req.VramFreeGb != 3.6 {
		t.Fatalf("显存字段未解出: %+v", req)
	}
	if req.ActiveRequests != 3 {
		t.Fatalf("active_requests 应为真值 3，实得 %d", req.ActiveRequests)
	}
	if len(req.Resident) != 2 {
		t.Fatalf("resident 应有 2 项，实得 %d", len(req.Resident))
	}
	var managedN, unmanagedN int
	for _, r := range req.Resident {
		if r.Managed {
			managedN++
			if r.CtxWindow != 262144 || r.ReqCount != 2 {
				t.Fatalf("托管项明细不全: %+v", r)
			}
		} else {
			unmanagedN++
		}
	}
	if managedN != 1 || unmanagedN != 1 {
		t.Fatalf("managed 区分错: managed=%d unmanaged=%d", managedN, unmanagedN)
	}
	if len(req.Unmanaged) != 1 || req.Unmanaged[0].Port != 9001 || req.Unmanaged[0].Managed {
		t.Fatalf("unmanaged 未如实解出（managed 必须 false）: %+v", req.Unmanaged)
	}

	// 落库
	s := NewStore()
	s.ReceiveHeartbeat(req)
	snap := s.GetSnapshot("x3")
	if snap == nil {
		t.Fatal("快照未创建")
	}
	if !snap.VramKnown || snap.VramUsedGb != 20.4 || snap.VramTotalGb != 24 {
		t.Fatalf("快照未带显存: %+v", snap)
	}
	if len(snap.Resident) != 2 || len(snap.Unmanaged) != 1 {
		t.Fatalf("快照未带驻留/未托管明细: %+v", snap)
	}
	if snap.Unmanaged[0].Managed {
		t.Fatalf("unmanaged 项 managed 必须 false: %+v", snap.Unmanaged[0])
	}
}

// TestHeartbeat_UnmanagedResidentVisible —— Q6：未托管项 managed=false，可见且立即可查。
func TestHeartbeat_UnmanagedResidentVisible(t *testing.T) {
	s := NewStore()
	s.ReceiveHeartbeat(HeartbeatRequest{
		Machine: "x3",
		Resident: []resources.ResidentEntry{
			{Digest: "d-managed", State: "ready", Managed: true},
			{Digest: "d-manual", State: "ready", Managed: false, Source: "manual"},
		},
		Unmanaged: []resources.UnmanagedProcess{
			{Port: 9001, Managed: false, Note: "手工 screen 起，只绑 127.0.0.1"},
		},
	})
	snap := s.GetSnapshot("x3")
	if len(snap.Resident) != 2 {
		t.Fatalf("应有 2 项驻留，实得 %d", len(snap.Resident))
	}
	umCount := 0
	for _, r := range snap.Resident {
		if !r.Managed {
			umCount++
		}
	}
	if umCount != 1 {
		t.Fatalf("应有 1 项未托管驻留，实得 %d", umCount)
	}
	if len(snap.Unmanaged) != 1 || snap.Unmanaged[0].Managed {
		t.Fatalf("未托管进程清单必须 managed=false: %+v", snap.Unmanaged)
	}
}

// TestHeartbeat_BackwardCompat_OldPayload —— 反例：旧子端心跳体（无任何新增字段）照常落库，旧字段语义不变。
func TestHeartbeat_BackwardCompat_OldPayload(t *testing.T) {
	old := []byte(`{"machine":"x3","model":"gemma-4-26B","mem_available_gb":40.0,"mem_total_gb":128,"gpu_used_gb":12.5,"backend_rss_gb":12.5,"active_requests":0,"healthy":true,"backend_state":"ready"}`)

	var req HeartbeatRequest
	if err := json.Unmarshal(old, &req); err != nil {
		t.Fatalf("旧心跳体解析失败: %v", err)
	}
	s := NewStore()
	s.ReceiveHeartbeat(req)
	snap := s.GetSnapshot("x3")
	if snap == nil || snap.GpuUsedGb != 12.5 || snap.BackendRssGb != 12.5 || !snap.Healthy {
		t.Fatalf("旧字段语义被破坏: %+v", snap)
	}
	// 新增字段缺席 → 零值（未知），不得被误当"有显存"
	if snap.VramKnown {
		t.Fatal("旧心跳体无 vram_known，应为 false（未知）")
	}
	if len(snap.Resident) != 0 || len(snap.Unmanaged) != 0 {
		t.Fatalf("旧心跳体不应凭空多出驻留明细: %+v", snap)
	}
}

// TestHeartbeat_VramUnknownZeroIsHonest —— 子端拿不到显存时（无 gpu_used_gb / vram_known=false），主控如实视为未知。
func TestHeartbeat_VramUnknownZeroIsHonest(t *testing.T) {
	// agent 在拿不到显存时不会发 gpu_used_gb
	payload := []byte(`{"machine":"local","backend_rss_gb":22.3,"vram_known":false,"active_requests":1,"healthy":true}`)
	var req HeartbeatRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if req.VramKnown {
		t.Fatal("vram_known 应为 false")
	}
	// gpu_used_gb 缺席 → 0；关键：它**不等于** RSS（不冒充）
	if req.GpuUsedGb != 0 {
		t.Fatalf("显存未知时 gpu_used_gb 应为 0（缺席），实得 %v", req.GpuUsedGb)
	}
	if req.GpuUsedGb == req.BackendRssGb {
		t.Fatalf("gpu_used_gb 不得等同 RSS: gpu=%v rss=%v", req.GpuUsedGb, req.BackendRssGb)
	}
}

// TestHeartbeat_JSONShape_ManagedFalseVisible —— 账本对外 JSON 里未托管项必须字面出现 "managed":false。
func TestHeartbeat_JSONShape_ManagedFalseVisible(t *testing.T) {
	snap := FleetSnapshot{
		Machine: "x3",
		Resident: []resources.ResidentEntry{
			{Digest: "d-managed", State: "ready", Managed: true, Source: "managed"},
			{Digest: "d-manual", State: "ready", Managed: false, Source: "manual"},
		},
		Unmanaged: []resources.UnmanagedProcess{{Port: 9001, Note: "手工 screen 起，只绑 127.0.0.1"}},
	}
	raw, err := json.Marshal(snap)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	got := string(raw)
	if !strings.Contains(got, `"managed":false`) {
		t.Fatalf("未托管项必须字面出现 \"managed\":false，实得: %s", got)
	}
	if !strings.Contains(got, `"managed":true`) {
		t.Fatalf("托管项必须字面出现 \"managed\":true，实得: %s", got)
	}
}
