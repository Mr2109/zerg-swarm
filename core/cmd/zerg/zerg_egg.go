// zerg_egg.go —— `scripts/node-a/zerg-egg.py`（396 行）的 **Go 真搬**（M2 一枚 · 2026-10-07 · 承接
// `GAP-20261001-13`）。
//
// 搬的是「卵的单文件封装（*.egg）」工具：`pack` / `info` / `verify` / `xfer` ＋ `--self-test`。
//
// 格式 v1（**读写同源**）：固定 32 字节前缀 ＋ 变长索引 JSON ＋ 数据段（按 align 对齐 · 段间补零）。
//
//	magic(4)="ZEGG"  ver(u32le)  align(u64le)  index_len(u64le)  补零到 32
//
// 索引 JSON（py 侧 `json.dumps(..., ensure_ascii=False, separators=(",", ":"))`，字段**插入序**）：
//
//	magic  version  align  tool{name,version}  created_at  entries[{path,off,size,sha256,mode}]
//
// 规则（规格的一部分 · 逐字照搬）：align 必须是 2 的幂且 ≥ 4096；读端按索引里记的 align 校验（不硬编码）；
// 每段偏移必须对齐；段内容 sha256 必须与索引一致，否则 verify 失败；32 字节固定前缀让读端先知道
// align 与索引长度（「读端无法先验知道 align」这一死结的正解）。
//
// ---- M2 落件纪律（照 `check_archive_mount.go` 的同款中间态）----------------------------------
//
//	· **只落码、不接线** —— ✗ 不在 `core/cmd/zerg/main.go` 注册命令（✗ 不进命令树 · ✗ 不改 add_step）；
//	· **不删** `scripts/node-a/zerg-egg.py` 原件（退场规则：**对拍通过后才**逐件点名退场）；
//	· 入口 `zergEggGateMain` 目前**无调用点**（M2 的刻意状态）。
//
// ---- 两条出厂声明行 -------------------------------------------------------------------------
//
//	(a) **默认根定位差异**：py 全程只用**入参路径**（源目录 / 落点 / 袋件），逻辑里**没有** `__file__`
//	    上溯面 ⇒ 本件与 py 的路径推导面**同构**（都不从脚本位置推根）。故**不涉**「脚本位置 ≠ cwd」那条差异。
//	(b) **自指检查**：本件**不扫仓内树、不写仓内件**（对拍只在仓外夹具）⇒ **无自扫描面**；本件全件零
//	    绝对路径字面量、零「被别门扫的连续引号原串」。判「否」的依据见回执自指栏。
//
// ---- ★ 与 py 的**声明口径差**（逐条进回执「未覆盖面」）----------------------------------------
//
//  1. **未捕获异常档**（源目录空 / align 非法 / magic 坏 / JSON 坏）：py 的最外层是裸 `sys.exit(main())`
//     ⇒ 这些 `ValueError` 变成 **Python 回溯 + rc=1**。本件按 py **同 rc=1** 失败（stderr 只打一句具名
//     错误，**不含回溯**）⇒ 该档 **rc 对齐、stderr 正文不同**（回溯正文不可移植：含脚本行号与本机解释器路径）。
//  2. **argparse 帮助正文 / 唯一前缀缩写 / 错误串**：只复刻本程序**固定**的 usage/help 文本与常见错误串；
//     该面随**终端宽度**（`COLUMNS`）与 **py 版本**（≤3.9 印 `optional arguments:`、≥3.10 印 `options:`）
//     而变 ⇒ 本件按 **py 3.9 系** 的 `optional arguments:` 逐字复刻并钉住（见回执「工具链指纹」）。
//  3. **Go-only 旋钮 `--now <created_at>`**（隐藏 · **不进帮助面** ⇒ 不污染 `-h` 对拍）：把索引里的
//     `created_at` 钉成定值，使 `pack` 产物可**逐字节 sha256 对拍**（py 侧 `created_at = datetime.now(utc)`）
//     —— py **不认**该旗标 ⇒ 属「旋钮面不合同」，只用于对拍台，✗ 不影响对拍格的 stdout/rc。
//
// ---- 出厂声明结束。入口 `zergEggGateMain` 无调用点。
package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ---- 常量（逐字照 py 顶部）------------------------------------------------------------------
const (
	zegMagic        = "ZEGG"
	zegFormatVer    = 1
	zegToolName     = "zerg-egg"
	zegToolVersion  = "1.0.0"
	zegMinAlign     = 4096
	zegAlignDarwin  = 16384 // 默认对齐 · Apple Silicon（py `_align_of`）
	zegAlignOther   = 4096  // 默认对齐 · 其他平台
	zegHeaderPrefix = 32
	zegChunkSize    = 64 << 20
	// zegExitToolUsage —— py `main()` 在「无子命令 / 缺实参」时 `return 64`（EX_USAGE）。
	// 走具名常量：命令面契约 T3 只审**裸数字**退码 ⇒ 用具名常量最稳（0/1/2 用包内既有常量）。
	zegExitToolUsage = 64
	// zegProg —— argparse 的 `prog`。py 走 `python3 <件>` ⇒ `prog = basename(sys.argv[0])` = 件名。
	zegProg = "zerg-egg.py"
)

