package backend

// p2_queue_test.go —— P2 批 3：队列出队重校验 + 不打断在飞。
//
// 设计依据：设计-子端沙箱化.md §7.7 修补 4 / §7.2 / §13 Q5/Q6（口径已拍定）。
// 验收点：
//   - 出队时卵已换 ⇒ 重走孵化流程，不得直接转发（修补 4——#946 lost-wakeup 的正解）；
//   - 出队时卵匹配 ⇒ 正常出队；
//   - 队列限长（Q5）：满 ⇒ 立即拒绝（上层 429），不排队不挂起；
//   - 不打断在飞（Q6）：生成中被切换请求 ⇒ 在飞计数不被清、卵不被收——等待完成；
//   - 引擎 /slots 不可用 ⇒ 上述判定照常（正确性不挂在引擎特性上，§7.7 修补 3）。

import (
	"sync"
	"testing"
	"time"
)

// 出队重校验正例：卵匹配 ⇒ 正常出队。
func TestP2B3_Dequeue_Match(t *testing.T) {
	q := newP2Queue(4)
	if !q.push(qReq{model: "A", req: "r1", eta: 0}) {
		t.Fatal("入队应成功")
	}
	item, ok, mismatch := q.popDequeue(func() string { return "A" }, false)
	if !ok || mismatch {
		t.Fatalf("卵匹配 ⇒ 应正常出队，实得 ok=%v mismatch=%v", ok, mismatch)
	}
	if item.model != "A" || item.req != "r1" {
		t.Fatalf("出队项不对: %+v", item)
	}
	if q.lenOf() != 0 {
		t.Fatalf("出队后队列应空，实得 %d", q.lenOf())
	}
}

// 批 3 核心负例：出队时卵已换（队头要 A、当前孵的是 B）⇒ 不出队、上报 mismatch——
// 上层据此重走孵化流程，绝不把请求发给一个正在死的进程（§7.7 修补 4）。
func TestP2B3_Dequeue_EggSwapped_MismatchNoForward(t *testing.T) {
	q := newP2Queue(4)
	q.push(qReq{model: "A", req: "r1", eta: 0})
	q.push(qReq{model: "A", req: "r2", eta: 0})

	// 当前卵已是 B（队头的 A 已被收/换）
	item, ok, mismatch := q.popDequeue(func() string { return "B" }, false)
	if ok || !mismatch {
		t.Fatalf("卵已换 ⇒ 必须报 mismatch 且不得出队，实得 ok=%v mismatch=%v", ok, mismatch)
	}
	if item.model != "A" {
		t.Fatalf("mismatch 上报的应是队头项，实得 %+v", item)
	}
	// 队列保持原样（队头未动——重走孵化后它仍是最先被服务的）
	if q.lenOf() != 2 {
		t.Fatalf("mismatch 时队列不得消费，实得 %d", q.lenOf())
	}
	if got := q.peekAll(); got[0].req != "r1" || got[1].req != "r2" {
		t.Fatalf("队列顺序应保持，实得 %v", got)
	}
}

// 重走孵化完成后（当前卵变回 A）⇒ 同一队列头正常出队（「被唤醒后重新发起加载」闭环）。
func TestP2B3_Dequeue_AfterReload_Matches(t *testing.T) {
	q := newP2Queue(4)
	q.push(qReq{model: "A", req: "r1", eta: 0})

	current := "B" // 切换期：当前卵是 B
	item, ok, mismatch := q.popDequeue(func() string { return current }, false)
	if ok || !mismatch {
		t.Fatalf("切换期应 mismatch，实得 ok=%v mismatch=%v", ok, mismatch)
	}
	current = "A" // 重走孵化流程装回 A
	item, ok, mismatch = q.popDequeue(func() string { return current }, false)
	if !ok || mismatch {
		t.Fatalf("重孵后应正常出队，实得 ok=%v mismatch=%v", ok, mismatch)
	}
	if item.req != "r1" {
		t.Fatalf("出队的应是原请求 r1，实得 %+v", item)
	}
}

// 队列限长（Q5）：满 ⇒ 立即拒绝（上层回 429），不排队、不挂起、不雪崩。
func TestP2B3_QueueCapacity(t *testing.T) {
	q := newP2Queue(2)
	if !q.push(qReq{model: "A", req: "r1"}) || !q.push(qReq{model: "A", req: "r2"}) {
		t.Fatal("前两个入队应成功")
	}
	if q.push(qReq{model: "A", req: "r3"}) {
		t.Fatal("队列满 ⇒ 第三个必须被立即拒绝（429 语义）")
	}
	if q.lenOf() != 2 {
		t.Fatalf("拒绝后队列长度不变，实得 %d", q.lenOf())
	}
}

