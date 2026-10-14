// family_gap_diff.go —— `zerg gap diff`（缺口 `GAP-20261013-20` 的**前置半**：差集核心 ＋ 行为契约）。
//
// ★ 本件**尚未接进命令树**（`main.go` 的 `commands` 表里现读**没有** `gap diff`）—— 按派单口径，命令树接线与
// `--help` / `cli-matrix` / 基线随动**归另一单**。本件只落「可独立的半」：差集核心（三类差）＋ 退码 / 输出形
// 契约 ＋ 定向四态（`family_gap_diff_test.go`）。接线随动清单逐条见本件尾「接线随动清单（归另单）」。
//
// 输入与格式（两份账 / 两份导出）：
//
//   - **账**   = 缺口真源（`<状态目录>/zerg-cli-gaps.jsonl`）的**任意一份拷贝**（每行一条 `gapRecord`，
//     读法与该真源 `readGapLedger` 同尺：非空行必须是 JSON）；
//   - **导出** = `zerg gap export` 的人面片段（一行形如「连字符 + 反引号 id · PRIO · 加粗 state · symptom ·
//     想要 + 反引号 want · 判据 + 反引号 verify_cmd」），见 `family_gap_export.go` ⑤ 段；
//   - **格式自动识别**：首个非空行以 `{` 开头 ⇒ 账；否则 ⇒ 导出。
//
// 三类差（`kind` 闭集 · 逐条可回溯到 `id` ＋ 哪一格）：
//
//	① `only_left`  —— 仅左：id 只出现在左件；
//	② `only_right` —— 仅右：id 只出现在右件；
//	③ `body_diff`  —— 同 id 不同体：两侧同 id，可比格里有格不同 ⇒ **逐格**一行（`field` 点名哪一格）。
//
// 「可比格」= 两侧**都承载**的格（账⟷账 = 账里出现过的全部格；账⟷导出 = 导出面那几格）。
// 只一侧带的格（如导出件不承载 `unit` / `fp` / `handmade`）**不计差异**，但进 `ExcludedKeys`
// 名单并在人面点名（✗ 不静默 —— 否则「账⟷导出」一对比就全红）。
//
// 退码（本面契约 · 逐条）：
//
//	0 = 两输入可比且 **无差异**（含 `--dry-run` 档）；
//	1 = 两输入可比且 **有差异**（逐条点名）；
//	2 = 输入取不到 / 格式不对 / 用法错 ⇒ **不给结论**（✗ 不假绿）。
//
//	★ 与同族 `gap export` / `gap ls` 的「真源读不到 ⇒ 8」**不同码**：本面是「两份**外部件**的对拍」，
//	  缺件/坏件属**前置/用法面**（派单口径 = 2）⇒ 取 `exitUsage`。接线那一单须在
//	  `cli-contract-baseline` / 矩阵 / docs 里把这条码口径登记清楚（见本件尾随动清单第 3 条）。
//
// `--json`：包封 `items` = 差异逐格 `{id, kind, field, left, right}`（无差异 ⇒ `[]` · rc=0）。
// `--out <路径>`（D2 写面）：把**报告件**写进该路径（✗ 不写任何真源 / 审计）；缺 `--yes` ⇒ 2；
// `--dry-run` ⇒ 计划件走 stdout、**一个字节不落**（rc 仍照真跑 = 0/1 —— 判定是结果，`--dry-run`
// 只抑制落盘，不改判定）。
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
)

// gapDiffArity —— 本命令要的**位置参数枚数**（左件 + 右件）。
const gapDiffArity = 2

// gapDiffFields —— `--json` 可取字段（差异逐格一行：id/kind/field/left/right）。
var gapDiffFields = []string{"id", "kind", "field", "left", "right"}

// 三类差（闭集 · 与「接线随动清单」里要登记的 `kind` 值逐字一致）。
const (
	gapDiffKindOnlyLeft  = "only_left"
	gapDiffKindOnlyRight = "only_right"
	gapDiffKindBodyDiff  = "body_diff"
)

