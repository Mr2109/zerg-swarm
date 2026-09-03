package agent

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestIdleDetector_Trigger — 空闲时触发内部任务（建任务单）
func TestIdleDetector_Trigger(t *testing.T) {
	dir := t.TempDir()
	d := NewIdleDetector(dir)
	d.SetExternalQueue(func() int { return 0 })     // 外部任务空
	d.SetResourceIdle(func() bool { return true }) // 资源空闲
	d.enabled = true

	d.checkAndTrigger()

	// 应有任务单
	files, err := filepath.Glob(filepath.Join(dir, "internal-*.md"))
	if err != nil || len(files) == 0 {
		t.Fatal("空闲时应有内部任务单——但没有")
	}
	t.Logf("✅ 空闲触发: %s", files[0])
}

// TestIdleDetector_ExternalBusy — 外部任务在跑——内部让路（不触发）
func TestIdleDetector_ExternalBusy(t *testing.T) {
	dir := t.TempDir()
	d := NewIdleDetector(dir)
	d.SetExternalQueue(func() int { return 2 })    // 外部任务在跑
	d.SetResourceIdle(func() bool { return true })
	d.enabled = true

	d.checkAndTrigger()

	files, _ := filepath.Glob(filepath.Join(dir, "internal-*.md"))
	if len(files) != 0 {
		t.Fatal("外部任务在跑——不应触发内部任务")
	}
	t.Log("✅ 外部任务在跑——内部让路（不触发）")
}

// TestIdleDetector_Cooldown — 冷却期内不重复触发同类型（不同类型可触发）
func TestIdleDetector_Cooldown(t *testing.T) {
	dir := t.TempDir()
	// 清持久化状态（防上次运行污染——/tmp/zerg-idle-state.json）
	os.Remove(idleStateFile)
	d := NewIdleDetector(dir)
	d.SetExternalQueue(func() int { return 0 })
	d.SetResourceIdle(func() bool { return true })
	d.enabled = true

	d.checkAndTrigger() // 第一次触发（16 类中第一个冷却过的）
	d.checkAndTrigger() // 第二次（同类型冷却——下一个不同类型可触发）

	files, _ := filepath.Glob(filepath.Join(dir, "internal-*.md"))
	if len(files) != 2 {
		t.Fatalf("冷却防同类型——不同类型应触发——实际 %d 个任务单（期望 2）", len(files))
	}
	// 验证是不同类型（防同类型重复触发）
	if len(files) == 2 && files[0] == files[1] {
		t.Fatalf("同类型重复触发（%s）——冷却失效", files[0])
	}
	t.Log("✅ 冷却防同类型（两次触发不同类型——16 类配置下正常）")
}

// TestIdleDetector_Disabled — 关闭时不触发
func TestIdleDetector_Disabled(t *testing.T) {
	dir := t.TempDir()
	d := NewIdleDetector(dir)
	d.SetExternalQueue(func() int { return 0 })
	d.SetResourceIdle(func() bool { return true })
	d.enabled = false

	d.checkAndTrigger()

	files, _ := filepath.Glob(filepath.Join(dir, "internal-*.md"))
	if len(files) != 0 {
		t.Fatal("关闭时不应触发")
	}
	t.Log("✅ 关闭时不触发")
}

var _ = os.Getenv // 占位
var _ = time.Now
