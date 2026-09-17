// posture.go — T5.4「无人值守姿态」：dontAsk（凡会弹窗的一律拒）与 yolo（把问人工具从工具表**摘掉**）。
//
// 要治的缺口（设计稿 docs/01-设计/设计-内建调试版-v1.2-20260917.md §〇 F4）：
// 无人值守时（CI / 夜间自动跑）没有人在屏幕前。两种缺法都要治：
//
//	· 缺 dontAsk ⇒ 判定里出现的 ask 没人可批 ⇒ 卡住到超时，或（更坏）被当成同意；
//	· 缺 yolo   ⇒ 工具表里还挂着问人工具 ⇒ 模型**白跑一轮**才拿到"没人可问"的答案。
//
// ── 两档姿态（+ 常态档）──
//
//	normal —— 常态：**什么都不做**（判定与工具表原样返回）。这是交互式会话的姿态。
//	dontAsk —— 凡会弹窗的一律拒：所有判定结果里的 **ask ⇒ deny**（不执行）。
//	          改了的是"要不要执行"，不是"要不要记" —— 原因码仍写 dontask_deny（与 T5.1 的档位同名同义：
//	          同一件事实只允许一个原因码，避免同一个"无人值守拒批"在两处各有各的字符串）。
//	yolo   —— **把问人工具从工具表里摘掉**（按能力标记 asks_human 或保留名）。
//	          摘掉是**有意**的：问题从"模型问人"变成"模型根本没有问人这个动作" ⇒ 省掉一次**必然失败**的往返。
//
// ── 三条写死的语义（每条都有用例钉住）──
//
//	① **yolo 只改工具表、不改判定**：它不把 ask 改写成 allow（那会越过地板 —— T5.1 的地板压过一切），
//	   也不改写成 deny。摘掉问人工具之后残留的 ask（来自规则/地板）怎么处理，属 T5.8「批准超时=拒绝」，
//	   本批**不猜**（写在这里，是为了让接线批一眼看到这个缺口，而不是让它静默存在）。
//	② **normal 是惰性的**：不摘任何工具、不把 ask 变 deny、不把 ask 变 allow。管线里加一层姿态
//	   **不许**凭空改变任何判定 —— 否则"装了策略引擎"这件事本身就开始有副作用了。
//	③ **姿态取值认不出 ⇒ 按最严处理**（空串 / 拼错的档位 / 未认识的档位值 ⇒ 按 dontAsk 语义走：
//	   会弹窗的一律拒）。理由与 T5.1 的 fail-closed 同源：姿态是**无人值守**的时候更该严的那一维，
//	   写错了按"最宽"解释等于把无人值守悄悄变成全放。**注意**：认不出时**不改工具表**
//	   （摘工具是显式意图才做的事，不该由一次拼写错误顺带做掉）。
//
// ── 跨扇出与跨重启（F4 的"沿子 agent/扇出继承、跨重启存活"）──
//
//	· **扇出继承**：`Inherit()` 给出子代理该拿的姿态 —— 只有 `InheritedByFanout=true` 才把当前姿态传下去，
//	  否则子代理回到**常态**。写死"默认不继承"的理由：父代理开了 yolo 之后，扇出的每个子代理
//	  都会在**用户的视线之外**跑；默认继承 = 一次开关静默放大到整棵子树。要让无人值守姿态沿扇出生效，
//	  必须显式声明 InheritedByFanout（声明本身就是一条可审计的意图）。
//	· **跨重启存活**：姿态本身是三个字段，`MarshalPosture/UnmarshalPosture` 走 JSON；
//	  取值认不出一律**报错**（绝不当成 normal 载入）。
//
// ── 本批边界（与 T5.1/T5.2/T5.3 同一纪律）──
//
//	· 叶子包：只 import 标准库（encoding/json / fmt / strings）。
//	· **不接任何执行路径**：不碰 toolobs / chat / gateway / agent（一行不动）。
//	  这里是"姿态 × {工具表, 判定} ⇒ 新工具表 / 新判定"的**纯函数**；把它接到真实工具注册表与
//	  派发链上属于接线批（本批只保证模型与判定是可继承、可序列化的）。
//	· **工具表的表示由本包自己给**（`Tool{Name, Caps}`）：agent 的 `ToolDef` 在别的包里，
//	  而本包是零内部依赖的叶包 ⇒ 接线批做一层 `ToolDef → policy.Tool` 的映射
//	  （能力标记由注册表提供，名字摘除只是前向兼容的兜底，见 ToolCapAsksHuman 的注释）。
package policy

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ── 姿态（模式 + 是否沿扇出继承）──────────────────────────────────────────────

