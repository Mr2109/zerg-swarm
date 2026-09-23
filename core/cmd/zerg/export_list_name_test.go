// export_list_name_test.go —— §一 序70（组3 §一 序12 · 缺口 `Q-012`）：帮助成文快照 ↔ 运行期**逐行对拍**。
//
// 已红的半：`zerg help export --json commands` 的 `items` **逐条同一个数重复**
// （`{"commands":"82"}` × 120）—— 消费方**指不到是哪条命令**。
//
// 本件钉住两件事（都在**进程内**跑当前源码，不碰盘上旧制品）：
//
//	① **逐条含命令名**：请求「摘要格」`commands` 时，`items` 的每一条**仍带行身份** `command`
//	   —— 请求的字段**一个不少**（§九 M6 字段只增不改）；
//	② **不画蛇添足**：请求里**已点名** `command` ⇒ 一个字段都不加（投影子集照旧）。
//
// 逐行对拍那一半（成文快照 ↔ 运行期）在 `scripts/gates/check-help-snapshot.py`（真制品现跑）。
package main

import (
	"strings"
	"testing"
)

func hasField(fields []string, want string) bool {
	for _, f := range fields {
		if f == want {
			return true
		}
	}
	return false
}

// TestHelpExportJSONListCarriesCommandName —— ① 逐条含命令名。
func TestHelpExportJSONListCarriesCommandName(t *testing.T) {
	got := helpExportFieldList([]string{"commands"})
	if !hasField(got, "commands") {
		t.Fatalf("请求的 `commands` 被吃掉了：%v", got)
	}
	if !hasField(got, "command") {
		t.Fatalf("清单面没有补上行身份 `command` ⇒ `items` 逐条指不到是哪条：%v", got)
	}
	if len(got) != 2 {
		t.Fatalf("只想补一格 `command`，实得 %v", got)
	}
	rows := helpExportRows(helpExportFacts{Path: "/tmp/x.md", DocsVersion: "v0", GeneratedAt: "1970-01-01T00:00:00Z"})
	if len(rows) == 0 {
		t.Fatal("真值行来源给了 0 行 —— 命令树读空了")
	}
	for i, r := range rows {
		if strings.TrimSpace(r["command"]) == "" {
			t.Fatalf("第 %d 行没有 `command` ⇒ 逐条指不到", i)
		}
		if r["commands"] == "" {
			t.Fatalf("第 %d 行的摘要格 `commands` 丢了（字段只增不改）", i)
		}
	}
}

// TestHelpExportJSONListKeepsRequestedOnly —— ② 点名了 `command` ⇒ 不加任何字段。
func TestHelpExportJSONListKeepsRequestedOnly(t *testing.T) {
	got := helpExportFieldList([]string{"command"})
	if len(got) != 1 || got[0] != "command" {
		t.Fatalf("已点名 `command` 时不许再加字段，实得 %v", got)
	}
	got = helpExportFieldList([]string{"command", "danger_level"})
	if len(got) != 2 {
		t.Fatalf("已点名 `command` 时不许再加字段，实得 %v", got)
	}
	// 传进去的切片**不许被就地改**（调用方可能还要用）
	src := []string{"commands"}
	_ = helpExportFieldList(src)
	if len(src) != 1 || src[0] != "commands" {
		t.Fatalf("把调用方的字段切片就地改了：%v", src)
	}
}
