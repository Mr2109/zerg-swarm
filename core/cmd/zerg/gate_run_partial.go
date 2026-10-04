// gate_run_partial.go —— 门禁「部分趟」**主链**（设计稿《设计-门禁部分趟-v0.7》§3.3–§3.10/§四/§五/§六 · 任务 T-PR-03·04）。
//
// ⚠ 候件（仓外）：拟落路径 `core/cmd/zerg/gate_run_partial.go`（**新件** · 现读不存在）。本件只落
// **已拍/已定稿**口径（✗ 不引入任何待拍选择）；待拍项（D3/D5/D6/D8/D10/D16 …）一律按底稿写死为
// 「现读实现」或本片不碰。
//
// 入口只有一条：`zerg gate run --since [<全量趟目录>]`（§3.1-①：默认全量；部分趟**必须显式**）。
// 缺 `--since` 时本件一个字节都不动 —— 那是 `gate.go` 那句「只转发、不翻译」的默认面。
//
// 口径（全部引自 v0.7 已拍/已定稿）：
//
//	· R（重跑步集）= ① 上趟非 PASS（**P2**：FAIL∪BLOCKED∪REPORT） ∪ ② 改动件闭包（§3.3）。
//	· ② 闭包**只并 `files[].steps`**，✗ 不并 `faces[face]`（§3.3 改正 · 与 `affected-steps.py` 现读实现一致）。
//	· 改动件集（§3.6）：基线 = 上趟 `freshness.json.head`；`git diff --name-status <基线>`（含 staged）
//	  ∪ `git ls-files --others --exclude-standard`；重命名取 `R 旧→新` 的**两侧路径**（§六负控③ 定稿）。
//	· 三处转全量**固定次序**（§3.4）：① 命中 undeclared → ② 命中五真源（P6） → ③ 否则才算 `|R|/N >= 0.60`（P5）。
//	  多命中**并列印**（D16 已定稿「并列」）；①② 结果相同（都 ⇒ 全量），次序只为可观测/可复跑。
//	· 对齐（§3.7）：`affected-map.faces` 命名 `D==N` 且与现读步集**逐字相等**，否则 **rc=2 不出**（fail-closed）。
//	· `--since` 只许指**全量趟**（P3）；与 `--scope`/`--fast`/`--step` 叠并 ⇒ `rc=2`（§3.2）。
//	· 台账**物理隔离**：只写 `results-partial.tsv`（+ `partial.json`），**从不**写 `results.tsv`（§3.1-②）。
//	· 日志序号沿用**全表 NN**（P4）；退码：全绿=3（P1）· 有 FAIL=1 · 无 FAIL 有 BLOCKED=2 · 用法/基线/对齐/歧义=2（§五）。
//
// ✗ 不做（红线 §3.2 / §十）：按内容指纹跳步；未跑步标「沿用/继承绿」；自动把部分趟当收口。
// ⚠ 本片**不碰** `freshness.json`（接口 α 属 D10 待拍 ⇒ 表乙 T-PR-15）与**退码表**（退码 3 的登记属
//
//	D15/P1 ⇒ 表乙 T-PR-20）；本件只 `return 3`，登记面清单见回执。
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ── 常量（逐字真源 · 别处不许再写一份）────────────────────────────────────────

const (
	gateResultsPartialTSV = "results-partial.tsv" // 部分趟台账（与 results.tsv **同列** · §4.1）
	gatePartialJSONFile   = "partial.json"        // 部分趟抬头/元数据（§4.2）
	gatePartialSchema     = "gate-partial/1"
	gatePartialKind       = "partial"
	gatePartialLedger     = gateResultsPartialTSV
)

// exitPartialRun —— 部分趟**独有**退码（全绿 · P1 已拍；**本片只返回、不登记** —— 登记面走表乙 T-PR-20/D15）。
const exitPartialRun = 3

// gatePartialThreshold —— 60% 阀（**P5 已拍**：`>= 0.60` ⇒ 直接全量；`|R|==102` 亦转全量）。
const gatePartialThreshold = 0.60

// gatePartialAffectedMapRel —— 闭包真源（只读；与脚本/`affected-steps.py` 同路径）。
const gatePartialAffectedMapRel = "scripts/gates/affected-map.json"

// gatePartialFiveTruths —— **P6 五真源白名单**（§3.4 = **逐字 · 只 5 条字面路径常量**）。
// 命中任一 ⇒ 直接全量（独立于声明表映射的硬规则）。**不许擅自扩为 7 件**（D14 待拍 · 表乙 T-PR-19）。
var gatePartialFiveTruths = []string{
	"scripts/gates/precommit-gates.sh",
	"scripts/gates/gate-coverage.config.json",
	"scripts/gates/newcmd-paths-baseline.json",
	"scripts/gates/affected-map.json",
	"scripts/gates/judge-registry.json",
}

// ── `--since` 识别与叠并拒绝（§3.2 / §3.5）──────────────────────────────────

// gatePartialSinceGiven —— tail 里有没有 `--since`（部分趟的**唯一**显式入口）。
func gatePartialSinceGiven(tail []string) bool {
	for _, a := range tail {
		if a == "--since" || strings.HasPrefix(a, "--since=") {
			return true
		}
	}
	return false
}

// gatePartialSinceArg —— `--since` 的值：给目录 ⇒ 它；裸 `--since`（下一枚是旗标/无值）⇒ 空串（= `--last` 同口径 · §3.5）。
func gatePartialSinceArg(tail []string) string {
	for i, a := range tail {
		if a == "--since" {
			if i+1 < len(tail) && !strings.HasPrefix(tail[i+1], "-") {
				return tail[i+1]
			}
			return ""
		}
		if strings.HasPrefix(a, "--since=") {
			return strings.TrimPrefix(a, "--since=")
		}
	}
	return ""
}

// gatePartialStripSince —— 转全量时把 `--since`（及其值）从 tail 里去掉，其余**逐字保留**（走原全量路径）。
func gatePartialStripSince(tail []string) []string {
	out := make([]string, 0, len(tail))
	for i := 0; i < len(tail); i++ {
		a := tail[i]
		if a == "--since" {
			if i+1 < len(tail) && !strings.HasPrefix(tail[i+1], "-") {
				i++ // 连值一起去
			}
			continue
		}
		if strings.HasPrefix(a, "--since=") {
			continue
		}
		out = append(out, a)
	}
	return out
}

