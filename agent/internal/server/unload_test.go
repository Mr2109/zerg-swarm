package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/agent/internal/backend"
)

// ── 批 3：/unload 支持"定向卸载"（只卸够），无体仍是旧语义（全卸） ─────────────

func unloadServer() *Server {
	return NewServer(NewAgent("node-a", "tok", nil, backend.NewManager(nil, "node-a"), ""))
}

func postUnload(t *testing.T, s *Server, token, body string) (int, map[string]interface{}) {
	t.Helper()
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest("POST", "/unload", nil)
	} else {
		req = httptest.NewRequest("POST", "/unload", strings.NewReader(body))
	}
	if token != "" {
		req.Header.Set("X-Auth-Token", token)
	}
	rec := httptest.NewRecorder()
	s.handleUnload(rec, req)
	var out map[string]interface{}
	if rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("响应不是 JSON: %q (%v)", rec.Body.String(), err)
		}
	}
	return rec.Code, out
}

// 带 {"models":[...]}：解析出目标清单；未驻留的名字如实报 skipped（不静默忽略）。
func TestHandleUnload_Targeted(t *testing.T) {
	s := unloadServer()
	code, out := postUnload(t, s, "tok", `{"models":["ghost-a","ghost-b"]}`)
	if code != 200 {
		t.Fatalf("状态码应为 200，实得 %d", code)
	}
	if out["ok"] != true {
		t.Fatalf("响应应 ok=true，实得 %+v", out)
	}
	skipped, _ := out["skipped"].([]interface{})
	if len(skipped) != 2 || skipped[0] != "ghost-a" || skipped[1] != "ghost-b" {
		t.Fatalf("未驻留目标应如实进 skipped，实得 %+v", out["skipped"])
	}
	reasons, _ := out["reasons"].(map[string]interface{})
	if reasons["ghost-a"] != "not_resident" {
		t.Fatalf("skipped 原因应为 not_resident，实得 %+v", reasons)
	}
	if _, hasStopped := out["stopped"]; !hasStopped {
		t.Fatalf("定向卸载响应应带 stopped 字段（可为空），实得 %+v", out)
	}
}

// 无体（老客户端/运维路径）：仍是「全卸」这一语义，但**钉住的默认模型必须留下**
// （缺口 GAP-20261019-01 负控：不得静默停掉被保护的默认模型）。
//
// 响应形状随之与定向档同形（stopped/skipped/reasons）：理由是「全卸也会跳项」，
// 跳了谁、为什么，调用方与运维必须看得见 —— 静默才是缺陷。
func TestHandleUnload_NoBodyKeepsLegacySemantics(t *testing.T) {
	s := unloadServer()
	code, out := postUnload(t, s, "tok", "")
	if code != 200 {
		t.Fatalf("状态码应为 200，实得 %d", code)
	}
	if out["ok"] != true {
		t.Fatalf("全卸响应应 ok=true，实得 %+v", out)
	}
	// 本夹具没有驻留项 ⇒ 两栏都空；关键是「跳项必须报出来」，而不是形状回到旧实现。
	if _, hasStopped := out["stopped"]; !hasStopped {
		t.Fatalf("全卸必须报出 stopped（可为空）—— 否则跳过的项无从分辨，实得 %+v", out)
	}
	if _, hasSkipped := out["skipped"]; !hasSkipped {
		t.Fatalf("全卸必须报出 skipped（可为空），实得 %+v", out)
	}
}

// 鉴权不变：无令牌一律 401。
func TestHandleUnload_Unauthorized(t *testing.T) {
	s := unloadServer()
	code, _ := postUnload(t, s, "", `{"models":["a"]}`)
	if code != 401 {
		t.Fatalf("无令牌应 401，实得 %d", code)
	}
}
