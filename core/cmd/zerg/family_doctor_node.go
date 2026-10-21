// family_doctor_node.go —— `zerg doctor node --machine <机>`：**目标机只读体检**（缺口 `GAP-20260926-194`）。
//
// 病根（账内逐字）：回滚位判定每次仍须手拼 6~8 条 ssh 单命令 ⇒ 现场靠人记、读数靠手抄，
// 漏一条就判错；且「哪一格读不到」与「哪一格本来就没有」在手工面里长一个样。
//
// 本件只做一件事：**一条命令、一次只读探针、一张判定表**。命令面在 `zerg` 名下（✗ 不另搞独立脚本），
// 归在 `doctor` 族（族语义 = 五值判定词闭集 `PASS/FAIL/BLOCKED/REPORT` · 见 `readonly.go:cmdDoctor`）。
//
// ★ 机名是**运行期数据**，不是件内字面量：目标机只从 `--machine <机>` 取（缺省按「本机候选名到名册里对」
// 解析 · 见 `dxxLocalMachine`）。本件 public=yes ⇒ 件内**零机型名 / 零机器名**，机型相关的读法一律
// 从名册现读（`fleet:` 的 `os` / `updates.nodes` 的 `root`·`components`），✗ 不按机型分支写死。
//
// 只读纪律（逐条）：
//
//	· 传输面只有一条：`ssh <名册 ssh 目标> sh -s`（**本机**那一台不绕 ssh，直接 `sh -s`），脚本经 **stdin**
//	  交出（✗ argv 拼串 ⇒ 无引号面）；
//	· 远端命令全是读：`cat` / `git rev-parse` / `git --no-optional-locks status` / `systemctl is-active|cat`
//	  / `ss -ltn`（Linux 面）/ `netstat -an -p tcp` ＋ `lsof -nP -iTCP -sTCP:LISTEN`（darwin 面）
//	  / `pgrep -af` / `ls` / `df -Pk` / `sha256sum` —— ✗ 不写盘、✗ 不改服务、✗ 不提权；
//	  `git status` 带 `--no-optional-locks` + `GIT_OPTIONAL_LOCKS=0`（否则它会**刷新索引** = 写私有仓）；
//	· ✗ 不调主控 API、✗ 不读令牌（本命令与令牌面零接触）。
//
// 判定口径（五值闭集，与 `doctor` 同一条）：
//
//	PASS    —— 读到了、且是健康值；
//	FAIL    —— 读到了、且是坏值（单元不在跑 / 该听的端口没听 / 磁盘 ≥90%）⇒ 退码 1；
//	BLOCKED —— **读不到**（ssh 不可达 / 探针没回话 / 名册缺这台）⇒ 退码 8（「没结论」不许当健康）；
//	REPORT  —— 「未接」：该格在该机上**结构性不存在或未接线**（照实印「未接」，只报告、不进退码）。
//	★ 为什么「未接」不进退码：模型服务不是 systemd 单元、卵面没有独立清单入口 ——
//	  这些格**永远不会**有值；把它们算成 SKIP（⇒ 8）会让本命令恒退 8，等于没有体检面。
//	  结构性缺席 ⇒ 照实说「未接」；「本该有值却读不到」⇒ BLOCKED（这才是 8 该管的）。
//	★ 同一条也管**不进 systemd 的机器**（名册 `os` 不是 linux 那一族）：那几台的守护单元格一律「未接」
//	  （本版不探 launchd 标签），✗ 不把「这台不走 systemd」读成「守护进程坏了」。
//
// 未接（本版照实登记，✗ 不造假数据）：
//
//	· `VERSION` 文件与 `bin/build-info.json` 在目标机现读**不存在 / 空** ⇒ 版本格印「未接」，
//	  只用源码 `git rev-parse` + 件 sha256 作身份（这两样是现读得到的真值）；
//	· 「卵」只有 `~/.zerg/work/<名>` 目录 + bwrap 命令行里的绑定路径两条现读线索（无卵清单件）；
//	· 端口面只判「该听的听没听」，✗ 不做进程归因（`ss -p` 要权限，提权出圈）；探针形态按名册 `os` 族选
//	  （`ss` 是 Linux 专有 —— darwin 面用 `netstat`/`lsof`，见 `dxxPortProbeShell`）。
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	// dxxProbeTimeout —— 一次探针的上限（ssh 连不上时**有界**，✗ 无界挂起）。
	dxxProbeTimeout = 25 * time.Second
	// dxxDiskFailPct —— 磁盘占用判定线（%）：≥ 此值判 FAIL（回滚位要先能落地）。
	dxxDiskFailPct = 90
	// dxxDiskWarnPct —— 磁盘占用提醒线（%）：≥ 此值、< 判红线 ⇒ REPORT（只报告）。
	dxxDiskWarnPct = 80
	// dxxModelPortDefault —— 模型面缺省端口（`fleet.yaml` 的 web 上游口径 127.0.0.1:9400）。
	// ★ 只在**读不到**运行中 llama-server 的 `--port` 时才用它；读到就一律以读数为准。
	dxxModelPortDefault = 9400
)

