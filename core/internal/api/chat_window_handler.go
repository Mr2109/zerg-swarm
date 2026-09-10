// chat_window_handler.go — 丙批补（2026-09-10）：召回指针的"回到原文"端点
//
// 背景：压缩后摘要尾部带 `(可搜回:session_search(session_id=…, around_id=…))`——
// 模型可据此检索，但**人**在界面上没法一键跳回去看被压缩的原文。本端点给 UI 用：
// 复用 session_search 的 read 模式（同一条窗口逻辑，不另写查询），返回可直接展示的文本块。
//
// GET /api/chat/sessions/{id}/window?around_id=N&limit=K
package api

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/Mr2109/zerg-swarm/core/internal/chat"
)

// ChatWindowHandler — 取会话中某条消息前后的一段（召回指针跳转/运维查看）
// around_id 缺省或 0 → 取最近 limit 条；limit 默认 20（上限 100）
func (h *ChatHandlers) ChatWindowHandler(w http.ResponseWriter, r *http.Request) {
	id := chiURLParam(r, "id")
	if id == "" {
		writeChatError(w, http.StatusBadRequest, fmt.Errorf("chat: 会话 id 必填"))
		return
	}
	aroundID, _ := strconv.ParseInt(r.URL.Query().Get("around_id"), 10, 64)
	limit := 20
	if l, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && l > 0 {
		if l > 100 {
			l = 100
		}
		limit = l
	}
	args := map[string]any{
		"session_id": id,
		"limit":      float64(limit),
	}
	if aroundID > 0 {
		args["around_id"] = float64(aroundID)
	}
	text, err := chat.SessionSearchExecute(args, h.store)
	if err != nil {
		writeChatError(w, http.StatusBadGateway, err)
		return
	}
	writeChatJSON(w, http.StatusOK, map[string]any{
		"session_id": id,
		"around_id":  aroundID,
		"limit":      limit,
		"text":       text,
	})
}

// ChatCompactResetHandler — 重置该会话的压缩冷却/硬熔断（丙批补：手动恢复入口）
// POST /api/chat/sessions/{id}/compact-reset
func (h *ChatHandlers) ChatCompactResetHandler(w http.ResponseWriter, r *http.Request) {
	id := chiURLParam(r, "id")
	if id == "" {
		writeChatError(w, http.StatusBadRequest, fmt.Errorf("chat: 会话 id 必填"))
		return
	}
	if err := chat.ResetCompactState(id); err != nil {
		writeChatError(w, http.StatusInternalServerError, fmt.Errorf("chat: 重置压缩状态失败: %w", err))
		return
	}
	inCooldown, disabled, until, streak := chat.CompactStateSnapshot(id)
	writeChatJSON(w, http.StatusOK, map[string]any{
		"ok":          true,
		"session_id":  id,
		"in_cooldown": inCooldown,
		"disabled":    disabled,
		"until":       until,
		"streak":      streak,
		"message":     "该会话压缩冷却/硬熔断已重置——下一次超阈值即重试",
	})
}
