// check_placeholder_residue_test.go —— `check_placeholder_residue.go`（M2 真搬）的**成对负控**测试件。
//
// 纪律（✗ 禁恒绿）：
//
//	① **负控有牙**：真残留仓 ⇒ 本件入口必须 rc=1（哑化 `cprIsResidue` ⇒ 该断言 FAIL 带行号）。
//	② **成对正控**：同一段正文「定义列」形态 ⇒ 必须 rc=0（防假红）。
//	③ **取不到 ≠ 零命中**：无 `.git` 物化树 ⇒ rc=2（不许静默当通过）。
//	④ **用法三态**（代 null/缺键面 · 本件无 JSON 入参）：缺省 / 空串 / 显式第二位置参数。
//	⑤ **词法同构钉格**：`PyJoin` / `PyNormpath`（双前导斜杠）/ `PyDirname` 各一枚。
//	⑥ **码点面钉格**：定义列判据按**码点**判邻位（中文前缀不得劈半）。
//	⑦ **两环境对拍**（乙 · 仅当 py 原件与**钉死**的 `/usr/bin/python3` 在盘）：仓外夹具 ＋ **每夹具自放 py 副本**。
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// cprTestPyExe —— 对拍用的解释器**钉死**（缺口 `GAP-20261001-61`：✗ 不用 PATH 上的裸 `python3`）。
//
//	PATH 一漂（3.9 的 `optional arguments:` ⟷ 3.14 的 `options:`）对拍即恒红，且与实现无关；
//	⇒ 与同族件同规（`cicgTestPython` / `ccatTestPyExe` / `cacsPyExe` / `tsmPython` 一律 `/usr/bin/python3`）。
const cprTestPyExe = "/usr/bin/python3"

// cprPhTest —— 占位符形态（拼接构造 ⇒ 源码里不落整串 · 本件落在自己扫描面内）。
func cprPhTest() string { return "<volume-" + "path>" }

