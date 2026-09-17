// constraints.go — T3.2：**约束登记表 + 保留率**（设计稿 v1.2 第〇节 H2）
//
// 设计依据（逐字）：
//
//	「H2 缺约束登记表与保留率 ⇒『长对话后模型不再照做』只能靠人感觉｜★★★｜
//	 `constraint_registry{id,text,canonical,sha256,source_msg,must_survive,inject_where}`；
//	  每次调用前在**实际发送的 token 序列**上做存在性检索，落 `constraint_check{total,present,missing_ids}`；
//	  missing 非空 ⇒ 重注入或告警」。
//
// 任务表：docs/项目文档/v2.5.10/任务表-内建调试版实施-20260917.md T3.2
// （落点 `core/internal/chat/constraints.go`；验收用例：压缩掉一条 must_survive ⇒ missing_ids 非空）。
//
// 要治的盲区（改本文件前先读）：**「模型忘了硬约束」目前只能靠人感觉**——
//   - 会话里说过「必须 X / 不要 Y」，几轮压缩之后模型是否还看得见这句话，**观测面一个字都没有**；
//   - 被压缩掉的消息只留下 sha256 与 id（T3.1），**内容级的保留率仍是空白**：
//     摘要里到底有没有把那条硬约束带上，答不上来；
//   - 于是「忘了」与「本来就没说」在事后同形 ⇒ 无法归因，也无法回归。
//
// 本文件的判据链（写死，读侧据此复算）：
//
//	① 登记：条约束 = {id, text, canonical, sha256, source_msg_id, must_survive, inject_where}；
//	② 冻结：canonical + sha256 一旦登记即不再变（同一条约束跨轮同 id —— 否则保留率无法跨轮比）；
//	③ 检索：在**实际发给模型的提示**上做存在性检索（canonical 子串 **或** sha256/十六位指纹），
//	   检索栈见 promptHaystack（system ‖ tools ‖ history ‖ reminders，两侧同一规范化）；
//	④ 落账：每次请求落 `event_name=constraint_check{total,present,missing_ids}`；
//	   另有 `constraint_missing_alert` 供告警（**不阻断请求**，见 ObservePromptCheck）。
//
// 精度边界（如实，写死在这里，不许改口径时顺口美化）：登记靠**闭集标记词的启发式抽取**
// （constraintMarkers），召回优先：
//   - 漏登记 = 盲区（一条硬约束没进表，就永远查不出它掉了）⇒ 标记词取宽；
//   - 误登记 = 噪声（一条软话被当硬约束，会给出一条"掉了"的告警）⇒ 标记词不取太宽的词
//     （「唯一」「建议」「最好」这类**不在**闭集里）；
//   - 要精确就把约束**显式登记**（RegisterConstraint）：抽取只是兜底，不是唯一入口。
//
// 纪律（与 obs.go 文件头三条铁律同源）：
//  1. **登记/检索/写盘不改变对话**：本文件自己不改任何 prompt 字节、不阻断请求。**重注入**（见文末「重注入」节）
//     是**纯函数**：给"当前待发提示 + must_survive 约束"回"重注入后的提示"——采纳与否在装配点（ObservePromptCheck
//     的调用方），本文件不发送、不持有别人的字符串。（此条此前写的是"不重注入"；2026-09-18 按实测缺陷②
//     ——"真缺失只报警不治本"——把口径改成"报警 + 治本"，依据与实现口径见文末该节。）
//  2. **best-effort**：写盘失败只记日志（obsWrite 已保证）；登记表读写失败不 panic、不外抛。
//  3. **只落指纹与（截断的）约束原文，不落整段对话**：告警里带 text 是为了"可行动"（看得见是哪条），
//     上限 constraintAlertTextMax 字符；消息正文一个字都不落。
package chat

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
)

// ── 约束（H2 字段逐字）────────────────────────────────────────────────

// Constraint — 一条硬约束的登记记录。
//
// 字段口径：
//
//	ID          稳定标识（= "uc-" + sha256(canonical) 前 12 位）——**由 canonical 唯一决定**，
//	            所以同一条约束在不同轮/不同消息里被重复登记时拿到同一个 id（跨轮可比保留率）
//	Text        原文（用于告警可读；落盘时截断，见 constraintAlertTextMax）
//	Canonical   规范化文本（去零宽/折叠空白/trim）——**检索与指纹都基于它**，不基于原文
//	SHA256      sha256(canonical) 全串十六进制（检索的第二条路：注入方可以只带指纹）
//	SourceMsgID 首次见到这条约束的消息 id（explicit 登记时由调用方给；拿不到 ⇒ 0 = 未知，不编造）
//	MustSurvive 是否**必须活着**（压缩/摘要不得吃掉它）——告警的主判据
//	InjectWhere 这条约束**应当**从哪进来（闭集见 injectWhere* 常量；未知 ⇒ "unknown"，不猜）
type Constraint struct {
	ID          string `json:"id"`
	Text        string `json:"text"`
	Canonical   string `json:"canonical"`
	SHA256      string `json:"sha256"`
	SourceMsgID int64  `json:"source_msg_id"`
	MustSurvive bool   `json:"must_survive"`
	InjectWhere string `json:"inject_where"`
}

