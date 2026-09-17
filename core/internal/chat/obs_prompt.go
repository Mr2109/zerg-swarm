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
// ── 2026-09-18 两处口径改正（都来自实测，不是理论）──────────────────────
//
// ① **纯净度判据**（缺陷③：误报）：原口径是"命中形态即报警"，实测里砸出一堆误报——
//
//	命中在 host=system_template，样本却是我们自己指令里的**静态示例**「日期用 ISO 8601（2026-09-11）」
//	（那句住在 ffp.Conventions 常量里，是模板源的一部分）。误报的代价是守卫被关掉（守卫被关掉 = 真盲区）。
//	新判据（写死）：**在实际发送的前缀上匹配到 且 该串不在模板源里 ⇒ 才报警**。
//	模板源 = ①调用方显式声明（PromptRender.TemplateSource，最准）②没有声明 ⇒ 本文件在**判定时采集**
//	的"代码侧字面量集合"（promptTemplateSource：ffp.Conventions + 装配处字面量清单）。
//	**已知限制（如实）**：系统提示的 base 常量住在 internal/api（chatSystemPrompt）——本包拿不到它的原文，
//	也拿不到调用方在别处拼进模板的字面量 ⇒ 那些字面量里的静态示例仍会被报脏；缓解就是①那条声明路径。
//	判据的固有代价：与模板源里**同形同值**的活时间戳判别不出来（会漏报那一例）——用声明收窄，别用放宽收窄。
//
// ② **约束重注入（治本）**（缺陷②：真缺失只报警不治本）：本文件是唯一入口，重注入在这里装配。
//
//	口径：ObservePromptCheck **只描述调用点给的字节**（H3 不变：segments/两个哈希/prompt_impurity 全部按入参算），
//	重注入是**产出**——`PromptCheckResult.System` 就是重注入后的系统提示，装配点用它当本请求真正发送的系统提示
//	（一行接线）；事件里落 reinjected / reinject_reason / reinjected_ids / injection_count / injection_id(+sha256)
//	/ injection_round，**这些是"这次入口产出了什么"的事实**，不代表调用方已经采纳（采纳后下一轮的入参会含注入块，
//	于是 reinject_stripped=true、且 template_stripped_hash 与上一轮 template_hash 相等 ⇒ 读侧能区分
//	"模板真的变了"与"只是重注入"）。
//
// 纪律（与 obs.go 文件头三条铁律同源）：
//  1. **入口不自作主张**：本文件读入参、算账、写盘，并把重注入后的**字节**交回调用方（采纳与否在调用方）；
//     它自己不发送、不改别人持有的字符串、不阻断请求。
//  2. **best-effort**：写失败只记日志；入口自带 recover 兜底（观测面绝不许把对话搞崩）。
//  3. **不落原文**：只落分段指纹/长度/估算 token、时钟与种子、以及（截断的）命中片段；
//     提示正文与消息正文一个字都不落（注入块原文也不落——只落它的指纹与所注入的约束 id）。
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
	"sync"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/ffp"
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

	// ── 重注入的可解释性（2026-09-18；H3"哈希变了要说清为什么"，见文件头口径②）──
	// 判读（写死，读侧照此复算）：
	//   TemplateStrippedHash —— 把**我们注入过的块**逐字剔除后**再算**的模板哈希（与 template_hash 同一对象类：
	//     模板去掉记忆块 ‖ history ‖ tools）。
	//   ReinjectStripped     —— 本次入参里**确实**含我们注入过的块（⇒ 上一轮的 PromptCheckResult.System 被采纳了）。
	// 于是：rendered_prefix_hash 变了不是问号 ——
	//   · template_stripped_hash 与上一轮相等（而 template_hash 变了）⇒ **只是重注入**（模板字节没变）；
	//   · template_stripped_hash 也变了 ⇒ **模板真的变了**（或注入了不同内容的块，那看 injection_id 就够了）。
	TemplateStrippedHash string `json:"template_stripped_hash"`
	ReinjectStripped     bool   `json:"reinject_stripped"`
}

