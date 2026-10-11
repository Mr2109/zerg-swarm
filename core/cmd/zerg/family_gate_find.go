// family_gate_find.go —— `zerg gate find <片段>`：**门件名 ⇒ 接线位置** 的那座桥（缺口 `GAP-20260927-07`）。
//
// 病根（父代理 2026-09-27 现场 · 两条 rc=2 逐字）：
//
//	`zerg gate show check-gate-coverage`    ⇒ 「步骤名里没有匹配 check-gate-coverage 的」
//	`zerg gate explain check-gate-coverage` ⇒ 「步骤名 %q **逐字**找不到（现读脚本里的 add_step 行 182 条）」
//
// 门族两条既有入口的键都是**步骤名**（`add_step` 的第二个引号里那串），而日常手上的名字是**门件名**
// （`scripts/gates/check-*.py`）—— 两者之间**没有一格把桥搭起来** ⇒ 想从门件名问出「这一步叫什么、
// 跑什么命令、什么档位、日志落哪」只能手搓 `grep add_step` ✗。
//
// ★ 补第二半（同夜 · 同一个缺口的另一半）：**接线面不止一条** —— 一件门件「跑得到」在本仓有多处面，
// 只有第一处**按名字点名**：
//
//	① `precommit-gates.sh` 的 `add_step <scope> "<名>" <模式> "<目录>" "<命令>"` —— 命令串点名；
//	② `all.sh` 收集面 `for f in "${GATEDIR}"/*.py "${GATEDIR}"/*.sh` —— **按通配收**：件在即跑得到，
//	   **名字一次都不出现**；★ 现读 **`.py` 与 `.sh` 都收**（`GAP-20260927-65` 补 `.sh` 后），
//	   行号由 `gateAllShFace` **读 all.sh 现算** —— ✗ 不再写死 `@179`（`GAP-20260928-111`：旧写死行号已漂到自检段）；
//	③ `real-gates.sh` —— 一份**名录**（`--only <门名>` 的过滤面），且现盘门面收敛为 `*.py`；
//	   名单外的件**默认档保守不进真跑面**（`skip-unclassified`：副作用未经实测 · `GAP-20260928-129`），
//	   只有 `--only <名>` 显式点名那一支才真跑（**不是**「可跑」✗ —— `GAP-20260928-129`）。
//
// ⇒ 只知道①，会把「门⑫根本没进 precommit（却跑得到）」误读成「这门不存在」✗。故本命令出**两种行**：
//
//	**步骤行**（片段命中步名/命令串）：八格 + 第九格 `wiring` 给出它在各接线面的位置；
//	**门件行**（片段命中 `scripts/gates/` 下的真件名，但没有任何 `add_step` 点名它）：一行说明
//	「在哪跑得到、在哪跑不到」—— 这就是「零命中」与「不存在」的分别 ✓。
//
// 九格与 `gate explain` 的关系（**同源，不另写一份**）：`scope`/`mode`/`verdict`/`log`/`source` 全部
// 从同一份 `add_step` 声明与同一组 helper（`parseAddSteps` / `gateShowModeCell` / `gateModeVerdict` /
// `gateStepLogNote`）读 —— 判据与日志路径在本仓只有**一处**口径（两处各写一份必然漂 · `G1-a`）。
// 第八格 `next` 是**可照抄的下一步**，第九格 `wiring` 是各接线面的**现读**结论（读脚本得的，不是猜的）。
//
// 退码：0 有命中 · 1 零命中 · 2 用法错（缺片段）· 8 不给结论（门禁脚本或 `scripts/gates/` 目录读不到）。
// 全程**只读**：只读脚本与件名，不跑任何步骤、不写任何件。
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// gateFindFields —— `gate find --json` 的闭集九格。前八格与 `gate explain` 同义字段逐字同名
// （机读侧拿这一格就能直接换命令用），第九格 `wiring` 是**三处接线位置**的现读结论。
var gateFindFields = []string{"name", "scope", "mode", "verdict", "cmd", "log", "source", "next", "wiring",
	// ★ 治 GAP-20260928-145：机器可读面加两格 —— 命中计数 `hits` 与唯一性标记 `unique`。
	//   病（现读）：片段命中 4 行仍 rc=0 且 stdout 照给 5 行表（超集）⇒ 只读 stdout 的
	//   消费者拿不到「命中不唯一」这个信号，会把超集读成一条结论。
	//   治法：**只加信息**（这两格由 `gateFindHitFace` 一处算、逐行同值）—— 不改退码（现 rc=0
	//   有现有消费者）、不改前九格的字节与顺序、不改任何既有函数行为。
	"hits", "unique",
	// ★ 治 GAP-20260928-49：机器可读面再加两格 —— 「解析不确定」读数（`uncertain` / `uncertain_lines`）。
	//   病（现读）：不确定性只在**人读面**（stderr 的 `reportUncertainSteps`）自报，`--json` 的
	//   items 里一个字都没有 ⇒ 机读侧（子代理 / 门）仍可能把「已解析条数」当全量、把「读不出」
	//   当「未点名」，同族纪律「读不到不许当健康」在**机器面**上无落点。
	//   治法：**只加信息**（两格由 `gateFindStampUncertain` 一处算、逐行同值）—— 与 `hits`/`unique`
	//   同规：不改退码（现 rc=0 有现有消费者）、不改前十一格的字节与顺序、不改任何既有函数行为。
	"uncertain", "uncertain_lines"}

