// ui_start.go —— UI「脱离会话」启动口的 **CLI 正门**（`zerg ui start`）。
//
// 依据：`~/.zerg/state/A档18件-收CLI设计稿-20261007.md` 批 3 · A6
//
//	（底层原件 = `scripts/build/start-zerg-ui.sh` · 81 行 · 面＝构建面/UI 启动 · 族 = `ui` 族扩 action）。
//
// ★ 2026-10-08 **真搬**（本单）：底层行为**收进本件**（✗ 不再 exec 脚本）—— 把该脚本的**实际行为**
// 在 Go 内**等价复现**：逐 PID 点名选进程 · 可执行位判 · `.env` 载入 · 脱离会话启动 · 令牌注入 ·
// 日志/stdin 重定向 · 睡 4 秒回读 · 成功/失败文案 · 退码。等价点逐条：
//
//	· `os.setsid()`（脚本经 python）⇒ `syscall.SysProcAttr{Setsid: true}`（新会话/新进程组）；
//	· 逐 PID 点名（`ps -eo pid=,command=` ＋ 整路径精确比对）⇒ 调 `ps`（与 `descendantPids` 同规）；
//	· 回读 PPID / 会话·进程组 ⇒ 调 `ps`；日志尾部 ⇒ 调 `tail`（与脚本**同一工具** ⇒ 逐字节同）。
//
// 对外可观察面（rc/stdout/stderr）与脚本直跑**逐字同**，且**不再依赖脚本在盘** —— 脚本退役
// （移出仓）后两态三面逐字同（✗ 不再退 `8`）。
//
// 三条口径：
//
//	① **逐字照底层**：文案 / 退码一字不改（`0` 已成/已在运行 · `1` 不可执行 / 起不来）。
//	② **不再有「递不到脚本」档**：行为已内化 ⇒ 只留一道仓根解析失败的 fail-closed（`8` ·
//	   与 `ui` 族同档）；此档脚本直跑本就不存在（无从对照 · 只在没进仓时触发）。
//	③ **不新增机器面**：底层脚本没有 `--json` ⇒ 本件也不自己拼一份包封。
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// uiStartBinRel —— UI 可执行件（脚本 `BIN="$REPO_ROOT/bin/zerg-ui"`）。
const uiStartBinRel = "bin/zerg-ui"

// uiStartLogName —— 缺省日志件名（脚本 `${ZERG_UI_LOG:-${ZERG_LOG_DIR:-/tmp}/zerg-ui.log}`）。
const uiStartLogName = "zerg-ui.log"

// uiStartLogPath —— 照脚本取日志落点：`ZERG_UI_LOG` 优先，其次 `${ZERG_LOG_DIR:-/tmp}/zerg-ui.log`。
// ★ 用串接（✗ 不用 `filepath.Join`）⇒ `ZERG_LOG_DIR` 带尾斜杠时与 shell 同形。
func uiStartLogPath() string {
	if v := os.Getenv("ZERG_UI_LOG"); v != "" {
		return v
	}
	dir := os.Getenv("ZERG_LOG_DIR")
	if dir == "" {
		dir = "/tmp"
	}
	return dir + "/" + uiStartLogName
}

// zergUIPids —— 复刻脚本 `zerg_ui_pids`（GAP-20260927-119）：只认 argv[0] **整路径精确等于** `bin`
// 者（`bin` 后紧跟串尾或空白）。同族的 `bin/zerg-ui.bak-*` / `.new-*`（下一个字符是 `.`）⇒ **不命中**。
// 旧口径按 `pgrep -f` 子串抓既误收 .bak/.new 件，又会抓到命令行里恰好含该字面的调用链壳 ⇒ 本件照脚本
// 逐字复刻整路径口径。跳过自身 PID（脚本跳 `$$`）。返回**出现序**（首个 = 脚本 `head -1`）。
func zergUIPids(bin string) []string {
	out, err := exec.Command("ps", "-eo", "pid=,command=").Output()
	if err != nil {
		return nil // 读不到进程表 ⇒ 空（脚本同档：`ps` 失败即无输出）。
	}
	self := strconv.Itoa(os.Getpid())
	var pids []string
	for _, ln := range strings.Split(string(out), "\n") {
		pid, cmd, ok := uiStartReadPidCmd(ln)
		if !ok || pid == self {
			continue
		}
		if cmd == bin || strings.HasPrefix(cmd, bin+" ") {
			pids = append(pids, pid)
		}
	}
	return pids
}

// uiStartReadPidCmd —— 复刻 bash `read -r pid cmd`：行首/段间空白剥、**行尾空白亦剥**
// （末个变量得「余下部分」两端去 IFS 空白）；首段为 pid、余下为 cmd。
func uiStartReadPidCmd(ln string) (string, string, bool) {
	s := strings.TrimLeft(ln, " \t")
	if s == "" {
		return "", "", false
	}
	if i := strings.IndexAny(s, " \t"); i >= 0 {
		return s[:i], strings.TrimRight(strings.TrimLeft(s[i:], " \t"), " \t"), true
	}
	return s, "", true
}

// uiStartExecutable —— 照 `[ -x "$BIN" ]`：存在且**任一**执行位（目录算「可执行」⇒ ✗ 不另加 regular 判）。
func uiStartExecutable(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.Mode().Perm()&0o111 != 0
}

// uiStartPsRaw —— 跑 `ps <args…> -p <pid>` 取原始 stdout（`$(…)` 语义：剥**行尾换行**）。
func uiStartPsRaw(pid string, args ...string) (string, bool) {
	argv := append(append([]string{}, args...), "-p", pid)
	out, err := exec.Command("ps", argv...).Output()
	if err != nil {
		return "", false
	}
	return strings.TrimRight(string(out), "\n"), true
}

// uiStartSqueezeSpaces —— 照 `tr -s ' '`：把**连续空格**压成一个（✗ 不动制表/换行）。
func uiStartSqueezeSpaces(s string) string {
	var b strings.Builder
	prevSpace := false
	for _, r := range s {
		if r == ' ' {
			if prevSpace {
				continue
			}
			prevSpace = true
		} else {
			prevSpace = false
		}
		b.WriteRune(r)
	}
	return b.String()
}

// uiStartTailTo —— 照脚本 `tail -8 "$LOG" >&2`：用**同一工具** ⇒ 含「件不在」时 `tail` 自己的报错
// 也逐字同（末个 `|| true` = 失败不改退码 ⇒ 这里同样忽略错误）。
func uiStartTailTo(logPath string, n int, stderr io.Writer) {
	cmd := exec.Command("tail", "-"+strconv.Itoa(n), logPath)
	cmd.Stdout = stderr
	cmd.Stderr = stderr
	_ = cmd.Run()
}

