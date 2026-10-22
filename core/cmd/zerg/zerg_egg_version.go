// zerg_egg_version.go —— 卵6（版本与兼容）的实现面：**不可变面/可加面二分类** · **min_readable 牙**
// · **签名块世代 ＝ 主号 3（旧读端必拒）** —— ★ 主号 **2 已被【段表化】占用**（设计稿 Q3 已拍 ·
// `zerg_egg.go` 的 `zegV2LayoutVer`）⇒ 签名块顺延到 3（见常量 `zegFormatVersionSigBlock` 的注）；
// · **13 档报话/退码表（D1–D13 · D6 拆 D6a/D6b）**。
//
// 真源（只读）：
//   - 设计稿 §十六 16.5（版本双分类 R1/R2 · 13 档表 · 漏点 F1「min_readable 有声明无牙」/ F2「加签名块必升主号」）
//   - 设计稿 §11.2 不变量（本批**只加不删**地补第 7 条：字段二分类 · 照 erofs `feature_compat`/`feature_incompat` 双位图）
//   - 设计稿 §11.5（签名块 v2 ＝ 主号 2、旧读端必拒）§ 2026-10-18 顺延：段表化占 2 ⇒ 签名块落 3
//   - 设计稿 §十九 #29（二分类写死）· #30（min_readable **先给牙**）· #4（D7/D8 退码归 **1**）· #31（签名块 v2）· #28（不许跳版）
//     ★ #31 的「主号 2」这一格 2026-10-18 顺延为 **3**（段表化先占了 2；旧读端对 2/3 都走 D2 前向拒）
//
// 三条落判（用户 2026-10-15 已拍 · 逐字照做）：
//
//	① 旧读端对**可加面** —— MUST NOT error（照 OCI 逐字「MUST NOT error on encountering a `mediaType`
//	  that is unknown」）；新增键 / 新增非必需段类型一律**忽略并登记**，✗ 不许报错。
//	② **不可变面**不符 —— 必须拒（点名档位）：magic · 固定前缀 32B 布局与逐字段偏移 · `version` 主号语义 ·
//	  段偏移必须对齐 `align` · `path` 禁 `..`；另含「**禁同一键换语义**」（同一键换了 JSON 类型 ⇒ 拒）。
//	③ **`min_readable` 先给牙** —— 卵声明 `min_readable` > 本端主号 ⇒ 拒 D3。
//	  ★ 为什么必须给牙：**改前该判据是永真判据**（见下「现读拄证」），一份「低于最低可读」的旧档不被拦。
//
// ---- 现读拄证：`min_readable` 改前为何是**永真判据**（逐行 · 件:行）--------------------------------
//
// 真源件 `core/internal/compat/migrate.go`（同一族「有声明、无牙」的结构病，片5 F1 点名）：
//
//	:86    `MinReadable int  \`json:"min_readable"\``   —— **只声明**（结构体字段）
//	:146-149 onDiskSchema 前置检查 —— 也只比 `e.CurrentSchema`（`ErrSchemaTooNew` 那一支），仍不看 `MinReadable`
//	:274   `func inspect(e Entry, path string, raw []byte) (Outcome, int, error) {`  —— **唯一判定口**
//	:285   `case ver > e.CurrentSchema:  return OutcomeFuture, ver, nil`   —— 只比 current
//	:287   `case ver == e.CurrentSchema: return OutcomeCurrent, ver, nil`  —— 只比 current
//	:289   `default:                     return OutcomeMigrated, ver, nil` —— ★ 「低于 current」一律进这里
//	:300-307 侧车档（`sc`）三段 switch（case 在 :301 / :303 / :305）—— **同上，只比 `e.CurrentSchema`**
//	:311   `func reconcile(...)` —— 只分派 Corrupt / Current / Future；其余一律 `migrateOne`（迁移，不是拒）
//
// ⇒ **逐行结论**：`inspect()` 的**全部分支**只读 `e.CurrentSchema`（＋`e.SchemaKey()`），**没有一条**读
// `e.MinReadable`；`readPath()`（:261）→ `reconcile()`（:311）→ `inspect()`（:274）这条链上，
// `MinReadable` 一次都不进判据。⇒ 任何输入下 `Outcome` 都与 `MinReadable` **无关** ⇒ 一条按
// `min_readable` 写的判据（「低于最低可读 ⇒ 拒」）在**任何输入上都不会为假** = **永真判据**（判据弱于对象）。
// 它现在只出现在三处**非判定面**：`:223/:229/:234/:239` 报表行、`compat.go:169-174` 自检一致性、`zerg-compat/main.go` 打印。
//
// ★ 本件的处置**不是**把那条永真判据换个名字留下：卵侧 `min_readable` 是**读端真的会读到并据以拒**的一格
// （`zegCompatGateMinReadable` ⇒ 拒 D3 并点名），且负控（哑化该格）会让自检**当场红**（见 `zegVersionSelfTest`）。
//
// ---- 出厂声明（本件**零** CLI 接线 · 入口由 `zerg_egg.go` 的两处调用点接入）-----------------------
//
//	· `zegParseIndex`（读端唯一实现 · §11.2 第 6 条「读写同源」）里插两处：前缀主号闸 ＋ 索引体兼容闸；
//	· `--self-test` 链尾加一行（`zegVersionSelfTest`）。
//	★ 本件**不改**任何退码映射：卵族命令面（`zerg cocoon pack/info/verify/xfer`）遇读端错误一律走既有
//	  `exitFail`（=1）通道 —— 与已拍的「`D7`/`D8` 退码归 1」一致；13 档表里的**目标码**逐档落表（见
//	  `zegVersionTiers` 的 `Code` 列 ＋ 报告「报话对拍」一节的逐条差异）。
package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ---- 一 · 版本常量（三分：本端主号 / 本端最低可读 / 签名块世代）-----------------------------------

const (
	// zegFormatVersionSigBlock —— **签名块世代的主号**（§11.5 · §十九 #31 · 片5 F2）：
	// 「加签名块 = 结构变更 ⇒ 必升主号 ⇒ 旧读端必拒」；★ ✗ 不是「忽略」—— 忽略会把预留块当成数据段读。
	// ★★ **2026-10-18 顺延（段表化落地）**：设计稿原把签名块钉在**主号 2**，而设计稿 Q3 拍定的**段表化**
	//   先占了主号 2（`zerg_egg.go` 的 `zegV2LayoutVer` ＝ 8 KiB 双槽头 ＋ 尾段表 ＋ 引擎段在尾）⇒
	//   签名块世代顺延到 **3**。口径不变：**旧读端对 2 与 3 一律 D2 前向拒**（`zegCompatGateMainVersion`）；
	//   `egg_sign.go` 的 `eggSlotV2Major = 2`（「indexMajor ≥ 2 ⇒ 旧读端拒」）**仍然成立**，那件未动。
	zegFormatVersionSigBlock = 3
	// zegMinReadableVersion —— **本端（读端）仍能读懂的最低主号**（＝读端侧 `min_readable`）。v1 端 = 1。
	// ★ 与**卵侧**声明的 `min_readable`（索引 JSON 里的可加面键）是**互为镜像的两格**：
	//   卵声明 `min_readable` > 本端主号 ⇒ 拒 D3（卵在说「你得够新」）；
	//   卵主号 < 本端 `min_readable` ⇒ 拒 D3（本端在说「你太旧」）。
	zegMinReadableVersion = 1
	// zegVersionSitePrefix / zegVersionSiteIndex —— 报话里点名**哪一处**判的（读写同源的两处闸）。
	zegVersionSitePrefix = "前缀"
	zegVersionSiteIndex  = "索引体"
	// zegVerProbeEgg —— 自检夹具卵的件名。★ **故意不带内容 sha**：`zegvVerifyEntry` 的内容寻址缓存
	// 只对 `<64 位 hex>.egg` 建键（`zegvKeyFromBase`）⇒ 本件名保证四态**每态都真解析索引**（✗ 不走命中档）。
	zegVerProbeEgg = "compat-probe.egg"
)

