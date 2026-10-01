// audit_go_logs_test.go —— 判据件 `scripts/gates/audit_go_logs.py` 的 Go 真搬
// （`core/cmd/zerg/audit_go_logs.go`）的**成对负控**测试（✗ 禁恒绿）。
//
// 覆盖（成对：正控 / 负控 / 边界各一格）：
//
//	SelfTestRc0_AndSelfCleans   · `--self-test` ⇒ rc=0 且**临时目录自回收 delta=0**
//	NoCoreRc2_StdoutTwoLines    · `--root` 指向无 `core/` 的树 ⇒ rc=2 + **stdout 两行**（py 原样）
//	UnknownFlagRc2              · 未知旗标 ⇒ rc=2 + argparse「usage + error」正文
//	BatchMissingValueRc2        · `--batch` 缺值 ⇒ rc=2 + 「expected one argument」
//	HelpRc0                     · `-h` / `--help` ⇒ rc=0 + 帮助正文逐字
//	JSONRefusedRc2              · `--json` / `--json=…` ⇒ rc=2 + 三行 stderr（前置拒收 · 先于 argparse）
//	ListOnFixture_ExactStdout   · 夹具 + `--list` ⇒ stdout **逐字**
//	BatchFilter_ZeroAndOne      · `--batch` 有的批 / 无的批（`明细（0 条）`）
//	CleanFixture_ZeroHits       · 干净夹具 ⇒ 0 条 + 空 Top
//	TraversalOrder_FilesFirst   · ★ 遍历铁律：同层**全部文件在前**、子目录按 **readdir 原序**下钻
//	UnreadableRc1_Traceback     · 读不了的件 ⇒ rc=1 + py C 类 traceback 正文（三帧 + PermissionError）
//	NonUTF8Rc1                  · 非 UTF-8 件 ⇒ rc=1（stderr 正文面未覆盖 · 只钉 rc/stdout）
//	RealRepo_SummaryRc0         · 真仓现读 ⇒ rc=0 且首行形如 `=== Go 含中文…：N 条 ===`
//	PyJoinSemantics             · 坐实 `aglPyJoin` 三语义（`..` / 后段绝对丢前段 / 空段 `join('a','')=='a/'`）
//	PyNormpath_LeadingSlash / PyRelpath_DotDot · 词法面
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func aglT(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var o, e bytes.Buffer
	rc := auditGoLogsGateMain(&o, &e, args)
	return rc, o.String(), e.String()
}

func aglW(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("建目录失败: %v", err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("写夹具失败: %v", err)
	}
	return p
}

// aglLogLine —— 夹具里的一行「含中文的日志调用」（真实中文：本件是 `_test.go` ⇒ 不进自门扫描面）。
func aglLogLine(batchKw, lit string) string {
	return "func f() { log.Println(\"" + batchKw + "：" + lit + "\") }\n"
}

