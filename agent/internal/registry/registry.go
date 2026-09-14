// Package registry 提供模型注册表解析（YAML → 模型配置）。
//
// 职责：
//   - 解析 agent_models.yaml 格式的模型注册表
//   - 提供模型名到配置的查找
//   - 列出所有已注册模型
package registry

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// ModelEntry 单个模型注册条目。
type ModelEntry struct {
	File     string  `yaml:"file"`
	Backend  string  `yaml:"backend"`
	MemGB    float64 `yaml:"mem_gb"`
	Modality string  `yaml:"modality"`
	// D3 多模态：视觉投影文件（llama-server -mm 参数——如 mmproj-*.gguf）
	MMProj string `yaml:"mmproj"`
	// ChatTemplate 可选的 chat template 文件路径（覆盖 GGUF 内嵌模板，如 ornith 系）。
	// 支持 ~ 前缀；相对路径按 agent 工作目录解析。留空则由适配器按环境变量/约定路径回退。
	// 例：chat_template: ~/.zerg/ornith_chat_template.jinja
	ChatTemplate string `yaml:"chat_template,omitempty"`
	// Cmd 自定义启动命令，兼容两种格式：
	//   字符串: "llama-server -m {file} --port {port}"
	//   数组:   ["llama-server", "-m", "{file}", "--port", "{port}"]
	// 可含 {file}/{port}/{dir} 占位符。不填则用默认命令。
	Cmd CmdString `yaml:"cmd,omitempty"`
	// ═══ 虫卵声明字段（P1，设计-子端沙箱化-20260914 §4.3 九项字段表）═══
	// 校验与失败语义见 egg_decl.go（ValidateEggDeclaration）；本处只放字段。
	//
	// SchemaVersion 卵声明格式版本号（设计 §4.3 第八项 / §6.8.4）：
	// 子端升级后据此判断「这枚旧卵我还认不认得」；**孵化前校验**。
	// 认不得 ⇒ 明确报错、拒孵；0 = 未声明（遗留条目，按告警处理，清单落地 P7 时强制补齐）。
	SchemaVersion int `yaml:"schema_version,omitempty"`
	// EnvReq 环境需求（设计 §4.3 第七项 / §6.7）：设备与卡号 / 库路径与版本 /
	// 环境变量（含按引擎覆盖 LD_LIBRARY_PATH）/ 权重路径 / ulimit 与 mmap 限额。
	// nil = 未声明（遗留条目）；非 nil 时**必填项缺一即拒孵**（孵化器只照单执行，不许自己推断）。
	EnvReq *EnvReq `yaml:"env_req,omitempty"`
	// IdleUnloadS 空窗收走阈值（秒；设计 §4.3 第九项 / §6.5 / §13 Q21）：
	// 这枚卵空闲多久被收走。缺省（<=0）取 DefaultIdleUnloadSeconds = 600；
	// 小模型（嵌入 / 重排 / 分类类）建议 120。**已废弃「常驻卵」类别**
	// ——「默认空」无例外，差别只在阈值长短。
	IdleUnloadS int `yaml:"idle_unload_s,omitempty"`
	// ═══ 硬盘流与权重供给（P5，设计-子端沙箱化-20260914 §9.4 / §9.5 / §9.7）═══
	// 归属铁律（§9.2）：--ssd-streaming* 与 --kv-disk-* 是 **ds4 的参数**，不是 llama.cpp 的；
	// llama.cpp 的对应物是 --moe-stream*（机制同源、page cache 策略相反）。
	// 适配器只按本声明发参数，孵化器不做任何推断（§9.5 结论句：虫卵提供条件，引擎自己控流）。
	//
	// SsdStreamingPreloadExperts 预热专家数（ds4 --ssd-streaming-preload-experts，§9.4）：
	// **必须来自实测档案**（P5 验收③：扫 512/1024/2048 的 prefill 实测表，按「标定铁律」
	// 凡数字必实测、禁估值、禁硬编码，§8.4）——无声明（<=0）⇒ 适配器**不发**该参数
	// （F6 的落地通路；无档案不预热）。
	SsdStreamingPreloadExperts int `yaml:"ssd_streaming_preload_experts,omitempty"`
	// KVDisk KV 盘声明（ds4 --kv-disk-dir / --kv-disk-space-mb，§9.4 / §9.7）：
	// **默认关闭**（nil = 不发）；声明了则 space_mb 必填（写盘上限**不许留成无限**，§9.7①），
	// 目录**按卵分目录**（缺省 ~/.zerg/kvdisk/<卵名>/，§9.7④），落「跨孵化保留」侧（§6.6）。
	KVDisk *KVDiskDecl `yaml:"kv_disk,omitempty"`
	// name 这枚卵的名字（注册表键；load/Reload 时盖进条目，见 EggName）。
	name string
	// Custom 存储任意额外字段（如 ssd、ssd_streaming_cache_experts 等）
	Custom map[string]interface{} `yaml:",inline"`
}

