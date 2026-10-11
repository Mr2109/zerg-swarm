// gate_ls_gates_total_test.go —— `zerg gate ls --json gates_total` 的**白名单 + 牙**判据
// （缺口 `GAP-20261010-13` 本片）。
//
// 症状（一句话）：步表机读面（`precommit-gates.sh --list --json`）早有了顶层格 `gates_total`
// （前序 DEV-2400 落的），而 CLI 侧投影白名单写死 `total,scope,mode,step` ⇒
// `zerg gate ls --json gates_total` 退 2「未知字段」（= 加格到机读面 ≠ CLI 打得着）。
//
// 正解（照同族体例 · ✗ 不自造）：
//
//	① 白名单：`gates_total` 走**可选字段层**（`optFields`，与 `name` 同一体例）⇒ 点得出来，
//	   且**不进**冻结 `fields:`（帮助/导出面与命令页一行不动）。
//	② 牙：机读面自证里加 `gates_total == len(steps)`（照 `pgsParseListJSON` 既有
//	   `count == len(steps)` 的「形状坏 ⇒ 响亮 rc≠0」正解体例）—— 不一致 ⇒ 响亮失败，✗ 不静默。
package main_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── 合成门禁脚本三张面（人面 / 机读面-一致 / 机读面-gates_total 不一致）──────────
const (
	glgtHumanFace = "── 步骤清单（scope 模式 名称）──\n" +
		"   go    empty  人面步一\n" +
		"   gates tri    人面步二\n" +
		"共 2 步\n"
	// 机读面一致：count=2 · gates_total=2 · steps=2 条
	glgtMachConsistent = `{"count":2,"gates_total":2,"steps":[` +
		`{"scope":"go","mode":"empty","name":"机读步一"},` +
		`{"scope":"gates","mode":"tri","name":"机读步二"}]}` + "\n"
	// 机读面**在场**而 `gates_total` 与 steps 长度不一致（count 仍自洽）⇒ 牙该响。
	glgtMachBadGatesTotal = `{"count":2,"gates_total":999,"steps":[` +
		`{"scope":"go","mode":"empty","name":"机读步一"},` +
		`{"scope":"gates","mode":"tri","name":"机读步二"}]}` + "\n"
)

// glgtRepo 造一棵合成仓（只放门禁脚本）；machBody = `--list --json` 出口那段（给 glgtHumanFace
// ⇒ 脚本不认 --json ⇒ 机读面不在 ⇒ 走人面退化路）。
func glgtRepo(t *testing.T, machBody string) string {
	t.Helper()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "core", "internal", "version", "version.go"), "package version\n")
	dir := filepath.Join(root, "scripts", "gates")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("合成目录造不出来：%v", err)
	}
	human := filepath.Join(dir, ".glgt-human")
	mach := filepath.Join(dir, ".glgt-mach")
	if err := os.WriteFile(human, []byte(glgtHumanFace), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mach, []byte(machBody), 0o644); err != nil {
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

// ① 白名单：`--json gates_total` ⇒ 退 0，只出顶层那一格 `{"gates_total":N}`（机读面一致）。
func TestGateLsJSON_GatesTotalProjects(t *testing.T) {
	root := glgtRepo(t, glgtMachConsistent)
	rc, out, errb := runZergRepo(t, root, "gate", "ls", "--json", "gates_total")
	if rc != 0 {
		t.Fatalf("`gate ls --json gates_total` ⇒ 退 0（可选字段层收下），得到 %d · stderr=%s", rc, errb)
	}
	var doc map[string]int
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("`--json gates_total` 要是一个 {\"gates_total\":N} 对象：%v · %q", err, out)
	}
	if doc["gates_total"] != 2 {
		t.Errorf("gates_total = %d（机读面 2 步）· %q", doc["gates_total"], out)
	}
	if strings.Contains(out, "steps") {
		t.Errorf("只点名 gates_total ⇒ 不许出 steps：%q", out)
	}
}

// ② 牙（负控真红）：机读面在场而 `gates_total` 与 steps 长度**不一致** ⇒ 该响亮 rc≠0（✗ 不静默退回人面）。
func TestGateLsJSON_GatesTotalBrokenLoud(t *testing.T) {
	root := glgtRepo(t, glgtMachBadGatesTotal)
	rc, out, errb := runZergRepo(t, root, "gate", "ls", "--json", "gates_total")
	if rc == 0 {
		t.Fatalf("gates_total 与 steps 不一致 ⇒ 该响亮 rc≠0，实得 rc=0 · out=%q", out)
	}
	if strings.Contains(out, "人面步") {
		t.Errorf("✗ 不许静默退回人面当绿（stdout 出现人面步名）：%q", out)
	}
	if !strings.Contains(errb, "gates_total") {
		t.Errorf("stderr 该点 gates_total（自证不过）：%q", errb)
	}
}

// ③ 负控：`gates_total` 是可选项，不进冻结字段表 —— 「合法字段:」那行逐字不变、且不出现 gates_total。
func TestGateLsJSON_GatesTotalNotInFrozenFieldList(t *testing.T) {
	root := glgtRepo(t, glgtMachConsistent)
	rc, _, errb := runZergRepo(t, root, "gate", "ls", "--json", "totally-bogus")
	if rc != 2 {
		t.Fatalf("`--json <闭集外>` ⇒ 退 2，得到 %d · stderr=%s", rc, errb)
	}
	if !strings.Contains(errb, "合法字段: total,scope,mode,step") {
		t.Errorf("冻结字段表须逐字不变：%q", errb)
	}
	if strings.Contains(errb, "gates_total") {
		t.Errorf("`gates_total` 属可选层，不许进冻结字段表：%q", errb)
	}
}

// ④ 退化路：机读面**不在**（脚本不认 --json）⇒ 人面顶上，`gates_total` 仍可读（= total，退 0）。
func TestGateLsJSON_GatesTotalHumanFallback(t *testing.T) {
	root := glgtRepo(t, glgtHumanFace)
	rc, out, errb := runZergRepo(t, root, "gate", "ls", "--json", "gates_total")
	if rc != 0 {
		t.Fatalf("机读面不在 ⇒ 该走人面退化路退 0，得到 %d · stderr=%s", rc, errb)
	}
	var doc map[string]int
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("解不动：%v · %q", err, out)
	}
	if doc["gates_total"] != 2 {
		t.Errorf("人面退化路 gates_total = %d（共 2 步）· %q", doc["gates_total"], out)
	}
}
