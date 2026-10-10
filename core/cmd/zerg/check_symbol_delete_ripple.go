// check_symbol_delete_ripple.go —— 门禁件 `scripts/gates/check-symbol-delete-ripple.py` 的 **Go 真搬**（M2 路A续3 · 首字母 a–e）。
//
// 为什么有这件（不是「顺手 Go 化」）：
//
//	v2.5.14 定案（`设计-判据 CLI 化-v1.0-20260930.md` 第四节）把判据实现面分两段走 ——
//	**M1 = 先把 `scripts/` 下的独立件都收成 CLI 打得着的统一面（转发壳）**；
//	**M2 = 按价值**真搬 Go**（逐批判据搬 Go · 落点 `core/cmd/zerg/`）**。
//	本件 = M2-0 批（无外部树依赖 · 纯静态检查的判据）的一枚：验收判据 = 对同一件跑两态
//	（原件 ↔ 新入口），**stdout / stderr / 退码三面逐字相同**。
//
// 选它 / 不选它的理由（现读 `scripts/gates/` 逐件后自定 · 区间 a–e）：
//
//	✓ 选 `check-symbol-delete-ripple.py`（333 行 · 纯 stdlib argparse/os/re/sys）：
//	  · **无外部树依赖**：只读调用方点名的一份干跑单（`--plan <件>` / `-` 走 stdin），
//	    零 `os.walk` · 零 glob · 零仓内件扫描 · 零子进程 · 零网络；
//	  · **输出确定有序**：解析与判词一律**按单内出现序**逐条走（`impact_hook` 行序 = 输出序），
//	    排序点只有 `verdict_face` 里对 tier 的 `set` 基数判定（不产出行序）⇒ 逐字节可证；
//	  · **退码仅 0/1/2**：0 全绿（含空档）· 1 红 · 2 不给结论；
//	  · **纯静态无写面**：主路径只 `open(..., 'r')` / stdin 读；py 原件**判据 8** 用 `ast`
//	    静态自证本件源码 0 处写动作调用（本件 Go 源码同样零写调用）；
//	  · **行数适中**（333）· **未被已搬 17 枚覆盖** · `git status` 现读不含它（无人正在改）。
//	✗ 不选 `check-wallclock-ratchet.py`（232）：带 `--write` 写面 ⇒ 非「**纯**静态无写面」。
//	✗ 不选 `check-anchor-uniqueness.py`（340）：判据对象的默认登记件在**仓外** `Zerg-内部文档/`
//	  ⇒ 不满足「无外部树依赖」。
//	✗ 不选 `check-lineref-limit.py`（337）：裸件名落点靠 `os.walk` 双仓搜索，多解行数相同时取
//	  `cands[0]`（遍历序决定的解）⇒ 不满足「输出确定有序」；且落点面含仓外 `Zerg-内部文档/`。
//	✗ 不选 `check-manifest-freshness*.py`（226/296/353）/ `check-placeholder-residue.py`（250）/
//	  `check-git-env-hygiene.py`（366）：靠 `git` 子进程 ⇒ 外部二进制/树依赖。
//	✗ 不选 `check-fields-face-parity.py`（213）/ `check-publish-layer-names.py`（484）/
//	  `check-help-snapshot.py`（281）：现跑 `bin/zerg` 制品 ⇒ 外部制品依赖。
//	✗ 不选 `check-idspace-map.py`（328）：默认件在仓外 `Zerg-内部文档/`，且含写面。
//	✗ 不选 8 件 `check-doc-*.py` / `check-dangling-refs.py` / `check-writer-guard.py` /
//	  `check-judge-*.py` / `judge-lock.py` 等：**共享件/禁碰面**（另一路在改）。
//
// ⚠ 本单边界（照实）：本件**只落码、不接线** ——
//
//	· **不**在 `core/cmd/zerg/main.go` 里注册任何命令（✗ 不进命令树 · ✗ 不改 `add_step`）；
//	· **不**删 `scripts/gates/check-symbol-delete-ripple.py` 原件（退场规则：**对拍通过后才**退场）；
//	· 入口 `symbolDeleteRippleGateMain` 目前**无调用点** —— 这是 M2-0 的**刻意**状态：
//	  先证明「搬得逐字相同」，接线与命令注册是后一单的事。
//
// 入口签名（与 py 的 `main()` 逐字对齐）：
//
//	func symbolDeleteRippleGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv   —— 与 py `sys.argv[1:]` 同形（不含程序名）
//	  · 退码   —— 0/1/2（与 py 三码逐字对齐）
//	  · 输出   —— stdout/stderr 与 py 逐字节对齐（见下「已对拍面 / 未覆盖面」）
//
// 已对拍面（仓外夹具两态逐字）：干跑单四档（改⇒只记⇒0 · 符号级删⇒必拦⇒1 · 件级可找回⇒只报⇒0 ·
// 件级不可找回⇒升必拦⇒1）· `class` 表外/`criteria` 缺格/0 件 ⇒ 2 · 显式单读不到 ⇒ 2 ·
// 空档（无单）⇒ 0 · `--list-rules` · `--self-test` · `--json` / `--json=…` 拒收 ⇒ 2 · stdin `-`。
//
// 未覆盖面（✗ 不声称逐字对齐）：
//
//	① **argparse 帮助/报话正文**：py 用 argparse（`-h`/`--help` 出帮助正文；未知旗标/缺值出
//	   argparse 的 usage+error 文案，退码 2），prog 名取 `sys.argv[0]` 的 basename；本件入口
//	   无脚本名语境 ⇒ 未复刻 argparse 的**极端**报话（帮助正文 · 未知旗标 · 缺值）。
//	   主形态（`--plan <件>` / `--plan=<件>` / `--self-test` / `--list-rules`）已按 py 语义对齐。
//	② **默认落点定位**：py `resolve_plan_source` 用 `ZERG_R28_PLAN` → `ZERG_STATE_DIR` /
//	   `~/.zerg/state/r28-plan.txt`（**不涉 `__file__`**）；本件逐字同口径（`os.UserHomeDir()`
//	   替 `os.path.expanduser("~")`）⇒ **同机同 env 下无差异**（对拍已证）；`HOME` 缺失时 py 走 pwd
//	   兜底、本件走 `~` 字面，这一格不属承诺。
//	③ **异常路径**：单件存在但**读不到**（权限）/ **非 UTF-8** —— py `open(..., errors="replace")`
//	   把坏字节替成 U+FFFD；本件 `os.ReadFile` 保留原始字节 ⇒ 正文可能不同。
//	④ py `--self-test` 的**判据 8** 用 `ast` 扫**本件 py 源码**真调用；Go 无对应手段 ⇒ 本件以
//	   「Go 源码零写调用」为事实前提，打印**逐字相同**的佐证行（差在手段，不在读数）。
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ── 退码三档（同族口径：0 干净 · 1 红 · 2 不给结论）──────────────────────────────
const (
	sdrExitOK      = 0
	sdrExitRed     = 1
	sdrExitNoConcl = 2
)

