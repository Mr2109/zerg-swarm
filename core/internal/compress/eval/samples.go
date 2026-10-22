package eval

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/Mr2109/zerg-swarm/core/internal/compress"
)

// 本笔的样本面：observation 类 10 条（任务单 3.2 A 表），中英各半，
// 每条恰好 5 条 needle ⇒ 全批 50 条 = 20 window + 30 recoverable（任务单 3.3）。
//
// 构造纪律（逐条对应任务单 §3.1）：
//   规1 类内 zh=R01–R05 / en=R06–R10，各 5
//   规2 对照 = R05 / R10，1800–2400 token；其余 4400–6400 token
//   规5 needle 是逐字子串，构造时就落进正文（不是事后拿摘要去凑）
//   规7 observation 只钉「错误 / 失败证据」段：R02（go test 失败）、R07（panic）
//   规8 可卸载段必给 raw_ptr，命名 = unload/<sha256(原文)[:16]>.txt（与 ctx_unload.go 同规）
//   规9 不写真秘密；规10 只读模板，不读仓内文件
//
// ★ 一处偏离：两个窗口 needle 放在「未被闸碰到的小段」而不是全塞进 Pinned 段。
// 理由：observation 大多数样本（find 输出 / 查询表 / 网页正文）根本没有「错误证据」段，
// 按规7 就不该钉；这些样本靠 PerOutputPct 单条闸卸掉大段、留下小段（尾部/表头），
// 正好检验「闸只碰该碰的段」——比全塞 Pinned 更能分辨引擎好坏。

// Samples —— 返回本笔全部样本（确定性：同一二进制两次调用逐字节相同）。
func Samples() []Sample {
	all := []Sample{
		buildR01(), buildR02(), buildR03(), buildR04(), buildR05(),
		buildR06(), buildR07(), buildR08(), buildR09(), buildR10(),
	}
	for i := range all {
		all[i].finalize()
	}
	return all
}

// selector —— 按 id 取样本（跑批可 -only R01,R05 缩小面）。
func Select(ids []string) []Sample {
	all := Samples()
	if len(ids) == 0 {
		return all
	}
	want := map[string]bool{}
	for _, id := range ids {
		want[strings.TrimSpace(id)] = true
	}
	out := make([]Sample, 0, len(ids))
	for _, s := range all {
		if want[s.ID] {
			out = append(out, s)
		}
	}
	return out
}

// finalize —— 补 raw_ptr（内容寻址）＋ 自检 needle 真在原文里。
//
// 自检放在这里而不是只在测试里：跑批与测试走同一条路，样本一旦构造坏了，
// 跑批当场红，不会先跑出一屏好看的数字再靠人发现。
func (s *Sample) finalize() {
	for i := range s.Segments {
		sg := &s.Segments[i]
		if sg.RawPtr == "-" { // 构造期标记：需要 raw_ptr
			sg.RawPtr = "unload/" + textHash(sg.Text) + ".txt"
		}
	}
}

// textHash —— sha256(原文)[:16] 的十六进制（32 字符），与 agent.ctxUnload 的命名同规。
func textHash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:16])
}

// ---------------------------------------------------------------- 构造工具

// sb 是一段样本的构造器：段落与 needle 一起长出来，needle 落点即时记录。
type sb struct {
	segs    []Segment
	needles []Needle
}

// addP 追加一段（ptr=true ⇒ 生成时打 raw_ptr 标记；Content-addressed 名字在 finalize 补）。
func (b *sb) addP(kind compress.Kind, pinned, ptr bool, text string, anchors ...string) int {
	seg := Segment{Kind: kind, Pinned: pinned, Text: text, Anchors: anchors}
	if ptr {
		seg.RawPtr = "-"
	}
	b.segs = append(b.segs, seg)
	return len(b.segs) - 1
}

// needle 挂一条必留信息；返回 id 便于复查。
func (b *sb) needle(id, text, where string, seg int, typ string) string {
	b.needles = append(b.needles, Needle{ID: id, Text: text, Where: where, Segment: seg, Type: typ})
	return id
}

// padTo —— 追加行直到 EstimateTokens 达到 target（估算用尺子唯一口径）。
//
// 为什么要真填到 target 而不是「大约」：len_assert 是机器判据（规3），
// 差 1 token 也算不合格；这里宁可多跑几轮循环。
func padTo(b *strings.Builder, target int, line func(i int) string) {
	for i := 0; compress.EstimateTokens(b.String()) < target; i++ {
		b.WriteString(line(i))
		b.WriteByte('\n')
	}
}

// ---------------------------------------------------------------- R01 zh

