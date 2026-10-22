// update_check.go —— `zerg update --check`：**只读**的「有没有新版本」检查（缺口 `Q-129` / `8741788000`）。
//
// 为什么有这件：
//
//	`zerg update` 在本版是**危险 D3 · 未开放**的壳（`cmdGuarded`：确认档给齐了也只回「本版未开放」），
//	而「只看看有没有新版本」这件事是**只读**的 —— 不下载、不构建、不换件 ⇒ 不该被 D3 的确认档拦住，
//	也不该算进危险档。此前这个动作**只挂在 `zerg-core update --check`**（真身在
//	`core/internal/selfupdate`），CLI 侧没有出口。
//
// 复用点（✗ 不重写检查逻辑、✗ 不搬空 zerg-core）：
//
//	`selfupdate.Check(g, remote, ref, stateDir, version, useCache)` —— 与 `zerg-core update --check`
//	**同一个函数**：同一份 6 小时结论缓存、同一条 `GitOrigin-RevId` 镜像映射、同一条
//	「inconclusive ⇒ 不写缓存」。本件只做三件事：① 把 CLI 的旗标面接上（`--check` / `--json`）
//	② 定**三态退码** ③ 出人读 / 机器两面。`zerg-core update` 的完整面（`--to` / `--force` /
//	`--role` / `--components` / `--no-cache` / `--no-ui`）**一个都不动**，usage 里逐字指路过去。
//
// 三态退码（只用 selfupdate 契约里**已有**的数字，不新造码 —— 真源
// `core/internal/contract/selfupdate.json`，本包常量是它的投影）：
//
//	0 = **有新版**（`behind` 落后 / `diverged` 分叉 —— 即 `CheckResult.Updatable()`）
//	2 = **无新版**（`up-to-date` 已是最新 / `local-ahead` 本地领先开发态）—— 与 `no_update` 同义
//	1 = **判不了**（fetch 失败 / 取不到代码身份 / 检查被配置关掉）—— ★ **绝不复用 2**：
//	    把「判不出」说成「无需更新」就是假绿，调用方会据此认为本机是最新的。
package main

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/Mr2109/zerg-swarm/core/internal/selfupdate"
	"github.com/Mr2109/zerg-swarm/core/internal/version"
)

// updateCheckFields —— `zerg update --check --json <字段>` 的字段表（与 `selfupdate.CheckResult` 同形，
// 另加 `version` 与 `updatable`）。★ 声明面与产出面**同名同数**：门58 现跑 `--json <坏名>` 取真身清单、
// 与命令树 `fields:` 逐名对拍 —— 多一个少一个都红，故两边都由本变量供。
var updateCheckFields = []string{
	"status", "version", "local_sha", "remote_sha",
	"behind", "ahead", "dirty", "source", "updatable", "message",
}

// 三态退码（照 selfupdate 契约的投影常量取名，✗ 不另写 0/1/2 字面量）。
const (
	updateRCNew     = selfupdate.ExitOK       // 0 —— 有新版可装
	updateRCNone    = selfupdate.ExitNoUpdate // 2 —— 无新版（`no_update` 同一码，不同义于「判不了」）
	updateRCUnknown = selfupdate.ExitFail     // 1 —— 判不了（★ 绝不复用 2）
)

// cmdUpdateOrGuarded —— `update` 的入口分岔：`--check` ⇒ 只读检查；其余 ⇒ 原危险档守卫（一字不动）。
//
// 为什么不把 `--check` 塞进 `cmdGuarded`：那条路的形态是「先 `--dry-run` 出计划件、再确认档、
// 最后未开放退 2」—— 只读检查既没有计划件也没有确认档，硬塞进去会让「只读」这件事继承
// D3 的 fail-closed 形状（`zerg update --check` 缺 `--yes` ⇒ 2），等于新旗标不可用。
func cmdUpdateOrGuarded(inv *invocation, stdout, stderr io.Writer) int {
	if inv.hasFlag("--check") {
		return cmdUpdateCheck(inv, stdout, stderr)
	}
	return cmdGuarded(inv, stdout, stderr)
}

