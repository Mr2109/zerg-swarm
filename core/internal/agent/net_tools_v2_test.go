package agent

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// ===== 综合测试：web_search v1.0.1 全参数 =====

// 1. 基础搜索（默认——引擎全跑）
func TestV2Basic(t *testing.T) {
	t0 := time.Now()
	r, err := WebSearchV2(SearchParams{Query: "DeepSeek V4 Flash 本地部署", Limit: 5})
	if err != nil {
		t.Fatalf("失败: %v", err)
	}
	t.Logf("①基础搜索 耗时 %.1fs 长度 %d\n%s", time.Since(t0).Seconds(), len(r), truncate(r, 350))
}

// 2. lang=en（英文结果）
func TestV2LangEn(t *testing.T) {
	t0 := time.Now()
	r, err := WebSearchV2(SearchParams{Query: "GLM-5.3 Flash GGUF quantization", Limit: 4, Lang: "en"})
	if err != nil {
		t.Fatalf("失败: %v", err)
	}
	t.Logf("②lang=en 耗时 %.1fs\n%s", time.Since(t0).Seconds(), truncate(r, 350))
}

// 3. time_range=week（最新一周）
func TestV2TimeWeek(t *testing.T) {
	t0 := time.Now()
	r, err := WebSearchV2(SearchParams{Query: "AI 开源模型", Limit: 4, TimeRange: "week"})
	if err != nil {
		t.Fatalf("失败: %v", err)
	}
	t.Logf("③time=week 耗时 %.1fs\n%s", time.Since(t0).Seconds(), truncate(r, 350))
}

// 4. query 重写对比（原 query vs 重写后）
func TestV2RewriteCompare(t *testing.T) {
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

// 6. 无结果查询（边界）
func TestV2NoResult(t *testing.T) {
	t0 := time.Now()
	r, err := WebSearchV2(SearchParams{Query: "zzzqqqxxxyyy 不存在的关键词", Limit: 3})
	if err != nil {
		t.Fatalf("失败: %v", err)
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
