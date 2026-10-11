// check_help_shape_test.go —— 判据件 Go 真搬的**成对负控测试件**。
//
// 为什么有这件（两件事都要钉住）：
//
//	① **负控有牙**：正控（夹具原样 ⇒ 绿）与负控（段缺 / 条数漂 / 值冲突 / 链分家 / 树多一族 ⇒ 各自判红）
//	   成对跑；断言打在**退码**、**判词子串**（点名到具体那条腿）与**禁响子串**（证明是这条腿在响、
//	   不是别的腿顺手一起红）上 ⇒ 哑化任一处构造 ⇒ 断言当场 FAIL，带行号。
//	② **三处 golden 逐字钉**：`--self-test` / `--list-rules` / `-h` 三档的**逐字 stdout**
//	   （golden 现读自对拍台 · ✗ 非手抄）⇒ 谁改标语/移位/改判词，这里先红。
//
// 覆盖与不覆盖：本件**不**依赖真制品路径（端到端那格用 t.TempDir() 里的 stub 壳脚本）。
// 真制品面的两环境逐字对拍在落件报告里另出（见回执的 26 格对拍表），不在此硬编跑测机路径。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// chsRunT —— 入口薄壳（argv 与 py `sys.argv[1:]` 同形）。
func chsRunT(argv ...string) (string, string, int) {
	var out, errb bytes.Buffer
	rc := checkHelpShapeGateMain(&out, &errb, argv)
	return out.String(), errb.String(), rc
}