// cmdUpdateCheck —— 只读检查。**零构建、零换件、零写仓**；唯一落盘是 6 小时结论缓存
// （`selfupdate.CachePath` · 与 `zerg-core update --check` 共用同一份 ⇒ 两条命令不会各说一个结论）。
func cmdUpdateCheck(inv *invocation, stdout, stderr io.Writer) int {
	// ① 只读档与「动」的那条路的旗标**互斥**：同时给 ⇒ 用法错 2（不静默取其一 —— §4.1 K14）。
	if inv.dryRun {
		usageMsg := "`update --check` 与 `--dry-run` 是两件事：前者只查版本（不碰任何件），后者是换件计划件（危险 D3 路径）—— 只给一个"
		fmt.Fprintf(stderr, "%s: %s\n", progName, usageMsg)
		inv.setErr("usage", "check_dryrun_conflict", usageMsg)
		return exitUsage
	}
	if inv.confirmGiven || inv.yes {
		usageMsg := "`update --check` 是只读检查 —— 不收 `--confirm` / `--yes`（那是换件那条路的确认档）"
		fmt.Fprintf(stderr, "%s: %s\n", progName, usageMsg)
		fmt.Fprintf(stderr, "要真换件：`%s update --confirm=<主机名> --yes`（本版未开放）；要完整检查面：`zerg-core update --check`\n", progName)
		inv.setErr("usage", "check_confirm_conflict", usageMsg)
		return exitUsage
	}
	if len(inv.nodes) > 0 || len(inv.args) > 0 {
		usageMsg := "`update --check` 查的是**本机**（本机检出 ⟷ 远端 ref）—— 不点名机器（不收 `--node` / 位置参数）"
		fmt.Fprintf(stderr, "%s: %s\n", progName, usageMsg)
		fmt.Fprintf(stderr, "要查别的机器：到那台机上跑同一条命令（或 `zerg-core update --check`）\n")
		inv.setErr("usage", "check_local_only", usageMsg)
		return exitUsage
	}

	// ★ `meta.changed` 不许说「改了」：只读档在包封里必须如实报 false（`emitEnvelopeWith` 的缺省
	//   口径是「危险命令 ⇒ changed:true」，那是**换件那条路**的语义；直接沿用会让只读读数自相矛盾）。
	inv.changed = boolPtr(false)

	// ② 配置闸：`updates.check: false` ⇒ **判不了**（不是「无新版」）。判据与 `zerg-core update --check`
	//    同一处（`selfupdate.CheckEnabled` ⇒ 内部 `updatesCheckEnabled` 一份实现，✗ 不在这里再读一遍 YAML）。
	opts := selfupdate.DefaultOptions()
	if !selfupdate.CheckEnabled(opts.FleetYAML) {
		msg := fmt.Sprintf("更新检查已被配置关闭（updates.check = false · %s）", opts.FleetYAML)
		fmt.Fprintf(stderr, "%s: %s ⇒ **判不了**（不是「无需更新」）· 退码 %d\n", progName, msg, updateRCUnknown)
		// ★ kind 必须与退码**同源**（`E1`：退码是 kind 的单值投影）。`blocked` 虽然语义贴切，
		//   它的投影是 **8** —— 与本档 1 打架就是「同一次调用两个码面」。`failed` 投影 = 1，
		//   且 1 也在 selfupdate 契约里（`zerg-core update --check` 判不了时同样退 1）⇒ 取它。
		inv.setErr("failed", "update_check_disabled", msg)
		return updateRCUnknown
	}

	// ③ 检查本体 —— 复用 selfupdate 那头的那一个函数。
	g := selfupdate.Git{Dir: opts.RepoDir}
	res := selfupdate.Check(g, opts.Remote, opts.Ref, opts.StateDir, version.Version, opts.UseCache)

	// ★ 「判不出」不许读成「零差异」：`CheckResult` 的约定是 **-1 = 判不出**，而 `Check` 在
	//   `inconclusive` 的两条早退里留下的是**零值 0** ⇒ 直接印出来会变成「落后 0 笔 · 领先 0 笔」
	//   （一个干净的假读数）。在出口这一层归一（✗ 不改共享实现的行为 —— 那会影响 zerg-core 侧读数）。
	if res.Status == selfupdate.StatusInconclusive {
		if res.Behind == 0 {
			res.Behind = -1
		}
		if res.Ahead == 0 {
			res.Ahead = -1
		}
	}

	rc := updateRCFor(res)

	// ④ 机器面 / 人面（两面对同一份读数，✗ 不各算一遍）。
	if inv.jsonGiven {
		if rc := requireFields(inv, stderr); rc != exitOK {
			return rc
		}
		row := map[string]string{
			"status":     res.Status,
			"version":    version.Version,
			"local_sha":  res.LocalSHA,
			"remote_sha": res.RemoteSHA,
			"behind":     strconv.Itoa(res.Behind),
			"ahead":      strconv.Itoa(res.Ahead),
			"dirty":      strconv.FormatBool(res.Dirty),
			"source":     res.Source,
			"updatable":  strconv.FormatBool(res.Updatable()),
			"message":    res.Message,
		}
		// ★ 投影用**用户点名的字段序**（`inv.fields`），✗ 别把整张登记表递进去：递整表 ⇒ `--json 打错名`
		//   会被静默忽略并**照样出全部字段**（§4.1 K1 的「点名字段」契约破），且门58 的「真身侧」是靠
		//   `--json <坏名>` 的 stderr「合法字段:」表取数的 —— 不拒就取不到那行。
		if rc2 := selectJSON(stdout, stderr, inv, inv.path, inv.fields, row); rc2 != exitOK {
			return rc2
		}
		return rc
	}

	fmt.Fprintf(stdout, "当前版: zerg %s @ %s%s\n", version.Version, ucShortSHA(res.LocalSHA), dirtyNote(res.Dirty))
	fmt.Fprintf(stdout, "最新版: %s @ %s（来源 %s）\n", opts.Ref, ucShortSHA(res.RemoteSHA), sourceNote(res.Source))
	fmt.Fprintf(stdout, "差  异: %s\n", deltaNote(res))
	switch rc {
	case updateRCNew:
		fmt.Fprintf(stdout, "结  论: **有新版**可装 —— 本次只读（未下载、未构建、未换件）；真换件走 `%s update --confirm=<主机名> --yes`（D3 · 本版未开放）或 `zerg-core update`\n", progName)
	case updateRCNone:
		fmt.Fprintf(stdout, "结  论: 无新版（%s）\n", res.Message)
	default:
		fmt.Fprintf(stdout, "结  论: **判不了** —— %s\n", res.Message)
		fmt.Fprintf(stderr, "%s: 判不了 ⇒ 不给结论（退码 %d · **不是**「无需更新」）；完整检查面：`zerg-core update --check`\n", progName, updateRCUnknown)
	}
	return rc
}

