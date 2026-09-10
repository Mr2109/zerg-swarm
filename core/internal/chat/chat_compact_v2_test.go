// chat_compact_v2_test.go — 丙批 §4.2 压缩单一入口单测（2026-09-10）
// 覆盖: 阈值边界（不足不压/到达触发）/ LLMLingua-2 优先与失败回退 /
//       冷却 60→300→900 递增与冷却内不重试 / force 旁路一次 / 3 次硬熔断后不再尝试 /
//       journal 落盘字段正确 / 摘要尾部召回指针文本正确 / 上下文超限错误识别。
// 状态文件隔离: ZERG_STATE_DIR 指向 t.TempDir()；库文件由 newTestStore 隔离。

package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// newCompactStore — 隔离的状态目录 + 隔离的对话库
func newCompactStore(t *testing.T) (*ChatStore, string) {
	t.Helper()
	stateDir := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", stateDir)
	return newTestStore(t), stateDir
}

// useCompactClock — 接管冷却时钟（返回可推进的当前时间指针）
func useCompactClock(t *testing.T, start time.Time) *time.Time {
	t.Helper()
	orig := compactNow
	cur := start
	compactNow = func() time.Time { return cur }
	t.Cleanup(func() { compactNow = orig })
	return &cur
}

// compactSeed — 批量插 n 条同内容消息（时间戳递增——保证顺序确定），返回消息 id
func compactSeed(t *testing.T, st *ChatStore, sid string, n int, content string) []int64 {
	t.Helper()
	ids := make([]int64, 0, n)
	for i := 0; i < n; i++ {
		id, err := st.AddMessage(&Message{
			SessionID: sid, Role: ssRoleFor(i), Content: content, Active: true,
			Timestamp: 1757500000.0 + float64(i),
		})
		if err != nil {
			t.Fatalf("插消息失败: %v", err)
		}
		ids = append(ids, id)
	}
	return ids
}

// compactActive — 取活动消息（MaybeCompact 的真实入参形态）
func compactActive(t *testing.T, st *ChatStore, sid string) []*Message {
	t.Helper()
	msgs, err := st.GetActiveMessages(sid)
	if err != nil {
		t.Fatalf("取活动消息失败: %v", err)
	}
	return msgs
}

// compactFindSummary — 找会话里的摘要消息（【历史摘要】开头）
func compactFindSummary(t *testing.T, st *ChatStore, sid string) *Message {
	t.Helper()
	msgs, err := st.ListMessages(sid)
	if err != nil {
		t.Fatalf("列消息失败: %v", err)
	}
	for _, m := range msgs {
		if strings.HasPrefix(m.Content, "【历史摘要】") {
			return m
		}
	}
	return nil
}

// readCompactJournal — 读 journal 全部记录
func readCompactJournal(t *testing.T, stateDir string) []compactJournalRecord {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(stateDir, "compact_journal.jsonl"))
	if err != nil {
		t.Fatalf("读 journal 失败: %v", err)
	}
	var out []compactJournalRecord
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var r compactJournalRecord
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatalf("journal 行解析失败: %v (%s)", err, line)
		}
		out = append(out, r)
	}
	return out
}

// approxEq — 浮点近似比较（冷却到期时间）
func approxEq(a, b float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < 0.01
}

// ── 阈值 ─────────────────────────────────────────────────────────────