// ── golden（现读自对拍台 · ✗ 非手抄）──
const chsSelfTestGolden = "── 自检（合成夹具 · 不碰真目标）──\n  ✓ 正控：原样 ⇒ 必绿（期望绿 · 实得绿）\n  ✓ 负控⑤：改一条的 `--confirm` 目标（等长换字 · 字节不变）⇒ 必红（期望红 · 实得红）\n      ↳ S4d 旗标串 ⟷ 真实用法串：新增 1 处脱钩（印了 `--confirm=<目标>`，但该命令在 `help --all` 的用法串里没有这个目标的核心名）—— model start\n  ✓ 负控⑥：表尾计数对不上（等长改数）⇒ 必红（期望红 · 实得红）\n      ↳ S4a 表尾自洽：上表 6 条 ≠ 已开放 1 + 未开放 4（= 5）\n      ↳ S4a 表尾 ⟷ 逐条：表尾称 6 条 ≠ 逐条行数 5（两处不许各说各话）\n      ↳ S4e 常量：`help dangerous` 表尾逐字 '上表 6 条：**已开放 1 条**（确认档齐就真执行 —— 逐条见上行标记）；**未开放 4 条**（真跑一律**拒执**、退码 2 = 不给结论，只有 `--dry-run` 出计划件 = 退码 0）。' ≠ 冻结读数 '上表 5 条：**已开放 1 条**（确认档齐就真执行 —— 逐条见上行标记）；**未开放 4 条**（真跑一律**拒执**、退码 2 = 不给结论，只有 `--dry-run` 出计划件 = 退码 0）。'\n  ✓ 负控⑦：删掉表尾一行 ⇒ 必红（期望红 · 实得红）\n      ↳ S4a 表尾：`help dangerous` 末非空行不是表尾形状（`上表 N 条：**已开放 X 条**…**未开放 Y 条**`）—— 现读 '  zerg core restart  D3  --dry-run · --confirm=<主机名> · --yes  它会动什么  [出处]  ← **已'（表尾被删 / 改字 / 换口径？）\n      ↳ S4e 常量：`help dangerous` lines 5 ≠ 冻结读数 7（该面漂了 —— 取法见件头 `BASELINE` 注释）\n      ↳ S4e 常量：`help dangerous` bytes 606 ≠ 冻结读数 825（该面漂了 —— 取法见件头 `BASELINE` 注释）\n  ✓ 负控⑧：抹掉一条的 `--yes`（旗标串破形）⇒ 必红（期望红 · 实得红）\n      ↳ S4c 逐条行形状：第 5 条不符（旗标串只许 `--dry-run · [--confirm=<目标> · ]--yes` · 档位只许 D2/D3）—— '  zerg core restart  D3  --confirm=<主机名> --yes  它会动什么  [出处]  ← **已开放**（确认档齐就真执行）'\n      ↳ S4e 常量：`help dangerous` bytes 809 ≠ 冻结读数 825（该面漂了 —— 取法见件头 `BASELINE` 注释）\n  ✓ 负控⑨：两面同印 `--confirm=` 但值不等（等长换字 · 名字仍在用法串里 ⇒ S4d/字节常量都看不见）⇒ S4f 必红（期望红 · 实得红）\n      ↳ S4f 两面同印 `--confirm=` ⟷ 值须逐字相等：新增 1 处**值冲突**（危险面印的名字 ≠ 用法串里印的名字 ⇒ 用户照哪面敲都可能被拒）—— core restart（危险面 `机器名` ≠ 用法串 `主机名`）\n  ✓ 负控⑩：危险面值挪出用法串（名字集合变化 = S4d 的账）⇒ 旧 S4d 仍必红、S4f 不许抢账（期望红 · 实得红）\n      ↳ S4d 旗标串 ⟷ 真实用法串：新增 1 处脱钩（印了 `--confirm=<目标>`，但该命令在 `help --all` 的用法串里没有这个目标的核心名）—— agent load\n  ✓ 对界：夹具正控上 S4f 不许误报（两面同印的值逐字相等 · 现读 1 条）⇒ 绿且无 S4f 断言（期望绿 · 实得绿）\n  ✓ 正控②：配置面原样（链 ⟷ 逐级说明同序 · 与裸 `help` 同源）⇒ 必绿且不许有 S5 断言（期望绿 · 实得绿）\n  ✓ 负控⑪：只改链行一个字（`内置默认`→`内置缺省` · **等长换字** ⇒ 字节常量看不见）⇒ S5a/S5b 必红、S5c 不许响（期望红 · 实得红）\n      ↳ S5a 同面交叉：优先级链 5 级（旗标 · ZERG_* 环境变量 · 项目 .env · ~/.zerg/config.yaml · 内置缺省）≠ 逐级说明 5 条（旗标 · ZERG_* 环境变量 · 项目 .env · ~/.zerg/config.yaml · 内置默认）—— 同一个面上的两处必须同序逐字相等\n      ↳ S5b 同源跨面：`help config` 的链行 '  旗标 > ZERG_* 环境变量 > 项目 .env > ~/.zerg/config.yaml > 内置缺省' ≠ 裸 `help` 的链行 '  旗标 > ZERG_* 环境变量 > 项目 .env > ~/.zerg/config.yaml > 内置默认'（两处同源 = `helptext.go` 的 `configPriorityLine` 常量 ⇒ 字面必须逐字相等；分家 = 有人把链抄成了第二份）\n  ✓ 负控⑫：只改**裸 `help`**那条同源链行（配置面一字不动）⇒ S5b 必红、S5a 不许抢账（期望红 · 实得红）\n      ↳ S5b 同源跨面：`help config` 的链行 '  旗标 > ZERG_* 环境变量 > 项目 .env > ~/.zerg/config.yaml > 内置默认' ≠ 裸 `help` 的链行 '  旗标 > ZERG_* 环境变量 > 项目 .env > ~/.zerg/config.yaml > 内置缺省'（两处同源 = `helptext.go` 的 `configPriorityLine` 常量 ⇒ 字面必须逐字相等；分家 = 有人把链抄成了第二份）\n  ✓ 负控⑬：配置面掉一条逐级说明 ⇒ S5a 必红（编号连号 ⟷ 链级数）（期望红 · 实得红）\n      ↳ S5a 逐级说明：编号必须是 `1..N` 连号无重复 —— 现读 2 · 3 · 4 · 5\n      ↳ S5a 同面交叉：优先级链 5 级（旗标 · ZERG_* 环境变量 · 项目 .env · ~/.zerg/config.yaml · 内置默认）≠ 逐级说明 4 条（ZERG_* 环境变量 · 项目 .env · ~/.zerg/config.yaml · 内置默认）—— 同一个面上的两处必须同序逐字相等\n      ↳ S5c 常量：`help config` lines 13 ≠ 冻结读数 14（该面漂了 —— 取法见件头 `BASELINE` 注释 · `--emit-baseline` 刷）\n  ✓ 对界：配置面喂**空串** ⇒ 必红（证明「这个面没了」不是静默绿）（期望红 · 实得红）\n      ↳ S5a 优先级链：`help config` 里那条链行现读 0 条（期望恰好 1 条 —— 链被删 / 被拆行 / 被抄出第二份？）\n      ↳ S5c 常量：`help config` lines 0 ≠ 冻结读数 14（该面漂了 —— 取法见件头 `BASELINE` 注释 · `--emit-baseline` 刷）\n      ↳ S5c 常量：`help config` bytes 0 ≠ 冻结读数 486（该面漂了 —— 取法见件头 `BASELINE` 注释 · `--emit-baseline` 刷）\n  ✓ 负控①：删掉族级段 ⇒ 必红（期望红 · 实得红）\n      ↳ S1 族级段：`help --all` 末尾**没有**族级段（段头 `族级面（` 找不到）—— 族名清单消失（这正是本件要防的病）\n      ↳ S2 常量：`help --all` 行数 12 ≠ 冻结读数 16（形状漂了 —— 取法见件头 `BASELINE` 注释）\n      ↳ S2 常量：`help --all` 字节 610 ≠ 冻结读数 780\n  ✓ 负控②a：族级段掉一行 ⇒ 必红（期望红 · 实得红）\n      ↳ S1 族级段：漏了 1 族（前 5：watch）—— 命令树里有、清单里没有\n      ↳ S2 关系式：族级段 2 行 ≠ 2 + ceil(9 族 / 8 族一行) = 3\n      ↳ S2 常量：`help --all` 行数 15 ≠ 冻结读数 16（形状漂了 —— 取法见件头 `BASELINE` 注释）\n  ✓ 负控②b：整面多一行 ⇒ 必红（期望红 · 实得红）\n      ↳ S1 族级段：多了 1 项（前 5：多余一行）—— 清单里有、命令树里没有\n      ↳ S2 关系式：族级段 4 行 ≠ 2 + ceil(9 族 / 8 族一行) = 3\n      ↳ S2 常量：`help --all` 行数 17 ≠ 冻结读数 16（形状漂了 —— 取法见件头 `BASELINE` 注释）\n  ✓ 负控③a：`--all` 末行改写 ⇒ 必红（期望红 · 实得红）\n      ↳ S1 族级段：漏了 1 族（前 5：watch）—— 命令树里有、清单里没有\n      ↳ S1 族级段：多了 3 项（前 5：ui · update · watch-XX）—— 清单里有、命令树里没有\n      ↳ S2 常量：`help --all` 字节 818 ≠ 冻结读数 780\n  ✓ 负控③b：裸 `help` 末行改写 ⇒ 必红（期望红 · 实得红）\n      ↳ S2 常量：`help` 字节 361 ≠ 冻结读数 407\n      ↳ S3 末行：裸 `help` 末行 '凭据：改过的一句。' ≠ 冻结读数 '      非交互用 `--token-stdin`；详见 `zerg help credentials`）。'\n  ✓ 负控④：清单漏一族 ⇒ 必红（期望红 · 实得红）\n      ↳ S1 族级段：漏了 1 族（前 5：watch）—— 命令树里有、清单里没有\n      ↳ S1 族级段：多了 1 项（前 5：watchx）—— 清单里有、命令树里没有\n      ↳ S2 常量：`help --all` 字节 781 ≠ 冻结读数 780\n  ✓ 对界：命令树多一族 ⇒ 必红（期望红 · 实得红）\n      ↳ S1 族级段：段头括注 9 族 ≠ 现算命令树族数 10\n      ↳ S1 族级段：漏了 1 族（前 5：zzz）—— 命令树里有、清单里没有\n      ↳ S3 末行：`help --all` 末行 '  watch' ≠ 由族名清单现算重建的 '  watch · zzz'\n  ✓ 负控⑭：抹掉一条危险档行的**逐条标记**（行还在 · 标记没了）⇒ S4b 必红（期望红 · 实得红）\n      ↳ S4b 逐条标记 ⟷ 表尾：`← **已开放**` 出现 0 次 ≠ 表尾「已开放 1 条」\n      ↳ S4b 逐条标记：逐条行 5 条，带标记的只有 4 条（有行没印 ← 已开放 / ← 本版未开放）\n      ↳ S4e 常量：`help dangerous` bytes 808 ≠ 冻结读数 825（该面漂了 —— 取法见件头 `BASELINE` 注释）\n  ✓ A 档①制品读得出\n  ✓ A 档②制品不在盘 ⇒ 判不了（不许当空面）\n  ✓ A 档③制品零输出 ⇒ 判不了\n  自检 24 格 · 失败 0 格\n"

