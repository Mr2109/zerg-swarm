package eval

import (
	"fmt"
	"strings"

	"github.com/Mr2109/zerg-swarm/core/internal/compress"
)

// 本文件是样本面第二类：**trajectory（对话与推理轨迹）** R11–R20（任务单 3.2 B 表）。
//
// 构造纪律（任务单 §3.1）：
//   规1 类内 zh=R11–R15 / en=R16–R20，各 5
//   规2 对照 = R15 / R20，1800–2400 token；其余 4400–6400 token
//   规4 每样本 5 条 needle = **4 window + 1 recoverable**（trajectory 配额 W40/R10）
//   规7 trajectory 钉最近 KeepRecent(2) 轮；失败/错误证据段（R13/R18）也钉
//   规8 可卸载段必给 raw_ptr（内容寻址，与 ctx_unload.go 同规）
//
// ★ 与 observation 类的关键差别：每轮对话都带一次工具调用与输出摘要 ⇒ 一条样本里
// 既有 KindTrajectory（对白）也有 KindObservation（工具输出）。PrunerFree 的单条闸与
// 余量闸**只对 KindObservation 生效**（pruner_free.go 第 ① ② 步都判 Kind），
// 因此 trajectory 类天然是「窗口里只有工具输出可卸」的形态 —— 这正是本类要量的东西：
// 轮次保护（Pinned）把结论、否决项钉住，被挤掉的是中段工具输出摘要里的细节。
//
// ★ needle 落点分三档，专门用来在 Budget 扫描里分辨「哪一档开始丢」：
//   - 钉住的最近 2 轮：任何档都不丢（硬闸面）；
//   - 中段工具输出（约 1.0k–1.3k token）：Budget ≤ 4096 时被单条闸/余量闸卸掉；
//   - 小尾巴（约 0.35k–0.45k token）：Budget = 1024 时（perOut=256）才被卸掉。
// ★ 每条 needle 在**全文里只出现一次**（TestTrajectory_Complete 断言）：观察段一律
// 「头部放决定性内容 ＋ 尾部填可变噪声」，噪声行绝不重复 needle 文本 —— 同一条信息
// 在两段各留一份，会让 Budget 扫描的「丢没丢」判读失真。

// TrajectorySamples —— trajectory 类 10 条（确定性：同一二进制两次调用逐字节相同）。
func TrajectorySamples() []Sample {
	all := []Sample{
		buildR11(), buildR12(), buildR13(), buildR14(), buildR15(),
		buildR16(), buildR17(), buildR18(), buildR19(), buildR20(),
	}
	for i := range all {
		all[i].finalize()
	}
	return all
}

// AllSamples —— 五类全量（observation 10 + trajectory 10 + plan 10 + memory 10 +
// representation 10 = 50 条，任务单 3.3 的硬配额面），扫描与全量跑批都用这一份。
func AllSamples() []Sample {
	out := make([]Sample, 0, 50)
	out = append(out, Samples()...)
	out = append(out, TrajectorySamples()...)
	out = append(out, PlanSamples()...)
	out = append(out, MemorySamples()...)
	out = append(out, RepresentationSamples()...)
	return out
}

// ClassNames —— 五类的稳定顺序（报告按这个顺序出表，不靠 map 迭代）。
var ClassNames = []string{"observation", "trajectory", "plan", "memory", "representation"}

// TwoClassSamples —— **冻结 cohort**：只有 observation R01–R10 ＋ trajectory R11–R20
// （前两笔的 20 条，顺序不变）。
//
// 用途：`core/internal/compress/pruner_free_eval_regress_test.go` 的「修前基线逐字节一致」
// 断言按这 20 条的 after/TPT 写死了基线表。样本面后来扩到 50 条，但那 20 条的历史读数
// 不该被新样本稀释或重算 —— 回归证据要冻在一个不变集合上，否则「一致」二字就没有意义。
// 新增样本 ⇒ 进 AllSamples()，**不进**这个 cohort。
func TwoClassSamples() []Sample {
	out := make([]Sample, 0, 20)
	out = append(out, Samples()...)
	out = append(out, TrajectorySamples()...)
	return out
}

// ---------------------------------------------------------------- 构造工具

var zhPool = []string{
	"用户：夜间批量跑完后第一次请求必超时，白天复现不了。",
	"助手：我先按「连接池耗尽」这条假设查 pool 指标，十分钟内回你。",
	"用户：日志贴在下面了，注意 09:41 那段有重试。",
	"助手：09:41 的重试是本进程发起的，不是上游重放。",
	"用户：要不要先把预热关掉压测一轮？",
	"助手：已关预热压测 30 分钟，超时仍在，说明不全是预热问题。",
	"用户：收到，继续。",
	"助手：补一条观测：失败请求的 p99 是 4.8s，几乎等于上游超时。",
}

var enPool = []string{
	"user: the nightly batch finishes, then the very first request times out; cannot reproduce in the morning.",
	"assistant: checking pool metrics first, under the \"pool exhaustion\" hypothesis.",
	"user: pasting the log below; note the retries around 09:41.",
	"assistant: those retries are issued by this process, not replayed by upstream.",
	"user: should we disable warmup and run a load test first?",
	"assistant: ran 30 minutes without warmup; timeouts persist, so it is not just warmup.",
	"user: acknowledged, keep going.",
	"assistant: one more datapoint: p99 for failed calls is 4.8s, which matches the upstream timeout.",
}

