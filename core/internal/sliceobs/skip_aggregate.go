// skip_aggregate.go — B 项⑥：**越片统计的聚合口径**（设计稿 v2.1 §4.8-2「片遵守度观测」）。
//
// ── 逐字来源（本件现读，不凭记忆）────────────────────────────────────────
//
//	docs/01-设计/设计-协作骨架-v2.1-20260918.md 第 265 行（§4.8 执行侧的反馈回路）：
//
//	  「- **片遵守度观测**：每步动作归到片的相位，统计**越片动作数**；执行循环**每 N 步重述当前片**
//	    （一手证据：arXiv 2604.12147〔预印本〕周期性计划提醒可减少违规并提升成功率；删/改错相位比不给计划更糟。
//	    **N 的取值 = 本仓标定取值（待实测复核）**）。」
//
//	第 111 行同一条（附录/依据表）：「本仓简化为「**越片动作数 + 每 N 步重述**」」。
//
// ── 本件**只做聚合**，不做触发点（硬规则 ✗ 不编造触发条件）──────────────────
//
//	稿面把这条拆成两半：①「每步动作归到片的**相位**」= **相位归属判定**（pipeline 侧，本仓**未落地**：
//	`grep 相位 core/` = 0 命中，登记见 docs/项目文档/v2.5.10/任务表-协作骨架v2.1仓内核对-S2至S3-20260918.md
//	的 §2.4-10「仓内未找到相位归属、越片动作计数、重述频率三件」）；②「统计**越片动作数**」= **聚合**（本件）。
//
//	⇒ 本文件只做 ②：**从已经落盘的片事件序列里数数**。它**不判断**「哪个动作越片」（那是 ① 的事，
//	  没落地就不许猜 ✗），也**不生成**任何越片事件（触发点在执行者侧，本仓未接线 ——
//	  见 sliceobs.go 的 `EmitSliceSkippedByExecutor` 注释）。清一色纯函数：无 IO、无时间、无全局态。
//
// ── 阈值 / N 的读数：只读标定档案，缺键 ⇒ 显式「未标定」，**绝不猜** ✗────────────
//
//	档案 = `~/.zerg/egg-profiles/task-budget-calib.yaml`（任务级预算标定档案；其抬头自述
//	「§8.4：实测得出，闸门**只读档案**；无档案不许当已标定」）。稿面同规：
//	  · §4.8-1「连续 2 步无进展〔**本仓标定取值（待实测复核）**〕……它来自**标定档案，不是写死的常数**」；
//	  · §4.8-2「N 的取值 = **本仓标定取值（待实测复核）**」；
//	  · §6.3「预算数值来自标定档案……闸门只读档案」，且明写「**档案路径与字段清单待 C 稿给出 —— 登记为清单 U5**」。
//
//	⇒ 两处**登记口径**（因为稿面没给键名，本件照 U5 的口径把键名登记出来，不冒充稿面用语）：
//	     `slice_skip_max`          越片动作数阈值（一片内允许的越片动作数上限）
//	     `restate_every_n_steps`   「每 N 步重述当前片」的 N
//	  档案里**没有**这两个键（现档只有 model/calib_runs/measured_at/steps_max/tool_calls_max/wall_s_max/
//	  assistant_tokens_est_max）⇒ 读数一律 `UncalibratedText`（**「未标定」**）。
//	  键在但值不是非负整数（字符串/浮点/负数/junk）⇒ 同样 `nil` ⇒ 「未标定」：**不截断、不四舍五入、
//	  不取默认值**（猜一个数 = 把未标定伪装成已标定 ✗）。
//	  值合法则**如实回显**（含 0 —— 0 是否合法由作者裁定，本件只读、不替作者判，见 `intKey`）。
//
// ── 与「事件名闭集」的关系 ──
//
//	本件的取数只认 §6.1 的第三名（越片动作）`slice_skipped_by_executor`：闭集外的名字**根本落不了盘**
//	（硬规则 ①），所以「混入其它事件不计」这件事在聚合侧仍要显式判一次（输入可能来自旧文件或人工造的序列 ⇒ 不省这一步）。
package sliceobs

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// UncalibratedText — 读数**无来源**时的显式文本。
//
// 与 `CriteriaVersionUncalibrated` **同字面**（稿面一律用「未标定」表示「无来源」），但**语义不同**：
// 那个是「判据版本」这一位的占位，这个是「标定读数」这一位的占位 ⇒ 各自一个常量，**不互相冒充**
// （将来任一处改字，不会把另一处悄悄改掉）。
const UncalibratedText = "未标定"

// ── 标定档案的键名（**登记口径**：稿面未给键名，见文件头 U5 引用）──────────────

