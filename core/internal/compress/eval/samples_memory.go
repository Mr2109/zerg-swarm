package eval

import (
	"fmt"

	"github.com/Mr2109/zerg-swarm/core/internal/compress"
)

// 本文件是样本面第四类：**memory state（长期记忆 / state.md 段落）** R31–R40
// （任务单 3.2 D 表）。
//
// 构造纪律（任务单 §3.1）：
//   规1 类内 zh=R31–R35 / en=R36–R40，各 5
//   规2 对照 = R35 / R40，1800–2400 token；其余 4400–6400 token
//   规4 每样本 5 条 needle = **5 window + 0 recoverable**（任务单 3.3：memory 配额 W50/R0）
//   规7 memory 钉「决策账与偏好禁区」；讨论过程 / 逐轮纪要 / 证据正文不钉、可卸载
//   规8 可卸载段必给 raw_ptr（内容寻址）
//
// ★ 与 plan 类同形的硬约束：memory 的 5 条 needle 全在 Pinned 的 KindMemory 段（决策句、
// 「已废」反转、禁令、术语定义）。PrunerFree 的两道闸只挑未钉的 KindObservation
// ⇒ 四档扫描里 memory 类不该丢 needle。真丢就是引擎改写了钉住段（红）。
//
// ★ 记忆类特有的构造点：**反转记录**（旧结论必须带「已废」标记）与**禁令**（不许）。
// 这两类句子是「压掉就会导致返工」的典型，因此全部落进 window needle，不用 recoverable
// 兜底 —— 一条「已废」的判断丢了，比丢一段日志更贵。

// MemorySamples —— memory 类 10 条（确定性：同一二进制两次调用逐字节相同）。
func MemorySamples() []Sample {
	all := []Sample{
		buildR31(), buildR32(), buildR33(), buildR34(), buildR35(),
		buildR36(), buildR37(), buildR38(), buildR39(), buildR40(),
	}
	for i := range all {
		all[i].finalize()
	}
	return all
}

// ---------------------------------------------------------------- R31 zh

// R31 zh · state.md 5 段决策账（日期＋拍板人＋结论，含 1 条反转记录，全段 Pinned）。
func buildR31() Sample {
	b := &sb{}

	b.addP(compress.KindObservation, false, true, obsBlock(2400, []string{
		"# 讨论过程与逐轮纪要（决策账见下一段；本段只是过程记录）",
	}, func(i int) string {
		switch i % 3 {
		case 0:
			return fmt.Sprintf("纪要 %02d：会上一共提了 %d 个方案，其中 %d 个被当场否掉", i, 3+i%4, i%3)
		case 1:
			return fmt.Sprintf("纪要 %02d：运维担心灰度窗口与月末结算冲突，讨论未收敛", i)
		default:
			return fmt.Sprintf("纪要 %02d：财务重申计费口径不变，任何人不得顺带改", i)
		}
	}), "纪要 00", "月末结算")

	s1 := b.addP(compress.KindMemory, true, false, obsBlock(1900, []string{
		"# state.md · 决策账（5 段，含 1 条反转；旧结论已标「已废」，不得再引用）",
		"决策 D1（2026-08-30，拍板人：产品）：先做只读导出，写路径下一版",
		"决策 D2（2026-09-05，拍板人：产品）：导出改成实时推送（**已于 D3 反转**）",
		"决策 D3（2026-09-14，拍板人：架构组）：导出走按天分片，实时推送撤回",
		"决策 D4（2026-10-02，拍板人：运维）：灰度窗口固定在周四 22:00",
		"已废 D2：早期结论「导出用实时推送」已被 D3 反转，任何引用都算错",
		"决策 D5（2026-10-11，拍板人：财务）：计费口径冻结，改动需财务会签",
		"证据指针：docs/zh/导出-设计.md#决策账（逐条可回查）",
	}, func(i int) string {
		return fmt.Sprintf("  决策账附注 %02d：本条的讨论细节不记入账，只留结论与拍板人", i)
	}), "决策 D3（2026-09-14", "docs/zh/导出-设计.md#决策账")

	b.addP(compress.KindObservation, false, true, obsBlock(700, []string{
		"# 纪要索引（按日期归档的原始纪要文件）",
	}, func(i int) string {
		return fmt.Sprintf("  归档 %02d：meeting-2026-%02d-%02d.md，出席 %d 人", i, i%12+1, i%28+1, 3+i%6)
	}), "meeting-2026-01-01.md")

	b.needle("N1", "决策 D3（2026-09-14，拍板人：架构组）：导出走按天分片", WhereWindow, s1, "decision")
	b.needle("N2", "决策 D4（2026-10-02，拍板人：运维）：灰度窗口固定在周四 22:00", WhereWindow, s1, "number")
	b.needle("N3", "已废 D2：早期结论「导出用实时推送」已被 D3 反转", WhereWindow, s1, "decision")
	b.needle("N4", "决策 D5（2026-10-11，拍板人：财务）：计费口径冻结", WhereWindow, s1, "decision")
	b.needle("N5", "docs/zh/导出-设计.md#决策账", WhereWindow, s1, "path")

	return Sample{
		ID: "R31", Class: "memory", Lang: "zh", Control: false,
		LenAssert: LenAssert{Min: 4400, Max: 6400},
		Segments:  b.segs, Needles: b.needles,
		Probe: Probe{Q: "导出的最终决策是哪条，哪条决策已废？", ExpectRegex: "D3.*已废 D2", Grader: "regex"},
	}
}

