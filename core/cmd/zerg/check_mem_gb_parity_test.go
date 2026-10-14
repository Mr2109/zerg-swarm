// check_mem_gb_parity_test.go —— `check-mem-gb-parity`（两侧同名模型 mem_gb 对拍门）的**三态 + 负控**定向单测。
//
// 口径：仓外夹具（`t.TempDir()` · ✗ 不脏真仓）。用例名公共前缀 `TestMemGBParity`，
// 定向：`go test ./cmd/zerg/ -count=1 -run 'MemGBParity'`。
//
// 三态（照件头）：
//
//	甲 值异 · 两侧都不标口径 ⇒ rc=1 并**逐字打两侧值** ＋ 点名叫缺标记的一侧；
//	乙 值异 · 两侧都标（字段形 `mem_gb_scope` / 就近注记形）⇒ rc=0 放行 ＋ 逐条理由；
//	丙 值同 ⇒ rc=0；
//	＋ 只标一侧 ⇒ 仍 rc=1（**负控**：✗ 一侧标就放行）· 表外 scope 值不算标记 ·
//	  真源读不到 / 可比值 0 条 ⇒ 不给结论（rc=2）。
//
// ★ 成对（✗ 禁恒绿）：`TestMemGBParityNegativeControl` 对同一条候选「两侧都标 ⇒ 绿」对
// 「只标一侧 ⇒ 红」「两侧都不标 ⇒ 红」成组断言 —— 若判据体不再真读口径标记，该用例必 FAIL（带行号）。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mgpRunT(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := checkMemGBParityGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

