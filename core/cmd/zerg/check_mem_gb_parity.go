// check_mem_gb_parity.go —— 判据件（Go inproc · 键 `check-mem-gb-parity`）：**两侧同名模型 `mem_gb` 对拍门**
// （缺口 `GAP-20261014-67`）。
//
// 病（缺口逐字）：同名模型在**主控** `gateway/fleet.yaml` 与**子端** `agent_models.yaml` 两侧的
// `mem_gb` 值可以不同（账内现读 3 例：34 对 86 / 18 对 34.97 / 23 对 24.716），而全表 214 步
// **没有任何门/命令比这两侧的值** —— 最近的一处（`agent models`）只做「名册有 vs 子端无」的有无
// 对账，`mem_gb` 列只印名册侧 ⇒ 两侧值不同它照报 rc=0。
//
// ★★ 本门**不把两侧逼成同值**（want 逐字的关键约束）：两侧 `mem_gb` **故意不同义** ——
//
//	主控侧 = 实测峰值 + 余量（口径标记 `usage`）；子端侧 = 闸门拍值 · 故意报大 · fail-closed
//	（口径标记 `gate-val`）；把 34 强推成 86 属**行为变更** ⇒ 本门只逼一件事：
//	**值不同时，两侧都须带显式口径标记**（标不出来才是红）。
//
// 三态判定（真源全复用 · ✗ 不新造第二真源）：
//
//	甲 值同           两侧同名模型都有 `mem_gb` ∧ 值等 ⇒ 过（不进红）。
//	乙 值异·已标口径   两侧**都**带显式口径标记 ⇒ 放行 ＋ 逐条打印理由（豁免）。
//	丙 值异·未标口径   至少一侧没标 ⇒ rc=1 并**逐字打两侧值** ＋ 点名缺标记的一侧。
//
// 真源（单点 · ✗ 不新造第二真源）：
//
//	· 主控侧 = `gateway/fleet.yaml` 的 `models:` 段（逐 `<模型>: [ { host: …, mem_gb: … } ]`）。
//	· 子端侧 = **本机子端注册表件** —— 落点**现读**自 `deploy/com.zerg.agent-<机>.plist` 的
//	  `--registry` 实参（与同包 `family_agent_ops.go` 的 `agentRegistryPath` **同一条启动声明真源**；
//	  ✗ 不按「大概在 `~/agent/agent_models.yaml`」猜路径 —— 猜出来的路径会指错件）。
//	· 本机机名 = `deploy/com.zerg.agent-*.plist` 里 `--registry` 路径**在盘上存在**的那一枚
//	  （现读推导 · ✗ 不写死机名/真值）；`--host <机>` / `--child <件>` / `--fleet <件>` 是
//	  夹具与跨机复核的**显式旋钮**（照同族「外部树依赖走显式入参」口径）。
//
// 「显式口径标记」两形态（**两侧各自判**，两侧都标才豁免）：
//
//	① 字段形：该侧该模型的条目/块里带 `mem_gb_scope: <usage|gate-val>`（值闭集；表外值**不算**标记）。
//	② 就近注记形：该侧该模型的**近旁注记**里有一行**同时含** `mem_gb` 与 `口径` 两词的注释行 ——
//	  主控侧的近旁 = `models:` **段头**注记 ∪ 该模型**块内**注记；
//	  子端侧的近旁 = 注册表**文件头**注记 ∪ 该模型**块内**注记。（口径两个字的判定是逐字词面，✗ 不猜语义。）
//
// 退码：0 全绿 ／ 1 门禁红（丙）／ 2 不给结论（真源读不到 · 本机定不了 · 可比值 0 条 · 用法错）。
//
// 旗标（家族同型 · 先现读 `check_cli_ownership.go` 对齐）：`--check`（**默认** · 只读）｜`--self-test`｜
// `--root <根>`｜`--fleet <件>`｜`--child <件>`｜`--host <机>`。`--check` 与 `--self-test` 互斥。
//
// 出厂声明 (a) 默认根定位：无 `__file__` 语境 ⇒ 不给 `--root` 时取 `repoRoot()`（`ZERG_REPO` →
// 可执行文件目录上溯 → cwd 上溯，锚 = `core/internal/version/version.go`）。`--fleet` / `--child`
// 给**相对**值时按根解析（给绝对路径则逐字采用 —— 夹具走这条）。对拍一律显式传 `--root`。
//
// 出厂声明 (b) 自指检查：本件是 `.go` ⇒ 落不进 `check-lang-freeze` 的 py/sh 扫描面；正文只出现
// **仓内相对**字面（`gateway/fleet.yaml` · `deploy/` · `com.zerg.agent-`）与 `--registry` 旗标名，
// 零绝对私有路径 ⇒ 对 `check-hardcoded-private-paths` / `check-public-marks` B3 零命中。
// 本件只读（默认档不写任何件 · 不起子进程 · 不联网）⇒ 便宜且普遍可用。
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const (
	// mgpJudgeKey —— 判据键名（与件名同型 · 家族口径）。
	mgpJudgeKey = "check-mem-gb-parity"
	// mgpProg —— 件名（报话前缀）。
	mgpProg = "check-mem-gb-parity"

	// mgpFleetRel —— 主控侧真源（仓内相对）。
	mgpFleetRel = pathGatewayFleet
	// mgpDeployDirRel —— 子端启动声明件目录（本机机名 + 注册表落点的**唯一**取数面）。
	mgpDeployDirRel = "deploy"
	// mgpAgentPlistPre / mgpAgentPlistExt —— `com.zerg.agent-<机>.plist` 的件名形状。
	mgpAgentPlistPre = "com.zerg.agent-"
	mgpAgentPlistExt = ".plist"
	// mgpRegArg —— plist 里注册表落点的旗标名（逐字 · 同 `agentRegistryPath`）。
	mgpRegArg = "--registry"

	// mgpNoteWordA / mgpNoteWordB —— 「就近注记」判定的两个**词面**（一行里同时出现才算注记）。
	mgpNoteWordA = "mem_gb"
	mgpNoteWordB = "口径"
)

