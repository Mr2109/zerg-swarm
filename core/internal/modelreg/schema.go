// Package modelreg 实现《标准-模型接入与目录贡献.md》的机器可校验部分。
//
// 批 1 只做 verify（纯离线，无引擎依赖）——它是"标准的执行者"：先有尺子，再量东西。
// probe（五探测器）属批 2；下载链、UI、路由属后续批次。
package modelreg

import "strings"

// SchemaV1 是本标准当前的记录版本号（标准 §十：新增字段必须可选，未知字段必须被忽略而不报错）。
const SchemaV1 = "zerg.model.v1"

// CapabilityNames 是能力标签的取值表（标准 §四）。不许自创。
var CapabilityNames = map[string]bool{
	"text": true, "vision": true, "audio_in": true, "audio_out": true,
	"image_gen": true, "video_gen": true, "music_gen": true,
	"tools": true, "embedding": true, "rerank": true,
	"reasoning": true, "code": true, "reward": true, "omni": true,
	"ocr": true, "timeseries": true, "world_model": true, "vla": true,
}

// CommercialStates 是许可证商用性的四态（标准 §五）。
var CommercialStates = map[string]bool{
	"yes": true, "no": true, "revenue_gated": true, "unknown": true,
}

// EvidenceSources 是断言的来源（标准 §二：证据优先）。
var EvidenceSources = map[string]bool{"probed": true, "declared": true, "manual": true}

// ValidChatTemplate 校验引擎配方里的 chat_template（标准 §六：三选一）。
func ValidChatTemplate(s string) bool {
	if s == "from_gguf" || s == "from_tokenizer" {
		return true
	}
	return strings.HasPrefix(s, "inline:")
}

// File 是模型"一组建材"里的一份（标准 §三：一个模型不是一个文件——VLM 还带 mmproj）。
type File struct {
	Role   string `json:"role"`
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
	URL    string `json:"url,omitempty"`
}

// Capability 是一条能力断言，必须带来源与证据（标准 §四）。
//
// Engines（待修补 #11）是这条断言被**证过成立**的引擎：
// 同一条能力在不同引擎上可以真假不同（实测 vision=true 是在 llama.cpp 侧探到的，
// 而 vLLM 侧没挂 mmproj）。硬门槛必须按**目标引擎**取能力；缺引擎维度 = 不可判定
// （不等于可用，绝不当作全局可用放行）。新增字段可选：旧记录/旧快照没有它照样合法
// （标准 §十 向后兼容）——此时按"不可判定"处理（见 EvaluateCapabilityForEngine）。
type Capability struct {
	Name     string   `json:"name"`
	Value    bool     `json:"value"`
	Source   string   `json:"source"`
	Evidence string   `json:"evidence,omitempty"`
	Conflict bool     `json:"conflict,omitempty"`
	Engines  []string `json:"engines,omitempty"`
}

// License 是许可证块（标准 §五：读权重，不读仓库徽章）。
type License struct {
	SPDX       string `json:"spdx"`
	Name       string `json:"license_name,omitempty"`
	Link       string `json:"license_link,omitempty"`
	Commercial string `json:"commercial"`
	Gated      bool   `json:"gated"`
	SourceURL  string `json:"source_url,omitempty"`
	AcceptedBy string `json:"accepted_by,omitempty"`
	AcceptedAt string `json:"accepted_at,omitempty"`
	// Evidence 是许可证断言的来源锚（待修补 #12）：probe.license.v1 写"读的是哪个键/哪个文件"，
	// 例如 `probe.license.v1 (gguf_key: general.license="apache-2.0")`；
	// 读不到时写 `probe.license.v1 (no_license_source: tried …)`。
	// 为什么必须有：标准 §二 要求每条断言可追溯；「读权重不读徽章」要能当场看出读的是权重还是别处。
	// 新增字段可选：旧读者遇未知字段忽略即可（标准 §十 向后兼容）。
	Evidence string `json:"evidence,omitempty"`
}

// EngineRecipe 是一个引擎的配方（标准 §六：可缺省；私有开关一律加 ZERG_ 前缀放 extra_env）。
type EngineRecipe struct {
	Args         []string           `json:"args,omitempty"`
	ChatTemplate string             `json:"chat_template,omitempty"`
	Sampling     map[string]float64 `json:"sampling,omitempty"`
	ExtraEnv     map[string]string  `json:"extra_env,omitempty"`
	Reason       string             `json:"reason,omitempty"`
	VerifiedAt   string             `json:"verified_at,omitempty"`
}

// Record 是一条模型登记记录（标准 §三）。
type Record struct {
	Schema  string                 `json:"schema"`
	ID      string                 `json:"id"`
	Digest  string                 `json:"digest"`
	Aliases []string               `json:"aliases,omitempty"`
	Name    string                 `json:"name,omitempty"`
	Params  map[string]interface{} `json:"params,omitempty"`
	Format  string                 `json:"format,omitempty"`
	// Parent 是血缘声明：同一 model_id 的上一版（version 形如 sha256-<hex>，或 digest 形如 sha256:<64hex>）。
	// BaseModel 是血缘声明：量化/微调前的基座 id（形如 id）。
	//
	// ⛔ 两条硬约束（待修补 #16）：
	//   - **只能由调用方显式传入**（probe --parent/--base），**绝不**由 store 按写入顺序/时间推断
	//     ——自动推断会让"同一批建材 → 记录正文逐字节相同"这条不变量失效（不同机器/不同写入顺序
	//     会得出不同的 parent，同一条记录因环境不同而不同字节）；
	//   - 可选字段：旧记录没有它们照样合法（标准 §十 向后兼容）。
	Parent    string `json:"parent,omitempty"`
	BaseModel string `json:"base_model,omitempty"`
	// 下面的字段见标准 §三/§四/§六。
	Modalities    map[string][]string     `json:"modalities,omitempty"`
	Capabilities  []Capability            `json:"capabilities,omitempty"`
	ContextWindow int                     `json:"context_window,omitempty"`
	Files         []File                  `json:"files,omitempty"`
	EngineRecipes map[string]EngineRecipe `json:"engine_recipes,omitempty"`
	License       License                 `json:"license"`
	SourceURL     string                  `json:"source_url,omitempty"`
	State         string                  `json:"state,omitempty"`
	Notes         string                  `json:"notes,omitempty"`
}
