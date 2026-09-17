package redact

import (
	"encoding/json"
	"sync/atomic"
	"unicode/utf8"
)

// ── fail-closed（D1 / 不变量②）───────────────────────────────────────────────
//
// 只写「写入时脱敏」而不定失败语义，best-effort 会退化成**静默明文**：脱敏一 panic，
// 调用方一个 recover 就把原始事件写下去了。故本文件的语义写死为：
//
//	脱敏 panic / 超预算 ⇒ 只产出 {"event":"redaction_failed"}，**绝不回退写原文**。
//
// 另有单调计数器 RedactionFailures（D1：单调计数器 + 阈值告警的本地形态）。

// EventRedactionFailed 是 fail-closed 的唯一落盘形态。
const EventRedactionFailed = "redaction_failed"

// DefaultMaxValueBytes / DefaultMaxEventBytes 是默认预算（八步清单第 7 步）。
const (
	DefaultMaxValueBytes = 4096    // 单值上限：2–4KB（对齐 OTTL truncate_all 语义）
	DefaultMaxEventBytes = 1 << 20 // 单事件上限：1MiB（超限 fail-closed，绝不全量落盘）
)

var (
	maxValueBytes atomic.Int64
	maxEventBytes atomic.Int64
	failures      atomic.Uint64
	// redactEventFn 是管线注入点：唯一目的是让 fail-closed 用例能注入一个 panic
	// （T2.4 的「注入 panic ⇒ 只写 marker」）。生产路径不要改写它。
	redactEventFn = RedactEvent
)

func init() {
	maxValueBytes.Store(DefaultMaxValueBytes)
	maxEventBytes.Store(DefaultMaxEventBytes)
}

// Limits 返回当前预算（值长上限, 事件上限）。
func Limits() (valueBytes, eventBytes int) {
	return int(maxValueBytes.Load()), int(maxEventBytes.Load())
}

// SetLimits 覆盖预算；参数 ≤0 表示保持默认。
func SetLimits(valueBytes, eventBytes int) {
	if valueBytes > 0 {
		maxValueBytes.Store(int64(valueBytes))
	}
	if eventBytes > 0 {
		maxEventBytes.Store(int64(eventBytes))
	}
}

// RedactionFailures 是单调计数器：fail-closed 触发过几次（panic 与超预算都算）。
// 只增不减 —— 任何"清零"都会让告警阈值失去意义（测试也不得清零，比较前后差值即可）。
func RedactionFailures() uint64 { return failures.Load() }

// RedactionFailedEvent 返回 fail-closed 的唯一事件形态。
func RedactionFailedEvent() map[string]any {
	return map[string]any{"event": EventRedactionFailed}
}

// RedactEventFailClosed 与 RedactEvent 的差别只有失败语义：
// panic 或结果超预算 ⇒ 返回 {"event":"redaction_failed"}。**绝不返回原文**。
func RedactEventFailClosed(ev map[string]any) map[string]any {
	out, ok := safeRedact(ev)
	if !ok {
		return RedactionFailedEvent()
	}
	return out
}

// safeRedact 是全部失败路径的收口：panic 必须在这里被吃掉（写路径上 panic 会带走整条调试流）。
func safeRedact(ev map[string]any) (out map[string]any, ok bool) {
	defer func() {
		if r := recover(); r != nil {
			failures.Add(1)
			out, ok = nil, false
		}
	}()
	out = redactEventFn(ev)
	b, err := json.Marshal(out)
	if err != nil || !utf8.Valid(b) {
		failures.Add(1)
		return nil, false
	}
	if int64(len(b)) > maxEventBytes.Load() {
		// 超预算 ⇒ fail-closed（不是截断：截断一条超预算事件仍可能留下没被脱敏的尾巴）
		failures.Add(1)
		return nil, false
	}
	return out, true
}

// MarshalRedacted 是**唯一落盘助手**：脱敏 → 预算复核 → 编码，任何一步失败都只产出 marker 行。
// 返回的字节即该写入 JSONL 的整行内容（调用方只负责换行/轮转，T2.1 的 emit 接入）。
func MarshalRedacted(ev map[string]any) []byte {
	red := RedactEventFailClosed(ev)
	b, err := json.Marshal(red)
	if err != nil || !utf8.Valid(b) || int64(len(b)) > maxEventBytes.Load() {
		failures.Add(1)
		b, _ = json.Marshal(RedactionFailedEvent()) // marker 自身不可能失败
	}
	return b
}
