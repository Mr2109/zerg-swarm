package api

import (
	"os"
	"path/filepath"
	"testing"
)

func TestZergTaskFileInit(t *testing.T) {
	dir := t.TempDir()
	z := NewZergTaskFile(dir)
	if err := z.Init("task-test-1", "internal", "测试任务"); err != nil {
		t.Fatalf("Init 失败: %v", err)
	}
	// 已存在——重复 Init 不覆盖
	if err := z.Init("task-test-1", "internal", "重复"); err != nil {
		t.Fatalf("重复 Init 失败: %v", err)
	}
	lines, err := z.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll 失败: %v", err)
	}
	if len(lines) != 1 {
		t.Fatalf("应 1 行（不覆盖），实际 %d", len(lines))
	}
	if lines[0].Phase != "init" {
		t.Fatalf("首行 phase 应 init，实际 %s", lines[0].Phase)
	}
	if lines[0].Output["task_id"] != "task-test-1" {
		t.Fatalf("task_id 不对: %v", lines[0].Output["task_id"])
	}
}

func TestZergTaskFileAppendAndSummary(t *testing.T) {
	dir := t.TempDir()
	z := NewZergTaskFile(dir)
	_ = z.Init("task-test-2", "external", "目标")
	// 方案轮
	_ = z.Append(ZergTaskLine{
		Round: 1, Phase: "plan",
		Output: map[string]interface{}{"plans": []string{"A", "B", "C"}},
		Summary: "出了3个方案",
		Time:    "t1",
	})
	// 选定轮
	_ = z.Append(ZergTaskLine{
		Round: 2, Phase: "select",
		Output: map[string]interface{}{"selected": "A", "steps": []string{"s1", "s2"}},
		Summary: "选定方案A",
		Time:    "t2",
	})
	// 执行轮
	_ = z.Append(ZergTaskLine{
		Round: 3, Phase: "execute", Step: "s1",
		Output:  map[string]interface{}{"done": true},
		Summary: "完成s1",
		Time:    "t3",
	})

	last, err := z.Last()
	if err != nil || last == nil {
		t.Fatalf("Last 失败: %v", err)
	}
	if last.Round != 3 || last.Phase != "execute" || last.Step != "s1" {
		t.Fatalf("Last 不对: %+v", last)
	}

	sum, err := z.Summary(5)
	if err != nil {
		t.Fatalf("Summary 失败: %v", err)
	}
	if !contains(sum, "选定方案A") || !contains(sum, "完成s1") {
		t.Fatalf("Summary 缺内容: %s", sum)
	}
	// 文件存在
	if _, err := os.Stat(filepath.Join(dir, "task.jsonl")); err != nil {
		t.Fatalf("task.jsonl 不存在: %v", err)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
