// pack.go — 茧打包（A6）：白名单收件 ＋ tar 包 ＋ 侧车 manifest（逐件 sha256）＋ VerifyPack。
//
// ★ 这一层要防的三件事（都不是"文件读不出来"，而是"包里有不该有的东西"）：
//
//	① **凭据外带**：茧根里只要躺着一个 `.git/config`，包就会把 remote URL（常带 token）
//	  带出去。所以它不是"跳过"，而是 **硬拒**——静默丢件会让调用方以为"打全了"，
//	  而实际包内容与源树不一致；宁可失败并点名。
//	② **越界读**：符号链接一旦被跟随，`workspace/link → $HOME/.ssh/id_ed25519` 就会把
//	  茧外的东西装进包里。本文件对符号链接一律 **遇到即拒并点名路径**，不做"跳过"：
//	  跳过等于替调用方决定"这一个不要了"，而它可能正是他以为装进去了的那个。
//	  硬链走的是**同一条路**（A6b 补上）：`ln ~/.ssh/id_rsa workspace/x` 造出来的是个
//	  **普通文件**，没有链接位可判、内容照样被复制进包 ⇒ 对 `nlink > 1` 的普通文件同样
//	  一律拒（见 ErrPackHardlink）。nlink 的读法分平台（Stat_t 字段跨平台不同）：unix 面在
//	  pack_unix.go，Windows 在 pack_other.go 里**显式不判**（退让，不是已解决）。
//	③ **提权件**：SUID/SGID 位在解包侧会变成"以别人身份执行"的入口；设备节点/FIFO/
//	  套接字同样不是"内容"，是**行为**。四类一律拒。
//
// ★ 白名单语义（fail-closed，默认最小）：
//
//	白名单 = (Allow 非空 ? Allow : DefaultAllow) ∪ Artifacts。
//	`Allow` **非空即整体替换默认值**（要保留工作区就把 "workspace" 自己写进去）——
//	不做"叠加"是因为叠加会让调用方无法**收窄**，只能放宽，那不是白名单。
//	匹配规则（相对茧根、slash 分隔）：
//	  "a/b"      ⇒ 该文件，或 `a/b/` 整棵子树
//	  "a/**"     ⇒ 同上的显式写法
//	  "a/*.bin"  ⇒ path.Match（**不跨 /**，glob 只作用在同一层）
//	"."、""、".."、绝对路径一律拒（ErrPackUnsafePath）：**不许"整根打包"**，
//	必须点名——"打包整个茧根"永远会收进一些没人审过的东西。
//
// ★ 收件 vs 拒件（两者不是一回事，别混）：
//
//	收件（入包）：命中白名单 且 是普通文件 且 `nlink == 1` 且 不在 `.git/` 下。
//	拒件（返回错误、整包失败）：任何符号链接、任何 SUID/SGID、任何非普通文件
//	  （设备/FIFO/套接字/不规则）、任何 `nlink > 1` 的普通文件（硬链）、`.git/config`（任意深度）。
//	不收（结构性排除、不报错）：`cocoon.toml`（声明文件本身：它记着产物清单与可能的本机
//	  路径，入包等于把"配方"当"成品"发出去）、`.git/**`（含 `hooks/`）、未命中白名单的件。
//
//	⇒ 拒件的判定在**全树**上做，与是否命中白名单无关：树里有个符号链接，哪怕它在
//	  未收件的目录里，也说明这棵树不是我们能负责打包的东西。
//
// ★ 格式 v1（刻意简单，可验优先）：
//
//	包      = 未压缩 tar（条目按 path 升序；只放 TypeReg；uid/gid 归零、uname/gname 清空——
//	          不把宿主用户名带进制品）；条目带 `mode` 的权限位与 mtime。
//	manifest= 同目录侧车 `<包>.manifest`，**JSONL**：第 0 行是头（format/version/created_at/
//	          entries/total_bytes/pack_size），第 1..N 行逐件记 `path/size/mode/sha256`。
//	          第 0 行先写、且自带 `format` 版本 ⇒ 读端不靠猜、也不靠"扫目录找最新"。
//
//	为什么不上压缩/签名：v1 的目标是**可验**（读写同源、逐件 sha），不是省字节。
//	签名缺口见文件末"已知缺口"。
//
// ★ 读写同源（吃过亏的地方，写下来）：
//
//	manifest 的写端（Pack）与读端（VerifyPack）共用 `PackEntry` 与 `PackHeader` 两个结构体，
//	字段名就是 JSON 键名，不存在"写一个名、读另一个名"的缝。Pack 写完默认**自验一遍**
//	（NoSelfVerify 可关）：自验失败 ⇒ 删包删 manifest 并返回错误——绝不留下
//	"文件在、验不过"的半成品。
//
// ★ 已知缺口（v1 显式声明，别当已解决）：
//
//	① 无签名：manifest 与包本身都可被**整体替换**成另一套自洽的内容；本层只保证
//	  "包与 manifest 一致"，不保证"这份东西来自谁"。
//	② Windows 面 nlink 不判（pack_other.go 恒 1）：NTFS 有硬链，`ln` 等价物仍可把树外
//	  的东西挂进白名单。unix 面已封（pack_unix.go）。
//	③ tar 的**填充字节**（条目之间的对齐 0 与尾部块）不在逐件 sha 的覆盖范围里；
//	  改动它们只会让 tar 解析失败或长度不符，不会假绿，但也不是"逐字节可验"。
package cocoon

