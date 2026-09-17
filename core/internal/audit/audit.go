// Package audit — T5.9「独立审计层」：**与人批同寿**、**append-only（单调序号 + 前序哈希链）**、
// **写入前必经脱敏钩子**的批准/决策留痕账本。
//
// 要治的缺口（设计稿 docs/01-设计/设计-内建调试版-v1.2-20260917.md §〇 F14）：
// **★ 批准留痕不能挂在调试级别上** —— Q1 的默认取向是"生产默认 OFF"，于是任何"顺手记在
// 调试级别 / 观测采样 / 日志文件"里的审批记录，**在生产上就是完全没有**：
// 出了事既说不清"谁批的"，也说不清"哪条规则判的、当时是什么档位"。本层把三件事钉死：
//
//	① **与人批同寿**：生命周期绑定"有人批/有人问"这件事本身，**不绑** ZERG_DEBUG_LEVEL、
//	   不绑观测采样率、不绑日志级别。本包**从不读任何环境变量**（用例把这个面钉住了 —— 见
//	   audit_test.go 的 TestAudit_SurvivesDebugLevelOff 与源码扫描守卫）：
//	   "审计层还在不在"这件事不该由环境开关决定，也不该由某个包忘了初始化决定。
//	② **append-only**：单调序号（1 起、连续、不跳）+ **前序哈希链**（每条含上一条的哈希）。
//	   本包**不提供 Update / Delete / 覆盖写接口** —— 不是"没实现"，是**不提供**：
//	   要改历史只能绕过本包去改存储，而那样 `Verify()` 一定抓到（改一条 ⇒ 内容哈希对不上；
//	   删一条 ⇒ 序号跳号 + 链断）。
//	③ **写入前必经脱敏**：`Append` 必须拿到调用方注入的 `Redactor` 钩子，**每一条**都过一遍；
//	   没接钩子 / 钩子报错 / 钩子删键加键 ⇒ **拒写**（fail-closed）。
//	   **本包不实现脱敏**（脱敏口径属 T2.x 的 internal/obs/redact）：本包是零依赖叶包，只留接口，
//	   并在此注明：**生产必须接脱敏器**（照抄 T2.1 的纪律：先脱敏、后编码/后哈希）。
//
// ── 字段（每条记录至少这些；对齐 gen_ai.* 见 GenAIAttrs）──
//
//	主体 Subject          —— 谁触发的（会话 id / 用户标识 / 子代理 id / "ci-bootstrap"…）
//	工具名 + 入参指纹     —— Tool / ArgsDigest（**只存指纹不存原文**：审计要"同不同"，不要"内容"）
//	决策 Decision         —— allow | deny | ask | approve | reject | correct | escalate（七类，无第八类）
//	依据                  —— RuleID（命中的规则 id）或 Judge（判官：人批=user / 模型判官=judge:<名>）：
//	                         **两个都空 ⇒ 拒写**（说不清"凭什么"的记录不是审计记录，是噪音）
//	生效模式              —— Mode（档位）+ ModeEffect（**档位对本次判定做了什么**，如 dontask:ask→deny）
//	时间 At               —— 调用方注入（本包**不读系统钟**：与 policy 的判定同一纪律）
//	效果幂等键            —— IdempotencyKey（F6：效果账本按键对齐；同一次副作用只能有一个键）
//
// ── 为什么"幂等键重复"不拒写 ──
//
//	同一次调用被重放、同一次询问被重问，都会产生**新的**审计记录（它们真的发生过）。
//	拒写等于丢事实。判重是**效果账本**的事：`LookupIdempotency(key)` 只提供查询，不下判断。
//
// ── 本批边界 ──
//
//	· 零依赖叶包：只 import 标准库（crypto/sha256 · encoding/hex · encoding/json · fmt ·
//	  sort · strconv · strings · sync · time）。**不 import 仓库内任何其它包**（可被任何层引用）。
//	· **不接任何执行路径**：不碰 toolobs / chat / gateway / policy / agent（一行不动）。
//	· **落盘属接线批**：本包提供账本本体 + 序列化（`Snapshot` / `MarshalJSON` / `LoadAudit`）；
//	  写到哪个文件、怎么轮转属接线批（禁写死私有路径 —— 用 statepath.Dir()，见 T1/T2 的纪律）。
//	· **导出到 SIEM**：`ExportJSONL` 给逐行 JSON、`GenAIAttrs` 给标准属性名映射；
//	  semconv **版本标记**属 T6.1（本包不猜版本号）。
package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// StateVersion — 审计账本的序列化格式版本。读到的版本不认识 ⇒ **报错**（fail-closed：
// 绝不按"空账本"载入 —— 那等于把全部审批留痕一次静默丢弃，比不记还坏）。
const StateVersion = 1

