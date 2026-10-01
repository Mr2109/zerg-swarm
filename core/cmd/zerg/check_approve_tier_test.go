// check_approve_tier_test.go —— 配对测试件（成对负控 · ✗ 禁恒绿）。
//
// 覆盖面（每条都带牙）：
//  1. 帮助面 **byte 级 golden**：`ccatHelp` 的 sha256 == py 原件 `-h` 的 sha256（389 字节）；
//     且（本机有 py 时）现跑 `/usr/bin/python3 <py 原件> -h` **逐字节**复核。
//  2. Q-176：给了 `--json` ⇒ rc=2 ＋ 三行 stderr **逐字**；stdout 必空；没给 ⇒ 不拦。
//  3. argparse 等价面：坏旗标 / `--root` 缺值 / 位置参数 / `--self-test=x` / `-hx` / `--` 分隔符 / `-h`。
//  4. 判定口正控 + 负控：有序超集（换序红 / 删项红 / **新增项允许**）；返回必须是**非 nil** 空切片。
//  5. 「丢已知列 / 丢已知行」与「有序超集」**互补共存**（两条互斥断言各自还要断言对方文案不出现）。
//  6. 行宽面：数据行少一格 ⇒ 红；一行都没有 ⇒ 红（点名「表头也缺」）。
//  7. `parse_show_labels` 的贪心复刻：`工具\t:` 取得出、全角 `：` 取不出、**前导空白**取不出。
//  8. 字段面九键：逐字等 ⇒ 0 条；换序 ⇒ 只命中「≠ 九键」；塞 `strength` ⇒ 命中「被塞进必填集」。
//  9. 「取不到 vs 空集合」两态**分开表意**：解不出 ⇒ (nil,false)；解得动但零引号串 ⇒ **非 nil** 空切片。
//  10. **运行档 py↔go 对拍**：同一仓外夹具 + 同一桩 `bin/zerg`，`/usr/bin/python3 <py 原件> …` 与
//     Go 入口的 rc/stdout/stderr **逐字节**同（`--self-test` 档 与 `--no-self-test` 全跑档各一）。
//  11. 子进程面：`ZERG_STATE_DIR` 真的注进了子进程；`CCAT_STUB_*` 旋钮真被读到（否则负控是假的）。
//  12. A 类止血回归牙：自检跑前/跑后同前缀临时目录计数 **delta=0**（py 侧 finally 回收 ⇒ Go defer 必须对齐）。
//  13. 外部真源/工具链钉死：档位真源相对路径 = `core/cmd/zerg/family_approve.go`；解释器一律 `/usr/bin/python3`。
//  14. 退码面：本件用到的裸数字退码 {0,1,2} ⊆ 真源退码表 `core/cmd/zerg/exitcodes.go` 已登记集。
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// ccatTestPyExe —— 钉死的解释器（GAP-20261001-61：✗ 不用 PATH 上的 python3）。
const ccatTestPyExe = "/usr/bin/python3"

// ccatTestPyHelpSHA256 / ccatTestPyHelpBytes —— 现读来源：
// `/usr/bin/python3 scripts/gates/check-approve-tier.py -h`（现读解释器版本 = Python 3.9.6 ⇒
// argparse 表头是 `optional arguments:`；帮助列对齐在 18 列）⇒ 389 字节 ·
// sha256 485750777bd97434aa33405077589f0a48a36054bdaa55338cb8a79b92af5b2a。
const ccatTestPyHelpSHA256 = "485750777bd97434aa33405077589f0a48a36054bdaa55338cb8a79b92af5b2a"
const ccatTestPyHelpBytes = 389

