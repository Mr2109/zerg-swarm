// obs_prompt.go — T3.3 **分段落账 + 渲染后 prefix 哈希**（H3）+ T3.5 **时钟与种子制度化 + 提示纯净度守卫**（H6）
//
// 设计依据（逐字）：
//
//	「H3 S1 指纹只存整体哈希 ⇒ 无法定位『变了哪一段』；且指纹对象错了（应对**渲染后实际发送的 prefix**
//	  取哈希）｜★★★｜分段落账：`segments[{id, sha256, tokens}]` + `rendered_prefix_hash` + `template_hash`
//	  （两者不一致 ⇒ 存在动态注入）」。
//	「H6 时间/随机未制度化（提示里禁 `now()`；时间只以『宿主注入变量+记录该值』或『工具结果』两种方式进入）
//	  ｜★★★｜每个请求带 `clock_iso` + `request_seed`，否则回放与回归不成立」。
//
// 任务表 T3.3 落点 `obs.go`（本文件与 obs.go 同属观测面，块与入口分文件、字段口径各写一处）+ T3.5「全局调用点」。
//
// 要治的盲区（改本文件前先读）：
//   - **"提示变了"目前只有一个整体哈希** ⇒ 提示抖动/被谁改这一层完全不可归因：是系统提示变了？
//     工具清单变了？还是历史变了？（H3 的第一半）
//   - **指纹对象错了**：整体哈希算在拼装**之前**的对象上，而不是"渲染后真的发出去的那串字节"⇒
//     拿它做回归基线必然假绿。（H3 的第二半 —— 本文件按渲染后字节取哈希）
//   - **时间/随机未制度化**：`now()`/随机数一旦混进提示（尤其在"提醒字串"这种代码注入里），
//     同一份输入两次渲染就得到两个提示 ⇒ **回放与回归在定义上不成立**（H6）。本文件把
//     "时钟只进事件、不进提示"变成一条可判定守卫（prompt_impurity），并把 clock_iso/request_seed
//     制度化到每个请求。
//
// 分段口径（写死，读侧据此复算；"分段账目"与"发送顺序"是两件事，不许混）：
//
//	segments（账目段，固定四段、固定顺序，**永远齐全**——空段落 tokens:0 而不是缺席）：
//	  system  —— 实际发出的系统提示原文（sysPrompt 原样字节）
//	  tools   —— 实际发出的工具定义（逐条 JSON，键序由 encoding/json 保证稳定）
//	  history —— 实际发出的历史消息（role/content[/reasoning_content]）
//	  memory  —— 记忆块原文（**嵌在 system 的 volatile 层里** ⇒ 账目上 memory ⊂ system，允许重叠）
//
//	rendered_prefix_hash = sha256(render(system) ‖ render(history) ‖ render(tools))
//	  —— 对**渲染后实际发送的字节**取哈希，顺序按实际请求体的语义顺序（system 在前、历史居中、工具定义在体尾）。
//	     （不按 map 键序：body 是 map，encoding/json 按键名字典序输出；"prefix"要的是模型看到的先后。）
//	template_hash        = sha256(render(system 去掉记忆块) ‖ render(history) ‖ render(tools))
//	  —— **与上面同一对象类**（同含历史与工具），唯一差别是系统提示里是否含动态注入块。
//	     两者不等 ⇔ 存在模板之外的动态注入（当前实现里唯一的动态注入是记忆块），
//	     且 segments 里 memory 段的 sha256 直接指出"差的是它"（H3 逐字要求）。
//
//	口径边界（如实）：history 每轮天然在变 ⇒ 两个哈希每轮都会变，**"是否相等"**才是注入判据，
//	不要把"哈希变了"读成"有动态注入"。若记忆块文本在系统提示里找不到（例如取块发生在提示冻结之后），
//	落 memory_embedded=false（= 本次**没判准**，读侧别当"无注入"用）。
//
// 纪律（与 obs.go 文件头三条铁律同源）：
//  1. **观测绝不改变对话行为**：本文件只读入参、只写盘；不改一个字节的提示、不阻断、不重注入。
//  2. **best-effort**：写失败只记日志；入口自带 recover 兜底（观测面绝不许把对话搞崩）。
//  3. **不落原文**：只落分段指纹/长度/估算 token、时钟与种子、以及（截断的）命中片段；
//     提示正文与消息正文一个字都不落。
package chat

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"
)

