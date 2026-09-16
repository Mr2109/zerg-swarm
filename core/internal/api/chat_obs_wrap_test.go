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
