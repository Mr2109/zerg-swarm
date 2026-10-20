// reconcile.go — 茧对账：盘上实况 vs meta.json 真源
//
// ★ 盘上布局（本文件定义的契约；A5 的清扫按此读，别私改）：
//
//	root/<id>/meta.json    茧目录 + 真源（meta.go 的 MetaPath 口径）
//	root/<id>.meta.json    平铺 meta（登记副本/墓碑：目录已不在、meta 还在的**唯一**可判面）
//	root/.meta-*.tmp       写一半的残渣（meta.go writeTmp 的产物，非目录、不参与四类）
//
// 为什么需要平铺面：meta 与目录同址 ⇒ "目录没了"会把 meta 一起带走，盘上不留任何证据，
// "有 meta 无目录"这一类就成了不可判的死角。平铺 meta（`<id>.meta.json`）是唯一能让
// 幽灵可判的落点：A5 先搬平再删目录，或崩在删除中途，就会留下它。本文件只**读**平铺面
// （Load / ReadTombstone），不自己造它——写端是 meta.go 的 WriteTombstone（A5 删目录前调）。
//
// ★ 四类结论（Report 的四个槽，逐条带 id + 原因）：
//
//	① 一致        目录在、meta 在、phase 与实况相符
//	② 孤儿目录    有目录无 meta        ⇒ 判为创建残渣/tmp 残渣，**不删真目录**，只记 pending-human
//	③ 幽灵 meta   有 meta 无目录        ⇒ 标 gone（墓碑留盘，待人清理）
//	④ phase 不符  phase=running 但无 pid / pid 已不在 / 无端口 ⇒ 降级
//
// （另有一槽 ⑤ meta 损坏：meta.json 在但读不出/结构非法。四类装不下它，又不能当孤儿——
// 它占着真源路径，硬塞进②会诱导人去删一个可能还有救的目录。单列、不动盘、待人。）
//
// ★ 处置顺序（固定，不可颠倒；颠倒会把"幽灵"和"降级"叠在同一份 meta 上，结论互相覆盖）：
//
//	第一步 ② 孤儿目录：只出结论，零写盘（无 meta 就没处 UpdatePhase，也不许凭空 Save）。
//	第二步 ③ 幽灵 meta：UpdatePhase(path, gone)。
//	第三步 ④ phase 不符：UpdatePhase(path, failed)。
//
// ★ 全部处置只走 UpdatePhase（读-校验-写同锁、单调前进），**不直接写盘**。
// 理由：A2 是**修账**不是**改账**——它没有资格把某个茧从 running 直接改成 gone，
// 只能沿着 phase 链把它往前推一格；倒退一律被 meta.go 挡住，这正是我们要的护栏。
//
// ★ 幂等：第二次跑必须零变更。三处保证：
//
//	② 从不写盘；
//	③ 已 gone 的墓碑**不再 UpdatePhase**（否则每次跑都重写一遍文件，"零变更"是假的）；
//	④ 降级落 failed ⇒ 第二遍看到的是 failed（不再是 running），自然收敛。
//
// ★ 撞约束（不硬编，照实上报）：
//
//	Ⅰ. phase=gone 但目录仍在：槽④能判出来，但 gone 是链尾，前进无路 ⇒ 只记 pending-human，
//	   不动盘（删目录是 A5 的事，A2 没有删除权）。
//	Ⅱ. "无端口" 只判 Ports 是否为空，不探端口是否真在听（dial 探测会把"起来了但还没绑好"误判为死，
//	   且在测试里不可控）。进程是否在 = kill(pid, 0) 判活（ESRCH 死、EPERM 活）。
//	Ⅲ. 降级落 failed 之后，"要不要重试"不归 A2 判：重试必须显式走 ResetForRetry
//	   （failed→starting + epoch+1），A2 只负责把不健康的茧判到 failed 就停手——
//	   对账每跑一次就自动重试一遍，等于把"一直在崩的茧"变成启动风暴。
package cocoon

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/sys/unix"
)

// flatMetaSuffix — 平铺 meta 的后缀：`<root>/<id>.meta.json`（与 MetaFileName 同尾，故先判 tmpPrefix）。
const flatMetaSuffix = ".meta.json"

// Kind — 一条对账结论的类别。
type Kind string

const (
	// KindConsistent — 目录/meta/phase 三者自洽。
	KindConsistent Kind = "consistent"
	// KindOrphanDir — 有目录无 meta（只记，不删）。
	KindOrphanDir Kind = "orphan-dir"
	// KindGhostMeta — 有 meta 无目录（标 gone）。
	KindGhostMeta Kind = "ghost-meta"
	// KindPhaseDrift — phase 与实况不符（降级）。
	KindPhaseDrift Kind = "phase-drift"
	// KindBrokenMeta — meta 在但读不出/结构非法（不动盘，待人）。
	KindBrokenMeta Kind = "broken-meta"
)

