// flag_given_test.go —— 值旗标「给没给」唯一判据（`inv.flagGiven`）的判据面 + 收敛自证。
//
// 缺口 `GAP-20260926-58`：三套口径（`hasFlag` / 直读 `kv` / 重扫 `orig`）收成一套。
// 本件钉三样：
//
//	① **行为**：`flagGiven` 对四类旗标（kv 取值旗标 / 类型化字段 / 专用 `*Given` 字段 / 布尔）
//	   的「给」与「没给」两态逐格对；
//	② **形态**：裸形（给了旗标没给值）⇒ 判据真、值空 —— 「给过」与「给了值」是两件事；
//	③ **收敛**：三处旧口径（直读 `kv` / 重扫 `orig`）改调它之后，输出与旧口径逐格相同；
//	   且仓里**只有一份** given 判据、一份 argv 扫描实现（结构守卫，防第四套口径回潮）。
package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// TestFlagGivenTwoStates —— 四类旗标 × 「给/没给」两态（argv 形态 × {裸形, `--k=v`, `--k v`}）。
func TestFlagGivenTwoStates(t *testing.T) {
	type row struct {
		why  string
		argv []string
		flag string
		want bool
	}
	rows := []row{
		// ── ① kv 取值旗标（进 `inv.kv`）────────────────────────────────────────────
		{"kv 取值旗标 · 空格形", []string{"script", "inventory", "sync", "--out", "/tmp/x"}, "--out", true},
		{"kv 取值旗标 · 等号形", []string{"script", "inventory", "sync", "--out=/tmp/x"}, "--out", true},
		{"kv 取值旗标 · 裸形（尾）", []string{"script", "inventory", "sync", "--out"}, "--out", true},
		{"kv 取值旗标 · 裸形（后跟旗标）", []string{"x", "--out", "--yes"}, "--out", true},
		{"kv 取值旗标 · 没给", []string{"script", "inventory", "sync"}, "--out", false},
		{"kv 取值旗标 · 前缀不算（--output ≠ --out）", []string{"x", "--output=/tmp"}, "--out", false},
		{"kv 取值旗标 · 值里长得像旗标也不算别的旗标", []string{"x", "--desc", "--out"}, "--file", false},
		// ── ② 类型化字段（`setValueFlag` ⇒ 专用字段，**不进 kv**）──────────────────
		{"类型化字段 --model · 空格形", []string{"agent", "run", "--model", "m1"}, "--model", true},
		{"类型化字段 --model · 等号形", []string{"agent", "run", "--model=m1"}, "--model", true},
		{"类型化字段 --model · 裸形", []string{"agent", "run", "--model"}, "--model", true},
		{"类型化字段 --model · 没给", []string{"agent", "run", "--task", "t"}, "--model", false},
		{"类型化字段 --set · 可重复", []string{"x", "--set", "a=1"}, "--set", true},
		{"类型化字段 --acceptance · 裸形", []string{"task", "submit", "--acceptance"}, "--acceptance", true},
		// ── ③ 专用 `*Given` 字段（既不进 kv、也不是 valueFlagName）─────────────────
		{"--json 裸形（jsonGiven）", []string{"code", "find", "x", "--json"}, "--json", true},
		{"--json 给字段", []string{"code", "find", "x", "--json", "count"}, "--json", true},
		{"--json 没给", []string{"code", "find", "x"}, "--json", false},
		{"--confirm 裸形（confirmGiven）", []string{"repo", "commit", "--confirm"}, "--confirm", true},
		{"--confirm 没给", []string{"repo", "commit"}, "--confirm", false},
		{"--ttl 裸形（ttlGiven）", []string{"approve", "new", "--ttl"}, "--ttl", true},
		{"--schema 裸形（schemaGiven）", []string{"x", "--schema"}, "--schema", true},
		// ── ④ 布尔档（`kvSet(name,"true")`）────────────────────────────────────────
		{"布尔 --count", []string{"code", "find", "x", "--count"}, "--count", true},
		{"布尔 --count 没给", []string{"code", "find", "x"}, "--count", false},
		{"布尔 --stale（gate reap）", []string{"gate", "reap", "--stale"}, "--stale", true},
		{"布尔 --stale 没给", []string{"gate", "reap"}, "--stale", false},
		// ── ⑤ 解析面不认识的旗标：argv 面照样判得出来（既有三套口径同此）────────────
		{"未登记旗标 · 判据看 argv 不看名字表", []string{"x", "--nope", "v"}, "--nope", true},
	}
	for _, r := range rows {
		inv, err := parseInvocation(r.argv)
		if err != nil {
			t.Fatalf("%s：parseInvocation(%v) 失败：%v", r.why, r.argv, err)
		}
		if got := inv.flagGiven(r.flag); got != r.want {
			t.Errorf("%s：flagGiven(%q) 在 %v 上 = %v，want %v", r.why, r.flag, r.argv, got, r.want)
		}
	}
}

