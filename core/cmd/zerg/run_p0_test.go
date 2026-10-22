// run_p0_test.go —— 判据件 `run_p0`（Go 真搬 · 见 run_p0.go）的**牙**。
//
// 三面各一组，缺一面就等于没有判据：
//
//	① 表一致性：Go 的用例表必须与 `scripts/chat-eval/run_p0.py` 的 CASES/CLEAN_CASES **逐条同**
//	   （id · 说明 · argv · 期望 rc）—— 这是两半并存的**漂移牙**：py 改一条而 Go 没跟 ⇒ 本格红。
//	② 两态：同一夹具下「制品都合规」⟷「--help 起不来」必须读出不同退码（✗ 禁恒绿）。
//	③ 元测试（套件的牙）：负控把 p0-01 期望改错后**必须红**；恒绿套件必须被自己判死。
//
// ★ 全部仓外合成：夹具根 = t.TempDir()，被判制品 = 一段 `#!/bin/sh` 脚本 ⇒ 不碰真仓、不碰
// 真 `bin/zerg`、不写真账（判据体自己会把 HOME/ZERG_STATE_DIR/TMPDIR 指到临时树）。
package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// ── 夹具 ────────────────────────────────────────────────────────────────────

// rp0FakeRoot —— 造一个「仓根」：`<根>/bin/zerg` = 给定脚本（0755）。
func rp0FakeRoot(t *testing.T, script string) string {
	t.Helper()
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatalf("夹具 bin/ 建不起来：%v", err)
	}
	if err := os.WriteFile(filepath.Join(bin, "zerg"), []byte(script), 0o755); err != nil {
		t.Fatalf("夹具 bin/zerg 写不进去：%v", err)
	}
	return root
}

// rp0ScriptAllPass —— 制品「全合规」档：两条 `--help` 成功且有输出，其余一律用法错（rc=2 · 零 stdout）。
const rp0ScriptAllPass = `#!/bin/sh
if [ "$1" = "chat" ] && [ "$2" = "--help" ]; then echo "usage: zerg chat"; exit 0; fi
if [ "$1" = "chat" ] && [ "$2" = "ask" ] && [ "$3" = "--help" ]; then echo "usage: zerg chat ask"; exit 0; fi
exit 2
`

// rp0ScriptNoHelp —— 制品「--help 起不来」档：一律 rc=2 零 stdout ⇒ 只有 p0-09/p0-10 会红。
const rp0ScriptNoHelp = `#!/bin/sh
exit 2
`

// rp0ScriptSick —— 制品「p0-01 已经坏了」档：`chat ask --json`（p0-01 的 argv）错误地 rc=0，
// 其余照旧（两条 --help 成功 · 其它用法错 rc=2）⇒ 正常档只红 p0-01 一条，而**改错后的期望**
// 反而成立 ⇒ 元测试必须报「没红」（这就是 py 那句「套件是恒绿的（无牙）」的真身）。
const rp0ScriptSick = `#!/bin/sh
if [ "$1" = "chat" ] && [ "$2" = "--help" ]; then echo "usage: zerg chat"; exit 0; fi
if [ "$1" = "chat" ] && [ "$2" = "ask" ] && [ "$3" = "--help" ]; then echo "usage: zerg chat ask"; exit 0; fi
if [ "$1" = "chat" ] && [ "$2" = "ask" ] && [ "$3" = "--json" ]; then exit 0; fi
exit 2
`

func rp0MustRun(t *testing.T, root string, selfTest bool) (int, string, string) {
	t.Helper()
	var out, errw bytes.Buffer
	rc := rp0Main(&out, &errw, root, selfTest)
	return rc, out.String(), errw.String()
}

// ── ① 表一致性（与 py 半逐条对拍）────────────────────────────────────────────

// rp0PyCase —— 从 py 件里解析出的一条（只取判据面四列）。
type rp0PyCase struct {
	ID   string
	Desc string
	Argv []string
	RC   int
}

