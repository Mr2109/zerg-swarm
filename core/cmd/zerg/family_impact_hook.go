// family_impact_hook.go —— 影响面 → `zerg dev edit` 干跑的**钩子分档**（`A4` 件 · 只读）。
//
// 依据（逐字）：
//
//	· `设计-变更影响面-v1.6` **§四.3 钩子分档**（分档线画在**可逆性**上：**改 = 只记** ·
//	  **删 = 必拦**）× **§八 挂接点**（干跑分支多一段影响面摘要 · **只在受影响项 ≥1 时**打三行 ·
//	  真写路径与批准件判据**一字不动**）；
//	· `拍板清单-影响面-v1.6-20260922` **R28 详细裁决**（Mr2109 2026-09-22「按建议」）：
//	  **不采用**「删 = 必拦不分档」，采 `R37` 分两档 —— **符号级删 ⇒ 必拦** ·
//	  **整件删 / 搬出 ⇒ 只报 + 附一行「可找回证据」**；**保险**：整件删**一旦「不可找回」即自动升为必拦**，
//	  判据三条任一命中即升（① 未跟踪件 · ② 无归档副本 · ③ 仓外件）。
//	  一句话口径：**「能找回的只报，找不回的必拦」**；
//	· `任务单-影响面实施-20260922` §二 `A4` 判据①–④ · 红线（不许改真写前置语义 · 不许让
//	  `impact_digest` 变成放行条件 · 不许在审计里留卡片全文 · 不许动批准件验签那一段）。
//
// 本件是**纯判据 + 只读取数**：分类器、三条不可找回判据、可找回证据两件、摘要指纹。
// 它**不写**任何东西（不落审计 · 不写缓存 · 不改件 · 不碰归档区）。
//
// 口径三条（本件写死，便于第三方复算）：
//
//	① **分类器**（机械可判，只看两件东西：写后的内容 + 件名后缀）：
//	     写后 **0 字节** ⇒ `件级（整件）`（本命令今天的件级动作只有「件级清空」这一种**可表达**形态；
//	     「整件搬出」走 `git mv`，属 `C1` 的删面 —— **照实点名，不假装等价**）；
//	     `.go` 件且改后**少了顶层符号**（函数 / 方法 / 类型 —— 与 `A2` 的 `impactGoFileSymbols` 同一口径，
//	     只是改成从**内存内容**解析）⇒ `符号级删`；
//	     其余 ⇒ `改`；判不了（改前/改后解析不了）⇒ `不判（判不了）` ⇒ 归 `只记`（**宁少报不猜报**）。
//	② **两件可找回证据**（§6.1 前置条：缺一不许删）：① 被删对象的 `sha256`（删之前**现算**，
//	     与 `dev edit` 的 `before_sha256` **同一算法**）；② **一条可找回的路径**（`git` 那笔提交的
//	     `sha` + 件路径 / 归档区里的副本路径 —— 两选一，必给一个）。
//	③ **三条不可找回判据**（R28 保险 · 任一命中即升为必拦）：
//	     ① 未跟踪件（`git ls-files --error-unmatch` 不认 ⇒ 无 git 历史）；
//	     ② 无归档副本（**两条找回路径都取不到** —— 见下面那条口径点名）；
//	     ③ 仓外件（件不在仓根下；今天 `dev edit` 那条既有的越界判据**先挡在前面**，本档只照实登记）。
//
// ★ **口径点名（照实写，不偷偷选一边）**：`R28` 保险第 ② 条原文是「无归档副本（不在 `Zerg-归档/` 内）」，
// 若照字面**单判**（归档区里没有副本就算命中），则「整件搬出」这一档**几乎恒命中** —— 因为副本是
// **搬出那一刻**才产生的，决策时归档区里当然还没有 ⇒ 95.6% 的删会被白拦，与 R28 的裁决理由
// （「一律拦会让九成半的拦是白拦 ⇒ 人开始习惯性绕 ⇒ 拦本身失效」）**直接冲突**。
// 故本件的口径 = **「可找回」立成两条路径（git 历史 / 归档副本），两条都取不到才算第 ② 条命中**
// （判据① 与 ② 的读数**逐条打印**，两种读法的差口在输出里看得见）。**这一条口径需复核**（见回执）。
//
// 与既有条的接缝（**不重复立项** ✗）：**不改真写前置语义** ✗（真写前置 = 人签批准件 +
// 审计先落盘 + 语法闸，一字未动）· **不新立退码**（照 §4.1 那张表：干跑里判「必拦」⇒ 退 `2`，
// 与既有的「缺确认档 ⇒ 2」同一档，不新造码）· **不新增对外面**（不加命令、不加旗标）。
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ---- 分档取值（闭集 · 与 §四.3 与 R28 裁决逐字同名）------------------------------------------