// gatePartialOverlap —— §3.2/§3.5：`--since` 与 `--scope`/`--fast`/`--step` 叠并 ⇒ 拒（rc=2，fail-closed，不猜交集/并集）。
// 命令面自己的三个机器面（`--json`/`--verify-live`/`--show-log`）**也一并拒** —— 它们都以 `--step` 为前提、
// 与部分趟不相容；静默忽略会给调用方一个「与敲的不一样」的结果 ⇒ 同规 fail-closed（宁拒不猜）。
func gatePartialOverlap(tail []string) string {
	incompat := []string{"--scope", "--fast", "--step", "--json", "--verify-live", "--show-log"}
	for _, a := range tail {
		name := a
		if i := strings.IndexByte(a, '='); i >= 0 {
			name = a[:i]
		}
		for _, k := range incompat {
			if name == k {
				return name
			}
		}
	}
	return ""
}

// ── 改动件集（§3.6 · 重命名两面 §六负控③ 定稿）──────────────────────────────

// gatePartialParseNameStatus —— 解析 `git diff --name-status` 的出参（**纯函数** · 便于单测）。
// ★ **D6（已拍 · 定稿）**：`R<score> 旧 → 新`（及 `C` 拷贝）的**两侧路径都并入**改动集 ——
//
//	✗ **不依赖**默认 rename 检测 `-M` 的**单侧**输出；两侧都查闭包、R 取并集；任一侧未声明 ⇒
//	整趟全量（fail-safe）。其余状态（`M/A/D/T/…`）取第二列那一个路径。
func gatePartialParseNameStatus(out string) []string {
	set := map[string]bool{}
	for _, ln := range strings.Split(out, "\n") {
		ln = strings.TrimRight(ln, "\r")
		if strings.TrimSpace(ln) == "" {
			continue
		}
		f := strings.Split(ln, "\t")
		if len(f) < 2 {
			continue
		}
		st := f[0]
		if strings.HasPrefix(st, "R") || strings.HasPrefix(st, "C") {
			if len(f) >= 3 { // 重命名/复制 ⇒ **两面都取**（§六负控③ · D6）
				set[f[1]] = true
				set[f[2]] = true
			}
			continue
		}
		set[f[1]] = true
	}
	names := make([]string, 0, len(set))
	for p := range set {
		names = append(names, p)
	}
	sort.Strings(names)
	return names
}

// gatePartialChangedSet —— 基线（上趟 head）→ 现工作树的改动件集（去重、排序）。
//
//	· `git diff --name-status <head>`：**含 staged**（基线树 → 现工作树的全部差）；重命名 `R 旧→新`
//	  的**两侧路径都并入**（✗ 不依赖 `--name-only` 的单侧）。
//	· ∪ `git ls-files --others --exclude-standard`（未跟踪、未忽略）。
//	· `-c core.quotepath=false`：路径**不转义**，与声明表/闭包的逐字比对口径一致（非 ASCII 件名不误判）。
//	★ **D7（已拍）**：「**只改 mtime / 属性位、内容未动 ⇒ 不转全量**」—— `git diff` 比的是
//	  **内容（tree 差）**，`touch` 不产生 diff ⇒ 判「没变」是**对的口径**（判定只认内容差、
//	  不认 mtime）。★ 这是**口径、非漏**：✗ 不该被读成「漏掉了一次改动」。
func gatePartialChangedSet(root, head string) ([]string, error) {
	out, err := gateGitReadOnly(root, "-c", "core.quotepath=false", "diff", "--name-status", head)
	if err != nil {
		return nil, err
	}
	set := map[string]bool{}
	for _, p := range gatePartialParseNameStatus(out) {
		set[p] = true
	}
	ot, err := gateGitReadOnly(root, "-c", "core.quotepath=false", "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	for _, ln := range strings.Split(ot, "\n") {
		ln = strings.TrimSpace(ln)
		if ln != "" {
			set[ln] = true
		}
	}
	changed := make([]string, 0, len(set))
	for p := range set {
		changed = append(changed, p)
	}
	sort.Strings(changed)
	return changed, nil
}

// ── 闭包（§3.3 ② · 只并 files[].steps）────────────────────────────────────

type gatePartialMapEntry struct {
	Pattern string   `json:"pattern"`
	Kind    string   `json:"kind"`
	Steps   []string `json:"steps"`
}

type gatePartialAffectedMap struct {
	Files []gatePartialMapEntry `json:"files"`
	Faces map[string][]string   `json:"faces"`
}

// gatePartialEntryMatch —— 逐字照 `affected-steps.py` 的 `entry_matches`：prefix/`**` 前缀、glob、literal。
func gatePartialEntryMatch(e gatePartialMapEntry, rel string) bool {
	pat := e.Pattern
	if e.Kind == "prefix" || strings.HasSuffix(pat, "/**") {
		base := strings.TrimSuffix(strings.TrimSuffix(pat, "/**"), "/")
		return rel == base || strings.HasPrefix(rel, base+"/")
	}
	if e.Kind == "glob" {
		return gatePartialGlobMatch(pat, rel)
	}
	return rel == pat
}

// gatePartialGlobMatch —— fnmatch 风格：`*` 跨 `/`（与 py `fnmatch.fnmatch` 同口径）、`?` 单字符。
// （现读 `affected-map.files` 只有 prefix/literal 两类；本函数为 glob 类防御性实现。）
func gatePartialGlobMatch(pat, name string) bool {
	for len(pat) > 0 {
		switch pat[0] {
		case '*':
			for len(pat) > 0 && pat[0] == '*' {
				pat = pat[1:]
			}
			if len(pat) == 0 {
				return true
			}
			for i := 0; i <= len(name); i++ {
				if gatePartialGlobMatch(pat, name[i:]) {
					return true
				}
			}
			return false
		case '?':
			if len(name) == 0 {
				return false
			}
			pat, name = pat[1:], name[1:]
		default:
			if len(name) == 0 || pat[0] != name[0] {
				return false
			}
			pat, name = pat[1:], name[1:]
		}
	}
	return len(name) == 0
}