// 两类输入格式（自动识别）。
const (
	gapDiffFormatLedger = "ledger"
	gapDiffFormatExport = "export"
)

// gapDiffExportLine —— 导出件的一行（`family_gap_export.go` ⑤ 段逐字形态）。
// 组：1=id · 2=prio · 3=state · 4=symptom · 5=want · 6=verify_cmd。
// 不匹配的行（件头 `#` / `>` / 页脚一类）**跳过** ⇒ 导出件只有这几行参与对拍。
var gapDiffExportLine = regexp.MustCompile("^\\s*-\\s+`([^`]+)`\\s+(\\S+)\\s+·\\s+\\*\\*([^*]+)\\*\\*\\s+·\\s+(.*?)\\s+·\\s+想要\\s+`([^`]*)`\\s+·\\s+判据\\s+`([^`]*)`\\s*$")

// gapDiffCell —— 一处差异（**可回溯**：`ID` ＋ `Kind`；同 id 不同体时 `Field` 点名哪一格）。
type gapDiffCell struct {
	ID        string
	Kind      string
	Field     string
	Left      string
	Right     string
	LeftLine  int // 左件里的行号（仅左 / 同 id 不同体）
	RightLine int // 右件里的行号（仅右 / 同 id 不同体）
}

// gapDiffRec —— 归一后的一条记录（`Cells` = 格名 → 格文本；只收非空格）。
type gapDiffRec struct {
	ID    string
	Cells map[string]string
	Line  int
}

// gapDiffSide —— 读进来的一侧（原样路径 + 格式 + 逐条记录 + 该侧承载过的格名集）。
type gapDiffSide struct {
	Path   string
	Format string
	Recs   []gapDiffRec
	Keys   map[string]bool
}

// gapDiffResult —— 差集结果（三类差 + 可比格/非可比格名单）。
type gapDiffResult struct {
	LeftPath    string
	RightPath   string
	LeftFormat  string
	RightFormat string
	LeftN       int
	RightN      int
	OnlyLeft    []gapDiffCell
	OnlyRight   []gapDiffCell
	BodyDiffs   []gapDiffCell
	// ComparedKeys —— 两侧**都承载**的格（逐格比这些）。
	ComparedKeys []string
	// ExcludedKeys —— 只一侧承载的格（**不计差异** · 读面点名，✗ 不静默）。
	ExcludedKeys []string
}

// Cells —— 三类差异按「仅左 → 仅右 → 同 id 不同体」的稳定次序拼成一条序列。
func (r gapDiffResult) Cells() []gapDiffCell {
	out := make([]gapDiffCell, 0, len(r.OnlyLeft)+len(r.OnlyRight)+len(r.BodyDiffs))
	out = append(out, r.OnlyLeft...)
	out = append(out, r.OnlyRight...)
	out = append(out, r.BodyDiffs...)
	return out
}

// HasDiff —— 三类差里任何一类非空即「有差异」。
func (r gapDiffResult) HasDiff() bool {
	return len(r.OnlyLeft)+len(r.OnlyRight)+len(r.BodyDiffs) > 0
}

// gapDiffFormatLabel —— 格式的**人面**名（读面用）。
func gapDiffFormatLabel(f string) string {
	if f == gapDiffFormatExport {
		return "导出件"
	}
	return "账"
}

// gapDiffDetectFormat —— 首个非空行以 `{` 开头 ⇒ 账；否则 ⇒ 导出。
// 空件（无非空行）也判「账」⇒ 随后在装载里因 0 条记录拒收（用法错 2）。
func gapDiffDetectFormat(b []byte) string {
	for _, ln := range strings.Split(string(b), "\n") {
		if strings.TrimSpace(ln) == "" {
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(ln), "{") {
			return gapDiffFormatLedger
		}
		return gapDiffFormatExport
	}
	return gapDiffFormatLedger
}

