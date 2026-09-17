package loopcore

import (
	"context"
	"errors"
	"strconv"
	"testing"
)

// delta_boundary_test.go — 第 ③ 层回归守卫（2026-09-16 主控 panic 事故）
//
// 断言（**带变异**：把 run.go 里的 callInfer 换回裸 `d.Infer(..., nil, nil)`，或让 normDelta
// 不再给 Noop 默认值 ⇒ 本文件必红）：
//  ① 内核每一次调 Infer，onDelta 都非 nil —— 「nil 到达回调」在边界上不再可能；
//  ② 无消费者的那一轮（Noop）**不产出任何 delta**；有消费者的轮**原样透传**（事件流可观测）。

// deltaCall — 一次 Infer 调用的记录
type deltaCall struct {
	idx        int
	toolsNil   bool // 收尾轮（不带工具）
	deltaNil   bool // 收到 nil onDelta —— 违反第 ① 层，必须为零
	deltaFired bool // 调 onDelta 之后事件流里是否新增 delta（透传验证）
}

// scriptedDelta — 记录型 Infer 桩：先调一次收到的 onDelta（调用本身不得 panic），再按剧本返回。
func scriptedDelta(calls *[]deltaCall, events *[]string, script []Response, errs map[int]error) Infer {
	i := 0
	return func(ctx context.Context, model, sysPrompt string, msgs []map[string]any,
		onDelta func(deltaType, text string), tools []map[string]any) (*Response, error) {
		idx := i
		i++
		rec := deltaCall{idx: idx, toolsNil: tools == nil, deltaNil: onDelta == nil}
		if onDelta != nil {
			before := len(*events)
			onDelta("output", "chunk-"+strconv.Itoa(idx))
			rec.deltaFired = len(*events) > before
		}
		*calls = append(*calls, rec)
		if e, ok := errs[idx]; ok {
			return nil, e
		}
		j := idx
		if j >= len(script) {
			j = len(script) - 1
		}
		r := script[j]
		return &r, nil
	}
}

var deltaTools = []map[string]any{{"type": "function", "function": map[string]any{"name": "read"}}}

func contentOnly(s string) Response { return Response{Content: s} }

func toolRoundWithArgs(args map[string]any) Response {
	return Response{ToolCalls: []ToolCall{{ID: "c1", Name: "read", Args: args, RawArgs: "{}"}}}
}

func runDeltaScenario(t *testing.T, cfg Config, script []Response, errs map[int]error) []deltaCall {
	t.Helper()
	var calls []deltaCall
	var events []string
	d := Deps{
		Infer: scriptedDelta(&calls, &events, script, errs),
		Exec: func(ctx context.Context, name string, args map[string]any) (string, string, error) {
			return "ok", "1ms", nil
		},
		Tools:  deltaTools,
		Events: func(event, payload string) { events = append(events, event) },
	}
	Run(context.Background(), cfg, "m", "sys", []map[string]any{{"role": "user", "content": "hi"}}, d)
	for _, c := range calls {
		if c.deltaNil {
			t.Errorf("第 %d 次 Infer 收到 nil onDelta —— 边界归一化失效（收尾轮必须传 NoopDelta）", c.idx)
		}
	}
	return calls
}

// TestNoopDeltaIsCallableAndPreservesNilSemantics —— 第 ① 层的单元面：
// 真 nil / typed-nil func ⇒ 归一化成可安全调用的 Noop；非 nil ⇒ 原样透传（不改语义）。
func TestNoopDeltaIsCallableAndPreservesNilSemantics(t *testing.T) {
	NoopDelta("output", "x") // 直接调用不得 panic
	normDelta(nil)("output", "x")

	if normDelta(nil) == nil {
		t.Fatal("normDelta(nil) 必须给出非 nil 的 Noop 默认值（nil 不允许到达 Infer）")
	}
	// ★ typed-nil：func 类型的 nil 无歧义（这里显式钉住 —— 若将来改成接口类型，这条会失真，
	// 必须先按能力判，不能沿用 `!= nil`）。
	var typedNil func(deltaType, text string)
	if normDelta(typedNil) == nil {
		t.Fatal("typed-nil func 也必须被归一化")
	}

	got := 0
	f := func(deltaType, text string) { got++ }
	normDelta(f)("output", "y")
	if got != 1 {
		t.Fatalf("非 nil 回调必须原样透传（实际被调用 %d 次）", got)
	}
}

