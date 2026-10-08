// check_password_argv_test.go —— `core/cmd/zerg/check_password_argv.go`（M2 真搬件）的成对负控。
//
// 目的：给 `checkPasswordArgvGateMain` 一道**带牙**的回归 —— 哑化任一条判据（如「裸口令长选项 + 实值 ⇒ 判红」
// 或「短选项喂口令只对**吃口令工具**生效」或「`--self-test` 的退码钳」）⇒ 必有一格 FAIL。合成件全在
// `t.TempDir()`（**仓外 · 不碰真目标**）。
//
// ★ 本件的夹具正文一律**拼串形态**落码（同已落仓 `check_token_argv_test.go` 的做法）⇒ 测试件本身
// **零**被本门扫中的形态（本门会把 `core/cmd/zerg/*.go` 读成**登记档**面）。
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// cpwTEntry —— 进程内取 (rc, stdout, stderr)。
func cpwTEntry(argv ...string) (int, string, string) {
	var so, se strings.Builder
	rc := checkPasswordArgvGateMain(&so, &se, argv)
	return rc, so.String(), se.String()
}

// cpwTFix —— 造一个空夹具目录（仓外 · `t.TempDir()`）。
func cpwTFix(t *testing.T) string {
	t.Helper()
	d := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	return d
}

