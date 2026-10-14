package eggstore

import (
	"path/filepath"
	"time"
)

// Store 卵库（名册 ＋ 引用图 ＋ 审计流水 ＋ df 采样器）—— 本包唯一的入口对象。
type Store struct {
	// Root 卵库根目录（df 读数的取证卷）。
	Root   string
	Roster *Roster
	Refs   *RefGraph
	Log    *AuditLog
	// Sampler df 探针（nil ⇒ StatfsAvailable）。注入它是为了让负控确定性可造。
	Sampler FreedSampler
	// Waterline 水线（nil ⇒ DefaultWaterline）。★ 注入它是为了能在**真盘读数**上
	// 把「到水线 ⇒ 拒孵」那一态真跑出来（本机盘面 × 注入阈值），✗ 不去真填满盘。
	Waterline *Waterline
	// Now 时钟（nil ⇒ time.Now）。测试与审计对拍用。
	Now func() time.Time
}

// Open 打开卵库：<root>/egg-roster.json · <root>/egg-refs.json ·
// <root>/egg-lifecycle-audit.jsonl（件都不存在 ⇒ 空库，✗ 不报错）。
func Open(root string) (*Store, error) {
	roster, err := OpenRoster(filepath.Join(root, RosterFileName))
	if err != nil {
		return nil, err
	}
	refs := NewRefGraph(filepath.Join(root, RefGraphName))
	if err := refs.Load(); err != nil {
		return nil, err
	}
	return &Store{
		Root:   root,
		Roster: roster,
		Refs:   refs,
		Log:    NewAuditLog(filepath.Join(root, AuditFileName)),
	}, nil
}

// SetState 改态 ＋ 落审计（**两件事必须一起成功**：审计写不进 ⇒ 这次变更不算数）。
func (s *Store) SetState(name string, to Lifecycle, by, reason string) (Egg, Egg, error) {
	refs := s.Refs.Count(name)
	before, after, err := s.Roster.SetState(name, to, refs, s.now())
	if err != nil {
		return Egg{}, Egg{}, err
	}
	if err := s.Log.Append(ChangeRecord{
		Egg: name, From: before.State, To: after.State, At: s.now(),
		By: by, Reason: reason, Refs: refs, SHA256: after.SHA256,
	}); err != nil {
		return before, after, err
	}
	return before, after, nil
}

// MarkBroken 内容不符 ⇒ 置 broken（只可删或重取）。
func (s *Store) MarkBroken(name, why string) error {
	_, _, err := s.SetState(name, StateBroken, "verify", why)
	return err
}

// Register 把一枚**已校验完工**的卵登记进名册（★ 半成品不许走这条路）。
//
// 不变量：进册的卵 ⇒ 完工标记 true 且块数 > 0；否则拒（✗ 不让半成品混进名册）。
func (s *Store) Register(e Egg) error {
	if !e.Complete || e.Blocks <= 0 {
		return &RefuseError{Egg: e.Name, State: e.State,
			Reason: "半成品不入名册（完工标记未置位 / 块数为 0）—— 落地走 LandFile，校验全过才登记"}
	}
	if _, ok := s.Roster.Get(e.Name); ok {
		return &RefuseError{Egg: e.Name, Reason: "名册里已有同名卵（✗ 不静默覆盖）"}
	}
	if err := s.Roster.Upsert(e); err != nil {
		return err
	}
	return s.Log.Append(ChangeRecord{
		Egg: e.Name, From: "", To: e.State, At: s.now(), By: "landing",
		Reason: "落地校验全过后登记（完工标记 true）", Refs: 0, SHA256: e.SHA256,
	})
}
