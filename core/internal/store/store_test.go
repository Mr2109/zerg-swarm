package store

// store_test.go — 内存存储核心测试（2026-08-29 q5 覆盖补齐）
// 目标: 心跳更新/快照读写/任务管理（0% 包首测）

import (
	"testing"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/resources"
)

func modelPtr(s string) *string { return &s }

// TestReceiveHeartbeat_New 新机器心跳 → 创建快照
func TestReceiveHeartbeat_New(t *testing.T) {
	s := NewStore()
	m := "node-a"
	resp := s.ReceiveHeartbeat(HeartbeatRequest{
		Machine: m,
		Model:   modelPtr("Qwen3.8-27B"),
		Healthy: true,
	})
	_ = resp
	if !resp.OK {
		t.Fatal("心跳应 OK")
	}
	snap := s.GetSnapshot(m)
	if snap == nil {
		t.Fatal("新机器应创建快照")
	}
	if snap.Model == nil || *snap.Model != "Qwen3.8-27B" {
		t.Fatalf("Model 未保存: %v", snap.Model)
	}
	if !snap.Healthy {
		t.Fatal("Healthy 未保存")
	}
}

// TestReceiveHeartbeat_Update 已有机器心跳 → 更新快照
func TestReceiveHeartbeat_Update(t *testing.T) {
	s := NewStore()
	s.ReceiveHeartbeat(HeartbeatRequest{Machine: "node-a", Healthy: true})
	// 更新——换模型 + 不健康
	s.ReceiveHeartbeat(HeartbeatRequest{
		Machine: "node-a",
		Model:   modelPtr("gemma-4-26B"),
		Healthy: false,
		CpuPct:  42.5,
		GpuPct:  88.0,
	})
	snap := s.GetSnapshot("node-a")
	if snap == nil || *snap.Model != "gemma-4-26B" {
		t.Fatalf("模型未更新: %v", snap)
	}
	if snap.Healthy {
		t.Fatal("健康状态未更新")
	}
	if snap.CpuPct != 42.5 || snap.GpuPct != 88.0 {
		t.Fatalf("CPU/GPU 未更新: %.1f/%.1f", snap.CpuPct, snap.GpuPct)
	}
}

// TestSetLocalSnapshot 本机快照写入
func TestSetLocalSnapshot(t *testing.T) {
	s := NewStore()
	s.SetLocalSnapshot(LocalSnapshotData{
		Machine: "local", Model: modelPtr("Qwen3.8-27B"), Models: []string{"Qwen3.8-27B"},
		Healthy: true, State: "ready", MemAvailableGb: 13.9, MemTotalGb: 64, Load: 1.79,
		CpuPct: 9.4, GpuPct: 70,
	})
	snap := s.GetSnapshot("local")
	if snap == nil || *snap.Model != "Qwen3.8-27B" {
		t.Fatalf("local 快照未写入: %v", snap)
	}
	if snap.BackendState != "ready" || snap.MemTotalGb != 64 {
		t.Fatalf("状态/内存未保存: %s/%v", snap.BackendState, snap.MemTotalGb)
	}
	// 未提供驻留/显存 → 如实缺席（不造值）
	if len(snap.Resident) != 0 {
		t.Fatalf("未提供驻留时 resident 应为空，实得 %v", snap.Resident)
	}
	if snap.VramKnown || snap.VramUnified {
		t.Fatalf("未提供显存时 vram_known/vram_unified 应为 false，实得 %v/%v", snap.VramKnown, snap.VramUnified)
	}
}

// TestSetLocalSnapshot_CarriesResidentAndVram —— 批 5（#30）：本机账本字段随快照写入
// （驻留明细 + 显存三态），使 /api/resources/ledger 的 local 一行与远程同口径（§八 Q7）。
func TestSetLocalSnapshot_CarriesResidentAndVram(t *testing.T) {
	s := NewStore()
	s.SetLocalSnapshot(LocalSnapshotData{
		Machine: "local", Model: modelPtr("example-35b-v2"), Models: []string{"example-35b-v2"},
		Healthy: true, State: "ready", MemAvailableGb: 20, MemTotalGb: 64,
		Resident: []resources.ResidentEntry{{
			Alias: "example-35b-v2", File: "/models/example-35b-v2.gguf",
			State: resources.StateReady, MemGb: 22, Managed: true, Source: "localback",
		}},
		VramUnified: true, // 本机 Apple Silicon：显存即内存
	})
	snap := s.GetSnapshot("local")
	if len(snap.Resident) != 1 {
		t.Fatalf("驻留明细应写入 1 项，实得 %v", snap.Resident)
	}
	r := snap.Resident[0]
	if r.Alias != "example-35b-v2" || r.File != "/models/example-35b-v2.gguf" || r.State != resources.StateReady ||
		r.MemGb != 22 || !r.Managed || r.Source != "localback" {
		t.Fatalf("驻留项字段不符: %+v", r)
	}
	if !snap.VramUnified || snap.VramKnown {
		t.Fatalf("统一内存应 vram_unified=true 且 vram_known=false，实得 %v/%v", snap.VramUnified, snap.VramKnown)
	}
}

