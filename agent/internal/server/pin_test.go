package server

// pin_test.go —— 子端 /pin /unpin 的反例优先测试（批 4：主控观测面把"不被自动驱逐"的锁定意图落到本端）。
//
// 红线（逐条有断言）：
//   - Q5：ttl_s<=0 一律 400；
//   - 非驻留绝不启动：/pin 一个不在本端驻留清单里的模型 → 409，且**不产生任何驻留项**（绝不启动/接管）。

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/agent/internal/backend"
)

func pinServer() *Server {
	return NewServer(NewAgent("x3", "tok", nil, backend.NewManager(nil, "x3"), ""))
}

// postPin 直接调 handler（与既有 unload_test.go 同风格），返回状态码 + 解析后的响应。
func postPin(t *testing.T, s *Server, token, path, body string) (int, map[string]interface{}) {
	t.Helper()
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest("POST", path, nil)
	} else {
		req = httptest.NewRequest("POST", path, strings.NewReader(body))
	}
	if token != "" {
		req.Header.Set("X-Auth-Token", token)
	}
	rec := httptest.NewRecorder()
	if path == "/pin" {
		s.handlePin(rec, req)
	} else {
		s.handleUnpin(rec, req)
	}
	var out map[string]interface{}
	if rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("响应不是 JSON: %q (%v)", rec.Body.String(), err)
		}
	}
	return rec.Code, out
}

// Q5：ttl_s<=0 一律 400（无 TTL 的 pin 等同内存泄漏）。
func TestHandlePin_TTLRequired(t *testing.T) {
	s := pinServer()
	for _, ttl := range []string{"0", "-1"} {
		code, out := postPin(t, s, "tok", "/pin", `{"model":"x","ttl_s":`+ttl+`}`)
		if code != http.StatusBadRequest {
			t.Fatalf("ttl_s=%s 应 400，实得 %d body=%+v", ttl, code, out)
		}
		if out["error"] != "pin_ttl_required" {
			t.Fatalf("错误码应为 pin_ttl_required，实得 %+v", out)
		}
	}
}

// 红线：非驻留项一律拒绝，且拒绝路径**绝不产生驻留项**（不启动、不接管）。
func TestHandlePin_NotResidentNeverAdopts(t *testing.T) {
	agent := NewAgent("x3", "tok", nil, backend.NewManager(nil, "x3"), "")
	s := NewServer(agent)
	code, out := postPin(t, s, "tok", "/pin", `{"model":"ghost","ttl_s":600}`)
	if code != http.StatusConflict {
		t.Fatalf("非驻留应 409，实得 %d body=%+v", code, out)
	}
	if out["error"] != "not_resident" {
		t.Fatalf("错误码应为 not_resident，实得 %+v", out)
	}
	if got := agent.backends.ResidentDetail(); len(got) != 0 {
		t.Fatalf("红线被破：拒绝路径竟产生了驻留项 %+v", got)
	}
}

// 鉴权不变：无令牌一律 401（/pin 不是免鉴权旁路）。
func TestHandlePin_Unauthorized(t *testing.T) {
	s := pinServer()
	if code, _ := postPin(t, s, "", "/pin", `{"model":"x","ttl_s":60}`); code != http.StatusUnauthorized {
		t.Fatalf("无令牌应 401，实得 %d", code)
	}
}

// 缺模型名 → 400。
func TestHandlePin_MissingModel(t *testing.T) {
	s := pinServer()
	if code, out := postPin(t, s, "tok", "/pin", `{"ttl_s":60}`); code != http.StatusBadRequest {
		t.Fatalf("缺 model 应 400，实得 %d body=%+v", code, out)
	}
}

// /unpin 同理：非驻留 → 409，且不产生驻留项。
func TestHandleUnpin_NotResident(t *testing.T) {
	agent := NewAgent("x3", "tok", nil, backend.NewManager(nil, "x3"), "")
	s := NewServer(agent)
	code, out := postPin(t, s, "tok", "/unpin", `{"model":"ghost"}`)
	if code != http.StatusConflict {
		t.Fatalf("非驻留 unpin 应 409，实得 %d body=%+v", code, out)
	}
	if len(agent.backends.ResidentDetail()) != 0 {
		t.Fatal("红线被破：unpin 拒绝路径产生了驻留项")
	}
}
