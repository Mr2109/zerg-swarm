// chat_compact_class_test.go — 丙批 §4.2 补测（2026-09-11）：压缩失败错误类型分流
//   + 硬熔断/冷却的手动重置（ResetCompactState）与只读状态快照（CompactStateSnapshot）。
// 覆盖:
//  1. DefaultErrClass 文本解析（HTTP 状态码 / status_code / 裸码 / Retry-After / 限流措辞）；
//  2. 4xx（非 429）→ 首次即直接硬熔断（不等 3 次），冷却取最大档 base；
//  3. 429 / 带 Retry-After → 冷却 = max(full jitter base, Retry-After)；
//  4. 5xx → 递进不变（回归：60→300→900，第 3 次熔断）；
//  5. WithErrClass 注入式失败元信息；WithErrClassifier 注入分类器；
//  6. ResetCompactState 清干净（冷却中调用后能立即重试，且落盘文件不再含该会话）；
//  7. CompactStateSnapshot 字段正确（冷却中/熔断/until/streak，未知会话全零）。
// 状态文件隔离: ZERG_STATE_DIR 指向 t.TempDir()（newCompactStore）。

package chat

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// pinCompactJitter — 钉住 full jitter 因子（返回 1=base / 0=零冷却），测试结束还原随机源
func pinCompactJitter(t *testing.T, f float64) {
	t.Helper()
	orig := compactJitterRand
	compactJitterRand = func() float64 { return f }
	t.Cleanup(func() { compactJitterRand = orig })
}

// ── 1. DefaultErrClass 文本解析 ──────────────────────────────────────

// TestCompactDefaultErrClass — 默认分类器从 error 文本抽取 HTTP 状态码与 Retry-After
func TestCompactDefaultErrClass(t *testing.T) {
	cases := []struct {
		msg        string
		wantStatus int
		wantRetry  time.Duration
	}{
		{"HTTP 403 Forbidden", 403, 0},
		{"http 401 unauthorized", 401, 0},
		{"status_code=422 unprocessable entity", 422, 0},
		{"status: 404 not found", 404, 0},
		{"HTTP 429 Too Many Requests", 429, 0},
		{"HTTP 429; Retry-After: 30", 429, 30 * time.Second},
		{"HTTP 503 Service Unavailable", 503, 0},
		{"摘要模型 500", 500, 0}, // 裸状态码
		{"too many requests", 429, 0},
		{"server busy; retry-after=12.5", 0, 12500 * time.Millisecond},
		{"connection refused", 0, 0},
		{"context deadline exceeded", 0, 0},
	}
	for _, tc := range cases {
		got := DefaultErrClass(errors.New(tc.msg))
		if got.HTTPStatus != tc.wantStatus || got.RetryAfter != tc.wantRetry {
			t.Errorf("DefaultErrClass(%q) = {status:%d retry:%v}, want {status:%d retry:%v}",
				tc.msg, got.HTTPStatus, got.RetryAfter, tc.wantStatus, tc.wantRetry)
		}
	}
	if DefaultErrClass(nil).HTTPStatus != 0 {
		t.Errorf("nil 错误应返回零值元信息")
	}
	t.Logf("✓ DefaultErrClass: %d 条文本解析全部符合预期（含 nil）", len(cases))
}

// ── 2. 4xx → 直接硬熔断（不等 3 次）───────────────────────────────────

