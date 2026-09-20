// cli_gate_explain_test.go —— P0-4 `zerg gate explain` 的**真二进制**判据（缺口-命令面-20260921 §十一）。
//
// 用**合成门禁脚本**钉住四条：① 精确名 ⇒ 0，且 `出处` 指到脚本的**那一行**（`文件:行`）；
// ② 序号以脚本自己的 `--list` 为准（合成清单里是第 2 步）；③ 名字不逐字相同 ⇒ **2**（B-4 那
// 条实据的对面：不许子串匹配给一坨）；④ 同名两处 ⇒ **2**（歧义要报，不许猜一个）。
package main_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// appendToFile 往合成夹具的件尾部追加（本层的合成脚本先建后补两行 add_step）。
func appendToFile(t *testing.T, path, more string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o755)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(more); err != nil {
		t.Fatal(err)
	}
}

const syntheticGateList = `if [ "${1:-}" = "--list" ]; then
  echo "── 步骤清单（scope 模式 名称）──"
  echo "  go    empty  gofmt -l core"
  echo "  gates tri    合成步"
  echo "共 2 步"
  exit 0
fi
exit 0
`

func TestGateExplain_ExactNameOnly(t *testing.T) {
	bin := zergBinary(t)
	root := syntheticRepo(t, syntheticGateList)
	// 脚本里逐字声明的两步（`add_step` 行 —— 这才是真源）。
	appendToFile(t, filepath.Join(root, "scripts", "gates", "precommit-gates.sh"),
		"\nadd_step go \"gofmt -l core\" empty \"${REPO_ROOT}/core\" \"gofmt -l .\"\n"+
			"add_step gates \"合成步\" tri \"${REPO_ROOT}\" \"python3 scripts/gates/synthetic.py\"\n")
	mustWrite(t, filepath.Join(root, "scripts", "gates", "synthetic.py"),
		"#!/usr/bin/env python3\n# synthetic.py — 合成门脚本（判据自述第一行）\n# 第二行自述\nprint(1)\n")

	rc, out, errb := execCase(t, bin, root, "gate", "explain", "合成步",
		"--json", "scope,mode,source,log,exit,criterion,script_say")
	if rc != 0 {
		t.Fatalf("精确名 ⇒ 退 0，得到 %d · stderr=%s", rc, errb)
	}
	var env struct {
		Items []map[string]string `json:"items"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("--json 不是合法包封：%v · %q", err, out)
	}
	if len(env.Items) != 1 {
		t.Fatalf("要一行，得到 %d", len(env.Items))
	}
	row := env.Items[0]
	if row["scope"] != "gates" || row["mode"] != "tri" {
		t.Errorf("scope/mode 读错：%+v", row)
	}
	if !strings.HasPrefix(row["source"], "scripts/gates/precommit-gates.sh:") {
		t.Errorf("出处必须是 `文件:行`：%q", row["source"])
	}
	if !strings.Contains(row["log"], "序号 2") {
		t.Errorf("日志序号应当来自脚本自己的 --list（合成清单里是第 2 步）：%q", row["log"])
	}
	if !strings.Contains(row["exit"], "不给结论") {
		t.Errorf("退码口径要逐字给出脚本头部那句：%q", row["exit"])
	}
	if !strings.Contains(row["script_say"], "synthetic.py") {
		t.Errorf("判据要引被调脚本的自述：%q", row["script_say"])
	}

	// ③ 子串（不逐字）⇒ 2
	rc, _, _ = execCase(t, bin, root, "gate", "explain", "合成")
	if rc != 2 {
		t.Errorf("非逐字名 ⇒ 退 2（**不许**子串匹配给一坨），得到 %d", rc)
	}
	rc, _, _ = execCase(t, bin, root, "gate", "explain")
	if rc != 2 {
		t.Errorf("缺步名 ⇒ 退 2，得到 %d", rc)
	}
}

// ④ 同名两处 ⇒ 歧义 ⇒ 2（判据要一个答案，不是一坨）。
func TestGateExplain_AmbiguousNameIsUsageError(t *testing.T) {
	bin := zergBinary(t)
	root := syntheticRepo(t, syntheticGateList)
	appendToFile(t, filepath.Join(root, "scripts", "gates", "precommit-gates.sh"),
		"\nadd_step gates \"重名步\" tri \"${REPO_ROOT}\" \"true\"\n"+
			"add_step docs \"重名步\" rc \"${REPO_ROOT}\" \"true\"\n")
	rc, _, errb := execCase(t, bin, root, "gate", "explain", "重名步")
	if rc != 2 {
		t.Errorf("同名两处 ⇒ 退 2，得到 %d · stderr=%s", rc, errb)
	}
	if !strings.Contains(errb, "歧义") {
		t.Errorf("要明说「歧义」：%q", errb)
	}
}
