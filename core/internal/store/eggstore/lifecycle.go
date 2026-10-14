package eggstore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ─────────────────────────── 八态闭集（S2）───────────────────────────

// Lifecycle 卵的生命周期状态 —— **八态闭集**。
//
// 闭集纪律（用户拍定）：状态值是**闭集**，闭集外的值即错（`ParseLifecycle` 拒、
// 名册 `Upsert` 拒、审计 `Append` 拒）—— ✗ 不回落默认值、✗ 不静默当未知态放过。
type Lifecycle string

const (
	// StateLanding 写出中（只存在于**临时名**下；终名上永不该看到它）。
	StateLanding Lifecycle = "landing"
	// StateLanded 写全未校验（终名已就位，逐块 sha256 尚未全过）。
	StateLanded Lifecycle = "landed"
	// StateVerified 逐块 sha256 全过 —— ★ **只有此态可孵**。
	StateVerified Lifecycle = "verified"
	// StateHatching 正被孵化 / 被服务（取锁）。
	StateHatching Lifecycle = "hatching"
	// StateIdle 空窗（孵过、当前无实例；仍可再孵）。
	StateIdle Lifecycle = "idle"
	// StateReclaimable 可回收（引用计数已归零 ⇒ 允许真删）。
	StateReclaimable Lifecycle = "reclaimable"
	// StateDeleted 已真删（名册里只留墓碑行；引用计数恒 0）。
	StateDeleted Lifecycle = "deleted"
	// StateBroken 任一态检出内容不符 ⇒ 只可删或重取（✗ 不可孵）。
	StateBroken Lifecycle = "broken"
)

// LifecycleStates 八态闭集（顺序＝设计稿 §16.2 的枚举顺序，供人面/机器面稳定输出）。
var LifecycleStates = []Lifecycle{
	StateLanding, StateLanded, StateVerified, StateHatching,
	StateIdle, StateReclaimable, StateDeleted, StateBroken,
}

// Valid 值在闭集内？
func (s Lifecycle) Valid() bool {
	for _, x := range LifecycleStates {
		if s == x {
			return true
		}
	}
	return false
}

// Hatchable ★ 只有 `verified` 可孵（唯一放行判据）。
func (s Lifecycle) Hatchable() bool { return s == StateVerified }

// GCReclaimable 允许被 GC 真删的态（其余态一律钉住 —— 见 Store.GC）。
func (s Lifecycle) GCReclaimable() bool {
	return s == StateReclaimable || s == StateBroken
}

// StateNames 八态名（人面一列 / 报错里列闭集用）。
func StateNames() []string {
	out := make([]string, 0, len(LifecycleStates))
	for _, s := range LifecycleStates {
		out = append(out, string(s))
	}
	return out
}

// ParseLifecycle 解析外部输入：闭集外 ⇒ 错（非法值即错）。
//
// ★ **不做 trim、不做大小写归一** —— 闭集就是闭集，`" verified"` / `"VERIFIED"` 一律拒。
// 外部输入（YAML / JSON / argv）该由调用方先规范好；✗ 不让本函数替它猜、替它纠正
// （「静默按错口径读」比「读不出来」糟，同 `agent/internal/registry/egg_decl.go` 的口径）。
func ParseLifecycle(v string) (Lifecycle, error) {
	s := Lifecycle(v)
	if !s.Valid() {
		return "", fmt.Errorf("未知生命周期状态 %q —— 取值只许逐字落在八态闭集内（✗ 不 trim、✗ 不大小写归一）：%s",
			v, strings.Join(StateNames(), " / "))
	}
	return s, nil
}

// lifecycleEdges 合法转移表（状态机本体）。
//
// 口径：每条边都是「设计稿点到的那条路」；未列出的转移一律非法 —— 尤其
// `deleted → *`（墓碑是终态）与 `broken → verified`（坏件不许直接变好人）。
var lifecycleEdges = map[Lifecycle][]Lifecycle{
	StateLanding:     {StateLanded, StateBroken},
	StateLanded:      {StateVerified, StateBroken, StateDeleted},
	StateVerified:    {StateHatching, StateIdle, StateReclaimable, StateBroken, StateDeleted},
	StateHatching:    {StateIdle, StateVerified, StateBroken},
	StateIdle:        {StateHatching, StateVerified, StateReclaimable, StateBroken, StateDeleted},
	StateReclaimable: {StateDeleted, StateVerified, StateBroken},
	StateBroken:      {StateDeleted, StateLanding}, // 重取 = 回 landing 重写
	StateDeleted:     {},
}