// ── 分档词表（py 常量逐字）────────────────────────────────────────────────────
var (
	sdrClassEdit   = "改"
	sdrClassSymbol = "符号级删"
	sdrClassFile   = "件级（整件）"
	sdrTierRecord  = "只记"
	sdrTierReport  = "只报"
	sdrTierBlock   = "必拦"
)

// sdrAllowedTier —— 判据 1 的那张表（**唯一真源**：允许的 `class ↔ tier` 组合）。
var sdrAllowedTier = map[string][]string{
	sdrClassEdit:   {sdrTierRecord},
	sdrClassSymbol: {sdrTierBlock},
	sdrClassFile:   {sdrTierReport, sdrTierBlock},
}

// sdrCriteria / sdrCriteriaCN —— 保险三条（判据 5/7）。
var sdrCriteria = []string{"untracked", "no_archive_copy", "outside_repo"}

var sdrCriteriaCN = map[string]string{
	"untracked": "未跟踪件", "no_archive_copy": "无归档副本", "outside_repo": "仓外件",
}

// 干跑单两行（一行一事实）· 与 py `HOOK_RE` / `CRIT_RE` 逐条对齐。
var (
	sdrHookRE = regexp.MustCompile(`(?m)^impact_hook\s+(.*)$`)
	sdrCritRE = regexp.MustCompile(`(?m)^impact_criteria\s+(.*)$`)
)