// TestCompact4xxDirectTrip — HTTP 403（不可重试）：首次失败即熔断、冷却取最大档 900s
func TestCompact4xxDirectTrip(t *testing.T) {
	pinCompactJitter(t, 1) // 冷却 = base
	st, stateDir := newCompactStore(t)
	se, err := st.CreateSession("gemma-12B", "desktop", "", "4xx 直接熔断会话")
	if err != nil {
		t.Fatal(err)
	}
	compactSeed(t, st, se.ID, 30, strings.Repeat("测", 300))
	t0 := time.Unix(1_800_000_000, 0)
	useCompactClock(t, t0)

	sumCalls := 0
	failSum := func(context.Context, []map[string]any) (string, error) {
		sumCalls++
		return "", errors.New("HTTP 403 Forbidden: 模型拒绝该请求")
	}
	call := func(force bool) (bool, error) {
		return st.MaybeCompact(context.Background(), se.ID, "gemma-12B", compactActive(t, st, se.ID), nil, failSum, force)
	}

	// 第 1 次失败 → 立即熔断（streak=1 而非 3）
	did, cerr := call(false)
	if did || cerr == nil {
		t.Fatalf("4xx 首次应失败: did=%v err=%v", did, cerr)
	}
	f := compactGetFail(se.ID)
	if f == nil || !f.Disabled || f.Streak != 1 {
		t.Fatalf("4xx 应首次即硬熔断(streak=1/disabled): %+v", f)
	}
	if f.Class != compactClass4xx {
		t.Fatalf("分类应为 4xx: %q", f.Class)
	}
	if !approxEq(f.Until, float64(t0.Unix())+900) {
		t.Fatalf("4xx 冷却应取最大档 900s: until=%v want=%v", f.Until, float64(t0.Unix())+900)
	}

	// 熔断/冷却内不再尝试
	before := sumCalls
	if did, cerr := call(false); did || cerr != nil {
		t.Fatalf("熔断内应静默跳过: did=%v err=%v", did, cerr)
	}
	if sumCalls != before {
		t.Fatalf("熔断内不应再调用摘要（%d→%d）", before, sumCalls)
	}

	// Class 已落盘
	b, rerr := os.ReadFile(filepath.Join(stateDir, "compact_cooldown.json"))
	if rerr != nil {
		t.Fatalf("读冷却文件失败: %v", rerr)
	}
	if !strings.Contains(string(b), `"class": "4xx"`) {
		t.Fatalf("冷却文件应含 class=4xx: %s", string(b))
	}
	t.Logf("✓ 4xx: 首次失败即熔断(Streak=%d Disabled=%v Class=%s 冷却=%.0fs)，不等 3 次",
		f.Streak, f.Disabled, f.Class, f.Until-float64(t0.Unix()))
}

// ── 3. 429 / Retry-After → 冷却下界 ──────────────────────────────────

// TestCompact429RetryAfterFloor — 429/带 Retry-After：冷却 = max(full jitter base, Retry-After)
func TestCompact429RetryAfterFloor(t *testing.T) {
	cases := []struct {
		name         string
		errMsg       string
		jitter       float64
		wantCooldown float64 // 相对 t0 的秒数
		wantClass    string
	}{
		{"429 Retry-After 大于 base → 取 Retry-After", "HTTP 429 Too Many Requests; Retry-After: 120", 1, 120, compactClass429},
		{"429 Retry-After 小于 base → base 胜", "HTTP 429 Too Many Requests; Retry-After: 30", 1, 60, compactClass429},
		{"429 Retry-After 兜底抖动下界(抖动=0)", "HTTP 429; Retry-After: 120", 0, 120, compactClass429},
		{"无限流码但带 Retry-After → 取 Retry-After", "upstream busy; Retry-After: 200", 1, 200, compactClass429},
		{"429 无 Retry-After → 正常 base", "HTTP 429 Too Many Requests", 1, 60, compactClass429},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pinCompactJitter(t, tc.jitter)
			st, _ := newCompactStore(t)
			se, err := st.CreateSession("gemma-12B", "desktop", "", "429 会话-"+tc.name)
			if err != nil {
				t.Fatal(err)
			}
			compactSeed(t, st, se.ID, 30, strings.Repeat("测", 300))
			t0 := time.Unix(1_800_000_000, 0)
			useCompactClock(t, t0)

			sum := func(context.Context, []map[string]any) (string, error) {
				return "", errors.New(tc.errMsg)
			}
			did, cerr := st.MaybeCompact(context.Background(), se.ID, "gemma-12B", compactActive(t, st, se.ID), nil, sum, false)
			if did || cerr == nil {
				t.Fatalf("应失败: did=%v err=%v", did, cerr)
			}
			f := compactGetFail(se.ID)
			if f == nil {
				t.Fatalf("应有失败态")
			}
			if f.Class != tc.wantClass {
				t.Fatalf("分类应为 %s: %q", tc.wantClass, f.Class)
			}
			if !approxEq(f.Until, float64(t0.Unix())+tc.wantCooldown) {
				t.Fatalf("冷却应为 %.0fs: until=%v want=%v", tc.wantCooldown, f.Until, float64(t0.Unix())+tc.wantCooldown)
			}
			// 关键不变量：冷却不小于服务端 Retry-After
			if ra := DefaultErrClass(errors.New(tc.errMsg)).RetryAfter; ra > 0 && f.Until-float64(t0.Unix())+0.01 < ra.Seconds() {
				t.Fatalf("冷却 %.0fs 小于 Retry-After %.0fs", f.Until-float64(t0.Unix()), ra.Seconds())
			}
			// 单次 429 不应熔断（仅递进）
			if f.Disabled {
				t.Fatalf("单次 429 不应硬熔断: %+v", f)
			}
		})
	}
	t.Logf("✓ 429/Retry-After: %d 组冷却下界断言全部通过", len(cases))
}

