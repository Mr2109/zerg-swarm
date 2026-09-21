// family_impact_timeface.go —— 时间面（**预测侧** · `B2` · 任务单-影响面实施-20260922 §三 `B2` 八字段）。
//
// 本件只做一件事：把「波纹的**预测**」做成**口径写清 · 可复算 · 尺可标定**的一件。设计稿 §五
// 把波纹的时间面定成**两个时刻**（`预测` / `实际`）—— 本件落**前一个**：
//
//	· **预测侧（本件）**：`predict_red[]`（闭集）逐条带**估法口径**（这一条凭什么被估成会红 ·
//	  出处是哪一层 · 是**现算**、是**投影**、还是**没跑**）—— 卡片里 `red` 那一格给的是**预测**，
//	  那么「怎么估的」必须写在同一份输出里（§5.1：不写口径的预测，连值不值都判不了）。
//	· **实测侧（**不属本件** ✗）**：门禁真红之后的对拍（命中 / 漏报 / 虚报）+ `impact_actual`
//	  + 回填件 ⇒ 属 `B3`（§5.2 / §5.3 / §5.5 · `R57`）。本件**只报预测侧**，绝不代 `B3` 落任何一枚
//	  实测数（**不许抢** ✗）—— 这一点在输出里逐字写明。
//
// 尺（§5.1 的「现成槽」· **不新造度量**）：`scripts/gates/check-slice.py` 第 7 行逐字写着
// 「探针集跑**混淆矩阵**（§3.5 质量门槛：**假绿 = 0 · 假红 ≤ 10%**）」⇒ 虫族**已经有一把尺**：
//
//	· `--probe`    = 全量探针集跑一遍混淆矩阵（**现跑重测**：探针条数 / 真阳 / 真阴 / 假绿率 / 假红率）；
//	· `--selftest` = 探针**自己先自证**（九条用例 + **前置缺件闸正反两半**：正 ⇒ rc=0 · 反 ⇒ rc=2）
//	  ⇒ 这正是「**成对**正控/负控」那一格：**只跑半边不算成对**。
//
// 三条口径（照任务单/设计稿逐字，缺一条都不许出数）：
//
//	① **阈值不许自设** ✗：本件**只引用**现成槽那把尺的门槛（`check-slice.py` §3.5 的
//	  「假绿 = 0 · 假红 ≤ 10%」），并写明出处；波纹自己的命中率**一个阈值都不设**
//	  （`R30` · §十一 `U20`：跨系统通用阈值**没有一手出处** ⇒ 口径可定、阈值不可编）。
//	② **标定数字必须现跑重测** ✓：凡能现跑的（探针条数 / 四格 / 假绿率 / 假红率 / 自证九条）
//	  一律**当场跑**、带**耗时与时刻**；在册的旧值**并留**（带出处 + **标定时刻**）⇒ 新旧**并排**，
//	  一眼看得出漂没漂。**不许拍脑袋常数** ✗（每个数都要么带现跑读数、要么带一手出处）。
//	③ **探针不许小额度** ✗（**会截断 ⇒ 假象**）：`--probe` 一条截断开关都不传（条数由脚本自报）；
//	  且四格与自报条数**不齐 ⇒ 取不到**（**不齐不出数**）。**失败 ≠ 事实为零** ✗：脚本跑不起来 /
//	  退码非 0 / 输出解析不了 ⇒ 一律写「**取不到**」，**绝不许写成「假绿 0」**。
//
// 预算（§4.4 · 与 `B1` **同一约束**）：本件的探针要起 `python3`（冷起 + 全量探针集）⇒ 归**按需档**；
// 默认档（`zerg dev edit` 干跑一律走的那一档）**一次都不跑**，照实写「未机检 + 怎么拉」（宁少报不猜报）。
// 档位开关沿用既有全局布尔 `--all`（**不新造旗标名** ✗）。
//
// 零副作用：两个探针入口都**只读**（`--probe` 判探针件、`--selftest` 在 `/tmp` 自建临时仓自证）
// ⇒ 不写缓存 / 不落审计 / 不改件 / 不改门禁脚本 ✗；本件也不写任何件（**只出文本**）。
//
// 红线（本件逐条）：**不设阈值** ✗ · **不进门禁** ✗ · **不自动改「会红」的算法** ✗ ·
// **不拿回填结果回改已发出的卡片** ✗ · **不抢 `B3` 的实测回填面** ✗ · **不改 `emitEnvelope*`** ✗
// （六键包封不动 ⇒ 同一份取值落 stderr，缺口照实点名）· **不动 `publish/` 七件生效面** ✗ ·
// 不引新依赖（只用 `os/exec` 那个底座 + 标准库）。
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// impactCalibScriptRel —— 尺（现成槽）的件路径：`scripts/gates/check-slice.py`（**只消费、不改** ✗）。
// 为什么用这一件当尺：设计 v1.6 §5.1/§5.5 逐字「虫族不需要新造一套度量，只要把表里的『期望』换成
// 波纹的『预测』」—— 那一件的四格（期望红 / 期望绿 / 假绿 / 假红）就是波纹要的那三格。
const impactCalibScriptRel = "scripts/gates/check-slice.py"

