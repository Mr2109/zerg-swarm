package redact

import (
	"os"
	"os/user"
	"regexp"
	"strings"
	"sync/atomic"
	"unicode/utf8"
)

// ── 字段三分表（A/B/C）───────────────────────────────────────────────────────
//
// 设计：设计-内建调试版附-D-组脱敏实测补强-v1.2.md §五「字段三分表」，细化 v1.2 的 D4。
//
//	A 原样保留（ClassKeep）：调试必需且不敏感 —— 模型名/权重哈希/采样参数/工具名/指纹/
//	  序号与时间/token 计数/时长/结束原因。**模型名与采样参数永不脱敏**：它们是复现一次判定的
//	  核心，把它们当秘密挡掉正是 D12 描述的「误报 ⇒ 团队关掉脱敏」。
//	B 必须脱敏：键级遮蔽（ClassMask：值形态本身即敏感）+ 值级扫描（ClassScan：扫干净就保留）。
//	C 直接丢弃（ClassDrop）：原始 prompt/completion/tool_args/output 这类「高敏感 + 高体积」字段。
//
// **未分类字段一律 ClassScan**（D4：每次新增字段都不该是一次新泄漏）。代价是新字段会进值级
// 管线，误报靠 allowlist 迭代收敛，而不是靠「默认放行」。
//
// 零值即 ClassScan：枚举写错/忘记初始化时落到「扫描」而不是「原样保留」。
type KeyClass int

const (
	ClassScan KeyClass = iota // 未分类默认：走值级管线，扫干净才保留
	ClassKeep                 // A 原样保留
	ClassMask                 // B（键级）一律遮蔽，不看值
	ClassDrop                 // C 一律丢弃（写占位符，不删键）
)

func (c KeyClass) String() string {
	switch c {
	case ClassKeep:
		return "keep"
	case ClassMask:
		return "mask"
	case ClassDrop:
		return "drop"
	default:
		return "scan"
	}
}

// Classify 按键名给出分类。键名统一小写比较（"API_KEY"/"Api_Key" 与 "api_key" 同类）。
//
// 与 verify.go 的 needle 提取共用本函数：门禁的 needle 集必须与脱敏器**同源同策略**
// （D18：needle 集是脱敏器的超集 ⇒ 误报 ⇒ 门禁被关掉，那才是真正的安全失效）。
func Classify(key string) KeyClass {
	k := strings.ToLower(key)
	switch k {
	// ── A 原样保留：调试必需且不敏感 ──
	case "schema", "seq", "ts", "trace_id", "span_id", "parent_span_id",
		"event", "step", "phase", "model", "model_id", "model_hash", "weights_sha256",
		"engine", "quant", "ctx_len", "temperature", "top_p", "top_k", "min_p", "seed",
		"repeat_penalty", "max_tokens", "stop_reason", "n_ctx_used",
		"tool", "tool_name", "turn", "attempt", "sampler", "batch_size",
		"prompt_sha256", "input_sha256", "output_sha256", "call_fingerprint",
		"duration_ms", "tokens_in", "tokens_out", "cache_hit", "exit_code",
		"level", "component", "role", "kind", "status", "ok", "retry_of":
		return ClassKeep

	// ── B 键级遮蔽：值形态本身就是敏感（永不「扫」——扫等于给了它一次被保留的机会）──
	case "api_key", "apikey", "authorization", "auth", "cookie", "set_cookie",
		"token", "access_token", "refresh_token", "id_token", "bearer",
		"password", "passwd", "secret", "client_secret", "private_key",
		"session", "session_id", "csrf", "nkey", "huggingface_token",
		"hf_token", "ssh_key", "credential", "credentials", "env_secret":
		return ClassMask

	// ── C 直接丢弃：原始模型/提示/工具载荷，默认不落盘（要复核时改存指纹+长度+首尾片段）──
	case "prompt", "prompt_raw", "messages", "completion", "response",
		"response_raw", "output", "input", "tool_args", "tool_result",
		"arguments", "content", "text", "chunk", "delta":
		return ClassDrop
	}
	// 未分类 ⇒ 扫描（fail-closed 倾向）
	return ClassScan
}

