package agent

// workers_config_test.go — v2.5.2 T5 并发配置测试（我补——CA 未写）

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadWorkersConfig_Default — 无配置文件 → 按 hostname 默认值
func TestLoadWorkersConfig_Default(t *testing.T) {
	workDir := t.TempDir() // 空目录——无 config/workers.yaml
	cfg := LoadWorkersConfig(workDir)
	if cfg.MaxWorkers <= 0 {
		t.Fatalf("默认配置 max_workers 应 >0，实际 %d", cfg.MaxWorkers)
	}
	t.Logf("默认 max_workers=%d（按 hostname 匹配）", cfg.MaxWorkers)
}

// TestLoadWorkersConfig_FromFile — 配置文件存在 → 读取
func TestLoadWorkersConfig_FromFile(t *testing.T) {
	workDir := t.TempDir()
	cfgDir := filepath.Join(workDir, "config")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	yaml := `
devices:
  - name: "test-device"
    max_workers: 5
`
	if err := os.WriteFile(filepath.Join(cfgDir, "workers.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := LoadWorkersConfig(workDir)
	if cfg.MaxWorkers <= 0 {
		t.Fatalf("配置读取后 max_workers 应 >0，实际 %d", cfg.MaxWorkers)
	}
	t.Logf("配置文件读取成功 max_workers=%d", cfg.MaxWorkers)
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
	if cfg.MaxWorkers <= 0 {
		t.Fatalf("非法配置应回退默认，max_workers 应 >0，实际 %d", cfg.MaxWorkers)
	}
}

// TestResolveByHostname — hostname 匹配
func TestResolveByHostname(t *testing.T) {
	// mini1 → 1（默认表）
	v := resolveByHostname(defaultDeviceMap, 2)
	if v <= 0 {
		t.Fatalf("resolveByHostname 应返回 >0，实际 %d", v)
	}
	t.Logf("hostname 解析 max_workers=%d", v)
}