const (
	// 三类动作（`R37` 的机械分类器落在「写后内容 + 件名后缀」上）。
	impactHookClassEdit    = "改"
	impactHookClassSymbol  = "符号级删"
	impactHookClassFile    = "件级（整件）"
	impactHookClassUnknown = "不判（判不了）"

	// 三档钩子（判决 · 干跑里打出来；「必拦」在干跑这一档退 `2`）。
	impactHookTierRecord = "只记"
	impactHookTierReport = "只报"
	impactHookTierBlock  = "必拦"

	// 归档区目录名（与本仓**并列**的兄弟目录 —— 不写死绝对路径：门 `check-hardcoded-private-paths.py`）。
	impactArchiveDirName = "Zerg-归档"

	// 归档区走查的条目上限（明说上限，不静默截；现读归档区 **38 件 / 6 目录** ⇒ 远不到这个数）。
	impactArchiveWalkMax = 20000
)

// impactHookFacts —— 判档要的**全部事实**（纯数据 ⇒ 成对负控能直接喂）。
type impactHookFacts struct {
	Class     string // 三类动作之一（impactHookClassOf 给）
	Rel       string // 件（仓内相对路径）
	BeforeSHA string // 被删对象的 sha256（删之前现算 · 与 dev edit 的 before_sha256 同一算法）
	Tracked   bool   // `git ls-files --error-unmatch` 认它
	GitSHA    string // `git log -1 --format=%H -- <件>`（可找回路径之一）
	Archive   string // 归档区里的副本路径（可找回路径之二；空 = 取不到）
	Outside   bool   // 件不在仓根下
}

// impactHook —— 一枚**钩子判决**（分档 + 依据 + 三条判据现读 + 可找回证据两件）。
type impactHook struct {
	Class    string
	Tier     string
	Why      string
	Recall   [3]string // 三条不可找回判据的现读判决（逐条给出，可用「命中 / 不命中」复算）
	Evidence []string  // 可找回证据两件（**只报**那一档必给；必拦那两档照实给读数）
	Facts    impactHookFacts
}

// ---- ① 分类器（机械可判）-------------------------------------------------------------------

// impactHookClassOf 判「这次是**改** / **符号级删** / **件级（整件）**」。
//
// 只看两件东西：**写后的内容**（0 字节 ⇒ 件级）与**件名后缀**（`.go` 才判符号面）。
// 不按「脚本里有没有某个字」猜 —— 判据要**机械可判**（与 `dev edit` 的语法自检同一条纪律）。
func impactHookClassOf(rel string, before, after []byte) (string, string) {
	if len(after) == 0 {
		return impactHookClassFile, "写后内容 **0 字节** ⇒ **件级（整件）**" +
			"（本命令今天的件级动作只有「件级清空」这一种可表达形态；「整件搬出」走 `git mv`，属 `C1` 的删面 —— 不等价，照实点名）"
	}
	if !strings.HasSuffix(strings.ToLower(rel), ".go") {
		return impactHookClassEdit, "非 `.go` 件 ⇒ **符号面判不了**（本件不自造第二套符号解析器）⇒ 归「改」（宁少报不猜报）"
	}
	bs, berr := impactGoSymbolsOfSource(before)
	if berr != nil {
		return impactHookClassUnknown, "改前的 `.go` 内容解析不了（" + impactFirstLine(berr.Error()) + "）⇒ 符号面**判不了**（不给结论）"
	}
	as, aerr := impactGoSymbolsOfSource(after)
	if aerr != nil {
		return impactHookClassUnknown, "改后的 `.go` 内容解析不了（" + impactFirstLine(aerr.Error()) + "）⇒ 符号面**判不了**（不给结论）"
	}
	if len(bs) == 0 {
		return impactHookClassEdit, "改前就没有顶层符号（函数 / 方法 / 类型一个都没有）⇒ 没有符号面可删 ⇒ 归「改」"
	}
	lost := []string{}
	for n := range bs {
		if _, ok := as[n]; !ok {
			lost = append(lost, n)
		}
	}
	if len(lost) > 0 {
		sort.Strings(lost)
		return impactHookClassSymbol, fmt.Sprintf("改后少了 **%d 个顶层符号**（%s）⇒ 内存里的形状变了",
			len(lost), strings.Join(lost, " · "))
	}
	return impactHookClassEdit, fmt.Sprintf("顶层符号一个不少（改前 %d 个 · 改后 %d 个）⇒ 只有内容面的改动", len(bs), len(as))
}

