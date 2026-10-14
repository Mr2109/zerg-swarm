package eggstore

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// ──────────── 展开物 / 克隆副本：登记进**同一引用图** ＋ 删前解引用（U3）────────────
//
// ★ 口径（用户拍定 · §十九 #14 / #16）：展开物与克隆副本**纳入同一引用计数**、
// **删前解引用**、真删判据＝**引用计数归零**；报数**只认 df**。
// 本件**复用** `refs.go` 的 `RefGraph`（✗ 不另造一张图、✗ 不搞「两套计数」），
// 复用 `gc.go` 的 `Store.sample()`（df 探针）与 `FreedSourceDF`（来源牌）。
//
// ★ 登记顺序（本件刻意）：**先登记引用 → 再落件**。
//   - 先登记后落件，中途崩 ⇒ 最多留一个「有名无实」的引用 ⇒ **钉住卵**（✗ 不会误删）——**安全向**；
//   - 反过来（先落件后登记）崩 ⇒ 留下**无引用**的副本 ⇒ 卵可能被 GC 删而副本仍占共享块 ⇒
//     读数失真（「已删卵 ≠ 已释放」正好踩这条）⇒ ✗ 不许。

// Expansion 一枚已登记的展开物 / 克隆副本（同一份引用图里的一个**引用者**）。
type Expansion struct {
	Egg   string    `json:"egg"`
	Ref   string    `json:"ref"`
	Kind  RefKind   `json:"kind"`
	Path  string    `json:"path"`
	Tier  CloneTier `json:"tier"`
	CoW   bool      `json:"cow"`
	Bytes int64     `json:"bytes"`
	At    time.Time `json:"at"`
}

// ExpandReport 一次展开的读数（★ 报数口径同 GC：**没有**「已释放」格）。
type ExpandReport struct {
	Expansion
	Result    CowResult `json:"result"`
	RefsAfter int       `json:"refs_after"`
}

// Expand 展开一枚卵到 dstPath：**先登记引用（同一引用图）→ 再 CoW 整文件落件**。
//
// 形状闸（都 fail-closed、都出声）：引用种类落闭集 · 卵在册且完工 · 态可展开 · 源路径非空。
func (s *Store) Expand(egg, refID, dstPath string, kind RefKind, opt CowOptions) (ExpandReport, error) {
	var rep ExpandReport
	if !kind.Valid() {
		return rep, fmt.Errorf("展开拒收：引用种类 %q 不在闭集内（只许 %s / %s）",
			string(kind), RefExpansion, RefClone)
	}
	if strings.TrimSpace(egg) == "" || strings.TrimSpace(refID) == "" || strings.TrimSpace(dstPath) == "" {
		return rep, fmt.Errorf("展开拒收：卵名 / 引用者 id / 目标路径 三者都不许为空")
	}
	e, ok := s.Roster.Get(egg)
	if !ok {
		return rep, &RefuseError{Egg: egg, Reason: "名册里没有这枚卵（✗ 不许展开不在册的卵）"}
	}
	if !e.Complete {
		return rep, &RefuseError{Egg: egg, State: e.State,
			Reason: "半成品：完工标记不是「已完成」（✗ 目录 / 文件存在 ≠ 可用）"}
	}
	switch e.State {
	case StateDeleted, StateBroken:
		return rep, &RefuseError{Egg: egg, State: e.State,
			Reason: "态不在可展开面（deleted 是墓碑 / broken 只可删或重取）"}
	}
	if strings.TrimSpace(e.Path) == "" {
		return rep, &RefuseError{Egg: egg, State: e.State, Reason: "卵文件路径为空 ⇒ 无源可展开"}
	}

	// ★ 先登记引用（同一引用图）—— 崩了也只留「钉住卵」的悬空引用（安全向）。
	if err := s.Refs.Acquire(egg, refID, kind); err != nil {
		return rep, err
	}
	if err := s.Refs.Save(); err != nil {
		return rep, err
	}
	res, err := CloneWholeFile(dstPath, e.Path, opt)
	if err != nil {
		// 回滚引用：✗ 不留悬空引用（名册与盘面必须一致）。
		_, _ = s.Refs.Release(egg, refID)
		_ = s.Refs.Save()
		return rep, fmt.Errorf("展开 %s → %s 失败（已回滚引用，✗ 不留悬空引用）：%w", egg, dstPath, err)
	}
	rep = ExpandReport{
		Expansion: Expansion{Egg: egg, Ref: refID, Kind: kind, Path: dstPath,
			Tier: res.Tier, CoW: res.CoW(), Bytes: res.Bytes, At: s.now()},
		Result: res, RefsAfter: s.Refs.Count(egg),
	}
	_ = s.Log.Append(ChangeRecord{Egg: egg, From: e.State, To: e.State, At: s.now(),
		By: "expand", Refs: rep.RefsAfter, SHA256: e.SHA256,
		Reason: fmt.Sprintf("展开登记 %s（档=%s · 共享块=%v）—— 展开物 / 克隆副本纳入**同一引用计数**",
			refID, res.Tier, res.CoW())})
	return rep, nil
}

