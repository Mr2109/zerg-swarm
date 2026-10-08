// check_ghost_decl_diff_test.go —— `check_ghost_decl_diff.go` 的成对负控测试件（M2 · 不接线）。
//
// 夹具一律落 `t.TempDir()`（**自回收** · 跑后不留件 · 不碰真目标）。
// 负控「有牙」的实证在仓外副本上做（把已落仓件的 S2 判据哑化 ⇒ 本用例 FAIL 带行号），
// 见本单回执；本文件只跑"真件"的绿灯面。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const tgGoodDecl = "# 夹具声明树\n" +
	"kind\tname\t声明件\t程序\t进程特征\t归属\t处置\n" +
	"launchd\tcom.zerg.core\t~/Library/LaunchAgents/com.zerg.core.plist\t/bin/bash scripts/svc/zerg-core-daemon.sh\tbin/zerg-core\t主控\t-\n" +
	"launchd\tcom.zerg.agent-Mr2109\t~/Library/LaunchAgents/com.zerg.agent-Mr2109.plist\tbin/zerg-agentd --machine Mr2109\tbin/zerg-agentd\t子端\t-\n" +
	"ghost\tcocoon-docs-service\t-\tbin/cocoon-docs-service -port 8610\tbin/cocoon-docs-service\t文档茧\t进 reap 候选 · 待 Mr2109 拍\n"

const tgGoodPS = "  68939     1       56:36 /repo/bin/zerg-core\n" +
	"  14149     1 03-15:30:40 /repo/bin/zerg-agentd --host 127.0.0.1 --port 8100 --machine Mr2109\n" +
	"  79317     1 04-22:34:32 /repo/bin/cocoon-docs-service -port 8610\n"

const tgGoodLC = "68939\t0\tcom.zerg.core\n14149\t0\tcom.zerg.agent-Mr2109\n"

func tgDoctor(n int) string {
	return `{"schema": "zerg/v1", "kind": "DoctorCheck", "items": [{"name": "回收候选（幽灵服务）", "verdict": "REPORT", "detail": "` +
		itoa(n) + ` 条：pid=79317 /repo/bin/cocoon-docs-service -port 8610", "advice": ""}]}`
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	if n == 1 {
		return "1"
	}
	return "2"
}

