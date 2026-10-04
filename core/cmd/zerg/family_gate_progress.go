// family_gate_progress.go —— `zerg gate progress`（只读快照）＋ `zerg gate wait`（有界阻塞）。
//
// 设计稿（真源）：设计-门进度可查-v0.3-20261002.md §三 / §六 / §十。
//
// 五条口径（逐条落地）：
//
//	① progress = **纯只读快照、立即返回、绝不阻塞**（§3.1 / §十.6）；退码 0=读到 · 1=门禁有红 ·
//	   2=用法错 · 8=判不出（§3.6 · 沿用退码表既有号，**不新增裸数字**）。
//	② 判活**同源** `family_gate_reap.go`（§3.5 / §六）：直接复用它的 `ps` 解析 / 谱系 / 活性原语
//	   （`gateReapScan` / `gateReapRoots` / `gateReapLiveInTree` / `gateReapAllStat` / `gateReapEtimeSecs` …），
//	   **绝不另写一份**。★ `family_gate_reap.go` 是**禁改件** ⇒ 「抽公共函数」以**同 package 直接调用**达成。
//	③ 进程树**一次** `ps -axo`（§十.3 / §十.9）：身份键 `(pid, start_time=lstart)`；**限定整趟进程组**
//	   内找最长路径的叶子当 deepest；**深度上限 8**，到顶报「更深层未展开」。
//	④ **挑趟口径**（缺口 `GAP-20261002-17` ＋ `GAP-20261004-05`）：`--last` 与正门 `gate results --last`
//	   **同口径** —— 复用 `gateFindLiveRun`（扫**全部**趟根 · **优选活趟**，否则按 `results.tsv` **mtime
//	   最新**挑），**绝不按目录名倒序**；并在「趟：」行明示依据。
//	⑤ **判活口径**（缺口 `GAP-20261002-16`）：`progress` 是**单快照** ⇒ **只判第 3 层（日志 mtime 推进）＋
//	   pcpu>0**、**不装三层**（§十.2）；**完整三层取或**只在**有时间窗口**的 `gate wait` 落（见 gateWaitAdvancing）。
//
// ★ `--json`（§十.5）：走设计稿的 **JSONL** 面 —— 首行 `{"type":"schema",…,"version":"1.0"}`，
// 其后逐条带 `type`；字段 `snake_case`；时间带偏移 RFC3339（另给单调秒字段）；未知值「省略键 ＋
// `<字段>_known:false`」；数组恒 `[]`。★ 这条与仓内六键包封（`zerg/v1`）**不同** —— 差异与理由见回执。
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	gateProgressSchemaName  = "zerg.gate.progress"
	gateProgressVersion     = "1.1" // §十.5：向后兼容的**增补**只涨 minor（加 elapsed_secs / slowest_steps · GAP-20261002-19）
	gateProgressDepthLimit  = 8     // §十.3 铁律三
	gateProgressSilentDef   = 120   // §十.2：silent_secs 报成 `x/120`
	gateProgressListLimit   = 20    // §十.9：历史趟懒加载默认只取最新 20
	gateProgressSlowDef     = 3     // GAP-20261002-19：`步耗时：最慢 N 步` 默认 N=3
	gateProgressSlowCap     = 20    // GAP-20261002-19：`--limit` 覆盖**带上限**（与 §十.9 的 `--limit` 同天花板）
	gateWaitDefaultTimeout  = 300   // §十.6：`--timeout` **默认有限**（不许 0=无穷）
	gateWaitDefaultInterval = 2     // §十.2：可见刷新 2~3s
)

// gateProgressFields —— `zerg gate progress --json` 的字段表（snake_case · §十.5）。
// ★ 必须写成 `[]string{…}` **字面量**（契约脚本的 `FIELDS_RE` 只认字面量）。
var gateProgressFields = []string{
	"run_dir", "finished", "alive", "root_pid", "origin_kind", "origin",
	"steps_done", "steps_total", "steps_source_sha",
	"current_step", "current_step_name", "current_secs", "elapsed_secs",
	"last_activity_ts", "silent_secs", "silent_threshold",
	"pass", "fail", "blocked", "report", "exit_code", "slowest_steps",
	"advancing", "deepest_pid", "deepest_cmd", "deepest_secs", "depth_limit_reached",
}

// gateWaitFields —— `zerg gate wait --json` 的字段表（在 progress 面之上续等待四格）。
var gateWaitFields = append(append([]string{}, gateProgressFields...),
	"waited_secs", "timed_out", "interval", "timeout")

// ── 进程树：一次 `ps -axo`（§十.3 / §十.9）──────────────────────────────────────────────────

type gateProcNode struct {
	pid   int
	ppid  int
	pgid  int
	stat  string
	cpu   string // `ps -o time=` 累计 CPU（libproc 不可用时的兜底判据 · §十.1）
	start string // `ps lstart=` 起跑时刻（身份键第二元 · 防 PID 复用 · §十.3 铁律一）
}

// gateProcTable —— **一次** `ps -axo pid=,ppid=,pgid=,stat=,time=,lstart=`（§十.9 预算内的一次 syscall）。
// 取不到 ⇒ 第二个返回值为假（调用方判「读不到不当没有」）。
func gateProcTable() (map[int]gateProcNode, bool) {
	out, err := exec.Command("ps", "-axo", "pid=,ppid=,pgid=,stat=,time=,lstart=").Output()
	if err != nil || len(out) == 0 {
		return nil, false
	}
	m := map[int]gateProcNode{}
	for _, ln := range strings.Split(string(out), "\n") {
		f := strings.Fields(ln)
		if len(f) < 5 {
			continue
		}
		pid, e1 := strconv.Atoi(f[0])
		ppid, e2 := strconv.Atoi(f[1])
		pgid, e3 := strconv.Atoi(f[2])
		if e1 != nil || e2 != nil || e3 != nil {
			continue
		}
		start := ""
		if len(f) > 5 {
			start = strings.Join(f[5:], " ") // lstart 含空格 ⇒ 余下全是起跑时刻
		}
		m[pid] = gateProcNode{pid: pid, ppid: ppid, pgid: pgid, stat: f[3], cpu: f[4], start: start}
	}
	return m, true
}

// gateProcDeepest —— 在**整趟进程组**内、自 rootPid 沿 ppid 链找**最长路径的叶子**（§十.3）。
// 深度上限 8；到顶仍未见底 ⇒ capped=true（报「更深层未展开」，不许假装到底）。取不到根 ⇒ ok=false。
func gateProcDeepest(procs map[int]gateProcNode, rootPid int) (pid int, depth int, capped, ok bool) {
	root, found := procs[rootPid]
	if !found {
		return 0, 0, false, false
	}
	pgid := root.pgid
	kids := map[int][]int{}
	for p, n := range procs {
		if n.pgid != pgid { // ★ 限定整趟进程组（§十.3 铁律二：父子链会被 reparent 打断，PGID 稳定）
			continue
		}
		kids[n.ppid] = append(kids[n.ppid], p)
	}
	best, bestDepth := rootPid, 0
	var walk func(p int, d int)
	walk = func(p int, d int) {
		cs := kids[p]
		if len(cs) == 0 {
			if d > bestDepth {
				best, bestDepth = p, d
			}
			return
		}
		if d >= gateProgressDepthLimit {
			capped = true
			return
		}
		for _, c := range cs {
			walk(c, d+1)
		}
	}
	walk(rootPid, 0)
	return best, bestDepth, capped, true
}