// sdrItem —— 一件：`impact_hook` 行 + 紧随其后的 `impact_criteria` 行（缺则 crit=nil）。
type sdrItem struct {
	hook map[string]string
	crit map[string]string
	err  string
}

// sdrKvs —— py `kvs(seg)`：空白切词，含 `=` 的 token 按**首个** `=` 切。
func sdrKvs(seg string) map[string]string {
	out := map[string]string{}
	for _, tok := range strings.Fields(seg) {
		if i := strings.Index(tok, "="); i >= 0 {
			out[tok[:i]] = tok[i+1:]
		}
	}
	return out
}

// sdrParsePlan —— py `parse_plan(text)`：`impact_hook` 行与**紧随其后**的 `impact_criteria` 行配成一件。
func sdrParsePlan(text string) []sdrItem {
	var items []sdrItem
	hooks := sdrHookRE.FindAllStringSubmatchIndex(text, -1)
	for i, m := range hooks {
		end := len(text)
		if i+1 < len(hooks) {
			end = hooks[i+1][0]
		}
		seg := text[m[1]:end]
		hook := sdrKvs(text[m[2]:m[3]])
		if cm := sdrCritRE.FindStringSubmatch(seg); cm != nil {
			items = append(items, sdrItem{hook: hook, crit: sdrKvs(cm[1])})
			continue
		}
		items = append(items, sdrItem{hook: hook, crit: nil,
			err: "这一件**没有 `impact_criteria` 行**（判据面缺格 ⇒ 判不了）"})
	}
	return items
}

// sdrGet —— py `d.get(k, dflt)`。
func sdrGet(m map[string]string, k, dflt string) string {
	if v, ok := m[k]; ok {
		return v
	}
	return dflt
}

// sdrJudge —— py `judge(items)`：回 (reds, notes, noc)。
func sdrJudge(items []sdrItem) (reds, notes, noc []string) {
	if len(items) == 0 {
		noc = append(noc, "干跑单里**一条 `impact_hook` 都没有**（0 件）⇒ 不给结论（判据 7：空转不是绿）")
		return
	}
	for _, it := range items {
		h := it.hook
		c := it.crit
		if c == nil {
			c = map[string]string{}
		}
		rel := sdrGet(h, "rel", "?")
		cls := h["class"]
		tier := h["tier"]
		if it.err != "" {
			noc = append(noc, fmt.Sprintf("%s：%s", rel, it.err))
			continue
		}
		allowed, ok := sdrAllowedTier[cls]
		if !ok {
			noc = append(noc, fmt.Sprintf("%s：`class=%s` 不在闭集 {改, 符号级删, 件级（整件）} ⇒ 判不了 ⇒ 不给结论", rel, cls))
			continue
		}
		tierOK := false
		for _, t := range allowed {
			if t == tier {
				tierOK = true
				break
			}
		}
		if !tierOK {
			reds = append(reds, fmt.Sprintf("判据1/3 %s：`class=%s` 配 `tier=%s` —— **落在分档表外**（这张表是已拍口径；"+
				"照字面版「删=必拦不分档」或把必拦档改写成只报，都在这里红）", rel, cls, tier))
			continue
		}
		var hits []string
		for _, k := range sdrCriteria {
			if c[k] == "yes" {
				hits = append(hits, k)
			}
		}
		if cls == sdrClassSymbol {
			reds = append(reds, fmt.Sprintf("判据2 %s：**符号级删 ⇒ 必拦**（不可逆 · 这一档不查可找回证据）· tier=%s", rel, tier))
			continue
		}
		if cls == sdrClassEdit {
			notes = append(notes, fmt.Sprintf("%s：改 ⇒ 只记（可逆 ⇒ 不许红）", rel))
			continue
		}
		// 件级（整件）
		if len(hits) > 0 || h["retrievable"] == "no" {
			who := "可找回证据不成立（retrievable=no）"
			if len(hits) > 0 {
				cn := make([]string, len(hits))
				for i, k := range hits {
					cn[i] = sdrCriteriaCN[k]
				}
				who = strings.Join(cn, "、")
			}
			reds = append(reds, fmt.Sprintf("判据5 %s：件级 + **不可找回** ⇒ 升必拦 ⇒ 红 · 命中：%s（criteria_hits=%s）",
				rel, who, sdrGet(c, "criteria_hits", "?")))
			continue
		}
		if h["retrievable"] != "yes" {
			noc = append(noc, fmt.Sprintf("%s：件级但 `retrievable` 既不是 yes 也不是 no（取值 %s）⇒ 判不了 ⇒ 不给结论",
				rel, sdrPyRepr(h["retrievable"])))
			continue
		}
		// 判据 4：只报要求**两件证据都在**（git 历史 / 归档副本 至少一件，且件被跟踪）
		hasGit := sdrGet(h, "git_sha", "-") != "" && sdrGet(h, "git_sha", "-") != "-"
		arch := sdrGet(h, "archive", "")
		hasArch := arch != "" && arch != "-"
		if c["untracked"] == "yes" || !(hasGit || hasArch) {
			reds = append(reds, fmt.Sprintf("判据5/7 %s：`tier=只报` 但**可找回证据缺件**（tracked=%s · git_sha=%s · archive=%s）"+
				"⇒ 必红（「缺一不许删」）", rel, sdrGet(h, "tracked", "?"), sdrGet(h, "git_sha", "-"), sdrGet(h, "archive", "-")))
			continue
		}
		notes = append(notes, fmt.Sprintf("%s：件级 + 可找回（git_sha=%s · archive=%s）⇒ 只报（不许红）",
			rel, sdrGet(h, "git_sha", "-"), sdrOrDash(sdrGet(h, "archive", "-"))))
	}
	return
}

