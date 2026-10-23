package eval

import (
	"fmt"

	"github.com/Mr2109/zerg-swarm/core/internal/compress"
)

// 本文件是样本面第三类：**plan & reasoning（计划 / 假设 / 步骤）** R21–R30
// （任务单 3.2 C 表）。
//
// 构造纪律（任务单 §3.1）：
//   规1 类内 zh=R21–R25 / en=R26–R30，各 5
//   规2 对照 = R25 / R30，1800–2400 token；其余 4400–6400 token
//   规4 每样本 5 条 needle = **5 window + 0 recoverable**（任务单 3.3：plan 配额 W50/R0）
//   规7 plan 钉「计划主体与未完成 todo」；执行细节段（示例命令输出）不钉、可卸载
//   规8 可卸载段必给 raw_ptr（内容寻址，与 ctx_unload.go 同规）
//
// ★ 3.2 C 表的「必留信息 / 可丢信息」两列在 3.3 的硬配额下合并成 5 条 window needle：
// 「必留信息」列可能列了 7–8 个要点，这里按「阶段名 / 验收判据 / 回滚点 / 工时」分组后
// 并到恰好 5 条；「可丢信息」列（每步执行细节、示例命令输出）整段落进**未钉的 observation
// 段**——该段不含任何 needle，丢它不影响 MRR，正好让 M4（删除比）/ M7（RVS）有得分母。
//
// ★ 该类在四档 Budget 上的预期形态（并被 scan 实测）：5 条 needle 全在 Pinned 的
// KindPlan 段里，而 PrunerFree 的①②两道闸都只挑 KindObservation 且跳过 Pinned
// ⇒ plan 类**在任何档都不该丢 needle**。这不是「闸松」，而是「钉住的东西闸碰不到」
// 的正向对照：若扫描里 plan 掉了 needle，那就是引擎 bug（钉住段被改写），必须当红。

// PlanSamples —— plan 类 10 条（确定性：同一二进制两次调用逐字节相同）。
func PlanSamples() []Sample {
	all := []Sample{
		buildR21(), buildR22(), buildR23(), buildR24(), buildR25(),
		buildR26(), buildR27(), buildR28(), buildR29(), buildR30(),
	}
	for i := range all {
		all[i].finalize()
	}
	return all
}

// ---------------------------------------------------------------- R21 zh

