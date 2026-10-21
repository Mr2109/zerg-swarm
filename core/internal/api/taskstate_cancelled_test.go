package api

// taskstate_cancelled_test.go — 拍板增量（2026-10-21）: **cancelled 不写 completed_at**。
//
// 口径: 「已撤」≠「已完成」—— 退场时刻不冒充完成时刻（UI 的耗时/完成率读 completed_at）。
// 这条与 I1 同源（时间字段只由 applyStatus 一处决定），但**只能打补丁后跑**:
// 原码没有 applyStatus ⇒ 本件在补丁前不属于「红」，而是「编译不过」——
// 故它不在先红那一趟的 overlay 里，只在补丁后的绿那一趟出现。
//
// 反向守卫（别把整条终态分支改坏）: done 仍必须有完成时刻；cancelled 的**残留值**也必须被清掉。

import (
	"testing"
	"time"
)

func TestCancelledCarriesNoCompletionStamp(t *testing.T) {
	now := time.Now()

	// ① 排队件被撤: 不许长出完成时刻
	queued := &Task{ID: "inv-cancel-1", Status: StatusQueued, Model: "m", CreatedAt: now.Add(-time.Hour)}
	queued.applyStatus(StatusCancelled, now)
	if queued.Status != StatusCancelled {
		t.Fatalf("applyStatus 没改态: %s", queued.Status)
	}
	if !queued.CompletedAt.IsZero() {
		t.Errorf("cancelled 破: 撤件被写了 completed_at=%s（拍板: 已撤不写完成时刻）",
			queued.CompletedAt.Format(time.RFC3339))
	}
	if queued.WaitingSince != nil {
		t.Errorf("cancelled 破: 撤件带着 waiting_since（等待窗计时源不该留给终态件）")
	}

	// ② 残留值必须清掉（旧实现/旧存量可能已经写过）
	stale := &Task{ID: "inv-cancel-2", Status: StatusDone, Model: "m", CreatedAt: now.Add(-2 * time.Hour), CompletedAt: now.Add(-time.Hour)}
	stale.applyStatus(StatusCancelled, now)
	if !stale.CompletedAt.IsZero() {
		t.Errorf("cancelled 破: 残留 completed_at=%s 没被清掉（不变式读不出来）",
			stale.CompletedAt.Format(time.RFC3339))
	}

	// ③ 反向: done 仍必须拿到完成时刻（别把终态分支一刀切掉）
	done := &Task{ID: "inv-done-cancel-guard", Status: StatusRunning, Model: "m", CreatedAt: now.Add(-time.Hour)}
	done.applyStatus(StatusDone, now)
	if !done.CompletedAt.Equal(now) {
		t.Errorf("I1 反破: done 件没拿到完成时刻（want %s, got %s）",
			now.Format(time.RFC3339), done.CompletedAt.Format(time.RFC3339))
	}
}
