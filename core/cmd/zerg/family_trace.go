// family_trace.go —— `zerg trace`（CA 全量追踪 · **归入命令面** · 批 B1）。
//
// 源件 = 独立入口 `core/cmd/zerg-trace/main.go`（设计：`docs/设计-CA跟踪程序.md` v3 · 2026-08-15）。
// 本件的分析函数是那一件的**逐面搬**：读 `events.jsonl` + `sysmetrics.jsonl`，出四种视图
// （时间线 / 某轮明细 / token 分析 / 错误清单）＋ JSON 面 ＋ 实时尾随。
//
// 形态（旗标**逐字**保留源件那五枚 · `<logdir>` 必填）：
//
//	zerg trace <logdir> [--live] [--json] [--step N] [--tokens] [--errors]
//
//	<logdir>    日志目录（含 `events.jsonl`；`sysmetrics.jsonl` 有则联动、没有就只出事件面）
//	--live      实时尾随（源件同义：跟着文件长，一直跑 —— 不给就一次性出全量视图）
//	--json      JSON 输出（源件同义：`{summary, steps, metrics}` 三段文档；
//	            ★ 给字段（`--json summary,steps`）时只出点名的那几段 —— 只增、不改裸形语义）
//	--step N    看第 N 轮明细（N 不是正整数 ⇒ 用法错 2）
//	--tokens    token 分析（逐轮 tokens / 时长 / 速度 / 累计）
//	--errors    错误清单（`level=error` / `tool_error` / `error` 非空的条目）
//
// 口径（与源件逐条一致，✗ 不改一处语义）：
//
//	· 视图优先级：`--live` > `--step N` > `--tokens` > `--errors` > 默认（`--json` 只决定默认档的形态）。
//	· 轮次：`Step == 0` 的条目用 `Seq/3` 近似（源件 v3 那条：有的会话没写 Step）；
//	  `loop_start` / `loop_end` 不参与近似。
//	· 空档告警：轮间间隔 > 30s 打 `⚠️gap`。
//	· 时间线最多打 61 轮（`i > 60` 处截断并提示 `--step N`）—— 与源件同一处截断。
//
// 退码（真源 = `exitcodes.go` 的表；本文件不写死数字）：
//
//	0  出了视图 / 出了 JSON
//	2  用法错（缺 `<logdir>` · `--step` 不是正整数 · `--json` 点了不存在的字段）
//	1  `events.jsonl` 打不开（源件同档：跑到了、没成功）
//
// ★ 双轨期：`core/cmd/zerg-trace` 二进制**一字不动**（本件只搬逻辑、不删入口）。
//
// 与源件的**照实差异**（三处 · 逐条写清，别处不猜）：
//
//	① **旗标位置无关**：源件用 Go `flag` ⇒ 旗标必须写在 `<logdir>` **前面**（写在后面会被当位置参数、
//	   `flag.Parse` 提前收工 ⇒ 静默不生效）。命令面按 CLI 惯例任意位置都认（`parseInvocation` 统一收）——
//	   这是**修正**源件「用法串说的与真行为不一致」，不是新语义。
//	② 缺 `<logdir>`：源件打用法 + 退 **1**；命令面统一走**用法错 2**（`exitcodes.go` 的 usage 档）。
//	③ `--json` 的**失败**路径（日志目录打不开）按 §九 M7 补 `error` 包封；源件无包封 —— 退码与
//	   人面判词不变（5 个视图的输出与源件**逐字节相同**，见 `family_trace_test.go` 与交付回执的两态对拍）。
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// traceFields —— `--json` 可点的字段（= JSON 文档的三段顶层键 · 与输出逐字同名）。
var traceFields = []string{"summary", "steps", "metrics"}

// traceEvent 与 logger.go 的 EventEntry 对应（读 events.jsonl）。
type traceEvent struct {
	Seq       int64     `json:"Seq"`
	TaskID    string    `json:"TaskID"`
	Type      string    `json:"Type"`
	Level     string    `json:"Level"`
	Action    string    `json:"Action"`
	ToolName  string    `json:"ToolName"`
	Prompt    string    `json:"Prompt"`
	Args      any       `json:"Args"`
	Result    string    `json:"Result"`
	Error     string    `json:"Error"`
	Step      int       `json:"Step"`
	Duration  string    `json:"Duration"`
	Timestamp time.Time `json:"Timestamp"`
}

