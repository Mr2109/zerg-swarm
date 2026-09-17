package gateway

import (
	"strings"
	"testing"
	"unicode/utf8"
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

// 事故回归（2026-09-17 23:45–00:03 实测的 5 次 other_error 真身）：
// 字符截断窗口 = 前 2400 / 后 1600 字节，而旧守卫只查 len > toolOutputMaxLen(800) ⇒
// 801…3999 字节的**单行**工具输出一律越界 panic
// （日志原文 `http: panic serving … [:0] with length 1210`）；net/http 在 handler panic 后只记日志、
// 不写响应、直接关连接 ⇒ 客户端只看到 `Post …: EOF`，对话层分类落到 other_error 兜底。
// 本用例必须**带住这个带**（把守卫退回 `len > 800` 就在这里 panic = 红）。
func TestMaskToolOutputCharWindowNeverPanics(t *testing.T) {
	// 边界四档 + 日志里真实出现过的长度（1210/1297/1700/1733/1768）
	for _, n := range []int{800, 801, 1210, 1297, 1600, 1700, 1733, 1768, 2399, 2400, 2401, 3999, 4000, 4001, 9000} {
		in := strings.Repeat("x", n)
		got := maskToolOutput(in) // 旧实现：801…3999 在这里 panic
		if len(got) < len(in) && !strings.Contains(got, "省略") {
			t.Errorf("len=%d：短了 %d 字节却没有省略标注（静默删内容）", n, len(in)-len(got))
		}
		if n <= 4000 && got != in {
			t.Errorf("len=%d：首尾窗口（2400+1600）装得下 ⇒ 必须原样返回，实际 %d 字节", n, len(got))
		}
		if n > 4000 && got == in {
			t.Errorf("len=%d：超过窗口总量 ⇒ 应裁剪", n)
		}
	}
	// 省略数必须是**真删掉的字节数**（旧写法写的是 len-maxLen=8200，与实际删掉的 5000 不符）
	if got := maskToolOutput(strings.Repeat("x", 9000)); !strings.Contains(got, "省略 5000 字符") {
		t.Errorf("省略标注的数不对（应 5000）：%q", got[:60])
	}
}

// 中文单行输出：按字节切会把一个汉字切成半个 ⇒ 提示词里出现非法 UTF-8。
// 打坏 runeStartBackward/runeStartForward（去掉边界修正）本用例必红。
func TestMaskToolOutputCutsOnRuneBoundary(t *testing.T) {
	in := "ab" + strings.Repeat("汉", 1500) // 4502 字节，2400/2902 两个切口都落在汉字中间
	got := maskToolOutput(in)
	if !utf8.ValidString(got) {
		t.Fatalf("切口切碎了多字节字符 ⇒ 非法 UTF-8（尾部 %q）", got[len(got)-12:])
	}
	if !strings.Contains(got, "省略") {
		t.Fatalf("超过窗口总量未裁剪")
	}
	if !strings.HasPrefix(got, "ab汉") || !strings.HasSuffix(got, "汉") {
		t.Fatalf("裁剪后首尾内容丢失：head=%q tail=%q", got[:9], got[len(got)-6:])
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