// mgpScopeClosedSet —— `mem_gb_scope` 值闭集（两侧通用；表外值不算标记）。
var mgpScopeClosedSet = map[string]bool{"usage": true, "gate-val": true}

var (
	// mgpFleetKV —— 从一段（内联或行内）条目文本里取 `mem_gb:` 的值（**不吃** `mem_gb_scope:` ——
	// 它冒号前一个字符是 `e` 不是 `:` ⇒ `mem_gb:` 不命中）。
	mgpFleetKV = regexp.MustCompile(`(?:^|[\s,{])mem_gb:\s*([^,}]+)`)
	// mgpFleetScope —— 同上取 `mem_gb_scope:` 的值。
	mgpFleetScope = regexp.MustCompile(`(?:^|[\s,{])mem_gb_scope:\s*([^,}]+)`)
	// mgpFleetHost —— 同上取 `host:` 的值。
	mgpFleetHost = regexp.MustCompile(`(?:^|[\s,{])host:\s*([^,}]+)`)
	// mgpFleetModelKey —— `models:` 段里的模型键行（**恰两个空格**缩进 ⇒ ✗ 不吃 4 空格的条目行）。
	mgpFleetModelKey = regexp.MustCompile(`^  ([^ \t#][^:]*):(.*)$`)
	// mgpChildModelKey —— 子端注册表的顶层模型键行（**零**缩进 ⇒ ✗ 不吃 2 空格缩进的字段行）。
	mgpChildModelKey = regexp.MustCompile(`^([^ \t#][^:]*):(.*)$`)
)

// ── 取数结构 ──────────────────────────────────────────────────────────────────

// mgpFleetEntry —— 主控侧一条声明（模型 × 机）。
type mgpFleetEntry struct {
	Model string
	Host  string
	MemGB string // 逐字（原样）
	Scope string // `mem_gb_scope` 字段值（"" = 无）
	Mark  bool   // 该模型**块内**注记（双词注释行）
}

// mgpChildEntry —— 子端侧一条声明（注册表是**每机一份** ⇒ 无 host 字段）。
type mgpChildEntry struct {
	Model string
	MemGB string
	Scope string
	Mark  bool // 该模型**块内**注记
}

// mgpResult —— 一次判定的读数（纯函数出口 · 负控可断言）。
type mgpResult struct {
	Rc      int      // 0 绿 / 1 红 / 2 不给结论
	FleetN  int      // 主控侧本机声明条数
	ChildN  int      // 子端侧条目数
	Pairs   int      // 两侧同名 ∧ 两侧都有值（**可比值**）
	Agree   int      // 值同
	Excused []string // 值异但两侧都标口径 ⇒ 豁免（逐条理由）
	Red     []string // 值异且未两侧标口径（逐条点名 · 逐字打两侧值）
	Notes   []string // 缺席 / 解析层说明（不进退码）
}

// mgpTrim —— 逐字取值的清尾：去空白 ＋ 去一层成对引号。
func mgpTrim(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && ((s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'')) {
		s = s[1 : len(s)-1]
	}
	return strings.TrimSpace(s)
}

// mgpSameValue —— 两值是否同（先按数比，数解析不动则逐字比；✗ 不因写法差判异）。
func mgpSameValue(a, b string) bool {
	fa, ea := strconv.ParseFloat(mgpTrim(a), 64)
	fb, eb := strconv.ParseFloat(mgpTrim(b), 64)
	if ea == nil && eb == nil {
		return fa == fb
	}
	return mgpTrim(a) == mgpTrim(b)
}