// gatePartialLoadMap —— 只读声明表（读不到/解析不了 ⇒ 调用方给 rc=2，不给结论）。
func gatePartialLoadMap(root string) (*gatePartialAffectedMap, error) {
	b, err := os.ReadFile(filepath.Join(root, gatePartialAffectedMapRel))
	if err != nil {
		return nil, err
	}
	var mp gatePartialAffectedMap
	if err := json.Unmarshal(b, &mp); err != nil {
		return nil, err
	}
	return &mp, nil
}

// gatePartialClosure —— 改动件 ⇒（受影响步集 · 未声明件清单）。**只并 `files[].steps`**，不并 faces（§3.3 改正）。
func gatePartialClosure(mp *gatePartialAffectedMap, changed []string) (affected []string, undeclared []string) {
	aff := map[string]bool{}
	for _, c := range changed {
		hit := false
		for _, e := range mp.Files {
			if gatePartialEntryMatch(e, c) {
				hit = true
				for _, s := range e.Steps {
					aff[s] = true
				}
			}
		}
		if !hit {
			undeclared = append(undeclared, c)
		}
	}
	affected = make([]string, 0, len(aff))
	for s := range aff {
		affected = append(affected, s)
	}
	sort.Strings(affected)
	return affected, undeclared
}

// gatePartialFacesNamed —— 声明表 `faces` 的**命名集合**（去重 · 排序）：C0 对齐的 D 面（§3.7）。
func gatePartialFacesNamed(mp *gatePartialAffectedMap) []string {
	seen := map[string]bool{}
	named := []string{}
	for _, f := range mp.Faces {
		for _, n := range f {
			if !seen[n] {
				seen[n] = true
				named = append(named, n)
			}
		}
	}
	sort.Strings(named)
	return named
}

// gatePartialAlignOK —— §3.7：`D==N` 且命名集合 == 现读步集（**逐字相等**）；否则部分趟 **rc=2 不出**。
func gatePartialAlignOK(mp *gatePartialAffectedMap, full []string) (bool, string) {
	named := gatePartialFacesNamed(mp)
	if len(named) != len(full) {
		return false, fmt.Sprintf("声明表命名 %d 步 ≠ 现读步骤表 %d 步（表已漂移 ⇒ 静默漏选）", len(named), len(full))
	}
	tset := map[string]bool{}
	for _, n := range full {
		tset[n] = true
	}
	miss := []string{}
	for _, n := range full {
		if !gatePartialHas(named, n) {
			miss = append(miss, n)
		}
	}
	ghost := []string{}
	for _, n := range named {
		if !tset[n] {
			ghost = append(ghost, n)
		}
	}
	if len(miss) > 0 || len(ghost) > 0 {
		return false, fmt.Sprintf("命名集合 ≠ 表集合（漏=%d 门 · 幻影=%d 门）", len(miss), len(ghost))
	}
	return true, ""
}

func gatePartialHas(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

// gatePartialFiveTruthsHit —— 改动件 ∩ 五真源白名单（字面相等 · §3.4）。
func gatePartialFiveTruthsHit(changed []string) []string {
	set := map[string]bool{}
	for _, c := range changed {
		set[c] = true
	}
	hits := []string{}
	for _, w := range gatePartialFiveTruths {
		if set[w] {
			hits = append(hits, w)
		}
	}
	return hits
}

// ── ① 上趟非 PASS（P2）──────────────────────────────────────────────────

// gatePartialNonPass —— 基线趟非 PASS 步（**P2**：FAIL ∪ BLOCKED ∪ REPORT），并给三档计数。
// ★ **D2（已拍 · 维持 P2）**：`REPORT` 计入非 PASS —— 依据「`REPORT` = 只报告 ≠ 绿」；它不进 ① 的话
//
//	修复之后就**永远不再看它**。★ **复议时点 = M0 实测后**（R 长期分布出来、收益过薄再议）——
//	本处**不**拆「常态只报告」豁免子类（现无据，不预先拆）。
func gatePartialNonPass(rows []gateResultsRow) (steps []string, fail, blocked, report int) {
	for _, r := range rows {
		switch r.Status {
		case "FAIL":
			fail++
		case "BLOCKED":
			blocked++
		case "REPORT":
			report++
		default:
			continue // PASS 不入 ①
		}
		steps = append(steps, r.Name)
	}
	return steps, fail, blocked, report
}

// ── 决策（三处转全量固定次序 · §3.4）─────────────────────────────────────

// gatePartialPlan —— 一趟部分趟的**选步/判定**全貌（抬头与 partial.json 同源）。
type gatePartialPlan struct {
	Full         bool     // true ⇒ 转全量
	Reasons      []string // 转全量原因（**并列**逐条 · D16 已定稿）
	Baseline     string   // 基线全量趟目录
	BaselineHead string
	BaselineFP   string
	ResultsMT    string

	Fail, Blocked, Report int // 基线趟非 PASS 三档计数
	NonPass               []string

	Changed    []string
	Undeclared []string
	FiveTruths []string
	Affected   []string // 闭包受影响步
	R          []string // 重跑步集（按全表 NN 排序）
	N          int
}

// gatePartialDecide —— 按**固定次序**① undeclared → ② 五真源 → ③ 阀（§3.4）判「转全量 / 出部分趟」。
// 多命中**并列**（reasons 逐条追加），✗ 不合并成一句。
func gatePartialDecide(p *gatePartialPlan, nonPass, affected []string, full []string) {
	// ① undeclared（§3.3-③）：结构性「该跑全局」
	if len(p.Undeclared) > 0 {
		p.Reasons = append(p.Reasons, fmt.Sprintf("未声明件 %d 件 ⇒ 转全量", len(p.Undeclared)))
	}
	// ② 五真源（P6 · §3.4）：结构性「该跑全局」（独立于 ①；两条**各自成立**，同时命中无害）
	if len(p.FiveTruths) > 0 {
		p.Reasons = append(p.Reasons, fmt.Sprintf("五真源命中 %d 件 ⇒ 转全量", len(p.FiveTruths)))
	}
	// ①② 都 ⇒ 全量 ⇒ **不再算阀、不出部分趟**（次序只为可观测/可复跑 · §3.4）
	if len(p.Reasons) > 0 {
		p.Full = true
		return
	}
	// ③ 否则才算阀：R = ① 非 PASS ∪ ② 闭包，按全表 NN 排序；|R|/N >= 0.60 ⇒ 全量（P5；分母 = **现读 N**，D5）
	set := map[string]bool{}
	for _, n := range nonPass {
		set[n] = true
	}
	for _, n := range affected {
		set[n] = true
	}
	p.R = gatePartialOrderR(full, set)
	ratio := 0.0
	if len(full) > 0 {
		ratio = float64(len(p.R)) / float64(len(full))
	}
	if ratio >= gatePartialThreshold {
		p.Full = true
		p.Reasons = append(p.Reasons, fmt.Sprintf("阀 |R|/N = %d/%d = %.1f%% >= 60%% ⇒ 转全量", len(p.R), len(full), ratio*100))
	}
}

// gatePartialOrderR —— R 按**全表 NN 序**（现读步集序）排；不在现读步集里的名（异常）补在末尾（保守带上）。
func gatePartialOrderR(full []string, set map[string]bool) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, n := range full {
		if set[n] && !seen[n] {
			out = append(out, n)
			seen[n] = true
		}
	}
	rest := []string{}
	for n := range set {
		if !seen[n] {
			rest = append(rest, n)
		}
	}
	sort.Strings(rest)
	return append(out, rest...)
}

