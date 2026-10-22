// hatch_from_egg.go —— 卵开工 · 卵4：孵化器接「从单文件孵」（core 侧编排件）。
//
// 出处：设计真源 `01-设计/设计-茧壁-统一封闭契约与等级自证.md` 的 **§9.6 第 3 步**（「孵化器加
// 『从单文件孵』（读索引 ＋ 现有沙箱契约）＋ 与旧形态对拍（判据 2）」）· **§9.5 判据 2**（单文件孵 ⟷
// 旧形态孵 输出逐字节同）· **§10.9**（定选「孵化时展开权重段」）；任务单
// `项目文档/v2.5.15/任务单-卵单文件封装-v2-20261015.md` 的 **卵4①** 行；闸 **#13**（仅 `verified` 可孵）
// 与 卵2③（水线到硬线 ⇒ 拒新孵化）。
//
// ---- 落点理由（现读选定 · ✗ 不是偏好）------------------------------------------------------
//
//  1. 本接口要接的两个入口都在 **core 模块**：`zegvVerifyEntry`（`core/cmd/zerg/zerg_egg_verify.go` ·
//     package main · **未导出**）与 `eggstore.Store.{Expand, PrecheckHatch}` ＋ `eggstore.CheckHatchable`
//     （`core/internal/store/eggstore/`）。
//  2. 子端模块 `github.com/Mr2109/zerg-swarm/agent` 的 `agent/go.mod` **既无 core 的 require、也无
//     replace**（现读：仓内无 `go.work`；`grep -rn "swarm/core/" agent` ⇒ 0 命中）⇒ 子端侧件
//     **导不到**上述两个入口。
//  3. ⇒ 「从单文件孵」的编排件只能落 **core**；又因 `zegvVerifyEntry` 未导出 ⇒ 只能在 **package main**。
//     ★ 派单示例给过 `agent/internal/backend/hatch_from_egg.go` —— 现读该落点**构造不出来**（第 2 条）；
//     这条前提校正写在报告 T1/T4。
//
// ---- 四步（顺序是契约，✗ 不可换）----------------------------------------------------------
//
//	① 校验：`zegvVerifyEntry`（唯一入口）⇒ 有 problem ⇒ **在校验处就拒**（✗ 先展开后拒）；
//	② 门   ：名册在册 ＋ 完工标记 ＋ **仅 `verified` 可孵**（`eggstore.CheckHatchable`）＋ 水线预检
//	          （`Store.PrecheckHatch` · ✗ 不改水线数值），任一不过 ⇒ 出声拒；
//	③ 展开：`eggstore.Expand`（★ 库件内：先登记引用 → 再 CoW 整文件）；展开物**去写位**
//	          （✗ 不得把展开物当可写 —— 照 §16.7②「展开物无写位」）；
//	④ 交接：组 `hfeHandoff`（引擎看得到的那一层 ＋ 展开物 ＋ 只读绑定语义）交给孵化器。
//	          ★ 本件**只到「交给孵化器」这一步**，✗ 不拉起推理：拉起在子端孵化器
//	          `agent/internal/backend/hatch_path.go` 的 `hatchStartLocked`（见报告 T1）。
//
// ---- 自检口（★ 本件无 CLI 开关）------------------------------------------------------------
//
// `hfeSelfTest(out, errOut) int` —— 四态 ＋ 两条负控 ＋ 逐格读数；由 `hatch_from_egg_test.go` 的
// `TestHatchFromEggSelfTest` 实跑（同 `eggstore.SelfTestNote`「自检走测试件面」的先例）。
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Mr2109/zerg-swarm/core/internal/store/eggstore"
)

// ─────────────────────────── 逐阶段点名（照 卵4③ 观测表 `fail_stage` 口径的先声）───────────
//
// 「卡在哪一阶段」必须是一等读数：只报「拒了」而不报「在哪拒的」，会让「坏卵被拒」与
// 「态不对被拒」看着一样（§16.3 / L17 的立项理由即此）。
type hfeStage string