// dialogue —— 构造一段轮次文本（轮号 from..to 循环），填到 target token 为止；
// extra 在填充之后追加一次（needle 落在这里，保证不被生成行重复）。
func dialogue(from, to int, zh bool, target int, extra ...string) string {
	pool := zhPool
	label := "轮 %d · %s"
	if !zh {
		pool = enPool
		label = "Turn %d · %s"
	}
	var b strings.Builder
	span := to - from + 1
	if span <= 0 {
		span = 1
	}
	for i := 0; compress.EstimateTokens(b.String()) < target; i++ {
		r := from + i%span
		fmt.Fprintf(&b, label+"\n", r, pool[i%len(pool)])
		if i%3 == 2 { // 每隔两轮插一条工具调用摘要（真实轨迹的形态）
			if zh {
				fmt.Fprintf(&b, "  ↳ 工具 cache.stat 返回 %d 项，耗时 %dms\n", 12+i, 3+i%40)
			} else {
				fmt.Fprintf(&b, "  -> tool cache.stat returned %d entries in %dms\n", 12+i, 3+i%40)
			}
		}
	}
	for _, e := range extra {
		b.WriteString(e)
		b.WriteByte('\n')
	}
	return b.String()
}

// obsBlock —— 工具输出段：head 先落「决定性内容」（needle 只能出现在这里），
// 再用 line(i) 生成可变噪声填到 target。噪声行带 i，天然不重复 head。
func obsBlock(target int, head []string, line func(i int) string) string {
	var b strings.Builder
	for _, h := range head {
		b.WriteString(h)
		b.WriteByte('\n')
	}
	padTo(&b, target, line)
	return b.String()
}

// ---------------------------------------------------------------- R11 zh

// R11 zh · 多轮调试对话 12 轮（现象→假设→验证→结论），中段是重复日志粘贴。
func buildR11() Sample {
	b := &sb{}

	b.addP(compress.KindTrajectory, false, false, dialogue(1, 4, true, 430,
		"助手：现象与三条候选假设（连接池耗尽 / 预热未完成 / 时钟漂移）已记录，开始逐条验证。"))

	s1 := b.addP(compress.KindObservation, false, true, obsBlock(2150, []string{
		"$ go test ./internal/cache -run TestWarmFork -count=1",
		"# 上面这条命令在 12 轮对话里被重复粘贴了 6 遍，下面是其中一遍的完整 debug 日志",
	}, func(i int) string {
		return fmt.Sprintf("2026-10-23T09:%02d:%02dZ cache[%d] level=debug msg=\"warm start\" key=user:%05d dur=%dms retry=%t",
			20+i%40, i%60, 4000+i%7, i, 3+i%90, i%11 == 0)
	}), "cache[4000]", "msg=\"warm start\"")

	b.addP(compress.KindTrajectory, false, false, dialogue(5, 9, true, 700,
		"助手：预热假设已被否决——关掉 warm 之后超时率从 3.1% 只降到 2.7%。"))

	s3 := b.addP(compress.KindObservation, false, true, obsBlock(1150, []string{
		"# 假设二验证结论：时钟漂移导致 TTL 早过期 —— 否",
		"# 证据：clock_offset_ms=0（ntp 同步，最大漂移 2ms），不足以解释 4.8s 超时",
	}, func(i int) string {
		switch i % 3 {
		case 0:
			return fmt.Sprintf("grep -c \"TTL expired during fork\" /var/log/cache/warm-%02d.log → %d", i%12, i%3)
		case 1:
			return fmt.Sprintf("clock_offset_ms=0 sample=%03d ntp_source=10.0.%d.%d", i, i%40, i%250)
		default:
			return fmt.Sprintf("pool.wait_ms p50=%dms p95=%dms in_flight=%d", 1+i%9, 20+i%40, i%16)
		}
	}), "clock_offset_ms=0", "pool.wait_ms p50=")

	s4 := b.addP(compress.KindObservation, false, true, obsBlock(380, []string{
		"$ git log --oneline -1 → a3f91c2 fix(cache): release mu before fork in warm()",
	}, func(i int) string {
		return fmt.Sprintf("  (附带 %02d 条分支提示：origin/fix/warm-fork-%d ahead 1)", i, i)
	}), "git log --oneline")

	s5 := b.addP(compress.KindTrajectory, true, false, dialogue(10, 12, true, 540,
		"助手：根因：warm() 在 fork 前就持了 mu，子进程继承锁后第一次写必然等到 TTL 过期。",
		"助手：否决假设一：连接池耗尽 —— pool.wait_ms 的 p95 只有 41ms，够不到 4.8s 超时。",
		"用户：结论落到提交上，别再翻旧假设了。"))

	b.needle("N1", "根因：warm() 在 fork 前就持了 mu", WhereWindow, s5, "decision")
	b.needle("N2", "否决假设一：连接池耗尽", WhereWindow, s5, "decision")
	b.needle("N3", "时钟漂移导致 TTL 早过期", WhereWindow, s3, "decision")
	b.needle("N4", "a3f91c2", WhereWindow, s4, "id")
	b.needle("N5", "go test ./internal/cache -run TestWarmFork -count=1", WhereRecoverable, s1, "cmd")

	return Sample{
		ID: "R11", Class: "trajectory", Lang: "zh", Control: false,
		LenAssert: LenAssert{Min: 4400, Max: 6400},
		Segments:  b.segs, Needles: b.needles,
		Probe: Probe{Q: "根因是什么，哪条假设被否决了？", ExpectRegex: "warm\\(\\).*mu.*连接池耗尽", Grader: "regex"},
	}
}

