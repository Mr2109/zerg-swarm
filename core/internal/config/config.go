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
	Name    string  `yaml:"name,omitempty"`   // 模型名称（V001 必填）
	Family  string  `yaml:"family,omitempty"` // 模型家族（V002/V014 必填/校验）
	Host    string  `yaml:"host"`
	Backend string  `yaml:"backend"`
	File    string  `yaml:"file"`
	MemGb   float64 `yaml:"mem_gb"`

	// ═══ GAP-20261014-68：`mem_gb` 的口径必须能申报（加性键 · 旧条目一字不改 · ✗ 不翻红既有条）═══
	// 同一个字段名 `mem_gb` 两侧装两种账：**本侧**（主控 `gateway/fleet.yaml`）= 占用/需求声明
	// （实测峰值 + 余量）；**子端** `agent_models.yaml` = 闸门拍值·故意报大·fail-closed。此前这条口径
	// 只活在注释里（`zerg code find mem_gb_scope` 零命中）⇒ 对拍工具/人都可能拿两侧数值直接比。
	// 本键把它变成机读值（真源常量见本件末尾 `MemGBScope*`）：
	//   `usage`    占用/需求声明口径（实测峰值 + 余量）—— 本侧本义；
	//   `gate-val` 闸门拍值·故意报大·fail-closed —— 子端本义。
	// 空 = 未申报（旧条目）⇒ 兼容读；判据 = `core/cmd/zerg/check_fleet_loadable.go` 判据 F
	// （`mem_gb` 有则口径必有 · 值 ∈ 闭集 · 本侧恒 `usage`）；写面 = `zerg model set --set mem_gb_scope=…`。
	MemGBScope string `yaml:"mem_gb_scope,omitempty"`
	SSD        bool   `yaml:"ssd"`
	CtxWindow  int    `yaml:"ctx_window"` // 模型上下文上限（V22，GGUF 元数据实测）
	// MaxTokens 该候选声明的**单次输出上限**（2026-09-19 ④）。
	//
	// 为什么补这一格：fleet.yaml 里已经写了 `max_tokens: 262144`，但解析侧没有这个字段 ⇒ yaml 里
	// 写了**直接被丢**（真机症状：主控日志只有 `temperature override` 与 `timeout override`，
	// 从来没有 `max_tokens override 262144` —— 声明是死字段）。语义见 gateway/tokencap.go：
	// 它是**上限**（cap），不是"每次都用这个值"。
	MaxTokens int    `yaml:"max_tokens,omitempty"`
	Arch      string `yaml:"arch,omitempty"` // 架构标识（V015 校验）
	// ═══ 模型详情补充字段（2026-08-27 Mr2109——fleet.yaml 写了但之前被丢弃）═══
	Architecture string `yaml:"architecture,omitempty"` // 架构家族（example-35b-v2/qwen35moe/gemma4——fleet 常用 key）
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

	// ═══ GAP-20261003-21：verified 三态与内容锚 —— 消费者侧【加性】承载 ═══
	// 旧 `Verified bool` 只能表达两态；判据 G 要的第三态 `reported`（仅声明）＋ `verified_where`
	// （自报机名）在消费者侧**存不下**（写成 `verified: reported` 会让整份 fleet.yaml 解析失败 ⇒
	// 既有已 verified 的全翻红）。按设计稿 §三乙案加性新增两枚字符串字段：旧 `verified: true/false`
	// 一字不改、照旧按 bool 读（零破坏）；新态走新键 ⇒ **新增面不翻红既有已 verified 的**。
	// ★ 本结构体只承载、不判 —— 判据 G 真身在 check_nodes_roster.go（读 raw YAML 文本，不受此影响）。
	VerifiedState string `yaml:"verified_state,omitempty"` // 三态：true|false|reported（空=旧读者按 Verified 回落）
	VerifiedWhere string `yaml:"verified_where,omitempty"` // 自报机名（reported 时必填 · ∈ 名册）

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
	// Kind 机器种类（2026-09-25 Mr2109 定）：`ai`（AI 专用机）/ `mini`（小型机：类似 Mac mini
	// 这种「内容小」的机器 ⇒ 只接装得下的模型）/ `work`（工作机：Mr2109 的工作机 ⇒ 最后才用）。
	// 溢出序 = ai → mini → work。**空值一律按最保守的 `work` 处理**（未声明种类的机器不得被当专用机
	// 优先使用 —— 宁可少用，不可误用他的工作机）。
	Kind string `yaml:"kind"`
}

// DefaultAgentPort 是子端 agent 的默认 HTTP 端口（fleet.yaml 的 fleet 段未给 port 时的回退口径）。
const DefaultAgentPort = 8100

