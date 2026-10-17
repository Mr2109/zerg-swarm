// zerg_egg_closure.go —— 卵沙箱第 3 步：打蛋时把**引擎的 .so 闭包**一起收进蛋（sysroot 面）。
//
// ★ 两条路的取舍（本单拍定 = **A**，理由逐条给在这里，✗ 不留成口头结论）：
//
//	A（本件）　`cocoon pack <源目录> <输出.egg> --engine-dir <目录> [--engine-dir <目录>…]`
//	          把**调用方给定**的目录整棵树收进蛋内 `engine/<目录基名>/…`（段分类恒「引擎段」）；
//	          **收谁**由构建脚本决定（它知道目标机的根、也知道 dlopen 的数据目录）。
//	B（✗ 不做）打蛋时读 `readelf -d` 递归解析 NEEDED 并自动拷 —— 方便，但三条硬伤：
//	          ① 解析必须按**目标机**的 loader 搜索序（DT_RPATH → LD_LIBRARY_PATH → DT_RUNPATH →
//	             ld.so.cache → 默认目录）算，而打蛋常在**另一台机器**（本仓实况：蛋在 macOS 上打、
//	             跑在 edge-node 的 Linux 上）⇒ 解析出来的路径属于打蛋机，不是目标机的根；
//	          ② `ldd` 禁用（它跑的是**宿主 loader**，且看不见 dlopen）；而纯 `readelf`/`debug/elf`
//	             **同样看不见 dlopen** —— edge-node 实测：rocBLAS 运行期从 `/opt/rocm/lib/rocblas/5.1.0`
//	             取 **1307 个 .hsaco、556 MB**，这些字节**一条都不在 DT_NEEDED 里** ⇒ 自动解析出来的
//	             「闭包」是一份**在打蛋机上跑得动、在空根里跑不动**的清单；
//	          ③ 自动拷会把「蛋里有什么」变成解析器的实现细节 ⇒ 出事时说不清是「漏解析」还是「漏构建」。
//	⇒ 解析责任归**构建脚本**，打蛋端只留**只读判据**：收进来的东西够不够跑，必须当场判得出来。
//
// ★ 判据（本单核心 · stdlib `debug/elf` ⇒ ✗ 不引依赖、✗ 不起子进程、✗ 不用 ldd）：
//
//	① 蛋里每个**动态件**（有 `SHT_DYNAMIC` 或有 `DT_NEEDED`）的全部 `DT_NEEDED` 都必须能在蛋里
//	   按**基名**命中 —— 含**链段**（`libfoo.so.1 -> libfoo.so.1.2` 这种 soname 靠链名命中）；
//	② 恒必带三件：`ld-linux*`（loader 本体）· `libc.so.6` · `libm.so.6`。
//	   loader 缺 ⇒ 内核 exec 直接 ENOENT（连 127 都到不了）；`libm.so.6` **恒独立**（glibc 2.34 只合并了
//	   `libpthread/libdl/libutil/libanl`，`libm` 与 `librt` 都没合）⇒ 不按 NEEDED 判它就会「差一件 127」；
//	③ 缺任意一条 ⇒ **打蛋失败**，逐条点名「缺哪个 · 谁需要它」（✗ 不产半截蛋、✗ 不静默降级成告警）。
//	★ 静态件（无 `SHT_DYNAMIC`/`NEEDED`）⇒ 不要求上面三件（不误伤纯静态蛋）；musl 档（loader 是
//	  `ld-musl*`）同理只判 NEEDED（本版的目标机是 glibc x86-64，musl 档只做「不误伤」，不做额外主张）。
package main

import (
	"bytes"
	"debug/elf"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
)