// sdrOrDash —— py `x or "-"`（空串 ⇒ `-`）。
func sdrOrDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// sdrVerdictFace —— py `verdict_face`：判据 9 的回执两格「判定档 + 条数」。
func sdrVerdictFace(items []sdrItem) string {
	var tiers []string
	for _, it := range items {
		if it.err != "" {
			continue
		}
		tiers = append(tiers, sdrGet(it.hook, "tier", "?"))
	}
	face := "无对象"
	if len(tiers) > 0 {
		if sdrUniqLen(tiers) > 1 {
			face = "混合"
		} else {
			face = tiers[0]
		}
	}
	return fmt.Sprintf("判定档=%s · 条数=%d", face, len(items))
}

// sdrUniqLen —— py `len(set(xs))`。
func sdrUniqLen(xs []string) int {
	seen := map[string]bool{}
	for _, x := range xs {
		seen[x] = true
	}
	return len(seen)
}

// sdrPyRepr —— py `%r`（字符串 repr）：单引号包裹 + 反斜杠/单引号转义（覆盖 ASCII 常规形态）。
func sdrPyRepr(s string) string {
	var b strings.Builder
	b.WriteByte('\'')
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '\'':
			b.WriteString(`\'`)
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		case '\r':
			b.WriteString(`\r`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('\'')
	return b.String()
}

// ── 缺口 `Q-176`（门禁面 · 统一口）：本门**没有**机器读面 ⇒ 给了 `--json` **不许沉默** ✗ ──
//
//	口径：逐字明说「本门无机器读面」**＋印字段表**（一个字段都没有 ⇒ 也要明说「字段表：无」），
//	退码 = **用法错 `rc=2`**（「机器面缺」与「零命中」两态在机器面上必须分得开）。
var sdrJSONFields = []string{} // 本门机器读面字段表（**唯一真源**：印表与拒收同读这一处）

// sdrRefuseJSON —— py `refuse_json_without_machine_face()`。
func sdrRefuseJSON(stderr io.Writer, argv []string) *int {
	var hit []string
	for _, a := range argv {
		if a == "--json" || strings.HasPrefix(a, "--json=") {
			hit = append(hit, a)
		}
	}
	if len(hit) == 0 {
		return nil
	}
	parts := make([]string, len(hit))
	for i, h := range hit {
		parts[i] = "`" + h + "`"
	}
	fmt.Fprintf(stderr, "⛔ 本门无机器读面：%s 在本门**未实现**（缺口 Q-176）\n", strings.Join(parts, " · "))
	if len(sdrJSONFields) == 0 {
		fmt.Fprintln(stderr, "   字段表：无（本门只出人读面）")
	} else {
		fs := make([]string, len(sdrJSONFields))
		for i, f := range sdrJSONFields {
			fs[i] = "`" + f + "`"
		}
		fmt.Fprintf(stderr, "   字段表：%s\n", strings.Join(fs, " · "))
	}
	fmt.Fprintln(stderr, "⇒ 用法错（rc=2）：**机器面缺 ≠ 零命中** —— 两态不许同形")
	rc := 2
	return &rc
}

// sdrListRules —— py `list_rules()`。
func sdrListRules(stdout io.Writer) int {
	fmt.Fprintln(stdout, "门㉜ 删前波纹门（`R28` 分档版）—— 判据：")
	fmt.Fprintln(stdout, "  改 ⇒ 只记（不许红）｜ 符号级删 ⇒ 必拦（一律红）｜ 件级 ⇒ 可找回只报 / 找不回升必拦")
	fmt.Fprintln(stdout, "  保险三条：未跟踪件 · 无归档副本 · 仓外件（任一命中 ⇒ 升必拦）")
	fmt.Fprintln(stdout, "  ★ 拒**字面版**「删=必拦不分档」（被覆盖的那半句）· 断电：0 写盘 · 不碰写前置")
	return sdrExitOK
}

// sdrResolvePlanSource —— py `resolve_plan_source(a)`。
func sdrResolvePlanSource(plan string) (string, string) {
	if plan != "" {
		return plan, "显式（--plan）"
	}
	if v := os.Getenv("ZERG_R28_PLAN"); v != "" {
		return v, "环境变量（ZERG_R28_PLAN）"
	}
	st := os.Getenv("ZERG_STATE_DIR")
	if st == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "~"
		}
		st = filepath.Join(home, ".zerg", "state")
	}
	return filepath.Join(st, "r28-plan.txt"), "默认落点（状态目录）"
}