import (
	"archive/tar"
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	// PackFormat — manifest 头行的格式标识。读端必须**全等**才认。
	PackFormat = "zerg-cocoon-pack"
	// PackFormatVersion — 当前写端版本；读端接受 1..PackFormatVersion。
	PackFormatVersion = 1
	// PackManifestSuffix — 侧车 manifest 的后缀：`<包>.manifest`。
	PackManifestSuffix = ".manifest"
	// DefaultPackName — Pack 在 outPath 是目录时的默认包名（便于调用方只给目录）。
	DefaultPackName = "cocoon.tar"
)

// DefaultAllow — 默认白名单：**只有工作区**。声明的产物路径由 PackOptions.Artifacts 追加。
var DefaultAllow = []string{"workspace"}

// 打包面的错误分类。调用方靠 errors.Is 判，别比字符串。
var (
	// ErrPackForbidden — 树里出现禁止入包的路径（目前是任意深度的 `.git/config`）。
	ErrPackForbidden = errors.New("cocoon: 树内有禁止入包的路径（.git/config），拒绝打包")
	// ErrPackSymlink — 树里有符号链接：不跟随、不跳过，整包拒。
	ErrPackSymlink = errors.New("cocoon: 树内有符号链接，拒绝打包（不跟随）")
	// ErrPackSetuid — 树里有 SUID/SGID 位的件：拒。
	ErrPackSetuid = errors.New("cocoon: 树内有 SUID/SGID 件，拒绝打包")
	// ErrPackHardlink — 树里有 `nlink > 1` 的普通文件（硬链）：拒。
	// 硬链是"树外还有一个名字"的凭据：`ln ~/.ssh/id_rsa workspace/x` 造出来的件没有链接位，
	// 只有 nlink 能把它和"普通文件"分开。判不出 nlink 也归这一类（fail-closed）。
	ErrPackHardlink = errors.New("cocoon: 树内有硬链（nlink > 1）件，拒绝打包")
	// ErrPackSpecial — 树里有非普通文件（设备/FIFO/套接字/不规则）：拒。
	ErrPackSpecial = errors.New("cocoon: 树内有非普通文件（设备/FIFO/套接字），拒绝打包")
	// ErrPackEmpty — 白名单一件都没命中：拒（空包没有意义，且多半是白名单写错了）。
	ErrPackEmpty = errors.New("cocoon: 白名单未命中任何文件，拒绝打空包")
	// ErrPackUnsafePath — 白名单写法或件路径不安全（绝对路径 / `..` / 整根）。
	ErrPackUnsafePath = errors.New("cocoon: 路径不安全（绝对路径 / .. / 整根），拒绝")
	// ErrPackRoot — 源根不存在、不是目录、或是符号链接。
	ErrPackRoot = errors.New("cocoon: 源根不是目录（或为符号链接），拒绝打包")
	// ErrPackOutputInside — 输出包落在源树内：会把自己打进去（且自验时读到的已不是同一棵树）。
	ErrPackOutputInside = errors.New("cocoon: 输出包路径在源树内，拒绝打包")
	// ErrPackChanged — 收集与读取之间件被换掉/改了大小（TOCTOU）：拒。
	ErrPackChanged = errors.New("cocoon: 打包过程中件被改动，拒绝打包")
	// ErrManifestMissing — 侧车 manifest 不在（包与 manifest 必须成对）。
	ErrManifestMissing = errors.New("cocoon: 缺少侧车 manifest")
	// ErrManifestFormat — manifest 头不合规（格式标识/版本/JSON 坏）。
	ErrManifestFormat = errors.New("cocoon: manifest 头不合规")
	// ErrPackMismatch — 包与 manifest 不一致（逐件 sha / 大小 / 权限 / 件数 / 多件缺件）。
	ErrPackMismatch = errors.New("cocoon: 包与 manifest 不一致")
)

