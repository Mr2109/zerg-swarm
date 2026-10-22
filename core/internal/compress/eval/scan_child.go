package eval

// 每档一个**子进程**跑，带硬超时 —— 这是**保险丝**，不是洁癖：
//
//	P2 早期 free 引擎在第 ② 步（余量闸）上存在不终止路径：窗口里还有不可卸载的质量
//	（pinned / 对白段）时，`total` 永远降不到 `Budget×HeadroomPct` 以下，而循环体里
//	没有 ctx 检查、`unload()` 又对同一段反复返回 true ⇒ 闷死在一档里。
//	该 bug 已在 compress/ 修掉（ctx 检查 + unload 幂等 + 进展检测），本批四档全跑完。
//
// 保险丝留着：尺子不能因为引擎再次退化就「跑不出数」或「把跑不完当读数」。撞超时就杀掉，
// 本档标 Status=timeout、不出任何指标（不记 0、不推算）；报告里有复现命令。

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	childFlagEnv   = "EVAL_SCAN_CHILD"
	childIDsEnv    = "EVAL_SCAN_IDS"
	childBudgetEnv = "EVAL_SCAN_BUDGET"
	childEngineEnv = "EVAL_SCAN_ENGINE"
	childWorkEnv   = "EVAL_SCAN_WORK"
	childOutEnv    = "EVAL_SCAN_OUT"
)

// SelectAll —— 按 id 从两类样本里取（子进程只收到 id 列表，避免样本构造漂移）。
func SelectAll(ids []string) []Sample {
	if len(ids) == 0 {
		return AllSamples()
	}
	want := map[string]bool{}
	for _, id := range ids {
		want[strings.TrimSpace(id)] = true
	}
	var out []Sample
	for _, s := range AllSamples() {
		if want[s.ID] {
			out = append(out, s)
		}
	}
	return out
}

// RunScanTierChild —— 子进程入口：跑一档、把 Report 写进 childOutEnv。
// 由 scan_test.go 里的 TestScanTierChild 调用（测试二进制才有的入口）。
func RunScanTierChild() error {
	budget, err := strconv.Atoi(os.Getenv(childBudgetEnv))
	if err != nil {
		return fmt.Errorf("budget: %w", err)
	}
	ids := strings.Split(os.Getenv(childIDsEnv), ",")
	samples := SelectAll(ids)
	if len(samples) != len(ids) {
		return fmt.Errorf("样本 id 对不上：要 %d 条，取到 %d 条", len(ids), len(samples))
	}
	engine := os.Getenv(childEngineEnv)
	if engine == "" {
		engine = "free"
	}
	rep, err := RunBatch(context.Background(), samples, Config{
		Budget: budget, Engine: engine, WorkDir: os.Getenv(childWorkEnv),
	})
	if err != nil {
		return err
	}
	b, err := json.Marshal(rep)
	if err != nil {
		return err
	}
	return os.WriteFile(os.Getenv(childOutEnv), b, 0o644)
}

// tierRun —— 一档的执行结果（读数 / 状态 / 说明）。
type tierRun struct {
	rep    *Report
	status string // ok | timeout | error
	note   string
}

// runTierIsolated —— 起子进程跑一档，硬超时后杀掉并标 timeout。
func runTierIsolated(samples []Sample, engine string, workDir string, budget int, timeout time.Duration) tierRun {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return tierRun{status: "error", note: "建 work 目录失败：" + err.Error()}
	}
	outPath := filepath.Join(workDir, "tier-report.json")
	_ = os.Remove(outPath)

	ids := make([]string, 0, len(samples))
	for _, s := range samples {
		ids = append(ids, s.ID)
	}
	exe, err := os.Executable()
	if err != nil {
		return tierRun{status: "error", note: "取自身可执行路径失败：" + err.Error()}
	}
	cmd := exec.Command(exe, "-test.run=^TestScanTierChild$", "-test.timeout=300s")
	cmd.Env = append(os.Environ(),
		childFlagEnv+"=1",
		childIDsEnv+"="+strings.Join(ids, ","),
		childBudgetEnv+"="+strconv.Itoa(budget),
		childEngineEnv+"="+engine,
		childWorkEnv+"="+workDir,
		childOutEnv+"="+outPath,
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = &stderr
	if err := cmd.Start(); err != nil {
		return tierRun{status: "error", note: "起子进程失败：" + err.Error()}
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case <-time.After(timeout):
		_ = cmd.Process.Kill()
		<-done
		return tierRun{status: "timeout", note: fmt.Sprintf(
			"子进程 %.0fs 未结束（free 的余量闸在「残留 > headroom 且候选段已全部卸载」时不终止），本档没有读数",
			timeout.Seconds())}
	case err := <-done:
		if err != nil {
			return tierRun{status: "error", note: "子进程失败：" + strings.TrimSpace(tail(stderr.String(), 400))}
		}
	}

	raw, err := os.ReadFile(outPath)
	if err != nil {
		return tierRun{status: "error", note: "子进程没写出报告：" + err.Error()}
	}
	var rep Report
	if err := json.Unmarshal(raw, &rep); err != nil {
		return tierRun{status: "error", note: "子进程报告解不开：" + err.Error()}
	}
	return tierRun{rep: &rep, status: "ok"}
}

func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}