// ── 4. 5xx 递进不变（回归）──────────────────────────────────────────

// TestCompact5xxProgressionUnchanged — HTTP 500：行为与旧实现一致（60→300→900，第 3 次熔断）
func TestCompact5xxProgressionUnchanged(t *testing.T) {
	pinCompactJitter(t, 1)
	st, _ := newCompactStore(t)
	se, err := st.CreateSession("gemma-12B", "desktop", "", "5xx 递进会话")
	if err != nil {
		t.Fatal(err)
	}
	compactSeed(t, st, se.ID, 30, strings.Repeat("测", 300))
	t0 := time.Unix(1_800_000_000, 0)
	cur := useCompactClock(t, t0)

	sumCalls := 0
	failSum := func(context.Context, []map[string]any) (string, error) {
		sumCalls++
		return "", errors.New("HTTP 500 Internal Server Error: upstream failure")
	}
	call := func() (bool, error) {
		return st.MaybeCompact(context.Background(), se.ID, "gemma-12B", compactActive(t, st, se.ID), nil, failSum, false)
	}

	// 第 1 次 → 60s，未熔断
	if did, cerr := call(); did || cerr == nil {
		t.Fatalf("第 1 次应失败: did=%v err=%v", did, cerr)
	}
	if f := compactGetFail(se.ID); f == nil || f.Streak != 1 || f.Disabled || f.Class != compactClass5xx ||
		!approxEq(f.Until, float64(t0.Unix())+60) {
		t.Fatalf("第 1 次应为 streak=1/未熔断/5xx/until=+60: %+v", f)
	}
	// 第 2 次（跨过冷却）→ 300s
	*cur = cur.Add(61 * time.Second)
	if did, cerr := call(); did || cerr == nil {
		t.Fatalf("第 2 次应失败: did=%v err=%v", did, cerr)
	}
	if f := compactGetFail(se.ID); f == nil || f.Streak != 2 || f.Disabled ||
		!approxEq(f.Until, float64(cur.Unix())+300) {
		t.Fatalf("第 2 次应为 streak=2/未熔断/until=+300: %+v", f)
	}
	// 第 3 次 → 900s + 熔断
	*cur = cur.Add(301 * time.Second)
	if did, cerr := call(); did || cerr == nil {
		t.Fatalf("第 3 次应失败: did=%v err=%v", did, cerr)
	}
	f3 := compactGetFail(se.ID)
	if f3 == nil || f3.Streak != 3 || !f3.Disabled || !approxEq(f3.Until, float64(cur.Unix())+900) {
		t.Fatalf("第 3 次应为 streak=3/熔断/until=+900: %+v", f3)
	}
	t.Logf("✓ 5xx 回归: 60→300→900 递进不变，第 3 次熔断（class=%s）", f3.Class)
}

// ── 5. 注入式失败元信息 / 分类器 ────────────────────────────────────

