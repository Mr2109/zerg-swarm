// check_upgrade_local_source.go —— 判据件 `scripts/gates/check-upgrade-local-source.sh`（274 行）的 **Go 真搬**
// （M2 一枚 · 键 `check-upgrade-local-source` · 入口 checkUpgradeLocalSourceGateMain · 承接 GAP-20261012-16 存量面）。
//
// 判的仍是同一件事：`<根>/scripts/build/zerg-upgrade.sh` 的「本机步」（锚 = 非注释行里的
// `ZERG_UPDATE_ROLE=controller`）必须**显式注入取源**：
//
//	A1 `ZERG_UPDATE_REMOTE=<值>` 在 且值**恒非空**（缺 / 空 / 无兜底 ⇒ 红）
//	A2 `ZERG_UPDATE_REF=<值>`    在 且值非空（缺 / 空字面 ⇒ 红）
//
// 「恒非空」（只对 REMOTE 这一格）= 字面量（值里不含 `$`）**或**带非空兜底的展开（`${VAR:-<非空默认>}` /
// `${VAR:=…}`）；纯展开 `${VAR}` / `$VAR` ⇒ 判红（变量未设时是空串 ⇒ 等于没注 ⇒ 落回内置公开源）。
//
// 退码（三档）：0 绿 · 1 命中（缺注入 / 值为空 / 无兜底）· 2 **不给结论**（件读不到 / 本机步锚 0 命中或多命中）。
//
// 出厂声明 (a) 默认根定位差异：sh 件零 `__file__` 语境 —— `--root` 缺省同样从 **cwd** 上溯找含
// `core/internal/version/version.go` 的目录（至多 10 级 · 找不到回落 cwd），本件逐字同构同一套上溯；
// 两侧同 cwd ⇒ **逐字同**。★ 仓根锚点用**稳定件** `core/internal/version/version.go`，
// ✗ 不用本判据自己的 sh 件（件一退役即失锚）、✗ 不用 `.git`（包目录下可能有游离嵌套仓）。
//
// 出厂声明 (b) 自指：本判据扫描面 = `scripts/build/zerg-upgrade.sh` 单件（只读），本件落 `core/cmd/zerg/`
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
	// cuplsProg —— 件名（报话前缀 · 与 sh 同名）。
	cuplsProg = "check-upgrade-local-source.sh"
	// cuplsRelDefault —— 缺省被判件（仓内相对）。
	cuplsRelDefault = "scripts/build/zerg-upgrade.sh"
	// cuplsKRef —— REF 键名（只在自检与报话里用；生产判定经 cuplsKeyRef）。
	cuplsKRef = "ZERG_UPDATE_REF="
	// cuplsRootAnchor —— 仓根锚（稳定件）。
	cuplsRootAnchor = "core/internal/version/version.go"
)

// cuplsKeyAnchor —— 本机步锚串。★ 用 `var`（✗ 不 const）以便测试件哑化留牙。
var cuplsKeyAnchor = "ZERG_UPDATE_ROLE=controller"

// cuplsKeyRemote —— REMOTE 键名。★ 用 `var` 同上（哑化留牙）。
var cuplsKeyRemote = "ZERG_UPDATE_REMOTE="

const (
	cuplsRCOK      = 0
	cuplsRCFail    = 1
	cuplsRCBlocked = 2
)

// cuplsUsageText —— usage() 的三行（逐字照 sh · 内含锚串现值 ⇒ 走函数取值）。
func cuplsUsageText() string {
	return "用法：" + cuplsProg + " [--root <仓根>] [--script <升级器件路径>] [--self-test] [--help]\n" +
		"判据：升级器「本机步」（锚 " + cuplsKeyAnchor + "）必须显式注入 REMOTE / REF（REMOTE 还须恒非空）。\n" +
		"退码：0 绿 · 1 红（缺注入 / 值为空 / 无兜底 ⇒ 会落回内置公开默认）· 2 不给结论（读不到件 / 锚 0 或 多命中）。\n"
}

func cuplsPrintUsage(out io.Writer) { fmt.Fprint(out, cuplsUsageText()) }

// cuplsBashSpace —— bash `[[:space:]]`（ASCII 六枚）。
func cuplsBashSpace(r rune) bool {
	switch r {
	case ' ', '\t', '\n', '\v', '\f', '\r':
		return true
	}
	return false
}

// cuplsSplitReadLines —— 照 `while IFS= read -r line || [ -n "$line" ]`（末尾空段丢弃）。
func cuplsSplitReadLines(data []byte) []string {
	parts := strings.Split(string(data), "\n")
	if n := len(parts); n > 0 && parts[n-1] == "" {
		parts = parts[:n-1]
	}
	return parts
}

