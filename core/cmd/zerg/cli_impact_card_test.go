// cli_impact_card_test.go —— `zerg impact` 的**波纹卡片**（`A3`）判据
// （任务单-影响面实施-20260922 §二 `A3` 判据①–④ · 设计-变更影响面-v1.6 §3.1–§3.8/§4.1/§九 判据⑨）。
//
// 落点在 `package main_test`（§九 M17 三层测试落点①②）：跑的是**当前源码**的运行期行为；
// 纯函数一律走 `export_test.go` 的只读桥（不许在这里另写一份判据副本）。
//
// 四条判据的机检口（**都在本文件里真跑**）：
//
//	① 尺子（条数 ≤ 12 · 总量 ≤ 1.2k · 单条 ≤ 3 行 / ≤ 60 token）—— 判定口 `ImpactCardJudgeForTest`；
//	② 「没有 `why` 的条目一律不进卡片」—— 判定口 `ImpactCardWhyJudgeForTest` + 造卡口 `ImpactCardOfForTest`；
//	③ 超预算三件（`truncated=true` / `warnings[]`「已裁 N 条」/ `meta.how_to_restore`）+
//	   「会红」那一行**置顶且最后才裁**；
//	④ 退法可执行性（`v1.6` §九 判据⑨）：档 1 那条命令要能在盘上解析到目标 · 档 3 行内必须有「无退法」三字。
package main_test

import (
	"encoding/json"
	"os/exec"
	"strings"
	"testing"

	zerg "github.com/Mr2109/zerg-swarm/core/cmd/zerg"
)

// impactCardRow 造一条卡片条目（测试夹具用 · 四字段）。
func impactCardRow(what, why, how, red string) map[string]string {
	return map[string]string{"what": what, "why": why, "how": how, "red": red}
}

// TestImpactCard_CapsAreHard —— 判据①（四个上限是**常量**，不是随跑随变的数）。
func TestImpactCard_CapsAreHard(t *testing.T) {
	items, tokens, lines, itemTokens := zerg.ImpactCardCapsForTest()
	if items != 12 {
		t.Errorf("条数上限 = %d（§3.1 要 12）", items)
	}
	if tokens != 1200 {
		t.Errorf("总量上限 = %d（§3.1/§4.1 要 1.2k —— **不许抬到 1.3k**）", tokens)
	}
	if lines != 3 {
		t.Errorf("单条行数上限 = %d（§3.1 要 3）", lines)
	}
	if itemTokens != 60 {
		t.Errorf("单条 token 上限 = %d（§3.1 要 60）", itemTokens)
	}
	// `why` 闭集**六选一**（含「公开面」）—— 不许有第七个键。
	why6 := zerg.ImpactWhySixForTest()
	if len(why6) != 6 {
		t.Fatalf("`why` 闭集 = %d 档（要六选一）：%v", len(why6), why6)
	}
	has := func(w string) bool {
		for _, x := range why6 {
			if x == w {
				return true
			}
		}
		return false
	}
	for _, w := range []string{"包反向", "调用边", "契约", "词法", "义近", "公开面"} {
		if !has(w) {
			t.Errorf("闭集里少了 %q（六档逐字）：%v", w, why6)
		}
	}
	// 负控：闭集里**不许**有「形近」（它是「词法（含形近）」那一格，归一后不另立键）。
	if has("形近") {
		t.Error("负控失败：闭集里出现了「形近」—— 那就成了第七个 `why` 键（§3.1 六选一）")
	}
}