const chsListRulesGolden = "判据（三条 · 现算 + 对界 · 缺口 GAP-20260927-401）\n-----------------------------------------------\n  S1 族级段（闭集相等）：现跑 `help --all` 末尾段 token ⟷ 命令树（`cli-matrix.json` 的\n     `cases[].command` 首段）· 段头在 · 括注族数 == 现算族数 · 双向差集空 · 裸 `help` 不带段\n  S2 条数与行数：rows(--all) == rows(help) + rows(help dangerous) ·\n     族级段行数 == 1 + ceil(族数/8)（段头之前必须是空行）；两个面的 lines/bytes == 冻结常量\n     （现跑取数 · --emit-baseline 刷）\n  S3 末行：`--all` 末行 == 由族名清单现算重建的那一行；裸 `help` 末行 == 冻结读数（逐字）\n  S4 危险档面（`help dangerous` · 2026-09-27 补 · 缺口 GAP-20260927-418）：\n     S4a 表尾计数（`上表 N 条：已开放 X / 未开放 Y` ⇒ N == X + Y == 逐条行数）·\n     S4b 逐条行尾标记出现次数 == 表尾 X / Y ·\n     S4c 逐条行形状闭集（`zerg <路径>  D2|D3  --dry-run · [--confirm=<目标> · ]--yes  …  ← 标记`）·\n     S4d 逐条 `--confirm=<目标>` 的核心名 ⟷ 该命令在 `help --all` 的用法串（现算脱钩集合 == 冻结名单）·\n     S4f 两面**同印** `--confirm=` 的值须**逐字相等**（现算值冲突集合 == 冻结名单 · 现读空集 —— 独立一条腿：\n         S4d 只判「名字在不在用法串里」⇒ 值冲突但名字集合不变时 S4d 看不见）·\n     S4e 该面的 lines / bytes / 条数 / 表尾逐字 == 冻结读数（现跑取数 · --emit-baseline 刷）\n  S5 配置主题面（`help config` · 2026-09-27 补 · 缺口 GAP-20260927-440）：\n     S5a 同面交叉：优先级链行现算的级名表 == 逐级说明 `N. **名字**` 的名字表（同序逐字 · 两边剥反引号）·\n         编号必须 `1..N` 连号无重复 · 链行恰好 1 条 ·\n     S5b 同源跨面：`help config` 的链行**逐字等于**裸 `help` 的那一行（同源 = `helptext.go` 的\n         `configPriorityLine` 常量 —— 分家 = 有人抄了第二份）·\n     S5c 该面的 lines / bytes == 冻结读数（现跑取数 · --emit-baseline 刷）\n\n不假装覆盖（照实登记）\n----------------------\n  · 只判**形状**（段 / 条数 / 行数 / 末行）；段的**说明文字**逐字、危险档判词、层级列不在本格\n  · 两态 rc 语义（`zerg <族> --help` 退 2 等）**不在本件**（那是 `cli_help_twostate_test.go` 的面，禁碰）\n  · `help config` 只判**形状/结构**：链 ⟷ 逐级说明同序 · 同源链行逐字 · lines/bytes 常量；\n    「凭据」那批条目的**文字**与主题表 `topics.go` 的 `Brief` 串同源不同源**不在本格**\n    （要另开一处读 Go 源的口径 —— 本单不开第三份口径）\n  · 建件时本件**未挂门**（挂门 + `scripts/公开标记.tsv` + `dryrun-semantics.json` 三处登记是随动口）\n"

