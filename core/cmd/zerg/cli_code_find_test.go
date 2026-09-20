// cli_code_find_test.go —— P0-1 `zerg code find` 的**真二进制**判据（缺口-命令面-20260921 §十一）。
//
// 为什么在 `main_test` 这一层（§九 M17「三层测试落点」的第三层）：退码是 `os.Exit` 之后的终值，
// 而且本命令的输入面是**文件树** —— 合成一个临时仓根比动真仓干净。
package main_test

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// TestCodeFind_HitZeroHitAndUsage —— 三档退码成对钉住：有命中 0 · 零命中 1 · 用法错 2。
func TestCodeFind_HitZeroHitAndUsage(t *testing.T) {
	bin := zergBinary(t)
	root := syntheticRepo(t, "exit 0\n")
	mustWrite(t, filepath.Join(root, "core", "internal", "version", "hit.go"),
		"package version\n\nfunc targetSymbol() {}\n")
	mustWrite(t, filepath.Join(root, "README.md"), "targetSymbol 也在这里\n")
	mustWrite(t, filepath.Join(root, "bin", "skip.go"), "package x\n// targetSymbol 不该被扫到\n")

	rc, out, errb := execCase(t, bin, root, "code", "find", "func targetSymbol")
	if rc != 0 {
		t.Fatalf("有命中 ⇒ 退 0，得到 %d · stderr=%s", rc, errb)
	}
	if !strings.Contains(out, "core/internal/version/hit.go") {
		t.Errorf("命中行没出来：%q", out)
	}
	if strings.Contains(out, "bin/skip.go") {
		t.Errorf("排除表里的目录（bin）不该进扫描面：%q", out)
	}

	rc, _, _ = execCase(t, bin, root, "code", "find", "zzz_never_there_zzz")
	if rc != 1 {
		t.Errorf("零命中 ⇒ 退 1（不是错，是「没有」），得到 %d", rc)
	}

	rc, _, errb = execCase(t, bin, root, "code", "find", "a(")
	if rc != 2 {
		t.Errorf("正则坏 ⇒ 退 2，得到 %d · stderr=%s", rc, errb)
	}
	rc, _, _ = execCase(t, bin, root, "code", "find")
	if rc != 2 {
		t.Errorf("缺正则 ⇒ 退 2，得到 %d", rc)
	}
}

// TestCodeFind_GlobAndPathNarrowing —— 两枚收窄旗标真能被解析、且真的收窄（K14 那条教训）。
func TestCodeFind_GlobAndPathNarrowing(t *testing.T) {
	bin := zergBinary(t)
	root := syntheticRepo(t, "exit 0\n")
	mustWrite(t, filepath.Join(root, "aaa", "one.go"), "package aaa\n// needle\n")
	mustWrite(t, filepath.Join(root, "aaa", "one.txt"), "needle\n")
	mustWrite(t, filepath.Join(root, "bbb", "two.go"), "package bbb\n// needle\n")

	rc, out, errb := execCase(t, bin, root, "code", "find", "needle", "--glob", "*.go", "--json", "path,line")
	if rc != 0 {
		t.Fatalf("收窄后仍有命中 ⇒ 退 0，得到 %d · stderr=%s", rc, errb)
	}
	var env struct {
		Items []struct {
			Path string `json:"path"`
			Line string `json:"line"`
		} `json:"items"`
		Meta struct{ Count int } `json:"meta"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("--json 不是合法包封：%v · %q", err, out)
	}
	if env.Meta.Count != 2 {
		t.Errorf("--glob '*.go' 应当只剩两件 ⇒ count=2，得到 %d（%+v）", env.Meta.Count, env.Items)
	}
	for _, it := range env.Items {
		if strings.Contains(it.Path, "one.txt") {
			t.Errorf("--glob '*.go' 没把 .txt 挡掉：%+v", it)
		}
	}

	rc, out, _ = execCase(t, bin, root, "code", "find", "needle", "--path", "aaa", "--json", "path")
	if rc != 0 {
		t.Fatalf("--path aaa ⇒ 退 0，得到 %d", rc)
	}
	if strings.Contains(out, "bbb/two.go") || !strings.Contains(out, "aaa/one.go") {
		t.Errorf("--path 没把面收窄到 aaa/：%q", out)
	}

	rc, _, _ = execCase(t, bin, root, "code", "find", "needle", "--nosuchflag-zz")
	if rc != 2 {
		t.Errorf("未知旗标 ⇒ 退 2，得到 %d", rc)
	}
}