// ── 台账写手（§4.1 / §4.2）───────────────────────────────────────────────

// gatePartialRow —— 部分趟一行（与 `results.tsv` **同列**：状态/步名/rc/秒/日志）。
type gatePartialRow struct {
	Status, Name, RC, Secs, Log string
}

func writeResultsPartial(dir string, rows []gatePartialRow) error {
	var sb strings.Builder
	for _, r := range rows {
		fmt.Fprintf(&sb, "%s\t%s\t%s\t%s\t%s\n", r.Status, r.Name, r.RC, r.Secs, r.Log)
	}
	return os.WriteFile(filepath.Join(dir, gateResultsPartialTSV), []byte(sb.String()), 0o644)
}

type gatePartialBasedOn struct {
	Dir        string            `json:"dir"`
	Head       string            `json:"head"`
	WorktreeFP string            `json:"worktree_fp"`
	ResultsMT  string            `json:"results_mtime"`
	NonPass    gatePartialCounts `json:"nonpass"`
}

type gatePartialCounts struct {
	Fail      int `json:"fail"`
	Blocked   int `json:"blocked"`
	Report    int `json:"report"`
	SeedSteps int `json:"seed_steps"`
}

type gatePartialChangedJSON struct {
	Files         int `json:"files"`
	Undeclared    int `json:"undeclared"`
	AffectedSteps int `json:"affected_steps"`
}

type gatePartialRerunJSON struct {
	Steps     int     `json:"steps"`
	Total     int     `json:"total"`
	Ratio     float64 `json:"ratio"`
	Threshold float64 `json:"threshold"`
	Rule      string  `json:"rule"`
}

type gatePartialSkippedJSON struct {
	Count int    `json:"count"`
	Note  string `json:"note"`
}

// gatePartialTriggersJSON —— D8（T-PR-13）：五真源 / 未声明命中的落痕（便于 M0 观察）。
// 五真源命中 ⇒ `five_truths` 逐件列出（**命中即全量**那一路也落这枚 partial.json）。
type gatePartialTriggersJSON struct {
	FiveTruths []string `json:"five_truths"`
	Undeclared []string `json:"undeclared,omitempty"`
	Order      string   `json:"order"`
}

type gatePartialJSONDoc struct {
	Schema   string                  `json:"schema"`
	Kind     string                  `json:"kind"`
	BasedOn  gatePartialBasedOn      `json:"based_on"`
	Changed  gatePartialChangedJSON  `json:"changed"`
	Rerun    gatePartialRerunJSON    `json:"rerun"`
	Skipped  gatePartialSkippedJSON  `json:"skipped"`
	Triggers gatePartialTriggersJSON `json:"triggers"`
	Ledger   string                  `json:"ledger"`
	Exit     int                     `json:"exit"`
}

func writePartialJSON(dir string, p *gatePartialPlan, exit int) error {
	ratio := 0.0
	if p.N > 0 {
		ratio = float64(len(p.R)) / float64(p.N)
	}
	trig := gatePartialTriggersJSON{
		FiveTruths: append([]string{}, p.FiveTruths...),
		Undeclared: append([]string{}, p.Undeclared...),
		Order:      "① undeclared → ② 五真源 → ③ 阀 >=0.60",
	}
	doc := gatePartialJSONDoc{
		Schema: gatePartialSchema,
		Kind:   gatePartialKind,
		BasedOn: gatePartialBasedOn{
			Dir: p.Baseline, Head: p.BaselineHead, WorktreeFP: p.BaselineFP, ResultsMT: p.ResultsMT,
			NonPass: gatePartialCounts{Fail: p.Fail, Blocked: p.Blocked, Report: p.Report,
				SeedSteps: p.Fail + p.Blocked + p.Report},
		},
		Changed: gatePartialChangedJSON{Files: len(p.Changed), Undeclared: len(p.Undeclared), AffectedSteps: len(p.Affected)},
		Rerun: gatePartialRerunJSON{Steps: len(p.R), Total: p.N, Ratio: ratio,
			Threshold: gatePartialThreshold, Rule: ">=0.60 ⇒ 全量"},
		Skipped:  gatePartialSkippedJSON{Count: p.N - len(p.R), Note: "未跑步不进本台账 · 不标沿用"},
		Triggers: trig,
		Ledger:   gatePartialLedger,
		Exit:     exit,
	}
	body, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, gatePartialJSONFile), append(body, '\n'), 0o644)
}

// ── 抬头（§4.3 逆字）─────────────────────────────────────────────────────

