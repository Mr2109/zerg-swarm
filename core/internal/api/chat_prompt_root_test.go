package api

import (
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
)

// 待修补 #44：提示词里的工作目录必须由运行时解析（写死路径在公开/机群版会被脱敏成占位符）。
// 本机解析结果必须与改动前的字面文本逐字相同——否则提示词内容被悄悄改了。
func TestChatSystemPromptResolvedIsLiteralOnThisMachine(t *testing.T) {
	got := chatSystemPromptResolved()
	if strings.Contains(got, "{{REPO_ROOT}}") {
		t.Fatalf("占位符未被替换：%q", got)
	}
	root := statepath.WorkspaceRoot()
	if root == "" {
		t.Skip("本测试机解析不出工作目录（非常规环境）")
	}
	want := "工作目录是虫族项目根（" + root + "）"
	if !strings.Contains(got, want) {
		t.Fatalf("提示词里没有解析出的工作目录；期望包含 %q", want)
	}
	// 注意：断言的是**常量**里不得写死路径（替换后的正文在本机本来就含真实路径，那是应该的）。
	// 写死路径一旦回到常量，公开快照脱敏后机群就会指向不存在的目录（待修补 #44 的根因）。
	if strings.Contains(chatSystemPrompt, "<volume-path>") || strings.Contains(chatSystemPrompt, "~") {
		t.Fatalf("chatSystemPrompt 常量里又出现了写死的私有路径")
	}
	if !strings.Contains(chatSystemPrompt, "{{REPO_ROOT}}") {
		t.Fatalf("chatSystemPrompt 常量里缺少 {{REPO_ROOT}} 占位符")
	}
}
