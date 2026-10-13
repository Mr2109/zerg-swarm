// check_dryrun_yes_conflict_coverage.go —— 判据件
// `scripts/gates/check-dryrun-yes-conflict-coverage.py`（170 行）的 **Go 真搬**（M2 · 搬运面 ·
// 入口 `checkDryrunYesConflictCoverageGateMain`）。
//
//	M2 = 按价值真搬 Go（逐批判据搬 Go · 落点 `core/cmd/zerg/`）。验收判据 = 对同一 argv 跑两态
//	（py 原件 ⟷ 本入口），**stdout / stderr / 退码 三面逐字相同**（除下列显式声明的 argparse
//	面差异与未覆盖面）。
//
// 本件做的事（与 py 同构 · 只读 · 不开子进程 · 不改任何状态）：扫描 `core/cmd/zerg/*.go`
//
//	（去测试件 · 去 main.go），认「件读【inv 点 dryRun】」为**应产者**，认「件含共享判定
//	dryRunYesConflict、执行门 cmdGuarded 或写面确认档 writeGuardYes」为**真产者**；应产者 ∖ 真产者
//	里落在冻结名单（只读干跑面 READ_ONLY_FACES）**或属只读/转发面（全件无 yes 面）**之外的 ⇒ 报红；
//	空域（0 应产者）⇒ fail-closed rc=2。
//
//	★ 2026-10-14（判据侧过时写法修 · 只扩真产者面 · 不动应产者面）：见下方 `dycHelperRe`。
//	★ 2026-10-14（判据侧语义收窄 · 缺口 GAP-20261014-65 · 只缩**报红面** · ✗ 不动应产者面）：
//	  见下方 `dycYesFaceRe` —— 只读面/只转发面（`gate.go` 门跑干跑档现读 `:223`、`remote.go` 干跑
//	  跳过「无目标拒执」现读 `:205`）曾因 READS_RE 过宽被当写面 ⇒ 恒红、永不收敛。
//
//	★★ 2026-10-14（判据侧**粒度升级** · 缺口 GAP-20261014-66 的后半 · 只加严 · ✗ 不放宽 · ✗ 不动应产者面）：
//	  见下方「★ 调用点粒度」一节 —— 件粒度 token 判（`dycHelperRe`）**与运行时不同尺**：一个件里只要
//	  **某一条**命令调了判定字面，整个件就绿，哪怕**另一条命令**的判定排在它自己的 `--dry-run` 干跑
//	  早退**之后**（两档同给时先 `return` ⇒ 判定走不到 ⇒ 静默吞 `--yes`）。本档把判面落到**命令块**：
//	  按列 0 的 `func` 边界切块、逐块算「干跑计划支」与「判定可达性」，缺则点名 `件:命令`；
//	  与件粒度面**取并集**（✗ 不减报任何旧报红面）⇒ 同一件再也不会「件粒度绿而运行时吞 `--yes`」。
//
//	★★ 2026-10-14（判据侧**收尾** · 缺口 GAP-20261014-66 后半第二刀 · 只收窄**假红面** · ✗ 不放宽 · ✗ 不动应产者面）：
//	  见下方「★ 被守卫的助手件档」一节 —— 件粒度 token 判对**纯助手件**（全件无 `cmd[A-Z]` 块、守卫长在
//	  它的**调用方命令块**里 ⇒ 跨件）只见「本件无判定字面」⇒ 判缺 = 假红。本档对每件**现算调用链**：
//	  读点函数的**每一条**跨件调用方块都含判定字面且排在调用**之前** ⇒ 视同真产者；取不到调用方 ⇒ 仍判缺。
//
// 适格（照实）：纯静态只读 —— 零 subprocess / 零联网 / 零 TTY / 零计时 / 默认档零写面；
//
//	`tempfile` 只在 `--self-test` 里造件（py 用 `mkdtemp` 无回收 = A 类 tempdir 泄漏；Go 半
//	按「建点成功那一支」`defer os.RemoveAll` 止血，输出不可见差异 ⇒ 已实测 delta=0）。
//	外部树走显式旋钮 `--root`（缺省 = cwd，与 py `a.root or os.getcwd()` 逐字同构）。
//	退码 0/1/2 三档齐 · 现读 py import 面：argparse / json / os / re / sys / tempfile。
//
// 本单边界（照实）：本件**只落码、不接线** ——
//
//	· ✗ 不在 `main.go` 里注册命令 · ✗ 不改 `gate_inproc_table.go` · ✗ 删 py 原件；
//	· 入口 `checkDryrunYesConflictCoverageGateMain` 目前**无调用点**（M2 的刻意状态）；
//	· ✗ 不改登记面 / `family_*.go` / 已落旧件。
//
// ═══════════════════ ★ 两条出厂声明 ═══════════════════
//
//	(a) 默认根定位差异：本 py 缺省根 = `a.root or os.getcwd()`（**cwd**，非 `__file__`）⟷ Go 同为
//	  cwd（`os.Getwd`，后者优先 `$PWD`）⇒ 两态同构，唯一残留是「符号链接 cwd」：py `os.getcwd()`
//	  返词法真路径、Go `os.Getwd()` 可能读 `$PWD` ⇒ 对拍/夹具一律落**非符号链接路径**（真仓 cwd
//	  非软链 ⇒ 不触发）。
//	(b) 自指检查：★ 本件**落在它自己的扫描面内**（本件就在 `core/cmd/zerg/`）⇒ **凡 `inv` 接 `.`
//	  接 `dryRun` 的连续字面一律拆串**（`dycInvDry` = 两段拼接），并把两处运行时文案（空域 stderr
//	  ＋ 报红 stdout）同法拆串。已用单测 `TestDycSourceSelfRefClean` **现读本件源**断言拆串有效
//	  （零命中）。判据件本体（py）住 `scripts/gates/`、不在扫描面内，故不受此条影响。
//
// ═══════════════════ ★ 遍历序（逐点交代）═══════════════════
//
//	py `sorted(os.listdir(d))` —— **显式排序**（按码点；UTF-8 下 == Go 字节序）⇒ Go 用
//	`os.ReadDir(-1)` 收全名 ＋ `sort.Strings` 复现。输出行序（reads / covered / exempt / missing
//	明细）的决定者 = 这份排序。
package main

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
)

const (
	dycVersion = "0.3.0（已接线 · 门98 · tri-report 只报告 · 缺口 GAP-20260927-392/374/389 + GAP-20261014-66 调用点粒度/被守卫的助手件档）"
	dycScanRel = "core/cmd/zerg"
	dycProg    = "check-dryrun-yes-conflict-coverage.py"
	dycSchema  = "zerg/dryrun-yes-conflict-coverage/v1"
	// 拆串：避免本件源码被自己扫成「应产者」（见件头声明 (b)）。
	dycInvDry = "inv." + "dryRun"
	// 拆串：yes 面探针字面（只用于自检夹具；同样避开本件源码出现连续字面）。
	dycInvYes = "inv." + "yes"
)

