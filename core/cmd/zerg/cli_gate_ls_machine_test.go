// cli_gate_ls_machine_test.go —— `zerg gate ls --json` 的**步表机读面优先**判据（缺口 `GAP-20261010-25` 本片）。
//
// 症状（一句话）：`gateLsJSON` 仍抠人面中文读数（「共 N 步」+ 三列）—— 闸脚本文案一改即不可机检。
// 正解（照先例 `preview_gate_steps.go` 的 `pgsParseListJSON`）：**机读面优先**（`precommit-gates.sh
// --list --json` 的单对象 `{"count":N,"steps":[…]}`），**人面只作退化路**；机读面在场而形状坏/自证不过
// ⇒ **响亮失败，不给结论**（✗ 不许静默退回人面把读数当绿）。
//
// 判据设计：合成门禁脚本让**人面**（`--list`）与**机读面**（`--list --json`）给出**不同**的步名
// （`人面步N` vs `机读步N`）⇒「读的是哪一面」一眼可判。
//
// 两态纪律：本件断言的是**改后**行为。⇒ 对**改前源码**（或把机读面那一支哑化）跑 ⇒ 必红
// （改前只抠人面，读回 `人面步N`）。落地（改后）⇒ 绿。
package main_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── 合成门禁脚本的三张面（人面 / 机读面 / 形状坏的机读面）─────────────────────────
const (
	gtmHumanFace = "── 步骤清单（scope 模式 名称）──\n" +
		"   go    empty  人面步一\n" +
		"   gates tri    人面步二\n" +
		"共 2 步\n"
	gtmMachineFace = `{"count":2,"steps":[{"scope":"go","mode":"empty","name":"机读步一"},` +
		`{"scope":"gates","mode":"tri","name":"机读步二"}]}` + "\n"
	// 形状坏：`count`=5 但 `steps` 只 1 条 ⇒ 机读面**在场**而计数自证不过。
	gtmBrokenFace = `{"count":5,"steps":[{"scope":"go","mode":"empty","name":"机读步一"}]}` + "\n"
)

// gtmRepo 造一棵合成仓（只放门禁脚本）；mode 决定机读面那一支：
//
//	"both"   —— 机读面齐（`--json` 出真机读面）
//	"broken" —— 机读面在场而形状坏（计数自证不过）
//	"human"  —— 脚本不认 `--json`（`--list --json` 只出人面）⇒ 机读面**不在**（走退化路）
func gtmRepo(t *testing.T, mode string) string {
	t.Helper()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "core", "internal", "version", "version.go"), "package version\n")
	dir := filepath.Join(root, "scripts", "gates")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("合成目录造不出来：%v", err)
	}
	human := filepath.Join(dir, ".gtm-human")
	mach := filepath.Join(dir, ".gtm-mach")
	if err := os.WriteFile(human, []byte(gtmHumanFace), 0o644); err != nil {
		t.Fatal(err)
	}
	body := gtmMachineFace
	switch mode {
	case "broken":
		body = gtmBrokenFace
	case "human":
		body = gtmHumanFace // `--list --json` 只出人面 ⇒ 机读面不在
	}
	if err := os.WriteFile(mach, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	script := "#!/usr/bin/env bash\nset -u\n" +
		"case \"${1:-}\" in\n" +
		"  --list)\n" +
		"    if [ \"${2:-}\" = \"--json\" ]; then cat \"" + mach + "\"; else cat \"" + human + "\"; fi ;;\n" +
		"  *) printf '✗ 未知参数: %s\\n' \"${1:-}\" >&2; exit 2 ;;\n" +
		"esac\n"
	if err := os.WriteFile(filepath.Join(dir, "precommit-gates.sh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

type gtmDoc struct {
	Total *int                     `json:"total"`
	Steps []map[string]interface{} `json:"steps"`
}

func gtmStepNames(steps []map[string]interface{}) []string {
	out := []string{}
	for _, r := range steps {
		s, _ := r["step"].(string)
		out = append(out, s)
	}
	return out
}

// ① 机读面优先：`gate ls --json` 该读机读面（机读步一,机读步二），✗ 不是人面（人面步一,人面步二）。
func TestGateLsJSON_MachineFaceFirst(t *testing.T) {
	root := gtmRepo(t, "both")
	rc, out, errb := runZergRepo(t, root, "gate", "ls", "--json")
	if rc != 0 {
		t.Fatalf("`gate ls --json` ⇒ 退 0，得到 %d · stderr=%s", rc, errb)
	}
	var doc gtmDoc
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("解不动：%v · %q", err, out)
	}
	got := gtmStepNames(doc.Steps)
	if strings.Join(got, ",") != "机读步一,机读步二" {
		t.Errorf("`gate ls --json` 该读**机读面**（机读步一,机读步二），实得 %v（读到人面 = 改前行为）", got)
	}
	if doc.Total == nil || *doc.Total != 2 {
		t.Errorf("total 该 = 2（机读面 `count`），实得 %v", doc.Total)
	}
}

// ② 形状坏：机读面在场而计数自证不过 ⇒ 响亮失败（rc≠0 · stdout 无结论 · stderr 点「形状坏」），✗ 不许退回人面当绿。
func TestGateLsJSON_BrokenMachineFaceLoud(t *testing.T) {
	root := gtmRepo(t, "broken")
	rc, out, errb := runZergRepo(t, root, "gate", "ls", "--json")
	if rc == 0 {
		t.Fatalf("机读面形状坏 ⇒ 该响亮 rc≠0，实得 rc=0 · out=%q", out)
	}
	if strings.Contains(out, "人面步") {
		t.Errorf("机读面形状坏 ⇒ ✗ 不许静默退回人面当绿（stdout 出现人面步名）：%q", out)
	}
	if !strings.Contains(errb, "形状坏") {
		t.Errorf("stderr 该点「形状坏」：%q", errb)
	}
}

// ③ 退化路：机读面**不在**（脚本不认 `--json`）⇒ 人面顶上（证「人面只作退化路」照样能用）。
func TestGateLsJSON_HumanFallback(t *testing.T) {
	root := gtmRepo(t, "human")
	rc, out, errb := runZergRepo(t, root, "gate", "ls", "--json")
	if rc != 0 {
		t.Fatalf("机读面不在 ⇒ 该走人面退化路退 0，得到 %d · stderr=%s", rc, errb)
	}
	var doc gtmDoc
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("解不动：%v · %q", err, out)
	}
	got := gtmStepNames(doc.Steps)
	if strings.Join(got, ",") != "人面步一,人面步二" {
		t.Errorf("机读面不在 ⇒ 该退化到人面（人面步一,人面步二），实得 %v", got)
	}
}
