package gateway

// gateway_breaker_reason_test.go — 「涨计数但 last_error 为空」缺口的反例优先用例。
//
// 现场（活系统实测）：GET /api/gateway/breakers 出现过 local/x3 都是 state=half-open、
// fail_count=11（= healthyTripLimit+1）、而 last_error 是空串。复位后 40s 无新失败 ⇒ 陈旧计数。
//
// 缺口本质：快照有两类计数来源（failCounts 由 markFailure/tripMachine/pickRouteExcluding 涨、
// tripCounts 由 tripMachine 涨），而 lastErr 只在 markFailure 传了原因时才写。于是
//   - pickRouteExcluding 的强制熔断（failCounts = healthyTripLimit+1 = 11，不写 lastErr）
//   - tripMachine（tripCounts +1、failCounts +1，不写 lastErr）
// 这两条路径涨的计数，在快照里看不到任何原因 —— 正好是现场那条「11 且原因空」。
//
// 本组用例先复现旧形态（计数有、原因无），再断言修后：
//   1. 计数路径一定留下原因（last_error 非空 + last_error_source 标出路径）；
//   2. 就算真没记录，对外也不再是空串，而是 breakerNoReasonText + last_error_recorded=false；
//   3. 兜底原因不含标准 §五 的禁用占位词。

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/config"
	"github.com/Mr2109/zerg-swarm/core/internal/store"
)

// breakerReasonPlaceholderWords 与标准 §五 / modelreg 门禁同表（禁用占位词）。
// 与 api 包的 licensePlaceholderWords 独立再列一遍：测的是「对外文本里不出现」，不依赖被测包的表。
var breakerReasonPlaceholderWords = []string{
	"unset", "pending", "tbd", "todo", "n/a", "na", "placeholder",
	"待定", "未定", "none", "null", "-",
}

// assertBreakerTextNoPlaceholder 断言熔断快照对外 JSON 里没有把禁用占位词当值输出。
func assertBreakerTextNoPlaceholder(t *testing.T, raw []byte) {
	t.Helper()
	body := strings.ToLower(string(raw))
	for _, w := range breakerReasonPlaceholderWords {
		if strings.Contains(body, `"`+w+`"`) {
			t.Errorf("熔断快照出现禁用占位值 %q（标准 §五 禁占位符）—— body=%s", w, raw)
		}
	}
}

// singleCandidateGateway 单候选（x3）网关——用于确定性地走 pickRouteExcluding 的强制熔断分支
// （选回被排除机器才会把 failCounts 顶到 healthyTripLimit+1）。
func singleCandidateGateway() *Gateway {
	cfg := &config.FleetConfig{
		Models: map[string][]config.ModelCandidate{
			"ornith": {{Host: "x3", File: "/data/ornith.gguf"}},
		},
		Fleet: map[string]config.FleetNode{
			"x3": {Host: "<worker-ip>", Port: 8100},
		},
	}
	return &Gateway{
		config:     cfg,
		store:      store.NewStore(),
		roundRobin: map[string]int{},
		failCounts: map[string]int{},
		failSince:  map[string]time.Time{},
		tripCounts: map[string]int{},
		lastErr:    map[string]string{},
		lastErrAt:  map[string]time.Time{},
		lastErrSrc: map[string]string{},
	}
}

// TestBreaker_StaleCountWithNoReason_ShowsExplicitNotRecorded 反例（现场形态）：
// 计数存在（fail_count=11 = healthyTripLimit+1、冷却已过）但一条原因都没有——
// 旧代码输出 last_error=""（与计数自相矛盾），修后必须是明确的「未记录」文本。
func TestBreaker_StaleCountWithNoReason_ShowsExplicitNotRecorded(t *testing.T) {
	g := breakerTestGateway()
	// 复刻旧计数路径留下的形态：只有计数与熔断时间，没有 lastErr/lastErrSrc
	g.failCounts["x3"] = healthyTripLimit + 1 // 11 —— 与现场一致
	g.failSince["x3"] = time.Now().Add(-(circuitCooldown + time.Second))

	b := findBreaker(g.BreakerSnapshot(), "x3")
	if b == nil {
		t.Fatal("快照里没有 x3")
	}
	if b.FailCount != healthyTripLimit+1 {
		t.Fatalf("前置条件：fail_count 应为 %d（现场值）——实际 %d", healthyTripLimit+1, b.FailCount)
	}
	if b.State != "half-open" {
		t.Fatalf("前置条件：冷却已过应为 half-open（现场值）——实际 %s", b.State)
	}
	// 缺口本体：有计数就不许给空串
	if b.LastError == "" {
		t.Error("有计数（>0）却输出空 last_error —— 这正是现场「fail_count=11 但原因为空」的迷惑形态")
	}
	if b.LastError != breakerNoReasonText {
		t.Errorf("未记录原因时 last_error 应为 %q——实际 %q", breakerNoReasonText, b.LastError)
	}
	if b.LastErrorRecorded {
		t.Error("没有原因记录时 last_error_recorded 必须为 false（消费方据此判定「未记录」）")
	}
	raw, err := json.Marshal(b)
	if err != nil {
		t.Fatalf("快照 JSON 序列化失败: %v", err)
	}
	assertBreakerTextNoPlaceholder(t, raw)
	t.Logf("现场形态复刻: state=%s fail_count=%d last_error=%q recorded=%v — %s",
		b.State, b.FailCount, b.LastError, b.LastErrorRecorded, raw)
}

