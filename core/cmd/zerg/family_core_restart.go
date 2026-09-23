// family_core_restart.go —— `zerg core restart`（`Q-103` · D3 族）的执行体。
//
// 病根（设计稿 `设计-命令面-主控重启-20260923.md` §〇 逐字）：「重启这件事今天**唯一**的实现是
// `scripts/build/zerg-swap-core.sh` 里的 `restart_master()`，可它与用户之间隔着一层 shell ⇒
// 2026-09-23 那次真事是**手敲** `launchctl kickstart -k gui/501/com.zerg.core`」——与自家教条
// 「命令面是唯一入口、不拼脚本」直接相悖。
//
// 本件的口径（照设计稿，不自造）：
//
//	① **不新增命令条目**：接在 `main.go` 已登记的 `core restart` 那一条上（`run` 换执行体 +
//	   `opened` 翻真）。危险档仍是 `dangerD3`（**不降档**）⇒ `--confirm=<主机名>` 与 `--yes`
//	   必须**同时到**（`guard.go:22` 的档位定义 · §十二 `P-014`）。
//	② **三态**：`--dry-run` ⇒ 八格计划件 + **rc=0**（零副作用：不执行、不落盘、不写审计）；
//	   缺 `--confirm` / 值不匹配 / 缺 `--yes` ⇒ fail-closed **rc=2**（从不提问、也从不偷偷重启）；
//	   确认档齐 ⇒ 真跑。
//	③ **真跑动作只有一个**：`launchctl kickstart -k gui/<uid>/com.zerg.core`（设计稿 §一.5 里
//	   `restart_master()` 的**定义一致**那一支）。**定义不一致 / 未装载**这两支**不在 Go 里重写**
//	   ——设计稿 §3.3 逐字「**不许**在 Go 里另写一遍 `bootout`/`bootstrap`/`kickstart` 顺序」⇒
//	   本件一律**不给结论（rc=8）**并指向那唯一入口 `scripts/build/zerg-swap-core.sh`。
//	④ **就绪判据必须绑自己的 pid**（设计稿 §五 · `family_port.go:3–4` 的血泪）：
//	   ① 作业在（`launchctl print` rc=0）② 三个端口（8580/8581/8082）的**属主 pid 等于新 pid**
//	   ③ 那个 pid 真的在跑且 `lstart` 晚于干跑记下的旧 `lstart`。三子句齐才算就绪；
//	   `lsof` / `ps` 取不到 ⇒ **rc=8**（拿不到就写拿不到，不许假绿）。HTTP 200 只作辅助、**单独不算就绪**。
//	⑤ **审计留痕**（设计稿 §七）：真跑前后各一行 JSON，落
//	   `ZERG_CORE_AUDIT` > `<ZERG_STATE_DIR>/core_restart_audit.jsonl` > `~/.zerg/state/core_restart_audit.jsonl`
//	   （**单开一件**：`edit_audit.jsonl` 的语义是「改件」）—— 审计写不进去 ⇒ **不动进程**（rc=8）。
//
// 红线（本波）：**绝不真重启主控**。执行动作与就绪等待各留一枚**函数级接缝**
// （`coreRestartAction` / `coreRestartAuditPath` …）+ 一枚自测桩环境变量 `ZERG_CORE_RESTART_STUB`
// ——后者的形态照 `scripts/build/zerg-swap-core.sh` 的 `ZERG_SWAP_RESTART_STUB` **同形**（那件也是
// 只在自测档设它）。本波的全部「真跑」证据都在这两个接缝上做，真 `kickstart` **一次都没发生**。
//
// 退码：0 干跑计划件 / 真跑且就绪三子句齐 · 1 动作失败或就绪不过 · 2 用法错（缺确认档）·
//
//	8 拿不到结论（定义不一致 / 审计落不下 / `lsof`·`ps` 取不到 / 仓根解析不到）。
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	coreRestartLabel   = "com.zerg.core"
	coreRestartPlistIn = "Library/LaunchAgents/com.zerg.core.plist" // `~/` 之下
	coreRestartAudit   = "core_restart_audit.jsonl"                 // `<状态目录>/` 之下
	coreRestartBinRel  = "bin/zerg-core"                            // 身份面：二进制件（本命令**不换件**）
)