// cmdGateFind —— `zerg gate find <片段> [--json <字段>]`。
//
// 放在 `cmdGate` 的分支里（与 `results` 同规：**只在命令面自己的旗标上生效**，不改脚本退码）。
// `root`/`script` 由 `cmdGate` 传进来（它已经把仓根与自举件路径核过一遍）。
func cmdGateFind(inv *invocation, stdout, stderr io.Writer, root, script string) int {
	probe := ""
	if len(inv.args) > 0 {
		probe = strings.TrimSpace(inv.args[0])
	}
	if probe == "" {
		inv.setErr("usage", "missing_step_name", "缺片段（门件名或命令串里的一段）")
		fmt.Fprintf(stderr, "%s: `gate find` 要给片段（例：%s gate find check-gate-coverage）\n", progName, progName)
		fmt.Fprintf(stderr, "口径：片段在**步骤名 + 命令串 + scripts/gates/ 下的件名**上做子串匹配（英文大小写不敏感）\n")
		fmt.Fprintf(stderr, "      拿到步名后再 `%s gate show '<步名>' --json`（命令串/档位/判据/日志路径）\n", progName)
		return exitUsage
	}

	b, err := os.ReadFile(script)
	if err != nil {
		inv.setErr("blocked", "gate_script_unreadable", "门禁脚本读不到")
		fmt.Fprintf(stderr, "%s: 读不到门禁脚本 %s（%v）⇒ 不给结论（退码 8）\n", progName, gateScriptRel, err)
		return exitBlocked
	}
	src := string(b)
	decls, unc := parseAddStepsStrict(src)
	reportUncertainSteps(stderr, unc)
	needle := strings.ToLower(probe)
	rows := []map[string]string{}

	// ① 步骤行：片段命中**步名**或**命令串**（粘成一段再 contains：门件名在命令串里，
	//    人给的词可能是步名里的词，两处都得能中）。
	for _, d := range decls {
		if !strings.Contains(strings.ToLower(d.Name+"\n"+d.Cmd), needle) {
			continue
		}
		logNote, _ := gateStepLogNote(root, d.Name)
		rows = append(rows, map[string]string{
			"name":    d.Name,
			"scope":   d.Scope,
			"mode":    gateShowModeCell(d.Mode),
			"verdict": gateModeVerdict(d.Mode),
			"cmd":     d.Cmd,
			"log":     logNote,
			"source":  fmt.Sprintf("%s:%d", gateScriptRel, d.Line),
			"next":    fmt.Sprintf("%s gate show '%s' --json", progName, d.Name),
			"wiring":  gateWiringNote(root, d.Cmd, decls, unc),
		})
	}

	// ★ 门件面先自查：`scripts/gates/` **读不到 ⇒ 不给结论（退码 8）**，与「零命中」分家。
	// 读不到时「件名面」这一格本来就没有读数 —— 再往下判就是拿半张表当全表（`GAP-20260927-221`）。
	// 体例逐字照抄同族同形的两处（`family_build.go:47-52` 的 `bin/` 读不到 · `family_approve.go:211-215`）：
	// 不许自创退码/文案 —— 「读不到」在本仓只有一条出口（退码表 8 = blocked「读不到不许当健康」）。
	gateFiles, gerr := gateFilesMatching(root, probe)
	if gerr != nil {
		inv.setErr("blocked", "gate_dir_unreadable", "scripts/gates/ 读不到")
		fmt.Fprintf(stderr, "%s: `scripts/gates/` 读不到（%v）⇒ 不给结论（退码 8）\n", progName, gerr)
		return exitBlocked
	}

	// ② 门件行：片段命中 `scripts/gates/` 下的**真件名**。这一行的理由：**「零命中」与「不存在」
	//    是两件事** —— 门⑫（`check-doc-cmds.py`）跑得到，但 `add_step` 里一个字都没有 ⇒ 只回
	//    「零命中」会把人引到「这门不存在」的错结论上 ✗（这正是补第二半的直接起因）。
	//
	// ★ 本批修的第三处：这一行的 `verdict`/`source` 两格旧形态是**硬编码文案「未点名」** ——
	//   件名一旦真被点名（如 `check-placeholder-residue.py` @2866），同一行就会出现
	//   「`verdict` 说未点名 ｜ `wiring` 说点名@2866」的**自相矛盾**。现改成**由接线结果现算**：
	//   点名了 ⇒ 档位判据取那一步的 `mode`、出处取那一行；没点名 ⇒ 才是「未点名」。
	for _, rel := range gateFiles {
		if d, ok := gateFileDecl(rel, decls); ok {
			mode := gateShowModeCell(d.Mode)
			logNote, _ := gateStepLogNote(root, d.Name)
			rows = append(rows, map[string]string{
				"name":    rel,
				"scope":   d.Scope,
				"mode":    mode,
				"verdict": fmt.Sprintf("（本件由步骤 %q 点名）%s", d.Name, gateModeVerdict(d.Mode)),
				"cmd":     d.Cmd,
				"log":     logNote,
				"source":  fmt.Sprintf("%s:%d（命令串点名）", gateScriptRel, d.Line),
				"next":    fmt.Sprintf("%s gate show '%s' --json", progName, d.Name),
				"wiring":  gateWiringNote(root, rel, decls, unc),
			})
			continue
		}
		rows = append(rows, map[string]string{
			"name":    rel,
			"scope":   "—",
			"mode":    "—",
			"verdict": gateFindUnnamedVerdict(rel, unc),
			"cmd":     "—",
			"log":     "—",
			"source":  gateFindUnnamedSource(rel, unc),
			"next":    fmt.Sprintf("%s code find '%s'", progName, filepath.Base(rel)),
			"wiring":  gateWiringNote(root, rel, decls, unc),
		})
	}

	if len(rows) == 0 {
		// ★ 零命中的口径与 `zerg code find` **逐字一致**：这不是错，是「没有」（退码 1）。
		// 所以这里**不** setErr（error 面只在真出错时出现）—— 只把「怎么找得着」印给人。
		fmt.Fprintf(stderr, "%s: 零命中 —— 步骤名、命令串与 scripts/gates/ 件名里都没有 %q（现读 add_step 行 %d 条 · 退码 1）\n",
			progName, probe, len(decls))
		fmt.Fprintf(stderr, "口径：零命中不是错，是「没有」；要看全表 `%s gate ls`，要按件名找 `%s find <名字片段>`\n",
			progName, progName)
		for _, s := range nearestSteps(decls, probe, 3) {
			fmt.Fprintf(stderr, "  像它的是：%s\n", s)
		}
		return exitFail
	}

	if len(inv.fields) == 0 {
		inv.fields = gateFindFields
	}
	// ★ 2026-09-28（缺口 `GAP-20260928-145`）：机器可读面逐行盖「命中计数 + 唯一性标记」两格。
	//   口径只有一处（`gateFindHitFace`）—— 表/JSON 两条路都走 `listCmd`，故两处**同时**有这两格。
	gateFindStampHits(rows)
	// ★ 治 GAP-20260928-49：把「解析不确定」读数也盖进机器面（与 hits/unique 同规 · 逐行同值）。
	gateFindStampUncertain(rows, unc)
	fmt.Fprintf(stderr, "%s: 片段 %q 命中 %d 行（现读 add_step 行 %d 条 · 接线面 = precommit / all.sh / real-gates.sh / 发布面（按需 · GAP-20260928-29））\n",
		progName, probe, len(rows), len(decls))
	if _, u := gateFindHitFace(len(rows)); u == gateFindUniqueNo {
		// 「命中不唯一」的那一档：机器面字段已给（hits/unique），这里再把**人面**也说清 ——
		// 只读 stdout 的消费者拿到的表是**超集**，不能读成一条结论。
		fmt.Fprintf(stderr, "%s: **命中不唯一**（%d 行）—— 机器面 `hits=%d · unique=%s`：读 stdout 的消费者别把这 %d 行读成一条结论\n",
			progName, len(rows), len(rows), gateFindUniqueNo, len(rows))
	}
	// 表/JSON 两条路都走 `listCmd`（与 `gate explain` 同一入口）⇒ 默认面与机器面**不各写一份**。
	return listCmd(inv, stdout, stderr, gateFindFields, rows)
}

