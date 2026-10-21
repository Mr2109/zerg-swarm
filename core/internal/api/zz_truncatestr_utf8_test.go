package api

// zz_truncatestr_utf8_test.go — truncateStr 的「头 n/2 + 尾 n/2」双切口必须落在字符边界。
// truncateStr 是 api 包唯一的截断器：read 预览下游（提交记忆 200 / 消息 80 / 验证反馈 300/100 /
// 命令输出 1500 / 错误面 40/30）全走它 ⇒ 一刀切在字中，四处输出都带半个字。
// 口径：预算仍是 n 字节（结果总长 ≤ n+3），只把两刀各退/推到字符起始。
// 造不出「两刀同时落在字中」的夹具就 Fatal。

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// straddleUTF8API — 造一份 len(s)==size 的合法 UTF-8 串，且每个 cut 都落在字符中间。
func straddleUTF8API(fill string, size int, cuts ...int) (string, bool) {
	w := len(fill)
	for pad := 0; pad < w; pad++ {
		nRep := (size - pad) / w
		if nRep <= 0 {
			continue
		}
		used := pad + nRep*w
		if used > size {
			continue
		}
		s := strings.Repeat("x", pad) + strings.Repeat(fill, nRep) + strings.Repeat("y", size-used)
		if len(s) != size || !utf8.ValidString(s) {
			continue
		}
		mid := true
		for _, c := range cuts {
			if c <= 0 || c >= len(s) || utf8.RuneStart(s[c]) {
				mid = false
				break
			}
		}
		if mid {
			return s, true
		}
	}
	return "", false
}

func TestTruncateStrUTF8Boundary(t *testing.T) {
	cases := []struct {
		name string
		fill string
		n    int
	}{
		{"中文 n200", "测", 200},
		{"emoji n200", "🙂", 200},
		{"四字节 n200", "𠀀", 200},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var in string
			found := false
			for size := tc.n + 8; size <= tc.n+60 && !found; size++ {
				if s, ok := straddleUTF8API(tc.fill, size, tc.n/2, size-tc.n/2); ok {
					in, found = s, true
				}
			}
			if !found {
				t.Fatalf("夹具造不出：fill=%q n=%d 无法让 n/2 与 len-n/2 两刀同时落在字中", tc.fill, tc.n)
			}
			got := truncateStr(in, tc.n)
			if !utf8.ValidString(got) {
				t.Fatalf("双切口切在字符中间：结果非法 UTF-8（头切口 %q · 尾切口 %q · 原文 %d 字节 n=%d）",
					in[tc.n/2-3:tc.n/2+3], in[len(in)-tc.n/2-3:len(in)-tc.n/2+3], len(in), tc.n)
			}
			i := strings.Index(got, "...")
			if i < 0 {
				t.Fatalf("缺少省略标记：%q", got)
			}
			head, tail := got[:i], got[i+3:]
			if !utf8.ValidString(head) || !utf8.ValidString(tail) {
				t.Fatalf("头/尾任一段非法 UTF-8：head=%v tail=%v", utf8.ValidString(head), utf8.ValidString(tail))
			}
			// 预算仍是字节：总长 ≤ n+3；两段各 ≤ n/2，且不许「退得过头」
			if len(got) > tc.n+3 {
				t.Fatalf("总长越预算：%d > n+3=%d", len(got), tc.n+3)
			}
			if len(head) > tc.n/2 || len(head) < tc.n/2-3 {
				t.Fatalf("头段越预算：%d 字节（预算 %d）", len(head), tc.n/2)
			}
			if len(tail) > tc.n/2 || len(tail) < tc.n/2-3 {
				t.Fatalf("尾段越预算：%d 字节（预算 %d）", len(tail), tc.n/2)
			}
			if !strings.HasPrefix(in, head) || !strings.HasSuffix(in, tail) {
				t.Fatal("头/尾不是原文真前缀/真后缀")
			}
		})
	}
}

// ASCII 旧口径逐字不变：切口不落在字中 ⇒ 结果必须与改前逐字节相同。
func TestTruncateStrASCIIUnchanged(t *testing.T) {
	s := strings.Repeat("ab", 100) // 200 字节
	old := s[:50] + "..." + s[len(s)-50:]
	if got := truncateStr(s, 100); got != old {
		t.Fatalf("ASCII 结果被改动：\n got=%q\n old=%q", got, old)
	}
	// 边界：短串不截断；n 为奇数时头段 = n/2（向下取整）不变
	if got := truncateStr("hello", 10); got != "hello" {
		t.Fatalf("短串被截断：%q", got)
	}
	if got := truncateStr("0123456789", 5); got != "01...89" {
		t.Fatalf("奇数 n 口径被改动：%q", got)
	}
}