// gapDiffLedgerCells —— 账行 → 归一格面（只收非空格；键名稳定，供 `--json` 的 `field` 列点名）。
// `want` 走 `gapWantText`（与导出件的 `想要` 同尺 ⇒ 账⟷导出 在这一格可比）。
func gapDiffLedgerCells(r gapRecord) map[string]string {
	m := map[string]string{}
	put := func(k, v string) {
		if strings.TrimSpace(v) != "" {
			m[k] = v
		}
	}
	put("symptom", r.Symptom)
	put("handmade", r.Handmade)
	put("impact", r.Impact)
	put("prio", r.Prio)
	put("state", r.State)
	put("want", gapWantText(r))
	put("want_family", r.WantFam)
	put("want_action", r.WantAct)
	put("want_argv", strings.Join(r.WantArgv, " "))
	put("repro_cmd", r.ReproCmd)
	put("verify_cmd", r.VerifyCmd)
	put("depends_on", strings.Join(r.DependsOn, " "))
	put("found_at", r.FoundAt)
	put("fp", r.FP)
	put("unit", r.Unit)
	put("module", r.Module)
	put("unit_source", r.UnitSource)
	put("solved_kind", r.SolvedKind)
	put("remainder", r.Remainder)
	put("triage_batch", r.TriageBatch)
	return m
}

// gapDiffParseExportLine —— 导出件的一行 → (id, 格面, 是否命中)。
// 命中行的格面 = 导出面承载的那几格（`prio`/`state`/`symptom`/`want`/`verify_cmd`）。
func gapDiffParseExportLine(line string) (string, map[string]string, bool) {
	m := gapDiffExportLine.FindStringSubmatch(line)
	if m == nil {
		return "", nil, false
	}
	id := strings.TrimSpace(m[1])
	if id == "" {
		return "", nil, false
	}
	cells := map[string]string{}
	for k, v := range map[string]string{
		"prio": m[2], "state": m[3], "symptom": m[4], "want": m[5], "verify_cmd": m[6],
	} {
		if strings.TrimSpace(v) != "" {
			cells[k] = v
		}
	}
	return id, cells, true
}

// gapDiffLoadFile —— 读一侧（格式自动识别 + 逐行校验）。
// fail-closed（本面口径 = 退码 2）：取不到 / 账某行不是 JSON / 账某行缺 `id` / 0 条记录 /
// 同侧 id 重复 ⇒ **一律 error**（不给结论 · ✗ 不静默当空）。
func gapDiffLoadFile(path string) (gapDiffSide, error) {
	s := gapDiffSide{Path: path, Keys: map[string]bool{}}
	b, err := os.ReadFile(path)
	if err != nil {
		return s, fmt.Errorf("取不到（%v）", err)
	}
	s.Format = gapDiffDetectFormat(b)
	for i, ln := range strings.Split(string(b), "\n") {
		if strings.TrimSpace(ln) == "" {
			continue
		}
		switch s.Format {
		case gapDiffFormatLedger:
			var r gapRecord
			if err := json.Unmarshal([]byte(ln), &r); err != nil {
				return s, fmt.Errorf("账第 %d 行不是 JSON（%v）", i+1, err)
			}
			if strings.TrimSpace(r.ID) == "" {
				return s, fmt.Errorf("账第 %d 行缺 `id`", i+1)
			}
			s.Recs = append(s.Recs, gapDiffRec{ID: r.ID, Cells: gapDiffLedgerCells(r), Line: i + 1})
		case gapDiffFormatExport:
			id, cells, ok := gapDiffParseExportLine(ln)
			if !ok {
				continue
			}
			s.Recs = append(s.Recs, gapDiffRec{ID: id, Cells: cells, Line: i + 1})
		}
	}
	if len(s.Recs) == 0 {
		return s, fmt.Errorf("%s面 0 条记录（空件 / 解析不出）", gapDiffFormatLabel(s.Format))
	}
	seen := map[string]bool{}
	for _, rec := range s.Recs {
		if seen[rec.ID] {
			return s, fmt.Errorf("同侧 id 重复：%s（对拍要求一侧一条）", rec.ID)
		}
		seen[rec.ID] = true
	}
	for _, rec := range s.Recs {
		for k := range rec.Cells {
			s.Keys[k] = true
		}
	}
	return s, nil
}

