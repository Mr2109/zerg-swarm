// rm.go — 茧根的安全递归删除（A5）：fd 相对、永不拼路径、不跟链接、先留墓碑。
//
// ★ 为什么不能用 os.RemoveAll / filepath.WalkDir：
//
//	它们都是**路径**递归：先 lstat 判型，再按拼出来的路径逐层动手。判型与动手之间那条
//	缝里，树内一个 `link → $HOME` 可以被换成真目录、一个普通子目录可以被换成别的挂载点，
//	删除就落到茧根之外——`$HOME` 被清空，而茧根目录自己安然无恙（这正是"负控"要钉的行为）。
//	本文件把这条链整段改写成 **fd 相对**：删一个条目的唯一手段是 `unlinkat(dirFd, name)`，
//	`dirFd` 只可能来自"上一层的目录 fd + 一个不含 / 的名字"，全程没有字符串拼接，
//	也就没有"重新解析路径"的窗口——在内核眼里这个名字属于哪个目录，从打开那一刻就定死了。
//
// ★ 三层防护（叠加，缺一层都能被绕过）：
//
//	① 不跟链接：Linux `RESOLVE_NO_SYMLINKS` + 每层 `O_NOFOLLOW`；macOS 每层
//	  `O_NOFOLLOW_ANY`（整条路径都不许有链接）。**链接条目只删链接本身**，目标一个字节不碰。
//	  ELOOP 因此有两种落点，别混：**开茧根**时 ELOOP ⇒ ErrRootNotDir（拒删，绝不跟到目标上）；
//	  **开子条目**时 ELOOP ⇒ errNotDir（它就是那个链接，unlink 掉名字即可）。
//	② 不跨挂载：Linux `RESOLVE_NO_XDEV`（内核直接拒）；macOS 无等价物 ⇒ 每层 fstat 比
//	  st_dev（与茧根不同即拒）。两处方向一致：**fail-closed**——EXDEV（以及任何不是
//	  "这条目就是个链接/普通文件"的错）一律返回错误，绝不"跳过这个条目、把剩下的删完"；
//	  跳过等于把"删了一半"当成功报上去。
//	③ 不出茧根：Linux `RESOLVE_BENEATH`（`..`/绝对路径即拒）；macOS 靠"每层只许一个
//	  名字" + 每层 O_NOFOLLOW_ANY 等价成立（名字里带 / 在这里就被 validEntryName 挡掉）。
//
// ★ 身份绑定（st_dev/st_ino）：
//
//	入参的 id 只是一个**名字**，名字与"此刻它指向的那个目录"之间没有任何绑定：别的进程
//	可以在两次操作之间换掉它（rename 一个同名目录上来，或把它改成指向 $HOME 的链接）。
//	所以开根之后第一件事是 fstat 比 st_dev/st_ino，与调用方给的期望值（RootID，通常来自
//	登记时的 StatRoot）不符 ⇒ 一个字节都不删。
//
// ★ 顺序（与 meta.go 抬头 ③④ 一致，不许调换）：
//
//	UpdatePhase(deleting) → WriteTombstone（目录一没，`<id>/meta.json` 就跟着没）
//	→ 才是本文件的删树。RemoveCocoon 把"留墓碑"收进来并要求它**先成功**：墓碑写不下去
//	（盘满/只读/base 权限不够）就一个字节都不删——不许出现"删得干干净净、盘上没有任何
//	证据说明这个茧存在过"的状态。
//
// ★ 降级必须显式声明（不许静默）：
//
//	内核 <5.6 或 seccomp 拦掉 openat2（ENOSYS/EPERM）⇒ 退到逐分量 `openat(O_NOFOLLOW)`
//	+ 自比 st_dev。这一档**放弃 bind-mount 防护**（bind mount 与被挂载点同 st_dev，
//	比不出来；只有 NO_XDEV 拦得住）。降级这件事不许只活在注释里：RemoveReport.Degraded
//	带着原话，且所有返回的错误里都再缀一次。
package cocoon

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// 删除面的错误分类。调用方靠 errors.Is 判，别去比字符串。
var (
	// ErrCrossMount — 茧根或某个子目录在别的挂载点上：拒删（fail-closed）。
	ErrCrossMount = errors.New("cocoon: 跨挂载点，拒删")
	// ErrRootIdentity — 茧根在盘上的 st_dev/st_ino 与调用方给的期望值不符：拒删。
	ErrRootIdentity = errors.New("cocoon: 茧根身份与期望不符，拒删")
	// ErrRootNotDir — 茧根本身是符号链接或不是目录：拒删（绝不跟到目标上去）。
	ErrRootNotDir = errors.New("cocoon: 茧根不是目录（或为符号链接），拒删")
	// ErrRootChanged — 开根过程中即发现被换掉（O_PATH fd 与可读 fd 身份不一致）。
	ErrRootChanged = errors.New("cocoon: 茧根在打开过程中被换掉，拒删")
	// ErrUnsafeName — 名字非法（空 / . / .. / 含路径分隔符 / 含 NUL）。
	ErrUnsafeName = errors.New("cocoon: 名字非法")
	// ErrNeedTombstone — 墓碑没留成功，删除被整体拒绝（顺序见抬头 ★顺序）。
	ErrNeedTombstone = errors.New("cocoon: 墓碑未留，拒绝删除")

	// errNotDir — 平台层内部信号：条目存在但**不是目录**（链接/普通文件/设备/fifo）。
	// 不是错误面上的东西：上层收到它就把这个名字 unlink 掉（不跟）。
	errNotDir = errors.New("cocoon: 非目录条目")
)