// PostureMode — 姿态档位。
type PostureMode string

const (
	// PostureNormal — 常态：惰性（不改判定、不动工具表）。
	PostureNormal PostureMode = "normal"
	// PostureDontAsk — 凡会弹窗的一律拒（ask ⇒ deny）。
	PostureDontAsk PostureMode = "dontask"
	// PostureYolo — 问人工具从工具表摘掉。
	PostureYolo PostureMode = "yolo"
)

// Valid — 是否三档之一。
func (m PostureMode) Valid() bool {
	switch m {
	case PostureNormal, PostureDontAsk, PostureYolo:
		return true
	}
	return false
}

// ParsePostureMode — 解析姿态文本（空串 = 常态）；认不出一律**报错**（不猜姿态）。
// 注意与 `Posture.effective()` 的分工：**这里报错**（配置解析要能拒绝启动），
// `effective()` 是"运行时已经拿到一个非法值"的兜底（按最严走，绝不 panic）。
func ParsePostureMode(raw string) (PostureMode, error) {
	m := PostureMode(strings.ToLower(strings.TrimSpace(raw)))
	if m == "" {
		return PostureNormal, nil
	}
	if !m.Valid() {
		return "", fmt.Errorf("未知姿态「%s」（只认 normal|dontask|yolo）", strings.TrimSpace(raw))
	}
	return m, nil
}

// Posture — 无人值守姿态。三个字段都是可序列化的（重启继承）、可比较的（判等即"姿态没变"）。
type Posture struct {
	// Mode — 姿态档位。零值（""）**不是** normal，而是"没设" ⇒ 按最严处理（见文件头 ③）。
	Mode PostureMode `json:"mode"`
	// InheritedByFanout — 是否沿子代理/扇出继承（F4）。默认 false = 子代理回到常态；
	// 要让无人值守姿态盖住整棵扇出树，必须显式置 true（声明即审计线索）。
	InheritedByFanout bool `json:"inherited_by_fanout"`
}

// effective — 运行时兜底：非法/未设档位 ⇒ 按**最严**走（dontAsk 语义）。
// 与 `ParsePostureMode` 的分工见其注释：配置期拒绝，运行期兜严。
func (p Posture) effective() PostureMode {
	if !p.Mode.Valid() {
		return PostureDontAsk
	}
	return p.Mode
}

// Inherit — 子代理/扇出得到的姿态。未声明继承 ⇒ 子代理拿常态（默认不放大）。
func (p Posture) Inherit() Posture {
	if !p.InheritedByFanout {
		return Posture{Mode: PostureNormal}
	}
	if !p.Mode.Valid() {
		// 父代理姿态本身非法：子代理拿"最严"而不是"常态"（非法值不会因为扇出而变宽）。
		return Posture{Mode: PostureDontAsk, InheritedByFanout: true}
	}
	return p
}

// MarshalPosture / UnmarshalPosture — 重启继承（写/读）。
// 读的一侧：JSON 坏 / 档位认不出 ⇒ **报错**（绝不静默载成 normal —— 那会把无人值守悄悄降级成常态）。
func MarshalPosture(p Posture) ([]byte, error) {
	return json.Marshal(p)
}