const chsHelpGolden = "usage: check-help-shape.py [-h] [--repo-root REPO_ROOT] [--bin BIN]\n" +
	"                           [--matrix MATRIX] [--faces-dir FACES_DIR] [--json]\n" +
	"                           [--emit-baseline] [--list-rules] [--self-test]\n" +
	"\n" +
	"`help` 面的形状判据（族级段 / 条数 / 末行）\n" +
	"\n" +
	"optional arguments:\n" +
	"  -h, --help            show this help message and exit\n" +
	"  --repo-root REPO_ROOT\n" +
	"  --bin BIN\n" +
	"  --matrix MATRIX\n" +
	"  --faces-dir FACES_DIR\n" +
	"                        A 档：四个 help 面改「读落盘制品目录」（help-all.txt/help.txt/help-\n" +
	"                        dangerous.txt/help-config.txt · 缺口 GAP-20261001-47）\n" +
	"  --json\n" +
	"  --emit-baseline\n" +
	"  --list-rules\n" +
	"  --self-test\n"

// ── ① 自检 golden + 退码 ───────────────────────────────────────────────
func TestCheckHelpShape_SelfTestGolden(t *testing.T) {
	out, errS, rc := chsRunT("--self-test")
	if rc != 0 {
		t.Fatalf("自检 rc=%d 期望 0（stderr=%q）", rc, errS)
	}
	if out != chsSelfTestGolden {
		t.Fatalf("自检 stdout 不逐字：\n实得 %q\n期望 %q", out, chsSelfTestGolden)
	}
	if !strings.Contains(out, "自检 24 格 · 失败 0 格") {
		t.Fatalf("自检格数不对：%q", out)
	}
}