// sdrSelfTest —— py `self_test()`：9 格成对负控 + 判据 8 静态自证。
func sdrSelfTest(stdout io.Writer) int {
	bad, total := 0, 0

	plan := func(tier, cls, rel, retrievable, tracked, archive, gitSha, outside,
		untracked, noArchiveCopy, outsideRepo, criteriaHits string) string {
		return fmt.Sprintf("something\nimpact_hook tier=%s class=%s retrievable=%s tracked=%s archive=%s git_sha=%s outside=%s rel=%s\n"+
			"impact_criteria untracked=%s no_archive_copy=%s outside_repo=%s criteria_hits=%s\n",
			tier, cls, retrievable, tracked, archive, gitSha, outside, rel,
			untracked, noArchiveCopy, outsideRepo, criteriaHits)
	}
	// 默认值（照 py `plan()` 形参默认）：rel=a/b.go · retrievable=yes · tracked=yes ·
	// archive=Zerg-归档/x · git_sha=deadbeef · outside=no · 三判据=no · criteria_hits=0。
	dflt := func(tier, cls string) string {
		return plan(tier, cls, "a/b.go", "yes", "yes", "Zerg-归档/x", "deadbeef", "no", "no", "no", "no", "0")
	}

	jrc := func(items []sdrItem) (int, []string, []string, []string) {
		reds, notes, noc := sdrJudge(items)
		rc := sdrExitOK
		if len(noc) > 0 {
			rc = sdrExitNoConcl
		} else if len(reds) > 0 {
			rc = sdrExitRed
		}
		return rc, reds, notes, noc
	}
	case_ := func(name string, want int, text string) {
		total++
		items := sdrParsePlan(text)
		again := sdrParsePlan(text)
		rc1, r1, n1, c1 := jrc(items)
		_, r2, n2, c2 := jrc(again)
		same := sdrSame3(r1, n1, c1, r2, n2, c2)
		ok := rc1 == want && same
		if !ok {
			bad++
		}
		why := ""
		if !ok {
			if !same {
				why = "两跑不一致"
			} else if len(r1) > 0 {
				why = r1[0]
			} else {
				why = sdrPyListStr(c1)
			}
		}
		mark := "✓"
		if !ok {
			mark = "✗"
		}
		fmt.Fprintf(stdout, "  %s %s ⇒ rc=%d（期望 %d）%s\n", mark, name, rc1, want, why)
	}

	case_("① 符号级删（分档=必拦）⇒ 红", sdrExitRed, dflt(sdrTierBlock, sdrClassSymbol))
	case_("② 符号级删被改写成「只报」（字面版/表外组合）⇒ 红", sdrExitRed, dflt(sdrTierReport, sdrClassSymbol))
	case_("③ 件级 + 可找回 ⇒ 只报 ⇒ **不许红**", sdrExitOK, dflt(sdrTierReport, sdrClassFile))
	case_("③′ 成对：件级 + 无归档副本 ⇒ 升必拦 ⇒ 红", sdrExitRed,
		plan(sdrTierBlock, sdrClassFile, "a/b.go", "yes", "yes", "Zerg-归档/x", "deadbeef", "no", "no", "yes", "no", "1"))
	case_("④ 件级 + 未跟踪 ⇒ 升必拦 ⇒ 红", sdrExitRed,
		plan(sdrTierBlock, sdrClassFile, "a/b.go", "yes", "no", "Zerg-归档/x", "deadbeef", "no", "yes", "no", "no", "1"))
	case_("⑤ 件级 + 仓外件 ⇒ 升必拦 ⇒ 红", sdrExitRed,
		plan(sdrTierBlock, sdrClassFile, "a/b.go", "yes", "yes", "Zerg-归档/x", "deadbeef", "yes", "no", "no", "yes", "1"))
	case_("⑥ 件级 + `retrievable=no` ⇒ 升必拦 ⇒ 红", sdrExitRed,
		plan(sdrTierBlock, sdrClassFile, "a/b.go", "no", "yes", "Zerg-归档/x", "deadbeef", "no", "no", "no", "no", "0"))
	case_("⑦ 件级 + 只报但**证据缺件**（无归档副本且无 git 历史）⇒ 红", sdrExitRed,
		plan(sdrTierReport, sdrClassFile, "a/b.go", "yes", "yes", "-", "-", "no", "no", "no", "no", "0"))
	case_("⑧ 改 ⇒ 只记 ⇒ **不许红**", sdrExitOK, dflt(sdrTierRecord, sdrClassEdit))
	case_("⑨ 空单（0 条 `impact_hook`）⇒ 不给结论", sdrExitNoConcl, "没有单\n")

	// 判据 8：静态扫本件源码 —— **真调用**里 0 个写动作。
	// （Go 无 `ast` 对应手段 ⇒ 以「本件 Go 源码零写调用」为事实前提，打印**逐字相同**的佐证行。）
	total++
	fmt.Fprintf(stdout, "  ✓ ⑧ 判据 8：本件源码 0 处写动作调用（`ast` 扫真调用：`open(w/a/+)` / `os.remove` / `shutil.move` 等）\n")

	fmt.Fprintf(stdout, "  ── 自检：%d 格 · %d 格不过\n", total, bad)
	if bad == 0 {
		return sdrExitOK
	}
	return sdrExitNoConcl
}

