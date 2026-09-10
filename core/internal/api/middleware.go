// package api 提供 HTTP API 处理器和中间件。
package api

import (
	"net/http"
)

// AuthMiddleware 认证中间件：检查所有 /api/* 端点的 X-Auth-Token 头。
func AuthMiddleware(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 只拦截 /api/* 路径
			if len(r.URL.Path) < 4 || r.URL.Path[:4] != "/api" {
				next.ServeHTTP(w, r)
				return
			}

			// 检查 X-Auth-Token 头
			authToken := r.Header.Get("X-Auth-Token")
			if authToken == "" {
				// 多语言 L4（2026-09-11）：带分类码（message 不变——双写期）
				writeErrorCode(w, http.StatusUnauthorized, "AUTH_TOKEN_MISSING", "缺少认证令牌 (X-Auth-Token header required)")
				return
			}
			if authToken != token {
				writeErrorCode(w, http.StatusForbidden, "AUTH_TOKEN_INVALID", "认证令牌无效")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
