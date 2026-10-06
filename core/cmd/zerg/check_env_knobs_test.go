// check_env_knobs_test.go —— `core/cmd/zerg/check_env_knobs.go`（M2 真搬件 · py `check-env-knobs.py`）的成对负控。
//
// 目的：给 `checkEnvKnobsGateMain` 一道**带牙**的回归 —— 哑化任一条判据（A 漏登记 / B 陈旧 / C 三格不齐 /
// 空域守卫 / 退码分档）⇒ 必有一格 FAIL（`t.Fatalf` 带行号）。合成件全在 `t.TempDir()`（仓外 · 不碰真目标）。
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// cenkTEntry —— 进程内取 (rc, stdout, stderr)。
func cenkTEntry(argv ...string) (int, string, string) {
	var so, se strings.Builder
	rc := checkEnvKnobsGateMain(&so, &se, argv)
	return rc, so.String(), se.String()
}

// cenkTFix —— 造一个「假仓根」夹具，回 (仓根, scripts/gates 目录)。
func cenkTFix(t *testing.T) (string, string) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "repo")
	g := filepath.Join(root, "scripts", "gates")
	if err := os.MkdirAll(g, 0o755); err != nil {
		t.Fatal(err)
	}
	return root, g
}

// cenkTWriteHost —— 把指定旋钮名写成 `${NAME:-/x}` 消费形态（正控用：从现行登记表逐键生成）。
func cenkTWriteHost(t *testing.T, g string, names []string) {
	t.Helper()
	var b strings.Builder
	for _, n := range names {
		fmt.Fprintf(&b, "R=\"${%s:-/x}\"\n", n)
	}
	if err := os.WriteFile(filepath.Join(g, "precommit-gates.sh"), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

// cenkTSetKnobs —— 临时改写包级登记表，测试结束自清还原（✗ 防跨用例传染）。
func cenkTSetKnobs(t *testing.T, m map[string]cenkKnob) {
	t.Helper()
	saved := cenkKnobs
	cenkKnobs = m
	t.Cleanup(func() { cenkKnobs = saved })
}

// TestCheckEnvKnobsGatePort —— 正控 + 成对负控（带牙：哑化任一判据 ⇒ 必 FAIL 带行号）。
func TestCheckEnvKnobsGatePort(t *testing.T) {
	t.Run("正控 全登记齐 ⇒ rc=0", func(t *testing.T) {
		root, g := cenkTFix(t)
		names := make([]string, 0, len(cenkKnobs))
		for k := range cenkKnobs {
			names = append(names, k)
		}
		cenkTWriteHost(t, g, names)
		rc, out, _ := cenkTEntry("--root", root)
		if rc != 0 || !strings.Contains(out, "⇒ 全绿（rc=0）") ||
			!strings.Contains(out, fmt.Sprintf("登记表 = %d 行", len(cenkKnobs))) {
			t.Fatalf("正控失败：rc=%d out=%q", rc, out)
		}
	})

	t.Run("负控 A 漏登记 ⇒ rc=1 逐条点名", func(t *testing.T) {
		root, g := cenkTFix(t)
		names := make([]string, 0, len(cenkKnobs)+1)
		for k := range cenkKnobs {
			names = append(names, k)
		}
		names = append(names, "ZERG_BRAND_NEW")
		cenkTWriteHost(t, g, names)
		rc, out, _ := cenkTEntry("--root", root)
		if rc != 1 || !strings.Contains(out, "A 漏登记（消费面检出、登记表里没有）：1 个") ||
			!strings.Contains(out, "✗ ZERG_BRAND_NEW（读者：scripts/gates/precommit-gates.sh）") ||
			!strings.Contains(out, "⇒ 违规 1 处（rc=1）") {
			t.Fatalf("A 面失败：rc=%d out=%q", rc, out)
		}
	})

	t.Run("负控 B 登记陈旧 ⇒ rc=1", func(t *testing.T) {
		root, g := cenkTFix(t)
		cenkTWriteHost(t, g, []string{"ZERG_GATE_REPO_ROOT"})
		rc, out, _ := cenkTEntry("--root", root)
		if rc != 1 || !strings.Contains(out, "B 登记陈旧（表里点名、消费面查无）：") ||
			!strings.Contains(out, "✗ ZERG_REAL_GATES_JOBS") {
			t.Fatalf("B 面失败：rc=%d out=%q", rc, out)
		}
	})

	t.Run("负控 C 三格不齐 ⇒ rc=2(BLOCKED·stderr)", func(t *testing.T) {
		root, g := cenkTFix(t)
		cenkTWriteHost(t, g, []string{"ZERG_GATE_REPO_ROOT"})
		cenkTSetKnobs(t, map[string]cenkKnob{"ZERG_GATE_REPO_ROOT": {Readers: "", Swap: "y", Poison: "z"}})
		rc, out, errs := cenkTEntry("--root", root)
		if rc != 2 || out != "" ||
			!strings.Contains(errs, "BLOCKED —— 登记项三格不齐：ZERG_GATE_REPO_ROOT ⇒ rc=2") {
			t.Fatalf("C 面失败：rc=%d out=%q stderr=%q", rc, out, errs)
		}
	})

	t.Run("负控 空域(宿主面 0 消费) ⇒ rc=2 fail-closed", func(t *testing.T) {
		root, g := cenkTFix(t)
		if err := os.WriteFile(filepath.Join(g, "precommit-gates.sh"), []byte("# 无旋钮\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		rc, out, errs := cenkTEntry("--root", root)
		if rc != 2 || out != "" ||
			!strings.Contains(errs, "BLOCKED —— 旋钮宿主面 0 消费（取不到 ≠ 无旋钮）⇒ rc=2") ||
			!strings.Contains(errs, "盘上缺：") {
			t.Fatalf("空域失败：rc=%d out=%q stderr=%q", rc, out, errs)
		}
	})

	t.Run("负控 根不存在(全 miss) ⇒ rc=2", func(t *testing.T) {
		root, _ := cenkTFix(t)
		rc, out, errs := cenkTEntry("--root", filepath.Join(root, "no-such"))
		if rc != 2 || out != "" || !strings.Contains(errs, "BLOCKED") ||
			!strings.Contains(errs, "scripts/gates/precommit-gates.sh") {
			t.Fatalf("缺件失败：rc=%d out=%q stderr=%q", rc, out, errs)
		}
	})

	t.Run("--json 机器面 A 面 ⇒ rc=1", func(t *testing.T) {
		root, g := cenkTFix(t)
		cenkTWriteHost(t, g, []string{"ZERG_BRAND_NEW"})
		rc, out, _ := cenkTEntry("--root", root, "--json")
		if rc != 1 || !strings.Contains(out, `"rc": 1`) ||
			!strings.Contains(out, `"unregistered": ["ZERG_BRAND_NEW"]`) {
			t.Fatalf("--json 失败：rc=%d out=%q", rc, out)
		}
	})

	t.Run("--json 空域 ⇒ rc=2(单行 JSON)", func(t *testing.T) {
		root, g := cenkTFix(t)
		if err := os.WriteFile(filepath.Join(g, "precommit-gates.sh"), []byte("# 无旋钮\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		rc, out, _ := cenkTEntry("--root", root, "--json")
		if rc != 2 || !strings.Contains(out, `"rc": 2`) || !strings.Contains(out, `"miss": [`) {
			t.Fatalf("--json 空域失败：rc=%d out=%q", rc, out)
		}
	})

	t.Run("--self-test 5 格", func(t *testing.T) {
		rc, out, errs := cenkTEntry("--self-test")
		if rc != 0 || !strings.Contains(out, "自检 PASS（5 格）") ||
			!strings.Contains(errs, "登记项三格不齐") || !strings.Contains(errs, "旋钮宿主面 0 消费") {
			t.Fatalf("--self-test 失败：rc=%d out=%q stderr=%q", rc, out, errs)
		}
	})

	t.Run("负控 未知旗标 ⇒ rc=2", func(t *testing.T) {
		rc, _, errs := cenkTEntry("--nope")
		if rc != 2 || !strings.Contains(errs, "用法错") {
			t.Fatalf("未知旗标失败：rc=%d stderr=%q", rc, errs)
		}
	})
}
