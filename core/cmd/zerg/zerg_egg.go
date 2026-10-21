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
//	· **已接线（inproc 表分派）** —— ✗ 不在 `core/cmd/zerg/main.go` 注册命令（✗ 不进命令树 · `add_step` **未挂**（`precommit-gates.sh` 现读零命中））；
//	· **不删** `scripts/node-a/zerg-egg.py` 原件（退场规则：**对拍通过后才**逐件点名退场）；
//	· **已接线**：入口 `zergEggGateMain` 由 `gateCheckGoEntry["zerg-egg"]` 分派（`zerg gate check zerg-egg`）· 原文此处断言与现读相反（只改文案 · 未改行为）。
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
// ---- 出厂声明结束。 **已接线**：入口 `zergEggGateMain` 由 `gateCheckGoEntry["zerg-egg"]` 分派（`zerg gate check zerg-egg`）· 原文此处断言与现读相反（只改文案 · 未改行为）。
package main

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
	zegMagic = "ZEGG"
	// zegFormatVer —— **本端（读端）认知的格式主号**：v2 起 = 段表化布局（8 KiB 双槽头 ＋ 尾段表）。
	// ★ 分工（三枚常量，✗ 别混）：`zegFormatVerV1` = 1（历史布局 · 读端**仍支持** ⇒ 后向兼容）、
	//   `zegV2LayoutVer` = 2（现读默认写端 `zegPackV2` 出的布局）、本常量 = **本端主号**（＝ 2）。
	zegFormatVer    = zegV2LayoutVer
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

// zegHelpText —— `-h` / 无参数时打的帮助正文（**形状**逐字取自 py 3.9 argparse · COLUMNS=80；★ 本版多一条 `--libs-dir` ⇒ 字节数已不是当年那 506 · 现值见 `zerg cocoon pack -h` 现跑）。
const zegHelpText = "usage: zerg-egg.py [-h] [--align ALIGN] [--engine-dir ENGINE_DIR]\n" +
	"                   [--libs-dir LIBS_DIR] [--rebuild-into REBUILD_INTO]\n" +
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
	"  --engine-dir ENGINE_DIR\n" +
	"                        打蛋时**一起收进蛋**的引擎目录（可给多次 ⇒ 蛋内 engine/<目录基名>/…；\n" +
	"                        闭包缺件会打蛋失败并逐条点名 —— 只收目录，收谁由构建脚本决定）\n" +
	"  --libs-dir LIBS_DIR\n" +
	"                        打蛋时**一起收进蛋**的库目录（可给多次 ⇒ 蛋内 libs/<目录基名>/…；\n" +
	"                        与 --engine-dir 同源：只收目录，闭包缺件会打蛋失败并逐条点名）\n" +
	"  --rebuild-into REBUILD_INTO\n" +
	"  --self-test\n" +
	"  --chunk-size CHUNK_SIZE\n" +
	"                        xfer 的分块大小（默认 64 MiB）\n"

// zegUsageText —— 错误档先打的 `usage:` 三行（与 help 正文同源片段）。
const zegUsageText = "usage: zerg-egg.py [-h] [--align ALIGN] [--engine-dir ENGINE_DIR]\n" +
	"                   [--libs-dir LIBS_DIR] [--rebuild-into REBUILD_INTO]\n" +
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
	// ---- ★ v2 布局面（v1 卵这些字段为零值：`Layout=1`、其余 0/空 ⇒ 旧面逐字不变）------------------
	// Layout —— 布局主号（1 = 历史 v1 · 2 = 段表化 v2）。
	Layout int
	// Gen / UUID —— 头槽里的**代际**与**卵唯一号**（★ 续传令牌取 `(uuid, gen, table_sha256)`，
	// ✗ 绝不用整件 ETag —— 追加式格式里整件 digest 每加一段就变）。
	Gen  int64
	UUID string
	// TableOff / TableLen / FileLen / TableSHA —— 段表的落点/长度、数据末尾、表内容 sha256（十六进制）。
	TableOff int64
	TableLen int64
	FileLen  int64
	TableSHA string
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