// mgpNoteLine —— 一行注释算不算「就近注记」（**须是注释行** ∧ 同时含两个词面）。
func mgpNoteLine(t string) bool {
	if !strings.HasPrefix(t, "#") {
		return false
	}
	return strings.Contains(t, mgpNoteWordA) && strings.Contains(t, mgpNoteWordB)
}

// mgpScopeOf —— `mem_gb_scope` 字段值是否在闭集里 ⇒ (标记成立?, 逐字值)。
func mgpScopeOf(raw string) (bool, string) {
	v := mgpTrim(raw)
	if v == "" {
		return false, ""
	}
	return mgpScopeClosedSet[v], v
}

// mgpParseFleet —— 解析主控侧 `models:` 段。返回 (条目, 段头注记成立?, 段找得到?)。
//
// 段边界：从零缩进的 `models:` 行起，到下一个**零缩进**的非空行为止（✗ 不含它）。
// 模型块：恰两个空格缩进的模型键行起，到下一个**恰两个空格**的模型键行或段尾为止；
// 块内的注释行参与「块内注记」判定。
func mgpParseFleet(text string) ([]mgpFleetEntry, bool, bool) {
	lines := strings.Split(text, "\n")
	start := -1
	for i, ln := range lines {
		if strings.TrimRight(ln, " \t") == "models:" {
			start = i
			break
		}
	}
	if start < 0 {
		return nil, false, false
	}
	var out []mgpFleetEntry
	headMarked := false
	blockOpen := false
	blockMarked := false
	curBlockModel := ""

	for i := start + 1; i < len(lines); i++ {
		ln := lines[i]
		trimmed := strings.TrimSpace(ln)
		if trimmed == "" {
			continue
		}
		indent := len(ln) - len(strings.TrimLeft(ln, " \t"))
		// 段尾：零缩进的非空行（下一个顶层键）。
		if indent == 0 {
			break
		}
		// 注释行：段头（还没开块）⇒ 段头注记；块内 ⇒ 块内注记。
		if strings.HasPrefix(trimmed, "#") {
			if mgpNoteLine(trimmed) {
				if !blockOpen {
					headMarked = true
				} else {
					blockMarked = true
				}
			}
			continue
		}
		// 模型键行（恰 2 空格缩进）。
		if m := mgpFleetModelKey.FindStringSubmatch(ln); m != nil {
			blockOpen = true
			blockMarked = false
			model := strings.TrimSpace(m[1])
			curBlockModel = model
			rest := m[2]
			if strings.Contains(rest, "{") {
				out = append(out, mgpFleetFields(model, rest, blockMarked)...)
			}
			// 记住「当前模型」以便后续条目行归位：借最后一组条目的 Model 不行（可能 0 条）⇒ 用闭包变量。
			continue
		}
		// 条目行（4 空格缩进的 `- { … }`）：归到**当前块**（块内尚无条目 ⇒ 用块变量记的模型名）。
		if curBlockModel != "" {
			out = append(out, mgpFleetFields(curBlockModel, ln, blockMarked)...)
		}
	}
	// 块内注记可能出现在条目行**之后** ⇒ 回填（同块共享标记）。
	blockMarked = false
	curModel := ""
	for i := start + 1; i < len(lines); i++ {
		ln := lines[i]
		trimmed := strings.TrimSpace(ln)
		if trimmed == "" {
			continue
		}
		indent := len(ln) - len(strings.TrimLeft(ln, " \t"))
		if indent == 0 {
			break
		}
		if strings.HasPrefix(trimmed, "#") {
			if mgpNoteLine(trimmed) && curModel != "" {
				for k := range out {
					if out[k].Model == curModel {
						out[k].Mark = true
					}
				}
			}
			continue
		}
		if m := mgpFleetModelKey.FindStringSubmatch(ln); m != nil {
			curModel = strings.TrimSpace(m[1])
		}
	}
	return out, headMarked, true
}

// mgpFleetFields —— 从一段条目文本里抽 (host, mem_gb, mem_gb_scope)；一次可能出多条（数组形）。
func mgpFleetFields(model, span string, marked bool) []mgpFleetEntry {
	var out []mgpFleetEntry
	// 按 `- {` 切段（内联单条 `{ … }` 不作切分 ⇒ 整段当一条）。
	segs := mgpSplitEntries(span)
	for _, s := range segs {
		kv := ""
		if m := mgpFleetKV.FindStringSubmatch(s); m != nil {
			kv = mgpTrim(m[1])
		}
		sc := ""
		if m := mgpFleetScope.FindStringSubmatch(s); m != nil {
			sc = mgpTrim(m[1])
		}
		host := ""
		if m := mgpFleetHost.FindStringSubmatch(s); m != nil {
			host = mgpTrim(m[1])
		}
		out = append(out, mgpFleetEntry{Model: model, Host: host, MemGB: kv, Scope: sc, Mark: marked})
	}
	return out
}