// TestCompactErrClassInjection — 错误文本无状态码时，WithErrClass / WithErrClassifier 可注入分类
func TestCompactErrClassInjection(t *testing.T) {
	pinCompactJitter(t, 1)

	// (a) WithErrClass 直接注入 403 → 直接熔断
	st, _ := newCompactStore(t)
	se, err := st.CreateSession("gemma-12B", "desktop", "", "注入元信息会话")
	if err != nil {
		t.Fatal(err)
	}
	compactSeed(t, st, se.ID, 30, strings.Repeat("测", 300))
	useCompactClock(t, time.Unix(1_800_000_000, 0))
	sum := func(context.Context, []map[string]any) (string, error) {
		return "", errors.New("纯文本错误：无任何状态码")
	}
	did, cerr := st.MaybeCompact(context.Background(), se.ID, "gemma-12B", compactActive(t, st, se.ID),
		nil, sum, false, WithErrClass(CompactErrClass{HTTPStatus: 403}))
	if did || cerr == nil {
		t.Fatalf("应失败: did=%v err=%v", did, cerr)
	}
	if f := compactGetFail(se.ID); f == nil || f.Class != compactClass4xx || !f.Disabled {
		t.Fatalf("注入 403 元信息应识别为 4xx 并熔断: %+v", f)
	}

	// (b) WithErrClassifier 注入自定义分类器（文本错误 → 429 + Retry-After 500s）
	st2, _ := newCompactStore(t)
	se2, err := st2.CreateSession("gemma-12B", "desktop", "", "注入分类器会话")
	if err != nil {
		t.Fatal(err)
	}
	compactSeed(t, st2, se2.ID, 30, strings.Repeat("测", 300))
	t0 := time.Unix(1_800_000_000, 0)
	useCompactClock(t, t0)
	did2, cerr2 := st2.MaybeCompact(context.Background(), se2.ID, "gemma-12B", compactActive(t, st2, se2.ID),
		nil, sum, false, WithErrClassifier(func(error) CompactErrClass {
			return CompactErrClass{HTTPStatus: 429, RetryAfter: 500 * time.Second}
		}))
	if did2 || cerr2 == nil {
		t.Fatalf("应失败: did=%v err=%v", did2, cerr2)
	}
	if f := compactGetFail(se2.ID); f == nil || f.Class != compactClass429 || !approxEq(f.Until, float64(t0.Unix())+500) {
		t.Fatalf("注入分类器应得 429/冷却 500s: %+v", f)
	}
	t.Logf("✓ 注入：WithErrClass(403)→4xx 熔断；WithErrClassifier(429/500s)→冷却 500s")
}

// ── 6. ResetCompactState ────────────────────────────────────────────

// TestCompactResetState — 冷却中重置后立即重试；落盘文件不再含该会话；幂等
func TestCompactResetState(t *testing.T) {
	pinCompactJitter(t, 1)
	st, stateDir := newCompactStore(t)
	se, err := st.CreateSession("gemma-12B", "desktop", "", "重置会话")
	if err != nil {
		t.Fatal(err)
	}
	compactSeed(t, st, se.ID, 30, strings.Repeat("测", 300))
	t0 := time.Unix(1_800_000_000, 0)
	useCompactClock(t, t0)

	failing := true
	sum := func(context.Context, []map[string]any) (string, error) {
		if failing {
			return "", errors.New("HTTP 429 Too Many Requests; Retry-After: 300")
		}
		return "重置后摘要正文", nil
	}
	call := func() (bool, error) {
		return st.MaybeCompact(context.Background(), se.ID, "gemma-12B", compactActive(t, st, se.ID), nil, sum, false)
	}

	// 制造一次失败 → 进入冷却（Retry-After 300s）
	if did, cerr := call(); did || cerr == nil {
		t.Fatalf("首次应失败: did=%v err=%v", did, cerr)
	}
	if ic, _, until, streak := CompactStateSnapshot(se.ID); !ic || streak != 1 || until <= float64(t0.Unix()) {
		t.Fatalf("失败后应处于冷却: inCooldown=%v until=%v streak=%d", ic, until, streak)
	}
	// 冷却内立即再试 → 静默跳过
	if did, cerr := call(); did || cerr != nil {
		t.Fatalf("冷却内应跳过: did=%v err=%v", did, cerr)
	}

	// 重置（落盘错误应为 nil）
	if rerr := ResetCompactState(se.ID); rerr != nil {
		t.Fatalf("ResetCompactState 应成功: %v", rerr)
	}
	if f := compactGetFail(se.ID); f != nil {
		t.Fatalf("重置后应无失败态: %+v", f)
	}
	if ic, dis, until, streak := CompactStateSnapshot(se.ID); ic || dis || until != 0 || streak != 0 {
		t.Fatalf("重置后快照应全零: ic=%v dis=%v until=%v streak=%d", ic, dis, until, streak)
	}
	b, rerr := os.ReadFile(filepath.Join(stateDir, "compact_cooldown.json"))
	if rerr != nil {
		t.Fatalf("读冷却文件失败: %v", rerr)
	}
	if strings.Contains(string(b), se.ID) {
		t.Fatalf("落盘文件不应再含该会话: %s", string(b))
	}

	// 冷却内本应跳过 → 重置后可立即重试并成功
	failing = false
	did, cerr := call()
	if cerr != nil || !did {
		t.Fatalf("重置后应可立即重试成功: did=%v err=%v", did, cerr)
	}
	// 幂等：无记录再重置仍返回 nil
	if rerr := ResetCompactState(se.ID); rerr != nil {
		t.Fatalf("幂等重置应返回 nil: %v", rerr)
	}
	t.Logf("✓ ResetCompactState: 冷却中重置 → 快照全零/落盘清空 → 立即重试成功；二次重置幂等 nil")
}