// UnmarshalPosture — 见 MarshalPosture。
func UnmarshalPosture(data []byte) (Posture, error) {
	var p Posture
	if err := json.Unmarshal(data, &p); err != nil {
		return Posture{}, fmt.Errorf("姿态载入失败（JSON 坏）⇒ 拒绝启用（不回退成「常态」）：%w", err)
	}
	if p.Mode != "" && !p.Mode.Valid() {
		return Posture{}, fmt.Errorf("姿态档位「%s」不认识 ⇒ 拒绝启用（不猜档位、不降级成常态）", string(p.Mode))
	}
	return p, nil
}

// ── 工具表（本包自己的最小表示）─────────────────────────────────────────────

// ToolCap — 工具的能力标记（接线批从工具注册表映射过来）。
type ToolCap string

const (
	// ToolCapAsksHuman — **会问人**：这个工具的用途就是"向用户提问 / 请求批准"。
	// 这是 yolo 摘除的**主判据**（能力标记是注册表的事实，名字只是兜底）：
	// 一切"机器可判"的摘除都必须尽量落在标记上，因为标记不随命名风格变化。
	ToolCapAsksHuman ToolCap = "asks_human"
)

// Tool — 工具表里的一项（**最小表示**：名字 + 能力标记）。
// 只带判定姿态需要的字段，故意不带 schema/处理器 —— 本包不执行任何东西。
type Tool struct {
	Name string    `json:"name"`
	Caps []ToolCap `json:"caps,omitempty"`
}

// Has — 是否带某个能力标记。
func (t Tool) Has(c ToolCap) bool {
	for _, x := range t.Caps {
		if x == c {
			return true
		}
	}
	return false
}

// AsksHuman — 是否是问人类工具：带能力标记，**或**名字命中保留名。
func (t Tool) AsksHuman() bool {
	return t.Has(ToolCapAsksHuman) || IsReservedAskToolName(t.Name)
}

// postureReservedAskToolNames — 问人工具的**保留名**（设计稿 F9 / F11 的 respond / reject 两轨）。
//
// 诚实说明（与 builtin_rules.go 的 pending.any.interactive_tools 同址记账）：**仓内到本批为止
// 还没有落地任何问人工具**（respond / reject 仍是设计稿），所以这份名单是**前向兼容**的名字兜底，
// 不是"仓内已有这些工具"的断言。它存在的唯一理由：接线时若注册表还没来得及打能力标记，
// 名字这一道也能拦住——而不是因为名字比标记更可靠（**标记才是主判据**）。
//
// 纪律：① 名字**逐字**命中才摘（不许前缀/模糊 —— 那会把 `respond_log` 这类正常工具误摘）；
// ② 往里加名字必须同时给出处（设计稿章节 or 注册表真名）；③ 删名字等于放开一道保护，须评审。
var postureReservedAskToolNames = []string{
	"respond",  // 设计稿 F9：向用户提交问题并等待答复
	"reject",   // 设计稿 F11：明确拒绝并"不要重试"（同属问人轨）
	"ask_user", // 别名（MCP / 插件生态里常见的同名工具）
}

// AsksHumanReservedNames — 保留名回显（只读副本；用例与 UI 都要能枚举"哪些名字会被摘掉"）。
func AsksHumanReservedNames() []string {
	out := make([]string, len(postureReservedAskToolNames))
	copy(out, postureReservedAskToolNames)
	return out
}

// IsReservedAskToolName — 名字是否命中保留名（归一去空白 + 小写；**不做模糊/前缀匹配**：
// 模糊匹配会把 `respond_log` 这类正常工具一起摘掉）。
func IsReservedAskToolName(name string) bool {
	n := NormalizeToolName(name)
	if n == "" {
		return false
	}
	for _, r := range postureReservedAskToolNames {
		if n == r {
			return true
		}
	}
	return false
}

// ── 姿态 ⇒ 判定 ─────────────────────────────────────────────────────────────