// TestImpactCard_RealRunFitsCapsAndFourFields —— 判据① 正控（**现跑**）：真目标的卡片条数 / 单条 /
// 总量都在尺子内，且每条四字段齐、`why` 落在闭集里。
func TestImpactCard_RealRunFitsCapsAndFourFields(t *testing.T) {
	root := repoRootFromCLI(t)
	t.Setenv("ZERG_REPO", root)
	// `A5`：`zerg impact` 自本批起**会落盘缓存**（落点在状态目录）⇒ 测试一律把状态目录改到合成目录，
	// **不碰真状态目录**（本件判据一个字不动；不这么做就是「测试有副作用」）。
	t.Setenv("ZERG_STATE_DIR", t.TempDir())
	itemsMax, _, _, _ := zerg.ImpactCardCapsForTest()

	rc, out, errb := runCapture("impact", "core/cmd/zerg/main.go", "--json", "what,why,how,red")
	if rc != 0 {
		t.Fatalf("有影响面的件 ⇒ 期望 rc=0，得到 %d · stderr=%s", rc, errb)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("机器面不是对象：%v", err)
	}
	var rows []map[string]string
	if err := json.Unmarshal(doc["items"], &rows); err != nil {
		t.Fatalf("items 不是数组：%v", err)
	}
	if len(rows) == 0 {
		t.Fatal("有影响面的件 ⇒ 卡片不该空")
	}
	if len(rows) > itemsMax {
		t.Errorf("卡片条数 %d > 上限 %d（§3.1）", len(rows), itemsMax)
	}
	for i, it := range rows {
		for _, f := range []string{"what", "why", "how"} {
			if strings.TrimSpace(it[f]) == "" {
				t.Errorf("第 %d 条字段面缺 %s（四字段 what/why/how/red · red 无则空）：%+v", i+1, f, it)
			}
		}
		if err := zerg.ImpactCardWhyJudgeForTest(rows[i : i+1]); err != nil {
			t.Errorf("第 %d 条的 why 不过闭集判据：%v", i+1, err)
		}
		if tk := zerg.ImpactCardItemTokensForTest(it); tk > 60 {
			t.Errorf("第 %d 条 %d token > 单条上限 60", i+1, tk)
		}
	}
	if err := zerg.ImpactCardWhyJudgeForTest(rows); err != nil {
		t.Errorf("判据② 破：%v", err)
	}
	// 卡片账（stderr）：上限逐字给出 + 「上限是死的」（不许抬到 1.3k）。
	for _, want := range []string{"上限 12", "1200", "上限是死的", "1.3k"} {
		if !strings.Contains(errb, want) {
			t.Errorf("卡片账里没有 %q：%s", want, lineOf(errb, "卡片账"))
		}
	}
	// 判据②：判定口的**负控** —— 喂一条无 `why`、一条第七键 ⇒ 必须报错。
	if err := zerg.ImpactCardWhyJudgeForTest([]map[string]string{impactCardRow("文件级:a.go:1", "", "h", "")}); err == nil {
		t.Error("负控失败：没有 why 的条目竟判过 —— 硬门槛没牙")
	}
	if err := zerg.ImpactCardWhyJudgeForTest([]map[string]string{impactCardRow("文件级:a.go:1", "第七键", "h", "")}); err == nil {
		t.Error("负控失败：不在闭集里的 why 竟判过")
	}
	// 判定口的负控（尺子）：条数多一条 / 单条超 60 token ⇒ 必须报错。
	good := zerg.ImpactCardOfForTest(rows, "", "core/cmd/zerg/main.go")
	if err := zerg.ImpactCardJudgeForTest(good); err != nil {
		t.Fatalf("正控就不成立：真跑的卡片竟判不过：%v", err)
	}
	more := good
	more.Items = append(append([]map[string]string{}, good.Items...), impactCardRow("文件级:zz.go:1", "词法", "h", ""))
	if err := zerg.ImpactCardJudgeForTest(more); err == nil {
		t.Errorf("负控失败：%d 条（上限 %d）竟判过", len(more.Items), itemsMax)
	}
	fat := good
	fat.Items = []map[string]string{impactCardRow("文件级:a.go:1", "词法", strings.Repeat("很长的一句话。", 60), "")}
	if err := zerg.ImpactCardJudgeForTest(fat); err == nil {
		t.Error("负控失败：单条超 60 token 竟判过")
	}
	over := good
	over.TotalTokens = 1201
	if err := zerg.ImpactCardJudgeForTest(over); err == nil {
		t.Error("负控失败：总量 1201 > 1.2k 竟判过 —— 「上限是死的」这一格没牙")
	}
}

