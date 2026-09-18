// Package sampling — 事件流采样（设计-内建调试版-v1.2 第〇节 C 组 C1–C6 / 开放问题 Q11；任务表 T6.4）。
//
// 为什么单独立一个叶包（不动观测落点）：
//   - 观测落在 core/internal/chat/obs.go 的 obsWrite（本批**不改它**，接入留后续批）；
//   - 本包**零依赖**：只 import 标准库，不 import chat / redact —— 采样只回答"这条要不要留"，
//     脱敏只回答"留下来的这条怎么写"。两者互不调用 ⇒ 可以各自被替换、各自被门禁覆盖。
//
// 六条写死的口径（改这里先读 docs/01-设计/设计-内建调试版-v1.2.md 第〇节 C 组）：
//
//	① **优先级序写死**（C1）：MUSTKEEP → DROP → PROBABILISTIC → **默认拒绝**。
//	   它不是"配置顺序"，是常量序（PolicyPrecedence）：无论策略怎么传进来，账里的
//	   matched_policies 一律按此序输出，MUSTKEEP 压一切。
//	② **drop 不得否决 must_keep**（C1）：除非显式打开 DropVetoesMustKeep；**打开时该否决必被记录**
//	   （Decision.OverriddenMustKeep + 账上 overridden_must_keep 的墓碑）。默认实现里
//	   "drop 命中"是**可取证的事实**（进 matched_policies），不是被吞掉的事实。
//	③ **概率采样必须是确定性哈希**（C3/C6 的本地化落点）：以 session/run id 为盐对事件身份取 sha256，
//	   不用 rand、不用 math/rand。否则同一条事件在重算/重放/复核时判词会变，判定账就不可核对。
//	④ **dry-run**（C4/Q11，首版默认）：只计算并记录决策、**不真裁** ⇒ 落盘条数 = 全量。
//	⑤ **判定字段 + 墓碑**（C5）：每 run 记 decision/matched/deciding/threshold/weight/degrade_level；
//	   被裁者留**墓碑**（tombstone=true），让"缺席"不再歧义。
//	⑥ **绝不阻塞 agent**（C2）：闸（字节/速率）与容量（ring_bytes）超限一律**当次判丢**（或破例放行必留），
//	   绝不 sleep、绝不重试、绝不把错误抛给调用方；落盘失败也只记降级等级并留碑。
package sampling

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"os"
	"time"
)

// ── 默认值（C6 写死初值；依据逐条注明，全部可覆盖）──────────────────────────
const (
	// DefaultDecisionWait — tail_delay = 2s（C6）：决策点（run.end）之后再等这么久收尾，
	// 让同一 run 内后到的判词赶上本批。
	DefaultDecisionWait = 2 * time.Second
	// DefaultTraceTimeout — trace_timeout = 60s（C6）：一条 run 超过它仍未收尾 ⇒ 强制决策（不许无限等）。
	DefaultTraceTimeout = 60 * time.Second
	// DefaultDiscardGrace — discard_grace = 10m（C6）：probation 宽限期。晚到的判词在这段时间内**可翻案**；
	// 过了即定案。容量不够时宽限期会被**提前终结**（degrade_level=ring）—— 因为"宽限"是机会、不是保证（C3）。
	DefaultDiscardGrace = 10 * time.Minute
	// DefaultRingBytes — ring = 256MiB（C6 写死的初值）。
	DefaultRingBytes = 256 << 20
	// DefaultRateBytesPerSec — 字节闸 8MiB/s（C2）：防"系统一慢就全采"的雪崩。
	// 8MiB/s ≈ 1024 条/s × 8KiB/条（含内容类字段的事件量级），与下一条同口径。
	DefaultRateBytesPerSec = 8 << 20
	// DefaultRateEventsPerSec — 速率闸 1024 条/s（C2）。
	DefaultRateEventsPerSec = 1024
	// DefaultMaxInflight — 未决（probation 里等翻案）条数上限 65536。
	// 依据 C3「ring 容量 ≥ 到达率 × 宽限期」：65536 × DefaultAvgEventBytes(512B) = 32MiB，落在 256MiB 内。
	// 反过来读：到达率 ≤ 65536/600s ≈ 109 条/s 才撑得满 10m 宽限期；超过就该调大它 ——
	// SizingOK 把这条约束**算给调用方看**，本包不做静默假设、也不因此阻塞。
	DefaultMaxInflight = 65536
	// DefaultAvgEventBytes — 尺寸约束换算用的单事件估值（512B）。它是**估值不是测量值**：
	// 调用方应按自己的真实事件重算（SizingOK 允许传入实际估值）。
	DefaultAvgEventBytes = 512
	// DefaultMaxRuns — 判定账保留的 run 数上限。与 obs.go 的 obsTraceSessionsMax 同思路：
	// 长跑进程里 run 无界，账必须有界；满了淘汰**最久未出现**的 run 的账（观测可容忍，内存不可无界）。
	DefaultMaxRuns = 4096
)