// updateRCFor —— 三态退码的**唯一**落点（纯函数：给 `CheckResult` ⇒ 退码；不碰网络、不写盘）。
//
// 为什么要抽出来：三态是本缺口的核心判据（0/2/1），散在命令体里就只能靠跑真仓库来验；
// 抽成纯函数后 `update_check_test.go` 能把五档状态**逐档**钉住（含 `diverged` 也归「有新版」）。
func updateRCFor(res selfupdate.CheckResult) int {
	switch {
	case res.Updatable(): // behind / diverged —— 远端确实有本地没有的东西
		return updateRCNew
	case res.Status == selfupdate.StatusUpToDate || res.Status == selfupdate.StatusLocalAhead:
		return updateRCNone
	}
	// inconclusive（fetch 失败 / 取不到代码身份）—— ★ 绝不复用 2
	return updateRCUnknown
}

// ucShortSHA —— 8 位短 sha（空 ⇒ 逐字「未知」，✗ 不落空白 —— 「读不到」不是「没有」）。
func ucShortSHA(s string) string {
	if strings.TrimSpace(s) == "" {
		return "未知"
	}
	if len(s) > 8 {
		return s[:8]
	}
	return s
}

func dirtyNote(dirty bool) string {
	if dirty {
		return "（本机检出 · 工作树有未提交改动）"
	}
	return "（本机检出）"
}

func sourceNote(src string) string {
	if src == "cache" {
		return "6 小时缓存 · `zerg-core update --check --no-cache` 可强制实时"
	}
	return "live"
}

func deltaNote(res selfupdate.CheckResult) string {
	if res.Status == selfupdate.StatusInconclusive {
		return "判不出（取不到代码身份 —— 本行不是「零差异」）"
	}
	parts := []string{res.Status}
	if res.Behind >= 0 {
		parts = append(parts, fmt.Sprintf("落后 %d 笔", res.Behind))
	}
	if res.Ahead >= 0 {
		parts = append(parts, fmt.Sprintf("领先 %d 笔", res.Ahead))
	}
	if res.Behind < 0 && res.Ahead < 0 {
		parts = append(parts, "笔数判不出（浅检出/历史不足）")
	}
	return strings.Join(parts, " · ")
}

// usageDeclaresExactFlag —— **精确**版 `usageDeclaresFlag`（后者是子串判）。
//
// 为什么需要它：`--check` 是 `--check-excludes-parity` 的**前缀** ⇒ 子串判会让
// `zerg publish run --check` 被「证明」已登记，随后落进实现里被**静默吞**（硬约束⑤：既有命令的
// 退码与输出一律不许变）。本函数把 usage 串里的旗标 token 逐个抠出来**逐字**比。
func usageDeclaresExactFlag(cmd *command, flag string) bool {
	if cmd == nil {
		return false
	}
	for _, tok := range strings.FieldsFunc(cmd.usage, func(r rune) bool {
		return !(r == '-' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9'))
	}) {
		if tok == flag {
			return true
		}
	}
	return false
}
