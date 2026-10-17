// eggformat_parity.go —— F24 的**只报告档**（✗ 不是门 · ✗ 不阻断 · ✗ 不改行为）：
// 把「同一份 `*.egg` 格式在两侧读端（A＝子端 eggpkg · B＝主控 zerg_egg.go）分叉」的那些格，
// 钉成**可复跑读数** —— 一条命令（`eggspecgen --parity`）逐格印出「格号 ＋ 格式面 ＋ 两侧现读值」，
// 并**现读校验**每格的拄证锚（file:line 上仍含那句 code ⇒ 锚活；否则标 anchor_ok=false ⇒ 读数已腐）。
//
// ★ 过渡期口径（用户 2026-10-15 拍定）：本档**先落**，把分叉钉成可复跑读数；**再**删旧判据。
// ★ 本件**不新增判据、不改两侧任何行为**（只读两侧源码 + 拼读数）。
package eggpkg

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EggParityCell —— 一格 A/B 分叉的读数（两侧各一条拄证锚 · 现读校验其活性）。
type EggParityCell struct {
	No       int    `json:"no"`             // 格号
	Field    string `json:"field"`          // 格式面
	SideA    string `json:"a_eggpkg"`       // 子端 eggpkg 现读值
	SideB    string `json:"b_zerg_egg"`     // 主控 zerg_egg.go 现读值
	AAnchor  string `json:"a_anchor"`       // "file:line"
	BAnchor  string `json:"b_anchor"`       // "file:line"
	AnchorOK bool   `json:"anchor_ok"`      // 两侧锚现读都点得住 ⇒ true（✗ 否则读数已腐）
	Note     string `json:"note,omitempty"` // 与 F21 的差（若有）
}

// ★ 2026-10-18 重锚（只改行号、**判据语义一字未动**）：`core/cmd/zerg/zerg_egg.go` 在
// 「引擎闭包 · sysroot」（`--engine-dir`）那一批里新增了若干行 ⇒ 本表 B 侧 10 个 `file:line` 锚
// 整体下移（前段 +5 / v2 打包段之后 +28）。锚的判据仍是「该文件第 N 行仍含这句 code」——
// 逐条按现读重算（不是放宽：`anchorLive` 一字未改）。
// ★ 2026-10-18 二次重锚（卵沙箱第 6 步 · `--libs-dir` ＋ `zegPackV2Tree`）：同一件前部再 +3 行、
// 打包段之后再 +28 行 ⇒ 同 10 个 B 锚再整体下移（+3 / +28），语义词与 A 侧锚**一格未动**。
// 判据仍是 `anchorLive`（该行仍含这句 code），✗ 没有放宽成「含关键词即可」。
// ★ 2026-10-18 三次重锚（同一笔 · `zegSegOfPath` 补 `libs/**` 与 `engine/**` 同类）：该函数在原
// 位置 +4 行 ⇒ 同 10 个 B 锚再整体 +4。
type parityAnchor struct {
	file string
	line int
	sub  string
}

func (a parityAnchor) label() string { return fmt.Sprintf("%s:%d", a.file, a.line) }

type parityEntry struct {
	no    int
	field string
	a     string
	b     string
	aAn   parityAnchor
	bAn   parityAnchor
	note  string
}