// impactGoSymbolsOfSource —— **从内存内容**取顶层函数 / 方法 / 类型名（`go/parser` 标准库 ⇒ 不引依赖）。
// 与 `impactGoFileSymbols`（从盘上读）同一口径，只是来源不同：干跑判的是**改完之后长什么样**，
// 那份内容**还没落盘**（落了盘才有副作用）。
//
// 返回三态：名字集合 / 出错（解析不了 —— 调用方按「判不了」处置，**不许**当成「符号都没了」）。
func impactGoSymbolsOfSource(src []byte) (map[string]int, error) {
	out := map[string]int{}
	if len(strings.TrimSpace(string(src))) == 0 {
		return out, nil
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "（内存内容）", src, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	for _, d := range f.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok && fn.Name != nil && fn.Name.Name != "_" {
			out[fn.Name.Name] = fset.Position(fn.Name.NamePos).Line
			continue
		}
		if gd, ok := d.(*ast.GenDecl); ok && gd.Tok == token.TYPE {
			for _, sp := range gd.Specs {
				if ts, ok := sp.(*ast.TypeSpec); ok && ts.Name != nil && ts.Name.Name != "_" {
					out[ts.Name.Name] = fset.Position(ts.Name.NamePos).Line
				}
			}
		}
	}
	return out, nil
}

// ---- ② 三条不可找回判据 + 可找回证据的取数（只读）------------------------------------------

// impactHookFactsOf 取判档要的事实（只读：`git ls-files` / `git log` / 走查归档区）。
func impactHookFactsOf(root, rel string, before []byte) impactHookFacts {
	f := impactHookFacts{Rel: rel, BeforeSHA: sha256Of(before)}
	if filepath.IsAbs(rel) {
		f.Outside = true
		return f
	}
	if r := filepath.ToSlash(filepath.Clean(filepath.FromSlash(rel))); r == ".." || strings.HasPrefix(r, "../") {
		f.Outside = true
		return f
	}
	if _, _, code, err := impactRunIn(root, "git", "ls-files", "--error-unmatch", "--", rel); err == nil && code == 0 {
		f.Tracked = true
	}
	f.GitSHA = impactLastCommit(root, rel)
	f.Archive = impactArchiveCopy(root, rel)
	return f
}

// impactArchiveCopy —— 归档区里有没有这一件的副本（**只读**：不写归档区、不 `chmod` —— 红线）。
// 口径两档写死：① 逐字同相对路径（`<归档区>/<件>`）优先；② 同名件（走查命中的第一条，字典序）。
// 归档区读不到 ⇒ 空串（**「读不到」不当「没有」**：`impactHookFacts` 里那条判据会把读数逐字打出来）。
func impactArchiveCopy(root, rel string) string {
	dir := impactArchiveDirOf(root)
	st, err := os.Stat(dir)
	if err != nil || !st.IsDir() {
		return ""
	}
	if st2, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel))); err == nil && !st2.IsDir() {
		return filepath.ToSlash(filepath.Join(impactArchiveDirName, rel))
	}
	base := filepath.Base(filepath.FromSlash(rel))
	best, n := "", 0
	_ = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		n++
		if n > impactArchiveWalkMax || d.IsDir() || d.Name() != base {
			return nil
		}
		if r, rerr := filepath.Rel(filepath.Dir(dir), p); rerr == nil {
			cur := filepath.ToSlash(r)
			if best == "" || cur < best {
				best = cur
			}
		}
		return nil
	})
	return best
}

// impactArchiveDirOf 归档区（与本仓并列的兄弟目录 · 环境变量优先 —— 不写死绝对路径）。
func impactArchiveDirOf(root string) string {
	if d := strings.TrimSpace(os.Getenv("ZERG_ARCHIVE_DIR")); d != "" {
		return d
	}
	return filepath.Join(filepath.Dir(filepath.Clean(root)), impactArchiveDirName)
}