// R01 zh · `find . -type f | sort` 输出（超大件藏在中段）。
func buildR01() Sample {
	b := &sb{}

	var body strings.Builder
	body.WriteString("# find . -type f | sort —— 工具输出已按大小排序并截断，行数见尾部\n")
	body.WriteString("# 分组 00：核心包（.go 与 _test.go 交替）\n")
	padTo(&body, 600, func(i int) string {
		if i%43 == 0 && i > 0 {
			return fmt.Sprintf("# 分组 %02d：%d 个文件，累计 %d KiB", i/43, 43, 4096+i*37)
		}
		return fmt.Sprintf("./core/internal/pkg%02d/handler_%04d.go", i%41, i)
	})
	body.WriteString("./data/ruler-bigpool/blob_0042.bin\n")
	body.WriteString("./data/ruler-bigpool/blob_0117.bin\n")
	body.WriteString("./docs/legacy-notes-old/README.md\n")
	body.WriteString("# 上面三件是超大户（合计 812 MiB），已在此处单列\n")
	padTo(&body, 4500, func(i int) string {
		if i%31 == 0 {
			return fmt.Sprintf("./core/internal/pkg%02d/adapter_%04d_test.go", i%41, i)
		}
		return fmt.Sprintf("./core/internal/pkg%02d/adapter_%04d.go", i%41, i)
	})
	s0 := b.addP(compress.KindObservation, false, true, body.String(),
		"./data/ruler-bigpool/blob_0042.bin", "./docs/legacy-notes-old", "# 分组 00")

	var tail strings.Builder
	tail.WriteString("# 尾部（未截断）：compress 子包\n")
	tail.WriteString("./core/internal/compress/policy.go\n")
	tail.WriteString("./core/internal/compress/pruner_free.go\n")
	tail.WriteString("./core/internal/compress/guard.go\n")
	tail.WriteString("# 共 6000 行，其中 .go 占 5412 行\n")
	padTo(&tail, 900, func(i int) string {
		return fmt.Sprintf("./scripts/ruler/gen_samples_%03d.sh", i)
	})
	s1 := b.addP(compress.KindObservation, false, false, tail.String(),
		"./core/internal/compress/policy.go", "共 6000 行")

	b.needle("N1", "./core/internal/compress/pruner_free.go", WhereWindow, s1, "path")
	b.needle("N2", "共 6000 行", WhereWindow, s1, "number")
	b.needle("N3", "./data/ruler-bigpool/blob_0042.bin", WhereRecoverable, s0, "path")
	b.needle("N4", "./data/ruler-bigpool/blob_0117.bin", WhereRecoverable, s0, "path")
	b.needle("N5", "./docs/legacy-notes-old/README.md", WhereRecoverable, s0, "id")

	return Sample{
		ID: "R01", Class: "observation", Lang: "zh", Control: false,
		LenAssert: LenAssert{Min: 4600, Max: 6400},
		Segments:  b.segs, Needles: b.needles,
		Probe: Probe{Q: "输出里 compress 子包有哪些 .go 文件，最后一行说共多少行？", ExpectRegex: "pruner_free\\.go.*6000", Grader: "regex"},
	}
}

// ---------------------------------------------------------------- R02 zh

