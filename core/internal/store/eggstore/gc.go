package eggstore

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// FreedSourceDF —— 报数来源的**唯一合法取值**。★ 只有 `df` 是地面真值。
//
// 为什么写死一张来源牌：`du` 对含克隆/共享块的目录树**高估 10–100 倍**
// （APFS 克隆在每个副本上都报完整大小却共享物理块）⇒ 任何「已释放 N GB」的读数
// 必须能自证它来自 `df`（`statfs.f_bavail`），而不是 `du` 或「卵文件的名义字节」。
const FreedSourceDF = "df:statfs.f_bavail×Bsize"

// NominalBytesNote 名义字节那一格的固定注脚（★ 不许拿它当已释放读数）。
const NominalBytesNote = "卵文件**名义字节**（Σ 名册 Bytes）—— ★ 只作登记，✗ 严禁当「已释放 N GB」上报（克隆/展开物共享物理块，实际释放量只认 df）"

// FreedSampler 量「地面真值可用空间」的探针（**只认 df**）。注入它是为了：
//   - 负控可确定性构造（例如「块被共享 ⇒ 真释放 0 字节」这一态，不必靠真盘噪声）；
//   - 默认实现 = `StatfsAvailable`（真实 df）。
type FreedSampler func(path string) (uint64, error)

// StatfsAvailable 真实 df 读数（`statfs.f_bavail` × `Bsize`）。
func StatfsAvailable(path string) (uint64, error) {
	v, err := ReadVolume(path)
	if err != nil {
		return 0, err
	}
	return v.AvailableBytes(), nil
}

// Pin 被钉住（本轮不删）的卵 ＋ 原因。
type Pin struct {
	Egg   string    `json:"egg"`
	Refs  int       `json:"refs"`
	State Lifecycle `json:"state"`
	Why   string    `json:"why"`
}

// GCReport GC 的一次读数。★ 报数口径就写在这张表里，✗ 不许在调用方另算一套。
type GCReport struct {
	Deleted []string `json:"deleted"`
	Pinned  []Pin    `json:"pinned"`
	// FreedBytesDF 本次**真删**之后的 df 差值（地面真值；噪声可能使它为负 ⇒ 原样报、不截断）。
	FreedBytesDF int64  `json:"freed_bytes_df"`
	FreedSource  string `json:"freed_source"`
	DFBefore     uint64 `json:"df_before"`
	DFAfter      uint64 `json:"df_after"`
	// NominalBytesNotFreed 被删卵的名义字节合计 —— ★ **不是**已释放读数（见 NominalBytesNote）。
	NominalBytesNotFreed int64  `json:"nominal_bytes_not_freed"`
	NominalNote          string `json:"nominal_note"`
	DryRun               bool   `json:"dry_run"`
	Note                 string `json:"note"`
}

// GC 回收一轮：★ 真删**只在引用计数归零时**发生。
//
// 钉住规则（任一成立即不删）：
//  1. 态 = `hatching`（正在孵化/被服务 —— 引用计数不参与放行）；
//  2. 引用计数 > 0（展开物 / 克隆副本仍在引用 ⇒ 删前必须先解引用）；
//  3. 态不在回收面（只回收 `reclaimable` / `broken`）。
func (s *Store) GC(dryRun bool) (GCReport, error) {
	rep := GCReport{FreedSource: FreedSourceDF, NominalNote: NominalBytesNotFreedNote(), DryRun: dryRun}
	var deletable []Egg
	for _, e := range s.Roster.List() {
		if e.State == StateDeleted {
			continue
		}
		n := s.Refs.Count(e.Name)
		switch {
		case e.State == StateHatching:
			rep.Pinned = append(rep.Pinned, Pin{Egg: e.Name, Refs: n, State: e.State,
				Why: "正在孵化/被服务（取锁）—— 引用计数不参与放行"})
			continue
		case n > 0:
			rep.Pinned = append(rep.Pinned, Pin{Egg: e.Name, Refs: n, State: e.State,
				Why: fmt.Sprintf("引用计数 %d > 0（展开物/克隆副本仍在引用）—— 删前必须先解引用", n)})
			continue
		case !e.State.GCReclaimable():
			rep.Pinned = append(rep.Pinned, Pin{Egg: e.Name, Refs: n, State: e.State,
				Why: fmt.Sprintf("态 %s 不在回收面（只回收 reclaimable / broken）", e.State)})
			continue
		}
		deletable = append(deletable, e)
	}
	if dryRun {
		for _, e := range deletable {
			rep.Deleted = append(rep.Deleted, e.Name)
		}
		rep.Note = "干跑：只出计划，未删任何件、未取 df 读数"
		return rep, nil
	}
	if len(deletable) == 0 {
		rep.Note = "本轮无可真删的卵（引用计数未归零 / 态不在回收面）"
		return rep, nil
	}

	before, err := s.sample()
	if err != nil {
		return rep, fmt.Errorf("GC 前取 df 读数失败（✗ 取不到就不报「已释放」）：%w", err)
	}
	for _, e := range deletable {
		if strings.TrimSpace(e.Path) != "" {
			if err := os.Remove(e.Path); err != nil && !os.IsNotExist(err) {
				return rep, fmt.Errorf("真删卵 %s 失败（%s）：%w", e.Name, e.Path, err)
			}
		}
		s.Refs.Drop(e.Name)
		if _, _, err := s.Roster.SetState(e.Name, StateDeleted, 0, s.now()); err != nil {
			// 件已删但册没改上 —— 出声，别静默（名册与盘面不一致必须可见）。
			return rep, fmt.Errorf("卵 %s 件已删但名册改态失败：%w", e.Name, err)
		}
		if err := s.Log.Append(ChangeRecord{
			Egg: e.Name, From: e.State, To: StateDeleted, At: s.now(),
			By: "gc", Refs: 0, SHA256: e.SHA256,
			Reason: fmt.Sprintf("引用计数归零 ⇒ 真删（删前计数 %d）；df 报数", e.Refs),
		}); err != nil {
			return rep, fmt.Errorf("卵 %s 的删却审计写不进 ⇒ 这笔变更没留痕：%w", e.Name, err)
		}
		rep.Deleted = append(rep.Deleted, e.Name)
		rep.NominalBytesNotFreed += e.Bytes
	}
	after, err := s.sample()
	if err != nil {
		return rep, fmt.Errorf("GC 后取 df 读数失败（✗ 取不到就不报「已释放」）：%w", err)
	}
	rep.DFBefore, rep.DFAfter = before, after
	rep.FreedBytesDF = int64(after) - int64(before)
	rep.Note = fmt.Sprintf("真删 %d 枚；已释放读数取自 df（%s），✗ 不用 du、✗ 不用名义字节",
		len(rep.Deleted), FreedSourceDF)
	return rep, nil
}

// NominalBytesNotFreedNote 名义字节那格的注脚（单独一枚函数：调用方与自检都读同一句）。
func NominalBytesNotFreedNote() string { return NominalBytesNote }

func (s *Store) sample() (uint64, error) {
	fs := s.Sampler
	if fs == nil {
		fs = StatfsAvailable
	}
	return fs(s.Root)
}

func (s *Store) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}