// TestCompactThresholdBoundary — 不足不压（消息数 < 30 / token 未达阈值均不压）
func TestCompactThresholdBoundary(t *testing.T) {
	st, _ := newCompactStore(t)
	se, err := st.CreateSession("gemma-12B", "desktop", "", "阈值边界会话")
	if err != nil {
		t.Fatal(err)
	}
	longCN := strings.Repeat("测", 300) // ≈428 token/条

	// 情形 A: 消息数不足（10 条——token 虽超阈值也不压）
	compactSeed(t, st, se.ID, 10, longCN)
	sumCalls := 0
	did, err := st.MaybeCompact(context.Background(), se.ID, "gemma-12B", compactActive(t, st, se.ID),
		nil, func(context.Context, []map[string]any) (string, error) { sumCalls++; return "摘要", nil }, false)
	if err != nil || did {
		t.Fatalf("消息数不足不应压缩: did=%v err=%v", did, err)
	}
	if sumCalls != 0 {
		t.Fatalf("消息数不足不应调用摘要函数（实调 %d 次）", sumCalls)
	}
	if f := compactGetFail(se.ID); f != nil {
		t.Fatalf("未压缩不应产生冷却态: %+v", f)
	}

	// 情形 B: 消息数够但 token 未达阈值（30 条极短消息）
	se2, _ := st.CreateSession("gemma-12B", "desktop", "", "低 token 会话")
	compactSeed(t, st, se2.ID, 30, "hi")
	did2, err2 := st.MaybeCompact(context.Background(), se2.ID, "gemma-12B", compactActive(t, st, se2.ID),
		nil, func(context.Context, []map[string]any) (string, error) { sumCalls++; return "摘要", nil }, false)
	if err2 != nil || did2 {
		t.Fatalf("token 未达阈值不应压缩: did=%v err=%v", did2, err2)
	}
	if sumCalls != 0 {
		t.Fatalf("token 未达阈值不应调用摘要函数（实调 %d 次）", sumCalls)
	}
	t.Logf("✓ 阈值边界: 消息数不足(10) 与 token 不足(30×hi) 均不压缩")
}

// TestCompactThresholdTrigger — 消息数 ≥30 且 token 超阈值 → 触发压缩
func TestCompactThresholdTrigger(t *testing.T) {
	st, _ := newCompactStore(t)
	se, err := st.CreateSession("gemma-12B", "desktop", "", "达阈值会话")
	if err != nil {
		t.Fatal(err)
	}
	compactSeed(t, st, se.ID, 30, strings.Repeat("测", 300))
	sumCalls := 0
	did, err := st.MaybeCompact(context.Background(), se.ID, "gemma-12B", compactActive(t, st, se.ID),
		nil, func(context.Context, []map[string]any) (string, error) { sumCalls++; return "达阈值摘要", nil }, false)
	if err != nil || !did {
		t.Fatalf("达阈值应触发压缩: did=%v err=%v", did, err)
	}
	if sumCalls != 1 {
		t.Fatalf("应恰好调用摘要函数 1 次（实 %d）", sumCalls)
	}
	if compactFindSummary(t, st, se.ID) == nil {
		t.Fatalf("压缩后应存在摘要消息")
	}
	// 中间段 = 索引 3..9（首 3 + 尾 20）——被压缩消息应失活
	active := compactActive(t, st, se.ID)
	if len(active) != 30-7+1 { // 30 - 7 被归档 + 1 摘要
		t.Fatalf("压缩后活动消息数应为 %d，实得 %d", 30-7+1, len(active))
	}
	t.Logf("✓ 阈值到达: 30 条超阈值 → 压缩中间段 7 条，活动窗口 %d 条", len(active))
}

// ── 算法选路 ─────────────────────────────────────────────────────────

