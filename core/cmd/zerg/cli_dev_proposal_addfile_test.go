// cli_dev_proposal_addfile_test.go —— `zerg dev proposal revise --add-file <路径>` 的判据机检
// （main.go 开窗第 5 条 · 外部测试包 `package main_test`）。
//
// 病灶（缺口语境）：本会话多次撞「提案的 `files[]` 漏了一个件」（实例：DEV-2399.files[] 缺那条要动的件、
// DEV-2405.files[] 缺 `gate ls gates_total` 那条）—— 当时处置是「补 `files[]`」，却**没有一个正门**能改
// **已落提案**的文件清单（`revise` 此前只能改 `--criterion`/`--criterion-new`）。本枚给 `revise` 加
// `--add-file <路径>`（取值旗标 · 可重复）：把路径**加进**既有提案的 `files[]`。
//
// 三态（逐条断言）：
//
//	① 给**新路径** ⇒ `files[]` **含**该路径（且给「已在数组里」的路径同批 ⇒ 只加一次）。
//	② 给**已在 `files[]` 里的**路径 ⇒ **幂等**（不重复加 · 件数不变 · rc=0）。
//	③ **不给** `--add-file` ⇒ `revise` 原行为**不变**（改 `--criterion` 照旧 · `files[]` 一字不动）；
//	   且一格都不给 ⇒ 仍按原档退 2。
//
// 零副作用：`--dry-run` 出计划件、盘上件一字不动。
package main_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/contract"
)

// readProposalFilesAddFile 读提案件里的 `files[]`（「补件」取值面）。
func readProposalFilesAddFile(t *testing.T, dir, id string) []string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, id+".json"))
	if err != nil {
		t.Fatalf("读落件 %s 失败：%v", id, err)
	}
	var rec map[string]any
	if err := json.Unmarshal(b, &rec); err != nil {
		t.Fatalf("落件 JSON 解不开：%v", err)
	}
	raw, _ := rec["files"].([]any)
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// hasStrAddFile / countStrAddFile —— 小助手（按值判在不在 / 数几次）。
func hasStrAddFile(ss []string, v string) bool {
	for _, x := range ss {
		if x == v {
			return true
		}
	}
	return false
}

func countStrAddFile(ss []string, v string) int {
	n := 0
	for _, x := range ss {
		if x == v {
			n++
		}
	}
	return n
}

