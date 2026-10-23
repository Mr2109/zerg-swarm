package main

// family_doctor_daemons.go —— `zerg doctor daemons`：本机 launchd 守护只读面（缺口 `GAP-20260926-224`）。
//
// 病（账内原话 + 收窄后的正确表述）：
//   `com.zerg.*` 五只守护（core / agent-Mr2109 / ui / tunnel.mini-node-1 / tunnel.mini-node-2）的**加载态、最近退出码、
//   重启次数**没有一次性可查的 CLI 面；`zerg core restart` 只覆盖 core 一只且以动作为主。
//   实测 `com.zerg.tunnel.mini-node-2` 反复以 255 退出（KeepAlive=true + ThrottleInterval=10 ⇒ 每 10s 重拉、
//   每次都失败），而命令行面既无读数也无告警。
//
// 本件只做一件事：**一条命令、一张表、五值判定词**（与 `doctor` 同一条闭集）。
//
// 只读纪律：
//   - 只跑 `launchctl list` 与读 plist 文本（✗ 不 kickstart / ✗ 不 bootout / ✗ 不写盘）；
//   - plist 只读 `~/Library/LaunchAgents/<label>.plist`（存在才读，读不到就照实说「找不到 plist」）；
//   - ✗ 不调主控 API、✗ 不读令牌。
//
// 判定口径（与 `doctor` / `doctor node` 同一套五值）：
//
//	PASS    —— 在跑（pid 有值）且最近退出码为 0；
//	FAIL    —— 退出码非 0 且 `KeepAlive=true`（**崩溃循环**：拉起来就死、死了又被拉起）⇒ 退码 1；
//	REPORT  —— 退出码非 0 但 KeepAlive 不为真（跑失败过一次、现在是停的，不算循环）；
//	BLOCKED —— `launchctl list` 本身跑不起来 / 输出空 ⇒ 退码 8（没结论不许当健康）。

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const daLabelPrefix = "com.zerg."

// daStableUptimeSec —— 「已稳跑」阈值（秒）：在跑且超过它 ⇒ 上一代那个非 0 退出码不判崩溃循环。
// 300s 与 KeepAlive+ThrottleInterval（10s）的节奏配套：真循环 5 分钟内必然被看见（每 10s 一次）。
const daStableUptimeSec = 300

// daDur —— 秒 ⇒ 「Nd HH:MM:SS」样式的短句（只为人眼看）。
func daDur(sec int) string {
	d := sec / 86400
	h := (sec % 86400) / 3600
	m := (sec % 3600) / 60
	ss := sec % 60
	if d > 0 {
		return fmt.Sprintf("%dd %02d:%02d:%02d", d, h, m, ss)
	}
	return fmt.Sprintf("%02d:%02d:%02d", h, m, ss)
}

// daProcUptimeSeconds —— 进程存活秒数（ps 现读；取不到 ⇒ 0 = 判不了，按「未稳跑」处理）。
func daProcUptimeSeconds(pid string) int {
	if pid == "" || pid == "-" {
		return 0
	}
	out, err := exec.Command("ps", "-o", "etime=", "-p", pid).Output()
	if err != nil {
		return 0
	}
	return daParseEtime(strings.TrimSpace(string(out)))
}

// daParseEtime —— `ps -o etime=` 的形态：[[dd-]hh:]mm:ss ⇒ 秒。
func daParseEtime(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	days := 0
	if i := strings.Index(s, "-"); i > 0 {
		if v, err := strconv.Atoi(s[:i]); err == nil {
			days = v
		}
		s = s[i+1:]
	}
	parts := strings.Split(s, ":")
	total := 0
	for _, p := range parts {
		v, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil {
			return 0
		}
		total = total*60 + v
	}
	return days*86400 + total
}