// ---------------------------------------------------------------- R12 zh

// R12 zh · 需求澄清往返 10 轮，用户改口 3 次（早期表述标「已废弃」）。
func buildR12() Sample {
	b := &sb{}

	b.addP(compress.KindTrajectory, false, false, dialogue(1, 3, true, 470,
		"用户：第一版想法：导出改成实时推送，前端就不用轮询了。",
		"助手：已记下（第 1 次改口前表述），先按实时推送做评估。"))

	s1 := b.addP(compress.KindObservation, false, true, obsBlock(2080, []string{
		"# 会议纪要摘录（第 2 次改口后整理）",
		"# 已废弃：导出改成实时推送（用户第 2 次改口时明确撤回，理由：运维面太重）",
	}, func(i int) string {
		switch i % 4 {
		case 0:
			return fmt.Sprintf("会议纪要 %02d：用户提到「推送要能断线重连」，语气不确定。", i)
		case 1:
			return fmt.Sprintf("会议纪要 %02d：运营希望保留按天分片的历史回溯。", i)
		case 2:
			return fmt.Sprintf("会议纪要 %02d：财务只关心计费口径不变。", i)
		default:
			return fmt.Sprintf("会议纪要 %02d：下次评审待定项 %d 条。", i, i%5)
		}
	}), "会议纪要 00", "断线重连")

	b.addP(compress.KindTrajectory, false, false, dialogue(4, 7, true, 600,
		"助手：第 2 次改口：实时推送改成先做只读导出，写路径下一版。",
		"用户：对，先把导出跑通。"))

	s3 := b.addP(compress.KindObservation, false, true, obsBlock(1040, []string{
		"# 需求草稿 v3（终稿候选）：导出文件按天分片（PRD-2.5.15-07）；期限 2026-11-15",
	}, func(i int) string {
		switch i % 3 {
		case 0:
			return fmt.Sprintf("需求草稿 v3 附注 %02d：按天分片后单天可单独重跑。", i)
		case 1:
			return fmt.Sprintf("需求草稿 v3 附注 %02d：批量任务支持断点续跑，续跑点落 state.md。", i)
		default:
			return fmt.Sprintf("需求草稿 v3 附注 %02d：评审排期已确认，待开发认领。", i)
		}
	}), "需求草稿 v3", "state.md")

	s4 := b.addP(compress.KindObservation, false, true, obsBlock(410, []string{
		"# 改口记录 3/3：不要做：不改动 scripts/billing/reconcile.go 的计费口径",
	}, func(i int) string {
		return fmt.Sprintf("改口记录 %02d：用户强调「这条是硬约束，评审会复述一遍」。", i)
	}), "scripts/billing/reconcile.go")

	s5 := b.addP(compress.KindTrajectory, true, false, dialogue(8, 10, true, 540,
		"助手：终稿需求一：批量任务必须支持断点续跑，续跑点在 state.md 里落盘。",
		"助手：终稿需求二：导出文件按天分片，按天可单独重跑；PRD-2.5.15-07 已同步。",
		"用户：按这两条定稿，其余旧表述一律作废。"))

	b.needle("N1", "终稿需求一：批量任务必须支持断点续跑", WhereWindow, s5, "decision")
	b.needle("N2", "终稿需求二：导出文件按天分片", WhereWindow, s5, "decision")
	b.needle("N3", "期限 2026-11-15", WhereWindow, s3, "number")
	b.needle("N4", "不要做：不改动 scripts/billing/reconcile.go 的计费口径", WhereWindow, s4, "path")
	b.needle("N5", "已废弃：导出改成实时推送", WhereRecoverable, s1, "decision")

	return Sample{
		ID: "R12", Class: "trajectory", Lang: "zh", Control: false,
		LenAssert: LenAssert{Min: 4400, Max: 6400},
		Segments:  b.segs, Needles: b.needles,
		Probe: Probe{Q: "终稿的两条需求和期限是什么？", ExpectRegex: "断点续跑.*按天分片.*2026-11-15", Grader: "regex"},
	}
}

// ---------------------------------------------------------------- R13 zh

