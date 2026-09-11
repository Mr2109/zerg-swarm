// chat_store_migrate.go — 甲批 T1/T4（2026-09-10 Mr2109拍板）
// T1 对话库迁出 /tmp：默认 ~/.zerg-chat/chat.db（旧 /tmp 库一次性迁移，失败保留旧库继续用）
// T4 中文检索修复：messages_fts 由 unicode61 → trigram（含全量重建 + 行数校验）
package chat

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// legacyChatDBPath — 旧对话库路径（/tmp：macOS 重启即清 + 3 天未访问被 tmp_cleaner 删除）
const legacyChatDBPath = "/tmp/zerg-chat/chat.db"

// ResolveChatDBPath — 解析对话库路径（ZERG_CHAT_DB_PATH 覆盖 → 默认 ~/.zerg-chat/chat.db）
// 启动时顺带执行一次性迁移（幂等：目标存在即跳过）。
func ResolveChatDBPath() string {
	// 显式指定路径（ZERG_CHAT_DB_PATH，如测试/多实例）→ 不做迁移：
	// 否则会把默认库"搬"进任意路径（实测踩坑：测试进程把真实库搬进 t.TempDir）
	if os.Getenv("ZERG_CHAT_DB_PATH") != "" {
		return os.Getenv("ZERG_CHAT_DB_PATH")
	}
	newPath := chatDBPath()
	if _, err := os.Stat(newPath); err == nil {
		return newPath // 目标已存在——直接用（幂等）
	}
	if _, err := os.Stat(legacyChatDBPath); err != nil {
		return newPath // 无旧库——新建
	}
	if err := migrateChatDB(legacyChatDBPath, newPath); err != nil {
		log.Printf("⚠️ chat DB migration failed (continuing with legacy path %s): %v", legacyChatDBPath, err)
		return legacyChatDBPath
	}
	log.Printf("✅ chat DB migrated: %s → %s (legacy DB kept, not deleted)", legacyChatDBPath, newPath)
	return newPath
}

// chatDBPath — 目标路径（默认 ~/.zerg-chat/chat.db）
func chatDBPath() string {
	if p := os.Getenv("ZERG_CHAT_DB_PATH"); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return legacyChatDBPath
	}
	return filepath.Join(home, ".zerg-chat", "chat.db")
}

// migrateChatDB — 一次性迁移：VACUUM INTO 一致性快照（含 WAL 中已提交事务）→ integrity_check → 行数比对 → 就位
// 铁律：只用 SQLite API（VACUUM INTO）取快照，绝不裸拷 db/-wal（WAL 是持久状态的一部分，裸拷会丢已提交事务）。
func migrateChatDB(oldPath, newPath string) error {
	if err := os.MkdirAll(filepath.Dir(newPath), 0o755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	tmp := newPath + ".migrating"
	_ = os.Remove(tmp)

	old, err := sql.Open("sqlite", oldPath)
	if err != nil {
		return fmt.Errorf("failed to open legacy DB: %w", err)
	}
	defer old.Close()
	// VACUUM INTO：生成一致性快照（官方推荐的备份方式，含 WAL 已提交数据）
	if _, err := old.Exec("VACUUM INTO ?", tmp); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("VACUUM INTO failed: %w", err)
	}

	// 校验：integrity_check + sessions/messages 行数一致
	chk, err := sql.Open("sqlite", tmp)
	if err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("failed to open snapshot: %w", err)
	}
	var ic string
	if err := chk.QueryRow("PRAGMA integrity_check").Scan(&ic); err != nil || ic != "ok" {
		chk.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("snapshot integrity check failed: %v (%s)", err, ic)
	}
	for _, tbl := range []string{"sessions", "messages"} {
		var a, b int
		if err := old.QueryRow("SELECT count(*) FROM " + tbl).Scan(&a); err != nil {
			chk.Close()
			_ = os.Remove(tmp)
			return fmt.Errorf("failed to count legacy DB table %s: %w", tbl, err)
		}
		if err := chk.QueryRow("SELECT count(*) FROM " + tbl).Scan(&b); err != nil {
			chk.Close()
			_ = os.Remove(tmp)
			return fmt.Errorf("failed to count snapshot table %s: %w", tbl, err)
		}
		if a != b {
			chk.Close()
			_ = os.Remove(tmp)
			return fmt.Errorf("row count mismatch (%s: legacy %d / new %d) — aborting migration", tbl, a, b)
		}
	}
	chk.Close()
	if err := os.Rename(tmp, newPath); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("failed to move into place: %w", err)
	}
	// 迁移标记（便于排查）
	_ = os.WriteFile(filepath.Join(filepath.Dir(newPath), ".migrated-from-tmp"), []byte(oldPath+"\n"), 0o644)
	return nil
}

// migrateFTSv4 — messages_fts 由 unicode61 重建为 trigram（T4）
// 只在检测到旧分词器时执行；重建后校验索引行数 = messages 行数，不一致则回滚事务。
func (s *ChatStore) migrateFTSv4() error {
	var ddl string
	err := s.db.QueryRow("SELECT COALESCE(sql,'') FROM sqlite_master WHERE name='messages_fts'").Scan(&ddl)
	if err == sql.ErrNoRows {
		return nil // 无旧表——schemaSQL 已按 trigram 建好
	}
	if err != nil {
		return err
	}
	if !strings.Contains(ddl, "unicode61") {
		return nil // 已是 trigram——幂等跳过
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	steps := []string{
		"DROP TABLE IF EXISTS messages_fts",
		"CREATE VIRTUAL TABLE messages_fts USING fts5(content, tokenize='trigram')",
		"INSERT INTO messages_fts(rowid, content) SELECT id, COALESCE(content,'') FROM messages",
	}
	for _, st := range steps {
		if _, err := tx.Exec(st); err != nil {
			return fmt.Errorf("failed to rebuild FTS (%s): %w", st, err)
		}
	}
	var nMsg, nFts int
	if err := tx.QueryRow("SELECT count(*) FROM messages").Scan(&nMsg); err != nil {
		return err
	}
	if err := tx.QueryRow("SELECT count(*) FROM messages_fts").Scan(&nFts); err != nil {
		return err
	}
	if nMsg != nFts {
		return fmt.Errorf("FTS rebuild row count mismatch (messages %d / fts %d) — rolled back", nMsg, nFts)
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	log.Printf("✅ CJK search fix: messages_fts rebuilt as trigram (%d rows)", nFts)
	return nil
}
