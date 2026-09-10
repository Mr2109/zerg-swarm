// chat_prompt.go — 丙批 §4.1：系统提示三档 + 会话内字节稳定（2026-09-10 Mr2109拍板）
//
// 背景：此前 sysPrompt 每轮重新拼装（`chatSystemPrompt + 身份 + 工具清单 + 记忆块`），
// 而渐进模式的常驻工具列表会**会话中途增长** → 系统提示字节变化 → 前缀缓存从变化点起全部失效。
//
// 三档（顺序固定，便于"稳定前缀尽量长"）：
//   stable    —— 基础指令常量 + 身份（除模型切换外永不变）
//   context   —— 会话元信息（模型名等；模型切换必须重建——缓存键含模型）
//   volatile  —— 记忆块 + 工具清单（会话内冻结：本会话首次构建后原样复用）
//
// 冻结：完整 sysPrompt 落 `sessions.system_prompt`(+`prompt_hash`+`prompt_model`)，
// 同会话后续轮**原样复用**；仅以下情况重建：①压缩后（ClearSessionPrompt）②模型切换（哈希中带模型，自动失配重建）。
// 渐进增长的代价：新发现的工具**不进本会话的提示清单**，但仍可调用（tool_search 会回传用法）——增长只对新会话生效。

package chat

import (
	"crypto/sha256"
	"encoding/hex"
	"github.com/Mr2109/zerg-swarm/core/internal/ffp"
	"log"
)

// PromptHash — 提示哈希（验收①"10 轮不变"与命中率版本基线的共同口径）
func PromptHash(p string) string {
	sum := sha256.Sum256([]byte(p))
	return hex.EncodeToString(sum[:8])
}

// BuildTieredSystemPrompt — 组装三档（纯函数——便于单测）
func BuildTieredSystemPrompt(base, model, sessionID string, rt *ToolRuntime) string {
	// stable：基础指令常量（调用方传入）+ 身份
	// 多语言 D2：执行接口约定进系统提示（常量 → 字节稳定，仅新会话生效）
	stable := base + "\n\n# 你的身份\n- 你当前运行在虫族本地模型集群——Mr2109的对话助手——不要调查或质疑自己的身份。" + "\n\n" + ffp.Conventions
	// context：会话元信息（模型切换 → 提示重建——缓存键含模型）
	ctx := "\n\n# 会话环境\n- 模型: " + model
	// volatile：记忆块 + 工具清单（会话内冻结）
	vol := ""
	if mem := MemoryBlock(sessionID); mem != "" {
		vol += "\n\n" + mem
	}
	vol += BuildHermesToolPrompt(rt)
	return stable + ctx + vol
}

// SessionSystemPrompt — 取会话系统提示：首次构建即落库，同会话后续轮原样复用（字节稳定）
// model 变化 → 自动重建（并把新提示写回）。
func (s *ChatStore) SessionSystemPrompt(sessionID, base, model string, rt *ToolRuntime) string {
	if sessionID != "" {
		if p, _, m := s.frozenPrompt(sessionID); p != "" && m == model {
			return p
		}
	}
	p := BuildTieredSystemPrompt(base, model, sessionID, rt)
	if sessionID != "" {
		if err := s.saveFrozenPrompt(sessionID, p, model); err != nil {
			log.Printf("⚠️ 系统提示冻结落库失败（会话 %s）: %v", sessionID, err)
		}
	}
	return p
}

// ClearSessionPrompt — 重建触发点：压缩后（以及未来的 /reset）调用；下一轮会重新构建并重新冻结
func (s *ChatStore) ClearSessionPrompt(sessionID string) {
	if sessionID == "" {
		return
	}
	if err := s.clearFrozenPrompt(sessionID); err != nil {
		log.Printf("⚠️ 清理系统提示冻结失败（会话 %s）: %v", sessionID, err)
	}
}

// FrozenPromptInfo — 当前冻结状态（验收/诊断用：hash 与是否已冻结）
func (s *ChatStore) FrozenPromptInfo(sessionID string) (hash, model string, frozen bool) {
	p, h, m := s.frozenPrompt(sessionID)
	return h, m, p != ""
}
