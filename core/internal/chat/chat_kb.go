// chat_kb.go — v2.5.7 对话知识库工具（C6——kb_search——Go 直查 knowledge.db）
// 决策: 不拉起 Python MCP 进程（每次调用加载 1.2GB db + embedding 太重——拖慢对话）
// Go 直查同库同 SQL（mcp_kb_server.py _search_keyword 逻辑复刻——BM25 trigram + LIKE 兜底）
// 知识库铁律（Mr2109）: 回答前不确定先查库——对话内置 kb_search——系统提示已声明

package chat

import (
	"database/sql"
	"fmt"
	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

// KBPath — 知识库数据库路径（与 mcp_kb_server.py 一致）
// KBPath — 知识库 SQLite 路径（2026-09-11 B 批：不再硬编码私有卷路径）
// 覆盖顺序：ZERG_KB_PATH → <工作区>/data/knowledge.db
// （Mr2109本机沿用原位置：已在仓库 .env 中设 ZERG_KB_PATH 指向本机知识库）
var KBPath = func() string {
	if v := os.Getenv("ZERG_KB_PATH"); v != "" {
		return v
	}
	return filepath.Join(statepath.WorkspaceRoot(), "data", "knowledge.db")
}()

// kbItem — 知识条目（搜索返回）
type kbItem struct {
	ID       int64
	Domain   string
	Content  string
	Tags     string
	Source   string
	FilePath string
	Score    float64
}

// KbSearchExecute — kb_search 工具执行（导出——SendMessage 工具循环用）
func KbSearchExecute(query string, limit int) (string, error) {
	if limit <= 0 || limit > 20 {
		limit = 10
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return "请提供搜索关键词", nil
	}
	if _, err := os.Stat(KBPath); err != nil {
		return fmt.Sprintf("知识库不可用: %v", err), nil
	}
	db, err := sql.Open("sqlite", KBPath)
	if err != nil {
		return fmt.Sprintf("打开知识库失败: %v", err), nil
	}
	defer db.Close()
	// 只读模式（防污染知识库）
	db.SetMaxOpenConns(1)

	rows, err := db.Query(`
		SELECT k.id, k.domain, k.content, k.tags, k.source, k.file_path, bm25(knowledge_fts_tg) as score
		FROM knowledge_fts_tg JOIN knowledge k ON k.id = knowledge_fts_tg.rowid
		WHERE knowledge_fts_tg MATCH ? ORDER BY score LIMIT ?`, query, limit)
	var items []kbItem
	if err == nil {
		items = scanKBItems(rows)
		rows.Close()
	}
	if len(items) == 0 {
		// LIKE 兜底（2 字词 trigram 无索引——拆词 OR 匹配——比 MCP 连续 LIKE 更实用）
		words := strings.Fields(query)
		if len(words) == 0 {
			words = []string{query}
		}
		conds := make([]string, 0, len(words))
		args := make([]any, 0, len(words)+1)
		for _, w := range words {
			conds = append(conds, "content LIKE ?")
			args = append(args, "%"+w+"%")
		}
		args = append(args, limit)
		rows2, err2 := db.Query(fmt.Sprintf(
			"SELECT id, domain, content, tags, source, file_path, 0 as score FROM knowledge WHERE %s LIMIT ?",
			strings.Join(conds, " OR ")), args...)
		if err2 == nil {
			items = scanKBItems(rows2)
			rows2.Close()
		}
	}
	if len(items) == 0 {
		return "知识库无结果", nil
	}
	// 格式化（同 MCP _fmt_item——摘要 + 元数据）——P4-41 低置信标记（score=0 噪音——防模型当身份线索编造）
	var out []string
	for _, it := range items {
		head := fmt.Sprintf("[%s] id=%d score=%.1f", it.Domain, it.ID, it.Score)
		if it.Score == 0 {
			head += " [低置信——仅供参考——不作为事实依据]"
		}
		if it.Tags != "" {
			head += " tags=" + it.Tags
		}
		if it.Source != "" {
			head += " src=" + it.Source
		}
		content := it.Content
		if len(content) > 200 {
			content = content[:200] + "…"
		}
		out = append(out, head+"\n  "+content)
	}
	return strings.Join(out, "\n\n"), nil
}

// scanKBItems — 扫描查询结果
func scanKBItems(rows *sql.Rows) []kbItem {
	var items []kbItem
	for rows.Next() {
		var it kbItem
		if err := rows.Scan(&it.ID, &it.Domain, &it.Content, &it.Tags, &it.Source, &it.FilePath, &it.Score); err != nil {
			continue
		}
		items = append(items, it)
	}
	return items
}