// daRow —— 一只守护的读数值（纯数据，便于单测直接喂）。
type daRow struct {
	Label      string `json:"label"`
	PID        string `json:"pid"`         // "-" = 未运行
	LastExit   string `json:"last_exit"`   // launchctl 第二列（"-" = 从没跑过/无记录）
	KeepAlive  bool   `json:"keep_alive"`  // plist 里 <key>KeepAlive</key><true/>
	PlistFound bool   `json:"plist_found"` // ~/Library/LaunchAgents/<label>.plist 在盘
	PlistPath  string `json:"plist_path"`  // 找不到时仍给出**期望路径**（便于人修）
	UptimeSec  int    `json:"uptime_sec"`  // 在跑时的进程存活秒数（0 = 未跑/取不到）
	Verdict    string `json:"verdict"`     // PASS / FAIL / REPORT / BLOCKED
	Note       string `json:"note,omitempty"`
}

// daParseList —— 解析 `launchctl list` 的输出。
//
// 形态（列以制表符分隔）：  PID\tStatus\tLabel
//
//	PID 为 "-" ⇒ 没在跑；Status 为负数是信号（-TERM 之类），正数是退出码。
func daParseList(text string) []daRow {
	var out []daRow
	sc := bufio.NewScanner(strings.NewReader(text))
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "PID") {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 3 {
			// 有些版本用多个空格；退一步按 2+ 空白切
			fields = strings.Fields(line)
		}
		if len(fields) < 3 {
			continue
		}
		label := strings.TrimSpace(fields[len(fields)-1])
		if !strings.HasPrefix(label, daLabelPrefix) {
			continue
		}
		out = append(out, daRow{
			Label:    label,
			PID:      strings.TrimSpace(fields[0]),
			LastExit: strings.TrimSpace(fields[1]),
		})
	}
	return out
}

// daPlistKeepAlive —— 从 plist 文本里取 KeepAlive（不引第三方库：两种形态都认）。
//
//	<key>KeepAlive</key><true/>        ⇒ true
//	<key>KeepAlive</key><dict>…</dict> ⇒ true（条件式常驻，也按常驻算：崩溃后同样会被拉起）
func daPlistKeepAlive(text string) bool {
	i := strings.Index(text, "<key>KeepAlive</key>")
	if i < 0 {
		return false
	}
	tail := text[i:]
	if j := strings.Index(tail, "</key>"); j >= 0 {
		tail = tail[j+len("</key>"):]
	}
	head := tail
	if len(head) > 200 {
		head = head[:200]
	}
	return strings.Contains(head, "<true/>") || strings.Contains(head, "<dict>") || strings.Contains(head, "<array>")
}

// daDecide —— 判定一只守护（纯函数，单测直接喂）。
func daDecide(r *daRow) {
	switch {
	case r.PID == "-" || r.PID == "":
		if r.LastExit != "" && r.LastExit != "0" && r.LastExit != "-" {
			if r.KeepAlive {
				r.Verdict = "FAIL"
				r.Note = "未运行 + 退出码 " + r.LastExit + " + KeepAlive=true ⇒ 崩溃循环（拉起即死）"
			} else {
				r.Verdict = "REPORT"
				r.Note = "未运行，最近一次退出码 " + r.LastExit + "（KeepAlive 不为真 ⇒ 不算循环）"
			}
			return
		}
		r.Verdict = "REPORT"
		r.Note = "未运行（无退出码记录）"
	case r.LastExit == "" || r.LastExit == "0":
		r.Verdict = "PASS"
	default:
		// 有 pid 但退出码非 0：launchctl 第二列是**上一代**（或更早）那一跑的退出码 ——
		// 现读实证：com.zerg.ui 的 pid 已连跑 1 天 0:31，退出码却还是 78（上一代留的）。
		// ★ 判据据此分两档（不然就是自己造红）：
		//   · 已稳跑（≥ daStableUptimeSec）⇒ REPORT：「曾以 X 退出，现已稳定运行」；
		//   · 刚被拉起（< 阈值）⇒ FAIL：说明它起来就又死了，这才是崩溃循环。
		if r.UptimeSec >= daStableUptimeSec {
			r.Verdict = "REPORT"
			r.Note = "在跑（pid " + r.PID + " · 已稳跑 " + daDur(r.UptimeSec) + "）；最近一次退出码 " + r.LastExit + " 是上一代实例的"
			return
		}
		if r.KeepAlive {
			r.Verdict = "FAIL"
			r.Note = "在跑（pid " + r.PID + " · 仅 " + daDur(r.UptimeSec) + "）但退出码 " + r.LastExit + " + KeepAlive=true ⇒ 反复重启"
		} else {
			r.Verdict = "REPORT"
			r.Note = "在跑（pid " + r.PID + "），最近退出码 " + r.LastExit
		}
	}
}

