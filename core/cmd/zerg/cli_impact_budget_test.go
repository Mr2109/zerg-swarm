// cli_impact_budget_test.go —— `zerg impact` 的 **`B4` 分层预算与降级**判据
// （任务单-影响面实施-20260922 §三 `B4` 八字段判据①–④ + 四条红线 · 设计-变更影响面-v1.6 §4.4 / §7.4）。
//
// 落在 `package main_test`（§九 M17 三层测试落点的层①②）：`RunForTest` 跑的是**当前源码**的
// 运行期行为，不是盘上旧制品。
//
// 四格判据的机检口（**都在本文件里真跑**）：
//
//	① **降级必须留痕**：`meta.layers_not_run[]` 里逐条点名（**含 `runtime` 恒在**）—— 正控（真仓现跑）
//	   + 成对负控（把 `runtime` 那一行 / 层表行数 / 降级掉的层各改一处 ⇒ 判定口必须报错）。
//	② **超时即降**到「编译器层 + 词法层」，`warnings[]` 里写明**哪几层没跑** —— 负控 = **人为压上限**
//	   （真仓镜像根 + 一份只改了 `实测输入` 的契约件）⇒ 必须降级且留痕（③⑤⑥ 未跑 · ④ 照跑）。
//	③ **缺档位的耗时不许进预算裁决** —— 缺 `head_sha` / 缓存态=未测 / 缺件三种合成件各配一条负控，
//	   另有**正控**（真契约件 ⇒ 两式都进裁决）证明这一格不是恒假。
//	④ **`N` 的绝对值仍待拍** ⇒ 实现里 **0 个阈值数**：静态自检（上限相关的数一个都不许出现在实现件里）
//	   + 与契约件**对拍**（乘数 × 实测输入 == 契约件记的 `算出值秒`）。
//
// 另加红线的机检：实现件里**没有删除动作 / 没有自动收窄·自动删·自动回滚串 / 没写盘**；
// 降级前后**层表恒六行**（形状只改内容与标注）；**缓存与降级不改答案**（两跑里都跑到的层逐字相同）。
package main_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"unicode"

	zerg "github.com/Mr2109/zerg-swarm/core/cmd/zerg"
)

// ---- 判定口（判据①②③ 的唯一判定口 · 负控直接喂改过的副本 ⇒ 必须报错）----------------------

// judgeImpactBudgetTrace 判「一份预算留痕 + 一份层表」成不成形。
//
// 判四件（判据① 的两半 + 形状不变 + 降级后逐条点名）：
//
//	· `meta.layers_not_run[]` 的那一行在（留痕有落点）；
//	· `runtime` 恒在那一条在（§7.4 固定名单）；
//	· 层表**恰六行**且层序逐行是 ①–⑥（降级前后形状不变）；
//	· 凡状态=未跑（超预算降级）的层，逐条在留痕里出现过（**不许悄悄少给**）。
func judgeImpactBudgetTrace(block string, rows []zerg.ImpactLayerViewForTest) error {
	if !strings.Contains(block, "`meta.layers_not_run[]`") {
		return errStr("留痕块里没有 `meta.layers_not_run[]` 那一格（判据① 没有落点）")
	}
	if !strings.Contains(block, "runtime（运行期面 · 恒在）") {
		return errStr("`meta.layers_not_run[]` 里没点名 `runtime`（§7.4：固定名单**恒在**）")
	}
	if len(rows) != 6 {
		return errStr(fmt.Sprintf("层表不是六行（实得 %d）—— 降级前后形状必须不变", len(rows)))
	}
	want := []string{"①", "②", "③", "④", "⑤", "⑥"}
	for i, r := range rows {
		if r.Seq != want[i] {
			return errStr(fmt.Sprintf("层表第 %d 行的层序是 %q（要 %q）—— 层序即命令里的顺序", i+1, r.Seq, want[i]))
		}
	}
	for _, r := range rows {
		if r.Status != zerg.ImpactBudgetDegradedStatusForTest() {
			continue
		}
		if !strings.Contains(block, r.Seq+" ") {
			return errStr(fmt.Sprintf("层%s 标了 %q 却没在留痕里逐条点名（悄悄少给）", r.Seq, r.Status))
		}
	}
	return nil
}

// ---- 合成件（不碰真仓：判据③ 的负控全部喂合成契约件）---------------------------------------

// budgetContractFix 拿真契约件的字节，按改法改一份副本（负控用）。
func budgetContractFix(t *testing.T, fix func(m map[string]any)) []byte {
	t.Helper()
	raw, err := zerg.ImpactBudgetContractBytesForTest()
	if err != nil {
		t.Fatalf("读真契约件失败：%v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("真契约件解不开：%v", err)
	}
	fix(m)
	out, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("改过的契约件拼不回去：%v", err)
	}
	return out
}

// budgetMeasured 取 `cap.实测输入` 里的某一项（判据③ 的三件就在这里）。
func budgetMeasured(m map[string]any, key string) map[string]any {
	return m["cap"].(map[string]any)["实测输入"].(map[string]any)[key].(map[string]any)
}

