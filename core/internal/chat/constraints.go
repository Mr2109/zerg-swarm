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
//  1. **观测绝不改变对话行为**：本文件只登记/检索/写盘，不重注入、不改任何 prompt 字节、不阻断请求。
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
	"strings"
	"sync"

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

// ConstraintCheckObs — `constraint_check` 事件的事实块（H2 逐字三字段 + 两处口径扩展）。
//
// 扩展（写死，读侧据此判读）：
//
//	MustSurviveMissingIDs —— missing 里 must_survive=true 的子集（告警的主判据；全缺时两者相等）
//	Alert                  —— 与 len(MissingIDs)>0 恒等（把它显式写出来，读侧不必再推导）
type ConstraintCheckObs struct {
	Total                 int      `json:"total"`       // 登记表里本会话的约束总数（分母）
	Present               int      `json:"present"`     // 在**本次实际发出的提示**上检索到的条数（分子）
	MissingIDs            []string `json:"missing_ids"` // 没检索到的约束 id（**永不为 null**：空集合落 []）
	MustSurviveMissingIDs []string `json:"must_survive_missing_ids"`
	Alert                 bool     `json:"alert"`
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

// checkConstraints — 在 haystack 上做存在性检索，产出事实块。
// missing_ids 顺序 = 登记顺序（可复算、可 diff；不用 map 迭代序）。
func checkConstraints(cs []Constraint, haystack string) ConstraintCheckObs {
	obs := ConstraintCheckObs{Total: len(cs), MissingIDs: []string{}, MustSurviveMissingIDs: []string{}}
	hayLower := strings.ToLower(haystack)
	for _, c := range cs {
		if constraintPresentOn(c, haystack, hayLower) {
			obs.Present++
			continue
		}
		obs.MissingIDs = append(obs.MissingIDs, c.ID)
		if c.MustSurvive {
			obs.MustSurviveMissingIDs = append(obs.MustSurviveMissingIDs, c.ID)
		}
	}
	obs.Alert = len(obs.MissingIDs) > 0
	return obs
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