// ApplyPostureVerdict — 姿态对**判定结果**的作用（只做一件事：dontAsk 下 ask ⇒ deny）。
//
//	· dontAsk：ask ⇒ deny（原因码仍是 dontask_deny；MatchedRule 保留 —— 审计要能看到"是哪条规则本来要问人"）
//	· normal / yolo：**原样返回**（含 Reason / MatchedRule / Note 一字不改）
//
// 非法的 v.Decision（不是 allow/ask/deny）**不动**：那不是本层的责任，
// T5.1 的 fail-closed 会在判定阶段就把"判不了"落成 ask。
func ApplyPostureVerdict(v Verdict, p Posture) Verdict {
	if p.effective() != PostureDontAsk {
		return v
	}
	if v.Decision != EffectAsk {
		return v
	}
	note := strings.TrimSpace(v.Note)
	if note != "" {
		note += "｜"
	}
	note += "无人值守姿态（dontAsk）：凡会弹窗的一律拒 ⇒ ask 转 deny（不执行）"
	return Verdict{
		Decision:    EffectDeny,
		Reason:      ReasonDontAskDeny,
		MatchedRule: v.MatchedRule, // 保留命中规则：审计要能回答"本来是哪条要问人"
		Note:        note,
	}
}

// ── 姿态 ⇒ 工具表 ───────────────────────────────────────────────────────────

// AppliedToolset — 过完姿态的工具表 + "摘了什么"的记账。
type AppliedToolset struct {
	Tools   []Tool   `json:"tools"`   // 实际交给模型的工具表（yolo 下**不含**问人类工具）
	Removed []string `json:"removed"` // 摘掉的工具名（保序、可枚举；空 = 一个没摘）
	Note    string   `json:"note"`    // 人类可读说明（含"这是有意的"）
}

// ApplyPosture — 姿态对**工具表**的作用。
//
//	yolo   ⇒ 返回的工具表**不含问人类工具**（按能力标记 asks_human 或保留名摘除），其余工具**原序原样**保留
//	dontAsk/normal ⇒ 工具表一字不改（摘工具是 yolo 的语义；dontAsk 管的是判定）
//
// 三条写死的细节：
//
//	① **入参不被修改**（既不就地删、也不改元素）：返回的是新切片 ⇒ 调用方手里的原表仍然完整，
//	   "摘掉"只发生在本次扇出的视图上（否则一次 yolo 会永久污染全局注册表）。
//	② **只摘"问人"，不摘别的**：不认识的工具、没有标记的工具一律留着（摘多了 = 悄悄削掉能力，
//	   而"模型为什么变笨了"这种问题极难查）。
//	③ `Removed` 与 `Note` 必须给出——用户看到工具变少时，要能一眼看到"是姿态摘的，不是工具没了"。
func ApplyPosture(toolset []Tool, p Posture) AppliedToolset {
	mode := p.effective()
	if mode != PostureYolo {
		out := make([]Tool, len(toolset))
		copy(out, toolset)
		note := "姿态=" + string(mode) + "：工具表原样保留（不摘任何工具）"
		if string(p.Mode) != string(mode) {
			note = "姿态取值非法/未设（" + fmt.Sprintf("%q", string(p.Mode)) + "）⇒ 判定按最严（dontAsk）走；工具表原样保留（摘工具只在 yolo 下发生）"
		}
		return AppliedToolset{Tools: out, Removed: nil, Note: note}
	}

	out := make([]Tool, 0, len(toolset))
	var removed []string
	for _, t := range toolset {
		if t.AsksHuman() {
			removed = append(removed, t.Name)
			continue
		}
		out = append(out, t)
	}
	note := fmt.Sprintf("姿态=yolo：**有意**从工具表摘掉 %d 个问人类工具（省掉一轮「问了也没人答」的往返，不是误删）；"+
		"判定结果不改写（残留 ask 的处理属 T5.8 批准超时=拒绝，本批不猜）", len(removed))
	if len(removed) == 0 {
		note = "姿态=yolo：工具表里本来就没有问人类工具（摘掉 0 个）"
	}
	return AppliedToolset{Tools: out, Removed: removed, Note: note}
}
