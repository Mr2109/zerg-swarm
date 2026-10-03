// check_fields_face_parity_test.go —— `check_fields_face_parity.go` 的**成对负控**单测
// （M2 一枚 · 承接 `GAP-20261001-13` · 入口 `checkFieldsFaceParityGateMain`）。
//
// 口径：✗ 禁恒绿 —— 每格都断言**判据面**（rc ＋ 关键正文），不是只断言「跑起来了」。
// 成对：正控（声明齐 ⇒ rc=0）与负控（多一名 / 少一名 / 同数不同名 / 棘轮不够 / 制品缺 /
// 无字段表 / 超时 / 读不了的件 / 坏旗标）各成子用例。
// 空值三形态 **逐形态**各一格：`[]`（空集合走方括号）· 缺键（`error` 档只有一键）·
// `null`（本门输出**永不产 `null`**）。
// 有牙实证（仓外副本哑化 ⇒ 本件用例 FAIL 带行号）见交付回执。
//
// ★ 夹具一律 `t.TempDir()`（仓外 · 非符号链接路径）· ✗ 不碰真仓件 · ✗ 不写真实状态目录。
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// ffpTestTree —— 造一棵**仓外夹具树**（`core/cmd/zerg/main.go` ＋ `bin/`）。
func ffpTestTree(t *testing.T, mainBody string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(ffpPyJoin(root, "core", "cmd", "zerg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if mainBody != "" {
		if err := os.WriteFile(ffpPyJoin(root, "core", "cmd", "zerg", "main.go"),
			[]byte(mainBody), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(ffpPyJoin(root, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func ffpTestBin(t *testing.T, root, body string) string {
	t.Helper()
	p := ffpPyJoin(root, "bin", "zerg")
	if err := os.WriteFile(p, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func ffpTestRun(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := checkFieldsFaceParityGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

// 合成命令树：两条命令 · 各带字段表（★ 只让**一条**命令的有尾 `\n\t}\n` 收口）。
const (
	ffpTMainGood = "package main\nvar commands = []*command{\n" +
		"\t{path: []string{\"zz\", \"ls\"}, fields: []string{\"name\", \"verdict\"}, run: x},\n" +
		"\t{path: []string{\"zz\", \"show\"}, fields: []string{\"alpha\"}, run: y},\n" +
		"\t}\n"
	ffpTMainMismatch = "package main\nvar commands = []*command{\n" +
		"\t{path: []string{\"zz\", \"ls\"}, fields: []string{\"name\", \"nope\"}, run: x},\n" +
		"\t}\n"
	ffpTMainEmptyFields = "package main\nvar commands = []*command{\n" +
		"\t{path: []string{\"zz\", \"ls\"}, fields: []string{}, run: x},\n" +
		"\t}\n"
	ffpTMainNoMark = "package main\nvar commands = []*command{}\n"
)

func ffpTBinGood() string {
	return "#!/bin/sh\ncase \"$*\" in\n" +
		"\"zz ls --json " + ffpBogus + "\") echo \"合法字段: name,verdict\" >&2; exit 2;;\n" +
		"\"zz show --json " + ffpBogus + "\") echo \"合法字段: alpha\" >&2; exit 2;;\n" +
		"*) echo \"boom\" >&2; exit 2;;\nesac\n"
}

func ffpTBinExit3() string { return "#!/bin/sh\necho nope >&2\nexit 3\n" }

func ffpTBinSleep() string { return "#!/bin/sh\nexec sleep 3\n" }

// ── T1 对拍机（正控 + 四条负控）────────────────────────────────────────────────

func TestFFPComparePair(t *testing.T) {
	cases := []struct {
		name         string
		decl, real   []string
		wantE, wantM []string
	}{
		{"正控_声明等于真身_无差", []string{"a", "b"}, []string{"a", "b"}, []string{}, []string{}},
		{"负控_声明多一名_点名extra", []string{"a", "b", "c"}, []string{"a", "b"}, []string{"c"}, []string{}},
		{"负控_声明少一名_点名runtime", []string{"a"}, []string{"a", "b"}, []string{}, []string{"b"}},
		{"负控_同数不同名_两边都点名", []string{"a", "z"}, []string{"a", "b"}, []string{"z"}, []string{"b"}},
		{"负控_仅顺序不同_无差不比序", []string{"b", "a"}, []string{"a", "b"}, []string{}, []string{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e, m := ffpCompare(c.decl, c.real)
			if strings.Join(e, ",") != strings.Join(c.wantE, ",") ||
				strings.Join(m, ",") != strings.Join(c.wantM, ",") {
				t.Fatalf("compare(%v,%v) = (%v,%v)，期望 (%v,%v)",
					c.decl, c.real, e, m, c.wantE, c.wantM)
			}
		})
	}
}

// ── T2 解析面：抓得到 path ＋ 声明名单（✗ 不是 bool）──────────────────────────

func TestFFPDeclaredFacesParse(t *testing.T) {
	t.Run("解析面_合成树_名单逐字", func(t *testing.T) {
		root := ffpTestTree(t, ffpTMainGood)
		faces, none, fatal := ffpDeclaredFaces(root)
		if none || fatal {
			t.Fatalf("declaredFaces 失败：none=%v fatal=%v", none, fatal)
		}
		if len(faces) != 2 {
			t.Fatalf("命令条数 %d，期望 2（%v）", len(faces), faces)
		}
		if faces[0].path != "zz ls" || strings.Join(faces[0].fields, ",") != "name,verdict" {
			t.Fatalf("第 1 条解析错：%q %v", faces[0].path, faces[0].fields)
		}
		if faces[1].path != "zz show" || strings.Join(faces[1].fields, ",") != "alpha" {
			t.Fatalf("第 2 条解析错：%q %v", faces[1].path, faces[1].fields)
		}
	})
	t.Run("解析面_空字段表_抓到空集非缺项", func(t *testing.T) {
		root := ffpTestTree(t, ffpTMainEmptyFields)
		faces, none, fatal := ffpDeclaredFaces(root)
		if none || fatal || len(faces) != 1 {
			t.Fatalf("空字段表解析：none=%v fatal=%v n=%d", none, fatal, len(faces))
		}
		if len(faces[0].fields) != 0 {
			t.Fatalf("空字段表应为空集，实得 %v", faces[0].fields)
		}
	})
}

// ── T3 空值三形态（`[]` · 缺键 · `null`）─────────────────────────────────────

func TestFFPEmptyForms(t *testing.T) {
	t.Run("空集合形态_出方括号_不出null", func(t *testing.T) {
		if got := ffpDump(ffpArr()); got != "[]" {
			t.Fatalf("空数组 dump = %q，期望 %q（✗ 不许落 null）", got, "[]")
		}
		if got := ffpDump(ffpStrArr(nil)); got != "[]" {
			t.Fatalf("nil 切片 dump = %q，期望 %q", got, "[]")
		}
		root := ffpTestTree(t, ffpTMainGood)
		ffpTestBin(t, root, ffpTBinGood())
		rc, out, _ := ffpTestRun(t, root, "--min-covered", "2", "--json")
		if rc != 0 {
			t.Fatalf("正控 rc=%d，期望 0（out=%s）", rc, out)
		}
		if !strings.Contains(out, `"mismatched": []`) || !strings.Contains(out, `"not_comparable": []`) {
			t.Fatalf("空集合未走方括号：%s", out)
		}
	})
	t.Run("缺键形态_error档只有一键", func(t *testing.T) {
		root := ffpTestTree(t, "") // main.go 缺席
		rc, out, _ := ffpTestRun(t, root, "--json")
		if rc != 2 {
			t.Fatalf("坏件 rc=%d，期望 2", rc)
		}
		var top map[string]any
		if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &top); err != nil {
			t.Fatalf("机器读面不是合法 JSON：%v（%s）", err, out)
		}
		items, _ := top["items"].([]any)
		if len(items) != 1 {
			t.Fatalf("items 条数 %d，期望 1", len(items))
		}
		item, _ := items[0].(map[string]any)
		if len(item) != 1 || item["error"] == nil {
			t.Fatalf("error 档应只出 `error` 一键，实得 %v", item)
		}
		for _, k := range []string{"declared", "covered", "min_covered", "matched", "mismatched", "not_comparable"} {
			if _, has := item[k]; has {
				t.Fatalf("error 档不该出键 %q（%v）", k, item)
			}
		}
	})
	t.Run("null形态_输出永不产null", func(t *testing.T) {
		root := ffpTestTree(t, ffpTMainMismatch)
		ffpTestBin(t, root, ffpTBinGood())
		_, out, _ := ffpTestRun(t, root, "--json")
		if strings.Contains(out, "null") {
			t.Fatalf("输出出现了 null（本门永不产 null）：%s", out)
		}
	})
}

// ── T4 读不了的件（py `except OSError: return None` ⇒ rc=2 逐字对齐）──────────

func TestFFPReadUnreadable(t *testing.T) {
	t.Run("读不了的件_main_go_不可读_rc2", func(t *testing.T) {
		root := ffpTestTree(t, ffpTMainGood)
		p := ffpPyJoin(root, "core", "cmd", "zerg", "main.go")
		if err := os.Chmod(p, 0); err != nil {
			t.Fatal(err)
		}
		rc, out, _ := ffpTestRun(t, root)
		if rc != 2 {
			t.Fatalf("读不了的件 rc=%d，期望 2（out=%s）", rc, out)
		}
		if !strings.Contains(out, "读不到命令树") {
			t.Fatalf("未点名「读不到命令树」：%s", out)
		}
	})
	t.Run("无commands标记_rc2", func(t *testing.T) {
		root := ffpTestTree(t, ffpTMainNoMark)
		rc, out, _ := ffpTestRun(t, root)
		if rc != 2 || !strings.Contains(out, "读不到命令树") {
			t.Fatalf("无标记 rc=%d out=%s", rc, out)
		}
	})
}

// ── T5 制品面（缺件 / 无字段表 / 超时 / 绝对路径）────────────────────────────

func TestFFPArtifactFace(t *testing.T) {
	t.Run("制品不存在_rc2_点名缺制品", func(t *testing.T) {
		root := ffpTestTree(t, ffpTMainGood)
		rc, out, _ := ffpTestRun(t, root, "--bin", ffpPyJoin(root, "bin", "nope"))
		if rc != 2 || !strings.Contains(out, "缺制品") {
			t.Fatalf("缺制品 rc=%d out=%s", rc, out)
		}
	})
	t.Run("制品在但无字段表_rc2_点名无字段表", func(t *testing.T) {
		root := ffpTestTree(t, ffpTMainGood)
		ffpTestBin(t, root, ffpTBinExit3())
		rc, out, _ := ffpTestRun(t, root)
		if rc != 2 {
			t.Fatalf("无字段表 rc=%d，期望 2", rc)
		}
		if !strings.Contains(out, "无字段表（rc=3") {
			t.Fatalf("未点名 rc=3 的无字段表：%s", out)
		}
	})
	t.Run("超时_rc2_出超时文案", func(t *testing.T) {
		root := ffpTestTree(t, ffpTMainGood)
		ffpTestBin(t, root, ffpTBinSleep())
		rc, out, _ := ffpTestRun(t, root, "--timeout", "1")
		if rc != 2 || !strings.Contains(out, "超时（>1.0s") {
			t.Fatalf("超时 rc=%d out=%s", rc, out)
		}
	})
	t.Run("传绝对路径_rc0", func(t *testing.T) {
		root := ffpTestTree(t, ffpTMainGood)
		abs := ffpTestBin(t, root, ffpTBinGood())
		rc, out, _ := ffpTestRun(t, root, "--bin", abs, "--min-covered", "2")
		if rc != 0 {
			t.Fatalf("绝对路径 rc=%d（out=%s）", rc, out)
		}
		if !strings.Contains(out, "2 条逐名相等") {
			t.Fatalf("未报逐名相等条数：%s", out)
		}
	})
	t.Run("棘轮不够_rc2_不给结论", func(t *testing.T) {
		root := ffpTestTree(t, ffpTMainGood)
		ffpTestBin(t, root, ffpTBinGood())
		rc, out, _ := ffpTestRun(t, root, "--min-covered", "29")
		if rc != 2 || !strings.Contains(out, "可对拍 2 < 棘轮 29") {
			t.Fatalf("棘轮 rc=%d out=%s", rc, out)
		}
	})
	t.Run("逐名差异_rc1_两边点名", func(t *testing.T) {
		root := ffpTestTree(t, ffpTMainMismatch)
		ffpTestBin(t, root, ffpTBinGood())
		rc, out, _ := ffpTestRun(t, root, "--min-covered", "1")
		if rc != 1 {
			t.Fatalf("差异 rc=%d，期望 1（out=%s）", rc, out)
		}
		if !strings.Contains(out, "声明多 ['nope']") || !strings.Contains(out, "真身多 ['verdict']") {
			t.Fatalf("未逐名点名：%s", out)
		}
	})
}

// ── T6 argparse 子集面 ────────────────────────────────────────────────────────

func TestFFPArgs(t *testing.T) {
	t.Run("帮助_rc0_逐字", func(t *testing.T) {
		rc, out, _ := ffpTestRun(t, "-h")
		if rc != 0 || out != ffpHelpText() {
			t.Fatalf("-h rc=%d 正文不符（%q）", rc, out)
		}
	})
	t.Run("未知旗标_rc2_逐字usage", func(t *testing.T) {
		rc, _, errs := ffpTestRun(t, "--badflag")
		want := ffpUsageHead() + ffpProg + ": error: unrecognized arguments: --badflag\n"
		if rc != 2 || errs != want {
			t.Fatalf("坏旗标 rc=%d err=%q 期望 %q", rc, errs, want)
		}
	})
	t.Run("缺值_rc2", func(t *testing.T) {
		rc, _, errs := ffpTestRun(t, "--bin")
		if rc != 2 || !strings.Contains(errs, "argument --bin: expected one argument") {
			t.Fatalf("缺值 rc=%d err=%q", rc, errs)
		}
	})
	t.Run("int非法_rc2", func(t *testing.T) {
		rc, _, errs := ffpTestRun(t, "--min-covered", "x")
		if rc != 2 || !strings.Contains(errs, "invalid int value: 'x'") {
			t.Fatalf("非法 int rc=%d err=%q", rc, errs)
		}
	})
	t.Run("float非法_rc2", func(t *testing.T) {
		rc, _, errs := ffpTestRun(t, "--timeout", "x")
		if rc != 2 || !strings.Contains(errs, "invalid float value: 'x'") {
			t.Fatalf("非法 float rc=%d err=%q", rc, errs)
		}
	})
	t.Run("唯一前缀缩写_rc2_棘轮不够", func(t *testing.T) {
		root := ffpTestTree(t, ffpTMainGood)
		ffpTestBin(t, root, ffpTBinGood())
		rc, out, _ := ffpTestRun(t, root, "--min", "29")
		if rc != 2 || !strings.Contains(out, "棘轮 29") {
			t.Fatalf("前缀缩写 rc=%d out=%s", rc, out)
		}
	})
}

// ── T7 自检面（rc ＋ 逐条子用例 ＋ 自回收 delta=0）────────────────────────────

func TestFFPSelfTest(t *testing.T) {
	t.Run("自检_rc0_七控逐条", func(t *testing.T) {
		rc, out, _ := ffpTestRun(t, "--self-test")
		if rc != 0 {
			t.Fatalf("自检 rc=%d（%s）", rc, out)
		}
		for _, want := range []string{
			"── check-fields-face-parity 自检",
			"正控：声明 == 真身 ⇒ 无差",
			"负控①：声明多一名 ⇒ 点名 extra=['c']",
			"负控②：声明少一名 ⇒ 点名 runtime_extra=['b']",
			"负控③：同数不同名 ⇒ 两边都点名",
			"负控④：仅顺序不同 ⇒ 无差（不比序）",
			"解析面：合成树 ⇒ 名单逐字（不是 bool）",
			"负控⑤：读不到命令树 ⇒ rc=2",
			"── 全过 ──",
		} {
			if !strings.Contains(out, want) {
				t.Fatalf("自检输出缺：%q\n%s", want, out)
			}
		}
	})
	t.Run("自检自回收_delta0", func(t *testing.T) {
		t.Setenv("TMPDIR", t.TempDir()) // GAP-20261003-16：私有计数面（只数本用例自建 tmp 目录）
		before := ffpCountSelftestDirs(t)
		if rc, out, _ := ffpTestRun(t, "--self-test"); rc != 0 {
			t.Fatalf("自检 rc=%d（%s）", rc, out)
		}
		after := ffpCountSelftestDirs(t)
		if after != before {
			t.Fatalf("自检临时目录未回收：before=%d after=%d（delta=%d）", before, after, after-before)
		}
	})
}

func ffpCountSelftestDirs(t *testing.T) int {
	t.Helper()
	ents, err := os.ReadDir(os.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), "check-fields-face-parity-selftest-") {
			n++
		}
	}
	return n
}

// ── T8 py 路径语义家族（三类差 + 双前导斜杠）──────────────────────────────────

func TestFFPPathSemantics(t *testing.T) {
	t.Run("pyJoin_点点段原样留串", func(t *testing.T) {
		if got := ffpPyJoin("a", "../b"); got != "a/../b" {
			t.Fatalf("join(a,../b) = %q，期望 a/../b（✗ 别归一）", got)
		}
	})
	t.Run("pyJoin_后段绝对丢前段", func(t *testing.T) {
		if got := ffpPyJoin("repo", "/abs"); got != "/abs" {
			t.Fatalf("join(repo,/abs) = %q，期望 /abs", got)
		}
	})
	t.Run("pyJoin_空段", func(t *testing.T) {
		if got := ffpPyJoin("a", ""); got != "a/" {
			t.Fatalf("join(a,'') = %q，期望 a/", got)
		}
		if got := ffpPyJoin("", "a"); got != "a" {
			t.Fatalf("join('',a) = %q，期望 a", got)
		}
		if got := ffpPyJoin("a", "", "b"); got != "a/b" {
			t.Fatalf("join(a,'',b) = %q，期望 a/b", got)
		}
	})
	t.Run("pyNormpath_恰两个前导斜杠保留", func(t *testing.T) {
		if got := ffpPyNormpath("//a/b"); got != "//a/b" {
			t.Fatalf("normpath(//a/b) = %q，期望 //a/b（✗ Clean 会折成 /a/b）", got)
		}
		if got := ffpPyNormpath("///a/b"); got != "/a/b" {
			t.Fatalf("normpath(///a/b) = %q，期望 /a/b", got)
		}
		if got := ffpPyNormpath("//a/b/../c"); got != "//a/c" {
			t.Fatalf("normpath(//a/b/../c) = %q，期望 //a/c", got)
		}
	})
	t.Run("pyDirname_posix_split语义", func(t *testing.T) {
		if got := ffpPyDirname("/a/b"); got != "/a" {
			t.Fatalf("dirname(/a/b) = %q", got)
		}
		if got := ffpPyDirname("/"); got != "/" {
			t.Fatalf("dirname(/) = %q，期望 /", got)
		}
		if got := ffpPyDirname("a"); got != "" {
			t.Fatalf("dirname(a) = %q，期望空串", got)
		}
	})
	t.Run("pyStrip_剥py空白含_1c_1f", func(t *testing.T) {
		if got := ffpPyStrip("\x1c a \x1f"); got != "a" {
			t.Fatalf("pyStrip = %q，期望 a（✗ TrimSpace 不剥 1c-1f）", got)
		}
		if got := ffpPyStrip("\u00a0b\u00a0"); got != "b" {
			t.Fatalf("pyStrip(nbsp) = %q，期望 b", got)
		}
	})
	t.Run("pySplitLines_十一行界", func(t *testing.T) {
		got := ffpSplitLines("a\r\nb\rc\x1cd")
		if strings.Join(got, "|") != "a|b|c|d" {
			t.Fatalf("splitlines = %v", got)
		}
	})
	t.Run("universalNewlines_折rn与裸r", func(t *testing.T) {
		if got := ffpUniversalNewlines("a\r\nb\rc"); got != "a\nb\nc" {
			t.Fatalf("universalNewlines = %q", got)
		}
	})
}
