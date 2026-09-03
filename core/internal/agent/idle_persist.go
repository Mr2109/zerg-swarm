package agent

// idle_persist.go — 内部任务断点续跑（2026-08-20 Mr2109）
// lastTriggered 落盘——重启后记住哪些类跑过——从暂停处继续（不从头重复）

import (
	"encoding/json"
	"log"
	"os"
	"time"
)

const idleStateFile = "/tmp/zerg-idle-state.json"

// saveIdleState 保存内部任务触发状态（调用方持锁）
func (d *IdleDetector) saveIdleState() {
	data := make(map[string]string, len(d.lastTriggered))
	for k, v := range d.lastTriggered {
		data[k] = v.Format(time.RFC3339)
	}
	buf, err := json.Marshal(data)
	if err != nil {
		return
	}
	_ = os.WriteFile(idleStateFile, buf, 0644)
}

// loadIdleState 加载内部任务触发状态（启动时调——断点续跑）
func (d *IdleDetector) loadIdleState() {
	buf, err := os.ReadFile(idleStateFile)
	if err != nil {
		return // 首次运行——无状态
	}
	var data map[string]string
	if err := json.Unmarshal(buf, &data); err != nil {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	for k, v := range data {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			d.lastTriggered[k] = t
		}
	}
	if len(data) > 0 {
		log.Printf("📜 内部任务断点恢复: %d 类已跑过（Mr2109——从暂停处继续）\n", len(data))
	}
}
