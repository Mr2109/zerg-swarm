// check_approve_closedset_test.go —— 配对测试件（成对负控 · ✗ 禁恒绿）。
//
// 覆盖面（每条都带牙）：
//  1. 判定口正控：真契约四格 == 批前登记值 ⇒ 0 条问题（且返回**非 nil 空切片**）。
//  2. 判定口负控 + **两条互斥断言**：①「条数被改」/ ②「条数不变、只把 `sig_alg` 塞进正文」
//     —— 各自命中自己那条文案，并各自断言**对方文案不出现**。
//  3. 「取不到 vs 空集合」两态**分开表意**（nil slice 不得兼表）：非法 JSON ⇒ 取不到（why 非空）；
//     `entries` 显式空数组 ⇒ 不是取不到（why 空、isList 真、0 条）；`entries` **缺键** ⇒ 第三种态。
//  4. 退码面：缺契约 ⇒ rc=2（不给结论）· `--json` ⇒ rc=2 ＋ 具名 stderr · 未知旗标 ⇒ rc=2 ＋ usage。
//  5. 帮助面 byte 级 golden：`cacsHelp` 的 sha256 == py 原件 `-h` 输出的 sha256（1108 字节）。
//  6. A 类止血回归牙：`cacsSelfTest` 跑前/跑后同前缀临时目录计数 **delta=0**（py 侧从不回收）。
//  7. 外部真源/工具链钉死：契约相对路径 · 同步门相对路径 · `python3` 一律 `/usr/bin/python3`。
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// cacsTestPyHelpSHA256 = `/usr/bin/python3 <件> -h` 的 stdout 逐字节 sha256（1108 字节）。
const cacsTestPyHelpSHA256 = "c181e31c801bc7b874212b86e0ae3f9590e7f70fc58e5a55ac9e29c4ed176a0c"

func cacsTestRegistryPath(t *testing.T) string {
	t.Helper()
	root := caciFindRepoRoot()
	p := caciPyJoin(root, cacsRegistryRel)
	if !caciExists(p) {
		t.Skipf("契约不在（%s）⇒ 本格跳过（吃真仓物料）", p)
	}
	return p
}

func cacsTestRealRegistry(t *testing.T) (cacsObj, bool, int, string, string) {
	t.Helper()
	p := cacsTestRegistryPath(t)
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("契约读不了：%v", err)
	}
	o, isList, n, why := cacsReadRegistry(raw)
	return o, isList, n, why, string(raw)
}

// ① 正控：真契约四格 == 批前登记值 ⇒ 不判红；且「空集合」面必须是**非 nil** 空切片。
func TestCacsJudgeClosedset_PositiveControl(t *testing.T) {
	o, isList, n, why, txt := cacsTestRealRegistry(t)
	if why != "" {
		t.Fatalf("真契约应解析成功，却 why=%q", why)
	}
	p := cacsJudgeClosedset(o.keys, isList, n, txt)
	if p == nil {
		t.Fatalf("正控返回 nil —— nil 不得兼表「取不到」与「空集合」（该返 []string{}）")
	}
	if len(p) != 0 {
		t.Fatalf("真契约被判红 %d 条：%v", len(p), p)
	}
	if n != cacsEntriesN {
		t.Fatalf("真契约 entries=%d ≠ 批前 %d", n, cacsEntriesN)
	}
}