// coreRestartPorts —— 主控的**三名端口**（设计稿 §1.8 那张现读表；重启命令要预告、也要复核它们）。
var coreRestartPorts = []string{"8580", "8581", "8082"}

// ── 函数级接缝（测试/自测桩在**进程内**替换它们 ⇒ 判据能在「不真重启」的前提下端到端跑）──────────
var (
	// coreRestartAction 真跑动作（默认 = `launchctl kickstart -k <域>/<标签>`）。
	coreRestartAction = coreRestartKickstart
	// coreRestartSleep 等就绪用的睡眠（测试换掉 ⇒ 不真等）。
	coreRestartSleep = time.Sleep
	// coreRestartReadyWait 等就绪的最长秒数（`zerg-swap-core.sh` 的 `wait_ready` 是同一种形状）。
	coreRestartReadyWait = 20 * time.Second
)

// coreRestartDomain —— launchd 域（本机用户域 `gui/<uid>`；2026-09-23 那次真事走的就是 `gui/501`）。
func coreRestartDomain() string { return fmt.Sprintf("gui/%d", os.Getuid()) }

// coreRestartServiceTarget —— `域/标签`（`launchctl` 的目标写法，逐字可复制）。
func coreRestartServiceTarget() string {
	return coreRestartDomain() + "/" + coreRestartLabel
}

// coreRestartPlistPath —— 盘上 plist 的绝对路径（`~/…` 展开；拿不到家目录 ⇒ 空）。
func coreRestartPlistPath() string {
	h, err := os.UserHomeDir()
	if err != nil || h == "" {
		return ""
	}
	return filepath.Join(h, filepath.FromSlash(coreRestartPlistIn))
}

// coreRestartAuditPath —— 审计落点（`ZERG_CORE_AUDIT` > `<状态目录>/core_restart_audit.jsonl`）。
func coreRestartAuditPath() string {
	if d := strings.TrimSpace(os.Getenv("ZERG_CORE_AUDIT")); d != "" {
		return d
	}
	return filepath.Join(stateDirOf(), coreRestartAudit)
}

// coreRestartAuditLine —— 审计的一行（一行一事件 · 追加只写 · **失败即拒**；形状照 `editAuditLine`）。
type coreRestartAuditLine struct {
	At           string `json:"at"`
	Event        string `json:"event"`
	By           string `json:"by"`
	Confirm      string `json:"confirm"`
	Label        string `json:"label"`
	Domain       string `json:"domain"`
	PidBefore    string `json:"pid_before,omitempty"`
	LstartBefore string `json:"lstart_before,omitempty"`
	PidAfter     string `json:"pid_after,omitempty"`
	LstartAfter  string `json:"lstart_after,omitempty"`
	Branch       string `json:"branch,omitempty"`
	Ports        string `json:"ports,omitempty"`
	BinarySHA256 string `json:"binary_sha256,omitempty"`
	BinaryMtime  string `json:"binary_mtime,omitempty"`
	Readiness    string `json:"readiness,omitempty"`
	RC           int    `json:"rc"`
	AuditPath    string `json:"audit_path"`
	Note         string `json:"note,omitempty"`
}

