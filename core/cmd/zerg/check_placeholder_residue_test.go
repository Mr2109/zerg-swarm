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
//	⑦ **内化金标**（乙 · 旧 py 对拍退场 ⇒ 逐格金标收 Go 常量 · 见下）。
//
// ── 内化缘起（甲类·两件两处内化 · 照 atomic_write_test.go / pack_release_test.go）──
//
// 本件原「py 原件 ⟷ Go 真搬」两环境对拍（旧名 `TestPlaceholderResidueParityWithPy`）的对拍
// 真身定位 = 仓内 `scripts/gates/check-placeholder-residue.py` 在盘（不在即 `t.Skip`）＋ 钉死
// 解释器 `/usr/bin/python3` 在盘；该 py 已退场（`10910224` M2 退役批）⇒ Skip 条件恒真、对拍归零
// （假绿、零保护）。⇒ 把金标真源**收回 Go 半现役行为本身**：
//
//	① 哨兵改锚 **Go 半** `core/cmd/zerg/check_placeholder_residue.go`（✗ 不锚已退场 py ——
//	   一退役即失锚）；找不到 ⇒ `cprAnchorGoHalf` `t.Fatalf` **fail-closed**（✗ 不留 Skip）。
//	② ★ 金标计算式（✗ 不臆造 · 逐格由「Go 半现役行为现算」得到）：逐格「夹具正文 + argv ⇒
//	   (rc, stdout, stderr)」由 `cprRunEntry` 现跑捕获（一次性现算、只读仓外夹具、✗ 不落仓），
//	   归一后冻成下表 `cprGoldenTable`；**本机位置类量**在比对前归一化（✗ 不冻结 —— 因其本非常量）：
//	     夹具根（`t.TempDir()`/`$TMPDIR` 随机路径 + macOS /var⟷/private/var 符号链接）⇒ `<FX>`。
//	   ★ 自指纪律：金标里连成一串的完整占位符形态一律以 `{PH}` token 存放，比对前/建夹具时由
//	     `cprPlaceholder()` 现拼（✗ 源码文本里不落整串 —— 否则本件会命中自己的扫描面）。
//	③ 两态（✗ 不许 Skip）：A 态 = 夹具无 py（真仓现态）；B 态 = 合成「旧态」夹具（把退役 py
//	   路径 `scripts/gates/check-placeholder-residue.py` 放回夹具并提交）⇒ 两态**对同一套金标
//	   逐字同命中**，证 py 在/不在盘都不改判据结论（内化彻底）。
//	④ ★ 哑化负控（"牙" · 见交付读数）：`-overlay` 把 `check_placeholder_residue.go` 的判据一臂
//	   （`cprIsResidue` 恒返回 false）哑化 ⇒ `..._GoldenInternalized` **必 FAIL 且带行号**
//	   （✗ 不真改仓内源 · 免 `dev edit` 前的仓内污染）。
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// cprPhTest —— 占位符形态（拼接构造 ⇒ 源码里不落整串 · 本件落在自己扫描面内）。
func cprPhTest() string { return "<volume-" + "path>" }

// cprGoHalfRel / cprRetiredPyRel —— 哨兵锚（Go 半）与已退场对拍真身（py）的仓根相对路径。
const (
	cprGoHalfRel    = "core/cmd/zerg/check_placeholder_residue.go"
	cprRetiredPyRel = "scripts/gates/check-placeholder-residue.py"
)

// cprAnchorGoHalf —— 哨兵：锚 **Go 半**在盘（✗ 不锚已退场 py）；找不到 ⇒ fail-closed（缺件 ≠ 绿）。
func cprAnchorGoHalf(t *testing.T, root string) {
	t.Helper()
	fi, err := os.Stat(filepath.Join(root, filepath.FromSlash(cprGoHalfRel)))
	if err != nil || fi.IsDir() {
		t.Fatalf("哨兵失守：上溯未见 Go 半 %s（root=%s err=%v）⇒ fail-closed（✗ 不留 Skip）",
			cprGoHalfRel, root, err)
	}
}