// eggParityEntries —— 14 格分叉（★ 本单**现读重核**得出的格号 · ✗ 不照抄 F21）。
//
// 面：A＝`agent/internal/eggpkg/eggpkg.go`（子端读端 · 只读解析）· B＝`core/cmd/zerg/zerg_egg.go`（主控读端）。
// 分叉方向分两类：**A 更严**（B 读端期不判：段偏移/越界/长度/非空）与 **A 落后**（B 有点名档位/内容比对/安全子集）。
func eggParityEntries() []parityEntry {
	return []parityEntry{
		{no: 1, field: "前缀 align ⟷ 索引 align 交叉核对",
			a:   "核对：索引里 align 必须等于前缀 align，不符 ⇒ 拒",
			b:   "不核对：前缀与索引体各自取值、各自验，无交叉比对",
			aAn: parityAnchor{"agent/internal/eggpkg/eggpkg.go", 106, "idx.Align != align"},
			bAn: parityAnchor{"core/cmd/zerg/zerg_egg.go", 1743, "raw.Align < zegMinAlign"}},
		{no: 2, field: "读端 path 规则集",
			a:    "拒 {空 / 绝对 / 非 Clean / 含 .. / 重复}（五类）",
			b:    "拒 zegPathRules 十二类（另含 Windows 保留名 / 尾随点 / 反斜杠 / NUL / 非 NFC 等）",
			aAn:  parityAnchor{"agent/internal/eggpkg/eggpkg.go", 121, `e.Path == ""`},
			bAn:  parityAnchor{"core/cmd/zerg/zerg_egg.go", 458, "func zegRefusePath(rel string) error {"},
			note: "★ 与 F21 差：F21 读得当年 B「不校验 path」；现读 B 已接卵1② 路径规则表 ⇒ 方向反转（现 B 更严）"},
		{no: 3, field: "段 off 对齐（读端期）",
			a:   "读端拒：off%align != 0 ⇒ 拒",
			b:   "读端不判：仅 zegVerify 的 problems 面报",
			aAn: parityAnchor{"agent/internal/eggpkg/eggpkg.go", 134, "e.Off%ix.Align != 0"},
			bAn: parityAnchor{"core/cmd/zerg/zerg_egg.go", 1853, "e.Off%align != 0"}},
		{no: 4, field: "段 size 越界（读端期）",
			a:   "读端拒：off+size > 文件长 ⇒ 拒",
			b:   "读端不判：仅 zegVerify 的 problems 面报",
			aAn: parityAnchor{"agent/internal/eggpkg/eggpkg.go", 137, "e.Off+e.Size > fileSize"},
			bAn: parityAnchor{"core/cmd/zerg/zerg_egg.go", 1856, "e.Off+e.Size > sizeFile"}},
		{no: 5, field: "段 sha256 长度 = 64（读端期）",
			a:   "读端拒：len(sha256) != 64 ⇒ 拒",
			b:   "读端不判长度：verify 比内容，长度不符落 problems 面 sha 不符",
			aAn: parityAnchor{"agent/internal/eggpkg/eggpkg.go", 140, "len(e.SHA256) != 64"},
			bAn: parityAnchor{"core/cmd/zerg/zerg_egg.go", 1881, "h.Sum(nil)) != e.SHA256"}},
		{no: 6, field: "段 sha256 内容比对",
			a:   "不比对内容：只校长度 64",
			b:   "比对内容：zegVerify 逐段现算 sha 与索引比",
			aAn: parityAnchor{"agent/internal/eggpkg/eggpkg.go", 140, "len(e.SHA256) != 64"},
			bAn: parityAnchor{"core/cmd/zerg/zerg_egg.go", 1881, "h.Sum(nil)) != e.SHA256"}},
		{no: 7, field: "段 mode 值域（安全子集）",
			a:    "不判值：mode 原样收（无值域判据）",
			b:    "拒：mode 须 ∈ {0644,0755,0700} 安全子集，禁 setuid/setgid/sticky",
			aAn:  parityAnchor{"agent/internal/eggpkg/eggpkg.go", 38, "Mode   uint32"},
			bAn:  parityAnchor{"core/cmd/zerg/zerg_egg.go", 699, "if !zegModeSafeSet[e.Mode] {"},
			note: "★ 与 F21 差：F21 读得当年「两份都未实现」（⚠A⚠B 两格）；现读 B 已接安全子集 ⇒ 收敛成 A 侧一格"},
		{no: 8, field: "entries 非空",
			a:   "读端拒：len(entries) == 0 ⇒ 拒",
			b:   "读端不拒：空表审计 0 迭代 ⇒ 收",
			aAn: parityAnchor{"agent/internal/eggpkg/eggpkg.go", 116, "if len(ix.Entries) == 0 {"},
			bAn: parityAnchor{"core/cmd/zerg/zerg_egg.go", 689, "for _, e := range idx.Entries {"}},
		{no: 9, field: "不可变面必需键缺项",
			a:   "只对 entries 缺项泛化拒（\"索引里没有段\"）；缺 magic/version/align 不判",
			b:   "具名 D5 逐键拒：zegAuditIndexSurface 点出缺哪个不可变面键",
			aAn: parityAnchor{"agent/internal/eggpkg/eggpkg.go", 116, "if len(ix.Entries) == 0 {"},
			bAn: parityAnchor{"core/cmd/zerg/zerg_egg_version.go", 369, "if _, has := top[spec.Key]; !has {"}},
		{no: 10, field: "禁同一键换语义",
			a:   "泛化解码错（无名）：json.Decode 的类型错串",
			b:   "具名 zegTierSurface：zegAuditKeySet 点「键 X 类型由 A 变成 B」",
			aAn: parityAnchor{"agent/internal/eggpkg/eggpkg.go", 103, "if err := dec.Decode(idx); err != nil {"},
			bAn: parityAnchor{"core/cmd/zerg/zerg_egg_version.go", 338, "return &zegRefusal{Tier: zegTierSurface, Msg: fmt.Sprintf("}},
		{no: 11, field: "min_readable 牙",
			a:   "忽略：无该键的概念",
			b:   "拒 D3 具名：zegCompatGateMinReadable（卵声明 min_readable > 本端主号 ⇒ 拒）",
			aAn: parityAnchor{"agent/internal/eggpkg/eggpkg.go", 144, "return nil"},
			bAn: parityAnchor{"core/cmd/zerg/zerg_egg_version.go", 432, "if eggMinReadable > localMain {"}},
		{no: 12, field: "签名块世代（v2 世代起）＝ 主号 3",
			a:   "泛化「格式版本不支持：N」",
			b:   "拒 D2 具名：zegCompatGateMainVersion 点「签名块 v2 世代 ⇒ 旧读端必拒」；★ 2026-10-18 主号 2 转给段表化 ⇒ 签名块落 3",
			aAn: parityAnchor{"agent/internal/eggpkg/eggpkg.go", 87, `fmt.Errorf("格式版本不支持：%d", ver)`},
			bAn: parityAnchor{"core/cmd/zerg/zerg_egg_version.go", 409, "if mainVer == zegFormatVersionSigBlock {"}},
		{no: 13, field: "主号前向 / 过旧（D2 / D3）",
			a:   "单句泛化：ver != 1 ⇒ 「格式版本不支持：%d」（无前向/过旧之分）",
			b:   "具名 D2 前向 / D3 过旧：zegCompatGateMainVersion（读端两处挂点）",
			aAn: parityAnchor{"agent/internal/eggpkg/eggpkg.go", 87, `fmt.Errorf("格式版本不支持：%d", ver)`},
			bAn: parityAnchor{"core/cmd/zerg/zerg_egg.go", 1699, "zegCompatGateMainVersion(ver, zegVersionSitePrefix)"}},
		{no: 14, field: "索引体 magic 取值 / 存在",
			a:    "解出但不校验：ReadIndex 无 idx.Magic 判据 ⇒ 索引 magic=XXXX 也收",
			b:    "校验：raw.Magic != zegMagic ⇒ 拒（且面审计先判「缺 magic 键 ⇒ D5」）",
			aAn:  parityAnchor{"agent/internal/eggpkg/eggpkg.go", 112, "return idx, nil"},
			bAn:  parityAnchor{"core/cmd/zerg/zerg_egg.go", 1736, "if raw.Magic != zegMagic {"},
			note: "★ 与 F21 差：F21 未列此格（其 14 格含「行 17」两格）；本单现读把「索引 magic 不校」补入、把行 17 收敛成一格 ⇒ 净仍 14 格"},
	}
}

