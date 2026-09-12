package modelreg

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ── 批 2：五个探测器（probe）───────────────────────────────────────────────
//
// 为什么每个探测器都产出"带变体与版本的 evidence"：
// 《标准-模型接入与目录贡献》§二 要求"证据优先"（每句断言可追溯）、§四 要求
// source=probed 必须写"探测器名 + 版本"；《开工方案-模型探测与校验》§四 还要求把
// 原始响应摘要（前 200 字符 + 状态码 + 耗时）留痕，并把失败归因到系统环节
// （failure_class），禁止写成"这模型不行"。下面把这三条落成类型与常量。
//
// 本批按开工方案 §四 实现五条：text / vision / tools / meta / template；
// 另按要求补 embedding / rerank 两条（标准 §四 能力表里有 embedding、rerank 标签，
// 但开工方案 §四 未列，故作为附加探测器，报告里已注明差异）。

// 探测器名（含变体与版本）。写死成常量，避免各处随手拼字符串导致证据不可复现。
const (
	EvidenceText       = "probe.text.v1"
	EvidenceVision     = "probe.vision.1x1.v1"
	EvidenceTools      = "probe.tools.v1"
	EvidenceEmbedding  = "probe.embedding.v1"
	EvidenceRerank     = "probe.rerank.v1"
	EvidenceMetaGGUF   = "probe.meta.gguf.v1"
	EvidenceMetaModels = "probe.meta.models.v1"
	EvidenceTemplate   = "probe.template.v1"
)

// chat_template 来源的合法取值（与 ValidChatTemplate 的三条取值一致；该函数未改动）：
//   - from_gguf：模板来自本地 GGUF 的 tokenizer.chat_template 键；
//   - from_tokenizer：模板来自端点只读元信息接口返回的 chat_template 字段。
//
// 第三条 inline:<模板> 由人工/声明填写，probe 不产出。
const (
	ChatTemplateFromGGUF      = "from_gguf"
	ChatTemplateFromTokenizer = "from_tokenizer"
)

// TemplateGGUFKey 是 GGUF 元数据里承载模板的键名——from_gguf 路径的证据锚（待修补 #22）。
const TemplateGGUFKey = "tokenizer.chat_template"

// CapabilityTemplate 是模板来源不可判定时写进 unverifiable[] 的名字。
// 它沿用探测器主题名（probe.template.v1 → template），与 #27 的 text/vision/tools 同格式；
// 注意：模板来源**不是**标准 §四 的能力标签，故不进 CapabilityNames、也不出现在 capabilities[]。
const CapabilityTemplate = "template"

// 失败分类（开工方案 §四 / 标准 §七）：失败必须能归因到"系统哪一环"。
const (
	FailCantStart  = "cant_start"
	FailBadFormat  = "bad_format"
	FailTimeout    = "timeout"
	FailNoVision   = "no_vision"
	FailMmprojMiss = "mmproj_missing"
	FailNoTools    = "no_tools"
	FailNoMeta     = "no_meta"
	FailNoTemplate = "no_template"
	// FailNoTemplateSource 表示"两条真路径都没拿到模板来源"（待修补 #22）：本地 GGUF 无
	// tokenizer.chat_template 键，端点只读元信息（/props 等）也没返回 chat_template。
	// 它**不是**"系统没探够"（那是 budget_exhausted/timeout），而是"没有可复现的证据"——
	// 按与 #27 同一套纪律：不写默认值、不生成条目，改记 unverifiable[]（缺=未知）。
	FailNoTemplateSource = "no_template_source"
	FailUnsupported      = "unsupported"
	// FailBudgetExhausted 表示"预算不足（或超时）导致探不出结论"，**不是**"确定没有这个能力"。
	// 按项目哲学（没有弱模型，只有不完善的系统）：这种情况不得报 capabilities.value=false，
	// 改为不生成该能力条目 + 在快照里写一条 unverifiable 记录（待修补 #27，见 Unverifiable）。
	FailBudgetExhausted = "budget_exhausted"
)

// DefaultProbeTimeout 是单次探测的超时（开工方案 §四：文本探测超时 60s）。
const DefaultProbeTimeout = 60 * time.Second

// Trace 是一条探测的原始留痕（开工方案 §四）。落进 notes 的 probe_trace 段。
// 注：标准 §三 的 notes 字段是字符串，无法直接承载 "notes.probe_trace[]" 数组，
// 故这里序列化成 notes 里的一段可读文本（见 buildNotes）。
type Trace struct {
	Probe      string `json:"probe"`
	OK         bool   `json:"ok"`
	HTTPStatus int    `json:"http_status,omitempty"`
	ElapsedMS  int64  `json:"elapsed_ms"`
	// Budget 是本次探测最终实际用掉的生成预算（max_tokens）。"先小后大"重试后，
	// 这是重试到的那一档——evidence 里据此可分辨实际预算（待修补 #27）。
	Budget       int    `json:"budget,omitempty"`
	FailureClass string `json:"failure_class,omitempty"`
	Summary      string `json:"summary,omitempty"`
}

// TraceSchemaV1 是留痕兄弟文件（<version>.trace.json）的版本号。
// 它与记录 schema 分开：留痕不是记录、不进目录语义，只承载易变信息
// （生成时间、每项探测器的耗时，待修补 #24）。
const TraceSchemaV1 = "zerg.model.probe_trace.v1"

