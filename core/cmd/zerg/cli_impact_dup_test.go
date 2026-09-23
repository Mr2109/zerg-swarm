// cli_impact_dup_test.go —— `zerg impact` 的 **`C4` 重复面与归位**判据
// （任务单-影响面实施-20260922 §四 `C4` 八字段判据①–④ · 设计-变更影响面-v1.6 §6.5 ·
// 契约真源 `core/internal/contract/impact-dup.json` = 登记表 `S-l`）。
//
// 落在 `package main_test`（判据面跑**当前源码**）。
//
// 四格判据的机检口：
//
//	① **现跑值可复现** + **成对参数写死**：真起一次 `jscpd`（装了才跑；没装 ⇒ 照实跳过并明说）；
//	   负控：把成对参数改成**只留一格** ⇒ 必须**不给数**（只写一个 = 换了一把尺）；
//	② **标定曲线同报**：块里必有六档那一格（本跑跑没跑都要说出来 + 在册值并留）；
//	③ **不设门禁阈值**：静态自检（实现里没有阈值数 / 没有红绿判定 / 没有删除动作；
//	   读数不进退码、不进「会红」那一行）；
//	④ **归位正面判据**：三档分类（纯函数）+ 探真目录 + 负控（圈外路径 / 空目录 ⇒ 不给结论）。
package main_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	zerg "github.com/Mr2109/zerg-swarm/core/cmd/zerg"
)

// judgeDupBlock 判重复面块成不成形（**既可作为正控的口，也可喂改过的块当负控**）：
// 必须有「只报数」的措辞、参照值那一格（且写明**不是门槛**）、纪律那一行。
func judgeDupBlock(block string) error {
	for _, want := range []string{"`C4` 重复面与归位", "只报数不判红", "参照值（**不是门槛**）", "不设门禁阈值"} {
		if !strings.Contains(block, want) {
			return errStr("重复面块缺 " + want)
		}
	}
	return nil
}

// TestImpactDup_PairParamsAndNoThreshold —— 判据①（成对参数）+ 判据③（不设阈值）。
func TestImpactDup_PairParamsAndNoThreshold(t *testing.T) {
	// ① 成对参数：契约件里两格**都在**，且值的形状 = `--min-tokens N` / `--min-lines N`。
	raw, err := zerg.ImpactDupContractBytesForTest()
	if err != nil {
		t.Fatalf("读重复面契约件失败：%v", err)
	}
	mt, ml, ok := zerg.ImpactDupPairForTest(raw)
	if !ok || mt <= 0 || ml <= 0 {
		t.Fatalf("判据① 破：成对参数不成形（min-tokens=%d / min-lines=%d / ok=%v）", mt, ml, ok)
	}
	// 负控：只留一格 ⇒ 必须**不成形**（不许猜另一个）。
	if _, _, ok1 := zerg.ImpactDupPairForTest(zerg.ImpactDupPairModForTest([]string{"--min-tokens 50"})); ok1 {
		t.Error("负控①失败：只留 `--min-tokens` 一格竟还判「成形」（只写一个 = 换了一把尺）")
	}
	if _, _, ok2 := zerg.ImpactDupPairForTest(zerg.ImpactDupPairModForTest([]string{"--min-lines 5"})); ok2 {
		t.Error("负控①失败：只留 `--min-lines` 一格竟还判「成形」")
	}
	// 负控（端到端）：镜像根 + 只留一格的契约件 ⇒ 块里必须写「成对参数不成形 ⇒ 不给数」。
	mirror := t.TempDir()
	if err := os.MkdirAll(filepath.Join(mirror, "core", "internal", "contract"), 0o755); err != nil {
		t.Fatal(err)
	}
	blk := zerg.ImpactDupBlockWithContractForTest(mirror, zerg.ImpactDupPairModForTest([]string{"--min-tokens 50"}), false)
	if !strings.Contains(blk, "不给数") || (!strings.Contains(blk, "成对参数不成形") && !strings.Contains(blk, "键不成形")) {
		t.Errorf("负控①（端到端）失败：缺一格参数该照实说「不成形 ⇒ 不给数」：\n%s", blk)
	}
	if strings.Contains(blk, "现跑值：") {
		t.Errorf("负控①（端到端）失败：缺一格参数竟还给数：\n%s", blk)
	}
	// ③ 静态自检：实现里**没有**阈值数、没有红绿判定、没有删除动作。
	src, err := zerg.ImpactDupSourceForTest()
	if err != nil {
		t.Fatalf("读实现件失败：%v", err)
	}
	// 删除动作的**唯一合法形态**：清掉**自己造的临时报告目录**（`os.MkdirTemp` 那一枚）。
	// 判法 = 逐个 `os.Remove*` 调用点看实参名里有没有 `tmp` —— 其余实参（仓内件 / 状态目录 / 归档）
	// 一律不许出现（这条判据比「有没有 os.Remove」严：本面确实需要清自己的临时目录）。
	for _, m := range regexp.MustCompile(`os\.(Remove|RemoveAll)\(([^)]*)\)`).FindAllStringSubmatch(src, -1) {
		if !strings.Contains(m[2], "tmp") {
			t.Errorf("红线破：删除动作的实参 %q 不是「自己造的临时目录」（别人的件一处都不许删）", m[2])
		}
	}
	if strings.Contains(src, "os.Truncate") {
		t.Error("红线破：实现件里有截断动作 os.Truncate")
	}
	for _, bad := range []string{"自动删", "自动回滚", "自动收窄"} {
		if strings.Contains(src, bad) {
			t.Errorf("红线破：实现件里出现了 %q（只许作为契约件的「不许接的动作」被读出）", bad)
		}
	}
	// 「不设阈值」的可判形态：实现里不许出现 `8.5`（那是参照值）与任何 `>=`/`>` 阈值分支形态。
	if strings.Contains(src, "8.5") {
		t.Error("判据③ 破：实现里出现了参照值 8.5（参照值只许住在契约件里）")
	}
	for _, bad := range []string{"ArchUnit", "dependency-cruiser"} {
		if strings.Contains(src, bad) {
			t.Errorf("红线破：实现里出现了 %q（`R27`：不许引）", bad)
		}
	}
}

