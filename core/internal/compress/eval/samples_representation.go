package eval

import (
	"fmt"

	"github.com/Mr2109/zerg-swarm/core/internal/compress"
)

// 本文件是样本面第五类：**representation（结构表征：树 / 表 / 索引 / 摘要块 / schema）**
// R41–R50（任务单 3.2 E 表）。
//
// 构造纪律（任务单 §3.1）：
//   规1 类内 zh=R41–R45 / en=R46–R50，各 5
//   规2 对照 = R45 / R50，1800–2400 token；其余 4400–6400 token
//   规4 每样本 5 条 needle = **2 window + 3 recoverable**（任务单 3.3：representation 配额 W20/R30）
//   规7 representation **不钉**（走 dedup/fold），但 anchors 必留（路径 / 命令 / 标识符）
//   规8 可卸载段必给 raw_ptr（内容寻址）
//
// ★ 为什么载荷段的 Kind 写 observation 而不是 representation（2026-10-23 实测）：
// `PrunerFree` 的第 ① ② 两道闸都判 `s.Kind != KindObservation ⇒ 跳过`。把本类载荷标成
// `KindRepresentation` 跑一遍四档，读数是 **trigger=overflow 而 after==before、删除比 0.000**
// —— 引擎一段都没卸，MRR 恒 1.000、RVS 分母为 0，尺子对该类零分辨力。
// 树 / 表 / 索引 / schema 在真实链路上本来**就是工具输出**（`tree`、查询结果、`grep -n`、
// 读回的 schema），所以本类按「到达形态」建模：载荷段 Kind=observation（未钉、带 raw_ptr）。
// 类标签 `class=representation` 描述**内容形态**，不是引擎的 Kind 枚举。
// 这条实测同时是 free 的能力边界读数：**非 observation 的段它一条都不碰**（见报告「读法」）。
//
// ★ 本类是唯一「window needle 落在可卸载段」的类：2 条 window needle 放在约 0.9–0.95k
// token 的**小表征块**里（Budget=8192/4096 时 perOut=2048/1024 ⇒ 碰不到；Budget=2048 时
// perOut=512 ⇒ 被单条闸卸掉）。因此扫描里 representation 的 MRR_win 应当**从 2048 档开始掉**，
// 而 MRR_rec 仍 1.000（小表征块带 raw_ptr，逐字节可取回）。这正是任务单 M5
// 「两口径必须分开报」的用例：窗口丢了 ≠ 信息丢了。
//
// ★ anchors 全填（任务单 3.2 E 表 R43/R48 明确要求）：锚点在 Segment.Anchors 字段上，
// unload 只改 Text、不动 Anchors ⇒ 卸载后 Guard 仍能按锚点复算通过（这一条也顺带证明了
// 「不可逆删除」与「软隐藏」在实现上是两件事）。

// RepresentationSamples —— representation 类 10 条（确定性：同一二进制两次调用逐字节相同）。
func RepresentationSamples() []Sample {
	all := []Sample{
		buildR41(), buildR42(), buildR43(), buildR44(), buildR45(),
		buildR46(), buildR47(), buildR48(), buildR49(), buildR50(),
	}
	for i := range all {
		all[i].finalize()
	}
	return all
}

// ---------------------------------------------------------------- R41 zh