// budgetMirror 造一棵**真仓镜像根**（符号链接；只把 `impact-budget.json` 换成压过上限的那一份）。
//
// 为什么不用纯合成仓：判据② 要证的是「**超时即降到编译器层 + 词法层**」——这要求 ①④ 两层在真语料上
// 仍能取到数（合成仓里 ① 找不到 module、④ 找不到 tracked 件 ⇒ 降到哪一档都看不出）。镜像根把真仓的
// 件树与 `.git` 链进来、只**换掉契约件那一件** ⇒ 真降级发生在真取数上，且**不动真仓一个字节**。
func budgetMirror(t *testing.T, fix func(m map[string]any)) string {
	t.Helper()
	real := repoRootFromCLI(t)
	root := t.TempDir()
	link := func(rel string) {
		dst := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			t.Fatalf("合成目录造不出来：%v", err)
		}
		if err := os.Symlink(filepath.Join(real, filepath.FromSlash(rel)), dst); err != nil {
			t.Fatalf("软链造不出来（%s）：%v", rel, err)
		}
	}
	if _, err := os.Stat(filepath.Join(real, ".git")); err == nil {
		link(".git")
	}
	for _, d := range []string{"agent", "scripts", "gateway", "publish", "wall", "ui", "tools", "docs", "shared", "data", "deploy", "styles", "mcp"} {
		if _, err := os.Stat(filepath.Join(real, d)); err == nil {
			link(d)
		}
	}
	// core/* 逐件链（除 internal）· core/internal/* 逐件链（除 contract）· contract/* 逐件链（除预算件）。
	for _, top := range []struct{ dir, skip string }{
		{"core", "internal"},
		{"core/internal", "contract"},
		{"core/internal/contract", "impact-budget.json"},
	} {
		ents, err := os.ReadDir(filepath.Join(real, filepath.FromSlash(top.dir)))
		if err != nil {
			t.Fatalf("列 %s 失败：%v", top.dir, err)
		}
		for _, e := range ents {
			if e.Name() == top.skip {
				continue
			}
			link(top.dir + "/" + e.Name())
		}
	}
	// 只换这一件：契约件（压上限的那一份）。
	raw := budgetContractFix(t, fix)
	if err := os.WriteFile(filepath.Join(root, "core", "internal", "contract", "impact-budget.json"), raw, 0o644); err != nil {
		t.Fatalf("镜像根里写契约件失败：%v", err)
	}
	return root
}

// budgetPushCap 把上限压成一个小额度（**人为压上限** = 判据② 的负控）。
//
// ★ 计数随动（2026-09-22 · `R32` 已定）：本跑上限的第一来路是 `cap.上限定值`（已定值）⇒
// 要真压住上限，**定值那一格也必须一起压**（只压实测输入 = 压不动 ⇒ 那一格会变成假负控）。
// 两式一并压，是为了让「回落路径」上的那一份也仍然压得住（定值缺时走它）。
func budgetPushCap(sec float64) func(m map[string]any) {
	return func(m map[string]any) {
		for _, k := range []string{"全层现算实测", "最贵单层现跑值"} {
			m["cap"].(map[string]any)["实测输入"].(map[string]any)[k].(map[string]any)["取值秒"] = sec
		}
		if fx, ok := m["cap"].(map[string]any)["上限定值"].(map[string]any); ok {
			fx["值秒"] = sec
		}
	}
}

// ---- ① 降级必须留痕（正控：真仓现跑）-------------------------------------------------------

// TestImpactBudget_TraceAlwaysNamesRuntime —— 判据① 的**正控**：真仓两档各跑一次，
// `meta.layers_not_run[]` 恒在 `runtime`，层表恒六行，没跑的层逐条点名；两档都**不降级**。
func TestImpactBudget_TraceAlwaysNamesRuntime(t *testing.T) {
	requireDeep(t) // 贵档闸 · 现读见本件头：ZERG_DEEP=1 才跑
	root := repoRootFromCLI(t)
	t.Setenv("ZERG_REPO", root)
	t.Setenv("ZERG_STATE_DIR", t.TempDir())
	tgt := "core/cmd/zerg/main.go"

	// CLI 通路（证明这一块真的挂在 `zerg impact` 的 stderr 上 · 只跑一次省时）。
	rc, _, errb := runCapture("impact", tgt)
	if rc != 0 {
		t.Fatalf("默认档 ⇒ 期望 rc=0，得到 %d · stderr=%s", rc, tail(errb, 400))
	}
	if !strings.Contains(errb, "`B4` 分层预算与降级") {
		t.Errorf("`zerg impact` 的 stderr 里没有预算块（没挂上）：%s", tail(errb, 400))
	}

	for _, tc := range []struct {
		name     string
		all      bool
		wantTier string
	}{
		{"默认档", false, "默认档"},
		{"按需档", true, "按需档"},
	} {
		view, err := zerg.ImpactLayersForTest(root, tgt, tc.all)
		if err != nil {
			t.Fatalf("%s ⇒ 跑链失败：%v", tc.name, err)
		}
		if err := judgeImpactBudgetTrace(view.Block, view.Layers); err != nil {
			t.Fatalf("%s ⇒ 判据① 破：%v", tc.name, err)
		}
		if view.Run.Degraded {
			t.Errorf("%s ⇒ 真仓两档都**不该降级**（上限来自实测分布 · 本跑实得上限 %.3fs）：%s",
				tc.name, view.Run.CapSec, view.Run.Trigger)
		}
		if !strings.Contains(view.Block, "式①") || !strings.Contains(view.Block, "式②") {
			t.Errorf("%s ⇒ 预算块里该把**两式**都打出来（判据④「只落两式」）：%s", tc.name, tail(view.Block, 600))
		}
		if !strings.Contains(view.Block, "裁决外：①编译层") && !strings.Contains(view.Block, "裁决外：①编译器层：**不进裁决**") {
			t.Errorf("%s ⇒ ① 层该被逐条点名「不进裁决」（判据③）：%s", tc.name, tail(view.Block, 800))
		}
		// 没跑的层逐条在留痕里（默认档里 = ② + ⑤ / 按需档里 = ⑤）。
		for _, l := range view.Layers {
			if l.Status == "未跑" || l.Status == "未建索引" {
				if !strings.Contains(view.Block, l.Seq+" ") {
					t.Errorf("%s ⇒ 层%s（%s）没在留痕里点名", tc.name, l.Seq, l.Status)
				}
			}
		}
		// 负控 ①：留痕里抹掉 `runtime` 那一行 ⇒ 判定口必须报错（证明这一格有牙）。
		if err := judgeImpactBudgetTrace(strings.ReplaceAll(view.Block,
			"runtime（运行期面 · 恒在）", "（本跑抹掉了这一行）"), view.Layers); err == nil {
			t.Error("负控①失败：抹掉 `runtime` 那一行竟判「成形」")
		}
		// 负控 ②：层表少一行 ⇒ 判定口必须报错（形状不变那一格）。
		if err := judgeImpactBudgetTrace(view.Block, view.Layers[:5]); err == nil {
			t.Error("负控②失败：层表只给五行竟判「成形」—— 形状不变这一格没牙")
		}
		// 负控 ③：把某一层的层序改掉 ⇒ 判定口必须报错。
		bad := append([]zerg.ImpactLayerViewForTest{}, view.Layers...)
		bad[3].Seq = "⑦"
		if err := judgeImpactBudgetTrace(view.Block, bad); err == nil {
			t.Error("负控③失败：层序改了竟判「成形」")
		}
		// 负控 ④：做一份「降级了但留痕里没有那一层」的副本 ⇒ 判定口必须报错。
		degraded := append([]zerg.ImpactLayerViewForTest{}, view.Layers...)
		degraded[2].Status = zerg.ImpactBudgetDegradedStatusForTest()
		degraded[2].Seq = "⑨"
		if err := judgeImpactBudgetTrace(view.Block, degraded); err == nil {
			t.Error("负控④失败：降级掉的层没在留痕里点名竟判「成形」（悄悄少给）")
		}
	}
}