// PromptImpurityObs — T3.5 提示纯净度守卫的事实块。
//
// 判据（2026-09-18 改正，见文件头口径①）：**在实际发送的前缀上匹配到 且 该串不在模板源里 ⇒ 才报警**。
// 于是这个块只在"真的脏"时出现：命中串必须在**我们写死的字面量**里找不到（= 它不是模板源里的静态示例）。
type PromptImpurityObs struct {
	Host     string   `json:"host"`     // 命中位置（闭集：system_template | tools | reminder）
	Patterns []string `json:"patterns"` // 命中的形态名（低基数：rfc3339/iso_date/clock_sec/cjk_date/epoch_ms/now_call/rand_call）
	Sample   string   `json:"sample"`   // 首个**不在模板源里**的命中片段（截断 ≤60）
	Excerpt  string   `json:"excerpt"`  // 命中处上下文（截断 ≤120）——"可行动"：看得见是哪句提醒在漏时间
	SHA256   string   `json:"sha256"`   // 被扫文本整体指纹（不落全文）
	// 判据②的取证（**只在真的要报警时**落——全被静态示例规则放行 ⇒ 一条事件都不落，否则"不报警"就说不通了）：
	// 模板源指纹（不落源原文）+ 本宿主里有多少个形态命中因"在模板源里"被放行。
	// 为什么要落它：否则读侧分不清"守卫正常只是判得细"与"守卫被改宽了"——那正是误报改正后的新盲区。
	TemplateSourceSHA256 string `json:"template_source_sha256,omitempty"`
	SuppressedStaticHits int    `json:"suppressed_static_hits,omitempty"`
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
// 判据（2026-09-18 改正后写死）：**在实际发送的前缀上匹配到 且 该串不在模板源里 ⇒ 才报警**。
// 为什么改（实测缺陷③）：老口径"命中形态即报警"把**我们自己写的静态示例**也报成脏
// ——命中在 host=system_template，样本是 ffp.Conventions 里那句「日期用 ISO 8601（2026-09-11）」。
// 误报的直接后果是守卫被关掉，而"守卫被关掉 = 真盲区"（见下"豁免"段的同一逻辑）。
//
// 扫描范围（闭集，宿主名即读数）：
//
//	system_template —— 系统提示去掉记忆块后的**模板部分**（基础指令 + 身份 + 工具清单说明）——代码写的，必须纯净
//	tools           —— 工具定义渲染——代码写的，必须纯净
//	reminder        —— 本轮额外注入的提醒字串（插话/纠正/引导）——代码写的，必须纯净
//
// 模板源（判据②的右边那一半，见文件头口径①）：
//
//	PromptPuritySource.TemplateSource —— 调用方声明优先；空 ⇒ promptTemplateSource() 判定时采集的代码侧字面量集合。
//	"在模板源里" = 命中串**逐字**能在它里面找到 ⇒ 那是我们自己写死的静态示例（示例永远是示例，不是活时间戳）。
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
	// TemplateSource — 本宿主的"模板源"（= 我们自己写死的字面量集合；调用方声明优先，空 ⇒ 判定时采集）。
	// 空 ⇒ **不豁免**：判据退化为老口径"匹配即报警"（这保证"没给源"不会被读成"什么都能放行"）。
	TemplateSource string
}

// promptSampleInTemplateSource — 判据②：命中串是否**在模板源里**（= 静态示例 ⇒ 不算脏）。
// 逐字比对（不做规范化）：模板源就是字节，示例也是字节——一边折叠一边比会把"活值恰好被折叠掉"变成漏报。
func promptSampleInTemplateSource(sample, source string) bool {
	if sample == "" || source == "" {
		return false
	}
	return strings.Contains(source, sample)
}

