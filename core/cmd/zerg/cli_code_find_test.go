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

// TestCodeFind_AbsolutePathIsExplicitRoot —— `W-03` 成对判据（2026-09-24 批七 · 序88）：
// ① **正控**：`--path` 给**绝对路径**的目录 ⇒ rc=0（旧写法一律与仓根 `Join` ⇒ 拼出来的路径
// 不存在 ⇒ 报「指的不是目录」的**假阴**）；
// ② **负控**：`--path` 给**既不是件也不是目录**的路径（这里用不存在的路径）⇒ rc=2
// （★ 2026-09-27 口径变更：`--path` 指**常规件**由「退 2」改判为**能扫到** ——
// 成对三控见 `TestCodeFind_PathAcceptsRegularFile`；★ 新文案仍含旧子串 ⇒ 分辨新老行为靠 rc，不靠文案）。
func TestCodeFind_AbsolutePathIsExplicitRoot(t *testing.T) {
	bin := zergBinary(t)
	root := syntheticRepo(t, "exit 0\n")
	mustWrite(t, filepath.Join(root, "aaa", "one.go"), "package aaa\n// needle\n")

	// ① 正控：绝对路径的目录（仓外也一样 —— 「显式给一根」）。
	abs := filepath.Join(root, "aaa")
	rc, out, errb := execCase(t, bin, root, "code", "find", "needle", "--path", abs, "--json", "path")
	if rc != 0 {
		t.Fatalf("--path <绝对路径目录> ⇒ 退 0，得到 %d · stderr=%s", rc, errb)
	}
	if !strings.Contains(out, "aaa/one.go") {
		t.Errorf("绝对根那一跑没扫到 aaa/one.go：%q", out)
	}

	// ② 负控：既不是件也不是目录（绝对路径指向不存在的路径）⇒ 仍 2（不许静默扫成零命中）。
	// ★ 口径变更（2026-09-27）：`--path` 指**常规件**已从「退 2」改判为**能扫到** ——
	// 成对三控见 `TestCodeFind_PathAcceptsRegularFile`（目录 ⇒ 扫到 · 常规件 ⇒ 扫到 · 不存在 ⇒ 2）。
	rc, _, errb = execCase(t, bin, root, "code", "find", "needle", "--path", filepath.Join(root, "zz-nope"))
	if rc != 2 {
		t.Errorf("--path 指不存在的路径 ⇒ 退 2，得到 %d · stderr=%s", rc, errb)
	}
	if strings.TrimSpace(errb) == "" {
		t.Errorf("--path 指不存在的路径退了 2 却没给理由（stderr 空）")
	}
}

// TestCodeFind_PathAcceptsRegularFile —— ★ 口径变更（2026-09-27 跨件单 · 命令面 `family_code.go:92`）：
// `--path` 收**目录**与**常规件**（同族先例：`core ps --path <声明件>` · `config reload --path <名册件>`）——
// 底层是 `filepath.Walk(base)`，对非目录根**恰好**调 walkFn 一次 ⇒ 单件与目录在扫描面上是同一件事。
// 三条成对：
// ① 指**目录** ⇒ 正常扫到（收窄到该目录）；
// ② 指**常规件** ⇒ **能扫到**（旧口径在这一格退 2 —— 已废）；
// ③ 指**不存在的路径** ⇒ 仍 **rc=2**（不许静默扫成零命中）。
// ★ 新文案仍含旧子串 ⇒ 分辨新老行为**靠 rc，不靠文案**。
func TestCodeFind_PathAcceptsRegularFile(t *testing.T) {
	bin := zergBinary(t)
	root := syntheticRepo(t, "exit 0\n")
	mustWrite(t, filepath.Join(root, "aaa", "one.go"), "package aaa\n// needle\n")
	mustWrite(t, filepath.Join(root, "bbb", "two.go"), "package bbb\n// needle\n")

	// ① 指目录 ⇒ 正常扫到（收窄到 aaa/，不碰 bbb/）。
	rc, out, errb := execCase(t, bin, root, "code", "find", "needle", "--path", "aaa", "--json", "path")
	if rc != 0 {
		t.Fatalf("--path <目录> ⇒ 退 0，得到 %d · stderr=%s", rc, errb)
	}
	if !strings.Contains(out, "aaa/one.go") || strings.Contains(out, "bbb/two.go") {
		t.Errorf("--path <目录> 没正常收窄到 aaa/：%q", out)
	}

	// ② 指常规件 ⇒ 能扫到（rc=0 · 恰好这一件）。
	rc, out, errb = execCase(t, bin, root, "code", "find", "needle", "--path", "aaa/one.go", "--json", "path")
	if rc != 0 {
		t.Fatalf("--path <常规件> ⇒ 退 0（旧口径在这里退 2），得到 %d · stderr=%s", rc, errb)
	}
	if !strings.Contains(out, "aaa/one.go") {
		t.Errorf("--path <常规件> 没扫到这一件：%q", out)
	}
	if strings.Contains(out, "bbb/two.go") {
		t.Errorf("--path <常规件> 不该扫到别的件：%q", out)
	}

	// ③ 指不存在的路径 ⇒ 仍退 2（且必须说出理由，不许静默扫成零命中）。
	rc, _, errb = execCase(t, bin, root, "code", "find", "needle", "--path", "zz-nope")
	if rc != 2 {
		t.Errorf("--path <不存在的路径> ⇒ 退 2，得到 %d · stderr=%s", rc, errb)
	}
	if strings.TrimSpace(errb) == "" {
		t.Errorf("--path <不存在的路径> 退了 2 却没给理由（stderr 空）")
	}
}
