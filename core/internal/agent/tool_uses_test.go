package agent

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// TestToolUsesMultiProcess — 2026-09-06: 多进程并发计数不互覆(Mr2109发现丢失)
// 模拟: 多个独立进程各记不同工具——写盘后都在(flock 读-改-写)
func TestToolUsesMultiProcess(t *testing.T) {
	orig := toolUsesFile
	toolUsesFile = filepath.Join(t.TempDir(), "tool-uses-test.json")
	defer func() { toolUsesFile = orig }()
	_ = os.WriteFile(toolUsesFile, []byte("{}"), 0o644)
	// 2026-09-13（CI 偶发丢计数排查）：本包其他测试/后台 goroutine 也会经 exec.go 调
	// RecordToolUse（写的是**同一个包级变量**指向的文件），因此断言必须看**增量**而不是绝对值，
	// 否则会被外部写入污染成"偶发"（CI 上曾出现 bash/glob 少 1）。基线 + 增量 = 既免疫干扰、仍证明累加。
	base := ToolUsesAll()

	runProc := func(tool string, n int, wg *sync.WaitGroup) {
		defer wg.Done()
		for i := 0; i < n; i++ {
			RecordToolUse(tool)
		}
	}
	var wg sync.WaitGroup
	wg.Add(4)
	go runProc("read", 3, &wg)
	go runProc("bash", 2, &wg)
	go runProc("read", 2, &wg)
	go runProc("glob", 4, &wg)
	wg.Wait()

	all := ToolUsesAll()
	if d := all["read"] - base["read"]; d < 5 {
		t.Fatalf("read 增量应 ≥5(3+2——跨 goroutine 累加): 实际增量 %d（基线 %d → 现 %d）", d, base["read"], all["read"])
	}
	if d := all["bash"] - base["bash"]; d < 2 {
		t.Fatalf("bash 增量应 ≥2: 实际增量 %d（基线 %d → 现 %d）", d, base["bash"], all["bash"])
	}
	if d := all["glob"] - base["glob"]; d < 4 {
		t.Fatalf("glob 增量应 ≥4: 实际增量 %d（基线 %d → 现 %d）", d, base["glob"], all["glob"])
	}
}

// TestToolEventsAppend — 2026-09-06: 事件流写入/按日读回/坏行容错
func TestToolEventsAppend(t *testing.T) {
	// 临时目录替换全局
	orig := toolEventsDir
	toolEventsDir = t.TempDir()
	defer func() { toolEventsDir = orig }()

	day := todayStr()
	AppendToolEvent("bash", 120)
	AppendToolEvent("read", 30)
	AppendToolEvent("bash", 80)

	evs := ToolEventsToday()
	if len(evs) != 3 {
		t.Fatalf("应 3 条: %d", len(evs))
	}
	if evs[0].Tool != "bash" || evs[0].DurMs != 120 {
		t.Fatalf("首条错: %+v", evs[0])
	}
	if evs[2].Tool != "bash" {
		t.Fatalf("末条错: %+v", evs[2])
	}
	// 按日读取
	evs2 := ToolEventsOfDay(day)
	if len(evs2) != 3 {
		t.Fatalf("按日应 3 条: %d", len(evs2))
	}
	// 坏行容错: 追加垃圾行后读
	f, _ := os.OpenFile(filepath.Join(toolEventsDir, "zerg-tool-events-"+day+".jsonl"), os.O_APPEND|os.O_WRONLY, 0o644)
	_, _ = f.WriteString("not-json\n")
	f.Close()
	evs3 := ToolEventsToday()
	if len(evs3) != 3 {
		t.Fatalf("坏行应跳过: %d", len(evs3))
	}
}