// ── 门进程表匹配（复用 reap 的扫描原语 · §3.5 同源）─────────────────────────────────────────

// gateProgressOutdirFromCmd —— 从一条门进程命令行取 `--outdir`（缺省档退回含 `zerg-gates-` 的 token）。
func gateProgressOutdirFromCmd(cmd string) string {
	fs := strings.Fields(cmd)
	for i := 0; i < len(fs); i++ {
		if fs[i] == "--outdir" && i+1 < len(fs) {
			return fs[i+1]
		}
		if strings.HasPrefix(fs[i], "--outdir=") {
			return strings.TrimPrefix(fs[i], "--outdir=")
		}
	}
	for _, t := range fs {
		if k := strings.Index(t, gateRunPrefix); k >= 0 {
			return t[k:]
		}
	}
	return ""
}

// gateProgressProcForDir —— 在门进程表里找**属于这一趟**的根（cmd 含该目录 / 其 basename，或 `--outdir` 相等）。
func gateProgressProcForDir(gate map[int]gateProc, dir string) (int, bool) {
	base := filepath.Base(dir)
	for pid, gp := range gate {
		if gp.cmd == "" {
			continue
		}
		if strings.Contains(gp.cmd, dir) || (base != "" && strings.Contains(gp.cmd, base)) {
			return pid, true
		}
		if d := gateProgressOutdirFromCmd(gp.cmd); d != "" && d == dir {
			return pid, true
		}
	}
	return 0, false
}

// ── holder 记录（§十.7 · 读侧宽容缺 outdir —— outdir 由 `-11` 补写）────────────────────────

type gateProgressHolder struct {
	pid     int
	start   string
	step    string
	cmd     string
	outdir  string
	present bool
}

func gateProgressReadHolder() gateProgressHolder {
	var h gateProgressHolder
	b, err := os.ReadFile(gateInstanceLockPath())
	if err != nil {
		return h
	}
	h.present = true
	for _, ln := range strings.Split(string(b), "\n") {
		k, v, ok := strings.Cut(ln, ":")
		if !ok {
			continue
		}
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		switch k {
		case "pid":
			h.pid, _ = strconv.Atoi(v)
		case "start":
			h.start = v
		case "step":
			h.step = v
		case "cmd":
			h.cmd = v
		case "outdir":
			h.outdir = v
		}
	}
	return h
}

// ── 趟目录发现（§十.9：优先 holder/当前趟指针，退而 readdir 按名倒序 —— 名含时间戳 ⇒ 倒序=最新）──

// gateProgressListDirs —— **全部趟根**下的 `zerg-gates-*` 目录（只 readdir，不 per-entry stat），按名**倒序**。
func gateProgressListDirs() ([]string, error) {
	dirs, err := gateRunDirsAll()
	if err != nil {
		return nil, err
	}
	sort.Sort(sort.Reverse(sort.StringSlice(dirs)))
	return dirs, nil
}

// gateProgressNewestRun —— 最新一趟（**优选有活趟**；没有活的 ⇒ 按 `results.tsv` 的 mtime 最新挑）。
//
// ★ 缺口 `GAP-20261002-17` ＋ `GAP-20261004-05`：**口径与正门 `zerg gate results --last` 逐字相同** ——
// 直接复用其 `gateFindLiveRun`（扫**全部**趟根下所有 `zerg-gates-*`、**优选活趟**，否则取 `results.tsv`
// mtime 最新那一份），**绝不按目录名字面倒序挑**（名字倒序 ≠ 写入顺序：实测会挑中陈年目录，并把它的
// `steps_total` 报成一个不属于本版脚本的数）。单一真源，不另写一份。
func gateProgressNewestRun() (string, bool, error) {
	d, _, ok, err := gateFindLiveRun()
	return d, ok, err
}

// gateProgressStepTable —— 步表来源（脚本 sha 前 16 位 ＋ 步数：**现读**，不硬编码 158 · §3.3 G9）。
// ★ 缺口 `GAP-20261004-07`（分母与 `zerg gate ls` 同源）：分母要数的是**本趟真会跑的步** ——
// `parseAddSteps` 会把脚本里**全部** `add_step` 声明都算进来，其中 `self_test()` 里的 47 条
// `add_step self "自检-…"` 是**只在 `--self-test` 档才 add** 的自检夹具步，默认全量趟不跑它们。
// 现读铁证：全表 236 条 ＝ 189 真步 ＋ 47 self 夹具，而 `zerg gate ls`（＝脚本 `--list`）现印
// 「共 189 步」⇒ 拿 236 当分母，进度恒报 N/236（单步趟也报 1/236），与本趟实跑行数永远对不上。
// ⇒ 只数 **scope ≠ self** 的那些（＝脚本 `--list` 的真步数）。
func gateProgressStepTable(root string) (count int, sha string, ok bool) {
	if root == "" {
		return 0, "", false
	}
	b, err := os.ReadFile(filepath.Join(root, gateScriptRel))
	if err != nil {
		return 0, "", false
	}
	sum := sha256.Sum256(b)
	for _, d := range parseAddSteps(string(b)) {
		if d.Scope == "self" { // self_test() 里的自检夹具步：不在默认全量趟里
			continue
		}
		count++
	}
	return count, hex.EncodeToString(sum[:])[:16], true
}

// gateProgressStepLogs —— 该趟日志件：最大步号 / 当前步名 / 最新 mtime / 件数。目录不可读 ⇒ ok=false。
func gateProgressStepLogs(dir string) (maxNum int, curName string, newest time.Time, count int, ok bool) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return 0, "", time.Time{}, 0, false
	}
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".log") {
			continue
		}
		num, _, has := strings.Cut(e.Name(), "-")
		n, err := strconv.Atoi(num)
		if err != nil || !has {
			continue
		}
		count++
		st, err := e.Info()
		if err != nil {
			continue
		}
		if n > maxNum {
			maxNum, curName = n, e.Name()
		}
		if st.ModTime().After(newest) {
			newest = st.ModTime()
		}
	}
	return maxNum, curName, newest, count, true
}

// ── 步耗时（GAP-20261002-19）────────────────────────────────────────────────────────────

// gateProgressStepCost —— 一条「已完成步耗时」（读 `results.tsv` 第 4 列）。
type gateProgressStepCost struct {
	name string
	secs int64
}

