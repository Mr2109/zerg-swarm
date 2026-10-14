package eggstore

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
)

// RefGraphName 引用图件（运行态；原子落盘）。
const RefGraphName = "egg-refs.json"

// RefKind 引用的种类 —— **闭集**。
//
// ★ 口径：**展开物与克隆副本纳入同一份引用计数**（用户拍定 · 设计 §14.4 M1）。
// 「两套计数」是被否掉的那个错解；这里只有一张图、一个计数。
type RefKind string

const (
	// RefExpansion 展开物（孵化时展开到宿主侧只读路径的那份）。
	RefExpansion RefKind = "expansion"
	// RefClone 克隆副本（CoW 克隆 / reflink / 硬链 —— 与原卵**共享物理块**）。
	RefClone RefKind = "clone"
)

// RefKinds 闭集。
var RefKinds = []RefKind{RefExpansion, RefClone}

// Valid 在闭集内？
func (k RefKind) Valid() bool {
	for _, x := range RefKinds {
		if k == x {
			return true
		}
	}
	return false
}

// RefGraph 卵 ⟷ 引用者 的引用图（同一份计数）。
//
// 为什么必须「同一份」：APFS `clonefile` / reflink / 硬链 的克隆与原文件**共享物理块**
// ⇒ 删任一方都不释放被共享的块。此时若按「引用者各自的账」算，就会出现「两边都说自己
// 是唯一持有者、于是各自删掉」——物理块被最后一方释放，而双方都以为对方还留着。
type RefGraph struct {
	path string
	mu   sync.Mutex
	m    map[string]map[string]RefKind // egg -> refID -> kind
}

// NewRefGraph 空图（path 非空 ⇒ 可 Save/Load）。
func NewRefGraph(path string) *RefGraph {
	return &RefGraph{path: path, m: map[string]map[string]RefKind{}}
}

// Path 落盘路径。
func (g *RefGraph) Path() string { return g.path }

// Acquire 加一条引用（引用者 id 幂等：同一 refID 重复 Acquire 只记一次，种类可更新）。
func (g *RefGraph) Acquire(egg, ref string, kind RefKind) error {
	if strings.TrimSpace(egg) == "" || strings.TrimSpace(ref) == "" {
		return fmt.Errorf("加引用拒收：卵名与引用者 id 都不许为空")
	}
	if !kind.Valid() {
		return fmt.Errorf("加引用拒收：引用种类 %q 不在闭集内（只许 %s / %s）",
			string(kind), RefExpansion, RefClone)
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.m[egg] == nil {
		g.m[egg] = map[string]RefKind{}
	}
	g.m[egg][ref] = kind
	return nil
}

// Release **解引用**（删卵的前置动作）。返回本次是否真的解掉一条
// （false = 本就没有这条引用 —— 幂等，不是错误）。
func (g *RefGraph) Release(egg, ref string) (bool, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	set, ok := g.m[egg]
	if !ok {
		return false, nil
	}
	if _, had := set[ref]; !had {
		return false, nil
	}
	delete(set, ref)
	if len(set) == 0 {
		delete(g.m, egg)
	}
	return true, nil
}

// Count 该卵当前的引用计数（**真删的唯一判据面**）。
func (g *RefGraph) Count(egg string) int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return len(g.m[egg])
}

// Zero 引用计数归零？（★ 只有归零才允许真删）
func (g *RefGraph) Zero(egg string) bool { return g.Count(egg) == 0 }

// RefsOf 该卵的引用者清单（排序）。
func (g *RefGraph) RefsOf(egg string) []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	out := make([]string, 0, len(g.m[egg]))
	for ref := range g.m[egg] {
		out = append(out, ref)
	}
	sort.Strings(out)
	return out
}

// Eggs 图里有引用的卵（排序）。
func (g *RefGraph) Eggs() []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	out := make([]string, 0, len(g.m))
	for egg := range g.m {
		out = append(out, egg)
	}
	sort.Strings(out)
	return out
}

// Drop 直接摘掉一枚卵的全部引用（真删之后调用；✗ 不得用它在删前「清账」）。
func (g *RefGraph) Drop(egg string) { g.mu.Lock(); delete(g.m, egg); g.mu.Unlock() }

type refRow struct {
	Egg  string  `json:"egg"`
	Ref  string  `json:"ref"`
	Kind RefKind `json:"kind"`
}

// Save 原子落盘。
func (g *RefGraph) Save() error {
	if g.path == "" {
		return fmt.Errorf("引用图没有落点路径 ⇒ 不落盘")
	}
	rows := []refRow{}
	g.mu.Lock()
	for egg, set := range g.m {
		for ref, kind := range set {
			rows = append(rows, refRow{Egg: egg, Ref: ref, Kind: kind})
		}
	}
	g.mu.Unlock()
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Egg != rows[j].Egg {
			return rows[i].Egg < rows[j].Egg
		}
		return rows[i].Ref < rows[j].Ref
	})
	b, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(g.path, append(b, '\n'))
}

// Load 从盘读回（不存在 ⇒ 空图）。
func (g *RefGraph) Load() error {
	if g.path == "" {
		return nil
	}
	b, err := os.ReadFile(g.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var rows []refRow
	if err := json.Unmarshal(b, &rows); err != nil {
		return fmt.Errorf("引用图不可解析（%s）：%w", g.path, err)
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.m = map[string]map[string]RefKind{}
	for _, r := range rows {
		if !r.Kind.Valid() {
			return fmt.Errorf("引用图（%s）里 %s→%s 的种类 %q 不在闭集内", g.path, r.Egg, r.Ref, string(r.Kind))
		}
		if g.m[r.Egg] == nil {
			g.m[r.Egg] = map[string]RefKind{}
		}
		g.m[r.Egg][r.Ref] = r.Kind
	}
	return nil
}