// ── 「命中唯一 / 命中多行」在机器可读面上的**唯一一处**定义（缺口 `GAP-20260928-145`）──
//
// 病（父代理 2026-09-28 现读）：`gate find check-glossary` 片段命中 4 行仍 `rc=0`、stdout 照给
// 5 行表（**超集**）⇒ 只读 stdout 的消费者手上没有「命中不唯一」这个机器可辨信号，会把
// 「多行的并集」读成一条结论（假绿）。
//
// 治法：**只加信息、不改退码**（现 `rc=0` 有现有消费者 —— 改码会破它们）。
// 两格由本处一处算：`hits` = 命中行数（十进制串）· `unique` = `yes`/`no`（机器面只认这两个字，
// 不做近似匹配）。表路（TSV 末两列）与 `--json` 路都走 `listCmd` ⇒ 两处**同时**有这两格，
// 口径不另写第二份。
const gateFindUniqueNo = "no"

// gateFindHitFace —— 命中行数 ⇒ （`hits`, `unique`）两格的**唯一**算法。
func gateFindHitFace(n int) (string, string) {
	if n == 1 {
		return "1", "yes"
	}
	return fmt.Sprintf("%d", n), gateFindUniqueNo
}

// gateFindStampHits —— 把这两格盖到每一行上（逐行同值：它们是**这一发**的读数，不是某行的属性）。
func gateFindStampHits(rows []map[string]string) {
	hits, unique := gateFindHitFace(len(rows))
	for _, r := range rows {
		r["hits"] = hits
		r["unique"] = unique
	}
}