// KVDiskDecl 卵的 KV 盘声明（P5，设计 §9.4 / §9.7）。
type KVDiskDecl struct {
	// SpaceMB 写盘上限（MB；--kv-disk-space-mb）。**必填正数**——KV 落盘是持续写，
	// 上限不许留成「无限」（§9.7①）；校验见 ValidateEggDeclaration（缺失即拒孵）。
	SpaceMB int `yaml:"space_mb"`
	// Dir 显式目录覆盖（可选）。缺省按「按卵分目录」取 ~/.zerg/kvdisk/<卵名>/（§9.7④）；
	// 支持 ~ 前缀（展开由适配器负责）。
	Dir string `yaml:"dir,omitempty"`
}

// CmdString 自定义类型：YAML 中既接受字符串也接受字符串数组。
type CmdString string

// UnmarshalYAML 兼容字符串和数组两种格式。
func (c *CmdString) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.SequenceNode {
		var parts []string
		if err := value.Decode(&parts); err != nil {
			return err
		}
		*c = CmdString(strings.Join(parts, " "))
		return nil
	}
	var s string
	if err := value.Decode(&s); err != nil {
		return err
	}
	*c = CmdString(s)
	return nil
}

// Registry 模型注册表，线程安全。
type Registry struct {
	mu     sync.RWMutex
	models map[string]*ModelEntry
	path   string
}

// New 创建并加载注册表。
func New(path string) (*Registry, error) {
	r := &Registry{
		models: make(map[string]*ModelEntry),
		path:   path,
	}
	if err := r.load(); err != nil {
		return nil, fmt.Errorf("加载注册表失败: %w", err)
	}
	return r, nil
}

// load 从 YAML 文件加载模型注册表。
func (r *Registry) load() error {
	data, err := os.ReadFile(r.path)
	if err != nil {
		return fmt.Errorf("读取注册表文件 %s: %w", r.path, err)
	}

	// 解析 YAML：顶层是 map[string]*ModelEntry
	var models map[string]*ModelEntry
	if err := yaml.Unmarshal(data, &models); err != nil {
		return fmt.Errorf("解析注册表 YAML: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// 把注册表键（卵名）盖进条目：适配器做「按卵分目录」（KV 盘缺省 ~/.zerg/kvdisk/<卵名>/，
	// 设计 §9.7④）需要知道这枚卵叫什么——路径规则不该散在适配器里重猜。
	for name, entry := range models {
		if entry != nil {
			entry.name = name
		}
	}
	r.models = models
	return nil
}

// Reload 重新加载注册表。
func (r *Registry) Reload() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	data, err := os.ReadFile(r.path)
	if err != nil {
		return fmt.Errorf("读取注册表文件 %s: %w", r.path, err)
	}

	var models map[string]*ModelEntry
	if err := yaml.Unmarshal(data, &models); err != nil {
		return fmt.Errorf("解析注册表 YAML: %w", err)
	}

	// 同 load：重载后卵名照样要盖进条目（否则热更新一轮 KV 盘目录就丢了卵名）。
	for name, entry := range models {
		if entry != nil {
			entry.name = name
		}
	}
	r.models = models
	return nil
}

// Get 查找模型配置。
func (r *Registry) Get(name string) (*ModelEntry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entry, ok := r.models[name]
	return entry, ok
}

// Names 返回所有已注册模型名称列表。
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.models))
	for name := range r.models {
		names = append(names, name)
	}
	return names
}

// List 返回所有已注册模型条目。
func (r *Registry) List() map[string]*ModelEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make(map[string]*ModelEntry, len(r.models))
	for k, v := range r.models {
		result[k] = v
	}
	return result
}
