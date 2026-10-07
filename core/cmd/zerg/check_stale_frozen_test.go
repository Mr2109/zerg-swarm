package main

// check_stale_frozen_test.go —— 同批成对负控（✗ 禁恒绿）。
// 正控：合成夹具三处纯 IO 冻结点自洽 ⇒ 各面「一致·过期 0」。
// 负控：冻错 sha256 ⇒ 必须报「过期·1」（哑化主件该比较 ⇒ 本用例 FAIL 带行号）；
//       冻结面键集不等 / 门件未登记 / 仓根取不到 ⇒ 各成一格。
import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func csfTestWrite(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func csfTestFace(t *testing.T, root, name string) *csfFace {
	t.Helper()
	var got *csfFace
	for _, f := range csfCollect(root, io.Discard) {
		if f.jian == name {
			got = f
		}
	}
	if got == nil {
		t.Fatalf("找不到面 %s", name)
	}
	return got
}

// 正控：内置自检全过（正控 3 面 · 负控成对 · 空转 2 格 · 形状 2 态）。
func TestCsfSelfTestPasses(t *testing.T) {
	var ob, eb bytes.Buffer
	if rc := csfGateMain(&ob, &eb, []string{"--self-test"}); rc != 0 {
		t.Fatalf("csf:--self-test 期望 rc=0 实得 rc=%d\nstdout=%s\nstderr=%s", rc, ob.String(), eb.String())
	}
	if !strings.Contains(ob.String(), "✓ check-stale-frozen 自检全过") {
		t.Fatalf("csf:--self-test 缺全过行：%q", ob.String())
	}
}

// 正控：合成夹具三处纯 IO 面「一致·过期 0」。
func TestCsfFixtureGreenFaces(t *testing.T) {
	root, err := csfSelftestFixture("")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)
	for _, name := range []string{"scripts/gates/judge-registry.json", "scripts/gates/real-gates.sh", "scripts/公开标记.tsv"} {
		f := csfTestFace(t, root, name)
		if f.status != "一致" || f.stale != 0 {
			t.Fatalf("面 %s 期望「一致·0」实得「%s·%d」", name, f.status, f.stale)
		}
	}
}

// 负控：冻错 sha256 ⇒ judge-registry 面必报「过期·1」。
func TestCsfNegativeFrozenMismatch(t *testing.T) {
	root, err := csfSelftestFixture(strings.Repeat("0", 64))
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)
	f := csfTestFace(t, root, "scripts/gates/judge-registry.json")
	if f.status != "过期" || f.stale != 1 {
		t.Fatalf("负控：judge-registry 期望「过期·1」实得「%s·%d」", f.status, f.stale)
	}
}

// 负控：SH_GATES ⟷ SH_TABLE 键集不等 ⇒ real-gates 面报过期。
func TestCsfNegativeRealGatesMismatch(t *testing.T) {
	root, err := csfSelftestFixture("")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)
	csfTestWrite(t, filepath.Join(root, "scripts", "gates", "real-gates.sh"),
		"SH_GATES=(x y)\n\nSH_TABLE=\"$(cat <<'EOF'\nx\tg_x\nEOF\n)\"\n")
	f := csfTestFace(t, root, "scripts/gates/real-gates.sh")
	if f.status != "过期" || f.stale != 1 {
		t.Fatalf("负控：real-gates 键集不等 期望「过期·1」实得「%s·%d」", f.status, f.stale)
	}
}

// 负控：盘上有门件、公开标记表未登记 ⇒ public-marks 面报过期。
func TestCsfNegativePublicMarksUnregistered(t *testing.T) {
	root, err := csfSelftestFixture("")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)
	csfTestWrite(t, filepath.Join(root, "scripts", "gates", "y.py"), "print(1)\n")
	f := csfTestFace(t, root, "scripts/公开标记.tsv")
	if f.status != "过期" || f.stale != 1 {
		t.Fatalf("负控：public-marks 未登记 期望「过期·1」实得「%s·%d」", f.status, f.stale)
	}
}

// 空转：仓根取不到 scripts/gates ⇒ rc=2（不静默绿）。
func TestCsfMissingRootRc2(t *testing.T) {
	dir := t.TempDir()
	var ob, eb bytes.Buffer
	if rc := csfGateMain(&ob, &eb, []string{"--root", filepath.Join(dir, "no-such-root")}); rc != 2 {
		t.Fatalf("空转：期望 rc=2 实得 rc=%d", rc)
	}
	if !strings.Contains(eb.String(), "仓根取不到") {
		t.Fatalf("空转：stderr 缺「仓根取不到」：%q", eb.String())
	}
	// 空目录（无 scripts/gates）同样 rc=2
	var ob2, eb2 bytes.Buffer
	if rc := csfGateMain(&ob2, &eb2, []string{"--root", dir}); rc != 2 {
		t.Fatalf("空转·无 scripts/gates：期望 rc=2 实得 rc=%d", rc)
	}
}