// ── 7. CompactStateSnapshot ─────────────────────────────────────────

// TestCompactStateSnapshot — 字段正确：冷却中/熔断/until/streak；到期后熔断自动反映为已解除；未知会话全零
func TestCompactStateSnapshot(t *testing.T) {
	pinCompactJitter(t, 1)
	st, _ := newCompactStore(t)
	se, err := st.CreateSession("gemma-12B", "desktop", "", "快照会话")
	if err != nil {
		t.Fatal(err)
	}
	compactSeed(t, st, se.ID, 30, strings.Repeat("测", 300))
	t0 := time.Unix(1_800_000_000, 0)
	cur := useCompactClock(t, t0)

	sum := func(context.Context, []map[string]any) (string, error) {
		return "", errors.New("HTTP 429 Too Many Requests; Retry-After: 120")
	}
	call := func() (bool, error) {
		return st.MaybeCompact(context.Background(), se.ID, "gemma-12B", compactActive(t, st, se.ID), nil, sum, false)
	}

	// 未知会话 → 全零
	if ic, dis, until, streak := CompactStateSnapshot("不存在的会话"); ic || dis || until != 0 || streak != 0 {
		t.Fatalf("未知会话快照应全零: ic=%v dis=%v until=%v streak=%d", ic, dis, until, streak)
	}

	// 第 1 次失败（429/Retry-After=120）→ 冷却中、未熔断
	if did, cerr := call(); did || cerr == nil {
		t.Fatalf("首次应失败: did=%v err=%v", did, cerr)
	}
	ic, dis, until, streak := CompactStateSnapshot(se.ID)
	if !ic || dis || streak != 1 || !approxEq(until, float64(t0.Unix())+120) {
		t.Fatalf("429 快照应为 冷却/未熔断/streak=1/until=+120: ic=%v dis=%v until=%v streak=%d", ic, dis, until, streak)
	}

	// 推进到第 3 次失败 → 硬熔断
	*cur = cur.Add(121 * time.Second) // 跨过第 1 次冷却
	if _, cerr := call(); cerr == nil {
		t.Fatalf("第 2 次应失败")
	}
	*cur = cur.Add(301 * time.Second) // 跨过第 2 次冷却（max(300,120)=300）
	if _, cerr := call(); cerr == nil {
		t.Fatalf("第 3 次应失败")
	}
	ic, dis, until, streak = CompactStateSnapshot(se.ID)
	if !ic || !dis || streak != 3 || until <= float64(cur.Unix()) {
		t.Fatalf("第 3 次后快照应为 冷却/熔断/streak=3: ic=%v dis=%v until=%v streak=%d", ic, dis, until, streak)
	}

	// 冷却到期（未触发压缩）→ 快照报「冷却解除、熔断已不生效」（下次尝试会自动清除标记）
	*cur = cur.Add(time.Duration(until-float64(cur.Unix()))*time.Second + time.Second)
	ic, dis, _, streak = CompactStateSnapshot(se.ID)
	if ic || dis || streak != 3 {
		t.Fatalf("冷却到期后快照应为 未冷却/未生效熔断/streak 保留=3: ic=%v dis=%v streak=%d", ic, dis, streak)
	}
	t.Logf("✓ CompactStateSnapshot: 未知全零 / 429 冷却未熔断 / 3 次熔断 / 到期解除（streak 保留）")
}
