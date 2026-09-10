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
	// Custom 存储任意额外字段（如 ssd、ssd_streaming_cache_experts 等）
	Custom map[string]interface{} `yaml:",inline"`
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
	mu      sync.RWMutex
	models  map[string]*ModelEntry
	path    string
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
