package agent

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// GAP-20260927-218：配置路径必须是 <workDir>/config/workers.yaml（单层 config）。
// 改前：filepath.Join(workDir,"config",workersConfigPath) 且 workersConfigPath 已含 "config/"
// ⇒ 拼成 <workDir>/config/config/workers.yaml，配置面永不生效。
func TestGAP218_WorkersYamlPathSingleConfig(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	spec := []byte("devices:\n  - hostname: node-a\n    max_workers: 3\n")
	if err := os.WriteFile(filepath.Join(dir, "config", "workers.yaml"), spec, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadDeviceMaxWorkers(dir)
	if err != nil {
		t.Fatalf("读 <workDir>/config/workers.yaml 失败（双重 config 路径？）：%v", err)
	}
	if len(cfg.Devices) != 1 || cfg.Devices[0].Hostname != "node-a" {
		t.Fatalf("解析结果不符：%+v", cfg.Devices)
	}
}

// GAP-20260927-218：配置文件缺失时须显式播报「未读到」（此前是静默回退）。
func TestGAP218_MissingConfigIsBroadcast(t *testing.T) {
	dir := t.TempDir() // 空目录——无 config/workers.yaml
	var buf bytes.Buffer
	old := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(old)
	LoadWorkersConfig(dir)
	if !strings.Contains(buf.String(), "未读到") {
		t.Fatalf("配置文件缺失时未播报（静默回退）：log=%q", buf.String())
	}
}
