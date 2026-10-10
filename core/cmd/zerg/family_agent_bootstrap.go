// family_agent_bootstrap.go —— `zerg agent bootstrap <机>`（缺口 `GAP-20260925-22` ·「agent bootstrap-user-domain」）。
//
// 账（逐字）：子端挂**系统域** LaunchDaemon 时连不上主控（`no route to host`）；同机同用户由 ssh 手动运行、
// 或挂**用户域** LaunchAgent 时心跳正常。⇒ **跨机换件没有「域选择」的作业定义与判据**，换件时只能手搓
// ssh + 请人敲提权命令。父代理现核（2026-09-27）把本笔收窄为：**没有一条能验证「系统域能不能连主控」的
// 实测路径**，并把文档四坑（`gateway/protocol.md:194-197` §12）搬进判据。
//
// ★ 硬线（本件遵守）：系统用户 / 权限 / launchd 面上**只做只读判定 + 可打印计划** ——
// ✗ 不真动本机用户、组、权限、launchd；✗ 不真下发；✗ 不真起引擎。真装**本版未开放**
// （`cmdAgentBootstrap` 的真跑分支原样转 `cmdGuarded`）⇒ 真跑行为与改前逐字不变。
//
// 只读源：本仓 `deploy/` 与 `gateway/deploy/` 下的**部署夹具 plist**（✗ 不读本机 `/Library/LaunchDaemons`
// 或 `~/Library/LaunchAgents`；✗ 不碰任何系统用户面）。取不到 ⇒ **非 0 不给结论**（退码 8 · ✗ 不许静默当已就绪）。
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// 域（§12 关键部署坑 · gateway/protocol.md:194-197）。
const (
	// agentBootDomainUser —— 用户域 LaunchAgent（`~/Library/LaunchAgents`）—— 本账实测可连主控。
	agentBootDomainUser = "user"
	// agentBootDomainSystem —— 系统域 LaunchDaemon（`/Library/LaunchDaemons`）—— 本账实测 no route to host（除非签名齐）。
	agentBootDomainSystem = "system"
)

// 逐格判词（三态：ok / warn / red）。
const (
	agentBootOK   = "ok"
	agentBootWarn = "warn"
	agentBootRed  = "red"
)

// agentBootFixtureDirs —— 本仓可读的部署夹具目录（只读扫描源 · 仓根相对 · 保序）。
var agentBootFixtureDirs = []string{"deploy", "gateway/deploy"}

// agentBootPlistFact —— 从一件 plist 夹具里读出的**域选择事实**（只读投影 · 不做判定）。
type agentBootPlistFact struct {
	Path       string // 夹具件（仓根相对）
	Machine    string // `--machine` 的值
	Program    string // 可执行件（`ProgramArguments` 数组里的**首个** `<string>`）
	Controller string // `--controller` 的值（拨号地址）
	ListenHost string // `--host` 的值（监听地址）
	Port       string // `--port` 的值
}

// agentBootCell —— 只读判定的一格（键 / 值 / 判词 / 提示）。
type agentBootCell struct {
	Key   string
	Value string
	Flag  string
	Note  string
}

// agentBootPlistStrings —— 抽出一件 plist 正文里**全部** `<string>…</string>` 的值（只读 · 保序）。
func agentBootPlistStrings(body string) []string {
	var out []string
	rest := body
	for {
		i := strings.Index(rest, "<string>")
		if i < 0 {
			break
		}
		rest = rest[i+len("<string>"):]
		j := strings.Index(rest, "</string>")
		if j < 0 {
			break
		}
		out = append(out, rest[:j])
		rest = rest[j+len("</string>"):]
	}
	return out
}