// impactCalibTierMaxMS —— 「尺的标定」这一块在**按需档**的自我交代口径（本机现跑：`--probe` 0.09s ·
// `--selftest` 0.30s ⇒ 合计 ≈ 0.4s）。★ 这是**现跑量出来的**（见 `emitImpactTimefaceBlock` 打印的本跑耗时），
// 不是拍脑袋的上限 —— 不设上限、只报读数（**不设阈值** ✗）。
const impactCalibTierObserved = "本机现跑：`--probe` ≈ 0.09s · `--selftest` ≈ 0.30s（合计 ≈ 0.4s · 冷起含 `python3` 启动）"

// impactCalibRegistered —— **在册旧值**（引用 · 逐条带出处与**标定时刻**；本件**不重算它们**）。
// 新旧并排是这一格的全部意义：在册值说明「上一次标定是什么时候量的」，现跑值说明「此刻是多少」。
type impactCalibRegistered struct {
	Label  string
	Value  string
	Source string
	At     string
}

var impactCalibRegisteredValues = []impactCalibRegistered{
	{
		Label: "探针条数", Value: "36 条（期望红 22 · 期望绿 11 · 期望需递归 1 · 期望错误 2）",
		Source: "设计-变更影响面-v1.6 §5.1/§5.5 · 现读 `scripts/gates/precommit-gates.sh`（在册 v1.5 记 `:1591` 是**漂前的行号** ⇒ 按内容找）",
		At:     "2026-09-21/22（v1.6 写稿时现跑）",
	},
	{
		Label: "假绿（期望红 · 未判红）", Value: "0",
		Source: "设计-变更影响面-v1.6 §5.1/§5.5 · `scripts/gates/check-slice.py` 第 7 行（§3.5 质量门槛）",
		At:     "2026-09-21/22（v1.6 写稿时现跑）",
	},
	{
		Label: "假红（期望绿 · 判红）", Value: "0.0%",
		Source: "设计-变更影响面-v1.6 §5.1/§5.5 · `scripts/gates/precommit-gates.sh` 现读那一行",
		At:     "2026-09-21/22（v1.6 写稿时现跑）",
	},
	{
		Label: "门槛（现成槽自陈）", Value: "假绿 = 0 · 假红 ≤ 10%",
		Source: "`scripts/gates/check-slice.py` 第 7 行逐字（§3.5 质量门槛）· **不是本件设的**",
		At:     "在本文件里（不随时间变 —— 它是脚本自陈的口径）",
	},
	{
		Label: "外部一手 · 选测系统的两个头条数", Value: "specificity ≈ 25%（Google Testing Blog · Efficacy Presubmit）",
		Source: "设计-变更影响面-v1.6 §5.1 `Y1`（本机 `curl` 200）",
		At:     "**不可现跑重测**（外部件 · 只引出处）",
	},
	{
		Label: "外部一手 · Meta Predictive Test Selection", Value: "抓到 > 99.9% 回归 · 只跑传递依赖测试的 1/3",
		Source: "设计-变更影响面-v1.6 §5.1 `Y2`（本机 `curl` 200）",
		At:     "**不可现跑重测**（外部件 · 只引出处）",
	},
}

// impactPredictItem —— 预测面的一条（**给卵/给 `B3` 用的闭集条目**）。
type impactPredictItem struct {
	Red    string // 「会红」闭集的取值（契约 id · 或门步名 —— §3.5 铁律：只有这两类进这一格）
	Why    string // 闭集六选一的归一值（契约 / 门步 / 编译面 ……）
	How    string // **估法口径**：这一条凭什么被估成会红（怎么估的）
	Layer  string // 出处层（③ / 门禁步名真源 / ①）
	Source string // 一手出处（件:内容 · 不写行号当结论）
	State  string // 预测（本件**只出这一态**：实测态属 `B3`）
}

