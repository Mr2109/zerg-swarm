package eggstore

import (
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"strings"
)

// ErrNoVolumeFacts 本平台拿不到 statfs 事实 —— ★ fail-closed 出声，✗ 不假装判过。
var ErrNoVolumeFacts = errors.New("本平台拿不到 statfs 事实（可用空间/可用 inode）——✗ 不给结论")

// VolumeFacts 一处路径所在卷的**现读**事实。
type VolumeFacts struct {
	Path   string `json:"path"`
	Mount  string `json:"mount"`
	FSType string `json:"fstype"`
	Bsize  uint64 `json:"bsize"`
	Blocks uint64 `json:"blocks"`
	// Bfree 全部空闲块（含特权保留）—— ★ 只作对拍，**不作判据**
	//（Linux ext4 默认保留 5% ⇒ 用 Bfree 会高估可用空间，判据必须走 Bavail）。
	Bfree uint64 `json:"bfree"`
	// Bavail **非特权用户可用块** —— ★ 唯一判据来源（`df` 口径）。
	Bavail uint64 `json:"bavail"`
	Files  uint64 `json:"files"`
	// Ffree 可用 inode（Linux 侧即 `statvfs.f_favail`；**独立**查，不从别的量推）。
	Ffree uint64 `json:"ffree"`

	// ── macOS 专属补充：APFS 容器 ────────────────────────────────────────────
	// 同一 APFS 容器上多卷**共享一个空闲池** ⇒ 逐卷 `df` 会各报不同数字，而容器实为一池。
	// ★ 水线判据**只认 df（Bavail）**；本格只作「容器级差异」标注；取不到 ⇒ 留空、✗ 不假装。
	APFSContainerFree *uint64 `json:"apfs_container_free,omitempty"`
	APFSContainerRef  string  `json:"apfs_container_ref,omitempty"`
	APFSProbeNote     string  `json:"apfs_probe_note,omitempty"`
}

// Sample 纯数值样本（判据入口取它 ⇒ 负控可用合成数字确定性复算，不依赖真盘）。
type Sample struct {
	Label  string `json:"label"`
	Bsize  uint64 `json:"bsize"`
	Blocks uint64 `json:"blocks"`
	Bfree  uint64 `json:"bfree"`
	Bavail uint64 `json:"bavail"`
	Files  uint64 `json:"files"`
	Ffree  uint64 `json:"ffree"`
}

// AvailableBytes 非特权可用字节（`df` 口径 · 地面真值）。
func (v Sample) AvailableBytes() uint64 { return v.Bavail * v.Bsize }

// FreeBytes 全部空闲字节（含保留）—— 仅供负控对拍（★ 不作判据）。
func (v Sample) FreeBytes() uint64 { return v.Bfree * v.Bsize }

// UsedPercent 已用百分比 = `(Blocks − Bavail) / Blocks`（★ 用 Bavail，不用 Bfree）。
func (v Sample) UsedPercent() float64 {
	if v.Blocks == 0 {
		return 0
	}
	return 100 * float64(v.Blocks-v.Bavail) / float64(v.Blocks)
}

// AvailablePercent 可用百分比（＝ 100 − UsedPercent）。
func (v Sample) AvailablePercent() float64 { return 100 - v.UsedPercent() }

// FreePercentOfBfree 用 `Bfree` 算的可用百分比 —— ★ **只给负控用**：
// 它就是「若拿 f_bfree 当判据」的那条错路，自检拿它证明判据真按 Bavail 走。
func (v Sample) FreePercentOfBfree() float64 {
	if v.Blocks == 0 {
		return 0
	}
	return 100 * float64(v.Bfree) / float64(v.Blocks)
}

// InodesFreePercent 可用 inode 百分比（`Ffree/Files`；Files=0 ⇒ 该卷不报 inode ⇒ 0 且视为不适用）。
func (v Sample) InodesFreePercent() float64 {
	if v.Files == 0 {
		return 0
	}
	return 100 * float64(v.Ffree) / float64(v.Files)
}

// Sample 取纯数值样本。
func (v VolumeFacts) Sample(label string) Sample {
	return Sample{Label: label, Bsize: v.Bsize, Blocks: v.Blocks, Bfree: v.Bfree,
		Bavail: v.Bavail, Files: v.Files, Ffree: v.Ffree}
}

// AvailableBytes 非特权可用字节（地面真值）。
func (v VolumeFacts) AvailableBytes() uint64 { return v.Bavail * v.Bsize }

