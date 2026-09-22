// family_impact_budget.go —— `B4` 分层预算与降级（`§4.4` + `§7.4` · 任务单-影响面实施-20260922 §三 `B4` 八字段）。
//
// 依据（逐字）：`设计-变更影响面-v1.6` §4.4（分层预算表：默认档只吃毫秒层 + 编译器层 · 贵层按需 ·
// **到点即停，不许硬等** · 超时即降级为「编译器层 + 词法层」并**明说哪几层没跑** · 缓存键三件）·
// §7.4（时效三件 + `meta.layers_not_run[]`：**没跑的层逐条点名**，含 `runtime`（运行期面）**恒在**）·
// 任务单 §三 `B4` 判据①–④ 与四条红线。
//
// 本件要证的四格（判据面对拍在 `cli_impact_budget_test.go`）：
//
//	① **降级必须留痕**：`meta.layers_not_run[]` 里逐条点名（**含 `runtime` 恒在**）；
//	② **超时即降**到「编译器层 + 词法层」，`warnings[]` 里写明**哪几层没跑**（宁少报不猜报）；
//	③ **缺档位的耗时不许进预算裁决**（一段耗时进裁决 ⇔ 口径 + `head_sha` + 缓存态 三件齐且
//	   缓存态 ≠ 未测；① 编译器层的缓存态恒「未测」⇒ 恒不进裁决）；
//	④ **`N` 的绝对值仍待拍**（`R32`）⇒ 本任务**只落两式**、不编定值 —— 两式与实测输入全在契约件
//	   `core/internal/contract/impact-budget.json`（登记表 `S-j`），**实现里 0 个阈值数**。
//
// 红线（逐条 · 不许做什么）：**不许硬等** ✗（到点即停：起下一层之前判，层内不另设中断机制 ⇒
// 停点 = **层边界**，照实写）· **不许悄悄少给几层** ✗（降级掉的层逐条进 `meta.layers_not_run[]`
// 与 `warnings[]`，并在层表里留一行 `状态=未跑（超预算降级）`）· **不许把「1 倍最贵层」当上限** ✗
// （v1.4 已证伪：1 倍最贵层**包不住**一次全层现算 ⇒ 两式都由契约件的乘数算，代码不选任何一个乘数）·
// **不许改既有门禁与批准件判据** ✗ · **不许把预算面接成模型输入** ✗。
//
// 出口纪律（九批一贯）：**不改 `emitEnvelope*`** ✗（六键包封 / 卡片四字段 / 人面三行骨架一个字不动）
// ⇒ `meta.layers_not_run[]` 与 `warnings[]` 只能落在**既有位置**（stderr 预算块，同一份取值逐字同形），
// 包封里 `meta` 只写 `count/source/changed`（+`node`/`idempotency_key`）这一格**照实记 CLI 缺口**。
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// impactBudgetContractRel —— 预算契约件（真源 · **只读** · 目录名不另写第二份）。
const impactBudgetContractRel = "core/internal/contract/impact-budget.json"

// impactBudgetSchema —— 契约件形状号（照 `A5` 的 `impactCacheSchema` 同款：不认的形状号 ⇒ 不裁）。
const impactBudgetSchema = "zerg/impact-budget/1"

// impactBudgetDegradedStatus —— 降级掉的层在层表里的状态（**留痕的第二个落点**）。
const impactBudgetDegradedStatus = "未跑（超预算降级）"

// impactBudgetUntested —— 「缓存态」这一档里的**未测**标记（判据③：未测 ⇒ 那一段耗时**不进裁决**）。
// 取值域由契约件 `cap.耗时准入.规则` 写死（「缓存态 ≠ 未测」）；这里只是那个标记本身，不是阈值。
const impactBudgetUntested = "未测"

// impactBudgetRuntimeEntry —— `meta.layers_not_run[]` 里**恒在**的那一条（§7.4 逐字：运行期面）。
const impactBudgetRuntimeEntry = "runtime（运行期面 · 恒在）：反射 / 配置串 / HTTP 路由 / 序列化 —— 静态图看不见 ⇒ 本跑未查"

