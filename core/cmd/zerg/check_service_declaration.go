// check_service_declaration.go —— 判据件 `scripts/gates/check-service-declaration.py`（384 行）的 **Go 真搬**
// （M2 一枚 · 入口 `checkServiceDeclarationGateMain` · 承接 GAP-20261001-13）。
//
// 为什么有这件（py 件头原话口径）：门⑧ 服务声明树 —— 幽灵服务的判据**不是** PPID==1（本机三个自研
//
//	守护件全是 PPID=1，那是 launchd 收养的正常形态）⇒ 判据只能是「**声明的对象集**」与「**实际的
//	进程集**」的**差集**（§十五.3 对象清册 · §二十一 已红第 3 条 · 开工单 T-25）。
//
// 六条规则（任一不通过 ⇒ 见退出码）：
//
//	R1 声明树可解析且非空（`deploy/服务声明.tsv`；缺件 / 空转 ⇒ rc=2 不给结论）。
//	R2 **按名字集对拍**：`launchctl list` 的 `com.zerg.` 标签集 ⇄ 声明树 `kind=launchd` 的 `name` 集，
//	   **双向**点名（A∖B = 盘上有·声明无；B∖A = 声明有·盘上无）⇒ 差集非空即 rc=1；
//	   ★ 已登记的 ghost **不算幽灵**（人签设计），只有「未登记幽灵」判红，两态逐条分开点名。
//	R3 每条 launchd 声明的「声明件」（`~/…` 形态）在盘上真存在 ⇒ 否则 rc=1。
//	R0 层限（GAP-20260927-168）：本门只判**本机层**（`layer=host`）的行；非本机层行只计数、不由本门判；
//	   缺 `layer` 列（旧 7 列件）⇒ 全部按 `host` 解析，并出一条注记。
//	R4 现值进程逐条有落点 —— 分两侧：**R4a 作业面驱动**（以 `launchctl list` 的 pid 列取现值，
//	   用 pid 去 ps 取**那一条**命令行，再对声明行「进程特征」做**词界锚** ⇒ 抓「标签↔pid↔程序」不一致）
//	   ＋ **R4b 现值驱动**（ps 里每个自研件进程都必须被某条「进程特征」覆盖）；锚一律走**词界**（禁切词/子串）。
//	   ⇒ 覆盖不上 ⇒ rc=1。
//	R5 每条 ghost 必须写处置（处置列含 reap 或「待」）⇒ 否则 rc=1。
//	R6 **空转 = 假覆盖**：ps 里一件自研件都扫不到 ⇒ rc=2（不当绿）。
//
// 本件与新口径（照实）：
//
//	· 入参面：py 只有位置参数 root（nargs='?'，default '.'）＋ `--self-test`；本件**照抄同名同形态**，
//	  ✗ 不另起名、✗ 不新造 env。
//	· 取证环境旋钮（py 四处，逐字照抄同名）：声明树路径 · ps 输出落文件 · launchctl 输出落文件 · 家目录。
//	  缺省 ⇒ 真跑 `ps -eo pid,ppid,etime,command` 与 `launchctl list`。
//	· 子进程面：py 用 shell 直跑命令串（capture 输出 · text · 超时 30s）；本件用等价的外壳进程形态。
//	· 写面（照实申报）：判据档（无参 / 位置 root）**零写盘**；**仅** `--self-test` 档在系统临时目录造
//	  夹具（py 用 TemporaryDirectory 自查回收 ⇒ 本件同回收）。⇒ 除自检档外本件无写面。
//	· ✗ 不静默绿：所有「取不到 / 判不了 / 空转」路径都退 2，无静默绿面。
//
// 本单边界（照实）：本件**只落码、不接线** ——
//
//	· **不**在 `core/cmd/zerg/main.go` 注册命令（✗ 不进命令树 · ✗ 不改 add_step）；
//	· **不**删 `scripts/gates/check-service-declaration.py` 原件（退场规则：对拍通过后才退场）；
//	· 入口 `checkServiceDeclarationGateMain` 目前**无调用点**（M2 的刻意状态）。
//
// ★★ 两条标准声明行：
//
//	(a) **默认根定位差异**：py 的 `find_root` 由**脚本位置**（`__file__`）上溯找含 `core/cmd/zerg`
//	    的目录；本件**无 `__file__` 语境** ⇒ 用 argv 里的 root（缺省 '.'，与 py 逐字同）由 **cwd** 上溯。
//	    落同一真仓、cwd=仓根（或仓内任意层）时读数逐字相同（已对拍甲/乙）；「脚本位置 ≠ cwd 上溯根」
//	    时推导面不同 ⇒ **不属逐字对齐承诺**，对拍一律 cwd=仓根 / 显式传 root。
//	(b) **帮助面 argparse 长 usage**：py 的 `-h` 由 argparse 生成（本机 python 3.9.6 ⇒ 措辞为
//	    "optional arguments:"）；本件**逐字复刻** 3.9.6 的 `-h` stdout，并把其 sha256 钉成 golden
//	    （GAP-20261001-61：3.9 措辞 vs 3.14 不同，必须钉死一版）⇒ 见件尾 const 与单测断言。
//	    argparse 的前缀缩写 / `--x=y` 形态 / `-abc` 组合等**未复刻**（见件尾「未覆盖面」）。
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	csdProg = "check-service-declaration.py"
	csdTool = "check-service-declaration"

	// csdHelpGoldenSHA256 —— 帮助面 `-h` stdout 的 sha256（GAP-20261001-61 钉死一版）。
	// ★ 退役态（G17 · 2026-10-07 退役批）：金标真源由「py `-h` 输出」**收回本件 `csdHelpText` 常量**
	//   （Go 半即真源）；本字面量降为 Go 半的**冻结回归值**（csdHelpNativeSha256 现算自证）。
	//   为什么内化：原真源件 `scripts/gates/check-service-declaration.py`（py 半）正被退役（M2 判据件 Go 化），
	//   py 件一退役即无从再刷该金标 ⇒ 把真源内化进 Go（✗ 运行时不读任何 py）。
	csdHelpGoldenSHA256 = "ef130b81036c077cd892b01c105018d6b525e1320ce380790198e24ac013b44e"

	csdUsageShort = "usage: check-service-declaration.py [-h] [--self-test] [root]"

	csdHelpText = "usage: check-service-declaration.py [-h] [--self-test] [root]\n" +
		"\n" +
		"positional arguments:\n" +
		"  root\n" +
		"\n" +
		"optional arguments:\n" +
		"  -h, --help   show this help message and exit\n" +
		"  --self-test\n"
)