func TestCheckHelpShape_ListRulesGolden(t *testing.T) {
	out, _, rc := chsRunT("--list-rules")
	if rc != 0 || out != chsListRulesGolden {
		t.Fatalf("--list-rules 不逐字（rc=%d）：\n实得 %q\n期望 %q", rc, out, chsListRulesGolden)
	}
}

func TestCheckHelpShape_HelpGolden(t *testing.T) {
	out, errS, rc := chsRunT("-h")
	if rc != 0 || out != chsHelpGolden || errS != "" {
		t.Fatalf("-h 不逐字（rc=%d stderr=%q）：\n实得 %q", rc, errS, out)
	}
}

// chsTestFixtureBaseline —— 夹具四面的冻结读数（与 py `self_test` 内那份同形）。
func chsTestFixtureBaseline(allT, bareT, dangT, cfgT string) chsBaselineSet {
	dRows := chsDangerRows(dangT)
	return chsBaselineSet{
		"help --all":  {HasLines: true, Lines: len(chsSplitLines(allT)), HasBytes: true, Bytes: len(allT)},
		"help":        {HasLines: true, Lines: len(chsSplitLines(bareT)), HasBytes: true, Bytes: len(bareT), HasLast: true, Last: chsBaseline["help"].Last},
		"help config": {HasLines: true, Lines: len(chsSplitLines(cfgT)), HasBytes: true, Bytes: len(cfgT)},
		"help dangerous": {HasLines: true, Lines: len(chsSplitLines(dangT)), HasBytes: true, Bytes: len(dangT),
			HasRows: true, Rows: len(dRows), HasTail: true, Tail: chsFixTail,
			ConfirmNotInUsage: []string{"agent reap"}, ConfirmValueMismatch: [][]string{}},
	}
}

func chsHasSub(bad []string, sub string) bool {
	for _, b := range bad {
		if strings.Contains(b, sub) {
			return true
		}
	}
	return false
}