// PackOptions — 打包选项。零值即为默认：只收 `workspace/`，写完自验。
type PackOptions struct {
	// Allow — 白名单（相对茧根）。非空即**整体替换** DefaultAllow；Artifacts 始终追加。
	Allow []string
	// Artifacts — 声明的产物路径（来自茧的声明文件；按白名单同一套规则解释）。
	Artifacts []string
	// Now — 注入时钟（manifest 的 created_at 取值）；nil 用 cocoon.Now()。
	Now func() time.Time
	// NoSelfVerify — 跳过"写完即验"。默认验（大体量茧可关；关了就得调用方自己 VerifyPack）。
	NoSelfVerify bool
}

// PackEntry — manifest 里的一件。写端读端共用（字段名即 JSON 键名，见抬头"读写同源"）。
type PackEntry struct {
	Path   string `json:"path"`   // 相对茧根、slash 分隔、不含 `..`
	Size   int64  `json:"size"`   // 字节数（与 tar 头里的 size 必须相等）
	Mode   string `json:"mode"`   // 权限位四位八进制（如 "0644"）；不含 SUID/SGID（它们进不了包）
	SHA256 string `json:"sha256"` // 内容 sha256（小写 hex）
}

// PackHeader — manifest 第 0 行（头）。**永远在第一行**，读端第一行就认格式版本。
type PackHeader struct {
	Format     string    `json:"format"`
	Version    int       `json:"version"`
	CreatedAt  time.Time `json:"created_at"`
	Entries    int       `json:"entries"`
	TotalBytes int64     `json:"total_bytes"`
	PackSize   int64     `json:"pack_size"`
}

// Manifest — Pack 的返回值（也是 VerifyPack 从盘上读回来的东西）。
type Manifest struct {
	PackPath     string      // 包文件绝对/传入路径
	Header       PackHeader  // 第 0 行
	Entries      []PackEntry // 第 1..N 行（按 Path 升序）
	ManifestFile string      // 侧车 manifest 的落点
}

// ManifestPath — 包对应的侧车 manifest 路径（`<包>.manifest`）。VerifyPack 靠它找 manifest，
// 所以命名必须是**确定**的：不接受"调用方另给一个路径"，否则读端可能验的是另一份 manifest。
func ManifestPath(packPath string) string { return packPath + PackManifestSuffix }

// packItem — 收集阶段的一条（比 PackEntry 多带写 tar 需要的源路径与 mtime）。
type packItem struct {
	entry PackEntry
	src   string
	mtime time.Time
}