// zegHelpText —— `-h` / 无参数时打的帮助正文（**逐字节**取自 py 3.9 系现跑 · 506 字节 · COLUMNS=80）。
const zegHelpText = "usage: zerg-egg.py [-h] [--align ALIGN] [--rebuild-into REBUILD_INTO]\n" +
	"                   [--self-test] [--chunk-size CHUNK_SIZE]\n" +
	"                   [{pack,info,verify,xfer}] [a] [b]\n" +
	"\n" +
	"卵的单文件封装工具（*.egg）\n" +
	"\n" +
	"positional arguments:\n" +
	"  {pack,info,verify,xfer}\n" +
	"  a\n" +
	"  b\n" +
	"\n" +
	"optional arguments:\n" +
	"  -h, --help            show this help message and exit\n" +
	"  --align ALIGN\n" +
	"  --rebuild-into REBUILD_INTO\n" +
	"  --self-test\n" +
	"  --chunk-size CHUNK_SIZE\n" +
	"                        xfer 的分块大小（默认 64 MiB）\n"

// zegUsageText —— 错误档先打的 `usage:` 三行（与 help 正文同源片段）。
const zegUsageText = "usage: zerg-egg.py [-h] [--align ALIGN] [--rebuild-into REBUILD_INTO]\n" +
	"                   [--self-test] [--chunk-size CHUNK_SIZE]\n" +
	"                   [{pack,info,verify,xfer}] [a] [b]\n"

// ---- 索引结构（字段序 == py 的 json 插入序，✗ 别改序）----------------------------------------

