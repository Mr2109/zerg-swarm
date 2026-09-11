// zerg-trace — CA 全量追踪（读 events.jsonl + sysmetrics.jsonl——时间线/每轮/token/错误/机器指标）
// 设计: docs/设计-CA跟踪程序.md（v3 定稿——2026-08-15）
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Event 与 logger.go 的 EventEntry 对应（读 events.jsonl）
type Event struct {
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

// StepInfo 每轮汇总
type StepInfo struct {
	Step        int
	Events      []Event
	ModelCalls  []Event
	ToolCalls   []Event
	ModelTime   time.Duration
	ToolTime    time.Duration
	TotalTokens int
	StartTime   time.Time
	EndTime     time.Time
}

// SysMetric 机器级采样（sysmetrics.jsonl）
type SysMetric struct {
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

// Summary 汇总
type Summary struct {
	TotalEvents int
	TotalSteps  int
	TotalTokens int
	ModelTime   time.Duration
	ToolTime    time.Duration
	StartTime   time.Time
	EndTime     time.Time
}

func main() {
	live := flag.Bool("live", false, "实时跟踪")
	jsonOut := flag.Bool("json", false, "JSON 输出")
	step := flag.Int("step", 0, "查看第 N 轮明细")
	tokens := flag.Bool("tokens", false, "token 分析")
	errors := flag.Bool("errors", false, "错误清单")
	flag.Parse()
	if flag.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "用法: zerg-trace <logdir> [--live] [--json] [--step N] [--tokens] [--errors]")
		os.Exit(1)
	}
	logDir := flag.Arg(0)
	eventsFile := filepath.Join(logDir, "events.jsonl")

	if *live {
		tailLive(eventsFile, jsonOut)
		return
	}

	steps, total := readEvents(eventsFile)
	metrics := readMetrics(filepath.Join(logDir, "sysmetrics.jsonl"))

	switch {
	case *step > 0:
		printStepDetail(steps, *step)
	case *tokens:
		printTokens(steps)
	case *errors:
		printErrors(steps)
	default:
		if *jsonOut {
			printJSON(steps, total, metrics)
		} else {
			printTimeline(steps, total, metrics)
		}
	}
}

// readEvents 读 events.jsonl——按 Step 分组
func readEvents(path string) ([]StepInfo, Summary) {
	var steps []StepInfo
	stepMap := map[int]*StepInfo{}
	var sum Summary
	first := true

	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "打开失败: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var ev Event
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
			st = &StepInfo{Step: ev.Step, StartTime: ev.Timestamp}
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
	return steps, sum
}

// readMetrics 读 sysmetrics.jsonl
func readMetrics(path string) []SysMetric {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var out []SysMetric
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var m SysMetric
		if json.Unmarshal([]byte(line), &m) == nil {
			out = append(out, m)
		}
	}
	return out
}

// printTimeline 视图0+1+4：时间线 + 总览 + 机器指标联动
func printTimeline(steps []StepInfo, sum Summary, metrics []SysMetric) {
	fmt.Println("═══ CA full trace ═══")
	fmt.Printf("task: %s → %s (total %s)\n",
		sum.StartTime.Format("15:04:05"), sum.EndTime.Format("15:04:05"),
		sum.EndTime.Sub(sum.StartTime).Round(time.Second))
	fmt.Printf("events: %d | steps: %d | tokens: %d\n", sum.TotalEvents, sum.TotalSteps, sum.TotalTokens)
	if sum.ModelTime > 0 {
		fmt.Printf("model time: %s | tool time: %s | avg token speed: %.1f tok/s\n",
			sum.ModelTime.Round(time.Millisecond), sum.ToolTime.Round(time.Millisecond),
			float64(sum.TotalTokens)/sum.ModelTime.Seconds())
	}
	// 空档检测（>30s 轮间间隔）
	fmt.Println("\n─── timeline (step + activity + machine metrics) ───")
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
		fmt.Println(line)
		if i > 60 {
			fmt.Println("...(more steps — use --step N for details)")
			break
		}
	}
}

// printStepDetail 视图2：某轮完整明细
func printStepDetail(steps []StepInfo, n int) {
	var st *StepInfo
	for i := range steps {
		if steps[i].Step == n {
			st = &steps[i]
			break
		}
	}
	if st == nil {
		fmt.Printf("step %d does not exist\n", n)
		return
	}
	fmt.Printf("═══ step %d detail (%s → %s) ═══\n", st.Step, st.StartTime.Format("15:04:05"), st.EndTime.Format("15:04:05"))
	for _, ev := range st.Events {
		fmt.Printf("  [%s] %s %s", ev.Timestamp.Format("15:04:05.000"), ev.Type, ev.Action)
		if ev.ToolName != "" {
			fmt.Printf(" %s", ev.ToolName)
		}
		if ev.Duration != "" {
			fmt.Printf(" (%s)", ev.Duration)
		}
		fmt.Println()
		if ev.Result != "" && len(ev.Result) > 0 {
			r := ev.Result
			if len(r) > 300 {
				r = r[:300] + "...（截断）"
			}
			fmt.Printf("    result: %s\n", r)
		}
		if ev.Error != "" {
			fmt.Printf("    error: %s\n", ev.Error)
		}
	}
}

// printTokens 视图3：token 分析
func printTokens(steps []StepInfo) {
	fmt.Println("═══ token analysis (per step) ═══")
	fmt.Printf("%-6s %-10s %-10s %-12s %s\n", "step", "tokens", "time", "speed", "cumulative")
	var cum int
	for _, st := range steps {
		cum += st.TotalTokens
		speed := 0.0
		if st.ModelTime.Seconds() > 0 {
			speed = float64(st.TotalTokens) / st.ModelTime.Seconds()
		}
		fmt.Printf("%-6d %-10d %-10s %-12.1f %d\n", st.Step, st.TotalTokens, st.ModelTime.Round(time.Millisecond), speed, cum)
	}
}

// printErrors 视图5：错误清单
func printErrors(steps []StepInfo) {
	fmt.Println("═══ error list ═══")
	count := 0
	for _, st := range steps {
		for _, ev := range st.Events {
			if ev.Level == "error" || ev.Type == "tool_error" || ev.Error != "" {
				count++
				fmt.Printf("  step%d [%s] %s %s", st.Step, ev.Timestamp.Format("15:04:05"), ev.Type, ev.Action)
				if ev.ToolName != "" {
					fmt.Printf(" %s", ev.ToolName)
				}
				if ev.Error != "" {
					fmt.Printf(" error: %s", ev.Error)
				}
				fmt.Println()
			}
		}
	}
	if count == 0 {
		fmt.Println("  no errors")
	}
}

// printJSON JSON 输出
func printJSON(steps []StepInfo, sum Summary, metrics []SysMetric) {
	out := map[string]any{
		"summary": sum,
		"steps":   steps,
		"metrics": metrics,
	}
	b, _ := json.MarshalIndent(out, "", "  ")
	fmt.Println(string(b))
}

// tailLive 实时跟踪
func tailLive(path string, jsonOut *bool) {
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
				var ev Event
				if json.Unmarshal([]byte(line), &ev) == nil {
					fmt.Printf("[%s] step%d %s %s %s\n", ev.Timestamp.Format("15:04:05"), ev.Step, ev.Type, ev.Action, ev.ToolName)
				}
			}
			offset = info.Size()
		}
		f.Close()
		time.Sleep(1 * time.Second)
	}
}
