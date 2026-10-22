// family_core_load.go —— `zerg core load`：主控当前负载的**只读可见面**（缺口 `GAP-20261021-03`）。
//
// 病灶（缺口原文）：主控进程长时间满载做 ONNX 推理而无人知晓 —— pid 16730 持续占 897% CPU
// （9 核）约 33 小时，采样热栈全是 MlasSgemm/ParallelFor/Compute，心跳日志仍正常（不是死锁，
// 是真在算）；而 `core status` 只给 `load_tasks=true` 与 pid/version，**看不到**在算什么 /
// 为谁算 / 还要多久，也没有 CPU 与耗时自监控。
//
// 本面只做**读**（✗ 不碰调度、✗ 不 kill、✗ 不写任何状态件、✗ 不新开路由）：
//
//	① 主控自报：`GET /api/core/status`（pid / version / started_at / load_tasks）—— 复用既有只读面；
//	② 本机现测：拿同一个 pid 跑**两帧** `ps -o time=,etime=,rss=`，按「ΔCPU 秒 ÷ Δ墙钟秒」算
//	   **瞬时**占用（`ps` 的一核 = 100%）。
//	   ★ 为什么非得两帧差分：`ps -o %cpu=` 报的是**进程整个生命期**的平均值 —— 一枚跑了 33 小时的
//	   进程即便此刻满载，读数也会被前面 33 小时摊薄（897% 是**事故时**的峰值读数）。只读一帧
//	   平均值的面**结构性看不见**这次事故，正是缺口「没有 CPU 自监控」那一层。
//	③ 判读：≥ 满档线 ⇒ `满载`；≥ 忙档线 ⇒ `忙`；否则 `空闲`；采不到 ⇒ 不给结论（退码 `8`）。
//	   档线按**本机核数**折算（`ps` 一核 = 100%，n 核机的满档 = `100·n`）—— 9 核占用在 10 核机上
//	   是 `满载`，在 32 核机上只是 `忙`，同一个数字不许用同一把尺子判。
//	④ 在算什么 / 为谁算 / 还要多久：逐枚 `load_tasks` 出 `model`（算什么）· `machine`（为谁算）·
//	   `phase` / `elapsed_s`（已经算了多久）；有 `expand` 进度（`bytes_done`/`bytes_total`）时按
//	   「已完成字节 ÷ 已耗时」推 `eta_s`（还要多久）。**没有进度读数 ⇒ `eta_s` 整格不给** —— 不编一个数。
//
// 退码（§九 M9：读不到不许当健康）：`0` 读到并判出 · `8` 主控在答但 pid 采不到（blocked）·
// `12` 打不到主控（unreachable · 由 `fetchInv` 判）· `2` 用法错。
package main

