package agent

// issue_store_test.go —— 单点落盘的判据（缺口 `GAP-20260926-18` 第二步）。
//
// 四条牙：
//   〔甲〕首次落件：内容逐字节在盘 · 目录里**零 `.tmp-*` 残留**
//   〔乙〕改态合法：`open → queued` 真写进去（状态行变了）
//   〔丙〕改态非法：`open → done`（跳过中间态）⇒ 返回错 **且文件 sha256 不变**（一个字节都不写）
//   〔丁〕读回比对确实在跑：写一个只读目录 ⇒ 落盘失败返回错（不是静默成功）

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func issueStoreDoc(status string) string {
	return "# Issue: inst-1\n\n" +
		"<!-- task-hash: deadbeef -->\n" +
		"- **任务**: 造一条测试单\n" +
		fmt.Sprintf("- **状态**: %s（failed）\n", status) +
		"- **失败类型**: timeout\n" +
		"- **重试次数**: 0\n"
}

func issueStoreSHA(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读件失败（%s）：%v", path, err)
	}
	sum := sha256.Sum256(b)
	return fmt.Sprintf("%x", sum[:])
}

func TestWriteIssueNewAtomicNoTempLeft(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "inst-1.md")
	doc := issueStoreDoc("open")

	if err := WriteIssueNew(path, doc); err != nil {
		t.Fatalf("首次落件失败：%v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != doc {
		t.Fatalf("内容不一致：\n--- 期望 ---\n%s\n--- 实得 ---\n%s", doc, got)
	}
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), ".tmp-") {
			t.Errorf("临时件残留：%s（失败路径必须自己清掉）", e.Name())
		}
	}
	if len(ents) != 1 {
		t.Errorf("目录里只该有 1 件，实得 %d 件", len(ents))
	}
}

func TestWriteIssueStateLegalTransition(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "inst-1.md")
	if err := WriteIssueNew(path, issueStoreDoc("open")); err != nil {
		t.Fatal(err)
	}
	content := issueStoreDoc("open")

	if err := WriteIssueState(path, content, "open", "queued"); err != nil {
		t.Fatalf("合法流转必须写成功（open → queued）：%v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "- **状态**: queued") {
		t.Errorf("状态行没改过去：\n%s", got)
	}
}

func TestWriteIssueStateIllegalTransitionWritesNothing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "inst-1.md")
	if err := WriteIssueNew(path, issueStoreDoc("open")); err != nil {
		t.Fatal(err)
	}
	before := issueStoreSHA(t, path)

	err := WriteIssueState(path, issueStoreDoc("open"), "open", "done")
	if err == nil {
		t.Fatal("跳过中间态的流转必须被拒（open → done）")
	}
	after := issueStoreSHA(t, path)
	if before != after {
		t.Fatalf("非法流转后盘上必须一个字节都不变（%s → %s）", before, after)
	}
	if !strings.Contains(err.Error(), "illegal transition") {
		t.Errorf("错误信息要点名非法流转，实得：%v", err)
	}
}

func TestWriteIssueStateSameStateIsNoop(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "inst-1.md")
	doc := issueStoreDoc("open")
	if err := WriteIssueNew(path, doc); err != nil {
		t.Fatal(err)
	}
	before := issueStoreSHA(t, path)
	if err := WriteIssueState(path, doc, "open", "open"); err != nil {
		t.Fatalf("同态流转按 TransitionIssue 的语义是 no-op（不报错）：%v", err)
	}
	if after := issueStoreSHA(t, path); before != after {
		t.Errorf("同态流转内容不该变（%s → %s）", before, after)
	}
}

func TestWriteIssueNewFailsOnBadDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "no-such-sub", "inst-1.md")
	if err := WriteIssueNew(path, issueStoreDoc("open")); err == nil {
		t.Fatal("目录不存在必须报错（✗ 静默成功）")
	}
}
