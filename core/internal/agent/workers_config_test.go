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
	if v, _ := resolveByDevice(devices, "qqq-other-sample", map[string]int{}, 2); v != 2 {
		t.Fatalf("不该匹配的主机名应回退 defaultVal==2（不得取设备值 9），实际 %d", v)
	}
}

// (b) 空 hostname 条目：GAP-20260927-299 已修（2026-10-04）——字符串匹配前断言两侧非空
// ⇒ 空条目不再命中任意主机，回退 defaultVal。本条随之翻转为断言「回退 2」。
// （函数名沿用旧名以免删顶层符号；语义已按 299 修法翻转。）
func TestResolveByDevice_EmptyHostname_MatchesAnyHost_BugPinned(t *testing.T) {
	nine := 9
	devices := []DeviceConfig{{Hostname: "", MaxWorkers: &nine}}
	v, _ := resolveByDevice(devices, "qqq-other-sample", map[string]int{}, 2)
	if v != 2 {
		t.Fatalf("空 hostname 条目必须回退 defaultVal==2（GAP-20260927-299 已修：空串不参与匹配），实际 %d", v)
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

// ─── GAP-20260927-298：文件显式 0=暂停 vs 缺省回退抬升 ─────────────

// TestResolveByDevice_ExplicitVsFallback — 钉住 (max_workers, 是否文件显式给值) 语义：
// 显式 0 ⇒ (0,true)（暂停·调用方不得抬升）；无匹配 ⇒ (defaultVal,false)（真·缺省路径）。
func TestResolveByDevice_ExplicitVsFallback(t *testing.T) {
	zero := 0
	five := 5
	// 文件显式 0（暂停）⇒ (0,true)，不得被当成「未设」
	if v, explicit := resolveByDevice([]DeviceConfig{{Hostname: "host-a", MaxWorkers: &zero}}, "host-a", map[string]int{}, 2); v != 0 || !explicit {
		t.Fatalf("文件显式 max_workers:0 应 (0,true)，实际 (%d,%v)", v, explicit)
	}
	// 文件显式正值 ⇒ (v,true)
	if v, explicit := resolveByDevice([]DeviceConfig{{Hostname: "host-b", MaxWorkers: &five}}, "host-b", map[string]int{}, 2); v != 5 || !explicit {
		t.Fatalf("文件显式 max_workers:5 应 (5,true)，实际 (%d,%v)", v, explicit)
	}
	// 无匹配 ⇒ 回退 defaultVal、explicit=false（真·缺省路径）
	if v, explicit := resolveByDevice([]DeviceConfig{{Hostname: "host-c", MaxWorkers: &five}}, "zzz-none", map[string]int{}, 2); v != 2 || explicit {
		t.Fatalf("无匹配应回退 defaultVal (2,false)，实际 (%d,%v)", v, explicit)
	}
}

// TestLoadWorkersConfig_FileExplicitZeroIsPause — GAP-20260927-298 主断言：
// 文件里本机显式 max_workers:0 = 暂停 ⇒ 必须原样 0，不得被旧 mw<=0 夹取抬成 defaultMaxWorkers。
func TestLoadWorkersConfig_FileExplicitZeroIsPause(t *testing.T) {
	workDir := t.TempDir()
	cfgDir := filepath.Join(workDir, "config")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	hn, err := os.Hostname()
	if err != nil {
		t.Fatalf("os.Hostname 失败: %v", err)
	}
	// 本机显式 0（暂停）
	yaml := "devices:\n  - hostname: \"" + hn + "\"\n    max_workers: 0\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "workers.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := LoadWorkersConfig(workDir)
	if cfg.MaxWorkers != 0 {
		t.Fatalf("文件显式 max_workers:0（暂停）应原样 ==0，不得抬升；实际 %d（旧 mw<=0 夹取会抬成 %d）", cfg.MaxWorkers, defaultMaxWorkers)
	}
	t.Logf("文件显式 0 生效 max_workers=%d（暂停）", cfg.MaxWorkers)
}

// TestLoadWorkersConfig_NilValueFallsBackAndLifts — 真·缺省路径：回退内置默认真值，0 不抬升（拍板 2026-10-04）。
// （函数名沿用旧名以免删顶层符号；语义已按 GAP-20260927-298 拍板翻转：）
// 文件里本机条目存在但未给 max_workers（nil）⇒ 回退内置默认；回退到 0（暂停）必须保持 0，不得抬成
// defaultMaxWorkers；抬升只属于真·非法值（负数）。
func TestLoadWorkersConfig_NilValueFallsBackAndLifts(t *testing.T) {
	workDir := t.TempDir()
	cfgDir := filepath.Join(workDir, "config")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	hn, err := os.Hostname()
	if err != nil {
		t.Fatalf("os.Hostname 失败: %v", err)
	}
	// hostname 匹配本机但不给 max_workers（nil ⇒ 非显式）
	yaml := "devices:\n  - hostname: \"" + hn + "\"\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "workers.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := LoadWorkersConfig(workDir)
	builtin := resolveByHostname(defaultDeviceMap, defaultMaxWorkers)
	want := builtin
	if builtin < 0 {
		want = defaultMaxWorkers // 仅真·非法值（负数）抬升；0 不抬升
	}
	if cfg.MaxWorkers != want {
		t.Fatalf("文件未给值应走缺省路径：回退 %d ⇒ 应原样 ==%d（0 不抬升），实际 %d", builtin, want, cfg.MaxWorkers)
	}
	if builtin == 0 && cfg.MaxWorkers != 0 {
		t.Fatalf("回退内置 0 必须保持 0（暂停·不抬升），实际 %d", cfg.MaxWorkers)
	}
	t.Logf("缺省回退=%d ⇒ max_workers=%d（0 不抬升）", builtin, cfg.MaxWorkers)
}

// TestLoadWorkersConfig_FilePresentHostAbsent_ZeroNotLifted — GAP-20260927-298 拍板（2026-10-04）：
// 「表在、但该机无条目」⇒ 回退内置默认；回退到 0（暂停）必须保持 0，绝不抬升。
// 哑化（把 mw<=0 的抬升还原）⇒ 本用例在回退 0 的机器上红。
func TestLoadWorkersConfig_FilePresentHostAbsent_ZeroNotLifted(t *testing.T) {
	workDir := t.TempDir()
	cfgDir := filepath.Join(workDir, "config")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// 表里**只有别的机器**，本机无条目 ⇒ 走缺省回退路径（不得命中任何设备）
	yaml := "devices:\n  - hostname: \"zzz-absent-host-xyz\"\n    max_workers: 9\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "workers.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := LoadWorkersConfig(workDir)
	builtin := resolveByHostname(defaultDeviceMap, defaultMaxWorkers)
	if cfg.MaxWorkers != builtin {
		t.Fatalf("表在但本机无条目应回退内置默认 ==%d（不抬升），实际 %d", builtin, cfg.MaxWorkers)
	}
	if builtin == 0 && cfg.MaxWorkers != 0 {
		t.Fatalf("回退内置 0 必须保持 0（暂停·绝不抬升），实际 %d", cfg.MaxWorkers)
	}
	t.Logf("表在/本机无条目 ⇒ 回退=%d，max_workers=%d（0 不抬升）", builtin, cfg.MaxWorkers)
}