// daLoadLocal —— 现读本机：launchctl list + 每只的 plist。
func daLoadLocal(stdout, stderr io.Writer) ([]daRow, string) {
	cmd := exec.Command("launchctl", "list")
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	raw, err := cmd.Output()
	if err != nil {
		return nil, fmt.Sprintf("launchctl list 跑不起来（%v）—— 本机不是 launchd 环境？", err)
	}
	rows := daParseList(string(raw))
	if len(rows) == 0 {
		return nil, "launchctl list 里一条 com.zerg.* 都没有（守护没装 / 输出形态变了）"
	}
	home, herr := os.UserHomeDir()
	for i := range rows {
		path := ""
		if herr == nil {
			path = filepath.Join(home, "Library", "LaunchAgents", rows[i].Label+".plist")
			rows[i].PlistPath = path
			if b, rerr := os.ReadFile(path); rerr == nil {
				rows[i].PlistFound = true
				rows[i].KeepAlive = daPlistKeepAlive(string(b))
			}
		}
		rows[i].UptimeSec = daProcUptimeSeconds(rows[i].PID)
		daDecide(&rows[i])
	}
	_ = stderr
	return rows, ""
}

// daRC —— 整表的退码：有 FAIL ⇒ 1；有 BLOCKED ⇒ 8；否则 0。
func daRC(rows []daRow) int {
	rc := 0
	for _, r := range rows {
		switch r.Verdict {
		case "BLOCKED":
			return 8
		case "FAIL":
			rc = 1
		}
	}
	return rc
}

func daField(row daRow, name string) string {
	switch name {
	case "label":
		return row.Label
	case "pid":
		return row.PID
	case "last_exit":
		return row.LastExit
	case "keep_alive":
		return strconv.FormatBool(row.KeepAlive)
	case "plist_found":
		return strconv.FormatBool(row.PlistFound)
	case "plist_path":
		return row.PlistPath
	case "uptime_sec":
		return strconv.Itoa(row.UptimeSec)
	case "verdict":
		return row.Verdict
	case "note":
		return row.Note
	}
	return ""
}

func cmdDoctorDaemons(inv *invocation, stdout, stderr io.Writer) int {
	rows, why := daLoadLocal(stdout, stderr)
	if rows == nil {
		fmt.Fprintf(stderr, "%s: ⛔ 不给结论（退码 8）：%s\n", progName, why)
		inv.setErr("blocked", "daemons_unreadable", why)
		return 8
	}
	cols := []string{"label", "pid", "last_exit", "keep_alive", "plist_found", "uptime_sec", "verdict", "note"}
	out := make([]map[string]string, 0, len(rows))
	for _, r := range rows {
		m := map[string]string{}
		for _, c := range cols {
			m[c] = daField(r, c)
		}
		out = append(out, m)
	}
	if !inv.jsonGiven {
		fmt.Fprintf(stdout, "共 %d 只 com.zerg.* 守护（现读 launchctl list · 只读 · ✗ 不动服务）\n", len(rows))
	}
	rc := daRC(rows)
	if lrc := listCmd(inv, stdout, stderr, cols, out); lrc != exitOK {
		return lrc // 用法面错误（如 --json 不给字段）优先
	}
	if inv.jsonGiven {
		return rc
	}
	switch rc {
	case 0:
		fmt.Fprintln(stdout, "⇒ 全绿（没有崩溃循环）")
	case 1:
		fmt.Fprintln(stdout, "⇒ 判红：有守护在崩溃循环（退出码非 0 且 KeepAlive=true）")
		inv.setErr("failed", "daemon_crash_loop", "有守护在崩溃循环")
	}
	return rc
}