// CanTransitionTo 该转移合法？
func (s Lifecycle) CanTransitionTo(to Lifecycle) bool {
	if !s.Valid() || !to.Valid() {
		return false
	}
	for _, x := range lifecycleEdges[s] {
		if x == to {
			return true
		}
	}
	return false
}

// RefuseError 拒孵（或拒动）—— 一律**出声**，✗ 不静默降级。
type RefuseError struct {
	Egg    string
	State  Lifecycle
	Reason string
}

func (e *RefuseError) Error() string {
	if e.Egg == "" {
		return "拒孵：" + e.Reason
	}
	if e.State == "" {
		return fmt.Sprintf("拒孵：卵 %s —— %s", e.Egg, e.Reason)
	}
	return fmt.Sprintf("拒孵：卵 %s 当前态 %s —— %s", e.Egg, e.State, e.Reason)
}

// CheckHatchable —— **孵化入口的唯一放行闸**（S2 的牙）。
func CheckHatchable(egg string, s Lifecycle) error {
	if !s.Valid() {
		return &RefuseError{Egg: egg, Reason: fmt.Sprintf("状态 %q 不在八态闭集内（非法值即错）", string(s))}
	}
	if !s.Hatchable() {
		return &RefuseError{Egg: egg, State: s, Reason: "非 verified 不可孵（只有 verified 允许进孵化入口）"}
	}
	return nil
}

// ─────────────────────────── 状态变更的可审计记录（S2）───────────────────────────

// AuditFileName 生命周期审计流水（append-only JSONL，一行一事件）。
const AuditFileName = "egg-lifecycle-audit.jsonl"

// ChangeRecord 一次状态变更的可审计记录。
type ChangeRecord struct {
	Egg    string    `json:"egg"`
	From   Lifecycle `json:"from"`
	To     Lifecycle `json:"to"`
	At     time.Time `json:"at"`
	By     string    `json:"by,omitempty"`
	Reason string    `json:"reason,omitempty"`
	// Refs 变更时刻的**引用计数快照**（删/回收那条路上这是关键证据）。
	Refs   int    `json:"refs"`
	SHA256 string `json:"sha256,omitempty"`
}

// AuditLog append-only 审计流水。
//
// 为什么不是「锁文件」：见 `core/internal/compat/compat.json` 的 `excluded` 里那条
// `llama-local.lock`「0 字节 —— 不是状态，不携带数据」—— 本件**携带数据**、是用来
// 事后对账的**记录**，与运行态互斥件不是一回事。
type AuditLog struct {
	path string
	mu   sync.Mutex
}

// NewAuditLog 指定审计件路径。
func NewAuditLog(path string) *AuditLog { return &AuditLog{path: path} }

// Path 审计件路径。
func (l *AuditLog) Path() string { return l.path }

// Append 落一条（O_APPEND + fsync；写不进 ⇒ 报错，调用方必须当成「这次状态变更没留痕」）。
func (l *AuditLog) Append(rec ChangeRecord) error {
	if !rec.To.Valid() {
		return fmt.Errorf("审计拒收：目标状态 %q 不在八态闭集内", string(rec.To))
	}
	if rec.From != "" && !rec.From.Valid() {
		return fmt.Errorf("审计拒收：起始状态 %q 不在八态闭集内", string(rec.From))
	}
	if strings.TrimSpace(rec.Egg) == "" {
		return fmt.Errorf("审计拒收：卵名是空的")
	}
	if rec.At.IsZero() {
		rec.At = time.Now()
	}
	b, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(l.path), 0o755); err != nil {
		return err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(b, '\n')); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// ReadAll 读回全部审计行（坏行 ⇒ 报错出声，✗ 不静默跳过）。
func (l *AuditLog) ReadAll() ([]ChangeRecord, error) {
	b, err := os.ReadFile(l.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []ChangeRecord
	for i, line := range strings.Split(string(b), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var rec ChangeRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			return out, fmt.Errorf("审计件第 %d 行不可解析：%w", i+1, err)
		}
		out = append(out, rec)
	}
	return out, nil
}

// Count 审计行数。
func (l *AuditLog) Count() (int, error) {
	recs, err := l.ReadAll()
	if err != nil {
		return 0, err
	}
	return len(recs), nil
}
