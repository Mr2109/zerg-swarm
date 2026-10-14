package eggstore

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// ─────────────────────────── CoW 展开（卵开工 · 卵3②）───────────────────────────
//
// # 落点（U1 · 现读选定 · 详见报告 U1 节）
//
// 本件**只加能力**，✗ 不改任何地基件的既有语义 —— 逐条复用现成面：
//   - `space.go`：`SameVolume()`（同卷判定）＋ `nearestExistingDir()`（终名未落盘也能判卷）；
//   - `atomic.go` / `landing.go`：`writeFileAtomic()` 与 `DoneMarker`（同一套落盘纪律）；
//   - `refs.go`：`RefGraph.Acquire/Release/Count/Zero` —— ★ 展开物/克隆副本**同一份引用计数**；
//   - `gc.go`：`Store.sample()`（df 探针）与 `FreedSourceDF` —— ★ 报数**只认 df**。
//
// # 口径（用户拍定 · 逐字照做）
//
//   - ★ **macOS 只有整文件 clonefile**（`clonefile(2)`）**没有**段级 reflink
//     （无 `FICLONERANGE` 等价物）⇒ 卵内**段范围**取用在 macOS 上**仍是一次真实拷贝**
//     （同量级秒，✗ **不是毫秒**）；「毫秒」只对**整文件 clonefile**成立。
//   - 回退链固定 **clone → copy_file_range → io.Copy**；`ENOTSUP/EOPNOTSUPP/EXDEV/ENOSYS/ENOTTY`
//     这五档「不支持」⇒ 回退；其余 errno（`ENOENT/EACCES/ENOSPC/EPERM/EINVAL/…`）
//     **必须上抛**、✗ 不许静默当拷贝（照 `go-fsctl/cowclone` 的 errno 分类口径）。

// CloneTier 本次「展开 / 克隆」**真走过的档** —— **闭集**（✗ 无第五档的暗路）。
type CloneTier string

const (
	// TierClonefile macOS 整文件 `clonefile(2)`（与卵共享物理块）。
	TierClonefile CloneTier = "clonefile"
	// TierReflink Linux 整文件 `FICLONE`（与卵共享物理块）。
	TierReflink CloneTier = "ficlone"
	// TierReflinkRange Linux 段范围 `FICLONERANGE`（与卵共享物理块）。
	TierReflinkRange CloneTier = "ficlonerange"
	// TierCopyRange `copy_file_range(2)`（内核直拷 · ✗ 不共享块）。
	TierCopyRange CloneTier = "copy_file_range"
	// TierByteCopy 用户态 `io.Copy`（真拷贝 · ✗ 不共享块）。
	TierByteCopy CloneTier = "io.Copy"
	// TierHardlink 硬链（**同一 inode** · 零新块 —— 见 hardlink.go）。
	TierHardlink CloneTier = "hardlink"
)

// CloneTiers 档位闭集（顺序稳定 ⇒ 人面/机器面输出可对拍）。
var CloneTiers = []CloneTier{
	TierClonefile, TierReflink, TierReflinkRange, TierCopyRange, TierByteCopy, TierHardlink,
}

// Valid 档位在闭集内？
func (t CloneTier) Valid() bool {
	for _, x := range CloneTiers {
		if t == x {
			return true
		}
	}
	return false
}

// SharedBlocks ★ 该档是否「与原文件**共享物理块**」—— 判据面是 **`df` 不涨**
// （✗ 不看 `du`：`du` 对共享块高估 10–100 倍，见 gc.go 的 `NominalBytesNote`）。
func (t CloneTier) SharedBlocks() bool {
	switch t {
	case TierClonefile, TierReflink, TierReflinkRange, TierHardlink:
		return true
	}
	return false
}

// RangeCapable 该档能否做**段范围**取用（现读面：只有 Linux `FICLONERANGE`）。
func (t CloneTier) RangeCapable() bool { return t == TierReflinkRange }

