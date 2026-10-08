// family_agent_workers.go —— `agent workers set` / `agent scheduler set` 两条 D2 写面
// （P0 #232 第四/第五面 · 2026-10-08；第三面 `itask interval set` 已落 · DEV-1934）。
//
// 病（现读勘定 · 另一路已核）：内核两包 `core/internal/agent` 的调度旋钮（并发上限 / 轮询间隔 /
// 死信阈值 / 失败冷却）此前**全是编译期常量**（`scheduler.go:24-30`），仓库里**没有任何运行期落点文件**；
// 命令树里能直接设的 = 0 条 ⇒ 出事只能改源码重编。
//
// 本票只开这两条命令面写口（✗ 不碰主控、✗ 不碰远端）：
//
//	· `agent workers set <机> --max N` —— **只写本地** `<仓根>/config/workers.yaml` 的 devices 段
//	  （把那台机的 `max_workers` 设成 N）。★ `<机> ≠ 本机` ⇒ **明确拒绝 · fail-closed**：
//	  设远端机的并发要子端控制端点，而**本版未开** ⇒ 拒执退 2，✗ 禁假写 / ✗ 禁假装成功。
//	· `agent scheduler set [--poll-interval <N>] [--max-retries <N>] [--cooldown <N>]` ——
//	  写 `<仓根>/config/workers.yaml` **顶加** `scheduler:` 段；`core/internal/agent/scheduler.go`
//	  的 `Start()` / `handleFailure()` **读侧接线**（编译期常量留作**缺省值** · ✗ 不许删常量）。
//
// 口径逐字照同族写面先例（`family_itask.go:215 cmdItaskIntervalSetGuarded` ＋ `:268 cmdItaskIntervalSetWrite`
// 的「落地值回读」；`family_sched.go` 的 `schedConfirmGate` · §九 M3 C1/C2/C4）：
//
//	· D2 档（有影响但可逆：再设回旧值即回原态）；
//	· `--dry-run` —— 只出**计划件**、零副作用、退 0（档序：干跑先于确认档）；
//	· `--confirm=<本机名>` + `--yes` —— fail-closed（缺一即退 2，从不提问）；
//	· `--dry-run` 与 `--yes` 同给 ⇒ `dryRunYesConflict` 判用法错 2；
//	· 真写后**落地值回读**（重读同一件，报出刚写的值）。
//
// 落点纪律：本件**只**写 `config/workers.yaml`（仓内件），✗ 不写别处、✗ 不 exec、✗ 不碰 git。
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// agentWorkersMaxCeiling —— `--max` 上界（与 `core/internal/agent/workers_config.go` 的
// `ValidateWorkersConfig`「should not exceed 16」同一档；0 = 显式暂停派活）。
const agentWorkersMaxCeiling = 16

// agentWorkersConfigPath —— 两条命令共用的落点（仓根下的 `config/workers.yaml`）；
// 取不到仓根 ⇒ 空串（调用方 fail-closed，✗ 不猜一个）。
func agentWorkersConfigPath() string {
	root := repoRoot()
	if root == "" {
		return ""
	}
	return filepath.Join(root, "config", "workers.yaml")
}

// yamlScalar —— 取 YAML 标量的裸值：剥两侧引号、剥行内注释（` # …`）、去首尾空白。
func yamlScalar(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, " #"); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	if len(s) >= 2 && (s[0] == '"' || s[0] == '\'') && s[len(s)-1] == s[0] {
		s = s[1 : len(s)-1]
	}
	return strings.TrimSpace(s)
}

// agentWorkersIndent —— 取一行前导空白（缩进）。
func agentWorkersIndent(line string) string {
	return line[:len(line)-len(strings.TrimLeft(line, " \t"))]
}

// parseDeviceMaxWorkers —— 从 workers.yaml 正文解析 `hostname(小写) → max_workers 原值`。
func parseDeviceMaxWorkers(text string) map[string]string {
	out := map[string]string{}
	inDev := false
	curHost := ""
	for _, l := range strings.Split(text, "\n") {
		if l != "" && !strings.HasPrefix(l, " ") && !strings.HasPrefix(l, "\t") {
			inDev = strings.TrimSpace(l) == "devices:"
			curHost = ""
			continue
		}
		if !inDev {
			continue
		}
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "- hostname:") {
			curHost = strings.ToLower(yamlScalar(strings.TrimPrefix(t, "- hostname:")))
			continue
		}
		if curHost != "" && strings.HasPrefix(t, "max_workers:") {
			out[curHost] = yamlScalar(strings.TrimPrefix(t, "max_workers:"))
		}
	}
	return out
}