// TestCallInferNeverHandsNilToImplementer —— 内核出口的契约面。
// 实现方（这里模拟 api 侧当年的写法：**无条件调用** onDelta）在经 callInfer 之后必须安全。
// 变异：callInfer 直通（不归一化）⇒ 本用例当场 panic ⇒ 红。
func TestCallInferNeverHandsNilToImplementer(t *testing.T) {
	sawNil := false
	d := Deps{Infer: func(ctx context.Context, model, sysPrompt string, msgs []map[string]any,
		onDelta func(deltaType, text string), tools []map[string]any) (*Response, error) {
		if onDelta == nil {
			sawNil = true
			return &Response{Content: "ok"}, nil
		}
		onDelta("output", "x") // 无条件调用（当年 api 包装器的姿势）
		return &Response{Content: "ok"}, nil
	}}
	if _, err := d.callInfer(context.Background(), "m", "sys", nil, nil, nil); err != nil {
		t.Fatalf("callInfer 返回错误：%v", err)
	}
	if sawNil {
		t.Fatal("callInfer 把裸 nil 交给了 Infer —— 收尾轮必须传 NoopDelta")
	}
}

// TestInferNeverReceivesNilDelta —— 端到端（内核四条路径）：每条路径上 Infer 都不得收到 nil，
// 且收尾轮（无消费者）不产出 delta、正常轮原样透传。
func TestInferNeverReceivesNilDelta(t *testing.T) {
	t.Run("自然收尾（有消费者，须透传）", func(t *testing.T) {
		calls := runDeltaScenario(t, Config{MaxRounds: 3}, []Response{contentOnly("答")}, nil)
		if len(calls) != 1 {
			t.Fatalf("期望 1 次推理，实际 %d", len(calls))
		}
		if calls[0].toolsNil {
			t.Errorf("自然轮不该是收尾轮（tools=nil）")
		}
		if !calls[0].deltaFired {
			t.Errorf("正常轮的 onDelta 必须原样透传（事件流未收到 delta）")
		}
	})

	t.Run("限时收尾轮（无消费者，须静默）", func(t *testing.T) {
		calls := runDeltaScenario(t, Config{MaxRounds: 3},
			[]Response{contentOnly("收尾答")}, map[int]error{0: errors.New("请求超时")})
		if len(calls) != 2 || !calls[1].toolsNil {
			t.Fatalf("期望「超时 ⇒ 收尾轮」共 2 次推理（末次不带工具），实际 %+v", calls)
		}
		if calls[1].deltaFired {
			t.Errorf("收尾轮无消费者 ⇒ 不得产出 delta（Noop 语义被破坏）")
		}
	})

	t.Run("空参数收尾轮", func(t *testing.T) {
		script := []Response{
			toolRoundWithArgs(map[string]any{}),
			toolRoundWithArgs(map[string]any{}),
			toolRoundWithArgs(map[string]any{}),
			contentOnly("收尾答"),
		}
		calls := runDeltaScenario(t, Config{MaxRounds: 6}, script, nil)
		if len(calls) < 4 || !calls[len(calls)-1].toolsNil {
			t.Fatalf("期望「连续空参数 ⇒ 收尾轮」，实际 %+v", calls)
		}
		if calls[len(calls)-1].deltaFired {
			t.Errorf("收尾轮无消费者 ⇒ 不得产出 delta")
		}
	})

	t.Run("守卫升级收尾轮", func(t *testing.T) {
		same := toolRoundWithArgs(map[string]any{"path": "a.txt"})
		script := []Response{same, same, same, same, same, contentOnly("收尾答")}
		calls := runDeltaScenario(t, Config{MaxRounds: 6}, script, nil)
		if len(calls) < 4 || !calls[len(calls)-1].toolsNil {
			t.Fatalf("期望「指纹守卫升级 ⇒ 收尾轮」，实际 %+v", calls)
		}
		if calls[len(calls)-1].deltaFired {
			t.Errorf("收尾轮无消费者 ⇒ 不得产出 delta")
		}
	})
}
