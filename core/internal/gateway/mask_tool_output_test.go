package gateway

import (
	"strings"
	"testing"
)

// 事故背景（2026-09-17 实测）：maskToolOutput 的阈值原为「前 5 行 + 后 3 行」⇒ 任何超过 9 行的工具输出
// 都被砍成首尾各几行，模型据此认为"文件被反复截断"⇒ 反复重读同一文件 ⇒ 轮数被空转吃光。
func TestMaskToolOutputSmallUntouched(t *testing.T) {
	// 75 行的设计稿量级 ⇒ 必须**原样**返回（不裁、不加省略标记）
	var b strings.Builder
	for i := 1; i <= 75; i++ {
		b.WriteString("line ")
		b.WriteString(itoa(i))
		b.WriteString("\n")
	}
	in := b.String()
	if got := maskToolOutput(in); got != in {
		t.Errorf("中小输出被改动（%d 字节 ⇒ %d 字节）\n开头：%q", len(in), len(got), got[:min(120, len(got))])
	}
}

func TestMaskToolOutputLargeAnnotatedActionable(t *testing.T) {
	var b strings.Builder
	for i := 1; i <= 400; i++ {
		b.WriteString("line ")
		b.WriteString(itoa(i))
		b.WriteString("\n")
	}
	got := maskToolOutput(b.String())
	if got == b.String() {
		t.Fatal("超大输出未被裁剪（上下文浪费）")
	}
	if !strings.Contains(got, "省略") {
		t.Errorf("裁剪未标注：%q", got[:min(200, len(got))])
	}
	if !strings.Contains(got, "offset") {
		t.Errorf("裁剪标注不可行动（未提示分段重读）：%q", got[:min(200, len(got))])
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