var (
	// py CASES 的元组行：("p0-01", "说明", ["chat", …], 2, 0),
	rp0PyTupleRE = regexp.MustCompile(`\("(p0-\d\d)",\s*"([^"]*)",\s*\[([^\]]*)\],\s*(\d+)`)
	// py CLEAN_CASES 的字典项：\"p0-12\": (\"说明\", [\"chat\", …], 2),
	rp0PyDictRE = regexp.MustCompile(`"(p0-\d\d)"\s*:\s*\(\s*"([^"]*)",\s*\[([^\]]*)\]\s*,\s*(\d+)`)
)

// rp0ParsePyArgv —— 把 py 行内的 `"a", "b"` 段切成 []string（空串也照收）。
func rp0ParsePyArgv(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ", ")
	for i, p := range parts {
		parts[i] = strings.Trim(strings.TrimSpace(p), `"`)
	}
	return parts
}

// TestRunP0CaseTableMatchesPy 表一致性：Go 侧 12 条必须与 py 半的 CASES + CLEAN_CASES 逐条同
// （id / 说明 / argv / 期望 rc 四列全比）。py 是两半并存的另一半 ⇒ 它一变而 Go 没跟，本格即红。
func TestRunP0CaseTableMatchesPy(t *testing.T) {
	py := filepath.Join("..", "..", "..", "scripts", "chat-eval", "run_p0.py")
	raw, err := os.ReadFile(py)
	if err != nil {
		t.Fatalf("py 半读不到（%s）：%v ⇒ ✗ 不许 Skip（漂移牙不能空转）", py, err)
	}
	want := map[string]rp0PyCase{}
	for _, ln := range strings.Split(string(raw), "\n") {
		var m []string
		if mm := rp0PyTupleRE.FindStringSubmatch(ln); mm != nil {
			m = mm
		} else if mm := rp0PyDictRE.FindStringSubmatch(ln); mm != nil {
			m = mm
		} else {
			continue
		}
		rc := 0
		if _, err := fmt.Sscanf(m[4], "%d", &rc); err != nil {
			t.Fatalf("py 行 rc 解析不了：%q", ln)
		}
		want[m[1]] = rp0PyCase{ID: m[1], Desc: m[2], Argv: rp0ParsePyArgv(m[3]), RC: rc}
	}
	if len(want) != 12 {
		t.Fatalf("py 半应解析出 12 条（11 条 CASES ＋ p0-12 一条副作用面），得 %d 条：%v", len(want), rp0KeysOfPy(want))
	}

	got := map[string]rp0Case{}
	for _, c := range rp0Cases {
		got[c.ID] = c
	}
	got[rp0CleanCase.ID] = rp0CleanCase
	if len(got) != len(want) {
		t.Fatalf("条数不同：Go %d 条 ⟷ py %d 条（Go=%v）", len(got), len(want), rp0KeysOfGo(got))
	}
	for id, w := range want {
		g, ok := got[id]
		if !ok {
			t.Errorf("py 有 %s 而 Go 没有 ⇒ 表漏条（判据少一格就是少一条牙）", id)
			continue
		}
		if g.Desc != w.Desc {
			t.Errorf("%s 说明漂移：Go %q ⟷ py %q", id, g.Desc, w.Desc)
		}
		if g.WantRC != w.RC {
			t.Errorf("%s 期望 rc 漂移：Go %d ⟷ py %d", id, g.WantRC, w.RC)
		}
		if strings.Join(g.Argv, "\x00") != strings.Join(w.Argv, "\x00") {
			t.Errorf("%s argv 漂移：\n  Go %q\n  py %q", id, g.Argv, w.Argv)
		}
	}
}

func rp0KeysOfPy(m map[string]rp0PyCase) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func rp0KeysOfGo(m map[string]rp0Case) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ── ② 两态（同一夹具 · 两种制品行为）────────────────────────────────────────

// TestRunP0AllGreen 制品全合规 ⇒ 12 条全过 ＋ 内嵌元测试有牙 ⇒ rc=0。
func TestRunP0AllGreen(t *testing.T) {
	root := rp0FakeRoot(t, rp0ScriptAllPass)
	rc, out, errw := rp0MustRun(t, root, false)
	if rc != 0 {
		t.Fatalf("期望 rc=0（全绿），得 rc=%d\nstdout:\n%s\nstderr:%s", rc, out, errw)
	}
	for _, want := range []string{
		"PASS p0-01",
		"PASS p0-09",
		"PASS p0-10",
		"PASS p0-12 用法错零副作用（不许建会话/登记别名）（rc=2 · 新增落盘=[]）",
		"P0 合计 11 条 · 失败 0 条（副作用面另 1 条 · 失败 0 条）",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("输出缺 %q\n实读：\n%s", want, out)
		}
	}
}