// cuplsTrunc400 —— bash `${var:0:400}`（按字符取前 400）。
func cuplsTrunc400(s string) string {
	rs := []rune(s)
	if len(rs) <= 400 {
		return s
	}
	return string(rs[:400])
}

// cuplsPyDirname —— posixpath.dirname 同构。
func cuplsPyDirname(p string) string {
	i := strings.LastIndexByte(p, '/') + 1
	head := p[:i]
	if head != "" && head != strings.Repeat("/", len(head)) {
		head = strings.TrimRight(head, "/")
	}
	return head
}

// cuplsFindRoot —— 照 sh find_root：cwd 上溯至多 10 级找含 cuplsRootAnchor 的目录；找不到 ⇒ 回落 cwd。
func cuplsFindRoot(cwd string) string {
	d := cwd
	for i := 0; i < 10; i++ {
		if st, err := os.Stat(d + "/" + cuplsRootAnchor); err == nil && st.Mode().IsRegular() {
			return d
		}
		if d == "/" || d == "" || d == "." {
			break
		}
		d = cuplsPyDirname(d)
	}
	return cwd
}

// cuplsExtractValue —— 从一行里抠 `<KEY><值>` 的值段（引号感知：单/双引号取到配对引号；裸值取到空白）。
// 第二个返回值为 false = 该行不含 KEY（照 sh extract_value 的 `return 1`）。
func cuplsExtractValue(line, key string) (string, bool) {
	idx := strings.Index(line, key)
	if idx < 0 {
		return "", false
	}
	rest := line[idx+len(key):]
	switch {
	case strings.HasPrefix(rest, "\""):
		rest = rest[1:]
		if i := strings.Index(rest, "\""); i >= 0 {
			return rest[:i], true
		}
		return rest, true
	case strings.HasPrefix(rest, "'"):
		rest = rest[1:]
		if i := strings.Index(rest, "'"); i >= 0 {
			return rest[:i], true
		}
		return rest, true
	}
	if i := strings.IndexAny(rest, " \t\n\v\f\r"); i >= 0 {
		return rest[:i], true
	}
	return rest, true
}

// cuplsValueAlwaysNonempty —— 「恒非空」判定：字面量 或 带非空兜底的展开；无兜底的展开 ⇒ ✗。
func cuplsValueAlwaysNonempty(v string) bool {
	if v == "" {
		return false
	}
	if strings.Contains(v, ":-") || strings.Contains(v, ":=") {
		d := ""
		if i := strings.Index(v, ":-"); i >= 0 {
			d = v[i+2:]
		} else if i := strings.Index(v, ":="); i >= 0 {
			d = v[i+2:]
		}
		// 照 sh 里的第二次取（同式冗余，逐字保留以免口径漂）。
		if i := strings.Index(v, ":-"); i >= 0 {
			d = v[i+2:]
		}
		if i := strings.Index(d, "}"); i >= 0 {
			d = d[:i]
		}
		d = strings.TrimSuffix(d, "\"")
		d = strings.TrimSuffix(d, "'")
		return d != ""
	}
	if strings.Contains(v, "$") {
		return false
	}
	return true
}