// impactPredictFace —— 时间面的**预测侧**（本件的全部实现）。
type impactPredictFace struct {
	Target   string
	HeadSHA  string
	At       string
	Tier     string // 默认档 / 按需档
	Items    []impactPredictItem
	NotRun   []string // **没跑**的面（逐条点名 · 不许把「没跑」写成「没有」）
	EstCalib []string // 估法的口径（三态：现算 / 投影 / 未跑）
	Cost     time.Duration
}

// impactPredictFaceOf 算预测面：把「会红」那一行的闭集取值逐条**连出处与估法**摘出来。
//
// 三条来源（就是 §5.2 表里「预测」那一侧的取值来源）：
//
//	① ③ 契约层的命中条 · `change_class == "B"` ⇒ 契约 id（口径 = 登记表自陈「改它会破承诺」）；
//	② 门步名 —— `B1` 的投影（真源 `--list` 现跑 + 逐名 `--emit-cmd` 恰好 1 条 · 真同源键 = 脚本路径）；
//	③ ① 编译层现跑已红 ⇒ 编译面（口径 = `go build ./...` 现跑读数，不是估计）。
//
// ★ 三条**都带「怎么估的」**；取不到的（② 在默认档、① 读不到）**逐条进 `NotRun`** —— 于是
// 「预测为空」与「预测没跑」在输出里是两件事（§十一 失败模式 `F6`：把「没报」读成「没影响」）。
func impactPredictFaceOf(tgt *impactTarget, layers []impactLayer, proj impactStepProjection, cheap bool) impactPredictFace {
	beg := time.Now()
	f := impactPredictFace{At: impactEffectiveAt(), Cost: 0}
	if tgt != nil {
		f.Target = tgt.Raw
	}
	tier := "按需档（`--all`）"
	if cheap {
		tier = "默认档"
	}
	f.Tier = tier
	seen := map[string]bool{}
	add := func(it impactPredictItem) {
		if it.Red == "" || seen[it.Red] {
			return
		}
		seen[it.Red] = true
		f.Items = append(f.Items, it)
	}
	// ① 契约 id（③ 层命中条 · `change_class=B`）。
	if l, ok := impactLayerBySeq(layers, "③"); ok {
		switch l.Status {
		case "取值":
			for _, r := range l.Rows {
				if strings.TrimSpace(r["red"]) == "" {
					continue
				}
				add(impactPredictItem{
					Red: r["red"], Why: "契约", Layer: "③",
					How:    "登记表自陈「改它会破承诺」（`change_class=B`）⇒ 进「会红」闭集（§3.5 铁律：语义级条目永不进这一行）",
					Source: impactRegistryRel + " 的 `change_class` / `change_note` 两列（现读 · 表行数见层表 ③）",
					State:  "预测",
				})
			}
		default:
			f.NotRun = append(f.NotRun, fmt.Sprintf("③ 契约层：%s（%s）⇒ 契约 id 这一格**没跑**，不是「没命中」", l.Status, impactFirstLine(l.Detail)))
		}
	} else {
		f.NotRun = append(f.NotRun, "③ 契约层：**本跑没有这一层**（不许当「没有契约条目」）")
	}
	// ② 门步名（`B1` 的投影 · 三态照抄，不在这里重判）。
	switch proj.Status {
	case "取值":
		for _, nm := range proj.Steps {
			add(impactPredictItem{
				Red: nm, Why: "门步", Layer: "门禁步名真源",
				How: fmt.Sprintf("真同源键 = **脚本路径**（`gate` 里那条脚本 ↔ 步**命令串**里同名脚本）· 本跑 %d 步里 join 上",
					proj.Total),
				Source: fmt.Sprintf("`bash %s --list` 现跑（head_sha=%s · 工作树=%s）+ 逐名 `--emit-cmd '<全名>'` 恰好 1 条",
					gateScriptRel, dashIfEmpty(proj.HeadSHA), proj.Dirty),
				State: "预测",
			})
		}
		if len(proj.Steps) == 0 {
			f.NotRun = append(f.NotRun, "门步那一格：**本跑没接上任何一步**（原因逐条在步名真源块）—— 「接不上」≠「不会红」")
		}
		for id, why := range proj.Unjoined {
			f.NotRun = append(f.NotRun, fmt.Sprintf("门步那一格：契约 %s **未接步**（%s）", id, why))
		}
	default:
		f.NotRun = append(f.NotRun, fmt.Sprintf("门步那一格：**%s**（%s）", impactStepStatusLabel(proj.Status), proj.Reason))
	}
	// ③ 编译面（① 层现跑已红 —— 这是**读数**，不是估计）。
	if l, ok := impactLayerBySeq(layers, "①"); ok {
		if strings.Contains(l.Detail, "编译面已红") {
			add(impactPredictItem{
				Red: "编译面", Why: "编译面", Layer: "①",
				How:    "`go build ./...` **现跑**已报错 ⇒ 编译面这一格不是估计，是读数",
				Source: "① 层读数（口径 = 现跑 `go build ./...` 的退出码与错面；层表 ① 那一行）",
				State:  "预测",
			})
		} else if l.Status != "取值" {
			f.NotRun = append(f.NotRun, fmt.Sprintf("① 编译层：%s ⇒ 编译面**没跑**（不许当「不红」）", l.Status))
		}
	}
	sort.Slice(f.Items, func(i, j int) bool {
		if f.Items[i].Why != f.Items[j].Why {
			return f.Items[i].Why < f.Items[j].Why
		}
		return f.Items[i].Red < f.Items[j].Red
	})
	sort.Strings(f.NotRun)
	// 估法口径三态（**每一条预测都要能落到其中一态**）。
	f.EstCalib = []string{
		"**现算**：① 编译层（现跑 `go build ./...` 的读数）· ③ 契约层（现读 `registry.json`）—— 这两态是「算出来的」",
		"**投影**：门步名（`B1`：真源 `--list` 现跑 + 逐名 `--emit-cmd` 恰好 1 条 ⇒ join 投影 **只覆盖 3/8 条契约**，接不上的逐条写「未接步」）",
		"**没跑**：默认档的贵项（② 符号层 · 门步真源探针）与「读不到」的层 —— 逐条进下面 `not_run[]`，**不许并进预测集**",
	}
	if _, ok := impactLayerBySeq(layers, "②"); ok && cheap {
		f.EstCalib = append(f.EstCalib, "② 符号层：默认档**未跑**（要看调用者给 `--all`）⇒ 「图距离 1 跳」这一维本跑恒哨兵值")
	}
	for _, l := range layers {
		if l.HeadSHA != "" {
			f.HeadSHA = l.HeadSHA
			break
		}
	}
	f.Cost = time.Since(beg)
	return f
}