// mgpSplitEntries —— 把一段可能含多条的文本按 `- {` 切（保留 `{`）。
func mgpSplitEntries(span string) []string {
	if strings.Count(span, "- {") <= 1 {
		if !strings.Contains(span, "{") {
			return []string{span}
		}
		return []string{span}
	}
	parts := strings.Split(span, "- {")
	var out []string
	for _, p := range parts {
		if strings.TrimSpace(p) == "" {
			continue
		}
		out = append(out, "{"+p)
	}
	if len(out) == 0 {
		out = []string{span}
	}
	return out
}

// mgpParseChild —— 解析子端注册表。返回 (模型 → 条目, 文件头注记成立?, 解析成功?)。
//
// 顶层键 = 零缩进行；字段 = 其后的缩进行（`mem_gb:` / `mem_gb_scope:`）；块内注释参「块内注记」。
func mgpParseChild(text string) (map[string]mgpChildEntry, bool, bool) {
	out := map[string]mgpChildEntry{}
	lines := strings.Split(text, "\n")
	headMarked := false
	cur := ""
	any := false
	for _, ln := range lines {
		trimmed := strings.TrimSpace(ln)
		if trimmed == "" {
			continue
		}
		indent := len(ln) - len(strings.TrimLeft(ln, " \t"))
		if indent == 0 {
			if strings.HasPrefix(trimmed, "#") {
				if mgpNoteLine(trimmed) {
					headMarked = true
				}
				continue
			}
			if m := mgpChildModelKey.FindStringSubmatch(ln); m != nil {
				cur = strings.TrimSpace(m[1])
				any = true
				e := out[cur]
				e.Model = cur
				out[cur] = e
				continue
			}
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			if mgpNoteLine(trimmed) && cur != "" {
				e := out[cur]
				e.Mark = true
				out[cur] = e
			}
			continue
		}
		if cur == "" {
			continue
		}
		e := out[cur]
		switch {
		case strings.HasPrefix(trimmed, "mem_gb_scope:"):
			e.Scope = mgpTrim(strings.TrimPrefix(trimmed, "mem_gb_scope:"))
		case strings.HasPrefix(trimmed, "mem_gb:"):
			e.MemGB = mgpTrim(strings.TrimPrefix(trimmed, "mem_gb:"))
		}
		out[cur] = e
	}
	if !any {
		return out, headMarked, false
	}
	return out, headMarked, true
}

// mgpJudge —— 三态判定（**纯函数** · 只看传入的两段文本 ＋ 本机机名 ⇒ 负控可喂合成件）。
func mgpJudge(fleetText, childText, host string) mgpResult {
	res := mgpResult{}
	fleet, fleetHead, okF := mgpParseFleet(fleetText)
	if !okF {
		res.Rc = 2
		res.Notes = append(res.Notes, "主控真源里找不到 `models:` 段 ⇒ 不给结论（rc=2）")
		return res
	}
	child, childHead, okC := mgpParseChild(childText)
	if !okC {
		res.Rc = 2
		res.Notes = append(res.Notes, "子端注册表解析不到任何模型条目 ⇒ 不给结论（rc=2）")
		return res
	}
	res.ChildN = len(child)
	host = strings.TrimSpace(host)

	seen := map[string]bool{}
	for _, fe := range fleet {
		if strings.TrimSpace(fe.Host) != host {
			continue
		}
		res.FleetN++
		ce, in := child[fe.Model]
		if !in {
			res.Notes = append(res.Notes, fmt.Sprintf("%s：子端无此模型 ⇒ 不参与对拍（缺席）", fe.Model))
			continue
		}
		if mgpTrim(fe.MemGB) == "" || mgpTrim(ce.MemGB) == "" {
			res.Notes = append(res.Notes, fmt.Sprintf("%s：一侧无 `mem_gb` ⇒ 不参与对拍（缺席）", fe.Model))
			continue
		}
		res.Pairs++
		if mgpSameValue(fe.MemGB, ce.MemGB) {
			res.Agree++
			continue
		}
		if seen[fe.Model] {
			continue
		}
		seen[fe.Model] = true
		fScopeOK, fScopeVal := mgpScopeOf(fe.Scope)
		cScopeOK, cScopeVal := mgpScopeOf(ce.Scope)
		fMarked := fScopeOK || fe.Mark || fleetHead
		cMarked := cScopeOK || ce.Mark || childHead
		if fMarked && cMarked {
			res.Excused = append(res.Excused, fmt.Sprintf(
				"豁免 %s：两侧均带显式口径标记（主控侧 %s · 子端侧 %s · 值 主控 %s ⟷ 子端 %s）",
				fe.Model, mgpMarkWhy(fScopeOK, fScopeVal, fe.Mark, fleetHead),
				mgpMarkWhy(cScopeOK, cScopeVal, ce.Mark, childHead),
				mgpTrim(fe.MemGB), mgpTrim(ce.MemGB)))
			continue
		}
		missing := "两侧"
		switch {
		case !fMarked && cMarked:
			missing = "主控侧"
		case fMarked && !cMarked:
			missing = "子端侧"
		}
		res.Red = append(res.Red, fmt.Sprintf(
			"%s 两侧 `mem_gb` 值不同且未两侧标口径（主控侧 %s ⟷ 子端侧 %s；缺标记的一侧 = %s）",
			fe.Model, mgpTrim(fe.MemGB), mgpTrim(ce.MemGB), missing))
	}
	if len(res.Red) > 0 {
		res.Rc = 1
	}
	return res
}

