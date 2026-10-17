package chat

// chat_infer_face_fallback_test.go — GAP-20261017-15（P0）回归。
//
// 病症（实测 chat_20261017_161924_71a510）：模型把 `{"command":"zerg gate check …"}` 当正文吐出，
// 第 1 轮执行了、**第 2 轮起不执行**。真因：生产对话路径是「提示面工具」（<tools> 写进系统提示，
// 请求体**不发** tools 字段）⇒ soleToolFace(nil) 恒为 nil ⇒ GAP-20261017-14 的裸参数对象兜底
// 在生产面永远够不到（它只在单测里被手工塞了 face 才绿）。
//
// 本文件钉三件：
//  1. 生产接线（不带 tools）下，裸参数对象**必须**被认成一次调用 —— 两条分支各跑一次；
//  2. 连续两轮都吐裸参数对象 ⇒ 两轮各出 1 次（本单主牙）；
//  3. 反向：带标签的单调用仍 1；多工具面不猜；显式「本轮无工具」（只有 __temp__ 标记）不认；
//     真坏格式仍判 broken（不静默当正文）。

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fxGap15BareArgs — chat_20261017_161924_71a510 #667 原文（逐字取自 ~/.zerg-chat/chat.db）。
const fxGap15BareArgs = "正文判据是 zerg gate check gen-gap-ledger --check，落账时刻 2026-10-15。现在可能已变。我要现读这条判据命令。先跑判据。同时可以跑 zerg gap export 看有没有 --check/干跑开关。先跑判据。\n{\"command\":\"zerg gate check gen-gap-ledger --check\"}"

// fxGap15Answer — 「真结论」轮（用来让假引擎在第 N 轮收口）。
const fxGap15Answer = "rc=0，判据已现读；结论：仍缺。"

// gap15Srv — 假引擎：按序返回 contents 里的一条（用完重复最后一条）；记下每次请求体是否带 tools。
type gap15Srv struct {
	srv     *httptest.Server
	bodies  []map[string]any
	repeats int
}

func newGap15Srv(t *testing.T, sse bool, contents ...string) *gap15Srv {
	t.Helper()
	g := &gap15Srv{}
	i := 0
	g.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var m map[string]any
		_ = json.Unmarshal(raw, &m)
		g.bodies = append(g.bodies, m)
		c := contents[len(contents)-1]
		if i < len(contents) {
			c = contents[i]
		}
		i++
		if !sse {
			b, _ := json.Marshal(map[string]any{
				"choices": []map[string]any{{
					"message":       map[string]any{"role": "assistant", "content": c},
					"finish_reason": "stop",
				}},
				"usage": map[string]any{"prompt_tokens": 10, "completion_tokens": 20},
			})
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(b)
			return
		}
		esc, _ := json.Marshal(c)
		w.Header().Set("Content-Type", "text/event-stream")
		var sb strings.Builder
		sb.WriteString(`data: {"choices":[{"delta":{"role":"assistant","content":` + string(esc) + `},"finish_reason":null}]}` + "\n\n")
		sb.WriteString("data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n")
		sb.WriteString("data: [DONE]\n\n")
		_, _ = w.Write([]byte(sb.String()))
	}))
	t.Cleanup(g.srv.Close)
	return g
}

func (g *gap15Srv) bodyHasTools() []bool {
	out := make([]bool, 0, len(g.bodies))
	for _, b := range g.bodies {
		_, ok := b["tools"]
		out = append(out, ok)
	}
	return out
}

var gap15Msgs = func() []map[string]any {
	return []map[string]any{{"role": "user", "content": "跑：zerg gate check gen-gap-ledger --check"}}
}

// TestGap15BareArgsWithoutToolsParam — 生产接线（不带 tools）：两条分支都必须出 1 次调用，
// 且正文里的裸参数对象被剥掉（= 没被当正文交付）。
func TestGap15BareArgsWithoutToolsParam(t *testing.T) {
	for _, sse := range []bool{true, false} {
		name := "非流式"
		if sse {
			name = "流式"
		}
		g := newGap15Srv(t, sse, fxGap15BareArgs)
		ci := NewChatInfer(g.srv.URL, "")
		ctx := WithSessionID(context.Background(), "gap15")
		var res *InferResult
		var err error
		if sse {
			res, err = ci.InferStream(ctx, "m", "sys", gap15Msgs(), nil) // 变参为空 = 生产现状（Deps.Tools 未接线）
		} else {
			res, err = ci.Infer(ctx, "m", "sys", gap15Msgs())
		}
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(res.ToolCalls) != 1 {
			t.Fatalf("%s：裸参数对象应认成 1 次调用，实际 %d（正文=%q）", name, len(res.ToolCalls), res.Content)
		}
		if res.ToolCalls[0].Name != "zerg" {
			t.Fatalf("%s：归属工具应为 zerg，实际 %q", name, res.ToolCalls[0].Name)
		}
		if cmd, _ := res.ToolCalls[0].Args["command"].(string); cmd != "zerg gate check gen-gap-ledger --check" {
			t.Fatalf("%s：命令取错 %q", name, cmd)
		}
		if strings.Contains(res.Content, `{"command"`) {
			t.Fatalf("%s：裸参数对象仍留在正文（=被当正文交付）%q", name, res.Content)
		}
		if has := g.bodyHasTools(); len(has) != 1 || has[0] {
			t.Fatalf("%s：本用例的请求体不该带 tools（生产现状），实际 %v", name, has)
		}
	}
}