// TestImpactCard_NoWhyAndClosedSetFilter —— 判据② 正控：**没有 `why` 的条目一条都不进**；
// 不在闭集里的也不进；「形近」归一成「词法」（六选一）；语义级条目带的 `red` 一律清空。
func TestImpactCard_NoWhyAndClosedSetFilter(t *testing.T) {
	rows := []map[string]string{
		impactCardRow("包级:x", "包反向", "h", ""),
		impactCardRow("文件级:a.go:1", "", "h", ""),    // 无 why ⇒ 必须被丢
		impactCardRow("文件级:b.go:2", "第七键", "h", ""), // 不在闭集 ⇒ 必须被丢
		impactCardRow("文件级:c.go:3", "形近", "h", ""),  // 归一 ⇒ 词法
		impactCardRow("件级:z", "义近", "h", "S-g"),     // 义近带 red ⇒ 清空（§3.5 铁律）
	}
	c := zerg.ImpactCardOfForTest(rows, "", "t")
	if c.NoWhy != 1 {
		t.Errorf("无 why 未进卡的计数 = %d（要 1）", c.NoWhy)
	}
	if c.UnknownWhy != 1 {
		t.Errorf("不在闭集未进卡的计数 = %d（要 1）", c.UnknownWhy)
	}
	if c.SemRed != 1 {
		t.Errorf("义近条目带 red 被清空的计数 = %d（要 1）", c.SemRed)
	}
	if len(c.Items) != 3 {
		t.Fatalf("进卡条数 = %d（要 3：包反向 + 归一后的词法 + 义近）· items=%+v", len(c.Items), c.Items)
	}
	sawNormalized, sawSemantic := false, false
	for _, it := range c.Items {
		switch it["why"] {
		case "词法":
			if strings.HasPrefix(it["what"], "文件级:c.go:") {
				sawNormalized = true
			}
		case "义近":
			sawSemantic = true
			if it["red"] != "" {
				t.Errorf("义近条目带 red=%q 进了卡片 —— §3.5：语义级只进「建议」行", it["red"])
			}
		}
	}
	if !sawNormalized {
		t.Errorf("「形近」没有被归一成「词法」（六选一）：%+v", c.Items)
	}
	if !sawSemantic {
		t.Errorf("义近条目被丢了（它该进卡、只是不进「会红」行）：%+v", c.Items)
	}
	if err := zerg.ImpactCardJudgeForTest(c); err != nil {
		t.Errorf("判定口把正控判红了：%v", err)
	}
	// 负控：判定口必须挡得住「语义级条目带 red」。
	if err := zerg.ImpactCardRedLineJudgeForTest([]map[string]string{impactCardRow("件级:z", "义近", "h", "S-g")}); err == nil {
		t.Error("负控失败：语义级（义近）条目带 red 竟判过 —— §3.5 铁律没牙")
	}
}

// TestImpactCard_CutByTierAndBudgetThreePieces —— 判据③：超预算时**同批写三件**，
// 且裁序按 §3.3 三档 —— 「会红」那一条**置顶且最后才裁**（这一跑不给它使，档④ 一条不裁）。
func TestImpactCard_CutByTierAndBudgetThreePieces(t *testing.T) {
	rows := []map[string]string{}
	for i := 0; i < 3; i++ { // 3 条会红（档④ · 最后才裁）
		rows = append(rows, impactCardRow("契约级:S-"+string(rune('a'+i)), "契约", "h", "S-"+string(rune('a'+i))))
	}
	for i := 0; i < 5; i++ { // 5 条义近（档① · 最先裁）
		rows = append(rows, impactCardRow("件级:y"+string(rune('a'+i)), "义近", "h", ""))
	}
	for i := 0; i < 12; i++ { // 12 条词法（档②）
		rows = append(rows, impactCardRow("文件级:f"+string(rune('a'+i))+".go:1", "词法", "h", ""))
	}
	c := zerg.ImpactCardOfForTest(rows, "", "t")
	if len(c.Items) != 12 {
		t.Fatalf("裁后条数 = %d（要 12）", len(c.Items))
	}
	if !c.Truncated {
		t.Fatal("裁了却 `truncated` 不为 true")
	}
	if c.CutTier[1] != 5 || c.CutTier[2] != 3 || c.CutTier[3] != 0 || c.CutTier[4] != 0 {
		t.Errorf("裁序档计数 = %v（要 档①5 · 档②3 · 档③0 · 档④0 —— 「会红」最后才裁，这一跑一条都没裁）", c.CutTier)
	}
	if c.RedN != 3 {
		t.Errorf("会红条目存活数 = %d（要 3 —— 档④ 排在最后裁）", c.RedN)
	}
	for i := 0; i < 3; i++ {
		if c.Items[i]["red"] == "" {
			t.Errorf("第 %d 条不是会红条目 —— 「会红」那一条必须**置顶**（§3.2 第一级）", i+1)
		}
	}
	joined := strings.Join(c.Warnings, "｜")
	if !strings.Contains(joined, "已裁 8 条") {
		t.Errorf("三件之二：`warnings[]` 里没有「已裁 8 条」（实得 %q）", joined)
	}
	if strings.TrimSpace(c.HowRestore) == "" {
		t.Error("三件之三：`meta.how_to_restore` 空着")
	}
	if !strings.Contains(c.HowRestore, "--for-human") {
		t.Errorf("找回路径不是人档那一条（§3.3：`建议 zerg impact … --for-human` 的形态）：%q", c.HowRestore)
	}
	if err := zerg.ImpactCardJudgeForTest(c); err != nil {
		t.Errorf("判定口把正控判红了：%v", err)
	}

	// 负控（三件逐件拆）：每一件缺了都必须红。
	f1 := c
	f1.Warnings = nil
	if err := zerg.ImpactCardJudgeForTest(f1); err == nil {
		t.Error("负控失败：裁了却 `warnings[]` 空（三件之二）竟判过")
	}
	f2 := c
	f2.HowRestore = ""
	if err := zerg.ImpactCardJudgeForTest(f2); err == nil {
		t.Error("负控失败：裁了却 `how_to_restore` 空（三件之三）竟判过")
	}
	f3 := c
	f3.Truncated = false // 三件留着、真正的旗标却摘了 ⇒ 必须红（余量不是「裁了」）
	if err := zerg.ImpactCardJudgeForTest(f3); err == nil {
		t.Error("负控失败：`truncated=false` 却留着三件竟判过")
	}
	// 负控（置顶）：把一条非会红条目挪到最前 ⇒ 判定口必须红。
	shuffled := c
	shuffled.Items = append([]map[string]string{impactCardRow("文件级:no-red.go:1", "词法", "h", "")},
		append([]map[string]string{}, c.Items...)...)
	if err := zerg.ImpactCardJudgeForTest(shuffled); err == nil {
		t.Error("负控失败：会红条目被非会红条目压在下面竟判过 —— 「置顶」这一格没牙")
	}
}