// TestFlagGivenBareFormKeepsEmptyValue —— 裸形 = 「给过、值空」：判据真而值读起来是空串
// （两态不合并 —— 各命令靠这一格报用法错 2，不许静默当「没给」）。
func TestFlagGivenBareFormKeepsEmptyValue(t *testing.T) {
	cases := []struct {
		why    string
		argv   []string
		flag   string
		readFn func(*invocation) string
	}{
		{"kv 取值旗标 --out", []string{"x", "--out"}, "--out", func(i *invocation) string { return i.flagVal("--out") }},
		{"kv 取值旗标 --gate-results", []string{"x", "--gate-results"}, "--gate-results", func(i *invocation) string { return i.flagVal("--gate-results") }},
		{"类型化字段 --model", []string{"x", "--model"}, "--model", func(i *invocation) string { return i.modelWant }},
		{"类型化字段 --desc", []string{"x", "--desc"}, "--desc", func(i *invocation) string { return i.desc }},
	}
	for _, c := range cases {
		inv, err := parseInvocation(c.argv)
		if err != nil {
			t.Fatalf("%s：parseInvocation 失败：%v", c.why, err)
		}
		if !inv.flagGiven(c.flag) {
			t.Errorf("%s：裸形必须判「给过」（flagGiven(%q)=false）", c.why, c.flag)
		}
		if v := c.readFn(inv); strings.TrimSpace(v) != "" {
			t.Errorf("%s：裸形的值必须是空串（实得 %q）—— 「给过」与「给了值」是两件事", c.why, v)
		}
	}
}

// TestFlagGivenKvWithoutOrig —— 判据的 `kv` 那一路**自带牙**：没有 `orig` 的 invocation
// （合成件 / 直接构造的 inv）只靠 `kv` 归一结果判「给没给」—— 丢了这一路，本格必红。
// 这也是 P3 变异探针的靶格（把 `flagGiven` 退化成「只看 argv」⇒ 本格红）。
func TestFlagGivenKvWithoutOrig(t *testing.T) {
	inv := &invocation{kv: map[string][]string{"--out": {""}, "--prefix": {"src/"}}}
	if !inv.flagGiven("--out") {
		t.Errorf("kv 有键（裸形归一：键在、值空）⇒ 必须判「给过」（无 orig 时判据只剩这一路）")
	}
	if !inv.flagGiven("--prefix") {
		t.Errorf("kv 有键（`--prefix src/`）⇒ 必须判「给过」")
	}
	if inv.flagGiven("--file") {
		t.Errorf("kv 没这个键 ⇒ 必须判「没给」")
	}
	if v := inv.flagVal("--out"); v != "" {
		t.Errorf("裸形的值必须是空串（实得 %q）", v)
	}
}