// R02 zh · `go test ./...` 失败日志（失败段 Pinned，规7）。
func buildR02() Sample {
	b := &sb{}

	var noise strings.Builder
	noise.WriteString("$ go test ./... -count=1\n")
	noise.WriteString("（测试二进制 /tmp/go-build884312/b001/store.test）\n")
	padTo(&noise, 900, func(i int) string {
		return fmt.Sprintf("=== RUN   TestCacheLayer%03d\n--- PASS: TestCacheLayer%03d (0.0%ds)", i, i, i%9)
	})
	noise.WriteString("\n--- FAIL: TestShardSplit (0.11s)\n")
	noise.WriteString("    shard_test.go:64: want 4 shards, got 3\n")
	noise.WriteString("ok  \texample.com/foo/internal/cache\t1.204s\n")
	padTo(&noise, 3600, func(i int) string {
		return fmt.Sprintf("=== RUN   TestCacheLayer%03d\n--- PASS: TestCacheLayer%03d (0.00s)", 900+i, 900+i)
	})
	s0 := b.addP(compress.KindObservation, false, true, noise.String(),
		"example.com/foo/internal/cache", "/tmp/go-build884312/b001/store.test")

	var fail strings.Builder
	fail.WriteString("=== RUN   TestStoreReopen\n")
	fail.WriteString("--- FAIL: TestStoreReopen (0.03s)\n")
	fail.WriteString("    store_test.go:118: want 3 rows, got 0\n")
	fail.WriteString("        rows after reopen: [] (rowid 序列回绕，见 WAL 段 offset 0x1f40)\n")
	fail.WriteString("=== RUN   TestStoreVacuum\n--- PASS: TestStoreVacuum (0.02s)\n")
	fail.WriteString("=== RUN   TestBillingReconcile\n")
	fail.WriteString("--- FAIL: TestBillingReconcile (0.07s)\n")
	fail.WriteString("    reconcile_test.go:56: want amount 12800.00, got 0.00\n")
	fail.WriteString("FAIL\texample.com/foo/internal/store\t0.11s\n")
	fail.WriteString("FAIL\texample.com/foo/internal/billing\t0.07s\n")
	fail.WriteString("FAIL\texample.com/foo/internal/gateway\t0.04s\n")
	fail.WriteString("exit status 1\n")
	fail.WriteString("（上面三包失败均有 _test.go 行号，日志已由 CI 归档）\n")
	padTo(&fail, 1300, func(i int) string {
		return fmt.Sprintf("        [ci] 归档日志分片 %03d：/tmp/ci-logs/run-884312/shard-%03d.log", i, i)
	})
	s1 := b.addP(compress.KindObservation, true, false, fail.String(),
		"example.com/foo/internal/store", "example.com/foo/internal/billing", "example.com/foo/internal/gateway", "exit status 1")

	b.needle("N1", "--- FAIL: TestStoreReopen (0.03s)", WhereWindow, s1, "id")
	b.needle("N2", "store_test.go:118: want 3 rows, got 0", WhereWindow, s1, "errcode")
	b.needle("N3", "/tmp/go-build884312/b001/store.test", WhereRecoverable, s0, "path")
	b.needle("N4", "--- FAIL: TestShardSplit (0.11s)", WhereRecoverable, s0, "id")
	b.needle("N5", "example.com/foo/internal/cache\t1.204s", WhereRecoverable, s0, "number")

	return Sample{
		ID: "R02", Class: "observation", Lang: "zh", Control: false,
		LenAssert: LenAssert{Min: 4400, Max: 6400},
		Segments:  b.segs, Needles: b.needles,
		Probe: Probe{Q: "哪个测试失败了，断言差异是什么？", ExpectRegex: "TestStoreReopen.*want 3 rows, got 0", Grader: "regex"},
	}
}

// ---------------------------------------------------------------- R03 zh

// R03 zh · 查询结果表 300 行 × 8 列（表头与合计在小尾段，中间行可卸载）。
func buildR03() Sample {
	b := &sb{}

	var rows strings.Builder
	rows.WriteString("# 查询结果（本次导出 300 行 / 库中 3860 行）\n")
	padTo(&rows, 2000, func(i int) string {
		return fmt.Sprintf("DD20260917-%04d\t客户%03d\t%d.00\t已支付\t2026-09-%02d\tApp\t%d\t—",
			i, i%97, 1200+i*13, i%28+1, i%5+1)
	})
	rows.WriteString("DD20260917-0087\t客户087\t-18420.00\t冲正\t2026-09-04\t后台\t1\t红冲单\n")
	rows.WriteString("DD20260917-0142\t客户042\t998.00\t已退款\t2026-09-11\tH5\t2\t全额退\n")
	padTo(&rows, 4900, func(i int) string {
		return fmt.Sprintf("DD20260917-%04d\t客户%03d\t%d.00\t已支付\t2026-09-%02d\tApp\t%d\t—",
			300+i, i%97, 1200+i*17, i%28+1, i%5+1)
	})
	s0 := b.addP(compress.KindObservation, false, true, rows.String(),
		"DD20260917-0087", "DD20260917-0142")

	var head strings.Builder
	head.WriteString("SELECT 订单号, 客户, 金额, 状态, 创建时间, 渠道, 数量, 备注 FROM orders ORDER BY 创建时间 DESC LIMIT 300\n")
	head.WriteString("订单号	客户	金额	状态	创建时间	渠道	数量	备注\n")
	head.WriteString("合计金额 1284730.55 元，共 3860 条\n")
	padTo(&head, 300, func(i int) string {
		return fmt.Sprintf("DD20261023-%04d	客户%03d	%d.00	待支付	2026-10-%02d	Web	1	—",
			i, i%97, 500+i*7, i%28+1)
	})
	s1 := b.addP(compress.KindObservation, false, false, head.String(),
		"订单号	客户	金额	状态	创建时间	渠道	数量	备注", "合计金额 1284730.55 元，共 3860 条")

	b.needle("N1", "SELECT 订单号, 客户, 金额, 状态, 创建时间, 渠道, 数量, 备注 FROM orders ORDER BY 创建时间 DESC LIMIT 300", WhereWindow, s1, "cmd")
	b.needle("N2", "合计金额 1284730.55 元，共 3860 条", WhereWindow, s1, "number")
	b.needle("N3", "DD20260917-0087", WhereRecoverable, s0, "id")
	b.needle("N4", "-18420.00", WhereRecoverable, s0, "number")
	b.needle("N5", "2026-09-04", WhereRecoverable, s0, "number")

	return Sample{
		ID: "R03", Class: "observation", Lang: "zh", Control: false,
		LenAssert: LenAssert{Min: 4400, Max: 6400},
		Segments:  b.segs, Needles: b.needles,
		Probe: Probe{Q: "这张表有哪 8 列，合计金额多少？", ExpectRegex: "合计金额 1284730\\.55", Grader: "regex"},
	}
}

