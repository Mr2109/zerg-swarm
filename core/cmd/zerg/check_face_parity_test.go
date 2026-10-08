// check_face_parity_test.go —— `check_face_parity.go`（判据件 `check-face-parity.py` 的 Go 真搬）的
// 成对负控测试。前缀 `TestCfpg`（现读本件全部 Test* 的公共前缀）。
//
// 边界：✗ 不进 main.go 命令树、✗ 不注册命令、✗ 不删 py 原件（M2 边界；本件已挂
// `gateCheckGoEntry["check-face-parity"]` ⇒ 走 inproc）。
//
// ★ 有牙三腿（每条负控都钉「哑化后必红」）：F1 物化 / F2 禁归档形态 / F3 面自陈各一条，
// 缺件档 + 空域档 + 自指排除档 + F4/F5 报面档各一条；另钉 argparse / 人面 / JSON 三面的逐字正文。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	cfpgTGoodHook = "#!/usr/bin/env bash\n# 索引面\n" +
		"git -C R checkout-index -a -f --prefix=X/\n" +
		"ZERG_GATE_REPO_ROOT=X bash gate --fast\n"
	cfpgTGoodGo = "// 索引面\n" +
		"func f(){ matRoot,_:=materializeIndex(root); _=gateEnv(matRoot) }\n" +
		"const k = \"ZERG_GATE_REPO_ROOT\"\n"
)

