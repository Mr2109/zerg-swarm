// check_links_lychee_test.go —— `checkLinksLycheeGateMain` 的**成对负控**用例（M2 · 甲案 C 类第 14 枚）。
//
// 覆盖四条硬面（每条都带能抳住它的牙）：
//
//	① py 路径/字符串原语的三类语义差各钉一格（join 吃 `..` 字面 · 后段绝对丢前段 · 空段留尾 `/`；
//	   normpath 保留恰两个前导斜杠；strip 剥 `\x1c-\x1f`；relpath 空 path ⇒ py 抛 ValueError）。
//	② ★ **nil 不得兼表「取不到」与「空集合」两条牙成对钉死**：空清单 ⇒ 只判「0 件」；清单取不到 ⇒
//	   只判「读不了」；两态的输出必须**互不出现**对方的文案（只钉一条会修过头）。
//	③ 运行档的**机器可核面**：`-h` 正文逐字 · 未知旗标 usage+error 逐字 · `--json`（Q-176）三行逐字 ·
//	   `--root` 缺参的 argparse 文案 · lychee **命令串**与**退码映射**（rc ∉ {0,2} ⇒ 不给结论）。
//	④ 输出序的决定者 = `broken_records` 末尾的 `sorted(key=(rel, str(line), str(column), url))` ——
//	   按 **`str()`** 排（行号 10 排在 9 之前），用插入序 (9,2,10) 的错表钉一格。
//
// ★ GAP-20261001-61 适配判断：本测试件**不起任何 python 子进程**（全部 in-process 调 Go 实现 ·
// 假 lychee 是 `#!/bin/sh` 的幂替身）⇒ 「钉 `/usr/bin/python3`」这条**不适用**（理由：本件无 python 依赖）。
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func cllTRun(t *testing.T, cwd string, argv ...string) (int, string, string) {
	t.Helper()
	var so, se bytes.Buffer
	rc := cllMain(&so, &se, argv, cwd)
	return rc, so.String(), se.String()
}

func cllTWrite(t *testing.T, path, body string) string {
	t.Helper()
	if dir := cllPyDirname(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("夹具建目录失败：%v", err)
		}
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("夹具写件失败：%v", err)
	}
	return path
}

// cllTMkLychee —— 落一枚**仓外幂替身**（`/bin/sh` 假 lychee），并把 PATH 钉到它所在目录。
// 返回 argv 落点。✗ 不联网、✗ 不起真 lychee。
func cllTMkLychee(t *testing.T, jsonBody string, rc int, stderr string) string {
	t.Helper()
	dir := t.TempDir()
	body := "#!/bin/sh\n" +
		"last=\"\"; for a in \"$@\"; do last=\"$a\"; done\n" +
		"if [ -n \"$FAKE_LYCHEE_LOG\" ]; then : > \"$FAKE_LYCHEE_LOG\"; for a in \"$@\"; do printf '%s\\n' \"$a\" >> \"$FAKE_LYCHEE_LOG\"; done; printf '%s\\n' --LIST-- >> \"$FAKE_LYCHEE_LOG\"; while IFS= read -r cllL || [ -n \"$cllL\" ]; do printf '%s\\n' \"$cllL\" >> \"$FAKE_LYCHEE_LOG\"; done < \"$last\"; fi\n" +
		"if [ -n \"$FAKE_LYCHEE_STDERR\" ]; then printf '%s' \"$FAKE_LYCHEE_STDERR\" >&2; fi\n" +
		"if [ -n \"$FAKE_LYCHEE_JSON\" ]; then while IFS= read -r cllL || [ -n \"$cllL\" ]; do printf '%s\\n' \"$cllL\"; done < \"$FAKE_LYCHEE_JSON\"; fi\n" +
		"exit \"${FAKE_LYCHEE_RC:-0}\"\n"
	if err := os.WriteFile(cllPyJoin(dir, "lychee"), []byte(body), 0o755); err != nil {
		t.Fatalf("假 lychee 落不了：%v", err)
	}
	jf := cllTWrite(t, cllPyJoin(dir, "canned.json"), jsonBody)
	logf := cllPyJoin(dir, "argv.log")
	t.Setenv("PATH", dir)
	t.Setenv("FAKE_LYCHEE_JSON", jf)
	t.Setenv("FAKE_LYCHEE_RC", cllPyStr(json.Number(cllItoa(rc))))
	t.Setenv("FAKE_LYCHEE_STDERR", stderr)
	t.Setenv("FAKE_LYCHEE_LOG", logf)
	return logf
}

func cllItoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		return "-" + string(b)
	}
	return string(b)
}

// ── ① 路径/字符串原语：每个已知语义差各钉一格 ──────────────────────────────────────────────

func TestCllPathPrimitives(t *testing.T) {
	cases := []struct{ name, got, want string }{
		{"join 吃 `..` 字面", cllPyJoin("/a", "../b"), "/a/../b"},
		{"join 后段绝对丢前段", cllPyJoin("repo", "/abs"), "/abs"},
		{"join 空段留尾 `/`", cllPyJoin("a", ""), "a/"},
		{"join 中间空段", cllPyJoin("a", "", "b"), "a/b"},
		{"normpath 保留恰两个前导斜杠", cllPyNormpath("//a/b"), "//a/b"},
		{"normpath 三个前导斜杠折一个", cllPyNormpath("///a/b"), "/a/b"},
		{"normpath 空串 ⇒ `.`", cllPyNormpath(""), "."},
		{"dirname 剥尾斜杠", cllPyDirname("/a/b/"), "/a/b"},
		{"strip 剥 \\x1c", cllPyStrip("\x1c x \x1c"), "x"},
		{"abspath = normpath∘join（`..` 归一）", cllPyAbspath("/cwd", "../x"), "/x"},
		{"abspath 纯词法（不做盘上校验）", cllPyAbspath("/cwd", "/no-such-dir-zzz/x"), "/no-such-dir-zzz/x"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Fatalf("%s：实得 %q（要 %q）", c.name, c.got, c.want)
		}
	}
	if got := cllPySplitLines("a\r\nb\r\rc\n"); strings.Join(got, "|") != "a|b||c" {
		t.Fatalf("splitlines：实得 %q（要 a|b||c）", got)
	}
	rel, ok := cllPyRelpath("/r", "/r/a/b", "/r")
	if !ok || rel != "a/b" {
		t.Fatalf("relpath：实得 (%q,%v)（要 (a/b,true)）", rel, ok)
	}
	if _, ok := cllPyRelpath("/r", "", "/r"); ok {
		t.Fatalf("relpath 空 path：应判 ok=false（py 抛 ValueError）")
	}
}

// ── ② 空集合 / 取不到：**成对**两条牙 ──────────────────────────────────────────────────────

func TestCllEmptyVsMissingLists(t *testing.T) {
	dir := t.TempDir()
	empty := cllTWrite(t, cllPyJoin(dir, "empty.txt"), "# 注释行不算件\n\n")
	rcE, outE, _ := cllTRun(t, dir, "--files", empty)
	if rcE != 2 || !strings.Contains(outE, "扫面 **0 件**") {
		t.Fatalf("空集合档：rc=%d out=%q（要 rc=2 且含「0 件」）", rcE, outE)
	}
	if strings.Contains(outE, "清单读不了") {
		t.Fatalf("空集合档被误判成「取不到」：out=%q", outE)
	}

	rcM, outM, _ := cllTRun(t, dir, "--files", cllPyJoin(dir, "nope.txt"))
	if rcM != 2 || !strings.Contains(outM, "清单读不了（") || !strings.Contains(outM, "[Errno 2]") {
		t.Fatalf("取不到档：rc=%d out=%q（要 rc=2 且含「清单读不了（…[Errno 2]」）", rcM, outM)
	}
	if strings.Contains(outM, "扫面 **0 件**") {
		t.Fatalf("取不到档被误判成「空集合」：out=%q", outM)
	}
}

// ── ③ 运行档机器可核面 ────────────────────────────────────────────────────────────────────

