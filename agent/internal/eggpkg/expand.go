// expand.go —— 蛋（`*.egg` v2）的**展开器**：把段表里的每一段真正落到盘上，并逐段复核 sha256。
//
// 为什么要有它（卵沙箱第 6 步）：`ReadIndex` / `BuildPlan` 只**读**不写（v1 面），core 侧的
// `cocoon verify --rebuild-into` 是**另一条**路径（且不产 `.complete`、不拒非空目录）——
// 子端（孵化侧）此前没有任何执行者能把一枚蛋落到 `<egg_dir>` 并复核 ⇒ 「原模型目录 /
// 原引擎目录可删」就缺这最后一块。本件补的就是它：**一个入口、一条写路径、一份凭据**。
//
// 格式口径（与 `core/cmd/zerg/zerg_egg.go` 的 v2 写端 **同源**；本件只读 + 只落段）：
//
//	[0, 8 KiB)          双槽头（两槽各 4 KiB）：magic / ver / gen / file_len / table_off /
//	                    table_len / align / uuid16 / table_sha256 / crc32c（前 96 字节的 Castagnoli 校验）
//	[align, file_len)   数据段（段之间按 align 补零）
//	[table_off, +len)   尾段表（JSON · 与 v1 同一份键形状 ＋ 可加面键 mediaType / linkTarget）
//
// 不变量（违背即拒，逐条具名 —— 头里的每个字段都当**不可信输入**判）：
//
//	· 头：magic = "ZEGG"、布局主号 = 2、两槽里**至少一槽** crc32c 有效（取 gen 最大者）；
//	· align 是 2 的幂且 ≥ 4096（只信文件里记的值，✗ 不按运行平台硬编码）；
//	· 表落点：8 KiB 之后、64 KiB 对齐、不出文件末尾；表长 ∈ (0, 64 MiB]；
//	· 表字节的 sha256 == 头槽里记的 table_sha256；表里的 version / align 与头**逐值一致**；
//	· 段：`path` 是规范化的相对路径（✗ 绝对 / ✗ 含 `..` / ✗ 含 `\` / ✗ 非 NFC 近似形态）、
//	  `[off, off+size)` 落在数据区内、`sha256` 为 64 位十六进制；
//	· 符号链接段：`linkTarget` 非空 ⇒ `size = 0`、链名**相对**、解析后仍在蛋根内、层数 ≤ 8、✗ 成环。
//
// 落盘纪律（本件的**全部写面**都在 `ExpandEgg` 里，别的函数一律只读）：
//
//	· 目标目录已存在且**有内容** ⇒ 拒（✗ 不静默覆盖）；
//	· 目标目录里已有 `.complete` 且与这枚蛋**同一代**（gen ＋ uuid ＋ table_sha256 ＋ 段数 ＋
//	  整件字节）⇒ **跳过**（幂等：不重写、不报错）；`gen` 变了（升引擎）⇒ 拒，✗ 不覆盖旧展开；
//	· 只剩 `.failed` / `.expanding` 这类标记 ⇒ 允许重试（清掉标记再来一遍）；
//	· 每段先写**临时名** → `fsync` → `rename`（不留半截件）；每段落盘后**重算 sha256** 与段表比对；
//	· 全段落完才写 `.complete` —— 它是「这一目录已完整」的**唯一凭据**（内含整件 sha256 与段数）；
//	· 任一段失败 ⇒ **回滚本次已落的件**：本次建的根且已空 ⇒ 连根删掉（✗ 不留半截目录）；
//	  根是调用方给的、删不掉的 ⇒ 留一枚 `.failed`（含原因与段名，可恢复、✗ 不假装成功）。
package eggpkg

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

// v2 布局常量（★ 与 `core/cmd/zerg/zerg_egg.go` 的同名常量**逐值同源**；改一处必须改另一处）。
const (
	// eggMagicV2 —— 头槽里的 magic（与 v1 的 `Magic` 同一个四字节串，布局主号区分两代）。
	eggMagicV2 = "ZEGG"
	// eggLayoutV2 —— v2 布局主号（v1 = 1：32 字节前缀 ＋ 变长索引；本件**只认** v2）。
	eggLayoutV2 = 2
	// eggHeaderBytes / eggSlotBytes —— 固定头 8 KiB ＝ 双槽 × 4 KiB。
	eggHeaderBytes = 8 << 10
	eggSlotBytes   = 4 << 10
	// eggTableAlign —— 段表落点：`file_len` 之后第一个 64 KiB 边界。
	eggTableAlign = 64 << 10
	// eggTableMaxBytes —— 读段表时的分配上界（防畸形头把内存吃光）。
	eggTableMaxBytes = 64 << 20
	// eggMinAlign —— 段对齐下限（与写端 `zegMinAlign` 同值）。
	eggMinAlign = 4096
	// eggMaxSymlinkDepth / eggMaxSymlinkTarget —— 链层数上限 / 链名字节上限（与写端同值）。
	eggMaxSymlinkDepth  = 8
	eggMaxSymlinkTarget = 4096
	// eggMarkerComplete / eggMarkerFailed / eggMarkerExpanding —— 展开目录里的三枚标记件。
	// ★ `.complete` 是「已完整」的唯一凭据；另两枚只说「这趟来过 / 失败过」，✗ 不充作完整凭据。
	//
	// ★ GAP-20261018-15：`.expanding` 同时是**进度面**（内容是一份 `ExpandProgress` JSON）。
	// 它仍是单写者锁（O_EXCL 建），只是把「已展开多少」也写进同一枚件 —— 展开期间观测面
	// 因此读得到 `expanding` ＋ 字节进度，而不必等 `.complete`（首次展开实测 471 s）。
	eggMarkerComplete  = ".complete"
	eggMarkerFailed    = ".failed"
	eggMarkerExpanding = ".expanding"
	// eggMarkerExpandingTmp —— 进度件的原子写临时名（rename 后即消失；目录判定里当标记件忽略）。
	eggMarkerExpandingTmp = ".expanding.tmp"
	// eggExpandStateExpanding —— 进度面里的中间态词（`/eggs` 的 `expand.state` 逐字用它）。
	eggExpandStateExpanding = "expanding"
	// eggReceiptVersion —— `.complete` 的 schema 主号（内容改了它才动）。
	eggReceiptVersion = 1
)

// 头槽内字段偏移（little endian · 槽 = 4096 B：前 96 B 语义 ＋ 4 B crc32c，其余留后续可加面）。
const (
	eggOffMagic    = 0
	eggOffLayout   = 4
	eggOffGen      = 8
	eggOffFileLen  = 16
	eggOffTableOff = 24
	eggOffTableLen = 32
	eggOffAlign    = 40
	eggOffUUID     = 48
	eggOffTableSHA = 64
	eggOffCRC      = 96
	eggCRCSpan     = 96
)