// dxxAgentUnit —— 目标机的**守护单元名**（`<机名>-agent.service`）。
// 派生规则与 `deploy_launch.go:dplUnitFromHost` / `deploy/服务声明.tsv` 的 `<机名>-agent.service`
// 同一条 ⇒ 机名换一台、单元名跟着换，✗ 件内不落任何具体机型名。
func dxxAgentUnit(machine string) string { return machine + "-agent.service" }

// dxxUnit —— 一次 `UNIT <名> <状态> <单元是否存在>` 读数。
type dxxUnit struct {
	Name   string
	State  string
	Exists bool
}

// dxxDisk —— 一次 `df -Pk` 读数。
type dxxDisk struct {
	Mount   string
	TotalKB int64
	UsedKB  int64
	Pct     int
}

// dxxProbe —— 一次只读探针的全部读数（字段缺省 = 该格没读到）。
type dxxProbe struct {
	Version   string
	BuildInfo string
	Head      string
	Dirty     int
	DirtyRead bool
	AgentdSha string
	Units     []dxxUnit
	Ports     map[int]bool
	Models    []string
	Eggs      []string
	EggRun    []string
	Disks     []dxxDisk
	Complete  bool // 收到 `@@END` ⇒ 探针跑完了（半截读数一律按「读不到」办）
}

// dxxRemoteScript —— 远端探针脚本（经 stdin 交给 `sh -s`；✗ 不拼进 argv）。
// `%s`①= 目标机检出根（已按 POSIX 单引号转义）· `%s`②= 要探的系统单元名（空格分隔 · 由 Go 侧按名册
// `components` 现算 —— 该机结构上没有的组件不去问，免得把「本来就没有」和「坏了」读成一个样）
// · `%s`③= **端口探针的命令形态**（由 `dxxPortProbeShell` 按名册 `os` 族选 —— `ss` 是 Linux 专有，
// macOS 上没有它，写死 `ss` 会让「该听的端口」永远判「未监听」= 假 FAIL）。
// **只读命令清单**见件头。
const dxxRemoteScript = `set +e
R=%s
U="%s"
echo "@@VER"
[ -r "$R/VERSION" ] && echo "VERSION $(cat "$R/VERSION" 2>/dev/null)"
[ -s "$R/bin/build-info.json" ] && echo "BUILDINFO $(tr -d '\n' < "$R/bin/build-info.json" 2>/dev/null)"
echo "HEAD $(git -C "$R" rev-parse --short=12 HEAD 2>/dev/null)"
echo "DIRTY $(GIT_OPTIONAL_LOCKS=0 git -C "$R" --no-optional-locks status --porcelain 2>/dev/null | wc -l | tr -d ' ')"
echo "AGENTD_SHA $(sha256sum "$R/bin/zerg-agentd" 2>/dev/null | cut -c1-12)"
echo "@@UNITS"
for u in $U; do
  st=$(systemctl is-active "$u" 2>/dev/null)
  [ -z "$st" ] && st=unknown
  found=no
  systemctl cat "$u" >/dev/null 2>&1 && found=yes
  echo "UNIT $u $st $found"
done
echo "@@PORTS"
%s
echo "@@MODELS"
pgrep -af llama-server 2>/dev/null | grep -v bwrap | head -5
echo "@@EGGS"
for d in "$HOME/.zerg/work"/*/; do [ -d "$d" ] && basename "$d"; done 2>/dev/null
echo "EGG_RUN $(pgrep -af bwrap 2>/dev/null | grep -o '\.zerg/work/[A-Za-z0-9._-]*' | sed 's#.*/##' | sort -u | tr '\n' ' ')"
echo "@@DISK"
df -Pk / /data 2>/dev/null
echo "@@END"
`

// dxxInlineFields —— 手搓**行内映射**解析（`{ k: v, k: v }`）：★ **成对引号内的分隔符不算分隔**。
// 为什么不用 `yamlInlineField`：它按「第一个 `,` / `}`」截断 ⇒ 名册里 `components: "core,agentd"`
// 会先被切在引号内那个逗号上，得到 `"core` 这种半截值（剥引号又不成立）⇒ 组件一个都不认识、
// 单元格整面塌成 8。本函数按引号状态切分，键值再按**第一个** `:` 分（值里的 `:` 如 URL 不受影响）。
func dxxInlineFields(body string) map[string]string {
	out := map[string]string{}
	s := strings.TrimSpace(body)
	s = strings.TrimSuffix(strings.TrimPrefix(s, "{"), "}")
	items := []string{}
	var cur strings.Builder
	var quote byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
			cur.WriteByte(c)
		case c == '"' || c == '\'':
			quote = c
			cur.WriteByte(c)
		case c == ',':
			items = append(items, cur.String())
			cur.Reset()
		default:
			cur.WriteByte(c)
		}
	}
	items = append(items, cur.String())
	for _, it := range items {
		k, v, ok := strings.Cut(it, ":")
		if !ok {
			continue
		}
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		if len(v) >= 2 {
			if (v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'') {
				v = v[1 : len(v)-1]
			}
		}
		if k != "" {
			out[k] = v
		}
	}
	return out
}