// EnvDryRun — C4 点名的首版默认开关：ZERG_DEBUG_SAMPLING_DRYRUN=1 ⇒ 只记录不裁。
// 只有显式的 0/false/off 才算关掉；其余（含未设置）都按**开**处理 —— 首版默认 dry-run 是设计要求。
const EnvDryRun = "ZERG_DEBUG_SAMPLING_DRYRUN"

// ── 判定序（C1：写死）────────────────────────────────────────────────────

// PolicyKind — 策略类别。四类策略 + 两个"不是策略"的判词来源。
type PolicyKind string

const (
	KindMustKeep      PolicyKind = "must_keep"     // 必留：出错/长尾等"丢了就查不清"的样本
	KindDrop          PolicyKind = "drop"          // 显式丢弃：噪声/健康检查/心跳
	KindProbabilistic PolicyKind = "probabilistic" // 概率采样（确定性哈希，非 rand）
	KindDefaultDeny   PolicyKind = "default_deny"  // **不是策略**：无任何策略命中时的兜底判词（默认拒绝）
	KindOff           PolicyKind = "off"           // 全采模式：闸被摘掉（不是策略命中，是"没有采样"）
)

// 判词来源名（deciding_policy）里两个保留名 —— 它们**不可能**与策略名撞车：策略名由调用方给，
// 而这两个值是本包写死的语法。空名策略会被 NewSampler 丢弃（见 sanitizePolicies）。
const (
	PolicyDefaultDeny = "default_deny"
	PolicyOff         = "sampling_off"
)

// PolicyPrecedence 是**写死**的判定序（C1：MUSTKEEP → DROP → PROBABILISTIC → 默认拒绝）。
// 两重含义，两条都不许改：
//  1. **定案**：靠前者先定案 —— MUSTKEEP 命中即 keep，drop 连发言权都没有（除显式开关）；
//  2. **记账**：matched_policies 一律按此序输出，与策略在 NewSampler 里的传入顺序无关。
//
// 默认拒绝放在序尾不是"优先最低"，而是"最后兜底"：它是"什么都没命中"这个事实的名字，
// 让"缺席"在账里也有位置（C1 的序尾就是它）。
var PolicyPrecedence = []PolicyKind{KindMustKeep, KindDrop, KindProbabilistic, KindDefaultDeny}

// precedenceOf — 在写死序里的位次；未登记的类别给尾位（等价于默认拒绝那一档：绝不因为"不认识"而放行样本）。
func precedenceOf(k PolicyKind) int {
	for i, p := range PolicyPrecedence {
		if p == k {
			return i
		}
	}
	return len(PolicyPrecedence)
}

// ── 事件与策略 ───────────────────────────────────────────────────────────

// Event — 采样器看到的一条事件（**只带判据，不带正文**：正文留在调用方，采样不做脱敏、不碰内容）。
type Event struct {
	Seq     int64             // run 内序号（Key 缺席时的身份兜底）
	Key     string            // 事件身份（同一 run 内唯一）；抽签与去重都用它
	Name    string            // 事件名（低基数）
	Session string            // 会话 id（抽签的盐之一）
	RunID   string            // run id（抽签的盐之一；也是判定账的分组键）
	Bytes   int               // 事件体积（字节闸与 ring 容量都用它）；≤0 ⇒ 按 DefaultAvgEventBytes 计
	Attrs   map[string]string // 低基数判据（策略只许命中这些，不许扫正文）
}