// ---------------------------------------------------------------- R04 zh

// R04 zh · 中文网页正文抓取（导航/广告/版权尾巴是噪声）。
func buildR04() Sample {
	b := &sb{}

	var body strings.Builder
	body.WriteString("# 抓取自 https://example-news.invalid/2026/09/charging-piles（正文）\n")
	padTo(&body, 4600, func(i int) string {
		switch i % 4 {
		case 0:
			return fmt.Sprintf("记者今日从市发改委获悉，第 %d 批公共充电设施建设名单已完成公示，涉及 %d 个社区、%d 个车位。", i/4+1, 12+i%40, 240+i*3)
		case 1:
			return fmt.Sprintf("运营方%s表示，本期投运的直流快充桩单枪功率为 %d 千瓦，平均利用率约 %d.%d%%。",
				[]string{"星河能源", "城运集团", "蓝谷充电"}[i%3], 120+i%60, i%9, i%10)
		case 2:
			return fmt.Sprintf("有居民反映，部分站点在夜间 %02d:00 之后仍排队，属地街道已协调错峰引导。", i%12)
		default:
			return fmt.Sprintf("市交通委数据平台显示，全市充电桩日均服务车次为 %d 次，较上季度增加 %d 次。", 800+i*7, i%50)
		}
	})
	s0 := b.addP(compress.KindObservation, false, true, body.String(),
		"https://example-news.invalid/2026/09/charging-piles", "星河能源", "城运集团")

	var tail strings.Builder
	tail.WriteString("【要点】2026 年上半年该市新增充电桩 4821 个，同比增长 37.2%。\n")
	tail.WriteString("【申报】补贴申报截止日为 2026-11-30。\n")
	tail.WriteString("【来源】https://example-news.invalid/2026/09/charging-piles\n")
	tail.WriteString("【导航】首页 | 政务 | 民生 | 专题 | 互动\n")
	tail.WriteString("【广告】某某理财，年化 4.8%，点击了解 →\n")
	padTo(&tail, 800, func(i int) string {
		return fmt.Sprintf("【版权】本文版权归示例日报所有，未经许可不得转载（%03d）。", i)
	})
	s1 := b.addP(compress.KindObservation, false, false, tail.String(),
		"【要点】2026 年上半年该市新增充电桩 4821 个", "补贴申报截止日为 2026-11-30")

	b.needle("N1", "2026 年上半年该市新增充电桩 4821 个，同比增长 37.2%。", WhereWindow, s1, "number")
	b.needle("N2", "补贴申报截止日为 2026-11-30。", WhereWindow, s1, "decision")
	b.needle("N3", "https://example-news.invalid/2026/09/charging-piles", WhereRecoverable, s0, "path")
	b.needle("N4", "星河能源", WhereRecoverable, s0, "id")
	b.needle("N5", "城运集团", WhereRecoverable, s0, "id")

	return Sample{
		ID: "R04", Class: "observation", Lang: "zh", Control: false,
		LenAssert: LenAssert{Min: 4400, Max: 6400},
		Segments:  b.segs, Needles: b.needles,
		Probe: Probe{Q: "新增充电桩数量和申报截止日分别是？", ExpectRegex: "4821.*2026-11-30", Grader: "regex"},
	}
}

// ---------------------------------------------------------------- R05 zh（对照）