import (
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// coreLoadFields —— `--json` 可取的全部字段（一条命令一张表 · 只增不改）。
//
// 形状约定与同族 `core ps` / `core db` 一致：取不到的格**整键略去**（✗ 绝不落 `0` / 空串）。
var coreLoadFields = []string{
	"level", "note",
	"pid", "version", "started_at", "uptime",
	"cpu_pct", "cpu_cores", "ncpu", "load1", "rss_mb",
	"tasks", "inflight",
	"task_id", "machine", "model", "phase", "elapsed_s", "eta_s", "progress",
}

// coreLoadDisplay —— 人面（非 `--json`）的表头列序：先判读（level），再看读数，最后看任务。
var coreLoadDisplay = []string{
	"level", "cpu_pct", "cpu_cores", "uptime", "model", "machine", "phase", "elapsed_s", "eta_s", "note",
}

// 判读档线（**唯一真源** · 按本机核数折算 ⇒ 同一条判据在大机小机上各自成立）。
const (
	coreLoadSaturatedRatio = 0.80 // 满档：吃掉本机 ≥ 80% 的算力
	coreLoadBusyRatio      = 0.25 // 忙档：≥ 25%
	coreLoadWindow         = 500 * time.Millisecond
)

// coreLoadPSCmd —— `ps` 的**唯一**调用口（单测换掉它 ⇒ 判据不依赖本机真进程）。
var coreLoadPSCmd = func(args ...string) (string, error) {
	out, err := exec.Command("ps", args...).Output()
	return string(out), err
}

// coreLoadSleep —— 采样窗口的等待口（单测换掉它 ⇒ 不真睡）。
var coreLoadSleep = func(d time.Duration) { time.Sleep(d) }

// coreLoadSpec —— 本命令的**登记形**。
//
// ★ 登记行应落在 `core/cmd/zerg/main.go` 的命令树里（本枚**不动**那一处：它正被另一路改写）。
// 这里保留同一份 spec 有两个用途：① 登记之前单测也能拼出六键包封；② `find()` 命中登记行时
// **以登记行为准**（`fields` 同一份表 ⇒ 两处不漂）。
func coreLoadSpec() *command {
	if c := find([]string{"core", "load"}); c != nil {
		return c
	}
	return &command{
		path:     []string{"core", "load"},
		kind:     "CoreLoad",
		usage:    "zerg core load [--json <字段>]",
		fields:   coreLoadFields,
		endpoint: "GET /api/core/status（＋本机 `ps` 两帧差分）",
	}
}

// coreLoadLevel —— 占用 ⇒ 判读词（`ok=false` ⇒ `未知`：采不到就是采不到，不落 `空闲`）。
//
// `cpuPct` 的口径 = `ps` 的一核百分比（n 核机满载 ≈ `100·n`）。
func coreLoadLevel(cpuPct float64, ncpu int, ok bool) string {
	if !ok || ncpu <= 0 || cpuPct < 0 {
		return "未知"
	}
	switch {
	case cpuPct >= coreLoadSaturatedRatio*float64(ncpu)*100:
		return "满载"
	case cpuPct >= coreLoadBusyRatio*float64(ncpu)*100:
		return "忙"
	}
	return "空闲"
}

// coreLoadProcSample —— 一枚主控进程的两帧差分采样结果。
type coreLoadProcSample struct {
	Pid     int
	CpuPct  float64 // 采样窗口内的瞬时占用（一核 = 100%）
	RssMB   float64
	UptimeS float64 // 进程已运行时长（`etime`）
	CpuSecs float64 // 进程累计 CPU 秒（第二帧）
	OK      bool
	Detail  string // 采不到时的原因（⇒ 调用方退 8，不报绿）
	WindowS float64
}

// sampleCoreProc —— 对 `pid` 跑两帧 `ps` 算瞬时占用（只读：`ps` 不碰目标进程）。
//
// 帧形 = `ps -p <pid> -o time=,etime=,rss=`（三格：累计 CPU 时间 / 已运行 / 常驻 KB）。
func sampleCoreProc(pid int) coreLoadProcSample {
	s := coreLoadProcSample{Pid: pid, WindowS: coreLoadWindow.Seconds()}
	if pid <= 0 {
		s.Detail = "主控没报 pid（`/api/core/status` 的 `pid` 取不到整数）"
		return s
	}
	args := []string{"-p", strconv.Itoa(pid), "-o", "time=,etime=,rss="}
	f1, err := coreLoadPSCmd(args...)
	t1 := time.Now()
	if err != nil {
		s.Detail = fmt.Sprintf("`ps -p %d` 跑不起来：%v", pid, err)
		return s
	}
	coreLoadSleep(coreLoadWindow)
	f2, err := coreLoadPSCmd(args...)
	t2 := time.Now()
	if err != nil {
		s.Detail = fmt.Sprintf("第二帧 `ps -p %d` 跑不起来：%v", pid, err)
		return s
	}
	cpu1, up1, rss1, ok1 := parsePSProcFrame(f1)
	cpu2, up2, rss2, ok2 := parsePSProcFrame(f2)
	if !ok1 || !ok2 {
		s.Detail = "`ps` 两帧读不出三格（累计 CPU 时间 / 已运行 / 常驻）—— 进程可能刚退出"
		return s
	}
	wall := t2.Sub(t1).Seconds()
	if wall <= 0 {
		wall = coreLoadWindow.Seconds()
	}
	pct := (cpu2 - cpu1) / wall * 100
	if pct < 0 { // 两帧之间发生计数器回绕 / 解析噪声 ⇒ 不落负数
		pct = 0
	}
	if rss2 <= 0 {
		rss2 = rss1
	}
	if up2 < up1 {
		up2 = up1
	}
	s.CpuPct = pct
	s.RssMB = rss2 / 1024
	s.UptimeS = up2
	s.CpuSecs = cpu2
	s.WindowS = wall
	s.OK = true
	return s
}

// parsePSProcFrame —— 把一帧 `time=,etime=,rss=` 拆成（累计 CPU 秒 / 已运行秒 / 常驻 KB）。
// 三格缺一或坏值 ⇒ `ok=false`（不拿坏读数顶替）。
func parsePSProcFrame(out string) (cpuS, uptimeS, rssKB float64, ok bool) {
	f := strings.Fields(strings.TrimSpace(out))
	if len(f) < 3 {
		return 0, 0, 0, false
	}
	cpuS, okC := parsePSTime(f[0])
	if !okC {
		return 0, 0, 0, false
	}
	uptimeS, okU := parsePSEtime(f[1])
	if !okU {
		return 0, 0, 0, false
	}
	r, err := strconv.ParseFloat(strings.ReplaceAll(f[2], ",", ""), 64)
	if err != nil || r < 0 {
		return 0, 0, 0, false
	}
	return cpuS, uptimeS, r, true
}

// parsePSTime —— `ps -o time=` 的取值 ⇒ 秒。形态：`MM:SS.ss` / `HH:MM:SS` / `D-HH:MM:SS`
// （BSD 的分钟位可超 60，按「段数」而不是「60 进制」解释）。
func parsePSTime(s string) (float64, bool) {
	return parsePSClock(s, false)
}

// parsePSEtime —— `ps -o etime=` 的取值 ⇒ 秒。同上形态（BSD 同样会带 `D-` 天前缀）。
func parsePSEtime(s string) (float64, bool) {
	return parsePSClock(s, true)
}

// parsePSClock —— 两枚时钟串的**同一套**解析（`etime` 与 `time` 只差「天前缀」的口径，本实现两处都认）。
func parsePSClock(s string, _ bool) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	days := 0.0
	if i := strings.IndexByte(s, '-'); i > 0 {
		d, err := strconv.Atoi(s[:i])
		if err != nil || d < 0 {
			return 0, false
		}
		days = float64(d)
		s = s[i+1:]
	}
	parts := strings.Split(s, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, false
	}
	vals := make([]float64, len(parts))
	for i, p := range parts {
		v, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
		if err != nil || v < 0 {
			return 0, false
		}
		vals[i] = v
	}
	sec := vals[len(vals)-1]
	min := vals[len(vals)-2]
	hr := 0.0
	if len(vals) == 3 {
		hr = vals[0]
	}
	return days*86400 + hr*3600 + min*60 + sec, true
}