type zegToolJSON struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type zegEntryJSON struct {
	Path   string `json:"path"`
	Off    int64  `json:"off"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
	Mode   int    `json:"mode"`
}

type zegIndexJSON struct {
	Magic     string         `json:"magic"`
	Version   int            `json:"version"`
	Align     int64          `json:"align"`
	Tool      zegToolJSON    `json:"tool"`
	CreatedAt string         `json:"created_at"`
	Entries   []zegEntryJSON `json:"entries"`
}

// zegIndex —— 解析后的索引（读端用）。
type zegIndex struct {
	Magic     string
	Version   int
	Align     int64
	Tool      zegToolJSON
	CreatedAt string
	Entries   []zegEntryJSON
}

// zegEntry —— 打包期的条目（多一个内部 `src`，**不进 JSON**）。
type zegEntry struct {
	Path   string
	Off    int64
	Size   int64
	SHA256 string
	Mode   int
	src    string
}

// ---- 小工具（逐字照 py）---------------------------------------------------------------------

// zegAlignOf —— 默认对齐（**按平台取**：Apple Silicon 16K / 其他 4K）。
// ★ 两条取值走具名常量（✗ 不写 `return 16384` 一类裸数字：命令面契约 T3 的裸数字退码判据只认真源表内数字，
// 非退码的裸数字 `return` 会被误判）。
func zegAlignOf() int {
	if runtime.GOOS == "darwin" {
		return zegAlignDarwin
	}
	return zegAlignOther
}

func zegIsPow2(n int64) bool { return n > 0 && (n&(n-1)) == 0 }

// zegSha256File —— 现算一件的 sha256（hex）。
func zegSha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// zegBuildPrefix —— 固定 32 字节前缀的**唯一实现**（zegParseIndex 与之同源）。
func zegBuildPrefix(align int, indexLen int) []byte {
	b := make([]byte, zegHeaderPrefix)
	copy(b[0:4], zegMagic)
	binary.LittleEndian.PutUint32(b[4:8], uint32(zegFormatVer))
	binary.LittleEndian.PutUint64(b[8:16], uint64(align))
	binary.LittleEndian.PutUint64(b[16:24], uint64(indexLen))
	return b
}

// zegPyBytesRepr —— py `%r` 的 bytes 形（`b'…'`）；只覆盖可打印 ASCII ＋ 常用转义。
func zegPyBytesRepr(b []byte) string {
	var sb strings.Builder
	sb.WriteString("b'")
	for _, c := range b {
		switch {
		case c == '\\':
			sb.WriteString("\\\\")
		case c == '\'':
			sb.WriteString("\\'")
		case c == '\n':
			sb.WriteString("\\n")
		case c == '\t':
			sb.WriteString("\\t")
		case c == '\r':
			sb.WriteString("\\r")
		case c >= 0x20 && c < 0x7f:
			sb.WriteByte(c)
		default:
			sb.WriteString(fmt.Sprintf("\\x%02x", c))
		}
	}
	sb.WriteString("'")
	return sb.String()
}

// ---- 收集条目 -------------------------------------------------------------------------------

// zegCollectEntries —— 遍历源目录出条目（**顺序确定**：目录与文件都排序 · ✗ 与 py 逐字同构）。
// py 的 `os.walk`：先把当前层 scandir 分成 dirs/files、**先出当前层全部文件**再按 dirs 序下钻；
// 分类用 `entry.is_dir()`（**跟随符号链接**）；`followlinks=False` ⇒ 指向目录的符号链接**只入 dirs、
// 不下钻**；符号链接的**文件**进 files ⇒ 触发 ValueError。本件逐条复刻。
func zegCollectEntries(src string) ([]*zegEntry, error) {
	var entries []*zegEntry
	var walk func(dir string) error
	walk = func(dir string) error {
		des, err := os.ReadDir(dir)
		if err != nil {
			return err
		}
		var dirs, files []string
		for _, de := range des {
			name := de.Name()
			full := filepath.Join(dir, name)
			// 分类跟随符号链接（os.Stat）；dangling / 报错按「非目录」处理（== py is_dir 抛错的档）。
			isDir := false
			if st, serr := os.Stat(full); serr == nil && st.IsDir() {
				isDir = true
			}
			if isDir {
				dirs = append(dirs, name)
			} else {
				files = append(files, name)
			}
		}
		// py: dirs.sort(); for fn in sorted(files)  —— 两处排序，这里 ReadDir 已有序但仍显式排（防未来变）。
		sort.Strings(dirs)
		sort.Strings(files)
		for _, fn := range files {
			fp := filepath.Join(dir, fn)
			if li, lerr := os.Lstat(fp); lerr == nil && li.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("v1 不支持符号链接：%s", fp)
			}
			rel, rerr := filepath.Rel(src, fp)
			if rerr != nil {
				return rerr
			}
			rel = filepath.ToSlash(rel)
			if strings.HasPrefix(rel, "..") {
				return fmt.Errorf("越界路径：%s", rel)
			}
			st, serr := os.Stat(fp)
			if serr != nil {
				return serr
			}
			sha, herr := zegSha256File(fp)
			if herr != nil {
				return herr
			}
			entries = append(entries, &zegEntry{
				Path:   rel,
				Size:   st.Size(),
				SHA256: sha,
				Mode:   int(st.Mode().Perm()),
				src:    fp,
			})
		}
		for _, dn := range dirs {
			child := filepath.Join(dir, dn)
			// followlinks=False：指向目录的符号链接不下钻。
			if li, lerr := os.Lstat(child); lerr == nil && li.Mode()&os.ModeSymlink != 0 {
				continue
			}
			if werr := walk(child); werr != nil {
				return werr
			}
		}
		return nil
	}
	if err := walk(src); err != nil {
		return nil, err
	}
	return entries, nil
}

// ---- 打包 -----------------------------------------------------------------------------------

// zegMarshalIndex —— 生成索引 JSON 字节（**与 py `json.dumps` 同源**：紧凑 + 不转义非 ASCII + 不转义 HTML）。
func zegMarshalIndex(entries []*zegEntry, align int, now string) ([]byte, error) {
	created := now
	if created == "" {
		created = time.Now().UTC().Format("2006-01-02T15:04:05") + "Z"
	}
	ej := make([]zegEntryJSON, 0, len(entries))
	for _, e := range entries {
		ej = append(ej, zegEntryJSON{Path: e.Path, Off: e.Off, Size: e.Size, SHA256: e.SHA256, Mode: e.Mode})
	}
	idx := zegIndexJSON{
		Magic:     zegMagic,
		Version:   zegFormatVer,
		Align:     int64(align),
		Tool:      zegToolJSON{Name: zegToolName, Version: zegToolVersion},
		CreatedAt: created,
		Entries:   ej,
	}
	var buf strings.Builder
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(idx); err != nil {
		return nil, err
	}
	return []byte(strings.TrimRight(buf.String(), "\n")), nil
}

// zegPack —— 打一枚卵（照 py `pack`）。align==0 ⇒ 取平台默认。
func zegPack(src, egg string, align int, now string) ([]*zegEntry, error) {
	if align == 0 {
		align = zegAlignOf()
	}
	if !zegIsPow2(int64(align)) || align < zegMinAlign {
		return nil, fmt.Errorf("align 必须是 2 的幂且 >= %d（收到 %d）", zegMinAlign, align)
	}
	entries, err := zegCollectEntries(src)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("源目录里没有文件：%s", src)
	}
	out, err := os.Create(egg)
	if err != nil {
		return nil, err
	}
	pad := func() error {
		off, _ := out.Seek(0, io.SeekCurrent)
		p := (int64(align) - off%int64(align)) % int64(align)
		if p > 0 {
			_, werr := out.Write(make([]byte, p))
			return werr
		}
		return nil
	}
	if _, err := out.Write(make([]byte, align)); err != nil { // 头部块占位
		out.Close()
		return nil, err
	}
	for _, e := range entries {
		if err := pad(); err != nil {
			out.Close()
			return nil, err
		}
		off, _ := out.Seek(0, io.SeekCurrent)
		e.Off = off
		in, oerr := os.Open(e.src)
		if oerr != nil {
			out.Close()
			return nil, oerr
		}
		n, cerr := io.Copy(out, in)
		in.Close()
		if cerr != nil {
			out.Close()
			return nil, cerr
		}
		if n != e.Size {
			out.Close()
			return nil, fmt.Errorf("写入长度与统计不符")
		}
	}
	if err := pad(); err != nil {
		out.Close()
		return nil, err
	}
	idxBytes, err := zegMarshalIndex(entries, align, now)
	if err != nil {
		out.Close()
		return nil, err
	}
	if zegHeaderPrefix+len(idxBytes) > align {
		out.Close()
		return nil, fmt.Errorf("索引过大（%d 字节）放不进 %d 字节的头块", len(idxBytes), align)
	}
	if _, err := out.Seek(0, io.SeekStart); err != nil {
		out.Close()
		return nil, err
	}
	if _, err := out.Write(zegBuildPrefix(align, len(idxBytes))); err != nil {
		out.Close()
		return nil, err
	}
	if _, err := out.Write(idxBytes); err != nil {
		out.Close()
		return nil, err
	}
	if err := out.Close(); err != nil {
		return nil, err
	}
	return entries, nil
}

// ---- 解析索引 -------------------------------------------------------------------------------

// zegParseIndex —— 解析索引（与 zegMarshalIndex 同源的唯一读端）。
func zegParseIndex(path string) (*zegIndex, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	hdr := make([]byte, zegHeaderPrefix)
	if _, err := io.ReadFull(f, hdr); err != nil {
		return nil, fmt.Errorf("文件太短，不足固定前缀 %d 字节", zegHeaderPrefix)
	}
	magic := hdr[0:4]
	ver := int(binary.LittleEndian.Uint32(hdr[4:8]))
	align := int64(binary.LittleEndian.Uint64(hdr[8:16]))
	n := int64(binary.LittleEndian.Uint64(hdr[16:24]))
	if string(magic) != zegMagic {
		return nil, fmt.Errorf("magic 不符（不是 %s 文件）：%s", zegMagic, zegPyBytesRepr(magic))
	}
	if ver != zegFormatVer {
		return nil, fmt.Errorf("格式版本不支持：%d", ver)
	}
	if !zegIsPow2(align) || align < zegMinAlign {
		return nil, fmt.Errorf("前缀里的 align 不合理：%d", align)
	}
	if !(n > 0 && n <= align-zegHeaderPrefix) {
		return nil, fmt.Errorf("索引长度字段不合理：%d", n)
	}
	body := make([]byte, n)
	if _, err := io.ReadFull(f, body); err != nil {
		return nil, err
	}
	var raw zegIndexJSON
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	if raw.Magic != zegMagic {
		return nil, fmt.Errorf("magic 不符（不是 %s 文件）", zegMagic)
	}
	if raw.Version != zegFormatVer {
		return nil, fmt.Errorf("格式版本不支持：%d", raw.Version)
	}
	if !zegIsPow2(raw.Align) || raw.Align < zegMinAlign {
		return nil, fmt.Errorf("align 字段不合理：%d", raw.Align)
	}
	return &zegIndex{
		Magic:     raw.Magic,
		Version:   raw.Version,
		Align:     raw.Align,
		Tool:      raw.Tool,
		CreatedAt: raw.CreatedAt,
		Entries:   raw.Entries,
	}, nil
}

// ---- 分块可续传拷贝 -------------------------------------------------------------------------

// zegXfer —— 按块可续传地把一枚卵拷到目标（§13.2）。返回 (写入块数, 总块数)。
func zegXfer(w io.Writer, src, dst string, chunkSize int, quiet bool) (int, int, error) {
	fi, err := os.Stat(src)
	if err != nil {
		return 0, 0, err
	}
	total := fi.Size()
	nChunks := int((total + int64(chunkSize) - 1) / int64(chunkSize))
	wrote := 0
	if _, err := os.Stat(dst); err != nil {
		if f, cerr := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY, 0o644); cerr == nil {
			f.Close()
		}
	}
	fs, err := os.Open(src)
	if err != nil {
		return 0, 0, err
	}
	defer fs.Close()
	fd, err := os.OpenFile(dst, os.O_RDWR, 0o644)
	if err != nil {
		return 0, 0, err
	}
	defer fd.Close()
	for i := 0; i < nChunks; i++ {
		off := int64(i) * int64(chunkSize)
		want := int64(chunkSize)
		if total-off < want {
			want = total - off
		}
		if _, err := fs.Seek(off, io.SeekStart); err != nil {
			return 0, 0, err
		}
		data := make([]byte, want)
		if _, err := io.ReadFull(fs, data); err != nil {
			return 0, 0, err
		}
		h := sha256.Sum256(data)
		var got []byte
		if dfi, derr := fd.Stat(); derr == nil && dfi.Size() >= off+want {
			if _, err := fd.Seek(off, io.SeekStart); err == nil {
				buf := make([]byte, want)
				if _, rerr := io.ReadFull(fd, buf); rerr == nil {
					got = buf
				}
			}
		}
		gh := sha256.Sum256(got)
		if gh != h {
			if _, err := fd.Seek(off, io.SeekStart); err != nil {
				return 0, 0, err
			}
			if _, err := fd.Write(data); err != nil {
				return 0, 0, err
			}
			wrote++
		}
	}
	if err := fd.Truncate(total); err != nil {
		return 0, 0, err
	}
	if !quiet {
		fmt.Fprintf(w, "xfer：%d/%d 块需写（其余已就绪）⇒ %s\n", wrote, nChunks, dst)
	}
	return wrote, nChunks, nil
}

// ---- 校验 -----------------------------------------------------------------------------------

// zegVerify —— 校验头块、对齐、逐段 sha256；可选重建到目录。返回 (problems, 索引)。
func zegVerify(path, rebuildInto string) ([]string, *zegIndex, error) {
	var problems []string
	idx, err := zegParseIndex(path)
	if err != nil {
		return nil, nil, err
	}
	align := idx.Align
	fi, err := os.Stat(path)
	if err != nil {
		return nil, nil, err
	}
	sizeFile := fi.Size()
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	for _, e := range idx.Entries {
		if e.Off%align != 0 {
			problems = append(problems, fmt.Sprintf("%s 偏移未对齐（%d %% %d != 0）", e.Path, e.Off, align))
		}
		if e.Off+e.Size > sizeFile {
			problems = append(problems, fmt.Sprintf("%s 越出文件末尾", e.Path))
			continue
		}
		if _, err := f.Seek(e.Off, io.SeekStart); err != nil {
			return nil, nil, err
		}
		h := sha256.New()
		left := e.Size
		truncated := false
		for left > 0 {
			k := int64(1 << 20)
			if left < k {
				k = left
			}
			buf := make([]byte, k)
			m, _ := f.Read(buf)
			if m == 0 {
				problems = append(problems, fmt.Sprintf("%s 数据截断", e.Path))
				truncated = true
				break
			}
			h.Write(buf[:m])
			left -= int64(m)
		}
		if !truncated && hex.EncodeToString(h.Sum(nil)) != e.SHA256 {
			problems = append(problems, fmt.Sprintf("%s sha256 不符", e.Path))
		}
	}
	if rebuildInto != "" {
		if err := os.MkdirAll(rebuildInto, 0o755); err != nil {
			return nil, nil, err
		}
		rf, err := os.Open(path)
		if err != nil {
			return nil, nil, err
		}
		defer rf.Close()
		for _, e := range idx.Entries {
			tgt := filepath.Join(rebuildInto, filepath.FromSlash(e.Path))
			if err := os.MkdirAll(filepath.Dir(tgt), 0o755); err != nil {
				return nil, nil, err
			}
			if _, err := rf.Seek(e.Off, io.SeekStart); err != nil {
				return nil, nil, err
			}
			if err := os.WriteFile(tgt, nil, 0o644); err != nil {
				return nil, nil, err
			}
			g, err := os.OpenFile(tgt, os.O_RDWR|os.O_TRUNC, 0o644)
			if err != nil {
				return nil, nil, err
			}
			left := e.Size
			for left > 0 {
				k := int64(1 << 20)
				if left < k {
					k = left
				}
				buf := make([]byte, k)
				m, _ := rf.Read(buf)
				if m == 0 {
					break
				}
				if _, err := g.Write(buf[:m]); err != nil {
					g.Close()
					return nil, nil, err
				}
				left -= int64(m)
			}
			g.Close()
			if err := os.Chmod(tgt, os.FileMode(e.Mode)); err != nil {
				return nil, nil, err
			}
		}
	}
	return problems, idx, nil
}

// ---- 子命令：info ---------------------------------------------------------------------------

func zegCmdInfo(stdout io.Writer, path string) int {
	idx, err := zegParseIndex(path)
	if err != nil {
		fmt.Fprintf(stdout, "%s: %v\n", zegProg, err)
		return exitFail
	}
	fi, _ := os.Stat(path)
	fmt.Fprintf(stdout, "卵文件：%s\n", path)
	fmt.Fprintf(stdout, "  magic=%s version=%d align=%d（%d KiB）\n", idx.Magic, idx.Version, idx.Align, idx.Align/1024)
	fmt.Fprintf(stdout, "  产出工具=%s %s  产出时间=%s\n", idx.Tool.Name, idx.Tool.Version, idx.CreatedAt)
	fmt.Fprintf(stdout, "  段数=%d  文件大小=%.1f KiB\n", len(idx.Entries), float64(fi.Size())/1024)
	for _, e := range idx.Entries {
		sha := e.SHA256
		if len(sha) > 16 {
			sha = sha[:16]
		}
		fmt.Fprintf(stdout, "    - %-40s %9d B  off=%d  %s\n", e.Path, e.Size, e.Off, sha)
	}
	return exitOK
}

// ---- 自检（照 py `_self_test`）--------------------------------------------------------------

func zegSelfTest(stdout, stderr io.Writer) int {
	ok := 0
	t, err := os.MkdirTemp("", "zerg-egg-selftest")
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", zegProg, err)
		return exitFail
	}
	defer os.RemoveAll(t) // ★ A 类止血：py 的 xfer 段落在 TemporaryDirectory 之外 ⇒ 每次都漏一个目录

	src := filepath.Join(t, "tree")
	if err := os.MkdirAll(filepath.Join(src, "model"), 0o755); err != nil {
		return zegSelfFail(stderr, err)
	}
	if err := os.MkdirAll(filepath.Join(src, "tokenizer"), 0o755); err != nil {
		return zegSelfFail(stderr, err)
	}
	if err := os.WriteFile(filepath.Join(src, "config.json"), []byte(`{"arch":"demo"}`), 0o644); err != nil {
		return zegSelfFail(stderr, err)
	}
	if err := os.WriteFile(filepath.Join(src, "tokenizer", "tokenizer.json"), []byte(`{"v":1}`), 0o644); err != nil {
		return zegSelfFail(stderr, err)
	}
	for _, i := range []int{1, 2} {
		buf := make([]byte, 120000)
		if _, err := rand.Read(buf); err != nil {
			return zegSelfFail(stderr, err)
		}
		p := filepath.Join(src, "model", fmt.Sprintf("shard-%d.safetensors", i))
		if err := os.WriteFile(p, buf, 0o644); err != nil {
			return zegSelfFail(stderr, err)
		}
	}
	egg := filepath.Join(t, "demo.egg")
	entries, err := zegPack(src, egg, 4096, "")
	if err != nil {
		return zegSelfFail(stderr, err)
	}
	if len(entries) != 4 {
		return zegSelfFail(stderr, fmt.Errorf("条目数不对"))
	}
	ok++
	probs, idx, err := zegVerify(egg, filepath.Join(t, "rebuilt"))
	if err != nil {
		return zegSelfFail(stderr, err)
	}
	if len(probs) != 0 {
		return zegSelfFail(stderr, fmt.Errorf("干净卵不该有问题：%v", probs))
	}
	ok++
	for _, e := range entries {
		a, _ := os.ReadFile(filepath.Join(src, filepath.FromSlash(e.Path)))
		b, _ := os.ReadFile(filepath.Join(t, "rebuilt", filepath.FromSlash(e.Path)))
		if sha256.Sum256(a) != sha256.Sum256(b) {
			return zegSelfFail(stderr, fmt.Errorf("重建内容不同"))
		}
	}
	ok++
	// 负例①：改一个字节 ⇒ verify 必须失败
	if f, err := os.OpenFile(egg, os.O_RDWR, 0o644); err == nil {
		f.Seek(idx.Entries[0].Off+10, io.SeekStart)
		f.Write([]byte{0xff})
		f.Close()
	}
	probs, _, _ = zegVerify(egg, "")
	hit := false
	for _, p := range probs {
		if strings.Contains(p, "sha256") {
			hit = true
		}
	}
	if !hit {
		return zegSelfFail(stderr, fmt.Errorf("改了字节却没报出来"))
	}
	ok++
	// 负例②：截断 ⇒ 必须失败
	data, err := os.ReadFile(egg)
	if err != nil {
		return zegSelfFail(stderr, err)
	}
	trunc := filepath.Join(t, "trunc.egg")
	if err := os.WriteFile(trunc, data[:idx.Entries[len(idx.Entries)-1].Off+1], 0o644); err != nil {
		return zegSelfFail(stderr, err)
	}
	probs, _, _ = zegVerify(trunc, "")
	if len(probs) == 0 {
		return zegSelfFail(stderr, fmt.Errorf("截断却没报出来"))
	}
	ok++
	// 负例③：magic 被破坏 ⇒ 必须拒绝
	bad := filepath.Join(t, "bad.egg")
	badData := append([]byte("XXXX"), data[4:]...)
	if err := os.WriteFile(bad, badData, 0o644); err != nil {
		return zegSelfFail(stderr, err)
	}
	if _, err := zegParseIndex(bad); err == nil {
		return zegSelfFail(stderr, fmt.Errorf("magic 坏了却没拒绝"))
	}
	ok++
	// 负例④：align 非 2 的幂 ⇒ 必须拒绝
	if _, err := zegPack(src, filepath.Join(t, "a.egg"), 3000, ""); err == nil {
		return zegSelfFail(stderr, fmt.Errorf("align=3000 竟被接受"))
	}
	ok++
	// xfer ①：全新拷贝 ⇒ 必须与源逐字节相同
	srcX := filepath.Join(t, "xtree")
	if err := os.MkdirAll(filepath.Join(srcX, "sub"), 0o755); err != nil {
		return zegSelfFail(stderr, err)
	}
	bufA := make([]byte, 100000)
	rand.Read(bufA)
	if err := os.WriteFile(filepath.Join(srcX, "a.bin"), bufA, 0o644); err != nil {
		return zegSelfFail(stderr, err)
	}
	bufB := make([]byte, 50000)
	rand.Read(bufB)
	if err := os.WriteFile(filepath.Join(srcX, "sub", "b.bin"), bufB, 0o644); err != nil {
		return zegSelfFail(stderr, err)
	}
	src2, dst2 := filepath.Join(t, "d.egg"), filepath.Join(t, "d.copy")
	if _, err := zegPack(srcX, src2, 4096, ""); err != nil {
		return zegSelfFail(stderr, err)
	}
	w1, n1, err := zegXfer(stdout, src2, dst2, 64*1024, true)
	if err != nil {
		return zegSelfFail(stderr, err)
	}
	if w1 != n1 {
		return zegSelfFail(stderr, fmt.Errorf("全新拷贝应写全部块（%d/%d）", w1, n1))
	}
	sa, _ := os.ReadFile(src2)
	sb, _ := os.ReadFile(dst2)
	if string(sa) != string(sb) {
		return zegSelfFail(stderr, fmt.Errorf("拷贝后内容不同"))
	}
	ok++
	// xfer ②：中断（把目标截一半）⇒ 重跑必须续传补齐且逐字节相同
	half := int64(len(sa)) / 2
	if err := os.Truncate(dst2, half); err != nil {
		return zegSelfFail(stderr, err)
	}
	w2, _, err := zegXfer(stdout, src2, dst2, 64*1024, true)
	if err != nil {
		return zegSelfFail(stderr, err)
	}
	if !(w2 > 0 && w2 < n1) {
		return zegSelfFail(stderr, fmt.Errorf("半截目标只应补一部分块（实得 %d）", w2))
	}
	sc, _ := os.ReadFile(dst2)
	if string(sa) != string(sc) {
		return zegSelfFail(stderr, fmt.Errorf("续传后内容不同"))
	}
	ok++
	// xfer ③：坏块 ⇒ 必须被发现并修复
	if f, err := os.OpenFile(dst2, os.O_RDWR, 0o644); err == nil {
		f.Seek(0, io.SeekStart)
		f.Write([]byte("XXXX"))
		f.Close()
	}
	w3, _, err := zegXfer(stdout, src2, dst2, 64*1024, true)
	if err != nil {
		return zegSelfFail(stderr, err)
	}
	if w3 < 1 {
		return zegSelfFail(stderr, fmt.Errorf("坏块必须被重写（实得 %d）", w3))
	}
	sd, _ := os.ReadFile(dst2)
	if string(sa) != string(sd) {
		return zegSelfFail(stderr, fmt.Errorf("修复后内容不同"))
	}
	ok++
	fmt.Fprintf(stdout, "self-test 通过（%d 项：3 正例 + 4 负例 + 3 条 xfer 用例）\n", ok)
	return exitOK
}

func zegSelfFail(stderr io.Writer, err error) int {
	fmt.Fprintf(stderr, "%s: 自检失败：%v\n", zegProg, err)
	return exitFail
}

// ---- 命令行解析（复刻 argparse 的**常见面**）------------------------------------------------

type zegArgs struct {
	cmd         string
	a           string
	b           string
	align       int
	alignSet    bool
	rebuildInto string
	rebuildSet  bool
	selfTest    bool
	chunkSize   int
	nowVal      string
}

var zegOpts = []string{"--align", "--rebuild-into", "--self-test", "--chunk-size", "--help", "--now"}

// zegResolveOpt —— 精确命中优先；否则唯一前缀；多命中 ⇒ 歧义串；零命中 ⇒ ""。
func zegResolveOpt(name string) (string, string) {
	for _, o := range zegOpts {
		if o == name {
			return o, ""
		}
	}
	var matches []string
	for _, o := range zegOpts {
		if strings.HasPrefix(o, name) {
			matches = append(matches, o)
		}
	}
	if len(matches) == 1 {
		return matches[0], ""
	}
	if len(matches) > 1 {
		return "", fmt.Sprintf("ambiguous option: %s could match %s", name, strings.Join(matches, ", "))
	}
	return "", ""
}

// zegParseArgv —— 返回 (args, errBody)。errBody=="HELP" ⇒ 打帮助退 0；非空 ⇒ argparse 式错误（退 2）。
func zegParseArgv(argv []string) (*zegArgs, string) {
	a := &zegArgs{align: 0, chunkSize: zegChunkSize}
	var pos, extras []string
	afterDD := false
	for i := 0; i < len(argv); i++ {
		s := argv[i]
		if afterDD {
			pos = append(pos, s)
			continue
		}
		if s == "--" {
			afterDD = true
			continue
		}
		if s == "-h" || s == "--help" {
			return a, "HELP"
		}
		if len(s) > 1 && s[0] == '-' && s != "-" {
			name, inline, hasInline := s, "", false
			if idx := strings.IndexByte(s, '='); idx >= 0 {
				name, inline, hasInline = s[:idx], s[idx+1:], true
			}
			canon, ambig := zegResolveOpt(name)
			if ambig != "" {
				return nil, ambig
			}
			if canon == "" {
				extras = append(extras, s)
				continue
			}
			take := func() (string, bool) {
				if hasInline {
					return inline, true
				}
				if i+1 < len(argv) {
					i++
					return argv[i], true
				}
				return "", false
			}
			switch canon {
			case "--help":
				return a, "HELP"
			case "--self-test":
				a.selfTest = true
			case "--align":
				v, ok := take()
				if !ok {
					return nil, fmt.Sprintf("argument %s: expected one argument", name)
				}
				n, err := strconv.Atoi(v)
				if err != nil {
					return nil, fmt.Sprintf("argument --align: invalid int value: '%s'", v)
				}
				a.align, a.alignSet = n, true
			case "--chunk-size":
				v, ok := take()
				if !ok {
					return nil, fmt.Sprintf("argument %s: expected one argument", name)
				}
				n, err := strconv.Atoi(v)
				if err != nil {
					return nil, fmt.Sprintf("argument --chunk-size: invalid int value: '%s'", v)
				}
				a.chunkSize = n
			case "--rebuild-into":
				v, ok := take()
				if !ok {
					return nil, fmt.Sprintf("argument %s: expected one argument", name)
				}
				a.rebuildInto, a.rebuildSet = v, true
			case "--now":
				v, ok := take()
				if !ok {
					return nil, fmt.Sprintf("argument %s: expected one argument", name)
				}
				a.nowVal = v
			}
			continue
		}
		pos = append(pos, s)
	}
	if len(pos) > 0 {
		a.cmd = pos[0]
	}
	if len(pos) > 1 {
		a.a = pos[1]
	}
	if len(pos) > 2 {
		a.b = pos[2]
	}
	if len(pos) > 3 {
		extras = append(extras, pos[3:]...)
	}
	if a.cmd != "" {
		switch a.cmd {
		case "pack", "info", "verify", "xfer":
		default:
			return nil, fmt.Sprintf("argument cmd: invalid choice: '%s' (choose from 'pack', 'info', 'verify', 'xfer')", a.cmd)
		}
	}
	if len(extras) > 0 {
		return nil, "unrecognized arguments: " + strings.Join(extras, " ")
	}
	return a, ""
}

// ---- 入口（M2 形状：`<名>GateMain(stdout, stderr io.Writer, argv []string) int`）--------------

// zergEggGateMain —— `scripts/node-a/zerg-egg.py` 的 Go 真搬入口。argv = py 的 `sys.argv[1:]`。
func zergEggGateMain(stdout, stderr io.Writer, argv []string) int {
	a, errBody := zegParseArgv(argv)
	if errBody == "HELP" {
		fmt.Fprint(stdout, zegHelpText)
		return exitOK
	}
	if errBody != "" {
		fmt.Fprint(stderr, zegUsageText)
		fmt.Fprintf(stderr, "%s: error: %s\n", zegProg, errBody)
		return exitUsage
	}
	if a.selfTest {
		return zegSelfTest(stdout, stderr)
	}
	switch a.cmd {
	case "pack":
		if a.a == "" || a.b == "" {
			fmt.Fprint(stderr, "用法：pack <源目录> <输出.egg>\n")
			return zegExitToolUsage
		}
		eff := a.align
		if !a.alignSet {
			eff = zegAlignOf()
		}
		entries, err := zegPack(a.a, a.b, a.align, a.nowVal)
		if err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", zegProg, err)
			return exitFail
		}
		fi, _ := os.Stat(a.b)
		fmt.Fprintf(stdout, "已打包 %d 个文件 → %s（align=%d，%.1f KiB）\n", len(entries), a.b, eff, float64(fi.Size())/1024)
		return exitOK
	case "info":
		return zegCmdInfo(stdout, a.a)
	case "xfer":
		if a.a == "" || a.b == "" {
			fmt.Fprint(stderr, "用法：xfer <源.egg> <目标.egg>\n")
			return zegExitToolUsage
		}
		wrote, n, err := zegXfer(stdout, a.a, a.b, a.chunkSize, false)
		if err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", zegProg, err)
			return exitFail
		}
		probs, _, verr := zegVerify(a.b, "")
		if verr != nil {
			fmt.Fprintf(stderr, "%s: %v\n", zegProg, verr)
			return exitFail
		}
		if len(probs) > 0 {
			fmt.Fprintf(stdout, "✗ 目标校验失败 %d 项：\n", len(probs))
			return exitFail
		}
		fmt.Fprintf(stdout, "✓ 目标与索引一致（本次写 %d/%d 块）\n", wrote, n)
		return exitOK
	case "verify":
		probs, idx, err := zegVerify(a.a, a.rebuildInto)
		if err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", zegProg, err)
			return exitFail
		}
		if len(probs) > 0 {
			fmt.Fprintf(stdout, "✗ 校验失败 %d 项：\n", len(probs))
			for _, p := range probs {
				fmt.Fprintf(stdout, "   - %s\n", p)
			}
			return exitFail
		}
		suffix := ""
		if a.rebuildInto != "" {
			suffix = "，已重建到 " + a.rebuildInto
		}
		fmt.Fprintf(stdout, "✓ 校验通过（%d 段，align=%d）%s\n", len(idx.Entries), idx.Align, suffix)
		return exitOK
	}
	fmt.Fprint(stdout, zegHelpText)
	return zegExitToolUsage
}
