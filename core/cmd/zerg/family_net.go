// family_net.go —— 环境面**只读**一格（组4 §二.4 `W-41` · `研-归:152` §五 栗①「建议形态」· 任务单序123）。
//
// 为什么它排这一波（源件原话，不重述）：本稿每一处取外网正文都得手写
// `-x http://127.0.0.1:7895`（直连 `huggingface.co` 当场 `SSL_ERROR_SYSCALL`）；而命令面现读
// `grep -ci proxy` ⇒ **0 命中** ⇒ **环境事实（代理 / 网口 / 凭据落点）今天没有命令面投影**，
// 每个会话各自手搓一次。
//
// 口径（照源件「建议形态」逐字，不自造）：
//
//	形态 `zerg net probe`（**只读** · 不写盘 · 不改状态 · 不新开端点 —— 只读本机既有的两处真源）
//	三格 `proxy / reachable / mode` = 本机代理在哪 / 走不走得通 / 直连还是代理
//	退码 `0` 三格都读到 · `8` **不许当绿** 的两档 · `2` 用法错（照既有 `K2` 一套，本命令不另立）
//
// ★ 退码 8 的**两档分得很清**（这是本命令最要紧的一条边界，源件那句「取不到 ⇒ rc=8」的落点）：
//
//	① **源读不到**（两处真源都读不到 ⇒ 讲不出「代理在哪」）⇒ rc=8 且**三格一格都不出**
//	   （stdout 0 字节；给了 `--json <字段>` 时按 §九 M7 出**错误包封**，那是错误面、不是结果面）
//	② **代理配着而探不到**（`reachable=no`）⇒ rc=8 **且三格照出** —— 投影照给，rc 才是判据。
//	   为什么不合档：①是「讲不出答案」，②是「答案就是不通」；把两者并成同一个形状
//	   （都 0 字节）会让消费侧丢掉 ② 的那个答案。
//
// 两处真源（都不新开一条路）：
//
//	① 环境变量（`https_proxy` → `HTTPS_PROXY` → `http_proxy` → … —— **认读序写死**，见 envProxyVars）
//	② 本机系统代理（`scutil --proxy` · 只读；`scutil` 不在 PATH ⇒ 这一处**读不到**，
//	   不许降级成「没配」—— 「读不到」与「直连」是两件事）
//
// ★ 一处照实记的偏离：源件写的是「代理 / 网口 / 凭据落点」，本枚只落**代理那一格**
// （三格逐字与判据栏同：代理在哪 / 走不走得通 / 直连还是代理）—— **网口**与**凭据落点**
// 本枚**未落** ✗（照实登记在回执 §七，不半落、不编）。
package main

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"
)

// netProbeFields —— `--json` 可取的三格（**顺序即判据栏那一句的顺序**：代理在哪 / 走不走得通 /
// 直连还是代理）。字段名一个都不多给：多给一格就与「出三格」这条判据对不上了。
//
// ★ 与 `main.go` 里那条登记**同一个值**（登记那一处必须写成 `[]string{…}` 字面量 ——
// 原因写在 `main.go` 该条的注释里：契约脚本的 `FIELDS_RE` 只认字面量）。两处同值由
// `cli_net_probe_test.go` 的 `TestNetProbe_UsageFace` 用现跑对拍钉住（裸给 `--json` 时
// stderr 印的那行「可选字段」就是登记那一份）。
var netProbeFields = []string{"proxy", "reachable", "mode"}

// envProxyVars —— 代理环境变量的**认读序**（写死一处，命令面与自检都读它）。
//
// 为什么 https 在 http 前面：本仓取外网正文几乎全走 TLS（`raw.githubusercontent.com` /
// `huggingface.co` / 上游 README），而 `curl` 对 https 目标只认 `https_proxy` ⇒
// 先认它才不会把「https 走代理、http 直连」两件事并成一个答案。
var envProxyVars = []string{
	"https_proxy", "HTTPS_PROXY", "http_proxy", "HTTP_PROXY", "all_proxy", "ALL_PROXY",
}

// netProbeTimeout —— 探针的墙钟上界。
//
// ★ 为什么这道上界只能写在 Go 里：本机（macOS）**没有 `timeout` 命令**（源件逐字记着这条
// 环境硬事实）⇒ 外挂一条 `timeout 2 curl …` 在本机**根本跑不起来**；`net.DialTimeout` 是
// 唯一现成的、不依赖外部件的上界。
const netProbeTimeout = 2 * time.Second

// proxyFromEnv 按 envProxyVars 的序取第一个**非空**的代理写法 ⇒ (地址, 变量名, 取到没)。
func proxyFromEnv() (addr, varName string, ok bool) {
	for _, name := range envProxyVars {
		raw := strings.TrimSpace(os.Getenv(name))
		if raw == "" {
			continue
		}
		return normalizeProxyAddr(raw), name, true
	}
	return "", "", false
}

// normalizeProxyAddr 把常见写法收敛成 `host:port`：去 scheme（`http://` / `socks5://` …）、
// 去 userinfo（`user:pass@` —— 凭据**不许带进人面**）、去尾斜杠。
//
// 边界照实：收敛不出可用形状（例如 `http://` 后面什么都没有）⇒ **原样返回**，不替人猜一个端口。
// 让 `reachable` 那一格去判它通不通；在这里静默编一个值 = 把「给错了」读成「给对了」。
func normalizeProxyAddr(raw string) string {
	s := strings.TrimSpace(raw)
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	if i := strings.LastIndex(s, "@"); i >= 0 {
		s = s[i+1:]
	}
	return strings.TrimSuffix(s, "/")
}