// ---- ② 超时即降（负控：人为压上限 ⇒ 必须降级且留痕）---------------------------------------

// TestImpactBudget_DegradeUnderPressedCap —— 判据①② 的**负控**：真仓镜像根 + 压过的契约件。
//
// 期望：③⑤⑥ 标 `未跑（超预算降级）`、④ **照跑**（降级档 = ①④）、① 不降级、层表恒六行、
// `warnings[]` 逐条写明哪几层没跑；退码与「不降级那一跑」**同码**（降级不改退码）。
func TestImpactBudget_DegradeUnderPressedCap(t *testing.T) {
	requireDeep(t) // 贵档闸 · 现读见本件头：ZERG_DEEP=1 才跑
	real := repoRootFromCLI(t)
	tgt := "core/cmd/zerg/main.go"
	// 比对照跑：镜像根 + **不压**上限（同一棵树，只差契约件那一格）。
	tall := budgetMirror(t, func(map[string]any) {})
	tshort := budgetMirror(t, budgetPushCap(0.0005))

	run := func(root string) (int, string, string, zerg.ImpactLayersViewForTest) {
		t.Helper()
		t.Setenv("ZERG_REPO", root)
		t.Setenv("ZERG_STATE_DIR", t.TempDir())
		rc, out, errb := runCapture("impact", tgt, "--all")
		v, err := zerg.ImpactLayersForTest(root, tgt, true)
		if err != nil {
			t.Fatalf("跑链失败（%s）：%v", root, err)
		}
		return rc, out, errb, v
	}
	rcA, outA, errA, viewA := run(tall)
	rcB, _, errB, viewB := run(tshort)
	if rcA != 0 || rcB != 0 {
		t.Fatalf("两跑都该 rc=0（降级**不改退码**）：%d / %d", rcA, rcB)
	}
	if viewA.Run.Degraded {
		t.Fatalf("对照跑不该降级：上限=%.3fs · %s", viewA.Run.CapSec, viewA.Run.Trigger)
	}
	if !viewB.Run.Degraded {
		t.Fatalf("压上限那一跑**必须降级**：上限=%.3fs 已计时=%.3fs", viewB.Run.CapSec, viewB.Run.Charged)
	}
	if err := judgeImpactBudgetTrace(viewB.Block, viewB.Layers); err != nil {
		t.Fatalf("判据①② 破（负控那一跑）：%v", err)
	}
	bytier := map[string]string{}
	for _, l := range viewB.Layers {
		bytier[l.Seq] = l.Status
	}
	// 降级档（契约件 = ①④）**照跑**；不在名单里的层（③⑤⑥）**不跑**并逐条点名。
	for _, seq := range []string{"③", "⑤", "⑥"} {
		if bytier[seq] != zerg.ImpactBudgetDegradedStatusForTest() {
			t.Errorf("层%s 该标 %q，实得 %q（§4.4：超时只给编译器层 + 词法层）",
				seq, zerg.ImpactBudgetDegradedStatusForTest(), bytier[seq])
		}
	}
	if bytier["④"] == zerg.ImpactBudgetDegradedStatusForTest() {
		t.Error("层④ 在降级档名单里 ⇒ 该**照跑**（§4.4：降到「编译器层 + 词法层」），实得「未跑」")
	}
	if bytier["①"] == zerg.ImpactBudgetDegradedStatusForTest() {
		t.Error("层① 在降级档名单里 ⇒ 该照跑，实得「未跑」")
	}
	// `warnings[]` 与 `meta.layers_not_run[]` 是**同一份取值**。
	for _, seq := range []string{"③", "⑤", "⑥"} {
		if !strings.Contains(viewB.Block, seq+" ") {
			t.Errorf("层%s 没在留痕/`warnings[]` 里点名（判据②「写明哪几层没跑」）", seq)
		}
	}
	warnLine := ""
	for _, ln := range strings.Split(viewB.Block, "\n") {
		if strings.Contains(ln, "`warnings[]`（判据②") {
			warnLine = ln
		}
	}
	if warnLine == "" {
		t.Fatal("预算块里没有 `warnings[]` 那一行（判据② 没有落点）")
	}
	for _, seq := range []string{"③", "⑤", "⑥"} {
		if !strings.Contains(warnLine, seq+" ") {
			t.Errorf("`warnings[]` 里没写明层%s 没跑：%s", seq, tail(warnLine, 300))
		}
	}
	// **缓存与降级不改答案**：两跑里**都跑到**的层（状态都不是降级那一档）条目逐字相同。
	same := 0
	for i, la := range viewA.Layers {
		lb := viewB.Layers[i]
		if la.Seq != lb.Seq {
			t.Fatalf("两跑层序不同：%s vs %s", la.Seq, lb.Seq)
		}
		if la.Status == zerg.ImpactBudgetDegradedStatusForTest() || lb.Status == zerg.ImpactBudgetDegradedStatusForTest() {
			continue
		}
		if la.RowsJSON != lb.RowsJSON {
			t.Errorf("层%s 两跑的条目**逐字不同**（降级改答案了）：\nA=%s\nB=%s", la.Seq, tail(la.RowsJSON, 300), tail(lb.RowsJSON, 300))
		}
		if la.Status == lb.Status {
			same++
		}
	}
	if same == 0 {
		t.Error("两跑里没有一个「都跑到」的层 ⇒ 这一格判不了（空与空比不算比）")
	}
	// 人面三行：降级前后都要在（形状不变），且两跑的骨架字头一致。
	head := func(out string) []string {
		ls := strings.Split(strings.TrimRight(out, "\n"), "\n")
		if len(ls) > 3 {
			ls = ls[:3]
		}
		return ls
	}
	for i, ln := range head(outA) {
		want := []string{"会牵动：", "会红：", "建议："}[i]
		if !strings.HasPrefix(ln, want) {
			t.Errorf("人面第 %d 行不是 %q：%s", i+1, want, ln)
		}
	}
	_ = errA
	_ = errB
	_ = real
}