// ② 成对负控（带牙 · 互斥）：两条断言各自命中自己那条文案，并各自断言**对方文案不出现**。
func TestCacsJudgeClosedset_SigAlgAndCountMutex(t *testing.T) {
	// ②a 条数被改（16 条）⇒ 命中「条数」那条
	keys := []string{"schema", "note", "authority", "entries", "change_flow"}
	txtA := `{"entries":[` + strings.Repeat(`{"id":"x"},`, cacsEntriesN) + `{"id":"y","sig_alg":"ed25519"}], "approve":1}`
	pA := cacsJudgeClosedset(keys, true, cacsEntriesN+1, txtA)
	if !cacsAnyContains(pA, "sig_alg") {
		t.Fatalf("②a 负控失败：含 sig_alg 却判绿（或没点名）：%v", pA)
	}
	if !cacsAnyContains(pA, "条 ≠ 批前") {
		t.Fatalf("②a 应同时点名条数被改：%v", pA)
	}

	// ②b 成对负控：**条数不变**、只把 `sig_alg` 悄悄塞进正文 ⇒ 仍判红，且**只有 sig_alg 那一条**
	body := make([]string, 0, cacsEntriesN)
	for i := 0; i < cacsEntriesN; i++ {
		body = append(body, `{"id":"e`+string(rune('a'+i))+`","note":"x sig_alg"}`)
	}
	txtB := `{"entries":[` + strings.Join(body, ",") + `], "approve":1}`
	pB := cacsJudgeClosedset(keys, true, cacsEntriesN, txtB)
	if !cacsAnyContains(pB, "sig_alg") {
		t.Fatalf("②b 负控失败：条数不变就判绿（= `sig_alg` 这格是虚的）：%v", pB)
	}
	if len(pB) != 1 {
		t.Fatalf("②b 期望恰好 1 条（只有 sig_alg 那格有牙），实得 %d 条：%v", len(pB), pB)
	}
	if cacsAnyContains(pB, "条 ≠ 批前") {
		t.Fatalf("②b **互斥断言破**：条数没变却出现「条数」文案：%v", pB)
	}

	// ②c 反向互斥：条数被改（14 条）而 sig_alg 0 ⇒ 判红且**不出现 sig_alg 文案**
	txtC := `{"entries":[` + strings.Repeat(`{"id":"x"},`, cacsEntriesN-1) + `{"id":"z"}], "approve":1}`
	pC := cacsJudgeClosedset(keys, true, cacsEntriesN-1, txtC)
	if !cacsAnyContains(pC, "条 ≠ 批前") {
		t.Fatalf("②c 负控失败：条数变了还判绿：%v", pC)
	}
	if cacsAnyContains(pC, "sig_alg") {
		t.Fatalf("②c **互斥断言破**：sig_alg 计数没变却出现 sig_alg 文案：%v", pC)
	}

	// ②d 顶层键序/集合被改 ⇒ 单独点名
	if p := cacsJudgeClosedset([]string{"schema", "note", "authority", "change_flow"}, true, cacsEntriesN, txtC); !cacsAnyContains(p, "顶层键") {
		t.Fatalf("②d 负控失败：顶层键少了还判绿：%v", p)
	}
	if p := cacsJudgeClosedset([]string{"note", "schema", "authority", "entries", "change_flow"}, true, cacsEntriesN, txtC); !cacsAnyContains(p, "顶层键") {
		t.Fatalf("②e 负控失败：**顺序**被换还判绿（序也是判据）：%v", p)
	}
}

// ③ 「取不到 vs 空集合 vs 缺键」三态分开表意（nil slice 不得兼表）。
func TestCacsReadRegistry_MissingVsEmptyVsUnreadable(t *testing.T) {
	// (i) 真取不到：不是合法 JSON
	if _, _, _, why := cacsReadRegistry([]byte("{ not json")); why == "" {
		t.Fatalf("(i) 非法 JSON 应判「取不到」（why 非空），实得 why=\"\"")
	}
	// (ii) 真取不到：顶层不是对象
	if _, _, _, why := cacsReadRegistry([]byte("[1,2]")); why == "" {
		t.Fatalf("(ii) 顶层数组应判「取不到」（why 非空）")
	}
	// (iii) **空集合 ≠ 取不到**：entries 显式空数组 ⇒ why=="" 且 isList 真且 0 条
	o3, isList3, n3, why3 := cacsReadRegistry([]byte(`{"schema":1,"note":2,"authority":3,"entries":[],"change_flow":4}`))
	if why3 != "" {
		t.Fatalf("(iii) 空 entries 被当成「取不到」了（why=%q）—— 空 ≠ 缺", why3)
	}
	if !isList3 || n3 != 0 {
		t.Fatalf("(iii) 空 entries 读数错：isList=%v n=%d", isList3, n3)
	}
	if p := cacsJudgeClosedset(o3.keys, isList3, n3, `{"entries":[]}`); len(p) == 0 {
		t.Fatalf("(iii) 空 entries 应判红（0 ≠ 15）")
	}
	// (iv) **缺键** ⇒ 第三态：why==""（契约本身合法）但 isList 假 ⇒ 判红（文案「不是数组」）
	o4, isList4, n4, why4 := cacsReadRegistry([]byte(`{"schema":1,"note":2,"authority":3,"change_flow":4}`))
	if why4 != "" {
		t.Fatalf("(iv) 缺 entries 键被当成「取不到」了（why=%q）", why4)
	}
	if isList4 || n4 != 0 {
		t.Fatalf("(iv) 缺键读数错：isList=%v n=%d", isList4, n4)
	}
	p4 := cacsJudgeClosedset(o4.keys, isList4, n4, `{}`)
	if !cacsAnyContains(p4, "不是数组") {
		t.Fatalf("(iv) 缺键应点名「不是数组」：%v", p4)
	}
}