// dycReadOnlyFaces —— py READ_ONLY_FACES（冻结名单：只读干跑面 · 有序）。
var dycReadOnlyFaces = []string{"export.go"}

// dycReadsRe —— py READS_RE `\binv\.dryRun\b`（拆串写，见声明 (b)）。
var dycReadsRe = regexp.MustCompile("\\binv\\." + "dryRun\\b")

// dycHelperRe —— py HELPER_RE `\bdryRunYesConflict\b|\bcmdGuarded\b` **＋ 本批扩面** `\bwriteGuardYes\b`
// （拆串写）。★ 2026-10-14（判据侧过时写法修）：`guard.go:211` 的 `writeGuardYes` 是**已开放**写面命令的
// fail-closed 确认档（唯一一处），它**第一句**就是共享判定 `dryRunYesConflict`（`guard.go:212`）⇒ 件里
// 调它 = 已接共享判定，与调 `cmdGuarded` 同义。此前只认 `dryRunYesConflict`/`cmdGuarded` 两枚字面 ⇒ 同族
// 真产者（同写面闸 ＋ 同三态，仅注释里没出现这两枚字面，如 `family_dev.go`）被判「未收敛」= 假红。
// ★ ✗ **不**并入 `guardedControlConfirmGate` / `writeGuardDryRun`：二者内部**不**调 `dryRunYesConflict`
// ⇒ 并入 = 把真未收敛件抹白（本批的红线）。
var dycHelperRe = regexp.MustCompile("\\bdryRun" + "YesConflict\\b|\\bcmd" + "Guarded\\b|\\bwrite" + "GuardYes\\b")

// dycYesFaceRe —— 写面/授权面判（判据侧语义收窄 · 缺口 GAP-20261014-65 · 拆串写，见声明 (b)）。
//
//	认「件能被 `--yes` 授权真落盘 / 真发写请求」为**写面**，形态有二：
//	  ① 件直读 yes 面（命令面自己的 `--yes` 确认档）；
//	  ② 件调任一写面闸助手（`dryRunYesConflict` / `cmdGuarded` / `writeGuardYes` —— 三者都排在「唯一
//	     写盘点之前」充当授权档，与直读 yes 面同义）。
//
//	★ 逐字反例（GAP-20261014-65 的两件**只读面/只转发面**：全件既无 yes 面、也不调任一写面闸助手
//	  ⇒ 结构上不存在「`--dry-run` 与 `--yes` 同给」病 ⇒ ✗ 不得当写面，否则恒红、永不收敛）：
//	  · `gate.go`（入账拄 `:220` / 现读 `:223`）门跑**干跑档** —— 只摆「将跑 vs 跳过」步表、不起
//	    子进程、立即 return；缺 `--dry-run` 时一个字节都不动。该行只**读** dryRun 决定「摆表 vs 照旧」，
//	    全件零写盘点。
//	  · `remote.go`（入账拄 / 现读 `:205`）干跑时**跳过**「无目标 ⇒ 拒执（exit 2）」这一路只改判词，
//	    全件零写盘点。
//
//	★ 这是**语义判面**（对每件现算）：新写面件自动进面、新只读面自动出面；✗ **不是**逐件白名单豁免
//	  （加白名单会遮盖新缺陷）。⇒ 本单 ✗ 不收「把两件加进 `dycReadOnlyFaces` 冻结名单」那条路。
var dycYesFaceRe = regexp.MustCompile("\\binv\\." + "yes\\b|\\bdryRun" + "YesConflict\\b|\\bcmd" + "Guarded\\b|\\bwrite" + "GuardYes\\b")

// ═══════════════ ★ 调用点粒度（缺口 GAP-20261014-66 的后半 · 只加严 · ✗ 不放宽）═══════════════
//
// 定义（一句话）：判面从「**件里**有没有判定字面」落到「**每条命令的判定调用点**在不在它自己的
// `--dry-run` 干跑早退**之前**」—— 前者对**次序**无鉴别力（字面在件里任何位置命中即绿，含注释、
// 含**别的函数**），后者与运行时同尺（同给 `--dry-run` 与 `--yes` 时，判定要是**走不到** ⇒
// 那就是「静默吞 `--yes`」）。
//
// 四条判据（缺者 ⟺ ⓪∧①∧②∧③∧¬④）：
//
//	⓪ **命令块**：块名是命令处理器（`cmdXxx` ⇒ 本仓命名法）—— 闸助手块（判定在调用点）不在本档判面，
//	  见 dycCmdBlockRe 的两态证伪；
//	① **真读**该字段：块内有 `inv` 接 `.` 接 `dryRun` 的**非赋值**形态（纯赋值 = 摆字段/转发 ⇒ ✗ 不算读）；
//	② **写面**：块内有 yes 面（`dycYesFaceRe` 同集）；
//	③ **干跑计划支**：块内有「一给 `--dry-run` 就进计划件」的 if 条件；
//	④ **判定可达**：首个判定字面排在「计划支起、其后首个 `return`」**之前**（含同行）。
//
// 实测三条真实形状（本档就是照它们定的）：
//
//	· `family_dev.go` 改前四条命令：判定排在计划支的 `return` **之后** ⇒ 缺（改前 DEV-2828 已前移 ⇒ 现读可达）。
//	· `family_gate_reap.go:432` 的形状：判定**嵌在**计划支里、先于它自己的 `return` ⇒ **可达**（✗ 不误伤）。
//	· `repo_commit` 的形状：计划支条件写成 `inv` 接 `.` 接 `dryRun` **或** 取反的 yes ⇒ 仍算计划支。
//
// 已知边界（照实 · ✗ 不藏）：判定字面出现在**注释**里同样算「可达」（与既有件粒度 token 判**同口径** ·
// 本档 ✗ 未换那条口径）；计划支条件**跨多行**书写（`if` 后接该字段再换行）⇒ 按**行**判 ⇒ 认不出（未覆盖）；
// 判定**经别的函数间接调用**（如 `writeGuardDryRun` 一类**不**在本档三枚登记字面里的助手）⇒ 本档按「无判定」判缺
// —— 这与件粒度面取并集后**只会更严**，✗ 不会更松。

// dycFuncLineRe —— 顶层函数起始行（列 0 的 `func`）⇒ 块切分锚（见 dycSplitBlocks）。
var dycFuncLineRe = regexp.MustCompile(`^func\s+(?:\([^)]*\)\s*)?([A-Za-z_][A-Za-z0-9_]*)\s*\(`)

// dycRetRe —— 返回行的词形探针（`\breturn\b`）。
var dycRetRe = regexp.MustCompile(`\breturn\b`)

