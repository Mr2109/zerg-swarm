// swap_ui.go —— 桌面 UI 换件口（`zerg-swap-ui.sh`）的 **CLI 正门**（`zerg swap ui`）· ★ 2026-10-08 **真搬**。
//
// 依据：`~/.zerg/state/A档18件-收CLI设计稿-20261007.md` 批 1 · A3
//
//	（`scripts/build/zerg-swap-ui.sh` · 211 行 · 现属面＝构建面/换件面 · 归族 = 新族 `swap`）。
//
// ★ 2026-10-08 **真搬**（本单）：底层行为**收进本件**（✗ 不再 exec `scripts/build/zerg-swap-ui.sh`）——
// 把该脚本的**实际行为**在 Go 内**等价复现**：参数解析（裸跑 / `--check` / `--help` / `-h` /
// `--swap` / 未知）· 用法文案 · 退码（0 / 1 / 2）· 两道前置闸（闸 A 工作树半成品 / 闸 B 启动链可用）·
// 四道门禁（① 构建口径身份四格 / ② 目标件验签 / ③ 回滚位在位且验签 / ④ 自证判据：进程活着 ∧ 映射到盘上件）·
// 逐闸判决与状态计数 · 日志双写（`say`）。
//
// 对外可观察面（rc/stdout/stderr）与脚本直跑**逐字同**；**不再依赖脚本在盘** —— 脚本退役（移出仓）后
// 两态三面逐字同（✗ 不再退 8；旧壳转账「递不到脚本 ⇒ 8」那档已消失 —— 行为已内化）。
//
// ★ **写面定性（现读）**：底层脚本是**只报告档** —— 它只有读动作（工作树读数 / 验签 / 进程 inode 对拍 /
// 摘要），`--swap` 一律 rc=2 且**零副作用**，全件**无任何写盘步骤**。⇒ 本件不新增任何写盘点。
// 命令树的**授权门**仍照现役口径保留（`swap` 族 D3 · `GAP-20260927-140` ①）：裸跑 / `--swap` /
// `--dry-run` / `--yes` 走本层 fail-closed 授权门 —— `--dry-run` 出计划件（零副作用）；缺 `--yes`
// 退 2 零写；只读档（`--check` / `--help` / `-h`）原样进入本件的报告/用法档、不进门。★ 此门**不是本件
// 引入**：现役 `cli-matrix.json` 的 must-fail 格（`swap/ui#缺 --yes（D3）` 与
// `swap/ui#--dry-run 与 --yes 同给`）早已钉住该口径，本单不动它（改它＝改对外可观察行为）。
//
// ★ 不可等价复现的步骤 ⇒ 保留**薄壳**（同一个 leaf 件/系统工具 · 逐条在此声明）：
//
//	· `say`（`echo "$@" | tee -a "$LOG"`）⇒ Go **双写** stdout ＋ 追加 LOG；
//	· 工作树读数 `git status --porcelain ui/ wall/ core/ agent/` ⇒ **薄壳**（同一个 git · 同参 · 同 cwd）；
//	· 验签取数/验签 `codesign -dv --verbose=2` / `codesign --verify` ⇒ **薄壳**（同一系统工具）；
//	· 进程读数 `pgrep -f` / `lsof -p` ⇒ **薄壳**（OS 原生面；POSIX 进程表不重写）；
//	· 身份四格（脚本内嵌 `python3` ＋ `json`）⇒ **薄壳**（内嵌**同一段** python ⇒ 逐字同）；
//	· 件摘要/inode（`shasum -a 256` / `stat -f %i`）⇒ Go stdlib（`crypto/sha256` / `os.Lstat`）——
//	  与 `family_swap_core.go` 同一条口径（该族把这些归 stdlib）；`os.Lstat` 与 BSD `stat -f %i` 同为
//	  **不跟随符号链接**的读法（现读坐实：符号链接两读法给出不同 inode）。
//
// ★ 两处**如实声明**（✗ 不改对外可观察行为）：
//
//	① 脚本的 `TS`（`date +%Y%m%dT%H%M%S`）与缺省 `LOG`（`/tmp/zerg-swap-ui-$TS.log`）逐秒取值 ⇒
//	   Go 取 `time.Now()` 同格式；**同一秒内两态逐字同**，跨秒则头部时间戳不同（非确定面，测试里归一）。
//	② 脚本 `judge` 遇未知判决词当场退 2 ⇒ Go 同（内部错误支 · 逐字照搬）。
//
// 三条口径（照仓里既有的真搬族写 —— `family_swap_core.go` / `ui_start.go`，不自己创式样）：
//
//	① **逐字照底层**：文案 / 退码一字不改（0 全通过 · 1 有闸 FAIL · 2 用法错/不给结论/真换件档未开放 ·
//	   3 保留号，只报告档跑不到）。
//	② **只留一道仓根解析失败的 fail-closed**（退码 8 · 与 `ui`/`swap` 族同档）。
//	③ **不新增机器面**：底层脚本没有 `--json` ⇒ 本件也不自己拼一份包封。
package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// swapUIScriptRel —— 桌面 UI 换件脚本的仓内路径。★ 真搬后本件**不再 exec 它**（行为已内化）——
// 保留常量只为 ① 报面/诊断指向人读件名 ② 测试用它定位「毒壳」落点（证「不再 exec」这条牙）。
const swapUIScriptRel = "scripts/build/zerg-swap-ui.sh"