// gateFindStampUncertain —— 「解析不确定」读数在机器面上的**唯一**盖法（`GAP-20260928-49`）。
//
// 病（现读）：读不出的 `add_step` 声明只经 `reportUncertainSteps` 报到**进程 stderr**（人读面），
// `gate find --json` 的 `items` 里没有对应格 ⇒ 机读侧（子代理 / 门）拿到的 items 看不出
// 「还有 N 条声明没读出来」，会把「已解析条数」当全量、把「读不出」当「未点名」。
// 同族纪律「读不到不许当健康」在**机器面**上没有落点。
//
// 治法（**只加信息** · 与 `hits`/`unique` 同规）：逐行同值盖两格 ——
//
//	`uncertain`       = `no`（无读不出的声明行）/ `yes`（有）
//	`uncertain_lines` = 空串 / 行号清单（形如 `L1958 · L2973`）
//
// 退码、前十一格的字节与顺序、任何既有函数行为**一字不动**。
func gateFindStampUncertain(rows []map[string]string, unc []gateStepUncertain) {
	val, lines := "no", ""
	if len(unc) > 0 {
		val = "yes"
		ls := make([]int, 0, len(unc))
		for _, u := range unc {
			ls = append(ls, u.Line)
		}
		lines = joinLines(ls)
	}
	for _, r := range rows {
		r["uncertain"] = val
		r["uncertain_lines"] = lines
	}
}