// gapDiffCompute —— 差集核心（纯函数：只读两份装载结果，不碰盘）。
func gapDiffCompute(left, right gapDiffSide) gapDiffResult {
	res := gapDiffResult{
		LeftPath: left.Path, RightPath: right.Path,
		LeftFormat: left.Format, RightFormat: right.Format,
		LeftN: len(left.Recs), RightN: len(right.Recs),
	}
	// 可比格 = 两侧都承载；非可比格 = 只一侧承载（点名，✗ 不计差异）。
	for k := range left.Keys {
		if right.Keys[k] {
			res.ComparedKeys = append(res.ComparedKeys, k)
		} else {
			res.ExcludedKeys = append(res.ExcludedKeys, k)
		}
	}
	for k := range right.Keys {
		if !left.Keys[k] {
			res.ExcludedKeys = append(res.ExcludedKeys, k)
		}
	}
	sort.Strings(res.ComparedKeys)
	sort.Strings(res.ExcludedKeys)

	rmap := map[string]gapDiffRec{}
	for _, r := range right.Recs {
		rmap[r.ID] = r
	}
	lid := map[string]bool{}
	for _, r := range left.Recs {
		lid[r.ID] = true
	}
	for _, lr := range left.Recs {
		rr, ok := rmap[lr.ID]
		if !ok {
			res.OnlyLeft = append(res.OnlyLeft, gapDiffCell{ID: lr.ID, Kind: gapDiffKindOnlyLeft, LeftLine: lr.Line})
			continue
		}
		for _, k := range res.ComparedKeys {
			lv, rv := lr.Cells[k], rr.Cells[k]
			if lv != rv {
				res.BodyDiffs = append(res.BodyDiffs, gapDiffCell{
					ID: lr.ID, Kind: gapDiffKindBodyDiff, Field: k,
					Left: lv, Right: rv, LeftLine: lr.Line, RightLine: rr.Line,
				})
			}
		}
	}
	for _, rr := range right.Recs {
		if !lid[rr.ID] {
			res.OnlyRight = append(res.OnlyRight, gapDiffCell{ID: rr.ID, Kind: gapDiffKindOnlyRight, RightLine: rr.Line})
		}
	}
	return res
}

// gapDiffKeysText —— 格名清单的人面文本（空 ⇒ 明说「（无）」）。
func gapDiffKeysText(keys []string) string {
	if len(keys) == 0 {
		return "（无）"
	}
	return strings.Join(keys, ", ")
}

// gapDiffValText —— 格值的人面文本（空 ⇒ `（空）`；换行折成 `⏎` 免撑破一行）。
func gapDiffValText(v string) string {
	if v == "" {
		return "（空）"
	}
	return strings.ReplaceAll(strings.ReplaceAll(v, "\n", "⏎"), "\r", "")
}

// gapDiffHumanReport —— 人面报告（人面与 `--json` **同一份差集**）；无差异时打出逐字「无差异」。
func gapDiffHumanReport(w io.Writer, res gapDiffResult) {
	fmt.Fprintf(w, "zerg gap diff · 左 %s（%s %d 条）· 右 %s（%s %d 条）\n",
		res.LeftPath, gapDiffFormatLabel(res.LeftFormat), res.LeftN,
		res.RightPath, gapDiffFormatLabel(res.RightFormat), res.RightN)
	fmt.Fprintf(w, "  可比格（两侧都带 · 逐格比）: %s\n", gapDiffKeysText(res.ComparedKeys))
	if len(res.ExcludedKeys) > 0 {
		fmt.Fprintf(w, "  非可比格（只一侧带 · **不计差异**）: %s\n", gapDiffKeysText(res.ExcludedKeys))
	}
	if !res.HasDiff() {
		fmt.Fprintf(w, "无差异（可比格逐格相等 · 可比 %d 格）\n", len(res.ComparedKeys))
		fmt.Fprintf(w, "⇒ 无差异（退码 0）\n")
		return
	}
	fmt.Fprintf(w, "差异 %d 处：仅左 %d · 仅右 %d · 同 id 不同体 %d\n",
		len(res.Cells()), len(res.OnlyLeft), len(res.OnlyRight), len(res.BodyDiffs))
	for _, c := range res.OnlyLeft {
		fmt.Fprintf(w, "  [仅左] %s（左第 %d 行 · 右无）\n", c.ID, c.LeftLine)
	}
	for _, c := range res.OnlyRight {
		fmt.Fprintf(w, "  [仅右] %s（右第 %d 行 · 左无）\n", c.ID, c.RightLine)
	}
	for _, c := range res.BodyDiffs {
		fmt.Fprintf(w, "  [同 id 不同体] %s · 格 `%s`：左 `%s` ⟷ 右 `%s`\n",
			c.ID, c.Field, gapDiffValText(c.Left), gapDiffValText(c.Right))
	}
	fmt.Fprintf(w, "⇒ 有差异（退码 1）\n")
}

