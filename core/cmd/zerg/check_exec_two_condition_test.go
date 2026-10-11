// check_exec_two_condition_test.go —— `core/cmd/zerg/check_exec_two_condition.go`（M2 真搬件）的成对负控。
//
// 目的：给 `checkExecTwoConditionGateMain` 一道**带牙**的回归 —— 哑化任一档（执行位六形态 / 报告档 / 登记档 /
// 四类护栏 / 空域守卫 / 分档不驱动 rc / json 三态）⇒ 必有一格 FAIL。合成件全在 `t.TempDir()`（仓外 · 不碰真目标）。
//
// ★ 自指面：本测试件落在**它自己扫描的面内**（`core/**/*.go`）⇒ 夹具内容一律走 base64（复用主件 `cetcB64`），
//
//	源码里无任何连续 E1–E6 / P1 / P2 / P3 / 仓外输入 字面形态。
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	cetcTbSwap = "QklOPSIke1pFUkdfU1dBUF9CSU46LWJpbi96ZXJnLWNvcmV9IgpldmFsICIkWkVSR19TV0FQX1JFU1RBUlRfU1RVQiIK"
	cetcTbOnly = "ZXZhbCAiJGxpdGVyYWxfY21kIgo="
	cetcTbP1   = "aW1wb3J0IHN1YnByb2Nlc3MsIG9zClggPSBvcy5lbnZpcm9uLmdldCgiWkVSR19CSU4iKQpzdWJwcm9jZXNzLnJ1bihYLCBzaGVsbD1UcnVlKQpZID0gIiR7WkVSR19IT01FOi0vdG1wfSIK"
	cetcTbP3   = "cGFja2FnZSBtYWluCgppbXBvcnQgIm9zIgoKZnVuYyBmKCkgewoJY21kIDo9IGV4ZWMuQ29tbWFuZChvcy5HZXRlbnYoIlpFUkdfQklOIiksICJydW4iKQoJXyA9IGNtZAp9Cg=="
	cetcTbP2   = "UlVOTkVSPSIke1pFUkdfUlVOTkVSOi1iaW4vcnVufSIKYmFzaCAiJFJVTk5FUiIgLS1nbwo="
	cetcTbHd   = "WD0iJHtaRVJHX1g6LXZ9IgpjYXQgPDxFT0YKYmFzaCAiJFJVTk5FUiIKRU9GCg=="
	cetcTbCm   = "WD0iJHtaRVJHX1g6LXZ9IgojIGJhc2ggIiRSVU5ORVIiCg=="
	cetcTbQ    = "WCA9ICIke1pFUkdfWDotdn0iCnRleHQgPSAnYmFzaCAiJFJVTk5FUiInCg=="
	cetcTbDup  = "QT0iJHtaRVJHX0E6LXh9IgpiYXNoICIkQSIKYmFzaCAiJEEiIC0tYWdhaW4K"
)

func cetcTEntry(argv ...string) (int, string, string) {
	var so, se strings.Builder
	rc := checkExecTwoConditionGateMain(&so, &se, argv)
	return rc, so.String(), se.String()
}

// cetcTFix —— 造「假仓根」：回 (仓根, <仓根>/scripts)。扫描根用**仓根**（与真仓同摆位）。
func cetcTFix(t *testing.T) (string, string) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "repo")
	s := filepath.Join(root, "scripts")
	if err := os.MkdirAll(s, 0o755); err != nil {
		t.Fatal(err)
	}
	return root, s
}