// Pack — 把 srcDir（茧根）按白名单打进 outPath（tar），并把逐件 sha256 写到侧车 manifest。
//
// 顺序（崩在任何一步都不留"看着像成品"的东西）：收集并全树拒件 → 写 `<out>.tmp` →
// fsync → 写 `<out>.manifest.tmp` → fsync → rename 包 → rename manifest → （默认）自验。
func Pack(srcDir, outPath string, opts PackOptions) (Manifest, error) {
	var zero Manifest

	root, err := filepath.Abs(srcDir)
	if err != nil {
		return zero, fmt.Errorf("%w: %v", ErrPackRoot, err)
	}
	// 源根本身：lstat，链接即拒（跟过去打的是别人的树）。
	rst, err := os.Lstat(root)
	if err != nil {
		return zero, fmt.Errorf("%w: %v", ErrPackRoot, err)
	}
	if rst.Mode()&os.ModeSymlink != 0 || !rst.IsDir() {
		return zero, fmt.Errorf("%w: %s", ErrPackRoot, root)
	}

	out, err := filepath.Abs(outPath)
	if err != nil {
		return zero, fmt.Errorf("cocoon: 输出路径非法: %v", err)
	}
	if st, err := os.Stat(out); err == nil && st.IsDir() {
		out = filepath.Join(out, DefaultPackName)
	}
	// 包落在源树内 ⇒ 收件时会把 `.tmp` 也走一遍，且自验读到的已不是同一棵树。直接拒。
	if rel, err := filepath.Rel(root, out); err == nil && rel != "." &&
		!strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != ".." {
		return zero, fmt.Errorf("%w: %s", ErrPackOutputInside, out)
	}

	allow := opts.Allow
	if len(allow) == 0 {
		allow = DefaultAllow
	}
	pats := make([]string, 0, len(allow)+len(opts.Artifacts))
	pats = append(pats, allow...)
	pats = append(pats, opts.Artifacts...)
	for _, p := range pats {
		if err := validateAllowPattern(p); err != nil {
			return zero, err
		}
	}

	items, err := collectPackItems(root, pats)
	if err != nil {
		return zero, err
	}
	if len(items) == 0 {
		return zero, fmt.Errorf("%w: 白名单 %v", ErrPackEmpty, pats)
	}

	now := Now()
	if opts.Now != nil {
		now = opts.Now().UTC()
	}

	// ① 包 → tmp（同目录，rename 才原子）。
	if err := os.MkdirAll(filepath.Dir(out), dirPerm); err != nil {
		return zero, fmt.Errorf("cocoon: 建输出目录: %w", err)
	}
	packTmp := out + ".tmp"
	manTmp := ManifestPath(out) + ".tmp"
	_ = os.Remove(packTmp)
	_ = os.Remove(manTmp)

	entries, packSize, err := writePackTar(packTmp, items)
	if err != nil {
		_ = os.Remove(packTmp)
		return zero, err
	}

	hdr := PackHeader{
		Format:    PackFormat,
		Version:   PackFormatVersion,
		CreatedAt: now,
		Entries:   len(entries),
		PackSize:  packSize,
	}
	for _, e := range entries {
		hdr.TotalBytes += e.Size
	}

	if err := writeManifest(manTmp, hdr, entries); err != nil {
		_ = os.Remove(packTmp)
		_ = os.Remove(manTmp)
		return zero, err
	}

	if err := os.Rename(packTmp, out); err != nil {
		_ = os.Remove(packTmp)
		_ = os.Remove(manTmp)
		return zero, fmt.Errorf("cocoon: 落包: %w", err)
	}
	if err := os.Rename(manTmp, ManifestPath(out)); err != nil {
		_ = os.Remove(manTmp)
		_ = os.Remove(out)
		return zero, fmt.Errorf("cocoon: 落 manifest: %w", err)
	}

	m := Manifest{
		PackPath:     out,
		Header:       hdr,
		Entries:      entries,
		ManifestFile: ManifestPath(out),
	}

	if !opts.NoSelfVerify {
		// 自验：读写同源的第一道兑现。不过就把刚落的两个文件都删掉——不留"在盘上但验不过"的包。
		if err := VerifyPack(out); err != nil {
			_ = os.Remove(out)
			_ = os.Remove(ManifestPath(out))
			return zero, fmt.Errorf("cocoon: 自验失败（已删包）: %w", err)
		}
	}
	return m, nil
}