// identity — 事件身份（抽签的输入）：Key 优先；Key 为空退化为 Name#Seq。
// 退化路径仍然**确定**（不引入任何随机量），只是盐更弱 ⇒ 账里仍可复算，因此不禁止。
func (ev Event) identity() string {
	if ev.Key != "" {
		return ev.Key
	}
	return fmt.Sprintf("%s#%d", ev.Name, ev.Seq)
}

// bytesOf — 事件体积（≤0 ⇒ 用估值，绝不写 0 让容量约束静默失效）。
func (ev Event) bytesOf() int {
	if ev.Bytes > 0 {
		return ev.Bytes
	}
	return DefaultAvgEventBytes
}

// Policy — 一条策略。
//
// 硬约束（写在类型上，不是写在文档里）：
//   - Match **为 nil ⇒ 永不命中**（不是"全命中"）：把"忘了写判据"变成"什么都不采"而不是"全放行"，
//     这是本包唯一可接受的默认方向（fail-closed 倾向，与 D 组同源思路）。
//   - Rate 只对 KindProbabilistic 有意义；其他类别一律忽略（写在那里也不会改变判词，见 Decide）。
type Policy struct {
	Name   string           // 策略名（进账：matched_policies / deciding_policy）；空名策略会被丢弃
	Kind   PolicyKind       // 四类之一
	Match  func(Event) bool // 判据；nil ⇒ 永不命中
	Rate   float64          // 概率采样率 ∈ [0,1]；≤0 ⇒ 永不中签，≥1 ⇒ 必中签
	Reason string           // 人类可读的判据来源（进账的 reason，低基数、别放正文）
}

// NewMustKeep — 必留策略。
func NewMustKeep(name, reason string, match func(Event) bool) Policy {
	return Policy{Name: name, Kind: KindMustKeep, Match: match, Reason: reason}
}

// NewDrop — 显式丢弃策略。
func NewDrop(name, reason string, match func(Event) bool) Policy {
	return Policy{Name: name, Kind: KindDrop, Match: match, Reason: reason}
}

// NewProbabilistic — 概率采样策略（rate ∈ [0,1]；抽签用确定性哈希，见 drawWeight）。
func NewProbabilistic(name, reason string, rate float64, match func(Event) bool) Policy {
	return Policy{Name: name, Kind: KindProbabilistic, Match: match, Rate: rate, Reason: reason}
}

// Always — 恒真判据（写显式策略时的常用件；**它不是 Match 的默认值**，默认值是 nil=永不命中）。
func Always() func(Event) bool { return func(Event) bool { return true } }

// MatchName — 事件名命中之一。
func MatchName(names ...string) func(Event) bool {
	return func(ev Event) bool {
		for _, n := range names {
			if ev.Name == n {
				return true
			}
		}
		return false
	}
}

// MatchAttr — 属性等于给定值（低基数判据的标准形态）。
func MatchAttr(key, value string) func(Event) bool {
	return func(ev Event) bool { return ev.Attrs[key] == value }
}

// MatchAttrAny — 属性键存在且非空（"有没有这一类事实"，不管取什么值）。
func MatchAttrAny(key string) func(Event) bool {
	return func(ev Event) bool { return ev.Attrs[key] != "" }
}

// ── 判词与降级等级 ───────────────────────────────────────────────────────

// Verdict — 判词。只有两种：留 / 丢（没有"看情况"这种第三态 —— 那正是"缺席变得歧义"的来源）。
type Verdict string

const (
	VerdictKeep Verdict = "keep"
	VerdictDrop Verdict = "drop"
)

// 降级等级（degrade_level，C5）：这一条的判词是**被什么力量**影响的，可枚举、低基数。
const (
	DegradeNone  = "none"  // 没降级：判词就是策略判词
	DegradeRate  = "rate"  // 速率闸（条/s）参与定案：非必留被裁；必留则为**破例放行**（计数器可查）
	DegradeBytes = "bytes" // 字节闸（B/s）参与定案，同上
	DegradeRing  = "ring"  // ring 容量（ring_bytes/max_inflight）参与定案：probation 被提前终结
	DegradeSink  = "sink"  // 落盘失败（best-effort 的终点）：事实缺席 ⇒ 留碑
)

