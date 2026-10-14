package eggstore

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

// RosterFileName 卵名册件（运行态；原子落盘）。
const RosterFileName = "egg-roster.json"

// Egg 名册里的一枚卵。
type Egg struct {
	Name   string    `json:"name"`
	SHA256 string    `json:"sha256"`
	Path   string    `json:"path"`
	State  Lifecycle `json:"state"`
	// Bytes 卵文件**名义字节** —— ★ 只作登记，✗ **不作「已释放」读数**
	//（克隆/展开物共享物理块时，名义字节与实际释放量无关；报数只认 df）。
	Bytes int64 `json:"bytes"`
	// Refs 引用计数快照（**真源**在 RefGraph；此格供人面直读、并进审计行）。
	Refs int `json:"refs"`
	// Complete 完工标记**状态** —— ★✗ 不是「目录/文件存在」。
	Complete bool `json:"complete"`
	// Blocks 已通过逐块校验的块数（0 = 未校验）。
	Blocks    int       `json:"blocks"`
	AddedAt   time.Time `json:"added_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Validate 名册行的形状闸（非法值即错；半成品**不进名册**由调用方在落地上游保证）。
func (e Egg) Validate() error {
	if strings.TrimSpace(e.Name) == "" {
		return fmt.Errorf("名册拒收：卵名是空的")
	}
	if strings.TrimSpace(e.SHA256) == "" {
		return fmt.Errorf("名册拒收：卵 %s 缺 sha256（内容寻址的身份缺不得）", e.Name)
	}
	if !e.State.Valid() {
		return fmt.Errorf("名册拒收：卵 %s 的状态 %q 不在八态闭集内", e.Name, string(e.State))
	}
	if e.Bytes < 0 || e.Refs < 0 || e.Blocks < 0 {
		return fmt.Errorf("名册拒收：卵 %s 的字节/引用/块数不许为负", e.Name)
	}
	return nil
}

// Roster 卵名册（运行态真源；原子落盘到 JSON）。
type Roster struct {
	path string
	mu   sync.Mutex
	eggs map[string]*Egg
}

// OpenRoster 打开（不存在 ⇒ 空册，✗ 不报错、✗ 也不创建 —— 创建只发生在写入路径）。
func OpenRoster(path string) (*Roster, error) {
	r := &Roster{path: path, eggs: map[string]*Egg{}}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return r, nil
		}
		return nil, err
	}
	var rows []Egg
	if err := json.Unmarshal(b, &rows); err != nil {
		return nil, fmt.Errorf("名册件不可解析（%s）：%w", path, err)
	}
	for i := range rows {
		e := rows[i]
		if err := e.Validate(); err != nil {
			return nil, fmt.Errorf("名册件（%s）第 %d 行：%w", path, i+1, err)
		}
		r.eggs[e.Name] = &e
	}
	return r, nil
}

// Path 名册件路径。
func (r *Roster) Path() string { return r.path }

// Count 册内条数。
func (r *Roster) Count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.eggs)
}

// Get 取一枚（副本）。
func (r *Roster) Get(name string) (Egg, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.eggs[name]
	if !ok {
		return Egg{}, false
	}
	return *e, true
}

// List 全表（按卵名排序 ⇒ 同状态多次读逐字相同，便于对拍）。
func (r *Roster) List() []Egg {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Egg, 0, len(r.eggs))
	for _, e := range r.eggs {
		out = append(out, *e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Upsert 写入/更新一行（先过形状闸；非法状态 ⇒ 拒，✗ 不落盘）。
func (r *Roster) Upsert(e Egg) error {
	if err := e.Validate(); err != nil {
		return err
	}
	r.mu.Lock()
	now := time.Now()
	if e.AddedAt.IsZero() {
		if old, ok := r.eggs[e.Name]; ok {
			e.AddedAt = old.AddedAt
		} else {
			e.AddedAt = now
		}
	}
	e.UpdatedAt = now
	cp := e
	r.eggs[e.Name] = &cp
	err := r.saveLocked()
	r.mu.Unlock()
	return err
}

// Remove 摘一行（真删之后的墓碑清理；调用方通常改 `deleted` 态而**不**摘行）。
func (r *Roster) Remove(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.eggs[name]; !ok {
		return nil
	}
	delete(r.eggs, name)
	return r.saveLocked()
}

// SetState 改态（过合法转移闸；非法 ⇒ 拒且不动盘）。
func (r *Roster) SetState(name string, to Lifecycle, refs int, now time.Time) (Egg, Egg, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cur, ok := r.eggs[name]
	if !ok {
		return Egg{}, Egg{}, &RefuseError{Egg: name, Reason: "名册里没有这枚卵"}
	}
	if !to.Valid() {
		return Egg{}, Egg{}, fmt.Errorf("改态拒收：目标状态 %q 不在八态闭集内", string(to))
	}
	if cur.State != to && !cur.State.CanTransitionTo(to) {
		return Egg{}, Egg{}, fmt.Errorf("改态拒收：卵 %s 的 %s → %s 不是合法转移（见 lifecycleEdges）",
			name, cur.State, to)
	}
	before := *cur
	if to == StateDeleted {
		refs = 0 // 墓碑的引用计数恒 0（真删已发生）
	}
	cur.State = to
	cur.Refs = refs
	cur.UpdatedAt = now
	after := *cur
	err := r.saveLocked()
	return before, after, err
}

// SetComplete 改**完工标记状态**（✗ 不是「目录存在」）。
func (r *Roster) SetComplete(name string, complete bool, blocks int, now time.Time) (Egg, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cur, ok := r.eggs[name]
	if !ok {
		return Egg{}, &RefuseError{Egg: name, Reason: "名册里没有这枚卵"}
	}
	cur.Complete = complete
	cur.Blocks = blocks
	cur.UpdatedAt = now
	cp := *cur
	err := r.saveLocked()
	return cp, err
}

// ReclaimableNames 可回收卵名（态＝reclaimable **且** 引用计数归零；✗ 不看时间）。
func (r *Roster) ReclaimableNames(refs *RefGraph) []string {
	var out []string
	for _, e := range r.List() {
		if e.State != StateReclaimable {
			continue
		}
		if refs != nil && refs.Count(e.Name) > 0 {
			continue
		}
		out = append(out, e.Name)
	}
	sort.Strings(out)
	return out
}

func (r *Roster) saveLocked() error {
	rows := make([]Egg, 0, len(r.eggs))
	for _, e := range r.eggs {
		rows = append(rows, *e)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Name < rows[j].Name })
	b, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(r.path, append(b, '\n'))
}