// dycIdentChar —— Go 标识符字符（判 `if` 词边界用）。
func dycIdentChar(b byte) bool {
	return b == '_' || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

// dycLinePlan —— 该行里**有没有**一条「干跑计划支条件」。
//
//	逐条扫行内的 `if`（按词边界认，跳过标识符里的 `if` 子串），取「`if` 之后到该行首个 `{`」那段
//	**条件正文**判两条：正向出现该字段（`!` 开头的不算）∧ **不**正向出现 yes 字段。
//	⇒ 单行写法（`if <该字段> { return 0 }`）与多语句同行都能认；而 `<该字段> || <yes 字段>` 这类
//	**两档同给即拒**的冲突拒执行、以及 `!<该字段> && !<yes 字段>` 的缺确认行，都 ✗ 不算计划支。
func dycLinePlan(line string) bool {
	for i := 0; i < len(line); {
		k := strings.Index(line[i:], "if")
		if k < 0 {
			return false
		}
		k += i
		if k > 0 && dycIdentChar(line[k-1]) {
			i = k + 2
			continue
		}
		if k+2 < len(line) && dycIdentChar(line[k+2]) {
			i = k + 2
			continue
		}
		rest := line[k+2:]
		j := strings.Index(rest, "{")
		if j < 0 {
			return false
		}
		cond := rest[:j]
		if dycPosMention(cond, dycInvDry) && !dycPosMention(cond, dycInvYes) {
			return true
		}
		i = k + 2
	}
	return false
}

// dycBlock —— 件内一段顶层函数块（名 + 该块的行）。
type dycBlock struct {
	Name  string
	Lines []string
}

// dycSplitBlocks —— 按顶层函数边界（列 0 的 `func`）切块。
//
//	· 首个函数之前（package / import / const / var）单独成块，名 `<件头>`；
//	· 件内无函数 ⇒ 整件即一块 `<件头>`（✓ 不丢内容）。
func dycSplitBlocks(txt string) []dycBlock {
	lines := strings.Split(txt, "\n")
	var out []dycBlock
	name, start := "<件头>", 0
	for i, ln := range lines {
		m := dycFuncLineRe.FindStringSubmatch(ln)
		if m == nil {
			continue
		}
		if i > start || name != "<件头>" {
			out = append(out, dycBlock{Name: name, Lines: lines[start:i]})
		}
		name, start = m[1], i
	}
	out = append(out, dycBlock{Name: name, Lines: lines[start:]})
	return out
}

// dycPosMention —— needle 在该行**非取反**地出现（`!<needle>` 不算「正向出现」）。
//
//	判的是「前一个非空白/非左括号字符不是 `!`」⇒ 条件里的该字段（正向）与 yes 字段（取反）
//	各自判得清楚（`<该字段> || !<yes 字段>` 里前者正向、后者取反）。
func dycPosMention(line, needle string) bool {
	off := 0
	for {
		i := strings.Index(line[off:], needle)
		if i < 0 {
			return false
		}
		i += off
		k := i - 1
		for k >= 0 && (line[k] == ' ' || line[k] == '\t' || line[k] == '(') {
			k--
		}
		if !(k >= 0 && line[k] == '!') {
			return true
		}
		off = i + len(needle)
	}
}

// dycHasDryRead —— 文本里有没有**非赋值形态**的 `inv` 接 `.` 接 `dryRun`（= 真的**读**该字段）。
//
//	赋值形态（该字段后跳空白紧跟**单个** `=`）不算读 —— 那是「摆字段」的解析/转发档
//	（实测反例 `deploy_launch.go:dplAuthFlags` 只**置位**该字段、无判定可言 ⇒ ✗ 不该进缺）。
//	`==` 是**比较**（算读）⇒ 逐 match 手工判（Go 的 RE2 无前瞻 · 见件头未覆盖面）。
func dycHasDryRead(txt string) bool {
	for _, m := range dycReadsRe.FindAllStringIndex(txt, -1) {
		j := m[1]
		for j < len(txt) && (txt[j] == ' ' || txt[j] == '\t') {
			j++
		}
		if j < len(txt) && txt[j] == '=' {
			if j+1 < len(txt) && txt[j+1] == '=' {
				return true // `==` 比较 ⇒ 读
			}
			continue // 单个 `=` ⇒ 赋值 ⇒ ✗ 不算读
		}
		return true
	}
	return false
}

// dycSiteFlags —— 单块的调用点判定输入（四项全由**块内**文本算 · ✗ 不跨块借）。
type dycSiteFlags struct {
	Reads   bool // ① 真读该字段
	YesFace bool // ② 有 yes 面
	Plan    bool // ③ 有干跑计划支
	Reach   bool // ④ 判定在「计划支起、首个 return」之前可达
}

// dycBlockSite —— 逐块算四项（判法与三条实测形状见本节抬头）。
//
//	「干跑计划支」= 首个含「干跑计划支条件」的行（条件判法见 dycLinePlan）。
//	  形态 a：条件里正向出现该字段（无 yes 字段）—— 干跑先 return ✓
//	  形态 b：`<该字段> || !<yes 字段>`（`repo commit` 的形状）—— 一给 `--dry-run` 就进计划件 ✓
//	  形态 c：`<该字段> || <yes 字段>`（两档同给即拒）—— **冲突拒执** ⇒ ✗ 不算计划支
//	  形态 d：`!<该字段> && !<yes 字段>`（缺 `--yes` 支）—— 反向条件 ⇒ ✗ 不算计划支
//
//	「可达」= 首个判定字面行号 ≤ 「计划支起、其后首个 return」行号（含同行）。
func dycBlockSite(b dycBlock) dycSiteFlags {
	reads := dycHasDryRead(strings.Join(b.Lines, "\n"))
	yesFace := false
	for _, ln := range b.Lines {
		if dycYesFaceRe.MatchString(ln) {
			yesFace = true
			break
		}
	}
	plan := -1
	for i, ln := range b.Lines {
		if dycLinePlan(ln) {
			plan = i
			break
		}
	}
	firstRet := -1
	if plan >= 0 {
		for i := plan; i < len(b.Lines); i++ {
			if dycRetRe.MatchString(b.Lines[i]) {
				firstRet = i
				break
			}
		}
	}
	helperIdx := -1
	for i, ln := range b.Lines {
		if dycHelperRe.MatchString(ln) {
			helperIdx = i
			break
		}
	}
	reach := helperIdx >= 0 && firstRet >= 0 && helperIdx <= firstRet
	return dycSiteFlags{Reads: reads, YesFace: yesFace, Plan: plan >= 0, Reach: reach}
}

// dycCmdBlockRe —— **命令块**名（本仓命名法：命令处理器一律 `cmdXxx`）。
//
//	★ 为什么只判命令块（实测 · ✗ 不是放水）：**闸助手块**（如 `family_eggs_cocoons.go:eggWriteGate`
//	—— 它自己先出计划件、后才查 `--yes`）的判定在**它的调用点**（调用者 `eggAct` 的块里、且排在
//	调用之前）⇒ 按块单看会把「助手 + 已被守卫的调用者」误判成缺。两态实测证伪：`zerg egg stop
//	<卵> --dry-run --yes` 与 `zerg egg run <卵> --dry-run --yes` **都 rc=2 + detail=dry_run_yes_conflict**
//	（真拦住了）⇒ 这两块 ✗ 不得报红。助手面由**件粒度面**兜底（✗ 不是整块不看）。
var dycCmdBlockRe = regexp.MustCompile(`^cmd[A-Z]`)

// dycSiteMissing —— 单件的「调用点粒度缺者」逐块点名（形如 `件:命令`）。
//
//	★ 冻结名单（只读干跑面）整件豁免 —— 与件粒度面**同规**（✗ 不顺带收紧冻结名单；那条口径在
//	`dycJudge` 里，本档只补粒度、✗ 不碰名单语义）。
//	★ 只判**命令块**（见 dycCmdBlockRe：命令处理器命名法 + 闸助手块的两态证伪）。
func dycSiteMissing(file, txt string) []string {
	var out []string
	for _, b := range dycSplitBlocks(txt) {
		if !dycCmdBlockRe.MatchString(b.Name) {
			continue
		}
		f := dycBlockSite(b)
		if f.Reads && f.YesFace && f.Plan && !f.Reach {
			out = append(out, file+":"+b.Name)
		}
	}
	return out
}

// ═══════════════ ★ 被守卫的助手件档（缺口 GAP-20261014-66 后半第二刀 · 只收窄**假红面** · ✗ 不放宽）══════
//
// 病（假红 · 逐字实证）：**纯助手件**（全件无 `cmd[A-Z]` 命令块）读 `inv` 接 `.` 接 `dryRun`、且有 yes 面，
// 但它的判定（干跑早退）长在**它的调用方命令块**里（跨件）⇒ 件粒度 token 判只见「本件无判定字面」⇒ 判缺。
//
// 反例逐字（`core/cmd/zerg/family_impact_backfill.go`）：
//
//	· 本件唯一读点 = `emitImpactBackfillBlock`（`:773` 的 `realWrite` 组合式 = `inv` 接 `.` 接 `dryRun` 取反
//	  ＋ `inv.confirmGiven` ＋ `inv` 接 `.` 接 `yes`；另 `:792 } else if ` 同该字段 ` {`）；
//	  唯一调用方 = `family_impact.go:269`（`cmdImpact` 块内）；
//	· `cmdImpact` 的判定排在调用点**之前**：`family_impact.go:186 writeGuardDryRun`（内含共享判定）
//	  ＋ `:189 writeGuardYes`（`guard.go:212` 首句即共享判定）⇒ 运行时 `--dry-run --yes` 同给 rc=2 **真拦**。
//	⇒ 该件结构上无「同给静默吞 `--yes`」病灶面 ⇒ ✗ 不得判缺。
//
// 判法（对每件现算 · fail-closed · **语义判**·✗ 非白名单）：件 F 无 `cmd[A-Z]` 块 ∧ F 的**每个**读点函数
// 的**每一条**跨件调用方块都含判定字面（`dycHelperRe`）且判定字面排在调用**之前**（含同行）⇒ 视同真产者。
// 任一读点函数取不到调用方、或有一条调用方块无判定字面 / 判定排在调用之后 ⇒ 仍判缺（✗ 不因「像助手」放行）。
//
// 已知边界（照实 · ✗ 不藏）：调用方判据同走 token 面（判定字面出现在**注释**里同样算「可达」· 与既有
// 件粒度 token 判**同口径**）；`func …(…)` **定义行**不算调用（跨件同名函数常见 ⇒ 见 dycCallLineIdx）。

// dycBlockRef —— 跨件块索引（件名 + 块的行），供本档现算调用方。
type dycBlockRef struct {
	File  string
	Lines []string
}

// dycAllRefs —— 每次 dycScan 起手清空重建（✗ 不许跨调用残留）。**每件（含非应产者）**都进
// —— 调用方未必是应产者（实测 `family_impact.go` 就不读该字段）。
var dycAllRefs []dycBlockRef

// dycGuardedHelper —— 「被守卫的助手件」档（对每件现算 · fail-closed）。
func dycGuardedHelper(file, txt string) bool {
	for _, b := range dycSplitBlocks(txt) {
		if dycCmdBlockRe.MatchString(b.Name) {
			return false // 有命令块 ⇒ 不是纯助手件（走件粒度原档 · ✗ 不放行）
		}
	}
	var readers []string
	for _, b := range dycSplitBlocks(txt) {
		if dycHasDryRead(strings.Join(b.Lines, "\n")) {
			readers = append(readers, b.Name)
		}
	}
	if len(readers) == 0 {
		return false
	}
	for _, fn := range readers {
		re := regexp.MustCompile(`\b` + regexp.QuoteMeta(fn) + `\s*\(`)
		found := false
		for _, r := range dycAllRefs {
			if r.File == file {
				continue // 只算**跨件**调用方（本件内互调 ⇒ ✗ 不算「守卫长在调用方」）
			}
			at := dycCallLineIdx(r.Lines, re)
			if at < 0 {
				continue
			}
			found = true
			if !dycHelperBeforeCall(r.Lines, at) {
				return false
			}
		}
		if !found {
			return false // 取不到调用方 ⇒ fail-closed
		}
	}
	return true
}

// dycCallLineIdx —— 块内首个「调用行」行号（`\bname\s*\(` 且该行不是 `func …` 定义行）；取不到 ⇒ -1。
func dycCallLineIdx(lines []string, re *regexp.Regexp) int {
	for i, ln := range lines {
		t := strings.TrimSpace(ln)
		if strings.HasPrefix(t, "func ") || strings.HasPrefix(t, "func(") {
			continue // 定义行 ✗ 不算调用（跨件同名函数 ⇒ 防误认）
		}
		if re.MatchString(ln) {
			return i
		}
	}
	return -1
}

// dycHelperBeforeCall —— 块内首个判定字面排在调用行之前（含同行）⇒ true。
func dycHelperBeforeCall(lines []string, call int) bool {
	for i := 0; i <= call && i < len(lines); i++ {
		if dycHelperRe.MatchString(lines[i]) {
			return true
		}
	}
	return false
}

// dycInList —— 名单判。
func dycInList(s string, list []string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// dycSitesOf —— 全场「调用点粒度缺者」（冻结名单件整件跳过 · 序 = rows 序 × 块序）。
func dycSitesOf(rows []dycRow, readOnly []string) []string {
	var out []string
	for _, r := range rows {
		if dycInList(r.File, readOnly) {
			continue
		}
		out = append(out, r.Sites...)
	}
	return out
}

// ── py 路径原语 ────────────────────────────────────────────────────────────
// dycPyJoin —— 复刻 CPython `posixpath.join`（空段 / 后段绝对路径 / 尾斜杠三态逐段直译）。
func dycPyJoin(parts ...string) string {
	if len(parts) == 0 {
		return ""
	}
	path := parts[0]
	for _, b := range parts[1:] {
		if strings.HasPrefix(b, "/") {
			path = b
		} else if path == "" || strings.HasSuffix(path, "/") {
			path += b
		} else {
			path += "/" + b
		}
	}
	return path
}

// ── py json.dumps(ensure_ascii=False) 的字符串字面 ─────────────────────────
func dycJSONStr(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 {
				b.WriteString(fmt.Sprintf(`\u%04x`, r))
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

func dycJSONStrList(items []string) string {
	parts := make([]string, len(items))
	for i, s := range items {
		parts[i] = dycJSONStr(s)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// ── 判据体 ────────────────────────────────────────────────────────────────
type dycRow struct {
	File    string
	Helper  bool
	YesFace bool
	// HelperGuarded —— 「被守卫的助手件」档（见本节抬头）：纯助手件、守卫长在它的调用方命令块里（跨件）
	// ⇒ 视同真产者（现算判定 · ✗ 不是白名单）。
	HelperGuarded bool
	// Sites —— 该件的「调用点粒度缺者」（`件:命令` · 只列**本件内**的块；冻结名单的过滤在 dycSitesOf）。
	Sites []string
}

// dycReadFails —— B 类读失败登记面（读不了的应产者件 ⇒ 逐件登记 + err 正文）。
// ★ B 类真口子修（与 py **有意分叉** · 同族先例 `check_doc_cmds.go` 的 `dcmdReadFails`）：
//
//	读失败 ⇒ 登记 + 入口判红（stderr BLOCKED 头 + 逐件点名）⇒ rc=2（✗ 不再静默少判）。
//	自清点：每次 `dycScan` 起手清空 ⇒ 不许跨调用残留。
var dycReadFails []string

// dycScan —— py `scan(root)`：返 (rows, errMsg)。errMsg != "" ⇒ 不给结论（rc=2）。
func dycScan(root string) ([]dycRow, string) {
	dycReadFails = nil // ★ 自清：包级登记面不许跨调用残留
	dycAllRefs = nil   // ★ 自清：「被守卫的助手件」档的跨件块索引不许跨调用残留
	d := dycPyJoin(root, dycScanRel)
	st, err := os.Stat(d)
	if err != nil || !st.IsDir() {
		return nil, fmt.Sprintf("扫描根不在盘：%s", d)
	}
	entries, err := os.ReadDir(d)
	if err != nil {
		// py `os.listdir` 抛未捕 OSError（C 类无兜底）⇒ 本件按新口径 fail-closed（并入「扫描根不在盘」）。
		return nil, fmt.Sprintf("扫描根不在盘：%s", d)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	// ★ 两趟：① 逐件读文本 + 建**跨件块索引**（调用方未必是应产者 ⇒ 每件都进）；
	//   ② 逐应产者算 row（「被守卫的助手件」档要**跨件现算** ⇒ 必须等索引齐了再判）。
	type dycSrc struct{ name, txt string }
	var cands []dycSrc
	for _, fn := range names {
		if !strings.HasSuffix(fn, ".go") || strings.HasSuffix(fn, "_test.go") {
			continue
		}
		if fn == "main.go" {
			continue
		}
		b, err := os.ReadFile(dycPyJoin(d, fn))
		if err != nil {
			// ★ B 类真口子修（原 py `except OSError: continue` 静默 · ✗ 不再静默）：登记 + 判红 + rc=2。
			dycReadFails = append(dycReadFails, fmt.Sprintf("%s（%v）", fn, err))
			continue
		}
		txt := string(b)
		for _, blk := range dycSplitBlocks(txt) {
			dycAllRefs = append(dycAllRefs, dycBlockRef{File: fn, Lines: blk.Lines})
		}
		if !dycReadsRe.MatchString(txt) {
			continue
		}
		cands = append(cands, dycSrc{name: fn, txt: txt})
	}
	var rows []dycRow
	for _, c := range cands {
		rows = append(rows, dycRow{File: c.name, Helper: dycHelperRe.MatchString(c.txt), YesFace: dycYesFaceRe.MatchString(c.txt),
			HelperGuarded: dycGuardedHelper(c.name, c.txt),
			Sites:         dycSiteMissing(c.name, c.txt)})
	}
	return rows, ""
}

// dycJudge —— py `judge(rows, read_only)`：四列按 rows 序分别追加（缺 / 真产 / 豁免 / 只读转发）。
func dycJudge(rows []dycRow, readOnly []string) (missing, covered, exempt, readOnlyFaces []string) {
	inRO := func(f string) bool {
		for _, x := range readOnly {
			if x == f {
				return true
			}
		}
		return false
	}
	for _, r := range rows {
		switch {
		case r.Helper:
			covered = append(covered, r.File)
		case inRO(r.File):
			exempt = append(exempt, r.File)
		case !r.YesFace:
			// ★ 只读面/只转发面（GAP-20261014-65）：全件无 yes 面 ⇒ 结构上无「--dry-run 与 --yes 同给」
			// 病 ⇒ 不入缺（**语义判面** · 对每件现算 · ✗ 不是逐件白名单豁免）。
			readOnlyFaces = append(readOnlyFaces, r.File)
		case r.HelperGuarded:
			// ★ 被守卫的助手件档（GAP-20261014-66 后半第二刀）：纯助手件、守卫长在它的调用方命令块里
			// （跨件）⇒ 视同真产者（**语义判面** · 现算调用链 · 取不到调用方 ⇒ 仍判缺 · ✗ 不是白名单）。
			covered = append(covered, r.File)
		default:
			missing = append(missing, r.File)
		}
	}
	return missing, covered, exempt, readOnlyFaces
}

// dycSelfTest —— py `self_test(out)`：7 格；建点成功才挂 `defer os.RemoveAll`（A 类止血）。
func dycSelfTest(out io.Writer) bool {
	ok := true
	total := 0
	d, err := os.MkdirTemp("", "drcov-selftest-")
	if err != nil {
		d = ""
	}
	if d != "" {
		defer os.RemoveAll(d)
	}
	scanDir := dycPyJoin(d, dycScanRel)
	_ = os.MkdirAll(scanDir, 0o755)
	put := func(name, body string) {
		_ = os.WriteFile(dycPyJoin(scanDir, name), []byte(body), 0o644)
	}
	// a.go：**写面**（含 yes 面）＋ 读 dryRun ＋ 无判定 ⇒ 缺者（报红）。
	put("a.go", "package main\nfunc f(inv *invocation) int { if "+dycInvDry+" { return 0 }; _ = "+dycInvYes+"; return 1 }\n")
	put("b.go", "package main\nfunc f(inv *invocation) int { if rc := dryRun"+"YesConflict(inv, nil); rc != 0 { return rc }; _ = "+dycInvDry+"; return 0 }\n")
	put("c.go", "package main\nfunc f(inv *invocation) int { if "+dycInvDry+" { return 0 }; return cmd"+"Guarded(inv, nil, nil) }\n")
	put("d_test.go", "package main\nfunc t() { _ = "+dycInvDry+" }\n")                                     // 测试件：排除
	put("e.go", "package main\nfunc f(inv *invocation) int { if "+dycInvDry+" { return 0 }; return 1 }\n") // 只读面：无 yes 面 ⇒ 不入缺
	rows, errMsg := dycScan(d)
	files := make([]string, 0, len(rows))
	for _, r := range rows {
		files = append(files, r.File)
	}
	hasFile := func(n string) bool {
		for _, f := range files {
			if f == n {
				return true
			}
		}
		return false
	}
	missing, covered, _, readOnly := dycJudge(rows, dycReadOnlyFaces)
	_, _, exemptRO, _ := dycJudge(rows, []string{"a.go"})

	empty, eerr := os.MkdirTemp("", "drcov-empty-")
	if eerr == nil {
		defer os.RemoveAll(empty)
		_ = os.MkdirAll(dycPyJoin(empty, dycScanRel), 0o755)
	}
	rowsE, errE := dycScan(empty)

	// ★ 调用点粒度夹具（**独立目录** ⇒ ✗ 不扰上面件粒度那几格）：
	//
	//	f.go —— 同件**一闸一吞**（cmdGood 判定在命令首句；cmdBad 的干跑支先 return ⇒ 吞 `--yes`）；
	//	g.go —— 成对负控：判定**排在干跑早退之后** ⇒ 必红点名（件粒度 token 判看不出的那一半）；
	//	h.go —— 反例：真写面 且**每条命令**的判定都在早退之前（两枚判定字面各用一枚）⇒ ✗ 不得点名。
	sitesS := []string{}
	errS := "调用点粒度夹具建点失败"
	if dS, serr := os.MkdirTemp("", "drcov-site-"); serr == nil {
		defer os.RemoveAll(dS)
		sd := dycPyJoin(dS, dycScanRel)
		_ = os.MkdirAll(sd, 0o755)
		putS := func(name, body string) {
			_ = os.WriteFile(dycPyJoin(sd, name), []byte(body), 0o644)
		}
		putS("f.go", "package main\n"+
			"func cmdGood(inv *invocation) int {\n"+
			"	if rc := dryRun"+"YesConflict(inv, nil); rc != 0 {\n		return rc\n	}\n"+
			"	if "+dycInvDry+" {\n		return 0\n	}\n"+
			"	_ = "+dycInvYes+"\n	return 1\n}\n"+
			"func cmdBad(inv *invocation) int {\n"+
			"	if "+dycInvDry+" {\n		return 0\n	}\n"+
			"	_ = "+dycInvYes+"\n	return 1\n}\n")
		putS("g.go", "package main\n"+
			"func cmdLate(inv *invocation) int {\n"+
			"	if "+dycInvDry+" {\n		return 0\n	}\n"+
			"	if rc := dryRun"+"YesConflict(inv, nil); rc != 0 {\n		return rc\n	}\n"+
			"	_ = "+dycInvYes+"\n	return 1\n}\n")
		putS("h.go", "package main\n"+
			"func cmdAlpha(inv *invocation) int {\n"+
			"	if rc := dryRun"+"YesConflict(inv, nil); rc != 0 {\n		return rc\n	}\n"+
			"	if "+dycInvDry+" {\n		return 0\n	}\n"+
			"	_ = "+dycInvYes+"\n	return 1\n}\n"+
			"func cmdBeta(inv *invocation) int {\n"+
			"	if rc := write"+"GuardYes(inv, nil, nil); rc != 0 {\n		return rc\n	}\n"+
			"	if "+dycInvDry+" {\n		return 0\n	}\n"+
			"	_ = "+dycInvYes+"\n	return 1\n}\n")
		rowsS, e := dycScan(dS)
		errS = e
		sitesS = dycSitesOf(rowsS, dycReadOnlyFaces)
	}

	// ★ 被守卫的助手件档夹具（**独立目录** ⇒ ✗ 不扰上面两批）：
	//
	//	i.go —— 命令块（判定排在最前）调**另一件** j.go 的读点函数；
	//	j.go —— 纯助手件（无 `cmd` 块）读该字段 + yes 面 ⇒ 守卫在调用方 ⇒ ✗ 不得判缺；
	//	k.go —— 成对负控：命令块**无**判定字面、调 l.go 的读点函数（两态必红：调用点 + 件粒度）；
	//	l.go —— 无守卫的助手件 ⇒ 必判缺。
	var hMissing, hCovered, hSites []string
	errH := "被守卫的助手件档夹具建点失败"
	if dH, herr := os.MkdirTemp("", "drcov-helper-"); herr == nil {
		defer os.RemoveAll(dH)
		hd := dycPyJoin(dH, dycScanRel)
		_ = os.MkdirAll(hd, 0o755)
		putH := func(name, body string) {
			_ = os.WriteFile(dycPyJoin(hd, name), []byte(body), 0o644)
		}
		putH("i.go", "package main\n"+
			"func cmdI(inv *invocation) int {\n"+
			"	if rc := dryRun"+"YesConflict(inv, nil); rc != 0 {\n		return rc\n	}\n"+
			"	if "+dycInvDry+" {\n		return 0\n	}\n"+
			"	_ = "+dycInvYes+"\n	return helperRead(inv)\n}\n")
		putH("j.go", "package main\n"+
			"func helperRead(inv *invocation) int {\n"+
			"	if "+dycInvDry+" {\n		return 0\n	}\n"+
			"	_ = "+dycInvYes+"\n	return 1\n}\n")
		putH("k.go", "package main\n"+
			"func cmdK(inv *invocation) int {\n"+
			"	if "+dycInvDry+" {\n		return 0\n	}\n"+
			"	_ = "+dycInvYes+"\n	return helperUnguarded(inv)\n}\n")
		putH("l.go", "package main\n"+
			"func helperUnguarded(inv *invocation) int {\n"+
			"	if "+dycInvDry+" {\n		return 0\n	}\n"+
			"	_ = "+dycInvYes+"\n	return 1\n}\n")
		rowsH, eH := dycScan(dH)
		errH = eH
		hMissing, hCovered, _, _ = dycJudge(rowsH, dycReadOnlyFaces)
		hSites = dycSitesOf(rowsH, dycReadOnlyFaces)
	}

	type tc struct {
		name string
		good bool
	}
	cases := []tc{
		{"① 应产者被抓住（a.go 无判定）", errMsg == "" && hasFile("a.go")},
		{"② a.go 报红（缺者报红）", len(missing) == 1 && missing[0] == "a.go"},
		{"③ b.go/c.go 真产者不入缺", len(covered) == 2 && covered[0] == "b.go" && covered[1] == "c.go"},
		{"④ 测试件被排除", !hasFile("d_test.go")},
		{"⑤ 冻结名单豁免", len(exemptRO) == 1 && exemptRO[0] == "a.go"},
		{"⑥ 空域：0 应产者（空域 ≠ 全绿）", errE == "" && len(rowsE) == 0},
		{"⑦ 只读/转发面（无 yes 面）不入缺", len(readOnly) == 1 && readOnly[0] == "e.go"},
		{"⑧ 调用点：同件一闸一吞 ⇒ 只点名吞的那条命令", errS == "" && len(sitesS) == 2 && sitesS[0] == "f.go:cmdBad"},
		{"⑨ 调用点：判定排在干跑早退之后 ⇒ 必红（成对负控）", len(sitesS) == 2 && sitesS[1] == "g.go:cmdLate"},
		{"⑩ 调用点：条条命令判定都在早退之前 ⇒ ✗ 不点名", !dycInList("h.go:cmdAlpha", sitesS) && !dycInList("h.go:cmdBeta", sitesS)},
		{"⑪ 被守卫的助手件：守卫在调用方命令块里 ⇒ ✗ 不判缺", errH == "" && dycInList("j.go", hCovered) && !dycInList("j.go", hMissing)},
		{"⑫ 成对负控：调用方命令块无判定字面 ⇒ 助手件与调用点两态同红", dycInList("l.go", hMissing) && dycInList("k.go", hMissing) && dycInList("k.go:cmdK", hSites)},
	}
	nGood := 0
	for _, c := range cases {
		total++
		if c.good {
			nGood++
		}
		mark := "✗"
		if c.good {
			mark = "✓"
		}
		fmt.Fprintf(out, "  %s %s\n", mark, c.name)
		ok = ok && c.good
	}
	fmt.Fprintf(out, "  self-test: %d/%d\n", nGood, total)
	return ok
}

// ── argparse 面（照 py 同名旗标；本件无短旗标除 -h）────────────────────────
func dycUsage() string {
	first := "usage: " + dycProg + " [-h] [--root ROOT] [--json]"
	second := strings.Repeat(" ", len("usage: ")+len(dycProg)+1) + "[--emit-baseline] [--self-test]"
	return first + "\n" + second
}

func dycArgErr(stderr io.Writer, msg string) int {
	fmt.Fprintf(stderr, "%s\n%s: error: %s\n", dycUsage(), dycProg, msg)
	return 2
}

// dycHelp —— argparse `-h/--help` 正文（照 py3.9 措辞复刻；★ py≥3.10 印 `options:` —— 见未覆盖面）。
func dycHelp(stdout io.Writer) {
	fmt.Fprintln(stdout, dycUsage())
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "optional arguments:")
	fmt.Fprintln(stdout, "  -h, --help       show this help message and exit")
	fmt.Fprintln(stdout, "  --root ROOT")
	fmt.Fprintln(stdout, "  --json")
	fmt.Fprintln(stdout, "  --emit-baseline")
	fmt.Fprintln(stdout, "  --self-test")
}

// dycLongOpts —— 本件全部长旗标（供 argparse 唯一前缀缩写解析）。
var dycLongOpts = []string{"--help", "--root", "--json", "--emit-baseline", "--self-test"}

// dycResolveLong —— argparse 规则：精确名优先，否则取唯一前缀；多匹 = 歧义。
func dycResolveLong(name string) (string, bool, bool) {
	for _, o := range dycLongOpts {
		if o == name {
			return o, false, true
		}
	}
	var matches []string
	for _, o := range dycLongOpts {
		if strings.HasPrefix(o, name) {
			matches = append(matches, o)
		}
	}
	if len(matches) == 1 {
		return matches[0], false, true
	}
	if len(matches) > 1 {
		return "", true, false
	}
	return "", false, false
}

var dycNegNumRe = regexp.MustCompile(`^-\d+$|^-\d*\.\d+$`)

func dycLooksOption(s string) bool {
	if !strings.HasPrefix(s, "-") || s == "-" {
		return false
	}
	if dycNegNumRe.MatchString(s) {
		return false
	}
	return true
}

// ── 输出组装 ──────────────────────────────────────────────────────────────
func dycRowsFiles(rows []dycRow) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.File
	}
	return out
}

func dycJSON(rows []dycRow, covered, exempt, readOnly, missing, missingSites []string) string {
	return "{" +
		`"schema": ` + dycJSONStr(dycSchema) + ", " +
		`"reads": ` + dycJSONStrList(dycRowsFiles(rows)) + ", " +
		`"covered": ` + dycJSONStrList(covered) + ", " +
		`"exempt": ` + dycJSONStrList(exempt) + ", " +
		`"read_only": ` + dycJSONStrList(readOnly) + ", " +
		`"missing": ` + dycJSONStrList(missing) + ", " +
		`"missing_sites": ` + dycJSONStrList(missingSites) + "}"
}

// checkDryrunYesConflictCoverageGateMain —— 入口（= py `main(argv)` · argv 不含程序名）。
func checkDryrunYesConflictCoverageGateMain(stdout, stderr io.Writer, argv []string) int {
	root := ""
	jsonOut := false
	emitBaseline := false
	selfTest := false
	var unrecognized []string
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "-h" || a == "--help":
			dycHelp(stdout)
			return 0
		case strings.HasPrefix(a, "-h") && !strings.HasPrefix(a, "--") && len(a) > 2:
			val := a[2:]
			if strings.HasPrefix(val, "=") {
				val = val[1:]
			}
			return dycArgErr(stderr, "argument -h/--help: ignored explicit argument '"+val+"'")
		case a == "--":
			// argparse：`--` 本身也进「未识别」清单，其后全部 token 原样归入未识别（选项不再解析）。
			unrecognized = append(unrecognized, a)
			unrecognized = append(unrecognized, argv[i+1:]...)
			i = len(argv)
		case strings.HasPrefix(a, "--"):
			name, val, hasVal := a, "", false
			if j := strings.Index(a, "="); j >= 0 {
				name, val, hasVal = a[:j], a[j+1:], true
			}
			canon, amb, ok := dycResolveLong(name)
			if amb {
				return dycArgErr(stderr, "ambiguous option: "+name)
			}
			if !ok {
				unrecognized = append(unrecognized, a)
				continue
			}
			switch canon {
			case "--help":
				if hasVal {
					return dycArgErr(stderr, "argument -h/--help: ignored explicit argument '"+val+"'")
				}
				dycHelp(stdout)
				return 0
			case "--root":
				if hasVal {
					root = val
				} else {
					if i+1 >= len(argv) || dycLooksOption(argv[i+1]) {
						return dycArgErr(stderr, "argument --root: expected one argument")
					}
					i++
					root = argv[i]
				}
			case "--json":
				if hasVal {
					return dycArgErr(stderr, "argument --json: ignored explicit argument '"+val+"'")
				}
				jsonOut = true
			case "--emit-baseline":
				if hasVal {
					return dycArgErr(stderr, "argument --emit-baseline: ignored explicit argument '"+val+"'")
				}
				emitBaseline = true
			case "--self-test":
				if hasVal {
					return dycArgErr(stderr, "argument --self-test: ignored explicit argument '"+val+"'")
				}
				selfTest = true
			}
		case strings.HasPrefix(a, "-") && len(a) > 1:
			unrecognized = append(unrecognized, a)
		default:
			unrecognized = append(unrecognized, a)
		}
	}
	if len(unrecognized) > 0 {
		return dycArgErr(stderr, "unrecognized arguments: "+strings.Join(unrecognized, " "))
	}
	if selfTest {
		if dycSelfTest(stdout) {
			return 0
		}
		return 2
	}
	if root == "" {
		// py `a.root or os.getcwd()`（空串也是 falsy ⇒ 回落 cwd）。
		if wd, err := os.Getwd(); err == nil {
			root = wd
		} else {
			root = "."
		}
	}
	rows, errMsg := dycScan(root)
	// ★ B 类真口子修：读失败 ⇒ 响亮 BLOCKED rc=2 + 逐件点名（✗ 不再静默少判）。
	if len(dycReadFails) > 0 {
		fmt.Fprintf(stderr, "BLOCKED —— 有 %d 件读不到（B 类读失败 ⇒ 登记 + 判红 · rc=2 · 不给结论）：\n", len(dycReadFails))
		for _, u := range dycReadFails {
			fmt.Fprintf(stderr, "  [读失败] %s\n", u)
		}
		return 2
	}
	if errMsg != "" {
		fmt.Fprintf(stderr, "%s: %s ⇒ 不给结论（退码 2）\n", dycVersion, errMsg)
		return 2
	}
	if len(rows) == 0 {
		fmt.Fprintf(stderr, "%s: 空域 —— %s 下 0 件读 `"+dycInvDry+"`（取不到输入 ⇒ 空域 ≠ 全绿）⇒ 不给结论（退码 2）\n", dycVersion, dycScanRel)
		return 2
	}
	missing, covered, exempt, readOnly := dycJudge(rows, dycReadOnlyFaces)
	// ★ 调用点粒度档（GAP-20261014-66 的后半）：与件粒度面**取并集** ⇒ 严格不减报（见本节抬头）。
	missingSites := dycSitesOf(rows, dycReadOnlyFaces)
	if emitBaseline {
		fmt.Fprintln(stdout, "{"+`"read_only_faces": `+dycJSONStrList(dycReadOnlyFaces)+"}")
		return 0
	}
	if jsonOut {
		fmt.Fprintln(stdout, dycJSON(rows, covered, exempt, readOnly, missing, missingSites))
		if len(missing) > 0 || len(missingSites) > 0 {
			return 1
		}
		return 0
	}
	fmt.Fprintf(stdout, "%s：应产者 %d 件 · 真产者 %d 件 · 冻结名单 %d 件 · 只读/转发面 %d 件 · 调用点缺 %d 处\n",
		dycVersion, len(rows), len(covered), len(exempt), len(readOnly), len(missingSites))
	if len(missing) > 0 {
		fmt.Fprintln(stdout, "⇒ 报红（rc=1 · 件粒度）：下列件读 `"+dycInvDry+"`、且属**写面**（有 yes 面），却既无共享判定、也不在只读干跑冻结名单 ——")
		fmt.Fprintln(stdout, "   「--dry-run 与 --yes 同给时静默吞 --yes」的病灶面：")
		for _, f := range missing {
			fmt.Fprintf(stdout, "   - core/cmd/zerg/%s\n", f)
		}
		fmt.Fprintln(stdout, "   治法：在**唯一写盘点之前**插 `if rc := dryRun"+"YesConflict(inv, stderr); rc != exitOK { return rc }`。")
	}
	if len(missingSites) > 0 {
		fmt.Fprintln(stdout, "⇒ 报红（rc=1 · 调用点粒度 `件:命令`）：下列**命令块**真读 `"+dycInvDry+"`、且有 yes 面，")
		fmt.Fprintln(stdout, "   但其共享判定**走不到**（排在它自己的干跑计划件早退**之后**、或整块没有判定）——")
		fmt.Fprintln(stdout, "   即「同上两档时静默吞 --yes」的病灶面（件粒度 token 判看不出的那一半）：")
		for _, s := range missingSites {
			fmt.Fprintf(stdout, "   - core/cmd/zerg/%s\n", s)
		}
		fmt.Fprintln(stdout, "   治法：把 `if rc := dryRun"+"YesConflict(inv, stderr); rc != exitOK { return rc }` 插到**该命令自己的干跑早退之前**（命令首句）。")
	}
	if len(missing) > 0 || len(missingSites) > 0 {
		return 1
	}
	fmt.Fprintln(stdout, "⇒ 全绿（rc=0）：应产者全部落在真产者 / 只读干跑冻结名单 / 只读转发面里，")
	fmt.Fprintln(stdout, "   且**每条命令的判定调用点都排在它自己的干跑早退之前**（件粒度与调用点粒度两面同绿）。")
	return 0
}

// ═══════════════════════ ★ 未覆盖面（逐条如实声明）═══════════════════════
//
//	① **py 版本相关措辞**：`-h` 正文 py≤3.9 印 `optional arguments:`、py≥3.10 印 `options:`；本件按
//	   py3.9（本机 `/usr/bin/python3`）复刻 ⇒ 门跑环境若用 py≥3.10 ⇒ 帮助面 stdout 分叉（rc 恒 0）。
//	   未知旗标 / 缺值 / `--json=1` 的 error 正文同为 py 版本相关措辞。usage 续行缩进 = argparse 按
//	   宽度折行得 `len("usage: ")+len(prog)+1`（= 45 空格）。
//	② **非 UTF-8 件**：py `open(..., encoding="utf-8")` 读到非法字节抛未捕 UnicodeDecodeError ⇒ 裸
//	   回溯 rc=1（C 类无兜底）；本件 Go 以字节当串（RE2 按字节匹配）⇒ 不回溯。真仓件件合法 UTF-8
//	   ⇒ 不触发；声明为未覆盖。
//	③ **符号链接 cwd / 扫描目录自身含链接**：见声明 (a)。
//	④ **`--` 之后的 token**：argparse 视 `--` 与其后全部为「未识别」—— 已逐字照搬。
