// cli_dev_test_scoped_test.go —— P0-3 `zerg dev test --pkg/--run` 的**真二进制**判据
// （缺口-命令面-20260921 §十一）。
//
// 用**合成 Go 模块**钉住红绿两头（不碰真仓的测试集，也不依赖本机当前谁在红）：
//
//	① 只跑命中的那一个测 ⇒ 0；② 命中一个必红的测 ⇒ **1**（且日志原文里有那句失败原因）；
//	③ 用法错三态 ⇒ 2（缺 --pkg · 前缀不在闭集 · 正则坏 · 与 --candidate 互斥）。
package main_test

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// synthGoModule 在合成仓里建一个最小 Go 模块（core/ 是闭集里的前缀之一）。
func synthGoModule(t *testing.T, root string) {
	t.Helper()
	mustWrite(t, filepath.Join(root, "core", "go.mod"), "module example.com/synth\n\ngo 1.21\n")
	mustWrite(t, filepath.Join(root, "core", "pkg", "foo.go"), "package pkg\n\nfunc Add(a, b int) int { return a + b }\n")
	mustWrite(t, filepath.Join(root, "core", "pkg", "foo_test.go"),
		"package pkg\n\nimport \"testing\"\n\nfunc TestOK(t *testing.T) { if Add(1, 2) != 3 { t.Fatal(\"加错了\") } }\n\nfunc TestBad(t *testing.T) { t.Fatal(\"故意红：boom-marker\") }\n")
}

func TestDevTestScoped_GreenAndRed(t *testing.T) {
	bin := zergBinary(t)
	root := syntheticRepo(t, "exit 0\n")
	synthGoModule(t, root)

	rc, out, errb := execCase(t, bin, root, "dev", "test", "--pkg", "core/pkg", "--run", "TestOK", "--json", "step,verdict,rc,log_path")
	if rc != 0 {
		t.Fatalf("绿的那个测 ⇒ 退 0，得到 %d · stderr=%s", rc, errb)
	}
	var env struct {
		Items []struct {
			Step    string `json:"step"`
			Verdict string `json:"verdict"`
			RC      string `json:"rc"`
			LogPath string `json:"log_path"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("--json 不是合法包封：%v · %q", err, out)
	}
	if len(env.Items) != 2 { // 一条步骤 + 一条合计
		t.Fatalf("要「逐条 + 合计」两行，得到 %d", len(env.Items))
	}
	if env.Items[0].Verdict != "PASS" || env.Items[0].RC != "0" {
		t.Errorf("绿的步应当是 PASS/0：%+v", env.Items[0])
	}
	if !strings.Contains(env.Items[0].Step, "go test ./pkg -run TestOK -count=1") {
		t.Errorf("step 要逐字给出真跑的命令串（可复跑）：%q", env.Items[0].Step)
	}
	if env.Items[0].LogPath == "" {
		t.Errorf("日志路径不许空（判据要能回读）：%+v", env.Items[0])
	}
	if !strings.Contains(env.Items[1].Step, "合计") {
		t.Errorf("第二行要是合计：%+v", env.Items[1])
	}

	// ② 红：命中一个必红的测 ⇒ 1，且日志原文里有失败原因（判据要原文，不要「失败了」三个字）
	rc, _, errb = execCase(t, bin, root, "dev", "test", "--pkg", "core/pkg", "--run", "TestBad")
	if rc != 1 {
		t.Errorf("必红的测 ⇒ 退 1，得到 %d · stderr=%s", rc, errb)
	}
	if !strings.Contains(errb, "boom-marker") {
		t.Errorf("红的时候要把日志原文贴出来（找不到失败原因原句）：%q", errb)
	}
}

func TestDevTestScoped_UsageErrors(t *testing.T) {
	bin := zergBinary(t)
	root := syntheticRepo(t, "exit 0\n")
	synthGoModule(t, root)
	for _, argv := range [][]string{
		{"dev", "test", "--run", "TestOK"},                              // 缺 --pkg
		{"dev", "test", "--pkg", "nope/whatever", "--run", "x"},         // 前缀不在闭集
		{"dev", "test", "--pkg", "core/pkg", "--run", "a("},             // 正则坏
		{"dev", "test", "--pkg", "core/pkg", "--candidate", "DEV-0001"}, // 两种问法互斥
	} {
		rc, _, _ := execCase(t, bin, root, argv...)
		if rc != 2 {
			t.Errorf("%v ⇒ 退 2，得到 %d", argv, rc)
		}
	}
}