// gateProgressStartTime —— 这一趟的**不可变起跑时刻**（GAP-20261002-19 的 B）。
//
// ★ 取法：趟目录内**最早那个文件**的 mtime —— 各步日志件与 `results.tsv` 都在起跑时创建，
// 之后只**追加**（`results.tsv` 的 mtime 会涨，当不了最小 ⇒ 不影响）。**绝不**取目录 mtime
// 或任何会被写入刷新的产物 mtime：旧实现正是拿 `max(results.tsv mtime, 最新日志 mtime)` 当起跑
// 时刻 ⇒ 每写一次就归零（实测趟已跑 6 分 24 秒时印成 `已 00:00:09`）。取不到 ⇒ ok=false。
func gateProgressStartTime(dir string) (time.Time, bool) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return time.Time{}, false
	}
	var earliest time.Time
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		st, err := e.Info()
		if err != nil {
			continue
		}
		m := st.ModTime()
		if earliest.IsZero() || m.Before(earliest) {
			earliest = m
		}
	}
	if earliest.IsZero() {
		return time.Time{}, false
	}
	return earliest, true
}

// gateProgressParseSecs —— 解 `results.tsv` 第 4 列的秒数（`103s` / `2s` / `0s`；纯数字也认）。
func gateProgressParseSecs(s string) (int64, bool) {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, "s")
	if s == "" {
		return 0, false
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n < 0 {
		return 0, false
	}
	return n, true
}

// gateProgressSlowest —— 已完成步按第 4 列秒数降序取前 n（GAP-20261002-19：默认 3 · `--limit` 覆盖带上限）。
func gateProgressSlowest(rows []gateResultsRow, n int) []gateProgressStepCost {
	if n <= 0 {
		n = gateProgressSlowDef
	}
	if n > gateProgressSlowCap {
		n = gateProgressSlowCap
	}
	costs := []gateProgressStepCost{}
	for _, r := range rows {
		if s, ok := gateProgressParseSecs(r.Secs); ok {
			costs = append(costs, gateProgressStepCost{name: r.Name, secs: s})
		}
	}
	sort.SliceStable(costs, func(a, b int) bool { return costs[a].secs > costs[b].secs })
	if len(costs) > n {
		costs = costs[:n]
	}
	return costs
}

// gateProgressSlowestJSON —— 最慢步清单 ⇒ JSON 数组（恒 `[]`，绝不 `null` · §十.5）。
func gateProgressSlowestJSON(items []gateProgressStepCost) string {
	parts := make([]string, 0, len(items))
	for _, s := range items {
		parts = append(parts, "{\"step\":"+gpJSONStr(s.name)+",\"secs\":"+strconv.FormatInt(s.secs, 10)+"}")
	}
	return "[" + strings.Join(parts, ",") + "]"
}

// ── 八格读数（§3.2）──────────────────────────────────────────────────────────────────────

type gateProgressInfo struct {
	runDir     string
	found      bool
	finished   bool
	alive      bool
	aliveKnown bool
	rootPid    int
	originKind string // machine / residue / unknown
	origin     string

	stepsDone    int
	stepsTotal   int
	stepsTotalOK bool
	stepsSrcSHA  string

	currentStep int
	currentName string
	currentSecs int64
	currentOK   bool

	// GAP-20261002-19：elapsed_* = **整趟已耗时**（now − 不可变起跑时刻 · 趟目录内最早文件 mtime）；
	// slowest = 已完成步最慢 N 条（读 `results.tsv` 第 4 列）。
	elapsedSecs int64
	elapsedOK   bool
	slowest     []gateProgressStepCost

	lastActivity time.Time
	lastOK       bool
	logMtime     time.Time
	silentSecs   int64
	clockSkew    bool
	silentN      int

	pass, fail, blocked, report int
	exitCode                    int
	countsOK                    bool

	advancing bool
	advKnown  bool
	advBasis  string

	// pickReason —— GAP-17：这一趟按什么依据挑中（在「趟：」行明示）。
	pickReason string

	deepestPid  int
	deepestCmd  string
	deepestSecs int64
	depthCapped bool
	deepestOK   bool
}

func (i *gateProgressInfo) silentThreshold() int {
	if i == nil || i.silentN <= 0 {
		return gateProgressSilentDef
	}
	return i.silentN
}