// ── ② 成对负控（正控绿 / 各腿红 · 点名子串必须有、禁响子串必须无）────────────
func TestCheckHelpShape_JudgePaired(t *testing.T) {
	allT, bareT, dangT, cfgT := chsFixTexts()
	bl := chsTestFixtureBaseline(allT, bareT, dangT, cfgT)
	ch := func(s string) string { return strings.ReplaceAll(s, "内置默认", "内置缺省") }
	dl := chsSplitLines(dangT)
	cases := []struct {
		name      string
		bad       []string
		wantRed   bool
		wantSub   string
		forbidSub string
	}{
		{"正控_夹具原样_绿", chsJudge(allT, bareT, dangT, chsFixCmds, bl, nil), false, "", "S4f"},
		{"负控_confirm目标等长换字_S4d红", chsJudge(allT, bareT, strings.ReplaceAll(dangT, "--confirm=<模型 id>", "--confirm=<主机 id>"), chsFixCmds, bl, nil), true, "S4d", "S4f"},
		{"负控_表尾计数改数_S4a红", chsJudge(allT, bareT, strings.ReplaceAll(dangT, "上表 5 条", "上表 6 条"), chsFixCmds, bl, nil), true, "S4a", ""},
		{"负控_删表尾一行_S4a红", chsJudge(allT, bareT, strings.Join(dl[:len(dl)-2], "\n")+"\n", chsFixCmds, bl, nil), true, "S4a", ""},
		{"负控_旗标串破形_S4c红", chsJudge(allT, bareT, strings.ReplaceAll(dangT, "--dry-run · --confirm=<主机名> · --yes", "--confirm=<主机名> --yes"), chsFixCmds, bl, nil), true, "S4c", ""},
		{"负控_两面值冲突_S4f红_S4d不许抢账", chsJudge(allT, bareT, strings.ReplaceAll(dangT, "zerg core restart  D3  --dry-run · --confirm=<主机名>", "zerg core restart  D3  --dry-run · --confirm=<机器名>"), chsFixCmds, bl, nil), true, "S4f", "S4d"},
		{"负控_值挪出用法串_S4d红_S4f不许抢账", chsJudge(allT, bareT, strings.ReplaceAll(dangT, "zerg agent load  D2  --dry-run · --confirm=<机器名>", "zerg agent load  D2  --dry-run · --confirm=<主机名>"), chsFixCmds, bl, nil), true, "S4d", "S4f"},
		{"正控_配置面原样_绿_无S5断言", chsJudge(allT, bareT, dangT, chsFixCmds, bl, &cfgT), false, "", "S5"},
		{"负控_链行等长换字_S5a红_S5c不许响", chsJudge(allT, bareT, dangT, chsFixCmds, bl, chsPtr(strings.ReplaceAll(cfgT, "  "+chsFixChain, ch("  "+chsFixChain)))), true, "S5a", "S5c"},
		{"负控_裸help同源链改字_S5b红_S5a不许抢账", chsJudge(allT, strings.ReplaceAll(bareT, chsFixChain, ch(chsFixChain)), dangT, chsFixCmds, bl, &cfgT), true, "S5b", "S5a"},
		{"负控_配置面喂空串_必红", chsJudge(allT, bareT, dangT, chsFixCmds, bl, chsPtr("")), true, "S5a", ""},
		{"负控_删族级段_S1红", chsJudge(chsDropSection(allT), bareT, dangT, chsFixCmds, bl, nil), true, "S1", ""},
		{"负控_清单漏一族_S1红", chsJudge(strings.ReplaceAll(allT, "watch", "watchx"), bareT, dangT, chsFixCmds, bl, nil), true, "S1", ""},
		{"对界_命令树多一族_S1红", chsJudge(allT, bareT, dangT, append(append([]string{}, chsFixCmds...), "zzz ls"), bl, nil), true, "S1", ""},
	}
	for _, c := range cases {
		gotRed := len(c.bad) > 0
		if gotRed != c.wantRed {
			t.Errorf("%s：判红=%v 期望 %v（判词前 3：%v）", c.name, gotRed, c.wantRed, chsHead3(c.bad))
		}
		if c.wantSub != "" && !chsHasSub(c.bad, c.wantSub) {
			t.Errorf("%s：判词里没有点名的 %q（实得前 3：%v）", c.name, c.wantSub, chsHead3(c.bad))
		}
		if c.forbidSub != "" && chsHasSub(c.bad, c.forbidSub) {
			t.Errorf("%s：禁响子串 %q 冒出来了（实得前 3：%v）", c.name, c.forbidSub, chsHead3(c.bad))
		}
	}
}

