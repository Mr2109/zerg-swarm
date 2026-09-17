// effects.go —— 效果级幂等键与账本（E4/E5/F6）。
//
// ── 为什么幂等要落在"效果"而不是"调用"（E4 ★★★）──
//
// 一次工具调用可能产生**多个效果**（bash 里 `set -e; mkdir a; touch b` 是两个文件系统效果），
// 而恢复/重放往往只走了一半就中断。按"调用"判重只有两种烂选择：整个重跑（前半段效果再来一次）
// 或整个跳过（后半段效果永远不发生）。所以判重键必须细到**效果**，并用 (call_id, effect_index)
// 标明"是哪一次调用的第几个效果"。
//
// ── 口径（写死）──
//
//	effect_key = sha256(工具名 ‖ 0x1f ‖ ArgsCanonical ‖ 0x1f ‖ 效果范围)
//	  · 用**规范化**后的参数（不是原始字节）：同一个逻辑效果必须算出同一个 key。
//	  · "效果范围"由调用方声明（如 "fs:/w/out.txt"、"proc:bash:git commit"）——本包**不猜**，
//	    因为"猜这个工具会不会写盘"猜错的方向正好是危险的那一侧。
//
//	先认领后执行（F6）：Claim 在**执行之前**把 key 记进账本。因此崩溃语义是：
//	  · 跨中断（进程内、有账本）⇒ exactly-once：第二次 Claim 返回"已发生"，不重放；
//	  · 跨崩溃（认领之后、执行之前挂掉）⇒ **at-least-once**：账本里已有 key，重启后不再执行，
//	    即"可能没发生，但系统当它发生了"。这条边界是设计稿 F6 的原话，如实写在这里。
//
//	去重存储分层（E4 原话：内存 LRU → 本地 KV → 服务端唯一约束）：本批只做**内存 + 可序列化**
//	  两档（序列化出来落盘就是"本地 KV"的地基）；服务端唯一约束不在本批。
package replay

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// EffectKey —— 效果级幂等键：sha256(工具名 + ArgsCanonical + 效果范围)。
func EffectKey(tool string, canonical []byte, scope string) string {
	h := sha256.New()
	h.Write([]byte(tool))
	h.Write([]byte{0x1f})
	h.Write(canonical)
	h.Write([]byte{0x1f})
	h.Write([]byte(scope))
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

// EffectClaim —— 认领结果。
type EffectClaim string

const (
	// EffectNew —— 首次出现 ⇒ 调用方可以执行。
	EffectNew EffectClaim = "new"
	// EffectAlreadyApplied —— 已发生过 ⇒ **不得重放**（也不得报成错误：这正是恢复路径的正常分支）。
	EffectAlreadyApplied EffectClaim = "already_applied"
)

// EffectEntry —— 账本里的一条事实。
//
// 前后置状态哈希（E5）：
//
//	· PreconditionStateHash —— **执行前**的状态指纹（回放/补做前校验它，不符即报"状态不匹配"）；
//	· ObservedAfterHash     —— **执行后**观测到的状态指纹（证明"这次执行确实把状态推到了这里"）。
//
// 两者都是**不透明字符串**：本包不规定状态怎么哈希（那是调用方的事，比如对产物目录取 sha256），
// 只规定"必须原样记住、比对必须 fail-closed"。
type EffectEntry struct {
	Key    string `json:"key"`   // effect_key
	Tool   string `json:"tool"`  // 工具名
	Scope  string `json:"scope"` // 效果范围
	CallID string `json:"call_id,omitempty"`
	Index  int    `json:"effect_index"`   // 本调用内的效果序号（0 起）
	Mode   Mode   `json:"mode,omitempty"` // 首次发生所在的路径（live/record/replay）
	// PreconditionStateHash —— 首次发生时（执行前）的状态指纹。空 = 当时没记（**不是**"没有前置状态"）。
	PreconditionStateHash string `json:"precondition_state_hash,omitempty"`
	// ObservedAfterHash —— 首次发生时（执行后）观测到的状态指纹。空 = 还没记到（执行未收尾）。
	ObservedAfterHash string `json:"observed_after_hash,omitempty"`
}

// String —— 便于日志/错误里指认是哪一条效果。
func (e EffectEntry) String() string {
	return fmt.Sprintf("%s(tool=%s scope=%s call_id=%s idx=%d)", e.Key, e.Tool, e.Scope, e.CallID, e.Index)
}

// EffectLedger —— 效果账本（内存 + 可序列化）。
type EffectLedger struct {
	mu    sync.Mutex
	seen  map[string]EffectEntry // effect_key → 首次发生的条目
	index map[string]string      // call_id ‖ 0x1f ‖ effect_index → effect_key
}

// NewEffectLedger —— 空账本。
func NewEffectLedger() *EffectLedger {
	return &EffectLedger{seen: map[string]EffectEntry{}, index: map[string]string{}}
}

func idxKey(callID string, index int) string {
	return callID + "\x1f" + strconv.Itoa(index)
}

// Claim —— 认领一个效果（先认领后执行）。
//
// 首次 ⇒ (EffectNew, entry)；再次 ⇒ (EffectAlreadyApplied, **首次那条** entry)。
// 已存在的条目**不会被后到者覆盖**：账本记的是"这件事第一次发生在哪"，后到者改不动它。
func (l *EffectLedger) Claim(e EffectEntry) (EffectClaim, EffectEntry) {
	if e.Key == "" {
		// 空 key ⇒ 判重不成立。宁可在这里炸，也不要在恢复路径上"看起来去过重了"。
		e.Key = EffectKey(e.Tool, nil, e.Scope)
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if first, ok := l.seen[e.Key]; ok {
		return EffectAlreadyApplied, first
	}
	l.seen[e.Key] = e
	if e.CallID != "" {
		l.index[idxKey(e.CallID, e.Index)] = e.Key
	}
	return EffectNew, e
}

// Applied —— 该 effect_key 是否已发生过（只读；回放路径只用它）。
func (l *EffectLedger) Applied(key string) (EffectEntry, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	e, ok := l.seen[key]
	return e, ok
}

// ErrEffectLedger —— 账本相关错误（key 不认识、状态哈希记不上、撤销不存在的东西……）。
var ErrEffectLedger = errors.New("replay: 效果账本不可用")

// ClaimChecked —— **认领 + 前置状态校验**（E5；先认领后执行的那一半语义）。
//
//	· 首次出现 ⇒ 记下 currentStateHash 作为 precondition_state_hash，返回 (EffectNew, 条目, nil)；
//	· 已发生过 ⇒ 校验"录制时的前置状态"与 currentStateHash：
//	     相符   ⇒ (EffectAlreadyApplied, **首次那条**条目, nil) —— 恢复正常分支，不是错误；
//	     不符 / 一方为空 ⇒ 返回 *StateMismatchError（**不是** already_applied！）
//
// 为什么不符不能退化成"已发生，跳过"：那样回放会**在错误的状态上**安静地跳过一个效果，
// 报告上写着"幂等通过"，而实际状态与 trace 记录的状态早已不是一回事 —— 这正是 E5 要拦的病。
// 三个方向都不许静默：录了前置而本次不同 / 录了前置而本次没给 / 没录前置而本次给了。
func (l *EffectLedger) ClaimChecked(e EffectEntry, currentStateHash string) (EffectClaim, EffectEntry, error) {
	if e.Key == "" {
		e.Key = EffectKey(e.Tool, nil, e.Scope)
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	first, ok := l.seen[e.Key]
	if !ok {
		e.PreconditionStateHash = currentStateHash
		l.seen[e.Key] = e
		if e.CallID != "" {
			l.index[idxKey(e.CallID, e.Index)] = e.Key
		}
		return EffectNew, e, nil
	}
	if err := stateMismatchOf(first, currentStateHash); err != nil {
		return EffectClaim(""), first, err
	}
	return EffectAlreadyApplied, first, nil
}

// VerifyState —— 对**已发生**的效果做前置状态校验（回放/补做前的只读检查）。
//
// key 不在账本里 ⇒ ErrEffectLedger（"没记录"不等于"状态对得上"）；状态不符 ⇒ *StateMismatchError。
func (l *EffectLedger) VerifyState(key, currentStateHash string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	e, ok := l.seen[key]
	if !ok {
		return fmt.Errorf("%w: effect_key %s 不在账本里（没有记录就无法校验状态；拒绝按「大概没发生过」放行）",
			ErrEffectLedger, key)
	}
	return stateMismatchOf(e, currentStateHash)
}

// Complete —— 记录**执行后**观测到的状态哈希（E5 的 observed_after_hash）。
//
//	· key 不在账本里 ⇒ ErrEffectLedger（执行了一个账本里没有的效果 = 认领与执行脱节）；
//	· 空哈希 ⇒ 报错（"没观测到"与"观测到空"是两件事，不许用空串糊过去）；
//	· 已有 after 且**不同** ⇒ 报错：同一个效果出现两个"执行后状态" = 它被**重做**过
//	  （这正是幂等要抓的事，必须可见，不能覆盖成最后一次）；
//	· 已有 after 且相同 ⇒ 幂等 no-op（重放同一份事实是允许的）。
func (l *EffectLedger) Complete(key, observedAfterHash string) error {
	if strings.TrimSpace(observedAfterHash) == "" {
		return fmt.Errorf("%w: 效果 %s 的 observed_after_hash 为空（无法证明执行后状态，拒绝记为已收尾）",
			ErrEffectLedger, key)
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	e, ok := l.seen[key]
	if !ok {
		return fmt.Errorf("%w: 效果 %s 未在账本里认领过（执行与认领脱节）", ErrEffectLedger, key)
	}
	if e.ObservedAfterHash != "" && e.ObservedAfterHash != observedAfterHash {
		return fmt.Errorf("%w: 效果 %s 已有 observed_after_hash=%s，本次为 %s —— 同一效果被重做过（拒绝覆盖）",
			ErrEffectLedger, key, e.ObservedAfterHash, observedAfterHash)
	}
	e.ObservedAfterHash = observedAfterHash
	l.seen[key] = e
	return nil
}

// Release —— **显式放弃**一次认领（让这条效果重新变成"没发生过"）。
//
// 只有在**确认副作用确实没有发生**之后才许调用（例如目的文件不存在、通知服务端查无此条）。
// 调用它就是把"重做"放了出来：确认错了 ⇒ 副作用来第二遍（重复通知 / 覆盖别人改过的文件）。
// 已记了 observed_after_hash 的条目**撤不动**（那说明它确实执行过）—— 撤销会返回错误而不是静默生效。
//
// 谁会用到它：ClaimChecked 之后执行失败 ⇒ 认领保留（保守一侧，见 resume.go），
// 这条"已认领、未收尾"会出现在 Unfinished() 里；人工核实"确实没发生"之后 Release，下次补做才会重试。
func (l *EffectLedger) Release(key string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	e, ok := l.seen[key]
	if !ok {
		return fmt.Errorf("%w: 释放认领时账本里没有 %s", ErrEffectLedger, key)
	}
	if e.ObservedAfterHash != "" {
		return fmt.Errorf("%w: 效果 %s 已记下执行后状态，不能释放（它确实执行过）", ErrEffectLedger, key)
	}
	delete(l.seen, key)
	if e.CallID != "" {
		delete(l.index, idxKey(e.CallID, e.Index))
	}
	return nil
}

// Unfinished —— **已认领、但没记下执行后状态**的条目（稳定序：按 key 排序）。
//
// 它们是"可疑条目"：执行可能只做了一半（认领了、没等到收尾就挂了/报错了）。
// 这条清单存在的意义就是**让这种状态可见**：不做成静默的"当它已完成"，
// 也不自动重做（重做会重复副作用）—— 由上层核实后走 Release（放行重做）或人工补偿。
//
// 实现上走 Snapshot()（已是稳定序的切片）：既省一遍 iterating map，也让"对外输出先排序"
// 这条纪律只有一处实现。
func (l *EffectLedger) Unfinished() []EffectEntry {
	snap := l.Snapshot()
	out := make([]EffectEntry, 0, len(snap))
	for _, e := range snap {
		if e.ObservedAfterHash == "" {
			out = append(out, e)
		}
	}
	return out
}

// stateMismatchOf —— 前置状态比对（fail-closed 三分支）：两边都空 ⇒ 通过（双方都没记/没给）。
func stateMismatchOf(e EffectEntry, currentStateHash string) error {
	switch {
	case e.PreconditionStateHash == "" && currentStateHash == "":
		return nil
	case e.PreconditionStateHash == "":
		return &StateMismatchError{
			Tool: e.Tool, Want: "", Got: currentStateHash, Fingerprint: e.Key,
			Effect: e.Key, Scope: e.Scope, CallID: e.CallID, Index: e.Index, IsEffect: true,
			Reason: "首次发生时未记前置状态（无法校验）",
		}
	case currentStateHash == "":
		return &StateMismatchError{
			Tool: e.Tool, Want: e.PreconditionStateHash, Got: "", Fingerprint: e.Key,
			Effect: e.Key, Scope: e.Scope, CallID: e.CallID, Index: e.Index, IsEffect: true,
			Reason: "调用方未提供当前状态指纹",
		}
	case e.PreconditionStateHash != currentStateHash:
		return &StateMismatchError{
			Tool: e.Tool, Want: e.PreconditionStateHash, Got: currentStateHash, Fingerprint: e.Key,
			Effect: e.Key, Scope: e.Scope, CallID: e.CallID, Index: e.Index, IsEffect: true,
			Reason: "前置状态指纹不符",
		}
	}
	return nil
}

// AppliedIndex —— 该 (call_id, effect_index) 是否已发生过（**逐效果**判重的读侧）。
func (l *EffectLedger) AppliedIndex(callID string, index int) (EffectEntry, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	key, ok := l.index[idxKey(callID, index)]
	if !ok {
		return EffectEntry{}, false
	}
	e, ok := l.seen[key]
	return e, ok
}

// Len —— 账本条数。
func (l *EffectLedger) Len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.seen)
}

// Snapshot —— 稳定序（按 key 排序）的条目快照：序列化与断言都靠它，不依赖 map 迭代序。
func (l *EffectLedger) Snapshot() []EffectEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]EffectEntry, 0, len(l.seen))
	for _, e := range l.seen {
		out = append(out, e)
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Key < out[b].Key })
	return out
}

