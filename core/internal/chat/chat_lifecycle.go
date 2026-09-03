// chat_lifecycle.go — v2.5.7 对话数据生命周期（90 天硬删——Mr2109）
// 启动时扫描 + 定时清理——对话是临时交流——知识沉淀走知识库

package chat

import (
	"log"
	"sync"
	"time"
)

// retentionDays — 对话保留天数（90 天硬删——Mr2109）
const retentionDays = 90

// Lifecycle — 对话生命周期管理（90 天硬删定时）
type Lifecycle struct {
	store  *ChatStore
	stopCh chan struct{}
	wg     sync.WaitGroup
}

// NewLifecycle — 创建生命周期管理
func NewLifecycle(store *ChatStore) *Lifecycle {
	return &Lifecycle{store: store, stopCh: make(chan struct{})}
}

// Start — 启动清理循环（立即扫一次 + 每天 3:00 清理）
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

// cleanupOnce — 执行一次 90 天硬删
func (l *Lifecycle) cleanupOnce() {
	cutoff := float64(time.Now().AddDate(0, 0, -retentionDays).Unix())
	n, err := l.store.DeleteOldSessions(cutoff)
	if err != nil {
		log.Printf("[chat] 90天清理失败: %v", err)
		return
	}
	if n > 0 {
		log.Printf("[chat] 90天硬删 %d 个过期会话", n)
	}
}
