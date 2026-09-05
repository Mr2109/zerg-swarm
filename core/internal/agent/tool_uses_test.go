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
	if all["read"] != 5 {
		t.Fatalf("read 应 5(3+2——多进程累加): %d", all["read"])
	}
	if all["bash"] != 2 || all["glob"] != 4 {
		t.Fatalf("bash/glob 计数错: %+v", all)
	}
}