// R21 zh · 5 阶段 × 3 步实施计划，每阶段带验收判据（计划主体 Pinned）。
func buildR21() Sample {
	b := &sb{}

	b.addP(compress.KindObservation, false, true, obsBlock(2500, []string{
		"# 实施细节执行日志（每一步的示例命令与输出，计划主体见下一段）",
	}, func(i int) string {
		switch i % 3 {
		case 0:
			return fmt.Sprintf("阶段 S%d 步骤 %d：./scripts/rollout/s%02d.sh --step %d → ok，输出 %d 行",
				i%5+1, i%3+1, i%5+1, i%3+1, 20+i%80)
		case 1:
			return fmt.Sprintf("  日志片段 %04d：shard-%02d 双写比对通过，diff 行数 %d", i, i%12, i%7)
		default:
			return fmt.Sprintf("  耗时统计 %04d：step 用时 %dms，重试 %d 次", i, 40+i%300, i%3)
		}
	}), "阶段 S1 步骤 1", "shard-01")

	s1 := b.addP(compress.KindPlan, true, false, obsBlock(1900, []string{
		"# 实施计划（5 阶段 × 3 步；阶段验收全列在此；未完成 todo 不删）",
		"阶段一：只读探针就绪",
		"  验收：只读探针在 3 个分片上均可返回 schema 版本与行数",
		"阶段二：影子读上线",
		"  验收：影子读与主读的差异行数 = 0，且持续 6 小时",
		"阶段三：双写切换",
		"  验收：双写两侧校验和一致率 100%，不一致即回滚本阶段",
		"阶段四：主读切到新路径",
		"  验收：切流后错误率不超过切换前基线的 1.05 倍",
		"阶段五：下线旧读路径",
		"  验收：旧读路径连续 72 小时零调用后才允许删代码",
		"回滚点：在阶段四前保留 v2 读路径（配置项 read.path=v2 不得删除）",
		"预计工时 26 人日（其中演练占 4 人日）",
		"# 未完成 todo：阶段二第 3 步、阶段三第 2 步仍在进行",
	}, func(i int) string {
		return fmt.Sprintf("  步骤说明 %02d：值班同学按 runbook 执行，命令与输出已归档到上一段", i)
	}), "阶段三：双写切换", "read.path=v2")

	b.addP(compress.KindObservation, false, true, obsBlock(700, []string{
		"# 阶段验收汇总（工具输出尾部）",
		"验收执行器：zerg rollout verify --stage all",
	}, func(i int) string {
		return fmt.Sprintf("  verify %02d：阶段 S%d 的 3 项检查中 %d 项通过", i, i%5+1, i%3+1)
	}), "zerg rollout verify --stage all")

	b.needle("N1", "阶段三：双写切换", WhereWindow, s1, "id")
	b.needle("N2", "双写两侧校验和一致率 100%", WhereWindow, s1, "decision")
	b.needle("N3", "回滚点：在阶段四前保留 v2 读路径", WhereWindow, s1, "decision")
	b.needle("N4", "阶段五：下线旧读路径", WhereWindow, s1, "id")
	b.needle("N5", "预计工时 26 人日", WhereWindow, s1, "number")

	return Sample{
		ID: "R21", Class: "plan", Lang: "zh", Control: false,
		LenAssert: LenAssert{Min: 4400, Max: 6400},
		Segments:  b.segs, Needles: b.needles,
		Probe: Probe{Q: "五个阶段名分别是什么，回滚点在哪一步？", ExpectRegex: "阶段五：下线旧读路径.*回滚点", Grader: "regex"},
	}
}

// ---------------------------------------------------------------- R22 zh

// R22 zh · 排查假设树 1 根 12 叶，含证据链（已排除分支的证据正文可丢）。
func buildR22() Sample {
	b := &sb{}

	b.addP(compress.KindObservation, false, true, obsBlock(2400, []string{
		"# 已排除分支的证据正文（12 叶中 5 叶已排除，日志粘贴如下）",
	}, func(i int) string {
		switch i % 3 {
		case 0:
			return fmt.Sprintf("分支 H%d 证据：pool.wait_ms p95=%dms，in_flight=%d，未见排队", i%12+1, 8+i%30, i%16)
		case 1:
			return fmt.Sprintf("分支 H%d 证据：clock_offset_ms=%d，ntp_source=203.0.113.%d", i%12+1, i%3, i%250)
		default:
			return fmt.Sprintf("分支 H%d 证据：gc_pause_p99=%dus，堆占用 %d MiB，无 STW 抖动", i%12+1, 200+i%900, 120+i%60)
		}
	}), "分支 H1 证据", "ntp_source=203.0.113.1")

	s1 := b.addP(compress.KindPlan, true, false, obsBlock(2100, []string{
		"# 排查假设树（1 根 12 叶；当前活跃分支 H7；已排除 5 叶）",
		"活跃分支 H7：连接池预热的 goroutine 泄漏",
		"已排除 H2：证据 pool.wait p95 仅 12ms，够不到 4.8s 超时",
		"已排除 H4：证据 cpu_throttle 计数为 0，容器未限频",
		"已排除 H9：证据 fork 后 fd 数量不变，句柄未泄漏",
		"关键证据一：core/internal/pool/warm.go:117 在 fork 前取锁",
		"关键证据二：core/internal/cache/fork.go:88 子进程首次写即阻塞",
		"待验假设 H11：子进程继承父进程的 fd 表",
		"剩余 3 条待验分支（H8 / H10 / H12），优先级按证据成本排序",
	}, func(i int) string {
		return fmt.Sprintf("  假设树注释 %02d：本叶证据正文已归档到上一段，判定不依赖注释", i)
	}), "活跃分支 H7", "core/internal/pool/warm.go:117", "core/internal/cache/fork.go:88")

	b.addP(compress.KindObservation, false, true, obsBlock(700, []string{
		"# 证据索引（每条的采集命令与时间戳）",
	}, func(i int) string {
		return fmt.Sprintf("  证据 %02d：采集命令 zerg probe --what leaf%d --window 6h", i, i%12+1)
	}), "zerg probe")

	b.needle("N1", "活跃分支 H7：连接池预热的 goroutine 泄漏", WhereWindow, s1, "decision")
	b.needle("N2", "已排除 H2：证据 pool.wait p95 仅 12ms", WhereWindow, s1, "decision")
	b.needle("N3", "core/internal/pool/warm.go:117", WhereWindow, s1, "path")
	b.needle("N4", "core/internal/cache/fork.go:88", WhereWindow, s1, "path")
	b.needle("N5", "剩余 3 条待验分支", WhereWindow, s1, "number")

	return Sample{
		ID: "R22", Class: "plan", Lang: "zh", Control: false,
		LenAssert: LenAssert{Min: 4400, Max: 6400},
		Segments:  b.segs, Needles: b.needles,
		Probe: Probe{Q: "当前活跃的是哪条分支，排除了几条，还剩几条待验？", ExpectRegex: "H7.*剩余 3 条", Grader: "regex"},
	}
}