// impactBudgetSkeleton —— 降级掉的层那一行的**骨架**（层序 / 层名 / 粒度四档之一）。
//
// 为什么要有它：降级掉的层**没跑**，取不到它自己拼的那一份名与粒度；而「降级前后**输出形状不变**」
// 要求那一行照打（只改内容与标注 ⇒ 状态 / 读数 / 留痕），不许少一行。
// ★ 与各层函数里写的那一份**必须逐字同** —— 对拍判据在 `cli_impact_budget_test.go`（拿真跑出来的
// 层名逐字比这份表；漂了就红）。
var impactBudgetSkeleton = []struct{ Seq, Name, Grane string }{
	{"①", "编译器层", "包级"},
	{"②", "符号层", "符号级"},
	{"③", "契约层", "契约级"},
	{"④", "词法 + 形近层", "文件级 + 名字级"},
	{"⑤", "语义层（按需）", "件级"},
	{"⑥", "公开面", "件级（件不是行）"},
}

// impactBudgetSkeletonOf 按层序取骨架（找不到 ⇒ 只给层序：名留空但行照打）。
func impactBudgetSkeletonOf(seq string) (name, grane string) {
	for _, s := range impactBudgetSkeleton {
		if s.Seq == seq {
			return s.Name, s.Grane
		}
	}
	return "", ""
}

// ---- 契约件（**读这一份；源码里 0 个阈值数**）------------------------------------------------

// impactBudgetFormula —— 一式（乘数 + 输入名 + 契约件里记的算出值）。
type impactBudgetFormula struct {
	ID    string  `json:"id"`
	Text  string  `json:"式"`
	Mul   float64 `json:"乘数"`
	Input string  `json:"输入"`
	Fixed float64 `json:"算出值秒"`
}

// impactBudgetFixed —— `cap.上限定值` 那一格（**`R32` 已拍**：Mr2109 2026-09-22 定「取大」）。
//
// 为什么另立一格而不是改 `cap.生效式`：那一格是**旧值的原文**（「★ 本格是暂定取法」）——
// 拍定之后**只许并留、不许改写**（数字纪律：改前原文要能逐字找回）⇒ 定值住本格，
// 旧文仍住 `cap.生效式` 与 `上限定值.旧值并留`（两处逐字同）。
type impactBudgetFixed struct {
	Value   float64 `json:"值秒"`
	Pick    string  `json:"取法"`
	Origin  string  `json:"出处"`
	Caliber string  `json:"口径"`
	OldText string  `json:"旧值并留"`
	Read    string  `json:"读法"`
}

// impactBudgetMeasured —— 一条**实测输入**（判据③ 的三件：口径 + `head_sha` + 缓存态）。
type impactBudgetMeasured struct {
	Value   float64 `json:"取值秒"`
	Range   string  `json:"区间秒"`
	Caliber string  `json:"口径"`
	Head    string  `json:"head_sha"`
	Cache   string  `json:"缓存态"`
}

// impactBudgetContract —— 预算契约件（字段名与契约件的键一一对应；多一格都不要）。
type impactBudgetContract struct {
	Schema string `json:"schema"`
	ID     string `json:"id"`
	Tiers  struct {
		DegradeTier struct {
			SeqList []string `json:"层名单"`
			Caliber string   `json:"口径"`
		} `json:"降级档"`
	} `json:"档位"`
	Cap struct {
		Formulas []impactBudgetFormula           `json:"两式"`
		Measured map[string]impactBudgetMeasured `json:"实测输入"`
		Fixed    impactBudgetFixed               `json:"上限定值"`
		Pick     string                          `json:"生效式取法"`
		PickSet  []string                        `json:"生效式取法闭集"`
		PickRule string                          `json:"生效式"`
		PickRead string                          `json:"取法读法"`
		Admit    struct {
			Rule    string `json:"规则"`
			NoAdmit []struct {
				Seq    string `json:"层"`
				Reason string `json:"理由"`
			} `json:"不进裁决的层"`
			Timer      string `json:"计时口"`
			TwoCaliber string `json:"两个口径"`
		} `json:"耗时准入"`
	} `json:"cap"`
	Degrade struct {
		Trigger    string   `json:"触发"`
		After      string   `json:"降级后"`
		KeepRun    string   `json:"已跑的不撤回"`
		NotRun     string   `json:"没跑的层口径"`
		NotRunSet  []string `json:"没跑的状态闭集"`
		NotThisSet []string `json:"不进没跑名单的状态"`
		Warnings   string   `json:"warnings"`
		Shape      string   `json:"形状不变"`
		Answer     string   `json:"不改答案"`
		ExitCode   string   `json:"不改退码"`
		NoActions  []string `json:"不许接的动作"`
		TraceShape string   `json:"留痕形状"`
	} `json:"degrade"`
}