// ── 事件名与记录种类（非空 event_name ⇒ 这条是事件）──
const (
	// obsKindPrompt — 本族记录的种类。**不占用既有 kind**（turn/compact/tool/behavior）：
	// "这次请求发出去的提示长什么样、还认不认得那些硬约束"与"这轮跑了多久/调了什么工具"是不同的问题，
	// 按 kind 过滤的既有读侧（长跑报告、工具统计）不受影响；按 event_name 找事件的一律能找到。
	obsKindPrompt = "prompt"

	// obsEventConstraintCheck — T3.2：每次请求一条（H2 逐字事件名）。
	// 即使登记表为空也落（它同时是 T3.3 分段账与 T3.5 时钟/种子的落点——**每请求一条账**）。
	obsEventConstraintCheck = "constraint_check"
	// obsEventConstraintMissingAlert — T3.2：missing 非空时的告警事件（**不阻断请求**）。
	obsEventConstraintMissingAlert = "constraint_missing_alert"
	// obsEventPromptImpurity — T3.5：提示纯净度守卫命中（提醒字串等代码注入里出现时间戳/随机形态）。
	obsEventPromptImpurity = "prompt_impurity"
)

// PromptSegmentObs — 一个分段的账目（H3 逐字三字段 + 两个便于复算的辅助量）。
type PromptSegmentObs struct {
	ID     string `json:"id"`     // system | tools | history | memory（闭集，固定四段）
	SHA256 string `json:"sha256"` // 该分段**渲染后字节**的 sha256（十六进制全串）
	Tokens int    `json:"tokens"` // estimateTokens(渲染字节) —— 与压缩阈值**同一估算器**（口径只此一处）
	Bytes  int    `json:"bytes"`  // 渲染字节数（UTF-8 字节；token 估算器是中文感知的，字节数与 char 数都不够）
}

// PromptLedgerObs — T3.3+T3.5 事实块（每请求一条）。
type PromptLedgerObs struct {
	Segments           []PromptSegmentObs `json:"segments"`
	RenderedPrefixHash string             `json:"rendered_prefix_hash"`
	TemplateHash       string             `json:"template_hash"`
	DynamicInjection   bool               `json:"dynamic_injection"` // = 两哈希不等（H3 逐字判据）
	DynamicParts       []string           `json:"dynamic_parts"`     // 模板外的动态注入段（当前只可能是 memory）
	MemoryEmbedded     bool               `json:"memory_embedded"`   // 记忆块是否确实嵌在系统提示里（false ⇒ 本次没判准）
	ClockISO           string             `json:"clock_iso"`         // T3.5：宿主注入的请求时钟（UTC RFC3339Nano）
	RequestSeed        int64              `json:"request_seed"`      // T3.5：本请求种子（可复算，见 NewRequestIdentity）
}

// PromptImpurityObs — T3.5 提示纯净度守卫的事实块。
type PromptImpurityObs struct {
	Host     string   `json:"host"`     // 命中位置（闭集：system_template | tools | reminder）
	Patterns []string `json:"patterns"` // 命中的形态名（低基数：rfc3339/iso_date/clock_sec/cjk_date/epoch_ms/now_call/rand_call）
	Sample   string   `json:"sample"`   // 首个命中片段（截断 ≤60）
	Excerpt  string   `json:"excerpt"`  // 命中处上下文（截断 ≤120）——"可行动"：看得见是哪句提醒在漏时间
	SHA256   string   `json:"sha256"`   // 被扫文本整体指纹（不落全文）
}