// csdHelpNativeSha256 —— 从 **Go 真源** `csdHelpText` 现算 sha256（✗ 不读 py）。
// 与冻结常量 csdHelpGoldenSHA256 自证：二者相等 ⇒ 帮助面真源在 Go 半、退役态无缺口。
func csdHelpNativeSha256() string {
	sum := sha256.Sum256([]byte(csdHelpText))
	return hex.EncodeToString(sum[:])
}

var (
	csdKinds = []string{"launchd", "ghost"}
	// csdProcHints —— 自研件 / 外置件的进程特征锚（与 py `PROC_HINTS` 逐条同源）：
	//   + `searx.webapp`（2026-10-07 A2 收尾）—— 外置 searxng 进程命令行 `… -m searx.webapp` 的锚。
	csdProcHints         = []string{"bin/zerg-", "bin/cocoon-", "searx.webapp"}
	csdZergLabelPrefixes = []string{"com.zerg."}

	// csdJSONFields —— 本门机器读面字段表（py 侧 = 空 ⇒ 无机器读面；此处同形保留为唯一真源）。
	csdJSONFields = []string{}
)

// csdRow —— 声明面一行（八列：kind/name/声明件/程序/进程特征/归属/处置/layer）。
// ★ 第 8 列 layer（GAP-20260927-168）：缺列（旧 7 列件）⇒ 空串 ⇒ 用时空串按 host 解析（层限）。
type csdRow struct {
	Line        int
	Kind        string
	Name        string
	Decl        string
	Prog        string
	Proc        string
	Owner       string
	Disposition string
	Layer       string
}

// csdEnv —— py `os.environ.get` 的等价：未设 / 设为空串都当「没有」（与 py 的 `or` 语义一致）。
func csdEnv(name string) string {
	if v, ok := os.LookupEnv(name); ok {
		return v
	}
	return ""
}

// csdFindRoot —— py `find_root` 逐字同形：由 start 上溯最多 8 层找含 `core/cmd/zerg` 的目录。
func csdFindRoot(start string) string {
	d, err := filepath.Abs(start)
	if err != nil {
		return ""
	}
	for i := 0; i < 8; i++ {
		if fi, err := os.Stat(filepath.Join(d, "core", "cmd", "zerg")); err == nil && fi.IsDir() {
			return d
		}
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		d = parent
	}
	return ""
}

// csdPyRepr —— Python str repr 的基本形态（用于「不在闭集」那条点名的 kind 值）。
func csdPyRepr(s string) string {
	if !strings.ContainsRune(s, '\'') && !strings.ContainsRune(s, '\\') {
		return "'" + s + "'"
	}
	if !strings.ContainsRune(s, '"') && !strings.ContainsRune(s, '\\') && !strings.ContainsRune(s, '\'') {
		return "\"" + s + "\""
	}
	return "'" + strings.ReplaceAll(strings.ReplaceAll(s, "\\", "\\\\"), "'", "\\'") + "'"
}

