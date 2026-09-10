// tool_desc_audit_test.go — L3/D1 工具描述审计（多语言 2026-09-11）
//
// 目的：把**真正注入模型的描述文本**逐条导出，作为 D1「信息密度审计 + 紧凑化」的前后对比基准。
// 覆盖七个来源：Hermes 工具定义表 / chat tools 参数 / tool_search 定义 / Hermes 工具提示固定段 /
// 对话扩展工具（deferred，tool_search 命中后回传）/ 注册中心搜索关键词 / agent 工具表。
//
// 默认不写盘、不改动仓库（CI 无副作用）：仅当 ZERG_DESC_AUDIT_OUT=<path> 时写出 TSV。
//
// 用法：
//
//	ZERG_DESC_AUDIT_OUT=docs/项目文档/v2.5.9/i18n-audit/L3-D1-描述审计.tsv \
//	  go test ./internal/chat -run TestToolDescAudit -count=1 -v
package chat

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/agent"
)

// descRow — 一条注入描述
type descRow struct {
	Source string // 来源
	Name   string // 工具名 / 段落名
	Runes  int    // 字符数（中文按字计）
	Bytes  int    // 字节数
	Lines  int    // 行数（含内嵌换行）
	Desc   string
}

// redundancy — 冗余启发式标记（D1 审计用；命中即候选压缩点）
func redundancy(s string) string {
	var marks []string
	if strings.Contains(s, "——") {
		marks = append(marks, "破折号连串")
	}
	if strings.Contains(s, "v1.0.") || strings.Contains(s, "P4-") || strings.Contains(s, "P5-") {
		marks = append(marks, "版本/任务号")
	}
	if strings.Contains(s, "【参数】") || strings.Contains(s, "参数:") || strings.Contains(s, "参数：") {
		marks = append(marks, "参数说明(疑与schema重复)")
	}
	if strings.Contains(s, "【示例】") || strings.Contains(s, "示例】") {
		marks = append(marks, "示例")
	}
	if strings.Contains(s, "（如") || strings.Contains(s, "(如") {
		marks = append(marks, "举例")
	}
	if strings.Contains(s, "专用工具") {
		marks = append(marks, "跨工具样板")
	}
	if strings.Contains(s, "中文") || strings.Contains(s, "English") {
		marks = append(marks, "语言提示")
	}
	if strings.Contains(s, "\n\n") {
		marks = append(marks, "多段")
	}
	return strings.Join(marks, "|")
}

