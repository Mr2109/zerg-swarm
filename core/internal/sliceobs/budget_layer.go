// budget_layer.go — B 项 **B10（其一）**：**预算项的定层标注**（设计稿 v2.1 §4.6-10 + §6.3）。
//
// ── 逐字来源（本件现读稿面，不凭记忆）────────────────────────────────────
//
//	docs/01-设计/设计-协作骨架-v2.1.md 第 241 行（§4.6 十条铁律之 10）：
//
//	  「10. **预算来源可追溯**（出处 bmas `test_classic_spec_compiler.py`）：全部预算项标注**定层**
//	    （档案／声明／默认），与既有 ctx 三态同规。」
//
//	同一文件第 380 行（§6.3 成本与预算可追溯）：
//
//	  「成本表记「显存 + **调用次数 / token / 墙钟**」；全部预算项标注**定层**（档案／声明／默认），
//	    与既有 ctx 三态同规；预算数值来自标定档案（**标定方式见《设计-回执门与结果对齐 v1.2.2》**；
//	    **档案路径与字段清单待 C 稿给出 —— 登记为清单 U5，不静默**），闸门只读档案。」
//
//	docs/01-设计/设计-任务模块骨架-v1.0.md 第 364 行（§3.2-R16）：
//
//	  「| **R16** | 观测面补三个字段级事实：**pending 原因码**、**熔断动作**（拒/排/杀/升级）、
//	    **预算定层的「首次定义层」** | 【事实】Slurm 明确限额在层级中「**首次定义处生效**」可当口径先例
//	    （L4-F17）⇒ 避免「上层写了但下层覆盖了」的静默 |」
//
// ── 有出处 ⇒ 做（三件）；无出处 ⇒ **不做** ✗（登记进回报的待定项）────────────
//
//	有出处：
//	  ① 定层**闭集逐字三名**：`档案` / `声明` / `默认`（§4.6-10 与 §6.3 两处同字；见 `DefinitionLayers`）；
//	  ② 取值顺序「**档案 ⇒ 声明 ⇒ 默认**」= **既有 ctx 三态的实现口径**（稿面两处明写「与既有 ctx 三态
//	     同规」）；既有实现现读：
//	       core/internal/gateway/gateway.go:778 逐字注释「来源三态：档案(事实) ⇒ 声明(意图) ⇒ 默认（卵未声明）」
//	       core/internal/gateway/gateway.go:785 逐字注释「取值顺序：**档案（事实）⇒ 配置声明（意图）⇒ 保守默认**」
//	       core/internal/gateway/gateway.go:790 标 `档案(事实)`；gateway/outbudget.go:89 `defaultCtxWindow` 是「默认」那档
//	     （本件只取**顺序**与**三档**，不搬它的中文括注字面 —— 定层的三个值是稿面给的字面：档案/声明/默认）；
//	  ③ **闸门只读档案**：§6.3 逐字「闸门只读档案」；既有档案
//	     `~/.zerg/egg-profiles/task-budget-calib.yaml` 抬头自述「§8.4：实测得出，闸门只读档案；
//	     无档案不许当已标定」；标定脚本 `scripts/calib-task-budget.py:6` 逐字「闸门日后只读它」。
//	     ⇒ 本件**只有读入口**，**没有任何写档案的路径**（无 WriteFile / 无 MkdirAll / 无 O_TRUNC）。
//
//	无出处 ⇒ 不做（只登记，绝不替 Mr2109 定）：
//	  · **档案的字段清单**：稿面明写「**档案路径与字段清单待 C 稿给出 —— 登记为清单 U5，不静默**」（§6.3）
//	    ⇒ 本件**不发明键名**，只认**现档里真实存在的四个键**（现读 `~/.zerg/egg-profiles/task-budget-calib.yaml`，
//	    见 `BudgetKeyStepsMax` 等四条常量）⇒ 稿面还没给清单时，多出来的预算项**认不出就是未标定**；
//	  · **递归深度条目 `depth` 与它的理由字段**：B10 清单点了「档案里的 `depth` 条目与理由字段」，
//	    但稿面**没给键名、也没给理由字段名** ⇒ 本件**不造键名** ✗（只登记，见回报「待定口径」）；
//	  · **「首次定义层」这个字段名**：R16 只说观测面要补这个**事实**，没给字段名 ⇒ 本件用 B10 验收判据
//	    里的字面 `definition_layer`，并在回报里登记这是**登记口径**（不是稿面字面）。
//
// ── 纪律（写死在代码里，由用例逐条钉住）──────────────────────────────────
//
//	① 闭集外的层名一律**认不出**（`ParseDefinitionLayer` ⇒ ok=false）—— 不猜、不归一化（大小写/空格/
//	   全角一律不认）；
//	② 缺档 / 缺键 / 值不合法（负数、非数、NaN/Inf）⇒ **显式「未标定」**（`UncalibratedText`，
//	   沿用 B 项⑥ 的同一字面）—— **不补 0、不取默认值、不截断、不四舍五入**；
//	③ **只读**：档案不存在时**不创建**（读不到就是读不到）；
//	④ 任何读失败**不改判定**（调用方拿到的是「未标定」，不是假的「已标定」）。
package sliceobs

