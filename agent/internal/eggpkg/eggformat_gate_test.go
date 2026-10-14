package eggpkg

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// f24SynthSource —— 合成真源（含两张表 ＋ 面常量）；extra 为空 ⇒ 基准形。
func f24SynthSource(extra string) []byte {
	return []byte(strings.Replace(eggSpecSynthTemplate, "%s", extra, 1))
}

// f24MakeRepo —— 造合成仓：写真源 ＋ 按现真源重生派生件；返回仓根。
func f24MakeRepo(t *testing.T, name string, src []byte) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(EggSpecSourcePath)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(root, filepath.FromSlash(EggSpecSourcePath))); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, filepath.Dir(filepath.FromSlash(EggSpecSourcePath))), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, filepath.Dir(filepath.FromSlash(EggDerivedPath))), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(EggSpecSourcePath)), src, 0o644); err != nil {
		t.Fatal(err)
	}
	der, err := eggRenderDerivedFromSource(src)
	if err != nil {
		t.Fatalf("重生派生件：%v", err)
	}
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(EggDerivedPath)), der, 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// ---- 四态亲测（真跑贴读数）--------------------------------------------------------------------

func TestF24SpecGateFourStates(t *testing.T) {
	base := f24SynthSource("\t")
	changed := f24SynthSource("\t{Key: \"min_readable\", Surface: zegSurfaceCompat, Kind: \"number\", Why: \"synth\"},\n")

	// 态①：一致态 ⇒ 绿
	r1 := f24MakeRepo(t, "s1", base)
	g1 := EggSpecGate(r1)
	t.Logf("态① 一致态：status=%d · %s", g1.Status, g1.Message)
	if g1.Status != EggSpecGateGreen {
		t.Fatalf("态① 期望绿（0），得 %d：%s", g1.Status, g1.Message)
	}

	// 态②：真源改动而派生件未重生 ⇒ 必红且点名真源
	r2 := f24MakeRepo(t, "s2", base)
	if err := os.WriteFile(filepath.Join(r2, filepath.FromSlash(EggSpecSourcePath)), changed, 0o644); err != nil {
		t.Fatal(err)
	}
	g2 := EggSpecGate(r2)
	t.Logf("态② 真源改未重生：status=%d named=%s · %s", g2.Status, g2.Named, g2.Message)
	if g2.Status != EggSpecGateRed || g2.Named != EggSpecSourcePath {
		t.Fatalf("态② 期望「红且点名真源 %s」，得 status=%d named=%q", EggSpecSourcePath, g2.Status, g2.Named)
	}

	// 态③：派生件被手改（头 sha 未动）⇒ 必红且点名派生件
	r3 := f24MakeRepo(t, "s3", base)
	der, err := os.ReadFile(filepath.Join(r3, filepath.FromSlash(EggDerivedPath)))
	if err != nil {
		t.Fatal(err)
	}
	tampered := bytes.Replace(der, []byte(`Why: "synth"`), []byte(`Why: "tampered"`), 1)
	if bytes.Equal(tampered, der) {
		t.Fatal("夹具坏：派生件里找不到可改处")
	}
	if err := os.WriteFile(filepath.Join(r3, filepath.FromSlash(EggDerivedPath)), tampered, 0o644); err != nil {
		t.Fatal(err)
	}
	g3 := EggSpecGate(r3)
	t.Logf("态③ 派生件手改：status=%d named=%s · %s", g3.Status, g3.Named, g3.Message)
	if g3.Status != EggSpecGateRed || g3.Named != EggDerivedPath {
		t.Fatalf("态③ 期望「红且点名派生件 %s」，得 status=%d named=%q", EggDerivedPath, g3.Status, g3.Named)
	}

	// 态④：真源取不到 ⇒ rc=2（不给结论）
	r4 := f24MakeRepo(t, "s4", base)
	if err := os.Remove(filepath.Join(r4, filepath.FromSlash(EggSpecSourcePath))); err != nil {
		t.Fatal(err)
	}
	g4 := EggSpecGate(r4)
	t.Logf("态④ 真源取不到：status=%d · %s", g4.Status, g4.Message)
	if g4.Status != EggSpecGateNoVerdict {
		t.Fatalf("态④ 期望不给结论（2），得 %d：%s", g4.Status, g4.Message)
	}
}

// ---- 两枚负控：证明「正文比对」有独立的牙 ------------------------------------------------------

// 负控①：只比「派生件头记的 sha ⟷ 真源 sha」的弱化检查器，在态③（正文被手改）上必须**绿** ——
// 否则正文比对就没有独立牙（说明它只是重复了 sha 比对）。
func TestF24SpecGateNegativeControl_ShaOnlyMissesTamper(t *testing.T) {
	base := f24SynthSource("\t")
	r := f24MakeRepo(t, "nc1", base)
	der, err := os.ReadFile(filepath.Join(r, filepath.FromSlash(EggDerivedPath)))
	if err != nil {
		t.Fatal(err)
	}
	tampered := bytes.Replace(der, []byte(`Why: "synth"`), []byte(`Why: "tampered"`), 1)
	if err := os.WriteFile(filepath.Join(r, filepath.FromSlash(EggDerivedPath)), tampered, 0o644); err != nil {
		t.Fatal(err)
	}
	src, _ := os.ReadFile(filepath.Join(r, filepath.FromSlash(EggSpecSourcePath)))
	want, err := eggSpecSourceSHAOf(src)
	if err != nil {
		t.Fatal(err)
	}
	shaOnly := eggDerivedRecordedSHA(tampered) == want
	t.Logf("负控①：只比 sha 的弱化检查器在态③上 sha_only_green=%v（真门 status=%d）", shaOnly, EggSpecGate(r).Status)
	if !shaOnly {
		t.Fatal("负控① 前提不成立：弱化检查器在态③上竟也红 ⇒ 正文比对无独立牙")
	}
	if EggSpecGate(r).Status != EggSpecGateRed {
		t.Fatal("负控① 真门在态③上应红")
	}
}

