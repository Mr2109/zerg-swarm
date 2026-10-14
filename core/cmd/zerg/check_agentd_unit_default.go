// check_agentd_unit_default.go —— 判据件 `scripts/gates/check-agentd-unit-default.sh`（280 行）的 **Go 真搬**
// （M2 一枚 · 键 `check-agentd-unit-default` · 入口 checkAgentdUnitDefaultGateMain · 承接 GAP-20261012-16 存量面）。
//
// 判的仍是同一件事：`<根>/scripts/svc/setup-zerg.sh` 里 agent 守护单元名的**缺省必须从机名推导**
// （`AGENTD_UNIT="${NODE_NAME}-agent"`）· ✗ 不得硬编码 `zerg-agentd` 当单元名缺省 · 且 darwin 半边
// （为 agent 写用户域 LaunchAgent 的 `write_agent_plist` 函数 ＋ 作业名 `com.zerg.agent-${NODE_NAME}`）同在。
//
// 静态只读 · 只读**一个**件；逐行剥**整行注释**后断言：
//
//	A1 单元名缺省从机名推导：存在一行**同时**含 `AGENTD_UNIT=` 与 `${NODE_NAME}-agent`
//	A2 不得硬编码 `zerg-agentd` 当单元名缺省：无任何行同时含 `AGENTD_UNIT=` 与字面 `zerg-agentd`
//	A3a darwin 半边在：件含 `write_agent_plist`，且一行同时含它与其后的 `)`
//	A3b LaunchAgent 作业名缺省从机名推导：一行同时含 `AGENTD_LABEL=` 与 `com.zerg.agent-${NODE_NAME}`
//	A4  机名缺省在（推导的根）：一行同时含 `NODE_NAME=` 与 `hostname`
//
// 退码（三档）：0 四格全过 · 1 命中（硬编码 / 缺机名推导 / 缺 darwin 半边 / 缺机名缺省）·
// 2 **不给结论**（件读不到 / 件不在盘 / 用法错）。
//
// 出厂声明 (a) 默认根定位差异：sh 件零 `__file__` 语境 —— `--root` 缺省同样从 **cwd** 上溯找含
// `core/internal/version/version.go` 的目录（至多 10 级 · 找不到回落 cwd），本件逐字同构同一套上溯；
// 两侧同 cwd ⇒ **逐字同**。★ 仓根锚点一律用**稳定件** `core/internal/version/version.go`，
// ✗ 不用本判据自己的 sh 件（件一退役即失锚）、✗ 不用 `.git`（包目录下可能有游离嵌套仓）。
//
// 出厂声明 (b) 自指：本判据扫描面 = `scripts/svc/setup-zerg.sh` 单件（只读），本件落 `core/cmd/zerg/`
// ⇒ 不落在自身扫描面内；件内无私有路径字面量、无解析后会悬空的文档树字面。
// ★ 本件**不 spawn 任何子进程**（零 os/exec）⇒ 走 inproc 只读快返档。
package main

import (
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	// caudProg —— 件名（报话前缀 · 与 sh 同名）。
	caudProg = "check-agentd-unit-default.sh"
	// caudRelDefault —— 缺省被判件（仓内相对）。
	caudRelDefault = "scripts/svc/setup-zerg.sh"
	// caudRootAnchor —— 仓根锚（稳定件）。
	caudRootAnchor = "core/internal/version/version.go"

	// 断言用字面（逐字照 sh 的常量段 · 含 `${…}` 一律原样）。
	caudKUnit        = "AGENTD_UNIT="
	caudKLabel       = "AGENTD_LABEL="
	caudKNode        = "NODE_NAME="
	caudVDerive      = "${NODE_NAME}-agent"
	caudVDeriveLabel = "com.zerg.agent-${NODE_NAME}"
	caudFnHostname   = "hostname"
)

// caudVHardcode —— A2 的「硬编码单元名」串。★ 用 `var`（✗ 不 const）以便测试件哑化留牙。
var caudVHardcode = "zerg-agentd"

// caudFnPlist —— darwin 半边的函数名（A3a）。★ 用 `var` 同上（哑化留牙）。
var caudFnPlist = "write_agent_plist"

const (
	caudRCOK      = 0
	caudRCFail    = 1
	caudRCBlocked = 2
)