// EggSegment —— 段表里的一段（一个文件的完整内容，或一条符号链接）。
type EggSegment struct {
	Path       string `json:"path"`
	Off        int64  `json:"off"`
	Size       int64  `json:"size"`
	SHA256     string `json:"sha256"`
	Mode       uint32 `json:"mode"`
	MediaType  string `json:"mediaType,omitempty"`
	LinkTarget string `json:"linkTarget,omitempty"`
}

// IsSymlink —— 该段是不是符号链接（`linkTarget` 非空即链 · 语义等价 tar 的 `TypeSymlink='2'`）。
func (s EggSegment) IsSymlink() bool { return s.LinkTarget != "" }

// EggIndex —— 一枚 v2 蛋的**头 ＋ 段表**（读端解析结果；不含载荷字节）。
type EggIndex struct {
	Path     string // 卵件路径（入参原样）
	Bytes    int64  // 卵件总字节（stat 来的）
	Align    int64  // 段对齐（只信头里记的值）
	Gen      int64  // 代际（头槽里最大的那个有效代）
	Slot     int    // 取中的是哪个槽（0 / 1）
	UUID     string // 卵唯一号（16 字节 → 32 位十六进制）
	FileLen  int64  // 数据区末尾
	TableOff int64  // 段表落点
	TableLen int64  // 段表长度
	TableSHA string // 段表字节的 sha256（头槽里记的）
	Entries  []EggSegment
	// DataBytes —— 各段 `size` 之和（符号链接段为 0）。★ 展开后**逐件核字节总数**就看它。
	DataBytes int64
}

// eggSlot —— 一个头槽的内容（读端只认「crc 有效且 gen 最大」）。
type eggSlot struct {
	Gen      int64
	FileLen  int64
	TableOff int64
	TableLen int64
	Align    int64
	UUID     [16]byte
	TableSHA [32]byte
	Slot     int
}

// eggCRC32C —— 头槽校验（Castagnoli 多项式，与写端 `zegV2CRC32C` 同表）。
func eggCRC32C(b []byte) uint32 { return crc32.Checksum(b, crc32.MakeTable(crc32.Castagnoli)) }

// eggDecodeSlot —— 解一个槽；magic / 布局主号 / crc 任一不符 ⇒ `(槽, false)`（**坏槽不参与择优**）。
func eggDecodeSlot(b []byte, slot int) (eggSlot, bool) {
	var s eggSlot
	if len(b) < eggSlotBytes || string(b[eggOffMagic:eggOffMagic+4]) != eggMagicV2 {
		return s, false
	}
	if binary.LittleEndian.Uint32(b[eggOffLayout:]) != uint32(eggLayoutV2) {
		return s, false
	}
	if eggCRC32C(b[:eggCRCSpan]) != binary.LittleEndian.Uint32(b[eggOffCRC:]) {
		return s, false
	}
	s.Gen = int64(binary.LittleEndian.Uint64(b[eggOffGen:]))
	s.FileLen = int64(binary.LittleEndian.Uint64(b[eggOffFileLen:]))
	s.TableOff = int64(binary.LittleEndian.Uint64(b[eggOffTableOff:]))
	s.TableLen = int64(binary.LittleEndian.Uint64(b[eggOffTableLen:]))
	s.Align = int64(binary.LittleEndian.Uint64(b[eggOffAlign:]))
	copy(s.UUID[:], b[eggOffUUID:eggOffUUID+16])
	copy(s.TableSHA[:], b[eggOffTableSHA:eggOffTableSHA+32])
	s.Slot = slot
	return s, true
}

// eggPickSlot —— 读 8 KiB 头，取「**crc 有效且 gen 最大**」那个槽（★ 读端只看这一条规则）。
func eggPickSlot(head []byte) (eggSlot, bool) {
	s0, ok0 := eggDecodeSlot(head[0:eggSlotBytes], 0)
	s1, ok1 := eggDecodeSlot(head[eggSlotBytes:2*eggSlotBytes], 1)
	switch {
	case ok0 && ok1:
		if s1.Gen > s0.Gen {
			return s1, true
		}
		return s0, true
	case ok0:
		return s0, true
	case ok1:
		return s1, true
	}
	return eggSlot{}, false
}

// eggIsPow2 —— `n` 是不是 2 的正整数幂。
func eggIsPow2(n int64) bool { return n > 0 && (n&(n-1)) == 0 }

// eggAlignUp —— 向上取整到 a（a 为 2 的幂；a ≤ 1 ⇒ 原样）。
func eggAlignUp(n, a int64) int64 {
	if a <= 1 {
		return n
	}
	if r := n % a; r != 0 {
		return n + (a - r)
	}
	return n
}

// ---- 路径规则（与 core 侧 `zegPathRules` **同表的口径**；本件只列展开端真正需要的那些）----

// eggWindowsReserved —— Windows 保留名（与写端同表 —— 蛋是**跨机**搬运的，落点也要经得起 Windows 解包）。
var eggWindowsReserved = map[string]bool{
	"con": true, "prn": true, "aux": true, "nul": true,
	"com1": true, "com2": true, "com3": true, "com4": true, "com5": true,
	"com6": true, "com7": true, "com8": true, "com9": true,
	"lpt1": true, "lpt2": true, "lpt3": true, "lpt4": true, "lpt5": true,
	"lpt6": true, "lpt7": true, "lpt8": true, "lpt9": true,
}

// eggHasCombiningMark —— 是否含**组合记号**（`Mn` 的主用区段）。不引 `x/text/unicode/norm`
// （本仓硬线：✗ 不引入新依赖）⇒ 用 fail-closed 近似：规范化后的路径不该含这些区段。
func eggHasCombiningMark(s string) bool {
	for _, r := range s {
		switch {
		case r >= 0x0300 && r <= 0x036F,
			r >= 0x1AB0 && r <= 0x1AFF,
			r >= 0x1DC0 && r <= 0x1DFF,
			r >= 0x20D0 && r <= 0x20FF,
			r >= 0xFE20 && r <= 0xFE2F:
			return true
		}
	}
	return false
}