// dxxNodeField —— 目标机的**名册字段**（真源 = `gateway/fleet.yaml` 的 `updates.nodes` 那一行 ·
// 与 `~/.zerg/contexts/<档>.yaml` 的 `nodes:` 逐字同源、由门 `check-nodes-roster` 对拍）。
// 取不到 ⇒ 退 8：✗ 猜一个 `<家目录>/zerg` 或猜「该有哪些组件」去读（猜错就是假读数）。
func dxxNodeField(root, machine, field string, stderr io.Writer) (string, int) {
	b, err := os.ReadFile(filepath.Join(root, agentFleetAddrFileRel))
	if err != nil {
		fmt.Fprintf(stderr, "%s: 名册件读不到（%s：%v）⇒ `%s` 问不出来（退码 8）\n", progName, agentFleetAddrFileRel, err, field)
		return "", exitBlocked
	}
	inUpdates, inNodes := false, false
	for _, line := range strings.Split(string(b), "\n") {
		if line == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			inUpdates = strings.TrimSpace(line) == "updates:"
			inNodes = false
			continue
		}
		if !inUpdates {
			continue
		}
		t := strings.TrimSpace(line)
		if t == "nodes:" {
			inNodes = true
			continue
		}
		if !inNodes || !strings.HasPrefix(t, "-") {
			continue
		}
		body := strings.TrimSpace(strings.TrimPrefix(t, "-"))
		if !strings.HasPrefix(body, "{") {
			continue
		}
		kv := dxxInlineFields(body)
		if kv["name"] != machine {
			continue
		}
		if v := strings.TrimSpace(kv[field]); v != "" {
			return v, exitOK
		}
		fmt.Fprintf(stderr, "%s: `updates.nodes` 里 %q 那条没有 `%s` 字段 ⇒ 问不出来（退码 8）\n", progName, machine, field)
		return "", exitBlocked
	}
	fmt.Fprintf(stderr, "%s: `updates.nodes` 里没有 %q 这一条 ⇒ 问不出来（退码 8 · ✗ 不猜）\n", progName, machine)
	return "", exitBlocked
}

// dxxRosterNames —— 名册 `updates.nodes` 里现有各台的 `name`（现读；读不到 ⇒ why 非空）。
func dxxRosterNames(root string) ([]string, string) {
	b, err := os.ReadFile(filepath.Join(root, agentFleetAddrFileRel))
	if err != nil {
		return nil, fmt.Sprintf("名册件读不到（%s：%v）", agentFleetAddrFileRel, err)
	}
	names := []string{}
	inUpdates, inNodes := false, false
	for _, line := range strings.Split(string(b), "\n") {
		if line == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			inUpdates = strings.TrimSpace(line) == "updates:"
			inNodes = false
			continue
		}
		if !inUpdates {
			continue
		}
		t := strings.TrimSpace(line)
		if t == "nodes:" {
			inNodes = true
			continue
		}
		if !inNodes || !strings.HasPrefix(t, "-") {
			continue
		}
		body := strings.TrimSpace(strings.TrimPrefix(t, "-"))
		if !strings.HasPrefix(body, "{") {
			continue
		}
		if v := strings.TrimSpace(dxxInlineFields(body)["name"]); v != "" {
			names = append(names, v)
		}
	}
	return names, ""
}

// dxxLocalAliases —— 本机的**候选名**（登录名 → 主机名 → 主机名短名；去重保序）。
// 名册里的机器名是**稳定 id**，与 OS 主机名不是一套口径 ⇒ 只能拿候选去名册里**对**，✗ 直接当机名用。
func dxxLocalAliases() []string {
	out := []string{}
	seen := map[string]bool{}
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			return
		}
		seen[s] = true
		out = append(out, s)
	}
	add(os.Getenv("USER"))
	if h, err := os.Hostname(); err == nil {
		h = strings.TrimSpace(h)
		add(h)
		if i := strings.IndexByte(h, '.'); i > 0 {
			add(h[:i])
		}
	}
	return out
}

// dxxLocalMachine —— **缺省机名**：本机候选名先到名册里对（名册是唯一口径），对不上就回落到第一候选
// （由调用方按「名册里没有这台」如实报 —— ✗ 编一个机名去探）。
func dxxLocalMachine(root string) string {
	cands := dxxLocalAliases()
	if len(cands) == 0 {
		return ""
	}
	if names, why := dxxRosterNames(root); why == "" {
		for _, c := range cands {
			for _, n := range names {
				if c == n {
					return n
				}
			}
		}
	}
	return cands[0]
}

// dxxLinuxOS —— 该机的 OS 是否属 **systemd 那一族**（名册 `fleet:` 段的 `os` 字段现读）。
// ✗ 按机型名分支：只认 OS 族，新机器进名册即自动归类。
func dxxLinuxOS(osName string) bool {
	switch strings.ToLower(strings.TrimSpace(osName)) {
	case "linux", "ubuntu", "debian", "centos", "fedora", "rhel", "arch", "rocky", "alma":
		return true
	}
	return false
}