// ── 决策（七类，无第八类）──────────────────────────────────────────────────

// Decision — 一条审计记录里的决策取值。**低基数**（可聚合、可告警）：自由文本一律进 Note / Attrs。
//
// 两段语义（与 policy.Effect 是**引用关系**而非别名 —— 本包零依赖，不 import policy）：
//
//	判定段（系统判的）  allow | deny | ask
//	人批段（人判的）    approve | reject | correct | escalate
//
// `correct` = 更正（F10：改参数后继续）；`escalate` = 升级（F15：交给更高一档的人/策略）。
type Decision string

const (
	DecisionAllow    Decision = "allow"    // 判定层放行
	DecisionDeny     Decision = "deny"     // 判定层拒绝（**含"批准超时 ⇒ 拒绝"**）
	DecisionAsk      Decision = "ask"      // 判定层要问人（还没问到）
	DecisionApprove  Decision = "approve"  // 人批：同意
	DecisionReject   Decision = "reject"   // 人批：拒绝（工具不执行）
	DecisionCorrect  Decision = "correct"  // 人批：更正后继续（F10）
	DecisionEscalate Decision = "escalate" // 升级到更高一档（F15）
)

// Valid — 是否七类之一（认不出一律**拒写**：审计的决策字段不许出现自由文本）。
func (d Decision) Valid() bool {
	switch d {
	case DecisionAllow, DecisionDeny, DecisionAsk, DecisionApprove, DecisionReject, DecisionCorrect, DecisionEscalate:
		return true
	}
	return false
}

// ParseDecision — 解析决策文本（大小写不敏感、去首尾空白）；认不出一律**报错**。
func ParseDecision(raw string) (Decision, error) {
	d := Decision(strings.ToLower(strings.TrimSpace(raw)))
	if !d.Valid() {
		return "", fmt.Errorf("未知决策「%s」（只认 allow|deny|ask|approve|reject|correct|escalate）", strings.TrimSpace(raw))
	}
	return d, nil
}

// ── 脱敏钩子（**必接**；本包不实现脱敏）──────────────────────────────────────

// Redactor — **脱敏钩子接口**（唯一形态：写入前必经）。
//
//	Redact(fields) —— 收到本条记录**待脱敏的字符串字段**（键固定，见 redactableKeys），
//	                  返回脱敏后的同键集合。**只许改值，不许删键、不许加键**（本包会校验）。
//	                  返回 error ⇒ **本包拒写**（fail-closed：绝不回退写原文）。
//	Name()         —— 脱敏器标识（写进每条记录的 Redactor 字段：审计要能回答
//	                  "这条是谁脱敏的、脱敏器是哪一版"；脱敏口径变了要能分辨）。
//
// **生产必须接脱敏器**：本包只留接口，不提供 Noop / Keep 之类的"空脱敏器"
// （提供它等于给"偷工"留一扇门：出事时没人能证明脱敏发生过）。
// 接线批的接法：适配 `core/internal/obs/redact`（同一纪律：**先脱敏，后编码/后哈希**）——
// 那份实现按**键分类表**工作，与本接口的固定键集合天然对齐。
//
// ⚠ 类型化 nil（`(*T)(nil)` 装进接口）本包测不出，会在 Redact 里 panic —— 接线批注入时
// 必须保证"非 nil 且可用"（本包只能挡住**接口值为 nil**这一种，见 Append）。
type Redactor interface {
	Redact(fields map[string]string) (map[string]string, error)
	Name() string
}