// gateProgressCollect —— 引擎：读一趟的八格（**只读** · 绝不写盘 / 碰进程）。
// 返回（读数 · 退码）：退码 0=读到 · 8=判不出（空趟 / 不可读 / 进程树取不到 · §3.8）。
func gateProgressCollect(dir, root string, silentN, slowN int) (*gateProgressInfo, int) {
	info := &gateProgressInfo{runDir: dir, silentN: silentN}
	if strings.TrimSpace(dir) == "" {
		return info, exitBlocked
	}
	// ① 结果表（真源）：读不到 / 空表 ⇒ 判不出（§3.8）。
	tsv := filepath.Join(dir, gateResultsTSV)
	rows, _, err := gateResultsReadTSV(tsv)
	if err != nil {
		return info, exitBlocked
	}
	if len(rows) == 0 {
		return info, exitBlocked
	}
	info.found = true
	counts := map[string]int{}
	for _, r := range rows {
		counts[r.Status]++
	}
	info.pass, info.fail, info.blocked, info.report = counts["PASS"], counts["FAIL"], counts["BLOCKED"], counts["REPORT"]
	info.stepsDone = len(rows)
	info.countsOK = true
	info.exitCode = gateResultsExitCode(counts)

	// ② 步表来源（现读 · 不硬编码 158）。
	if c, sha, ok := gateProgressStepTable(root); ok {
		info.stepsTotal, info.stepsTotalOK, info.stepsSrcSHA = c, true, sha
	}

	// ③ 日志件（当前步 = 最大已写日志步号；未超过已完步数时取 steps_done+1 · §3.3 / G5）。
	maxNum, curName, newest, _, _ := gateProgressStepLogs(dir)
	if maxNum > info.stepsDone {
		info.currentStep = maxNum
	} else {
		info.currentStep = info.stepsDone + 1
	}
	info.currentName = curName
	info.logMtime = newest // GAP-16 第 3 层信号：最新步日志件 mtime

	// ④ 最后活动 = max(results.tsv mtime, 最新日志 mtime)（§3.3 第一硬信号）。
	act := time.Time{}
	if st, e := os.Stat(tsv); e == nil {
		act = st.ModTime()
	}
	if newest.After(act) {
		act = newest
	}
	if !act.IsZero() {
		info.lastOK, info.lastActivity = true, act
		d := int64(time.Since(act).Seconds())
		if d < 0 {
			d, info.clockSkew = 0, true // §3.3 G8：负数一律 0 并标注时钟跳变
		}
		info.silentSecs = d
	}
	// ④′ 该步已跑 = now − **该步日志件**的 mtime（GAP-20261002-19 · 与整趟耗时**分开**）：
	//     日志件边跑边写 ⇒ `newest` 就是当前步那份 `NN-*.log` 的 mtime（§3.3 current_secs）。
	if !newest.IsZero() {
		s := int64(time.Since(newest).Seconds())
		if s < 0 {
			s = 0
		}
		info.currentSecs, info.currentOK = s, true
	}
	// ④″ 整趟已耗时 = now − **不可变**的起跑时刻（GAP-20261002-19 的 B：**不许再取会变的产物 mtime**）。
	if st, ok := gateProgressStartTime(dir); ok {
		e := int64(time.Since(st).Seconds())
		if e < 0 {
			e = 0
		}
		info.elapsedSecs, info.elapsedOK = e, true
	}
	// ④‴ 本趟最慢 N 步 = `results.tsv` 第 4 列秒数降序取前 N（GAP-20261002-19）。
	info.slowest = gateProgressSlowest(rows, slowN)

	// ⑤ 进程面（与 reap **同源**）：门进程表 + 全表 + 活性。
	gate, gateOK := gateReapScan()
	procs, procsOK := gateProcTable()
	if !gateOK && !procsOK {
		return info, exitBlocked // §3.8：进程树取不到（权限）⇒ 判不了，不许写「已收工」
	}
	info.aliveKnown = true
	if gateOK {
		if pid, ok := gateProgressProcForDir(gate, dir); ok {
			info.alive, info.rootPid = true, pid
			info.originKind, info.origin = "machine", "zerg gate run"
			if gate[pid].ppid == 1 {
				info.originKind = "residue" // §3.7 取法 2：祖链已断（ppid=1）⇒ 残留茬
			}
		}
	}
	if h := gateProgressReadHolder(); h.present && h.pid > 0 && (h.outdir == "" || h.outdir == dir) {
		if _, live := procs[h.pid]; live {
			info.alive = true
			if info.rootPid == 0 {
				info.rootPid = h.pid
			}
			info.originKind = "machine"
			if h.cmd != "" {
				info.origin = h.cmd
			}
		}
	}
	// ★ 缺口 `GAP-20261004-05`：门进程命令行没带 `--outdir`（脚本默认命名档）、且它的持锁者不在
	//   **本查询进程**的根里时，上面两条都认不出「这一趟有活门跑」⇒ 会误判「已收工 · 根 pid 0」
	//   （实测：活趟 `/var/folders/…/T/zerg-gates-…` 报「已收工 · 根 pid 0」而同一屏又印「最后活动
	//   距今 0 秒」＝自相矛盾）。⇒ 补一条**根级**判活：这一趟的根里有活的单实例持锁者、且它是该根
	//   下最新候选（扫**全部趟根**，不只本进程那一个）。
	if gateOK && !info.alive {
		if pid, ok := gateRootLiveHolder(dir, gate); ok {
			info.alive, info.rootPid = true, pid
			info.originKind, info.origin = "machine", "zerg gate run"
		}
	}
	if !info.alive {
		info.finished = true // 趟目录在、产物读到、无活进程 ⇒ 已收工（§3.9 退化为 results 视图）
	}

	// ⑥ 最深进程（整趟进程组内最长的叶子 · §十.3）。
	if procsOK && info.rootPid > 0 {
		pid, _, capped, ok := gateProcDeepest(procs, info.rootPid)
		if ok {
			info.deepestOK, info.deepestPid, info.depthCapped = true, pid, capped
			if cmds := gateReapAllCmds(); cmds != nil {
				info.deepestCmd = cmds[pid]
			}
			if st := gateReapAllStat(); st != nil {
				if s, ok2 := st[pid]; ok2 {
					info.deepestSecs = int64(gateReapEtimeSecs(s.etime))
				}
			}
		}
	}

	// ⑦ advancing —— ★★ GAP-20261002-16：**`gate progress` 是单快照，只判两层中的一层 ＋ pcpu**，不装三层。
	//    设计稿 §十.2 自己写过「**progress 自己不做长采样**（它是快照命令）」⇒ 单快照根本拿不到「两帧」，
	//    故此处**只**能落：第 3 层（当前步日志件 mtime 在结果表之后仍被写）＋ reap 同源的「谱系内 pcpu>0」。
	//    ★ 完整三层（活动计数 / PID 换过 / 日志 mtime）只在**有时间窗口**的 `gate wait` 里做（见 gateWaitAdvancing）。
	//    两层都取不到 ⇒ `advKnown=false`（**明写判不了，不许猜** · §3.4）。
	if info.alive {
		tsvM := time.Time{}
		if st, e := os.Stat(tsv); e == nil {
			tsvM = st.ModTime()
		}
		if !newest.IsZero() && newest.After(tsvM.Add(time.Second)) {
			info.advancing, info.advKnown, info.advBasis = true, true, "progress:log_mtime" // 第 3 层
		} else if info.rootPid > 0 {
			stat := gateReapAllStat()
			allPPID := gateReapAllPPID()
			if stat != nil && allPPID != nil {
				if live, _, _ := gateReapLiveInTree(info.rootPid, allPPID, stat); live {
					info.advancing, info.advKnown, info.advBasis = true, true, "progress:pcpu" // 谱系 pcpu>0
				}
			}
		}
	}
	// ⑧ 退码（§3.6 · 缺口 `GAP-20261002-20`）：读到 ⇒ 0；**已收工且那一趟有红** ⇒ 1。
	//    ★ 在跑的趟即使表里已有 FAIL 也退 0 —— 「读成功」≠「门禁绿」，两者必须分开（gh 的 watch 陷阱反面）。
	if info.finished && info.fail > 0 {
		return info, exitFail
	}
	return info, exitOK
}

// ── 命令面（`gate progress`）─────────────────────────────────────────────────────────────