// TestCompactLinguaPriority — LLMLingua-2 可用 → 优先，摘要函数不被调用
func TestCompactLinguaPriority(t *testing.T) {
	st, _ := newCompactStore(t)
	se, err := st.CreateSession("gemma-12B", "desktop", "", "LLMLingua 优先会话")
	if err != nil {
		t.Fatal(err)
	}
	compactSeed(t, st, se.ID, 30, strings.Repeat("测", 300))

	linguaCalls, sumCalls := 0, 0
	lingua := func(_ context.Context, text string) (string, error) {
		linguaCalls++
		if !strings.Contains(text, "测") {
			return "", errors.New("输入文本异常")
		}
		return "LLMLingua 删除式摘要正文", nil
	}
	sum := func(context.Context, []map[string]any) (string, error) { sumCalls++; return "不应被调用", nil }

	did, err := st.MaybeCompact(context.Background(), se.ID, "gemma-12B", compactActive(t, st, se.ID), lingua, sum, false)
	if err != nil || !did {
		t.Fatalf("LLMLingua 可用应压缩成功: did=%v err=%v", did, err)
	}
	if linguaCalls != 1 {
		t.Fatalf("LLMLingua 应被调用 1 次（实 %d）", linguaCalls)
	}
	if sumCalls != 0 {
		t.Fatalf("LLMLingua 成功时摘要函数不应被调用（实 %d 次）", sumCalls)
	}
	if s := compactFindSummary(t, st, se.ID); s == nil || !strings.Contains(s.Content, "LLMLingua 删除式摘要正文") {
		t.Fatalf("摘要应来自 LLMLingua: %+v", s)
	}
	t.Logf("✓ 选路: LLMLingua-2 成功 → 摘要函数零调用")
}

// TestCompactLinguaFallbackToLLM — LLMLingua 失败/未加载/返回空 → 回退 LLM 摘要
func TestCompactLinguaFallbackToLLM(t *testing.T) {
	cases := []struct {
		name   string
		lingua CompactFn
	}{
		{"未加载(nil)", nil},
		{"调用报错", func(context.Context, string) (string, error) { return "", errors.New("onnx 未加载") }},
		{"返回空串", func(context.Context, string) (string, error) { return "   ", nil }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			st, _ := newCompactStore(t)
			se, err := st.CreateSession("gemma-12B", "desktop", "", "回退会话-"+tc.name)
			if err != nil {
				t.Fatal(err)
			}
			compactSeed(t, st, se.ID, 30, strings.Repeat("测", 300))
			sumCalls := 0
			sum := func(_ context.Context, msgs []map[string]any) (string, error) {
				sumCalls++
				want := 30 - CompactProtectFirstN - CompactProtectLastN // 中间段 7 条
				if len(msgs) != want {
					return "", fmt.Errorf("摘要输入条数应为 %d，实得 %d", want, len(msgs))
				}
				return "LLM 结构化摘要正文", nil
			}
			did, err := st.MaybeCompact(context.Background(), se.ID, "gemma-12B", compactActive(t, st, se.ID), tc.lingua, sum, false)
			if err != nil || !did {
				t.Fatalf("回退应压缩成功: did=%v err=%v", did, err)
			}
			if sumCalls != 1 {
				t.Fatalf("回退应调用摘要函数 1 次（实 %d）", sumCalls)
			}
			if s := compactFindSummary(t, st, se.ID); s == nil || !strings.Contains(s.Content, "LLM 结构化摘要正文") {
				t.Fatalf("摘要应来自 LLM 回退: %+v", s)
			}
		})
	}
	t.Logf("✓ 回退: nil/报错/空串 三种情形均回退 LLM 摘要成功")
}

// ── 冷却 ─────────────────────────────────────────────────────────────

