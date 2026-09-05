package api

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mockModel 模拟模型（按阶段返回不同 JSON——方案/选定/报告）
func mockModel(systemPrompt, userPrompt string, maxTokens int) (string, error) {
	if strings.Contains(systemPrompt, "请判断这个任务属于什么类型") {
		return "代码任务", nil
	}
	if strings.Contains(systemPrompt, "请以 JSON 格式输出") {
		// 方案轮（含 plans）或选定轮（含 selected）——区分
		if strings.Contains(systemPrompt, "refined_steps") {
			return `{"selected":"A","reason":"简单","refined_steps":["写文件","验证"]}`, nil
		}
		return `{"plans":[{"id":"A","title":"直接创建","steps":["写文件","验证"],"approach":"echo"}]}`, nil
	}
	if strings.Contains(systemPrompt, "封闭报告生成者") {
		// v2.5.6 2026-08-29: 报告空壳拦截——mock 必须返回实质内容（>100 字）
		return "# 结论报告\n\n本任务创建了 hello.txt 文件并完成流程验证。\n\n## 做了什么\n- 创建了测试文件\n- 完成全流程验证\n\n## 验证结果\n- 文件已写入成功\n- 全流程通过\n\n## 经验教训\n- 测试 mock 需覆盖封闭报告场景\n- 空壳报告会被拦截", nil
	}
	// 执行轮——写文件（模拟模型调工具——写到当前工作目录——验证用 git status 检查）
	// 从 systemPrompt 提取任务目录（【当前小任务】前面没有——用固定路径）
	// 简化: 写到 /tmp/zerg-tasks/ 下的当前测试任务目录（扫出最新任务目录）
	// 更稳: 写多个可能路径（task-flow-test-* 的目录——保证验证能找到）
	content := []byte("hello-" + userPrompt)
	if strings.Contains(userPrompt, "验证") {
		content = []byte("verified-" + userPrompt)
	}
	// v2.5.6 封闭报告 mock（2026-08-29——报告空壳拦截后测试需实质报告）
	// 封闭轮 prompt 含"报告"——返回实质内容（>100 字——过报告验证）
	if strings.Contains(userPrompt, "报告") {
		content = []byte("## 任务报告\n\n本任务创建了 hello.txt 文件并完成验证。\n\n### 做了什么\n- 创建了测试文件\n- 完成流程验证\n\n### 验证结果\n- 文件已写入\n- 全流程通过\n\n### 结论\n任务正常完成，产出真实有效。")
	}
	// 扫描 /tmp/zerg-tasks/task-flow-test-* 目录——写到每个（保证执行器验证能找到）
	if entries, err := os.ReadDir("/tmp/zerg-tasks"); err == nil {
		for _, en := range entries {
			if strings.HasPrefix(en.Name(), "task-flow-test-") && en.IsDir() {
				_ = os.WriteFile(filepath.Join("/tmp/zerg-tasks", en.Name(), "hello.txt"), content, 0o644)
			}
		}
	}
	return "完成: " + userPrompt, nil
}

func TestZergFlowExecutorRun(t *testing.T) {
	// 清理旧的 task-flow-test-* 目录（避免 git 残留累积——commit 数错乱）
	if entries, err := os.ReadDir("/tmp/zerg-tasks"); err == nil {
		for _, en := range entries {
			if strings.HasPrefix(en.Name(), "task-flow-test-") {
				_ = os.RemoveAll(filepath.Join("/tmp/zerg-tasks", en.Name()))
			}
		}
	}
	// 准备执行目录（模型写文件用）
	_ = os.MkdirAll("/tmp/zerg-test-flow", 0o755)
	_ = os.RemoveAll("/tmp/zerg-test-flow/hello.txt")

	task := &Task{
		ID: "task-flow-test-1",
		// 用唯一目录避免并发测试冲突
		Description: "创建 hello.txt 文件",
		Type:        "internal",
		Model:       "mock",
		Flow:        "zerg",
	}
	e, err := NewZergFlowExecutor(task, mockModel, "/tmp/zerg-test-flow/skills")
	if err != nil {
		t.Fatalf("执行器创建失败: %v", err)
	}
	if err := e.Run(); err != nil {
		t.Fatalf("全流程失败: %v", err)
	}
	// 验证: git 树完整（多轮 commit）
	if e.Git.CommitCount() < 4 {
		t.Fatalf("git 树应 >=4 commit（init+方案+选定+执行+封闭）: %d", e.Git.CommitCount())
	}
	// task.jsonl 完整（多轮总结）
	lines, _ := e.TaskFile.ReadAll()
	if len(lines) < 4 {
		t.Fatalf("task.jsonl 应 >=4 行: %d", len(lines))
	}
	sum, _ := e.TaskFile.Summary(10)
	for _, want := range []string{"出了1个方案", "选定方案A", "hello.txt"} {
		if !strings.Contains(sum, want) {
			t.Fatalf("task.jsonl 缺总结 %s: %s", want, sum)
		}
	}
	// 报告存在
	if _, err := os.Stat(filepath.Join(e.TaskDir, "reports", "final.md")); err != nil {
		t.Fatalf("报告应存在: %v", err)
	}
	// 任务状态 done
	if task.Status != "done" {
		t.Fatalf("任务应 done: %s", task.Status)
	}
	_ = fmt.Sprint() // 保持 fmt 导入
}