// Label — 给人看的类别名。
func (k Kind) Label() string {
	switch k {
	case KindConsistent:
		return "一致"
	case KindOrphanDir:
		return "孤儿目录"
	case KindGhostMeta:
		return "幽灵meta"
	case KindPhaseDrift:
		return "phase不符"
	case KindBrokenMeta:
		return "meta损坏"
	default:
		return string(k)
	}
}

// Finding — 单条结论：哪个茧、什么问题、怎么处置的、要不要人接手。
type Finding struct {
	Kind Kind
	// ID — 茧 ID（孤儿目录取目录名；幽灵/损坏取 meta 里的 id，取不到才退回文件名）。
	ID string
	// Path — 出问题的那个路径（目录 or meta 文件）。
	Path string
	// Phase — 处置前的 phase（零值 = 没读到 meta）。
	Phase Phase
	// Next — 处置要落的 phase（空 = 本次不动盘）。
	Next Phase
	// Changed — 本次真的写了盘。
	Changed bool
	// PendingHuman — 需要人接手（不自动处置，或处置后仍有尾巴）。
	PendingHuman bool
	// Reason — 一句话原因（直接给人读）。
	Reason string
	// Err — 处置失败的原因（非 nil 时 Changed 必为 false、PendingHuman 必为 true）。
	Err error
}

// Report — 一次对账的完整结论。四类分槽 + 计数 + 可直接打印。
type Report struct {
	// Root — 被扫的茧根（绝对路径）。
	Root string

	Consistent []Finding
	OrphanDirs []Finding
	GhostMetas []Finding
	PhaseDrift []Finding
	Broken     []Finding
	// Residue — root 下的 `.meta-*.tmp`（不在四类内，未动）。
	Residue []string
}

// Changed — 本次一共写了几份 meta。
func (r *Report) Changed() int {
	n := 0
	for _, group := range [][]Finding{r.Consistent, r.OrphanDirs, r.GhostMetas, r.PhaseDrift, r.Broken} {
		for _, f := range group {
			if f.Changed {
				n++
			}
		}
	}
	return n
}

// PendingHuman — 需要人接手的条数。
func (r *Report) PendingHuman() int {
	n := 0
	for _, group := range [][]Finding{r.Consistent, r.OrphanDirs, r.GhostMetas, r.PhaseDrift, r.Broken} {
		for _, f := range group {
			if f.PendingHuman {
				n++
			}
		}
	}
	return n
}

// String — 一屏能看完的对账报告：总数一行 + 逐条 id/原因。
func (r *Report) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "茧对账 root=%s\n", r.Root)
	fmt.Fprintf(&b, "  一致 %d / 孤儿目录 %d / 幽灵 meta %d / phase 不符 %d / meta 损坏 %d（本次处置 %d 项，待人 %d 项）\n",
		len(r.Consistent), len(r.OrphanDirs), len(r.GhostMetas), len(r.PhaseDrift), len(r.Broken),
		r.Changed(), r.PendingHuman())
	if len(r.Residue) > 0 {
		fmt.Fprintf(&b, "  tmp 残渣 %d 个（不在四类内，未动）：%s\n",
			len(r.Residue), strings.Join(r.Residue, ", "))
	}
	for _, group := range [][]Finding{r.OrphanDirs, r.GhostMetas, r.PhaseDrift, r.Broken, r.Consistent} {
		for _, f := range group {
			fmt.Fprintf(&b, "  [%s] %s  %s", f.Kind.Label(), f.ID, f.Reason)
			if f.Err != nil {
				fmt.Fprintf(&b, "；失败：%v", f.Err)
			}
			b.WriteString("\n")
		}
	}
	return b.String()
}

// FlatMetaPath — 平铺 meta 的路径：`<root>/<id>.meta.json`（幽灵的可判面，见抬头）。
func FlatMetaPath(root, id string) string { return filepath.Join(root, id+flatMetaSuffix) }

