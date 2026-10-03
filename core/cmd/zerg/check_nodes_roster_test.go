// check_nodes_roster_test.go —— `check_nodes_roster.go` 的定向单测（成对负控 · ✗ 禁恒绿）。
//
// 覆盖面（与件头「已对拍面 / 未覆盖面」对齐）：
//
//	① 自检十档全过（正控 rc=0）· ② 显式三路径表（全对 0 / 漂移 1 / 无声明 1 / 名册不在 2 / 空名册 2 /
//	plist 不在 0）· ③ `--json` 用法错 rc=2（Q-176 本门无机器读面）· ④ `-h` 帮助 rc=0 ·
//	⑤ 未知旗标 rc=2（unrecognized）· ⑥ 缺值 rc=2（expected one argument）· ⑦ `--list-rules` rc=0 ·
//	⑧ A 类对齐：自检后临时目录同前缀计数 delta=0 · ⑨ 漂移**点名到字段**（负控锚点：哑化后必红）。
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// cnrWriteFixture —— 写一枚夹具件，返回其路径。
func cnrWriteFixture(t *testing.T, dir, name, text string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
		t.Fatalf("写夹具失败 %s: %v", p, err)
	}
	return p
}

// cnrRunMain —— 进程内复用入口，取 (rc, stdout, stderr)。
func cnrRunMain(argv ...string) (int, string, string) {
	var so, se strings.Builder
	rc := checkNodesRosterGateMain(&so, &se, argv)
	return rc, so.String(), se.String()
}

// TestCheckNodesRoster_SelfTestAllPass —— ① 自检十档（正控：全过 ⇒ rc=0）。
func TestCheckNodesRoster_SelfTestAllPass(t *testing.T) {
	var so, se strings.Builder
	rc := cnrSelfTest(&so, &se)
	out := so.String()
	if rc != 0 {
		t.Fatalf("自检应 rc=0，实得 %d\n%s", rc, out)
	}
	if !strings.Contains(out, "自检结论：全过（10 条，失败 0）") {
		t.Fatalf("自检结论行不符，实得尾部：\n%s", out)
	}
	// 十档里负控必须真红过（成对：少一台/host 漂/声明缺/ssh 漂/models 未登记/plist 未登记）
	for _, want := range []string{"fleet: host 改一位 ⇒ 1", "updates 段声明删掉 ⇒ 1", "plist 机器名未登记 ⇒ 1"} {
		if !strings.Contains(out, want) {
			t.Fatalf("自检输出缺档 %q", want)
		}
	}
}

// TestCheckNodesRoster_ExplicitTrioTable —— ② 显式三路径逐档退码。
func TestCheckNodesRoster_ExplicitTrioTable(t *testing.T) {
	d := t.TempDir()
	roster := cnrWriteFixture(t, d, "roster.yaml", cnrRosterFix)
	fleetOK := cnrWriteFixture(t, d, "fleet.yaml", cnrFleetFix)
	plist := cnrWriteFixture(t, d, "a.plist", cnrPlistFix)
	fleetDrift := cnrWriteFixture(t, d, "fleet_drift.yaml",
		strings.ReplaceAll(cnrFleetFix, "edge-node: { host: <worker-ip>", "edge-node: { host: <worker-ip>"))
	fleetNoDecl := cnrWriteFixture(t, d, "fleet_nodecl.yaml",
		strings.Replace(cnrFleetFix, "  # 投影源：真源 = ~/.zerg/contexts/default.yaml 的 nodes:\n", "", 2))
	emptyRoster := cnrWriteFixture(t, d, "empty_roster.yaml", "nodes:\n")
	missingRoster := filepath.Join(d, "nope.yaml")
	missingPlist := filepath.Join(d, "nope.plist")

	cases := []struct {
		name                 string
		roster, fleet, plist string
		want                 int
	}{
		{"全对", roster, fleetOK, plist, 0},
		{"host 漂", roster, fleetDrift, plist, 1},
		{"两段无声明", roster, fleetNoDecl, plist, 1},
		{"名册不在", missingRoster, fleetOK, plist, 2},
		{"名册空", emptyRoster, fleetOK, plist, 2},
		// ★ py 现读分歧：docstring 把「fleet.yaml 不在」写在 rc=2 档，但 `check_fleet` 实际把它
		//   放进 `fails` ⇒ **rc=1**（漂移档）。本件逐字沿用**代码**（✗ 不按注释抬成 2）。
		{"fleet 不在（py 代码实为 rc=1）", roster, filepath.Join(d, "nope.yaml"), plist, 1},
		{"plist 不在（跳过该处）", roster, fleetOK, missingPlist, 0},
	}
	for _, c := range cases {
		rc, _, _ := cnrRunMain("--roster", c.roster, "--fleet", c.fleet, "--plist", c.plist)
		if rc != c.want {
			t.Errorf("档 %q：期望 rc=%d 实得 rc=%d", c.name, c.want, rc)
		}
	}
}