// TestImpactCard_FourLevelOrder —— §3.2 四级排序（全是整数比较）：① 会红 → ② 图距离 1 跳
// （直接调用者）→ ③ 词法命中数（同件命中条数）→ ④ 义近分（今天恒 0，只作同分 tie-break）。
func TestImpactCard_FourLevelOrder(t *testing.T) {
	rows := []map[string]string{
		impactCardRow("文件级:x.go:1", "词法", "h", ""),  // 词法命中数 1（同一件 1 条）
		impactCardRow("符号级:Caller", "调用边", "h", ""), // 图距离 1 跳 ⇒ 排在词法之前
		impactCardRow("包级:P", "包反向", "h", ""),       // 无图距离（哨兵 99）且词法命中 0 ⇒ 最后
	}
	c := zerg.ImpactCardOfForTest(rows, "", "t")
	if len(c.Items) != 3 {
		t.Fatalf("条数 = %d（要 3）", len(c.Items))
	}
	got := []string{c.Items[0]["why"], c.Items[1]["why"], c.Items[2]["why"]}
	want := []string{"调用边", "词法", "包反向"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("四级排序不对：得 %v（要 %v —— ② 图距离 1 跳在 ③ 词法命中数之前；④ 义近分今天恒 0）", got, want)
		}
	}
	if c.Dist1N != 1 || c.SimN != 0 || c.LexMax != 1 {
		t.Errorf("排序账不对：图距离1跳=%d（要 1）· 义近分命中=%d（要 0 · ⑤ 未建索引）· 词法命中数max=%d（要 1）",
			c.Dist1N, c.SimN, c.LexMax)
	}
	// ④ 义近分（今天恒 0）：义近条目在**同 red、同图距离、同词法命中数**时只按 `what` 定序，
	// 不因为「分」插队 —— 口径写明，不编分。
	sim := zerg.ImpactCardOfForTest([]map[string]string{
		impactCardRow("件级:b", "义近", "h", ""),
		impactCardRow("件级:a", "义近", "h", ""),
	}, "", "t")
	if len(sim.Items) != 2 || sim.Items[0]["what"] != "件级:a" {
		t.Errorf("义近分今天恒 0 ⇒ 同分按 `what` 定序（要 件级:a 在前）：%+v", sim.Items)
	}
}