// cpwTWrite —— 在夹具里写一件。
func cpwTWrite(t *testing.T, p, body string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestCheckPasswordArgvGatePort —— 正控 + 逐型负控 + 阻塞档 + 自指面（带牙：哑化任一判据 ⇒ 必 FAIL）。
func TestCheckPasswordArgvGatePort(t *testing.T) {
	t.Run("正控 口令档正门 + 单元文件正门 ⇒ rc=0", func(t *testing.T) {
		d := cpwTFix(t)
		cpwTWrite(t, filepath.Join(d, "a.sh"),
			"#!/usr/bin/env bash\n"+cpwDocOpt+") PW=\"$2\"\n")
		cpwTWrite(t, filepath.Join(d, "x.service"),
			"[Service]\n"+cpwPwEnvFile+"-/run/x.env\n")
		rc, out, _ := cpwTEntry("--root", d)
		if rc != 0 || !strings.Contains(out, "零处") || !strings.Contains(out, "判红面件 2 个") {
			t.Fatalf("正控失败：rc=%d out=%q", rc, out)
		}
	})

	t.Run("负控① 裸口令长选项 + 实值 ⇒ rc=1 [PW-LITERAL-ARGV]", func(t *testing.T) {
		d := cpwTFix(t)
		cpwTWrite(t, filepath.Join(d, "a.sh"),
			"#!/usr/bin/env bash\ndeploy.sh --"+"pass"+"word 'S3cr3t-Pw!'\n")
		rc, out, _ := cpwTEntry("--root", d)
		if rc != 1 || !strings.Contains(out, "a.sh:2  [PW-LITERAL-ARGV]") {
			t.Fatalf("负控①失败：rc=%d out=%q", rc, out)
		}
	})

	t.Run("负控① 变量值照旧判红（命令行上照样展开进 argv）", func(t *testing.T) {
		d := cpwTFix(t)
		cpwTWrite(t, filepath.Join(d, "a.sh"),
			"#!/usr/bin/env bash\ndeploy.sh --"+"pass"+"word=\"$MINI_PW\"\n")
		rc, out, _ := cpwTEntry("--root", d)
		if rc != 1 || !strings.Contains(out, "[PW-LITERAL-ARGV]") {
			t.Fatalf("负控①b失败：rc=%d out=%q", rc, out)
		}
	})

	t.Run("负控② 短选项喂口令给吃口令工具 ⇒ rc=1 [PW-SHORTOPT-ARGV]", func(t *testing.T) {
		d := cpwTFix(t)
		cpwTWrite(t, filepath.Join(d, "a.sh"),
			"#!/usr/bin/env bash\nssh"+"pass -p 'Passw0rd!' ssh root@<cluster-ip> true\n")
		rc, out, _ := cpwTEntry("--root", d)
		if rc != 1 || !strings.Contains(out, "a.sh:2  [PW-SHORTOPT-ARGV]") {
			t.Fatalf("负控②失败：rc=%d out=%q", rc, out)
		}
	})

	t.Run("负控② 工具名挡一道：非吃口令工具的 -p 不判红", func(t *testing.T) {
		d := cpwTFix(t)
		// `mkdir -p <目录>` 一类同形选项满地都是 ⇒ 首词不在工具表 ⇒ **不判红**。
		cpwTWrite(t, filepath.Join(d, "a.sh"), "#!/usr/bin/env bash\nmkdir -p /tmp/zz\n")
		rc, out, _ := cpwTEntry("--root", d)
		if rc != 0 || strings.Contains(out, "PW-SHORTOPT-ARGV") {
			t.Fatalf("负控②b失败：rc=%d out=%q", rc, out)
		}
	})

	t.Run("登记档 端口/路径形态的 -p（非口令）⇒ rc=0", func(t *testing.T) {
		d := cpwTFix(t)
		cpwTWrite(t, filepath.Join(d, "a.sh"), "#!/usr/bin/env bash\nss"+"h -p 2222 host\n")
		rc, out, _ := cpwTEntry("--root", d)
		if rc != 0 || !strings.Contains(out, "PW-MENTION") {
			t.Fatalf("登记档失败：rc=%d out=%q", rc, out)
		}
	})

	t.Run("负控③ 读了弃用环境变量但全文无警告 ⇒ rc=1 [ENV-NOWARN-ARGV]", func(t *testing.T) {
		d := cpwTFix(t)
		cpwTWrite(t, filepath.Join(d, "a.sh"),
			"#!/usr/bin/env bash\nMINI"+"_PASS=\"${"+"MINI"+"_PASS:-}\"\nscp x mini:/tmp/\n")
		rc, out, _ := cpwTEntry("--root", d)
		if rc != 1 || !strings.Contains(out, "a.sh:2  [ENV-NOWARN-ARGV]") {
			t.Fatalf("负控③失败：rc=%d out=%q", rc, out)
		}
	})

	t.Run("负控③b 有一次性警告（弃用 + 变量名 + >&2）⇒ 不判红", func(t *testing.T) {
		d := cpwTFix(t)
		cpwTWrite(t, filepath.Join(d, "a.sh"),
			"#!/usr/bin/env bash\nif true; then\n"+
				"  echo \"已弃用入口："+"MINI"+"_PASS 仍能跑\" >&2\n"+
				"  X="+"${"+"MINI"+"_PASS:-}\"\nfi\n")
		rc, _, _ := cpwTEntry("--root", d)
		if rc != 0 {
			t.Fatalf("负控③b失败：rc=%d", rc)
		}
	})

	t.Run("负控④ usage 仍在教弃用环境变量入口 ⇒ rc=1 [ENV-TAUGHT-USAGE]", func(t *testing.T) {
		d := cpwTFix(t)
		cpwTWrite(t, filepath.Join(d, "a.sh"),
			"#!/usr/bin/env bash\nusage() { echo \"export X"+"3_PASS=<口令> 后再跑\"; }\n")
		rc, out, _ := cpwTEntry("--root", d)
		if rc != 1 || !strings.Contains(out, "a.sh:2  [ENV-TAUGHT-USAGE]") {
			t.Fatalf("负控④失败：rc=%d out=%q", rc, out)
		}
	})

	t.Run("登记档 注释行同形 ⇒ 只登记不判红", func(t *testing.T) {
		d := cpwTFix(t)
		cpwTWrite(t, filepath.Join(d, "a.sh"),
			"#!/usr/bin/env bash\n# 旧式：--"+"pass"+"word <明文>\ntrue\n")
		rc, out, _ := cpwTEntry("--root", d)
		if rc != 0 || !strings.Contains(out, "注释行") {
			t.Fatalf("注释档失败：rc=%d out=%q", rc, out)
		}
	})

	t.Run("兼容臂 值指向已存在档 ⇒ 只登记不判红", func(t *testing.T) {
		d := cpwTFix(t)
		cpwTWrite(t, filepath.Join(d, "secret600"), "pw\n")
		cpwTWrite(t, filepath.Join(d, "a.sh"),
			"#!/usr/bin/env bash\ndeploy.sh --"+"pass"+"word ./secret600\n")
		rc, out, _ := cpwTEntry("--root", d)
		if rc != 0 || !strings.Contains(out, "PW-COMPAT-PASSFILE") {
			t.Fatalf("兼容臂失败：rc=%d out=%q", rc, out)
		}
	})

	t.Run("自指面 .go 件被读成登记档（不判红）", func(t *testing.T) {
		d := cpwTFix(t)
		// 假仓里的 .go 件带危险字面 ⇒ 后缀 ∉ 判红面 ⇒ 只进 registered、rc=0（另放一枚 .sh 撑起判红面）。
		cpwTWrite(t, filepath.Join(d, "a.sh"), "#!/usr/bin/env bash\ntrue\n")
		cpwTWrite(t, filepath.Join(d, "x.go"),
			"package main\nvar s = \""+cpwDocOpt+"\"\n")
		rc, out, _ := cpwTEntry("--root", d)
		if rc != 0 || !strings.Contains(out, "PASSFILE-SITE") {
			t.Fatalf("自指面失败：rc=%d out=%q", rc, out)
		}
	})

	t.Run("阻塞档 扫描根不存在 ⇒ rc=2", func(t *testing.T) {
		d := cpwTFix(t)
		rc, _, se := cpwTEntry("--root", filepath.Join(d, "nope"))
		if rc != 2 || !strings.Contains(se, "BLOCKED") {
			t.Fatalf("阻塞档失败：rc=%d stderr=%q", rc, se)
		}
	})

	t.Run("阻塞档 零脚本/单元件 ⇒ rc=2", func(t *testing.T) {
		d := cpwTFix(t)
		cpwTWrite(t, filepath.Join(d, "a.md"), "# 只有登记面件\n")
		rc, _, se := cpwTEntry("--root", d)
		if rc != 2 || !strings.Contains(se, "0 个脚本/单元件") {
			t.Fatalf("阻塞档b失败：rc=%d stderr=%q", rc, se)
		}
	})

	t.Run("阻塞档 B类读失败（chmod 000）⇒ rc=2 + BLOCKED + 点名 + 分母守恒", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root 无视 chmod 000 ⇒ 该格无鉴别力")
		}
		d := cpwTFix(t)
		cpwTWrite(t, filepath.Join(d, "ok.sh"), "#!/usr/bin/env bash\ntrue\n")
		locked := filepath.Join(d, "locked.sh")
		cpwTWrite(t, locked, "#!/usr/bin/env bash\ndeploy.sh --"+"pass"+"word 'S3cr3t-Pw!'\n")
		if err := os.Chmod(locked, 0o000); err != nil {
			t.Skipf("chmod 000 不支持：%v", err)
		}
		defer func() { _ = os.Chmod(locked, 0o644) }() // ★ 自回收：跑完恢复（同包体例）
		rc, out, se := cpwTEntry("--root", d)
		if rc != 2 {
			t.Fatalf("读失败应 rc=2（B 类真口子 ⇒ 不许静默当绿）：rc=%d out=%q stderr=%q", rc, out, se)
		}
		if !strings.Contains(se, "BLOCKED") || !strings.Contains(se, "有 1 件读不到") {
			t.Fatalf("读失败缺 BLOCKED 头 / 计数：stderr=%q", se)
		}
		if out != "" {
			t.Fatalf("读失败不应给结论（stdout 须空）：out=%q", out)
		}
		res, ok := cpwScan(d)
		if !ok || res == nil {
			t.Fatalf("扫描面取不到：ok=%v", ok)
		}
		if res.NFace != 2 {
			t.Fatalf("分母守恒破：判红面件应含读不到的件（want 2）got=%d", res.NFace)
		}
		if res.Unread != 1 {
			t.Fatalf("读失败件数应为 1：got=%d", res.Unread)
		}
		named := false
		for _, h := range res.Judged {
			if h.Rel == "locked.sh" && h.Tag == "PW-UNREADABLE" {
				named = true
			}
		}
		if !named {
			t.Fatalf("读不到的件未被逐件点名 [PW-UNREADABLE]：%+v", res.Judged)
		}
	})

	t.Run("阻塞档 --root 以 - 开头（拼错旗标）⇒ rc=2", func(t *testing.T) {
		rc, _, se := cpwTEntry("--root", "-oops")
		if rc != 2 || !strings.Contains(se, "防呆") {
			t.Fatalf("阻塞档c失败：rc=%d stderr=%q", rc, se)
		}
	})

	t.Run("--json 面 键表齐全（judged_sites 逐站）", func(t *testing.T) {
		d := cpwTFix(t)
		cpwTWrite(t, filepath.Join(d, "a.sh"),
			"#!/usr/bin/env bash\ndeploy.sh --"+"pass"+"word 'S3cr3t-Pw!'\n")
		rc, out, _ := cpwTEntry("--root", d, "--json")
		if rc != 1 || !strings.Contains(out, `"judged": 1`) ||
			!strings.Contains(out, `"shape": "PW-LITERAL-ARGV"`) ||
			!strings.Contains(out, `"exempt_pw": []`) {
			t.Fatalf("--json 失败：rc=%d out=%q", rc, out)
		}
	})

	t.Run("--self-test 全档跑得起（含退码钳与防空绿档）", func(t *testing.T) {
		_, out, _ := cpwTEntry("--self-test")
		if !strings.Contains(out, "[退码 0-1-2 钳]") || !strings.Contains(out, "[防空绿档]") ||
			!strings.Contains(out, "⇒ 自检") {
			t.Fatalf("--self-test 失败：out=%q", out)
		}
	})

	t.Run("未知旗标 ⇒ rc=2", func(t *testing.T) {
		rc, _, se := cpwTEntry("--nope")
		if rc != 2 || !strings.Contains(se, "未知旗标") {
			t.Fatalf("未知旗标失败：rc=%d stderr=%q", rc, se)
		}
	})
}