// ---- ③ 缺档位的耗时不许进预算裁决（正控 + 三条负控）· ④ 与契约件对拍 ------------------------

// TestImpactBudget_CapAdmissionAndFiledMath —— 判据③④ 的机检：喂合成契约件。
func TestImpactBudget_CapAdmissionAndFiledMath(t *testing.T) {
	// 正控：真契约件 ⇒ 两式都进裁决，生效式 = 契约件写的那一格（取法归契约件）。
	real, err := zerg.ImpactBudgetContractBytesForTest()
	if err != nil {
		t.Fatalf("读真契约件失败：%v", err)
	}
	p := zerg.ImpactBudgetPlanBytesForTest(real)
	if !p.OK || !p.Armed {
		t.Fatalf("真契约件该裁：ok=%v armed=%v why=%s", p.OK, p.Armed, p.Why)
	}
	if len(p.Formulas) != 2 {
		t.Fatalf("契约件写了两式，实得 %d", len(p.Formulas))
	}
	for i, f := range p.Formulas {
		if !f.Admit {
			t.Errorf("正控失败：式%d 该进裁决，实得「不进裁决（%s）」", i+1, f.Why)
		}
	}
	pick := ""
	for _, f := range p.Formulas {
		pick = f.ID
		if f.Value > p.CapSec || strings.Contains(pick, "-") {
			t.Errorf("式%s 的算出值 %.6f 与生效上限 %.6f 不成形", f.ID, f.Value, p.CapSec)
		}
	}
	want := "max"
	if p.Pick != want {
		t.Errorf("生效式取法该从契约件读（要 %q，实得 %q）", want, p.Pick)
	}
	if len(p.Allow) == 0 {
		t.Error("降级档层名单该从契约件读（实得空）")
	}
	// 正控（`R32` 已拍 · Mr2109 2026-09-22 定「取大」）：上限来自契约件 `cap.上限定值`，
	// 值逐字等于契约件那一格，出处逐字回引。
	var fx struct {
		Cap struct {
			Fixed struct {
				Value  float64 `json:"值秒"`
				Origin string  `json:"出处"`
			} `json:"上限定值"`
		} `json:"cap"`
	}
	if err := json.Unmarshal(real, &fx); err != nil {
		t.Fatalf("真契约件解不开（定值那一格）：%v", err)
	}
	if !(fx.Cap.Fixed.Value > 0) {
		t.Fatalf("真契约件没写 `cap.上限定值.值秒`（`R32` 已定 ⇒ 这一格必须是有值的定值）")
	}
	if !p.Fixed {
		t.Errorf("正控失败：真契约件有定值 %.6f s 却没被采用（%s）", fx.Cap.Fixed.Value, p.FixedWhy)
	}
	if d := p.CapSec - fx.Cap.Fixed.Value; d > 1e-9 || d < -1e-9 {
		t.Errorf("正控失败：生效上限 %.6f s ≠ 契约件定值 %.6f s", p.CapSec, fx.Cap.Fixed.Value)
	}
	if p.FixedOrig != fx.Cap.Fixed.Origin {
		t.Errorf("正控失败：定值出处没逐字回引契约件（要 %q，实得 %q）", fx.Cap.Fixed.Origin, p.FixedOrig)
	}
	// ④ 对拍：契约件里记的 `算出值秒` 必须等于 乘数 × 实测输入取值（两边都由实现算，不心算）。
	for _, f := range p.Formulas {
		if !f.MatchesFixed {
			t.Errorf("式%s 与契约件记的 `算出值秒` **对不上**：算出 %.6f · 契约件记 %.6f", f.ID, f.Value, f.Fixed)
		}
	}

	// 负控 A：某式输入缺 `head_sha` ⇒ **那一式**不进裁决（判据③）。
	mA := budgetContractFix(t, func(m map[string]any) { budgetMeasured(m, "最贵单层现跑值")["head_sha"] = "" })
	pA := zerg.ImpactBudgetPlanBytesForTest(mA)
	nA := 0
	for _, f := range pA.Formulas {
		if !f.Admit {
			nA++
			if !strings.Contains(f.Why, "三件不齐") {
				t.Errorf("负控A：不进裁决的原因该写「三件不齐」，实得 %q", f.Why)
			}
		}
	}
	if nA != 1 {
		t.Errorf("负控A：缺一件 ⇒ 该恰好**一式**不进裁决，实得 %d", nA)
	}
	// 负控 B：某式输入缓存态=未测 ⇒ 那一式不进裁决。
	mB := budgetContractFix(t, func(m map[string]any) { budgetMeasured(m, "全层现算实测")["缓存态"] = "未测" })
	pB := zerg.ImpactBudgetPlanBytesForTest(mB)
	if !pB.Formulas[0].Admit && !strings.Contains(pB.Formulas[0].Why, "未测") {
		t.Errorf("负控B：缓存态=未测该写明原因，实得 %q", pB.Formulas[0].Why)
	}
	admB := 0
	for _, f := range pB.Formulas {
		if f.Admit {
			admB++
		}
	}
	if admB != 1 {
		t.Errorf("负控B：只有另一式该进裁决，实得 %d 式", admB)
	}
	// 负控 C：两式都不进裁决 ⇒ **不裁**（不许拿 0 当上限 ⇒ 一次都不该降级）。
	// ★ 计数随动（2026-09-22）：本格自 `R32` 拍定起带**已定值** ⇒ 这一格测的是**回落路径**
	// （定值缺 + 两式都不进裁决 ⇒ 不裁），故先把 `cap.上限定值` 拿掉再跑。
	mC := budgetContractFix(t, func(m map[string]any) {
		delete(m["cap"].(map[string]any), "上限定值")
		budgetMeasured(m, "全层现算实测")["head_sha"] = ""
		budgetMeasured(m, "最贵单层现跑值")["缓存态"] = "未测"
	})
	pC := zerg.ImpactBudgetPlanBytesForTest(mC)
	if !pC.OK || pC.Armed {
		t.Errorf("负控C：两式都不进裁决 ⇒ 该 OK=true / Armed=false（不裁），实得 ok=%v armed=%v", pC.OK, pC.Armed)
	}
	if !strings.Contains(pC.Why, "不裁") {
		t.Errorf("负控C：不裁要写明原因，实得 %q", pC.Why)
	}
	// 负控 D：取法不在闭集 ⇒ 不裁（不许自选一个默认取法）。★ 同样走回落路径（先拿掉定值）。
	mD := budgetContractFix(t, func(m map[string]any) {
		delete(m["cap"].(map[string]any), "上限定值")
		m["cap"].(map[string]any)["生效式取法"] = "mean"
	})
	pD := zerg.ImpactBudgetPlanBytesForTest(mD)
	if pD.Armed {
		t.Error("负控D：取法不在闭集竟还裁了（自选默认取法 = 越线）")
	}
	if !strings.Contains(pD.Why, "闭集") {
		t.Errorf("负控D：不裁要写明原因，实得 %q", pD.Why)
	}
	// 负控 D2（新格 · `R32` 已定后）：**定值自己**的取法不在闭集 ⇒ 不裁（不许自选、也不许悄悄回落）。
	mD2 := budgetContractFix(t, func(m map[string]any) {
		m["cap"].(map[string]any)["上限定值"].(map[string]any)["取法"] = "mean"
	})
	pD2 := zerg.ImpactBudgetPlanBytesForTest(mD2)
	if pD2.Armed {
		t.Error("负控D2：定值的取法不在闭集竟还裁了（判不了就该不裁）")
	}
	if !strings.Contains(pD2.Why, "上限定值.取法") || !strings.Contains(pD2.Why, "闭集") {
		t.Errorf("负控D2：不裁要写明是哪一格的取法漂了，实得 %q", pD2.Why)
	}
	// 负控 F（新格）：定值 `值秒` ≤ 0 ⇒ **回落**到两式取（且写明换档，不静默），上限 = 两式取大。
	mF := budgetContractFix(t, func(m map[string]any) {
		m["cap"].(map[string]any)["上限定值"].(map[string]any)["值秒"] = 0
	})
	pF := zerg.ImpactBudgetPlanBytesForTest(mF)
	if pF.Fixed {
		t.Error("负控F：定值非正竟还被采用")
	}
	if !pF.OK || !pF.Armed {
		t.Errorf("负控F：定值非正 ⇒ 该回落到两式取（armed），实得 ok=%v armed=%v why=%s", pF.OK, pF.Armed, pF.Why)
	}
	if !strings.Contains(pF.FixedWhy, "没被采用") || !strings.Contains(pF.FixedWhy, "回落") {
		t.Errorf("负控F：换档要写明（不静默），实得 %q", pF.FixedWhy)
	}
	fmax := 0.0
	for _, f := range pF.Formulas {
		if f.Admit && f.Value > fmax {
			fmax = f.Value
		}
	}
	if d := pF.CapSec - fmax; d > 1e-9 || d < -1e-9 {
		t.Errorf("负控F：回落后的上限该 = 两式取大 %.6f，实得 %.6f", fmax, pF.CapSec)
	}
	// 负控 G（新格）：定值 `出处` 为空 ⇒ 回落（出处是「谁定的」那一格，缺了就不许当已定值用）。
	mG := budgetContractFix(t, func(m map[string]any) {
		m["cap"].(map[string]any)["上限定值"].(map[string]any)["出处"] = ""
	})
	pG := zerg.ImpactBudgetPlanBytesForTest(mG)
	if pG.Fixed {
		t.Error("负控G：定值出处为空竟还被采用")
	}
	if !strings.Contains(pG.FixedWhy, "没被采用") {
		t.Errorf("负控G：换档要写明，实得 %q", pG.FixedWhy)
	}
	// 负控 E：契约件读不到（形状号不认）⇒ 不裁。
	mE := budgetContractFix(t, func(m map[string]any) { m["schema"] = "zerg/impact-budget/9" })
	pE := zerg.ImpactBudgetPlanBytesForTest(mE)
	if pE.OK || pE.Armed {
		t.Error("负控E：形状号不认竟还裁了")
	}
	if !strings.Contains(pE.Why, "形状号不认") {
		t.Errorf("负控E：不裁要写明原因，实得 %q", pE.Why)
	}
	_ = rootlessBudgetSanity(t, p)
}