// scanPromptImpurity — 扫一个宿主字串，**只对"不在模板源里"的命中**产出事实块（判据②）。
// 返回 nil 的三种情形要分清（读侧别混）：① 文本为空/属豁免通道；② 一个形态都没命中；
// ③ 命中了但每一个都在模板源里（静态示例）——第③种是本条改动的目的，**不落事件**。
func scanPromptImpurity(src PromptPuritySource) *PromptImpurityObs {
	if src.Text == "" || src.TimeAllowed {
		return nil
	}
	var names []string
	firstMatch := ""
	excerpt := ""
	suppressed := 0
	for _, p := range promptImpurityPatterns {
		dirtyIdx := -1
		dirtySample := ""
		for _, loc := range p.Re.FindAllStringIndex(src.Text, -1) {
			s := src.Text[loc[0]:loc[1]]
			if promptSampleInTemplateSource(s, src.TemplateSource) {
				suppressed++ // 静态示例：我们自己写死的（含在模板源里）⇒ 不算脏（缺陷③的误报就在这里）
				continue
			}
			dirtyIdx, dirtySample = loc[0], s
			break
		}
		if dirtyIdx < 0 {
			continue
		}
		names = append(names, p.Name)
		if firstMatch == "" {
			firstMatch = dirtySample
			lo := dirtyIdx - 30
			if lo < 0 {
				lo = 0
			}
			hi := dirtyIdx + len(dirtySample) + 60
			if hi > len(src.Text) {
				hi = len(src.Text)
			}
			excerpt = src.Text[lo:hi]
		}
	}
	if len(names) == 0 {
		return nil
	}
	imp := &PromptImpurityObs{
		Host:     src.Host,
		Patterns: names,
		Sample:   trunca(firstMatch, promptImpuritySampleMax),
		Excerpt:  trunca(excerpt, promptImpurityExcerptMax),
		SHA256:   sha256HexOf(src.Text),
	}
	if src.TemplateSource != "" {
		imp.TemplateSourceSHA256 = sha256HexOf(src.TemplateSource)
		imp.SuppressedStaticHits = suppressed
	}
	return imp
}

// ── 模板源（判据②的右边那一半）────────────────────────────────────────

// promptTemplateLiterals — 装配处字面量清单（与 chat_prompt.go 三档模板**同源的只读副本**）。
// 只用于判据②的"静态示例"识别，**不参与渲染** ⇒ 它漂移的最坏后果 = 一处静态示例被误报（退回老口径），
// 不会改变任何一个提示字节。
var promptTemplateLiterals = []string{
	"# 你的身份",
	"- 你当前运行在虫族本地模型集群——Mr2109的对话助手——不要调查或质疑自己的身份。",
	"# 会话环境",
}

var (
	promptTemplateSrcOnce sync.Once
	promptTemplateSrc     string
)

// promptTemplateSource — 模板源（**代码侧字面量集合**，判定时采集一次并缓存）。
//
// 口径与限制（如实写死，别顺口美化）：
//   - 现在能直接取到的是：ffp.Conventions（执行接口约定常量——静态示例「日期用 ISO 8601（2026-09-11）」就住在这句里）
//   - promptTemplateLiterals（装配处字面量清单）。采集是**判定时**做的（不是从 template_hash 反推：
//     哈希回不到原文，拿渲染后的模板当自己的源 ⇒ 判据②恒真，守卫等于被改宽成永远不报）。
//   - **已知限制**：系统提示的 base 常量住在 internal/api（chatSystemPrompt），本包（chat）拿不到它的原文，
//     也拿不到调用方在别处拼进模板的字面量 ⇒ 那些字面量里的静态示例仍会被报脏。缓解 = 调用方把模板源
//     **显式声明**进 PromptRender.TemplateSource（一行接线），那条路不受此限。
//   - 判据的固有代价：与模板源里**同形同值**的活时间戳判别不出来（会漏报那一例）。这是"用静态源区分静态/动态"
//     的必然代价；要收窄就补声明，**不许**靠放宽形态闭集（那是把守卫改成永远不报）。
func promptTemplateSource() string {
	promptTemplateSrcOnce.Do(func() {
		parts := append([]string{ffp.Conventions}, promptTemplateLiterals...)
		promptTemplateSrc = strings.Join(parts, "\n")
	})
	return promptTemplateSrc
}

