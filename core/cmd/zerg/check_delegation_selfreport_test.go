package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// cdsrRunArgs —— 进程内复用入口（同 argv 口径：不含程序名）。
func cdsrRunArgs(args ...string) (int, string, string) {
	var so, se strings.Builder
	rc := checkDelegationSelfreportGateMain(&so, &se, args)
	return rc, so.String(), se.String()
}

// TestCdsrSelfTestAllGreen —— `--self-test` 逐字（含 %r 复刻）+ rc=0。
func TestCdsrSelfTestAllGreen(t *testing.T) {
	rc, out, errOut := cdsrRunArgs("--self-test")
	if rc != 0 {
		t.Fatalf("rc=%d want 0 (stderr=%q)", rc, errOut)
	}
	first := "  ✓ Go 体变检测（gapFoundDays 被改）（got=(['gapFoundDays'], [], ['gapQidOf']) · want=(['gapFoundDays'], [], ['gapQidOf'])）\n"
	if !strings.Contains(out, first) {
		t.Fatalf("自检首行失配：got=%q", out)
	}
	if !strings.HasSuffix(out, "自检结论: 全过\n") {
		t.Fatalf("自检结论行失配：%q", out)
	}
}

// TestCdsrListRules —— `--list-rules` 正文逐字。
func TestCdsrListRules(t *testing.T) {
	rc, out, _ := cdsrRunArgs("--list-rules")
	if rc != 0 {
		t.Fatalf("rc=%d want 0", rc)
	}
	want := "登记面：\n" +
		"  R1 自述件可读可解析（缺件/非 JSON/缺 declared ⇒ rc=2）\n" +
		"  R2 改动面可算（git base 取不到 / 件两侧读不到 ⇒ rc=2）\n" +
		"  R3 对账：改动面(modified∪removed) - 自述声明 非空 ⇒ 判红；claims_no_existing_modification 直接证伪\n" +
		"  R4 空改动 / 无函数 ⇒ 只报告\n" +
		"  退码：0 过 · 1 判红（自述不实）· 2 不给结论\n" +
		"  限度：Go 顶格 func + 花括号配平 / Py 缩进 / 其余行区间近似（只报不确定不硬判）\n"
	if out != want {
		t.Fatalf("list-rules 正文失配：\n got=%q\nwant=%q", out, want)
	}
}

// TestCdsrVersion —— `--version` 逐字。
func TestCdsrVersion(t *testing.T) {
	rc, out, _ := cdsrRunArgs("--version")
	if rc != 0 || out != "check-delegation-selfreport.py v1.0.0\n" {
		t.Fatalf("version 失配 rc=%d out=%q", rc, out)
	}
}

// TestCdsrArgparseFaces —— 帮助 / 未知旗标 / 缺值 / 歧义前缀 / 版本优先（逐字节）。
func TestCdsrArgparseFaces(t *testing.T) {
	rc, out, _ := cdsrRunArgs("-h")
	if rc != 0 || out != cdsrHelp {
		t.Fatalf("-h 失配 rc=%d out=%q", rc, out)
	}
	rc, _, errOut := cdsrRunArgs("--foo")
	if rc != 2 || !strings.HasSuffix(errOut, cdsrProg+": error: unrecognized arguments: --foo\n") ||
		!strings.HasPrefix(errOut, cdsrUsage) {
		t.Fatalf("未知旗标失配 rc=%d err=%q", rc, errOut)
	}
	rc, _, errOut = cdsrRunArgs("--report")
	if rc != 2 || !strings.HasSuffix(errOut, "error: argument --report: expected one argument\n") {
		t.Fatalf("缺值失配 rc=%d err=%q", rc, errOut)
	}
	rc, _, errOut = cdsrRunArgs("--r", "x")
	if rc != 2 || !strings.HasSuffix(errOut, "error: ambiguous option: --r could match --report, --root\n") {
		t.Fatalf("歧义前缀失配 rc=%d err=%q", rc, errOut)
	}
	rc, out, _ = cdsrRunArgs("--foo", "--version")
	if rc != 0 || out != "check-delegation-selfreport.py v1.0.0\n" {
		t.Fatalf("版本优先失配 rc=%d out=%q", rc, out)
	}
}

