package loopcore

import "context"

// delta.go — 流式增量回调（onDelta）的**边界归一化**（2026-09-16 主控 panic 事故的治本层）
//
// ── 事故现场（实测，/tmp/zerg-core.log 中共 6 次，2026-09-16 20:34–21:55）──
//
//	panic: runtime error: invalid memory address or nil pointer dereference
//	 -> api.(*ChatHandlers).SendMessage.func3.1        chat_handlers.go:860
//	    chat.(*ChatInfer).InferStream                   chat_infer.go:271
//	    api.(*ChatHandlers).SendMessage.func3           chat_handlers.go:862
//	    loopcore.Run                                    run.go:233
//	    api.(*ChatHandlers).SendMessage                 chat_handlers.go:877
//
// 机理：内核的收尾轮（loopguard 升级 / 空参数 / 限时）过去直接写 `d.Infer(..., nil, nil)`
// —— 即「本轮没有增量消费者」。调用方（api 的 OBS-1 计时包装器）包一层之后**无条件调用**
// onDelta ⇒ 非 nil 的包装器里握着真 nil 的被包装体 ⇒ 空指针。三次现象同时出现：
// ①客户端流中断无 done ②助手回复不落库 ③HTTP 仍 200（net/http 每连接 recover）。
//
// ── 三层口径（本文件负责第 ① 层）──
//
//	① 接口边界让 nil 不可能：内核**永不**把裸 nil 交给 Infer —— 一律经 callInfer 归一化，
//	   nil 换成显式 Noop 默认值（NoopDelta）。「可选」在边界处就变成「必填，但可为 noop」。
//	② 运行时守卫 + 保留 nil 语义：调用方包装可选回调时，必须「真 nil 跳过 / 非 nil 原样透传」
//	   （api 侧 wrapObsDelta；有 TestWrapObsDeltaNilSafe + AST 守卫两重守门）。
//	③ 回归用例（带变异）：本包用 TestInferNeverReceivesNilDelta 断言「每次 Infer 都拿到非 nil
//	   onDelta，且 Noop 轮不产出任何 delta」；把 callInfer 换回裸 nil ⇒ 用例必红。
//
// ★ typed-nil 陷阱（为什么这里坚持 func 值而**不**改造成接口）：
// 接口里装「值为 nil 的指针」时 `x == nil` 恒为假 —— 判 nil 会静默失效，正是最难查的一类
// 空指针。func 值的 nil 语义**无歧义**（不存在 typed-nil），所以「显式 Noop + 判 nil 透传」
// 这套在 func 上是可靠的。若将来把 onDelta 改成接口类型，必须先按能力判（或禁止 typed-nil），
// 不能沿用 `!= nil`。

// DeltaFunc — 增量回调的形状（对外显式命名，方便调用方与实现方在签名上对齐）。
// deltaType: "reasoning"/"output"；text: 本次增量文本。
type DeltaFunc func(deltaType, text string)

// NoopDelta — 显式 Noop 默认值：什么都不做，但**非 nil**。
// 用途：内核某条路径确实没有增量消费者时用它，而不是传裸 nil（调用方不必再猜 nil 的含义）。
// 语义与「传 nil 且实现方判 nil 跳过」完全一致：不产出任何 delta。
func NoopDelta(deltaType, text string) {}

// normDelta — 「可选 → 必填」的归一化：真 nil 换 NoopDelta；非 nil **原样透传**（不包一层、
// 不改语义）。这是第 ① 层的唯一实现点。
func normDelta(onDelta DeltaFunc) DeltaFunc {
	if onDelta == nil {
		return NoopDelta
	}
	return onDelta
}

// callInfer — 对 d.Infer 的**唯一出口**。内核里所有「调模型」都必须从这里走（run.go 中不再
// 出现裸 d.Infer(...)）。契约：
//
//	· 对实现方（Infer）：onDelta 永远非 nil（无消费者时是 NoopDelta，调用它安全且无副作用）。
//	  实现方**仍应**保留自己的判 nil（别的调用方可能直接调它），但那已是第二道防线。
//	· 对内核：nil 不再是一个需要调用方理解的概念 —— 「没有消费者」= NoopDelta。
func (d Deps) callInfer(ctx context.Context, model, sysPrompt string, msgs []map[string]any,
	onDelta DeltaFunc, tools []map[string]any) (*Response, error) {
	return d.Infer(ctx, model, sysPrompt, msgs, normDelta(onDelta), tools)
}

// EmitDelta — 给**实现方/包装方**的 nil-safe 发射器：真 nil 跳过；非 nil 原样透传（参数不动）。
// 包装可选回调时用它，就不会再写出「无条件调用」那种把 nil 保护抹掉的实现。
func EmitDelta(onDelta DeltaFunc, deltaType, text string) {
	if onDelta != nil {
		onDelta(deltaType, text)
	}
}