// R41 zh · 目录树 400 行（含注释与禁止写标记）。
func buildR41() Sample {
	b := &sb{}

	b.addP(compress.KindObservation, false, true, obsBlock(3600, []string{
		"# 目录树（400 行，含注释；深层 testdata 子树全部在内）",
		"core/internal/compress/",
		"├── policy.go                # 策略与 Apply 入口",
		"└── pruner_free.go           # free 引擎（单条闸 ＋ 余量闸 ＋ 去重）",
	}, func(i int) string {
		switch i % 3 {
		case 0:
			return fmt.Sprintf("│   ├── testdata/golden/shard-%02d/part-%04d.bin   # golden 夹具，%d KiB", i%12, i, 8+i%400)
		case 1:
			return fmt.Sprintf("│   ├── pkg%02d/module_%04d.go                    # 内部实现，勿直接引用", i%41, i)
		default:
			return fmt.Sprintf("│   └── docs/notes/draft-%03d.md                  # 草稿，未评审", i)
		}
	}), "testdata/golden/shard-00/part-0000.bin", "pruner_free.go")

	b.addP(compress.KindObservation, false, true, obsBlock(900, []string{
		"# 关键目录与写保护（目录树尾部摘要）",
		"必读目录：core/internal/compress",
		"必读目录：core/internal/compress/eval",
		"禁止写：core/internal/contract/*.json 由生成器维护",
		"禁止写：vendor/ 下的任何文件",
		"最大件：data/ruler-pool/blob_0117.bin（412 MiB）",
		"# 统计：400 行目录项，其中 37 项是 testdata 子树",
	}, func(i int) string {
		return fmt.Sprintf("  dir note %02d: 本行只是目录项说明，判定看上面的写保护与最大件", i)
	}), "core/internal/compress/eval", "core/internal/contract/*.json")

	b.needle("N1", "必读目录：core/internal/compress/eval", WhereWindow, 1, "path")
	b.needle("N2", "禁止写：core/internal/contract/*.json 由生成器维护", WhereWindow, 1, "decision")
	b.needle("N3", "testdata/golden/shard-06/part-0018.bin", WhereRecoverable, 0, "path")
	b.needle("N4", "禁止写：vendor/ 下的任何文件", WhereRecoverable, 1, "decision")
	b.needle("N5", "最大件：data/ruler-pool/blob_0117.bin（412 MiB）", WhereRecoverable, 1, "number")

	return Sample{
		ID: "R41", Class: "representation", Lang: "zh", Control: false,
		LenAssert: LenAssert{Min: 4400, Max: 6400},
		Segments:  b.segs, Needles: b.needles,
		Probe: Probe{Q: "哪些目录必读、哪些不许写，最大件是哪个？", ExpectRegex: "禁止写：core/internal/contract.*412 MiB", Grader: "regex"},
	}
}

// ---------------------------------------------------------------- R42 zh

// R42 zh · 指标台账表 80 行 × 6 列。
func buildR42() Sample {
	b := &sb{}

	b.addP(compress.KindObservation, false, true, obsBlock(3500, []string{
		"指标\t口径\t分母\t读数\t阈值\t判定",
	}, func(i int) string {
		return fmt.Sprintf("RAT-%04d\t第 %d 类口径\t%d\t%d.%04d\t%s\t%s",
			i, i%6+1, 10+i%40, i%100, i*37%10000,
			[]string{"≥ 0.50", "= 1.000", "≤ 50 ms", "≤ 800 ms"}[i%4],
			[]string{"达标", "达标", "不达标"}[i%3])
	}), "指标\t口径\t分母\t读数\t阈值\t判定", "RAT-0000")

	b.addP(compress.KindObservation, false, true, obsBlock(950, []string{
		"# 指标台账表头与异常行（80 行中的 3 行异常单列）",
		"指标\t口径\t分母\t读数\t阈值\t判定",
		"RAT-0042\tP95 延迟\t50\t183.4000 ms\t≤ 800 ms\t不达标",
		"RAT-0057\t删除比中位\t10\t0.3121\t≥ 0.50\t不达标",
		"RAT-0063\tRVS\t18\t1.0000\t= 1.000\t达标",
		"合计：20 个指标中 17 个达标",
		"判定：三行不达标均因分母不足 10 条",
	}, func(i int) string {
		return fmt.Sprintf("  table note %02d：备注列已省略，异常行已在上方单列", i)
	}), "RAT-0042\tP95 延迟", "合计：20 个指标中 17 个达标")

	b.needle("N1", "RAT-0042\tP95 延迟\t50\t183.4000 ms\t≤ 800 ms\t不达标", WhereWindow, 1, "number")
	b.needle("N2", "判定：三行不达标均因分母不足 10 条", WhereWindow, 1, "decision")
	b.needle("N3", "RAT-0057\t删除比中位\t10\t0.3121\t≥ 0.50\t不达标", WhereRecoverable, 1, "id")
	b.needle("N4", "RAT-0063\tRVS\t18\t1.0000\t= 1.000\t达标", WhereRecoverable, 1, "id")
	b.needle("N5", "合计：20 个指标中 17 个达标", WhereRecoverable, 1, "number")

	return Sample{
		ID: "R42", Class: "representation", Lang: "zh", Control: false,
		LenAssert: LenAssert{Min: 4400, Max: 6400},
		Segments:  b.segs, Needles: b.needles,
		Probe: Probe{Q: "有几行不达标，分别是哪几行？", ExpectRegex: "RAT-0042.*RAT-0057", Grader: "regex"},
	}
}

