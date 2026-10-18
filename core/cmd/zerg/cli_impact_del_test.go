// cli_impact_del_test.go —— `zerg impact` 的 **`C1` 删面**判据
// （任务单-影响面实施-20260922 §四 `C1` 八字段判据①–⑥ · 设计-变更影响面-v1.6 §6.6 ·
// 契约真源 `core/internal/contract/impact-del.json` = 登记表 `S-m`）。
//
// 落在 `package main_test`（判据面跑**当前源码**）。
//
// **本件只做「不碰标记」那一半**（`R18` / `R21` 未拍）：不写标记、不批量动作、零自动删。
//
// 机检口：
//
//	① **五类盲区恒带五格**（正控：真仓块里五格齐 + 负控：抹掉一格 ⇒ 判定口必须报错）；
//	② **两件可找回证据缺一不出**（正控：真仓件 ⇒ `sha256` + 找回路经；负控：非仓件 ⇒ 报「缺」）；
//	③ **「整件搬出」与「符号级删」分行报**（正控 + 负控各喂一次）；
//	④ **零自动删**（静态自检：实现里没有删除动作、没有第二条路径、没有写标记的口）；
//	⑤ **文档联动行恒带**（默认档块里要有「文档面：」那一行 · `C3` 倒排口径 B）；
//	⑥ **默认档照实写「未跑 ≠ 没有候选」**（不许把「没跑」读成「没有候选」）。
package main_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	zerg "github.com/Mr2109/zerg-swarm/core/cmd/zerg"
)

// judgeDelBlind 判「五类盲区」那一行成不成形（判据①）：**契约件现读的每一格**都要在，
// 且要写明「缺一不出结论」。★ 格名**一律从契约件现读**（本件不抄名单 —— 手抄一份在测件里，
// 测件就只会为「代码与测试两份手抄一致」背书）。
// 负控：抹掉任意一格 ⇒ 必须报错。
func judgeDelBlind(block string, keys []string) error {
	if !strings.Contains(block, "缺一不出结论") {
		return errStr("五类盲区那一行没写「缺一不出结论」（判据①）")
	}
	if len(keys) == 0 {
		return errStr("准备态不对：契约件现读的类别格名是 0 格")
	}
	for _, k := range keys {
		if !strings.Contains(block, k+"=") {
			return errStr("五类盲区缺一格：" + k)
		}
	}
	return nil
}

// TestImpactDel_BlindSpotsAlwaysFive —— 判据①（五类盲区恒带 · 缺一不出结论）。
func TestImpactDel_BlindSpotsAlwaysFive(t *testing.T) {
	root := repoRootFromCLI(t)
	blk := zerg.ImpactDelBlockForTest(root, "core/cmd/zerg/main.go", false)
	keys := zerg.ImpactDelBlindKeysForTest(root)
	if err := judgeDelBlind(blk, keys); err != nil {
		t.Errorf("判据① 破：%v\n%s", err, blk)
	}
	// 负控：抹掉**契约件现读的末一格** ⇒ 必须报错（证明每一格都有牙；不写死是哪个词）。
	last := keys[len(keys)-1]
	if err := judgeDelBlind(strings.ReplaceAll(blk, last+"=", "（抹掉）="), keys); err == nil {
		t.Errorf("负控①失败：抹掉 `%s` 那一格盲区竟判过（缺一却不报 = 这一格没牙）", last)
	}
	// 负控（契约件层面）：格名只从契约件读 —— 抹掉整节 / 只剩注记格 ⇒ 读口必须给「不给结论」的理由。
	raw, err := zerg.ImpactDelContractBytesForTest()
	if err != nil {
		t.Fatalf("读契约件失败：%v", err)
	}
	if got := zerg.ImpactDelBlindKeysForTest(root); len(got) != len(keys) || len(got) == 0 {
		t.Errorf("判据① 破：契约件现读的类别格名不稳（%d ⟷ %d）", len(got), len(keys))
	}
	if _, why := zerg.ImpactDelBlindKeysOfForTest([]byte(`{"schema":"zerg/impact-del/1"}`)); why == "" {
		t.Error("负控①失败：契约件里没有 `五类盲区` 这一节，读口竟给了结论（fail-closed 破了）")
	}
	if _, why := zerg.ImpactDelBlindKeysOfForTest([]byte(`{"五类盲区":{"口径":"只有注记格"}}`)); why == "" {
		t.Error("负控①失败：`五类盲区` 只剩注记格（类别 0 格），读口竟给了结论（静默空名单）")
	}
	if _, why := zerg.ImpactDelBlindKeysOfForTest(raw); why != "" {
		t.Errorf("正控①失败：真契约件竟读不出类别格名（%s）", why)
	}
}