// traceStepInfo 每轮汇总（★ 字段名/JSON 键与源件逐字同 —— 源件无 tag ⇒ 键 = 字段名）。
type traceStepInfo struct {
	Step        int
	Events      []traceEvent
	ModelCalls  []traceEvent
	ToolCalls   []traceEvent
	ModelTime   time.Duration
	ToolTime    time.Duration
	TotalTokens int
	StartTime   time.Time
	EndTime     time.Time
}

// traceSysMetric 机器级采样（sysmetrics.jsonl）。
type traceSysMetric struct {
	Timestamp  time.Time `json:"ts"`
	Step       int       `json:"step"`
	LocalCPU   float64   `json:"local_cpu,omitempty"`
	LocalMem   float64   `json:"local_mem,omitempty"`
	DiskRead   float64   `json:"disk_read_mbps,omitempty"`
	DiskWrite  float64   `json:"disk_write_mbps,omitempty"`
	X3GPU      float64   `json:"x3_gpu,omitempty"`
	X3CPU      float64   `json:"x3_cpu,omitempty"`
	X3Active   int       `json:"x3_active,omitempty"`
	X3MemAvail float64   `json:"x3_mem_avail_gb,omitempty"`
}

// traceSummary 汇总。
type traceSummary struct {
	TotalEvents int
	TotalSteps  int
	TotalTokens int
	ModelTime   time.Duration
	ToolTime    time.Duration
	StartTime   time.Time
	EndTime     time.Time
}

// cmdTrace —— `zerg trace <logdir> [--live] [--json] [--step N] [--tokens] [--errors]`。
func cmdTrace(inv *invocation, stdout, stderr io.Writer) int {
	// ★ `<logdir>` **必填**（源件同档：`flag.NArg() < 1` ⇒ 用法 + 退 1；命令面统一按用法错 2）。
	if len(inv.args) < 1 || strings.TrimSpace(inv.args[0]) == "" {
		inv.setErr("usage", "missing_logdir", "缺位置参数 <logdir>")
		fmt.Fprintf(stderr, "%s: 缺位置参数 <logdir> —— 本命令要一个**日志目录**（含 `events.jsonl`）\n", progName)
		fmt.Fprintf(stderr, "用法: zerg trace <logdir> [--live] [--json] [--step N] [--tokens] [--errors]\n")
		return exitUsage
	}
	logDir := inv.args[0]
	eventsFile := filepath.Join(logDir, "events.jsonl")

	// `--step N`：取值旗标 ⇒ 必须**取值**读（门73 面③/面⑤ 同一条口径）。
	stepN := 0
	if raw := strings.TrimSpace(inv.flagVal("--step")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 {
			inv.setErr("usage", "bad_step", "`--step` 不是正整数")
			fmt.Fprintf(stderr, "%s: `--step` 要一个**正整数**（现得 %q）—— 看第 N 轮明细\n", progName, raw)
			return exitUsage
		}
		stepN = n
	}

	if inv.hasFlag("--live") {
		traceTailLive(stdout, eventsFile)
		return exitOK
	}

	steps, sum := traceReadEvents(eventsFile, stderr)
	if steps == nil {
		inv.setErr("failed", "trace_open_failed", "events.jsonl 打不开")
		return exitFail
	}
	metrics := traceReadMetrics(filepath.Join(logDir, "sysmetrics.jsonl"))

	switch {
	case stepN > 0:
		tracePrintStepDetail(stdout, steps, stepN)
	case inv.hasFlag("--tokens"):
		tracePrintTokens(stdout, steps)
	case inv.hasFlag("--errors"):
		tracePrintErrors(stdout, steps)
	default:
		if inv.jsonGiven {
			return traceEmitJSON(inv, stdout, stderr, steps, sum, metrics)
		}
		tracePrintTimeline(stdout, steps, sum, metrics)
	}
	return exitOK
}