// Line 人面一行（把两个口径都摆出来，便于一眼看出我们没有偷换 Bfree）。
func (v VolumeFacts) Line() string {
	s := v.Sample("")
	mount := v.Mount
	if mount == "" {
		mount = "?"
	}
	out := fmt.Sprintf("%s（卷 %s · %s）：可用 %.2f%%（Bavail=%d×%dB）· 全部空闲 %.2f%%（Bfree，仅对拍）· inode 可用 %.2f%%（Ffree/Files=%d/%d）",
		v.Path, mount, fsTypeOr(v.FSType), s.AvailablePercent(), v.Bavail, v.Bsize,
		s.FreePercentOfBfree(), s.InodesFreePercent(), v.Ffree, v.Files)
	if v.APFSContainerFree != nil {
		out += fmt.Sprintf(" · APFS 容器空闲 %d B（%s）", *v.APFSContainerFree, v.APFSContainerRef)
	} else if v.APFSProbeNote != "" {
		out += " · APFS 容器：" + v.APFSProbeNote
	}
	return out
}

func fsTypeOr(t string) string {
	if strings.TrimSpace(t) == "" {
		return "未报型（✗ 不假装判过）"
	}
	return t
}

// ReadVolume 现读一处路径所在卷。
func ReadVolume(path string) (VolumeFacts, error) {
	dir, err := nearestExistingDir(path)
	if err != nil {
		return VolumeFacts{}, fmt.Errorf("取卷事实失败：%s 及其祖先都不在盘（%w）", path, err)
	}
	raw, err := statfsRaw(dir)
	if err != nil {
		return VolumeFacts{}, err
	}
	v := VolumeFacts{
		Path: path, Mount: raw.mount, FSType: raw.fstype,
		Bsize: raw.bsize, Blocks: raw.blocks, Bfree: raw.bfree,
		Bavail: raw.bavail, Files: raw.files, Ffree: raw.ffree,
	}
	free, ref, note := platformAPFSProbe(raw.mount)
	v.APFSContainerFree, v.APFSContainerRef, v.APFSProbeNote = free, ref, note
	return v, nil
}

// ─────────────────────────── 水线与预检（S4）───────────────────────────

// Waterline 水线（照 k8s 默认）。
//
// 「高 85 / 低 80」与「硬线 imagefs<15%」是**同一条线的两种写法**
// （已用 85% ⇔ 可用 15%）：本实现把两者都算出来、都摆进判词，✗ 不靠换算蒙人。
type Waterline struct {
	HighUsedPercent     float64 `json:"high_used_percent"`
	LowUsedPercent      float64 `json:"low_used_percent"`
	ImageFSAvailPercent float64 `json:"imagefs_avail_percent"`
	NodeFSAvailPercent  float64 `json:"nodefs_avail_percent"`
	InodesFreePercent   float64 `json:"inodes_free_percent"`
}

// DefaultWaterline 默认水线：高 85 / 低 80 ＋ 硬线 15 / 10 / 5。
var DefaultWaterline = Waterline{
	HighUsedPercent:     85,
	LowUsedPercent:      80,
	ImageFSAvailPercent: 15,
	NodeFSAvailPercent:  10,
	InodesFreePercent:   5,
}

// 判词三档（闭集）。
const (
	VerdictAllow   = "allow"         // 放行
	VerdictReclaim = "reclaim-first" // 到低水线 ⇒ 先回收（本版不拒，但出声 + 给清单）
	VerdictRefuse  = "refuse"        // 到高水线 / 硬线 ⇒ 拒新孵化（✗ 不是静默继续）
)

// Precheck 孵化前预检结论。
type Precheck struct {
	Verdict     string   `json:"verdict"`
	Reasons     []string `json:"reasons"`
	Reclaimable []string `json:"reclaimable"`
	ImageFS     Sample   `json:"imagefs"`
	NodeFS      Sample   `json:"nodefs"`
	Waterline   Waterline
}

// Line 人面一行。
func (p Precheck) Line() string {
	return fmt.Sprintf("预检 %s · imagefs %s · nodefs %s%s",
		p.Verdict, p.ImageFS.Label, p.NodeFS.Label, func() string {
			if len(p.Reasons) == 0 {
				return ""
			}
			return " · " + strings.Join(p.Reasons, "；")
		}())
}

