// hopbyhop_test.go —— GAP-20261019-09 的判据件。
//
// 两态：① 逐跳头（大小写/空白不敏感）一律判真 —— 尤其 transfer-encoding 与 content-length；
// ② 端到端头（Content-Type/X-Zerg-*/X-Request-Id）一律判假 —— 别把该透传的也拦掉。
package server

import "testing"

func TestIsHopByHopHeader(t *testing.T) {
	hop := []string{
		"Transfer-Encoding", "transfer-encoding", " TRANSFER-ENCODING ",
		"Content-Length", "Connection", "Keep-Alive", "TE", "Trailer", "Upgrade",
		"Proxy-Authenticate", "Proxy-Authorization",
	}
	for _, h := range hop {
		if !isHopByHopHeader(h) {
			t.Errorf("%q 应判为逐跳头（不透传）", h)
		}
	}
	endToEnd := []string{
		"Content-Type", "X-Zerg-Served-Model", "X-Zerg-Served-Host", "X-Request-Id",
		"Cache-Control", "Date",
	}
	for _, h := range endToEnd {
		if isHopByHopHeader(h) {
			t.Errorf("%q 是端到端头，不该被拦", h)
		}
	}
}