// collectPackItems — 全树走一遍：先判拒件（与白名单无关），再判收件。
func collectPackItems(root string, pats []string) ([]packItem, error) {
	var items []packItem
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)

		// lstat 语义：WalkDir 自己**不跟**符号链接，d.Info() 拿到的是链接本身的 stat。
		info, err := d.Info()
		if err != nil {
			return err
		}
		mode := info.Mode()

		// ① 拒件（先于收件：白名单命中也不能救）。
		if mode&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: %s", ErrPackSymlink, rel)
		}
		if mode&(os.ModeSetuid|os.ModeSetgid) != 0 {
			return fmt.Errorf("%w: %s (%04o)", ErrPackSetuid, rel, uint32(mode.Perm())|modeBits(mode))
		}
		if d.IsDir() {
			// `.git`：其 config 是凭据面 ⇒ 硬拒；其余（含 hooks/）整棵不收，也不下去走
			// （大仓的 objects 走一遍没有意义，且里面没有"能进包"的东西）。
			if path.Base(rel) == ".git" {
				cfg := filepath.Join(p, "config")
				if _, err := os.Lstat(cfg); err == nil {
					return fmt.Errorf("%w: %s", ErrPackForbidden, rel+"/config")
				} else if !errors.Is(err, fs.ErrNotExist) {
					return err
				}
				return fs.SkipDir
			}
			return nil
		}
		if !mode.IsRegular() {
			return fmt.Errorf("%w: %s", ErrPackSpecial, rel)
		}
		// ①' 硬链（同属拒件）：目录天然 nlink ≥ 2，所以这一判**必须在 IsRegular 之后**。
		// `ln ~/.ssh/id_rsa workspace/x` 造出来的是普通文件、无链接位，只有 nlink 能认出来；
		// 内容会被原样复制进包 ⇒ 一律拒（与白名单无关），错误里只点名路径与计数，不碰内容。
		nl, err := stNlink(info)
		if err != nil {
			return fmt.Errorf("%w: %s（nlink 读不出: %v）", ErrPackHardlink, rel, err)
		}
		if nl > 1 {
			return fmt.Errorf("%w: %s（nlink=%d）", ErrPackHardlink, rel, nl)
		}

		// ② 结构性不收。
		if path.Base(rel) == "cocoon.toml" {
			return nil
		}
		if path.Base(rel) == MetaFileName {
			// meta.json 是宿主态（phase/端口/pid），不是茧内容：除非白名单明确点名，否则不收。
			if !matchAllow(rel, pats) {
				return nil
			}
		}
		// ③ 收件。
		if !matchAllow(rel, pats) {
			return nil
		}
		items = append(items, packItem{
			entry: PackEntry{
				Path:   rel,
				Size:   info.Size(),
				Mode:   fmt.Sprintf("%04o", mode.Perm()),
				SHA256: "", // 写 tar 时边读边算
			},
			src:   p,
			mtime: info.ModTime(),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(items, func(i, j int) bool { return items[i].entry.Path < items[j].entry.Path })
	return items, nil
}

// modeBits — 把 ModeSetuid/ModeSetgid 折成 0o4000/0o2000，只为错误信息里报得准。
func modeBits(m fs.FileMode) uint32 {
	var b uint32
	if m&os.ModeSetuid != 0 {
		b |= 0o4000
	}
	if m&os.ModeSetgid != 0 {
		b |= 0o2000
	}
	return b
}

// writePackTar — 写 tar 到 path，返回逐件 sha 与包字节数。
func writePackTar(path string, items []packItem) ([]PackEntry, int64, error) {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, filePerm)
	if err != nil {
		return nil, 0, fmt.Errorf("cocoon: 建包: %w", err)
	}
	defer f.Close()

	tw := tar.NewWriter(f)
	entries := make([]PackEntry, 0, len(items))
	for _, it := range items {
		// 收集与读取之间的缝：重开前再 stat 一次，开完 fstat 比 inode（见 openPackSrc）。
		src, st, err := openPackSrc(it.src, it.entry)
		if err != nil {
			return nil, 0, err
		}
		perm, _ := parsePerm(it.entry.Mode)

		h := &tar.Header{
			Typeflag: tar.TypeReg,
			Name:     it.entry.Path,
			Size:     st.Size(),
			Mode:     int64(perm),
			ModTime:  st.ModTime().UTC().Truncate(time.Second),
			Uid:      0, // 不把宿主 uid/gid 与用户名带进制品
			Gid:      0,
			Uname:    "",
			Gname:    "",
			Format:   tar.FormatPAX,
		}
		if err := tw.WriteHeader(h); err != nil {
			src.Close()
			return nil, 0, fmt.Errorf("cocoon: 写 tar 头（%s）: %w", it.entry.Path, err)
		}

		hsh := sha256.New()
		n, cErr := io.CopyN(io.MultiWriter(tw, hsh), src, st.Size())
		src.Close()
		if cErr != nil {
			return nil, 0, fmt.Errorf("%w: %s（读 %d/%d 字节: %v）",
				ErrPackChanged, it.entry.Path, n, st.Size(), cErr)
		}
		e := it.entry
		e.Size = st.Size()
		e.SHA256 = hex.EncodeToString(hsh.Sum(nil))
		entries = append(entries, e)
	}
	if err := tw.Close(); err != nil {
		return nil, 0, fmt.Errorf("cocoon: 封 tar: %w", err)
	}
	if err := f.Sync(); err != nil {
		return nil, 0, fmt.Errorf("cocoon: fsync 包: %w", err)
	}
	if err := f.Close(); err != nil {
		return nil, 0, fmt.Errorf("cocoon: 关包: %w", err)
	}
	st, err := os.Stat(path)
	if err != nil {
		return nil, 0, err
	}
	return entries, st.Size(), nil
}

