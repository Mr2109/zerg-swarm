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

// 失败分类（开工方案 §四 / 标准 §七）：失败必须能归因到"系统哪一环"。
const (
	FailCantStart   = "cant_start"
	FailBadFormat   = "bad_format"
	FailTimeout     = "timeout"
	FailNoVision    = "no_vision"
	FailMmprojMiss  = "mmproj_missing"
	FailNoTools     = "no_tools"
	FailNoMeta      = "no_meta"
	FailNoTemplate  = "no_template"
	FailUnsupported = "unsupported"
)

// DefaultProbeTimeout 是单次探测的超时（开工方案 §四：文本探测超时 60s）。
const DefaultProbeTimeout = 60 * time.Second

// Trace 是一条探测的原始留痕（开工方案 §四）。落进 notes 的 probe_trace 段。
// 注：标准 §三 的 notes 字段是字符串，无法直接承载 "notes.probe_trace[]" 数组，
// 故这里序列化成 notes 里的一段可读文本（见 buildNotes）。
type Trace struct {
	Probe        string `json:"probe"`
	OK           bool   `json:"ok"`
	HTTPStatus   int    `json:"http_status,omitempty"`
	ElapsedMS    int64  `json:"elapsed_ms"`
	FailureClass string `json:"failure_class,omitempty"`
	Summary      string `json:"summary,omitempty"`
}

// TraceSchemaV1 是留痕兄弟文件（<version>.trace.json）的版本号。
// 它与记录 schema 分开：留痕不是记录、不进目录语义，只承载易变信息
// （生成时间、每项探测器的耗时，待修补 #24）。
const TraceSchemaV1 = "zerg.model.probe_trace.v1"

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

func formatGeneratedAt(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
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
}

// ProbeReport 汇总一次探测的全部产物，供生成记录与人工评审。
type ProbeReport struct {
	Target       string
	Endpoint     string
	ModelID      string
	Files        []File
	Meta         *GGUFMeta
	Capabilities []Capability
	Traces       []Trace
	ChatTemplate string
	TemplateOK   bool
	OnlineProbed bool
	// GeneratedAt 是本次探测的时间。它**不进记录正文**（正文必须随内容确定），
	// 只写进留痕兄弟文件（待修补 #24）。
	GeneratedAt time.Time
}