// gateFilesMatching —— `scripts/gates/` 下件名含片段的真件（仓根相对路径 · 排序后返回）。
// **只列件名，不读件内容**：本格要回答「这门叫什么、挂在哪」，不是「它判什么」。
//
// ★ 目录**读不到**时把 err **交给调用方**（`GAP-20260927-221`）：旧形态在这里 `return nil`，
// 于是「目录读不到」与「真的一件都没有」在退码与输出上完全同形（都是「零命中」退码 1）⇒
// 取证面拿到的「零命中」可能是假的。本格不吞错，退码归调用方那一条「读不到 ⇒ 8」出口。
func gateFilesMatching(root, probe string) ([]string, error) {
	ents, err := os.ReadDir(filepath.Join(root, "scripts", "gates"))
	if err != nil {
		return nil, err
	}
	needle := strings.ToLower(probe)
	// ★ 治 GAP-20260928-150：门件面原只用**整串** probe 做子串 ⇒ 传**仓内相对路径**
	//   （scripts/gates/x.py）恒零命中，而传**基名**（x.py）才命中 ⇒ 同一件因入参形态
	//   而在/不在可见面（假阴）。现并上**基名**那一档（filepath.Base），与步名面/门件面同尺。
	baseNeedle := strings.ToLower(filepath.Base(filepath.FromSlash(probe)))
	out := []string{}
	for _, e := range ents {
		n := strings.ToLower(e.Name())
		if e.IsDir() || (!strings.Contains(n, needle) && !strings.Contains(n, baseNeedle)) {
			continue
		}
		out = append(out, filepath.ToSlash(filepath.Join("scripts", "gates", e.Name())))
	}
	sort.Strings(out)
	return out, nil
}

// gateFileDecl —— 该门件在本脚本的 `add_step` 声明里**第一个**点名它的一步（没有 ⇒ ok=false）。
//
// 口径：按命令串的**基名子串**找（件名在命令串里就是 `scripts/gates/x.py` 形态，基名足够）；
// 声明按源码行序 ⇒ 「第一个」是**源码里最先**点名它的那一步（不是运行期步序 —— 运行期步序是
// `gate show --json` 的 `log` 格/`gate ls` 的事，这里不混）。
func gateFileDecl(rel string, decls []gateStepDecl) (gateStepDecl, bool) {
	base := filepath.Base(rel)
	for _, d := range decls {
		if strings.Contains(d.Cmd, base) {
			return d, true
		}
	}
	return gateStepDecl{}, false
}

// gateFindUnnamedVerdict / gateFindUnnamedSource —— 门件行「未点名」两格的**现算**口径。
//
// ★ 与旧的硬编码文案的差别只有一处、但很关键：当件名出现在某条**解析不确定**的声明行原文里时，
// 「未点名」这三个字**不成立**（那条声明读不出来 ⇒ 不知它点没点这个件）。此时两格改成
// 「**解析不确定**」并点名行号 —— 与同族「读不到不许当健康」同口径（宁可说不知道，不假装扫过）。
func gateFindUnnamedVerdict(rel string, unc []gateStepUncertain) string {
	if ls := uncertainLinesNaming(rel, unc); len(ls) > 0 {
		return fmt.Sprintf("— **解析不确定**：该件名出现在读不出的声明行 %s 原文里 ⇒ 点没点名**判不了**（不当「未点名」）",
			joinLines(ls))
	}
	return "—（真件在，但 `add_step` 步骤表里未点名 —— 判据看 `wiring` 格的接线面）"
}

func gateFindUnnamedSource(rel string, unc []gateStepUncertain) string {
	if ls := uncertainLinesNaming(rel, unc); len(ls) > 0 {
		return fmt.Sprintf("%s:%s（**解析不确定** —— 行读不出来，出处存疑）", gateScriptRel, joinLines(ls))
	}
	return "scripts/gates/ 下的真件（命令串面：`add_step` 未点名）"
}

// uncertainLinesNaming —— 读不出的声明行里**原文含该件基名**的行号（用于把「未点名」降级成「判不了」）。
func uncertainLinesNaming(rel string, unc []gateStepUncertain) []int {
	base := filepath.Base(rel)
	out := []int{}
	for _, u := range unc {
		if strings.Contains(u.Raw, base) {
			out = append(out, u.Line)
		}
	}
	return out
}

func joinLines(ls []int) string {
	parts := []string{}
	for _, l := range ls {
		parts = append(parts, fmt.Sprintf("L%d", l))
	}
	return strings.Join(parts, " · ")
}