// ReleaseReport 一次「删前解引用」的读数（★ 已释放**只认 df**）。
type ReleaseReport struct {
	Egg          string `json:"egg"`
	Ref          string `json:"ref"`
	Path         string `json:"path"`
	Released     bool   `json:"released"`
	RefsLeft     int    `json:"refs_left"`
	Zero         bool   `json:"zero"`
	DFBefore     uint64 `json:"df_before"`
	DFAfter      uint64 `json:"df_after"`
	FreedBytesDF int64  `json:"freed_bytes_df"`
	FreedSource  string `json:"freed_source"`
	NominalNote  string `json:"nominal_note"`
}

// ReleaseExpansion **删前解引用**：删掉展开物 / 副本件 ＋ 解掉那一条引用。
//
// ★ 顺序：**先删件 → 再解引用**。反过来（先解引用后删件）存在窗口期「计数已归零、件还在」
// ⇒ GC 可能把卵删掉而副本仍占共享块 ⇒ 读数失真。先删件的最坏情形只是「暂留一个钉住卵的
// 引用」——**安全向**。返回的 `FreedBytesDF` 取自 `df`（✗ 不用 du、✗ 不用名义字节）。
func (s *Store) ReleaseExpansion(egg, refID, path string) (ReleaseReport, error) {
	rep := ReleaseReport{Egg: egg, Ref: refID, Path: path,
		FreedSource: FreedSourceDF, NominalNote: NominalBytesNote}
	before, err := s.sample()
	if err != nil {
		return rep, fmt.Errorf("解引用前取 df 失败（✗ 取不到就不报「已释放」）：%w", err)
	}
	if strings.TrimSpace(path) != "" {
		if rerr := os.Remove(path); rerr != nil && !os.IsNotExist(rerr) {
			return rep, fmt.Errorf("删副本件失败（%s）：%w", path, rerr)
		}
	}
	n, err := s.Refs.Release(egg, refID)
	if err != nil {
		return rep, err
	}
	rep.Released = n
	if err := s.Refs.Save(); err != nil {
		return rep, err
	}
	after, err := s.sample()
	if err != nil {
		return rep, fmt.Errorf("解引用后取 df 失败（✗ 取不到就不报「已释放」）：%w", err)
	}
	rep.DFBefore, rep.DFAfter = before, after
	rep.FreedBytesDF = int64(after) - int64(before)
	rep.RefsLeft = s.Refs.Count(egg)
	rep.Zero = s.Refs.Zero(egg)
	if e, ok := s.Roster.Get(egg); ok {
		_ = s.Log.Append(ChangeRecord{Egg: egg, From: e.State, To: e.State, At: s.now(),
			By: "release", Refs: rep.RefsLeft, SHA256: e.SHA256,
			Reason: fmt.Sprintf("解引用 %s（删件 %s）⇒ 计数 %d · 归零=%v · 已释放读数取自 df（%s）",
				refID, path, rep.RefsLeft, rep.Zero, FreedSourceDF)})
	}
	return rep, nil
}

// DeleteIfZero 真删**只在引用计数归零时**发生（薄封装 GC；✗ 不改 GC 语义）。
//
// 未归零 ⇒ 出声拒（`RefuseError`），✗ 不静默、✗ 不「先清账再删」。
func (s *Store) DeleteIfZero(egg string) (GCReport, error) {
	if n := s.Refs.Count(egg); n > 0 {
		return GCReport{}, &RefuseError{Egg: egg, Reason: fmt.Sprintf(
			"引用计数 %d > 0（展开物 / 克隆副本仍在引用）—— ✗ 不许真删，**删前必须先解引用**", n)}
	}
	return s.GC(false)
}