// ---------------------------------------------------------------- R43 zh

// R43 zh · 符号索引 300 条 `file:line — 函数`（anchors 全填）。
func buildR43() Sample {
	b := &sb{}

	b.addP(compress.KindObservation, false, true, obsBlock(3600, []string{
		"# 符号索引（300 条 / file:line — 函数），按包前缀分组",
		"core/internal/compress/policy.go:120 DefaultPolicy — 缺省参数",
		"core/internal/compress/pruner_free.go:24 (*PrunerFree).Compress — 免费引擎主体",
	}, func(i int) string {
		switch i % 3 {
		case 0:
			return fmt.Sprintf("core/internal/pkg%02d/symbol_%04d.go:%d Helper%04d — 内部辅助", i%41, i, 10+i%400, i)
		case 1:
			return fmt.Sprintf("core/internal/pkg%02d/symbol_%04d_test.go:%d TestHelper%04d — 用例", i%41, i, 8+i%300, i)
		default:
			return fmt.Sprintf("core/internal/pkg%02d/symbol_%04d.go:%d helper%04dImpl — 实现", i%41, i, 20+i%500, i)
		}
	}), "core/internal/compress/policy.go:151 Apply", "core/internal/compress/pruner_free.go:24")

	b.addP(compress.KindObservation, false, true, obsBlock(900, []string{
		"# 索引核心锚点（12 条）与已删标记",
		"core/internal/compress/policy.go:151 Apply",
		"core/internal/compress/guard.go:22 Guard.Check",
		"core/internal/compress/eval/scan.go:142 RunScan",
		"core/internal/compress/eval/runner.go:166 RunBatch",
		"已删标记：gui/legacy/panel.go:44 的 DrawLegacy 已不存在",
		"# 索引统计：300 条中 12 条是锚点，其余按指针可回查",
	}, func(i int) string {
		return fmt.Sprintf("  index note %02d：索引行描述文字已归档，锚点保留在上面", i)
	}), "core/internal/compress/guard.go:22 Guard.Check", "core/internal/compress/eval/scan.go:142 RunScan")

	b.needle("N1", "core/internal/compress/policy.go:151 Apply", WhereWindow, 1, "path")
	b.needle("N2", "已删标记：gui/legacy/panel.go:44 的 DrawLegacy 已不存在", WhereWindow, 1, "decision")
	b.needle("N3", "core/internal/compress/guard.go:22 Guard.Check", WhereRecoverable, 1, "path")
	b.needle("N4", "core/internal/compress/eval/scan.go:142 RunScan", WhereRecoverable, 1, "path")
	b.needle("N5", "core/internal/compress/eval/runner.go:166 RunBatch", WhereRecoverable, 1, "id")

	return Sample{
		ID: "R43", Class: "representation", Lang: "zh", Control: false,
		LenAssert: LenAssert{Min: 4400, Max: 6400},
		Segments:  b.segs, Needles: b.needles,
		Probe: Probe{Q: "Apply 与 RunScan 的 file:line 分别是什么？", ExpectRegex: "policy\\.go:151.*scan\\.go:142", Grader: "regex"},
	}
}

// ---------------------------------------------------------------- R44 zh