// 不打断在飞（Q6）：生成中被异模型切换请求 ⇒ 在飞计数保持、卵不被收、生成等它跑完。
// 场景：A 有 1 个在飞请求；此时 B 的请求到达 RequestModel ⇒ blocked；
// A 的生成跑完（release ⇒ 归零转空窗计时中）⇒ B 再请求 ⇒ 赢权收卵。
// 全程 A 的进程句柄从未被 stop（不打断、不硬杀）。
func TestP2B3_SwitchWaitsForInflightGeneration(t *testing.T) {
	now := time.Now()
	m := p2Manager(3, map[string]*subproc{
		"A": {model: "A", entry: inflightEntry(), port: 9431,
			state: StateReady, lastUsed: now, inflight: 1},
	})
	stopped := 0
	m.stopHook = func(sp *subproc) { stopped++ }

	// 切换请求 B 到达：A 有在飞 ⇒ blocked（排队等待，Q5），不得动作
	needUnload, blocked := m.RequestModel("B")
	if needUnload || !blocked {
		t.Fatalf("有在飞 ⇒ 必须 blocked，实得 needUnload=%v blocked=%v", needUnload, blocked)
	}
	if stopped != 0 {
		t.Fatalf("在飞生成期间不得停进程（Q6 不打断），实得停了 %d 次", stopped)
	}
	if m.inflightCount("A") != 1 {
		t.Fatalf("在飞计数不得被切换请求清掉，实得 %d", m.inflightCount("A"))
	}

	// A 的生成跑完（正常完成路径：releaseInflight）
	m.releaseInflight("A")
	if got := m.StateOf("A"); got != StateIdleArmed {
		t.Fatalf("生成完成后 A 应转空窗计时中，实得 %s", got)
	}
	if stopped != 0 {
		t.Fatalf("空窗计时中也不得立刻杀（要等巡检赢权），实得停了 %d 次", stopped)
	}

	// B 再战 ⇒ 赢权；出锁动作段才停进程（恰好一次）
	if needUnload, blocked := m.RequestModel("B"); !needUnload || blocked {
		t.Fatalf("A 已无在飞 ⇒ B 应赢权收卵，实得 needUnload=%v blocked=%v", needUnload, blocked)
	}
	m.mu.Lock()
	var victimSP *subproc
	for _, sp := range m.procs {
		if sp.state == StateDraining {
			victimSP = sp
			break
		}
	}
	m.mu.Unlock()
	if victimSP == nil {
		t.Fatal("赢权后应有 draining 态的受害卵")
	}
	m.stopVictimProcess(victimSP.proc, victimSP)
	if stopped != 1 {
		t.Fatalf("出锁动作段应恰好停一次，实得 %d", stopped)
	}
	// 停完句柄后摘除受害卵（finalize——与 ReapIdle 的重新持锁摘 map 同一套纪律）
	m.mu.Lock()
	for name, sp := range m.procs {
		if sp == victimSP {
			delete(m.procs, name)
		}
	}
	m.mu.Unlock()
	if got := m.StateOf("A"); got != "" {
		t.Fatalf("收卵后 A 不再驻留，实得 %q", got)
	}
}

// 并发出队：多消费抢同一队列 ⇒ 每项只被消费一次，重校验语义不被并发破坏。
func TestP2B3_Dequeue_ConcurrentConsumers(t *testing.T) {
	q := newP2Queue(0) // 默认 64
	for i := 0; i < 50; i++ {
		if !q.push(qReq{model: "A", req: i}) {
			t.Fatalf("入队 %d 应成功（容量 64）", i)
		}
	}
	var mu sync.Mutex
	got := map[interface{}]bool{}
	var wg sync.WaitGroup
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				item, ok, mismatch := q.popDequeue(func() string { return "A" }, false)
				if !ok {
					if mismatch {
						continue // 极小概率的 mismatch（并发队头变更）：重试
					}
					return // 队列空，收工
				}
				mu.Lock()
				got[item.req] = true
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if len(got) != 50 {
		t.Fatalf("50 项应各被消费恰好一次，实得 %d", len(got))
	}
}

// ETA 字段存在且可填充（Q5：排队必须含 ETA——字段与填充口径在这里钉住）。
func TestP2B3_QueueETAField(t *testing.T) {
	q := newP2Queue(4)
	q.push(qReq{model: "A", req: "r1", eta: 42 * time.Second})
	got := q.peekAll()
	if got[0].eta != 42*time.Second {
		t.Fatalf("ETA 字段应如实保存，实得 %v", got[0].eta)
	}
}
