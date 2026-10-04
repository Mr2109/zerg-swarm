// gate_check_health_test.go —— Go 真搬的**成对负控**定向单测（M2 · 4.2 附加四条之「负控成对」）。
//
// ✗ 禁恒绿：本件同时断言「必然判绿」「必然判红」「不给结论」三态 —— 任一态被写死都必失败。
// 用例名：TestCheckGateHealthGatePort（定向：go test -run TestCheckGateHealthGatePort）。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// gchGoodShell —— 合规壳件文本（含解释器闭集 ＋ 目录派生 ＋ 无写死名件路径）。
const gchGoodShell = "package main\n" +
	"var gateCheckInterpreter = map[string]string{\".py\": \"python3\", \".sh\": \"bash\"}\n" +
	"err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error { return nil })\n"

// gchFixture —— 在 t.TempDir() 下造一棵「壳件 ＋ scripts/」夹具根（仓外）。
func gchFixture(t *testing.T, shell string, extra map[string]string) string {
	t.Helper()
	root := t.TempDir()
	zdir := filepath.Join(root, "core", "cmd", "zerg")
	if err := os.MkdirAll(zdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if shell != "" {
		if err := os.WriteFile(filepath.Join(zdir, "family_gate_check.go"), []byte(shell), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// ★ 门80 扫描面（`GAP-20261001-75` 路A）：第二件（inproc 分发表）也在扫描面内 ⇒ 夹具须同时落它。
	if err := os.WriteFile(filepath.Join(zdir, "gate_inproc_table.go"), []byte("package main\n// inproc 分发表（夹具）\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for rel, body := range extra {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func gchRun(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := gateCheckHealthMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

func TestCheckGateHealthGatePort(t *testing.T) {
	// ── 正控（必然判绿）：合规壳件 ＋ 名录/注册表计数相符 ⇒ rc=0 ───────────
	green := gchFixture(t, gchGoodShell, map[string]string{
		"scripts/gates/a.py":                    "# a\n",
		"scripts/b.sh":                          "# b\n",
		"scripts/gates/check-judge-registry.py": "# 模块副本（只判在盘）\n",
		"scripts/gates/judge-registry.json":     "{\"count\": 3}\n",
	})
	rcGreen, outGreen, _ := gchRun(t, "--root", green)
	if rcGreen != 0 || !strings.Contains(outGreen, "✓ 全绿") {
		t.Fatalf("正控失败：合规夹具应 rc=0，得 rc=%d out=%q", rcGreen, outGreen)
	}
	if !strings.Contains(outGreen, "注册表条数 == 现读名录件数（3）") {
		t.Fatalf("正控失败：应打「注册表条数 == 现读名录件数（3）」，得 out=%q", outGreen)
	}

	// ── 负控①（必然判红）：壳件出现按名名单标记（gateCheckAllow）⇒ rc=1 ─────
	rcName, outName, _ := gchRun(t, "--root", gchFixture(t,
		gchGoodShell+"var gateCheckAllow = 3\n", nil))
	if rcName != 1 || !strings.Contains(outName, "按名名单/特殊化标记") {
		t.Fatalf("负控①-按名名单 失败：得 rc=%d out=%q", rcName, outName)
	}
	// 成对（✗ 禁恒绿）：绿 ↔ 红 必须不同码
	if rcGreen == rcName {
		t.Fatalf("成对断言失败：正控 rc=%d 与负控① rc=%d 同码 ⇒ 无鉴别力", rcGreen, rcName)
	}

	// ── 负控②：解释器闭集被改宽 ⇒ rc=1 ───────────────────────────────────
	rcWide, outWide, _ := gchRun(t, "--root", gchFixture(t,
		"var gateCheckInterpreter = map[string]string{\".py\": \"python3\", \".sh\": \"bash\", \"\": \"sh\"}\n"+
			"filepath.WalkDir\n", nil))
	if rcWide != 1 || !strings.Contains(outWide, "解释器闭集不是") {
		t.Fatalf("负控②-闭集改宽 失败：得 rc=%d out=%q", rcWide, outWide)
	}

	// ── 负控③：缺 filepath.WalkDir（名录非目录派生）⇒ rc=1 ────────────────
	rcNoWalk, outNoWalk, _ := gchRun(t, "--root", gchFixture(t,
		"var gateCheckInterpreter = map[string]string{\".py\": \"python3\", \".sh\": \"bash\"}\n", nil))
	if rcNoWalk != 1 || !strings.Contains(outNoWalk, "目录派生") {
		t.Fatalf("负控③-缺 WalkDir 失败：得 rc=%d out=%q", rcNoWalk, outNoWalk)
	}

	// ── 负控④：壳件写死 `\"scripts/…\"` 名件路径 ⇒ rc=1 ───────────────────
	rcHard, outHard, _ := gchRun(t, "--root", gchFixture(t,
		gchGoodShell+"var p = \"scripts/gates/foo\"\n", nil))
	if rcHard != 1 || !strings.Contains(outHard, "名件路径") {
		t.Fatalf("负控④-写死名件路径 失败：得 rc=%d out=%q", rcHard, outHard)
	}

	// ── 负控⑥（门80 扫描面 · `GAP-20261001-75` 路A）：按名名单标记落在**第二件**
	//    `gate_inproc_table.go`（而非壳件本体）⇒ 也必须被抓（证扫描面确实覆盖该件）。
	rcShell2, outShell2, _ := gchRun(t, "--root", gchFixture(t, "", map[string]string{
		"core/cmd/zerg/family_gate_check.go": gchGoodShell,
		"core/cmd/zerg/gate_inproc_table.go": gchGoodShell + "var gateCheckDeny = 1\n",
	}))
	if rcShell2 != 1 || !strings.Contains(outShell2, "按名名单/特殊化标记") {
		t.Fatalf("负控⑥-第二壳件扫描面 失败：得 rc=%d out=%q", rcShell2, outShell2)
	}

	// ── 负控⑤：注册表计数 ≠ 现读名录件数 ⇒ rc=1 ──────────────────────────
	rcMism, outMism, _ := gchRun(t, "--root", gchFixture(t, gchGoodShell, map[string]string{
		"scripts/gates/a.py":                    "# a\n",
		"scripts/gates/check-judge-registry.py": "# 模块副本\n",
		"scripts/gates/judge-registry.json":     "{\"count\": 99}\n",
	}))
	if rcMism != 1 || !strings.Contains(outMism, "≠ 现读名录件数") {
		t.Fatalf("负控⑤-注册表计数不符 失败：得 rc=%d out=%q", rcMism, outMism)
	}

	// ── 不给结论（≠ 绿）：root 无壳件 ⇒ rc=2 ───────────────────────────────
	rcNoShell, outNoShell, _ := gchRun(t, "--root", gchFixture(t, "", nil))
	if rcNoShell != 2 || !strings.Contains(outNoShell, "壳件读不到") {
		t.Fatalf("取不到输入失败：得 rc=%d out=%q", rcNoShell, outNoShell)
	}
	if rcNoShell == rcGreen {
		t.Fatalf("成对断言失败：不给结论 rc=%d 与正控 rc=%d 同码", rcNoShell, rcGreen)
	}

	// ── 取不到输入：root 路径不存在 ⇒ rc=2 ─────────────────────────────────
	rcMiss, outMiss, _ := gchRun(t, "--root", filepath.Join(t.TempDir(), "nope"))
	if rcMiss != 2 || !strings.Contains(outMiss, "壳件读不到") {
		t.Fatalf("根不存在 失败：得 rc=%d out=%q", rcMiss, outMiss)
	}

	// ── 语法错面：未知旗标 ⇒ rc=2（stderr 明说）───────────────────────────
	rcFlag, _, errFlag := gchRun(t, "--no-such-flag")
	if rcFlag != 2 || !strings.Contains(errFlag, "unrecognized arguments: --no-such-flag") {
		t.Fatalf("未知旗标失败：得 rc=%d stderr=%q", rcFlag, errFlag)
	}

	// ── 语法错面：`--root` 缺值 ⇒ rc=2（argparse 口径）─────────────────────
	rcNoVal, _, errNoVal := gchRun(t, "--root")
	if rcNoVal != 2 || !strings.Contains(errNoVal, "argument --root: expected one argument") {
		t.Fatalf("缺值失败：得 rc=%d stderr=%q", rcNoVal, errNoVal)
	}

	// ── `-h` ⇒ rc=0 且印 argparse usage ─────────────────────────────────────
	rcHelp, outHelp, _ := gchRun(t, "-h")
	if rcHelp != 0 || !strings.Contains(outHelp, "usage: gate-check-health.py") {
		t.Fatalf("-h 失败：得 rc=%d out=%q", rcHelp, outHelp)
	}

	// ── 机器面：`--json`（同结构多渲染 · 键序 root/checks/problems/notes）───
	rcJSON, outJSON, _ := gchRun(t, "--root", green, "--json")
	if rcJSON != 0 || !strings.Contains(outJSON, "\"root\":") ||
		!strings.Contains(outJSON, "\"problems\": []") {
		t.Fatalf("--json 失败：得 rc=%d out=%q", rcJSON, outJSON)
	}

	// ── `--self-test` 五档自身必须全过（防恒绿/恒红）────────────────────────
	rcST, outST, _ := gchRun(t, "--self-test")
	if rcST != 0 || !strings.Contains(outST, "负控④：取不到版本 ⇒ None") {
		t.Fatalf("--self-test 失败：得 rc=%d out=%q", rcST, outST)
	}
}