// traceEmitJSON —— `--json` 面。
//
// 不给字段（裸 `--json`）⇒ 源件的整份文档 `{summary, steps, metrics}`（**逐字节同源件**：
// 同一份 `map[string]any` + `MarshalIndent`）。给了字段 ⇒ 只出点名的那几段（按给定序拼，
// 键名 = 命令树 `fields:` 表里的名字）；点到不存在的字段 ⇒ 用法错 2 + 合法字段表。
func traceEmitJSON(inv *invocation, stdout, stderr io.Writer, steps []traceStepInfo,
	sum traceSummary, metrics []traceSysMetric) int {
	full := map[string]any{"summary": sum, "steps": steps, "metrics": metrics}
	if len(inv.fields) == 0 {
		b, _ := json.MarshalIndent(full, "", "  ")
		fmt.Fprintln(stdout, string(b))
		return exitOK
	}
	known := map[string]bool{}
	for _, f := range traceFields {
		known[f] = true
	}
	for _, f := range inv.fields {
		if !known[f] {
			// 与 `validateSelected` 逐字同档（`usage` / `json_field_unknown:<名>`）⇒ 用法类错误
			// **不吐包封**（`stdout` 零字节 · K2 甲档），判词与合法字段表全在 stderr。
			inv.setErr("usage", "json_field_unknown:"+f, fmt.Sprintf("未知字段 %q", f))
			return reportBadField(stderr, inv.path, f)
		}
	}
	var b strings.Builder
	b.WriteString("{")
	for i, f := range inv.fields {
		if i > 0 {
			b.WriteString(",")
		}
		raw, err := json.Marshal(full[f])
		if err != nil {
			inv.setErr("failed", "json_marshal_failed", err.Error())
			fmt.Fprintf(stderr, "%s: JSON 面组不出来（%v）\n", progName, err)
			return exitFail
		}
		fmt.Fprintf(&b, "%q:%s", f, raw)
	}
	b.WriteString("}")
	fmt.Fprintln(stdout, b.String())
	return exitOK
}

// traceReadEvents 读 events.jsonl —— 按 Step 分组。打不开 ⇒ (nil, 零值)（调用方判退码）。
func traceReadEvents(path string, stderr io.Writer) ([]traceStepInfo, traceSummary) {
	var steps []traceStepInfo
	stepMap := map[int]*traceStepInfo{}
	var sum traceSummary
	first := true

	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(stderr, "打开失败: %v\n", err)
		return nil, sum
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var ev traceEvent
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			continue
		}
		sum.TotalEvents++
		// v3: Step=0 时用 Seq 近似轮次（有的会话 Step 没写）
		if ev.Step == 0 && ev.Type != "loop_start" && ev.Type != "loop_end" {
			ev.Step = int(ev.Seq / 3)
		}
		if first {
			sum.StartTime = ev.Timestamp
			first = false
		}
		sum.EndTime = ev.Timestamp
		st, ok := stepMap[ev.Step]
		if !ok {
			st = &traceStepInfo{Step: ev.Step, StartTime: ev.Timestamp}
			stepMap[ev.Step] = st
		}
		st.Events = append(st.Events, ev)
		if ev.Timestamp.After(st.EndTime) {
			st.EndTime = ev.Timestamp
		}
		switch ev.Type {
		case "model_call":
			st.ModelCalls = append(st.ModelCalls, ev)
			if d, err := time.ParseDuration(ev.Duration); err == nil {
				st.ModelTime += d
				sum.ModelTime += d
			}
			if m, ok := ev.Args.(map[string]any); ok {
				if t, ok := m["tokens"].(float64); ok {
					st.TotalTokens += int(t)
					sum.TotalTokens += int(t)
				}
			}
		case "tool_call", "tool_result", "tool_error", "tool_denied":
			st.ToolCalls = append(st.ToolCalls, ev)
			if d, err := time.ParseDuration(ev.Duration); err == nil {
				st.ToolTime += d
				sum.ToolTime += d
			}
		}
	}
	for _, st := range stepMap {
		steps = append(steps, *st)
	}
	sort.Slice(steps, func(i, j int) bool { return steps[i].Step < steps[j].Step })
	sum.TotalSteps = len(steps)
	if steps == nil {
		steps = []traceStepInfo{} // 空件 ⇒ 空数组（✗ 不落 null：JSON 面形状不变）
	}
	return steps, sum
}