// ---------------------------------------------------------------- R23 zh

// R23 zh · 迁移步骤清单 40 步，含顺序约束与不可逆步骤。
func buildR23() Sample {
	b := &sb{}

	b.addP(compress.KindObservation, false, true, obsBlock(2600, []string{
		"# 每步说明文字与勾选记录（清单主体见下一段；本段不含任何结论）",
	}, func(i int) string {
		switch i % 3 {
		case 0:
			return fmt.Sprintf("步骤 %02d 勾选记录：执行人值班-%02d，开始 09:%02d，结束 10:%02d", i%40+1, i%9, i%60, i%60)
		case 1:
			return fmt.Sprintf("步骤 %02d 说明续：该步可重入，失败后原样重跑即可（详见 runbook 第 %d 节）", i%40+1, i%17)
		default:
			return fmt.Sprintf("步骤 %02d 输出摘要：影响行数 %d，耗时 %dms，无锁等待", i%40+1, 1200+i*7, 30+i%400)
		}
	}), "步骤 01 勾选记录", "runbook")

	s1 := b.addP(compress.KindPlan, true, false, obsBlock(1800, []string{
		"# 迁移步骤清单（40 步；顺序约束与不可逆步骤单列）",
		"顺序依赖：步骤 12 必须在步骤 15 之前完成（15 会读 12 建出来的索引）",
		"顺序依赖：步骤 31 依赖步骤 29 的回填结果，回填未完成不许开 31",
		"不可逆步骤：步骤 27 会删除 v1 表（删后只能从快照恢复）",
		"校验命令：zerg migrate verify --target shard-07",
		"回滚命令：zerg migrate undo --to step-26",
		"回滚窗口 72 小时（超过窗口只能走快照恢复，耗时约 6 小时）",
		"# 未完成 todo：步骤 28–34 尚未勾选",
	}, func(i int) string {
		return fmt.Sprintf("  清单备注 %02d：本步的详细说明已归档，判定只看上面的顺序与不可逆标记", i)
	}), "步骤 27 会删除 v1 表", "zerg migrate verify --target shard-07")

	b.addP(compress.KindObservation, false, true, obsBlock(700, []string{
		"# 清单导出尾部（工具输出）：40 步中已勾选 27 步",
	}, func(i int) string {
		return fmt.Sprintf("  export row %02d：step=%02d state=%s", i, i%40+1, []string{"done", "todo", "skipped"}[i%3])
	}), "已勾选 27 步")

	b.needle("N1", "顺序依赖：步骤 12 必须在步骤 15 之前完成", WhereWindow, s1, "decision")
	b.needle("N2", "不可逆步骤：步骤 27 会删除 v1 表", WhereWindow, s1, "decision")
	b.needle("N3", "zerg migrate verify --target shard-07", WhereWindow, s1, "cmd")
	b.needle("N4", "zerg migrate undo --to step-26", WhereWindow, s1, "cmd")
	b.needle("N5", "回滚窗口 72 小时", WhereWindow, s1, "number")

	return Sample{
		ID: "R23", Class: "plan", Lang: "zh", Control: false,
		LenAssert: LenAssert{Min: 4400, Max: 6400},
		Segments:  b.segs, Needles: b.needles,
		Probe: Probe{Q: "哪些步骤有顺序约束，哪一步不可逆，回滚窗口多长？", ExpectRegex: "步骤 27.*72 小时", Grader: "regex"},
	}
}

