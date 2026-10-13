package chatlog

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newTestStore —— 在临时目录开一枚隔离的会话存储（t.Cleanup 关索引）。
func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(t.TempDir(), OpenOptions{})
	if err != nil {
		t.Fatalf("Open 失败: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func mustAppend(t *testing.T, s *Store, ev Event) Event {
	t.Helper()
	got, err := s.Append(ev)
	if err != nil {
		t.Fatalf("Append 失败: %v", err)
	}
	return got
}

// ───────────────────── ① 追加后 JSONL 逐行可解析且不可变 ─────────────────────

// TestAppend_JSONLParseableAndImmutable —— 追加后：
//   - 每一行都是合法 JSON（ReadSession 走同一解析路径，坏行必报错）；
//   - 先追加的字节在后续追加后**逐字不变**（append-only · ✗ 不改写旧行）；
//   - seq 会话内单调递增（keyset 分页的前提）。
//
// 变异自证（能红）：把 Append 的 O_APPEND 改成 O_TRUNC（或任何回写旧行的写法）
// ⇒ 前缀断言 + 行数断言当场红。去掉 seq 递增 ⇒ 单调断言红。
func TestAppend_JSONLParseableAndImmutable(t *testing.T) {
	s := newTestStore(t)
	const sid = "sess-immutable"
	p := filepath.Join(s.Dir(), sid+JSONLSuffix)

	for i := 1; i <= 3; i++ {
		mustAppend(t, s, Event{SessionID: sid, Type: "message", Role: "user", Content: fmt.Sprintf("第 %d 条", i)})
	}
	before, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("读会话档失败: %v", err)
	}
	if len(before) == 0 {
		t.Fatal("追加 3 条后会话档仍为空")
	}

	for i := 4; i <= 5; i++ {
		mustAppend(t, s, Event{SessionID: sid, Type: "message", Role: "assistant", Content: fmt.Sprintf("第 %d 条", i)})
	}
	after, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("读会话档失败: %v", err)
	}
	if !strings.HasPrefix(string(after), string(before)) {
		t.Fatalf("追加后旧字节被改写 —— append-only 破（前 %d 字节不再逐字相同）", len(before))
	}
	if len(after) <= len(before) {
		t.Fatalf("追加后文件没变长（before=%d after=%d）", len(before), len(after))
	}

	// 逐行独立解析（✗ 不借 ReadSession 的结论）。
	lines := strings.Split(strings.TrimRight(string(after), "\n"), "\n")
	if len(lines) != 5 {
		t.Fatalf("会话档应有 5 行，实得 %d", len(lines))
	}
	for i, ln := range lines {
		var ev Event
		if err := json.Unmarshal([]byte(ln), &ev); err != nil {
			t.Fatalf("第 %d 行不是合法 JSON: %v", i+1, err)
		}
		if ev.Seq != int64(i+1) {
			t.Fatalf("第 %d 行 seq=%d（期望 %d）", i+1, ev.Seq, i+1)
		}
		if ev.SessionID != sid || ev.UUID == "" || ev.TS == 0 {
			t.Fatalf("第 %d 行缺字段（session_id/uuid/timestamp）: %+v", i+1, ev)
		}
	}

	// 真源读同口径。
	evs, err := s.ReadSession(sid)
	if err != nil {
		t.Fatalf("ReadSession 失败: %v", err)
	}
	if len(evs) != 5 {
		t.Fatalf("ReadSession 应得 5 条，实得 %d", len(evs))
	}
}

// ───────────────────── ② FTS5 命中且重建索引后结果一致 ─────────────────────