// caudUsage —— usage() 的三行（逐字照 sh）。
const caudUsage = "用法：" + caudProg + " [--root <仓根>] [--script <安装器件路径>] [--self-test] [--help]\n" +
	"判据：agent 单元名缺省必须**从机名推导**（AGENTD_UNIT=${NODE_NAME}-agent），✗ 不得硬编码 zerg-agentd；且 darwin 半边 write_agent_plist 同在。\n" +
	"退码：0 绿 · 1 红（硬编码 / 缺推导 / 缺 darwin 半边 / 缺机名缺省）· 2 不给结论（读不到件 / 件不在盘）。\n"

func caudPrintUsage(out io.Writer) { fmt.Fprint(out, caudUsage) }

// caudBashSpace —— bash `[[:space:]]`（ASCII 六枚）。
func caudBashSpace(r rune) bool {
	switch r {
	case ' ', '\t', '\n', '\v', '\f', '\r':
		return true
	}
	return false
}

// caudSplitReadLines —— 照 `while IFS= read -r line || [ -n "$line" ]`：按 `\n` 切；
// 末尾若恰是空段（件以 `\n` 收尾）则丢弃（＝那一次 read 返回 EOF 非零且 line 为空 ⇒ 不进循环体）。
func caudSplitReadLines(data []byte) []string {
	parts := strings.Split(string(data), "\n")
	if n := len(parts); n > 0 && parts[n-1] == "" {
		parts = parts[:n-1]
	}
	return parts
}

// caudTrunc400 —— bash `${var:0:400}`（按**字符**取前 400）。
func caudTrunc400(s string) string {
	rs := []rune(s)
	if len(rs) <= 400 {
		return s
	}
	return string(rs[:400])
}

// caudPyDirname —— posixpath.dirname 同构（`dirname(1)` 语义）。
func caudPyDirname(p string) string {
	i := strings.LastIndexByte(p, '/') + 1
	head := p[:i]
	if head != "" && head != strings.Repeat("/", len(head)) {
		head = strings.TrimRight(head, "/")
	}
	return head
}

// caudFindRoot —— 照 sh find_root：从 cwd 上溯至多 10 级找含 caudRootAnchor 的目录；找不到 ⇒ 回落 cwd。
func caudFindRoot(cwd string) string {
	d := cwd
	for i := 0; i < 10; i++ {
		if st, err := os.Stat(d + "/" + caudRootAnchor); err == nil && st.Mode().IsRegular() {
			return d
		}
		if d == "/" || d == "" || d == "." {
			break
		}
		d = caudPyDirname(d)
	}
	return cwd
}