import (
	"math"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// ── 定层闭集（§4.6-10 / §6.3 逐字三名）─────────────────────────────────────

// DefinitionLayer — 一个预算项的**定层**（它的数值第一次是在哪一层被定出来的）。
//
// 闭集**只有三个值**，逐字取自 §4.6-10 与 §6.3：「（档案／声明／默认）」。R16 的口径是
// 「**首次定义层**」（Slurm：限额在层级中「首次定义处生效」）⇒ 取值顺序也就是层的高低序。
type DefinitionLayer string

const (
	// LayerArchive — 档案：实测标定档案给的（**事实**）。
	LayerArchive DefinitionLayer = "档案"
	// LayerDeclared — 声明：调用方/配置声明的（**意图**）。
	LayerDeclared DefinitionLayer = "声明"
	// LayerDefault — 默认：前两层都没有时的内置保守值（**默认**）。
	LayerDefault DefinitionLayer = "默认"
)

// DefinitionLayers — 定层闭集本身（升序 = 取值顺序：档案 ⇒ 声明 ⇒ 默认）。
//
// 返回的是**副本**：调用方改它不会改到闭集。
func DefinitionLayers() []DefinitionLayer {
	return []DefinitionLayer{LayerArchive, LayerDeclared, LayerDefault}
}

// Valid — 是否落在闭集内（闭集外的值一律不认）。
func (l DefinitionLayer) Valid() bool {
	switch l {
	case LayerArchive, LayerDeclared, LayerDefault:
		return true
	}
	return false
}

// ParseDefinitionLayer — 把外部字符串解析成定层；**认不出 ⇒ ok=false**（不猜、不归一化）。
//
// 「不归一化」是刻意的：`" 档案"`（前导空格）、`"档案 "`（尾随空格）、`"ARCHIVE"`、`"声明（意图）"`
// 全部 ok=false —— 认不出就是认不出，宁可让调用方显式写对，也不悄悄改写成三层之一。
func ParseDefinitionLayer(s string) (DefinitionLayer, bool) {
	l := DefinitionLayer(s)
	if !l.Valid() {
		return "", false
	}
	return l, true
}

// ── 预算项（键名 = 现档里真实存在的键，不是本件发明的）──────────────────────
//
// 现读 `~/.zerg/egg-profiles/task-budget-calib.yaml` 的四个数值键（抬头 + model/calib_runs/
// measured_at 三类元数据不计入预算项）：
//
//	steps_max: 3 · tool_calls_max: 2 · wall_s_max: 43.6 · assistant_tokens_est_max: 118
const (
	// BudgetKeyStepsMax — 步数上限。
	BudgetKeyStepsMax = "steps_max"
	// BudgetKeyToolCallsMax — 工具调用次数上限。
	BudgetKeyToolCallsMax = "tool_calls_max"
	// BudgetKeyWallSMax — 墙钟上限（秒）。
	BudgetKeyWallSMax = "wall_s_max"
	// BudgetKeyAssistantTokensMax — 助手 token 上限（估算）。
	BudgetKeyAssistantTokensMax = "assistant_tokens_est_max"
)

// ArchiveBudgetKeys — 现档里真实存在的预算键（顺序固定 ⇒ 输出可复现）。
func ArchiveBudgetKeys() []string {
	return []string{
		BudgetKeyStepsMax,
		BudgetKeyToolCallsMax,
		BudgetKeyWallSMax,
		BudgetKeyAssistantTokensMax,
	}
}

// ── 定层取值 ──────────────────────────────────────────────────────────────

// BudgetValue — 一个预算项的**定层取值**。
//
//	Value == nil || Layer == ""  ⇔  **未标定**（没有任何一层给过值）——
//	两种读法都是「未标定」：`Text()` 与 `LayerText()` 各自兜底成 `UncalibratedText`。
type BudgetValue struct {
	// Key — 预算项键名（档案键 / 调用方给的键）。
	Key string `json:"key"`
	// Value — 数值；nil = **未标定**（不补 0）。
	Value *float64 `json:"value,omitempty"`
	// Layer — 定层（闭集三名之一）；空 = **未标定**。
	Layer DefinitionLayer `json:"definition_layer,omitempty"`
}

// Text — 数值的可读形态：数字，或 `UncalibratedText`（**未标定**）。
func (b BudgetValue) Text() string {
	if b.Value == nil {
		return UncalibratedText
	}
	return strconv.FormatFloat(*b.Value, 'f', -1, 64)
}

// LayerText — 定层名的可读形态：`档案` / `声明` / `默认`，或 `UncalibratedText`（**未标定**）。
func (b BudgetValue) LayerText() string {
	if !b.Layer.Valid() {
		return UncalibratedText
	}
	return string(b.Layer)
}

// Calibrated — 这一项是否**有来源**（值与层**都**在才算）。
func (b BudgetValue) Calibrated() bool { return b.Value != nil && b.Layer.Valid() }

// ResolveBudgetValue — **首次定义层**取值（R16 口径：首次定义处生效）。
//
// 取值顺序（= 既有 ctx 三态的实现口径，见文件头 ②）：
//
//	① 档案（事实）：`archive[key]` 读得出且值合法 ⇒ `LayerArchive`
//	② 声明（意图）：调用方给了 `declared` ⇒ `LayerDeclared`
//	③ 默认：调用方给了 `def` ⇒ `LayerDefault`
//	④ 三处都没有 ⇒ `Value=nil` / `Layer=""` ⇒ **未标定**（不猜）
//
// 非法值（负数 / 非数 / NaN / Inf）与**没给**同等对待（`nil`）—— 与同包 `intKey` 的口径一致。
func ResolveBudgetValue(key string, archive map[string]any, declared, def *float64) BudgetValue {
	if v := floatKey(archive, key); v != nil {
		return BudgetValue{Key: key, Value: v, Layer: LayerArchive}
	}
	if declared != nil {
		if v := nonNegative(declared); v != nil {
			return BudgetValue{Key: key, Value: v, Layer: LayerDeclared}
		}
	}
	if def != nil {
		if v := nonNegative(def); v != nil {
			return BudgetValue{Key: key, Value: v, Layer: LayerDefault}
		}
	}
	return BudgetValue{Key: key}
}

// nonNegative — 认一个「非负有限数」；认不出 ⇒ nil（不猜）。
func nonNegative(v *float64) *float64 {
	if v == nil {
		return nil
	}
	if *v < 0 || math.IsNaN(*v) || math.IsInf(*v, 0) {
		return nil
	}
	n := *v
	return &n
}

// ── 标定档案的**只读**读数 ─────────────────────────────────────────────────

// BudgetCalib — **闸门只读**的标定档案读数：现档每个预算项一行（值 + 定层）。
type BudgetCalib struct {
	// Path — 读的是哪个档案（可追溯；空 = 没给路径）。
	Path string `json:"path"`
	// Items — 逐项读数（顺序 = `ArchiveBudgetKeys()`；缺键项 ⇒ 值为 nil / 层为空 = **未标定**）。
	Items []BudgetValue `json:"items,omitempty"`
	// ReadErr — 读/解析失败的低基数分类（空 = 档案读到了）；读失败时**每一项**都是「未标定」。
	ReadErr string `json:"read_err,omitempty"`
}

// Calibrated — **每一项**都有来源才算整档已标定（缺一 ⇒ 未标定 ⇒ 调用方不得据它判阈值）。
func (c BudgetCalib) Calibrated() bool {
	if c.ReadErr != "" || len(c.Items) == 0 {
		return false
	}
	for _, it := range c.Items {
		if !it.Calibrated() {
			return false
		}
	}
	return true
}

// Item — 按键取一项；没有这个键（不在本档的项集里）⇒ ok=false（不凭空造项）。
func (c BudgetCalib) Item(key string) (BudgetValue, bool) {
	for _, it := range c.Items {
		if it.Key == key {
			return it, true
		}
	}
	return BudgetValue{}, false
}

// ItemText — 按键取可读值（取不到 ⇒ `UncalibratedText`）。
func (c BudgetCalib) ItemText(key string) string {
	if it, ok := c.Item(key); ok {
		return it.Text()
	}
	return UncalibratedText
}

// ItemLayerText — 按键取定层名的可读形态（取不到 ⇒ `UncalibratedText`）。
func (c BudgetCalib) ItemLayerText(key string) string {
	if it, ok := c.Item(key); ok {
		return it.LayerText()
	}
	return UncalibratedText
}

// LoadBudgetCalibFrom — 从**指定路径**只读一个标定档案（IO 入口，不是纯函数）。
//
// 纪律：不 panic、不阻断、**不创建文件**；任何失败只把低基数分类写进 `ReadErr`，
// 并让每一项都停在「未标定」（= 读数不给来源，**绝不猜**）。
func LoadBudgetCalibFrom(path string) BudgetCalib {
	c := BudgetCalib{Path: path}
	keys := ArchiveBudgetKeys()
	if strings.TrimSpace(path) == "" {
		c.ReadErr = calibReadErrEmpty
		c.Items = uncalibratedItems(keys)
		return c
	}
	data, err := os.ReadFile(path)
	if err != nil {
		c.ReadErr = calibReadErrRead
		if os.IsNotExist(err) {
			c.ReadErr = calibReadErrNotExist
		}
		c.Items = uncalibratedItems(keys)
		return c
	}
	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		c.ReadErr = calibReadErrDecode
		c.Items = uncalibratedItems(keys)
		return c
	}
	c.Items = make([]BudgetValue, 0, len(keys))
	for _, k := range keys {
		// 闸门只读档案 ⇒ 本入口不接「声明 / 默认」两档（那是调用方的事，见 ResolveBudgetValue）。
		c.Items = append(c.Items, ResolveBudgetValue(k, raw, nil, nil))
	}
	return c
}