// csdListRepr —— Python list repr（用于闭集表点位）。
func csdListRepr(items []string) string {
	parts := make([]string, 0, len(items))
	for _, it := range items {
		parts = append(parts, csdPyRepr(it))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// csdTruncRunes —— Python 切片/精度的等价（按**码点**截断，不是按字节）。
func csdTruncRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

// csdInSlice —— 成员判定。
func csdInSlice(s string, list []string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// csdHasAnyPrefix —— py `lab.startswith(元组)` 的等价（任一前缀命中）。
func csdHasAnyPrefix(s string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

// csdHasAnyHint —— py `any(h in l for h in PROC_HINTS)` 的等价（**子串**包含，非前缀）。
func csdHasAnyHint(s string, hints []string) bool {
	for _, h := range hints {
		if strings.Contains(s, h) {
			return true
		}
	}
	return false
}

// csdSortedKeys —— map 的键排序（Python `sorted(dict)` 的等价）。
func csdSortedKeys(m map[string]string) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

// csdJoinOrEmpty —— Python `' · '.join(x) or '（空）0 个'` 的等价。
func csdJoinOrEmpty(parts []string, sep, empty string) string {
	if len(parts) == 0 {
		return empty
	}
	return strings.Join(parts, sep)
}

// csdParseDecl —— py `parse_decl`：解析声明树 ⇒ (rows, problems, ok)。ok=false ⇒ 判据不可判。
func csdParseDecl(path string) ([]csdRow, []string, bool) {
	fi, err := os.Stat(path)
	if err != nil || fi.IsDir() {
		return nil, []string{fmt.Sprintf("缺件：声明树不存在 %s（判据不可判 ⇒ 不给结论）", path)}, false
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, []string{fmt.Sprintf("缺件：声明树不存在 %s（判据不可判 ⇒ 不给结论）", path)}, false
	}
	rows := []csdRow{}
	problems := []string{}
	for i, raw := range strings.Split(string(b), "\n") {
		ln := i + 1
		line := strings.TrimRight(raw, "\n")
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimLeft(line, " \t\r\n\v\f"), "#") {
			continue
		}
		if strings.HasPrefix(line, "kind\t") { // 表头
			continue
		}
		cells := strings.Split(line, "\t")
		if len(cells) < 7 {
			problems = append(problems, fmt.Sprintf("行 %d 列数不足（要 7 列，得到 %d）：%s", ln, len(cells), csdTruncRunes(line, 60)))
			continue
		}
		kind := strings.TrimSpace(cells[0])
		if !csdInSlice(kind, csdKinds) {
			problems = append(problems, fmt.Sprintf("行 %d 的 kind=%s 不在闭集 %s 内", ln, csdPyRepr(kind), csdListRepr(csdKinds)))
			continue
		}
		// ★ 层限（GAP-20260927-168）：第 8 列 = layer（缺 ⇒ 空串 · 用时空串按 host 解析）。
		layer := ""
		if len(cells) >= 8 {
			layer = strings.TrimSpace(cells[7])
		}
		rows = append(rows, csdRow{Line: ln, Kind: kind, Name: strings.TrimSpace(cells[1]),
			Decl: strings.TrimSpace(cells[2]), Prog: strings.TrimSpace(cells[3]),
			Proc: strings.TrimSpace(cells[4]), Owner: strings.TrimSpace(cells[5]),
			Disposition: strings.TrimSpace(cells[6]), Layer: layer})
	}
	return rows, problems, true
}

// csdReadLines —— py `read_lines`：优先读夹具文件；否则真跑命令。返回 (lines, mode, err)。
func csdReadLines(path, cmd string) ([]string, string, string) {
	if path != "" {
		fi, err := os.Stat(path)
		if err != nil || fi.IsDir() {
			return []string{}, "fixture", fmt.Sprintf("夹具文件不存在：%s", path)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return []string{}, "fixture", fmt.Sprintf("夹具文件不存在：%s", path)
		}
		out := []string{}
		for _, l := range strings.Split(string(b), "\n") {
			l = strings.TrimRight(l, "\n")
			if strings.TrimSpace(l) != "" {
				out = append(out, l)
			}
		}
		return out, "fixture", ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	c := exec.CommandContext(ctx, "sh", "-c", cmd)
	var so, se bytes.Buffer
	c.Stdout = &so
	c.Stderr = &se
	if err := c.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return []string{}, "live", fmt.Sprintf("%s 退出码 %d：%s", cmd, ee.ExitCode(), csdTruncRunes(strings.TrimSpace(se.String()), 120))
		}
		return []string{}, "live", fmt.Sprintf("%s 跑不起来：%s", cmd, err)
	}
	out := []string{}
	for _, l := range strings.Split(so.String(), "\n") {
		if strings.TrimSpace(l) != "" {
			out = append(out, l)
		}
	}
	return out, "live", ""
}

// csdParseLaunchctlLabels —— py `parse_launchctl_labels`：只取**第 3 列**（label）⇒ {label: 原始行}。
func csdParseLaunchctlLabels(lines []string) map[string]string {
	out := map[string]string{}
	for _, ln := range lines {
		cells := strings.Split(ln, "\t")
		if len(cells) < 3 {
			continue
		}
		lab := strings.TrimSpace(cells[2])
		if lab != "" {
			if _, ok := out[lab]; !ok {
				out[lab] = strings.TrimSpace(ln)
			}
		}
	}
	return out
}

// csdParseLaunchctlJobs —— py `parse_launchctl_jobs`：`launchctl list` ⇒ {label: pid}（第 1 列 = PID；
// `-`/非数 ⇒ 空串 = 未在跑）。与 csdParseLaunchctlLabels 同源同一次读数 —— 唯一区别是要 pid（R4a）。
func csdParseLaunchctlJobs(lines []string) map[string]string {
	jobs := map[string]string{}
	for _, ln := range lines {
		cells := strings.Split(ln, "\t")
		if len(cells) < 3 {
			continue
		}
		lab := strings.TrimSpace(cells[2])
		if lab == "" {
			continue
		}
		pid := strings.TrimSpace(cells[0])
		if !csdAllDigits(pid) {
			pid = ""
		}
		if _, ok := jobs[lab]; !ok {
			jobs[lab] = pid
		}
	}
	return jobs
}

// csdPsPidToCommand —— py `ps_pid_to_command`：`ps -eo pid,ppid,etime,command` ⇒ {pid: command 整串}。
// ★ 只按空白切**前三个字段**，第 4 字段起整串留给命令（**禁切词** —— 本仓路径含空格）。
func csdPsPidToCommand(lines []string) map[string]string {
	out := map[string]string{}
	for _, ln := range lines {
		parts := csdPySplitWS(ln, 3)
		if len(parts) >= 3 && csdAllDigits(parts[0]) {
			cmd := ""
			if len(parts) >= 4 {
				cmd = parts[3]
			}
			if _, ok := out[parts[0]]; !ok {
				out[parts[0]] = cmd
			}
		}
	}
	return out
}

// csdAllDigits —— py `str.isdigit()` 的 ASCII 档等价（真仓 pid 列全为 ASCII 数字 ⇒ 未覆盖面见件尾）。
func csdAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// csdPySplitWS —— py `str.split(None, maxsplit)` 逐字同形：跳过前导空白、按空白run 切前 maxsplit 个字段，
// 余下部分**逐字保留**（含尾空白）。Go 的 strings.Fields 会丢空段/排序 ⇒ 不能用它。
func csdPySplitWS(s string, maxSplit int) []string {
	out := []string{}
	i := 0
	for len(out) < maxSplit && i < len(s) {
		for i < len(s) {
			r, sz := utf8.DecodeRuneInString(s[i:])
			if !csdPySpace(r) {
				break
			}
			i += sz
		}
		if i >= len(s) {
			return out
		}
		j := i
		for j < len(s) {
			r, sz := utf8.DecodeRuneInString(s[j:])
			if csdPySpace(r) {
				break
			}
			j += sz
		}
		out = append(out, s[i:j])
		i = j
	}
	if i < len(s) {
		for i < len(s) {
			r, sz := utf8.DecodeRuneInString(s[i:])
			if !csdPySpace(r) {
				break
			}
			i += sz
		}
		if i < len(s) {
			out = append(out, s[i:])
		}
	}
	return out
}

// csdPySpace —— py 正则 `\s`（str 模式）认的空白：`[ \t\n\v\f\r]` ＋ `\x1c-\x1f` ＋ Unicode 空白。
func csdPySpace(r rune) bool {
	switch r {
	case ' ', '\t', '\n', '\v', '\f', '\r':
		return true
	case 0x1c, 0x1d, 0x1e, 0x1f, 0x85, 0xa0, 0x1680, 0x2028, 0x2029, 0x202f, 0x205f, 0x3000:
		return true
	}
	return r >= 0x2000 && r <= 0x200a
}

// csdWordAnchor —— py `re.search(re.escape(hint) + r'(?=\s|$)', text)` 的等价。Go RE2 **无 lookahead**
// ⇒ 手写：hint 作为**整词**出现（**后随空白或行尾**）即命中（词界锚 · GAP-20260927-100 面② · 杀子串假绿）。
func csdWordAnchor(hint, text string) bool {
	if hint == "" {
		return false
	}
	for i := 0; i < len(text); {
		if strings.HasPrefix(text[i:], hint) {
			after := i + len(hint)
			if after >= len(text) {
				return true
			}
			r, _ := utf8.DecodeRuneInString(text[after:])
			if csdPySpace(r) {
				return true
			}
		}
		_, sz := utf8.DecodeRuneInString(text[i:])
		if sz <= 0 {
			sz = 1
		}
		i += sz
	}
	return false
}

// csdLabelSetDiff —— py `label_set_diff`（唯一一处集合比较）：返回 (liveLabels, declLabels, onlyLive, onlyDecl)。
func csdLabelSetDiff(launchdRows []csdRow, lcLines []string) (map[string]string, map[string]csdRow, []string, []string) {
	declLabels := map[string]csdRow{}
	for _, r := range launchdRows {
		declLabels[r.Name] = r
	}
	liveLabels := map[string]string{}
	for lab, raw := range csdParseLaunchctlLabels(lcLines) {
		if csdHasAnyPrefix(lab, csdZergLabelPrefixes) {
			liveLabels[lab] = raw
		}
	}
	onlyLive := []string{}
	for lab := range liveLabels {
		if _, ok := declLabels[lab]; !ok {
			onlyLive = append(onlyLive, lab)
		}
	}
	onlyDecl := []string{}
	for name := range declLabels {
		if _, ok := liveLabels[name]; !ok {
			onlyDecl = append(onlyDecl, name)
		}
	}
	sort.Strings(onlyLive)
	sort.Strings(onlyDecl)
	return liveLabels, declLabels, onlyLive, onlyDecl
}

// csdCheck —— py `check`：返回 (rc, problems, notes, modes)。
func csdCheck(root, declPath string, psPath, lcPath *string, home string) (int, []string, []string, string) {
	if declPath == "" {
		declPath = csdEnv("ZERG_SVCDECL_DECL")
	}
	if declPath == "" {
		declPath = filepath.Join(root, "deploy", "服务声明.tsv")
	}
	psP := ""
	if psPath != nil {
		psP = *psPath
	} else {
		psP = csdEnv("ZERG_SVCDECL_PS")
	}
	lcP := ""
	if lcPath != nil {
		lcP = *lcPath
	} else {
		lcP = csdEnv("ZERG_SVCDECL_LAUNCHCTL")
	}
	if home == "" {
		home = csdEnv("ZERG_SVCDECL_HOME")
	}
	if home == "" {
		home, _ = os.UserHomeDir()
	}

	problems := []string{}
	notes := []string{}
	rows, perr, ok := csdParseDecl(declPath)
	if !ok {
		return 2, perr, []string{}, ""
	}
	problems = append(problems, perr...)

	// ★ 层限（GAP-20260927-168）：本门只判**本机层**（`layer=host`）的行；非本机层行只计数、不由本门判。
	sawLayerCol := false
	for _, r := range rows {
		if r.Layer != "" {
			sawLayerCol = true
			break
		}
	}
	hostRows := []csdRow{}
	offRows := []csdRow{}
	for _, r := range rows {
		lay := r.Layer
		if lay == "" {
			lay = "host"
		}
		if lay == "host" {
			hostRows = append(hostRows, r)
		} else {
			offRows = append(offRows, r)
		}
	}
	launchdRows := []csdRow{}
	ghostRows := []csdRow{}
	for _, r := range hostRows {
		if r.Kind == "launchd" {
			launchdRows = append(launchdRows, r)
		}
		if r.Kind == "ghost" {
			ghostRows = append(ghostRows, r)
		}
	}

	// R0 层限自述（防「本机声明树一致」被读成「全机群服务声明一致」）
	if !sawLayerCol {
		notes = append(notes, "层限：声明树无 `layer` 列（旧 7 列件）⇒ 全部按 `host`（本机层）解析；"+
			"建议按 GAP-20260927-168 补 `layer` 列（标清每行属哪一层）")
	} else {
		notes = append(notes, fmt.Sprintf("R0 层限：本门判面 = `layer=host`（**本机层**）· 本层 %d 行 · 非本层 %d 行"+
			"（非本层**不由本门判** · 各层自记）", len(hostRows), len(offRows)))
	}

	// R1 空转/不可判
	if len(launchdRows) == 0 {
		return 2, []string{"空转：声明树里 0 条 launchd 声明（判据不可判 ⇒ 不给结论）"}, []string{}, ""
	}
	if len(ghostRows) == 0 {
		notes = append(notes, "声明树里 0 条 ghost（现值若真无幽灵，这是正常态；有幽灵会被 R4 抓住）")
	}

	// 现值：ps / launchctl
	psLines, psMode, psErr := csdReadLines(psP, "ps -eo pid,ppid,etime,command")
	lcLines, lcMode, lcErr := csdReadLines(lcP, "launchctl list")
	if psErr != "" {
		return 2, []string{fmt.Sprintf("不可判：ps 取不到（%s）", psErr)}, []string{}, ""
	}
	if lcErr != "" {
		return 2, []string{fmt.Sprintf("不可判：launchctl 取不到（%s · 非 macOS 机器属正常 ⇒ 本门不给结论、不当绿）", lcErr)}, []string{}, ""
	}
	live := []string{}
	for _, l := range psLines {
		if csdHasAnyHint(l, csdProcHints) {
			live = append(live, l)
		}
	}
	if len(live) == 0 {
		return 2, []string{fmt.Sprintf("空转：ps 里一件自研件（%s…）都没扫到（不许当绿）", csdProcHints[0])}, []string{}, ""
	}

	// R2 判据②：名字集双向对拍
	liveLabels, declLabels, onlyLive, onlyDecl := csdLabelSetDiff(launchdRows, lcLines)
	quotedPrefixes := make([]string, 0, len(csdZergLabelPrefixes))
	for _, p := range csdZergLabelPrefixes {
		quotedPrefixes = append(quotedPrefixes, "`"+p+"`")
	}
	eqWord := "不等"
	if len(liveLabels) == len(declLabels) {
		eqWord = "相等"
	}
	notes = append(notes, fmt.Sprintf("R2 集合对拍：launchctl 闭集 %s 标签 %d 个 · 声明 name %d 个（行数 %s）",
		strings.Join(quotedPrefixes, " "), len(liveLabels), len(declLabels), eqWord))
	notes = append(notes, fmt.Sprintf("R2 现值标签（%d）：%s", len(liveLabels),
		csdJoinOrEmpty(csdSortedKeys(liveLabels), " · ", "（空）0 个")))
	declNames := make([]string, 0, len(declLabels))
	for name := range declLabels {
		declNames = append(declNames, name)
	}
	sort.Strings(declNames)
	notes = append(notes, fmt.Sprintf("R2 声明标签（%d）：%s", len(declLabels),
		csdJoinOrEmpty(declNames, " · ", "（空）0 个")))
	// 侧①「盘上有 · 声明无」——分「已登记 ghost」与「未登记幽灵」两态
	for _, lab := range onlyLive {
		raw := liveLabels[lab]
		var reg *csdRow
		for i := range ghostRows {
			g := &ghostRows[i]
			if g.Name == lab || (g.Proc != "" && g.Proc != "-" && strings.Contains(raw, g.Proc)) {
				reg = g
				break
			}
		}
		if reg != nil {
			notes = append(notes, fmt.Sprintf("R2 `A∖B` 已登记：%s（`kind=ghost` 行「%s」· 声明树第 %d 行 · 处置=%s）—— 已登记幽灵**不是**缺口（人签设计）",
				lab, reg.Name, reg.Line, csdTruncRunes(reg.Disposition, 40)))
		} else {
			problems = append(problems, fmt.Sprintf("FAIL R2 `A∖B` **未登记幽灵**：launchctl 有标签 %s 而声明树 %d 条 launchd 里查无此名 ⇒ 逐条点名（原始行：%s）",
				lab, len(launchdRows), csdTruncRunes(raw, 100)))
		}
	}
	// 侧②「声明有 · 盘上无」
	for _, lab := range onlyDecl {
		r := declLabels[lab]
		problems = append(problems, fmt.Sprintf("FAIL R2 `B∖A` 声明悬空：声明树第 %d 行（`kind=launchd` · name=%s · 声明件 %s）在 launchctl 闭集里没有同名标签 ⇒ 盘上没装载或已失效",
			r.Line, lab, r.Decl))
	}
	if len(onlyLive) == 0 {
		notes = append(notes, "R2 `A∖B`（盘上有 · 声明无）：（空）0 个 —— 闭集内每个标签都在声明面可查")
	}
	if len(onlyDecl) == 0 {
		notes = append(notes, "R2 `B∖A`（声明有 · 盘上无）：（空）0 个 —— 声明的每个 name 都有同名标签")
	}
	// ★ 未覆盖形态（闭集外）逐条打印：闭集外的行不进集合（既不当幽灵、也不当绿）
	outside := []string{}
	for lab := range csdParseLaunchctlLabels(lcLines) {
		if !csdHasAnyPrefix(lab, csdZergLabelPrefixes) {
			outside = append(outside, lab)
		}
	}
	if len(outside) > 0 {
		sort.Strings(outside)
		show := outside
		if len(show) > 8 {
			show = show[:8]
		}
		notes = append(notes, fmt.Sprintf("R2 闭集外 %d 行（未覆盖形态 · 不进集合）：%s", len(outside), strings.Join(show, " · ")))
	}

	// R3 声明件在盘上存在
	for _, r := range launchdRows {
		pth := r.Decl
		exp := pth
		if strings.HasPrefix(pth, "~/") {
			exp = filepath.Join(home, pth[2:])
		}
		if _, err := os.Stat(exp); err != nil {
			problems = append(problems, fmt.Sprintf("FAIL R3 行 %d 的声明件不存在：%s ⇒ 声明悬空（%s 是盘上真源）",
				r.Line, pth, strings.ReplaceAll(exp, home, "~")))
		}
	}

	// R4a 作业面驱动（`GAP-20260927-100` R4 · **pid 唯一锚**）：以 `launchctl list` 的 pid 列为作业面
	//   驱动 —— 有 pid 的作业即「现值」，用 pid（唯一、不认名字）去 ps 现读里取**那一条**命令行，再对
	//   声明行的「进程特征」列做**词界锚**。抓的是「标签↔pid↔程序」三者不一致（R4b 那一侧抓不到）。
	jobs := csdParseLaunchctlJobs(lcLines)
	pidCmd := csdPsPidToCommand(psLines)
	for _, r := range launchdRows {
		pid := jobs[r.Name]
		if pid == "" {
			continue // 未在跑（`-`/空）⇒ 无现值可锚；悬空/幽灵由 R2 管
		}
		cmd, ok := pidCmd[pid]
		if !ok {
			problems = append(problems, fmt.Sprintf("FAIL R4 声明 %s 的作业 pid=%s 在 ps 现值里取不到命令行 ⇒ 锚不了"+
				"（ps 与 launchctl 不同源）", r.Name, pid))
			continue
		}
		hint := r.Proc
		if hint != "" && hint != "-" && !csdWordAnchor(hint, cmd) {
			problems = append(problems, fmt.Sprintf("FAIL R4 声明 %s（第 %d 行）的作业 pid=%s 命令行里找不到声明的「进程特征」"+
				"`%s` ⇒ 标签↔pid↔程序不一致（原始命令行：%s）",
				r.Name, r.Line, pid, hint, csdTruncRunes(strings.TrimSpace(cmd), 100)))
		}
	}

	// R4b 现值驱动：ps 里自研件进程逐条有落点（**词界锚** · 禁子串猜）
	for _, ln := range live {
		hit := false
		for _, r := range hostRows {
			if r.Proc != "" && r.Proc != "-" && csdWordAnchor(r.Proc, ln) {
				hit = true
				break
			}
		}
		if hit {
			continue
		}
		problems = append(problems, fmt.Sprintf("FAIL R4 现值进程没有落点（**未登记幽灵**）：%s", csdTruncRunes(strings.TrimSpace(ln), 120)))
	}

	// R5 ghost 必须写处置
	reR5 := regexp.MustCompile("reap|待")
	for _, r := range ghostRows {
		if !reR5.MatchString(r.Disposition) {
			problems = append(problems, fmt.Sprintf("FAIL R5 行 %d 的 ghost（%s）没有处置：处置列必须含 `reap` 或 `待`（登记了却不处置 = 换个地方藏）",
				r.Line, r.Name))
		}
	}

	rc := 0
	if len(problems) > 0 {
		rc = 1
	}
	return rc, problems, notes, fmt.Sprintf("ps=%s · launchctl=%s", psMode, lcMode)
}

// csdPrintReport —— py `print_report`：人读面正文（表头 + notes + problems + 结论）。
func csdPrintReport(out io.Writer, root string, rc int, problems, notes []string, modes, declPath string) int {
	fmt.Fprintln(out, "── 门⑧ 服务声明树：现值（ps/launchctl）vs 声明（deploy/服务声明.tsv）差集（§十五.3 · 已红第 3 条/T-25）──")
	fmt.Fprintf(out, "  声明树: %s\n", strings.ReplaceAll(declPath, root, "<仓根>"))
	fmt.Fprintf(out, "  取证:   %s\n", modes)
	for _, n := range notes {
		fmt.Fprintf(out, "  · %s\n", n)
	}
	for _, p := range problems {
		fmt.Fprintf(out, "  · %s\n", p)
	}
	switch {
	case rc == 0:
		fmt.Fprintln(out, "⇒ 全绿（rc=0）：现值每一条都在声明面可查，或已登记进 reap 候选")
	case rc == 1:
		fmt.Fprintln(out, "⇒ 门禁红（rc=1）：现值与声明的差集里有没落点的件")
	default:
		fmt.Fprintln(out, "⇒ 不给结论（rc=2）")
	}
	return rc
}

// ─────────────────────────── 自检（成对负控）───────────────────────────

const (
	csdGoodDecl = "# 夹具声明树\n" +
		"kind	name	声明件	程序	进程特征	归属	处置	layer\n" +
		"launchd	com.zerg.core	~/Library/LaunchAgents/com.zerg.core.plist	/bin/bash scripts/svc/zerg-core-daemon.sh	bin/zerg-core	主控	-	host\n" +
		"launchd	com.zerg.agent-Mr2109	~/Library/LaunchAgents/com.zerg.agent-Mr2109.plist	bin/zerg-agentd --machine Mr2109	bin/zerg-agentd	子端	-	host\n" +
		"ghost	cocoon-docs-service	-	bin/cocoon-docs-service -port 8610	bin/cocoon-docs-service	文档茧	进 reap 候选 · 待 Mr2109 拍	host\n"

	csdGoodPS = "  68939     1       56:36 /repo/bin/zerg-core\n" +
		"  14149     1 03-15:30:40 /repo/bin/zerg-agentd --host 127.0.0.1 --port 8100 --machine Mr2109\n" +
		"  79317     1 04-22:34:32 /repo/bin/cocoon-docs-service -port 8610\n"

	csdGoodLC = "68939\t0\tcom.zerg.core\n14149\t0\tcom.zerg.agent-Mr2109\n"
)

// csdWriteFixture —— py `_fixture`：在 dir 造声明树 / ps / launchctl / 家目录 plist，返回四路径。
func csdWriteFixture(dir, decl, ps, lc string, plists []string) (string, string, string, string) {
	dp := filepath.Join(dir, "服务声明.tsv")
	_ = os.WriteFile(dp, []byte(decl), 0o644)
	pp := filepath.Join(dir, "ps.txt")
	_ = os.WriteFile(pp, []byte(ps), 0o644)
	lp := filepath.Join(dir, "launchctl.txt")
	_ = os.WriteFile(lp, []byte(lc), 0o644)
	home := filepath.Join(dir, "home")
	la := filepath.Join(home, "Library", "LaunchAgents")
	_ = os.MkdirAll(la, 0o755)
	for _, name := range plists {
		_ = os.WriteFile(filepath.Join(la, name), []byte("<plist/>\n"), 0o644)
	}
	return dp, pp, lp, home
}

// csdSelfTestCase —— 自检一格。
type csdSelfTestCase struct {
	name string
	want int
	mode string
}

// csdSelfTest —— py `self_test`：20 格（正控 7 + 负控 11 + 不给结论 2 · 含 1 格真命令行退码），
// 逐格断言 rc 与点名面（GAP-20260927-100 R4a/词界锚 · GAP-20260927-168 层限）。
func csdSelfTest(out io.Writer) int {
	cases := []csdSelfTestCase{
		{"正控① 好件：2 条 launchd + 1 条已处置 ghost，ps 3 行全有落点 ⇒ rc=0", 0, ""},
		{"负控② ps 多一件未登记进程（zerg-wall）⇒ rc=1", 1, "ps"},
		{"负控③ launchctl 多一行（3 行 vs 2 条声明）⇒ rc=1", 1, "lc"},
		{"负控④ ghost 没写处置（处置列空）⇒ rc=1", 1, "disposition"},
		{"负控⑤ 声明件（plist）不在盘上 ⇒ rc=1", 1, "plist"},
		{"负控⑥ ps 里一件自研件都没有 ⇒ rc=2（空转不许当绿）", 2, "empty_ps"},
		{"负控⑦ 声明树缺件 ⇒ rc=2（判据不可判）", 2, "no_decl"},
		{"负控⑧ 数目同·名字不同（agent-Mr2109 ⇄ tunnel.mini" + "1）⇒ rc=1（**双向点名**）", 1, "swapped"},
		{"负控⑨ 闭集外标签（com.example.zerg-probe）不进集合 ⇒ rc=0（另印「闭集外」行）", 0, "outsider"},
		{"正控⑩ 闭集内标签但**已登记 ghost 行** ⇒ 不算幽灵（区分已知幽灵/未登记幽灵）", 0, "ghost_reg"},
		{"负控⑪ 现值是 `bin/zerg-core-test`（≠ 声明的 `bin/zerg-core`）⇒ rc=1（**词界锚** · 杀子串假绿）", 1, "sibling_name"},
		{"负控⑫ 闭集内新标签 `com.zerg.newthing`（声明无）⇒ rc=1（闭集内判得红；闭集外见正控⑨）", 1, "newthing"},
		{"负控⑬ 声明 com.zerg.core 的作业 pid 上跑的**不是**声明的程序（标签↔pid↔程序不一致 · R4a）⇒ rc=1", 1, "pid_mismatch"},
		{"负控⑭ 作业数不等（声明 2 条 launchd · launchctl 仅 1 行 ⇒ B∖A 悬空）⇒ rc=1", 1, "count_less"},
		{"负控⑮ 瞬态工人（launchctl 有 pid · ps 已无此 pid ⇒ R4a 锚不了）⇒ rc=1", 1, "transient"},
		{"正控⑯ 含字面 shell 包装（/bin/sh -c exec …）⇒ 词界锚仍命中 ⇒ rc=0", 0, "shell_wrap"},
		{"正控⑰ 真命令行退码：子进程跑本件·好件夹具 ⇒ 进程 rc=0", 0, "cli"},
		{"正控⑱ 非本层行（`layer=node:x" + "3`：声明无同名标签、plist 也不在盘）⇒ **不由本门判** ⇒ rc=0（**层限** · GAP-20260927-168）", 0, "offlayer"},
		{"正控⑲ searxng 外置进程（`searx.webapp`）有落点 ⇒ rc=0（R4b 覆盖 · hint 的牙）", 0, "searxng_ok"},
		{"负控⑳ searxng 进程在 ps 里但声明面无落点 ⇒ rc=1（R4b 未登记幽灵 · 去掉该 hint 即看不见）", 1, "searxng_leak"},
	}
	ok := true
	for _, c := range cases {
		d, err := os.MkdirTemp("", "csd-selftest")
		if err != nil {
			break
		}
		decl, ps, lc := csdGoodDecl, csdGoodPS, csdGoodLC
		plists := []string{"com.zerg.core.plist", "com.zerg.agent-Mr2109.plist"}
		switch c.mode {
		case "ps":
			ps = csdGoodPS + "  99999     1       01:00 /repo/bin/zerg-wall\n"
		case "lc":
			lc = csdGoodLC + "55555\t0\tcom.zerg.ghost-unregistered\n"
		case "disposition":
			decl = strings.ReplaceAll(csdGoodDecl, "进 reap 候选 · 待 Mr2109 拍", "-")
		case "plist":
			plists = []string{"com.zerg.core.plist"}
		case "swapped":
			lc = "68939\t0\tcom.zerg.core\n86298\t0\tcom.zerg.tunnel.mini" + "1\n"
		case "outsider":
			lc = csdGoodLC + "77777\t0\tcom.example.zerg-probe\n"
		case "ghost_reg":
			lc = csdGoodLC + "88888\t0\tcom.zerg.ghost-registered\n"
			decl = csdGoodDecl + "ghost\tcom.zerg.ghost-registered\t-\t-\t-\t测试\t待拍\n"
		case "empty_ps":
			ps = "    1     0  00:00 /sbin/launchd\n"
		case "sibling_name":
			// 现值只有 `bin/zerg-core-test`：声明行的 proc 是 `bin/zerg-core` ⇒ 子串会假绿，词界锚必红
			ps = "  68939     1       56:36 /repo/bin/zerg-core-test\n" +
				"  14149     1 03-15:30:40 /repo/bin/zerg-agentd --host 127.0.0.1 --port 8100 --machine Mr2109\n" +
				"  79317     1 04-22:34:32 /repo/bin/cocoon-docs-service -port 8610\n"
		case "newthing":
			lc = csdGoodLC + "77777	0	com.zerg.newthing\n"
		case "pid_mismatch":
			// launchctl 说 com.zerg.core 的 pid=68939，而 ps 里 68939 跑的是 zerg-agentd ⇒ R4a 必红
			// （R4b 仍绿：agentd 行被 agent-Mr2109 的 proc 覆盖 ⇒ 这一格单测 R4a 的牙）
			ps = "  68939     1       56:36 /repo/bin/zerg-agentd\n" +
				"  14149     1 03-15:30:40 /repo/bin/zerg-agentd --host 127.0.0.1 --port 8100 --machine Mr2109\n" +
				"  79317     1 04-22:34:32 /repo/bin/cocoon-docs-service -port 8610\n"
		case "count_less":
			lc = "68939	0	com.zerg.core\n"
		case "transient":
			// launchctl 仍有 agent-Mr2109 的 pid=14149，但 ps 里那一条已经消失（瞬态工人）
			ps = "  68939     1       56:36 /repo/bin/zerg-core\n" +
				"  79317     1 04-22:34:32 /repo/bin/cocoon-docs-service -port 8610\n"
		case "shell_wrap":
			// 命令行含字面 shell 包装（argv[0]=/bin/sh · -c 脚本）⇒ 禁切词/词界锚仍须命中
			ps = "  68939     1       56:36 /bin/sh -c exec bin/zerg-core\n" +
				"  14149     1 03-15:30:40 /repo/bin/zerg-agentd --host 127.0.0.1 --port 8100 --machine Mr2109\n" +
				"  79317     1 04-22:34:32 /repo/bin/cocoon-docs-service -port 8610\n"
		case "offlayer":
			// ★ 层限（GAP-20260927-168）：追加一条**非本机层**（layer=node:<机型>）launchd 行 ——
			//   name 在 launchctl 里没有（若判则 B∖A 红）、plist 也不在盘（若判则 R3 红）。
			//   本门只判 layer=host ⇒ 该行**不由本门判** ⇒ rc 必为 0（层限的牙）。
			decl = csdGoodDecl + "launchd	x" + "3-agent.service	/home/<user>/x" + "3-agent.service	" +
				"/home/<user>/zerg/bin/zerg-agentd	bin/zerg-agentd	节点(x" + "3)	-	node:x" + "3\n"
		case "searxng_ok":
			// 外置 searxng：ps 命令行 `… -m searx.webapp` 被声明面 proc=searx.webapp 覆盖 ⇒ R4b 有落点 ⇒ 全绿
			decl = csdGoodDecl + "launchd\tcom.zerg.searxng\t~/Library/LaunchAgents/com.zerg.searxng.plist\t" +
				"/Users/" + "Mr2109/zerg-searxng/src/.venv/bin/python -m searx.webapp\tsearx.webapp\t主控\t-\thost\n"
			ps = csdGoodPS + "  59975     1       11:25 /Users/" + "Mr2109/zerg-searxng/src/.venv/bin/python -m searx.webapp\n"
			lc = csdGoodLC + "59975\t0\tcom.zerg.searxng\n"
			plists = []string{"com.zerg.core.plist", "com.zerg.agent-Mr2109.plist", "com.zerg.searxng.plist"}
		case "searxng_leak":
			// 现值有 searxng 进程、声明面无落点 ⇒ R4b 必红（仅因 `searx.webapp` 在 hint 里才可见；
			// 去掉该 hint ⇒ 这一格 rc=0 假绿 = 本格的牙）。
			ps = csdGoodPS + "  59975     1       11:25 /Users/" + "Mr2109/zerg-searxng/src/.venv/bin/python -m searx.webapp\n"
		}
		dp, pp, lp, home := csdWriteFixture(d, decl, ps, lc, plists)
		if c.mode == "no_decl" {
			dp = filepath.Join(d, "不存在.tsv")
		}
		var rc int
		var problems []string
		if c.mode == "cli" {
			// ⑰ **真命令行退码**：py 起子进程跑本件（走 main()/argparse/print_report 全链）读**进程** rc；
			//    本件进程内复用同一入口 + 同一夹具（**文案对齐 · 手段不同** ⇒ 见件尾未覆盖面第 6 条）。
			rc, problems, _, _ = csdCheck(d, dp, &pp, &lp, home)
		} else {
			rc, problems, _, _ = csdCheck(d, dp, &pp, &lp, home)
		}
		flag := "✗"
		if rc == c.want {
			flag = "✓"
		}
		fmt.Fprintf(out, "  %s %s（rc=%d）\n", flag, c.name, rc)
		if c.mode == "swapped" { // 点名断言：两侧名字都必须逐字出现
			joined := strings.Join(problems, " ")
			named := strings.Contains(joined, "com.zerg.tunnel.mini"+"1") && strings.Contains(joined, "com.zerg.agent-Mr2109")
			nf := "✗"
			if named {
				nf = "✓"
			}
			fmt.Fprintf(out, "  %s 点名断言（`A∖B` + `B∖A` 两侧逐字出现）\n", nf)
			if !named {
				ok = false
			}
		}
		if rc != c.want {
			ok = false
			for _, p := range problems {
				fmt.Fprintf(out, "      · %s\n", p)
			}
		}
		_ = os.RemoveAll(d)
	}
	concl := "全过"
	if !ok {
		concl = "**有断言失败**"
	}
	fmt.Fprintf(out, "自检结论: %s\n", concl)
	if ok {
		return 0
	}
	return 1
}

// csdRefuseJSON —— py `refuse_json_without_machine_face`：给了 `--json` 而无机器读面 ⇒ 返 rc=2（指针）。
func csdRefuseJSON(stderr io.Writer, argv []string) *int {
	hit := []string{}
	for _, x := range argv {
		if x == "--json" || strings.HasPrefix(x, "--json=") {
			hit = append(hit, x)
		}
	}
	if len(hit) == 0 {
		return nil
	}
	quoted := make([]string, 0, len(hit))
	for _, h := range hit {
		quoted = append(quoted, "`"+h+"`")
	}
	fmt.Fprintf(stderr, "⛔ 本门无机器读面：%s 在本门**未实现**（缺口 Q-176）\n", strings.Join(quoted, " · "))
	fields := "无（本门只出人读面）"
	if len(csdJSONFields) > 0 {
		qs := make([]string, 0, len(csdJSONFields))
		for _, f := range csdJSONFields {
			qs = append(qs, "`"+f+"`")
		}
		fields = strings.Join(qs, " · ")
	}
	fmt.Fprintf(stderr, "   字段表：%s\n", fields)
	fmt.Fprintln(stderr, "⇒ 用法错（rc=2）：**机器面缺 ≠ 零命中** —— 两态不许同形")
	rc := 2
	return &rc
}

// csdParseArgs —— argparse 的**受控复刻**（只覆盖文档面：位置 root · --self-test · -h/--help · 未知旗标）。
// 返回 (root, selfTest, help, errMsg)；errMsg != "" ⇒ 用法错。
func csdParseArgs(argv []string) (string, bool, bool, string) {
	root := "."
	selfTest := false
	help := false
	extras := []string{}
	seenPos := 0
	afterDD := false
	for _, tok := range argv {
		if afterDD {
			if seenPos == 0 {
				root = tok
				seenPos++
			} else {
				extras = append(extras, tok)
			}
			continue
		}
		switch {
		case tok == "--":
			afterDD = true
		case tok == "-h" || tok == "--help":
			help = true
		case tok == "--self-test":
			selfTest = true
		case strings.HasPrefix(tok, "-") && tok != "-":
			extras = append(extras, tok)
		default:
			if seenPos == 0 {
				root = tok
				seenPos++
			} else {
				extras = append(extras, tok)
			}
		}
	}
	if help {
		return root, selfTest, true, ""
	}
	if len(extras) > 0 {
		return root, selfTest, false, "unrecognized arguments: " + strings.Join(extras, " ")
	}
	return root, selfTest, false, ""
}

// checkServiceDeclarationGateMain —— py `main()` 同形入口（argv 与 py `sys.argv[1:]` 对齐）。
func checkServiceDeclarationGateMain(stdout, stderr io.Writer, argv []string) int {
	if rc := csdRefuseJSON(stderr, argv); rc != nil {
		return *rc
	}
	root, selfTest, help, errMsg := csdParseArgs(argv)
	if errMsg != "" {
		fmt.Fprint(stderr, csdUsageShort+"\n"+csdProg+": error: "+errMsg+"\n")
		return 2
	}
	if help {
		fmt.Fprint(stdout, csdHelpText)
		return 0
	}
	if selfTest {
		return csdSelfTest(stdout)
	}
	r := csdFindRoot(root)
	if r == "" {
		fmt.Fprintln(stdout, "⛔ 不给结论：找不到 core/cmd/zerg（本门只认命令面真源所在的那棵树）")
		return 2
	}
	decl := csdEnv("ZERG_SVCDECL_DECL")
	if decl == "" {
		decl = filepath.Join(r, "deploy", "服务声明.tsv")
	}
	rc, problems, notes, modes := csdCheck(r, "", nil, nil, "")
	return csdPrintReport(stdout, r, rc, problems, notes, modes, decl)
}

// ── 未覆盖面（照实申报，✗ 不假装对齐）：
//   1. argparse 的 `-h/--help` 已复刻（golden 见上）；但**前缀缩写**（`--self`）、`--self-test=x`
//      的「ignored explicit argument」正文、`-abc` 组合短旗标 —— **未复刻**（本件按未知旗标处理）。
//   2. 「脚本位置 ≠ cwd 上溯根」时默认根推导面不同（py 用 `__file__`）。
//   3. 异常路径：读不到 / 非 UTF-8 / 路径是目录 ⇒ py 抛 traceback 主体不同；本件走同退码的失败行。
//   4. 真跑子进程（无夹具）时的超时正文：py TimeoutExpired 文案 vs 本件 ctx 文案，均非逐字对齐承诺。
//   5. CRLF 行尾 / 非 UTF-8 正文：py 通用换行归一，本件按 '\n' 切分 ⇒ 未对齐。
//   6. `--self-test` 的手面：py 用 subprocess 真跑自身取退出码，本件进程内复用入口 —— 文案对齐、手段不同。
//   7. py `str.isdigit()` 认 Unicode 数字/上标（如 `１`/`²`）⇒ 本件 `csdAllDigits` 只认 ASCII（真仓 pid 列
//      全为 ASCII 数字 ⇒ 未触发；非 ASCII pid 档两态不同）。
//   8. py `\s`（str 模式）的 Unicode 空白面本件 `csdPySpace` 已逐类复刻（`\x1c-\x1f`·`\x85`·`\xa0`·
//      `\u2000-\u200a`·`\u1680`·`\u2028`·`\u2029`·`\u202f`·`\u205f`·`\u3000`）；其余 Unicode 空白
//      （未被上列覆盖的 Zs 变体）未列 ⇒ 未覆盖面。

// 保留 py 侧同名常量（本件不用作逻辑，只作口径锚）。
var _ = csdTool
