// family_impact_dup_test.go —— 退役容忍负控（缺口 `GAP-20261007-16` 类①「Go 半硬要求在盘 py」）。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// wallDupMkTree 造一棵**最小有效**茧壁树（三档闭集常量 + 一份带 `argv` 的 plan ⇒ Go 半判 rc=0）。
func wallDupMkTree(t *testing.T, mk func(string, string)) {
	t.Helper()
	mk("core/cmd/zerg/wall.go", "package main\n")
	mk("core/cmd/zerg/layers.go",
		"package main\n\nconst (\n\tlayerHost  = \"host\"\n\tlayerNode  = \"node\"\n\tlayerSpace = \"space\"\n)\n")
	mk("wall/plan.json", "{\"argv\": [\"ls\"]}\n")
}

// ★ 退役容忍（缺口 `GAP-20261007-16` · 类①「硬要求在盘 ⇒ 退役态不过问」）：茧壁门 py 半
// `scripts/gates/check-wall-layer.py` 退役（不在盘）而其 **Go 半** `core/cmd/zerg/check_wall_layer.go`
// 在盘 ⇒ `impactDupWallGate` 回落 Go 半进程内现跑（rc 与**有效树**上的一致、✗ 不落「读不到」rc=2）；
// 两半皆缺（真消失）⇒ 保留原 fail-closed（「读不到」· 缺件 ≠ 绿）。
// 哑化 `impactDupWallGate` 里的 Go 半回落支 ⇒ 第 ① 格 FAIL（带本行号）。
func TestImpactDupWallGateRetiredTolerant(t *testing.T) {
	dir := t.TempDir()
	mk := func(rel, body string) {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	wallDupMkTree(t, mk)
	mk(impactDupWallGoHalfRel, "package main\n")
	// ① py 缺 · Go 半在盘 ⇒ 退役：走 Go 半（rc 与「有效树」一致 = 0 · ✗ 不落「读不到」rc=2）
	if !impactDupWallRetired(dir) {
		t.Fatal("退役判定错：py 缺 + Go 半在 ⇒ 应判退役")
	}
	rc, first, _ := impactDupWallGate(dir)
	if rc < 0 || first == "读不到" {
		t.Fatalf("退役态应回落 Go 半（✗ 不落「读不到」），实 rc=%d first=%q", rc, first)
	}
	if rc != 0 {
		t.Fatalf("退役态（有效树）应 rc=0（✗ 不得 rc=2），实 rc=%d first=%q", rc, first)
	}
	var out, errb bytes.Buffer
	if wantRC := walllayerGateMain(&out, &errb, []string{dir}); rc != wantRC {
		t.Fatalf("回落读数与 Go 半不一致：rc=%d 期望 %d", rc, wantRC)
	}
	// ② py 在盘 ⇒ 非退役（在盘照走 py 子进程 · 单一真源不变）
	mk(impactDupWallPyRel, "#!/usr/bin/env python3\nprint('STUB-WALL')\n")
	if impactDupWallRetired(dir) {
		t.Fatal("py 在盘 ⇒ 非退役")
	}
	if rc2, first2, _ := impactDupWallGate(dir); rc2 != 0 || first2 != "STUB-WALL" {
		t.Fatalf("py 在盘应走 py 子进程（rc=0 · 首行 STUB-WALL），实 rc=%d first=%q", rc2, first2)
	}
	// ③ 两缺（真消失）⇒ 非退役 ⇒ fail-closed 保留「读不到」（缺件 ≠ 绿）
	if err := os.Remove(filepath.Join(dir, filepath.FromSlash(impactDupWallPyRel))); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, filepath.FromSlash(impactDupWallGoHalfRel))); err != nil {
		t.Fatal(err)
	}
	if impactDupWallRetired(dir) {
		t.Fatal("两缺 ⇒ 非退役（真消失，不许当退役）")
	}
	if rc3, first3, _ := impactDupWallGate(dir); rc3 != -1 || first3 != "读不到" {
		t.Fatalf("两缺应保留 fail-closed「读不到」，实 rc=%d first=%q", rc3, first3)
	}
}
