// inplace_window_test.go —— `inplace_window.go`（`scripts/docs/inplace-window.py` 的 Go 真搬）
// 成对负控。断言全是「只有实现对才过」的**逐字**等式（哑化任一语义面必红 —— 见回执的
// 仓外副本哑化实证：把主件语义改坏 ⇒ 本件 FAIL 带行号）。
//
// ★ 无 TestMain / 无包级 init（撞名规避）；helper 一律 `ipw` 前缀。
// ★ 黄金串取自 `/usr/bin/python3 <原件>` 的现读输出（py 3.9.6）；非确定 token（pid/ts）逐处处理：
//
//	pid 用 `os.Getpid()`（同进程 ⇒ 恒存活 ⇒ 状态行为确定），ts 走正则（只证「是带小数点的浮点」）。
package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func ipwRun(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var so, se bytes.Buffer
	rc := ipwGateMain(&so, &se, argv)
	return rc, so.String(), se.String()
}

// ipwFixture —— 仓外夹具：临时状态目录 ＋（可选）target 件。回 (stateDir, targetPath)。
func ipwFixture(t *testing.T, withTarget bool) (string, string) {
	t.Helper()
	base := t.TempDir()
	state := filepath.Join(base, "state")
	t.Setenv("ZERG_STATE_DIR", state)
	tgt := filepath.Join(base, "t.txt")
	if withTarget {
		if err := os.WriteFile(tgt, []byte("ORIG"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return state, tgt
}

// ── 黄金串（现读 py 3.9.6 输出 · 逐字）────────────────────────────────────

const ipwGoldenTopHelp = "usage: inplace-window.py [-h] [--json] [--self-test]\n" +
	"                         {arm,status,restore,sweep} ...\n\n" +
	"原地改写窗口公共件（GAP-20260927-112）\n\n" +
	"positional arguments:\n" +
	"  {arm,status,restore,sweep}\n\n" +
	"optional arguments:\n" +
	"  -h, --help            show this help message and exit\n" +
	"  --json\n" +
	"  --self-test\n"

const ipwGoldenArmHelp = "usage: inplace-window.py arm [-h] [--run-id RUN_ID] target\n\n" +
	"positional arguments:\n  target\n\n" +
	"optional arguments:\n  -h, --help       show this help message and exit\n  --run-id RUN_ID\n"

const ipwGoldenTopUsageLine = "usage: inplace-window.py [-h] [--json] [--self-test]\n" +
	"                         {arm,status,restore,sweep} ...\n"

// ── posixpath 同构 ───────────────────────────────────────────────────────

func TestIpwPyPathSemantics(t *testing.T) {
	if got := ipwPyJoin("a", "b"); got != "a/b" {
		t.Errorf("ipwPyJoin(a,b) = %q, want a/b", got)
	}
	if got := ipwPyJoin("repo", "/abs"); got != "/abs" {
		t.Errorf("ipwPyJoin(repo,/abs) = %q, want /abs（后段绝对丢前段）", got)
	}
	if got := ipwPyJoin("a", ""); got != "a/" {
		t.Errorf("ipwPyJoin(a,'') = %q, want a/", got)
	}
	if got := ipwPyJoin("a", "../b"); got != "a/../b" {
		t.Errorf("ipwPyJoin(a,../b) = %q, want a/../b（原样留 ..）", got)
	}
	if got := ipwPyNormpath("//a/b/../c"); got != "//a/c" {
		t.Errorf("ipwPyNormpath(//a/b/../c) = %q, want //a/c（保留恰两个前导斜杠）", got)
	}
	if got := ipwPyNormpath("/a/b/../c"); got != "/a/c" {
		t.Errorf("ipwPyNormpath(/a/b/../c) = %q, want /a/c", got)
	}
	if got := ipwPyDirname("/a/b/c.txt"); got != "/a/b" {
		t.Errorf("ipwPyDirname = %q, want /a/b", got)
	}
	if got := ipwPyDirname("c.txt"); got != "" {
		t.Errorf("ipwPyDirname(裸名) = %q, want ''", got)
	}
	if got := ipwPyBasename("/a/b/"); got != "" {
		t.Errorf("ipwPyBasename(/a/b/) = %q, want ''（posixpath 剥尾斜杠）", got)
	}
	// abspath 对绝对串是恒等（join 丢 cwd ⇒ 与 cwd 符号链接无关）
	if got := ipwPyAbspath("/a/../b"); got != "/b" {
		t.Errorf("ipwPyAbspath(/a/../b) = %q, want /b", got)
	}
}

// ── JSON 面（py json.dumps ensure_ascii=False · 默认分隔符 · 键插入序）─────

func TestIpwJSONFaces(t *testing.T) {
	if got := ipwFloatRepr(1.0); got != "1.0" {
		t.Errorf("ipwFloatRepr(1.0) = %q, want \"1.0\"（✗ \"1\"）", got)
	}
	if got := ipwFloatRepr(1791316736.5); got != "1791316736.5" {
		t.Errorf("ipwFloatRepr = %q", got)
	}
	obj := &ipwObj{F: []ipwKV{{"a", 1}, {"b", "x"}, {"c", true}}}
	if got := ipwEnc(obj); got != `{"a": 1, "b": "x", "c": true}` {
		t.Errorf("ipwEnc(obj) = %q, want 默认分隔符 `, ` / `: ` + 键插入序", got)
	}
	// 空集合 = `[]`（✗ null）
	if got := ipwEnc(ipwArr{}); got != "[]" {
		t.Errorf("ipwEnc(空 ipwArr) = %q, want []（空集不许落 null）", got)
	}
	// ✗ 不转义 HTML（Go encoding/json 默认会转成 \u003c）
	if got := ipwQuote("<a>&"); got != `"<a>&"` {
		t.Errorf("ipwQuote = %q, want 原样（✗ \\u003c）", got)
	}
	if got := ipwQuote("x\u0001y"); got != `"x\u0001y"` {
		t.Errorf("ipwQuote 控制符 = %q", got)
	}
}

// ── argparse 面（帮助 / 前缀缩写 / 错误正文逐字）──────────────────────────

func TestIpwHelpFaces(t *testing.T) {
	rc, so, se := ipwRun(t)
	if rc != 2 || so != ipwGoldenTopHelp || se != "" {
		t.Errorf("无参 = (rc=%d,out=%q,err=%q)：要 rc=2 ＋ 顶帮助逐字", rc, so, se)
	}
	rc, so, _ = ipwRun(t, "-h")
	if rc != 0 || so != ipwGoldenTopHelp {
		t.Errorf("-h = (rc=%d,out=%q)：要 rc=0 ＋ 顶帮助逐字", rc, so)
	}
	rc, so, _ = ipwRun(t, "--h")
	if rc != 0 || so != ipwGoldenTopHelp {
		t.Errorf("--h（唯一前缀）应匹 --help ⇒ rc=%d", rc)
	}
	rc, so, _ = ipwRun(t, "arm", "-h")
	if rc != 0 || so != ipwGoldenArmHelp {
		t.Errorf("arm -h = (rc=%d,out=%q)：要子帮助逐字", rc, so)
	}
	rc, so, se = ipwRun(t, "--json")
	if rc != 2 || so != ipwGoldenTopHelp {
		t.Errorf("--json 无子命令 = (rc=%d)：要 print_help ＋ rc=2", rc)
	}
	_ = se
}

func TestIpwParseErrorFaces(t *testing.T) {
	rc, so, se := ipwRun(t, "--nope")
	if rc != 2 || so != "" || se != ipwGoldenTopUsageLine+"inplace-window.py: error: unrecognized arguments: --nope\n" {
		t.Errorf("--nope = (rc=%d,err=%q)", rc, se)
	}
	rc, _, se = ipwRun(t, "frobnicate")
	if rc != 2 || se != ipwGoldenTopUsageLine+"inplace-window.py: error: argument cmd: invalid choice: 'frobnicate' (choose from 'arm', 'status', 'restore', 'sweep')\n" {
		t.Errorf("非法子命令 err = %q", se)
	}
	rc, _, se = ipwRun(t, "arm")
	if rc != 2 || se != "usage: inplace-window.py arm [-h] [--run-id RUN_ID] target\n"+
		"inplace-window.py arm: error: the following arguments are required: target\n" {
		t.Errorf("arm 缺 target err = %q", se)
	}
	rc, _, se = ipwRun(t, "status", "extra")
	if rc != 2 || se != ipwGoldenTopUsageLine+"inplace-window.py: error: unrecognized arguments: extra\n" {
		t.Errorf("status extra err = %q（顶层 prog 报未识别）", se)
	}
	rc, _, se = ipwRun(t, "arm", "t", "--json")
	if rc != 2 || se != ipwGoldenTopUsageLine+"inplace-window.py: error: unrecognized arguments: --json\n" {
		t.Errorf("arm t --json err = %q（顶层旗标须在子命令前）", se)
	}
	rc, _, se = ipwRun(t, "--json=1")
	if rc != 2 || se != ipwGoldenTopUsageLine+"inplace-window.py: error: argument --json: ignored explicit argument '1'\n" {
		t.Errorf("--json=1 err = %q", se)
	}
	rc, _, se = ipwRun(t, "sweep", "--heal=1")
	if rc != 2 || se != "usage: inplace-window.py sweep [-h] [--heal]\n"+
		"inplace-window.py sweep: error: argument --heal: ignored explicit argument '1'\n" {
		t.Errorf("sweep --heal=1 err = %q", se)
	}
	// ★「缺必填」先于「未识别」：py 报 required（✗ 不是 unrecognized arguments: --json）
	rc, _, se = ipwRun(t, "arm", "--json")
	if rc != 2 || se != "usage: inplace-window.py arm [-h] [--run-id RUN_ID] target\n"+
		"inplace-window.py arm: error: the following arguments are required: target\n" {
		t.Errorf("arm --json err = %q（缺必填须先于未识别）", se)
	}
	rc, _, se = ipwRun(t, "restore", "--json")
	if rc != 2 || se != "usage: inplace-window.py restore [-h] run_id\n"+
		"inplace-window.py restore: error: the following arguments are required: run_id\n" {
		t.Errorf("restore --json err = %q（缺必填须先于未识别）", se)
	}
	rc, _, se = ipwRun(t, "arm", "t", "--run-id")
	if rc != 2 || se != "usage: inplace-window.py arm [-h] [--run-id RUN_ID] target\n"+
		"inplace-window.py arm: error: argument --run-id: expected one argument\n" {
		t.Errorf("arm --run-id 缺值 err = %q", se)
	}
}

// ── 自检（自建自回收）────────────────────────────────────────────────────

func TestIpwSelfTestPass(t *testing.T) {
	_, _ = ipwFixture(t, false)
	rc, so, se := ipwRun(t, "--self-test")
	if rc != 0 || so != "自检 PASS\n" || se != "" {
		t.Errorf("--self-test = (rc=%d,out=%q,err=%q), want (0,\"自检 PASS\\n\",\"\")", rc, so, se)
	}
	// `--self-test` 恒优先于子命令（现读 py 行为）
	rc, so, _ = ipwRun(t, "--self-test", "arm", "x")
	if rc != 0 || so != "自检 PASS\n" {
		t.Errorf("--self-test arm x = (rc=%d,out=%q)", rc, so)
	}
	rc, so, _ = ipwRun(t, "--s")
	if rc != 0 || so != "自检 PASS\n" {
		t.Errorf("--s（唯一前缀）应匹 --self-test ⇒ rc=%d", rc)
	}
}

// ── arm/status/restore 主逻辑（逐字 ＋ pid 用本进程）──────────────────────

func TestIpwArmRestoreStatusFaces(t *testing.T) {
	state, tgt := ipwFixture(t, true)
	rid := "DEADBEEF00000001"
	rc, so, se := ipwRun(t, "arm", tgt, "--run-id", rid)
	if rc != 0 || se != "" {
		t.Fatalf("arm rc=%d err=%q", rc, se)
	}
	if so != fmt.Sprintf("已 arm run_id=%s target=%s\n", rid, tgt) {
		t.Errorf("arm out = %q", so)
	}
	// 台账行：键序 ＋ 默认分隔符 ＋ ts 带小数点 ＋ 64 位 sha（pid = 本进程 ⇒ 恒存活）
	led, err := os.ReadFile(filepath.Join(state, "inplace-window", "ledger.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	pre := filepath.Join(state, "inplace-window", "pre", rid+".t.txt.pre")
	re := regexp.MustCompile(`^\{"event": "arm", "run_id": "` + regexp.QuoteMeta(rid) +
		`", "target": "` + regexp.QuoteMeta(tgt) + `", "preimage": "` + regexp.QuoteMeta(pre) +
		`", "target_sha": "[0-9a-f]{64}", "pid": ` + fmt.Sprint(os.Getpid()) +
		`, "ts": [0-9]+\.[0-9]+\}\n$`)
	if !re.MatchString(string(led)) {
		t.Errorf("台账行格式不逐字：%q", string(led))
	}
	// status：本进程存活 ⇒ 「存活」；pid = 本进程
	rc, so, _ = ipwRun(t, "status")
	want := fmt.Sprintf("inplace-window 台账：1 条窗口\n  %s target=%s pid=%d ⇒ 存活\n", rid, tgt, os.Getpid())
	if rc != 0 || so != want {
		t.Errorf("status out = %q\n want %q", so, want)
	}
	rc, so, _ = ipwRun(t, "--json", "status")
	want = fmt.Sprintf("{\"windows\": [{\"run_id\": \"%s\", \"target\": \"%s\", \"pid\": %d, \"alive\": true, \"restored\": false}]}\n", rid, tgt, os.Getpid())
	if rc != 0 || so != want {
		t.Errorf("--json status out = %q\n want %q", so, want)
	}
	// restore ⇒ 「已复原」＋ target 字节复原
	rc, so, se = ipwRun(t, "restore", rid)
	if rc != 0 || so != "已复原 1 个窗口\n" || se != "" {
		t.Errorf("restore = (rc=%d,out=%q,err=%q)", rc, so, se)
	}
	if b, _ := os.ReadFile(tgt); string(b) != "ORIG" {
		t.Errorf("restore 后 target = %q, want ORIG", string(b))
	}
	rc, so, _ = ipwRun(t, "--json", "restore", rid)
	if rc != 0 || so != "{\"restored\": 1}\n" {
		t.Errorf("--json restore out = %q", so)
	}
	rc, so, _ = ipwRun(t, "status")
	if rc != 0 || !strings.Contains(so, "⇒ 已复原") {
		t.Errorf("复原后 status = %q", so)
	}
}

func TestIpwMissingTargetAndUnknownRun(t *testing.T) {
	_, tgt := ipwFixture(t, false)
	if _, err := os.Stat(tgt); !os.IsNotExist(err) {
		t.Fatalf("夹具 target 不该在盘")
	}
	rc, so, se := ipwRun(t, "arm", tgt)
	want := fmt.Sprintf("inplace-window：target 不在盘，无可前像：%s ⇒ rc=2\n", tgt)
	if rc != 2 || so != "" || se != want {
		t.Errorf("arm 缺目标 = (rc=%d,out=%q,err=%q)\n want err=%q", rc, so, se, want)
	}
	rc, _, se = ipwRun(t, "restore", "NOSUCH")
	if rc != 2 || se != "inplace-window：台账里没有该 run 的窗口：NOSUCH ⇒ rc=2\n" {
		t.Errorf("restore 未知 run = (rc=%d,err=%q)", rc, se)
	}
	rc, _, se = ipwRun(t, "--json", "restore", "NOSUCH")
	if rc != 2 || se != "inplace-window：台账里没有该 run 的窗口：NOSUCH ⇒ rc=2\n" {
		t.Errorf("--json restore 未知 run = (rc=%d,err=%q)（rc 与正文不随 --json 变）", rc, se)
	}
	// 状态目录解析不来（根）⇒ rc=2
	t.Setenv("ZERG_STATE_DIR", "/")
	rc, _, se = ipwRun(t, "--json", "status")
	if rc != 2 || se != "inplace-window：状态目录解析不来（ZERG_STATE_DIR 为空/根）⇒ 拒跑（rc=2）\n" {
		t.Errorf("根状态目录 = (rc=%d,err=%q)", rc, se)
	}
}

func TestIpwSweepStaleAndHeal(t *testing.T) {
	state, _ := ipwFixture(t, false)
	root := filepath.Join(state, "inplace-window")
	if err := os.MkdirAll(filepath.Join(root, "pre"), 0o755); err != nil {
		t.Fatal(err)
	}
	rec := `{"event": "arm", "run_id": "STALE01", "target": "/x.txt", "preimage": "` +
		filepath.Join(root, "pre", "STALE01.x.txt.pre") + `", "target_sha": "aa", "pid": 999999, "ts": 1.0}` + "\n"
	if err := os.WriteFile(filepath.Join(root, "ledger.jsonl"), []byte(rec), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "pre", "STALE01.x.txt.pre"), []byte("AA"), 0o644); err != nil {
		t.Fatal(err)
	}
	// 先证「空 ≠ 取不到」两条牙：sweep 空仓（无台账件）也回 []，不是 null
	rc, so, _ := ipwRun(t, "--json", "sweep")
	if rc != 0 || so != "{\"stale\": [\"STALE01\"], \"healed\": []}\n" {
		t.Errorf("--json sweep out = %q", so)
	}
	rc, so, _ = ipwRun(t, "sweep")
	if rc != 0 || !strings.Contains(so, "疑似残留（持有者已死、未复原）= 1 条") ||
		!strings.Contains(so, "加 --heal 复原；SIGKILL 场景只能靠本列表 + 人工。") {
		t.Errorf("sweep out = %q", so)
	}
	// --heal：前像 sha 与登记不符 ⇒ 记「失败：…」但不改进程退码（现读 py 行为）
	rc, so, _ = ipwRun(t, "sweep", "--heal")
	if rc != 0 || !strings.Contains(so, "✓ 已复原 STALE01 ⇒ 失败：前像 sha 与登记不符：") {
		t.Errorf("sweep --heal out = %q", so)
	}
	// 空仓（无台账件）⇒ 空集 `[]`（✗ null）
	t.Setenv("ZERG_STATE_DIR", t.TempDir())
	rc, so, _ = ipwRun(t, "--json", "sweep")
	if rc != 0 || so != "{\"stale\": [], \"healed\": []}\n" {
		t.Errorf("空仓 --json sweep = %q（空集不许落 null）", so)
	}
}

func TestIpwLiveHolderRejectsRC3(t *testing.T) {
	state, tgt := ipwFixture(t, true)
	root := filepath.Join(state, "inplace-window")
	if err := os.MkdirAll(filepath.Join(root, "pre"), 0o755); err != nil {
		t.Fatal(err)
	}
	// 占用者 pid = 本进程（恒存活）⇒ 第二跑必拒（跨真进程面 → 此处用「活 pid」代表）
	rec := fmt.Sprintf(`{"event": "arm", "run_id": "LIVE01", "target": "%s", "preimage": "%s", "target_sha": "x", "pid": %d, "ts": 1.0}`+"\n",
		tgt, filepath.Join(root, "pre", "LIVE01.t.txt.pre"), os.Getpid())
	if err := os.WriteFile(filepath.Join(root, "ledger.jsonl"), []byte(rec), 0o644); err != nil {
		t.Fatal(err)
	}
	rc, so, se := ipwRun(t, "arm", tgt, "--run-id", "NEW02")
	if rc != 3 || so != "" || se != fmt.Sprintf("inplace-window：同 target 已有存活改写窗口 ⇒ 并发第二跑被拒（rc=3）：%s\n", tgt) {
		t.Errorf("活占用者 = (rc=%d,out=%q,err=%q), want rc=3", rc, so, se)
	}
}

// ── 形状① 库面（窗口写入 ⇒ 复原；verify_outside 读 sha）──────────────────

func TestIpwWindowFacade(t *testing.T) {
	_, tgt := ipwFixture(t, true)
	rid, done, e := ipwWindow(tgt, "WIN00001")
	if e != nil {
		t.Fatalf("ipwWindow err = %v", e)
	}
	if rid != "WIN00001" {
		t.Errorf("window rid = %q", rid)
	}
	if err := ipwWindowWriteBytes(tgt, []byte("MUT")); err != nil {
		t.Fatal(err)
	}
	if _, ok := ipwWindowVerifyOutside(tgt, ""); !ok {
		t.Errorf("verify_outside 无期望值应回 true")
	}
	done()
	if b, _ := os.ReadFile(tgt); string(b) != "ORIG" {
		t.Errorf("窗口退出后 target = %q, want ORIG", string(b))
	}
}

// TestIpwPyRepr —— argparse 错误正文里的 %r 形态（单引号包裹）
func TestIpwPyRepr(t *testing.T) {
	if got := ipwPyRepr("frobnicate"); got != "'frobnicate'" {
		t.Errorf("ipwPyRepr = %q", got)
	}
	if got := ipwPyRepr("a'b"); got != `'a\'b'` {
		t.Errorf("ipwPyRepr 含引号 = %q", got)
	}
}

// TestIpwParseSubAbbrev —— 子命令旗标唯一前缀（--hea ⇒ --heal；--run-i ⇒ --run-id）
func TestIpwParseSubAbbrev(t *testing.T) {
	a, h, e := ipwParseArgs([]string{"sweep", "--hea"})
	if e != nil || h != "" || !a.heal {
		t.Errorf("sweep --hea = (%+v,h=%q,e=%v), want heal=true", a, h, e)
	}
	a, h, e = ipwParseArgs([]string{"arm", "t", "--run-i", "Z"})
	if e != nil || h != "" || !a.haveRunID || a.runID != "Z" || a.target != "t" {
		t.Errorf("arm --run-i Z = (%+v,h=%q,e=%v)", a, h, e)
	}
	// 顶层唯一前缀：--j ⇒ --json
	a, _, e = ipwParseArgs([]string{"--j", "status"})
	if e != nil || !a.json || a.cmd != "status" {
		t.Errorf("--j status = (%+v,e=%v)", a, e)
	}
}

// ── ★ inproc 入口体例（`ipwGateMain`）────────────────────────────────────
//
// 接线批点名：本件原只有 `ipwMain` ⇒ 体例与 `gate_inproc_table.go` 的 `gateCheckGoEntry`
// 元素**对不上**、分发批接不上 ⇒ 本批补 `ipwGateMain`（转发 `ipwMain`，单实现）。本用例把
// 「入口体例」两面钉死。
//
// 哑化实证（仓外副本 · 见回执）：① 令入口按 py `sys.argv` 口径吃掉 `argv[0]` ⇒ 断言(②)红
// （首 token 被当程序名丢弃 ⇒ 退化成顶帮助 rc=2，带行号）；② 入口名不回 `*GateMain` ⇒
// 断言(①)在编译期即红（`undefined: ipwGateMain`）。
func TestIpwGateMainInprocEntry(t *testing.T) {
	// ① 入口签名逐字 = 分发表元素类型（编译期钉住：名字/形参/回值任一改 ⇒ 本行编不过）
	var entry func(stdout, stderr io.Writer, argv []string) int = ipwGateMain
	if entry == nil {
		t.Fatal("ipwGateMain 未挂入口体例")
	}

	// ② argv **不含程序名**：首 token 直接被当子命令解析。
	_, _ = ipwFixture(t, false)
	var so, se bytes.Buffer
	if rc := ipwGateMain(&so, &se, []string{"status"}); rc != 0 {
		t.Fatalf("ipwGateMain([\"status\"]) rc=%d err=%q —— inproc 契约：argv 不含程序名，"+
			"首 token 即真参数（status）；若入口吃掉 argv[0] 则退化为顶帮助 rc=2", rc, se.String())
	}
	// 反证同一入口：塞进程序名 ⇒ 它会被当**非法子命令**（rc=2）—— 证明入口真按「不含」口径读首 token。
	var so2, se2 bytes.Buffer
	if rc := ipwGateMain(&so2, &se2, []string{"inplace-window.py", "status"}); rc != 2 {
		t.Errorf("ipwGateMain([\"inplace-window.py\",\"status\"]) rc=%d —— 按 inproc 口径首 token 即参数"+
			"⇒ 该串非法子命令应 rc=2（✗ 不许悄悄跳过当程序名）", rc)
	}
}
