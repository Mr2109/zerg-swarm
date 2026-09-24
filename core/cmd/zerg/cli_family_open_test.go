// cli_family_open_test.go —— 族级帮助「已开放 / 拒执 / 危险档」三档计数（缺口 序33 · 2026-09-24 已拍）。
//
// 病（修前照实现读出来的）：`renderFamilyHelp` 的计数只写 `danger == nil` ⇒ 把「非危险档」当成
// 「已开放」的**代理**，于是一条**真跑拒执**的非危险档命令**两头占**（既算「已开放」、又不进危险档）
// ⇒ 双计。现读实据：`zerg help egg` 报「动作 6 条 · 已开放 3 · 危险档 3」—— 6 = 3 + 3 看着自洽，
// 而 `egg run` 真跑退 8（`cmdEggRun`：卵写面本波不做）⇒ 它本不该算「已开放」。
//
// 本件的判据（全部**现读**，不抄第二份清单）：
//
//	① 正控：对命令树里**每一个**族 —— 首行三档计数之和 == 该族成员数；且
//	   「已开放」== 该族 `openedForRun` 为真的条数 · 「拒执」== 声明了 `refuses` 的非危险档条数 ·
//	   「危险档」== `danger != nil` 的条数（三档**各归各的**，一个都不许重复计）。
//	② 对界（本条点名的那一枚）：`zerg help egg` 的「已开放」**不含** `egg run`，且它落在「拒执」档。
//	③ 成对负控（证明这条判据**有牙**）：拿**修前的口径**（`danger == nil` 即算「已开放」）重算 `egg`
//	   族 ⇒ 条数必与「已开放」档**不等**（若相等 ⇒ 这条判据在修前也给绿 ⇒ 它是空转）。
//	④ 口径 ↔ 真行为绑定：`egg run <卵 id>` 真跑（**不给干跑档**）**退 8** —— 「拒执」那一档
//	   不是纸面分类，是它现跑真拒。只读、不写任何件、不碰网络（该条无条件退 8）。
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
		t.Errorf("只判到 %d 个族（应 ≥10 —— 命令树 127 条里的族远多于这个数）⇒ 判据近乎空转", judged)
	}
	t.Logf("现读：判过 %d 个族 · 主题名压过族名（跳过，见 `cmdHelp` 边界②）%d 个：%s",
		judged, len(shadowed), strings.Join(shadowed, " · "))
}

// TestFamilyOpenCounts_EggRunIsRefusedNotOpen —— ② 对界：`egg run` 落「拒执」档、**不在**「已开放」档里。
func TestFamilyOpenCounts_EggRunIsRefusedNotOpen(t *testing.T) {
	eggRun := find([]string{"egg", "run"})
	if eggRun == nil {
		t.Fatal("命令树里找不到 `egg run`（判据不可判）")
	}
	if eggRun.danger != nil {
		t.Fatalf("`egg run` 现读是危险档（%s）⇒ 本条的落点变了，判据要重写（不硬套）", eggRun.danger.Level)
	}
	if !eggRun.refuses {
		t.Errorf("`egg run` 该声明 `refuses`（它真跑退 8 —— 见下一条用例），现读 false")
	}
	if openedForRun(eggRun) {
		t.Errorf("`egg run` 真跑拒执 ⇒ 口径 `openedForRun` 必须是 false（否则「已开放」里又把它算进去了）")
	}
	line := helpFirstLineForTest(t, "egg")
	m := familyLineRE.FindStringSubmatch(line)
	if m == nil {
		t.Fatalf("`zerg help egg` 首行形状不认：%q", line)
	}
	if m[3] == m[2] { // 已开放 == 动作数 ⇒ 一条都没落在别的档里
		t.Errorf("`zerg help egg` 首行「已开放 == 动作数」（%q）⇒ 拒执的那条仍被算进「已开放」", line)
	}
	if _, refused, _ := familyHelpCountsForTest("egg"); refused < 1 {
		t.Errorf("`egg` 族的「拒执」档现算 %d 条（应 ≥1 —— `egg run` 真跑退 8）", refused)
	}
}

// TestFamilyOpenCounts_NegativeControl_OldRuleWouldBeGreen —— ③ 成对负控：
// 拿**修前的口径**（`danger == nil` 即算「已开放」）重算 `egg` 族 ⇒ 必与现报的「已开放」档**不等**。
// 若相等，说明这条判据在**修前**也给绿（空转），那它就抓不住「双计」这个病。
func TestFamilyOpenCounts_NegativeControl_OldRuleWouldBeGreen(t *testing.T) {
	oldOpen := 0
	for _, c := range familyMembersForTest("egg") {
		if c.danger == nil { // 修前的口径：非危险档 = 已开放
			oldOpen++
		}
	}
	_, refused, _ := familyHelpCountsForTest("egg")
	if refused < 1 {
		t.Fatal("`egg` 族的「拒执」档现算 0 条 ⇒ 负控无对象（判据不可判）")
	}
	newOpen, _, _ := familyHelpCountsForTest("egg")
	if oldOpen == newOpen {
		t.Errorf("负控失败：修前口径算得 %d 条 == 修后口径算得 %d 条 ⇒ 本判据抓不住「双计」", oldOpen, newOpen)
	}
}

// TestFamilyOpenCounts_RefusalIsRealBehaviour —— ④ 口径 ↔ 真行为绑定：
// `egg run <卵 id>` 真跑（**不给干跑档**）必须退 8（blocked）—— 「拒执」那一档是**现跑真拒**，不是纸面分类。
// 只读：该条只往 stderr 打两句、不写任何件、不碰网络、不碰真机。
func TestFamilyOpenCounts_RefusalIsRealBehaviour(t *testing.T) {
	var out, errb strings.Builder
	rc := run([]string{"egg", "run", "any-egg-id"}, &out, &errb)
	if rc != 8 {
		t.Fatalf("`egg run` 真跑该退 8（不给结论 · kind=blocked），得到 rc=%d · stderr=%q", rc, errb.String())
	}
	if out.String() != "" {
		t.Errorf("`egg run` 真跑的 stdout 该是 0 字节，得到 %q", out.String())
	}
	if !strings.Contains(errb.String(), "不给结论") {
		t.Errorf("`egg run` 真跑该逐字报「不给结论」，得到 %q", errb.String())
	}
	// 顺带把「零副作用」钉一下：这一跑一个文件都不该被写（该命令不碰盘）。
	if rc := run([]string{"egg", "run", "any-egg-id"}, io.Discard, io.Discard); rc != 8 {
		t.Errorf("第二次现跑 rc=%d（两跑必须一致 —— 该条无状态）", rc)
	}
}
