package agent

// workers_config.go — v2.5.2 设备感知并发配置
// 设计：config/workers.yaml 按 hostname 匹配设备 → 自动选择 max_workers
// 内置默认：X3=3, local=2, mini1=1, mini2=1

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ─── 新类型（不与 scheduler.go 的 WorkersConfig 冲突）─────────

// DeviceConfig — 单个设备的并发配置
type DeviceConfig struct {
	Hostname   string `yaml:"hostname"`
	MaxWorkers int    `yaml:"max_workers"`
}

// DeviceAwareConfig — config/workers.yaml 完整结构
type DeviceAwareConfig struct {
	Devices []DeviceConfig `yaml:"devices"`
}

// ─── 内置设备映射（文件不存在时的回退）──────────────────────

// defaultDeviceMap — 内置设备名 → max_workers（用于文件不存在或解析失败）
var defaultDeviceMap = map[string]int{
	"mini1": 1,
	"x3":    3,
	"local": 2,
	"mini2": 1,
}

// ─── 设备感知加载函数（供 scheduler.go 的 LoadWorkersConfig 调用）─

// loadDeviceMaxWorkers — 从 config/workers.yaml 加载并解析设备配置
func loadDeviceMaxWorkers(workDir string) (*DeviceAwareConfig, error) {
	cfgPath := filepath.Join(workDir, "config", workersConfigPath)
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read workers.yaml: %w", err)
	}
	var cfg DeviceAwareConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse workers.yaml: %w", err)
	}
	return &cfg, nil
}

// resolveByDevice — 按 hostname 匹配设备配置，返回 max_workers
func resolveByDevice(devices []DeviceConfig, hostname string, fallback map[string]int, defaultVal int) int {
	// 精确匹配
	for _, d := range devices {
		if strings.EqualFold(d.Hostname, hostname) {
			if d.MaxWorkers > 0 {
				return d.MaxWorkers
			}
		}
	}
	// 模糊匹配：hostname 包含设备名 或 设备名包含 hostname
	for _, d := range devices {
		if strings.Contains(strings.ToLower(hostname), strings.ToLower(d.Hostname)) ||
			strings.Contains(strings.ToLower(d.Hostname), strings.ToLower(hostname)) {
			if d.MaxWorkers > 0 {
				return d.MaxWorkers
			}
		}
	}
	// 回退到内置默认值
	return resolveByHostname(fallback, defaultVal)
}

// resolveByHostname — 用内置默认映射解析 max_workers
func resolveByHostname(deviceMap map[string]int, defaultVal int) int {
	hostname, err := os.Hostname()
	if err != nil {
		return defaultVal
	}
	// 尝试内置映射
	if w, ok := deviceMap[strings.ToLower(hostname)]; ok {
		return w
	}
	// 模糊匹配
	for k, v := range deviceMap {
		if strings.Contains(strings.ToLower(hostname), k) || strings.Contains(k, strings.ToLower(hostname)) {
			return v
		}
	}
	return defaultVal
}

// GetDeviceName — 获取当前设备名称（调试/日志用）
func GetDeviceName(devices []DeviceConfig) string {
	hostname, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	for _, d := range devices {
		if strings.EqualFold(d.Hostname, hostname) {
			return d.Hostname
		}
	}
	for _, d := range devices {
		if strings.Contains(strings.ToLower(hostname), strings.ToLower(d.Hostname)) {
			return d.Hostname
		}
	}
	return "unknown"
}

// ValidateWorkersConfig — 验证设备配置合法性
func ValidateWorkersConfig(devices []DeviceConfig) []string {
	var issues []string
	for i, d := range devices {
		if strings.TrimSpace(d.Hostname) == "" {
			issues = append(issues, fmt.Sprintf("devices[%d]: hostname must not be empty", i))
			continue
		}
		if d.MaxWorkers <= 0 {
			issues = append(issues, fmt.Sprintf("devices[%d] (%s): max_workers must be > 0, current=%d", i, d.Hostname, d.MaxWorkers))
		}
		if d.MaxWorkers > 16 {
			issues = append(issues, fmt.Sprintf("devices[%d] (%s): max_workers should not exceed 16, current=%d", i, d.Hostname, d.MaxWorkers))
		}
	}
	return issues
}
