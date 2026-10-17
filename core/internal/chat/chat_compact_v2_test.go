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
	"log"
	"math/rand"
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

// TestCompactThresholdBoundary — 不足不压（条数 < 极低拦截线 6 / token 未达阈值 均不压）
func TestCompactThresholdBoundary(t *testing.T) {
	st, _ := newCompactStore(t)
	se, err := st.CreateSession("gemma-12B", "desktop", "", "阈值边界会话")
	if err != nil {
		t.Fatal(err)
	}
	longCN := strings.Repeat("测", 300) // ≈428 token/条

	// 情形 A: 条数低于极低拦截线（5 条——token 虽超阈值也不压；GAP-20261018-02 后的口径：
	// 拦截线从 30 降到 6，只防「刚开局就压」，6 条以上一律交给 token 判）
	compactSeed(t, st, se.ID, 5, longCN)
	sumCalls := 0
	did, err := st.MaybeCompact(context.Background(), se.ID, "gemma-12B", compactActive(t, st, se.ID),
		nil, func(context.Context, []map[string]any) (string, error) { sumCalls++; return "摘要", nil }, false)
	if err != nil || did {
		t.Fatalf("条数低于极低拦截线不应压缩: did=%v err=%v", did, err)
	}
	if sumCalls != 0 {
		t.Fatalf("条数不足不应调用摘要函数（实调 %d 次）", sumCalls)
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
	t.Logf("✓ 阈值边界: 条数不足(5，< 极低拦截线 %d) 与 token 不足(30×hi) 均不压缩", CompactHardMinMessages)
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
	compactJitterRand = func() float64 { return 1 } // 钉住抖动（=base）——本用例断言精确冷却时长
	t.Cleanup(func() { compactJitterRand = rand.Float64 })
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
	compactJitterRand = func() float64 { return 1 } // 钉住抖动（=base）——本用例断言精确冷却时长
	t.Cleanup(func() { compactJitterRand = rand.Float64 })
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

// TestCompactCooldownFullJitter — 冷却含 full jitter：random(0, base)，且 base 随失败次数递增
func TestCompactCooldownFullJitter(t *testing.T) {
	defer func() { compactJitterRand = rand.Float64 }()
	// 抖动因子 0.5 → 冷却 = base/2
	compactJitterRand = func() float64 { return 0.5 }
	if got := compactCooldownFor(1); got != 30*time.Second {
		t.Fatalf("streak=1 抖动 0.5 → 期望 30s(60×0.5)，实际 %v", got)
	}
	if got := compactCooldownFor(2); got != 150*time.Second {
		t.Fatalf("streak=2 抖动 0.5 → 期望 150s(300×0.5)，实际 %v", got)
	}
	// 抖动因子 0 → 冷却为 0（full jitter 允许立即重试）；因子 1 → 等于 base
	compactJitterRand = func() float64 { return 0 }
	if got := compactCooldownFor(3); got != 0 {
		t.Fatalf("抖动 0 → 期望 0s，实际 %v", got)
	}
	compactJitterRand = func() float64 { return 1 }
	if got := compactCooldownFor(3); got != 900*time.Second {
		t.Fatalf("抖动 1 → 期望 900s（上限不变），实际 %v", got)
	}
	t.Log("✓ full jitter：随机(0,base) 生效；base 分级 60/300/900s 不变")
}

// ── GAP-20261018-02：条数只防频繁压，✗ 不当「token 已超也不压」的一票否决 ──────────

// TestCompactByTokens_Under30MessagesStillCompacts — 验收②：
//
//	20 条（< 30）但估算 token 已超阈值 ⇒ 触发（旧判据在这里 return false）
//	26 条（< 30 · 有中间段）        ⇒ 真压出摘要（端到端）
//	5 条小消息                      ⇒ 不压（极低拦截线：防首轮就压）
func TestCompactByTokens_Under30MessagesStillCompacts(t *testing.T) {
	longCN := strings.Repeat("测", 300) // estimateTokens: 300/0.7 ≈ 428 token/条
	small := func(n int) []Message {
		out := make([]Message, n)
		for i := range out {
			out[i] = Message{ID: int64(i + 1), Role: "user", Content: longCN, Active: true}
		}
		return out
	}

	// ① 触发面：20 条 < 30 —— 先现读 token 总量，确认它真超了阈值（不然本格测的是空气）
	msgs20 := small(20)
	total := 0
	for _, m := range msgs20 {
		total += compactMessageTokens(m)
	}
	trig := CompactTriggerTokens("gemma-12B")
	if total <= trig {
		t.Fatalf("夹具有问题：20 条合计 %d token 未超阈值 %d", total, trig)
	}
	if !ShouldCompact(msgs20, "gemma-12B") {
		t.Fatalf("20 条（< %d）但 %d token > 阈值 %d ⇒ 必须触发压缩", CompactMinMessages, total, trig)
	}
	t.Logf("✓ 触发面: 20 条（< %d）· %d token > %d ⇒ ShouldCompact=true", CompactMinMessages, total, trig)

	// ② 端到端：26 条 < 30，中间段 = 26−3−20 = 3 条 ⇒ 真压（摘要落库 + 首 3 尾 20 保留）
	st, _ := newCompactStore(t)
	se, err := st.CreateSession("gemma-12B", "desktop", "", "少于30条也要压")
	if err != nil {
		t.Fatal(err)
	}
	compactSeed(t, st, se.ID, 26, longCN)
	sumCalls := 0
	did, err := st.MaybeCompact(context.Background(), se.ID, "gemma-12B", compactActive(t, st, se.ID),
		nil, func(context.Context, []map[string]any) (string, error) { sumCalls++; return "少于30条摘要", nil }, false)
	if err != nil || !did {
		t.Fatalf("26 条（< %d）超阈值应触发压缩: did=%v err=%v", CompactMinMessages, did, err)
	}
	if sumCalls != 1 {
		t.Fatalf("应恰好调用摘要函数 1 次（实 %d）", sumCalls)
	}
	if compactFindSummary(t, st, se.ID) == nil {
		t.Fatal("压缩后应存在摘要消息（摘要区在）")
	}
	// 26 − 3 被归档 + 1 摘要 = 24 条活动（首 3 + 尾 20 + 摘要 ⇒ 仍能继续对话）
	if active := compactActive(t, st, se.ID); len(active) != 26-3+1 {
		t.Fatalf("压缩后活动消息数应为 %d，实得 %d", 26-3+1, len(active))
	}
	t.Logf("✓ 端到端: 26 条 ⇒ 压中间 3 条 + 摘要 1 条 ⇒ 活动 24 条（首 %d + 尾 %d + 摘要）",
		CompactProtectFirstN, CompactProtectLastN)

	// ③ 对照：5 条小消息 ⇒ 不压（低于极低拦截线；且 token 也远不够）
	small5 := make([]Message, 5)
	for i := range small5 {
		small5[i] = Message{ID: int64(i + 1), Role: "user", Content: "hi", Active: true}
	}
	if ShouldCompact(small5, "gemma-12B") {
		t.Fatal("5 条小消息不应触发压缩（防首轮就压）")
	}
	t.Logf("✓ 对照: 5 条小消息（< %d）⇒ ShouldCompact=false", CompactHardMinMessages)
}

// ── GAP-20261018-03：压缩阈值扣掉实际固定开销（✗ 不硬编码 16K）────────────────

// overheadASCII — 造一个恰好 tokens 个 token 的 ASCII 串（本包 estimateTokens: ascii/4）。
func overheadASCII(tokens int) string {
	if tokens <= 0 {
		return ""
	}
	return strings.Repeat("x", tokens*4) // ascii/4 = tokens
}

// overheadMsgs — 造 n 条消息，合计约 tokensPerMsg×n token（ASCII 口径：4 字节/token）。
func overheadMsgs(n, tokensPerMsg int) []Message {
	body := overheadASCII(tokensPerMsg)
	out := make([]Message, n)
	for i := range out {
		out[i] = Message{ID: int64(i + 1), Role: "user", Content: body, Active: true}
	}
	return out
}

// TestCompactTriggerTokensOverhead — 动态扣减两组数字 + 零回归。
func TestCompactTriggerTokensOverhead(t *testing.T) {
	// ① 不传固定开销 ⇒ 与老口径逐字一致（8K 模型：8192×0.5 = 4096）
	if got, want := CompactTriggerTokensWithOverhead("gemma-12B", "", ""), CompactTriggerTokens("gemma-12B"); got != want {
		t.Fatalf("空参数应与老口径逐字一致：%d ≠ %d", got, want)
	}
	if got := CompactTriggerTokens("gemma-12B"); got != 4096 {
		t.Fatalf("gemma-12B 老口径阈值应为 4096，实得 %d", got)
	}
	// ② 固定开销 6000 ⇒ 预算 8192−6000=2192 ⇒ 阈值 1096 → 下限 MinWindow 2000
	if got := CompactTriggerTokensWithOverhead("gemma-12B", overheadASCII(6000), ""); got != CompactMinWindow {
		t.Fatalf("固定开销 6000 ⇒ 阈值应落到 MinWindow %d，实得 %d", CompactMinWindow, got)
	}
	// ③ 只给工具定义也走同一条扣减路
	if got := CompactTriggerTokensWithOverhead("gemma-12B", "", overheadASCII(6000)); got != CompactMinWindow {
		t.Fatalf("只有工具定义 6000 ⇒ 阈值也应落到 %d，实得 %d", CompactMinWindow, got)
	}
	// ④ 32K 模型：固定开销 16000 ⇒ 8384，但活跃窗口上限 8000 仍收口（对话口径不变）
	ctx, fixed, budget, threshold := CompactBudgetReadout("Qwen3.8-27B", overheadASCII(16000), "")
	if ctx != 32768 || fixed != 16000 || budget != 16768 {
		t.Fatalf("读数不符：ctx=%d fixed=%d budget=%d", ctx, fixed, budget)
	}
	if threshold != CompactActiveWindow {
		t.Fatalf("32K 模型阈值应被活跃窗口 %d 收口，实得 %d", CompactActiveWindow, threshold)
	}
	t.Logf("✓ 动态：gemma 4096 → 2000；Qwen 32K：ctx=%d 固定开销=%d 有效预算=%d 阈值=%d（未扣开销=%d）",
		ctx, fixed, budget, threshold, CompactTriggerTokens("Qwen3.8-27B"))
}

// TestCompactBudgetOverheadFloor — 固定开销 ≥ ctx ⇒ 有效预算不为负，回落 2048。
func TestCompactBudgetOverheadFloor(t *testing.T) {
	ctx, fixed, budget, threshold := CompactBudgetReadout("gemma-12B", overheadASCII(32000), "")
	if fixed != 32000 {
		t.Fatalf("固定开销应为 32000，实得 %d", fixed)
	}
	if budget < 0 || budget != CompactFixedOverheadFloor {
		t.Fatalf("有效预算必须非负且回落到 %d，实得 %d（ctx=%d）", CompactFixedOverheadFloor, budget, ctx)
	}
	if threshold != CompactMinWindow {
		t.Fatalf("触底后阈值应为 MinWindow %d（1024 → 抬到 2000），实得 %d", CompactMinWindow, threshold)
	}
	// 固定开销 > ctx 也绝不为负
	if _, _, b2, _ := CompactBudgetReadout("gemma-12B", overheadASCII(40000), ""); b2 != CompactFixedOverheadFloor {
		t.Fatalf("固定开销 40000 > ctx ⇒ 仍应回落 %d，实得 %d", CompactFixedOverheadFloor, b2)
	}
	t.Logf("✓ 下限保护：ctx=%d 固定开销=%d ⇒ 有效预算=%d（非负）· 阈值=%d", ctx, fixed, budget, threshold)
}

// TestShouldCompactWithOverhead — 判据接线两态：同一批消息，给不给固定开销结论相反。
func TestShouldCompactWithOverhead(t *testing.T) {
	msgs := overheadMsgs(12, 250) // 12 条 × 250 = 3000 token（≥ 极低拦截线 6）

	// 老口径（gemma 阈值 4096）⇒ 3000 不超 ⇒ 不压
	if ShouldCompact(msgs, "gemma-12B") {
		t.Fatalf("3000 token 未超老阈值 %d ⇒ 不该压", CompactTriggerTokens("gemma-12B"))
	}
	// 空参数 ⇒ 回落老口径（零回归）
	if ShouldCompactWithOverhead(msgs, "gemma-12B", "", "") {
		t.Fatal("空参数应回落老口径（不压）")
	}
	ptr := make([]*Message, len(msgs))
	for i := range msgs {
		m := msgs[i]
		ptr[i] = &m
	}
	if ShouldCompactPtrWithOverhead(ptr, "gemma-12B", "", "") != ShouldCompactPtr(ptr, "gemma-12B") {
		t.Fatal("指针版空参数应与老口径一致")
	}
	// 扣掉 6000 固定开销（阈值 2000）⇒ 3000 已超 ⇒ 压
	if !ShouldCompactWithOverhead(msgs, "gemma-12B", overheadASCII(6000), "") {
		t.Fatal("扣掉 6000 固定开销后（阈值 2000）3000 token 应触发压缩")
	}
	if !ShouldCompactPtrWithOverhead(ptr, "gemma-12B", overheadASCII(6000), "") {
		t.Fatal("指针版扣掉固定开销后应触发压缩")
	}
	t.Logf("✓ 两态：3000 token / gemma：老阈值 %d ⇒ 不压；扣 6000 开销（阈值 %d）⇒ 压",
		CompactTriggerTokens("gemma-12B"), CompactTriggerTokensWithOverhead("gemma-12B", overheadASCII(6000), ""))
}

// TestMaybeCompactWithFixedOverhead — 单一入口真正吃 WithFixedOverhead（含观测行四个数）。
func TestMaybeCompactWithFixedOverhead(t *testing.T) {
	st, _ := newCompactStore(t)
	se, err := st.CreateSession("gemma-12B", "desktop", "", "固定开销会话")
	if err != nil {
		t.Fatal(err)
	}
	// 24 条 × 125 token = 3000（老阈值 4096 ⇒ 不压；首 3 + 尾 20 之外仍有 1 条可压）
	longCN := strings.Repeat("测", 88) // 88/0.7 ≈ 125 token/条
	ids := compactSeed(t, st, se.ID, 24, longCN)
	msgs := compactActive(t, st, se.ID)
	if len(msgs) != len(ids) {
		t.Fatalf("夹具消息数不符：%d ≠ %d", len(msgs), len(ids))
	}
	total := 0
	for _, m := range msgs {
		total += compactMessageTokens(*m)
	}
	if total > CompactTriggerTokens("gemma-12B") {
		t.Fatalf("夹具有问题：%d token 已超老阈值 %d", total, CompactTriggerTokens("gemma-12B"))
	}

	var buf strings.Builder
	origOut, origFlags := log.Writer(), log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	defer func() { log.SetOutput(origOut); log.SetFlags(origFlags) }()

	// 不传固定开销 ⇒ 不压（零回归）
	if did, err := st.MaybeCompact(context.Background(), se.ID, "gemma-12B", msgs,
		nil, func(context.Context, []map[string]any) (string, error) { return "不该被调用", nil }, false); did || err != nil {
		t.Fatalf("不传固定开销时 %d token 未超阈值 ⇒ 不该压（did=%v err=%v）", total, did, err)
	}
	// 传固定开销 ⇒ 压，且打出 ctx/固定开销/有效预算/阈值四个数
	did, err := st.MaybeCompact(context.Background(), se.ID, "gemma-12B", msgs,
		nil, func(context.Context, []map[string]any) (string, error) { return "扣开销摘要", nil }, false,
		WithFixedOverhead(overheadASCII(6000), ""))
	if err != nil || !did {
		t.Fatalf("扣固定开销后应触发压缩: did=%v err=%v", did, err)
	}
	line := buf.String()
	for _, want := range []string{"ctx=8192", "fixed=", "budget=", "threshold=2000", "baseline=4096"} {
		if !strings.Contains(line, want) {
			t.Fatalf("观测行缺 %q：\n%s", want, line)
		}
	}
	t.Logf("✓ 端到端：%d token ⇒ 老阈值不压 / 扣开销后压；观测行：%s", total, strings.TrimSpace(line))
}