// impactBudgetContractOf 现读预算契约件（**只读真源** · 读不到 / 形状号不认 / 键不成形 ⇒ 明写原因，
// 由调用方「不裁」—— 不猜取值、不自选一个默认取法）。
func impactBudgetContractOf(root string) (impactBudgetContract, string) {
	var c impactBudgetContract
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(impactBudgetContractRel)))
	if err != nil {
		return c, "契约件读不到（" + err.Error() + "）"
	}
	if err := json.Unmarshal(b, &c); err != nil {
		return c, "契约件解不开（" + err.Error() + "）"
	}
	if c.Schema != impactBudgetSchema {
		return c, fmt.Sprintf("形状号不认（`schema`=%q ≠ %q ⇒ 不裁、不猜）", c.Schema, impactBudgetSchema)
	}
	if len(c.Cap.Formulas) == 0 || len(c.Tiers.DegradeTier.SeqList) == 0 {
		return c, "键不成形（`cap.两式` / `档位.降级档.层名单` 缺一 ⇒ 不裁）"
	}
	return c, ""
}

// ---- 上限裁决（两式 → 生效式 → 本跑上限）-----------------------------------------------------

// impactBudgetFormulaPlan —— 一式的裁决视图（进 / 不进裁决都照实留着，各带原因）。
type impactBudgetFormulaPlan struct {
	ID    string
	Text  string
	Input string
	Value float64 // 算出值（乘数 × 输入取值）—— 只有进裁决的那几式才有值
	Fixed float64 // 契约件里记的算出值（对拍用）
	Admit bool
	Why   string
}

// impactBudgetPlan —— 本跑的上限裁决（**全部从契约件读**：定值 / 两式 / 乘数 / 实测输入 / 取法 / 降级档名单）。
type impactBudgetPlan struct {
	OK         bool   // 契约件读得到且形状认（**与 `Armed` 分开**：读到了也可能「不裁」）
	Armed      bool   // 本跑有没有在效的
	Why        string // 不裁的原因（读不到 / 定值非法且取法不在闭集 / 两式都不进裁决）
	Formulas   []impactBudgetFormulaPlan
	Pick       string  // 回落路径的取法（契约件原样：`min` / `max`）
	PickedID   string  // 生效的那一式 / 定值
	CapSec     float64 // 本跑上限（秒）
	Rule       string  // `cap.生效式` 那一格原样（人面照引 · **旧值并留**）
	Fixed      bool    // 本跑上限来自 `cap.上限定值`（**已定值** · `R32` 已拍）
	FixedPick  string  // `上限定值.取法` 原样
	FixedWhy   string  // 定值没被采用时的原因（回落 / 不裁 —— 照实写明，不静默换档）
	FixedOrig  string  // `上限定值.出处` 原样（逐字回引）
	FixedCal   string  // `上限定值.口径` 原样
	FixedOld   string  // `上限定值.旧值并留` 原样（改前原文逐字找回）
	FixedRead  string  // `上限定值.读法` 原样
	Allow      []string
	AllowWhy   string
	NoAdmit    map[string]string // 层 ⇒ 不进裁决的理由（契约件 `耗时准入.不进裁决的层`）
	StatusIn   map[string]bool   // 「没跑」的状态闭集（契约件 `degrade.没跑的状态闭集`）
	NotRun     string
	Shape      string
	Answer     string
	Exit       string
	Warnings   string
	NoAction   []string
	Timer      string
	TwoCaliber string
}