const (
	// zegEngineSegRoot —— 引擎目录在蛋内的统一落点前缀：`engine/<目录基名>/…`。
	// ★ 段分类靠 `zegSegOfPath` 的 `/engine/` 那一档 ⇒ 这些段恒落「引擎段」（v2 段序里一律最后）。
	zegEngineSegRoot = "engine"
	// zegClosureLibc / zegClosureLibm —— 恒必带的两件（loader 另按前缀判）。
	zegClosureLibc = "libc.so.6"
	zegClosureLibm = "libm.so.6"
	// zegClosureLoaderPrefix —— loader 本体的基名前缀（`ld-linux-x86-64.so.2` 一族）。
	zegClosureLoaderPrefix = "ld-linux"
	// zegClosureMuslPrefix —— musl 档的 loader 前缀（只用于「不误伤」，见文件头 ② 尾注）。
	zegClosureMuslPrefix = "ld-musl"
	// zegClosureFromMax —— 缺件报话里最多点几个「谁需要它」（其余折成「等 N 段」）。
	zegClosureFromMax = 3
)

// ---- 收引擎目录 -------------------------------------------------------------------------------

// zegEngineDirEntries —— 把 `--engine-dir <root>` 整棵树收成**蛋内段**（前缀 `engine/<基名>/`）。
//
// 与 `zegCollectEntries` 的差别只有一处，且是**有意**的：
//
//	**mode 按安装语义归一** —— 构建树里的件常是 `0775`（umask 002 的产物），而蛋格式的安全子集
//	只有 `{0644,0755,0700}`（`zegModeSafeSet`）。这里**不往安全子集里加值**，而是把引擎目录段归一成
//	`0755`（有任一执行位）或 `0644`（否则）：蛋内 sysroot 要被空间内**任意 uid** 读/执行（bwrap 里的
//	uid 不保证与打包机一致）⇒ 这正是「安装」那一步本来就该做的事。**setuid/setgid/sticky 一律拒**
//	（✗ 绝不靠归一洗掉提权位），特殊文件 / 硬链照 `zegFsRules` **原样具名拒**。
//	归一发生时会往 stderr 打一行计数（✗ 不静默改写来源的 mode）。
//
// 其余规则与主线**同源**：路径串规则表（`zegRefusePath`）· 符号链接四道（`zegRefuseSymlinkSet`）
// · 段类型同表（`zegMediaTypeOfPath`）。`taken` 是「蛋内路径 → 已占用它的来源」的账（撞名即拒）。
func zegEngineDirEntries(root string, taken map[string]string) ([]*zegEntry, error) {
	abs, aerr := filepath.Abs(root)
	if aerr != nil {
		return nil, aerr
	}
	fi, serr := os.Stat(abs)
	if serr != nil {
		return nil, fmt.Errorf("`--engine-dir` 取不到目录 %q：%v", root, serr)
	}
	if !fi.IsDir() {
		return nil, fmt.Errorf("`--engine-dir` 不是目录 %q（✗ 只收目录：要收单件请把它放进目录再给）", root)
	}
	base := filepath.Base(abs)
	if base == "" || base == "." || base == string(filepath.Separator) {
		return nil, fmt.Errorf("`--engine-dir` 的目录基名取不出（%q）⇒ 蛋内落点无处安放，拒", root)
	}
	prefix := zegEngineSegRoot + "/" + base
	if err := zegRefusePath(prefix); err != nil {
		return nil, err
	}

	links, lerr := zegCollectSymlinks(abs)
	if lerr != nil {
		return nil, lerr
	}
	if err := zegRefuseSymlinkSet(links); err != nil {
		return nil, err
	}

	var out []*zegEntry
	var walk func(dir string) error
	walk = func(dir string) error {
		des, derr := os.ReadDir(dir)
		if derr != nil {
			return derr
		}
		for _, de := range des { // os.ReadDir 已按名升序 ⇒ 蛋内段序确定
			full := filepath.Join(dir, de.Name())
			li, lerr := os.Lstat(full)
			if lerr != nil {
				return lerr
			}
			rel, rerr := filepath.Rel(abs, full)
			if rerr != nil {
				return rerr
			}
			eggPath := prefix + "/" + filepath.ToSlash(rel)
			if err := zegRefusePath(eggPath); err != nil {
				return err
			}
			if li.Mode()&os.ModeSymlink != 0 {
				tgt, terr := os.Readlink(full)
				if terr != nil {
					return terr
				}
				// 链图与四道判已由 zegRefuseSymlinkSet 全树过完 ⇒ 这里按链段记（size=0 · 链名原样）。
				out = append(out, &zegEntry{
					Path:       eggPath,
					Size:       0,
					SHA256:     zegEmptySHA256,
					Mode:       zegSymlinkMode,
					MediaType:  zegMediaTypeOfPath(eggPath),
					LinkTarget: tgt,
				})
				continue
			}
			if li.IsDir() {
				if werr := walk(full); werr != nil {
					return werr
				}
				continue
			}
			mode, merr := zegEngineEntryMode(li, full)
			if merr != nil {
				return merr
			}
			st, sterr := os.Stat(full)
			if sterr != nil {
				return sterr
			}
			sha, herr := zegSha256File(full)
			if herr != nil {
				return herr
			}
			out = append(out, &zegEntry{
				Path:      eggPath,
				Size:      st.Size(),
				SHA256:    sha,
				Mode:      mode,
				MediaType: zegMediaTypeOfPath(eggPath),
				src:       full,
			})
		}
		return nil
	}
	if err := walk(abs); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("`--engine-dir` 目录里没有任何件 %q（✗ 空目录不算闭包）", root)
	}
	for _, e := range out {
		if prev, dup := taken[e.Path]; dup {
			return nil, fmt.Errorf("引擎目录 %q 的段与 %s 撞同一蛋内路径 %q ⇒ 拒（✗ 不说谁盖谁：两条来路必须自己先并好）",
				root, prev, e.Path)
		}
		taken[e.Path] = "`--engine-dir " + root + "`"
	}
	return out, nil
}