// redactableKeys — 过脱敏的字符串字段（**固定集合**，改它必须同步改用例）。
//
// 为什么枚举字段（decision / mode / mode_effect）**不**过脱敏：它们是**低基数、可聚合**的判据字段
// （按 decision 分组出报表、按 mode_effect 追责），脱敏器改它们等于毁掉审计的可聚合性
// （把 "deny" 遮成 "***" 之后，这份账本还剩什么用？）。它们本身也不承载内容/秘密。
var redactableKeys = []string{
	"subject",
	"tool",
	"args_digest",
	"rule_id",
	"judge",
	"idempotency_key",
	"note",
}

// attrPrefix — Attrs 里的键过脱敏时用的前缀（`attr.<键>`），避免与固定键撞名。
const attrPrefix = "attr."

// ── 记录 ────────────────────────────────────────────────────────────────────

// Record — 一条**审计记录**。Seq / PrevHash / Hash / Redactor 由账本写（调用方填了会被拒 —— 见 Append）。
type Record struct {
	// Seq — 单调序号（1 起、连续、不跳）。由账本定死：这是 append-only 的第一半。
	Seq int64 `json:"seq"`

	// At — 记录时刻（**由调用方注入**，本包不读系统钟）。零值 ⇒ 拒写。
	At time.Time `json:"at"`

	// Subject — 主体：谁触发的这次决策/批准。
	Subject string `json:"subject"`

	// Tool — 工具名（非工具决策留空）。
	Tool string `json:"tool,omitempty"`

	// ArgsDigest — **入参指纹**（见 ArgsFingerprint）。**不存原文**：审计要的是"同不同"，不是"内容"。
	ArgsDigest string `json:"args_digest,omitempty"`

	// Decision — 决策（七类，见 Decision）。
	Decision Decision `json:"decision"`

	// RuleID — 依据①：命中的规则 id（内置条目 `any.*` / `never.*`、地板条目、文本规则定位串）。
	RuleID string `json:"rule_id,omitempty"`

	// Judge — 依据②：判官（人批填 `user` / 具体用户标识；模型判官填 `judge:<名字>`）。
	// RuleID 与 Judge **至少一个非空**（否则拒写）。
	Judge string `json:"judge,omitempty"`

	// Mode — 生效模式（会话档位：default | bypass | dontask | …）。
	Mode string `json:"mode,omitempty"`

	// ModeEffect — **档位对本次判定做了什么**（如 `dontask:ask→deny` / `default:无命中⇒ask` /
	// `floor:压过档位`）。它是"为什么最后是这个决策"的那一句。
	ModeEffect string `json:"mode_effect,omitempty"`

	// IdempotencyKey — **效果幂等键**（F6：效果账本按键对齐，同一次副作用只有一个键）。
	IdempotencyKey string `json:"idempotency_key"`

	// Note — 人类可读补充（过脱敏；仅取证，程序判据只看上面的字段）。
	Note string `json:"note,omitempty"`

	// Attrs — 附加字段（键值都是字符串；过脱敏，键前缀 `attr.` 进脱敏面）。
	// 用途：对齐 gen_ai.* 属性、挂 trace/span id、挂升级链路（F15）等。
	Attrs map[string]string `json:"attrs,omitempty"`

	// Redactor — 这条记录是哪个脱敏器处理的（脱敏器标识；由 Append 从 Redactor.Name() 填）。
	Redactor string `json:"redactor,omitempty"`

	// PrevHash — 前一条的哈希（第一条为空串）。append-only 的第二半：链。
	PrevHash string `json:"prev_hash"`

	// Hash — 本条哈希 = sha256(前序哈希 ‖ 规范化字段)。**改一个字节 ⇒ 重算不上**。
	Hash string `json:"hash"`
}