// impactBudgetPlanOf 把契约件折成裁决计划（**纯函数** —— 测试可以直接喂合成契约件，不碰真仓）。
func impactBudgetPlanOf(c impactBudgetContract) impactBudgetPlan {
	p := impactBudgetPlan{
		Pick:       c.Cap.Pick,
		Rule:       c.Cap.PickRule,
		FixedPick:  c.Cap.Fixed.Pick,
		FixedOrig:  c.Cap.Fixed.Origin,
		FixedCal:   c.Cap.Fixed.Caliber,
		FixedOld:   c.Cap.Fixed.OldText,
		FixedRead:  c.Cap.Fixed.Read,
		Allow:      append([]string{}, c.Tiers.DegradeTier.SeqList...),
		AllowWhy:   c.Tiers.DegradeTier.Caliber,
		NoAdmit:    map[string]string{},
		StatusIn:   map[string]bool{},
		NotRun:     c.Degrade.NotRun,
		Shape:      c.Degrade.Shape,
		Answer:     c.Degrade.Answer,
		Exit:       c.Degrade.ExitCode,
		Warnings:   c.Degrade.Warnings,
		NoAction:   append([]string{}, c.Degrade.NoActions...),
		Timer:      c.Cap.Admit.Timer,
		TwoCaliber: c.Cap.Admit.TwoCaliber,
	}
	for _, na := range c.Cap.Admit.NoAdmit {
		p.NoAdmit[na.Seq] = na.Reason
	}
	for _, s := range c.Degrade.NotRunSet {
		p.StatusIn[s] = true
	}
	// 逐式判「进不进裁决」：三件齐（口径 + head_sha + 缓存态）且 缓存态 ≠ 未测（判据③）。
	for _, f := range c.Cap.Formulas {
		fp := impactBudgetFormulaPlan{ID: f.ID, Text: f.Text, Input: f.Input, Fixed: f.Fixed}
		in, has := c.Cap.Measured[f.Input]
		switch {
		case strings.TrimSpace(f.Input) == "":
			fp.Why = "契约件没写输入名 ⇒ 这一式不进裁决"
		case !has:
			fp.Why = fmt.Sprintf("实测输入 `%s` 不在 `cap.实测输入` 里 ⇒ 这一式不进裁决（不当 0 看）", f.Input)
		case strings.TrimSpace(in.Caliber) == "" || strings.TrimSpace(in.Head) == "" || strings.TrimSpace(in.Cache) == "":
			fp.Why = fmt.Sprintf("实测输入 `%s` 的**三件不齐**（口径 / `head_sha` / 缓存态）⇒ 这一式不进裁决（判据③）", f.Input)
		case strings.Contains(in.Cache, impactBudgetUntested):
			fp.Why = fmt.Sprintf("实测输入 `%s` 的**缓存态=未测** ⇒ 这一式不进裁决（判据③：缺档位的耗时不许进预算裁决）", f.Input)
		case in.Value <= 0:
			fp.Why = fmt.Sprintf("实测输入 `%s` 的取值不是正数（%v）⇒ 这一式不进裁决", f.Input, in.Value)
		default:
			fp.Admit = true
			fp.Value = f.Mul * in.Value
		}
		p.Formulas = append(p.Formulas, fp)
	}
	admitted := []impactBudgetFormulaPlan{}
	for _, fp := range p.Formulas {
		if fp.Admit {
			admitted = append(admitted, fp)
		}
	}
	// **定值优先**（`R32` 已拍：Mr2109 2026-09-22 定「取大」）—— 读法由契约件
	// `上限定值.读法` 写死：`值秒` 是正数**且** `出处` 非空 ⇒ 生效上限 = 定值；定值里的
	// `取法` 若给了，必须落在 `生效式取法闭集` 里（不在 ⇒ 不裁：定值的取法漂了就是判不了，
	// 不许自选、也不许悄悄回落）。定值缺 / 值非正 / 出处空 ⇒ **回落**到 `cap.生效式取法` 逐式取。
	fx := c.Cap.Fixed
	fxPick := strings.TrimSpace(fx.Pick)
	fxBad := fxPick != "" && !containsStr(c.Cap.PickSet, fxPick)
	switch {
	case fx.Value > 0 && strings.TrimSpace(fx.Origin) != "" && !fxBad:
		p.OK, p.Armed, p.Fixed = true, true, true
		p.PickedID, p.CapSec = "定值", fx.Value
	case fx.Value > 0 && strings.TrimSpace(fx.Origin) != "" && fxBad:
		p.OK, p.Armed = true, false
		p.FixedWhy = fmt.Sprintf("`cap.上限定值.取法` = %q 不在闭集 %v 里 ⇒ **不裁**（不许自选一个默认取法，也不许悄悄回落）", fx.Pick, c.Cap.PickSet)
		p.Why = p.FixedWhy
	default:
		switch {
		case fx.Value != 0 || strings.TrimSpace(fx.Origin) != "":
			p.FixedWhy = "定值这一格**没被采用**（值不是正数 / 出处为空）⇒ 回落到 `cap.生效式取法` 逐式取（照实写明换档，不静默）"
		default:
			p.FixedWhy = "定值这一格**本跑缺失** ⇒ 回落到 `cap.生效式取法` 逐式取（照实写明换档，不静默）"
		}
		switch {
		case len(admitted) == 0:
			p.OK, p.Armed = true, false
			p.Why = "两式都不进裁决（判据③：缺档位的耗时不许进预算裁决）⇒ 本跑**不裁**（照实明写，不当 0 看）"
		case c.Cap.Pick != "min" && c.Cap.Pick != "max":
			p.OK, p.Armed = true, false
			p.Why = fmt.Sprintf("`cap.生效式取法` = %q 不在闭集 %v 里 ⇒ **不裁**（不许自选一个默认取法）", c.Cap.Pick, c.Cap.PickSet)
		default:
			p.OK, p.Armed = true, true
			best := admitted[0]
			for _, fp := range admitted[1:] {
				if (c.Cap.Pick == "max" && fp.Value > best.Value) || (c.Cap.Pick == "min" && fp.Value < best.Value) {
					best = fp
				}
			}
			p.PickedID, p.CapSec = best.ID, best.Value
		}
	}
	return p
}