// KeyClasses 返回三分表的只读快照（键 → 分类），供文档/工具与「表里有几条」类断言使用。
// 表里没有的键即 ClassScan。
func KeyClasses() map[string]KeyClass {
	out := make(map[string]KeyClass, 128)
	for _, k := range []string{
		"schema", "seq", "ts", "trace_id", "span_id", "parent_span_id",
		"event", "step", "phase", "model", "model_id", "model_hash", "weights_sha256",
		"engine", "quant", "ctx_len", "temperature", "top_p", "top_k", "min_p", "seed",
		"repeat_penalty", "max_tokens", "stop_reason", "n_ctx_used",
		"tool", "tool_name", "turn", "attempt", "sampler", "batch_size",
		"prompt_sha256", "input_sha256", "output_sha256", "call_fingerprint",
		"duration_ms", "tokens_in", "tokens_out", "cache_hit", "exit_code",
		"level", "component", "role", "kind", "status", "ok", "retry_of",
	} {
		out[k] = ClassKeep
	}
	for _, k := range []string{
		"api_key", "apikey", "authorization", "auth", "cookie", "set_cookie",
		"token", "access_token", "refresh_token", "id_token", "bearer",
		"password", "passwd", "secret", "client_secret", "private_key",
		"session", "session_id", "csrf", "nkey", "huggingface_token",
		"hf_token", "ssh_key", "credential", "credentials", "env_secret",
	} {
		out[k] = ClassMask
	}
	for _, k := range []string{
		"prompt", "prompt_raw", "messages", "completion", "response",
		"response_raw", "output", "input", "tool_args", "tool_result",
		"arguments", "content", "text", "chunk", "delta",
	} {
		out[k] = ClassDrop
	}
	return out
}

// ── 用户折叠（裸用户名 → ${USER}）────────────────────────────────────────────
//
// 为什么它在 Tier-0：「Mr2109」单独出现在一个字段里时没有任何分隔符 ⇒ 任何启发式预过滤都会
// 整段跳过（D13 实测：fuzz 报出裸用户名漏脱敏）。故 ① 折叠在预过滤之前判定；
// ② 判定用**必要条件**（子串存在性），不是「像不像路径」的启发式。
//
// 本包不写死任何用户名（生产必须由入口设置，见 SetUser）。

type userFoldState struct {
	name string
	re   *regexp.Regexp
}

// userFold 为 nil 表示未启用折叠（不写死用户名；生产入口应调 SetUser）。
var userFold atomic.Pointer[userFoldState]

func init() { SetUser(osUserName()) }

// osUserName 取当前用户名：os/user → USER/LOGNAME 环境变量。都取不到就留空（关闭折叠）。
func osUserName() string {
	if u, err := user.Current(); err == nil && u.Username != "" {
		return u.Username
	}
	for _, k := range []string{"USER", "LOGNAME"} {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}

// SetUser 设置要折叠成 ${USER} 的用户名；"" 关闭折叠。
// 生产调用点：唯一写入口初始化时传 os/user.Current().Username（或 osUserName）。
func SetUser(name string) {
	if name == "" {
		userFold.Store(nil)
		return
	}
	userFold.Store(&userFoldState{name: name, re: regexp.MustCompile(`(?i)` + regexp.QuoteMeta(name))})
}

// User 返回当前折叠的用户名（"" = 未启用）。脱敏器与回扫门禁读同一份状态。
func User() string {
	if st := userFold.Load(); st != nil {
		return st.name
	}
	return ""
}

// hasUser 是折叠正则的**必要条件**门。原型里那条 `len(s) < 8KB` 上界是省 CPU 的启发式，
// 会让长文本里的裸用户名整段跳过 —— 本包去掉该上界（必要条件判定本身够便宜，安全优先）。
func hasUser(s string) bool {
	st := userFold.Load()
	if st == nil {
		return false
	}
	return len(s) >= len(st.name) && strings.Contains(strings.ToLower(s), strings.ToLower(st.name))
}

// ── 值长上限（八步清单第 7 步：值长上限 2–4KB，对齐 OTTL truncate_all）────────
//
// 截断发生在**脱敏之后**（绝不先截断再脱敏：那会把秘密切成半截、同时躲开模式匹配）。
// 截断结果长度 ≤ 上限：先扣掉标记自身的长度 ⇒ 再截一次是恒等（幂等）。
func truncateValue(s string) string {
	limit := maxValueBytes.Load()
	if limit <= 0 || int64(len(s)) <= limit {
		return s
	}
	valueTruncations.Add(1) // 诊断计数：被值长上限截断的值个数（gate.go）
	keep := int(limit) - len(PlaceholderTruncated)
	if keep < 0 {
		keep = 0
	}
	for keep > 0 && !utf8.RuneStart(s[keep]) {
		keep--
	}
	return s[:keep] + PlaceholderTruncated
}
