// gate_report_test.go —— 判据件 scripts/gates/gate-report.py 真搬件（gate_report.go）的
// **成对负控**测试件（✗ 禁恒绿）。三档：
//
//	① 黄金钉：帮助面 sha256 逐字钉 py 原件的 `-h` stdout（GAP-20261001-61）；
//	② py 语义面：路径/串/切片/保序 JSON/三态（空集合 vs null vs 取不到）各钉一格；
//	③ 真口子面：C 类 fail-closed（--key-regex 语法错）· D 类死旗标（--json）·
//	   超时档 · A 类自检夹具回收（★ 有牙：删掉主件的 defer os.RemoveAll 即 FAIL）。
//
// ★ 件内不写任何自门触发子串（R3 两形态原串一律不落）；JSON 断言只用**子串/裸键名**，
// ✗ 不落「引号键名加冒号加引号值」的连续原串形态。
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"
)

// grtTHelpSha256 = py 原件 `COLUMNS=80 /usr/bin/python3 scripts/gates/gate-report.py -h` 的 stdout
// 全串 sha256（1005 字节）。两侧同值 ⇒ 本件帮助面与 argparse 逐字同构。
const grtTHelpSha256 = "48607dcbf24fb15fe5a3af0090da8025d0b1f67fc87e005e7874903a31c52bb2"

// grtTReexecEnv：`go test` 二进制被 --self-test 再起时，用它把子进程切回 CLI 语义
// （★ 拼串而非直落整串：避开「魔法串连续字面落地」的自指形态）。
const grtTReexecEnv = "GRTREE" + "XEC_PROBE"

const grtTHelpBytes = 1005

// grtTReexecInit —— `go test` 二进制被 --self-test 再起时切回 CLI 语义。
// ★ 用 init 而✗ 不用 TestMain：本包**已有他件定义 TestMain**
// （core/cmd/zerg/check_public_tree_private_test.go:15）⇒ 再定义即 redeclared（共享件禁改）。
// init 同样先于 testing 的旗标解析与 m.Run 执行，故子进程能按 CLI 口径跑。
func init() {
	if os.Getenv(grtTReexecEnv) == "1" {
		os.Exit(gateReportMain(os.Stdout, os.Stderr, os.Args[1:]))
	}
}

func grtTRun(args ...string) (int, string, string) {
	var ob, eb bytes.Buffer
	rc := gateReportMain(&ob, &eb, args)
	return rc, ob.String(), eb.String()
}