// TestCompactCooldownEscalation — 连续失败冷却 60→300→900 递增，冷却内不重试
func TestCompactCooldownEscalation(t *testing.T) {
	st, _ := newCompactStore(t)
	se, err := st.CreateSession("gemma-12B", "desktop", "", "冷却递增会话")
	if err != nil {
		t.Fatal(err)
	}
	compactSeed(t, st, se.ID, 30, strings.Repeat("测", 300))

	t0 := time.Unix(1_800_000_000, 0)
	cur := useCompactClock(t, t0)

	sumCalls := 0
	failSum := func(context.Context, []map[string]any) (string, error) {
		sumCalls++
		return "", errors.New("摘要模型 500")
	}
	call := func(force bool) (bool, error) {
		return st.MaybeCompact(context.Background(), se.ID, "gemma-12B", compactActive(t, st, se.ID), nil, failSum, force)
	}

	// 第 1 次失败 → 冷却 60s
	did, err := call(false)
	if did || err == nil {
		t.Fatalf("第 1 次应失败: did=%v err=%v", did, err)
	}
	if f := compactGetFail(se.ID); f == nil || f.Streak != 1 || !approxEq(f.Until, float64(t0.Unix())+60) {
		t.Fatalf("第 1 次失败态应为 streak=1/until=+60: %+v", f)
	}

	// 冷却内立即再试 → 不重试（摘要函数不再被调用）
	before := sumCalls
	did, err = call(false)
	if did || err != nil {
		t.Fatalf("冷却内应静默跳过: did=%v err=%v", did, err)
	}
	if sumCalls != before {
		t.Fatalf("冷却内不应重试（摘要调用 %d→%d）", before, sumCalls)
	}

	// 冷却到期（+61s）→ 第 2 次失败 → 冷却 300s
	*cur = cur.Add(61 * time.Second)
	if did, err = call(false); did || err == nil {
		t.Fatalf("冷却到期后第 2 次应失败: did=%v err=%v", did, err)
	}
	if f := compactGetFail(se.ID); f == nil || f.Streak != 2 || !approxEq(f.Until, float64(cur.Unix())+300) {
		t.Fatalf("第 2 次失败态应为 streak=2/until=+300: %+v", f)
	}

	// 到期（+301s）→ 第 3 次失败 → 冷却 900s + 硬熔断
	*cur = cur.Add(301 * time.Second)
	if did, err = call(false); did || err == nil {
		t.Fatalf("冷却到期后第 3 次应失败: did=%v err=%v", did, err)
	}
	f3 := compactGetFail(se.ID)
	if f3 == nil || f3.Streak != 3 || !f3.Disabled || !approxEq(f3.Until, float64(cur.Unix())+900) {
		t.Fatalf("第 3 次失败态应为 streak=3/disabled/until=+900: %+v", f3)
	}

	// 到期（+901s）→ 恢复可试（第 4 次失败——streak=4，冷却封顶 900）
	*cur = cur.Add(901 * time.Second)
	if did, err = call(false); did || err == nil {
		t.Fatalf("冷却到期后应恢复可试: did=%v err=%v", did, err)
	}
	if f := compactGetFail(se.ID); f == nil || f.Streak != 4 || !approxEq(f.Until, float64(cur.Unix())+900) {
		t.Fatalf("第 4 次失败态应为 streak=4/until=+900（封顶）: %+v", f)
	}
	t.Logf("✓ 冷却: 60→300→900 递增，冷却内零重试，到期恢复，streak=%d", compactGetFail(se.ID).Streak)
}

// TestCompactHardTrip — 连续 3 次失败硬熔断后不再尝试；手动重置可恢复
func TestCompactHardTrip(t *testing.T) {
	st, _ := newCompactStore(t)
	se, err := st.CreateSession("gemma-12B", "desktop", "", "硬熔断会话")
	if err != nil {
		t.Fatal(err)
	}
	compactSeed(t, st, se.ID, 30, strings.Repeat("测", 300))

	t0 := time.Unix(1_800_000_000, 0)
	cur := useCompactClock(t, t0)
	sumCalls := 0
	failSum := func(context.Context, []map[string]any) (string, error) {
		sumCalls++
		return "", errors.New("摘要模型 429")
	}
	call := func(force bool) (bool, error) {
		return st.MaybeCompact(context.Background(), se.ID, "gemma-12B", compactActive(t, st, se.ID), nil, failSum, force)
	}

	// 累计 3 次失败（每次跨过冷却）→ 硬熔断
	for i := 0; i < 3; i++ {
		if _, err := call(false); err == nil {
			t.Fatalf("第 %d 次应失败", i+1)
		}
		*cur = cur.Add(time.Duration(compactCooldownFor(i+1)) + time.Second)
	}
	// 回到第 3 次失败冷却窗口内（未到期）验证硬熔断
	*cur = cur.Add(-30 * time.Second)
	if f := compactGetFail(se.ID); f == nil || !f.Disabled || f.Streak != 3 {
		t.Fatalf("应为硬熔断态: %+v", f)
	}
	// 熔断期间（未到期）不再尝试
	before := sumCalls
	if did, err := call(false); did || err != nil {
		t.Fatalf("硬熔断期间应静默跳过: did=%v err=%v", did, err)
	}
	if sumCalls != before {
		t.Fatalf("硬熔断期间不应尝试（摘要调用 %d→%d）", before, sumCalls)
	}

	// 手动重置 → 失败态清空 → 可再试
	ResetCompactSession(se.ID)
	if f := compactGetFail(se.ID); f != nil {
		t.Fatalf("手动重置后应无失败态: %+v", f)
	}
	if _, err := call(false); err == nil {
		t.Fatalf("重置后应可再次尝试（此摘要桩必然失败）")
	}
	t.Logf("✓ 硬熔断: 3 次失败停用（期间零尝试）→ 手动重置恢复尝试")
}

