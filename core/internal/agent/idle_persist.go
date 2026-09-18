package agent

// idle_persist.go — 内部任务断点续跑（2026-08-20 Mr2109）
// lastTriggered 落盘——重启后记住哪些类跑过——从暂停处继续（不从头重复）
//
// 2026-09-18 修（/tmp 硬编码——口径同 api/tasks_persist.go）:
//   原写死 `const idleStateFile = "/tmp/zerg-idle-state.json"` —— macOS 重启 /tmp 即清 +
//   tmp_cleaner 3 天未访问即删 ⇒ 断点状态丢（重启从头重复）；多实例还共用同一份文件。
//   改为 statepath 统一状态目录派生（ZERG_STATE_DIR → ~/.zerg/state/zerg-idle-state.json）。
//   迁移兼容（首次）: 新路径不存在而旧 /tmp 存在 ⇒ **读旧一次**（不丢断点）；
//   **写只写新路径**；旧文件**不删、不改**。新路径已存在 ⇒ 旧路径完全不看。
//   显式覆盖（测试隔离）⇒ 不走旧回退（防测试读真机 /tmp）。

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
)

// idleStateFileName 断点状态文件在统一状态目录下的文件名。
const idleStateFileName = "zerg-idle-state.json"

// idleLegacyDefaultPath 旧硬编码落点字面量（2026-08-20 起写死）。
// 只作**首次迁移读取**来源：读一次；不删、不改、永不写入。
const idleLegacyDefaultPath = "/tmp/zerg-idle-state.json"

// legacyIdleStateFile 旧路径（包级变量 = 上面的字面量；迁移用例/测试进程隔离可切换）。
var legacyIdleStateFile = idleLegacyDefaultPath

// idleStateFile 断点状态文件（包级变量——测试可切换隔离路径）。
// 语义: 非空 ⇒ 直接用该路径（测试隔离/显式覆盖）；
// 空 ⇒ 走 statepath 统一状态目录派生（生产默认，见 idleWritePath）。
var idleStateFile = ""

// idleWritePath 写路径：永远是统一状态目录（ZERG_STATE_DIR → ~/.zerg/state/<idleStateFileName>）。
// 永不写旧 /tmp 路径——多实例各写各的。
func idleWritePath() string {
	if p := strings.TrimSpace(idleStateFile); p != "" {
		return p
	}
	return statepath.File(idleStateFileName)
}

// idleReadPath 读路径：统一状态目录优先；新路径不存在且旧 /tmp 存在 ⇒ 读旧一次（迁移兼容）。
// 新路径存在 ⇒ 旧路径完全不看（不 stat、不读）。
// 显式覆盖 idleStateFile（测试隔离）时不退旧路径：覆盖即「我已指定唯一来源」。
func idleReadPath() string {
	p := idleWritePath()
	if _, err := os.Stat(p); err == nil {
		return p // 新路径已存在——旧路径完全不看
	}
	if strings.TrimSpace(idleStateFile) != "" {
		return p
	}
	legacy := strings.TrimSpace(legacyIdleStateFile)
	if legacy == "" {
		return p
	}
	if _, err := os.Stat(legacy); err != nil {
		return p // 无旧文件——首次运行（无断点）
	}
	log.Printf("📜 内部任务断点首次迁移: 读旧 %s（只读一次——旧文件保留不删；写入只落 %s）\n", legacy, p)
	return legacy
}

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
	// 写只写新路径（统一状态目录）——旧 /tmp 路径永不写（2026-09-18）
	dst := idleWritePath()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		log.Printf("⚠️ 内部任务断点目录不可用（%s）: %v\n", dst, err)
		return
	}
	if err := os.WriteFile(dst, buf, 0644); err != nil {
		log.Printf("⚠️ 内部任务断点写盘失败（%s）: %v\n", dst, err)
	}
}

// loadIdleState 加载内部任务触发状态（启动时调——断点续跑）
func (d *IdleDetector) loadIdleState() {
	// 读路径：新（统一状态目录）优先；新缺失 + 旧 /tmp 在 ⇒ 读旧一次（2026-09-18 迁移兼容）
	buf, err := os.ReadFile(idleReadPath())
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