// mgpMarkWhy —— 豁免理由里逐字写「标记是哪一形态」（字段形带值 / 注记形）。
func mgpMarkWhy(scopeOK bool, scopeVal string, blockMark, headMark bool) string {
	var why []string
	if scopeOK {
		why = append(why, fmt.Sprintf("字段 mem_gb_scope=%s", scopeVal))
	}
	if blockMark || headMark {
		loc := "块内注记"
		if headMark && !blockMark {
			loc = "段/文件头注记"
		}
		why = append(why, loc)
	}
	if len(why) == 0 {
		return "（无）"
	}
	return strings.Join(why, " ＋ ")
}

// ── 取数面（盘） ───────────────────────────────────────────────────────────────

// mgpPlistRegistry —— 从一枚 `com.zerg.agent-<机>.plist` 里取 `--registry` 的 `<string>` 实参。
//
// ★ 收尾 `</string>` 从**值的开头之后**找（`--registry` 自己也被 `</string>` 收尾 ⇒ 从头找会
// 一律抓到旗标自己的收尾 —— 与 `family_agent_ops.go:agentRegistryPath` 同一条实测坑）。
func mgpPlistRegistry(body string) (string, bool) {
	i := strings.Index(body, mgpRegArg)
	if i < 0 {
		return "", false
	}
	tail := body[i+len(mgpRegArg):]
	open := strings.Index(tail, "<string>")
	if open < 0 {
		return "", false
	}
	after := tail[open+len("<string>"):]
	closeIdx := strings.Index(after, "</string>")
	if closeIdx < 0 {
		return "", false
	}
	v := strings.TrimSpace(after[:closeIdx])
	if v == "" {
		return "", false
	}
	return v, true
}

// mgpLocalChild —— 现读「本机子端注册表」落点：扫 `deploy/com.zerg.agent-*.plist`，逐枚取
// `--registry` 实参，**路径在盘上存在**的那一枚即本机 ⇒ (机名, 注册表件路径, rc)。
//
// rc≠0 ⇒ 不给结论（0 枚命中 / 多枚命中 / 读不到 deploy/）—— fail-closed，✗ 不猜路径。
func mgpLocalChild(root string) (string, string, int) {
	dir := filepath.Join(root, filepath.FromSlash(mgpDeployDirRel))
	ents, err := os.ReadDir(dir)
	if err != nil {
		return "", "", 2
	}
	type hit struct{ machine, path string }
	var hits []hit
	for _, e := range ents {
		name := e.Name()
		if e.IsDir() || !strings.HasPrefix(name, mgpAgentPlistPre) || !strings.HasSuffix(name, mgpAgentPlistExt) {
			continue
		}
		machine := strings.TrimSuffix(strings.TrimPrefix(name, mgpAgentPlistPre), mgpAgentPlistExt)
		if machine == "" {
			continue
		}
		b, rerr := os.ReadFile(filepath.Join(dir, name))
		if rerr != nil {
			continue
		}
		p, ok := mgpPlistRegistry(string(b))
		if !ok {
			continue
		}
		if st, serr := os.Stat(p); serr == nil && st.Mode().IsRegular() {
			hits = append(hits, hit{machine: machine, path: p})
		}
	}
	if len(hits) != 1 {
		return "", "", 2
	}
	return hits[0].machine, hits[0].path, 0
}

// mgpResolve —— 把入参旋钮解成 (fleet 件, child 件, 机名, rc)。rc≠0 ⇒ 不给结论。
func mgpResolve(root, fleetArg, childArg, hostArg string, stderr io.Writer) (string, string, string, int) {
	abs := func(p string) string {
		if filepath.IsAbs(p) {
			return p
		}
		return filepath.Join(root, filepath.FromSlash(p))
	}
	fleet := fleetArg
	if strings.TrimSpace(fleet) == "" {
		fleet = mgpFleetRel
	}
	fleetPath := abs(fleet)
	host := strings.TrimSpace(hostArg)
	childPath := strings.TrimSpace(childArg)
	if host == "" {
		m, p, rc := mgpLocalChild(root)
		if rc != 0 {
			fmt.Fprintf(stderr, "%s: 本机子端定不了（`%s/%s*%s` 里 `%s` 实参在盘上存在者恰 1 枚的条件不成立）⇒ 不给结论（rc=2）\n",
				mgpProg, mgpDeployDirRel, mgpAgentPlistPre, mgpAgentPlistExt, mgpRegArg)
			return "", "", "", 2
		}
		host = m
		if childPath == "" {
			childPath = p
		}
	}
	if childPath == "" {
		fmt.Fprintf(stderr, "%s: 没给出子端注册表件（缺 `--child` 且本机落点取不到）⇒ 不给结论（rc=2）\n", mgpProg)
		return "", "", "", 2
	}
	return fleetPath, abs(childPath), host, 0
}