// ── T3.5 时钟与种子制度化 ──────────────────────────────────────────────

// RequestIdentity — 一个请求的"制度化身份"：时钟 + 种子。
//
// 语义（写死，回放/回归据此复现）：
//   - ClockISO：宿主注入的请求时钟（UTC，RFC3339Nano）。**它只进事件，不进提示**——
//     提示里要时间必须走"宿主注入变量并记录该值"或"工具结果"两条合法通道（H6）。
//   - Seed：本请求的种子，由 sha256(会话 ‖ 轮次 ‖ 时钟) 前 8 字节取非负值得出 ⇒ **可复算**：
//     回放时把录制的 clock_iso/request_seed 一起注入（FixedRequestIdentity），同一份输入就得到同一支账。
//   - ⚠ 边界（如实）：本批**只把种子制度化进事件**，未把它注入采样器（改采样器会改变模型输出，
//     属"观测改变行为"，越界；llama.cpp 侧的 seed 属 G3，另立项）。故请求体字节零改动。
type RequestIdentity struct {
	ClockISO string `json:"clock_iso"`
	Seed     int64  `json:"request_seed"`
}

// NewRequestIdentity — 造一个真实请求的身份（时钟取当前 UTC；种子由三元组推导）。
func NewRequestIdentity(sessionID string, round int) RequestIdentity {
	return identityFor(sessionID, round, time.Now().UTC())
}

// identityFor — 纯函数（时钟注入 ⇒ 可测、可在回放里复现）
func identityFor(sessionID string, round int, now time.Time) RequestIdentity {
	clock := now.UTC().Format(time.RFC3339Nano)
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%d\x00%s", sessionID, round, clock)))
	seed := int64(binary.BigEndian.Uint64(sum[:8]) & 0x7fffffffffffffff) // 非负：LLM seed 惯例
	return RequestIdentity{ClockISO: clock, Seed: seed}
}

// FixedRequestIdentity — 回放/用例入口：直接把录制值钉进本次身份（**不读墙上时钟**）。
func FixedRequestIdentity(clockISO string, seed int64) RequestIdentity {
	return RequestIdentity{ClockISO: clockISO, Seed: seed}
}

// ── 分段渲染（确定性：同输入 ⇒ 同字节）────────────────────────────────

// segmentIDs — 账目段的固定闭集与固定顺序（**永远齐全**：空段落 tokens:0/bytes:0，不缺席）
const (
	segmentSystem  = "system"
	segmentTools   = "tools"
	segmentHistory = "history"
	segmentMemory  = "memory"
)

// segJoinSep — 分段拼接分隔符（\x00 在正常提示文本里不出现；用来防"相邻段拼起来正好等于另一段"的串扰）
const segJoinSep = "\x00"

// sha256HexOf — 十六进制 sha256 全串（本文件唯一指纹口径）
func sha256HexOf(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// renderToolsSegment — 工具定义渲染：逐条 JSON（encoding/json 对 map 键**排序输出** ⇒ 确定性），一行一条。
func renderToolsSegment(tools []map[string]any) string {
	if len(tools) == 0 {
		return ""
	}
	var b strings.Builder
	for _, t := range tools {
		j, err := json.Marshal(t)
		if err != nil {
			continue // 序列化不了的单条跳过（观测面不为一条坏定义整体失败）
		}
		b.Write(j)
		b.WriteByte('\n')
	}
	return b.String()
}

// promptValueString — 把消息里的任意值渲染成稳定字符串（字符串原样；其它类型走 JSON）
func promptValueString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	default:
		if j, err := json.Marshal(v); err == nil {
			return string(j)
		}
		return fmt.Sprintf("%v", v)
	}
}

