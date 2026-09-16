package chat

// 丙（看门狗判卡）—— 纯函数，好测：区分"慢"与"卡"。
//
// 背景（2026-09-17 实测 + 业界调研）：
//
//	· 本地推理不该写死时限；正确形态是"看门狗"：**首 token 闸**（等到第一个字）+ **中途空档**（出字过程中停住）。
//	· 我们的引擎是**突发式送达**（实测 total 4089ms vs first_byte 4081ms ⇒ 273 块只用 8ms），
//	  所以"中途空档"(stall_ms) 常为 0 ⇒ 判"卡"必须同时看**首字节时长**，不能只看 stall。
//	· 判据只用于**解释与留痕**（为什么这轮被掐），不改变任何安全语义。
type StallVerdict string

const (
	VerdictOK      StallVerdict = "正常" // 首字节在闸内，且出字过程中没有卡住
	VerdictSlow    StallVerdict = "慢"  // 一直在出字，只是慢（不该杀）
	VerdictStalled StallVerdict = "卡"  // 首字节超闸，或中途长时间没有新块
)

// StallVerdictOf —— firstByteMS 首字节耗时；stallMS 首字节之后的最大空档；gateSec 首 token 闸（秒）。
// stalledAfterMS 判"中途卡住"的阈值（毫秒）——调用方给（如 60000）。gateSec<=0 表示没有闸。
func StallVerdictOf(firstByteMS, stallMS int64, gateSec int, stalledAfterMS int64) StallVerdict {
	if gateSec > 0 && firstByteMS > int64(gateSec)*1000 {
		return VerdictStalled // 首字节就没等到 ⇒ 卡（这才是我们今晚的病象）
	}
	if stalledAfterMS > 0 && stallMS > stalledAfterMS {
		return VerdictStalled // 出字中途停住 ⇒ 卡
	}
	if stallMS > 0 {
		return VerdictSlow // 有过空档但没超阈 ⇒ 慢
	}
	return VerdictOK
}