// ---------------------------------------------------------------- R24 zh

// R24 zh · 排期与预算表 12 行（里程碑 / 人力 / 超支条件）。
func buildR24() Sample {
	b := &sb{}

	b.addP(compress.KindObservation, false, true, obsBlock(2500, []string{
		"# 人力明细行与备注（12 行主子表见下一段）",
	}, func(i int) string {
		switch i % 3 {
		case 0:
			return fmt.Sprintf("人力明细 %02d：后端 %d.5 人月，前端 %d.0 人月，测试 %d.5 人月", i, i%5+1, i%4, i%3+1)
		case 1:
			return fmt.Sprintf("备注 %02d：该行人力按 8 月排期折算，假期已扣除 %d 天", i, i%9)
		default:
			return fmt.Sprintf("风险备注 %02d：外部依赖方交付延迟 %d 天，已计入缓冲", i, i%15)
		}
	}), "人力明细 00", "外部依赖方")

	s1 := b.addP(compress.KindPlan, true, false, obsBlock(1600, []string{
		"# 排期与预算（12 行：里程碑 / 人力 / 超支条件）",
		"总预算 96 万元（含 8 万元应急）",
		"里程碑 M1：2026-11-08 环境就绪",
		"里程碑 M2：2026-11-29 影子读通过",
		"里程碑 M3：2026-12-20 灰度完成",
		"超支触发条件：累计支出超过 78 万元即上评审（不许先花后报）",
		"投入 7.5 人月，其中运维 0.5 人月",
		"# 未完成 todo：M2 的验收材料尚未归档",
	}, func(i int) string {
		return fmt.Sprintf("  排期备注 %02d：里程碑评审按双周节奏走，日期变更需重排本表", i)
	}), "总预算 96 万元", "里程碑 M3：2026-12-20 灰度完成")

	b.addP(compress.KindObservation, false, true, obsBlock(700, []string{
		"# 预算执行率导出（工具输出）",
	}, func(i int) string {
		return fmt.Sprintf("  month %02d：计划支出 %d.%d 万元，实际支出 %d.%d 万元", i, 4+i%6, i%10, 3+i%6, i%10)
	}), "预算执行率")

	b.needle("N1", "总预算 96 万元", WhereWindow, s1, "number")
	b.needle("N2", "里程碑 M1：2026-11-08 环境就绪", WhereWindow, s1, "id")
	b.needle("N3", "里程碑 M3：2026-12-20 灰度完成", WhereWindow, s1, "id")
	b.needle("N4", "超支触发条件：累计支出超过 78 万元即上评审", WhereWindow, s1, "decision")
	b.needle("N5", "投入 7.5 人月", WhereWindow, s1, "number")

	return Sample{
		ID: "R24", Class: "plan", Lang: "zh", Control: false,
		LenAssert: LenAssert{Min: 4400, Max: 6400},
		Segments:  b.segs, Needles: b.needles,
		Probe: Probe{Q: "总预算、三个里程碑日期和超支触发条件分别是什么？", ExpectRegex: "96 万元.*2026-12-20.*78 万元", Grader: "regex"},
	}
}

// ---------------------------------------------------------------- R25 zh（对照）

