// obs_prompt_version.go — T3.4 **prompt 版本外键 + label→version 快照**（H4）
//   - T3.6 **提示脚手架策略版本化**（H7）
//
// 设计依据（逐字，取自 docs/01-设计/设计-内建调试版-v1.2.md 第〇节）：
//
//	「H4 缺 prompt 版本外键与 label→version 快照 ⇒ "这次劣化是哪个提示版本造成的"无法回答｜★★★｜
//	  记 `prompt_name/version/label` + 快照；回滚也能复现当时线上状态」。
//	「H7 **缺"策略本身"的版本化**——我们观察到的"大目标⇒只读不交差；小目标⇒真动手"属**提示脚手架差异**｜★★★｜
//	  把策略当 prompt 的 config 一起版本化：`strategy_id / step_index / 有无验收标准 / 是否要求先出 tool_call`」。
//
// 要治的盲区（改本文件前先读）：
//   - **「这次劣化是哪个提示版本造成的」现在答不出来**：事件里只有分段落哈希（T3.3 的 rendered_prefix_hash /
//     template_hash）——哈希能说"变了"，说不出"是哪一版"，更说不出"当时那一版挂着哪个 label"。
//     本文件给**模板**一个稳定版本标识（sha256 前 8 字节十六进制）并把 label→version 快照**随事件落盘**
//     ⇒ 回滚之后，翻当时那几行就能复现"那一刻线上跑的是哪版提示"（H4 的"回滚也能复现当时线上状态"）。
//   - **「给大目标⇒只读不交差；给一步一验的小目标⇒真动手」不可归因**：这是**提示脚手架（策略）**的差异，
//     而策略在事件里**一个字段都没有** ⇒ 事后只能凭印象。本文件把策略版本化：strategy_id（由**生效配置**
//     规范化后取指纹 ⇒ 配置改一个字节 id 就变）+ step_index（本会话第几步）+ 两个三态布尔
//     （有验收标准 / 要求先出工具调用）⇒ 这个差异从"感觉"变成**可查询的字段**。
//
// 口径（写死，读侧据此复算；与 obs.go 文件头三条铁律同源）：
//
//	① **版本标识对"模板"取指纹**：
//	     prompt_version = hex(sha256(渲染后系统提示 **去掉记忆块**)[:8])   —— 16 字符。
//	   与 T3.3 的 template_hash 同一对象（system-without-memory ‖ history ‖ tools）里的**模板那一半**，
//	   所以：改提示模板 ⇒ prompt_version 变；只改**记忆块**（模板外的动态注入）⇒ prompt_version **不变**
//	   （变的是 rendered_prefix_hash）。这正是 H3/H4 要的分工——"模板换版"与"动态注入"必须可分。
//
//	② **名字与标签由调用方声明，声明不了就缺席**：prompt_name 是装配处的稳定名字（如 chat.tiered_system）；
//	   prompt_label 是运行环境声明的发布标签（见 PromptLabelEnv）。两者的键**要么带着真实值出现，要么不出现**
//	   ——绝不写空串冒充（"没声明"与"声明了空串"必须可分，同 obs.go「0 与未知必须可分」）。
//
//	③ **label→version 快照**：每次观察把 (name,label)→version 记进进程内表并**随事件落盘**
//	   （prompt_label_versions）。**事件是权威**（回滚后翻当时的行即可复现当时线上状态），
//	   进程内表只是给"就地回查"用的便利（见 PromptLabelVersion）。表有界（长跑守护进程里名字/标签数无界）。
//
//	④ **策略是调用方声明的配置事实，不是从提示文本猜出来的**：本文件只做规范化 + 取指纹 + 记数。
//	   为什么不扫提示文本反推策略？——文本里没有机器可读的策略声明，靠形态猜出来的字段**看着有、实际不可信**
//	   （I1「探针本身要自证」）。宁可缺席，也不造一个读侧会当真的值。
//
//	⑤ **三态**：has_acceptance_criteria / requires_tool_call_first 是**指针** —— nil=缺席（没声明）、
//	   显式 false **照落**（声明了"没有"）。step_index 由观测面按**本会话的提示装配计数**推进
//	   （调用方给了显式值就用显式值）——"本会话第几步"是可数的事实，不是猜的。
//
// 纪律：观测面只读入参、只写盘；best-effort（写失败只记日志）；不落提示正文一个字（只落指纹/计数/名字/标签）。
package chat

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"sort"
	"strings"
	"sync"
)

// ── 闭集常量（改动即契约变更：读侧按这些名字挑行/取值）──────────────────