// ---------------------------------------------------------------- R32 zh

// R32 zh · 术语表 60 条（中英对照 ＋ 定义），其余 50 条按指针可回查。
func buildR32() Sample {
	b := &sb{}

	b.addP(compress.KindObservation, false, true, obsBlock(2600, []string{
		"# 术语表其余 50 条定义正文（核心 10 条见下一段）",
	}, func(i int) string {
		return fmt.Sprintf("术语 T%02d：英文 %s；定义：%d 字以内的解释，示例与出处已归档到术语表附录第 %02d 节",
			i+11, []string{"compaction", "prune", "mask", "fold", "pin", "anchor", "budget", "headroom"}[i%8], 40+i%40, i%60)
	}), "术语 T11", "术语表附录")

	s1 := b.addP(compress.KindMemory, true, false, obsBlock(1800, []string{
		"# 术语表核心 10 条（中英对照 ＋ 定义；易混词单列）",
		"软隐藏：把原文换成指针，原文仍可逐字节回查",
		"必留闸：压缩前后逐条比对，任一不满足即整次作废",
		"压缩比（compression_ratio）：tokens_before / tokens_after，> 1 才表示压下去（实测基线 0.445）",
		"误混：卸载（unload）≠ 删除（delete）—— 前者窗口里只留指针，盘上有原件",
		"术语映射：HeadroomPct 中文作「余量闸线」",
		"软隐藏与折叠的区别：折叠保头尾、无指针，卸载换占位符、有指针",
		"必留闸与 M5 的区别：闸在引擎内（失败即回退），M5 是尺子上的独立复算",
		"钉住（pinned）与最近轮（keep_recent）的区别：前者按类，后者按位置",
	}, func(i int) string {
		return fmt.Sprintf("  术语附注 %02d：定义以中文为准，英文只做检索键", i)
	}), "软隐藏：把原文换成指针", "余量闸线")

	b.addP(compress.KindObservation, false, true, obsBlock(700, []string{
		"# 术语表附录索引",
	}, func(i int) string {
		return fmt.Sprintf("  附录 %02d：交叉引用 T%02d–T%02d，含 %d 个反例", i, i%10+11, i%10+31, i%4)
	}), "交叉引用 T11")

	b.needle("N1", "软隐藏：把原文换成指针，原文仍可逐字节回查", WhereWindow, s1, "decision")
	b.needle("N2", "必留闸：压缩前后逐条比对，任一不满足即整次作废", WhereWindow, s1, "decision")
	b.needle("N3", "压缩比（compression_ratio）：tokens_before / tokens_after，> 1 才表示压下去", WhereWindow, s1, "number")
	b.needle("N4", "误混：卸载（unload）≠ 删除（delete）", WhereWindow, s1, "decision")
	b.needle("N5", "术语映射：HeadroomPct 中文作「余量闸线」", WhereWindow, s1, "path")

	return Sample{
		ID: "R32", Class: "memory", Lang: "zh", Control: false,
		LenAssert: LenAssert{Min: 4400, Max: 6400},
		Segments:  b.segs, Needles: b.needles,
		Probe: Probe{Q: "压缩比的方向怎么判，卸载和删除的区别是什么？", ExpectRegex: "compression_ratio.*卸载（unload）", Grader: "regex"},
	}
}