// R13 zh · 工具调用链 20 步，含 3 次失败重试（失败段 Pinned，规7）。
func buildR13() Sample {
	b := &sb{}

	b.addP(compress.KindTrajectory, false, false, dialogue(1, 6, true, 470,
		"助手：打包链路上前 6 步都是成功步骤，完整入参见下面的日志段。"))

	s1 := b.addP(compress.KindObservation, false, true, obsBlock(2350, []string{
		"# 打包链路公共超时设置：timeout=90s（每一步都沿用，失败时不会自动加长）",
	}, func(i int) string {
		switch i % 3 {
		case 0:
			return fmt.Sprintf("step %02d ok  cmd=\"zerg stage --input build/%d --out stage/%d\" dur=%dms", i%20+1, i%5, i%5, 40+i%300)
		case 1:
			return fmt.Sprintf("step %02d ok  cmd=\"zerg sign --key test-key-%02d --in stage/%d\" dur=%dms", i%20+1, i%9, i%5, 30+i%200)
		default:
			return fmt.Sprintf("step %02d ok  stdout=%d bytes  retries=0", i%20+1, 1024+i*7)
		}
	}), "timeout=90s", "zerg sign")

	s2 := b.addP(compress.KindObservation, true, false, obsBlock(700, []string{
		"step 07 FAIL exit status 2: missing target dir (build/target-a 不存在)",
		"step 11 FAIL error=EZIPARTIAL detail=\"archive truncated at offset 0x1f40\"",
		"step 14 FAIL exit status 2: target dir vanished after retry (同一错误码，第三次)",
	}, func(i int) string {
		return fmt.Sprintf("  [ci] 归档日志分片 %03d：/tmp/ci-logs/run-884312/shard-%03d.log", i, i)
	}), "exit status 2", "EZIPARTIAL")

	s3 := b.addP(compress.KindObservation, false, true, obsBlock(1250, []string{
		"# 第 18 步起改用下面这条命令后一路成功：",
		"$ zerg pack --out dist/app.tgz --threads 4",
	}, func(i int) string {
		switch i % 2 {
		case 0:
			return fmt.Sprintf("step %02d ok  phase=final dur=%dms", i%20+15, 800+i%900)
		default:
			return fmt.Sprintf("step %02d ok  artifact=dist/part-%02d.bin size=%d", i%20+15, i%9, 4096+i*3)
		}
	}), "zerg pack --out dist/app.tgz --threads 4")

	s4 := b.addP(compress.KindObservation, false, true, obsBlock(380, []string{
		"# 产物清单（首行即总清单入口）：dist/manifest-2.5.15.json",
	}, func(i int) string {
		return fmt.Sprintf("  part-%02d.bin sha256=9c1e%04x bytes=%d", i, 4096+i, 4096+i*3)
	}), "dist/manifest-2.5.15.json")

	b.addP(compress.KindTrajectory, true, false, dialogue(18, 20, true, 520,
		"助手：三次失败的共同点是目标目录不存在，不是签名或压缩算法问题。",
		"用户：结论清楚了，收尾。"))

	b.needle("N1", "exit status 2: missing target dir", WhereWindow, s2, "errcode")
	b.needle("N2", "EZIPARTIAL", WhereWindow, s2, "errcode")
	b.needle("N3", "zerg pack --out dist/app.tgz --threads 4", WhereWindow, s3, "cmd")
	b.needle("N4", "dist/manifest-2.5.15.json", WhereWindow, s4, "path")
	b.needle("N5", "timeout=90s", WhereRecoverable, s1, "number")

	return Sample{
		ID: "R13", Class: "trajectory", Lang: "zh", Control: false,
		LenAssert: LenAssert{Min: 4400, Max: 6400},
		Segments:  b.segs, Needles: b.needles,
		Probe: Probe{Q: "三次失败的错误原文是什么，最终成功命令是哪条？", ExpectRegex: "EZIPARTIAL.*zerg pack", Grader: "regex"},
	}
}

// ---------------------------------------------------------------- R14 zh