// CapabilitySnapshotSchemaV1 是能力快照兄弟文件（<version>.capabilities.json）的版本号。
// 与记录 schema 分开：快照不是记录、不进目录语义，承载"它现在能干什么"（能力断言 + 证据）。
//
// 待修补 #27 新增可选字段 unverifiable[]：承载"预算不足/超时导致探不出结论"的能力，
// 与 capabilities 里"确定不支持"（value=false）严格区分（缺 = 未知，绝不 = 没有）。
// 新增字段可选、旧读者忽略即可（标准 §十：新增字段必须可选，未知字段必须被忽略而不报错）。
const CapabilitySnapshotSchemaV1 = "zerg.model.capability_snapshot.v1"

// dateRE 认出 RFC3339 形态的时间戳（用于测试断言正文/notes 不含生成时间）。
var dateRE = regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}`)

// ProbeTraceArtifact 是探测留痕的独立产物，作为记录正文的**兄弟文件**落盘。
//
// 为什么必须移出正文（待修补 #24）：生成时间、每项探测器的耗时（elapsed_ms）都是
// 易变信息；记录正文是"内容寻址"的锚（digest 只吃 role+sha256），正文一旦含易变值，
// 同一模型重复 probe 就会被判成"同摘要不同内容"，撞上 Store.Put 的防覆盖保护。
// 留痕是给人的证据，仍然保留 —— 只是搬到正文之外（--json 里照给，--store 时写兄弟文件）。
type ProbeTraceArtifact struct {
	Schema       string  `json:"schema"`
	ID           string  `json:"id,omitempty"`
	Digest       string  `json:"digest,omitempty"`
	Target       string  `json:"target,omitempty"`
	Endpoint     string  `json:"endpoint,omitempty"`
	GeneratedAt  string  `json:"generated_at,omitempty"`
	OnlineProbed bool    `json:"online_probed"`
	Traces       []Trace `json:"traces"`
}

// TraceSiblingPath 由记录路径推出留痕兄弟文件路径：
//
//	<...>/<version>.json → <...>/<version>.trace.json
//
// 与记录同目录、同摘要前缀（同一条记录只有一个版本 = 一个摘要）。
func TraceSiblingPath(recordPath string) string {
	if strings.HasSuffix(recordPath, ".json") {
		return strings.TrimSuffix(recordPath, ".json") + ".trace.json"
	}
	return recordPath + ".trace.json"
}

// WriteProbeTraceSibling 把探测留痕原子写成记录旁的兄弟文件。
//
// 留痕**允许**随探测波动（它就是承载耗时与生成时间的），故这里不比对、允许重写；
// 记录的防覆盖保护不受影响——兄弟文件与记录正文互不干扰（Store.Put 一字未改）。
func WriteProbeTraceSibling(recordPath string, rec *Record, rep *ProbeReport) (string, error) {
	art := ProbeTraceArtifact{
		Schema:       TraceSchemaV1,
		ID:           rec.ID,
		Digest:       rec.Digest,
		Target:       rep.Target,
		Endpoint:     rep.Endpoint,
		GeneratedAt:  formatGeneratedAt(rep.GeneratedAt),
		OnlineProbed: rep.OnlineProbed,
		Traces:       rep.Traces,
	}
	data, err := json.MarshalIndent(art, "", "  ")
	if err != nil {
		return "", fmt.Errorf("序列化探测留痕失败：%w", err)
	}
	data = append(data, '\n')
	path := TraceSiblingPath(recordPath)
	if _, err := WriteFileAtomic(path, data); err != nil {
		return path, err
	}
	return path, nil
}

// CapabilitySnapshotArtifact 是能力实测快照，作为记录正文的**兄弟文件**落盘。
//
// 为什么能力必须移出正文（本批的核心）：
//   - 记录正文承载"它是谁"——digest 只吃建材（role+sha256），同一建材重复探测必须逐字节相同；
//   - 能力承载"它现在能干什么"——随端点/引擎/时间变化，必须带证据（source+evidence）、可刷新。
//
// 两者混在一处时，"先 probe --store（无端点，能力为空）、后 probe --endpoint --store
// （有端点，能力有实测证据）"这条自然流程会撞上 Store.Put 的防覆盖保护（ConflictError）——
// 同摘要、同路径、内容却因能力而不同。分层后：正文只装身份（换端点也逐字节相同），
// 能力+证据写本快照（可随时刷新，不影响 identity）。
type CapabilitySnapshotArtifact struct {
	Schema       string       `json:"schema"`
	ID           string       `json:"id,omitempty"`
	Digest       string       `json:"digest,omitempty"`
	Target       string       `json:"target,omitempty"`
	Endpoint     string       `json:"endpoint,omitempty"`
	GeneratedAt  string       `json:"generated_at,omitempty"`
	OnlineProbed bool         `json:"online_probed"`
	Capabilities []Capability `json:"capabilities"`
	// Unverifiable 是本轮"没探出结论"的能力（预算不足/超时导致），与 capabilities 里
	// "确定不支持"（value=false）严格区分：缺 = 未知，不等于没有（待修补 #27）。
	// 新增字段可选：旧读者遇未知字段忽略即可（标准 §十 向后兼容）。
	Unverifiable []Unverifiable `json:"unverifiable,omitempty"`
	// EndpointChatTemplate 是端点只读元信息探到的 chat_template（from_tokenizer）——"现状"。
	// 它只进本快照、**绝不进记录正文**：正文里 engine_recipes.chat_template 仅允许来自本地
	// GGUF（from_gguf）。端点没给/没探到则省略（缺 = 未知，绝不回退默认值）。待修补 #22 收口。
	// 新增字段可选：旧读者遇未知字段忽略即可（标准 §十 向后兼容）。
	EndpointChatTemplate *EndpointChatTemplate `json:"endpoint_chat_template,omitempty"`
}

// EndpointChatTemplate 是端点只读元信息里探到的模板来源与证据，落在能力快照的
// endpoint_chat_template 字段（待修补 #22 收口）。
//
// 为什么不进记录正文：它随"这台引擎此刻报什么"变化——同一建材换端点/不给端点就会不同，
// 一旦写进正文就让"同一批建材 → 正文逐字节相同"这条不变量（待修补 #24）失效。它属于
// "它现在能干什么"（现状），与能力断言同层，故落快照，可随探测刷新、不影响 identity。
type EndpointChatTemplate struct {
	Value    string `json:"value"`              // 固定 from_tokenizer（与 ValidChatTemplate 取值一致）
	Evidence string `json:"evidence,omitempty"` // 探测器名 + 版本 + 端点 + 字段名（可复现）
}

// NewCapabilitySnapshot 由一次探测的产物构造能力快照（写盘与 --json 共用同一份内容）。
func NewCapabilitySnapshot(rec *Record, rep *ProbeReport) CapabilitySnapshotArtifact {
	return CapabilitySnapshotArtifact{
		Schema:               CapabilitySnapshotSchemaV1,
		ID:                   rec.ID,
		Digest:               rec.Digest,
		Target:               rep.Target,
		Endpoint:             rep.Endpoint,
		GeneratedAt:          formatGeneratedAt(rep.GeneratedAt),
		OnlineProbed:         rep.OnlineProbed,
		Capabilities:         rep.Capabilities,
		Unverifiable:         rep.Unverifiable,
		EndpointChatTemplate: rep.EndpointChatTemplate,
	}
}

// CapabilitySnapshotPath 由记录路径推出能力快照兄弟文件路径：
//
//	<...>/<version>.json → <...>/<version>.capabilities.json
//
// 与记录同目录、同摘要前缀（同一条记录只有一个版本 = 一个摘要），命名风格照 TraceSiblingPath。
func CapabilitySnapshotPath(recordPath string) string {
	if strings.HasSuffix(recordPath, ".json") {
		return strings.TrimSuffix(recordPath, ".json") + ".capabilities.json"
	}
	return recordPath + ".capabilities.json"
}

// LoadCapabilitySnapshot 只读地读一份能力快照兄弟文件（<version>.capabilities.json）。
//
// 与 Load 同一只读纪律：**只**做 os.ReadFile + json.Unmarshal，绝不创建/改写任何文件；
// 未知字段按 encoding/json 默认被忽略（标准 §十 向后兼容）。
// 文件不存在 / 不是合法 JSON 一律返回错误，由调用方决定降级策略——例如
// GET /api/models/registry 据此把该条能力留空，不 500、不造值（读不到 ≠ 没能力，
// 也 ≠ 编一个出来）。读取逻辑只此一份，写盘（WriteCapabilitySnapshot）与读盘共用同一类型。
func LoadCapabilitySnapshot(path string) (*CapabilitySnapshotArtifact, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var art CapabilitySnapshotArtifact
	if err := json.Unmarshal(b, &art); err != nil {
		return nil, fmt.Errorf("能力快照不是合法 JSON：%w", err)
	}
	return &art, nil
}

// WriteCapabilitySnapshot 把能力快照原子写成记录旁的兄弟文件。
//
// 快照**允许**随探测刷新（它承载的正是"现在能干什么"，端点/引擎一变就该更新），故这里
// 不比对、允许重写；记录正文的防覆盖保护不受影响——兄弟文件与记录正文互不干扰
// （Store.Put 一字未改）。
func WriteCapabilitySnapshot(recordPath string, rec *Record, rep *ProbeReport) (string, error) {
	art := NewCapabilitySnapshot(rec, rep)
	data, err := json.MarshalIndent(art, "", "  ")
	if err != nil {
		return "", fmt.Errorf("序列化能力快照失败：%w", err)
	}
	data = append(data, '\n')
	path := CapabilitySnapshotPath(recordPath)
	if _, err := WriteFileAtomic(path, data); err != nil {
		return path, err
	}
	return path, nil
}

func formatGeneratedAt(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// Unverifiable 是一条"无法判定"记录：这次探测因预算不足（或超时）没能得出结论。
//
// 为什么不写进 capabilities 的 value=false：false 意味着"确定没有这个能力"，
// 而这里的真相是"系统没探够"（待修补 #27）。按项目哲学（没有弱模型，只有不完善的系统），
// 缺 = 未知、绝不 = 没有：于是不生成能力条目，改在本记录留下可追责的痕迹。
type Unverifiable struct {
	Name     string `json:"name"`               // 能力标签（标准 §四 取值表）
	Reason   string `json:"reason"`             // 原因分类：budget_exhausted / timeout
	Evidence string `json:"evidence,omitempty"` // 探测器名 + 版本 + 实际预算 + 原始响应摘要
}

// CapabilityProbe 是能力类探测器的结论。
// Value=false 是"实测没过"，不是"猜它不行"——Evidence 里必须带失败分类与原因。
type CapabilityProbe struct {
	Name         string // 能力标签（标准 §四 取值表）
	Value        bool
	Evidence     string
	FailureClass string
	Trace        Trace
}

// runResult 是探测器内部统一返回：值 + 失败分类 + 留痕。
type runResult struct {
	Value        bool
	FailureClass string
	Trace        Trace
	// Undetermined=true 表示"没探出结论"（预算不足 / 超时），既不是通过、也不是确定不支持。
	// 上层据此**不生成** capabilities 条目，而在 unverifiable[] 留痕（待修补 #27）。
	Undetermined bool
}

// ProbeReport 汇总一次探测的全部产物，供生成记录与人工评审。
type ProbeReport struct {
	Target       string
	Endpoint     string
	ModelID      string
	Files        []File
	Meta         *GGUFMeta
	Capabilities []Capability
	// Unverifiable 记录本次"探不出结论"的能力（预算不足/超时）。这些能力**不**出现在
	// Capabilities 里——缺 = 未知，绝不写成 value=false（待修补 #27）。
	Unverifiable []Unverifiable
	Traces       []Trace
	// ChatTemplate 是 probe.template.v1 的结论（from_gguf / from_tokenizer）；为空表示两条
	// 真路径都没拿到证据——此时**不写默认值**，改在 Unverifiable[] 记一条（待修补 #22）。
	//
	// ⚠️ 注意区分：本字段是**报告层**的结论（供留痕/快照/--json 用）。记录**正文**里
	// engine_recipes.chat_template 只允许来自 GGUF——当它为 from_tokenizer 时，正文不写
	// chat_template（端点模板改由 EndpointChatTemplate 进快照，见待修补 #22 收口）。
	ChatTemplate string
	TemplateOK   bool
	// EndpointChatTemplate 是端点只读元信息探到的模板（from_tokenizer）——"现状"。它**不进
	// 记录正文**，改由能力快照的 endpoint_chat_template 字段承载；端点没给就是 nil（缺=未知）。
	EndpointChatTemplate *EndpointChatTemplate
	OnlineProbed         bool
	// LocalFile 表示本次探测的目标是一个本地文件（而不是端点 URL）。
	// 记录正文里那句"未做在线探测"的说明由它决定，**不由**是否给了端点决定——
	// 正文必须随建材确定：同一建材给不给端点、换哪个端点，正文都要逐字节相同。
	LocalFile bool
	// GeneratedAt 是本次探测的时间。它**不进记录正文**（正文必须随内容确定），
	// 只写进留痕兄弟文件（待修补 #24）。
	GeneratedAt time.Time
}

// ProbeOptions 是探测入参。
type ProbeOptions struct {
	Target   string        // 本地路径 或 端点 URL
	Endpoint string        // 本地文件时另给的端点（可选）；只给路径时不做在线探测
	Engine   string        // llama.cpp / vllm / ollama（可选，决定 engine_recipes 的键名）
	Model    string        // 请求里的 model 字段（可选；默认向端点问 /v1/models）
	ID       string        // 覆盖自动推导的 id（可选）
	Timeout  time.Duration // 单次探测超时（默认 60s）
	Now      func() time.Time
}

// UnreachableError 表示用户显式给出的端点连不上（CLI 映射 exit 4）。
type UnreachableError struct {
	Endpoint string
	Err      error
}

func (e *UnreachableError) Error() string {
	return fmt.Sprintf("引擎端点不可达 %s：%v", e.Endpoint, e.Err)
}

// ProbeTargetError 表示目标本身就不是一个可探测的模型（CLI 映射 exit 1）。
type ProbeTargetError struct {
	Target       string
	FailureClass string
	Err          error
}

func (e *ProbeTargetError) Error() string {
	return fmt.Sprintf("探测目标不可用 %s（%s）：%v", e.Target, e.FailureClass, e.Err)
}

// SynthesizeDigest 由 files[] 的 (role, sha256) 合成模型身份摘要。
//
// ⛔ 硬规则（开工方案 §五.2 待修补 #1）：**绝不**把路径或文件名喂进来。
// 路径与文件名属于"存放位置"，不是身份；否则同一批文件换个目录、改个文件名
// 就会被判成"另一个模型"，导致重复下载与校验失败。因此本函数只看 Role 与
// SHA256，Name/URL 一律不参与——同一组 (role, sha256) 无论顺序、文件名、路径，
// digest 恒等。
func SynthesizeDigest(files []File) string {
	parts := make([]string, 0, len(files))
	for _, f := range files {
		parts = append(parts, f.Role+"\x00"+f.SHA256)
	}
	sort.Strings(parts)
	h := sha256.New()
	for _, p := range parts {
		h.Write([]byte(p))
		h.Write([]byte{'\n'})
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

// HashFile 计算一个本地文件的 (role, sha256, size)。
// role 只按文件名里的 "mmproj" 提示分类（不做目录扫描——那是批 3）。
func HashFile(path string) (File, error) {
	fh, err := os.Open(path)
	if err != nil {
		return File{}, err
	}
	defer fh.Close()
	h := sha256.New()
	n, err := io.Copy(h, fh)
	if err != nil {
		return File{}, err
	}
	base := filepath.Base(path)
	role := "weights"
	if strings.Contains(strings.ToLower(base), "mmproj") {
		role = "mmproj"
	}
	return File{Role: role, Name: base, SHA256: hex.EncodeToString(h.Sum(nil)), Size: n}, nil
}

var idSanitize = regexp.MustCompile(`[^a-z0-9-]+`)

// sanitizeID 把任意来源的名字收成标准要求的 id（小写、连字符、无空格）。
func sanitizeID(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = idSanitize.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		return "model"
	}
	return s
}

func sha256hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// Probe 跑探测器并把结果汇总成一条 Record。
//
// 三个返回值：记录 / 探测留痕报告 / 错误。
// 只有"目标本身不是可探测的模型"（本地空文件、路径不存在）或"显式端点连不上"
// 才返回错误（CLI 据此映射 exit 1 / 4）。单个能力探测失败**不算**错误——
// 如实记 value=false 即可（不许猜，也不许把失败当异常吞掉）。
func Probe(opts ProbeOptions) (*Record, *ProbeReport, error) {
	nowFn := opts.Now
	if nowFn == nil {
		nowFn = time.Now
	}
	now := nowFn()
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = DefaultProbeTimeout
	}
	rep := &ProbeReport{Target: opts.Target, GeneratedAt: now}

	lower := strings.ToLower(opts.Target)
	isURL := strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")

	endpointBase := ""
	derivedID := ""

	if isURL {
		endpointBase = opts.Target
	} else {
		fi, err := os.Stat(opts.Target)
		if err != nil {
			return nil, rep, &ProbeTargetError{opts.Target, FailCantStart, fmt.Errorf("本地路径不可读：%w", err)}
		}
		if fi.IsDir() {
			return nil, rep, &ProbeTargetError{opts.Target, FailCantStart, fmt.Errorf("目标是目录：本批只探测单个文件（目录扫描属后续批）")}
		}
		if fi.Size() == 0 {
			return nil, rep, &ProbeTargetError{opts.Target, FailCantStart, fmt.Errorf("目标文件为空：不是可用的模型")}
		}
		fl, err := HashFile(opts.Target)
		if err != nil {
			return nil, rep, &ProbeTargetError{opts.Target, FailCantStart, fmt.Errorf("无法读取并摘要文件：%w", err)}
		}
		rep.Files = append(rep.Files, fl)
		derivedID = strings.TrimSuffix(filepath.Base(opts.Target), filepath.Ext(opts.Target))

		if strings.EqualFold(filepath.Ext(opts.Target), ".gguf") {
			meta, tr, err := ProbeMetaGGUFFile(opts.Target)
			rep.Traces = append(rep.Traces, tr)
			if err == nil {
				rep.Meta = meta
			}
		}
		endpointBase = opts.Endpoint
		rep.LocalFile = true
	}

	ep := Endpoint{BaseURL: endpointBase, Model: opts.Model, Timeout: timeout}

	if endpointBase != "" {
		rep.Endpoint = endpointBase

		// probe.meta：向端点要 /v1/models，拿模型标识（端点通常不报上下文档位）。
		modelID, mres := ProbeMetaModels(ep)
		rep.Traces = append(rep.Traces, mres.Trace)
		if modelID != "" {
			rep.ModelID = modelID
			if ep.Model == "" {
				ep.Model = modelID
			}
			if derivedID == "" {
				derivedID = modelID
			}
		} else if derivedID == "" {
			// 端点没元数据；模型标识退回端点地址，仅用于请求体。
			derivedID = endpointBase
		}
		if ep.Model == "" {
			ep.Model = "zerg-probe"
		}

		// probe.text：主探测。连不上即"引擎不可达"（P4，最多 1 次，不重试轰炸）。
		txt := ProbeText(ep)
		rep.Traces = append(rep.Traces, txt.Trace)
		if !txt.Value && txt.FailureClass == FailCantStart {
			return nil, rep, &UnreachableError{Endpoint: endpointBase, Err: fmt.Errorf("%s", txt.Trace.Summary)}
		}
		rep.OnlineProbed = true
		addCapability(rep, "text", EvidenceText, txt)

		// probe.vision：本仓最看重的一项——必须实测，不许因为模型自称多模态就写 true。
		vis := ProbeVision(ep)
		rep.Traces = append(rep.Traces, vis.Trace)
		addCapability(rep, "vision", EvidenceVision, vis)

		// probe.tools
		tools := ProbeTools(ep)
		rep.Traces = append(rep.Traces, tools.Trace)
		addCapability(rep, "tools", EvidenceTools, tools)

		// 附加：embedding / rerank（端点不支持就如实记 false 并注明）
		emb := ProbeEmbedding(ep)
		rep.Traces = append(rep.Traces, emb.Trace)
		addCapability(rep, "embedding", EvidenceEmbedding, emb)

		rr := ProbeRerank(ep)
		rep.Traces = append(rep.Traces, rr.Trace)
		addCapability(rep, "rerank", EvidenceRerank, rr)
	}

	// probe.template：判定 chat_template 来源（开工方案 §四）——**只认真证据**（待修补 #22）。
	//
	// 两条真路径：① 本地 GGUF 元数据里确有非空的 tokenizer.chat_template 键（from_gguf，
	// 证据写键名）；② 端点只读元信息接口（/props，退一步 /v1/models）返回了非空
	// chat_template 字段（from_tokenizer，证据写端点字段名）。
	//
	// 两条都不成立 → **不给默认值**（缺=未知）：不写 engine_recipes 的 chat_template，改在
	// unverifiable[] 记一条（reason=no_template_source）。旧实现按"端点能回话"推断来源
	// （vllm→from_tokenizer、其余→from_gguf），属"推断冒充实测"，整段删除。
	//
	// 优先级：本地建材（GGUF）高于端点——GGUF 里有模板时它就是权威来源（本地事实优先于
	// 这台引擎此刻报的状态）；这样同一建材换端点/不给端点，正文里的 engine_recipes 才一致
	// （待修补 #24 的正文确定性）。
	//
	// 收口（待修补 #22）：正文里的 engine_recipes.chat_template **只允许来自 GGUF**。端点探到
	// 的模板（from_tokenizer）属于"现状"，只进能力快照的 endpoint_chat_template 字段——
	// 否则同一建材给不给端点，正文就会不一致，破坏 #24 的逐字节确定性。
	var epTpl *endpointTemplate
	if endpointBase != "" {
		t := ProbeChatTemplateFromEndpoint(ep)
		epTpl = &t
	}
	tres := resolveChatTemplate(rep.Meta, epTpl)
	ttr := Trace{Probe: EvidenceTemplate, OK: tres.Source != ""}
	if ttr.OK {
		ttr.Summary = tres.Evidence
		rep.ChatTemplate = tres.Source
		rep.TemplateOK = true
	} else {
		uv := templateUnverifiable(rep.Meta, endpointBase != "")
		ttr.FailureClass = tres.FailClass
		ttr.Summary = uv.Evidence
		rep.Unverifiable = append(rep.Unverifiable, uv)
	}
	rep.Traces = append(rep.Traces, ttr)

	// 端点探到的模板：记进快照承载结构（证据写清端点 + 字段名），**不进记录正文**。
	// 与 GGUF 是否也有模板无关——它就是"这台引擎此刻报什么"，端点给了就如实留痕；
	// 端点没给则保持 nil（缺 = 未知，绝不回退默认值）。
	if epTpl != nil && epTpl.OK {
		rep.EndpointChatTemplate = &EndpointChatTemplate{
			Value:    ChatTemplateFromTokenizer,
			Evidence: templateEvidence(epTpl.Anchor + " @ " + endpointBase),
		}
	}

	// 端点探测无本地文件：用端点模型标识造一条"虚拟建材料"，让记录结构完整
	// （files[] 必填）。它不是真实文件，notes 已注明待人工补。
	//
	// ⛔ 摘要只吃"模型身份"（模型 id），不吃端点 URL：URL 是"存放位置"，
	// 同一端点写成 host:port 或 host:port/v1 必须得到同一个 digest
	// （同一条硬规则，见 SynthesizeDigest）。
	if len(rep.Files) == 0 {
		mid := rep.ModelID
		if mid == "" {
			mid = ep.Model
		}
		rep.Files = []File{{Role: "endpoint", Name: mid, SHA256: sha256hex("zerg.endpoint:" + mid), Size: 0}}
	}

	id := opts.ID
	if id == "" {
		id = derivedID
	}
	rec := rep.toRecord(id, opts)
	return rec, rep, nil
}

// toRecord 把探测产物落成一条符合标准的 Record。
func (rep *ProbeReport) toRecord(id string, opts ProbeOptions) *Record {
	rec := &Record{
		Schema: SchemaV1,
		ID:     sanitizeID(id),
		Digest: SynthesizeDigest(rep.Files),
		Files:  rep.Files,
		// ⛔ 能力断言**不进记录正文**：正文只装"它是谁"（身份随建材确定）；
		// 能力 + 证据写记录旁的 <version>.capabilities.json 快照（可刷新，不影响 identity）。
		// 这样"同一建材、不同端点两次探测 → 正文逐字节相同"，先登记后补能力实测不再撞
		// Store.Put 的防覆盖保护。Capabilities 字段保留在 schema 里（标准 §十：新增字段
		// 必须可选），仍由 Verify 校验——供人工/声明的断言使用，只是 probe 不再填它。
		State: "known",
	}

	// 许可证：读权重文件里能读到的；读不到写 unknown（标准 §二/§五：绝不默认 yes）。
	spdx, lname, llink := "unknown", "", ""
	if rep.Meta != nil {
		spdx, lname, llink = canonicalSPDX(rep.Meta.LicenseSPDX, rep.Meta.LicenseName, rep.Meta.LicenseLink)
	}
	rec.License = License{
		SPDX:       spdx,
		Name:       lname,
		Link:       llink,
		Commercial: "unknown", // 绝不默认 yes
		// accepted_by / accepted_at **留空，不写占位值**（批 3 修正；待修补 #21 修改后）：
		// 探测不代表任何人接受条款。按新规则：commercial=unknown 允许留痕为空；
		// no/revenue_gated 才必填；任何情况下非空即不许是占位值。人工审许可后自行填写。
	}

	if rep.Meta != nil {
		rec.Format = "gguf"
		rec.Name = rep.Meta.Name
		rec.ContextWindow = rep.Meta.ContextWindow
	}

	// 模态：只写**建材能确定**的（本批只探单个文件，不猜 mmproj）→ 恒为 text→text。
	// 不再按"这次探到 vision"往正文里加 image——那会让正文随端点变化。
	// 能力（含 vision）以实测为准，见 <version>.capabilities.json 快照。
	rec.Modalities = map[string][]string{"in": {"text"}, "out": {"text"}}

	// 引擎配方：记录正文里的 chat_template **只允许来自本地 GGUF**（from_gguf，随建材确定）。
	// 本地 GGUF 没有 tokenizer.chat_template 时，**即使端点探到了模板**，正文也不写
	// chat_template——端点模板是"这台引擎此刻的状态"，会随端点变；它改由能力快照的
	// endpoint_chat_template 承载（见 rep.EndpointChatTemplate）。这样同一批建材、给不给端点，
	// 正文才逐字节相同（待修补 #24 的不变量，收口 #22 时不得被牺牲）。
	// 私有开关一律 ZERG_ 前缀放 extra_env。
	ggufTemplate := ""
	if rep.ChatTemplate == ChatTemplateFromGGUF {
		ggufTemplate = rep.ChatTemplate
	}
	if ggufTemplate != "" || opts.Engine != "" {
		eng := opts.Engine
		if eng == "" {
			eng = "llama.cpp"
		}
		recipe := EngineRecipe{ChatTemplate: ggufTemplate}
		if rep.Meta != nil && rep.Meta.ContextWindow > 0 {
			recipe.Args = []string{"--ctx-size", strconv.Itoa(rep.Meta.ContextWindow)}
		}
		if ggufTemplate == "" {
			recipe.Reason = "probe.template.v1 no_template：未从本地 GGUF 定出模板来源，待人工确认"
		}
		rec.EngineRecipes = map[string]EngineRecipe{eng: recipe}
	}

	rec.Notes = rep.buildNotes()
	return rec
}

func hasCap(caps []Capability, name string) bool {
	for _, c := range caps {
		if c.Name == name && c.Value {
			return true
		}
	}
	return false
}

// canonicalSPDX 把 GGUF 里的许可串收成 license.spdx 允许的形态。
// 读不到就 unknown；能读到名字+链接但非 SPDX 时写 other（必须同时有 link，
// 否则 verify 会报错——这里顺带守住这条）。
func canonicalSPDX(raw, name, link string) (string, string, string) {
	raw = strings.TrimSpace(raw)
	switch strings.ToLower(raw) {
	case "":
		if name != "" && link != "" {
			return "other", name, link
		}
		return "unknown", "", ""
	case "apache-2.0", "apache 2.0", "apache2.0":
		return "Apache-2.0", "", ""
	case "mit":
		return "MIT", "", ""
	}
	if !strings.ContainsAny(raw, " \t") {
		return raw, "", ""
	}
	return "unknown", "", ""
}

// templateResolution 是 probe.template.v1 的结论（真证据优先；拿不到就如实说不知道）。
type templateResolution struct {
	Source    string // from_gguf / from_tokenizer；空表示未定
	Evidence  string // 证据串：probe.template.v1 (锚)
	FailClass string // Source=="" 时的原因分类（no_template_source）
}

// resolveChatTemplate 判定 chat_template 来源（probe.template.v1）——只认真证据，拒绝推断（待修补 #22）。
//
// 两条真路径：
//   - from_gguf：本地 GGUF 元数据里确有非空的 tokenizer.chat_template 键；
//   - from_tokenizer：端点只读元信息接口返回了非空 chat_template 字段（epTpl.OK）。
//
// 二者都不成立 → Source 为空、FailClass=no_template_source（缺=未知，**绝不**回退默认值）。
//
// 优先级：GGUF 高于端点。GGUF 里的模板是建材自带的权威事实；端点报的模板是"这台引擎此刻
// 的状态"。同一建材换端点重探时，正文里的 engine_recipes 必须以建材为准（待修补 #24）。
func resolveChatTemplate(meta *GGUFMeta, epTpl *endpointTemplate) templateResolution {
	if meta != nil && strings.TrimSpace(meta.ChatTemplate) != "" {
		return templateResolution{
			Source:   ChatTemplateFromGGUF,
			Evidence: templateEvidence("gguf_key: " + TemplateGGUFKey),
		}
	}
	if epTpl != nil && epTpl.OK {
		return templateResolution{
			Source:   ChatTemplateFromTokenizer,
			Evidence: templateEvidence(epTpl.Anchor),
		}
	}
	return templateResolution{FailClass: FailNoTemplateSource}
}

// templateEvidence 生成 probe.template.v1 的证据串（形如 `probe.template.v1 (锚)`）。
func templateEvidence(anchor string) string {
	return fmt.Sprintf("%s (%s)", EvidenceTemplate, anchor)
}

// templateUnverifiable 在两条真路径都不成立时，产出一条不可判定记录（待修补 #22，承 #27）：
// 不生成 engine_recipes 的 chat_template，改在能力快照的 unverifiable[] 留痕。
func templateUnverifiable(meta *GGUFMeta, endpointGiven bool) Unverifiable {
	ggufPart := "本地 GGUF 未读到 " + TemplateGGUFKey + " 键"
	if meta == nil {
		ggufPart = "本地 GGUF 元数据不可用（未读到 " + TemplateGGUFKey + " 键）"
	}
	epPart := "未给端点，无法读只读元信息（/props 等）"
	if endpointGiven {
		epPart = "端点只读元信息（/props、/v1/models）未返回非空 chat_template 字段"
	}
	return Unverifiable{
		Name:     CapabilityTemplate,
		Reason:   FailNoTemplateSource,
		Evidence: truncate(fmt.Sprintf("%s (%s: %s；%s)", EvidenceTemplate, FailNoTemplateSource, ggufPart, epPart), 200),
	}
}

// evidenceName 给探测器名附上实际预算，让读者能分辨这次用了多大预算（待修补 #27）。
func evidenceName(probeName string, budget int) string {
	if budget > 0 {
		return fmt.Sprintf("%s budget=%d", probeName, budget)
	}
	return probeName
}

// capabilityOf 把一次探测落成能力断言（source=probed；失败带分类与原因）。
// Value=false 只用于"确定不支持"；"预算不足/超时"由 unverifiableOf 另记（待修补 #27）。
func capabilityOf(name, probeName string, r runResult) Capability {
	ev := evidenceName(probeName, r.Trace.Budget)
	if !r.Value {
		reason := truncate(r.Trace.Summary, 120)
		if r.FailureClass != "" {
			ev += " (" + r.FailureClass + ": " + reason + ")"
		} else {
			ev += " (false: " + reason + ")"
		}
	}
	return Capability{Name: name, Value: r.Value, Source: "probed", Evidence: ev}
}

// unverifiableOf 把一次"探不出结论"的探测落成不可判定记录（待修补 #27）。
// 它与 capabilityOf 泾渭分明：前者是"没探够"，后者是"实测确认（通过或不支持）"。
func unverifiableOf(name, probeName string, r runResult) Unverifiable {
	ev := evidenceName(probeName, r.Trace.Budget)
	if r.FailureClass != "" {
		ev += " (" + r.FailureClass + ": " + truncate(r.Trace.Summary, 120) + ")"
	}
	return Unverifiable{Name: name, Reason: r.FailureClass, Evidence: ev}
}

// addCapability 按探测结论分流：探出结论 → 写能力断言；没探出结论（预算不足/超时）→
// 写 unverifiable，**绝不**写成 value=false（待修补 #27：false 意味着"确定没有"）。
func addCapability(rep *ProbeReport, name, probeName string, r runResult) {
	if r.Undetermined {
		rep.Unverifiable = append(rep.Unverifiable, unverifiableOf(name, probeName, r))
		return
	}
	rep.Capabilities = append(rep.Capabilities, capabilityOf(name, probeName, r))
}

// buildNotes 生成记录的 notes。**必须是确定文本**（待修补 #24 的硬要求）：
// 不许出现生成时间戳、探测耗时或逐项探测留痕——那些易变信息随留痕搬到兄弟文件
// （<version>.trace.json）与 `probe --json` 输出；记录正文只留随内容确定的内容说明。
//
// 为什么正文必须确定：正文才是内容寻址的锚（digest 只吃 role+sha256）。正文一旦含
// 易变值，同一模型重复 probe 就会被判成"同摘要不同内容"，撞上 Store.Put 的防覆盖保护，
// 表现为"重复跑就报错"（CLI exit 3），而不是存储层承诺的 changed=false。
func (rep *ProbeReport) buildNotes() string {
	var b strings.Builder
	b.WriteString("本记录由 zerg-model probe 自动生成：正文确定，不含生成时间与探测耗时（那些易变信息见记录旁的 <version>.trace.json 兄弟文件，以及 probe --json 输出里附的探测留痕）。")
	b.WriteString("\n待人工补：license.spdx / license.commercial / license.accepted_by / license.accepted_at（标准 §五：拿不到权重许可就写 unknown，绝不默认 yes）。")
	b.WriteString("\nlicense.accepted_by/accepted_at 留空：探测不代表任何人接受条款。按待修补 #21 的规则（unknown 允许无留痕；no/revenue_gated 必填；任何情况下不许占位），本记录 commercial=unknown 故留痕可为空，且不得写入占位值。人工审许可后填写这两个字段；commercial != yes 之前不得作为默认项（标准 §五 红线）。")
	if rep.Meta == nil {
		if rep.LocalFile {
			b.WriteString("\n未读到 GGUF 元数据（probe.meta.gguf.v1 no_meta）：context_window / chat_template 待人工补。")
		} else {
			b.WriteString("\n端点未提供上下文档位（probe.meta.models.v1 只给出模型标识）：context_window 待人工补。")
		}
	}
	// 本地文件探测：正文里没有能力断言——能力 + 证据已移到记录旁的
	// <version>.capabilities.json 快照（可刷新）。这一段的开关只认"目标是不是本地文件"，
	// **不认**这次给没给端点：同一建材换端点/不给端点重探，正文必须逐字节相同，
	// 否则第二遍会撞上 Store.Put 的防覆盖保护（ConflictError）。文案逐字保留，
	// 以便与此前已登记的记录（同一建材、无端点那次）保持字节一致。
	if rep.LocalFile {
		b.WriteString("\n未做在线探测：只给了本地文件、未给端点（开工方案 §八 风险2）。text/vision/tools 等能力未实测，故未写断言。")
	}
	return b.String()
}