// Decision — 一条事件的判定结果（C5 点名的六个字段都在这里）。
//
// 字段语义（写死，读侧别自作聪明）：
//   - Verdict 是**判词**；Threshold/Weight 是**判词依据**（概率采样才有；其余类别为 nil —— "不适用"
//     与"值等于 0"必须可分，故用指针，绝不写 0 顶替）；
//   - MatchedPolicies 按 PolicyPrecedence 输出（含"命中了但没定案"的，例如被必留压住的 drop）；
//   - DecidingPolicy/DecidingKind 是**定案者**（可以是 default_deny / sampling_off 这两个保留名）；
//   - DegradeLevel 说明闸/容量/落盘是否参与了这次定案；GateEnforced 说明判词是否被闸**覆盖**过
//     （覆盖时 DecidingPolicy 仍是"谁想留它"，degrade_level 才是"谁把它裁了"—— 两个事实都留着）；
//   - Emitted/Tombstone 由 Ring 在落地时回填（纯判定 Sampler.Decide 里恒为 false）：
//     Emitted=true ⇔ 真的交给了落盘 sink 且成功；Tombstone=true ⇔ 真的缺席且已留碑。
type Decision struct {
	Key       string
	EventName string
	RunID     string

	Verdict            Verdict
	MatchedPolicies    []string
	DecidingPolicy     string
	DecidingKind       PolicyKind
	Threshold          *float64
	Weight             *float64
	DegradeLevel       string
	GateEnforced       bool
	OverriddenMustKeep bool

	DryRun    bool
	Emitted   bool
	Tombstone bool
	Reason    string
	DecidedAt time.Time
}

// EffectiveVerdict — **真正执行**的判词：dry-run ⇒ 一律 keep（C4：只计算并记录决策、不真裁）。
// 它与 Verdict 必须分开：账里记 Verdict（判词，dry-run 下也照记），落地看 EffectiveVerdict。
func (d Decision) EffectiveVerdict() Verdict {
	if d.DryRun || d.Verdict == VerdictKeep {
		return VerdictKeep
	}
	return VerdictDrop
}

// ── 配置 ─────────────────────────────────────────────────────────────────

// Sink — 落盘出口（唯一形态）：返回非 nil 即视为**没落成**（本包不重试、不阻塞）。
// 若要异步/带队列，用 ChanSink 包一层（它的语义是"写不进去立刻报满"）。
type Sink func(Event) error

// Config — 全部旋钮（C6 的四个参数都在这里，且都有默认值；零值结构体经 Normalize 后即默认配置）。
//
// 零值纪律：本结构体**读得懂零值**（Normalize 填默认），但 DryRun 是个裸 bool ⇒ 零值是"非 dry-run"。
// 首版默认走 DefaultConfig()/ConfigFromEnv()（它把 DryRun 置 true，C4）。这是刻意选的：
// 让"关掉 dry-run"必须由人显式写出来，不是靠忘了写。
type Config struct {
	// ── 交付要求点名的四个可参数化项 ──
	DecisionWait time.Duration // decision_wait：决策点后再等多久（默认 DefaultDecisionWait=2s）
	RingBytes    int           // ring_bytes：probation 缓冲的字节上限（默认 256MiB）
	DiscardGrace time.Duration // discard_grace：丢弃宽限期（默认 10m）
	MaxInflight  int           // max_inflight：未决条数上限（默认 65536）

	// ── 闸（C2）──
	RateBytesPerSec  int // 字节闸（默认 8MiB/s）
	RateEventsPerSec int // 速率闸（默认 1024 条/s）

	// ── 行为开关 ──
	DryRun             bool   // C4：true ⇒ 只记录不裁（落盘条数 = 全量）
	DropVetoesMustKeep bool   // C1 的显式开关：true ⇒ drop 可否决必留（**否决必被记录**）
	SamplingOff        bool   // 全采模式：闸被摘掉，一条不丢（判词 deciding_policy=sampling_off）
	Salt               string // 抽签盐的前缀（与 session/run 一起进哈希）

	// ── 出口与时钟 ──
	Sink    Sink             // nil ⇒ 本包只出账、不落盘（Emitted 不增；用于"只量召回"的离线用法）
	Clock   func() time.Time // nil ⇒ time.Now；**用例/回放注入冻结时钟**用（本包不读系统时间以外的任何时间源）
	MaxRuns int              // 判定账保留的 run 数（默认 4096）
}