// cprRepoRootFromTest —— 从测试 cwd（包目录）上溯三层取仓根；哨兵锚 Go 半 ⇒ fail-closed（✗ 不 Skip）。
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
	cprAnchorGoHalf(t, abs)
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

// ── 内化金标：旧 py 对拍的逐格金标收成 Go 常量 · 现算自证（✗ 不 Skip）────────────────────────

// cprGoldenTable —— 金标真源（3 格 · 逐格由 `cprRunEntry` 现跑捕获 · 见件头金标计算式）。
// ★ 输出里的完整占位符形态以 `{PH}` token 存放（自指纪律，见件头 ②）。
var cprGoldenTable = []struct {
	name    string
	wantRC  int
	wantOut string // stdout 归一化（夹具根 ⇒ `<FX>` · 占位符 ⇒ `{PH}`）后
	wantErr string
}{
	{name: "残留_余部路径样", wantRC: 1, wantOut: "   ✓ want=False got=False | 规则表定义（模式文本，非残留）                            | \n   ✓ want=True  got=True  | 截断残留（带空格续部）                                | {PH} 中文字目录/示例.json\n   ✓ want=False got=False | 正常正文：后面是普通词                                | \n   ✓ want=False got=False | 纯占位符，无余部                                   | \n   ✓ want=True  got=True  | 无空格直连的续部                                   | {PH}/中文字目录/示例\n   ✓ want=True  got=True  | 无空格直连的中文目录（含斜杠，形态成立）                       | {PH}中文字目录/示例.md\n   ✓ want=False got=False | 规则表定义列（占位符自成列 · 说明含斜杠）                     | \n   自测: 全过\n结果: 不通过——发现 1 处残留（1 个文件）：\n   HEAD:说明.md\n      1:{PH} 中文字目录/示例.json\n", wantErr: ""},
	{name: "干净_余部非路径样", wantRC: 0, wantOut: "   ✓ want=False got=False | 规则表定义（模式文本，非残留）                            | \n   ✓ want=True  got=True  | 截断残留（带空格续部）                                | {PH} 中文字目录/示例.json\n   ✓ want=False got=False | 正常正文：后面是普通词                                | \n   ✓ want=False got=False | 纯占位符，无余部                                   | \n   ✓ want=True  got=True  | 无空格直连的续部                                   | {PH}/中文字目录/示例\n   ✓ want=True  got=True  | 无空格直连的中文目录（含斜杠，形态成立）                       | {PH}中文字目录/示例.md\n   ✓ want=False got=False | 规则表定义列（占位符自成列 · 说明含斜杠）                     | \n   自测: 全过\n结果: 通过——未发现残留（扫 <FX> mode=HEAD）\n", wantErr: ""},
	{name: "定义列_占位符自成列", wantRC: 0, wantOut: "   ✓ want=False got=False | 规则表定义（模式文本，非残留）                            | \n   ✓ want=True  got=True  | 截断残留（带空格续部）                                | {PH} 中文字目录/示例.json\n   ✓ want=False got=False | 正常正文：后面是普通词                                | \n   ✓ want=False got=False | 纯占位符，无余部                                   | \n   ✓ want=True  got=True  | 无空格直连的续部                                   | {PH}/中文字目录/示例\n   ✓ want=True  got=True  | 无空格直连的中文目录（含斜杠，形态成立）                       | {PH}中文字目录/示例.md\n   ✓ want=False got=False | 规则表定义列（占位符自成列 · 说明含斜杠）                     | \n   自测: 全过\n结果: 通过——未发现残留（扫 <FX> mode=HEAD）\n", wantErr: ""},
}

// cprBodyByName —— 金标档名 ⇒ 夹具正文（占位符现拼 · ✗ 源码不落整串）。
func cprBodyByName(name, ph string) string {
	switch name {
	case "残留_余部路径样":
		return "配置在 " + ph + " 中文字目录/示例.json 里\n"
	case "干净_余部非路径样":
		return "路径见 " + ph + " 即可（后面是普通词，无余部）\n"
	case "定义列_占位符自成列":
		return "regex:/Volumes/" + `(AHZ|BXC)*` + "\t" + ph + "\t私有卷路径（停在引号/反引号/换行）\n"
	}
	return ""
}