func gatePartialPrintHeader(out io.Writer, p *gatePartialPlan) {
	ratio := 0.0
	if p.N > 0 {
		ratio = float64(len(p.R)) / float64(p.N)
	}
	fmt.Fprintln(out, "── 门禁【部分趟】partial · **不覆盖全量** ────────────────────────────")
	fmt.Fprintln(out, "趟类型  partial（部分趟；✗ 不可读作「全跑过且全绿」）")
	fmt.Fprintf(out, "根据    --since（%s）：%s\n", gatePartialSinceArgHint, p.Baseline)
	fmt.Fprintf(out, "上趟    id=%s · head=%s · worktree_fp=%s · 非 PASS %d 步（FAIL %d + BLOCKED %d + REPORT %d）\n",
		filepath.Base(p.Baseline), p.BaselineHead, short16(p.BaselineFP),
		p.Fail+p.Blocked+p.Report, p.Fail, p.Blocked, p.Report)
	fmt.Fprintf(out, "改动件  %d 件（基线 = 上趟 head %s · 含已提交/已暂存/未暂存 ∪ 未跟踪）\n",
		len(p.Changed), p.BaselineHead)
	fmt.Fprintf(out, "重跑    R = %d 步 / N = %d 步（%d/%d = %.1f%% < 60%% ⇒ 出部分趟；≥60%% 即转全量）\n",
		len(p.R), p.N, len(p.R), p.N, ratio*100)
	fmt.Fprintf(out, "未跑    %d 步（**本趟不跑 · 不入本趟台账 · 不标「沿用」**）\n", p.N-len(p.R))
	fmt.Fprintf(out, "台账    %s（另存 · 与 results.tsv 物理隔离）\n", gateResultsPartialTSV)
	fmt.Fprintln(out, "退码    全绿=3 · 有红=1 · 有 BLOCKED=2 —— **本档绝不退 0**（3 为部分趟独有 · 见 §五）")
	fmt.Fprintln(out, "── 重跑步序（全表序号 NN · 与全量趟同名同号）───────────────────────")
	for _, n := range p.R {
		fmt.Fprintf(out, "  %s\n", n)
	}
}

// gatePartialSinceArgHint —— 抬头「根据」行用（`--since` 带值 vs 裸）；调用前设。
var gatePartialSinceArgHint = ""

// gatePartialPrintFullHeader —— 转全量抬头（三处固定次序 · 多命中**并列**印 · §3.4/D16）。
func gatePartialPrintFullHeader(out io.Writer, p *gatePartialPlan) {
	fmt.Fprintln(out, "── 门禁【部分趟→转全量】partial→full ────────────────────────────────")
	fmt.Fprintf(out, "根据    --since（%s）：%s\n", gatePartialSinceArgHint, p.Baseline)
	fmt.Fprintf(out, "上趟    id=%s · head=%s · 非 PASS %d 步\n",
		filepath.Base(p.Baseline), p.BaselineHead, p.Fail+p.Blocked+p.Report)
	fmt.Fprintf(out, "改动件  %d 件\n", len(p.Changed))
	fmt.Fprintf(out, "N = %d 步（**现读**默认集步数 · 阀分母 · D5）\n", p.N)
	for _, r := range p.Reasons {
		fmt.Fprintf(out, "转全量原因  %s\n", r)
	}
	if len(p.Undeclared) > 0 {
		fmt.Fprintln(out, "  未声明件：")
		for _, c := range clipList(p.Undeclared, 20) {
			fmt.Fprintf(out, "    ？ %s\n", c)
		}
	}
	if len(p.FiveTruths) > 0 {
		fmt.Fprintln(out, "  五真源命中：")
		for _, c := range p.FiveTruths {
			fmt.Fprintf(out, "    ▶ %s\n", c)
		}
	}
	fmt.Fprintln(out, "（①② 结果相同 · 次序只为可观测/可复跑 · §3.4；本趟按**全量**路径执行）")
}

// ── 主链 ─────────────────────────────────────────────────────────────────