// impactCalib — 「尺的标定」那一格（**现跑重测** + 在册旧值并留）。
type impactCalib struct {
	Status  string // 取值 / 未机检 / 取不到
	Reason  string
	Runs    []impactCalibRun     // 两条探针（`--probe` 全量混淆矩阵 · `--selftest` 自证）
	Live    []impactCalibReading // 现跑读数（逐条带耗时）
	Old     []impactCalibRegistered
	Cost    time.Duration
	At      string
	Script  string
	ScriptK bool // 脚本在盘上（只判存在 —— 默认档连它都不跑）
}

// impactCalibRun —— 一条探针的现跑结果（**只报读数**：脚本自报什么写什么，不在本件重算）。
type impactCalibRun struct {
	Name   string // `--probe` / `--selftest`
	RC     int
	Lines  int
	Tail   string // 结论行（原样）
	Paired []string
	Cost   time.Duration
}

// impactCalibReading —— 一条现跑读数（`Label` 与在册旧值同名 ⇒ 可并排比）。
type impactCalibReading struct {
	Label string
	Value string
}

// 现跑解析用的四个锚（**按内容找，不按行号** —— 设计 v1.5 的 `:1591` ⇒ `:1679` 就是行号漂移的实证）。
var (
	// `合计：探针 36 条（期望红 22 · 期望绿 11 · 期望需递归 1 · 期望错误 2）`
	impactCalibTotalRe = regexp.MustCompile(`合计：探针\s*(\d+)\s*条（期望红\s*(\d+)\s*·\s*期望绿\s*(\d+)\s*·\s*期望需递归\s*(\d+)\s*·\s*期望错误\s*(\d+)）`)
	// `真阳 TP（期望红·判红）= 22 ｜ 假阴 FN/假绿（期望红·未判红）= 0`
	impactCalibTPRe = regexp.MustCompile(`真阳 TP[^=]*=\s*(\d+)\s*｜\s*假阴 FN/假绿[^=]*=\s*(\d+)`)
	// `真阴 TN（期望绿·判绿）= 11 ｜ 假阳 FP/假红（期望绿·判红）= 0`
	impactCalibTNRe = regexp.MustCompile(`真阴 TN[^=]*=\s*(\d+)\s*｜\s*假阳 FP/假红[^=]*=\s*(\d+)`)
	// `假绿率 = 0 ✓…` / `假红率 = 0.0% ✓（≤ 10.0%，0/11）`
	impactCalibRateRe = regexp.MustCompile(`^(假绿率|假红率)\s*=\s*([0-9.]+%?)\s*(✓|✗)?`)
	// `结论：探针集 过门槛（假绿 0 / 假红在限内）；rc = 0`
	impactCalibVerdictRe = regexp.MustCompile(`^结论：(.+)$`)
	// `自证结论：全过（用例 9 条，失败 0 条）`
	impactCalibSelfRe = regexp.MustCompile(`^自证结论：(.+)$`)
	// 成对闸那两条（`✓ (e) 前置缺件闸·正 …` / `✓ (f) 前置缺件闸·反 …`）——**成对**才算成对。
	impactCalibPairRe = regexp.MustCompile(`^[✓✗]\s*\(([ef])\)\s*(.+)$`)
)