// appendCoreRestartAudit 追加一行（O_APPEND · 一行一事件）。**失败即拒**（调用方据此不动进程）。
func appendCoreRestartAudit(path string, line coreRestartAuditLine) error {
	if path == "" {
		return fmt.Errorf("审计落点解析不出来（HOME / ZERG_STATE_DIR 都取不到）")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	body, err := json.Marshal(line)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(append(body, '\n')); err != nil {
		return err
	}
	return f.Sync()
}

// ── 现值读数（全部只读：`launchctl list` / `launchctl print` / `plutil` / `lsof` / `ps`）────────

// coreRestartJobPID 读 `launchctl list` 里本标签那一行的 pid（没装载 ⇒ 空）。
func coreRestartJobPID() (string, string) {
	out, err := exec.Command("launchctl", "list").Output()
	if err != nil {
		return "", fmt.Sprintf("`launchctl list` 跑不起来：%v", err)
	}
	for _, ln := range strings.Split(string(out), "\n") {
		f := strings.Fields(ln)
		if len(f) >= 3 && f[2] == coreRestartLabel {
			return f[0], ""
		}
	}
	return "", ""
}

// coreRestartLoaded 判作业装载 + 取已加载定义里的 `arguments = {…}`（缓存的那一份）。
func coreRestartLoaded() (loaded bool, program string) {
	out, err := exec.Command("launchctl", "print", coreRestartServiceTarget()).Output()
	if err != nil {
		return false, ""
	}
	txt := string(out)
	if strings.Contains(txt, "Could not find service") {
		return false, ""
	}
	re := regexp.MustCompile(`(?s)arguments = \{(.*?)\}`)
	m := re.FindStringSubmatch(txt)
	if m == nil {
		return true, ""
	}
	parts := []string{}
	for _, x := range strings.Split(m[1], "\n") {
		if s := strings.TrimSpace(x); s != "" {
			parts = append(parts, s)
		}
	}
	return true, strings.Join(parts, " ")
}

// coreRestartPlistProgram 读盘上 plist 的 ProgramArguments（`plutil -extract … json`，逐字取）。
func coreRestartPlistProgram() (string, string) {
	p := coreRestartPlistPath()
	if p == "" {
		return "", "家目录取不到 ⇒ 盘上 plist 路径解析不出来"
	}
	if _, err := os.Stat(p); err != nil {
		return "", fmt.Sprintf("盘上 plist 不在（%s）", coreRestartTildePath(p))
	}
	out, err := exec.Command("plutil", "-extract", "ProgramArguments", "json", "-o", "-", p).Output()
	if err != nil {
		return "", fmt.Sprintf("`plutil` 读不了 plist：%v", err)
	}
	var arr []string
	if err := json.Unmarshal(out, &arr); err != nil {
		return "", fmt.Sprintf("plist 的 ProgramArguments 不是字符串数组：%v", err)
	}
	return strings.Join(arr, " "), ""
}

// coreRestartTildePath 把绝对路径写成 `~/…`（只为报面短一点；家目录取不到 ⇒ 原样）。
func coreRestartTildePath(p string) string {
	h, err := os.UserHomeDir()
	if err != nil || h == "" {
		return p
	}
	if strings.HasPrefix(p, h+string(os.PathSeparator)) {
		return "~" + p[len(h):]
	}
	return p
}

// coreRestartBranch 判「将走哪一条分支」（**只读**两读数；拿不到 ⇒ why 非空）。
//
//	kickstart-k        已加载定义 == 盘上 plist（本版**唯一**会自动真跑的那一支）
//	bootout+bootstrap  已加载定义 ≠ 盘上 plist（kickstart 用缓存定义会起不来 —— 老鬼）
//	bootstrap          launchd 里没有这个作业（首次装载）
func coreRestartBranch() (branch, why string) {
	loaded, lp := coreRestartLoaded()
	if !loaded {
		return "bootstrap", ""
	}
	dp, perr := coreRestartPlistProgram()
	if perr != "" {
		return "", perr
	}
	if lp != dp {
		return "bootout+bootstrap", ""
	}
	return "kickstart-k", ""
}

// coreRestartPortOwner 读某个端口 LISTEN 的属主（`lsof -F pc` ⇒ `p<pid>` / `c<命令>`）。
// 拿不到 `lsof` 本身 ⇒ why 非空（**不给结论**，照 `family_port.go` 的现成口径）。
func coreRestartPortOwner(port string) (pid, cmd, why string) {
	out, err := exec.Command("lsof", "-nP", "-iTCP:"+port, "-sTCP:LISTEN", "-F", "pc").Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 1 {
			return "", "", "" // rc=1 = 没有占用者（不是「读不到」）
		}
		return "", "", fmt.Sprintf("`lsof` 跑不起来（端口 %s）：%v", port, err)
	}
	for _, ln := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(ln, "p") {
			pid = strings.TrimSpace(ln[1:])
		}
		if strings.HasPrefix(ln, "c") {
			cmd = strings.TrimSpace(ln[1:])
		}
		if pid != "" && cmd != "" {
			break
		}
	}
	return pid, cmd, ""
}