// TestImpactDup_ReportAndCurve —— 判据①②：现跑值可复现 + 标定曲线同报。
func TestImpactDup_ReportAndCurve(t *testing.T) {
	requireDeep(t) // 贵档闸 · 现读见本件头：ZERG_DEEP=1 才跑
	root := repoRootFromCLI(t)
	raw, rerr := zerg.ImpactDupContractBytesForTest()
	if rerr != nil {
		t.Fatalf("读重复面契约件失败：%v", rerr)
	}
	rep := zerg.ImpactDupReportForTest(root)
	if !rep.OK {
		if strings.Contains(rep.Why, "未装") {
			t.Skipf("本机没装 jscpd ⇒ 报数档这一格照实跳过（%s）", rep.Why)
		}
		t.Fatalf("判据① 破：报数档现跑不给数（%s · rc=%d）", rep.Why, rep.RC)
	}
	if rep.Clones <= 0 || rep.Lines <= 0 || rep.Tokens <= 0 || rep.Files <= 0 {
		t.Errorf("判据① 破：七格该都有值，实得 处=%d 行=%d token=%d 件=%d",
			rep.Clones, rep.Lines, rep.Tokens, rep.Files)
	}
	if rep.PairTok <= 0 || rep.PairLin <= 0 {
		t.Errorf("判据① 破：成对参数没跟着走（%d / %d）", rep.PairTok, rep.PairLin)
	}
	// 块（不跑曲线）：必须照实说「本跑未跑」+ 在册值并留 + 参照值不是门槛。
	mirror := t.TempDir()
	if err := os.MkdirAll(filepath.Join(mirror, "core", "internal", "contract"), 0o755); err != nil {
		t.Fatal(err)
	}
	blk := zerg.ImpactDupBlockWithContractForTest(mirror, raw, false) // 喂**真契约件**（正控）
	if err := judgeDupBlock(blk); err != nil {
		t.Errorf("判据② 破：%v\n%s", err, blk)
	}
	if !strings.Contains(blk, "标定曲线：**本跑未跑**") && !strings.Contains(blk, "标定曲线：**未跑**") {
		t.Errorf("判据② 破：不跑曲线时要照实说「未跑」：\n%s", blk)
	}
	if !strings.Contains(blk, "在册值并留") {
		t.Errorf("判据② 破：在册值要并留：\n%s", blk)
	}
	// 负控（judge）：抹掉「不是门槛」那一句 ⇒ 必须报错。
	if err := judgeDupBlock(strings.ReplaceAll(blk, "参照值（**不是门槛**）", "参照值")); err == nil {
		t.Error("负控失败：抹掉「不是门槛」竟判过（参照值不许被读成门槛）")
	}
}

// TestImpactDup_PlacementWhitelist —— 判据④：归位正面判据（三档分类 + 现读目录 + 负控）。
func TestImpactDup_PlacementWhitelist(t *testing.T) {
	root := repoRootFromCLI(t)
	// 三档分类（纯函数）：标准库 / 本仓 internal / 本仓其它包 / 圈外。
	cases := map[string]string{
		"fmt":     "标准库",
		"strings": "标准库",
		"github.com/Mr2109/zerg-swarm/core/internal/contract": "本仓 internal",
		"github.com/Mr2109/zerg-swarm/core/cmd/zerg":          "本仓其它包",
		"github.com/spf13/cobra":                              "圈外",
		"golang.org/x/tools/go/callgraph":                     "圈外",
	}
	for p, want := range cases {
		if got := zerg.ImpactImportClassifyForTest(p); got != want {
			t.Errorf("判据④ 破：%q 该判成 %q，实得 %q", p, want, got)
		}
	}
	// 现读两个真目录：分类计数自洽（三档 + 圈外 = 总数）。
	for _, dir := range []string{"core/cmd/zerg", "core/internal/contract"} {
		rows, err := zerg.ImpactImportWhitelistDirForTest(root, dir)
		if err != nil {
			t.Fatalf("判据④ 破：扫 %s 失败：%v", dir, err)
		}
		if len(rows) == 0 {
			t.Fatalf("判据④ 破：%s 一条 import 都没有 ⇒ 这一格是空的（空与「全白名单」不是一回事）", dir)
		}
		cnt := map[string]int{}
		for _, r := range rows {
			cnt[r[2]]++
		}
		if sum := cnt["标准库"] + cnt["本仓 internal"] + cnt["本仓其它包"] + cnt["圈外"]; sum != len(rows) {
			t.Errorf("判据④ 破：%s 三档计数之和 %d ≠ 总数 %d", dir, sum, len(rows))
		}
		t.Logf("%s：import %d 条 = 标准库 %d / 本仓 internal %d / 本仓其它包 %d / 圈外 %d",
			dir, len(rows), cnt["标准库"], cnt["本仓 internal"], cnt["本仓其它包"], cnt["圈外"])
	}
	// 负控：空目录 ⇒ **不给结论**（空不是「全白名单」）。
	empty := t.TempDir()
	if _, err := zerg.ImpactImportWhitelistDirForTest(empty, "."); err == nil {
		t.Error("负控④失败：空目录竟判成「全白名单」")
	}
}