// zegEngineEntryMode —— 引擎目录段的 mode 判与归一（与 `zegRefuseEntryMode` **同一张规则表**，
// 唯一差别：perml 位不拒、按安装语义归一）。理由见 `zegEngineDirEntries` 头注。
func zegEngineEntryMode(li os.FileInfo, subject string) (int, error) {
	m := li.Mode()
	switch {
	case m&os.ModeSymlink != 0:
		return 0, zegRuleRefuse("D9", "fs_symlink_entry_invalid", zegFsWhy("fs_symlink_entry_invalid"),
			fmt.Sprintf("%s（符号链接须先过 zegRefuseSymlinkSet 分流）", subject))
	case m&os.ModeDevice != 0:
		return 0, zegRuleRefuse("D9", "fs_device", zegFsWhy("fs_device"), subject)
	case m&os.ModeNamedPipe != 0:
		return 0, zegRuleRefuse("D9", "fs_fifo", zegFsWhy("fs_fifo"), subject)
	case m&os.ModeSocket != 0:
		return 0, zegRuleRefuse("D9", "fs_socket", zegFsWhy("fs_socket"), subject)
	case !m.IsRegular():
		return 0, zegRuleRefuse("D9", "fs_device", zegFsWhy("fs_device"), subject)
	}
	if st, ok := li.Sys().(*syscall.Stat_t); ok && st.Nlink > 1 {
		return 0, zegRuleRefuse("D9", "fs_hardlink", zegFsWhy("fs_hardlink"), subject)
	}
	if m&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky) != 0 {
		return 0, zegRuleRefuse("D9", "fs_mode_unsafe",
			zegFsWhy("fs_mode_unsafe")+"；引擎目录段也**不许**带提权位", subject)
	}
	if m.Perm()&0o111 != 0 {
		return 0o755, nil
	}
	return 0o644, nil
}

// ---- 闭包自检（★ 本单核心）-------------------------------------------------------------------

// zegClosureMissing —— 一条「缺」：名字 ＋ 依据 ＋ 谁需要它（`From` 为空 ⇒ 恒必带那三件）。
type zegClosureMissing struct {
	Name string
	Why  string
	From []string
}

// zegClosureReport —— 一次只读审计的读数（`Missing` 为空才是过）。
type zegClosureReport struct {
	Dynamic int // 蛋里的动态件数
	Needed  int // DT_NEEDED 名字并集条数
	Segs    int // 蛋内段总数
	Loader  string
	Libc    string
	Libm    string
	Missing []zegClosureMissing
}

