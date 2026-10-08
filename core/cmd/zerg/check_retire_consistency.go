// check_retire_consistency.go —— 判据件 `scripts/gates/check-retire-consistency.py` 的 Go 真搬
// （M2 · 入口 `checkRetireConsistencyGateMain` · 承接缺口 GAP-20261008-05）。
//
// 义（缺口逐字）：凡 `retire_ledger` 已登记且**不在盘**的件，不得在任何登记/排除面留**悬空行**。
//
// 口径参照（✗ 另起语汇）：门75「戊 件面现读存在性」（Go 半 `check_gap_axis_reconcile.go` 的
//
//	gapAxisRecExistenceHits）—— 存在性 = **现读** `git -C <仓> ls-files -z`（无缓存）；命中词汇
//	「悬空行 / 存量豁免」逐字沿用；拿不到取数面 ⇒ rc=2 **不给结论**。
//
// 验收判据 = 对同一 argv 跑两态（py 原件 ⟷ 本入口），stdout / stderr / 退码**三面逐字相同**。
//
// ★ 本件只落码、不接线（不改 main.go 命令树 · 不删 py 原件）。inproc 键登记见 gate_inproc_table.go。
//
// 退码（与 py 逐字同）：0 无**新**悬空（存量棘轮内） · 1 有新悬空（tri-report ⇒ 落 REPORT） ·
//
//	2 取数面拿不到（账本读不了 / git ls-files 失败 ⇒ 不给结论）。
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const crcTool = "check-retire-consistency.py"
const crcVersion = "v1.0.0"
const crcGateName = "退件↔登记/排除面 一致性（retire_ledger 已退且不在盘者不得留悬空行）"
const crcLedgerRel = "core/internal/contract/adopt-retire.json"

const (
	crcRCOK     = 0
	crcRCReport = 1 // 有新悬空（tri-report ⇒ 落 REPORT）
	crcRCBlock  = 2 // 取数面拿不到 ⇒ 不给结论
)

// ── 例外白名单：有意保留的 (surface_id, ref) + 逐条理由（✗ 非静默跳过）───────────
type crcKeep struct{ sid, ref, reason string }

var crcKeeps = []crcKeep{}

// （首读现读：见 `--baseline` 输出。空 = 首读无「有意保留」项；只增不删。）

// ── 冷启动棘轮基线（存量表）：首读即存在的 (surface_id, ref) 对 ⇒ 只打印、不进退码 ──
// 表外的（=**新出现的**）才是「悬空」⇒ rc=1。**只增不删**。
// ★ 2026-10-08 首读现读（真账 142 退件 · `git ls-files` 现跑 · 悬空 13 行）：首读收成 3 对（已清理见下）。
var crcBaseline = [][2]string{
	// ★ 2026-10-08 清理批（GAP-20261008-05）：首读收的 3 对（registry 9 条 + 两器 4 条）已逐面**摘行清空**
	//   ⇒ 棘轮基线**同步到新值 = 空**（照现读机制：表外的才是「悬空」；清完悬空/存量均 0）。
}

// ── 九面（面 ④「两器」= 一件面两件 ⇒ 恰八面 ＋ 2026-10-08 切源补第 ⑨ 面 = 行式排除表）──────
type crcPart struct{ rel, ex string }
type crcSurface struct {
	id, label string
	parts     []crcPart
}

var crcSurfaces = []crcSurface{
	{"marks", "公开标记.tsv", []crcPart{{"scripts/公开标记.tsv", "marks"}}},
	{"private", "private-paths.txt", []crcPart{{"publish/private-paths.txt", "private"}}},
	{"dangling", "dangling-refs-registry.json", []crcPart{{"scripts/gates/dangling-refs-registry.json", "dangling"}}},
	{"two", "两器 EXCLUDES/DROP", []crcPart{
		{"scripts/build/publish-public.sh", "excludes"},
		{"publish/mirror-public-lib.py", "drop"},
	}},
	{"coverage", "gate-coverage.config.json", []crcPart{{"scripts/gates/gate-coverage.config.json", "coverage"}}},
	{"affected", "affected-map.json", []crcPart{{"scripts/gates/affected-map.json", "affected"}}},
	{"judge", "judge-registry.json", []crcPart{{"scripts/gates/judge-registry.json", "judge"}}},
	{"slice", "slice-probes.json", []crcPart{{"scripts/gates/slice-probes.json", "slice"}}},
	// ★ 2026-10-08 切源（`GAP-20260926-223` 账[3]）：行式排除表真源也进覆盖面 ——
	//   已退件不得在 `publish/excludes.txt` 里留**悬空行**（今天那条空指正是这面该抓而没抓的）。
	{"excl", "publish/excludes.txt", []crcPart{{"publish/excludes.txt", "excllines"}}},
}