// promptTemplateSourceOf — 取本次判定用的模板源：调用方声明优先，空 ⇒ 采集值。
func promptTemplateSourceOf(declared string) string {
	if declared != "" {
		return declared
	}
	return promptTemplateSource()
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

	// TemplateSource — 纯净度判据②的"模板源"（调用方**可选**声明；见文件头口径①）。
	// 声明了 ⇒ 以声明为准（把模板原文交给观测面 ⇒ 静态示例不会被误报）；
	// 空 ⇒ 用 obs_prompt.go 判定时采集的代码侧字面量集合（已知限制见 promptTemplateSource 注释）。
	TemplateSource string

	// T3.4 prompt 版本外键（H4）：本次提示的**装配处名字**与**发布标签**。
	// 两者是调用方声明的事实 —— 声明不了就传空 ⇒ 对应键**缺席**（不写空串冒充，见 obs_prompt_version.go 口径②）。
	// prompt_version **不用传**：它是渲染后模板的 sha256 前 8 字节，由观测面在真字节上算出（口径①）。
	PromptName  string
	PromptLabel string

	// T3.6 提示脚手架策略（H7）：调用方声明的配置事实；**nil ⇒ 四个策略键一个都不落**（不写 0/空串冒充）。
	Strategy *PromptStrategyIn

	// Sources — 这些历史消息的**原消息**（含 ID）——用于登记约束（拿不到就登记不了：
	// source_msg_id 不许编造）。可传 nil（则本轮不登记新约束，只做检索）。
	Sources []*Message

	Identity RequestIdentity
}

// PromptCheckResult — 唯一入口的返回值：装配点据此**采纳**重注入（一行接线：`sysPrompt = res.System`）。
//
// 语义（写死）：
//
//	System      —— 重注入后的系统提示（本次没发生重注入 ⇒ 与入参**逐字节相同**）。装配点把它当本请求真正
//	               发送的系统提示 ⇒ "头尾各一份"才真的进了上下文（治本那半的落点）。⚠ 若装配点不采纳，
//	               System 与入参相同（本函数不改别人持有的字符串）。
//	Reinjection —— 这次重注入的事实（原因/条数/指纹/本会话累计次数/轮次/时钟）
//	Check       —— constraint_check 的事实块（含重注入字段）——**描述的是入参字节**（诊断口径不动：
//	               治本不许把"这一刻缺了什么"弄瞎，见 checkConstraints 注释）
//	Ledger      —— 分段落账块（同样描述入参字节；template_stripped_hash 用来解释哈希变化）
type PromptCheckResult struct {
	System      string
	Reinjection Reinjection
	Check       ConstraintCheckObs
	Ledger      *PromptLedgerObs
}