// TestCheckNodesRoster_DriftNamesField —— ⑨ 漂移必须**点名到字段与两侧值**（负控锚点）。
// 哑化 `if got != want`（或把 cnrStripQuotes 变成恒等）⇒ 本条**必红**。
func TestCheckNodesRoster_DriftNamesField(t *testing.T) {
	d := t.TempDir()
	roster := cnrWriteFixture(t, d, "roster.yaml", cnrRosterFix)
	plist := cnrWriteFixture(t, d, "a.plist", cnrPlistFix)
	drift := cnrWriteFixture(t, d, "drift.yaml",
		strings.ReplaceAll(cnrFleetFix, "edge-node: { host: <worker-ip>", "edge-node: { host: <worker-ip>"))
	rc, out, _ := cnrRunMain("--roster", roster, "--fleet", drift, "--plist", plist)
	if rc != 1 {
		t.Fatalf("漂移应 rc=1，实得 %d\n%s", rc, out)
	}
	want := "C `edge-node` 的 host 漂了：fleet='<worker-ip>' 名册='<worker-ip>'"
	if !strings.Contains(out, want) {
		t.Fatalf("漂移明细未点名：缺 %q\n实得：\n%s", want, out)
	}
}

// TestCheckNodesRoster_JSONRefused —— ③ Q-176：本门无机器读面 ⇒ `--json` 恒拒 rc=2。
func TestCheckNodesRoster_JSONRefused(t *testing.T) {
	rc, out, errOut := cnrRunMain("--json")
	if rc != 2 {
		t.Fatalf("`--json` 应 rc=2，实得 %d", rc)
	}
	if out != "" {
		t.Fatalf("拒收档 stdout 应为空，实得 %q", out)
	}
	for _, want := range []string{"本门无机器读面", "缺口 Q-176", "字段表：无（本门只出人读面）", "机器面缺 ≠ 零命中"} {
		if !strings.Contains(errOut, want) {
			t.Fatalf("stderr 缺 %q\n实得：\n%s", want, errOut)
		}
	}
	// 等号形也拒
	if rc2, _, _ := cnrRunMain("--json=x"); rc2 != 2 {
		t.Fatalf("`--json=x` 应 rc=2，实得 %d", rc2)
	}
}

// TestCheckNodesRoster_Help —— ④ `-h/--help` rc=0 且首行逐字。
func TestCheckNodesRoster_Help(t *testing.T) {
	for _, a := range []string{"-h", "--help"} {
		rc, out, _ := cnrRunMain(a)
		if rc != 0 {
			t.Fatalf("%s 应 rc=0，实得 %d", a, rc)
		}
		if !strings.HasPrefix(out, "usage: check-nodes-roster.py [-h] [--roster ROSTER] [--fleet FLEET]\n") {
			t.Fatalf("%s 帮助首行不符：\n%s", a, out)
		}
		if !strings.Contains(out, "optional arguments:") {
			t.Fatalf("%s 帮助缺 `optional arguments:` 段", a)
		}
	}
}

// TestCheckNodesRoster_ArgErrors —— ⑤⑥ argparse 同形错误（rc=2）。
func TestCheckNodesRoster_ArgErrors(t *testing.T) {
	rc, _, e := cnrRunMain("--bogus")
	if rc != 2 || !strings.Contains(e, "check-nodes-roster.py: error: unrecognized arguments: --bogus") {
		t.Fatalf("未知旗标档不符 rc=%d\n%s", rc, e)
	}
	rc, _, e = cnrRunMain("--roster")
	if rc != 2 || !strings.Contains(e, "check-nodes-roster.py: error: argument --roster: expected one argument") {
		t.Fatalf("缺值档不符 rc=%d\n%s", rc, e)
	}
	if !strings.HasPrefix(e, "usage: check-nodes-roster.py ") {
		t.Fatalf("错误档缺 usage 首行：\n%s", e)
	}
	if _, _, e = cnrRunMain("--", "--bogus"); !strings.Contains(e, "unrecognized arguments: -- --bogus") {
		t.Fatalf("`--` 终止符档不符：\n%s", e)
	}
}

