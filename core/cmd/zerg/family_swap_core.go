// family_swap_core.go —— 主控换件口的 **CLI 正门**（`zerg swap core`）· ★ 2026-10-08 **真搬**。
//
// 依据：`~/.zerg/state/A档18件-收CLI设计稿-20261007.md` 批 1 · A1（★事故面）
//
//	（`scripts/build/zerg-swap-core.sh` · 417 行 · 现属面＝构建面/换件面 · 归族 = 新族 `swap`）。
//
// ★ 2026-10-08 **真搬**（本单）：底层行为**收进本件**（✗ 不再 exec `scripts/build/zerg-swap-core.sh`）——
// 把该脚本的**实际行为**在 Go 内**等价复现**：参数解析（裸跑 / `--swap` / `--help` / `-h` /
// `--self-test-rollback` / 未知）· 用法文案 · 退码（0 / 1 / 2 / 3）· 两道前置闸（工作树半成品 /
// launchd 定义可用）· 三道门禁（构建只走 build-all.sh / 换前验签（adhoc 判红）/ 起不来即回滚且回滚件
// 同样验签）· 换前现场与备份 · 重启主控（定义不一致即 bootout+bootstrap）· 等就绪与读身份 · 回滚路径 ·
// 回滚路径自测（抛靶 /tmp）。
//
// 对外可观察面（rc/stdout/stderr）与脚本直跑**逐字同**；**不再依赖脚本在盘** —— 脚本退役（移出仓）后
// 两态三面逐字同（✗ 不再退 `8`；旧壳转账「递不到脚本 ⇒ 8」那档已消失 —— 行为已内化）。
//
// ★ 不可等价复现的步骤 ⇒ 保留**薄壳**（同一个 leaf 件/系统工具 · 逐条在此声明）：
//
//	· `say`（`echo "$@" | tee -a "$LOG"`）⇒ Go **双写** stdout ＋ 追加 LOG；
//	· 构建 `bash scripts/build/build-all.sh --only-core` ⇒ **薄壳**（真编生产件 ＝ 不可等价复现 ⇒ 不重写 ·
//	  只转发；身份注入/重签仍在那边 —— 动生产 `bin/` 是冻结面，须与那边同源）；
//	· 验签取数 `codesign -dvvv` / `codesign --verify` / 回滚重签 `codesign -s - --force --identifier` ⇒ **薄壳**（同一二进制工具）；
//	· launchd 读/写（`launchctl print|list|bootstrap|bootout|kickstart`）⇒ **薄壳**（OS 原生面）；
//	· plist 解析（脚本内嵌 `python3` ＋ `plistlib`）⇒ **薄壳**（内嵌**同一段** python ⇒ 逐字同）；
//	· 端点探针 `curl -m …`（令牌经 **stdin** ＋ `-H @-` · ✗ 不落 argv）⇒ **薄壳**；
//	· 件尺寸/摘要/字节比（`stat -f%z` / `shasum -a 256` / `cmp -s`）⇒ Go stdlib（`os.Stat` / `sha256` / `bytes.Equal`）；
//	· 睡 `sleep 2`（就绪等待）⇒ `time.Sleep`（接缝 `swcSleep` 可被测试替换）。
//
// ★ 两处**如实声明**（✗ 不改对外可观察行为）：
//
//	① 脚本的 `restart_master` 有一枚自测桩 `ZERG_SWAP_RESTART_STUB`（`eval` 一枚**仓外命令串**）——
//	   那是「执行位 ← 仓外输入」的形态（缺口 `GAP-20260927-145` 同族）⇒ 本件**去掉该分支**：自测桩只在
//	   Go 内以**内建空动作**（等价脚本自测内部设的 `:`）实现 ⇒ 外部 `export` 该变量**无效**
//	   （脚本自测内部设的就是 `:` ⇒ 自测面逐字同）。
//	② 脚本内 `history_drop_ok` 现读**无调用点**（定义在、正文不调）⇒ 本件**逐字保留**为 `swcHistoryDropOK`
//	   （无副作用 · 无调用点 —— 只为与原件逐条对得上）。
//
// 三条口径（照仓里既有的真搬族写 —— `ui_start.go` / `isolate_ro.go`，不自己创式样）：
//
//	① **逐字照底层**：文案 / 退码一字不改（0 成功 · 1 不成功（未动生产 / 回滚成功 ROLLED_BACK）·
//	   2 用法错 · 3 回滚也失败（需人工））。
//	② **只留一道仓根解析失败的 fail-closed**（`8` · 与 `ui`/`swap` 族同档）——
//	   旧壳转账「递不到脚本 ⇒ 8」那档已消失（行为已内化）。
//	③ **不新增机器面**：底层脚本没有 `--json` ⇒ 本件也不自己拼一份包封（契约在命令面只有一处）。
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
	"strings"
	"time"
)

// swapCoreScriptRel —— 主控换件脚本的仓内路径。★ 真搬后本件**不再 exec 它**（行为已内化）——
// 保留常量只为 ① 报面/诊断指向人读件名 ② 测试用它定位「毒壳」落点（证「不再 exec」这条牙）。
const swapCoreScriptRel = pathBuildSwapCoreSh

// swcHR —— 脚本 `hr()` 的那条横线（46 × U+2500）。
var swcHR = strings.Repeat("\u2500", 46)