// impactHookDecide —— **判档的唯一判定口**（纯函数 ⇒ 成对负控能直接喂「可找回 / 不可找回」两组事实）。
//
//	改 / 不判  ⇒ **只记**（`R14` 已拍「先只记」那一步 —— 「拒执」是第二步，**本件不写**）
//	符号级删   ⇒ **必拦**（无反向件、难找回；这一档**不查**三条判据）
//	件级（整件）⇒ 三条判据**任一命中** ⇒ 升 **必拦**；都不命中 ⇒ **只报 + 两件可找回证据**
func impactHookDecide(f impactHookFacts) impactHook {
	h := impactHook{Class: f.Class, Facts: f}
	// 三条不可找回判据（`R28` 保险 · 逐条现读，可用读数复算）。
	c1 := !f.Tracked
	c2 := strings.TrimSpace(f.Archive) == "" && strings.TrimSpace(f.GitSHA) == ""
	c3 := f.Outside
	h.Recall[0] = impactRecallLine("① 未跟踪件（无 git 历史）", c1, ifStr(!f.Tracked,
		"`git ls-files --error-unmatch -- "+f.Rel+"` 不认它（未跟踪 或 该目录不是 git 仓）", "认它（tracked）"))
	h.Recall[1] = impactRecallLine("② 无归档副本（两条找回路径都取不到）", c2,
		"git 提交="+dashIfEmpty(f.GitSHA)+" · 归档副本="+dashIfEmpty(f.Archive))
	h.Recall[2] = impactRecallLine("③ 仓外件", c3, ifStr(f.Outside, "件不在仓根下（绝对路径 / 带 `..` 段）", "件在仓根下"))
	hits := []string{}
	if c1 {
		hits = append(hits, "①")
	}
	if c2 {
		hits = append(hits, "②")
	}
	if c3 {
		hits = append(hits, "③")
	}

	// 可找回证据两件（§6.1 前置条：① sha256 ② 一条找回路径 —— **缺一不许删**）。
	path := ""
	if f.Archive != "" {
		path = "归档副本 `" + f.Archive + "`"
	}
	if f.GitSHA != "" {
		short := f.GitSHA
		if len(short) > 12 {
			short = short[:12]
		}
		p := "git 提交 `" + short + "` + 件路径 `" + f.Rel + "`（`git revert " + short + " -- " + f.Rel + "`）"
		if path == "" {
			path = p
		} else {
			path += " 或 " + p
		}
	}
	if path == "" {
		path = "**取不到**（两条路都没有：git 历史空 · 归档区无副本）"
	}
	h.Evidence = []string{
		"① 被删对象的 sha256 = " + dashIfEmpty(f.BeforeSHA) + "（删之前**现算** · 与 `dev edit` 的 `before_sha256` 同一算法）",
		"② 找回路径 = " + path,
	}

	switch f.Class {
	case impactHookClassSymbol:
		h.Tier = impactHookTierBlock
		h.Why = "**符号级删**（删函数 / 分支 / 契约字段 / 命令 —— 内存里的形状变了）⇒ 无反向件、难找回 ⇒ **必拦**" +
			"（`R28` 分档表第 1 行；这一档**不查**三条判据 —— 符号级删一律必拦）"
	case impactHookClassFile:
		if len(hits) > 0 {
			h.Tier = impactHookTierBlock
			h.Why = "**件级（整件）删，但不可找回** ⇒ 按 `R28` 的**保险**升为**必拦**（命中判据 " +
				strings.Join(hits, " · ") + "）—— 「能找回的只报，找不回的必拦」"
		} else {
			h.Tier = impactHookTierReport
			h.Why = "**件级（整件）删，可找回** ⇒ **只报 + 附一行「可找回证据」**（`R28` 分档表第 2 行）" +
				"—— 两条找回路径至少一条在：git 历史 / 归档副本"
		}
	default: // 改 / 不判（判不了）
		h.Tier = impactHookTierRecord
		if f.Class == impactHookClassUnknown {
			h.Why = "**改**（符号面判不了 ⇒ 归这一档）⇒ **只记**（`R14` 已拍「先只记」那一步；「拒执」是第二步，本件不写）" +
				" —— **照实点名：判不了的那一格今天漏拦**（缺口，不是绿）"
		} else {
			h.Why = "**改**（可逆：审计行有 `before_sha256` / `after_sha256` / 前后字节数）⇒ **只记**" +
				"（`R14` 已拍「先只记」那一步；「拒执」是第二步，且必须走 `V0–V7`）"
		}
	}
	return h
}

// impactRecallLine 一条判据的一行判决（命中 / 不命中 + 现读读数 —— 读数照实给，不润色）。
func impactRecallLine(name string, hit bool, reading string) string {
	return name + " = " + ifStr(hit, "**命中**", "不命中") + " · 读数： " + reading
}