// TestRunP0TwoStates 成对断言：同一夹具「制品全合规(0)」⟷「--help 起不来(1)」退码必不同，
// 且红的一态**逐条点名** p0-09/p0-10（✗ 禁恒绿 · ✗ 禁只给总数）。
func TestRunP0TwoStates(t *testing.T) {
	rcGreen, _, _ := rp0MustRun(t, rp0FakeRoot(t, rp0ScriptAllPass), false)
	rcRed, outRed, _ := rp0MustRun(t, rp0FakeRoot(t, rp0ScriptNoHelp), false)
	if rcGreen == rcRed {
		t.Fatalf("两态同码 rc=%d ⇒ 判据无鉴别力（恒绿）", rcGreen)
	}
	if rcRed != 1 {
		t.Fatalf("「--help 起不来」应判 rc=1，得 %d\n%s", rcRed, outRed)
	}
	if !strings.Contains(outRed, "FAIL p0-09") || !strings.Contains(outRed, "FAIL p0-10") {
		t.Errorf("红态应逐条点名 p0-09 / p0-10，实读：\n%s", outRed)
	}
	if !strings.Contains(outRed, "P0 合计 11 条 · 失败 2 条") {
		t.Errorf("红态应报「失败 2 条」，实读：\n%s", outRed)
	}
	// 副作用面那条在两种制品下都该过（它只判「别名有没有被登记」）。
	if !strings.Contains(outRed, "PASS p0-12") {
		t.Errorf("副作用面不该被 --help 面牵连，实读：\n%s", outRed)
	}
}

// ── ③ 元测试（套件的牙）────────────────────────────────────────────────────

// TestRunP0SelfTest 负控：p0-01 的期望被改错后必须红；恒绿套件必须被自己判死。
func TestRunP0SelfTest(t *testing.T) {
	rcOK, outOK, errOK := rp0MustRun(t, rp0FakeRoot(t, rp0ScriptAllPass), true)
	if rcOK != 0 {
		t.Fatalf("负控应红 ⇒ --self-test 应 rc=0，得 %d（stderr=%s）", rcOK, errOK)
	}
	// ★ 逐字（防波门 check-py-go-pairing-drift 的口径）：本行必须与 py 半 `--self-test` 逐字节同。
	if want := "元测试：负控已红（1 条）⇒ 套件有牙 ✓\n"; outOK != want {
		t.Fatalf("--self-test 输出须与 py 半逐字同\n  期望 %q\n  实读 %q", want, outOK)
	}
	// 元测试档 ✗ 不许跑主用例（否则「只自证」变成「又跑一遍全套」）。
	if strings.Contains(outOK, "P0 合计") {
		t.Errorf("--self-test 档不该跑主用例，实读：\n%s", outOK)
	}

	// 制品本来就坏（p0-01 已失效）⇒ 改错后的期望反而成立 ⇒ 元测试必须报「没红」并判失败。
	rcBad, outBad, _ := rp0MustRun(t, rp0FakeRoot(t, rp0ScriptSick), true)
	if rcBad != 1 {
		t.Fatalf("无牙档应 rc=1（套件无牙判死），得 %d\n%s", rcBad, outBad)
	}
	if !strings.Contains(outBad, "✗ 元测试没红 ⇒ 套件是恒绿的（无牙）") {
		t.Errorf("无牙档应报「元测试没红」，实读：\n%s", outBad)
	}
}

// TestRunP0EmbeddedMetaFailsGreen 内嵌元测试真的挡在前面：制品「p0-01 已坏」⇒ 主用例红一条
// ＋ 内嵌元测试没红 ⇒ rc=1（并且两处都点名，不靠单一面）。
func TestRunP0EmbeddedMetaFailsGreen(t *testing.T) {
	rc, out, _ := rp0MustRun(t, rp0FakeRoot(t, rp0ScriptSick), false)
	if rc != 1 {
		t.Fatalf("无牙档应 rc=1，得 %d\n%s", rc, out)
	}
	if !strings.Contains(out, "FAIL p0-01") {
		t.Errorf("应先红 p0-01（制品坏了），实读：\n%s", out)
	}
	if !strings.Contains(out, "✗ 内嵌元测试没红 ⇒ 套件无牙 ⇒ 判失败") {
		t.Errorf("应点名内嵌元测试没红，实读：\n%s", out)
	}
}