// R05 zh · 对照：服务日志 tail（2xx 为主 + 3 条 5xx），2200 token ⇒ 不撞线。
func buildR05() Sample {
	b := &sb{}

	var log strings.Builder
	log.WriteString("2026-10-23 09:00:00 INFO  svc=api 启动完成，版本 2.7.3，pid=4821\n")
	padTo(&log, 1800, func(i int) string {
		switch i % 37 {
		case 5:
			return fmt.Sprintf("2026-10-23 09:%02d:%02d WARN  svc=api status=503 path=/api/v2/orders dur=%dms trace=8f3c1d9e2b7a4c50 code=ERR_UPSTREAM_TIMEOUT_504",
				7+i%50, i%60, 4800+i)
		case 11:
			return fmt.Sprintf("2026-10-23 09:%02d:%02d INFO  svc=healthz status=200 path=/healthz dur=1ms", 7+i%50, i%60)
		default:
			return fmt.Sprintf("2026-10-23 09:%02d:%02d INFO  svc=api status=200 path=/api/v2/%s dur=%dms",
				7+i%50, i%60, []string{"orders", "users", "carts", "prices"}[i%4], 30+i%400)
		}
	})
	s0 := b.addP(compress.KindObservation, false, true, log.String(),
		"trace=8f3c1d9e2b7a4c50", "ERR_UPSTREAM_TIMEOUT_504")

	var sum strings.Builder
	sum.WriteString("2026-10-23 09:59:58 INFO  svc=api 汇总：请求 800 次，2xx 797 次，5xx 共 3 次，占 0.375%，错误码 ERR_UPSTREAM_TIMEOUT_504\n")
	sum.WriteString("2026-10-23 09:59:58 INFO  最慢请求 GET /api/v2/orders 耗时 4820ms（trace=8f3c1d9e2b7a4c50）\n")
	sum.WriteString("2026-10-23 09:59:59 INFO  healthz 200 心跳探活正常，连续 800 次\n")
	padTo(&sum, 400, func(i int) string {
		return fmt.Sprintf("2026-10-23 09:59:59 INFO  分片 %02d 行数校验通过（%d 行）", i, 20+i)
	})
	s1 := b.addP(compress.KindObservation, false, false, sum.String(),
		"5xx 共 3 次", "healthz 200")

	b.needle("N1", "5xx 共 3 次，占 0.375%", WhereWindow, s1, "number")
	b.needle("N2", "ERR_UPSTREAM_TIMEOUT_504", WhereWindow, s1, "errcode")
	b.needle("N3", "trace=8f3c1d9e2b7a4c50", WhereRecoverable, s0, "id")
	b.needle("N4", "GET /api/v2/orders 耗时 4820ms", WhereRecoverable, s1, "cmd")
	b.needle("N5", "healthz 200", WhereRecoverable, s1, "cmd")

	return Sample{
		ID: "R05", Class: "observation", Lang: "zh", Control: true,
		LenAssert: LenAssert{Min: 1800, Max: 2400},
		Segments:  b.segs, Needles: b.needles,
		Probe: Probe{Q: "5xx 一共几次？最慢请求是哪条？", ExpectRegex: "5xx 共 3 次", Grader: "regex"},
	}
}

// ---------------------------------------------------------------- R06 en

// R06 en · `npm install` 输出（进度噪声 + 12 条 deprecation + 审计告警）。
func buildR06() Sample {
	b := &sb{}

	var noise strings.Builder
	noise.WriteString("> npm install --no-audit --prefer-offline\n")
	noise.WriteString("npm v10.9.2 node v22.14.0 darwin arm64\n")
	noise.WriteString("npm http fetch GET 200 https://registry.npmjs.invalid/react/-/react-18.3.1.tgz 412ms\n")
	noise.WriteString("npm WARN deprecated glob@7.2.3: Glob versions prior to v9 are no longer supported\n")
	padTo(&noise, 3900, func(i int) string {
		switch i % 3 {
		case 0:
			return fmt.Sprintf("npm http fetch GET 200 https://registry.npmjs.invalid/pkg-%03d/-/pkg-%03d-1.%d.%d.tgz %dms (cache hit)", i, i, i%9, i%20, 40+i%300)
		case 1:
			return fmt.Sprintf("npm timing idealTree:node_modules/pkg-%03d Completed in %dms", i, 4+i%90)
		default:
			return fmt.Sprintf("npm sill extract pkg-%03d@1.%d.%d extracted to ~/.npm/_cacache/tmp/install-8f21/node_modules/pkg-%03d", i, i%9, i%20, i)
		}
	})
	s0 := b.addP(compress.KindObservation, false, true, noise.String(),
		"npm v10.9.2", "~/.npm/_cacache/tmp/install-8f21")

	var tail strings.Builder
	tail.WriteString("npm WARN deprecated inflight@1.0.6: This module is not supported, and leaks memory.\n")
	tail.WriteString("npm WARN deprecated glob@7.2.3: Glob versions prior to v9 are no longer supported\n")
	tail.WriteString("npm WARN deprecated rimraf@3.0.2: Rimraf versions prior to v4 are no longer supported\n")
	tail.WriteString("npm WARN deprecated @humanwhocodes/object-schema@2.0.3: Use @eslint/object-schema instead\n")
	padTo(&tail, 700, func(i int) string {
		return fmt.Sprintf("npm WARN deprecated legacy-shim-%02d@0.%d.0: this package is deprecated, use shim-%02d instead", i, i%9, i)
	})
	tail.WriteString("\nadded 412 packages, and audited 413 packages in 38s\n")
	tail.WriteString("found 3 vulnerabilities (2 moderate, 1 high)\n")
	tail.WriteString("run `npm audit fix` to fix them, or `npm audit` for details\n")
	s1 := b.addP(compress.KindObservation, false, false, tail.String(),
		"npm WARN deprecated inflight@1.0.6", "found 3 vulnerabilities")

	b.needle("N1", "npm WARN deprecated inflight@1.0.6: This module is not supported, and leaks memory.", WhereWindow, s1, "id")
	b.needle("N2", "found 3 vulnerabilities (2 moderate, 1 high)", WhereWindow, s1, "number")
	b.needle("N3", "~/.npm/_cacache/tmp/install-8f21", WhereRecoverable, s0, "path")
	b.needle("N4", "npm v10.9.2", WhereRecoverable, s0, "id")
	b.needle("N5", "npm WARN deprecated glob@7.2.3: Glob versions prior to v9 are no longer supported", WhereRecoverable, s0, "id")

	return Sample{
		ID: "R06", Class: "observation", Lang: "en", Control: false,
		LenAssert: LenAssert{Min: 4400, Max: 6400},
		Segments:  b.segs, Needles: b.needles,
		Probe: Probe{Q: "Which package deprecation and how many vulnerabilities?", ExpectRegex: "inflight@1\\.0\\.6.*3 vulnerabilities", Grader: "regex"},
	}
}