// ccatTestStub —— 仓外夹具的桩 `bin/zerg`（两侧**共用同一份** · 逐字节同）。
// 旋钮（注入面 · 两侧都从 env 读到同一份）：CCAT_STUB_SWAP / DROP_DANG / NARROW / DROP_TTL /
// TOKEN / LS_RC / UNKNOWN_RC / LEAK。
const ccatTestStub = `#!/bin/sh
a="$1"; b="$2"; c="$3"; d="$4"
if [ "$a" = "approve" ] && [ "$b" = "ls" ]; then
  if [ "$c" = "--json" ]; then
    case "$d" in
      nosuchfield-xyz)
        echo 'zerg: 未知字段 "nosuchfield-xyz"' >&2
        echo "合法字段: tool,approver,approved_at,scope,note,sig_alg,key_id,state,path" >&2
        exit "${CCAT_STUB_UNKNOWN_RC:-2}" ;;
      strength)
        echo '{"schema":"zerg/v1","items":[],"meta":{"count":0}}'
        exit 0 ;;
    esac
  fi
  if [ "${CCAT_STUB_SWAP}" = "1" ]; then
    printf 'approver\ttool\tapproved_at\tscope\tstate\tpath\t档\n'
  elif [ "${CCAT_STUB_DROP_DANG}" = "1" ]; then
    printf 'tool\tapprover\tapproved_at\tscope\tstate\tpath\n'
  else
    printf 'tool\tapprover\tapproved_at\tscope\tstate\tpath\t档\n'
  fi
  if [ "${CCAT_STUB_NARROW}" = "1" ]; then
    printf 'x\ty\tz\n'
  else
    printf '合成件\tMr2109\t2026-09-22T10:00:00+08:00\t*\t无签名（不算批准）\t%s\tstrength=passphrase\n' "${ZERG_STATE_DIR}/approvals/合成件.json"
  fi
  exit "${CCAT_STUB_LS_RC:-0}"
fi
if [ "$a" = "approve" ] && [ "$b" = "show" ]; then
  printf '工具     : %s\n' "$c"
  printf '批准者   : Mr2109\n'
  printf '签的时间 : 2026-09-22T10:00:00+08:00\n'
  printf '范围     : *\n'
  printf '理由     : 夹具\n'
  printf '签名     : alg=ed25519 key_id=0000000000000000\n'
  printf '档       : strength=passphrase\n'
  printf '钥匙身份 : key_id=0000000000000000\n'
  if [ "${CCAT_STUB_DROP_TTL}" != "1" ]; then printf '有效期   : 不过期\n'; fi
  printf '判决     : 无签名（不算批准）\n'
  printf '落点     : %s/approvals/%s.json\n' "${ZERG_STATE_DIR}" "$c"
  if [ "${CCAT_STUB_TOKEN}" = "1" ]; then printf '备注     : token 触摸\n'; fi
  exit 0
fi
if [ "$a" = "approve" ] && [ "$b" = "keygen" ]; then
  if [ "${CCAT_STUB_LEAK}" = "1" ]; then
    mkdir -p "${ZERG_STATE_DIR}/approvals"
    : > "${ZERG_STATE_DIR}/approvals/漏件.json"
  fi
  echo 'zerg: 未知旗标 "--strength"' >&2
  echo "See 'zerg --help'。" >&2
  exit 2
fi
exit 0
`

// ccatTestFamilyApprove —— 夹具里那一枚**档位真源**的最小同形件（九键逐字 = `family_approve.go:66`）。
const ccatTestFamilyApprove = `package main

// 夹具替身：只保判据件 T3 要读的那一枚变量（形状与在册同）。
var approveFields = []string{"tool", "approver", "approved_at", "scope", "note", "sig_alg", "key_id", "state", "path"}
`

func ccatTestSHA(t *testing.T, s string) string {
	t.Helper()
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// ccatTestRepoRoot = cwd 上溯至最近的含 `scripts/gates/` 的目录。
func ccatTestRepoRoot(t *testing.T) string {
	t.Helper()
	d, err := os.Getwd()
	if err != nil {
		t.Fatalf("取不到 cwd：%v", err)
	}
	for i := 0; i < 12 && d != "" && d != "/"; i++ {
		if st, err := os.Stat(ccatPyJoin(d, "scripts", "gates")); err == nil && st.IsDir() {
			return d
		}
		p := filepath.Dir(d)
		if p == d {
			break
		}
		d = p
	}
	t.Skipf("找不到仓根（%s 上没有 scripts/gates/）⇒ 本格跳过", d)
	return ""
}

// ccatTestPyPath —— 仓内 py 原件（**只读** · ✗ 本单不改它）。
func ccatTestPyPath(t *testing.T) string {
	t.Helper()
	p := ccatPyJoin(ccatTestRepoRoot(t), "scripts", "gates", "check-approve-tier.py")
	if _, err := os.Stat(p); err != nil {
		t.Skipf("py 原件不在（%s）⇒ 本格跳过", p)
	}
	return p
}

// ccatTestCleanEnv —— 洗掉外部注入面后给 py 侧的 env（与 Go 侧 os.Environ 同一份口径）。
func ccatTestCleanEnv(base []string) []string {
	out := make([]string, 0, len(base))
	for _, e := range base {
		k := e
		if i := strings.Index(e, "="); i >= 0 {
			k = e[:i]
		}
		if k == "PYTHONPATH" || k == "ZERG_STATE_DIR" || strings.HasPrefix(k, "ZERG_PST_") {
			continue
		}
		out = append(out, e)
	}
	return out
}

// ccatTestWashInjection —— 侧面钉：两档共用同一份洗净后的外部注入面。
func ccatTestWashInjection(t *testing.T) {
	t.Helper()
	saved := map[string]string{}
	for _, k := range []string{"PYTHONPATH", "ZERG_STATE_DIR"} {
		saved[k] = os.Getenv(k)
		os.Unsetenv(k)
	}
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "ZERG_PST_") {
			k := e[:strings.Index(e, "=")]
			saved[k] = os.Getenv(k)
			os.Unsetenv(k)
		}
	}
	t.Cleanup(func() {
		for k, v := range saved {
			if v == "" {
				os.Unsetenv(k)
				continue
			}
			os.Setenv(k, v)
		}
	})
}