// gapDiffOutPlan —— `--out` 的**计划件**（`--dry-run` ⇒ 走 stdout · 缺 `--yes` ⇒ 走 stderr）。
// 纯函数：只算、只写 `io.Writer` ⇒ dryrun.v1「零副作用」的落点就在这一行（不落件 · 不写真源 · 不写审计）。
func gapDiffOutPlan(w io.Writer, res gapDiffResult, out, why string) {
	rc := exitOK
	if res.HasDiff() {
		rc = exitFail
	}
	fmt.Fprintf(w, "计划件（%s · `zerg gap diff … --out %s`）\n", why, out)
	fmt.Fprintf(w, "  落点     : %s（**报告件** —— ✗ 不写真源、不写审计）\n", out)
	fmt.Fprintf(w, "  左/右    : %s ⟷ %s\n", res.LeftPath, res.RightPath)
	fmt.Fprintf(w, "  差异     : 仅左 %d · 仅右 %d · 同 id 不同体 %d ⇒ 退码 %d\n",
		len(res.OnlyLeft), len(res.OnlyRight), len(res.BodyDiffs), rc)
	fmt.Fprintf(w, "  未执行   : %s —— 本档**一个字节都不落**（真写要 `--yes`）\n", why)
}

// gapDiffInputErr —— 输入取不到 / 不可比的唯一出口（退 2 · **不给结论** · ✗ 不假绿）。
func gapDiffInputErr(inv *invocation, stderr io.Writer, path string, err error) int {
	inv.setErr("usage", "diff_input_unreadable", err.Error())
	fmt.Fprintf(stderr, "%s: `gap diff` 输入取不到/不可比：%s（%v）⇒ **不给结论**（退码 2 · ✗ 不假绿）\n",
		progName, path, err)
	fmt.Fprintf(stderr, "  ⇒ 两份输入须各是「账（jsonl 每行一条 · 首非空行以 `{` 开头）」或「导出件（`gap export` 人面片段）」\n")
	return exitUsage
}

// gapDiffCommandSpec —— 本命令的命令树元数据（**尚未登记进 `main.go` 的 `commands` 表**）。
// 接线那一单把它登记进命令树；本文件先以**同一份 spec** 直接发同形包封
// （`emitEnvelopeWith`）⇒ 落件前后 `--json` 包封逐字节同，接线只需换 `selectJSONList`。
func gapDiffCommandSpec() *command {
	return &command{
		path:    []string{"gap", "diff"},
		kind:    "GapDiff",
		summary: "缺口账**差集面**（只读）：两份账 / 两份导出对拍 ⇒ 三类差（仅左 · 仅右 · 同 id 不同体），逐条点名到 id ＋ 哪一格",
		usage:   "zerg gap diff <左件> <右件> [--json <字段>] [--out <件路径> [--dry-run|--yes]]",
		fields:  gapDiffFields,
		arity:   arityAny,
		run:     cmdGapDiff,
	}
}