// DefaultConfig — C6 写死的首版默认（含 C4 的 dry-run 默认开）。
func DefaultConfig() Config {
	return Config{
		DecisionWait:     DefaultDecisionWait,
		RingBytes:        DefaultRingBytes,
		DiscardGrace:     DefaultDiscardGrace,
		MaxInflight:      DefaultMaxInflight,
		RateBytesPerSec:  DefaultRateBytesPerSec,
		RateEventsPerSec: DefaultRateEventsPerSec,
		DryRun:           true, // C4/Q11：首版默认 dry-run（只记录不裁）
		Clock:            time.Now,
		MaxRuns:          DefaultMaxRuns,
	}
}

// ConfigFromEnv — 默认配置 + 环境变量覆盖（C4 点名的 ZERG_DEBUG_SAMPLING_DRYRUN）。
// 只有 0/false/off（大小写不敏感）算关；其余都算开 ⇒ "首版默认 dry-run"不因拼错变量名而失效。
func ConfigFromEnv() Config {
	cfg := DefaultConfig()
	switch lower(os.Getenv(EnvDryRun)) {
	case "0", "false", "off":
		cfg.DryRun = false
	}
	return cfg
}

// Normalize — 把零值补成默认值（返回副本；不就地改调用方的结构体）。
// 只补"零值 = 未设置"的数值项；三个布尔开关**不补**（它们的零值就是明确的语义：非 dry-run / 不容否决 / 采样开）。
func (c Config) Normalize() Config {
	if c.DecisionWait <= 0 {
		c.DecisionWait = DefaultDecisionWait
	}
	if c.RingBytes <= 0 {
		c.RingBytes = DefaultRingBytes
	}
	if c.DiscardGrace <= 0 {
		c.DiscardGrace = DefaultDiscardGrace
	}
	if c.MaxInflight <= 0 {
		c.MaxInflight = DefaultMaxInflight
	}
	if c.RateBytesPerSec <= 0 {
		c.RateBytesPerSec = DefaultRateBytesPerSec
	}
	if c.RateEventsPerSec <= 0 {
		c.RateEventsPerSec = DefaultRateEventsPerSec
	}
	if c.MaxRuns <= 0 {
		c.MaxRuns = DefaultMaxRuns
	}
	if c.Clock == nil {
		c.Clock = time.Now
	}
	return c
}

// SizingOK — C3 的尺寸约束自检：ring ≥ 到达率 × 宽限期（字节口径，按 avgBytes 折算）。
// 返回 (是否满足, 满足约束所需的 ring 字节)。不满足**不报错也不阻塞**：宽限期会被提前终结
// （账里那些墓碑的 degrade_level=ring），调用方据此调大 ring 或调小宽限。
func (c Config) SizingOK(ratePerSec, avgBytes float64) (bool, int) {
	if avgBytes <= 0 {
		avgBytes = DefaultAvgEventBytes
	}
	want := int(ratePerSec*c.DiscardGrace.Seconds()*avgBytes + 0.5)
	return c.RingBytes >= want, want
}

// RequiredRingBytes — 只算"需要多少"，不看配置（SizingOK 的裸计算部分）。
func RequiredRingBytes(ratePerSec float64, grace time.Duration, avgBytes float64) int {
	if avgBytes <= 0 {
		avgBytes = DefaultAvgEventBytes
	}
	return int(ratePerSec*grace.Seconds()*avgBytes + 0.5)
}

func lower(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] += 'a' - 'A'
		}
	}
	return string(b)
}

// ── 判定器 ───────────────────────────────────────────────────────────────