// caudJudge —— 主判据（f = 安装器件路径）。全部输出走 out（照 sh 的 printf 全落 stdout）。
func caudJudge(f string, out io.Writer) int {
	st, serr := os.Stat(f)
	if serr != nil || !st.Mode().IsRegular() {
		// sh `[ ! -f ]`：不在盘（含「是目录」—— `-f` 对目录为假）。
		fmt.Fprintf(out, "⛔ 不给结论（rc=%d）：安装器件不在盘：%s\n", caudRCBlocked, f)
		fmt.Fprintf(out, "%s\n", "   ⇒ 仓根/`--script` 可能给错（本判据只读这一个件）")
		return caudRCBlocked
	}
	fh, oerr := os.Open(f)
	if oerr != nil {
		// sh `[ ! -r ]`：读不到。
		fmt.Fprintf(out, "⛔ 不给结论（rc=%d）：安装器件读不到：%s\n", caudRCBlocked, f)
		return caudRCBlocked
	}
	data, rerr := io.ReadAll(fh)
	_ = fh.Close()
	if rerr != nil {
		fmt.Fprintf(out, "⛔ 不给结论（rc=%d）：安装器件读不到：%s\n", caudRCBlocked, f)
		return caudRCBlocked
	}

	n := 0
	a1, a2, a3fn, a3lbl, a4 := 0, 0, 0, 0, 0
	a1no, a2no, a3fnno, a3lblno, a4no := 0, 0, 0, 0, 0
	a2line := ""

	for _, line := range caudSplitReadLines(data) {
		n++
		stripped := strings.TrimLeftFunc(line, caudBashSpace)
		if strings.HasPrefix(stripped, "#") {
			continue
		}
		if a1 == 0 && strings.Contains(line, caudKUnit) && strings.Contains(line, caudVDerive) {
			a1, a1no = 1, n
		}
		if a2 == 0 && strings.Contains(line, caudKUnit) && strings.Contains(line, caudVHardcode) {
			a2, a2no, a2line = 1, n, line
		}
		if a3fn == 0 && strings.Contains(line, caudFnPlist) && strings.Contains(line, ")") {
			a3fn, a3fnno = 1, n
		}
		if a3lbl == 0 && strings.Contains(line, caudKLabel) && strings.Contains(line, caudVDeriveLabel) {
			a3lbl, a3lblno = 1, n
		}
		if a4 == 0 && strings.Contains(line, caudKNode) && strings.Contains(line, caudFnHostname) {
			a4, a4no = 1, n
		}
	}

	bad := 0
	if a2 != 0 {
		fmt.Fprintf(out, "✗ 红（rc=%d）：agent 单元名缺省**硬编码** `%s`（与实况 `<机名>-agent` 不符）\n", caudRCFail, caudVHardcode)
		fmt.Fprintf(out, "   件:行 = %s:%d\n", f, a2no)
		fmt.Fprintf(out, "   现行  = %s\n", caudTrunc400(a2line))
		fmt.Fprintf(out, "%s\n", "   修法  = 缺省改为按**机名推导**，例如：")
		fmt.Fprintf(out, "%s\n", "           AGENTD_UNIT=\"${ZERG_SETUP_AGENTD_UNIT:-}\"   # 缺省留空，参数面解析后按真源推导")
		fmt.Fprintf(out, "%s\n", "           [ -n \"$AGENTD_UNIT\" ] || AGENTD_UNIT=\"${NODE_NAME}-agent\"")
		fmt.Fprintf(out, "%s\n", "           ★ `<机名>` = NODE_NAME（缺省 $(hostname -s)）—— 与 deploy/服务声明.tsv 的 `<机名>-agent.service` 同源")
		bad = 1
	}
	if a1 == 0 {
		fmt.Fprintf(out, "✗ 红（rc=%d）：找不到「单元名缺省**从机名推导**」那一行（应有同时含 `%s` 与 `%s` 的行）\n", caudRCFail, caudKUnit, caudVDerive)
		fmt.Fprintf(out, "%s\n", "   修法  = 加一行把缺省钉到机名（参数面解析之后，✗ 别在参数面之前固定死）：")
		fmt.Fprintf(out, "%s\n", "           [ -n \"$AGENTD_UNIT\" ] || AGENTD_UNIT=\"${NODE_NAME}-agent\"")
		bad = 1
	}
	if a3fn == 0 {
		fmt.Fprintf(out, "✗ 红（rc=%d）：darwin 半边缺失 —— 件里没有 `%s`（为 agent 写用户域 LaunchAgent 的函数）\n", caudRCFail, caudFnPlist)
		fmt.Fprintf(out, "%s\n", "   修法  = 补回 darwin 侧 agent 托管的 `write_agent_plist()`（写 ~/Library/LaunchAgents/<作业名>.plist · 作业名 = com.zerg.agent-<机名>）")
		bad = 1
	}
	if a3lbl == 0 {
		fmt.Fprintf(out, "✗ 红（rc=%d）：找不到 LaunchAgent 作业名缺省从机名推导那一行（应有同时含 `%s` 与 `%s` 的行）\n", caudRCFail, caudKLabel, caudVDeriveLabel)
		fmt.Fprintf(out, "%s\n", "   修法  = 加一行： [ -n \"$AGENTD_LABEL\" ] || AGENTD_LABEL=\"com.zerg.agent-${NODE_NAME}\"")
		bad = 1
	}
	if a4 == 0 {
		fmt.Fprintf(out, "✗ 红（rc=%d）：找不到机名缺省那一行（应有同时含 `%s` 与 `%s` 的行 —— 推导的根）\n", caudRCFail, caudKNode, caudFnHostname)
		fmt.Fprintf(out, "%s\n", "   修法  = NODE_NAME=\"${ZERG_SETUP_NODE_NAME:-$(hostname -s 2>/dev/null || hostname 2>/dev/null || echo node)}\"")
		bad = 1
	}

	if bad != 0 {
		fmt.Fprintf(out, "⇒ 命中（rc=%d）：agent 单元名缺省口径不合真源（须 `<机名>-agent` 同形 · darwin 半边同在）\n", caudRCFail)
		return caudRCFail
	}

	fmt.Fprintf(out, "✓ 绿（rc=%d）：agent 单元名缺省从机名推导 · 无硬编码 `%s` · darwin 半边在\n", caudRCOK, caudVHardcode)
	fmt.Fprintf(out, "   件:行 = 推导=%s:%d · 机名缺省=%s:%d · LaunchAgent 作业=%s:%d · %s=%s:%d\n",
		f, a1no, f, a4no, f, a3lblno, caudFnPlist, f, a3fnno)
	return caudRCOK
}

// caudTmpBase —— sh 的 `${TMPDIR:-/tmp}`。
func caudTmpBase() string {
	if v := os.Getenv("TMPDIR"); v != "" {
		return v
	}
	return "/tmp"
}