// Reconcile — 扫 root 下每个茧目录，与 meta.json 对账并按固定顺序处置。
//
// 返回的 Report 无论成败都可用（逐条结论都在里面）；error 只在两类情况下非 nil：
//   - root 读不了（不存在 ⇒ ErrNotFound；权限等 ⇒ 原样）；
//   - 有条目的处置写盘失败（Err 已记在对应 Finding 上）——此时**其余条目照样处置完**，
//     错误用 errors.Join 汇总返回，便于调用方一次看全。
func Reconcile(root string) (Report, error) {
	rep := Report{Root: root}

	abs, err := filepath.Abs(root)
	if err != nil {
		return rep, fmt.Errorf("cocoon: 解析茧根 %q 失败: %w", root, err)
	}
	rep.Root = abs

	entries, err := os.ReadDir(abs)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return rep, fmt.Errorf("%w: 茧根 %s 不存在", ErrNotFound, abs)
		}
		return rep, fmt.Errorf("cocoon: 读茧根 %s 失败: %w", abs, err)
	}

	type flatMeta struct{ id, path string }
	var (
		dirs  []string
		flats []flatMeta
	)
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, tmpPrefix) && strings.HasSuffix(name, tmpSuffix) {
			rep.Residue = append(rep.Residue, filepath.Join(abs, name))
			continue
		}
		if e.IsDir() {
			dirs = append(dirs, filepath.Join(abs, name))
			continue
		}
		if strings.HasSuffix(name, flatMetaSuffix) {
			flats = append(flats, flatMeta{id: strings.TrimSuffix(name, flatMetaSuffix), path: filepath.Join(abs, name)})
		}
	}
	sort.Strings(dirs)
	sort.Slice(flats, func(i, j int) bool { return flats[i].id < flats[j].id })

	var errs []error

	// ── 第一步：② 孤儿目录（有目录无 meta）——零写盘 ──────────────────
	// 同时把读到的 meta 收起来，留给第三步判 drift（顺序不能提前：先判孤儿，再判幽灵，最后降级）。
	loaded := make(map[string]*Meta, len(dirs))
	for _, dir := range dirs {
		id := filepath.Base(dir)
		m, err := Load(MetaPath(dir))
		switch {
		case err == nil:
			loaded[dir] = m
		case errors.Is(err, ErrNotFound):
			rep.OrphanDirs = append(rep.OrphanDirs, Finding{
				Kind:         KindOrphanDir,
				ID:           id,
				Path:         dir,
				PendingHuman: true,
				Reason:       "有目录无 meta.json：判为创建残渣/tmp 残渣，未删真目录，待人确认后清",
			})
		default:
			rep.Broken = append(rep.Broken, Finding{
				Kind:         KindBrokenMeta,
				ID:           id,
				Path:         MetaPath(dir),
				PendingHuman: true,
				Err:          err,
				Reason:       "meta.json 读不出或结构非法：未动盘（删目录会连带毁掉唯一证据）",
			})
		}
	}

	// ── 第二步：③ 幽灵 meta（有 meta 无目录）——标 gone ────────────────
	for _, f := range flats {
		m, err := Load(f.path)
		if err != nil {
			rep.Broken = append(rep.Broken, Finding{
				Kind:         KindBrokenMeta,
				ID:           f.id,
				Path:         f.path,
				PendingHuman: true,
				Err:          err,
				Reason:       "平铺 meta 读不出或结构非法：未动盘，待人确认",
			})
			continue
		}
		// 平铺 meta 但目录还在 = 重复登记，不算幽灵（"有 meta 无目录"不成立），不擅自删。
		if _, stErr := os.Stat(filepath.Join(abs, m.ID)); stErr == nil {
			rep.GhostMetas = append(rep.GhostMetas, Finding{
				Kind:         KindGhostMeta,
				ID:           m.ID,
				Path:         f.path,
				Phase:        m.Phase,
				PendingHuman: true,
				Reason:       "平铺 meta 的目录其实还在（重复登记）：未处置，待人确认哪份是真源",
			})
			continue
		}

		fd := Finding{Kind: KindGhostMeta, ID: m.ID, Path: f.path, Phase: m.Phase}
		if m.ID != f.id {
			fd.Reason = fmt.Sprintf("文件名 id=%s 与 meta.id=%s 不符；", f.id, m.ID)
		}
		switch {
		case m.Phase == PhaseGone:
			// 已经是墓碑：再 UpdatePhase(gone) 只是把同一份 JSON 重写一遍 ⇒ 破坏"零变更"。
			fd.PendingHuman = true
			fd.Reason += "有 meta 无目录，已标 gone；墓碑留盘待人清理（本次零变更）"
		default:
			next, uerr := UpdatePhase(f.path, PhaseGone)
			if uerr != nil {
				fd.Err = uerr
				fd.PendingHuman = true
				fd.Reason += fmt.Sprintf("有 meta 无目录，标 gone 失败：%v", uerr)
				errs = append(errs, fmt.Errorf("cocoon: 幽灵 %s → gone 失败: %w", m.ID, uerr))
			} else {
				fd.Next = PhaseGone
				fd.Changed = true
				fd.Reason += fmt.Sprintf("有 meta 无目录：标 gone（%s → %s）", m.Phase, next.Phase)
			}
		}
		rep.GhostMetas = append(rep.GhostMetas, fd)
	}

	// ── 第三步：④ phase 与实况不符——降级 ──────────────────────────────
	for _, dir := range dirs {
		m, ok := loaded[dir]
		if !ok {
			continue
		}
		reason := driftReason(m)
		if reason == "" {
			rep.Consistent = append(rep.Consistent, Finding{
				Kind:   KindConsistent,
				ID:     m.ID,
				Path:   dir,
				Phase:  m.Phase,
				Reason: consistentReason(m),
			})
			continue
		}
		fd := Finding{Kind: KindPhaseDrift, ID: m.ID, Path: MetaPath(dir), Phase: m.Phase, Reason: reason}
		if m.Phase == PhaseGone {
			// 链尾无路可走：只报不处置（删目录归 A5）。
			fd.PendingHuman = true
			fd.Reason += "；gone 是链尾，无处可前进，待人确认后删目录"
			rep.PhaseDrift = append(rep.PhaseDrift, fd)
			continue
		}
		next, uerr := UpdatePhase(fd.Path, PhaseFailed)
		if uerr != nil {
			fd.Err = uerr
			fd.PendingHuman = true
			fd.Reason += fmt.Sprintf("；降级失败：%v", uerr)
			errs = append(errs, fmt.Errorf("cocoon: %s 降级 failed 失败: %w", m.ID, uerr))
		} else {
			fd.Next = PhaseFailed
			fd.Changed = true
			fd.Reason += fmt.Sprintf("；判为 failed（%s → %s），重试须显式走 ResetForRetry", m.Phase, next.Phase)
		}
		rep.PhaseDrift = append(rep.PhaseDrift, fd)
	}

	for _, group := range [][]Finding{rep.Consistent, rep.OrphanDirs, rep.GhostMetas, rep.PhaseDrift, rep.Broken} {
		sort.SliceStable(group, func(i, j int) bool { return group[i].ID < group[j].ID })
	}

	return rep, errors.Join(errs...)
}