// rootlessBudgetSanity 收口：生效上限必须**大于 0**（不裁与「0 当上限」是两回事）。
func rootlessBudgetSanity(t *testing.T, p zerg.ImpactBudgetPlanForTest) error {
	t.Helper()
	if p.Armed && !(p.CapSec > 0) {
		t.Errorf("裁了却给出非正的上限 %.6f（那会让每一次跑都降级）", p.CapSec)
	}
	return nil
}

// ---- ④ 实现里 0 个阈值数 + 红线静态自检 ----------------------------------------------------

// TestImpactBudget_NoThresholdsNoWritesInSource —— 判据④ + 红线的静态自检（源件自检）。
//
// 三条：① 上限相关的数**一个都不许**出现在实现件里（两式与取法都在契约件）；
// ② 实现件里**没有删除动作**、也没有「自动收窄 / 自动删 / 自动回滚」这些动作串；
// ③ 本件**不写盘**（只读契约件 + 只渲染 stderr）。
func TestImpactBudget_NoThresholdsNoWritesInSource(t *testing.T) {
	src, err := zerg.ImpactBudgetSourceForTest()
	if err != nil {
		t.Fatalf("读实现件失败：%v", err)
	}
	// ① 上限相关的数（含设计 §4.4 在册值与本机实测值）一个都不许写进实现。
	for _, num := range []string{"1.5", "2.0", "4.996", "7.494", "9.992", "6.2", "2.19", "3.77", "3.5"} {
		if strings.Contains(src, num) {
			t.Errorf("判据④ 破：实现件里出现了上限相关的数 %q（阈值必须住在契约件里）", num)
		}
	}
	// ② 四个「不许接的动作」只许住在契约件里；实现件里连字面都不许出现。
	for _, bad := range []string{"自动收窄", "自动删", "自动回滚"} {
		if strings.Contains(src, bad) {
			t.Errorf("红线破：实现件里出现了 %q（它只能作为契约件的「不许接的动作」被读出）", bad)
		}
	}
	for _, bad := range []string{"os.Remove", "os.RemoveAll", "os.Truncate"} {
		if strings.Contains(src, bad) {
			t.Errorf("红线破：实现件里有删除动作 %q", bad)
		}
	}
	// ③ 不写盘：只 os.ReadFile，不许出现任何写口。
	for _, bad := range []string{"os.WriteFile", "os.Create", "os.OpenFile", "os.MkdirAll", "os.Rename"} {
		if strings.Contains(src, bad) {
			t.Errorf("红线破：实现件里出现了写口 %q（本件只读契约件 + 只渲染 stderr）", bad)
		}
	}
	// ④ 出口纪律：不许改包封（实现件里不许出现包封那两枚函数的改动痕迹）。
	for _, bad := range []string{"emitEnvelopeWith(", "\"items\":", "\"meta\":{"} {
		if strings.Contains(src, bad) {
			t.Errorf("红线破：实现件里出现了包封写口 %q（九批一贯：不改 `emitEnvelope*`）", bad)
		}
	}
}