// dxxPortProbeShell —— 端口探针的**命令形态**（按目标机 OS 族选；真源 = 名册 `fleet:` 段的 `os`）。
//
// ★ 病根（2026-10-21 实测）：原先只有 Linux 那一套（`ss -ltnH` → `netstat -ltn` 兜底）⇒ macOS 上
// 没有 `ss`，而 BSD 的 `netstat -ltn` **不按「只听」过滤**（实测吐整张 ESTABLISHED 表，`$4` 里混进
// `1..524288` 一串假端口）⇒ 该听的 `8100` 永远判「未监听」、整行假 FAIL，还捎带一长串假「其余在听」。
//
// darwin 面取两枚 BSD 自有只读工具的**并集**（两枚都不提权）：
//
//	· `netstat -an -p tcp` 的 `LISTEN` 行 —— 见得到**别的用户**的监听口（实测 root 的 sshd:22 就在里面）；
//	· `lsof -nP -iTCP -sTCP:LISTEN` —— 非 root 时看不到 root 的口（实测 22 看不到），但给的是
//	  「真有进程持有」的视图，与 netstat 互补 ⇒ 取并集才不丢格。
//
// 两枚都只吐「本地地址 + 端口」一枚 token（`127.0.0.1:8100` / `*:8100` / `*.22` / `[::1]:8021`），
// 端口由 Go 侧 `dxxPortFromAddrToken` 现解 —— ✗ 不在 shell 里 `sed 's/.*://'` 硬剥：这张表里 IPv4 /
// IPv6 / BSD 点分三种写法混着，剥错一点整格就塌（`*.22` 剥完是 `*`，`::1.8021` 剥完是 `1.8021`）。
func dxxPortProbeShell(osName string) string {
	if dxxLinuxOS(osName) {
		return `(ss -ltnH 2>/dev/null || netstat -ltn 2>/dev/null) | awk 'NF >= 4 {print $4}' | sort -u`
	}
	return `{ netstat -an -p tcp 2>/dev/null | awk '$NF == "LISTEN" {print $4}'; ` +
		`lsof -nP -iTCP -sTCP:LISTEN 2>/dev/null | sed 's/[[:space:]]*(LISTEN)[[:space:]]*$//' | awk 'NF {print $NF}'; } | sort -u`
}

// dxxTransport —— 探针的**传输面**（人面一句话 ＋ 怎么跑）。
type dxxTransport struct {
	Via    string // 人面印的那一句
	Local  bool   // true ⇒ 本机直接 `sh -s`
	Target string // 远端 ⇒ 名册 ssh 目标
}

// dxxProbeTransport —— 本机（回环地址 · 或名册 `ssh` 就是 `-`）⇒ 直接 `sh -s`；
// 其余 ⇒ `ssh <名册 ssh 目标> sh -s`。
// ★ 本机那一支不绕 ssh：名册里本机的 `ssh` 是 `-`，硬走 ssh 会把「本机」读成「不可达」= 假 BLOCKED。
func dxxProbeTransport(machine string, a agentAddr) dxxTransport {
	target := strings.TrimSpace(agentRestartSSHTarget(machine, a.Host))
	if agentAddrIsLocal(a) || target == "" || target == "-" {
		return dxxTransport{Via: "本机（`sh -s` · 不绕 ssh）", Local: true}
	}
	return dxxTransport{Via: "ssh " + target, Target: target}
}

// dxxProbeRun —— 跑一次只读探针（脚本经 stdin 交 `sh -s`），把读数解析成 dxxProbe。
// `why != ""` ⇒ **读不到**（调用方按 BLOCKED 办 · 退 8）。
func dxxProbeRun(machine, root string, a agentAddr, units []string, stderr io.Writer) (*dxxProbe, string) {
	tr := dxxProbeTransport(machine, a)
	var cmd *exec.Cmd
	if tr.Local {
		cmd = armChildGroup(exec.Command("sh", "-s"))
	} else {
		cmd = armChildGroup(exec.Command("ssh", "-o", "BatchMode=yes", "-o", "ConnectTimeout=8", tr.Target, "sh -s"))
	}
	cmd.Stdin = strings.NewReader(fmt.Sprintf(dxxRemoteScript, agentRestartShQuote(root), strings.Join(units, " "), dxxPortProbeShell(a.OS)))
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	runningChild = cmd
	defer func() { runningChild = nil }()
	if err := cmd.Start(); err != nil {
		return nil, fmt.Sprintf("探针起不来：%v", err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(dxxProbeTimeout):
		terminateChildTree(cmd)
		<-done
		return nil, fmt.Sprintf("探针超时（上限 %s）—— 不给结论", dxxProbeTimeout)
	}
	p := dxxParseProbe(out.String())
	if !p.Complete {
		lines := strings.Split(strings.TrimSpace(out.String()), "\n")
		tail := ""
		if n := len(lines); n > 0 {
			if n > 3 {
				n = 3
			}
			tail = strings.Join(lines[len(lines)-n:], " ⏎ ")
		}
		if tail != "" {
			return nil, fmt.Sprintf("探针没跑完（缺 `@@END`）· 远端回话尾部：%s", tail)
		}
		return nil, "探针没跑完（缺 `@@END`）· 远端零回话（ssh 不可达 / 目标机拒连）"
	}
	return p, ""
}

// dxxParseProbe —— 把探针输出解析成读数（按 `@@段名` 分段；未知段忽略，✗ 不猜）。
func dxxParseProbe(raw string) *dxxProbe {
	p := &dxxProbe{Ports: map[int]bool{}}
	sec := ""
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.HasPrefix(line, "@@") {
			sec = strings.TrimPrefix(line, "@@")
			if sec == "END" {
				p.Complete = true
			}
			continue
		}
		if strings.TrimSpace(line) == "" {
			continue
		}
		switch sec {
		case "VER":
			k, v := dxxCutKV(line)
			switch k {
			case "VERSION":
				p.Version = v
			case "BUILDINFO":
				p.BuildInfo = v
			case "HEAD":
				p.Head = v
			case "DIRTY":
				if n, err := strconv.Atoi(v); err == nil {
					p.Dirty, p.DirtyRead = n, true
				}
			case "AGENTD_SHA":
				p.AgentdSha = v
			}
		case "UNITS":
			f := strings.Fields(line)
			if len(f) >= 3 && f[0] == "UNIT" {
				p.Units = append(p.Units, dxxUnit{Name: f[1], State: f[2], Exists: len(f) >= 4 && f[3] == "yes"})
			}
		case "PORTS":
			if n, ok := dxxPortFromListenLine(line); ok {
				p.Ports[n] = true
			}
		case "MODELS":
			p.Models = append(p.Models, strings.TrimSpace(line))
		case "EGGS":
			if strings.HasPrefix(line, "EGG_RUN") {
				_, v := dxxCutKV(line)
				p.EggRun = strings.Fields(v)
				continue
			}
			p.Eggs = append(p.Eggs, strings.TrimSpace(line))
		case "DISK":
			f := strings.Fields(line)
			// POSIX `df -Pk`：Filesystem 1024-blocks Used Available Capacity Mounted-on（≥ 6 栏）
			if len(f) >= 6 && strings.HasSuffix(f[4], "%") {
				total, _ := strconv.ParseInt(f[1], 10, 64)
				used, _ := strconv.ParseInt(f[2], 10, 64)
				pct, err := strconv.Atoi(strings.TrimSuffix(f[4], "%"))
				if err != nil {
					continue
				}
				p.Disks = append(p.Disks, dxxDisk{Mount: f[5], TotalKB: total, UsedKB: used, Pct: pct})
			}
		}
	}
	return p
}