// ── ④ 隔离面 / 取不到制品 / 用法 ─────────────────────────────────────────────

// TestRunP0EnvIsolation 一例一临时树：三个旋钮必须指向该树 · 三枚不许带进来 · 同键只留一份。
func TestRunP0EnvIsolation(t *testing.T) {
	dir := t.TempDir()
	env := rp0EnvFor(dir)
	got := map[string]string{}
	seen := map[string]int{}
	for _, kv := range env {
		k := kv
		if i := strings.IndexByte(kv, '='); i >= 0 {
			k = kv[:i]
		}
		seen[k]++
		got[k] = kv[len(k)+1:]
	}
	for k, n := range seen {
		if n != 1 {
			t.Errorf("环境键 %s 出现 %d 次 ⇒ 子进程可能取到旧值（隔离失效）", k, n)
		}
	}
	for k, wantUnder := range map[string]string{
		"HOME":           filepath.Join(dir, "home"),
		"ZERG_STATE_DIR": filepath.Join(dir, "state"),
		"TMPDIR":         filepath.Join(dir, "tmp"),
	} {
		if got[k] != wantUnder {
			t.Errorf("%s 应指向临时树（%q），得 %q", k, wantUnder, got[k])
		}
	}
	for _, k := range []string{"ZERG_REPO", "ZERG_GATE_LOCK_DIR", "GIT_INDEX_FILE"} {
		if _, ok := got[k]; ok {
			t.Errorf("%s 不许带进被判进程（py 逐字剔这三枚）", k)
		}
	}
}

// TestRunP0MissingArtifact 制品不在盘 ⇒ **不给结论**（rc=2）：✗ 不许把环境的错记成契约的错。
func TestRunP0MissingArtifact(t *testing.T) {
	root := t.TempDir() // 无 bin/zerg
	rc, out, errw := rp0MustRun(t, root, false)
	if rc != 2 {
		t.Fatalf("制品缺失应 rc=2，得 %d\nstdout:%s\nstderr:%s", rc, out, errw)
	}
	if !strings.Contains(errw, "不给结论") {
		t.Errorf("应报「不给结论」，实读 stderr：%s", errw)
	}
	if out != "" {
		t.Errorf("rc=2 档不该有判定输出，实读：%q", out)
	}
}

// TestRunP0Usage 用法面三档（未知旗标 / 互斥 / 缺值）＋ 帮助档。
func TestRunP0Usage(t *testing.T) {
	root := rp0FakeRoot(t, rp0ScriptAllPass)
	cases := []struct {
		name string
		argv []string
		want int
	}{
		{"未知旗标", []string{"--nope"}, 2},
		{"--check 与 --self-test 互斥", []string{"--check", "--self-test"}, 2},
		{"--root 缺值", []string{"--root"}, 2},
		{"--root= 空值", []string{"--root="}, 2},
		{"帮助档", []string{"--help"}, 0},
	}
	for _, c := range cases {
		var out, errw bytes.Buffer
		if rc := runP0GateMain(&out, &errw, c.argv); rc != c.want {
			t.Errorf("%s：期望 rc=%d，得 %d（stdout=%q stderr=%q）", c.name, c.want, rc, out.String(), errw.String())
		}
	}
	// `--root <根>` 显式档要真被判（✗ 不回落本仓）。
	var out, errw bytes.Buffer
	if rc := runP0GateMain(&out, &errw, []string{"--root", root}); rc != 0 {
		t.Fatalf("显式 --root 应判绿，得 rc=%d\n%s\n%s", rc, out.String(), errw.String())
	}
	if !strings.Contains(out.String(), "P0 合计 11 条") {
		t.Errorf("显式 --root 应真跑（输出应有合计行），实读：\n%s", out.String())
	}
}
