// chat_prompt_store.go — 丙批 §4.1：系统提示冻结的落库读写（2026-09-10）
// 表结构：sessions.system_prompt / prompt_hash / prompt_model（schema v6 迁移见 chat_store.go）

package chat

import (
	"database/sql"
	"fmt"
)

// frozenPrompt — 读冻结的系统提示（"", "", "")=未冻结
func (s *ChatStore) frozenPrompt(sessionID string) (prompt, hash, model string) {
	row := s.db.QueryRow(
		`SELECT COALESCE(system_prompt,''), COALESCE(prompt_hash,''), COALESCE(prompt_model,'')
		 FROM sessions WHERE id = ?`, sessionID)
	switch err := row.Scan(&prompt, &hash, &model); err {
	case nil:
		return prompt, hash, model
	case sql.ErrNoRows:
		return "", "", ""
	default:
		return "", "", ""
	}
}

// saveFrozenPrompt — 落库（含哈希——验收①与命中率基线共用口径）
func (s *ChatStore) saveFrozenPrompt(sessionID, prompt, model string) error {
	_, err := s.db.Exec(
		`UPDATE sessions SET system_prompt = ?, prompt_hash = ?, prompt_model = ? WHERE id = ?`,
		prompt, PromptHash(prompt), model, sessionID)
	if err != nil {
		return fmt.Errorf("chat: 冻结系统提示失败: %w", err)
	}
	return nil
}

// clearFrozenPrompt — 清空冻结（压缩后/模型切换后重建）
func (s *ChatStore) clearFrozenPrompt(sessionID string) error {
	_, err := s.db.Exec(
		`UPDATE sessions SET system_prompt = NULL, prompt_hash = NULL, prompt_model = NULL WHERE id = ?`,
		sessionID)
	return err
}