// anchorLive —— 现读校验一条拄证锚：该件第 line 行（1 基）仍含 sub（子串）⇒ true。
func anchorLive(repoRoot string, a parityAnchor) bool {
	b, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(a.file)))
	if err != nil {
		return false
	}
	lines := strings.Split(string(b), "\n")
	if a.line < 1 || a.line > len(lines) {
		return false
	}
	return strings.Contains(lines[a.line-1], a.sub)
}

// EggParityReport —— 现读重核 14 格分叉：逐格给「格号 ＋ 格式面 ＋ 两侧现读值 ＋ 拄证锚活性」。
func EggParityReport(repoRoot string) []EggParityCell {
	ents := eggParityEntries()
	out := make([]EggParityCell, 0, len(ents))
	for _, e := range ents {
		ok := anchorLive(repoRoot, e.aAn) && anchorLive(repoRoot, e.bAn)
		out = append(out, EggParityCell{
			No: e.no, Field: e.field, SideA: e.a, SideB: e.b,
			AAnchor: e.aAn.label(), BAnchor: e.bAn.label(), AnchorOK: ok, Note: e.note,
		})
	}
	return out
}

// EggParityJSON —— 上者的 JSON 形（给命令面打印）。
func EggParityJSON(repoRoot string) ([]byte, error) {
	cells := EggParityReport(repoRoot)
	bad := 0
	for _, c := range cells {
		if !c.AnchorOK {
			bad++
		}
	}
	doc := struct {
		Schema     string          `json:"schema"`
		Cells      []EggParityCell `json:"cells"`
		CellCount  int             `json:"cell_count"`
		StaleAnch  int             `json:"stale_anchors"`
		ReportOnly bool            `json:"report_only"`
	}{
		Schema: "zerg-egg-ab-parity/1", Cells: cells, CellCount: len(cells),
		StaleAnch: bad, ReportOnly: true,
	}
	return json.MarshalIndent(doc, "", "  ")
}