// TestBreaker_ForcedExclusionCountPath_RecordsReasonAndSource 真因路径：
// pickRouteExcluding 的强制熔断（failCounts = healthyTripLimit+1 = 11）历史上不写 lastErr——
// 修后必须留下原因，且 last_error_source 指明是这条路径。
func TestBreaker_ForcedExclusionCountPath_RecordsReasonAndSource(t *testing.T) {
	g := singleCandidateGateway()
	// 单候选 = x3，排除 x3 后仍会选回 x3 → 触发强制熔断分支
	_, err := g.pickRouteExcluding("ornith", "x3",
		"failover: backend x3 forward failed: context deadline exceeded")
	// 强制熔断后重选只有更明确的错误（唯一候选被熔断）——这里关心的是计数与原因是否成对出现
	if err == nil {
		t.Fatal("唯一候选被强制熔断后重选应失败（无其他候选）——实际成功")
	}
	if g.failCounts["x3"] != healthyTripLimit+1 {
		t.Fatalf("强制熔断应把 fail_count 顶到 %d——实际 %d", healthyTripLimit+1, g.failCounts["x3"])
	}
	if g.lastErr["x3"] == "" {
		t.Error("强制熔断涨了计数却没写 lastErr —— 这正是本缺口（修后不允许）")
	}
	b := findBreaker(g.BreakerSnapshot(), "x3")
	if b == nil {
		t.Fatal("快照里没有 x3")
	}
	if !b.LastErrorRecorded || b.LastError == "" {
		t.Errorf("计数路径必须留下可读原因——recorded=%v last_error=%q", b.LastErrorRecorded, b.LastError)
	}
	if !strings.Contains(b.LastError, "context deadline exceeded") {
		t.Errorf("原因应保留调用方给的原文——实际 %q", b.LastError)
	}
	if b.LastErrorSource != "pickRouteExcluding" {
		t.Errorf("last_error_source 应为 pickRouteExcluding——实际 %q", b.LastErrorSource)
	}
	raw, _ := json.Marshal(b)
	assertBreakerTextNoPlaceholder(t, raw)
	t.Logf("强制熔断路径: fail_count=%d state=%s last_error=%q source=%s",
		b.FailCount, b.State, b.LastError, b.LastErrorSource)
}

// TestBreaker_EmptyReason_IsAutoFilledWithCallSite 兜底：调用点给了空串（动态拼接失败）时，
// 计数点也绝不能写出空原因——自动补一条带调用点的明确文本，且带 last_error_source=unspecified。
func TestBreaker_EmptyReason_IsAutoFilledWithCallSite(t *testing.T) {
	fileLine := regexp.MustCompile(`at [A-Za-z0-9_.]+\.go:\d+`)

	t.Run("markFailure", func(t *testing.T) {
		g := breakerTestGateway()
		g.markFailure("x3", "")
		b := findBreaker(g.BreakerSnapshot(), "x3")
		if b == nil {
			t.Fatal("快照里没有 x3")
		}
		if b.LastError == "" {
			t.Fatal("空原因不得写出空 last_error")
		}
		if !strings.HasPrefix(b.LastError, breakerReasonUnspecified) {
			t.Errorf("兜底原因应以 %q 开头——实际 %q", breakerReasonUnspecified, b.LastError)
		}
		if !fileLine.MatchString(b.LastError) {
			t.Errorf("兜底原因应带调用点 file:line（便于定位漏传原因的调用方）——实际 %q", b.LastError)
		}
		if b.LastErrorSource != "markFailure" {
			t.Errorf("last_error_source 应为计数路径 markFailure（兜底只体现在文本前缀）——实际 %q", b.LastErrorSource)
		}
		t.Logf("markFailure 空原因兜底: %q", b.LastError)
	})

	t.Run("tripMachine", func(t *testing.T) {
		g := breakerTestGateway()
		g.tripMachine("mini1", "   ") // 仅空白也视为没给原因
		b := findBreaker(g.BreakerSnapshot(), "mini1")
		if b == nil {
			t.Fatal("快照里没有 mini1")
		}
		if b.LastError == "" {
			t.Fatal("空原因不得写出空 last_error")
		}
		if !strings.HasPrefix(b.LastError, breakerReasonUnspecified) {
			t.Errorf("兜底原因应以 %q 开头——实际 %q", breakerReasonUnspecified, b.LastError)
		}
		if b.LastErrorSource != "tripMachine" {
			t.Errorf("last_error_source 应为计数路径 tripMachine——实际 %q", b.LastErrorSource)
		}
		t.Logf("tripMachine 空原因兜底: %q", b.LastError)
	})
}