// Sampler — 纯判定器（策略 + 配置，**不缓冲、不落盘、不记账**）。
// Ring 在它之上加缓冲/宽限/闸/账；这样"判词对不对"与"缓冲对不对"可以各自被用例钉住。
type Sampler struct {
	cfg      Config
	mustKeep []Policy
	drop     []Policy
	prob     []Policy
}

// NewSampler — 建判定器。空名策略被丢弃（空名进账等于没有取证价值，且会撞保留名 default_deny/sampling_off）。
func NewSampler(cfg Config, policies ...Policy) *Sampler {
	s := &Sampler{cfg: cfg.Normalize()}
	for _, p := range policies {
		if p.Name == "" {
			continue
		}
		switch p.Kind {
		case KindMustKeep:
			s.mustKeep = append(s.mustKeep, p)
		case KindDrop:
			s.drop = append(s.drop, p)
		case KindProbabilistic:
			s.prob = append(s.prob, p)
		default:
			// 未登记类别一律不入表 ⇒ 永远不会定案（等价于这条策略没写；不会静默放行样本）。
			continue
		}
	}
	return s
}

// Config — 返回归一化后的配置（读侧用它核对默认值与覆盖结果）。
func (s *Sampler) Config() Config { return s.cfg }

// Policies — 按写死序返回已登记的策略（must_keep → drop → probabilistic）。
func (s *Sampler) Policies() []Policy {
	out := make([]Policy, 0, len(s.mustKeep)+len(s.drop)+len(s.prob))
	out = append(out, s.mustKeep...)
	out = append(out, s.drop...)
	out = append(out, s.prob...)
	return out
}

func (s *Sampler) now() time.Time {
	if s.cfg.Clock != nil {
		return s.cfg.Clock()
	}
	return time.Now()
}

// saltFor — 抽签盐：配置盐 ‖ session ‖ run（三者都可为空 ⇒ 判词仍确定，只是盐更弱）。
// **不含时间、不含进程随机量** —— 这是"账可复算"的前提（C3）。
func (s *Sampler) saltFor(ev Event) string {
	return s.cfg.Salt + "\x00" + ev.Session + "\x00" + ev.RunID
}

// drawWeight — 确定性抽签（**不用 rand / math/rand**）：sha256(盐 ‖ 0x00 ‖ 事件身份) 取前 8 字节 ⇒ [0,1)。
// 取 53 位（float64 尾数宽度）而不是 64 位：避免 float64 舍入把两个不同摘要折成同一个可比值。
// 变异自证点（本文件最承重的一处）：把 salt 从哈希输入里去掉 ⇒ 用例⑤的"换盐至少翻转一条判词"必红。
func drawWeight(salt, identity string) float64 {
	h := sha256.New()
	h.Write([]byte(salt))
	h.Write([]byte{0})
	h.Write([]byte(identity))
	sum := h.Sum(nil)
	v := binary.BigEndian.Uint64(sum[:8])
	return float64(v>>11) / float64(uint64(1)<<53)
}

// matchedPolicies — 按**写死序**（不是传入顺序）输出命中判据的策略名。
// 这里是"配置顺序不得影响账"这句口径的唯一落地处。
func (s *Sampler) matchedPolicies(ev Event) ([]string, []Policy, []Policy, []Policy) {
	mks := matchAll(s.mustKeep, ev)
	drs := matchAll(s.drop, ev)
	prs := matchAll(s.prob, ev)
	out := make([]string, 0, len(mks)+len(drs)+len(prs))
	for _, p := range mks {
		out = append(out, p.Name)
	}
	for _, p := range drs {
		out = append(out, p.Name)
	}
	for _, p := range prs {
		out = append(out, p.Name)
	}
	return out, mks, drs, prs
}

func matchAll(ps []Policy, ev Event) []Policy {
	var out []Policy
	for _, p := range ps {
		if p.Match != nil && p.Match(ev) {
			out = append(out, p)
		}
	}
	return out
}