// TestCompactForceBypass — force 旁路一次（无视冷却与硬熔断），成功后失败态清零
func TestCompactForceBypass(t *testing.T) {
	st, _ := newCompactStore(t)
	se, err := st.CreateSession("gemma-12B", "desktop", "", "force 旁路会话")
	if err != nil {
		t.Fatal(err)
	}
	compactSeed(t, st, se.ID, 30, strings.Repeat("测", 300))

	t0 := time.Unix(1_800_000_000, 0)
	useCompactClock(t, t0)

	failing := true
	sumCalls := 0
	sum := func(context.Context, []map[string]any) (string, error) {
		sumCalls++
		if failing {
			return "", errors.New("摘要模型 500")
		}
		return "手动触发摘要正文", nil
	}
	call := func(force bool) (bool, error) {
		return st.MaybeCompact(context.Background(), se.ID, "gemma-12B", compactActive(t, st, se.ID), nil, sum, force)
	}

	// 制造一次失败 → 进入 60s 冷却
	if did, err := call(false); did || err == nil {
		t.Fatalf("首次应失败: did=%v err=%v", did, err)
	}
	// 冷却内普通调用 → 跳过
	before := sumCalls
	if did, err := call(false); did || err != nil {
		t.Fatalf("冷却内普通调用应跳过: did=%v err=%v", did, err)
	}
	if sumCalls != before {
		t.Fatalf("冷却内普通调用不应尝试")
	}
	// force=true 旁路（摘要现可成功）→ 压缩成功且失败态清零
	failing = false
	did, err := call(true)
	if err != nil || !did {
		t.Fatalf("force 应旁路冷却并成功: did=%v err=%v", did, err)
	}
	if f := compactGetFail(se.ID); f != nil {
		t.Fatalf("成功后失败态应清零: %+v", f)
	}
	if s := compactFindSummary(t, st, se.ID); s == nil || !strings.Contains(s.Content, "手动触发摘要正文") {
		t.Fatalf("force 触发的摘要应落盘: %+v", s)
	}
	t.Logf("✓ force: 冷却内旁路一次成功，失败态清零")
}

// ── 召回指针 / journal ───────────────────────────────────────────────