// coreRestartProcLstart 读某 pid 的起时（`ps -p <pid> -o lstart=`；读不到 ⇒ why 非空）。
func coreRestartProcLstart(pid string) (string, string) {
	if strings.TrimSpace(pid) == "" {
		return "", "pid 为空 ⇒ 读不了起时"
	}
	out, err := exec.Command("ps", "-p", pid, "-o", "lstart=").Output()
	if err != nil {
		return "", fmt.Sprintf("`ps -p %s` 读不到：%v", pid, err)
	}
	s := strings.TrimSpace(string(out))
	if s == "" {
		return "", fmt.Sprintf("`ps -p %s` 没有输出（进程不在了）", pid)
	}
	return s, ""
}

// coreRestartBinaryIdentity 读 `bin/zerg-core` 的身份（sha256 前 16 + mtime；**本命令不换件**）。
func coreRestartBinaryIdentity() (sha, mtime string) {
	root := repoRoot()
	if root == "" {
		return "", ""
	}
	p := filepath.Join(root, filepath.FromSlash(coreRestartBinRel))
	if st, err := os.Stat(p); err == nil {
		mtime = st.ModTime().Format("2006-01-02 15:04:05")
	}
	if s := fileSHA256(p); s != "" {
		sha = shortSHA(s)
	}
	return sha, mtime
}

// ── 计划件（`--dry-run` 的八格 · 设计稿 §四）──────────────────────────────────────────────────

// coreRestartPlan 渲染计划件的八格（不新造字段：逐格都是现成读数）。
func coreRestartPlan(inv *invocation, stdout io.Writer) {
	pid, _ := coreRestartJobPID()
	lstart, lw := coreRestartProcLstart(pid)
	loaded, lp := coreRestartLoaded()
	branch, bwhy := coreRestartBranch()
	sha, mtime := coreRestartBinaryIdentity()

	fmt.Fprintln(stdout, "计划件（--dry-run · 零副作用 —— 未执行、未改任何状态、**未写审计**）")
	fmt.Fprintf(stdout, "  动作     : %s core restart（停 + 起主控）\n", progName)
	fmt.Fprintf(stdout, "  危险档   : D3（不可逆：**整个虫群的控制面会断一会儿**）\n")
	fmt.Fprintf(stdout, "  执行要   : --confirm=<主机名> 与 --yes 同时到（`--confirm` 值须与本机主机名逐字相同：%s）\n", planHost())
	fmt.Fprintf(stdout, "  ① 将重启 : pid=%s 起时=%s 目标=%s\n", orX(pid, "未装载"), orX(lstart, lw), coreRestartLabel)
	fmt.Fprintf(stdout, "  ② launchd : label=%s domain=%s loaded=%s\n", coreRestartLabel, coreRestartDomain(),
		map[bool]string{true: "yes", false: "no"}[loaded])
	if bwhy != "" {
		fmt.Fprintf(stdout, "  ③ 将走分支: branch=不判（%s）\n", bwhy)
	} else {
		fmt.Fprintf(stdout, "  ③ 将走分支: branch=%s%s\n", branch, coreRestartBranchNote(branch))
	}
	fmt.Fprintf(stdout, "  ④ 受影响端口: %s\n", coreRestartPortsLine())
	fmt.Fprintf(stdout, "  ⑤ 受影响服务: %s\n", coreRestartServicesLine(inv))
	fmt.Fprintf(stdout, "  ⑥ 二进制身份: 件=%s sha256=%s mtime=%s\n", coreRestartBinRel, orX(sha, "（读不到）"), orX(mtime, "（读不到）"))
	fmt.Fprintf(stdout, "  ⑦ 会不会换件: 换件=否（本命令只重启进程 —— 换件是 `scripts/build/zerg-swap-core.sh` 的另一件事）\n")
	fmt.Fprintf(stdout, "  ⑧ 出口语句 : %s\n", coreRestartExitLine(branch))
	fmt.Fprintf(stdout, "  已加载定义: %s\n", orX(lp, "（读不到）"))
	if dp, perr := coreRestartPlistProgram(); perr == "" {
		fmt.Fprintf(stdout, "  盘上 plist: %s\n", dp)
	} else {
		fmt.Fprintf(stdout, "  盘上 plist: （%s）\n", perr)
	}
	fmt.Fprintf(stdout, "  可逆性   : 能（重启本身可逆：再跑一次本命令即可回到「进程在跑」态）；但**进程重启不可撤销**（起时/日志/中断的连接都回不来）\n")
	fmt.Fprintf(stdout, "  来源     : §三 C 族 · 开工单 T-45 · 缺口 `Q-103`（设计稿 §四）\n")
}