// impactCalibProbeRun 跑一条探针（**只读** · 不传任何截断开关 ⇒ **不许小额度探针** ✗）。
// 返回 (读数, 原文, 取不到的原因)；原因非空 ⇒ 这条不许当结论。
func impactCalibProbeRun(root, flag string) (impactCalibRun, string, string) {
	beg := time.Now()
	out, errb, rc, err := impactRunIn(root, "python3", impactCalibScriptRel, flag)
	r := impactCalibRun{Name: flag, RC: rc, Cost: time.Since(beg)}
	if err != nil {
		return r, "", fmt.Sprintf("起不来（%v）", err)
	}
	if rc != 0 {
		return r, out, fmt.Sprintf("退码 %d（stderr 头一行：%s）", rc, impactFirstLine(errb))
	}
	r.Lines = impactCountLines(out)
	for _, ln := range strings.Split(out, "\n") {
		t := strings.TrimSpace(ln)
		// 结论行：探针集那条（不含「结论：共 N 片」那种单片的结论行）· 自证那条。
		if m := impactCalibVerdictRe.FindStringSubmatch(t); m != nil && strings.Contains(m[1], "探针集") {
			r.Tail = t
		}
		if m := impactCalibSelfRe.FindStringSubmatch(t); m != nil {
			r.Tail = t
		}
		if m := impactCalibPairRe.FindStringSubmatch(t); m != nil {
			r.Paired = append(r.Paired, t)
		}
	}
	return r, out, ""
}

// impactCalibLive 从 `--probe` 的现跑输出里摘四格（**不齐 ⇒ 取不到** · 不许拿部分当全部）。
//
// 四格 = 期望红 / 期望绿 / 假绿 / 假红（§5.1 的现成槽）；另摘两条率与结论行。齐不齐的判据：
// 自报条数 == 期望红 + 期望绿 + 期望需递归 + 期望错误，且 真阳+假阴 == 期望红、真阴+假阳 == 期望绿。
func impactCalibLive(out string) ([]impactCalibReading, string) {
	var tot, expRed, expGreen, expRec, expErr int
	var tp, fn, tn, fp int
	var hasTot, hasTP, hasTN bool
	greenRate, redRate := "", ""
	verdict := ""
	for _, ln := range strings.Split(out, "\n") {
		t := strings.TrimSpace(ln)
		if m := impactCalibTotalRe.FindStringSubmatch(t); m != nil {
			tot, _ = strconv.Atoi(m[1])
			expRed, _ = strconv.Atoi(m[2])
			expGreen, _ = strconv.Atoi(m[3])
			expRec, _ = strconv.Atoi(m[4])
			expErr, _ = strconv.Atoi(m[5])
			hasTot = true
		}
		if m := impactCalibTPRe.FindStringSubmatch(t); m != nil {
			tp, _ = strconv.Atoi(m[1])
			fn, _ = strconv.Atoi(m[2])
			hasTP = true
		}
		if m := impactCalibTNRe.FindStringSubmatch(t); m != nil {
			tn, _ = strconv.Atoi(m[1])
			fp, _ = strconv.Atoi(m[2])
			hasTN = true
		}
		if m := impactCalibRateRe.FindStringSubmatch(t); m != nil {
			if m[1] == "假绿率" {
				greenRate = m[2]
			} else {
				redRate = m[2]
			}
		}
		if m := impactCalibVerdictRe.FindStringSubmatch(t); m != nil && strings.Contains(m[1], "探针集") {
			verdict = t
		}
	}
	if !hasTot || !hasTP || !hasTN {
		return nil, "现跑输出里四格**摘不全**（自报条数 / 真阳行 / 真阴行缺任一）⇒ **取不到**（不齐不出数）"
	}
	if tot != expRed+expGreen+expRec+expErr || expRed != tp+fn || expGreen != tn+fp {
		return nil, fmt.Sprintf("现跑输出**不齐**：自报 %d 条 ⇔ 22/11/1/2 四格之和 %d · 期望红 %d ⇔ 真阳+假阴 %d · 期望绿 %d ⇔ 真阴+假阳 %d ⇒ **取不到**",
			tot, expRed+expGreen+expRec+expErr, expRed, tp+fn, expGreen, tn+fp)
	}
	if greenRate == "" || redRate == "" {
		return nil, "现跑输出里两条**率**（假绿率 / 假红率）缺任一 ⇒ **取不到**（阈值那一格不许只有一半）"
	}
	out2 := []impactCalibReading{
		{Label: "探针条数", Value: fmt.Sprintf("%d 条（期望红 %d · 期望绿 %d · 期望需递归 %d · 期望错误 %d）",
			tot, expRed, expGreen, expRec, expErr)},
		{Label: "假绿（期望红 · 未判红）", Value: fmt.Sprintf("%d（真阳 %d / 假阴 %d）", fn, tp, fn)},
		{Label: "假红（期望绿 · 判红）", Value: fmt.Sprintf("%s（真阴 %d / 假阳 %d）", redRate, tn, fp)},
		{Label: "假绿率（脚本自陈）", Value: greenRate},
		{Label: "假红率（脚本自陈）", Value: redRate},
	}
	if verdict != "" {
		out2 = append(out2, impactCalibReading{Label: "脚本结论行", Value: verdict})
	}
	return out2, ""
}

