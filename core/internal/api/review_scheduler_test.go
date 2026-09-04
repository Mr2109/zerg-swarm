package api

// review_scheduler_test.go — 复查/恢复调度核心测试（2026-08-29 q5 覆盖补齐）
// 目标: recoverWaitingLocked(0%) / submitReviewTaskLocked(0%) / handleReviewDoneLocked(0%)

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestRecoverWaiting_ExpiredDowngrade 超上限(>30min) → 降级 failed
func TestRecoverWaiting_ExpiredDowngrade(t *testing.T) {
	s := NewMasterScheduler("", 1)
	task := &Task{
		ID:          "wait-test-1",
		Description: "测试",
		Status:      "waiting_retry",
		Model:       "Qwen3.8-27B",
		CompletedAt: time.Now().Add(-maxWaitRetry - time.Minute), // 超上限
	}
	s.waiting[task.ID] = task
	s.recoverWaitingLocked()
	// 超时 → 删除 waiting → 进 history 标 failed
	if _, ok := s.waiting[task.ID]; ok {
		t.Fatal("超时任务应从 waiting 删除")
	}
	h, ok := s.history[task.ID]
	if !ok || h.Status != "failed" {
		t.Fatalf("超时任务应 failed 进 history——实际 %v", h)
	}
	if !strings.Contains(h.FailReason, "自动放弃") {
		t.Fatalf("失败原因应含自动放弃: %s", h.FailReason)
	}
}

// TestRecoverWaiting_PingFailStaysWaiting ping 不通 → 继续 waiting（不降级不重派）
func TestRecoverWaiting_PingFailStaysWaiting(t *testing.T) {
	s := NewMasterScheduler("", 1)
	task := &Task{
		ID:          "wait-test-2",
		Description: "测试",
		Status:      "waiting_retry",
		Model:       "Qwen3.8-27B",
		CompletedAt: time.Now(),
	}
	s.waiting[task.ID] = task
	s.recoverWaitingLocked()
	// ping 不通（测试环境网关可能通——但任务模型无法验证）——任务应仍在 waiting 或已恢复
	// 关键断言: 不降级 failed（环境故障不杀任务）
	if h, ok := s.history[task.ID]; ok && h.Status == "failed" {
		t.Fatalf("环境故障不应降级 failed: %s", h.FailReason)
	}
	// 清理
	delete(s.waiting, task.ID)
}

// TestSubmitReviewTaskLocked 执行完成 → 派复查（跨家族模型——不同源）
func TestSubmitReviewTaskLocked(t *testing.T) {
	s := NewMasterScheduler("", 1)
	execTask := &Task{
		ID:          "exec-test-1",
		Description: "执行任务",
		Model:       "Qwen3.8-27B", // qwen 家族
		Workdir:     "/tmp/zerg-test-flow",
		Status:      "reviewing",
	}
	// 复查模型必须不同家族（qwen 执行 → 不能 qwen 复查）
	s.submitReviewTaskLocked(execTask, "/tmp/report.md", "")
	// 复查任务入队
	reviewID := "review-exec-test-1"
	found := false
	for _, q := range s.queue {
		if q.ID == reviewID {
			found = true
			if q.Model == execTask.Model {
				t.Fatalf("复查模型不能与执行同源: %s", q.Model)
			}
			if q.Type != "review" {
				t.Fatalf("复查任务 Type 应为 review: %s", q.Type)
			}
		}
	}
	if !found {
		t.Fatal("复查任务未入队")
	}
	// 执行任务状态应是 reviewing（复查通过才 done）
	if execTask.Status != "reviewing" {
		t.Fatalf("执行任务应 reviewing: %s", execTask.Status)
	}
}

// TestHandleReviewDone_Pass 复查通过 → 执行任务 done
func TestHandleReviewDone_Pass(t *testing.T) {
	s := NewMasterScheduler("", 1)
	execTask := &Task{
		ID:          "exec-test-2",
		Description: "执行任务",
		Model:       "Qwen3.8-27B",
		Status:      "reviewing",
	}
	s.history[execTask.ID] = execTask
	reviewTask := &Task{
		ID:        "review-exec-test-2",
		RefTaskID: execTask.ID,
		Model:     "gemma-4-26B",
		Workdir:   "/tmp/zerg-test-flow",
	}
	// 复查报告（通过——无打回关键词——≥100 字节过新守卫——2026-09-05）
	_ = os.MkdirAll("/tmp/zerg-test-flow", 0o755)
	_ = os.WriteFile(filepath.Join("/tmp/zerg-test-flow", "internal-task-report.md"), []byte("## 复查结论\n\n结论: 通过。\n\n理由: 报告内容与实际改动核对一致，真实性检查通过，完成度满足任务要求，未发现虚构或遗漏，质量符合预期。\n"), 0o644)
	s.handleReviewDoneLocked(reviewTask)
	if execTask.Status != "done" {
		t.Fatalf("复查通过后执行任务应 done: %s", execTask.Status)
	}
}

// TestHandleReviewDone_Rework 复查打回 → 派重做任务
func TestHandleReviewDone_Rework(t *testing.T) {
	s := NewMasterScheduler("", 1)
	execTask := &Task{
		ID:          "exec-test-3",
		Description: "执行任务",
		Model:       "Qwen3.8-27B",
		Status:      "reviewing",
	}
	s.history[execTask.ID] = execTask
	reviewTask := &Task{
		ID:        "review-exec-test-3",
		RefTaskID: execTask.ID,
		Model:     "gemma-4-26B",
		Workdir:   "/tmp/zerg-test-flow",
	}
	// 复查报告（打回——含"打回"关键词）
	_ = os.MkdirAll("/tmp/zerg-test-flow", 0o755)
	_ = os.WriteFile(filepath.Join("/tmp/zerg-test-flow", "internal-task-report.md"), []byte("打回——幻觉代码"), 0o644)
	s.handleReviewDoneLocked(reviewTask)
	// 应派重做任务（rework- 前缀）
	found := false
	for _, q := range s.queue {
		if strings.HasPrefix(q.ID, "rework-exec-test-3") {
			found = true
			if q.Model != execTask.Model {
				t.Fatalf("重做任务应保持执行模型: %s", q.Model)
			}
		}
	}
	if !found {
		t.Fatal("打回后应派重做任务")
	}
}