// cmdGateProgress —— `zerg gate progress [--dir <趟> | --last] [--json <字段>]`。
func cmdGateProgress(inv *invocation, stdout, stderr io.Writer) int {
	dir := strings.TrimSpace(inv.flagVal("--dir"))
	if dir != "" && inv.last {
		inv.setErr("usage", "both_sources", "两个来源不许混")
		fmt.Fprintf(stderr, "%s: `--dir` 与 `--last` 是**两个来源** ⇒ 同给 = 用法错 2\n", progName)
		return exitUsage
	}
	if len(inv.args) > 0 {
		inv.setErr("usage", "extra_args", "多余位置参数")
		fmt.Fprintf(stderr, "%s: `gate progress` 不收位置参数（那一趟由 `--dir` / `--last` 指）—— 多给了 %q\n", progName, inv.args[0])
		return exitUsage
	}
	// 非 TTY：`--pick` 拒执（§3.1 增补 G4 · 绝不弹提示、绝不读 stdin）。
	if inv.hasFlag("--pick") && !inv.tty {
		inv.setErr("usage", "pick_needs_tty", "非 TTY 禁用 --pick")
		fmt.Fprintf(stderr, "%s: stdout 不是终端 ⇒ `--pick` 拒执（退码 2）—— 请用 `--dir <目录>` 点名\n", progName)
		return exitUsage
	}
	if inv.jsonGiven {
		if len(inv.fields) == 0 {
			return requireFields(inv, stderr) // K2：给了 --json 不给字段 ⇒ 用法码 2 + stderr 列字段表
		}
		for _, f := range inv.fields {
			if !gateProgressFieldLegal(f, gateProgressFields) {
				inv.setErr("usage", "json_field_unknown:"+f, "未知字段")
				return reportBadField(stderr, inv.path, f)
			}
		}
	}
	if inv.list {
		return gateProgressList(inv, stdout, stderr)
	}
	silent := gateProgressIntFlag(inv, "--silent-threshold", gateProgressSilentDef)
	slowN := gateProgressIntFlag(inv, "--limit", gateProgressSlowDef) // GAP-20261002-19：`--limit` 覆盖「最慢 N 步」的 N（带上限）
	root := repoRoot()

	resolved, multi, rc := gateProgressResolveDir(inv, dir, stderr)
	if rc != exitOK {
		return rc
	}
	// ★ 缺口 `GAP-20261002-17`：记下「这一趟按什么依据挑中」⇒ 在「趟：」行明示（用户要能看见口径）。
	pick := ""
	switch {
	case dir != "":
		pick = "由 `--dir` 点名"
	case inv.last:
		pick = "优选活趟、否则按最新写入挑（全趟根 · 与 `gate results --last` 同口径）"
	}
	if resolved == "" && len(multi) > 1 {
		// §3.7.1：多趟活 = 病症。**打印候选清单 + 非零退 2**，绝不弹提示、绝不读 stdin（§十.4）。
		inv.setErr("usage", "multiple_live_runs", "多趟活")
		fmt.Fprintf(stderr, "%s: ⚠ 同时有 %d 趟活 —— 单实例闸未管住残留茬，建议 `zerg gate reap --stale`\n", progName, len(multi))
		fmt.Fprintf(stderr, "候选趟（请用 `--dir <目录>` 点名）：\n")
		for _, d := range multi {
			fmt.Fprintf(stderr, "  %s\n", d)
		}
		fmt.Fprintf(stderr, "%s: 不猜哪一趟 —— 退码 2\n", progName)
		return exitUsage
	}
	if resolved == "" && len(multi) == 1 {
		resolved = multi[0]
		pick = "有活进程的趟（唯一）"
	}
	if resolved == "" {
		resolved, _, _ = gateProgressNewestRun() // 无活趟 ⇒ 退最近一趟（§3.1）
		if pick == "" {
			pick = "无活趟 ⇒ 按最新写入退最近一趟（全趟根 · 与 `gate results --last` 同口径）"
		}
	}

	info, code := gateProgressCollect(resolved, root, silent, slowN)
	info.pickReason = pick
	if inv.jsonGiven {
		return gateProgressEmitJSON(inv, stdout, info, code)
	}
	gateProgressEmitHuman(stdout, info, code)
	if code == exitBlocked {
		inv.setErr("blocked", "gate_progress_unreadable", "判不出")
		fmt.Fprintf(stderr, "%s: 判不出（读不到那一趟 / 空趟 / 进程树取不到）⇒ 不给结论（退码 8）· 不写「已收工」\n", progName)
	}
	return code
}

// ── 命令面（`gate wait`）─────────────────────────────────────────────────────────────────

// cmdGateWait —— `zerg gate wait [--interval <秒>] [--timeout <秒>] [--exit-status]`。
// **有界阻塞**（§十.6）：`--timeout` **默认有限**；把 interval/timeout 打印出来。
func cmdGateWait(inv *invocation, stdout, stderr io.Writer) int {
	interval := gateProgressIntFlag(inv, "--interval", gateWaitDefaultInterval)
	timeout := gateProgressIntFlag(inv, "--timeout", gateWaitDefaultTimeout)
	if interval <= 0 {
		interval = gateWaitDefaultInterval
	}
	if timeout <= 0 { // 显式 0 / 负 ⇒ 拒执（**不学 kubectl 的 0=无穷** · §十.6）
		inv.setErr("usage", "timeout_unbounded", "timeout 必须有限")
		fmt.Fprintf(stderr, "%s: `--timeout` 必须**有限**（0/负 不作「无穷」解 —— 不学 `kubectl rollout status` · §十.6）⇒ 退码 2\n", progName)
		return exitUsage
	}
	if len(inv.args) > 0 {
		inv.setErr("usage", "extra_args", "多余位置参数")
		fmt.Fprintf(stderr, "%s: `gate wait` 不收位置参数 —— 多给了 %q\n", progName, inv.args[0])
		return exitUsage
	}
	dir := strings.TrimSpace(inv.flagVal("--dir"))
	silent := gateProgressIntFlag(inv, "--silent-threshold", gateProgressSilentDef)
	if inv.jsonGiven {
		if len(inv.fields) == 0 {
			return requireFields(inv, stderr)
		}
		for _, f := range inv.fields {
			if !gateProgressFieldLegal(f, gateWaitFields) {
				inv.setErr("usage", "json_field_unknown:"+f, "未知字段")
				return reportBadField(stderr, inv.path, f)
			}
		}
	}
	root := repoRoot()
	fmt.Fprintf(stderr, "%s: 等待门跑收工（interval=%ds · timeout=%ds · dir=%s）\n", progName, interval, timeout, dashIfEmpty(dir))

	started := time.Now()
	deadline := started.Add(time.Duration(timeout) * time.Second)
	var last *gateProgressInfo
	// ★★ 缺口 `GAP-20261002-16`：`gate wait` **有时间窗口** ⇒ 在**这里**落**完整三层**判活（§十.1）；
	//   与 `gate progress` 的快照口径（只判第 3 层＋pcpu · 注释见 gateProgressCollect ⑦）**不同**，
	//   两边口径差见 gateWaitAdvancing 头注与各自人读面的「推进判据」行。
	var prevFrame gateWaitFrame
	prevHave := false
	for {
		resolved := dir
		pick := ""
		if resolved == "" {
			r, multi, rc := gateProgressResolveDir(inv, "", stderr)
			if rc != exitOK {
				return rc
			}
			if r != "" {
				resolved = r
			} else if len(multi) >= 1 {
				resolved = multi[0]
				pick = "有活进程的趟"
			} else {
				resolved, _, _ = gateProgressNewestRun()
				pick = "无活趟 ⇒ 按最新写入退最近一趟（全趟根 · 与 `gate results --last` 同口径）"
			}
		} else {
			pick = "由 `--dir` 点名"
		}
		info, code := gateProgressCollect(resolved, root, silent, gateProgressSlowDef)
		if info != nil {
			info.pickReason = pick
		}
		// ★ 跨帧采样的**完整三层**（活动计数 / PID 换过 / 日志 mtime）—— 有第二帧后才成立。
		if info != nil && info.alive {
			cur := gateWaitCapture(info)
			if prevHave {
				if adv, known := gateWaitAdvancing(prevFrame, cur); known {
					info.advancing, info.advKnown, info.advBasis = adv, true, "wait:3layer"
				}
			}
			prevFrame, prevHave = cur, true
		}
		last = info
		waited := int(time.Since(started).Seconds())
		if code == exitBlocked {
			if inv.jsonGiven {
				return gateProgressEmitJSONWait(inv, stdout, info, exitBlocked, interval, timeout, waited, false)
			}
			gateProgressEmitHuman(stdout, info, code)
			fmt.Fprintf(stderr, "%s: 判不出 ⇒ 不给结论（退码 8 · 读不到不当健康）\n", progName)
			return exitBlocked
		}
		if info != nil && !info.alive {
			break // 收工
		}
		if time.Now().After(deadline) {
			if inv.jsonGiven {
				return gateProgressEmitJSONWait(inv, stdout, info, exitTimeout, interval, timeout, waited, true)
			}
			gateProgressEmitHuman(stdout, info, exitOK)
			fmt.Fprintf(stderr, "%s: 等超时（%ds）⇒ 不给结论（不判死、不处置 · §十.2 / Q4）· 退码 %d\n", progName, timeout, exitTimeout)
			return exitTimeout
		}
		time.Sleep(time.Duration(interval) * time.Second)
	}
	// 收工：默认 0（读成功）；`--exit-status` 把门禁的红透成退码（§十.6）。
	code := exitOK
	if last != nil {
		code = last.exitCode
	}
	if inv.hasFlag("--exit-status") {
		switch code {
		case 1: // 那一趟有 FAIL（脚本口径）
			code = exitFail
		case 2: // 只有 BLOCKED ⇒ 判不出档（沿用退码表既有号）
			code = exitBlocked
		default:
			code = exitOK
		}
	} else {
		code = exitOK
	}
	if inv.jsonGiven {
		return gateProgressEmitJSONWait(inv, stdout, last, code, interval, timeout, int(time.Since(started).Seconds()), false)
	}
	gateProgressEmitHuman(stdout, last, code)
	return code
}

