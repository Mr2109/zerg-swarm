package agent

// workers_config.go — v2.5.2 设备感知并发配置
// 设计：config/workers.yaml 按 hostname 匹配设备 → 自动选择 max_workers
// 内置默认：node-a=3, local=2, node-b=1, node-c=1

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
	MaxWorkers *int   `yaml:"max_workers"` // nil=未给（回退默认）· 0=显式暂停 · >0=并发上限
}

// DeviceAwareConfig — config/workers.yaml 完整结构
type DeviceAwareConfig struct {
	Devices   []DeviceConfig    `yaml:"devices"`
	Scheduler *SchedulerSection `yaml:"scheduler"`
}

// SchedulerSection — config/workers.yaml 顶层 `scheduler:` 段（P0 #232 · 2026-10-08）。
// 三枚运行旋钮的可选落点：给值 ⇒ 覆盖编译期常量；缺省(nil / 空串) ⇒ 回退常量（见 scheduler.go 的
// LoadSchedulerConfig —— 常量即缺省值，✗ 不删常量）。
type SchedulerSection struct {
	PollInterval string `yaml:"poll_interval"` // 轮询间隔（时长串，如 5s / 1m）
	MaxRetries   *int   `yaml:"max_retries"`   // 死信阈值（nil = 未给 ⇒ 回退常量）
	Cooldown     string `yaml:"cooldown"`      // 失败冷却（时长串，如 30s）
}

// ─── 内置设备映射（文件不存在时的回退）──────────────────────

// defaultDeviceMap — 内置设备名 → max_workers（用于文件不存在或解析失败）
var defaultDeviceMap = map[string]int{
	"mini" + "1": 1,
	"x" + "3":    3,
	"Mr2109":       0, // ★ 显式暂停（Mr2109 2026-09-27 拍板）：0 ⇒ scheduler.go:319 的 activeCount>=maxWorkers 恒真 ⇒ 不派活。恢复：改回 2
	"mini" + "2": 1,
}

// ─── 设备感知加载函数（供 scheduler.go 的 LoadWorkersConfig 调用）─

// loadDeviceMaxWorkers — 从 config/workers.yaml 加载并解析设备配置
func loadDeviceMaxWorkers(workDir string) (*DeviceAwareConfig, error) {
	cfgPath := filepath.Join(workDir, workersConfigPath) // 修双 config：workersConfigPath 已含 config/
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

// resolveByDevice — 按 hostname 匹配设备配置，返回 (max_workers, 是否文件显式给值)（GAP-20260927-298）
func resolveByDevice(devices []DeviceConfig, hostname string, fallback map[string]int, defaultVal int) (int, bool) {
	// 精确匹配
	for _, d := range devices {
		// ★ GAP-20260927-299：空串两侧都不参与匹配（EqualFold("","") 恒真 ⇒ 空条目误命中任意主机）
		if d.Hostname == "" || hostname == "" {
			continue
		}
		if strings.EqualFold(d.Hostname, hostname) {
			if d.MaxWorkers != nil {
				return *d.MaxWorkers, true
			}
		}
	}
	// 模糊匹配：hostname 包含设备名 或 设备名包含 hostname
	// ★ GAP-20260927-299 / -298：空串会让 strings.Contains(x, "") 恒真 ⇒ 误命中任意主机/设备
	//   （yaml 空 hostname 条目，或 os.Hostname() 取失败而 hostname="" 的错误分支）——
	//   名实不符：自述「回退默认」却漏出别机的值（含 0=暂停）。匹配前断言两侧都非空。
	for _, d := range devices {
		if d.Hostname == "" || hostname == "" {
			continue
		}
		if strings.Contains(strings.ToLower(hostname), strings.ToLower(d.Hostname)) ||
			strings.Contains(strings.ToLower(d.Hostname), strings.ToLower(hostname)) {
			if d.MaxWorkers != nil {
				return *d.MaxWorkers, true
			}
		}
	}
	// 回退到内置默认值
	return resolveByHostname(fallback, defaultVal), false
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
	// 模糊匹配（两侧都须非空：Contains(x, "") 恒真，空键会当万能匹配 —— GAP-20260927-298/299）
	for k, v := range deviceMap {
		if k == "" || hostname == "" {
			continue
		}
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
		// 空串不参与匹配（GAP-20260927-299）
		if d.Hostname == "" || hostname == "" {
			continue
		}
		if strings.EqualFold(d.Hostname, hostname) {
			return d.Hostname
		}
	}
	for _, d := range devices {
		if d.Hostname == "" || hostname == "" {
			continue
		}
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
		if d.MaxWorkers != nil && *d.MaxWorkers < 0 {
			issues = append(issues, fmt.Sprintf("devices[%d] (%s): max_workers must be >= 0 (0=暂停调度), current=%d", i, d.Hostname, *d.MaxWorkers))
		}
		if d.MaxWorkers != nil && *d.MaxWorkers > 16 {
			issues = append(issues, fmt.Sprintf("devices[%d] (%s): max_workers should not exceed 16, current=%d", i, d.Hostname, *d.MaxWorkers))
		}
	}
	return issues
}