// TestDevProposalReviseAddFile —— `revise --add-file` 三态 + 幂等 + dry-run 零写。
func TestDevProposalReviseAddFile(t *testing.T) {
	proposalEvidenceDir(t)
	ids, err := contract.DevTargets()
	if err != nil || len(ids) == 0 {
		t.Fatalf("回指清单读不出来：%v", err)
	}
	dir := t.TempDir()
	t.Setenv("ZERG_PROPOSAL_DIR", dir)

	// 先提一件（`files[]` 含两枚：main.go 与 family_dev.go）。
	const mainGo = "core/cmd/zerg/main.go"
	const famDev = "core/cmd/zerg/family_dev.go"
	const newFile = "core/cmd/zerg/cli_dev_proposal_addfile_test.go"
	rc0, out0, errb0 := runProposalNew(t, "--title", "补件", "--target", "待办:"+ids[0], "--goal", "g",
		"--evidence", proposalEvidenceInLedger, "--rollback", "r", "--criterion", testCriterion,
		"--file", mainGo, "--file", famDev)
	if rc0 != 0 {
		t.Fatalf("正控件提不出来：rc=%d · stderr=%s", rc0, errb0)
	}
	id := strings.TrimSpace(out0)
	if id == "" {
		t.Fatalf("成功时 stdout 要给提案 id")
	}
	if got := readProposalFilesAddFile(t, dir, id); len(got) != 2 || !hasStrAddFile(got, mainGo) || !hasStrAddFile(got, famDev) {
		t.Fatalf("初始 files[] 形态不对：%v", got)
	}

	// ---- 态①：给「已在数组里」(main.go) + 「新路径」(newFile) 两枚 ⇒ rc=0；
	//          files[] 含新路径；已在的那枚**只加一次**（幂等 · 不重复加）。----
	rc1, _, err1 := runProposalRevise(t, id, "--add-file", mainGo, "--add-file", newFile, "--yes")
	if rc1 != 0 {
		t.Fatalf("态① `--add-file` 应 rc=0，实得 rc=%d · stderr=%s", rc1, err1)
	}
	got1 := readProposalFilesAddFile(t, dir, id)
	if !hasStrAddFile(got1, newFile) {
		t.Errorf("态①：新路径 %q 未进 files[]：%v", newFile, got1)
	}
	if !hasStrAddFile(got1, mainGo) {
		t.Errorf("态①：已在的 %q 反而丢了：%v", mainGo, got1)
	}
	if c := countStrAddFile(got1, mainGo); c != 1 {
		t.Errorf("态①幂等：已在的 %q 出现 %d 次（要 1 次）", mainGo, c)
	}
	// 其余字段不动（以 id 为准）。
	var rec1 map[string]any
	if b, e := os.ReadFile(filepath.Join(dir, id+".json")); e == nil {
		_ = json.Unmarshal(b, &rec1)
	}
	if rec1["id"] != id || rec1["target"] != ids[0] {
		t.Errorf("态①动了别的字段：id=%v target=%v", rec1["id"], rec1["target"])
	}

	// ---- 态②：再给**已在**的 newFile ⇒ rc=0 · 幂等（件数不变 · 只 1 次）。----
	rc2, _, err2 := runProposalRevise(t, id, "--add-file", newFile, "--yes")
	if rc2 != 0 {
		t.Errorf("态② 幂等应 rc=0，实得 rc=%d · stderr=%s", rc2, err2)
	}
	got2 := readProposalFilesAddFile(t, dir, id)
	if len(got2) != len(got1) {
		t.Errorf("态②幂等被破坏：件数 %d → %d：%v", len(got1), len(got2), got2)
	}
	if c := countStrAddFile(got2, newFile); c != 1 {
		t.Errorf("态②幂等：%q 出现 %d 次（要 1 次）", newFile, c)
	}

	// ---- 态③：**不给** `--add-file` ⇒ revise 原行为不变（改 --criterion 照旧 · files[] 一字不动）。----
	const newCrit = "zerg gate run --fast"
	rc3, _, err3 := runProposalRevise(t, id, "--criterion", newCrit, "--yes")
	if rc3 != 0 {
		t.Errorf("态③ 改判据应 rc=0，实得 rc=%d · stderr=%s", rc3, err3)
	}
	if got := readProposalCriterion(t, dir, id); got != newCrit {
		t.Errorf("态③ 判据未逐字落件：%q（要 %q）", got, newCrit)
	}
	if got3 := readProposalFilesAddFile(t, dir, id); !reflect.DeepEqual(got3, got2) {
		t.Errorf("态③ 不给 --add-file 却动了 files[]：%v → %v", got2, got3)
	}

	// ---- 态③b：一格都不给 ⇒ 仍按原档退 2（点名「没给」）。----
	rc4, _, err4 := runProposalRevise(t, id, "--yes")
	if rc4 != 2 || !strings.Contains(err4, "没给") {
		t.Errorf("态③b 一格不给 ⇒ rc=%d（要 2）、stderr 要点名「没给」：%q", rc4, err4)
	}

	// ---- `--dry-run`：出计划件（示出补的件）· 盘上件一字不动（零写）。----
	const dryFile = "core/cmd/zerg/zzz_dryrun_only.go"
	before := readProposalFilesAddFile(t, dir, id)
	rc5, out5, _ := runProposalRevise(t, id, "--add-file", dryFile, "--dry-run")
	if rc5 != 0 {
		t.Errorf("dry-run 应 rc=0，实得 rc=%d", rc5)
	}
	after := readProposalFilesAddFile(t, dir, id)
	if !reflect.DeepEqual(before, after) {
		t.Errorf("dry-run 却写了盘：%v → %v", before, after)
	}
	if !strings.Contains(out5, dryFile) {
		t.Errorf("dry-run 计划件应示出要补的件 %q：%s", dryFile, out5)
	}
}