// RootID — 目录身份（st_dev + st_ino）。
//
// 为什么要它：id 是名字，RootID 才指向"那一个目录"。登记时用 StatRoot 取一份记在账上，
// 删除时作为 want 传回来，两次之间被换掉就能当场认出（否则"删的到底是哪个目录"无从判定）。
type RootID struct {
	Dev uint64
	Ino uint64
}

// Pinned — 是否带上了可比的期望值（两个字段都非零）。
// 平台不提供时恒为零值（见 rm_other.go）⇒ 调用方应当知道"这次没绑身份"。
func (r RootID) Pinned() bool { return r.Dev != 0 && r.Ino != 0 }

func (r RootID) String() string { return fmt.Sprintf("dev=%d/ino=%d", r.Dev, r.Ino) }

// node — 平台层"一个已打开的目录"句柄。**唯一**的平台差异面就收在这一个类型里：
//
//	linux/darwin：fd 是真目录 fd，后续全部 fd 相对（unlinkat），永不拼路径；
//	other：r 是 os.Root、rel 是相对根的名字（os.Root 内部也是 fd 相对且拒绝出根，
//	  但它会跟随根**内部**的链接 ⇒ 那一路必须先 Lstat 判型再动手）。
type node struct {
	fd  int
	r   *os.Root
	rel string
}

// RemoveReport — 一次删树的读数（成败都填，便于审计与对账落账）。
type RemoveReport struct {
	// Root — 实际删掉的茧根身份（rm_other.go 恒零值：该平台拿不到稳定读数）。
	Root RootID
	// Verified — 是否与调用方期望比对过且相符（want 未固定时恒 false）。
	Verified bool
	// Files — 删掉的非目录条目数（普通文件/链接/设备/fifo）。
	Files int
	// Dirs — 删掉的子目录数（不含茧根自身）。
	Dirs int
	// Degraded — 非空 = 已降级，内容是"放弃了哪项防护"的原话（见抬头 ★降级）。
	Degraded string
}

// StatRoot — 用与删除**同一套**打开方式取茧根身份（供登记时记账）。
//
// 只读、不删任何东西；拿到的值原样作为 want 回传给 RemoveCocoon/RemoveTree。
func StatRoot(base, id string) (RootID, error) {
	if err := validEntryName(id); err != nil {
		return RootID{}, err
	}
	absBase, err := filepath.Abs(base)
	if err != nil {
		return RootID{}, fmt.Errorf("cocoon: 解析茧根父目录 %q 失败: %w", base, err)
	}
	baseNode, err := openBase(absBase)
	if err != nil {
		return RootID{}, err
	}
	defer closeNode(baseNode)

	rootNode, rid, err := openRootAt(baseNode, id)
	if err != nil {
		return RootID{}, err
	}
	if err := closeNode(rootNode); err != nil {
		return rid, fmt.Errorf("cocoon: 关闭茧根 fd 失败: %w", err)
	}
	return rid, nil
}

// RemoveCocoon — 删一个茧：<base>/<m.ID>，**先留墓碑再删**。
//
// 前置（见 meta.go 抬头 ③，本函数不代劳）：调用方应先把 m 推到 PhaseDeleting 并落盘，
// 再调本函数——否则墓碑里记的是旧 phase，等于在盘上留一句谎。
//
// 顺序（见抬头）：
//  1. m.Validate()（id/phase 合法）——名字非法时一个 syscall 都不发；
//  2. WriteTombstone(base, m) 落盘（失败 ⇒ 返回 ErrNeedTombstone，一个字节都不删）；
//  3. RemoveTree(base, m.ID, want)。
//
// want 由调用方从 StatRoot 取（零值 = 不绑身份，只按名字删——生产路径上不该这么用，
// 但 A5 的清扫面要能处理"没登记过身份的残骸"，所以留了这个口子，且 Report 里
// Verified=false 会让调用方看见这次没绑）。
func RemoveCocoon(base string, m *Meta, want RootID) (RemoveReport, error) {
	if m == nil {
		return RemoveReport{}, fmt.Errorf("%w: meta 为 nil", ErrInvalidMeta)
	}
	if err := m.Validate(); err != nil {
		return RemoveReport{}, err
	}
	if err := WriteTombstone(base, m); err != nil {
		return RemoveReport{}, fmt.Errorf("%w（先留墓碑失败，未删除任何东西）: %v", ErrNeedTombstone, err)
	}
	return RemoveTree(base, m.ID, want)
}