// ---------------------------------------------------------------- R33 zh

// R33 zh · 用户偏好 25 条（风格 / 工具 / 禁区，禁区段 Pinned）。
func buildR33() Sample {
	b := &sb{}

	b.addP(compress.KindObservation, false, true, obsBlock(2300, []string{
		"# 偏好条目的解释性句子与出处（禁令正文在下一段）",
	}, func(i int) string {
		switch i % 3 {
		case 0:
			return fmt.Sprintf("出处 %02d：来自 2026 年 %02d 月的复盘记录，原话已归档", i, i%12+1)
		case 1:
			return fmt.Sprintf("风格示例 %02d：报告先给结论，再给证据，最后给待办", i)
		default:
			return fmt.Sprintf("风格示例 %02d：数字后面跟分母，表格不换行，标题不用问句", i)
		}
	}), "出处 00", "复盘记录")

	s1 := b.addP(compress.KindMemory, true, false, obsBlock(1700, []string{
		"# 用户偏好（25 条：风格 / 工具 / 禁区；禁区全列在此）",
		"不许：未经允许执行 git push 或 git commit",
		"不许：改动 ~/.zerg/state 下的真身文件",
		"不许：在没有回滚命令的情况下跑迁移",
		"不许：为了让指标好看而放宽引擎阈值",
		"不许：把「没测到」写成「没问题」",
		"不许：用同义词替换代替逐字命中",
		"硬偏好：所有命令必须先 cd 到仓根再跑",
		"硬偏好：报告里数字必须带分母（如 18/20）",
		"硬偏好：先读设计稿再动手，不猜接口",
	}, func(i int) string {
		return fmt.Sprintf("  偏好附注 %02d：解释写在出处段，本段只留可判定的句子", i)
	}), "不许：未经允许执行 git push", "硬偏好：所有命令必须先 cd 到仓根")

	b.addP(compress.KindObservation, false, true, obsBlock(700, []string{
		"# 偏好条目索引（含最近一次违反记录）",
	}, func(i int) string {
		return fmt.Sprintf("  记录 %02d：条目 %02d 上周被违反 %d 次，已当面纠正", i, i%25+1, i%2)
	}), "最近一次违反记录")

	b.needle("N1", "不许：未经允许执行 git push 或 git commit", WhereWindow, s1, "decision")
	b.needle("N2", "不许：改动 ~/.zerg/state 下的真身文件", WhereWindow, s1, "path")
	b.needle("N3", "不许：在没有回滚命令的情况下跑迁移", WhereWindow, s1, "decision")
	b.needle("N4", "硬偏好：所有命令必须先 cd 到仓根再跑", WhereWindow, s1, "decision")
	b.needle("N5", "硬偏好：报告里数字必须带分母（如 18/20）", WhereWindow, s1, "number")

	return Sample{
		ID: "R33", Class: "memory", Lang: "zh", Control: false,
		LenAssert: LenAssert{Min: 4400, Max: 6400},
		Segments:  b.segs, Needles: b.needles,
		Probe: Probe{Q: "六条禁令分别是什么？", ExpectRegex: "不许：未经允许.*不许：改动 ~/.zerg", Grader: "regex"},
	}
}