// dxxPortFromListenLine —— 解析 PORTS 段的一行（探针交回来的就是「本地地址 + 端口」一枚 token）。
// ★ 也吃**整行**形态（`ss` / `netstat` / `lsof` 原样一行）：从右往左找第一枚能解出端口的 token ——
// 这样换探针形态（darwin 的 netstat / lsof）不必同步改解析面，改错一端也不会整格塌。
// 读不出的行（表头 / 通配口 / 状态词）⇒ 0,false（✗ 宁缺不猜）。
func dxxPortFromListenLine(line string) (int, bool) {
	if n, ok := dxxPortFromAddrToken(line); ok {
		return n, true
	}
	f := strings.Fields(line)
	nf := len(f)
	for i := nf - 1; i >= 0; i-- {
		if n, ok := dxxPortFromAddrToken(f[i]); ok {
			return n, true
		}
	}
	return 0, false
}

// dxxPortFromAddrToken —— 从一枚地址 token 里解出端口。认这几种写法（实测同一张表里混着）：
//
//	`127.0.0.1:8100`（ss / lsof）· `*:8100`（lsof 通配）· `[::1]:8100`（IPv6 带方括号）·
//	`*.22` / `127.0.0.1.8100` / `::1.8021` / `fe80::343f:ecff:.58783`（BSD `netstat` 用点分）·
//	裸数字 `8100`（旧探针那一形态，向后兼容）。
//
// 非端口 token（`0.0.0.0:*` / `NAME` / `LISTEN` / 越界数）⇒ 0,false。
func dxxPortFromAddrToken(tok string) (int, bool) {
	t := strings.TrimSpace(tok)
	for _, suf := range []string{"(LISTEN)", "LISTEN"} {
		if strings.HasSuffix(t, suf) {
			t = strings.TrimSpace(strings.TrimSuffix(t, suf))
		}
	}
	if t == "" {
		return 0, false
	}
	cands := []string{t}
	if i := strings.LastIndexByte(t, ':'); i >= 0 {
		cands = append(cands, t[i+1:])
	}
	if i := strings.LastIndexByte(t, '.'); i >= 0 {
		cands = append(cands, t[i+1:])
	}
	for _, c := range cands {
		if n, err := strconv.Atoi(strings.TrimSpace(c)); err == nil && n > 0 && n <= 65535 {
			return n, true
		}
	}
	return 0, false
}

// dxxCutKV —— 拆 `KEY value`（值可为空；只按第一个空白切）。
func dxxCutKV(line string) (string, string) {
	k, v, _ := strings.Cut(strings.TrimSpace(line), " ")
	return k, strings.TrimSpace(v)
}

// dxxGrow —— 判定词聚合（同 `doctor`：FAIL(1) > BLOCKED(8) > 其余）。
func dxxGrow(worst, v string) string {
	rank := func(s string) int {
		switch s {
		case "FAIL":
			return 3
		case "BLOCKED":
			return 2
		case "REPORT":
			return 1
		}
		return 0
	}
	if rank(v) > rank(worst) {
		return v
	}
	return worst
}

// dxxPortName —— 端口在**该机**上的身份（只在**已知**端口上给名字；✗ 不猜）。
func dxxPortName(port, agentdPort, modelPort int) string {
	switch port {
	case 22:
		return "sshd"
	case agentdPort:
		return "zerg-agentd"
	case modelPort:
		return "llama-server"
	}
	return ""
}

