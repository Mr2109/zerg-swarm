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
type EffectEntry struct {
	Key    string `json:"key"`   // effect_key
	Tool   string `json:"tool"`  // 工具名
	Scope  string `json:"scope"` // 效果范围
	CallID string `json:"call_id,omitempty"`
	Index  int    `json:"effect_index"`   // 本调用内的效果序号（0 起）
	Mode   Mode   `json:"mode,omitempty"` // 首次发生所在的路径（live/record/replay）
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
