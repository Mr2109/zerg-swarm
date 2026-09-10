// chat_compact_acceptance_test.go — 丙批 §4.2 验收 ②③（2026-09-10）
//
// ② 压缩后旧消息可被 session_search 找回，摘要里指针可用
// ③ 摘要失败进入冷却，冷却内不再重试，force 可恢复一次
//
// 运行：cd core && go test ./internal/chat/ -run BatchC -v

package chat

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"zerg/core/internal/statepath"
)

// 造足够大的历史以越过阈值（触发条件：≥30 条消息 AND 估算 token > 8000）
func bigHistory(t *testing.T, st *ChatStore, sid string) {
	t.Helper()
	blob := strings.Repeat("这是一段用于触发压缩的长文本，包含足够多的字符以越过压缩阈值。", 30) // ~1000 字
	contents := make([]string, 0, 40)
	for i := 0; i < 40; i++ {
		contents = append(contents, blob+" 第 "+strconv.Itoa(i+1)+" 段")
	}
	ssAddMsgs(t, st, sid, 1757700000.0, contents...)
}

// ② 压缩 → 摘要带召回指针；被压缩的旧消息仍可被 session_search 找回；journal 落盘
func TestBatchC_Acceptance2_CompactRecallPointer(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", stateDir)
	st := newTestStore(t)
	se, err := st.CreateSession("example-35b-v2", "desktop", "", "丙2召回指针验收")
	if err != nil {
		t.Fatal(err)
	}
	bigHistory(t, st, se.ID)
	msgs, err := st.GetActiveMessages(se.ID)
	if err != nil {
		t.Fatal(err)
	}

	sum := func(ctx context.Context, m []map[string]any) (string, error) {
		return "【历史摘要】丙批验收②：中间段已压缩为摘要", nil
	}
	did, cerr := st.MaybeCompact(context.Background(), se.ID, "example-35b-v2", msgs, nil, sum, false)
	if cerr != nil {
		t.Fatalf("压缩不应失败: %v", cerr)
	}
	if !did {
		t.Fatalf("历史已越过阈值（%d 条），应执行压缩", len(msgs))
	}

	// 摘要消息里应带召回指针
	after, _ := st.GetActiveMessages(se.ID)
	pointerOK := false
	for _, m := range after {
		if strings.Contains(m.Content, "可搜回:session_search(session_id=") {
			pointerOK = true
		}
	}
	if !pointerOK {
		t.Fatalf("摘要应带召回指针（可搜回:session_search(...)）")
	}

	// 被压缩（active=0）的旧消息仍可被检索到
	out, err := SessionSearchToolExec(map[string]any{"query": "触发压缩的长文本"})
	if err != nil {
		t.Fatalf("session_search 失败: %v", err)
	}
	if !strings.Contains(out, "触发压缩的长文本") {
		t.Fatalf("压缩后的旧消息应仍可搜回，实际输出:\n%s", out)
	}

	// journal 落盘
	jp := filepath.Join(stateDir, "compact_journal.jsonl")
	raw, err := os.ReadFile(jp)
	if err != nil {
		t.Fatalf("压缩 journal 应落盘（%s）: %v", jp, err)
	}
	line := strings.TrimSpace(strings.Split(strings.TrimSpace(string(raw)), "\n")[0])
	var rec map[string]any
	if err := json.Unmarshal([]byte(line), &rec); err != nil {
		t.Fatalf("journal 行应是 JSON: %v（%s）", err, line)
	}
	if rec["session_id"] != se.ID {
		t.Fatalf("journal 会话 id 不符: %v != %s", rec["session_id"], se.ID)
	}
	for _, k := range []string{"from_id", "to_id", "summary_id", "at"} {
		if _, ok := rec[k]; !ok {
			t.Fatalf("journal 缺字段 %s: %s", k, line)
		}
	}
	t.Logf("✓ 验收②：压缩 %d 条 → 摘要带指针；旧消息仍可搜回；journal=%s", len(msgs), line)
}

// ③ 摘要连续失败 → 冷却生效（冷却内不再重试）；force 可旁路一次
func TestBatchC_Acceptance3_CooldownBlocksThenForce(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", stateDir)
	st := newTestStore(t)
	se, err := st.CreateSession("example-35b-v2", "desktop", "", "丙2冷却验收")
	if err != nil {
		t.Fatal(err)
	}
	bigHistory(t, st, se.ID)
	msgs, _ := st.GetActiveMessages(se.ID)

	failing := func(ctx context.Context, m []map[string]any) (string, error) {
		return "", context.DeadlineExceeded
	}
	// 第一次：尝试 → 失败（返回 err）
	did, cerr := st.MaybeCompact(context.Background(), se.ID, "example-35b-v2", msgs, nil, failing, false)
	if did || cerr == nil {
		t.Fatalf("首次失败应返回 err 且 did=false，实际 did=%v err=%v", did, cerr)
	}
	// 冷却内：直接跳过（不重试、无 err）
	did2, cerr2 := st.MaybeCompact(context.Background(), se.ID, "example-35b-v2", msgs, nil, failing, false)
	if did2 || cerr2 != nil {
		t.Fatalf("冷却内应静默跳过（did=false, err=nil），实际 did=%v err=%v", did2, cerr2)
	}
	// force：无视冷却再试一次（仍失败——但证明"能恢复尝试"）
	_, cerr3 := st.MaybeCompact(context.Background(), se.ID, "example-35b-v2", msgs, nil, failing, true)
	if cerr3 == nil {
		t.Fatal("force 旁路后应再次尝试（摘要函数仍失败 → 应有 err）")
	}
	// 冷却文件落盘
	if _, err := os.Stat(statepath.File("compact_cooldown.json")); err != nil {
		t.Fatalf("冷却应落盘: %v", err)
	}
	t.Log("✓ 验收③：首次失败入冷却；冷却内静默跳过；force 可旁路重试；状态已落盘")
}