// ccatTestFixtureRoot —— 仓外夹具根：桩 bin/zerg ＋ 档位真源同形件（可切除/换成解不出的形态）。
func ccatTestFixtureRoot(t *testing.T, fields string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(ccatPyJoin(root, "bin"), 0o755); err != nil {
		t.Fatalf("建夹具 bin 失败：%v", err)
	}
	if err := os.MkdirAll(ccatPyJoin(root, "core", "cmd", "zerg"), 0o755); err != nil {
		t.Fatalf("建夹具 core 失败：%v", err)
	}
	if err := os.WriteFile(ccatPyJoin(root, "bin", "zerg"), []byte(ccatTestStub), 0o755); err != nil {
		t.Fatalf("写桩失败：%v", err)
	}
	if fields != "" {
		if err := os.WriteFile(ccatPyJoin(root, ccatApproveGoRel), []byte(fields), 0o644); err != nil {
			t.Fatalf("写夹具真源失败：%v", err)
		}
	}
	return root
}

// ccatTestRunPyGo —— 同一 argv / cwd / env：甲 = 仓外 py 副本所在的原件（钉 /usr/bin/python3），乙 = Go 入口。
func ccatTestRunPyGo(t *testing.T, argv []string) (int, string, string, int, string, string) {
	t.Helper()
	ccatTestWashInjection(t)
	py := ccatTestPyPath(t)
	root := ccatTestRepoRoot(t)
	if _, err := os.Stat(ccatTestPyExe); err != nil {
		t.Skipf("%s 不在 ⇒ 跳过 py↔go 对拍档", ccatTestPyExe)
	}
	cmd := exec.Command(ccatTestPyExe, append([]string{py}, argv...)...)
	cmd.Dir = root
	cmd.Env = ccatTestCleanEnv(os.Environ())
	var po, pe bytes.Buffer
	cmd.Stdout, cmd.Stderr = &po, &pe
	rcPy := 0
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			rcPy = ee.ExitCode()
		} else {
			t.Fatalf("跑 py 侧失败：%v", err)
		}
	}
	var goOut, goErr bytes.Buffer
	rcGo := checkApproveTierGateMain(&goOut, &goErr, argv)
	return rcPy, po.String(), pe.String(), rcGo, goOut.String(), goErr.String()
}