// traceReadMetrics 读 sysmetrics.jsonl（不在盘/读不通 ⇒ 空 —— 机器指标是联动面，不是判据面）。
func traceReadMetrics(path string) []traceSysMetric {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var out []traceSysMetric
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var m traceSysMetric
		if json.Unmarshal([]byte(line), &m) == nil {
			out = append(out, m)
		}
	}
	return out
}

// tracePrintTimeline 视图0+1+4：时间线 + 总览 + 机器指标联动。
func tracePrintTimeline(w io.Writer, steps []traceStepInfo, sum traceSummary, metrics []traceSysMetric) {
	fmt.Fprintln(w, "═══ CA full trace ═══")
	fmt.Fprintf(w, "task: %s → %s (total %s)\n",
		sum.StartTime.Format("15:04:05"), sum.EndTime.Format("15:04:05"),
		sum.EndTime.Sub(sum.StartTime).Round(time.Second))
	fmt.Fprintf(w, "events: %d | steps: %d | tokens: %d\n", sum.TotalEvents, sum.TotalSteps, sum.TotalTokens)
	if sum.ModelTime > 0 {
		fmt.Fprintf(w, "model time: %s | tool time: %s | avg token speed: %.1f tok/s\n",
			sum.ModelTime.Round(time.Millisecond), sum.ToolTime.Round(time.Millisecond),
			float64(sum.TotalTokens)/sum.ModelTime.Seconds())
	}
	// 空档检测（>30s 轮间间隔）
	fmt.Fprintln(w, "\n─── timeline (step + activity + machine metrics) ───")
	prevEnd := sum.StartTime
	for i, st := range steps {
		gap := st.StartTime.Sub(prevEnd)
		gapMark := ""
		if gap > 30*time.Second {
			gapMark = fmt.Sprintf(" ⚠️gap %s(30s+)", gap.Round(time.Second))
		}
		prevEnd = st.EndTime
		// 动作摘要
		var actions []string
		for _, mc := range st.ModelCalls {
			ts := 0.0
			cold := false
			if m, ok := mc.Args.(map[string]any); ok {
				if t, ok := m["tokens"].(float64); ok {
					ts = t
				}
				if c, ok := m["cold_start"].(bool); ok {
					cold = c
				}
			}
			speed := 0.0
			if d, err := time.ParseDuration(mc.Duration); err == nil && d.Seconds() > 0 {
				speed = ts / d.Seconds()
			}
			marker := ""
			if cold {
				marker = "❄冷启动 "
			}
			actions = append(actions, fmt.Sprintf("model[%s%s %.0ftok %.0ft/s]", marker, mc.Duration, ts, speed))
		}
		for _, tc := range st.ToolCalls {
			actions = append(actions, fmt.Sprintf("%s[%s]", tc.ToolName, tc.Type))
		}
		// 机器指标（该轮）
		metricStr := ""
		for _, m := range metrics {
			if m.Step == st.Step {
				metricStr = fmt.Sprintf(" CPU%.0f%% GPU%.0f%% disk%.1f/%.1fMB/s", m.LocalCPU, m.X3GPU, m.DiskRead, m.DiskWrite)
			}
		}
		// 时间点
		ts := st.StartTime.Format("15:04:05")
		line := fmt.Sprintf("step%d %s%s: %s", st.Step, ts, gapMark, strings.Join(actions, " "))
		if metricStr != "" {
			line += metricStr
		}
		fmt.Fprintln(w, line)
		if i > 60 {
			fmt.Fprintln(w, "...(more steps — use --step N for details)")
			break
		}
	}
}

