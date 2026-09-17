package api

import (
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/chat"
)

// TestWrapObsDeltaNilSafe —— 回归守卫：onDelta 为 nil 时**不得 panic**，且计时器仍照常计数。
// 事故背景：包装器无条件调用 onDelta ⇒ 非流式路径空指针 ⇒ 流中断 + 不落库（2026-09-16 实测）。
func TestWrapObsDeltaNilSafe(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", dir)
	timer := chat.NewObsTimer("sess-wrap", 1, "m")
	w := wrapObsDelta(timer, nil)
	for i := 0; i < 3; i++ {
		w("output", "x") // 旧实现这里必 panic
	}
	timer.Finish("finish")

	// 再验：非 nil 时要**原样透传**（包装不改语义）
	var got [][2]string
	w2 := wrapObsDelta(timer, func(dt, tx string) { got = append(got, [2]string{dt, tx}) })
	w2("output", "hello")
	if len(got) != 1 || got[0] != [2]string{"output", "hello"} {
		t.Fatalf("包装器改变了下游语义：%v", got)
	}
}

// TestWrapObsDeltaKeepsNonNilWrapper —— 第二重不变量：**onDelta 为 nil 时包装器本身也必须非 nil**。
// 理由：InferStream 只在 onDelta != nil 时才回调；若"顺手"把 nil 直接透传回去，计时器就再也不会
// 记分块（观测面静默丢数据——一种不报错但更难查的回归）。
// 变异：把 wrapObsDelta 改成 `if onDelta == nil { return nil }` ⇒ 本用例必红。
func TestWrapObsDeltaKeepsNonNilWrapper(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", dir)
	timer := chat.NewObsTimer("sess-wrap2", 2, "m")
	if w := wrapObsDelta(timer, nil); w == nil {
		t.Fatal("onDelta 为 nil 时包装器不得为 nil（否则计时器丢失分块）")
	}
}

// TestObsDeltaWrapperIsTheHistoricalPanicShape —— 复现 2026-09-16 事故的**调用形状**并钉住修复：
// 上游（loopcore 收尾轮）本来就可能给出 nil 回调，本包包一层后再交给 InferStream；InferStream
// 内部按"非 nil 就调用"处理包装器，因此包装器必须自己把 nil 吞掉。
// 变异：包装器里改成无条件 `onDelta(deltaType, text)`（= 事故当晚的写法）⇒ 本用例当场 panic ⇒ 红。
func TestObsDeltaWrapperIsTheHistoricalPanicShape(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", dir)
	timer := chat.NewObsTimer("sess-wrap3", 3, "m")

	// 上游给出 nil（收尾轮无增量消费者），本包包装后交给下游
	var upstream func(deltaType, text string)
	wrapped := wrapObsDelta(timer, upstream)

	// 下游 InferStream 的姿势：非 nil 就调（包装器非 nil ⇒ 必被调）
	for i := 0; i < 3; i++ {
		wrapped("reasoning", "r")
		wrapped("output", "o")
	}
	timer.Finish("finish")
}