// ---- 二 · 不可变面 / 可加面二分类（机检物 ①：表 ＋ 检查器 · §11.2 不变量 7 · §十九 #29）----------

// zegSurface —— 索引 JSON 里一个键的**面**归属（照 erofs `feature_incompat` / `feature_compat` 双位图）。
type zegSurface string

const (
	// zegSurfaceIncompat —— **不可变面**（erofs `feature_incompat`）：改一处 ⇒ **必升主号** ⇒ 旧读端**必拒**。
	zegSurfaceIncompat zegSurface = "incompat"
	// zegSurfaceCompat —— **可加面**（erofs `feature_compat`）：新增键 / 新增非必需段类型 ⇒ 旧读端
	// **MUST NOT error**（忽略并登记）。
	zegSurfaceCompat zegSurface = "compat"
)

// zegKeySpec —— 一条「已知键」的规格：面归属 ＋ 声明的 JSON 类型（kind）。
//
// ★ 二分类**写死**在这里（＝ §11.2 不变量 7 的机检物）；`Kind` 那一格是「**禁同一键换语义**」的判据 ——
// 同一个键换了 JSON 类型 ⇒ 语义已变 ⇒ 拒（要新键 ＋ 升主号，✗ 不许原地改）。
type zegKeySpec struct {
	Key     string
	Surface zegSurface
	Kind    string // "string" | "number" | "array" | "object"
	Why     string
}

// ★ F24（`GAP-20261015-13` · 乙·单向派生）：下面两张表 `zegIndexKeySpecs` / `zegEntryKeySpecs` 是
// `*.egg` 格式规格的**唯一机检真源** —— 子端 `agent/internal/eggpkg` 的键判据由它们**单向派生**
// （派生件 `agent/internal/eggpkg/eggformat_gen.go` ＋ 一致性门 `eggspecgen --check`：真源规格 sha16
// 不符或派生件被手改 ⇒ 判红点名；✗ 不许在子端再手写第二份判据表）。
//
// zegIndexKeySpecs —— v1 索引 JSON **顶层已知键**的二分类表（逐条给依据）。
var zegIndexKeySpecs = []zegKeySpec{
	{Key: "magic", Surface: zegSurfaceIncompat, Kind: "string",
		Why: "`magic` = 4B `ZEGG`：取值语义不可变（§11.2 不变量 7 · 不可变面第一项）"},
	{Key: "version", Surface: zegSurfaceIncompat, Kind: "number",
		Why: "`version` 主号语义不可变（只在不可变面变化时 +1 · §16.5 R2）"},
	{Key: "align", Surface: zegSurfaceIncompat, Kind: "number",
		Why: "段偏移必须对齐 `align`（读端只信文件里的 `align` · §11.2 第 1/2/3 条）"},
	{Key: "entries", Surface: zegSurfaceIncompat, Kind: "array",
		Why: "段表本身 ＝ 固定前缀 `index_len` 之后的结构；逐字段偏移（`off`/`size`）是读端解析的前提"},
	{Key: "tool", Surface: zegSurfaceCompat, Kind: "object",
		Why: "元数据（产出工具名/版本）：按 §16.5 R2 属「不影响结构的演进 ⇒ 不动主号、只进索引」"},
	{Key: "created_at", Surface: zegSurfaceCompat, Kind: "string",
		Why: "元数据（产出时刻）：同 R2（不影响结构）"},
}

// zegEntryKeySpecs —— `entries[]` 元素里的**已知键**（逐字段偏移与语义面 · §11.2 不变量 4）。
var zegEntryKeySpecs = []zegKeySpec{
	{Key: "path", Surface: zegSurfaceIncompat, Kind: "string",
		Why: "`path` 相对路径语义不可变（禁 `..` · §11.2 第 5 条）"},
	{Key: "off", Surface: zegSurfaceIncompat, Kind: "number",
		Why: "段偏移（必须对齐 `align`）：偏移即格式本身"},
	{Key: "size", Surface: zegSurfaceIncompat, Kind: "number",
		Why: "段字节数：与 `off` 一起定段界，读端解析的前提"},
	{Key: "sha256", Surface: zegSurfaceIncompat, Kind: "string",
		Why: "逐段内容摘要（十六进制）：校验面语义不可变（§11.2 第 3 条）"},
	{Key: "mode", Surface: zegSurfaceIncompat, Kind: "number",
		Why: "段权限位：重建面的语义（`os.FileMode` 十进制），换语义会静默改权限"},
}

// zegAdditiveKeyExamples —— **可加面**的已知样例（✗ 不进上面两张「已知键」表 ⇒ 它们在读端就是**未知键**，
// 走「忽略 ＋ 登记」那一支）。★ 立此表只为**把二分类的另一半也写成机检物**（谁是可加面，逐条可读）。
var zegAdditiveKeyExamples = []string{
	"min_readable",     // 卵声明「最低可读主号」（§16.5 R2 · 本件给它牙）
	"wall_min_version", // 卵侧声明茧壁最低版（§16.5 V2 · 加索引键 ⇒ 可加面 ⇒ 不动主号）
	"mediaType",        // 段类型声明（§十九 #11：先只加 `mediaType` 声明、✗ 不引 ORAS）
	"source_sha",       // 构建自证四字段（§13.3 · GAP-20261015-14）
	"dirty",            // 同上
	"toolchain",        // 同上
	"host",             // 同上
}

// zegTierSurface —— **二分类专档**（刻意**不占** D 号）：D1–D13 是「卵不能孵时怎么报」那 13 档；
// 「同一键换语义」是**分类规则**本身的拒，不是那 13 档之一 ⇒ 单列一档，✗ 不塞进 D 号里充数。
const zegTierSurface = "分类·同键换语义"

// zegRefusal —— 读端拒（**点名档位** ＝ 能机检的那一格）。
//
// ★ 退码面（`GAP-20261015-24` 落 D1/D4/D13；`GAP-20261015-25` 追加 D2/D9）：`Coded`/`Code` 两格只在
// **13 档表里有码、且已把该档接到命令面**的档上置位 —— 码值**取自 `zegVersionTiers` 同一行**（单真源），
// ✗ 不在调用点手抄。其余档仍走卵族命令面既有的 `exitFail`（=1）通道（本单 ✗ 不扩面改它们）。
// `Detail` 是给人看的补充读数（实读 magic 的十六进制 / 非法 align 值 / `json.Unmarshal` 原生错），
// **不进** `Msg`：`Msg` 必须逐字等于 13 档表的报话列（判据靠这一条才有牙）。
type zegRefusal struct {
	Tier   string // 档位号（D1–D13 / D6a/D6b / zegTierSurface）
	Msg    string // 报话（人面逐字 ＝ 13 档表 `Msg` 列）
	Detail string // 细节读数（可空 · 与报话分两行打）
	Code   int    // 退码（仅 `Coded` 为真时有效）
	Coded  bool   // 本档是否已按 13 档表接上退码
}