// R25 zh · 对照：短计划 6 条（含 1 条验收），约 1950 token ⇒ Budget=8192 未撞线。
func buildR25() Sample {
	b := &sb{}

	b.addP(compress.KindObservation, false, true, obsBlock(900, []string{
		"# 执行备注（短计划配套，不含判定信息）",
	}, func(i int) string {
		return fmt.Sprintf("备注 %02d：按周例会节奏推进，阻塞项同步到看板", i)
	}), "按周例会节奏推进")

	s1 := b.addP(compress.KindPlan, true, false, obsBlock(1000, []string{
		"# 短计划（6 条，含 1 条验收）",
		"计划 1：先跑通只读探针，不做任何写操作",
		"计划 2：把影子读挂到主读之后，只比对不改写",
		"计划 3：把导出任务拆成按天分片",
		"计划 4：分片任务支持断点续跑",
		"计划 5：灰度前补齐回滚脚本",
		"计划 6：灰度窗口定在周四 22:00",
		"验收：单天重跑不产生重复行",
		"owner：@zhang-san",
		"预计 2 人日",
	}, func(i int) string {
		return fmt.Sprintf("  计划附注 %02d：解释性从句已省略，只留动作与判据", i)
	}), "计划 3：把导出任务拆成按天分片", "owner：@zhang-san")

	b.needle("N1", "计划 3：把导出任务拆成按天分片", WhereWindow, s1, "decision")
	b.needle("N2", "验收：单天重跑不产生重复行", WhereWindow, s1, "decision")
	b.needle("N3", "owner：@zhang-san", WhereWindow, s1, "id")
	b.needle("N4", "计划 6：灰度窗口定在周四 22:00", WhereWindow, s1, "decision")
	b.needle("N5", "预计 2 人日", WhereWindow, s1, "number")

	return Sample{
		ID: "R25", Class: "plan", Lang: "zh", Control: true,
		LenAssert: LenAssert{Min: 1800, Max: 2400},
		Segments:  b.segs, Needles: b.needles,
		Probe: Probe{Q: "六条计划的验收标准和 owner 是什么？", ExpectRegex: "单天重跑.*zhang-san", Grader: "regex"},
	}
}

// ---------------------------------------------------------------- R26 en

// R26 en · RFC 计划：背景 / 方案 / 风险 / 回滚（决定段 Pinned）。
func buildR26() Sample {
	b := &sb{}

	b.addP(compress.KindObservation, false, true, obsBlock(2500, []string{
		"# background research log (related work, benchmark runs; no decisions here)",
	}, func(i int) string {
		switch i % 3 {
		case 0:
			return fmt.Sprintf("bench run %03d: dual-write overhead %d.%d%% on the ledger path, samples=%d", i, 1+i%4, i%10, 500+i*7)
		case 1:
			return fmt.Sprintf("related work %03d: industry write-ups recommend shadow reads before cutover (%d references)", i, i%9)
		default:
			return fmt.Sprintf("stakeholder note %03d: finance wants the ledger readable during the migration window", i)
		}
	}), "bench run 000", "related work 001")

	s1 := b.addP(compress.KindPlan, true, false, obsBlock(2100, []string{
		"# RFC: dual-write gateway (decision, alternatives, risk, rollback)",
		"Decision: adopt the dual-write gateway with a 15-minute shadow window",
		"Rejected: rewrite the ledger service (est. 9 sprints, no rollback path)",
		"Rejected: keep the single-writer queue and add batching (cannot meet the SLO)",
		"Rollback: flip routing key gw.primary back to gw.legacy in one command",
		"Rollback step 2: drain the shadow queue, then replay the last 15 minutes",
		"SLO: p99 write latency must stay under 180 ms during cutover",
		"Risk: checksum divergence between the two paths; monitored every 60 s",
		"# open todo: confirm the review slot with the storage team",
	}, func(i int) string {
		return fmt.Sprintf("  rfc note %02d: wording only, the decision text above is authoritative", i)
	}), "Decision: adopt the dual-write gateway", "gw.legacy")

	b.addP(compress.KindObservation, false, true, obsBlock(700, []string{
		"# appendix: load-test tail output",
	}, func(i int) string {
		return fmt.Sprintf("  sample %03d: rtt=%dms ok=%t region=eu-west-1", i, 9+i%120, i%7 != 0)
	}), "region=eu-west-1")

	b.needle("N1", "Decision: adopt the dual-write gateway with a 15-minute shadow window", WhereWindow, s1, "decision")
	b.needle("N2", "Rejected: rewrite the ledger service (est. 9 sprints, no rollback path)", WhereWindow, s1, "decision")
	b.needle("N3", "Rollback: flip routing key gw.primary back to gw.legacy in one command", WhereWindow, s1, "cmd")
	b.needle("N4", "SLO: p99 write latency must stay under 180 ms", WhereWindow, s1, "number")
	b.needle("N5", "Rejected: keep the single-writer queue and add batching", WhereWindow, s1, "decision")

	return Sample{
		ID: "R26", Class: "plan", Lang: "en", Control: false,
		LenAssert: LenAssert{Min: 4400, Max: 6400},
		Segments:  b.segs, Needles: b.needles,
		Probe: Probe{Q: "What was decided, what was rejected, and what is the rollback?", ExpectRegex: "dual-write gateway.*gw\\.legacy", Grader: "regex"},
	}
}

