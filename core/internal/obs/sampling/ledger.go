// ledger.go — 判定账与墓碑（C5）。
//
// 要治的盲区：采样一旦生效，"没有那条记录"就有了两种完全不同的含义 ——
// ① 这件事没发生；② 发生了但被裁掉了。不落账的话，事后**无法分辨**，must-keep 召回率也只能靠感觉。
//
// 三条写死的口径：
//
//	① **每 run 一本账**：run id 是分组键（C5"每 run 记"）。账里每行都带
//	   decision / matched_policies / deciding_policy / threshold / weight / degrade_level。
//	② **被裁者留墓碑**（tombstone=true）：墓碑不是"多余的一行"，它是让"缺席"不再歧义的那一行。
//	   dry-run 下 tombstone 恒为 false（没真裁）；判词（sampling.decision）**照记** —— 这正是 dry-run 的用处：
//	   量 must-keep 召回（C4）。
//	③ **append-only，不回头改行**：翻案与"宽限期被提前终结"都**追加**一条 amendment 行
//	   （from_state → to_state），旧行的 `probation_until` 保持不变。墓碑是否终局由读侧比较时间得出，
//	   不由写侧回头涂改 —— 账一旦可改，取证价值就没了。
package sampling

import (
	"encoding/json"
	"sync"
	"time"
)

// Record — 判定账的一行（JSONL 形态）。键名对齐 C5 点名的字段（sampling.* 前缀，与 semconv 的 gen_ai.* 同风格）。
type Record struct {
	RunID     string `json:"run_id"`
	Seq       int    `json:"seq"`
	TS        string `json:"ts"`
	Key       string `json:"key"`
	EventName string `json:"event_name,omitempty"`
	Bytes     int    `json:"bytes,omitempty"`

	Decision        string   `json:"sampling.decision"`                   // keep | drop —— **判词**（dry-run 下照记）
	MatchedPolicies []string `json:"sampling.matched_policies,omitempty"` // 按写死序（含命中但未定案者）
	DecidingPolicy  string   `json:"sampling.deciding_policy"`            // 定案者（可为 default_deny / sampling_off）
	DecidingKind    string   `json:"sampling.deciding_kind"`
	Threshold       *float64 `json:"sampling.threshold,omitempty"` // 判词依据；不适用 ⇒ 缺席（不写 0）
	Weight          *float64 `json:"sampling.weight,omitempty"`
	DegradeLevel    string   `json:"sampling.degrade_level"`
	GateEnforced    bool     `json:"sampling.gate_enforced,omitempty"` // 判词被闸覆盖（deciding_policy 仍是"谁想留它"）

	Tombstone          bool   `json:"tombstone"`                      // true ⇔ 真的缺席
	DryRun             bool   `json:"dry_run,omitempty"`              // true ⇔ 判词只是计算（没真裁）
	OverriddenMustKeep bool   `json:"overridden_must_keep,omitempty"` // C1 的显式否决：**必被记录**
	ProbationUntil     string `json:"probation_until,omitempty"`      // 宽限到此为止（终局判据，读侧比较时间）

	Amendment bool   `json:"sampling.amendment,omitempty"` // 改判行：翻案 / 宽限提前终结
	FromState string `json:"sampling.from_state,omitempty"`
	ToState   string `json:"sampling.to_state,omitempty"`

	Reason string `json:"reason,omitempty"`
}

// 改判行的两个状态名（低基数）。
const (
	StateProbation  = "probation"  // 已判丢但仍在宽限期内（晚到的判词可翻案）
	StateOverturned = "overturned" // 翻案：改为留下
	StateExpired    = "expired"    // 宽限到期，墓碑终局
	StateEvicted    = "evicted"    // 容量淘汰：宽限期被**提前**终结（degrade_level=ring）
)

// Ledger — 一个 run 的判定账（append-only；零值不可用，用 newLedger）。
type Ledger struct {
	mu    sync.Mutex
	runID string
	seq   int
	recs  []Record

	emitted    int // 真的落盘成功的事件条数（"落盘条数"）
	tombstones int
	amendments int
}

func newLedger(runID string) *Ledger { return &Ledger{runID: runID} }

// RunID — 本账所属的 run。
func (l *Ledger) RunID() string {
	if l == nil {
		return ""
	}
	return l.runID
}

// Records — 全部行（判定行 + 改判行），按写入顺序（append-only ⇒ 顺序即因果）。
func (l *Ledger) Records() []Record {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]Record, len(l.recs))
	copy(out, l.recs)
	return out
}

// Count — 判定行条数（不含改判行）。
func (l *Ledger) Count() int {
	if l == nil {
		return 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.recs) - l.amendments
}

