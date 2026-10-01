// check_no_symlink_farm_test.go —— 判据件 `scripts/gates/check-no-symlink-farm` `.py` 的 Go 真搬
// （`core/cmd/zerg/check_no_symlink_farm.go`）的**成对负控**测试（✗ 禁恒绿）。
//
// 覆盖（成对：正控 / 负控 / 边界各一格）：
//
//	SelfTestRc0_AndSelfCleans   · `--self-test` ⇒ rc=0 且**临时目录自回收 delta=0**（A 类实证 · 含 ④ 的自建符号链接）
//	UsageRc2_RootMissing        · `--root` 不存在 ⇒ rc=2 + stderr 正文
//	EmptySpinRc2                · 候选 0 件（空转）⇒ rc=2（**空转 ≠ 绿**）
//	UnknownArgRc2               · 未知旗标 ⇒ rc=2 + stderr 正文
//	A2CrossLink_Rc1             · 建链指回「真仓」+ 删链下 ⇒ rc=1 且判据 A2
//	A3RepoPrefix_Rc1            · 删真仓前缀变量下 ⇒ rc=1 且判据 A3
//	A1ReportOnly_Rc0            · 链接指回真仓但**无删除指向它** ⇒ rc=0 且只报告 A1
//	A1Question_Cmdsub_Strict    · 命令替换前缀 ⇒ 点名 A1?（rc=0）；`--strict-links` ⇒ rc=1
//	FarmB_Rc1_And_NoneRc0       · `--farm` 现读链接指回真仓 ⇒ rc=1；不给 `--farm` ⇒ rc=0 且「B 档跳过」
//	JSONShape_FarmNullVsString  · `--json` 的 `farm` 键：缺省 `null` ⟷ 给了则字符串；`items`/`reports` 空 = `[]`
//	UnreadableSilent_BClass     · 读不了的件（chmod 000）⇒ py 现状**静默跳过**（钉住 B 类真口子）
//	WalkFollowsSymlinkDirs      · ★ 铁律：符号链接**目录**按 py `entry.is_dir()`（跟随）归 dirnames、且不下钻
//	                               （✗ 若用 Go `DirEntry.IsDir()` 的 Lstat 语义则归 files ⇒ 判反）
//	TraversalOrder_SortedFiles  · 同目录文件序 = `sorted(filenames)`（码点序）· 跨目录 = scandir 原序
//	SymlinkEntryInsideFixtureOK · 夹具**内部**造符号链接条目 ⇒ 正常参与判据（标准③只禁夹具**路径**含链接）
//	PyJoinSemantics             · 坐实 `cnsfPyJoin` 三语义（`..` / 后段绝对 / 空段 `join('a','')=='a/'`）
//	RegexParity                 · 负后顾（`xln` 不命 / ` ln` 命）· Unicode 空白 `\s` · `\b` 词界 · 行尾注释剥离
//	JSONEscapesBackslash        · `why` 里的反斜杠/引号走**转义安全形态**（`\\` / `\"`）
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func cnsfT(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var o, e bytes.Buffer
	rc := checkNoSymlinkFarmGateMain(&o, &e, args)
	return rc, o.String(), e.String()
}

func cnsfWrite(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("写夹具失败: %v", err)
	}
	return p
}

func cnsfMkdir(t *testing.T, parts ...string) string {
	t.Helper()
	p := filepath.Join(parts...)
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatalf("建目录失败: %v", err)
	}
	return p
}

