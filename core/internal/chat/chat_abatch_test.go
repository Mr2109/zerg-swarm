package chat

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
)

// newTestStore — 独立临时库（避开 /tmp 迁移路径，防测试互相污染）
func newTestStore(t *testing.T) *ChatStore {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("ZERG_CHAT_DB_PATH", filepath.Join(dir, "chat.db"))
	st, err := OpenChatStore()
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

// TestFTSV4MigrationChineseSearch — 甲批 T4 回归：
// 旧库（unicode61）启动迁移后，中文查询必须能命中（修复前 MATCH 零命中）
func TestFTSV4MigrationChineseSearch(t *testing.T) {
	st := newTestStore(t)
	se0, err := st.CreateSession("example-35b-v2", "desktop", "", "中文检索回归")
	if err != nil {
		t.Fatal(err)
	}
	sid := se0.ID
	for _, c := range []string{
		"请用 sys_info 工具查看系统信息，然后用中文总结",
		"继续",
		"工具调用格式无效，请重新输出格式正确的工具调用",
		"the tool call is invalid, retry",
	} {
		if _, err := st.AddMessage(&Message{SessionID: sid, Role: "user", Content: c, Active: true, Timestamp: now()}); err != nil {
			t.Fatal(err)
		}
	}

	// 模拟旧库：把 FTS 换回 unicode61 + schema v3
	if _, err := st.db.Exec("DROP TABLE IF EXISTS messages_fts"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.db.Exec("CREATE VIRTUAL TABLE messages_fts USING fts5(content, tokenize='unicode61')"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.db.Exec("INSERT INTO messages_fts(rowid, content) SELECT id, COALESCE(content,'') FROM messages"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.db.Exec("UPDATE schema_version SET version = 3"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.db.Exec("UPDATE sessions SET archived = 0"); err != nil {
		t.Fatal(err)
	}
	st.Close()

	// 重新打开 → 触发 v4 迁移
	st2, err := OpenChatStore()
	if err != nil {
		t.Fatalf("重开（迁移）失败: %v", err)
	}
	defer st2.Close()

	var ddl string
	if err := st2.db.QueryRow("SELECT COALESCE(sql,'') FROM sqlite_master WHERE name='messages_fts'").Scan(&ddl); err != nil {
		t.Fatal(err)
	}
	if !containsSub(ddl, "trigram") {
		t.Fatalf("v4 迁移未生效（分词器仍非 trigram）: %s", ddl)
	}

	// 2 字中文查询（查询侧走 LIKE）
	got, err := st2.SearchMessages("工具", 10)
	if err != nil {
		t.Fatalf("2 字中文查询失败: %v", err)
	}
	if len(got) == 0 {
		t.Fatalf("2 字中文查询零命中（应走 LIKE 命中 2 条）")
	}

	// 3 字以上中文（trigram MATCH）
	got3, err := st2.SearchMessages("工具调用", 10)
	if err != nil || len(got3) == 0 {
		t.Fatalf("3 字中文查询失败或零命中: %v (%d)", err, len(got3))
	}

	// 整词中文
	got2, err := st2.SearchMessages("继续", 10)
	if err != nil || len(got2) == 0 {
		t.Fatalf("「继续」查询零命中: %v (%d)", err, len(got2))
	}

	// 英文不回归
	gotEn, err := st2.SearchMessages("invalid", 10)
	if err != nil || len(gotEn) == 0 {
		t.Fatalf("英文查询零命中（回归）: %v (%d)", err, len(gotEn))
	}
	t.Logf("✓ 中文检索修复：工具=%d 工具调用=%d 继续=%d invalid=%d", len(got), len(got3), len(got2), len(gotEn))
}

// TestTwoPhaseCleanup — 甲批 T3 回归：软删 → 宽限 → 级联硬删；幂等
func TestTwoPhaseCleanup(t *testing.T) {
	st := newTestStore(t)
	seOld, err := st.CreateSession("example-35b-v2", "desktop", "", "过期会话")
	if err != nil {
		t.Fatal(err)
	}
	old := seOld.ID
	oldMsg, err := st.AddMessage(&Message{SessionID: old, Role: "user", Content: "很久以前的对话", Active: true, Timestamp: now()})
	if err != nil {
		t.Fatal(err)
	}
	seNew, err := st.CreateSession("example-35b-v2", "desktop", "", "新会话")
	if err != nil {
		t.Fatal(err)
	}
	fresh := seNew.ID

	// 把 old 会话的活动时间改到 100 天前
	oldSec := float64(time.Now().AddDate(0, 0, -100).Unix())
	if _, err := st.db.Exec("UPDATE sessions SET last_activity_at = ? WHERE id = ?", oldSec, old); err != nil {
		t.Fatal(err)
	}

	nowSec := float64(time.Now().Unix())
	cutoff := float64(time.Now().AddDate(0, 0, -retentionDays).Unix())
	purgeAfter := float64(time.Now().AddDate(0, 0, graceDays).Unix())

	// 干跑：应报"将软删 1 / 将硬删 0"
	soft, purge, err := st.CountCleanupPlan(cutoff, nowSec)
	if err != nil || soft != 1 || purge != 0 {
		t.Fatalf("干跑统计错误: soft=%d purge=%d err=%v（应为 1/0）", soft, purge, err)
	}

	// 阶段1：软删
	n, err := st.SoftDeleteExpired(cutoff, purgeAfter)
	if err != nil || n != 1 {
		t.Fatalf("阶段1 软删应 1 个: n=%d err=%v", n, err)
	}
	// 幂等：再跑一次应为 0
	if n, _ := st.SoftDeleteExpired(cutoff, purgeAfter); n != 0 {
		t.Fatalf("阶段1 不幂等（重复软删了 %d 个）", n)
	}
	// 宽限期内不硬删
	if n, err := st.PurgeSoftDeleted(nowSec); err != nil || n != 0 {
		t.Fatalf("宽限期内不应硬删: n=%d err=%v", n, err)
	}
	// 新会话不受影响
	se, err := st.GetSession(fresh)
	if err != nil || se == nil || se.Archived {
		t.Fatalf("新会话被误处理: %+v err=%v", se, err)
	}

	// 宽限期满 → 级联硬删
	n, err = st.PurgeSoftDeleted(purgeAfter + 1)
	if err != nil || n != 1 {
		t.Fatalf("宽限期满应硬删 1 个: n=%d err=%v", n, err)
	}
	if _, err := st.GetSession(old); err == nil {
		t.Fatalf("硬删后会话仍存在")
	}
	// 消息与 FTS 一并清理
	var msgCnt, ftsCnt int
	_ = st.db.QueryRow("SELECT count(*) FROM messages WHERE id = ?", oldMsg).Scan(&msgCnt)
	_ = st.db.QueryRow("SELECT count(*) FROM messages_fts WHERE rowid = ?", oldMsg).Scan(&ftsCnt)
	if msgCnt != 0 || ftsCnt != 0 {
		t.Fatalf("级联删除不完整: messages=%d fts=%d", msgCnt, ftsCnt)
	}
	// 幂等
	if n, _ := st.PurgeSoftDeleted(purgeAfter + 1); n != 0 {
		t.Fatalf("阶段2 不幂等（重复硬删 %d 个）", n)
	}
	t.Logf("✓ 两阶段清理：干跑 1/0 → 软删 1（幂等）→ 宽限内 0 → 期满硬删 1（级联 FTS+消息，幂等）")
}

// TestStateDirMigration — 甲批 T2 回归：/tmp 状态文件 → ~/.zerg/state（有旧文件则搬，无则用新路径）
func TestStateDirMigration(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, "legacy.json")
	if err := os.WriteFile(legacy, []byte(`{"bash":3}`), 0o644); err != nil {
		t.Fatal(err)
	}
	stateDir := filepath.Join(dir, "state")
	t.Setenv("ZERG_STATE_DIR", stateDir)

	got := statepath.MigrateIfNeeded(legacy, "tool_uses.json")
	if got != filepath.Join(stateDir, "tool_uses.json") {
		t.Fatalf("迁移目标路径错误: %s", got)
	}
	b, err := os.ReadFile(got)
	if err != nil || string(b) != `{"bash":3}` {
		t.Fatalf("旧文件未搬入新目录: %v %s", err, string(b))
	}
	if _, err := os.Stat(legacy); err != nil {
		t.Fatalf("旧文件不应被删除（保守迁移）: %v", err)
	}
	// 幂等：目标已存在则不覆盖
	if err := os.WriteFile(got, []byte(`{"grep":9}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = statepath.MigrateIfNeeded(legacy, "tool_uses.json")
	b2, _ := os.ReadFile(got)
	if string(b2) != `{"grep":9}` {
		t.Fatalf("目标已存在时被覆盖（破坏幂等）: %s", string(b2))
	}
	t.Logf("✓ 状态迁移：首搬成功 + 旧文件保留 + 幂等不覆盖")
}

func containsSub(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