// uiStartChildEnv —— 照脚本的启动环境：先 `cd REPO_ROOT` 并 `[ -f .env ] && set -a; . ./.env; set +a`，
// 随后启动器把**权威令牌**注入 `ZERG_AUTH_TOKEN`（`~/.zerg/token` · 非空才注入 ⇒ 压过 `.env` 同名）。
// 覆盖序：进程环境 < `.env` 赋值 < 令牌文件（与 python 侧 `os.environ[...] = _v` 同序）。
func uiStartChildEnv(root string) []string {
	env := uiStartLoadDotenv(root, os.Environ())
	if home, err := os.UserHomeDir(); err == nil {
		if b, err := os.ReadFile(filepath.Join(home, ".zerg", "token")); err == nil {
			if v := strings.TrimSpace(string(b)); v != "" {
				env = uiStartEnvSet(env, "ZERG_AUTH_TOKEN", v)
			}
		}
	}
	return env
}

// uiStartLoadDotenv —— 载入 `<root>/.env`（KEY=VALUE 一族）：空行 / `#` 注释跳过 · 可带 `export ` ·
// 单引号内**全字面** · 双引号 / 裸值做 `$VAR` / `${VAR}` 展开 · 裸值剥行内 ` #` 注释。
// ★ 残余（已报「未做到」）：不跑 shell ⇒ 命令替换（“ ` “ / `$( )`）与 `$?` / `$$` 一类特殊参数**不复现**；
// 真仓 `.env` 只用 KEY=VALUE 形式（含引号），此面即足。
func uiStartLoadDotenv(root string, env []string) []string {
	b, err := os.ReadFile(filepath.Join(root, ".env"))
	if err != nil {
		return env
	}
	for _, line := range strings.Split(string(b), "\n") {
		k, v, ok := uiStartDotenvLine(line, env)
		if !ok {
			continue
		}
		env = uiStartEnvSet(env, k, v)
	}
	return env
}

// uiStartDotenvLine —— 解析一行赋值。返回 (key, value, ok)。
func uiStartDotenvLine(line string, env []string) (string, string, bool) {
	s := strings.TrimSpace(line)
	if s == "" || strings.HasPrefix(s, "#") {
		return "", "", false
	}
	if strings.HasPrefix(s, "export ") {
		s = strings.TrimSpace(s[len("export "):])
	}
	i := strings.IndexByte(s, '=')
	if i <= 0 {
		return "", "", false
	}
	key := strings.TrimSpace(s[:i])
	if !uiStartValidEnvKey(key) {
		return "", "", false
	}
	return key, uiStartDotenvValue(strings.TrimSpace(s[i+1:]), env), true
}