// impactCalibrationOf 拉「尺的标定」那一格。`pull=false`（默认档）⇒ **一次子进程都不起**，
// 照实写「未机检 + 怎么拉」（与 `B1` 的真源探针同一条预算纪律 §4.4）。
func impactCalibrationOf(root string, pull bool) (c impactCalib) {
	c = impactCalib{Script: impactCalibScriptRel, At: impactEffectiveAt(), Old: impactCalibRegisteredValues}
	beg := time.Now()
	// 具名返回值 + defer ⇒ 每条 return 都带上本块的耗时（耗时属于口径三件套 §4.4：不许静默丢）。
	defer func() { c.Cost = time.Since(beg) }()
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(impactCalibScriptRel))); err != nil {
		c.Status = "取不到"
		c.Reason = fmt.Sprintf("尺的件不在盘上（%s · %v）⇒ 标定**取不到**（「取不到」**不许**读成「假绿 0」）", impactCalibScriptRel, err)
		return c
	}
	c.ScriptK = true
	if !pull {
		c.Status = "未机检"
		c.Reason = "尺的标定要起 `python3`（" + impactCalibTierObserved + "）⇒ 按 §4.4 与 `B1` **同一预算约束**" +
			"不进默认档；要看现跑读数给 `zerg impact <目标> --all`（按需档）"
		return c
	}
	probe, raw, why := impactCalibProbeRun(root, "--probe")
	c.Runs = append(c.Runs, probe)
	if why != "" {
		c.Status = "取不到"
		c.Reason = "`--probe` " + why + " ⇒ 标定**取不到**（**失败 ≠ 事实为零**：这一格今天就是「不知道」，不是「一例假绿都没有」）"
		return c
	}
	live, why := impactCalibLive(raw)
	if why != "" {
		c.Status = "取不到"
		c.Reason = why
		return c
	}
	c.Live = live
	self, _, why2 := impactCalibProbeRun(root, "--selftest")
	c.Runs = append(c.Runs, self)
	if why2 != "" {
		c.Status = "取不到"
		c.Reason = "`--selftest` " + why2 + " ⇒ 尺**自己没自证** ⇒ 上面那几个数不当结论"
		return c
	}
	if len(self.Paired) != 2 {
		c.Status = "取不到"
		c.Reason = fmt.Sprintf("`--selftest` 的**成对闸**没成对（本跑只摘到 %d 条 `(e)/(f)` 行）⇒ 「成对正控/负控」不成立 ⇒ 取不到", len(self.Paired))
		return c
	}
	c.Status = "取值"
	c.Reason = "两条探针都现跑成功（`--probe` 全量探针集 · `--selftest` 九条用例 + **前置缺件闸正反成对**）"
	return c
}