// openPackSrc — 重开待打包件并钉住身份：lstat（判型/判位/判大小）→ open → fstat 比 inode。
// 中间被换成符号链接时，open 跟过去的目标与 lstat 的 dev/ino 不同 ⇒ 这里抓住。
func openPackSrc(p string, want PackEntry) (*os.File, os.FileInfo, error) {
	li, err := os.Lstat(p)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %s: %v", ErrPackChanged, want.Path, err)
	}
	if li.Mode()&os.ModeSymlink != 0 {
		return nil, nil, fmt.Errorf("%w: %s", ErrPackSymlink, want.Path)
	}
	if li.Mode()&(os.ModeSetuid|os.ModeSetgid) != 0 {
		return nil, nil, fmt.Errorf("%w: %s", ErrPackSetuid, want.Path)
	}
	if !li.Mode().IsRegular() {
		return nil, nil, fmt.Errorf("%w: %s", ErrPackSpecial, want.Path)
	}
	if li.Size() != want.Size || fmt.Sprintf("%04o", li.Mode().Perm()) != want.Mode {
		return nil, nil, fmt.Errorf("%w: %s（大小/权限在收集后变了）", ErrPackChanged, want.Path)
	}
	f, err := os.Open(p)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %s: %v", ErrPackChanged, want.Path, err)
	}
	fi, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, nil, fmt.Errorf("%w: %s: %v", ErrPackChanged, want.Path, err)
	}
	if !os.SameFile(li, fi) || fi.Size() != want.Size {
		f.Close()
		return nil, nil, fmt.Errorf("%w: %s（inode 在收集后被换掉）", ErrPackChanged, want.Path)
	}
	// 收集之后被挂上硬链（或读到时才看得出的那些）⇒ 同样拒。判的是**已开 fd 的 fstat**，
	// 比路径上的 lstat 更贴"我即将读的这个 inode"。
	nl, err := stNlink(fi)
	if err != nil {
		f.Close()
		return nil, nil, fmt.Errorf("%w: %s（nlink 读不出: %v）", ErrPackHardlink, want.Path, err)
	}
	if nl > 1 {
		f.Close()
		return nil, nil, fmt.Errorf("%w: %s（nlink=%d）", ErrPackHardlink, want.Path, nl)
	}
	return f, fi, nil
}

// writeManifest — 写侧车 manifest（JSONL，第 0 行头）。
func writeManifest(p string, hdr PackHeader, entries []PackEntry) error {
	f, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, filePerm)
	if err != nil {
		return fmt.Errorf("cocoon: 建 manifest: %w", err)
	}
	defer f.Close()
	bw := bufio.NewWriterSize(f, 64<<10)
	enc := json.NewEncoder(bw)
	if err := enc.Encode(hdr); err != nil { // 第 0 行
		return fmt.Errorf("cocoon: 写 manifest 头: %w", err)
	}
	for _, e := range entries {
		if err := enc.Encode(e); err != nil {
			return fmt.Errorf("cocoon: 写 manifest 件（%s）: %w", e.Path, err)
		}
	}
	if err := bw.Flush(); err != nil {
		return fmt.Errorf("cocoon: flush manifest: %w", err)
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("cocoon: fsync manifest: %w", err)
	}
	return f.Close()
}