// ---------------------------------------------------------------- R27 en

// R27 en · root-cause 假设树（1 confirmed + 5 ruled-out）。
func buildR27() Sample {
	b := &sb{}

	b.addP(compress.KindObservation, false, true, obsBlock(2400, []string{
		"# ruled-out branch evidence (raw logs and stacks, kept for audit only)",
	}, func(i int) string {
		switch i % 3 {
		case 0:
			return fmt.Sprintf("evidence %03d: pool.wait_ms p95=%d, in_flight=%d, queue depth %d", i, 8+i%30, i%16, i%5)
		case 1:
			return fmt.Sprintf("evidence %03d: clock_offset_ms=%d, ntp drift over 6h %d ms", i, i%3, i%4)
		default:
			return fmt.Sprintf("evidence %03d: gc pause p99 %dus, heap %d MiB, no stop-the-world spike", i, 200+i%900, 120+i%60)
		}
	}), "evidence 000", "ntp drift over 6h")

	s1 := b.addP(compress.KindPlan, true, false, obsBlock(2000, []string{
		"# root-cause hypothesis tree (1 root, 12 leaves, 1 confirmed, 5 ruled out)",
		"Confirmed cause: fd table is inherited across fork in warm()",
		"Ruled out: connection pool exhaustion (p95 wait 12 ms)",
		"Ruled out: clock drift (offset 0 ms over 6 hours)",
		"Ruled out: CPU throttling (throttle count 0 during the window)",
		"repro: zerg replay --trace 8f21c0 --seed 7 --restarts 3",
		"monitor: pool_wait_p95_ms",
		"# open: two leaves still unchecked (H11, H12)",
	}, func(i int) string {
		return fmt.Sprintf("  tree note %02d: leaf text is stable, evidence bodies live in the segment above", i)
	}), "Confirmed cause: fd table is inherited", "pool_wait_p95_ms")

	b.addP(compress.KindObservation, false, true, obsBlock(700, []string{
		"# probe timeline tail",
	}, func(i int) string {
		return fmt.Sprintf("  probe %03d: leaf=H%d window=6h status=%s", i, i%12+1, []string{"done", "pending"}[i%2])
	}), "leaf=H1")

	b.needle("N1", "Confirmed cause: fd table is inherited across fork in warm()", WhereWindow, s1, "decision")
	b.needle("N2", "Ruled out: connection pool exhaustion (p95 wait 12 ms)", WhereWindow, s1, "decision")
	b.needle("N3", "Ruled out: clock drift (offset 0 ms over 6 hours)", WhereWindow, s1, "decision")
	b.needle("N4", "repro: zerg replay --trace 8f21c0 --seed 7 --restarts 3", WhereWindow, s1, "cmd")
	b.needle("N5", "monitor: pool_wait_p95_ms", WhereWindow, s1, "id")

	return Sample{
		ID: "R27", Class: "plan", Lang: "en", Control: false,
		LenAssert: LenAssert{Min: 4400, Max: 6400},
		Segments:  b.segs, Needles: b.needles,
		Probe: Probe{Q: "What is the confirmed cause, the repro command, and the monitor?", ExpectRegex: "fd table.*replay.*pool_wait_p95_ms", Grader: "regex"},
	}
}