// TestGetAllModels 多快照模型去重
func TestGetAllModels(t *testing.T) {
	s := NewStore()
	s.ReceiveHeartbeat(HeartbeatRequest{Machine: "node-a", Models: []string{"Qwen3.8-27B", "gemma-4-26B"}})
	s.ReceiveHeartbeat(HeartbeatRequest{Machine: "local", Models: []string{"Qwen3.8-27B"}})
	models := s.GetAllModels()
	if len(models) != 2 {
		t.Fatalf("应 2 个去重模型: %v", models)
	}
}

// TestCreateTask 创建任务（ID 递增）
func TestCreateTask(t *testing.T) {
	s := NewStore()
	t1 := s.CreateTask("Qwen3.8-27B", "node-a")
	t2 := s.CreateTask("gemma-4-26B", "local")
	if t1.ID == t2.ID {
		t.Fatalf("任务 ID 应递增: %s/%s", t1.ID, t2.ID)
	}
	if t1.Status != "pending" || t2.Machine != "local" {
		t.Fatalf("任务字段错误: %+v %+v", t1, t2)
	}
	if len(s.GetTasks()) != 2 {
		t.Fatalf("应有 2 任务: %d", len(s.GetTasks()))
	}
}

// TestMachineCount 机器计数
func TestMachineCount(t *testing.T) {
	s := NewStore()
	if s.MachineCount() != 0 {
		t.Fatal("初始应为 0")
	}
	s.ReceiveHeartbeat(HeartbeatRequest{Machine: "node-a"})
	s.ReceiveHeartbeat(HeartbeatRequest{Machine: "local"})
	s.ReceiveHeartbeat(HeartbeatRequest{Machine: "node-a"}) // 重复——不增加
	if s.MachineCount() != 2 {
		t.Fatalf("应 2 机器: %d", s.MachineCount())
	}
}

// TestLastSeen 心跳更新 LastSeen
func TestLastSeen(t *testing.T) {
	s := NewStore()
	s.ReceiveHeartbeat(HeartbeatRequest{Machine: "node-a"})
	before := s.GetSnapshot("node-a").LastSeen
	time.Sleep(5 * time.Millisecond)
	s.ReceiveHeartbeat(HeartbeatRequest{Machine: "node-a"})
	after := s.GetSnapshot("node-a").LastSeen
	if !after.After(before) {
		t.Fatal("LastSeen 应更新")
	}
}

// TestForget GAP-20260925-29：名册删除面（幂等 + 只删点名那条）。
func TestForget(t *testing.T) {
	s := NewStore()
	s.ReceiveHeartbeat(HeartbeatRequest{Machine: "node-a"})
	s.ReceiveHeartbeat(HeartbeatRequest{Machine: "probe"})
	if !s.Forget("probe") {
		t.Fatal("在册条目应删成功（返回 true）")
	}
	if s.GetSnapshot("probe") != nil {
		t.Fatal("Forget 后该条目应消失")
	}
	if s.GetSnapshot("node-a") == nil {
		t.Fatal("Forget 不得波及其他机器")
	}
	if s.Forget("probe") {
		t.Fatal("不在册条目再删应返回 false（幂等，不是错误）")
	}
	if s.MachineCount() != 1 {
		t.Fatalf("删除后机器数应为 1，实得 %d", s.MachineCount())
	}
}

// TestStaleMachines GAP-20260925-29：过期判据（只报告不删 · 按名排序）。
func TestStaleMachines(t *testing.T) {
	s := NewStore()
	s.ReceiveHeartbeat(HeartbeatRequest{Machine: "fresh"})
	s.ReceiveHeartbeat(HeartbeatRequest{Machine: "old"})
	// 把 old 的最后心跳人为推旧
	s.mu.Lock()
	s.snapshots["old"].LastSeen = time.Now().Add(-time.Hour)
	s.mu.Unlock()

	ttl := 10 * time.Minute
	now := time.Now()
	stale := s.StaleMachines(ttl, now)
	if len(stale) != 1 || stale[0] != "old" {
		t.Fatalf("应只报 old 一台，实得 %v", stale)
	}
	// 只报告不删：快照仍在
	if s.GetSnapshot("old") == nil {
		t.Fatal("StaleMachines 不得删除条目（只报告）")
	}
	// 排序稳定：两次读逐字相同
	if got := s.StaleMachines(ttl, now); len(got) != 1 || got[0] != "old" {
		t.Fatalf("排序应稳定，实得 %v", got)
	}
	// ttl 放大到覆盖所有 ⇒ 空
	if got := s.StaleMachines(2*time.Hour, now); len(got) != 0 {
		t.Fatalf("ttl 覆盖全部时应为空，实得 %v", got)
	}
}
