package main

// check_marks_coverage_parity_test.go —— `check_marks_coverage_parity.go`（M2 真搬 · DEV-0933）的
// **成对负控**测试件。夹具全落**非符号链接路径**（t.TempDir() ⇒ /var/folders/… 或 TMPDIR），
// ✗ 不用 /var/tmp（符号链接 ⇒ rel 算成假失配）。✗ 不恒绿：每一条判据都有「造红 ⇒ 修正 ⇒ 回绿」两态。

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const mkcpTHead = "文件\tpublic\t理由\n"

type mkcpFx struct {
	root string
}

func mkcpWrite(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// mkcpMakeFx 造一棵最小树：表 + config + 两器排除表 + 白名单 + 盘上件 + git 索引。
func mkcpMakeFx(t *testing.T, tsv, globs string, files, excludes, dropExact []string) string {
	t.Helper()
	root := t.TempDir()
	mkcpWrite(t, root, mkcpTableRel, mkcpTHead+tsv)
	mkcpWrite(t, root, mkcpConfigRel,
		`{"schema":"gate-coverage/1","baseline":0,"scan":{"script_globs":`+globs+`},`+
			`"whitelist":{"build_dirs":[],"scripts":[]}}`)
	ex := ""
	for _, e := range excludes {
		ex += "  \"" + e + "\"\n"
	}
	mkcpWrite(t, root, mkcpPublishShRel, "#!/usr/bin/env bash\nEXCLUDES=(\n"+ex+")\n")
	drop := ""
	for i, d := range dropExact {
		if i > 0 {
			drop += ", "
		}
		drop += "\"" + d + "\""
	}
	mkcpWrite(t, root, mkcpMirrorLibRel, "DROP_PREFIX = ()\nDROP_EXACT = {"+drop+"}\n")
	mkcpWrite(t, root, mkcpWhitelistRel, "scripts/\n")
	for _, f := range files {
		mkcpWrite(t, root, f, "#!/usr/bin/env python3\nprint('fixture')\n")
	}
	if _, err := exec.LookPath("git"); err == nil {
		_ = exec.Command("git", "init", "-q").Run()
		c := exec.Command("git", "init", "-q")
		c.Dir = root
		_ = c.Run()
		if len(files) > 0 {
			args := append([]string{"add", "-f"}, files...)
			a := exec.Command("git", args...)
			a.Dir = root
			_ = a.Run()
		}
	}
	return root
}

func mkcpRunFx(t *testing.T, root string, args ...string) (int, string, string) {
	t.Helper()
	var so, se bytes.Buffer
	code := checkMarksCoverageParityGateMain(&so, &se, append([]string{"check-marks-coverage-parity.py", "--root", root}, args...))
	return code, so.String(), se.String()
}

var (
	mkcpFxFiles = []string{"scripts/build/kept.py", "scripts/gates/excluded.py", "scripts/exportnames/plug.py"}
	mkcpFxGlobs = `["scripts/*.sh","scripts/*.py","scripts/build/*.sh","scripts/build/*.py","scripts/gates/*.sh","scripts/gates/*.py"]`
	mkcpFxTSV   = "scripts/build/kept.py\tyes\tA 类：合成夹具（构建链）\n" +
		"scripts/gates/excluded.py\tno\tB 类：合成夹具（门禁）\n" +
		"scripts/exportnames/plug.py\tno\t面外域·合成夹具（豁免登记表里那一个）\n"
	mkcpFxExcl = []string{"scripts/gates/excluded.py", "scripts/exportnames/plug.py"}
	mkcpFxDrop = []string{"scripts/gates/excluded.py", "scripts/exportnames/plug.py"}
	mkcpFxArgs = []string{}
)

// 正控：两侧域对齐 + 面外域走登记豁免 + 已跟踪 no 件两侧都收 ⇒ rc=0
func TestMkcpPositiveControlGreen(t *testing.T) {
	root := mkcpMakeFx(t, mkcpFxTSV, mkcpFxGlobs, mkcpFxFiles, mkcpFxExcl, mkcpFxDrop)
	code, so, se := mkcpRunFx(t, root, mkcpFxArgs...)
	if code != 0 {
		t.Fatalf("正控应 rc=0，实得 rc=%d\nstdout=%s\nstderr=%s", code, so, se)
	}
	for _, want := range []string{
		"R1-a 门扫的域、表里一件都没登记：0 个",
		"R1-b 表登记的多余域（未登记豁免）：0 个",
		"R2-a 已跟踪且缺侧甲（= 判据⑥ 会红的那批的预演）：0 件",
		"全绿（rc=0）",
	} {
		if !strings.Contains(so, want) {
			t.Fatalf("正控输出缺字样 %q\nstdout=%s", want, so)
		}
	}
}

// 负控①（R1-a）：门侧多一个域（dev）而表里 0 行 ⇒ rc=1 且逐字点名 —— 再把它补进表 ⇒ 回绿。
func TestMkcpNegativeControlR1a(t *testing.T) {
	globs := `["scripts/*.sh","scripts/*.py","scripts/build/*.sh","scripts/build/*.py",` +
		`"scripts/gates/*.sh","scripts/gates/*.py","scripts/dev/*.sh","scripts/dev/*.py"]`
	files := append(append([]string{}, mkcpFxFiles...), "scripts/dev/lone.py")
	excl := append(append([]string{}, mkcpFxExcl...), "scripts/dev/lone.py")
	root := mkcpMakeFx(t, mkcpFxTSV, globs, files, excl, excl)
	code, so, _ := mkcpRunFx(t, root)
	if code != 1 {
		t.Fatalf("R1-a 应 rc=1，实得 rc=%d\nstdout=%s", code, so)
	}
	want := "dev  ← `script_globs` 有 `scripts/dev/*.sh|*.py`（实测 1 件），`公开标记.tsv` 里 0 行"
	if !strings.Contains(so, "R1-a 门扫的域、表里一件都没登记：1 个") || !strings.Contains(so, want) {
		t.Fatalf("R1-a 未逐字点名\nstdout=%s", so)
	}
	// 成对：补表 ⇒ 回绿（证明不是恒红）
	root2 := mkcpMakeFx(t, mkcpFxTSV+"scripts/dev/lone.py\tno\tB 类：合成夹具（dev 域）\n",
		globs, files, excl, excl)
	code2, so2, _ := mkcpRunFx(t, root2)
	if code2 != 0 || !strings.Contains(so2, "全绿（rc=0）") {
		t.Fatalf("成对修正后应回绿，实得 rc=%d\nstdout=%s", code2, so2)
	}
}

// 负控②（R2-a）：已跟踪的 no 件两器都缺 ⇒ rc=1 且点名「缺侧甲 + 侧乙」。
func TestMkcpNegativeControlR2a(t *testing.T) {
	root := mkcpMakeFx(t, mkcpFxTSV, mkcpFxGlobs, mkcpFxFiles,
		[]string{"scripts/exportnames/plug.py"}, []string{"scripts/exportnames/plug.py"})
	code, so, _ := mkcpRunFx(t, root)
	if code != 1 {
		t.Fatalf("R2-a 应 rc=1，实得 rc=%d\nstdout=%s", code, so)
	}
	if !strings.Contains(so, "R2-a 已跟踪且缺侧甲（= 判据⑥ 会红的那批的预演）：1 件") ||
		!strings.Contains(so, "scripts/gates/excluded.py（表第 3 行）· 缺侧甲 + 侧乙（两器都会发它）") {
		t.Fatalf("R2-a 未逐字点名\nstdout=%s", so)
	}
}

// 钳①：标记表缺件 ⇒ rc=2（判不了 ≠ 绿）
func TestMkcpClampMissingTable(t *testing.T) {
	root := mkcpMakeFx(t, mkcpFxTSV, mkcpFxGlobs, mkcpFxFiles, mkcpFxExcl, mkcpFxDrop)
	if err := os.Remove(filepath.Join(root, mkcpTableRel)); err != nil {
		t.Fatal(err)
	}
	code, _, se := mkcpRunFx(t, root)
	if code != 2 || !strings.Contains(se, "不给结论") {
		t.Fatalf("缺表应 rc=2 且报「不给结论」，实得 rc=%d stderr=%s", code, se)
	}
}

// 钳②：script_globs 为空 ⇒ rc=2（空转）
func TestMkcpClampEmptyGlobs(t *testing.T) {
	root := mkcpMakeFx(t, mkcpFxTSV, `[]`, mkcpFxFiles, mkcpFxExcl, mkcpFxDrop)
	code, _, se := mkcpRunFx(t, root)
	if code != 2 || !strings.Contains(se, "scan.script_globs 不是非空列表") {
		t.Fatalf("空 globs 应 rc=2，实得 rc=%d stderr=%s", code, se)
	}
}

// Q-176：本门无机器读面 ⇒ --json 一律拒（rc=2 · 三行逐字）
func TestMkcpJSONRefused(t *testing.T) {
	var so, se bytes.Buffer
	code := checkMarksCoverageParityGateMain(&so, &se, []string{"check-marks-coverage-parity.py", "--json"})
	if code != 2 {
		t.Fatalf("--json 应 rc=2，实得 rc=%d", code)
	}
	if !strings.Contains(se.String(), "⛔ 本门无机器读面：`--json` 在本门**未实现**（缺口 Q-176）") {
		t.Fatalf("--json 未逐字说明\nstderr=%s", se.String())
	}
}

// 未知旗标 ⇒ argparse 口径 rc=2
func TestMkcpUnknownFlag(t *testing.T) {
	var so, se bytes.Buffer
	code := checkMarksCoverageParityGateMain(&so, &se, []string{"prog", "--nope"})
	if code != 2 || !strings.Contains(se.String(), "unrecognized arguments: --nope") {
		t.Fatalf("未知旗标应 rc=2 且报 unrecognized，实得 rc=%d stderr=%s", code, se.String())
	}
}