// R44 zh · 六段结构化摘要块两份（goal/progress/key_decisions/relevant_files/
// next_steps/critical_context），含 summary_sha256 与 field 缺失说明。
func buildR44() Sample {
	b := &sb{}

	b.addP(compress.KindObservation, false, true, obsBlock(3500, []string{
		"# 第一份六段摘要块（模板示例正文，字段名与取值）",
		"goal: 把上下文压缩的尺子补齐到 50 条样本",
		"progress: 两类已落（observation/trajectory），三类待补",
		"key_decisions: 只经 compress.Apply 一个入口；不改引擎阈值",
		"relevant_files: core/internal/compress/eval/samples.go",
		"next_steps: 补 plan / memory / representation 三类，跑四档扫描",
		"critical_context: 缺模型端点 ⇒ M1 记 N/A，不记 0",
	}, func(i int) string {
		return fmt.Sprintf("  摘要块示例行 %02d：这一段是模板正文，字段值以真实摘要为准", i)
	}), "core/internal/compress/eval/samples.go", "critical_context")

	b.addP(compress.KindObservation, false, true, obsBlock(950, []string{
		"# 第二份摘要块的段名与校验（六段名全列）",
		"段名：goal / progress / key_decisions / relevant_files / next_steps / critical_context",
		"summary_sha256=9c1e4a7f2b88",
		"field 缺失说明：第二份摘要缺 relevant_files 段",
		"next_steps（3 条，含 1 条回滚步骤）",
		"critical_context：低预算下 representation 会被整段卸载，须回查",
	}, func(i int) string {
		return fmt.Sprintf("  block note %02d：模板示例正文在上一段，本段只留段名与校验值", i)
	}), "summary_sha256=9c1e4a7f2b88", "field 缺失说明")

	b.needle("N1", "段名：goal / progress / key_decisions / relevant_files / next_steps / critical_context", WhereWindow, 1, "id")
	b.needle("N2", "summary_sha256=9c1e4a7f2b88", WhereWindow, 1, "id")
	b.needle("N3", "field 缺失说明：第二份摘要缺 relevant_files 段", WhereRecoverable, 1, "decision")
	b.needle("N4", "next_steps（3 条，含 1 条回滚步骤）", WhereRecoverable, 1, "number")
	b.needle("N5", "critical_context: 缺模型端点 ⇒ M1 记 N/A", WhereRecoverable, 0, "decision")

	return Sample{
		ID: "R44", Class: "representation", Lang: "zh", Control: false,
		LenAssert: LenAssert{Min: 4400, Max: 6400},
		Segments:  b.segs, Needles: b.needles,
		Probe: Probe{Q: "六段摘要的段名是什么，第二份缺哪一段？", ExpectRegex: "critical_context.*缺 relevant_files", Grader: "regex"},
	}
}

// ---------------------------------------------------------------- R45 zh（对照）

// R45 zh · 对照：短配置清单 25 行，约 1900 token ⇒ Budget=8192 未撞线。
func buildR45() Sample {
	b := &sb{}

	b.addP(compress.KindObservation, false, true, obsBlock(900, []string{
		"# 配置清单的注释与出处（取值在下一段）",
	}, func(i int) string {
		return fmt.Sprintf("注释 %02d：该键的取值以代码里的 DefaultPolicy 为准，配置文件只是覆盖", i)
	}), "DefaultPolicy")

	b.addP(compress.KindObservation, false, true, obsBlock(1000, []string{
		"# 短配置清单（25 行中的关键 5 项 ＋ 默认值出处）",
		"budget = 8192",
		"engine = free",
		"headroom_pct = 0.70",
		"per_output_pct = 0.25",
		"keep_recent = 2",
		"默认值出处：core/internal/compress/policy.go DefaultPolicy",
		"备注：keep_pinned 恒 true，不允许关",
	}, func(i int) string {
		return fmt.Sprintf("  配置附注 %02d：本行是注释，判定看上面的键值", i)
	}), "budget = 8192", "默认值出处：core/internal/compress/policy.go DefaultPolicy")

	b.needle("N1", "budget = 8192", WhereWindow, 1, "number")
	b.needle("N2", "engine = free", WhereWindow, 1, "id")
	b.needle("N3", "headroom_pct = 0.70", WhereRecoverable, 1, "number")
	b.needle("N4", "默认值出处：core/internal/compress/policy.go DefaultPolicy", WhereRecoverable, 1, "path")
	b.needle("N5", "备注：keep_pinned 恒 true，不允许关", WhereRecoverable, 1, "decision")

	return Sample{
		ID: "R45", Class: "representation", Lang: "zh", Control: true,
		LenAssert: LenAssert{Min: 1800, Max: 2400},
		Segments:  b.segs, Needles: b.needles,
		Probe: Probe{Q: "默认 budget 与 headroom 各是多少，默认值出处在哪？", ExpectRegex: "8192.*0\\.70.*DefaultPolicy", Grader: "regex"},
	}
}

// ---------------------------------------------------------------- R46 en