// TestSearch_FTSHitAndRebuildStable —— 三态：
//   - 建索引后 FTS5 检索**命中**；
//   - RebuildIndex 之后命中集**逐条一致**；
//   - **索引损坏**（写垃圾字节）后仍可 Open、真源 ReadSession 照读，Search 明报 ErrIndexUnavailable；
//     从损坏态再 RebuildIndex ⇒ 检索复原、命中集与原始一致。
//
// 变异自证（能红）：把 RebuildIndex 改成「从索引自身重读」⇒ 损坏态重建不出任何命中 ⇒ 红。
// 把 ReadSession 改成走索引 ⇒ 索引损坏时真源读红（这条正是「索引不是真源」的牙）。
func TestSearch_FTSHitAndRebuildStable(t *testing.T) {
	s := newTestStore(t)
	const sid = "sess-fts"
	seed := []string{
		"alpha needle content one",
		"beta haystack two",
		"gamma 会话存储 中文检索 needle",
	}
	for _, c := range seed {
		mustAppend(t, s, Event{SessionID: sid, Type: "message", Role: "user", Content: c})
	}

	hits, err := s.Search("needle", 10)
	if err != nil {
		t.Fatalf("Search 失败: %v", err)
	}
	if len(hits) != 2 {
		t.Fatalf("'needle' 应命中 2 条，实得 %d（%v）", len(hits), hitKeys(hits))
	}
	want := hitKeys(hits)

	if err := s.RebuildIndex(); err != nil {
		t.Fatalf("RebuildIndex 失败: %v", err)
	}
	hits2, err := s.Search("needle", 10)
	if err != nil {
		t.Fatalf("重建后 Search 失败: %v", err)
	}
	if got := hitKeys(hits2); !sameKeys(want, got) {
		t.Fatalf("重建索引后命中集不一致：\n 前=%v\n 后=%v", want, got)
	}

	// ── 索引损坏：真源照读，检索明报不可用 ──
	if err := s.Close(); err != nil {
		t.Fatalf("Close 失败: %v", err)
	}
	if err := os.WriteFile(filepath.Join(s.Dir(), IndexDBName), []byte("这不是一个 sqlite 库，只是一堆垃圾字节………………………………"), 0o644); err != nil {
		t.Fatalf("写坏索引库失败: %v", err)
	}
	_ = os.Remove(filepath.Join(s.Dir(), IndexDBName+"-wal"))
	_ = os.Remove(filepath.Join(s.Dir(), IndexDBName+"-shm"))

	s2, err := Open(s.Dir(), OpenOptions{})
	if err != nil {
		t.Fatalf("索引损坏后 Open 失败（不该 —— 真源不依赖索引）: %v", err)
	}
	defer s2.Close()
	if ok, _ := s2.IndexStatus(); ok {
		t.Fatal("索引已损坏，IndexStatus 竟报可用")
	}
	evs, err := s2.ReadSession(sid)
	if err != nil {
		t.Fatalf("索引损坏后真源读失败（真源 = JSONL，不该依赖索引）: %v", err)
	}
	if len(evs) != len(seed) {
		t.Fatalf("索引损坏后真源读应仍得 %d 条，实得 %d", len(seed), len(evs))
	}
	if _, err := s2.Search("needle", 10); !errors.Is(err, ErrIndexUnavailable) {
		t.Fatalf("索引损坏时 Search 应报 ErrIndexUnavailable，实得 %v", err)
	}

	if err := s2.RebuildIndex(); err != nil {
		t.Fatalf("从损坏态重建索引失败: %v", err)
	}
	hits3, err := s2.Search("needle", 10)
	if err != nil {
		t.Fatalf("重建后 Search 失败: %v", err)
	}
	if got := hitKeys(hits3); !sameKeys(want, got) {
		t.Fatalf("从损坏态重建后命中集与原始不一致：\n 前=%v\n 后=%v", want, got)
	}
}

// ───────────────────── ③ 大内容外置后行不大、sha 可核 ─────────────────────

