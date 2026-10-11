// gap_20261010_25_test.go —— 缺口 `GAP-20261010-25` 的判据。
//
// 症状（一句话）：同族多个 gate 判定点仍在抠 `gate ls`/`--list` 的**人面中文读数**（脆、易随文案漂移）。
// 正解（照先例 `preview_gate_steps.go`）：**步表机读面优先**（`precommit-gates.sh --list --json` 的单对象
// `{"count":N,"steps":[…]}`），**人面只作退化路**；机读面在场而形状坏/自证不过 ⇒ **响亮失败，不给结论**
// （✗ 不许静默退回人面把读数当绿）。
//
// 判据设计：合成门禁脚本让**人面**（`--list`）与**机读面**（`--list --json`）给出**不同**的步名
// （`人面步N` vs `机读步N`）⇒「读的是哪一面」一眼可判。
//
// 两态纪律：本件断言的是**改后**行为。⇒ 对**改前源码**跑（或把机读面那一支哑化）⇒ 必红
// （改前只抠人面，读回 `人面步N`）。落地（改后）⇒ 绿。
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── 合成门禁脚本的三张面（人面 / 机读面 / 形状坏的机读面）─────────────────────────
const (
	gap25HumanFace = "── 步骤清单（scope 模式 名称）──\n" +
		"   go    empty  人面步一\n" +
		"   gates tri    人面步二\n" +
		"共 2 步\n"
	gap25MachineFace = `{"count":2,"steps":[{"scope":"go","mode":"empty","name":"机读步一"},` +
		`{"scope":"gates","mode":"tri","name":"机读步二"}]}` + "\n"
	// 形状坏：`count`=5 但 `steps` 只 1 条 ⇒ 机读面**在场**而计数自证不过。
	gap25BrokenFace = `{"count":5,"steps":[{"scope":"go","mode":"empty","name":"机读步一"}]}` + "\n"
)