// parseSchedulerSection —— 从 workers.yaml 正文解析顶层 `scheduler:` 段的 `键 → 值`。
func parseSchedulerSection(text string) map[string]string {
	out := map[string]string{}
	inSched := false
	for _, l := range strings.Split(text, "\n") {
		if l != "" && !strings.HasPrefix(l, " ") && !strings.HasPrefix(l, "\t") {
			inSched = strings.TrimSpace(l) == "scheduler:"
			continue
		}
		if !inSched {
			continue
		}
		t := strings.TrimSpace(l)
		if i := strings.Index(t, ":"); i > 0 {
			out[strings.TrimSpace(t[:i])] = yamlScalar(t[i+1:])
		}
	}
	return out
}

// agentDeviceMaxFor —— 从正文里取 `<机>` 的 max_workers 现读值。
// 两级匹配（与 workers_config.go 的 resolveByDevice **同一口径**）：先精确、再模糊（两侧非空；
// 模糊键按字面序取第一个，避免 map 迭代序带来的不确定）。返回 (值, 是否命中)。
func agentDeviceMaxFor(text, machine string) (string, bool) {
	devs := parseDeviceMaxWorkers(text)
	want := strings.ToLower(strings.TrimSpace(machine))
	if v, ok := devs[want]; ok {
		return v, true
	}
	keys := make([]string, 0, len(devs))
	for k := range devs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if k == "" || want == "" {
			continue
		}
		if strings.Contains(want, k) || strings.Contains(k, want) {
			return devs[k], true
		}
	}
	return "", false
}

// setWorkerMaxWorkers —— 把 `<机>` 条目的 `max_workers` 设成 max（就地改那一行，保留行内注释与全文其余字节）。
// 找不到该机条目 ⇒ 在 `devices:` 块末尾追加一条新条目。返回 (新正文, 是否改动, 错)。
func setWorkerMaxWorkers(text, machine string, max int) (string, bool, error) {
	lines := strings.Split(text, "\n")
	devIdx := -1
	for i, l := range lines {
		if l != "" && !strings.HasPrefix(l, " ") && !strings.HasPrefix(l, "\t") && strings.TrimSpace(l) == "devices:" {
			devIdx = i
			break
		}
	}
	if devIdx < 0 {
		return "", false, fmt.Errorf("没有顶层 `devices:` 段")
	}
	type span struct {
		start, end int
		host       string
	}
	var spans []span
	blockEnd := len(lines)
	for i := devIdx + 1; i < len(lines); i++ {
		l := lines[i]
		if l != "" && !strings.HasPrefix(l, " ") && !strings.HasPrefix(l, "\t") {
			blockEnd = i
			break
		}
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "- hostname:") {
			host := strings.ToLower(yamlScalar(strings.TrimPrefix(t, "- hostname:")))
			if len(spans) > 0 {
				spans[len(spans)-1].end = i
			}
			spans = append(spans, span{start: i, end: -1, host: host})
		}
	}
	if len(spans) > 0 && spans[len(spans)-1].end < 0 {
		spans[len(spans)-1].end = blockEnd
	}
	want := strings.ToLower(strings.TrimSpace(machine))
	target := ""
	for _, sp := range spans {
		if sp.host == want {
			target = sp.host
			break
		}
	}
	if target == "" && want != "" {
		// 模糊：与 resolveByDevice 同一口径（两侧非空；hostname 含设备名 / 设备名含 hostname）。
		for _, sp := range spans {
			if sp.host == "" {
				continue
			}
			if strings.Contains(want, sp.host) || strings.Contains(sp.host, want) {
				target = sp.host
				break
			}
		}
	}
	for _, sp := range spans {
		if target == "" || sp.host != target {
			continue
		}
		for i := sp.start; i < sp.end; i++ {
			if strings.HasPrefix(strings.TrimSpace(lines[i]), "max_workers:") {
				comment := ""
				if ci := strings.Index(lines[i], " #"); ci >= 0 {
					comment = lines[i][ci:]
				}
				lines[i] = agentWorkersIndent(lines[i]) + "max_workers: " + strconv.Itoa(max) + comment
				return strings.Join(lines, "\n"), true, nil
			}
		}
		// 条目里没有 max_workers ⇒ 插在 hostname 行之后
		ins := sp.start + 1
		add := agentWorkersIndent(lines[sp.start]) + "  max_workers: " + strconv.Itoa(max)
		out := append([]string{}, lines[:ins]...)
		out = append(out, add)
		out = append(out, lines[ins:]...)
		return strings.Join(out, "\n"), true, nil
	}
	// 没找到该机条目 ⇒ 追加（devices 块末尾）
	out := append([]string{}, lines[:blockEnd]...)
	for len(out) > 0 && strings.TrimSpace(out[len(out)-1]) == "" {
		out = out[:len(out)-1]
	}
	out = append(out, "  - hostname: "+strings.TrimSpace(machine), "    max_workers: "+strconv.Itoa(max))
	out = append(out, lines[blockEnd:]...)
	return strings.Join(out, "\n"), true, nil
}