// ── `gate wait` 专用：两帧采样的**完整三层**判活（§十.1 · 缺口 `GAP-20261002-16`）──────────────
//
// ★ 两边口径差（**不许两边各说一套** —— 两处注释与帮助面必须一致）：
//   · `gate progress`（**单快照**）：只判 **第 3 层（当前步日志 mtime 推进）＋ 谱系 pcpu>0**
//     （见 gateProgressCollect ⑦）—— **不做长采样**（**设计稿 §十.2 自己写的**：progress 是快照命令）。
//   · `gate wait`（**有界阻塞 · 有时间窗口**）：落 **完整三层取或**（本段）：
//     ① 活动计数增长 ② 最深层 PID 换过 ③ 当前步日志 mtime 推进。
//   ★ 「活动计数」首选 `proc_pid_rusage`（`ri_diskio_bytesread/written` · `ri_interrupt_wkups` ·
//     `ri_user_time+ri_system_time` · §十.1）；本包 **cgo 关闭**、不引 libproc ⇒ 取设计稿认可的
//     **`ps -o time=`（累计 CPU）兜底**（§十.1 尾注）：CPU 涨 ⇒ 在推进。
//   ★ **单帧不算**：两帧都在才判；两帧间 `ps` 精度是厘秒、窗口过短时 `delta=0` 是**量化噪声**不是卡死
//     （§十.2 / §十.10）⇒ 故 **跨帧取或**（任一层动即判推进），**不靠单帧缺席判死**。

type gateWaitFrame struct {
	deepestPid int
	cpuSecs    float64
	logMtime   time.Time
}

// gateWaitCapture —— 采一帧（当前读数的最深 pid ＋ 其累计 CPU ＋ 最新步日志 mtime）。
func gateWaitCapture(info *gateProgressInfo) gateWaitFrame {
	var f gateWaitFrame
	if info == nil {
		return f
	}
	f.deepestPid = info.deepestPid
	if s, ok := gateWaitCpuSecs(info.deepestPid); ok {
		f.cpuSecs = s
	}
	f.logMtime = info.logMtime
	return f
}

// gateWaitCpuSecs —— `ps -o time= -p <pid>` 的累计 CPU 秒（`[[DD-]HH:]MM:SS.hh` · 厘秒精度 · §十.10）。
// ★ 这是 §十.1 里 `proc_pid_rusage` 的**降级兜底**（本包 cgo 关闭）—— 只看得见 CPU，看不见 IO 等待
//
//	（阻塞在 IO 时它不涨）；故它**只作三层里的一层**，另两层（PID 换过 / 日志 mtime 推进）补盲区。
func gateWaitCpuSecs(pid int) (float64, bool) {
	if pid <= 0 {
		return 0, false
	}
	out, err := exec.Command("ps", "-o", "time=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return 0, false
	}
	return gateWaitParseTime(strings.TrimSpace(string(out)))
}

// gateWaitParseTime —— 解 `ps -o time=` 的 `[[DD-]HH:]MM:SS.hh`（厘秒 ⇒ 秒）。
func gateWaitParseTime(s string) (float64, bool) {
	if s == "" {
		return 0, false
	}
	days := 0.0
	if i := strings.IndexByte(s, '-'); i >= 0 {
		d, err := strconv.Atoi(s[:i])
		if err != nil {
			return 0, false
		}
		days, s = float64(d), s[i+1:]
	}
	parts := strings.Split(s, ":")
	if len(parts) > 3 {
		return 0, false
	}
	total := 0.0
	for _, p := range parts {
		v, err := strconv.ParseFloat(p, 64)
		if err != nil {
			return 0, false
		}
		total = total*60 + v
	}
	return days*86400 + total, true
}

// gateWaitAdvancing —— 两帧 **完整三层取或**（§十.1）：任一层推进 ⇒ advancing=true。
// 两帧都在但三层都没动 ⇒ advancing=false（**仍只报读数、不下判定** · Q4）。
// 取不到可比的两帧 ⇒ known=false（**明写判不了**，不许猜）。
func gateWaitAdvancing(prev, cur gateWaitFrame) (advancing, known bool) {
	// 层② 最深层 PID 换过（zerg 现场已验的简化信号）—— 两个 pid 都取到且不同才作数。
	if prev.deepestPid > 0 && cur.deepestPid > 0 && prev.deepestPid != cur.deepestPid {
		return true, true
	}
	// 层① 活动计数增长：累计 CPU 涨（`ps -o time=` 兜底；`proc_pid_rusage` 见本段头注）。
	if cur.cpuSecs > prev.cpuSecs {
		return true, true
	}
	// 层③ 当前步日志件 mtime 推进（对「边跑边写日志」最灵敏）。
	if !prev.logMtime.IsZero() && cur.logMtime.After(prev.logMtime) {
		return true, true
	}
	// 两帧都取到了可比信号 ⇒ 判得出「三层都没动」；否则判不了。
	if prev.deepestPid > 0 && cur.deepestPid > 0 {
		return false, true
	}
	if !prev.logMtime.IsZero() || !cur.logMtime.IsZero() {
		return false, true
	}
	return false, false
}

// ── 辅助：解析趟目录 / 发现活趟 ───────────────────────────────────────────────────────────

