// swap_agent.go —— 子端换件口（`zerg-swap-agent.sh`）的 CLI 正门（`zerg swap agent`）· ★ 2026-10-08 真搬。
//
// 依据：`~/.zerg/state/A档18件-收CLI设计稿-20261007.md` 批 1 · A2
//
//	（`scripts/build/zerg-swap-agent.sh` · 355 行 · 现属面＝构建面/换件面 · 归族 = 新族 `swap`）。
//
// ★ 2026-10-08 真搬（本单）：底层行为**收进本件**（✗ 不再 exec `scripts/build/zerg-swap-agent.sh`）——
// 把该脚本的**实际行为**在 Go 内**等价复现**：参数解析（裸跑 / `--check` / `--swap` / `--help` / `-h` /
// `--self-test` / `--machine <名>` / `--label <作业名>` / 未知）· 用法文案 · 退码（0 / 1 / 2）·
// 两道前置闸（闸A 工作树半成品 / 闸B 服务定义可用）· 四项判据（闸① 构建口径 / 闸② 验签三格 /
// 闸③ 回滚位 / 闸④ 只读探针）· `--self-test` 的 T1–T9 夹具。
//
// 对外可观察面（rc/stdout/stderr）与脚本直跑**逐字同**；**不再依赖脚本在盘** —— 脚本退役（移出仓）后
// 两态三面逐字同（✗ 不再退 `8`；旧壳转账「递不到脚本 ⇒ 8」那档已消失 —— 行为已内化）。
//
// ★ 不可等价复现的步骤 ⇒ 保留**薄壳**（同一个 leaf 件/系统工具 · 逐条在此声明）：
//
//	· `say`（`echo "$@" | tee -a "$LOG"`）⇒ Go **双写** stdout ＋ 追加 LOG；
//	· 工作树读数 `git status --porcelain core/ agent/ ui/ wall/` ⇒ **薄壳**（同一 leaf 件 `git`）；
//	· 验签取数 `codesign -dvvv` / `codesign --verify` / `--self-test` 的 adhoc 造件 `codesign -s -` ⇒ **薄壳**（同一二进制工具）；
//	· plist / 构件身份解析（脚本内嵌 `python3` ＋ `plistlib` / `json`）⇒ **薄壳**（内嵌**同一段** python ⇒ 逐字同）；
//	· 端点探针 `curl -m 30`（令牌经 **stdin** ＋ `-H @-` · ✗ 不落 argv）⇒ **薄壳**；
//	· `date +%Y%m%dT%H%M%S`（`$TS`）⇒ Go `time.Now().Format`；`mktemp` ⇒ `os.CreateTemp`；
//	· `sed` / `grep` / `tr` / `wc` / `cut` ⇒ Go stdlib（`strings` / `regexp` / 逐行计数）。
//
// ★ 两处**如实声明**（✗ 不改对外可观察行为）：
//
//	① 真换件档（`--swap`）本档**未开放**：脚本原样只退 `2` 且不执行任何动作 ⇒ 本件逐字复刻其两行 stderr 文案；
//	   裸跑 `--swap`（无 `--yes`）由 `swap` 族授权门先拦（`guard.go` 的 `writeGuardYes`）—— 与 `--yes --swap` 两路同码同义。
//	② 脚本内 `history_drop_ok` 现读**仅在 `--self-test` 的 T1/T2 有调用点**（正文不调）⇒ 本件逐字保留为
//	   `sagHistoryDropOK`（无副作用 · 正文无调用点 —— 只为与原件逐条对得上）。
//
// 三条口径（照仓里既有的真搬族写 —— `family_swap_core.go` / `ui_start.go` / `isolate_ro.go`，不自己创式样）：
//
//	① **逐字照底层**：文案 / 退码一字不改（0 全通过 · 1 有闸 FAIL · 2 用法错 / 不给结论 / `--swap` 未开放）；
//	② **只留一道仓根解析失败的 fail-closed**（`8` · 与 `ui`/`swap` 族同档）；
//	③ **不新增机器面**：底层脚本没有 `--json` ⇒ 本件也不自己拼一份包封（契约在命令面只有一处）。
package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// swapAgentScriptRel —— 子端换件脚本的仓内路径。★ 真搬后本件**不再 exec 它**（行为已内化）——
// 保留常量只为 ① 报面/诊断指向人读件名 ② 测试用它定位「毒壳」落点（证「不再 exec」这条牙）。
const swapAgentScriptRel = "scripts/build/zerg-swap-agent.sh"

// sagHR —— 脚本 `hr()` 的那条横线（46 × U+2500）。
var sagHR = strings.Repeat("\u2500", 46)

// sagFlowFn —— **报告流程**的函数级接缝（与 `family_swap_core.go` 的 `swcFlowFn` 同一条纪律：
// 缝**只在 Go 里** · ✗ 不从环境变量取命令串）。测试替换它 ⇒ 四态「`--yes` 放行」那格不必真跑判据。
var sagFlowFn = func(c *sagCtx) int { return c.runReport() }