// swcSleep —— 就绪等待用的睡眠（脚本 `sleep 2`）· 测试接缝（测试换掉 ⇒ 不真等）。
var swcSleep = time.Sleep

// swcFlowFn —— **真跑流程**的函数级接缝（★ 安全线：单测**绝不**真跑换件 ⇒ 测试替换本变量为桩）。
// 与 `family_core_restart.go` 的 `coreRestartAction` 同一条纪律：缝**只在 Go 里**（✗ 不从环境变量取命令串）。
var swcFlowFn = func(c *swcCtx) int { return c.runFlow() }

// swcEnv —— 脚本顶部那几枚**可覆盖点**（默认即生产路径；`bash` 版先 `cd "$REPO_ROOT"` ⇒ 相对路径以仓根为基）。
type swcEnv struct {
	root, bin, label, domain, plist, endpoint, ts, log, backup, tokenFile string
}

func swcEnvOf(root string) swcEnv {
	home, _ := os.UserHomeDir()
	get := func(k, def string) string {
		if v := os.Getenv(k); v != "" {
			return v
		}
		return def
	}
	e := swcEnv{root: root}
	e.ts = time.Now().Format("20060102T150405")
	e.bin = get("ZERG_SWAP_BIN", "bin/zerg-core")
	e.label = get("ZERG_SWAP_LABEL", "com.zerg.core")
	e.domain = fmt.Sprintf("gui/%d", os.Getuid())
	e.plist = filepath.Join(home, "Library", "LaunchAgents", e.label+".plist")
	e.endpoint = get("ZERG_SWAP_ENDPOINT", "http://127.0.0.1:8580/api/capabilities")
	e.log = get("ZERG_SWAP_LOG", "/tmp/zerg-swap-"+e.ts+".log")
	e.backup = get("ZERG_SWAP_BACKUP", "bin/_history/zerg-core.bak-"+e.ts)
	e.tokenFile = get("TOKEN_FILE", filepath.Join(home, ".zerg", "token"))
	return e
}

// swcAbs 把仓内相对路径按仓根展开（绝对路径原样 —— 脚本 `cd "$REPO_ROOT"` 后同形）。
func swcAbs(root, p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(root, p)
}

func (e swcEnv) binPath() string    { return swcAbs(e.root, e.bin) }
func (e swcEnv) backupPath() string { return swcAbs(e.root, e.backup) }

// ── 输出面（脚本 `say` = `echo "$@" | tee -a "$LOG"`）────────────────────────────────────────────

// swcCtx —— 真搬执行上下文（env ＋ 三条输出面 ＋ 自测桩态）。
type swcCtx struct {
	e        swcEnv
	out      io.Writer
	errw     io.Writer
	log      string // 追加日志落点（脚本 $LOG）
	stubOn   bool   // 自测桩态（只有回滚路径自测置真）
	stubText string // 自测桩正文（脚本自测里 = ":"）
}

// say 逐字复刻脚本 `say`：写 stdout，并把同一行**追加**到 LOG（`tee -a` 语义；LOG 写不进 ⇒ 只落 stdout，同 tee 行为）。
func (c *swcCtx) say(s string) {
	fmt.Fprintln(c.out, s)
	c.appendLog(s + "\n")
}

// hr 逐字复刻脚本 `hr`。
func (c *swcCtx) hr() { c.say(swcHR) }

// teeText 逐字复刻脚本的 `… | tee -a "$LOG"`：把已渲染好的文本（含换行）**同时**写 stdout 与 LOG。
func (c *swcCtx) teeText(s string) {
	if s == "" {
		return
	}
	fmt.Fprint(c.out, s)
	c.appendLog(s)
}

func (c *swcCtx) appendLog(s string) {
	if c.log == "" {
		return
	}
	f, err := os.OpenFile(c.log, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	fmt.Fprint(f, s)
	_ = f.Close()
}

// ── 子进程小工具（薄壳：调用与脚本**同一个** leaf 件/系统工具）────────────────────────────────────

// swcExec 跑一条命令并取**合并**输出（脚本里的 `2>&1`）⇒ (out, rc)；起不动 ⇒ rc=-1（区别于「跑起来但退非零」）。
func swcExec(dir, stdin, name string, args ...string) (string, int) {
	cmd := exec.Command(name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Stdin = strings.NewReader(stdin)
	var buf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &buf, &buf
	err := cmd.Run()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return buf.String(), ee.ExitCode()
		}
		return buf.String(), -1
	}
	return buf.String(), 0
}

// swcExecStdout 只取 stdout（脚本里的 `… 2>/dev/null |` 形态）。
func swcExecStdout(stdin, name string, args ...string) string {
	cmd := exec.Command(name, args...)
	cmd.Stdin = strings.NewReader(stdin)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = io.Discard
	_ = cmd.Run()
	return buf.String()
}

// execOut 取子进程 **stdout**，stderr 直连本命令 stderr（脚本里 `$(…)` 抓 stdout、stderr 走脚本 stderr 的同形）。
func (c *swcCtx) execOut(stdin, name string, args ...string) string {
	cmd := exec.Command(name, args...)
	cmd.Stdin = strings.NewReader(stdin)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = c.errw
	_ = cmd.Run()
	return buf.String()
}