type ledgerWire struct {
	V       int           `json:"v"`
	Effects []EffectEntry `json:"effects"`
}

// MarshalLedger —— 序列化（稳定序 ⇒ 同样的账本必然同样字节；落盘/比对都靠这一点）。
func (l *EffectLedger) MarshalLedger() ([]byte, error) {
	b, err := json.Marshal(ledgerWire{V: SchemaVersion, Effects: l.Snapshot()})
	if err != nil {
		return nil, fmt.Errorf("replay: 序列化效果账本：%w", err)
	}
	return b, nil
}

// LoadEffectLedger —— 反序列化。**严格**：版本不认识、key 为空、重复 key 都报错
// （账本坏掉 = 幂等判断全错，属"宁可起不来"的那一类）。
func LoadEffectLedger(b []byte) (*EffectLedger, error) {
	var w ledgerWire
	if err := json.Unmarshal(b, &w); err != nil {
		return nil, fmt.Errorf("replay: 效果账本不是合法 JSON：%w", err)
	}
	if w.V != 0 && w.V != SchemaVersion {
		return nil, fmt.Errorf("replay: 效果账本 v=%d 高于本包已知 v=%d（拒绝猜读）", w.V, SchemaVersion)
	}
	l := NewEffectLedger()
	for i, e := range w.Effects {
		if strings.TrimSpace(e.Key) == "" {
			return nil, fmt.Errorf("replay: 效果账本第 %d 条 key 为空", i)
		}
		if _, dup := l.seen[e.Key]; dup {
			return nil, fmt.Errorf("replay: 效果账本第 %d 条 key 重复（%s）", i, e.Key)
		}
		l.seen[e.Key] = e
		if e.CallID != "" {
			l.index[idxKey(e.CallID, e.Index)] = e.Key
		}
	}
	return l, nil
}