const (
	// PromptNameChatTieredSystem — 现有唯一的系统提示装配处（BuildTieredSystemPrompt 的三档装配）的稳定名字。
	// 它就是 H4 的 prompt_name：日后若有第二个装配处（子端/CA），各自给自己的名字，事件里即可分账。
	PromptNameChatTieredSystem = "chat.tiered_system"

	// PromptLabelEnv — 运行环境声明**发布标签**的变量名（如 dev / canary / prod）。
	// 仓内没有版本发布机制 ⇒ 标签只能由运行环境声明；**未声明 ⇒ label 键缺席**（不编造一个默认标签：
	// 编出来的"local"会让读侧以为是有人指定过的发布通道）。
	PromptLabelEnv = "ZERG_PROMPT_LABEL"

	// promptFingerprintHexLen — 版本标识长度：sha256 前 8 字节的十六进制 = 16 字符。
	// 与 PromptHash（chat_prompt.go，提示冻结/命中率基线）**同一口径**：两处都是"sha256 前 8 字节"，
	// 免得同一件事出现两种长度的指纹。
	promptFingerprintHexLen = 16
)

// ── ① 版本标识（对"模板"取指纹）────────────────────────────────────────

// PromptTemplateVersion — T3.4：模板版本标识 = hex(sha256(template)[:8])（16 字符）。
//
// 入参必须是**渲染后实际发出的系统提示去掉记忆块**的那串字节（调用点已由
// splitSystemMemory 逐字剔除记忆块得到 template，见 obs_prompt.go）——不是拼装前的对象：
// 对象错了，版本标识就只是在自证（H3 的第二半，同一个教训）。
//
// 决定论：纯函数、不含时钟/随机的字节 ⇒ 同一模板两次装配必得同一版本（用例①）。
func PromptTemplateVersion(template string) string {
	return promptFingerprintOf(template)
}

// promptFingerprintOf — 本文件唯一指纹口径（sha256 前 8 字节十六进制）
func promptFingerprintOf(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:8])
}

// ── ② 发布标签（由运行环境声明）────────────────────────────────────────

// PromptLabelFromEnv — 读运行环境声明的发布标签（未声明 ⇒ ""=缺席，不编造默认通道）。
func PromptLabelFromEnv() string {
	return strings.TrimSpace(os.Getenv(PromptLabelEnv))
}

// ── ③ label→version 快照（进程内表 + 随事件落盘）──────────────────────

// promptLabelNamesMax / promptLabelPerNameMax — 表容量（长跑守护进程里名字与标签数无界，表必须有界）。
// 满了淘汰**最久未登记**的名字（观测可容忍；内存不可无界——与 obs.go 的 obsTraceSessionsMax 同源口径）。
const (
	promptLabelNamesMax   = 256
	promptLabelPerNameMax = 32
)

// promptLabelBucket — 一个提示名下的 label→version 快照（带 LRU 序号，便于有界淘汰）
type promptLabelBucket struct {
	versions map[string]string
	order    []string // 登记顺序（满了从队头淘汰）
	seq      int64    // 名字级 LRU 序号
}

var (
	promptLabelMu  sync.Mutex
	promptLabelSeq int64
	promptLabelTab = map[string]*promptLabelBucket{}
)

// RegisterPromptLabelVersion — 记一条 (name,label)→version 快照（供回查；幂等：同键后写覆盖）。
//
// 纪律：名字/标签/版本三者缺一**不登记**（回查需要一个键，缺键的登记只会造出无从检索的行）。
// 返回是否登记成功（调用方一般不需要——观测面 best-effort）。
func RegisterPromptLabelVersion(name, label, version string) bool {
	name = strings.TrimSpace(name)
	label = strings.TrimSpace(label)
	if name == "" || label == "" || version == "" {
		return false
	}
	promptLabelMu.Lock()
	defer promptLabelMu.Unlock()
	b := promptLabelTab[name]
	if b == nil {
		if len(promptLabelTab) >= promptLabelNamesMax {
			promptLabelEvictOldestLocked()
		}
		b = &promptLabelBucket{versions: map[string]string{}}
		promptLabelTab[name] = b
	}
	promptLabelSeq++
	b.seq = promptLabelSeq
	if _, ok := b.versions[label]; !ok {
		if len(b.order) >= promptLabelPerNameMax {
			drop := b.order[0]
			b.order = b.order[1:]
			delete(b.versions, drop)
		}
		b.order = append(b.order, label)
	}
	if b.versions[label] == version {
		return true // 同值重记：无变化（不算新事实）
	}
	b.versions[label] = version
	return true
}