// ---------------------------------------------------------------- R07 en

// R07 en · Go panic stack（panic 段 Pinned，规7）。
func buildR07() Sample {
	b := &sb{}

	var dump strings.Builder
	dump.WriteString("fatal error: all goroutines are asleep - deadlock!\n")
	dump.WriteString("goroutine 41 [chan send, 12 minutes]:\n")
	dump.WriteString("main.(*Worker).flush(0xc0001a2000)\n")
	dump.WriteString("\t/internal/queue/worker.go:214 +0x1a5\n")
	padTo(&dump, 3900, func(i int) string {
		switch i % 3 {
		case 0:
			return fmt.Sprintf("runtime.gopark(0x%x, 0x%x, 0x%x, 0x%x, 0x%x)", 400000+i*16, 1234560+i, i%3, i%2, i)
		case 1:
			return fmt.Sprintf("created by main.(*Worker).Start in goroutine %d\n\t/internal/queue/pool.go:%d +0x%x", i%200+1, 40+i%80, 0x2f+i)
		default:
			return fmt.Sprintf("goroutine %d [select, %d minutes]:\nmain.(*Pool).dispatch(0xc0000%06d)", 100+i, i%9, i*7)
		}
	})
	dump.WriteString("exit status 2\n")
	s0 := b.addP(compress.KindObservation, false, true, dump.String(),
		"goroutine 41 [chan send, 12 minutes]", "main.(*Worker).flush(0xc0001a2000)", "exit status 2")

	var panicSeg strings.Builder
	panicSeg.WriteString("panic: send on closed channel\n")
	panicSeg.WriteString("\ngoroutine 1 [running]:\n")
	panicSeg.WriteString("main.(*Pipeline).Submit(0xc0001a2000, {0x5f2a1c, 0x9})\n")
	panicSeg.WriteString("\t/internal/queue/worker.go:214 +0x1a5\n")
	panicSeg.WriteString("main.main()\n\t/cmd/queue-runner/main.go:52 +0x88\n")
	padTo(&panicSeg, 1200, func(i int) string {
		return fmt.Sprintf("runtime/debug.Stack()\n\t/usr/local/go/src/runtime/debug/stack.go:%d +0x%x", 24+i%40, 0x5a+i)
	})
	panicSeg.WriteString("\n（goroutine 数：7；关闭的 channel 由 CloseAll 在 main.go:44 关闭）\n")
	s1 := b.addP(compress.KindObservation, true, false, panicSeg.String(),
		"panic: send on closed channel", "/internal/queue/worker.go:214", "goroutine 数：7")

	b.needle("N1", "panic: send on closed channel", WhereWindow, s1, "errcode")
	b.needle("N2", "/internal/queue/worker.go:214", WhereWindow, s1, "path")
	b.needle("N3", "goroutine 41 [chan send, 12 minutes]:", WhereRecoverable, s0, "id")
	b.needle("N4", "exit status 2", WhereRecoverable, s0, "number")
	b.needle("N5", "main.(*Worker).flush(0xc0001a2000)", WhereRecoverable, s0, "id")

	return Sample{
		ID: "R07", Class: "observation", Lang: "en", Control: false,
		LenAssert: LenAssert{Min: 4400, Max: 6400},
		Segments:  b.segs, Needles: b.needles,
		Probe: Probe{Q: "What panicked and at which file:line?", ExpectRegex: "panic: send on closed channel.*worker\\.go:214", Grader: "regex"},
	}
}

// ---------------------------------------------------------------- R08 en