func (r *zegRefusal) Error() string { return r.Msg }

// zegTierRefuse —— 按 **13 档表同一行**造一条带码的读端拒：报话取 `Msg`（`args` 非空时按模板格式化），
// 退码取 `Code`。★ 这就是「报话与退码都对齐设计表」的结构保证：改表即改行为，✗ 无需再同步第二处文案。
func zegTierRefuse(tier, detail string, args ...any) *zegRefusal {
	t, ok := zegTierOf(tier)
	if !ok {
		return &zegRefusal{Tier: tier, Msg: fmt.Sprintf("拒孵：13 档表里没有档位 %s（%s）", tier, detail),
			Detail: detail, Code: exitUsage, Coded: true}
	}
	msg := t.Msg
	if len(args) > 0 {
		msg = fmt.Sprintf(t.Msg, args...)
	}
	return &zegRefusal{Tier: tier, Msg: msg, Detail: detail, Code: t.Code, Coded: true}
}

// zegRefusalDetail —— 取细节读数（非带细节的拒 ⇒ 空串）。
func zegRefusalDetail(err error) string {
	if r, ok := err.(*zegRefusal); ok {
		return r.Detail
	}
	return ""
}

// zegRefusalExit —— 带码拒 ⇒ (码, true)；其余 ⇒ (0,false)（调用方回落既有 `exitFail` 通道）。
func zegRefusalExit(err error) (int, bool) {
	if r, ok := err.(*zegRefusal); ok && r.Coded {
		return r.Code, true
	}
	return 0, false
}

// ---- 三 · 13 档报话 / 退码表（机检物 ② · 逐档「谁报 · 报什么 · 退码 · 依据」）------------------

// zegVersionTier —— 一档的完整格（★ 触发 / **谁报** / **报什么** / **退码** / 依据 / 现读状态）。
type zegVersionTier struct {
	Tier    string // 档位号（D6 按**必需/非必需**拆 D6a/D6b ⇒ 13 档 = 14 行）
	Trigger string // ① 触发条件
	Who     string // ② 谁报（报话的产生面）
	Msg     string // ③ 报话（逐字）
	Code    int    // ④ 退码（目标码 · 取自本仓既有命名空间常量）
	Basis   string // 依据（§节 ＋ 外部逐字）
	Now     string // 现读状态（件:行或本包函数 ＋ 与目标的差，『未实现』逐字写出）
}

