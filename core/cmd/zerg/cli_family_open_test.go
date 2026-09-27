// cli_family_open_test.go —— 族级帮助「已开放 / 拒执 / 危险档」三档计数（缺口 序33 · 2026-09-24 已拍）。
//
// 病（修前照实现读出来的）：`renderFamilyHelp` 的计数只写 `danger == nil` ⇒ 把「非危险档」当成
// 「已开放」的**代理**，于是一条**真跑拒执**的非危险档命令**两头占**（既算「已开放」、又不进危险档）
// ⇒ 双计。现读实据（修前）：`zerg help egg` 报「动作 6 条 · 已开放 3 · 危险档 3」—— 6 = 3 + 3 看着自洽，
// 而 `egg run` 真跑退 8（`cmdEggRun`：卵写面本波不做）⇒ 它本不该算「已开放」。
//
// ★ 2026-09-26 随动：卵面**写档开出来了**（`egg run` 装载 / `egg stop` 卸载 —— 内部 = 控制面
//
//	  `POST /api/control/load|unload`）⇒ `egg run` 的 `refuses` 撤掉、它进「已开放」档。
//	  本件的 ②/③/④ 三条判据**随动重写**（旧的 ②/③/④ 硬点名的正是「`egg run` 拒执」这一事实，
//	  事实没了就得改判据，不许留一条永远说旧话的绿）：
//
//		① 正控（不变）：对命令树里**每一个**族 —— 首行三档计数之和 == 该族成员数；且
//		   「已开放」== 该族 `openedForRun` 为真的条数 · 「拒执」== 声明了 `refuses` 的非危险档条数 ·
//		   「危险档」== `danger != nil` 的条数（三档**各归各的**，一个都不许重复计）。
//		② 对界（重写）：`egg run` / `egg stop` 是**已开放**的写档 —— 声明无 `refuses`、`danger == nil`、
//		   `openedForRun` 为真；`help egg` 首行现报「动作 7 · 已开放 4 · 拒执 0 · 危险档 3」（与现算逐格相等）。
//		③ 成对负控（**改成动态找对象**，不硬套某一条）：命令树里仍有「非危险档 + 真跑拒执」的命令
//		   （`cocoon open` / 发布档 / `apply`）⇒ 拿**修前的口径**（`danger == nil` 即算「已开放」）重算
//		   **它那一族** ⇒ 条数必与现报的「已开放」档**不等**（若相等 ⇒ 这条判据在修前也给绿 ⇒ 空转）。
//		④ 口径 ↔ 真行为绑定（重写）：`egg run <卵 id>`（不给 `--yes`/`--dry-run`）真跑 ⇒ **退 2**
//		   （D2 缺 `--yes` ⇒ 不执行）· stdout **0 字节** · 且**先于任何网络**判（该条只读、零副作用、
//		   不碰主控 —— 主控没起也照样给得出这条判词）。
package main

