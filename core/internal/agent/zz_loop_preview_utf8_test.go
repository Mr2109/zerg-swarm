package agent

// zz_loop_preview_utf8_test.go — persistOversized 的「头 3000 + 尾 1000」预览双切口
// 必须落在 UTF-8 字符边界：预览会整段进 prompt（超大工具结果回喂），
// 半个字进 prompt = 模型读到乱码；且双切口在同一条 union 里，两刀都可能中。
// 口径：字节预算不变（头 ≤3000 字节 / 尾 ≤1000 字节），只把切口退到字符起始。
// 造不出「两刀同时落在字中」的夹具就 Fatal —— 不许静默跳过。

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

// straddleUTF8Agent — 造一份 len(s)==size 的合法 UTF-8 串，且每个 cut 都落在字符中间。
// pad（ASCII 'x'）+ N 份 fill + ASCII 'y' 补尾 ⇒ 相位可控、源串本身永远合法。
func straddleUTF8Agent(fill string, size int, cuts ...int) (string, bool) {
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

// splitOversizedPreview 从 persistOversized 的回喂串里切出 head / tail 两段。
func splitOversizedPreview(out string) (head, tail string, ok bool) {
	const headMark = "\n…[中间省略 "
	const tailMark = "]…\n"
	const footMark = "\n[完整内容在上述文件——需要时用 read 读取该文件]"
	i := strings.Index(out, "]\n") // 头行 "[超大输出 N 字节——已持久化到 …]" 结束
	j := strings.Index(out, headMark)
	k := strings.Index(out, tailMark)
	f := strings.Index(out, footMark)
	if i < 0 || j < 0 || k < 0 || f < 0 || !(i+2 <= j && j < k && k+len(tailMark) <= f) {
		return "", "", false
	}
	return out[i+2 : j], out[k+len(tailMark) : f], true
}

func TestPersistOversizedPreviewUTF8Boundary(t *testing.T) {
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
			var content string
			found := false
			for size := 4101; size <= 4300 && !found; size++ {
				if s, ok := straddleUTF8Agent(tc.fill, size, 3000, size-1000); ok {
					content, found = s, true
				}
			}
			if !found {
				t.Fatalf("夹具造不出：fill=%q 在 4101..4300 内无法让 3000 与 size-1000 两刀同时落在字中", tc.fill)
			}

			ls := &loopState{agent: &Agent{execContext: &ExecContext{WorkDir: t.TempDir()}}}
			out, err := ls.persistOversized("read", content)
			if err != nil {
				t.Fatalf("persistOversized: %v", err)
			}
			if !utf8.ValidString(out) {
				t.Fatalf("预览切口切在字符中间：整段回喂串非法 UTF-8（头切口 %q · 尾切口 %q · 原文 %d 字节）",
					content[2997:3003], content[len(content)-1003:len(content)-997], len(content))
			}
			head, tail, ok := splitOversizedPreview(out)
			if !ok {
				t.Fatalf("预览标记解析失败：%q", out[:previewMin(160, len(out))])
			}
			if !utf8.ValidString(head) || !utf8.ValidString(tail) {
				t.Fatalf("头/尾任一段非法 UTF-8：head 合法=%v tail 合法=%v",
					utf8.ValidString(head), utf8.ValidString(tail))
			}
			// 预算仍是字节：头 ≤3000、尾 ≤1000，且不许「退得过头」
			if len(head) > 3000 || len(head) < 2990 {
				t.Fatalf("头预算越界：%d 字节（预算 3000，允许切口后退 0–3 字节）", len(head))
			}
			if len(tail) > 1000 || len(tail) < 990 {
				t.Fatalf("尾预算越界：%d 字节（预算 1000，允许切口后推 0–3 字节）", len(tail))
			}
			// 头尾必须真是原文的子串（不是重排/补白）
			if !strings.HasPrefix(content, head) || !strings.HasSuffix(content, tail) {
				t.Fatal("头/尾不是原文的真前缀/真后缀")
			}
			// 文件里必须是完整内容（预览变了，持久化全文不能变）
			files, _ := filepath.Glob(filepath.Join(ls.agent.execContext.WorkDir, ".zerg", "tool_output", "*"))
			if len(files) != 1 {
				t.Fatalf("持久化文件数=%d，应 1", len(files))
			}
			saved, err := os.ReadFile(files[0])
			if err != nil {
				t.Fatalf("读回持久化文件: %v", err)
			}
			if string(saved) != content {
				t.Fatalf("持久化内容被改动：%d 字节 ≠ 原文 %d 字节", len(saved), len(content))
			}
		})
	}
}

// ASCII 旧口径逐字不变：切口不落在字中时，两段必须与改前完全一致。
func TestPersistOversizedPreviewASCIIUnchanged(t *testing.T) {
	content := strings.Repeat("a", 3000) + strings.Repeat("b", 200) + strings.Repeat("c", 1000)
	ls := &loopState{agent: &Agent{execContext: &ExecContext{WorkDir: t.TempDir()}}}
	out, err := ls.persistOversized("read", content)
	if err != nil {
		t.Fatalf("persistOversized: %v", err)
	}
	head, tail, ok := splitOversizedPreview(out)
	if !ok {
		t.Fatal("预览标记解析失败")
	}
	if head != content[:3000] || tail != content[len(content)-1000:] {
		t.Fatalf("ASCII 切口被改动：head=%d tail=%d", len(head), len(tail))
	}
}

// 短输出（≤4000）不截断：整段原样回喂。
func TestPersistOversizedPreviewShortUnchanged(t *testing.T) {
	content := "短输出"
	ls := &loopState{agent: &Agent{execContext: &ExecContext{WorkDir: t.TempDir()}}}
	out, err := ls.persistOversized("read", content)
	if err != nil {
		t.Fatalf("persistOversized: %v", err)
	}
	if !strings.Contains(out, content) || strings.Contains(out, "中间省略") {
		t.Fatalf("短输出被截断：%q", out)
	}
}

func previewMin(a, b int) int {
	if a < b {
		return a
	}
	return b
}