// eggRefusePath —— 段 `path` 的**展开端**判据（命中即具名拒；规则名与 core 侧逐字同名，
// 测试与门能按类点名）。★ 展开端**必须**自己判一遍：✗ 只信打包端（那是「双向校验」的读端一半）。
func eggRefusePath(rel string) error {
	if rel == "" {
		return fmt.Errorf("拒：段 path 为空 —— 命中规则 path_empty（空路径）")
	}
	if strings.HasPrefix(rel, "/") {
		return fmt.Errorf("拒：段 path %q —— 命中规则 path_absolute（绝对路径：展开落点会跑到蛋根之外）", rel)
	}
	if len(rel) > 1 && rel[1] == ':' &&
		((rel[0] >= 'a' && rel[0] <= 'z') || (rel[0] >= 'A' && rel[0] <= 'Z')) {
		return fmt.Errorf("拒：段 path %q —— 命中规则 path_windows_drive（盘符）", rel)
	}
	if strings.HasPrefix(rel, "//") || strings.HasPrefix(rel, `\\`) {
		return fmt.Errorf("拒：段 path %q —— 命中规则 path_unc（UNC 形）", rel)
	}
	if strings.Contains(rel, `\`) {
		return fmt.Errorf("拒：段 path %q —— 命中规则 path_backslash（反斜杠混入）", rel)
	}
	if strings.ContainsRune(rel, 0) {
		return fmt.Errorf("拒：段 path %q —— 命中规则 path_nul（含 NUL 字节）", rel)
	}
	for _, seg := range strings.Split(rel, "/") {
		if seg == ".." {
			return fmt.Errorf("拒：段 path %q —— 命中规则 path_dotdot（`..` 路径穿越：落到蛋根之外）", rel)
		}
	}
	if path.Clean(rel) != rel {
		return fmt.Errorf("拒：段 path %q —— 命中规则 path_not_normalized（`.` 段 / 空段 / 尾斜杠）", rel)
	}
	for _, seg := range strings.Split(rel, "/") {
		if seg == "" {
			continue
		}
		if strings.HasSuffix(seg, ".") || strings.HasSuffix(seg, " ") {
			return fmt.Errorf("拒：段 path %q —— 命中规则 path_trailing_dot_space（段以点或空格结尾）", rel)
		}
		head := seg
		if i := strings.IndexByte(head, '.'); i >= 0 {
			head = head[:i]
		}
		if eggWindowsReserved[strings.ToLower(head)] {
			return fmt.Errorf("拒：段 path %q —— 命中规则 path_windows_reserved（Windows 保留名）", rel)
		}
	}
	if !utf8.ValidString(rel) {
		return fmt.Errorf("拒：段 path %q —— 命中规则 path_not_utf8（不是合法 UTF-8）", rel)
	}
	if eggHasCombiningMark(rel) {
		return fmt.Errorf("拒：段 path %q —— 命中规则 path_not_nfc（含组合记号，归一化碰撞的病灶）", rel)
	}
	return nil
}

// ---- 符号链接（与写端 `zegSymlinkResolve` / `zegRefuseSymlinkSet` 同口径：只许相对链）----

// eggSymlinkResolve —— 把一条链的**原始相对链名**按「所在目录」解成根相对 slash 路径。
// 返回 (解出的根相对路径, 是否仍在蛋根内)。★ 纯算术：不读盘、不跟随（✗ `EvalSymlinks` = TOCTOU 入口）。
func eggSymlinkResolve(rel, target string) (string, bool) {
	joined := path.Join(path.Dir(rel), filepath.ToSlash(target))
	clean := filepath.Clean(filepath.FromSlash(joined))
	back, err := filepath.Rel(".", clean)
	if err != nil {
		return filepath.ToSlash(clean), false
	}
	back = filepath.ToSlash(back)
	if back == ".." || strings.HasPrefix(back, "../") {
		return back, false
	}
	return back, true
}

// eggRefuseSymlinkSet —— 整张段表里**全部**符号链接的联判：先逐条过「绝对 / 越界」，
// 再按链图走一遍「成环 / 超深」。★ 展开端**再判一次**（✗ 只信打包端与读端）。
func eggRefuseSymlinkSet(links map[string]string) error {
	if len(links) == 0 {
		return nil
	}
	rels := make([]string, 0, len(links))
	for rel := range links {
		rels = append(rels, rel)
	}
	sort.Strings(rels)
	for _, rel := range rels {
		target := links[rel]
		subj := fmt.Sprintf("段 %q 的符号链接（→ %q）", rel, target)
		if target == "" || len(target) > eggMaxSymlinkTarget || strings.ContainsRune(target, 0) {
			return fmt.Errorf("拒：%s —— 命中规则 fs_symlink_entry_invalid（链名为空 / 超长）", subj)
		}
		if strings.HasPrefix(target, "/") || filepath.IsAbs(target) {
			return fmt.Errorf("拒：%s —— 命中规则 fs_symlink_absolute（只许相对链）", subj)
		}
		if next, inside := eggSymlinkResolve(rel, target); !inside {
			return fmt.Errorf("拒：%s —— 命中规则 fs_symlink_escape（解析到 %q，越出蛋根）", subj, next)
		}
	}
	for _, rel := range rels {
		seen := map[string]bool{rel: true}
		cur := rel
		depth := 1
		for {
			next, _ := eggSymlinkResolve(cur, links[cur])
			if _, isLink := links[next]; !isLink {
				break
			}
			if seen[next] {
				return fmt.Errorf("拒：段 %q 的符号链接 —— 命中规则 fs_symlink_cycle（链走到 %q 又回到已访过的段）", rel, next)
			}
			seen[next] = true
			depth++
			if depth > eggMaxSymlinkDepth {
				return fmt.Errorf("拒：段 %q 的符号链接 —— 命中规则 fs_symlink_depth（链层数 %d > %d）", rel, depth, eggMaxSymlinkDepth)
			}
			cur = next
		}
	}
	return nil
}

// eggModeOf —— 段表里的 mode → 落盘用的权限位。★ 只许权限位，✗ 禁 setuid / setgid / sticky；
// 全零权限一律拒（写出来连属主都读不到 ⇒ 一定是坏段表）。
func eggModeOf(rel string, mode uint32) (os.FileMode, error) {
	if mode&uint32(os.ModeSetuid|os.ModeSetgid|os.ModeSticky|0o7000) != 0 {
		return 0, fmt.Errorf("拒：段 %q 的 mode=%#o —— 命中规则 fs_mode_unsafe（禁 setuid / setgid / sticky）", rel, mode)
	}
	perm := os.FileMode(mode & 0o777)
	if perm&0o444 == 0 {
		return 0, fmt.Errorf("拒：段 %q 的 mode=%#o —— 没有读位（段表坏，✗ 不按它写盘）", rel, mode)
	}
	return perm, nil
}

// ---- 读端：v2 头 ＋ 尾段表 ---------------------------------------------------------------

// eggTableJSON —— 尾段表的 JSON 形状（与 v1 索引**同一份键** ＋ 两个可加面键）。
type eggTableJSON struct {
	Magic     string          `json:"magic"`
	Version   int             `json:"version"`
	Align     int64           `json:"align"`
	Tool      json.RawMessage `json:"tool"`
	CreatedAt string          `json:"created_at"`
	Entries   []EggSegment    `json:"entries"`
}

// ReadEggIndex —— 读一枚 v2 蛋的头与段表（**只读**：不落任何件、不改任何状态）。
//
// 返回值里的 `TableSHA` / `Gen` / `UUID` 是 `.complete` 幂等判据的三枚锚；`DataBytes` 是
// 「展开后逐件字节总数」的期望值。
func ReadEggIndex(eggPath string) (*EggIndex, error) {
	f, err := os.Open(eggPath)
	if err != nil {
		return nil, fmt.Errorf("卵取不到（%s）：%w", eggPath, err)
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("卵 stat 失败（%s）：%w", eggPath, err)
	}
	size := fi.Size()
	if size < eggHeaderBytes {
		return nil, fmt.Errorf("卵太短（%d 字节 < 固定头 %d）：%s —— 拒（✗ 按半截头猜）", size, eggHeaderBytes, eggPath)
	}
	head := make([]byte, eggHeaderBytes)
	if _, err := io.ReadFull(f, head); err != nil {
		return nil, fmt.Errorf("卵头读不满 %d 字节：%w", eggHeaderBytes, err)
	}
	slot, ok := eggPickSlot(head)
	if !ok {
		return nil, fmt.Errorf("卵头的双槽 crc32c 都不对（没有可提交的世代）—— 拒：%s", eggPath)
	}
	if slot.Align < eggMinAlign || !eggIsPow2(slot.Align) {
		return nil, fmt.Errorf("头槽里的 align=%d 不是 2 的幂或 < %d —— 拒（读端只信文件里记的 align）", slot.Align, eggMinAlign)
	}
	if slot.TableLen <= 0 || slot.TableLen > eggTableMaxBytes {
		return nil, fmt.Errorf("头槽里的段表长度 %d 不合理（须 ∈ (0, %d]）—— 拒", slot.TableLen, eggTableMaxBytes)
	}
	if slot.TableOff < eggHeaderBytes || slot.TableOff%eggTableAlign != 0 || slot.TableOff+slot.TableLen > size {
		return nil, fmt.Errorf("段表落点不合规（off=%d len=%d 须在 8 KiB 头之后、64 KiB 对齐、不出文件末尾 %d）—— 拒",
			slot.TableOff, slot.TableLen, size)
	}
	if slot.FileLen < eggHeaderBytes || slot.FileLen > slot.TableOff {
		return nil, fmt.Errorf("头槽里的 file_len=%d 不在 [%d, 段表落点 %d] 区间 —— 拒", slot.FileLen, eggHeaderBytes, slot.TableOff)
	}
	body := make([]byte, slot.TableLen)
	if _, err := f.ReadAt(body, slot.TableOff); err != nil {
		return nil, fmt.Errorf("段表读不满（off=%d len=%d）：%w", slot.TableOff, slot.TableLen, err)
	}
	if sum := sha256.Sum256(body); sum != slot.TableSHA {
		return nil, fmt.Errorf("段表字节的 sha256=%s ≠ 头里记的 %s —— 拒（指针指向一张坏表，✗ 不猜）",
			hex.EncodeToString(sum[:])[:16], hex.EncodeToString(slot.TableSHA[:])[:16])
	}
	var raw eggTableJSON
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("段表 JSON 解析失败：%w", err)
	}
	if raw.Magic != eggMagicV2 {
		return nil, fmt.Errorf("段表里的 magic=%q 不符（不是 %s 文件）—— 拒", raw.Magic, eggMagicV2)
	}
	if raw.Version != eggLayoutV2 {
		return nil, fmt.Errorf("段表里的 version=%d ≠ %d（头与表不一致）—— 拒", raw.Version, eggLayoutV2)
	}
	if raw.Align != slot.Align {
		return nil, fmt.Errorf("段表里的 align=%d 与头槽 align=%d 不一致 —— 拒", raw.Align, slot.Align)
	}
	if len(raw.Entries) == 0 {
		return nil, fmt.Errorf("段表里没有段 —— 拒（空蛋不是蛋）")
	}
	ix := &EggIndex{
		Path:     eggPath,
		Bytes:    size,
		Align:    slot.Align,
		Gen:      slot.Gen,
		Slot:     slot.Slot,
		UUID:     hex.EncodeToString(slot.UUID[:]),
		FileLen:  slot.FileLen,
		TableOff: slot.TableOff,
		TableLen: slot.TableLen,
		TableSHA: hex.EncodeToString(slot.TableSHA[:]),
		Entries:  raw.Entries,
	}
	seen := make(map[string]bool, len(raw.Entries))
	links := make(map[string]string, len(raw.Entries))
	for i, e := range raw.Entries {
		if err := eggRefusePath(e.Path); err != nil {
			return nil, fmt.Errorf("段 %d：%w", i, err)
		}
		if seen[e.Path] {
			return nil, fmt.Errorf("段 %d 的 path 重复：%q —— 拒（同一条路径两段 ⇒ 谁盖谁没有定义）", i, e.Path)
		}
		seen[e.Path] = true
		if e.Off < eggHeaderBytes {
			return nil, fmt.Errorf("段 %q 的 off=%d 落在固定头里 —— 拒", e.Path, e.Off)
		}
		if e.Off%ix.Align != 0 {
			return nil, fmt.Errorf("段 %q 的 off=%d 未按 align=%d 对齐 —— 拒", e.Path, e.Off, ix.Align)
		}
		if e.Size < 0 {
			return nil, fmt.Errorf("段 %q 的 size=%d 为负 —— 拒", e.Path, e.Size)
		}
		if e.Off+e.Size > slot.FileLen {
			return nil, fmt.Errorf("段 %q 的 [off,off+size)=[%d,%d) 落在数据区 [%d,%d] 之外 —— 拒",
				e.Path, e.Off, e.Off+e.Size, eggHeaderBytes, slot.FileLen)
		}
		if e.IsSymlink() {
			if e.Size != 0 {
				return nil, fmt.Errorf("段 %q 是符号链接却带 %d 字节载荷 —— 拒（链段必须 size=0）", e.Path, e.Size)
			}
			links[e.Path] = e.LinkTarget
		} else {
			if len(e.SHA256) != 64 {
				return nil, fmt.Errorf("段 %q 的 sha256 长度=%d ≠ 64 —— 拒", e.Path, len(e.SHA256))
			}
			if _, err := hex.DecodeString(e.SHA256); err != nil {
				return nil, fmt.Errorf("段 %q 的 sha256 不是十六进制：%q —— 拒", e.Path, e.SHA256)
			}
			if e.Size == 0 && e.SHA256 != eggEmptySHA {
				return nil, fmt.Errorf("段 %q 是空段但 sha256 不是空串摘要 —— 拒", e.Path)
			}
		}
		ix.DataBytes += e.Size
	}
	if err := eggRefuseSymlinkSet(links); err != nil {
		return nil, err
	}
	return ix, nil
}

// eggEmptySHA —— 空载荷的 sha256（链段 / 空文件段的期望值）。
const eggEmptySHA = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

// ---- 展开 -----------------------------------------------------------------------------

// ExpandResult —— 一次展开的读数（成功与幂等跳过共用同一形状）。
type ExpandResult struct {
	Egg          string `json:"egg"`
	Dst          string `json:"dst"`
	Skipped      bool   `json:"skipped"` // true = 目标目录里已有同一代的 `.complete` ⇒ 一个字节都没写
	Gen          int64  `json:"gen"`
	UUID         string `json:"uuid"`
	TableSHA256  string `json:"table_sha256"`
	EggSHA256    string `json:"egg_sha256"` // 整件 sha256（跳过档 = `.complete` 里记的那个）
	EggBytes     int64  `json:"egg_bytes"`
	Segments     int    `json:"segments"`
	Files        int    `json:"files"`
	Symlinks     int    `json:"symlinks"`
	Bytes        int64  `json:"bytes"` // 落盘字节总数（＝各段 size 之和；链段 0）
	CompletePath string `json:"complete_path"`
}

// eggReceipt —— `.complete` 的内容（★ 「这一目录已完整」的唯一凭据）。
//
// 幂等判据取 `Gen` ＋ `UUID` ＋ `TableSHA256` ＋ `Segments` ＋ `EggBytes` 这五枚**便宜**的锚
// （都来自 8 KiB 头与段表，不必把 84 GB 再读一遍）；`EggSHA256` 只在真展开那一趟算一次，
// 作为**出处证据**记下来（✗ 不拿它当每次孵化的前置 —— 那会把启动时间乘以整个蛋的体积）。
type eggReceipt struct {
	Version     int    `json:"version"`
	Egg         string `json:"egg"`
	EggSHA256   string `json:"egg_sha256"`
	EggBytes    int64  `json:"egg_bytes"`
	Gen         int64  `json:"gen"`
	UUID        string `json:"uuid"`
	TableSHA256 string `json:"table_sha256"`
	Segments    int    `json:"segments"`
	Bytes       int64  `json:"bytes"`
	CompletedAt string `json:"completed_at"`
}

// eggFailedReceipt —— `.failed` 的内容（失败可恢复的证据面：哪一枚蛋、哪一段、为什么）。
type eggFailedReceipt struct {
	Version     int    `json:"version"`
	Egg         string `json:"egg"`
	EggBytes    int64  `json:"egg_bytes"`
	Gen         int64  `json:"gen"`
	UUID        string `json:"uuid"`
	TableSHA256 string `json:"table_sha256"`
	Segments    int    `json:"segments"`
	Reason      string `json:"reason"`
	FailedAt    string `json:"failed_at"`
}

// ExpandEgg 把一枚 v2 蛋**真正展开**到 `dstDir`，并逐段复核 sha256。
//
// 幂等：`dstDir` 里已有同一代（gen ＋ uuid ＋ table_sha256 ＋ 段数 ＋ 整件字节都相等）的
// `.complete` ⇒ 返回 `Skipped=true`，**不重写、不报错**（一次孵化一次展开，重复调用只做
// 「读头 ＋ 读表 ＋ 读凭据」三件小事）。
//
// 拒绝（具名，逐条带「为什么不猜」）：
//
//	· `dstDir` 已存在且**有内容** ⇒ 拒（✗ 静默覆盖别人的目录）；
//	· `.complete` 属于**另一代**（蛋升过引擎 / 换过蛋）⇒ 拒（✗ 拿旧展开冒充新蛋）；
//	· 另一趟展开正在跑（`.expanding` 在）⇒ 拒（单写者）；
//	· 段表里任何一条穿越 / 越界 / 链越界 ⇒ 拒（在写第一个字节**之前**判完）。
//
// 失败：回滚本次已落的件（本次建的根且已空 ⇒ 连根删掉；否则留 `.failed`），✗ 不留半截目录。
func ExpandEgg(eggPath, dstDir string) (*ExpandResult, error) {
	return ExpandEggWithProgress(eggPath, dstDir, nil)
}

// ExpandProgress —— 一次展开的**中间态快照**（GAP-20261018-15）。
//
// 为什么要它：首次展开一枚 113 GB 的蛋在 edge-node 实测 471 s，此前这 471 s 里外部**只能看到
// 「一个字节都没写」，看不到「正在写、已写多少」** ⇒ 主控的装载判据没有依据，只能按超时
// 判红。本结构是 `.expanding` 那枚件的正文，也是 `onProgress` 回调的形状：
//
//	State          —— 恒 `expanding`（进度面里的中间态词）
//	SegmentsDone / SegmentsTotal —— 已落段数 / 段表总段数（含链段）
//	BytesDone / BytesTotal       —— 已落字节 / 数据区总字节（＝各段 size 之和）
//
// ★ 它只是**观测**：不参与完整性判定（那只有 `.complete` 配）。读的人拿它算「还要多久 /
// 是不是卡住了」，✗ 不许拿它当「已展开完」。
type ExpandProgress struct {
	State         string `json:"state"`
	Egg           string `json:"egg"`
	Dst           string `json:"dst"`
	Gen           int64  `json:"gen"`
	UUID          string `json:"uuid"`
	SegmentsDone  int    `json:"segments_done"`
	SegmentsTotal int    `json:"segments_total"`
	BytesDone     int64  `json:"bytes_done"`
	BytesTotal    int64  `json:"bytes_total"`
	PID           int    `json:"pid"`
	StartedAt     string `json:"started_at"`
	UpdatedAt     string `json:"updated_at"`
}

// ReadExpandProgress 读一枚展开目录的进度（`.expanding` 的 JSON 正文）。
//
// 返回错误 = 这一目录此刻**不在展开中**（没有 `.expanding`）或那枚件读不懂（旧格式 / 空件）
// ⇒ 调用方必须把它当「没有进度可报」，✗ 不许编一个 0/0 的进度出来。
func ReadExpandProgress(dstDir string) (*ExpandProgress, error) {
	p := filepath.Join(dstDir, eggMarkerExpanding)
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, fmt.Errorf("展开进度件 %s 读不到（%v）——该目录此刻不在展开中", p, err)
	}
	var pr ExpandProgress
	if err := json.Unmarshal(b, &pr); err != nil {
		return nil, fmt.Errorf("展开进度件 %s 不是合法 JSON（%v）——不按它报进度", p, err)
	}
	return &pr, nil
}

// ExpandEggWithProgress 同 `ExpandEgg`，外加一枚**逐段进度回调**（nil = 不要进度）。
//
// 回调在每落成一段之后被调一次（同一 goroutine 内），入参是**累计**快照；同时 `.expanding`
// 里也写入同一份 JSON ⇒ 别进程（观测面 / CLI）读盘也能拿到同样的读数。
func ExpandEggWithProgress(eggPath, dstDir string, onProgress func(ExpandProgress)) (*ExpandResult, error) {
	if strings.TrimSpace(eggPath) == "" {
		return nil, fmt.Errorf("卵件路径为空 —— 拒（无蛋可展开）")
	}
	if strings.TrimSpace(dstDir) == "" {
		return nil, fmt.Errorf("展开落点为空 —— 拒（✗ 不往当前目录里撒）")
	}
	dst, err := filepath.Abs(expandTildePath(dstDir))
	if err != nil {
		return nil, fmt.Errorf("展开落点 %q 解不出绝对路径：%w", dstDir, err)
	}
	ix, err := ReadEggIndex(eggPath)
	if err != nil {
		return nil, err
	}
	res := &ExpandResult{
		Egg:          eggPath,
		Dst:          dst,
		Gen:          ix.Gen,
		UUID:         ix.UUID,
		TableSHA256:  ix.TableSHA,
		EggBytes:     ix.Bytes,
		Segments:     len(ix.Entries),
		Bytes:        ix.DataBytes,
		CompletePath: filepath.Join(dst, eggMarkerComplete),
	}

	// ---- ① 目标目录的状态判定（★ 全部在写第一个字节之前）----
	createdRoot, err := eggPrepareDir(dst)
	if err != nil {
		return nil, err
	}
	// ★ 这两档**拒绝但不动盘**（此刻一个字节都还没写）：
	//   ① 目录里已有 `.complete` 且属于**另一代** ⇒ 旧展开必须原样留着（调用方可能还在用它）；
	//   ② `.expanding` 在 ⇒ 另一趟正在展开（或上一趟崩了），✗ 不替别人清锁。
	if done, rerr := eggCheckComplete(dst, ix); rerr != nil {
		return nil, rerr
	} else if done != nil {
		res.Skipped = true
		res.EggSHA256 = done.EggSHA256
		return res, nil
	}
	// 单写者：`.expanding` 用 O_EXCL 建；已存在 ⇒ 另一趟在跑（✗ 两路同写 = 段级互相覆盖）。
	// ★ GAP-20261018-15：这枚件同时是**进度面**（正文 = ExpandProgress JSON）。建它的那一刻
	// 就写进「0 / 总字节」的起点 —— 外部从第一个字节起就能看到 `expanding`，而不是一片空白。
	expandMark := filepath.Join(dst, eggMarkerExpanding)
	prog := ExpandProgress{
		State:         eggExpandStateExpanding,
		Egg:           eggPath,
		Dst:           dst,
		Gen:           ix.Gen,
		UUID:          ix.UUID,
		SegmentsTotal: len(ix.Entries),
		BytesTotal:    ix.DataBytes,
		PID:           os.Getpid(),
		StartedAt:     time.Now().UTC().Format(time.RFC3339),
	}
	prog.UpdatedAt = prog.StartedAt
	mf, merr := os.OpenFile(expandMark, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if merr != nil {
		// 崩在半路的那枚标记里记着 pid：确认没有那个进程在跑之后删掉它再重试即可。
		if createdRoot {
			_ = os.Remove(dst) // 本次刚建的空根 ⇒ 连根收掉（✗ 不留一个空目录冒充「展开过」）
		}
		return nil, fmt.Errorf("展开标记 %s 建不出来（= 另一趟展开正在跑，或上一趟崩在半路没清锁）：%w —— 拒；"+
			"确认没有展开进程在跑之后，删掉该文件再重试（✗ 不替别人清锁：清了就回到两路同写）", expandMark, merr)
	}
	if body, jerr := json.Marshal(&prog); jerr == nil {
		_, _ = mf.Write(body)
	}
	mf.Close()

	// ★ 从这一刻起才是「本次写面」：任何失败都要回滚本次落的件（✗ 不留半截目录）。
	fail := func(e error) (*ExpandResult, error) {
		if rerr := eggRollback(dst, createdRoot, ix, e); rerr != nil {
			return nil, fmt.Errorf("%w（另外：回滚也不干净：%v —— 需人工清理 %s）", e, rerr, dst)
		}
		return nil, e
	}

	// ---- ② 逐段落盘（先普通文件、再符号链接：链的父目录那时都已就位）----
	// 每落成一段：先把累计进度写进 `.expanding`（别进程读盘可见），再回调（同进程观测面可见）。
	landed := func(seg EggSegment) {
		if !seg.IsSymlink() {
			prog.BytesDone += seg.Size
		}
		prog.SegmentsDone++
		prog.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		_ = eggWriteExpandProgress(expandMark, prog)
		if onProgress != nil {
			onProgress(prog)
		}
	}
	if err := eggLandSegments(eggPath, dst, ix, res, landed); err != nil {
		return fail(err)
	}

	// ---- ③ 整件 sha256（出处证据；一次顺序读）＋ `.complete` ----
	eggSHA, err := eggSha256File(eggPath)
	if err != nil {
		return fail(fmt.Errorf("整件 sha256 算不出来：%w", err))
	}
	res.EggSHA256 = eggSHA
	rc := eggReceipt{
		Version:     eggReceiptVersion,
		Egg:         eggPath,
		EggSHA256:   eggSHA,
		EggBytes:    ix.Bytes,
		Gen:         ix.Gen,
		UUID:        ix.UUID,
		TableSHA256: ix.TableSHA,
		Segments:    len(ix.Entries),
		Bytes:       ix.DataBytes,
		CompletedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if err := eggWriteJSONAtomic(res.CompletePath, rc); err != nil {
		return fail(fmt.Errorf("`.complete` 写不出来：%w", err))
	}
	// `.expanding` 收工（`.complete` 已在 ⇒ 它只是一个进度痕迹，删不掉也不影响完整性判定）。
	_ = os.Remove(expandMark)
	return res, nil
}

// eggPrepareDir —— 目标目录的状态判定：不存在 ⇒ 建（记「本次建的」）；空 ⇒ 用；只有标记件 ⇒ 清标记重来；
// 有内容 且 无 `.complete` ⇒ 拒；有 `.complete` ⇒ 交给 `eggCheckComplete` 判代际。
func eggPrepareDir(dst string) (created bool, err error) {
	st, serr := os.Lstat(dst)
	switch {
	case os.IsNotExist(serr):
		if merr := os.MkdirAll(dst, 0o755); merr != nil {
			return false, fmt.Errorf("展开落点 %s 建不出来：%w —— 拒", dst, merr)
		}
		return true, nil
	case serr != nil:
		return false, fmt.Errorf("展开落点 %s 读不到：%w —— 拒", dst, serr)
	case !st.IsDir():
		return false, fmt.Errorf("展开落点 %s 已存在且不是目录 —— 拒（✗ 覆盖盘上非目录件）", dst)
	}
	des, derr := os.ReadDir(dst)
	if derr != nil {
		return false, fmt.Errorf("展开落点 %s 列不出来：%w —— 拒", dst, derr)
	}
	var others []string
	for _, de := range des {
		switch de.Name() {
		case eggMarkerComplete, eggMarkerFailed, eggMarkerExpanding, eggMarkerExpandingTmp:
			continue
		}
		others = append(others, de.Name())
	}
	if len(others) == 0 {
		// 只有标记件：`.failed` 是「上一趟失败」的痕迹 ⇒ 清掉重来（可恢复）。
		// ★ `.expanding` **不动**：它是**单写者锁**（见下面 O_EXCL 那一段）—— 在这里替别人清锁，
		// 就等于把「两趟同时展开」的竞态又放回来了（P1 建锁 → P2 清锁再建自己的锁 → 两路同写）。
		if rerr := os.Remove(filepath.Join(dst, eggMarkerFailed)); rerr != nil && !os.IsNotExist(rerr) {
			return false, fmt.Errorf("清上一趟的失败标记 %s 失败：%w", eggMarkerFailed, rerr)
		}
		return false, nil
	}
	if _, cerr := os.Lstat(filepath.Join(dst, eggMarkerComplete)); cerr == nil {
		// 有 `.complete` 且有内容 —— 典型形态：已完整展开过。代际判据在 `eggCheckComplete`。
		return false, nil
	}
	return false, fmt.Errorf("展开落点 %s 已存在且**有内容**（%d 项，如 %q）—— 拒：✗ 静默覆盖别人的目录；"+
		"要重展开请换一个落点，或先人工确认后清空它", dst, len(others), others[0])
}

// eggCheckComplete —— 已有 `.complete` 时判代际：同一代 ⇒ 返回凭据（调用方跳过）；
// 另一代 / 凭据读不了 ⇒ 拒（✗ 拿旧展开冒充新蛋）。
func eggCheckComplete(dst string, ix *EggIndex) (*eggReceipt, error) {
	p := filepath.Join(dst, eggMarkerComplete)
	b, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("`.complete` 读不出来（%s）：%w —— 拒（凭据读不了就不敢说这目录已完整）", p, err)
	}
	var rc eggReceipt
	if err := json.Unmarshal(b, &rc); err != nil {
		return nil, fmt.Errorf("`.complete` 不是合法 JSON（%s）：%w —— 拒（凭据坏了就说不清是什么代）", p, err)
	}
	same := rc.Version == eggReceiptVersion && rc.Gen == ix.Gen && rc.UUID == ix.UUID &&
		rc.TableSHA256 == ix.TableSHA && rc.Segments == len(ix.Entries) && rc.EggBytes == ix.Bytes
	if !same {
		return nil, fmt.Errorf("展开落点 %s 里的 `.complete` 属于**另一代**（gen %d→%d · uuid %s→%s · 表 %s→%s · 段 %d→%d · 字节 %d→%d）—— 拒："+
			"✗ 拿旧展开冒充新蛋（升过引擎 / 换过蛋 ⇒ 换一个落点，或先 `egg release` 掉旧展开）",
			dst, rc.Gen, ix.Gen, shortSHA(rc.UUID), shortSHA(ix.UUID), shortSHA(rc.TableSHA256), shortSHA(ix.TableSHA),
			rc.Segments, len(ix.Entries), rc.EggBytes, ix.Bytes)
	}
	return &rc, nil
}

// eggLandSegments —— 逐段落盘并复核 sha256。普通文件在前（先建父目录），链段在最后。
// `landed` 在每落成一段之后被调一次（nil = 不报进度）；它只做观测，✗ 不改落盘语义。
func eggLandSegments(eggPath, dst string, ix *EggIndex, res *ExpandResult, landed func(EggSegment)) error {
	f, err := os.Open(eggPath)
	if err != nil {
		return fmt.Errorf("卵重新打开失败：%w", err)
	}
	defer f.Close()

	var files, links []EggSegment
	for _, e := range ix.Entries {
		if e.IsSymlink() {
			links = append(links, e)
			continue
		}
		files = append(files, e)
	}
	for _, e := range files {
		tgt, terr := eggJoin(dst, e.Path)
		if terr != nil {
			return terr
		}
		if derr := os.MkdirAll(filepath.Dir(tgt), 0o755); derr != nil {
			return fmt.Errorf("建父目录（段 %q）失败：%w", e.Path, derr)
		}
		mode, merr := eggModeOf(e.Path, e.Mode)
		if merr != nil {
			return merr
		}
		if _, lerr := eggLandOne(f, tgt, e, mode); lerr != nil {
			return lerr
		}
		res.Files++
		if landed != nil {
			landed(e)
		}
	}
	for _, e := range links {
		tgt, terr := eggJoin(dst, e.Path)
		if terr != nil {
			return terr
		}
		if derr := os.MkdirAll(filepath.Dir(tgt), 0o755); derr != nil {
			return fmt.Errorf("建父目录（链段 %q）失败：%w", e.Path, derr)
		}
		if li, lerr := os.Lstat(tgt); lerr == nil {
			if li.Mode()&os.ModeSymlink == 0 {
				return fmt.Errorf("链段 %q 的落点已存在且不是符号链接（%s）—— 拒（✗ 覆盖盘上普通件）", e.Path, tgt)
			}
			if rerr := os.Remove(tgt); rerr != nil {
				return fmt.Errorf("清旧链 %s 失败：%w", tgt, rerr)
			}
		}
		if serr := os.Symlink(e.LinkTarget, tgt); serr != nil {
			return fmt.Errorf("建符号链接 %s → %q 失败：%w", tgt, e.LinkTarget, serr)
		}
		res.Symlinks++
		if landed != nil {
			landed(e)
		}
	}
	return nil
}

// eggLandOne —— 落一个普通文件：临时名 → 写 → 逐字节复核 sha256 → `fsync` → `chmod` → `rename`。
// ★ 顺序是刻意的：**sha 不符就不 rename** ⇒ 盘上绝不出现一个「名字对、内容坏」的件。
func eggLandOne(f *os.File, tgt string, e EggSegment, mode os.FileMode) (string, error) {
	var rnd [4]byte
	if _, err := rand.Read(rnd[:]); err != nil {
		return "", fmt.Errorf("临时名随机源失败：%w", err)
	}
	tmp := filepath.Join(filepath.Dir(tgt),
		fmt.Sprintf(".%s.zerg-egg-part-%d-%s", filepath.Base(tgt), os.Getpid(), hex.EncodeToString(rnd[:])))
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return "", fmt.Errorf("段 %q 的临时件建不出来（%s）：%w", e.Path, tmp, err)
	}
	cleanup := func() {
		out.Close()
		os.Remove(tmp)
	}
	h := sha256.New()
	sr := io.NewSectionReader(f, e.Off, e.Size)
	n, cerr := io.Copy(io.MultiWriter(out, h), sr)
	if cerr != nil {
		cleanup()
		return "", fmt.Errorf("段 %q 拷贝失败（off=%d size=%d）：%w", e.Path, e.Off, e.Size, cerr)
	}
	if n != e.Size {
		cleanup()
		return "", fmt.Errorf("段 %q 只拷到 %d / %d 字节（卵被截断？）—— 拒", e.Path, n, e.Size)
	}
	got := hex.EncodeToString(h.Sum(nil))
	if got != e.SHA256 {
		cleanup()
		return "", fmt.Errorf("段 %q 落盘后 sha256 不符（段表 %s ⟷ 实得 %s）—— 拒：✗ 不落这枚坏段（也不 rename 它）",
			e.Path, shortSHA(e.SHA256), shortSHA(got))
	}
	if serr := out.Sync(); serr != nil { // fsync：rename 之前先把内容落到盘
		cleanup()
		return "", fmt.Errorf("段 %q fsync 失败：%w", e.Path, serr)
	}
	if cerr := out.Chmod(mode); cerr != nil { // mode 先落临时件（rename 后就没有「权限不对」的窗口）
		cleanup()
		return "", fmt.Errorf("段 %q chmod %#o 失败：%w", e.Path, mode, cerr)
	}
	if cerr := out.Close(); cerr != nil {
		os.Remove(tmp)
		return "", fmt.Errorf("段 %q 关闭失败：%w", e.Path, cerr)
	}
	if rerr := os.Rename(tmp, tgt); rerr != nil {
		os.Remove(tmp)
		return "", fmt.Errorf("段 %q rename 到 %s 失败：%w", e.Path, tgt, rerr)
	}
	return tgt, nil
}

// eggJoin —— 段 path → 宿主落点（路径规则的**第二道**：解出来的落点必须在根内）。
func eggJoin(root, rel string) (string, error) {
	if err := eggRefusePath(rel); err != nil {
		return "", err
	}
	clean := filepath.Clean(filepath.Join(root, filepath.FromSlash(rel)))
	rootClean := filepath.Clean(root)
	if clean != rootClean && !strings.HasPrefix(clean, rootClean+string(os.PathSeparator)) {
		return "", fmt.Errorf("拒：段 %q 的落点 %s 解出来跑到蛋根 %s 之外（第二个可穿点）", rel, clean, rootClean)
	}
	return clean, nil
}

// eggRollback —— 失败回滚：清掉本次落的全部件（本目录里除标记外的所有东西都属本次 ——
// 非空目录已在入口拒掉，这条前提由 `eggPrepareDir` 保证），再按「本次建的根则连根删」收口。
func eggRollback(dst string, createdRoot bool, ix *EggIndex, cause error) error {
	var firstErr error
	des, err := os.ReadDir(dst)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		firstErr = err
	}
	for _, de := range des {
		switch de.Name() {
		case eggMarkerComplete, eggMarkerFailed, eggMarkerExpanding, eggMarkerExpandingTmp:
			continue
		}
		if rerr := os.RemoveAll(filepath.Join(dst, de.Name())); rerr != nil && firstErr == nil {
			firstErr = rerr
		}
	}
	for _, n := range []string{eggMarkerExpandingTmp, eggMarkerExpanding, eggMarkerComplete} {
		if rerr := os.Remove(filepath.Join(dst, n)); rerr != nil && !os.IsNotExist(rerr) && firstErr == nil {
			firstErr = rerr
		}
	}
	if createdRoot {
		// 本次建的根：空 ⇒ 连根删掉（✗ 不留一个空目录冒充「展开过」）。
		if rerr := os.Remove(dst); rerr == nil || os.IsNotExist(rerr) {
			return firstErr
		}
	}
	fr := eggFailedReceipt{
		Version:     eggReceiptVersion,
		Egg:         ix.Path,
		EggBytes:    ix.Bytes,
		Gen:         ix.Gen,
		UUID:        ix.UUID,
		TableSHA256: ix.TableSHA,
		Segments:    len(ix.Entries),
		Reason:      cause.Error(),
		FailedAt:    time.Now().UTC().Format(time.RFC3339),
	}
	if werr := eggWriteJSONAtomic(filepath.Join(dst, eggMarkerFailed), fr); werr != nil && firstErr == nil {
		firstErr = werr
	}
	return firstErr
}

// eggWriteJSONAtomic —— 原子写一枚 JSON 标记（临时名 → fsync → rename）。
func eggWriteJSONAtomic(p string, v interface{}) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	tmp := p + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(b); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, p)
}

// eggWriteExpandProgress —— 原子写一份展开进度进 `.expanding`（临时名 → rename）。
//
// ★ 不做 fsync：这是**观测**，不是凭据 —— 进度丢一帧只影响「这一秒看到多少」，不影响完整性
// （完整性只看 `.complete` 配）。同理失败一律由调用方忽略（✗ 不许因为写不了进度就失败整趟展开）。
func eggWriteExpandProgress(p string, v ExpandProgress) error {
	b, err := json.Marshal(&v)
	if err != nil {
		return err
	}
	tmp := filepath.Join(filepath.Dir(p), eggMarkerExpandingTmp)
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

// eggSha256File —— 整件 sha256（十六进制）。★ 只在**真展开**那一趟算一次（幂等跳过档不算）。
func eggSha256File(p string) (string, error) {
	f, err := os.Open(p)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	buf := make([]byte, 1<<20)
	for {
		n, rerr := f.Read(buf)
		if n > 0 {
			h.Write(buf[:n])
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return "", rerr
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// expandTildePath —— 展开路径开头的 `~`（只展开首个 `~`；与仓内既有口径一致）。
func expandTildePath(p string) string {
	p = strings.TrimSpace(p)
	if p == "~" {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			return home
		}
		return p
	}
	if strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			return filepath.Join(home, p[2:])
		}
	}
	return p
}

// shortSHA —— 报话里用前 16 位（✗ 不把 64 位糊满一行；对账仍按整串）。
func shortSHA(s string) string {
	if len(s) <= 16 {
		return s
	}
	return s[:16]
}