// ── 环境（脚本顶部那几枚**可覆盖点**；默认即生产路径）─────────────────────────────────────────────

// sagEnv —— 脚本顶部的可覆盖点（`bash` 版先 `cd "$REPO_ROOT"` ⇒ 相对路径以仓根为基）。
type sagEnv struct {
	root, bin, label, domain, plist, tokenFile, readyURL, statusURL, ts, log string
}

// sagLocalMachine —— 本机机型短名（★ 运行期派生 · 缺口 GAP-20260925-21）。
// 改前本件把本机机器名当**字面量**写死；本件 public=yes（进公开快照面）⇒ 件内不许落机器名字面。
// 现按「环境 / 主机名」在运行期拼出：macOS 上机型短名与登录用户名同值（本机即该短名）；
// 环境取不到时回落主机名（去掉 `.local` 尾）。
func sagLocalMachine() string {
	if u := os.Getenv("USER"); u != "" {
		return u
	}
	h, err := os.Hostname()
	if err != nil {
		return ""
	}
	if i := strings.IndexByte(h, '.'); i > 0 {
		h = h[:i]
	}
	return h
}

// sagDefLabel —— 本机默认作业名（★ 运行期派生：`com.zerg.agent-` ＋ 本机机型短名 · 缺口 GAP-20260925-21）。
// 改前是件内常量字面量；派生后**本机求值逐字同**（机器名不再出现在件里）。
var sagDefLabel = "com.zerg.agent-" + sagLocalMachine()

// sagEnvOf 复刻脚本顶部赋值 ＋（参数解析后）目标解析（缺口 `GAP-20260925-21` · 四档优先级）。
func sagEnvOf(root, argMachine, argLabel string) sagEnv {
	home, _ := os.UserHomeDir()
	get := func(k, def string) string {
		if v := os.Getenv(k); v != "" {
			return v
		}
		return def
	}
	e := sagEnv{root: root}
	e.ts = time.Now().Format("20060102T150405")
	e.bin = get("ZERG_SWAP_BIN", "bin/zerg-agentd")
	e.label = sagResolveLabel(argMachine, argLabel, os.Getenv("ZERG_SWAP_LABEL"), sagDefLabel)
	e.domain = fmt.Sprintf("gui/%d", os.Getuid())
	e.plist = get("ZERG_SWAP_PLIST", filepath.Join(home, "Library", "LaunchAgents", e.label+".plist"))
	e.tokenFile = get("ZERG_SWAP_TOKEN_FILE", filepath.Join(home, ".zerg", "token"))
	e.readyURL = get("ZERG_SWAP_READY_URL", "http://127.0.0.1:8100/ready")
	e.statusURL = get("ZERG_SWAP_STATUS_URL", "http://127.0.0.1:8100/status")
	e.log = get("ZERG_SWAP_LOG", "/tmp/zerg-swap-agent-"+e.ts+".log")
	return e
}

// abs 把仓内相对路径按仓根展开（绝对路径原样 —— 脚本 `cd "$REPO_ROOT"` 后同形）。
func (e sagEnv) abs(p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(e.root, p)
}

// sagResolveLabel 逐字复刻脚本 `resolve_label`（四档优先级）：显式 `--label` > `--machine` 推导
// （仅在没有环境 label 时）> 环境 `ZERG_SWAP_LABEL` > 本机默认。
func sagResolveLabel(machine, label, envLabel, defLabel string) string {
	if label != "" {
		return label
	}
	if machine != "" && envLabel == "" {
		return "com.zerg.agent-" + machine
	}
	if envLabel != "" {
		return envLabel
	}
	return defLabel
}

// ── 输出面（脚本 `say` = `echo "$@" | tee -a "$LOG"`）────────────────────────────────────────────

// sagCtx —— 真搬执行上下文（env ＋ 两条输出面 ＋ 判决计数）。
type sagCtx struct {
	e                    sagEnv
	out, errw            io.Writer
	nPass, nFail, nBlock int
}

// say 逐字复刻脚本 `say`：写 stdout，并把同一行**追加**到 LOG（`tee -a` 语义；LOG 写不进 ⇒ 只落 stdout，同 tee 行为）。
func (c *sagCtx) say(s string) {
	fmt.Fprintln(c.out, s)
	c.appendLog(s + "\n")
}

// hr 逐字复刻脚本 `hr`。
func (c *sagCtx) hr() { c.say(sagHR) }

// teeText 逐字复刻脚本的 `… | tee -a "$LOG"`：把已渲染好的文本（含换行）**同时**写 stdout 与 LOG。
func (c *sagCtx) teeText(s string) {
	if s == "" {
		return
	}
	fmt.Fprint(c.out, s)
	c.appendLog(s)
}