// TestImpactReversibility_ThreeTiersAndExecutability —— 判据④（`v1.6` §九 判据⑨）：
// 档 1 那条命令要能在盘上解析到目标；档 3 行内必须出现「无退法」三字；负控：把该行清空 ⇒ 必须红。
func TestImpactReversibility_ThreeTiersAndExecutability(t *testing.T) {
	root := repoRootFromCLI(t)

	// 档 1（**真实盘上**）：`bin/zerg-core` 是 `bin/` 产物（不入库）且 `bin/_history/` 里有现成回滚件。
	r1 := zerg.ImpactReversibilityOfForTest(root, "bin/zerg-core")
	if r1.Tier != 1 {
		t.Errorf("`bin/zerg-core` 的退法档 = %d（要 1：现成回滚件在场）· line=%s", r1.Tier, r1.Line)
	}
	if len(r1.Resolve) == 0 || !strings.Contains(r1.Cmd, "cp -p") {
		t.Errorf("档 1 必须写**那条命令**并给出要解析的路径：cmd=%q resolve=%v", r1.Cmd, r1.Resolve)
	}
	if err := zerg.ImpactJudgeReversibilityForTest(root, r1); err != nil {
		t.Errorf("判据④ 把真档 1 判红了：%v", err)
	}
	// 档 1 负控：回滚件指向盘上没有的路径 ⇒ 必须红（`test -e` 那条）。
	bad1 := r1
	bad1.Resolve = []string{"bin/_history/没有这一件.prev", "bin/zerg-core"}
	if err := zerg.ImpactJudgeReversibilityForTest(root, bad1); err == nil {
		t.Error("负控失败：档 1 的回滚件在盘上取不到竟判过 —— 「能在盘上解析到」这一格没牙")
	}

	// 档 2（**真实盘上**）：已提交的件 ⇒ 证据 = git 提交 sha + 件路径（可复算）。
	r2 := zerg.ImpactReversibilityOfForTest(root, "core/cmd/zerg/main.go")
	if r2.Tier != 2 {
		t.Fatalf("`core/cmd/zerg/main.go` 的退法档 = %d（要 2：无回滚件、但有 git 那笔提交）", r2.Tier)
	}
	if len(r2.SHA) < 8 {
		t.Errorf("档 2 的 sha 太短（要能复算）：%q", r2.SHA)
	}
	if err := zerg.ImpactJudgeReversibilityForTest(root, r2); err != nil {
		t.Errorf("判据④ 把真档 2 判红了：%v", err)
	}
	if out, err := exec.Command("git", "-C", root, "cat-file", "-e", r2.SHA).CombinedOutput(); err != nil {
		t.Errorf("档 2 的证据 sha 复算不过（`git cat-file -e %s`）：%v %s", r2.SHA, err, out)
	}

	// 档 3：既没有现成回滚件、也没有可找回证据（未跟踪 / 无提交）⇒ **必须明写「无退法」**。
	r3 := zerg.ImpactReversibilityDecideForTest(false, "", "", "core/cmd/zerg/一个还没提交的新件.go")
	if r3.Tier != 3 {
		t.Fatalf("档 3 判定口给出 %d", r3.Tier)
	}
	if !strings.Contains(r3.Line, "无退法") {
		t.Errorf("档 3 行内没有「无退法」三字：%q", r3.Line)
	}
	if err := zerg.ImpactJudgeReversibilityForTest(root, r3); err != nil {
		t.Errorf("判据④ 把档 3 判红了：%v", err)
	}
	// 负控：把档 3 那一行**清空** ⇒ 本判据必须红（§九 判据⑨ 逐字）。
	empty := r3
	empty.Line = ""
	if err := zerg.ImpactJudgeReversibilityForTest(root, empty); err == nil {
		t.Error("负控失败：档 3 的行被清空竟判过 —— 「空着 = 红」这一格没牙")
	}
	// 负控b：档 3 写成「建议谨慎」而不写「无退法」⇒ 也必须红。
	vague := r3
	vague.Line = "这一步能退吗？建议谨慎。"
	if err := zerg.ImpactJudgeReversibilityForTest(root, vague); err == nil {
		t.Error("负控失败：档 3 写成「建议谨慎」竟判过 —— 措辞必须逐字「无退法」")
	}
}

