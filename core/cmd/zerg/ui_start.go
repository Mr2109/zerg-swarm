// ui_start.go —— UI「脱离会话」启动口的 **CLI 正门**（`zerg ui start`）· 壳转账薄壳（只转发、不翻译）。
//
// 依据：`~/.zerg/state/A档18件-收CLI设计稿-20261007.md` 批 3 · A6
//
//	（`scripts/build/start-zerg-ui.sh` · 81 行 · 现属面＝构建面/UI 启动 · 归族 = 现役 `ui` 族扩 action）。
//
// 落法 **(b) 壳转账**（设计稿逐字）：底层唯一实现 = `scripts/build/start-zerg-ui.sh`（真跑
// `os.setsid()` 脱离会话 ＋ 逐 PID 点名选进程 ＋ 日志/stdin 重定向 · OS 原生面 ⇒ 无法干净真 Go 化）；
// 命令面**只做入口 ＋ 门禁 ＋ 转发**，✗ 不重写脚本里的任何一段（否则就长出第二个真源 ——
// 与脚本里「逐 PID 点名、禁子串/通配」同一条纪律）。
//
// ★ M2 纪律：本件**不接线**（✗ 不在 `main.go` 命令表登记 `ui start`、✗ 不改任何 `family_*.go`、
//
//	✗ 不改任何 `add_step`）。登记那一条（`path: []string{"ui","start"}; run: cmdUIStart`）要改
//	`main.go` ＝ **另一路独占**（派单 ④）⇒ 本件先落实现、接线延后；在此之前 `cmdUIStart`
//	无调用点（与同批 M2 落件同态）。
//
// 三条口径（照仓里既有的透传族写 —— `build_wall.go` / `family_isolate_ro.go`，不自己创式样）：
//
//	① **只转发、不翻译**：`zerg ui start` 之后的**一切**原样交给脚本，连退码也不改写
//	   （0 / 1 逐档原样转出；脚本 `set -euo pipefail`，未成即 1）。底层逻辑只有一处 ⇒ 本件不许重写 ✗。
//	② **命令面自己只在「连脚本都递不到」那一档退码**：解析不到仓根 / 脚本不在 ⇒ `8`（不给结论 ·
//	   与 `ui` 族同档）；**不拿脚本的 1 冒充**（两件事不许并成一个形状）。
//	③ **不新增机器面**：底层脚本没有 `--json` ⇒ 本件也不自己拼一份包封（契约在命令面只有一处）。
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// uiStartScriptRel —— UI 脱离会话启动的**唯一实现**（真源）。本件一个字都不重写它：
// 脱离会话（`os.setsid`）、逐 PID 点名（`zerg_ui_pids`）、令牌注入（`~/.zerg/token`）、
// 日志/stdin 重定向、退码（0 已成 / 1 未成）全在那边。
const uiStartScriptRel = "scripts/build/start-zerg-ui.sh"

// cmdUIStart —— `zerg ui start`（A6 的 Go 正门 · 只转发不翻译）。
//
// 退码口径（**逐档照底层**）：脚本的 `0` / `1` 原样转出 ——
//
//	0 成功（UI 已起，或已在运行 ⇒ 幂等不重起）· 1 失败（UI 二进制不可执行 / 起来又退了）。
//
// 命令面自己只在「递不到脚本」那一档退码：解析不到仓根 / 脚本不在 ⇒ `8`（不给结论 · 与 `ui` 族同档）。
func cmdUIStart(inv *invocation, stdout, stderr io.Writer) int {
	root := repoRoot()
	if root == "" {
		inv.setErr("blocked", "repo_root_absent", "解析不到仓根")
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 递不到 UI 启动脚本（不给结论 · 退码 8）\n", progName)
		return exitBlocked
	}
	script := filepath.Join(root, uiStartScriptRel)
	if _, err := os.Stat(script); err != nil {
		inv.setErr("blocked", "ui_start_script_absent", "UI 启动脚本不在")
		fmt.Fprintf(stderr, "%s: UI 启动脚本不在（%s）⇒ 不给结论（退码 8）\n", progName, uiStartScriptRel)
		return exitBlocked
	}
	// 透传面：`zerg ui start` 之后的**一切**原样交给脚本。
	// `ui` 族占两个路径段（族 `ui` ＋ 动作 `start`）⇒ 取 `inv.orig[len(inv.path):]`，
	// 与 `build_wall.go`（`build` 族亦两段）同一取法。
	tail := []string{}
	if len(inv.orig) > len(inv.path) {
		tail = inv.orig[len(inv.path):]
	}
	fmt.Fprintf(stderr, "%s: 底层 = %s（**唯一真源** · 命令面只转发、不翻译）\n", progName, uiStartScriptRel)
	cmd := armChildGroup(exec.Command("bash", append([]string{script}, tail...)...))
	runningChild = cmd
	defer func() { runningChild = nil }()
	cmd.Dir = root
	cmd.Stdin = os.Stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			// ★ 只转发、不翻译：脚本的码原样返回（与 `build`/`ui` 族同一条口径 · §4.3 U1）。
			fmt.Fprintf(stderr, "%s: UI 启动脚本退 %d（原样转发，不翻译）\n", progName, ee.ExitCode())
			return ee.ExitCode()
		}
		inv.setErr("failed", "ui_start_script_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 起不动 UI 启动脚本：%v\n", progName, err)
		return exitFail
	}
	return exitOK
}