var (
	// ErrNoReflink 本平台/本卷不支持 clone（reflink）—— **严格档**（`MustClone`）下拒，
	// 非严格档退拷贝。
	ErrNoReflink = errors.New("本平台/本卷不支持 clone（reflink）—— 严格档（MustClone）下拒，非严格档退拷贝")
	// ErrNoRangeClone ★ **macOS 无段级 reflink** —— `clonefile(2)/clonefileat/fclonefileat` 一律
	// **整文件**、无 `FICLONERANGE` 等价物 ⇒ 卵内**段范围**取用只能真拷贝（首次展开**不是毫秒**）。
	// Linux（XFS/Btrfs）有 `FICLONERANGE`。
	ErrNoRangeClone = errors.New("本平台无「段级 reflink」（macOS clonefile 一律整文件）⇒ 段范围取用只能真拷贝")
	// ErrCopyRangeUnsupported 本平台/本内核无 `copy_file_range`（例：macOS 无此系统调用）。
	ErrCopyRangeUnsupported = errors.New("本平台/本内核不支持 copy_file_range")
	// ErrCloneCrossVolume 严格档下跨卷 clone —— **确定性拒**（✗ 不去撞 errno）。
	ErrCloneCrossVolume = errors.New("严格档（MustClone）下跨卷 clone ⇒ 拒（reflink 不可能跨卷）")
)

// isFallbackErr 该错是否属「本档不支持 ⇒ 允许回退」那一类。
//
// ★ 白名单式（**默认上抛**）：只有五个「不支持」errno ＋ 三枚本包哨兵算「可回退」；
// 其余一切（`ENOENT/EACCES/ENOSPC/EPERM/EINVAL/…`）**必须上抛**、✗ 不许静默当拷贝。
func isFallbackErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrNoReflink) || errors.Is(err, ErrNoRangeClone) || errors.Is(err, ErrCopyRangeUnsupported) {
		return true
	}
	for _, e := range fallbackErrnos() {
		if errors.Is(err, e) {
			return true
		}
	}
	return false
}

// ByteRange 段范围（卵内偏移 · ✗ 只对 `CloneSegment` 有意义）。
type ByteRange struct {
	Offset int64 `json:"offset"`
	Length int64 `json:"length"`
}

// Valid 段范围合法？（偏移非负、长度为正）。
func (r ByteRange) Valid() bool { return r.Offset >= 0 && r.Length > 0 }

// CowOptions 展开参数。
type CowOptions struct {
	// MustClone 严格档（＝ `cp --reflink=always` 语义）：只许**真 CoW / 硬链**，
	// ✗ 不许退字节拷贝；clone 不可用 ⇒ 拒（出声，✗ 不静默降级）。
	MustClone bool
	// ChunkSize `io.Copy` 档的缓冲（<=0 ⇒ 4 MiB）。
	ChunkSize int
	// Now 计时时钟（nil ⇒ time.Now）。
	Now func() time.Time
}

// CowResult 一次展开/克隆的读数（★ 已释放量**只认 df**，本结构里**没有**「已释放」格）。
type CowResult struct {
	Src   string     `json:"src"`
	Dst   string     `json:"dst"`
	Tier  CloneTier  `json:"tier"`
	Bytes int64      `json:"bytes"` // 真拷贝的字节数（共享块档 ⇒ 0）
	Whole bool       `json:"whole"` // 整文件？
	Range *ByteRange `json:"range,omitempty"`
	DurMS float64    `json:"dur_ms"`
	// FellBack clone 档失败并回退过？
	FellBack bool   `json:"fell_back"`
	Note     string `json:"note"`
}

// CoW 本次是否走了「共享物理块」档（真 CoW / 硬链）。
func (r CowResult) CoW() bool { return r.Tier.SharedBlocks() }

// Line 人面一行（✗ 不出现「已释放」这类词 —— 那是 df 的活）。
func (r CowResult) Line() string {
	scope := "整文件"
	if !r.Whole && r.Range != nil {
		scope = fmt.Sprintf("段[off=%d len=%d]", r.Range.Offset, r.Range.Length)
	}
	return fmt.Sprintf("%s → %s（%s · 档=%s · 共享块=%v · 真拷贝=%d B · 耗时=%.3f ms · 回退过=%v）%s",
		r.Src, r.Dst, scope, r.Tier, r.CoW(), r.Bytes, r.DurMS, r.FellBack, r.Note)
}