// ④ 退码面：缺契约 ⇒ rc=2；`--json` ⇒ rc=2 ＋ 具名 stderr；未知旗标 ⇒ rc=2 ＋ usage。
func TestCacsGateMain_RcFaces(t *testing.T) {
	tmp := t.TempDir()
	missing := caciPyJoin(tmp, "nope", "registry.json")
	var out, errb bytes.Buffer
	rc := cacsScan(&out, tmp, tmp, tmp, missing, true, 5)
	if rc != 2 {
		t.Fatalf("缺契约应 rc=2（不给结论），实得 %d", rc)
	}
	if !strings.Contains(out.String(), "未取数") {
		t.Fatalf("缺契约应打「未取数」：%q", out.String())
	}

	out.Reset()
	errb.Reset()
	rc = checkApproveClosedsetGateMain(&out, &errb, []string{"--json"})
	if rc != 2 {
		t.Fatalf("--json 应 rc=2，实得 %d", rc)
	}
	if out.Len() != 0 {
		t.Fatalf("--json 档 stdout 应为 0 字节，实得 %d 字节", out.Len())
	}
	if !strings.Contains(errb.String(), "本门无机器读面") || !strings.Contains(errb.String(), "字段表：无（本门只出人读面）") {
		t.Fatalf("--json 档 stderr 文案不全：%q", errb.String())
	}

	out.Reset()
	errb.Reset()
	rc = checkApproveClosedsetGateMain(&out, &errb, []string{"--bogus"})
	if rc != 2 || !strings.Contains(errb.String(), "unrecognized arguments: --bogus") {
		t.Fatalf("未知旗标应 rc=2 ＋ usage 错行，实得 rc=%d err=%q", rc, errb.String())
	}
	if !strings.HasPrefix(errb.String(), "usage: ") {
		t.Fatalf("未知旗标 stderr 应先打 usage：%q", errb.String())
	}

	out.Reset()
	errb.Reset()
	rc = checkApproveClosedsetGateMain(&out, &errb, []string{"--timeout", "abc"})
	if rc != 2 || !strings.Contains(errb.String(), "argument --timeout: invalid int value: 'abc'") {
		t.Fatalf("--timeout 非整数应 rc=2 ＋ 具名错，实得 rc=%d err=%q", rc, errb.String())
	}
}

// ⑤ 帮助面 golden（byte 级 · 与 py 原件 -h 输出逐字节同）。
func TestCacsHelp_GoldenSHA256(t *testing.T) {
	sum := sha256.Sum256([]byte(cacsHelp))
	got := hex.EncodeToString(sum[:])
	if got != cacsTestPyHelpSHA256 {
		t.Fatalf("帮助面漂移：got sha256=%s（%d 字节）· want %s（1108 字节）", got, len(cacsHelp), cacsTestPyHelpSHA256)
	}
	var out, errb bytes.Buffer
	if rc := checkApproveClosedsetGateMain(&out, &errb, []string{"-h"}); rc != 0 {
		t.Fatalf("-h 应 rc=0，实得 %d", rc)
	}
	if out.String() != cacsHelp {
		t.Fatalf("-h stdout 与 cacsHelp 不同")
	}
}