// uiStartGateMain —— `cmdUIStart` 的 **inproc 同签名包装入口**（与 `gateCheckGoEntry` 表值
// 同签名：`func(stdout, stderr io.Writer, argv []string) int`）。
//
// ★ **仅转发**：把 argv 原样当作 `zerg ui start` 之后的实参，转交 `cmdUIStart`
// （唯一实现仍是 `cmdUIStart` ＋ 底层脚本 `scripts/build/start-zerg-ui.sh` —— 本件 ✗ 不重写
// 任何一段、✗ 不改退码口径、✗ 不新增机器面；两态三面与 `cmdUIStart` 逐字同）。
// ★ 单一真源：底层脚本恒为 A6 面 `scripts/build/start-zerg-ui.sh`（顶层 `start-zerg-ui.sh`＝A17
// 是**另一件**、待退 ⇒ ✗ 不在此双登）。
// ★ M2 纪律：本入口**暂不接线**（✗ 未在 `core/cmd/zerg/gate_inproc_table.go` 登记键 —— 表键另路一并接）。
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
)

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

// cmdSandboxVerify —— `zerg sandbox verify [--wall <茧壁二进制> | --self-test]`（判据7 两态门禁 · 批1）。
// 旗标原样透传给 `verify-two-states.py`（`--wall` / `--self-test` 等由底层脚本认）。
func cmdSandboxVerify(inv *invocation, stdout, stderr io.Writer) int {
	tail := []string{}
	if len(inv.orig) > len(inv.path) {
		tail = inv.orig[len(inv.path):]
	}
	return sandboxRunScript(inv, stdout, stderr, "python3", sandboxVerifyRel, tail)
}

// cmdSandboxNegctl —— `zerg sandbox negctl`（两态门禁的负例活体控制 · 批1 · bash）。
func cmdSandboxNegctl(inv *invocation, stdout, stderr io.Writer) int {
	tail := []string{}
	if len(inv.orig) > len(inv.path) {
		tail = inv.orig[len(inv.path):]
	}
	return sandboxRunScript(inv, stdout, stderr, "bash", sandboxNegctlRel, tail)
}

// cmdSandboxTest —— `zerg sandbox test <套件>`：沙箱夹具（fleet/relay/setup/update）归**批3**
// （依赖被测件正门 + 夹具环境）⇒ 本版**未接**，fail-closed 退 `2` ＋ 逐字报「未接」（✗ 绝不静默返回 0）。
func cmdSandboxTest(inv *invocation, stdout, stderr io.Writer) int {
	inv.setErr("usage", "sandbox_test_not_wired", "沙箱夹具动作未接（批3）")
	fmt.Fprintf(stderr, "%s: `sandbox test` **未接** —— 沙箱夹具（fleet/relay/setup/update）归批3（依赖被测件正门 + 夹具环境）⇒ 不给结论（退码 2）\n", progName)
	fmt.Fprintf(stderr, "error.kind=usage · detail=sandbox_test_not_wired · retryable=false\n")
	return exitUsage
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
	extra := []string{}
	if len(inv.args) > 1 {
		extra = inv.args[1:]
	}
	return wallRunScript(inv, stdout, stderr, "python3", rel, extra)
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