func ccatTestCountTemp(prefix string) int {
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

// ── ① 帮助面 byte 级 golden ──────────────────────────────────────────────────

func TestCcatHelpGoldenAgainstPy(t *testing.T) {
	if len(ccatHelp) != ccatTestPyHelpBytes {
		t.Fatalf("内置 ccatHelp 不是 %d 字节（现读 %d 字节）", ccatTestPyHelpBytes, len(ccatHelp))
	}
	if got := ccatTestSHA(t, ccatHelp); got != ccatTestPyHelpSHA256 {
		t.Fatalf("内置 ccatHelp 的 sha256 ≠ py `-h` 原件：\n  现读 %s\n  期望 %s", got, ccatTestPyHelpSHA256)
	}
	py := ccatTestPyPath(t)
	if _, err := os.Stat(ccatTestPyExe); err != nil {
		t.Skipf("%s 不在 ⇒ 跳过「现跑 py 原件复核」那一半", ccatTestPyExe)
	}
	cmd := exec.Command(ccatTestPyExe, py, "-h")
	var ob, eb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &ob, &eb
	if err := cmd.Run(); err != nil {
		t.Fatalf("跑 py `-h` 失败：%v（stderr=%s）", err, eb.String())
	}
	if ob.String() != ccatHelp {
		t.Fatalf("Go 内置 help 与 py `-h` **逐字节不同**（%d vs %d 字节）", len(ccatHelp), ob.Len())
	}
}

// ── ② Q-176：给了 `--json` 不许沉默 ───────────────────────────────────────────

func TestCcatRefuseJSONWithoutMachineFace(t *testing.T) {
	if got := ccatJSONArgs([]string{"--root", ".", "--self-test"}); len(got) != 0 {
		t.Fatalf("没给 `--json` 却拦下了：%v", got)
	}
	want := "⛔ 本门无机器读面：`--json` 在本门**未实现**（缺口 Q-176）\n" +
		"   字段表：无（本门只出人读面）\n" +
		"⇒ 用法错（rc=2）：**机器面缺 ≠ 零命中** —— 两态不许同形\n"
	var so, se bytes.Buffer
	if rc := checkApproveTierGateMain(&so, &se, []string{"--json", "x"}); rc != 2 {
		t.Fatalf("给了 `--json` 应 rc=2，得 rc=%d", rc)
	}
	if so.Len() != 0 {
		t.Fatalf("此档 stdout 必须**空**（机器面缺 ≠ 零命中）：%q", so.String())
	}
	if se.String() != want {
		t.Fatalf("stderr 不逐字：\n  现读 %q\n  期望 %q", se.String(), want)
	}
	var so2, se2 bytes.Buffer
	if rc := checkApproveTierGateMain(&so2, &se2, []string{"--root", ".", "--json=strength"}); rc != 2 {
		t.Fatalf("`--json=<字段>` 形态也应 rc=2，得 %d", rc)
	}
	if !strings.Contains(se2.String(), "`--json=strength`") {
		t.Fatalf("`--json=<字段>` 形态应逐字点名那一枚 token：%q", se2.String())
	}
}

// ── ③ argparse 等价面（每条都带牙）────────────────────────────────────────────

func TestCcatParseArgsUsageGrid(t *testing.T) {
	usage := ccatUsageLine + "\n" + ccatProg + ": error: "
	cases := []struct {
		name string
		argv []string
		want string
	}{
		{"坏旗标", []string{"--bogus"}, "unrecognized arguments: --bogus"},
		{"root 缺值", []string{"--root"}, "argument --root: expected one argument"},
		{"位置参数", []string{"foo"}, "unrecognized arguments: foo"},
		{"显式实参", []string{"--self-test=x"}, "argument --self-test: ignored explicit argument 'x'"},
		{"单横簇", []string{"-hx"}, "argument -h/--help: ignored explicit argument 'x'"},
		{"分隔符", []string{"--", "foo"}, "unrecognized arguments: -- foo"},
	}
	for _, c := range cases {
		var so, se bytes.Buffer
		rc := checkApproveTierGateMain(&so, &se, c.argv)
		if rc != 2 {
			t.Fatalf("%s：应 rc=2，得 %d", c.name, rc)
		}
		if so.Len() != 0 {
			t.Fatalf("%s：stdout 必须空，得 %q", c.name, so.String())
		}
		if want := usage + c.want + "\n"; se.String() != want {
			t.Fatalf("%s：stderr 不逐字：\n  现读 %q\n  期望 %q", c.name, se.String(), want)
		}
	}
	for _, argv := range [][]string{{"-h"}, {"--help"}} {
		var so, se bytes.Buffer
		if rc := checkApproveTierGateMain(&so, &se, argv); rc != 0 {
			t.Fatalf("%v：应 rc=0，得 %d", argv, rc)
		}
		if so.String() != ccatHelp {
			t.Fatalf("%v：stdout 应 == ccatHelp（%d vs %d 字节）", argv, so.Len(), len(ccatHelp))
		}
		if se.Len() != 0 {
			t.Fatalf("%v：stderr 必须空，得 %q", argv, se.String())
		}
	}
}

// ── ④⑤ 判定口：有序超集 / 已知项丢失（互斥断言）──────────────────────────────

func TestCcatJudgeOrderedSuperset(t *testing.T) {
	cols := []string{"tool", "approver", "approved_at", "scope", "state", "path", "档"}
	if p := ccatJudgeColumns(cols, nil); p == nil {
		t.Fatalf("返回 nil —— nil 不得兼表「取不到」与「空集合」（该返 []string{}）")
	} else if len(p) != 0 {
		t.Fatalf("正控应 0 条，得 %v", p)
	}
	// 新增项允许（有序超集）
	if p := ccatJudgeColumns(append([]string{"新列"}, cols...), nil); len(p) != 0 {
		t.Fatalf("新增列应允许，却判红 %v", p)
	}
	// 换序 ⇒ 红
	swap := append([]string{}, cols...)
	swap[0], swap[1] = swap[1], swap[0]
	if p := ccatJudgeColumns(swap, nil); len(p) == 0 {
		t.Fatalf("换序了还判绿 ⇒ 这一格是虚的")
	}
	// 删项 ⇒ 红
	if p := ccatJudgeColumns(cols[:len(cols)-2], nil); len(p) == 0 {
		t.Fatalf("删了必填列还判绿")
	}
}

func TestCcatKnownLostMutex(t *testing.T) {
	cols := []string{"tool", "approver", "approved_at", "scope", "state", "path", "档"}
	// ㈠ 丢「档」（不在必填集里）⇒ **只有**已知列判据抓得住（互斥：有序超集必须一条都不出）
	lost := cols[:len(cols)-1]
	sup := ccatJudgeColumns(lost, nil)
	lostP := ccatJudgeKnownColumnsLost(lost, nil)
	if sup == nil || lostP == nil {
		t.Fatalf("两判据都不得返 nil 空切片兼表「取不到」")
	}
	if len(sup) != 0 {
		t.Fatalf("丢「档」时有序超集不该红（它抓不住这一格）：%v", sup)
	}
	if len(lostP) == 0 {
		t.Fatalf("丢「档」时已知列判据必须红")
	}
	if !strings.Contains(lostP[0], "已知列") || strings.Contains(lostP[0], "该序里找不到") {
		t.Fatalf("㈠ 命中了自己那条文案却带了对方文案：%v", lostP)
	}
	// ㈡ 丢必填列 `path` ⇒ 有序超集红，且其文案里**不出现**「已知列」
	drop := []string{"tool", "approver", "approved_at", "scope", "state", "档"}
	sup2 := ccatJudgeColumns(drop, nil)
	if len(sup2) == 0 {
		t.Fatalf("丢必填列 `path` 时有序超集必须红")
	}
	if strings.Contains(sup2[0], "已知列") {
		t.Fatalf("㈡ 有序超集那条不该带「已知列」文案：%v", sup2)
	}
	// ㈢ 行标签同族
	labels := []string{"工具", "批准者", "签的时间", "范围", "理由", "签名", "档", "钥匙身份", "有效期", "判决", "落点"}
	lostL := append([]string{}, labels...)
	lostL = ccatRemoveFirst(lostL, "有效期")
	if p := ccatJudgeLabels(lostL, nil); len(p) != 0 {
		t.Fatalf("丢「有效期」（非必填）时有序超集不该红：%v", p)
	}
	if p := ccatJudgeKnownLabelsLost(lostL, nil); len(p) == 0 {
		t.Fatalf("丢「有效期」时已知行判据必须红")
	}
}

// ── ⑥ 行宽面 ────────────────────────────────────────────────────────────────

func TestCcatRowWidths(t *testing.T) {
	good := "tool\tapprover\tapproved_at\tscope\tstate\tpath\t档\n合成件\tMr2109\tx\t*\t无签名\t/p\tstrength=passphrase\n"
	if p := ccatJudgeRowWidths(good); len(p) != 0 {
		t.Fatalf("正控应 0 条，得 %v", p)
	}
	narrow := "tool\tapprover\tapproved_at\tscope\tstate\tpath\t档\nx\tx\tx\tx\tx\tx\n"
	p := ccatJudgeRowWidths(narrow)
	if len(p) == 0 {
		t.Fatalf("数据行少一格还判绿")
	}
	if !strings.Contains(p[0], "第 2 行格数 6 ≠ 表头 7") {
		t.Fatalf("点名不逐字：%v", p)
	}
	empty := ccatJudgeRowWidths("  \n\n")
	if len(empty) != 1 || !strings.Contains(empty[0], "一行都没有") {
		t.Fatalf("空输入应点名「表头也缺」：%v", empty)
	}
	if cols := ccatParseLsColumns("  \n\n"); cols == nil || len(cols) != 0 {
		t.Fatalf("空输入应给**非 nil** 空切片，得 %#v", cols)
	}
}

// ── ⑦ parse_show_labels 的贪心复刻 ───────────────────────────────────────────

func TestCcatParseShowLabelsGreedy(t *testing.T) {
	cases := []struct {
		line string
		want string
		ok   bool
	}{
		{"工具\t: 合成件", "工具", true},
		{"工具: 合成件", "工具", true},
		{"工具:", "工具", true},
		{"a: :b", "a:", true}, // 贪心：`\S+` 取到第二个冒号前
		{" a: x", "", false},  // 前导空白 ⇒ `^(\S+)` 不匹配
		{"工具：合成件", "", false}, // 全角冒号 ⇒ 不匹配
		{"没有冒号", "", false},
	}
	for _, c := range cases {
		got, ok := ccatShowLabelOf(c.line)
		if got != c.want || ok != c.ok {
			t.Fatalf("行 %q：得 (%q,%v)，期望 (%q,%v)", c.line, got, ok, c.want, c.ok)
		}
	}
	labels := ccatParseShowLabels("工具\t: a\n批准者   : b\n别的一行\n")
	if len(labels) != 2 || labels[0] != "工具" || labels[1] != "批准者" {
		t.Fatalf("多行解析得 %v", labels)
	}
}

// ── ⑧ 字段面九键（互斥文案）───────────────────────────────────────────────────

func TestCcatFieldsFace(t *testing.T) {
	if p := ccatJudgeFields(ccatExpectedFields); p == nil || len(p) != 0 {
		t.Fatalf("九键正控应 0 条（且非 nil），得 %#v", p)
	}
	// 换序：**只有**「≠ 九键」那条，不带「被塞进必填集」
	swapped := append([]string{ccatExpectedFields[1], ccatExpectedFields[0]}, ccatExpectedFields[2:]...)
	p := ccatJudgeFields(swapped)
	if len(p) == 0 {
		t.Fatalf("换序还判绿")
	}
	if !strings.Contains(p[0], "≠ 九键") || strings.Contains(p[0], "被塞进必填集") {
		t.Fatalf("换序命中了自己那条却带了对方文案：%v", p)
	}
	// 塞 `strength`：命中「被塞进必填集」，且**不**把九键那条当唯一依据
	squeezed := append(append([]string{}, ccatExpectedFields...), "strength")
	p2 := ccatJudgeFields(squeezed)
	hit := false
	for _, x := range p2 {
		if strings.Contains(x, "被塞进必填集") && strings.Contains(x, "strength") {
			hit = true
		}
	}
	if !hit {
		t.Fatalf("塞 `strength` 必须点名「被塞进必填集」（`D1` 红线）：%v", p2)
	}
	if p3 := ccatJudgeFields(append(append([]string{}, ccatExpectedFields...), "key_id_in_use")); len(p3) < 2 {
		t.Fatalf("两枚红线字段都该各自命中：%v", p3)
	}
}

// ── ⑨ 取不到 vs 空集合（两态分开表意）────────────────────────────────────────

func TestCcatParseFieldsFromGoTwoStates(t *testing.T) {
	if got, ok := ccatParseFieldsFromGo("package main\n"); ok || got != nil {
		t.Fatalf("解不出应 (nil,false)（= 取不到），得 (%#v,%v)", got, ok)
	}
	got, ok := ccatParseFieldsFromGo("package main\n\nvar approveFields = []string{}\n")
	if !ok {
		t.Fatalf("「解得动但零枚」不是「取不到」")
	}
	if got == nil {
		t.Fatalf("空集合必须是**非 nil** 空切片（nil 不得兼表两态）")
	}
	if len(got) != 0 {
		t.Fatalf("应 0 枚，得 %#v", got)
	}
	// 吃真仓物料：九键逐字 == 真源 `core/cmd/zerg/family_approve.go`
	raw, err := os.ReadFile(ccatPyJoin(ccatTestRepoRoot(t), ccatApproveGoRel))
	if err != nil {
		t.Skipf("档位真源读不到（%v）⇒ 本格跳过", err)
	}
	real, ok2 := ccatParseFieldsFromGo(string(raw))
	if !ok2 {
		t.Fatalf("真源 `%s` 里解不出 `approveFields`", ccatApproveGoRel)
	}
	if !ccatStrSliceEq(real, ccatExpectedFields) {
		t.Fatalf("真源九键与期望不逐字：\n  现读 %v\n  期望 %v", real, ccatExpectedFields)
	}
}

// ── ⑩ 运行档 py↔go 对拍（逐字节）────────────────────────────────────────────

func TestCcatParityPySelfTestByte(t *testing.T) {
	root := ccatTestFixtureRoot(t, ccatTestFamilyApprove)
	argv := []string{"--root", root, "--self-test"}
	rcP, outP, errP, rcG, outG, errG := ccatTestRunPyGo(t, argv)
	if rcP != 0 || rcG != 0 {
		t.Fatalf("正控档应两侧 rc=0，得 py=%d go=%d\npy out=%s\ngo out=%s", rcP, rcG, outP, outG)
	}
	if outP != outG {
		t.Fatalf("运行档 stdout 不逐字节：\npy=%q\ngo=%q", outP, outG)
	}
	if errP != errG {
		t.Fatalf("运行档 stderr 不逐字节：\npy=%q\ngo=%q", errP, errG)
	}
	if !strings.Contains(outG, "① 正控：现跑人面六列 / 八行 / 九字段都过（列数 7 · 行标签 11）") {
		t.Fatalf("正控档没打到那一行（桩或夹具不对）：%q", outG)
	}
	if !strings.Contains(outG, "10 格过 · 0 格不过") {
		t.Fatalf("自检 10 格应全过：%q", outG)
	}
}

func TestCcatParityPyScanByte(t *testing.T) {
	root := ccatTestFixtureRoot(t, ccatTestFamilyApprove)
	argv := []string{"--root", root, "--no-self-test"}
	rcP, outP, errP, rcG, outG, errG := ccatTestRunPyGo(t, argv)
	if rcP != rcG || outP != outG || errP != errG {
		t.Fatalf("全跑档不逐字节：rc py=%d go=%d\npy out=%q\ngo out=%q\npy err=%q\ngo err=%q",
			rcP, rcG, outP, outG, errP, errG)
	}
	if rcG != 0 {
		t.Fatalf("桩档应全绿（rc=0），得 %d：%s", rcG, outG)
	}
	if !strings.Contains(outG, "⇒ 全绿（rc=0）：T1–T6 全过 · 漂移 0 条") {
		t.Fatalf("缺全绿行：%q", outG)
	}
	// 负控（同一桩 · 换序旋钮）：两侧都必须 rc=1 且逐字节同
	t.Setenv("CCAT_STUB_SWAP", "1")
	rcP2, outP2, errP2, rcG2, outG2, errG2 := ccatTestRunPyGo(t, argv)
	if rcP2 != 1 || rcG2 != 1 {
		t.Fatalf("换序档应两侧 rc=1，得 py=%d go=%d\n%s", rcP2, rcG2, outG2)
	}
	if outP2 != outG2 || errP2 != errG2 {
		t.Fatalf("换序档不逐字节：\npy=%q\ngo=%q", outP2, outG2)
	}
	if !strings.Contains(outG2, "T1 `approve ls` 列：'approver' 在该序里找不到") {
		t.Fatalf("换序档没点名那一列（换序后 `tool` 落在 index 1 ⇒ 找不到的是它后面的 'approver'）：%q", outG2)
	}
	// 丢已知列「档」：两侧都必须点名「已知列 '档' 丢了」
	t.Setenv("CCAT_STUB_SWAP", "")
	t.Setenv("CCAT_STUB_DROP_DANG", "1")
	rcP3, outP3, _, rcG3, outG3, _ := ccatTestRunPyGo(t, argv)
	if rcP3 != 1 || rcG3 != 1 {
		t.Fatalf("丢「档」档应两侧 rc=1，得 py=%d go=%d", rcP3, rcG3)
	}
	if outP3 != outG3 {
		t.Fatalf("丢「档」档不逐字节：\npy=%q\ngo=%q", outP3, outG3)
	}
	if !strings.Contains(outG3, "已知列 '档' 丢了") {
		t.Fatalf("丢「档」档没点名：%q", outG3)
	}
}

// ── ⑪ 子进程面：ZERG_STATE_DIR 真注进去 / 旋钮真被读到 ────────────────────────

func TestCcatRunCLIInjectsStateDirAndReadsKnobs(t *testing.T) {
	root := ccatTestFixtureRoot(t, ccatTestFamilyApprove)
	state := t.TempDir()
	argv := []string{"--root", root, "--no-self-test"}
	if rc, out, _ := (func() (int, string, string) {
		var o, e bytes.Buffer
		rc := checkApproveTierGateMain(&o, &e, argv)
		return rc, o.String(), e.String()
	})(); rc != 0 {
		t.Fatalf("正控档应 rc=0，得 %d：%s", rc, out)
	}
	// 旋钮真被读到（否则上面那些负控就是假的）：打一发 env 里带 TOKEN 的档
	t.Setenv("CCAT_STUB_TOKEN", "1")
	var o, e bytes.Buffer
	if rc := checkApproveTierGateMain(&o, &e, argv); rc != 1 {
		t.Fatalf("注入 `token` 应 rc=1（T5 破），得 %d：%s", rc, o.String())
	}
	if !strings.Contains(o.String(), "T5 破：输出里出现 'token'") {
		t.Fatalf("没点名 T5 破：%q", o.String())
	}
	// 状态目录真注进去了：桩把 ${ZERG_STATE_DIR} 回显在 T2 行里 ⇒ 只要它非空就说明注进去了
	if !strings.Contains(o.String(), "T2 `approve show` 行标签") {
		t.Fatalf("缺 T2 行：%q", o.String())
	}
	rs := ccatRunCLI(ccatPyJoin(root, "bin", "zerg"), root, state, []string{"approve", "ls"})
	if rs.rcNone || rs.rc != 0 {
		t.Fatalf("直接跑桩应 rc=0，得 rcNone=%v rc=%d err=%q", rs.rcNone, rs.rc, rs.err)
	}
	if !strings.Contains(rs.out, state) {
		t.Fatalf("`ZERG_STATE_DIR` 没注进子进程（桩回显里看不到 %q）：%q", state, rs.out)
	}
}

// ── ⑫ A 类止血回归牙：临时目录 delta=0 ───────────────────────────────────────

func TestCcatNoTempLeak(t *testing.T) {
	// ★ 把本测试的 TMPDIR 指到自己的私有目录（`os.TempDir()` 每次调用**现读** `TMPDIR`，不缓存）。
	// 下面按**固定前缀**数 `os.TempDir()` 里的残件 —— 那是全局面：门分片器并发起多进程跑同包时，
	// 别的组会同时增删同前缀目录 ⇒ 计数飘 ⇒ 假红（缺口 `GAP-20261002-05`）。私有 TMPDIR 后
	// 「跑前/跑后」只看得见本测试自己的产物 ⇒ 判据不变严也不放宽，只去掉跨进程噪声。
	tmpOwn := t.TempDir()
	t.Setenv("TMPDIR", tmpOwn)
	root := ccatTestFixtureRoot(t, ccatTestFamilyApprove)
	pre := ccatTestCountTemp("zerg-approve-tier")
	if pre < 0 {
		t.Skipf("读不到 %s ⇒ 本格跳过", os.TempDir())
	}
	var o, e bytes.Buffer
	if rc := checkApproveTierGateMain(&o, &e, []string{"--root", root, "--self-test"}); rc != 0 {
		t.Fatalf("自检档应 rc=0，得 %d：%s", rc, o.String())
	}
	if got := ccatTestCountTemp("zerg-approve-tier"); got != pre {
		t.Fatalf("自检残件：跑前 %d 个 · 跑后 %d 个（py 侧 finally 回收 ⇒ Go 侧 defer 必须对齐）", pre, got)
	}
	var o2, e2 bytes.Buffer
	if rc := checkApproveTierGateMain(&o2, &e2, []string{"--root", root, "--no-self-test"}); rc != 0 {
		t.Fatalf("全跑档应 rc=0，得 %d：%s", rc, o2.String())
	}
	if got := ccatTestCountTemp("zerg-approve-tier"); got != pre {
		t.Fatalf("全跑档残件：跑前 %d 个 · 跑后 %d 个", pre, got)
	}
}

// ── ⑬⑭ 外部真源/退码表 ──────────────────────────────────────────────────────

func TestCcatExternalSourceAndExitCodes(t *testing.T) {
	root := ccatTestRepoRoot(t)
	if ccatApproveGoRel != "core/cmd/zerg/family_approve.go" {
		t.Fatalf("档位真源相对路径被改：%q", ccatApproveGoRel)
	}
	if ccatTestPyExe != "/usr/bin/python3" {
		t.Fatalf("解释器必须钉死（GAP-20261001-61）：%q", ccatTestPyExe)
	}
	raw, err := os.ReadFile(ccatPyJoin(root, "core", "cmd", "zerg", "exitcodes.go"))
	if err != nil {
		t.Skipf("退码表读不到（%v）⇒ 本格跳过", err)
	}
	re := regexp.MustCompile(`(?m)^\s*\{(\d+),\s*"`)
	declared := map[int]bool{}
	for _, m := range re.FindAllStringSubmatch(string(raw), -1) {
		n, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		declared[n] = true
	}
	if len(declared) == 0 {
		t.Fatalf("退码表一个码都没解析到（空转 = 假覆盖）")
	}
	for _, rc := range []int{0, 1, 2} {
		if !declared[rc] {
			t.Errorf("本件用到的裸数字退码 %d **不在**真源退码表 `core/cmd/zerg/exitcodes.go` 已登记集里", rc)
		}
	}
}

// ── 补充：取不到读数两态（缺真源 / 解不出）────────────────────────────────────

func TestCcatCollectReadingsWhy(t *testing.T) {
	// 缺真源：夹具里没有 `family_approve.go`
	nofield := ccatTestFixtureRoot(t, "")
	if _, why := ccatCollectReadings(nofield, t.TempDir(), t.TempDir()); !strings.Contains(why, "取不到字段面") {
		t.Fatalf("缺真源应报「取不到字段面」，得 why=%q", why)
	}
	// 解不出：有件但没有那一枚变量
	nomatch := ccatTestFixtureRoot(t, "package main\n\n// 没有那一枚变量\n")
	if _, why := ccatCollectReadings(nomatch, t.TempDir(), t.TempDir()); !strings.Contains(why, "解不出来") {
		t.Fatalf("解不出应报「解不出来」，得 why=%q", why)
	}
	// 两态**分开表意**：why 非空的两种文案不互串
	var o, e bytes.Buffer
	if rc := checkApproveTierGateMain(&o, &e, []string{"--root", nofield, "--no-self-test"}); rc != 2 {
		t.Fatalf("缺真源应 rc=2（不给结论），得 %d", rc)
	}
	if !strings.Contains(o.String(), "⛔ 未取数：") || !strings.Contains(o.String(), "不许把读不到当绿") {
		t.Fatalf("缺真源档没按 Q-176 同族口径点名：%q", o.String())
	}
	_ = io.Discard
}
