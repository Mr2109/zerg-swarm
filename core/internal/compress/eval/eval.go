// Package eval 是上下文压缩的**尺子**（P2 · 最小可跑批）。
//
// 边界（设计稿 v4 §10 / 任务单-P2 §2.1）：
//   - 只 import stdlib ＋ core/internal/compress；
//   - 只经 compress.Apply 一个入口，绝不自己实现剪枝；
//   - 不接模型、不动消费者、不写仓内结果。
//
// 覆盖面（同一包内）：
//   - 样本：五类各 10 条、共 50 条、中英各半、每类含 2 条未撞线对照
//     （R01–R10 observation / R11–R20 trajectory / R21–R30 plan / R31–R40 memory /
//     R41–R50 representation；对照 = 各类的第 5 / 第 10 条）；
//     needle 全批 250 条 = 180 window + 70 recoverable（任务单 3.3 的硬配额）；
//   - 指标：M1 DSR（N/A，无模型端点）/ M2 TPT / M3 LAT / M4 RAT / M5 MRR（win,rec）/
//     M6 RBR / M7 RVS / M8 RPT（N/A，尺子内无工具调用）—— 八条全部出表，
//     不可判定的记 N/A 并带分母，绝不记 0、不推算；
//   - **Budget 扫描**（scan.go）：同批样本只动 Budget 一维跑 8192/4096/2048/1024，
//     用来找「哪一类从哪一档开始把必留信息挤出窗口」—— 8192 单档 MRR 满格时尺子没有
//     分辨力，有临界才有。
//
// ★ 扫描每档跑在子进程里（硬超时）：这是保险丝。P2 期间 free 的余量闸在低档曾存在
// 不终止路径（pruner_free.go 第 ② 步，见 scan_child.go 注释），已修；撞超时的档
// **不出读数**，标 status=timeout —— 不记 0、不推算、不拿别的批次补。
//
// 算式逐条对应任务单 §4，改算式必须同步改 score 相关单测的手算用例。
//
// 样本真源是 samples.go / samples_trajectory.go 里的模板＋种子填充（任务单 规10：
// 不读仓内文件、不拷真代码）；生成物逐字节可复现（TestSamples_Deterministic /
// TestTrajectory_Deterministic），跑批时整批落到 <work>/samples.json 供审计。
package eval

import "github.com/Mr2109/zerg-swarm/core/internal/compress"

// 段落/needle 的固定取值（任务单 2.3 schema）。
const (
	WhereWindow      = "window"      // 硬闸：必须仍在窗口文本里
	WhereRecoverable = "recoverable" // 可经 RawPtr 逐字节取回即可
)

// Segment —— 样本的一段内容（对 compress.Segment 的 JSON 友好镜像）。
//
// 为什么要镜像而不直接用 compress.Segment：后者无 json tag（审计件用大驼峰字段名是
// 刻意为之，见任务单 §7 风险 4），而样本文件是**构造口径真源**，要给人读。
type Segment struct {
	Kind    compress.Kind `json:"kind"`
	Pinned  bool          `json:"pinned"`
	Text    string        `json:"text"`
	RawPtr  string        `json:"raw_ptr,omitempty"`
	Anchors []string      `json:"anchors,omitempty"`
}

// Needle —— 一条必留信息（逐字子串，任务单 规5）。
type Needle struct {
	ID      string `json:"id"`
	Text    string `json:"text"`
	Where   string `json:"where"`
	Segment int    `json:"segment"` // 原文所在段（raw_ptr 取回用它）
	Type    string `json:"type"`    // errcode|path|cmd|id|decision|number
}

// Probe —— 下游探针题（本笔不接模型 ⇒ M1 DSR 记 N/A，题面先随样本入库）。
type Probe struct {
	Q           string `json:"q"`
	ExpectRegex string `json:"expect_regex"`
	Grader      string `json:"grader"`
}

// LenAssert —— EstimateTokens 断言区间（任务单 规3：越界即生成失败，禁「大约」）。
type LenAssert struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

// Sample —— 一条样本。
type Sample struct {
	ID        string    `json:"id"`
	Class     string    `json:"class"`
	Lang      string    `json:"lang"` // zh|en
	Control   bool      `json:"control"`
	LenAssert LenAssert `json:"len_assert"`
	Segments  []Segment `json:"segments"`
	Needles   []Needle  `json:"needles"`
	Probe     Probe     `json:"probe"`
}

// toCompress —— 交给 compress.Apply 的形态。
//
// Source 用样本 id（同一会话同一来源，与 Guard 按 Source 分桶的口径一致）；
// Tokens 先填 0，由 Apply 的 TotalTokens 用 EstimateTokens 补——尺子只用这一个估算器
// （任务单 §7 风险 3：三处估算器混用会造成「同一段文字三个数」）。
func (s Sample) toCompress() []compress.Segment {
	out := make([]compress.Segment, len(s.Segments))
	for i, sg := range s.Segments {
		out[i] = compress.Segment{
			Kind:    sg.Kind,
			Pinned:  sg.Pinned,
			Text:    sg.Text,
			Source:  s.ID,
			RawPtr:  sg.RawPtr,
			Anchors: append([]string(nil), sg.Anchors...),
		}
	}
	return out
}

// Tokens —— 该样本原文的 token 数（尺子唯一口径）。
func (s Sample) Tokens() int {
	t := 0
	for _, sg := range s.Segments {
		t += compress.EstimateTokens(sg.Text)
	}
	return t
}

// CountNeedles —— 按 where 计数（任务单 3.3 的硬配额靠它断言）。
func (s Sample) CountNeedles(where string) int {
	n := 0
	for _, nd := range s.Needles {
		if nd.Where == where {
			n++
		}
	}
	return n
}
