package chat

// chat_docsearch_pinned_test.go —— `doc_search` 的「当前版目录无发布件 ⇒ fail-closed 明报」成对正负控
// （缺口 `GAP-20260927-121` · 夹具复用 `chat_docs_root_test.go` 的 isolateDocsRoots / writeDocsFixture）。
//
// 正控：当前版目录（`v<version.Tag>`）**含发布件** ⇒ doc_search 照常给结论（不报错）；
// 负控：当前版目录**无发布件**（只在途设计稿 / 收版中间态）⇒ doc_search **不给结论**（返回错误）
//       + 逐字明报「当前版目录 … 无发布件，故本消费者取不到」。
//       ★ 变异：删掉 chat_tool_docsearch.go 里那道 fail-closed 闸 ⇒ 负控必红。

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/version"
)

// 正控：含发布件的当前版目录 ⇒ 不触发 fail-closed（照常出结果）。
func TestDocSearch_PinnedCurrentVersionWithArtifactOK(t *testing.T) {
	_, alt := isolateDocsRoots(t)
	writeDocsFixture(t, alt, version.Tag, "MARKER-gap121-ok", 3)

	if _, err := docSearch(map[string]any{"query": "MARKER-gap121-ok"}, ""); err != nil {
		t.Fatalf("当前版目录含发布件时 doc_search 不该报错，实得: %v", err)
	}
}

// 负控：当前版目录无发布件 ⇒ doc_search fail-closed 明报（缺口 GAP-20260927-121）。
func TestDocSearch_PinnedCurrentVersionWithoutArtifactFailsClosed(t *testing.T) {
	_, alt := isolateDocsRoots(t)
	// 只在途件、**不放**发布件（使用-虫族指南-*.md）。
	dir := filepath.Join(alt, "项目文档", version.Tag)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "INDEX.md"), []byte("# x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := docSearch(map[string]any{"query": "任何关键词-gap121"}, ""); err == nil {
		t.Fatal("当前版目录无发布件时 doc_search 必须 fail-closed（不给结论），✗ 不许静默回「无命中」或结果")
	} else if !strings.Contains(err.Error(), "无发布件，故本消费者取不到") {
		t.Fatalf("缺件错误要逐字含「无发布件，故本消费者取不到」，实得: %v", err)
	}
}