// gateProgressResolveDir —— 解「要读哪一趟」。返回（单一趟 · 多趟清单 · 退码）。
// 多趟活 ⇒ 清单非空、单一趟空（调用方据 §3.7.1 报病症退 2）。
func gateProgressResolveDir(inv *invocation, dir string, stderr io.Writer) (string, []string, int) {
	if dir != "" {
		return dir, nil, exitOK
	}
	if inv.last {
		// ★ GAP-20261002-17 ＋ GAP-20261004-05：`gateProgressNewestRun` 现与正门 `gate results --last`
		// **同口径**（扫全部趟根 · 优选活趟，否则按 `results.tsv` mtime 最新挑 —— 不是按目录名倒序）。
		d, ok, err := gateProgressNewestRun()
		if err != nil {
			inv.setErr("blocked", "gates_unreadable", "临时目录读不到")
			fmt.Fprintf(stderr, "%s: 找「最近一趟」时读盘读不动 ⇒ 不给结论（退码 8）：%v\n", progName, err)
			return "", nil, exitBlocked
		}
		if !ok {
			return "", nil, exitOK // 交给 collect 判「判不出」
		}
		return d, nil, exitOK
	}
	gate, ok := gateReapScan()
	if !ok {
		return "", nil, exitOK
	}
	// ★ 缺口 `GAP-20261002-21`：多趟发现**不再只扫 `gateReapRoots`** —— 门内语境下（本命令/本测试
	//   进程的祖先进程就是那道真门 `precommit-gates.sh`）真在跑的门会被 `gateReapRoots` 沿 ppid
	//   剔成**非根**（它们的祖链上有**另一个**门），roots-only 收集会**漏掉**它们 ⇒ 退回
	//   `gateProgressNewestRun` 去猜最近趟（`gpIsolateEnv` 把 TMPDIR 指空、且假趟名非 `zerg-gates-*`）
	//   ⇒ 判不出退 8（规格要退 2）。改扫**全部**门进程的 `--outdir`（按目录去重）⇒ 有几趟活就报几趟。
	//   ★ 这不是放宽判据：「同时有多趟活」是**病症**，本改动把「静默回退去猜一趟」换成
	//     「**出声退 2 ＋ 逐个候选趟**」——是**更严**（fail-loud）；`gateReapScan` / `gateReapRoots`
	//     的认门口径一字未动（只换了「拿谁去收集 outdir」）。
	multi, seen := []string{}, map[string]bool{}
	for _, gp := range gate {
		if d := gateProgressOutdirFromCmd(gp.cmd); d != "" && !seen[d] {
			seen[d] = true
			multi = append(multi, d)
		}
	}
	sort.Strings(multi) // map 遍历无序 ⇒ 排序，让候选清单稳定（只影响打印次序）
	if h := gateProgressReadHolder(); h.outdir != "" && !seen[h.outdir] {
		multi = append(multi, h.outdir)
	}
	if len(multi) == 1 {
		return multi[0], multi, exitOK
	}
	return "", multi, exitOK
}

// gateProgressFieldLegal —— 字段是否在闭集表内（与仓库既有 `gateResultsFieldLegal` 同规）。
func gateProgressFieldLegal(f string, table []string) bool {
	for _, ok := range table {
		if f == ok {
			return true
		}
	}
	return false
}

// gateProgressIntFlag —— 取一枚整数旗标（缺省 / 坏值 ⇒ def）。