// cmdGapDiff —— `zerg gap diff` 的入口（与同族 `func cmdGapXxx(inv, stdout, stderr) int` 同形）。
//
// ★ 契约（逐条）：rc 0 = 无差异 · 1 = 有差异（逐条点名）· 2 = 输入取不到/格式不对/用法错（不给结论）。
func cmdGapDiff(inv *invocation, stdout, stderr io.Writer) int {
	if rc := dryRunYesConflict(inv, stderr); rc != exitOK {
		return rc
	}
	// ① 用法面（在任何盘面动作之前）
	if inv.jsonGiven && len(inv.fields) == 0 {
		inv.setErr("usage", "json_fields_required", "--json 不给字段")
		fmt.Fprintf(stderr, "%s: `--json` 要给逗号分隔的字段（本族口径 = 用法错 2）\n", progName)
		fmt.Fprintf(stderr, "可选字段: %s\n", strings.Join(gapDiffFields, ","))
		return exitUsage
	}
	if inv.hasFlag("--out") && inv.jsonGiven {
		inv.setErr("usage", "out_json_conflict", "--out 与 --json 不许同给")
		fmt.Fprintf(stderr, "%s: `--out`（落报告件）与 `--json`（机读投影）**不许同给** ⇒ 用法错 2\n", progName)
		return exitUsage
	}
	if len(inv.args) != gapDiffArity {
		inv.setErr("usage", "diff_arity", fmt.Sprintf("要 2 枚输入，得到 %d", len(inv.args)))
		fmt.Fprintf(stderr, "%s: `gap diff` 要 **2 枚输入**（左件 右件），现得 %d 枚 ⇒ 用法错 2（不给结论）\n",
			progName, len(inv.args))
		fmt.Fprintf(stderr, "See '%s gap diff --help'。\n", progName)
		return exitUsage
	}

	// ② 读两输入（取不到 / 格式不对 ⇒ 2 · **不给结论** · ✗ 不假绿）
	left, err := gapDiffLoadFile(inv.args[0])
	if err != nil {
		return gapDiffInputErr(inv, stderr, inv.args[0], err)
	}
	right, err := gapDiffLoadFile(inv.args[1])
	if err != nil {
		return gapDiffInputErr(inv, stderr, inv.args[1], err)
	}
	res := gapDiffCompute(left, right)
	rc := exitOK
	if res.HasDiff() {
		rc = exitFail
	}

	// ③ `--out <路径>` ⇒ 落**报告件**（D2 写面）；`--dry-run` ⇒ 计划件 + **零写盘**（rc 仍照真跑）
	if inv.hasFlag("--out") {
		out := strings.TrimSpace(inv.flagVal("--out"))
		if out == "" {
			inv.setErr("usage", "out_empty", "--out 没给路径")
			fmt.Fprintf(stderr, "%s: `--out` 要给一个**件路径**（裸给 / 空值 ⇒ 用法错 2 —— 不给落点就不动盘）\n", progName)
			return exitUsage
		}
		if inv.dryRun {
			gapDiffOutPlan(stdout, res, out, "--dry-run")
			fmt.Fprintf(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未落任何件；两输入与审计一个字节未动）\n")
			return rc
		}
		if !inv.yes {
			gapDiffOutPlan(stderr, res, out, "缺 `--yes`（D2 档）")
			inv.setErr("usage", "yes_required", "缺 --yes")
			fmt.Fprintf(stderr, "See '%s gap diff --help'。\n", progName)
			return exitUsage
		}
		var b strings.Builder
		gapDiffHumanReport(&b, res)
		if werr := os.WriteFile(out, []byte(b.String()), 0o644); werr != nil {
			inv.setErr("failed", "diff_out_write_failed", "报告件写不进")
			fmt.Fprintf(stderr, "%s: 报告件 `%s` 写不进：%v ⇒ 退 1（fail-closed：不静默少写）\n",
				progName, out, werr)
			return exitFail
		}
		gapDiffHumanReport(stdout, res)
		fmt.Fprintf(stdout, "  报告件   : %s（%d 字节）\n", out, len(b.String()))
		return rc
	}

	// ④ `--json` ⇒ 机读（与人面同一份差集）
	if inv.jsonGiven {
		for _, f := range inv.fields {
			if !gapDiffFieldKnown(f) {
				inv.setErr("usage", "json_field_unknown:"+f, fmt.Sprintf("未知字段 %q", f))
				fmt.Fprintf(stderr, "%s: 未知字段 %q\n", progName, f)
				fmt.Fprintf(stderr, "合法字段: %s\n", strings.Join(gapDiffFields, ","))
				fmt.Fprintf(stderr, "See '%s gap diff --help'。\n", progName)
				return exitUsage
			}
		}
		objs := make([]string, 0, len(res.Cells()))
		for _, c := range res.Cells() {
			row := map[string]string{
				"id": c.ID, "kind": c.Kind, "field": c.Field, "left": c.Left, "right": c.Right,
			}
			objs = append(objs, marshalObjectRegistered(inv.fields, row))
		}
		inv.changed = boolPtr(false)
		emitEnvelopeWith(stdout, gapDiffCommandSpec(), "["+strings.Join(objs, ",")+"]", len(res.Cells()), inv)
		return rc
	}

	// ⑤ 人面
	inv.changed = boolPtr(false)
	gapDiffHumanReport(stdout, res)
	return rc
}

// gapDiffFieldKnown —— `--json` 字段在册判（与 `gapDiffFields` 同尺）。
func gapDiffFieldKnown(f string) bool {
	for _, k := range gapDiffFields {
		if k == f {
			return true
		}
	}
	return false
}

// ── 接线随动清单（归另单 · ★ 本件 ✗ 不碰 `main.go`）────────────────────────────────────────────
//
// 本件只落「可独立的半」。把 `zerg gap diff` 真正接上命令面时，**另单**须逐条办：
//
//	1. **命令树条目**（`core/cmd/zerg/main.go` 的 `commands` 表）—— 直接登记
//	   `gapDiffCommandSpec()` 的那一份（path/kind/summary/usage/fields/arity/run）；`gap` 族
//	   现读 31 条动作 ⇒ `zerg help gap` 的动作数与「已开放」计数随动。
//	2. **`--help` 文本** —— `main.go` 里该条 usage 串 ＋ 族级帮助（`zerg help gap`）随动。
//	   ★ `--out` 是**取值旗标**（`valueFlagName` 已登记），但 dispatch 的 `usageDeclaresFlag`
//	   收口要求**本命令 usage 串里写明 `--out`**，否则 `gap diff --out x` 会被当未知旗标退 2。
//	3. **退码口径登记** —— 本面「输入取不到 ⇒ 2」（≠ 同族「真源读不到 ⇒ 8」）须进
//	   `scripts/gates/cli-contract-baseline.json` 的 `counts`/`mustfail_cases` ＋ 矩阵。
//	4. **`core/cmd/zerg/testdata/cli-matrix.json`** —— 新增命令须 ≥1 格 case 或带 reason 的豁免
//	   （「矩阵不留白」门恒红）；成对 must-fail 至少两格（缺输入 ⇒ 2 · `--dry-run`+`--yes` ⇒ 2）。
//	5. **`scripts/gates/newcmd-paths-baseline.json`** —— 新命令路径登记（门78 名单）。
//	6. **`core/internal/contract/dryrun-semantics.json`** —— `--dry-run` 在本命令是**真站点**
//	   （`--out` 落盘面的零副作用档）⇒ 一并入既有 `dryrun.v1` id（✗ 别造 `dryrun.v2`）。
//	7. **`scripts/公开标记.tsv`** ＋ 新件登记面 —— 两枚新件（本件 ＋ 测试件）逐件一行。
//	8. **docs 命令行页**（`docs/{zh,en}/参考-命令行.md` ＋ `publish/docs/CLI.*.md`）—— 生成区随动
//	   （须排在**干净重编**之后 `--emit`）。
//	9. **`zerg gap show GAP-20261013-20` 的 `verify_cmd`** —— 现读是
//	   `zerg gate check check-cli-contract --dry-run`（`--dry-run` 后缀使其**恒 0**）⇒ 接线后换真。