// CloneWholeFile 展开**整文件**：回退链 **clone → copy_file_range → io.Copy**。
//
// 顺序与判据：
//  1. **同卷判定先行**（`SameVolume`）：不同卷 ⇒ reflink 根本不可能（`EXDEV`），
//     ✗ 不去撞 errno —— 严格档直接拒，非严格档直接进拷贝档；
//  2. clone 档失败时**分流**：`isFallbackErr` ⇒ 删掉半成品目标、退下一档；
//     其余 errno ⇒ **上抛**（✗ 不许静默当拷贝）；
//  3. 末档 `io.Copy` 仍是「先落临时名再换名」之外的**真拷贝**，失败即删目标（✗ 不留半件）。
func CloneWholeFile(dst, src string, opt CowOptions) (CowResult, error) {
	res := CowResult{Src: src, Dst: dst, Whole: true}
	now := opt.Now
	if now == nil {
		now = time.Now
	}
	t0 := now()
	defer func() { res.DurMS = float64(now().Sub(t0).Microseconds()) / 1000.0 }()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return res, err
	}
	same, verr := SameVolume(src, dst)
	if verr != nil {
		return res, fmt.Errorf("同卷判定失败（✗ 判不了就不 clone）：%w", verr)
	}

	// ── 档 1：clone（共享块）────────────────────────────────────────────────
	if same {
		tier, cerr := platformCloneWhole(dst, src)
		switch {
		case cerr == nil:
			res.Tier = tier
			res.Note = fmt.Sprintf("整文件 CoW（%s）⇒ 与卵**共享物理块**、df 不涨（✗ 不用 du 估）", tier)
			return res, nil
		case isFallbackErr(cerr):
			res.FellBack = true
			_ = os.Remove(dst) // 半成品目标不留
		default:
			return res, fmt.Errorf("clone 拒（errno 非「不支持」档 ⇒ ✗ 不许静默当拷贝）：%w", cerr)
		}
	} else {
		res.FellBack = true
	}
	if opt.MustClone {
		if !same {
			return res, fmt.Errorf("%w（源/目标不同卷）", ErrCloneCrossVolume)
		}
		return res, fmt.Errorf("%w（严格档 MustClone：只许真 CoW / 硬链）", ErrNoReflink)
	}

	// ── 档 2：copy_file_range（内核直拷 · ✗ 不共享块）──────────────────────
	n, cerr := platformCopyRange(dst, 0, src, 0, 0)
	switch {
	case cerr == nil:
		res.Tier, res.Bytes = TierCopyRange, n
		res.Note = fmt.Sprintf("copy_file_range 档（内核直拷 %d B；✗ 不共享块）", n)
		return res, nil
	case isFallbackErr(cerr):
		res.FellBack = true
		_ = os.Remove(dst)
	default:
		return res, fmt.Errorf("copy_file_range 拒（errno 非「不支持」档 ⇒ 上抛）：%w", cerr)
	}

	// ── 档 3：io.Copy（用户态真拷贝）──────────────────────────────────────
	n2, err := byteCopyFile(dst, src, opt.ChunkSize)
	if err != nil {
		return res, err
	}
	res.Tier, res.Bytes = TierByteCopy, n2
	res.Note = "io.Copy 档（用户态真拷贝）"
	return res, nil
}