// impactHookOf 取事实 ⇒ 判档（`dev edit` 干跑与真写两条路都走这一个口）。
func impactHookOf(root, rel string, before, after []byte) impactHook {
	class, why := impactHookClassOf(rel, before, after)
	f := impactHookFactsOf(root, rel, before)
	f.Class = class
	h := impactHookDecide(f)
	h.Why = "类=" + class + "（" + why + "）· " + h.Why
	return h
}

// ---- ③ 摘要指纹（审计行只留指纹，不留正文 —— §四.2）----------------------------------------

// impactDigestOf 波纹摘要的 `sha256`（"哪怕空集也算「有记录」" —— 摘要正文逐字进哈希，
// **正文本身不进审计**）。第三者可复算：摘要正文 → 同一条 `sha256`。
func impactDigestOf(summary string) string { return sha256Of([]byte(summary)) }

// ---- ④ 干跑里的那一段摘要（§八 挂接 1–3）----------------------------------------------------

// impactDryRunSummary —— 干跑要打的那一段（正文 + 条数 + 三行 + 没打三行的原因）。
type impactDryRunSummary struct {
	Text    string    // 摘要正文（`impact_digest` 逐字取它的 sha256）
	Taken   bool      // **取到数了**（`false` = 未取数 ⇒ 指纹那一格不许写，照「缺摘要」处置）
	Rows    int       // 六层全量条数（**裁前** —— 与 `zerg impact` 的退码口径同一份）
	Lines   [3]string // 人面三行（**只在受影响项 ≥1 时**才有）
	HeadSHA string
	Layers  []impactLayer
	Reason  string // 没打三行的原因（未取数 / 面内未见）
}

// impactDryRunSummaryOf 取数（**默认档**：§4.4 只吃毫秒层 + 编译器层；贵层按需 —— 干跑不背贵层）。
// 只读 · 零副作用：不写缓存、不落审计、不改件（缓存与毫秒档属 `A5`）。
func impactDryRunSummaryOf(root, rel string) impactDryRunSummary {
	s := impactDryRunSummary{}
	tgt, why := impactResolve(root, rel)
	if tgt == nil {
		s.Reason = "**未取数**（目标解析不到：`" + why + "` —— 件不在盘上 / 不在仓内 / 在册契约表读不到）"
		s.Text = "影响面：" + s.Reason + " —— 这是**没取到数**，不是「没影响」（`F6`：把『没报』读成『没影响』）"
		return s
	}
	layers := impactPullLayers(root, tgt, true)
	rows, total := impactCollectRows(layers)
	s.Layers, s.Rows, s.Taken = layers, total, true
	if l, ok := impactLayerBySeq(layers, "①"); ok {
		s.HeadSHA = l.HeadSHA
	}
	rev := impactReversibilityOf(root, tgt)
	l1, l2, l3 := impactHumanLines(tgt, layers, rows, rev, true)
	if total == 0 {
		s.Reason = "**面内未见**（六层全量 **0** 条）"
		s.Text = fmt.Sprintf("影响面：%s —— 按 §八 第 2 件**不打三行**（零影响时打三行 = 灌噪声）；「没报 ≠ 没事」（`F6`）· head_sha=%s",
			s.Reason, dashIfEmpty(s.HeadSHA))
		return s
	}
	s.Lines = [3]string{l1, l2, l3}
	s.Text = l1 + "\n" + l2 + "\n" + l3 + "\n" + impactPublicLine(layers) + "\n" +
		fmt.Sprintf("（六层全量 %d 条 · head_sha=%s）", total, dashIfEmpty(s.HeadSHA))
	return s
}