// zegVersionTiers —— 13 档（D6 拆两行 ⇒ 14 行）报话/退码表。★ 退码常量取自本仓**既有**命名空间
// （`exitOK`/`exitFail`/`exitUsage`/`exitBlocked`/`exitResource`）⇒ ✗ 不新立码位。
// ★ 用户 2026-10-15 已拍：**`D7`/`D8` 退码归 1**（`exitFail`），逐字落在下面两行。
var zegVersionTiers = []zegVersionTier{
	{Tier: "D1", Trigger: "前缀畸形：前 4B ≠ `ZEGG`，或前缀 < 32B",
		Who: "读端统一入口 `zegParseIndex`（`zerg cocoon info|verify|xfer` 都走它）",
		Msg: "拒孵：不是卵（magic 期望 ZEGG，实读 %s）", Code: exitUsage,
		Basis: "§11.1 固定前缀；erofs「magic 不符 ⇒ 读者必须拒」",
		Now:   "已实现（GAP-20261015-24）：前缀档与索引体档都经 `zegTierRefuse(\"D1\", …)` 出话（实读 4B 以十六进制填 `%s`）；退码走本行 `Code`＝2。改前：`magic 不符（不是 ZEGG 文件）：b'…'` / `文件太短，不足固定前缀 32 字节`，退码 1"},
	{Tier: "D2", Trigger: "`version` 高于本机认知（**前向** · 含段表化 2 / 签名块世代 3）",
		Who: "读端 `zegCompatGateMainVersion`（前缀 ＋ 索引体两处同源）",
		Msg: "拒孵：卵格式 version 高于本机认知 —— 不按新格式猜着跑；请升级孵化器（或用卵内茧壁兜底）", Code: exitUsage,
		Basis: "§11.5（签名块 v2 ＝ 主号 2 · 旧读端必拒；★ 2026-10-18 主号 2 转给段表化 ⇒ 签名块落 3）· §十九 #31 · 片5 F2；`compat/migrate.go` `ErrSchemaTooNew`",
		Now:   "已实现（GAP-20261015-25）：前缀档与索引体档两条分支都经 `zegTierRefuse(\"D2\", …)` 出话 —— 报话**逐字** ＝ 本行 `Msg`；version 实读/本机主号/「签名块 v2 世代」那段进 `Detail` 行（✗ 不再拼进报话）；退码走本行 `Code`＝2。改前（GAP-20261015-24 只报未修）：`拒孵：卵格式 version=3 高于本机认知 2 —— 前向拒（★ 签名块 v2 世代…）[前缀]`，退码 1"},
	{Tier: "D3", Trigger: "`version` 低于本端最低可读（**过旧**）／**卵声明 `min_readable` > 本端主号**",
		Who: "读端 `zegCompatGateMinReadable`（★ 本件**新给牙**的那一格）",
		Msg: "拒孵：卵 version 低于本机最低可读（过旧，迁移路径已退役）—— 改名或重打", Code: exitUsage,
		Basis: "§16.5 · §十九 #30（`min_readable` **先给牙**）；★ 改前**永真无牙**（拄证见件头 · 片5 F1）",
		Now:   "已实现（本件 · **新牙**）：改前无此分支（`min_readable` 一格都不判定）⇒ 改前该态放行"},
	{Tier: "D4", Trigger: "`align` 非法：非 2 的幂 / < 4096 / 与段偏移不符",
		Who: "读端 `zegParseIndex`（前缀档 / 索引体档 / v2 头槽档 —— 三处同一条 `zegTierRefuse`）",
		Msg: "拒孵：align 不合法（须 2 的幂且 ≥4096）／段偏移未对齐", Code: exitUsage,
		Basis: "§11.2 第 1、3 条；§10.8「读端只信文件里的 align」",
		Now:   "已实现（GAP-20261015-24）：前缀档 `align 不合理` / 索引体档 `align 字段不合理` / v2 头槽档 `命中规则 v2_header_align_invalid` 三处都经 `zegTierRefuse(\"D4\", …)` 出话（非法值进 `Detail` 行）；退码走本行 `Code`＝2。★ 触发里的「与段偏移不符」子支仍只在 `zegVerify` 的 problems 面（报 `%s 偏移未对齐` 并退 1）—— 本单 ✗ 不改 problems 面（见账注）"},
	{Tier: "D5", Trigger: "契约段缺**必需字段**（引擎入口/绑定/限额任一为 nil）",
		Who: "孵化器（`hatch_from_egg.go` → `hgtContractGate` / `hgtCheckContract`）· 契约语义真源 `agent/internal/enclosure/contract.go`",
		Msg: "拒孵：契约段缺必需字段（缺项必拒，不默默取默认值）", Code: exitUsage,
		Basis: "`agent/internal/enclosure/contract.go` 头注逐字「缺项必拒」",
		Now:   "已实现（GAP-20261015-25 · 本单）：`hgtContractGate` 在孵化入口按段名闭集找到契约段（`contract/contract.json` 等）⇒ `hgtCheckContract` 逐维判缺项（十维 nil / `filesystem.scratch_dir` 空 / `ro_binds` 不全 / `readiness.probe` 越界）⇒ `zegTierRefuse` 点名 D5，报话**逐字** ＝ 本行 `Msg`（缺项清单进 `Detail`）；退码走本行 `Code`＝2。★ 契约语义真源在 agent 模块（core 导不到）⇒ 本件落**读端镜像**，`TestHgtContractSourceParity` 现读真源十维逐条对拍。卵里不带契约段 ⇒ 本档不介入（✗ 不把「没有」当「缺字段」）"},
	{Tier: "D6a", Trigger: "**必需**段类型未知（`mediaType` 在本机不认识且为孵化必需）",
		Who: "孵化器（`hatch_from_egg.go` → `hgtMediaTypeGate`）· 必需性来自段表 path 推出的五段类",
		Msg: "拒孵：段的 mediaType 本机不认识，且为孵化必需 —— 拒（不猜）", Code: exitUsage,
		Basis: "erofs `feature_incompat`；§9.11②",
		Now:   "已实现（GAP-20261015-25 · 本单）：`hgtMediaTypeGate` 逐段判 —— 已声明（非空）且在 `zegMediaTypeClosedSet` 之外，且该段按 `zegSegOfPath` 属**引擎段/权重段/契约段**（孵化必需）⇒ `zegTierRefuse` 点名 D6a，报话逐字 ＝ 本行 `Msg`；退码本行 `Code`＝2。非必需段（配方段）闭集外 ⇒ 走 D6b（跳过并登记，见 `hgtMediaTypeGate` 的 notes ⇒ 交接件 `d6b_skipped`）。★ 「未标 mediaType」不入本档（读端已登记 `idx.UnlabeledMedia`）—— 本档触发词是「不认识」，不是「没声明」"},
	{Tier: "D6b", Trigger: "**非必需**段类型未知",
		Who: "读端（忽略并收进 `zegSurfaceAudit`）＋ 孵化面登记（卵4 卵档案）",
		Msg: "孵化继续：跳过未知段（已在卵档案登记）", Code: exitOK,
		Basis: "OCI「MUST NOT error」；§十九 #5（登记面：卵档案 ＋ 观测面**双登记**）",
		Now:   "半实现（本件）：读端**已收进审计面**（`zegSurfaceAudit.AdditiveEntryKeys`）且**不报错**；卵档案登记面归 卵4"},
	// ★ D7 的报话在**改后**落成 `%s` 模板（`GAP-20261015-25`）：设计原文「期望 a，实读 b」里的 a/b 与 D1 的
	// 「实读十六进制」同属**占位词**（见 GAP-20261015-24 对拍表 §五-④）⇒ 本仓填**整段 sha256 十六进制**：
	// `%s`＝索引里的期望值、第二个 `%s`＝实读计算值。判据按「模板 ＋ 填值」逐字对，✗ 不按占位字面。
	{Tier: "D7", Trigger: "逐段 `sha256` 与索引不符",
		Who: "校验面 `zegvVerifyEntry` / `zegVerify`（`zerg cocoon verify`）",
		Msg: "拒孵：段内容 sha256 与索引不符（期望 %s，实读 %s）", Code: exitFail,
		Basis: "§9.5 判据 3；★ **用户 2026-10-15 拍定：归 1**（`§十九` #4 · 片5 推荐 1）",
		Now:   "已实现（GAP-20261015-25）：单线程档 `zegVerify` 与并行档 `zegvVerifySeg` 共用 `zegSHAMismatchProblem`（真源 ＝ 本行 `Msg`），人面行 ＝ `段 path: ` ＋ 本行报话（填期望/实读两枚 sha256）；退码走本行 `Code`＝1（改前即 1 ⇒ 本单只对齐报话）。改前：`%s sha256 不符`"},
	{Tier: "D8", Trigger: "签名不匹配 /（v2 起）签名缺失",
		Who: "签名面（`§11.5` 预留槽 · 拄 cosign/Sigstore · 归 卵1④/卵5）",
		Msg: "拒孵：签名校验失败（索引＋全部段的签名对不上）／v2 卵缺签名", Code: exitFail,
		Basis: "§11.5 / §9.9；§9.11③ 改拄 cosign；★ **用户 2026-10-15 拍定：归 1**（`§十九` #4）",
		Now:   "未实现（他面）：现读索引无签名块（GAP-20261015-14）· 本单只落「主号 2 ⇒ 旧读端必拒」那条规则"},
	{Tier: "D9", Trigger: "`path` 非法：含 `..` / 绝对 / 盘符 / 空段 / NUL",
		Who: "读端 `zegAuditIndexEntries` → `zegRefusePath`（打包端 / 读端 / 重建端三处同表）；`zerg cocoon info|verify`",
		Msg: "拒孵：段 path 非法（须规范化相对路径，禁绝对/盘符/空段/..）", Code: exitUsage,
		Basis: "§11.2 第 5 条（现读只挡 `..` 且只在打包端）；§15.8 L26 · §十九 #26",
		Now:   "已实现（GAP-20261015-25）：`zegRefusePath` 命中 12 条 `zegPathRules` 任一条 ⇒ `zegTierRefuse(\"D9\", …)` —— 报话**逐字** ＝ 本行 `Msg`；命中规则名 ＋ 依据 ＋ 实读路径进 `Detail` 行（✗ 不再拼进报话）；退码走本行 `Code`＝2。改前（GAP-20261015-24 只报未修）：`拒：段 path \"../x\"（须为规范化 NFC 相对路径） —— 命中规则 path_dotdot（…）`，退码 1。★ 借挂同一 D9 号的**非 path 规则**（`fs_mode_unsafe` / `fs_symlink_*` / `rebuild_*`）仍走旧形态（`zegRuleRefuse` · rc=1 · 见账注）"},
	{Tier: "D10", Trigger: "卵本体**半成品**（临时名未 rename / 逐块校验未过）",
		Who: "落地面（卵2 `core/internal/store/eggstore/`：同卷临时名 ＋ 完工标记）＋ 孵化入口 `hgtHalfRefuse`",
		Msg: "拒孵：卵是未完工的半成品（缺完工标记）", Code: exitBlocked,
		Basis: "主表 `8`＝有 BLOCKED；§15.4 L12 / §15.7 L22 · `§十九` #13",
		Now:   "已实现（GAP-20261015-25 · 本单）：孵化入口 `hgtHalfRefuse` 两条腿 —— ①临时件未 rename（卵件旁有 `.part` / `.tmp` 同级件）②名册 `Complete` 未置位 ⇒ `zegTierRefuse` 点名 D10，报话逐字 ＝ 本行 `Msg`（证据件路径进 `Detail`）；退码本行 `Code`＝8。判据真源仍是卵2 `eggstore`（`DoneMarker` / `Roster` · ✗ 本单不另造一份）"},
	{Tier: "D11", Trigger: "孵化前磁盘检查不足",
		Who: "预算面（卵2 水线 ＋ `§13.2`）＋ 孵化入口 `hgtDiskGate`",
		Msg: "拒孵：可用磁盘不足（需更多 %s）；可回收卵清单：%s", Code: exitResource,
		Basis: "主表 `10`＝资源不足；§13.2 · `§十九` #15",
		Now:   "已实现（GAP-20261015-25 · 本单）：孵化入口 `hgtDiskGate` 判两条 —— ①`eggstore.PrecheckHatch` 判词 `refuse`（水线/硬线/inode）②本卵展开所需（段表 payload 之和）> 现读可用（imagefs/nodefs 取小）⇒ `zegTierRefuse` 点名 D11。★ 报话两个 `%s` ＝ **还差多少**（人面字节）／**可回收卵清单**（`hfeListOrNone`），与 D7 同法按「模板 ＋ 填值」对；退码本行 `Code`＝10"},
	{Tier: "D12", Trigger: "**卵内茧壁 ⟷ 机器茧壁**版本不匹配",
		Who: "孵化器（`hatch_from_egg.go` → `hgtWallGate` · §五之三）",
		Msg: "孵化继续（不报错）：机器茧壁与卵声明不匹配 ⇒ 改用卵内那份；已在观测面标注", Code: exitOK,
		Basis: "§五之三；§15.3 L9 · `§十九` #32（V1 三段判：机器那份 ≥ `wall_min` 且同主号 ⇒ 用机器那份）",
		Now:   "已实现（GAP-20261015-25 · 本单 · 判定面）：`hgtWallGate` 按 V1 三段判出用哪一份 —— 机器那份 ≥ 卵声明 `wall_min_version` 且同主号 ⇒ machine；否则（未装/主号不同/低于最低版）⇒ 改用**卵内那份**（不报错 · 退码 0），结论与标注进交接件 `wall` / `wall_note`。★ 卵声明最低版从段表可加面键 `wall_min_version` 读（`zegIndex.WallMin`）。★ **需真机的一半**：机器已装茧壁的**版本探测**口 `hgtMachineWallVersion` 默认只认 env 旋钮（探测不到即「未装」）—— 真机上要接装好的茧壁件版本真源"},
	{Tier: "D13", Trigger: "索引 JSON 不可解析",
		Who: "读端 `zegParseIndex`（结构化解码前的面审计 `zegAuditIndexSurface` ＋ 解码本身）",
		Msg: "拒孵：索引 JSON 不可解析；文件保持不动", Code: exitUsage,
		Basis: "`wall/src/spec.rs`；`compat/migrate.go` 逐字「不静默丢状态」",
		Now:   "已实现（GAP-20261015-24）：`zegAuditIndexSurface` / `zegEggIntKey` / v1 解码三处都经 `zegTierRefuse(\"D13\", …)` 出话（原生错进 `Detail` 行，✗ 不再拼进报话）；退码走本行 `Code`＝2；读端只读不写 ⇒ 「文件保持不动」成立"},
}