// NodeAddr 按机器名取地址（纯函数）：机器名 → (IP, 端口)。
//
// 地址一律来自 fleet.yaml 的 fleet 段（配置即事实源）；名册缺该机器、或该机器未给 host 时，
// 回退机器名本身 + DefaultAgentPort（与既有 gateway.fleetNode 口径逐字一致）。
//
// GAP-20260926-71：取址口径此前只作为 gateway 包的私有方法存在（fleetNode/agentURLFor），
// config/agent/chat 三包都够不着 ⇒ 各处只能写死地址。本函数把口径提到 config 包，
// 三包（含 gateway.fleetNode）共用同一真源，杜绝第二份口径。
func NodeAddr(fleet map[string]FleetNode, host string) (string, int) {
	if node, ok := fleet[host]; ok && node.Host != "" {
		port := node.Port
		if port <= 0 {
			port = DefaultAgentPort
		}
		return node.Host, port
	}
	return host, DefaultAgentPort
}

// AuthConfig 认证配置。
type AuthConfig struct {
	Token string `yaml:"token"`
	// Source 令牌**来源标记**（env:ZERG_TOKEN / file:~/.zerg/token / 旧名 / yaml 兜底）——
	// 只供报话与判据用，**永不含令牌值**；yaml:"-" ⇒ 不参与解析（GAP-20261017-04）。
	Source string `yaml:"-"`
}

// FleetConfig 整个 fleet.yaml 的结构。
type FleetConfig struct {
	Auth    AuthConfig                  `yaml:"auth"`
	Models  map[string][]ModelCandidate `yaml:"models"`
	Aliases map[string]string           `yaml:"aliases"` // 客户端别名 → 标准模型名（Hermes 发 zerg-example-35b-v2 等）
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
	// 2026-09-11 A 批（库内零明文）：共享令牌不再依赖 fleet.yaml 里的明文。
	// GAP-20261017-02/-04 收敛：唯一链 = env:ZERG_TOKEN > file:~/.zerg/token > 旧名（兼容·降级）。
	// 三路都空时保留 yaml 中的值（兜底兼容），为空则由调用方（cmd/zerg-core）启动即报错。
	if t, src := ResolveAuthTokenWithSource(); t != "" {
		cfg.Auth.Token = t
		cfg.Auth.Source = string(src)
		AnnounceLegacyTokenSource(src)
	} else if cfg.Auth.Token != "" {
		cfg.Auth.Source = string(TokenSourceConfig)
	} else {
		cfg.Auth.Source = string(TokenSourceMissing)
	}
	return cfg, nil
}

// TokenFilePath 共享令牌文件路径（默认 ~/.zerg/token，单行）。
//
// 2026-09-11 A 批（库内零明文）约定：令牌绝不写进源码或仓库内配置。
// 唯一链（GAP-20261017-02 收敛 · 三处读取面同源）：
//  1. 环境变量 ZERG_TOKEN（**首位**）
//  2. 家目录文件 ~/.zerg/token（单行；仓库外，天然不入库）
//  3. 兼容旧名 ZERG_AUTH_TOKEN / ZERG_API_TOKEN（降级为最后一级，**保留不删**）
//
// 另可用 .env 供给上述环境变量（`set -a; . ./.env; set +a`，见仓库根 .env.example）。
func TokenFilePath() string {
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".zerg", "token")
	}
	return ""
}

// ResolveAuthToken 解析共享令牌（首尾空白已去）：只取令牌值；来源见 ResolveAuthTokenWithSource。
// 找不到返回空字符串——调用方必须显式处理（启动即报错并给指引），不得静默放行。
// ★ 旧名命中时**出声一次**（只报来源，✗ 不回显令牌）。
func ResolveAuthToken() string {
	v, src := ResolveAuthTokenWithSource()
	if v != "" {
		AnnounceLegacyTokenSource(src)
	}
	return v
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
//   - 单 dict: { host: node-a, backend: ds4-server, ... }
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

// mem_gb 口径闭集（GAP-20261014-68）—— 判据 F、写面（`zerg model set`）与消费侧的**真源**
// （`core` 模块内唯一一份；子端侧同名常量在 `agent/internal/registry/egg_decl.go` —— `core` / `agent` 是
// 两个 module ⇒ 各存一份、值面逐字相同，改动任一侧必须同批改另一侧）。
// ★ 收口项：同包 `check_mem_gb_parity.go`（GAP-20261014-67 · 两侧 `mem_gb` 对拍门）自带的
// `mgpScopeClosedSet` 与本闭集**同值两份**（usage / gate-val）⇒ 宜收口为引用本真源（✗ 别各留一份）。
const (
	// MemGBScopeUsage 占用/需求声明口径（实测峰值 + 余量）—— 主控 `gateway/fleet.yaml` 的本义。
	MemGBScopeUsage = "usage"
	// MemGBScopeGateVal 闸门拍值·故意报大·fail-closed —— 子端 `agent_models.yaml` 的本义。
	MemGBScopeGateVal = "gate-val"
)

// MemGBScopes 口径闭集（序即报文里的列举序）。
var MemGBScopes = []string{MemGBScopeUsage, MemGBScopeGateVal}

// MemGBScopeKnown 值是否在闭集内（空串 = 未申报 ⇒ 由调用方按「缺」或「兼容读」分别处置）。
func MemGBScopeKnown(v string) bool {
	for _, s := range MemGBScopes {
		if v == s {
			return true
		}
	}
	return false
}
