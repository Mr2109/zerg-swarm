package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// 网关卡表（circuit breaker）只读快照 + 手动复位 HTTP 接口。
//
// 路由（注册在 cmd/zerg-core/main.go，与既有 /api/* 同一套 AuthMiddleware 鉴权）：
//
//	GET  /api/gateway/breakers        → {"breakers":[{"host":"x3","state":"open","last_error":"...","cooldown_remaining_s":21.3,...}]}
//	POST /api/gateway/breakers/reset  → 请求体可选 {"host":"x3"}（缺省/空 = 全部复位）→ {"reset":N}
//
// 动机：熔断后客户端只见「无可用候选（全部熔断或排除）」——看不到原因/恢复时间/怎么复位。
// 这两个接口把网关内存态 breaker 状态只读暴露出来，并提供与 chat 包 compact-reset 同风格的手动复位入口。

// maxBreakerResetBody 复位请求体读取上限（防超大 body）
const maxBreakerResetBody = 4096

// BreakersHandler — GET /api/gateway/breakers（只读熔断状态快照）
func (h *Handlers) BreakersHandler(w http.ResponseWriter, r *http.Request) {
	if h.Gateway == nil {
		writeErrorCode(w, http.StatusServiceUnavailable, "GATEWAY_NOT_READY", "网关未就绪（未注入 Gateway 实例）")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"breakers": h.Gateway.BreakerSnapshot(),
	})
}

// BreakersResetHandler — POST /api/gateway/breakers/reset（手动复位熔断状态）
// 请求体可选 JSON {"host":"x3"}；host 缺省/空串 = 全部复位。返回 {"reset":N}（被清掉的机器数）。
func (h *Handlers) BreakersResetHandler(w http.ResponseWriter, r *http.Request) {
	if h.Gateway == nil {
		writeErrorCode(w, http.StatusServiceUnavailable, "GATEWAY_NOT_READY", "网关未就绪（未注入 Gateway 实例）")
		return
	}
	var req struct {
		Host string `json:"host"`
	}
	// 请求体可选——为空则全量复位（UI「复位全部」按钮不带 body）
	if r.Body != nil {
		body, err := io.ReadAll(io.LimitReader(r.Body, maxBreakerResetBody))
		if err != nil {
			writeErrorCode(w, http.StatusBadRequest, "INVALID_BODY", "请求体读取失败: "+err.Error())
			return
		}
		if len(bytes.TrimSpace(body)) > 0 {
			if err := json.Unmarshal(body, &req); err != nil {
				writeErrorCode(w, http.StatusBadRequest, "INVALID_BODY", "请求体解析失败: "+err.Error())
				return
			}
		}
	}
	n := h.Gateway.ResetBreakers(strings.TrimSpace(req.Host))
	writeJSON(w, http.StatusOK, map[string]interface{}{"reset": n})
}
