package agent

import (
	"fmt"
	"github.com/Mr2109/zerg-swarm/core/internal/config"
	"github.com/Mr2109/zerg-swarm/core/internal/testutil"
	"os"
	"strings"
	"testing"
	"time"
)

// ===== 综合测试：web_search v1.0.1 全参数 =====

// 1. 基础搜索（默认——引擎全跑）
func TestV2Basic(t *testing.T) {
	testutil.RequireEnv(t, "ZERG_TEST_NET", "searx 桥（Python 依赖 + 外网）")
	t0 := time.Now()
	r, err := WebSearchV2(SearchParams{Query: "DeepSeek V4 Flash 本地部署", Limit: 5})
	if err != nil {
		t.Fatalf("失败: %v", err)
	}
	t.Logf("①基础搜索 耗时 %.1fs 长度 %d\n%s", time.Since(t0).Seconds(), len(r), truncate(r, 350))
}

// 2. lang=en（英文结果）
func TestV2LangEn(t *testing.T) {
	testutil.RequireEnv(t, "ZERG_TEST_NET", "searx 桥（Python 依赖 + 外网）")
	t0 := time.Now()
	r, err := WebSearchV2(SearchParams{Query: "GLM-5.3 Flash GGUF quantization", Limit: 4, Lang: "en"})
	if err != nil {
		t.Fatalf("失败: %v", err)
	}
	t.Logf("②lang=en 耗时 %.1fs\n%s", time.Since(t0).Seconds(), truncate(r, 350))
}

// 3. time_range=week（最新一周）
func TestV2TimeWeek(t *testing.T) {
	testutil.RequireEnv(t, "ZERG_TEST_NET", "searx 桥（Python 依赖 + 外网）")
	t0 := time.Now()
	r, err := WebSearchV2(SearchParams{Query: "AI 开源模型", Limit: 4, TimeRange: "week"})
	if err != nil {
		t.Fatalf("失败: %v", err)
	}
	t.Logf("③time=week 耗时 %.1fs\n%s", time.Since(t0).Seconds(), truncate(r, 350))
}

// 4. query 重写对比（原 query vs 重写后）
func TestV2RewriteCompare(t *testing.T) {
	testutil.RequireEnv(t, "ZERG_TEST_NET", "searx 桥（Python 依赖 + 外网）")
	raw := "请问 帮我查一下 虫族AI 分布式节点 的 最新 进展 怎么样 呢"
	rewritten := rewriteQuery(raw)
	t.Logf("④重写对比: %q → %q", raw, rewritten)
	if rewritten == raw {
		t.Fatalf("重写未生效")
	}
	// 重写后搜索（结果应该更好——噪音词没了）
	t0 := time.Now()
	r1, err1 := WebSearchV2(SearchParams{Query: raw, Limit: 3})
	r2, err2 := WebSearchV2(SearchParams{Query: rewritten, Limit: 3})
	if err1 == nil && err2 == nil {
		t.Logf("  原 query 结果 %d 字符 / 重写后 %d 字符（耗时 %.1fs/%.1fs）",
			len(r1), len(r2), time.Since(t0).Seconds(), time.Since(t0).Seconds())
	}
}

// 5. 缓存多轮命中
func TestV2CacheMulti(t *testing.T) {
	testutil.RequireEnv(t, "ZERG_TEST_NET", "searx 桥（Python 依赖 + 外网）")
	q := "searxng 部署教程"
	var times []time.Duration
	for i := 0; i < 3; i++ {
		t0 := time.Now()
		_, err := WebSearchV2(SearchParams{Query: q, Limit: 3})
		if err != nil {
			t.Fatalf("第 %d 次失败: %v", i+1, err)
		}
		times = append(times, time.Since(t0))
	}
	t.Logf("⑤缓存多轮: %v / %v / %v", times[0].Round(time.Millisecond), times[1].Round(time.Millisecond), times[2].Round(time.Millisecond))
	if times[1] > times[0]/2 {
		t.Logf("（第2次未明显加速——缓存可能未命中——检查 key）")
	}
}

