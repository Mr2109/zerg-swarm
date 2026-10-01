// check_newfile_registry_test.go —— 门76 Go 真搬的**成对负控**定向单测（M2 新口径 · 外部树走显式旋钮）。
//
// ✗ 禁恒绿：本件同时断言「必然判绿」「必然出红明细」「不给结论」三态，并钉住**退码档的牙**：
// 报告档含缺登记 ⇒ rc=0（只报告）⟷ 阻断档同树 ⇒ rc=1 ⟷ 分母 0 / 真源不在 ⇒ rc=2。
// 用例名：TestCheckNewfileRegistryGatePort（定向：go test -run TestCheckNewfileRegistryGatePort）。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func gnrRunT(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := checkNewfileRegistryGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

// gnrMkFixture —— 造一棵合成夹具（仓外 · t.TempDir）。
func gnrMkFixture(t *testing.T, register map[string]bool) string {
	t.Helper()
	tmp := t.TempDir()
	gnrBuildFixture(t, tmp, gnrProbeRel, register)
	return tmp
}

func gnrWriteInv(t *testing.T, tmp, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(tmp, "inv.tsv"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCheckNewfileRegistryGatePort(t *testing.T) {
	allFive := map[string]bool{"①": true, "②": true, "③": true, "⑤": true}

	// ── 正控（必然判绿）：五表齐（阻断档）⇒ rc=0 ─────────────────────────
	fx := gnrMkFixture(t, allFive)
	gnrWriteInv(t, fx, "件\n"+gnrProbeRel+"\n")
	rcG, outG, errG := gnrRunT(t, "--repo", fx, "--new-file", gnrProbeRel, "--inventory", "inv.tsv", "--strict")
	if rcG != 0 || errG != "" {
		t.Fatalf("正控失败：五表齐应 rc=0/stderr 空，得 rc=%d stderr=%q out=%s", rcG, errG, outG)
	}
	for _, want := range []string{
		"门76 新件登记完整性", "判到了 —— 五张登记表逐面 **0 条缺登记**",
		"面① 公开标记表缺登记：0", "面② 干跑登记缺件：0", "面③ 门① 两张真源缺件：0",
		"面④ 脚本台账缺登记：0", "面⑤ 公开链两器排除表：0",
	} {
		if !strings.Contains(outG, want) {
			t.Fatalf("正控失败：缺 %q\nout=%s", want, outG)
		}
	}

	// ── 负控①（有红明细 · 报告档不改码）：未登记 ⇒ rc=0 但逐条点名 ────────────
	fxU := gnrMkFixture(t, nil)
	gnrWriteInv(t, fxU, "件\n"+gnrProbeRel+"\n")
	rcR, outR, _ := gnrRunT(t, "--repo", fxU, "--new-file", gnrProbeRel, "--inventory", "inv.tsv")
	if rcR != 0 {
		t.Fatalf("负控①：报告档命中不改码 ⇒ 应 rc=0，得 rc=%d", rcR)
	}
	if !strings.Contains(outR, "面① 公开标记表缺登记：1") ||
		!strings.Contains(outR, "← 表里无此行") ||
		!strings.Contains(outR, "面③ 门① 两张真源缺件：1") ||
		!strings.Contains(outR, "3 条缺登记") {
		t.Fatalf("负控①失败：应点名 3 条缺登记，得 out=%s", outR)
	}
	// 成对（✗ 禁恒绿）：绿 ↔ 红 正文必须不同
	if outR == outG {
		t.Fatalf("成对断言失败：负控①正文与正控逐字相同 ⇒ 判据无鉴别力")
	}

	// ── 负控②（退码档的牙）：同树阻断档 ⇒ rc=1（≠ 报告档 0）───────────────
	rcB, outB, _ := gnrRunT(t, "--repo", fxU, "--new-file", gnrProbeRel, "--inventory", "inv.tsv", "--strict")
	if rcB != 1 {
		t.Fatalf("负控②失败：阻断档应 rc=1，得 rc=%d out=%s", rcB, outB)
	}
	if rcB == rcR {
		t.Fatalf("成对断言失败：报告档 rc=%d 与阻断档 rc=%d 同码 ⇒ 退码档无牙", rcR, rcB)
	}
	if !strings.Contains(outB, "判据结论：**3 条缺登记**") {
		t.Fatalf("负控②失败：应逐条点名，得 out=%s", outB)
	}

	// ── 不给结论①（分母 0 件）⇒ rc=2 ────────────────────────────────────
	rc0, out0, _ := gnrRunT(t, "--repo", fxU, "--new-file", "")
	if rc0 != 2 || !strings.Contains(out0, "不给结论（rc=2）") {
		t.Fatalf("判不了①失败：分母 0 应 rc=2，得 rc=%d out=%s", rc0, out0)
	}
	if rc0 == rcR || rc0 == rcB {
		t.Fatalf("成对断言失败：判不了 rc=2 与报告/阻断档同码")
	}

	// ── 不给结论②（外部树显式旋钮取不到）⇒ rc=2 ─────────────────────────
	rcL, outL, _ := gnrRunT(t, "--repo", fxU, "--new-file", gnrProbeRel, "--inventory", "路径不存在.tsv")
	if rcL != 2 || !strings.Contains(outL, "面④ 脚本台账") {
		t.Fatalf("判不了②失败：仓外台账取不到应 rc=2 并照实打印，得 rc=%d out=%s", rcL, outL)
	}
	// 成对（新口径的牙）：单棵缺席照判在场那棵 ⇒ 台账在场（inv.tsv）⇒ rc=0 ≠ 都取不到 2
	rcOne, _, _ := gnrRunT(t, "--repo", fxU, "--new-file", gnrProbeRel, "--inventory", "inv.tsv")
	if rcOne != 0 {
		t.Fatalf("退2边界失败：台账在场 ⇒ 应 rc=0，得 rc=%d", rcOne)
	}
	if rcOne == rcL {
		t.Fatalf("成对断言失败：台账在场 rc=%d 与取不到 rc=%d 同码 ⇒ 边界无牙", rcOne, rcL)
	}

	// ── 不给结论③（基线不在盘）⇒ rc=2 ───────────────────────────────────
	rcBase, outBase, _ := gnrRunT(t, "--repo", fxU, "--baseline", "no-such-baseline.tsv")
	if rcBase != 2 || !strings.Contains(outBase, "冻结基线不在盘") {
		t.Fatalf("判不了③失败：基线不在盘应 rc=2，得 rc=%d out=%s", rcBase, outBase)
	}

	// ── 弱挂载档不判红 ──────────────────────────────────────────────────
	fxW := gnrMkFixture(t, map[string]bool{"①": true, "②": true, "⑤": true})
	f, err := os.OpenFile(filepath.Join(fxW, gnrStepsRel), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("add_step \"build\" tri \"构建\" bash scripts/build/new-probe.sh\n")
	f.Close()
	gnrWriteInv(t, fxW, "件\n"+gnrProbeRel+"\n")
	rcW, outW, _ := gnrRunT(t, "--repo", fxW, "--new-file", gnrProbeRel, "--inventory", "inv.tsv", "--strict")
	if rcW != 0 || !strings.Contains(outW, "弱挂载 1 件") {
		t.Fatalf("弱挂载失败：应 rc=0 且计弱挂载 1 件，得 rc=%d out=%s", rcW, outW)
	}

	// ── py 口子①（--dump-baseline 生成口 · 逐字保留）⇒ 打当前树后分母恒 0 ─────
	dump := filepath.Join(t.TempDir(), "base.tsv")
	rcD, outD, _ := gnrRunT(t, "--repo", fx, "--dump-baseline", dump)
	if rcD != 0 || !strings.Contains(outD, "基线已写：") {
		t.Fatalf("--dump-baseline 失败：得 rc=%d out=%s", rcD, outD)
	}
	rcAfter, outAfter, _ := gnrRunT(t, "--repo", fx, "--baseline", dump)
	if rcAfter != 2 || !strings.Contains(outAfter, "分母（现读新件 ∖ 冻结基线）：0 件") {
		t.Fatalf("py 口子①失守：生成口打当前树后应分母 0 ⇒ rc=2，得 rc=%d out=%s", rcAfter, outAfter)
	}

	// ── 读不了的件：公开标记表 chmod 000 ⇒ 面① 判不了 ⇒ rc=2（fail-closed）────
	fxX := gnrMkFixture(t, allFive)
	os.Chmod(filepath.Join(fxX, gnrMarksRel), 0)
	rcX, outX, _ := gnrRunT(t, "--repo", fxX, "--new-file", gnrProbeRel, "--baseline", "no-base.tsv")
	if rcX != 2 || !strings.Contains(outX, "面①公开标记表 判不了") {
		t.Fatalf("读不了的件失败：应 rc=2 且点名面① 判不了，得 rc=%d out=%s", rcX, outX)
	}

	// ── 清单 / 自检 / 帮助 / 用法错 ─────────────────────────────────────
	rcST, outST, _ := gnrRunT(t, "--self-test")
	if rcST != 0 || !strings.Contains(outST, "自检结论: 全过（断言 8 条 · 失败 0）") {
		t.Fatalf("--self-test 失败：得 rc=%d out=%s", rcST, outST)
	}
	rcH, outH, _ := gnrRunT(t, "-h")
	if rcH != 0 || !strings.Contains(outH, "usage: check-newfile-registry.py [-h] [--repo REPO]") {
		t.Fatalf("-h 失败：得 rc=%d out=%s", rcH, outH)
	}
	rcZ, outZ, errZ := gnrRunT(t, "--zzz")
	if rcZ != 2 || outZ != "" || !strings.Contains(errZ, "unrecognized arguments: --zzz") {
		t.Fatalf("未知旗标失败：得 rc=%d stdout=%q stderr=%q", rcZ, outZ, errZ)
	}
	rcM, _, errM := gnrRunT(t, "--repo")
	if rcM != 2 || !strings.Contains(errM, "argument --repo: expected one argument") {
		t.Fatalf("缺值失败：得 rc=%d stderr=%q", rcM, errM)
	}
}