// PromptLabelVersion — 回查：某提示名下某标签**当前**指向哪一版（ok=false ⇒ 从没登记过，不编造）。
func PromptLabelVersion(name, label string) (string, bool) {
	promptLabelMu.Lock()
	defer promptLabelMu.Unlock()
	b := promptLabelTab[strings.TrimSpace(name)]
	if b == nil {
		return "", false
	}
	v, ok := b.versions[strings.TrimSpace(label)]
	return v, ok
}

// PromptLabelVersionsOf — 某提示名下的 label→version 快照（**副本**；无登记 ⇒ nil ⇒ 键缺席）。
// 它就是要落进事件的那份快照：回滚后翻当时的行，即可回答"那一刻线上哪几个通道各指哪版"。
func PromptLabelVersionsOf(name string) map[string]string {
	promptLabelMu.Lock()
	defer promptLabelMu.Unlock()
	b := promptLabelTab[strings.TrimSpace(name)]
	if b == nil || len(b.versions) == 0 {
		return nil
	}
	out := make(map[string]string, len(b.versions))
	for k, v := range b.versions {
		out[k] = v
	}
	return out
}

// promptLabelEvictOldestLocked — 表满时淘汰最久未登记的名字。调用方须持 promptLabelMu。
func promptLabelEvictOldestLocked() {
	var oldestKey string
	var oldestSeq int64 = -1
	for k, v := range promptLabelTab {
		if oldestSeq < 0 || v.seq < oldestSeq {
			oldestKey, oldestSeq = k, v.seq
		}
	}
	if oldestKey != "" {
		delete(promptLabelTab, oldestKey)
	}
}

// resetPromptVersionTables — 丢掉进程内表（**仅测试用**：同一进程内换用例/换会话时必须干净起步）。
func resetPromptVersionTables() {
	promptLabelMu.Lock()
	promptLabelTab = map[string]*promptLabelBucket{}
	promptLabelSeq = 0
	promptLabelMu.Unlock()

	promptStepMu.Lock()
	promptStepTab = map[string]*promptStepCounter{}
	promptStepSeq = 0
	promptStepMu.Unlock()
}

// ── ④ 策略标识（由生效配置规范化后取指纹）──────────────────────────────

// PromptScaffoldSpec — 一次请求**实际生效**的提示脚手架配置（调用方给的配置事实）。
//
// 为什么用"配置集合"当版本：仓内没有策略版本号（没有 v1/v2 的发布机制），编一个 +1 的序号
// 是不可验证的自称；而**配置本身**是可复算的——同一份配置规范化后指纹相同，改一个字节指纹必变
// （用例③附带钉住这一点）。于是"策略换版"这件事变成了可判定的事实，而不是记账人的记忆。
type PromptScaffoldSpec struct {
	Strategy string   // 策略名（装配处的稳定标识，如 "chat.loop"）
	Features []string // 生效的开关/特性（闭集文本，如 "tools"/"no_terminator"/"loopguard"）
	Params   []string // 关键参数（如 "max_rounds=10"）；**集合语义**（顺序无关，内部排序去重）
}

// StrategyIDOf — 策略标识 = hex(sha256(规范化配置)[:8])（16 字符）。
// 三项全空 ⇒ 返回 ""（**一项配置事实都没有 ⇒ 缺席**，不编造一个看着像 id 的值）。
func StrategyIDOf(s PromptScaffoldSpec) string {
	name := strings.TrimSpace(s.Strategy)
	feats := promptNormalizeSpecList(s.Features)
	params := promptNormalizeSpecList(s.Params)
	if name == "" && len(feats) == 0 && len(params) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(name)
	for _, f := range feats {
		b.WriteString("\x00f:")
		b.WriteString(f)
	}
	for _, p := range params {
		b.WriteString("\x00p:")
		b.WriteString(p)
	}
	return promptFingerprintOf(b.String())
}

// promptNormalizeSpecList — 规范化一个配置串列表：trim、丢空、去重、**排序**（集合语义 ⇒ 与书写顺序无关）。
func promptNormalizeSpecList(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	if len(out) == 0 {
		return nil
	}
	sort.Strings(out)
	return out
}

// PromptStrategyIn — 调用方对本次请求「提示脚手架策略」的**声明**（配置事实；nil 的字段 = 没声明）。
//
// 语义（写死）：
//
//	StrategyID            —— 生效配置的指纹（用 StrategyIDOf 从 PromptScaffoldSpec 得来）；空=没声明
//	StepIndex             —— 本会话第几步；0 = 让观测面按**提示装配计数**填（见 PromptStepIndex）
//	HasAcceptanceCriteria —— 本次脚手架里**有没有可判定的验收标准**（有 ⇒ 模型被要求"交付可验收的东西"）
//	RequiresToolCallFirst —— 本次脚手架**是否要求先出工具调用**（而不是直接给终答）
//
// 后两个字段是**指针**：nil = 没声明（键缺席）、显式 false 照落（声明了"没有"）。
// 「给大目标（只读不交差）还是给一步一验的小目标（真动手）」这个实测差异，
// 就落在这两个布尔 + step_index 上——不落它，这个差异永远归因不了（H7）。
type PromptStrategyIn struct {
	StrategyID            string
	StepIndex             int
	HasAcceptanceCriteria *bool
	RequiresToolCallFirst *bool
}