func mgpWriteT(t *testing.T, root, rel, body string) string {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

const (
	mgpTChildBad  = "Alpha:\n  file: /tmp/a.gguf\n  mem_gb: 86\nBeta:\n  file: /tmp/b.gguf\n  mem_gb: 11\n"
	mgpTChildSame = "Alpha:\n  file: /tmp/a.gguf\n  mem_gb: 34\nBeta:\n  file: /tmp/b.gguf\n  mem_gb: 11\n"
	mgpTChildScop = "Alpha:\n  file: /tmp/a.gguf\n  mem_gb: 86\n  mem_gb_scope: gate-val\nBeta:\n  file: /tmp/b.gguf\n  mem_gb: 11\n"
	mgpTChildNote = "# 本机子端注册表：mem_gb = 闸门拍值（口径 gate-val · 故意报大·fail-closed）\n" + mgpTChildBad
)

// mgpFleetT —— 主控侧夹具（`mark` 形态："" = 不标 / "scope" = 字段形 / "note" = 段头注记形）。
func mgpFleetT(mark string) string {
	head := "models:\n"
	alpha := "  Alpha:\n    - { host: boxy, backend: llama-server, mem_gb: 34 }\n"
	switch mark {
	case "scope":
		alpha = "  Alpha:\n    - { host: boxy, backend: llama-server, mem_gb: 34, mem_gb_scope: usage }\n"
	case "note":
		head = "models:\n  # mem_gb 字段口径规约：本段 mem_gb = 实测峰值 + 余量（占用口径）\n"
	}
	return head + alpha + "  Beta:\n    - { host: boxy, backend: llama-server, mem_gb: 11 }\n"
}

// mgpRootT —— 仓外夹具根：主控侧件 ＋ 子端注册表件（路径由测试传，✗ 不碰真仓）。
func mgpRootT(t *testing.T, fleet, child string) (string, string, string) {
	t.Helper()
	root := t.TempDir()
	fp := mgpWriteT(t, root, "fx/fleet.yaml", fleet)
	cp := mgpWriteT(t, root, "fx/agent_models.yaml", child)
	return root, fp, cp
}

// TestMemGBParityStates —— 三态 ＋ 缺席 ＋ 可比值 0 条（成对：绿↔红不同码）。
func TestMemGBParityStates(t *testing.T) {
	// ── 甲 · 值异 · 两侧都不标 ⇒ rc=1 ＋ 逐字两侧值 ──
	rootA, fpA, cpA := mgpRootT(t, mgpFleetT(""), mgpTChildBad)
	rcA, outA, errA := mgpRunT(t, "--check", "--root", rootA, "--fleet", fpA, "--child", cpA, "--host", "boxy")
	if rcA != 1 {
		t.Fatalf("甲·值异未标口径 应 rc=1，得 rc=%d out=%q err=%q", rcA, outA, errA)
	}
	want := "Alpha 两侧 `mem_gb` 值不同且未两侧标口径（主控侧 34 ⟷ 子端侧 86；缺标记的一侧 = 两侧）"
	if !strings.Contains(outA, want) {
		t.Fatalf("甲·判词不符（须逐字打两侧值），want=%q out=%q", want, outA)
	}
	if !strings.Contains(outA, "值同 1") {
		t.Fatalf("甲·Beta(11/11) 应计入值同：out=%q", outA)
	}

	// ── 乙 · 值异 · 两侧都标（字段形）⇒ rc=0 ＋ 理由 ──
	rootB, fpB, cpB := mgpRootT(t, mgpFleetT("scope"), mgpTChildScop)
	rcB, outB, _ := mgpRunT(t, "--root", rootB, "--fleet", fpB, "--child", cpB, "--host", "boxy")
	if rcB != 0 {
		t.Fatalf("乙·两侧都标（字段形）应 rc=0，得 rc=%d out=%q", rcB, outB)
	}
	if !strings.Contains(outB, "字段 mem_gb_scope=usage") || !strings.Contains(outB, "字段 mem_gb_scope=gate-val") {
		t.Fatalf("乙·豁免理由未逐条打印两侧标记形态：out=%q", outB)
	}

	// ── 乙' · 值异 · 两侧都标（就近注记形）⇒ rc=0 ──
	rootB2, fpB2, cpB2 := mgpRootT(t, mgpFleetT("note"), mgpTChildNote)
	rcB2, outB2, _ := mgpRunT(t, "--root", rootB2, "--fleet", fpB2, "--child", cpB2, "--host", "boxy")
	if rcB2 != 0 || !strings.Contains(outB2, "段/文件头注记") {
		t.Fatalf("乙'·两侧都标（注记形）应 rc=0 且理由写注记形，得 rc=%d out=%q", rcB2, outB2)
	}

	// ── 丙 · 值同 ⇒ rc=0 ──
	rootC, fpC, cpC := mgpRootT(t, mgpFleetT(""), mgpTChildSame)
	rcC, outC, _ := mgpRunT(t, "--root", rootC, "--fleet", fpC, "--child", cpC, "--host", "boxy")
	if rcC != 0 {
		t.Fatalf("丙·值同应 rc=0，得 rc=%d out=%q", rcC, outC)
	}

	// 成对（✗ 禁恒绿）：红(甲) ↔ 绿(乙/乙'/丙) 必须不同码。
	if rcA == rcB || rcA == rcB2 || rcA == rcC {
		t.Fatalf("成对断言失败：红 rc=%d 与绿 rc=%d/%d/%d 同码 ⇒ 无鉴别力", rcA, rcB, rcB2, rcC)
	}

	// ── 值异 · 只标一侧（主控标、子端不标）⇒ rc=1 ＋ 点名缺标记的一侧 = 子端侧 ──
	rootD, fpD, cpD := mgpRootT(t, mgpFleetT("scope"), mgpTChildBad)
	rcD, outD, _ := mgpRunT(t, "--root", rootD, "--fleet", fpD, "--child", cpD, "--host", "boxy")
	if rcD != 1 || !strings.Contains(outD, "缺标记的一侧 = 子端侧") {
		t.Fatalf("只标一侧应 rc=1 且点名子端侧，得 rc=%d out=%q", rcD, outD)
	}

	// ── 表外 scope 值不算标记 ⇒ rc=1（值闭集口径的牙）──
	rootE, fpE, cpE := mgpRootT(t,
		"models:\n  Alpha:\n    - { host: boxy, backend: llama-server, mem_gb: 34, mem_gb_scope: whatever }\n", mgpTChildScop)
	rcE, outE, _ := mgpRunT(t, "--root", rootE, "--fleet", fpE, "--child", cpE, "--host", "boxy")
	if rcE != 1 {
		t.Fatalf("表外 mem_gb_scope 值应 rc=1，得 rc=%d out=%q", rcE, outE)
	}

	// ── 可比值 0 条（机名不匹配 / 一侧全无 mem_gb）⇒ 不给结论（rc=2 · 空 ≠ 缺）──
	rootF, fpF, cpF := mgpRootT(t, mgpFleetT(""), mgpTChildBad)
	rcF, _, errF := mgpRunT(t, "--root", rootF, "--fleet", fpF, "--child", cpF, "--host", "otherbox")
	if rcF != 2 || !strings.Contains(errF, "空 ≠ 缺") {
		t.Fatalf("可比值 0 条应 rc=2 且报「空 ≠ 缺」，得 rc=%d err=%q", rcF, errF)
	}

	// ── 缺子端注册表件 ⇒ 不给结论（rc=2）──
	rootG, fpG, _ := mgpRootT(t, mgpFleetT(""), mgpTChildBad)
	rcG, _, errG := mgpRunT(t, "--root", rootG, "--fleet", fpG, "--child", filepath.Join(rootG, "fx", "nope.yaml"), "--host", "boxy")
	if rcG != 2 || !strings.Contains(errG, "读不到子端注册表") {
		t.Fatalf("缺子端件应 rc=2 且报「读不到子端注册表」，得 rc=%d err=%q", rcG, errG)
	}
}

// TestMemGBParitySubstanceOnlyMarkedBoth —— ★ 负控（哑化「口径标记」读取支）：只标一侧 ⇒ 红 ≠ 两侧都标 ⇒ 绿。
//
// 若判据体改成「看一眼某侧有标记就放行」，本用例必 FAIL（`t.Fatalf` 打 件:行）。
func TestMemGBParitySubstanceOnlyMarkedBoth(t *testing.T) {
	fleet := mgpFleetT("scope")
	childBare := mgpTChildBad
	childMarked := mgpTChildScop

	pair := mgpJudge(fleet, childMarked, "boxy")
	if pair.Rc != 0 || len(pair.Excused) != 1 {
		t.Fatalf("两侧都标 应 rc=0 且豁免 1 条，得 rc=%d excused=%v red=%v ⇒ 口径标记未生效", pair.Rc, pair.Excused, pair.Red)
	}
	half := mgpJudge(fleet, childBare, "boxy")
	if half.Rc != 1 || len(half.Red) != 1 {
		t.Fatalf("只标一侧 应 rc=1（✗ 一侧标就放行），得 rc=%d red=%v", half.Rc, half.Red)
	}
	none := mgpJudge(mgpFleetT(""), childBare, "boxy")
	if none.Rc != 1 {
		t.Fatalf("两侧都不标 应 rc=1，得 rc=%d", none.Rc)
	}
	// 成对：三格不同码（0/1/1）且「都标」与「只标一侧」必须可分。
	if pair.Rc == half.Rc {
		t.Fatalf("成对断言失败：都标 rc=%d 与 只标一侧 rc=%d 同码 ⇒ 判据不真读两侧标记", pair.Rc, half.Rc)
	}
}

// TestMemGBParityLocalDerivation —— 本机落点**现读**推导：`deploy/com.zerg.agent-<机>.plist` 的
// `--registry` 实参路径在盘上 ⇒ 机名 + 子端件都从它取（✗ 不写死机名、✗ 不猜 `~/agent/` 路径）。
func TestMemGBParityLocalDerivation(t *testing.T) {
	root := t.TempDir()
	cp := mgpWriteT(t, root, "machine/agent_models.yaml", mgpTChildSame)
	mgpWriteT(t, root, "gateway/fleet.yaml", mgpFleetT(""))
	mgpWriteT(t, root, "deploy/com.zerg.agent-boxy.plist",
		"<key>ProgramArguments</key><array>\n"+
			"<string>--machine</string><string>boxy</string>\n"+
			"<string>"+mgpRegArg+"</string><string>"+cp+"</string>\n"+
			"<string>--log-file</string><string>/tmp/x.log</string>\n"+
			"</array>\n")
	rc, out, errs := mgpRunT(t, "--check", "--root", root)
	if rc != 0 {
		t.Fatalf("本机推导档应 rc=0（机名 boxy 取自 plist · 值 34/34 同），得 rc=%d out=%q err=%q", rc, out, errs)
	}
	if !strings.Contains(out, "本机 boxy") || !strings.Contains(out, cp) {
		t.Fatalf("应报「本机 boxy」且子端件 = plist 的 --registry 实参：out=%q", out)
	}
	// 多枚 plist 命中（两枚注册表都在盘）⇒ fail-closed rc=2（✗ 不挑一枚蒙）
	cp2 := mgpWriteT(t, root, "machine2/agent_models.yaml", mgpTChildSame)
	mgpWriteT(t, root, "deploy/com.zerg.agent-other.plist",
		"<array><string>"+mgpRegArg+"</string><string>"+cp2+"</string></array>\n")
	rc2, _, err2 := mgpRunT(t, "--check", "--root", root)
	if rc2 != 2 || !strings.Contains(err2, "本机子端定不了") {
		t.Fatalf("多枚命中应 rc=2 且报「本机子端定不了」，得 rc=%d err=%q", rc2, err2)
	}
}

// TestMemGBParitySelfTest —— 合成自检（纯函数夹具 ＋ 仓外临时树）⇒ rc=0。
func TestMemGBParitySelfTest(t *testing.T) {
	rc, out, errs := mgpRunT(t, "--self-test")
	if rc != 0 {
		t.Fatalf("自检应 rc=0，得 rc=%d out=%q err=%q", rc, out, errs)
	}
	if !strings.Contains(out, "0 格不过") {
		t.Fatalf("自检应报「0 格不过」：out=%q", out)
	}
}

// TestMemGBParityUsage —— 用法档：未知旗标 / 两档互斥 / `-h` ⇒ 退码与文案。
func TestMemGBParityUsage(t *testing.T) {
	if rc, _, _ := mgpRunT(t, "--nope"); rc != 2 {
		t.Fatalf("未知旗标 应 rc=2，得 rc=%d", rc)
	}
	if rc, _, _ := mgpRunT(t, "--check", "--self-test"); rc != 2 {
		t.Fatalf("`--check` 与 `--self-test` 同给 应 rc=2，得 rc=%d", rc)
	}
	if rc, out, _ := mgpRunT(t, "-h"); rc != 0 || !strings.Contains(out, mgpJudgeKey) {
		t.Fatalf("`-h` 应 rc=0 且带件名 %s，得 rc=%d out=%q", mgpJudgeKey, rc, out)
	}
}