// agentBootProgramFrom —— 取 `ProgramArguments` 数组里的**首个** `<string>`（= 可执行件）。
//
// ★ ✗ 不用「正文首个 `<string>`」当可执行件：两件夹具都**先**给 `<key>Label</key><string>…</string>`
// ⇒ 那样取到的是 **Label**（`com.x3gw.agent` / `com.zerg.agent-Mr2109`），不是程序 —— 实测踩过。
// 取不到（没有 `ProgramArguments` / 其后没有 `<string>`）⇒ 空串（= 可执行件取不到 · 由判据如实标 warn）。
func agentBootProgramFrom(body string) string {
	i := strings.Index(body, "<key>ProgramArguments</key>")
	if i < 0 {
		return ""
	}
	rest := body[i:]
	j := strings.Index(rest, "<string>")
	if j < 0 {
		return ""
	}
	rest = rest[j+len("<string>"):]
	k := strings.Index(rest, "</string>")
	if k < 0 {
		return ""
	}
	return rest[:k]
}

// agentBootUnquote —— 剥掉值两侧成对的一层引号（`"…"` / `'…'`）；不成对 / 无引号 ⇒ 原样。
func agentBootUnquote(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// agentBootPlistFactFrom —— 从一件 plist 正文里认出一台机的域选择事实。
//
// 认法（只读 · 不猜）：把所有 `<string>` 值按空白切成 token，逐枚找 `--machine/--host/--port/--controller`
// 的**值**；可执行件取 `ProgramArguments` 数组首个 `<string>`（`agentBootProgramFrom`）。兼容两种形态：
// ① 逐枚 `<string>` 一旗标一值（`deploy/com.zerg.agent-Mr2109.plist`）；② `/bin/sh -c "…"` 单串内嵌
// （`gateway/deploy/com.x3gw.agent.mini.plist`）。认不出（无任何 `<string>` / 无 `--machine`）⇒ ok=false
// （**取不到** · 由调用方判非 0）。
func agentBootPlistFactFrom(path, body string) (agentBootPlistFact, bool) {
	strs := agentBootPlistStrings(body)
	if len(strs) == 0 {
		return agentBootPlistFact{}, false
	}
	f := agentBootPlistFact{Path: path, Program: agentBootProgramFrom(body)}
	var toks []string
	for _, s := range strs {
		toks = append(toks, strings.Fields(s)...)
	}
	for i := 0; i < len(toks); i++ {
		val := func() string {
			if i+1 < len(toks) {
				return agentBootUnquote(toks[i+1])
			}
			return ""
		}
		switch toks[i] {
		case "--machine":
			f.Machine = val()
		case "--host":
			f.ListenHost = val()
		case "--port":
			f.Port = val()
		case "--controller":
			f.Controller = val()
		}
	}
	if f.Machine == "" {
		return agentBootPlistFact{}, false
	}
	return f, true
}

// agentBootFindFixture —— 在仓内部署夹具里找 `--machine=<machine>` 的那一件（只读 · 保序 · 首命中即返）。
// 找不到 / 目录读不到 / 认不出 ⇒ ok=false（**取不到** · ✗ 不静默当已就绪）。
func agentBootFindFixture(root, machine string) (agentBootPlistFact, bool) {
	machine = strings.TrimSpace(machine)
	for _, rel := range agentBootFixtureDirs {
		abs := filepath.Join(root, filepath.FromSlash(rel))
		ents, err := os.ReadDir(abs)
		if err != nil {
			continue
		}
		names := make([]string, 0, len(ents))
		for _, e := range ents {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".plist") {
				continue
			}
			names = append(names, e.Name())
		}
		sort.Strings(names)
		for _, n := range names {
			raw, err := os.ReadFile(filepath.Join(abs, n))
			if err != nil {
				continue
			}
			if fact, ok := agentBootPlistFactFrom(rel+"/"+n, string(raw)); ok && fact.Machine == machine {
				return fact, true
			}
		}
	}
	return agentBootPlistFact{}, false
}