// dxxModelNames —— 从 llama-server 命令行里取 `-m` 那一枚的基名（读不到 ⇒ 空串）。
func dxxModelNames(lines []string) []string {
	out := []string{}
	for _, ln := range lines {
		f := strings.Fields(ln)
		name := ""
		for i, tok := range f {
			if tok == "-m" && i+1 < len(f) {
				name = filepath.Base(f[i+1])
			}
		}
		if name != "" {
			out = append(out, name)
		}
	}
	return out
}

// dxxModelPorts —— 从 llama-server 命令行里取 `--port`（读不到 ⇒ 空表）。
func dxxModelPorts(lines []string) []int {
	out := []int{}
	for _, ln := range lines {
		f := strings.Fields(ln)
		for i, tok := range f {
			if tok == "--port" && i+1 < len(f) {
				if n, err := strconv.Atoi(f[i+1]); err == nil && n > 0 {
					out = append(out, n)
				}
			}
		}
	}
	return out
}

// cmdDoctorNode —— `zerg doctor node [--machine <机>]`：目标机只读体检（**缺省 = 本机**）。
// 机名只从 `--machine` 取；本版不带位置参数、✗ 不在这里偷开多机面（多机名册走 `--json` 之前必须是另一笔）。
func cmdDoctorNode(inv *invocation, stdout, stderr io.Writer) int {
	// ── 用法面先判（零副作用 · stdout 0 字节）──
	if len(inv.args) > 0 {
		inv.setErr("usage", "node_takes_no_positional", "本命令不吃位置参数（机名走 --machine）")
		fmt.Fprintf(stderr, "%s: `doctor node` 不吃位置参数（多给的是 %q）—— 机名走 `--machine <机>`\n", progName, inv.args[0])
		return exitUsage
	}
	root := repoRoot()
	if root == "" {
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 读不到名册（%s）⇒ 不给结论（退码 8）\n", progName, agentFleetAddrFileRel)
		return exitBlocked
	}
	machine := strings.TrimSpace(inv.flagVal("--machine"))
	if machine == "" {
		machine = dxxLocalMachine(root)
		if machine == "" {
			fmt.Fprintf(stderr, "%s: 解析不出本机名（`USER` 与主机名都取不到）⇒ 给 `--machine <机>` 再来（退码 8· ✗ 不猜）\n", progName)
			return exitBlocked
		}
	}
	// 机名先过名册（`fleet:` 段）：不认识的机名是**用法错**（2 · 附最像的合法输入），不是「读不到」（8）。
	a, arc := agentChildAddr(machine, stderr)
	if arc != exitOK {
		return arc
	}
	nodeRoot, rc := dxxNodeField(root, machine, "root", stderr)
	if rc != exitOK {
		return rc
	}
	components, crc := dxxNodeField(root, machine, "components", stderr)
	if crc != exitOK {
		return crc
	}
	// 单元探针面 = 名册 `components` 现算（核心：`core` ⇒ zerg-core.service；子端：`agentd` ⇒ `<机名>-agent.service`）。
	// ✗ 不问该机结构上没有的单元（那会把「本来就没有」读成「坏了」）。
	unitName := dxxAgentUnit(machine)
	units := []string{}
	for _, c := range strings.Split(components, ",") {
		switch strings.TrimSpace(c) {
		case "core":
			units = append(units, "zerg-core.service")
		case "agentd":
			units = append(units, unitName)
		}
	}
	if len(units) == 0 {
		// 名册 components 一个都不认识 ⇒ 不猜单元面（fail-closed），但**出声**：静默 8 等于没给结论。
		fmt.Fprintf(stderr, "%s: 名册 `updates.nodes` 里 %q 的 `components` 现读 %q —— 一个组件名都不认识 ⇒ 不猜单元格（退码 8）\n",
			progName, machine, components)
		return exitBlocked
	}
	via := dxxProbeTransport(machine, a).Via
	p, why := dxxProbeRun(machine, nodeRoot, a, units, stderr)
	if why != "" {
		// ★ 读不到 ⇒ BLOCKED（8）：✗ 不把「没结论」印成「健康」、✗ 不填假数据。
		fmt.Fprintf(stdout, "%s 只读体检（%s · 检出根 %s）\n", machine, via, nodeRoot)
		fmt.Fprintf(stdout, "  判定     : BLOCKED（%s）\n", why)
		fmt.Fprintf(stdout, "  口径     : 只读探针（一次 ssh / 本机 sh · 只读命令）· 读不到 ⇒ 退码 8（✗ 不当健康、✗ 不填读数）\n")
		fmt.Fprintf(stderr, "%s: %s 体检读不到：%s ⇒ 不给结论（退码 8）\n", progName, machine, why)
		return exitBlocked
	}

	linux := dxxLinuxOS(a.OS)
	agentdPort := a.Port
	modelPorts := dxxModelPorts(p.Models)
	modelPort := dxxModelPortDefault
	if len(modelPorts) > 0 {
		modelPort = modelPorts[0]
	}

	worst := "PASS"
	line := func(name, reading, verdict string) {
		fmt.Fprintf(stdout, "  %s : %s  [%s]\n", dxxPadName(name), reading, verdict)
		worst = dxxGrow(worst, verdict)
	}
	head12 := p.Head
	if head12 == "" {
		head12 = "（读不到）"
	}
	fmt.Fprintf(stdout, "%s 只读体检（%s · 检出根 %s）\n", machine, via, nodeRoot)

	// ── ① 版本 / sha ─────────────────────────────────────────────────────────
	// 读序：`VERSION` 文件 → `bin/build-info.json`（带 version/commit/build_time 三键）→ 两样都没有 ⇒ 未接。
	// ★ 只用**读得到**的身份印版本；读不到就印「未接」，✗ 不用本机版本号冒充远端（那是造数据）。
	verParts := []string{}
	bi := dxxParseBuildInfo(p.BuildInfo)
	switch {
	case p.Version != "":
		verParts = append(verParts, "VERSION "+p.Version)
	case bi.Version != "":
		one := "构建件 " + bi.Version
		if bi.Commit != "" {
			one += " · commit " + bi.Commit
		}
		if bi.BuildTime != "" {
			one += " · 构建于 " + bi.BuildTime
		}
		verParts = append(verParts, one+"（`bin/build-info.json`）")
	default:
		verParts = append(verParts, fmt.Sprintf("版本 未接（%s 上 `VERSION` 与 `bin/build-info.json` 现读不存在/读不出 —— ✗ 不编版本号）", machine))
	}
	verParts = append(verParts, "源码 HEAD "+head12)
	if p.DirtyRead {
		verParts = append(verParts, fmt.Sprintf("工作树脏 %d 件", p.Dirty))
	} else {
		verParts = append(verParts, "工作树脏 未接（git status 读不到）")
	}
	if p.AgentdSha != "" {
		verParts = append(verParts, "zerg-agentd sha256 "+p.AgentdSha)
	} else {
		verParts = append(verParts, "zerg-agentd sha256 未接（件不在/读不到）")
	}
	verVerdict := "PASS"
	if p.Head == "" {
		verVerdict = "REPORT"
	}
	line("版本/sha", strings.Join(verParts, " · "), verVerdict)

	// ── ② 单元状态 ───────────────────────────────────────────────────────────
	unitParts := []string{}
	unitVerdict := "PASS"
	for _, u := range p.Units {
		switch {
		case u.State == "active":
			unitParts = append(unitParts, u.Name+" active")
		case !u.Exists && !linux:
			// 该机不走 systemd ⇒ 这一格**结构性缺席**（本版不探 launchd）⇒ 未接，✗ 判红。
			unitParts = append(unitParts, u.Name+" 未接（该机不走 systemd · 本版不探 launchd）")
			unitVerdict = dxxGrow(unitVerdict, "REPORT")
		case !u.Exists && u.Name == unitName:
			// 该机的守护单元不存在 ⇒ 真坏（这台机靠它接单）。
			unitParts = append(unitParts, u.Name+" 未接（该机无此 systemd 单元）")
			unitVerdict = dxxGrow(unitVerdict, "FAIL")
		case !u.Exists:
			unitParts = append(unitParts, u.Name+" 未接（该机无此单元）")
			unitVerdict = dxxGrow(unitVerdict, "REPORT")
		default:
			unitParts = append(unitParts, fmt.Sprintf("%s %s", u.Name, u.State))
			unitVerdict = dxxGrow(unitVerdict, "FAIL")
		}
	}
	if len(p.Units) == 0 {
		unitParts = append(unitParts, "未接（探针没给单元读数）")
		unitVerdict = dxxGrow(unitVerdict, "REPORT")
	}
	// 模型服务不是 systemd 单元（现读：llama-server 由 bwrap 起、无 `.service`）⇒ 从单元格里点名。
	unitParts = append(unitParts, "llama-server 未接（不是 systemd 单元 · 见「模型/卵」格）")
	line("单元状态", strings.Join(unitParts, " · "), unitVerdict)
	// ── ③ 端口 ───────────────────────────────────────────────────────────────
	portParts := []string{}
	portVerdict := "PASS"
	addPort := func(port int, must bool, label string) {
		if p.Ports[port] {
			portParts = append(portParts, fmt.Sprintf("%d %s 监听", port, label))
			return
		}
		portParts = append(portParts, fmt.Sprintf("%d %s 未监听", port, label))
		if must {
			portVerdict = dxxGrow(portVerdict, "FAIL")
		} else {
			portVerdict = dxxGrow(portVerdict, "REPORT")
		}
	}
	addPort(22, true, "sshd")
	addPort(agentdPort, true, "zerg-agentd·名册端口")
	if len(p.Models) > 0 {
		addPort(modelPort, true, "llama-server·读数端口")
	} else {
		portParts = append(portParts, fmt.Sprintf("%d llama-server 未接（该机当前没在跑模型进程）", modelPort))
		portVerdict = dxxGrow(portVerdict, "REPORT")
	}
	// 其余在听的口只看一眼（✗ 不当判据 —— 谁开的不知道就不判）。
	extra := []string{}
	for port := range p.Ports {
		if port == 22 || port == agentdPort || (len(p.Models) > 0 && port == modelPort) {
			continue
		}
		extra = append(extra, strconv.Itoa(port))
	}
	extra = dxxSortIntsByString(extra)
	if len(extra) > 0 {
		portParts = append(portParts, "其余在听 "+strings.Join(extra, ","))
	}
	line("端口", strings.Join(portParts, " · "), portVerdict)

	// ── ④ 模型 / 卵 ──────────────────────────────────────────────────────────
	modParts := []string{}
	modVerdict := "PASS"
	if names := dxxModelNames(p.Models); len(names) > 0 {
		modParts = append(modParts, fmt.Sprintf("模型在跑 %d 个（%s）· 端口 %d", len(names), strings.Join(names, " / "), modelPort))
	} else {
		modParts = append(modParts, "模型在跑 0 个（未接：无 llama-server 进程 —— 该机此刻没挂模型）")
		modVerdict = dxxGrow(modVerdict, "REPORT")
	}
	if len(p.Eggs) == 0 && len(p.EggRun) == 0 {
		modParts = append(modParts, fmt.Sprintf("卵 未接（`~/.zerg/work` 空或读不到 ⇒ %s 上无卵在跑）", machine))
		modVerdict = dxxGrow(modVerdict, "REPORT")
	} else {
		running := map[string]bool{}
		for _, e := range p.EggRun {
			running[e] = true
		}
		parts := []string{}
		for _, e := range p.Eggs {
			if running[e] {
				parts = append(parts, e+"（在跑）")
				delete(running, e)
			} else {
				parts = append(parts, e+"（目录·未跑）")
			}
		}
		for e := range running {
			parts = append(parts, e+"（在跑·无目录）")
		}
		modParts = append(modParts, fmt.Sprintf("卵 %d 只（%s）", len(parts), strings.Join(parts, " · ")))
	}
	line("模型/卵", strings.Join(modParts, " · "), modVerdict)

	// ── ⑤ 磁盘 ───────────────────────────────────────────────────────────────
	diskParts := []string{}
	diskVerdict := "PASS"
	for _, d := range p.Disks {
		diskParts = append(diskParts, fmt.Sprintf("%s %d%%（%s/%s）", d.Mount, d.Pct, dxxGB(d.UsedKB), dxxGB(d.TotalKB)))
		switch {
		case d.Pct >= dxxDiskFailPct:
			diskVerdict = dxxGrow(diskVerdict, "FAIL")
		case d.Pct >= dxxDiskWarnPct:
			diskVerdict = dxxGrow(diskVerdict, "REPORT")
		}
	}
	if len(p.Disks) == 0 {
		diskParts = append(diskParts, "未接（df 读不到）")
		diskVerdict = dxxGrow(diskVerdict, "REPORT")
	}
	line("磁盘", strings.Join(diskParts, " · "), diskVerdict)

	fmt.Fprintf(stdout, "  判定     : %s\n", worst)
	fmt.Fprintf(stdout, "  口径     : 一条 ssh + 只读命令（`cat`/`git rev-parse`/`--no-optional-locks status`/`systemctl is-active|cat`/`ss -ltn`（Linux 面）/`netstat`＋`lsof`（macOS 面）/`pgrep -af`/`ls`/`df -Pk`/`sha256sum`）—— ✗ 不写盘、✗ 不改服务、✗ 不提权\n")
	fmt.Fprintf(stdout, "  判定词   : PASS 读到且健康 · FAIL 读到且坏（退 1）· BLOCKED 读不到（退 8）· REPORT 未接（只报告）\n")
	switch worst {
	case "FAIL":
		return exitFail
	case "BLOCKED":
		return exitBlocked
	}
	return exitOK
}