func TestAuditGoLogsGatePort(t *testing.T) {
	t.Run("SelfTestRc0_AndSelfCleans", func(t *testing.T) {
		before, _ := filepath.Glob(filepath.Join(os.TempDir(), "agl-selftest-*"))
		rc, out, errs := aglT(t, "--self-test")
		if rc != 0 {
			t.Fatalf("rc=%d，期望 0（stderr=%q）", rc, errs)
		}
		if !strings.Contains(out, "⇒ 自检 全过（rc=0）") || !strings.Contains(out, "自证用例：9 条 · 断言条数 9") {
			t.Fatalf("自检 stdout 未见过关行：%q", out)
		}
		after, _ := filepath.Glob(filepath.Join(os.TempDir(), "agl-selftest-*"))
		if len(after) != len(before) {
			t.Fatalf("自检夹具未回收（delta≠0）：before=%d after=%d", len(before), len(after))
		}
	})

	t.Run("NoCoreRc2_StdoutTwoLines", func(t *testing.T) {
		d := t.TempDir()
		rc, out, errs := aglT(t, "--root", d)
		if rc != 2 {
			t.Fatalf("rc=%d，期望 2", rc)
		}
		if errs != "" {
			t.Fatalf("stderr 非空：%q", errs)
		}
		want := "⛔ 不给结论（rc=2）：扫描根不在盘：" + d + "/core（真仓缺 `core/`）\n" +
			"   「空转」≠「干净」：真 0 条与本态在 rc ＋ 输出上**分家**（缺口 `GAP-20260926-130`）\n"
		if out != want {
			t.Fatalf("stdout 不符：\n got=%q\nwant=%q", out, want)
		}
	})

	t.Run("UnknownFlagRc2", func(t *testing.T) {
		rc, out, errs := aglT(t, "--foo", "bar", "--list")
		if rc != 2 || out != "" {
			t.Fatalf("rc=%d out=%q，期望 rc=2 / stdout 空", rc, out)
		}
		want := "usage: audit_go_logs.py [-h] [--list] [--batch BATCH] [--self-test]\n" +
			"audit_go_logs.py: error: unrecognized arguments: --foo bar\n"
		if errs != want {
			t.Fatalf("stderr 不符：\n got=%q\nwant=%q", errs, want)
		}
	})

	t.Run("BatchMissingValueRc2", func(t *testing.T) {
		rc, _, errs := aglT(t, "--batch")
		if rc != 2 || !strings.HasSuffix(errs, "audit_go_logs.py: error: argument --batch: expected one argument\n") {
			t.Fatalf("rc=%d err=%q", rc, errs)
		}
	})

	t.Run("HelpRc0", func(t *testing.T) {
		for _, fl := range []string{"-h", "--help"} {
			rc, out, errs := aglT(t, fl)
			if rc != 0 || errs != "" || !strings.HasPrefix(out, "usage: audit_go_logs.py [-h] [--list] [--batch BATCH] [--self-test]\n") {
				t.Fatalf("%s: rc=%d err=%q out=%q", fl, rc, errs, out)
			}
			if out != aglHelpText {
				t.Fatalf("%s: 帮助正文与常量不符", fl)
			}
		}
	})

	t.Run("JSONRefusedRc2", func(t *testing.T) {
		want := "⛔ 本门无机器读面：`--json=abc` 在本门**未实现**（缺口 Q-176）\n" +
			"   字段表：无（本门只出人读面）\n" +
			"⇒ 用法错（rc=2）：**机器面缺 ≠ 零命中** —— 两态不许同形\n"
		rc, out, errs := aglT(t, "--json=abc")
		if rc != 2 || out != "" || errs != want {
			t.Fatalf("rc=%d out=%q err=%q", rc, out, errs)
		}
		rc, _, _ = aglT(t, "--self-test", "--json")
		if rc != 2 {
			t.Fatalf("--json 先于 --self-test 判：rc=%d，期望 2", rc)
		}
	})

	t.Run("CleanFixture_ZeroHits", func(t *testing.T) {
		d := t.TempDir()
		aglW(t, d, "core/clean.go", "package clean\n// 启动：注释里的中文不算\nvar s = \"x\"\nfunc p() { log.Printf(\"plain ascii\") }\n")
		rc, out, _ := aglT(t, "--root", d)
		if rc != 0 {
			t.Fatalf("rc=%d，期望 0", rc)
		}
		if !strings.Contains(out, "=== Go 含中文的日志/横幅语句：0 条 ===") {
			t.Fatalf("未报 0 条：%q", out)
		}
		if !strings.Contains(out, "文件 Top 15：\n\n") && !strings.HasSuffix(out, "文件 Top 15：\n") {
			t.Fatalf("空 Top 形态不符：%q", out)
		}
	})

	t.Run("ListOnFixture_ExactStdout", func(t *testing.T) {
		d := t.TempDir()
		aglW(t, d, "core/app.go", "package core\n"+aglLogLine("启动", "监听端口失败"))
		rc, out, _ := aglT(t, "--root", d, "--list")
		want := "=== Go 含中文的日志/横幅语句：1 条 ===\n" +
			"\n按优先批（设计稿 §4.4 顺序）：\n" +
			"  启动             1\n  认证             0\n  模型加载           0\n  任务生命周期         0\n  熔断             0\n  其他/内部诊断        0\n" +
			"\n文件 Top 15：\n  core/app.go                                          1\n" +
			"\n=== 明细（1 条）===\n  [" + "启动" + "] core/app.go:2 func f() { log.Println(\"" + "启动：监听端口失败" + "\") }\n"
		if rc != 0 || out != want {
			t.Fatalf("stdout 不符：\n got=%q\nwant=%q", out, want)
		}
	})

	t.Run("BatchFilter_ZeroAndOne", func(t *testing.T) {
		d := t.TempDir()
		aglW(t, d, "core/app.go", "package core\n"+aglLogLine("熔断", "冷却中"))
		rc, out, _ := aglT(t, "--root", d, "--batch", "熔断")
		if rc != 0 || !strings.Contains(out, "=== 明细（1 条）===\n  [熔断] core/app.go:2 ") {
			t.Fatalf("命中批 rc=%d out=%q", rc, out)
		}
		rc, out, _ = aglT(t, "--root", d, "--batch", "启动")
		if rc != 0 || !strings.Contains(out, "=== 明细（0 条）===\n") {
			t.Fatalf("零命中批 rc=%d out=%q", rc, out)
		}
	})

	t.Run("TraversalOrder_FilesFirst", func(t *testing.T) {
		d := t.TempDir()
		// 三个子目录 + 两个同层件；子目录**夹在** readdir 原序中间（create 交错）⇒ 钉「先文件后下钻 + 原序」。
		aglW(t, d, "core/zz.go", "package core\n"+aglLogLine("启动", "a"))
		aglW(t, d, "core/d1/x.go", "package core\n"+aglLogLine("认证", "b"))
		aglW(t, d, "core/aa.go", "package core\n"+aglLogLine("熔断", "c"))
		aglW(t, d, "core/d2/x.go", "package core\n"+aglLogLine("模型加载", "d"))
		aglW(t, d, "core/d0/x.go", "package core\n"+aglLogLine("任务", "派单"))
		rc, out, _ := aglT(t, "--root", d, "--list")
		if rc != 0 {
			t.Fatalf("rc=%d", rc)
		}
		// ① 同层全部文件（sorted）在前
		ia, iz := strings.Index(out, "core/aa.go:2"), strings.Index(out, "core/zz.go:2")
		if ia < 0 || iz < 0 || ia > iz {
			t.Fatalf("同层文件序不符（应 aa.go 在 zz.go 前）：%q", out)
		}
		// ② 子目录行**一律**在同层文件行之后
		for _, sub := range []string{"core/d0/x.go:2", "core/d1/x.go:2", "core/d2/x.go:2"} {
			if strings.Index(out, sub) < iz {
				t.Fatalf("子目录 %s 早于同层文件（早钻缺陷）：%q", sub, out)
			}
		}
		// ③ 子目录先后 = **readdir 原序**（✗ 非字典序）
		fh, err := os.Open(filepath.Join(d, "core"))
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		ents, _ := fh.ReadDir(-1)
		fh.Close()
		var rawDirs, sortedDirs []string
		for _, e := range ents {
			if fi, err := os.Stat(filepath.Join(d, "core", e.Name())); err == nil && fi.IsDir() {
				rawDirs = append(rawDirs, e.Name())
			}
		}
		sortedDirs = append(sortedDirs, rawDirs...)
		for i := range sortedDirs {
			for j := i + 1; j < len(sortedDirs); j++ {
				if sortedDirs[j] < sortedDirs[i] {
					sortedDirs[i], sortedDirs[j] = sortedDirs[j], sortedDirs[i]
				}
			}
		}
		if len(rawDirs) < 2 {
			t.Skipf("夹具只造出 %d 个子目录，跳过早钻/原序断言", len(rawDirs))
		}
		prev := -1
		for _, dn := range rawDirs {
			pos := strings.Index(out, "core/"+dn+"/x.go:2")
			if pos < 0 {
				t.Fatalf("子目录 %s 的行缺失：%q", dn, out)
			}
			if pos < prev {
				t.Fatalf("子目录下钻序 ≠ readdir 原序（应 %v）：%q", rawDirs, out)
			}
			prev = pos
		}
		// 若原序恰与字典序相同，则本格对「排序实现」不敏感 ⇒ 明说
		same := true
		for i := range rawDirs {
			if rawDirs[i] != sortedDirs[i] {
				same = false
			}
		}
		if same {
			t.Logf("注意：本夹具 readdir 原序恰等于字典序（%v）⇒ 该格对排序实现不敏感", rawDirs)
		}
	})

	t.Run("UnreadableRc1_Traceback", func(t *testing.T) {
		d := t.TempDir()
		aglW(t, d, "core/a.go", "package core\n"+aglLogLine("认证", "令牌过期"))
		bad := aglW(t, d, "core/b.go", "package core\n"+aglLogLine("熔断", "冷却中"))
		if err := os.Chmod(bad, 0o000); err != nil {
			t.Skipf("chmod 不支持：%v", err)
		}
		defer os.Chmod(bad, 0o644)
		if os.Geteuid() == 0 {
			t.Skip("root 不受 000 权限限制")
		}
		rc, out, errs := aglT(t, "--root", d)
		if rc != 1 || out != "" {
			t.Fatalf("rc=%d out=%q，期望 rc=1 / stdout 空", rc, out)
		}
		if !strings.Contains(errs, "Traceback (most recent call last):") ||
			!strings.Contains(errs, "line 65, in scan") ||
			!strings.HasSuffix(errs, "PermissionError: [Errno 13] Permission denied: '"+bad+"'\n") {
			t.Fatalf("traceback 正文不符：%q", errs)
		}
	})

	t.Run("NonUTF8Rc1", func(t *testing.T) {
		d := t.TempDir()
		aglW(t, d, "core/a.go", "package core\n"+aglLogLine("认证", "令牌过期"))
		p := filepath.Join(d, "core", "bad.go")
		if err := os.WriteFile(p, []byte("package core\nvar x = \"\xff\xfe\"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		rc, out, _ := aglT(t, "--root", d)
		if rc != 1 || out != "" {
			t.Fatalf("rc=%d out=%q，期望 rc=1 / stdout 空", rc, out)
		}
	})

	t.Run("RealRepo_SummaryRc0", func(t *testing.T) {
		root := aglPyAbspath(aglDefaultRepoRoot())
		if !aglStatIsDir(aglPyJoin(root, "core")) {
			t.Skipf("未定位到真仓（cwd=%s）", root)
		}
		rc, out, _ := aglT(t, "--root", root)
		if rc != 0 {
			t.Fatalf("rc=%d，期望 0", rc)
		}
		if !strings.Contains(out, "=== Go 含中文的日志/横幅语句：") || !strings.Contains(out, " 条 ===") {
			t.Fatalf("首行形态不符：%q", out)
		}
	})
}

func TestAuditGoLogsLexical(t *testing.T) {
	t.Run("PyJoinSemantics", func(t *testing.T) {
		if got := aglPyJoin("a", "b", "c"); got != "a/b/c" {
			t.Fatalf("普通: %q", got)
		}
		if got := aglPyJoin("/x", "..", "y"); got != "/x/../y" {
			t.Fatalf("join **不规范化**（吃 `..` 字面）：%q", got)
		}
		if got := aglPyJoin("/x", "/abs/y"); got != "/abs/y" {
			t.Fatalf("后段绝对 ⇒ **丢前段**：%q", got)
		}
		if got := aglPyJoin("a", ""); got != "a/" {
			t.Fatalf("空段 ⇒ `join('a','')=='a/'`：%q", got)
		}
		if got := aglPyJoin("", "b"); got != "b" {
			t.Fatalf("空前段：%q", got)
		}
	})

	t.Run("PyNormpath_LeadingSlash", func(t *testing.T) {
		if got := aglPyNormpath("//a/b/../c"); got != "//a/c" {
			t.Fatalf("双前导斜杠**保留**：%q", got)
		}
		if got := aglPyNormpath("///a/b"); got != "/a/b" {
			t.Fatalf("三斜杠折一：%q", got)
		}
		if got := aglPyNormpath("a/./b/../.."); got != "." {
			t.Fatalf("相对回溯到本层 ⇒ `.`：%q", got)
		}
		if got := aglPyNormpath("a/b/../../c"); got != "c" {
			t.Fatalf("回溯后余段：%q", got)
		}
		if got := aglPyNormpath("a/../../b"); got != "../b" {
			t.Fatalf("相对回溯越界保留 `..`：%q", got)
		}
		if got := aglPyNormpath(""); got != "." {
			t.Fatalf("空串 ⇒ `.`：%q", got)
		}
	})

	t.Run("PyRelpath_DotDot", func(t *testing.T) {
		if got := aglPyRelpath("/a/b/c/d.go", "/a/b"); got != "c/d.go" {
			t.Fatalf("子路径：%q", got)
		}
		if got := aglPyRelpath("/a/b", "/a/b"); got != "." {
			t.Fatalf("同路径 ⇒ `.`：%q", got)
		}
		if got := aglPyRelpath("/a/x", "/a/b/c"); got != "../../x" {
			t.Fatalf("回溯：%q", got)
		}
	})

	t.Run("PyStrip_PyWhitespace", func(t *testing.T) {
		if got := aglPyStrip("\x1c\x1f  x  \u0085"); got != "x" {
			t.Fatalf("py 空白集合（含 \\x1c-\\x1f / \\x85）：%q", got)
		}
		if got := aglPyStrip("\u3000y\u3000"); got != "y" {
			t.Fatalf("全角空格：%q", got)
		}
	})

	t.Run("TruncRunes_NotBytes", func(t *testing.T) {
		in := strings.Repeat("中", 200)
		if got := aglRunes(in, 150); len([]rune(got)) != 150 {
			t.Fatalf("应按码点切 150：%d", len([]rune(got)))
		}
	})
}