// ---- 契约 ↔ 实现对拍（键名 / 键都在）-------------------------------------------------------

// TestImpactBudget_ContractKeysMatchImplementation —— 实现在读的每一个键都必须**逐字**是契约件里的键，
// 且必须是 Go 认的 tag（`encoding/json` 对 tag 有一套字符白名单：全角括号一类**非法字符会让整条 tag
// 被静默丢弃** ⇒ 字段读不到。本判据就是把这一格钉住）。
func TestImpactBudget_ContractKeysMatchImplementation(t *testing.T) {
	src, err := zerg.ImpactBudgetSourceForTest()
	if err != nil {
		t.Fatalf("读实现件失败：%v", err)
	}
	raw, err := zerg.ImpactBudgetContractBytesForTest()
	if err != nil {
		t.Fatalf("读契约件失败：%v", err)
	}
	var c map[string]any
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatalf("契约件解不开：%v", err)
	}
	keys := map[string]bool{}
	var walk func(o any)
	walk = func(o any) {
		switch v := o.(type) {
		case map[string]any:
			for k, vv := range v {
				keys[k] = true
				walk(vv)
			}
		case []any:
			for _, vv := range v {
				walk(vv)
			}
		}
	}
	walk(c)

	tags := regexp.MustCompile("`json:\"([^\"]+)\"`").FindAllStringSubmatch(src, -1)
	if len(tags) == 0 {
		t.Fatal("实现件里没读到一个 json tag（判不了）")
	}
	for _, m := range tags {
		tag := m[1]
		if !keys[tag] {
			t.Errorf("实现读的键 %q 不在契约件里（键名漂了 ⇒ 字段静默读不到）", tag)
		}
		if !tagIsGoLegal(tag) {
			t.Errorf("键 %q 不是 Go 认的 tag（含白名单外的字符 ⇒ `encoding/json` 会把整条 tag 丢掉）", tag)
		}
	}
	// 反向：契约件的**机读**键（非说明性的那几个）不许被实现漏读。
	for _, need := range []string{"两式", "实测输入", "生效式取法", "生效式取法闭集", "耗时准入", "降级档", "层名单", "没跑的状态闭集", "不许接的动作"} {
		if !keys[need] {
			t.Errorf("契约件里缺键 %q", need)
		}
		if !strings.Contains(src, "`json:\""+need+"\"`") {
			t.Errorf("契约件有键 %q 而实现没读它（两处会漂）", need)
		}
	}
	// 成对负控：喂一个非法 tag 的键名 ⇒ 判别口必须报错（证明这一格有牙）。
	if tagIsGoLegal("两个口径（不可互换）") {
		t.Error("负控失败：含全角括号的键竟被判成合法 tag —— 这一格没牙")
	}
	if tagIsGoLegal("") {
		t.Error("负控失败：空键竟被判成合法 tag")
	}
}