// dxxBuildInfo —— 目标机 `bin/build-info.json` 的三键（读不到 ⇒ 全空）。
type dxxBuildInfo struct {
	Version   string
	Commit    string
	BuildTime string
}

// dxxParseBuildInfo —— 解析 `bin/build-info.json`（现读实测四键：version/commit/build_time/builder_host）。
// 解析不了 ⇒ 空结构（调用方按「未接」办 · ✗ 拿半截串冒充版本号）。
func dxxParseBuildInfo(raw string) dxxBuildInfo {
	if strings.TrimSpace(raw) == "" {
		return dxxBuildInfo{}
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return dxxBuildInfo{}
	}
	get := func(k string) string {
		if s, ok := m[k].(string); ok {
			return strings.TrimSpace(s)
		}
		return ""
	}
	return dxxBuildInfo{Version: get("version"), Commit: get("commit"), BuildTime: get("build_time")}
}

// dxxW —— 终端显示宽（CJK/全角按 2 列算；只为对齐，✗ 不进任何判定）。
func dxxW(s string) int {
	w := 0
	for _, r := range s {
		if r >= 0x2E80 {
			w += 2
		} else {
			w++
		}
	}
	return w
}

// dxxPadName —— 格名补到同一显示宽（读数行左对齐）。
func dxxPadName(s string) string {
	if pad := 8 - dxxW(s); pad > 0 {
		return s + strings.Repeat(" ", pad)
	}
	return s
}

// dxxGB —— KB ⇒ 人类可读（GiB 一位小数；不足 1G 用 MiB）。
func dxxGB(kb int64) string {
	if kb >= 1024*1024 {
		return fmt.Sprintf("%.1fG", float64(kb)/(1024*1024))
	}
	return fmt.Sprintf("%dM", kb/1024)
}

// dxxSortIntsByString —— 端口串按**数值**升序（字符串排序会把 100 排到 99 前面）。
func dxxSortIntsByString(in []string) []string {
	out := append([]string(nil), in...)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0; j-- {
			a, _ := strconv.Atoi(out[j-1])
			b, _ := strconv.Atoi(out[j])
			if a <= b {
				break
			}
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}
