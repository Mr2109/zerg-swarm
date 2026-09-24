package gateway

import (
	"net/http"
	"testing"
)

// 三级会话键的单测（设计 v1.7 §二 · 缺口 Q-215 的另一半）——正 3 负 3。

// 正 1：显式头 `X-Zerg-Session` 优先（比体里的会话号更优先）。
func TestDeriveSessionKeyExplicitHeaderWins(t *testing.T) {
	h := http.Header{sessionHeaderName: []string{"explicit-1"}}
	body := []byte(`{"model":"m","session_id":"from-body"}`)
	if got := deriveSessionKey(h, body); got != "explicit-1" {
		t.Fatalf("显式头应优先 ⇒ 期望 explicit-1，实得 %q", got)
	}
}

// 负 1：无头时取体里的会话号（**既有行为逐字不变** ✓）。
func TestDeriveSessionKeyFromBody(t *testing.T) {
	for _, key := range []string{"session_id", "session", "llm_request_id"} {
		body := []byte(`{"model":"m","` + key + `":"body-1"}`)
		if got := deriveSessionKey(http.Header{}, body); got != "body-1" {
			t.Fatalf("%s 应从体里取 ⇒ 期望 body-1，实得 %q", key, got)
		}
	}
}

// 正 2：隐式键 —— 同 model + 同 system（**含空白/换行差异** ✓ 规范化）⇒ 同键，且带 imp- 前缀。
func TestImplicitKeyStableAndNormalized(t *testing.T) {
	a := []byte(`{"model":"example-35b-v2","messages":[{"role":"system","content":"你是 Zerg 助手。"},{"role":"user","content":"hi"}]}`)
	b := []byte(`{"model":"example-35b-v2","messages":[{"role":"system","content":"  你是 Zerg\n助手。\t "}]}`)
	ka, kb := deriveSessionKey(http.Header{}, a), deriveSessionKey(http.Header{}, b)
	if ka == "" || kb == "" {
		t.Fatalf("隐式键不该为空：ka=%q kb=%q", ka, kb)
	}
	if ka != kb {
		t.Fatalf("同 model + 同 system（仅空白差异）应同键：%q vs %q", ka, kb)
	}
	if len(ka) < 5 || ka[:4] != implicitKeyPrefix {
		t.Fatalf("隐式键应带前缀 %q，实得 %q", implicitKeyPrefix, ka)
	}
}

// 负 2：system 不同 ⇒ 键不同（不许把不同会话粘到一起）。
func TestImplicitKeyDiffersBySystem(t *testing.T) {
	a := []byte(`{"model":"m","messages":[{"role":"system","content":"A"}]}`)
	b := []byte(`{"model":"m","messages":[{"role":"system","content":"B"}]}`)
	if deriveSessionKey(http.Header{}, a) == deriveSessionKey(http.Header{}, b) {
		t.Fatalf("不同 system 不该同键")
	}
}

// 负 3：不同 model ⇒ 键不同（模型换了不该复用绑定）。
func TestImplicitKeyDiffersByModel(t *testing.T) {
	a := []byte(`{"model":"m1","messages":[{"role":"system","content":"same"}]}`)
	b := []byte(`{"model":"m2","messages":[{"role":"system","content":"same"}]}`)
	if deriveSessionKey(http.Header{}, a) == deriveSessionKey(http.Header{}, b) {
		t.Fatalf("不同 model 不该同键")
	}
}

// 负 4：**无 system ⇒ 空键**（不启用粘性 —— 绝不臆造会话 ✓ 行为与今天逐字一致 ✓）。
func TestImplicitKeyEmptyWithoutSystem(t *testing.T) {
	cases := [][]byte{
		[]byte(`{"model":"m","messages":[{"role":"user","content":"hi"}]}`),
		[]byte(`{"model":"m","messages":[{"role":"system","content":"   "}]}`),
		[]byte(`{"model":"m","system":""}`),
		[]byte(`not-json`),
		[]byte(``),
	}
	for _, body := range cases {
		if got := deriveSessionKey(http.Header{}, body); got != "" {
			t.Fatalf("无 system/非法体应返回空键，实得 %q（体=%s）", got, body)
		}
	}
	// 内容数组形状的 system 也要能取到（正控 ✓）
	arr := []byte(`{"model":"m","messages":[{"role":"system","content":[{"type":"text","text":"S"}]}]}`)
	if deriveSessionKey(http.Header{}, arr) == "" {
		t.Fatalf("数组形状的 system 应能推导出键")
	}
}