// TestCheckPasswordArgvSiblingRetiredPredicate —— 退役态牙（类① · 本件退件前置）：
// 兄弟门 py 缺席 且 其 Go 半在盘 ⇒ 判「已退役」；py 在盘 ⇒ 否；py 缺席且 Go 半也不在盘（真消失）⇒ 否
// （交叉档照旧判「断言不过」· fail-closed 保留）。
// ★ 哑化负控：把 cpwSiblingRetired 恒置 false ⇒ ①段当场 FAIL 并给出本行号。
func TestCheckPasswordArgvSiblingRetiredPredicate(t *testing.T) {
	mk := func(t *testing.T, withPy, withGo bool) string {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, "core", "cmd", "zerg"), 0o755); err != nil {
			t.Fatal(err)
		}
		cpwTWrite(t, filepath.Join(root, "core", "cmd", "zerg", "main.go"), "package main\n")
		if withGo {
			cpwTWrite(t, filepath.Join(root, "core", "cmd", "zerg", "check_token_argv.go"), "package main\n")
		}
		if withPy {
			if err := os.MkdirAll(filepath.Join(root, "scripts", "gates"), 0o755); err != nil {
				t.Fatal(err)
			}
			cpwTWrite(t, filepath.Join(root, "scripts", "gates", "check-token-argv.py"), "# stub\n")
		}
		return root
	}
	within := func(t *testing.T, root string, fn func()) {
		t.Helper()
		old, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chdir(root); err != nil {
			t.Fatal(err)
		}
		defer func() { _ = os.Chdir(old) }()
		fn()
	}

	t.Run("①兄弟门py在盘_未退役", func(t *testing.T) {
		within(t, mk(t, true, true), func() {
			if cpwSiblingRetired() {
				t.Fatalf("py 在盘不应判退役")
			}
		})
	})
	t.Run("②py缺席_Go半在盘_已退役", func(t *testing.T) {
		within(t, mk(t, false, true), func() {
			if !cpwSiblingRetired() {
				t.Fatalf("py 缺席 + Go 半在盘应判退役（否则 fail-closed）")
			}
		})
	})
	t.Run("③py缺席_Go半也不在盘_未退役_真消失", func(t *testing.T) {
		within(t, mk(t, false, false), func() {
			if cpwSiblingRetired() {
				t.Fatalf("双缺（真消失）不得判退役")
			}
		})
	})
}
