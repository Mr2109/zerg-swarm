package redact

import "sync/atomic"

// ── 类型/分类门控 + 分级（T2.7 / D6）────────────────────────────────────────
//
// 设计附件 §三 措施③「先判类型，只有 string 进管线」，字段三分表 §五 把 A 类定义成
// 「**原样保留**（调试必需且不敏感）」。T2.7 把「原样」落成**零扫描**：A 类标量一个字节都不进值管线。
//
// 为什么值得单独立一条门：A 类字段（model · model_id · quant · temperature/top_p/seed/max_tokens ·
// tool_name · call_fingerprint · *_sha256 · seq/ts/trace_id · duration_ms · tokens_in/out ·
// stop_reason/exit_code · level/component/ok …）是**低基数枚举 + 数值**，一次会话里出现成千上万次。
// 把它们丢进 12 条正则的管线，换来的只有误报风险（D12：误报 ⇒ 团队关掉脱敏），一分钱好处也没有。
//
// 分级（本包实际落地的三层，对应省钱顺序里的「丢弃 > 分级 > 截断」）：
//
//	① 丢弃 —— ClassDrop / ClassMask：值一个字节都不看，直接写占位符（连截断都不需要）；
//	② 分级 —— ClassKeep 标量零扫描（本文件）· ClassScan 自由文本才进管线（walkValue 的
//	   string 分支）· 非 string（数字/布尔/nil/json.Number）原样通过；
//	③ 截断 —— 值长上限对所有**落盘**字符串生效（含 A 类：长度检查是 O(1)，不是扫描）。
//
// 容器例外（这条不能省）：A 类键上挂的 map/slice **仍然递归** —— 否则
// `"meta": {"model": {"path": "/Users/…"}}` 就是一条绕过脱敏的后门（探针语料里的 nested-map+array
// 就是这个形状：只有顶层字段才脱敏的基线实测 15 条漏 10 条，D15）。
//
// 计数器：给用例做**可判定**断言用（不是耗时断言 —— 耗时在 CI 上会抖动，门禁会因此变成噪音）。
// 它们是**诊断计数器**，不承担告警语义；与 failclosed.go 的 RedactionFailures（只增不减）不同，
// ResetPerfCounters 只给用例做前后差值比较。
var (
	pipelineEntries  atomic.Uint64 // 进值管线的字符串个数（RedactValue 的进入次数）
	keepScalarPasses atomic.Uint64 // A 类标量原样通过（零扫描）的个数
	valueTruncations atomic.Uint64 // 被值长上限截断的值个数
)

// PipelineEntries 返回值管线进入次数（单调，供用例做前后差值）。
func PipelineEntries() uint64 { return pipelineEntries.Load() }

// KeepScalarPasses 返回 A 类标量零扫描通过次数（单调，供用例做前后差值）。
func KeepScalarPasses() uint64 { return keepScalarPasses.Load() }

// ValueTruncations 返回被值长上限截断的值个数（单调，供用例做前后差值）。
func ValueTruncations() uint64 { return valueTruncations.Load() }

// ResetPerfCounters 归零三个诊断计数器。**只给用例用**：它没有告警语义，
// 而 fail-closed 的 RedactionFailures 按纪律永不清零。
func ResetPerfCounters() {
	pipelineEntries.Store(0)
	keepScalarPasses.Store(0)
	valueTruncations.Store(0)
}

// keepValue 是 A 类字段的**零扫描**落地：标量逐字通过（连类型转换都不做，只做 O(1) 的长度上限），
// 容器照旧递归（见文件头「容器例外」）。
func keepValue(v any, depth int) any {
	switch t := v.(type) {
	case map[string]any, []any:
		return walkValue(v, depth) // 容器递归：A 类不是绕过脱敏的后门
	case string:
		keepScalarPasses.Add(1)
		return truncateValue(t) // 长度检查 O(1)：不扫描，但落盘体积仍受值长上限约束
	default:
		keepScalarPasses.Add(1)
		return v
	}
}