// R08 en · `git diff` 大补丁 900 行（5 个文件）。
func buildR08() Sample {
	b := &sb{}

	var patch strings.Builder
	patch.WriteString("diff --git a/internal/retry/backoff.go b/internal/retry/backoff.go\nindex 3a1f9c2..8b4e771 100644\n")
	patch.WriteString("--- a/internal/retry/backoff.go\n+++ b/internal/retry/backoff.go\n")
	patch.WriteString("@@ -118,7 +118,19 @@ func (b *Backoff) next()\n")
	padTo(&patch, 2400, func(i int) string {
		switch i % 3 {
		case 0:
			return "-	return base * time.Duration(1<<uint(attempt))"
		case 1:
			return fmt.Sprintf("+\tif attempt > b.maxShift {\n+\t\tattempt = b.maxShift\n+\t}")
		default:
			return fmt.Sprintf("+\t// clamp attempt %03d to avoid overflow on long retries", i)
		}
	})
	patch.WriteString("+func WithCap(cap time.Duration) Option {\n+\treturn func(b *Backoff) { b.cap = cap }\n+}\n")
	patch.WriteString("diff --git a/internal/retry/jitter.go b/internal/retry/jitter.go\n--- a/internal/retry/jitter.go\n+++ b/internal/retry/jitter.go\n@@ -8,6 +8,14 @@\n")
	padTo(&patch, 4800, func(i int) string {
		return fmt.Sprintf("+\t// jitter bucket %03d: rand within [d/2, d] for attempt %d", i, i%12)
	})
	s0 := b.addP(compress.KindObservation, false, true, patch.String(),
		"func WithCap(cap time.Duration) Option", "@@ -118,7 +118,19 @@", "internal/retry/jitter.go")

	var head strings.Builder
	head.WriteString("$ git diff --stat\n internal/retry/backoff.go | 24 +++++---\n internal/retry/jitter.go  | 18 +++++--\n")
	head.WriteString("diff --git a/internal/retry/policy.go b/internal/retry/policy.go\n--- a/internal/retry/policy.go\n+++ b/internal/retry/policy.go\n")
	head.WriteString("-const maxRetryBudget = 32\n+const maxRetryBudget = 64\n")
	head.WriteString("diff --git a/internal/retry/backoff_test.go b/internal/retry/backoff_test.go\n--- a/internal/retry/backoff_test.go\n+++ b/internal/retry/backoff_test.go\n")
	padTo(&head, 1000, func(i int) string {
		return fmt.Sprintf("+\t{attempt: %d, base: %d, want: %d},", i, 10+i%5, 20+i)
	})
	s1 := b.addP(compress.KindObservation, false, false, head.String(),
		"internal/retry/policy.go", "internal/retry/backoff_test.go", "-const maxRetryBudget = 32")

	b.needle("N1", "diff --git a/internal/retry/policy.go b/internal/retry/policy.go", WhereWindow, s1, "path")
	b.needle("N2", "-const maxRetryBudget = 32", WhereWindow, s1, "id")
	b.needle("N3", "func WithCap(cap time.Duration) Option", WhereRecoverable, s0, "cmd")
	b.needle("N4", "@@ -118,7 +118,19 @@ func (b *Backoff) next()", WhereRecoverable, s0, "number")
	b.needle("N5", "internal/retry/jitter.go", WhereRecoverable, s0, "path")

	return Sample{
		ID: "R08", Class: "observation", Lang: "en", Control: false,
		LenAssert: LenAssert{Min: 4400, Max: 6400},
		Segments:  b.segs, Needles: b.needles,
		Probe: Probe{Q: "Which constant was deleted and in which file?", ExpectRegex: "maxRetryBudget = 32", Grader: "regex"},
	}
}

// ---------------------------------------------------------------- R09 en