// gateRunPartial —— `zerg gate run --since …` 的主链。
//
// 返回（rc, handled）：
//
//	handled=true  ⇒ 已就地处置（rc 即本命令退码）；
//	handled=false ⇒ **转全量**：调用方（`gate.go`）应把 tail **去掉 `--since`** 后按原全量路径继续（一字不改）。
func gateRunPartial(inv *invocation, stdout, stderr io.Writer, root, script string, tail []string) (int, bool) {
	// ① 叠并拒绝（§3.2/§3.5）—— 判在**一切**之前。
	if bad := gatePartialOverlap(tail); bad != "" {
		inv.setErr("usage", "partial_flag_overlap", "`--since` 与 "+bad+" 叠并")
		fmt.Fprintf(stderr, "%s: `--since` 与 `%s` 叠并 ⇒ 拒执（退码 2）—— 四者语义各自独立，叠并语义未定义，**不**猜交集/并集\n", progName, bad)
		fmt.Fprintf(stderr, "口径：部分趟唯一入口 = `--since`；要按面/快档/单步跑就别带 `--since`（§3.2 · P3 同族 fail-closed）\n")
		return exitUsage, true
	}

	// ② 现读全量步集（N 与全表 NN 序 —— 唯一真源 = 脚本自己的 `--list`）。
	full, err := gateFullStepNames(root)
	if err != nil {
		inv.setErr("blocked", "gate_step_list_unreadable", err.Error())
		fmt.Fprintf(stderr, "%s: 读不回门件自己的步骤清单 ⇒ 不给结论（退码 8）：%v\n", progName, err)
		return exitBlocked, true
	}

	// ③ 基线全量趟（§3.5 硬前置）。
	sinceArg := gatePartialSinceArg(tail)
	gatePartialSinceArgHint = "最新写入"
	if sinceArg != "" {
		gatePartialSinceArgHint = "点名"
	}
	baseDir, ok, berr := gatePartialBaselineDir(root, sinceArg)
	if berr != nil {
		inv.setErr("blocked", "gate_runs_unreadable", berr.Error())
		fmt.Fprintf(stderr, "%s: 找基线趟时读盘读不动 ⇒ 不给结论（退码 8）：%v\n", progName, berr)
		return exitBlocked, true
	}
	if !ok {
		inv.setErr("usage", "partial_baseline_absent", "取不到基线全量趟")
		fmt.Fprintf(stderr, "%s: 取不到基线全量趟（`--since` %s）⇒ 拒执（退码 2）—— ✗ 不退回全量、✗ 不当空集\n",
			progName, sinceArg)
		fmt.Fprintf(stderr, "口径：部分趟必须先有一趟**全量**做基线（有 results.tsv + freshness.json）\n")
		return exitUsage, true
	}
	rec, rows, prob := gatePartialValidateBaseline(baseDir, full)
	if prob != "" {
		inv.setErr("usage", "partial_baseline_invalid", prob)
		fmt.Fprintf(stderr, "%s: 基线趟不可用 ⇒ 拒执（退码 2）：%s\n", progName, prob)
		fmt.Fprintf(stderr, "口径：`--since` 只许指**全量趟**（P3）· 全量判据 = results.tsv 行数==N 且步名集合逐字相等（§3.5 最小判据）\n")
		return exitUsage, true
	}

	// ④ 对齐（§3.7）：声明表命名 D==N 且逐字相等；否则 rc=2 不出。
	mp, merr := gatePartialLoadMap(root)
	if merr != nil {
		inv.setErr("blocked", "affected_map_unreadable", merr.Error())
		fmt.Fprintf(stderr, "%s: 声明表读不到/解析不了（%s）⇒ 不给结论（退码 8）：%v\n", progName, gatePartialAffectedMapRel, merr)
		return exitBlocked, true
	}
	if ok, why := gatePartialAlignOK(mp, full); !ok {
		inv.setErr("usage", "partial_map_drift", why)
		fmt.Fprintf(stderr, "%s: 声明表**步面漂移** ⇒ 部分趟拒执（退码 2，fail-closed · §3.7/§七(a)）：%s\n", progName, why)
		fmt.Fprintf(stderr, "口径：先重生成 `affected-map.json`（表甲 T-PR-01）并接线步面机检（T-PR-14）后，部分趟方可安全启用\n")
		return exitUsage, true
	}

	// ⑤ 改动件集（§3.6）。
	changed, cerr := gatePartialChangedSet(root, rec.Head)
	if cerr != nil {
		inv.setErr("blocked", "worktree_diff_failed", cerr.Error())
		fmt.Fprintf(stderr, "%s: 算不出改动件集（基线 head %s）⇒ 不给结论（退码 8）：%v\n", progName, rec.Head, cerr)
		return exitBlocked, true
	}

	// ⑥ 计算三源（① 非 PASS · ② 闭包 · 五真源）。
	nonPass, fail, blocked, report := gatePartialNonPass(rows)
	affected, undeclared := gatePartialClosure(mp, changed)
	five := gatePartialFiveTruthsHit(changed)

	plan := &gatePartialPlan{
		Baseline: baseDir, BaselineHead: rec.Head, BaselineFP: rec.Fingerprint, ResultsMT: gatePartialResultsMTime(baseDir),
		Fail: fail, Blocked: blocked, Report: report, NonPass: nonPass,
		Changed: changed, Undeclared: undeclared, FiveTruths: five, Affected: affected, N: len(full),
	}

	// ⑦ 决策（固定次序 · §3.4）。
	gatePartialDecide(plan, nonPass, affected, full)

	// ⑧ 抬头：转全量 ⇒ 印因后交回 gate.go 走原全量路径；部分趟 ⇒ 印抬头后执行。
	if plan.Full {
		gatePartialPrintFullHeader(stdout, plan)
		// ★ D8（T-PR-13）：五真源 / 未声明命中 ⇒ **落 `partial.json`**（含 `triggers.five_truths`），
		//   即便这一趟**转全量**也落 —— 兑现「命中时落该字段」；并把这**同一趟目录**交给 `gate.go`
		//   作为后续全量跑的 `--outdir`（使 partial.json 与全量 results.tsv 同目录）。
		if dir, oerr := gatePartialMakeOutdir(root, tail); oerr == nil {
			if werr := writePartialJSON(dir, plan, 0); werr != nil {
				fmt.Fprintf(stderr, "%s: ⚠ 转全量：%s 落不下去（%v）—— 退码不变\n", progName, gatePartialJSONFile, werr)
			}
			setGatePartialFullOutdir(dir)
		} else {
			fmt.Fprintf(stderr, "%s: ⚠ 转全量：定不了趟目录（%v）⇒ 不落 %s（退码不变）\n", progName, oerr, gatePartialJSONFile)
		}
		return exitOK, false
	}
	gatePartialPrintHeader(stdout, plan)

	// ★ seal 接线（部分趟侧「章账引用自比」· 设计-门禁部分趟 §13.1 docB 条 4 / D11）：
	//   S_keep = full − R（本趟不重跑的步）；可引者**只作引用**（本趟不真跑）、不可引者**回填进 R**
	//   （§13.1 步⑥「回填重跑」· 严格性优先于收益）。判不了（rc≠0）⇒ 整趟拒执（✗ 不当空集 / ✗ 不当全绿）。
	keep := gatePartialSealKeepSet(full, plan.R)
	_, backfill, sealRC := gatePartialSealRefs(root, plan.Baseline, keep, full, "", stdout)
	if sealRC != exitOK {
		inv.setErr("usage", "partial_seal_undecidable", "章账引用自比判不了 ⇒ 整趟拒执")
		fmt.Fprintf(stderr, "%s: 章账引用自比判不了（rc=%d）⇒ 部分趟拒执（退码 2）—— ✗ 不当空集 / ✗ 不当全绿\n", progName, sealRC)
		return exitUsage, true
	}
	if len(backfill) > 0 {
		set := map[string]bool{}
		for _, n := range plan.R {
			set[n] = true
		}
		for _, n := range backfill {
			set[n] = true
		}
		plan.R = gatePartialOrderR(full, set)
	}

	// ⑨ 部分趟：建目录 → 逐 R 步执行 → 写台账 → 退码（§五）。
	outdir, oerr := gatePartialMakeOutdir(root, tail)
	if oerr != nil {
		inv.setErr("failed", "partial_outdir_unwritable", oerr.Error())
		fmt.Fprintf(stderr, "%s: 建不了部分趟目录 ⇒ 不给结论（退码 8）：%v\n", progName, oerr)
		return exitBlocked, true
	}
	// ★ D10（α · 已拍）：部分趟**另发** `freshness-partial.json`（含 `based_on` 基线四格）——
	//   指纹在执行**之前**取（= 这一趟看到的那棵树），与 `gate.go` 全量那一路同规。
	on := gatePartialBasedOn{
		Dir: plan.Baseline, Head: plan.BaselineHead, WorktreeFP: plan.BaselineFP, ResultsMT: plan.ResultsMT,
		NonPass: gatePartialCounts{Fail: plan.Fail, Blocked: plan.Blocked, Report: plan.Report,
			SeedSteps: plan.Fail + plan.Blocked + plan.Report},
	}
	gatePartialLastBasedOn = on
	fp, head, ferr := worktreeFingerprint(root)
	rc := gatePartialExecute(root, script, plan, full, outdir, stdout, stderr)
	if ferr == nil {
		if werr := writeGateFreshnessPartial(outdir, root, fp, head, on); werr != nil {
			fmt.Fprintf(stderr, "%s: ⚠ 门新鲜度闸：部分趟指纹随件落不下去（%v）⇒ 这一趟没记指纹（退码不变）\n", progName, werr)
		}
	}
	return rc, true
}