// zegTierRequiredIDs —— 档位号闭集（**13 档**：D6 按必需/非必需**拆两行** ⇒ 14 个号）。
var zegTierRequiredIDs = []string{
	"D1", "D2", "D3", "D4", "D5", "D6a", "D6b", "D7", "D8", "D9", "D10", "D11", "D12", "D13",
}

// zegTierIDs —— 表内档位号（排序；给自印/对拍用）。
func zegTierIDs() []string {
	out := make([]string, 0, len(zegVersionTiers))
	for _, t := range zegVersionTiers {
		out = append(out, t.Tier)
	}
	sort.Strings(out)
	return out
}

// zegTierOf —— 按档位号取一档（不存在 ⇒ ok=false）。
func zegTierOf(tier string) (zegVersionTier, bool) {
	for _, t := range zegVersionTiers {
		if t.Tier == tier {
			return t, true
		}
	}
	return zegVersionTier{}, false
}

// ---- 四 · 面审计（跑在结构化解码**之前**）------------------------------------------------------

// zegSurfaceAudit —— 一次索引体面审计的结论。
//
// ★ `AdditiveTopKeys` / `AdditiveEntryKeys` 是**可加面的登记输入**：卵6① 的验收逐字要求
// 「非必需段类型未知 ⇒ 孵化继续 rc=0 且**登记卵档案**（✗ 不许只 warn）」⇒ 读端把「见到了哪些可加面键」
// **返回给调用方**（不是打一行日志了事）；卵档案那一半归孵化面（卵4）。
type zegSurfaceAudit struct {
	AdditiveTopKeys   []string // 顶层未知键（可加面 · 排序）
	AdditiveEntryKeys []string // `entries[]` 里的未知键（可加面 · 含 `mediaType` 一类 · 排序）
	IncompatSeen      []string // 命中的不可变面键（排序 · 便于对拍）
}

// zegJSONKind —— 一个 JSON 值的**类型名**（只认 token 首字符：body 已通过整段可解析 ⇒ 无需重解析）。
func zegJSONKind(raw json.RawMessage) string {
	s := strings.TrimSpace(string(raw))
	if s == "" {
		return "empty"
	}
	switch s[0] {
	case '"':
		return "string"
	case '{':
		return "object"
	case '[':
		return "array"
	case 't', 'f':
		return "bool"
	case 'n':
		return "null"
	}
	return "number"
}

// zegAuditKeySet —— 一组键的面判定（`where` 只用于报话点名 · `top` 决定未知键收哪一面登记）。
// ★ 不可变面键收进审计；未知键收**登记面**（✗ 不报错 —— 可加面 MUST NOT error）。
func zegAuditKeySet(m map[string]json.RawMessage, specs []zegKeySpec, where string, top bool, audit *zegSurfaceAudit) error {
	known := make(map[string]zegKeySpec, len(specs))
	for _, s := range specs {
		known[s.Key] = s
	}
	for k, v := range m {
		spec, ok := known[k]
		if !ok {
			if top {
				audit.AdditiveTopKeys = append(audit.AdditiveTopKeys, k)
			} else {
				audit.AdditiveEntryKeys = append(audit.AdditiveEntryKeys, k)
			}
			continue
		}
		got := zegJSONKind(v)
		if got != spec.Kind {
			return &zegRefusal{Tier: zegTierSurface, Msg: fmt.Sprintf(
				"拒孵：%s 的键 `%s` 类型由 %s 变成 %s —— 「同一键换了语义」禁止原地改（要新键 ＋ 升主号）；依据 §11.2 不变量 7 / §十九 #29 / `compat/migrate.go` 头注逐字「同一键换了含义 … 那比读不到更危险」",
				where, k, spec.Kind, got)}
		}
		if spec.Surface == zegSurfaceIncompat {
			audit.IncompatSeen = append(audit.IncompatSeen, where+"."+k)
		}
	}
	return nil
}

