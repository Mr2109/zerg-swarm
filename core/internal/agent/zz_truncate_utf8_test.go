package agent

// zz_truncate_utf8_test.go —— truncate 按字节截断必须退到字符边界（差分判据，出树存放）。
//
// 三档非 ASCII 必测：中文（3 字节）、emoji（4 字节，可能被切在高位/续字节处）、
// 4 字节生僻字（U+20000 一类，SMP 外）。判据：
//   utf8.ValidString(前缀) == true  且  len(前缀) <= limit（字节口径不变，不放大预算）
// 另加口径文案与越界/负值不 panic 两条守卫。

import (
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
)

const truncSuffixMark = "\n...(output truncated"

// truncPrefixOf 剥掉提示尾，返回被截出来的前缀（无提示尾 ⇒ 直接 Fatal：说明没截）。
func truncPrefixOf(t *testing.T, got string) string {
	t.Helper()
	p, _, ok := strings.Cut(got, truncSuffixMark)
	if !ok {
		t.Fatalf("输出里没有截断提示尾：%q", got)
	}
	return p
}

// TestTruncateUTF8Boundary — 三类非 ASCII：合法 UTF-8 + 前缀不超过 limit 字节。
func TestTruncateUTF8Boundary(t *testing.T) {
	cases := []struct {
		name  string
		src   string
		limit int
	}{
		// 3 字节汉字：limit=20 落在第 7 枚「字」（21 字节）中间 ⇒ 旧实现出半枚。
		{"中文", strings.Repeat("这是一个很长的工具输出", 20), 20},
		{"中文_limit落字符首", strings.Repeat("虫族", 30), 18}, // 6 枚×3B ⇒ 18 正好是边界
		// 4 字节 emoji：7 = 4 + 3，第二枚被切在续字节上。
		{"emoji", strings.Repeat("🙂🚀", 20), 7},
		{"emoji_limit落字符中", strings.Repeat("🐛", 30), 10}, // 8 是边界，10 落在第三枚中间
		// 4 字节生僻字（U+20000 𠀀，CJK 扩展 B 区）。
		{"四字节生僻字", strings.Repeat("𠀀𠁀𠂀", 20), 10},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := truncate(c.src, c.limit)
			pre := truncPrefixOf(t, got)
			if !utf8.ValidString(pre) {
				t.Errorf("截断切在字符中间（非法 UTF-8）：%q（limit=%d）", pre, c.limit)
			}
			if !utf8.ValidString(got) {
				t.Errorf("整串含非法 UTF-8：%q", got)
			}
			if len(pre) > c.limit {
				t.Errorf("前缀 %d 字节 > limit %d（预算被放大）", len(pre), c.limit)
			}
			// 退边界只允许丢 < 4 字节（最大 rune 宽度），否则是「退太多」。
			if c.limit-len(pre) > 3 {
				t.Errorf("前缀 %d 字节，比 limit %d 少了 %d 字节（退过头）", len(pre), c.limit, c.limit-len(pre))
			}
			if !strings.HasSuffix(got, strconv.Itoa(len(c.src))+" bytes total)") {
				t.Errorf("提示尾应为字节口径 %d bytes total：%q", len(c.src), got)
			}
			if !strings.HasPrefix(c.src, pre) {
				t.Errorf("前缀不是源串的真前缀：%q", pre)
			}
		})
	}
}

// TestTruncateLimitClamp — limit 越界/负值不得 panic（compactor.go 记过的事故口径）。
func TestTruncateLimitClamp(t *testing.T) {
	safe := func(s string, limit int) (out string, panicked any) {
		defer func() { panicked = recover() }()
		return truncate(s, limit), nil
	}
	// limit > len(s)：原样返回，不截。
	if out, p := safe("短", 100); p != nil || out != "短" {
		t.Errorf("limit>len(s) 应原样返回：out=%q panic=%v", out, p)
	}
	// limit == len(s)：原样返回。
	if out, p := safe("虫族", 6); p != nil || out != "虫族" {
		t.Errorf("limit==len(s) 应原样返回：out=%q panic=%v", out, p)
	}
	// limit < 0：不得 panic，且前缀为空。
	out, p := safe("虫族abc", -3)
	if p != nil {
		t.Fatalf("limit<0 触发 panic：%v", p)
	}
	if pre := truncPrefixOf(t, out); pre != "" {
		t.Errorf("limit<0 前缀应为空，实得 %q", pre)
	}
}

// TestTruncateASCIIUnchanged — 纯 ASCII 档与旧实现逐字同（除提示口径词）：证明「只退边界、不改语义」。
func TestTruncateASCIIUnchanged(t *testing.T) {
	s := strings.Repeat("abcdefghij", 30) // 300B
	got := truncate(s, 100)
	if want := s[:100]; truncPrefixOf(t, got) != want {
		t.Errorf("ASCII 前缀应与旧实现逐字同：got %q want %q", truncPrefixOf(t, got), want)
	}
	if !strings.HasSuffix(got, "\n...(output truncated — 300 bytes total)") {
		t.Errorf("ASCII 提示尾不对：%q", got)
	}
}