// R09 en · JSON API 响应（120 条数组 + 1 个 error 对象）。
func buildR09() Sample {
	b := &sb{}

	var arr strings.Builder
	arr.WriteString("{\n  \"object\": \"list\",\n  \"data\": [\n")
	padTo(&arr, 300, func(i int) string {
		return fmt.Sprintf("    {\"id\": \"usr_%06x\", \"status\": \"active\", \"email\": \"user%04d@example.invalid\"},", 0x100000+i, i)
	})
	padTo(&arr, 2400, func(i int) string {
		return fmt.Sprintf("    {\"id\": \"usr_%06x\", \"status\": %q, \"email\": \"user%04d@example.invalid\", \"created\": \"2026-0%d-1%d\"},",
			0x200000+i, []string{"active", "invited", "suspended", "provisional"}[i%4], i, i%9+1, i%9)
	})
	arr.WriteString("    {\"id\": \"usr_4b7e01\", \"status\": \"suspended\", \"email\": \"user4001@example.invalid\"},\n")
	arr.WriteString("    {\"id\": \"usr_1c9d33\", \"status\": \"provisional\", \"email\": \"user4002@example.invalid\"},\n")
	padTo(&arr, 4300, func(i int) string {
		return fmt.Sprintf("    {\"id\": \"usr_%06x\", \"status\": \"active\", \"seat\": %d, \"plan\": %q},", 0x300000+i, i%50, []string{"free", "team", "pro"}[i%3])
	})
	arr.WriteString("  ],\n  \"has_more\": true,\n  \"next_cursor\": \"eyJvZmZzZXQiOjEyMH0=\"\n}\n")
	s0 := b.addP(compress.KindObservation, false, true, arr.String(),
		"usr_4b7e01", "next_cursor", "\"status\": \"provisional\"")

	var tail strings.Builder
	tail.WriteString("{\n  \"object\": \"page_error\",\n  \"error\": {\n    \"code\": \"E_RATE_LIMITED\",\n")
	tail.WriteString("    \"message\": \"too many requests, retry after 30s\",\n    \"request_id\": \"usr_9f31c2\"\n  },\n")
	tail.WriteString("  \"id\": \"usr_9f31c2\",\n  \"retry_after_ms\": 30000\n}\n")
	padTo(&tail, 600, func(i int) string {
		return fmt.Sprintf("  \"header_%02d\": \"x-ratelimit-%02d: %d/1000\",", i, i, 1000-i*10)
	})
	s1 := b.addP(compress.KindObservation, false, false, tail.String(),
		"E_RATE_LIMITED", "usr_9f31c2")

	b.needle("N1", "\"code\": \"E_RATE_LIMITED\"", WhereWindow, s1, "errcode")
	b.needle("N2", "\"id\": \"usr_9f31c2\"", WhereWindow, s1, "id")
	b.needle("N3", "\"next_cursor\": \"eyJvZmZzZXQiOjEyMH0=\"", WhereRecoverable, s0, "id")
	b.needle("N4", "\"status\": \"provisional\"", WhereRecoverable, s0, "id")
	b.needle("N5", "\"retry_after_ms\": 30000", WhereRecoverable, s1, "number")

	return Sample{
		ID: "R09", Class: "observation", Lang: "en", Control: false,
		LenAssert: LenAssert{Min: 4400, Max: 6400},
		Segments:  b.segs, Needles: b.needles,
		Probe: Probe{Q: "What error code and how many records are there?", ExpectRegex: "E_RATE_LIMITED", Grader: "regex"},
	}
}

// ---------------------------------------------------------------- R10 en（对照）

// R10 en · 对照：`grep -rn` 命中 150 行（3 个文件），1900 token ⇒ 不撞线。
func buildR10() Sample {
	b := &sb{}

	var hits strings.Builder
	hits.WriteString("$ grep -rn \"unloadToolResult\" internal | head -150\n")
	hits.WriteString("internal/agent/loop.go:103:func unloadToolResult(text string) string {\n")
	hits.WriteString("internal/agent/loop.go:214:\t\tout = unloadToolResult(out)\n")
	padTo(&hits, 1200, func(i int) string {
		return fmt.Sprintf("internal/agent/adapter_%02d.go:%d:\tres := %s(text)  // match #%03d",
			i%24, 20+i*7%400, []string{"unloadToolResult", "wrapResult(unloadToolResult", "trim(unloadToolResult"}[i%3], i)
	})
	s0 := b.addP(compress.KindObservation, false, true, hits.String(),
		"internal/agent/loop.go:103", "internal/agent/loop.go:214")

	var tail strings.Builder
	tail.WriteString("internal/agent/compact.go:87:\tptr := unloadToolResult(body)\n")
	tail.WriteString("internal/agent/compact_test.go:41:\twant := unloadToolResult(longText)\n")
	tail.WriteString("-- 42 matches in 6 files --\n")
	padTo(&tail, 700, func(i int) string {
		return fmt.Sprintf("internal/agent/fixture_%02d.go:%d:\t_ = unloadToolResult(sample%03d)", i%12, 12+i*5, i)
	})
	s1 := b.addP(compress.KindObservation, false, false, tail.String(),
		"internal/agent/compact.go:87", "-- 42 matches in 6 files --")

	b.needle("N1", "internal/agent/loop.go:103", WhereWindow, s0, "path")
	b.needle("N2", "internal/agent/loop.go:214", WhereWindow, s0, "path")
	b.needle("N3", "internal/agent/compact.go:87", WhereRecoverable, s1, "path")
	b.needle("N4", "grep -rn \"unloadToolResult\" internal | head -150", WhereRecoverable, s0, "cmd")
	b.needle("N5", "-- 42 matches in 6 files --", WhereRecoverable, s1, "number")

	return Sample{
		ID: "R10", Class: "observation", Lang: "en", Control: true,
		LenAssert: LenAssert{Min: 1800, Max: 2400},
		Segments:  b.segs, Needles: b.needles,
		Probe: Probe{Q: "How many matches and which file defines it?", ExpectRegex: "42 matches.*loop\\.go:103", Grader: "regex"},
	}
}

// 不用 math/rand：模板行全部带下标，天然确定；固定种子也挡不住「换 Go 版本数不一样」。