// 负控②：态②（真源改未重生）判红之后，把改后真源**重生一次** ⇒ 门必须转绿（红是「未重生」之因，非门永红）。
func TestF24SpecGateNegativeControl_RegenTurnsGreen(t *testing.T) {
	base := f24SynthSource("\t")
	changed := f24SynthSource("\t{Key: \"min_readable\", Surface: zegSurfaceCompat, Kind: \"number\", Why: \"synth\"},\n")
	r := f24MakeRepo(t, "nc2", base)
	if err := os.WriteFile(filepath.Join(r, filepath.FromSlash(EggSpecSourcePath)), changed, 0o644); err != nil {
		t.Fatal(err)
	}
	if got := EggSpecGate(r).Status; got != EggSpecGateRed {
		t.Fatalf("负控② 前提：改了真源未重生应先红，得 %d", got)
	}
	der, err := eggRenderDerivedFromSource(changed)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(r, filepath.FromSlash(EggDerivedPath)), der, 0o644); err != nil {
		t.Fatal(err)
	}
	got := EggSpecGate(r).Status
	t.Logf("负控②：重生后 status=%d（应 0 绿）", got)
	if got != EggSpecGateGreen {
		t.Fatalf("负控② 重生后应转绿，得 %d", got)
	}
}

// ---- 自检末行逐字对拍 -------------------------------------------------------------------------

func TestF24SpecSelfTestLastLineVerbatim(t *testing.T) {
	var out, errb bytes.Buffer
	rc := EggSpecSelfTest(&out, &errb)
	if rc != EggSpecGateGreen {
		t.Fatalf("自检 rc=%d · stderr=%s", rc, errb.String())
	}
	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	last := lines[len(lines)-1]
	t.Logf("自检末行=%s", last)
	if last != EggSpecSelfTestLastLine {
		t.Fatalf("自检末行不逐字：\n得   %q\n期望 %q", last, EggSpecSelfTestLastLine)
	}
}

// ---- 表驱动判据（判据表来自派生件）------------------------------------------------------------

func TestF24EggAuditBySpecTableDriven(t *testing.T) {
	// 缺不可变面必需键 + 同键换语义 + 可加面未知键，三档各一条 ⇒ 判据表驱动。
	body := []byte(`{"magic":"ZEGG","version":"NOT_A_NUMBER","align":4096,"entries":[],"min_readable":1}`)
	findings, err := EggAuditBySpec(body)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(findings, " | ")
	t.Logf("表驱动判据读数：%s", joined)
	if !strings.Contains(joined, "version") || !strings.Contains(joined, "同键换语义") {
		t.Fatalf("应点名 version 同键换语义，得：%s", joined)
	}
	if !strings.Contains(joined, "min_readable") || !strings.Contains(joined, "可加面未知键") {
		t.Fatalf("应把 min_readable 归可加面未知键，得：%s", joined)
	}
}

// ---- 只报告档：14 格分叉 ＋ 拄证锚活性 ---------------------------------------------------------

func TestF24ParityReportAnchorsLive(t *testing.T) {
	root := f24RepoRoot(t)
	if root == "" {
		t.Skip("找不到仓根（合成仓外跑）")
	}
	cells := EggParityReport(root)
	if len(cells) != 14 {
		t.Fatalf("分叉格数应为 14，得 %d", len(cells))
	}
	var stale []string
	for _, c := range cells {
		t.Logf("格 %2d · %s｜A(eggpkg)=%s｜B(zerg_egg)=%s｜锚 %s / %s ok=%v %s",
			c.No, c.Field, c.SideA, c.SideB, c.AAnchor, c.BAnchor, c.AnchorOK, c.Note)
		if !c.AnchorOK {
			stale = append(stale, c.Field)
		}
	}
	if len(stale) > 0 {
		t.Fatalf("有 %d 格的拄证锚已腐（两侧源码已变 ⇒ 读数待重核）：%v", len(stale), stale)
	}
}

// ---- 真仓现读：一致性门 ------------------------------------------------------------------------

func TestF24RealRepoGate(t *testing.T) {
	root := f24RepoRoot(t)
	if root == "" {
		t.Skip("找不到仓根")
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(EggDerivedPath))); err != nil {
		t.Skipf("派生件尚未落盘（%s）：%v", EggDerivedPath, err)
	}
	g := EggSpecGate(root)
	t.Logf("真仓一致性门：status=%d · %s", g.Status, g.Message)
	if g.Status != EggSpecGateGreen {
		t.Fatalf("真仓期望一致绿，得 status=%d：%s", g.Status, g.Message)
	}
}

// f24RepoRoot —— 由本件位置（agent/internal/eggpkg）上溯 3 级；找不到 ⇒ ""。
func f24RepoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		return ""
	}
	if _, err := os.Stat(filepath.Join(root, "core", "cmd", "zerg", "zerg_egg_version.go")); err != nil {
		return ""
	}
	if _, err := os.Stat(filepath.Join(root, "agent", "go.mod")); err != nil {
		return ""
	}
	return root
}
