package agent

// workers_config_test.go — v2.5.2 T5 并发配置测试（我补——CA 未写）

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLoadWorkersConfig_Default — 负例(c)：无配置文件 → 内置表按 hostname 解析
// 真值路径：loadDeviceMaxWorkers 失败 ⇒ scheduler.go:110-112 直接
// resolveByHostname(defaultDeviceMap, defaultMaxWorkers)
// ★ 无文件分支不过 scheduler.go:123-125 的 mw<=0 夹取——只有「有文件」分支才夹取
func TestLoadWorkersConfig_Default(t *testing.T) {
	workDir := t.TempDir() // 空目录——无 config/workers.yaml
	cfg := LoadWorkersConfig(workDir)
	// 机无关真值：同源同参解析（本机 <host> ⇒ 内置表 Mr2109=0 显式暂停）
	want := resolveByHostname(defaultDeviceMap, defaultMaxWorkers)
	if cfg.MaxWorkers != want {
		t.Fatalf("无文件回退 max_workers 应 ==%d（内置表按 hostname 解析），实际 %d", want, cfg.MaxWorkers)
	}
	t.Logf("无文件回退 max_workers=%d（内置表按 hostname 解析）", cfg.MaxWorkers)
}

// TestLoadWorkersConfig_FromFile — 配置文件存在 → 读取
func TestLoadWorkersConfig_FromFile(t *testing.T) {
	workDir := t.TempDir()
	cfgDir := filepath.Join(workDir, "config")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	hn, err := os.Hostname()
	if err != nil {
		t.Fatalf("os.Hostname 失败: %v", err)
	}
	// ★ 键名必须是 hostname（workers_config.go:20 tag）
	//   旧件写 name ⇒ 无对应字段 ⇒ 被 yaml 忽略 ⇒ Hostname="" ⇒ 靠「空串包含」误命中
	yaml := "devices:\n  - hostname: \"" + hn + "\"\n    max_workers: 7\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "workers.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := LoadWorkersConfig(workDir)
	// ★ 相等断言：hostname 精确命中 ⇒ resolveByDevice 直返该设备 max_workers=7
	if cfg.MaxWorkers != 7 {
		t.Fatalf("hostname 精确命中后 max_workers 应 ==7，实际 %d", cfg.MaxWorkers)
	}
	t.Logf("配置文件读取成功 max_workers=%d（hostname 精确命中 %s）", cfg.MaxWorkers, hn)
}

// TestLoadWorkersConfig_Invalid — 非法配置文件 → 回退默认
func TestLoadWorkersConfig_Invalid(t *testing.T) {
	workDir := t.TempDir()
	cfgDir := filepath.Join(workDir, "config")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// 非法 yaml
	if err := os.WriteFile(filepath.Join(cfgDir, "workers.yaml"), []byte("::not: [valid"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := LoadWorkersConfig(workDir)
	// 解析失败 ⇒ 与「无文件」同分支 scheduler.go:110-112（不经 :123 夹取）
	want := resolveByHostname(defaultDeviceMap, defaultMaxWorkers)
	if cfg.MaxWorkers != want {
		t.Fatalf("非法配置应回退内置表 ==%d，实际 %d", want, cfg.MaxWorkers)
	}
}

// TestResolveByHostname — 内置表解析：精确命中 / 不命中回退 defaultVal（机无关·确定性）
func TestResolveByHostname(t *testing.T) {
	hn, err := os.Hostname()
	if err != nil {
		t.Fatalf("os.Hostname 失败: %v", err)
	}
	// 命中：键用真实 hostname 构造 ⇒ 必中，值可预测
	if v := resolveByHostname(map[string]int{strings.ToLower(hn): 7}, 2); v != 7 {
		t.Fatalf("内置表精确命中应 ==7，实际 %d", v)
	}
	// 不命中：键与本机 hostname 互不含 ⇒ 回退 defaultVal
	if v := resolveByHostname(map[string]int{"zzz-definitely-absent-xyz": 5}, 2); v != 2 {
		t.Fatalf("内置表不命中应回退 defaultVal ==2（不得取 5），实际 %d", v)
	}
}

// ─── 负例三连（2026-09-27）─────────────────────────────────

// (a) 不该匹配的主机名不得匹配（resolveByDevice 直调 + 空 fallback 表 ⇒ 机无关确定性）
func TestResolveByDevice_NonMatchingHostname(t *testing.T) {
	nine := 9
	devices := []DeviceConfig{{Hostname: "zzz-not-this-box", MaxWorkers: &nine}}
	if v := resolveByDevice(devices, "qqq-other-sample", map[string]int{}, 2); v != 2 {
		t.Fatalf("不该匹配的主机名应回退 defaultVal==2（不得取设备值 9），实际 %d", v)
	}
}

// (b) 空 hostname 条目：现实现会命中任意主机（workers_config.go:67-68 的
// strings.Contains(host, "")==true）⇒ 本条钉死既存行为（真值 9）。
// 若要「空 hostname 不得匹配」，必须改 workers_config.go（禁区·需拍板）；
// 届时本条须同步改为断言回退值 2。
func TestResolveByDevice_EmptyHostname_MatchesAnyHost_BugPinned(t *testing.T) {
	nine := 9
	devices := []DeviceConfig{{Hostname: "", MaxWorkers: &nine}}
	v := resolveByDevice(devices, "qqq-other-sample", map[string]int{}, 2)
	if v != 9 {
		t.Fatalf("空 hostname 条目现行为应命中任意主机（既存缺陷值 9）——行为已变，复核 workers_config.go:56-76；实际 %d", v)
	}
}

// (b-备选) 空 hostname 条目必须被校验器拒绝（workers_config.go:117-122）
func TestValidateWorkersConfig_RejectsEmptyHostname(t *testing.T) {
	nine := 9
	issues := ValidateWorkersConfig([]DeviceConfig{{Hostname: "", MaxWorkers: &nine}})
	if len(issues) == 0 {
		t.Fatal("空 hostname 条目必须被 ValidateWorkersConfig 标记（workers_config.go:120-122），实际无 issue")
	}
}