const (
	// hfeStageVerify ①校验阶段（zegvVerifyEntry）。
	hfeStageVerify hfeStage = "verify"
	// hfeStageState ②门阶段 · 名册/完工标记/生命周期（仅 verified 可孵）。
	hfeStageState hfeStage = "state"
	// hfeStageWaterline ②门阶段 · 盘面水线（imagefs/nodefs/inode）。
	hfeStageWaterline hfeStage = "waterline"
	// hfeStageExpand ③展开阶段（eggstore.Expand ＋ 展开物去写位）。
	hfeStageExpand hfeStage = "expand"
	// hfeStageHandoff ④交接阶段（组交接件交给孵化器）。
	hfeStageHandoff hfeStage = "handoff"
)

// hfeRefusal 拒孵 —— 一律**逐阶段点名 ＋ 出声**，✗ 不静默降级。
//
// ★ `GAP-20261015-25`：D5/D6a/D10/D11 四档的**报话与退码**取 13 档表同一行（`zegVersionTiers`），
// 本结构多带 `Tier` / `Msg` / `Code` / `Detail` 四格 —— 表改了行为就跟着改，✗ 不在调用点手抄文案。
// 其余阶段（校验/展开/交接）仍走 `Reason` 那句具名说明（它们不属「卵不能孵时怎么报」那 13 档）。
type hfeRefusal struct {
	Stage  hfeStage
	Reason string
	Tier   string // 非空 ⇒ 报话/退码取 13 档表该行（D5 / D6a / D10 / D11）
	Msg    string // 13 档表报话（逐字 · 已按模板填值）
	Code   int    // 13 档表退码
	Detail string // 细节读数（人面补充 · 不进第一行）
}

func (e *hfeRefusal) Error() string {
	if e.Msg != "" {
		if e.Detail != "" {
			return e.Msg + "\n详情（阶段 " + string(e.Stage) + "）：" + e.Detail
		}
		return e.Msg
	}
	return fmt.Sprintf("拒孵（阶段 %s）：%s", string(e.Stage), e.Reason)
}

// hfeFromTierRefusal —— 把一条 13 档表的读端拒（`*zegRefusal`）挂上孵化阶段。
func hfeFromTierRefusal(stage hfeStage, r *zegRefusal) *hfeRefusal {
	if r == nil {
		return &hfeRefusal{Stage: stage, Reason: "（空拒：✗ 不当成放行）"}
	}
	return &hfeRefusal{Stage: stage, Tier: r.Tier, Msg: r.Msg, Code: r.Code, Detail: r.Detail}
}

// hfeAsRefusal —— 任意错 ⇒ 拒孵错（13 档表的拒保留档位与退码；其余按阶段具名）。
func hfeAsRefusal(stage hfeStage, err error) *hfeRefusal {
	if err == nil {
		return nil
	}
	if r, ok := err.(*zegRefusal); ok {
		return hfeFromTierRefusal(stage, r)
	}
	if r, ok := err.(*hfeRefusal); ok {
		return r
	}
	return &hfeRefusal{Stage: stage, Reason: err.Error()}
}

// hfeExitCode —— 拒孵错 → (退码, true)。★ 13 档表的档 ⇒ 取表里的码（D10=8 / D11=10 / D5·D6a=2）；
// 其余拒 ⇒ `exitFail`（一般失败），✗ 不返回 0 冒充放行。
func hfeExitCode(err error) (int, bool) {
	if r, ok := err.(*hfeRefusal); ok {
		if r.Tier != "" {
			return r.Code, true
		}
		return exitFail, true
	}
	if r, ok := err.(*zegRefusal); ok && r.Coded {
		return r.Code, true
	}
	return 0, false
}

// hfeStageOf 取一个错的拒孵阶段（非拒孵错 ⇒ 返回空串，调用方据此区分「算不出来」与「拒了」）。
func hfeStageOf(err error) hfeStage {
	if r, ok := err.(*hfeRefusal); ok {
		return r.Stage
	}
	return ""
}

// ─────────────────────────── 交接件（交给孵化器的全部输入）───────────────────────────