// TestImpactDel_EvidenceTwoPieces —— 判据②（两件可找回证据 · 缺一不出）。
func TestImpactDel_EvidenceTwoPieces(t *testing.T) {
	root := repoRootFromCLI(t)
	// 正控：真仓里的一件（有提交）⇒ 两件齐。
	sha, path, missing := zerg.ImpactDelEvidenceForTest(root, "core/cmd/zerg/family_code.go")
	if missing != "" {
		t.Errorf("判据② 破：真仓件该两件齐，实得缺 %q", missing)
	}
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(sha) {
		t.Errorf("判据② 破：`sha256` 形状不对（%q）", sha)
	}
	if path == "" || (!strings.Contains(path, "git 提交") && !strings.Contains(path, "归档副本")) {
		t.Errorf("判据② 破：找回路经要能指名（提交 sha 或归档副本路径），实得 %q", path)
	}
	// 负控：临时件（不在任何 git 仓内、也没有归档副本）⇒ **必须报「缺」**。
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "孤儿.go"), []byte("package x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sha2, path2, missing2 := zerg.ImpactDelEvidenceForTest(tmp, "孤儿.go")
	if missing2 == "" {
		t.Errorf("负控②失败：找不回的件竟两件齐（sha=%q path=%q）—— 「找不回的必拦」没落地", sha2, path2)
	}
	if !strings.Contains(missing2, "找不回") {
		t.Errorf("负控②失败：缺件要说清是「找不回」（实得 %q）", missing2)
	}
}

// TestImpactDel_ClassifyTwoTiers —— 判据④（`R37`：整件搬出 / 符号级删 **分行报**）。
func TestImpactDel_ClassifyTwoTiers(t *testing.T) {
	root := repoRootFromCLI(t)
	rel := "core/internal/statepath/statepath.go"
	syms := zerg.ImpactGoFileSymbolsForTest(root, rel)
	if len(syms) == 0 {
		t.Fatalf("准备态不对：%s 解析不出顶层符号", rel)
	}
	// 正控：该件**全部**顶层符号都在候选里 ⇒ 整件搬出。
	tier, total, hit := zerg.ImpactDelClassifyForTest(root, rel, syms)
	if !strings.HasPrefix(tier, "整件搬出") || hit != total {
		t.Errorf("判据④ 破：全命中该判「整件搬出」，实得 %q（%d/%d）", tier, hit, total)
	}
	// 负控：只给一部分符号 ⇒ 符号级删。
	if len(syms) > 1 {
		tier2, _, hit2 := zerg.ImpactDelClassifyForTest(root, rel, syms[:1])
		if !strings.HasPrefix(tier2, "符号级删") || hit2 != 1 {
			t.Errorf("负控④失败：只覆盖 1 个符号该判「符号级删」，实得 %q（hit=%d）", tier2, hit2)
		}
	}
	// 两档**分行报**：块里两档的措辞都要在（口径逐字）。
	blk := zerg.ImpactDelBlockForTest(root, "core/cmd/zerg/main.go", false)
	for _, want := range []string{"整件搬出", "符号级删", "分行报"} {
		if !strings.Contains(blk, want) {
			t.Errorf("判据④ 破：块里缺 %q（两档要分行报）", want)
		}
	}
}

// TestImpactDel_ZeroAutoDeleteAndDocLine —— 判据③⑤⑥（零自动删 + 文档联动行 + 「未跑 ≠ 没有候选」）。
func TestImpactDel_ZeroAutoDeleteAndDocLine(t *testing.T) {
	root := repoRootFromCLI(t)
	blk := zerg.ImpactDelBlockForTest(root, "core/cmd/zerg/main.go", false)
	// ⑤ 文档联动行**恒带**（口径 B · 复用 `C3` 倒排）。
	if !strings.Contains(blk, "文档面：") {
		t.Errorf("判据⑤ 破：删面块里没有「文档面：」那一行：\n%s", blk)
	}
	if !strings.Contains(blk, "口径 B") {
		t.Errorf("判据⑤ 破：文档联动行没写口径（要「口径 B · 反引号包住的符号名」）：\n%s", blk)
	}
	// ⑥ 默认档照实写「未跑」+「未跑 ≠ 没有候选」。
	if !strings.Contains(blk, "本跑没跑 ⇒ 不给候选") || !strings.Contains(blk, "未跑 ≠ 没有候选") {
		t.Errorf("判据⑥ 破：默认档要把「没跑」与「没有候选」分开写：\n%s", blk)
	}
	// ③ 零自动删：静态自检（没有删除动作 / 没有第二条路径 / 没有写标记）。
	src, err := zerg.ImpactDelSourceForTest()
	if err != nil {
		t.Fatalf("读实现件失败：%v", err)
	}
	for _, bad := range []string{"os.Remove", "os.RemoveAll", "os.Truncate", "os.WriteFile", "os.Rename"} {
		if strings.Contains(src, bad) {
			t.Errorf("红线破：实现件里有 %q（删面只出候选：既不删也不写）", bad)
		}
	}
	for _, bad := range []string{"自动删", "自动回滚"} {
		if strings.Contains(src, bad) {
			t.Errorf("红线破：实现件里出现了 %q（只许作为契约件的红线被读出）", bad)
		}
	}
	// `-test` 必带：实现里 deadcode 的调用串要带 `-test`（`R19`）。
	if !strings.Contains(src, `"-test", "./..."`) {
		t.Error("红线破：`deadcode` 的调用没带 `-test`（不带 ⇒ 85.1% 是「只被测试引用」· `R19`）")
	}
	// 标记面（`R18`/`R21` 未拍）⇒ 实现里一个字都不许提「标记」这套机制的写入。
	if strings.Contains(src, "allow_dead") || strings.Contains(src, "不可删标记") {
		t.Error("红线破：实现里出现了标记面的东西（`R18`/`R21` 未拍 ⇒ 不碰）")
	}
}