// ---------------------------------------------------------------- R34 zh

// R34 zh · 历史结论账 40 条（结论 ＋ 证据指针），其中 3 条已被推翻。
func buildR34() Sample {
	b := &sb{}

	b.addP(compress.KindObservation, false, true, obsBlock(2500, []string{
		"# 结论的证据正文（只留正文，结论与指针在下一段）",
	}, func(i int) string {
		switch i % 3 {
		case 0:
			return fmt.Sprintf("证据正文 %02d：跑批 %d 次，样本 %d 条，读数已归档到跑批目录", i, 3+i%5, 20+i%30)
		case 1:
			return fmt.Sprintf("证据正文 %02d：单测覆盖 %d 个用例，其中 %d 个是负控", i, 40+i%60, i%9)
		default:
			return fmt.Sprintf("证据正文 %02d：复跑两次的确定性指纹一致，差异 0 处", i)
		}
	}), "证据正文 00", "确定性指纹")

	s1 := b.addP(compress.KindMemory, true, false, obsBlock(1900, []string{
		"# 历史结论账（40 条：10 条仍有效、3 条已废、其余仅存档）",
		"结论 C-07 仍有效：free 引擎在 8192 档不丢必留信息",
		"结论 C-11 仍有效：卸载原件按 sha256 前 16 位寻址",
		"已废 C-03：free 在低预算会死循环（2026-10-23 已修复）",
		"已废 C-08：summary 引擎默认开启（从未开启，默认关）",
		"已废 C-12：压缩比按字节算（口径已改为 token）",
		"证据指针：core/internal/compress/pruner_free.go#unload",
		"已废结论 3 条，仍有效 10 条，共 13 条入账",
		"# 未完成 todo：其余 27 条只留档，等有人要再回查",
	}, func(i int) string {
		return fmt.Sprintf("  结论账附注 %02d：本段只留结论句与状态，证据正文在上段", i)
	}), "结论 C-07 仍有效", "core/internal/compress/pruner_free.go#unload")

	b.addP(compress.KindObservation, false, true, obsBlock(700, []string{
		"# 结论账导出尾部（按有效性分组计数）",
	}, func(i int) string {
		return fmt.Sprintf("  group %02d: state=%s count=%d", i, []string{"valid", "void", "archived"}[i%3], i%9+1)
	}), "state=valid")

	b.needle("N1", "结论 C-07 仍有效：free 引擎在 8192 档不丢必留信息", WhereWindow, s1, "decision")
	b.needle("N2", "结论 C-11 仍有效：卸载原件按 sha256 前 16 位寻址", WhereWindow, s1, "decision")
	b.needle("N3", "已废 C-03：free 在低预算会死循环（2026-10-23 已修复）", WhereWindow, s1, "decision")
	b.needle("N4", "core/internal/compress/pruner_free.go#unload", WhereWindow, s1, "path")
	b.needle("N5", "已废结论 3 条，仍有效 10 条", WhereWindow, s1, "number")

	return Sample{
		ID: "R34", Class: "memory", Lang: "zh", Control: false,
		LenAssert: LenAssert{Min: 4400, Max: 6400},
		Segments:  b.segs, Needles: b.needles,
		Probe: Probe{Q: "哪些结论已废，仍有效的有几条？", ExpectRegex: "已废 C-03.*仍有效 10 条", Grader: "regex"},
	}
}

// ---------------------------------------------------------------- R35 zh（对照）