// coreRestartBranchNote 给分支那一格加一句它是干什么的（判据可读性）。
func coreRestartBranchNote(branch string) string {
	switch branch {
	case "kickstart-k":
		return "（已加载定义 == 盘上 plist ⇒ 本版**唯一**会自动真跑的那一支）"
	case "bootout+bootstrap":
		return "（已加载定义 ≠ 盘上 plist ⇒ **本版不自动走**：换件入口 `scripts/build/zerg-swap-core.sh` 才是它的落点）"
	case "bootstrap":
		return "（launchd 里没有这个作业 ⇒ **本版不自动走**：同上，归换件入口）"
	}
	return ""
}

// coreRestartExitLine 出口语句（照分支给逐字可复制的形态）。
func coreRestartExitLine(branch string) string {
	if branch == "kickstart-k" {
		return "launchctl kickstart -k " + coreRestartServiceTarget()
	}
	return "（本版不自动执行这一支）→ bash scripts/build/zerg-swap-core.sh（重启+换件的唯一入口）"
}

// coreRestartPortsLine 逐口给属主（`8580 ← pid 81385（本命令目标）`）。
func coreRestartPortsLine() string {
	pid, _ := coreRestartJobPID()
	parts := []string{}
	for _, p := range coreRestartPorts {
		op, _, why := coreRestartPortOwner(p)
		switch {
		case why != "":
			parts = append(parts, fmt.Sprintf("%s ← （读不到：%s）", p, why))
		case op == "":
			parts = append(parts, fmt.Sprintf("%s ← （无占用者）", p))
		case op == pid && pid != "":
			parts = append(parts, fmt.Sprintf("%s ← pid %s（本命令目标）", p, op))
		default:
			parts = append(parts, fmt.Sprintf("%s ← pid %s（**不是本命令目标**）", p, op))
		}
	}
	return strings.Join(parts, " · ")
}

// coreRestartServicesLine 读**声明树**分三列（受影响 / 不受影响 / 不属管辖）—— 照设计稿 §四 ⑤。
func coreRestartServicesLine(inv *invocation) string {
	path, perr := svcDeclPath(inv)
	if perr != "" {
		return "（读不到声明树：" + perr + "）"
	}
	decl, derr := loadSvcDecl(path)
	if derr != "" {
		return "（读不到声明树：" + derr + "）"
	}
	affected, untouched, outside := []string{}, []string{}, []string{}
	for _, r := range decl {
		if r.Kind == "launchd" {
			if r.Name == coreRestartLabel {
				affected = append(affected, fmt.Sprintf("%s（kind=launchd · %s）", r.Owner, r.Name))
			} else {
				untouched = append(untouched, fmt.Sprintf("%s %s", r.Owner, r.Name))
			}
			continue
		}
		if r.Kind == "ghost" {
			pid := coreRestartGhostPID(r.ProcFeature)
			outside = append(outside, fmt.Sprintf("%s（kind=ghost · %s）", r.Name, orX(pid, "现值未命中")))
		}
	}
	out := []string{}
	out = append(out, "受影响："+orDashList(affected))
	out = append(out, "不受影响："+orDashList(untouched))
	out = append(out, "不属管辖："+orDashList(outside))
	return strings.Join(out, " · ")
}

// coreRestartGhostPID 在现值 `ps` 里找某枚进程特征命中的 pid（只读；找不到 ⇒ 空）。
func coreRestartGhostPID(feature string) string {
	if feature == "" || feature == "-" {
		return ""
	}
	lines, why := psLines()
	if why != "" {
		return ""
	}
	for _, ln := range lines {
		if !strings.Contains(ln, feature) {
			continue
		}
		if f := strings.Fields(ln); len(f) > 0 {
			return f[0]
		}
	}
	return ""
}