// coreLoadProgress —— 迭代进度的读数（`expand` 子对象）⇒ 一句人读文本；没有就不给（返回空串）。
func coreLoadProgress(o jsonObj) string {
	ex := asObj(o["expand"])
	if ex == nil {
		return ""
	}
	done, okD := numOf(ex["bytes_done"])
	total, okT := numOf(ex["bytes_total"])
	state := cell(ex["state"])
	if !okD || !okT || total <= 0 {
		return strings.TrimSpace(state)
	}
	pct := done / total * 100
	return fmt.Sprintf("%s %.1f%%（%.2f/%.2f GB）", strings.TrimSpace(state), pct, done/(1<<30), total/(1<<30))
}

// coreLoadEta —— 「还要多久」：按「已完成字节 ÷ 已耗时」的速率外推剩余字节。
//
// ★ 判据只认**真读数**：没有 `expand` 进度、或已完成 0 字节（速率无从算）⇒ `ok=false`
// ⇒ 调用方**整格不给**（✗ 不编一个 `eta=0` 冒充「马上好」）。单位 = 秒。
func coreLoadEta(o jsonObj) (float64, bool) {
	ex := asObj(o["expand"])
	if ex == nil {
		return 0, false
	}
	done, okD := numOf(ex["bytes_done"])
	total, okT := numOf(ex["bytes_total"])
	elapsed, okE := numOf(o["elapsed_s"])
	if !okE {
		elapsed, okE = numOf(ex["elapsed_s"])
	}
	if !okD || !okT || !okE || done <= 0 || total <= 0 || elapsed <= 0 || done >= total {
		return 0, false
	}
	rate := done / elapsed // 字节/秒
	eta := (total - done) / rate
	if eta < 0 {
		return 0, false
	}
	return eta, true
}