// tracePrintStepDetail 视图2：某轮完整明细。
func tracePrintStepDetail(w io.Writer, steps []traceStepInfo, n int) {
	var st *traceStepInfo
	for i := range steps {
		if steps[i].Step == n {
			st = &steps[i]
			break
		}
	}
	if st == nil {
		fmt.Fprintf(w, "step %d does not exist\n", n)
		return
	}
	fmt.Fprintf(w, "═══ step %d detail (%s → %s) ═══\n", st.Step, st.StartTime.Format("15:04:05"), st.EndTime.Format("15:04:05"))
	for _, ev := range st.Events {
		fmt.Fprintf(w, "  [%s] %s %s", ev.Timestamp.Format("15:04:05.000"), ev.Type, ev.Action)
		if ev.ToolName != "" {
			fmt.Fprintf(w, " %s", ev.ToolName)
		}
		if ev.Duration != "" {
			fmt.Fprintf(w, " (%s)", ev.Duration)
		}
		fmt.Fprintln(w)
		if ev.Result != "" && len(ev.Result) > 0 {
			r := ev.Result
			if len(r) > 300 {
				r = r[:300] + "...（截断）"
			}
			fmt.Fprintf(w, "    result: %s\n", r)
		}
		if ev.Error != "" {
			fmt.Fprintf(w, "    error: %s\n", ev.Error)
		}
	}
}

// tracePrintTokens 视图3：token 分析。
func tracePrintTokens(w io.Writer, steps []traceStepInfo) {
	fmt.Fprintln(w, "═══ token analysis (per step) ═══")
	fmt.Fprintf(w, "%-6s %-10s %-10s %-12s %s\n", "step", "tokens", "time", "speed", "cumulative")
	var cum int
	for _, st := range steps {
		cum += st.TotalTokens
		speed := 0.0
		if st.ModelTime.Seconds() > 0 {
			speed = float64(st.TotalTokens) / st.ModelTime.Seconds()
		}
		fmt.Fprintf(w, "%-6d %-10d %-10s %-12.1f %d\n", st.Step, st.TotalTokens, st.ModelTime.Round(time.Millisecond), speed, cum)
	}
}

// tracePrintErrors 视图5：错误清单。
func tracePrintErrors(w io.Writer, steps []traceStepInfo) {
	fmt.Fprintln(w, "═══ error list ═══")
	count := 0
	for _, st := range steps {
		for _, ev := range st.Events {
			if ev.Level == "error" || ev.Type == "tool_error" || ev.Error != "" {
				count++
				fmt.Fprintf(w, "  step%d [%s] %s %s", st.Step, ev.Timestamp.Format("15:04:05"), ev.Type, ev.Action)
				if ev.ToolName != "" {
					fmt.Fprintf(w, " %s", ev.ToolName)
				}
				if ev.Error != "" {
					fmt.Fprintf(w, " error: %s", ev.Error)
				}
				fmt.Fprintln(w)
			}
		}
	}
	if count == 0 {
		fmt.Fprintln(w, "  no errors")
	}
}

// traceTailLive 实时跟踪（源件同义：一直跟、不返回 —— 文件还没出现就等它出现）。
func traceTailLive(w io.Writer, path string) {
	offset := int64(0)
	for {
		f, err := os.Open(path)
		if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}
		info, _ := f.Stat()
		if info.Size() < offset {
			offset = 0
		}
		if info.Size() > offset {
			f.Seek(offset, 0)
			sc := bufio.NewScanner(f)
			sc.Buffer(make([]byte, 1024*1024), 1024*1024)
			for sc.Scan() {
				line := strings.TrimSpace(sc.Text())
				if line == "" {
					continue
				}
				var ev traceEvent
				if json.Unmarshal([]byte(line), &ev) == nil {
					fmt.Fprintf(w, "[%s] step%d %s %s %s\n", ev.Timestamp.Format("15:04:05"), ev.Step, ev.Type, ev.Action, ev.ToolName)
				}
			}
			offset = info.Size()
		}
		f.Close()
		time.Sleep(1 * time.Second)
	}
}
