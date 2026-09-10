// chat_lifecycle.go — v2.5.8 对话数据生命周期（甲批 T3 2026-09-10 Mr2109拍板：两阶段清理）
// 两阶段：①软删（过期 → archived=1 + purge_after=now+宽限）②宽限期满 → 级联硬删（消息+FTS+图片）
// 纪律：幂等（可重复跑）+ 指数退避 + full jitter（防多会话/多进程同步重试）+ --dry-run（干跑）
// 说明：旧版是一阶段直删（DeleteOldSessions）——升级为两阶段以留出"误删可挽回"窗口。

package chat

import (
	"log"
	"math/rand"
	"os"
	"sync"
	"time"
)

// retentionDays — 对话保留天数（90 天——Mr2109）
const retentionDays = 90

// graceDays — 软删后宽限天数（甲批 T3 默认 7 天——窗口内可人工恢复，到期才级联硬删）
const graceDays = 7

// Lifecycle — 对话生命周期管理（两阶段清理定时）
type Lifecycle struct {
	store  *ChatStore
	stopCh chan struct{}
	wg     sync.WaitGroup
}

// NewLifecycle — 创建生命周期管理
func NewLifecycle(store *ChatStore) *Lifecycle {
	return &Lifecycle{store: store, stopCh: make(chan struct{})}
}

// Start — 启动清理循环（立即扫一次 + 每天一次）
func (l *Lifecycle) Start() {
	l.wg.Add(1)
	go func() {
		defer l.wg.Done()
		l.cleanupOnce() // 启动即扫
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-l.stopCh:
				return
			case <-ticker.C:
				l.cleanupOnce()
			}
		}
	}()
}

// Stop — 停止清理循环
func (l *Lifecycle) Stop() {
	close(l.stopCh)
	l.wg.Wait()
}

// withRetryJitter — 重试封装：最多 3 次，指数退避（1s→2s→4s 基准）× full jitter(0.5–1.0)
// 为什么：多进程/多定时器若同步失败会一起重试打爆后端；jitter 打散（AWS 退避实践）。
func withRetryJitter(label string, fn func() (int, error)) (int, error) {
	var (
		n   int
		err error
	)
	base := time.Second
	for attempt := 0; attempt < 3; attempt++ {
		n, err = fn()
		if err == nil {
			return n, nil
		}
		if attempt == 2 {
			break
		}
		delay := time.Duration(float64(base<<attempt) * (0.5 + rand.Float64()*0.5))
		log.Printf("[chat] %s 第 %d 次失败（%v）——%v 后重试", label, attempt+1, err, delay.Round(time.Millisecond))
		time.Sleep(delay)
	}
	return n, err
}

// cleanupOnce — 执行一次两阶段清理（幂等）
func (l *Lifecycle) cleanupOnce() {
	now := float64(time.Now().Unix())
	cutoff := float64(time.Now().AddDate(0, 0, -retentionDays).Unix())
	purgeAfter := float64(time.Now().AddDate(0, 0, graceDays).Unix())

	// 干跑：只统计不动数据
	if os.Getenv("ZERG_CHAT_CLEANUP_DRY_RUN") == "1" {
		soft, purge, err := l.store.CountCleanupPlan(cutoff, now)
		if err != nil {
			log.Printf("[chat] 清理干跑统计失败: %v", err)
			return
		}
		log.Printf("[chat] 清理干跑（dry-run）：将软删 %d 个（%d 天未活动），将级联硬删 %d 个（宽限期满）", soft, retentionDays, purge)
		return
	}

	// 阶段1：软删（过期 → 归档 + 排入清理队列）
	soft, err := withRetryJitter("清理阶段1（软删）", func() (int, error) {
		return l.store.SoftDeleteExpired(cutoff, purgeAfter)
	})
	if err != nil {
		log.Printf("[chat] 清理阶段1 最终失败（下轮重试）: %v", err)
	}
	if soft > 0 {
		log.Printf("[chat] 阶段1 软删 %d 个过期会话（%d 天未活动；%d 天宽限后级联硬删）", soft, retentionDays, graceDays)
	}

	// 阶段2：级联硬删（宽限期满）
	purged, err := withRetryJitter("清理阶段2（硬删）", func() (int, error) {
		return l.store.PurgeSoftDeleted(now)
	})
	if err != nil {
		log.Printf("[chat] 清理阶段2 最终失败（下轮重试）: %v", err)
	}
	if purged > 0 {
		log.Printf("[chat] 阶段2 级联硬删 %d 个会话（含消息/FTS/图片）", purged)
	}
}
