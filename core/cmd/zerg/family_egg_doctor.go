// family_egg_doctor.go —— `zerg egg doctor`：卵的**体检正门**（缺口 `GAP-20261018-13` 第二层）。
//
// # 它治的是什么
//
// `egg ls` 只给一格 `state`（矩阵要能对拍、要短）；而「为什么是这一态、还差哪一面」要靠人自己
// 拼三条链：主控声明（`/api/models/{name}`）· 子端真后端（`/eggs` 的 `port`/`pid`）· 驻留账本
// （`/eggs` 的 `residency[]`，进程存在性的证据面）。真机上这一次「模型静默全消失」就是三条链
// 各说各话、却没有一条命令把它们**摆在同一行**：
//
//	声明=已加载 · 进程不在 · 端口不听 · 账本记 vanished@<时刻>
//
// ⇒ 本命令一次列出 **声明态 / 进程态 / 端口态 / GTT 占用 / 上次装载时刻 / 不一致项点名**，
// 一行一枚卵，**逐项点名**不一致（不汇总成一句「有异常」）。
//
// # 退码约定（本命令的契约 · 写死）
//
//	0  全一致（每一枚卵的「声明 ⟷ 真后端 ⟷ 账本」三面对得上）
//	1  **有不一致**（点名到卵；含「声明已加载而进程不在」「进程在而名册没有」等）
//	8  判不了（子端 `/eggs` 打不到 / 名册源读不到 / 该机名册里一枚卵都没有）
//
// ★ 优先级：**1 压过 8** —— 已经抓到的不一致是真结论，不因为另一台读不到就退回「不给结论」；
// 8 只在「一处不一致都没抓到、且有面读不到」时出现（那时确实什么都判不了）。
//
// # 只读
//
// 本命令只 GET（`/api/fleet/models` · `/api/fleet/status` · `/api/models/{name}` · 子端 `/eggs`），
// **不启停任何卵**、不写任何件。★ 也**不**自动重拉：重拉的取舍见交付件「自动重拉该不该做」一节。
package main

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
)

// eggDoctorDisplay —— 人面九格（机器面另有更细的 pid/port_live/last_seen 等格）。
var eggDoctorDisplay = []string{"host", "model", "declared", "state", "process", "port", "gtt_gb", "loaded_at", "verdict"}

// eggDoctorFields —— `--json` 字段表（比人面多 pid / port_live / residency_state / last_seen / problem）。
var eggDoctorFields = []string{
	"host", "model", "egg_id", "declared", "state", "process", "pid", "port", "port_live",
	"gtt_gb", "residency_state", "loaded_at", "last_seen", "verdict", "problem",
}

// 体检判词三词闭集（人面与机器面共用一处取值）。
const (
	eggDoctorOK      = "一致"
	eggDoctorBad     = "不一致"
	eggDoctorUnknown = "判不了"
)