// gatePartialBaselineDir —— 选定基线趟目录：`--since <目录>` 点名 / 裸 `--since` 取最新写入（= `--last` 同口径）。
func gatePartialBaselineDir(root, sinceArg string) (string, bool, error) {
	if sinceArg != "" {
		dir := sinceArg
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(root, dir)
		}
		if _, err := os.Stat(filepath.Join(dir, gateResultsTSV)); err != nil {
			return "", false, nil
		}
		return dir, true, nil
	}
	dir, _, ok, err := gateResultsFindLast()
	if err != nil {
		return "", false, err
	}
	return dir, ok, nil
}

// gatePartialValidateBaseline —— §3.5 硬前置（三条全过才是可用基线）：
// ★ **D3（已拍）**：前置③ 先落**最小判据**（`results.tsv` **行数 == N** 且**步名集合逐字相符**）——
//
//	 现读唯一能分辨「全量 vs 子集」的字段组合（`freshness.json` 存在 / `results-partial.tsv` 缺席
//	 **均不足**判全量）。「给趟目录加显式趟类型 `kind` 字段」作**后续硬化**（使判据降为 O(1)）。
//	1. 目录里**无** `results-partial.tsv`（有 ⇒ 它不是全量趟 · P3）；
//	2. 有 `freshness.json`（⇒ 有 `head` 基线给 ② 用）；
//	3. `results.tsv` 行数 == N **且**步名集合 == 现读步集（逐字相等）—— 最小拒收判据（§3.5 负验）。
func gatePartialValidateBaseline(dir string, full []string) (gateFreshnessRecord, []gateResultsRow, string) {
	if _, err := os.Stat(filepath.Join(dir, gateResultsPartialTSV)); err == nil {
		return gateFreshnessRecord{}, nil, "该趟目录里已有 " + gateResultsPartialTSV + " ⇒ 它**不是全量趟**"
	}
	rows, _, err := gateResultsReadTSV(filepath.Join(dir, gateResultsTSV))
	if err != nil {
		return gateFreshnessRecord{}, nil, "上趟 results.tsv 读不到"
	}
	rec, err := readGateFreshness(dir)
	if err != nil {
		return gateFreshnessRecord{}, nil, "该趟缺 freshness.json（基线 head 取不到 · ✗ 不退回 HEAD 凑一个）"
	}
	if len(rows) != len(full) {
		return gateFreshnessRecord{}, nil, fmt.Sprintf("上趟 results.tsv 行数 %d ≠ 现读步数 N=%d ⇒ 它不是全量趟（子集趟被误认全量 ⇒ 静默走错）", len(rows), len(full))
	}
	have := map[string]bool{}
	for _, r := range rows {
		have[r.Name] = true
	}
	miss := []string{}
	for _, n := range full {
		if !have[n] {
			miss = append(miss, n)
		}
	}
	if len(miss) > 0 {
		return gateFreshnessRecord{}, nil, fmt.Sprintf("上趟步名集合 ≠ 现读步集（漏 %d 步）⇒ 基线不可用", len(miss))
	}
	return rec, rows, ""
}

func gatePartialResultsMTime(dir string) string {
	if st, err := os.Stat(filepath.Join(dir, gateResultsTSV)); err == nil {
		return st.ModTime().Format(time.RFC3339)
	}
	return ""
}

// gatePartialMakeOutdir —— 这一趟部分趟的目录：`--outdir` 给了就用它（但**不许**它已带 results.tsv ⇒ §3.8 歧义 rc=2）；
// 没给就按脚本缺省式在 `<TMPDIR>` 下自建。
func gatePartialMakeOutdir(root string, tail []string) (string, error) {
	if dir := gateRunOutdirArg(tail); dir != "" {
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(root, dir)
		}
		if _, err := os.Stat(filepath.Join(dir, gateResultsTSV)); err == nil {
			return "", fmt.Errorf("`--outdir %s` 里已有 %s ⇒ 与 %s 共存即歧义（§3.8 fail-closed）", dir, gateResultsTSV, gateResultsPartialTSV)
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", err
		}
		return dir, nil
	}
	prefix := gateRunPrefix + "partial-" + time.Now().Format("20060102-150405") + "-"
	return os.MkdirTemp(os.TempDir(), prefix+"*")
}