// TestCompactJournalAndPointer — journal 落盘字段正确 + 摘要尾部召回指针文本正确
func TestCompactJournalAndPointer(t *testing.T) {
	st, stateDir := newCompactStore(t)
	se, err := st.CreateSession("gemma-12B", "desktop", "", "召回指针会话")
	if err != nil {
		t.Fatal(err)
	}
	ids := compactSeed(t, st, se.ID, 30, strings.Repeat("测", 300))

	t0 := time.Unix(1_800_000_000, 0)
	useCompactClock(t, t0)

	did, err := st.MaybeCompact(context.Background(), se.ID, "gemma-12B", compactActive(t, st, se.ID),
		nil, func(context.Context, []map[string]any) (string, error) { return "召回指针摘要正文", nil }, false)
	if err != nil || !did {
		t.Fatalf("压缩应成功: did=%v err=%v", did, err)
	}

	wantFrom, wantTo := ids[CompactProtectFirstN], ids[len(ids)-CompactProtectLastN-1]

	// 摘要尾部指针
	sumMsg := compactFindSummary(t, st, se.ID)
	if sumMsg == nil {
		t.Fatalf("应有摘要消息")
	}
	wantPtr := fmt.Sprintf("(可搜回:session_search(session_id=%s, around_id=%d))", se.ID, wantFrom)
	if !strings.HasPrefix(sumMsg.Content, "【历史摘要】") {
		t.Fatalf("摘要应带【历史摘要】前缀: %s", sumMsg.Content)
	}
	if !strings.HasSuffix(strings.TrimSpace(sumMsg.Content), wantPtr) {
		t.Fatalf("摘要尾部指针不正确\nwant 后缀: %s\n实得: %s", wantPtr, sumMsg.Content)
	}
	if !strings.Contains(sumMsg.Content, "召回指针摘要正文") {
		t.Fatalf("摘要正文缺失: %s", sumMsg.Content)
	}

	// journal 字段
	recs := readCompactJournal(t, stateDir)
	if len(recs) != 1 {
		t.Fatalf("journal 应有 1 条记录（实 %d）", len(recs))
	}
	r := recs[0]
	if r.SessionID != se.ID {
		t.Fatalf("journal session_id 错误: %s ≠ %s", r.SessionID, se.ID)
	}
	if r.FromID != wantFrom || r.ToID != wantTo {
		t.Fatalf("journal from_id/to_id 错误: got %d..%d want %d..%d", r.FromID, r.ToID, wantFrom, wantTo)
	}
	if r.SummaryID != sumMsg.ID {
		t.Fatalf("journal summary_id 应指向摘要消息: got %d want %d", r.SummaryID, sumMsg.ID)
	}
	if !approxEq(r.At, float64(t0.Unix())) {
		t.Fatalf("journal at 错误: %f ≠ %d", r.At, t0.Unix())
	}

	// 软归档语义: 被压缩消息 active=0/compacted=1（可搜回）
	all, _ := st.ListMessages(se.ID)
	archived := 0
	for _, m := range all {
		if m.ID >= wantFrom && m.ID <= wantTo {
			if m.Active || !m.Compacted {
				t.Fatalf("被压缩消息应 active=0/compacted=1: id=%d active=%v compacted=%v", m.ID, m.Active, m.Compacted)
			}
			archived++
		}
	}
	if archived != int(wantTo-wantFrom+1) {
		t.Fatalf("软归档条数错误: %d", archived)
	}

	// session_search 可用指针读回该段（集成验证——指针语义）
	out, err := SessionSearchExecute(map[string]any{"session_id": se.ID, "around_id": float64(wantFrom)}, st)
	if err != nil {
		t.Fatalf("session_search 读回失败: %v", err)
	}
	if ids := ssExtractIDs(out); len(ids) == 0 || ids[0] > wantFrom {
		t.Fatalf("指针 around_id=%d 应能读回该段: %s", wantFrom, out)
	}
	t.Logf("✓ journal+指针: from=%d to=%d summary=#%d at=%d；指针可经 session_search 读回",
		r.FromID, r.ToID, r.SummaryID, t0.Unix())
}

// TestCompactContextLimitErrorDetect — provider 上下文超限错误识别（供调用方决定 force）
func TestCompactContextLimitErrorDetect(t *testing.T) {
	hits := []error{
		errors.New("This model's maximum context length is 8192 tokens"),
		errors.New("context_length_exceeded"),
		errors.New("请求超过上下文窗口上限"),
	}
	for _, e := range hits {
		if !IsContextLimitError(e) {
			t.Fatalf("应识别为上下文超限: %v", e)
		}
	}
	if IsContextLimitError(errors.New("connection refused")) || IsContextLimitError(nil) {
		t.Fatalf("普通错误/空错误不应识别为上下文超限")
	}
	t.Logf("✓ 上下文超限识别: 3 命中 / 2 未命中")
}
