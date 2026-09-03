package api

// docs_ops.go — 文档文件操作 API（v2.5.6——Mr2109 2026-08-29: UI 右键菜单）
// 新建目录/重命名/删除/复制/保存——docs/ 目录内文件管理

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const docsRoot = "<repo>/docs"

// safeDocPath 校验相对路径（防穿越）——返回绝对路径或空
func safeDocPath(rel string) string {
	if rel == "" || strings.Contains(rel, "..") || strings.HasPrefix(rel, "/") {
		return ""
	}
	abs := filepath.Join(docsRoot, rel)
	// 确认在 docsRoot 内
	if !strings.HasPrefix(abs, docsRoot) {
		return ""
	}
	return abs
}

// DocMkdirHandler POST /api/docs/mkdir {"dir":"项目文档/v2.5.7"} 新建目录（可多级）
func (h *Handlers) DocMkdirHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Dir string `json:"dir"`
	}
	if err := readJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "请求体解析失败: "+err.Error())
		return
	}
	abs := safeDocPath(req.Dir)
	if abs == "" {
		writeError(w, http.StatusBadRequest, "非法路径")
		return
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		writeError(w, http.StatusInternalServerError, "新建目录失败: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "dir": req.Dir})
}

// DocRenameHandler POST /api/docs/rename {"old":"...","new":"..."} 重命名（目录/文件）
func (h *Handlers) DocRenameHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Old string `json:"old"`
		New string `json:"new"`
	}
	if err := readJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "请求体解析失败: "+err.Error())
		return
	}
	oldAbs := safeDocPath(req.Old)
	newAbs := safeDocPath(req.New)
	if oldAbs == "" || newAbs == "" {
		writeError(w, http.StatusBadRequest, "非法路径")
		return
	}
	if _, err := os.Stat(oldAbs); err != nil {
		writeError(w, http.StatusNotFound, "目标不存在: "+req.Old)
		return
	}
	if err := os.Rename(oldAbs, newAbs); err != nil {
		writeError(w, http.StatusInternalServerError, "重命名失败: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "old": req.Old, "new": req.New})
}

// DocDeleteHandler POST /api/docs/delete {"path":"..."} 删除（目录/文件）
func (h *Handlers) DocDeleteHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
	}
	if err := readJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "请求体解析失败: "+err.Error())
		return
	}
	abs := safeDocPath(req.Path)
	if abs == "" {
		writeError(w, http.StatusBadRequest, "非法路径")
		return
	}
	if _, err := os.Stat(abs); err != nil {
		writeError(w, http.StatusNotFound, "目标不存在: "+req.Path)
		return
	}
	if err := os.RemoveAll(abs); err != nil {
		writeError(w, http.StatusInternalServerError, "删除失败: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "path": req.Path})
}

// DocCopyHandler POST /api/docs/copy {"from":"...","to":"..."} 复制文件
func (h *Handlers) DocCopyHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		From string `json:"from"`
		To   string `json:"to"`
	}
	if err := readJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "请求体解析失败: "+err.Error())
		return
	}
	fromAbs := safeDocPath(req.From)
	toAbs := safeDocPath(req.To)
	if fromAbs == "" || toAbs == "" {
		writeError(w, http.StatusBadRequest, "非法路径")
		return
	}
	content, err := os.ReadFile(fromAbs)
	if err != nil {
		writeError(w, http.StatusNotFound, "源不存在: "+req.From)
		return
	}
	if err := os.MkdirAll(filepath.Dir(toAbs), 0o755); err != nil {
		writeError(w, http.StatusInternalServerError, "创建目标目录失败: "+err.Error())
		return
	}
	if err := os.WriteFile(toAbs, content, 0o644); err != nil {
		writeError(w, http.StatusInternalServerError, "复制失败: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "from": req.From, "to": req.To})
}

// DocSaveHandler POST /api/docs/save {"path":"...","content":"..."} 保存 md 内容（编辑器用）
func (h *Handlers) DocSaveHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := readJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "请求体解析失败: "+err.Error())
		return
	}
	abs := safeDocPath(req.Path)
	if abs == "" {
		writeError(w, http.StatusBadRequest, "非法路径")
		return
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		writeError(w, http.StatusInternalServerError, "创建目录失败: "+err.Error())
		return
	}
	if err := os.WriteFile(abs, []byte(req.Content), 0o644); err != nil {
		writeError(w, http.StatusInternalServerError, "保存失败: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "path": req.Path})
}

// readJSONBody 读请求体并解析 JSON
func readJSONBody(r *http.Request, v interface{}) error {
	body, err := ioReadAll(r.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, v)
}