// cuplsJudge —— 主判据（f = 升级器件路径）。全部输出走 out（照 sh 的 printf 全落 stdout）。
func cuplsJudge(f string, out io.Writer) int {
	st, serr := os.Stat(f)
	if serr != nil || !st.Mode().IsRegular() {
		fmt.Fprintf(out, "⛔ 不给结论（rc=%d）：升级器件不在盘：%s\n", cuplsRCBlocked, f)
		fmt.Fprintf(out, "%s\n", "   ⇒ 仓根/`--script` 可能给错（本判据只读这一个件）")
		return cuplsRCBlocked
	}
	fh, oerr := os.Open(f)
	if oerr != nil {
		fmt.Fprintf(out, "⛔ 不给结论（rc=%d）：升级器件读不到：%s\n", cuplsRCBlocked, f)
		return cuplsRCBlocked
	}
	data, rerr := io.ReadAll(fh)
	_ = fh.Close()
	if rerr != nil {
		fmt.Fprintf(out, "⛔ 不给结论（rc=%d）：升级器件读不到：%s\n", cuplsRCBlocked, f)
		return cuplsRCBlocked
	}

	n := 0
	hits := 0
	anchorLine := ""
	anchorNo := 0
	for _, line := range cuplsSplitReadLines(data) {
		n++
		stripped := strings.TrimLeftFunc(line, cuplsBashSpace)
		if strings.HasPrefix(stripped, "#") {
			continue
		}
		if strings.Contains(line, cuplsKeyAnchor) {
			hits++
			if anchorNo == 0 {
				anchorLine, anchorNo = line, n
			}
		}
	}

	if hits == 0 {
		fmt.Fprintf(out, "⛔ 不给结论（rc=%d）：在 %s 里找不到**本机步**锚 %s（0 命中）\n", cuplsRCBlocked, f, cuplsKeyAnchor)
		fmt.Fprintf(out, "%s\n", "   ⇒ 结构已变（本判据认的锚不在件里）⇒ ✗ 不静默判绿；请人工核「本机步」现在长什么样，并同批更新本判据")
		return cuplsRCBlocked
	}
	if hits > 1 {
		fmt.Fprintf(out, "⛔ 不给结论（rc=%d）：本机步锚 %s 在 %s 里命中 %d 行（>1 ⇒ 歧义）\n", cuplsRCBlocked, cuplsKeyAnchor, f, hits)
		fmt.Fprintf(out, "%s\n", "   ⇒ ✗ 不猜哪一行是本机步；请人工核并同批收窄本判据的锚")
		return cuplsRCBlocked
	}

	bad := 0
	remoteVal, remoteOK := cuplsExtractValue(anchorLine, cuplsKeyRemote)
	if !remoteOK {
		fmt.Fprintf(out, "✗ 红（rc=%d）：本机步**没有注入** %s\n", cuplsRCFail, cuplsKeyRemote)
		fmt.Fprintf(out, "   件:行 = %s:%d\n", f, anchorNo)
		fmt.Fprintf(out, "   现行  = %s\n", cuplsTrunc400(anchorLine))
		fmt.Fprintf(out, "%s\n", "   修法  = 本机步按**远程步同形**补注取源三键（至少 REMOTE/REF），例如：")
		fmt.Fprintf(out, "%s\n", "           ZERG_UPDATE_REPO=\"$l_root\" ZERG_UPDATE_REF=\"$FLEET_REF\" ZERG_UPDATE_REMOTE=\"${FLEET_REMOTE:-$l_root}\"")
		fmt.Fprintf(out, "%s\n", "           ★ REMOTE 缺省必须钉到**本机仓根**（不联网、不取公网）：只注空值的 FLEET_REMOTE 等于没注 ⇒ 仍落公开源")
		bad = 1
	} else if !cuplsValueAlwaysNonempty(remoteVal) {
		fmt.Fprintf(out, "✗ 红（rc=%d）：本机步的 %s 值**不能保证非空**（现读 %s）\n", cuplsRCFail, cuplsKeyRemote, remoteVal)
		fmt.Fprintf(out, "   件:行 = %s:%d\n", f, anchorNo)
		fmt.Fprintf(out, "%s\n", "   修法  = 给非空兜底（无兜底的展开在变量未设时是空串 ⇒ 等于没注 ⇒ 落回内置公开源），例如：")
		fmt.Fprintf(out, "%s\n", "           ZERG_UPDATE_REMOTE=\"${FLEET_REMOTE:-$l_root}\"")
		bad = 1
	}
	refVal, refOK := cuplsExtractValue(anchorLine, cuplsKRef)
	if !refOK {
		fmt.Fprintf(out, "✗ 红（rc=%d）：本机步**没有注入** %s\n", cuplsRCFail, cuplsKRef)
		fmt.Fprintf(out, "   件:行 = %s:%d\n", f, anchorNo)
		fmt.Fprintf(out, "%s\n", "   修法  = 本机步按远程步同形补注 ZERG_UPDATE_REF（取机群目标 ref，如 ${FLEET_REF}）")
		bad = 1
	} else if refVal == "" {
		fmt.Fprintf(out, "✗ 红（rc=%d）：本机步的 %s 是**空值**（现读 %s）\n", cuplsRCFail, cuplsKRef, refVal)
		fmt.Fprintf(out, "   件:行 = %s:%d\n", f, anchorNo)
		fmt.Fprintf(out, "%s\n", "   修法  = 注机群 ref（FLEET_REF 缺省 `main`·非空），如 ZERG_UPDATE_REF=\"$FLEET_REF\"")
		bad = 1
	}

	if bad != 0 {
		fmt.Fprintf(out, "⇒ 命中（rc=%d）：本机步取源缺格 ⇒ CLI 会落回内置公开默认（DefaultPublicRepoURL）⇒ 主控可能被换成公开旧版\n", cuplsRCFail)
		return cuplsRCFail
	}

	fmt.Fprintf(out, "✓ 绿（rc=%d）：本机步已显式注入取源（%s %s · %s %s）\n", cuplsRCOK, cuplsKeyRemote, remoteVal, cuplsKRef, refVal)
	fmt.Fprintf(out, "   件:行 = %s:%d\n", f, anchorNo)
	return cuplsRCOK
}