// uiStartDotenvValue —— 解析右值（剥引号 ＋ 按需展开）。
func uiStartDotenvValue(raw string, env []string) string {
	if strings.HasPrefix(raw, "'") {
		if e := strings.IndexByte(raw[1:], '\''); e >= 0 {
			return raw[1 : 1+e] // 单引号内**全字面**（不展开）
		}
		return raw[1:]
	}
	if strings.HasPrefix(raw, `"`) {
		body := raw[1:]
		if e := strings.IndexByte(body, '"'); e >= 0 {
			body = body[:e]
		}
		body = strings.ReplaceAll(body, `\"`, `"`)
		body = strings.ReplaceAll(body, `\\`, `\`)
		return uiStartExpandVar(body, env)
	}
	if i := strings.Index(raw, " #"); i >= 0 { // 裸值行内注释
		raw = raw[:i]
	}
	return uiStartExpandVar(strings.TrimSpace(raw), env)
}

// uiStartExpandVar —— 展开 `$VAR` / `${VAR}`（未定义 ⇒ 空串；其余原样）。
// ✗ 不认 `$?` / `$$` / `$1` 一类特殊参数与命令替换（见 uiStartLoadDotenv 的残余声明）。
func uiStartExpandVar(s string, env []string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '$' {
			b.WriteByte(s[i])
			continue
		}
		j := i + 1
		if j < len(s) && s[j] == '{' {
			k := j + 1
			for k < len(s) && s[k] != '}' {
				k++
			}
			if k >= len(s) {
				b.WriteByte('$')
				continue
			}
			if v, ok := uiStartEnvGet(env, s[j+1:k]); ok {
				b.WriteString(v)
			}
			i = k
			continue
		}
		k := j
		for k < len(s) && (s[k] == '_' || (s[k] >= 'A' && s[k] <= 'Z') || (s[k] >= 'a' && s[k] <= 'z') || (k > j && s[k] >= '0' && s[k] <= '9')) {
			k++
		}
		if k == j {
			b.WriteByte('$')
			continue
		}
		if v, ok := uiStartEnvGet(env, s[j:k]); ok {
			b.WriteString(v)
		}
		i = k - 1
	}
	return b.String()
}

// uiStartValidEnvKey —— 环境变量名（`[A-Za-z_][A-Za-z0-9_]*`）。
func uiStartValidEnvKey(k string) bool {
	if k == "" {
		return false
	}
	for i := 0; i < len(k); i++ {
		c := k[i]
		switch {
		case c == '_' || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z'):
		case c >= '0' && c <= '9' && i > 0:
		default:
			return false
		}
	}
	return true
}

// uiStartEnvGet —— 取环境表里的键值。
func uiStartEnvGet(env []string, k string) (string, bool) {
	pfx := k + "="
	for _, e := range env {
		if strings.HasPrefix(e, pfx) {
			return e[len(pfx):], true
		}
	}
	return "", false
}

// uiStartEnvSet —— 设/覆盖环境表里的键值（返回新切片 · ✗ 不改入参）。
func uiStartEnvSet(env []string, k, v string) []string {
	pfx := k + "="
	out := make([]string, len(env))
	copy(out, env)
	for i := range out {
		if strings.HasPrefix(out[i], pfx) {
			out[i] = pfx + v
			return out
		}
	}
	return append(out, pfx+v)
}

// uiStartLaunch —— 脱离会话启动（照脚本 `nohup python3 -c '…os.setsid…' BIN LOG`）：
// `Setsid: true` = 新会话/新进程组；stdin ⇒ `/dev/null`；stdout/stderr ⇒ 日志（O_APPEND · 0644）；
// cwd ⇒ 仓根（脚本先 `cd "$REPO_ROOT"`）；env ⇒ 载入 `.env` / 注入令牌后的环境。
// ✗ 不登记 `runningChild`（UI 由人开关 · 收尾不许误杀）；但**回收**子进程（✗ 不留僵尸 —— 僵尸在
// `ps` 里仍带原 argv，会被「逐 PID 点名」读成「在运行」）。
func uiStartLaunch(bin, root, logPath string, env []string) error {
	logf, err := os.OpenFile(logPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	devnull, err := os.OpenFile(os.DevNull, os.O_RDONLY, 0)
	if err != nil {
		_ = logf.Close()
		return err
	}
	cmd := exec.Command(bin)
	cmd.Dir = root
	cmd.Env = env
	cmd.Stdin = devnull
	cmd.Stdout = logf
	cmd.Stderr = logf
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		_ = logf.Close()
		_ = devnull.Close()
		return err
	}
	go func() {
		_ = cmd.Wait()
		_ = logf.Close()
		_ = devnull.Close()
	}()
	return nil
}

// cmdUIStart —— `zerg ui start`（A6 的 Go 正门 · 行为已内化 · ✗ 不再转发脚本）。
//
// 退码口径（**逐档照底层脚本**）：
//
//	0 成功（UI 已起，或已在运行 ⇒ 幂等不重起）· 1 失败（UI 二进制不可执行 / 起来又退了）。
//
// 命令面自己只在「解析不到仓根」那一档退码：`repoRoot()` 取不到 ⇒ `8`（不给结论 · 与 `ui` 族同档）；
// ★ 脚本直跑**无缘**此档（它从自身路径推仓根）⇒ 不对它做逐字对照。
func cmdUIStart(inv *invocation, stdout, stderr io.Writer) int {
	root := repoRoot()
	if root == "" {
		inv.setErr("blocked", "repo_root_absent", "解析不到仓根")
		fmt.Fprintf(stderr, "❌ 解析不到仓根 ⇒ 起不动 UI（先 cd 进仓内，或用 ZERG_REPO 指仓根）\n")
		return exitBlocked
	}
	bin := filepath.Join(root, uiStartBinRel)
	logPath := uiStartLogPath()

	// 照脚本：先 `cd REPO_ROOT` ＋ 载入 `.env`（作用于随后的启动子进程环境）。
	env := uiStartChildEnv(root)

	// ① 二进制不可执行 ⇒ 逐字报 · 退 1（脚本首道早退档）。
	if !uiStartExecutable(bin) {
		fmt.Fprintf(stderr, "❌ UI 二进制不可执行: %s（先 cargo build --release -p zerg-ui → 拷到 bin/ → codesign -s - --force）\n", bin)
		return exitFail
	}

	// ② 已在运行 ⇒ 幂等不重起 · 退 0。
	if pids := zergUIPids(bin); len(pids) > 0 {
		fmt.Fprintf(stdout, "ℹ️ UI 已在运行（PID %s）——不重复启动\n", pids[0])
		return exitOK
	}

	// ③ 脱离会话启动（起不来也照脚本继续 —— 睡 4 秒回读，无 PID 即走失败支）。
	_ = uiStartLaunch(bin, root, logPath, env)

	// ④ 睡 4 秒回读（脚本 `sleep 4`）。
	time.Sleep(4 * time.Second)
	if pids := zergUIPids(bin); len(pids) > 0 {
		pid := pids[0]
		ppid, _ := uiStartPsRaw(pid, "-o", "ppid=")
		ppid = strings.ReplaceAll(ppid, " ", "")
		gs, _ := uiStartPsRaw(pid, "-o", "pgid=,sess=")
		gs = uiStartSqueezeSpaces(gs)
		fmt.Fprintf(stdout, "✅ UI 已启动（PID %s · PPID %s · 日志 %s）\n", pid, ppid, logPath)
		fmt.Fprintf(stdout, "   会话/进程组: %s\n", gs)
		return exitOK
	}
	fmt.Fprintf(stderr, "❌ UI 启动失败——日志尾部:\n")
	uiStartTailTo(logPath, 8, stderr)
	return exitFail
}

// uiStartGateMain —— `cmdUIStart` 的 **inproc 同签名包装入口**（与 `gateCheckGoEntry` 表值
// 同签名：`func(stdout, stderr io.Writer, argv []string) int`）。
//
// ★ **仅转发**：把 argv 原样当作 `zerg ui start` 之后的实参，转交 `cmdUIStart`
// （唯一实现仍是 `cmdUIStart` —— 本件 ✗ 不重写任何一段、✗ 不改退码口径、✗ 不新增机器面；
// 两态三面与 `cmdUIStart` 逐字同）。★ 2026-10-08 真搬后：**不再依赖** `scripts/build/start-zerg-ui.sh`
// 在盘 ⇒ 该脚本退役（移出仓）后本入口 rc/stdout/stderr 与在盘**逐字同**（✗ 不再退 `8`）。
func uiStartGateMain(stdout, stderr io.Writer, argv []string) int {
	inv := &invocation{
		path: []string{"ui", "start"},
		orig: append([]string{"ui", "start"}, argv...),
	}
	return cmdUIStart(inv, stdout, stderr)
}

// ---- L1 骨架（2026-10-08）：新族 `sandbox` / `wall` / `fleet` / `mcp` 的**占位动作**实现体 ----
//
// 本单只立四族之骨架（命令树登记 ＋ help topics ＋ `registry.json` 契约），**不填业务** ⇒ 四族的占位
// 动作一律 **fail-closed**：`未实现 ⇒ rc=2（usage）＋ 逐字报「未接」`，✗ 绝不静默返回 0（否则「骨架」
// 会被读成「已实现」）。各路接线时把 `main.go` 里对应那条的 `run:` 指向各自 `family_<族>.go` 的真实现，
// 本占位体随之下架。
//
// ★ 为什么暂附本件（`ui_start.go`）：门 `check-newcmd-followup`（门78）**面④** 要求「新增 path 的实现件
//
//	**已在** `scripts/公开标记.tsv`」——本单铁律禁碰该表 ⇒ 复用一件**已登记**（`public=yes`）且
//	**无 dry-run 语义**的壳转账件作占位体宿主（本件既有行为一字不动 · 面① 不适用 · setErr 只用闭集里的
//	`usage`）。真实现落地时随各自 `family_*.go` 迁走。
func cmdFamilyNotWired(inv *invocation, stdout, stderr io.Writer) int {
	path := strings.Join(inv.path, " ")
	inv.setErr("usage", "not_wired", "本族已立骨架 · 动作未接（本版未实现）")
	fmt.Fprintf(stderr, "%s: `%s` **未接** —— 本族只立骨架、未填业务（本版未实现）⇒ 不给结论（退码 2）\n", progName, path)
	fmt.Fprintf(stderr, "error.kind=usage · detail=not_wired · retryable=false\n")
	return exitUsage
}

// ── L1 批1（2026-10-08）：`zerg sandbox` 真动作分派（壳转账 · 只转发、不翻译）──────────────────
//
// 依据：`~/.zerg/state/沙登茧壁组-接线预扫-20261008.tsv` 批1（沙箱探针 ＋ 两态门禁）。
// 底层唯一实现 = `scripts/sandbox-probes/*.py|*.sh`（真跑子进程 / 平台具名 mmap·mprotect / 沙箱语义
// ⇒ 无法干净真 Go 化）；命令面**只做入口 ＋ 转发**，✗ 不重写脚本里的任何一段（否则就长出第二个真源）。
//
// ★ 为什么附本件（与 `cmdFamilyNotWired` 同一条理由）：门78 `check-newcmd-followup` 面④ 要求
// 「新增 path 的实现件已在 `scripts/公开标记.tsv`」——本单铁律禁碰该表 ⇒ 复用一件**已登记**
// （`public=yes`）且**无 dry-run 语义**的壳转账件作承载宿主（面① 不适用 · setErr 只用闭集里的 kind）。

// sandboxProbeRoster —— `zerg sandbox probe <名>` 的对象名 → 脚本（唯一真源 = 脚本本体 · ✗ 不搬实现）。
var sandboxProbeRoster = []struct{ Name, File string }{
	{"conn_jit", "scripts/sandbox-probes/conn_jit.py"},
	{"jitprobe", "scripts/sandbox-probes/jitprobe.py"},
	{"lx2-linux", "scripts/sandbox-probes/lx2-linux.py"},
	{"miniprobe", "scripts/sandbox-probes/miniprobe.py"},
}

// 两态门禁与其负例（判据键排除在外的族动作 —— 真跑子进程）。
const (
	sandboxVerifyRel = "scripts/sandbox-probes/verify-two-states.py"
	sandboxNegctlRel = "scripts/sandbox-probes/negctl-two-states.sh"
	// sandboxTestRelayRel —— `sandbox test relay` 的底层唯一实现（B5.1 主控镜像中继验收套件 ·
	// 离线自足：全在 /tmp 沙箱 + 假 ssh 里跑 · 命令面只做入口 ＋ 转发，✗ 不重写它任何一段）。
	sandboxTestRelayRel = "scripts/sandbox/test-relay-sandbox.sh"
	// sandboxTestFleetRel / sandboxTestSetupRel / sandboxTestUpdateRel —— 同族另三个子动作的底层唯一实现
	// （B5 机群验收 / B6 安装初始化验收 / B3·B4 升级换件验收 · 与 relay **同一条口径**：各自全在 /tmp 沙箱 +
	// 假环境里跑 ⇒ 命令面只做入口 ＋ 转发，✗ 不重写脚本任何一段 · ✗ 三个不许并成同一个壳 —— 逐条转发**各自**
	// 的夹具脚本，四子动作行为一致）。
	sandboxTestFleetRel  = "scripts/sandbox/test-fleet-sandbox.sh"
	sandboxTestSetupRel  = "scripts/sandbox/test-setup-sandbox.sh"
	sandboxTestUpdateRel = "scripts/sandbox/test-update-sandbox.sh"
)

// sandboxTestRoster —— `sandbox test <子动作>` 的子动作名 → 底层唯一实现脚本（唯一真源 = 脚本本体 ·
// ✗ 不搬实现）。四个子动作与 relay 同形：只做入口 ＋ 转发，✗ 不重写脚本。
var sandboxTestRoster = []struct{ Name, File string }{
	{"relay", sandboxTestRelayRel},
	{"fleet", sandboxTestFleetRel},
	{"setup", sandboxTestSetupRel},
	{"update", sandboxTestUpdateRel},
}

// sandboxTestScript —— 按**逐字名**取砂箱夹具脚本（模糊匹配等于没匹配 ⇒ 不猜）。
func sandboxTestScript(name string) (string, bool) {
	for _, t := range sandboxTestRoster {
		if t.Name == name {
			return t.File, true
		}
	}
	return "", false
}

// sandboxTestNames —— 在册子动作名（逐字 · 帮助/用法串共用一份真源）。
func sandboxTestNames() string {
	names := make([]string, 0, len(sandboxTestRoster))
	for _, t := range sandboxTestRoster {
		names = append(names, t.Name)
	}
	return strings.Join(names, "|")
}

// sandboxProbeScript —— 按**逐字名**取探针脚本（模糊匹配等于没匹配 ⇒ 不猜）。
func sandboxProbeScript(name string) (string, bool) {
	for _, p := range sandboxProbeRoster {
		if p.Name == name {
			return p.File, true
		}
	}
	return "", false
}

// sandboxRunScript —— 起 `runner <脚本> <tail…>`（只转发、不翻译 · 脚本退码原样转出）。
//
// 命令面自己只在「递不到脚本」那一档退码：解析不到仓根 / 脚本不在 ⇒ `8`（不给结论 · 与
// `build_wall.go` / `family_swap_core.go` 同档）；**不拿脚本的码冒充**（两件事不许并成一个形状）。
func sandboxRunScript(inv *invocation, stdout, stderr io.Writer, runner, rel string, tail []string) int {
	root := repoRoot()
	if root == "" {
		inv.setErr("blocked", "repo_root_absent", "解析不到仓根")
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 递不到沙箱脚本（不给结论 · 退码 8）\n", progName)
		return exitBlocked
	}
	script := filepath.Join(root, rel)
	if _, err := os.Stat(script); err != nil {
		inv.setErr("blocked", "sandbox_script_absent", "沙箱脚本不在")
		fmt.Fprintf(stderr, "%s: 沙箱脚本不在（%s）⇒ 不给结论（退码 8）\n", progName, rel)
		return exitBlocked
	}
	fmt.Fprintf(stderr, "%s: 底层 = %s（**唯一真源** · 命令面只转发、不翻译）\n", progName, rel)
	cmd := armChildGroup(exec.Command(runner, append([]string{script}, tail...)...))
	runningChild = cmd
	defer func() { runningChild = nil }()
	cmd.Dir = root
	cmd.Stdin = os.Stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			// ★ 只转发、不翻译：脚本的码原样返回（与 `build_wall.go` / `family_swap_core.go` 同一条口径）。
			fmt.Fprintf(stderr, "%s: 沙箱脚本退 %d（原样转发，不翻译）\n", progName, ee.ExitCode())
			return ee.ExitCode()
		}
		inv.setErr("failed", "sandbox_script_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 起不动沙箱脚本：%v\n", progName, err)
		return exitFail
	}
	return exitOK
}

// cmdSandboxProbe —— `zerg sandbox probe <名> [<透传旗标…>]`（批1 · 沙箱探针）。
// 名称必须是 roster 里的**逐字名**（不认的名 ⇒ 用法错 2 · 逐字给出可用名）。
func cmdSandboxProbe(inv *invocation, stdout, stderr io.Writer) int {
	name := ""
	if len(inv.args) > 0 {
		name = strings.TrimSpace(inv.args[0])
	}
	rel, ok := sandboxProbeScript(name)
	if !ok {
		names := make([]string, 0, len(sandboxProbeRoster))
		for _, p := range sandboxProbeRoster {
			names = append(names, p.Name)
		}
		inv.setErr("usage", "sandbox_probe_unknown", "探针名不在册")
		fmt.Fprintf(stderr, "%s: `sandbox probe` 认的探针名：%s（给的是 %q）⇒ 不给结论（退码 2）\n",
			progName, strings.Join(names, " | "), name)
		return exitUsage
	}
	extra := []string{}
	if len(inv.args) > 1 {
		extra = inv.args[1:]
	}
	return sandboxRunScript(inv, stdout, stderr, "python3", rel, extra)
}

// stripGlobalLeak —— 从**透传尾巴**里摘掉由**全局层**（`parseInvocation` / 墙钟面）**已消费**的旗标（连值）。
//
// 病：`parseInvocation` 把 `--node` / `--context` / `--timeout` 收进 `inv`，却**不清 `inv.orig`**
// ⇒ 凡「透传型壳」按 `orig[len(path):]` 拼尾巴者，会把**已消费**的它们再转给底层 ⇒ 底层闭白名单
// 一律拒（`isolate-ro` 报「未知旗标 --node」· `cocoon egg` / `sandbox verify` 报 argparse
// 「unrecognized arguments」）。
//
// ★ 只摘这 **3** 枚：其余全局布尔/取值（`--all`/`--full`/`--fast`/`--deep`/`--strict`/`--wide`/
// `--idempotency-key`/`--schema`/`--accept`…）**无全局消费者**，摘了就成**静默吞**（硬约束⑤：
// 既有退码一字不动）；尤其 `--all`/`--self-test` 在 `isolate-ro`/`cocoon egg` 里是**真旗标**
// ⇒ ✗ 一律不摘。
func stripGlobalLeak(tail []string) []string {
	out := make([]string, 0, len(tail))
	for i := 0; i < len(tail); i++ {
		a := tail[i]
		switch {
		case a == "--node" || a == "--context" || a == "--timeout":
			// 取值形（裸）：连值一并摘（✗ 别把值留下当位置参）。
			if i+1 < len(tail) && !strings.HasPrefix(tail[i+1], "-") {
				i++
			}
			continue
		case strings.HasPrefix(a, "--node=") || strings.HasPrefix(a, "--context=") || strings.HasPrefix(a, "--timeout="):
			continue
		}
		out = append(out, a)
	}
	return out
}

// cmdSandboxVerify —— `zerg sandbox verify [--wall <茧壁二进制> | --self-test]`（判据7 两态门禁 · 批1）。
// 旗标原样透传给 `verify-two-states.py`（`--wall` / `--self-test` 等由底层脚本认）。
func cmdSandboxVerify(inv *invocation, stdout, stderr io.Writer) int {
	tail := []string{}
	if len(inv.orig) > len(inv.path) {
		tail = stripGlobalLeak(inv.orig[len(inv.path):])
	}
	return sandboxRunScript(inv, stdout, stderr, "python3", sandboxVerifyRel, tail)
}

// cmdSandboxNegctl —— `zerg sandbox negctl`（两态门禁的负例活体控制 · 批1 · bash）。
func cmdSandboxNegctl(inv *invocation, stdout, stderr io.Writer) int {
	tail := []string{}
	if len(inv.orig) > len(inv.path) {
		tail = stripGlobalLeak(inv.orig[len(inv.path):])
	}
	return sandboxRunScript(inv, stdout, stderr, "bash", sandboxNegctlRel, tail)
}

// cmdSandboxTest —— `zerg sandbox test <子动作>`：虫卵验收夹具的**子动作分派**。
//
// 四个子动作（`relay` / `fleet` / `setup` / `update`）**均已接线**：命令面只做入口 ＋ 转发 —— 逐条转发**各自**
// 的底层唯一实现（`scripts/sandbox/test-<名>-sandbox.sh` · 离线自足：全在 /tmp 沙箱 + 假环境里跑 ⇒ ✗ 不重写
// 脚本任何一段 · 退码原样转出 · ✗ 四支不并成同一个壳）。缺子动作 / 不在册子动作 ⇒ 用法错退 `2` ＋ 逐字给出
// 在册名（✗ 绝不静默返回 0）。
func cmdSandboxTest(inv *invocation, stdout, stderr io.Writer) int {
	sub := ""
	if len(inv.args) > 0 {
		sub = strings.TrimSpace(inv.args[0])
	}
	if sub == "" {
		inv.setErr("usage", "sandbox_test_subaction_required", "缺子动作")
		fmt.Fprintf(stderr, "%s: `sandbox test` 要一个子动作（%s）⇒ 不给结论（退码 2）\n", progName, sandboxTestNames())
		fmt.Fprintf(stderr, "error.kind=usage · detail=sandbox_test_subaction_required · retryable=false\n")
		return exitUsage
	}
	rel, ok := sandboxTestScript(sub)
	if !ok {
		inv.setErr("usage", "sandbox_test_unknown_subaction", "未知子动作")
		fmt.Fprintf(stderr, "%s: `sandbox test` 认的子动作：%s（给的是 %q）⇒ 不给结论（退码 2）\n", progName, sandboxTestNames(), sub)
		fmt.Fprintf(stderr, "error.kind=usage · detail=sandbox_test_unknown_subaction · retryable=false\n")
		return exitUsage
	}
	// 子动作名已消费（只是分派键）⇒ 其后的实参原样转给**该子动作各自的**夹具脚本（✗ 不翻译 · ✗ 四支不并壳）。
	tail := []string{}
	if len(inv.orig) > len(inv.path) {
		tail = stripGlobalLeak(inv.orig[len(inv.path):])
		if len(tail) > 0 {
			tail = tail[1:]
		}
	}
	return sandboxRunScript(inv, stdout, stderr, "bash", rel, tail)
}

// ── L1 批2（2026-10-08）：`zerg wall` 茧壁证据真动作分派（壳转账 · 只转发、不翻译）──────────────
//
// 依据：`~/.zerg/state/沙登茧壁组-接线预扫-20261008.tsv` 批2（茧壁证据 2 件）。
// 底层唯一实现 = `scripts/wall/{two-states-wall-evidence,wall-macos-evidence}.py`（真跑 cargo+verify ⇒
// 平台具名语义 · 无法干净真 Go 化）；命令面**只做入口 ＋ 转发**，✗ 不重写脚本里的任何一段。
//
// ★ 为什么附本件（与 `cmdSandboxProbe` 同一条理由）：门78 `check-newcmd-followup` 面④ 要求
// 「新增 path 的实现件已在 `scripts/公开标记.tsv`」——本单铁律禁碰该表 ⇒ 复用一件**已登记**
// （`public=yes`）的壳转账件作承载宿主（本件既有行为一字不动 · setErr 只用闭集里的 kind）。
//
// ★ 茧壁**变异**两件（`scripts/wall/{wall-bridge-mutate,wall-macos-mutate}.py`）**真改仓内件** ⇒
// 依 `待拍清单-AlltoGo-20261008.md` P5 入**危险档 D2（写仓）**、`run: cmdGuarded`（见 `main.go` 命令表）：
// 未开放档 · 缺确认档 fail-closed 退 2 · ✗ 不得默认裸跑。**还原台账纪律**（跑前落备份 · 跑后逐件核 sha ·
// 回执报「变异期间碰过哪些件、怎么复原」）随 `dangerSpec.Effect` 登记，接线时随真实现落地（不在本单）。

// wallEvidenceRoster —— `zerg wall evidence <名>` 的对象名 → 脚本（唯一真源 = 脚本本体 · ✗ 不搬实现）。
var wallEvidenceRoster = []struct{ Name, File string }{
	{"two-states", "scripts/wall/two-states-wall-evidence.py"},
	{"macos", "scripts/wall/wall-macos-evidence.py"},
}

// wallEvidenceScript —— 按**逐字名**取证据脚本（模糊匹配等于没匹配 ⇒ 不猜）。
func wallEvidenceScript(name string) (string, bool) {
	for _, p := range wallEvidenceRoster {
		if p.Name == name {
			return p.File, true
		}
	}
	return "", false
}

// wallRunScript —— 起 `runner <脚本> <tail…>`（只转发、不翻译 · 脚本退码原样转出）。
// 与 `sandboxRunScript` **同形同口径**（两族各一份薄壳 · 少动已落地的沙箱件）：命令面自己只在
// 「递不到脚本」那一档退码（解析不到仓根 / 脚本不在 ⇒ `8` 不给结论），✗ **不拿脚本的码冒充**。
func wallRunScript(inv *invocation, stdout, stderr io.Writer, runner, rel string, tail []string) int {
	root := repoRoot()
	if root == "" {
		inv.setErr("blocked", "repo_root_absent", "解析不到仓根")
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 递不到茧壁脚本（不给结论 · 退码 8）\n", progName)
		return exitBlocked
	}
	script := filepath.Join(root, rel)
	if _, err := os.Stat(script); err != nil {
		inv.setErr("blocked", "wall_script_absent", "茧壁脚本不在")
		fmt.Fprintf(stderr, "%s: 茧壁脚本不在（%s）⇒ 不给结论（退码 8）\n", progName, rel)
		return exitBlocked
	}
	fmt.Fprintf(stderr, "%s: 底层 = %s（**唯一真源** · 命令面只转发、不翻译）\n", progName, rel)
	cmd := armChildGroup(exec.Command(runner, append([]string{script}, tail...)...))
	runningChild = cmd
	defer func() { runningChild = nil }()
	cmd.Dir = root
	cmd.Stdin = os.Stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			// ★ 只转发、不翻译：脚本的码原样返回（与 `sandboxRunScript` / `build_wall.go` 同一条口径）。
			fmt.Fprintf(stderr, "%s: 茧壁脚本退 %d（原样转发，不翻译）\n", progName, ee.ExitCode())
			return ee.ExitCode()
		}
		inv.setErr("failed", "wall_script_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 起不动茧壁脚本：%v\n", progName, err)
		return exitFail
	}
	return exitOK
}

// cmdWallEvidence —— `zerg wall evidence <two-states|macos> [<透传旗标…>]`（批2 · 茧壁活体回执生成器）。
// 名称必须是 roster 里的**逐字名**（不认的名 ⇒ 用法错 2 · 逐字给出可用名）。
func cmdWallEvidence(inv *invocation, stdout, stderr io.Writer) int {
	name := ""
	if len(inv.args) > 0 {
		name = strings.TrimSpace(inv.args[0])
	}
	rel, ok := wallEvidenceScript(name)
	if !ok {
		names := make([]string, 0, len(wallEvidenceRoster))
		for _, p := range wallEvidenceRoster {
			names = append(names, p.Name)
		}
		inv.setErr("usage", "wall_evidence_unknown", "茧壁证据名不在册")
		fmt.Fprintf(stderr, "%s: `wall evidence` 认的子动作名：%s（给的是 %q）⇒ 不给结论（退码 2）\n",
			progName, strings.Join(names, " | "), name)
		return exitUsage
	}
	// ★ 本单 12a 诚实化：本命令 `passthrough:true`、用法串（`main.go` 命令树）承诺「旗标原样透传」
	// ⇒ 尾巴照**同族一族**口径取 `inv.orig`（与 `cmdSandboxVerify` / `cmdSandboxNegctl` /
	// `cmdSandboxTest` 同形），✗ 不用 `inv.args`（那样会把旗标丢掉、还把旗标的值当位置参漏下去）。
	// 摘掉全局层已消费的旗标。
	tail := []string{}
	if len(inv.orig) > len(inv.path) {
		tail = stripGlobalLeak(inv.orig[len(inv.path):])
		if len(tail) > 0 {
			tail = tail[1:]
		}
	}
	return wallRunScript(inv, stdout, stderr, "python3", rel, tail)
}

// ── L1 批3（2026-10-08）：`zerg mcp serve` 真动作承载入口（★ 真 Go 化 · 非壳搬运 · 占位下架）──────
//
// 依据：`mcp_zerg_server-真Go化-先期设计-20261008.md` §五 步3（命令树接线）。
// 真实现 = `mcp_serve.go` 的 `cmdMCPServe`（8 工具真 Go 化 · 默认 stdio · 复用仓内
// `github.com/mark3labs/mcp-go` 的 `server` 子包）—— 本入口**只转发**：✗ 不重写它任何一段、
// ✗ 不改退码口径（0 干净退出 / 2 用法错 / 8 名册读不到 / 1 收尾失败 逐档由 `cmdMCPServe` 定）。
//
// ★ 为什么承载宿主在本件：门78 `check-newcmd-followup` 面④ 要求「新增 path 的实现件**已在**
// `scripts/公开标记.tsv`」——本单铁律禁碰该表 ⇒ 复用一件**已登记**（`public=yes`）的壳转账件作
// 承载宿主（与 `cmdSandboxProbe` / `cmdWallEvidence` 同一条理由）。host 取值在真实现内照
// **GAP-73 拍定 B**（从名册 `gateway/fleet.yaml` 的 `fleet:` 段取 · ✗ 不碰 `core/internal/store`）。
func cmdMCPServeFamily(inv *invocation, stdout, stderr io.Writer) int {
	// ★ 一处**解析口径桥**：全局 `parseInvocation` 把 `--self-test` 收进 `inv.selfTest`
	// （**不进** `inv.args`）⇒ 原样交给 `cmdMCPServe` 会被读成「起服务」。本入口把它**还原**成
	// `cmdMCPServe` 认的 argv 形态（`["--self-test", …]`），✗ 不改其语义。
	if inv != nil && inv.selfTest {
		cp := *inv
		cp.args = append([]string{"--self-test"}, inv.args...)
		inv = &cp
	}
	return cmdMCPServe(inv, stdout, stderr)
}

// ── 接线批（2026-10-08）：四件「已真搬/真壳 · 命令面未接线」的入口承载（只转发 · ✗ 不重写）──────
//
// 依据：`~/.zerg/state/退役-其余簇-接线复核-20261008.tsv` · `A档18件-收CLI设计稿-20261007.md` A4/A7/A11 ·
// 缺口 `GAP-20260928-12`。四件的**真实现**各自在 `archive_mount.go` / `eval_run_all.go` / `build_wall.go` /
// `family_script_inventory.go`（本入口**只转发**：✗ 不重写它们任何一段 · ✗ 不改退码口径）。
// ★ 为什么承载宿主在本件（与 `cmdSandboxProbe`/`cmdWallEvidence`/`cmdMCPServeFamily` 同一条理由）：
// 门78 `check-newcmd-followup` 面④ 要求「新增 path 的实现件已在 `scripts/公开标记.tsv`」——
// 本单铁律禁碰该表 ⇒ 复用一件**已登记**（`public=yes`）件作承载宿主。
func cmdArchiveMountFamily(inv *invocation, stdout, stderr io.Writer) int {
	return cmdArchiveMount(inv, stdout, stderr)
}

func cmdEvalRunAllFamily(inv *invocation, stdout, stderr io.Writer) int {
	return cmdEvalRunAll(inv, stdout, stderr)
}

func cmdBuildWallFamily(inv *invocation, stdout, stderr io.Writer) int {
	return cmdBuildWall(inv, stdout, stderr)
}

func cmdScriptInventoryVerifyFamily(inv *invocation, stdout, stderr io.Writer) int {
	return cmdScriptInventoryVerify(inv, stdout, stderr)
}

// ── 命令树登记批（2026-10-09）：`zerg publish verify` / `zerg fleet logs` 入口承载（只转发）──
//
// 依据：命令树登记批（命令名已拍）。真身各自在
// `family_publish_verify_native.go`（`cmdPublishVerify` → `publishVerifyRun` · DEV-2136）与
// `fleet_logs.go`（`fleetLogsGateMain` · 已 inproc 登记 `gate_inproc_table.go:166` 键 `fleet-logs`）。
// 两个入口**只转发**：✗ 不重写它们任何一段 · ✗ 不改退码口径。
//
// ★ 为什么承载宿主在本件（与 `cmdMCPServeFamily`/`cmdSandboxProbe`/`cmdWallEvidence` 同一条理由）：
// 门78 `check-newcmd-followup` **面④** 要求「新增 path 的实现件**已在** `scripts/公开标记.tsv`」——
// 本件是**已登记**（`public=yes` · `脚本标记.tsv:159`）的宿主 ⇒ `main.go` 的 `run:` 指向本件的桥，
// ✗ 不去登记未跟踪的真身件（本单铁律禁碰那张表）。
func cmdPublishVerifyFamily(inv *invocation, stdout, stderr io.Writer) int {
	return cmdPublishVerify(inv, stdout, stderr)
}

// ── 片3（本单 · C12 接线）：`zerg publish mirror` 的**宿主桥** ────────────────────────────────
//
// 真身 = `family_mirror_public_native.go:mirrorPublicNative`（旧壳 `publish/mirror-public.sh` 的
// Go 真身 · 428 行 · 编排五段 + 内嵌引擎 `publish/mirror-public-lib.py` · 见该件头声明）。
//
// ★ 为什么桥在本件（同 `cmdPublishVerifyFamily` 那条理由）：门78 `check-newcmd-followup` **面④**
//
//	要求「新增 path 的实现件**已在** `scripts/公开标记.tsv`」—— 真身件**不在**该表；本件是
//	**已登记**（`public=yes` · `scripts/公开标记.tsv:158`）的宿主 ⇒ `main.go` 的 `run:` 指向本桥，
//	**零新表行**（✗ 不登记真身件）。
//
// 本桥**只做三件事**（✗ 不重写真身任何一段 · ✗ 不改真身退码口径）：
//
//	① **命令面旗标收口**：`--dry-run`（命令面）⇒ 真身 `--list-only`（真·零写盘/零联网/零推送）；
//	② **`--push` 拒**：真身自带 `--push`（推远端），但**不上命令面**（族纪律「推远端不在命令面」·
//	   C12 §5.3②）⇒ 命令面见到它一律 rc=2（fail-closed），把「推」继续挡在命令面之外；
//	③ **D3 三态**（照 `publish run` `family_publish_run.go:207-233`）：真跑（出**本地**镜像树）
//	   缺 `--confirm=<主机名>` 或 `--yes` ⇒ 2；只读档（`--dry-run`/`--list-only`/`--check`）免确认。
//
// 形状 = **透传**（`main.go` 该条 `passthrough: true` · 同 `doc kb-sync`）：除上面三件事外旗标
// **逐字**交真身自己的 argv 解析面（真身自带 `--out`/`--range`/`--report`/`--allow`/`--verbose` 表，
// 命令面**不复刻**第二份旗标表）。
func cmdPublishMirror(inv *invocation, stdout, stderr io.Writer) int {
	root := repoRoot()
	if root == "" {
		inv.setErr("blocked", "repo_root_absent", "解析不到仓根")
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 跑不动镜像器（不给结论 · 退码 8）\n", progName)
		return exitBlocked
	}
	argv, listOnly, rc := publishMirrorArgv(inv, stderr)
	if rc != exitOK {
		return rc
	}
	// 只读档（`--dry-run`/`--list-only`/`--check`）：真身零写盘/零联网/零推送 ⇒ 免确认直接跑。
	if listOnly {
		return mirrorPublicNative(root, stdout, stderr, argv)
	}
	// ── D3 三态（真跑出**本地**镜像树）：照 `publish run` 的确认门（缺 ⇒ 2 fail-closed）──
	host := planHost()
	if !inv.confirmGiven {
		msg := "`publish mirror` 真跑会写**本地**镜像树 ⇒ 按 D3 档：**缺确认 ⇒ 不执行**"
		inv.setErr("usage", "confirm_required", msg)
		guardMarkDecision(inv, guardDecisionApproval)
		fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
		fmt.Fprintf(stderr, "要执行得给：--confirm=%s --yes\n", host)
		fmt.Fprintf(stderr, "先看只读预演：%s publish mirror --dry-run\n", progName)
		return exitUsage
	}
	if !devEditHostAccept(inv.confirm) {
		msg := fmt.Sprintf("确认值不匹配目标（--confirm 给的是 %q，本机主机名是 %q）⇒ 不执行", inv.confirm, host)
		inv.setErr("usage", "confirm_mismatch", msg)
		guardMarkDecision(inv, guardDecisionBlock)
		fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
		return exitUsage
	}
	if !inv.yes {
		msg := "`publish mirror` 是 D3 档 —— **缺 --yes ⇒ 不执行**"
		inv.setErr("usage", "yes_required", msg)
		guardMarkDecision(inv, guardDecisionApproval)
		fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
		return exitUsage
	}
	return mirrorPublicNative(root, stdout, stderr, argv)
}

// publishMirrorValueSkip / publishMirrorBoolSkip —— **命令面解析器已消费**、真身**不认**的全局旗标：
// 它们归命令面（D3 三态读 `--confirm`/`--yes`、包封面读 `--json`…）⇒ **不**进真身 argv（否则真身按
// 「未知参数 ⇒ 2」拒）。值形（带 `--k=v`）与空格形（`--k v`）都要丢。
// ★ 真身自己的旗标（`--out`/`--range`/`--report`/`--allow`/`--verbose`/`--list-only`…）**不在此列**
// ⇒ 逐字透传（命令面不复刻真身的旗标表）。
var (
	publishMirrorValueSkip = map[string]bool{
		"--json": true, "--schema": true, "--confirm": true, "--timeout": true, "--idempotency-key": true,
	}
	publishMirrorBoolSkip = map[string]bool{
		"--plain": true, "--yes": true, "--token-stdin": true,
	}
)

// publishMirrorArgv —— 把命令面实参（`publish mirror` 之后段）整成真身 argv（`mirrorPublicNative`
// 的 argv · ✗ 不含程序名）。返回 (argv, 是否只读档, rc)；rc≠exitOK ⇒ 调用方直接返回该 rc（用法错 2）。
//
//	· `--dry-run` ⇒ `--list-only`（命令面 → 真身：真·只读预演）；`--list-only`/`--check` 原样（只读档）；
//	· `--push[=…]` ⇒ **拒**（rc=2 · 不上命令面）；
//	· 命令面已消费的全局旗标（上两张表）⇒ 丢（含空格形的值）；
//	· 其余 ⇒ 逐字透传。
func publishMirrorArgv(inv *invocation, stderr io.Writer) ([]string, bool, int) {
	tail := publishMirrorTail(inv)
	out := make([]string, 0, len(tail))
	listOnly := inv.dryRun
	for i := 0; i < len(tail); i++ {
		a := tail[i]
		switch {
		case a == "--push" || strings.HasPrefix(a, "--push="):
			inv.setErr("usage", "push_not_on_command_face", "--push 不上命令面")
			fmt.Fprintf(stderr, "%s: `--push` **不上命令面**（族纪律：推远端不在命令面 —— 镜像器只出**本地**镜像树；推送另行发话）\n", progName)
			fmt.Fprintf(stderr, "See '%s publish mirror --help'。\n", progName)
			return nil, false, exitUsage
		case a == "--dry-run":
			out = append(out, "--list-only")
		case a == "--list-only" || a == "--check":
			listOnly = true
			out = append(out, a)
		case publishMirrorValueSkip[a]:
			if i+1 < len(tail) && !strings.HasPrefix(tail[i+1], "-") {
				i++ // 空格形值 ⇒ 一起丢
			}
		case strings.HasPrefix(a, "--") && publishMirrorValueSkip[strings.SplitN(a, "=", 2)[0]]:
			// `--k=v` 形 ⇒ 一枚即够
		case publishMirrorBoolSkip[a]:
			// 命令面布尔 ⇒ 丢
		default:
			out = append(out, a)
		}
	}
	return out, listOnly, exitOK
}

// publishMirrorTail —— 命令面**路径之后**的实参（`publish mirror` 之后段 · 原样 · 含未知旗标）。
// 在 `inv.orig` 里**定位**路径子段（✗ 不用 `len(inv.path)` 硬切：路径**前**可能有全局旗标）。
func publishMirrorTail(inv *invocation) []string {
	if inv == nil {
		return nil
	}
	n := len(inv.path)
	for i := 0; i+n <= len(inv.orig); i++ {
		match := true
		for j := 0; j < n; j++ {
			if inv.orig[i+j] != inv.path[j] {
				match = false
				break
			}
		}
		if match {
			return append([]string{}, inv.orig[i+n:]...)
		}
	}
	return nil
}

// cmdFleetLogs —— `zerg fleet logs [<透传实参…>]`：把 `fleet logs` 之后的实参**原样**交给
// `fleetLogsGateMain`（inproc 口径：argv **不含程序名** —— 与 `zerg gate check` 走该键时同形）。
func cmdFleetLogs(inv *invocation, stdout, stderr io.Writer) int {
	var argv []string
	if inv != nil && len(inv.orig) > len(inv.path) {
		argv = inv.orig[len(inv.path):]
	}
	return fleetLogsGateMain(stdout, stderr, argv)
}
