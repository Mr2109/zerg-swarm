package store

// store_test.go — 内存存储核心测试（2026-08-29 q5 覆盖补齐）
// 目标: 心跳更新/快照读写/任务管理（0% 包首测）

import (
	"testing"
	"time"
)

func modelPtr(s string) *string { return &s }

// TestReceiveHeartbeat_New 新机器心跳 → 创建快照
func TestReceiveHeartbeat_New(t *testing.T) {
	s := NewStore()
	m := "x3"
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
	s.ReceiveHeartbeat(HeartbeatRequest{Machine: "x3", Healthy: true})
	// 更新——换模型 + 不健康
	s.ReceiveHeartbeat(HeartbeatRequest{
		Machine: "x3",
		Model:   modelPtr("gemma-4-26B"),
		Healthy: false,
		CpuPct:  42.5,
		GpuPct:  88.0,
	})
	snap := s.GetSnapshot("x3")
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
	s.SetLocalSnapshot("local", modelPtr("Qwen3.8-27B"), []string{"Qwen3.8-27B"}, true, "ready", 13.9, 64, 1.79, 0, 9.4, 70)
	snap := s.GetSnapshot("local")
	if snap == nil || *snap.Model != "Qwen3.8-27B" {
		t.Fatalf("local 快照未写入: %v", snap)
	}
	if snap.BackendState != "ready" || snap.MemTotalGb != 64 {
		t.Fatalf("状态/内存未保存: %s/%v", snap.BackendState, snap.MemTotalGb)
	}
}

// TestGetAllModels 多快照模型去重
func TestGetAllModels(t *testing.T) {
	s := NewStore()
	s.ReceiveHeartbeat(HeartbeatRequest{Machine: "x3", Models: []string{"Qwen3.8-27B", "gemma-4-26B"}})
	s.ReceiveHeartbeat(HeartbeatRequest{Machine: "local", Models: []string{"Qwen3.8-27B"}})
	models := s.GetAllModels()
	if len(models) != 2 {
		t.Fatalf("应 2 个去重模型: %v", models)
	}
}

// TestCreateTask 创建任务（ID 递增）
func TestCreateTask(t *testing.T) {
	s := NewStore()
	t1 := s.CreateTask("Qwen3.8-27B", "x3")
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
	s.ReceiveHeartbeat(HeartbeatRequest{Machine: "x3"})
	s.ReceiveHeartbeat(HeartbeatRequest{Machine: "local"})
	s.ReceiveHeartbeat(HeartbeatRequest{Machine: "x3"}) // 重复——不增加
	if s.MachineCount() != 2 {
		t.Fatalf("应 2 机器: %d", s.MachineCount())
	}
}

// TestLastSeen 心跳更新 LastSeen
func TestLastSeen(t *testing.T) {
	s := NewStore()
	s.ReceiveHeartbeat(HeartbeatRequest{Machine: "x3"})
	before := s.GetSnapshot("x3").LastSeen
	time.Sleep(5 * time.Millisecond)
	s.ReceiveHeartbeat(HeartbeatRequest{Machine: "x3"})
	after := s.GetSnapshot("x3").LastSeen
	if !after.After(before) {
		t.Fatal("LastSeen 应更新")
	}
}