// R14 zh · 方案 A/B/C 对比讨论（各 3 优点 3 缺点），未选方案论证可丢。
func buildR14() Sample {
	b := &sb{}

	b.addP(compress.KindTrajectory, false, false, dialogue(1, 2, true, 400,
		"用户：三条路线各自算一遍，别只讲好的。"))

	s1 := b.addP(compress.KindObservation, false, true, obsBlock(1900, []string{
		"# 评估矩阵列：p99 写入延迟｜内存峰值｜实现成本｜运维面｜回滚成本｜外部依赖",
	}, func(i int) string {
		switch i % 3 {
		case 0:
			return fmt.Sprintf("矩阵行 %02d：方案 %s · p99 写入延迟 %dms · 内存峰值 %dMiB",
				i, []string{"A", "B", "C"}[i%3], 12+i%90, 512+i*3)
		case 1:
			return fmt.Sprintf("矩阵行 %02d：方案 %s · 实现成本 %d.5 人月 · 运维面 %d 条命令",
				i, []string{"A", "B", "C"}[i%3], 1+i%4, 2+i%9)
		default:
			return fmt.Sprintf("矩阵行 %02d：方案 %s · 回滚成本 %s · 依赖 %s",
				i, []string{"A", "B", "C"}[i%3], []string{"低", "中", "高"}[i%3], []string{"无", "kafka", "外部存储"}[i%3])
		}
	}), "矩阵行 00", "p99 写入延迟")

	b.addP(compress.KindTrajectory, false, false, dialogue(3, 5, true, 540,
		"助手：A 的内存峰值会翻倍，C 要多一套外部依赖，这两条基本能否掉了。"))

	s3 := b.addP(compress.KindObservation, false, true, obsBlock(1000, []string{
		"# 未选方案 C 的否决理由（评审记录）：否决 C：多一套外部依赖（kafka 集群要另建）",
	}, func(i int) string {
		return fmt.Sprintf("未选方案展开论证 %02d：若选 C，需要额外 %d 个补偿任务，且夜间批次要拆成 %d 段。",
			i, 2+i%6, 3+i%5)
	}), "未选方案展开论证 00")

	s4 := b.addP(compress.KindObservation, false, true, obsBlock(420, []string{
		"# 成本核算结论：成本 3.2 人月（其中运维 0.4 人月，含两次演练）",
	}, func(i int) string {
		return fmt.Sprintf("成本核算明细 %02d：人天数 %d，单价按内部口径。", i, 3+i%11)
	}), "成本核算结论")

	s5 := b.addP(compress.KindTrajectory, true, false, dialogue(6, 7, true, 520,
		"助手：选定方案 B：分片写入＋后台合并，理由是内存峰值只涨 8% 且回滚是一条命令。",
		"助手：否决 A：内存峰值翻倍（实测 +104%），夜间批次会顶到容器上限。",
		"用户：就 B，成本数字留档。"))

	b.needle("N1", "选定方案 B：分片写入＋后台合并", WhereWindow, s5, "decision")
	b.needle("N2", "否决 A：内存峰值翻倍", WhereWindow, s5, "decision")
	b.needle("N3", "否决 C：多一套外部依赖", WhereWindow, s3, "decision")
	b.needle("N4", "成本 3.2 人月", WhereWindow, s4, "number")
	b.needle("N5", "评估矩阵列：p99 写入延迟", WhereRecoverable, s1, "id")

	return Sample{
		ID: "R14", Class: "trajectory", Lang: "zh", Control: false,
		LenAssert: LenAssert{Min: 4400, Max: 6400},
		Segments:  b.segs, Needles: b.needles,
		Probe: Probe{Q: "最终选了哪个方案，为什么否决另外两个？", ExpectRegex: "方案 B.*内存峰值.*外部依赖", Grader: "regex"},
	}
}

// ---------------------------------------------------------------- R15 zh（对照）

// R15 zh · 对照：5 轮短对话（问→答→确认），2000 token ⇒ Budget=8192 时未撞线。
func buildR15() Sample {
	b := &sb{}

	b.addP(compress.KindTrajectory, false, false, dialogue(1, 3, true, 700,
		"用户：只读导出能不能先上？",
		"助手：能，写路径下一版再说。"))

	s1 := b.addP(compress.KindObservation, false, true, obsBlock(420, []string{
		"$ zerg export --dry-run --out /tmp/ruler-export/out.csv",
		"# 试跑：写入上面这条命令给的路径，共 120 行，0 条冲突",
	}, func(i int) string {
		return fmt.Sprintf("  row %03d: id=%d status=ok", 120+i, 5000+i)
	}), "/tmp/ruler-export/out.csv")

	s2 := b.addP(compress.KindObservation, false, false, obsBlock(300, []string{
		"# 待确认：是否需要覆盖同名文件（用户尚未答复）",
	}, func(i int) string {
		return fmt.Sprintf("  备注 %02d：导出格式沿用上一版 CSV 头。", i)
	}), "待确认")

	s3 := b.addP(compress.KindTrajectory, true, false, dialogue(4, 5, true, 400,
		"助手：结论：先做只读导出，写路径下一版再排。",
		"助手：设计稿已更新：docs/zh/导出-设计.md。",
		"用户：确认，就按这个来。"))

	b.needle("N1", "结论：先做只读导出，写路径下一版再排", WhereWindow, s3, "decision")
	b.needle("N2", "docs/zh/导出-设计.md", WhereWindow, s3, "path")
	b.needle("N3", "/tmp/ruler-export/out.csv", WhereWindow, s1, "path")
	b.needle("N4", "待确认：是否需要覆盖同名文件", WhereWindow, s2, "decision")
	b.needle("N5", "zerg export --dry-run", WhereRecoverable, s1, "cmd")

	return Sample{
		ID: "R15", Class: "trajectory", Lang: "zh", Control: true,
		LenAssert: LenAssert{Min: 1800, Max: 2400},
		Segments:  b.segs, Needles: b.needles,
		Probe: Probe{Q: "这轮对话的结论和待确认项是什么？", ExpectRegex: "只读导出.*覆盖同名文件", Grader: "regex"},
	}
}

// ---------------------------------------------------------------- R16 en