// TestBlob_LargeContentExternalizedAndHashVerifiable ——
//   - 超大正文外置：事件列里只留 blob_ref + sha256（✗ 正文不进列 ⇒ 行不大）；
//   - sha 可核：读回外置件按 sha256 核对；被篡改 ⇒ 明报 ErrBlobCorrupt；
//   - 小正文仍内联（✗ 不无脑外置）。
//
// 变异自证（能红）：把内联上限调成永不外置（或删掉外置分支）⇒ 行长大断言红；
// 把 VerifyBlob 的 sha 比对去掉 ⇒ 篡改用例红。
func TestBlob_LargeContentExternalizedAndHashVerifiable(t *testing.T) {
	s := newTestStore(t)
	const sid = "sess-blob"
	big := strings.Repeat("大内容ABCDEF0123456789", 20000) // ≈ 440 KB
	ev := mustAppend(t, s, Event{SessionID: sid, Type: "tool_result", Role: "tool", Content: big})

	if ev.BlobRef == "" {
		t.Fatal("超大正文竟未外置（blob_ref 空）")
	}
	if ev.Content != "" {
		t.Fatalf("外置后事件列里仍带正文（len=%d）", len(ev.Content))
	}
	wantSHA := sha256.Sum256([]byte(big))
	if ev.SHA256 != hex.EncodeToString(wantSHA[:]) {
		t.Fatalf("登记的 sha256 不符：got=%s want=%s", ev.SHA256, hex.EncodeToString(wantSHA[:]))
	}

	p := filepath.Join(s.Dir(), sid+JSONLSuffix)
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("读会话档失败: %v", err)
	}
	if len(raw) > 4*1024 {
		t.Fatalf("外置后会话档仍很大（%d 字节 > 4 KiB）—— 正文没真外置", len(raw))
	}
	if strings.Contains(string(raw), big[:64]) {
		t.Fatal("行里竟含正文片段 —— 正文未外置干净")
	}

	if err := s.VerifyBlob(ev.BlobRef, ev.SHA256); err != nil {
		t.Fatalf("外置内容 sha 校验失败: %v", err)
	}
	got, err := s.Content(ev)
	if err != nil {
		t.Fatalf("Content 取回失败: %v", err)
	}
	if got != big {
		t.Fatalf("取回正文与原不符（len %d vs %d）", len(got), len(big))
	}

	// 篡改外置件 ⇒ sha 校验必须拦住（sha 可核的牙）。
	bp := filepath.Join(s.Dir(), filepath.FromSlash(ev.BlobRef))
	orig, err := os.ReadFile(bp)
	if err != nil {
		t.Fatalf("读外置件失败: %v", err)
	}
	if err := os.WriteFile(bp, append([]byte("X"), orig...), 0o644); err != nil {
		t.Fatalf("篡改外置件失败: %v", err)
	}
	if err := s.VerifyBlob(ev.BlobRef, ev.SHA256); !errors.Is(err, ErrBlobCorrupt) {
		t.Fatalf("外置件被篡改后应报 ErrBlobCorrupt，实得 %v", err)
	}

	// 小正文仍内联。
	small := mustAppend(t, s, Event{SessionID: sid, Type: "message", Content: "短正文"})
	if small.BlobRef != "" || small.Content != "短正文" {
		t.Fatalf("小正文应内联，实得 blob_ref=%q content=%q", small.BlobRef, small.Content)
	}
}

// ───────────────────── ④ keyset 分页（(session_id, seq) 游标） ─────────────────────

// TestPageAfter_KeysetPagination —— 真源侧与索引侧 keyset 翻页均无重无漏（按 seq 升序）。
func TestPageAfter_KeysetPagination(t *testing.T) {
	s := newTestStore(t)
	const sid = "sess-page"
	for i := 1; i <= 7; i++ {
		mustAppend(t, s, Event{SessionID: sid, Type: "message", Content: fmt.Sprintf("m%d", i)})
	}

	var got []int64
	var cursor int64
	for {
		page, err := s.PageAfter(sid, cursor, 3)
		if err != nil {
			t.Fatalf("PageAfter 失败: %v", err)
		}
		if len(page) == 0 {
			break
		}
		for _, ev := range page {
			got = append(got, ev.Seq)
		}
		cursor = page[len(page)-1].Seq
	}
	if len(got) != 7 {
		t.Fatalf("真源 keyset 翻页应得 7 条，实得 %d（%v）", len(got), got)
	}
	for i, v := range got {
		if v != int64(i+1) {
			t.Fatalf("真源 keyset 第 %d 个 seq=%d（期望 %d）", i, v, i+1)
		}
	}

	var idxSeqs []int64
	cursor = 0
	for {
		page, err := s.PageIndex(sid, cursor, 3)
		if err != nil {
			t.Fatalf("PageIndex 失败: %v", err)
		}
		if len(page) == 0 {
			break
		}
		for _, h := range page {
			idxSeqs = append(idxSeqs, h.Seq)
		}
		cursor = page[len(page)-1].Seq
	}
	if len(idxSeqs) != 7 {
		t.Fatalf("索引 keyset 翻页应得 7 条，实得 %d（%v）", len(idxSeqs), idxSeqs)
	}
	for i, v := range idxSeqs {
		if v != int64(i+1) {
			t.Fatalf("索引 keyset 第 %d 个 seq=%d（期望 %d）", i, v, i+1)
		}
	}
}

// ───────────────────── helpers ─────────────────────

func hitKeys(hits []Hit) []string {
	out := make([]string, 0, len(hits))
	for _, h := range hits {
		out = append(out, fmt.Sprintf("%s#%d", h.SessionID, h.Seq))
	}
	return out
}

func sameKeys(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	set := map[string]bool{}
	for _, k := range a {
		set[k] = true
	}
	for _, k := range b {
		if !set[k] {
			return false
		}
	}
	return true
}
