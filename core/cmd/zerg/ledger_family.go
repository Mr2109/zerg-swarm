// ledger_family.go —— 「多件族」账本登记口径的**唯一登记处**（GAP-20260927-323）。
// 门与生成器不得再各写一套件名规则；新账本名/新族名只进这张表。
//
// 收编面 v0：check_face_parity.go 的族名关键词、check_gap_axis_reconcile.go 的账本名。
// 其余散写 zerg-cli-gaps.jsonl 的件后续逐件收编（另立缺口，勿在本件扩大）。
//
// ── 族名口径（GAP-20260927-323 定档）─────────────────────────────────────────────
//
// 族 = 同前缀、后缀落在**闭集**内的件集：
//
//	前缀 = `缺口总账-<日期>`（日期两种写法：`YYYYMMDD` / `YYYY-MM-DD`）
//	后缀 ∈ { 空（索引件 / 单件名）, `-<面>-pNN`（面 ∈ ledgerFamilyFaces · NN 两位数字）,
//	          `-页`, `-附`, `-1` … `-9` }
//
// 收件面只有两处，且都调本件：门 `check_gap_ledger_view.go` 的 `glvCollectFamily` 与
// 生成器 `gen_gap_ledger.go` 的 `gglLedgerCandidates` / `gglPickFamily`。
// ★ 单件名只是族里「后缀=空」那一件 —— ✗ 不许任何一侧再自写 `^缺口总账-…$` 单件名正则：
// 那套写法会把页件挡在族外（旧读法 `gglLedgerNameRE` 与 `glvLedgerNameRE` 各写一份，
// 一套逻辑产出的族只有最后一件被看见 = 本缺口）。

package main

import (
	"regexp"
	"sort"
	"strings"
)

// ledgerFamilyNames —— 账本文件名（多件族成员名）登记表。
var ledgerFamilyNames = map[string]string{
	"gaps": "zerg-cli-gaps.jsonl",
}

// ledgerFamilyHints —— 门件名里判定「账本族」的关键词（族名口径）。
var ledgerFamilyHints = []string{"ledger", "judge"}

// ledgerFamilyName —— 族名前缀（唯一字面量；gen_gap_ledger.go 的 gglLedgerPrefix 引用本值）。
const ledgerFamilyName = "缺口总账"

// ledgerFamilyFaces —— 页件后缀里的「面」闭集（与门 check-gap-ledger-view 的 IMPACT_CLOSED 同集）。
var ledgerFamilyFaces = []string{"命令面", "门禁面", "文档面", "公开面", "换件面", "归档面"}

var (
	// ledgerFamilyDatePat —— 日期两种写法；`\p{Nd}` 对齐 py 的 `\d`（Unicode 数字）。
	ledgerFamilyDatePat = `(\p{Nd}{8}|\p{Nd}{4}-\p{Nd}{2}-\p{Nd}{2})`
	// ledgerFamilyIndexRE —— 族里「后缀=空」那一件（索引件 · 亦即旧读法的「单件名」）。
	ledgerFamilyIndexRE = regexp.MustCompile(`^` + ledgerFamilyName + `-` + ledgerFamilyDatePat + `\.md$`)
	// ledgerFamilyPageRE —— 族里「面 + 页号」那一类页件（组：1=日期 · 2=面 · 3=页号两位）。
	ledgerFamilyPageRE = regexp.MustCompile(`^` + ledgerFamilyName + `-` + ledgerFamilyDatePat +
		`-(` + strings.Join(ledgerFamilyFaces, "|") + `)-p(\p{Nd}{2})\.md$`)
	// ledgerFamilySuffixRE —— 族里其余闭集后缀（`-页` / `-附` / `-1..-9`）。
	ledgerFamilySuffixRE = regexp.MustCompile(`^` + ledgerFamilyName + `-` + ledgerFamilyDatePat +
		`-(?:页|附|[1-9])\.md$`)
)

// ledgerFamilyPiece —— 一件族成员名的拆解结果（门与生成器共用同一份拆法）。
type ledgerFamilyPiece struct {
	Key  string // 族键 = `缺口总账-<日期>`（逐字，不归一化）
	Kind string // "index" / "page" / "member"
	Date string // 件名里的日期（逐字：`20261018` 或 `2026-10-18`）
	Face string // 页件的面（六值闭集之一），其余为空
	Page string // 页件的页号（两位数字），其余为空
}

// ledgerFamilySplit —— 件名 → (族成员拆解, 是否族成员)。
// ok=false ⇒ 该件不属于任何账本族（✗ 不许当「后缀任取」放行）。
func ledgerFamilySplit(name string) (ledgerFamilyPiece, bool) {
	if m := ledgerFamilyIndexRE.FindStringSubmatch(name); m != nil {
		return ledgerFamilyPiece{Key: ledgerFamilyName + "-" + m[1], Kind: "index", Date: m[1]}, true
	}
	if m := ledgerFamilyPageRE.FindStringSubmatch(name); m != nil {
		return ledgerFamilyPiece{Key: ledgerFamilyName + "-" + m[1], Kind: "page",
			Date: m[1], Face: m[2], Page: m[3]}, true
	}
	if m := ledgerFamilySuffixRE.FindStringSubmatch(name); m != nil {
		return ledgerFamilyPiece{Key: ledgerFamilyName + "-" + m[1], Kind: "member", Date: m[1]}, true
	}
	return ledgerFamilyPiece{}, false
}

// ledgerFamilyKey —— 件名 → (族键, 是否族成员)；族键 = `缺口总账-<日期>`。
func ledgerFamilyKey(name string) (string, bool) {
	p, ok := ledgerFamilySplit(name)
	return p.Key, ok
}

// ledgerFamilyIsIndex —— 该件是否族里的索引件（后缀=空 ⇒ 旧读法的「单件名」）。
func ledgerFamilyIsIndex(name string) bool {
	p, ok := ledgerFamilySplit(name)
	return ok && p.Kind == "index"
}

// ledgerFamilyDateKey —— 族键里的日期归一成「只留数字」（`20261018` / `2026-10-18` ⇒ 同值）。
// 门与生成器挑「最新族」时同一把尺子（✗ 各自按字符串比）。
func ledgerFamilyDateKey(key string) string {
	var b strings.Builder
	for _, r := range strings.TrimPrefix(key, ledgerFamilyName+"-") {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ledgerFamilies —— 件名集 → (族键有序表, 族键 → 成员件名有序表)。
// 只有真族成员进表（ledgerFamilySplit 判 false 的件一律不进）。
func ledgerFamilies(names []string) ([]string, map[string][]string) {
	m := map[string][]string{}
	for _, n := range names {
		if k, ok := ledgerFamilyKey(n); ok {
			m[k] = append(m[k], n)
		}
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		sort.Strings(m[k])
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys, m
}