// systemProxyRaw 读本机系统代理那一处真源（**只读**：`scutil --proxy` 只打印配置，不改盘）。
//
// 三条「读不到」的形态**分开报**（都归 rc=8 那一档 ①，但不许合成一句模糊话）：
// `scutil` 不在 PATH / 跑不起来 / 输出读不回来。
func systemProxyRaw() (string, error) {
	p, err := exec.LookPath("scutil")
	if err != nil {
		return "", fmt.Errorf("`scutil` 不在 PATH ⇒ 系统代理那一处真源读不到")
	}
	out, err := exec.Command(p, "--proxy").Output()
	if err != nil {
		return "", fmt.Errorf("`scutil --proxy` 跑不起来：%v", err)
	}
	return string(out), nil
}

// systemProxyOf 从 `scutil --proxy` 的现读输出里取代理落点 ⇒ (地址, 取到没)。
//
// 解析纪律（与本波序124「抓取/解析要有解析器或显式分界符」同族）：**显式分隔符** `" : "` +
// 逐行键值，**不用正则兜** —— 键（`HTTPEnable` / `HTTPSProxy` …）里不会有空格，而散文里
// 「代理」两字随处可见（同一个词在别的族里也出现过 ⇒ 拿词去搜必然假命中）。
//
// 认读序与 envProxyVars **同向**：HTTPS 先于 HTTP 先于 SOCKS。
// 「取到没」= 有没有一个 Enable 为 1 且 host/port 都非空的档 —— **`ok=false` 是一条答案**
// （本机系统面说「直连」），**不是**「读不到」（那一档在 systemProxyRaw 就返回了）。
func systemProxyOf(raw string) (string, bool) {
	kv := map[string]string{}
	for _, ln := range strings.Split(raw, "\n") {
		k, v, found := strings.Cut(ln, " : ")
		if !found {
			continue // 表头 / `<dictionary> {` / `}` —— 一律不当键值
		}
		kv[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	for _, p := range [][3]string{
		{"HTTPSEnable", "HTTPSProxy", "HTTPSPort"},
		{"HTTPEnable", "HTTPProxy", "HTTPPort"},
		{"SOCKSEnable", "SOCKSProxy", "SOCKSPort"},
	} {
		if kv[p[0]] != "1" {
			continue
		}
		host, port := kv[p[1]], kv[p[2]]
		if host == "" || port == "" {
			continue
		}
		return host + ":" + port, true
	}
	return "", false
}

// dialProbe 探一次 TCP（**只连不发**：连上就关 ⇒ 零写、零副作用、不取正文）。
//
// 探测对象 = **代理自己**，不是外网：本命令要答的是「本机那条出网路通不通」，
// 而那条路的入口就是代理 —— 打外网会把这个判据变成「外网现在活不活」，换一个问法。
func dialProbe(addr string) (bool, string) {
	conn, err := net.DialTimeout("tcp", addr, netProbeTimeout)
	if err != nil {
		return false, err.Error()
	}
	_ = conn.Close()
	return true, ""
}

// cmdNetProbe —— `zerg net probe` 的实现面（只读；三格见文件头）。
func cmdNetProbe(inv *invocation, stdout, stderr io.Writer) int {
	addr, src, fromEnv := proxyFromEnv()
	if !fromEnv {
		raw, err := systemProxyRaw()
		if err != nil {
			// 档 ①：两处真源都读不到 ⇒ **三格一格都不出**（退码 8 · 不许当绿）。
			inv.setErr("blocked", "net_source_unreadable", "环境面两处真源都读不到")
			fmt.Fprintf(stderr, "%s: 环境面**取不到** —— %v；且 %d 个代理环境变量（%s）一个都没设\n",
				progName, err, len(envProxyVars), strings.Join(envProxyVars, " "))
			fmt.Fprintf(stderr, "%s: 照 `G-10` 那条口径：取不到**不许当绿**（退码 8）—— 三格一格都不出（不许 rc=0 + 空格）\n", progName)
			return exitBlocked
		}
		src = "scutil --proxy"
		if a, ok := systemProxyOf(raw); ok {
			addr = a
		}
	}

	mode, reach := "direct", "n/a"
	switch {
	case addr != "":
		mode = "proxy"
		if ok, detail := dialProbe(addr); ok {
			reach = "yes"
		} else {
			reach = "no"
			fmt.Fprintf(stderr, "%s: 代理 %s 配着、**探不到**（%s）\n", progName, addr, detail)
		}
	default:
		fmt.Fprintf(stderr, "%s: 本机没配代理（源：%s）⇒ 直连档：那一格没有代理可探（`n/a` 是一条答案，不是取不到）\n",
			progName, src)
	}

	fmt.Fprintf(stderr, "%s: 源 = %s · 代理 = %s · 走不走得通 = %s · 档 = %s\n",
		progName, src, dashIfBlank(addr), reach, mode)
	rc := listCmd(inv, stdout, stderr, netProbeFields, []map[string]string{{
		"proxy":     dashIfBlank(addr),
		"reachable": reach,
		"mode":      mode,
	}})
	if rc != exitOK {
		return rc
	}
	if reach == "no" {
		// 档 ②：答案就是「不通」⇒ 三格照出（上面已出），但**不许当绿**。
		inv.setErr("blocked", "proxy_unreachable", "代理配着但探不到")
		fmt.Fprintf(stderr, "%s: 代理面**不通** ⇒ 不许当绿（退码 8）—— 三格照出（见上 / 见 `--json`）\n", progName)
		return exitBlocked
	}
	return exitOK
}

// dashIfBlank —— 空格子**不许出现**（判据逐字「不许 rc=0 + 空格」）⇒ 空值一律写成 `-`。
// 为什么不是「不写这一格」：三格是**定长投影**，缺一格会让消费侧的两条命令的输出对不齐。
func dashIfBlank(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}