// gateWhatFileName —— 从 `what`（**门件路径**或**步骤命令串**）里取**门件文件名**（`GAP-20260928-113`）。
//
// 病根：旧形态对整串取 `filepath.Base`。门件行传的是件路径（`scripts/gates/x.py`）⇒ 没问题；但
// **步骤行**传的是**带实参的命令串**（`python3 scripts/gates/x.py --scope devdocs --missing=fail`）
// ⇒ 基名成 `x.py --scope devdocs --missing=fail` ⇒ 后缀判 `.py` **不成立** ⇒ 落进非目标支、
// 被**误印成「该面不收」** ✗，且 precommit 点名面与 real-gates 名录面按基名找也**全部落空**。
// 现按**空白切词**认那一概含 `gates/` 且以 `.py`/`.sh` 收尾的词（本命令的接线面都住在
// `scripts/gates/`）⇒ 取它的 `filepath.Base`。
//
// **范围**：只认**门件目录**里那一概 —— 不含 `gates/` 的形态（如 `scripts/docs/gen-x.py --check`、
// `scripts/*.sh` 通配）**照旧**落回整串 `filepath.Base`，与改前**逐字同**（两态判据②）。
func gateWhatFileName(what string) string {
	for _, f := range strings.Fields(what) {
		t := strings.Trim(f, "\"'`")
		if !strings.Contains(t, "gates/") {
			continue
		}
		if strings.HasSuffix(t, ".py") || strings.HasSuffix(t, ".sh") {
			return filepath.Base(t)
		}
	}
	return filepath.Base(what)
}

// gateFaceNotApplicable —— 「**面态＝不适用**」那句的**唯一**定义处（`GAP-20260928-102` ·
// `GAP-20260928-129` 复用同一处，不另写第二份文案）。
//
// 用在：某件在**某个接线面**上因该面的收集条件（`*.py` 通配）而**恒不出现**时 —— 要把它印成
// 「**该面对它不适用**」，不是「这件没接线」✗。`ext` 传该件的后缀（如 `.sh`）。
func gateFaceNotApplicable(face, ext string) string {
	return fmt.Sprintf(" · **面态＝不适用**：%s 面：对 %s 件不生效", face, ext)
}

// gateRealGatesUnlisted —— real-gates 面「**名单外件**」那句的**唯一**定义处（`GAP-20260928-129`）。
//
// 现读真源（`scripts/gates/real-gates.sh`）：默认档选面走 `else` 支，名录外件（既不在 `FAST_NAMES`
// 也不在 `SLOW_NAMES`、且不在白名单）**一律** `add "$n" skip-unclassified`，汇总里该态的 reason
// 逐字是「unclassified: 名单外新件（副作用未经实测 ⇒ 保守不进真跑面，请登记后并入名单）」
// ⇒ **真实动作＝保守跳过，不是「可跑」**。只有 `--only <名>` 那一支**不查名录**、照跑
// （同件的 `GAP-20260928-112`）。
//
// 旧文案（`GAP-20260928-129` 正文引的那句）把「保守跳过」印成**可跑**＝伪结论 ✗；
// 本函数只改**说法**：选面逻辑、退码、名录筛选一字未动。
func gateRealGatesUnlisted(base string) string {
	return "real-gates: **名单外** —— 默认档口径＝`skip-unclassified`（副作用未经实测）" +
		"⇒ **保守不进真跑面**；只有 `--only " + base + "` 显式点名那一支才真跑"
}

// gateAllShFace —— all.sh 收集面的**现读**（缺口 `GAP-20260928-111` / `GAP-20260928-50`）：
//
//	读 `scripts/gates/all.sh` 找那只 `for f in "${GATEDIR}"/*…` 收集循环，取它的**行号**与是否含 `*.sh`。
//	★ 旧形态把收集面**写死成 `@179`** ⇒ 行号一漂就指向自检段；且旧文硬说「只收 `.py`」
//	而现读已含 `.sh` ⇒ `.sh` 门被印成「该面不收」＝**伪结论**。两处**同源现算**：行号与后缀面都出自这一处
//	（两类行的结论不再相反 ✗）。读不到 ⇒ ok=false（调用方印「判不了」，不当一切正常）。
func gateAllShFace(root string) (line int, collectsSh bool, ok bool) {
	b, err := os.ReadFile(filepath.Join(root, "scripts", "gates", "all.sh"))
	if err != nil {
		return 0, false, false
	}
	for i, ln := range strings.Split(string(b), "\n") {
		if strings.Contains(ln, "for f in") && strings.Contains(ln, `"${GATEDIR}"/*`) {
			return i + 1, strings.Contains(ln, "*.sh"), true
		}
	}
	return 0, false, false
}