// tagIsGoLegal 复刻 `encoding/json` 的 tag 字符白名单（`isValidTag`）。
func tagIsGoLegal(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if strings.ContainsRune("!#$%&()*+-./:;<=>?@[]^_{|}~ ", r) {
			continue
		}
		if !isLetterOrDigit(r) {
			return false
		}
	}
	return true
}

func isLetterOrDigit(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}

// ---- 层表形状（正控：六行逐行同形 · 降级行也同形）-----------------------------------------

// TestImpactBudget_LayerRowsKeepShape —— 「降级前后输出形状不变」：把两档 + 压上限那一跑的层表
// 逐行按同一个正则拆字段 ⇒ 字段序与个数必须一致（只有值不同）。
func TestImpactBudget_LayerRowsKeepShape(t *testing.T) {
	requireDeep(t) // 贵档闸 · 现读见本件头：ZERG_DEEP=1 才跑
	root := repoRootFromCLI(t)
	rel := "core/cmd/zerg/main.go"
	rowRe := regexp.MustCompile(`^  层([①②③④⑤⑥]) layer=(.+?) 粒度=(.+?) 口径=(.+?) 状态=(\S+) 耗时=(\S+) head_sha=(\S+) 时刻=(\S+)$`)
	check := func(name, block string) {
		lines := []string{}
		for _, ln := range strings.Split(block, "\n") {
			if strings.HasPrefix(ln, "  层") && strings.Contains(ln, "layer=") {
				lines = append(lines, ln)
			}
		}
		if len(lines) != 6 {
			t.Fatalf("%s ⇒ 层表该恒六行，实得 %d", name, len(lines))
		}
		seen := []string{}
		for _, ln := range lines {
			m := rowRe.FindStringSubmatch(ln)
			if m == nil {
				t.Fatalf("%s ⇒ 这一行拆不出七格（形状变了）：%s", name, ln)
			}
			seen = append(seen, m[1])
		}
		if strings.Join(seen, "") != "①②③④⑤⑥" {
			t.Errorf("%s ⇒ 层序该逐行是 ①–⑥，实得 %v", name, seen)
		}
	}

	// 正控 1：真仓默认档（缓存关 ⇒ 不碰状态目录）。
	t.Setenv("ZERG_STATE_DIR", t.TempDir())
	t.Setenv("ZERG_REPO", root)
	v1, err := zerg.ImpactLayersForTest(root, rel, false)
	if err != nil {
		t.Fatalf("跑链失败：%v", err)
	}
	check("真仓默认档", v1.Block)
	// 正控 2：压上限那一跑（真仓镜像根）—— 三行是「未跑（超预算降级）」，形状与其它行**同形**。
	short := budgetMirror(t, budgetPushCap(0.0005))
	t.Setenv("ZERG_REPO", short)
	v2, err := zerg.ImpactLayersForTest(short, rel, true)
	if err != nil {
		t.Fatalf("镜像根跑链失败：%v", err)
	}
	check("压上限（降级）那一跑", v2.Block)
	if !v2.Run.Degraded {
		t.Fatalf("压上限那一跑该降级（上限 %.3fs）", v2.Run.CapSec)
	}
	if n := strings.Count(v2.Block, zerg.ImpactBudgetDegradedStatusForTest()); n == 0 {
		t.Error("降级那一跑里一行 `未跑（超预算降级）` 都没有 ⇒ 留痕丢了")
	}
	// 降级行也带 head_sha（层规三件不许因为降级就缺件）。
	for _, ln := range strings.Split(v2.Block, "\n") {
		if strings.Contains(ln, zerg.ImpactBudgetDegradedStatusForTest()) && strings.HasPrefix(ln, "  层") {
			if !strings.Contains(ln, "head_sha=") || strings.Contains(ln, "head_sha=（取不到）") {
				t.Errorf("降级那一行缺 `head_sha`（层规三件）：%s", ln)
			}
			if !strings.Contains(ln, "口径=") {
				t.Errorf("降级那一行缺 `口径=`：%s", ln)
			}
		}
	}
	// 结论面：人面三行（stdout）与层表结论行在两跑里都要在（形状不变的另一半）。
	rc, out, errOut := runCapture("impact", rel)
	if rc != 0 {
		t.Fatalf("真仓默认档该 rc=0，实得 %d · %s", rc, tail(errOut, 300))
	}
	for _, key := range []string{"会牵动：", "会红：", "建议："} {
		if !strings.Contains(out, key) {
			t.Errorf("人面骨架字头 %q 丢了（降级前后都必须恒三行）", key)
		}
	}
	for _, blk := range []string{v1.Block, v2.Block} {
		if !strings.Contains(blk, "条目：卡片 `items`") {
			t.Error("层表结论行（条目那一行）丢了")
		}
		if !strings.Contains(blk, "`B4` 分层预算与降级") {
			t.Error("预算块丢了（降级前后都要在）")
		}
	}
}