// impactBudgetPlanFor 现读契约件 + 折成计划（读不到 ⇒ 计划带原因、`OK=false`）。
func impactBudgetPlanFor(root string) impactBudgetPlan {
	c, why := impactBudgetContractOf(root)
	if why != "" {
		return impactBudgetPlan{Why: why, NoAdmit: map[string]string{}, StatusIn: map[string]bool{}}
	}
	return impactBudgetPlanOf(c)
}

// ---- 本跑账（到点即停 · 降级留痕）-----------------------------------------------------------

// impactBudgetRun —— 本跑账：上限 + 已计时（只算**进裁决**的层）+ 降级触发 + 逐条留痕。
type impactBudgetRun struct {
	Plan          impactBudgetPlan
	Charged       float64       // 进裁决层耗时之和（秒）
	Wall          time.Duration // 本跑墙钟（**只报 · 不进裁决** —— 它含非层部分）
	Degraded      bool
	Trigger       string // 触发点（逐字：层序 + 已计时 + 上限 + 停点）
	ChargedLayers []string
	NoAdmitNote   []string // 进了「不进裁决」名单 / 没自计时的层（逐条点名，不当 0 看）
	NotRunNow     []string // 本跑**因降级**没跑的层（逐条）
}

// impactBudgetAdmitOf 这一层的耗时进不进裁决（契约件的 `耗时准入.不进裁决的层`）。
func (r *impactBudgetRun) impactBudgetAdmitOf(seq string) (bool, string) {
	if why, ok := r.Plan.NoAdmit[seq]; ok {
		return false, why
	}
	return true, ""
}

