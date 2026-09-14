// internal_engine_test.go — 内部任务引擎状态（2026-09-10 治本 APP-A07）
// 验证单一真相源的四条关键语义：门控、用户意图、心跳停滞、持久化往返。
package api

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/compat"
)

// 每个用例独立状态目录（避免污染真实 ~/.zerg/state）
func resetEngineForTest(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", dir)
	engineFile = "" // 清缓存——让 engineStatePath() 重新解析
	engineMu.Lock()
	engine = engineState{}
	engineMu.Unlock()
}

// 门控未放行：running 必须为 false 且原因可读（不得谎报运行中）
func TestEngineDisabledNeverRunning(t *testing.T) {
	resetEngineForTest(t)
	InitInternalEngine()
	SetEngineEnabled(false, "引擎未启用：测试")

	v := InternalEngineStateView()
	if v.Enabled || v.Running {
		t.Fatalf("门控未放行时 enabled/running 应为 false，实际 %+v", v)
	}
	if !strings.Contains(v.Reason, "未启用") {
		t.Fatalf("原因应说明未启用，实际 %q", v.Reason)
	}
}

// 启用 + 未停止 + 心跳新鲜 → running=true
func TestEngineRunningWithFreshHeartbeat(t *testing.T) {
	resetEngineForTest(t)
	InitInternalEngine()
	SetEngineEnabled(true, "已启用")
	SetEngineStopped(false, "运行中")
	EngineTick(60 * time.Second)

	v := InternalEngineStateView()
	if !v.Running {
		t.Fatalf("启用心跳新鲜时应为运行中，实际 %+v", v)
	}
	if v.LastTick == "" || v.NextTick == "" {
		t.Fatalf("心跳/下次心跳应被记录，实际 %+v", v)
	}
}

// 心跳停滞（>3 分钟）→ 不再谎报运行中，且原因指出异常
func TestEngineStaleHeartbeatReportedAsAnomaly(t *testing.T) {
	resetEngineForTest(t)
	InitInternalEngine()
	SetEngineEnabled(true, "已启用")
	SetEngineStopped(false, "运行中")
	engineMu.Lock()
	engine.LastTick = time.Now().Add(-10 * time.Minute) // 模拟引擎卡死/崩溃
	engineMu.Unlock()

	v := InternalEngineStateView()
	if v.Running {
		t.Fatalf("心跳停滞时不应报运行中，实际 %+v", v)
	}
	if !strings.Contains(v.Reason, "心跳") {
		t.Fatalf("原因应指出心跳问题，实际 %q", v.Reason)
	}
}

// 用户停止意图必须持久化（重启恢复——这是原实现的漂移根因）
func TestEngineStoppedPersistsAcrossRestart(t *testing.T) {
	resetEngineForTest(t)
	InitInternalEngine()
	SetEngineEnabled(true, "已启用")
	SetEngineStopped(true, "已停止")

	if _, err := os.Stat(filepath.Join(os.Getenv("ZERG_STATE_DIR"), "internal_engine.json")); err != nil {
		t.Fatalf("停止后应落盘，实际 %v", err)
	}

	// 模拟主控重启：内存清零后重新加载
	engineMu.Lock()
	engine = engineState{}
	engineMu.Unlock()
	InitInternalEngine()

	v := InternalEngineStateView()
	if !v.Stopped {
		t.Fatalf("重启后应恢复“已停止”，实际 %+v", v)
	}
	if v.Enabled {
		t.Fatalf("重启后门控需由 main.go 重新登记，此时应为 false，实际 %+v", v)
	}
}

// 未停止 = 默认（兼容现状：无文件时不停）
func TestEngineDefaultNotStopped(t *testing.T) {
	resetEngineForTest(t)
	InitInternalEngine()
	if InternalEngineStopped() {
		t.Fatal("无持久化文件时应默认未停止")
	}
}

// ─────────────── B7 / G10：跨版本状态文件兼容层接线验收 ───────────────

// 路径必须「同源」：compat 清单解析出的路径与引擎自己的解析器必须一致，
// 否则兼容层迁的是 A、引擎读的是 B —— 迁移「成功」但状态依旧读旧文件。
func TestEngineStatePathMatchesCompatManifest(t *testing.T) {
	resetEngineForTest(t)
	e, ok := compat.Lookup("internal_engine")
	if !ok {
		t.Fatal("compat 清单缺 internal_engine 条目")
	}
	want := e.Path(compat.DefaultDirs())
	if got := engineStatePath(); got != want {
		t.Fatalf("两套路径解析不一致：engine=%s compat=%s", got, want)
	}
}

// 端到端：旧版（无 schema）文件被 InitInternalEngine **原地**迁移，用户意图照常恢复，且二跑幂等。
func TestEngineLegacyFileMigratedOnStartup(t *testing.T) {
	resetEngineForTest(t)
	path := filepath.Join(os.Getenv("ZERG_STATE_DIR"), "internal_engine.json")
	legacy := `{"stopped":true,"since":"2026-01-02T03:04:05Z"}`
	if err := os.WriteFile(path, []byte(legacy), 0o644); err != nil {
		t.Fatalf("写旧版夹具失败: %v", err)
	}

	InitInternalEngine()

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("迁移后文件必须还在: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("迁移后不是合法 JSON: %v", err)
	}
	if got["schema"] != float64(1) {
		t.Errorf("应被迁移到 schema=1，实际 %v", got["schema"])
	}
	if got["stopped"] != true || got["since"] != "2026-01-02T03:04:05Z" {
		t.Errorf("用户意图字段必须一个不少，实际 %v", got)
	}
	if !InternalEngineStopped() {
		t.Error("旧版 stopped=true 必须被恢复")
	}
	if baks, _ := filepath.Glob(path + ".bak-*"); len(baks) != 1 {
		t.Errorf("应恰好 1 份迁移前备份，实际 %d", len(baks))
	}

	InitInternalEngine() // 二次启动 = 幂等
	if baks, _ := filepath.Glob(path + ".bak-*"); len(baks) != 1 {
		t.Errorf("二次启动不得新增备份，实际 %d", len(baks))
	}
}

// 端到端：高版本 schema ⇒ 只读降级——已识别字段照常恢复，但**不迁移、不回写、不备份**。
func TestEngineFutureSchemaIsReadOnly(t *testing.T) {
	resetEngineForTest(t)
	path := filepath.Join(os.Getenv("ZERG_STATE_DIR"), "internal_engine.json")
	future := `{"schema":99,"stopped":true,"since":"2026-01-02T03:04:05Z"}`
	if err := os.WriteFile(path, []byte(future), 0o644); err != nil {
		t.Fatalf("写高版本夹具失败: %v", err)
	}

	InitInternalEngine()

	body, _ := os.ReadFile(path)
	if string(body) != future {
		t.Errorf("高版本文件不得被改写（不猜），实际 %q", string(body))
	}
	if !InternalEngineStopped() {
		t.Error("只读降级仍应恢复已识别字段 stopped=true")
	}
	if baks, _ := filepath.Glob(path + ".bak-*"); len(baks) != 0 {
		t.Errorf("高版本不该触发备份，实际 %d", len(baks))
	}
}