// PromptStrategyFlag — 三态布尔的取值助手（`PromptStrategyFlag(false)` = **声明了"没有"**，与 nil 不同）
func PromptStrategyFlag(b bool) *bool { return &b }

// strategyFieldsOf — 把调用方声明落成事件字段（**逐字段三态**：没声明就不写）。
//
// 返回 (strategyID, stepIndex, hasAcceptance, requiresToolFirst)：
//   - strategyID 空 = 缺席；stepIndex nil = 缺席（无会话且没给显式值 ⇒ 数不出来，不写 0 冒充）；
//   - 两个布尔指针原样透传（调用方的 false 是真声明，必须落）。
//
// 一项事实都没有 ⇒ 全零返回 ⇒ 事件里四个键**一个都不出现**（反例④：不写 0/空串冒充）。
func strategyFieldsOf(in *PromptStrategyIn, session string) (string, *int, *bool, *bool) {
	if in == nil {
		return "", nil, nil, nil
	}
	id := strings.TrimSpace(in.StrategyID)
	// 计数**恒推进**（它数的是"本会话装配了几次提示"），记录值优先用调用方给的显式值：
	// 否则显式值轮不推进计数 ⇒ 后面自动轮的步序会与"装配了几次"对不上（同一件事两套数）。
	counted := 0
	if session != "" {
		counted = PromptStepIndex(session)
	}
	var step *int
	switch {
	case in.StepIndex > 0:
		v := in.StepIndex // 调用方给了显式值 ⇒ 用显式值（它知道得比数数更准）
		step = &v
	case counted > 0:
		v := counted
		step = &v
	}
	var acc, tool *bool
	if in.HasAcceptanceCriteria != nil {
		v := *in.HasAcceptanceCriteria
		acc = &v
	}
	if in.RequiresToolCallFirst != nil {
		v := *in.RequiresToolCallFirst
		tool = &v
	}
	return id, step, acc, tool
}

// ── ⑤ step_index：本会话第几步（观测面自己数的提示装配计数）──────────────

// promptStepSessionsMax — 会话计数表容量上限（同 obsTraceSessionsMax 口径：表满淘汰最久未出现的会话）
const promptStepSessionsMax = 4096

type promptStepCounter struct {
	steps int
	seq   int64
}

var (
	promptStepMu  sync.Mutex
	promptStepSeq int64
	promptStepTab = map[string]*promptStepCounter{}
)

// PromptStepIndex — 推进一步并返回**本会话第几步**（1 起；无会话 ⇒ 0=数不出来）。
//
// 为什么由观测面自己数（而不是拿轮次号顶替）：轮次号是**一次请求内**的序号（重新发一条消息又从 1 开始），
// 而 H7 要的是**会话内**的步序（"第几步小目标"）。装配一次提示 = 一步，这是可数的事实。
// 纪律：只记数、不参与任何判定（观测面铁律①）。
func PromptStepIndex(session string) int {
	if session == "" {
		return 0
	}
	promptStepMu.Lock()
	defer promptStepMu.Unlock()
	c := promptStepTab[session]
	if c == nil {
		if len(promptStepTab) >= promptStepSessionsMax {
			promptStepEvictOldestLocked()
		}
		c = &promptStepCounter{}
		promptStepTab[session] = c
	}
	promptStepSeq++
	c.seq = promptStepSeq
	c.steps++
	return c.steps
}

// promptStepEvictOldestLocked — 表满时淘汰最久未推进的会话。调用方须持 promptStepMu。
func promptStepEvictOldestLocked() {
	var oldestKey string
	var oldestSeq int64 = -1
	for k, v := range promptStepTab {
		if oldestSeq < 0 || v.seq < oldestSeq {
			oldestKey, oldestSeq = k, v.seq
		}
	}
	if oldestKey != "" {
		delete(promptStepTab, oldestKey)
	}
}

// PromptStepCountOf — 回查某会话已经装配了几步（0 = 没见过这个会话；不编造）。
func PromptStepCountOf(session string) int {
	promptStepMu.Lock()
	defer promptStepMu.Unlock()
	if c := promptStepTab[session]; c != nil {
		return c.steps
	}
	return 0
}
