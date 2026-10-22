package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Mr2109/zerg-swarm/core/internal/compress"
	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
)

// ctxUnloadBytes —— 工具输出超过这个字节数就卸载（原文落盘、摘要只留指针）。
//
// 取 4 KiB 的理由：任务压缩的摘要行本身只截 150 字，4 KiB 以下的工具结果
// 压成摘要后信息损失有限；超过 4 KiB 的多是文件全文 / 命令大输出，
// 截断后证据就没了（重启、复跑、纠错都要不到原文）⇒ 改为**卸载**。
const ctxUnloadBytes = 4 * 1024

// ctxUnloadDir —— 卸载落点（内容寻址：同名内容只存一份，可逆、可重复）。
func ctxUnloadDir() string { return statepath.File("ctx-unload") }

// ctxCompressAuditPath —— 压缩审计落点（与 `zerg context stats` 的 events 同址同件）。
//
// 一件一事件一行的 JSONL（`compress-audit.jsonl`），字段就是 compress.Event 的十个字段；
// 读侧 = `zerg context stats` 的 events 列（cmd/zerg/family_context.go: ctxCountAuditEvents）。
func ctxCompressAuditPath() string { return statepath.File("compress-audit.jsonl") }

// writeCtxAudit —— 追加一行审计事件（**只追加，从不重写整件**）。
//
// 为什么用 O_APPEND 而不是「读全件 → 加一行 → 写回」：重写会在中途失败时把旧事件整批
// 弄丢（审计是证据，只可增不可减）；O_APPEND 下已落的行永不被触碰，一次 Write 一行。
// 写失败**不阻断**卸载本身（见 unloadToolResult：丢 token 可以，丢证据不行；同理丢一条
// 审计行可以，丢原文不行）—— 错误交给调用方决定要不要出声。
func writeCtxAudit(ev compress.Event) error {
	f, err := os.OpenFile(ctxCompressAuditPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	return compress.NewJSONLSink(f).Emit(ev)
}

// unloadToolResult —— 大工具输出 ⇒ 原文落盘 ＋ 返回指针；小输出原样返回。
//
// 卸载失败（盘满 / 无权限）时**退回原样**：宁可让上层按老口径截断，
// 也不能因为卸载失败把工具结果整个丢掉（丢证据比丢 token 严重）。
//
// ★ 审计接线点（本笔）：unloadToolResult 只在压缩路径（compressOneRound → summarizeRound
// 的 tool 分支）被调用，所以「真卸了一次」= 一次压缩真的动了工具输出。事件就写在这里：
//   - 真卸载 ⇒ 一行 Phase=completed / Mechanism=unload，Before/After 是卸载前后的 token 估；
//   - 小输出（没卸）⇒ 不写（没发生「卸载」这件事，写一行就是灌水）；
//   - 卸载失败 ⇒ 退回原样、也不写（没落盘就没有可回查的证据，写了反而骗 stats）。
func unloadToolResult(text string) string {
	if len(text) <= ctxUnloadBytes {
		return text
	}
	dir := ctxUnloadDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return text
	}
	sum := sha256.Sum256([]byte(text))
	name := hex.EncodeToString(sum[:16]) + ".txt"
	full := filepath.Join(dir, name)
	if st, err := os.Stat(full); err != nil || st.Size() != int64(len(text)) {
		if werr := os.WriteFile(full, []byte(text), 0o600); werr != nil {
			return text
		}
	}
	ptr := fmt.Sprintf("〔已卸载 %d 字节 · 原文 %s〕", len(text), full)
	// 审计：只加不减。写不进审计不影响返回值（原件已落盘，可回查才是主证据）。
	_ = writeCtxAudit(compress.Event{
		Phase:      "completed",
		Trigger:    compress.TriggerPressure, // 到线（4 KiB 单条闸）才卸
		Mechanism:  "unload",                 // 无损：原文落盘 + 指针，可逆
		Aggressive: "light",
		Before:     compress.EstimateTokens(text),
		After:      compress.EstimateTokens(ptr),
		Fallback:   false,
		Reason:     fmt.Sprintf("工具输出 %d 字节 > %d 字节 ⇒ 原文落盘（%s）", len(text), ctxUnloadBytes, full),
	})
	return ptr
}
