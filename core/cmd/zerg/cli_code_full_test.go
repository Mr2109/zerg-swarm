// cli_code_full_test.go —— `code find` / `code show` 的 `--full`（不截断）判据（缺口 `GAP-20260927-09`）。
//
// 病根（父代理现场实测）：`zerg code show gateway/fleet.yaml:115` 出到 `…mmproj: "~/zerg…O…`
// 就断了 —— 而这一格是**人面与机器面共用**的同一份 rows ⇒ 长行尾巴在**两面上一起消失** ✗，
// 取证只能退回手搓 `read_file`。判据钉住**两条腿**：
//
//	① 默认面**一个字节不动**（既有行为是契约的一部分 ⇒ 不许被这次修补加长）；
//	② `--full` ⇒ 同行**完整**（长行尾巴在 · 无 `…`）。
package main_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLICodeFullKeepsLongLine(t *testing.T) {
	bin := zergBinary(t)
	root := syntheticRepo(t, "true\n")
	long := strings.Repeat("x", 260) + "-TAIL"
	mustWrite(t, filepath.Join(root, "aaa", "long.txt"), "head\n"+long+"\n")
	_ = os.MkdirAll(filepath.Join(root, "aaa"), 0o755)

	rc, out, errb := execCase(t, bin, root, "code", "show", "aaa/long.txt:2", "--ctx", "0")
	if rc != 0 {
		t.Fatalf("默认档应 rc=0，实得 rc=%d（stderr=%s）", rc, errb)
	}
	if strings.Contains(out, "-TAIL") {
		t.Fatalf("默认档必须**保持既有截断行为**（本修补只加显式档），实得 stdout=%q", out)
	}
	if !strings.Contains(out, "…") {
		t.Fatalf("默认档应带截断标记 `…`（人面好看的既有行为），实得 stdout=%q", out)
	}

	rc2, out2, errb2 := execCase(t, bin, root, "code", "show", "aaa/long.txt:2", "--ctx", "0", "--full")
	if rc2 != 0 {
		t.Fatalf("--full 应 rc=0，实得 rc=%d（stderr=%s）", rc2, errb2)
	}
	if !strings.Contains(out2, "-TAIL") {
		t.Fatalf("--full 必须给出**完整**同行（尾巴不许丢），实得 stdout=%q", out2)
	}
	if strings.Contains(out2, "…") {
		t.Fatalf("--full 档不许带截断标记，实得 stdout=%q", out2)
	}
}