// R46 en · directory tree 450 lines（含注释与 do-not-write）。
func buildR46() Sample {
	b := &sb{}

	b.addP(compress.KindObservation, false, true, obsBlock(3600, []string{
		"# directory tree (450 lines, comments included)",
		"core/internal/compress/",
		"├── policy.go                # Policy, Apply and the three engines seam",
		"└── pruner_free.go           # free engine: per-output gate, headroom gate, dedup",
	}, func(i int) string {
		switch i % 3 {
		case 0:
			return fmt.Sprintf("│   ├── testdata/golden/shard-%02d/part-%04d.bin   # golden fixture, %d KiB", i%12, i, 8+i%400)
		case 1:
			return fmt.Sprintf("│   ├── pkg%02d/module_%04d.go                    # internal, do not import", i%41, i)
		default:
			return fmt.Sprintf("│   └── docs/notes/draft-%03d.md                  # draft, unreviewed", i)
		}
	}), "testdata/golden/shard-00/part-0000.bin", "pruner_free.go")

	b.addP(compress.KindObservation, false, true, obsBlock(900, []string{
		"# key directories and write protection (tail of the tree)",
		"must-read: core/internal/compress/eval",
		"must-read: core/internal/compress/adapter",
		"do-not-write: core/internal/contract/*.json is generated",
		"do-not-write: any file under vendor/",
		"largest file: data/ruler-pool/blob_0219.bin (388 MiB)",
		"# totals: 450 entries, 41 of them under testdata",
	}, func(i int) string {
		return fmt.Sprintf("  dir note %02d: this line is bookkeeping, the guards above decide", i)
	}), "core/internal/compress/eval", "core/internal/contract/*.json")

	b.needle("N1", "must-read: core/internal/compress/eval", WhereWindow, 1, "path")
	b.needle("N2", "do-not-write: core/internal/contract/*.json is generated", WhereWindow, 1, "decision")
	b.needle("N3", "testdata/golden/shard-06/part-0018.bin", WhereRecoverable, 0, "path")
	b.needle("N4", "do-not-write: any file under vendor/", WhereRecoverable, 1, "decision")
	b.needle("N5", "largest file: data/ruler-pool/blob_0219.bin (388 MiB)", WhereRecoverable, 1, "number")

	return Sample{
		ID: "R46", Class: "representation", Lang: "en", Control: false,
		LenAssert: LenAssert{Min: 4400, Max: 6400},
		Segments:  b.segs, Needles: b.needles,
		Probe: Probe{Q: "Which directories are must-read, what is write-protected, and the largest file?", ExpectRegex: "do-not-write.*388 MiB", Grader: "regex"},
	}
}

// ---------------------------------------------------------------- R47 en

// R47 en · markdown table 90 rows（含 4 个 outlier）。
func buildR47() Sample {
	b := &sb{}

	b.addP(compress.KindObservation, false, true, obsBlock(3500, []string{
		"| metric | denom | reading | threshold | verdict |",
		"|---|---|---|---|---|",
	}, func(i int) string {
		return fmt.Sprintf("| T-%03d | %d | %d.%02d | %s | %s |",
			i, 10+i%40, i%100, i*13%100,
			[]string{"<= 0.85x free", "= 1.000", "<= 50 ms", ">= 0.90"}[i%4],
			[]string{"pass", "pass", "fail"}[i%3])
	}), "| metric | denom | reading | threshold | verdict |", "T-000")

	b.addP(compress.KindObservation, false, true, obsBlock(950, []string{
		"# table header and the 4 outlier rows (of 90)",
		"| metric | denom | reading | threshold | verdict |",
		"| TPT-021 | 50 | 6120 | <= 0.85x free | fail |",
		"| LAT-033 | 50 | 183.4 ms | <= 50 ms | fail |",
		"| RVS-008 | 18 | 1.0000 | = 1.000 | pass |",
		"| DSR-012 | 41 | 0.9024 | >= 0.90 | pass |",
		"total: 90 rows, 4 outliers",
		"verdict note: one failure has a denominator below 10",
	}, func(i int) string {
		return fmt.Sprintf("  table note %02d: the notes column is omitted, outliers are above", i)
	}), "| TPT-021 | 50 | 6120 |", "total: 90 rows, 4 outliers")

	b.needle("N1", "| TPT-021 | 50 | 6120 | <= 0.85x free | fail |", WhereWindow, 1, "number")
	b.needle("N2", "verdict note: one failure has a denominator below 10", WhereWindow, 1, "decision")
	b.needle("N3", "| LAT-033 | 50 | 183.4 ms | <= 50 ms | fail |", WhereRecoverable, 1, "number")
	b.needle("N4", "| DSR-012 | 41 | 0.9024 | >= 0.90 | pass |", WhereRecoverable, 1, "id")
	b.needle("N5", "total: 90 rows, 4 outliers", WhereRecoverable, 1, "number")

	return Sample{
		ID: "R47", Class: "representation", Lang: "en", Control: false,
		LenAssert: LenAssert{Min: 4400, Max: 6400},
		Segments:  b.segs, Needles: b.needles,
		Probe: Probe{Q: "How many outliers and which metric rows failed?", ExpectRegex: "TPT-021.*LAT-033", Grader: "regex"},
	}
}