// charge 记一层：进裁决且自计时了才上账；否则逐条点名（**不当 0 看**）。
func (r *impactBudgetRun) charge(lay impactLayer) {
	if ok, why := r.impactBudgetAdmitOf(lay.Seq); !ok {
		r.NoAdmitNote = append(r.NoAdmitNote, fmt.Sprintf("%s%s：**不进裁决**（%s）", lay.Seq, dashIfEmpty(lay.Name), why))
		return
	}
	if lay.BudgetWall <= 0 {
		r.NoAdmitNote = append(r.NoAdmitNote, fmt.Sprintf("%s%s：本跑**未自计时**（层表照打 `耗时=—`）⇒ 对本跑账贡献 0（不补零、不估）", lay.Seq, dashIfEmpty(lay.Name)))
		return
	}
	r.Charged += lay.BudgetWall.Seconds()
	r.ChargedLayers = append(r.ChargedLayers, lay.Seq)
}

// beforeLayer —— **到点即停**的唯一判点（起这一层之前；判点 = 层边界，照实写）。
//
// 返回 (skip, why)：skip=true ⇒ 这一层**不跑**（降级掉），why 是要逐字进留痕的原因。
// 语义（§4.4 逐字）：`已计时 ≥ 上限` ⇒ 降级；降级之后**只有降级档名单（①④）里的层照跑**，
// 其余一律不跑并逐条点名 —— 这就是「超时即降级为『编译器层 + 词法层』」。
func (r *impactBudgetRun) beforeLayer(seq string) (bool, string) {
	name, _ := impactBudgetSkeletonOf(seq)
	if r.Plan.OK && r.Plan.Armed && !r.Degraded && r.Charged >= r.Plan.CapSec {
		r.Degraded = true
		r.Trigger = fmt.Sprintf("起 %s%s 之前：已计时 %.3fs ≥ 上限 %.3fs ⇒ **降级**（到点即停 · 停点 = 层边界）",
			seq, dashIfEmpty(name), r.Charged, r.Plan.CapSec)
	}
	if !r.Degraded {
		return false, ""
	}
	for _, s := range r.Plan.Allow {
		if s == seq {
			return false, "" // 降级档名单里的层**照跑**（§4.4：降到编译器层 + 词法层）
		}
	}
	r.NotRunNow = append(r.NotRunNow, fmt.Sprintf("%s %s（%s）", seq, dashIfEmpty(name), impactBudgetDegradedStatus))
	return true, fmt.Sprintf("**超预算降级**：本跑已计时 %.3fs ≥ 上限 %.3fs（到点即停）⇒ 这一层**没跑**"+
		"（不在降级档名单 %v 里）；`宁少报不猜报`：**没跑 ≠ 没影响**", r.Charged, r.Plan.CapSec, r.Plan.Allow)
}

// ---- 没跑的层（留痕名单 · 判据①②）----------------------------------------------------------

// impactBudgetNotRunOf 从**层表本身**算出「没跑的层」名单（同一份取值 ⇒ 两处不会漂）。
// 口径 = 契约件 `degrade.没跑的状态闭集`；跑了的面（取值 / 命中 / 未命中）**不进**这份名单。
func impactBudgetNotRunOf(layers []impactLayer, plan impactBudgetPlan) []string {
	out := []string{}
	for _, l := range layers {
		if len(plan.StatusIn) == 0 {
			break
		}
		if !plan.StatusIn[l.Status] {
			continue
		}
		out = append(out, fmt.Sprintf("%s %s（%s）：%s", l.Seq, dashIfEmpty(l.Name), l.Status, impactFirstLine(l.Detail)))
	}
	return out
}

// ---- 渲染（stderr 预算块 · 六键包封一个字不动）---------------------------------------------