// zegAuditIndexSurface —— 索引体的**面审计**（必须跑在 `json.Unmarshal` 之前）：
//
//	① **已知键换了 JSON 类型** ⇒ 拒（「禁同一键换语义」· 档位 `zegTierSurface`）；
//	   ★ 为什么必须在前：`align` 由 number 变成 string 时，结构化解码只会吐一句无名的类型错
//	   （`cannot unmarshal string into Go struct field …`）⇒ 档位点不出来、也点不出「该升主号」。
//	② **未知键** ⇒ **可加面，不报错**（收进审计面交调用方登记 · 照 OCI 逐字「MUST NOT error」）。
//	③ **不可变面必需键缺项** ⇒ 拒（少 `magic`/`version`/`align`/`entries` 任一条，读端解析的前提就没了）。
func zegAuditIndexSurface(body []byte) (zegSurfaceAudit, error) {
	var audit zegSurfaceAudit
	var top map[string]json.RawMessage
	if err := json.Unmarshal(body, &top); err != nil {
		return audit, zegTierRefuse("D13", err.Error())
	}
	if err := zegAuditKeySet(top, zegIndexKeySpecs, zegVersionSiteIndex, true, &audit); err != nil {
		return audit, err
	}
	for _, spec := range zegIndexKeySpecs {
		if spec.Surface != zegSurfaceIncompat {
			continue
		}
		if _, has := top[spec.Key]; !has {
			return audit, &zegRefusal{Tier: "D5", Msg: fmt.Sprintf(
				"拒孵：索引体缺不可变面必需键 `%s`（%s）；缺项必拒，✗ 不按零值兜底", spec.Key, spec.Why)}
		}
	}
	rawEntries, hasEntries := top["entries"]
	if !hasEntries {
		return audit, nil
	}
	var ents []map[string]json.RawMessage
	if err := json.Unmarshal(rawEntries, &ents); err != nil {
		return audit, &zegRefusal{Tier: "D5", Msg: fmt.Sprintf(
			"拒孵：`entries` 不是对象数组（%v）；段表是读端解析的前提，✗ 不猜", err)}
	}
	for i, e := range ents {
		where := fmt.Sprintf("entries[%d]", i)
		if err := zegAuditKeySet(e, zegEntryKeySpecs, where, false, &audit); err != nil {
			return audit, err
		}
		for _, spec := range zegEntryKeySpecs {
			if spec.Surface != zegSurfaceIncompat {
				continue
			}
			if _, has := e[spec.Key]; !has {
				return audit, &zegRefusal{Tier: "D5", Msg: fmt.Sprintf(
					"拒孵：%s 缺不可变面必需键 `%s`（%s）", where, spec.Key, spec.Why)}
			}
		}
	}
	sort.Strings(audit.AdditiveTopKeys)
	sort.Strings(audit.AdditiveEntryKeys)
	sort.Strings(audit.IncompatSeen)
	return audit, nil
}

// ---- 五 · 版本闸（主号 ＋ `min_readable` 牙）---------------------------------------------------

// zegCompatGateMainVersion —— **主号闸**：前向拒（`version` 高于本端认知）＋ 过旧拒（低于本端最低可读）。
// `site` ＝ 判在前缀 还是 索引体（读写同源两处 · 报话里点名便于定位）。
//
// ★ `GAP-20261015-25`（D2 两差 → 对齐）：前向拒的两条分支都走 `zegTierRefuse("D2", …)` ——
// 报话**逐字** ＝ 13 档表 `Msg`（即设计表原文），版本实读/本机主号/「签名块 v2 世代」那段**读数**
// 进 `Detail` 行（✗ 不再拼进报话）；退码取表内 `Code`＝2。
// ★ D3（过旧 / `min_readable`）**未动** —— 该行设计表把两个互为反向的触发写在一格、只给了「过旧」
// 那句报话（见账注：设计表自相矛盾 ⇒ 本单拒做），仍走既有 `exitFail`（1）通道。
func zegCompatGateMainVersion(mainVer int, site string) error {
	if mainVer == zegFormatVersionSigBlock {
		return zegTierRefuse("D2", fmt.Sprintf(
			"version=%d > 本机认知 %d；★ 签名块 v2 世代 ＝ 主号 ≥ %d（主号 2 是段表化）⇒ 旧读端必拒，✗ 不是忽略：忽略会把预留块当数据段读（§11.5 / §十九 #31）[%s]",
			mainVer, zegFormatVer, zegFormatVersionSigBlock, site))
	}
	if mainVer > zegFormatVer {
		return zegTierRefuse("D2", fmt.Sprintf(
			"version=%d > 本机认知 %d（前向拒：不按新格式猜着跑）[%s]", mainVer, zegFormatVer, site))
	}
	if mainVer < zegMinReadableVersion {
		return &zegRefusal{Tier: "D3", Msg: fmt.Sprintf(
			"拒孵：卵 version=%d 低于本机最低可读 %d（过旧，迁移路径已退役）—— 改名或重打 [%s]",
			mainVer, zegMinReadableVersion, site)}
	}
	return nil
}

// zegCompatGateMinReadable —— ★ **`min_readable` 的牙**（本件新增 · §十九 #30）。
//
// 卵侧声明 `min_readable` ＝「读我所需的**最低读端主号**」；本端主号 < 它 ⇒ **拒 D3 并点名**。
// ★ 改前该判据**永真无牙**（拄证见件头：`compat/migrate.go` 的 `inspect()` 全分支只比 `current_schema`）。
func zegCompatGateMinReadable(eggMinReadable, localMain int) error {
	if eggMinReadable > localMain {
		return &zegRefusal{Tier: "D3", Msg: fmt.Sprintf(
			"拒孵：卵声明 min_readable=%d 高于本端主号 %d（本端最低可读 %d）—— 这份卵要用更新的读端；请升级孵化器，✗ 不按旧格式猜着读（§11.2 不变量 7 / §十九 #30）",
			eggMinReadable, localMain, zegMinReadableVersion)}
	}
	return nil
}

// zegCompatGateLegacy —— **改前形态**的对照面（★ 只为负控/对拍而存在，✗ 不被任何生产路径调用）：
// 逐字复刻改前那一条 —— 只比「等于本端主号」，不看 `min_readable`（＝片5 F1 说的「有声明无牙」）。
//
// ★ 它使「牙有没有牙」可机检：把 `zegCompatGateMinReadable` 那一支哑化掉之后，本件的结论必须与它**逐字同**
// （＝退回永真放行）⇒ 自检里那格「两个结论必须不同」当场红。
func zegCompatGateLegacy(mainVer int) error {
	if mainVer != zegFormatVer {
		return &zegRefusal{Tier: "改前", Msg: fmt.Sprintf("格式版本不支持：%d", mainVer)}
	}
	return nil
}

// zegEggIntKey —— 从索引体顶层读一个整数键（缺 ⇒ (0,false)；非整数 ⇒ 拒「已知语义的键须保持形状」）。
func zegEggIntKey(body []byte, key string) (int, bool, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(body, &top); err != nil {
		return 0, false, zegTierRefuse("D13", err.Error())
	}
	raw, has := top[key]
	if !has {
		return 0, false, nil
	}
	var v int
	if err := json.Unmarshal(raw, &v); err != nil {
		return 0, false, &zegRefusal{Tier: zegTierSurface, Msg: fmt.Sprintf(
			"拒孵：`%s` 不是整数（%v）—— 已知语义的键必须保持形状；依据 §11.2 不变量 7", key, err)}
	}
	return v, true, nil
}