// R16 en · bug triage thread：5 人 14 条，含 assignee 与 repro。
func buildR16() Sample {
	b := &sb{}

	b.addP(compress.KindTrajectory, false, false, dialogue(1, 4, false, 450,
		"user: filing #4812 against 2.5.15; the scheduler drops queued tasks after a restart."))

	s1 := b.addP(compress.KindObservation, false, true, obsBlock(2200, []string{
		"# issue #4812 · thread dump (14 comments, deduplicated): every \"+1 me too\" line kept once below",
	}, func(i int) string {
		switch i % 3 {
		case 0:
			return fmt.Sprintf("comment %02d by @reporter: reproduced again on a clean install (trace %04x)", i, 8000+i)
		case 1:
			return fmt.Sprintf("comment %02d by @oncall: same here, but only when the queue had more than %d items", i, 200+i)
		default:
			return fmt.Sprintf("comment %02d by @bot: screenshot description attached, 3 identical images of the same table", i)
		}
	}), "thread dump", "@reporter")

	b.addP(compress.KindTrajectory, false, false, dialogue(5, 9, false, 640,
		"assistant: repro narrowed down to the restart path; the queue file is rewritten before the drain finishes."))

	s3 := b.addP(compress.KindObservation, false, true, obsBlock(1100, []string{
		"# triage note: affected: 2.5.14 and 2.5.15 (2.5.13 drains correctly)",
	}, func(i int) string {
		switch i % 2 {
		case 0:
			return fmt.Sprintf("triage note %02d: workaround is to pause the queue before restart (%d occurrences)", i, i%9)
		default:
			return fmt.Sprintf("triage note %02d: duplicate report closed, see the umbrella ticket", i)
		}
	}), "triage note")

	s4 := b.addP(compress.KindObservation, false, true, obsBlock(400, []string{
		"# maintainer reply: won't-fix candidate: upstream SDK keeps the socket open across fork",
	}, func(i int) string {
		return fmt.Sprintf("  rationale line %02d: mitigation lives in our restart path, not in the SDK.", i)
	}), "won't-fix candidate")

	s5 := b.addP(compress.KindTrajectory, true, false, dialogue(10, 14, false, 560,
		"assistant: assignee: @dmitri-volkov, milestone 2.5.16.",
		"assistant: repro: zerg replay --trace 8f21c0 --seed 7 (fails on the 3rd restart).",
		"user: agreed, moving it into the sprint."))

	b.needle("N1", "assignee: @dmitri-volkov", WhereWindow, s5, "id")
	b.needle("N2", "repro: zerg replay --trace 8f21c0 --seed 7", WhereWindow, s5, "cmd")
	b.needle("N3", "affected: 2.5.14 and 2.5.15", WhereWindow, s3, "number")
	b.needle("N4", "won't-fix candidate: upstream SDK keeps the socket open across fork", WhereWindow, s4, "decision")
	b.needle("N5", "issue #4812", WhereRecoverable, s1, "id")

	return Sample{
		ID: "R16", Class: "trajectory", Lang: "en", Control: false,
		LenAssert: LenAssert{Min: 4400, Max: 6400},
		Segments:  b.segs, Needles: b.needles,
		Probe: Probe{Q: "Who owns the issue, what is the repro, which versions are affected?", ExpectRegex: "dmitri-volkov.*replay.*2\\.5\\.14", Grader: "regex"},
	}
}

// ---------------------------------------------------------------- R17 en

// R17 en · PR review 18 条评论（3 条 blocking，其余 nit）。
func buildR17() Sample {
	b := &sb{}

	b.addP(compress.KindTrajectory, false, false, dialogue(1, 3, false, 420,
		"user: opening the review; 18 comments, 3 of them blocking."))

	s1 := b.addP(compress.KindObservation, false, true, obsBlock(2000, []string{
		"# thread 03 (resolved): renamed cfg.TTL to cfg.SessionTTL in 3 call sites",
	}, func(i int) string {
		switch i % 3 {
		case 0:
			return fmt.Sprintf("thread %02d (resolved): nit: prefer errors.Is over == in %d places", i, 1+i%5)
		case 1:
			return fmt.Sprintf("thread %02d (resolved): nit: comment wording, no behaviour change", i)
		default:
			return fmt.Sprintf("thread %02d (open, nit): variable shadowing in handler_%02d.go", i, i%40)
		}
	}), "thread 03")

	b.addP(compress.KindTrajectory, false, false, dialogue(4, 8, false, 600,
		"assistant: two of the blocking threads are about the log path and the retry loop."))

	s3 := b.addP(compress.KindObservation, false, true, obsBlock(1050, []string{
		"# thread 07 (open, blocking): retry loop has no backoff cap (ERR_RETRY_UNBOUNDED)",
	}, func(i int) string {
		return fmt.Sprintf("thread %02d (open, nit): rename loop variable, no functional impact", i)
	}), "thread 07")

	s4 := b.addP(compress.KindObservation, false, true, obsBlock(390, []string{
		"# diff anchor: internal/auth/session.go:214 writes req.Token at debug level",
	}, func(i int) string {
		return fmt.Sprintf("  hunk line %02d: context unchanged", i)
	}), "diff anchor")

	s5 := b.addP(compress.KindTrajectory, true, false, dialogue(12, 18, false, 560,
		"assistant: blocking: session token is written to the debug log; that must go before merge.",
		"assistant: review status: APPROVED after 3 blocking threads resolved.",
		"user: merging once CI is green."))

	b.needle("N1", "blocking: session token is written to the debug log", WhereWindow, s5, "decision")
	b.needle("N2", "review status: APPROVED", WhereWindow, s5, "decision")
	b.needle("N3", "retry loop has no backoff cap", WhereWindow, s3, "errcode")
	b.needle("N4", "internal/auth/session.go:214", WhereWindow, s4, "path")
	b.needle("N5", "renamed cfg.TTL to cfg.SessionTTL", WhereRecoverable, s1, "decision")

	return Sample{
		ID: "R17", Class: "trajectory", Lang: "en", Control: false,
		LenAssert: LenAssert{Min: 4400, Max: 6400},
		Segments:  b.segs, Needles: b.needles,
		Probe: Probe{Q: "What is the blocking comment and the review status?", ExpectRegex: "debug log.*APPROVED", Grader: "regex"},
	}
}