// setSchedulerSection —— 在 `scheduler:` 段里落地 updates（有段就地改 / 无段**顶加**在 `devices:` 之前）。
// 返回 (新正文, 是否改动, 错)。
func setSchedulerSection(text string, updates map[string]string) (string, bool, error) {
	order := []string{"poll_interval", "max_retries", "cooldown"}
	lines := strings.Split(text, "\n")
	sIdx := -1
	for i, l := range lines {
		if l != "" && !strings.HasPrefix(l, " ") && !strings.HasPrefix(l, "\t") && strings.TrimSpace(l) == "scheduler:" {
			sIdx = i
			break
		}
	}
	if sIdx >= 0 {
		end := len(lines)
		for i := sIdx + 1; i < len(lines); i++ {
			l := lines[i]
			if l != "" && !strings.HasPrefix(l, " ") && !strings.HasPrefix(l, "\t") {
				end = i
				break
			}
		}
		remaining := map[string]string{}
		for k, v := range updates {
			remaining[k] = v
		}
		for i := sIdx + 1; i < end; i++ {
			t := strings.TrimSpace(lines[i])
			if j := strings.Index(t, ":"); j > 0 {
				k := strings.TrimSpace(t[:j])
				if v, ok := remaining[k]; ok {
					lines[i] = agentWorkersIndent(lines[i]) + k + ": " + v
					delete(remaining, k)
				}
			}
		}
		if len(remaining) > 0 {
			var add []string
			for _, k := range order {
				if v, ok := remaining[k]; ok {
					add = append(add, "  "+k+": "+v)
				}
			}
			out := append([]string{}, lines[:end]...)
			out = append(out, add...)
			out = append(out, lines[end:]...)
			return strings.Join(out, "\n"), true, nil
		}
		return strings.Join(lines, "\n"), true, nil
	}
	// 无段 ⇒ 顶加在 `devices:` 之前
	devIdx := -1
	for i, l := range lines {
		if l != "" && !strings.HasPrefix(l, " ") && !strings.HasPrefix(l, "\t") && strings.TrimSpace(l) == "devices:" {
			devIdx = i
			break
		}
	}
	ins := devIdx
	if ins < 0 {
		ins = len(lines)
	}
	add := []string{"scheduler:"}
	for _, k := range order {
		if v, ok := updates[k]; ok {
			add = append(add, "  "+k+": "+v)
		}
	}
	out := append([]string{}, lines[:ins]...)
	out = append(out, add...)
	out = append(out, "")
	out = append(out, lines[ins:]...)
	return strings.Join(out, "\n"), true, nil
}

// ── ① `zerg agent workers set <机> --max N`（D2 · 只写本机 config/workers.yaml）──────────────