// mgpCheck —— 默认 / `--check`：只读判定（两侧真源 ⇒ 三态）。
func mgpCheck(root, fleetArg, childArg, hostArg string, stdout, stderr io.Writer) int {
	fleetPath, childPath, host, rc := mgpResolve(root, fleetArg, childArg, hostArg, stderr)
	if rc != 0 {
		return 2
	}
	fb, err := os.ReadFile(fleetPath)
	if err != nil {
		fmt.Fprintf(stderr, "%s: 读不到主控真源 %s（%v）⇒ 不给结论（rc=2）\n", mgpProg, fleetPath, err)
		return 2
	}
	cb, err := os.ReadFile(childPath)
	if err != nil {
		fmt.Fprintf(stderr, "%s: 读不到子端注册表 %s（%v）⇒ 不给结论（rc=2）\n", mgpProg, childPath, err)
		return 2
	}
	res := mgpJudge(string(fb), string(cb), host)
	fmt.Fprintf(stdout, "── 两侧同名模型 mem_gb 对拍门 ── %s\n", mgpJudgeKey)
	fmt.Fprintf(stdout, "本机 %s｜主控侧声明 %d 条（%s）｜子端侧条目 %d 条（%s）\n",
		host, res.FleetN, fleetPath, res.ChildN, childPath)
	fmt.Fprintf(stdout, "可比值 %d 条｜值同 %d｜豁免 %d｜未接口径 %d\n",
		res.Pairs, res.Agree, len(res.Excused), len(res.Red))
	for _, l := range res.Notes {
		fmt.Fprintf(stdout, "  · %s\n", l)
	}
	for _, l := range res.Red {
		fmt.Fprintf(stdout, "  ✗ %s\n", l)
	}
	for _, l := range res.Excused {
		fmt.Fprintf(stdout, "  ✓ %s\n", l)
	}
	if res.Rc == 2 {
		fmt.Fprintln(stderr, strings.Join(res.Notes, "；"))
		return 2
	}
	if res.Pairs == 0 {
		fmt.Fprintf(stderr, "%s: 两侧同名且两侧都有 `mem_gb` 的模型 **0 条** ⇒ **空 ≠ 缺**，不给结论（rc=2）\n", mgpProg)
		return 2
	}
	if res.Rc == 1 {
		fmt.Fprintln(stdout, "⇒ 门禁红（rc=1）：值不同且未两侧标口径（标法见件头「显式口径标记」两形态）")
		return 1
	}
	fmt.Fprintln(stdout, "⇒ 全绿（rc=0）：同名模型值同，或值异但两侧都带显式口径标记")
	return 0
}

