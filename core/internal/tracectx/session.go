// session.go —— 进程内「会话 → 上游 trace」绑定（T1.6 用来把两侧拼在同一条链上的那张小表）。
//
// 解决什么：一次入站请求可能**上游没给 traceparent** ⇒ 我们生成 root 并（可选）沿用；
// 而这次请求往下转发（出站到子端）时需要一个**与本请求一致**的 trace —— 若出站点自己再生成一条新 root，
// 就会出现"入站事件一条 trace、出站事件另一条 trace"，同一件事被劈成两条链 ⇒ 反而更查不清。
// 因此入站采纳（或生成）后在此登记；出站按会话取回 ⇒ 一次调用从头到尾同一条 trace。
//
// 为什么不放在 chat 包：gateway 与 chat 刻意不互相 import（见 gateway/obs_failover.go 文件头），
// 而两边都要读写这张表 ⇒ 只能落在双方都能 import 的叶子包（本包）。
//
// 纪律：
//   - 只读写这一张有界表，**不改变任何请求语义**（拿不到绑定 ⇒ 调用方自己生成 root，行为与没有它一致）。
//   - 表满淘汰"最久未出现"的会话（与 chat 的 obsTraceTab 同口径 4096）——长跑守护进程里会话数无界。
package tracectx

import "sync"

// sessionTraceMax —— 表容量上限（与 core/internal/chat 的 obsTraceSessionsMax 同口径）。
const sessionTraceMax = 4096

type sessionBind struct {
	trace TraceContext
	seq   int64 // LRU 序号（淘汰最久未出现者）
}

var (
	bindMu  sync.Mutex
	bindSeq int64
	binds   = map[string]*sessionBind{}
)

// BindSession —— 把一个**上游（或本侧新生成）的 trace** 钉到会话上。
// 只接受合法上下文（非法/空会话 ⇒ false，不登记——绝不把垃圾绑上去）。
// 覆盖语义：**后到者为准**（每次入站采纳的结果都是"这一次请求的事实"，出站必须与它一致；
// 若改成"首次为准"，会出现"本次入站事件一条 trace、本次出站另一条"的自相矛盾）。
func BindSession(session string, t TraceContext) bool {
	if session == "" || t.Validate() != nil {
		return false
	}
	bindMu.Lock()
	defer bindMu.Unlock()
	bindSeq++
	if b := binds[session]; b != nil {
		b.trace, b.seq = t, bindSeq
		return true
	}
	if len(binds) >= sessionTraceMax {
		evictOldestBindLocked()
	}
	binds[session] = &sessionBind{trace: t, seq: bindSeq}
	return true
}

// BindSessionTraceID —— 只给 trace-id 的绑定（跨包用，见下）。
//
// 为什么需要它：chat 侧的轮次 span 是 **32 位**内部 id（T1.1 口径），而线上 span 只有 **16 位** ——
// 两者不是同一个 id 空间，禁止截断/填充去凑。所以这里由本包**新开一个线上 span**（只把 trace-id 对齐），
// 出站各跳再各自开自己的线上 span。效果：chat 的轮次事件、出站到网关那一跳、网关再转给子端那一跳
// **全部同一条 trace-id**，而各跳的 span 各自独立（不混两个 id 空间）。
//
// 返回 false = trace-id 非法（不是 32 位小写十六进制 / 全零）或会话为空 ⇒ 不登记。
func BindSessionTraceID(session, traceID string) bool {
	if session == "" {
		return false
	}
	if err := checkLowerHex(traceID, TraceIDHexLen, "trace-id"); err != nil {
		return false
	}
	sid, err := newID(SpanIDHexLen / 2)
	if err != nil {
		return false
	}
	return BindSession(session, TraceContext{TraceID: traceID, SpanID: sid, Flags: FlagSampled})
}

// SessionTrace —— 取会话当前绑定的 trace（ok=false = 没绑过 ⇒ 调用方自己生成 root）。
func SessionTrace(session string) (TraceContext, bool) {
	if session == "" {
		return TraceContext{}, false
	}
	bindMu.Lock()
	defer bindMu.Unlock()
	b := binds[session]
	if b == nil {
		return TraceContext{}, false
	}
	bindSeq++
	b.seq = bindSeq // 命中即刷新（LRU）
	return b.trace, true
}

// evictOldestBindLocked —— 表满时淘汰最久未出现者。调用方须持 bindMu。
func evictOldestBindLocked() {
	var oldestKey string
	var oldestSeq int64 = -1
	for k, v := range binds {
		if oldestSeq < 0 || v.seq < oldestSeq {
			oldestKey, oldestSeq = k, v.seq
		}
	}
	if oldestKey != "" {
		delete(binds, oldestKey)
	}
}

// resetSessionBindings —— 仅供本包用例隔离（跨包用例请用互不相同的会话名）。
func resetSessionBindings() {
	bindMu.Lock()
	defer bindMu.Unlock()
	binds = map[string]*sessionBind{}
	bindSeq = 0
}