// hashTag — 哈希域的域分隔标签（换算法/换字段集时必须改它 ⇒ 旧账本的 Verify 会明确报"哈希派生方式变了"，
// 而不是含糊地"对不上"）。
const hashTag = "zerg-audit-v1"

// hashRecord — 单条记录的哈希：sha256(域标签 ‖ 逐字段**长度前缀**编码)。
//
// 两条写死的细节：
//
//	① **长度前缀**（`<键长>:<键>=<值长>:<值>`）：避免拼接歧义 —— 不然改一个字段的边界
//	   与改两个字段可能算出同一个哈希（`a|b` vs `ab|`）。
//	② **Attrs 按键名排序**：map 迭代序是随机的 ⇒ 不排序会让同一份内容算出两个哈希
//	   （Verify 会莫名其妙地红，而且只在有 Attrs 时红）。
//
// 覆盖范围 = 除 Hash 自身以外的**全部字段**（含 Seq / PrevHash / At / Redactor）。
func hashRecord(r Record) string {
	var b strings.Builder
	b.WriteString(hashTag)
	b.WriteString("\x1f")
	seg := func(k, v string) {
		b.WriteString(strconv.Itoa(len(k)))
		b.WriteString(":")
		b.WriteString(k)
		b.WriteString("=")
		b.WriteString(strconv.Itoa(len(v)))
		b.WriteString(":")
		b.WriteString(v)
		b.WriteString("\x1f")
	}
	seg("seq", strconv.FormatInt(r.Seq, 10))
	seg("at", r.At.UTC().Format(time.RFC3339Nano)) // 统一 UTC + 纳秒：跨时区/跨进程重算一致
	seg("subject", r.Subject)
	seg("tool", r.Tool)
	seg("args_digest", r.ArgsDigest)
	seg("decision", string(r.Decision))
	seg("rule_id", r.RuleID)
	seg("judge", r.Judge)
	seg("mode", r.Mode)
	seg("mode_effect", r.ModeEffect)
	seg("idempotency_key", r.IdempotencyKey)
	seg("note", r.Note)
	seg("redactor", r.Redactor)
	seg("prev_hash", r.PrevHash)
	keys := make([]string, 0, len(r.Attrs))
	for k := range r.Attrs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		seg(attrPrefix+k, r.Attrs[k])
	}
	sum := sha256.Sum256([]byte(b.String()))
	return "sha256:" + hex.EncodeToString(sum[:])
}

// ArgsFingerprint — **入参指纹**的**统一口径**（policy 的 Pending.ArgsDigest 与审计记录都用它，
// 两处必须同源，否则同一个调用在两个面里对不上）。
//
//	· 载荷**永不进记录**：只留 sha256 十六进制串（前缀 `sha256:`）；审计要的是"同不同"。
//	· 确定性：json.Marshal 对 map 的键**按字典序**输出 ⇒ 同参数字典必然同指纹；
//	  值不可序列化（chan/func/NaN）⇒ 返回错误（调用方自己决定怎么记"算不出指纹"这件事）。
func ArgsFingerprint(args map[string]any) (string, error) {
	if args == nil {
		return "", nil // 空 = 没有入参（不是"算不出"）
	}
	b, err := json.Marshal(args)
	if err != nil {
		return "", fmt.Errorf("入参指纹：参数不可序列化 ⇒ 算不出指纹（不要把原文当指纹写进审计）：%w", err)
	}
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

// ── 账本 ────────────────────────────────────────────────────────────────────

// Store — 审计账本（**只增不改**）。并发安全（RWMutex）。
//
// ⚠ 与 policy 的判定同一纪律：**时间由调用方注入**（`Record.At`），本包不读系统钟、
// 不读环境变量 —— 于是"记了什么"完全由调用方给出的输入决定（可测、可回放、可核对）。
type Store struct {
	mu       sync.RWMutex
	redactor Redactor
	records  []Record
	// keys — 幂等键 ⇒ 首次出现的下标（**只查询用**，不拒写：见文件头"为什么幂等键重复不拒写"）。
	keys map[string]int
}

// NewStore — 建账本并绑定脱敏钩子。`red == nil` 时账本可以建（载入/取证场景要能读），
// 但**任何 Append 都会被拒**（"生产必须接脱敏器"，见 Redactor）。
func NewStore(red Redactor) *Store {
	return &Store{redactor: red, keys: map[string]int{}}
}

// SetRedactor — 换脱敏钩子（例如配置热更新后换了脱敏器版本）。会影响**之后**的每一条记录。
func (s *Store) SetRedactor(red Redactor) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.redactor = red
}

