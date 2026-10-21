package api

// fixture_test.go — ExecuteAction "read" 的「前 2000 + 后 1000」双切口 UTF-8 判据。
//
// 被测点：zerg_semantic_parse.go:219（`content[:2000]` + `content[len(content)-1000:]`）。
// PRE（仓内原样）应红、POST（两头都退字符起始）应绿；断言同形。
// 四条判据：① 整串合法 UTF-8 ② 头 ≤2000 字节 · 尾 ≤1000 字节 ③ 头/尾各是原内容的前缀/后缀
// ④ 预算没被浪费（头 ≥1990、尾 ≥990）。

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

// straddleBoth — 造一段内容，使「第 head 字节」与「第 len-tail 字节」**都**落在字符中间。
func straddleBoth(t *testing.T, unit string, head, tail, minLen int) string {
	t.Helper()
	for pad := 0; pad < 4; pad++ {
		for suf := 0; suf < 4; suf++ {
			var sb strings.Builder
			sb.WriteString(strings.Repeat("x", pad))
			for sb.Len() <= minLen {
				sb.WriteString(unit)
			}
			s := sb.String() + strings.Repeat("y", suf)
			if len(s) <= minLen || len(s) <= tail {
				continue
			}
			if !utf8.RuneStart(s[head]) && !utf8.RuneStart(s[len(s)-tail]) {
				return s
			}
		}
	}
	t.Fatalf("夹具有问题：造不出「%d 与 len-%d 双切口都落字符中间」的内容（unit=%q）", head, tail, unit)
	return ""
}

func TestReadTruncationUTF8Boundary(t *testing.T) {
	cases := []struct{ name, unit string }{
		{"中文", "测"},     // 3 字节
		{"emoji", "🙂"},  // 4 字节
		{"四字节生僻字", "𠀀"}, // 4 字节
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			content := straddleBoth(t, c.unit, 2000, 1000, 4200)
			if err := os.WriteFile(filepath.Join(dir, "big.txt"), []byte(content), 0o644); err != nil {
				t.Fatalf("夹具写入失败: %v", err)
			}
			a := ParseAction("看看 big.txt", dir)
			if a.Type != "read" {
				t.Fatalf("夹具有问题：ParseAction 没认出 read（type=%s）", a.Type)
			}
			out, err := ExecuteActionInDir(a, dir)
			if err != nil {
				t.Fatalf("read 执行失败: %v", err)
			}
			// 拆出头/尾两段：header\n<head>\n...\n<tail>
			parts := strings.SplitN(out, "\n...\n", 2)
			if len(parts) != 2 {
				t.Fatalf("输出形态变了（找不到 \\n...\\n 分隔）：%q", out)
			}
			body := parts[0]
			sep := strings.Index(body, ":\n")
			if sep < 0 {
				t.Fatalf("输出形态变了（头段没有 ':\\n'）：%q", body)
			}
			headPart, tailPart := body[sep+2:], parts[1]
			// 两刀分开断言 —— 红了要看得出是头还是尾（不是一句「整串非法」）
			headOK, tailOK := utf8.ValidString(headPart), utf8.ValidString(tailPart)
			if !headOK || !tailOK {
				t.Fatalf("切口切在字符中间：头合法=%v（头尾字节 %q）· 尾合法=%v（尾首字节 %q）· 文件共 %d 字节",
					headOK, lastBytes(headPart), tailOK, firstBytes(tailPart), len(content))
			}
			if !utf8.ValidString(out) {
				t.Fatalf("整串含非法 UTF-8（两段各自合法，说明分隔串或 header 有问题）")
			}
			if len(headPart) > 2000 {
				t.Fatalf("头部超预算：%d > 2000 字节", len(headPart))
			}
			if len(tailPart) > 1000 {
				t.Fatalf("尾部超预算：%d > 1000 字节", len(tailPart))
			}
			if len(headPart) < 1990 || len(tailPart) < 990 {
				t.Fatalf("预算浪费过头：头 %d 字节 / 尾 %d 字节（最多各少 3 字节）", len(headPart), len(tailPart))
			}
			if !strings.HasPrefix(content, headPart) {
				t.Fatalf("头段不是原文件前缀")
			}
			if !strings.HasSuffix(content, tailPart) {
				t.Fatalf("尾段不是原文件后缀")
			}
		})
	}
}

// TestReadShortFileUnchanged — 短文件（≤4000 字节）整份返回，口径不变。
func TestReadShortFileUnchanged(t *testing.T) {
	dir := t.TempDir()
	content := "短文件内容：这是一段中文，含 emoji 🙂 和生僻字 𠀀。"
	if err := os.WriteFile(filepath.Join(dir, "small.txt"), []byte(content), 0o644); err != nil {
		t.Fatalf("夹具写入失败: %v", err)
	}
	a := ParseAction("看看 small.txt", dir)
	out, err := ExecuteActionInDir(a, dir)
	if err != nil {
		t.Fatalf("read 执行失败: %v", err)
	}
	if !strings.Contains(out, content) {
		t.Fatalf("短文件应整份返回：%q", out)
	}
	if !utf8.ValidString(out) {
		t.Fatalf("短文件路径不该产生非法 UTF-8")
	}
}

// TestReadASCIITruncationUnchanged — ASCII 逐字不变：头 2000 + 尾 1000 原样。
func TestReadASCIITruncationUnchanged(t *testing.T) {
	dir := t.TempDir()
	content := strings.Repeat("a", 2000) + strings.Repeat("b", 2500) + strings.Repeat("c", 1000)
	if err := os.WriteFile(filepath.Join(dir, "ascii.txt"), []byte(content), 0o644); err != nil {
		t.Fatalf("夹具写入失败: %v", err)
	}
	a := ParseAction("看看 ascii.txt", dir)
	out, err := ExecuteActionInDir(a, dir)
	if err != nil {
		t.Fatalf("read 执行失败: %v", err)
	}
	wantHead, wantTail := content[:2000], content[len(content)-1000:]
	if !strings.Contains(out, wantHead) || !strings.Contains(out, wantTail) {
		t.Fatalf("ASCII 双切口口径变了（头 %d / 尾 %d）", len(wantHead), len(wantTail))
	}
}

func lastBytes(s string) string {
	if len(s) <= 8 {
		return s
	}
	return s[len(s)-8:]
}

func firstBytes(s string) string {
	if len(s) <= 8 {
		return s
	}
	return s[:8]
}