func gateProgressIntFlag(inv *invocation, name string, def int) int {
	v := strings.TrimSpace(inv.flagVal(name))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

// ── 辅助：历史趟清单（懒加载 · §十.9）────────────────────────────────────────────────────

func gateProgressList(inv *invocation, stdout, stderr io.Writer) int {
	limit := gateProgressIntFlag(inv, "--limit", gateProgressListLimit)
	dirs, err := gateProgressListDirs()
	if err != nil {
		inv.setErr("blocked", "gates_unreadable", "临时目录读不到")
		fmt.Fprintf(stderr, "%s: 读不到临时目录 ⇒ 不给结论（退码 8）：%v\n", progName, err)
		return exitBlocked
	}
	shown := dirs
	if limit >= 0 && len(shown) > limit {
		shown = shown[:limit]
	}
	fmt.Fprintf(stdout, "历史趟（最新在前 · 共 %d · 本页 %d）\n", len(dirs), len(shown))
	for _, d := range shown {
		line := "（无 results.tsv）"
		if b, e := os.ReadFile(filepath.Join(d, gateResultsTSV)); e == nil {
			s := strings.TrimSpace(string(b))
			if i := strings.IndexByte(s, '\n'); i >= 0 {
				s = strings.TrimSpace(s[:i])
			}
			line = s
		}
		fmt.Fprintf(stdout, "  %s  %s\n", d, line)
	}
	return exitOK
}

// ── 输出面：人读（§3.2）──────────────────────────────────────────────────────────────────

func gateProgressEmitHuman(w io.Writer, info *gateProgressInfo, code int) {
	if info == nil || !info.found {
		fmt.Fprintf(w, "趟：（判不出：读不到那一趟 / 空趟）\n")
		return
	}
	state := "**还在跑**"
	if !info.alive {
		state = "已收工"
	}
	fmt.Fprintf(w, "趟：%s（%s · 根 pid %d · 已 %s · 挑取依据：%s）\n",
		info.runDir, state, info.rootPid, gateProgressDur(info.elapsedSecs), dashIfEmpty(info.pickReason))
	originText := "判不了"
	switch info.originKind {
	case "machine":
		originText = "由 " + info.origin + " 发起（本机）"
	case "residue":
		originText = "**残留茬** —— 建议 `zerg gate reap --stale`"
	}
	fmt.Fprintf(w, "     归属：%s\n", originText)
	total := "（步表读数取不到）"
	if info.stepsTotalOK {
		total = strconv.Itoa(info.stepsTotal)
	}
	fmt.Fprintf(w, "进度：**%d / %s 步**（PASS %d · FAIL %d · BLOCKED %d · REPORT %d）\n",
		info.stepsDone, total, info.pass, info.fail, info.blocked, info.report)
	if len(info.slowest) > 0 {
		parts := make([]string, 0, len(info.slowest))
		for _, s := range info.slowest {
			parts = append(parts, fmt.Sprintf("%ds %s", s.secs, s.name))
		}
		fmt.Fprintf(w, "步耗时：最慢 %d 步 —— %s\n", len(info.slowest), strings.Join(parts, " · "))
	}
	if info.currentStep > 0 {
		cur := ""
		if info.currentOK {
			cur = fmt.Sprintf(" · 该步已 %ds", info.currentSecs)
		}
		fmt.Fprintf(w, "正在跑：第 %d 步 —— %s%s\n", info.currentStep, dashIfEmpty(info.currentName), cur)
	}
	sil := fmt.Sprintf("%d/%d", info.silentSecs, info.silentThreshold())
	if info.clockSkew {
		sil += "（时钟跳变）"
	}
	deep := "（进程树取不到）"
	if info.deepestOK {
		deep = fmt.Sprintf("%s(%d)", dashIfEmpty(info.deepestCmd), info.deepestPid)
	}
	fmt.Fprintf(w, "         已跑 %s · 最后活动距今 **%s 秒** · 最深：%s\n",
		gateProgressDur(info.currentSecs), sil, deep)
	// ★ GAP-20261002-16：两边口径**各说各的** —— 人读面必须**明写**是哪一档，
	//   不许用一句「advancing」把「单快照只判一层」与「wait 两帧三层」糊成同一件事。
	adv := "判不了（单次快照不做长采样 · 本档只判第 3 层＝当前步日志 mtime 推进 ＋ 谱系 pcpu>0 · §十.2）"
	if info.advKnown {
		switch info.advBasis {
		case "wait:3layer":
			if info.advancing {
				adv = "advancing=true（`gate wait` 两帧 · 完整三层取或：活动计数/PID 换过/日志 mtime · §十.1）⇒ **在干活，不是卡死**"
			} else {
				adv = "advancing=false（`gate wait` 两帧 · 三层都没动 ⇒ 疑似卡死 · 仍不下判定 · Q4）"
			}
		default:
			if info.advancing {
				adv = "advancing=true（本档只判第 3 层＝当前步日志 mtime 推进 ＋ 谱系 pcpu>0 · **不装三层** · §十.2）⇒ **在干活，不是卡死**"
			} else {
				adv = "advancing=false（本档只判第 3 层＋pcpu · §十.2）"
			}
		}
	}
	fmt.Fprintf(w, "         推进判据：%s\n", adv)
	if info.depthCapped {
		fmt.Fprintf(w, "         ⚠ 更深层未展开（深度上限 %d）\n", gateProgressDepthLimit)
	}
	fmt.Fprintf(w, "（退码 %d：0=读到（在跑 / 已收工且全绿）· 1=读到且**那一趟有红**（仅已收工才透 · §3.6）· 2=用法错 · 8=判不出）\n", code)
}

func gateProgressDur(secs int64) string {
	if secs < 0 {
		secs = 0
	}
	return fmt.Sprintf("%02d:%02d:%02d", secs/3600, (secs%3600)/60, secs%60)
}

// ── 输出面：JSONL（§十.5）───────────────────────────────────────────────────────────────

func gpJSONStr(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// gateProgressJSONHeader —— 首行版本对象（§十.5）。
func gateProgressJSONHeader(w io.Writer) {
	fmt.Fprintf(w, "{\"type\":\"schema\",\"schema\":%s,\"version\":%s}\n",
		gpJSONStr(gateProgressSchemaName), gpJSONStr(gateProgressVersion))
}

// gateProgressValues —— 八格 ⇒ 字段原始 JSON 值（未知值**不进 map** ⇒ 拼装处写 `<字段>_known:false`）。
func gateProgressValues(info *gateProgressInfo) map[string]string {
	v := map[string]string{}
	if info == nil || !info.found {
		return v
	}
	v["run_dir"] = gpJSONStr(info.runDir)
	v["finished"] = strconv.FormatBool(info.finished)
	v["root_pid"] = strconv.Itoa(info.rootPid)
	v["origin_kind"] = gpJSONStr(info.originKind)
	v["origin"] = gpJSONStr(info.origin)
	v["steps_done"] = strconv.Itoa(info.stepsDone)
	if info.stepsTotalOK {
		v["steps_total"] = strconv.Itoa(info.stepsTotal)
		v["steps_source_sha"] = gpJSONStr(info.stepsSrcSHA)
	}
	if info.currentStep > 0 {
		v["current_step"] = strconv.Itoa(info.currentStep)
		v["current_step_name"] = gpJSONStr(info.currentName)
	}
	if info.currentOK {
		v["current_secs"] = strconv.FormatInt(info.currentSecs, 10)
	}
	if info.elapsedOK {
		v["elapsed_secs"] = strconv.FormatInt(info.elapsedSecs, 10)
	}
	if info.found {
		v["slowest_steps"] = gateProgressSlowestJSON(info.slowest)
	}
	if info.lastOK {
		v["last_activity_ts"] = gpJSONStr(info.lastActivity.Format(time.RFC3339))
		v["silent_secs"] = strconv.FormatInt(info.silentSecs, 10)
	}
	v["silent_threshold"] = strconv.Itoa(info.silentThreshold())
	if info.countsOK {
		v["pass"] = strconv.Itoa(info.pass)
		v["fail"] = strconv.Itoa(info.fail)
		v["blocked"] = strconv.Itoa(info.blocked)
		v["report"] = strconv.Itoa(info.report)
		v["exit_code"] = strconv.Itoa(info.exitCode)
	}
	if info.aliveKnown {
		v["alive"] = strconv.FormatBool(info.alive)
	}
	if info.advKnown {
		v["advancing"] = strconv.FormatBool(info.advancing)
	}
	if info.deepestOK {
		v["deepest_pid"] = strconv.Itoa(info.deepestPid)
		v["deepest_cmd"] = gpJSONStr(info.deepestCmd)
		v["deepest_secs"] = strconv.FormatInt(info.deepestSecs, 10)
	}
	v["depth_limit_reached"] = strconv.FormatBool(info.depthCapped)
	return v
}

// gateProgressRecord —— 一条 `type` 记录：按字段序拼；未知值**省略键**并写 `<字段>_known:false`（§十.5）。
func gateProgressRecord(typ string, fields []string, vals map[string]string) string {
	var b strings.Builder
	b.WriteString("{\"type\":" + gpJSONStr(typ))
	for _, f := range fields {
		if raw, ok := vals[f]; ok {
			b.WriteString("," + gpJSONStr(f) + ":" + raw)
			continue
		}
		b.WriteString("," + gpJSONStr(f+"_known") + ":false")
	}
	b.WriteString("}")
	return b.String()
}

func gateProgressEmitJSON(inv *invocation, stdout io.Writer, info *gateProgressInfo, code int) int {
	fields := inv.fields
	if len(fields) == 0 {
		fields = gateProgressFields
	}
	gateProgressJSONHeader(stdout)
	fmt.Fprintln(stdout, gateProgressRecord("progress", fields, gateProgressValues(info)))
	return code
}

func gateProgressEmitJSONWait(inv *invocation, stdout io.Writer, info *gateProgressInfo, code, interval, timeout, waited int, timedOut bool) int {
	fields := inv.fields
	if len(fields) == 0 {
		fields = gateWaitFields
	}
	vals := gateProgressValues(info)
	vals["waited_secs"] = strconv.Itoa(waited)
	vals["timed_out"] = strconv.FormatBool(timedOut)
	vals["interval"] = strconv.Itoa(interval)
	vals["timeout"] = strconv.Itoa(timeout)
	gateProgressJSONHeader(stdout)
	fmt.Fprintln(stdout, gateProgressRecord("wait", fields, vals))
	return code
}