// cuplsTmpBase —— sh 的 `${TMPDIR:-/tmp}`。
func cuplsTmpBase() string {
	if v := os.Getenv("TMPDIR"); v != "" {
		return v
	}
	return "/tmp"
}

// cuplsJudgeToDiscard —— 跑判据、吞输出（照 sh 的 `> /dev/null 2>&1`）。
func cuplsJudgeToDiscard(f string) int {
	var b strings.Builder
	return cuplsJudge(f, &b)
}

// 自检夹具正文（逐字照 sh selftest 的 printf 段）。
const (
	cuplsFixGood = `some_other_line=1
lout="$(ZERG_BUILD_TIME="$BUILD_TIME" ZERG_UPDATE_ROLE=controller ZERG_UPDATE_REPO="$l_root" ZERG_UPDATE_REF="$FLEET_REF" ZERG_UPDATE_REMOTE="${FLEET_REMOTE:-$l_root}" sh -c "$lcmd" 2>&1)" || lrc=$?
`
	cuplsFixBadMissing = `# 注释里提一下 ZERG_UPDATE_REMOTE= 不算（本判据剥注释）
lout="$(ZERG_BUILD_TIME="$BUILD_TIME" ZERG_UPDATE_ROLE=controller sh -c "$lcmd" 2>&1)" || lrc=$?
`
	cuplsFixBadEmpty = `lout="$(ZERG_UPDATE_ROLE=controller ZERG_UPDATE_REF="$FLEET_REF" ZERG_UPDATE_REMOTE="" sh -c "$lcmd" 2>&1)" || lrc=$?
`
	cuplsFixBadNoVar = `lout="$(ZERG_UPDATE_ROLE=controller ZERG_UPDATE_REF="$FLEET_REF" ZERG_UPDATE_REMOTE="$FLEET_REMOTE" sh -c "$lcmd" 2>&1)" || lrc=$?
`
	cuplsFixBadRefEmpty = `lout="$(ZERG_UPDATE_ROLE=controller ZERG_UPDATE_REF="" ZERG_UPDATE_REMOTE="${FLEET_REMOTE:-$l_root}" sh -c "$lcmd" 2>&1)" || lrc=$?
`
	cuplsFixBadAnchor = `lout="$(ZERG_UPDATE_ROLE=node sh -c "$lcmd" 2>&1)" || lrc=$?
`
)