// RedactorName — 当前脱敏器标识（空串 = 没接）。
func (s *Store) RedactorName() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.redactor == nil {
		return ""
	}
	return s.redactor.Name()
}

// Append — 记一条。**append-only**：只追加、由账本填 Seq/PrevHash/Hash/Redactor。
//
// 拒写条件（全部 fail-closed，逐条有用例）：
//
//	· **没接脱敏器**（接口值为 nil）⇒ 拒写 —— "生产必须接脱敏器"，不留后门
//	· 脱敏器报错 / 返回 nil / **删键** / **加键** ⇒ 拒写（脱敏器只许改值）
//	· At 零值（时间必须注入）· Subject 空（说不清是谁）· Decision 非法（自由文本不许进决策字段）
//	· RuleID 与 Judge 都空（**说不清"凭什么"**）
//	· IdempotencyKey 空（效果账本对不上账）
//	· 调用方自己填了 Seq / Hash / PrevHash / Redactor（这些只能由账本写 —— 否则改历史可从接口绕进来）
//
// 返回**落库后的那条**（Seq / Hash / Redactor 已填），调用方必须用返回值。
func (s *Store) Append(rec Record) (Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.redactor == nil {
		return Record{}, fmt.Errorf("审计拒写：**未接脱敏器**（Redactor 为 nil）⇒ 审计记录必经脱敏，" +
			"生产必须接脱敏器（接线批接 internal/obs/redact；本包不提供空脱敏器）")
	}
	if rec.At.IsZero() {
		return Record{}, fmt.Errorf("审计拒写：At 为零值 ⇒ 时间必须由调用方注入（本包不读系统钟，否则不可回放）")
	}
	if strings.TrimSpace(rec.Subject) == "" {
		return Record{}, fmt.Errorf("审计拒写：主体（subject）为空 ⇒ 记了也说不清「谁触发的」")
	}
	if !rec.Decision.Valid() {
		return Record{}, fmt.Errorf("审计拒写：决策「%s」非法（只认 allow|deny|ask|approve|reject|correct|escalate）",
			string(rec.Decision))
	}
	if strings.TrimSpace(rec.RuleID) == "" && strings.TrimSpace(rec.Judge) == "" {
		return Record{}, fmt.Errorf("审计拒写：依据为空（rule_id 与 judge 都是空）⇒ 说不清「凭什么」的记录不是审计记录")
	}
	if strings.TrimSpace(rec.IdempotencyKey) == "" {
		return Record{}, fmt.Errorf("审计拒写：效果幂等键（idempotency_key）为空 ⇒ 效果账本对不上账")
	}
	if rec.Seq != 0 || rec.Hash != "" || rec.PrevHash != "" || rec.Redactor != "" {
		return Record{}, fmt.Errorf("审计拒写：seq/hash/prev_hash/redactor 只能由账本写（调用方填了 ⇒ 拒绝，" +
			"否则改历史可以绕过 append-only 从接口进来）")
	}

	// ① 先脱敏（**在哈希之前**：否则哈希锁住的是原文，改历史反而"看起来对"）。
	sanitized, err := s.redactLocked(rec)
	if err != nil {
		return Record{}, err
	}
	rec = sanitized

	// ② 再落到账本尾部：序号 = 尾部 +1（连续、不跳），前序哈希 = 尾部哈希。
	rec.Seq = int64(len(s.records)) + 1
	if n := len(s.records); n > 0 {
		rec.PrevHash = s.records[n-1].Hash
	}
	rec.Redactor = s.redactor.Name()
	rec.Hash = hashRecord(rec)

	if _, ok := s.keys[rec.IdempotencyKey]; !ok {
		s.keys[rec.IdempotencyKey] = len(s.records)
	}
	s.records = append(s.records, rec)
	return cloneRecord(rec), nil
}