// ── 就绪判据（三子句 · 必须绑自己的 pid）─────────────────────────────────────────────────────

// coreRestartWaitReady 等就绪并判三子句 ⇒ (就绪?, 判词, 退码)。
// 退码 8 = 拿不到结论（`lsof` 跑不起来 / 新 pid 读不到起时）；1 = 等到了但没就绪。
func coreRestartWaitReady(oldPid, oldLstart string) (bool, string, int) {
	waited := time.Duration(0)
	for {
		ok, det, rc := coreRestartReadyOnce(oldPid, oldLstart)
		if ok {
			return true, det, 0
		}
		if rc == exitBlocked {
			return false, det, exitBlocked
		}
		if waited >= coreRestartReadyWait {
			return false, det, exitFail
		}
		coreRestartSleep(2 * time.Second)
		waited += 2 * time.Second
	}
}

// coreRestartReadyOnce 判一次三子句 ⇒ (就绪?, 判词, 退码)。退码非 0 是**终局**（不给结论或红）。
func coreRestartReadyOnce(oldPid, oldLstart string) (bool, string, int) {
	// ① 作业在
	loaded, _ := coreRestartLoaded()
	if !loaded {
		return false, "① 作业不在 launchd 里（`launchctl print` 不认这个作业）", 0
	}
	newPid, _ := coreRestartJobPID()
	if newPid == "" {
		return false, "① 作业在但 `launchctl list` 里没有 pid（正在起重启窗口）", 0
	}
	// ③ 那个 pid 真的在跑且起时晚于旧起时（先判它，②才不是「老进程还占着端口」的假绿）
	lstart, why := coreRestartProcLstart(newPid)
	if why != "" {
		return false, "③ " + why, exitBlocked
	}
	if oldLstart != "" && lstart == oldLstart && newPid == oldPid {
		return false, fmt.Sprintf("③ 进程还是旧的（pid=%s 起时=%s 与重启前逐字相同）", newPid, lstart), 0
	}
	// ② 三个端口的属主 pid 都是新 pid
	bad := []string{}
	for _, p := range coreRestartPorts {
		op, _, why := coreRestartPortOwner(p)
		if why != "" {
			return false, fmt.Sprintf("② %s（端口 %s）", why, p), exitBlocked
		}
		if op == "" {
			return false, fmt.Sprintf("② 端口 %s **没有属主**（没起来）", p), 0
		}
		if op != newPid {
			bad = append(bad, fmt.Sprintf("%s 属主是 pid %s", p, op))
		}
	}
	if len(bad) > 0 {
		return false, "② " + strings.Join(bad, " · "), 0
	}
	return true, fmt.Sprintf("三子句齐：作业在 · 8580/8581/8082 属主都是 pid=%s · 起时=%s（晚于重启前的 %s）",
		newPid, lstart, orX(oldLstart, "（未记录）")), 0
}

// ── 真跑动作（默认实现；自测桩见 coreRestartAction 与 `ZERG_CORE_RESTART_STUB`）───────────────

// coreRestartKickstart —— `launchctl kickstart -k <域>/<标签>`（设计稿 §一.5 的 kickstart-k 那一支）。
//
// ★ 自测桩：`ZERG_CORE_RESTART_STUB` 非空 ⇒ 只跑那一串（照 `zerg-swap-core.sh` 的
// `ZERG_SWAP_RESTART_STUB` 同形，本件**只在测试/探针里设它**）——不真重启。
func coreRestartKickstart() error {
	if stub := strings.TrimSpace(os.Getenv("ZERG_CORE_RESTART_STUB")); stub != "" {
		fmt.Fprintf(os.Stderr, "%s: [重启] 自测桩：%s（**未真重启**）\n", progName, stub)
		out, err := exec.Command("bash", "-c", stub).CombinedOutput()
		if len(out) > 0 {
			fmt.Fprint(os.Stderr, string(out))
		}
		return err
	}
	cmd := exec.Command("launchctl", "kickstart", "-k", coreRestartServiceTarget())
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}

// ── 执行体 ───────────────────────────────────────────────────────────────────────────────────

