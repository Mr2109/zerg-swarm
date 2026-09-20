// cli_gate_matrix_test.go —— P0-5 `zerg gate matrix` 的**真二进制**判据（缺口-命令面-20260921 §十一）。
//
// 三面钉住：① 格数与**真源文件**里的格数**逐字一致**（不自己数、不猜）；② `--out` 导出的件
// 行数 = 格数 + 表头（导不出/少一行就是白导）；③ 真源不在 ⇒ 退 8（**不许把读不到当空矩阵**）。
package main_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGateMatrix_CountsAgainstSourceOfTruth(t *testing.T) {
	bin := zergBinary(t)
	root := filepath.Dir(filepath.Dir(bin)) // bin/zerg 的上两级 = 仓根
	rel := filepath.Join(root, "core", "cmd", "zerg", "testdata", "cli-matrix.json")
	b, err := os.ReadFile(rel)
	if err != nil {
		t.Skipf("真源不在（本层不合成它）：%v", err)
	}
	var mf struct {
		Cases []struct {
			ID     string `json:"id"`
			WantRC int    `json:"want_rc"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(b, &mf); err != nil {
		t.Fatal(err)
	}
	want := len(mf.Cases)

	rc, out, errb := execCase(t, bin, root, "gate", "matrix", "--json", "command,case,want_rc")
	if rc != 0 {
		t.Fatalf("矩阵读得到 ⇒ 退 0，得到 %d · stderr=%s", rc, errb)
	}
	var env struct {
		Items []struct {
			Command string `json:"command"`
			Case    string `json:"case"`
			WantRC  string `json:"want_rc"`
		} `json:"items"`
		Meta struct{ Count int } `json:"meta"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("--json 不是合法包封：%v", err)
	}
	if env.Meta.Count != want {
		t.Errorf("格数对不上真源：命令=%d 真源=%d", env.Meta.Count, want)
	}
	for _, it := range env.Items {
		if it.Command == "" || it.Case == "" || it.WantRC == "" {
			t.Fatalf("有一格三列没给全（逐格可读 = 补格的前提）：%+v", it)
		}
		if it.WantRC == "0" {
			t.Errorf("矩阵里不该有 expect-rc=0 的格（T2：must-fail 不许被稀释）：%+v", it)
		}
	}

	outFile := filepath.Join(t.TempDir(), "matrix.tsv")
	rc, _, errb = execCase(t, bin, root, "gate", "matrix", "--out", outFile)
	if rc != 0 {
		t.Fatalf("导出 ⇒ 退 0，得到 %d · stderr=%s", rc, errb)
	}
	got, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("导出件不在：%v", err)
	}
	lines := strings.Count(strings.TrimRight(string(got), "\n"), "\n") + 1
	if lines != want+2 { // ① 注释头 ② 列名 ③ 逐格
		t.Errorf("导出件行数不对：得到 %d 行（want %d = 注释头 1 + 列名 1 + 格 %d）", lines, want+2, want)
	}
}

// TestGateMatrix_UnreadableIsBlocked —— **成对负控**：真源缺席 ⇒ 退 8，不许当「零格」过去。
func TestGateMatrix_UnreadableIsBlocked(t *testing.T) {
	bin := zergBinary(t)
	root := syntheticRepo(t, "exit 0\n") // 合成仓根里没有 testdata/cli-matrix.json
	rc, out, _ := execCase(t, bin, root, "gate", "matrix")
	if rc != 8 {
		t.Errorf("矩阵读不到 ⇒ 退 8（不是 0、也不是空矩阵），得到 %d · stdout=%q", rc, out)
	}
	rc, _, _ = execCase(t, bin, root, "gate", "matrix", "--nosuchflag-zz")
	if rc != 2 {
		t.Errorf("未知旗标 ⇒ 退 2，得到 %d", rc)
	}
}