// TestFlagGivenMatchesLegacyIdioms —— 收敛自证 ①：三处旧口径（重扫 orig 的两枚 + export 的值读器）
// 与唯一判据在同一批输入上**逐格同答**（旧口径 = `flagArgGiven`/`flagArgScan`，判据 = kv ‖ 它）。
func TestFlagGivenMatchesLegacyIdioms(t *testing.T) {
	argvs := [][]string{
		{"impact", "x"},
		{"impact", "x", "--gate-results"},
		{"impact", "x", "--gate-results=r.tsv"},
		{"impact", "x", "--gate-results", "r.tsv"},
		{"impact", "x", "--gate-results", "--yes"}, // 裸形：后跟旗标 ⇒ 值空
		{"impact", "x", "--json", "what"},
		{"impact", "--", "x", "--gate-results"}, // `--` 之后：argv 面看得见、kv 面看不见
		{"agent", "run", "--task", "t", "--model", "m"},
	}
	for _, argv := range argvs {
		inv, err := parseInvocation(argv)
		if err != nil {
			t.Fatalf("parseInvocation(%v) 失败：%v", argv, err)
		}
		// 旧口径 A：`impactBackfillFlagGiven`（重扫 orig）—— 生产路径已改调 flagGiven，本壳只喂黑盒用例。
		if got, want := inv.flagGiven("--gate-results"), impactBackfillFlagGiven(argv); got != want {
			t.Errorf("%v：flagGiven(--gate-results)=%v ≠ 旧口径 impactBackfillFlagGiven=%v", argv, got, want)
		}
		// 旧口径 B：`agentRunFlagGiven`（重扫 orig）—— 已改调 flagGiven，两者必须同答。
		if got, want := agentRunFlagGiven(inv, "--model"), flagArgGiven(argv, "--model"); got != want {
			t.Errorf("%v：agentRunFlagGiven(--model)=%v ≠ 旧口径 flagArgGiven=%v", argv, got, want)
		}
		// 旧口径 C：`gateReapHasFlag`（扫尾段）—— 已改调 flagGiven。
		if got, want := gateReapHasFlag(inv, "--stale"), flagArgGiven(argv, "--stale"); got != want {
			t.Errorf("%v：gateReapHasFlag(--stale)=%v ≠ 旧口径 flagArgGiven=%v", argv, got, want)
		}
	}
}

// TestFlagValueKeepsLegacySemantics —— export 的 `flagValue` 只剩「值」这一问：出现过**且**有值槽
// 才算取到；`--out --yes` 取到下一格（旧行为逐字，收敛不改它）。
func TestFlagValueKeepsLegacySemantics(t *testing.T) {
	cases := []struct {
		argv   []string
		wantV  string
		wantOK bool
		why    string
	}{
		{[]string{"help", "export", "--out", "/tmp/x"}, "/tmp/x", true, "空格形取到值"},
		{[]string{"help", "export", "--out=/tmp/x"}, "/tmp/x", true, "等号形取到值"},
		{[]string{"help", "export", "--out"}, "", false, "裸形在尾 = 没取到（旧行为）"},
		{[]string{"help", "export", "--out", "--yes"}, "--yes", true, "后跟旗标仍收下当值（旧行为逐字）"},
		{[]string{"help", "export"}, "", false, "没给"},
	}
	for _, c := range cases {
		v, ok := flagValue(c.argv, "--out")
		if v != c.wantV || ok != c.wantOK {
			t.Errorf("%s：flagValue(%v) = (%q,%v)，want (%q,%v)", c.why, c.argv, v, ok, c.wantV, c.wantOK)
		}
	}
}

// TestScriptInvDenyWriteFlagsUsesGivenBit —— 收敛自证 ②：写旗标拒执守卫（旧「直读 kv」）语义不变：
// `--out`/`--file` 的三种给法都拒，`--set` 仍走 `setPairs`，没给就放行。
func TestScriptInvDenyWriteFlagsUsesGivenBit(t *testing.T) {
	cases := []struct {
		argv []string
		want []string
	}{
		{[]string{"script", "inventory", "sync"}, nil},
		{[]string{"script", "inventory", "sync", "--out", "/tmp/x"}, []string{"--out"}},
		{[]string{"script", "inventory", "sync", "--out=/tmp/x"}, []string{"--out"}},
		{[]string{"script", "inventory", "sync", "--out"}, []string{"--out"}},
		{[]string{"script", "inventory", "sync", "--file", "f"}, []string{"--file"}},
		{[]string{"script", "inventory", "sync", "--set", "a=1"}, []string{"--set"}},
	}
	for _, c := range cases {
		inv, err := parseInvocation(c.argv)
		if err != nil {
			t.Fatalf("parseInvocation(%v) 失败：%v", c.argv, err)
		}
		got := scriptInvDenyWriteFlags(inv)
		if strings.Join(got, ",") != strings.Join(c.want, ",") {
			t.Errorf("%v：拒执面 = %v，want %v", c.argv, got, c.want)
		}
	}
}