// ⑥ A 类止血回归牙：`cacsSelfTest` 跑前/跑后同前缀临时目录计数 delta=0（py 侧从不回收）。
//
//	窄牙负控（仓外副本删 `defer os.RemoveAll(tmp)`）应使本用例 FAIL —— 参见收尾实测。
func TestCacsSelfTest_ReclaimsFixtureDir(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir()) // GAP-20261003-16：私有计数面（只数本用例自建 tmp 目录）
	for _, a := range os.Args[1:] {
		if a == "--no-self-test" || a == "--static-only" {
			t.Skip("自检子进程：跳过（防递归）")
		}
	}
	cacsUseProbe(t) // ★ 测试二进制不是 CLI 入口 ⇒ 把「自身可执行件」换仓外探针壳（照 judge_lock_test.go 口径）
	root := caciFindRepoRoot()
	if !caciExists(caciPyJoin(root, cacsRegistryRel)) {
		t.Skipf("契约不在（%s）⇒ 本格跳过（吃真仓物料）", caciPyJoin(root, cacsRegistryRel))
	}
	before := cacsCountTempPrefix("closedset-")
	var sink bytes.Buffer
	_ = cacsSelfTest(&sink, root, "")
	after := cacsCountTempPrefix("closedset-")
	if after != before {
		t.Fatalf("自检夹具未回收：跑前 %d 个 · 跑后 %d 个（delta=%d，应 0）", before, after, after-before)
	}
	if !strings.Contains(sink.String(), "自检：") {
		t.Fatalf("自检应打汇总行，实得：%q", sink.String())
	}
	// ★ 回归牙（真命令行面）：⑬/⑭ 走**真子进程**；缺 inproc 路由前缀 ⇒ 两格恒 ✗（py 侧走 python3 无此层）。
	if !strings.Contains(sink.String(), "✓ ⑬") || !strings.Contains(sink.String(), "✓ ⑭") {
		t.Fatalf("真命令行两面未过（自检子进程未带 inproc 路由？）：\n%s", sink.String())
	}
}

func cacsCountTempPrefix(prefix string) int {
	ents, err := os.ReadDir(os.TempDir())
	if err != nil {
		return -1
	}
	n := 0
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), prefix) {
			n++
		}
	}
	return n
}

// ⑦ 外部真源 / 工具链钉死（GAP-20261001-61：python3 一律 /usr/bin/python3）。
func TestCacsPinnedTruthAndToolchain(t *testing.T) {
	if cacsRegistryRel != "core/internal/contract/registry.json" {
		t.Fatalf("契约相对路径漂移：%q", cacsRegistryRel)
	}
	if cacsSyncGateRel != "scripts/gates/check-publish-face-sync.py" {
		t.Fatalf("同步门相对路径漂移：%q", cacsSyncGateRel)
	}
	if cacsPyExe != "/usr/bin/python3" {
		t.Fatalf("python3 未钉 /usr/bin/python3（GAP-20261001-61）：%q", cacsPyExe)
	}
	if got := filepath.ToSlash(cacsDevdocsTail); got != "项目文档/v2.5.11" {
		t.Fatalf("开发文档面尾段漂移：%q", got)
	}
	if len(cacsJSONFields) != 0 {
		t.Fatalf("本门无机器读面 ⇒ 字段表必须为空（唯真源 cacsJSONFields）")
	}
	if !cacsStrSliceEq(cacsReservedFlags, []string{"--strength", "--token", "--rekey"}) {
		t.Fatalf("预留旗标名表/序漂移：%v", cacsReservedFlags)
	}
}

// ⑧ 登记表判定口：缺句/状态词被改 ⇒ 判红（与自检⑤⑥⑦同源，这里独立复验）。
func TestCacsJudgeLedger(t *testing.T) {
	good := "## 单向铁律\n" + cacsI4Verbatim + "\n状态：**未落地** · 预留旗标名**名字还没被占**\n"
	if p := cacsJudgeLedger(good); len(p) != 0 {
		t.Fatalf("正控失败：齐了还判红：%v", p)
	}
	if p := cacsJudgeLedger(strings.ReplaceAll(good, "未落地", "已落地")); len(p) == 0 {
		t.Fatalf("负控失败：把「未落地」写成「已落地」还判绿")
	}
	if p := cacsJudgeLedger(strings.ReplaceAll(good, cacsI4Verbatim, "")); len(p) == 0 {
		t.Fatalf("负控失败：抹掉 I4 逐字句还判绿")
	}
}