// ObservePromptCheck — T3.2 + T3.3 + T3.5 的唯一落账入口（best-effort；绝不外抛/阻断/panic）。
//
// 落三件事（同一入口，读侧按 event_name 分派）：
//  1. `constraint_check` —— 约束保留率（total/present/missing_ids）+ 重注入事实 + 分段落账（segments/两个哈希）
//     + 时钟与种子（clock_iso/request_seed）。**每请求一条**（登记表为空也落：它是本请求的账）。
//  2. `constraint_missing_alert` —— 有缺失时的告警（带可行动明细；**不阻断请求**）。
//  3. `prompt_impurity` —— 纯净度守卫命中时每条一个（宿主 + 形态 + 片段；判据②见文件头口径①）。
//
// 返回值见 PromptCheckResult（重注入后的系统提示就在里面）。
func ObservePromptCheck(r PromptRender) (res PromptCheckResult) {
	res.System = r.System // 默认：不重注入 ⇒ 原样退回（panic 时调用方也不会拿到空串）
	defer func() {        // 观测面绝不许把对话搞崩（与 obs.go 铁律①同源）
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
	// 重注入可解释性（H3）：把**我们注入过的块**剔掉再算一次模板哈希（同一对象类）。
	// 采纳了重注入的下一轮：template_hash 变了而 template_stripped_hash 与上一轮相等 ⇒ 是重注入，不是模板变了。
	templateStripped, reinjectStripped := stripReinjectBlocks(template)
	templateStrippedHash := sha256HexOf(templateStripped + segJoinSep + histRender + segJoinSep + toolsRender)

	// ③ 约束存在性检索（H2）：在**本次实际发出的提示**上检索 canonical 或其指纹
	cs := SessionConstraints(r.Session)
	check := checkConstraints(cs, promptHaystack(r))

	// ③′【治本】重注入（2026-09-18 缺陷②）：missing 的 must_survive ⇒ 头尾各补一份；
	// 没 missing 但到周期（默认每 4 轮）⇒ 定时补一次；幂等（先剔后放，见 constraints.go「重注入」节）。
	// 时钟用**本请求的制度化身份**（不读墙上时钟：渲染不吃时钟那条性质不许破——H6）。
	rej := ReinjectConstraints(r.System, cs, ReinjectOptions{
		Session: r.Session, Round: r.Round, ClockISO: r.Identity.ClockISO,
	})
	applyReinjectionToObs(&check, rej)
	res.System, res.Reinjection, res.Check = rej.Prompt, rej, check
	if rej.Injected {
		log.Printf("📌 constr_reinject: session %s round %d reason=%s 注入 %d 条（累计 %d；该会话 must_survive=%d）",
			r.Session, rej.Round, rej.Reason, rej.Count, rej.InjectionCount, len(cs))
	}

	ledger := &PromptLedgerObs{
		Segments:             segments,
		RenderedPrefixHash:   renderedPrefixHash,
		TemplateHash:         templateHash,
		DynamicInjection:     renderedPrefixHash != templateHash,
		DynamicParts:         dynamicParts,
		MemoryEmbedded:       memEmbedded,
		ClockISO:             r.Identity.ClockISO,
		RequestSeed:          r.Identity.Seed,
		TemplateStrippedHash: templateStrippedHash,
		ReinjectStripped:     reinjectStripped,
	}
	res.Ledger = ledger
	seed := r.Identity.Seed
	base := ObsRecord{
		Kind: obsKindPrompt, Session: r.Session, Round: r.Round, Model: r.Model,
		EventName:   obsEventConstraintCheck,
		Constraints: &check,
		Prompt:      ledger,
		ClockISO:    r.Identity.ClockISO,
		RequestSeed: &seed,
	}

	// ③″ T3.4 版本外键（H4）：版本对**模板**取指纹（渲染后系统提示去掉记忆块 ⇒ 只改记忆块不改版本），
	// 名字/标签是调用方声明的事实（声明不了 ⇒ 键缺席）。label→version 快照**随事件落盘**（事件是权威）。
	base.PromptVersion = PromptTemplateVersion(template)
	base.PromptName = r.PromptName
	base.PromptLabel = r.PromptLabel
	if r.PromptName != "" && r.PromptLabel != "" {
		RegisterPromptLabelVersion(r.PromptName, r.PromptLabel, base.PromptVersion)
		base.PromptLabelVersions = PromptLabelVersionsOf(r.PromptName)
	}

	// ③‴ T3.6 策略版本化（H7）：调用方没声明 ⇒ 四个键**一个都不落**（不写 0/空串冒充）；
	// 声明了 false 也照落（"没声明"与"声明了没有"必须可分）。
	base.StrategyID, base.StepIndex, base.HasAcceptanceCriteria, base.RequiresToolCallFirst =
		strategyFieldsOf(r.Strategy, r.Session)
	obsWrite(base)

	// ④ 告警（missing 非空）——**不阻断请求**：观测面只报告，处置已由上面的重注入给出（治本那半）
	if check.Alert {
		alert := base
		alert.EventName = obsEventConstraintMissingAlert
		alert.ConstraintMissing = missingDetailsOf(cs, check.MissingIDs)
		obsWrite(alert)
	}

	// ⑤ 纯净度守卫（T3.5 + 判据②）：只扫**代码注入**的字串（豁免记忆/历史，理由见 promptImpurityPatterns 上方注释）；
	// "在模板源里"的命中 = 我们自己写死的静态示例 ⇒ 放行（缺陷③的误报改正）。
	tmplSrc := promptTemplateSourceOf(r.TemplateSource)
	sources := []PromptPuritySource{
		{Host: "system_template", Text: template, TemplateSource: tmplSrc},
		{Host: "tools", Text: toolsRender, TemplateSource: tmplSrc},
	}
	for _, rem := range r.Reminders {
		sources = append(sources, PromptPuritySource{Host: "reminder", Text: rem, TemplateSource: tmplSrc})
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
	return res
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
