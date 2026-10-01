// check_public_tree_selfcheck_test.go —— 上件（`check_public_tree_selfcheck.go`）的**成对负控**（✗ 禁恒绿）。
//
// 覆盖面（每条都带「哑化即红」的牙）：
//
//	① 判决面 7 格（含 must-fail 两类 / 正控 / 空转）+ 区分度 —— 判据出入口 `cptsJudge`。
//	② 双方 HEAD sha 三档（对应 / 不对应 / 未证）—— 判据出入口 `cptsJudgeHeads`。
//	③ ★ **真执行面** `cptsRunProbe` 真起子进程、真回传文本（哑化 ⇒ 本格红）。
//	④ ★ **取数面** `cptsReadHead` 真读 40 位 HEAD（哑化 ⇒ 本格红）。
//	⑤ 自检档整体 rc（含三方 sha 判据与上面两面）。
//	⑥ `--list-rules` / 未知旗标 / `--json` 字段名打错 三档的 rc 与报话。
//	⑦ ★ **A 类回归牙**：clone 失败档不许残留 `zerg-selfcheck-*` 临时树（把主件的 `defer os.RemoveAll`
//	   删掉 ⇒ 本格必 FAIL 带读数）。
//	⑧ dedupe 的**首次出现序**（输出行序守恒）。
//
// ★ GAP-20261001-61：测试件里跑到 python3 的地方**一律钉 `/usr/bin/python3`**（✗ 不裸写 `python3`）。
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const cptsTPy = "/usr/bin/python3"

func cptsTRun(argv []string, env map[string]string) (int, string, string) {
	var ob, eb bytes.Buffer
	for k, v := range env {
		old, had := os.LookupEnv(k)
		_ = os.Setenv(k, v)
		defer func(k, old string, had bool) {
			if had {
				_ = os.Setenv(k, old)
			} else {
				_ = os.Unsetenv(k)
			}
		}(k, old, had)
	}
	rc := checkPublicTreeSelfcheckGateMain(&ob, &eb, argv)
	return rc, ob.String(), eb.String()
}

func cptsTRunClean(argv []string) (int, string, string) {
	var ob, eb bytes.Buffer
	rc := checkPublicTreeSelfcheckGateMain(&ob, &eb, argv)
	return rc, ob.String(), eb.String()
}

func TestCptsJudgeGridAndDiscrimination(t *testing.T) {
	for _, c := range cptsSelfTestCases() {
		got, _, _ := cptsJudge(c.text, c.rc, cptsProbeByID(c.pid))
		if got != c.want {
			t.Errorf("用例 %q：期望 %s，实得 %s", c.name, c.want, got)
		}
	}
	mf1, _, _ := cptsJudge(cptsMF1Text, 1, cptsProbeByID("P1"))
	pc1, _, _ := cptsJudge(cptsPC1Text, 0, cptsProbeByID("P1"))
	if mf1 != "hit" || pc1 != "green" {
		t.Fatalf("区分度失效：MF1 ⇒ %s（应 hit）、PC1 ⇒ %s（应 green）", mf1, pc1)
	}
	if v, _, _ := cptsJudge("", 0, cptsProbeByID("P1")); v != "vacuous" {
		t.Fatalf("空转档：零输出应 vacuous，实得 %s", v)
	}
}

func TestCptsJudgeHeadsThreeStates(t *testing.T) {
	a40 := strings.Repeat("a", 40)
	b40 := strings.Repeat("b", 40)
	if v, _ := cptsJudgeHeads(a40, a40); v != cptsCorrespond {
		t.Errorf("正控：同 sha 应「对应」，实得 %s", v)
	}
	if v, _ := cptsJudgeHeads(a40, b40); v != cptsUnrespond {
		t.Errorf("负控：假 HEAD 应「不对应」，实得 %s", v)
	}
	if v, _ := cptsJudgeHeads("", b40); v != cptsUnproven {
		t.Errorf("缺一侧应「未证」，实得 %s", v)
	}
}