// cmdEggDoctor —— `zerg egg doctor [--machine <机>] [--json <字段>]`（只读）。
func cmdEggDoctor(inv *invocation, stdout, stderr io.Writer) int {
	if inv.jsonGiven {
		if rc := requireFields(inv, stderr); rc != exitOK {
			return rc
		}
	}
	if len(inv.args) > 0 {
		eggUsageErr(inv, stderr, fmt.Sprintf("不收位置参数（多给了 %q）—— 只看一台机走 `--machine`", inv.args[0]))
		return exitUsage
	}
	only := strings.TrimSpace(inv.flagVal("--machine"))

	rows, problems, rc := projectEggs(newClient(), inv, stderr, "")
	if rc != exitOK {
		return rc
	}
	if len(problems) > 0 {
		// 名册/声明面自身对不上（主控 status 出闭集 / 点名的机不在名册）⇒ 判红点名，别继续算。
		if rc := eggRowsGuard(inv, rows, problems, stderr); rc != exitOK {
			return rc
		}
	}
	hostsInRoster := map[string]bool{}
	for _, r := range rows {
		hostsInRoster[r["host"]] = true
	}
	if only != "" {
		if !hostsInRoster[only] {
			msg := fmt.Sprintf("名册里没有 %q 这台机（或它上面一枚卵都没有）⇒ 没得体检、不给结论（退码 8）", only)
			inv.setErr("blocked", "doctor_machine_absent", msg)
			fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
			fmt.Fprintf(stderr, "看有哪些机：%s egg ls（host 那一列）\n", progName)
			return exitBlocked
		}
		kept := rows[:0]
		for _, r := range rows {
			if r["host"] == only {
				kept = append(kept, r)
			}
		}
		rows = kept
	}
	if len(rows) == 0 {
		msg := "名册展开后一枚卵都没有 ⇒ 没得体检、不给结论（退码 8 · 空结论不等于一致）"
		inv.setErr("blocked", "doctor_roster_empty", msg)
		fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
		return exitBlocked
	}
	// 先让**唯一那份**对账逻辑落定 `state`（含 `声明在但进程没了`）—— 本命令不重写判据。
	facts, rcRecon := eggReconcileBackendsWithFacts(inv, rows, stderr)

	// 逐行体检。
	judged := 0     // 真判得了的行数
	bad := 0        // 不一致行数
	unreadable := 0 // 判不了的行数
	out := make([]map[string]string, 0, len(rows))
	badList := []string{}
	for _, r := range rows {
		f := facts[r["host"]]
		dr := map[string]string{
			"host":     r["host"],
			"model":    r["model"],
			"egg_id":   r["egg_id"],
			"declared": r["declared"],
			"state":    r["state"],
			"gtt_gb":   f.GTT[r["model"]],
		}
		if rec, ok := f.Residency[r["model"]]; ok {
			dr["residency_state"] = rec.State
			dr["loaded_at"] = rec.LoadedAt
			dr["last_seen"] = rec.LastSeen
			dr["port_live"] = rec.PortLive
		}
		_, live := f.Running[r["model"]]
		if pid := f.PIDs[r["model"]]; pid > 0 {
			dr["pid"] = strconv.Itoa(pid)
			dr["process"] = fmt.Sprintf("在(pid=%d)", pid)
		} else if live {
			dr["process"] = "在(pid 未报)"
		} else if rec, ok := f.Residency[r["model"]]; ok && rec.State == residencyStateVanished {
			dr["process"] = "不在（账本记 vanished）"
			if rec.PID > 0 {
				dr["pid"] = strconv.Itoa(rec.PID)
			}
		} else if ok && rec.State == eggResidencyStateLoaded {
			// 账本说装过，而子端此刻既不在管它、也不报活进程 ⇒ 句柄丢了 / 进程没了。
			dr["process"] = "不在（账本=loaded，子端未报）"
		} else {
			dr["process"] = "不在（无装载证据）"
		}
		if live {
			dr["port"] = ":" + strconv.Itoa(f.Running[r["model"]])
		} else if rec, ok := f.Residency[r["model"]]; ok && rec.Port > 0 {
			dr["port"] = fmt.Sprintf(":%d（已停）", rec.Port)
		} else {
			dr["port"] = "—"
		}

		// 三面对账（★ 每一处不一致都**点名到卵 + 说清是哪一面**）。
		var why []string
		if !f.Reachable {
			dr["verdict"] = eggDoctorUnknown
			unreadable++
			why = append(why, "子端 `/eggs` 读不到 ⇒ 进程/端口两面判不了（"+orX(f.Why, "未给原因")+"）")
		} else {
			judged++
			switch {
			case live && r["declared"] != eggStateLoaded:
				why = append(why, "真后端在跑，而主控声明不是「已加载」（声明滞后）")
			case !live && r["declared"] == eggStateLoaded:
				if rec, ok := f.Residency[r["model"]]; ok && rec.State == residencyStateVanished {
					why = append(why, fmt.Sprintf("**进程没了**：账本记 vanished，最后见到活着 %s（pid=%d · :%d）",
						orX(rec.LastSeen, "（未记）"), rec.PID, rec.Port))
				} else {
					why = append(why, "主控声明「已加载」，而子端**没有任何装载证据**（从未装上 / 起不来）")
				}
			}
			if rec, ok := f.Residency[r["model"]]; ok && live && rec.PortLive == "false" {
				why = append(why, "进程在，但它该在听的端口拨不通（半死：请求会打到空处）")
			}
			if rec, ok := f.Residency[r["model"]]; ok && rec.State == residencyStateVanished && r["declared"] != eggStateLoaded {
				why = append(why, fmt.Sprintf("账本记 vanished（%s 后进程没了），主控已不声明已加载 —— 掉线已发生、需人确认要不要重装",
					orX(rec.LastSeen, "（未记时刻）")))
			}
			if len(why) > 0 {
				dr["verdict"] = eggDoctorBad
				bad++
			} else {
				dr["verdict"] = eggDoctorOK
			}
		}
		if len(why) > 0 {
			dr["problem"] = strings.Join(why, "；")
			badList = append(badList, fmt.Sprintf("%s%s%s：%s", r["model"], eggSep, r["host"], dr["problem"]))
		}
		out = append(out, dr)
	}

	// ★ 「进程在，而名册里没有这一枚」——声明面漏项也是不一致（第二类负控的正题）。
	extraBad := 0
	for _, host := range eggDoctorSortedHosts(facts) {
		f := facts[host]
		if !f.Reachable {
			continue
		}
		declaredHere := map[string]bool{}
		for _, r := range rows {
			if r["host"] == host {
				declaredHere[r["model"]] = true
			}
		}
		models := make([]string, 0, len(f.Running))
		for m := range f.Running {
			models = append(models, m)
		}
		sort.Strings(models)
		for _, m := range models {
			if declaredHere[m] {
				continue
			}
			dr := map[string]string{
				"host": host, "model": m, "egg_id": m + eggSep + host, "declared": "（名册没有）",
				"state": eggStateLoaded, "process": fmt.Sprintf("在(pid=%s)", orX(strconv.Itoa(f.PIDs[m]), "0")),
				"port": ":" + strconv.Itoa(f.Running[m]), "gtt_gb": f.GTT[m],
				"verdict": eggDoctorBad,
				"problem": fmt.Sprintf("**进程在跑（:%d）而名册里没有这一枚** ⇒ 声明面漏项（它不在任何一枚卵的名下）", f.Running[m]),
			}
			if rec, ok := f.Residency[m]; ok {
				dr["residency_state"], dr["loaded_at"], dr["last_seen"], dr["port_live"] = rec.State, rec.LoadedAt, rec.LastSeen, rec.PortLive
			}
			out = append(out, dr)
			bad++
			extraBad++
			badList = append(badList, fmt.Sprintf("%s%s%s：%s", m, eggSep, host, dr["problem"]))
		}
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i]["host"] != out[j]["host"] {
			return out[i]["host"] < out[j]["host"]
		}
		return out[i]["model"] < out[j]["model"]
	})
	// 汇总三格一律**从最终行集里数**（含上面补进来的「名册没有」行）—— 不许与表体各算一套，
	// 否则会出现「表里一行写着在(pid=7777)、汇总却说真在跑 0 枚」这种自相矛盾（夹具里真撞到过）。
	liveCount, vanishedCount, gttSum := 0, 0, 0.0
	for _, r := range out {
		if strings.HasPrefix(r["process"], "在(") {
			liveCount++
		}
		if r["residency_state"] == residencyStateVanished {
			vanishedCount++
		}
		if v := r["gtt_gb"]; v != "" {
			if x, err := strconv.ParseFloat(v, 64); err == nil {
				gttSum += x
			}
		}
	}

	// 退码：1 压过 8（见件头「退码约定」）。
	rcOut := exitOK
	switch {
	case bad > 0:
		rcOut = exitFail
	case unreadable > 0 || rcRecon != exitOK:
		rcOut = exitBlocked
	}

	if inv.jsonGiven {
		if jrc := selectJSONList(stdout, stderr, inv, inv.path, inv.fields, out); jrc != exitOK {
			return jrc
		}
	} else {
		renderRows(stdout, stderr, inv.tty && !inv.plain, eggDoctorDisplay, doctorTable(out))
		fmt.Fprintf(stdout, "\n汇总：卵 %d 枚 · 声明已加载 %d 枚 · 真在跑 %d 枚 · 账本记 vanished %d 枚 · GTT 合计 %.1f GiB\n",
			len(out), countDeclaredLoaded(out), liveCount, vanishedCount, gttSum)
		if len(badList) > 0 {
			fmt.Fprintf(stdout, "不一致 %d 项（逐项点名）：\n", len(badList))
			for _, b := range badList {
				fmt.Fprintf(stdout, "  - %s\n", b)
			}
		} else if rcOut == exitOK {
			fmt.Fprintf(stdout, "不一致 0 项（每一枚卵：声明 ⟷ 真后端 ⟷ 驻留账本 三面对得上）\n")
		}
		fmt.Fprintf(stdout, "判词：%s\n", doctorVerdictWord(rcOut))
		if rcOut == exitBlocked {
			fmt.Fprintf(stderr, "%s: 有面读不到 ⇒ 部分卵**判不了**（退码 8 · 不给结论；已抓到的不一致会优先判红 1）\n", progName)
		}
	}
	fmt.Fprintf(stderr, "%s: 只读体检 —— 不启停任何卵、不写任何件（重拉的取舍见交付件；本命令**不自动重拉**）\n", progName)
	return rcOut
}