// zegCompatGateIndexBody —— **索引体闸**：① 面审计（同键换语义 ⇒ 拒 / 未知键 ⇒ 可加面不报错）
// ② 主号闸（与前缀同源）③ `min_readable` 牙。★ 返回的审计结论供调用方**登记**（可加面那一半）。
func zegCompatGateIndexBody(body []byte) (zegSurfaceAudit, error) {
	audit, err := zegAuditIndexSurface(body)
	if err != nil {
		return audit, err
	}
	mainVer, hasVer, err := zegEggIntKey(body, "version")
	if err != nil {
		return audit, err
	}
	if !hasVer {
		mainVer = 0 // 缺 `version` 已由面审计的「不可变面必需键缺项」拒掉；这里只是兜底（✗ 不静默放行）
	}
	if err := zegCompatGateMainVersion(mainVer, zegVersionSiteIndex); err != nil {
		return audit, err
	}
	eggMin, hasMin, err := zegEggIntKey(body, "min_readable")
	if err != nil {
		return audit, err
	}
	if hasMin {
		if err := zegCompatGateMinReadable(eggMin, zegFormatVer); err != nil {
			return audit, err
		}
	}
	return audit, nil
}

// ---- 六 · 机检物导出（表 ↦ JSON）--------------------------------------------------------------

// zegCompatTableJSON —— 把二分类表 ＋ 13 档表导成 JSON（**机检物就是这份表本身**，不是散文）。
// ✗ 不写盘（调用方/自检决定怎么用）。
func zegCompatTableJSON() ([]byte, error) {
	type keyRow struct {
		Key     string `json:"key"`
		Surface string `json:"surface"`
		Kind    string `json:"kind"`
		Why     string `json:"why"`
	}
	type tierRow struct {
		Tier    string `json:"tier"`
		Trigger string `json:"trigger"`
		Who     string `json:"who"`
		Msg     string `json:"msg"`
		Code    int    `json:"code"`
		Basis   string `json:"basis"`
		Now     string `json:"now"`
	}
	conv := func(specs []zegKeySpec) []keyRow {
		out := make([]keyRow, 0, len(specs))
		for _, s := range specs {
			out = append(out, keyRow{Key: s.Key, Surface: string(s.Surface), Kind: s.Kind, Why: s.Why})
		}
		return out
	}
	tiers := make([]tierRow, 0, len(zegVersionTiers))
	for _, t := range zegVersionTiers {
		tiers = append(tiers, tierRow{Tier: t.Tier, Trigger: t.Trigger, Who: t.Who, Msg: t.Msg, Code: t.Code, Basis: t.Basis, Now: t.Now})
	}
	doc := struct {
		Schema           string    `json:"schema"`
		MainVersion      int       `json:"main_version"`
		MinReadable      int       `json:"min_readable"`
		SigBlockVersion  int       `json:"sig_block_main_version"`
		IndexKeys        []keyRow  `json:"index_keys"`
		EntryKeys        []keyRow  `json:"entry_keys"`
		AdditiveExamples []string  `json:"additive_key_examples"`
		Tiers            []tierRow `json:"tiers"`
	}{
		Schema:           "zerg-egg-compat/1",
		MainVersion:      zegFormatVer,
		MinReadable:      zegMinReadableVersion,
		SigBlockVersion:  zegFormatVersionSigBlock,
		IndexKeys:        conv(zegIndexKeySpecs),
		EntryKeys:        conv(zegEntryKeySpecs),
		AdditiveExamples: zegAdditiveKeyExamples,
		Tiers:            tiers,
	}
	return json.MarshalIndent(doc, "", "  ")
}

// ---- 七 · 自检（四态 ＋ 负控 · 真跑真造件）-----------------------------------------------------