// renderHistorySegment — 历史渲染：逐条 "role\x00content[\x00reasoning_content]\n"。
// 只取这三样：它们是**决定提示语义**的字段（tools 定义在 tools 段；消息里的其它键不入账——口径写死）。
func renderHistorySegment(msgs []map[string]any) string {
	if len(msgs) == 0 {
		return ""
	}
	var b strings.Builder
	for _, m := range msgs {
		role, _ := m["role"].(string)
		b.WriteString(role)
		b.WriteString(segJoinSep)
		b.WriteString(promptValueString(m["content"]))
		if rc, ok := m["reasoning_content"]; ok {
			b.WriteString(segJoinSep)
			b.WriteString(promptValueString(rc))
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// splitSystemMemory — 从系统提示里**逐字剔除**记忆块（返回模板与是否剔除成功）。
// 判据：记忆块是 BuildTieredSystemPrompt 的 volatile 层里原样拼进来的 ⇒ 拿得到原文就能逐字剔除；
// 拿不到（记忆为空/提示冻结后记忆又变过）⇒ 模板 = 原文，并报 false（读侧据此知道"没判准"）。
func splitSystemMemory(system, memory string) (template string, embedded bool) {
	if memory == "" {
		return system, false
	}
	if i := strings.Index(system, memory); i >= 0 {
		return system[:i] + system[i+len(memory):], true
	}
	return system, false
}

// ── 提示纯净度守卫（T3.5 的"可判定"那半）────────────────────────────────
//
// 判据（写死）：**代码注入进提示的字串**里不得出现时间戳/时随机形态。
// 扫描范围（闭集，宿主名即读数）：
//
//	system_template —— 系统提示去掉记忆块后的**模板部分**（基础指令 + 身份 + 工具清单说明）——代码写的，必须纯净
//	tools           —— 工具定义渲染——代码写的，必须纯净
//	reminder        —— 本轮额外注入的提醒字串（插话/纠正/引导）——代码写的，必须纯净
//
// 豁免（H6 逐字给的合法通道，**不扫**，且豁免理由写在调用处注释里）：
//
//	memory / history（工具结果、用户与模型的正文）—— 时间是允许从"工具结果"进入提示的；
//	对它们扫会把"工具查到的日期"全部报成脏，那守卫会被关掉（守卫被关掉 = 真盲区）。
var promptImpurityPatterns = []struct {
	Name string
	Re   *regexp.Regexp
}{
	{"rfc3339", regexp.MustCompile(`\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}(:\d{2})?`)},
	{"iso_date", regexp.MustCompile(`\d{4}-\d{2}-\d{2}`)},
	{"clock_sec", regexp.MustCompile(`\d{1,2}:\d{2}:\d{2}`)},
	{"cjk_date", regexp.MustCompile(`\d{4}\s*年\s*\d{1,2}\s*月\s*\d{1,2}\s*日`)},
	{"epoch_ms", regexp.MustCompile(`\b1[6-9]\d{11}\b`)}, // unix 毫秒（2020-09～2033 区间；窄窗防误报）
	{"now_call", regexp.MustCompile(`\bnow\(\)|\btime\.Now\(\)|\bDate\.now\(\)`)},
	{"rand_call", regexp.MustCompile(`\bmath/rand\b|\buuid\.New\b|\brandomUUID\b|\bUUIDv4\b`)},
}

// promptImpurityMaxEvents — 单次检查里最多落几条（防"一句话里十个时间戳"把观测文件打爆）
const promptImpurityMaxEvents = 8

// promptImpuritySampleMax / ExcerptMax — 命中片段与其上下文的上限（截断，不落全文）
const (
	promptImpuritySampleMax  = 60
	promptImpurityExcerptMax = 120
)

// PromptPuritySource — 一个待扫的代码注入字串。
// TimeAllowed=true ⇒ H6 明列的合法通道（工具结果/宿主注入变量），**不扫**（豁免理由由调用方声明）。
type PromptPuritySource struct {
	Host        string
	Text        string
	TimeAllowed bool
}

// scanPromptImpurity — 扫一个宿主字串，命中则产出事实块（多个形态按闭集顺序去重）。
func scanPromptImpurity(src PromptPuritySource) *PromptImpurityObs {
	if src.Text == "" || src.TimeAllowed {
		return nil
	}
	var names []string
	firstMatch := ""
	excerpt := ""
	for _, p := range promptImpurityPatterns {
		loc := p.Re.FindStringIndex(src.Text)
		if loc == nil {
			continue
		}
		names = append(names, p.Name)
		if firstMatch == "" {
			firstMatch = p.Re.FindString(src.Text)
			lo := loc[0] - 30
			if lo < 0 {
				lo = 0
			}
			hi := loc[1] + 60
			if hi > len(src.Text) {
				hi = len(src.Text)
			}
			excerpt = src.Text[lo:hi]
		}
	}
	if len(names) == 0 {
		return nil
	}
	return &PromptImpurityObs{
		Host:     src.Host,
		Patterns: names,
		Sample:   trunca(firstMatch, promptImpuritySampleMax),
		Excerpt:  trunca(excerpt, promptImpurityExcerptMax),
		SHA256:   sha256HexOf(src.Text),
	}
}

// ── 唯一入口：一次请求一次账 ───────────────────────────────────────────

// PromptRender — 一次「**实际发给模型的提示**」的真实入参（调用方给真字节，观测面不许自己拼提示）。
//
// 为什么必须由调用方给（H3 的第二半）：只有调用点才知道"真正发出去的是哪串字节"。
// 传进来的 System/History/Tools 必须与随后交给推理客户端的**同一份**对象（同一个变量），
// 否则指纹对象又错了。
type PromptRender struct {
	Session string
	Round   int
	Model   string

	System  string           // 实际发出的系统提示原文
	Memory  string           // 嵌在系统提示 volatile 层的记忆块原文（拿不到 ⇒ ""，落 memory_embedded=false）
	Tools   []map[string]any // 实际发出的工具定义（可为 nil）
	History []map[string]any // 实际发出的历史消息（含本轮用户消息）

	// Reminders — 本轮额外注入的**代码写**的提醒字串（插话/纠正/引导）。只用于纯净度扫描（不入分段账）。
	Reminders []string

	// Sources — 这些历史消息的**原消息**（含 ID）——用于登记约束（拿不到就登记不了：
	// source_msg_id 不许编造）。可传 nil（则本轮不登记新约束，只做检索）。
	Sources []*Message

	Identity RequestIdentity
}

// ObservePromptCheck — T3.2 + T3.3 + T3.5 的唯一落账入口（best-effort；绝不外抛/阻断/panic）。
//
// 落三件事（同一入口，读侧按 event_name 分派）：
//  1. `constraint_check` —— 约束保留率（total/present/missing_ids）+ 分段落账（segments/两个哈希）
//     + 时钟与种子（clock_iso/request_seed）。**每请求一条**（登记表为空也落：它是本请求的账）。
//  2. `constraint_missing_alert` —— 有缺失时的告警（带可行动明细；**不阻断请求**）。
//  3. `prompt_impurity` —— 纯净度守卫命中时每条一个（宿主 + 形态 + 片段）。
func ObservePromptCheck(r PromptRender) {
	defer func() { // 观测面绝不许把对话搞崩（与 obs.go 铁律①同源）
		if v := recover(); v != nil {
			log.Printf("⚠️ prompt_check: panic recovered (对话不受影响): %v", v)
		}
	}()

	// ① 登记：本轮活动窗口里的硬约束入册（幂等；表满拒新，不挤老约束）
	indexConstraints(r.Session, r.Sources)

	// ② 分段账（渲染后字节——与真正发出去的是同一份对象）
	sysRender := r.System
	toolsRender := renderToolsSegment(r.Tools)
	histRender := renderHistorySegment(r.History)
	template, memEmbedded := splitSystemMemory(r.System, r.Memory)

	segments := []PromptSegmentObs{
		segObs(segmentSystem, sysRender),
		segObs(segmentTools, toolsRender),
		segObs(segmentHistory, histRender),
		segObs(segmentMemory, r.Memory),
	}
	// 发送序：system 在前、历史居中、工具定义在体尾（口径见文件头）
	renderedPrefixHash := sha256HexOf(sysRender + segJoinSep + histRender + segJoinSep + toolsRender)
	templateHash := sha256HexOf(template + segJoinSep + histRender + segJoinSep + toolsRender)
	dynamicParts := []string{}
	if r.Memory != "" && memEmbedded {
		dynamicParts = append(dynamicParts, segmentMemory)
	}

	// ③ 约束存在性检索（H2）：在**本次实际发出的提示**上检索 canonical 或其指纹
	cs := SessionConstraints(r.Session)
	check := checkConstraints(cs, promptHaystack(r))

	ledger := &PromptLedgerObs{
		Segments:           segments,
		RenderedPrefixHash: renderedPrefixHash,
		TemplateHash:       templateHash,
		DynamicInjection:   renderedPrefixHash != templateHash,
		DynamicParts:       dynamicParts,
		MemoryEmbedded:     memEmbedded,
		ClockISO:           r.Identity.ClockISO,
		RequestSeed:        r.Identity.Seed,
	}
	seed := r.Identity.Seed
	base := ObsRecord{
		Kind: obsKindPrompt, Session: r.Session, Round: r.Round, Model: r.Model,
		EventName:   obsEventConstraintCheck,
		Constraints: &check,
		Prompt:      ledger,
		ClockISO:    r.Identity.ClockISO,
		RequestSeed: &seed,
	}
	obsWrite(base)

	// ④ 告警（missing 非空）——**不阻断请求**：观测面只报告，处置留给上层/人
	if check.Alert {
		alert := base
		alert.EventName = obsEventConstraintMissingAlert
		alert.ConstraintMissing = missingDetailsOf(cs, check.MissingIDs)
		obsWrite(alert)
	}

	// ⑤ 纯净度守卫（T3.5）：只扫**代码注入**的字串（豁免记忆/历史，理由见 promptImpurityPatterns 上方注释）
	sources := []PromptPuritySource{
		{Host: "system_template", Text: template},
		{Host: "tools", Text: toolsRender},
	}
	for _, rem := range r.Reminders {
		sources = append(sources, PromptPuritySource{Host: "reminder", Text: rem})
	}
	emitted := 0
	for _, src := range sources {
		if emitted >= promptImpurityMaxEvents {
			break
		}
		imp := scanPromptImpurity(src)
		if imp == nil {
			continue
		}
		rec := base
		rec.EventName = obsEventPromptImpurity
		rec.Impurity = imp
		obsWrite(rec)
		emitted++
	}
}

// segObs — 一个分段的账目（空段也落：tokens:0/bytes:0 是**测到的 0**，不是"没有这段"）
func segObs(id, render string) PromptSegmentObs {
	return PromptSegmentObs{
		ID:     id,
		SHA256: sha256HexOf(render),
		Tokens: estimateTokens(render),
		Bytes:  len(render),
	}
}

// promptHaystack — 约束检索栈：把**实际发出的全部提示文本**规范化后拼成一串（两侧同一规范化）。
// 顺序：system ‖ tools ‖ history ‖ reminders（顺序不影响子串检索，只影响可复现性 ⇒ 固定写死）。
func promptHaystack(r PromptRender) string {
	parts := []string{r.System, renderToolsSegment(r.Tools), renderHistorySegment(r.History)}
	parts = append(parts, r.Reminders...)
	return CanonicalConstraint(strings.Join(parts, "\n"))
}
