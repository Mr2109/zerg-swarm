// dev_proposal_deep_test.go —— ★ GAP-20260927-277：`dev proposal check --deep`（判据**真跑**核真值 · 报 rc 与命中数）。
//
// 病灶（缺口逐字）：「判据面只判『可机检』不核真值：173 件全报可跑，真跑 16 件不命中」。
// 本档把每一条命令面判据**真跑一次**、把退码收回来：命中 = 0；不命中 = 非 0 / 跑不动 / 超时。
//
// 落点：`package main`（内部测试包）—— 要改写 `proposalDeepExe`（未导出）把真跑的子进程指到现成制品。
// 两态（同一夹具，只差 `--deep`）：不深档 ⇒ 一行不变、rc 由现有规则定；深档 ⇒ 多两列 + 命中数 + 不命中即 2。
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// deepFixture —— 落一件夹具提案（只带 check 读得到的字段）并返回夹具目录。
func deepFixture(t *testing.T, dir, id, criterion string) {
	t.Helper()
	rec := map[string]any{
		"id": id, "title": "夹具", "target": "D3b", "goal": "g",
		"evidence": []string{"夹具证据"}, "rollback_ref": "r",
		"criterion": criterion, "files": []string{"core/cmd/zerg/family_dev.go"}, "state": "未决",
	}
	b, _ := json.Marshal(rec)
	if err := os.WriteFile(filepath.Join(dir, id+".json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

// deepUseBuiltExe —— 把「真跑判据」的子进程二进制指到现成 `bin/zerg`（测试进程的 os.Executable()
// 是测试二进制、裸 `zerg` 解析不到真制品）⇒ 没有 bin/zerg 就跳过（本档依赖真制品才有意义）。
func deepUseBuiltExe(t *testing.T) {
	t.Helper()
	root := repoRoot()
	if root == "" {
		t.Fatal("仓根解析不到（repoRoot 空）")
	}
	bin := filepath.Join(root, "bin", "zerg")
	if _, err := os.Stat(bin); err != nil {
		t.Skipf("没有 %s（本档真跑判据需现成制品）", bin)
	}
	oldExe := proposalDeepExe
	proposalDeepExe = func() (string, error) { return bin, nil }
	t.Cleanup(func() { proposalDeepExe = oldExe })
}

// TestDevProposalCheckDeepRunsCriteria —— 正控：合规提案 `--deep` 报 rc=0 且命中数逐字相符；
// 负控「普通档行为逐字不变」：同夹具不带 `--deep` 仍 rc=0、一个 deep 字样都不出现。
func TestDevProposalCheckDeepRunsCriteria(t *testing.T) {
	deepUseBuiltExe(t)

	dir := t.TempDir()
	t.Setenv("ZERG_PROPOSAL_DIR", dir)
	deepFixture(t, dir, "DEV-9001", "zerg version") // 真命中（rc=0）

	var out, errb strings.Builder

	// ① 正控：`--deep` ⇒ rc=0，命中数逐字相符，表里多 deep 两列。
	if rc := RunForTest([]string{"dev", "proposal", "check", "--deep"}, &out, &errb); rc != 0 {
		t.Fatalf("判据真命中 ⇒ --deep rc=%d（要 0）· stderr=%s", rc, errb.String())
	}
	if !strings.Contains(errb.String(), "真跑 1 件判据 · 1 命中 · 0 不命中") {
		t.Errorf("--deep 未报命中数（要「真跑 1 件判据 · 1 命中 · 0 不命中」）：%q", errb.String())
	}
	if !strings.HasPrefix(out.String(), "id\ttarget\tstate\tcriterion_state\tcriterion\tfiles\twhy\tdeep_rc\tdeep") {
		t.Errorf("--deep 表头缺 deep 两列：%q", out.String())
	}

	// ② 负控：普通档（不带 --deep）逐字不变 —— rc=0、无 deep 字样、无命中数行。
	out.Reset()
	errb.Reset()
	if rc := RunForTest([]string{"dev", "proposal", "check"}, &out, &errb); rc != 0 {
		t.Fatalf("普通档 ⇒ rc=%d（要 0）· stderr=%s", rc, errb.String())
	}
	if strings.Contains(errb.String(), "真跑") || strings.Contains(out.String(), "deep") {
		t.Errorf("普通档被本枚污染（不该出现 deep 字样）：stdout=%q stderr=%q", out.String(), errb.String())
	}
	if !strings.Contains(errb.String(), "全部**可机检**") {
		t.Errorf("普通档未印原成功行：%q", errb.String())
	}
}

// TestDevProposalCheckDeepCatchesFalseRunnable —— 负控：一条**命令能解析、真跑却拿不到绿**的判据
// （缺口的「假可跑」形态）⇒ 普通档误报 rc=0；`--deep` 必须 rc≠0 且逐字点名不命中数。
func TestDevProposalCheckDeepCatchesFalseRunnable(t *testing.T) {
	deepUseBuiltExe(t)

	dir := t.TempDir()
	t.Setenv("ZERG_PROPOSAL_DIR", dir)
	// `version` 在命令树里 ⇒ criterionRunnable 判「可跑」；但真跑 `--zzz-bogus` 拿不到绿（rc=2）。
	deepFixture(t, dir, "DEV-9002", "zerg version --zzz-bogus")

	var out, errb strings.Builder
	// 普通档**误报绿** —— 这正是缺口「只判可机检不核真值」。
	if rc := RunForTest([]string{"dev", "proposal", "check"}, &out, &errb); rc != 0 {
		t.Fatalf("普通档对假可跑件应误报 rc=0（缺口形态），实得 rc=%d · stderr=%s", rc, errb.String())
	}
	// 深档必须**真红**（不给结论）且逐字报命中数。
	out.Reset()
	errb.Reset()
	if rc := RunForTest([]string{"dev", "proposal", "check", "--deep"}, &out, &errb); rc == 0 {
		t.Fatalf("--deep 对真跑不命中件应 rc≠0（不给结论），实得 rc=0 · stdout=%q stderr=%q", out.String(), errb.String())
	}
	if !strings.Contains(errb.String(), "真跑 1 件判据 · 0 命中 · 1 不命中") {
		t.Errorf("--deep 未逐字报命中数（要「真跑 1 件判据 · 0 命中 · 1 不命中」）：%q", errb.String())
	}
	if !strings.Contains(errb.String(), "真跑不命中") {
		t.Errorf("--deep 未点名「真跑不命中」：%q", errb.String())
	}
}
