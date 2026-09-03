package api

// archive_api.go — 归档检索 API（2026-08-22 Mr2109补充）
// GET /api/archive —— 归档列表（读索引 index.jsonl）
// GET /api/archive?q=<关键词> —— 检索（task_id 模糊匹配）
// GET /api/archive/{task_id} —— 单归档详情（索引信息）

import (
	"bufio"
	"encoding/json"
	"net/http"
	"os"
	"strings"
)

// ArchiveEntry 归档索引条目
type ArchiveEntry struct {
	TaskID   string `json:"task_id"`
	Archived string `json:"archived"`
	Size     int64  `json:"size"`
	Expires  string `json:"expires"`
	Deleted  string `json:"deleted,omitempty"`
}

// loadArchiveIndex 读归档索引（index.jsonl——全部条目）
func loadArchiveIndex() []ArchiveEntry {
	var entries []ArchiveEntry
	f, err := os.Open(archiveIndex)
	if err != nil {
		return entries
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		var e ArchiveEntry
		if json.Unmarshal(scanner.Bytes(), &e) == nil {
			entries = append(entries, e)
		}
	}
	return entries
}

// ArchiveHandler 归档列表/检索（2026-08-22 Mr2109）
// GET /api/archive?q=<关键词>（q 空=全部）
func (h *Handlers) ArchiveHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	entries := loadArchiveIndex()
	if query != "" {
		var filtered []ArchiveEntry
		for _, e := range entries {
			if strings.Contains(e.TaskID, query) {
				filtered = append(filtered, e)
			}
		}
		entries = filtered
	}
	// 倒序（最新归档在前）
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"count":   len(entries),
		"entries": entries,
		"note":    "归档生命周期: 30 天归档（tar.gz）→ 90 天删除（Mr2109）",
	})
}