// R35 zh · 对照：短 backlog 8 条，约 1900 token ⇒ Budget=8192 未撞线。
func buildR35() Sample {
	b := &sb{}

	b.addP(compress.KindObservation, false, true, obsBlock(900, []string{
		"# backlog 备注（不含判定信息）",
	}, func(i int) string {
		return fmt.Sprintf("备注 %02d：条目按周例会过一遍，过期即删", i)
	}), "条目按周例会过一遍")

	s1 := b.addP(compress.KindMemory, true, false, obsBlock(1000, []string{
		"# 短 backlog（8 条待办 ＋ 优先级）",
		"待办 B-1：补齐 observation 类样本",
		"待办 B-3：补齐 representation 类样本",
		"待办 B-8：把扫描产物落到 state",
		"待办 B-2：修 summary 引擎的降级路径",
		"优先级：B-1 > B-3 > B-2",
		"owner：@li-si",
		"截止 2026-10-30",
		"备注：B-8 依赖 B-1 与 B-3 完成",
	}, func(i int) string {
		return fmt.Sprintf("  待办附注 %02d：说明文字已省略，只留动作", i)
	}), "待办 B-8：把扫描产物落到 state", "@li-si")

	b.needle("N1", "待办 B-3：补齐 representation 类样本", WhereWindow, s1, "decision")
	b.needle("N2", "优先级：B-1 > B-3 > B-2", WhereWindow, s1, "decision")
	b.needle("N3", "待办 B-8：把扫描产物落到 state", WhereWindow, s1, "decision")
	b.needle("N4", "owner：@li-si", WhereWindow, s1, "id")
	b.needle("N5", "截止 2026-10-30", WhereWindow, s1, "number")

	return Sample{
		ID: "R35", Class: "memory", Lang: "zh", Control: true,
		LenAssert: LenAssert{Min: 1800, Max: 2400},
		Segments:  b.segs, Needles: b.needles,
		Probe: Probe{Q: "backlog 的优先级和截止日是什么？", ExpectRegex: "B-1 > B-3 > B-2.*2026-10-30", Grader: "regex"},
	}
}

// ---------------------------------------------------------------- R36 en

// R36 en · decisions log 20 条（ADR 风格，含 2 个 superseded 链）。
func buildR36() Sample {
	b := &sb{}

	b.addP(compress.KindObservation, false, true, obsBlock(2400, []string{
		"# ADR context and consequences sections (bodies only; decisions below)",
	}, func(i int) string {
		switch i % 3 {
		case 0:
			return fmt.Sprintf("context %02d: the team debated %d options over %d sessions", i, 3+i%4, 1+i%5)
		case 1:
			return fmt.Sprintf("consequence %02d: operational load shifts to the weekly rotation", i)
		default:
			return fmt.Sprintf("consequence %02d: rollback requires one config flip, no data loss", i)
		}
	}), "context 00", "weekly rotation")

	s1 := b.addP(compress.KindMemory, true, false, obsBlock(2100, []string{
		"# decisions log (20 ADRs; 2 superseded chains marked below)",
		"ADR-014 (accepted): the ruler scores only through compress.Apply",
		"ADR-011 (accepted): unload uses sha256[:16] content addressing",
		"ADR-009 is superseded by ADR-011",
		"ADR-006 is superseded by ADR-014",
		"ADR-002 (accepted): token estimates use a single estimator",
		"review deadline 2026-11-06",
		"evidence pointer: docs/zh/adr-011.md",
		"# open: ADR-015 (external engine gating) is still a draft",
	}, func(i int) string {
		return fmt.Sprintf("  log note %02d: context and consequences live in the segment above", i)
	}), "ADR-014 (accepted)", "docs/zh/adr-011.md")

	b.addP(compress.KindObservation, false, true, obsBlock(700, []string{
		"# ADR index export tail",
	}, func(i int) string {
		return fmt.Sprintf("  adr %03d: status=%s owner=@adr-%02d", i, []string{"accepted", "superseded", "draft"}[i%3], i%9)
	}), "status=accepted")

	b.needle("N1", "ADR-014 (accepted): the ruler scores only through compress.Apply", WhereWindow, s1, "decision")
	b.needle("N2", "ADR-011 (accepted): unload uses sha256[:16] content addressing", WhereWindow, s1, "id")
	b.needle("N3", "ADR-009 is superseded by ADR-011", WhereWindow, s1, "decision")
	b.needle("N4", "ADR-006 is superseded by ADR-014", WhereWindow, s1, "decision")
	b.needle("N5", "review deadline 2026-11-06", WhereWindow, s1, "number")

	return Sample{
		ID: "R36", Class: "memory", Lang: "en", Control: false,
		LenAssert: LenAssert{Min: 4400, Max: 6400},
		Segments:  b.segs, Needles: b.needles,
		Probe: Probe{Q: "Which ADRs are accepted and which chains are superseded?", ExpectRegex: "ADR-011.*superseded by ADR-014", Grader: "regex"},
	}
}