// redactLocked — 过一遍脱敏钩子并把结果回写到记录上。调用方必须已持写锁（并发下调 Redact 也不该乱序）。
//
// 契约（写死，且都有用例）：**只许改值** —— 少一个键（删键）或多一个键（加键）都拒写。
// 理由：删键会让"这条记录有什么"变得不可预期（读的人分不清"没采到"和"被删了"）；
// 加键等于让脱敏器往审计里塞未经审计的字段。
func (s *Store) redactLocked(rec Record) (Record, error) {
	// 待脱敏字段集合（固定键 + attr.<键>）。
	fields := make(map[string]string, len(redactableKeys)+len(rec.Attrs))
	for _, k := range redactableKeys {
		switch k {
		case "subject":
			fields[k] = rec.Subject
		case "tool":
			fields[k] = rec.Tool
		case "args_digest":
			fields[k] = rec.ArgsDigest
		case "rule_id":
			fields[k] = rec.RuleID
		case "judge":
			fields[k] = rec.Judge
		case "idempotency_key":
			fields[k] = rec.IdempotencyKey
		case "note":
			fields[k] = rec.Note
		}
	}
	for k, v := range rec.Attrs {
		fields[attrPrefix+k] = v
	}

	out, err := s.redactor.Redact(fields)
	if err != nil {
		return Record{}, fmt.Errorf("审计拒写：脱敏器报错（**绝不回退写原文**，fail-closed）：%w", err)
	}
	if out == nil {
		return Record{}, fmt.Errorf("审计拒写：脱敏器返回 nil ⇒ 判不了脱敏结果（**绝不回退写原文**）")
	}
	for k := range fields {
		if _, ok := out[k]; !ok {
			return Record{}, fmt.Errorf("审计拒写：脱敏器**删掉了**字段 %q（脱敏器只许改值，不许删键/加键）", k)
		}
	}
	for k := range out {
		if _, ok := fields[k]; !ok {
			return Record{}, fmt.Errorf("审计拒写：脱敏器**凭空加了**字段 %q（字段集固定，脱敏器只许改值）", k)
		}
	}

	rec.Subject = out["subject"]
	rec.Tool = out["tool"]
	rec.ArgsDigest = out["args_digest"]
	rec.RuleID = out["rule_id"]
	rec.Judge = out["judge"]
	rec.IdempotencyKey = out["idempotency_key"]
	rec.Note = out["note"]
	if len(rec.Attrs) > 0 {
		attrs := make(map[string]string, len(rec.Attrs))
		for k := range rec.Attrs {
			attrs[k] = out[attrPrefix+k]
		}
		rec.Attrs = attrs
	}
	return rec, nil
}

// cloneRecord — 深拷贝（Attrs 是 map ⇒ 必须复制，否则调用方能改到账本内部状态）。
func cloneRecord(r Record) Record {
	if r.Attrs == nil {
		return r
	}
	attrs := make(map[string]string, len(r.Attrs))
	for k, v := range r.Attrs {
		attrs[k] = v
	}
	r.Attrs = attrs
	return r
}

// ── 读（append-only ⇒ 只有"读"与"校验"，没有"改"）──────────────────────────

// Read — 前 n 条（`n <= 0` 或超过总数 ⇒ 全部）。返回**深拷贝**：调用方改返回值不污染账本。
func (s *Store) Read(n int) []Record {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if n <= 0 || n > len(s.records) {
		n = len(s.records)
	}
	out := make([]Record, 0, n)
	for _, r := range s.records[:n] {
		out = append(out, cloneRecord(r))
	}
	return out
}

// Len — 当前有几条。
func (s *Store) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.records)
}