// ③ 真执行面：哑化 `cptsRunProbe`（不真起子进程）⇒ 本格必红。
func TestCptsRunProbeTrulyLive(t *testing.T) {
	dir := t.TempDir()
	p := cptsProbe{
		id: "T", name: "合成", cwd: ".",
		argv:      []string{cptsTPy, "-c", "print('LEAK: test-truly-live')"},
		needsArgv: []string{}, needsTree: []string{},
		marks: []string{"LEAK:"}, failLines: []string{"--- FAIL"},
		why: "测试受控探针",
	}
	rc, text, _, note := cptsRunProbe(dir, p, 60)
	if rc == nil {
		t.Fatalf("真执行面哑化/起不来：rc=nil note=%q", note)
	}
	if *rc != 0 {
		t.Fatalf("rc 期望 0，实得 %d（note=%q）", *rc, note)
	}
	if !strings.Contains(text, "LEAK: test-truly-live") {
		t.Fatalf("未回传子进程输出，实得 %q", text)
	}
	if v, named, _ := cptsJudge(text, *rc, p); v != "hit" || len(named) == 0 {
		t.Fatalf("判决面对真输出应 hit，实得 %s（具名 %d）", v, len(named))
	}
	// 缺件档：needsTree 里点一个不存在的件 ⇒ rc=nil + 具名 note（**空转/缺件不许当绿**）
	miss := p
	miss.needsTree = []string{"no-such-dir-xyz"}
	rc2, _, _, note2 := cptsRunProbe(dir, miss, 60)
	if rc2 != nil || !strings.Contains(note2, "树里缺件：no-such-dir-xyz") {
		t.Fatalf("缺件档：期望 rc=nil + 「树里缺件：…」，实得 rc=%v note=%q", rc2, note2)
	}
	// 工具不在 PATH 档
	noTool := p
	noTool.needsArgv = []string{"cpts-no-such-tool-xyz"}
	rc3, _, _, note3 := cptsRunProbe(dir, noTool, 60)
	if rc3 != nil || !strings.Contains(note3, "工具不在 PATH：cpts-no-such-tool-xyz") {
		t.Fatalf("工具缺失档：期望 rc=nil + 「工具不在 PATH：…」，实得 rc=%v note=%q", rc3, note3)
	}
}

// ④ 取数面：哑化 `cptsReadHead`（回空串）⇒ 本格必红。
func TestCptsReadHeadTrulyLive(t *testing.T) {
	dir := t.TempDir()
	env := append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@e",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@e",
		"GIT_AUTHOR_DATE=2005-04-07T22:13:13+08:00", "GIT_COMMITTER_DATE=2005-04-07T22:13:13+08:00")
	run := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Env = env
		var ob bytes.Buffer
		cmd.Stdout = &ob
		cmd.Stderr = &ob
		if err := cmd.Run(); err != nil {
			t.Fatalf("git %v: %v（%s）", args, err, ob.String())
		}
		return strings.TrimSpace(ob.String())
	}
	run("init", "-q", "-b", "main", dir)
	run("-C", dir, "commit", "-q", "--allow-empty", "-m", "fx")
	want := run("-C", dir, "rev-parse", "HEAD")
	got := cptsReadHead(dir, true)
	if got != want || len(got) != 40 {
		t.Fatalf("取数面哑化：want %q，got %q（len=%d）", want, got, len(got))
	}
	if s := cptsReadHead(dir, false); len(s) == 0 || len(s) >= 40 {
		t.Fatalf("短 sha 档异常：%q", s)
	}
	if s := cptsReadHead(filepath.Join(dir, "no-such-tree"), true); s != "" {
		t.Fatalf("缺树档应回空串（缺件 ≠ 绿），实得 %q", s)
	}
}

// ⑤ 自检档整体：钉 /usr/bin/python3（GAP-61）⇒ rc=0 且两面（真执行面/取数面）都在。
func TestCptsSelfTestEntryGreen(t *testing.T) {
	rc, out, _ := cptsTRun([]string{"--self-test"}, map[string]string{"ZERG_PST_PYTHON": cptsTPy})
	if rc != 0 {
		t.Fatalf("自检档 rc 期望 0，实得 %d\n%s", rc, out)
	}
	if !strings.Contains(out, "自检结论: 全过") {
		t.Fatalf("自检未全过：\n%s", out)
	}
	if !strings.Contains(out, "runprobe/readhead-truly-live") {
		t.Fatalf("自检缺真执行面/取数面两格：\n%s", out)
	}
	if !strings.Contains(out, "run_probe 真跑合成命令 ⇒ rc=0") {
		t.Fatalf("自检缺「真跑」格：\n%s", out)
	}
}

// ⑥ 三档报话面。
func TestCptsEntryListRulesAndErrors(t *testing.T) {
	rc, out, _ := cptsTRunClean([]string{"--list-rules"})
	if rc != 0 || !strings.HasPrefix(out, "⑭ 公开树自洽检查（③+⑦ 合并）· 只报告档起步 · 不进 FAST_STEPS\n") {
		t.Fatalf("--list-rules：rc=%d out 首行不符", rc)
	}
	if !strings.Contains(out, "退码：0 全绿 · 1 有命中 · 2 不给结论（clone 失败/缺件/空转/自检不过）") {
		t.Fatalf("--list-rules 缺退码行")
	}
	rc2, _, err2 := cptsTRunClean([]string{"--json", "schema,bogus"})
	if rc2 != 2 {
		t.Fatalf("--json 打错字段名：rc 期望 2，实得 %d", rc2)
	}
	if !strings.Contains(err2, "⛔ `--json` 不认的字段名：`bogus`") ||
		!strings.Contains(err2, "可选字段表：`schema` · `tree` · `head` · `local_head` · `correspondence` · `correspondence_why` · `note` · `rows`") {
		t.Fatalf("--json 报话不符：%q", err2)
	}
	rc3, _, err3 := cptsTRunClean([]string{"--no-such-flag"})
	if rc3 != 2 || !strings.HasPrefix(err3, "usage: check-public-tree-selfcheck.py ") ||
		!strings.Contains(err3, "check-public-tree-selfcheck.py: error: unrecognized arguments: --no-such-flag") {
		t.Fatalf("未知旗标档不符：rc=%d stderr=%q", rc3, err3)
	}
	rc4, _, err4 := cptsTRunClean([]string{"--timeout", "abc"})
	if rc4 != 2 || !strings.Contains(err4, "error: argument --timeout: invalid int value: 'abc'") {
		t.Fatalf("非法 int 档不符：rc=%d stderr=%q", rc4, err4)
	}
}