// gateWiringNote —— 一件门件在**各接线面**的现读结论（同夜补的第二半）：
//
//	★ GAP-20260928-29：第四处面 = **发布面**（scripts/build/publish-preflight.sh 闸④）——门件被它真命令位调用时**按需**补印；
//
//	precommit：`add_step` 的命令串里点到它的那几步（步名 + 出处行）；
//	all.sh  ：收集面 = `for f in "${GATEDIR}"/*.py "${GATEDIR}"/*.sh`（行号由 `gateAllShFace` **现算**）
//	          —— **`.py` 与 `.sh` 都按通配收**（`GAP-20260927-65` 补 `.sh` 后）；
//	          ★ `GAP-20260928-50`：旧文案对 `.sh` 一律印「该面不收」而现读已收 ⇒ 两类行相反；现按同一处现读判；
//	          ★ 后缀判按**门件文件名**（`gateWhatFileName`：从命令串里认那一概含 `gates/` 的件）
//	          —— `GAP-20260928-113`：旧形态对整串取 `filepath.Base` ⇒ 步骤行的**带实参命令串**
//	          （`…/check-x.py --scope devdocs`）后缀判不成立 ⇒ `.py` 门件被**误印成「该面不收」** ✗；
//	real-gates：名录里点到它的首行；名单外 ⇒ `GAP-20260928-129`：印**默认档口径＝保守不进真跑面**
//	          （`skip-unclassified`），非 `.py` 件再加一句**面态＝不适用**（该面收集面只收 `*.py`）——
//	          旧文案（`GAP-20260928-129` 正文引的那句）把「保守跳过」印成「可跑」＝**伪结论** ✗。
//
// `what` 既可能是**命令串**（步骤行）也可能是**件路径**（门件行）⇒ 两种都按基名子串找，宁多勿漏。
func gateWiringNote(root, what string, decls []gateStepDecl, unc []gateStepUncertain) string {
	base := gateWhatFileName(what)
	parts := []string{}

	named := []string{}
	for _, d := range decls {
		if strings.Contains(d.Cmd, base) {
			named = append(named, fmt.Sprintf("%s@%d", d.Name, d.Line))
		}
	}
	if len(named) > 0 {
		parts = append(parts, "precommit: 点名 "+strings.Join(named, " · "))
	} else if ls := uncertainLinesNaming(base, unc); len(ls) > 0 {
		// ★ 读不出的声明行原文里点到过它 ⇒ **不许报「未点名」**（那是拿读不到当「没有」）。
		parts = append(parts, fmt.Sprintf(
			"precommit: **解析不确定**（读不出的声明行 %s 原文里点到过它 ⇒ 点没点名判不了，别当「不在步骤表里」）",
			joinLines(ls)))
	} else {
		parts = append(parts, "precommit: 未点名（不在步骤表里）")
	}

	// ★ GAP-20260928-111：行号**不写死** —— 读 all.sh 现算收集面那一行（旧写死 `@179` 已漂到自检段）。
	// ★ GAP-20260928-50：`*.py`/`*.sh` **同源现读** —— 旧形态把 `.sh` 一律印成「该面不收」，
	//   而现读收集面已含 `.sh`（`GAP-20260927-65`）⇒ **两类行结论相反**；此处按**同一处现读**判两后缀。
	allshLine, allshSh, allshOK := gateAllShFace(root)
	exts := "`*.py`"
	if allshSh {
		exts = "`*.py`+`*.sh`"
	}
	if allshOK && (strings.HasSuffix(base, ".py") || (strings.HasSuffix(base, ".sh") && allshSh)) {
		parts = append(parts, fmt.Sprintf("all.sh: 按通配收（%s @%d）", exts, allshLine))
	} else if allshOK {
		parts = append(parts, fmt.Sprintf("all.sh: **不收**（`all.sh:%d` 只收 %s ⇒ 非目标后缀件在这一面不可见）", allshLine, exts))
	} else {
		parts = append(parts, "all.sh: 读不到该件 ⇒ 判不了")
	}

	if rb, err := os.ReadFile(filepath.Join(root, "scripts", "gates", "real-gates.sh")); err == nil {
		// ★ 治 GAP-20260928-149：旧形态对**整件原文**做裸子串扫 ⇒ 件头注释里出现的自己的名字
		//   （如 real-gates.sh 第 2 行、precommit-gates.sh 第 10 行）被读成「名录点名」＝**假报**。
		//   现只认**真名录**：SH_TABLE 清册块（heredoc 里 名<TAB>class<TAB>理由 形态）的条目行。
		if rline, _, rok := gateRealGatesRoster(string(rb), base); rok {
			parts = append(parts, fmt.Sprintf("real-gates: 名录点名@%d", rline))
		} else if strings.HasSuffix(base, ".sh") {
			// ★ 治法（`GAP-20260928-129`）：名录外的 `.sh` 门件在这一面**两处都不是**旧文案说的那样 ——
			//   ① 它是名录外件 ⇒ **默认档口径＝`skip-unclassified` 保守不进真跑面**（不是「可跑」）；
			//   ② 该面的**收集面**是现盘 `*.py`（`LIVE`，现读）⇒ 它在那一面的逐只表里**根本不出现**
			//   ⇒ 面态＝**不适用**（与 `all.sh` 面同形 · `GAP-20260928-102` 那句**复用同一处定义**）。
			parts = append(parts, gateRealGatesUnlisted(base)+
				gateFaceNotApplicable("real-gates", ".sh")+
				"（该面的收集面只收 `*.py`（现盘 `LIVE`）⇒ `.sh` 门在那一面的逐只表里**根本不出现** · 现读 `real-gates.sh`）"+
				"—— 「该面不含它」≠「这件没接线」")
		} else {
			parts = append(parts, gateRealGatesUnlisted(base))
		}
	} else {
		parts = append(parts, "real-gates: 读不到该件 ⇒ 判不了")
	}

	// ★ GAP-20260928-29：第四处接线面 = **发布面**（scripts/build/publish-preflight.sh 的闸④）。
	//   发布面闸（如 check-public-tree-hazards.py）按设计不在 precommit-gates.sh 的 add_step 里，
	//   此前 gate find 只印前两面、不指向这条真家 ⇒ 发布面闸「无正门读数口」。
	//   现读该件：点名 ⇒ 印出处行；未点名 ⇒ 不加这一面（不给其余二百多件添噪声）。
	if pb, err := os.ReadFile(filepath.Join(root, "scripts", "build", "publish-preflight.sh")); err == nil {
		line := 0
		for i, ln := range strings.Split(string(pb), "\n") {
			if strings.Contains(ln, base) {
				line = i + 1
				break
			}
		}
		if line > 0 {
			parts = append(parts, fmt.Sprintf("发布面: 点名@%d（scripts/build/publish-preflight.sh 闸④ —— 发布面闸的真家）", line))
		}
	}

	return strings.Join(parts, " ｜ ")
}