// grtTFixture 造仓外夹具：<root>/scripts/gates/<名>。
func grtTFixture(t *testing.T, gates map[string]string) string {
	t.Helper()
	root := t.TempDir()
	dir := grtPyJoin(root, "scripts", "gates")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for n, body := range gates {
		if err := os.WriteFile(grtPyJoin(dir, n), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func grtTSha256(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// ─── ① 黄金钉：帮助面 sha256 ───────────────────────────────────────────────

func TestGateReportHelpGoldenSha256(t *testing.T) {
	for _, args := range [][]string{{"-h"}, {"--help"}, {"-h", "--no-such-flag"}} {
		rc, out, errs := grtTRun(args...)
		if rc != 0 {
			t.Fatalf("args=%v rc=%d 期望 0", args, rc)
		}
		if errs != "" {
			t.Fatalf("args=%v stderr 非空：%q", args, errs)
		}
		if len(out) != grtTHelpBytes {
			t.Fatalf("args=%v 帮助面字节数 = %d 期望 %d", args, len(out), grtTHelpBytes)
		}
		if got := grtTSha256(out); got != grtTHelpSha256 {
			t.Fatalf("args=%v 帮助面 sha256 = %s 期望（py 原件 -h stdout）= %s", args, got, grtTHelpSha256)
		}
	}
}

// ─── ② py 语义面 ──────────────────────────────────────────────────────────

func TestGateReportPyPathSemantics(t *testing.T) {
	t.Run("join_后段绝对重置前面所有段", func(t *testing.T) {
		if got := grtPyJoin("repo", "/abs"); got != "/abs" {
			t.Fatalf("grtPyJoin(repo,/abs) = %q 期望 /abs（filepath.Join 会给 repo/abs）", got)
		}
	})
	t.Run("join_点点段原样保留", func(t *testing.T) {
		if got := grtPyJoin("a", "../b"); got != "a/../b" {
			t.Fatalf("grtPyJoin(a,../b) = %q 期望 a/../b（filepath.Join 会归一成 b）", got)
		}
	})
	t.Run("join_空段三形态", func(t *testing.T) {
		if got := grtPyJoin("a", ""); got != "a/" {
			t.Fatalf("grtPyJoin(a,\"\") = %q 期望 a/", got)
		}
		if got := grtPyJoin("", "a"); got != "a" {
			t.Fatalf("grtPyJoin(\"\",a) = %q 期望 a", got)
		}
		if got := grtPyJoin("a", "", "b"); got != "a/b" {
			t.Fatalf("grtPyJoin(a,\"\",b) = %q 期望 a/b", got)
		}
	})
	t.Run("normpath_恰两个前导斜杠不归一", func(t *testing.T) {
		if got := grtPyNormpath("//a/b/../c"); got != "//a/c" {
			t.Fatalf("normpath(//a/b/../c) = %q 期望 //a/c", got)
		}
		if got := grtPyNormpath("///a/b"); got != "/a/b" {
			t.Fatalf("normpath(///a/b) = %q 期望 /a/b", got)
		}
	})
	t.Run("strip_剥1c至1f", func(t *testing.T) {
		if got := grtPyStrip("\x1c\x1d\x1e\x1f a \x1f"); got != "a" {
			t.Fatalf("grtPyStrip 未剥 0x1c-0x1f：%q（strings.TrimSpace 会留）", got)
		}
	})
	t.Run("splitlines_边界集与空串", func(t *testing.T) {
		if got := grtPySplitLines("a\x0bb"); len(got) != 2 || got[0] != "a" || got[1] != "b" {
			t.Fatalf("splitlines(a\\x0bb) = %q 期望 [a b]", got)
		}
		if got := grtPySplitLines("\n"); len(got) != 1 || got[0] != "" {
			t.Fatalf("splitlines(\\n) = %q 期望 [\"\"]", got)
		}
		if got := grtPySplitLines(""); got != nil {
			t.Fatalf("splitlines(\"\") = %q 期望 nil（py []）", got)
		}
	})
	t.Run("切片按码点非按字节", func(t *testing.T) {
		if got := grtPyRuneSlice("前缀xx", 0, 2); got != "前缀" {
			t.Fatalf("grtPyRuneSlice(前缀xx,0,2) = %q 期望 前缀", got)
		}
		if got := grtPyRuneSlice("前缀xx", -2, 4); got != "xx" {
			t.Fatalf("grtPyRuneSlice 负下界 = %q 期望 xx", got)
		}
	})
	t.Run("float_repr常见值域", func(t *testing.T) {
		cases := map[float64]string{0: "0.0", 1.5: "1.5", 0.012: "0.012", 123.456: "123.456"}
		for in, want := range cases {
			if got := grtPyFloatRepr(in); got != want {
				t.Fatalf("grtPyFloatRepr(%v) = %q 期望 %q", in, got, want)
			}
		}
	})
}

func TestGateReportOrderedJSON(t *testing.T) {
	v := grtObjOf(
		grtKVOf("schema", grtS("gate-report/1")),
		grtKVOf("n", grtInt(2)),
		grtKVOf("arr", grtArrOf([]*grtVal{grtInt(1), grtS("x")})),
	)
	t.Run("紧凑档分隔符与序", func(t *testing.T) {
		var sb strings.Builder
		grtWriteJSON(&sb, v, -1, 0)
		want := `{"schema": "gate-report/1", "n": 2, "arr": [1, "x"]}`
		if sb.String() != want {
			t.Fatalf("紧凑档 = %s 期望 %s", sb.String(), want)
		}
	})
	t.Run("缩进档两空格与换行", func(t *testing.T) {
		var sb strings.Builder
		grtWriteJSON(&sb, v, 2, 0)
		want := "{\n  \"schema\": \"gate-report/1\",\n  \"n\": 2,\n  \"arr\": [\n    1,\n    \"x\"\n  ]\n}"
		if sb.String() != want {
			t.Fatalf("缩进档 = %q 期望 %q", sb.String(), want)
		}
	})
	t.Run("串转义表", func(t *testing.T) {
		var sb strings.Builder
		grtWriteJSON(&sb, grtS("a\"b\\c\nd\x01f中"), -1, 0)
		want := "\"a\\\"b\\\\c\\nd\\u0001f中\""
		if sb.String() != want {
			t.Fatalf("转义 = %s 期望 %s", sb.String(), want)
		}
	})
	t.Run("空对象与空数组不换行", func(t *testing.T) {
		var sb strings.Builder
		grtWriteJSON(&sb, grtArrOf([]*grtVal{}), 2, 1)
		if sb.String() != "[]" {
			t.Fatalf("空数组缩进档 = %q 期望 []", sb.String())
		}
		sb.Reset()
		grtWriteJSON(&sb, grtObjOf(), 2, 1)
		if sb.String() != "{}" {
			t.Fatalf("空对象缩进档 = %q 期望 {}", sb.String())
		}
	})
}

func TestGateReportJSONParseOrderAndEscapes(t *testing.T) {
	t.Run("同名键原位覆盖保首次插入序", func(t *testing.T) {
		v, err := grtJSONParse(`{"b":1,"a":2,"b":3}`)
		if err != nil {
			t.Fatal(err)
		}
		var sb strings.Builder
		grtWriteJSON(&sb, v, -1, 0)
		if sb.String() != `{"b": 3, "a": 2}` {
			t.Fatalf("got %s 期望 {\"b\": 3, \"a\": 2}（py dict 语义）", sb.String())
		}
	})
	t.Run("u转义与代理对", func(t *testing.T) {
		v, err := grtJSONParse(`{"k":"\u4e2d\ud83d\ude00"}`)
		if err != nil {
			t.Fatal(err)
		}
		if v.obj[0].V.s != "中😀" {
			t.Fatalf("解码 = %q 期望 中😀", v.obj[0].V.s)
		}
	})
	t.Run("非法JSON一律拒", func(t *testing.T) {
		for _, bad := range []string{"{not json}", "{}x", "{'a':1}", `{"a":}`, ""} {
			if _, err := grtJSONParse(bad); err == nil {
				t.Fatalf("grtJSONParse(%q) 未拒", bad)
			}
		}
		if _, ok := grtLooksLikeJSON("{not json}"); ok {
			t.Fatal("_looks_like_json 同构：{not json} 不该被收")
		}
		if _, ok := grtLooksLikeJSON("  {}  "); !ok {
			t.Fatal("_looks_like_json 同构：前导空白后 {} 应被收")
		}
	})
}

// ★ 三态各成一格：空集合（键在场且空）· null（键在场为 null）· 取不到（键不在场）。
func TestGateReportNullEmptyAbsentThreeStates(t *testing.T) {
	root := grtTFixture(t, map[string]string{
		"silent-gate.py": "import sys\nsys.exit(0)\n",
	})
	t.Run("空集合_不出现null也不当作取不到", func(t *testing.T) {
		rc, out, errs := grtTRun("--root", root, "--json-only")
		if rc != 0 {
			t.Fatalf("rc=%d stderr=%q", rc, errs)
		}
		if !strings.Contains(out, `"key_lines": []`) {
			t.Fatalf("空集合未显式置空：%s", out)
		}
		if strings.Contains(out, `"key_lines": null`) {
			t.Fatal("空集合被写成 null（py 给 []）")
		}
		if strings.Contains(out, "门禁文件不存在") {
			t.Fatal("空集合档出现了「取不到」文案")
		}
	})
	t.Run("取不到_具名退码且不出现空集合文案", func(t *testing.T) {
		rc, out, errs := grtTRun("--root", root, "--gate", "no-such-gate.py", "--json-only")
		if rc != 1 {
			t.Fatalf("rc=%d stderr=%q", rc, errs)
		}
		if !strings.Contains(out, "门禁文件不存在") {
			t.Fatalf("取不到档未给具名文案：%s", out)
		}
		if strings.Contains(out, `"key_lines": null`) {
			t.Fatal("取不到档的 key_lines 被写成 null（py 给 []）")
		}
	})
	t.Run("键不在场_取不到档无stderr键而正常档有", func(t *testing.T) {
		_, out, _ := grtTRun("--root", root, "--gate", "no-such-gate.py", "--json-only")
		if strings.Contains(out, `"stderr"`) {
			t.Fatalf("取不到档不该有 stderr 键：%s", out)
		}
		if !strings.Contains(out, `"error"`) {
			t.Fatalf("取不到档缺 error 键：%s", out)
		}
		_, out2, _ := grtTRun("--root", root, "--json-only")
		if !strings.Contains(out2, `"stderr"`) {
			t.Fatalf("正常档应有 stderr 键：%s", out2)
		}
	})
	t.Run("null档_只有detail可为null", func(t *testing.T) {
		_, out, _ := grtTRun("--root", root, "--json-only")
		if !strings.Contains(out, `"registered_falses_detail": null`) {
			t.Fatalf("detail null 档缺失：%s", out)
		}
	})
}

func TestGateReportExtractKeyLines(t *testing.T) {
	pat := mustCompileT(t, grtDefaultKeyRegex)
	t.Run("按码点截200并加省略号", func(t *testing.T) {
		got := grtExtractKeyLines("命中 "+strings.Repeat("长", 210), pat)
		if len(got) != 1 {
			t.Fatalf("条数 = %d 期望 1", len(got))
		}
		if n := len([]rune(got[0])); n != grtKeyLineWidth+1 {
			t.Fatalf("截断后码点数 = %d 期望 %d", n, grtKeyLineWidth+1)
		}
	})
	t.Run("去重且保首现序", func(t *testing.T) {
		got := grtExtractKeyLines("命中 a\n命中 b\n命中 a", pat)
		if len(got) != 2 || got[0] != "命中 a" || got[1] != "命中 b" {
			t.Fatalf("去重结果 = %q", got)
		}
	})
	t.Run("上限20条", func(t *testing.T) {
		var sb strings.Builder
		for i := 0; i < 40; i++ {
			sb.WriteString("命中 " + strings.Repeat("x", i+1) + "\n")
		}
		if got := grtExtractKeyLines(sb.String(), pat); len(got) != grtKeyLineCap {
			t.Fatalf("条数 = %d 期望 %d", len(got), grtKeyLineCap)
		}
	})
	t.Run("strip后去重_空白差异算同一条", func(t *testing.T) {
		got := grtExtractKeyLines("命中 a\n  命中 a  \n", pat)
		if len(got) != 1 {
			t.Fatalf("条数 = %d 期望 1", len(got))
		}
	})
}

func TestGateReportRegisteredFalses(t *testing.T) {
	t.Run("非本门一律不判", func(t *testing.T) {
		flag, detail := grtExtractRegisteredFalses("已登记 8 处，均不计命中\n有 1 处为未登记", "check-other.py")
		if flag || detail != nil {
			t.Fatalf("非 check-dangling-refs 门不该判：flag=%v detail=%v", flag, detail)
		}
	})
	t.Run("登记远超未登记判真", func(t *testing.T) {
		flag, detail := grtExtractRegisteredFalses("已登记 8 处，均不计命中\n有 1 处为未登记", "check-dangling-refs")
		if !flag || detail == nil {
			t.Fatalf("应判已登记假红：flag=%v detail=%v", flag, detail)
		}
	})
	t.Run("未登记占比大判假", func(t *testing.T) {
		flag, _ := grtExtractRegisteredFalses("已登记 8 处，均不计命中\n有 3 处为未登记", "check-dangling-refs")
		if flag {
			t.Fatal("8 >= 5*3 不成立 ⇒ 不该判已登记假红")
		}
	})
	t.Run("缺字样不判", func(t *testing.T) {
		if flag, _ := grtExtractRegisteredFalses("有 1 处为未登记", "check-dangling-refs"); flag {
			t.Fatal("缺「已登记/不计命中」字样 ⇒ 不该判")
		}
	})
}

func TestGateReportStderrTail(t *testing.T) {
	got := grtStderrTail(strings.Repeat("A", 700) + strings.Repeat("B", 300))
	if n := len([]rune(got)); n != 500 {
		t.Fatalf("末 500 码点档 = %d 码点", n)
	}
	if got[:200] != strings.Repeat("A", 200) || got[200:] != strings.Repeat("B", 300) {
		t.Fatalf("尾部切片未取末段：前 8 = %q 末 8 = %q", got[:8], got[len(got)-8:])
	}
	if got := grtStderrTail("  x  "); got != "x" {
		t.Fatalf("未 strip：%q", got)
	}
}

// ─── ③ 真口子面 ───────────────────────────────────────────────────────────

func TestGateReportArgparseFaces(t *testing.T) {
	root := grtTFixture(t, map[string]string{"ok-gate.py": "import sys\nsys.exit(0)\n"})
	cases := []struct {
		name string
		args []string
		rc   int
		want string
	}{
		{"未知旗标", []string{"--no-such-flag"}, 2, "unrecognized arguments: --no-such-flag"},
		{"多枚未知", []string{"--foo", "bar"}, 2, "unrecognized arguments: --foo bar"},
		{"前缀歧义", []string{"--js"}, 2, "ambiguous option: --js could match --json, --json-only"},
		{"缺值", []string{"--root"}, 2, "argument --root: expected one argument"},
		{"值形如旗标", []string{"--root", "--list"}, 2, "argument --root: expected one argument"},
		{"store_true带等号", []string{"--json=1"}, 2, "argument --json: ignored explicit argument '1'"},
		{"帮助带等号", []string{"--help=x"}, 2, "argument -h/--help: ignored explicit argument 'x'"},
		{"短簇带参数", []string{"-hx"}, 2, "argument -h/--help: ignored explicit argument 'x'"},
		{"位置参数", []string{"foo"}, 2, "unrecognized arguments: foo"},
		{"裸双横", []string{"--"}, 2, "unrecognized arguments: --"},
		{"双横两项", []string{"--", "--"}, 2, "unrecognized arguments: -- --"},
		{"双竖线后全入未识别", []string{"--list", "--", "foo"}, 2, "unrecognized arguments: -- foo"},
		{"不在仓根", []string{"--root", grtPyJoin(root, "nope")}, 2, "不在仓根"},
		{"清单为空", []string{"--root", grtTFixture(t, map[string]string{})}, 2, "门禁清单为空"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rc, out, errs := grtTRun(c.args...)
			if rc != c.rc {
				t.Fatalf("args=%v rc=%d 期望 %d（stdout=%q stderr=%q）", c.args, rc, c.rc, out, errs)
			}
			if !strings.Contains(errs, c.want) {
				t.Fatalf("args=%v stderr=%q 未含 %q", c.args, errs, c.want)
			}
		})
	}
	t.Run("帮助先于未知项", func(t *testing.T) {
		rc, out, _ := grtTRun("--no-such-flag", "-h")
		if rc != 0 || len(out) != grtTHelpBytes {
			t.Fatalf("rc=%d 帮助字节=%d", rc, len(out))
		}
	})
	t.Run("缩写唯一即命中", func(t *testing.T) {
		rc, out, errs := grtTRun("--ro", root, "--gate", "ok-gate.py")
		if rc != 0 || errs != "" {
			t.Fatalf("rc=%d stderr=%q out=%q", rc, errs, out)
		}
	})
}

// D 类死旗标：py 注册 --json 但正文从不读 ⇒ 输出仍走 human 面。★ 若日后有人把它做活，本用例立刻红。
func TestGateReportDeadFlagJSON(t *testing.T) {
	root := grtTFixture(t, map[string]string{"ok-gate.py": "import sys\nsys.exit(0)\n"})
	_, out, _ := grtTRun("--root", root, "--gate", "ok-gate.py", "--json")
	if strings.HasPrefix(strings.TrimSpace(out), "{") {
		t.Fatalf("--json 竟产出 JSON 本体（死旗标被激活）：%q", out)
	}
	if !strings.Contains(out, "gate-report · 仓") {
		t.Fatalf("--json 未走 human 面：%q", out)
	}
}

// C 类最外层无兜底：py 侧 `--key-regex '['` 会 re.error 回溯 rc=1；本件按新口径 rc=2 fail-closed。
func TestGateReportInvalidKeyRegexFailClosed(t *testing.T) {
	root := grtTFixture(t, map[string]string{"ok-gate.py": "import sys\nsys.exit(0)\n"})
	rc, out, errs := grtTRun("--root", root, "--gate", "ok-gate.py", "--key-regex", "[")
	if rc != 2 {
		t.Fatalf("rc=%d 期望 2（fail-closed）", rc)
	}
	if out != "" {
		t.Fatalf("不给结论档 stdout 应为空：%q", out)
	}
	if !strings.Contains(errs, "--key-regex 不可编译") {
		t.Fatalf("缺具名文案：%q", errs)
	}
}

// 超时档（py timeout=600；本件走 grtGateTimeout 可调，单测缩短以真跑该分支）。
func TestGateReportTimeoutBranch(t *testing.T) {
	root := grtTFixture(t, map[string]string{"slow-gate.py": "import time\ntime.sleep(5)\n"})
	old := grtGateTimeout
	grtGateTimeout = 120 * time.Millisecond
	defer func() { grtGateTimeout = old }()
	start := time.Now()
	rc, out, _ := grtTRun("--root", root, "--gate", "slow-gate.py", "--json-only")
	if d := time.Since(start); d > 4*time.Second {
		t.Fatalf("超时未生效，耗时 %v", d)
	}
	if rc != 1 {
		t.Fatalf("rc=%d 期望 1（门 rc=2 ⇒ 有待办）", rc)
	}
	if !strings.Contains(out, grtTimeoutTail) {
		t.Fatalf("缺超时文案：%s", out)
	}
	if !strings.Contains(out, `"errored": true`) {
		t.Fatalf("超时档未标 errored：%s", out)
	}
}

// 非 UTF-8 门禁 stdout：py 侧 UnicodeDecodeError ⇒ rc=2 errored（本件 rc/正文对齐，措辞尽力复刻）。
func TestGateReportNonUTF8Output(t *testing.T) {
	root := grtTFixture(t, map[string]string{
		"bad-gate.py": "import sys\nsys.stdout.buffer.write(b\"\\xff\")\nsys.stdout.buffer.flush()\n",
	})
	rc, out, _ := grtTRun("--root", root, "--gate", "bad-gate.py", "--json-only")
	if rc != 1 {
		t.Fatalf("rc=%d 期望 1", rc)
	}
	if !strings.Contains(out, "codec can") || !strings.Contains(out, "invalid start byte") {
		t.Fatalf("未复刻解码错文案：%s", out)
	}
	if !strings.Contains(out, `"errored": true`) {
		t.Fatalf("未标 errored：%s", out)
	}
}

// A 类自检夹具泄漏面：py 用 mkdtemp + finally rmtree（**本身回收**）⇒ 本件同法回收。
// ★ 本用例有牙：删掉主件的 defer os.RemoveAll(tmp) ⇒ after = before+1 ⇒ FAIL 带行号。
func TestGateReportSelfTestRecyclesTempDirs(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir()) // GAP-20261003-16：私有计数面（只数本用例自建 tmp 目录）
	t.Setenv(grtTReexecEnv, "1")
	before := grtTCountSelfTestDirs(t)
	rc, out, errs := grtTRun("--self-test")
	after := grtTCountSelfTestDirs(t)
	if rc != 0 {
		t.Fatalf("自检 rc=%d 期望 0（stdout=%q stderr=%q）", rc, out, errs)
	}
	if !strings.Contains(out, "全过 ✓") {
		t.Fatalf("自检未全过：%q", out)
	}
	if after != before {
		t.Fatalf("自检夹具未回收：before=%d after=%d（主件缺 defer os.RemoveAll）", before, after)
	}
}

func TestGateReportSelfTestThreeGears(t *testing.T) {
	t.Setenv(grtTReexecEnv, "1")
	rc, out, _ := grtTRun("--self-test")
	if rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	for _, want := range []string{"档① 正控", "档② 负控", "档③ 用法错"} {
		if !strings.Contains(out, want) {
			t.Fatalf("缺档 %q：%q", want, out)
		}
	}
	rc, out, _ = grtTRun("--selftest")
	if rc != 0 || !strings.Contains(out, "三档") {
		t.Fatalf("--selftest 别名档 rc=%d out=%q", rc, out)
	}
}

// 真跑子进程面：逐门真取退出码（含 rc=2 档 ⇒ 状态列 ERR）+ 关键行 + stderr 末行。
func TestGateReportRunsRealGates(t *testing.T) {
	root := grtTFixture(t, map[string]string{
		"ok-gate.py":   "import sys\nprint('结论：全部通过 rc=0')\nsys.exit(0)\n",
		"warn-gate.py": "import sys\nprint('结论：判不了')\nsys.exit(2)\n",
		"bad-gate.py":  "import sys\nprint('结论：有未收敛项')\nsys.stderr.write('末行警告\\n')\nsys.exit(1)\n",
	})
	rc, out, _ := grtTRun("--root", root)
	if rc != 1 {
		t.Fatalf("rc=%d 期望 1", rc)
	}
	if !strings.Contains(out, "PASS") || !strings.Contains(out, "ERR") || !strings.Contains(out, "PENDING") {
		t.Fatalf("三状态列缺一：%q", out)
	}
	if !strings.Contains(out, "待办门禁：bad-gate.py, warn-gate.py") {
		t.Fatalf("待办点名序应为 --gate 缺省下的 sorted 门名序：%q", out)
	}
	if !strings.Contains(out, "        ! 末行警告") {
		t.Fatalf("缺 stderr 末行档：%q", out)
	}
	if !strings.Contains(out, "        · 结论：全部通过 rc=0") {
		t.Fatalf("缺关键行档：%q", out)
	}
}

func TestGateReportListSkipsSelfAndSh(t *testing.T) {
	root := grtTFixture(t, map[string]string{
		"a-gate.py":         "import sys\nsys.exit(0)\n",
		"gate-report.py":    "import sys\nsys.exit(0)\n",
		"who-references.py": "import sys\nsys.exit(0)\n",
		"z-gate.sh":         "#!/bin/sh\nexit 0\n",
	})
	rc, out, _ := grtTRun("--root", root, "--list")
	if rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	if out != "a-gate.py\n" {
		t.Fatalf("列名 = %q 期望只 a-gate.py（自排除 + 跳过 .sh）", out)
	}
}

func mustCompileT(t *testing.T, re string) *regexp.Regexp {
	t.Helper()
	p, err := regexp.Compile(re)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func grtTCountSelfTestDirs(t *testing.T) int {
	t.Helper()
	ents, err := os.ReadDir(os.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), "gatereport-selftest-") {
			n++
		}
	}
	return n
}
