// chat_tool_registry_i18n_test.go — 多语言 L3/D1：tool_search 中英双语索引（缺陷 3 回归防线）
//
// 背景：注册中心的关键词与类别原本只有中文 → 英文用户搜 "system"/"video" 一律落空。
// 修复：CategoryEN 类别英文别名 + 工具名分词命中（与中文同权）。
// 本测试同时锁住"短词不误召回"的长度守卫。
package chat

import (
	"strings"
	"testing"
)

func TestChatToolSearchBilingual(t *testing.T) {
	// 探针模式：ZERG_SEARCH_PROBE=1 时打印各查询命中，便于人工确认召回质量
	probe := func(query string) []string {
		return ChatToolSearch(query, map[string]bool{}, 8)
	}

	cases := []struct {
		name  string
		query string
		// wantAll：命中结果必须包含这些工具
		wantAll []string
		// wantNonEmpty：至少命中一个
		wantNonEmpty bool
	}{
		{name: "中文-系统", query: "系统", wantNonEmpty: true},
		{name: "英文-system", query: "system", wantNonEmpty: true},
		{name: "英文-video", query: "video", wantNonEmpty: true},
		{name: "英文-music", query: "music", wantNonEmpty: true},
		{name: "英文-knowledge", query: "knowledge", wantNonEmpty: true},
		{name: "英文-file", query: "file", wantNonEmpty: true},
		{name: "英文-tool(工具名分词)", query: "footage", wantNonEmpty: true},
		{name: "中文-剪辑", query: "剪辑", wantNonEmpty: true},
		{name: "英文-port", query: "port", wantNonEmpty: true},
		{name: "英文-task", query: "task", wantNonEmpty: true},
	}

	for _, c := range cases {
		got := probe(c.query)
		if c.wantNonEmpty && len(got) == 0 {
			t.Errorf("%s: query=%q 零命中（双语索引失效）", c.name, c.query)
		}
		for _, w := range c.wantAll {
			if !contains(got, w) {
				t.Errorf("%s: query=%q 未命中 %q（实得 %v）", c.name, c.query, w, got)
			}
		}
		t.Logf("query=%-12q → %v", c.query, got)
	}

	// 中英文同一意图必须有交集（system ↔ 系统 都指向系统类工具）
	zh := probe("系统")
	en := probe("system")
	if len(zh) == 0 || len(en) == 0 {
		t.Fatalf("系统类中英查询应均有命中：zh=%v en=%v", zh, en)
	}
	if !intersects(zh, en) {
		t.Errorf("中英同义查询无交集：zh=%v en=%v", zh, en)
	}

	// 长度守卫（两道）：
	//   ① 英文类别/工具名分词：短词（<4）只做全等；
	//   ② 工具名子串：短词（<3）只做全等。
	// 反例 "id"：修复前会命中 validate/video 等子串（既有泄漏），修复后应零命中。
	short := probe("id")
	if len(short) != 0 {
		t.Errorf("短词 \"id\" 召回 %d 条（应 0，长度守卫失效）：%v", len(short), short)
	}
	// 但真正的短工具名要能搜到（全等路径）
	if got := probe("ls"); !contains(got, "ls") {
		t.Errorf("短工具名 \"ls\" 应命中自身，实得 %v", got)
	}
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

func intersects(a, b []string) bool {
	for _, x := range a {
		if contains(b, x) {
			return true
		}
	}
	return false
}

// TestCategoryENCoverage — 每个注册条目的类别都必须有英文别名（防新增类别漏配 → 英文又搜不到）
func TestCategoryENCoverage(t *testing.T) {
	missing := map[string]bool{}
	for _, meta := range chatToolRegistry {
		if len(CategoryENOf(meta.Category)) == 0 {
			missing[meta.Category] = true
		}
	}
	if len(missing) > 0 {
		names := make([]string, 0, len(missing))
		for c := range missing {
			names = append(names, c)
		}
		t.Errorf("以下类别缺英文别名（CategoryEN）：%s", strings.Join(names, ", "))
	}
}