// ---------------------------------------------------------------- R18 en

// R18 en · agent 重试循环：同一工具 6 次，4 次失败（失败段 Pinned，规7）。
func buildR18() Sample {
	b := &sb{}

	b.addP(compress.KindTrajectory, false, false, dialogue(1, 5, false, 480,
		"assistant: calling the upload tool; it keeps failing, retrying with adjusted flags."))

	s1 := b.addP(compress.KindObservation, false, true, obsBlock(2450, []string{
		"# retry plan written to state: attempt 5 of 6 will switch to single-part upload",
	}, func(i int) string {
		switch i % 3 {
		case 0:
			return fmt.Sprintf("probe %02d ok: upload.tail reachable, latency %dms, region eu-west-1", i, 20+i%90)
		case 1:
			return fmt.Sprintf("probe %02d ok: upload.head reachable, latency %dms, region eu-west-1", i, 18+i%80)
		default:
			return fmt.Sprintf("probe %02d ok: bucket listing returned %d objects (unchanged)", i, 100+i%40)
		}
	}), "probe 00 ok", "eu-west-1")

	s2 := b.addP(compress.KindObservation, true, false, obsBlock(760, []string{
		"attempt 1 FAIL err=ERR_ETIMEDOUT after 30000ms",
		"attempt 2 FAIL err=ERR_CONNREFUSED peer=upload.internal:8443",
		"attempt 3 FAIL err=ERR_EPROTO tls: first record does not look like a TLS handshake",
		"attempt 4 FAIL err=ERR_EBADGATEWAY status=502 body=\"upstream reset\"",
	}, func(i int) string {
		return fmt.Sprintf("  retry bookkeeping %02d: sleep=%ds backoff=none", i, 1+i%5)
	}), "ERR_ETIMEDOUT", "ERR_CONNREFUSED", "ERR_EPROTO", "ERR_EBADGATEWAY")

	s3 := b.addP(compress.KindObservation, false, true, obsBlock(1200, []string{
		"# attempt 6 ok: zerg upload --bucket rulertest --max-inflight=8 --timeout=45s",
	}, func(i int) string {
		return fmt.Sprintf("attempt 6 ok: chunk %02d uploaded, etag=abc%03d, bytes=%d", i, i, 65536+i*11)
	}), "bucket rulertest")

	b.addP(compress.KindObservation, false, true, obsBlock(380, []string{
		"# abandoned path: multipart upload via /legacy/gw (deprecated, returns 4xx)",
	}, func(i int) string {
		return fmt.Sprintf("  note %02d: gateway team says the path is decommissioned.", i)
	}), "/legacy/gw")

	b.addP(compress.KindTrajectory, true, false, dialogue(12, 16, false, 540,
		"assistant: the 502 was the load balancer, the timeout was ours; the new flags fixed it.",
		"user: note the abandoned legacy path for the postmortem."))

	b.needle("N1", "ERR_ETIMEDOUT", WhereWindow, s2, "errcode")
	b.needle("N2", "ERR_CONNREFUSED", WhereWindow, s2, "errcode")
	b.needle("N3", "ERR_EPROTO", WhereWindow, s2, "errcode")
	b.needle("N4", "zerg upload --bucket rulertest --max-inflight=8 --timeout=45s", WhereWindow, s3, "cmd")
	b.needle("N5", "attempt 5 of 6", WhereRecoverable, s1, "number")

	return Sample{
		ID: "R18", Class: "trajectory", Lang: "en", Control: false,
		LenAssert: LenAssert{Min: 4400, Max: 6400},
		Segments:  b.segs, Needles: b.needles,
		Probe: Probe{Q: "Which err codes appeared and what flags finally worked?", ExpectRegex: "ERR_EPROTO.*max-inflight=8", Grader: "regex"},
	}
}

// ---------------------------------------------------------------- R19 en