// caudWriteFixture —— 造一枚自检夹具（全落仓外临时根）。
func caudWriteFixture(path string, body string) bool {
	return os.WriteFile(path, []byte(body), 0o644) == nil
}

// 自检夹具正文（逐字照 sh selftest 的 printf 段）。
const (
	caudFixGood = `NODE_NAME="${ZERG_SETUP_NODE_NAME:-$(hostname -s 2>/dev/null || echo node)}"
AGENTD_UNIT="${ZERG_SETUP_AGENTD_UNIT:-}"
AGENTD_LABEL="${ZERG_SETUP_AGENTD_LABEL:-}"
write_agent_plist() {
  local plist="$LAUNCHD_DIR/${AGENTD_LABEL}.plist"
  say "   🚀 已注册 ${PREFIX}/zerg-agentd"
}
[ -n "$AGENTD_UNIT" ]  || AGENTD_UNIT="${NODE_NAME}-agent"
[ -n "$AGENTD_LABEL" ] || AGENTD_LABEL="com.zerg.agent-${NODE_NAME}"
`
	caudFixBadHard = `NODE_NAME="${ZERG_SETUP_NODE_NAME:-$(hostname -s 2>/dev/null || echo node)}"
# 旧形：硬编码单元名（本判据剥注释，但真硬编码在非注释行）
AGENTD_UNIT="${ZERG_SETUP_AGENTD_UNIT:-zerg-agentd}"
AGENTD_LABEL="com.zerg.agent-${NODE_NAME}"
write_agent_plist() { :; }
`
	caudFixBadNoDerive = `NODE_NAME="${ZERG_SETUP_NODE_NAME:-$(hostname -s)}"
AGENTD_UNIT="${ZERG_SETUP_AGENTD_UNIT:-}"
AGENTD_LABEL="com.zerg.agent-${NODE_NAME}"
write_agent_plist() { :; }
`
	caudFixBadNoDarwin = `NODE_NAME="${ZERG_SETUP_NODE_NAME:-$(hostname -s)}"
AGENTD_UNIT="${ZERG_SETUP_AGENTD_UNIT:-}"
AGENTD_LABEL="${ZERG_SETUP_AGENTD_LABEL:-}"
[ -n "$AGENTD_UNIT" ]  || AGENTD_UNIT="${NODE_NAME}-agent"
[ -n "$AGENTD_LABEL" ] || AGENTD_LABEL="com.zerg.agent-${NODE_NAME}"
`
	caudFixBadNoLabel = `NODE_NAME="${ZERG_SETUP_NODE_NAME:-$(hostname -s)}"
AGENTD_UNIT="${ZERG_SETUP_AGENTD_UNIT:-}"
AGENTD_LABEL="com.zerg.agent-fixed"
write_agent_plist() { :; }
[ -n "$AGENTD_UNIT" ]  || AGENTD_UNIT="${NODE_NAME}-agent"
`
)