func (c *sagCtx) appendLog(s string) {
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

// judge 逐字复刻脚本 `judge`：计档 ＋ `say "  [闸] $1 ⇒ $2 —— $3"`。
func (c *sagCtx) judge(name, verdict, why string) {
	switch verdict {
	case "PASS":
		c.nPass++
	case "FAIL":
		c.nFail++
	case "BLOCKED":
		c.nBlock++
	default:
		// 构造上到不了（本件只会传三档之一）；脚本此处置 `exit 2`，本件照实报到 stderr。
		fmt.Fprintf(c.errw, "!! 内部错误：未知判决 %s\n", verdict)
	}
	c.say("  [闸] " + name + " ⇒ " + verdict + " —— " + why)
}

// ── 子进程小工具（薄壳：调用与脚本**同一个** leaf 件/系统工具）────────────────────────────────────

// sagIOMode —— 子进程 stderr 的落法（对齐脚本里三种重定向形态）。
type sagIOMode int

const (
	sagIOOut   sagIOMode = iota // `… 2>/dev/null`：stdout 捕获 · stderr 丢弃
	sagIOMerge                  // `… 2>&1`：stdout 捕获合并 stderr
	sagIOErr                    // 裸 `$(…)`：stdout 捕获 · stderr 直连本命令 stderr
	sagIOQuiet                  // `… >/dev/null 2>&1`：两者都丢弃
)

// execX 跑一条命令；返回 (stdout, rc)。起不动 ⇒ rc=-1（区别于「跑起来但退非零」）。
func (c *sagCtx) execX(dir, stdin string, mode sagIOMode, name string, args ...string) (string, int) {
	cmd := exec.Command(name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Stdin = strings.NewReader(stdin)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	switch mode {
	case sagIOMerge:
		cmd.Stderr = &buf
	case sagIOErr:
		cmd.Stderr = c.errw
	default: // sagIOOut / sagIOQuiet
		cmd.Stderr = io.Discard
	}
	err := cmd.Run()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return buf.String(), ee.ExitCode()
		}
		return buf.String(), -1
	}
	return buf.String(), 0
}