func TestCllHelpAndUsageVerbatim(t *testing.T) {
	dir := t.TempDir()
	rc, out, errS := cllTRun(t, dir, "-h")
	if rc != 0 || errS != "" || out != cllHelpText {
		t.Fatalf("help 档：rc=%d err=%q out==帮助正文? %v", rc, errS, out == cllHelpText)
	}
	rc, out, errS = cllTRun(t, dir, "--bogus")
	wantErr := cllUsageLine + cllProg + ": error: unrecognized arguments: --bogus\n"
	if rc != 2 || out != "" || errS != wantErr {
		t.Fatalf("未知旗标档：rc=%d out=%q err=%q（要 rc=2 且 err 逐字 usage+error）", rc, out, errS)
	}
	rc, _, errS = cllTRun(t, dir, "--root")
	if rc != 2 || !strings.Contains(errS, "argument --root: expected one argument") {
		t.Fatalf("缺参档：rc=%d err=%q", rc, errS)
	}
}

func TestCllRefuseJSONQ176(t *testing.T) {
	rc, out, errS := cllTRun(t, t.TempDir(), "--json")
	want := "⛔ 本门无机器读面：`--json` 在本门**未实现**（缺口 Q-176）\n" +
		"   字段表：无（本门只出人读面）\n" +
		"⇒ 用法错（rc=2）：**机器面缺 ≠ 零命中** —— 两态不许同形\n"
	if rc != 2 || out != "" || errS != want {
		t.Fatalf("--json 档：rc=%d out=%q err=%q", rc, out, errS)
	}
}

// ── ④ 输出序决定者：按 str(line) 排（10 < 2 < 9 的字面序）────────────────────────────────

func TestCllBrokenRecordsSortByStrLine(t *testing.T) {
	dir := t.TempDir()
	src := cllPyJoin(dir, "a.md")
	raw := "{\"error_map\":{\"" + src + "\":[" +
		"{\"span\":{\"line\":9,\"column\":1},\"url\":\"file://" + dir + "/x9.md\"}," +
		"{\"span\":{\"line\":2,\"column\":7},\"url\":\"file://" + dir + "/x2.md\"}," +
		"{\"span\":{\"line\":10,\"column\":3},\"url\":\"file://" + dir + "/x10.md\"}]}}"
	var v any
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		t.Fatalf("造样本 JSON 失败：%v", err)
	}
	recs, errS := cllBrokenRecords(v.(map[string]any), dir, dir)
	if errS != "" {
		t.Fatalf("broken_records 报错：%q", errS)
	}
	got := []string{}
	for _, r := range recs {
		got = append(got, r.line)
	}
	if strings.Join(got, ",") != "10,2,9" {
		t.Fatalf("错条目序：实得 %v（要 10,2,9 —— 按 str(line) 排，不是数值排）", got)
	}
	for _, r := range recs {
		if !strings.HasPrefix(r.url, "/") || strings.Contains(r.url, "file://") {
			t.Fatalf("url 未去 `file://`：%q", r.url)
		}
	}
}

// ── ③b lychee 命令串与退码映射（仓外幂替身）──────────────────────────────────────────────

func TestCllLycheeArgsAndRCMapping(t *testing.T) {
	dir := t.TempDir()
	f := cllTWrite(t, cllPyJoin(dir, "a.md"), "# a\n")
	logf := cllTMkLychee(t, "{\"errors\":0,\"error_map\":{},\"total\":0,\"unique\":0,\"successful\":0,\"excludes\":0}", 0, "")

	data, rc, errS := cllRunLychee([]string{f})
	if errS != "" || rc != 0 || data == nil {
		t.Fatalf("跑假 lychee：rc=%d err=%q data==nil? %v", rc, errS, data == nil)
	}
	raw, rerr := os.ReadFile(logf)
	if rerr != nil {
		t.Fatalf("argv 回执读不了：%v", rerr)
	}
	lines := cllPySplitLines(string(raw))
	want := []string{"--offline", "--no-progress", "--format", "json", "--files-from"}
	if len(lines) != 8 {
		t.Fatalf("argv 回执条数：实得 %d（%v）", len(lines), lines)
	}
	for i, w := range want {
		if lines[i] != w {
			t.Fatalf("argv[%d]：实得 %q（要 %q）", i, lines[i], w)
		}
	}
	if lines[6] != "--LIST--" || lines[7] != f {
		t.Fatalf("--files-from 清单正文：实得 marker=%q 内容=%q（要 --LIST-- / %q）", lines[6], lines[7], f)
	}

	// 退码 ∉ {0,2} ⇒ 不给结论
	cllTMkLychee(t, "", 3, "fatal: kaboom")
	_, rc2, err2 := cllRunLychee([]string{f})
	if rc2 != 3 || !strings.Contains(err2, "不在 {0, 2}") || !strings.Contains(err2, "fatal: kaboom") {
		t.Fatalf("退码映射档：rc=%d err=%q", rc2, err2)
	}
}