// cmdCoreRestart —— `zerg core restart --confirm=<主机名> --yes [--dry-run]`（D3 · 三态）。
func cmdCoreRestart(inv *invocation, stdout, stderr io.Writer) int {
	host := planHost()

	// ① `--dry-run`：计划件 · 零副作用 —— 三态里**唯一会返回 0** 的那一态（§九 M3 C4）。
	if inv.dryRun {
		coreRestartPlan(inv, stdout)
		fmt.Fprintln(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未执行、未改任何状态、未写审计）")
		return exitOK
	}

	// ② D3 确认档：`--confirm=<主机名>` 与 `--yes` **同时到**（`guard.go:22` 的档位定义 · §十二 P-014）。
	if !inv.confirmGiven {
		msg := fmt.Sprintf("`core restart` 是 D3 档（不可逆：停 + 起主控）——**缺确认 ⇒ 不执行**")
		inv.setErr("usage", "confirm_required", msg)
		fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
		fmt.Fprintf(stderr, "要执行得给：--confirm=%s --yes\n", host)
		fmt.Fprintf(stderr, "先看计划件：%s core restart --dry-run\n", progName)
		fmt.Fprintf(stderr, "error.kind=usage · detail=confirm_required · retryable=false · remedy=fix_usage\n")
		return exitUsage
	}
	if inv.confirm != host {
		msg := fmt.Sprintf("确认值不匹配目标（--confirm 给的是 %q，本机主机名是 %q）⇒ 不执行", inv.confirm, host)
		inv.setErr("usage", "confirm_mismatch", msg)
		fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
		fmt.Fprintf(stderr, "`--confirm` 的值必须与目标**逐字相同**（§4.1 K7）—— 给错值一律拒绝，不许「当没给」\n")
		return exitUsage
	}
	if !inv.yes {
		// 三态：没带干跑旗标而只缺 `--yes` ⇒ **计划件 + rc=2**（fail-closed：从不提问，也不偷偷重启）。
		coreRestartPlan(inv, stdout)
		msg := fmt.Sprintf("`core restart` 是 D3 档 —— **缺 --yes ⇒ 不执行**")
		inv.setErr("usage", "yes_required", msg)
		fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
		fmt.Fprintf(stderr, "确认档齐了才真跑：%s core restart --confirm=%s --yes\n", progName, host)
		fmt.Fprintf(stderr, "error.kind=usage · detail=yes_required · retryable=false · remedy=fix_usage\n")
		return exitUsage
	}

	// ③ 真跑。先判分支：**本版只自动走定义一致那一支**（余下两支归换件入口 —— 不写第二份口径）。
	branch, bwhy := coreRestartBranch()
	if branch != "kickstart-k" {
		msg := fmt.Sprintf("不给结论：重启分支是 %q（%s）—— 本版只自动走 `kickstart-k` 一支；"+
			"定义不一致 / 未装载那两支的**唯一**入口是 `scripts/build/zerg-swap-core.sh`（不许在命令面里另写一遍 bootout/bootstrap 顺序）",
			branch, orX(bwhy, "判据读不到"))
		inv.setErr("blocked", "restart_branch_requires_swap", msg)
		fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
		fmt.Fprintf(stderr, "先看计划件：%s core restart --dry-run（③ 那一格写着将走哪一支）\n", progName)
		return exitBlocked
	}

	oldPid, _ := coreRestartJobPID()
	oldLstart, _ := coreRestartProcLstart(oldPid)
	sha, mtime := coreRestartBinaryIdentity()
	auditPath := coreRestartAuditPath()
	by := strings.TrimSpace(inv.flagVal("--by"))
	if by == "" {
		by = strings.TrimSpace(os.Getenv("ZERG_BY"))
	}
	if by == "" {
		by = "（未声明）"
	}
	base := coreRestartAuditLine{
		By: by, Confirm: inv.confirm, Label: coreRestartLabel, Domain: coreRestartDomain(),
		PidBefore: oldPid, LstartBefore: oldLstart, Branch: branch,
		Ports: strings.Join(coreRestartPorts, ","), BinarySHA256: sha, BinaryMtime: mtime,
		AuditPath: auditPath,
	}

	// ④ 审计**先落盘**（写不进日志就不许执行 · §九 M3 C5 / 设计稿 §七）
	pre := base
	pre.At, pre.Event, pre.RC, pre.Note = time.Now().Format(time.RFC3339), "executed", 0,
		"动作前一行（进程还没动）"
	if err := appendCoreRestartAudit(auditPath, pre); err != nil {
		msg := fmt.Sprintf("审计落不下（%s）：%v ⇒ **未动进程**（不给结论 · 退码 8）", auditPath, err)
		inv.setErr("blocked", "audit_unwritable", msg)
		fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
		fmt.Fprintf(stderr, "口径：无审计的重启 = 不可回读的动作 ⇒ 审计写不进去就绝不执行（设计稿 §七）\n")
		return exitBlocked
	}

	// ⑤ 执行动作（唯一一条）
	fmt.Fprintf(stderr, "%s: 执行 `%s`（pid_before=%s 起时=%s · 审计 %s）\n",
		progName, coreRestartExitLine(branch), orX(oldPid, "未装载"), orX(oldLstart, "（读不到）"), auditPath)
	if err := coreRestartAction(); err != nil {
		line := base
		line.At, line.Event, line.RC, line.Readiness = time.Now().Format(time.RFC3339), "failed", exitFail, "action_failed"
		line.Note = err.Error()
		_ = appendCoreRestartAudit(auditPath, line)
		msg := fmt.Sprintf("动作失败（%s）：%v ⇒ 退码 1", coreRestartExitLine(branch), err)
		inv.setErr("failed", "restart_action_failed", msg)
		fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
		fmt.Fprintf(stderr, "先看计划件与现场：%s core restart --dry-run · 日志 /tmp/zerg-core.log\n", progName)
		return exitFail
	}

	// ⑥ 就绪三子句（必须绑自己的 pid）
	okReady, det, rc := coreRestartWaitReady(oldPid, oldLstart)
	newPid, _ := coreRestartJobPID()
	newLstart, _ := coreRestartProcLstart(newPid)
	line := base
	line.At, line.PidAfter, line.LstartAfter = time.Now().Format(time.RFC3339), newPid, newLstart
	if okReady {
		line.Event, line.Readiness, line.RC = "readiness", "ok", exitOK
	} else if rc == exitBlocked {
		line.Event, line.Readiness, line.RC = "readiness", "unknown", exitBlocked
	} else {
		line.Event, line.Readiness, line.RC = "readiness", "failed", exitFail
	}
	if err := appendCoreRestartAudit(auditPath, line); err != nil {
		fmt.Fprintf(stderr, "%s: ⚠ 就绪那一行审计落不下：%v（判词照实打出来，不改判决）\n", progName, err)
	}
	if !okReady {
		if rc == exitBlocked {
			inv.setErr("blocked", "readiness_unjudgeable", det)
			fmt.Fprintf(stderr, "%s: **就绪判不了**（拿不到就写拿不到，不许假绿）：%s ⇒ 退码 8\n", progName, det)
			fmt.Fprintf(stderr, "%s: 就绪审计已落一行 `readiness=unknown`（%s）\n", progName, auditPath)
			return exitBlocked
		}
		inv.setErr("failed", "readiness_failed", det)
		fmt.Fprintf(stderr, "%s: **起来了但没就绪 ⇒ 退码 1**：%s\n", progName, det)
		fmt.Fprintf(stderr, "%s: 回滚口径：本命令**不回滚件**（件的回滚归 `scripts/build/zerg-swap-core.sh`）；进程面可再跑一次本命令\n", progName)
		fmt.Fprintf(stderr, "%s: 就绪审计已落一行 `readiness=failed`（%s）\n", progName, auditPath)
		return exitFail
	}
	fmt.Fprintf(stderr, "%s: 重启后逐条核 ✓ %s（审计 %s）\n", progName, det, auditPath)
	fmt.Fprintf(stdout, "pid=%s 起时=%s 目标=%s\n", newPid, newLstart, coreRestartLabel)
	return exitOK
}

// orX 空值兜底成给定的那一串（报面用；不改任何判据）。
func orX(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}