// ⑦ A 类回归牙：clone 失败档**不许残留**临时树（把主件 `defer os.RemoveAll` 删掉 ⇒ 本格红）。
func TestCptsEntryCloneFailureNoTempLeak(t *testing.T) {
	pat := regexp.MustCompile(`^zerg-selfcheck-`)
	count := func() int {
		ents, err := os.ReadDir(os.TempDir())
		if err != nil {
			t.Fatalf("读 TMPDIR 失败：%v", err)
		}
		n := 0
		for _, e := range ents {
			if e.IsDir() && pat.MatchString(e.Name()) {
				n++
			}
		}
		return n
	}
	before := count()
	rc, out, _ := cptsTRun([]string{"--repo", "/nonexistent/xyz-zzz.git"},
		map[string]string{"ZERG_PST_FROZEN": "1"})
	after := count()
	if rc != 2 {
		t.Fatalf("clone 失败档 rc 期望 2，实得 %d\n%s", rc, out)
	}
	if !strings.Contains(out, "公开树拿不到：clone 失败（rc=") ||
		!strings.Contains(out, "⇒ 缺件 ≠ 绿：本门不给结论（rc=2）—— 不许把「clone 失败」当通过") {
		t.Fatalf("clone 失败报话不符：\n%s", out)
	}
	if delta := after - before; delta != 0 {
		t.Fatalf("A 类泄漏回归：临时树残留 delta=%d（before=%d after=%d）", delta, before, after)
	}
}

// ⑧ 输出行序守恒：dedupe 的首次出现序（✗ 不受 map 迭代序影响）。
func TestCptsDedupeFirstSeenOrder(t *testing.T) {
	in := []string{
		"  a.go:10: 夹具失效：x",
		"  b.go:20: LEAK: y",
		"  a.go:10: 夹具失效：x",
		"  c.go:30: LEAK: z",
	}
	grp, total := cptsDedupe(in, 12)
	if total != 3 || len(grp) != 3 {
		t.Fatalf("去重组数：total 期望 3，实得 %d（grp=%d）", total, len(grp))
	}
	if grp[0].text != "a.go:10: 夹具失效：x" || grp[0].count != 2 ||
		grp[1].text != "b.go:20: LEAK: y" || grp[2].text != "c.go:30: LEAK: z" {
		t.Fatalf("首次出现序被破坏：%+v", grp)
	}
	if g2, _ := cptsDedupe(in, 2); len(g2) != 2 {
		t.Fatalf("cap 未生效：%d", len(g2))
	}
}

// `--tree` 不存在的目录档 + shas-only 三档（不走网、不走探针）。
func TestCptsEntryTreeMissingAndShasOnly(t *testing.T) {
	rc, out, _ := cptsTRunClean([]string{"--tree", filepath.Join(t.TempDir(), "nope")})
	if rc != 2 || !strings.Contains(out, "缺件：--tree ") || !strings.Contains(out, "不是目录 ⇒ 不给结论") {
		t.Fatalf("--tree 缺件档不符：rc=%d out=%q", rc, out)
	}
	dir := t.TempDir()
	a40 := strings.Repeat("a", 40)
	b40 := strings.Repeat("b", 40)
	rc2, out2, _ := cptsTRunClean([]string{"--tree", dir, "--shas-only", "--local-head", a40, "--public-head", a40})
	if rc2 != 0 || !strings.Contains(out2, "⇒ --shas-only：**只核双方 HEAD sha**（不跑探针）⇒ 对应 · rc=0") {
		t.Fatalf("--shas-only 对应档不符：rc=%d out=%q", rc2, out2)
	}
	rc3, out3, _ := cptsTRunClean([]string{"--tree", dir, "--shas-only", "--local-head", a40, "--public-head", b40})
	if rc3 != 1 || !strings.Contains(out3, "不对应") {
		t.Fatalf("--shas-only 不对应档不符：rc=%d out=%q", rc3, out3)
	}
	rc4, out4, _ := cptsTRunClean([]string{"--tree", dir, "--shas-only"})
	_ = rc4
	if !strings.Contains(out4, "公开树 HEAD读不到") && !strings.Contains(out4, "未证") {
		t.Fatalf("--shas-only 未证档不符：rc=%d out=%q", rc4, out4)
	}
}