// cprPhMask —— 把现拼的占位符掩成 `{PH}` token（与冻值同形 · ✗ 源码不落整串）。
func cprPhMask(s string) string { return strings.ReplaceAll(s, cprPlaceholder(), "{PH}") }

// cprRunGolden —— 逐格现建夹具、现跑 Go 半、归一（夹具根 ⇒ `<FX>` · 占位符 ⇒ `{PH}`）后与冻值逐字比；
// 返回失配格数。plant 非 nil 时在建夹具后落「旧态」py（两态用）。
func cprRunGolden(t *testing.T, tag string, plant func(d string)) int {
	t.Helper()
	ph := cprPlaceholder()
	mism := 0
	for _, c := range cprGoldenTable {
		d := cprFixture(t, "fx", cprBodyByName(c.name, ph), true, false)
		if plant != nil {
			plant(d)
		}
		rc, out, errb := cprRunEntry(d, "HEAD")
		gotOut := cprPhMask(strings.ReplaceAll(out, d, "<FX>"))
		gotErr := cprPhMask(strings.ReplaceAll(errb, d, "<FX>"))
		if rc != c.wantRC {
			mism++
			t.Errorf("[%s/%s] rc=%d（冻值 %d）", tag, c.name, rc, c.wantRC)
		}
		if gotOut != c.wantOut {
			mism++
			t.Errorf("[%s/%s] stdout 漂移\n实得=%q\n冻值=%q", tag, c.name, gotOut, c.wantOut)
		}
		if gotErr != c.wantErr {
			mism++
			t.Errorf("[%s/%s] stderr 漂移\n实得=%q\n冻值=%q", tag, c.name, gotErr, c.wantErr)
		}
	}
	return mism
}

// TestPlaceholderResidueParityGoldenInternalized —— A 态（真仓 · 退役 py 不在盘）金标自证
// （✗ 不 Skip · 逐格真断言 · ✗ 不放宽为「只判非空」）。
func TestPlaceholderResidueParityGoldenInternalized(t *testing.T) {
	root := cprRepoRootFromTest(t) // 哨兵：锚 Go 半在盘 ⇒ fail-closed（✗ 不留 Skip）
	if _, err := os.Stat(filepath.Join(root, cprRetiredPyRel)); err == nil {
		t.Fatalf("A 态前提破：退役 py 竟在盘 %s（本件前提 = 内化后无 py）", cprRetiredPyRel)
	}
	if len(cprGoldenTable) < 3 {
		t.Fatalf("金标格数 %d < 3", len(cprGoldenTable))
	}
	if n := cprRunGolden(t, "A真仓无py", nil); n != 0 {
		t.Fatalf("A 态金标失配 %d 格", n)
	}
}

// TestPlaceholderResidueParityGoldenTwoStates —— 两态逐字同：A 态 = 夹具无 py（真仓现态）；
// B 态 = 合成「旧态」夹具（把退役 py 路径 `scripts/gates/check-placeholder-residue.py` 放回
// 夹具并提交）⇒ 两态对同一套金标逐字同命中（✗ 不 Skip）。证 py 在/不在盘都不改判据结论。
func TestPlaceholderResidueParityGoldenTwoStates(t *testing.T) {
	cprRepoRootFromTest(t) // 哨兵锚 Go 半（✗ 不留 Skip）
	if n := cprRunGolden(t, "A夹具无py", nil); n != 0 {
		t.Fatalf("A 态金标失配 %d 格", n)
	}
	plant := func(d string) {
		pd := filepath.Join(d, "scripts", "gates")
		if err := os.MkdirAll(pd, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(pd, "check-placeholder-residue.py"),
			[]byte("# stub：造「旧态」——退役 py 在位（本件只证 py 在/不在盘不改判据结论）\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		cprGitIn(t, d, "add", "-A")
		cprGitIn(t, d, "commit", "-q", "-m", "old-state-py")
	}
	if n := cprRunGolden(t, "B旧态有py", plant); n != 0 {
		t.Fatalf("B 态金标失配 %d 格（py 在盘不得改判据结论）", n)
	}
}