// 6. 无结果查询（边界）—— A-06（2026-10-07）起空结果 fail-closed：桥把「服务不可达」与
// 「真无结果」都回 {"results": []}（无法区分）⇒ 一律带具名原因报 error，✗ 不静默「（无搜索结果）」。
func TestV2NoResult(t *testing.T) {
	testutil.RequireEnv(t, "ZERG_TEST_NET", "searx 桥（Python 依赖 + 外网）")
	t0 := time.Now()
	r, err := WebSearchV2(SearchParams{Query: "zzzqqqxxxyyy 不存在的关键词", Limit: 3})
	if err != nil {
		if !strings.Contains(err.Error(), "空结果") {
			t.Fatalf("空结果 fail-closed 的错误未带具名原因「空结果」: %v", err)
		}
		t.Logf("⑥无结果 耗时 %.1fs → fail-closed error: %v", time.Since(t0).Seconds(), err)
		return
	}
	t.Logf("⑥无结果 耗时 %.1fs → %q", time.Since(t0).Seconds(), r)
}

// 7. 超长 query 压缩
func TestV2LongQuery(t *testing.T) {
	long := strings.Repeat("虫族AI分布式节点算力汇聚开源模型部署架构设计研究 ", 5)
	rewritten := rewriteQuery(long)
	t.Logf("⑦超长压缩: 原 %d 字 → %d 字", len([]rune(long)), len([]rune(rewritten)))
	if len([]rune(rewritten)) > 120 {
		t.Fatalf("压缩未生效: %d 字", len([]rune(rewritten)))
	}
}

// 8. 并发（两个同时搜——缓存/goroutine 安全）
func TestV2Concurrent(t *testing.T) {
	testutil.RequireEnv(t, "ZERG_TEST_NET", "searx 桥（Python 依赖 + 外网）")
	done := make(chan string, 2)
	go func() {
		r, e := WebSearchV2(SearchParams{Query: "Go 语言", Limit: 2})
		if e != nil {
			done <- "err1"
		} else {
			done <- fmt.Sprintf("ok1(%d)", len(r))
		}
	}()
	go func() {
		r, e := WebSearchV2(SearchParams{Query: "Rust 语言", Limit: 2})
		if e != nil {
			done <- "err2"
		} else {
			done <- fmt.Sprintf("ok2(%d)", len(r))
		}
	}()
	a, b := <-done, <-done
	t.Logf("⑧并发: %s / %s", a, b)
}

// ===== 无硬编码取址（GAP-20260926-71）=====

// TestSSHTargetForFromConfig —— 取址（sshTargetFor）必须由 fleet 真源拼出，不得写死 IP。
// 哑化：overlay 把 config.NodeAddr 换成「忽略 fleet、恒回退机器名」⇒ 本用例红。
func TestSSHTargetForFromConfig(t *testing.T) {
	fleet := map[string]config.FleetNode{
		"edge-node":    {Host: "10.9.9.9", Port: 8100},
		"mini-node-1": {Host: "10.9.9.10"},
	}
	if got := sshTargetFor("edge-node", fleet); got != "g01@10.9.9.9" {
		t.Fatalf("sshTargetFor(edge-node) = %q，want g01@10.9.9.9（地址须来自 fleet 真源）", got)
	}
	if got := sshTargetFor("mini-node-1", fleet); got != "g01@10.9.9.10" {
		t.Fatalf("sshTargetFor(mini-node-1) = %q，want g01@10.9.9.10", got)
	}
	// 名册无此机 ⇒ NodeAddr 回退机器名（与 gateway.fleetNode 同口径）。
	if got := sshTargetFor("nope", fleet); got != "g01@nope" {
		t.Fatalf("sshTargetFor(nope) = %q，want g01@nope（回退口径）", got)
	}
}

// TestNetToolsNoHardcodedX3IP —— net_tools.go 的**代码行**不得含硬编码 EDGE-NODE IP。
// 与 gateway/ds4_route_evict_test.go:TestNoHardcodedAgentURLInSource 同口径：注释行不参与
// 扫描（注释里留旧值只为 traceability），取址必须走 config.NodeAddr（fleet.yaml 真源）。
// 哑化：把 net_tools.go 换回写死 IP 的旧版 ⇒ 本用例红。
func TestNetToolsNoHardcodedX3IP(t *testing.T) {
	b, err := os.ReadFile("net_tools.go")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(b), "\n")
	scanned := 0
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "//") {
			continue
		}
		scanned++
		if strings.Contains(line, "<worker-ip>") {
			t.Fatalf("net_tools.go:%d 代码行仍含硬编码 IP <worker-ip> —— 地址必须来自 fleet.yaml（config.NodeAddr）", i+1)
		}
	}
	if scanned == 0 {
		t.Fatal("没有扫到任何代码行——断言无效")
	}
}
