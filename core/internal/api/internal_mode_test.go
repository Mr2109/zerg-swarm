package api

// internal_mode_test.go — 内部任务运行模式开关测试（2026-08-28 Mr2109）
// 验证: 默认自动 / 切换手动 / 持久化 / 恢复

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInternalModeDefaultAuto(t *testing.T) {
	// 未设置 = 默认自动运行
	if !IsInternalAuto("resource-grow") {
		t.Fatal("未设置的内部任务应默认自动运行")
	}
}

func TestInternalModeSwitchToManual(t *testing.T) {
	// 切到临时文件（测试隔离——不影响真实 modes）
	oldFile := modeFile
	modeFile = filepath.Join(t.TempDir(), "modes_test.json")
	defer func() { modeFile = oldFile }()

	SetInternalMode("resource-grow", false)
	if IsInternalAuto("resource-grow") {
		t.Fatal("切换到手动后 IsInternalAuto 应为 false")
	}
	// 其他任务不受影响
	if !IsInternalAuto("health-check") {
		t.Fatal("未切换的任务应保持自动")
	}

	// 切回自动
	SetInternalMode("resource-grow", true)
	if !IsInternalAuto("resource-grow") {
		t.Fatal("切回自动后 IsInternalAuto 应为 true")
	}
}

func TestInternalModePersistAndReload(t *testing.T) {
	oldFile := modeFile
	modeFile = filepath.Join(t.TempDir(), "modes_test.json")
	defer func() { modeFile = oldFile }()

	SetInternalMode("code-quality", false)
	SetInternalMode("kb-digest", false)

	// 模拟重启（清内存 map——重新加载）
	modeMu.Lock()
	modes = map[string]bool{}
	modeMu.Unlock()
	loadModes()

	if IsInternalAuto("code-quality") {
		t.Fatal("重启恢复后 code-quality 应保持手动")
	}
	if IsInternalAuto("kb-digest") {
		t.Fatal("重启恢复后 kb-digest 应保持手动")
	}
	if !IsInternalAuto("resource-grow") {
		t.Fatal("未持久化的任务应默认自动")
	}
}

func TestInternalModesSnapshot(t *testing.T) {
	oldFile := modeFile
	modeFile = filepath.Join(t.TempDir(), "modes_test.json")
	defer func() { modeFile = oldFile }()

	SetInternalMode("doc-consistency", false)
	snap := InternalModes()
	if auto, ok := snap["doc-consistency"]; !ok || auto {
		t.Fatalf("InternalModes 应包含 doc-consistency=false: %+v", snap)
	}
}

var _ = os.ModePerm