// ---- 数字纪律：块里的两式值 = 契约件记的值（不心算 · 程序对拍）---------------------------

// TestImpactBudget_BlockEchoesContractNumbers —— 预算块里打出来的两式值，必须与契约件里记的
// `算出值秒` 逐字（到小数点后三位）相同；且块里必须明写「`R32` 绝对值仍待拍」。
func TestImpactBudget_BlockEchoesContractNumbers(t *testing.T) {
	requireDeep(t) // 贵档闸 · 现读见本件头：ZERG_DEEP=1 才跑
	root := repoRootFromCLI(t)
	t.Setenv("ZERG_REPO", root)
	t.Setenv("ZERG_STATE_DIR", t.TempDir())
	raw, err := zerg.ImpactBudgetContractBytesForTest()
	if err != nil {
		t.Fatalf("读契约件失败：%v", err)
	}
	var c struct {
		Cap struct {
			Formulas []struct {
				ID    string  `json:"id"`
				Fixed float64 `json:"算出值秒"`
			} `json:"两式"`
			Pick  string `json:"生效式取法"`
			Fixed struct {
				Value  float64 `json:"值秒"`
				Origin string  `json:"出处"`
			} `json:"上限定值"`
		} `json:"cap"`
	}
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatalf("契约件解不开：%v", err)
	}
	v, err := zerg.ImpactLayersForTest(root, "core/cmd/zerg/main.go", true)
	if err != nil {
		t.Fatalf("跑链失败：%v", err)
	}
	// `R32` 自 2026-09-22 起**已定**（Mr2109 定：取大）⇒ 块里要把「已定 + 出处 + 旧值并留」
	// 三件都打出来；旧文的「暂定」措辞仍以**旧值并留**那一行的形式逐字在块里（改前原文可找回）。
	if !strings.Contains(v.Block, "`R32` **已定**") {
		t.Error("预算块里该明写「`N` 的绝对值 `R32` 已定」（2026-09-22 Mr2109 定：取大）")
	}
	if !strings.Contains(v.Block, "已定值（契约件 `cap.上限定值`") {
		t.Errorf("预算块里该有「已定值」那一行：\n%s", tail(v.Block, 900))
	}
	if !strings.Contains(v.Block, c.Cap.Fixed.Origin) {
		t.Errorf("已定值那一行该逐字回引出处的原文 %q：\n%s", c.Cap.Fixed.Origin, tail(v.Block, 900))
	}
	if want := strconv.FormatFloat(c.Cap.Fixed.Value, 'f', 3, 64); !strings.Contains(v.Block, want) {
		t.Errorf("已定值 %s s（契约件记的）没在预算块里逐字出现：\n%s", want, tail(v.Block, 900))
	}
	if !strings.Contains(v.Block, "旧值并留（改前原文") {
		t.Error("预算块里该有「旧值并留」那一行（改前原文逐字找回）")
	}
	for _, f := range c.Cap.Formulas {
		want := strconv.FormatFloat(f.Fixed, 'f', 3, 64)
		if !strings.Contains(v.Block, want) {
			t.Errorf("式%s 的算出值 %s s（契约件记的）没在预算块里逐字出现：\n%s", f.ID, want, tail(v.Block, 900))
		}
	}
	// 生效式取法也要逐字回引契约件那一格。
	if !strings.Contains(v.Block, "`"+c.Cap.Pick+"`（契约件 `cap.生效式取法`）") {
		t.Errorf("生效式那一行该回引契约件的取法 %q：\n%s", c.Cap.Pick, tail(v.Block, 600))
	}
	// 排序稳定：同一判据跑两遍，块里那两式的行逐字相同（§九 M11）。
	v2, err := zerg.ImpactLayersForTest(root, "core/cmd/zerg/main.go", true)
	if err != nil {
		t.Fatalf("第二跑失败：%v", err)
	}
	lines := func(b string) []string {
		out := []string{}
		for _, ln := range strings.Split(b, "\n") {
			if strings.Contains(ln, "式①") || strings.Contains(ln, "式②") || strings.Contains(ln, "生效式：") {
				out = append(out, ln)
			}
		}
		sort.Strings(out)
		return out
	}
	if strings.Join(lines(v.Block), "\n") != strings.Join(lines(v2.Block), "\n") {
		t.Errorf("两跑的式行不一致（同一判据跑两遍该逐字一致）：\nA=%v\nB=%v", lines(v.Block), lines(v2.Block))
	}
	_ = fmt.Sprint
}
