package chat

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// 事故回归（2026-09-17 23:56:42 实测的那 1 次 upstream_timeout 真身）：
// 对话层客户端自带写死 http.Client{Timeout: 120s} ⇒ 它压过 loopcore 每轮按卵推导的
// RoundTimeout（Qwen3.8-27B = 1200s）**和**网关首 token 闸（600s），成为实际生效的单轮上限。
// 那一轮引擎真跑了 174s 且**正常完成**（子端 task 926：14150 token，23:54:42→23:57:36 status=200，
// 网关全程没断它）⇒ 被这个 120s 掐死在 total_ms=120006（err_text 原文：
// `Client.Timeout exceeded while awaiting headers`）。
// 判据只有这一条：**单轮时长不许有第二个更短的真源**。
// 变色条件：把 NewChatInfer 的 client 改回 &http.Client{Timeout: 120 * time.Second} ⇒ 本用例如实变红。
func TestChatInferClientHasNoHardcodedRoundCap(t *testing.T) {
	t.Setenv("ZERG_EGG_PROFILE_DIR", t.TempDir()) // 隔离：不读本机真实卵档案
	c := NewChatInfer("http://127.0.0.1:1", "")
	if c.client.Timeout != 0 {
		t.Fatalf("客户端自带写死上限 %s，而按卵单轮限时是 %s（首 token 闸 %ds）⇒ 闸与轮预算都到不了，长思考轮必被它先掐",
			c.client.Timeout, RoundTimeoutFor("Qwen3.8-27B"), EffectiveGateSec("Qwen3.8-27B"))
	}
}

// roundCtxFor 的语义两条都要有牙齿：
//
//	① 调用方没给截止 ⇒ 按卵推导补上（且必须 > 写死的 120s，否则等于没修）；
//	② 调用方给了截止（loopcore 的 roundCtx 是唯一真源）⇒ **原样透传**，既不放宽也不收紧。
//
// 改成"总是覆盖调用方"或"永不补"都会在这里红。
func TestRoundCtxForSemantics(t *testing.T) {
	t.Setenv("ZERG_EGG_PROFILE_DIR", t.TempDir())
	const model = "Qwen3.8-27B"

	// ① 无截止 ⇒ 补
	ctx, cancel := roundCtxFor(context.Background(), model)
	defer cancel()
	dl, ok := ctx.Deadline()
	if !ok {
		t.Fatal("没有截止 ⇒ 裸请求会无限期挂住")
	}
	got, want := time.Until(dl), RoundTimeoutFor(model)
	if got < want-time.Second || got > want+time.Second {
		t.Fatalf("补的限时 %s 与按卵推导 %s 不一致（口径漂了就是两套真相）", got, want)
	}
	if got <= 120*time.Second {
		t.Fatalf("生效限时 %s 仍 ≤ 写死的 120s（事故真身；首 token 闸 %ds 根本到不了）", got, EffectiveGateSec(model))
	}

	// ② 调用方已给截止 ⇒ 原样透传
	parent, pcancel := context.WithTimeout(context.Background(), 900*time.Second)
	defer pcancel()
	pdl, _ := parent.Deadline()
	ctx2, cancel2 := roundCtxFor(parent, model)
	defer cancel2()
	d2, ok2 := ctx2.Deadline()
	if !ok2 || !d2.Equal(pdl) {
		t.Fatalf("调用方的截止被改写了：%v（应原样 %v）= 轮预算被没收", d2, pdl)
	}
}

// 慢轮不被提前掐：经真实 HTTP 走一遍（假网关 1.2s 后回 200）。
// 守的是"去掉硬上限"没有把正常轮次弄坏、也没退化成"无限期挂住"（无 ctx 截止时仍按卵推导补）。
func TestChatInferSlowRoundSurvives(t *testing.T) {
	t.Setenv("ZERG_EGG_PROFILE_DIR", t.TempDir())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(1200 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":1}}`)
	}))
	defer srv.Close()

	c := NewChatInfer(srv.URL, "tok")
	start := time.Now()
	res, err := c.Infer(context.Background(), "Qwen3.8-27B", "sys", []map[string]any{{"role": "user", "content": "hi"}})
	if err != nil {
		t.Fatalf("慢轮（1.2s）不该失败：%v", err)
	}
	if res == nil || res.Content != "ok" {
		t.Fatalf("响应解析异常：%+v", res)
	}
	if d := time.Since(start); d < 1200*time.Millisecond {
		t.Fatalf("假网关根本没等满就被返回（%s）——用例本身失效", d)
	}
}

// 反向守卫：客户端字段若被重新写死，上面第一条必须能发现；这里额外把"三个时限"的
// 大小关系钉死（闸 ≤ 单轮 ≤ 墙钟），否则"按卵推导"会被无声改成常数。
func TestRoundBudgetOrdering(t *testing.T) {
	t.Setenv("ZERG_EGG_PROFILE_DIR", t.TempDir())
	for _, m := range []string{"Qwen3.8-27B", "gemma-4-26B", "example-35b-v2"} {
		g := time.Duration(EffectiveGateSec(m)) * time.Second
		rt := RoundTimeoutFor(m)
		wc := WallClockFor(m)
		if rt < 120*time.Second {
			t.Errorf("%s：单轮限时 %s < 120s 下限", m, rt)
		}
		if g > 0 && rt < g {
			t.Errorf("%s：单轮限时 %s < 首 token 闸 %s ⇒ 闸还没到就被掐（自相矛盾）", m, rt, g)
		}
		if wc < rt {
			t.Errorf("%s：墙钟 %s < 单轮 %s", m, wc, rt)
		}
	}
}