// doctorTable 体检行 → 人面九格（缺值一律 `—`，✗ 不留空白格让人以为「0」）。
func doctorTable(rows []map[string]string) [][]string {
	table := make([][]string, 0, len(rows))
	for _, r := range rows {
		line := make([]string, 0, len(eggDoctorDisplay))
		for _, f := range eggDoctorDisplay {
			line = append(line, orX(strings.TrimSpace(r[f]), "—"))
		}
		table = append(table, line)
	}
	return table
}

// doctorVerdictWord 退码 → 判词（人面最后一行；三词闭集）。
func doctorVerdictWord(rc int) string {
	switch rc {
	case exitOK:
		return eggDoctorOK + "（退码 0）"
	case exitFail:
		return eggDoctorBad + "（退码 1 —— 逐项见上「不一致」）"
	default:
		return eggDoctorUnknown + "（退码 8 —— 有面读不到，不给结论）"
	}
}

// countDeclaredLoaded 声明为「已加载」的行数（汇总行用）。
func countDeclaredLoaded(rows []map[string]string) int {
	n := 0
	for _, r := range rows {
		if r["declared"] == eggStateLoaded {
			n++
		}
	}
	return n
}

// sortedKeys 定序的键（体检输出必须可对拍 ⇒ 次序不许随 map 走）。
func eggDoctorSortedHosts(m map[string]eggBackendFact) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