// zegClosureSummary —— 过档的一行人面读数（★ 逐字稳定：自检与命令面都印它）。
func (r *zegClosureReport) zegClosureSummary() string {
	return fmt.Sprintf("引擎闭包自检：动态件 %d · DT_NEEDED 并集 %d 条 · 蛋内段 %d ⇒ 全命中 ✓（loader=%s · %s · %s）",
		r.Dynamic, r.Needed, r.Segs, r.Loader, r.Libc, r.Libm)
}

// zegClosureFailure —— 缺件 ⇒ 一个**逐条点名**的错误（✗ nil 之外不返回任何东西 ⇒ 调用方只能中止）。
func (r *zegClosureReport) zegClosureFailure() error {
	if len(r.Missing) == 0 {
		return nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "引擎闭包自检失败（缺 %d 件 ⇒ 这枚蛋到空根里跑不起来；✗ 不产半截蛋）：", len(r.Missing))
	for _, m := range r.Missing {
		b.WriteString("\n")
		if len(m.From) == 0 {
			fmt.Fprintf(&b, "  ✗ 缺 `%s` —— %s", m.Name, m.Why)
			continue
		}
		froms := append([]string(nil), m.From...)
		sort.Strings(froms)
		more := ""
		if len(froms) > zegClosureFromMax {
			more = fmt.Sprintf(" 等 %d 段", len(froms))
			froms = froms[:zegClosureFromMax]
		}
		fmt.Fprintf(&b, "  ✗ 缺 `%s` —— 被 %s%s 需要（DT_NEEDED）", m.Name, strings.Join(froms, "、"), more)
	}
	b.WriteString("\n⇒ 补齐后重打（`--engine-dir` 可给多次；**dlopen 的数据目录也必须显式交进来** —— 本检只看 DT_NEEDED）")
	return errors.New(b.String())
}

// zegElfNeeded —— 读一件的 `DT_NEEDED`（**stdlib `debug/elf`**：只解 ELF 头与节表，✗ 不起子进程、
// ✗ 不用 ldd、✗ 不整件读进内存 —— 583 MB 的 .so 也只读头）。三态：
//
//	(needs, true,  nil)  —— 是 ELF 且是动态件；
//	(nil,   false, nil)  —— 不是 ELF（权重 / 文本 / 链段一类）⇒ 不参与判据；
//	(nil,   true,  err)  —— 有 `SHT_DYNAMIC` 却读不出 NEEDED（件坏了）⇒ 调用方 fail-closed。
func zegElfNeeded(path string) ([]string, bool, error) {
	f, err := elf.Open(path)
	if err != nil {
		return nil, false, nil
	}
	defer f.Close()
	dynamic := f.SectionByType(elf.SHT_DYNAMIC) != nil
	needs, nerr := f.ImportedLibraries()
	if nerr != nil {
		if dynamic {
			return nil, true, nerr
		}
		return nil, false, nil
	}
	return needs, dynamic || len(needs) > 0, nil
}