// cmdAgentWorkersSetGuarded —— `zerg agent workers set` 的唯一执行门（D2 · 确认档齐 ⇒ 转真写）。
func cmdAgentWorkersSetGuarded(inv *invocation, stdout, stderr io.Writer) int {
	if rc := dryRunYesConflict(inv, stderr); rc != exitOK {
		return rc
	}
	if len(inv.args) == 0 {
		inv.setErr("usage", "missing_target", "缺机器名")
		fmt.Fprintf(stderr, "%s: `agent workers set` 要一个机器名（如 `%s agent workers set %s --max 2`）\n",
			progName, progName, planHost())
		return exitUsage
	}
	machine := strings.TrimSpace(inv.args[0])
	// `--max N` 必填值旗标：缺 / 坏值 / 越界 ⇒ 用法错 2 · **零写**（判在 dry-run 与写之前）。
	if !inv.hasFlag("--max") {
		inv.setErr("usage", "missing_flag", "缺 --max N")
		fmt.Fprintf(stderr, "%s: `agent workers set` 要 --max N（0 = 暂停派活 · 范围 0~%d）\n",
			progName, agentWorkersMaxCeiling)
		return exitUsage
	}
	raw := strings.TrimSpace(inv.flagVal("--max"))
	n, err := strconv.Atoi(raw)
	if err != nil {
		inv.setErr("usage", "bad_flag_value", "max 要一个整数")
		fmt.Fprintf(stderr, "%s: `--max` 要一个整数（0 = 暂停 · 范围 0~%d）—— 给的是 %q\n",
			progName, agentWorkersMaxCeiling, raw)
		return exitUsage
	}
	if n < 0 || n > agentWorkersMaxCeiling {
		inv.setErr("usage", "bad_flag_value", "max 范围 0~16")
		fmt.Fprintf(stderr, "%s: `--max` 范围 0（暂停）~ %d —— 给的是 %d\n", progName, agentWorkersMaxCeiling, n)
		return exitUsage
	}
	// ★ 远端机 fail-closed：`<机> ≠ 本机` ⇒ **明确拒绝**（需子端控制端点 · 本版未开）；✗ 禁假写/假装成功。
	if !devEditHostAccept(machine) {
		msg := fmt.Sprintf("`agent workers set` 只能设**本机**（给的是 %q，本机名是 %q）—— 设远端机的并发要子端控制端点，**本版未开** ⇒ 不执行", machine, planHost())
		inv.setErr("usage", "remote_machine_unsupported", msg)
		fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
		fmt.Fprintf(stderr, "本机并发落点 = 仓根 `config/workers.yaml`；远端机由那台机自己的 CLI 设（本版无跨机写入口 ⇒ fail-closed，绝不假写）\n")
		return exitUsage
	}
	if inv.dryRun {
		emitAgentWorkersSetPlan(stdout, stderr, machine, n)
		return exitOK
	}
	if rc := schedConfirmGate(inv, stderr, "agent workers set"); rc != exitOK {
		return rc
	}
	return cmdAgentWorkersSetWrite(inv, stdout, stderr, machine, n)
}