// gateRealGatesRoster —— real-gates 面**真名录**的现读（缺口 `GAP-20260928-149`）。
//
// 病（父代理 2026-10-12 现读 · 两条逐字读数）：旧形态对 `real-gates.sh` **整件原文**逐行做裸子串
// Contains(ln, base) 并取**首个命中行**当「名录点名」⇒ 件头注释里出现的自己的名字被读成名录条目：
//
//	zerg gate find real-gates.sh      ⇒ 「名录点名@2」（第 2 行是它自己的件头注释）
//	zerg gate find precommit-gates.sh ⇒ 「名录点名@10」（第 10 行是件头里那句「一字未动 all.sh /
//	                                        precommit-gates.sh」；真条目在 SH_TABLE@208）
//
// 治法（**只改读数、不新造语义**）：只认 SH_TABLE 清册块里的**条目行**（heredoc 内
// 名<TAB>class<TAB>理由），返回它的**真行号**；块外的任何提到（注释/散文）**不再**算「点名」。
// .py 面本无名录（FAST_NAMES/SLOW_NAMES 已随 C 类退役离盘 · 2026-10-09）⇒ .py 件恒走
// else 支的「名单外」口径（与现读真源一致）。
func gateRealGatesRoster(src, base string) (line int, class string, ok bool) {
	inTable := false
	for i, ln := range strings.Split(src, "\n") {
		t := strings.TrimSpace(ln)
		if strings.HasPrefix(t, "SH_TABLE=") {
			inTable = true
			continue
		}
		if !inTable {
			continue
		}
		if t == "EOF" {
			return 0, "", false
		}
		cols := strings.SplitN(ln, "\t", 3)
		if len(cols) >= 2 && cols[0] == base {
			return i + 1, cols[1], true
		}
	}
	return 0, "", false
}