// emitImpactTimefaceBlock —— 时间面**预测侧**块（stderr · 人读 + 一条机读行）。
//
// 为什么落 stderr：六键包封里**没有**第七个键（`A1`–`A5`/`B1` 同款红线「不改 `emitEnvelope*`」）⇒
// 同一份取值落 stderr，缺口照实点名（`B3` 要消费 `predict_red[]` 也读这**同一个**闭集）。
func emitImpactTimefaceBlock(w io.Writer, tgt *impactTarget, f impactPredictFace, c impactCalib, cheap bool) {
	fmt.Fprintf(w, "%s: `B2` 时间面（**预测侧** · 设计-变更影响面-v1.6 §五 · 任务单-影响面实施-20260922 §三 `B2`）—— "+
		"两个时刻里的**前一个**：本块只出「预测」这一侧，带**估法口径**\n", progName)
	fmt.Fprintf(w, "  本跑：目标=%s · 档位=%s · head_sha=%s · 取数时刻=%s · 本块耗时=%s\n",
		dashIfEmpty(f.Target), f.Tier, dashIfEmpty(f.HeadSHA), f.At, f.Cost.Round(time.Microsecond).String())
	// ① 预测集（闭集 · 逐条带「怎么估的」）。
	fmt.Fprintf(w, "  预测集 `predict_red[]`：本跑 %d 条（闭集取值 = 契约 id + 门步名 + 编译面 —— §3.5 铁律：语义级条目**永不**进这一格）\n",
		len(f.Items))
	for _, it := range f.Items {
		fmt.Fprintf(w, "    · [%s] %s ← 怎么估的：%s\n", it.Why, it.Red, it.How)
		fmt.Fprintf(w, "        出处：%s · 状态：**%s**（实测态不属本件 ⇒ `B3`）\n", it.Source, it.State)
	}
	if len(f.Items) == 0 {
		fmt.Fprintf(w, "    （本跑闭集为空 —— 逐条看下面 `not_run[]`：**「预测为空」与「没跑」是两件事**）\n")
	}
	// ② 没跑的面（逐条点名 —— 宁少报不猜报）。
	fmt.Fprintf(w, "  `not_run[]`（**没跑 ≠ 没有** · §十一 `F6`）：本跑 %d 条\n", len(f.NotRun))
	for _, s := range f.NotRun {
		fmt.Fprintf(w, "    · %s\n", s)
	}
	// ③ 估法口径三态（§5.1：不写口径的预测，连值不值都判不了）。
	for _, s := range f.EstCalib {
		fmt.Fprintf(w, "  估法口径：%s\n", s)
	}
	// ④ 机读行（同一份取值 ⇒ `B3` 拿它当**预测侧的输入**；本件不落盘、不写件）。
	line := impactPredictJSON(f)
	fmt.Fprintf(w, "  机读（stderr 一行 · **不落盘** · `B3` 的回填件读的是这同一个闭集）：%s\n", line)
	// ⑤ 实测侧：明写未做（**不抢 `B3`** ✗）。
	fmt.Fprintf(w, "  ★ 实测侧（命中 / 漏报 / 虚报 三个整数 · `impact_actual` · 回填件）：**本批不做** ✗ —— 属 `B3`（§5.2/§5.3/§5.5 · `R57`）；"+
		"本件**只报预测侧**、不代它落任何一枚实测数；`BLOCKED` 不计入漏报那条纪律也在 `B3`（本件连这一格都不碰）\n")
	// ⑥ 尺：在册旧值并留 + 现跑重测（按需档）。
	fmt.Fprintf(w, "  尺（§5.1 的现成槽 · **不新造度量**）= `%s`：它的四格（期望红 / 期望绿 / 假绿 / 假红）就是波纹要的那几格\n",
		impactCalibScriptRel)
	fmt.Fprintf(w, "  在册旧值（**并留** · 逐条带出处与标定时刻 —— 本件不重算它们）：\n")
	for _, v := range c.Old {
		fmt.Fprintf(w, "    · %s = %s｜出处：%s｜标定时刻：%s\n", v.Label, v.Value, v.Source, v.At)
	}
	switch c.Status {
	case "取值":
		fmt.Fprintf(w, "  现跑重测（**本跑** · 时刻=%s · 两条探针耗时合计=%s · %s）：\n",
			c.At, c.Cost.Round(time.Millisecond).String(), impactCalibTierObserved)
		for _, r := range c.Runs {
			fmt.Fprintf(w, "    · `%s` rc=%d · 输出 %d 行 · 耗时 %s · 结论行：%s\n",
				r.Name, r.RC, r.Lines, r.Cost.Round(time.Millisecond).String(), kvOrDash(r.Tail))
		}
		for _, rd := range c.Live {
			fmt.Fprintf(w, "    · 【现跑】%s = %s\n", rd.Label, rd.Value)
		}
		fmt.Fprintf(w, "  自证成对（`--selftest` · **只跑半边不算成对**）：\n")
		for _, r := range c.Runs {
			for _, s := range r.Paired {
				fmt.Fprintf(w, "    %s\n", s)
			}
		}
		fmt.Fprintf(w, "  ★ **不许小额度探针** ✗：两条探针都**不传任何截断开关**（条数由脚本自报 = 上面那个数）；"+
			"四格与自报条数**不齐 ⇒ 取不到**（本跑齐 ⇒ 出数）\n")
	case "未机检":
		fmt.Fprintf(w, "  现跑重测：**未机检** —— %s\n", c.Reason)
		fmt.Fprintf(w, "    ⇒ 上面那些**在册旧值**照留（它们是「上一次标定」）；本跑**一个现跑数都不编**（宁少报不猜报）\n")
	default:
		fmt.Fprintf(w, "  现跑重测：**取不到** —— %s\n", c.Reason)
		fmt.Fprintf(w, "    ⇒ **失败 ≠ 事实为零** ✗：这一格今天就是「**不知道**」，**不许**读成「假绿 0 / 假红 0.0%%」\n")
	}
	// ⑦ 阈值那一格（**只引出处 · 不自设**）。
	fmt.Fprintf(w, "  阈值（**谁设的**）：现成槽自陈的那一条 —— **假绿 = 0 · 假红 ≤ 10%%**，出处 = `%s` 第 7 行（§3.5 质量门槛，逐字）；"+
		"**波纹自己不设任何阈值** ✗（`R30` · §十一 `U20`：跨系统通用阈值**没有一手出处** ⇒ 口径可定、阈值不可编）\n", impactCalibScriptRel)
	fmt.Fprintf(w, "  ★ 与 `B1` 同一预算（§4.4）：本块的探针是**按需档**项（默认档一次都不起子进程）—— 默认档那一档照实写「未机检 + 怎么拉」\n")
	fmt.Fprintf(w, "  零副作用：`--probe` 只读探针件、`--selftest` 在临时目录自证 ⇒ 不写缓存 / 不落审计 / 不改件 / 不改门禁脚本；本块也**不落盘**（只出文本）\n")
	fmt.Fprintf(w, "  诚实边界（§十一）：波纹保不了「**该不该红**」—— 这一块只回答「**预测了哪些** · **怎么估的** · **这尺上一次标定是什么时候/此刻是多少**」\n")
}