// Emitted — **落盘条数**：真的交给 sink 且成功的事件条数（dry-run 下 = 全量，C4 的验收口径）。
func (l *Ledger) Emitted() int {
	if l == nil {
		return 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.emitted
}

// TombstoneCount — 墓碑条数（被裁者的条数）。
func (l *Ledger) TombstoneCount() int {
	if l == nil {
		return 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.tombstones
}

// Amendments — 改判行（翻案 / 宽限提前终结）。
func (l *Ledger) Amendments() []Record {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []Record
	for _, r := range l.recs {
		if r.Amendment {
			out = append(out, r)
		}
	}
	return out
}

// Tombstones — 墓碑行（被裁者的完整取证件）。
func (l *Ledger) Tombstones() []Record {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []Record
	for _, r := range l.recs {
		if r.Tombstone {
			out = append(out, r)
		}
	}
	return out
}

// TombstoneFor — 按事件身份取墓碑（后续批的消费侧按 key 查"它到哪去了"）。
func (l *Ledger) TombstoneFor(key string) (Record, bool) {
	if l == nil {
		return Record{}, false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, r := range l.recs {
		if r.Tombstone && r.Key == key {
			return r, true
		}
	}
	return Record{}, false
}

// JSONL — 整本账的 JSONL 字节（本包**不落盘**：字节交给调用方，唯一写入口在脱敏/chat 那一侧）。
// 编码失败的行会被**跳过而不是伪造** —— 账宁可少一行也不许写一行假的。
func (l *Ledger) JSONL() []byte {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []byte
	for _, r := range l.recs {
		b, err := json.Marshal(r)
		if err != nil {
			continue
		}
		out = append(out, b...)
		out = append(out, '\n')
	}
	return out
}

// nextSeqLocked — 单调序号（账内唯一）：调用方须持 l.mu。
func (l *Ledger) nextSeqLocked() int {
	l.seq++
	return l.seq
}

// recordDecision — 追加一条判定行。
//
// tombstone/emitted/degrade 这些**落地事实**由 Ring 传进来（判定行不猜落地结果）：
// 判词（Decision）与落地（是否真落盘）是两件事，账里也必须分得开。
func (l *Ledger) recordDecision(now time.Time, ev Event, d Decision, emitted, tombstone bool, probationUntil time.Time, degrade string, gateEnforced bool) Record {
	l.mu.Lock()
	defer l.mu.Unlock()
	rec := Record{
		RunID:              l.runID,
		Seq:                l.nextSeqLocked(),
		TS:                 now.Format(time.RFC3339Nano),
		Key:                ev.Key,
		EventName:          ev.Name,
		Bytes:              ev.bytesOf(),
		Decision:           string(d.Verdict),
		MatchedPolicies:    append([]string(nil), d.MatchedPolicies...),
		DecidingPolicy:     d.DecidingPolicy,
		DecidingKind:       string(d.DecidingKind),
		Threshold:          d.Threshold,
		Weight:             d.Weight,
		DegradeLevel:       degrade,
		GateEnforced:       gateEnforced,
		OverriddenMustKeep: d.OverriddenMustKeep,
		Tombstone:          tombstone,
		DryRun:             d.DryRun,
		Reason:             d.Reason,
	}
	if tombstone && !probationUntil.IsZero() {
		rec.ProbationUntil = probationUntil.Format(time.RFC3339Nano)
	}
	if emitted {
		l.emitted++
	}
	if tombstone {
		l.tombstones++
	}
	l.recs = append(l.recs, rec)
	return rec
}

// recordAmendment — 追加一条改判行（翻案或宽限提前终结）。**不改旧行**（append-only）。
// emitted=true ⇔ 这次改判真的把事件交付到了 sink（翻案成功且落盘成功）。
func (l *Ledger) recordAmendment(now time.Time, key, from, to, reason, degrade string, emitted bool) Record {
	l.mu.Lock()
	defer l.mu.Unlock()
	rec := Record{
		RunID:          l.runID,
		Seq:            l.nextSeqLocked(),
		TS:             now.Format(time.RFC3339Nano),
		Key:            key,
		Decision:       string(VerdictKeep), // 改判行的"新状态"落在 to_state；decision 保持可读的"从丢改留"
		DecidingPolicy: PolicyOff,           // 改判不是任何策略命中的结果 ⇒ 不用策略名冒充定案者
		DegradeLevel:   degrade,
		Amendment:      true,
		FromState:      from,
		ToState:        to,
		Reason:         reason,
	}
	if to == StateExpired || to == StateEvicted {
		rec.Decision = string(VerdictDrop) // 提前终结仍然是没有落盘 ⇒ 判词仍是丢
	}
	if emitted {
		l.emitted++
	}
	l.amendments++
	l.recs = append(l.recs, rec)
	return rec
}