// ---------------------------------------------------------------- R48 en

// R48 en · symbol index 350 条（anchors 全填）。
func buildR48() Sample {
	b := &sb{}

	b.addP(compress.KindObservation, false, true, obsBlock(3600, []string{
		"# symbol index (350 entries / file:line — symbol), grouped by package",
		"core/internal/compress/eval/runner.go:200 RunBatchRow — one row per sample",
		"core/internal/compress/eval/scan.go:178 TierRow — one budget tier",
	}, func(i int) string {
		switch i % 3 {
		case 0:
			return fmt.Sprintf("core/internal/pkg%02d/symbol_%04d.go:%d Helper%04d — internal helper", i%41, i, 10+i%400, i)
		case 1:
			return fmt.Sprintf("core/internal/pkg%02d/symbol_%04d_test.go:%d TestHelper%04d — test case", i%41, i, 8+i%300, i)
		default:
			return fmt.Sprintf("core/internal/pkg%02d/symbol_%04d.go:%d helper%04dImpl — implementation", i%41, i, 20+i%500, i)
		}
	}), "core/internal/compress/eval/runner.go:166 RunBatch", "core/internal/compress/policy.go:151 Apply")

	b.addP(compress.KindObservation, false, true, obsBlock(900, []string{
		"# core anchors (15) and the deleted marker",
		"core/internal/compress/policy.go:151 Apply",
		"core/internal/compress/guard.go:22 Guard.Check",
		"core/internal/compress/eval/runner.go:166 RunBatch",
		"core/internal/compress/eval/scan.go:142 RunScan",
		"deleted marker: api/legacy/handler.go:88 ServeLegacy is gone",
		"# totals: 350 entries, 15 anchors, the rest recoverable by pointer",
	}, func(i int) string {
		return fmt.Sprintf("  index note %02d: descriptions archived, anchors kept above", i)
	}), "core/internal/compress/guard.go:22 Guard.Check", "core/internal/compress/eval/runner.go:166 RunBatch")

	b.needle("N1", "core/internal/compress/policy.go:151 Apply", WhereWindow, 1, "path")
	b.needle("N2", "deleted marker: api/legacy/handler.go:88 ServeLegacy is gone", WhereWindow, 1, "decision")
	b.needle("N3", "core/internal/compress/guard.go:22 Guard.Check", WhereRecoverable, 1, "path")
	b.needle("N4", "core/internal/compress/eval/scan.go:142 RunScan", WhereRecoverable, 1, "id")
	b.needle("N5", "core/internal/compress/eval/runner.go:166 RunBatch", WhereRecoverable, 1, "path")

	return Sample{
		ID: "R48", Class: "representation", Lang: "en", Control: false,
		LenAssert: LenAssert{Min: 4400, Max: 6400},
		Segments:  b.segs, Needles: b.needles,
		Probe: Probe{Q: "Give the file:line for Apply, Check and RunScan.", ExpectRegex: "policy\\.go:151.*guard\\.go:22.*scan\\.go:142", Grader: "regex"},
	}
}

// ---------------------------------------------------------------- R49 en

