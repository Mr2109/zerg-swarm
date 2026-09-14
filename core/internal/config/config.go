// package config 负责解析 fleet.yaml 配置文件。
// 支持 v1 的多候选数组格式：模型名可以映射到候选列表数组，也可以映射到单个 dict。
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ModelCandidate 一个模型在某个主机上的候选部署信息。
type ModelCandidate struct {
	Name      string  `yaml:"name,omitempty"`   // 模型名称（V001 必填）
	Family    string  `yaml:"family,omitempty"` // 模型家族（V002/V014 必填/校验）
	Host      string  `yaml:"host"`
	Backend   string  `yaml:"backend"`
	File      string  `yaml:"file"`
	MemGb     float64 `yaml:"mem_gb"`
	SSD       bool    `yaml:"ssd"`
	CtxWindow int     `yaml:"ctx_window"`     // 模型上下文上限（V22，GGUF 元数据实测）
	Arch      string  `yaml:"arch,omitempty"` // 架构标识（V015 校验）
	// ═══ 模型详情补充字段（2026-08-27 Mr2109——fleet.yaml 写了但之前被丢弃）═══
	Architecture string `yaml:"architecture,omitempty"` // 架构家族（ornith/qwen35moe/gemma4——fleet 常用 key）
	Thinking     *bool  `yaml:"thinking,omitempty"`     // 思考模型（默认开 <think>）
	Mmproj       string `yaml:"mmproj,omitempty"`       // 多模态投影文件（视觉/视频）
	Moe          string `yaml:"moe,omitempty"`          // MoE 结构（256e8a=256专家8激活）
	Template     string `yaml:"template,omitempty"`     // chat template（builtin/jinja）

	// ═══ M0 配置模块完整字段（2026-08-12 扩展——无模块无法调用铁律）═══
	Cmd         []string `yaml:"cmd,omitempty"`          // 启动参数（--jinja 等新架构必须参数）
	Env         []string `yaml:"env,omitempty"`          // 依赖环境变量（LD_LIBRARY_PATH 等）
	Modality    string   `yaml:"modality,omitempty"`     // text/vision
	ToolSupport *bool    `yaml:"tool_support,omitempty"` // 工具调用支持（nil=未知，true/false 显式）
	Description string   `yaml:"description,omitempty"`  // 模型描述
	Added       string   `yaml:"added,omitempty"`        // 接入日期
	Verified    bool     `yaml:"verified,omitempty"`     // 验证状态

	// ═══ P1：虫卵的「引擎实现/变体」（设计-子端沙箱化-20260914 §1.2 / §4.7 / 附录 C·C1）═══
	// 承载字段就是上面的 Cmd（哪个二进制 / build / fork，**含包装脚本与 env 处理**）——
	// 「引擎实现/变体」是卵的**必需字段**，不能只给引擎名（§1.2）。
	// 本布尔是**通用声明开关**：当架构还没进 MainlineUnsupportedArchitectures 清单、
	// 而这枚卵确实要非主线实现时用它显式声明。两者任一为真且缺 cmd: ⇒ **Fatal（拒孵）**，
	// 不得静默退回主线 llama-server（详见 validator.go V016）。
	EngineImplRequired bool `yaml:"engine_impl_required,omitempty"`
}

// FleetNode 集群中一个节点的配置信息。
type FleetNode struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
	OS   string `yaml:"os"`
}

// AuthConfig 认证配置。
type AuthConfig struct {
	Token string `yaml:"token"`
}

// FleetConfig 整个 fleet.yaml 的结构。
type FleetConfig struct {
	Auth    AuthConfig                  `yaml:"auth"`
	Models  map[string][]ModelCandidate `yaml:"models"`
	Aliases map[string]string           `yaml:"aliases"` // 客户端别名 → 标准模型名（Hermes 发 zerg-ornith 等）
	Fleet   map[string]FleetNode        `yaml:"fleet"`
}

// LoadFleetConfig 从指定路径加载 fleet.yaml 配置文件。
func LoadFleetConfig(path string) (*FleetConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}
	cfg, err := ParseFleetConfig(data)
	if err != nil {
		return nil, err
	}
	// 2026-09-11 A 批（库内零明文）：共享令牌不再依赖 fleet.yaml 里的明文——
	// 解析顺序：环境变量 ZERG_AUTH_TOKEN / ZERG_API_TOKEN → ~/.zerg/token 文件。
	// 两处都没有时保留 yaml 中的值（向后兼容），为空则由调用方（cmd/zerg-core）启动即报错。
	if t := ResolveAuthToken(); t != "" {
		cfg.Auth.Token = t
	}
	return cfg, nil
}

// TokenFilePath 共享令牌文件路径（默认 ~/.zerg/token，单行）。
//
// 2026-09-11 A 批（库内零明文）约定：令牌绝不写进源码或仓库内配置。
// 推荐三种提供方式（优先级从高到低）：
//  1. 环境变量 ZERG_AUTH_TOKEN（兼容旧名 ZERG_API_TOKEN）
//  2. 家目录文件 ~/.zerg/token（单行；仓库外，天然不入库）
//  3. .env 文件 + `set -a; . ./.env; set +a`（见仓库根 .env.example）
func TokenFilePath() string {
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".zerg", "token")
	}
	return ""
}