// LookupIdempotency — 按效果幂等键找**首次出现**的那条（效果账本对齐用）。
// 第二个返回值为 false = 这个键没出现过。**只查询、不下判断**（重复键不拒写，见文件头）。
func (s *Store) LookupIdempotency(key string) (Record, bool) {
	key = strings.TrimSpace(key)
	if key == "" {
		return Record{}, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	i, ok := s.keys[key]
	if !ok || i >= len(s.records) {
		return Record{}, false
	}
	return cloneRecord(s.records[i]), true
}

// Verify — **校验哈希链**（本包最重要的读操作）。逐条：
//
//	① 序号必须是 1,2,3…（**连续、不跳**）：跳号 ⇒ 有记录被删掉/重排
//	② 本条 PrevHash 必须等于上一条 Hash：不等 ⇒ 链断（插入/替换/换序）
//	③ 本条内容重算的哈希必须等于本条 Hash：不等 ⇒ **这条历史被改写过**
//
// 返回第一条坏记录的**序号 + 原因**（取证时"从哪一条开始不对"比"对不上"有用得多）。
// 空账本 ⇒ 通过（没记过 ≠ 记坏了）。
func (s *Store) Verify() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return verifyRecords(s.records)
}

// verifyRecords — Verify 的本体（拆出来便于 LoadAudit 在装入前核对一遍）。
func verifyRecords(records []Record) error {
	prev := ""
	for i, r := range records {
		want := int64(i + 1)
		if r.Seq != want {
			return fmt.Errorf("审计链断在位置 %d：seq=%d（期望 %d）⇒ 有记录被删除或重排（append-only 被破坏）",
				i+1, r.Seq, want)
		}
		if r.PrevHash != prev {
			return fmt.Errorf("审计链断在 seq=%d：prev_hash=%s ≠ 上一条 hash=%s ⇒ 链被改（插入/替换/换序）",
				r.Seq, shortHash(r.PrevHash), shortHash(prev))
		}
		if got := hashRecord(r); got != r.Hash {
			return fmt.Errorf("审计记录 seq=%d 的内容与自身哈希不符（**历史被改写**）：hash=%s 重算=%s",
				r.Seq, shortHash(r.Hash), shortHash(got))
		}
		prev = r.Hash
	}
	return nil
}

// shortHash — 回显用短哈希（错误信息里放全 64 位会淹掉真正的原因）。
func shortHash(h string) string {
	if h == "" {
		return "(空)"
	}
	if len(h) > 23 {
		return h[:23] + "…"
	}
	return h
}

// ── 序列化（落盘属接线批；本包只给写入侧/读取侧）────────────────────────────

// State — 账本的序列化形态。
type State struct {
	Version int      `json:"version"` // 必须 == StateVersion
	Records []Record `json:"records"` // 按 Seq 序（append-only 的自然序）
}

// Snapshot — 导出可序列化状态（含全部事实；JSON 由调用方落盘）。
func (s *Store) Snapshot() State {
	return State{Version: StateVersion, Records: s.Read(0)}
}

// MarshalJSON — 账本 ⇒ JSON（落盘的写入侧）。
func (s *Store) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.Snapshot())
}