const (
	// CalibKeySliceSkipMax — 越片动作数阈值（一片内允许的越片动作数上限）。
	CalibKeySliceSkipMax = "slice_skip_max"
	// CalibKeyRestateEveryNSteps — 「每 N 步重述当前片」的 N。
	CalibKeyRestateEveryNSteps = "restate_every_n_steps"
)

// 读档案失败的**低基数**分类（不把 OS 错误原文带进观测面/日志之外的任何地方）。
const (
	calibReadErrNotExist = "档案不存在"
	calibReadErrEmpty    = "路径为空"
	calibReadErrRead     = "读档案失败"
	calibReadErrDecode   = "YAML 解析失败"
)

// DefaultSliceCalibPath — 任务级预算标定档案的默认路径：`<home>/.zerg/egg-profiles/task-budget-calib.yaml`。
//
// 取不到 home（极端环境）⇒ 返回空串（调用方看到空路径 ⇒ 读数「未标定」，不会误读成「已标定」）。
func DefaultSliceCalibPath() string {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return ""
	}
	return filepath.Join(home, ".zerg", "egg-profiles", "task-budget-calib.yaml")
}

// SliceCalib — 从标定档案读到的两处读数（**缺键 ⇒ nil**，绝不填默认值）。
type SliceCalib struct {
	// Path — 读的是哪个档案（可追溯；空 = 没给路径）。
	Path string `json:"path"`
	// SkipMax — 越片动作数阈值；nil = **未标定**（缺键 / 值不合法 / 档案读不到）。
	SkipMax *int `json:"skip_max,omitempty"`
	// RestateEveryN — 「每 N 步重述当前片」的 N；nil = **未标定**。
	RestateEveryN *int `json:"restate_every_n,omitempty"`
	// ReadErr — 读/解析失败的低基数分类（空 = 档案读到了）；读失败时两个读数必为 nil。
	ReadErr string `json:"read_err,omitempty"`
}

// Calibrated — 两处读数是否**都**有来源（只要有一处缺 ⇒ 未标定 ⇒ 调用方不得据它判阈值）。
func (c SliceCalib) Calibrated() bool { return c.SkipMax != nil && c.RestateEveryN != nil }

// SkipMaxText — 阈值的可读形态：数字，或 `UncalibratedText`（**未标定**）。
func (c SliceCalib) SkipMaxText() string { return intTextOrUncalibrated(c.SkipMax) }

// RestateEveryNText — N 的可读形态：数字，或 `UncalibratedText`（**未标定**）。
func (c SliceCalib) RestateEveryNText() string { return intTextOrUncalibrated(c.RestateEveryN) }

// intTextOrUncalibrated — 读数的文本化（**唯一实现点**：nil ⇒ 「未标定」，绝不写 0 或空串冒充）。
func intTextOrUncalibrated(v *int) string {
	if v == nil {
		return UncalibratedText
	}
	return strconv.Itoa(*v)
}

// LoadSliceCalibFrom — 从指定路径读两处读数（**IO 入口，不是纯函数**；聚合函数本身仍是纯的）。
//
// 纪律：任何失败都**不 panic、不阻断**，只把失败分类写进 ReadErr 并把两个读数留成 nil
// （= 「未标定」）—— 观测/统计路径**不许**因为档案读不到就改变判定。
func LoadSliceCalibFrom(path string) SliceCalib {
	c := SliceCalib{Path: path}
	if strings.TrimSpace(path) == "" {
		c.ReadErr = calibReadErrEmpty
		return c
	}
	data, err := os.ReadFile(path)
	if err != nil {
		c.ReadErr = calibReadErrRead
		if os.IsNotExist(err) {
			c.ReadErr = calibReadErrNotExist
		}
		return c
	}
	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		c.ReadErr = calibReadErrDecode
		return c
	}
	c.SkipMax = intKey(raw, CalibKeySliceSkipMax)
	c.RestateEveryN = intKey(raw, CalibKeyRestateEveryNSteps)
	return c
}

// LoadSliceCalib — 从**默认**档案路径读两处读数（生产路径；测试请用 LoadSliceCalibFrom(t.TempDir()…)）。
func LoadSliceCalib() SliceCalib { return LoadSliceCalibFrom(DefaultSliceCalibPath()) }

