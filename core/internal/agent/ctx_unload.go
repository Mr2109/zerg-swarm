package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

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

// unloadToolResult —— 大工具输出 ⇒ 原文落盘 ＋ 返回指针；小输出原样返回。
//
// 卸载失败（盘满 / 无权限）时**退回原样**：宁可让上层按老口径截断，
// 也不能因为卸载失败把工具结果整个丢掉（丢证据比丢 token 严重）。
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
	return fmt.Sprintf("〔已卸载 %d 字节 · 原文 %s〕", len(text), full)
}
