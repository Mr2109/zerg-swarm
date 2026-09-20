// cli_gate_bench_test.go —— P0-8 `zerg gate bench` 的**真二进制**判据（缺口-命令面-20260921 §十一）。
//
// 判据用**合成门禁脚本**（不跑真门禁 —— 那要 18 秒且与批 A 记录耦合）：
//
//	① 一趟 rc=0 ⇒ 命令退 0，且 real_ms 是**真量到的正数**（不是写死的 0）；
//	② 脚本退 3 ⇒ 命令退 1（有 FAIL），且逐趟 rc 原样报出（**直通不翻译**）；
//	③ `--repeat` / 档位互斥 等用法错 ⇒ 2。
package main_test

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

func TestGateBench_MeasuresAndCarriesExitCode(t *testing.T) {
	bin := zergBinary(t)
	root := syntheticRepo(t, "sleep 0.05\nexit 0\n")
	rc, out, errb := execCase(t, bin, root, "gate", "bench", "--repeat", "2", "--json", "run,real_ms,rc,log")
	if rc != 0 {
		t.Fatalf("全绿 ⇒ 退 0，得到 %d · stderr=%s", rc, errb)
	}
	var env struct {
		Items []struct {
			Run    string `json:"run"`
			RealMS string `json:"real_ms"`
			RC     string `json:"rc"`
			Log    string `json:"log"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("--json 不是合法包封：%v · %q", err, out)
	}
	if len(env.Items) != 3 { // 两趟 + 一行中位/最差
		t.Fatalf("--repeat 2 ⇒ 逐趟 2 行 + 汇总 1 行，得到 %d 行", len(env.Items))
	}
	for _, it := range env.Items[:2] {
		ms, err := strconv.ParseFloat(it.RealMS, 64)
		if err != nil || ms < 40 { // 脚本自己 sleep 50ms ⇒ 量到的 real 必须大于它
			t.Errorf("real_ms 不是真量到的正数（脚本 sleep 50ms）：%+v", it)
		}
		if it.RC != "0" {
			t.Errorf("该趟 rc 应当是 0：%+v", it)
		}
		if it.Log == "" {
			t.Errorf("每趟要有自己的日志落点：%+v", it)
		}
	}

	// 合成脚本退 3 ⇒ 命令面退 1（**原样报出脚本的码**，不改写成「非 0 一律 1」）
	root2 := syntheticRepo(t, "exit 3\n")
	rc, out, errb = execCase(t, bin, root2, "gate", "bench", "--json", "run,rc")
	if rc != 1 {
		t.Errorf("脚本非 0 ⇒ 命令退 1，得到 %d · stderr=%s", rc, errb)
	}
	if !strings.Contains(out, `"rc":"3"`) {
		t.Errorf("逐趟 rc 应当**原样**是 3（直通不翻译）：%q", out)
	}
}

func TestGateBench_UsageErrors(t *testing.T) {
	bin := zergBinary(t)
	root := syntheticRepo(t, "exit 0\n")
	for _, argv := range [][]string{
		{"gate", "bench", "--repeat", "0"},
		{"gate", "bench", "--repeat", "abc"},
		{"gate", "bench", "--fast", "--scope", "go"},
		{"gate", "bench", "--nosuchflag-zz"},
	} {
		rc, _, _ := execCase(t, bin, root, argv...)
		if rc != 2 {
			t.Errorf("%v ⇒ 退 2，得到 %d", argv, rc)
		}
	}
}