// zegBuildPrefix —— **v1** 固定 32 字节前缀的**唯一实现**（`zegParseIndexV1` 与之同源）。
// ★ 前缀里的主号恒 `zegFormatVerV1`（＝1）：v2 的头是 8 KiB 双槽（见 `zegV2SlotEncode`），✗ 不走这里。
func zegBuildPrefix(align int, indexLen int) []byte {
	b := make([]byte, zegHeaderPrefix)
	copy(b[0:4], zegMagic)
	binary.LittleEndian.PutUint32(b[4:8], uint32(zegFormatVerV1))
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
	// ★ 卵沙箱第 6 步补一条：`libs/**` 与 `engine/**` **同类**（它们是同一批引擎侧 .so，只是打包时
	// 由 `--libs-dir` 收进来的）。不加这条时 `libggml.so.0`（扩展名是 `.0` 而非 `.so`）会落到
	// 「配方段」—— 位置对了、**声明错了**（mediaType 写着 config，而它是共享库）。
	if strings.HasPrefix(lower, "engine/") || strings.Contains(lower, "/engine/") ||
		strings.HasPrefix(lower, "libs/") || strings.Contains(lower, "/libs/") ||
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
//
// ★ `GAP-20261015-25`（D9「两差」→ 对齐）：报话取 13 档表 D9 行（＝设计原文，**逐字**），
// 规则名/依据/实读路径进 `Detail` 行，退码取表内 `Code`＝2（改前：`拒：段 path … —— 命中规则 …`，rc=1）。
// ★ 注意：本函数**只管 path 语义**；借挂 D9 号的 `fs_mode_unsafe` / `fs_symlink_*` / `rebuild_*`
// 是另一类触发（不在设计 D9 的「含 `..` / 绝对 / 盘符 / 空段 / NUL」里）⇒ 仍走 `zegRuleRefuse`（rc=1）。
func zegRefusePath(rel string) error {
	for _, r := range zegPathRules {
		if r.Bad(rel) {
			return zegTierRefuse("D9", fmt.Sprintf("命中规则 %s（%s）：段 path %q（须为规范化 NFC 相对路径）",
				r.Name, r.Why, rel))
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

// ---- v2 布局：段表化 ＋ 引擎段放尾（设计稿 Q3 已拍 · 本件唯一真源）------------------------------
//
// 形状（★ 与 v1 不兼容 ⇒ 主号 2；旧读端由主号闸 **D2 前向拒**，✗ 不按旧偏移猜着读）：
//
//	[0, 8192)          固定头：**两槽 × 4096**，各带 `gen`（单调）＋ `crc32c`；读时取「crc 有效且 gen 最大」
//	[align, file_len)  数据段：**非引擎段在前（路径序）· 引擎段一律最后（路径序）**
//	table_off          `= align_up(file_len, 65536)` ⇒ **段表放文件尾**
//	[table_off, +len)  段表（JSON · 与 v1 **同一份键形状**：magic/version/align/tool/created_at/entries）
//
// 提交点 = **头槽指针翻转**（表不双写 —— 依据 LMDB「Transaction N writes meta page #(N % 2)」）：
//
//	data/engine → fsync → table → fsync → header(slot = gen % 2) → fsync（darwin 另加 F_FULLFSYNC）
//
// 任一步崩 ⇒ 旧 gen 那一槽仍 crc 有效且指向旧表 ⇒ **旧代仍可读**。升引擎只**追加**新引擎段
// ＋ 重写尾表 ＋ 翻一次头槽 ⇒ **前面的权重/配方段一个字节都不碰**（证据面见 `zegV2UpgradeEngine`）。
const (
	// zegFormatVerV1 —— v1 主号（历史布局：32 B 前缀 ＋ 变长索引 ＋ 按路径序的段）。读端**仍支持**。
	zegFormatVerV1 = 1
	// zegV2LayoutVer —— v2 主号：段表化布局（双槽头 ＋ 尾段表 ＋ 引擎段在尾）。
	zegV2LayoutVer = 2
	// zegV2HeaderBytes / zegV2SlotBytes —— 固定头 8 KiB ＝ 双槽 × 4 KiB（两槽交替写 ⇒ 指针翻转即提交点）。
	zegV2HeaderBytes = 8 << 10
	zegV2SlotBytes   = 4 << 10
	// zegV2TableAlign —— 段表落点：`file_len` 之后第一个 64 KiB 边界。
	zegV2TableAlign = 64 << 10
	// zegV2TableMaxBytes —— 读段表时的分配上界（防畸形头把内存吃光）。
	zegV2TableMaxBytes = 64 << 20
	// zegSegEngine —— 段分类里「引擎段」的取值（排序与 info/升级都读它 ⇒ 单点真源）。
	zegSegEngine = "引擎段"
	// 头槽内字段偏移（little endian · 槽 = 4096 B：前 96 B 语义 ＋ 4 B crc32c，其余留后续可加面）。
	zegV2OffMagic    = 0
	zegV2OffLayout   = 4
	zegV2OffGen      = 8
	zegV2OffFileLen  = 16
	zegV2OffTableOff = 24
	zegV2OffTableLen = 32
	zegV2OffAlign    = 40
	zegV2OffUUID     = 48
	zegV2OffTableSHA = 64
	zegV2OffCRC      = 96
	zegV2CRCSpan     = 96
	// zegV2FCNTLFullFsync —— Darwin 的 `F_FULLFSYNC`（`fsync` 在 macOS 上不保证落盘）。
	// ★ 只在 `runtime.GOOS == "darwin"` 那一支用（Linux 无此常量 ⇒ ✗ 不写 `syscall.F_FULLFSYNC`）。
	zegV2FCNTLFullFsync = 51
)

// zegV2Slot —— 一个头槽的内容（**唯一提交点**：翻转它才算这一代落成）。
type zegV2Slot struct {
	Gen      int64
	FileLen  int64
	TableOff int64
	TableLen int64
	Align    int64
	UUID     [16]byte
	TableSHA [32]byte
	Slot     int
}

// zegV2CRC32C —— 头槽校验（Castagnoli 多项式，与 OCI/etcd 的 crc32c 同族）。
func zegV2CRC32C(b []byte) uint32 { return crc32.Checksum(b, crc32.MakeTable(crc32.Castagnoli)) }

// zegV2SlotEncode —— 把一个槽编成 4096B（前 96B 语义 ＋ 末位 crc32c，其余补零 ⇒ 可加面留白）。
func zegV2SlotEncode(s zegV2Slot) []byte {
	b := make([]byte, zegV2SlotBytes)
	copy(b[zegV2OffMagic:], zegMagic)
	binary.LittleEndian.PutUint32(b[zegV2OffLayout:], uint32(zegV2LayoutVer))
	binary.LittleEndian.PutUint64(b[zegV2OffGen:], uint64(s.Gen))
	binary.LittleEndian.PutUint64(b[zegV2OffFileLen:], uint64(s.FileLen))
	binary.LittleEndian.PutUint64(b[zegV2OffTableOff:], uint64(s.TableOff))
	binary.LittleEndian.PutUint64(b[zegV2OffTableLen:], uint64(s.TableLen))
	binary.LittleEndian.PutUint64(b[zegV2OffAlign:], uint64(s.Align))
	copy(b[zegV2OffUUID:zegV2OffUUID+16], s.UUID[:])
	copy(b[zegV2OffTableSHA:zegV2OffTableSHA+32], s.TableSHA[:])
	binary.LittleEndian.PutUint32(b[zegV2OffCRC:], zegV2CRC32C(b[:zegV2CRCSpan]))
	return b
}

// zegV2SlotDecode —— 解一个槽；magic / 布局主号 / crc 任一不符 ⇒ `(槽, false)`（**坏槽不参与择优**）。
func zegV2SlotDecode(b []byte, slot int) (zegV2Slot, bool) {
	var s zegV2Slot
	if len(b) < zegV2SlotBytes || string(b[zegV2OffMagic:zegV2OffMagic+4]) != zegMagic {
		return s, false
	}
	if binary.LittleEndian.Uint32(b[zegV2OffLayout:]) != uint32(zegV2LayoutVer) {
		return s, false
	}
	if zegV2CRC32C(b[:zegV2CRCSpan]) != binary.LittleEndian.Uint32(b[zegV2OffCRC:]) {
		return s, false
	}
	s.Gen = int64(binary.LittleEndian.Uint64(b[zegV2OffGen:]))
	s.FileLen = int64(binary.LittleEndian.Uint64(b[zegV2OffFileLen:]))
	s.TableOff = int64(binary.LittleEndian.Uint64(b[zegV2OffTableOff:]))
	s.TableLen = int64(binary.LittleEndian.Uint64(b[zegV2OffTableLen:]))
	s.Align = int64(binary.LittleEndian.Uint64(b[zegV2OffAlign:]))
	copy(s.UUID[:], b[zegV2OffUUID:zegV2OffUUID+16])
	copy(s.TableSHA[:], b[zegV2OffTableSHA:zegV2OffTableSHA+32])
	s.Slot = slot
	return s, true
}

// zegV2AlignUp —— 向上取整到 a（a 为 2 的幂；a ≤ 1 ⇒ 原样）。
func zegV2AlignUp(n, a int64) int64 {
	if a <= 1 {
		return n
	}
	if r := n % a; r != 0 {
		return n + (a - r)
	}
	return n
}

// zegV2PickSlot —— 读头 8 KiB，取「**crc 有效且 gen 最大**」那个槽（★ 读端只看这一条规则）。
// 两槽的 gen 不同（初打 = 槽0 gen0 基线 / 槽1 gen1 现行；升引擎 = 槽 (gen%2) 交替）⇒ 取大者即最新代。
func zegV2PickSlot(f *os.File) (zegV2Slot, error) {
	head := make([]byte, zegV2HeaderBytes)
	if _, err := f.ReadAt(head, 0); err != nil {
		return zegV2Slot{}, zegRuleRefuse("D1", "v2_header_short",
			"v2 固定头要 8 KiB（双槽），读不满 ⇒ 拒（✗ 不猜）", "卵的头")
	}
	s0, ok0 := zegV2SlotDecode(head[0:zegV2SlotBytes], 0)
	s1, ok1 := zegV2SlotDecode(head[zegV2SlotBytes:2*zegV2SlotBytes], 1)
	switch {
	case ok0 && ok1:
		if s1.Gen > s0.Gen {
			return s1, nil
		}
		return s0, nil
	case ok0:
		return s0, nil
	case ok1:
		return s1, nil
	}
	return zegV2Slot{}, zegRuleRefuse("D1", "v2_header_no_valid_slot",
		"v2 双槽的 crc32c 都不对（没有可提交的世代 ⇒ 拒，✗ 按半截头猜）", "卵的头")
}

// zegV2ShaRange —— 对 [off, off+n) 现算 sha256（升级前后各算一次 ⇒ 「前面没被重写」的机器证据）。
func zegV2ShaRange(f *os.File, off, n int64) (string, error) {
	h := sha256.New()
	buf := make([]byte, 1<<20)
	for n > 0 {
		k := int64(len(buf))
		if n < k {
			k = n
		}
		m, err := f.ReadAt(buf[:k], off)
		if m > 0 {
			h.Write(buf[:m])
			off += int64(m)
			n -= int64(m)
		}
		if err != nil {
			if n == 0 && err == io.EOF {
				break
			}
			return "", err
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// zegV2FullSync —— 落盘语义：先 `fsync`，darwin 再补一刀 `F_FULLFSYNC`（✗ 只靠 fsync 不保证到盘）。
func zegV2FullSync(f *os.File) error {
	if err := f.Sync(); err != nil {
		return err
	}
	if runtime.GOOS == "darwin" {
		if _, _, e := syscall.Syscall(syscall.SYS_FCNTL, f.Fd(), uintptr(zegV2FCNTLFullFsync), 0); e != 0 {
			return e
		}
	}
	return nil
}

// zegV2Inode —— 取 st_ino（升级是**原地写** ⇒ inode 不变；作「没有换件」的旁证）。
func zegV2Inode(fi os.FileInfo) uint64 {
	if st, ok := fi.Sys().(*syscall.Stat_t); ok {
		return uint64(st.Ino)
	}
	return 0
}

// zegV2OrderEngineLast —— v2 段序：**非引擎段在前 · 引擎段一律最后**（两段各自保持原路径序）。
func zegV2OrderEngineLast(entries []*zegEntry) []*zegEntry {
	out := make([]*zegEntry, 0, len(entries))
	for _, e := range entries {
		if zegSegOfPath(e.Path) != zegSegEngine {
			out = append(out, e)
		}
	}
	for _, e := range entries {
		if zegSegOfPath(e.Path) == zegSegEngine {
			out = append(out, e)
		}
	}
	return out
}

// zegV2EngineAtTail —— 段表里「引擎段是否全在尾部」（info 面与自检共用同一判据）。
func zegV2EngineAtTail(entries []zegEntryJSON) bool {
	seen := false
	for _, e := range entries {
		if zegSegOfPath(e.Path) == zegSegEngine {
			seen = true
			continue
		}
		if seen {
			return false
		}
	}
	return true
}

// zegV2ParseIndex —— v2 读端（唯一入口仍由 `zegParseIndex` 分派）：头取槽 → 表 → JSON → 审计。
// ★ 头的每个字段都当**不可信输入**判：align / 表长 / 表落点 / file_len / 表 sha256 逐条有闸。
func zegV2ParseIndex(f *os.File) (*zegIndex, error) {
	slot, err := zegV2PickSlot(f)
	if err != nil {
		return nil, err
	}
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	size := fi.Size()
	if slot.Align < zegMinAlign || !zegIsPow2(slot.Align) {
		return nil, zegTierRefuse("D4", fmt.Sprintf("v2 头槽里的 align=%d（须 2 的幂且 ≥%d · 读端只信文件里记的 align）", slot.Align, zegMinAlign))
	}
	if slot.TableLen <= 0 || slot.TableLen > zegV2TableMaxBytes {
		return nil, zegRuleRefuse("D1", "v2_table_len_invalid",
			"头槽里的段表长度不合理（≤0 或超上界）", "卵的头槽")
	}
	if slot.TableOff < zegV2HeaderBytes || slot.TableOff%zegV2TableAlign != 0 ||
		slot.TableOff+slot.TableLen > size {
		return nil, zegRuleRefuse("D1", "v2_table_out_of_range",
			"段表落点不合规（须在 8 KiB 头之后、64 KiB 对齐、且不出文件末尾）", "卵的头槽")
	}
	if slot.FileLen < zegV2HeaderBytes || slot.FileLen > slot.TableOff {
		return nil, zegRuleRefuse("D1", "v2_file_len_invalid",
			"头槽里的 file_len 不在 [8 KiB, 段表落点] 区间", "卵的头槽")
	}
	body := make([]byte, slot.TableLen)
	if _, err := f.ReadAt(body, slot.TableOff); err != nil {
		return nil, fmt.Errorf("段表读不满（off=%d len=%d）：%v", slot.TableOff, slot.TableLen, err)
	}
	if sum := sha256.Sum256(body); sum != slot.TableSHA {
		return nil, zegRuleRefuse("D1", "v2_table_sha_mismatch",
			"段表字节与头里的 sha256 记录不符（指针指向一张坏表 ⇒ 拒，✗ 不猜）", "卵的段表")
	}
	// ★ 与 v1 同源的三道（面审计 / 主号 / `min_readable` 牙）：✗ 不另写第二份判据。
	if _, err := zegCompatGateIndexBody(body); err != nil {
		return nil, err
	}
	var raw zegIndexJSON
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, zegTierRefuse("D13", err.Error())
	}
	if raw.Magic != zegMagic {
		return nil, zegTierRefuse("D1", fmt.Sprintf("段表里的 magic=%q", raw.Magic),
			hex.EncodeToString([]byte(raw.Magic)))
	}
	if raw.Version != zegV2LayoutVer {
		return nil, fmt.Errorf("段表里的 version=%d ≠ %d（头与表不一致）", raw.Version, zegV2LayoutVer)
	}
	if raw.Align != slot.Align {
		return nil, zegTierRefuse("D4", fmt.Sprintf("段表 align=%d ≠ 头槽 align=%d（同一条卵里两处 align 必须一致）", raw.Align, slot.Align))
	}
	idx := &zegIndex{
		Magic:     raw.Magic,
		Version:   raw.Version,
		Align:     raw.Align,
		Tool:      raw.Tool,
		CreatedAt: raw.CreatedAt,
		Entries:   raw.Entries,
		Layout:    zegV2LayoutVer,
		Gen:       slot.Gen,
		UUID:      hex.EncodeToString(slot.UUID[:]),
		TableOff:  slot.TableOff,
		TableLen:  slot.TableLen,
		FileLen:   slot.FileLen,
		TableSHA:  hex.EncodeToString(slot.TableSHA[:]),
	}
	for _, e := range idx.Entries {
		if e.Off < zegV2HeaderBytes || e.Off+e.Size > slot.FileLen {
			return nil, zegRuleRefuse("D1", "v2_segment_outside_data",
				fmt.Sprintf("段 %q 的 [off,off+size)=[%d,%d) 落在数据区 [%d,%d] 之外",
					e.Path, e.Off, e.Off+e.Size, zegV2HeaderBytes, slot.FileLen), "卵的段表")
		}
	}
	if err := zegAuditIndexEntries(idx); err != nil {
		return nil, err
	}
	return idx, nil
}

// zegRecipeWeightsDecl —— 蛋**自带的**机器可读权重声明（`recipe.json` 的可选面）。
//
// 为什么要有它（GAP-20261018-16 的「打蛋面」）：从蛋孵化时 `/models` 下的一切只从蛋内
// `<egg>/models/` 取（backend 侧 `eggWeightSource`），所以**打蛋时**就该判「声明要的权重真收进蛋了吗」，
// ✗ 不留到孵化器 spec 期甚至引擎装载期。打包端（本文件）**不认识名册**（它是纯文件工具）⇒
// 声明只能来自源树自己带的 `recipe.json`：
//
//	{"weights": ["Qwen…-00001-of-00004.gguf", …], "require_models": true}
//
// ★ 无这两个键 ⇒ 不适用（返回 nil）：老蛋的 recipe 是自由文本（如 real10 的 `note`），
// 行为**逐字不变**（零回归）。判据只认这两格，✗ 不从 note 里猜。
type zegWeightsDecl struct {
	Weights       []string `json:"weights"`
	RequireModels bool     `json:"require_models"`
}

// zegWeightsDeclOf 从 recipe.json 的字节解析声明；文件缺失 ⇒ (nil, nil)；有但解析不了 ⇒ 具名报错。
func zegWeightsDeclOf(b []byte, where string) (*zegWeightsDecl, error) {
	if len(b) == 0 {
		return nil, nil
	}
	var d zegWeightsDecl
	if err := json.Unmarshal(b, &d); err != nil {
		return nil, fmt.Errorf("%s：recipe.json 不是合法 JSON（%v）——拒打包/拒认（声明面读不出就不许当「没声明」）", where, err)
	}
	if len(d.Weights) == 0 && !d.RequireModels {
		return nil, nil // 不适用（老蛋形态：自由文本 recipe）
	}
	for i, w := range d.Weights {
		if strings.TrimSpace(w) == "" {
			return nil, fmt.Errorf("%s：recipe.json 的 weights[%d] 是空串——拒（声明面自相矛盾）", where, i)
		}
	}
	return &d, nil
}

// zegWeightsDeclCheck 声明的每一件在蛋里必须真有 `models/<基名>`；缺则**具名拒**（列出缺的基名）。
//
// hasModels —— 「蛋里有没有这一件」的判据由两个面各自给（打包面看源树/收进来的段；校验面看索引）。
// ★ 判据只认 `models/<基名>`：蛋模式空间内落点就是 `/models/<基名>`，蛋里放在别处等于没带。
func zegWeightsDeclCheck(d *zegWeightsDecl, hasModels func(name string) bool, hasModelsLayer bool, where string) error {
	if d == nil {
		return nil
	}
	if d.RequireModels && !hasModelsLayer {
		return fmt.Errorf("%s：recipe.json 声明了 require_models，但蛋里**没有 models/ 这一层**——"+
			"拒：从蛋孵化时 /models 下的一切只从蛋内取（✗ 不回落宿主，回落等于「原目录可删」不成立）", where)
	}
	var missing []string
	for _, w := range d.Weights {
		name := filepath.Base(strings.TrimSpace(w))
		if name == "" || name == "." || name == ".." || name == string(filepath.Separator) {
			return fmt.Errorf("%s：recipe.json 的 weights 项 %q 推不出基名——拒", where, w)
		}
		if !hasModels(name) {
			missing = append(missing, "models/"+name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("%s：recipe.json 声明要带 %d 件权重，但蛋里缺 %d 件：%s —— 拒："+
			"从蛋孵化时空间内 /models/<基名> 的挂载源就是蛋内 models/<基名>（片组按原基名平铺）；"+
			"缺件会被推到孵化期具名拒、或更晚的引擎装载期", where, len(d.Weights), len(missing), strings.Join(missing, "、"))
	}
	return nil
}

// zegWeightsDeclGateTree —— **打蛋面**：源树的 recipe.json 声明的权重必须真在收进来的段里。
func zegWeightsDeclGateTree(src string, entries []*zegEntry) error {
	p := filepath.Join(src, "recipe.json")
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // 源树没带 recipe.json ⇒ 无声明可判（不适用）
		}
		return fmt.Errorf("读 %s：%w —— 拒打包（声明面读不出就不许当「没声明」）", p, err)
	}
	d, derr := zegWeightsDeclOf(b, "打蛋面")
	if derr != nil {
		return derr
	}
	if d == nil {
		return nil
	}
	inEgg := make(map[string]bool, len(entries))
	layer := false
	for _, e := range entries {
		if e.LinkTarget != "" {
			continue // 链段不算「带进蛋的件」
		}
		inEgg[e.Path] = true
		if strings.HasPrefix(e.Path, "models/") {
			layer = true
		}
	}
	return zegWeightsDeclCheck(d, func(name string) bool { return inEgg["models/"+name] }, layer, "打蛋面")
}

// zegWeightsDeclGateEgg —— **校验面**（`cocoon verify`）：蛋内 recipe.json 声明的权重必须真在索引里。
//
// 索引是「壳」（缓存命中档不带 entries）时跳过并明说：没有条目就没有判据可言，此时**不给结论**
// 比假绿好，但也不该把整枚蛋判红（缓存那半条已有它自己的判据）。
func zegWeightsDeclGateEgg(eggPath string, ix *zegIndex) error {
	if ix == nil || len(ix.Entries) == 0 {
		return nil
	}
	have := make(map[string]*zegEntryJSON, len(ix.Entries))
	layer := false
	for i := range ix.Entries {
		e := &ix.Entries[i]
		if e.LinkTarget != "" {
			continue
		}
		have[e.Path] = e
		if strings.HasPrefix(e.Path, "models/") {
			layer = true
		}
	}
	rec, ok := have["recipe.json"]
	if !ok {
		return nil
	}
	f, err := os.Open(eggPath)
	if err != nil {
		return fmt.Errorf("校验面：读 %s：%w", eggPath, err)
	}
	defer f.Close()
	if rec.Size <= 0 || rec.Size > 1<<20 {
		return nil // recipe 异常大小 ⇒ 不在这里判（段长/越界已有其它判据）
	}
	buf := make([]byte, rec.Size)
	if _, err := f.ReadAt(buf, rec.Off); err != nil {
		return fmt.Errorf("校验面：读 recipe.json 段（off=%d size=%d）：%w", rec.Off, rec.Size, err)
	}
	d, derr := zegWeightsDeclOf(buf, "校验面")
	if derr != nil {
		return derr
	}
	return zegWeightsDeclCheck(d, func(name string) bool { _, ok := have["models/"+name]; return ok }, layer, "校验面")
}

// zegPackV2 —— **写端默认档**（命令面 `cocoon pack` 走它）：打一枚 v2 卵。
// 写序照 ③：data/engine → fsync → table → fsync → header（槽 0 基线 gen0，槽 1 现行 gen1）→ fsync。
//
// ★ 卵沙箱第 3 步：`engineDirs` = 打蛋时**一起收进蛋**的引擎目录（可给多次 ⇒ 蛋内 `engine/<基名>/…`）。
// 收完**一律过闭包自检**（`zegClosureGate` ⇒ 见 `zerg_egg_closure.go`）：缺一件 NEEDED / 缺
// loader / 缺 libc / 缺 libm ⇒ **当场失败并逐条点名**，且**在写第一个字节之前**就失败（✗ 不产半截蛋）。
// 不给 `engineDirs` 时行为与旧档**逐字不变**（只多跑一遍只读自检：无动态件 ⇒ 不要求三件）。
func zegPackV2(src, egg string, align int, now string, engineDirs ...string) ([]*zegEntry, *zegV2Slot, error) {
	return zegPackV2Tree(src, egg, align, now, engineDirs, nil)
}

// zegPackV2Tree —— `zegPackV2` 的**双目录面**版本（引擎目录 ＋ 库目录）。
//
// ★ 为什么另开一个入口而不是把 `zegPackV2` 改成两枚切片参数：`zegPackV2` 的**变参形态**已被
// 多处调用点与用例钉住（`zegPackV2(src, egg, align, now, dirs...)`）—— 改签名会造一堆无谓随动；
// 而 `--libs-dir` 是**新增面**，让它走新入口最省。「不给 libDirs 时行为与旧档逐字不变」这条
// 由 `zegPackV2` 直接转调保证（`nil` 切片 ⇒ 下面那段整块跳过）。
func zegPackV2Tree(src, egg string, align int, now string, engineDirs, libDirs []string) ([]*zegEntry, *zegV2Slot, error) {
	if align == 0 {
		align = zegAlignOf()
	}
	if !zegIsPow2(int64(align)) || align < zegMinAlign {
		return nil, nil, fmt.Errorf("align 必须是 2 的幂且 >= %d（收到 %d）", zegMinAlign, align)
	}
	entries, err := zegCollectEntries(src)
	if err != nil {
		return nil, nil, err
	}
	if len(entries) == 0 && len(engineDirs) == 0 && len(libDirs) == 0 {
		return nil, nil, fmt.Errorf("源目录里没有文件：%s", src)
	}
	// ★ 引擎目录（sysroot）：段路径 `engine/<基名>/…` ⇒ 段分类恒「引擎段」；撞名即拒。
	if len(engineDirs) > 0 {
		taken := make(map[string]string, len(entries))
		for _, e := range entries {
			taken[e.Path] = "源目录"
		}
		for _, d := range engineDirs {
			ee, derr := zegEngineDirEntries(d, taken)
			if derr != nil {
				return nil, nil, derr
			}
			entries = append(entries, ee...)
		}
	}
	// ★ 库目录（卵沙箱第 6 步）：段路径 `libs/<基名>/…` ⇒ backend 从蛋孵化时整棵落到空间 /libs。
	// 与引擎目录**同一套规则**（只收目录 / 撞名即拒 / mode 安装语义归一 / 闭包缺件即失败）。
	if len(libDirs) > 0 {
		taken := make(map[string]string, len(entries))
		for _, e := range entries {
			taken[e.Path] = "源目录 / 引擎目录"
		}
		for _, d := range libDirs {
			ee, derr := zegLibsDirEntries(d, taken)
			if derr != nil {
				return nil, nil, derr
			}
			entries = append(entries, ee...)
		}
	}
	// ★ GAP-20261018-16「打蛋面」自检：recipe.json 机器可读声明（`weights` / `require_models`）里
	// 点名的权重，必须在**收进来的段**里真有 `models/<基名>` —— 缺则具名拒，且在写第一个字节之前。
	// （老蛋的 recipe 是自由文本 ⇒ 本判据不适用，行为逐字不变。）
	if werr := zegWeightsDeclGateTree(src, entries); werr != nil {
		return nil, nil, werr
	}
	// ★ 闭包自检（本单核心）：缺件 ⇒ 具名拒，且**在落盘之前**（✗ 不产半截蛋）。
	if _, gerr := zegClosureGate(entries); gerr != nil {
		return nil, nil, gerr
	}
	entries = zegV2OrderEngineLast(entries)
	var uuid [16]byte
	if _, err := rand.Read(uuid[:]); err != nil {
		return nil, nil, err
	}
	out, err := os.Create(egg)
	if err != nil {
		return nil, nil, err
	}
	fail := func(e error) ([]*zegEntry, *zegV2Slot, error) {
		out.Close()
		os.Remove(egg)
		return nil, nil, e
	}
	if _, err := out.WriteAt(make([]byte, zegV2HeaderBytes), 0); err != nil { // 头块占位（真头最后写）
		return fail(err)
	}
	cur := zegV2AlignUp(zegV2HeaderBytes, int64(align))
	for _, e := range entries {
		cur = zegV2AlignUp(cur, int64(align))
		e.Off = cur
		if e.LinkTarget != "" { // 链段：`size=0` ⇒ 只占对齐位、不拷载荷
			continue
		}
		in, oerr := os.Open(e.src)
		if oerr != nil {
			return fail(oerr)
		}
		if _, serr := out.Seek(cur, io.SeekStart); serr != nil {
			in.Close()
			return fail(serr)
		}
		n, cerr := io.Copy(out, in)
		in.Close()
		if cerr != nil {
			return fail(cerr)
		}
		if n != e.Size {
			return fail(fmt.Errorf("写入长度与统计不符"))
		}
		cur += n
	}
	if err := zegV2FullSync(out); err != nil { // ③ 第一刀：数据先落盘
		return fail(err)
	}
	fileLen := int64(zegV2HeaderBytes)
	for _, e := range entries {
		if end := e.Off + e.Size; end > fileLen {
			fileLen = end
		}
	}
	table, err := zegMarshalIndexVer(entries, align, now, zegV2LayoutVer)
	if err != nil {
		return fail(err)
	}
	tableOff := zegV2AlignUp(fileLen, zegV2TableAlign)
	if _, err := out.WriteAt(table, tableOff); err != nil {
		return fail(err)
	}
	if err := zegV2FullSync(out); err != nil { // ③ 第二刀：表落盘
		return fail(err)
	}
	sum := sha256.Sum256(table)
	base := zegV2Slot{Gen: 0, FileLen: fileLen, TableOff: tableOff, TableLen: int64(len(table)),
		Align: int64(align), UUID: uuid, TableSHA: sum}
	if _, err := out.WriteAt(zegV2SlotEncode(base), 0); err != nil { // 槽 0 = gen 0（基线，指向同一张表）
		return fail(err)
	}
	curSlot := base
	curSlot.Gen = 1 // ★ 槽 = gen % 2 ⇒ gen 1 落槽 1（这一刀才是提交点）
	if _, err := out.WriteAt(zegV2SlotEncode(curSlot), zegV2SlotBytes); err != nil {
		return fail(err)
	}
	if err := zegV2FullSync(out); err != nil { // ③ 第三刀：头落盘
		return fail(err)
	}
	if err := out.Close(); err != nil {
		return nil, nil, err
	}
	return entries, &curSlot, nil
}

// zegV2UpgradeReport —— 升引擎的读数（★ 验收要的「前面没被重写」证据全在这）。
type zegV2UpgradeReport struct {
	GenOld         int64
	GenNew         int64
	UUID           string
	EnginesChanged []string
	EnginesDropped []string
	BytesAppended  int64
	FirstWriteOff  int64  // 本次**唯一**写区间的起点
	NonEngineEnd   int64  // 非引擎段（权重/配方/契约）的最大末尾
	DataSHABefore  string // [8 KiB, 旧 file_len) ＝ **全部数据段**（含权重/配方/引擎）的摘要
	DataSHAAfter   string
	FileLenOld     int64
	FileLenNew     int64
	TableOffOld    int64
	TableOffNew    int64
	InodeBefore    uint64
	InodeAfter     uint64
	Changed        bool
}

// zegV2UpgradeEngine —— **升引擎只写尾部**：读旧 v2 头 → 只把「变了的引擎段」追加到旧末尾之后
// → 重写尾表 → 翻一次头槽（gen+1）。★ 非引擎段必须逐字不变（变了 ⇒ 拒，让调用方整件重打）。
// ★ 证据面：`FirstWriteOff > NonEngineEnd`（写区间起点在所有权重/配方段之后）＋
//
//	`DataSHABefore == DataSHAAfter`（[8 KiB, 旧 file_len) **全部数据段**逐字节同；头不在这一区 ——
//	头本来就要翻，那正是提交点）＋ inode 不变（原地写、没换件）。
func zegV2UpgradeEngine(egg, src, now string) (*zegV2UpgradeReport, error) {
	f, err := os.OpenFile(egg, os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	fi0, err := f.Stat()
	if err != nil {
		return nil, err
	}
	head := make([]byte, 8)
	if _, err := f.ReadAt(head, 0); err != nil {
		return nil, fmt.Errorf("卵太短，读不满 8 字节头：%v", err)
	}
	if string(head[0:4]) != zegMagic {
		return nil, zegTierRefuse("D1", "升引擎入口实读前 4B", hex.EncodeToString(head[0:4]))
	}
	if ver := binary.LittleEndian.Uint32(head[4:8]); ver != zegV2LayoutVer {
		return nil, fmt.Errorf("升引擎只吃 v2 卵（本件 version=%d）—— v1 请先整件重打成 v2（✗ 按旧偏移原地改）", ver)
	}
	slot, err := zegV2PickSlot(f)
	if err != nil {
		return nil, err
	}
	oldIdx, err := zegParseIndex(egg)
	if err != nil {
		return nil, err
	}
	rep := &zegV2UpgradeReport{
		GenOld: slot.Gen, GenNew: slot.Gen, UUID: hex.EncodeToString(slot.UUID[:]),
		FileLenOld: slot.FileLen, FileLenNew: slot.FileLen, TableOffOld: slot.TableOff,
		InodeBefore: zegV2Inode(fi0),
	}
	oldByPath := make(map[string]zegEntryJSON, len(oldIdx.Entries))
	for _, e := range oldIdx.Entries {
		oldByPath[e.Path] = e
	}
	fresh, err := zegCollectEntries(src)
	if err != nil {
		return nil, err
	}
	if len(fresh) == 0 {
		return nil, fmt.Errorf("源目录里没有文件：%s", src)
	}
	freshByPath := make(map[string]*zegEntry, len(fresh))
	for _, e := range fresh {
		freshByPath[e.Path] = e
	}
	var appends []*zegEntry
	for _, e := range fresh {
		old, ok := oldByPath[e.Path]
		if zegSegOfPath(e.Path) != zegSegEngine {
			if !ok {
				return nil, fmt.Errorf("升引擎只许改引擎段：源目录新增了非引擎段 %q（旧段表里没有）", e.Path)
			}
			if old.SHA256 != e.SHA256 || old.Size != e.Size || old.Mode != e.Mode || old.LinkTarget != e.LinkTarget {
				return nil, fmt.Errorf("升引擎只许改引擎段：非引擎段 %q 的内容/权限变了（旧 sha256=%s）⇒ 拒原地升级，请整件重打",
					e.Path, zegShortSHA(old.SHA256))
			}
			e.Off = old.Off // 偏移照旧 ⇒ 那位字节不会被写
			continue
		}
		if ok && old.SHA256 == e.SHA256 && old.Size == e.Size && old.Mode == e.Mode && old.LinkTarget == e.LinkTarget {
			e.Off = old.Off // 引擎段没变 ⇒ 复用旧偏移（不重写）
			continue
		}
		appends = append(appends, e)
		rep.EnginesChanged = append(rep.EnginesChanged, e.Path)
	}
	for _, old := range oldIdx.Entries {
		if _, ok := freshByPath[old.Path]; ok {
			continue
		}
		if zegSegOfPath(old.Path) != zegSegEngine {
			return nil, fmt.Errorf("升引擎只许改引擎段：非引擎段 %q 在源目录里没了 ⇒ 拒原地升级", old.Path)
		}
		rep.EnginesDropped = append(rep.EnginesDropped, old.Path)
	}
	// 段序：非引擎段照**旧表序**（偏移不动）· 引擎段按路径序排在最后。
	ordered := make([]*zegEntry, 0, len(fresh))
	for _, old := range oldIdx.Entries {
		if zegSegOfPath(old.Path) == zegSegEngine {
			continue
		}
		if e, ok := freshByPath[old.Path]; ok {
			ordered = append(ordered, e)
		}
	}
	engines := make([]*zegEntry, 0, len(fresh))
	for _, e := range fresh {
		if zegSegOfPath(e.Path) == zegSegEngine {
			engines = append(engines, e)
		}
	}
	ordered = append(ordered, engines...)
	for _, e := range oldIdx.Entries {
		if zegSegOfPath(e.Path) != zegSegEngine {
			if end := e.Off + e.Size; end > rep.NonEngineEnd {
				rep.NonEngineEnd = end
			}
		}
	}
	if len(appends) == 0 && len(rep.EnginesDropped) == 0 {
		rep.Changed = false // 没变化 ⇒ 不动（gen 不增、零字节写）
		return rep, nil
	}
	// ★ 证据面①：数据区 [8 KiB, 旧 file_len) 的摘要 —— **头不属于这一区**（头本来就要翻，那正是提交点）。
	before, err := zegV2ShaRange(f, zegV2HeaderBytes, slot.FileLen-zegV2HeaderBytes)
	if err != nil {
		return nil, err
	}
	rep.DataSHABefore = before
	// 追加区起点：**旧表之后**的第一个对齐界（✗ 不覆盖旧表 —— 它是崩前的唯一提交记录）。
	base := zegV2AlignUp(slot.TableOff+slot.TableLen, slot.Align)
	if base < slot.TableOff+slot.TableLen {
		base = slot.TableOff + slot.TableLen
	}
	rep.FirstWriteOff = base
	cur := base
	for _, e := range appends {
		cur = zegV2AlignUp(cur, slot.Align)
		e.Off = cur
		if e.LinkTarget != "" {
			continue
		}
		in, oerr := os.Open(e.src)
		if oerr != nil {
			return nil, oerr
		}
		if _, serr := f.Seek(cur, io.SeekStart); serr != nil {
			in.Close()
			return nil, serr
		}
		n, cerr := io.Copy(f, in)
		in.Close()
		if cerr != nil {
			return nil, cerr
		}
		if n != e.Size {
			return nil, fmt.Errorf("写入长度与统计不符：%s", e.Path)
		}
		cur += n
		rep.BytesAppended += n
	}
	if err := zegV2FullSync(f); err != nil { // ③ 第一刀：新引擎段落盘
		return nil, err
	}
	fileLenNew := int64(zegV2HeaderBytes)
	for _, e := range ordered {
		if end := e.Off + e.Size; end > fileLenNew {
			fileLenNew = end
		}
	}
	rep.FileLenNew = fileLenNew
	table, err := zegMarshalIndexVer(ordered, int(slot.Align), now, zegV2LayoutVer)
	if err != nil {
		return nil, err
	}
	tableOffNew := zegV2AlignUp(fileLenNew, zegV2TableAlign)
	rep.TableOffNew = tableOffNew
	if _, err := f.WriteAt(table, tableOffNew); err != nil {
		return nil, err
	}
	if err := f.Truncate(tableOffNew + int64(len(table))); err != nil {
		return nil, err
	}
	if err := zegV2FullSync(f); err != nil { // ③ 第二刀：表落盘
		return nil, err
	}
	sum := sha256.Sum256(table)
	next := zegV2Slot{
		Gen: slot.Gen + 1, FileLen: fileLenNew, TableOff: tableOffNew, TableLen: int64(len(table)),
		Align: slot.Align, UUID: slot.UUID, TableSHA: sum,
	}
	if _, err := f.WriteAt(zegV2SlotEncode(next), int64(next.Gen%2)*zegV2SlotBytes); err != nil {
		return nil, err
	}
	if err := zegV2FullSync(f); err != nil { // ③ 第三刀：头（唯一提交点）
		return nil, err
	}
	after, err := zegV2ShaRange(f, zegV2HeaderBytes, slot.FileLen-zegV2HeaderBytes)
	if err != nil {
		return nil, err
	}
	rep.DataSHAAfter = after
	rep.GenNew = next.Gen
	rep.Changed = true
	if fi1, ferr := f.Stat(); ferr == nil {
		rep.InodeAfter = zegV2Inode(fi1)
	}
	return rep, nil
}

// zegShortSHA —— sha256 前 16 位（只给人看；✗ 不参与判据）。
func zegShortSHA(s string) string {
	if len(s) > 16 {
		return s[:16]
	}
	return s
}

// zegMarshalIndex —— 生成索引 JSON 字节（**与 py `json.dumps` 同源**：紧凑 + 不转义非 ASCII + 不转义 HTML）。
func zegMarshalIndex(entries []*zegEntry, align int, now string) ([]byte, error) {
	// ★ v1 写端（历史契约 · golden 与向后兼容夹具都靠它）：主号恒 1，✗ 别改成 zegFormatVer。
	return zegMarshalIndexVer(entries, align, now, zegFormatVerV1)
}

// zegMarshalIndexVer —— 同一份索引 JSON 形状，主号由调用方给（v1 写端 = 1 / v2 写端 = 2）。
// ★ 键集合与字段序**一字不动**（段表 v1/v2 共用同一份形状 ⇒ 旧读端的可加面判据不用改）。
func zegMarshalIndexVer(entries []*zegEntry, align int, now string, ver int) ([]byte, error) {
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
		Version:   ver,
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

// zegParseIndex —— 解析索引（**唯一读端**：v1 与 v2 都在这里分派，✗ 不开第二份解析器）。
// ★ 分派只按固定头的前 8 B：`magic` ＋ `version`；主号闸先跑（D2 前向拒 / D3 过旧拒），再按布局走。
func zegParseIndex(path string) (*zegIndex, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	hdr := make([]byte, zegHeaderPrefix)
	if n, err := io.ReadFull(f, hdr); err != nil {
		// ★ D1（GAP-20261015-24）：前缀不足 32B 也是「不是卵」那一档 —— 报话与退码都取 13 档表。
		k := n
		if k > 4 {
			k = 4
		}
		return nil, zegTierRefuse("D1",
			fmt.Sprintf("固定前缀不足 %d 字节（实读 %d 字节）：%v", zegHeaderPrefix, n, err),
			hex.EncodeToString(hdr[:k]))
	}
	magic := hdr[0:4]
	ver := int(binary.LittleEndian.Uint32(hdr[4:8]))
	if string(magic) != zegMagic {
		return nil, zegTierRefuse("D1", fmt.Sprintf("前 4B 实读 %q", magic), hex.EncodeToString(magic))
	}
	// ★ 卵6：**主号闸**（前向拒 D2；过旧拒 D3）—— 两种布局共用同一道闸（✗ 各写一份）。
	if err := zegCompatGateMainVersion(ver, zegVersionSitePrefix); err != nil {
		return nil, err
	}
	switch ver {
	case zegFormatVerV1:
		return zegParseIndexV1(f, hdr)
	case zegV2LayoutVer:
		return zegV2ParseIndex(f)
	}
	return nil, fmt.Errorf("格式版本不支持：%d", ver)
}

// zegParseIndexV1 —— **v1 布局**的读端（历史形态逐字保留：32 B 前缀 ＋ 变长索引 ＋ 按路径序的段）。
// ★ `zegPack`（v1 写端）与盘上所有既有 `.egg` 走这一条 ⇒ ✗ 不许按 v2 的偏移去读它。
func zegParseIndexV1(f *os.File, hdr []byte) (*zegIndex, error) {
	align := int64(binary.LittleEndian.Uint64(hdr[8:16]))
	n := int64(binary.LittleEndian.Uint64(hdr[16:24]))
	if !zegIsPow2(align) || align < zegMinAlign {
		return nil, zegTierRefuse("D4", fmt.Sprintf("前缀里的 align=%d（须 2 的幂且 ≥%d）", align, zegMinAlign))
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
		return nil, zegTierRefuse("D13", err.Error())
	}
	if raw.Magic != zegMagic {
		return nil, zegTierRefuse("D1", fmt.Sprintf("索引体里的 magic=%q", raw.Magic),
			hex.EncodeToString([]byte(raw.Magic)))
	}
	// ★ 前缀与索引体的主号必须**同一个**（v1 档 = 1）；✗ 别拿本端主号去比 —— 那是 v2 的号。
	if raw.Version != zegFormatVerV1 {
		return nil, fmt.Errorf("格式版本不支持：%d", raw.Version)
	}
	if !zegIsPow2(raw.Align) || raw.Align < zegMinAlign {
		return nil, zegTierRefuse("D4", fmt.Sprintf("索引体里的 align=%d（须 2 的幂且 ≥%d）", raw.Align, zegMinAlign))
	}
	idx := &zegIndex{
		Magic:     raw.Magic,
		Version:   raw.Version,
		Align:     raw.Align,
		Tool:      raw.Tool,
		CreatedAt: raw.CreatedAt,
		Entries:   raw.Entries,
		Layout:    zegFormatVerV1,
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
		if !truncated {
			if got := hex.EncodeToString(h.Sum(nil)); got != e.SHA256 {
				problems = append(problems, zegSHAMismatchProblem(e.Path, e.SHA256, got))
			}
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

// zegEggErrExit —— 读端错误 ↦ 卵族命令面退码：**带码拒**（`GAP-20261015-24` 的 D1/D4/D13 ＋
// `GAP-20261015-25` 的 D2/D9，码取自 13 档表同一行）走表里的码；其余错误仍走既有 `exitFail`（=1）通道
// （✗ 不扩面改其余档的退码 —— 例如 D7 的拍定码本来就是 1，D3/D5/D8/D10–D12 仍未接）。
func zegEggErrExit(err error) int {
	if code, ok := zegRefusalExit(err); ok {
		return code
	}
	return exitFail
}

// zegEggErrLine —— 打一条读端错误：**报话一行**（逐字 ＝ 13 档表 `Msg` 列）＋ **细节一行**（有才打）。
// ★ 两行分工是为了「报话可逐字对表」而诊断读数不丢：细节（实读字节 / 非法值 / 原生解码错）不进报话。
func zegEggErrLine(w io.Writer, err error) {
	fmt.Fprintf(w, "%s: %v\n", zegProg, err)
	if d := zegRefusalDetail(err); d != "" {
		fmt.Fprintf(w, "  细节：%s\n", d)
	}
}

func zegCmdInfo(stdout io.Writer, path string) int {
	idx, err := zegParseIndex(path)
	if err != nil {
		zegEggErrLine(stdout, err)
		return zegEggErrExit(err)
	}
	fi, _ := os.Stat(path)
	fmt.Fprintf(stdout, "卵文件：%s\n", path)
	fmt.Fprintf(stdout, "  magic=%s version=%d align=%d（%d KiB）\n", idx.Magic, idx.Version, idx.Align, idx.Align/1024)
	fmt.Fprintf(stdout, "  产出工具=%s %s  产出时间=%s\n", idx.Tool.Name, idx.Tool.Version, idx.CreatedAt)
	fmt.Fprintf(stdout, "  段数=%d  文件大小=%.1f KiB\n", len(idx.Entries), float64(fi.Size())/1024)
	// ★ v2 面（设计稿 Q3）：版本/代际/卵唯一号/尾表落点/引擎段是否在尾 —— 逐条能读出才算数。
	if idx.Layout == zegV2LayoutVer {
		last := ""
		if n := len(idx.Entries); n > 0 {
			last = idx.Entries[n-1].Path
		}
		atTail := "否"
		if zegV2EngineAtTail(idx.Entries) {
			atTail = "是"
		}
		fmt.Fprintf(stdout, "  ★ 布局=v2（段表化）  gen=%d  uuid=%s\n", idx.Gen, idx.UUID)
		fmt.Fprintf(stdout, "  段表=%d..%d（%d B · file_len=%d · 64 KiB 对齐落点）  table_sha256=%s\n",
			idx.TableOff, idx.TableOff+idx.TableLen, idx.TableLen, idx.FileLen, zegShortSHA(idx.TableSHA))
		fmt.Fprintf(stdout, "  引擎段在尾=%s（末段=%s）\n", atTail, last)
	}
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
		// ★ D9（GAP-20261015-25）：规则名从报话移进 `Detail`（报话逐字 ＝ 13 档表 D9 行）⇒ 判据点细节行，
		// 但「拒执必须点名规则」这条口径不变。
		if !ok || ref.Tier != "D9" || !strings.Contains(zegRefusalDetail(perr), rule) {
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

// zegV2SelfTest —— v2 布局自检（**六态 ＋ 两负控**）：打完能读回 · 双槽 crc 有效 · gen 单调递增 ·
// 升引擎只写尾部（前缀 sha 前后一致 ＋ 写区间起点在非引擎段之后 ＋ inode 不变）· 三级链完整 ·
// 负控 A「改权重段 ⇒ 升级必拒」· 负控 B「v1 卵 ⇒ `--update` 必拒（只吃 v2）」。
func zegV2SelfTest(stdout, stderr io.Writer) int {
	t, err := os.MkdirTemp("", "zerg-egg-v2-selftest")
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", zegProg, err)
		return exitFail
	}
	defer os.RemoveAll(t) // 自回收（TMPDIR 前后 delta=0）

	src := filepath.Join(t, "tree")
	for _, d := range []string{"model", "engine", "sysroot/lib"} {
		if err := os.MkdirAll(filepath.Join(src, filepath.FromSlash(d)), 0o755); err != nil {
			return zegSelfFail(stderr, err)
		}
	}
	// 1 MiB 假权重（真 GGUF magic）—— 验收里「权重段前后 sha 一致」的那一枚。
	weight := make([]byte, 1<<20)
	copy(weight, []byte("GGUF"))
	if _, err := rand.Read(weight[4:]); err != nil {
		return zegSelfFail(stderr, err)
	}
	if err := os.WriteFile(filepath.Join(src, "model", "w.gguf"), weight, 0o644); err != nil {
		return zegSelfFail(stderr, err)
	}
	if err := os.WriteFile(filepath.Join(src, "config.json"), []byte(`{"arch":"v2"}`), 0o644); err != nil {
		return zegSelfFail(stderr, err)
	}
	if err := os.WriteFile(filepath.Join(src, "sysroot", "lib", "libc.so.6"), []byte("stub"), 0o644); err != nil {
		return zegSelfFail(stderr, err)
	}
	enginePath := filepath.Join(src, "engine", "zerg-engine")
	if err := os.WriteFile(enginePath, []byte("#!/bin/sh\necho v1\n"), 0o755); err != nil {
		return zegSelfFail(stderr, err)
	}
	if err := os.WriteFile(filepath.Join(src, "engine", "libfoo.so.1.12"), []byte("ELF"), 0o644); err != nil {
		return zegSelfFail(stderr, err)
	}
	if err := os.Symlink("libfoo.so.1.12", filepath.Join(src, "engine", "libfoo.so.1")); err != nil {
		return zegSelfFail(stderr, err)
	}
	if err := os.Symlink("libfoo.so.1", filepath.Join(src, "engine", "libfoo.so")); err != nil {
		return zegSelfFail(stderr, err)
	}
	egg := filepath.Join(t, "v2.egg")
	entries, slot, err := zegPackV2(src, egg, 4096, "2000-01-01T00:00:00Z")
	if err != nil {
		return zegSelfFail(stderr, err)
	}
	weightSHA0, err := zegSha256File(filepath.Join(src, "model", "w.gguf"))
	if err != nil {
		return zegSelfFail(stderr, err)
	}

	// ---- 态①：打完能读回（版本 / 段数 / 代际 / 卵唯一号 / 引擎段在尾）----
	idx, err := zegParseIndex(egg)
	if err != nil {
		return zegSelfFail(stderr, fmt.Errorf("态① v2 卵读不回：%v", err))
	}
	if idx.Layout != zegV2LayoutVer || idx.Version != zegV2LayoutVer {
		return zegSelfFail(stderr, fmt.Errorf("态① 读回的布局/主号=%d/%d（要 2/2）", idx.Layout, idx.Version))
	}
	if idx.Gen != 1 || slot.Gen != 1 || len(idx.UUID) != 32 {
		return zegSelfFail(stderr, fmt.Errorf("态① gen=%d（要 1）· uuid=%q（要 32 hex）", idx.Gen, idx.UUID))
	}
	if len(idx.Entries) != len(entries) || len(idx.Entries) != 7 {
		return zegSelfFail(stderr, fmt.Errorf("态① 段数 写端 %d ⟷ 读端 %d（夹具要 7 段）", len(entries), len(idx.Entries)))
	}
	if !zegV2EngineAtTail(idx.Entries) {
		return zegSelfFail(stderr, fmt.Errorf("态① 引擎段不在尾：%v", zegV2SegOrder(idx.Entries)))
	}
	if want := "engine/zerg-engine"; idx.Entries[len(idx.Entries)-1].Path != want {
		return zegSelfFail(stderr, fmt.Errorf("态① 末段=%q（引擎段按路径序排在最后 ⇒ 要 %q）",
			idx.Entries[len(idx.Entries)-1].Path, want))
	}

	// ---- 态②：header 双槽 crc 有效（两槽都能解出来，且 gen 一大一小）----
	both, gens, err := zegV2ReadBothSlots(egg)
	if err != nil {
		return zegSelfFail(stderr, err)
	}
	if !both {
		return zegSelfFail(stderr, fmt.Errorf("态② header 双槽不是都 crc 有效"))
	}
	if !(gens[1] > gens[0]) {
		return zegSelfFail(stderr, fmt.Errorf("态② 两槽 gen 不单调（槽0=%d 槽1=%d）", gens[0], gens[1]))
	}

	// ---- 态③④：升引擎（改一个只属引擎段的小文件）⇒ gen 单调递增 ＋ 只写尾部 ----
	if err := os.WriteFile(enginePath, []byte("#!/bin/sh\necho v2\n"), 0o755); err != nil {
		return zegSelfFail(stderr, err)
	}
	if err := os.Chmod(enginePath, 0o755); err != nil {
		return zegSelfFail(stderr, err)
	}
	rep1, err := zegV2UpgradeEngine(egg, src, "2000-01-01T00:01:00Z")
	if err != nil {
		return zegSelfFail(stderr, fmt.Errorf("态③ 升引擎失败：%v", err))
	}
	if !rep1.Changed || rep1.GenNew != rep1.GenOld+1 || rep1.GenNew != 2 {
		return zegSelfFail(stderr, fmt.Errorf("态③ gen 不是 +1 单调（%d → %d · changed=%v）",
			rep1.GenOld, rep1.GenNew, rep1.Changed))
	}
	if !(rep1.FirstWriteOff > rep1.NonEngineEnd) {
		return zegSelfFail(stderr, fmt.Errorf("态④ 写区间起点 %d 没越过非引擎段末尾 %d（前面可能被重写）",
			rep1.FirstWriteOff, rep1.NonEngineEnd))
	}
	if rep1.DataSHABefore != rep1.DataSHAAfter {
		return zegSelfFail(stderr, fmt.Errorf("态④ 数据区 sha 变了（%s → %s）⇒ 前面被重写",
			zegShortSHA(rep1.DataSHABefore), zegShortSHA(rep1.DataSHAAfter)))
	}
	if rep1.InodeBefore != rep1.InodeAfter || rep1.InodeBefore == 0 {
		return zegSelfFail(stderr, fmt.Errorf("态④ inode 变了（%d → %d）⇒ 不是原地追加",
			rep1.InodeBefore, rep1.InodeAfter))
	}
	idx1, err := zegParseIndex(egg)
	if err != nil {
		return zegSelfFail(stderr, err)
	}
	if idx1.Gen != 2 {
		return zegSelfFail(stderr, fmt.Errorf("态③ 升级后读回 gen=%d（要 2）", idx1.Gen))
	}
	// 再升一次（同一文件再改）⇒ gen 3（★ 两槽交替：2 落槽 0、3 落槽 1）
	if err := os.WriteFile(enginePath, []byte("#!/bin/sh\necho v3\n"), 0o755); err != nil {
		return zegSelfFail(stderr, err)
	}
	rep2, err := zegV2UpgradeEngine(egg, src, "2000-01-01T00:02:00Z")
	if err != nil {
		return zegSelfFail(stderr, err)
	}
	if rep2.GenNew != 3 {
		return zegSelfFail(stderr, fmt.Errorf("态③ 第二次升级 gen=%d（要 3 ⇒ 单调递增）", rep2.GenNew))
	}
	idx2, err := zegParseIndex(egg)
	if err != nil || idx2.Gen != 3 {
		return zegSelfFail(stderr, fmt.Errorf("态③ 第二次升级后读回失败或 gen 不对：err=%v gen=%d", err, idx2.Gen))
	}

	// ---- 态⑤：升级后整件仍自洽（verify 0 problem ＋ 重建的引擎内容 == 新内容 ＋ 权重段 sha 不变）----
	probs, _, err := zegVerify(egg, filepath.Join(t, "rebuilt"))
	if err != nil || len(probs) != 0 {
		return zegSelfFail(stderr, fmt.Errorf("态⑤ 升级后 verify 报问题：err=%v probs=%v", err, probs))
	}
	got, err := os.ReadFile(filepath.Join(t, "rebuilt", "engine", "zerg-engine"))
	if err != nil {
		return zegSelfFail(stderr, err)
	}
	if string(got) != "#!/bin/sh\necho v3\n" {
		return zegSelfFail(stderr, fmt.Errorf("态⑤ 重建后的引擎内容不是新内容：%q", string(got)))
	}
	weightSHA1, err := zegSha256File(filepath.Join(t, "rebuilt", "model", "w.gguf"))
	if err != nil {
		return zegSelfFail(stderr, err)
	}
	if weightSHA0 != weightSHA1 {
		return zegSelfFail(stderr, fmt.Errorf("态⑤ 权重段 sha 前后不一致（%s → %s）",
			zegShortSHA(weightSHA0), zegShortSHA(weightSHA1)))
	}

	// ---- 态⑥：三级链在 v2 下仍逐字完好（重建后 readlink 逐字同）----
	for _, c := range []struct{ path, want string }{
		{"engine/libfoo.so", "libfoo.so.1"},
		{"engine/libfoo.so.1", "libfoo.so.1.12"},
	} {
		full := filepath.Join(t, "rebuilt", "engine", filepath.Base(c.path))
		li, lerr := os.Lstat(full)
		if lerr != nil || li.Mode()&os.ModeSymlink == 0 {
			return zegSelfFail(stderr, fmt.Errorf("态⑥ 重建后 %s 不是符号链接（err=%v）", c.path, lerr))
		}
		if link, rerr := os.Readlink(full); rerr != nil || link != c.want {
			return zegSelfFail(stderr, fmt.Errorf("态⑥ 重建后 %s 的链名 %q ≠ 原件 %q（err=%v）",
				c.path, link, c.want, rerr))
		}
	}

	// ---- 负控 A：改**权重段** ⇒ 升级必拒（且件一个字节不动）----
	before, err := os.ReadFile(egg)
	if err != nil {
		return zegSelfFail(stderr, err)
	}
	if err := os.WriteFile(filepath.Join(src, "model", "w.gguf"), append(weight[:len(weight)-1:len(weight)-1], 0x00), 0o644); err != nil {
		return zegSelfFail(stderr, err)
	}
	if _, uerr := zegV2UpgradeEngine(egg, src, ""); uerr == nil {
		return zegSelfFail(stderr, fmt.Errorf("负控 A 改了权重段竟被原地升级（✗ 必须拒）"))
	} else if !strings.Contains(uerr.Error(), "非引擎段") {
		return zegSelfFail(stderr, fmt.Errorf("负控 A 拒了但没点名「非引擎段」：%v", uerr))
	}
	after, err := os.ReadFile(egg)
	if err != nil {
		return zegSelfFail(stderr, err)
	}
	if string(before) != string(after) {
		return zegSelfFail(stderr, fmt.Errorf("负控 A 拒执却动了件（%d → %d B）", len(before), len(after)))
	}

	// ---- 负控 B：v1 卵走 `--update` ⇒ 必拒（只吃 v2）----
	v1egg := filepath.Join(t, "v1.egg")
	if _, perr := zegPack(src, v1egg, 4096, "2000-01-01T00:00:00Z"); perr != nil {
		return zegSelfFail(stderr, perr)
	}
	if _, uerr := zegV2UpgradeEngine(v1egg, src, ""); uerr == nil {
		return zegSelfFail(stderr, fmt.Errorf("负控 B v1 卵竟被原地升级（✗ 只吃 v2）"))
	} else if !strings.Contains(uerr.Error(), "只吃 v2") {
		return zegSelfFail(stderr, fmt.Errorf("负控 B 拒了但没点名「只吃 v2」：%v", uerr))
	}

	fmt.Fprintf(stdout, "self-test（卵 v2 布局）通过（6 态 ＋ 2 负控）：① v2 打完能读回（主号 2 · %d 段 · gen=1 · uuid 32 hex · 引擎段在尾）· ② header 双槽 crc 有效且 gen 单调（槽0=%d < 槽1=%d）· ③ 连升两次 gen 1→2→3 单调递增 · ④ 升引擎只写尾部（写区间起点 %d > 非引擎段末尾 %d · 数据区 sha 前后一致 · inode 不变）· ⑤ 升级后 verify 0 problem 且重建内容/权重 sha 不变 · ⑥ 三级相对链重建后 readlink 逐字同 · 负控 A 改权重段 ⇒ 拒「非引擎段」且件零字节变动 · 负控 B v1 卵 ⇒ 拒「只吃 v2」\n",
		len(idx.Entries), gens[0], gens[1], rep1.FirstWriteOff, rep1.NonEngineEnd)
	return exitOK
}

// zegV2ReadBothSlots —— 自检用：两槽都解得开才算 crc 有效；返回 (都有效, [槽0 gen, 槽1 gen])。
func zegV2ReadBothSlots(path string) (bool, [2]int64, error) {
	var gens [2]int64
	f, err := os.Open(path)
	if err != nil {
		return false, gens, err
	}
	defer f.Close()
	head := make([]byte, zegV2HeaderBytes)
	if _, err := io.ReadFull(f, head); err != nil {
		return false, gens, err
	}
	s0, ok0 := zegV2SlotDecode(head[0:zegV2SlotBytes], 0)
	s1, ok1 := zegV2SlotDecode(head[zegV2SlotBytes:2*zegV2SlotBytes], 1)
	gens[0], gens[1] = s0.Gen, s1.Gen
	return ok0 && ok1, gens, nil
}

// zegV2SegOrder —— 自检报话用：段序的「段分类」串（只给人看）。
func zegV2SegOrder(entries []zegEntryJSON) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Path+"|"+zegSegOfPath(e.Path))
	}
	return out
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
	// update —— ★ 升引擎的**内部路径**（隐藏取旗标 · 照 `--now`/`--workers` 同款：✗ 不进帮助面）：
	// `pack <源目录> <既有 v2 卵> --update` ⇒ 只把变了的**引擎段**追加到旧末尾之后 ＋ 翻一次头槽。
	update bool
	// workers —— 卵3① 的**隐藏取旗标**（`--workers N` · 照 `--now` 的同款：✗ 不进帮助面 ⇒ 不污染 `-h` 对拍）。
	// 0 ＝ 自动档（并发度 `W = clamp(ceil(D/h), 1, NumCPU)` · `h` 由内存探针现测）。
	workers int
	// engineDirs —— ★ 卵沙箱第 3 步：打蛋时**一起收进蛋**的引擎目录（可给多次 ⇒ 蛋内 `engine/<基名>/…`）。
	// 只对 `pack` 有效；收谁由调用方（构建脚本）决定，打蛋端只做**只读闭包自检**（见 zerg_egg_closure.go）。
	engineDirs []string
	// libDirs —— ★ 卵沙箱第 6 步：打蛋时**一起收进蛋**的库目录（可给多次 ⇒ 蛋内 `libs/<基名>/…`）。
	// ★ 为什么需要它：从蛋孵化时 backend 把 `<蛋>/libs` 整棵只读落到空间 `/libs`，而**库里那些
	// 件此前没有任何入口进蛋**（`--engine-dir` 只落 `engine/`）⇒ 真蛋被「具名拒孵」。语义与
	// `--engine-dir` **逐条同源**（只收目录 · 撞名即拒 · 闭包缺件即失败 · mode 按安装语义归一）。
	libDirs []string
}

var zegOpts = []string{"--align", "--engine-dir", "--libs-dir", "--rebuild-into", "--self-test", "--chunk-size", "--help", "--now", "--workers", "--update"}

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
			case "--update":
				a.update = true
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
			case "--engine-dir":
				v, ok := take()
				if !ok {
					return nil, fmt.Sprintf("argument %s: expected one argument", name)
				}
				a.engineDirs = append(a.engineDirs, v)
			case "--libs-dir":
				v, ok := take()
				if !ok {
					return nil, fmt.Sprintf("argument %s: expected one argument", name)
				}
				a.libDirs = append(a.libDirs, v)
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
		if rc := zegFormatSelfTest(stdout, stderr); rc != exitOK {
			return rc
		}
		// ★ 第五行：v2 布局（段表化 ＋ 引擎段在尾）—— ✗ 与前四行混行（前四行契约读数一字不动）。
		if rc := zegV2SelfTest(stdout, stderr); rc != exitOK {
			return rc
		}
		// ★ 第六行：引擎闭包 · sysroot（`--engine-dir` ＋ 缺件即拒）—— ✗ 与前五行混行。
		return zegClosureSelfTest(stdout, stderr)
	}
	if len(a.engineDirs) > 0 && a.cmd != "pack" {
		fmt.Fprintf(stderr, "%s: `--engine-dir` 只对 `pack` 有效（收到子命令 %q ⇒ ✗ 静默忽略）\n", zegProg, a.cmd)
		return exitUsage
	}
	if len(a.libDirs) > 0 && a.cmd != "pack" {
		fmt.Fprintf(stderr, "%s: `--libs-dir` 只对 `pack` 有效（收到子命令 %q ⇒ ✗ 静默忽略）\n", zegProg, a.cmd)
		return exitUsage
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
		if a.update && len(a.engineDirs) > 0 {
			fmt.Fprintf(stderr, "%s: `--engine-dir` 与 `--update` 互斥（升引擎只**追加引擎段**、不收目录；要换闭包请重打一枚蛋）\n", zegProg)
			return zegExitToolUsage
		}
		if a.update && len(a.libDirs) > 0 {
			fmt.Fprintf(stderr, "%s: `--libs-dir` 与 `--update` 互斥（升引擎只**追加引擎段**、不收目录；要换闭包请重打一枚蛋）\n", zegProg)
			return zegExitToolUsage
		}
		if a.update {
			// ★ 升引擎只写尾部（设计稿 Q3 的核心）：证据面三条一起印 —— 写区间 / 前缀 sha / inode。
			rep, uerr := zegV2UpgradeEngine(a.b, a.a, a.nowVal)
			if uerr != nil {
				zegEggErrLine(stderr, uerr)
				return zegEggErrExit(uerr)
			}
			if !rep.Changed {
				fmt.Fprintf(stdout, "升引擎：无变化 ⇒ 不动（gen=%d 不增 · 零字节写）\n", rep.GenOld)
				return exitOK
			}
			fmt.Fprintf(stdout, "升引擎完成（原地追加 · 只写尾部）：gen %d → %d · 引擎段新写 %d 段 %d B",
				rep.GenOld, rep.GenNew, len(rep.EnginesChanged), rep.BytesAppended)
			if len(rep.EnginesDropped) > 0 {
				fmt.Fprintf(stdout, " · 弃用旧引擎段 %d 段", len(rep.EnginesDropped))
			}
			fmt.Fprintf(stdout, "\n")
			fmt.Fprintf(stdout, "  写区间 = [%d, EOF) · 非引擎段（权重/配方/契约）末尾 = %d ⇒ 起点 > 末尾 = %v\n",
				rep.FirstWriteOff, rep.NonEngineEnd, rep.FirstWriteOff > rep.NonEngineEnd)
			fmt.Fprintf(stdout, "  数据区 [%d, %d) 的 sha256 前后一致 = %v（%s）· inode %d → %d\n",
				zegV2HeaderBytes, rep.FileLenOld, rep.DataSHABefore == rep.DataSHAAfter, zegShortSHA(rep.DataSHAAfter),
				rep.InodeBefore, rep.InodeAfter)
			fmt.Fprintf(stdout, "  段表 %d → %d（file_len %d → %d · uuid=%s）\n",
				rep.TableOffOld, rep.TableOffNew, rep.FileLenOld, rep.FileLenNew, rep.UUID)
			return exitOK
		}
		entries, slot, err := zegPackV2Tree(a.a, a.b, a.align, a.nowVal, a.engineDirs, a.libDirs)
		if err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", zegProg, err)
			return exitFail
		}
		fi, _ := os.Stat(a.b)
		fmt.Fprintf(stdout, "已打包 %d 个文件 → %s（align=%d，%.1f KiB）\n", len(entries), a.b, eff, float64(fi.Size())/1024)
		engines := 0
		for _, e := range entries {
			if zegSegOfPath(e.Path) == zegSegEngine {
				engines++
			}
		}
		fmt.Fprintf(stdout, "  ★ v2 布局：gen=%d · uuid=%s · 引擎段 %d 段在尾（%d 段）· 段表@%d · 段表 sha256=%s\n",
			slot.Gen, hex.EncodeToString(slot.UUID[:]), engines, len(entries), slot.TableOff, zegShortSHA(hex.EncodeToString(slot.TableSHA[:])))
		// ★ 闭包自检的**过档读数**（有动态件才印；缺件这条路走不到这里 —— 已在写盘前失败）。
		if rep, rerr := zegClosureAudit(entries); rerr == nil && rep.Dynamic > 0 {
			fmt.Fprintf(stdout, "  ★ %s\n", rep.zegClosureSummary())
		}
		if len(a.engineDirs) > 0 {
			// ★ 归一**逐件机检**（✗ 不静默改写）：数与来源 mode 不符的引擎段，人面明说。
			norm := 0
			for _, e := range entries {
				if e.src == "" || !strings.HasPrefix(e.Path, zegEngineSegRoot+"/") {
					continue
				}
				if fi, serr := os.Stat(e.src); serr == nil && int(fi.Mode().Perm()) != e.Mode {
					norm++
				}
			}
			fmt.Fprintf(stdout, "  引擎目录 %d 个已收进蛋（落点 engine/<基名>/… · 段分类恒「引擎段」）· mode 归一 %d 件（安装语义 0755/0644 · 安全子集不变）\n",
				len(a.engineDirs), norm)
		}
		if len(a.libDirs) > 0 {
			// ★ 库目录：落点 `libs/<基名>/…`（backend 从蛋孵化时整棵只读落到空间 /libs）。
			fmt.Fprintf(stdout, "  库目录 %d 个已收进蛋（落点 libs/<基名>/… · 从蛋孵化时整棵落到空间 /libs）\n",
				len(a.libDirs))
		}
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
			zegEggErrLine(stderr, err)
			return zegEggErrExit(err)
		}
		probs, _, verr := zegVerify(a.b, "")
		if verr != nil {
			zegEggErrLine(stderr, verr)
			return zegEggErrExit(verr)
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
			zegEggErrLine(stderr, err)
			return zegEggErrExit(err)
		}
		if len(res.Problems) > 0 {
			fmt.Fprintf(stdout, "✗ 校验失败 %d 项：\n", len(res.Problems))
			for _, p := range res.Problems {
				fmt.Fprintf(stdout, "   - %s\n", p)
			}
			return exitFail
		}
		// ★ GAP-20261018-16「校验面」自检：蛋内 recipe.json 声明的权重必须真在索引里
		// （缺 ⇒ 具名拒；壳索引/无声明 ⇒ 不适用）。孵化器那条判据在 spec 期，这条更早一档：
		// 蛋**离开打包机之前**就该是自包含的。
		if derr := zegWeightsDeclGateEgg(a.a, res.Index); derr != nil {
			fmt.Fprintf(stderr, "%s: %v\n", zegProg, derr)
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