// hfeSegment 卵里的一段在**空间内**的相对路径（＝引擎看得到的那一层）。
// ★ 与子端孵化器挂载配方的对应关系（`/models/<基名>` 一类）由它决定，本件只如实把段表交出去，
// ✗ 不替它编空间内落点。
type hfeSegment struct {
	Path   string `json:"path"`
	Off    int64  `json:"off"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
	Mode   int    `json:"mode"`
}

// hfeHandoff 「从单文件孵」的交接件 —— 交给孵化器的**全部输入**。
type hfeHandoff struct {
	Stage        hfeStage          `json:"stage"`
	Egg          string            `json:"egg"`
	EggPath      string            `json:"egg_path"`
	EggSHA256    string            `json:"egg_sha256"`
	RefID        string            `json:"ref"`
	Expanded     string            `json:"expanded"`
	ReadOnly     bool              `json:"read_only"`
	ExpandedMode uint32            `json:"expanded_mode"`
	Tier         string            `json:"tier"`
	CoW          bool              `json:"cow"`
	Bytes        int64             `json:"bytes"`
	RefsAfter    int               `json:"refs_after"`
	Segments     []hfeSegment      `json:"segments"`
	DataBytes    int64             `json:"data_bytes"`
	Align        int64             `json:"align"`
	CacheHit     bool              `json:"cache_hit"`
	Precheck     eggstore.Precheck `json:"precheck"`
	// ★ GAP-20261015-25 · D12：实际用哪份茧壁（`machine` / `egg` / 空=本档不介入）＋ 观测面标注一行。
	Wall     string `json:"wall"`
	WallNote string `json:"wall_note"`
	// ★ GAP-20261015-25 · D6b：非必需段类型未知 ⇒ 跳过并登记（读端登记面的交接副本）。
	D6bSkipped []string `json:"d6b_skipped,omitempty"`
}

// hfeOptions 「从单文件孵」的入参。
//
// ImagePath / NodePath 是水线判据的**两个卷**（照 卵2③：imagefs ＋ nodefs）；本件只读它们，
// ✗ 不选卷、✗ 不改造水线数值。
type hfeOptions struct {
	EggPath   string
	EggName   string
	RefID     string
	Dst       string
	ImagePath string
	NodePath  string
	Store     *eggstore.Store
	Stdout    io.Writer
	Workers   int
}

// ─────────────────────────── 注入点（照本包既有体例：包级 var ＋ 默认真实现）────────────
//
// 三个都只为「负控能确定性哑化一处闸」而存在（同 `hatchGateRead` / `mainlineEngineProbe` 的先例）：
// 正路恒走真实现，✗ 不引入任何生产分支。
var (
	// hfeVerify ①校验（唯一入口 · 真源 `zegvVerifyEntry`）。
	hfeVerify = zegvVerifyEntry
	// hfeReadIndex 读端索引解析（唯一入口 · 真源 `zegParseIndex` · §11.2 第 6 条「读写同源」）。
	hfeReadIndex = zegParseIndex
	// hfePrecheck 水线预检（唯一入口 · 真源 `Store.PrecheckHatch`）。
	hfePrecheck = func(st *eggstore.Store, imagePath, nodePath string) (eggstore.Precheck, error) {
		return st.PrecheckHatch(imagePath, nodePath)
	}
)

// hatchFromEgg —— 「从单文件孵」的唯一入口：给定一枚 `.egg` ⇒ ① 校验 → ② 门 → ③ 展开 → ④ 交接。
//
// ★ 两条硬约束（写在签名旁边，✗ 不许由调用方绕过）：
//   - **✗ 不得绕开①校验**：校验在**展开之前**，且展开只在①绿之后才被调到（负控「哑化①校验」
//     会当场证出这条顺序有牙）；
//   - **✗ 不得把展开物当可写**：`hfeHandoff.ReadOnly` 恒 true，且展开物落盘后**去掉全部写位**
//     （去不掉 ⇒ 拒，✗ 不带着可写展开物往下走）。
func hatchFromEgg(opts hfeOptions) (hfeHandoff, error) {
	var h hfeHandoff
	out := opts.Stdout
	if out == nil {
		out = io.Discard
	}
	eggPath := strings.TrimSpace(opts.EggPath)
	if eggPath == "" {
		return h, &hfeRefusal{Stage: hfeStageVerify,
			Reason: "卵件路径为空 —— 给定一枚 .egg 才谈得上「从单文件孵」"}
	}

	// ① 校验 —— ★ 唯一入口、✗ 不得绕开；坏卵必须在**这里**就被拒（✗ 不许先展开后拒）
	vres, verr := hfeVerify(out, eggPath, "", opts.Workers)
	if verr != nil {
		return h, &hfeRefusal{Stage: hfeStageVerify,
			Reason: fmt.Sprintf("校验跑不起来（%v）—— 校不过就不孵（fail-closed）", verr)}
	}
	if len(vres.Problems) != 0 {
		return h, &hfeRefusal{Stage: hfeStageVerify, Reason: fmt.Sprintf(
			"坏卵：%d 段不符（点名：%s）—— 在校验处就拒，✗ 不许先展开后拒",
			len(vres.Problems), strings.Join(vres.Problems, " / "))}
	}

	// ①b 读索引（读端唯一入口 `zegParseIndex` —— ✗ 不另写解析器）：后面四档闸都要段表。
	idx, ierr := hfeReadIndex(eggPath)
	if ierr != nil {
		return h, hfeAsRefusal(hfeStageVerify, ierr)
	}
	// ①c D5：契约段缺必需字段（引擎入口/绑定/限额任一为 nil ⇒ 拒，✗ 不默默取默认值）。
	if cerr := hgtContractGate(eggPath, idx); cerr != nil {
		return h, hfeAsRefusal(hfeStageVerify, cerr)
	}
	// ①d D6a / D6b：必需段类型本机不认识 ⇒ 拒；非必需段 ⇒ 跳过并登记（照 OCI「MUST NOT error」）。
	d6b, merr := hgtMediaTypeGate(idx)
	if merr != nil {
		return h, hfeAsRefusal(hfeStageVerify, merr)
	}
	// ①e D12：V1 三段判「用机器那份还是卵内那份」（不报错，结论写进交接件＝观测面标注）。
	wallChoice, wallNote, werr := hgtWallGate(idx)
	if werr != nil {
		return h, &hfeRefusal{Stage: hfeStageState, Reason: werr.Error()}
	}

	// ② 门：在册 ＋ 完工标记 ＋ 仅 verified 可孵 ＋ 水线（都 fail-closed、都出声）
	st := opts.Store
	if st == nil {
		return h, &hfeRefusal{Stage: hfeStageState, Reason: "卵库未打开（✗ 无库不孵）"}
	}
	e, ok := st.Roster.Get(opts.EggName)
	if !ok {
		return h, &hfeRefusal{Stage: hfeStageState,
			Reason: fmt.Sprintf("名册里没有这枚卵 %q（✗ 不许孵不在册的卵）", opts.EggName)}
	}
	// D10：半成品（临时件未 rename ／ 完工标记未置位）⇒ 拒，码 8。
	if herr := hgtHalfRefuse(eggPath, opts.EggName, e.Complete); herr != nil {
		return h, hfeAsRefusal(hfeStageState, herr)
	}
	if err := eggstore.CheckHatchable(opts.EggName, e.State); err != nil {
		return h, &hfeRefusal{Stage: hfeStageState, Reason: err.Error()}
	}
	pre, perr := hfePrecheck(st, opts.ImagePath, opts.NodePath)
	if perr != nil {
		return h, &hfeRefusal{Stage: hfeStageWaterline, Reason: perr.Error()}
	}
	// D11：水线/硬线已到，或本卵展开所需 > 现读可用 ⇒ 拒，码 10（附可回收卵清单）。
	if derr := hgtDiskGate(pre, hgtNeedBytes(idx)); derr != nil {
		return h, derr
	}

	// ③ 展开（★ 库件内顺序：先登记引用 → 再 CoW 整文件）
	rep, xerr := st.Expand(opts.EggName, opts.RefID, opts.Dst, eggstore.RefExpansion, eggstore.CowOptions{})
	if xerr != nil {
		return h, &hfeRefusal{Stage: hfeStageExpand, Reason: xerr.Error()}
	}

	// 展开物**去写位**（✗ 不得把展开物当可写）
	mode, merr2 := hfeMakeReadOnly(rep.Path)
	if merr2 != nil {
		return h, &hfeRefusal{Stage: hfeStageExpand,
			Reason: fmt.Sprintf("展开物去写位失败（✗ 不当可写才交给孵化器）：%v", merr2)}
	}

	// 引擎看得到的那一层：索引段表（同一份读端 `zegParseIndex`，✗ 不另写一份解析器）
	segs := make([]hfeSegment, 0, len(idx.Entries))
	var dataBytes int64
	for _, s := range idx.Entries {
		segs = append(segs, hfeSegment{Path: s.Path, Off: s.Off, Size: s.Size, SHA256: s.SHA256, Mode: s.Mode})
		dataBytes += s.Size
	}

	// ④ 交接件（★ 到这一步就是「交给孵化器」——✗ 不真拉起推理）
	h = hfeHandoff{
		Stage:        hfeStageHandoff,
		Egg:          opts.EggName,
		EggPath:      eggPath,
		EggSHA256:    e.SHA256,
		RefID:        opts.RefID,
		Expanded:     rep.Path,
		ReadOnly:     true,
		ExpandedMode: mode,
		Tier:         string(rep.Tier),
		CoW:          rep.CoW,
		Bytes:        rep.Bytes,
		RefsAfter:    rep.RefsAfter,
		Segments:     segs,
		DataBytes:    dataBytes,
		Align:        idx.Align,
		CacheHit:     vres.CacheHit,
		Precheck:     pre,
		Wall:         wallChoice,
		WallNote:     wallNote,
		D6bSkipped:   d6b,
	}
	return h, nil
}

// hfeMakeReadOnly 把展开物**去掉全部写位**（0644 → 0444 一类），并回报现读 mode。
//
// 为什么在这层做（✗ 不在 eggstore 内部）：§16.7② 的判据是「展开物**无写位**」；展开物是
// **本编排件**落下的交出物 ⇒ 只读纪律由交接面负责，✗ 不改库件语义（卵4 边界）。
// 去不掉（仍有写位）⇒ 报错 ⇒ 调用方拒（✗ 不带着可写展开物往下走）。
func hfeMakeReadOnly(path string) (uint32, error) {
	if strings.TrimSpace(path) == "" {
		return 0, fmt.Errorf("展开物路径为空")
	}
	fi, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	ro := fi.Mode().Perm() &^ 0o222
	if ro != fi.Mode().Perm() {
		if cerr := os.Chmod(path, ro); cerr != nil {
			return uint32(fi.Mode().Perm()), cerr
		}
	}
	got, err := os.Stat(path)
	if err != nil {
		return uint32(ro), err
	}
	perm := uint32(got.Mode().Perm())
	if perm&0o222 != 0 {
		return perm, fmt.Errorf("展开物仍有写位（%04o）—— ✗ 不当可写才交给孵化器", perm)
	}
	return perm, nil
}

// hfeListOrNone 清单为空 ⇒ 明写「（无）」（✗ 不留空 —— 空串会被读成「没读到」）。
func hfeListOrNone(xs []string) string {
	if len(xs) == 0 {
		return "（无）"
	}
	return strings.Join(xs, " / ")
}

// ═══════════════════════════ 自检口（hfeSelfTest）═══════════════════════════
//
// ★ 本件**无 CLI 开关**（✗ 不新造旗标、✗ 不动命令树）⇒ 自检走测试件面，同
// `eggstore.SelfTestNote` 的既有先例：`TestHatchFromEggSelfTest` 调它并断言 rc=0。
// 夹具全在**仓外临时目录**（✗ 不碰真 22 GB 权重、✗ 不碰真卵库）。

// hfeSelfCase 一格自检（`Run` 返回 nil ⇔ 该格断言成立 —— 负控亦然）。
type hfeSelfCase struct {
	Name string
	Run  func() error
}

// hfeSelfTest 跑「从单文件孵」的成对正负控（四态 ＋ 两条负控）。rc=0 ⇔ 全过。
func hfeSelfTest(out, errOut io.Writer) int {
	dir, err := os.MkdirTemp("", "zerg-hfe-")
	if err != nil {
		fmt.Fprintf(errOut, "自检前置失败：建临时目录：%v\n", err)
		return 1
	}
	defer func() { _ = os.RemoveAll(dir) }()
	return hfeSelfTestIn(dir, out, errOut)
}

// hfeSelfTestIn 在给定目录下建夹具并逐格跑（纯夹具 + 逐格，便于单测复用）。
func hfeSelfTestIn(dir string, out, errOut io.Writer) int {
	src := filepath.Join(dir, "src")
	if err := os.MkdirAll(filepath.Join(src, "sub"), 0o755); err != nil {
		fmt.Fprintf(errOut, "自检前置失败：建夹具源目录：%v\n", err)
		return 1
	}
	if err := os.WriteFile(filepath.Join(src, "config.json"), []byte(`{"model":"demo","n":1}`), 0o644); err != nil {
		fmt.Fprintf(errOut, "自检前置失败：写 config.json：%v\n", err)
		return 1
	}
	payload := make([]byte, 4096)
	for i := range payload {
		payload[i] = byte(i % 251)
	}
	if err := os.WriteFile(filepath.Join(src, "sub", "weights.bin"), payload, 0o644); err != nil {
		fmt.Fprintf(errOut, "自检前置失败：写 weights.bin：%v\n", err)
		return 1
	}
	goodEgg := filepath.Join(dir, "good.egg")
	if _, perr := zegPack(src, goodEgg, 0, "2026-10-15T00:00:00Z"); perr != nil {
		fmt.Fprintf(errOut, "自检前置失败：打包好卵：%v\n", perr)
		return 1
	}
	// 坏卵：**段内**中点改 1 字节（✗ 不改补零区 —— 补零区不在索引的 sha 覆盖内，改了不算坏卵）
	badEgg := filepath.Join(dir, "bad.egg")
	if cerr := hfeCopyFile(goodEgg, badEgg); cerr != nil {
		fmt.Fprintf(errOut, "自检前置失败：复制坏卵：%v\n", cerr)
		return 1
	}
	if cerr := hfeCorruptSegment(badEgg); cerr != nil {
		fmt.Fprintf(errOut, "自检前置失败：改坏卵段内字节：%v\n", cerr)
		return 1
	}
	storeRoot := filepath.Join(dir, "store")
	st, oerr := eggstore.Open(storeRoot)
	if oerr != nil {
		fmt.Fprintf(errOut, "自检前置失败：打开卵库：%v\n", oerr)
		return 1
	}
	goodSHA, serr := zegSha256File(goodEgg)
	if serr != nil {
		fmt.Fprintf(errOut, "自检前置失败：算卵 sha：%v\n", serr)
		return 1
	}
	goodFi, ferr := os.Stat(goodEgg)
	if ferr != nil {
		fmt.Fprintf(errOut, "自检前置失败：取卵大小：%v\n", ferr)
		return 1
	}
	if rerr := st.Register(eggstore.Egg{Name: "good", SHA256: goodSHA, Path: goodEgg,
		State: eggstore.StateVerified, Bytes: goodFi.Size(), Complete: true, Blocks: 1}); rerr != nil {
		fmt.Fprintf(errOut, "自检前置失败：登记 good：%v\n", rerr)
		return 1
	}
	if rerr := st.Register(eggstore.Egg{Name: "bad", SHA256: goodSHA, Path: badEgg,
		State: eggstore.StateVerified, Bytes: goodFi.Size(), Complete: true, Blocks: 1}); rerr != nil {
		fmt.Fprintf(errOut, "自检前置失败：登记 bad：%v\n", rerr)
		return 1
	}
	// 非 verified（landed）：Blocks=0 ⇒ Register 会拒（半成品不入名册）⇒ 直接走 Roster.Upsert
	if uerr := st.Roster.Upsert(eggstore.Egg{Name: "unverified", SHA256: goodSHA, Path: goodEgg,
		State: eggstore.StateLanded, Bytes: goodFi.Size(), Complete: true}); uerr != nil {
		fmt.Fprintf(errOut, "自检前置失败：登记 unverified：%v\n", uerr)
		return 1
	}

	waterRefuse := func() *eggstore.Store {
		s2, _ := eggstore.Open(storeRoot)
		s2.Waterline = &eggstore.Waterline{HighUsedPercent: 0, LowUsedPercent: 0,
			ImageFSAvailPercent: 0, NodeFSAvailPercent: 0, InodesFreePercent: 0}
		return s2
	}

	cases := []hfeSelfCase{
		{Name: "态①·正常卵 ⇒ 一路走到「交给孵化器」（✗ 不真拉起推理）", Run: func() error {
			dst := filepath.Join(dir, "exp", "good.egg")
			h, err := hatchFromEgg(hfeOptions{EggPath: goodEgg, EggName: "good", RefID: "r1",
				Dst: dst, ImagePath: dir, NodePath: dir, Store: st, Stdout: io.Discard, Workers: 2})
			if err != nil {
				return fmt.Errorf("正常卵本该走到交接，却被拒：%v", err)
			}
			fmt.Fprintf(out, "      读数：stage=%s read_only=%v expanded_mode=%04o tier=%s cow=%v refs_after=%d segments=%d data_bytes=%d cache_hit=%v expanded=%s\n",
				string(h.Stage), h.ReadOnly, h.ExpandedMode, h.Tier, h.CoW, h.RefsAfter,
				len(h.Segments), h.DataBytes, h.CacheHit, h.Expanded)
			if h.Stage != hfeStageHandoff {
				return fmt.Errorf("stage 应为 handoff，实得 %q", string(h.Stage))
			}
			if !h.ReadOnly {
				return fmt.Errorf("交接件必须标只读（✗ 不得把展开物当可写）")
			}
			if h.ExpandedMode&0o222 != 0 {
				return fmt.Errorf("展开物仍有写位（%04o）", h.ExpandedMode)
			}
			if h.RefsAfter != 1 {
				return fmt.Errorf("展开后引用计数应为 1，实得 %d", h.RefsAfter)
			}
			if n := len(h.Segments); n < 1 {
				return fmt.Errorf("段表为空（应 ≥1），实得 %d", n)
			}
			if _, xerr := os.Stat(h.Expanded); xerr != nil {
				return fmt.Errorf("交接件点名的展开物不在盘：%v", xerr)
			}
			return nil
		}},
		{Name: "态②·坏卵 ⇒ 在①校验处就拒（✗ 不得先展开后拒）", Run: func() error {
			dst := filepath.Join(dir, "exp", "bad.egg")
			_, err := hatchFromEgg(hfeOptions{EggPath: badEgg, EggName: "bad", RefID: "r2",
				Dst: dst, ImagePath: dir, NodePath: dir, Store: st, Stdout: io.Discard, Workers: 2})
			if err == nil {
				return fmt.Errorf("坏卵本该被拒，却一路放行")
			}
			fmt.Fprintf(out, "      读数：%v\n", err)
			if got := hfeStageOf(err); got != hfeStageVerify {
				return fmt.Errorf("坏卵应在校验处被拒（stage=verify），实得 stage=%q", string(got))
			}
			if _, serr := os.Stat(dst); serr == nil {
				return fmt.Errorf("坏卵被拒前**展开目录已建**（%s）—— 先展开后拒，✗ 不许", dst)
			}
			return nil
		}},
		{Name: "态③·非 verified（landed）⇒ 拒孵（照 #13 仅 verified 可孵）", Run: func() error {
			_, err := hatchFromEgg(hfeOptions{EggPath: goodEgg, EggName: "unverified", RefID: "r3",
				Dst: filepath.Join(dir, "exp", "unverified.egg"), ImagePath: dir, NodePath: dir,
				Store: st, Stdout: io.Discard, Workers: 2})
			if err == nil {
				return fmt.Errorf("非 verified 的卵本该被拒，却一路放行")
			}
			fmt.Fprintf(out, "      读数：%v\n", err)
			if got := hfeStageOf(err); got != hfeStageState {
				return fmt.Errorf("应在门（state）处被拒，实得 stage=%q", string(got))
			}
			return nil
		}},
		{Name: "态④·盘面到水线 ⇒ 拒孵（照 卵2③ 口径 · 真读真盘 × 注入阈值）", Run: func() error {
			_, err := hatchFromEgg(hfeOptions{EggPath: goodEgg, EggName: "good", RefID: "r4",
				Dst: filepath.Join(dir, "exp", "waterline.egg"), ImagePath: dir, NodePath: dir,
				Store: waterRefuse(), Stdout: io.Discard, Workers: 2})
			if err == nil {
				return fmt.Errorf("到水线本该被拒，却一路放行")
			}
			fmt.Fprintf(out, "      读数：%v\n", err)
			if got := hfeStageOf(err); got != hfeStageWaterline {
				return fmt.Errorf("应在水线（waterline）处被拒，实得 stage=%q", string(got))
			}
			return nil
		}},
		{Name: "负控·哑化①校验 ⇒ 态② 必红（且展开目录被建 ⇒ 证「先校验后展开」这条闸在①校验手里）", Run: func() error {
			saved := hfeVerify
			hfeVerify = func(io.Writer, string, string, int) (zegvVerifyResult, error) {
				return zegvVerifyResult{}, nil // 哑化：恒报「无问题」
			}
			defer func() { hfeVerify = saved }()
			dst := filepath.Join(dir, "exp", "bad-nc.egg")
			h, err := hatchFromEgg(hfeOptions{EggPath: badEgg, EggName: "bad", RefID: "r5",
				Dst: dst, ImagePath: dir, NodePath: dir, Store: st, Stdout: io.Discard, Workers: 2})
			_, dstErr := os.Stat(dst)
			fmt.Fprintf(out, "      读数：哑化后 err=%v stage=%q 展开目录已建=%v handoff.stage=%q\n",
				err, string(hfeStageOf(err)), dstErr == nil, string(h.Stage))
			if hfeStageOf(err) == hfeStageVerify {
				return fmt.Errorf("哑化①校验后坏卵仍在校验处被拒 ⇒ 前者不是①校验在管（负控不成立）")
			}
			if err != nil {
				return fmt.Errorf("哑化①校验后本应一路走到交接，却仍被拒：%v", err)
			}
			if dstErr != nil {
				return fmt.Errorf("哑化①校验后展开目录仍没建 ⇒ 这条负控没测到「先校验后展开」的顺序")
			}
			if h.Stage != hfeStageHandoff {
				return fmt.Errorf("哑化后应走到交接，实得 stage=%q", string(h.Stage))
			}
			return nil
		}},
		{Name: "负控·哑化②水线判据 ⇒ 态④ 必红（证水线拒在②门手里）", Run: func() error {
			saved := hfePrecheck
			hfePrecheck = func(*eggstore.Store, string, string) (eggstore.Precheck, error) {
				return eggstore.EvalWaterline(eggstore.DefaultWaterline,
					eggstore.Sample{Label: "imagefs", Blocks: 100, Bavail: 100},
					eggstore.Sample{Label: "nodefs", Blocks: 100, Bavail: 100}, nil), nil
			}
			defer func() { hfePrecheck = saved }()
			_, err := hatchFromEgg(hfeOptions{EggPath: goodEgg, EggName: "good", RefID: "r6",
				Dst: filepath.Join(dir, "exp", "waterline-nc.egg"), ImagePath: dir, NodePath: dir,
				Store: waterRefuse(), Stdout: io.Discard, Workers: 2})
			fmt.Fprintf(out, "      读数：哑化后 err=%v stage=%q\n", err, string(hfeStageOf(err)))
			if hfeStageOf(err) == hfeStageWaterline {
				return fmt.Errorf("哑化②水线后仍在水线处被拒 ⇒ 该闸不是②水线在管（负控不成立）")
			}
			if err != nil {
				return fmt.Errorf("哑化②水线后本应一路走到交接（注入阈值已失效），却仍被拒：%v", err)
			}
			return nil
		}},
	}
	return hfeRunCases(cases, out)
}

// hfeRunCases 逐格跑并打印读数/结论；rc=0 ⇔ 全过。
func hfeRunCases(cases []hfeSelfCase, out io.Writer) int {
	fail := 0
	for i, c := range cases {
		err := c.Run()
		if err == nil {
			fmt.Fprintf(out, "  [%d] PASS %s\n", i+1, c.Name)
			continue
		}
		fail++
		fmt.Fprintf(out, "  [%d] FAIL %s —— %v\n", i+1, c.Name, err)
	}
	fmt.Fprintf(out, "  自检：%d 格 · %d 通过 · %d 失败\n", len(cases), len(cases)-fail, fail)
	if fail != 0 {
		return 1
	}
	return 0
}

// hfeCopyFile 整件复制（自检夹具用 · 仓外临时目录）。
func hfeCopyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	outf, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, cerr := io.Copy(outf, in); cerr != nil {
		_ = outf.Close()
		return cerr
	}
	return outf.Close()
}

// hfeCorruptSegment 在**段内**中点改 1 字节（异或 0xff ⇒ 必与原字节不同 ⇒ sha 必变）。
func hfeCorruptSegment(egg string) error {
	idx, err := zegParseIndex(egg)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(egg, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	for _, e := range idx.Entries {
		if e.Size < 4 {
			continue
		}
		off := e.Off + e.Size/2
		buf := make([]byte, 1)
		if _, rerr := f.ReadAt(buf, off); rerr != nil {
			return rerr
		}
		buf[0] ^= 0xff
		if _, werr := f.WriteAt(buf, off); werr != nil {
			return werr
		}
		return nil
	}
	return fmt.Errorf("没有可改的段（都太短）")
}