// emitAgentWorkersSetPlan —— `agent workers set --dry-run` 的计划件（人面 · stderr 一行收口 · 零副作用）。
func emitAgentWorkersSetPlan(stdout, stderr io.Writer, machine string, max int) {
	fmt.Fprintln(stdout, schedPlanHeader)
	fmt.Fprintf(stdout, "  动作     : %s agent workers set\n", progName)
	fmt.Fprintf(stdout, "  危险档   : %s（有影响但可逆 —— 再设回旧值即回原态；要 --confirm=<本机名> 与 --yes）\n", dangerD2)
	fmt.Fprintf(stdout, "  目标     : %s（本机）\n", machine)
	fmt.Fprintf(stdout, "  落点     : %s\n", agentWorkersConfigPath())
	fmt.Fprintf(stdout, "  它会动   : 把本机 devices 条目的 max_workers 设成 %d（0 = 暂停派活；**持久化**到 config/workers.yaml —— scheduler 下次 NewScheduler 读到生效）\n", max)
	fmt.Fprintf(stdout, "  留痕     : 写失败即拒（一个字节不落）；写完立刻重读同一件回读现读值\n")
	fmt.Fprintf(stdout, "  来源     : P0 #232 · 同族写面先例 `itask interval set`（D2 + --confirm + --yes + 落地值回读）\n")
	fmt.Fprintln(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未执行、未改任何状态）")
}

// cmdAgentWorkersSetWrite —— 真写：改 `<仓根>/config/workers.yaml` 的该机 max_workers，再**回读**核实。
func cmdAgentWorkersSetWrite(inv *invocation, stdout, stderr io.Writer, machine string, max int) int {
	path := agentWorkersConfigPath()
	if path == "" {
		inv.setErr("blocked", "no_repo_root", "取不到仓根（不给结论）")
		fmt.Fprintf(stderr, "%s: 取不到仓根 ⇒ 落点未知，不给结论（退码 8）\n", progName)
		return exitBlocked
	}
	text, err := os.ReadFile(path)
	if err != nil {
		inv.setErr("blocked", "config_unreadable", "workers.yaml 读不了")
		fmt.Fprintf(stderr, "%s: 读不了 %s：%v ⇒ 不给结论（退码 8）\n", progName, path, err)
		return exitBlocked
	}
	newText, changed, werr := setWorkerMaxWorkers(string(text), machine, max)
	if werr != nil {
		inv.setErr("blocked", "config_shape_unknown", werr.Error())
		fmt.Fprintf(stderr, "%s: workers.yaml 形状认不出：%v ⇒ 不给结论（退码 2）\n", progName, werr)
		return exitUsage
	}
	if changed {
		if err := os.WriteFile(path, []byte(newText), 0o644); err != nil {
			inv.setErr("failed", "config_write_failed", err.Error())
			fmt.Fprintf(stderr, "%s: 写 %s 失败：%v ⇒ 拒（一个字节没落）\n", progName, path, err)
			return exitFail
		}
	}
	// 落地值回读：重读同一件，报出刚写的值。
	back := "(回读不到)"
	if b, rerr := os.ReadFile(path); rerr == nil {
		if v, ok := agentDeviceMaxFor(string(b), machine); ok {
			back = v
		}
	}
	row := map[string]string{
		"machine": machine,
		"max":     strconv.Itoa(max),
		"landed":  back,
		"changed": strconv.FormatBool(changed),
		"path":    path,
	}
	return listCmd(inv, stdout, stderr, []string{"machine", "max", "landed", "changed", "path"}, []map[string]string{row})
}

// ── ② `zerg agent scheduler set [--poll-interval N] [--max-retries N] [--cooldown N]`（D2）────

// agentSchedulerKnobs —— 三枚值旗标 → 落点键（顺序即计划件/回读列序）。
var agentSchedulerKnobs = []struct{ flag, key string }{
	{"--poll-interval", "poll_interval"},
	{"--max-retries", "max_retries"},
	{"--cooldown", "cooldown"},
}

// agentScheduleNormalize —— 校验一枚旋钮的裸值，返回落点串（坏值 ⇒ ("", 说明)）。
func agentScheduleNormalize(flag, raw string) (string, string) {
	if flag == "--max-retries" {
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 {
			return "", "要正整数（死信阈值）"
		}
		return strconv.Itoa(n), ""
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return "", "要正时长（如 5s / 30s / 1m）"
	}
	return d.String(), ""
}

// cmdAgentSchedulerSetGuarded —— `zerg agent scheduler set` 的唯一执行门（D2 · 确认档齐 ⇒ 转真写）。
func cmdAgentSchedulerSetGuarded(inv *invocation, stdout, stderr io.Writer) int {
	if rc := dryRunYesConflict(inv, stderr); rc != exitOK {
		return rc
	}
	updates := map[string]string{}
	for _, k := range agentSchedulerKnobs {
		if !inv.hasFlag(k.flag) {
			continue
		}
		raw := strings.TrimSpace(inv.flagVal(k.flag))
		if raw == "" {
			inv.setErr("usage", "missing_flag_value", k.flag+" 缺值")
			fmt.Fprintf(stderr, "%s: `%s` 给了旗标却没给值 ⇒ 用法错 2（不静默当「没给」）\n", progName, k.flag)
			return exitUsage
		}
		norm, why := agentScheduleNormalize(k.flag, raw)
		if why != "" {
			inv.setErr("usage", "bad_flag_value", k.flag+" "+why)
			fmt.Fprintf(stderr, "%s: `%s` %s —— 给的是 %q\n", progName, k.flag, why, raw)
			return exitUsage
		}
		updates[k.key] = norm
	}
	if len(updates) == 0 {
		inv.setErr("usage", "missing_flag", "至少给一枚旋钮旗标")
		fmt.Fprintf(stderr, "%s: `agent scheduler set` 至少给一枚：--poll-interval <N> / --max-retries <N> / --cooldown <N>\n", progName)
		return exitUsage
	}
	if inv.dryRun {
		emitAgentSchedulerSetPlan(stdout, stderr, updates)
		return exitOK
	}
	if rc := schedConfirmGate(inv, stderr, "agent scheduler set"); rc != exitOK {
		return rc
	}
	return cmdAgentSchedulerSetWrite(inv, stdout, stderr, updates)
}

// emitAgentSchedulerSetPlan —— `agent scheduler set --dry-run` 的计划件（零副作用）。
func emitAgentSchedulerSetPlan(stdout, stderr io.Writer, updates map[string]string) {
	fmt.Fprintln(stdout, schedPlanHeader)
	fmt.Fprintf(stdout, "  动作     : %s agent scheduler set\n", progName)
	fmt.Fprintf(stdout, "  危险档   : %s（有影响但可逆 —— 再设回旧值即回原态；要 --confirm=<本机名> 与 --yes）\n", dangerD2)
	fmt.Fprintf(stdout, "  落点     : %s（顶加/更新 `scheduler:` 段）\n", agentWorkersConfigPath())
	for _, k := range agentSchedulerKnobs {
		if v, ok := updates[k.key]; ok {
			fmt.Fprintf(stdout, "  设置     : %s = %s\n", k.key, v)
		}
	}
	fmt.Fprintf(stdout, "  它会动   : 改 config/workers.yaml 的 `scheduler:` 段（**持久化**；scheduler.go 的 Start()/handleFailure() 读侧接线 —— 未给值时用编译期常量兜底）\n")
	fmt.Fprintf(stdout, "  留痕     : 写失败即拒（一个字节不落）；写完立刻重读同一件回读现读值\n")
	fmt.Fprintf(stdout, "  来源     : P0 #232 · 同族写面先例 `itask interval set`（D2 + --confirm + --yes + 落地值回读）\n")
	fmt.Fprintln(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未执行、未改任何状态）")
}

// cmdAgentSchedulerSetWrite —— 真写：改 `scheduler:` 段，再**回读**三键现读值。
func cmdAgentSchedulerSetWrite(inv *invocation, stdout, stderr io.Writer, updates map[string]string) int {
	path := agentWorkersConfigPath()
	if path == "" {
		inv.setErr("blocked", "no_repo_root", "取不到仓根（不给结论）")
		fmt.Fprintf(stderr, "%s: 取不到仓根 ⇒ 落点未知，不给结论（退码 8）\n", progName)
		return exitBlocked
	}
	text, err := os.ReadFile(path)
	if err != nil {
		inv.setErr("blocked", "config_unreadable", "workers.yaml 读不了")
		fmt.Fprintf(stderr, "%s: 读不了 %s：%v ⇒ 不给结论（退码 8）\n", progName, path, err)
		return exitBlocked
	}
	newText, changed, werr := setSchedulerSection(string(text), updates)
	if werr != nil {
		inv.setErr("blocked", "config_shape_unknown", werr.Error())
		fmt.Fprintf(stderr, "%s: workers.yaml 形状认不出：%v ⇒ 不给结论（退码 2）\n", progName, werr)
		return exitUsage
	}
	if changed {
		if err := os.WriteFile(path, []byte(newText), 0o644); err != nil {
			inv.setErr("failed", "config_write_failed", err.Error())
			fmt.Fprintf(stderr, "%s: 写 %s 失败：%v ⇒ 拒（一个字节没落）\n", progName, path, err)
			return exitFail
		}
	}
	// 落地值回读：重读同一件，报出三键现读值（刚写的值必须出现在这里）。
	back := map[string]string{}
	if b, rerr := os.ReadFile(path); rerr == nil {
		back = parseSchedulerSection(string(b))
	}
	row := map[string]string{
		"path":    path,
		"changed": strconv.FormatBool(changed),
	}
	for _, k := range agentSchedulerKnobs {
		row[k.key] = back[k.key]
	}
	return listCmd(inv, stdout, stderr, []string{"path", "poll_interval", "max_retries", "cooldown", "changed"}, []map[string]string{row})
}