// Decide — 判定一条事件（C1 的序，逐条可核对）。
//
// 遍历次序写死为 PolicyPrecedence：MUSTKEEP → DROP → PROBABILISTIC → 默认拒绝。
//
//  1. 有必留命中 ⇒ **留**（必留压一切）；此时若 drop 也命中：
//     · 默认（DropVetoesMustKeep=false）⇒ 仍是留，但 drop 的名字进 matched_policies；
//     · 显式开关打开 ⇒ 改为丢，且 OverriddenMustKeep=true（**否决必被记录**，不静默）。
//  2. 无必留、有 drop 命中 ⇒ **丢**（概率采样根本不参与定案 —— 哪怕抽签必中）。
//  3. 无必留、无 drop、有概率策略命中判据 ⇒ 按**确定性抽签**：weight < rate ⇒ 留，否则丢。
//     此时 threshold/weight 都落账（"凭什么丢的"必须可核）。
//  4. 什么都没命中 ⇒ **默认拒绝**（丢）；threshold/weight **缺席**（不是"不适用写 0"，是不该有）。
func (s *Sampler) Decide(ev Event) Decision {
	d := Decision{
		Key:          ev.Key,
		EventName:    ev.Name,
		RunID:        ev.RunID,
		Verdict:      VerdictDrop,
		DegradeLevel: DegradeNone,
		DryRun:       s.cfg.DryRun,
		DecidedAt:    s.now(),
	}
	if s.cfg.SamplingOff {
		// 全采模式：闸被摘掉（不是"策略命中"）⇒ 判词 keep、定案者 sampling_off、无阈值。
		d.Verdict = VerdictKeep
		d.DecidingKind = KindOff
		d.DecidingPolicy = PolicyOff
		d.Reason = "全采模式（显式关闭采样）：无裁决、一条不丢"
		return d
	}

	matched, mks, drs, prs := s.matchedPolicies(ev)
	d.MatchedPolicies = matched

	switch {
	case len(mks) > 0:
		d.Verdict = VerdictKeep
		d.DecidingKind = KindMustKeep
		d.DecidingPolicy = mks[0].Name
		d.Reason = mks[0].Reason
		d.MatchedPolicies = matched
		if len(drs) > 0 {
			if s.cfg.DropVetoesMustKeep {
				d.Verdict = VerdictDrop
				d.DecidingKind = KindDrop
				d.DecidingPolicy = drs[0].Name
				d.OverriddenMustKeep = true
				d.Reason = fmt.Sprintf("显式开关 DropVetoesMustKeep=true：%s 否决了必留 %s", drs[0].Name, mks[0].Name)
			} else {
				d.Reason = fmt.Sprintf("%s（%s 命中但不得否决必留）", mks[0].Reason, drs[0].Name)
			}
		}
	case len(drs) > 0:
		d.Verdict = VerdictDrop
		d.DecidingKind = KindDrop
		d.DecidingPolicy = drs[0].Name
		d.Reason = drs[0].Reason
		if len(prs) > 0 {
			d.Reason = fmt.Sprintf("%s（drop 定案，概率采样不参与；%s 未定案）", drs[0].Reason, prs[0].Name)
		}
	case len(prs) > 0:
		p := prs[0]
		w := drawWeight(s.saltFor(ev), ev.identity())
		th := p.Rate
		d.Threshold, d.Weight = &th, &w
		d.DecidingKind = KindProbabilistic
		d.DecidingPolicy = p.Name
		if w < th {
			d.Verdict = VerdictKeep
			d.Reason = fmt.Sprintf("%s（确定性抽签命中：%.6f < %.6f）", p.Reason, w, th)
		} else {
			d.Verdict = VerdictDrop
			d.Reason = fmt.Sprintf("%s（确定性抽签未命中：%.6f ≥ %.6f）", p.Reason, w, th)
		}
	default:
		// C1 序尾：没命中就是**不采**。这一支是本包最容易写成"默认全采"的地方。
		d.Verdict = VerdictDrop
		d.DecidingKind = KindDefaultDeny
		d.DecidingPolicy = PolicyDefaultDeny
		d.Reason = "无任何策略命中 ⇒ 默认拒绝（C1 序尾）"
	}
	return d
}

// Unused export guard: 让"序里的每一项都可达"成为可断言的事实（用例③用它防"某档被删掉"）。
func (s *Sampler) kindsInPrecedence() []PolicyKind { return PolicyPrecedence }