// run 把子进程 stdout/stderr 直连本命令的两个输出面（脚本里不接管道的裸命令）。
func (c *swcCtx) run(name string, args ...string) int {
	cmd := exec.Command(name, args...)
	cmd.Stdin = strings.NewReader("")
	cmd.Stdout, cmd.Stderr = c.out, c.errw
	err := cmd.Run()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return ee.ExitCode()
		}
		return -1
	}
	return 0
}

// ── 逐字助手 ──────────────────────────────────────────────────────────────────────────────────

// swcSed 复刻 `sed 's/^/PREFIX /'`：逐行加前缀；输入无尾换行时输出仍补一支（sed 行为）。
func swcSed(text, prefix string) string {
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

// swcFirstCapRe 取首个匹配的捕获组（脚本里的 `sed -n 's/…/\1/p' | head -1`）。
func swcFirstCapRe(s, pat string) string {
	re := regexp.MustCompile(pat)
	m := re.FindStringSubmatch(s)
	if m == nil {
		return ""
	}
	return m[1]
}

// swcSHA256 复刻 `shasum -a 256 <件> | awk '{print $1}'`（读不到 ⇒ 空）。
func swcSHA256(p string) string {
	b, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return fmt.Sprintf("%x", sum)
}

// swcFilesEqual 复刻 `cmp -s <a> <b>`（读不到任一件 ⇒ 假）。
func swcFilesEqual(a, b string) bool {
	ba, ea := os.ReadFile(a)
	bb, eb := os.ReadFile(b)
	if ea != nil || eb != nil {
		return false
	}
	return bytes.Equal(ba, bb)
}

// ── 脚本顶部：两道只读判据（历史沿用体例）──────────────────────────────────────────────────────

// swcSignThreeCells 逐字复刻脚本 `sign_three_cells`（换前验签的**取数档**）：
// `codesign -dvvv` 同档出 `Signature=` / `flags=` / `cdhash` 三格；退码 0 三格齐且**非 adhoc**（放行）·
// 1 **判红**（adhoc / 三格缺）· 2 **不给结论**（件不在 / codesign 起不来）。返回 (输出正文, rc)。
func (c *swcCtx) swcSignThreeCells(bin string) (string, int) {
	if _, err := os.Stat(bin); err != nil {
		return "件不存在：" + bin, 2
	}
	out, rc := swcExec("", "", "codesign", "-dvvv", bin)
	if rc != 0 {
		return "codesign -dvvv 起不来：" + bin, 2
	}
	sig := swcFirstCapRe(out, `(?m)^Signature=(.*)$`)
	sigsize := swcFirstCapRe(out, `(?m)^Signature size=(.*)$`)
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
	sigCell := orX(sig, "真签（size="+orX(sigsize, "缺格")+"）")
	cells := fmt.Sprintf("Signature=%s · flags=%s · cdhash=%s", sigCell, orX(flags, "（缺格）"), orX(cdhash, "（缺格）"))
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

// swcHistoryDropOK 逐字保留脚本 `history_drop_ok`（★ 脚本内**无调用点** · 只为逐条对得上）：留档落点
// 一律 `<仓根>/bin/_history/…`；落在 `bin/` 根里平铺一份 `*.bak-*` 判红（0 合规 · 1 判红）。
func swcHistoryDropOK(rel string) int {
	if strings.HasPrefix(rel, "bin/_history/") {
		return 0
	}
	fmt.Printf("留档落点 %s 不在 bin/_history/ 之下（产出面根里平铺 *.bak-* ⇒ 判红）\n", rel)
	return 1
}

// ── 端点探针 / 身份（薄壳 curl ＋ python3）──────────────────────────────────────────────────────

// curlTok 逐字复刻脚本 `curl_tok`：令牌经 **stdin** 交给 curl（`-H @-`）· ✗ 不落 argv；缺件 ⇒ 退不带该头的形态。
// 自带 `-m 30` 兜底；调用方给的 `-m` 覆盖它（curl 后者胜）。
func (c *swcCtx) curlTok(args ...string) string {
	full := []string{"-m", "30"}
	raw, err := os.ReadFile(c.e.tokenFile)
	if err == nil {
		full = append(full, "-H", "@-")
		full = append(full, args...)
		cmd := exec.Command("curl", full...)
		cmd.Stdin = strings.NewReader("X-Auth-Token: *** \"" + strings.TrimRight(string(raw), "\n") + "\"\n")
		cmd.Stderr = c.errw
		var buf bytes.Buffer
		cmd.Stdout = &buf
		_ = cmd.Run()
		return strings.TrimRight(buf.String(), "\n")
	}
	full = append(full, args...)
	cmd := exec.Command("curl", full...)
	cmd.Stderr = c.errw
	var buf bytes.Buffer
	cmd.Stdout = &buf
	_ = cmd.Run()
	return strings.TrimRight(buf.String(), "\n")
}

// probe 复刻脚本 `probe`：`curl -s -o /tmp/swap-probe.json -w '%{http_code}' -m 4 <端点>`。
func (c *swcCtx) probe() string {
	return c.curlTok("-s", "-o", "/tmp/swap-probe.json", "-w", "%{http_code}", "-m", "4", c.e.endpoint)
}

// swcIdentityPy —— 脚本 `identity()` 内嵌的那段 python（逐字）。
const swcIdentityPy = `import json
try:
    d=json.load(open('/tmp/swap-probe.json'))
    print('code_sha=%s version=%s' % (d.get('code_sha'), d.get('version')))
except Exception as e:
    print('(读不到身份: %s)' % e)`

// identity 复刻脚本 `identity`（读上一发探针落件）。
func (c *swcCtx) identity() string {
	out := c.execOut("", "python3", "-c", swcIdentityPy)
	return strings.TrimRight(out, "\n")
}

// waitReady 复刻脚本 `wait_ready`：最多 `secs/2` 轮，每轮 `sleep 2` ＋ 探一发，命中 200 即退。
func (c *swcCtx) waitReady(secs int) string {
	code := "000"
	for i := 0; i < secs/2; i++ {
		swcSleep(2 * time.Second)
		code = c.probe()
		if code == "200" {
			break
		}
	}
	return code
}

// ── launchd 定义的两个读法（都只读 · 薄壳 python3/launchctl）─────────────────────────────────────

// swcPlistProgramPy —— 脚本 `plist_program` 内嵌 python（逐字）。
const swcPlistProgramPy = `import plistlib, sys
try:
    d = plistlib.load(open(sys.argv[1], "rb"))
except Exception as e:
    print("(读不到 plist: %s)" % e); sys.exit(1)
print(" ".join(d.get("ProgramArguments") or []))`

// swcPlistMissingPy —— 脚本 `plist_missing_files` 内嵌 python（逐字）。
const swcPlistMissingPy = `import os, plistlib, sys
try:
    d = plistlib.load(open(sys.argv[1], "rb"))
except Exception:
    sys.exit(0)
for a in (d.get("ProgramArguments") or []):
    if a.startswith("/") and not os.path.exists(a):
        print(a)`

// swcLoadedProgramPy —— 脚本 `loaded_program` 内嵌 python（逐字）。
const swcLoadedProgramPy = `import re, sys
txt = sys.stdin.read()
m = re.search(r"arguments = \{(.*?)\}", txt, re.S)
if not m:
    print(""); sys.exit(0)
print(" ".join(x.strip() for x in m.group(1).split("\n") if x.strip()))`

// plistProgram 复刻脚本 `plist_program`（盘上 plist 的 ProgramArguments，空格连接；读不到 ⇒ 打印原因）。
func (c *swcCtx) plistProgram() string {
	out := c.execOut(swcPlistProgramPy, "python3", "-", c.e.plist)
	return strings.TrimRight(out, "\n")
}

// plistMissingFiles 复刻脚本 `plist_missing_files`（plist 里「看起来是路径但盘上不存在」的项，每行一个）。
func (c *swcCtx) plistMissingFiles() string {
	out := c.execOut(swcPlistMissingPy, "python3", "-", c.e.plist)
	return strings.TrimRight(out, "\n")
}

// loadedProgram 复刻脚本 `loaded_program`（launchd **已加载**定义里的 ProgramArguments；没有该作业 ⇒ 空）。
func (c *swcCtx) loadedProgram() string {
	text := swcExecStdout("", "launchctl", "print", c.e.domain+"/"+c.e.label)
	out, _ := swcExec("", text, "python3", "-c", swcLoadedProgramPy)
	return strings.TrimRight(out, "\n")
}

// jobLoaded 复刻脚本 `job_loaded`（作业在 launchd 里吗）。
func (c *swcCtx) jobLoaded() bool {
	_, rc := swcExec("", "", "launchctl", "print", c.e.domain+"/"+c.e.label)
	return rc == 0
}

// waitJobGone 复刻脚本 `wait_job_gone`（bootout 是**异步的** —— 等它真注销）。
func (c *swcCtx) waitJobGone(secs int) bool {
	for i := 0; i < secs; i++ {
		if !c.jobLoaded() {
			return true
		}
		swcSleep(time.Second)
	}
	return false
}

// bootstrapJob 复刻脚本 `bootstrap_job`（bootstrap ＋ 复核 ＋ 重试；EIO 是「上一个作业还没注销完」的典型形态）。
func (c *swcCtx) bootstrapJob() bool {
	for i := 1; i <= 5; i++ {
		out, _ := swcExec("", "", "launchctl", "bootstrap", c.e.domain, c.e.plist)
		c.teeText(out)
		if c.jobLoaded() {
			return true
		}
		c.say(fmt.Sprintf("  [重启] bootstrap 第 %d 次后作业仍未装载 ⇒ 等 2s 重试（bootout 异步；立刻 bootstrap 会 EIO）", i))
		swcSleep(2 * time.Second)
	}
	return c.jobLoaded()
}

// ── 唯一的重启入口（复刻脚本 `restart_master`）──────────────────────────────────────────────────
//
// 为什么不能一律 `kickstart -k`：launchd **缓存的是加载时的作业定义**。盘上 plist 被改过而作业没重新
// 加载时，kickstart 仍按**缓存定义**拉 —— 缓存里的程序路径若已悬空，行为是「先把在跑的主控 kill 掉、
// 再用不存在的程序起、起不来」。判据落在**运行时**：定义不一致就 bootout+bootstrap。
func (c *swcCtx) restartMaster() int {
	if c.stubOn {
		c.say("  [重启] 自测桩：" + c.stubText)
		return 0
	}
	lp := c.loadedProgram()
	dp := c.plistProgram()
	if lp == "" {
		c.say("  [重启] launchd 里没有 " + c.e.label + " ⇒ bootstrap（首次装载）")
		if !c.bootstrapJob() {
			c.say("!! bootstrap 五次都没装载成功")
			return 1
		}
		return 0
	}
	if lp != dp {
		c.say("  [重启] 已加载定义 ≠ 盘上 plist ⇒ bootout + bootstrap（刷新定义；kickstart 用缓存定义会起不来）")
		c.say("         已加载: " + lp)
		c.say("         盘上  : " + dp)
		_, _ = swcExec("", "", "launchctl", "bootout", c.e.domain+"/"+c.e.label)
		if !c.waitJobGone(10) {
			c.say("!! bootout 后 10s 作业仍在 launchd 里 ⇒ 不 bootstrap（先查为什么注销不掉）")
			return 1
		}
		c.say("  [重启] 作业已注销（异步完成）⇒ bootstrap 新定义")
		if !c.bootstrapJob() {
			c.say("!! bootstrap 五次都没装载成功")
			return 1
		}
		return 0
	}
	c.say("  [重启] 已加载定义 == 盘上 plist ⇒ kickstart -k")
	return c.run("launchctl", "kickstart", "-k", c.e.domain+"/"+c.e.label)
}

// ── 回滚路径（**同一个方法**被两条入口用：真失败支 ＋ `--self-test-rollback`）─────────────────────
//
// 返回码就是本件的退码语义：1 = 回滚成功（ROLLED_BACK）· 3 = 回滚也失败（需人工）。
func (c *swcCtx) rollbackToBackup() int {
	c.hr()
	c.say("[4] 新件起不来 ⇒ 回滚（含验签）")
	c.say("  失败现场（" + c.log + " 尾）：")
	_ = c.run("tail", "-25", c.log)
	if rc := c.run("cp", "-p", c.e.backupPath(), c.e.binPath()); rc != 0 {
		c.say("!! 复制回滚件失败 ⇒ 人工介入（error.kind=ROLLBACK_FAILED）")
		return 3
	}
	if st, err := os.Stat(c.e.binPath()); err == nil {
		c.say(fmt.Sprintf("  回滚件已就位：%d 字节", st.Size()))
	}
	signOut, _ := swcExec("", "", "codesign", "-s", "-", "--force", "--identifier", c.e.label, c.e.binPath())
	c.teeText(swcSed(signOut, "  sign: "))
	verifyOut, verifyRC := swcExec("", "", "codesign", "--verify", "--verbose=2", c.e.binPath())
	c.teeText(swcSed(verifyOut, "  verify: "))
	if verifyRC != 0 {
		c.say("!! 回滚件验签失败 ⇒ 人工介入（这正是 2026-09-16 二次事故的形态 · error.kind=ROLLBACK_FAILED）")
		return 3
	}
	if rc := c.restartMaster(); rc != 0 {
		c.say("!! 回滚后的重启动作失败（见上行）")
	}
	rbCode := c.waitReady(60)
	c.say("  回滚后 /api/capabilities=" + rbCode)
	c.teeText(swcSed(c.identity(), "  rolled "))
	if rbCode == "200" {
		c.say("✓ 已回滚到换件前状态")
		c.say("error.kind=ROLLED_BACK   ← 回滚成功（契约 §十二 P-129 的唯一说法：退码 1 + ROLLED_BACK；不是 0、也不是 2）")
		return 1
	}
	c.say("!! 回滚也没起来 ⇒ 人工介入（error.kind=ROLLBACK_FAILED）")
	return 3
}

// ── 回滚路径自测（抛靶：目标全在 /tmp · **不碰生产件、不碰 launchd 的 com.zerg.core**）─────────────
func (c *swcCtx) selfTestRollback() int {
	t := "/tmp/zerg-swap-selftest-" + c.e.ts
	_ = os.MkdirAll(t, 0o755)
	portOut, _ := swcExec("", "", "python3", "-c",
		`import socket;s=socket.socket();s.bind(("127.0.0.1",0));print(s.getsockname()[1]);s.close()`)
	port := strings.TrimSpace(portOut)
	_ = c.run("cp", "-p", c.e.binPath(), filepath.Join(t, "backup-good")) // 回滚件 = 当前生产件的副本（真签名件）
	// 被换上去的坏件（模拟「新件起不来」）
	_ = os.WriteFile(filepath.Join(t, "zerg-core"), []byte("不是可执行件——模拟「新件起不来」\n"), 0o644)
	srvPath := filepath.Join(t, "stub-server.py")
	_ = os.WriteFile(srvPath, []byte(swcStubServerPy), 0o644)
	srvLog, _ := os.Create(filepath.Join(t, "server.log"))
	srv := exec.Command("python3", srvPath, port)
	srv.Stdout, srv.Stderr = srvLog, srvLog
	_ = srv.Start()
	// 抛靶：把四个可覆盖点全指向 /tmp 靶（✗ 不碰生产件/生产 launchd 作业）
	c.e.bin = filepath.Join(t, "zerg-core")
	c.e.backup = filepath.Join(t, "backup-good")
	c.e.endpoint = "http://127.0.0.1:" + port + "/api/capabilities"
	c.log = filepath.Join(t, "selftest.log")
	_ = os.WriteFile(c.log, nil, 0o644)
	c.stubOn, c.stubText = true, ":"
	c.say("== 回滚路径自测（抛靶 · 不碰生产）· 靶目录 " + t + " ==")
	if st, err := os.Stat(c.e.binPath()); err == nil {
		c.say(fmt.Sprintf("   被换上去的坏件 = %s（%d 字节）", c.e.binPath(), st.Size()))
	}
	if st, err := os.Stat(c.e.backupPath()); err == nil {
		c.say(fmt.Sprintf("   回滚件          = %s（%d 字节）", c.e.backupPath(), st.Size()))
	}
	c.say("   启动动作        = 桩 `:`（内建空命令）· 探针 = 真探针（一次性服务 127.0.0.1:" + port + "）")
	c.say("   回滚前 坏件 sha = " + swcSHA256(c.e.binPath()))
	rc := c.rollbackToBackup()
	// 判据：把「回滚件」按回滚路径**同一条重签命令**处理成期望形态，与回滚后的目标件逐字节对拍。
	//   ★ 为什么要这样比：回滚路径里有 `codesign -s -`（重签），**整文件 sha 与回滚件必然不同**
	//     （ad-hoc 重签会改文件字节）——那是已登记的 T-28 债；ad-hoc 重签实测**确定性**。
	expect := filepath.Join(t, "expected-after-rollback")
	_ = c.run("cp", "-p", c.e.backupPath(), expect)
	_, _ = swcExec("", "", "codesign", "-s", "-", "--force", "--identifier", c.e.label, expect)
	c.say("   回滚后 件  sha  = " + swcSHA256(c.e.binPath()))
	c.say("   回滚件     sha  = " + swcSHA256(c.e.backupPath()) + "   （整件 sha 不同 = 预期：重签改字节）")
	c.say("   期望形态   sha  = " + swcSHA256(expect) + "   （回滚件 + 同一条重签）")
	if swcFilesEqual(c.e.binPath(), expect) {
		c.say("   判据：回滚后目标件 == 「回滚件 + 同一条重签」（逐字节相同 ⇒ 回滚真的把件还原了）✓")
	} else {
		c.say("   判据：回滚后目标件与期望形态不同 ✗")
	}
	if _, vrc := swcExec("", "", "codesign", "--verify", "--verbose=1", c.e.binPath()); vrc == 0 {
		c.say("   判据：回滚后目标件验签通过 ✓")
	} else {
		c.say("   判据：回滚后目标件验签不过 ✗")
	}
	c.say(fmt.Sprintf("   自测退出码 = %d（1 = ROLLED_BACK 语义符合预期）", rc))
	if srv.Process != nil {
		_ = srv.Process.Kill()
	}
	return rc
}

// swcStubServerPy —— 脚本 `--self-test-rollback` 内嵌的一次性 HTTP 服务（逐字）。
const swcStubServerPy = `import sys
from http.server import BaseHTTPRequestHandler, HTTPServer
class H(BaseHTTPRequestHandler):
    def do_GET(self):
        b = b'{"code_sha":"selftest-stub","version":"selftest"}'
        self.send_response(200); self.send_header("Content-Type","application/json")
        self.send_header("Content-Length", str(len(b))); self.end_headers(); self.wfile.write(b)
    def log_message(self, *a): pass
HTTPServer(("127.0.0.1", int(sys.argv[1])), H).serve_forever()
`

// ── 主换件流程（复刻脚本正文）─────────────────────────────────────────────────────────────────────

// swcFilterCodesign 复刻脚本验签那步的 `grep -E '^(Executable|Identifier|Format|CodeDirectory |CandidateCDHash |CDHash=)' | sed 's/^/  /'`。
func swcFilterCodesign(text string) string {
	var sb strings.Builder
	for _, ln := range strings.Split(text, "\n") {
		if strings.HasPrefix(ln, "Executable=") || strings.HasPrefix(ln, "Identifier=") ||
			strings.HasPrefix(ln, "Format=") || strings.HasPrefix(ln, "CodeDirectory ") ||
			strings.HasPrefix(ln, "CandidateCDHash ") || strings.HasPrefix(ln, "CDHash=") {
			sb.WriteString("  " + ln + "\n")
		}
	}
	return sb.String()
}

// runFlow 复刻脚本主正文（前置闸 → 换前现场/备份 → 构建 → 验签 → 重启/等就绪 → 起不来即回滚）。
func (c *swcCtx) runFlow() int {
	c.say("== 主控换件 " + c.e.ts + " ==")
	c.say("日志：" + c.e.log)

	// ── 前置闸（未过 ⇒ 不换件、不动生产）──
	c.hr()
	c.say("[闸] 换件前置闸（未过 ⇒ 不换件、不动生产）")
	dirtyOut := c.execOut("", "git", "status", "--porcelain", "core/", "agent/")
	dirty := 0
	for _, ln := range strings.Split(dirtyOut, "\n") {
		if ln == "" || strings.HasPrefix(ln, "?? ") {
			continue
		}
		dirty++
	}
	c.say(fmt.Sprintf("  闸A 工作树半成品：core/ agent/ 未提交改动数 = %d（须为 0——换件是从工作树编译的）", dirty))
	if dirty != 0 {
		o := c.execOut("", "git", "-c", "core.quotepath=false", "status", "--porcelain", "core/", "agent/")
		var sb strings.Builder
		for _, ln := range strings.Split(o, "\n") {
			if ln == "" || strings.HasPrefix(ln, "?? ") {
				continue
			}
			sb.WriteString("       " + ln + "\n")
		}
		c.teeText(sb.String())
		c.say("!! 有并行半成品 ⇒ 不换件（等它落地再换）")
		return 1
	}
	c.say("  闸B 盘上 plist：" + c.e.plist)
	c.say("      程序串：" + c.plistProgram())
	if missing := c.plistMissingFiles(); missing != "" {
		c.say("!! plist 引用的程序盘上不存在：" + missing)
		c.say("   ⇒ 不换件：kickstart/回滚都会起不来（launchd 用的是**缓存定义**，不只影响这一次）")
		c.say("   修法：bash scripts/build/start-zerg-core.sh 会按当前仓根重写 plist（它生成的形态即真源）；")
		c.say("         改完盘上 plist 后**不用**手工重载——本脚本的重启步会因「已加载定义 ≠ 盘上 plist」自动 bootout+bootstrap")
		return 1
	}
	c.say("  前置闸通过 ✓")

	// ── 0. 换件前现场（必须可回滚）──
	c.hr()
	c.say("[0] 换件前现场")
	before := c.probe()
	c.say("  换件前 /api/capabilities=" + before)
	c.teeText(swcSed(c.identity(), "  before "))
	if rc := c.run("cp", "-p", c.e.binPath(), c.e.backupPath()); rc != 0 {
		c.say("!! 备份失败 ⇒ 不换件")
		return 1
	}
	if st, err := os.Stat(c.e.binPath()); err == nil {
		c.say(fmt.Sprintf("  备份：%s（%d 字节）", c.e.backup, st.Size()))
	}

	// ── 1. 构建（只走 build-all.sh --only-core）──
	c.hr()
	c.say("[1] 构建（go run ./cmd/zerg-build-all --only-core —— 身份注入 + 重签；只写 bin/zerg-core）")
	buildOut, buildRC := swcExec(filepath.Join(c.e.root, "scripts/build"), "", "go", "run", "./cmd/zerg-build-all", "--only-core")
	c.teeText(buildOut)
	if buildRC != 0 {
		c.say("!! 构建失败 ⇒ 未动生产（现场见 " + c.e.log + "）")
		_ = c.run("tail", "-20", c.e.log)
		return 1
	}
	if st, err := os.Stat(c.e.binPath()); err == nil {
		c.say(fmt.Sprintf("  构建完成：%d 字节", st.Size()))
	}

	// ── 2. 验签（门禁②：不过就不换）──
	c.hr()
	c.say("[2] 验签（取数档 = codesign -dvvv ⇒ Signature= / flags= / cdhash 三格同档；输出原样打印，不吞）")
	dvvv, _ := swcExec("", "", "codesign", "-dvvv", c.e.binPath())
	c.teeText(swcFilterCodesign(dvvv))
	verifyOut, verifyRC := swcExec("", "", "codesign", "--verify", "--verbose=2", c.e.binPath())
	c.teeText(verifyOut)
	if verifyRC != 0 {
		c.say("!! 验签失败 ⇒ 不换件")
		return 1
	}
	// ★ 条 10 · `序 121`：**三格齐 且 非 adhoc** 才放行 —— adhoc 件 `--verify` rc=0（实测），只认它就等于负控永不红。
	cells, signRC := c.swcSignThreeCells(c.e.binPath())
	c.say("  " + cells)
	switch signRC {
	case 0:
	case 1:
		c.say("!! 换前验签**判红**（条 10 · 取数档 -dvvv：adhoc / 三格缺）⇒ 不换件")
		return 1
	default:
		c.say("!! 换前验签**判不了** ⇒ 不给结论（不换件 · 退码 2）")
		return 2
	}
	c.say("  验签通过 ✓")

	// ── 3. 换件 + 起 + 验端点 + 自证 ──
	c.hr()
	c.say("[3] 重启主控（见 restart_master：定义不一致则 bootout+bootstrap）+ 等就绪 + 读身份")
	if rc := c.restartMaster(); rc != 0 {
		c.say("!! 重启动作失败（详见上行输出）")
	}
	after := c.waitReady(60)
	c.say("  换件后 /api/capabilities=" + after)
	c.teeText(swcSed(c.identity(), "  after  "))
	if after == "200" {
		c.say("✓ 换件成功（退码 0）")
		return 0
	}

	// ── 4. 起不来 ⇒ 回滚（回滚件**同样验签**）+ 保留现场 ──
	return c.rollbackToBackup()
}

// ── 用法（`--help` 原文即下面这段；**退码与 ROLLED_BACK 的唯一定义处**）──────────────────────────

// swcUsage 逐字返回脚本 `usage()` 的正文（`cat <<'EOF' … EOF` ⇒ 行间 \n ＋ 末尾 \n）。
func swcUsage() string {
	return strings.Join([]string{
		"zerg-swap-core.sh —— 主控换件的受控流程（换件只走本件；构建只走 go run ./cmd/zerg-build-all --only-core）",
		"",
		"用法：",
		"  bash scripts/build/zerg-swap-core.sh                        真换件（前置闸 → 构建 → 验签 → 重启 → 自证）",
		"  bash scripts/build/zerg-swap-core.sh --help                 本页",
		"  bash scripts/build/zerg-swap-core.sh --self-test-rollback    只跑**回滚路径**（抛靶 /tmp 目标，不碰生产）",
		"",
		"前置闸（任一不过 ⇒ 退出码 1 且**未动生产**）：",
		"  闸 A 工作树半成品：`git status --porcelain core/ agent/` 的非未跟踪项计数必须为 0（换件是从工作树编译的）",
		"  闸 B launchd 定义可用：盘上 plist 引用的程序必须真存在（脚本搬家会让那句路径悬空；launchd 用的是**缓存定义**）",
		"",
		"退出码（唯一说法）：",
		"  0  换件成功（新件已上线并自证：端点 200 + 进程自报新 code_sha）",
		"  1  不成功。**两支，靠 error.kind 行分辨**：",
		"       · 未动生产：闸 A/闸 B 不过，或构建失败，或验签失败（那三处都不碰生产件与进程）",
		"       · 回滚成功：**error.kind=ROLLED_BACK** —— 新件起不来，已回滚到换件前状态并重新起来",
		"         （契约 §十二 P-129 定的唯一说法：回滚成功 = `1` + error.kind=ROLLED_BACK；**不是 0，也不是 2**）",
		"  2  用法错（未知旗标；不跑任何步骤）",
		"  3  回滚也失败（error.kind=ROLLBACK_FAILED）⇒ **需人工介入**",
		"",
		"为什么回滚成功是 1 而不是 0/2（照 P-129，不另立理由）：",
		"  · 不是 0：换件**没成功**（用户要的那件事没做成），0 会让调用方以为上线了；",
		"  · 不是 2：命令面主表里 `2` = 用法错/不给结论 —— 拿它表达「回滚成功」就是同数字两义（AI 看数字判不出该做什么），",
		"    这正是批 A · T-04 登记的撞号处；本件按拍板把它收敛到 `1` + 独立 kind。",
	}, "\n") + "\n"
}

// ── 授权侧（`swap` 族三动作共用 · 与底层脚本动作面同构）────────────────────────────────────────────

// swapReadOnlyTier —— `swap` 族三动作（`core`/`agent`/`ui`）的**只读档**判定：
// 带下列任一「只报告」旗标 ⇒ 只读档（**原样透传 · 不进门**）；其余（裸跑 / `--swap` / `--dry-run` /
// `--yes` / 未知旗标）= **写盘档** ⇒ 走 fail-closed 授权门（`guard.go` 的 `writeGuardDryRun` /
// `writeGuardYes`）。
//
// ★ 判据取**透传尾巴原样**（`inv.orig` 去掉 `inv.path`）而不是解析面字段：本族是 `passthrough: true`
// 且验收有「仓外合成夹具**进程内直调** `cmdSwapX`」两路（那条路手工造 `invocation`，解析面字段为空）
// ⇒ 只认尾巴才能在两路上同判。
func swapReadOnlyTier(inv *invocation) bool {
	tail := []string{}
	if len(inv.orig) > len(inv.path) {
		tail = inv.orig[len(inv.path):]
	}
	for _, a := range tail {
		switch a {
		case "--help", "-h", "--check", "--self-test", "--self-test-rollback", "--machine", "--label":
			return true
		}
		if strings.HasPrefix(a, "--machine=") || strings.HasPrefix(a, "--label=") {
			return true
		}
	}
	return false
}

// swapForwardTail —— 取 `zerg swap <动作>` 之后的透传尾巴，并**摘掉**由本层授权门消费掉的旗标
// （`--dry-run` / `--yes`）—— 它们只在本层判，✗ 不交给底层。
func swapForwardTail(inv *invocation) []string {
	tail := []string{}
	if len(inv.orig) > len(inv.path) {
		tail = inv.orig[len(inv.path):]
	}
	out := make([]string, 0, len(tail))
	for _, a := range tail {
		if a == "--dry-run" || a == "--yes" {
			continue
		}
		out = append(out, a)
	}
	return out
}

// ── 执行体（`zerg swap core` 的真搬正门）─────────────────────────────────────────────────────────

// cmdSwapCore —— `zerg swap core [--help | --self-test-rollback] [--dry-run | --yes]`（真搬正门）。
//
// 退码口径（**逐字照底层**）：0 成功 · 1 不成功（未动生产 / 回滚成功两支，靠 error.kind=ROLLED_BACK
// 行分辨）· 2 用法错 · 3 回滚也失败（需人工）。
//
// ★ 授权侧（`GAP-20260927-140` ① · Mr2109 2026-10-08 拍「默认档要 fail-closed」）：写盘档（裸跑 /
// `--swap` / `--dry-run` / `--yes`）走本层授权门 —— `--dry-run` 出计划件（零副作用）；缺 `--yes` 退 2 零写；
// 只读档（`--help`/`-h`/`--self-test-rollback`）原样透传、不进门。本件自己只在「解析不到仓根」那一档退 8。
func cmdSwapCore(inv *invocation, stdout, stderr io.Writer) int {
	root := repoRoot()
	if root == "" {
		inv.setErr("blocked", "repo_root_absent", "解析不到仓根")
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 不给结论（退码 8）\n", progName)
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
	ctx := &swcCtx{e: swcEnvOf(root), out: stdout, errw: stderr}
	switch first {
	case "", "--swap":
		ctx.log = ctx.e.log
		return swcFlowFn(ctx)
	case "--help", "-h":
		fmt.Fprint(stdout, swcUsage())
		return exitOK
	case "--self-test-rollback":
		ctx.log = ctx.e.log
		return ctx.selfTestRollback()
	default:
		fmt.Fprintf(stderr, "✗ 未知参数: %s（用法见：bash scripts/build/zerg-swap-core.sh --help）\n", first)
		return exitUsage
	}
}