// ---------------------------------------------------------------- R37 en

// R37 en · glossary 70 条（含 2 个 deprecated 别名）。
func buildR37() Sample {
	b := &sb{}

	b.addP(compress.KindObservation, false, true, obsBlock(2600, []string{
		"# glossary extended entries (58 of 70; core entries in the next segment)",
	}, func(i int) string {
		return fmt.Sprintf("entry G%02d: term %s; definition kept under %d words, examples archived in appendix %02d",
			i+13, []string{"fold", "mask", "pin", "anchor", "budget", "headroom", "dedup", "journal"}[i%8], 30+i%30, i%60)
	}), "entry G13", "appendix 00")

	s1 := b.addP(compress.KindMemory, true, false, obsBlock(1800, []string{
		"# glossary core entries (12) and deprecated aliases (2)",
		"soft-hide: replace the text with a pointer, the original stays byte-recoverable",
		"pinned gate: byte-compare every kept segment; one mismatch voids the run",
		"deprecated alias: compaction ratio, use compression_ratio",
		"deprecated alias: eviction, use unload",
		"anchor: a file path, command or identifier that must never be dropped",
		"keep_recent: the last N segments are exempt from the headroom gate",
		"aggressive: light for free, balanced for summary, hard for external",
		"see docs/en/glossary.md (70 entries)",
	}, func(i int) string {
		return fmt.Sprintf("  glossary note %02d: the English definition is the retrieval key", i)
	}), "soft-hide: replace the text with a pointer", "docs/en/glossary.md")

	b.addP(compress.KindObservation, false, true, obsBlock(700, []string{
		"# glossary appendix index",
	}, func(i int) string {
		return fmt.Sprintf("  appendix %02d: cross references G%02d-G%02d, %d counter-examples", i, i%12+1, i%12+31, i%4)
	}), "counter-examples")

	b.needle("N1", "soft-hide: replace the text with a pointer, the original stays byte-recoverable", WhereWindow, s1, "decision")
	b.needle("N2", "pinned gate: byte-compare every kept segment; one mismatch voids the run", WhereWindow, s1, "decision")
	b.needle("N3", "deprecated alias: compaction ratio, use compression_ratio", WhereWindow, s1, "decision")
	b.needle("N4", "deprecated alias: eviction, use unload", WhereWindow, s1, "id")
	b.needle("N5", "see docs/en/glossary.md (70 entries)", WhereWindow, s1, "path")

	return Sample{
		ID: "R37", Class: "memory", Lang: "en", Control: false,
		LenAssert: LenAssert{Min: 4400, Max: 6400},
		Segments:  b.segs, Needles: b.needles,
		Probe: Probe{Q: "What do soft-hide and the pinned gate mean, and which aliases are deprecated?", ExpectRegex: "soft-hide.*deprecated alias: eviction", Grader: "regex"},
	}
}

// ---------------------------------------------------------------- R38 en