// ── 取数工具 ─────────────────────────────────────────────────────────────────
func crcReadText(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func crcStripQuoted(line string) string {
	s := strings.TrimSpace(line)
	if strings.HasPrefix(s, "#") {
		return ""
	}
	m := regexp.MustCompile(`^"([^"]*)"`).FindStringSubmatch(s)
	if m == nil {
		return ""
	}
	return m[1]
}

func crcBlock(txt, headerRe string) string {
	re := regexp.MustCompile(headerRe)
	loc := re.FindStringIndex(txt)
	if loc == nil {
		return ""
	}
	var b strings.Builder
	depth := 1
	for j := loc[1]; j < len(txt); j++ {
		ch := txt[j]
		switch ch {
		case '{', '(', '[':
			depth++
		case '}', ')', ']':
			depth--
			if depth == 0 {
				return b.String()
			}
		}
		b.WriteByte(ch)
	}
	return b.String()
}

func crcSplitLines(s string) []string { return strings.Split(s, "\n") }

// ── 抽取器 ───────────────────────────────────────────────────────────────────
func crcExMarks(txt string) []string {
	var out []string
	for _, ln := range crcSplitLines(txt) {
		if strings.TrimSpace(ln) == "" || strings.HasPrefix(strings.TrimLeft(ln, " \t"), "#") {
			continue
		}
		cols := strings.Split(ln, "\t")
		if len(cols) > 0 && strings.TrimSpace(cols[0]) != "" {
			out = append(out, strings.TrimSpace(cols[0]))
		}
	}
	return out
}

func crcExPrivate(txt string) []string {
	var out []string
	for _, ln := range crcSplitLines(txt) {
		s := strings.TrimSpace(ln)
		if s == "" || strings.HasPrefix(s, "#") {
			continue
		}
		out = append(out, s)
	}
	return out
}

func crcAsMap(v interface{}) (map[string]interface{}, bool) {
	m, ok := v.(map[string]interface{})
	return m, ok
}

func crcAsList(v interface{}) ([]interface{}, bool) {
	l, ok := v.([]interface{})
	return l, ok
}

func crcExDangling(txt string) []string {
	var j map[string]interface{}
	if err := json.Unmarshal([]byte(txt), &j); err != nil {
		return nil
	}
	var out []string
	if l, ok := crcAsList(j["entries"]); ok {
		for _, e := range l {
			if m, ok := crcAsMap(e); ok {
				if s, ok := m["file"].(string); ok && s != "" {
					out = append(out, s)
				}
			}
		}
	}
	return out
}

func crcExExcludes(txt string) []string {
	body := crcBlock(txt, `EXCLUDES\s*=\s*\(`)
	var out []string
	for _, ln := range crcSplitLines(body) {
		if v := crcStripQuoted(ln); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// crcExExclLines —— 行式排除表真源 `publish/excludes.txt`（切源 2026-10-08 · `GAP-20260926-223`
// 账[3]）。行口径 = 唯一 Go 解析件 `excludes_rules.go::parseExcludesRules`（strip 传 `strings.TrimSpace`）。
func crcExExclLines(txt string) []string {
	return parseExcludesRules(crcSplitLines(txt), strings.TrimSpace)
}

func crcExDrop(txt string) []string {
	var out []string
	for _, hdr := range []string{`DROP_EXACT\s*=\s*\{`, `DROP_PREFIX\s*=\s*\(`} {
		body := crcBlock(txt, hdr)
		for _, ln := range crcSplitLines(body) {
			if v := crcStripQuoted(ln); v != "" {
				out = append(out, v)
			}
		}
	}
	return out
}

func crcStrList(v interface{}) []string {
	var out []string
	if l, ok := crcAsList(v); ok {
		for _, x := range l {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
	}
	return out
}

func crcExCoverage(txt string) []string {
	var j map[string]interface{}
	if err := json.Unmarshal([]byte(txt), &j); err != nil {
		return nil
	}
	var out []string
	if scan, ok := crcAsMap(j["scan"]); ok {
		out = append(out, crcStrList(scan["script_globs"])...)
	}
	if wl, ok := crcAsMap(j["whitelist"]); ok {
		for _, k := range []string{"build_dirs", "scripts"} {
			if l, ok := crcAsList(wl[k]); ok {
				for _, e := range l {
					if m, ok := crcAsMap(e); ok {
						if s, ok := m["path"].(string); ok {
							out = append(out, s)
						}
					}
				}
			}
		}
	}
	if ex, ok := crcAsMap(j["exclude"]); ok {
		out = append(out, crcStrList(ex["path_components"])...)
		out = append(out, crcStrList(ex["prefixes"])...)
	}
	return out
}

func crcExAffected(txt string) []string {
	var j map[string]interface{}
	if err := json.Unmarshal([]byte(txt), &j); err != nil {
		return nil
	}
	var out []string
	if l, ok := crcAsList(j["files"]); ok {
		for _, e := range l {
			if m, ok := crcAsMap(e); ok {
				if s, ok := m["pattern"].(string); ok {
					out = append(out, s)
				}
			}
		}
	}
	if faces, ok := crcAsMap(j["faces"]); ok {
		for _, v := range faces {
			out = append(out, crcStrList(v)...)
		}
	}
	return out
}

func crcExJudge(txt string) []string {
	var j map[string]interface{}
	if err := json.Unmarshal([]byte(txt), &j); err != nil {
		return nil
	}
	var out []string
	if l, ok := crcAsList(j["entries"]); ok {
		for _, e := range l {
			if m, ok := crcAsMap(e); ok {
				if s, ok := m["path"].(string); ok {
					out = append(out, s)
				}
			}
		}
	}
	return out
}

func crcCollectStrings(o interface{}, out *[]string) {
	switch v := o.(type) {
	case string:
		*out = append(*out, v)
	case []interface{}:
		for _, x := range v {
			crcCollectStrings(x, out)
		}
	case map[string]interface{}:
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			crcCollectStrings(v[k], out)
		}
	}
}

func crcExSlice(txt string) []string {
	var j map[string]interface{}
	if err := json.Unmarshal([]byte(txt), &j); err != nil {
		return nil
	}
	var out []string
	if l, ok := crcAsList(j["probes"]); ok {
		for _, e := range l {
			if m, ok := crcAsMap(e); ok {
				crcCollectStrings(m["slice"], &out)
			}
		}
	}
	return out
}

func crcExtract(ex, txt string) ([]string, error) {
	switch ex {
	case "marks":
		return crcExMarks(txt), nil
	case "private":
		return crcExPrivate(txt), nil
	case "dangling":
		return crcExDangling(txt), nil
	case "excludes":
		return crcExExcludes(txt), nil
	case "excllines":
		return crcExExclLines(txt), nil
	case "drop":
		return crcExDrop(txt), nil
	case "coverage":
		return crcExCoverage(txt), nil
	case "affected":
		return crcExAffected(txt), nil
	case "judge":
		return crcExJudge(txt), nil
	case "slice":
		return crcExSlice(txt), nil
	}
	return nil, fmt.Errorf("未知抽取器：%s", ex)
}

// ── 取数面 ───────────────────────────────────────────────────────────────────
func crcRepoRoot(arg string) string {
	if arg != "" {
		a, _ := filepath.Abs(arg)
		return a
	}
	if env := os.Getenv("ZERG_REPO_ROOT"); env != "" {
		a, _ := filepath.Abs(env)
		return a
	}
	if env := os.Getenv("ZERG_REPO"); env != "" {
		a, _ := filepath.Abs(env)
		return a
	}
	p, _ := os.Getwd()
	for {
		if st, err := os.Stat(filepath.Join(p, ".git")); err == nil && st.IsDir() {
			return p
		}
		nxt := filepath.Dir(p)
		if nxt == p {
			wd, _ := os.Getwd()
			return wd
		}
		p = nxt
	}
}

func crcGitTracked(root string) (map[string]bool, string) {
	cmd := exec.Command("git", "-C", root, "-c", "core.quotepath=false", "ls-files", "-z")
	var outb, errb strings.Builder
	cmd.Stdout = &outb
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		code := -1
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		}
		return nil, fmt.Sprintf("git ls-files 失败（rc=%d）：%s", code, strings.TrimSpace(errb.String()))
	}
	set := map[string]bool{}
	for _, x := range strings.Split(outb.String(), "\x00") {
		if x != "" {
			set[x] = true
		}
	}
	return set, ""
}

func crcLoadLedger(root string) (retired []string, exceptions []string, errMsg string) {
	path := filepath.Join(root, crcLedgerRel)
	if _, err := os.Stat(path); err != nil {
		return nil, nil, "账本不在盘：" + crcLedgerRel
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, "账本读不了：" + err.Error()
	}
	var j map[string]interface{}
	if err := json.Unmarshal(b, &j); err != nil {
		return nil, nil, "账本解析失败：" + err.Error()
	}
	if l, ok := crcAsList(j["retire_ledger"]); ok {
		for _, e := range l {
			if m, ok := crcAsMap(e); ok {
				if s, ok := m["object"].(string); ok && s != "" {
					retired = append(retired, s)
				}
			}
		}
	}
	if l, ok := crcAsList(j["retire_exceptions"]); ok {
		for _, e := range l {
			if m, ok := crcAsMap(e); ok {
				if s, ok := m["object"].(string); ok && s != "" {
					exceptions = append(exceptions, s)
				}
			}
		}
	}
	return retired, exceptions, ""
}

func crcIsDangling(ref string, absent map[string]bool) bool {
	if absent[ref] {
		return true
	}
	if strings.HasSuffix(ref, "/") {
		for a := range absent {
			if strings.HasPrefix(a, ref) {
				return true
			}
		}
	}
	return false
}

// ── 判定 ─────────────────────────────────────────────────────────────────────
type crcRowKind struct{ ref, kind string }
type crcSurfaceResult struct {
	id, label, rels, err string
	rows                 []crcRowKind
}
type crcReport struct {
	trackedN, retiredN, exceptionsN int
	absent                          []string
	perSurface                      []crcSurfaceResult
	dangling                        [][3]string // sid,label,ref
	stock                           [][3]string
	exempt                          [][3]string // sid,ref,reason
}

func crcCompute(root string, tracked map[string]bool, retired, exceptions []string, basePairs [][2]string) crcReport {
	absSet := map[string]bool{}
	for _, p := range retired {
		if !tracked[p] {
			absSet[p] = true
		}
	}
	absent := make([]string, 0, len(absSet))
	for p := range absSet {
		absent = append(absent, p)
	}
	sort.Strings(absent)
	exemptSet := map[string]bool{}
	for _, e := range exceptions {
		exemptSet[e] = true
	}
	keepSet := map[[2]string]string{}
	for _, k := range crcKeeps {
		keepSet[[2]string{k.sid, k.ref}] = k.reason
	}
	baseSet := map[[2]string]bool{}
	for _, b := range basePairs {
		baseSet[b] = true
	}

	var rep crcReport
	rep.trackedN = len(tracked)
	rep.retiredN = len(retired)
	rep.exceptionsN = len(exceptions)
	rep.absent = absent
	for _, s := range crcSurfaces {
		var refs []string
		var rels []string
		errStr := ""
		for _, p := range s.parts {
			rels = append(rels, p.rel)
			path := filepath.Join(root, p.rel)
			if _, err := os.Stat(path); err != nil {
				if errStr == "" {
					errStr = "面不在盘：" + p.rel
				}
				continue
			}
			txt, rerr := crcReadText(path)
			if rerr != nil {
				if errStr == "" {
					errStr = "解析失败：" + rerr.Error() + "（" + p.rel + "）"
				}
				continue
			}
			got, xerr := crcExtract(p.ex, txt)
			if xerr != nil {
				if errStr == "" {
					errStr = "解析失败：" + xerr.Error() + "（" + p.rel + "）"
				}
				continue
			}
			refs = append(refs, got...)
		}
		res := crcSurfaceResult{id: s.id, label: s.label, rels: strings.Join(rels, " ＋ "), err: errStr}
		if errStr == "" {
			for _, ref := range refs {
				if !crcIsDangling(ref, absSet) {
					continue
				}
				switch {
				case exemptSet[ref]:
					res.rows = append(res.rows, crcRowKind{ref, "例外·退场例外名册"})
					rep.exempt = append(rep.exempt, [3]string{s.id, ref, "retire_exceptions 名册（有意保留）"})
				case keepSet[[2]string{s.id, ref}] != "":
					res.rows = append(res.rows, crcRowKind{ref, "例外·白名单"})
					rep.exempt = append(rep.exempt, [3]string{s.id, ref, keepSet[[2]string{s.id, ref}]})
				case baseSet[[2]string{s.id, ref}]:
					res.rows = append(res.rows, crcRowKind{ref, "存量"})
					rep.stock = append(rep.stock, [3]string{s.id, s.label, ref})
				default:
					res.rows = append(res.rows, crcRowKind{ref, "悬空"})
					rep.dangling = append(rep.dangling, [3]string{s.id, s.label, ref})
				}
			}
		}
		rep.perSurface = append(rep.perSurface, res)
	}
	return rep
}

func crcCount(rows []crcRowKind, kind string) int {
	n := 0
	for _, r := range rows {
		if r.kind == kind {
			n++
		}
	}
	return n
}

// ── 渲染（与 py 逐字对齐）────────────────────────────────────────────────────
func crcRender(rep crcReport, unknown string) string {
	var out []string
	out = append(out, "── "+crcGateName+" ──")
	out = append(out, "取数面 : "+crcLedgerRel)
	out = append(out, fmt.Sprintf("现读   : git ls-files 共 %d 件 ｜ retire_ledger %d 条 ｜ retire_exceptions %d 条 ｜ 已退且不在盘 %d 件",
		rep.trackedN, rep.retiredN, rep.exceptionsN, len(rep.absent)))
	out = append(out, "")
	totalNew, totalStock := 0, 0
	for _, s := range rep.perSurface {
		dang := crcCount(s.rows, "悬空")
		stock := crcCount(s.rows, "存量")
		exc := len(s.rows) - dang - stock
		totalNew += dang
		totalStock += stock
		mark := "✓"
		if dang > 0 {
			mark = "✗"
		} else if s.err != "" {
			mark = "?"
		}
		note := ""
		if s.err != "" {
			note = "  [" + s.err + "]"
		}
		out = append(out, fmt.Sprintf("  %s 面 %-24s 悬空 %d ｜ 存量 %d ｜ 例外 %d%s", mark, s.label, dang, stock, exc, note))
		for _, r := range s.rows {
			switch r.kind {
			case "悬空":
				out = append(out, "        · 新悬空 ← "+r.ref)
			case "存量":
				out = append(out, "        · 存量 ← "+r.ref)
			default:
				out = append(out, "        · 例外 ← "+r.ref)
			}
		}
	}
	out = append(out, "")
	out = append(out, fmt.Sprintf("合计 : 新悬空 %d 行 ｜ 存量（棘轮内）%d 行 ｜ 例外 %d 行", totalNew, totalStock, len(rep.exempt)))
	if len(rep.exempt) > 0 {
		out = append(out, "")
		out = append(out, "例外/白名单（逐条理由 · ✗ 非静默跳过）：")
		for _, e := range rep.exempt {
			out = append(out, fmt.Sprintf("  · [%s] %s —— %s", e[0], e[1], e[2]))
		}
	}
	out = append(out, "")
	out = append(out, "档 : tri-report（只报告 · 新悬空 rc=1 落 REPORT · ✗ 不阻断）")
	if unknown != "" {
		out = append(out, "✗ 取数面拿不到："+unknown+" ⇒ **不给结论**（rc=2）")
	}
	return strings.Join(out, "\n")
}

func crcBaselineTSV(rep crcReport) string {
	out := []string{"surface\tpath\tnew_dangling\tstock\texempt"}
	for _, s := range rep.perSurface {
		dang := crcCount(s.rows, "悬空")
		stock := crcCount(s.rows, "存量")
		exc := len(s.rows) - dang - stock
		out = append(out, fmt.Sprintf("%s\t%s\t%d\t%d\t%d", s.label, s.rels, dang, stock, exc))
	}
	out = append(out, fmt.Sprintf("TOTAL\t_\t%d\t%d\t%d", len(rep.dangling), len(rep.stock), len(rep.exempt)))
	return strings.Join(out, "\n")
}

// ── self-test（合成夹具 · 不碰真仓）───────────────────────────────────────────
func crcSelfTest(stdout io.Writer) int {
	base, err := os.MkdirTemp("", "retire-consistency-selftest-")
	if err != nil {
		fmt.Fprintf(stdout, "✗ 自检：建临时目录失败：%v\n", err)
		return 1
	}
	defer os.RemoveAll(base)

	type crcCheck struct {
		label     string
		ok        bool
		got, want string
	}
	var checks []crcCheck
	nf := 0
	report := func(label, got, want string) {
		ok := got == want
		checks = append(checks, crcCheck{label, ok, got, want})
		if !ok {
			nf++
		}
		mark := "✓"
		if !ok {
			mark = "✗"
		}
		fmt.Fprintf(stdout, "  %s %s ⇒ 得 %s（要 %s）\n", mark, label, got, want)
	}

	if err := os.MkdirAll(filepath.Join(base, "core", "internal", "contract"), 0o755); err != nil {
		fmt.Fprintf(stdout, "✗ 自检：建目录失败：%v\n", err)
		return 1
	}
	for _, d := range []string{"scripts/gates", "publish", "scripts/build", "scripts/x"} {
		_ = os.MkdirAll(filepath.Join(base, filepath.FromSlash(d)), 0o755)
	}
	ledger := map[string]interface{}{
		"retire_ledger": []interface{}{
			map[string]interface{}{"object": "scripts/x/GONE.py", "state": "已删"},
			map[string]interface{}{"object": "scripts/x/GONE2.py", "state": "已删"},
		},
		"retire_exceptions": []interface{}{},
	}
	lb, _ := json.Marshal(ledger)
	_ = os.WriteFile(filepath.Join(base, filepath.FromSlash(crcLedgerRel)), lb, 0o644)
	_ = os.WriteFile(filepath.Join(base, filepath.FromSlash("scripts/x/GONE2.py")), []byte{}, 0o644)
	_ = os.WriteFile(filepath.Join(base, filepath.FromSlash("scripts/x/LIVE.py")), []byte{}, 0o644)
	_ = os.WriteFile(filepath.Join(base, filepath.FromSlash("scripts/公开标记.tsv")),
		[]byte("文件\tpublic\t理由\nscripts/x/GONE.py\tno\t测试\n# scripts/x/GONE.py 注释不算\n"), 0o644)
	_ = os.WriteFile(filepath.Join(base, filepath.FromSlash("publish/private-paths.txt")),
		[]byte("# c\nscripts/x/GONE.py\n"), 0o644)
	_ = os.WriteFile(filepath.Join(base, filepath.FromSlash("scripts/build/publish-public.sh")),
		[]byte("EXCLUDES=(\n  \"scripts/x/GONE.py\"\n  # \"scripts/x/GONE.py\" 注释\n)\n"), 0o644)
	_ = os.WriteFile(filepath.Join(base, filepath.FromSlash("publish/mirror-public-lib.py")),
		[]byte("DROP_EXACT = {\n    \"scripts/x/GONE.py\",\n}\nDROP_PREFIX = (\n    \"scripts/x/\",\n)\n"), 0o644)
	// ★ 九面夹具（切源 2026-10-08）：行式排除表真源（`publish/excludes.txt`）。
	exclRel := "publish/excludes.txt"
	_ = os.WriteFile(filepath.Join(base, filepath.FromSlash(exclRel)),
		[]byte("# c\nscripts/x/LIVE.py\n"), 0o644)
	for rel, body := range map[string]string{
		"scripts/gates/dangling-refs-registry.json": `{"entries":[]}`,
		"scripts/gates/gate-coverage.config.json":   `{"scan":{"script_globs":[]},"exclude":{},"whitelist":{}}`,
		"scripts/gates/affected-map.json":           `{"files":[],"faces":{}}`,
		"scripts/gates/judge-registry.json":         `{"entries":[]}`,
		"scripts/gates/slice-probes.json":           `{"probes":[]}`,
	} {
		_ = os.WriteFile(filepath.Join(base, filepath.FromSlash(rel)), []byte(body), 0o644)
	}
	// 真 git 定存在性（ls-files 只需 add）
	_ = exec.Command("git", "init", "-q", base).Run()
	_ = exec.Command("git", "-C", base, "add", "-A").Run()
	tracked, gerr := crcGitTracked(base)
	report("selftest·git 可取数", gerr, "")
	ret, exc, lerr := crcLoadLedger(base)
	report("selftest·账本可读", lerr, "")
	rep := crcCompute(base, tracked, ret, exc, crcBaseline)
	report("selftest·absent 只 GONE.py", strings.Join(rep.absent, ","), "scripts/x/GONE.py")
	report("selftest·新悬空行数（含 DROP_PREFIX 前缀命中）", fmt.Sprint(len(rep.dangling)), "5")
	report("selftest·例外行数", fmt.Sprint(len(rep.exempt)), "0")
	// ★ py 独有断言搬入（2026-10-08 分叉普查）：取数面拿不到（缺账本）⇒ 错误串非空
	//   （rc=2 档 · 防「静默当空账」）。判据在 crcLoadLedger：账本不在盘 ⇒ 返回非空错误串。
	_, _, lerr2 := crcLoadLedger(filepath.Join(base, "不存在的根"))
	report("selftest·缺账本 ⇒ 报错非空", fmt.Sprint(lerr2 != ""), "true")
	// ★ 九面（切源 2026-10-08 · `GAP-20260926-223` 账[3]）：行式排除表面进覆盖面（八面→九面）。
	report("selftest·九面（含 publish/excludes.txt 面）", fmt.Sprint(len(rep.perSurface)), "9")
	// ★ 九面有牙：往排除表塞一条**已退且不在盘**的件 ⇒ 该面出新悬空（GONE.py 此时仍不在盘）。
	_ = os.WriteFile(filepath.Join(base, filepath.FromSlash(exclRel)),
		[]byte("# c\nscripts/x/LIVE.py\nscripts/x/GONE.py\n"), 0o644)
	repX := crcCompute(base, tracked, ret, exc, crcBaseline)
	report("负控·排除表塞一条已退件 ⇒ 新悬空 6（九面有牙）", fmt.Sprint(len(repX.dangling)), "6")
	_ = os.WriteFile(filepath.Join(base, filepath.FromSlash(exclRel)),
		[]byte("# c\nscripts/x/LIVE.py\n"), 0o644) // 复原（不污染后续读数）
	repB := crcCompute(base, tracked, ret, exc, [][2]string{{"marks", "scripts/x/GONE.py"}})
	report("selftest·棘轮命中 ⇒ 新悬空 4", fmt.Sprint(len(repB.dangling)), "4")
	report("selftest·棘轮命中 ⇒ 存量 1", fmt.Sprint(len(repB.stock)), "1")
	// 负控：把 GONE.py 造回盘 ⇒ 悬空归 0
	_ = os.WriteFile(filepath.Join(base, filepath.FromSlash("scripts/x/GONE.py")), []byte{}, 0o644)
	_ = exec.Command("git", "-C", base, "add", "-A").Run()
	tracked2, _ := crcGitTracked(base)
	rep2 := crcCompute(base, tracked2, ret, exc, crcBaseline)
	report("负控·件造回盘 ⇒ 悬空 0", fmt.Sprint(len(rep2.dangling)), "0")

	fmt.Fprintf(stdout, "self-test：%d 格 · %d 失败\n", len(checks), nf)
	if nf == 0 {
		return crcRCOK
	}
	return crcRCReport
}

// ── 入口（argparse 同构的旗标面）────────────────────────────────────────────
type crcArgs struct {
	root     string
	check    bool
	baseline bool
	json     bool
	selfTest bool
	version  bool
	help     bool
}

const crcUsage = "usage: check-retire-consistency.py [-h] [--check] [--baseline] [--json]\n" +
	"                                        [--self-test] [--repo-root REPO_ROOT]\n" +
	"                                        [--version]\n"

const crcHelpText = crcUsage +
	"\n" +
	"退件↔登记/排除面 一致性（retire_ledger 已退且不在盘者不得留悬空行 · 只报告）\n" +
	"\n" +
	"optional arguments:\n" +
	"  -h, --help            show this help message and exit\n" +
	"  --check               只报告（tri-report 默认档）\n" +
	"  --baseline            出基线表（TSV）\n" +
	"  --json                机器可读输出\n" +
	"  --self-test           合成夹具成对负控（不碰真目标）\n" +
	"  --repo-root REPO_ROOT 仓根（也认 --root / env ZERG_REPO_ROOT / ZERG_REPO / cwd 上溯）\n" +
	"  --version             打印版本\n"

func crcParseArgs(argv []string) (crcArgs, string) {
	a := crcArgs{}
	for i := 0; i < len(argv); i++ {
		tok := argv[i]
		switch {
		case tok == "-h" || tok == "--help":
			a.help = true
		case tok == "--check":
			a.check = true
		case tok == "--baseline":
			a.baseline = true
		case tok == "--json":
			a.json = true
		case tok == "--self-test":
			a.selfTest = true
		case tok == "--version":
			a.version = true
		case tok == "--repo-root" || tok == "--root":
			if i+1 >= len(argv) {
				return a, "argument " + tok + ": expected one argument"
			}
			i++
			a.root = argv[i]
		case strings.HasPrefix(tok, "--repo-root="):
			a.root = strings.TrimPrefix(tok, "--repo-root=")
		case strings.HasPrefix(tok, "--root="):
			a.root = strings.TrimPrefix(tok, "--root=")
		default:
			return a, "unrecognized arguments: " + tok
		}
	}
	return a, ""
}

// checkRetireConsistencyGateMain 是 `zerg gate check check-retire-consistency` 的进程内入口。
func checkRetireConsistencyGateMain(stdout, stderr io.Writer, argv []string) int {
	a, errMsg := crcParseArgs(argv)
	if errMsg != "" {
		fmt.Fprintf(stderr, "%s%s: error: %s\n", crcUsage, crcTool, errMsg)
		return crcRCBlock
	}
	if a.help {
		fmt.Fprint(stdout, crcHelpText)
		return crcRCOK
	}
	if a.version {
		fmt.Fprintf(stdout, "%s %s\n", crcTool, crcVersion)
		return crcRCOK
	}
	if a.selfTest {
		return crcSelfTest(stdout)
	}

	root := crcRepoRoot(a.root)
	tracked, gerr := crcGitTracked(root)
	ret, exc, lerr := crcLoadLedger(root)
	unknown := ""
	switch {
	case gerr != "":
		unknown = gerr
	case lerr != "":
		unknown = lerr
	}
	if unknown != "" {
		fmt.Fprintf(stderr, "%s: %s\n", crcTool, unknown)
		fmt.Fprintln(stderr, "档 : tri-report · ✗ 取数面拿不到 ⇒ 不给结论（rc=2）")
		return crcRCBlock
	}

	rep := crcCompute(root, tracked, ret, exc, crcBaseline)
	if a.json {
		rep.outJSON(stdout)
		return crcRCOKorReport(rep)
	}
	if a.baseline {
		fmt.Fprint(stdout, crcBaselineTSV(rep)+"\n")
		return crcRCOKorReport(rep)
	}
	fmt.Fprint(stdout, crcRender(rep, "")+"\n")
	return crcRCOKorReport(rep)
}

func crcRCOKorReport(rep crcReport) int {
	if len(rep.dangling) > 0 {
		return crcRCReport
	}
	return crcRCOK
}

func (rep crcReport) outJSON(stdout io.Writer) {
	type pair struct {
		Surface string `json:"surface"`
		Ref     string `json:"ref"`
	}
	type pairReason struct {
		Surface string `json:"surface"`
		Ref     string `json:"ref"`
		Reason  string `json:"reason"`
	}
	obj := map[string]interface{}{
		"tracked":       rep.trackedN,
		"retired":       rep.retiredN,
		"exceptions":    rep.exceptionsN,
		"absent":        rep.absent,
		"dangling_rows": len(rep.dangling),
		"stock_rows":    len(rep.stock),
	}
	var dlist []pair
	for _, d := range rep.dangling {
		dlist = append(dlist, pair{d[0], d[2]})
	}
	var slist []pair
	for _, s := range rep.stock {
		slist = append(slist, pair{s[0], s[2]})
	}
	var elist []pairReason
	for _, e := range rep.exempt {
		elist = append(elist, pairReason{e[0], e[1], e[2]})
	}
	obj["dangling"] = dlist
	obj["stock"] = slist
	obj["exempt"] = elist
	sc := map[string]int{}
	sn := map[string]int{}
	for _, s := range rep.perSurface {
		sc[s.id] = crcCount(s.rows, "悬空") + crcCount(s.rows, "存量")
		sn[s.id] = crcCount(s.rows, "悬空")
	}
	obj["surface_counts"] = sc
	obj["surface_new"] = sn
	b, _ := json.MarshalIndent(obj, "", " ")
	fmt.Fprintln(stdout, string(b))
}