// cfpgTMkFixture —— 合成一棵最小三件树（+ scripts/gates/seed.py，供空域守卫用）。
func cfpgTMkFixture(t *testing.T, hook, gofile string, extraGates map[string]string) string {
	t.Helper()
	d := t.TempDir()
	for _, sub := range []string{".githooks", "core/cmd/zerg", "scripts/gates"} {
		if err := os.MkdirAll(filepath.Join(d, sub), 0o755); err != nil {
			t.Fatalf("建夹具目录 %s: %v", sub, err)
		}
	}
	if err := os.WriteFile(filepath.Join(d, ".githooks", "pre-commit"), []byte(hook), 0o644); err != nil {
		t.Fatal(err)
	}
	if gofile != "" {
		if err := os.WriteFile(filepath.Join(d, "core", "cmd", "zerg", "family_repo_commit.go"),
			[]byte(gofile), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(d, "scripts", "gates", "seed.py"), []byte("x = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, body := range extraGates {
		if err := os.WriteFile(filepath.Join(d, "scripts", "gates", name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return d
}

func cfpgTRun(argv []string) (int, string, string) {
	var out, errb bytes.Buffer
	rc := checkFaceParityGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

// ── F1 物化索引（牙 · 哑化「缺必须串 ⇒ 进 hard」这一处 ⇒ 本条必红）────────────────────────
func TestCfpgF1MaterializeTeeth(t *testing.T) {
	d := cfpgTMkFixture(t, cfpgTGoodHook, cfpgTGoodGo, nil)
	if rc, _, _ := cfpgTRun([]string{d}); rc != 0 {
		t.Fatalf("正控：两条出口齐 ⇒ 应 rc=0，得 %d", rc)
	}
	// 负控：钩子去掉物化串 ⇒ F1 硬命中 rc=1
	h := filepath.Join(d, ".githooks", "pre-commit")
	if err := os.WriteFile(h, []byte(strings.ReplaceAll(cfpgTGoodHook, cfpgNeedCheckout, "XX")), 0o644); err != nil {
		t.Fatal(err)
	}
	rc, out, _ := cfpgTRun([]string{d})
	if rc != 1 {
		t.Fatalf("F1 牙：钩子缺物化串 ⇒ 应 rc=1，得 %d（输出 %q）", rc, out)
	}
	if !strings.Contains(out, "✗ F1 .githooks/pre-commit（出口② 提交钩子（物化索引 + 指面跑闸））：缺 "+
		cfpgNeedCheckout) {
		t.Fatalf("F1 牙：应逐字点名该件 + 缺失串，得 %q", out)
	}
}

// ── F3 面自陈（牙 · 哑化「自陈缺 ⇒ 进 selfdecl」这一处 ⇒ 本条必红）────────────────────────
func TestCfpgF3SelfDeclTeeth(t *testing.T) {
	d := cfpgTMkFixture(t, cfpgTGoodHook, cfpgTGoodGo, nil)
	// 负控：钩子去掉面自陈词 ⇒ F3 硬命中 rc=1
	h := filepath.Join(d, ".githooks", "pre-commit")
	if err := os.WriteFile(h, []byte(strings.ReplaceAll(cfpgTGoodHook, cfpgSelfDeclWord, "XX")), 0o644); err != nil {
		t.Fatal(err)
	}
	rc, out, _ := cfpgTRun([]string{d})
	if rc != 1 {
		t.Fatalf("F3 牙：钩子缺面自陈词 ⇒ 应 rc=1，得 %d（输出 %q）", rc, out)
	}
	if !strings.Contains(out, "✗ F3 .githooks/pre-commit：面自陈缺 '"+cfpgSelfDeclWord+"'") {
		t.Fatalf("F3 牙：应逐字点名该件 + 缺失词，得 %q", out)
	}
}

// ── F2 禁归档形态（牙）：代码行命中 ⇒ rc=1；注释行命中不算 ─────────────────────────────
func TestCfpgF2ForbidTeeth(t *testing.T) {
	d := cfpgTMkFixture(t, cfpgTGoodHook, cfpgTGoodGo, nil)
	g := filepath.Join(d, "core", "cmd", "zerg", "family_repo_commit.go")
	if err := os.WriteFile(g, []byte(cfpgTGoodGo+"var zzArchive = \""+cfpgForbidArchive+" HEAD\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rc, out, _ := cfpgTRun([]string{d})
	if rc != 1 {
		t.Fatalf("F2 牙：代码行出现禁用形态 ⇒ 应 rc=1，得 %d（输出 %q）", rc, out)
	}
	if !strings.Contains(out, "✗ F2 core/cmd/zerg/family_repo_commit.go：出现禁用串 "+
		cfpgPyRepr(cfpgForbidArchive)) {
		t.Fatalf("F2 牙：应逐字点名，得 %q", out)
	}
	// 成对反向：**注释行**写该形态 ⇒ 不算命中（说明文里写「不用 X」不该被自己点名）
	if err := os.WriteFile(g, []byte(cfpgTGoodGo+"// 我们不用 "+cfpgForbidArchive+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if rc, _, _ := cfpgTRun([]string{d}); rc != 0 {
		t.Fatalf("F2 反向：注释行含该形态 ⇒ 应 rc=0，得 %d", rc)
	}
}

// ── 缺件 / 空域 / 自指排除 ─────────────────────────────────────────────────────────────
func TestCfpgMissingFileRc2(t *testing.T) {
	d := cfpgTMkFixture(t, cfpgTGoodHook, cfpgTGoodGo, nil)
	if err := os.Remove(filepath.Join(d, "core", "cmd", "zerg", "family_repo_commit.go")); err != nil {
		t.Fatal(err)
	}
	rc, out, _ := cfpgTRun([]string{d})
	if rc != 2 {
		t.Fatalf("缺件 ⇒ 应 rc=2（不给结论），得 %d", rc)
	}
	want := "✗ 缺件 / 读不到：core/cmd/zerg/family_repo_commit.go（[Errno 2] No such file or directory: '" +
		filepath.Join(d, "core", "cmd", "zerg", "family_repo_commit.go") + "')⇒ 判不了（不许当好）\n"
	if !strings.Contains(out, "✗ 缺件 / 读不到：core/cmd/zerg/family_repo_commit.go（[Errno 2] No such file or directory: '") {
		t.Fatalf("缺件正文应含 ENOENT 形态，得 %q（want 前缀 %q）", out, want)
	}
}

func TestCfpgEmptyDomainRc2(t *testing.T) {
	d := cfpgTMkFixture(t, cfpgTGoodHook, cfpgTGoodGo, nil)
	if err := os.Remove(filepath.Join(d, "scripts", "gates", "seed.py")); err != nil {
		t.Fatal(err)
	}
	rc, out, _ := cfpgTRun([]string{d})
	if rc != 2 {
		t.Fatalf("空域 ⇒ 应 rc=2（fail-closed · 不许空域当绿），得 %d", rc)
	}
	if !strings.Contains(out, "空域：scripts/gates 下取不到任何 .py 件") {
		t.Fatalf("空域正文应点名，得 %q", out)
	}
}

func TestCfpgSelfRefExcluded(t *testing.T) {
	// 把 py 原件基名的一份「两面选择器齐备」的件放进扫描面 ⇒ 必须被自指排除（不进 mixed）
	d := cfpgTMkFixture(t, cfpgTGoodHook, cfpgTGoodGo, map[string]string{
		cfpgSelfName: "import os\nX = \"git status --porcelain\"\nZERG_GATE_REPO_ROOT = os.environ[\"ZERG_GATE_REPO_ROOT\"]\n",
	})
	rc, out, _ := cfpgTRun([]string{d})
	if rc != 0 {
		t.Fatalf("自指排除：应 rc=0，得 %d", rc)
	}
	if strings.Contains(out, cfpgGatesDirRel+"/"+cfpgSelfName) {
		t.Fatalf("自指排除：本件基名不应出现在 mixed，得 %q", out)
	}
}

// ── F4 / F5 报面（只报告 · 不计 rc）──────────────────────────────────────────────────────
func TestCfpgReportF4F5(t *testing.T) {
	d := cfpgTMkFixture(t, cfpgTGoodHook, cfpgTGoodGo, map[string]string{
		"z_mix.py": "import os\nX = \"git diff --name-only HEAD\"\n" +
			"ZERG_GATE_REPO_ROOT = os.environ[\"ZERG_GATE_REPO_ROOT\"]\n",
		"check-judge-ledger.py": "L = \"judge-run-ledger.jsonl\"\nY = \"git status --porcelain\"\n",
	})
	rc, out, _ := cfpgTRun([]string{d})
	if rc != 0 {
		t.Fatalf("F4/F5 只报告 ⇒ 应 rc=0，得 %d", rc)
	}
	if !strings.Contains(out, cfpgGatesDirRel+"/z_mix.py") {
		t.Fatalf("F4 应点名混两棵树的件，得 %q", out)
	}
	if !strings.Contains(out, cfpgGatesDirRel+"/check-judge-ledger.py") {
		t.Fatalf("F5 应点名台账类件，得 %q", out)
	}
	if !strings.Contains(out, "F4 同一包混入两棵树（只报告）：1 件") ||
		!strings.Contains(out, "F5 台账类判据应改索引面（只报告）：1 件") {
		t.Fatalf("报面计数应逐字，得 %q", out)
	}
}

// ── argparse / 人面 / JSON 三面逐字 ─────────────────────────────────────────────────────
func TestCfpgHelpAndArgErrors(t *testing.T) {
	rc, out, errb := cfpgTRun([]string{"-h"})
	if rc != 0 || errb != "" {
		t.Fatalf("-h ⇒ rc=0 且 stderr 空，得 rc=%d err=%q", rc, errb)
	}
	wantHelp := "usage: check-face-parity.py [-h] [--self-test] [--json] [root]\n\n" +
		"positional arguments:\n  root\n\n" +
		"optional arguments:\n  -h, --help   show this help message and exit\n" +
		"  --self-test\n  --json\n"
	if out != wantHelp {
		t.Fatalf("-h 正文应逐字，得 %q", out)
	}
	rc, out, errb = cfpgTRun([]string{"--bogus", "."})
	if rc != 2 || out != "" {
		t.Fatalf("未知旗标 ⇒ rc=2 且 stdout 空，得 rc=%d out=%q", rc, out)
	}
	wantErr := "usage: check-face-parity.py [-h] [--self-test] [--json] [root]\n" +
		"check-face-parity.py: error: unrecognized arguments: --bogus\n"
	if errb != wantErr {
		t.Fatalf("未知旗标正文应逐字，得 %q", errb)
	}
	rc, _, errb = cfpgTRun([]string{"--self-test=x"})
	if rc != 2 || !strings.Contains(errb, "argument --self-test: ignored explicit argument 'x'") {
		t.Fatalf("store_true 带值应逐字报话，得 rc=%d err=%q", rc, errb)
	}
}

func TestCfpgJSONFace(t *testing.T) {
	d := cfpgTMkFixture(t, cfpgTGoodHook, cfpgTGoodGo, nil)
	rc, out, _ := cfpgTRun([]string{"--json", d})
	if rc != 0 {
		t.Fatalf("--json 干净夹具 ⇒ rc=0，得 %d", rc)
	}
	want := "{\"schema\": \"check-face-parity/1\", \"rc\": 0, \"root\": \"" + d + "\", " +
		"\"result\": {\"hard\": [], \"selfdecl\": [], \"forbid\": [], \"mixed\": [], \"ledger\": []}}\n"
	if out != want {
		t.Fatalf("--json 正文应逐字（含空集合 [] 与分隔符 \", \"）：\n got=%q\nwant=%q", out, want)
	}
	// 缺件档：result 只有 error 一键（缺键形态 · ✗ 不补默认键）
	if err := os.Remove(filepath.Join(d, "core", "cmd", "zerg", "family_repo_commit.go")); err != nil {
		t.Fatal(err)
	}
	_, out, _ = cfpgTRun([]string{"--json", d})
	if !strings.Contains(out, "\"result\": {\"error\": ") || strings.Contains(out, "\"hard\"") {
		t.Fatalf("缺件档 JSON 应只含 error 键，得 %q", out)
	}
}

// ── py 路径原语三态（`.filepath.Join` 不适用面 · 声明 (a)）──────────────────────────────
func TestCfpgPathPrimitives(t *testing.T) {
	if got := cfpgPyJoin("a", "../b"); got != "a/../b" { // py 原样留 `..`
		t.Fatalf("pyJoin 应保留 `..` 段，得 %q", got)
	}
	if got := cfpgPyJoin("repo", "/abs"); got != "/abs" { // 后段绝对 ⇒ 重置
		t.Fatalf("pyJoin 后段绝对应重置，得 %q", got)
	}
	if got := cfpgPyJoin("a", "", "b"); got != "a/b" {
		t.Fatalf("pyJoin 空段应得 a/b，得 %q", got)
	}
	if got := cfpgPyJoin("a", ""); got != "a/" {
		t.Fatalf("pyJoin 尾空段应保留尾斜杠，得 %q", got)
	}
	if got := cfpgPyNormpath("//a/b/../c"); got != "//a/c" { // 恰两个前导斜杠保留
		t.Fatalf("pyNormpath 应保留双前导斜杠，得 %q", got)
	}
	if got := cfpgPyAbspath("/x", "a/b"); got != "/x/a/b" {
		t.Fatalf("pyAbspath 应纯词法拼接，得 %q", got)
	}
}

// ── 自检档（7 格全过）＋ 自回收（TMPDIR 前后 delta=0）──────────────────────────────────
func TestCfpgSelfTestGrid(t *testing.T) {
	// 把 TMPDIR 收进私有域 ⇒ 「跑前/跑后同域计数」判据不受门分片并发影响。
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	before := cfpgTCount(t, tmp)
	var out bytes.Buffer
	rc := cfpgSelfTest(&out)
	if rc != 0 {
		t.Fatalf("自检应 rc=0，得 %d\n%s", rc, out.String())
	}
	if !strings.Contains(out.String(), "── 全过（7 格）──") {
		t.Fatalf("自检应 7 格全过，得 %q", out.String())
	}
	if strings.Contains(out.String(), "✗") {
		t.Fatalf("自检不应有红格，得 %q", out.String())
	}
	if after := cfpgTCount(t, tmp); after != before {
		t.Fatalf("自检应自回收（TMPDIR 前后 delta=0），before=%d after=%d", before, after)
	}
}

func cfpgTCount(t *testing.T, dir string) int {
	t.Helper()
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("读 TMPDIR: %v", err)
	}
	return len(ents)
}