// CloneSegment 取用**卵内的一段范围**（卵是「固定前缀 ＋ 索引 ＋ 各段」的单文件 ⇒
// 整文件 clone 不适用，必须走**段范围**档）。
//
// ★★ 本平台差异（本单核心 · ✗ 不许写错）：
//   - **Linux**：`FICLONERANGE`（XFS/Btrfs）⇒ 与卵**共享块**、零字节拷贝；
//   - **macOS**：`clonefile(2)` **一律整文件**、**无段级 reflink** ⇒ 段范围取用
//     **必然落到 `io.Copy` 真拷贝**（同量级**秒**，**✗ 不是毫秒**）。
func CloneSegment(dst, src string, r ByteRange, opt CowOptions) (CowResult, error) {
	res := CowResult{Src: src, Dst: dst, Whole: false, Range: &r}
	now := opt.Now
	if now == nil {
		now = time.Now
	}
	t0 := now()
	defer func() { res.DurMS = float64(now().Sub(t0).Microseconds()) / 1000.0 }()

	if !r.Valid() {
		return res, fmt.Errorf("段范围不合法（offset=%d length=%d）⇒ 拒", r.Offset, r.Length)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return res, err
	}
	same, verr := SameVolume(src, dst)
	if verr != nil {
		return res, fmt.Errorf("同卷判定失败（✗ 判不了就不 clone）：%w", verr)
	}

	// ── 档 1：段级 reflink（只有 Linux 有）────────────────────────────────
	if same {
		tier, cerr := platformCloneRange(dst, 0, src, r.Offset, r.Length)
		switch {
		case cerr == nil:
			res.Tier = tier
			res.Note = "段级 reflink（FICLONERANGE）⇒ 与卵**共享块**、零字节拷贝"
			return res, nil
		case isFallbackErr(cerr):
			res.FellBack = true
			_ = os.Remove(dst)
		default:
			return res, fmt.Errorf("段级 reflink 拒（errno 非「不支持」档 ⇒ 上抛）：%w", cerr)
		}
	} else {
		res.FellBack = true
	}
	if opt.MustClone {
		return res, fmt.Errorf("%w（严格档下**段范围**取用不可 CoW）", ErrNoRangeClone)
	}

	// ── 档 2：copy_file_range 带偏移（只有 Linux 有）──────────────────────
	n, cerr := platformCopyRange(dst, 0, src, r.Offset, r.Length)
	switch {
	case cerr == nil:
		res.Tier, res.Bytes = TierCopyRange, n
		res.Note = fmt.Sprintf("copy_file_range 段档（%d B；✗ 不共享块）", n)
		return res, nil
	case isFallbackErr(cerr):
		res.FellBack = true
		_ = os.Remove(dst)
	default:
		return res, fmt.Errorf("段 copy_file_range 拒（errno 非「不支持」档 ⇒ 上抛）：%w", cerr)
	}

	// ── 档 3：io.Copy 段（★ macOS 必走此档 · **真拷贝**）──────────────────
	n2, err := byteCopySegment(dst, src, r, opt.ChunkSize)
	if err != nil {
		return res, err
	}
	res.Tier, res.Bytes = TierByteCopy, n2
	res.Note = "★ io.Copy 段档：macOS 无段级 reflink ⇒ 首次从卵取段**是一次真实拷贝**（✗ 不是毫秒）"
	return res, nil
}

// byteCopyFile 用户态整文件拷贝（末档）。
func byteCopyFile(dst, src string, chunk int) (int64, error) {
	if chunk <= 0 {
		chunk = 4 << 20
	}
	in, err := os.Open(src)
	if err != nil {
		return 0, fmt.Errorf("末档拷贝打开源失败：%w", err)
	}
	defer func() { _ = in.Close() }()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return 0, fmt.Errorf("末档拷贝建目标失败：%w", err)
	}
	keep := false
	defer func() {
		if !keep {
			_ = os.Remove(dst)
		}
	}()
	n, err := io.CopyBuffer(out, in, make([]byte, chunk))
	if err != nil {
		_ = out.Close()
		return n, fmt.Errorf("末档拷贝中途失败（半件已弃）：%w", err)
	}
	if err := out.Sync(); err != nil {
		_ = out.Close()
		return n, err
	}
	if err := out.Close(); err != nil {
		return n, err
	}
	keep = true
	return n, nil
}

// byteCopySegment 用户态段拷贝（macOS 的段取用必走此档）。
func byteCopySegment(dst, src string, r ByteRange, chunk int) (int64, error) {
	if chunk <= 0 {
		chunk = 4 << 20
	}
	in, err := os.Open(src)
	if err != nil {
		return 0, fmt.Errorf("段拷贝打开源失败：%w", err)
	}
	defer func() { _ = in.Close() }()
	if _, err := in.Seek(r.Offset, io.SeekStart); err != nil {
		return 0, fmt.Errorf("段拷贝定位失败（offset=%d）：%w", r.Offset, err)
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return 0, fmt.Errorf("段拷贝建目标失败：%w", err)
	}
	keep := false
	defer func() {
		if !keep {
			_ = os.Remove(dst)
		}
	}()
	n, err := io.CopyBuffer(out, io.LimitReader(in, r.Length), make([]byte, chunk))
	if err != nil {
		_ = out.Close()
		return n, fmt.Errorf("段拷贝中途失败（半件已弃）：%w", err)
	}
	if err := out.Sync(); err != nil {
		_ = out.Close()
		return n, err
	}
	if err := out.Close(); err != nil {
		return n, err
	}
	keep = true
	return n, nil
}