// TestImpactCard_HumanFaceStaysThreeLinesAndCarriesReversibility —— 人面**恒三行**一字不改
// （`A1` 判据③ · `A3` 把 §3.8 的可逆性**内联在第③行同一行**，不新增行数）+ 卡片块落 stderr
// （`--for-human` 出人档全文 · `--for-model` 明说同效 · `--json` 可叠加）。
func TestImpactCard_HumanFaceStaysThreeLinesAndCarriesReversibility(t *testing.T) {
	root := repoRootFromCLI(t)
	t.Setenv("ZERG_REPO", root)
	// `A5`：`zerg impact` 自本批起**会落盘缓存**（落点在状态目录）⇒ 测试一律把状态目录改到合成目录，
	// **不碰真状态目录**（本件判据一个字不动；不这么做就是「测试有副作用」）。
	t.Setenv("ZERG_STATE_DIR", t.TempDir())
	tgt := "core/cmd/zerg/main.go"

	rc, out, errb := runCapture("impact", tgt)
	if rc != 0 {
		t.Fatalf("期望 rc=0，得到 %d · stderr=%s", rc, errb)
	}
	if err := judgeImpactHuman(out, impactHeads); err != nil {
		t.Errorf("判据③ 破（人面恒三行）：%v · stdout=%q", err, out)
	}
	l3 := lineOf(out, impactHeads[2])
	if !strings.Contains(l3, "能退吗？") {
		t.Errorf("第③行没带可逆性那一问（§3.8：建议行内给出下一步命令的**同一行**）：%s", l3)
	}
	if !strings.Contains(l3, "git revert") && !strings.Contains(l3, "cp -p") && !strings.Contains(l3, "无退法") {
		t.Errorf("第③行的退法既不是那条命令、也不是「无退法」：%s", l3)
	}
	// 卡片块（stderr）：铁律 + 三件那一行 + 排序/裁/账 + 退法。
	for _, want := range []string{"卡片", "永不构成批准", "提 ≠ 批", "truncated=", "meta.how_to_restore=",
		"卡片退法", "卡片排序（§3.2 四级整数比较", "卡片裁（§3.3"} {
		if !strings.Contains(errb, want) {
			t.Errorf("卡片块里没有 %q", want)
		}
	}
	// 两档：`--for-human` ⇒ 人档全文；`--for-model` ⇒ 与默认同效（照实明说）。
	_, outH, errH := runCapture("impact", tgt, "--for-human")
	if !strings.Contains(errH, "人档全文") || !strings.Contains(errH, "不限长") {
		t.Errorf("`--for-human` 没给人档全文（§4.1）：%s", lineOf(errH, "卡片（"))
	}
	if n := countNonEmpty(outH); n != 3 {
		t.Errorf("人档仍是人面恒三行（stdout 不许因为开档多出行）：实得 %d 行", n)
	}
	_, _, errM := runCapture("impact", tgt, "--for-model")
	if !strings.Contains(errM, "同效") {
		t.Error("`--for-model` 没明说「与默认档同效」（照实明说，不静默）")
	}
	// `R38`：两档与 `--json` **不是同一条**、**可叠加**，且都不改六键包封。
	rc3, outJ, errJ := runCapture("impact", tgt, "--for-human", "--json", "what,why")
	if rc3 != 0 {
		t.Fatalf("`--for-human` + `--json` 可叠加（§4.1/R38）⇒ 期望 rc=0，得到 %d · stderr=%s", rc3, errJ)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal([]byte(outJ), &doc); err != nil {
		t.Fatalf("叠加时机器面不是包封：%v · out=%q", err, outJ)
	}
	if err := judgeImpactEnvelopeAny(doc); err != nil {
		t.Errorf("叠加时六键形状破：%v", err)
	}
	// K2 仍在两档上生效：`--for-human --json` 不给字段 ⇒ 退 1 + stdout 0 字节。
	rc4, out4, _ := runCapture("impact", tgt, "--for-human", "--json")
	if rc4 != 1 || len(out4) != 0 {
		t.Errorf("K2 在两档上仍生效（§4.1：两档与 --json 是两条）：rc=%d · %d 字节", rc4, len(out4))
	}
}

// judgeImpactEnvelopeAny 只判「六键恒在 · 一个不多一个不少」（条数由卡片尺子管，不在这里判值）。
func judgeImpactEnvelopeAny(doc map[string]json.RawMessage) error {
	for _, k := range impactEnvelopeKeys {
		if _, ok := doc[k]; !ok {
			return errStr("包封缺键 " + k)
		}
	}
	for k := range doc {
		found := false
		for _, want := range impactEnvelopeKeys {
			if k == want {
				found = true
			}
		}
		if !found {
			return errStr("包封多了未声明键 " + k)
		}
	}
	return nil
}