// caudSelftest —— 合成夹具（正控 ＋ 成对负控 · 全在仓外临时根 · 跑完自回收）。
func caudSelftest(out io.Writer) int {
	tmp, err := os.MkdirTemp(caudTmpBase(), "caud-selftest-")
	if err != nil {
		fmt.Fprint(out, "✗ 自检：临时根建不起来\n")
		return caudRCBlocked
	}
	defer os.RemoveAll(tmp) // 安全删点：仅在 MkdirTemp 成功此支挂 defer

	good := tmp + "/good.sh"
	badHard := tmp + "/bad-hardcoded.sh"
	badNoDerive := tmp + "/bad-noderive.sh"
	badNoDarwin := tmp + "/bad-nodarwin.sh"
	badNoLabel := tmp + "/bad-nolabel.sh"
	for _, fx := range []struct {
		path string
		body string
	}{
		{good, caudFixGood},
		{badHard, caudFixBadHard},
		{badNoDerive, caudFixBadNoDerive},
		{badNoDarwin, caudFixBadNoDarwin},
		{badNoLabel, caudFixBadNoLabel},
	} {
		if !caudWriteFixture(fx.path, fx.body) {
			fmt.Fprint(out, "✗ 自检：临时根建不起来\n")
			return caudRCBlocked
		}
	}

	ok := 1
	rc := caudJudgeToDiscard(good)
	if rc == 0 {
		fmt.Fprint(out, "  ✓ ① 正控（机名推导 + 无硬编码 + darwin 半边在）⇒ rc=0\n")
	} else {
		fmt.Fprintf(out, "  ✗ ① 正控该绿没绿（rc=%d）\n", rc)
		ok = 0
	}
	rc = caudJudgeToDiscard(badHard)
	if rc == 1 {
		fmt.Fprint(out, "  ✓ ② 负控（硬编码 zerg-agentd · 即改前旧形）⇒ rc=1\n")
	} else {
		fmt.Fprintf(out, "  ✗ ② 负控该红没红（rc=%d）\n", rc)
		ok = 0
	}
	var b strings.Builder
	_ = caudJudge(badHard, &b)
	if strings.Contains(b.String(), badHard+":3") {
		fmt.Fprint(out, "  ✓ ②b 且点名 件:行（:3）\n")
	} else {
		fmt.Fprint(out, "  ✗ ②b 没点名 件:行\n")
		ok = 0
	}
	rc = caudJudgeToDiscard(badNoDerive)
	if rc == 1 {
		fmt.Fprint(out, "  ✓ ③ 负控（缺机名推导 · A1）⇒ rc=1\n")
	} else {
		fmt.Fprintf(out, "  ✗ ③ 该红没红（rc=%d）\n", rc)
		ok = 0
	}
	rc = caudJudgeToDiscard(badNoDarwin)
	if rc == 1 {
		fmt.Fprint(out, "  ✓ ④ 负控（缺 write_agent_plist · darwin 半边不在）⇒ rc=1\n")
	} else {
		fmt.Fprintf(out, "  ✗ ④ 该红没红（rc=%d）\n", rc)
		ok = 0
	}
	rc = caudJudgeToDiscard(badNoLabel)
	if rc == 1 {
		fmt.Fprint(out, "  ✓ ⑤ 负控（LaunchAgent 作业名硬编码 · 缺机名推导 · A3b）⇒ rc=1\n")
	} else {
		fmt.Fprintf(out, "  ✗ ⑤ 该红没红（rc=%d）\n", rc)
		ok = 0
	}
	rc = caudJudgeToDiscard(tmp + "/not-there.sh")
	if rc == 2 {
		fmt.Fprint(out, "  ✓ ⑥ 负控（件不在盘）⇒ rc=2\n")
	} else {
		fmt.Fprintf(out, "  ✗ ⑥ 该 rc=2 没到（rc=%d）\n", rc)
		ok = 0
	}

	if ok == 1 {
		fmt.Fprint(out, "自检 PASS（6 格）\n")
		return caudRCOK
	}
	fmt.Fprint(out, "自检 FAIL（6 格）\n")
	return caudRCBlocked
}

// caudJudgeToDiscard —— 跑判据、吞输出（照 sh 的 `> /dev/null 2>&1`）。
func caudJudgeToDiscard(f string) int {
	var b strings.Builder
	return caudJudge(f, &b)
}

// checkAgentdUnitDefaultGateMain = 本判据的 Go 入口（argv 不含程序名；stdout/stderr 由调用方给）。
// ★ sh 件的一切输出（含 usage / 用法错）都走 stdout ⇒ 本件逐字同（stderr 不落字）。
func checkAgentdUnitDefaultGateMain(stdout, stderr io.Writer, argv []string) int {
	root := ""
	script := ""
	doSelf := false
	i := 0
	for i < len(argv) {
		a := argv[i]
		switch {
		case a == "--root":
			i++
			if i >= len(argv) {
				fmt.Fprintln(stdout, "用法错：`--root` 缺值 ⇒ 不给结论（rc=2）")
				return caudRCBlocked
			}
			root = argv[i]
		case strings.HasPrefix(a, "--root="):
			root = a[len("--root="):]
		case a == "--script":
			i++
			if i >= len(argv) {
				fmt.Fprintln(stdout, "用法错：`--script` 缺值 ⇒ 不给结论（rc=2）")
				return caudRCBlocked
			}
			script = argv[i]
		case strings.HasPrefix(a, "--script="):
			script = a[len("--script="):]
		case a == "--self-test" || a == "--selftest":
			doSelf = true
		case a == "-h" || a == "--help":
			caudPrintUsage(stdout)
			return caudRCOK
		default:
			fmt.Fprintf(stdout, "用法错：未知参数 %s ⇒ 不给结论（rc=2）\n", a)
			caudPrintUsage(stdout)
			return caudRCBlocked
		}
		i++
	}
	if doSelf {
		return caudSelftest(stdout)
	}
	if root == "" {
		cwd, _ := os.Getwd()
		root = caudFindRoot(cwd)
	}
	f := script
	if f == "" {
		f = root + "/" + caudRelDefault
	}
	return caudJudge(f, stdout)
}
