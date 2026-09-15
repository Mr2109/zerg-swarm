// waitqueue.go —— P7 批 2/3：等待队列的对外（包外）出口。
//
// 设计真源：设计-子端沙箱化-20260914.md §7.7 修补 4（出队必须重校验当前卵）·
// §5.3（队列满 ⇒ 立即 429）· §13 Q5（排队 + ETA，队列限长）。
//
// 为什么单独一个出口文件：队列本体（p2Queue / qReq）留在 p2_lifecycle.go 里**不导出**，
// 由本文件只暴露「入队 / 出队（带重校验）/ 观测」三件，避免调用方绕过重校验直接搬运。
//
// ⚠ 锁序不变式（必须遵守，否则死锁）：
//   - 队列自己的 q.mu 与 Manager 的 m.mu 是两把锁；
//   - 出队重校验的回调 **在持 q.mu 时调用**，而回调（如 CurrentModel）会取 m.mu
//     ⇒ 合法顺序是 **q.mu → m.mu**；
//   - 因此：**任何持有 m.mu 的代码路径都不得调用本文件的任何 WaitQ* 方法**
//     （waitQueue() 内部只做惰性创建，创建过程不再取 q.mu，故不构成反向嵌套）。
package backend

import "time"

// WaitItem 等待队列项的只读视图（观测面用：不含请求体）。
type WaitItem struct {
	Model string        // 它要的模型（入队时快照）
	ETA   time.Duration // 预计还要等多久（入队时估算；0 = 未知，不编造）
}

// waitQueue 惰性创建并返回队列本体。只做创建，不取 q.mu（见文件头锁序不变式）。
// 手写构造的 Manager（测试里 &Manager{...}）没有初始化 waitQ，靠这里补上。
func (m *Manager) waitQueue() *p2Queue {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.waitQ == nil {
		m.waitQ = newP2Queue(p2QueueCapacity)
	}
	return m.waitQ
}

// WaitQPush 入队（返回 false = 队列已满 ⇒ 调用方必须立即回 429，不挂起、不排队）。
func (m *Manager) WaitQPush(model string, payload interface{}, eta time.Duration) bool {
	return m.waitQueue().push(qReq{model: model, req: payload, eta: eta})
}

// WaitQPop 出队 + 重校验（§7.7 修补 4）。
//   - ok=true            ⇒ 正常取到（该请求可以转发）；
//   - mismatch=true      ⇒ 队头要的模型不是当前卵 ⇒ **未摘除**，调用方必须先重走孵化流程
//     再回到本方法（否则该项会被饿死；也不得直接转发）；
//   - ok=false,mismatch=false ⇒ 队列空（wait=false 时立即返回）。
//
// current 回调用于读「当前卵是哪枚」，会在持 q.mu 时被调用 ⇒ 只能是只读且不阻塞的读（见文件头）。
func (m *Manager) WaitQPop(current func() string, wait bool) (WaitItem, interface{}, bool, bool) {
	item, ok, mismatch := m.waitQueue().popDequeue(current, wait)
	if mismatch {
		// 队伍头未摘除：仍把队头信息回报给上层（它据此知道"谁挡着"并重走孵化）。
		return WaitItem{Model: item.model, ETA: item.eta}, item.req, false, true
	}
	if !ok {
		return WaitItem{}, nil, false, false
	}
	return WaitItem{Model: item.model, ETA: item.eta}, item.req, true, false
}

// WaitQLen 当前队列长度（观测面/背压指标）。
func (m *Manager) WaitQLen() int { return m.waitQueue().lenOf() }

// WaitQPeek 队列快照（不出队；观测面 SSE 的排队深度与 ETA 来源）。
func (m *Manager) WaitQPeek() []WaitItem {
	items := m.waitQueue().peekAll()
	out := make([]WaitItem, 0, len(items))
	for _, it := range items {
		out = append(out, WaitItem{Model: it.model, ETA: it.eta})
	}
	return out
}

// WaitQHeadETA 队头预计等待（队空 ⇒ 0,false；ETA 未知 ⇒ 0,true——不编造数字）。
func (m *Manager) WaitQHeadETA() (float64, bool) {
	items := m.waitQueue().peekAll()
	if len(items) == 0 {
		return 0, false
	}
	return items[0].eta.Seconds(), true
}

// WaitQDropHead 摘除队头并返回它（ok=false = 队空）。
// 用途（§7.7 修补 4 的兜底）：worker 重走孵化后仍不匹配 ⇒ 该模型装不起来，
// 必须把这一项摘掉并回报失败，否则队头会永远卡住（饿死后面所有项）或热旋重试。
func (m *Manager) WaitQDropHead() (WaitItem, interface{}, bool) {
	item, ok := m.waitQueue().dropHead()
	if !ok {
		return WaitItem{}, nil, false
	}
	return WaitItem{Model: item.model, ETA: item.eta}, item.req, true
}

// WaitQueueCapacityForTest 默认队列限长（供包外测试构造"队列满"场景；产品路径不读它）。
func WaitQueueCapacityForTest() int { return p2QueueCapacity }