// swuiHR —— 脚本 `hr()` 的那条横线（46 × U+2500）。
var swuiHR = strings.Repeat("\u2500", 46)

// swuiReportFn —— **报告流程**的函数级接缝（★ 安全线：单测**绝不**真跑六闸 ⇒ 测试替换本变量为桩）。
// 与 `family_swap_core.go` 的 `swcFlowFn` 同一条纪律：缝**只在 Go 里**（✗ 不从环境变量取命令串）。
var swuiReportFn = func(c *swuiCtx) int { return c.runReport() }

// swuiIdentityAssertPy —— 脚本闸① 的第一段内嵌 python（逐字 · 只取退码）。
const swuiIdentityAssertPy = `import json, sys
d = json.load(open(sys.argv[1]))
assert d.get("version") and d.get("commit") and d.get("build_time") and d.get("builder_host")
`

// swuiIdentityPy —— 脚本闸① 的第二段内嵌 python（逐字 · 出身份四格串）。
const swuiIdentityPy = `import json,sys;d=json.load(open(sys.argv[1]));print("%s · %s · %s · %s"%(d["version"],d["commit"],d["build_time"],d["builder_host"]))`

// swuiEnv —— 脚本顶部那几枚可覆盖点（默认即生产路径；`bash` 版先 `cd "$REPO_ROOT"` ⇒ 相对路径以仓根为基）。
type swuiEnv struct {
	root, bin, launcher, ts, log string
}

func swuiEnvOf(root string) swuiEnv {
	get := func(k, def string) string {
		if v := os.Getenv(k); v != "" {
			return v
		}
		return def
	}
	e := swuiEnv{root: root}
	e.ts = time.Now().Format("20060102T150405")
	e.bin = get("ZERG_SWAP_BIN", "bin/zerg-ui")
	e.launcher = get("ZERG_SWAP_LAUNCHER", "bin/zerg")
	e.log = get("ZERG_SWAP_LOG", "/tmp/zerg-swap-ui-"+e.ts+".log")
	return e
}

// ── 输出面（脚本 `say` = `echo "$@" | tee -a "$LOG"`）────────────────────────────────────────────

// swuiCtx —— 真搬执行上下文（env ＋ 两个输出面 ＋ 三档计数）。
type swuiCtx struct {
	e                    swuiEnv
	out                  io.Writer
	errw                 io.Writer
	nPass, nFail, nBlock int
}

// say 逐字复刻脚本 `say`：写 stdout，并把同一行**追加**到 LOG（`tee -a` 语义；LOG 写不进 ⇒ 只落 stdout）。
func (c *swuiCtx) say(s string) {
	fmt.Fprintln(c.out, s)
	c.appendLog(s + "\n")
}

// hr 逐字复刻脚本 `hr`。
func (c *swuiCtx) hr() { c.say(swuiHR) }

// teeText 逐字复刻脚本的 `… | tee -a "$LOG"`：把已渲染好的文本（含换行）**同时**写 stdout 与 LOG。
func (c *swuiCtx) teeText(s string) {
	if s == "" {
		return
	}
	fmt.Fprint(c.out, s)
	c.appendLog(s)
}