// R38 en · user preferences 30 条（含 8 条 prohibition，禁区段 Pinned）。
func buildR38() Sample {
	b := &sb{}

	b.addP(compress.KindObservation, false, true, obsBlock(2300, []string{
		"# rationale and examples behind the preferences (prohibitions below)",
	}, func(i int) string {
		switch i % 3 {
		case 0:
			return fmt.Sprintf("rationale %02d: the last incident came from an unchecked assumption", i)
		case 1:
			return fmt.Sprintf("example %02d: a report that led with the verdict and then the numbers", i)
		default:
			return fmt.Sprintf("example %02d: a table where every rate carried its denominator", i)
		}
	}), "rationale 00", "unchecked assumption")

	s1 := b.addP(compress.KindMemory, true, false, obsBlock(1800, []string{
		"# user preferences (30 items; all prohibitions listed here)",
		"Never: run git commit or git push without explicit approval",
		"Never: write under ~/.zerg/state without an isolated ZERG_STATE_DIR",
		"Never: change engine thresholds to make a metric look better",
		"Never: report a rate without its denominator (e.g. 0/50 is not 0%)",
		"Never: substitute a paraphrase for an exact substring match",
		"Never: claim a metric passed when its denominator is zero",
		"Never: merge sample construction with engine tuning",
		"Never: skip the design doc and guess the interface",
		"Must use: compress.EstimateTokens as the only token estimator",
		"Must use: a rollback command before any migration step",
		"Must use: the class rule when deciding what to pin",
	}, func(i int) string {
		return fmt.Sprintf("  preference note %02d: only the sentences above are enforceable", i)
	}), "Never: run git commit", "compress.EstimateTokens")

	b.addP(compress.KindObservation, false, true, obsBlock(700, []string{
		"# preference index (last violation recorded)",
	}, func(i int) string {
		return fmt.Sprintf("  record %02d: item %02d violated %d time(s) last month", i, i%30+1, i%2)
	}), "last violation recorded")

	b.needle("N1", "Never: run git commit or git push without explicit approval", WhereWindow, s1, "decision")
	b.needle("N2", "Never: write under ~/.zerg/state without an isolated ZERG_STATE_DIR", WhereWindow, s1, "path")
	b.needle("N3", "Never: change engine thresholds to make a metric look better", WhereWindow, s1, "decision")
	b.needle("N4", "Must use: compress.EstimateTokens as the only token estimator", WhereWindow, s1, "decision")
	b.needle("N5", "Never: report a rate without its denominator (e.g. 0/50 is not 0%)", WhereWindow, s1, "number")

	return Sample{
		ID: "R38", Class: "memory", Lang: "en", Control: false,
		LenAssert: LenAssert{Min: 4400, Max: 6400},
		Segments:  b.segs, Needles: b.needles,
		Probe: Probe{Q: "List the prohibitions and the must-use tools.", ExpectRegex: "Never: run git commit.*Must use: compress\\.EstimateTokens", Grader: "regex"},
	}
}

// ---------------------------------------------------------------- R39 en