// R19 en · planning chat 拆出 15 条任务，含依赖与 owner。
func buildR19() Sample {
	b := &sb{}

	b.addP(compress.KindTrajectory, false, false, dialogue(1, 3, false, 400,
		"user: let's split the migration into 15 tickets, with dependencies and owners."))

	s1 := b.addP(compress.KindObservation, false, true, obsBlock(1800, []string{
		"# sizing note: estimate: 13.5h for the migrator rewrite (largest single ticket)",
	}, func(i int) string {
		switch i % 3 {
		case 0:
			return fmt.Sprintf("ticket T-%04d: migrate shard %02d, estimate: %d.5h", 300+i, i%16, 1+i%7)
		case 1:
			return fmt.Sprintf("ticket T-%04d: add dual-read switch, estimate: %d.0h", 300+i, 2+i%8)
		default:
			return fmt.Sprintf("ticket T-%04d: drop legacy view after soak, estimate: %d.5h", 300+i, 1+i%4)
		}
	}), "sizing note")

	b.addP(compress.KindTrajectory, false, false, dialogue(4, 9, false, 560,
		"assistant: three of the tickets are pure brainstorming leftovers, dropping them."))

	s3 := b.addP(compress.KindObservation, false, true, obsBlock(950, []string{
		"# board dependency edges: depends-on: T-0307, T-0312",
	}, func(i int) string {
		return fmt.Sprintf("dependency %02d: blocks: T-%04d (dual-read switch needs the new column)", i, 318+i)
	}), "board dependency edges")

	s4 := b.addP(compress.KindObservation, false, true, obsBlock(400, []string{
		"# ownership: owner: @lin-ops for the soak window",
	}, func(i int) string {
		return fmt.Sprintf("  ownership note %02d: reviewer rotates weekly.", i)
	}), "ownership")

	s5 := b.addP(compress.KindTrajectory, true, false, dialogue(12, 15, false, 520,
		"assistant: AC1: every ticket must have an owner and an acceptance check.",
		"assistant: AC2: a ticket whose dependency is unmet must not enter the execution queue.",
		"user: agreed, that's the board."))

	b.needle("N1", "AC1: every ticket must have an owner and an acceptance check", WhereWindow, s5, "decision")
	b.needle("N2", "AC2: a ticket whose dependency is unmet must not enter the execution queue", WhereWindow, s5, "decision")
	b.needle("N3", "owner: @lin-ops", WhereWindow, s4, "id")
	b.needle("N4", "depends-on: T-0307, T-0312", WhereWindow, s3, "id")
	b.needle("N5", "estimate: 13.5h", WhereRecoverable, s1, "number")

	return Sample{
		ID: "R19", Class: "trajectory", Lang: "en", Control: false,
		LenAssert: LenAssert{Min: 4400, Max: 6400},
		Segments:  b.segs, Needles: b.needles,
		Probe: Probe{Q: "What are the acceptance criteria and what does the board depend on?", ExpectRegex: "AC1.*AC2.*T-0307", Grader: "regex"},
	}
}

// ---------------------------------------------------------------- R20 en（对照）

// R20 en · 对照：4 条短 thread（报告→追问→答复），1900 token ⇒ 8192 档未撞线。
func buildR20() Sample {
	b := &sb{}

	b.addP(compress.KindTrajectory, false, false, dialogue(1, 4, false, 800,
		"user: can you confirm the gateway rollback is safe on build 4412?"))

	s1 := b.addP(compress.KindObservation, false, true, obsBlock(430, []string{
		"# ci note: https://ci.invalid/runs/8841 passed on darwin/arm64",
		"# artifact: /srv/gateway/releases/4412 checksum matches the signed manifest",
		"# manual check: curl -sS localhost:8080/healthz returns ok",
	}, func(i int) string {
		return fmt.Sprintf("  ci detail %02d: %d tests passed, 0 skipped", i, 120+i)
	}), "https://ci.invalid/runs/8841", "/srv/gateway/releases/4412")

	b.addP(compress.KindObservation, false, false, obsBlock(250, []string{
		"# signature: -- Lin, platform on-call",
	}, func(i int) string {
		return fmt.Sprintf("  thread %02d: thanks, closing.", i)
	}), "-- Lin, platform on-call")

	// 最近 2 轮（结论）钉住，且必须是最后一段 —— 与 observation 类「小尾段在末尾」的形态相反。
	s3 := b.addP(compress.KindTrajectory, true, false, dialogue(5, 8, false, 420,
		"assistant: conclusion: roll the gateway back to build 4412.",
		"assistant: conclusion: cache warmup is a no-op on cold start."))

	b.needle("N1", "conclusion: roll the gateway back to build 4412", WhereWindow, s3, "decision")
	b.needle("N2", "cache warmup is a no-op on cold start", WhereWindow, s3, "decision")
	b.needle("N3", "https://ci.invalid/runs/8841", WhereWindow, s1, "id")
	b.needle("N4", "/srv/gateway/releases/4412", WhereWindow, s1, "path")
	b.needle("N5", "curl -sS localhost:8080/healthz", WhereRecoverable, s1, "cmd")

	return Sample{
		ID: "R20", Class: "trajectory", Lang: "en", Control: true,
		LenAssert: LenAssert{Min: 1800, Max: 2400},
		Segments:  b.segs, Needles: b.needles,
		Probe: Probe{Q: "What did the thread conclude about the rollback?", ExpectRegex: "roll the gateway back to build 4412", Grader: "regex"},
	}
}