// intKey — 取一个「非负整数」键值；**认不出 ⇒ nil**（不猜：不截断、不四舍五入、不取默认值）。
//
// 接受 int / int64 / 整数值的 float64 / 十进制整数串；负数与非整数一律 nil。
// 值 0 **照收**（0 是否合法是作者的事 —— 本件只读档案，不替作者裁定；见文件头）。
func intKey(raw map[string]any, key string) *int {
	v, ok := raw[key]
	if !ok {
		return nil
	}
	switch t := v.(type) {
	case int:
		n := t
		if n < 0 {
			return nil
		}
		return &n
	case int64:
		if t < 0 {
			return nil
		}
		n := int(t)
		return &n
	case float64:
		if t < 0 || t != float64(int64(t)) { // 有小数 ⇒ 认不出（不截断）
			return nil
		}
		n := int(t)
		return &n
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(t))
		if err != nil || n < 0 {
			return nil
		}
		return &n
	}
	return nil
}

// ── 聚合（纯函数）────────────────────────────────────────────────────────

// SliceSkipCount — 一片的越片动作数（聚合产物，不是判定）。
type SliceSkipCount struct {
	// SliceID — 片 id（逐字取自事件；空 = 该事件没带 slice_id，属观测缺陷 —— 见 SkipsWithoutSliceID）。
	SliceID string `json:"slice_id"`
	// Skips — 该片的越片动作数。
	Skips int `json:"skips"`
}

// SkipStats — 越片统计（§4.8-2 的「越片动作数」+ 两处标定读数）。
type SkipStats struct {
	// SkipActions — **越片动作数**：只数 `slice_skipped_by_executor` 事件（§6.1 闭集第三名）。
	SkipActions int `json:"skip_actions"`
	// BySlice — 按片拆开的越片动作数（`SliceID` 升序，稳定可复现；空 id 排最前）。
	BySlice []SliceSkipCount `json:"by_slice,omitempty"`
	// SkipsWithoutSliceID — 越片事件里**缺 slice_id** 的条数（观测缺陷，单列不静默）。
	SkipsWithoutSliceID int `json:"skips_without_slice_id,omitempty"`
	// OtherEvents — 输入里**不计入**越片动作数的其它事件条数（显式写出来 —— 免得「混入的没数」
	// 被误读成「漏数」）。
	OtherEvents int `json:"other_events,omitempty"`
	// Calib — 两处标定读数（缺键 ⇒ 「未标定」，**绝不猜**）。
	Calib SliceCalib `json:"calib"`
	// BreachesSkipMax — 越片动作数是否**超过**阈值。**nil = 判不了**（阈值未标定 ⇒ 不猜 ✗）。
	BreachesSkipMax *bool `json:"breaches_skip_max,omitempty"`
}

// SkipMaxText — 阈值的可读形态（数字 / 「未标定」）。
func (s SkipStats) SkipMaxText() string { return s.Calib.SkipMaxText() }

// RestateEveryNText — 「每 N 步重述当前片」的 N（数字 / 「未标定」）。
func (s SkipStats) RestateEveryNText() string { return s.Calib.RestateEveryNText() }

// AggregateSkips — **纯函数**：从片事件序列聚合出越片动作数（+ 两处标定读数的可读形态）。
//
// 口径（写死在这里，由用例逐条钉住）：
//
//	① **只认** `EventSliceSkippedByExecutor`；其它事件（含 `slice_created` / `slice_rejected` /
//	   `slice_escalated`，以及闭集外的名字 —— 它们本该落不了盘）一律只计进 `OtherEvents`，
//	   **绝不**计进 `SkipActions`；
//	② 阈值判定只在**已标定**时给：阈值 nil ⇒ `BreachesSkipMax = nil`（「判不了」，不是 false ——
//	   把未标定读成「没超阈值」等于把未标定伪装成合格 ✗）；
//	③ 输出**确定性**：`BySlice` 按 `SliceID` 升序（同输入必得同输出，可复现可断言）；
//	④ 不读时间、不读 IO、不改入参（`events` 只读；`calib` 按值传）。
//
// **不生成事件、不判相位、不推断触发条件** —— 只数已经落盘的痕（见文件头）。
func AggregateSkips(events []Event, calib SliceCalib) SkipStats {
	st := SkipStats{Calib: calib}
	perSlice := map[string]int{}
	for _, ev := range events {
		if ev.Event != EventSliceSkippedByExecutor {
			st.OtherEvents++
			continue
		}
		st.SkipActions++
		if strings.TrimSpace(ev.SliceID) == "" {
			st.SkipsWithoutSliceID++
		}
		perSlice[ev.SliceID]++
	}
	if len(perSlice) > 0 {
		ids := make([]string, 0, len(perSlice))
		for id := range perSlice {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		st.BySlice = make([]SliceSkipCount, 0, len(ids))
		for _, id := range ids {
			st.BySlice = append(st.BySlice, SliceSkipCount{SliceID: id, Skips: perSlice[id]})
		}
	}
	if calib.SkipMax != nil {
		breached := st.SkipActions > *calib.SkipMax
		st.BreachesSkipMax = &breached
	}
	return st
}