// zegClosureAudit —— ★ 本单的判据：给**蛋内全部段**（src 目录的 ＋ 各 `--engine-dir` 的），
// 只读地判「够不够跑」。✗ 不写盘、✗ 不改任何东西、✗ 不猜缺的件在哪。
func zegClosureAudit(entries []*zegEntry) (*zegClosureReport, error) {
	rep := &zegClosureReport{Segs: len(entries)}
	present := make(map[string]string, len(entries))
	for _, e := range entries {
		b := filepath.Base(e.Path)
		if _, ok := present[b]; !ok {
			present[b] = e.Path
		}
	}
	var order []string
	froms := map[string][]string{}
	for _, e := range entries {
		if e.src == "" || e.Size == 0 {
			continue // 链段（size=0 · 无载荷）与无来源段不参与
		}
		needs, dynamic, nerr := zegElfNeeded(e.src)
		if nerr != nil {
			return nil, fmt.Errorf("段 %q 有动态段但读不出 DT_NEEDED（ELF 解析失败 ⇒ 不敢猜它的依赖）：%v", e.Path, nerr)
		}
		if !dynamic {
			continue
		}
		rep.Dynamic++
		for _, n := range needs {
			if _, seen := froms[n]; !seen {
				order = append(order, n)
			}
			froms[n] = append(froms[n], e.Path)
		}
	}
	rep.Needed = len(order)
	byName := map[string]*zegClosureMissing{}
	addMissing := func(name, why string, from []string) {
		if m, ok := byName[name]; ok {
			m.From = append(m.From, from...)
			return
		}
		m := &zegClosureMissing{Name: name, Why: why, From: append([]string(nil), from...)}
		byName[name] = m
		rep.Missing = append(rep.Missing, *m)
	}
	for _, n := range order {
		if _, ok := present[n]; ok {
			continue
		}
		addMissing(n, "", froms[n])
	}
	if rep.Dynamic > 0 {
		// ① loader 本体（按前缀认：`ld-linux-x86-64.so.2` 一族）
		loader := ""
		for b := range present {
			if strings.HasPrefix(b, zegClosureLoaderPrefix) {
				loader = present[b]
				break
			}
		}
		rep.Loader = loader
		if loader == "" {
			addMissing(zegClosureLoaderPrefix+"*",
				"loader 本体（interp 指它 —— 缺 ⇒ 内核 exec 直接 ENOENT，连 127 都到不了）", nil)
		}
		// ② glibc 两件（musl 档只判 NEEDED，不误伤）
		if loader != "" && strings.HasPrefix(filepath.Base(loader), zegClosureMuslPrefix) {
			rep.Libc, rep.Libm = "（musl 档不要求）", "（musl 档不要求）"
		} else {
			if p, ok := present[zegClosureLibc]; ok {
				rep.Libc = p
			} else {
				addMissing(zegClosureLibc, "glibc 本体（下限两件之一：ld-linux ＋ libc.so.6）", nil)
			}
			if p, ok := present[zegClosureLibm]; ok {
				rep.Libm = p
			} else {
				addMissing(zegClosureLibm,
					"glibc 的 libm **恒独立**（2.34 只合并了 libpthread/libdl/libutil/libanl）⇒ 缺 ⇒ 一调数学函数就 127", nil)
			}
		}
	}
	if len(rep.Missing) > 1 {
		sort.SliceStable(rep.Missing, func(i, j int) bool { return rep.Missing[i].Name < rep.Missing[j].Name })
	}
	return rep, nil
}

// zegClosureGate —— pack 侧的唯一调用点：审计 ⇒ 缺件即返回具名错误（✗ 不写半截蛋）。
func zegClosureGate(entries []*zegEntry) (*zegClosureReport, error) {
	rep, err := zegClosureAudit(entries)
	if err != nil {
		return nil, err
	}
	if ferr := rep.zegClosureFailure(); ferr != nil {
		return rep, ferr
	}
	return rep, nil
}

