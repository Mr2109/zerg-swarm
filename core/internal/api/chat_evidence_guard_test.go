package api

import (
	"sort"
	"strings"
	"testing"
)

// TestAnnotateUnverifiedClaim —— 「声称实测但本轮无工具调用」标注（A07）的四格触发面。
func TestAnnotateUnverifiedClaim(t *testing.T) {
	cases := []struct {
		name      string
		content   string
		toolCalls int
		wantAnnot bool
		wantTerm  string
	}{
		{"① 零工具 + 『实测』", "三条命令我都实测了，rc 全 0。", 0, true, "实测"},
		{"①b 零工具 + 英文 verified（大写）", "I Verified all three gates already.", 0, true, "verified"},
		{"② 跑过 3 次工具 + 『实测』", "三条命令我都实测了，rc 全 0。", 3, false, ""},
		{"③ 零工具 + 无该类词", "这三条命令看着应该没问题，你先试试。", 0, false, ""},
		{"④ 空正文", "   ", 0, false, ""},
	}
	for _, c := range cases {
		got := annotateUnverifiedClaim(c.content, c.toolCalls)
		annotated := got != c.content
		if annotated != c.wantAnnot {
			t.Fatalf("%s：期望标注=%v 实得=%v（%q）", c.name, c.wantAnnot, annotated, got)
		}
		if !c.wantAnnot {
			if got != c.content {
				t.Fatalf("%s：不该动正文，实得 %q", c.name, got)
			}
			continue
		}
		if !strings.HasPrefix(got, c.content) {
			t.Fatalf("%s：原正文未逐字保留：%q", c.name, got)
		}
		if !strings.Contains(got, evidenceClaimMarker) {
			t.Fatalf("%s：缺标注前缀：%q", c.name, got)
		}
		if !strings.Contains(got, "「"+c.wantTerm+"」") {
			t.Fatalf("%s：应点名命中的「%s」，实得：%q", c.name, c.wantTerm, got)
		}
		if !strings.Contains(got, "没有可核的证据") {
			t.Fatalf("%s：标注缺少「无可核证据」结论：%q", c.name, got)
		}
	}
}

// TestUnverifiedClaimSkipsNegation —— 否定语境不算「声称实测」（保守起见不标）。
func TestUnverifiedClaimSkipsNegation(t *testing.T) {
	for _, in := range []string{"这些还没实测，只是推断。", "The gates are unverified so far."} {
		if got := annotateUnverifiedClaim(in, 0); got != in {
			t.Fatalf("否定语境被误标：%q ⇒ %q", in, got)
		}
	}
}

// TestEvidenceAndDegenerateCoexist —— 与退化标注**互不吞没**：两条都在、原正文逐字保留、先标后标都一样。
func TestEvidenceAndDegenerateCoexist(t *testing.T) {
	const base = "实测 pong! 🏓" // 短正文（<16 字符）⇒ 退化判据命中；含「实测」⇒ 实测判据命中
	orders := map[string]string{
		"先退化后实测": annotateUnverifiedClaim(annotateDegenerateAnswer(base, 5), 0),
		"先实测后退化": annotateDegenerateAnswer(annotateUnverifiedClaim(base, 0), 5),
	}
	for name, got := range orders {
		if !strings.HasPrefix(got, base) {
			t.Fatalf("%s：原正文未逐字保留：%q", name, got)
		}
		if !strings.Contains(got, evidenceClaimMarker) {
			t.Fatalf("%s：少了实测标注：%q", name, got)
		}
		if !strings.Contains(got, degenerateGuardMarker) {
			t.Fatalf("%s：少了退化标注：%q", name, got)
		}
		if body := strings.TrimSpace(stripGuardAnnotations(got)); body != base {
			t.Fatalf("%s：剥掉两条标注后正文应逐字等于原稿，实得 %q", name, body)
		}
	}
	a, b := orders["先退化后实测"], orders["先实测后退化"]
	if got, want := strings.Join(blocksOf(a, base), "\n--\n"), strings.Join(blocksOf(b, base), "\n--\n"); got != want {
		t.Fatalf("两条标注的内容顺序相关（应无关）：\n先退化后实测=%q\n先实测后退化=%q", a, b)
	}
}

// blocksOf —— 把正文之后的标注块切出来排序（块之间谁先谁后不影响判断，前缀已能各自认出）。
func blocksOf(content, body string) []string {
	rest := strings.TrimPrefix(content, body)
	var out []string
	for _, p := range strings.Split(rest, "\n\n⚠ **") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out
}

// TestDegenerateTextDoesNotSelfTrigger —— 退化标注文案自带「跑过」，不得反过来触发实测标注。
func TestDegenerateTextDoesNotSelfTrigger(t *testing.T) {
	got := annotateDegenerateAnswer("pong! 🏓", 5)
	if again := annotateUnverifiedClaim(got, 0); again != got {
		t.Fatalf("退化标注文案自触发实测标注：%q", again)
	}
}