// mgpSelfTest —— 合成自检（纯函数夹具 · 仓外临时树只占「缺件」两格）：全过 ⇒ rc=0。
//
// ★ 成对（✗ 禁恒绿）：同一条「值异」候选，**两侧都标 ⇒ 绿** 对 **只标一侧 ⇒ 红** 对 **两侧都不标 ⇒ 红**
// —— 若判据体不再真读「口径标记」，该组必失败（`report` 打格名）。
func mgpSelfTest(stdout, stderr io.Writer) int {
	okN, badN := 0, 0
	report := func(name string, cond bool, detail string) {
		if cond {
			fmt.Fprintf(stdout, "   ✓ %s\n", name)
			okN++
			return
		}
		fmt.Fprintf(stdout, "   ✗ %s：%s\n", name, detail)
		badN++
	}

	const (
		fleetBare = "models:\n  Alpha:\n    - { host: boxy, backend: llama-server, mem_gb: 34 }\n  Beta:\n    - { host: boxy, backend: llama-server, mem_gb: 11 }\n"
		fleetNote = "models:\n  # mem_gb 字段口径规约：本段 mem_gb = 实测峰值 + 余量（占用口径）\n  Alpha:\n    - { host: boxy, backend: llama-server, mem_gb: 34 }\n"
		fleetScop = "models:\n  Alpha:\n    - { host: boxy, backend: llama-server, mem_gb: 34, mem_gb_scope: usage }\n"
		childBare = "Alpha:\n  file: /tmp/a.gguf\n  mem_gb: 86\nBeta:\n  file: /tmp/b.gguf\n  mem_gb: 11\n"
		childNote = "# 本机子端注册表：mem_gb = 闸门拍值（口径 gate-val · 故意报大）\nAlpha:\n  file: /tmp/a.gguf\n  mem_gb: 86\n"
		childScop = "Alpha:\n  file: /tmp/a.gguf\n  mem_gb: 86\n  mem_gb_scope: gate-val\n"
		childSame = "Alpha:\n  file: /tmp/a.gguf\n  mem_gb: 34\n"
	)

	// ① 值异 · 两侧都不标 ⇒ 必红 ＋ 逐字打两侧值 ＋ 点名缺标记的一侧 = 两侧
	r1 := mgpJudge(fleetBare, childBare, "boxy")
	want1 := "Alpha 两侧 `mem_gb` 值不同且未两侧标口径（主控侧 34 ⟷ 子端侧 86；缺标记的一侧 = 两侧）"
	report("甲 值异·两侧都不标 ⇒ rc=1 ＋ 逐字两侧值 ＋ 点名两侧",
		r1.Rc == 1 && ccoHas(r1.Red, want1), fmt.Sprintf("期望 rc=1 且点名 %q，得 rc=%d red=%v", want1, r1.Rc, r1.Red))

	// ② 值异 · 两侧都标（字段形）⇒ 必绿 ＋ 逐条理由
	r2 := mgpJudge(fleetScop, childScop, "boxy")
	report("乙 值异·两侧都标（字段形 usage/gate-val）⇒ rc=0 ＋ 理由",
		r2.Rc == 0 && len(r2.Excused) == 1 &&
			strings.Contains(r2.Excused[0], "字段 mem_gb_scope=usage") &&
			strings.Contains(r2.Excused[0], "字段 mem_gb_scope=gate-val"),
		fmt.Sprintf("期望 rc=0 且理由带两侧字段值，得 rc=%d excused=%v", r2.Rc, r2.Excused))

	// ③ 值异 · 两侧都标（就近注记形）⇒ 必绿
	r3 := mgpJudge(fleetNote, childNote, "boxy")
	report("丙 值异·两侧都标（段头/文件头就近注记）⇒ rc=0", r3.Rc == 0 && len(r3.Excused) == 1,
		fmt.Sprintf("期望 rc=0，得 rc=%d red=%v", r3.Rc, r3.Red))

	// ④ 值异 · **只标一侧**（主控标、子端不标）⇒ 必红 ＋ 点名欠标的那一侧 = 子端侧
	r4 := mgpJudge(fleetScop, childBare, "boxy")
	report("丁 值异·只一侧标 ⇒ rc=1 ＋ 点名欠标侧（✗ 一侧标就放行）",
		r4.Rc == 1 && len(r4.Red) == 1 && strings.Contains(r4.Red[0], "缺标记的一侧 = 子端侧"),
		fmt.Sprintf("期望 rc=1 且点名子端侧，得 rc=%d red=%v", r4.Rc, r4.Red))

	// ⑤ 成对断言（✗ 禁恒绿）：都标(绿) ≠ 只标一侧(红) ≠ 都不标(红)
	report("戊 成对：都标(0) ≠ 只标一侧(1) ≠ 都不标(1)",
		r2.Rc == 0 && r4.Rc == 1 && r1.Rc == 1,
		fmt.Sprintf("两格同码 rc=%d/%d/%d ⇒ 无鉴别力", r2.Rc, r4.Rc, r1.Rc))

	// ⑥ 值同 ⇒ 必绿（不进豁免/不进红）
	r6 := mgpJudge(fleetBare, childSame, "boxy")
	report("己 值同 ⇒ rc=0 且无可比值以外噪声", r6.Rc == 0 && r6.Agree == 1 && len(r6.Excused) == 0 && len(r6.Red) == 0,
		fmt.Sprintf("期望 rc=0 agree=1，得 rc=%d agree=%d", r6.Rc, r6.Agree))

	// ⑦ 表外 scope 值**不算**标记 ⇒ 值异仍红（闭集口径的牙）
	r7 := mgpJudge(
		"models:\n  Alpha:\n    - { host: boxy, backend: llama-server, mem_gb: 34, mem_gb_scope: whatever }\n",
		childScop, "boxy")
	report("庚 表外 mem_gb_scope 值不算标记 ⇒ rc=1",
		r7.Rc == 1 && len(r7.Red) == 1, fmt.Sprintf("期望 rc=1，得 rc=%d red=%v", r7.Rc, r7.Red))

	// ⑧ 机名不匹配 ⇒ 主控侧 0 条声明（可比值 0 ⇒ 调用方 rc=2；这里判读数）
	r8 := mgpJudge(fleetBare, childBare, "otherbox")
	report("辛 机名不匹配 ⇒ 主控侧声明 0 条（可比值 0）", r8.FleetN == 0 && r8.Pairs == 0,
		fmt.Sprintf("期望 0/0，得 %d/%d", r8.FleetN, r8.Pairs))

	// ⑨ 缺段 / 空注册表 ⇒ rc=2（不给结论）
	r9 := mgpJudge("auth:\n  token: \"\"\n", childBare, "boxy")
	r10 := mgpJudge(fleetBare, "# 只有注释\n", "boxy")
	report("壬 主控侧无 models: 段 / 子端空注册表 ⇒ rc=2（不给结论）",
		r9.Rc == 2 && r10.Rc == 2, fmt.Sprintf("期望 2/2，得 %d/%d", r9.Rc, r10.Rc))

	// ⑩ 现读真源（真仓两件）解析得动（防呆 · 证取数面没写漂）
	tmp, err := os.MkdirTemp("", "mgp-selftest-")
	if err != nil {
		tmp = os.TempDir()
	} else {
		defer os.RemoveAll(tmp)
	}
	missFleet := filepath.Join(tmp, "nope-fleet.yaml")
	missChild := filepath.Join(tmp, "nope-child.yaml")
	var o10, e10 strings.Builder
	rc10 := mgpCheck(tmp, missFleet, missChild, "boxy", &o10, &e10)
	report("癸 缺两侧真源 ⇒ 不给结论（rc=2）",
		rc10 == 2 && strings.Contains(e10.String(), "读不到主控真源"),
		fmt.Sprintf("期望 rc=2 且报「读不到主控真源」，得 rc=%d err=%q", rc10, e10.String()))

	fmt.Fprintf(stdout, "   自检：%d 格过 · %d 格不过（纯函数夹具 ＋ 仓外临时树 · 未碰真仓）\n", okN, badN)
	if badN == 0 {
		return 0
	}
	return 2
}