func chsPtr(s string) *string { return &s }

func chsHead3(xs []string) []string {
	if len(xs) > 3 {
		return xs[:3]
	}
	return xs
}

func chsDropSection(allT string) string {
	lines := chsSplitLines(allT)
	idx := 0
	for i, l := range lines {
		if strings.HasPrefix(l, chsSecHeaderPrefix) {
			idx = i
			break
		}
	}
	return strings.Join(lines[:idx-1], "\n") + "\n"
}

// ── ③ 命令树：缺键 vs 显式 null 两态各成一格（✗ 不许一刀切）────────────────
func TestCheckHelpShape_MatrixNullVsMissing(t *testing.T) {
	dir := t.TempDir()
	nullP := filepath.Join(dir, "null.json")
	missP := filepath.Join(dir, "miss.json")
	os.WriteFile(nullP, []byte(`{"cases": [{"command": null}, {"command": "agent ls"}]}`), 0o644)
	os.WriteFile(missP, []byte(`{"cases": [{}, {"command": "agent ls"}]}`), 0o644)
	nullCmds, why1 := chsMatrixCommands(nullP)
	missCmds, why2 := chsMatrixCommands(missP)
	if why1 != "" || why2 != "" {
		t.Fatalf("读矩阵失败：%q / %q", why1, why2)
	}
	// 显式 null ⇒ py `str(None)` = "None" ⇒ 进集；缺键 ⇒ 默认 "" ⇒ 跳过。
	if strings.Join(nullCmds, ",") != "None,agent ls" {
		t.Errorf("显式 null 档：实得 %v 期望 [None agent ls]（py str(None)==\"None\"）", nullCmds)
	}
	if strings.Join(missCmds, ",") != "agent ls" {
		t.Errorf("缺键档：实得 %v 期望 [agent ls]（缺键走默认 \"\" 被过滤）", missCmds)
	}
	if strings.Join(nullCmds, ",") == strings.Join(missCmds, ",") {
		t.Errorf("两态被合成一态了 —— 缺键与显式 null 不可同判")
	}
}

// ── ④ py `json` 报话正文（逐字）────────────────────────────────────────
func TestCheckHelpShape_JSONErrorText(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", "Expecting value: line 1 column 1 (char 0)"},
		{" ", "Expecting value: line 1 column 2 (char 1)"},
		{"{", "Expecting property name enclosed in double quotes: line 1 column 2 (char 1)"},
		{"[", "Expecting value: line 1 column 2 (char 1)"},
		{"[1", "Expecting ',' delimiter: line 1 column 3 (char 2)"},
		{`{"a"`, "Expecting ':' delimiter: line 1 column 5 (char 4)"},
		{`{"a":1,}`, "Expecting property name enclosed in double quotes: line 1 column 8 (char 7)"},
		{"truex", "Extra data: line 1 column 5 (char 4)"},
		{`"abc`, "Unterminated string starting at: line 1 column 1 (char 0)"},
		{"{\"a\": \"b\"}\n x", "Extra data: line 2 column 2 (char 12)"},
	}
	for _, c := range cases {
		if got := chsPyJSONErr([]byte(c.in)); got != c.want {
			t.Errorf("json 报话 %q：实得 %q 期望 %q", c.in, got, c.want)
		}
	}
	if got := chsPyJSONErr([]byte(`{"cases": []}`)); got != "" {
		t.Errorf("合法 JSON 应返回空串（实得 %q）", got)
	}
}

// ── ⑤ 端到端：stub 制品 + 自写矩阵（红档必经各路）· rc=2 各路 · 未知旗标 ────
func chsWrite(t *testing.T, path, body string, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), mode); err != nil {
		t.Fatalf("写 %s 失败：%v", path, err)
	}
}