// TestFlagGivenSingleImplementation —— 收敛自证 ③（结构守卫，防第四套口径回潮）：
//
//	① `flagGiven` 的**唯一**定义在本包（非测试件里恰好一处）；
//	② argv 扫描的**唯一**实现在 `flag_given.go`（`flagArgScan` 恰好一处定义）；
//	③ 除 `main.go`（本单禁区）与 `flag_given.go` 外，本包非测试件**零**处直读 `inv.kv[`；
//	④ 四枚旧壳（`agentRunFlagGiven` / `impactBackfillFlagGiven` / `gateReapHasFlag` / `flagValue`）
//	   的体里必须出现唯一实现的名字，且**不许**再自带 `range` 循环。
func TestFlagGivenSingleImplementation(t *testing.T) {
	files := flagGivenPkgFiles(t)

	defineGiven, defineScan := 0, 0
	for name, src := range files {
		if name == "flag_given.go" {
			defineScan += strings.Count(src, "func flagArgScan(")
			continue // 本件自己就是唯一实现，下面「零直读 kv」对它不适用
		}
		if name == "main.go" {
			continue // 禁区：`hasFlag` 的定义在另一路手上（待改行已单独报）
		}
		defineGiven += strings.Count(src, "func (inv *invocation) flagGiven(")
		if strings.Contains(flagGivenStripLineComments(src), "inv.kv[") {
			t.Errorf("%s 里出现「直读 inv.kv」—— 这是被收敛掉的口径 ②，请改调 inv.flagGiven", name)
		}
	}
	if defineGiven != 0 {
		t.Errorf("`inv.flagGiven` 只许在 flag_given.go 定义一次，现读另有 %d 处定义", defineGiven)
	}
	if defineScan != 1 {
		t.Errorf("`flagArgScan` 只许有一份实现（flag_given.go），现读 %d 处", defineScan)
	}

	// ④ 四枚旧壳：只许转调，不许自带扫描循环。
	shells := []struct{ fn, file string }{
		{"agentRunFlagGiven", "family_agent_run.go"},
		{"impactBackfillFlagGiven", "family_impact_backfill.go"},
		{"gateReapHasFlag", "family_gate_reap.go"},
		{"flagValue", "export.go"},
	}
	for _, s := range shells {
		src, ok := files[s.file]
		if !ok {
			t.Fatalf("读不到 %s（结构守卫的分母缺件 ⇒ 判不了，不许当绿）", s.file)
		}
		body, ok := flagGivenFuncBody(src, s.fn)
		if !ok {
			t.Errorf("%s 里找不到 %s 的函数体（判不了 ⇒ 不许当绿）", s.file, s.fn)
			continue
		}
		if !strings.Contains(body, "flagGiven(") && !strings.Contains(body, "flagArgGiven(") && !strings.Contains(body, "flagArgScan(") {
			t.Errorf("%s.%s 的体里没有唯一实现（flagGiven/flagArgGiven/flagArgScan）—— 又一套口径？\n%s", s.file, s.fn, body)
		}
		if strings.Contains(body, "range ") {
			t.Errorf("%s.%s 的体里仍有自带的 argv 扫描循环 —— 应转调唯一实现：\n%s", s.file, s.fn, body)
		}
	}
}

// flagGivenPkgFiles —— 本包非测试 `.go` 件（件名 ⇒ 正文）；读不到件 ⇒ 直接红（分母缺件不许当绿）。
func flagGivenPkgFiles(t *testing.T) map[string]string {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("读包目录失败：%v", err)
	}
	out := map[string]string{}
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() || !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") {
			continue
		}
		b, err := os.ReadFile(n)
		if err != nil {
			t.Fatalf("读 %s 失败：%v", n, err)
		}
		out[n] = string(b)
	}
	if len(out) < 100 {
		t.Fatalf("本包非测试件只读到 %d 枚（远小于预期 ⇒ 分母缺件，判不了）", len(out))
	}
	return out
}

// flagGivenStripLineComments —— 去掉 `//` 行注释后的正文（结构守卫要看**代码**，不看注释里提到的旧口径）。
func flagGivenStripLineComments(src string) string {
	var b strings.Builder
	for _, ln := range strings.Split(src, "\n") {
		if i := strings.Index(ln, "//"); i >= 0 {
			ln = ln[:i]
		}
		b.WriteString(ln)
		b.WriteString("\n")
	}
	return b.String()
}

// flagGivenFuncBody —— 取 `func <name>(…) … { … }` 的体（首个列 0 的 `}` 收尾；本包这几个壳都是平体）。
func flagGivenFuncBody(src, name string) (string, bool) {
	re := regexp.MustCompile(`(?s)func ` + regexp.QuoteMeta(name) + `\([^\n]*\{(.*?)\n\}`)
	m := re.FindStringSubmatch(src)
	if m == nil {
		return "", false
	}
	return m[1], true
}