// coreLoadHumanSecs —— 秒 ⇒ 人读时长（`33h01m` / `4m07s` / `9s`）。
func coreLoadHumanSecs(sec float64) string {
	if sec < 0 {
		return ""
	}
	s := int64(sec + 0.5)
	switch {
	case s >= 86400:
		return fmt.Sprintf("%dd%02dh", s/86400, (s%86400)/3600)
	case s >= 3600:
		return fmt.Sprintf("%dh%02dm", s/3600, (s%3600)/60)
	case s >= 60:
		return fmt.Sprintf("%dm%02ds", s/60, s%60)
	}
	return fmt.Sprintf("%ds", s)
}

// coreLoadNote —— 判读的一句话解释（人面 `note` 格）：**判词 ＋ 读数 ＋ 口径**，不只印数字。
func coreLoadNote(level string, s coreLoadProcSample, ncpu int, l1 float64, loadOK bool, tasks, inflight int) string {
	if !s.OK {
		return "CPU 采样拿不到 ⇒ 负载不给结论（缺口账口径：读不到不许当健康）"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s：%.1f%% ≈ %.2f 核 / %d 核", level, s.CpuPct, s.CpuPct/100, ncpu)
	fmt.Fprintf(&b, " · 采样窗口 %.0fms 两帧差分（非生命期均值）", s.WindowS*1000)
	if loadOK {
		fmt.Fprintf(&b, " · load1=%.2f", l1)
	}
	fmt.Fprintf(&b, " · 在装 %d 枚（accepted %d）", tasks, inflight)
	if level == "满载" {
		b.WriteString(" ⇒ 主控正在吃满本机算力：先看下面「model / machine」是谁在算、`elapsed_s` 跑了多久")
	}
	return b.String()
}

// coreLoadBadField —— `--json` 点到未登记字段：与同族（`reportBadField`）**同 kind 同 detail 同退码**，
// 但字段表取自本件（命令树尚未登记时也能把「合法字段」列全）。
func coreLoadBadField(inv *invocation, stderr io.Writer, bad string) int {
	inv.setErr("usage", "json_field_unknown:"+bad, fmt.Sprintf("未知字段 %q", bad))
	fmt.Fprintf(stderr, "%s: 未知字段 %q\n", progName, bad)
	fmt.Fprintf(stderr, "合法字段: %s\n", strings.Join(coreLoadFields, ","))
	fmt.Fprintf(stderr, "See '%s --help'。\n", progName)
	return exitUsage
}

// cmdCoreLoad —— `zerg core load`：主控当前负载（只读）。
func cmdCoreLoad(inv *invocation, stdout, stderr io.Writer) int {
	// ① 字段面前置判定（**先判后碰**：坏字段 ⇒ 零副作用退 2，不去打主控、不去采样）
	if inv.jsonGiven {
		if len(inv.fields) == 0 {
			return requireFields(inv, stderr)
		}
		allowed := make(map[string]bool, len(coreLoadFields))
		for _, f := range coreLoadFields {
			allowed[f] = true
		}
		for _, f := range inv.fields {
			if !allowed[f] {
				return coreLoadBadField(inv, stderr, f)
			}
		}
	}

	// ② 主控自报（既有只读面 · 不新开路由）
	c := newClient()
	var resp jsonObj
	if rc := fetchInv(inv, c, "/api/core/status", &resp, stderr); rc != exitOK {
		return rc
	}
	pidStr := strings.TrimSpace(cell(resp["pid"]))
	pid, _ := strconv.Atoi(pidStr)
	version := cell(resp["version"])
	startedAt := cell(resp["started_at"])
	tasks := asList(resp["load_tasks"])

	// ③ 本机现测（只读 `ps` 两帧差分）
	smp := sampleCoreProc(pid)
	ncpu := hostNCPU()
	l1, _, _, loadOK := hostLoadAvg()
	level := coreLoadLevel(smp.CpuPct, ncpu, smp.OK)
	if !smp.OK {
		inv.setErr("blocked", "cpu_sample_absent", smp.Detail)
		fmt.Fprintf(stderr, "%s: 主控在答（pid=%s）但本机 CPU 采样拿不到：%s\n", progName, pidStr, smp.Detail)
		fmt.Fprintf(stderr, "⇒ 负载**不给结论**（退码 8 · 读不到不许当健康）；主控自身的 pid/version 仍可看 `%s core status`\n", progName)
		return exitBlocked
	}

	inflight := 0
	for _, it := range tasks {
		if o := asObj(it); o != nil && cell(o["phase"]) == "accepted" {
			inflight++
		}
	}

	base := map[string]string{
		"level":      level,
		"note":       coreLoadNote(level, smp, ncpu, l1, loadOK, len(tasks), inflight),
		"pid":        pidStr,
		"version":    version,
		"started_at": startedAt,
		"uptime":     coreLoadHumanSecs(smp.UptimeS),
		"cpu_pct":    fmt.Sprintf("%.1f", smp.CpuPct),
		"cpu_cores":  fmt.Sprintf("%.2f", smp.CpuPct/100),
		"ncpu":       strconv.Itoa(ncpu),
		"rss_mb":     fmt.Sprintf("%.0f", smp.RssMB),
		"tasks":      strconv.Itoa(len(tasks)),
		"inflight":   strconv.Itoa(inflight),
	}
	if loadOK {
		base["load1"] = fmt.Sprintf("%.2f", l1)
	}

	// ④ 在算什么 / 为谁算 / 还要多久 —— 逐枚在装任务一行；一枚都没有 ⇒ 只出汇总行
	rows := make([]map[string]string, 0, len(tasks))
	for _, it := range tasks {
		o := asObj(it)
		if o == nil {
			continue
		}
		row := make(map[string]string, len(base)+6)
		for k, v := range base {
			row[k] = v
		}
		row["task_id"] = cell(o["task_id"])
		row["machine"] = cell(o["machine"])
		row["model"] = cell(o["model"])
		row["phase"] = cell(o["phase"])
		if el, ok := numOf(o["elapsed_s"]); ok {
			row["elapsed_s"] = fmt.Sprintf("%.0f", el)
		}
		if eta, ok := coreLoadEta(o); ok {
			row["eta_s"] = fmt.Sprintf("%.0f", eta)
		}
		if p := coreLoadProgress(o); p != "" {
			row["progress"] = p
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		rows = append(rows, base)
	}

	if inv.jsonGiven {
		objs := make([]string, 0, len(rows))
		for _, row := range rows {
			objs = append(objs, marshalObjectRegistered(inv.fields, row))
		}
		emitEnvelopeWith(stdout, coreLoadSpec(), "["+strings.Join(objs, ",")+"]", len(rows), inv)
		return exitOK
	}

	table := make([][]string, 0, len(rows))
	for _, r := range rows {
		line := make([]string, 0, len(coreLoadDisplay))
		for _, f := range coreLoadDisplay {
			line = append(line, r[f])
		}
		table = append(table, line)
	}
	renderRows(stdout, stderr, inv.tty && !inv.plain, coreLoadDisplay, table)
	fmt.Fprintf(stderr, "%s: 判读 = 主控进程 %s（pid=%s · 已运行 %s）· 只读：没碰调度、没停进程、没写状态件\n",
		progName, level, pidStr, coreLoadHumanSecs(smp.UptimeS))
	return exitOK
}