func (c *swuiCtx) appendLog(s string) {
	if c.e.log == "" {
		return
	}
	f, err := os.OpenFile(c.e.log, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	fmt.Fprint(f, s)
	_ = f.Close()
}

// judge 逐字复刻脚本 `judge`：计数 ＋ 打印 `  [闸] 名 ⇒ 判词 —— 依据`。
// 返回 internalErr=true 表示判决词不认识（脚本那支是当场退 2）。
func (c *swuiCtx) judge(name, verdict, why string) (internalErr bool) {
	switch verdict {
	case "PASS":
		c.nPass++
	case "FAIL":
		c.nFail++
	case "BLOCKED":
		c.nBlock++
	default:
		fmt.Fprintf(c.errw, "!! 内部错误：未知判决 %s\n", verdict)
		return true
	}
	c.say("  [闸] " + name + " ⇒ " + verdict + " —— " + why)
	return false
}

// ── 子进程小工具（薄壳：调用与脚本**同一个** leaf 件/系统工具）────────────────────────────────────

// swuiExecMerged 跑一条命令并取**合并**输出（脚本里的 `2>&1`）⇒ (out, rc)；起不动 ⇒ rc=-1。
func swuiExecMerged(dir, name string, args ...string) (string, int) {
	cmd := exec.Command(name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Stdin = strings.NewReader("")
	var buf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &buf, &buf
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return buf.String(), ee.ExitCode()
		}
		return buf.String(), -1
	}
	return buf.String(), 0
}

// swuiExecStdin 跑一条命令：**stdin** = 给定正文（复刻 `python3 - <<'PY'` 那支）· 输出合并取回 ⇒ (out, rc)。
func swuiExecStdin(stdin, dir, name string, args ...string) (string, int) {
	cmd := exec.Command(name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Stdin = strings.NewReader(stdin)
	var buf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &buf, &buf
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return buf.String(), ee.ExitCode()
		}
		return buf.String(), -1
	}
	return buf.String(), 0
}