// driftReason — phase 与实况不符的一句话原因；空串 = 相符。
// 判据（只判 presence，不探端口是否真在听，见抬头撞约束 Ⅱ）：
//
//	running 且 pid>0 且进程在 且 Ports 非空 ⇒ 相符
//
// 只审 running：failed 是降级的**落点**，再判它就成了自我循环（第二遍永远不收敛）；
// 要不要从 failed 重试归调用方，见撞约束 Ⅲ。
func driftReason(m *Meta) string {
	if m == nil {
		return "meta 为 nil"
	}
	if m.Phase == PhaseGone {
		return "phase=gone 但目录仍在（墓碑没清干净）"
	}
	if m.Phase != PhaseRunning {
		return ""
	}
	if m.PID <= 0 {
		return "phase=running 但 pid=0（没有进程）"
	}
	if !pidAlive(m.PID) {
		return fmt.Sprintf("phase=running 但 pid=%d 已不在", m.PID)
	}
	if len(m.Ports) == 0 {
		return "phase=running 但没有端口"
	}
	return ""
}

// consistentReason — 一致条目的证据（给人看，不是给程序判）。
func consistentReason(m *Meta) string {
	switch {
	case m.Phase == PhaseRunning:
		return fmt.Sprintf("phase=running，pid=%d 在，端口 %d 个", m.PID, len(m.Ports))
	case m.Phase == PhaseCreated || m.Phase == PhaseStarting:
		return fmt.Sprintf("phase=%s，尚未起（无进程属正常）", m.Phase)
	case m.Phase == PhaseStopping || m.Phase == PhaseDeleting:
		return fmt.Sprintf("phase=%s，等 A5 回收（重入点）", m.Phase)
	case m.Phase == PhaseFailed:
		return fmt.Sprintf("phase=failed（epoch=%d），等重试/回收", m.Epoch)
	case m.Phase == PhaseGone:
		return "phase=gone（墓碑）"
	default:
		return fmt.Sprintf("phase=%s", m.Phase)
	}
}

// pidAlive — pid 对应的进程是否还活着（kill 0 探活：ESRCH=死，EPERM=活但没权限，其余当活）。
func pidAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := unix.Kill(pid, 0)
	if err == nil {
		return true
	}
	return errors.Is(err, unix.EPERM)
}