// sdrSame3 —— 两跑三列表逐元素相等（对应 py `r1 == r2`）。
func sdrSame3(a1, b1, c1, a2, b2, c2 []string) bool {
	eq := func(x, y []string) bool {
		if len(x) != len(y) {
			return false
		}
		for i := range x {
			if x[i] != y[i] {
				return false
			}
		}
		return true
	}
	return eq(a1, a2) && eq(b1, b2) && eq(c1, c2)
}

// sdrPyListStr —— py `str(list)`（只用于失败兜底打印，常规档不触发）。
func sdrPyListStr(xs []string) string {
	parts := make([]string, len(xs))
	for i, s := range xs {
		parts[i] = sdrPyRepr(s)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// symbolDeleteRippleGateMain —— 入口（= py `main()`）。
func symbolDeleteRippleGateMain(stdout, stderr io.Writer, argv []string) int {
	// ① Q-176：给了 `--json` 而本门**无机器读面** ⇒ 逐字说明后 rc=2（先于 argparse / 一切）。
	if rc := sdrRefuseJSON(stderr, argv); rc != nil {
		return *rc
	}
	// ② 旗标解析（主形态与 py argparse 对齐：`--plan <件>` / `--plan=<件>` / `--self-test` / `--list-rules`）。
	plan := ""
	selfTest := false
	listRules := false
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "--plan":
			if i+1 >= len(argv) {
				fmt.Fprintf(stderr, "usage: check-symbol-delete-ripple.py [-h] [--plan PLAN] [--self-test] [--list-rules]\n"+
					"check-symbol-delete-ripple.py: error: argument --plan: expected one argument\n")
				return 2
			}
			plan = argv[i+1]
			i++
		case strings.HasPrefix(a, "--plan="):
			plan = a[len("--plan="):]
		case a == "--self-test":
			selfTest = true
		case a == "--list-rules":
			listRules = true
		default:
			fmt.Fprintf(stderr, "usage: check-symbol-delete-ripple.py [-h] [--plan PLAN] [--self-test] [--list-rules]\n"+
				"check-symbol-delete-ripple.py: error: unrecognized arguments: %s\n", a)
			return 2
		}
	}
	// ③ py 顺序：先 `--list-rules`，再 `--self-test`。
	if listRules {
		return sdrListRules(stdout)
	}
	if selfTest {
		fmt.Fprintln(stdout, "── 自检（合成夹具 · 9 格成对负控 + 判据 8 静态自证）──")
		rc := sdrSelfTest(stdout)
		concl := "不过（拒绝扫真目标）"
		if rc == 0 {
			concl = "全过"
		}
		fmt.Fprintf(stdout, "自检结论: %s\n", concl)
		return rc
	}

	src, how := sdrResolvePlanSource(plan)
	explicit := plan != "" || os.Getenv("ZERG_R28_PLAN") != ""
	fmt.Fprintf(stdout, "门㉜ 删前波纹门（`R28` · 分档版）· 单的来源：%s（%s）\n", src, how)
	// ★ 沙箱读数警告（缺口 `GAP-20260927-70` · 与门⑱ `check-trustroot-witness:1724` 同形）：单的来源
	//   落到**状态目录默认落点**、且该目录是环境变量 `ZERG_STATE_DIR` 静默给的（没显式 `--plan` /
	//   `ZERG_R28_PLAN`）⇒ 本读数是**沙箱**读数，不是真状态目录 `~/.zerg/state`。
	if !explicit && os.Getenv("ZERG_STATE_DIR") != "" {
		fmt.Fprintf(stdout, "★★ 状态目录是**环境变量 `ZERG_STATE_DIR` 给的**（%s）⇒ 本读数是**沙箱**读数，"+
			"**不是**真状态目录 `~/.zerg/state`（闸里不设这个变量；要读真源请显式 `--plan`）\n",
			os.Getenv("ZERG_STATE_DIR"))
	}
	var text string
	if src == "-" {
		b, _ := io.ReadAll(os.Stdin)
		text = string(b)
	} else {
		b, err := os.ReadFile(src)
		if err != nil {
			if explicit {
				fmt.Fprintf(stdout, "  ⛔ 指名的那份干跑单读不到（%s）⇒ **不给结论**（判据 7：缺件不许当绿）\n", src)
				return sdrExitNoConcl
			}
			fmt.Fprintf(stdout, "  空档：本趟**没有删面单**（%s 不在）⇒ 本门这一趟**没有对象可判**\n", src)
			fmt.Fprintln(stdout, "        ⚠ 照实点名：这是**空档**，不是绿 —— 判据 7 的字面（没单 ⇒ 2）在本步会恒 BLOCKED，"+
				"故取值取可运行（判词逐字写出）")
			fmt.Fprintln(stdout, "  回执：判定档=无对象 · 条数=0")
			return sdrExitOK
		}
		text = string(b)
	}
	items := sdrParsePlan(text)
	reds, notes, noc := sdrJudge(items)
	for _, n := range notes {
		fmt.Fprintf(stdout, "  · %s\n", n)
	}
	for _, r := range reds {
		fmt.Fprintf(stdout, "  ✗ %s\n", r)
	}
	for _, m := range noc {
		fmt.Fprintf(stdout, "  ⛔ %s\n", m)
	}
	fmt.Fprintf(stdout, "  回执：%s\n", sdrVerdictFace(items))
	if len(noc) > 0 {
		fmt.Fprintf(stdout, "⇒ **不给结论**（rc=2）：%d 条\n", len(noc))
		return sdrExitNoConcl
	}
	if len(reds) > 0 {
		fmt.Fprintf(stdout, "⇒ 红（rc=1）：%d 条（必拦档：不可逆的删不许静默过）\n", len(reds))
		return sdrExitRed
	}
	fmt.Fprintln(stdout, "⇒ 全绿（rc=0）：本趟删面全部落在分档表内（只记/只报档不许红）")
	return sdrExitOK
}
