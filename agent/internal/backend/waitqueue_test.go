// waitqueue_test.go —— P7 批 2/3：等待队列对外出口的验收。
//
// 覆盖：入队/出队基本语义、队列限长（满即 false ⇒ 上层 429）、出队重校验（卵已换 ⇒
// mismatch 且**不摘除**——防饿死）、观测面（len/peek/headETA）、以及惰性创建
// （手写构造的 Manager 也能用）。
package backend

import (
	"testing"
	"time"
)

// 惰性创建：测试里 &Manager{} 直接构造也要能入队（waitQueue 内部建）。
func TestWaitQueue_LazyInitAndBasic(t *testing.T) {
	m := &Manager{}
	if m.WaitQLen() != 0 {
		t.Fatalf("新队列应为空，实得 %d", m.WaitQLen())
	}
	if !m.WaitQPush("qwen", "payload-1", 42*time.Second) {
		t.Fatal("入队应成功")
	}
	if m.WaitQLen() != 1 {
		t.Fatalf("入队后长度应为 1，实得 %d", m.WaitQLen())
	}
	items := m.WaitQPeek()
	if len(items) != 1 || items[0].Model != "qwen" || items[0].ETA != 42*time.Second {
		t.Fatalf("peek 应如实给出 model/ETA，实得 %+v", items)
	}
	eta, ok := m.WaitQHeadETA()
	if !ok || eta != 42 {
		t.Fatalf("队头 ETA 应为 42，实得 %v ok=%v", eta, ok)
	}

	// 当前卵就是它要的 ⇒ 正常出队。
	item, payload, ok, mismatch := m.WaitQPop(func() string { return "qwen" }, false)
	if !ok || mismatch {
		t.Fatalf("一致时应正常出队，ok=%v mismatch=%v", ok, mismatch)
	}
	if item.Model != "qwen" || payload != "payload-1" {
		t.Fatalf("出队内容不对：%+v / %v", item, payload)
	}
	if m.WaitQLen() != 0 {
		t.Fatalf("出队后应为空，实得 %d", m.WaitQLen())
	}
}

// 出队重校验（§7.7 修补 4 核心）：卵已换 ⇒ mismatch 且**不摘除**（下次仍能看到队头，防饿死）。
func TestWaitQueue_MismatchKeepsHead(t *testing.T) {
	m := &Manager{}
	if !m.WaitQPush("k2", "p", 0) {
		t.Fatal("入队应成功")
	}
	item, _, ok, mismatch := m.WaitQPop(func() string { return "qwen" }, false)
	if ok || !mismatch {
		t.Fatalf("卵已换时应 mismatch 且不视为出队成功，ok=%v mismatch=%v", ok, mismatch)
	}
	if item.Model != "k2" {
		t.Fatalf("mismatch 时仍应回报队头内容供上层重走孵化，实得 %+v", item)
	}
	if m.WaitQLen() != 1 {
		t.Fatalf("mismatch 不得摘除队头（防饿死），实得长度 %d", m.WaitQLen())
	}
	// 重走孵化后当前卵变成 k2 ⇒ 同一项可正常出队。
	_, _, ok2, mismatch2 := m.WaitQPop(func() string { return "k2" }, false)
	if !ok2 || mismatch2 {
		t.Fatalf("孵化完成后应可出队，ok=%v mismatch=%v", ok2, mismatch2)
	}
}

// 限长：满 ⇒ push 返回 false（Q5：上层立即 429，不挂起、不排队）。
func TestWaitQueue_FullRejectsImmediately(t *testing.T) {
	q := newP2Queue(3)
	for i := 0; i < 3; i++ {
		if !q.push(qReq{model: "m"}) {
			t.Fatalf("第 %d 项应入队成功", i+1)
		}
	}
	if q.push(qReq{model: "m"}) {
		t.Fatal("队列满时 push 必须返回 false（上层回 429）")
	}
	if q.lenOf() != 3 {
		t.Fatalf("被拒的项不得入队，长度应仍为 3，实得 %d", q.lenOf())
	}
}

// 队空 + wait=false ⇒ 立即返回空（不得阻塞在线程里）。
func TestWaitQueue_EmptyPopNonBlocking(t *testing.T) {
	m := &Manager{}
	done := make(chan struct{})
	go func() {
		_, _, ok, mismatch := m.WaitQPop(func() string { return "x" }, false)
		if ok || mismatch {
			t.Errorf("空队列非等待模式应返回 (false,false)，实得 ok=%v mismatch=%v", ok, mismatch)
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("空队列非等待 pop 阻塞了（应立即可返回）")
	}
}

// dropHead：摘除队头（"重走孵化后仍不匹配"的兜底，防队头卡死饿死后续项）。
func TestWaitQueue_DropHead(t *testing.T) {
	m := &Manager{}
	m.WaitQPush("a", "pa", 0)
	m.WaitQPush("b", "pb", 0)
	item, payload, ok := m.WaitQDropHead()
	if !ok || item.Model != "a" || payload != "pa" {
		t.Fatalf("应摘除并返回队头 a，实得 ok=%v %+v %v", ok, item, payload)
	}
	if m.WaitQLen() != 1 {
		t.Fatalf("摘除后应剩 1 项，实得 %d", m.WaitQLen())
	}
	if items := m.WaitQPeek(); len(items) != 1 || items[0].Model != "b" {
		t.Fatalf("剩下的应是 b，实得 %+v", items)
	}
	if _, _, ok := m.WaitQDropHead(); !ok {
		t.Fatal("应能摘掉最后一项")
	}
	if _, _, ok := m.WaitQDropHead(); ok {
		t.Fatal("队空时 dropHead 必须返回 ok=false")
	}
}