// zegVersionSelfTest —— 卵6 的自检：**四态亲测 ＋ 一条负控**（✗ 不与旧档/卵3① 档混行 —— 三行分开印）。
//
//	① 同主号**可加面**未知段 ⇒ 旧读端**照旧跑**（不 error）
//	② 同主号**不可变面改语义** ⇒ **必拒**（点名 `zegTierSurface`）
//	③ **`min_readable` 超本端**（不可变面不符）⇒ **必拒且点名**（D3）★ 本格就是新牙的那一格
//	④ **主号 3（签名块世代）** ⇒ 旧读端**必拒**（D2）
//	负控：同一份 `min_readable` 超本端输入走**改前形态**（`zegCompatGateLegacy`）必须**放行**，
//	      且与牙的结论**必须不同** ⇒ 哑化牙 ⇒ 本格红。
func zegVersionSelfTest(stdout, stderr io.Writer) int {
	t, err := os.MkdirTemp("", "zerg-egg-version-selftest")
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", zegProg, err)
		return exitFail
	}
	defer os.RemoveAll(t) // 自回收（TMPDIR 前后 delta=0）

	src := filepath.Join(t, "tree")
	if err := os.MkdirAll(filepath.Join(src, "model"), 0o755); err != nil {
		return zegSelfFail(stderr, err)
	}
	payload := make([]byte, 4096)
	for i := range payload {
		payload[i] = byte(i * 3)
	}
	if err := os.WriteFile(filepath.Join(src, "model", "w.safetensors"), payload, 0o644); err != nil {
		return zegSelfFail(stderr, err)
	}
	if err := os.WriteFile(filepath.Join(src, "config.json"), []byte(`{"arch":"compat-selftest"}`), 0o644); err != nil {
		return zegSelfFail(stderr, err)
	}
	egg := filepath.Join(t, zegVerProbeEgg)
	if _, err := zegPack(src, egg, 4096, "2000-01-01T00:00:00Z"); err != nil {
		return zegSelfFail(stderr, err)
	}

	// 每一态各造一份**专属**卵（✗ 不共用：造态会改盘上件）。
	build := func(name string, mutate func(idx []byte) ([]byte, error), prefixVer int) (string, error) {
		p := filepath.Join(t, name)
		d, err := os.ReadFile(egg)
		if err != nil {
			return "", err
		}
		if err := os.WriteFile(p, d, 0o644); err != nil {
			return "", err
		}
		if err := zegVerRebuildEgg(p, mutate, prefixVer); err != nil {
			return "", err
		}
		return p, nil
	}
	bodyOf := func(p string) []byte {
		d, _ := os.ReadFile(p)
		return d[zegHeaderPrefix : zegHeaderPrefix+zegIdxLenOf(d)]
	}
	putRaw := func(m map[string]json.RawMessage, k, v string) { m[k] = json.RawMessage(v) }
	editIdx := func(f func(m map[string]json.RawMessage) error) func(idx []byte) ([]byte, error) {
		return func(idx []byte) ([]byte, error) {
			var m map[string]json.RawMessage
			if err := json.Unmarshal(idx, &m); err != nil {
				return nil, err
			}
			if err := f(m); err != nil {
				return nil, err
			}
			return json.Marshal(m)
		}
	}

	// ---- 态①：可加面未知段 ⇒ 照旧跑（不 error）----
	eggAdd, err := build("additive.egg", editIdx(func(m map[string]json.RawMessage) error {
		putRaw(m, "min_readable", "1")     // 可加面键（声明的就是本端主号 ⇒ ✗ 不该拒）
		putRaw(m, "wall_min_version", "1") // 可加面键（§16.5 V2）
		var ents []map[string]json.RawMessage
		if err := json.Unmarshal(m["entries"], &ents); err != nil {
			return err
		}
		putRaw(ents[0], "mediaType", `"application/vnd.zerg.weights.v9+unknown"`) // 新增非必需段类型
		eb, err := json.Marshal(ents)
		if err != nil {
			return err
		}
		m["entries"] = eb
		return nil
	}), 0)
	if err != nil {
		return zegSelfFail(stderr, err)
	}
	auditAdd, aerr := zegCompatGateIndexBody(bodyOf(eggAdd))
	if aerr != nil {
		return zegSelfFail(stderr, fmt.Errorf("态① 可加面必须**不报错**（照 OCI「MUST NOT error」），实得：%v", aerr))
	}
	if _, err := zegParseIndex(eggAdd); err != nil {
		return zegSelfFail(stderr, fmt.Errorf("态① 可加面未知段被旧读端拒了（✗ 应为照旧跑）：%v", err))
	}
	resAdd, err := zegvVerifyEntry(io.Discard, eggAdd, "", 1)
	if err != nil || len(resAdd.Problems) != 0 {
		return zegSelfFail(stderr, fmt.Errorf("态① 可加面档 `verify` 不该红：probs=%v err=%v", resAdd.Problems, err))
	}

	// ---- 态②：同主号不可变面改语义 ⇒ 必拒（点名 zegTierSurface）----
	eggSem, err := build("samekey.egg", editIdx(func(m map[string]json.RawMessage) error {
		putRaw(m, "align", `"4096"`) // ★ 同一键换语义：类型由 number 变成 string
		return nil
	}), 0)
	if err != nil {
		return zegSelfFail(stderr, err)
	}
	_, semErr := zegParseIndex(eggSem)
	if semErr == nil {
		return zegSelfFail(stderr, fmt.Errorf("态② 同键换语义竟被放行（✗ 不可变面不符必须拒）"))
	}
	if ref, ok := semErr.(*zegRefusal); !ok || ref.Tier != zegTierSurface {
		return zegSelfFail(stderr, fmt.Errorf("态② 拒了但没点名档位 %s：%v", zegTierSurface, semErr))
	}

	// ---- 态③：`min_readable` 超本端 ⇒ 必拒且点名（D3）----
	eggMin, err := build("minread.egg", editIdx(func(m map[string]json.RawMessage) error {
		putRaw(m, "min_readable", fmt.Sprintf("%d", zegFormatVersionSigBlock)) // 卵要「更新世代（%d）的读端」
		return nil
	}), 0)
	if err != nil {
		return zegSelfFail(stderr, err)
	}
	_, minErr := zegParseIndex(eggMin)
	if minErr == nil {
		return zegSelfFail(stderr, fmt.Errorf("态③ min_readable 超本端竟被放行（✗ 改前永真的那一格）"))
	}
	if ref, ok := minErr.(*zegRefusal); !ok || ref.Tier != "D3" || !strings.Contains(ref.Msg, "min_readable") {
		return zegSelfFail(stderr, fmt.Errorf("态③ 拒了但没点名 D3/min_readable：%v", minErr))
	}
	// ★ 负控：改前形态（`zegCompatGateLegacy` 只比主号）必须**放行** —— 与上面那格「拒」的结论必须不同。
	if legErr := zegCompatGateLegacy(zegFormatVer); legErr != nil {
		return zegSelfFail(stderr, fmt.Errorf("负控前提不成立：改前形态在同一输入上竟也拒（%v）", legErr))
	}

	// ---- 态④：主号 3（签名块世代）⇒ 旧读端必拒（D2）----
	eggSig, err := build("sigblock.egg", func(idx []byte) ([]byte, error) { return idx, nil }, zegFormatVersionSigBlock)
	if err != nil {
		return zegSelfFail(stderr, err)
	}
	_, sigErr := zegParseIndex(eggSig)
	if sigErr == nil {
		return zegSelfFail(stderr, fmt.Errorf("态④ 主号 %d（签名块世代）竟被旧读端放行（✗ 必拒 · 不是忽略）", zegFormatVersionSigBlock))
	}
	// ★ D2（GAP-20261015-25）：报话逐字 ＝ 13 档表 `Msg`（不含细节）；「签名块 v2」那段读数落在 `Detail` 行
	// ⇒ 判据点 `Detail`，✗ 不再点 `Msg`（否则「报话逐字」这条就被细节吃掉）。
	if ref, ok := sigErr.(*zegRefusal); !ok || ref.Tier != "D2" || !strings.Contains(zegRefusalDetail(sigErr), "签名块 v2") {
		return zegSelfFail(stderr, fmt.Errorf("态④ 拒了但没点名签名块 v2/D2（细节行）：%v", sigErr))
	}

	fmt.Fprintf(stdout, "self-test（卵6 版本与兼容）通过（4 态 ＋ 1 负控）：① 可加面未知段 ⇒ 照旧跑（不 error · 登记面收 %d 个顶层键 ＋ %d 个段键）· ② 同键换语义（align: number→string）⇒ 拒[%s] · ③ min_readable=%d > 本端 %d ⇒ 拒[D3] 且点名 min_readable · ④ 主号 %d 签名块 ⇒ 拒[D2] 且点名「签名块 v2」· 负控：同一输入走改前形态 ⇒ 放行（＝改前永真；两结论不同 ⇒ 哑化牙本格必红）\n",
		len(auditAdd.AdditiveTopKeys), len(auditAdd.AdditiveEntryKeys),
		zegTierSurface, zegFormatVersionSigBlock, zegFormatVer, zegFormatVersionSigBlock)
	return exitOK
}

// zegIdxLenOf —— 从前缀里读索引体长度（自检内部用 · 与 `zegParseIndex` 同一字段序）。
func zegIdxLenOf(d []byte) int {
	dl := len(d)
	if dl < zegHeaderPrefix {
		return 0
	}
	n := int64(binary.LittleEndian.Uint64(d[16:24]))
	total := int64(zegHeaderPrefix) + n
	if n < 0 || total > int64(dl) {
		return 0
	}
	return int(n)
}

// zegVerRebuildEgg —— 把一枚卵的**索引体**换成 `mutate(旧体)`（保持 `align` 与段偏移不变），
// 可选把**前缀主号**改成 `prefixVer`（0 ＝ 不改）。★ 头块恒为 `align` 字节 ⇒ 段偏移一位不动。
func zegVerRebuildEgg(path string, mutate func(idx []byte) ([]byte, error), prefixVer int) error {
	d, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	dl := int64(len(d))
	if dl < int64(zegHeaderPrefix) {
		return fmt.Errorf("件太短，不足固定前缀 %d 字节", zegHeaderPrefix)
	}
	align := int64(binary.LittleEndian.Uint64(d[8:16]))
	n := int64(binary.LittleEndian.Uint64(d[16:24]))
	if n <= 0 || int64(zegHeaderPrefix)+n > dl {
		return fmt.Errorf("索引长度字段不合理：%d", n)
	}
	if align < zegMinAlign || int64(zegHeaderPrefix) > align {
		return fmt.Errorf("align 不合理：%d", align)
	}
	body := d[zegHeaderPrefix : zegHeaderPrefix+int(n)]
	newBody, err := mutate(body)
	if err != nil {
		return err
	}
	need := int64(zegHeaderPrefix) + int64(len(newBody))
	if need > align {
		return fmt.Errorf("改后索引体（%d 字节）放不进 %d 字节的头块", len(newBody), align)
	}
	head := make([]byte, int(align))
	copy(head, zegBuildPrefix(int(align), len(newBody)))
	if prefixVer != 0 {
		binary.LittleEndian.PutUint32(head[4:8], uint32(prefixVer))
	}
	copy(head[zegHeaderPrefix:], newBody)
	out := make([]byte, 0, len(head)+len(d)-int(align))
	out = append(out, head...)
	out = append(out, d[align:]...)
	return os.WriteFile(path, out, 0o644)
}