// ── ④b 登记豁免口：命中（现读存在性背书）⟷ 落空（照旧判红）──────────────────────────────

func TestCllOffsetExemptionBothSides(t *testing.T) {
	dir := t.TempDir()
	cllTWrite(t, cllPyJoin(dir, "publish", "docs", "QUICKSTART.en.md"), "# q\n")
	src := cllTWrite(t, cllPyJoin(dir, "publish", "docs", "README.md"), "# r\n")
	hit := cllRec{rel: "publish/docs/README.md", fname: src, line: "3", col: "1",
		url: cllPyJoin(dir, "publish", "docs", "docs", "QUICKSTART.en.md")}
	why, ok := cllOffsetOK(hit, dir, dir)
	if !ok || !strings.Contains(why, "复核命中") {
		t.Fatalf("豁免命中档：ok=%v why=%q", ok, why)
	}
	miss := hit
	miss.url = cllPyJoin(dir, "publish", "docs", "docs", "NOPE.md")
	if _, ok := cllOffsetOK(miss, dir, dir); ok {
		t.Fatalf("豁免落空档：被误判成命中（豁免只挡「基准错」这一半）")
	}
	if _, ok := cllOffsetOK(cllRec{rel: "a.md", fname: src}, dir, dir); ok {
		t.Fatalf("未登记件：不该进豁免口")
	}
}

// ── ④c 登记过期（登记表的牙）：件在域内 0 命中 ⇒ rc=2 且逐字点名 ─────────────────────────

func TestCllRegistryStaleIsRed(t *testing.T) {
	dir := t.TempDir()
	cllTWrite(t, cllPyJoin(dir, "publish", "docs", "QUICKSTART.en.md"), "# q\n")
	cllTWrite(t, cllPyJoin(dir, "publish", "docs", "README.md"), "# r\n\n[q](QUICKSTART.en.md)\n")
	lst := cllTWrite(t, cllPyJoin(dir, "l.txt"), cllPyJoin(dir, "publish", "docs", "README.md")+"\n")
	cllTMkLychee(t, "{\"errors\":0,\"error_map\":{},\"total\":0,\"unique\":0,\"successful\":0,\"excludes\":0}", 0, "")
	rc, out := cllJudge(dir, "", lst, true, dir)
	if rc != 2 || !strings.Contains(strings.Join(out, "\n"), "登记过期") {
		t.Fatalf("登记过期档：rc=%d out=%q（要 rc=2 且点名登记过期）", rc, out)
	}
}

// ── ④d 全绿档：无错表 ⇒ rc=0 且尾行逐字 ─────────────────────────────────────────────────

func TestCllAllGreenTailLine(t *testing.T) {
	dir := t.TempDir()
	f := cllTWrite(t, cllPyJoin(dir, "a.md"), "# a\n")
	lst := cllTWrite(t, cllPyJoin(dir, "l.txt"), f+"\n")
	cllTMkLychee(t, "{\"errors\":0,\"error_map\":{},\"total\":1,\"unique\":1,\"successful\":1,\"excludes\":0}", 0, "")
	rc, out := cllJudge(dir, "", lst, false, dir)
	if rc != 0 {
		t.Fatalf("全绿档 rc=%d（要 0）out=%q", rc, out)
	}
	if out[len(out)-1] != "⇒ 无本地断链（errors 0）· rc=0" {
		t.Fatalf("全绿尾行：实得 %q", out[len(out)-1])
	}
	if !strings.Contains(out[0], "扫面 1 件 · lychee 退 0 · links 1（unique 1 · successful 1 · excludes 0）") {
		t.Fatalf("全绿首行：实得 %q", out[0])
	}
}