// TestCdsrMissingReportR1 —— 无 `--report` ⇒ R1 rc=2 逐字。
func TestCdsrMissingReportR1(t *testing.T) {
	rc, out, _ := cdsrRunArgs()
	if rc != 2 || out != "⛔ 不给结论（rc=2）：缺 `--report`（没有自述面无从对账）\n" {
		t.Fatalf("R1 缺报告失配 rc=%d out=%q", rc, out)
	}
}

// TestCdsrEntryGitlessRootR4 —— 入口级：非 git 根 ⇒ 件全跳过 ⇒ R4 报告 rc=0（不碰真仓）。
func TestCdsrEntryGitlessRootR4(t *testing.T) {
	dir := t.TempDir()
	rep := filepath.Join(dir, "rep.json")
	if err := os.WriteFile(rep, []byte(`{"schema":"zerg/v1","files":["a.go"],"declared":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	rc, out, _ := cdsrRunArgs("--report", rep, "--root", dir)
	if rc != 0 {
		t.Fatalf("rc=%d want 0 out=%q", rc, out)
	}
	if !strings.Contains(out, "无既有函数被触及（改动面只在新增）") {
		t.Fatalf("R4 行缺失：%q", out)
	}
}

// TestCdsrAnalyzeGoAndPy —— analyze 三态（改/删/增）。
func TestCdsrAnalyzeGoAndPy(t *testing.T) {
	baseGo := "package zerg\n\nfunc gapFoundDays(s string) int {\n\tn := 0\n\treturn n\n}\n\nfunc gapNextID() string { return \"GAP\" }\n"
	newGo := "package zerg\n\nfunc gapFoundDays(s string) int {\n\treturn 0\n}\n\nfunc gapQidOf() string { return \"Q\" }\n"
	m, r, ad := cdsrAnalyze(baseGo, newGo, "x.go")
	if got := strings.Join(cdsrSortedKeys(m), ","); got != "gapFoundDays" {
		t.Fatalf("modified=%q want gapFoundDays", got)
	}
	if got := strings.Join(cdsrSortedKeys(r), ","); got != "gapNextID" {
		t.Fatalf("removed=%q want gapNextID", got)
	}
	if got := strings.Join(cdsrSortedKeys(ad), ","); got != "gapQidOf" {
		t.Fatalf("added=%q want gapQidOf", got)
	}
	basePy := "def a():\n    return 1\n\ndef b():\n    return 2\n"
	newPy := "def a():\n    return 9\n\ndef b():\n    return 2\n"
	m2, r2, a2 := cdsrAnalyze(basePy, newPy, "x.py")
	if strings.Join(cdsrSortedKeys(m2), ",") != "a" || len(r2) != 0 || len(a2) != 0 {
		t.Fatalf("py analyze 失配 m=%v r=%v a=%v", cdsrSortedKeys(m2), cdsrSortedKeys(r2), cdsrSortedKeys(a2))
	}
	// 尾随空白不算改动
	m3, _, _ := cdsrAnalyze("func f() {\n\treturn 1\n}\n", "func f() {   \n\treturn 1\n}\n", "x.go")
	if len(m3) != 0 {
		t.Fatalf("尾随空白应不算改动：%v", cdsrSortedKeys(m3))
	}
}

// TestCdsrReconcileR3 —— 对账判红（未声明被点 · claims 直接证伪 · 声明后不红）。
func TestCdsrReconcileR3(t *testing.T) {
	mod := map[string]bool{"gapFoundDays": true}
	rem := map[string]bool{"oldFn": true}
	reds := cdsrReconcile(mod, rem, map[string]bool{}, map[string]bool{}, false)
	if len(reds) != 2 {
		t.Fatalf("未声明应 2 红，got %d：%v", len(reds), reds)
	}
	if reds[0].What != "既有函数 gapFoundDays 函数体被改 · 自述面未声明" {
		t.Fatalf("mod 红文案失配：%q", reds[0].What)
	}
	if reds[1].What != "既有函数 oldFn 被删 · 自述面未声明" {
		t.Fatalf("rem 红文案失配：%q", reds[1].What)
	}
	reds2 := cdsrReconcile(map[string]bool{"gapFoundDays": true}, map[string]bool{},
		map[string]bool{"gapFoundDays": true}, map[string]bool{}, true)
	if len(reds2) != 1 || !strings.Contains(reds2[0].What, "自述不实") {
		t.Fatalf("claims 证伪失配：%v", reds2)
	}
	reds3 := cdsrReconcile(mod, rem, map[string]bool{"gapFoundDays": true}, map[string]bool{"oldFn": true}, false)
	if len(reds3) != 0 {
		t.Fatalf("声明后应不红：%v", reds3)
	}
}

// TestCdsrJSONEmptySlicesNotNull —— 空切片必须 `[]`（✗ 不得 `null`）。
func TestCdsrJSONEmptySlicesNotNull(t *testing.T) {
	js := cdsrMainJSON("/r", "HEAD", "/p.json", false, []string{}, []string{}, []string{}, nil, []cdsrRed{}, nil, 0)
	for _, want := range []string{`"modified": []`, `"removed": []`, `"added": []`, `"per_file": []`, `"reds": []`, `"reports": []`} {
		if !strings.Contains(js, want) {
			t.Fatalf("空切片失配（应为 %s）：%s", want, js)
		}
	}
	if strings.Contains(js, ": null") {
		t.Fatalf("不应出现 null：%s", js)
	}
}

// TestCdsrJSONDecodeErrorText —— py `str(JSONDecodeError)` 逐字（非 Go 编码器文本）。
func TestCdsrJSONDecodeErrorText(t *testing.T) {
	cases := map[string]string{
		"not json at all\n": "Expecting value: line 1 column 1 (char 0)",
		"":                  "Expecting value: line 1 column 1 (char 0)",
		"{ \"a\":\n}":       "Expecting value: line 2 column 1 (char 7)",
		"{} {}":             "Extra data: line 1 column 4 (char 3)",
		"[1 2]":             "Expecting ',' delimiter: line 1 column 4 (char 3)",
		"{ not json":        "Expecting property name enclosed in double quotes: line 1 column 3 (char 2)",
		"01":                "Extra data: line 1 column 2 (char 1)",
		"{\"a\":1,}":        "Expecting property name enclosed in double quotes: line 1 column 8 (char 7)",
		"{\"a\"":            "Expecting ':' delimiter: line 1 column 5 (char 4)",
		"1.5e":              "Extra data: line 1 column 4 (char 3)",
		"{\"a\": }":         "Expecting value: line 1 column 7 (char 6)",
		"[1,2":              "Expecting ',' delimiter: line 1 column 5 (char 4)",
	}
	for in, want := range cases {
		if _, got := cdsrLoadJSON(in); got != want {
			t.Fatalf("json 错误正文失配 in=%q\n got=%q\nwant=%q", in, got, want)
		}
	}
	if v, got := cdsrLoadJSON(`{"a": [1, 2], "b": true}`); got != "" {
		t.Fatalf("合法 JSON 应无错：%q", got)
	} else if m, ok := v.(map[string]any); !ok || len(m) != 2 {
		t.Fatalf("合法 JSON 值面失配：%v", v)
	}
}

// TestCdsrPyPrimitives —— py 词法原语同构（join 空段 / 后段绝对 / normpath 双斜杠）。
func TestCdsrPyPrimitives(t *testing.T) {
	if got := cdsrPyJoin("a", ""); got != "a/" {
		t.Fatalf("join(a,'')=%q want a/", got)
	}
	if got := cdsrPyJoin("a", "", "b"); got != "a/b" {
		t.Fatalf("join(a,'',b)=%q want a/b", got)
	}
	if got := cdsrPyJoin("repo", "/abs"); got != "/abs" {
		t.Fatalf("join(repo,/abs)=%q want /abs", got)
	}
	if got := cdsrPyNormpath("//a/b/../c"); got != "//a/c" {
		t.Fatalf("normpath 双前导斜杠失配：%q", got)
	}
	if got := cdsrPyRStrip("x\u00a0\u001c"); got != "x" {
		t.Fatalf("py rstrip 未剥 \\xa0/\\x1c：%q", got)
	}
}

// TestCdsrNoAbsolutePathLiterals —— 自指面②：帮助/用法正文零**绝对路径**字面量（不落仓内路径）。
func TestCdsrNoAbsolutePathLiterals(t *testing.T) {
	for _, bad := range []string{"/Volumes/", "/Users/", "/private/", "/tmp/", "/home/"} {
		if strings.Contains(cdsrHelp, bad) || strings.Contains(cdsrUsage, bad) {
			t.Fatalf("帮助/用法正文含绝对路径字面量 %q（自指面②风险）", bad)
		}
	}
}