// EvalWaterline 判水线（**唯一判据入口** —— 自检与负控都打这里）。
func EvalWaterline(w Waterline, image, node Sample, reclaimable []string) Precheck {
	p := Precheck{Verdict: VerdictAllow, ImageFS: image, NodeFS: node, Waterline: w,
		Reclaimable: append([]string{}, reclaimable...)}
	refuse := func(format string, args ...interface{}) {
		p.Verdict = VerdictRefuse
		p.Reasons = append(p.Reasons, fmt.Sprintf(format, args...))
	}
	if image.UsedPercent() >= w.HighUsedPercent {
		refuse("imagefs 已用 %.2f%% ≥ 高水线 %.0f%%（同一件事的另一面：可用 %.2f%% < 硬线 %.0f%%）",
			image.UsedPercent(), w.HighUsedPercent, image.AvailablePercent(), w.ImageFSAvailPercent)
	} else if image.AvailablePercent() < w.ImageFSAvailPercent {
		refuse("imagefs 可用 %.2f%% < 硬线 %.0f%%", image.AvailablePercent(), w.ImageFSAvailPercent)
	}
	if node.AvailablePercent() < w.NodeFSAvailPercent {
		refuse("nodefs 可用 %.2f%% < 硬线 %.0f%%", node.AvailablePercent(), w.NodeFSAvailPercent)
	}
	if image.Files > 0 && image.InodesFreePercent() < w.InodesFreePercent {
		refuse("imagefs inode 可用 %.2f%% < 硬线 %.0f%%（Ffree=%d / Files=%d）",
			image.InodesFreePercent(), w.InodesFreePercent, image.Ffree, image.Files)
	}
	if node.Files > 0 && node.InodesFreePercent() < w.InodesFreePercent {
		refuse("nodefs inode 可用 %.2f%% < 硬线 %.0f%%（Ffree=%d / Files=%d）",
			node.InodesFreePercent(), w.InodesFreePercent, node.Ffree, node.Files)
	}
	if p.Verdict == VerdictRefuse {
		return p
	}
	if image.UsedPercent() >= w.LowUsedPercent {
		p.Verdict = VerdictReclaim
		p.Reasons = append(p.Reasons, fmt.Sprintf(
			"imagefs 已用 %.2f%% ≥ 低水线 %.0f%% ⇒ 先回收（本版不拒，但出声 + 给可回收卵清单）",
			image.UsedPercent(), w.LowUsedPercent))
	}
	return p
}

// ReclaimableList 名册 + 引用图上的可回收卵（引用计数归零才入列）。
func (s *Store) ReclaimableList() []string { return s.Roster.ReclaimableNames(s.Refs) }

// PrecheckHatch 孵化**前**的盘面预检（水线 + 硬线 + inode）。
func (s *Store) PrecheckHatch(imagePath, nodePath string) (Precheck, error) {
	img, err := ReadVolume(imagePath)
	if err != nil {
		return Precheck{}, fmt.Errorf("盘满预检取数失败（✗ 取不到就不放行）：%w", err)
	}
	node, err := ReadVolume(nodePath)
	if err != nil {
		return Precheck{}, fmt.Errorf("盘满预检取数失败（✗ 取不到就不放行）：%w", err)
	}
	return EvalWaterline(s.waterline(), img.Sample("imagefs"), node.Sample("nodefs"), s.ReclaimableList()), nil
}

// WaterlineNow 本轮生效的水线（可注入；✗ 不改默认档的值）。
func (s *Store) WaterlineNow() Waterline { return s.waterline() }

func (s *Store) waterline() Waterline {
	if s.Waterline != nil {
		return *s.Waterline
	}
	return DefaultWaterline
}

// HatchGate 孵化入口：**只有 `verified` 可孵** ＋ 水线预检。
//
// 两道闸都 fail-closed、都出声；`refuse` 时把**可回收卵清单**一并带出
// （✗ 不是静默继续）。
func (s *Store) HatchGate(eggName, imagePath, nodePath string) (Precheck, error) {
	e, ok := s.Roster.Get(eggName)
	if !ok {
		return Precheck{}, &RefuseError{Egg: eggName, Reason: "名册里没有这枚卵"}
	}
	if !e.Complete {
		return Precheck{}, &RefuseError{Egg: eggName, State: e.State,
			Reason: "半成品：完工标记不是「已完成」（✗ 目录/文件存在 ≠ 可用）"}
	}
	if err := CheckHatchable(eggName, e.State); err != nil {
		return Precheck{}, err
	}
	p, err := s.PrecheckHatch(imagePath, nodePath)
	if err != nil {
		return Precheck{}, err
	}
	if p.Verdict == VerdictRefuse {
		return p, &RefuseError{Egg: eggName, State: e.State, Reason: fmt.Sprintf(
			"盘面到水线/硬线 ⇒ 拒新孵化：%s；可回收卵清单：%s",
			strings.Join(p.Reasons, "；"), listOrNone(p.Reclaimable))}
	}
	return p, nil
}

func listOrNone(xs []string) string {
	if len(xs) == 0 {
		return "（无）"
	}
	return strings.Join(xs, " / ")
}

// VolumeKeyOf 取一处路径所在卷的设备号（同卷判定用）。
func VolumeKeyOf(path string) (uint64, error) {
	dir, err := nearestExistingDir(path)
	if err != nil {
		return 0, err
	}
	return deviceOf(dir)
}

// SameVolume 两处路径是否同卷（★ `rename(2)` 的原子性只在同 mount 内成立）。
func SameVolume(a, b string) (bool, error) {
	ka, err := VolumeKeyOf(a)
	if err != nil {
		return false, err
	}
	kb, err := VolumeKeyOf(b)
	if err != nil {
		return false, err
	}
	return ka == kb, nil
}

// BytesToGiB 人面格式化（只读展示，✗ 不参与判据）。
func BytesToGiB(b int64) float64 { return math.Round(float64(b)/1024/1024/1024*100) / 100 }

// JoinPath 小助手：把目录与名拼起来（保持调用方一眼可读）。
func JoinPath(dir, name string) string { return filepath.Join(dir, name) }