// LoadAudit — JSON ⇒ 账本（落盘的读取侧）。fail-closed 三条：
//
//	① JSON 坏 / 版本不认识 ⇒ 报错（绝不按"空账本"载入）
//	② 序号不连续 / 链断 / 内容与哈希不符 ⇒ **报错**（`Verify` 通过才允许装入：
//	   载入是"把审计接回服务"的那一步，带着坏链接回来等于让后面所有记录都建在坏链上）
//	③ 字段取值非法（决策不是七类之一）⇒ 报错（不猜）
//
// `red` 允许为 nil（**读不需要脱敏器**）：载入是为了查询/取证；但之后任何 Append 都会被拒
// （"生产必须接脱敏器"）—— 要么用 SetRedactor 接上，要么只能读。
func LoadAudit(data []byte, red Redactor) (*Store, error) {
	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, fmt.Errorf("审计账本载入失败（JSON 坏）⇒ 拒绝启用（不回退成「空账本」）：%w", err)
	}
	if st.Version != StateVersion {
		return nil, fmt.Errorf("审计账本版本 %d 不认识（本实现只认 %d）⇒ 拒绝启用（不猜格式）",
			st.Version, StateVersion)
	}
	for i, r := range st.Records {
		if !r.Decision.Valid() {
			return nil, fmt.Errorf("审计账本载入失败：第 %d 条的决策「%s」非法 ⇒ 拒绝启用（不猜取值）",
				i+1, string(r.Decision))
		}
	}
	if err := verifyRecords(st.Records); err != nil {
		return nil, fmt.Errorf("审计账本载入失败：哈希链校验不过 ⇒ 拒绝启用（坏链不许接回服务）：%w", err)
	}

	s := NewStore(red)
	s.records = make([]Record, 0, len(st.Records))
	for _, r := range st.Records {
		s.records = append(s.records, cloneRecord(r))
		if _, ok := s.keys[r.IdempotencyKey]; !ok {
			s.keys[r.IdempotencyKey] = len(s.records) - 1
		}
	}
	return s, nil
}

// ── 导出（SIEM / gen_ai.* 对齐）──────────────────────────────────────────────

// ExportJSONL — 逐行 JSON（一行一条，按 Seq 序）：给 SIEM / 日志管道的导出口。
// 空账本 ⇒ 空切片（不是 nil 也不是报错）。
func (s *Store) ExportJSONL() ([]byte, error) {
	recs := s.Read(0)
	var b strings.Builder
	for _, r := range recs {
		line, err := json.Marshal(r)
		if err != nil {
			return nil, fmt.Errorf("审计导出失败：第 %d 条序列化失败：%w", r.Seq, err)
		}
		b.Write(line)
		b.WriteByte('\n')
	}
	return []byte(b.String()), nil
}

// GenAIAttrs — 一条记录的 **gen_ai.\* 对齐**映射（导出到 SIEM 时用标准属性名，
// 非标准字段统一加 `zerg.` 前缀，便于下游按前缀分流）。
//
//	gen_ai.operation.name   —— 固定 "execute_tool"（本层的记录都挂在一次工具决策上）
//	gen_ai.tool.name        —— 工具名
//	gen_ai.tool.call.id     —— 效果幂等键（一次副作用一个 id）
//	gen_ai.agent.id         —— 主体（会话/用户/子代理标识）
//	zerg.*                  —— 决策 / 依据 / 生效模式 / 入参指纹 / 序号 / 时刻
//
// semconv **版本标记**不在这里（属 T6.1：本包不猜版本号）。
func (r Record) GenAIAttrs() map[string]string {
	m := map[string]string{
		"gen_ai.operation.name": "execute_tool",
		"gen_ai.agent.id":       r.Subject,
		"gen_ai.tool.call.id":   r.IdempotencyKey, // 一次副作用一个 id（幂等键就是它）
		"zerg.seq":              strconv.FormatInt(r.Seq, 10),
		"zerg.decision":         string(r.Decision),
		"zerg.idempotency_key":  r.IdempotencyKey,
	}
	if r.Tool != "" {
		m["gen_ai.tool.name"] = r.Tool
	}
	if r.ArgsDigest != "" {
		m["zerg.args_digest"] = r.ArgsDigest
	}
	if r.RuleID != "" {
		m["zerg.basis.rule_id"] = r.RuleID
	}
	if r.Judge != "" {
		m["zerg.basis.judge"] = r.Judge
	}
	if r.Mode != "" {
		m["zerg.mode"] = r.Mode
	}
	if r.ModeEffect != "" {
		m["zerg.mode_effect"] = r.ModeEffect
	}
	if !r.At.IsZero() {
		m["zerg.at"] = r.At.UTC().Format(time.RFC3339Nano)
	}
	if r.Hash != "" {
		m["zerg.audit.hash"] = r.Hash
	}
	if r.Redactor != "" {
		m["zerg.audit.redactor"] = r.Redactor
	}
	for k, v := range r.Attrs {
		m[attrPrefix+k] = v
	}
	return m
}