// cuplsSelftest —— 合成夹具（成对负控 · 全在仓外临时根 · 跑完自回收）。
func cuplsSelftest(out io.Writer) int {
	tmp, err := os.MkdirTemp(cuplsTmpBase(), "cupls-selftest-")
	if err != nil {
		fmt.Fprint(out, "✗ 自检：临时根建不起来\n")
		return cuplsRCBlocked
	}
	defer os.RemoveAll(tmp) // 安全删点：仅在 MkdirTemp 成功此支挂 defer

	good := tmp + "/good.sh"
	badMissing := tmp + "/bad-missing.sh"
	badEmpty := tmp + "/bad-empty.sh"
	badNoVar := tmp + "/bad-novar.sh"
	badRefEmpty := tmp + "/bad-refempty.sh"
	badAnchor := tmp + "/bad-anchor.sh"
	for _, fx := range []struct {
		path string
		body string
	}{
		{good, cuplsFixGood},
		{badMissing, cuplsFixBadMissing},
		{badEmpty, cuplsFixBadEmpty},
		{badNoVar, cuplsFixBadNoVar},
		{badRefEmpty, cuplsFixBadRefEmpty},
		{badAnchor, cuplsFixBadAnchor},
	} {
		if err := os.WriteFile(fx.path, []byte(fx.body), 0o644); err != nil {
			fmt.Fprint(out, "✗ 自检：临时根建不起来\n")
			return cuplsRCBlocked
		}
	}

	ok := 1
	rc := cuplsJudgeToDiscard(good)
	if rc == 0 {
		fmt.Fprint(out, "  ✓ ① 正控（已注入 REMOTE/REF）⇒ rc=0\n")
	} else {
		fmt.Fprintf(out, "  ✗ ① 正控该绿没绿（rc=%d）\n", rc)
		ok = 0
	}
	rc = cuplsJudgeToDiscard(badMissing)
	if rc == 1 {
		fmt.Fprint(out, "  ✓ ② 负控（缺 REMOTE/REF · 即改前原形）⇒ rc=1\n")
	} else {
		fmt.Fprintf(out, "  ✗ ② 负控该红没红（rc=%d）\n", rc)
		ok = 0
	}
	var b strings.Builder
	_ = cuplsJudge(badMissing, &b)
	if strings.Contains(b.String(), badMissing+":2") {
		fmt.Fprint(out, "  ✓ ②b 且点名 件:行（:2）\n")
	} else {
		fmt.Fprint(out, "  ✗ ②b 没点名 件:行\n")
		ok = 0
	}
	rc = cuplsJudgeToDiscard(badEmpty)
	if rc == 1 {
		fmt.Fprint(out, "  ✓ ③ 负控（REMOTE 给空串）⇒ rc=1\n")
	} else {
		fmt.Fprintf(out, "  ✗ ③ 该红没红（rc=%d）\n", rc)
		ok = 0
	}
	rc = cuplsJudgeToDiscard(badNoVar)
	if rc == 1 {
		fmt.Fprint(out, "  ✓ ④ 负控（REMOTE 无兜底展开 ⇒ 可为空）⇒ rc=1\n")
	} else {
		fmt.Fprintf(out, "  ✗ ④ 该红没红（rc=%d）\n", rc)
		ok = 0
	}
	rc = cuplsJudgeToDiscard(badRefEmpty)
	if rc == 1 {
		fmt.Fprint(out, "  ✓ ④b 负控（REF 给空字面）⇒ rc=1\n")
	} else {
		fmt.Fprintf(out, "  ✗ ④b 该红没红（rc=%d）\n", rc)
		ok = 0
	}
	rc = cuplsJudgeToDiscard(badAnchor)
	if rc == 2 {
		fmt.Fprint(out, "  ✓ ⑤ 负控（本机步锚 0 命中 ⇒ 结构变）⇒ rc=2\n")
	} else {
		fmt.Fprintf(out, "  ✗ ⑤ 该 rc=2 没到（rc=%d）\n", rc)
		ok = 0
	}
	rc = cuplsJudgeToDiscard(tmp + "/not-there.sh")
	if rc == 2 {
		fmt.Fprint(out, "  ✓ ⑥ 负控（件不在盘）⇒ rc=2\n")
	} else {
		fmt.Fprintf(out, "  ✗ ⑥ 该 rc=2 没到（rc=%d）\n", rc)
		ok = 0
	}

	if ok == 1 {
		fmt.Fprint(out, "自检 PASS（7 格）\n")
		return cuplsRCOK
	}
	fmt.Fprint(out, "自检 FAIL（7 格）\n")
	return cuplsRCBlocked
}

// checkUpgradeLocalSourceGateMain = 本判据的 Go 入口（argv 不含程序名；stdout/stderr 由调用方给）。
// ★ sh 件的一切输出（含 usage / 用法错）都走 stdout ⇒ 本件逐字同（stderr 不落字）。
func checkUpgradeLocalSourceGateMain(stdout, stderr io.Writer, argv []string) int {
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
				return cuplsRCBlocked
			}
			root = argv[i]
		case strings.HasPrefix(a, "--root="):
			root = a[len("--root="):]
		case a == "--script":
			i++
			if i >= len(argv) {
				fmt.Fprintln(stdout, "用法错：`--script` 缺值 ⇒ 不给结论（rc=2）")
				return cuplsRCBlocked
			}
			script = argv[i]
		case strings.HasPrefix(a, "--script="):
			script = a[len("--script="):]
		case a == "--self-test" || a == "--selftest":
			doSelf = true
		case a == "-h" || a == "--help":
			cuplsPrintUsage(stdout)
			return cuplsRCOK
		default:
			fmt.Fprintf(stdout, "用法错：未知参数 %s ⇒ 不给结论（rc=2）\n", a)
			cuplsPrintUsage(stdout)
			return cuplsRCBlocked
		}
		i++
	}
	if doSelf {
		return cuplsSelftest(stdout)
	}
	if root == "" {
		cwd, _ := os.Getwd()
		root = cuplsFindRoot(cwd)
	}
	f := script
	if f == "" {
		f = root + "/" + cuplsRelDefault
	}
	return cuplsJudge(f, stdout)
}
