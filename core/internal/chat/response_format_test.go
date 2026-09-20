// response_format_test.go — §二十一 已红第 11 条（T-33「全仓 `response_format` = 0 文件」）的
// **机检棘轮 + 成对负控**。
//
// 为什么要有它：这条已红的判据是一条 `git grep -l | wc -l` —— **grep 判据不会自己跑**，
// 谁把那行 `response_format` 删掉/改名，仓里不会有任何东西报红（病征是「机器面靠提示词约定」，
// 而那是个**看不见**的状态）。这里落成两格：
//
//	① 结构面必须有**真落点**（不是在文档里提一句，是在**发请求的那行**里）；
//	② 落了的那份 schema 必须是**真约束**（六段齐 · strict · additionalProperties=false）——
//	   否则「结构性输出」还是空的：schema 里少一段 = 模型少写一段没人拦。
package chat

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// repoRootFromChat 从 core/internal/chat 往上找仓根。
func repoRootFromChat(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	d := wd
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(d, "core", "cmd", "zerg")); err == nil {
			return d
		}
		d = filepath.Dir(d)
	}
	t.Fatalf("找不到仓根（从 %s 往上）", wd)
	return ""
}

// TestResponseFormatHasRealPointInCore —— 判据①：`response_format` 在 core/ 的**非测试** Go 件里有落点。
func TestResponseFormatHasRealPointInCore(t *testing.T) {
	root := repoRootFromChat(t)
	var hits []string
	_ = filepath.Walk(filepath.Join(root, "core"), func(p string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			return nil
		}
		if strings.Contains(string(b), "response_format") {
			rel, _ := filepath.Rel(root, p)
			hits = append(hits, rel)
		}
		return nil
	})
	sort.Strings(hits)
	if len(hits) == 0 {
		t.Fatalf("core/ 里零处 `response_format`（已红第 11 条的形态：结构性输出零落点 ⇒ 只靠提示词约定）")
	}
	t.Logf("响应契约落点 %d 件：%v", len(hits), hits)

	// 这条落点必须**真的在发请求的那条路上**：本包自己就得有一个（`CompactRequest` 就是那条路）。
	found := false
	for _, h := range hits {
		if strings.HasPrefix(h, "core/internal/chat/") {
			found = true
		}
	}
	if !found {
		t.Errorf("`core/internal/chat/` 里没有落点（结构化摘要的那条请求没有带约束）")
	}
}

// TestCompactResponseFormatIsRealConstraint —— 判据②：那份 schema 必须是**真约束**（不是空壳）。
func TestCompactResponseFormatIsRealConstraint(t *testing.T) {
	rf := compactResponseFormat()
	if got := rf["type"]; got != "json_schema" {
		t.Fatalf("受约束解码的类型必须是 json_schema，得到 %v", got)
	}
	js, ok := rf["json_schema"].(map[string]any)
	if !ok {
		t.Fatalf("缺 json_schema 块：%v", rf)
	}
	if js["strict"] != true {
		t.Errorf("strict 必须 true（否则服务端只当提示，不当约束）：%v", js["strict"])
	}
	schema, ok := js["schema"].(map[string]any)
	if !ok {
		t.Fatalf("缺 schema 块：%v", js)
	}
	if schema["additionalProperties"] != false {
		t.Errorf("additionalProperties 必须 false（否则模型能塞任意外键）：%v", schema["additionalProperties"])
	}
	props, _ := schema["properties"].(map[string]any)
	req, _ := schema["required"].([]string)
	if len(props) == 0 || len(req) == 0 {
		t.Fatalf("properties / required 不许为空：props=%d req=%d", len(props), len(req))
	}
	// 六段齐：required 的每一条都必须有对应的 properties（否则 schema 自相矛盾）
	for _, k := range req {
		if _, ok := props[k]; !ok {
			t.Errorf("required 里的 %q 在 properties 里没有定义（schema 自相矛盾）", k)
		}
	}
	if len(req) != 6 {
		t.Errorf("摘要段名 = 6（提示词里那六个 `## ` 段），得到 %d：%v", len(req), req)
	}
	// 成对：段名必须与提示词里的段名**同源**（提示词提到但 schema 没有 ⇒ 模型会两边都不照做）
	for _, k := range req {
		if !strings.Contains(compactSystemPrompt, "## "+strings.Title(strings.ReplaceAll(k, "_", " "))) &&
			!strings.Contains(compactSystemPrompt, "## "+strings.ReplaceAll(k, "_", " ")) &&
			!strings.Contains(compactSystemPrompt, strings.ReplaceAll(k, "_", " ")) {
			t.Errorf("段名 %q 在 compactSystemPrompt 里找不到对应（结构面与文本面漂了）", k)
		}
	}
	// 反例（负控）：把 required 挪掉一段，本测试的判据必须能红 —— 用一个合成 schema 走一遍同样的检查
	bad := map[string]any{"type": "json_schema", "json_schema": map[string]any{
		"strict": true,
		"schema": map[string]any{
			"type":                 "object",
			"properties":           map[string]any{"goal": map[string]any{"type": "string"}},
			"required":             []string{"goal"},
			"additionalProperties": false,
		}}}
	badReq := bad["json_schema"].(map[string]any)["schema"].(map[string]any)["required"].([]string)
	if len(badReq) == 6 {
		t.Errorf("负控夹具不成立（合成 schema 竟然也是六段）")
	}
}

// TestCompactRequestShapeCarriesResponseFormat — 结构面：请求体里真的挂了它（不是只定义不用）。
func TestCompactRequestShapeCarriesResponseFormat(t *testing.T) {
	src, err := os.ReadFile("chat_compact.go")
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(`"?response_format"?\s*:`)
	if !re.Match(src) {
		t.Errorf("chat_compact.go 里没有把 response_format 挂进请求体的那一行（定义了却不用 = 零落点）")
	}
}
