package eggstore

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ─────────────────────── 硬链共享（U4 · §十九 #16 / #17）───────────────────────
//
// ★ 拍定（用户 2026-10-15）：**展开物跨卵共享先走硬链（最省）**；`containerd fs/`
// symlink 思路**留作后手**。引擎段**同版**时，多枚卵的展开目录里那份引擎段可以
// **硬链**到同一 inode ⇒ 零新块、零新 inode。
//
// ★ 硬链 ⟷ 不可变（本单**现读**结论，见报告 U4 与 `ProbeHardlinkImmutability`）：
//   - 硬链 = **同一个 inode**（✗ 不是 CoW 的新 inode）⇒ **任意一条链上的写会串改全部链**；
//   - 但它**不破**「卵内容不可变」这条**不变量本身** —— 卵按 sha256 内容寻址、**从不原地写**
//     （`Egg.SHA256` 是身份面；改名/删除是换 inode，不是改 inode）；
//   - 它破的是「**展开区可写**」这个假设 ⇒ ★ 硬链**只在展开区只读**（§十九 #25：属主＝孵化器 ·
//     `0700` · 租户不可达 ＋ 只读挂点）时可用；✗ **不得**把可写缓存区（JIT/编译缓存）与硬链件
//     放同一目录。
//   - 另一处代价：硬链**共享 mode/属主**（`chmod` 一条 = 改全部链）⇒ 权限面必须一次定死。

// ErrHardlinkCrossVolume 硬链要求**同卷**（跨卷硬链在本平台不存在）⇒ 拒。
var ErrHardlinkCrossVolume = errors.New("硬链要求**同卷**（跨卷硬链不存在）⇒ 拒")

// HardlinkShare 把一个只读件（典型：引擎段）硬链到 dst。
//
// 前检（都 fail-closed）：同卷（`SameVolume`）· 目标**不在盘**（✗ 不静默覆盖）。
func HardlinkShare(dst, src string) error {
	if strings.TrimSpace(src) == "" || strings.TrimSpace(dst) == "" {
		return fmt.Errorf("硬链拒收：源 / 目标都不许为空")
	}
	same, err := SameVolume(src, dst)
	if err != nil {
		return fmt.Errorf("硬链同卷判定失败（✗ 判不了就不链）：%w", err)
	}
	if !same {
		return ErrHardlinkCrossVolume
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if _, err := os.Lstat(dst); err == nil {
		return fmt.Errorf("硬链目标已在盘（✗ 不静默覆盖）：%s", dst)
	}
	if err := os.Link(src, dst); err != nil {
		return fmt.Errorf("硬链失败：%w", err)
	}
	return nil
}

// HardlinkFacts 一对硬链件的现读事实。
type HardlinkFacts struct {
	Path      string `json:"path"`
	Link      string `json:"link"`
	SameInode bool   `json:"same_inode"`
	Nlink     uint64 `json:"nlink"`
	Size      int64  `json:"size"`
}

// ReadHardlinkFacts 现读「两名字是否同一个 inode」（`os.SameFile` = dev+ino 比较）。
func ReadHardlinkFacts(path, link string) (HardlinkFacts, error) {
	fa, err := os.Stat(path)
	if err != nil {
		return HardlinkFacts{}, err
	}
	fb, err := os.Stat(link)
	if err != nil {
		return HardlinkFacts{}, err
	}
	n, _ := hardlinkCount(link) // 本平台取不到 ⇒ 0（✗ 不假装）
	return HardlinkFacts{Path: path, Link: link, SameInode: os.SameFile(fa, fb), Nlink: n, Size: fb.Size()}, nil
}

// ImmutabilityProbe 现读确认「**硬链是否破不可变**」的读数。
type ImmutabilityProbe struct {
	Src        string `json:"src"`
	Dst        string `json:"dst"`
	SameInode  bool   `json:"same_inode"`
	NlinkAfter uint64 `json:"nlink_after"`
	// ContentIdentical 两条链读回的内容逐字节同？
	ContentIdentical bool `json:"content_identical"`
	// WriteThroughOneChangesOther ★ 关键格：经一条链写，另一条链**读回也变**？
	// true ⇒ 硬链**共享数据**（写会串改全部链）⇒ 只在**只读**纪律下才不破不可变。
	WriteThroughOneChangesOther bool   `json:"write_through_one_changes_other"`
	Verdict                     string `json:"verdict"`
}

// ProbeHardlinkImmutability 真造一对硬链、真写一次、真读回 —— **现读**回答
// 「硬链是否破不可变」（✗ 不靠背口径）。
func ProbeHardlinkImmutability(dir string, payload []byte) (ImmutabilityProbe, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return ImmutabilityProbe{}, err
	}
	src := filepath.Join(dir, "immut-src.bin")
	dst := filepath.Join(dir, "immut-link.bin")
	if err := os.WriteFile(src, payload, 0o644); err != nil {
		return ImmutabilityProbe{}, err
	}
	if err := HardlinkShare(dst, src); err != nil {
		return ImmutabilityProbe{}, err
	}
	facts, err := ReadHardlinkFacts(src, dst)
	if err != nil {
		return ImmutabilityProbe{}, err
	}
	back, err := os.ReadFile(dst)
	if err != nil {
		return ImmutabilityProbe{}, err
	}
	identical := string(back) == string(payload)

	// 经**一条链**写 ⇒ 读**另一条链**（证「共享数据」）。
	changed := false
	if len(payload) > 0 {
		fh, err := os.OpenFile(src, os.O_WRONLY, 0o644)
		if err != nil {
			return ImmutabilityProbe{}, err
		}
		if _, err := fh.WriteAt([]byte{payload[0] ^ 0xff}, 0); err != nil {
			_ = fh.Close()
			return ImmutabilityProbe{}, err
		}
		_ = fh.Close()
		after, err := os.ReadFile(dst)
		if err != nil {
			return ImmutabilityProbe{}, err
		}
		if len(after) > 0 && after[0] == payload[0]^0xff {
			changed = true
		}
	}
	p := ImmutabilityProbe{
		Src: src, Dst: dst, SameInode: facts.SameInode, NlinkAfter: facts.Nlink,
		ContentIdentical: identical, WriteThroughOneChangesOther: changed,
	}
	switch {
	case !facts.SameInode:
		p.Verdict = "未共享同一 inode ⇒ 这不是硬链（✗ 结论不成立）"
	case changed:
		p.Verdict = "硬链**共享同一 inode**：一条链上的写**串改全部链** ⇒ 「硬链**不破**卵内容不可变（卵按 sha 寻址、只读）**但破**『展开区可写』」⇒ ★ 只在展开区**只读**（0700 ＋ 只读挂点）时才用"
	default:
		p.Verdict = "写未串改（异常：硬链应共享数据）⇒ 结论存疑，✗ 不给「可用」结论"
	}
	return p, nil
}

// CowVsHardlink 硬链 ⟷ CoW 的取舍一行（供报告与人面复用同一句）。
func CowVsHardlink() string {
	return "取舍：**硬链** = 同 inode（零新块、**页缓存天然共享**；但 mode/属主共享、一条链的写串改全部）；" +
		"**CoW** = 新 inode（块共享、**页缓存不共享** W4；可在副本上独立改）。⇒ 只读的**引擎段同版**走硬链最省" +
		"（§十九 #16「先走硬链」）；需要各自独立可写的副本走 CoW。两者都被**同一引用计数**覆盖（#14）。"
}