// R49 en · JSON schema 200 字段（含 3 个 required、2 个 enum、1 个 $id）。
func buildR49() Sample {
	b := &sb{}

	b.addP(compress.KindObservation, false, true, obsBlock(3500, []string{
		"{",
		"  \"$schema\": \"https://json-schema.invalid/draft/2020-12/schema\",",
		"  \"$defs\": {",
	}, func(i int) string {
		return fmt.Sprintf("    \"field_%03d\": {\"type\": \"%s\", \"description\": \"generated field %d, description only\"},",
			i, []string{"string", "integer", "boolean"}[i%3], i)
	}), "\"$defs\"", "\"$schema\"")

	b.addP(compress.KindObservation, false, true, obsBlock(950, []string{
		"  # schema core block (identity, required, enums)",
		"  \"$id\": \"https://schemas.invalid/context-compress/v2.json\",",
		"  \"required\": [\"budget\", \"engine\", \"max_ratio\"],",
		"  \"enum\": [\"free\", \"summary\", \"external\"],",
		"  \"aggressive\": {\"enum\": [\"light\", \"balanced\", \"hard\"]},",
		"  \"minimum\": 1,",
		"  # deprecated field: rotate_pct (use rot_pct)",
		"  \"# total_fields\": 200",
	}, func(i int) string {
		return fmt.Sprintf("  // schema note %02d: field descriptions are generated, the core block decides", i)
	}), "\"$id\": \"https://schemas.invalid/context-compress/v2.json\"", "\"required\": [\"budget\"")

	b.needle("N1", "\"$id\": \"https://schemas.invalid/context-compress/v2.json\"", WhereWindow, 1, "path")
	b.needle("N2", "\"required\": [\"budget\", \"engine\", \"max_ratio\"]", WhereWindow, 1, "id")
	b.needle("N3", "\"enum\": [\"free\", \"summary\", \"external\"]", WhereRecoverable, 1, "id")
	b.needle("N4", "\"minimum\": 1,", WhereRecoverable, 1, "number")
	b.needle("N5", "deprecated field: rotate_pct (use rot_pct)", WhereRecoverable, 1, "decision")

	return Sample{
		ID: "R49", Class: "representation", Lang: "en", Control: false,
		LenAssert: LenAssert{Min: 4400, Max: 6400},
		Segments:  b.segs, Needles: b.needles,
		Probe: Probe{Q: "What is the $id, the required list and the engine enum?", ExpectRegex: "v2\\.json.*budget.*external", Grader: "regex"},
	}
}

// ---------------------------------------------------------------- R50 en（对照）

// R50 en · 对照：short schema 30 字段，约 1900 token ⇒ Budget=8192 未撞线。
func buildR50() Sample {
	b := &sb{}

	b.addP(compress.KindObservation, false, true, obsBlock(900, []string{
		"# schema comments (values in the next segment)",
	}, func(i int) string {
		return fmt.Sprintf("  comment %02d: descriptions are generated from struct tags, not hand written", i)
	}), "struct tags")

	b.addP(compress.KindObservation, false, true, obsBlock(1000, []string{
		"{",
		"  \"$id\": \"https://schemas.invalid/context-compress/v1-short.json\",",
		"  \"required\": [\"budget\", \"engine\"],",
		"  \"enum\": [\"light\", \"balanced\", \"hard\"],",
		"  \"maxLength\": 4096,",
		"  \"# deprecated\": \"aggressive_level (use aggressive)\",",
		"  \"# total_fields\": 30",
		"}",
	}, func(i int) string {
		return fmt.Sprintf("  // schema note %02d: field count stays at 30 in this short form", i)
	}), "\"$id\": \"https://schemas.invalid/context-compress/v1-short.json\"", "\"required\": [\"budget\", \"engine\"]")

	b.needle("N1", "\"$id\": \"https://schemas.invalid/context-compress/v1-short.json\"", WhereWindow, 1, "path")
	b.needle("N2", "\"required\": [\"budget\", \"engine\"]", WhereWindow, 1, "id")
	b.needle("N3", "\"enum\": [\"light\", \"balanced\", \"hard\"]", WhereRecoverable, 1, "id")
	b.needle("N4", "\"maxLength\": 4096,", WhereRecoverable, 1, "number")
	b.needle("N5", "\"# deprecated\": \"aggressive_level (use aggressive)\"", WhereRecoverable, 1, "decision")

	return Sample{
		ID: "R50", Class: "representation", Lang: "en", Control: true,
		LenAssert: LenAssert{Min: 1800, Max: 2400},
		Segments:  b.segs, Needles: b.needles,
		Probe: Probe{Q: "What is the short schema $id, required list and enum?", ExpectRegex: "v1-short\\.json.*balanced", Grader: "regex"},
	}
}
