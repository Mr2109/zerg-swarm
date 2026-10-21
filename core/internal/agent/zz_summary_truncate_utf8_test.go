package agent

// fixture_test.go — 滚动摘要四个截断点（300/150/150/60）的 UTF-8 边界判据。
//
// 被测点：context_window.go `summarizeRound`（assistant 300 · tool 150 · user 150）与
// `truncateArgs`（60）。PRE（仓内原样）红、POST（退字符起始）绿；两态断言同形。
// 每处单独一个子测试 —— 红了能直接定位到是哪一刀（不是「整段非法」这种笼统读数）。

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// straddleByteCW — 造一段内容，使第 cut 字节正落在字符中间；造不出就 Fatal。
func straddleByteCW(t *testing.T, unit string, cut, total int) string {
	t.Helper()
	for pad := 0; pad < 4; pad++ {
		var sb strings.Builder
		sb.WriteString(strings.Repeat("x", pad))
		for sb.Len() <= total {
			sb.WriteString(unit)
		}
		s := sb.String()
		if len(s) > cut && !utf8.RuneStart(s[cut]) {
			return s
		}
	}
	t.Fatalf("夹具有问题：造不出「第 %d 字节落字符中间」的内容（unit=%q）", cut, unit)
	return ""
}

func tailOfCW(s string) string {
	if len(s) <= 16 {
		return s
	}
	return s[len(s)-16:]
}

func TestSummarizeRoundUTF8Boundary(t *testing.T) {
	cases := []struct{ name, unit string }{
		{"中文", "测"},
		{"emoji", "🙂"},
		{"四字节生僻字", "𠀀"},
	}
	for _, c := range cases {
		// ① assistant 决策 300
		t.Run(c.name+"/assistant300", func(t *testing.T) {
			content := straddleByteCW(t, c.unit, 300, 400)
			got := summarizeRound([]Message{{Role: "assistant", Content: content}})
			if !utf8.ValidString(got) {
				t.Fatalf("assistant 300 切在字符中间（非法 UTF-8）：%q", tailOfCW(got))
			}
			if !strings.HasPrefix(got, "\n决策: ") {
				t.Fatalf("摘要形态变了：%q", got)
			}
			if n := len(strings.TrimPrefix(got, "\n决策: ")); n > 303 {
				t.Fatalf("assistant 段超预算：%d 字节（应 ≤300+…=303）", n)
			}
		})
		// ② tool 结果 150
		t.Run(c.name+"/tool150", func(t *testing.T) {
			content := straddleByteCW(t, c.unit, 150, 250)
			got := summarizeRound([]Message{{Role: "tool", Content: content}})
			if !utf8.ValidString(got) {
				t.Fatalf("tool 150 切在字符中间（非法 UTF-8）：%q", tailOfCW(got))
			}
			if !strings.HasPrefix(got, "\n工具结果: ") {
				t.Fatalf("摘要形态变了：%q", got)
			}
		})
		// ③ user 150
		t.Run(c.name+"/user150", func(t *testing.T) {
			content := straddleByteCW(t, c.unit, 150, 250)
			got := summarizeRound([]Message{{Role: "user", Content: content}})
			if !utf8.ValidString(got) {
				t.Fatalf("user 150 切在字符中间（非法 UTF-8）：%q", tailOfCW(got))
			}
			if !strings.HasPrefix(got, "\n用户: ") {
				t.Fatalf("摘要形态变了：%q", got)
			}
		})
		// ④ 工具参数 60
		t.Run(c.name+"/args60", func(t *testing.T) {
			val := straddleByteCW(t, c.unit, 60, 100)
			got := truncateArgs(map[string]any{"path": val})
			if !utf8.ValidString(got) {
				t.Fatalf("参数 60 切在字符中间（非法 UTF-8）：%q", tailOfCW(got))
			}
			if !strings.HasPrefix(got, "path=") {
				t.Fatalf("参数形态变了：%q", got)
			}
			if n := len(strings.TrimPrefix(got, "path=")); n > 63 {
				t.Fatalf("参数段超预算：%d 字节（应 ≤60+…=63）", n)
			}
		})
	}
}

// TestSummarizeRoundMixedRound — 一整轮（user + assistant + tool）拼起来的摘要也必须整体合法，
// 且四刀各在自己的位置 —— 这一格模拟真实压缩路径（compressOneRound → summarizeRound）。
func TestSummarizeRoundMixedRound(t *testing.T) {
	msgs := []Message{
		{Role: "user", Content: straddleByteCW(t, "测", 150, 250)},
		{Role: "assistant", Content: straddleByteCW(t, "测", 300, 400), ToolCalls: []ToolCall{
			{Name: "read", Args: map[string]any{"path": straddleByteCW(t, "测", 60, 100)}},
		}},
		{Role: "tool", Content: straddleByteCW(t, "测", 150, 250)},
	}
	got := summarizeRound(msgs)
	if !utf8.ValidString(got) {
		t.Fatalf("整轮摘要含非法 UTF-8（%d 字节）：%q", len(got), tailOfCW(got))
	}
	for _, want := range []string{"\n用户: ", "\n决策: ", "\n调用工具: read(", "\n工具结果: "} {
		if !strings.Contains(got, want) {
			t.Fatalf("摘要缺段 %q：%q", want, got)
		}
	}
}

// TestSummarizeRoundASCIIUnchanged — ASCII 逐字不变（切口永不退）。
func TestSummarizeRoundASCIIUnchanged(t *testing.T) {
	asst := strings.Repeat("a", 301)
	got := summarizeRound([]Message{{Role: "assistant", Content: asst}})
	want := "\n决策: " + asst[:300] + "…\n"
	if got != want {
		t.Fatalf("ASCII 摘要口径变了：\n实得 %q\n应得 %q", got, want)
	}
	tool := strings.Repeat("b", 151)
	got = summarizeRound([]Message{{Role: "tool", Content: tool}})
	if got != "\n工具结果: "+tool[:150]+"…\n" {
		t.Fatalf("ASCII 工具结果口径变了：%q", got)
	}
	user := strings.Repeat("c", 151)
	got = summarizeRound([]Message{{Role: "user", Content: user}})
	if got != "\n用户: "+user[:150]+"…\n" {
		t.Fatalf("ASCII 用户段口径变了：%q", got)
	}
	arg := strings.Repeat("d", 61)
	if got := truncateArgs(map[string]any{"k": arg}); got != "k="+arg[:60]+"…" {
		t.Fatalf("ASCII 参数口径变了：%q", got)
	}
}