// readManifest — 读回头与逐件（第 0 行必须是头；行数必须与头里的 entries 相等）。
func readManifest(p string) (PackHeader, []PackEntry, error) {
	var hdr PackHeader
	f, err := os.Open(p)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return hdr, nil, fmt.Errorf("%w: %s", ErrManifestMissing, p)
		}
		return hdr, nil, fmt.Errorf("cocoon: 开 manifest: %w", err)
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64<<10), 1<<20) // 路径可能很长
	line := 0
	var entries []PackEntry
	for sc.Scan() {
		b := sc.Bytes()
		if len(strings.TrimSpace(string(b))) == 0 {
			line++
			continue
		}
		if line == 0 {
			if err := json.Unmarshal(b, &hdr); err != nil {
				return hdr, nil, fmt.Errorf("%w: 第 0 行不是头: %v", ErrManifestFormat, err)
			}
			if hdr.Format != PackFormat {
				return hdr, nil, fmt.Errorf("%w: format=%q（要 %q）", ErrManifestFormat, hdr.Format, PackFormat)
			}
			if hdr.Version < 1 || hdr.Version > PackFormatVersion {
				return hdr, nil, fmt.Errorf("%w: version=%d（要 1..%d）", ErrManifestFormat, hdr.Version, PackFormatVersion)
			}
			line++
			continue
		}
		var e PackEntry
		if err := json.Unmarshal(b, &e); err != nil {
			return hdr, nil, fmt.Errorf("%w: 第 %d 行不成件: %v", ErrManifestFormat, line, err)
		}
		if err := validatePackPath(e.Path); err != nil {
			return hdr, nil, err
		}
		if e.SHA256 == "" || e.Size < 0 {
			return hdr, nil, fmt.Errorf("%w: 件 %q 缺 sha256/大小非法", ErrManifestFormat, e.Path)
		}
		entries = append(entries, e)
		line++
	}
	if err := sc.Err(); err != nil {
		return hdr, nil, fmt.Errorf("cocoon: 读 manifest: %w", err)
	}
	if line == 0 {
		return hdr, nil, fmt.Errorf("%w: %s 是空文件", ErrManifestFormat, p)
	}
	if hdr.Entries != len(entries) {
		return hdr, nil, fmt.Errorf("%w: 头记 %d 件，逐件是 %d 件", ErrPackMismatch, hdr.Entries, len(entries))
	}
	return hdr, entries, nil
}

// VerifyPack — 逐件校验：包里每件的字节 sha256/大小/权限 == manifest；件数双向相等；
// 包里不许有未登记的件、不许有符号链接/特殊件/SUID、路径不许逃出根。
func VerifyPack(packPath string) error {
	packPath, err := filepath.Abs(packPath)
	if err != nil {
		return err
	}
	hdr, entries, err := readManifest(ManifestPath(packPath))
	if err != nil {
		return err
	}
	byPath := make(map[string]PackEntry, len(entries))
	for _, e := range entries {
		if _, dup := byPath[e.Path]; dup {
			return fmt.Errorf("%w: manifest 里 %q 重复", ErrPackMismatch, e.Path)
		}
		byPath[e.Path] = e
	}

	f, err := os.Open(packPath)
	if err != nil {
		return fmt.Errorf("cocoon: 开包: %w", err)
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return fmt.Errorf("cocoon: stat 包: %w", err)
	}
	if !st.Mode().IsRegular() {
		return fmt.Errorf("%w: 包不是普通文件", ErrPackMismatch)
	}
	if hdr.PackSize > 0 && st.Size() != hdr.PackSize {
		return fmt.Errorf("%w: 包大小 %d ≠ manifest 头 %d", ErrPackMismatch, st.Size(), hdr.PackSize)
	}

	tr := tar.NewReader(f)
	seen := make(map[string]PackEntry, len(entries))
	var total int64
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("%w: 解析 tar: %v", ErrPackMismatch, err)
		}
		name := path.Clean(strings.TrimPrefix(h.Name, "./"))
		if err := validatePackPath(name); err != nil {
			return err
		}
		if h.Typeflag != tar.TypeReg && h.Typeflag != tar.TypeRegA {
			return fmt.Errorf("%w: 包里 %q 不是普通文件（typeflag=%q）", ErrPackSpecial, name, h.Typeflag)
		}
		if h.Mode&0o6000 != 0 {
			return fmt.Errorf("%w: 包里 %q 带 SUID/SGID（mode=%04o）", ErrPackSetuid, name, h.Mode)
		}
		want, ok := byPath[name]
		if !ok {
			return fmt.Errorf("%w: 包里多出未登记的件 %q", ErrPackMismatch, name)
		}
		if _, dup := seen[name]; dup {
			return fmt.Errorf("%w: 包里 %q 重复", ErrPackMismatch, name)
		}
		hsh := sha256.New()
		n, err := io.Copy(hsh, tr)
		if err != nil {
			return fmt.Errorf("%w: 读 %q: %v", ErrPackMismatch, name, err)
		}
		got := hex.EncodeToString(hsh.Sum(nil))
		if n != want.Size || h.Size != want.Size {
			return fmt.Errorf("%w: %q 大小 包=%d 头=%d manifest=%d", ErrPackMismatch, name, n, h.Size, want.Size)
		}
		if got != want.SHA256 {
			return fmt.Errorf("%w: %q sha256 包=%s manifest=%s", ErrPackMismatch, name, got, want.SHA256)
		}
		if m := fmt.Sprintf("%04o", h.Mode&0o777); m != want.Mode {
			return fmt.Errorf("%w: %q 权限 包=%s manifest=%s", ErrPackMismatch, name, m, want.Mode)
		}
		seen[name] = want
		total += n
	}
	for p := range byPath {
		if _, ok := seen[p]; !ok {
			return fmt.Errorf("%w: 包里缺件 %q", ErrPackMismatch, p)
		}
	}
	if hdr.TotalBytes != total {
		return fmt.Errorf("%w: 总字节 包=%d manifest 头=%d", ErrPackMismatch, total, hdr.TotalBytes)
	}
	return nil
}