// ---------------------------------------------------------------- R28 en

// R28 en · rollout checklist 30 项（3 个 gate 项）。
func buildR28() Sample {
	b := &sb{}

	b.addP(compress.KindObservation, false, true, obsBlock(2500, []string{
		"# per-item notes for the 30-item rollout checklist (no decisions below)",
	}, func(i int) string {
		switch i % 3 {
		case 0:
			return fmt.Sprintf("item %02d note: owner ran it twice, second run was a no-op (%d s)", i%30+1, 20+i%90)
		case 1:
			return fmt.Sprintf("item %02d note: precheck repeated because the first window was dirty", i%30+1)
		default:
			return fmt.Sprintf("item %02d note: output archived to /tmp/rollout/run-%03d/item-%02d.log", i%30+1, i%9, i%30+1)
		}
	}), "item 01 note", "/tmp/rollout/run-0")

	s1 := b.addP(compress.KindPlan, true, false, obsBlock(1800, []string{
		"# rollout checklist (30 items, 3 gates, order constraints, kill switch)",
		"Gate 1: schema migration applied on every shard, verified by row count",
		"Gate 2: shadow traffic must show zero checksum mismatches for 24h",
		"Gate 3: on-call sign-off from the storage team",
		"Order: run the schema backfill before flipping the read path",
		"Order: never enable dual-write before gate 1 is signed off",
		"Kill switch: zerg rollout abort --scope gateway --force",
		"Contact: @ops-mira (pager rotation B)",
		"# open: items 27-30 remain unchecked",
	}, func(i int) string {
		return fmt.Sprintf("  checklist note %02d: tick boxes are bookkeeping, the gates above decide", i)
	}), "Gate 2: shadow traffic", "Kill switch: zerg rollout abort")

	b.addP(compress.KindObservation, false, true, obsBlock(700, []string{
		"# checklist export tail",
	}, func(i int) string {
		return fmt.Sprintf("  export row %02d: item=%02d state=%s", i, i%30+1, []string{"done", "open"}[i%2])
	}), "item=01 state=done")

	b.needle("N1", "Gate 2: shadow traffic must show zero checksum mismatches for 24h", WhereWindow, s1, "decision")
	b.needle("N2", "Gate 3: on-call sign-off from the storage team", WhereWindow, s1, "decision")
	b.needle("N3", "Order: run the schema backfill before flipping the read path", WhereWindow, s1, "decision")
	b.needle("N4", "Kill switch: zerg rollout abort --scope gateway --force", WhereWindow, s1, "cmd")
	b.needle("N5", "Contact: @ops-mira (pager rotation B)", WhereWindow, s1, "id")

	return Sample{
		ID: "R28", Class: "plan", Lang: "en", Control: false,
		LenAssert: LenAssert{Min: 4400, Max: 6400},
		Segments:  b.segs, Needles: b.needles,
		Probe: Probe{Q: "What are the gates, the order constraint, and the kill switch?", ExpectRegex: "Gate 2.*abort --scope gateway", Grader: "regex"},
	}
}

// ---------------------------------------------------------------- R29 en

