package orchestrator

// zz_truncate_utf8_test.go — orchestrator.truncate 的切口必须落在 UTF-8 字符边界。
// 唯一调用点：orchestrator.go:152 `agentstate.NewState("MoA 执行: "+truncate(request, 100), …)`
// —— 这是状态文件里的 objective（人面/机面都会读，JSON 落盘时非法字节会被替换成 U+FFFD）。
// 口径：预算仍是 n 字节；切口向左退到字符起始（≤n，严格不超预算）。
// 造不出「切口落在字中」的夹具就 Fatal。

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// straddleUTF8Orch — 造 len(s)==size 的合法 UTF-8 串，且 cut 落在字符中间。
func straddleUTF8Orch(fill string, size int, cut int) (string, bool) {
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
		if cut > 0 && cut < len(s) && !utf8.RuneStart(s[cut]) {
			return s, true
		}
	}
	return "", false
}

func TestTruncateUTF8Boundary(t *testing.T) {
	cases := []struct {
		name string
		fill string
	}{
		{"中文", "测"},
		{"emoji", "🙂"},
		{"四字节生僻字", "𠀀"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var in string
			found := false
			for size := 101; size <= 140 && !found; size++ {
				if s, ok := straddleUTF8Orch(tc.fill, size, 100); ok {
					in, found = s, true
				}
			}
			if !found {
				t.Fatalf("夹具造不出：fill=%q 无法让第 100 字节落在字中", tc.fill)
			}
			got := truncate(in, 100)
			if !utf8.ValidString(got) {
				lo, hi := 100-3, 100+3
				if lo < 0 {
					lo = 0
				}
				if hi > len(in) {
					hi = len(in)
				}
				t.Fatalf("切口切在字符中间：objective 非法 UTF-8（切口附近字节 %q · 原文 %d 字节）",
					in[lo:hi], len(in))
			}
			if len(got) > 100 || len(got) < 97 {
				t.Fatalf("预算越界：%d 字节（预算 100，允许向左退 0–3 字节）", len(got))
			}
			if !strings.HasPrefix(in, got) {
				t.Fatal("结果不是原文真前缀")
			}
		})
	}
}

// ASCII 旧口径逐字不变（既有 TestTruncate 已钉 same 语义，这里再钉一次逐字节）。
func TestTruncateASCIIUnchanged(t *testing.T) {
	if got := truncate("hello world", 5); got != "hello" {
		t.Fatalf("ASCII 结果被改动：%q", got)
	}
	if got := truncate("hello", 10); got != "hello" {
		t.Fatalf("短串被截断：%q", got)
	}
	if got := truncate("", 3); got != "" {
		t.Fatalf("空串被改动：%q", got)
	}
}