// cprRepoRootFromTest —— 从测试 cwd（包目录）上溯三层取仓根（不写死绝对路径）。
func cprRepoRootFromTest(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	abs, err := filepath.Abs(filepath.Join(wd, "..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

// cprTestEnv —— 与对拍同口径的环境：unset ZERG_* ＋ TMPDIR=/private/var/tmp。
func cprTestEnv() []string {
	var out []string
	for _, kv := range os.Environ() {
		k := kv
		if i := strings.IndexByte(kv, '='); i >= 0 {
			k = kv[:i]
		}
		switch k {
		case "ZERG_STATE_DIR", "ZERG_REPO", "ZERG_REPO_ROOT", "ZERG_PROPOSAL_DIR":
			continue
		}
		out = append(out, kv)
	}
	return append(out, "TMPDIR=/private/var/tmp")
}

// cprGitIn —— 在夹具里真跑 git（只在仓外夹具上 init/commit · ✗ 不碰真仓）。
func cprGitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	base := []string{"-C", dir, "-c", "user.email=fx@invalid", "-c", "user.name=fx", "-c", "commit.gpgsign=false"}
	cmd := exec.Command("git", append(base, args...)...)
	cmd.Env = append(cprTestEnv(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v 失败：%v\n%s", args, err, out)
	}
}

// cprFixture —— 造一枚仓外 git 夹具（含可选 py 副本）。base 用**非符号链接**路径。
func cprFixture(t *testing.T, name, body string, withGit bool, withPyCopy bool) string {
	t.Helper()
	base := "/private/var/tmp"
	if st, err := os.Stat(base); err != nil || !st.IsDir() {
		base = t.TempDir()
	}
	d, err := os.MkdirTemp(base, "cpr-go-fx-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(d) })
	d = filepath.Join(d, name)
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	if withGit {
		cprGitIn(t, d, "init", "-q", "-b", "main")
	}
	if err := os.WriteFile(filepath.Join(d, "说明.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if withPyCopy {
		pd := filepath.Join(d, "scripts", "gates")
		if err := os.MkdirAll(pd, 0o755); err != nil {
			t.Fatal(err)
		}
		src := filepath.Join(cprRepoRootFromTest(t), "scripts", "gates", "check-placeholder-residue.py")
		if b, err := os.ReadFile(src); err == nil {
			_ = os.WriteFile(filepath.Join(pd, "check-placeholder-residue.py"), b, 0o644)
		}
	}
	if withGit {
		cprGitIn(t, d, "add", "-A")
		cprGitIn(t, d, "commit", "-q", "-m", "fixture")
	}
	return d
}

// cprRunEntry —— 进程内跑入口（对拍 = 两环境三面）。
func cprRunEntry(argv ...string) (int, string, string) {
	var out, errb bytes.Buffer
	rc := checkPlaceholderResidueGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

// ── 负控（有牙）＋ 成对正控 ────────────────────────────────────────────────────────────────
func TestPlaceholderResidueNegativeControl(t *testing.T) {
	ph := cprPhTest()
	cases := []struct {
		name   string
		body   string
		git    bool
		argv2  []string
		wantRC int
	}{
		{"残留（占位符＋路径样余部）", "配置在 " + ph + " 中文字目录/示例.json 里\n", true, nil, 1},
		{"干净（余部非路径样）", "路径见 " + ph + " 即可（后面是普通词，无余部）\n", true, nil, 0},
		{"成对正控（定义列 ⇒ 不许红）", "regex:/Volumes/" + `(AHZ|BXC)*` + "\t" + ph + "\t私有卷路径（停在引号/反引号/换行）\n", true, nil, 0},
		{"取不到（无 .git 物化树）⇒ 判不了", "配置在 " + ph + " 中文字目录/示例.json 里\n", false, nil, 2},
		{"用法错（未知旗标）", "路径见 " + ph + " 即可\n", true, []string{"--no-such-flag"}, 2},
		{"用法错（--json 无机器读面）", "路径见 " + ph + " 即可\n", true, []string{"--json"}, 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := cprFixture(t, "fx", c.body, c.git, false)
			argv := append([]string{d, "HEAD"}, c.argv2...)
			rc, _, _ := cprRunEntry(argv...)
			if rc != c.wantRC {
				t.Fatalf("入口 rc=%d，要 %d（夹具 %s）", rc, c.wantRC, d)
			}
		})
	}
}

// ── 用法三态（代 null/缺键面 · 本件无 JSON 入参 ⇒ 逐字说明并给理由）────────────────────────
func TestPlaceholderResidueArgvTriState(t *testing.T) {
	// 本件**无 JSON 入参** ⇒ 「缺键 / 显式 null / 空集」三形态在入参面不存在；
	// 改用**位置参数三态**：缺省（只给根）/ 空串第二参数 / 显式第二参数。
	// 用**干净**夹具（rc=0 ⇒ 「结果: 通过」那一行才带 `mode=`；rc=1 的不通过行不带 mode）。
	ph := cprPhTest()
	d := cprFixture(t, "tri", "路径见 "+ph+" 即可（后面是普通词，无余部）\n", true, false)

	rc1, out1, _ := cprRunEntry(d) // 缺省 mode ⇒ HEAD
	if rc1 != 0 || !strings.Contains(out1, "mode=HEAD") {
		t.Fatalf("缺省第二参数：rc=%d（要 0）· out 含 mode=HEAD=%v\n%s", rc1, strings.Contains(out1, "mode=HEAD"), out1)
	}
	rc2, out2, _ := cprRunEntry(d, "") // 空串 ⇒ mode=""（非 --all ⇒ 仍扫 HEAD；印出 `mode=）`）
	if rc2 != 0 || !strings.Contains(out2, "mode=）") {
		t.Fatalf("空串第二参数：rc=%d（要 0）· out 含 mode=空=%v\n%s", rc2, strings.Contains(out2, "mode=）"), out2)
	}
	rc3, out3, _ := cprRunEntry(d, "somebranch") // 显式非 --all ⇒ 只印 mode、扫 HEAD
	if rc3 != 0 || !strings.Contains(out3, "mode=somebranch") {
		t.Fatalf("显式第二参数：rc=%d（要 0）· out 含 mode=somebranch=%v\n%s", rc3, strings.Contains(out3, "mode=somebranch"), out3)
	}
}

// ── 码点面钉格：定义列判据按码点判邻位 ────────────────────────────────────────────────────
func TestPlaceholderResidueDefinitionColumnRune(t *testing.T) {
	ph := cprPhTest()
	// 中文前缀（多字节）后接「\t 占位符 \t」⇒ 定义列 ⇒ 非残留。
	if hit, _ := cprIsResidue("中文字段\t" + ph + "\t说明/含斜杠\n"); hit {
		t.Fatalf("定义列（中文前缀）被判成残留 ⇒ 码点邻位判错")
	}
	// 同一段但前缀不是制表符 ⇒ 残余（余部含斜杠）⇒ 命中。
	if hit, _ := cprIsResidue("中文" + ph + "/中文字目录/示例\n"); !hit {
		t.Fatalf("紧贴中文的路径样余部未被判成残留")
	}
}

// ── 词法同构三枚（标准 ①）──────────────────────────────────────────────────────────────
func TestPlaceholderResiduePyJoinLexical(t *testing.T) {
	cases := []struct{ got, want, why string }{
		{cprPyJoin("a", ".."), "a/..", "py join 吃 .. 原样留串（Go filepath.Join 会归一成 '.'）"},
		{cprPyJoin("repo", "/abs"), "/abs", "py join 后段绝对 ⇒ 丢前段（Go 拼成 repo/abs）"},
		{cprPyJoin("a", ""), "a/", "py join 空段保留尾斜杠（Go 忽略空段）"},
		{cprPyNormpath("//a/b"), "//a/b", "py normpath 保留恰两个前导斜杠（filepath.Clean 折成 /a/b）"},
		{cprPyNormpath("//a/b/../c"), "//a/c", "py normpath 双前导斜杠 ＋ .. 归并"},
		{cprPyNormpath("///a/b"), "/a/b", "三个及以上前导斜杠折成一个"},
		{cprPyDirname("/a/b/c"), "/a/b", "py dirname"},
		{cprPyDirname("a"), "", "py dirname 无斜杠 ⇒ 空"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s：got=%q want=%q", c.why, c.got, c.want)
		}
	}
}

// ── 自检前言（selftest）＋ `--self-test` ─────────────────────────────────────────────────
func TestPlaceholderResidueSelfTestFlag(t *testing.T) {
	rc, out, _ := cprRunEntry("--self-test")
	if rc != 0 {
		t.Fatalf("--self-test rc=%d（要 0）\n%s", rc, out)
	}
	if !strings.Contains(out, "── 自检：全过 ✓") {
		t.Fatalf("--self-test 未报「全过」：\n%s", out)
	}
}

// ── 两环境对拍（乙 · 仓外夹具 ＋ 每夹具自放 py 副本）────────────────────────────────────────
func TestPlaceholderResidueParityWithPy(t *testing.T) {
	pySrc := filepath.Join(cprRepoRootFromTest(t), "scripts", "gates", "check-placeholder-residue.py")
	if _, err := os.Stat(pySrc); err != nil {
		t.Skip("py 原件不在盘 ⇒ 跳过对拍")
	}
	if _, err := os.Stat(cprTestPyExe); err != nil {
		t.Skipf("%s 不在盘 ⇒ 跳过对拍（GAP-20261001-61：钉解释器，✗ 不掉进 PATH 上的裸 python3）", cprTestPyExe)
	}
	ph := cprPhTest()
	d := cprFixture(t, "parity", "配置在 "+ph+" 中文字目录/示例.json 里\n", true, true)
	pyCopy := filepath.Join(d, "scripts", "gates", "check-placeholder-residue.py")

	runPy := func(prog string, argv ...string) (int, string, string) {
		cmd := exec.Command(cprTestPyExe, append([]string{prog}, argv...)...)
		cmd.Env = cprTestEnv()
		cmd.Dir = d
		var so, se bytes.Buffer
		cmd.Stdout, cmd.Stderr = &so, &se
		err := cmd.Run()
		rc := 0
		if err != nil {
			if ee, ok := err.(*exec.ExitError); ok {
				rc = ee.ExitCode()
			} else {
				t.Fatalf("起 python3 失败：%v", err)
			}
		}
		return rc, so.String(), se.String()
	}

	prc, pout, perr := runPy(pyCopy, d, "HEAD")
	grc, gout, gerr := cprRunEntry(d, "HEAD")
	if prc != grc || pout != gout || perr != gerr {
		t.Fatalf("两环境对拍失配：\n py rc=%d\n go rc=%d\n py out=%q\n go out=%q\n py err=%q\n go err=%q",
			prc, grc, pout, gout, perr, gerr)
	}
}