func tgWrite(t *testing.T, p, s string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

// tgFixture 建一套自回收夹具（在 t.TempDir() 内）。
func tgFixture(t *testing.T, plists []string) []string {
	t.Helper()
	d := t.TempDir()
	_ = os.MkdirAll(filepath.Join(d, "scripts", "gates"), 0o755)
	// 同源件（门⑧）必须在对拍根下 —— 从真仓拷一份（夹具在 t.TempDir() 内 · 自回收）
	tgWrite(t, filepath.Join(d, "scripts", "gates", "check-ghost-decl-diff.py"), "#!/usr/bin/env python3\n")
	if b, err := os.ReadFile(filepath.Join(tgRepoRoot(t), "scripts", "gates", "check-service-declaration.py")); err == nil {
		tgWrite(t, filepath.Join(d, "scripts", "gates", "check-service-declaration.py"), string(b))
	}
	_ = os.MkdirAll(filepath.Join(d, "core", "cmd", "zerg"), 0o755)
	// ★ 退役态（G17 · 2026-10-07 退役批）：同源件 py 退役后口径真源改由 Go 半承载 ⇒ 夹具同备一份
	//   （含四个同源口径符号，与 gddSiblingGoHalfOK 现读的那组一一对应）。py 在盘时这份不被读（走 py 那把）；
	//   退役态它顶上 ⇒ 测「在盘 / 退役」两态都过。
	tgWrite(t, filepath.Join(d, "core", "cmd", "zerg", "check_service_declaration.go"),
		"package main\n\nfunc csdParseDecl() {}\n\nvar csdProcHints = []string{}\n\nfunc csdReadLines() {}\n\nfunc csdLabelSetDiff() {}\n")
	tgWrite(t, filepath.Join(d, "core", "cmd", "zerg", "main.go"), "package main\n")
	dp := filepath.Join(d, "服务声明.tsv")
	pp := filepath.Join(d, "ps.txt")
	lp := filepath.Join(d, "launchctl.txt")
	tgWrite(t, dp, tgGoodDecl)
	tgWrite(t, pp, tgGoodPS)
	tgWrite(t, lp, tgGoodLC)
	home := filepath.Join(d, "home")
	for _, n := range plists {
		tgWrite(t, filepath.Join(home, "Library", "LaunchAgents", n), "plist\n")
	}
	return []string{d, dp, pp, lp, home}
}

func tgRepoRoot(t *testing.T) string {
	t.Helper()
	d, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for n := 0; n < 10; n++ {
		if st, err := os.Stat(filepath.Join(d, "scripts", "gates", "check-service-declaration.py")); err == nil && !st.IsDir() {
			return d
		}
		// ★ 退役态（G17 · 2026-10-07 退役批）：同源件 py 退役 ⇒ 改认其 **Go 半**所在件做仓根锚
		//   （口径真源换人后仓根仍可定位）·✗ 不再 t.Fatal。
		if st, err := os.Stat(filepath.Join(d, "core", "cmd", "zerg", "check_service_declaration.go")); err == nil && !st.IsDir() {
			return d
		}
		p := filepath.Dir(d)
		if p == d {
			break
		}
		d = p
	}
	t.Skip("仓根未定位到同源件（check-service-declaration.py 或 Go 半）⇒ 跳过对拍（退役容忍）")
	return ""
}

func tgRun(argv []string) (int, string, string) {
	var so, se bytes.Buffer
	rc := checkGhostDeclDiffGateMain(&so, &se, argv)
	return rc, so.String(), se.String()
}

func TestCheckGhostDeclDiffGatePort(t *testing.T) {
	t.Run("AllGreen", func(t *testing.T) {
		f := tgFixture(t, []string{"com.zerg.core.plist", "com.zerg.agent-Mr2109.plist"})
		df := filepath.Join(f[0], "doctor.json")
		tgWrite(t, df, tgDoctor(1))
		rc, out, _ := tgRun([]string{"--root", f[0], "--decl", f[1], "--ps", f[2], "--launchctl", f[3],
			"--home", f[4], "--doctor", df})
		if rc != 0 {
			t.Fatalf("rc=%d want 0\n%s", rc, out)
		}
		if !strings.Contains(out, "侧①「**盘上有 / 声明无**」") || !strings.Contains(out, "：1 条") {
			t.Fatalf("side1 未出 1 条:\n%s", out)
		}
		if !strings.Contains(out, "逐字同值 ✓") {
			t.Fatalf("S2 同源断言未过:\n%s", out)
		}
	})

	t.Run("S2DriftRed", func(t *testing.T) {
		f := tgFixture(t, []string{"com.zerg.core.plist", "com.zerg.agent-Mr2109.plist"})
		df := filepath.Join(f[0], "doctor0.json")
		tgWrite(t, df, tgDoctor(0))
		rc, out, _ := tgRun([]string{"--root", f[0], "--decl", f[1], "--ps", f[2], "--launchctl", f[3],
			"--home", f[4], "--doctor", df})
		if rc != 1 {
			t.Fatalf("rc=%d want 1（同源漂移判红）\n%s", rc, out)
		}
		if !strings.Contains(out, "FAIL S2 同源漂移") {
			t.Fatalf("缺 S2 漂移正文:\n%s", out)
		}
	})

	t.Run("Side2MissingRing", func(t *testing.T) {
		f := tgFixture(t, []string{"com.zerg.core.plist"})
		df := filepath.Join(f[0], "doctor.json")
		tgWrite(t, df, tgDoctor(1))
		rc, out, _ := tgRun([]string{"--root", f[0], "--decl", f[1], "--ps", f[2], "--launchctl", f[3],
			"--home", f[4], "--doctor", df})
		if rc != 0 {
			t.Fatalf("rc=%d want 0（报差不驱动退码）\n%s", rc, out)
		}
		if !strings.Contains(out, "声明件不在盘") {
			t.Fatalf("侧② 缺环未点名:\n%s", out)
		}
	})

	t.Run("EmptySpinBlocked", func(t *testing.T) {
		f := tgFixture(t, []string{"com.zerg.core.plist", "com.zerg.agent-Mr2109.plist"})
		pe := filepath.Join(f[0], "ps-empty.txt")
		tgWrite(t, pe, "    1     0  00:00 /sbin/launchd\n")
		df := filepath.Join(f[0], "doctor.json")
		tgWrite(t, df, tgDoctor(1))
		rc, out, _ := tgRun([]string{"--root", f[0], "--decl", f[1], "--ps", pe, "--launchctl", f[3],
			"--home", f[4], "--doctor", df})
		if rc != 2 {
			t.Fatalf("rc=%d want 2（空转不许当绿）\n%s", rc, out)
		}
		if !strings.Contains(out, "不给结论") {
			t.Fatalf("缺不给结论正文:\n%s", out)
		}
	})

	t.Run("MissingDeclBlocked", func(t *testing.T) {
		f := tgFixture(t, []string{"com.zerg.core.plist"})
		df := filepath.Join(f[0], "doctor.json")
		tgWrite(t, df, tgDoctor(1))
		rc, _, _ := tgRun([]string{"--root", f[0], "--decl", filepath.Join(f[0], "无.tsv"), "--ps", f[2],
			"--launchctl", f[3], "--home", f[4], "--doctor", df})
		if rc != 2 {
			t.Fatalf("rc=%d want 2（缺件 ⇒ 不给结论）", rc)
		}
	})

	t.Run("LaunchctlUnavailableBlocked", func(t *testing.T) {
		f := tgFixture(t, []string{"com.zerg.core.plist"})
		df := filepath.Join(f[0], "doctor.json")
		tgWrite(t, df, tgDoctor(1))
		rc, _, _ := tgRun([]string{"--root", f[0], "--decl", f[1], "--ps", f[2],
			"--launchctl", filepath.Join(f[0], "没有.txt"), "--home", f[4], "--doctor", df})
		if rc != 2 {
			t.Fatalf("rc=%d want 2（launchctl 取不到 ⇒ 不当绿）", rc)
		}
	})

	t.Run("WriteFlagRejected", func(t *testing.T) {
		for _, flag := range []string{"--reap", "--clean", "--fix", "commit", "--stop", "--kill", "--apply"} {
			rc, out, _ := tgRun([]string{flag})
			if rc != 2 {
				t.Fatalf("写旗标 %s rc=%d want 2", flag, rc)
			}
			if !strings.Contains(out, "写旗标") {
				t.Fatalf("写旗标 %s 缺拒收正文:\n%s", flag, out)
			}
		}
	})

	t.Run("JsonShape", func(t *testing.T) {
		f := tgFixture(t, []string{"com.zerg.core.plist", "com.zerg.agent-Mr2109.plist"})
		df := filepath.Join(f[0], "doctor.json")
		tgWrite(t, df, tgDoctor(1))
		rc, out, _ := tgRun([]string{"--root", f[0], "--decl", f[1], "--ps", f[2], "--launchctl", f[3],
			"--home", f[4], "--doctor", df, "--json"})
		if rc != 0 {
			t.Fatalf("rc=%d want 0", rc)
		}
		if !strings.HasPrefix(out, `{"schema": "zerg/v1", "kind": "GhostDeclDiff", "items": [`) {
			t.Fatalf("机器面形状不符:\n%s", out)
		}
		if !strings.Contains(out, `"warnings": []`) || !strings.Contains(out, `"truncated": false`) {
			t.Fatalf("空集合/布尔键面不符:\n%s", out)
		}
	})

	t.Run("SelfTestRecycles", func(t *testing.T) {
		rc, out, _ := tgRun([]string{"--self-test"})
		if rc != 0 {
			t.Fatalf("rc=%d want 0\n%s", rc, out)
		}
		if !strings.Contains(out, "自检结论: 全过") {
			t.Fatalf("自检未全过:\n%s", out)
		}
	})

	t.Run("SelfTestTwoStateByteEqual", func(t *testing.T) {
		// ★ 两态逐字同（本单：断「Go 半读 py 源」依赖 —— 源码级牙 / 自检 ④b④c / 只读指纹
		// 三面数据来源改**内嵌快照** `gddSelfPySource`）：把本件 py 真身从**仓外夹具**移开，
		// 自检输出必须与「在盘」态**逐字节同**、rc=0/0，且 ④b/④c 两格照出（✗ 不再有「退役态」分叉）。
		// ★ 回归牙：把 ④b/④c 改回读盘上 py 件（或删掉内嵌快照）⇒ 本格 FAIL 带行号。
		fA := tgFixture(t, []string{"com.zerg.core.plist"})
		var bufA bytes.Buffer
		rcA := gddSelfTestRoot(&bufA, fA[0])
		fB := tgFixture(t, []string{"com.zerg.core.plist"})
		if err := os.Remove(filepath.Join(fB[0], "scripts", "gates", "check-ghost-decl-diff.py")); err != nil {
			t.Fatalf("移开夹具 py 真身失败：%v", err)
		}
		var bufB bytes.Buffer
		rcB := gddSelfTestRoot(&bufB, fB[0])
		if rcA != 0 || rcB != 0 {
			t.Fatalf("自检 rc 在盘=%d 移出=%d want 0/0\n--- 在盘 ---\n%s\n--- 移出 ---\n%s", rcA, rcB, bufA.String(), bufB.String())
		}
		if bufA.String() != bufB.String() {
			t.Fatalf("两态自检输出不逐字同（本单回归牙）：\n--- 在盘 ---\n%s\n--- 移出 ---\n%s", bufA.String(), bufB.String())
		}
		if !strings.Contains(bufA.String(), "④b") || !strings.Contains(bufA.String(), "④c") {
			t.Fatalf("源码级牙两条正例未出（应走内嵌快照）：\n%s", bufA.String())
		}
		if strings.Contains(bufA.String(), "退役态") {
			t.Fatalf("旧「退役态」分叉仍在（应已由内嵌快照统一）：\n%s", bufA.String())
		}
	})

	t.Run("BogusFlagUsageError", func(t *testing.T) {
		rc, _, serr := tgRun([]string{"--bogus"})
		if rc != 2 {
			t.Fatalf("rc=%d want 2", rc)
		}
		if !strings.Contains(serr, "unrecognized arguments: --bogus") {
			t.Fatalf("用法错正文不符:\n%s", serr)
		}
	})
}

// TestGddSiblingGoHalfRetiredFallback —— G17（2026-10-07 退役批）：门⑧ py 半
// `check-service-declaration.py` 退役（不在盘）⇒ gddLoadSibling 改查其 **Go 半**
// `core/cmd/zerg/check_service_declaration.go` 的同源口径符号（✗ 不因缺 py 而 rc=2 fail-closed）。
// ★ 成对负控：Go 半也不在盘 / 缺符号 ⇒ 仍「不给结论」（✗ 不静默放行）。
// ★ 哑化负控：抽掉 gddLoadSibling 里 `if why := gddSiblingGoHalfOK(root); why == ""` 那段 ⇒ 本用例 FAIL（带行号）。
func TestGddSiblingGoHalfRetiredFallback(t *testing.T) {
	root := t.TempDir()
	// 前置：无 py、无 Go 半 ⇒ 该报「不给结论」。
	if why := gddLoadSibling(root, ""); why == "" {
		t.Fatal("前置不成立：无 py 又无 Go 半竟通过")
	}
	// 放 Go 半（含四个同源口径符号）⇒ 退役态通过。
	goHalf := filepath.Join(root, "core", "cmd", "zerg", "check_service_declaration.go")
	if err := os.MkdirAll(filepath.Dir(goHalf), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "package main\n\nfunc csdParseDecl() {}\n\nvar csdProcHints = []string{}\n\nfunc csdReadLines() {}\n\nfunc csdLabelSetDiff() {}\n"
	if err := os.WriteFile(goHalf, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if why := gddLoadSibling(root, ""); why != "" {
		t.Fatalf("退役态（Go 半在盘）该通过，却：%q", why)
	}
	// 负控：Go 半缺一个符号 ⇒ 仍不给结论。
	if err := os.WriteFile(goHalf, []byte("package main\n\nfunc csdParseDecl() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if why := gddLoadSibling(root, ""); why == "" {
		t.Fatal("负控：Go 半缺符号该仍不给结论")
	}
}