// LoadBudgetCalib — 从**默认**档案路径只读（生产路径 = 任务级预算标定档案）。
//
// 路径复用 B 项⑥ 的同一个解析点（`DefaultSliceCalibPath`），**不另造第二个真源**；
// 测试请用 `LoadBudgetCalibFrom(t.TempDir()…)`。
func LoadBudgetCalib() BudgetCalib { return LoadBudgetCalibFrom(DefaultSliceCalibPath()) }

// uncalibratedItems — 档案读不到时的一整组「未标定」项（键仍是四个 —— 键**位**在、来源不在）。
func uncalibratedItems(keys []string) []BudgetValue {
	out := make([]BudgetValue, 0, len(keys))
	for _, k := range keys {
		out = append(out, BudgetValue{Key: k})
	}
	return out
}

// floatKey — 取一个「非负有限数」键值；**认不出 ⇒ nil**（不猜：不截断、不四舍五入、不取默认值）。
//
// 接受 int / int64 / 整数值或小数值的 float64 / 十进制数字串；负数、NaN、Inf、非数串一律 nil。
// 值 0 **照收**（0 是否合法是作者的事 —— 本件只读档案，不替作者裁定；与同包 `intKey` 同规）。
func floatKey(raw map[string]any, key string) *float64 {
	v, ok := raw[key]
	if !ok {
		return nil
	}
	switch t := v.(type) {
	case int:
		if t < 0 {
			return nil
		}
		f := float64(t)
		return &f
	case int64:
		if t < 0 {
			return nil
		}
		f := float64(t)
		return &f
	case float64:
		if t < 0 || math.IsNaN(t) || math.IsInf(t, 0) {
			return nil
		}
		f := t
		return &f
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(t), 64)
		if err != nil || f < 0 || math.IsNaN(f) || math.IsInf(f, 0) {
			return nil
		}
		return &f
	}
	return nil
}
