// chat_paths_hardcode_test.go — 硬编码路径治理批 B-2 回归用例（2026-09-18）
// 覆盖两处落点：
//
//	① logTail(target="ui") — 原写死 "/tmp/zerg-ui.log" ⇒ 改为 statepath 运行期日志目录派生
//	② sql_query / db_tables 默认库 — 原写死 "/tmp/zerg-chat/chat.db"（迁移前的旧库）⇒ 改为解析真实在用库
//
// 测试纪律（与 obs_*.go 同源）：**绝不碰真机落点** ——
//
//	· 真机 /tmp/zerg-ui.log、真机 /tmp/zerg-core.log 一律不读不写（ZERG_LOG_DIR 指到 t.TempDir()）
//	· 真机聊天库（~/.zerg-chat/chat.db）与真机旧库（/tmp/zerg-chat/chat.db）一律不碰
//	  —— HOME 指到 t.TempDir() 隔离新库；旧库路径用 legacyChatFile 变量切到临时目录（绝不 stat 真机 /tmp）
package chat

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// isolateLegacyChatDB — 把「旧库路径」切到临时目录（避免用例读写真机 /tmp/zerg-chat/chat.db）。
// 返回临时旧库路径（不创建文件——由用例决定）。
func isolateLegacyChatDB(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "legacy-chat.db")
	old := legacyChatFile
	legacyChatFile = p
	t.Cleanup(func() { legacyChatFile = old })
	return p
}

// isolateHome — 把 HOME 指到临时目录（新库 ~/.zerg-chat/chat.db 落在隔离目录里）。
func isolateHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("ZERG_CHAT_DB_PATH", "")
	return home
}