// agentBootJudge —— 只读判据（纯函数 · 不碰系统面）：域选择 + 文档四坑 ⇒（逐格 · 整单判词）。
//
// 四格（照父代理现核 2026-09-27 修法 ⑶「拨号地址 + 是否 `0.0.0.0` + 签名」）：
//
//	domain       域选择：**用户域**（LaunchAgent · 实测可连主控）；系统域仅当「二进制在 /usr/local/bin + 已重新签名」才可用。
//	controller   拨号地址（`--controller`）：非空（空 ⇒ red —— 子端认不出主控）。
//	listen_host  监听地址（`--host`）：`0.0.0.0` ⇒ ok；空（裸 `:port`）⇒ **red**（Go 默认 IPv6-only ⇒ 子端 IPv4 连不上 · §12 第4坑）；回环 ⇒ warn。
//	signed       可执行件路径（签名假）：`/tmp/**` ⇒ **red**（ad-hoc 签名 launchd 不信任 · §12 第3坑）；`/usr/local/bin/**` ⇒ ok；其余 ⇒ warn。
//
// 整单判词：任一格 red ⇒ red；否则 ok（warn 不翻红）。
func agentBootJudge(f agentBootPlistFact) ([]agentBootCell, string) {
	cells := []agentBootCell{
		{Key: "domain", Value: agentBootDomainUser, Flag: agentBootOK,
			Note: "域选择：用户域 LaunchAgent（本账实测可连主控）；系统域 LaunchDaemon 仅在签名齐时可用"},
	}
	// controller（拨号地址）
	ctrl := agentBootCell{Key: "controller", Value: f.Controller, Flag: agentBootOK}
	if strings.TrimSpace(f.Controller) == "" {
		ctrl.Flag = agentBootRed
		ctrl.Note = "拨号地址取不到 ⇒ 子端认不出主控"
	}
	cells = append(cells, ctrl)
	// listen_host（监听地址）
	h := agentBootCell{Key: "listen_host", Value: f.ListenHost, Flag: agentBootOK}
	switch strings.TrimSpace(f.ListenHost) {
	case "0.0.0.0":
		h.Flag = agentBootOK
	case "":
		h.Flag = agentBootRed
		h.Note = "裸 `:port` ⇒ Go 默认 IPv6-only ⇒ 子端 IPv4 连不上（§12 第4坑）"
	case "127.0.0.1", "localhost", "::1":
		h.Flag = agentBootWarn
		h.Note = "回环 ⇒ **远端**子端连不上（本机自用可）"
	default:
		h.Flag = agentBootOK
	}
	cells = append(cells, h)
	// signed（可执行件路径）
	s := agentBootCell{Key: "signed", Value: f.Program, Flag: agentBootOK}
	switch {
	case strings.HasPrefix(f.Program, "/tmp/"):
		s.Flag = agentBootRed
		s.Note = "/tmp 下 ad-hoc 签名 launchd 不信任（OS_REASON_CODESIGNING · §12 第3坑）"
	case strings.HasPrefix(f.Program, "/usr/local/bin/"):
		s.Flag = agentBootOK
	default:
		s.Flag = agentBootWarn
		s.Note = "可执行件不在 /usr/local/bin ⇒ 建议落 /usr/local/bin 并重新签名"
	}
	cells = append(cells, s)
	verdict := agentBootOK
	for _, x := range cells {
		if x.Flag == agentBootRed {
			verdict = agentBootRed
		}
	}
	return cells, verdict
}