// TestBreaker_ClosedIdleMachine_KeepsEmptyReason 反向守门：没有任何熔断状态（closed + 计数 0）时
// 不硬塞文本——空 last_error 在这里不构成误导（不是「有计数却没原因」）。
func TestBreaker_ClosedIdleMachine_KeepsEmptyReason(t *testing.T) {
	g := breakerTestGateway()
	for _, host := range []string{"x3", "local", "mini1"} {
		b := findBreaker(g.BreakerSnapshot(), host)
		if b == nil {
			t.Fatalf("快照里没有 %s", host)
		}
		if b.State != "closed" || b.FailCount != 0 || b.TripCount != 0 {
			t.Fatalf("前置条件：%s 应为 closed/0/0——实际 %s/%d/%d", host, b.State, b.FailCount, b.TripCount)
		}
		if b.LastError != "" {
			t.Errorf("无熔断状态时 last_error 应保持空（不硬塞文本）——%s 实际 %q", host, b.LastError)
		}
		if b.LastErrorRecorded {
			t.Errorf("无熔断状态时 last_error_recorded 应为 false——%s 实际 true", host)
		}
	}
}

// TestBreaker_TripCountOnly_NoReasonStillNeverBlank tripCounts 单涨（不带 failCounts 的场景）也不能显示空：
// 造「只涨 trip 计数、没有任何原因」的形态（旧代码 tripMachine 的极端表象），断言对外是明确「未记录」。
func TestBreaker_TripCountOnly_NoReasonStillNeverBlank(t *testing.T) {
	g := breakerTestGateway()
	g.tripCounts["mini1"] = 2 // 只有 trip 计数，无 failCounts / 无 lastErr
	b := findBreaker(g.BreakerSnapshot(), "mini1")
	if b == nil {
		t.Fatal("快照里没有 mini1")
	}
	if b.TripCount != 2 {
		t.Fatalf("前置条件：trip_count 应为 2——实际 %d", b.TripCount)
	}
	if b.LastError == "" {
		t.Error("有 trip 计数却输出空 last_error——与「计数非零」自相矛盾")
	}
	if b.LastError != breakerNoReasonText || b.LastErrorRecorded {
		t.Errorf("未记录原因时应为 %q + recorded=false——实际 %q/%v",
			breakerNoReasonText, b.LastError, b.LastErrorRecorded)
	}
	raw, _ := json.Marshal(b)
	assertBreakerTextNoPlaceholder(t, raw)
}

// TestBreaker_SnapshotJSON_ReasonFieldsConsistent 端到端形状：一次真实失败后，
// JSON 里 last_error / last_error_recorded / last_error_source 三者自洽（UI/运维只读快照的口径）。
func TestBreaker_SnapshotJSON_ReasonFieldsConsistent(t *testing.T) {
	g := breakerTestGateway()
	g.markFailure("x3", "backend x3 returned 503: upstream saturated")
	b := findBreaker(g.BreakerSnapshot(), "x3")
	if b == nil {
		t.Fatal("快照里没有 x3")
	}
	raw, err := json.Marshal(b)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("反序列化失败: %v", err)
	}
	if v, _ := got["last_error"].(string); v != "backend x3 returned 503: upstream saturated" {
		t.Errorf("last_error 应为原因原文——实际 %v", got["last_error"])
	}
	if v, _ := got["last_error_recorded"].(bool); !v {
		t.Errorf("last_error_recorded 应为 true——实际 %v", got["last_error_recorded"])
	}
	if v, _ := got["last_error_source"].(string); v != "markFailure" {
		t.Errorf("last_error_source 应为 markFailure——实际 %v", got["last_error_source"])
	}
	assertBreakerTextNoPlaceholder(t, raw)
	t.Logf("快照 JSON: %s", raw)
}
