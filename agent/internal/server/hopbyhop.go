// hopbyhop.go —— 逐跳头过滤（GAP-20261019-09）。
//
// 病（实测）：子端把上游引擎的响应头**整表**拷进自己的响应，其中含
// `Transfer-Encoding: chunked`；随后又显式 `Set Content-Length` ⇒ Go 每次请求都打
//
//	http: WriteHeader called with both Transfer-Encoding of "chunked" and a Content-Length of N
//
// 这类响应在网关/代理一侧可被截断（历史上网关读到 3902/4082 的截断正是这一族）。
//
// 口径：按 RFC 7230 §6.1 的**逐跳头**跳过；`Content-Length` 也跳过（本端自己按
// 实际 body 长度设，绝不照抄上游的值 —— 上游可能来自 chunked 而无此头）。
// 其余端到端头（Content-Type、X-Zerg-* 等）原样透传，✗ 不做别的过滤。
package server

import "strings"

// isHopByHopHeader —— 该响应头是否**只属于这一跳**（不得透传给下一跳）。
func isHopByHopHeader(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "connection", "keep-alive", "proxy-authenticate", "proxy-authorization",
		"te", "trailer", "transfer-encoding", "upgrade",
		"content-length": // 本端按实际 body 长度自设 —— ✗ 照抄上游
		return true
	}
	return false
}