// validatePackPath — 件路径的硬规则：相对、slash、无 `..`、无空段、无 NUL/反斜杠。
func validatePackPath(p string) error {
	if p == "" || p == "." {
		return fmt.Errorf("%w: 空路径", ErrPackUnsafePath)
	}
	if strings.HasPrefix(p, "/") || strings.ContainsRune(p, 0) || strings.Contains(p, "\\") {
		return fmt.Errorf("%w: %q", ErrPackUnsafePath, p)
	}
	if filepath.IsAbs(p) || filepath.VolumeName(p) != "" {
		return fmt.Errorf("%w: %q", ErrPackUnsafePath, p)
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return fmt.Errorf("%w: %q", ErrPackUnsafePath, p)
		}
	}
	return nil
}

// validateAllowPattern — 白名单写法：不许绝对路径、不许 `..`、不许整根（"."/""）。
func validateAllowPattern(p string) error {
	s := strings.TrimSpace(filepath.ToSlash(p))
	if s == "" || s == "." || s == "/" {
		return fmt.Errorf("%w: 白名单 %q 等于整根", ErrPackUnsafePath, p)
	}
	if strings.HasPrefix(s, "/") || filepath.IsAbs(s) || filepath.VolumeName(s) != "" {
		return fmt.Errorf("%w: 白名单 %q 是绝对路径", ErrPackUnsafePath, p)
	}
	for _, seg := range strings.Split(strings.TrimSuffix(s, "/"), "/") {
		if seg == ".." || seg == "" || seg == "." {
			return fmt.Errorf("%w: 白名单 %q 含非法段", ErrPackUnsafePath, p)
		}
	}
	return nil
}

// matchAllow — 白名单匹配（语义见文件抬头 ★白名单语义）。
func matchAllow(rel string, pats []string) bool {
	for _, raw := range pats {
		p := strings.TrimSuffix(filepath.ToSlash(strings.TrimSpace(raw)), "/")
		if p == "" {
			continue
		}
		if base, ok := strings.CutSuffix(p, "/**"); ok {
			if rel == base || strings.HasPrefix(rel, base+"/") {
				return true
			}
			continue
		}
		if strings.ContainsAny(p, "*?[") {
			if ok, err := path.Match(p, rel); err == nil && ok {
				return true
			}
			continue
		}
		if rel == p || strings.HasPrefix(rel, p+"/") {
			return true
		}
	}
	return false
}

// parsePerm — "0644" → 0o644（manifest 读回/写 tar 用；坏值按 0644 兜底，绝不放大权限）。
func parsePerm(s string) (fs.FileMode, error) {
	if len(s) != 4 {
		return filePerm, fmt.Errorf("cocoon: 权限串 %q 不是四位八进制", s)
	}
	var m fs.FileMode
	for _, c := range s {
		if c < '0' || c > '7' {
			return filePerm, fmt.Errorf("cocoon: 权限串 %q 含非八进制字符", s)
		}
		m = m<<3 | fs.FileMode(c-'0')
	}
	return m, nil
}