// R39 en · lessons learned 25 条（含事故日期与监控指标）。
func buildR39() Sample {
	b := &sb{}

	b.addP(compress.KindObservation, false, true, obsBlock(2500, []string{
		"# incident narratives and postmortem timelines (lessons below)",
	}, func(i int) string {
		switch i % 3 {
		case 0:
			return fmt.Sprintf("timeline %02d: alert fired, on-call acked in %d min, mitigation at %d min", i, 1+i%9, 8+i%40)
		case 1:
			return fmt.Sprintf("timeline %02d: rollback executed, service recovered after %d min", i, 3+i%20)
		default:
			return fmt.Sprintf("review note %02d: the postmortem ran %d days after the incident", i, 2+i%9)
		}
	}), "timeline 00", "on-call acked")

	s1 := b.addP(compress.KindMemory, true, false, obsBlock(1700, []string{
		"# lessons learned (25 items; never-rules and incident ids below)",
		"Never ship a loop without a progress check (INC-2026-014)",
		"Never treat 0 events as 0 value (INC-2025-031)",
		"Never pin by default; pin by class rule",
		"monitor: compaction_failed rate on the chat_obs stream",
		"fix landed in 3 commits, last 44db50a09",
		"# open: the runbook still lacks the low-budget replay section",
	}, func(i int) string {
		return fmt.Sprintf("  lesson note %02d: the narrative lives above, this line is bookkeeping", i)
	}), "Never ship a loop", "44db50a09")

	b.addP(compress.KindObservation, false, true, obsBlock(700, []string{
		"# lesson index export tail",
	}, func(i int) string {
		return fmt.Sprintf("  lesson %02d: severity=%s incidents=%d", i, []string{"sev1", "sev2", "sev3"}[i%3], i%4)
	}), "severity=sev1")

	b.needle("N1", "Never ship a loop without a progress check (INC-2026-014)", WhereWindow, s1, "decision")
	b.needle("N2", "Never treat 0 events as 0 value (INC-2025-031)", WhereWindow, s1, "decision")
	b.needle("N3", "monitor: compaction_failed rate on the chat_obs stream", WhereWindow, s1, "id")
	b.needle("N4", "Never pin by default; pin by class rule", WhereWindow, s1, "decision")
	b.needle("N5", "fix landed in 3 commits, last 44db50a09", WhereWindow, s1, "number")

	return Sample{
		ID: "R39", Class: "memory", Lang: "en", Control: false,
		LenAssert: LenAssert{Min: 4400, Max: 6400},
		Segments:  b.segs, Needles: b.needles,
		Probe: Probe{Q: "Which never-rules and incident ids were recorded?", ExpectRegex: "INC-2026-014.*INC-2025-031", Grader: "regex"},
	}
}

// ---------------------------------------------------------------- R40 en（对照）

// R40 en · 对照：6 条 memory，约 1900 token ⇒ Budget=8192 未撞线。
func buildR40() Sample {
	b := &sb{}

	b.addP(compress.KindObservation, false, true, obsBlock(900, []string{
		"# memory notes (no verdicts here)",
	}, func(i int) string {
		return fmt.Sprintf("  note %02d: reviewed at the weekly sync, nothing to change", i)
	}), "weekly sync")

	s1 := b.addP(compress.KindMemory, true, false, obsBlock(1000, []string{
		"# memory (6 facts kept from earlier sessions)",
		"fact: the free engine never summarises",
		"fact: raw pointers are content addressed with sha256[:16]",
		"fact: the ruler runs four budgets 8192/4096/2048/1024",
		"fact: M1 and M8 stay N/A without a model endpoint",
		"fact: pinned segments survive every gate by construction",
		"source: docs/en/ruler.md",
	}, func(i int) string {
		return fmt.Sprintf("  memory note %02d: wording fixed, only the facts above count", i)
	}), "the free engine never summarises", "docs/en/ruler.md")

	b.needle("N1", "fact: the free engine never summarises", WhereWindow, s1, "decision")
	b.needle("N2", "fact: raw pointers are content addressed with sha256[:16]", WhereWindow, s1, "decision")
	b.needle("N3", "fact: the ruler runs four budgets 8192/4096/2048/1024", WhereWindow, s1, "number")
	b.needle("N4", "fact: M1 and M8 stay N/A without a model endpoint", WhereWindow, s1, "decision")
	b.needle("N5", "source: docs/en/ruler.md", WhereWindow, s1, "path")

	return Sample{
		ID: "R40", Class: "memory", Lang: "en", Control: true,
		LenAssert: LenAssert{Min: 1800, Max: 2400},
		Segments:  b.segs, Needles: b.needles,
		Probe: Probe{Q: "What do the six memory facts say?", ExpectRegex: "never summarises.*M1 and M8", Grader: "regex"},
	}
}
