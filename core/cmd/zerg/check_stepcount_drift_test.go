// check_stepcount_drift_test.go —— 与 `check_stepcount_drift.go` 成对（M2 · 搬运面 搬运粒17）。
//
// 定位：**成对负控**（✗ 禁恒绿）—— 每一格都钉住 py 的一条可观测行为，实现回退即红。
// 用例前缀 `TestScd`（现读 `code find 'func TestScd'` 零命中 ⇒ 不与在仓测试件撞名）。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// scdRunMain —— 进程内跑入口，收 rc/stdout/stderr 三面。
func scdRunMain(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := stepcountDriftGateMain(&out, &errb, args)
	return rc, out.String(), errb.String()
}

// scdWrite —— 落一枚夹具件（夹具恒落 t.TempDir ⇒ 自回收、不进仓树）。
func scdWrite(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestScdHelpAndArgparse —— argparse 面：帮助 / 未知旗标 / 缺值 / 歧义 / 显式值拒收 / 前缀缩写。
func TestScdHelpAndArgparse(t *testing.T) {
	rc, out, errb := scdRunMain(t, "-h")
	if rc != 0 || errb != "" || !strings.HasPrefix(out, "usage: check-stepcount-drift.py [-h] [--script SCRIPT] [--json] [--self-test]\n") {
		t.Fatalf("help rc=%d out=%q err=%q", rc, out, errb)
	}
	if !strings.Contains(out, "optional arguments:\n  -h, --help       show this help message and exit\n  --script SCRIPT\n  --json\n  --self-test\n") {
		t.Fatalf("help 正文与 py 3.9 不符: %q", out)
	}

	rc, _, errb = scdRunMain(t, "--bogus")
	if rc != 2 || errb != scdUsage+"check-stepcount-drift.py: error: unrecognized arguments: --bogus\n" {
		t.Fatalf("bogus rc=%d err=%q", rc, errb)
	}

	rc, _, errb = scdRunMain(t, "--script")
	if rc != 2 || !strings.Contains(errb, "check-stepcount-drift.py: error: argument --script: expected one argument\n") {
		t.Fatalf("noval rc=%d err=%q", rc, errb)
	}

	rc, _, errb = scdRunMain(t, "--s")
	if rc != 2 || !strings.Contains(errb, "error: ambiguous option: --s could match --script, --self-test\n") {
		t.Fatalf("ambiguous rc=%d err=%q", rc, errb)
	}

	rc, _, errb = scdRunMain(t, "--help=x")
	if rc != 2 || !strings.Contains(errb, "argument -h/--help: ignored explicit argument 'x'") {
		t.Fatalf("help=x rc=%d err=%q", rc, errb)
	}

	rc, _, errb = scdRunMain(t, "--json=1")
	if rc != 2 || !strings.Contains(errb, "argument --json: ignored explicit argument '1'") {
		t.Fatalf("json=1 rc=%d err=%q", rc, errb)
	}

	rc, out, _ = scdRunMain(t, "--self-") // 唯一前缀 ⇒ 命中 --self-test
	if rc != 0 || !strings.HasPrefix(out, "步数常量扫描 · 自检（合成门件夹具 · 临时树 · 成对正负控）\n") {
		t.Fatalf("--self- rc=%d out=%.60q", rc, out)
	}
}

// TestScdJudgeBody —— 判据体：正控 0 漂移 · 负控点名行 · 降噪（泛化数字只报不判）· 混合只挑规范条。
func TestScdJudgeBody(t *testing.T) {
	dir := t.TempDir()

	d := scdJudge(scdWrite(t, dir, "pos.sh", scdSPos))
	if d.rc != 0 || len(d.canon) != 1 || len(d.drift) != 0 || d.nArr != 3 {
		t.Fatalf("正控 rc=%d canon=%d drift=%d nArr=%d", d.rc, len(d.canon), len(d.drift), d.nArr)
	}

	d = scdJudge(scdWrite(t, dir, "neg.sh", scdSNeg))
	if d.rc != 1 || len(d.drift) != 1 || d.drift[0].line != 2 || d.drift[0].num != "4" {
		t.Fatalf("负控 rc=%d drift=%+v", d.rc, d.drift)
	}

	d = scdJudge(scdWrite(t, dir, "noise.sh", scdSNoise))
	if d.rc != 0 || len(d.canon) != 0 || d.generic == 0 {
		t.Fatalf("降噪负控 rc=%d canon=%d generic=%d", d.rc, len(d.canon), d.generic)
	}

	d = scdJudge(scdWrite(t, dir, "mixed.sh", scdSMixed))
	if d.rc != 1 || len(d.drift) != 1 || d.drift[0].line != 3 {
		t.Fatalf("混合负控 rc=%d drift=%+v", d.rc, d.drift)
	}

	d = scdJudge(scdWrite(t, dir, "noarr.sh", scdSNoArr))
	if d.rc != 2 || d.reason != "array-unparseable" {
		t.Fatalf("无数组负控 rc=%d reason=%q", d.rc, d.reason)
	}
}

// TestScdEndToEnd —— 入口端到端：好件 rc=0 / 漂移 rc=1 / 缺件 rc=2 / --json 面。
func TestScdEndToEnd(t *testing.T) {
	dir := t.TempDir()
	pos := scdWrite(t, dir, "pos.sh", scdSPos)
	neg := scdWrite(t, dir, "neg.sh", scdSNeg)

	rc, out, errb := scdRunMain(t, "--script", pos)
	if rc != 0 || errb != "" || !strings.Contains(out, "  FAST_STEPS 数组现读长度 = 3\n") ||
		!strings.Contains(out, "    （无 —— 规范计数与数组一致）\n") {
		t.Fatalf("pos rc=%d out=%q err=%q", rc, out, errb)
	}

	rc, out, errb = scdRunMain(t, "--script", neg)
	if rc != 1 || errb != "" || !strings.Contains(out, neg+":2  数=4  ← # 快速档：`FAST_STEPS` 名单长度 = 4 步（登记面 · 写错）\n") {
		t.Fatalf("neg rc=%d out=%q err=%q", rc, out, errb)
	}

	rc, out, errb = scdRunMain(t, "--script", filepath.Join(dir, "nope.sh"))
	if rc != 2 || out != "" || !strings.Contains(errb, "读不到门禁件：") {
		t.Fatalf("missing rc=%d out=%q err=%q", rc, out, errb)
	}
}

// TestScdJSONFace —— JSON 面：键序/分隔/空集合 `[]`/drift 布尔（✗ 不落 null）。
func TestScdJSONFace(t *testing.T) {
	dir := t.TempDir()
	empty := scdWrite(t, dir, "empty.sh", scdArr3) // 只有数组、无规范计数 ⇒ 两个集合都空
	rc, out, _ := scdRunMain(t, "--json", "--script", empty)
	want := `{"script": ` + scdJSONStr(empty) + `, "fast_array_len": 3, "canonical_hits": [], "drift_vs_array": [], "generic_count_suppressed": 0, "drift": false}` + "\n"
	if rc != 0 || out != want {
		t.Fatalf("empty json rc=%d\n got %q\nwant %q", rc, out, want)
	}

	neg := scdWrite(t, dir, "neg.sh", scdSNeg)
	rc, out, _ = scdRunMain(t, "--json", "--script", neg)
	if rc != 1 || !strings.Contains(out, `"drift": true`) || !strings.Contains(out, `"drift_vs_array": [{"line": 2, "num": "4",`) {
		t.Fatalf("neg json rc=%d out=%q", rc, out)
	}
}

// TestScdSelfTest —— 自检入口：12 断言全过 rc=0，且印「断言 12 条 · 失败 0 条」。
func TestScdSelfTest(t *testing.T) {
	rc, out, errb := scdRunMain(t, "--self-test")
	if errb != "" {
		t.Fatalf("err=%q", errb)
	}
	if rc != 0 {
		t.Fatalf("rc=%d out=%q", rc, out)
	}
	if !strings.Contains(out, "步数常量扫描 · 自检（合成门件夹具 · 临时树 · 成对正负控）\n") ||
		!strings.Contains(out, "── 自检结论：断言 12 条 · 失败 0 条 ──\n") {
		t.Fatalf("自检正文: %q", out)
	}
}

// TestScdPyStripUniversalNewlines —— 自写 pyStrip（剥 py Unicode 空白 · 含 `\x1c`/`\xa0`）与 universal newlines。
func TestScdPyStripUniversalNewlines(t *testing.T) {
	if got := scdPyStrip("\x1c\u00a0 	FAST_STEPS\x0b\x0c\r\n"); got != "FAST_STEPS" {
		t.Fatalf("pyStrip=%q", got)
	}
	if got := scdPyStrip(" x "); got != "x" {
		t.Fatalf("pyStrip ascii=%q", got)
	}
	dir := t.TempDir()
	p := scdWrite(t, dir, "crlf.sh", "a\r\nb\rc\nd")
	lines, err := scdReadScript(p)
	if err != nil || len(lines) != 4 || lines[0] != "a" || lines[1] != "b" || lines[2] != "c" || lines[3] != "d" {
		t.Fatalf("readscript=%q err=%v", lines, err)
	}
	// ★ `read()` 后是 `.split("\n")` ⇒ 尾换行产生**一个空尾段**（与 splitlines 不同）。
	if lines, _ := scdReadScript(scdWrite(t, dir, "tail.sh", "x\n")); len(lines) != 2 || lines[1] != "" {
		t.Fatalf("尾空段: %q", lines)
	}
}

// TestScdUnicodeDigitRegex —— `\d`→`\p{Nd}` 展开的牙：py `\d` 认 Unicode 数字（若改回 Go ASCII `\d` ⇒ 本格红）。
func TestScdUnicodeDigitRegex(t *testing.T) {
	// 阿拉伯-印度数字 ٣ 是 `\p{Nd}`；py `\d` 认、ASCII `\d` 不认。
	if !scdNumStep.MatchString("٣步") {
		t.Fatalf("NUM_STEP 未认 Unicode 数字 ⇒ `\\p{Nd}` 展开被回退")
	}
	if !scdArrow.MatchString("→ ٤") {
		t.Fatalf("ARROW 未认 Unicode 数字")
	}
	// 规范计数里的数字是 Unicode 数字 ⇒ 命中（num 为 `٣`，与 str(n_arr) 不等 ⇒ 判漂移）。
	body := "#!/bin/sh\n# `FAST_STEPS` 名单长度 = ٣ 步\n" + scdArr3
	d := scdJudge(scdWrite(t, t.TempDir(), "u.sh", body))
	if len(d.canon) != 1 || d.canon[0].num != "٣" || d.rc != 1 {
		t.Fatalf("Unicode 数字 canon=%+v rc=%d", d.canon, d.rc)
	}
}

// TestScdEmptyVsAbsent —— ★ nil 不得兼表「空集合」与「取不到」：空规范计数 ⇒ rc=0（非取不到），缺件 ⇒ rc=2。
func TestScdEmptyVsAbsent(t *testing.T) {
	dir := t.TempDir()
	d := scdJudge(scdWrite(t, dir, "empty.sh", scdArr3))
	if len(d.canon) != 0 || d.rc != 0 {
		t.Fatalf("空集合档：canon=%d rc=%d（应 rc=0：数组在盘、只是无规范计数）", len(d.canon), d.rc)
	}
	d = scdJudge(filepath.Join(dir, "absent.sh"))
	if d.rc != 2 || d.reason != "script-absent" {
		t.Fatalf("缺件档 rc=%d reason=%q", d.rc, d.reason)
	}
}