// RemoveTree — 按名字删 <base>/<id> 整棵树（**不含**墓碑那一步，墓碑由调用方负责）。
//
// 返回的 RemoveReport 在出错时也是可用的（已经删掉多少条都在里面），便于对账记账：
// 出错的语义是"停在这里、后面的一条没动"，不是"跳过坏的继续删"。
func RemoveTree(base, id string, want RootID) (rep RemoveReport, err error) {
	rep.Degraded = degradeNote()
	// 降级说明必须跟着错误一起出去（不许只在 Report 里躺着）。
	defer func() { err = noteDegraded(err, rep.Degraded) }()

	if err := validEntryName(id); err != nil {
		return rep, err
	}
	absBase, err := filepath.Abs(base)
	if err != nil {
		return rep, fmt.Errorf("cocoon: 解析茧根父目录 %q 失败: %w", base, err)
	}

	baseNode, err := openBase(absBase)
	if err != nil {
		return rep, err
	}
	defer closeNode(baseNode)

	rootNode, rid, err := openRootAt(baseNode, id)
	if err != nil {
		return rep, err
	}
	rep.Root = rid
	if want.Pinned() {
		// 盘上拿不到身份（rm_other.go）也算"没绑上"——比对要求两边都能比。
		if !rid.Pinned() || rid != want {
			_ = closeNode(rootNode)
			return rep, fmt.Errorf("%w: 期望 %s，盘上是 %s", ErrRootIdentity, want, rid)
		}
		rep.Verified = true
	}

	if err := removeChildren(rootNode, rid, &rep); err != nil {
		_ = closeNode(rootNode)
		return rep, err
	}
	// rmdir 之前先关本层 fd（见抬头：子 fd 一律先关再删名字）。
	if err := closeNode(rootNode); err != nil {
		return rep, fmt.Errorf("cocoon: 关闭茧根 fd 失败（未 rmdir）: %w", err)
	}
	if err := removeEntry(baseNode, id, true); err != nil {
		return rep, fmt.Errorf("cocoon: rmdir 茧根 %q 失败: %w", id, err)
	}
	return rep, nil
}

// removeChildren — 清空 dir 的全部子项（不含 dir 自身）。
//
// 每个子项只有两条路：**是目录**（平台层 openSub 成功）⇒ 递归 → 关 fd → unlinkat(AT_REMOVEDIR)；
// **不是目录**（errNotDir：链接/普通文件/设备）⇒ 直接 unlinkat 删名字，目标一个字节不碰。
// 其余任何错误（EXDEV/EACCES/ELOOP…）一律原样上抛：这是 fail-closed 的落点。
func removeChildren(dir node, root RootID, rep *RemoveReport) error {
	names, err := listNames(dir)
	if err != nil {
		return fmt.Errorf("cocoon: 读目录失败（fail-closed，一条都没动）: %w", err)
	}
	for _, name := range names {
		child, cid, err := openSub(dir, name)
		switch {
		case err == nil:
			// 目录：继续往下。
		case errors.Is(err, errNotDir):
			if e := removeEntry(dir, name, false); e != nil {
				return fmt.Errorf("cocoon: 删除条目 %q 失败: %w", name, e)
			}
			rep.Files++
			continue
		case errors.Is(err, fs.ErrNotExist):
			// 已经被别人删掉了：目标状态已达成，可重入。
			continue
		default:
			return fmt.Errorf("cocoon: 打开子目录 %q 失败（fail-closed，不跳过）: %w", name, err)
		}

		// ② 不跨挂载：macOS 无 NO_XDEV 等价物，靠这一比兜底；Linux 上 NO_XDEV 已经拦过一道。
		if root.Pinned() && cid.Pinned() && cid.Dev != root.Dev {
			_ = closeNode(child)
			return fmt.Errorf("%w: 子目录 %q（%s）与茧根（%s）不在同一挂载点", ErrCrossMount, name, cid, root)
		}
		if err := removeChildren(child, root, rep); err != nil {
			_ = closeNode(child)
			return err
		}
		// rmdir 之前先关子 fd。
		if err := closeNode(child); err != nil {
			return fmt.Errorf("cocoon: 关闭子目录 %q 的 fd 失败（未 rmdir）: %w", name, err)
		}
		if err := removeEntry(dir, name, true); err != nil {
			return fmt.Errorf("cocoon: rmdir 子目录 %q 失败: %w", name, err)
		}
		rep.Dirs++
	}
	return nil
}

// validEntryName — 单分量名字的自检（本文件"永不拼路径"的前提）。
//
// 反斜杠在 unix 上是合法文件名字符，这里照样拒：茧 id 一旦带 `\`，同一个 id 在 Windows
// 分支上就成了两条路径，跨平台的"同一个茧"就没法判了。
func validEntryName(name string) error {
	if name == "" || name == "." || name == ".." ||
		strings.ContainsAny(name, `/\`) || strings.IndexByte(name, 0) >= 0 {
		return fmt.Errorf("%w: %q", ErrUnsafeName, name)
	}
	return nil
}

// noteDegraded — 把降级声明缀进错误（%w 保留 errors.Is 链）。
func noteDegraded(err error, note string) error {
	if err == nil || note == "" {
		return err
	}
	return fmt.Errorf("%w（降级警告：%s）", err, note)
}