// gatePartialExecute —— 逐 R 步执行（执行面 = 脚本自己，`--step <步名> --only-step`），
// 组结果到 `results-partial.tsv` + `partial.json`；日志**沿用全表 NN**（P4）。
// 退码（§五）：有 FAIL=1 · 无 FAIL 有 BLOCKED=2 · 否则=3（部分趟全绿 · **绝不退 0**）。
func gatePartialExecute(root, script string, plan *gatePartialPlan, full []string, outdir string, stdout, stderr io.Writer) int {
	idx := map[string]int{}
	for i, n := range full {
		idx[n] = i
	}
	src, err := os.ReadFile(script)
	if err != nil {
		fmt.Fprintf(stderr, "%s: 读不到门禁脚本 %s（%v）⇒ 部分趟不给结论（退码 2）\n", progName, gateScriptRel, err)
		return exitUsage
	}
	decls, _ := parseAddStepsStrict(string(src))
	byName := map[string]gateStepDecl{}
	for _, d := range decls {
		byName[normStepName(d.Name)] = d
	}

	rows := make([]gatePartialRow, 0, len(plan.R))
	for _, name := range plan.R {
		d, ok := byName[normStepName(name)]
		if !ok {
			fmt.Fprintf(stderr, "%s: 步名 %q 在脚本 add_step 里找不到 ⇒ 部分趟不给结论（退码 2）\n", progName, name)
			return exitUsage
		}
		sub, terr := os.MkdirTemp(outdir, ".step-")
		if terr != nil {
			fmt.Fprintf(stderr, "%s: 建单步临时目录失败（%v）⇒ 部分趟不给结论（退码 2）\n", progName, terr)
			return exitUsage
		}
		cmd := armChildGroup(exec.Command("bash", script, "--step", d.Name, "--only-step", "--outdir", sub))
		cmd.Dir = root
		cmd.Stdin = os.Stdin
		cmd.Stdout = stderr // 单步人面 → stderr（stdout 只留部分趟抬头 · 与 `gate run --step --json` 同规）
		cmd.Stderr = stderr
		runningChild = cmd
		_ = cmd.Run()
		runningChild = nil

		// 判决**从脚本自己的结果表读回**（不自己判 —— 唯一真源）。
		one, _, rerr := gateResultsReadTSV(filepath.Join(sub, gateResultsTSV))
		row := gatePartialRow{Status: "BLOCKED", Name: normStepName(name), RC: "?", Secs: "?", Log: "?"}
		if rerr == nil && len(one) > 0 {
			row = gatePartialRow{Status: one[0].Status, Name: one[0].Name, RC: one[0].RC, Secs: one[0].Secs, Log: one[0].Log}
			// 日志名：脚本按**本趟局部序**（`--only-step` ⇒ 00）落 ⇒ 改名成**全表 NN**（P4）。
			if base := filepath.Base(row.Log); base != "" {
				if ni, known := idx[row.Name]; known {
					newBase := fmt.Sprintf("%02d", ni) + base[2:]
					newPath := filepath.Join(outdir, newBase)
					if os.Rename(row.Log, newPath) == nil {
						row.Log = newPath
					}
				}
			}
		} else {
			fmt.Fprintf(stderr, "%s: 步 %q 没落结果表（读不到）⇒ 记为 BLOCKED（不许当绿）\n", progName, name)
		}
		rows = append(rows, row)
		_ = os.RemoveAll(sub)
	}

	rc := gatePartialExit(rows)
	if err := writeResultsPartial(outdir, rows); err != nil {
		fmt.Fprintf(stderr, "%s: 写 %s 失败（%v）⇒ 部分趟不给结论（退码 2）\n", progName, gateResultsPartialTSV, err)
		return exitUsage
	}
	if err := writePartialJSON(outdir, plan, rc); err != nil {
		fmt.Fprintf(stderr, "%s: 写 %s 失败（%v）⇒ 部分趟不给结论（退码 2）\n", progName, gatePartialJSONFile, err)
		return exitUsage
	}
	fmt.Fprintf(stdout, "── 部分趟完成：已跑 %d 步 · 台账 %s ──\n", len(rows), filepath.Join(outdir, gateResultsPartialTSV))
	return rc
}

// gatePartialExit —— 部分趟退码（§五）：有 FAIL ⇒ 1；无 FAIL 有 BLOCKED ⇒ 2；全绿 ⇒ 3（**绝不退 0**）。
func gatePartialExit(rows []gatePartialRow) int {
	blocked := false
	for _, r := range rows {
		switch r.Status {
		case "FAIL":
			return 1
		case "BLOCKED":
			blocked = true
		}
	}
	if blocked {
		return 2
	}
	return exitPartialRun
}

// ── D8 转全量那一路的趟目录交接 ＋ D10 分派兜底 ───────────────────────────────

// gatePartialFullOutdir —— 转全量那一路的趟目录（D8）：`gate.go` 据此给自举件补 `--outdir`，
// 使 `partial.json`（含五真源 triggers）与全量 `results.tsv` 落在**同一趟目录**。
var gatePartialFullOutdir string

func setGatePartialFullOutdir(dir string) { gatePartialFullOutdir = dir }

// gatePartialTakeFullOutdir —— 取走并清空（一次性交接）。
func gatePartialTakeFullOutdir() string {
	d := gatePartialFullOutdir
	gatePartialFullOutdir = ""
	return d
}

// gatePartialLastBasedOn —— D10：最近一次部分趟的基线四格；`gate.go` 的 `recordRunFreshness`
// 走部分趟分支（兜底分派）时取它。没有 ⇒ 零值（照实写空 based_on，不编造）。
var gatePartialLastBasedOn gatePartialBasedOn

func gatePartialBasedOnSnapshot() gatePartialBasedOn { return gatePartialLastBasedOn }

// ── D19（T-PR-24）· 同基线多趟「最近」引用规则 ───────────────────────────────
//
// 口径（★ 定理化 · §六负控⑥）：多趟部分趟**共享同一基线全量趟**（非链式 —— 裸 `--since` **永远**
// 选中同一趟全量，因部分趟**不写** `results.tsv`）⇒ 「引用上趟绿」时取：**该基线趟下、
// `results-partial.tsv` mtime 最新的一趟** 为引用源；✗ **不链式**（不许取「上一部分趟」——
// 那会让覆盖面**越滚越窄**）。
// ★ 只在 `baseDir` **同一基线**内取（据 D10 α 随件 `freshness-partial.json` 的 `based_on.dir` 判定），
//
//	✗ **不许跨基线**。
func gatePartialNewestSnapshot(baseDir string) (string, error) {
	base := os.TempDir()
	ents, err := os.ReadDir(base)
	if err != nil {
		return "", err
	}
	best := ""
	var bestT time.Time
	for _, e := range ents {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), gateRunPrefix) {
			continue
		}
		p := filepath.Join(base, e.Name())
		if p == baseDir {
			continue
		}
		if _, err := os.Stat(filepath.Join(p, gateResultsPartialTSV)); err != nil {
			continue
		}
		// 同一基线：读 α 随件 `freshness-partial.json` 的 `based_on.dir` ⇒ ✗ 跨基线不取。
		rec, err := readGateFreshnessPartial(p)
		if err != nil || rec.BasedOn.Dir != baseDir {
			continue
		}
		st, err := os.Stat(filepath.Join(p, gateResultsPartialTSV))
		if err != nil {
			continue
		}
		if best == "" || st.ModTime().After(bestT) {
			best, bestT = p, st.ModTime()
		}
	}
	if best == "" {
		return "", fmt.Errorf("基线 %s 下没有可引用的部分趟快照", baseDir)
	}
	return best, nil
}
