// egg_profile.go —— 实测档案（EggProfile）+ 读取/校验。
//
// 设计真源：§8.4「标定铁律 + 实测档案」制度框（2026-09-15 第五轮拍定）：
//   - 「凡容量/预算/阈值类数字，必须由必做的测试机制实测得出——禁止估值、禁止照抄声明值」。
//   - 「实测档案 = 每枚卵一份实测事实卡」，字段写死六项：
//     权重体积 / 峰值显存（GTT）/ 峰值内存 / 装载耗时 / 吞吐 / 建议空闲阈值。
//   - 「闸门与预算一律读实测档案的数字」——卵声明里的估值（entry.MemGB）只作
//     「填不出档案时的提醒」，**不作为放行依据**（C6：声明 18 GB 实测 32.70 GiB）。
//   - 标定流程（§11 P3）：空载读全局 GTT → 装已知模型 → 读峰值 → 反复三次取上界。
//
// 与 P1 协同：schema_version（registry.EggSchemaVersionCurrent=1）与 EnvReq 已是
// 卵声明字段；本批只定义**档案结构与读取/校验**，闸门接线归 P4 后统一。
package monitor

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// EggProfile 实测档案：每枚卵一份实测事实卡（六字段写死，§8.4 制度框）。
//
// 数字单位与既有口径对齐：体积用 GB（GiB，与 baseline_mem / MemAvailableGb 一致），
// 时长用秒，吞吐用 tokens/s。
type EggProfile struct {
	// WeightSizeGb 权重体积（GB）：权重文件实测字节数换算。装载下限与两笔账的输入端（§9.6）。
	WeightSizeGb float64 `yaml:"weight_size_gb"`
	// PeakGttGb 峰值显存（GTT，GB）：装载+推理期间全局 GTT 增量的实测上界（反复三次取上界）。
	PeakGttGb float64 `yaml:"peak_gtt_gb"`
	// PeakMemGb 峰值内存（GB）：MemAvailable 谷值对应峰值占用的实测上界。
	PeakMemGb float64 `yaml:"peak_mem_gb"`
	// LoadSeconds 装载耗时（秒）：冷装载实测。
	LoadSeconds float64 `yaml:"load_seconds"`
	// ThroughputTokS 吞吐（tokens/s）：实测生成速度。
	ThroughputTokS float64 `yaml:"throughput_tok_s"`
	// SuggestedIdleUnloadS 建议空闲阈值（秒）：喂给卵声明的 idle_unload_s 口径（Q21）。
	SuggestedIdleUnloadS int `yaml:"suggested_idle_unload_s"`

	// ═══ 溯源元数据（实测档案必须可追溯，否则退化成"又一份声明"） ═══

	// SchemaVersion 档案格式版本（与 registry.EggSchemaVersionCurrent 协同；当前 1）。
	SchemaVersion int `yaml:"schema_version"`
	// MeasuredAt 实测时间（RFC3339）。
	MeasuredAt time.Time `yaml:"measured_at"`
	// Machine 实测机型标识（如 "x3"）——档案绑定机型，跨机不得互抄（§1.5 多设备）。
	Machine string `yaml:"machine"`
	// CalibRuns 标定轮数（§11 P3：反复三次取上界 ⇒ 有效值 ≥3）。
	CalibRuns int `yaml:"calib_runs"`
}

// EggProfileSchemaVersion 当前实测档案格式版本。
const EggProfileSchemaVersion = 1

// Validate 校验实测档案：闸门只信"测够轮数、非零、溯源完整"的档案。
// 返回第一个发现的问题（fail-closed：宁可拒孵，不许拿残档案凑数）。
func (p EggProfile) Validate() error {
	if p.SchemaVersion != EggProfileSchemaVersion {
		return fmt.Errorf("实测档案 schema_version=%d，当前子端只认 %d", p.SchemaVersion, EggProfileSchemaVersion)
	}
	if p.Machine == "" {
		return fmt.Errorf("实测档案缺 machine（档案绑定机型，跨机不得互抄）")
	}
	if p.MeasuredAt.IsZero() {
		return fmt.Errorf("实测档案缺 measured_at")
	}
	if p.CalibRuns < 3 {
		return fmt.Errorf("实测档案 calib_runs=%d < 3（标定流程：反复三次取上界，§11 P3）", p.CalibRuns)
	}
	if p.WeightSizeGb <= 0 {
		return fmt.Errorf("实测档案 weight_size_gb=%v 非（>0）", p.WeightSizeGb)
	}
	if p.PeakGttGb <= 0 {
		return fmt.Errorf("实测档案 peak_gtt_gb=%v 非（>0）——标定铁律：GTT 账必须有实测值", p.PeakGttGb)
	}
	if p.PeakMemGb <= 0 {
		return fmt.Errorf("实测档案 peak_mem_gb=%v 非（>0）", p.PeakMemGb)
	}
	if p.LoadSeconds <= 0 {
		return fmt.Errorf("实测档案 load_seconds=%v 非（>0）", p.LoadSeconds)
	}
	if p.ThroughputTokS <= 0 {
		return fmt.Errorf("实测档案 throughput_tok_s=%v 非（>0）", p.ThroughputTokS)
	}
	if p.SuggestedIdleUnloadS <= 0 {
		return fmt.Errorf("实测档案 suggested_idle_unload_s=%v 非（>0）", p.SuggestedIdleUnloadS)
	}
	return nil
}

// LoadEggProfile 从 YAML 文件读一份实测档案并校验。
// 文件不存在/解析失败/校验不过都如实报错——调用方（闸门）拿不到档案就必须拒孵。
func LoadEggProfile(path string) (EggProfile, error) {
	var p EggProfile
	b, err := os.ReadFile(path)
	if err != nil {
		return p, fmt.Errorf("读实测档案 %s: %w", path, err)
	}
	if err := yaml.Unmarshal(b, &p); err != nil {
		return p, fmt.Errorf("解析实测档案 %s: %w", path, err)
	}
	if err := p.Validate(); err != nil {
		return p, err
	}
	return p, nil
}

// GttNeed 按闸门口径取 need_gtt：峰值 × 安全系数 1.1（§8.4 第 2 步，沿用
// ensureMemoryForLocked 的口径）。
func (p EggProfile) GttNeed() float64 { return p.PeakGttGb * 1.1 }

// MemNeed 同上，取 need_mem。
func (p EggProfile) MemNeed() float64 { return p.PeakMemGb * 1.1 }