// TestGap15TwoConsecutiveRoundsEachOneCall — 本单主牙：同一个提示面、连续两轮都吐裸参数对象 ⇒ 各出 1 次。
func TestGap15TwoConsecutiveRoundsEachOneCall(t *testing.T) {
	g := newGap15Srv(t, true, fxGap15BareArgs, fxGap15BareArgs)
	ci := NewChatInfer(g.srv.URL, "")
	ctx := WithSessionID(context.Background(), "gap15")
	msgs := gap15Msgs()
	for round := 1; round <= 2; round++ {
		res, err := ci.InferStream(ctx, "m", "sys", msgs, nil)
		if err != nil {
			t.Fatalf("第 %d 轮: %v", round, err)
		}
		if len(res.ToolCalls) != 1 {
			t.Fatalf("第 %d 轮：应出 1 次调用，实际 %d（正文=%q）", round, len(res.ToolCalls), res.Content)
		}
		// 模拟内核的下一轮上下文（assistant 回显 + tool 回执）
		msgs = append(msgs,
			map[string]any{"role": "assistant", "content": res.ToolCalls[0].RawArgs},
			map[string]any{"role": "tool", "tool_call_id": "t1", "content": "[成功·3字] rc=0"},
		)
	}
}

// TestGap15TaggedSingleCallStillOne — 反向①：正常带标签的单调用仍解析为 1，且不受兜底影响。
func TestGap15TaggedSingleCallStillOne(t *testing.T) {
	const tagged = "我先跑版本。\n<tool_call>\n{\"name\": \"zerg\", \"arguments\": {\"command\": \"zerg version\"}}\n</tool_call>"
	g := newGap15Srv(t, true, tagged)
	ci := NewChatInfer(g.srv.URL, "")
	res, err := ci.InferStream(context.Background(), "m", "sys", gap15Msgs(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.ToolCalls) != 1 || res.ToolCalls[0].Name != "zerg" {
		t.Fatalf("带标签单调用应仍为 1：%+v", res.ToolCalls)
	}
	if strings.Contains(res.Content, "<tool_call>") {
		t.Fatalf("标签块应被剥离，实际正文 %q", res.Content)
	}
}

// TestGap15ExplicitNoToolFaceMarker — 反向③：调用方显式声明「本轮没有工具面」（只给 __temp__ 标记）⇒ 不认。
// 这是收尾轮（不能再调工具）的口径：兜底不得把收尾轮的正文变成工具调用。
func TestGap15ExplicitNoToolFaceMarker(t *testing.T) {
	g := newGap15Srv(t, true, fxGap15BareArgs)
	ci := NewChatInfer(g.srv.URL, "")
	res, err := ci.InferStream(context.Background(), "m", "sys", gap15Msgs(), nil,
		[]map[string]any{{"__temp__": 0.0}})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.ToolCalls) != 0 {
		t.Fatalf("显式无工具面时不得认调用，实际 %d", len(res.ToolCalls))
	}
	if !strings.Contains(res.Content, `{"command"`) {
		t.Fatalf("正文应原样保留，实际 %q", res.Content)
	}
	if has := g.bodyHasTools(); len(has) != 1 || has[0] {
		t.Fatalf("__temp__ 标记不得进请求体 tools，实际 %v", has)
	}
}

// TestGap15MultiToolFaceDoesNotGuess — 反向④：多工具面仍不猜归属（裸参数对象 0 调用）。
func TestGap15MultiToolFaceDoesNotGuess(t *testing.T) {
	g := newGap15Srv(t, true, fxGap15BareArgs)
	ci := NewChatInfer(g.srv.URL, "")
	two := []map[string]any{ChatZergToolParam(), ChatZergToolParam()}
	res, err := ci.InferStream(context.Background(), "m", "sys", gap15Msgs(), nil, two)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.ToolCalls) != 0 {
		t.Fatalf("多工具面不得猜归属，实际 %d", len(res.ToolCalls))
	}
}

// TestGap15BrokenFragmentNotSwallowed — 反向②：标签在、内容解不出 ⇒ broken=true 且原样保留（上游照旧判坏格式）。
func TestGap15BrokenFragmentNotSwallowed(t *testing.T) {
	const broken = "<tool_call>\n{不是 JSON 也不是参数对象\n</tool_call>"
	calls, rest, bad := parseTextToolCalls(broken, promptSoleFace())
	if len(calls) != 0 || !bad {
		t.Fatalf("坏残片应 calls=0/broken=true，实际 calls=%d broken=%v", len(calls), bad)
	}
	if !strings.Contains(rest, "<tool_call>") {
		t.Fatalf("坏残片必须原样（含标签）交上游判坏格式，实际 %q", rest)
	}
	fmt.Printf("[GAP15] 坏残片：calls=%d broken=%v rest_has_tag=%v\n", len(calls), bad, strings.Contains(rest, "<tool_call>"))
}