// swuiExecStdout 只取 stdout（脚本里的 `… 2>/dev/null |` 形态）。
func swuiExecStdout(dir, name string, args ...string) string {
	cmd := exec.Command(name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Stdin = strings.NewReader("")
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = io.Discard
	_ = cmd.Run()
	return buf.String()
}

// ── 逐字助手 ──────────────────────────────────────────────────────────────────────────────────

// swuiAbs 把仓内相对路径按仓根展开（绝对路径原样 —— 脚本 `cd "$REPO_ROOT"` 后同形）。
func swuiAbs(root, p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(root, p)
}

// swuiExists 复刻 `[ -e <件> ]`（跟随符号链接）。
func swuiExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// swuiIsFile 复刻 `[ -f <件> ]`。
func swuiIsFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// swuiIsExec 复刻 `[ -x <件> ]`（有任何一位可执行位即真；目录同名同判）。
func swuiIsExec(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.Mode().Perm()&0o111 != 0
}

// swuiSHA256 复刻 `shasum -a 256 <件> | awk '{print $1}'`（读不到 ⇒ 空）。
func swuiSHA256(p string) string {
	b, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return fmt.Sprintf("%x", sum)
}

// swuiInode 复刻 `stat -f %i <件>`（BSD stat 缺省**不跟随**符号链接 ⇒ `os.Lstat`）。
func swuiInode(p string) string {
	fi, err := os.Lstat(p)
	if err != nil {
		return ""
	}
	if st, ok := fi.Sys().(*syscall.Stat_t); ok {
		return strconv.FormatUint(st.Ino, 10)
	}
	return ""
}

// swuiFirst16 复刻 `${VAR:0:16}`。
func swuiFirst16(s string) string {
	if len(s) > 16 {
		return s[:16]
	}
	return s
}

// swuiGrepTr 复刻 `grep -E 'A|B|C' | tr '\n' ' '`：命中行逐行后跟一个空格（尾行也带）。
func swuiGrepTr(text string, subs []string) string {
	var sb strings.Builder
	for _, ln := range strings.Split(text, "\n") {
		hit := false
		for _, s := range subs {
			if strings.Contains(ln, s) {
				hit = true
				break
			}
		}
		if hit {
			sb.WriteString(ln)
			sb.WriteString(" ")
		}
	}
	return sb.String()
}

// swuiSed 复刻 `sed 's/^/PREFIX /'`（逐行加前缀；末行无换行则输出也不补）。
func swuiSed(text, prefix string) string {
	if text == "" {
		return ""
	}
	hadNL := strings.HasSuffix(text, "\n")
	lines := strings.Split(text, "\n")
	if hadNL {
		lines = lines[:len(lines)-1]
	}
	var sb strings.Builder
	for i, ln := range lines {
		sb.WriteString(prefix)
		sb.WriteString(ln)
		if i < len(lines)-1 || hadNL {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

// ── 闸B 的 exec 目标推导（复刻脚本那行 grep -oE | tail -1 | sed）──────────────────────────────
//
// 脚本：`grep -oE 'uiStartBinRel[[:space:]]*=[[:space:]]*"[^"]+"' core/cmd/zerg/ui_start.go 2>/dev/null
//
//	| tail -1 | sed 's/^[^"]*"//; s/"$//'` —— 从 Go 真源常量取「启动器 exec 的件」。
//
// 解析不到 ⇒ 空串（脚本据此判 BLOCKED）。★ 保留「读真源文件」这一步（✗ 不直接内联常量值）：
// 与脚本同面、同 BLOCKED 支。
func swuiParseUIStartBinRel(root string) string {
	b, err := os.ReadFile(filepath.Join(root, "core", "cmd", "zerg", "ui_start.go"))
	if err != nil {
		return ""
	}
	re := regexp.MustCompile("uiStartBinRel[ \t]*=[ \t]*\"" + "[^\"]+\"")
	ms := re.FindAllString(string(b), -1)
	if len(ms) == 0 {
		return ""
	}
	last := ms[len(ms)-1]
	i := strings.Index(last, "\"")
	if i < 0 {
		return ""
	}
	return strings.TrimSuffix(last[i+1:], "\"")
}

// ── 工作树读数（薄壳 git；脚本闸A 那两发）─────────────────────────────────────────────────────

// swuiGitStatusLines 跑 `[git -c core.quotepath=false] status --porcelain ui/ wall/ core/ agent/`（stderr 丢弃）。
func (c *swuiCtx) swuiGitStatusLines(quotepath bool) []string {
	args := []string{"status", "--porcelain", "ui/", "wall/", "core/", "agent/"}
	if quotepath {
		args = append([]string{"-c", "core.quotepath=false"}, args...)
	}
	cmd := exec.Command("git", args...)
	cmd.Dir = c.e.root
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = io.Discard
	_ = cmd.Run()
	var out []string
	for _, ln := range strings.Split(buf.String(), "\n") {
		if ln != "" {
			out = append(out, ln)
		}
	}
	return out
}

// swuiNonUntracked 复刻 `grep -v '^?? '`。
func swuiNonUntracked(lines []string) []string {
	var out []string
	for _, ln := range lines {
		if strings.HasPrefix(ln, "?? ") {
			continue
		}
		out = append(out, ln)
	}
	return out
}

// pids 复刻 `pgrep -f "$BIN_REL" 2>/dev/null | tr '\n' ' ' | sed 's/ $//'`。
func (c *swuiCtx) pids() string {
	out := swuiExecStdout(c.e.root, "pgrep", "-f", c.e.bin)
	return strings.Join(strings.Fields(out), " ")
}

// lsofMapped 复刻 `lsof -p <pid> 2>/dev/null | awk -v p="$BIN_REL" 'index($NF,p)>0 {print $NF; exit}'`。
func (c *swuiCtx) lsofMapped(pid string) string {
	out := swuiExecStdout(c.e.root, "lsof", "-p", pid)
	for _, ln := range strings.Split(out, "\n") {
		fs := strings.Fields(ln)
		if len(fs) == 0 {
			continue
		}
		last := fs[len(fs)-1]
		if strings.Contains(last, c.e.bin) {
			return last
		}
	}
	return ""
}

// ── 报告正文（逐字复刻脚本主正文）────────────────────────────────────────────────────────────

// runReport 复刻脚本 ACTION=check 的正文：六闸逐条判 ＋ 状态计数 ＋ 退码（0/1/2）。
func (c *swuiCtx) runReport() int {
	c.say("== UI 换件闸（只报告 · 只读）" + c.e.ts + " ==")
	c.say("日志：" + c.e.log)
	c.say("目标件：" + c.e.bin + " · 启动链：" + c.e.launcher)

	// ── 闸A 工作树半成品 ──
	c.hr()
	c.say("[闸A] 工作树半成品（换件是从工作树编译的）")
	dirty := swuiNonUntracked(c.swuiGitStatusLines(false))
	if len(dirty) == 0 {
		if c.judge("A 工作树半成品", "PASS", "未提交改动数 = 0") {
			return exitUsage
		}
	} else {
		var sb strings.Builder
		for _, ln := range swuiNonUntracked(c.swuiGitStatusLines(true)) {
			sb.WriteString("       " + ln + "\n")
		}
		c.teeText(sb.String())
		if c.judge("A 工作树半成品", "FAIL",
			"未提交改动数 = "+strconv.Itoa(len(dirty))+"（换件会把半成品编进生产件）") {
			return exitUsage
		}
	}

	// ── 闸B 启动链可用 ──
	c.hr()
	c.say("[闸B] 启动链可用（A6 真源：正门 zerg ui start → 启动器在位 ∧ 它 exec 的件在位）")
	cliRel := "bin/zerg"
	if !swuiIsExec(swuiAbs(c.e.root, cliRel)) {
		if c.judge("B 启动链", "FAIL", "启动器不存在（"+c.e.launcher+"）—— 换上去也起不来") {
			return exitUsage
		}
	} else {
		execTarget := swuiParseUIStartBinRel(c.e.root)
		c.say("      启动器：" + c.e.launcher + " · 它 exec 的件：" + orX(execTarget, "（解析不到）"))
		if execTarget == "" {
			if c.judge("B 启动链", "BLOCKED", "启动器里解析不出 exec 目标 —— 判不了，不许当绿") {
				return exitUsage
			}
		} else if !swuiExists(swuiAbs(c.e.root, execTarget)) {
			if c.judge("B 启动链", "FAIL",
				"启动器 exec 的件盘上不存在（"+execTarget+"）—— 悬空，同族于主控件 2026-09-20 那次") {
				return exitUsage
			}
		} else {
			if c.judge("B 启动链", "PASS",
				"启动器（A6 真源 "+c.e.launcher+"）与它 exec 的件（"+execTarget+"）都在位") {
				return exitUsage
			}
		}
	}

	// ── 闸① 构建口径 ──
	c.hr()
	c.say("[闸①] 构建口径（件必须由 build-all.sh 产出 · 身份不许丢）")
	biRel := "bin/build-info.json"
	biAbs := swuiAbs(c.e.root, biRel)
	if !swuiIsFile(biAbs) {
		if c.judge("① 构建口径", "BLOCKED", "缺 "+biRel+"（读不到构件身份 ⇒ 判不了，不许当绿）") {
			return exitUsage
		}
	} else if _, rc := swuiExecStdin(swuiIdentityAssertPy, c.e.root, "python3", "-", biAbs); rc != 0 {
		if c.judge("① 构建口径", "BLOCKED",
			biRel+" 解析不出四格身份（version/commit/build_time/builder_host）") {
			return exitUsage
		}
	} else {
		ident, _ := swuiExecStdin("", c.e.root, "python3", "-c", swuiIdentityPy, biAbs)
		ident = strings.TrimRight(ident, "\n")
		if c.judge("① 构建口径", "PASS",
			"构件身份齐："+ident+"（造件入口 = scripts/build/build-all.sh）") {
			return exitUsage
		}
	}

	// ── 闸② 验签 ──
	c.hr()
	c.say("[闸②] 验签（codesign --verify —— 输出原样打印，不吞）")
	if !swuiExists(swuiAbs(c.e.root, c.e.bin)) {
		if c.judge("② 验签", "BLOCKED", "件不存在（"+c.e.bin+"）—— 判不了") {
			return exitUsage
		}
	} else {
		dv, _ := swuiExecMerged(c.e.root, "codesign", "-dv", "--verbose=2", c.e.bin)
		c.say("      " + swuiGrepTr(dv, []string{"Identifier", "Signature", "Format"}))
		vOut, vrc := swuiExecMerged(c.e.root, "codesign", "--verify", "--verbose=2", c.e.bin)
		c.teeText(swuiSed(vOut, "      verify: "))
		if vrc == 0 {
			if c.judge("② 验签", "PASS", c.e.bin+" 验签通过") {
				return exitUsage
			}
		} else {
			if c.judge("② 验签", "FAIL", c.e.bin+" 验签不过（UI 件签名失效会被秒杀 EXIT137）") {
				return exitUsage
			}
		}
	}

	// ── 闸③ 回滚位 ──
	c.hr()
	c.say("[闸③] 回滚位（回滚件必须**在位且验签通过**）")
	rbRel := "bin/_history/" + filepath.Base(c.e.bin) + ".prev"
	if !swuiExists(swuiAbs(c.e.root, rbRel)) {
		if c.judge("③ 回滚位", "BLOCKED",
			"无回滚件（"+rbRel+"）—— 换上去就退不回来 ⇒ 不给结论（**不许当绿**）") {
			return exitUsage
		}
	} else if _, rc := swuiExecMerged(c.e.root, "codesign", "--verify", "--verbose=1", rbRel); rc == 0 {
		if c.judge("③ 回滚位", "PASS", "回滚件在位且验签通过（"+rbRel+"）") {
			return exitUsage
		}
	} else {
		if c.judge("③ 回滚位", "FAIL", "回滚件在位但**验签不过**（"+rbRel+"）—— 2026-09-16 二次事故的形态") {
			return exitUsage
		}
	}

	// ── 闸④ 自证判据 ──
	c.hr()
	c.say("[闸④] 换件后自证判据可跑（UI 无 HTTP 端点 ⇒ 进程活着 ∧ 跑的件 == 盘上件）")
	if !swuiExists(swuiAbs(c.e.root, c.e.bin)) {
		if c.judge("④ 自证判据", "BLOCKED", "件不存在 ⇒ 无从对拍——判不了") {
			return exitUsage
		}
	} else {
		binAbs := swuiAbs(c.e.root, c.e.bin)
		c.say("      盘上件 inode=" + swuiInode(binAbs) + " sha256=" + swuiFirst16(swuiSHA256(binAbs)) + "…")
		pids := c.pids()
		if pids == "" {
			if c.judge("④ 自证判据", "BLOCKED",
				"UI 进程没在跑（pgrep 零命中）—— 判据本身活着，但此刻无从自证（**不是绿**）") {
				return exitUsage
			}
		} else {
			c.say("      在跑进程：" + pids)
			ok := true
			for _, pid := range strings.Fields(pids) {
				lof := c.lsofMapped(pid)
				if lof != c.e.bin && lof != filepath.Join(c.e.root, c.e.bin) {
					c.say("        pid " + pid + " 映射的件解析不出（lsof 读到：" +
						orX(lof, "（空）") + "）⇒ 本闸不给结论")
					ok = false
				}
			}
			if ok {
				if c.judge("④ 自证判据", "PASS",
					"进程在跑且映射到 "+c.e.bin+"（判据：进程活着 ∧ 跑的就是盘上那个件）") {
					return exitUsage
				}
			} else {
				if c.judge("④ 自证判据", "BLOCKED",
					"进程在跑但「它跑的是哪个件」解析不出 ⇒ 判不了，不许当绿") {
					return exitUsage
				}
			}
		}
	}

	// ── 状态计数 ＋ 退码 ──
	c.hr()
	c.say("状态计数：PASS " + strconv.Itoa(c.nPass) + " · FAIL " + strconv.Itoa(c.nFail) +
		" · BLOCKED(不给结论) " + strconv.Itoa(c.nBlock))
	c.say("日志：" + c.e.log)
	if c.nFail > 0 {
		c.say("⇒ 有闸判 FAIL（rc=1）—— 逐条见上")
		return exitFail
	}
	if c.nBlock > 0 {
		c.say("⇒ 有闸 BLOCKED（rc=2 · **不给结论 ⇒ 不许当绿**，与提交闸同码同义）")
		return exitUsage
	}
	c.say("⇒ 六闸全通过（rc=0 · 只读判定；本件**未换件、未重启、未改任何文件**）")
	return exitOK
}

// ── 用法（`--help` 原文即下面这段；**退码与档位的唯一定义处**）───────────────────────────────

// swuiUsage 逐字返回脚本 `usage()` 的正文（`cat <<'EOF' … EOF` ⇒ 行间 \n ＋ 末尾 \n）。
func swuiUsage() string {
	return strings.Join([]string{
		"zerg-swap-ui.sh —— 桌面 UI（zerg-ui）换件的受控流程 · **只报告档**（本轮未开放真换件）",
		"",
		"用法：",
		"  bash scripts/build/zerg-swap-ui.sh            只报告：逐闸出判决（**只读**，零副作用）",
		"  bash scripts/build/zerg-swap-ui.sh --check    同上（显式写法）",
		"  bash scripts/build/zerg-swap-ui.sh --help     本页",
		"  bash scripts/build/zerg-swap-ui.sh --swap     真换件 —— **本档未开放**，rc=2 且不执行任何动作",
		"",
		"判的六闸（三闸形态 + 两前置闸；逐条对应 scripts/build/zerg-swap-core.sh）：",
		"  闸 A 工作树半成品：`git status --porcelain ui/ wall/ core/ agent/` 的非未跟踪项计数必须 0",
		"  闸 B 启动链可用：A6 真源＝**CLI 面**（正门 `zerg ui start` · `bin/zerg` 在盘），**且**它启动的 `bin/zerg-ui` 真存在",
		"  闸① 构建口径：件必须由 scripts/build/build-all.sh 产出；读 bin/build-info.json 判身份四格",
		"  闸② 验签：目标件必须 `codesign --verify` 通过（UI 件验签失效 = 秒杀 EXIT137）",
		"  闸③ 回滚位：回滚件（bin/_history/zerg-ui.prev）必须**在位且验签通过**",
		"  闸④ 换件后自证判据可跑：UI **无 HTTP 端点** ⇒ 判据 = 进程活着 **且** 它当前跑的件与盘上件同 inode/sha256",
		"",
		"退码（唯一说法）：",
		"  0  全部闸 PASS（只读判定通过）",
		"  1  有闸判 FAIL（真红）",
		"  2  用法错 · 或**不给结论**（有闸 BLOCKED）· 或敲了 `--swap`（**真换件档未开放**，要开先拍）",
		"  3  **保留号**（回滚也失败 ⇒ 需人工）；只报告档跑不到它，占号是为与主控件逐号对齐",
	}, "\n") + "\n"
}

// ── 执行体（`zerg swap ui` 的真搬正门）─────────────────────────────────────────────────────────

// cmdSwapUI —— `zerg swap ui [--check | --help] [--dry-run | --yes]`（真搬正门）。
//
// 退码口径（**逐字照底层**）：0 六闸全通过 · 1 有闸 FAIL · 2 用法错/不给结论（有 BLOCKED）/真换件档未开放。
//
// ★ 授权门照现役口径（见件头「写面定性」）；本件自己只在「解析不到仓根」那一档退 8。
func cmdSwapUI(inv *invocation, stdout, stderr io.Writer) int {
	root := repoRoot()
	if root == "" {
		inv.setErr("blocked", "repo_root_absent", "解析不到仓根")
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 不给结论（退码 %d）\n", progName, exitBlocked)
		return exitBlocked
	}
	if !swapReadOnlyTier(inv) {
		if rc, done := writeGuardDryRun(inv, stdout, stderr); done {
			return rc
		}
		if rc := writeGuardYes(inv, stderr); rc != exitOK {
			return rc
		}
	}
	tail := swapForwardTail(inv)
	first := ""
	if len(tail) > 0 {
		first = tail[0]
	}
	switch first {
	case "", "--check":
		ctx := &swuiCtx{e: swuiEnvOf(root), out: stdout, errw: stderr}
		return swuiReportFn(ctx)
	case "--help", "-h":
		fmt.Fprint(stdout, swuiUsage())
		return exitOK
	case "--swap":
		fmt.Fprint(stderr, "✗ zerg-swap-ui.sh：**真换件档未开放**（P13 拍板：先工装 · 先只报告）\n")
		fmt.Fprint(stderr, "  ⇒ 零副作用退出（rc=2）。要开先拍；拍后本档补：备份 → build-all.sh 的 UI 档 → 验签 → 重启 → 自证 → 回滚路径。\n")
		return exitUsage
	default:
		fmt.Fprintf(stderr, "✗ 未知参数: %s（用法见：bash scripts/build/zerg-swap-ui.sh --help）\n", first)
		return exitUsage
	}
}