// ProbeOptions 是探测入参。
type ProbeOptions struct {
	Target   string        // 本地路径 或 端点 URL
	Endpoint string        // 本地文件时另给的端点（可选）；只给路径时不做在线探测
	Engine   string        // llama.cpp / vllm / ollama（可选，影响 chat_template 来源判定）
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
	}

	ep := Endpoint{BaseURL: endpointBase, Model: opts.Model, Timeout: timeout}
	textOK := false

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
		textOK = txt.Value
		rep.Capabilities = append(rep.Capabilities, capabilityOf("text", EvidenceText, txt))

		// probe.vision：本仓最看重的一项——必须实测，不许因为模型自称多模态就写 true。
		vis := ProbeVision(ep)
		rep.Traces = append(rep.Traces, vis.Trace)
		rep.Capabilities = append(rep.Capabilities, capabilityOf("vision", EvidenceVision, vis))

		// probe.tools
		tools := ProbeTools(ep)
		rep.Traces = append(rep.Traces, tools.Trace)
		rep.Capabilities = append(rep.Capabilities, capabilityOf("tools", EvidenceTools, tools))

		// 附加：embedding / rerank（端点不支持就如实记 false 并注明）
		emb := ProbeEmbedding(ep)
		rep.Traces = append(rep.Traces, emb.Trace)
		rep.Capabilities = append(rep.Capabilities, capabilityOf("embedding", EvidenceEmbedding, emb))

		rr := ProbeRerank(ep)
		rep.Traces = append(rep.Traces, rr.Trace)
		rep.Capabilities = append(rep.Capabilities, capabilityOf("rerank", EvidenceRerank, rr))
	}

	// probe.template：判定 chat_template 来源（开工方案 §四）。
	tmpl, tclass, tok := resolveChatTemplate(rep.Meta, textOK, opts.Engine)
	ttr := Trace{Probe: EvidenceTemplate, OK: tok}
	if tok {
		ttr.Summary = "chat_template=" + tmpl
	} else {
		ttr.FailureClass = tclass
		ttr.Summary = "缺模板：既无 GGUF tokenizer.chat_template，端点也未在无模板调用下成功回话"
	}
	rep.Traces = append(rep.Traces, ttr)
	if tok {
		rep.ChatTemplate = tmpl
		rep.TemplateOK = true
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
		Schema:       SchemaV1,
		ID:           sanitizeID(id),
		Digest:       SynthesizeDigest(rep.Files),
		Files:        rep.Files,
		Capabilities: rep.Capabilities,
		State:        "known",
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

	// 模态：以实测能力为准（vision 过才写 image 入）。
	ins := []string{"text"}
	if hasCap(rep.Capabilities, "vision") {
		ins = append(ins, "image")
	}
	rec.Modalities = map[string][]string{"in": ins, "out": {"text"}}

	// 引擎配方：能定出模板来源就写 chat_template；私有开关一律 ZERG_ 前缀放 extra_env。
	if rep.ChatTemplate != "" || opts.Engine != "" {
		eng := opts.Engine
		if eng == "" {
			eng = "llama.cpp"
		}
		recipe := EngineRecipe{ChatTemplate: rep.ChatTemplate}
		if rep.Meta != nil && rep.Meta.ContextWindow > 0 {
			recipe.Args = []string{"--ctx-size", strconv.Itoa(rep.Meta.ContextWindow)}
		}
		if rep.ChatTemplate == "" {
			recipe.Reason = "probe.template.v1 no_template：未能定出模板来源，待人工确认"
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

// resolveChatTemplate 判定 chat_template 来源（probe.template.v1）。
//
// 开工方案 §四 的原意是"故意用不带模板的调用方式触发引擎报错，看是否走兜底链"。
// 本实现退一步：不改引擎启动参数时无法"故意触发"，只能从两处证据推断来源——
// GGUF 里的 tokenizer.chat_template，或端点在不带模板调用下仍能回话。
// 推不出就如实报 no_template 并说明缺什么。
func resolveChatTemplate(meta *GGUFMeta, textOK bool, engine string) (tmpl, failClass string, ok bool) {
	if meta != nil && strings.TrimSpace(meta.ChatTemplate) != "" {
		return "from_gguf", "", true
	}
	if textOK {
		switch strings.ToLower(engine) {
		case "vllm":
			return "from_tokenizer", "", true
		default:
			return "from_gguf", "", true
		}
	}
	return "", FailNoTemplate, false
}

// capabilityOf 把一次探测落成能力断言（source=probed；失败带分类与原因）。
func capabilityOf(name, probeName string, r runResult) Capability {
	ev := probeName
	if !r.Value {
		reason := truncate(r.Trace.Summary, 120)
		if r.FailureClass != "" {
			ev = probeName + " (" + r.FailureClass + ": " + reason + ")"
		} else {
			ev = probeName + " (false: " + reason + ")"
		}
	}
	return Capability{Name: name, Value: r.Value, Source: "probed", Evidence: ev}
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
		if rep.Endpoint == "" {
			b.WriteString("\n未读到 GGUF 元数据（probe.meta.gguf.v1 no_meta）：context_window / chat_template 待人工补。")
		} else {
			b.WriteString("\n端点未提供上下文档位（probe.meta.models.v1 只给出模型标识）：context_window 待人工补。")
		}
	}
	if !rep.OnlineProbed {
		b.WriteString("\n未做在线探测：只给了本地文件、未给端点（开工方案 §八 风险2）。text/vision/tools 等能力未实测，故未写断言。")
	}
	return b.String()
}