// TestCheckNodesRoster_ListRules —— ⑦ `--list-rules` rc=0 且带路径来源行。
func TestCheckNodesRoster_ListRules(t *testing.T) {
	rc, out, _ := cnrRunMain("--list-rules", "--roster", "/tmp/rr", "--fleet", "/tmp/ff", "--plist", "/tmp/pp")
	if rc != 0 {
		t.Fatalf("`--list-rules` 应 rc=0，实得 %d", rc)
	}
	if !strings.HasPrefix(out, "check-nodes-roster.py — 机器名册的**唯一真源核对**") {
		t.Fatalf("doc 首行不符：\n%.120s", out)
	}
	if !strings.Contains(out, "路径来源：名册 /tmp/rr · fleet /tmp/ff · plist /tmp/pp（都可覆盖；自检用夹具）") {
		t.Fatalf("路径来源行不符（覆盖未生效）：\n%s", out)
	}
}

// TestCheckNodesRoster_TmpDirDeltaZero —— ⑧ A 类对齐：自检前后**本用例私有 TMPDIR** 内同前缀临时目录计数 delta=0。
//
// ★ GAP-20261003-16：旧写法扫的是**共享** `os.TempDir()`（TMPDIR=/var/tmp）⇒ 同包兄弟用例 / 同机并发门跑
// 在 before/after 之间并发建删同类 `tmp*` 目录时，同一份代码会被踩成 delta=±1（先红后绿）。
// 修法：把 TMPDIR 指到本用例自建的空目录（私有面）⇒ 只数本用例自建目录，不再与任何兄弟共享计数面。
// 断言一字未放宽：仍是 `before != after` ⇒ 报「delta=%d」。负控：删自检档 `defer os.RemoveAll(d)`
// 时，私有面内必留 `tmp*` ⇒ after=before+1 ⇒ 本用例必红（真泄漏仍抓得住）。
// （本包零 `t.Parallel()` ⇒ `t.Setenv` 不会 panic。）
func TestCheckNodesRoster_TmpDirDeltaZero(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir()) // 私有计数面：本次用例独占
	tmp := os.TempDir()
	count := func() int {
		ents, err := os.ReadDir(tmp)
		if err != nil {
			return -1
		}
		n := 0
		for _, e := range ents {
			if e.IsDir() && strings.HasPrefix(e.Name(), "tmp") {
				n++
			}
		}
		return n
	}
	before := count()
	var so, se strings.Builder
	_ = cnrSelfTest(&so, &se)
	after := count()
	if before != after {
		t.Fatalf("自检泄漏临时目录：前 %d 后 %d（delta=%d）", before, after, after-before)
	}
}

// TestCheckNodesRoster_PyParsePrimitives —— 纯解析原语定点（翻译语义差守门）。
func TestCheckNodesRoster_PyParsePrimitives(t *testing.T) {
	// unquote / strip 的 py 语义
	if got := cnrUnquote(`"http://127.0.0.1:8580"`); got != "http://127.0.0.1:8580" {
		t.Fatalf("unquote 失败：%q", got)
	}
	// flow 映射：值**不**按 `:` 二次切（partition 只切首个冒号）
	m := cnrParseFlow(`{ host: <worker-ip>, api: "http://x:8580", components: "core,agentd" }`)
	if m["host"] != "<worker-ip>" || m["api"] != "http://x:8580" || m["components"] != "core,agentd" {
		t.Fatalf("parse_flow 结果不符：%#v", m)
	}
	// section_range 只认列首键
	lines := []string{"  nodes:", "nodes:", "  edge-node:", "    host: 1.2.3.4"}
	s, e, ok := cnrSectionRange(lines, "nodes")
	if !ok || s != 2 || e != 4 {
		t.Fatalf("section_range 不符：%d,%d,%v", s, e, ok)
	}
	// E：`\\bhost\\s*:\\s*<值>` 的手写复刻
	if h, ok := cnrModelsHost("    - { host: mini-node-1, backend: llama-server }"); !ok || h != "mini-node-1" {
		t.Fatalf("models host 抽取不符：%q %v", h, ok)
	}
	if _, ok := cnrModelsHost("  hostname: edge-node"); ok {
		t.Fatalf("`hostname:` 不应命中")
	}
}