// ⑨ 旗标「没被占」判定口（成对：rc=2 且 stdout 0 字节 ／ 任一不合 ⇒ 红）。
func TestCacsJudgeFlagNotTaken(t *testing.T) {
	if p := cacsJudgeFlagNotTaken("--strength", 2, false, 0); len(p) != 0 {
		t.Fatalf("正控失败：rc=2 且 stdout 0 字节还判红：%v", p)
	}
	if p := cacsJudgeFlagNotTaken("--strength", 0, false, 0); len(p) == 0 {
		t.Fatalf("负控失败：rc=0（名字被占）还判绿")
	}
	if p := cacsJudgeFlagNotTaken("--rekey", 2, false, 5); len(p) == 0 {
		t.Fatalf("负控失败：stdout 非空还判绿")
	}
}

// ⑩ 全绿档：真仓 + 真文档面 + --static-only ⇒ rc=0 且 stdout 末行是「全绿」。
func TestCacsScan_StaticOnlyGreen(t *testing.T) {
	root := caciFindRepoRoot()
	devdocs := cacsDevdocsRootOf(root, "", false)
	if !caciExists(caciPyJoin(devdocs, cacsLedgerFile)) {
		t.Skipf("登记表不在（%s）⇒ 本格跳过（吃仓外文档面）", devdocs)
	}
	var out bytes.Buffer
	rc := cacsScan(&out, root, caciPyJoin(root, ".zerg", "state"), devdocs, "", true, 10)
	if rc != 0 {
		t.Fatalf("真仓静态档应 rc=0，实得 %d：\n%s", rc, out.String())
	}
	if !strings.Contains(out.String(), "⇒ 全绿（rc=0）") {
		t.Fatalf("全绿档文案缺失：\n%s", out.String())
	}
}

// ── 探针壳（测试二进制不是 CLI 入口 ⇒ 把 argv 转回 TestCacsHelperProcess 再直调本门入口）──

func cacsShQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func cacsChildArgs() []string {
	for i, a := range os.Args {
		if a == "--" {
			return os.Args[i+1:]
		}
	}
	return nil
}

// TestCacsHelperProcess —— 探针壳的落点。仅当 argv 里带 `--` 分界（即被探针壳调起）时才当 CLI 入口。
func TestCacsHelperProcess(t *testing.T) {
	args := cacsChildArgs()
	if args == nil {
		return
	}
	// 探针壳收到的 argv 必须带 inproc 路由前缀（`gate check <键>`）—— 缺前缀 = 主件的重跑面破形
	// （`bin/zerg` 会把本门旗标当未知命令 ⇒ 子进程退 2）⇒ 这里**不**兜底，让牙当场红。
	if !(len(args) >= 3 && args[0] == "gate" && args[1] == "check" && args[2] == "check-approve-closedset") {
		io.WriteString(os.Stderr, "探针壳：argv 缺 inproc 路由前缀（gate check check-approve-closedset）⇒ 重跑面破形\n")
		os.Exit(2)
	}
	os.Exit(checkApproveClosedsetGateMain(os.Stdout, os.Stderr, args[3:]))
}

// cacsUseProbe —— 把「自身可执行件」换成仓外探针壳（主件默认档一字未改）。
func cacsUseProbe(t *testing.T) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	script := filepath.Join(t.TempDir(), "cacs-probe.sh")
	body := "#!/bin/sh\nexec " + cacsShQuote(exe) + " '-test.run=^TestCacsHelperProcess$' -- \"$@\"\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatalf("写探针壳: %v", err)
	}
	old := cacsSelfExe
	cacsSelfExe = func() (string, error) { return script, nil }
	t.Cleanup(func() { cacsSelfExe = old })
}

var _ = io.Discard
