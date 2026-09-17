// toolobs.go — 工具调用「允许/拒绝」判定的观测出口（任务表 v2.5.10 / T1.2）
//
// 要治的观测盲区（一句话）：**「被系统拒绝」与「模型根本没想调」在观测面长得一模一样**
// —— 两者都只是「没有那条工具事件」⇒ 无法归因。本包把「判定」变成一等事件。
//
// 为什么是一个**叶子包**（零内部依赖）：
//
//	拒绝判定发生在两处 —— internal/agent（工具执行，exec.go/bash_v101.go）与 internal/loopcore（轮次循环，
//	工具被隐藏时不执行）；而落盘在 internal/chat（obs.go）。依赖方向是 chat → agent → loopcore 单向，
//	两边都**不能反向 import chat**（会成环）。故这里只放"事件形状 + 投递口"，由 chat 侧注册 sink
//	决定落到哪（不注册 = 事件丢弃，绝不 panic、绝不阻塞、绝不改变任何判定）。
//
// 三条纪律（与 chat/obs.go 的三条铁律同源）：
//  1. **判定不变**：本包只上报事实；拒绝仍是拒绝，放行仍是放行——不放宽任何白名单、不改工具行为。
//  2. **best-effort**：sink 里出任何岔子都只记日志，绝不向工具执行路径抛错/panic。
//  3. **不落原文**：只带参数**摘要**（sha256 前 16 位），不带参数原文。
package toolobs

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"sync"
)

// 判定结果（闭集两态——第三态不存在：没调用就没有事件，见 ObsToolDecision 的语义注释）
const (
	DecisionAllow = "allow"
	DecisionDeny  = "deny"
)

// 拒绝原因码（低基数、可枚举——**新增拒绝分支必须在此登记**）。
//
// 口径：一条 deny 事件必带恰好一个原因码；原因码是**判定类别**，不是错误原文
// （原文走既有 Trace/Error 与 OBS-3 的 result，不进本事件——防高基数爆表 + 防隐私外泄）。
const (
	ReasonGateBlock       = "gate_block"             // 安全门（ToolGater）拦截：权限
	ReasonGateError       = "gate_error"             // 安全门本身报错（未能判定 ⇒ 不执行）
	ReasonGateApproval    = "gate_approval_required" // 安全门要求审批（无审批通道 ⇒ 本轮不执行）
	ReasonPathOutside     = "path_outside_workspace" // 路径不在工作区/白名单内（沙箱边界）
	ReasonCwdOutside      = "cwd_outside_workspace"  // bash cwd 出域（bash 沙盒=工作区）
	ReasonDangerousCmd    = "dangerous_command"      // 危险命令黑名单命中（rm -rf /、mkfs…）
	ReasonObfuscatedCmd   = "obfuscated_command"     // 解码/eval 动态执行链（base64 -d | sh、eval $()）
	ReasonRmHome          = "rm_home_guard"          // rm 目标命中家目录/根目录保护
	ReasonRmOutOfScope    = "rm_out_of_scope"        // rm 目标不在允许删除域（工作区/tmp/白名单）
	ReasonBashPokaYoke    = "bash_pokayoke"          // bash 结构性防呆（空命令/裸交互命令/引号未闭合/末尾 &）
	ReasonParamMissing    = "param_missing"          // 必填参数缺失/为空（参数不合法：形态）
	ReasonParamContract   = "param_contract"         // 参数不合 schema 约定（enum/x-zerg-format：值域）
	ReasonUnknownTool     = "unknown_tool"           // 工具名不在注册表
	ReasonAmbiguousTool   = "ambiguous_tool_name"    // 工具名歧义（只差大小写 ⇒ 拒，不猜）
	ReasonPlaceholderTool = "placeholder_tool_name"  // 把教学示例里的占位词当工具名
	ReasonPathIsDir       = "path_is_directory"      // write 目标指向目录
	ReasonContentTooLarge = "content_too_large"      // 写入内容超上限（防 OOM/巨型误写）
	ReasonNonTextTarget   = "non_text_target"        // write/edit 目标是非文本类（防毁结构）
	ReasonToolHidden      = "tool_hidden"            // 工具被系统隐藏（连续失败 ⇒ 本轮不执行）
	ReasonUnsupported     = "unsupported_target"     // 本工具不执行该目标（read 图像/音视频/二进制；能力缺失如 spawn_agent 无父上下文）
)

// Decision — 一次工具调用的判定事件（allow|deny + 拒绝原因 + 参数摘要 + 会话）。
//
// Session 为空 ⇒ 观测面无 trace/span 骨架（chat 侧口径：**有会话才补骨架**，不编造 id）。
type Decision struct {
	Session    string // 会话 ID（空=未知）
	Tool       string // 工具名（**注册表真名**）
	Decision   string // allow | deny
	Reason     string // 拒绝原因码（allow 时为空）
	ArgsDigest string // 参数摘要（sha256 前 16 位；不落原文）
}

var (
	obsMu sync.RWMutex
	sink  func(Decision)
)

// SetSink — 注册判定事件的落点（chat 侧在 init 里接一次）。传 nil = 不落（丢弃）。
// 并发安全；重复注册以后者为准（进程内只有一个观测面）。
func SetSink(f func(Decision)) {
	obsMu.Lock()
	sink = f
	obsMu.Unlock()
}

// Emit — 上报一次判定。**best-effort**：没接 sink、sink 报错、sink panic —— 都不影响工具执行路径。
func Emit(d Decision) {
	obsMu.RLock()
	f := sink
	obsMu.RUnlock()
	if f == nil {
		return // 没人接 = 丢弃（不是错误；判定照旧生效）
	}
	// sink 是外部代码（且会写盘）⇒ 必须挡住它把异常带回工具执行路径
	func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("⚠️ toolobs: sink panic（工具执行不受影响）: %v", r)
			}
		}()
		f(d)
	}()
}

// Digest — 参数摘要：规范化 JSON（encoding/json 对 map 键排序 ⇒ 同参同摘）后 sha256，取前 16 位十六进制。
//
// 为什么不能落参数原文：参数里常含路径/命令/正文（隐私 + 高基数）。
// 为什么空参也给摘要（而非缺席）：args==nil 与 args=={} 都是**确定的空参数集**，
// 给出确定摘要才能「同一次调用」可关联；给空串会把"空参数"读成"没测到"。
func Digest(args map[string]any) string {
	if args == nil {
		args = map[string]any{}
	}
	b, err := json.Marshal(args)
	if err != nil {
		return "" // 编不动 ⇒ 缺席（不编造一个看着像摘要的值）
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])[:16]
}