// R29 en · risk register 15 行（2 high / 6 medium / 7 low）。
func buildR29() Sample {
	b := &sb{}

	b.addP(compress.KindObservation, false, true, obsBlock(2500, []string{
		"# low-risk rows and free-form notes (7 low rows live here)",
	}, func(i int) string {
		switch i % 3 {
		case 0:
			return fmt.Sprintf("low row %02d: R-%02d likelihood 1 x impact %d, accept and monitor", i, 9+i%7, i%3+1)
		case 1:
			return fmt.Sprintf("note %02d: medium rows were re-scored after the dry run", i)
		default:
			return fmt.Sprintf("note %02d: mitigation owners confirmed at the weekly review", i)
		}
	}), "low row 00", "re-scored after the dry run")

	s1 := b.addP(compress.KindPlan, true, false, obsBlock(1700, []string{
		"# risk register (15 rows: 2 high / 6 medium / 7 low; high rows below)",
		"High R-01: dual-write divergence; mitigation: nightly checksum reconciliation",
		"High R-02: gateway memory growth; mitigation: heap cap at 3 GiB",
		"Trigger threshold: divergence rate above 0.02% for 10 minutes",
		"Owner: @ravi-k (platform)",
		"Medium R-07 score 9 (likelihood 3 x impact 3)",
		"# open: R-02 mitigation needs a load test before sign-off",
	}, func(i int) string {
		return fmt.Sprintf("  register note %02d: medium rows stay in this table, low rows are archived", i)
	}), "High R-01: dual-write divergence", "@ravi-k")

	b.addP(compress.KindObservation, false, true, obsBlock(700, []string{
		"# register export tail",
	}, func(i int) string {
		return fmt.Sprintf("  export row %02d: risk=%02d severity=%s", i, i%15+1, []string{"low", "medium"}[i%2])
	}), "risk=01 severity=low")

	b.needle("N1", "High R-01: dual-write divergence; mitigation: nightly checksum reconciliation", WhereWindow, s1, "decision")
	b.needle("N2", "High R-02: gateway memory growth; mitigation: heap cap at 3 GiB", WhereWindow, s1, "decision")
	b.needle("N3", "Trigger threshold: divergence rate above 0.02% for 10 minutes", WhereWindow, s1, "number")
	b.needle("N4", "Owner: @ravi-k (platform)", WhereWindow, s1, "id")
	b.needle("N5", "Medium R-07 score 9 (likelihood 3 x impact 3)", WhereWindow, s1, "number")

	return Sample{
		ID: "R29", Class: "plan", Lang: "en", Control: false,
		LenAssert: LenAssert{Min: 4400, Max: 6400},
		Segments:  b.segs, Needles: b.needles,
		Probe: Probe{Q: "What are the two high risks, their mitigations, and the trigger threshold?", ExpectRegex: "R-01.*R-02.*0\\.02%", Grader: "regex"},
	}
}

// ---------------------------------------------------------------- R30 en（对照）

// R30 en · 对照：5 条 acceptance criteria，约 1950 token ⇒ Budget=8192 未撞线。
func buildR30() Sample {
	b := &sb{}

	b.addP(compress.KindObservation, false, true, obsBlock(900, []string{
		"# acceptance evidence stub (not needed for the decision)",
	}, func(i int) string {
		return fmt.Sprintf("  evidence stub %02d: captured during the dry run, no verdict attached", i)
	}), "captured during the dry run")

	s1 := b.addP(compress.KindPlan, true, false, obsBlock(1000, []string{
		"# acceptance criteria (5)",
		"AC1: every migrated shard returns identical checksums on both paths",
		"AC2: no request errors above the pre-migration baseline",
		"AC3: rollback completes in under 90 seconds",
		"AC4: the shadow queue drains within one minute of cutover",
		"AC5: p99 read latency does not regress by more than 5%",
		"test command: go test ./internal/migrate -run TestDualRead -count=1",
		"owner: @lin-ops",
	}, func(i int) string {
		return fmt.Sprintf("  ac note %02d: wording fixed, thresholds above are the contract", i)
	}), "AC1: every migrated shard", "@lin-ops")

	b.needle("N1", "AC1: every migrated shard returns identical checksums on both paths", WhereWindow, s1, "decision")
	b.needle("N2", "AC3: rollback completes in under 90 seconds", WhereWindow, s1, "decision")
	b.needle("N3", "AC5: p99 read latency does not regress by more than 5%", WhereWindow, s1, "number")
	b.needle("N4", "test command: go test ./internal/migrate -run TestDualRead -count=1", WhereWindow, s1, "cmd")
	b.needle("N5", "owner: @lin-ops", WhereWindow, s1, "id")

	return Sample{
		ID: "R30", Class: "plan", Lang: "en", Control: true,
		LenAssert: LenAssert{Min: 1800, Max: 2400},
		Segments:  b.segs, Needles: b.needles,
		Probe: Probe{Q: "What are the acceptance criteria and the test command?", ExpectRegex: "AC1.*TestDualRead", Grader: "regex"},
	}
}