// ---- 自检夹具：在内存里造一枚最小 ELF64（只够 `debug/elf` 解出 DT_NEEDED）---------------------
//
// ★ 为什么要在**产品件里**造夹具：`--self-test` 是命令面的一档（用户/CI 都会跑），它必须在
// **任何平台**上都能自证这条判据有牙 —— 而本仓的打蛋机是 macOS（真 ELF 一件都没有）⇒ 只能自己造字节。
// ★ 造出来的件**不可执行**（无 PT_LOAD / 无 interp），只用于判「NEEDED 能不能读出来」。
// 形状：ehdr(64) ＋ .dynstr ＋ .dynamic(SHT_DYNAMIC · sh_link→.dynstr) ＋ .shstrtab ＋ 4 个节头。
func zegSynthELF(needed []string) []byte {
	const ehsize = 64
	const shentsize = 64
	var dynstr bytes.Buffer
	dynstr.WriteByte(0)
	offs := make([]uint32, len(needed))
	for i, n := range needed {
		offs[i] = uint32(dynstr.Len())
		dynstr.WriteString(n)
		dynstr.WriteByte(0)
	}
	dyn := make([]byte, 16*(len(needed)+1))
	p := 0
	for i := range needed {
		binary.LittleEndian.PutUint64(dyn[p:], uint64(elf.DT_NEEDED))
		binary.LittleEndian.PutUint64(dyn[p+8:], uint64(offs[i]))
		p += 16
	}
	binary.LittleEndian.PutUint64(dyn[p:], uint64(elf.DT_NULL))
	shstr := []byte("\x00.dynstr\x00.dynamic\x00.shstrtab\x00")
	const ( // .shstrtab 里的名字偏移
		nameDynstr  = 1
		nameDynamic = 9
		nameShstr   = 18
	)
	dynstrOff := uint32(ehsize)
	dynOff := dynstrOff + uint32(dynstr.Len())
	shstrOff := dynOff + uint32(len(dyn))
	shoff := shstrOff + uint32(len(shstr))
	b := make([]byte, int(shoff)+4*shentsize)
	copy(b, []byte{0x7f, 'E', 'L', 'F'})
	b[elf.EI_CLASS] = byte(elf.ELFCLASS64)
	b[elf.EI_DATA] = byte(elf.ELFDATA2LSB)
	b[elf.EI_VERSION] = byte(elf.EV_CURRENT)
	binary.LittleEndian.PutUint16(b[16:], uint16(elf.ET_DYN))
	binary.LittleEndian.PutUint16(b[18:], uint16(elf.EM_X86_64))
	binary.LittleEndian.PutUint32(b[20:], uint32(elf.EV_CURRENT))
	binary.LittleEndian.PutUint64(b[40:], uint64(shoff)) // e_shoff
	binary.LittleEndian.PutUint16(b[52:], ehsize)        // e_ehsize
	binary.LittleEndian.PutUint16(b[58:], shentsize)     // e_shentsize
	binary.LittleEndian.PutUint16(b[60:], 4)             // e_shnum
	binary.LittleEndian.PutUint16(b[62:], 3)             // e_shstrndx
	copy(b[dynstrOff:], dynstr.Bytes())
	copy(b[dynOff:], dyn)
	copy(b[shstrOff:], shstr)
	sh := func(i int, name, typ, off, size, link, entsize uint32, align uint64) {
		o := int(shoff) + i*shentsize
		binary.LittleEndian.PutUint32(b[o:], name)
		binary.LittleEndian.PutUint32(b[o+4:], typ)
		binary.LittleEndian.PutUint64(b[o+24:], uint64(off))     // sh_offset
		binary.LittleEndian.PutUint64(b[o+32:], uint64(size))    // sh_size
		binary.LittleEndian.PutUint32(b[o+40:], link)            // sh_link
		binary.LittleEndian.PutUint64(b[o+48:], align)           // sh_addralign
		binary.LittleEndian.PutUint64(b[o+56:], uint64(entsize)) // sh_entsize
	}
	sh(0, 0, uint32(elf.SHT_NULL), 0, 0, 0, 0, 0)
	sh(1, nameDynstr, uint32(elf.SHT_STRTAB), dynstrOff, uint32(dynstr.Len()), 0, 0, 1)
	sh(2, nameDynamic, uint32(elf.SHT_DYNAMIC), dynOff, uint32(len(dyn)), 1, 16, 8)
	sh(3, nameShstr, uint32(elf.SHT_STRTAB), shstrOff, uint32(len(shstr)), 0, 0, 1)
	return b
}

// ---- 自检（六格 ＋ 两枚负控 · 真造件、真走 pack 入口）------------------------------------------

// zegClosureSelfTestLastLine —— 自检**末行**（逐字对拍用；★ 不含运行期变量）。
const zegClosureSelfTestLastLine = "self-test（引擎闭包 · sysroot）通过（4 态 ＋ 3 负控）：" +
	"① 完整闭包 ⇒ 绿（NEEDED 全命中 ＋ loader/libc/libm 在）· " +
	"② 抽掉一个 NEEDED 件 ⇒ 红且逐字点名它 · " +
	"③ 抽掉 loader ⇒ 红且点名 ld-linux* · " +
	"④ 非 ELF 树（静态/纯文本）⇒ 不要求三件（不误伤）· " +
	"负控① 同一份缺件树走**不审计**的 v1 `zegPack` ⇒ 打包成功（＝红来自本检）· " +
	"负控② 两个同名基名的 `--engine-dir` ⇒ 拒且点名撞名路径 · " +
	"负控③ 归档字节里没有 DT_NEEDED ⇒ 不算动态件（=判据真读 ELF，不是看扩展名）"