// 用法错：未知旗标 ⇒ rc=2 ＋ argparse 正文。
func TestCsfUnknownFlagRc2(t *testing.T) {
	var ob, eb bytes.Buffer
	if rc := csfGateMain(&ob, &eb, []string{"--nope"}); rc != 2 {
		t.Fatalf("未知旗标：期望 rc=2 实得 rc=%d", rc)
	}
	if !strings.Contains(eb.String(), "unrecognized arguments: --nope") {
		t.Fatalf("未知旗标：stderr 缺 argparse 正文：%q", eb.String())
	}
}

// 帮助面：-h ⇒ rc=0 ＋ usage 行。
func TestCsfHelpRc0(t *testing.T) {
	var ob, eb bytes.Buffer
	if rc := csfGateMain(&ob, &eb, []string{"-h"}); rc != 0 {
		t.Fatalf("帮助面：期望 rc=0 实得 rc=%d", rc)
	}
	if !strings.Contains(ob.String(), "usage: check-stale-frozen.py [-h] [--root ROOT] [--json [字段]] [--self-test]") {
		t.Fatalf("帮助面：stdout 缺 usage 行：%q", ob.String())
	}
}

// --json：报告档 payload 形状（mode=report-only · rc=0 · faces 在场）。
func TestCsfJSONShape(t *testing.T) {
	root, err := csfSelftestFixture("")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)
	var ob, eb bytes.Buffer
	if rc := csfGateMain(&ob, &eb, []string{"--root", root, "--json"}); rc != 0 {
		t.Fatalf("--json：期望 rc=0 实得 rc=%d", rc)
	}
	line := ""
	for _, l := range csfSplitLines(ob.String()) {
		if strings.HasPrefix(l, "{") {
			line = l
		}
	}
	if line == "" {
		t.Fatalf("--json：无 JSON 行：%q", ob.String())
	}
	for _, want := range []string{"\"mode\": \"report-only\"", "\"rc\": 0", "\"faces\": [", "\"明细\": []"} {
		if !strings.Contains(line, want) {
			t.Fatalf("--json：payload 缺 %s", want)
		}
	}
}

// TestCsfBridgeAffectedMapRetiredFallback —— G16（2026-10-07 退役批）：兄弟件
// `scripts/gates/check-affected-map.py` 退役（不在盘）⇒ `csfBridge("affected_map")` 回落**同判据
// Go 半** `afmBuildMap`（✗ 不因缺 py 而「重算失败 ⇒ 不给结论」fail-closed）。
// ★ 成对：兄弟件在盘（哪怕是个空壳）⇒ 仍走 importlib 那条桥（单一真源不变）。
// ★ 哑化负控：抽掉 csfBridge 里 `if op == "affected_map"` 那段 ⇒ 本用例 FAIL（带行号）。
func TestCsfBridgeAffectedMapRetiredFallback(t *testing.T) {
	root := afmFix(t) // 仓外夹具（自带脚本面）· 其中**无** check-affected-map.py
	if _, err := os.Stat(filepath.Join(root, "scripts", "gates", "check-affected-map.py")); err == nil {
		t.Fatal("前置不成立：夹具里竟有 check-affected-map.py")
	}
	res, errStr := csfBridge(root, "affected_map", io.Discard)
	if errStr != "" {
		t.Fatalf("退役态该回落 Go 半 afmBuildMap（✗ 不 fail-closed）：%q", errStr)
	}
	if res == nil || csfPctS(res["fp"]) == "" || csfPctS(res["head"]) == "" {
		t.Fatalf("回落读数缺 fp/head：%v", res)
	}
	// 成对负控：把（空壳）兄弟件放回盘 ⇒ 仍走 importlib 桥（空壳 ⇒ 桥必报错 ⇒ 证明在盘时走 py 那把）。
	py := filepath.Join(root, "scripts", "gates", "check-affected-map.py")
	if err := os.WriteFile(py, []byte("raise SystemExit('stub')\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, e2 := csfBridge(root, "affected_map", io.Discard); e2 == "" {
		t.Fatalf("兄弟件在盘时该走 importlib 桥（成对负控：空壳 py 应让桥报错）")
	}
}
