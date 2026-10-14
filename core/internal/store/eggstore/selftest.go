package eggstore

import (
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// ErrSkip 自检里「本机造不出这一态」的哨兵 —— 只对 `Optional` 格有效，
// 且**必须带原因**（✗ 不许用「跳过」掩饰没做）。
var ErrSkip = errors.New("本机造不出这一态")

// SelfCase 一格自检用例。
//
// 契约（两条都统一成一句）：**`Run` 返回 nil ⇔ 该格断言成立**。
//   - `Negative=false`（正控）：断言 = 「这件事做成了」；
//   - `Negative=true`（负控）：断言 = 「这件事被拒了」。
//
// 于是负控也必须 `return nil` 才算过（✗ 不是「返回错就算过」）。
type SelfCase struct {
	Name     string
	Negative bool
	// Optional true ⇒ 本机造不出该态时 `Run` 可返回 `ErrSkip`（计入「不适用」，不计过）。
	Optional bool
	Run      func() error
}

// SelfCases 全量自检格（正控 + 负控；顺序稳定 ⇒ 输出可对拍）。
func SelfCases() []SelfCase {
	// ★ 本单（卵3②）增量：CoW 展开 / 回退链 / 硬链 的自检格**折进同一个自检口**
	//（CowSelfCases() 在 cow_selftest.go）—— ✗ 不另立第二个口。
	cases := []SelfCase{
		{Name: "正控·八态闭集齐备且逐值合法", Run: func() error {
			if len(LifecycleStates) != 8 {
				return fmt.Errorf("闭集应有 8 个值，实有 %d", len(LifecycleStates))
			}
			for _, s := range LifecycleStates {
				if !s.Valid() {
					return fmt.Errorf("%q 应合法却不合法", s)
				}
			}
			return nil
		}},
		{Name: "负控·闭集外状态被拒（非法值即错）", Negative: true, Run: func() error {
			for _, bad := range []string{"hatched", "VERIFIED", "landed2", "egg", " verified", "verified ", "deletedx"} {
				if got, err := ParseLifecycle(bad); err == nil {
					return fmt.Errorf("闭集外取值 %q 本该被拒，却收成了 %q", bad, string(got))
				}
			}
			return nil
		}},
		{Name: "正控·只有 verified 可孵（其余七态逐格拒）", Run: func() error {
			if err := CheckHatchable("e", StateVerified); err != nil {
				return fmt.Errorf("verified 应放行：%w", err)
			}
			for _, s := range LifecycleStates {
				if s == StateVerified {
					continue
				}
				if err := CheckHatchable("e", s); err == nil {
					return fmt.Errorf("%s 不该放行", s)
				}
			}
			return nil
		}},
		{Name: "负控·非法转移被拒（deleted 是墓碑终态 / broken 不许直变好人）", Negative: true, Run: func() error {
			if StateDeleted.CanTransitionTo(StateVerified) {
				return fmt.Errorf("deleted→verified 本该被拒")
			}
			if StateBroken.CanTransitionTo(StateVerified) {
				return fmt.Errorf("broken→verified 本该被拒（坏件只可删或重取）")
			}
			return nil
		}},
		{Name: "正控·合法链 landing→landed→verified 通过", Run: func() error {
			if !StateLanding.CanTransitionTo(StateLanded) || !StateLanded.CanTransitionTo(StateVerified) {
				return fmt.Errorf("landing→landed→verified 应合法")
			}
			if !StateVerified.CanTransitionTo(StateHatching) || !StateVerified.CanTransitionTo(StateReclaimable) {
				return fmt.Errorf("verified→hatching / verified→reclaimable 应合法")
			}
			return nil
		}},
		{Name: "正控·状态变更留可审计记录（写 N 条读回 N 条）", Run: func() error {
			dir, err := os.MkdirTemp("", "eggstore-st-audit-")
			if err != nil {
				return err
			}
			defer func() { _ = os.RemoveAll(dir) }()
			lg := NewAuditLog(filepath.Join(dir, AuditFileName))
			for i := 0; i < 3; i++ {
				if err := lg.Append(ChangeRecord{Egg: fmt.Sprintf("e%d", i), From: StateLanded,
					To: StateVerified, Refs: 0, SHA256: "aa"}); err != nil {
					return err
				}
			}
			recs, err := lg.ReadAll()
			if err != nil {
				return err
			}
			if len(recs) != 3 {
				return fmt.Errorf("审计应读回 3 条，实 %d 条", len(recs))
			}
			if recs[0].To != StateVerified || recs[0].At.IsZero() {
				return fmt.Errorf("审计行字段不全：%+v", recs[0])
			}
			return nil
		}},
		{Name: "负控·审计拒收闭集外状态", Negative: true, Run: func() error {
			dir, err := os.MkdirTemp("", "eggstore-st-auditneg-")
			if err != nil {
				return err
			}
			defer func() { _ = os.RemoveAll(dir) }()
			lg := NewAuditLog(filepath.Join(dir, AuditFileName))
			if aerr := lg.Append(ChangeRecord{Egg: "e", To: Lifecycle("hatched")}); aerr == nil {
				return fmt.Errorf("审计本该拒收闭集外状态")
			}
			return nil
		}},
		{Name: "负控·名册拒收非法状态", Negative: true, Run: func() error {
			dir, err := os.MkdirTemp("", "eggstore-st-rosterneg-")
			if err != nil {
				return err
			}
			defer func() { _ = os.RemoveAll(dir) }()
			r, err := OpenRoster(filepath.Join(dir, RosterFileName))
			if err != nil {
				return err
			}
			if uerr := r.Upsert(Egg{Name: "e", SHA256: "aa", State: Lifecycle("nope")}); uerr == nil {
				return fmt.Errorf("名册本该拒收闭集外状态")
			}
			return nil
		}},
		{Name: "负控·半成品不进名册（Register 拒）", Negative: true, Run: func() error {
			s, _, cleanup, err := newSelfStore()
			if err != nil {
				return err
			}
			defer cleanup()
			if rerr := s.Register(Egg{Name: "half", SHA256: "aa", State: StateLanded, Complete: false}); rerr == nil {
				return fmt.Errorf("半成品本该被名册拒（✗ 不许混进名册）")
			}
			return nil
		}},
		{Name: "正控·引用计数同一引用者幂等（不会重复计数）", Run: func() error {
			g := NewRefGraph("")
			if err := g.Acquire("e", "exp-1", RefExpansion); err != nil {
				return err
			}
			if err := g.Acquire("e", "exp-1", RefExpansion); err != nil {
				return err
			}
			if n := g.Count("e"); n != 1 {
				return fmt.Errorf("同一引用者重复 Acquire 后应仍为 1，实 %d", n)
			}
			if _, err := g.Release("e", "exp-1"); err != nil {
				return err
			}
			if !g.Zero("e") {
				return fmt.Errorf("解引用后应归零")
			}
			return nil
		}},
		{Name: "负控·引用种类闭集外被拒", Negative: true, Run: func() error {
			g := NewRefGraph("")
			if err := g.Acquire("e", "r1", RefKind("hardlink")); err == nil {
				return fmt.Errorf("引用种类闭集外本该被拒")
			}
			return nil
		}},
		{Name: "正控·引用计数未归零 ⇒ GC 不删且点名引用", Run: func() error {
			s, dir, cleanup, err := newSelfStore()
			if err != nil {
				return err
			}
			defer cleanup()
			eggFile := filepath.Join(dir, "e1.egg")
			if err := os.WriteFile(eggFile, []byte("x"), 0o644); err != nil {
				return err
			}
			if err := s.registerSelf("e1", eggFile); err != nil {
				return err
			}
			if err := s.Refs.Acquire("e1", "exp-1", RefExpansion); err != nil {
				return err
			}
			rep, err := s.GC(false)
			if err != nil {
				return err
			}
			if len(rep.Deleted) != 0 {
				return fmt.Errorf("引用计数 >0 时不许删，却删了 %v", rep.Deleted)
			}
			if len(rep.Pinned) != 1 || !strings.Contains(rep.Pinned[0].Why, "引用计数") {
				return fmt.Errorf("应被钉住并点名引用计数，实 %+v", rep.Pinned)
			}
			if _, err := os.Stat(eggFile); err != nil {
				return fmt.Errorf("件不该被删：%v", err)
			}
			return nil
		}},
		{Name: "正控·引用计数归零 ⇒ GC 真删 + 名册留墓碑", Run: func() error {
			s, dir, cleanup, err := newSelfStore()
			if err != nil {
				return err
			}
			defer cleanup()
			eggFile := filepath.Join(dir, "e2.egg")
			if err := os.WriteFile(eggFile, []byte("yy"), 0o644); err != nil {
				return err
			}
			if err := s.registerSelf("e2", eggFile); err != nil {
				return err
			}
			if err := s.Refs.Acquire("e2", "clone-1", RefClone); err != nil {
				return err
			}
			if _, err := s.Refs.Release("e2", "clone-1"); err != nil {
				return err
			}
			rep, err := s.GC(false)
			if err != nil {
				return err
			}
			if len(rep.Deleted) != 1 {
				return fmt.Errorf("归零后应删 1 枚，实 %v（钉住 %+v）", rep.Deleted, rep.Pinned)
			}
			if _, err := os.Stat(eggFile); !os.IsNotExist(err) {
				return fmt.Errorf("件应已删（err=%v）", err)
			}
			e, ok := s.Roster.Get("e2")
			if !ok || e.State != StateDeleted || e.Refs != 0 {
				return fmt.Errorf("名册应留墓碑（态 deleted / 计数 0），实 %+v ok=%v", e, ok)
			}
			return nil
		}},
		{Name: "正控·报数来源恒为 df（✗ 不是 du / 名义字节）", Run: func() error {
			if !strings.Contains(FreedSourceDF, "df") || strings.Contains(strings.ToLower(FreedSourceDF), "du") {
				return fmt.Errorf("报数来源牌不对：%q", FreedSourceDF)
			}
			s, dir, cleanup, err := newSelfStore()
			if err != nil {
				return err
			}
			defer cleanup()
			eggFile := filepath.Join(dir, "e3.egg")
			if err := os.WriteFile(eggFile, make([]byte, 4096), 0o644); err != nil {
				return err
			}
			if err := s.registerSelf("e3", eggFile); err != nil {
				return err
			}
			// 地面真值故意不动（模拟「块被克隆共享 ⇒ 真删不释放」）。
			s.Sampler = func(p string) (uint64, error) { return 1000, nil }
			rep, err := s.GC(false)
			if err != nil {
				return err
			}
			if rep.FreedSource != FreedSourceDF {
				return fmt.Errorf("报告来源应为 %q，实 %q", FreedSourceDF, rep.FreedSource)
			}
			if rep.FreedBytesDF != 0 {
				return fmt.Errorf("df 读数没变 ⇒ 已释放必须是 0，实 %d", rep.FreedBytesDF)
			}
			if rep.NominalBytesNotFreed != 4096 {
				return fmt.Errorf("名义字节应单列 4096，实 %d", rep.NominalBytesNotFreed)
			}
			if !strings.Contains(rep.NominalNote, "严禁") {
				return fmt.Errorf("名义字节那格必须带「严禁当已释放上报」的注，实 %q", rep.NominalNote)
			}
			return nil
		}},
		{Name: "正控·高水线（可用 14%）⇒ 拒新孵化", Run: func() error {
			p := EvalWaterline(DefaultWaterline, sample(100, 86, 14), sample(100, 50, 50), nil)
			if p.Verdict != VerdictRefuse || len(p.Reasons) == 0 {
				return fmt.Errorf("应 refuse 且给理由，实 %s / %v", p.Verdict, p.Reasons)
			}
			return nil
		}},
		{Name: "正控·判据按 Bavail 不按 Bfree（保留块救不了它）", Run: func() error {
			// Bfree 给到 30%（若拿 f_bfree 当判据会判绿），Bavail 只有 14% ⇒ 必须红。
			s := Sample{Label: "imagefs", Bsize: 1, Blocks: 100, Bfree: 30, Bavail: 14, Files: 1000, Ffree: 900}
			if s.FreePercentOfBfree() < DefaultWaterline.ImageFSAvailPercent {
				return fmt.Errorf("构造态不对：Bfree 口径 %.1f%% 本该判绿", s.FreePercentOfBfree())
			}
			p := EvalWaterline(DefaultWaterline, s, sample(100, 50, 50), nil)
			if p.Verdict != VerdictRefuse {
				return fmt.Errorf("Bavail=14%% 必须拒（✗ 不许用 f_bfree 当判据）")
			}
			return nil
		}},
		{Name: "负控·同一样本换成 Bavail=Bfree=30% ⇒ 不拒（证上一条不是恒红）", Negative: true, Run: func() error {
			p := EvalWaterline(DefaultWaterline, sample(100, 30, 30), sample(100, 50, 50), nil)
			if p.Verdict == VerdictRefuse {
				return fmt.Errorf("Bavail=30%% 不该拒，实 %s：%v", p.Verdict, p.Reasons)
			}
			return nil
		}},
		{Name: "正控·nodefs 硬线（可用 9%<10%）⇒ 拒", Run: func() error {
			p := EvalWaterline(DefaultWaterline, sample(100, 50, 50), sample(100, 91, 9), nil)
			if p.Verdict != VerdictRefuse {
				return fmt.Errorf("nodefs 9%% 必须拒，实 %s", p.Verdict)
			}
			return nil
		}},
		{Name: "正控·inode 硬线（Ffree 4%<5%）⇒ 拒", Run: func() error {
			s := Sample{Label: "imagefs", Bsize: 1, Blocks: 100, Bfree: 50, Bavail: 50, Files: 100, Ffree: 4}
			p := EvalWaterline(DefaultWaterline, s, sample(100, 50, 50), nil)
			if p.Verdict != VerdictRefuse {
				return fmt.Errorf("inode 可用 4%% 必须拒，实 %s：%v", p.Verdict, p.Reasons)
			}
			return nil
		}},
		{Name: "正控·低水线（可用 19%）⇒ 先回收但不拒 + 带可回收清单", Run: func() error {
			p := EvalWaterline(DefaultWaterline, sample(100, 81, 19), sample(100, 50, 50), []string{"old-1"})
			if p.Verdict != VerdictReclaim {
				return fmt.Errorf("可用 19%% 应判先回收，实 %s", p.Verdict)
			}
			if len(p.Reclaimable) != 1 || p.Reclaimable[0] != "old-1" {
				return fmt.Errorf("应带上可回收清单，实 %v", p.Reclaimable)
			}
			return nil
		}},
		{Name: "负控·跨卷临时名 ⇒ 拒（rename 会退化成 copy）", Negative: true, Optional: true, Run: func() error {
			other, ok := secondVolumeDir()
			if !ok {
				return fmt.Errorf("%w：本机只有一个卷", ErrSkip)
			}
			dstDir := filepath.Join(other, "eggstore-selftest-cvx")
			if err := os.MkdirAll(dstDir, 0o755); err != nil {
				return err
			}
			defer func() { _ = os.RemoveAll(dstDir) }()
			dst := filepath.Join(dstDir, "x.egg")
			tmp := filepath.Join(os.TempDir(), "eggstore-cvx.part")
			if same, err := SameVolume(tmp, dst); err == nil && same {
				return fmt.Errorf("%w：本机两卷被判定成同卷", ErrSkip)
			}
			_, lerr := LandFile(dst, strings.NewReader("hello"), LandOptions{TmpPath: tmp})
			if !errors.Is(lerr, ErrCrossVolume) {
				return fmt.Errorf("跨卷临时件本该被 ErrCrossVolume 拒，实 err=%v", lerr)
			}
			if _, serr := os.Stat(tmp); !os.IsNotExist(serr) {
				return fmt.Errorf("被拒时不该留下临时件（err=%v）", serr)
			}
			return nil
		}},
		{Name: "正控·落地中途失败 ⇒ 终名不产生、临时件清掉（可重试）", Run: func() error {
			dir, err := os.MkdirTemp("", "eggstore-st-fail-")
			if err != nil {
				return err
			}
			defer func() { _ = os.RemoveAll(dir) }()
			dst := filepath.Join(dir, "e.egg")
			if _, err := LandFile(dst, &failAfter{data: []byte("abcdefghij"), n: 4}, LandOptions{ChunkSize: 2}); err == nil {
				return fmt.Errorf("源中途出错 ⇒ 落地应失败")
			}
			if _, err := os.Stat(dst); !os.IsNotExist(err) {
				return fmt.Errorf("终名不该产生（err=%v）", err)
			}
			if _, err := os.Stat(TmpNameFor(dst)); !os.IsNotExist(err) {
				return fmt.Errorf("半成品临时件应被清掉（err=%v）", err)
			}
			if _, err := LandFile(dst, strings.NewReader("abcde"), LandOptions{}); err != nil {
				return fmt.Errorf("可重试：同一终名再来一次应成功，实 %v", err)
			}
			return nil
		}},
		{Name: "正控·落地成功 ⇒ 逐块校验 + 同卷 + 父目录 fsync + sha 64 位", Run: func() error {
			dir, err := os.MkdirTemp("", "eggstore-st-ok-")
			if err != nil {
				return err
			}
			defer func() { _ = os.RemoveAll(dir) }()
			dst := filepath.Join(dir, "ok.egg")
			res, err := LandFile(dst, strings.NewReader("0123456789"), LandOptions{ChunkSize: 4})
			if err != nil {
				return err
			}
			if res.CrossVolume {
				return fmt.Errorf("同目录临时名不该判跨卷")
			}
			if res.Blocks != 3 || res.Bytes != 10 {
				return fmt.Errorf("块应 3（4+4+2）· 字节应 10，实 %d / %d", res.Blocks, res.Bytes)
			}
			if !res.ReadBack || res.Verified != res.Blocks {
				return fmt.Errorf("逐块回读校验应逐块过，实 %d/%d", res.Verified, res.Blocks)
			}
			if !res.DirSynced {
				return fmt.Errorf("父目录 fsync 应成功")
			}
			if len(res.SHA256) != 64 {
				return fmt.Errorf("sha256 应为 64 位十六进制，实 %q", res.SHA256)
			}
			if _, err := hex.DecodeString(res.SHA256); err != nil {
				return fmt.Errorf("sha256 不是十六进制：%v", err)
			}
			return nil
		}},
		{Name: "负控·逐块 sha256 与期望不符 ⇒ 拒（✗ 不落成终名）", Negative: true, Run: func() error {
			dir, err := os.MkdirTemp("", "eggstore-st-blocksha-")
			if err != nil {
				return err
			}
			defer func() { _ = os.RemoveAll(dir) }()
			dst := filepath.Join(dir, "b.egg")
			_, lerr := LandFile(dst, strings.NewReader("abcd"), LandOptions{ChunkSize: 4,
				ExpectedBlocks: []string{"0000000000000000000000000000000000000000000000000000000000000000"}})
			if lerr == nil {
				return fmt.Errorf("块 sha256 不符本该被拒")
			}
			if _, serr := os.Stat(dst); !os.IsNotExist(serr) {
				return fmt.Errorf("被拒后终名不该存在（err=%v）", serr)
			}
			return nil
		}},
		{Name: "负控·缺完工标记 ⇒ 不可用（目录明明在盘）", Negative: true, Run: func() error {
			dir, err := os.MkdirTemp("", "eggstore-st-nomark-")
			if err != nil {
				return err
			}
			defer func() { _ = os.RemoveAll(dir) }()
			exp := filepath.Join(dir, "expanded")
			if err := os.MkdirAll(exp, 0o755); err != nil {
				return err
			}
			if ok, _ := DirUsable(exp); ok {
				return fmt.Errorf("缺完工标记的目录本该判不可用")
			}
			return nil
		}},
		{Name: "正控·完工标记齐备 ⇒ 可用", Run: func() error {
			dir, err := os.MkdirTemp("", "eggstore-st-mark-")
			if err != nil {
				return err
			}
			defer func() { _ = os.RemoveAll(dir) }()
			exp := filepath.Join(dir, "expanded")
			if err := os.MkdirAll(exp, 0o755); err != nil {
				return err
			}
			if err := WriteDoneMarker(exp, DoneMarker{SHA256: "aa", Blocks: 2}); err != nil {
				return err
			}
			if ok, why := DirUsable(exp); !ok {
				return fmt.Errorf("完工标记齐备应判可用，实不可用：%s", why)
			}
			return nil
		}},
		{Name: "负控·空完工标记被拒（✗ 不许写空标记充完工）", Negative: true, Run: func() error {
			dir, err := os.MkdirTemp("", "eggstore-st-emptymark-")
			if err != nil {
				return err
			}
			defer func() { _ = os.RemoveAll(dir) }()
			if werr := WriteDoneMarker(dir, DoneMarker{}); werr == nil {
				return fmt.Errorf("空完工标记本该被拒")
			}
			return nil
		}},
		{Name: "正控·同卷判定：同目录两路径同卷", Run: func() error {
			dir, err := os.MkdirTemp("", "eggstore-st-samevol-")
			if err != nil {
				return err
			}
			defer func() { _ = os.RemoveAll(dir) }()
			same, err := SameVolume(filepath.Join(dir, "a.egg"), filepath.Join(dir, ".a.egg.part"))
			if err != nil {
				return err
			}
			if !same {
				return fmt.Errorf("同目录两路径应同卷")
			}
			return nil
		}},
	}
	return append(cases, CowSelfCases()...)
}

// SelfTest 跑全量自检：返回 (通过格数, 失败格数, 不适用格数, 逐行输出)。
func SelfTest() (passed, failed, skipped int, lines []string) {
	for _, c := range SelfCases() {
		err := c.Run()
		switch {
		case errors.Is(err, ErrSkip):
			if !c.Optional {
				failed++
				lines = append(lines, fmt.Sprintf("FAIL %s：返回了「不适用」，但这一格不是可选项", c.Name))
				continue
			}
			skipped++
			lines = append(lines, fmt.Sprintf("SKIP %s：%s", c.Name, err))
		case c.Negative:
			if err == nil {
				passed++
				lines = append(lines, fmt.Sprintf("PASS %s（负控·已按预期被拒）", c.Name))
				continue
			}
			failed++
			lines = append(lines, fmt.Sprintf("FAIL %s（负控·本该被拒却没被拒）：%s", c.Name, err))
		default:
			if err == nil {
				passed++
				lines = append(lines, "PASS "+c.Name)
				continue
			}
			failed++
			lines = append(lines, fmt.Sprintf("FAIL %s：%v", c.Name, err))
		}
	}
	return passed, failed, skipped, lines
}

// SelfTestSummary 自检结论一行（人面）。
func SelfTestSummary() string {
	p, f, s, _ := SelfTest()
	verdict := "全过"
	if f > 0 {
		verdict = "有失败"
	}
	return fmt.Sprintf("自检结论: %s（断言 %d 格 · 失败 %d 格 · 不适用 %d 格）", verdict, p+f+s, f, s)
}

// ── 自检内部小助手 ────────────────────────────────────────────────

func newSelfStore() (*Store, string, func(), error) {
	dir, err := os.MkdirTemp("", "eggstore-st-")
	if err != nil {
		return nil, "", func() {}, err
	}
	s, err := Open(dir)
	if err != nil {
		_ = os.RemoveAll(dir)
		return nil, "", func() {}, err
	}
	// 自检里不让真 df 噪声干扰「引用计数」那两格：采样器固定。
	s.Sampler = func(p string) (uint64, error) { return 1 << 20, nil }
	return s, dir, func() { _ = os.RemoveAll(dir) }, nil
}

func (s *Store) registerSelf(name, path string) error {
	st, err := os.Stat(path)
	if err != nil {
		return err
	}
	return s.Register(Egg{Name: name, SHA256: "sha-" + name, Path: path,
		Bytes: st.Size(), State: StateReclaimable, Complete: true, Blocks: 1})
}

// sample 造一个百分比样本：Bsize=1、Blocks=100 ⇒ Bavail 即「可用百分比」。
func sample(blocks, bfree, bavail uint64) Sample {
	return Sample{Label: "sample", Bsize: 1, Blocks: blocks, Bfree: bfree, Bavail: bavail,
		Files: 1000, Ffree: 900}
}

// secondVolumeDir 本机是否真有第二个卷（跨卷负控要真跨卷，✗ 不拿假的凑）。
func secondVolumeDir() (string, bool) {
	if runtime.GOOS != "darwin" {
		return "", false
	}
	boot, err := deviceOf("/")
	if err != nil {
		return "", false
	}
	ents, err := os.ReadDir("/Volumes")
	if err != nil {
		return "", false
	}
	for _, e := range ents {
		p := filepath.Join("/Volumes", e.Name())
		dev, err := deviceOf(p)
		if err != nil || dev == boot {
			continue
		}
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			return p, true
		}
	}
	return "", false
}

// failAfter 一个「读到第 n 字节后报错」的源 —— 用来造「落地中途失败」那一态。
type failAfter struct {
	data []byte
	n    int
	pos  int
}

func (f *failAfter) Read(p []byte) (int, error) {
	if f.pos >= f.n {
		return 0, fmt.Errorf("注入的读失败（第 %d 字节之后）", f.n)
	}
	rest := f.n - f.pos
	if rest > len(p) {
		rest = len(p)
	}
	if f.pos+rest > len(f.data) {
		rest = len(f.data) - f.pos
	}
	n := copy(p, f.data[f.pos:f.pos+rest])
	f.pos += n
	return n, nil
}