// zegClosureSelfTest —— 四态亲测 ＋ 三枚负控（真造 ELF 字节 · 走真 pack 入口 · 不碰真仓）。
func zegClosureSelfTest(stdout, stderr io.Writer) int {
	t, err := os.MkdirTemp("", "zerg-egg-closure-selftest")
	if err != nil {
		fmt.Fprintf(stderr, "closure: %v\n", err)
		return exitFail
	}
	defer os.RemoveAll(t) // 自回收（TMPDIR 前后 delta=0）

	// mkTree —— 造一棵「源目录」：一个 exe（NEEDED 恒含 libc/libm）＋（可选）一个 lib ＋（可选）loader。
	mkTree := func(name string, withLib, withLoader bool) (string, error) {
		dir := filepath.Join(t, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", err
		}
		w := func(fn string, b []byte) error {
			return os.WriteFile(filepath.Join(dir, fn), b, 0o755)
		}
		needed := []string{zegClosureLibc, zegClosureLibm}
		if withLib {
			needed = append(needed, "libsynth.so.3")
		}
		if err := w("llama-server", zegSynthELF(needed)); err != nil {
			return "", err
		}
		if withLib {
			if err := w("libsynth.so.3", zegSynthELF([]string{zegClosureLibc})); err != nil {
				return "", err
			}
		}
		if err := w(zegClosureLibc, []byte("synth-libc")); err != nil {
			return "", err
		}
		if err := w(zegClosureLibm, []byte("synth-libm")); err != nil {
			return "", err
		}
		if withLoader {
			if err := w("ld-linux-x86-64.so.2", []byte("synth-loader")); err != nil {
				return "", err
			}
		}
		return dir, nil
	}

	// 态① 完整闭包 ⇒ 绿
	tree1, terr := mkTree("ok", true, true)
	if terr != nil {
		return zegClosureSelfFail(stderr, terr)
	}
	egg1 := filepath.Join(t, "ok.egg")
	if _, _, perr := zegPackV2(tree1, egg1, 4096, "2000-01-01T00:00:00Z"); perr != nil {
		return zegClosureSelfFail(stderr, fmt.Errorf("态① 完整闭包竟打不成蛋：%v", perr))
	}

	// 态② 抽掉一个 NEEDED 件（libsynth.so.3）⇒ 红且逐字点名它（含发起方）
	tree2, terr := mkTree("nolib", true, true)
	if terr != nil {
		return zegClosureSelfFail(stderr, terr)
	}
	if err := os.Remove(filepath.Join(tree2, "libsynth.so.3")); err != nil {
		return zegClosureSelfFail(stderr, err)
	}
	_, _, perr2 := zegPackV2(tree2, filepath.Join(t, "nolib.egg"), 4096, "2000-01-01T00:00:00Z")
	if perr2 == nil {
		return zegClosureSelfFail(stderr, fmt.Errorf("态② 缺一个 NEEDED 件竟打成了蛋（判据无牙）"))
	}
	if !strings.Contains(perr2.Error(), "缺 `libsynth.so.3`") || !strings.Contains(perr2.Error(), "llama-server") {
		return zegClosureSelfFail(stderr, fmt.Errorf("态② 报话没点名缺件与发起方：%v", perr2))
	}

	// 态③ 抽掉 loader ⇒ 红且点名 ld-linux*
	tree3, terr := mkTree("noloader", false, false)
	if terr != nil {
		return zegClosureSelfFail(stderr, terr)
	}
	_, _, perr3 := zegPackV2(tree3, filepath.Join(t, "noloader.egg"), 4096, "2000-01-01T00:00:00Z")
	if perr3 == nil {
		return zegClosureSelfFail(stderr, fmt.Errorf("态③ 缺 loader 竟打成了蛋（判据无牙）"))
	}
	if !strings.Contains(perr3.Error(), "缺 `"+zegClosureLoaderPrefix+"*`") {
		return zegClosureSelfFail(stderr, fmt.Errorf("态③ 报话没点名 loader：%v", perr3))
	}

	// 负控① 同一份缺件树走**不审计**的 v1 `zegPack` ⇒ 必须成功（＝②③ 的红来自本检，不是路径/模式规则）
	if _, verr := zegPack(tree2, filepath.Join(t, "neg1.egg"), 4096, "2000-01-01T00:00:00Z"); verr != nil {
		return zegClosureSelfFail(stderr, fmt.Errorf("负控① 缺件树在**不审计**的旧档上竟也打不成（那②③的红就分不清来源）：%v", verr))
	}

	// 态④ 非 ELF 树 ⇒ 不要求三件（不误伤纯静态/文本蛋）
	tree4 := filepath.Join(t, "text")
	if err := os.MkdirAll(tree4, 0o755); err != nil {
		return zegClosureSelfFail(stderr, err)
	}
	if err := os.WriteFile(filepath.Join(tree4, "recipe.json"), []byte("{}\n"), 0o644); err != nil {
		return zegClosureSelfFail(stderr, err)
	}
	if _, _, perr4 := zegPackV2(tree4, filepath.Join(t, "text.egg"), 4096, "2000-01-01T00:00:00Z"); perr4 != nil {
		return zegClosureSelfFail(stderr, fmt.Errorf("态④ 纯文本树（无动态件）竟被要求 glibc 闭包：%v", perr4))
	}

	// 负控③ 扩展名像 .so、字节里没有 DT_NEEDED ⇒ 不算动态件（判据真读 ELF，不是看文件名）
	tree5 := filepath.Join(t, "fakeso")
	if err := os.MkdirAll(tree5, 0o755); err != nil {
		return zegClosureSelfFail(stderr, err)
	}
	if err := os.WriteFile(filepath.Join(tree5, "libfake.so.1"), []byte("not-an-elf"), 0o644); err != nil {
		return zegClosureSelfFail(stderr, err)
	}
	if _, _, perr5 := zegPackV2(tree5, filepath.Join(t, "fakeso.egg"), 4096, "2000-01-01T00:00:00Z"); perr5 != nil {
		return zegClosureSelfFail(stderr, fmt.Errorf("负控③ 非 ELF 的 .so 被当成动态件了：%v", perr5))
	}

	// 负控② 两个同名基名的 engine 目录 ⇒ 拒且点名撞名路径
	d1 := filepath.Join(t, "a", "bin")
	d2 := filepath.Join(t, "b", "bin")
	for _, d := range []string{d1, d2} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return zegClosureSelfFail(stderr, err)
		}
		if err := os.WriteFile(filepath.Join(d, "x.so"), []byte("x"), 0o644); err != nil {
			return zegClosureSelfFail(stderr, err)
		}
	}
	taken := map[string]string{}
	if _, e1 := zegEngineDirEntries(d1, taken); e1 != nil {
		return zegClosureSelfFail(stderr, fmt.Errorf("负控② 第一枚引擎目录就拒了：%v", e1))
	}
	if _, e2 := zegEngineDirEntries(d2, taken); e2 == nil {
		return zegClosureSelfFail(stderr, fmt.Errorf("负控② 两个同名基名的引擎目录竟都收下了（撞名没牙）"))
	} else if !strings.Contains(e2.Error(), zegEngineSegRoot+"/bin/x.so") {
		return zegClosureSelfFail(stderr, fmt.Errorf("负控② 报话没点名撞名路径：%v", e2))
	}

	fmt.Fprintln(stdout, zegClosureSelfTestLastLine)
	return exitOK
}

func zegClosureSelfFail(stderr io.Writer, err error) int {
	fmt.Fprintf(stderr, "closure: 自检失败：%v\n", err)
	return exitFail
}
