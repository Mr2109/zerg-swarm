package eggstore

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// TestEggstoreCowSelfTest 自检口（含 cow_selftest.go 折进来的 CoW 格）。
func TestEggstoreCowSelfTest(t *testing.T) {
	passed, failed, skipped, lines := SelfTest()
	for _, l := range lines {
		t.Log(l)
	}
	t.Log(SelfTestSummary())
	if failed != 0 {
		t.Fatalf("自检有 %d 格失败（详见上面 FAIL 行）", failed)
	}
	if passed == 0 {
		t.Fatalf("自检一格都没跑 ⇒ 恒绿（✗ 不给结论）")
	}
	t.Logf("passed=%d failed=%d skipped=%d", passed, failed, skipped)
}

// ─────────────── 四态亲测（真跑 · 真 df 读数 · ✗ 不合成数字）───────────────
//
// ★ 四态是**地面读数**：① clonefile 展开（df 前后 ＋ 耗时）② 同卵两实例共享（df 不变）
// ③ 删一份 ⇒ df 释放 0 / 删最后一份 ⇒ df 才释放（实锤「已删卵 ≠ 已释放」）
// ④ Linux 分支（交叉编译 + vet；负控在本文件真跑）。
func TestEggstoreCowFourStates(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("四态①②③ 的真跑读数在 macOS 上采（Linux 走交叉编译 + vet，见报告 ④）")
	}
	root := t.TempDir()
	v0, err := ReadVolume(root)
	if err != nil {
		t.Fatalf("卷事实取不到：%v", err)
	}
	t.Logf("取证卷：%s（卷 %s · %s · Bsize=%d）", root, v0.Mount, v0.FSType, v0.Bsize)

	const size = int64(64 << 20) // 64 MiB ⇒ df 能看见
	src := filepath.Join(root, "big.egg")
	if err := writeBig(src, size); err != nil {
		t.Fatalf("造大件失败：%v", err)
	}
	sum := shaOf(t, src)

	// ── 态 ①：clonefile 整文件展开 ───────────────────────────────────────
	dst1 := filepath.Join(root, "exp1.bin")
	dfA := availB(t, root)
	t0 := time.Now()
	res1, err := CloneWholeFile(dst1, src, CowOptions{})
	dur := time.Since(t0)
	if err != nil {
		t.Fatalf("整文件 clonefile 展开失败：%v", err)
	}
	dfB := availB(t, root)
	if shaOf(t, dst1) != sum {
		t.Fatalf("展开物内容与卵不符")
	}
	t.Logf("态① 整文件 clonefile 展开：档=%s · 共享块=%v · df前=%d B · df后=%d B · Δdf=%d B · 源名义=%d B · 墙钟=%.2f ms",
		res1.Tier, res1.CoW(), dfA, dfB, int64(dfB)-int64(dfA), size, float64(dur.Microseconds())/1000.0)
	t.Logf("      判据：Δdf ≈ 0（共享块）—— Δdf=%d B ⟷ 名义 %d B（比值 %.6f）",
		int64(dfB)-int64(dfA), size, float64(int64(dfB)-int64(dfA))/float64(size))

	// ── 态 ②：同卵两实例共享 ⇒ df 不变 ──────────────────────────────────
	dst2 := filepath.Join(root, "exp2.bin")
	dfC := availB(t, root)
	res2, err := CloneWholeFile(dst2, src, CowOptions{})
	if err != nil {
		t.Fatalf("第二份 clonefile 失败：%v", err)
	}
	dfD := availB(t, root)
	if shaOf(t, dst2) != sum {
		t.Fatalf("第二份展开物内容与卵不符")
	}
	hlPath := filepath.Join(root, "engine.link")
	dfE := availB(t, root)
	if err := HardlinkShare(hlPath, src); err != nil {
		t.Fatalf("硬链共享失败：%v", err)
	}
	dfF := availB(t, root)
	hlFacts, _ := ReadHardlinkFacts(src, hlPath)
	t.Logf("态② 同卵两实例共享：第二份 clonefile 档=%s · Δdf=%d B（共享块 ⇒ 应 ≈0）/ 名义 %d B",
		res2.Tier, int64(dfD)-int64(dfC), size)
	t.Logf("      硬链共享：SameInode=%v · nlink=%d · Δdf=%d B", hlFacts.SameInode, hlFacts.Nlink, int64(dfF)-int64(dfE))
	if shaOf(t, hlPath) != sum {
		t.Fatalf("硬链内容与源不符")
	}

	// ── 态 ③：删一份 ⇒ df 释放 0；删最后一份 ⇒ df 才释放 ────────────────
	dfG := availB(t, root)
	if err := os.Remove(dst1); err != nil {
		t.Fatalf("删第一份副本失败：%v", err)
	}
	dfH := availB(t, root)
	t.Logf("态③a 删一份副本（exp1，另有 exp2 + 硬链仍在）⇒ Δdf=%d B（★ 名义 %d B ⇒ 实锤「已删卵 ≠ 已释放」）",
		int64(dfH)-int64(dfG), size)

	if err := os.Remove(dst2); err != nil {
		t.Fatalf("删第二份副本失败：%v", err)
	}
	dfI := availB(t, root)
	t.Logf("态③b 再删一份副本（exp2，硬链仍在）⇒ Δdf=%d B", int64(dfI)-int64(dfH))

	if err := os.Remove(hlPath); err != nil {
		t.Fatalf("删硬链失败：%v", err)
	}
	dfJ := availB(t, root)
	t.Logf("态③c 删硬链（此时 src 仍是唯一持有者）⇒ Δdf=%d B", int64(dfJ)-int64(dfI))

	if err := os.Remove(src); err != nil {
		t.Fatalf("删最后一份失败：%v", err)
	}
	dfK := availB(t, root)
	t.Logf("态③d 删**最后一**份（src）⇒ Δdf=%d B（★ 这才释放，≈名义 %d B · 比值 %.4f）",
		int64(dfK)-int64(dfJ), size, float64(int64(dfK)-int64(dfJ))/float64(size))

	// ── 态 ④（负控·真跑）：跨卷严格档 ⇒ 拒 / 引用未归零 ⇒ 拒删 ──────────
	if other, ok := secondVolumeDir(); ok {
		crossDir := filepath.Join(other, "eggstore-four-state-xvol")
		if err := os.MkdirAll(crossDir, 0o755); err == nil {
			defer func() { _ = os.RemoveAll(crossDir) }()
			src2 := filepath.Join(root, "xvol-src.bin")
			_ = os.WriteFile(src2, []byte("x"), 0o644)
			crossDst := filepath.Join(crossDir, "clone.bin")
			_, cerr := CloneWholeFile(crossDst, src2, CowOptions{MustClone: true})
			t.Logf("态④a 负控·跨卷严格档 clone：err=%v ⇒ 拒=%v", cerr, cerr != nil && (errors.Is(cerr, ErrCloneCrossVolume) || errors.Is(cerr, ErrNoReflink)))
		}
	} else {
		t.Logf("态④a 负控·跨卷严格档 clone：本机只有一个卷 ⇒ 未真跑（见报告）")
	}

	s, dir, cleanup, err := newSelfStore()
	if err != nil {
		t.Fatalf("建自检库失败：%v", err)
	}
	defer cleanup()
	eggFile := filepath.Join(dir, "nc.egg")
	_ = os.WriteFile(eggFile, []byte("zz"), 0o644)
	if err := s.registerSelf("nc", eggFile); err != nil {
		t.Fatalf("登记失败：%v", err)
	}
	_ = s.Refs.Acquire("nc", "exp-1", RefExpansion)
	_, derr := s.DeleteIfZero("nc")
	_, serr := os.Stat(eggFile)
	t.Logf("态④b 负控·引用未归零 ⇒ 拒删：err=%v · 卵件仍在盘=%v", derr, serr == nil)
}

// ───────────────────────────── 小助手 ─────────────────────────────

func availB(t *testing.T, p string) uint64 {
	t.Helper()
	v, err := StatfsAvailable(p)
	if err != nil {
		t.Fatalf("df 读数失败（%s）：%v", p, err)
	}
	return v
}

func shaOf(t *testing.T, path string) string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("打开 %s 失败：%v", path, err)
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	buf := make([]byte, 1<<20)
	for {
		n, rerr := f.Read(buf)
		if n > 0 {
			h.Write(buf[:n])
		}
		if rerr != nil {
			break
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

// writeBig 真写真块（✗ 不用 Truncate —— 稀疏件不占块 ⇒ df 不动）。
func writeBig(path string, size int64) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	chunk := make([]byte, 1<<20)
	for i := range chunk {
		chunk[i] = byte(i * 31)
	}
	var written int64
	for written < size {
		n := int64(len(chunk))
		if size-written < n {
			n = size - written
		}
		if _, err := f.Write(chunk[:n]); err != nil {
			return err
		}
		written += n
	}
	if err := f.Sync(); err != nil {
		return err
	}
	return nil
}

var _ = fmt.Sprintf