// sagSed 复刻 `sed 's/^/PREFIX /'`：逐行加前缀；输入无尾换行时输出仍补一支（sed 行为）。
func sagSed(text, prefix string) string {
	if text == "" {
		return ""
	}
	lines := strings.Split(text, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	var sb strings.Builder
	for _, ln := range lines {
		sb.WriteString(prefix)
		sb.WriteString(ln)
		sb.WriteString("\n")
	}
	return sb.String()
}

// sagFirstCapRe 取首个匹配的捕获组（脚本里的 `sed -n 's/…/\1/p' | head -1`）。
func sagFirstCapRe(s, pat string) string {
	re := regexp.MustCompile(pat)
	m := re.FindStringSubmatch(s)
	if m == nil {
		return ""
	}
	return m[1]
}

// ── 两枚纯判据（与主控件 `zerg-swap-core.sh` 逐条对应）──────────────────────────────────────────

// sagSignThreeCells 逐字复刻脚本 `sign_three_cells`（换前验签的**取数档**）：
// `codesign -dvvv` 同档出 `Signature=` / `flags=` / `cdhash` 三格；退码 0 三格齐且**非 adhoc**（放行）·
// 1 **判红**（adhoc / 三格缺）· 2 **不给结论**（件不在 / codesign 起不来）。返回 (输出正文, rc)。
func (c *sagCtx) sagSignThreeCells(bin string) (string, int) {
	if _, err := os.Stat(c.e.abs(bin)); err != nil {
		return "件不存在：" + bin, 2
	}
	out, rc := c.execX(c.e.root, "", sagIOMerge, "codesign", "-dvvv", bin)
	if rc != 0 {
		return "codesign -dvvv 起不来：" + bin, 2
	}
	sig := sagFirstCapRe(out, `(?m)^Signature=(.*)$`)
	sigsize := sagFirstCapRe(out, `(?m)^Signature size=(.*)$`)
	flags := ""
	for _, ln := range strings.Split(out, "\n") {
		if !strings.HasPrefix(ln, "CodeDirectory ") {
			continue
		}
		if i := strings.Index(ln, "flags="); i >= 0 {
			rest := ln[i+len("flags="):]
			if sp := strings.IndexAny(rest, " \t"); sp >= 0 {
				rest = rest[:sp]
			}
			flags = rest
			break
		}
	}
	cdhash := ""
	for _, ln := range strings.Split(out, "\n") {
		if strings.HasPrefix(ln, "CDHash=") || strings.HasPrefix(ln, "CandidateCDHash=") {
			if i := strings.Index(ln, "="); i >= 0 {
				cdhash = ln[i+1:]
			}
			break
		}
	}
	cells := fmt.Sprintf("Signature=%s · flags=%s · cdhash=%s",
		orX(sig, "真签（size="+orX(sigsize, "缺格")+"）"), orX(flags, "（缺格）"), orX(cdhash, "（缺格）"))
	if sig == "adhoc" {
		return cells + "\n判红：Signature=adhoc —— 验签 rc=0 不算过（adhoc 件换上会被秒杀 EXIT137）", 1
	}
	if strings.Contains(flags, "adhoc") {
		return cells + "\n判红：flags=" + flags + "（ad-hoc 档）", 1
	}
	if flags == "" || cdhash == "" || (sig == "" && sigsize == "") {
		return cells + "\n判红：三格不齐（Signature / flags / cdhash）", 1
	}
	return cells, 0
}

// sagHistoryDropOK 逐字保留脚本 `history_drop_ok`（★ 脚本正文**无调用点** · 只在 `--self-test` T1/T2 有）：
// 留档落点一律 `<仓根>/bin/_history/…`；落在 `bin/` 根里平铺一份 `*.bak-*` 判红（0 合规 · 1 判红）。
func sagHistoryDropOK(rel string) (string, int) {
	if strings.HasPrefix(rel, "bin/_history/") {
		return "", 0
	}
	return fmt.Sprintf("留档落点 %s 不在 bin/_history/ 之下（产出面根里平铺 *.bak-* ⇒ 判红）", rel), 1
}

// ── 端点探针（薄壳 curl；令牌经 stdin）───────────────────────────────────────────────────────────

// curlTokCode 逐字复刻脚本 `curl_tok … 2>/dev/null || echo 000`：令牌经 **stdin** 交给 curl
// （`-H @-`）· ✗ 不落 argv；自带 `-m 30` 兜底。返回 http_code 读数（连接失败 = curl 的 `000` ＋ `|| echo 000`）。
func (c *sagCtx) curlTokCode(args ...string) string {
	full := []string{"-m", "30"}
	raw, err := os.ReadFile(c.e.tokenFile)
	var out string
	var rc int
	if err == nil {
		full = append(full, "-H", "@-")
		full = append(full, args...)
		out, rc = c.execX("", "X-Auth-Token: "+strings.TrimRight(string(raw), "\n")+"\n", sagIOQuiet, "curl", full...)
	} else {
		full = append(full, args...)
		out, rc = c.execX("", "", sagIOQuiet, "curl", full...)
	}
	if rc != 0 {
		out += "000\n"
	}
	return strings.TrimRight(out, "\n")
}

// ── 内嵌 python（薄壳：与脚本**逐字同一段** ⇒ 输出逐字同）──────────────────────────────────────

const sagPlistMissingPy = `import os, plistlib, sys
try:
    d = plistlib.load(open(sys.argv[1], "rb"))
except Exception as e:
    print("(读不到 plist: %s)" % e); sys.exit(0)
for a in (d.get("ProgramArguments") or []):
    if a.startswith("/") and not os.path.exists(a):
        print(a)`

const sagPlistProgramPy = `import plistlib, sys
try:
    d = plistlib.load(open(sys.argv[1], "rb"))
    print(" ".join(d.get("ProgramArguments") or []))
except Exception as e:
    print("(读不到 plist: %s)" % e)`

const sagBuildInfoAssertPy = `import json, sys
d = json.load(open(sys.argv[1]))
assert d.get("version") and d.get("commit") and d.get("build_time") and d.get("builder_host")`

const sagIdentPy = `import json,sys;d=json.load(open(sys.argv[1]));print("%s · %s · %s · %s"%(d["version"],d["commit"],d["build_time"],d["builder_host"]))`

const sagMachinePy = `import json;print(json.load(open("/tmp/zerg-swap-agent-status.json")).get("machine",""))`

// ── 报告流程（复刻脚本正文六闸）─────────────────────────────────────────────────────────────────

func (c *sagCtx) runReport() int {
	c.say("== 子端换件闸（只报告 · 只读）" + c.e.ts + " ==")
	c.say("日志：" + c.e.log)
	c.say("目标件：" + c.e.bin + " · 作业：" + c.e.label)

	// ── 闸A 工作树半成品 ──
	c.hr()
	c.say("[闸A] 工作树半成品（换件是从工作树编译的）")
	raw, _ := c.execX(c.e.root, "", sagIOOut, "git", "status", "--porcelain", "core/", "agent/", "ui/", "wall/")
	dirty := 0
	for _, ln := range strings.Split(raw, "\n") {
		if ln == "" || strings.HasPrefix(ln, "?? ") {
			continue
		}
		dirty++
	}
	if dirty == 0 {
		c.judge("A 工作树半成品", "PASS", "未提交改动数 = 0")
	} else {
		full, _ := c.execX(c.e.root, "", sagIOOut, "git", "-c", "core.quotepath=false", "status", "--porcelain", "core/", "agent/", "ui/", "wall/")
		var sb strings.Builder
		for _, ln := range strings.Split(full, "\n") {
			if ln == "" || strings.HasPrefix(ln, "?? ") {
				continue
			}
			sb.WriteString("       " + ln + "\n")
		}
		c.teeText(sb.String())
		c.judge("A 工作树半成品", "FAIL", fmt.Sprintf("未提交改动数 = %d（换件会把半成品编进生产件）", dirty))
	}

	// ── 闸B 服务定义可用 ──
	c.hr()
	c.say("[闸B] 服务定义可用（盘上 plist 引用的程序必须真存在）")
	c.say("      作业：" + c.e.domain + "/" + c.e.label + " · plist：" + c.e.plist)
	if st, err := os.Stat(c.e.plist); err != nil || st.IsDir() {
		c.judge("B 服务定义", "BLOCKED", "盘上 plist 不存在（"+c.e.plist+"）—— 判不了，不许当绿")
	} else {
		missingOut, _ := c.execX(c.e.root, sagPlistMissingPy, sagIOErr, "python3", "-", c.e.plist)
		missing := strings.TrimRight(missingOut, "\n")
		progOut, _ := c.execX(c.e.root, sagPlistProgramPy, sagIOErr, "python3", "-", c.e.plist)
		progStr := strings.TrimRight(progOut, "\n")
		c.say("      程序串：" + progStr)
		if missing != "" {
			c.say("      plist 引用但盘上不存在的项：")
			var sb strings.Builder
			for _, fld := range strings.Fields(missing) {
				sb.WriteString("        " + fld + "\n")
			}
			c.teeText(sb.String())
			c.judge("B 服务定义", "FAIL", "plist 引用的程序盘上不存在（launchd 用缓存定义 ⇒ kickstart 与回滚都会起不来）")
		} else {
			c.judge("B 服务定义", "PASS", "plist 引用的程序全部在位")
		}
	}

	// ── 闸① 构建口径 ──
	c.hr()
	c.say("[闸①] 构建口径（件必须由 build-all.sh 产出 · 身份不许丢）")
	// ★ 本件 `bin/build-info.json` 是**派生件 · 非身份真源**（全量档的聚合生成物 · 设计 §5 `O-12` 取（乙）· 2026-09-24 待拍清单终版 条 4/5）：
	//	✗ **勿手改**（下次全量档会被覆写）；✗ **勿当身份真源** —— 身份真源 = **制品自身嵌入段**（读法 `zerg version --build-info`）。
	bi := "bin/build-info.json"
	if st, err := os.Stat(c.e.abs(bi)); err != nil || st.IsDir() {
		c.judge("① 构建口径", "BLOCKED", "缺 "+bi+"（读不到构件身份 ⇒ 判不了，不许当绿）")
	} else if _, rc := c.execX(c.e.root, sagBuildInfoAssertPy, sagIOQuiet, "python3", "-", bi); rc != 0 {
		c.judge("① 构建口径", "BLOCKED", bi+" 解析不出四格身份（version/commit/build_time/builder_host）")
	} else {
		identOut, _ := c.execX(c.e.root, "", sagIOErr, "python3", "-c", sagIdentPy, bi)
		ident := strings.TrimRight(identOut, "\n")
		c.judge("① 构建口径", "PASS", "构件身份齐："+ident+"（造件入口 = scripts/build/build-all.sh）")
	}

	// ── 闸② 验签（取数档 -dvvv） ──
	c.hr()
	c.say("[闸②] 验签（取数档 = codesign -dvvv ⇒ Signature= / flags= / cdhash 三格同档；输出原样打印，不吞）")
	if _, err := os.Stat(c.e.abs(c.e.bin)); err != nil {
		c.judge("② 验签", "BLOCKED", "件不存在（"+c.e.bin+"）—— 判不了")
	} else {
		dv, _ := c.execX(c.e.root, "", sagIOMerge, "codesign", "-dvvv", c.e.bin)
		var f []string
		for _, ln := range strings.Split(dv, "\n") {
			if strings.HasPrefix(ln, "Identifier=") || strings.HasPrefix(ln, "CodeDirectory ") ||
				strings.HasPrefix(ln, "CandidateCDHash ") || strings.HasPrefix(ln, "CDHash=") {
				f = append(f, ln)
			}
		}
		joined := ""
		if len(f) > 0 {
			joined = strings.Join(f, " ") + " "
		}
		c.say("      " + joined)
		cells, crc := c.sagSignThreeCells(c.e.bin)
		c.say("      三格：" + cells)
		switch {
		case crc == 1:
			c.judge("② 验签", "FAIL", c.e.bin+" **判红**（条 10 · 取数档 -dvvv）："+cells)
		case crc != 0:
			c.judge("② 验签", "BLOCKED", c.e.bin+" 三格取不到 ⇒ 判不了（不许当绿）："+cells)
		default:
			vout, vrc := c.execX(c.e.root, "", sagIOMerge, "codesign", "--verify", "--verbose=2", c.e.bin)
			c.teeText(sagSed(vout, "      verify: "))
			if vrc == 0 {
				c.judge("② 验签", "PASS", c.e.bin+" 验签通过（Signature / flags / cdhash 三格齐 · 非 adhoc）")
			} else {
				c.judge("② 验签", "FAIL", c.e.bin+" 验签不过（签名失效的件换上会被秒杀 EXIT137）")
			}
		}
	}

	// ── 闸③ 回滚位 ──
	c.hr()
	c.say("[闸③] 回滚位（回滚件必须**在位且验签通过** —— 「退得回去」得先证明）")
	rb := "bin/_history/" + filepath.Base(c.e.bin) + ".prev"
	if _, err := os.Stat(c.e.abs(rb)); err != nil {
		c.judge("③ 回滚位", "BLOCKED", "无回滚件（"+rb+"）—— 换上去就退不回来 ⇒ 不给结论（**不许当绿**）")
	} else if _, rc := c.execX(c.e.root, "", sagIOQuiet, "codesign", "--verify", "--verbose=1", rb); rc == 0 {
		c.judge("③ 回滚位", "PASS", "回滚件在位且验签通过（"+rb+"）")
	} else {
		c.judge("③ 回滚位", "FAIL", "回滚件在位但**验签不过**（"+rb+"）—— 这正是 2026-09-16 二次事故的形态")
	}

	// ── 闸④ 换件后自证判据可跑 ──
	c.hr()
	c.say("[闸④] 换件后自证判据可跑（只读探针 —— 判据活着才谈得上自证）")
	ready := c.curlTokCode("-s", "-o", "/tmp/zerg-swap-agent-ready.json", "-w", "%{http_code}", "-m", "4", c.e.readyURL)
	status := c.curlTokCode("-s", "-o", "/tmp/zerg-swap-agent-status.json", "-w", "%{http_code}", "-m", "4", c.e.statusURL)
	c.say("      /ready=" + ready + " · /status=" + status)
	if ready == "200" && status == "200" {
		machineOut, _ := c.execX("", "", sagIOOut, "python3", "-c", sagMachinePy)
		machine := strings.TrimRight(machineOut, "\n")
		c.judge("④ 自证判据", "PASS", "探针 200 · 子端自报 machine="+machine)
	} else if ready == "000" && status == "000" {
		c.judge("④ 自证判据", "BLOCKED", "两个探针都打不到（服务没在跑？）—— 判不了，不许当绿")
	} else {
		c.judge("④ 自证判据", "FAIL", "探针异常（/ready="+ready+" · /status="+status+"；401 = 令牌不对）")
	}

	c.hr()
	c.say(fmt.Sprintf("状态计数：PASS %d · FAIL %d · BLOCKED(不给结论) %d", c.nPass, c.nFail, c.nBlock))
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

// ── `--self-test`（本件**纯判据**的逻辑夹具 · 缺口 `GAP-20260926-195` · 不碰真件/服务/launchd）──

func (c *sagCtx) selfTest() int {
	stPass, stFail := 0, 0
	st := func(name string, want, got int) {
		if want == got {
			c.say(fmt.Sprintf("  [自检] %s ⇒ PASS（rc=%d）", name, got))
			stPass++
		} else {
			c.say(fmt.Sprintf("  [自检] %s ⇒ FAIL（期望 rc=%d · 实得 rc=%d）", name, want, got))
			stFail++
		}
	}
	rcIf := func(cond bool) int {
		if cond {
			return 0
		}
		return 1
	}
	c.say("== zerg-swap-agent.sh 自检（--self-test · 不碰真件/服务/launchd）" + c.e.ts + " ==")
	_, r1 := sagHistoryDropOK("bin/_history/zerg-agentd.prev")
	st("T1 留档落点合规（bin/_history/）", 0, r1)
	_, r2 := sagHistoryDropOK("bin/zerg-agentd.bak-x")
	st("T2 落点平铺判红", 1, r2)
	_, r3 := c.sagSignThreeCells("/no/such/bin-zsa-" + strconv.Itoa(os.Getpid()))
	st("T3 件不存在⇒不给结论", 2, r3)
	ft, ferr := os.CreateTemp("", "zsa-selftest")
	if ferr == nil {
		ftPath := ft.Name()
		_ = ft.Close()
		if eb, err := os.ReadFile("/bin/echo"); err == nil {
			_ = os.WriteFile(ftPath, eb, 0o755)
		}
		_, _ = c.execX("", "", sagIOQuiet, "codesign", "--force", "-s", "-", ftPath)
		ftOut, _ := c.execX("", "", sagIOMerge, "codesign", "-dvvv", ftPath)
		if strings.Contains(ftOut, "Signature=adhoc") {
			_, r4 := c.sagSignThreeCells(ftPath)
			st("T4 adhoc 件判红", 1, r4)
		} else {
			c.say("  [自检] T4 adhoc 夹具建不起来 ⇒ FAIL（需 codesign 权限）")
			stFail++
		}
		_ = os.Remove(ftPath)
	}
	if _, err := os.Stat("/bin/echo"); err == nil {
		_, r5 := c.sagSignThreeCells("/bin/echo")
		st("T5 真签件放行", 0, r5)
	} else {
		c.say("  [自检] T5 缺 /bin/echo 夹具 ⇒ FAIL")
		stFail++
	}
	// T6–T9：目标解析纯函数（缺口 `GAP-20260925-21` · 四档优先级）
	st("T6 默认作业名 = "+sagDefLabel, 0, rcIf(sagResolveLabel("", "", "", sagDefLabel) == sagDefLabel))
	st("T7 --machine demo1 推导作业名", 0, rcIf(sagResolveLabel("demo1", "", "", sagDefLabel) == "com.zerg.agent-demo1"))
	st("T8 --label 覆盖 --machine", 0, rcIf(sagResolveLabel("demo1", "com.zerg.agent-x", "", sagDefLabel) == "com.zerg.agent-x"))
	st("T9 环境 label 优先于 --machine", 0, rcIf(sagResolveLabel("demo1", "", "com.zerg.agent-env", sagDefLabel) == "com.zerg.agent-env"))
	c.say(fmt.Sprintf("自检：%d 过 / %d 红（共 %d 条）", stPass, stFail, stPass+stFail))
	if stFail == 0 {
		return exitOK
	}
	return exitFail
}

// ── 用法（`--help` 原文即下面这段；**退码与档位的唯一定义处**）────────────────────────────────────

// sagUsage 逐字返回脚本 `usage()` 的正文（`cat <<'EOF' … EOF` ⇒ 行间 \n ＋ 末尾 \n）。
func sagUsage() string {
	return strings.Join([]string{
		"zerg-swap-agent.sh —— 子端（zerg-agentd）换件的受控流程 · **只报告档**（本轮未开放真换件）",
		"",
		"用法：",
		"  bash scripts/build/zerg-swap-agent.sh            只报告：逐闸出判决（**只读**，零副作用）",
		"  bash scripts/build/zerg-swap-agent.sh --check    同上（显式写法）",
		"  bash scripts/build/zerg-swap-agent.sh --help     本页",
		"  bash scripts/build/zerg-swap-agent.sh --self-test 自检（跑本件纯判据的夹具用例 · 不碰真件/服务/launchd）",
		"  bash scripts/build/zerg-swap-agent.sh --swap     真换件 —— **本档未开放**，rc=2 且不执行任何动作",
		"  bash scripts/build/zerg-swap-agent.sh --machine <名> [--label <作业名>]   指定目标机 / 作业名",
		"",
		"目标（默认「本机」= 作业 " + sagDefLabel + " · 目标件 bin/zerg-agentd）：",
		"  --machine <名>     按机器名推导默认作业名（com.zerg.agent-<名>）与其 plist 路径；",
		"  --label <作业名>   直接点名 launchd 作业名（覆盖 --machine 与环境默认）。",
		"  ★ 环境旋钮（7 个 · 均以 `${VAR:-默认}` 覆盖）：",
		"    ZERG_SWAP_BIN（bin/zerg-agentd）· ZERG_SWAP_LABEL（" + sagDefLabel + "）·",
		"    ZERG_SWAP_PLIST（~/Library/LaunchAgents/<label>.plist）· ZERG_SWAP_TOKEN_FILE（~/.zerg/token）·",
		"    ZERG_SWAP_READY_URL / ZERG_SWAP_STATUS_URL（http://127.0.0.1:8100/ready|/status）· ZERG_SWAP_LOG。",
		"  ★ 本件是「**本机作案**」的受控流程：--machine 改的是作业名/plist 路径，探针**默认仍打",
		"    127.0.0.1:8100**（本机子端）⇒ 给**远端**机器换件时，探针须显式给 ZERG_SWAP_READY_URL /",
		"    ZERG_SWAP_STATUS_URL（端口取自该机名册），否则闸④ 打的是本机、不是目标机。",
		"",
		"判的六闸（三闸形态 + 两前置闸；逐条对应 scripts/build/zerg-swap-core.sh）：",
		"  闸 A 工作树半成品：`git status --porcelain core/ agent/ ui/ wall/` 的非未跟踪项计数必须 0（换件是从工作树编译的）",
		"  闸 B 服务定义可用：盘上 launchd plist 引用的程序必须真存在（脚本/产物搬家会让那句路径悬空）",
		"  闸① 构建口径：件必须由 scripts/build/build-all.sh 产出（身份注入 + 重签）；读 bin/build-info.json 判",
		"  闸② 验签：取数档 `codesign -dvvv` ⇒ `Signature=` / `flags=` / `cdhash` **三格齐且非 adhoc**（adhoc ⇒ **判红**；输出原样打印，不吞）",
		"  闸③ 回滚位：回滚件（bin/_history/<件>.prev）必须**在位且验签通过** —— 「退得回去」得先证明",
		"  闸④ 换件后自证判据可跑：探针打到子端自身 `/ready` + `/status`（只读 GET），证明判据活着",
		"",
		"纪律句（**不是判据** · 启动方式口径 · ⒟甲）：",
		"  一切执行体一律 `bash <绝对路径>` 启动 ⇒ **不依赖 x 位**（真机上件可能没有 x 位；判据不声称「可直接执行」）",
		"",
		"退码（唯一说法）：",
		"  0  全部闸 PASS（只读判定通过）",
		"  1  有闸判 FAIL（真红：件缺/验签不过/工作树有半成品/plist 悬空…）",
		"  2  用法错 · 或**不给结论**（有闸 BLOCKED：缺件、读不动、探针打不到、空转——**不许当绿**）",
		"     · 或敲了 `--swap`（**真换件档未开放**：要开先拍，见下）",
		"  3  **保留号**（回滚也失败 ⇒ 需人工）；只报告档跑不到它，占号是为与主控件逐号对齐",
		"",
		"真换件档为什么未开放（要开先拍，不偷偷开）：",
		"  ① 本件今天**红两处**：`bin/_history/` 里没有 `zerg-agentd.prev` 回滚位；跨架构件无签名（§11 B7/P4）；",
		"  ② 换件要动正在跑的子端进程 ⇒ 属**不可逆档**，按仓内口径要 Mr2109 拍；",
		"  ③ 拍之前，本件只做「证明它会不会红」这件事本身（先只报告 → 归零 → 升阻断）。",
	}, "\n") + "\n"
}

// ── 执行体（`zerg swap agent` 的真搬正门）───────────────────────────────────────────────────────

// cmdSwapAgent —— `zerg swap agent [--check | --help | --self-test | --machine <名> [--label <作业名>]] [--dry-run | --yes]`（真搬正门）。
//
// 退码口径（**逐字照底层**）：0 六闸全通过 · 1 有闸判 FAIL · 2 用法错 / 不给结论 / `--swap` 未开放。
//
// ★ 授权侧（`GAP-20260927-140` ① · Mr2109 2026-10-08 拍「默认档要 fail-closed」）：写盘档（裸跑 /
// `--swap` / `--dry-run` / `--yes`）走本层授权门 —— `--dry-run` 出计划件（零副作用）；缺 `--yes` 退 2 零写；
// 只读档（`--check` / `--help` / `--self-test` / `--machine` / `--label`）原样透传、不进门。
// 本件自己只在「解析不到仓根」那一档退 8（旧壳转账「递不到脚本 ⇒ 8」那档已随真搬消失）。
func cmdSwapAgent(inv *invocation, stdout, stderr io.Writer) int {
	root := repoRoot()
	if root == "" {
		inv.setErr("blocked", "repo_root_absent", "解析不到仓根")
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 不给结论（退码 8）\n", progName)
		return exitBlocked
	}
	// ★ 授权侧：写盘档（裸跑 / `--swap` / `--dry-run` / `--yes`）走 fail-closed 授权门；只读档不进。
	if !swapReadOnlyTier(inv) {
		if rc, done := writeGuardDryRun(inv, stdout, stderr); done {
			return rc
		}
		if rc := writeGuardYes(inv, stderr); rc != exitOK {
			return rc
		}
	}
	return sagDispatch(root, swapForwardTail(inv), stdout, stderr)
}

// sagDispatch 复刻脚本的参数解析 ＋ 动作分发。tail = `zerg swap agent` 之后的透传尾巴
// （授权旗标 `--dry-run`/`--yes` 已由本层消费 · 不到这里）。
func sagDispatch(root string, tail []string, stdout, stderr io.Writer) int {
	action, argMachine, argLabel := "", "", ""
	for i := 0; i < len(tail); {
		a := tail[i]
		switch {
		case a == "" || a == "--check":
			if action == "" {
				action = "check"
			}
			i++
		case a == "--swap":
			action = "swap"
			i++
		case a == "--help" || a == "-h":
			fmt.Fprint(stdout, sagUsage())
			return exitOK
		case a == "--self-test":
			action = "selftest"
			i++
		case a == "--machine":
			if i+1 >= len(tail) {
				fmt.Fprintf(stderr, "✗ --machine 缺取值（用法见：bash scripts/build/zerg-swap-agent.sh --help）\n")
				return exitUsage
			}
			argMachine = tail[i+1]
			i += 2
		case strings.HasPrefix(a, "--machine="):
			argMachine = strings.TrimPrefix(a, "--machine=")
			i++
		case a == "--label":
			if i+1 >= len(tail) {
				fmt.Fprintf(stderr, "✗ --label 缺取值（用法见：bash scripts/build/zerg-swap-agent.sh --help）\n")
				return exitUsage
			}
			argLabel = tail[i+1]
			i += 2
		case strings.HasPrefix(a, "--label="):
			argLabel = strings.TrimPrefix(a, "--label=")
			i++
		default:
			fmt.Fprintf(stderr, "✗ 未知参数: %s（用法见：bash scripts/build/zerg-swap-agent.sh --help）\n", a)
			return exitUsage
		}
	}
	if action == "" {
		action = "check"
	}
	// ★ 真换件档未开放（P13 拍板：先工装 · 先只报告）—— 逐字复刻脚本两行 stderr 文案。
	if action == "swap" {
		fmt.Fprintln(stderr, "✗ zerg-swap-agent.sh：**真换件档未开放**（P13 拍板：先工装 · 先只报告）")
		fmt.Fprintln(stderr, "  ⇒ 零副作用退出（rc=2）。要开先拍；拍后本档补：备份 → build-all.sh --only-agentd → 验签 → 重启 → 自证 → 回滚路径。")
		return exitUsage
	}
	ctx := &sagCtx{e: sagEnvOf(root, argMachine, argLabel), out: stdout, errw: stderr}
	if action == "selftest" {
		return ctx.selfTest()
	}
	return sagFlowFn(ctx)
}
