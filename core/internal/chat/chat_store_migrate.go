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
// 语义 = **一次性迁移的读取来源**（ResolveChatDBPath 只在「新库不存在且旧库存在」时读它一次；
// 旧库永不写入、永不删除）⇒ 硬编码治理批 B-2 判定：合规，保留它（不改成 statepath 派生）。
const legacyChatDBPath = "/tmp/zerg-chat/chat.db"

// legacyChatFile — 旧库路径（包级变量 = 上面的字面量；迁移用例可切换隔离路径，避免测试碰真机 /tmp）。
// 口径与 internal/api/tasks_persist.go 的 legacyTasksFile、internal/agent/idle_persist.go 的
// legacyIdleStateFile 同源：const 是文档化的字面量，var 供用例切换。
var legacyChatFile = legacyChatDBPath

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
	legacy := strings.TrimSpace(legacyChatFile)
	if legacy == "" {
		return newPath // 旧库路径被显式清空（仅测试隔离用）——直接新建
	}
	if _, err := os.Stat(legacy); err != nil {
		return newPath // 无旧库——新建
	}
	if err := migrateChatDB(legacy, newPath); err != nil {
		log.Printf("⚠️ chat DB migration failed (continuing with legacy path %s): %v", legacy, err)
		return legacy
	}
	log.Printf("✅ chat DB migrated: %s → %s (legacy DB kept, not deleted)", legacy, newPath)
	return newPath
}

// activeChatDBPath — 对话库**真实在用**路径的只读解析（工具侧用；不迁移、不建目录、不搬不删任何文件）。
// 口径与 internal/api/tasks_persist.go 的 tasksReadPath、internal/agent/idle_persist.go 的 idleReadPath 一致：
//
//	① ZERG_CHAT_DB_PATH 显式覆盖（测试/多实例）⇒ 直接用（指定即「唯一来源」，不退旧路径）
//	② 新库（~/.zerg-chat/chat.db）存在 ⇒ 新库（迁移已完成 / 本机从无旧库）
//	③ 新库不存在而旧库（/tmp/zerg-chat/chat.db）存在 ⇒ **读旧**（此刻真实在用的就是旧库）
//	④ 两者都不存在 ⇒ 新库（首次运行——调用方自己新建）
//
// 硬化背景（2026-09-18 硬编码治理批 B-2）：sql_query / db_tables 的默认库原先写死
// "/tmp/zerg-chat/chat.db"（迁移前的旧路径；本机实测该路径已不存在 ⇒ 工具指向空库、静默查不到数据）。
// 这里只做**只读判定**：迁移仍然只在 OpenChatStore（ResolveChatDBPath）启动时执行一次，本函数绝不搬、不删、不写。
func activeChatDBPath() string {
	if p := os.Getenv("ZERG_CHAT_DB_PATH"); p != "" {
		return p
	}
	newPath := chatDBPath()
	if _, err := os.Stat(newPath); err == nil {
		return newPath // 新库存在——旧库完全不看（不 stat、不读）
	}
	legacy := strings.TrimSpace(legacyChatFile)
	if legacy != "" {
		if _, err := os.Stat(legacy); err == nil {
			return legacy // 读旧——不搬不删（旧库保留）
		}
	}
	return newPath // 无旧库——首次运行
}

// chatDBPath — 目标路径（默认 ~/.zerg-chat/chat.db）
func chatDBPath() string {
	if p := os.Getenv("ZERG_CHAT_DB_PATH"); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return legacyChatFile
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