// impactPredictJSON 预测面的机读行（**定序 · 无时钟无耗時** ⇒ 同一目标两跑逐字相同 · `M8`）。
// 形状（不是六键包封的一部分 —— 包封那一格**一个键都不加** ✗）：
//
//	{"signal":"impact.predict","target":…,"head_sha":…,"tier":…,"closed_set_from":{…},"predict_red":[…],"not_run":[…]}
func impactPredictJSON(f impactPredictFace) string {
	type item struct {
		Red    string `json:"red"`
		Why    string `json:"why"`
		How    string `json:"how"`
		Layer  string `json:"layer"`
		Source string `json:"source"`
		State  string `json:"state"`
	}
	doc := struct {
		Signal       string            `json:"signal"`
		Target       string            `json:"target"`
		HeadSHA      string            `json:"head_sha"`
		Tier         string            `json:"tier"`
		ClosedSet    map[string]string `json:"closed_set_from"`
		PredictRed   []item            `json:"predict_red"`
		NotRun       []string          `json:"not_run"`
		ActualSideBy string            `json:"actual_side"`
		ThresholdBy  string            `json:"threshold_from"`
	}{
		Signal: "impact.predict", Target: f.Target, HeadSHA: f.HeadSHA, Tier: f.Tier,
		ClosedSet: map[string]string{
			"契约 id": impactRegistryRel + " 的 `change_class`（现读 · 只有 `B` 进闭集）",
			"门步名":   "`bash " + gateScriptRel + " --list` 现跑 + 逐名 `--emit-cmd`（按需档）；默认档 = 未机检",
			"编译面":   "① 层现跑 `go build ./...` 的读数",
			"语义级条目": "**永不进闭集**（§3.5 铁律）",
		},
		PredictRed:   []item{},
		NotRun:       []string{},
		ActualSideBy: "B3（本件不落实测值）",
		ThresholdBy:  impactCalibScriptRel + " 第 7 行（§3.5：假绿 = 0 · 假红 ≤ 10%）—— 波纹自己不设阈值",
	}
	for _, it := range f.Items {
		doc.PredictRed = append(doc.PredictRed, item{Red: it.Red, Why: it.Why, How: it.How, Layer: it.Layer, Source: it.Source, State: it.State})
	}
	doc.NotRun = append(doc.NotRun, f.NotRun...)
	b, err := json.Marshal(doc)
	if err != nil {
		return fmt.Sprintf("（机读行拼不出来：%v）", err)
	}
	return string(b)
}