// TestNoSymlinkFarmGate —— 成对负控总入口（定向跑：`-run '^TestNoSymlinkFarmGate$'`）。
func TestNoSymlinkFarmGate(t *testing.T) {
	t.Run("SelfTestRc0_AndSelfCleans", func(t *testing.T) {
		tmpBefore, _ := filepath.Glob(filepath.Join(os.TempDir(), "zerg-nosymfarm-*"))
		rc, _, _ := cnsfT(t, "--self-test")
		if rc != 0 {
			t.Fatalf("rc=%d，期望 0", rc)
		}
		tmpAfter, _ := filepath.Glob(filepath.Join(os.TempDir(), "zerg-nosymfarm-*"))
		if len(tmpAfter) != len(tmpBefore) {
			t.Fatalf("自检夹具未回收：before=%d after=%d", len(tmpBefore), len(tmpAfter))
		}
	})

	t.Run("UsageRc2_RootMissing", func(t *testing.T) {
		rc, out, errs := cnsfT(t, "--root", filepath.Join(t.TempDir(), "no-such"), "--no-self-test")
		if rc != 2 {
			t.Fatalf("rc=%d，期望 2", rc)
		}
		if out != "" {
			t.Fatalf("stdout 非空：%q", out)
		}
		if !strings.HasPrefix(errs, "✗ 判不了：--root 不是目录 —— ") || !strings.HasSuffix(errs, "（rc=2）\n") {
			t.Fatalf("stderr 正文不符：%q", errs)
		}
	})

	t.Run("EmptySpinRc2", func(t *testing.T) {
		d := t.TempDir()
		rc, _, errs := cnsfT(t, "--root", d, "--no-self-test")
		if rc != 2 || !strings.Contains(errs, "✗ 空转：") {
			t.Fatalf("rc=%d err=%q", rc, errs)
		}
	})

	t.Run("UnknownArgRc2", func(t *testing.T) {
		rc, _, errs := cnsfT(t, "--no-such-flag", "--no-self-test")
		if rc != 2 || !strings.HasPrefix(errs, "✗ 用法错：未知参数 --no-such-flag") {
			t.Fatalf("rc=%d err=%q", rc, errs)
		}
	})

	t.Run("A2CrossLink_Rc1", func(t *testing.T) {
		base := t.TempDir()
		real := cnsfMkdir(t, base, "real")
		cnsfMkdir(t, real, "core", "cmd", "zerg", "testdata")
		cnsfWrite(t, filepath.Join(real, "core", "cmd", "zerg", "testdata"), "cli-matrix.json", "{}\n")
		d := cnsfMkdir(t, base, "bad")
		cnsfMkdir(t, d, "farm")
		cnsfWrite(t, d, "farm.sh",
			"REPO=\""+real+"\"\nFARM=\""+filepath.Join(d, "farm")+"\"\n"+
				"ln -s \"${REPO}/core/cmd/zerg/testdata\" \"$FARM/case-a/testdata\"\n"+
				"rm -f \"$FARM/case-a/testdata/cli-matrix.json\"\n")
		rc, _, _ := cnsfT(t, "--root", d, "--real-repo", real, "--no-self-test")
		if rc != 1 {
			t.Fatalf("rc=%d，期望 1（A2）", rc)
		}
	})

	t.Run("A3RepoPrefix_Rc1", func(t *testing.T) {
		base := t.TempDir()
		real := cnsfMkdir(t, base, "real")
		d := cnsfMkdir(t, base, "bad-var")
		cnsfWrite(t, d, "wipe.sh", "REPO=\""+real+"\"\nrm -rf \"$REPO/scripts/tmp\"\n")
		rc, _, _ := cnsfT(t, "--root", d, "--real-repo", real, "--no-self-test")
		if rc != 1 {
			t.Fatalf("rc=%d，期望 1（A3）", rc)
		}
	})

	t.Run("A1ReportOnly_Rc0", func(t *testing.T) {
		base := t.TempDir()
		real := cnsfMkdir(t, base, "real")
		d := cnsfMkdir(t, base, "link-only")
		cnsfWrite(t, d, "l.sh", "REPO=\""+real+"\"\nln -s \"${REPO}/x\" \"${REPO}/y\"\n")
		rc, out, _ := cnsfT(t, "--root", d, "--real-repo", real, "--no-self-test")
		if rc != 0 {
			t.Fatalf("rc=%d，期望 0（A1 只报告）", rc)
		}
		if !strings.Contains(out, "**只报告**") || !strings.Contains(out, "[A1]") {
			t.Fatalf("stdout 未见只报告 A1：%q", out)
		}
	})

	t.Run("A1Question_Cmdsub_Strict", func(t *testing.T) {
		base := t.TempDir()
		real := cnsfMkdir(t, base, "real")
		d := cnsfMkdir(t, base, "cmdsub")
		cnsfWrite(t, d, "f.sh",
			"ROOT=\"$(cd /tmp && pwd)\"\nKERNEL=\"$ROOT/x.sh\"\nln -sf \"$KERNEL\" /tmp/zz\n")
		rc, out, _ := cnsfT(t, "--root", d, "--real-repo", real, "--no-self-test")
		if rc != 0 || !strings.Contains(out, "[A1?]") {
			t.Fatalf("rc=%d out=%q，期望 rc=0 且点名 A1?", rc, out)
		}
		rc, _, _ = cnsfT(t, "--root", d, "--real-repo", real, "--no-self-test", "--strict-links")
		if rc != 1 {
			t.Fatalf("--strict-links rc=%d，期望 1", rc)
		}
	})

	t.Run("FarmB_Rc1_And_NoneRc0", func(t *testing.T) {
		base := t.TempDir()
		real := cnsfMkdir(t, base, "real")
		cnsfMkdir(t, real, "core", "cmd", "zerg", "testdata")
		d := cnsfMkdir(t, base, "root")
		cnsfWrite(t, d, "ok.sh", "echo hi\n")
		farm := cnsfMkdir(t, base, "farm")
		if err := os.Symlink(filepath.Join(real, "core", "cmd", "zerg", "testdata"), filepath.Join(farm, "ln")); err != nil {
			t.Fatalf("建符号链接失败: %v", err)
		}
		rc, _, _ := cnsfT(t, "--root", d, "--real-repo", real, "--farm", farm, "--no-self-test")
		if rc != 1 {
			t.Fatalf("--farm rc=%d，期望 1（B）", rc)
		}
		rc, out, _ := cnsfT(t, "--root", d, "--real-repo", real, "--no-self-test")
		if rc != 0 || !strings.Contains(out, "**未给 ⇒ B 档跳过**") {
			t.Fatalf("不带 --farm rc=%d out=%q", rc, out)
		}
	})

	t.Run("JSONShape_FarmNullVsString", func(t *testing.T) {
		base := t.TempDir()
		real := cnsfMkdir(t, base, "real")
		d := cnsfMkdir(t, base, "root")
		cnsfWrite(t, d, "ok.sh", "echo hi\n")
		_, out, _ := cnsfT(t, "--root", d, "--real-repo", real, "--json", "--no-self-test")
		if !strings.Contains(out, "\"farm\": null") || !strings.Contains(out, "\"items\": []") ||
			!strings.Contains(out, "\"reports\": []") {
			t.Fatalf("JSON 空形态不符：%q", out)
		}
		farm := cnsfMkdir(t, base, "farm")
		_, out2, _ := cnsfT(t, "--root", d, "--real-repo", real, "--farm", farm, "--json", "--no-self-test")
		if strings.Contains(out2, "\"farm\": null") || !strings.Contains(out2, "\"farm\": \""+farm+"\"") {
			t.Fatalf("JSON farm 字符串形态不符：%q", out2)
		}
	})

	t.Run("UnreadableSilent_BClass", func(t *testing.T) {
		d := t.TempDir()
		p := cnsfWrite(t, d, "secret.sh", "rm -rf /nope\n")
		if err := os.Chmod(p, 0o000); err != nil {
			t.Skipf("chmod 不支持：%v", err)
		}
		defer os.Chmod(p, 0o644)
		if _, err := os.ReadFile(p); err == nil {
			t.Skip("当前用户可读（多为 root）⇒ 无法钉住 B 类")
		}
		rc, out, _ := cnsfT(t, "--root", d, "--no-self-test")
		// py 现状：读不了的件**静默跳过**（既不进红也不进报告）⇒ 分母仍计 1 件 ⇒ rc=0。
		if rc != 0 || !strings.Contains(out, "候选 1 件") {
			t.Fatalf("rc=%d out=%q，期望 rc=0 且候选 1 件（B 类静默）", rc, out)
		}
		if strings.Contains(out, "[A3]") || strings.Contains(out, "[A2]") {
			t.Fatalf("读不了的件不应产出命中：%q", out)
		}
	})

	t.Run("WalkFollowsSymlinkDirs", func(t *testing.T) {
		// ★ 铁律：py `entry.is_dir()` 跟随链 ⇒ 指向目录的**符号链接**归 dirnames（且不下钻）；
		//   ✗ 若错用 Go `DirEntry.IsDir()`（Lstat）则归 filenames ⇒ 判反。这里直接坐实两态。
		base := t.TempDir()
		target := cnsfMkdir(t, base, "realdir")
		cnsfWrite(t, target, "inner.sh", "echo x\n")
		link := filepath.Join(base, "zdir-link")
		if err := os.Symlink(target, link); err != nil {
			t.Fatalf("建符号链接失败: %v", err)
		}
		// Lstat 语义（Go DirEntry.IsDir 的行为）：false
		if fi, err := os.Lstat(link); err != nil || fi.IsDir() {
			t.Fatalf("Lstat 判目录符号链接应为 false：err=%v isDir=%v", err, fi.IsDir())
		}
		// 跟随语义（py entry.is_dir 的行为）：true
		if !cnsfStatIsDir(link) {
			t.Fatal("cnsfStatIsDir 应跟随链判 true")
		}
		var dirsAtRoot []string
		visited := map[string]bool{}
		cnsfWalk(base, nil, func(dirpath string, dirs, files []string) {
			visited[dirpath] = true
			if dirpath == base {
				dirsAtRoot = append(dirsAtRoot, dirs...)
			}
		})
		found := false
		for _, d := range dirsAtRoot {
			if d == "zdir-link" {
				found = true
			}
		}
		if !found {
			t.Fatalf("符号链接目录未归 dirnames（判反了）：dirs=%v", dirsAtRoot)
		}
		if visited[filepath.Join(base, "zdir-link")] {
			t.Fatal("符号链接目录不应被下钻（followlinks=False）")
		}
		if !visited[target] {
			t.Fatal("真实目录应被下钻")
		}
	})

	t.Run("TraversalOrder_SortedFiles", func(t *testing.T) {
		base := t.TempDir()
		real := cnsfMkdir(t, base, "real")
		d := cnsfMkdir(t, base, "root")
		cnsfWrite(t, d, "z.sh", "REPO=\""+real+"\"\nln -s \"${REPO}/a\" \"${REPO}/b\"\n")
		cnsfWrite(t, d, "a.sh", "REPO=\""+real+"\"\nln -s \"${REPO}/a\" \"${REPO}/b\"\n")
		_, out, _ := cnsfT(t, "--root", d, "--real-repo", real, "--no-self-test")
		ia := strings.Index(out, "~ a.sh:")
		iz := strings.Index(out, "~ z.sh:")
		if ia < 0 || iz < 0 || ia > iz {
			t.Fatalf("同目录文件序应 = 码点序（a.sh 先于 z.sh）：%q", out)
		}
	})

	t.Run("SymlinkEntryInsideFixtureOK", func(t *testing.T) {
		// ★ 标准③：禁的是**夹具所在路径**含符号链接；夹具**内部**按判据需要可造符号链接条目。
		base := t.TempDir()
		real := cnsfMkdir(t, base, "real")
		cnsfMkdir(t, real, "td")
		farm := cnsfMkdir(t, base, "farm")
		cnsfWrite(t, farm, "ok.sh", "echo hi\n")
		if err := os.Symlink(filepath.Join(real, "td"), filepath.Join(farm, "ln")); err != nil {
			t.Fatalf("建符号链接失败: %v", err)
		}
		rc, _, _ := cnsfT(t, "--root", farm, "--real-repo", real, "--farm", farm, "--no-self-test")
		if rc != 1 {
			t.Fatalf("夹具内部符号链接应正常入判据：rc=%d", rc)
		}
	})

	t.Run("PyJoinSemantics", func(t *testing.T) {
		if got := cnsfPyJoin("a", ".."); got != "a/.." {
			t.Fatalf("吃 .. 语义错：%q", got)
		}
		if got := cnsfPyJoin("repo", "/abs"); got != "/abs" {
			t.Fatalf("后段绝对丢前段语义错：%q", got)
		}
		if got := cnsfPyJoin("a", ""); got != "a/" {
			t.Fatalf("空段语义错：%q", got)
		}
		if got := cnsfPyJoin("a", "", "b"); got != "a/b" {
			t.Fatalf("空段中段语义错：%q", got)
		}
		if got := cnsfPyNormpath("//a/b"); got != "//a/b" {
			t.Fatalf("normpath 双斜杠保留错：%q", got)
		}
	})

	t.Run("RegexParity", func(t *testing.T) {
		// 负后顾 `(?<![\w.-])`：`xln` 不命、` ln` 命。
		if m := cnsfSearchAnchored("xln -s a b", cnsfReLN, cnsfLbOK); m != nil {
			t.Fatal("`xln` 不应命中（负后顾失守）")
		}
		if m := cnsfSearchAnchored("ok ln -s a b", cnsfReLN, cnsfLbOK); m == nil {
			t.Fatal("` ln` 应命中")
		}
		// Unicode 空白 `\s`（U+00A0）—— Go 默认 `\s` 不认，本件展开类应认。
		if m := cnsfSearchAnchored("ln\u00a0-s a b", cnsfReLN, cnsfLbOK); m == nil {
			t.Fatal("U+00A0 应认作 \\s（展开类失守）")
		}
		// `\b` 词界：`xrmtree(` 不命、` rmtree(` 命。
		if got := cnsfFindAllBoundary("xrmtree(a)", cnsfReBndRmtree); len(got) != 0 {
			t.Fatal("`xrmtree` 不应命中 \\brmtree")
		}
		if got := cnsfFindAllBoundary("go rmtree(a, b)", cnsfReBndRmtree); len(got) != 1 || got[0][1] != "a" {
			t.Fatalf("` rmtree(a` 应命中且组= a：%v", got)
		}
		// 行尾注释剥离：`\s#\s`。
		if got := cnsfStripLineComment("rm -f x # note"); got != "rm -f x" {
			t.Fatalf("行尾注释剥离错：%q", got)
		}
		if got := cnsfStripLineComment("a  #b"); got != "a  #b" {
			t.Fatalf("`#b` 无后空白 ⇒ 不剥离：%q", got)
		}
		// splitlines：`\v` 是行界。
		if got := cnsfSplitLines("a\vb\nc"); len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "c" {
			t.Fatalf("splitlines 错：%q", got)
		}
		// RE_ASSIGN 的 `^\s*`（前导缩进）与 `local`/`export`/`declare` 前缀 —— 静默漏赋值会让 A1?/A3 整片判反。
		for _, s := range []string{"  local fix=\"${tmp}/fix\"", "export R=\"/x\"", "declare -xr V=1", "fix=\"${tmp}/fix\""} {
			if m := cnsfReAssign.FindStringSubmatch(s); m == nil {
				t.Fatalf("RE_ASSIGN 应命中 %q", s)
			}
		}
		if m := cnsfReAssign.FindStringSubmatch("foo=bar"); m != nil && m[1] != "foo" {
			t.Fatalf("RE_ASSIGN 组名错：%v", m)
		}
	})

	t.Run("JSONEscapesBackslash", func(t *testing.T) {
		base := t.TempDir()
		real := cnsfMkdir(t, base, "real")
		d := cnsfMkdir(t, base, "root")
		cnsfWrite(t, d, "f.sh", "REPO=\""+real+"\"\nROOT=\"$(cd x && pwd)\"\nln -sf \"$ROOT/y\" z\n")
		_, out, _ := cnsfT(t, "--root", d, "--real-repo", real, "--json", "--no-self-test")
		if !strings.Contains(out, "\\\"$(cd x && pwd)\\\"") {
			t.Fatalf("why 里的引号未走转义安全形态：%q", out)
		}
	})
}