// inject_where 闭集（H2 只给了字段名，取值口径由本文件写死——低基数、可枚举）。
const (
	injectWhereSystem  = "system"  // 应当由系统提示（含模板/约定/身份）承担
	injectWhereMemory  = "memory"  // 应当由记忆块承担
	injectWhereHistory = "history" // 随对话历史携带（被 MarkCompacted 软归档后即消失——最常见的丢失形态）
	injectWherePinned  = "pinned"  // 应当被 pinned（永不参与摘要），见 H13
	injectWhereUnknown = "unknown" // 调用方没给/给了非法值 ⇒ 未知（不猜、不编造）
)

// normalizeInjectWhere — 非法/空取值一律归为 unknown（不编造、不落到闭集之外）
func normalizeInjectWhere(w string) string {
	switch w {
	case injectWhereSystem, injectWhereMemory, injectWhereHistory, injectWherePinned:
		return w
	default:
		return injectWhereUnknown
	}
}

// constraintAlertTextMax — 告警里约束原文的上限（**可行动**与**不落整段对话**的折中）
const constraintAlertTextMax = 120

// ── 规范化与指纹 ──────────────────────────────────────────────────────

// CanonicalConstraint — 规范化：去零宽字符、全角空格→半角、CR/LF/Tab→空格、折叠连续空白、trim。
//
// 为什么要规范化（不是洁癖）：同一句话在库里可能带换行/缩进/零宽字符，直接子串检索会**假性别离**
// （约束明明还在提示里，却报 missing ⇒ 告警噪声 ⇒ 团队关掉告警，这正是 H2 要避免的退化）。
// 两侧（登记侧与检索侧）**必须用同一个函数**，否则口径不一致 ⇒ 判据不可复算。
func CanonicalConstraint(text string) string {
	var b strings.Builder
	b.Grow(len(text))
	for _, r := range text {
		switch r {
		case '\u200b', '\u200c', '\u200d', '\ufeff', '\u2060': // 零宽/连接符/词连接
			continue
		case '\u3000': // 全角空格
			r = ' '
		case '\r', '\n', '\t', '\v', '\f':
			r = ' '
		}
		b.WriteRune(r)
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// ConstraintFingerprint — sha256(canonical) 全串十六进制（唯一指纹口径，别处不许另算）
func ConstraintFingerprint(canonical string) string {
	sum := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(sum[:])
}

// ConstraintID — 稳定 id（= "uc-" + 指纹前 12 位）。
// 由 canonical 唯一决定 ⇒ 同一条约束跨消息/跨轮**同 id**（保留率才能跨轮比，才不会因重述而翻倍）。
func ConstraintID(canonical string) string {
	return "uc-" + ConstraintFingerprint(canonical)[:12]
}

// NewConstraint — 从原文造一条登记记录（Text 保留原文，Canonical/SHA256/ID 由规范口径算出）
func NewConstraint(text string, sourceMsgID int64, mustSurvive bool, injectWhere string) Constraint {
	c := CanonicalConstraint(text)
	return Constraint{
		ID:          ConstraintID(c),
		Text:        text,
		Canonical:   c,
		SHA256:      ConstraintFingerprint(c),
		SourceMsgID: sourceMsgID,
		MustSurvive: mustSurvive,
		InjectWhere: normalizeInjectWhere(injectWhere),
	}
}

// ── 抽取（启发式兜底——不是唯一入口）──────────────────────────────────

// constraintMarkers — 中文硬约束标记词（闭集；**召回优先**，见文件头"精度边界"）。
// 刻意**不含**：唯一 / 建议 / 最好 / 尽量 / 可以 / 不能（"不能"在正常叙述里太常见——误报源）。
var constraintMarkers = []string{
	"必须", "务必", "不得", "禁止", "严禁", "不许", "不准",
	"绝不能", "永远不要", "一定要", "记住", "只能", "不要",
}

// constraintMarkersASCII — 英文硬约束标记词（小写后匹配）
var constraintMarkersASCII = []string{"must", "never", "do not", "don't", "required", "mandatory"}

// constraintSentenceMinRunes / Max — 约束句的长度窗（下限防噪声词，上限防把整段话当约束）
const (
	constraintSentenceMinRunes = 4
	constraintSentenceMaxRunes = 160
)

// splitConstraintSentences — 按句末标点切句（。！？!?；;\n）
func splitConstraintSentences(text string) []string {
	return strings.FieldsFunc(text, func(r rune) bool {
		switch r {
		case '。', '！', '？', '!', '?', '；', ';', '\n':
			return true
		}
		return false
	})
}

// isConstraintSentence — 一句话是否算硬约束（闭集标记词 + 长度窗；口径见文件头"精度边界"）
func isConstraintSentence(canonical string) bool {
	n := len([]rune(canonical))
	if n < constraintSentenceMinRunes || n > constraintSentenceMaxRunes {
		return false
	}
	low := strings.ToLower(canonical)
	for _, m := range constraintMarkersASCII {
		if strings.Contains(low, m) {
			return true
		}
	}
	for _, m := range constraintMarkers {
		if strings.Contains(canonical, m) {
			return true
		}
	}
	return false
}

// constraintsFromMessages — 从消息里抽约束。
//
// 口径（写死）：
//   - **只看 role=user**：约束是人的指令；assistant 的话不是约束（把模型自己的话当约束会出假告警）。
//   - 句级抽取（splitConstraintSentences），命中闭集标记词且长度合规者登记为 must_survive=true，
//     inject_where=history（它当前**就是**靠历史携带的——掉了就是掉了）。
//   - source_msg_id = 该消息 id（拿不到 ⇒ 0 = 未知）。
//   - 空消息/空句一律跳过（不登记空约束）。
func constraintsFromMessages(msgs []*Message) []Constraint {
	var out []Constraint
	for _, m := range msgs {
		if m == nil || m.Role != "user" {
			continue
		}
		for _, s := range splitConstraintSentences(m.Content) {
			c := CanonicalConstraint(s)
			if !isConstraintSentence(c) {
				continue
			}
			out = append(out, NewConstraint(s, m.ID, true, injectWhereHistory))
		}
	}
	return out
}

// ── 登记表（进程内缓存 + <state>/constraint_registry.json 落盘）──────────
//
// 为什么要落盘：长对话档**跨进程重启**（守护进程重启、换件）——只放内存的话，重启后登记表空
// ⇒「压缩吃掉约束」这件事在重启后**查不出来**（真盲区）。落盘口径与 compact_cooldown.json 同源
// （json + 临时文件 rename 原子替换），但**多一层进程内缓存**：登记只在出现新约束时变，
// 不必每轮 read-modify-write。
const (
	// constraintRegMaxSessions — 会话数上限（长跑守护进程里会话无界，表必须有界；满则拒新会话登记）
	constraintRegMaxSessions = 512
	// constraintRegMaxPerSession — 单会话约束数上限。**满了拒新、不挤老** ——
	// 老约束恰是最该守的（会话开头的"必须/不要"，正是长对话里最先被压掉的），
	// 用 LRU 把它们淘汰掉会把本文件要治的盲区重新造出来。
	constraintRegMaxPerSession = 64
)

// constraintRegistryFile — 登记表文件（statepath 每次调用解析 ⇒ 测试隔离友好）
func constraintRegistryFile() string { return statepath.File("constraint_registry.json") }

// constraintRegistryDoc — 落盘结构（按会话分桶）
type constraintRegistryDoc struct {
	Version  int                     `json:"version"`
	Sessions map[string][]Constraint `json:"sessions"`
}

// constraintRegVersion — 结构版本（将来改口径时读侧可分支；不是提示版本）
const constraintRegVersion = 1

var (
	constraintRegMu     sync.Mutex
	constraintRegCache  map[string][]Constraint
	constraintRegLoaded bool
)

// loadConstraintRegistryLocked — 读登记表（首次读盘，之后走缓存）。调用方须持 constraintRegMu。
// 读失败/文件损坏 ⇒ 空表（不 panic、不阻断；下一条约束登记会把文件重建）。
func loadConstraintRegistryLocked() map[string][]Constraint {
	if constraintRegLoaded && constraintRegCache != nil {
		return constraintRegCache
	}
	m := map[string][]Constraint{}
	if b, err := os.ReadFile(constraintRegistryFile()); err == nil {
		var doc constraintRegistryDoc
		if json.Unmarshal(b, &doc) == nil && doc.Sessions != nil {
			m = doc.Sessions
		}
	}
	constraintRegCache, constraintRegLoaded = m, true
	return m
}

// saveConstraintRegistryLocked — 原子落盘（临时文件 + rename）。失败只记日志（best-effort）。
func saveConstraintRegistryLocked(m map[string][]Constraint) {
	if err := os.MkdirAll(statepath.Dir(), 0o755); err != nil {
		log.Printf("⚠️ constraint_registry: mkdir failed (观测不受影响): %v", err)
		return
	}
	b, err := json.Marshal(constraintRegistryDoc{Version: constraintRegVersion, Sessions: m})
	if err != nil {
		log.Printf("⚠️ constraint_registry: marshal failed (观测不受影响): %v", err)
		return
	}
	final := constraintRegistryFile()
	tmp := final + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		log.Printf("⚠️ constraint_registry: write failed (观测不受影响): %v", err)
		return
	}
	if err := os.Rename(tmp, final); err != nil {
		log.Printf("⚠️ constraint_registry: rename failed (观测不受影响): %v", err)
	}
}

// RegisterConstraint — 显式登记一条约束（唯一的**精确**入口：调用方自己有权威判据时用它）。
// 幂等：同 canonical（⇒ 同 SHA256）重复登记直接吞掉。返回 true 表示本次真的新增了一条。
// 无会话 ⇒ 不登记（不编造全局表）。
func RegisterConstraint(sessionID string, c Constraint) bool {
	if sessionID == "" {
		return false
	}
	if c.Canonical == "" {
		c = NewConstraint(c.Text, c.SourceMsgID, c.MustSurvive, c.InjectWhere)
	}
	if c.SHA256 == "" {
		c.SHA256 = ConstraintFingerprint(c.Canonical)
	}
	if c.ID == "" {
		c.ID = ConstraintID(c.Canonical)
	}
	c.InjectWhere = normalizeInjectWhere(c.InjectWhere)
	constraintRegMu.Lock()
	defer constraintRegMu.Unlock()
	return registerConstraintsLocked(sessionID, []Constraint{c}) > 0
}

// registerConstraintsLocked — 批量合并（按 SHA256 去重；表满拒新）。返回新增条数。调用方须持锁。
func registerConstraintsLocked(sessionID string, add []Constraint) int {
	if sessionID == "" || len(add) == 0 {
		return 0
	}
	m := loadConstraintRegistryLocked()
	cur := m[sessionID]
	if cur == nil && len(m) >= constraintRegMaxSessions {
		log.Printf("⚠️ constraint_registry: 会话表已满(%d)，拒绝为 %s 新建登记（老会话优先）", constraintRegMaxSessions, sessionID)
		return 0
	}
	seen := make(map[string]struct{}, len(cur))
	for _, c := range cur {
		seen[c.SHA256] = struct{}{}
	}
	added := 0
	dirty := false
	for _, c := range add {
		if c.SHA256 == "" || c.Canonical == "" {
			continue
		}
		if _, dup := seen[c.SHA256]; dup {
			continue
		}
		if len(cur) >= constraintRegMaxPerSession {
			log.Printf("⚠️ constraint_registry: 会话 %s 登记数已满(%d)，本条不入表（不挤掉老约束）: %s",
				sessionID, constraintRegMaxPerSession, c.ID)
			break
		}
		cur = append(cur, c)
		seen[c.SHA256] = struct{}{}
		added++
		dirty = true
	}
	if dirty {
		m[sessionID] = cur
		saveConstraintRegistryLocked(m)
	}
	return added
}

// indexConstraints — 内部入口：把消息里抽到的约束并入登记表（返回新增条数）。
// 它就是"登记"这件事的唯一自动路径 —— 在**每次请求的提示检查**与**每次压缩开始**两处调用：
//   - 提示检查处：当前活动窗口里的硬约束立刻入册；
//   - 压缩开始处：**即将被压掉的那段**也入册 —— 否则"压掉一条 must_survive"这件事
//     在首次检查之前就发生的话（首轮即超阈值的会话），我们连它存在都不知道 ⇒ 关键用例无从谈起。
func indexConstraints(sessionID string, msgs []*Message) int {
	cs := constraintsFromMessages(msgs)
	if len(cs) == 0 {
		return 0
	}
	constraintRegMu.Lock()
	defer constraintRegMu.Unlock()
	n := registerConstraintsLocked(sessionID, cs)
	if n > 0 {
		log.Printf("📌 constr_registry: session %s +%d 条硬约束（共 %d）", sessionID, n, len(loadConstraintRegistryLocked()[sessionID]))
	}
	return n
}

// SessionConstraints — 取某会话的登记表（副本；按登记顺序返回 ⇒ 读侧可复算）。
func SessionConstraints(sessionID string) []Constraint {
	constraintRegMu.Lock()
	defer constraintRegMu.Unlock()
	cur := loadConstraintRegistryLocked()[sessionID]
	out := make([]Constraint, len(cur))
	copy(out, cur)
	return out
}

// ResetSessionConstraints — 清某会话的登记表（维护/用例用；幂等）。
func ResetSessionConstraints(sessionID string) {
	constraintRegMu.Lock()
	defer constraintRegMu.Unlock()
	m := loadConstraintRegistryLocked()
	if _, ok := m[sessionID]; !ok {
		return
	}
	delete(m, sessionID)
	saveConstraintRegistryLocked(m)
}

// resetConstraintRegistryCache — 丢弃进程内缓存（**仅测试用**：同一进程内换 ZERG_STATE_DIR 后必须重读）。
func resetConstraintRegistryCache() {
	constraintRegMu.Lock()
	defer constraintRegMu.Unlock()
	constraintRegCache, constraintRegLoaded = nil, false
}

// ── 存在性检索 + 事件块 ───────────────────────────────────────────────

// ConstraintCheckObs — `constraint_check` 事件的事实块（H2 逐字三字段 + 两处口径扩展 + 重注入事实）。
//
// 扩展（写死，读侧据此判读）：
//
//	MustSurviveMissingIDs —— missing 里 must_survive=true 的子集（告警的主判据；全缺时两者相等）
//	Alert                  —— 与 len(MissingIDs)>0 恒等（把它显式写出来，读侧不必再推导）
//
// 重注入事实（2026-09-18 缺陷②的治本那半；口径与实现见文末「重注入」节）：
//
//	Reinjected      —— 本次入口**产出了**重注入（调用方采纳返回的 PromptCheckResult.System 后才落到发送字节上）
//	ReinjectReason  —— 触发原因闭集：none / missing（刚检到缺失）/ periodic（每 N 轮的定时补注）
//	ReinjectedIDs   —— 本次真正注入的约束 id（**永不为 null**：空集合落 []）
//	InjectionCount  —— 本会话累计注入次数（0 = 一次没注过；幂等重注也计数）
//	InjectionID/SHA256 —— 本次注入块指纹（前 16 位 / 全串）——**哈希可解释性**的支点：
//	                       与上一轮比：injection_id 不变而 rendered_prefix_hash 变 ⇒ 模板/历史变了（不是重注）；
//	                       injection_id 变 + injection_count +1 ⇒ **就是重注入**（块内容变了）。
type ConstraintCheckObs struct {
	Total                 int      `json:"total"`       // 登记表里本会话的约束总数（分母）
	Present               int      `json:"present"`     // 在**本次实际发出的提示**上检索到的条数（分子）
	MissingIDs            []string `json:"missing_ids"` // 没检索到的约束 id（**永不为 null**：空集合落 []）
	MustSurviveMissingIDs []string `json:"must_survive_missing_ids"`
	Alert                 bool     `json:"alert"`

	// ── 重注入（治本）──
	Reinjected        bool     `json:"reinjected"`
	ReinjectReason    string   `json:"reinject_reason"`     // none | missing | periodic
	ReinjectedIDs     []string `json:"reinjected_ids"`      // 永不为 null
	InjectionCount    int      `json:"injection_count"`     // 本会话截至本轮的累计注入次数（0 = 该会话一次没注过）
	InjectionID       string   `json:"injection_id"`        // 注入块指纹前 16 位（无注入 ⇒ ""）
	InjectionSHA256   string   `json:"injection_sha256"`    // 注入块指纹全串（无注入 ⇒ ""）
	InjectionRound    int      `json:"injection_round"`     // 本次注入发生在第几轮（0 ⇒ 调用方没给轮次）
	InjectionClockISO string   `json:"injection_clock_iso"` // 注入时的**宿主注入时钟**（只进事件，不进提示——H6）
}

// ConstraintMissingObs — 告警事件里的一条缺失明细（"可行动"= 看得见是哪条、本该在哪、从哪来）。
type ConstraintMissingObs struct {
	ID          string `json:"id"`
	SHA256      string `json:"sha256"`
	MustSurvive bool   `json:"must_survive"`
	InjectWhere string `json:"inject_where"`
	SourceMsgID int64  `json:"source_msg_id"`
	Text        string `json:"text,omitempty"` // 截断 ≤ constraintAlertTextMax 字符（可读性；不落整段对话）
}

// constraintPresentOn — 一条约束是否出现在 haystack 上。
//
// 两条路（H2 逐字"canonical 或其哈希/指纹"）：
//  1. canonical 子串（两侧同一规范化 ⇒ 换行/空白/零宽不导致假性别离）；
//  2. 指纹：sha256 全串 **或** 前 16 位（注入方可能只带指纹不带原文；16 位足够定位且不撑爆提示词）。
//     指纹按小写比（十六进制注入惯例），canonical 原文按原样比（中文大小写无意义，英文标记词不参与检索）。
func constraintPresentOn(c Constraint, haystack, haystackLower string) bool {
	if c.Canonical != "" && strings.Contains(haystack, c.Canonical) {
		return true
	}
	if c.SHA256 != "" {
		if strings.Contains(haystackLower, c.SHA256) {
			return true
		}
		if len(c.SHA256) >= 16 && strings.Contains(haystackLower, c.SHA256[:16]) {
			return true
		}
	}
	return false
}

// missingConstraints — 在 haystack 上找**不在场**的约束（canonical 子串 或 指纹）。
// 顺序 = 登记顺序（可复算、可 diff；不用 map 迭代序）。**唯一口径**：
// checkConstraints（落账）与重注入的目标选择（见文末）都调它——两处口径若各写一份，判据就不可复算了。
func missingConstraints(cs []Constraint, haystack string) []Constraint {
	hayLower := strings.ToLower(haystack)
	var out []Constraint
	for _, c := range cs {
		if constraintPresentOn(c, haystack, hayLower) {
			continue
		}
		out = append(out, c)
	}
	return out
}

// checkConstraints — 在 haystack 上做存在性检索，产出事实块（**只描述给定字节**，不含重注入）。
// 为什么重注入不参与本块：missing 是"这一刻的提示缺了什么"的**事实**——若拿重注入后的提示再检一遍，
// missing 恒为空、告警事件消失，缺陷②那种"真缺失"就又看不见了（治本不许把诊断弄瞎）。
// missing_ids 顺序 = 登记顺序（可复算、可 diff；不用 map 迭代序）。
func checkConstraints(cs []Constraint, haystack string) ConstraintCheckObs {
	missing := missingConstraints(cs, haystack)
	obs := ConstraintCheckObs{
		Total:      len(cs),
		Present:    len(cs) - len(missing),
		MissingIDs: []string{}, MustSurviveMissingIDs: []string{},
		// 重注入字段的零值即"没注"（读侧三态：reinject_reason=none + injection_count=0 + 空 id）
		ReinjectReason: ReinjectReasonNone, ReinjectedIDs: []string{},
	}
	for _, c := range missing {
		obs.MissingIDs = append(obs.MissingIDs, c.ID)
		if c.MustSurvive {
			obs.MustSurviveMissingIDs = append(obs.MustSurviveMissingIDs, c.ID)
		}
	}
	obs.Alert = len(obs.MissingIDs) > 0
	return obs
}

// ── 重注入（治本：从"报警"到"补上"）────────────────────────────────────
//
// 依据（实测，别当理论）：`constraint_missing_alert` 抓到过**真缺失**——缺的正是任务书那句
// 「只做 G2，不要动别的缺陷，也不要动 git」（must_survive=true, inject_where=history），
// 而那一刻**实际发出的提示里确实检索不到它**；当时只有"检测 + 报警"、没有"治疗" ⇒
// 告警响了、模型照样不照做。这一节就是"治疗"那一半。
//
// 定案口径（调研后写死，改前先读；四条的出处见任务书）：
//
//	a) **每 3–5 轮重注入一次浓缩规则提醒** ⇒ 本文件默认 N=4（`ZERG_CONSTRAINT_REINJECT_EVERY` 可配）；
//	b) **关键约束同时放上下文的开头与结尾**（头一份 + 尾一份，逐字节同一块）；
//	c) 注入块**只放规则、不放示例、不带时间/随机** ⇒ 块内**不含轮次/时钟/随机数**：
//	   H6 禁"提示里进时间"，且字节稳定 ⇒ 我们自己不把前缀缓存打碎（同一约束集 ⇒ 同一块 ⇒ 同一字节）；
//	d) 关键约束走 pinned、永不参与摘要 —— 本文件的判据是 must_survive（抽取时 inject_where=history，
//	   显式登记时可由调用方给 injectWherePinned；两者都在"必须活着"这个集合里）。
//
// 幂等（"已存在的不得重复堆叠"）的实现口径 = **先剔后放**：
//
//	剔除 —— 只剔**我们自己**注入的块（标记 + 头行双条件；别人写了同形标记不会被误删）；
//	判在 —— 注入前对每条约束在当前整串上做存在性检索（missingConstraints，与落账同一函数、同一口径）；
//	        已在 ⇒ 本次不放它（避免"每轮叠一份"把提示词撑爆）；
//	结果 —— 同一 (提示, 约束集, 轮次) 再调一次 ⇒ **逐字节相同**（用例②钉住）。
//
// 放置形态（写死，strip 的可逆性就靠它）：
//
//	重注入后 = 块 ‖ "\n" ‖ **原字节** ‖ "\n" ‖ 块     （原字节原样、不动一个字符；不注入 ⇒ 逐字节等于入参）
//
// 口径边界（如实）：本函数**只认字节**——装配点把哪串交给它，头尾就是那串的头尾。当前唯一入口交的是
// **系统提示**（r.System）⇒ 头份在上下文最前、尾份紧挨历史之前（这是"开头与结尾"在本装配口径下的落点）；
// 若将来装配方要尾份落在整个提示的最末，把同一函数用在"含历史的整串"上即可，字节口径一个字都不用改。
const (
	reinjectMarkOpen   = "⟦zerg:硬约束⟧"
	reinjectMarkClose  = "⟦/zerg:硬约束⟧"
	reinjectHeader     = "【硬约束重注入】以下约束整场会话有效——直接照做，不要复述："
	reinjectLinePrefix = "- "
)

// reinjectEveryDefault — 定时重注入的默认周期（轮）。调研口径 a 给的区间是 3–5，取中值 4。
const reinjectEveryDefault = 4

// reinjectMaxSessions — 周期状态表的会话数上限（与约束登记表同源的有界策略：满则淘汰"最久没注入"的那个）
const reinjectMaxSessions = 512

// 重注入触发原因闭集（低基数、可枚举；事件里落字符串，读侧按它分派）
const (
	ReinjectReasonNone     = "none"     // 没注入（约束都在场 且 未到周期）
	ReinjectReasonMissing  = "missing"  // 刚检到 must_survive 缺失 ⇒ 立刻补
	ReinjectReasonPeriodic = "periodic" // 到周期（每 N 轮）⇒ 定时补一次
)

// reinjectEveryNRounds — 周期 N（可配）：`ZERG_CONSTRAINT_REINJECT_EVERY` 为正整数则用它，否则默认 4。
// 非法/非正/空 ⇒ 默认（不猜、不写死 0——0 会把定时重注整个关掉，那正是"忘了就没人管"）。
func reinjectEveryNRounds() int {
	if v := strings.TrimSpace(os.Getenv("ZERG_CONSTRAINT_REINJECT_EVERY")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return reinjectEveryDefault
}

// ReinjectOptions — 一次重注入的输入事实（**纯函数入参**：不读墙上时钟、不读全局态之外的任何东西）。
//
//	Session      —— 会话（周期状态的键；空 ⇒ 只做"刚检到缺失"的即时补充，不编造定时状态）
//	Round        —— 当前轮次（周期判据用；≤0 ⇒ 视为"调用方没给轮次" ⇒ 只做即时补充）
//	EveryNRounds —— 周期 N（≤0 ⇒ 取 reinjectEveryNRounds()）
//	ClockISO     —— 宿主注入的请求时钟（**只进事件，不进提示**——H6；空 ⇒ 事件里该键为空）
type ReinjectOptions struct {
	Session      string
	Round        int
	EveryNRounds int
	ClockISO     string
}

// Reinjection — 一次重注入的结果事实（装配点据此采纳 Prompt；事件据此落账）。
type Reinjection struct {
	Prompt          string   // 重注入后的提示（未重注入 ⇒ 与入参**逐字节相同**）
	Injected        bool     // 本次是否真的重注入了
	Reason          string   // none | missing | periodic
	IDs             []string // 本次注入的约束 id（**永不为 null**）
	Count           int      // 本次注入条数
	InjectionCount  int      // 本会话**截至本轮**的累计注入次数（会话级事实：未注入也回报现状；无会话 ⇒ 0）
	InjectionID     string   // 注入块指纹前 16 位（本轮没注入 ⇒ ""）
	InjectionSHA256 string   // 注入块指纹全串（本轮没注入 ⇒ ""）
	Round           int      // 本次注入发生在第几轮
	ClockISO        string   // 注入时的宿主注入时钟（只进事件）
}

// ReinjectConstraints — **治本入口**：给"当前待发提示 + 本会话约束"，回"重注入后的提示"。
//
// 判据（顺序写死，别换）：
//  1. 只看 must_survive（软约束不进注入块——否则块越长越像摘要，"浓缩规则提醒"就退化了）；
//  2. missing = 在当前整串上检索不到的那些（同一口径 missingConstraints）⇒ 有 ⇒ reason=missing，注入 missing；
//  3. 没 missing 但**到周期**（round - 最近一次注入轮次 ≥ N，起点锚 = 会话第 0 轮）⇒ reason=periodic，注入**全部** must_survive
//     （"浓缩规则提醒"要的是头尾都在，不只是缺的那条）；
//  4. 其余 ⇒ 一个字都不动（Reason=none，Prompt 逐字节等于入参）。
//
// 纯函数：除"周期状态表"（每会话一个计数 + 最近注入轮次）外无副作用；状态表**有界**（满则淘汰最久没注入的会话）。
func ReinjectConstraints(prompt string, cs []Constraint, opt ReinjectOptions) Reinjection {
	every := opt.EveryNRounds
	if every <= 0 {
		every = reinjectEveryNRounds()
	}
	res := Reinjection{
		Prompt: prompt, Reason: ReinjectReasonNone, IDs: []string{},
		Round: opt.Round, ClockISO: opt.ClockISO,
		InjectionCount: reinjectCountOf(opt.Session), // 会话级事实：本轮不注入也回报现状（读侧不用自己攒）
	}
	ms := make([]Constraint, 0, len(cs))
	for _, c := range cs {
		if c.MustSurvive && (c.Canonical != "" || c.Text != "") {
			ms = append(ms, c)
		}
	}
	if len(ms) == 0 {
		return res // 没有"必须活着"的约束 ⇒ 没什么可补的（不编造块）
	}
	var targets []Constraint
	switch missing := missingConstraints(ms, prompt); {
	case len(missing) > 0:
		targets, res.Reason = missing, ReinjectReasonMissing
	case reinjectDue(opt.Session, opt.Round, every):
		targets, res.Reason = ms, ReinjectReasonPeriodic
	default:
		return res
	}
	block := buildReinjectBlock(targets)
	base, _ := stripReinjectBlocks(prompt) // 先剔（把我们上次放的剔掉）⇒ 后面放的不会叠上去
	res.Prompt = block + "\n" + base + "\n" + block
	res.Injected = true
	res.InjectionSHA256 = ConstraintFingerprint(block) // 指纹口径**只此一处**（同一 canonical 同指纹）
	res.InjectionID = res.InjectionSHA256[:16]
	res.IDs = constraintIDsOf(targets)
	res.Count = len(targets)
	res.InjectionCount = noteReinjection(opt.Session, opt.Round)
	return res
}

// constraintIDsOf — 约束 id 列表（id 缺失 ⇒ 按 canonical 现算，不编造）
func constraintIDsOf(cs []Constraint) []string {
	out := make([]string, 0, len(cs))
	for _, c := range cs {
		id := c.ID
		if id == "" {
			id = ConstraintID(c.Canonical)
		}
		out = append(out, id)
	}
	return out
}

// buildReinjectBlock — 注入块原文（逐字节确定：同约束集 ⇒ 同块）。
// 块内**不带**轮次/时钟/随机（H6 + 前缀缓存：见本节口径 c）。**不截断**约束原文——
// 截断会让下一轮的在场检索（canonical 子串）认不出它，等于自己造一条永久 missing。
func buildReinjectBlock(cs []Constraint) string {
	var b strings.Builder
	b.WriteString(reinjectMarkOpen)
	b.WriteByte('\n')
	b.WriteString(reinjectHeader)
	b.WriteByte('\n')
	for _, c := range cs {
		text := c.Canonical
		if text == "" {
			text = CanonicalConstraint(c.Text)
		}
		if text == "" {
			continue
		}
		b.WriteString(reinjectLinePrefix)
		b.WriteString(text)
		b.WriteByte('\n')
	}
	b.WriteString(reinjectMarkClose)
	return b.String()
}

// stripReinjectBlocks — 逐字剔除**我们自己**注入过的块（返回剔除后的串 + 是否真的剔到了）。
//
// 双条件（防误删别人）：① 有我们的开/闭标记；② 标记之间的内容含我们的头行 reinjectHeader。
// 两条都满足才剔，且**连同紧邻的那一个分隔换行**一起剔（那个换行是放置时加的）——
// 于是 strip(块 ‖ "\n" ‖ base ‖ "\n" ‖ 块) == base，逐字节可逆（用例②钉住）。
func stripReinjectBlocks(prompt string) (string, bool) {
	if !strings.Contains(prompt, reinjectMarkOpen) {
		return prompt, false
	}
	var b strings.Builder
	rest, removed := prompt, false
	for {
		i := strings.Index(rest, reinjectMarkOpen)
		if i < 0 {
			break
		}
		w := rest[i+len(reinjectMarkOpen):]
		j := strings.Index(w, reinjectMarkClose)
		if j < 0 {
			break
		}
		end := i + len(reinjectMarkOpen) + j + len(reinjectMarkClose)
		if !strings.Contains(w[:j], reinjectHeader) {
			// 同形标记但不是我们的块 ⇒ 原样保留，继续往后找（别人的字节一个都不许动）
			b.WriteString(rest[:end])
			rest = rest[end:]
			continue
		}
		start := i
		stop := end
		// 剔除我们放置时加的那**一个**分隔换行：优先左邻（尾份），没有才取右邻（头份放在开头时）。
		// 为什么必须二选一而不是两边都剔：调用方若在尾份之后又接了自己的文本（下一轮拼接），
		// 那个文本自带的换行不是我们的 —— 两边都剔会吃掉它（strip 就不再可逆了，用例里钉住）。
		if start > 0 && rest[start-1] == '\n' {
			start--
		} else if stop < len(rest) && rest[stop] == '\n' {
			stop++
		}
		b.WriteString(rest[:start])
		rest = rest[stop:]
		removed = true
	}
	b.WriteString(rest)
	if !removed {
		return prompt, false
	}
	return b.String(), true
}

// ── 周期状态（每会话：累计注入次数 + 最近一次注入的轮次）────────────────────
//
// 为什么只放进程内（不落盘，与登记表不同）：周期是**运行期节奏**，不是"这条约束存在过"那种事实。
// 重启后锚点归零 ⇒ 下一轮 round ≥ N 时立刻补一次（宁可多补一次，不可长期不补）；
// 而"补了几次"这件事照样进事件（injection_count 从零重数——事件里同一会话会看到计数回绕，
// 读侧按"同一进程内的段"读，别把回绕当异常）。
type reinjectState struct {
	Count     int
	LastRound int
}

var (
	reinjectMu     sync.Mutex
	reinjectStates map[string]reinjectState
)

// reinjectDue — 到周期了吗（round - 锚点 ≥ N）。锚点 = 上次注入的轮次；没状态 ⇒ 0（会话起点）。
// 无会话/无轮次 ⇒ false（不编造节奏）。
func reinjectDue(session string, round, every int) bool {
	if session == "" || round <= 0 || every <= 0 {
		return false
	}
	reinjectMu.Lock()
	defer reinjectMu.Unlock()
	anchor := 0
	if st, ok := reinjectStates[session]; ok {
		anchor = st.LastRound
	}
	return round-anchor >= every
}

// reinjectCountOf — 本会话**截至此刻**的累计注入次数（会话级事实：本轮没注入也回报现状；无会话 ⇒ 0）
func reinjectCountOf(session string) int {
	if session == "" {
		return 0
	}
	reinjectMu.Lock()
	defer reinjectMu.Unlock()
	return reinjectStates[session].Count
}

// noteReinjection — 记一次注入（返回本会话累计次数）。无会话 ⇒ 0（不编造全局计数）。调用方不持锁。
func noteReinjection(session string, round int) int {
	if session == "" {
		return 0
	}
	reinjectMu.Lock()
	defer reinjectMu.Unlock()
	if reinjectStates == nil {
		reinjectStates = map[string]reinjectState{}
	}
	if _, ok := reinjectStates[session]; !ok && len(reinjectStates) >= reinjectMaxSessions {
		dropStalestReinjectLocked()
	}
	st := reinjectStates[session]
	st.Count++
	st.LastRound = round
	reinjectStates[session] = st
	return st.Count
}

// dropStalestReinjectLocked — 状态表满 ⇒ 淘汰"最近注入轮次最小"的会话（确定性地选一个，不靠 map 迭代序）。调用方持锁。
func dropStalestReinjectLocked() {
	victim, found := "", false
	for s, st := range reinjectStates {
		if !found || st.LastRound < reinjectStates[victim].LastRound || (st.LastRound == reinjectStates[victim].LastRound && s < victim) {
			victim, found = s, true
		}
	}
	if found {
		delete(reinjectStates, victim)
		log.Printf("⚠️ constr_reinject: 状态表已满(%d)，淘汰最久未注入的会话 %s", reinjectMaxSessions, victim)
	}
}

// ReinjectStats — 某会话的注入状态（诊断/用例：累计次数 + 最近注入轮次）
func ReinjectStats(session string) (count, lastRound int) {
	reinjectMu.Lock()
	defer reinjectMu.Unlock()
	st := reinjectStates[session]
	return st.Count, st.LastRound
}

// ResetReinjectState — 清周期状态（维护/用例用；幂等）。空串 ⇒ 全清。
func ResetReinjectState(session string) {
	reinjectMu.Lock()
	defer reinjectMu.Unlock()
	if session == "" {
		reinjectStates = nil
		return
	}
	delete(reinjectStates, session)
}

// ── 事件侧的装配（ObservePromptCheck 用：把纯函数结果写进事实块）────────────

// applyReinjectionToObs — 把一次重注入的结果写进 constraint_check 事实块（**只写事实**，不改检索口径）。
func applyReinjectionToObs(obs *ConstraintCheckObs, rej Reinjection) {
	obs.Reinjected = rej.Injected
	obs.ReinjectReason = rej.Reason
	obs.ReinjectedIDs = rej.IDs
	if obs.ReinjectedIDs == nil {
		obs.ReinjectedIDs = []string{} // 永不为 null（wire 层判据与 missing_ids 同源）
	}
	obs.InjectionCount = rej.InjectionCount
	obs.InjectionID = rej.InjectionID
	obs.InjectionSHA256 = rej.InjectionSHA256
	obs.InjectionRound = rej.Round
	obs.InjectionClockISO = rej.ClockISO
}

// ── 供测试与诊断的时钟口径（避免用例各自发明格式）───────────────────────

// ReinjectClockISO — 把时间格式化成事件里用的宿主注入时钟口径（UTC RFC3339Nano；与 RequestIdentity 同源）。
// 存在意义：调用方（含用例）把一个时点转成 clock_iso 时**不必**再写一遍格式串（口径只有一处）。
func ReinjectClockISO(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}

// missingDetailsOf — 把缺失 id 还原成可行动明细（id → 登记记录；找不到的 id 跳过，不编造）。
func missingDetailsOf(cs []Constraint, missingIDs []string) []ConstraintMissingObs {
	if len(missingIDs) == 0 {
		return nil
	}
	want := make(map[string]struct{}, len(missingIDs))
	for _, id := range missingIDs {
		want[id] = struct{}{}
	}
	out := make([]ConstraintMissingObs, 0, len(missingIDs))
	for _, c := range cs {
		if _, ok := want[c.ID]; !ok {
			continue
		}
		out = append(out, ConstraintMissingObs{
			ID: c.ID, SHA256: c.SHA256, MustSurvive: c.MustSurvive,
			InjectWhere: c.InjectWhere, SourceMsgID: c.SourceMsgID,
			Text: trunca(c.Text, constraintAlertTextMax),
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