func cetcTWrite(t *testing.T, p, b64 string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, cetcB64(b64), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestCheckExecTwoConditionGatePort —— 正控 + 成对负控（带牙：哑化任一档 ⇒ 必 FAIL）。
func TestCheckExecTwoConditionGatePort(t *testing.T) {
	t.Run("正控 同件双条件(E1+仓外输入) ⇒ rc=1 点名", func(t *testing.T) {
		root, s := cetcTFix(t)
		cetcTWrite(t, filepath.Join(s, "build", "swap.sh"), cetcTbSwap)
		rc, out, _ := cetcTEntry("--root", root)
		if rc != 1 || !strings.Contains(out, "未白名单命中 = 1 处") || !strings.Contains(out, "scripts/build/swap.sh 形态 E1") {
			t.Fatalf("正控失败：rc=%d out=%q", rc, out)
		}
	})
	t.Run("负控 只有执行位无仓外输入 ⇒ rc=0", func(t *testing.T) {
		root, s := cetcTFix(t)
		cetcTWrite(t, filepath.Join(s, "only.sh"), cetcTbOnly)
		rc, out, _ := cetcTEntry("--root", root)
		if rc != 0 || !strings.Contains(out, "未白名单命中 = 0 处") {
			t.Fatalf("负控失败：rc=%d out=%q", rc, out)
		}
	})
	t.Run("负控 补形态只报告/登记不驱动 rc ⇒ rc=0", func(t *testing.T) {
		root, s := cetcTFix(t)
		cetcTWrite(t, filepath.Join(s, "a.py"), cetcTbP1)
		cetcTWrite(t, filepath.Join(s, "b.go"), cetcTbP3)
		cetcTWrite(t, filepath.Join(s, "c.sh"), cetcTbP2)
		rc, out, _ := cetcTEntry("--root", root)
		if rc != 0 || !strings.Contains(out, "报告档 2 处") || !strings.Contains(out, "登记档 1 处") {
			t.Fatalf("分档失败：rc=%d out=%q", rc, out)
		}
	})
	t.Run("负控 护栏① heredoc 体内 ⇒ 不登记 P2", func(t *testing.T) {
		root, s := cetcTFix(t)
		cetcTWrite(t, filepath.Join(s, "hd.sh"), cetcTbHd)
		rc, out, _ := cetcTEntry("--root", root)
		if rc != 0 || !strings.Contains(out, "登记档 0 处") {
			t.Fatalf("护栏①失败：rc=%d out=%q", rc, out)
		}
	})
	t.Run("负控 护栏③ 注释位 ⇒ 不登记 P2", func(t *testing.T) {
		root, s := cetcTFix(t)
		cetcTWrite(t, filepath.Join(s, "cm.sh"), cetcTbCm)
		rc, out, _ := cetcTEntry("--root", root)
		if rc != 0 || !strings.Contains(out, "登记档 0 处") {
			t.Fatalf("护栏③失败：rc=%d out=%q", rc, out)
		}
	})
	t.Run("负控 护栏② 行内引号文案 ⇒ 不登记 P2", func(t *testing.T) {
		root, s := cetcTFix(t)
		cetcTWrite(t, filepath.Join(s, "q.py"), cetcTbQ)
		rc, out, _ := cetcTEntry("--root", root)
		if rc != 0 || !strings.Contains(out, "登记档 0 处") {
			t.Fatalf("护栏②失败：rc=%d out=%q", rc, out)
		}
	})
	t.Run("负控 ⑤ P2 重复行去重 ⇒ 登记档 1 处", func(t *testing.T) {
		root, s := cetcTFix(t)
		cetcTWrite(t, filepath.Join(s, "dup.sh"), cetcTbDup)
		rc, out, _ := cetcTEntry("--root", root)
		if rc != 0 || !strings.Contains(out, "登记档 1 处") {
			t.Fatalf("负控⑤失败：rc=%d out=%q", rc, out)
		}
	})
	t.Run("负控 空域(0 件) ⇒ rc=2 不给结论", func(t *testing.T) {
		root, _ := cetcTFix(t)
		rc, out, errs := cetcTEntry("--root", root)
		if rc != 2 || !strings.Contains(errs, "扫面 0 件") || out != "" {
			t.Fatalf("空域失败：rc=%d out=%q err=%q", rc, out, errs)
		}
	})
	t.Run("负控 根不存在 ⇒ rc=2 BLOCKED", func(t *testing.T) {
		root, _ := cetcTFix(t)
		rc, _, errs := cetcTEntry("--root", filepath.Join(root, "nope"))
		if rc != 2 || !strings.Contains(errs, "扫面根不存在") {
			t.Fatalf("根不存在失败：rc=%d err=%q", rc, errs)
		}
	})
	t.Run("--json 机器面逐键 + 三态空数组", func(t *testing.T) {
		root, s := cetcTFix(t)
		cetcTWrite(t, filepath.Join(s, "build", "swap.sh"), cetcTbSwap)
		rc, out, _ := cetcTEntry("--root", root, "--json")
		if rc != 1 || !strings.Contains(out, `"rc": 1`) ||
			!strings.Contains(out, `{"file": "scripts/build/swap.sh", "shape": "E1"}`) {
			t.Fatalf("--json 失败：rc=%d out=%q", rc, out)
		}
		if !strings.Contains(out, `"report": []`) || !strings.Contains(out, `"register": []`) {
			t.Fatalf("--json 空数组三态失败：out=%q", out)
		}
	})
	t.Run("--self-test ⇒ rc=0 且 13 格 PASS", func(t *testing.T) {
		rc, out, _ := cetcTEntry("--self-test")
		if rc != 0 || !strings.Contains(out, "自检 PASS（13 格）") {
			t.Fatalf("--self-test 失败：rc=%d out=%q", rc, out)
		}
	})
	t.Run("缩写前缀 --r/--j ⇒ 同 --root/--json", func(t *testing.T) {
		root, s := cetcTFix(t)
		cetcTWrite(t, filepath.Join(s, "build", "swap.sh"), cetcTbSwap)
		rc, out, _ := cetcTEntry("--r", root, "--j")
		if rc != 1 || !strings.Contains(out, `"file": "scripts/build/swap.sh"`) {
			t.Fatalf("缩写前缀失败：rc=%d out=%q", rc, out)
		}
	})
	t.Run("未知旗标 ⇒ rc=2", func(t *testing.T) {
		rc, _, errs := cetcTEntry("--nope")
		if rc != 2 || !strings.Contains(errs, "usage:") {
			t.Fatalf("未知旗标失败：rc=%d err=%q", rc, errs)
		}
	})
	t.Run("默认根走 env ZERG_ROOT", func(t *testing.T) {
		root, s := cetcTFix(t)
		cetcTWrite(t, filepath.Join(s, "build", "swap.sh"), cetcTbSwap)
		t.Setenv("ZERG_ROOT", root)
		rc, out, _ := cetcTEntry()
		if rc != 1 || !strings.Contains(out, "形态 E1") {
			t.Fatalf("默认根(env)失败：rc=%d out=%q", rc, out)
		}
	})
	t.Run("-h 帮助 ⇒ rc=0", func(t *testing.T) {
		rc, out, _ := cetcTEntry("-h")
		if rc != 0 || !strings.Contains(out, "usage:") {
			t.Fatalf("-h 失败：rc=%d out=%q", rc, out)
		}
	})
}

// ── B 类（读失败静默）回归牙 ────────────────────────────────────────────────────
// 扫描面内一件 / 一个目录锁成 chmod 000（仓外夹具 · ✗ 不删仓内件）⇒ 读失败登记面（cetcReadFails）必响：
// rc=2 + stderr BLOCKED 头 + 逐件点名。删回静默（原 py `except OSError` 返回空集 / `os.walk(onerror=None)`）
// 本用例必红。同族先例：check_doc_cmds_test.go 的 TestCheckDocCmdsReadFailTeeth。
func TestCheckExecTwoConditionReadFailTeeth(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root 绕过权限位 ⇒ chmod 000 不挡读，本用例无鉴别力")
	}
	t.Run("件面", func(t *testing.T) {
		root, s := cetcTFix(t)
		cetcTWrite(t, filepath.Join(s, "good.py"), cetcTbP1)
		locked := filepath.Join(s, "locked.py")
		cetcTWrite(t, locked, cetcTbP1)
		if err := os.Chmod(locked, 0o000); err != nil {
			t.Fatal(err)
		}
		defer os.Chmod(locked, 0o644)
		rc, out, errs := cetcTEntry("--root", root)
		if rc != 2 {
			t.Fatalf("件读失败回归牙：应 rc=2，得 rc=%d out=%q stderr=%q", rc, out, errs)
		}
		if !strings.Contains(errs, "BLOCKED") || !strings.Contains(errs, "读不到") || !strings.Contains(errs, "locked.py") {
			t.Fatalf("件读失败回归牙：stderr 须带 BLOCKED 头 + 点名 locked.py，得 stderr=%q", errs)
		}
		if out != "" {
			t.Fatalf("件读失败不应给结论(stdout 须空)：out=%q", out)
		}
	})
	t.Run("目录面", func(t *testing.T) {
		root, s := cetcTFix(t)
		cetcTWrite(t, filepath.Join(s, "good.py"), cetcTbP1)
		locked := filepath.Join(s, "lockeddir")
		if err := os.MkdirAll(locked, 0o755); err != nil {
			t.Fatal(err)
		}
		cetcTWrite(t, filepath.Join(locked, "inner.py"), cetcTbP1)
		if err := os.Chmod(locked, 0o000); err != nil {
			t.Fatal(err)
		}
		defer os.Chmod(locked, 0o755)
		rc, out, errs := cetcTEntry("--root", root)
		if rc != 2 {
			t.Fatalf("目录读失败回归牙：应 rc=2，得 rc=%d out=%q stderr=%q", rc, out, errs)
		}
		if !strings.Contains(errs, "BLOCKED") || !strings.Contains(errs, "lockeddir") {
			t.Fatalf("目录读失败回归牙：stderr 须带 BLOCKED 头 + 点名 lockeddir，得 stderr=%q", errs)
		}
		if out != "" {
			t.Fatalf("目录读失败不应给结论(stdout 须空)：out=%q", out)
		}
	})
	// 成对反控（✗ 禁恒红）：可读同形 ⇒ rc=0 且 stderr 无 BLOCKED。
	t.Run("成对反控", func(t *testing.T) {
		root, s := cetcTFix(t)
		cetcTWrite(t, filepath.Join(s, "only.sh"), cetcTbOnly)
		rc, _, errs := cetcTEntry("--root", root)
		if rc != 0 || strings.Contains(errs, "BLOCKED") {
			t.Fatalf("成对反控失败：好件应 rc=0 且无 BLOCKED，得 rc=%d stderr=%q", rc, errs)
		}
	})
}
