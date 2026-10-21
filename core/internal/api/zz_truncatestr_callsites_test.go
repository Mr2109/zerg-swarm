package api

// zz_truncatestr_callsites_test.go — truncateStr 各调用点的预算面（含 200）必须全程合法 UTF-8。
//
// 调用点清单（本单核过的一手位置，全部走同一个 truncateStr）：
//
//	zerg_flow_executor.go:318  CommitRoundWithFS map{"result": truncateStr(res.LastOutput, 200)}  ← 任务记忆（进下一轮 prompt）
//	zerg_flow_executor.go:345  CommitRoundWithFS map{"result": truncateStr(res.LastOutput, 200)}  ← 重做路径同上
//	zerg_flow_executor.go:345  "重做成功: "+truncateStr(res.LastOutput, 60)
//	zerg_flow_executor.go:766  truncateStr(body, 200)   ← 模型错误面（回喂给模型重试）
//	zerg_controlled_loop.go:59 truncateStr(string(out), 300)  ← 验证失败反馈（进 prompt）
//	zerg_controlled_loop.go:61 truncateStr(string(out), 100)  ← 验证通过回执
//	zerg_semantic_parse.go:256/258 truncateStr(string(out), 1500) ← run 输出
//	zerg_semantic_parse.go:200/207 truncateStr(act.OldText/NewText, 40/30)
//
// 其中「read 预览（前 2000 + 后 1000）→ LastOutput → truncateStr(...,200)」是真实的二次切链条：
// 上游把预览修成合法 UTF-8 之后，下游这一刀仍会切在字中 ⇒ 两处必须各修一刀。

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
)

// straddleUTF8Chain — 造 len(s)==size 的合法 UTF-8 串，且每个 cut 都落在字符中间。
func straddleUTF8Chain(fill string, size int, cuts ...int) (string, bool) {
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

// TestReadThenTruncateStrChainUTF8 — 真链条：ExecuteActionInDir(read) → truncateStr(...,200)。
func TestReadThenTruncateStrChainUTF8(t *testing.T) {
	dir := t.TempDir()
	var content string
	found := false
	for size := 4101; size <= 4300 && !found; size++ {
		if s, ok := straddleUTF8Chain("测", size, 2000, size-1000); ok {
			content, found = s, true
		}
	}
	if !found {
		t.Fatal("夹具造不出：无法让 read 的 2000 与 size-1000 两刀同时落在字中")
	}
	if err := os.WriteFile(filepath.Join(dir, "big.txt"), []byte(content), 0o644); err != nil {
		t.Fatalf("写夹具: %v", err)
	}
	out, err := ExecuteActionInDir(&Action{Type: "read", Path: "big.txt"}, dir)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !utf8.ValidString(out) {
		t.Fatalf("read 预览的双切口切在字符中间：回灌串非法 UTF-8（%q / %q · 原文 %d 字节）",
			content[1997:2003], content[len(content)-1003:len(content)-997], len(content))
	}
	// 下游二次切：提交记忆 / 错误面 / 验证反馈都走这一刀
	msg := truncateStr(out, 200)
	if !utf8.ValidString(msg) {
		i := strings.Index(msg, "...")
		t.Fatalf("下游 truncateStr(...,200) 二次切在字符中间：非法 UTF-8（%q）", msg[maxInt(0, i-6):minIntChain(len(msg), i+9)])
	}
	if len(msg) > 203 {
		t.Fatalf("下游 200 预算越界：%d > 203", len(msg))
	}
}

// TestTruncateStrCallSiteBudgets — 把上面清单里的每个预算逐个钉一遍（夹具两刀都落在字中）。
func TestTruncateStrCallSiteBudgets(t *testing.T) {
	budgets := []struct {
		n    int
		from string
	}{
		{200, "CommitRoundWithFS result / body 错误面"},
		{80, "CommitRoundWithFS message"},
		{60, "重做成功 消息"},
		{300, "验证失败反馈"},
		{100, "验证通过回执"},
		{1500, "run 输出"},
		{40, "OldText 错误提示"},
		{30, "patch 成功回执"},
		{1200, "Skill 回灌"},
		{4000, "Skill 全文回灌"},
		{2000, "Skill summary 回灌"},
	}
	for _, b := range budgets {
		t.Run(strings.Join([]string{strings.TrimSpace(b.from), strconv.Itoa(b.n)}, "/"), func(t *testing.T) {
			var in string
			found := false
			for size := b.n + 8; size <= b.n+60 && !found; size++ {
				if s, ok := straddleUTF8Chain("测", size, b.n/2, size-b.n/2); ok {
					in, found = s, true
				}
			}
			if !found {
				t.Fatalf("夹具造不出：n=%d 无法让两刀同时落在字中", b.n)
			}
			got := truncateStr(in, b.n)
			if !utf8.ValidString(got) {
				t.Fatalf("n=%d 切口切在字符中间：非法 UTF-8（%q）", b.n, got)
			}
			if len(got) > b.n+3 {
				t.Fatalf("n=%d 预算越界：%d > %d", b.n, len(got), b.n+3)
			}
		})
	}
}

func minIntChain(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