// TestLogTail_UI_DerivesFromRuntimeLogDir — UI 日志必须从 statepath 运行期日志目录读
// （ZERG_LOG_DIR 改道后读自己那份；老实现写死 /tmp 字面量 ⇒ 读的是别处的文件）
func TestLogTail_UI_DerivesFromRuntimeLogDir(t *testing.T) {
	dir := t.TempDir() // 注意：不是 /tmp —— 只有真派生的实现才可能命中
	t.Setenv("ZERG_LOG_DIR", dir)
	const marker = "UI-LOG-MARKER-b2-77f1"

	var sb strings.Builder
	for i := 0; i < 35; i++ {
		sb.WriteString("noise-line-" + strconv.Itoa(i) + "\n")
	}
	sb.WriteString(marker + "\n")
	if err := os.WriteFile(filepath.Join(dir, "zerg-ui.log"), []byte(sb.String()), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := logTail(map[string]any{"target": "ui", "lines": float64(30)})
	if err != nil {
		t.Fatalf("logTail(ui) 报错: %v", err)
	}
	if !strings.Contains(out, marker) {
		t.Fatalf("UI 日志未从 ZERG_LOG_DIR 派生目录读取（期望含 %q；实得 %d 字节: %q）", marker, len(out), out)
	}
	if strings.Contains(out, "日志不存在") {
		t.Fatalf("派生路径下文件存在却被报「日志不存在」: %q", out)
	}
}

// TestLogTail_UI_MissingFileReportsDerivedPath — 文件不存在时所报路径必须是派生路径（不是 /tmp 字面量）。
// 这条直接锁住「读的是哪个路径」：老实现会报 "/tmp/zerg-ui.log"。
func TestLogTail_UI_MissingFileReportsDerivedPath(t *testing.T) {
	dir := t.TempDir() // 空目录——没有 zerg-ui.log
	t.Setenv("ZERG_LOG_DIR", dir)
	want := filepath.Join(dir, "zerg-ui.log")

	out, err := logTail(map[string]any{"target": "ui"})
	if err != nil {
		t.Fatalf("logTail(ui) 报错: %v", err)
	}
	if !strings.Contains(out, "日志不存在: "+want) {
		t.Fatalf("期望报告派生路径 %q；实得: %q", want, out)
	}
	if strings.Contains(out, "/tmp/zerg-ui.log") {
		t.Fatalf("仍在读写死的 /tmp/zerg-ui.log 字面量: %q", out)
	}
}

// TestActiveChatDBPath_NewPreferred — 新库存在 ⇒ 用新库（旧库存在也完全不看）
func TestActiveChatDBPath_NewPreferred(t *testing.T) {
	home := isolateHome(t)
	legacy := isolateLegacyChatDB(t)
	writeFileT(t, legacy, []byte("legacy-not-a-real-db")) // 旧库存在：不该被选中

	newPath := filepath.Join(home, ".zerg-chat", "chat.db")
	writeFileT(t, newPath, []byte("live-db"))

	if got := activeChatDBPath(); got != newPath {
		t.Fatalf("新库存在时应取新库 %q，实得 %q", newPath, got)
	}
}

// TestActiveChatDBPath_LegacyReadWhenNewAbsent — 新无旧有 ⇒ 读旧；且**不搬、不删、不建**（红线断言）
func TestActiveChatDBPath_LegacyReadWhenNewAbsent(t *testing.T) {
	home := isolateHome(t)
	legacy := isolateLegacyChatDB(t)
	payload := []byte("legacy-payload-must-survive")
	writeFileT(t, legacy, payload)

	if got := activeChatDBPath(); got != legacy {
		t.Fatalf("新库不存在而旧库存在时应读旧库 %q，实得 %q", legacy, got)
	}

	// 红线：旧库原封不动（字节相同），新路径与其目录都没有被创建（只读判定——不搬不建）
	after, err := os.ReadFile(legacy)
	if err != nil {
		t.Fatalf("旧库被动了（读不到）: %v", err)
	}
	if string(after) != string(payload) {
		t.Fatalf("旧库内容被改写: %q", after)
	}
	newDir := filepath.Join(home, ".zerg-chat")
	if _, err := os.Stat(newDir); !os.IsNotExist(err) {
		t.Fatalf("只读解析不得创建新库目录 %q（err=%v）", newDir, err)
	}
}

// TestActiveChatDBPath_ExplicitOverrideWins — ZERG_CHAT_DB_PATH 显式覆盖优先（测试/多实例隔离）
// （与 ResolveChatDBPath 同口径：显式指定即唯一来源，不退旧路径）
func TestActiveChatDBPath_ExplicitOverrideWins(t *testing.T) {
	home := isolateHome(t)
	legacy := isolateLegacyChatDB(t)
	writeFileT(t, filepath.Join(home, ".zerg-chat", "chat.db"), []byte("live-db"))
	writeFileT(t, legacy, []byte("legacy-db"))

	override := filepath.Join(t.TempDir(), "custom.db")
	t.Setenv("ZERG_CHAT_DB_PATH", override)

	if got := activeChatDBPath(); got != override {
		t.Fatalf("显式覆盖应优先，期望 %q，实得 %q", override, got)
	}
}

// TestDBTools_DefaultDBIsLivePath — 端到端（真跑 sqlite3）：
// 不传 db 参数时查到的必须是「真实在用」的库，且**读旧语义**成立（新无旧有 ⇒ 查到旧库内容）。
func TestDBTools_DefaultDBIsLivePath(t *testing.T) {
	if _, err := exec.LookPath("sqlite3"); err != nil {
		t.Skip("sqlite3 CLI 不可用——跳过默认库端到端用例")
	}

	t.Run("新库存在⇒查新库", func(t *testing.T) {
		home := isolateHome(t)
		legacy := isolateLegacyChatDB(t)
		newPath := filepath.Join(home, ".zerg-chat", "chat.db")
		mkSQLiteT(t, newPath, "CREATE TABLE live_probe(x INTEGER);")
		mkSQLiteT(t, legacy, "CREATE TABLE legacy_probe(x INTEGER);")

		out, err := dbTables(map[string]any{})
		if err != nil {
			t.Fatalf("dbTables 默认库报错: %v", err)
		}
		if !strings.Contains(out, "live_probe") {
			t.Fatalf("默认库未指向在用新库（期望含 live_probe；实得 %q）", out)
		}
		if strings.Contains(out, "legacy_probe") {
			t.Fatalf("默认库指向了旧库（不应含 legacy_probe）: %q", out)
		}

		q, err := sqlQuery(map[string]any{"query": "SELECT name FROM sqlite_master WHERE type='table'"})
		if err != nil {
			t.Fatalf("sqlQuery 默认库报错: %v", err)
		}
		if !strings.Contains(q, "live_probe") || strings.Contains(q, "legacy_probe") {
			t.Fatalf("sqlQuery 默认库不是在用新库: %q", q)
		}
	})

	t.Run("新库不存在⇒读旧库", func(t *testing.T) {
		isolateHome(t) // 新库目录为空（不存在）
		legacy := isolateLegacyChatDB(t)
		mkSQLiteT(t, legacy, "CREATE TABLE legacy_probe(x INTEGER);")

		out, err := dbTables(map[string]any{})
		if err != nil {
			t.Fatalf("dbTables 默认库（退旧）报错: %v", err)
		}
		if !strings.Contains(out, "legacy_probe") {
			t.Fatalf("新库不存在时应退到旧库（期望含 legacy_probe；实得 %q）", out)
		}
	})
}

// ─────────────── 小工具 ───────────────

func writeFileT(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func mkSQLiteT(t *testing.T, path, ddl string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("sqlite3", path, ddl).CombinedOutput()
	if err != nil {
		t.Fatalf("建库失败 %s: %v (%s)", path, err, out)
	}
}