// gap25Root 造一棵合成仓（只放门禁脚本）；mode 决定机读面那一支：
//
//	"both"   —— 机读面齐（`--json` 出真机读面）
//	"broken" —— 机读面在场而形状坏（计数自证不过）
//	"human"  —— 脚本不认 `--json`（`--list --json` 只出人面）⇒ 机读面**不在**（走退化路）
func gap25Root(t *testing.T, mode string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "scripts", "gates")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("合成目录造不出来：%v", err)
	}
	human := filepath.Join(dir, ".human-face")
	mach := filepath.Join(dir, ".machine-face")
	if err := os.WriteFile(human, []byte(gap25HumanFace), 0o644); err != nil {
		t.Fatal(err)
	}
	machBody := gap25MachineFace
	switch mode {
	case "broken":
		machBody = gap25BrokenFace
	case "human":
		machBody = gap25HumanFace // `--list --json` 只出人面 ⇒ 机读面不在
	}
	if err := os.WriteFile(mach, []byte(machBody), 0o644); err != nil {
		t.Fatal(err)
	}
	script := "#!/usr/bin/env bash\n" +
		"case \"${1:-}\" in\n" +
		"  --list)\n" +
		"    if [ \"${2:-}\" = \"--json\" ]; then\n" +
		"      cat \"" + mach + "\"\n" +
		"    else\n" +
		"      cat \"" + human + "\"\n" +
		"    fi ;;\n" +
		"  *) exit 2 ;;\n" +
		"esac\n"
	if err := os.WriteFile(filepath.Join(dir, "precommit-gates.sh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func gap25Join(t *testing.T, names []string) string {
	t.Helper()
	return strings.Join(names, ",")
}

// ── ①–⑤ 机读面优先（改后行为：读回 `机读步N`；改前会读回 `人面步N` ⇒ 本件对改前必红）──────────

func TestGap20261010_25_GateFullStepNamesMachineFirst(t *testing.T) {
	root := gap25Root(t, "both")
	names, err := gateFullStepNames(root)
	if err != nil {
		t.Fatalf("机读面在场却报错（%v）—— 该读机读面", err)
	}
	if got := gap25Join(t, names); got != "机读步一,机读步二" {
		t.Errorf("gateFullStepNames 该读**机读面**（机读步一,机读步二），实得 %q（读到人面 = 改前行为）", got)
	}
}

func TestGap20261010_25_SlgStepCmdsMachineFirst(t *testing.T) {
	root := gap25Root(t, "both")
	names, _, ok := slgStepCmds(root)
	if !ok {
		t.Fatal("机读面在场却判不了（ok=false）—— 该读机读面")
	}
	if got := gap25Join(t, names); got != "机读步一,机读步二" {
		t.Errorf("slgStepCmds 该读**机读面**（机读步一,机读步二），实得 %q（读到人面 = 改前行为）", got)
	}
}

func TestGap20261010_25_ImpactStepTablePullMachineFirst(t *testing.T) {
	root := gap25Root(t, "both")
	tbl := impactStepTablePull(root)
	if tbl.Status != "取值" {
		t.Fatalf("机读面在场该取到数，实得 status=%s · detail=%s", tbl.Status, tbl.Detail)
	}
	if tbl.Total != 2 || tbl.Listed != 2 {
		t.Errorf("机读面 count=2 ⇒ Total/Listed 该都是 2，实得 %d/%d", tbl.Total, tbl.Listed)
	}
	names := []string{}
	for _, r := range tbl.Rows {
		names = append(names, r.Name)
	}
	if got := gap25Join(t, names); got != "机读步一,机读步二" {
		t.Errorf("impactStepTablePull 该读**机读面**（机读步一,机读步二），实得 %q（读到人面 = 改前行为）", got)
	}
}

func TestGap20261010_25_GateStepIndexMachineFirst(t *testing.T) {
	root := gap25Root(t, "both")
	idx, ok := gateStepIndex(root, "机读步二")
	if !ok || idx != 2 {
		t.Errorf("gateStepIndex 该在机读面里把这步定到序号 2，实得 idx=%d ok=%v", idx, ok)
	}
	if _, ok := gateStepIndex(root, "人面步一"); ok {
		t.Errorf("gateStepIndex 不该拿人面名命中（人面只作退化路）")
	}
}

func TestGap20261010_25_AsParseListMachineFirst(t *testing.T) {
	f := filepath.Join(t.TempDir(), "steps.txt")
	if err := os.WriteFile(f, []byte(gap25MachineFace), 0o644); err != nil {
		t.Fatal(err)
	}
	names, err := asParseList(f)
	if err != nil {
		t.Fatalf("步表件的机读形在场却报错（%v）", err)
	}
	if got := gap25Join(t, names); got != "机读步一,机读步二" {
		t.Errorf("asParseList 该读**机读面**（机读步一,机读步二），实得 %q", got)
	}
}

// ── ⑥ 形状坏：机读面在场而自证不过 ⇒ 响亮失败（不给结论），✗ 不许退回人面当绿 ──────────────────

func TestGap20261010_25_BrokenMachineFaceIsLoud(t *testing.T) {
	root := gap25Root(t, "broken")

	if _, err := gateFullStepNames(root); err == nil || !strings.Contains(err.Error(), "形状坏") {
		t.Errorf("gateFullStepNames：机读面形状坏 ⇒ 该响亮失败（err 含「形状坏」），实得 err=%v", err)
	}
	if _, _, ok := slgStepCmds(root); ok {
		t.Errorf("slgStepCmds：机读面形状坏 ⇒ 该 ok=false（不给结论），✗ 不许退回人面")
	}
	if tbl := impactStepTablePull(root); tbl.Status == "取值" || !strings.Contains(tbl.Detail, "形状坏") {
		t.Errorf("impactStepTablePull：机读面形状坏 ⇒ 不许给「取值」且 Detail 该点「形状坏」，实得 status=%s · detail=%s", tbl.Status, tbl.Detail)
	}
	if _, ok := gateStepIndex(root, "机读步一"); ok {
		t.Errorf("gateStepIndex：机读面形状坏 ⇒ 不给结论（✗ 不许退回人面）")
	}

	f := filepath.Join(t.TempDir(), "broken.txt")
	if err := os.WriteFile(f, []byte(gap25BrokenFace), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := asParseList(f); err == nil || !strings.Contains(err.Error(), "形状坏") {
		t.Errorf("asParseList：机读面形状坏 ⇒ 该响亮失败，实得 err=%v", err)
	}
}

// ── ⑦ 退化路：机读面**不在** ⇒ 人面顶上（证「人面只作退化路」照样能用）──────────────────────

func TestGap20261010_25_HumanFallback(t *testing.T) {
	root := gap25Root(t, "human")
	names, err := gateFullStepNames(root)
	if err != nil {
		t.Fatalf("机读面不在时该走人面退化路，实得 err=%v", err)
	}
	if got := gap25Join(t, names); got != "人面步一,人面步二" {
		t.Errorf("机读面不在 ⇒ 该退化到人面（人面步一,人面步二），实得 %q", got)
	}
	if n2, _, ok := slgStepCmds(root); !ok || gap25Join(t, n2) != "人面步一,人面步二" {
		t.Errorf("slgStepCmds 退化路该读人面，实得 ok=%v %v", ok, n2)
	}
}