// emitImpactBudgetBlock —— `B4` 的**预算与降级**那一块（stderr · 人读；`A2` 的层表一个字不动）。
//
// 打五件：① 上限两式（逐式标明进不进裁决 + 算出值与契约件记的值对拍）· ② 生效式与本跑上限 ·
// ③ `meta.layers_not_run[]`（`runtime` 恒在 + 没跑的层逐条）· ④ `warnings[]`（同一份取值）·
// ⑤ 本跑账与纪律（形状不变 / 不改答案 / 不改退码 / 不接四个动作 / CLI 缺口）。
func emitImpactBudgetBlock(w io.Writer, run impactBudgetRun, layers []impactLayer) {
	p := run.Plan
	fmt.Fprintf(w, "%s: `B4` 分层预算与降级（§4.4 默认档只吃毫秒层 + 编译器层 · 贵层按需 · **超时只降不猜**）\n", progName)
	if !p.OK {
		fmt.Fprintf(w, "  契约件 `%s`：%s ⇒ 本跑**不裁**（照实明写「不裁」，不当 0 看；也不自选一个默认取法）\n",
			impactBudgetContractRel, p.Why)
	} else {
		fmt.Fprintf(w, "  上限两式（契约件 `%s` 原样读 · `N` 的绝对值 `R32` **已定**（Mr2109 2026-09-22 定：取大）"+
			" ⇒ 两式**只作来路与回落**，生效上限看下一条「已定值」；实现里 0 个阈值数）：\n",
			impactBudgetContractRel)
		for _, f := range p.Formulas {
			if f.Admit {
				fmt.Fprintf(w, "    式%s %s = **%.3f s**（进裁决 · 换算 = 契约件乘数 × 实测输入；与契约件记的 `算出值秒` %.3f 对拍 = %s）\n",
					f.ID, f.Text, f.Value, f.Fixed, yesno(sameSeconds(f.Value, f.Fixed)))
			} else {
				fmt.Fprintf(w, "    式%s %s —— **不进裁决**（%s）\n", f.ID, f.Text, f.Why)
			}
		}
	}
	// ① 已定值（`R32` 已拍 · Mr2109 2026-09-22 定「取大」）：生效上限的第一来路。
	//   照引三件：出处（谁定的）· 口径（怎么来的）· **旧值并留**（改前原文一字未改）。
	if p.Fixed {
		fmt.Fprintf(w, "  已定值（契约件 `cap.上限定值` · 出处 **%s**）：**%.3f s** —— %s\n",
			p.FixedOrig, p.CapSec, p.FixedCal)
		fmt.Fprintf(w, "    读法（契约件原样）：%s\n", p.FixedRead)
		fmt.Fprintf(w, "    旧值并留（改前原文**一字未改** · 逐字回引）：%s\n", p.FixedOld)
	} else if p.OK {
		fmt.Fprintf(w, "  已定值：本跑**没采用**（%s）\n", p.FixedWhy)
	}
	if p.OK && p.Armed {
		if p.Fixed {
			fmt.Fprintf(w, "  生效式（回落路径 · 旧值并留）：`%s`（契约件 `cap.生效式取法`）—— 本跑上限由**已定值**给出（%.3f s）；定值缺 / 值非正 / 出处空时按本格逐式取\n",
				p.Pick, p.CapSec)
		} else {
			fmt.Fprintf(w, "  生效式：`%s`（契约件 `cap.生效式取法`）⇒ 本跑上限 = 式%s 的 **%.3f s**\n", p.Pick, p.PickedID, p.CapSec)
		}
	} else {
		fmt.Fprintf(w, "  生效式：**本跑不裁**（%s）\n", p.Why)
	}
	// ③ 判据①：`meta.layers_not_run[]` —— `runtime` **恒在** + 本跑没跑的层逐条点名。
	fmt.Fprintf(w, "  `meta.layers_not_run[]`（判据① 留痕 · 同形落点 = 本块）：\n")
	fmt.Fprintf(w, "    · %s\n", impactBudgetRuntimeEntry)
	notRun := impactBudgetNotRunOf(layers, p)
	for _, ln := range notRun {
		fmt.Fprintf(w, "    · %s\n", ln)
	}
	// ④ 判据②：`warnings[]` —— 缺的那几层逐条（同一份取值）。
	if len(notRun) == 0 {
		fmt.Fprintf(w, "  `warnings[]`（判据② · 同一份取值）：[] —— 本跑六层**都跑到了**（没跑的只有恒在的 `runtime`）\n")
	} else {
		fmt.Fprintf(w, "  `warnings[]`（判据② · 同一份取值 · 逐字写明**哪几层没跑**）：%s\n", strings.Join(notRun, " · "))
	}
	// ⑤ 本跑账。
	charged := "（本跑没有进裁决的层）"
	if len(run.ChargedLayers) > 0 {
		charged = fmt.Sprintf("%s（合计 %.3fs）", strings.Join(run.ChargedLayers, ""), run.Charged)
	}
	if p.Armed {
		fmt.Fprintf(w, "  本跑账：已计时层 = %s（进裁决 · 含本跑没取数的层：它们各花掉一次判档的闭包时间）；上限 = %.3fs ⇒ %s\n", charged, p.CapSec,
			ternary(run.Degraded, "**已降级**（降级档层名单 "+strings.Join(p.Allow, "")+" 照跑 · 其余逐条点名）", "**未降级**"))
	} else {
		fmt.Fprintf(w, "  本跑账：已计时层 = %s（进裁决 · 含本跑没取数的层）；**本跑不裁**（无上限在效）⇒ 不降级\n", charged)
	}
	fmt.Fprintf(w, "  停点：**层边界**（起下一层之前判 `已计时 ≥ 上限` —— 层是不可分割的取数单元，层内不另设中断机制 ⇒ 照实写）；本跑墙钟 %s（**只报 · 不进裁决**：它含非层部分）\n",
		run.Wall.Round(time.Millisecond).String())
	if run.Trigger != "" {
		fmt.Fprintf(w, "  触发：%s\n", run.Trigger)
	}
	for _, s := range run.NoAdmitNote {
		fmt.Fprintf(w, "  裁决外：%s\n", s)
	}
	if p.TwoCaliber != "" {
		fmt.Fprintf(w, "  计时口（契约件原样）：%s\n", p.Timer)
		fmt.Fprintf(w, "  两个口径（契约件原样）：%s\n", p.TwoCaliber)
	}
	// 纪律（照实打，不靠自觉）。
	fmt.Fprintf(w, "  纪律：降级前后**层表恒六行**（%s）· 降级**不改退码**（%s）· 缓存与降级**不改答案**（%s）\n",
		ternary(p.Shape != "", "形状只改内容与标注", "形状不变口径见契约件"), ternary(p.Exit != "", "§7.5 一字不动", "见契约件"),
		ternary(p.Answer != "", "都跑到的层逐字相同 · `M8`", "见契约件"))
	fmt.Fprintf(w, "  **不接**的动作（契约件 `degrade.不许接的动作` 原样）：%s\n", strings.Join(p.NoAction, " · "))
	fmt.Fprintf(w, "  ★ CLI 缺口（照实标 · 与 `A5`/`B3` 同一处）：§7.4 的 `meta.layers_not_run[]` 与 `warnings[]` 在**六键包封里没有落点**"+
		"（`emitEnvelopeWith` 把 `warnings` 恒写 `[]`、`meta` 只写 `count/source/changed`）⇒ 两份留痕落在**本块**（同一份取值，逐字同形）；"+
		"九批一贯红线「不改 `emitEnvelope*`」本批未解禁 ⇒ 包封那一格照实记缺口（不偷偷改包封）\n")
}

// sameSeconds 两枚秒值是不是同一枚（对拍用 · 容差只为浮点表示，不为凑绿）。
func sameSeconds(a, b float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < 1e-9
}

// impactBudgetNoRunNote —— 降级留下的一句人话（层表那一行与留痕共用同一份口径）。
func impactBudgetNoRunNote(run impactBudgetRun) string {
	if !run.Degraded {
		return ""
	}
	return fmt.Sprintf("（本跑已降级：降级档层名单 %v 照跑 · 其余逐条进 `meta.layers_not_run[]`)", run.Plan.Allow)
}

// impactBudgetSortedUniq 排序去重（留痕名单稳序 ⇒ 同一判据跑两遍逐字一致）。
func impactBudgetSortedUniq(in []string) []string {
	sort.Strings(in)
	out := in[:0]
	for i, s := range in {
		if i == 0 || s != in[i-1] {
			out = append(out, s)
		}
	}
	return out
}

func ternary(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}