import (
	"io"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// familyMembersForTest —— 某族在命令树里的全部成员（名字序与 `renderFamilyHelp` 同源：都走 `catalog()`）。
func familyMembersForTest(family string) []*command {
	var out []*command
	for _, c := range catalog() {
		if len(c.path) > 0 && c.path[0] == family {
			out = append(out, c)
		}
	}
	return out
}

// familyHelpCountsForTest —— 按修后的口径现算某族的三档计数（**不重抄一份逻辑**：逐条问 `openedForRun`）。
func familyHelpCountsForTest(family string) (open, refused, danger int) {
	for _, c := range familyMembersForTest(family) {
		switch {
		case c.danger != nil:
			danger++
		case openedForRun(c):
			open++
		default:
			refused++
		}
	}
	return open, refused, danger
}

// helpFirstLineForTest —— `help <族>` 现跑的首行（三档计数就在这一行上）。
func helpFirstLineForTest(t *testing.T, family string) string {
	t.Helper()
	var out, errb strings.Builder
	if rc := run([]string{"help", family}, &out, &errb); rc != 0 {
		t.Fatalf("`zerg help %s` 要退 0（判据不可判），得到 %d · stderr=%q", family, rc, errb.String())
	}
	return strings.SplitN(out.String(), "\n", 2)[0]
}

var familyLineRE = regexp.MustCompile(`^zerg (\S+) —— 族级用法（动作 (\d+) 条 · 已开放 (\d+) · 拒执 (\d+) · 危险档 (\d+)）$`)

// TestFamilyOpenCounts_ThreeTiersAreExclusive —— ① 每一个族：三档之和 == 动作数，且三档逐档与命令树现算相等。
func TestFamilyOpenCounts_ThreeTiersAreExclusive(t *testing.T) {
	families := map[string]bool{}
	for _, c := range catalog() {
		if len(c.path) > 0 {
			families[c.path[0]] = true
		}
	}
	if len(families) == 0 {
		t.Fatal("命令树里一个族都取不到（判据不可判）")
	}
	judged, shadowed := 0, []string{}
	for f := range families {
		// ★ 主题表先判（`cmdHelp` 的边界 ②）：`version` 一类**既是族名又是主题名** ⇒ 主题面赢、
		// 出的是专题页而**不是**族级帮助 ⇒ 这一族不参与本判据（跳过，但**逐条打印**，不静默）。
		if isHelpTopic(f) {
			shadowed = append(shadowed, f)
			continue
		}
		line := helpFirstLineForTest(t, f)
		m := familyLineRE.FindStringSubmatch(line)
		if m == nil {
			t.Fatalf("`zerg help %s` 首行形状不认（三档计数各一格 + 与动作数并列）：%q", f, line)
		}
		judged++
		if m[1] != f {
			t.Errorf("首行报的族名 %q ≠ 请求的族名 %q", m[1], f)
		}
		num := func(i int) int { n, _ := strconv.Atoi(m[i]); return n }
		nTotal, nOpen, nRefused, nDanger := num(2), num(3), num(4), num(5)
		wOpen, wRefused, wDanger := familyHelpCountsForTest(f)
		wTotal := len(familyMembersForTest(f))
		if nTotal != wTotal || nOpen != wOpen || nRefused != wRefused || nDanger != wDanger {
			t.Errorf("`zerg help %s` 现报「动作 %d · 已开放 %d · 拒执 %d · 危险档 %d」≠ 命令树现算「动作 %d · 已开放 %d · 拒执 %d · 危险档 %d」",
				f, nTotal, nOpen, nRefused, nDanger, wTotal, wOpen, wRefused, wDanger)
		}
		if nOpen+nRefused+nDanger != nTotal {
			t.Errorf("`zerg help %s` 三档相加 %d+%d+%d ≠ 动作数 %d —— 有一条被**双计**或漏计（本条判据要的就是「各归各的」）",
				f, nOpen, nRefused, nDanger, nTotal)
		}
	}
	// ★ **不许空转**：判过的族太少（或跳过了一堆非主题名的族）⇒ 这条判据就没在判东西。
	if judged < 10 {
		t.Errorf("只判到 %d 个族（应 ≥10 —— 命令树 133 条里的族远多于这个数）⇒ 判据近乎空转", judged)
	}
	t.Logf("现读：判过 %d 个族 · 主题名压过族名（跳过，见 `cmdHelp` 边界②）%d 个：%s",
		judged, len(shadowed), strings.Join(shadowed, " · "))
}

// TestFamilyOpenCounts_EggWriteFaceIsOpen —— ② 对界（2026-09-26 重写）：
// `egg run` / `egg stop` 是**已开放**的写档（真跑执行装载/卸载），不再落「拒执」档。
//
// 旧文（已作废，逐字留此免得下一个人以为它还在）：「`egg run` 落「拒执」档、**不在**「已开放」档里
// —— 它真跑退 8（写面本波不做）」。
func TestFamilyOpenCounts_EggWriteFaceIsOpen(t *testing.T) {
	for _, p := range [][]string{{"egg", "run"}, {"egg", "stop"}} {
		c := find(p)
		if c == nil {
			t.Fatalf("命令树里找不到 `%s`（判据不可判）", strings.Join(p, " "))
		}
		if c.danger != nil {
			t.Fatalf("`%s` 现读是危险档（%s）⇒ 本条的落点变了，判据要重写（不硬套）",
				strings.Join(p, " "), c.danger.Level)
		}
		if c.refuses {
			t.Errorf("`%s` 现读仍声明 `refuses` —— 写面已开出（真跑执行装载/卸载）⇒ 该标记必须撤掉"+
				"（否则「已开放」那一档把它漏掉，首行计数与实际不符）", strings.Join(p, " "))
		}
		if !openedForRun(c) {
			t.Errorf("`%s` 真跑执行 ⇒ 口径 `openedForRun` 必须是 true（写档算「已开放」）", strings.Join(p, " "))
		}
	}
	open, refused, danger := familyHelpCountsForTest("egg")
	if open != 4 || refused != 0 || danger != 3 {
		t.Errorf("`egg` 族现算「已开放 %d · 拒执 %d · 危险档 %d」（要 4 / 0 / 3：`ls`+`show`+`run`+`stop` 已开放 · `pin`+`unpin`+`retire` 危险档）",
			open, refused, danger)
	}
	line := helpFirstLineForTest(t, "egg")
	m := familyLineRE.FindStringSubmatch(line)
	if m == nil {
		t.Fatalf("`zerg help egg` 首行形状不认：%q", line)
	}
	if m[3] != "4" || m[4] != "0" || m[5] != "3" {
		t.Errorf("`zerg help egg` 首行报「已开放 %s · 拒执 %s · 危险档 %s」（要 4 / 0 / 3）—— 「报数走唯一真源 `openedForRun`」这一条现读不成立：%q",
			m[3], m[4], m[5], line)
	}
	if _, refusedNow, _ := familyHelpCountsForTest("egg"); refusedNow != 0 {
		t.Errorf("`egg` 族的「拒执」档现算 %d 条（要 0 —— 写档开出后本族没有真跑拒执的命令了）", refusedNow)
	}
}

// TestFamilyOpenCounts_NegativeControl_OldRuleWouldBeGreen —— ③ 成对负控（2026-09-26 重写）：
// 拿**修前的口径**（`danger == nil` 即算「已开放」）重算**某条真跑拒执的族** ⇒ 必与现报的「已开放」档**不等**。
// 若相等，说明这条判据在**修前**也给绿（空转），那它就抓不住「双计」这个病。
//
// ★ 为什么改成**动态找对象**：旧版硬点名 `egg` 族（当时 `egg run` 是那条「非危险档 + 拒执」的样本），
// 写面开出后 `egg` 族里没有拒执命令了 ⇒ 硬点名的版本要么永远说旧话、要么只能删。改成从命令树里
// **现找**一条 `refuses` 的非危险档命令（`cocoon open` / 发布档 / `apply` 都在），找不到才 Fatal。
func TestFamilyOpenCounts_NegativeControl_OldRuleWouldBeGreen(t *testing.T) {
	var target *command
	for _, c := range catalog() {
		if c.danger == nil && c.refuses {
			target = c
			break
		}
	}
	if target == nil {
		t.Fatal("命令树里找不到任何「非危险档 + 真跑拒执」的命令 ⇒ 负控无对象（判据不可判 —— 要么 `refuses` 已全撤，要么口径变了）")
	}
	fam := target.path[0]
	oldOpen := 0
	for _, c := range familyMembersForTest(fam) {
		if c.danger == nil { // 修前的口径：非危险档 = 已开放
			oldOpen++
		}
	}
	newOpen, refused, _ := familyHelpCountsForTest(fam)
	if refused < 1 {
		t.Fatalf("`%s` 族的「拒执」档现算 0 条（负控对象 = `%s` · 它该落在这一档）",
			fam, strings.Join(target.path, " "))
	}
	if oldOpen == newOpen {
		t.Errorf("负控失败：族 `%s`（对象 = `%s`）修前口径算得 %d 条 == 修后口径算得 %d 条 ⇒ 本判据抓不住「双计」",
			fam, strings.Join(target.path, " "), oldOpen, newOpen)
	}
}

// TestFamilyOpenCounts_WriteFaceGateIsRealBehaviour —— ④ 口径 ↔ 真行为绑定（2026-09-26 重写）：
// `egg run <卵 id>` / `egg stop <卵 id>`（不给 `--yes`/`--dry-run`）真跑必须**退 2**（D2 缺 `--yes` ⇒ 不执行）。
//
// 旧文（已作废）：「`egg run <卵 id>` 真跑必须退 8（blocked）」。
//
// 零副作用：授权闸在**读名册之前**判 ⇒ 这一跑一个请求都不发、不写任何件、不碰真机（主控没起也一样）。
func TestFamilyOpenCounts_WriteFaceGateIsRealBehaviour(t *testing.T) {
	var out, errb strings.Builder
	rc := run([]string{"egg", "run", "any-egg-id"}, &out, &errb)
	if rc != 2 {
		t.Fatalf("`egg run` 缺 `--yes` 该退 2（D2 fail-closed ⇒ 不执行），得到 rc=%d · stderr=%q", rc, errb.String())
	}
	if out.String() != "" {
		t.Errorf("`egg run` 缺 `--yes` 的 stdout 该是 0 字节（计划件走 stderr），得到 %q", out.String())
	}
	if !strings.Contains(errb.String(), "--yes") {
		t.Errorf("`egg run` 缺 `--yes` 该逐字报「缺 --yes ⇒ 不执行」并给出下一步（加 --yes），得到 %q", errb.String())
	}
	// ★ 卸载档走**同一条**授权闸（同一张表）—— 两档不许分叉。
	rc = run([]string{"egg", "stop", "any-egg-id"}, io.Discard, io.Discard)
	if rc != 2 {
		t.Errorf("`egg stop` 缺 `--yes` 该退 2（与装载档同一张授权表），得到 rc=%d", rc)
	}
	// 顺带把「两跑一致（无状态）」钉一下。
	if rc := run([]string{"egg", "run", "any-egg-id"}, io.Discard, io.Discard); rc != 2 {
		t.Errorf("第二次现跑 rc=%d（两跑必须一致 —— 执行前判那一档无状态）", rc)
	}
}
