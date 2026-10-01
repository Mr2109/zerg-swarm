// check_curl_timeout_test.go —— `core/cmd/zerg/check_curl_timeout.go`（M2 真搬件）的成对负控。
//
// 目的：给 `checkCurlTimeoutGateMain` 一道**带牙**的回归 —— 哑化任一条判据（如「漏上界 ⇒ 红」或
// 「整行注释 ⇒ 夹具档」）⇒ 必有一格 FAIL。合成件全在 `t.TempDir()`（仓外 · 不碰真目标）。
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// cktTEntry —— 进程内取 (rc, stdout, stderr)。
func cktTEntry(argv ...string) (int, string, string) {
	var so, se strings.Builder
	rc := checkCurlTimeoutGateMain(&so, &se, argv)
	return rc, so.String(), se.String()
}

// cktTFix —— 造一个「假仓根/scripts」夹具，回 (仓根, scripts 目录)。
func cktTFix(t *testing.T) (string, string) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "repo")
	s := filepath.Join(root, "scripts")
	if err := os.MkdirAll(s, 0o755); err != nil {
		t.Fatal(err)
	}
	return root, s
}

func cktTWrite(t *testing.T, p, body string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestCheckCurlTimeoutGatePort —— 正控 + 成对负控（带牙：哑化任一判据 ⇒ 必 FAIL）。
func TestCheckCurlTimeoutGatePort(t *testing.T) {
	t.Run("正控 真调用逐行带 -m", func(t *testing.T) {
		root, s := cktTFix(t)
		cktTWrite(t, filepath.Join(s, "a.sh"), "curl -m 5 https://example.invalid/a\n")
		cktTWrite(t, filepath.Join(s, "b.sh"), "curl --max-time 10 https://example.invalid/b\n")
		rc, out, _ := cktTEntry("--root", s)
		if rc != 0 || !strings.Contains(out, "口径成立") || !strings.Contains(out, "真调用 2 行") {
			t.Fatalf("正控失败：rc=%d out=%q", rc, out)
		}
		_ = root
	})

	t.Run("负控 漏上界 ⇒ rc=1 逐行点名", func(t *testing.T) {
		_, s := cktTFix(t)
		cktTWrite(t, filepath.Join(s, "bad.sh"), "curl https://example.invalid/bad\n")
		rc, out, _ := cktTEntry("--root", s)
		if rc != 1 || !strings.Contains(out, "漏上界 1 行") || !strings.Contains(out, "bad.sh:1") {
			t.Fatalf("负控失败：rc=%d out=%q", rc, out)
		}
	})

	t.Run("负控 整行注释 ⇒ 夹具档 not 红", func(t *testing.T) {
		_, s := cktTFix(t)
		cktTWrite(t, filepath.Join(s, "c.sh"), "# curl https://example.invalid/c\n")
		rc, out, _ := cktTEntry("--root", s)
		if rc != 0 || !strings.Contains(out, "夹具/注释 1 行") || !strings.Contains(out, "[夹具/注释] c.sh:1") {
			t.Fatalf("注释档失败：rc=%d out=%q", rc, out)
		}
	})

	t.Run("负控 .py 无 subprocess ⇒ 夹具档", func(t *testing.T) {
		_, s := cktTFix(t)
		cktTWrite(t, filepath.Join(s, "d.py"), "Y = \"x curl https://example.invalid/d\"\n")
		rc, out, _ := cktTEntry("--root", s)
		if rc != 0 || !strings.Contains(out, "夹具/注释 1 行") {
			t.Fatalf(".py 字面量档失败：rc=%d out=%q", rc, out)
		}
	})

	t.Run("负控 .py 有 subprocess ⇒ 真调用", func(t *testing.T) {
		_, s := cktTFix(t)
		cktTWrite(t, filepath.Join(s, "e.py"), "subprocess.run(\"x curl https://x/e\")\n")
		rc, out, _ := cktTEntry("--root", s)
		if rc != 1 || !strings.Contains(out, "漏上界 1 行") || !strings.Contains(out, "e.py:1") {
			t.Fatalf(".py 真调用档失败：rc=%d out=%q", rc, out)
		}
	})

	t.Run("负控 --max-time= 等号形 ⇒ 带界", func(t *testing.T) {
		_, s := cktTFix(t)
		cktTWrite(t, filepath.Join(s, "f.sh"), "curl --max-time=3 https://example.invalid/f\n")
		rc, out, _ := cktTEntry("--root", s)
		if rc != 0 || !strings.Contains(out, "真调用 1 行") || !strings.Contains(out, "漏上界 0 行") {
			t.Fatalf("等号形失败：rc=%d out=%q", rc, out)
		}
	})

	t.Run("负控 根不存在 ⇒ rc=2 不给结论(stderr)", func(t *testing.T) {
		_, s := cktTFix(t)
		rc, _, errs := cktTEntry("--root", filepath.Join(s, "no-such"))
		if rc != 2 || !strings.Contains(errs, "BLOCKED") || !strings.Contains(errs, "不给结论") {
			t.Fatalf("根不存在失败：rc=%d stderr=%q", rc, errs)
		}
	})

	t.Run("负控 B类读失败 chmod000 ⇒ rc=2 BLOCKED 逐件点名", func(t *testing.T) {
		_, s := cktTFix(t)
		cktTWrite(t, filepath.Join(s, "ok.sh"), "curl -m 5 https://example.invalid/ok\n")
		locked := filepath.Join(s, "locked.sh")
		cktTWrite(t, locked, "curl -m 5 https://example.invalid/locked\n")
		if err := os.Chmod(locked, 0o000); err != nil {
			t.Skipf("chmod 不支持：%v", err)
		}
		defer os.Chmod(locked, 0o644) // ★ 自回收：跑完恢复可读（同包 delta=0 体例）
		if os.Geteuid() == 0 {
			t.Skip("root 下 chmod 000 仍可读 ⇒ 该格无鉴别力")
		}
		rc, out, errs := cktTEntry("--root", s)
		if rc != 2 {
			t.Fatalf("B类读失败应 rc=2：rc=%d out=%q stderr=%q", rc, out, errs)
		}
		if !strings.Contains(errs, "BLOCKED") || !strings.Contains(errs, "读不到") || !strings.Contains(errs, "不给结论") {
			t.Fatalf("读失败缺 BLOCKED 头：stderr=%q", errs)
		}
		if !strings.Contains(errs, "有 1 件") || !strings.Contains(errs, "[读失败] locked.sh") {
			t.Fatalf("读失败未逐件点名锁死件：stderr=%q", errs)
		}
		if out != "" {
			t.Fatalf("读失败不应给结论(stdout 须空)：out=%q", out)
		}
	})

	t.Run("空面(0 件) ⇒ rc=0", func(t *testing.T) {
		_, s := cktTFix(t)
		rc, out, _ := cktTEntry("--root", s)
		if rc != 0 || !strings.Contains(out, "真调用 0 行") {
			t.Fatalf("空面失败：rc=%d out=%q", rc, out)
		}
	})

	t.Run("--json 机器面", func(t *testing.T) {
		_, s := cktTFix(t)
		cktTWrite(t, filepath.Join(s, "bad.sh"), "curl https://example.invalid/bad\n")
		rc, out, _ := cktTEntry("--root", s, "--json")
		if rc != 1 || !strings.Contains(out, "CurlTimeoutScan") ||
			!strings.Contains(out, `"without_bound": 1`) || !strings.Contains(out, `"file": "bad.sh"`) {
			t.Fatalf("--json 失败：rc=%d out=%q", rc, out)
		}
	})

	t.Run("--json 空集面", func(t *testing.T) {
		_, s := cktTFix(t)
		rc, out, _ := cktTEntry("--root", s, "--json")
		if rc != 0 || !strings.Contains(out, `"missing": []`) {
			t.Fatalf("--json 空集失败：rc=%d out=%q", rc, out)
		}
	})

	t.Run("--self-test 三档", func(t *testing.T) {
		rc, out, _ := cktTEntry("--self-test")
		if rc != 0 || !strings.Contains(out, "自检：全过") {
			t.Fatalf("--self-test 失败：rc=%d out=%q", rc, out)
		}
	})

	t.Run("负控 未知旗标 ⇒ rc=2", func(t *testing.T) {
		rc, _, errs := cktTEntry("--nope")
		if rc != 2 || !strings.Contains(errs, "用法错") {
			t.Fatalf("未知旗标失败：rc=%d stderr=%q", rc, errs)
		}
	})
}