// agentBootPrintPlan —— 打印**可打印计划**（只读判定 + 域选择 + 步骤）到 w（✗ 不执行任何一步）。
func agentBootPrintPlan(machine string, f agentBootPlistFact, w io.Writer) {
	cells, verdict := agentBootJudge(f)
	fmt.Fprintf(w, "计划件（--dry-run · 只读判定 + 可打印计划 · ✗ 不真动本机用户/组/权限/launchd）\n")
	fmt.Fprintf(w, "  动作     : %s agent bootstrap %s\n", progName, machine)
	fmt.Fprintf(w, "  域选择   : %s（LaunchAgent · ~/Library/LaunchAgents —— 本账实测可连主控）\n", agentBootDomainUser)
	fmt.Fprintf(w, "             系统域（%s · LaunchDaemon · /Library/LaunchDaemons）仅当「二进制在 /usr/local/bin + 已重新签名」才可用，否则 no route to host（§12 · gateway/protocol.md:194-197）\n", agentBootDomainSystem)
	fmt.Fprintf(w, "  夹具     : %s（--machine=%s · host=%s · port=%s）\n", f.Path, f.Machine, f.ListenHost, f.Port)
	fmt.Fprintf(w, "  只读判定 : 整单 = %s\n", verdict)
	for _, c := range cells {
		note := ""
		if c.Note != "" {
			note = " —— " + c.Note
		}
		fmt.Fprintf(w, "    %-12s : %-30s [%s]%s\n", c.Key, c.Value, c.Flag, note)
	}
	fmt.Fprintf(w, "  步骤     :\n")
	fmt.Fprintf(w, "    1. 二进制落 /usr/local/bin/ 并 codesign 重新签名（✗ /tmp 下 ad-hoc 签名 launchd 不信任）\n")
	fmt.Fprintf(w, "    2. plist 落**用户域** ~/Library/LaunchAgents/com.zerg.agent-%s.plist（✗ 系统域 /Library/LaunchDaemons）\n", machine)
	fmt.Fprintf(w, "    3. launchctl bootout gui/<uid>/<label> ; launchctl bootstrap gui/<uid> <plist>\n")
	fmt.Fprintf(w, "    4. 回读：`%s agent ls` 里 %s 的 healthy=true\n", progName, machine)
	fmt.Fprintf(w, "  本版状态 : **真装未开放** —— 本面只做只读判定 + 可打印计划（✗ 不真下发 / ✗ 不真起引擎）\n")
	fmt.Fprintf(w, "  来源     : 缺口 GAP-20260925-22 · §12 gateway/protocol.md:194-197\n")
}

// cmdAgentBootstrap —— `zerg agent bootstrap <机> [--confirm=<机> --yes | --dry-run]`。
//
// ★ 真跑分支**原样转 `cmdGuarded`**（缺确认 / 确认值不匹配 / 本版未开放三档逐字不变 · 与改前逐字同）。
// `--dry-run` 分支 = 本缺口补的那一段：**只读判定 + 可打印计划**（域选择 + 四坑），零副作用。
func cmdAgentBootstrap(inv *invocation, stdout, stderr io.Writer) int {
	if !inv.dryRun {
		return cmdGuarded(inv, stdout, stderr)
	}
	if rc := dryRunYesConflict(inv, stderr); rc != exitOK {
		return rc
	}
	if len(inv.args) == 0 {
		inv.setErr("usage", "missing_target", "缺机器名")
		fmt.Fprintf(stderr, "%s: `agent bootstrap` 要一台机器名（`%s agent ls` 看名册）\n", progName, progName)
		return exitUsage
	}
	if len(inv.args) > 1 {
		inv.setErr("usage", "too_many_targets", "多余位置参数（本档只吃一台机）")
		fmt.Fprintf(stderr, "%s: `agent bootstrap` 只吃**一台**机（多给的是 %q）\n", progName, inv.args[1])
		return exitUsage
	}
	machine := strings.TrimSpace(inv.args[0])
	root := repoRoot()
	if root == "" {
		inv.setErr("blocked", "repo_root_absent", "解析不到仓根")
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 读不了部署夹具（不给结论 · 退码 8）\n", progName)
		fmt.Fprintf(stderr, "在仓内跑，或设 ZERG_REPO=<仓根>\n")
		return exitBlocked
	}
	fact, ok := agentBootFindFixture(root, machine)
	if !ok {
		inv.setErr("blocked", "no_deploy_fixture", fmt.Sprintf("仓内部署夹具里没有 %q 这一台", machine))
		fmt.Fprintf(stderr, "%s: 仓内部署夹具（deploy/ · gateway/deploy/*.plist）里没有一台 --machine=%s\n", progName, machine)
		fmt.Fprintf(stderr, "⇒ **取不到 ⇒ 不给结论**（退码 8）—— ✗ 不把「没有夹具」当「已就绪」\n")
		return exitBlocked
	}
	agentBootPrintPlan(machine, fact, stdout)
	fmt.Fprintln(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未执行、未改任何状态、✗ 未动本机用户/组/权限/launchd）")
	return exitOK
}