func TestCheckHelpShape_StubArtifactEndToEnd(t *testing.T) {
	dir := t.TempDir()
	mp := filepath.Join(dir, "cli-matrix.json")
	chsWrite(t, mp, `{"cases":[{"command":"agent ls"},{"command":"core restart"},{"command":"gap ls"},{"command":"gate run"},{"command":"help"}]}`, 0o644)
	allT, bareT, dangT, cfgT := chsFixTexts()
	chsWrite(t, filepath.Join(dir, "all.txt"), allT, 0o644)
	chsWrite(t, filepath.Join(dir, "bare.txt"), bareT, 0o644)
	chsWrite(t, filepath.Join(dir, "danger.txt"), dangT, 0o644)
	chsWrite(t, filepath.Join(dir, "config.txt"), cfgT, 0o644)
	stub := filepath.Join(dir, "zerg")
	chsWrite(t, stub, "#!/bin/sh\n"+
		"case \"$*\" in\n"+
		"  \"help --all\") cat "+filepath.Join(dir, "all.txt")+" ;;\n"+
		"  \"help\") cat "+filepath.Join(dir, "bare.txt")+" ;;\n"+
		"  \"help dangerous\") cat "+filepath.Join(dir, "danger.txt")+" ;;\n"+
		"  \"help config\") cat "+filepath.Join(dir, "config.txt")+" ;;\n"+
		"  *) exit 3 ;;\n"+
		"esac\n", 0o755)
	out, errS, rc := chsRunT("--repo-root", dir, "--bin", stub, "--matrix", mp)
	if rc != 1 {
		t.Fatalf("stub 档 rc=%d 期望 1（真制品常量 ⇒ 必红；stderr=%q）", rc, errS)
	}
	for _, sub := range []string{"`help` 面形状判据 · v1.3.0", "⇒ 判红（rc=1）：", "S1 族级段", "S2 常量"} {
		if !strings.Contains(out, sub) {
			t.Errorf("stub 档 stdout 缺 %q（前 3 行：%v）", sub, chsHead3(chsSplitLines(out)))
		}
	}
	// --json 面：bad 非空且是 JSON 数组
	out2, _, rc2 := chsRunT("--repo-root", dir, "--bin", stub, "--matrix", mp, "--json")
	if rc2 != 1 || !strings.Contains(out2, `"bad": ["`) || !strings.Contains(out2, `"mode": "judge"}`) {
		t.Fatalf("--json 红档不对（rc=%d）：%q", rc2, out2)
	}
}

func TestCheckHelpShape_Rc2AndArgvPaths(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		name string
		args []string
		rc   int
		sub  string
	}{
		{"制品不在盘", []string{"--repo-root", dir}, 2, "不给结论（rc=2）"},
		{"矩阵不在盘", []string{"--repo-root", dir, "--bin", "/nonexistent/zerg", "--matrix", "/nonexistent/m.json"}, 2, "命令树矩阵不在盘"},
		{"未知旗标", []string{"--bogus"}, 2, ""},
		{"缺值", []string{"--repo-root"}, 2, ""},
	}
	for _, c := range cases {
		out, errS, rc := chsRunT(c.args...)
		if rc != c.rc {
			t.Errorf("%s：rc=%d 期望 %d（stdout=%q stderr=%q）", c.name, rc, c.rc, out, errS)
		}
		if c.sub != "" && !strings.Contains(out, c.sub) {
			t.Errorf("%s：stdout 缺 %q（实得 %q）", c.name, c.sub, out)
		}
		if c.name == "未知旗标" && !strings.Contains(errS, "error: unrecognized arguments: --bogus") {
			t.Errorf("未知旗标 stderr 不对：%q", errS)
		}
		if c.name == "缺值" && !strings.Contains(errS, "argument --repo-root: expected one argument") {
			t.Errorf("缺值 stderr 不对：%q", errS)
		}
	}
	// 制品是目录 ⇒ 「不在盘」（os.Stat 后 isfile 为假）
	out, _, rc := chsRunT("--repo-root", dir, "--bin", dir, "--matrix", filepath.Join(dir, "x.json"))
	if rc != 2 || !strings.Contains(out, "不在盘") {
		t.Fatalf("目录制品档不对（rc=%d）：%q", rc, out)
	}
}
