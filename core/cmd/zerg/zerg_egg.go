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
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"
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
	// MediaType —— ★ 卵1① 的**段类型声明**（OCI 三档闭集 · 可加面键）。★ 用 `omitempty`：
	// 空串**不落 JSON** ⇒ 旧形态（未标）产出的字节逐字不变（后向兼容）。
	MediaType string `json:"mediaType,omitempty"`
	// LinkTarget —— ★ 符号链接段的**原始链名**（相对形式原样保留 ✗ 不 normalize）。
	// 语义等价 tar 的 `TypeSymlink='2'`：有该键 ⇒ 该段是链、`size=0`、无载荷字节。
	// ★ `omitempty`：非链段不落键 ⇒ 旧形态的索引字节**逐字不变**（向后兼容 + 旧读端按可加面忽略）。
	LinkTarget string `json:"linkTarget,omitempty"`
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
	// ★ 卵1① 的**登记面**（读端填；✗ 不是拒执）：未标 `mediaType` 的段 / 闭集外的 `mediaType`。
	// 照 OCI「MUST NOT error on encountering a mediaType that is unknown」⇒ 两档都**放行**，
	// 但必须**登记并印出**（✗ 不许静默当已标）。
	UnlabeledMedia []string
	UnknownMedia   []string
}

// zegEntry —— 打包期的条目（多一个内部 `src`，**不进 JSON**）。
type zegEntry struct {
	Path      string
	Off       int64
	Size      int64
	SHA256    string
	Mode      int
	MediaType string // ★ 卵1① 写端算出的段类型（与读端同一张 `zegSegMediaType`）
	// LinkTarget —— ★ `GAP-20261018-05`：非空 ⇒ 该段**是符号链接**（`size=0`、无载荷字节，
	// 语义等价 tar 的 `TypeSymlink='2'`）。**原样保留相对链名**（✗ 不 normalize 成绝对）。
	LinkTarget string
	src        string
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

// ---- ★ 卵1①：段语义对齐 OCI `mediaType`（**纯声明** · §9.11② / §16.1 · §十九 #11）----------------
//
// ★ **纯声明** ✗ 别读成实现：本段只加「**段 → `mediaType` 字符串**」这一层**声明 ＋ 判读**，
// ✗ 不引 ORAS / registry、✗ 不实现拉取、✗ 不换后端（换 erofs / OCI 是另一批，见 §9.11⑤）。
//
// 语义照 OCI Image Spec 三档（manifest / config / layer）逐段对号；`mediaType` 属 `§11.2`
// 不变量 7 的**可加面（compat）** ⇒ 旧读端必须「**忽略且不报错**」（照 OCI 逐字
// 「MUST NOT error on encountering a `mediaType` that is unknown」）⇒ 读端遇**未标 / 闭集外**
// 一律 **放行 ＋ 登记**（✗ 不 error）；只有「**闭集内却与该段 `path` 推出的类不符**」才拒
// —— 那正是 `§11.2` 不变量 7.3「**禁同一键换语义**」。
const (
	// zegMediaManifest —— OCI `manifest` 档（索引与签名段：段表 / 逐段 sha256 / 整体签名 / 版本）。
	zegMediaManifest = "application/vnd.oci.image.manifest.v1+json"
	// zegMediaConfig —— OCI `config` 档（契约段 / 配方段：封闭策略 / 绑定 / 参数 / 标定档案）。
	zegMediaConfig = "application/vnd.oci.image.config.v1+json"
	// zegMediaLayer —— OCI `layer` 档（引擎段 / 权重段：引擎二进制与依赖库 / 模型权重）。
	zegMediaLayer = "application/vnd.oci.image.layer.v1.tar"
)

// zegSegMediaType —— `§9.1` **五段** → OCI 三档的**逐类映射表**（★ 本文件**唯一真源**：
// **写端算 `mediaType`**（`zegMediaTypeOfPath`）与 **读端判读**（`zegMediaTypeClosedSet`）都读这一张
// ⇒ 「**双向校验**」＝ 两侧同表逐条对拍）。
var zegSegMediaType = []struct {
	Seg   string
	Class string
	Media string
	Why   string
}{
	{Seg: "索引与签名段", Class: "manifest", Media: zegMediaManifest,
		Why: "段表（偏移/长度/页对齐）、逐段 sha256、整体签名、版本 —— 即 OCI 的 manifest 档"},
	{Seg: "契约段", Class: "config", Media: zegMediaConfig,
		Why: "封闭策略、绑定映射、环境/限额、引擎入口 —— 配置语义 ⇒ OCI config 档"},
	{Seg: "配方段", Class: "config", Media: zegMediaConfig,
		Why: "适配器、运行配方、参数、标定档案 —— 配置语义 ⇒ OCI config 档"},
	{Seg: "引擎段", Class: "layer", Media: zegMediaLayer,
		Why: "引擎二进制 ＋ 依赖库 —— 内容层 ⇒ OCI layer 档"},
	{Seg: "权重段", Class: "layer", Media: zegMediaLayer,
		Why: "模型 GGUF / safetensors —— 内容层 ⇒ OCI layer 档"},
}

// zegMediaTypeClosedSet —— `mediaType` **闭集**（取值 → OCI 类名）。★ 闭集外**不算已知类**：
// 照 OCI 放行 ＋ 登记（见本段头注），✗ 不 error。
func zegMediaTypeClosedSet() map[string]string {
	m := make(map[string]string, len(zegSegMediaType))
	for _, r := range zegSegMediaType {
		m[r.Media] = r.Class
	}
	return m
}

// zegSegOfPath —— 段路径 → `§9.1` 五段之一（**写端与读端同一套**；✗ 不随平台/时间取值）。
// 序敏感：先**契约面**（入口名 / contract 目录）→ 再**引擎面**（扩展名 / 引擎名）→ 再**权直面**
// （张量扩展名 / model 目录）→ 其余**一律配方段**（配置类语义最保守的归属 ⇒ 未知件不会误判成层）。
func zegSegOfPath(rel string) string {
	base := strings.ToLower(filepath.Base(rel))
	lower := strings.ToLower(filepath.ToSlash(rel))
	ext := strings.ToLower(filepath.Ext(base))
	if strings.HasPrefix(lower, "contract/") || strings.Contains(lower, "/contract/") {
		return "契约段"
	}
	switch base {
	case "policy.json", "contract.json", "bindings.json", "limits.json", "engine.json", "enclosure.json":
		return "契约段"
	}
	switch ext {
	case ".so", ".dylib", ".dll":
		return "引擎段"
	}
	if strings.HasPrefix(lower, "engine/") || strings.Contains(lower, "/engine/") ||
		base == "zerg-engine" || base == "llama-server" || base == "ds4-server" {
		return "引擎段"
	}
	switch ext {
	case ".gguf", ".safetensors", ".onnx", ".pt", ".pth":
		return "权重段"
	}
	if strings.HasPrefix(lower, "model/") || strings.Contains(lower, "/model/") ||
		strings.HasPrefix(lower, "weights/") || strings.Contains(lower, "/weights/") {
		return "权重段"
	}
	return "配方段"
}

// zegMediaTypeOfPath —— 段路径 → `mediaType`（**写端写入用**；读端判读走 `zegMediaTypeClosedSet`）。
func zegMediaTypeOfPath(rel string) string {
	seg := zegSegOfPath(rel)
	for _, r := range zegSegMediaType {
		if r.Seg == seg {
			return r.Media
		}
	}
	return zegMediaConfig // 不可达（表里恒含五段）；兜底取配置类（✗ 不落空串）
}

// ---- ★ 卵1②：路径拒绝口径（**规则表 ＋ 检查器** · `§11.2` 新增第 8 条 · §15.8 L25/L26 · §十九 #26）----
//
// ★ 本段 = 拍定口径的**机检物**：一张**逐条具名**的规则表 ＋ 一个检查器，**打包端 / 读端 / 重建端
// 三处同表**（＝「**双向校验**」）；★ 能真跑：人造恶意路径 ⇒ **逐类必拒且点名规则**
// （见 `zegFormatSelfTest` 与 `TestZergEggMediaPathFourStates`）。规则口径逐字照 `§十九` #26 的拍定：
// **NFC 相对路径 · 拒特殊文件〔硬链 / 设备 / FIFO / socket〕· mode 安全子集 · 双向校验 ·
// `O_NOFOLLOW|O_EXCL`**。
type zegPathRule struct {
	Name string // 规则名（**拒执报话点名它** ⇒ 机器可判「哪一类」）
	Why  string
	Bad  func(rel string) bool
}

// zegWindowsReserved —— Windows 保留名（照 §十九 #26 逐字「Windows 保留名」）。
var zegWindowsReserved = map[string]bool{
	"con": true, "prn": true, "aux": true, "nul": true,
	"com1": true, "com2": true, "com3": true, "com4": true, "com5": true,
	"com6": true, "com7": true, "com8": true, "com9": true,
	"lpt1": true, "lpt2": true, "lpt3": true, "lpt4": true, "lpt5": true,
	"lpt6": true, "lpt7": true, "lpt8": true, "lpt9": true,
}

// zegHasCombiningMark —— 是否含**组合记号**（Unicode `Mn` 的主用区段）。★ 本仓硬线「✗ 不引入新依赖」
// ⇒ **不引** `golang.org/x/text/unicode/norm`，改用**fail-closed 近似**：规范化（NFC）后的路径**不该**
// 含这些区段的组合记号（CVE-2026-59890 的 NFC/NFD 归一化碰撞正是靠 NFD 形态绕过）。
func zegHasCombiningMark(s string) bool {
	for _, r := range s {
		switch {
		case r >= 0x0300 && r <= 0x036F, // Combining Diacritical Marks
			r >= 0x1AB0 && r <= 0x1AFF, // Combining Diacritical Marks Extended
			r >= 0x1DC0 && r <= 0x1DFF, // Combining Diacritical Marks Supplement
			r >= 0x20D0 && r <= 0x20FF, // Combining Diacritical Marks for Symbols
			r >= 0xFE20 && r <= 0xFE2F: // Combining Half Marks
			return true
		}
	}
	return false
}

// zegSegsOf —— `path` 按 `/` 切段（规则表内部用；✗ 不走 `path` 包 —— 那是本机分隔符）。
func zegSegsOf(rel string) []string { return strings.Split(rel, "/") }

// zegAnySeg —— 任一段满足 `f` 即真。
func zegAnySeg(rel string, f func(s string) bool) bool {
	for _, s := range zegSegsOf(rel) {
		if f(s) {
			return true
		}
	}
	return false
}

// zegPathRules —— **索引里 `path` 必须过**的规则表（逐条独立 ⇒ 命中即点名；✗ 不合成一个大判断）。
var zegPathRules = []zegPathRule{
	{Name: "path_empty", Why: "空路径", Bad: func(rel string) bool { return rel == "" }},
	{Name: "path_absolute", Why: "绝对路径（禁）", Bad: func(rel string) bool { return strings.HasPrefix(rel, "/") }},
	{Name: "path_windows_drive", Why: "盘符（`C:` 一类）",
		Bad: func(rel string) bool {
			return len(rel) > 1 && rel[1] == ':' &&
				((rel[0] >= 'a' && rel[0] <= 'z') || (rel[0] >= 'A' && rel[0] <= 'Z'))
		}},
	{Name: "path_unc", Why: "UNC 形（`//` 或 `\\`）",
		Bad: func(rel string) bool { return strings.HasPrefix(rel, "//") || strings.HasPrefix(rel, "\\") }},
	{Name: "path_backslash", Why: "反斜杠（Windows 分隔符混入）",
		Bad: func(rel string) bool { return strings.Contains(rel, "\\") }},
	{Name: "path_nul", Why: "含 NUL 字节", Bad: func(rel string) bool { return strings.ContainsRune(rel, 0) }},
	{Name: "path_dotdot", Why: "段里出现 `..`",
		Bad: func(rel string) bool { return zegAnySeg(rel, func(s string) bool { return s == ".." }) }},
	{Name: "path_not_normalized", Why: "非规范化形（`.` 段 / 空段 / 尾斜杠 —— `Clean(rel) != rel`）",
		Bad: func(rel string) bool { return filepath.Clean(rel) != rel }},
	{Name: "path_trailing_dot_space", Why: "段以点或空格结尾（Windows 会把它们吃掉 ⇒ 归一化碰撞）",
		Bad: func(rel string) bool {
			return zegAnySeg(rel, func(s string) bool {
				return s != "" && (strings.HasSuffix(s, ".") || strings.HasSuffix(s, " "))
			})
		}},
	{Name: "path_windows_reserved", Why: "Windows 保留名（CON / PRN / AUX / NUL / COM1-9 / LPT1-9）",
		Bad: func(rel string) bool {
			return zegAnySeg(rel, func(s string) bool {
				head := s
				if i := strings.IndexByte(head, '.'); i >= 0 {
					head = head[:i]
				}
				return zegWindowsReserved[strings.ToLower(head)]
			})
		}},
	{Name: "path_not_utf8", Why: "不是合法 UTF-8", Bad: func(rel string) bool { return !utf8.ValidString(rel) }},
	{Name: "path_not_nfc", Why: "非 NFC 形态（含组合记号 —— NFC/NFD 归一化碰撞的病灶）",
		Bad: func(rel string) bool { return zegHasCombiningMark(rel) }},
}

// zegModeSafeSet —— `mode` **安全子集**（`§11.2` 新增第 8 条：只存安全子集 · 禁 setuid/setgid/sticky）。
var zegModeSafeSet = map[int]bool{0o644: true, 0o755: true, 0o700: true}

// zegRuleRefuse —— **具名拒**（Tier ＋ 规则名 ＋ 依据 ＋ 主体）。★ 报话格式**唯一**：
// `拒：<主体> —— 命中规则 <规则名>（<依据>）` ⇒ 机器面（测试 / 自检 / 门）能逐类点名。
func zegRuleRefuse(tier, rule, why, subject string) *zegRefusal {
	return &zegRefusal{Tier: tier, Msg: fmt.Sprintf("拒：%s —— 命中规则 %s（%s）", subject, rule, why)}
}

// zegRefusePath —— 按 `zegPathRules` 逐条判一条索引里的 `path`；命中 ⇒ **具名拒**（D9 · 点名规则）。
// 返回 nil ⇒ 该段 `path` 合法。★ **打包端 / 读端 / 重建端三处都调它**（＝双向校验）。
func zegRefusePath(rel string) error {
	for _, r := range zegPathRules {
		if r.Bad(rel) {
			return zegRuleRefuse("D9", r.Name, r.Why, fmt.Sprintf("段 path %q（须为规范化 NFC 相对路径）", rel))
		}
	}
	return nil
}

// zegFsRule —— **文件系统级**规则（只看路径串看不出来 ⇒ 必须 `lstat`）。
type zegFsRule struct {
	Name string
	Why  string
}

// zegFsRules —— 特殊文件 / 硬链 / mode 安全子集（逐字照 `§十九` #26：「拒特殊文件〔硬链 / 设备 /
// FIFO / socket〕· mode 安全子集」）。
var zegFsRules = []zegFsRule{
	// ★ `GAP-20261018-05`（设计稿 Q4 拍定）：**只许相对链** —— 四条具名拒 + 一条「段自身不合法」，
	// 取代 v1 那条「符号链接一律拒」。TOCTOU 的防线改成「绝对路径 / 越界 / 成环 / 超深」四道；
	// 链本身照 `tar` 的 `TypeSymlink='2'` 语义进段表（`size=0`、无载荷字节），重建端用 `os.Symlink` 还原。
	{Name: "fs_symlink_absolute", Why: "符号链接目标是绝对路径（只许相对链：绝对链把蛋内落点绑死在宿主路径上）"},
	{Name: "fs_symlink_escape", Why: "相对链解析后越出蛋根（含 `..` 穿出：链成了蛋外的第二入口）"},
	{Name: "fs_symlink_cycle", Why: "符号链接成环（a→b→a：解析不终止 ＝ 自指式陷阱）"},
	{Name: "fs_symlink_depth", Why: "符号链接层数超过 8（深链放大解析成本、且是成环的近似形态）"},
	{Name: "fs_symlink_entry_invalid", Why: "符号链接段自身不合法（链名为空 / size≠0 / 链名超过 4096 字节上限）"},
	{Name: "fs_device", Why: "设备文件（块设备 / 字符设备）"},
	{Name: "fs_fifo", Why: "FIFO（命名管道）"},
	{Name: "fs_socket", Why: "socket"},
	{Name: "fs_hardlink", Why: "硬链接（nlink > 1：同 inode 多路入口 ⇒ 越界写的载体）"},
	{Name: "fs_mode_unsafe", Why: "mode 不在安全子集（只许 0644 / 0755 / 0700，禁 setuid / setgid / sticky）"},
}

// zegFsWhy —— 取规则依据（✗ 不静默回落空串：取不到就逐字点名规则名本身）。
func zegFsWhy(rule string) string {
	for _, r := range zegFsRules {
		if r.Name == rule {
			return r.Why
		}
	}
	return rule
}

// ---- ★ 符号链接：只许相对链（`GAP-20261018-05` · 设计稿 Q4 拍板）--------------------------------
//
// 口径：链目标**必为相对路径**、解析后**必在蛋根内**、**必不成环**、**层数 ≤ 8**。
// ★ 路径安全只做**算术**（`filepath.Clean` ＋ `filepath.Rel` 判是否以 `..` 开头），
// ✗ 不用 `filepath.EvalSymlinks` —— 那会真去读盘 ＝ TOCTOU 的入口。
// ★ 四道判在这里**逐条具名**（打包端 / 读端 `zegAuditIndexEntries` / 重建端三处共用一张表）。
const (
	// zegSymlinkMaxDepth —— 链层数上限（含自身；> 8 即拒 · 防环的第二道保险）。
	zegSymlinkMaxDepth = 8
	// zegSymlinkMaxTargetLen —— 链名字节上限（POSIX `PATH_MAX` 同级）。
	zegSymlinkMaxTargetLen = 4096
	// zegSymlinkMode —— 段表里给链段记的 mode（链自身的权限位无意义 ⇒ 取安全子集里的 0755）。
	zegSymlinkMode = 0o755
)

// zegEmptySHA256 —— 空载荷的 sha256（链段 `size=0` ⇒ 校验面就是它 · ✗ 别把链当空文件读）。
const zegEmptySHA256 = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

// zegSymlinkResolve —— 把一条链的**原始相对链名**按「所在目录」解成根相对 slash 路径。
// 返回 (解出的根相对路径, 是否仍在蛋根内)。★ 纯算术：不读盘、不跟随。
func zegSymlinkResolve(rel, target string) (string, bool) {
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

// zegSymlinkOne —— 单条链的**前两道**判：段自身合法（非空 / 不太长）· 目标非绝对 · 解析后不出根。
func zegSymlinkOne(rel, target string) error {
	subj := fmt.Sprintf("段 %q 的符号链接（→ %q）", rel, target)
	if target == "" || len(target) > zegSymlinkMaxTargetLen || strings.ContainsRune(target, 0) {
		return zegRuleRefuse("D9", "fs_symlink_entry_invalid", zegFsWhy("fs_symlink_entry_invalid"), subj)
	}
	if strings.HasPrefix(target, "/") || filepath.IsAbs(target) {
		return zegRuleRefuse("D9", "fs_symlink_absolute", zegFsWhy("fs_symlink_absolute"), subj)
	}
	next, inside := zegSymlinkResolve(rel, target)
	if !inside {
		return zegRuleRefuse("D9", "fs_symlink_escape", zegFsWhy("fs_symlink_escape"),
			fmt.Sprintf("段 %q 的符号链接（→ %q ⇒ 解析到 %q，越出蛋根）", rel, target, next))
	}
	return nil
}

// zegRefuseSymlinkSet —— 一棵树 / 一张段表里**全部**符号链接的联判：先逐条过「绝对 / 越界」，
// 再按链图走一遍「成环 / 超深」。★ 打包端 · 读端 · 重建端三处共用（＝双向校验）。
func zegRefuseSymlinkSet(links map[string]string) error {
	if len(links) == 0 {
		return nil
	}
	rels := make([]string, 0, len(links))
	for rel := range links {
		rels = append(rels, rel)
	}
	sort.Strings(rels)
	for _, rel := range rels {
		if err := zegSymlinkOne(rel, links[rel]); err != nil {
			return err
		}
	}
	for _, rel := range rels {
		seen := map[string]bool{rel: true}
		cur := rel
		depth := 1
		for {
			next, _ := zegSymlinkResolve(cur, links[cur])
			if _, isLink := links[next]; !isLink {
				break
			}
			if seen[next] {
				return zegRuleRefuse("D9", "fs_symlink_cycle", zegFsWhy("fs_symlink_cycle"),
					fmt.Sprintf("段 %q 的符号链接（链走到 %q 又回到已访过的段）", rel, next))
			}
			seen[next] = true
			depth++
			if depth > zegSymlinkMaxDepth {
				return zegRuleRefuse("D9", "fs_symlink_depth", zegFsWhy("fs_symlink_depth"),
					fmt.Sprintf("段 %q 的符号链接（链层数 %d > %d）", rel, depth, zegSymlinkMaxDepth))
			}
			cur = next
		}
	}
	return nil
}

// zegRefuseEntryMode —— 逐条 `zegFsRules` 判一个 `os.FileInfo`（**打包端与重建端共用**；lstat 来的
// 那个 ⇒ ✗ 不跟随符号链接）。命中 ⇒ 具名拒（D9）。
//
// ★ 符号链接**不在这里判**（`GAP-20261018-05`）：只许相对链的四道判在 `zegRefuseSymlinkSet`
// （打包端 / 读端 / 重建端三处都调它）。走到这里说明调用方漏了分流 ⇒ **fail-closed** 按「段自身不合法」拒。
func zegRefuseEntryMode(li os.FileInfo, subject string) error {
	m := li.Mode()
	switch {
	case m&os.ModeSymlink != 0:
		return zegRuleRefuse("D9", "fs_symlink_entry_invalid", zegFsWhy("fs_symlink_entry_invalid"),
			fmt.Sprintf("%s（符号链接须先过 zegRefuseSymlinkSet 分流）", subject))
	case m&os.ModeDevice != 0:
		return zegRuleRefuse("D9", "fs_device", zegFsWhy("fs_device"), subject)
	case m&os.ModeNamedPipe != 0:
		return zegRuleRefuse("D9", "fs_fifo", zegFsWhy("fs_fifo"), subject)
	case m&os.ModeSocket != 0:
		return zegRuleRefuse("D9", "fs_socket", zegFsWhy("fs_socket"), subject)
	case !m.IsRegular():
		// 其余非常规件一律按设备档拒（fail-closed：✗ 不给未知形态留口子）。
		return zegRuleRefuse("D9", "fs_device", zegFsWhy("fs_device"), subject)
	}
	if st, ok := li.Sys().(*syscall.Stat_t); ok && st.Nlink > 1 {
		return zegRuleRefuse("D9", "fs_hardlink", zegFsWhy("fs_hardlink"), subject)
	}
	if m&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky) != 0 || !zegModeSafeSet[int(m.Perm())] {
		return zegRuleRefuse("D9", "fs_mode_unsafe", zegFsWhy("fs_mode_unsafe"), subject)
	}
	return nil
}

// zegRefuseSourceEntry —— 打包端对**源件**的检查（`lstat` ⇒ 逐条具名拒）。
// ★ 符号链接已由调用方分流（`zegRefuseSymlinkSet`）；这里只判**非链**源件。
func zegRefuseSourceEntry(fp string) error {
	li, err := os.Lstat(fp)
	if err != nil {
		return err
	}
	return zegRefuseEntryMode(li, fp)
}

// zegRebuildTarget —— 重建端落点的**唯一取法**：先过路径规则表，再断言「解出来的落点仍在根内」
// （**第二个可穿点** —— ✗ 不许只靠打包端；`§十九` #26 的「双向校验」）。
func zegRebuildTarget(root, rel string) (string, error) {
	if err := zegRefusePath(rel); err != nil {
		return "", err
	}
	clean := filepath.Clean(filepath.Join(root, filepath.FromSlash(rel)))
	rootClean := filepath.Clean(root)
	if clean != rootClean && !strings.HasPrefix(clean, rootClean+string(os.PathSeparator)) {
		return "", zegRuleRefuse("D9", "rebuild_escapes_root", "重建落点解出来跑到根外（第二个可穿点）",
			fmt.Sprintf("段 %q ⇒ %s", rel, clean))
	}
	return clean, nil
}

// zegOpenRebuildTarget —— 重建端落件的**唯一开件点**：`O_NOFOLLOW|O_EXCL`（`§11.2` 新增第 8 条）。
// 已存在 ⇒ 先 `lstat` 断言「普通件 ＋ 非符号链接 ＋ 非特殊件 ＋ 非硬链」（✗ **不跟随符号链接**），
// 再以 `O_NOFOLLOW|O_TRUNC` 重开；不存在 ⇒ `O_NOFOLLOW|O_EXCL` **原子建**（✗ 静默覆盖盘上非普通件）。
func zegOpenRebuildTarget(tgt string) (*os.File, error) {
	if li, err := os.Lstat(tgt); err == nil {
		// ★ 符号链接**在这里必须拒**（`O_NOFOLLOW` 语义）：重建端 ✗ 顺着盘上已有的链写出去。
		// 链段的重建走 `zegCreateSymlinkTarget`（先按段表判，再 `os.Symlink`）；走到这里的链 ⇒ 名字冲突。
		if li.Mode()&os.ModeSymlink != 0 {
			return nil, zegRuleRefuse("D9", "rebuild_target_symlink",
				"重建落点已存在且是符号链接（O_NOFOLLOW：✗ 不跟随、✗ 覆盖）", tgt)
		}
		if rerr := zegRefuseEntryMode(li, tgt); rerr != nil {
			return nil, rerr
		}
		return os.OpenFile(tgt, os.O_RDWR|os.O_TRUNC|syscall.O_NOFOLLOW, 0o644)
	}
	return os.OpenFile(tgt, os.O_RDWR|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0o644)
}

// zegAuditIndexEntries —— 读端的**逐段审计**（`§11.2` 新增第 8 条 ＋ 不变量 7）：
//
//	① `path` 逐条过 `zegPathRules`（命中 ⇒ 具名拒 D9）；
//	② `mode` 必须在**安全子集**内（禁 setuid / setgid / sticky）；
//	③ `mediaType` —— 未标 / 闭集外 ⇒ **放行 ＋ 登记**（照 OCI「MUST NOT error」）；
//	   闭集内却与该段 `path` 推出的类不符 ⇒ 拒（`§11.2` 不变量 7.3「**禁同一键换语义**」）。
//
// ★ 登记面写回 `idx`（`UnlabeledMedia` / `UnknownMedia`）⇒ 命令面（`info` / `verify`）逐字印出
// （✗ 不静默当已标 —— `§十九` #5 的「登记」在卵族命令面的落点）。
func zegAuditIndexEntries(idx *zegIndex) error {
	closed := zegMediaTypeClosedSet()
	// ★ `GAP-20261018-05`：段表里的链**整表联判**一次（绝对 / 越界 / 成环 / 超深）——
	// 读端守与打包端**同一口径**（「双向校验」）。链名原样取自段表，✗ 不去盘上跟。
	links := make(map[string]string, len(idx.Entries))
	for _, e := range idx.Entries {
		if e.LinkTarget != "" {
			links[e.Path] = e.LinkTarget
		}
	}
	if err := zegRefuseSymlinkSet(links); err != nil {
		return err
	}
	for _, e := range idx.Entries {
		if err := zegRefusePath(e.Path); err != nil {
			return err
		}
		if e.LinkTarget != "" {
			// ★ 链段：权限位 / setuid 对链本身不适用 ⇒ **跳过 mode 检查**；但段长与链名仍要校。
			if e.Size != 0 {
				return zegRuleRefuse("D9", "fs_symlink_entry_invalid", zegFsWhy("fs_symlink_entry_invalid"),
					fmt.Sprintf("段 %q 是符号链接但 size=%d（链段必须 size=0）", e.Path, e.Size))
			}
		} else if !zegModeSafeSet[e.Mode] {
			return zegRuleRefuse("D9", "fs_mode_unsafe", zegFsWhy("fs_mode_unsafe"),
				fmt.Sprintf("段 %q 的 mode=%#o", e.Path, e.Mode))
		}
		mt := strings.TrimSpace(e.MediaType)
		if mt == "" {
			idx.UnlabeledMedia = append(idx.UnlabeledMedia, e.Path)
			continue
		}
		class, known := closed[mt]
		if !known {
			idx.UnknownMedia = append(idx.UnknownMedia, e.Path+"="+mt)
			continue
		}
		want, ok := closed[zegMediaTypeOfPath(e.Path)]
		if !ok || want != class {
			return zegRuleRefuse(zegTierSurface, "mediaType_class_mismatch",
				"同一个 `mediaType` 键换了语义：取值声明的 OCI 类与该段 path 推出的类不符（§11.2 不变量 7.3）",
				fmt.Sprintf("段 %q 的 mediaType=%q（声明类 %s，path 推出类 %s）", e.Path, mt, class, want))
		}
	}
	return nil
}

// ---- 收集条目 -------------------------------------------------------------------------------

// zegCollectSymlinks —— 先整棵树收一遍**符号链接**（`Lstat` + `Readlink`，✗ 不跟随、不下钻）：
// 返回「根相对 slash 路径 → 原始链名」。★ 链名**原样保留**（✗ normalize 成绝对、✗ 改写 `..`）。
// 用途：`zegRefuseSymlinkSet` 需要**整棵树的链图**才能判「成环 / 超深」（单条链看不出这两类）。
func zegCollectSymlinks(src string) (map[string]string, error) {
	links := map[string]string{}
	var walk func(dir string) error
	walk = func(dir string) error {
		des, err := os.ReadDir(dir)
		if err != nil {
			return err
		}
		for _, de := range des {
			full := filepath.Join(dir, de.Name())
			li, lerr := os.Lstat(full)
			if lerr != nil {
				return lerr
			}
			if li.Mode()&os.ModeSymlink != 0 {
				rel, rerr := filepath.Rel(src, full)
				if rerr != nil {
					return rerr
				}
				rel = filepath.ToSlash(rel)
				if err := zegRefusePath(rel); err != nil {
					return err
				}
				tgt, terr := os.Readlink(full)
				if terr != nil {
					return terr
				}
				links[rel] = tgt
				continue // followlinks=False：指向目录的链**不下钻**
			}
			if li.IsDir() {
				if werr := walk(full); werr != nil {
					return werr
				}
			}
		}
		return nil
	}
	if err := walk(src); err != nil {
		return nil, err
	}
	return links, nil
}

// zegCollectEntries —— 遍历源目录出条目（**顺序确定**：目录与文件都排序 · ✗ 与 py 逐字同构）。
// py 的 `os.walk`：先把当前层 scandir 分成 dirs/files、**先出当前层全部文件**再按 dirs 序下钻；
// 分类用 `entry.is_dir()`（**跟随符号链接**）；`followlinks=False` ⇒ 指向目录的符号链接**只入 dirs、
// 不下钻**；符号链接的**文件**进 files ⇒ 触发 ValueError。本件逐条复刻。
//
// ★ `GAP-20261018-05` 起的偏差（**有意的**）：符号链接不再一律拒 —— 先整树联判「只许相对链」
// （绝对 / 越界 / 成环 / 超深），过了就以 `LinkTarget` 记成**链段**（`size=0`）；且**分类不再跟随**
// （链一律进 files 档、✗ 不下钻）⇒ 指向目录的相对链也进段表，✗ 不会被静默丢掉。
func zegCollectEntries(src string) ([]*zegEntry, error) {
	links, lerr := zegCollectSymlinks(src)
	if lerr != nil {
		return nil, lerr
	}
	if err := zegRefuseSymlinkSet(links); err != nil {
		return nil, err
	}
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
			// 分类：符号链接**不跟随**（Lstat 先判）⇒ 一律当「非目录」（入 files 档、不下钻）；
			// 非链才用 os.Stat 判目录（dangling / 报错按「非目录」处理，与 py `is_dir` 抛错的档对齐）。
			isDir := false
			if li, lerr := os.Lstat(full); lerr == nil && li.Mode()&os.ModeSymlink != 0 {
				isDir = false
			} else if st, serr := os.Stat(full); serr == nil && st.IsDir() {
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
			rel, rerr := filepath.Rel(src, fp)
			if rerr != nil {
				return rerr
			}
			rel = filepath.ToSlash(rel)
			// ★ 卵1② 打包端**第一道**：路径串规则表（与读端 `zegParseIndex`、重建端 `zegRebuildTarget`
			// **同表** ⇒ 「双向校验」）。
			if err := zegRefusePath(rel); err != nil {
				return err
			}
			// ★ 卵1② 打包端**第二道**：`lstat`（**不跟随**）⇒ 特殊文件 / 硬链 / mode 安全子集，逐条具名拒。
			// ★ `GAP-20261018-05`：符号链接先分流（只许相对链的四道判已在 `zegRefuseSymlinkSet` 全树过完）
			// ⇒ 记成**链段**（`size=0` · 链名原样 · `tar TypeSymlink='2'` 语义），✗ 不读成空文件。
			li, lerr := os.Lstat(fp)
			if lerr != nil {
				return lerr
			}
			if li.Mode()&os.ModeSymlink != 0 {
				entries = append(entries, &zegEntry{
					Path:       rel,
					Size:       0,
					SHA256:     zegEmptySHA256,
					Mode:       zegSymlinkMode,
					MediaType:  zegMediaTypeOfPath(rel), // ★ 卵1① 写端：与读端**同一张**表
					LinkTarget: links[rel],
				})
				continue
			}
			if err := zegRefuseSourceEntry(fp); err != nil {
				return err
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
				Path:      rel,
				Size:      st.Size(),
				SHA256:    sha,
				Mode:      int(st.Mode().Perm()),
				MediaType: zegMediaTypeOfPath(rel), // ★ 卵1① 写端：与读端**同一张**表
				src:       fp,
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
		ej = append(ej, zegEntryJSON{Path: e.Path, Off: e.Off, Size: e.Size, SHA256: e.SHA256, Mode: e.Mode, MediaType: e.MediaType, LinkTarget: e.LinkTarget})
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
		// ★ 链段没有载荷字节（`tar TypeSymlink='2'` 语义：`size=0`）⇒ 只占一个对齐位，不拷内容。
		if e.LinkTarget != "" {
			continue
		}
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
	// ★ 卵6：**主号闸**（前向拒 D2 —— 含「签名块 v2 ＝ 主号 2 ⇒ 旧读端必拒」；过旧拒 D3）。
	if err := zegCompatGateMainVersion(ver, zegVersionSitePrefix); err != nil {
		return nil, err
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
	// ★ 卵6：**索引体兼容闸**（不可变面/可加面二分类 ＋ `min_readable` 牙 ＋「禁同键换语义」）——
	// 必须跑在 `json.Unmarshal` **之前**：同一键换类（如 `align` 由 number 变成 string）会让结构化解码
	// 直接吐一句无名的类型错 ⇒ 档位点不出来、也点不出「该升主号」（见 `zerg_egg_version.go`）。
	if _, err := zegCompatGateIndexBody(body); err != nil {
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
	idx := &zegIndex{
		Magic:     raw.Magic,
		Version:   raw.Version,
		Align:     raw.Align,
		Tool:      raw.Tool,
		CreatedAt: raw.CreatedAt,
		Entries:   raw.Entries,
	}
	// ★ 卵1② ＋ 卵1①：读端**逐段审计**（path / mode / mediaType 三道）—— 这是「双向校验」的另一半；
	// 跑在结构化解码之后、返回之前（拒 ⇒ 具名 `*zegRefusal`，点名档位与规则）。
	if err := zegAuditIndexEntries(idx); err != nil {
		return nil, err
	}
	return idx, nil
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
			// ★ 卵1② 重建端：落点走 `zegRebuildTarget`（路径规则表 ＋ 根内断言）＋
			// `zegOpenRebuildTarget`（`O_NOFOLLOW|O_EXCL`）⇒ **第二个可穿点也加固**。
			tgt, terr := zegRebuildTarget(rebuildInto, e.Path)
			if terr != nil {
				return nil, nil, terr
			}
			if err := os.MkdirAll(filepath.Dir(tgt), 0o755); err != nil {
				return nil, nil, err
			}
			// ★ `GAP-20261018-05`：链段 ⇒ **`os.Symlink(linkname, 目标路径)`**，✗ 不写普通文件。
			// 链名**原样**落下（相对形式 ✗ 不改写）；链本身**不 chmod**（权限位对链无意义）。
			// 重建端**再判一次**（双向校验 · ✗ 只信打包端 / 读端）。
			if e.LinkTarget != "" {
				if rerr := zegSymlinkOne(e.Path, e.LinkTarget); rerr != nil {
					return nil, nil, rerr
				}
				if li, lerr := os.Lstat(tgt); lerr == nil {
					if li.Mode()&os.ModeSymlink == 0 {
						return nil, nil, zegRuleRefuse("D9", "rebuild_target_symlink",
							"重建落点已存在且不是符号链接（✗ 覆盖盘上普通件）", tgt)
					}
					if rerr := os.Remove(tgt); rerr != nil {
						return nil, nil, rerr
					}
				}
				if serr := os.Symlink(e.LinkTarget, tgt); serr != nil {
					return nil, nil, serr
				}
				continue
			}
			if _, err := rf.Seek(e.Off, io.SeekStart); err != nil {
				return nil, nil, err
			}
			g, err := zegOpenRebuildTarget(tgt)
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
	// ★ 卵1①：索引与签名段自身的声明（OCI manifest 档 —— 段表即该段）。
	fmt.Fprintf(stdout, "  索引段 mediaType=%s（OCI manifest）\n", zegMediaManifest)
	for _, e := range idx.Entries {
		sha := e.SHA256
		if len(sha) > 16 {
			sha = sha[:16]
		}
		// ★ 卵1①：逐段**多印** `mediaType=`（取值落 OCI 闭集内）；未标 ⇒ 逐字「未标 mediaType」
		//（✗ 不许静默当已标 —— 照 OCI 放行但必须登记）。
		mt := strings.TrimSpace(e.MediaType)
		if mt == "" {
			mt = "未标 mediaType（登记）"
		}
		extra := ""
		if e.LinkTarget != "" {
			// ★ 链段逐字点出链名（`GAP-20261018-05`）—— 别让「size=0 的段」看着像空文件。
			extra = "  linkTarget=" + e.LinkTarget
		}
		fmt.Fprintf(stdout, "    - %-40s %9d B  off=%d  %s  mediaType=%s  seg=%s%s\n",
			e.Path, e.Size, e.Off, sha, mt, zegSegOfPath(e.Path), extra)
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

// zegFormatSelfTest —— 卵1①②（格式与工具面）的自检：**四态亲测 ＋ 四枚负控**
// （✗ 不与旧档 / 卵3① / 卵6 三行混行 —— 四行分开印）。
//
//	态① 正常卵 ⇒ **写端表的 `mediaType`** ⟷ **读端判读**逐条一致（两侧同表逐条对拍）
//	态② `mediaType` 缺失 / 闭集外 ⇒ **照 OCI 放行 ＋ 登记**（✗ 不 error）
//	态③ 恶意路径逐类 ⇒ **必拒且点名规则**（`../` · 绝对 · 盘符 · 空段 · NUL · 尾随点 · 保留名 · 反斜杠 · 非 NFC）
//	态④ 「同键换语义」（`mediaType` 在闭集内、却与该段 `path` 推出的类不符）⇒ **拒** 且点名档位
//	负控 A：同一份「未标」输入走**不审计**的旧形态 ⇒ **放行**（＝证明这道牙不是永真）
//	负控 B：合法树 pack → verify（重建）⇒ 重建内容与源**逐字节同**（拒执不得伤正路）
//	负控 C：`setuid` 源件 ⇒ 拒 `fs_mode_unsafe`
//	负控 D1–D4：符号链接四道拒各自具名（绝对 / 越界 / 成环 / 超深）· 正控 E：三级相对链 pack ⇒ rebuild 链名逐字同
func zegFormatSelfTest(stdout, stderr io.Writer) int {
	t, err := os.MkdirTemp("", "zerg-egg-format-selftest")
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", zegProg, err)
		return exitFail
	}
	defer os.RemoveAll(t) // 自回收（TMPDIR 前后 delta=0）

	src := filepath.Join(t, "tree")
	for _, d := range []string{"model", "tokenizer"} {
		if err := os.MkdirAll(filepath.Join(src, d), 0o755); err != nil {
			return zegSelfFail(stderr, err)
		}
	}
	if err := os.WriteFile(filepath.Join(src, "config.json"), []byte("{\"arch\":\"format-selftest\"}"), 0o644); err != nil {
		return zegSelfFail(stderr, err)
	}
	if err := os.WriteFile(filepath.Join(src, "tokenizer", "tokenizer.json"), []byte("{\"v\":1}"), 0o644); err != nil {
		return zegSelfFail(stderr, err)
	}
	for _, i := range []int{1, 2} {
		buf := make([]byte, 4096)
		if _, err := rand.Read(buf); err != nil {
			return zegSelfFail(stderr, err)
		}
		if err := os.WriteFile(filepath.Join(src, "model", fmt.Sprintf("w-%d.safetensors", i)), buf, 0o644); err != nil {
			return zegSelfFail(stderr, err)
		}
	}
	egg := filepath.Join(t, "format.egg")
	entries, err := zegPack(src, egg, 4096, "2000-01-01T00:00:00Z")
	if err != nil {
		return zegSelfFail(stderr, err)
	}
	entryN := len(entries)

	// clone —— 复制一枚卵并把**索引体**按 `edit` 改过（段偏移一位不动）。
	clone := func(name string, edit func(ents []map[string]json.RawMessage) error) (string, error) {
		p := filepath.Join(t, name)
		d, rerr := os.ReadFile(egg)
		if rerr != nil {
			return "", rerr
		}
		if werr := os.WriteFile(p, d, 0o644); werr != nil {
			return "", werr
		}
		rerr = zegVerRebuildEgg(p, func(idx []byte) ([]byte, error) {
			var m map[string]json.RawMessage
			if uerr := json.Unmarshal(idx, &m); uerr != nil {
				return nil, uerr
			}
			var ents []map[string]json.RawMessage
			if uerr := json.Unmarshal(m["entries"], &ents); uerr != nil {
				return nil, uerr
			}
			if eerr := edit(ents); eerr != nil {
				return nil, eerr
			}
			eb, merr := json.Marshal(ents)
			if merr != nil {
				return nil, merr
			}
			m["entries"] = eb
			return json.Marshal(m)
		}, 0)
		if rerr != nil {
			return "", rerr
		}
		return p, nil
	}
	bodyOf := func(p string) []byte {
		d, rerr := os.ReadFile(p)
		if rerr != nil {
			return nil
		}
		n := zegIdxLenOf(d)
		dl := len(d)
		if n <= 0 || zegHeaderPrefix+n > dl {
			return nil
		}
		return d[zegHeaderPrefix : zegHeaderPrefix+n]
	}
	putPath := func(ents []map[string]json.RawMessage, i int, p string) error {
		qb, qerr := json.Marshal(p)
		if qerr != nil {
			return qerr
		}
		ents[i]["path"] = json.RawMessage(qb)
		return nil
	}
	findEntry := func(ents []map[string]json.RawMessage, want string) int {
		for i, e := range ents {
			var p string
			if json.Unmarshal(e["path"], &p) == nil && p == want {
				return i
			}
		}
		return -1
	}
	// 态③ 的九类恶意路径（逐类一条规则 ⇒ 拒执必须点名它）。
	malicious := []struct {
		Name string
		Path string
		Rule string
	}{
		{Name: "..", Path: "../etc/passwd", Rule: "path_dotdot"},
		{Name: "绝对", Path: "/etc/passwd", Rule: "path_absolute"},
		{Name: "盘符", Path: "C:/windows/system32", Rule: "path_windows_drive"},
		{Name: "空段", Path: "a//b", Rule: "path_not_normalized"},
		{Name: "NUL", Path: "a\x00b", Rule: "path_nul"},
		{Name: "尾随点", Path: "a.", Rule: "path_trailing_dot_space"},
		{Name: "保留名", Path: "con.txt", Rule: "path_windows_reserved"},
		{Name: "反斜杠", Path: "a\\b", Rule: "path_backslash"},
		{Name: "非NFC", Path: "cafe\u0301.json", Rule: "path_not_nfc"},
	}

	// ---- 态①：写端 ⟷ 读端 `mediaType` 逐条一致 ----
	idx, err := zegParseIndex(egg)
	if err != nil {
		return zegSelfFail(stderr, err)
	}
	unlabeled0 := len(idx.UnlabeledMedia)
	unknown0 := len(idx.UnknownMedia)
	if unlabeled0 != 0 || unknown0 != 0 {
		return zegSelfFail(stderr, fmt.Errorf("态① 新打的卵不该有未标/未知 mediaType：未标=%v 未知=%v",
			idx.UnlabeledMedia, idx.UnknownMedia))
	}
	entriesN := len(idx.Entries)
	if entriesN != entryN {
		return zegSelfFail(stderr, fmt.Errorf("态① 段数对不上：写端 %d ⟷ 读端 %d", entryN, entriesN))
	}
	for i, e := range idx.Entries {
		want := zegMediaTypeOfPath(e.Path)
		if e.MediaType != want {
			return zegSelfFail(stderr, fmt.Errorf("态① 段 %q 的 mediaType=%q ≠ 表推出 %q", e.Path, e.MediaType, want))
		}
		if entries[i].MediaType != e.MediaType {
			return zegSelfFail(stderr, fmt.Errorf("态① 写端 ⟷ 读端不一致：段 %q 写 %q 读 %q", e.Path, entries[i].MediaType, e.MediaType))
		}
	}

	// ---- 态②：未标 / 闭集外 ⇒ 放行 ＋ 登记（✗ 不 error）----
	oldEgg, err := clone("old.egg", func(ents []map[string]json.RawMessage) error {
		for i := range ents {
			delete(ents[i], "mediaType")
		}
		return nil
	})
	if err != nil {
		return zegSelfFail(stderr, err)
	}
	oldIdx, oerr := zegParseIndex(oldEgg)
	if oerr != nil {
		return zegSelfFail(stderr, fmt.Errorf("态② 未标 mediaType 竟被拒（✗ 照 OCI 应放行）：%v", oerr))
	}
	unlabeledOld := len(oldIdx.UnlabeledMedia)
	unknownOld := len(oldIdx.UnknownMedia)
	if unlabeledOld != entryN || unknownOld != 0 {
		return zegSelfFail(stderr, fmt.Errorf("态② 未标档登记面不对：未标 %d（应 %d）未知 %d", unlabeledOld, entryN, unknownOld))
	}
	unkEgg, err := clone("unk.egg", func(ents []map[string]json.RawMessage) error {
		ents[0]["mediaType"] = json.RawMessage("\"application/vnd.zerg.weights.v9+unknown\"")
		rest := len(ents)
		for i := 1; i < rest; i++ {
			delete(ents[i], "mediaType")
		}
		return nil
	})
	if err != nil {
		return zegSelfFail(stderr, err)
	}
	unkIdx, uerr := zegParseIndex(unkEgg)
	if uerr != nil {
		return zegSelfFail(stderr, fmt.Errorf("态② 闭集外 mediaType 竟被拒（✗ 照 OCI 应放行）：%v", uerr))
	}
	unknownUnk := len(unkIdx.UnknownMedia)
	unlabeledUnk := len(unkIdx.UnlabeledMedia)
	if unknownUnk != 1 || unlabeledUnk != entryN-1 {
		return zegSelfFail(stderr, fmt.Errorf("态② 未知类登记面不对：未知 %d（应 1）未标 %d（应 %d）",
			unknownUnk, unlabeledUnk, entryN-1))
	}

	// ---- 态③：恶意路径逐类必拒且点名规则 ----
	for i, mc := range malicious {
		name, rule, badPath := mc.Name, mc.Rule, mc.Path
		badEgg, cerr := clone(fmt.Sprintf("bad-%d.egg", i), func(ents []map[string]json.RawMessage) error {
			return putPath(ents, 0, badPath)
		})
		if cerr != nil {
			return zegSelfFail(stderr, cerr)
		}
		if bodyOf(badEgg) == nil {
			return zegSelfFail(stderr, fmt.Errorf("态③ %s 取不到索引体（夹具坏）", name))
		}
		_, perr := zegParseIndex(badEgg)
		if perr == nil {
			return zegSelfFail(stderr, fmt.Errorf("态③ 恶意路径 %s（%q）竟被放行", name, badPath))
		}
		ref, ok := perr.(*zegRefusal)
		if !ok || ref.Tier != "D9" || !strings.Contains(ref.Msg, rule) {
			return zegSelfFail(stderr, fmt.Errorf("态③ %s 拒了但没点名 D9/规则 %s：%v", name, rule, perr))
		}
	}

	// ---- 态④：同键换语义（闭集内却与该段 path 推出的类不符）⇒ 必拒 ----
	semEgg, err := clone("sem.egg", func(ents []map[string]json.RawMessage) error {
		j := findEntry(ents, "config.json")
		if j < 0 {
			return fmt.Errorf("夹具里找不到 config.json 段")
		}
		qb, qerr := json.Marshal(zegMediaLayer)
		if qerr != nil {
			return qerr
		}
		ents[j]["mediaType"] = json.RawMessage(qb)
		return nil
	})
	if err != nil {
		return zegSelfFail(stderr, err)
	}
	_, semErr := zegParseIndex(semEgg)
	if semErr == nil {
		return zegSelfFail(stderr, fmt.Errorf("态④ 同键换语义竟被放行（✗ 闭集内但类不符必须拒）"))
	}
	semRef, ok := semErr.(*zegRefusal)
	if !ok || semRef.Tier != zegTierSurface || !strings.Contains(semRef.Msg, "mediaType_class_mismatch") {
		return zegSelfFail(stderr, fmt.Errorf("态④ 拒了但没点名 %s/mediaType_class_mismatch：%v", zegTierSurface, semErr))
	}

	// ---- 负控 A：同一份「未标」输入走**不审计**的旧形态 ⇒ 放行（＝本牙非永真）----
	oldBody := bodyOf(oldEgg)
	if oldBody == nil {
		return zegSelfFail(stderr, fmt.Errorf("负控 A 取不到未标档索引体（夹具坏）"))
	}
	auditOld, aerr := zegCompatGateIndexBody(oldBody)
	if aerr != nil {
		return zegSelfFail(stderr, fmt.Errorf("负控 A 前提不成立：不审计的兼容闸在未标档上竟报错：%v", aerr))
	}
	var probe zegIndexJSON
	if jerr := json.Unmarshal(oldBody, &probe); jerr != nil {
		return zegSelfFail(stderr, fmt.Errorf("负控 A 前提不成立：不审计的结构化解码在未标档上竟失败：%v", jerr))
	}

	// ---- 负控 B：合法树 pack → verify（重建）⇒ 重建内容与源逐字节同 ----
	rebuilt := filepath.Join(t, "rebuilt")
	probs, _, verr := zegVerify(egg, rebuilt)
	if verr != nil {
		return zegSelfFail(stderr, verr)
	}
	probN := len(probs)
	if probN != 0 {
		return zegSelfFail(stderr, fmt.Errorf("负控 B 干净卵重建报问题：%v", probs))
	}
	for _, e := range entries {
		a, _ := os.ReadFile(filepath.Join(src, filepath.FromSlash(e.Path)))
		b, _ := os.ReadFile(filepath.Join(rebuilt, filepath.FromSlash(e.Path)))
		if sha256.Sum256(a) != sha256.Sum256(b) {
			return zegSelfFail(stderr, fmt.Errorf("负控 B 重建内容不同：%s", e.Path))
		}
	}

	// ---- 负控 C：`setuid` 源件 ⇒ 拒 `fs_mode_unsafe` ----
	csrc := filepath.Join(t, "csrc")
	if err := os.MkdirAll(csrc, 0o755); err != nil {
		return zegSelfFail(stderr, err)
	}
	cf := filepath.Join(csrc, "f.bin")
	if err := os.WriteFile(cf, []byte("x"), 0o644); err != nil {
		return zegSelfFail(stderr, err)
	}
	if err := os.Chmod(cf, 0o644|os.ModeSetuid); err != nil {
		return zegSelfFail(stderr, err)
	}
	_, cerrC := zegPack(csrc, filepath.Join(t, "c.egg"), 4096, "")
	if cerrC == nil {
		return zegSelfFail(stderr, fmt.Errorf("负控 C setuid 源件竟被打包（✗ 必须拒）"))
	}
	if !strings.Contains(cerrC.Error(), "fs_mode_unsafe") {
		return zegSelfFail(stderr, fmt.Errorf("负控 C 拒了但没点名 fs_mode_unsafe：%v", cerrC))
	}

	// ---- 负控 D1–D4 ＋ 正控 E：符号链接只许相对链（`GAP-20261018-05`）----
	// 四道拒必须**各自具名、互不串名**；正控是三级相对链（打得进 ⇒ 重建得回 ⇒ 链名逐字同）。
	symCase := func(name string, mk func(dir string) error) (string, error) {
		d := filepath.Join(t, name)
		if err := os.MkdirAll(d, 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(filepath.Join(d, "real.bin"), []byte("z"), 0o644); err != nil {
			return "", err
		}
		if err := mk(d); err != nil {
			return "", err
		}
		return d, nil
	}
	symAll := []string{"fs_symlink_absolute", "fs_symlink_escape", "fs_symlink_cycle", "fs_symlink_depth"}
	symWant := []struct {
		name string
		rule string
		mk   func(dir string) error
	}{
		{"d1", "fs_symlink_absolute", func(d string) error {
			return os.Symlink("/etc/hosts", filepath.Join(d, "link.bin"))
		}},
		{"d2", "fs_symlink_escape", func(d string) error {
			if err := os.MkdirAll(filepath.Join(d, "sub"), 0o755); err != nil {
				return err
			}
			return os.Symlink("../../outside.bin", filepath.Join(d, "sub", "link.bin"))
		}},
		{"d3", "fs_symlink_cycle", func(d string) error {
			if err := os.Symlink("b.bin", filepath.Join(d, "a.bin")); err != nil {
				return err
			}
			return os.Symlink("a.bin", filepath.Join(d, "b.bin"))
		}},
		{"d4", "fs_symlink_depth", func(d string) error {
			for i := 0; i < 10; i++ {
				tgt := fmt.Sprintf("l%02d.bin", i+1)
				if i == 9 {
					tgt = "real.bin"
				}
				if err := os.Symlink(tgt, filepath.Join(d, fmt.Sprintf("l%02d.bin", i))); err != nil {
					return err
				}
			}
			return nil
		}},
	}
	for _, c := range symWant {
		dir, derr := symCase("sym-"+c.name, c.mk)
		if derr != nil {
			return zegSelfFail(stderr, derr)
		}
		_, perr := zegPack(dir, filepath.Join(t, "sym-"+c.name+".egg"), 4096, "")
		if perr == nil {
			return zegSelfFail(stderr, fmt.Errorf("负控 %s 竟被打包（✗ 必须拒 %s）", c.name, c.rule))
		}
		if !strings.Contains(perr.Error(), c.rule) {
			return zegSelfFail(stderr, fmt.Errorf("负控 %s 拒了但没点名 %s：%v", c.name, c.rule, perr))
		}
		for _, other := range symAll {
			if other != c.rule && strings.Contains(perr.Error(), other) {
				return zegSelfFail(stderr, fmt.Errorf("负控 %s 串名（本该 %s，报话里还有 %s）：%v", c.name, c.rule, other, perr))
			}
		}
	}

	// ---- 正控 E：三级相对链（`libfoo.so` → `libfoo.so.1` → `libfoo.so.1.12`）----
	// 打得进 ⇒ 重建得回 ⇒ 重建后 `readlink` 逐字等于原链名、且是链不是普通件。
	psrc := filepath.Join(t, "sym-ok")
	if err := os.MkdirAll(psrc, 0o755); err != nil {
		return zegSelfFail(stderr, err)
	}
	if err := os.WriteFile(filepath.Join(psrc, "libfoo.so.1.12"), []byte("ELF"), 0o644); err != nil {
		return zegSelfFail(stderr, err)
	}
	if err := os.Symlink("libfoo.so.1.12", filepath.Join(psrc, "libfoo.so.1")); err != nil {
		return zegSelfFail(stderr, err)
	}
	if err := os.Symlink("libfoo.so.1", filepath.Join(psrc, "libfoo.so")); err != nil {
		return zegSelfFail(stderr, err)
	}
	pegg := filepath.Join(t, "sym-ok.egg")
	if _, perr := zegPack(psrc, pegg, 4096, ""); perr != nil {
		return zegSelfFail(stderr, fmt.Errorf("正控 E 三级相对链竟被拒：%v", perr))
	}
	prb := filepath.Join(t, "sym-ok-rebuilt")
	if probs, _, verr := zegVerify(pegg, prb); verr != nil || len(probs) != 0 {
		return zegSelfFail(stderr, fmt.Errorf("正控 E 重建报错：err=%v probs=%v", verr, probs))
	}
	if fi, lerr := os.Lstat(filepath.Join(prb, "libfoo.so.1.12")); lerr != nil || !fi.Mode().IsRegular() {
		return zegSelfFail(stderr, fmt.Errorf("正控 E 重建后链尾不是普通件：err=%v", lerr))
	}
	for _, c := range []struct{ path, want string }{
		{"libfoo.so", "libfoo.so.1"},
		{"libfoo.so.1", "libfoo.so.1.12"},
	} {
		full := filepath.Join(prb, c.path)
		fi, lerr := os.Lstat(full)
		if lerr != nil || fi.Mode()&os.ModeSymlink == 0 {
			return zegSelfFail(stderr, fmt.Errorf("正控 E 重建后 %s 不是符号链接（err=%v）", c.path, lerr))
		}
		got, rerr := os.Readlink(full)
		if rerr != nil || got != c.want {
			return zegSelfFail(stderr, fmt.Errorf("正控 E 重建后 %s 的链名 %q ≠ 原件 %q（err=%v）", c.path, got, c.want, rerr))
		}
	}

	addKeys := len(auditOld.AdditiveEntryKeys)
	segN := len(zegSegMediaType)
	clsN := len(zegMediaTypeClosedSet())
	malN := len(malicious)
	fmt.Fprintf(stdout, "self-test（卵1 格式与工具面）通过（4 态 ＋ 4 负控 ＋ 1 正控）：① 写端 ⟷ 读端 mediaType 逐条一致（%d 段 · 映射表 %d 段 %d 类）· ② 未标 %d 段 / 未知 %d 段 ⇒ 照 OCI 放行 ＋ 登记（✗ 不 error）· ③ 恶意路径 %d 类逐类必拒且点名规则 · ④ 「同键换语义」mediaType 闭集内却与 path 推出的类不符 ⇒ 拒[%s] 且点名 mediaType_class_mismatch · 负控 A 未标输入走不审计旧形态 ⇒ 放行（牙非永真 · 兼容闸登记可加面段键 %d 个）· 负控 B 合法树 rebuild 逐字节同 · 负控 C setuid 源件 ⇒ 拒[fs_mode_unsafe] · 负控 D1–D4 符号链接四道拒各自具名且互不串名[fs_symlink_absolute / fs_symlink_escape / fs_symlink_cycle / fs_symlink_depth] · 正控 E 三级相对链（libfoo.so→libfoo.so.1→libfoo.so.1.12）pack ⇒ rebuild 链名逐字同\n",
		entryN, segN, clsN, unlabeledOld, unknownUnk, malN, zegTierSurface, addKeys)
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
	// workers —— 卵3① 的**隐藏取旗标**（`--workers N` · 照 `--now` 的同款：✗ 不进帮助面 ⇒ 不污染 `-h` 对拍）。
	// 0 ＝ 自动档（并发度 `W = clamp(ceil(D/h), 1, NumCPU)` · `h` 由内存探针现测）。
	workers int
}

var zegOpts = []string{"--align", "--rebuild-into", "--self-test", "--chunk-size", "--help", "--now", "--workers"}

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
			case "--workers":
				v, ok := take()
				if !ok {
					return nil, fmt.Sprintf("argument %s: expected one argument", name)
				}
				n, werr := strconv.Atoi(v)
				if werr != nil {
					return nil, fmt.Sprintf("argument --workers: invalid int value: '%s'", v)
				}
				a.workers = n
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
		// 旧档（py 真搬面 · 10 项）先跑；★ 再跑卵3① 的并行＋缓存档、卵6 的版本与兼容档、
		// 卵1 的格式与工具面档（**四行分开印** ⇒ 前三行契约读数一字不动 · ✗ 不许漂）。
		if rc := zegSelfTest(stdout, stderr); rc != exitOK {
			return rc
		}
		if rc := zegvSelfTest(stdout, stderr); rc != exitOK {
			return rc
		}
		if rc := zegVersionSelfTest(stdout, stderr); rc != exitOK {
			return rc
		}
		return zegFormatSelfTest(stdout, stderr)
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
		// ★ 卵3①：走 `zerg_egg_verify.go` 的**并行 ＋ 内容寻址缓存**入口（判据/问题串与旧档同源 ·
		// 逐字节一致由 `TestZergEggVerifyParallelMatchesSingle` 钉着）。旧档 `zegVerify` 仍是行为真源。
		res, err := zegvVerifyEntry(stdout, a.a, a.rebuildInto, a.workers)
		if err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", zegProg, err)
			return exitFail
		}
		if len(res.Problems) > 0 {
			fmt.Fprintf(stdout, "✗ 校验失败 %d 项：\n", len(res.Problems))
			for _, p := range res.Problems {
				fmt.Fprintf(stdout, "   - %s\n", p)
			}
			return exitFail
		}
		suffix := ""
		if a.rebuildInto != "" {
			suffix = "，已重建到 " + a.rebuildInto
		}
		mark := ""
		if res.CacheHit {
			mark = " " + zegvCacheMark
		}
		fmt.Fprintf(stdout, "✓ 校验通过（%d 段，align=%d）%s%s\n", len(res.Index.Entries), res.Index.Align, mark, suffix)
		// ★ 卵1① 的**登记面**印出（✗ 不静默当已标）：未标 / 闭集外的 `mediaType` 照 OCI **放行**
		// 但必须逐字点名。★ 命中缓存档是**空壳索引**（无 entries）⇒ 本段不印（口径见 `zegvShellIndex`）。
		if res.Index != nil {
			unlabeled := len(res.Index.UnlabeledMedia)
			unknown := len(res.Index.UnknownMedia)
			if unlabeled > 0 {
				fmt.Fprintf(stdout, "· 未标 mediaType %d 段（已登记 · 照 OCI「MUST NOT error」放行）：%s\n",
					unlabeled, strings.Join(res.Index.UnlabeledMedia, " · "))
			}
			if unknown > 0 {
				fmt.Fprintf(stdout, "· 未知 mediaType %d 段（已登记 · 照 OCI 放行）：%s\n",
					unknown, strings.Join(res.Index.UnknownMedia, " · "))
			}
		}
		return exitOK
	}
	fmt.Fprint(stdout, zegHelpText)
	return zegExitToolUsage
}
