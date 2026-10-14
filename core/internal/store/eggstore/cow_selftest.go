package eggstore

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// CowSelfCases ★ 本单（卵开工 · 卵3②）的 CoW 展开自检格 —— **折进同一个自检口**
// （`SelfCases()` 末尾 `append(cases, CowSelfCases()...)`，✗ 不另立第二个口）。
//
// 契约照 selftest.go：`Run` 返回 nil ⇔ 断言成立；`Negative=true` 时 nil ⇔ 「**被拒了**」。
func CowSelfCases() []SelfCase {
	return []SelfCase{
		{Name: "正控·整文件 CoW 展开（本平台档）内容逐字节同", Run: func() error {
			dir, err := os.MkdirTemp("", "eggstore-cow-whole-")
			if err != nil {
				return err
			}
			defer func() { _ = os.RemoveAll(dir) }()
			src := filepath.Join(dir, "src.bin")
			dst := filepath.Join(dir, "dst.bin")
			payload := cowPayload(1 << 20)
			if err := os.WriteFile(src, payload, 0o644); err != nil {
				return err
			}
			res, err := CloneWholeFile(dst, src, CowOptions{})
			if err != nil {
				return fmt.Errorf("整文件展开应成功：%w", err)
			}
			if !res.Tier.Valid() {
				return fmt.Errorf("档位 %q 不在闭集内", res.Tier)
			}
			if err := sameBytes(dst, payload); err != nil {
				return err
			}
			if res.Whole != true {
				return fmt.Errorf("整文件档应标 Whole=true")
			}
			return nil
		}},
		{Name: "正控·段范围取用内容逐字节同（★ macOS 必落真拷贝 io.Copy）", Run: func() error {
			dir, err := os.MkdirTemp("", "eggstore-cow-range-")
			if err != nil {
				return err
			}
			defer func() { _ = os.RemoveAll(dir) }()
			src := filepath.Join(dir, "egg.bin")
			dst := filepath.Join(dir, "seg.bin")
			payload := cowPayload(4096)
			if err := os.WriteFile(src, payload, 0o644); err != nil {
				return err
			}
			r := ByteRange{Offset: 500, Length: 1500}
			res, err := CloneSegment(dst, src, r, CowOptions{})
			if err != nil {
				return fmt.Errorf("段取用应成功：%w", err)
			}
			if !res.Tier.Valid() {
				return fmt.Errorf("档位 %q 不在闭集内", res.Tier)
			}
			// macOS 上段档**必然**是 io.Copy（无段级 reflink）—— 这一格是口径的牙。
			if res.Tier == TierClonefile {
				return fmt.Errorf("macOS clonefile 一律整文件 ⇒ 段档不可能报 clonefile")
			}
			return sameBytes(dst, payload[500:2000])
		}},
		{Name: "负控·严格档（MustClone）跨卷 clone ⇒ 拒（✗ 不静默降级成拷贝）", Negative: true, Optional: true, Run: func() error {
			other, ok := secondVolumeDir()
			if !ok {
				return fmt.Errorf("%w：本机只有一个卷", ErrSkip)
			}
			boot, err := os.MkdirTemp("", "eggstore-cow-xvol-")
			if err != nil {
				return err
			}
			defer func() { _ = os.RemoveAll(boot) }()
			otherDir := filepath.Join(other, "eggstore-cow-xvol")
			if err := os.MkdirAll(otherDir, 0o755); err != nil {
				return err
			}
			defer func() { _ = os.RemoveAll(otherDir) }()
			src := filepath.Join(boot, "src.bin")
			if err := os.WriteFile(src, cowPayload(4096), 0o644); err != nil {
				return err
			}
			dst := filepath.Join(otherDir, "clone.bin")
			if same, serr := SameVolume(src, dst); serr == nil && same {
				return fmt.Errorf("%w：本机两卷被判定成同卷", ErrSkip)
			}
			_, cerr := CloneWholeFile(dst, src, CowOptions{MustClone: true})
			if cerr == nil {
				return fmt.Errorf("严格档跨卷 clone 本该被拒")
			}
			if !errors.Is(cerr, ErrCloneCrossVolume) && !errors.Is(cerr, ErrNoReflink) {
				return fmt.Errorf("拒的形态不对（应 ErrCloneCrossVolume / ErrNoReflink）：%w", cerr)
			}
			if _, serr := os.Stat(dst); !os.IsNotExist(serr) {
				return fmt.Errorf("被拒时不该留下目标件（err=%v）", serr)
			}
			return nil
		}},
		{Name: "负控·引用计数未归零 ⇒ DeleteIfZero 拒（✗ 不许真删）", Negative: true, Run: func() error {
			s, dir, cleanup, err := newSelfStore()
			if err != nil {
				return err
			}
			defer cleanup()
			eggFile := filepath.Join(dir, "nc.egg")
			if err := os.WriteFile(eggFile, []byte("zz"), 0o644); err != nil {
				return err
			}
			if err := s.registerSelf("nc", eggFile); err != nil {
				return err
			}
			if err := s.Refs.Acquire("nc", "exp-1", RefExpansion); err != nil {
				return err
			}
			if _, derr := s.DeleteIfZero("nc"); derr == nil {
				return fmt.Errorf("引用未归零时本该拒删")
			}
			if _, serr := os.Stat(eggFile); serr != nil {
				return fmt.Errorf("被拒时件不该被删：%v", serr)
			}
			return nil
		}},
		{Name: "正控·展开登记进同一引用图 ⇒ GC 钉住不删", Run: func() error {
			s, dir, cleanup, err := newSelfStore()
			if err != nil {
				return err
			}
			defer cleanup()
			eggFile := filepath.Join(dir, "e1.egg")
			if err := os.WriteFile(eggFile, cowPayload(64<<10), 0o644); err != nil {
				return err
			}
			if err := s.registerSelf("e1", eggFile); err != nil {
				return err
			}
			expPath := filepath.Join(dir, "exp1.bin")
			rep, err := s.Expand("e1", "exp-1", expPath, RefExpansion, CowOptions{})
			if err != nil {
				return fmt.Errorf("展开登记失败：%w", err)
			}
			if rep.RefsAfter != 1 {
				return fmt.Errorf("展开后引用计数应为 1，实 %d", rep.RefsAfter)
			}
			if !rep.Result.Tier.Valid() {
				return fmt.Errorf("档位不合法")
			}
			gcrep, err := s.GC(false)
			if err != nil {
				return err
			}
			if len(gcrep.Deleted) != 0 {
				return fmt.Errorf("被展开物引用时 GC 不该删，却删了 %v", gcrep.Deleted)
			}
			if len(gcrep.Pinned) != 1 {
				return fmt.Errorf("应被钉住 1 枚，实 %+v", gcrep.Pinned)
			}
			if _, serr := os.Stat(eggFile); serr != nil {
				return fmt.Errorf("卵件不该被删：%v", serr)
			}
			return nil
		}},
		{Name: "负控·展开登记引用种类闭集外 ⇒ 拒", Negative: true, Run: func() error {
			s, dir, cleanup, err := newSelfStore()
			if err != nil {
				return err
			}
			defer cleanup()
			eggFile := filepath.Join(dir, "e2.egg")
			if err := os.WriteFile(eggFile, []byte("q"), 0o644); err != nil {
				return err
			}
			if err := s.registerSelf("e2", eggFile); err != nil {
				return err
			}
			if _, eerr := s.Expand("e2", "r", filepath.Join(dir, "x.bin"), RefKind("snapshot"), CowOptions{}); eerr == nil {
				return fmt.Errorf("引用种类闭集外本该被拒")
			}
			return nil
		}},
		{Name: "正控·解引用（删件＋摘引用）⇒ 计数归零", Run: func() error {
			s, dir, cleanup, err := newSelfStore()
			if err != nil {
				return err
			}
			defer cleanup()
			eggFile := filepath.Join(dir, "e3.egg")
			if err := os.WriteFile(eggFile, cowPayload(32<<10), 0o644); err != nil {
				return err
			}
			if err := s.registerSelf("e3", eggFile); err != nil {
				return err
			}
			expPath := filepath.Join(dir, "exp3.bin")
			if _, err := s.Expand("e3", "clone-1", expPath, RefClone, CowOptions{}); err != nil {
				return err
			}
			rr, err := s.ReleaseExpansion("e3", "clone-1", expPath)
			if err != nil {
				return err
			}
			if !rr.Released || !rr.Zero || rr.RefsLeft != 0 {
				return fmt.Errorf("解引用后应归零：%+v", rr)
			}
			if rr.FreedSource != FreedSourceDF {
				return fmt.Errorf("报数来源应为 %q，实 %q", FreedSourceDF, rr.FreedSource)
			}
			if _, serr := os.Stat(expPath); !os.IsNotExist(serr) {
				return fmt.Errorf("副本件应已删（err=%v）", serr)
			}
			return nil
		}},
		{Name: "正控·硬链共享同一 inode（os.SameFile）", Run: func() error {
			dir, err := os.MkdirTemp("", "eggstore-hl-")
			if err != nil {
				return err
			}
			defer func() { _ = os.RemoveAll(dir) }()
			src := filepath.Join(dir, "engine.bin")
			dst := filepath.Join(dir, "engine.link")
			if err := os.WriteFile(src, []byte("engine-seg"), 0o644); err != nil {
				return err
			}
			if err := HardlinkShare(dst, src); err != nil {
				return err
			}
			f, err := ReadHardlinkFacts(src, dst)
			if err != nil {
				return err
			}
			if !f.SameInode {
				return fmt.Errorf("硬链应共享同一 inode")
			}
			if f.Size != int64(len("engine-seg")) {
				return fmt.Errorf("链上大小应同源，实 %d", f.Size)
			}
			return nil
		}},
		{Name: "负控·硬链目标已在盘 ⇒ 拒（✗ 不静默覆盖）", Negative: true, Run: func() error {
			dir, err := os.MkdirTemp("", "eggstore-hlexist-")
			if err != nil {
				return err
			}
			defer func() { _ = os.RemoveAll(dir) }()
			src := filepath.Join(dir, "a.bin")
			dst := filepath.Join(dir, "b.bin")
			if err := os.WriteFile(src, []byte("a"), 0o644); err != nil {
				return err
			}
			if err := os.WriteFile(dst, []byte("b"), 0o644); err != nil {
				return err
			}
			if err := HardlinkShare(dst, src); err == nil {
				return fmt.Errorf("目标已在盘时硬链本该被拒")
			}
			return nil
		}},
		{Name: "负控·硬链跨卷 ⇒ 拒", Negative: true, Optional: true, Run: func() error {
			other, ok := secondVolumeDir()
			if !ok {
				return fmt.Errorf("%w：本机只有一个卷", ErrSkip)
			}
			dir, err := os.MkdirTemp("", "eggstore-hlx-")
			if err != nil {
				return err
			}
			defer func() { _ = os.RemoveAll(dir) }()
			src := filepath.Join(dir, "a.bin")
			if err := os.WriteFile(src, []byte("a"), 0o644); err != nil {
				return err
			}
			dst := filepath.Join(other, "eggstore-hlx-link.bin")
			if same, serr := SameVolume(src, dst); serr == nil && same {
				return fmt.Errorf("%w：本机两卷被判定成同卷", ErrSkip)
			}
			if err := HardlinkShare(dst, src); !errors.Is(err, ErrHardlinkCrossVolume) {
				return fmt.Errorf("跨卷硬链本该被 ErrHardlinkCrossVolume 拒，实 %v", err)
			}
			return nil
		}},
		{Name: "正控·硬链不可变现读：写会串改 ⇒ 只在只读纪律下才用", Run: func() error {
			dir, err := os.MkdirTemp("", "eggstore-immut-")
			if err != nil {
				return err
			}
			defer func() { _ = os.RemoveAll(dir) }()
			p, err := ProbeHardlinkImmutability(dir, []byte("AAAA"))
			if err != nil {
				return err
			}
			if !p.SameInode {
				return fmt.Errorf("应共享同一 inode")
			}
			if !p.ContentIdentical {
				return fmt.Errorf("两链内容应逐字节同")
			}
			if !p.WriteThroughOneChangesOther {
				return fmt.Errorf("经一条链写应串改另一条链（硬链共享数据）——本格是「只读才用」的牙")
			}
			return nil
		}},
	}
}

// cowPayload 造确定性载荷（长度 n）。
func cowPayload(n int) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = byte(i*7 + 13)
	}
	return out
}

// sameBytes 读回 dst，与 want 逐字节比。
func sameBytes(path string, want []byte) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	got, err := io.ReadAll(f)
	if err != nil {
		return err
	}
	if !bytes.Equal(got, want) {
		return fmt.Errorf("内容不符：期望 %d B · 实读 %d B", len(want), len(got))
	}
	return nil
}