func TestToolDescAudit(t *testing.T) {
	var rows []descRow
	add := func(source, name, desc string) {
		if strings.TrimSpace(desc) == "" {
			return
		}
		rows = append(rows, descRow{
			Source: source, Name: name,
			Runes: len([]rune(desc)), Bytes: len(desc),
			Lines: strings.Count(desc, "\n") + 1,
			Desc:  desc,
		})
	}

	// ① Hermes 工具定义表（<tools> 分支 L0）
	for _, d := range HermesToolDefs() {
		add("hermes_defs", d.Name, d.Desc)
	}
	// ①b Hermes 定义表里的参数说明
	for _, d := range HermesToolDefs() {
		keys := make([]string, 0, len(d.Props))
		for k := range d.Props {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if m, ok := d.Props[k].(map[string]any); ok {
				if s, ok2 := m["description"].(string); ok2 && s != "" {
					add("hermes_defs.param", d.Name+"."+k, s)
				}
			}
		}
	}
	// ② chat 格式 tools 参数（含 P4-38 few-shot 追加）
	for _, m := range BuildToolsParam() {
		fn, ok := m["function"].(map[string]any)
		if !ok {
			continue
		}
		name, _ := fn["name"].(string)
		desc, _ := fn["description"].(string)
		add("tools_param", name, desc)
		if props, ok := fn["parameters"].(map[string]any); ok {
			if pm, ok2 := props["properties"].(map[string]any); ok2 {
				keys := make([]string, 0, len(pm))
				for k := range pm {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				for _, k := range keys {
					if mm, ok3 := pm[k].(map[string]any); ok3 {
						if s, ok4 := mm["description"].(string); ok4 && s != "" {
							add("tools_param.param", name+"."+k, s)
						}
					}
				}
			}
		}
	}
	// ③ tool_search 定义（chat 格式）
	if m, ok := BuildToolSearchParam()["function"].(map[string]any); ok {
		name, _ := m["name"].(string)
		desc, _ := m["description"].(string)
		add("tool_search", name, desc)
		if props, ok := m["parameters"].(map[string]any); ok {
			if pm, ok2 := props["properties"].(map[string]any); ok2 {
				for k, v := range pm {
					if mm, ok3 := v.(map[string]any); ok3 {
						if s, ok4 := mm["description"].(string); ok4 && s != "" {
							add("tool_search.param", name+"."+k, s)
						}
					}
				}
			}
		}
	}
	// ④ Hermes 工具提示里的固定文字段（剔除 <tools> JSON 与示例大括号块）
	for _, line := range strings.Split(BuildHermesToolPrompt(nil), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "<tools>") || strings.HasPrefix(trimmed, "{\"") {
			continue
		}
		if len([]rune(trimmed)) < 12 {
			continue
		}
		add("hermes_prompt", "prompt_block", trimmed)
	}
	// ⑤ 对话扩展工具（deferred——tool_search 命中后回传描述）
	if defs := ChatExtraToolDefs(); defs != nil {
		names := make([]string, 0, len(defs))
		for n := range defs {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, n := range names {
			if fn, ok := defs[n]["function"].(map[string]any); ok {
				if s, ok2 := fn["description"].(string); ok2 {
					add("extra_defs", n, s)
				}
			}
		}
	}
	// ⑥ 注册中心搜索关键词（缺陷 3：中文索引 → 英文用户搜不到）
	for _, e := range chatToolRegistry {
		add("registry_keywords", e.Name, strings.Join(e.Keywords, " "))
	}
	// ⑦ agent 工具表（resident / help:true 用）
	for _, d := range agent.AllTools() {
		add("agent_tools", d.Function.Name, d.Function.Description)
		for k, v := range d.Function.Parameters {
			if k != "properties" {
				continue
			}
			if pm, ok := v.(map[string]any); ok {
				keys := make([]string, 0, len(pm))
				for kk := range pm {
					keys = append(keys, kk)
				}
				sort.Strings(keys)
				for _, kk := range keys {
					if mm, ok3 := pm[kk].(map[string]any); ok3 {
						if s, ok4 := mm["description"].(string); ok4 && s != "" {
							add("agent_tools.param", d.Function.Name+"."+kk, s)
						}
					}
				}
			}
		}
	}

	// 汇总
	totalRunes, totalBytes := 0, 0
	bySource := map[string][2]int{} // source → {条数, 字符数}
	for _, r := range rows {
		totalRunes += r.Runes
		totalBytes += r.Bytes
		v := bySource[r.Source]
		bySource[r.Source] = [2]int{v[0] + 1, v[1] + r.Runes}
	}
	t.Logf("描述条数=%d 总字符=%d 总字节=%d", len(rows), totalRunes, totalBytes)
	srcs := make([]string, 0, len(bySource))
	for s := range bySource {
		srcs = append(srcs, s)
	}
	sort.Strings(srcs)
	for _, s := range srcs {
		t.Logf("  %-20s 条数=%-4d 字符=%d", s, bySource[s][0], bySource[s][1])
	}

	out := os.Getenv("ZERG_DESC_AUDIT_OUT")
	if out == "" {
		t.Log("（未设置 ZERG_DESC_AUDIT_OUT——仅报告，未写盘）")
		return
	}
	var b strings.Builder
	b.WriteString("来源\t名称\t字符数\t字节数\t行数\t冗余标记\t描述\n")
	for _, r := range rows {
		desc := strings.ReplaceAll(r.Desc, "\n", "\\n")
		desc = strings.ReplaceAll(desc, "\t", " ")
		b.WriteString(fmt.Sprintf("%s\t%s\t%d\t%d\t%d\t%s\t%s\n",
			r.Source, r.Name, r.Runes, r.Bytes, r.Lines, redundancy(r.Desc), desc))
	}
	if err := os.WriteFile(out, []byte(b.String()), 0o644); err != nil {
		t.Fatalf("写审计文件失败: %v", err)
	}
	t.Logf("写出: %s（%d 行）", out, len(rows))
}