// emitImpactHookBlock —— stderr 的**钩子判决块**（人读几行 + **两行机器可读**）。
//
// 机器可读信号为什么落 stderr：六键包封里 `warnings` / `truncated` 在 `emitEnvelopeWith` 里是**硬写死**的
// （`warnings` 恒 `[]`、`truncated` 恒 `false`、`meta` 只给 `count/source/changed`）⇒ 照 `A1`/`A2`/`A3`
// 红线**不改 `emitEnvelope*`** ⇒ 信号落 stderr（缺口再点一次名，见层表那一条同款）。
func emitImpactHookBlock(w io.Writer, hook impactHook) {
	fmt.Fprintf(w, "%s: 影响面钩子（`A4` · §4.3 **钩子分档** · `R28` 裁决「能找回的只报，找不回的必拦」）\n", progName)
	fmt.Fprintf(w, "  类       ：%s\n", hook.Class)
	fmt.Fprintf(w, "  分档     ：**%s**\n", hook.Tier)
	fmt.Fprintf(w, "  分档依据 ：%s\n", hook.Why)
	fmt.Fprintf(w, "  三条不可找回判据（任一命中即升为必拦）：\n")
	for _, ln := range hook.Recall {
		fmt.Fprintf(w, "    · %s\n", ln)
	}
	fmt.Fprintf(w, "  可找回证据（两件 · §6.1「缺一不许删」）：\n")
	for _, ln := range hook.Evidence {
		fmt.Fprintf(w, "    · %s\n", ln)
	}
	// ★ 字面差口（照实点名，不粉饰）：本判决只落在**干跑**这一档。
	if hook.Tier == impactHookTierBlock {
		fmt.Fprintf(w, "  ★ 范围（照实点名）：本判决落在**干跑**这一档（退码 `2`）—— `dev edit` 的**真写前置**"+
			"（人签批准件 + 审计先落盘 + 语法闸）**一字未动**；`R28` 的「删前波纹门」是**另开一门**，属 `C1`，"+
			"本批**不许**顺手塞进 `dev edit` 的真写前置 ✗ ⇒ **今天这一条不拦真写**（差口照实登记，不假装已拦）\n")
	}
	fmt.Fprintf(w, "  ★ 卡片铁律（§3.5）：**只做参考 · 永不构成批准**（提 ≠ 批）—— 本判决**不是放行条件** ✗\n")
	// 机器可读两行（key=value · 一行一事实；**纯 ASCII 取值**，便于脚本切分；不给封包的第七键）。
	fmt.Fprintf(w, "impact_hook tier=%s class=%s retrievable=%s tracked=%s archive=%s git_sha=%s outside=%s rel=%s\n",
		hook.Tier, hook.Class, yn(hook.Tier != impactHookTierBlock),
		yn(hook.Facts.Tracked), yn(strings.TrimSpace(hook.Facts.Archive) != ""),
		kvOrDash(hook.Facts.GitSHA), yn(hook.Facts.Outside), hook.Facts.Rel)
	fmt.Fprintf(w, "impact_criteria untracked=%s no_archive_copy=%s outside_repo=%s criteria_hits=%d\n",
		yn(strings.Contains(hook.Recall[0], "**命中**")),
		yn(strings.Contains(hook.Recall[1], "**命中**")),
		yn(strings.Contains(hook.Recall[2], "**命中**")),
		impactCriteriaHits(hook))
}

// yn —— 机器可读那一行的取值（纯 ASCII：`yes` / `no` —— 让脚本切分不用碰中文标点）。
func yn(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// kvOrDash 机器可读那一行的空值占位（空 ⇒ `-`，不吐中文括号）。
func kvOrDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return strings.TrimSpace(s)
}

// impactCriteriaHits 三条不可找回判据命中了几条（机器面用；人面那三行逐条给读数）。
func impactCriteriaHits(h impactHook) int {
	n := 0
	for _, ln := range h.Recall {
		if strings.Contains(ln, "**命中**") {
			n++
		}
	}
	return n
}

// emitImpactDryRunSummary —— 干跑里那一段（人面三行 **只在受影响项 ≥1 时**打 · §八 第 2 件）。
// 摘要正文一律打（哪怕「未取数」/「面内未见」—— 那是**照实说没取到**，不是灌噪声）。
//
// ★ 判「打不打三行」看的是 `Lines[0]`（`Lines` 是定长数组 ⇒ `len()` 恒 3，用它判会恒真 —— 踩过）。
func emitImpactDryRunSummary(w io.Writer, s impactDryRunSummary) {
	if strings.TrimSpace(s.Lines[0]) == "" {
		fmt.Fprintf(w, "%s: 影响面摘要：%s\n", progName, s.Text)
		return
	}
	for _, ln := range s.Lines {
		// 人面三行**字头与顺序**与 `zerg impact` 同一份（不另立字头 §3.1）。
		fmt.Fprintf(w, "%s\n", ln)
	}
	if pl := impactPublicLine(s.Layers); pl != "" {
		fmt.Fprintf(w, "%s\n", pl)
	}
	fmt.Fprintf(w, "%s: 影响面摘要：六层全量 %d 条 · head_sha=%s（默认档 · §4.4 只吃毫秒层 + 编译器层）\n",
		progName, s.Rows, dashIfEmpty(s.HeadSHA))
}
