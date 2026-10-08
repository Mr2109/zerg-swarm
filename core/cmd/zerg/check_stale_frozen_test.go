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
	"reflect"
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

// TestCsfBridgeHelpShapeSourceIsGoHalf —— 2026-10-08（本单 · 解 check-help-shape.py 退役读耦合）：
// `csfBridge("help_shape")` 的**取源一律走 Go 半** `chsBaseline`（与 py 半同一冻结真源 ·
// ✗ 非第二条实现），**✗ 不再读**兄弟件 `scripts/gates/check-help-shape.py` 取 `BASELINE`
// ⇒ 该 py 件对本件**解除读耦合**（可退）。★「两半同值」由两侧各自判据 ＋门113 维持，✗ 不取消。
// ★ 两态：py **不在盘** ⇒ 取到 Go 半（✗ 不 fail-closed）；py **在盘**（放一枚**会抛异常的空壳**）
//
//	⇒ 读数**逐字不变**（证明真没读它）—— 这正是「解耦」的机器面。
//
// ★ 哑化负控：抽掉 csfBridge 里 `if op == "help_shape"` 那段（或恢复成读 py）⇒ 本用例 FAIL（带行号）。
func TestCsfBridgeHelpShapeSourceIsGoHalf(t *testing.T) {
	root := t.TempDir() // 仓外空夹具 · 其中**无** check-help-shape.py
	if _, err := os.Stat(filepath.Join(root, "scripts", "gates", "check-help-shape.py")); err == nil {
		t.Fatal("前置不成立：夹具里竟有 check-help-shape.py")
	}
	check := func(tag string) map[string]any {
		t.Helper()
		res, errStr := csfBridge(root, "help_shape", io.Discard)
		if errStr != "" {
			t.Fatalf("%s：取源该走 Go 半 chsBaseline（✗ 不 fail-closed）：%q", tag, errStr)
		}
		bl, _ := res["baseline"].(map[string]any)
		if len(bl) == 0 {
			t.Fatalf("%s：读数缺 baseline：%v", tag, res)
		}
		for _, k := range []string{"help --all", "help", "help dangerous", "help config"} {
			m, _ := bl[k].(map[string]any)
			if m == nil || m["lines"] == nil || m["bytes"] == nil {
				t.Errorf("%s：baseline 缺 %q 的 lines/bytes：%v", tag, k, bl[k])
			}
		}
		return bl
	}
	blOff := check("py 不在盘")
	// py 在盘（会抛异常的空壳）⇒ 读数**逐字不变**（真没读它 ⇒ 解耦实证）。
	py := filepath.Join(root, "scripts", "gates", "check-help-shape.py")
	if err := os.MkdirAll(filepath.Dir(py), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(py, []byte("raise SystemExit('stub')\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	blOn := check("py 在盘（空壳）")
	if !reflect.DeepEqual(blOff, blOn) {
		t.Fatalf("两态读数该逐字同（取源不读 py）：\n  不在盘=%v\n  在盘  =%v", blOff, blOn)
	}
}

// csfCliContractFixture —— 仓外夹具：只放 cli_contract 桥的 **Go 半**输入面
// （`core/cmd/zerg/main.go` 的最小命令树 + `testdata/cli-matrix.json`），**无** check-cli-contract.py。
func csfCliContractFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	mk := func(rel, body string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mk("core/cmd/zerg/main.go",
		"package main\n\nfunc buildCommands() []*command {\n"+
			"	commands = []*command{\n"+
			"		{path: []string{\"gate\", \"check\"}},\n"+
			"	}\n	return commands\n}\n")
	mk("core/cmd/zerg/testdata/cli-matrix.json",
		`{"cases": [{"want_rc": 2}, {"want_rc": 0}, {"want_rc": 2}, {"argv": ["x"]}]}`)
	return root
}

// TestCsfBridgeCliContractRetiredFallback —— 2026-10-07 退役批：兄弟件
// `scripts/gates/check-cli-contract.py` 退役（不在盘）⇒ `csfBridge("cli_contract")` 回落**同判据的
// Go 半** `clicParseTree` / `clicLoadMatrix`（✗ 不因缺 py 而「现读重算失败 ⇒ 不给结论」fail-closed）。
// ★ 成对：兄弟件在盘（哪怕是个空壳）⇒ 仍走 importlib 那条桥（单一真源不变）。
// ★ 负控：两半皆缺（无 py ∧ Go 半输入面缺）⇒ fail-closed（✗ 静默出数）。
// ★ 哑化负控：抽掉 csfBridge 里 `if op == "cli_contract"` 那段 ⇒ 本用例第①②格 FAIL（带行号）。
func TestCsfBridgeCliContractRetiredFallback(t *testing.T) {
	root := csfCliContractFixture(t)
	if _, err := os.Stat(filepath.Join(root, "scripts", "gates", "check-cli-contract.py")); err == nil {
		t.Fatal("前置不成立：夹具里竟有 check-cli-contract.py")
	}
	res, errStr := csfBridge(root, "cli_contract", io.Discard)
	if errStr != "" {
		t.Fatalf("退役态该回落 Go 半 clicParseTree/clicLoadMatrix（✗ 不 fail-closed）：%q", errStr)
	}
	if n, _ := csfNumInt(res["n_cmd"]); n != 1 {
		t.Fatalf("回落 n_cmd 期望 1（夹具一件命令）实得 %v", res["n_cmd"])
	}
	if n, _ := csfNumInt(res["n_mf"]); n != 2 {
		t.Fatalf("回落 n_mf 期望 2（want_rc≠0 两格）实得 %v", res["n_mf"])
	}
	// 负控（两半皆缺 ⇒ fail-closed）：裸临时目录（无 main.go / cli-matrix.json）
	if _, e := csfBridge(t.TempDir(), "cli_contract", io.Discard); e == "" {
		t.Fatal("两半皆缺该 fail-closed（✗ 静默出数）")
	}
	// 成对负控：把（空壳）兄弟件放回盘 ⇒ 仍走 importlib 桥（空壳 ⇒ 桥必报错）。
	if err := os.MkdirAll(filepath.Join(root, "scripts", "gates"), 0o755); err != nil {
		t.Fatal(err)
	}
	py := filepath.Join(root, "scripts", "gates", "check-cli-contract.py")
	if err := os.WriteFile(py, []byte("raise SystemExit('stub')\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, e2 := csfBridge(root, "cli_contract", io.Discard); e2 == "" {
		t.Fatalf("兄弟件在盘时该走 importlib 桥（成对负控：空壳 py 应让桥报错）")
	}
}

// TestCsfBridgeApproveBackfillRetiredFallback —— 同族（2026-10-07 退役批）：兄弟件
// `scripts/gates/check-approve-backfill.py` 退役（不在盘）⇒ `csfBridge("approve_backfill")` 回落
// **同判据的 Go 半常量** `cafbDevEditSHA256` / `cafbTicketRel`（✗ 不因缺 py 而「件读不到 ⇒ 取不到」）。
// ★ 哑化负控：抽掉 csfBridge 里 `if op == "approve_backfill"` 那段 ⇒ 本用例第①格 FAIL（带行号）。
func TestCsfBridgeApproveBackfillRetiredFallback(t *testing.T) {
	root := t.TempDir() // 仓外空夹具 · 其中**无** check-approve-backfill.py
	if _, err := os.Stat(filepath.Join(root, "scripts", "gates", "check-approve-backfill.py")); err == nil {
		t.Fatal("前置不成立：夹具里竟有 check-approve-backfill.py")
	}
	res, errStr := csfBridge(root, "approve_backfill", io.Discard)
	if errStr != "" {
		t.Fatalf("退役态该回落 Go 半常量 cafbDevEditSHA256/cafbTicketRel（✗ 不 fail-closed）：%q", errStr)
	}
	if csfPctS(res["sha"]) != cafbDevEditSHA256 || csfPctS(res["rel"]) != cafbTicketRel {
		t.Fatalf("回落读数错：sha=%v rel=%v（期望 %s / %s）", res["sha"], res["rel"], cafbDevEditSHA256, cafbTicketRel)
	}
	// 成对负控：把（空壳）兄弟件放回盘 ⇒ 仍走 importlib 桥（空壳 ⇒ 桥必报错）。
	py := filepath.Join(root, "scripts", "gates", "check-approve-backfill.py")
	if err := os.MkdirAll(filepath.Dir(py), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(py, []byte("raise SystemExit('stub')\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, e2 := csfBridge(root, "approve_backfill", io.Discard); e2 == "" {
		t.Fatalf("兄弟件在盘时该走 importlib 桥（成对负控：空壳 py 应让桥报错）")
	}
}