// ResolveAuthToken 解析共享令牌：环境变量优先，其次 ~/.zerg/token 文件（首尾空白已去）。
// 找不到返回空字符串——调用方必须显式处理（启动即报错并给指引），不得静默放行。
func ResolveAuthToken() string {
	for _, k := range []string{"ZERG_AUTH_TOKEN", "ZERG_API_TOKEN"} {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
	}
	if p := TokenFilePath(); p != "" {
		if b, err := os.ReadFile(p); err == nil {
			if v := strings.TrimSpace(string(b)); v != "" {
				return v
			}
		}
	}
	return ""
}

// MaskToken 打日志用的令牌掩码（保留前 4 位与长度，绝不打印完整令牌）。
func MaskToken(t string) string {
	if t == "" {
		return "(未设置)"
	}
	if len(t) <= 4 {
		return "****"
	}
	return t[:4] + strings.Repeat("*", 6) + fmt.Sprintf("(len=%d)", len(t))
}

// ParseFleetConfig 从 YAML 字节数据解析配置。
// 关键处理：兼容单 dict 和多候选数组两种格式。
// 使用 yaml.Node 做自定义解析，因为 yaml.v3 无法将 inline flow map 自动转为 slice。
func ParseFleetConfig(data []byte) (*FleetConfig, error) {
	cfg := &FleetConfig{
		Models: make(map[string][]ModelCandidate),
		Fleet:  make(map[string]FleetNode),
	}

	// 预处理：清理注释
	cleaned := parseWithCleanup(data)

	// 解析为 yaml.Node 树
	var root yaml.Node
	if err := yaml.Unmarshal(cleaned, &root); err != nil {
		return nil, fmt.Errorf("解析 YAML 失败: %w", err)
	}

	// root.Content[0] 是顶层映射节点
	if len(root.Content) == 0 {
		return nil, fmt.Errorf("空的 YAML 文件")
	}
	topMapping := root.Content[0]

	// 解析 auth
	authNode, ok := findMapValue(topMapping, "auth")
	if ok {
		if err := authNode.Decode(&cfg.Auth); err != nil {
			return nil, fmt.Errorf("解析 auth 失败: %w", err)
		}
	}

	// 解析 fleet
	fleetNode, ok := findMapValue(topMapping, "fleet")
	if ok {
		if err := fleetNode.Decode(&cfg.Fleet); err != nil {
			return nil, fmt.Errorf("解析 fleet 失败: %w", err)
		}
	}

	// 解析 aliases（客户端别名 → 标准模型名）
	aliasesNode, ok := findMapValue(topMapping, "aliases")
	if ok {
		if err := aliasesNode.Decode(&cfg.Aliases); err != nil {
			return nil, fmt.Errorf("解析 aliases 失败: %w", err)
		}
	}

	// 解析 models：自定义处理单 dict 和多候选数组两种格式
	modelsNode, ok := findMapValue(topMapping, "models")
	if ok {
		// modelsNode 是一个映射节点，其 Content 是 key-value entry 对
		for i := 0; i+1 < len(modelsNode.Content); i += 2 {
			keyNode := modelsNode.Content[i]
			valNode := modelsNode.Content[i+1]

			// 解析模型名
			var modelName string
			if err := keyNode.Decode(&modelName); err != nil {
				return nil, fmt.Errorf("解析模型名失败: %w", err)
			}

			// 解析候选列表
			candidates, err := parseCandidates(valNode)
			if err != nil {
				return nil, fmt.Errorf("解析模型 %s 的候选列表失败: %w", modelName, err)
			}
			cfg.Models[modelName] = candidates
		}
	}

	return cfg, nil
}

// parseCandidates 解析单个模型的候选列表。
// 支持：
//   - 单 dict: { host: x3, backend: ds4-server, ... }
//   - 多候选数组: [{host, ...}, {host, ...}]
func parseCandidates(node *yaml.Node) ([]ModelCandidate, error) {
	if node.Kind == yaml.MappingNode {
		// 单 dict，转为只有一个元素的数组
		var candidate ModelCandidate
		if err := node.Decode(&candidate); err != nil {
			return nil, err
		}
		return []ModelCandidate{candidate}, nil
	}

	if node.Kind == yaml.SequenceNode {
		// 数组，直接解码
		var candidates []ModelCandidate
		if err := node.Decode(&candidates); err != nil {
			return nil, err
		}
		return candidates, nil
	}

	return nil, fmt.Errorf("不支持的节点类型: %d", node.Kind)
}

// findMapValue 在映射节点中找到指定的键，返回对应的值节点。
func findMapValue(node *yaml.Node, key string) (*yaml.Node, bool) {
	if node.Kind != yaml.MappingNode {
		return nil, false
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1], true
		}
	}
	return nil, false
}

// parseWithCleanup 清理 YAML 中的注释。
func parseWithCleanup(data []byte) []byte {
	lines := strings.Split(string(data), "\n")
	var cleaned []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		cleaned = append(cleaned, line)
	}
	return []byte(strings.Join(cleaned, "\n"))
}