// checkMemGBParityGateMain —— 入口（argv 与家族同形 · 不含程序名）。
func checkMemGBParityGateMain(stdout, stderr io.Writer, argv []string) int {
	rootArg, fleetArg, childArg, hostArg := "", "", "", ""
	rootSet := false
	doCheck, selfTest := false, false

	for i := 0; i < len(argv); i++ {
		a := argv[i]
		val := func(name string) (string, bool) {
			if a == name {
				if i+1 >= len(argv) {
					return "", false
				}
				i++
				return argv[i], true
			}
			return strings.TrimPrefix(a, name+"="), true
		}
		switch {
		case a == "--root" || strings.HasPrefix(a, "--root="):
			v, okv := val("--root")
			if !okv {
				fmt.Fprintf(stderr, "%s: `--root` 缺值 ⇒ 不给结论（rc=2）\n", mgpProg)
				return 2
			}
			rootArg, rootSet = v, true
		case a == "--fleet" || strings.HasPrefix(a, "--fleet="):
			v, okv := val("--fleet")
			if !okv {
				fmt.Fprintf(stderr, "%s: `--fleet` 缺值 ⇒ 不给结论（rc=2）\n", mgpProg)
				return 2
			}
			fleetArg = v
		case a == "--child" || strings.HasPrefix(a, "--child="):
			v, okv := val("--child")
			if !okv {
				fmt.Fprintf(stderr, "%s: `--child` 缺值 ⇒ 不给结论（rc=2）\n", mgpProg)
				return 2
			}
			childArg = v
		case a == "--host" || strings.HasPrefix(a, "--host="):
			v, okv := val("--host")
			if !okv {
				fmt.Fprintf(stderr, "%s: `--host` 缺值 ⇒ 不给结论（rc=2）\n", mgpProg)
				return 2
			}
			hostArg = v
		case a == "--check":
			doCheck = true
		case a == "--self-test":
			selfTest = true
		case a == "-h" || a == "--help":
			fmt.Fprintf(stdout, "用法：%s [--check | --self-test] [--root <根>] [--fleet <件>] [--child <件>] [--host <机>]\n", mgpProg)
			return 0
		default:
			fmt.Fprintf(stderr, "%s: 未知旗标 %s ⇒ 不给结论（rc=2）\n", mgpProg, a)
			return 2
		}
	}
	if doCheck && selfTest {
		fmt.Fprintf(stderr, "%s: `--check` / `--self-test` 互斥 ⇒ 不给结论（rc=2）\n", mgpProg)
		return 2
	}
	if selfTest {
		return mgpSelfTest(stdout, stderr)
	}
	root := rootArg
	if !rootSet {
		root = repoRoot()
	}
	if strings.TrimSpace(root) == "" {
		fmt.Fprintf(stderr, "%s: 解析不到仓根（`--root` 未给且 `repoRoot()` 取不到）⇒ 不给结论（rc=2）\n", mgpProg)
		return 2
	}
	if abs, err := filepath.Abs(root); err == nil {
		root = abs
	}
	return mgpCheck(root, fleetArg, childArg, hostArg, stdout, stderr)
}
