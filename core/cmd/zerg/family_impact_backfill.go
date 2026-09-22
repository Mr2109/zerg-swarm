// family_impact_backfill.go —— 实测回填闭环（`B3` · 任务单-影响面实施-20260922 §三 `B3` 八字段 ·
// 设计-变更影响面-v1.6 §5.2 三个数 / §5.3 `impact_actual` / §5.5 预测回填闭环〔M3 的落点〕）。
//
// # 任务单 §三 `B3` 八字段**逐字**（现读 `Zerg-内部文档/项目文档/v2.5.11/任务单-影响面实施-20260922.md`）
//
//	编号（L136）      ：B3
//	目标（L137）      ：改完自动对拍（预测 vs 实际）⇒ 落一枚 `impact_actual` + **回填成机读件**（命中 / 漏报 / 虚报）。
//	判据（L138）①     ：拿 v1.6 §5.2 的两步跑通一次，落一枚 `impact_actual`（**三个整数 + `head_sha` +
//	                    blocked 步名[]**），且**数字可复算**（同一次门禁结果 + 同 `head_sha` ⇒ 三数逐字相同）；
//	判据（L138）②     ：**`BLOCKED` 不计入漏报** ✓；
//	判据（L138）③     ：**回填件追加式**：连跑两次 ⇒ **历史行逐字节不变**（`head -n <旧行数>` 逐字节比）；
//	判据（L138）④     ：**回填件至少八键**（`head_sha` / `target` / `predict_red[]` / `actual_fail[]` /
//	                    `actual_blocked[]` / `hit` / `miss` / `false_alarm` / `gate_run_id` / `at`），
//	                    **缺任一 ⇒ 该行不许写**；
//	判据（L138）⑤     ：**不可机检** ✗ —— 「这三个数好不好」判不了（无一手阈值）；
//	产物（L139）      ：审计行第二枚字段（`impact_actual`）+ `<状态目录>/impact-backfill.jsonl`（**建议名**）；
//	前置依赖（L140）  ：`R30` / `R33` / `R57` 已拍；门禁四档（`PASS` / `FAIL` / `BLOCKED` / `REPORT`）语义现成 ✓；
//	风险（L141）      ：**⚠ 冲突⑧** —— 「自动回填」（记账）与「拿对拍结果自动收窄预测」（反馈回路）**只差一步**
//	                    ⇒ 实现时若有人顺手把回填件接进预测算法，**越线**；另一风险 = 人记的账会漂
//	                    ⇒ 本任务的技术前提就是「**不许逐条人工记**」；
//	红线（L142）      ：**不许设阈值** ✗ · **不许进门禁** ✗ · **不许自动改「会红」的算法** ✗ ·
//	                    **不许把 `BLOCKED` 混进漏报** ✗ · **不许拿回填结果回改已发出的卡片** ✗（事后改卡片 =
//	                    伪造历史）· **不许改历史行** ✗；
//	预计改动面（L143）：未估。
//
// # ⚠ 冲突⑧ 的裁定**逐字**（拍板清单-影响面-v1.6-20260922 §三）
//
//	「**允许自动回填，但必须同时拍死一句**：**「回填件只被读，不作任何算法的输入」** ⇒
//	 把回填接成反馈回路 = **越线** ✗」
//
// ⇒ 本件把这一句落成**可判的两半**（都在输出里打出来 —— 「拍死」不是一句注释）：
//
//	① **静态**（`impactBackfillReadersScan`）：预测 / 排序 / 裁条 / 退码四条路径的源件里，对**回填件名**
//	   与**那两个碰件的函数名**的引用 **0 处**（逐件现读 · 读不到的源件 ⇒ 这一半**不给结论**，不当 0 处）；
//	② **运行期**（`cli_impact_backfill_test.go` 里真跑）：同一目标改回填值 ⇒ 预测 / 排序 / 退码**逐字不变**。
//
// # 触发点（`R57` 的「三候选择一」· 本件选**第 ① 个**：**门禁跑完的收口**）
//
// 设计 §5.5 逐字给三个候选 —— ① 门禁跑完的收口（**唯一能拿到「真红」的时刻**）· ② `zerg dev edit`
// 真写后的收口（**与 `impact_digest` 同一落点**）· ③ 独立的低频批处理（口径最干净、最不打扰人，但会迟到）。
// 本件选 ①，理由逐条（**不是口味，是两条硬约束推出来的**）：
//
//	· `zerg dev edit` 的次序是**真写在前、门禁跑在后** ⇒ 真写那一刻**没有**「真红」（② 拿不到账）；
//	· §5.5 的共同约束逐字是「回填**只读门禁与审计件**」⇒ 回填**不许自己跑门禁** ⇒ 只有「门禁跑完」这一时刻
//	  两个集合都在手（预测侧可现算、实际侧 = **那次门禁自己的 `results.tsv`**）；
//	· 落法 = **一条显式命令**（`zerg impact <目标> --gate-results <那次门禁的结果表 | 它的日志目录>`）——
//	  它就是那次门禁的**收口动作**；**不塞进 `gate` 族透传面**（`gate.go` 的铁律「只转发、不翻译」一字不动 ✗）；
//	· 迟到的代价**照实写**：回填发生在「门禁跑完之后」，不是「改动落地之前」—— 与候选 ③ 的迟到同族，
//	  只是由人 / 代理在**那次门禁的收口**显式触发（不靠谁记得：输入**必须**是那次门禁自己的结果表）。
//
// # 三条口径（照任务单 / 设计稿逐字，缺一条都不许出数）
//
//	① **只记三数、不设阈值** ✗（`R30`）：「这三个数好不好」**判不了**（无一手阈值）—— 本件**只引现成槽**
//	   （`scripts/gates/check-slice.py` 第 7 行的混淆矩阵「假绿 = 0 · 假红 ≤ 10%」）当**参照**，
//	   波纹自己**一个阈值都不设**（§5.5 逐字：**不需要新造度量**，把「期望」换成波纹的「预测」）。
//	② **步名闭集 = 真源 `--list` 现跑**（与 `B1` 同规矩 · **不写常量** ✗）：预测侧的步名走 `B1` 的 join 投影
//	   （真同源键 = 脚本路径）；实际侧的步名走**那次门禁自己的结果表**；**两边都不许手抄**。
//	   齐不齐判据：结果表里的步名集合必须与真源现跑的**两个档之一**（**全量档** / **快速档**）**逐字相等**
//	   ⇒ 对不上**取不到**（不许拿 `--scope` 子集当全量）。
//	③ **`BLOCKED` 不进漏报** ✓（判据②）：三数只在「判过红绿」的步上算（`FAIL` = 真红 · `PASS` = 判绿）；
//	   `BLOCKED`（不给结论）与 `REPORT`（只报告）**逐条单列** —— 既不算漏报，也不算虚报
//	   （两档都**不是「判绿」** ⇒ 落在它们上面的预测**没有反证**，不许当成虚报）。
//
// # 零副作用边界（逐条都是可判的）
//
//	· **干跑不写回填** ✗：不给确认档（或给 `--dry-run`）⇒ 只算、只打、**一个字节都不落**；
//	· **写失败不影响主流程退码** ✓：追加失败只 stderr 点名，退码仍由「对拍 / 取数」那一档决定
//	  （见 `emitImpactBackfillBlock` 三档出口）；
//	· **只追加**：`append_only`（契约件逐字）—— 一行一枚，**历史行一个字都不改**；
//	· **缺任一必填键 ⇒ 该行不许写**（判据④ · 宁缺不猜）。
//
// # 落点那一格的缺口（**照实标，不伪造**）
//
// 产物① 点名的「审计行第二枚字段（`impact_actual`）」在 `A4` 已立（`editAuditLine.ImpactActual`）；
// 但**真红只在门禁跑完之后才有**，而审计行在 `dev edit` **真写那一刻**就落盘 ⇒ 事后回填那一格 =
// **改历史行** ✗（红线）。故本批的落法是：`impact_actual` 的**取值**（§5.3 逐字形状）落在**回填件那一行**
// （同一份取值 · 见 `impactActualPayload`），审计行那一格**照 `omitempty` 不写空串冒充**
// （「读不到」不当「没有」）。要它真落进审计行 ⇒ 得让真写**之后**的收口能**追加**一枚审计事件 ——
// 那要动 `dev edit` 真写面 / 审计件（`R14` 未拍 ⇒ 拍前不许动工 ✗），**属另一批**。
//
// # 为什么这一块落 stderr
//
// 六键包封（`schema`/`kind`/`items`/`meta`/`warnings`/`truncated`）**一个键都不加** ✗（`A1`–`B2` 同一条红线
// 「不改 `emitEnvelope*`」）⇒ 同一份取值落 stderr，缺口照实点名（与层表 / 时间面 / 契约面三块同款）。
//
// # 红线（本件逐条）
//
// **不设阈值** ✗ · **不进门禁** ✗ · **不自动改「会红」的算法** ✗ · **不把 `BLOCKED` 混进漏报** ✗ ·
// **不拿回填结果回改已发出的卡片** ✗ · **不改历史行** ✗ · **回填件只被读、不作任何算法的输入** ✗（冲突⑧）·
// **不改 `emitEnvelope*`** ✗ · **不动 `publish/` 七件生效面** ✗ · **不动 `dev edit` 真写前置** ✗（`R14`）·
// **不动归档区** ✗ · 不引新依赖（只用标准库）。
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ---- 契约真源（`S-i` · `core/internal/contract/impact-backfill.json`）-------------------------

// impactBackfillContractRel —— 回填闭环的**契约真源**（登记在 `registry.json` 的 `S-i`）。
// 落点文件名与十键都**读这一份**，源码里不另写第二份（与 `A5` 读 `impact-state.json` 同一条纪律）。
const impactBackfillContractRel = "core/internal/contract/impact-backfill.json"

// impactBackfillContract —— 契约件的形状（本件读它，不复制它的口径文本）。
type impactBackfillContract struct {
	Schema       string   `json:"schema"`
	ID           string   `json:"id"`
	File         string   `json:"file"`
	AppendOnly   bool     `json:"append_only"`
	RequiredKeys []string `json:"required_keys"`
	Slot         struct {
		File      string `json:"file"`
		Line      int    `json:"line"`
		Threshold string `json:"threshold"`
	} `json:"slot"`
	NotInputTo []string `json:"not_input_to"`
}

// impactBackfillFlagGiven —— `--gate-results` **给没给**（值给没给另判）：`flagValue` 只在「后面还有一格」时
// 才算给过 ⇒ 收尾那一格的「**给了旗标没给值**」它看不见，而这一格正是要判的用法错（退 2 · stdout 0 字节）。
func impactBackfillFlagGiven(orig []string) bool {
	for _, a := range orig {
		if a == "--gate-results" || strings.HasPrefix(a, "--gate-results=") {
			return true
		}
	}
	return false
}

// impactBackfillContractOf 现读契约件（**只读真源**）。返回原因非空 ⇒ 调用方按**不给结论**（退 8）处理
// —— 「读不到」不许当「没有要求」（否则十键判据会静默退化成「随便写」）。
func impactBackfillContractOf(root string) (impactBackfillContract, string) {
	c := impactBackfillContract{}
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(impactBackfillContractRel)))
	if err != nil {
		return c, fmt.Sprintf("契约件读不到（%s · %v）", impactBackfillContractRel, err)
	}
	if err := json.Unmarshal(b, &c); err != nil {
		return c, fmt.Sprintf("契约件解不开（%v）", err)
	}
	if strings.TrimSpace(c.File) == "" || len(c.RequiredKeys) == 0 {
		return c, "契约件的 `file` / `required_keys` 缺任一（**空表不许当「没有要求」**）"
	}
	return c, ""
}

// ---- ① 那次门禁的结果表（**只读** · 真源 = 脚本自己落的 `results.tsv`）----------------------

// impactGateStatusFour —— 四档（门禁脚本自陈的闭集；本件只认这四值 —— 认不出的行 ⇒ 取不到）。
var impactGateStatusFour = []string{"PASS", "FAIL", "BLOCKED", "REPORT"}

// impactGateResultRow 结果表的一行（脚本 `run_step` 的原形：`STATUS<TAB>步名<TAB>rc<TAB>耗时<TAB>日志`，
// 逐列不改 —— 本件不另立一套解析）。
type impactGateResultRow struct {
	Status string
	Name   string
	RC     string
	Secs   string
	Log    string
}

// impactGateRun 那次门禁的**只读快照**（本件只读它 ⇒ §5.5「回填只读门禁与审计件」那一句的落点）。
//
// `Fail` / `Blocked` / `Report` / `Pass` 四列**按结果表原序**（那次门禁自己的步序）—— 不许重排
// （重排会让「读回来的账」与那次门禁的报告对不上）。
type impactGateRun struct {
	Arg      string // 用户给的（原样）
	Dir      string // 结果表所在目录（绝对）
	Path     string // `results.tsv`（绝对）
	RunID    string // `gate_run_id` = <目录基名>@<结果表 sha256 前 16>（**第三者可用同一件复算**）
	TableSHA string
	Rows     []impactGateResultRow
	Fail     []string
	Blocked  []string
	Report   []string
	Pass     []string
	Status   string // 取值 / 取不到
	Reason   string
	Cost     time.Duration
}

// impactGateResultsRead 读那次门禁的结果表（**只读**；目录 ⇒ 自动接 `results.tsv`）。
// 五条「取不到」（全部照实写原因，**一条都不许当成「没有红」**）：
// 没给 / 找不到 / 读不动 / 空表 / 有一行不是 5 列或状态不在四档内。
func impactGateResultsRead(root, arg string) (r impactGateRun) {
	beg := time.Now()
	r = impactGateRun{Arg: arg, Status: "取不到"}
	defer func() { r.Cost = time.Since(beg) }()
	if strings.TrimSpace(arg) == "" {
		r.Reason = "没给结果表（`--gate-results <那次门禁的结果表 | 它的日志目录>`）"
		return r
	}
	p := arg
	if !filepath.IsAbs(p) {
		p = filepath.Join(root, filepath.FromSlash(p))
	}
	st, err := os.Stat(p)
	if err != nil {
		r.Reason = fmt.Sprintf("结果表 / 目录读不到（%s · %v）—— 「读不到」不当「没有」", p, err)
		return r
	}
	if st.IsDir() {
		r.Dir = p
		p = filepath.Join(p, "results.tsv")
	} else {
		r.Dir = filepath.Dir(p)
	}
	b, err := os.ReadFile(p)
	if err != nil {
		r.Reason = fmt.Sprintf("结果表读不到（%s · %v）", p, err)
		return r
	}
	if strings.TrimSpace(string(b)) == "" {
		r.Reason = fmt.Sprintf("结果表是空的（%s）⇒ 取不到（**空表不许当「一条都没红」**）", p)
		return r
	}
	r.Path = p
	h := sha256.Sum256(b)
	r.TableSHA = hex.EncodeToString(h[:])
	r.RunID = filepath.Base(r.Dir) + "@" + r.TableSHA[:16]
	for _, ln := range strings.Split(strings.TrimRight(string(b), "\n"), "\n") {
		f := strings.Split(ln, "\t")
		if len(f) < 5 {
			r.Reason = fmt.Sprintf("结果表有一行不是 5 列（%d 列：%q）⇒ 取不到（**半份表不当结论**）",
				len(f), impactTruncText(ln, 60))
			return r
		}
		row := impactGateResultRow{Status: f[0], Name: f[1], RC: f[2], Secs: f[3], Log: f[4]}
		if !containsStr(impactGateStatusFour, row.Status) {
			r.Reason = fmt.Sprintf("结果表有第 %q 档状态（四档闭集外）⇒ 取不到", row.Status)
			return r
		}
		if strings.TrimSpace(row.Name) == "" {
			r.Reason = "结果表有一行的**步名是空的** ⇒ 取不到（拿空步名对拍 = 拿噪声对拍）"
			return r
		}
		r.Rows = append(r.Rows, row)
		switch row.Status {
		case "FAIL":
			r.Fail = append(r.Fail, row.Name)
		case "BLOCKED":
			r.Blocked = append(r.Blocked, row.Name)
		case "REPORT":
			r.Report = append(r.Report, row.Name)
		default:
			r.Pass = append(r.Pass, row.Name)
		}
	}
	if len(r.Rows) == 0 {
		r.Reason = "结果表 0 行 ⇒ 取不到（「没跑到」不许当「全绿」）"
		return r
	}
	r.Status = "取值"
	return r
}

// ---- ② 真源现跑：两个档的步名闭集（**步数不写常量** · 与 `B1` 同规矩）-----------------------

// impactBackfillListSteps 拉一次真源清单（`--list` / `--list --fast`）—— 解析器与 `B1` **共用**
// （`impactStepParseList`：不另造第二套切法）。不齐（解析条数 ≠ 自报步数）⇒ 返回原因。
func impactBackfillListSteps(root string, fast bool) (steps []string, total int, why string) {
	args := []string{gateScriptRel, "--list"}
	flag := "--list"
	if fast {
		args = append(args, "--fast")
		flag = "--list --fast"
	}
	out, errb, code, err := impactRunIn(root, "bash", args...)
	if err != nil {
		return nil, 0, fmt.Sprintf("`%s` 起不来（%v）", flag, err)
	}
	if code != 0 {
		return nil, 0, fmt.Sprintf("`%s` 退码 %d（stderr 头一行：%s）", flag, code, impactFirstLine(errb))
	}
	rows, n := impactStepParseList(out)
	if n == 0 || len(rows) != n {
		return nil, 0, fmt.Sprintf("`%s` 清单解析 %d 条 ≠ 自报步数 %d ⇒ **不对齐**（两个数都不当结论）",
			flag, len(rows), n)
	}
	names := make([]string, 0, len(rows))
	for _, x := range rows {
		names = append(names, x.Name)
	}
	return impactStepSortedUniq(names), n, ""
}

// impactBackfillTruth 真源现跑的两档快照（**只跑 `--list`**，一个门步骤都不跑）。
type impactBackfillTruth struct {
	FullTotal int
	FullSteps []string
	FastTotal int
	FastSteps []string
	Status    string // 取值 / 取不到
	Reason    string
	Cost      time.Duration
}

func impactBackfillTruthPull(root string) (t impactBackfillTruth) {
	beg := time.Now()
	defer func() { t.Cost = time.Since(beg) }()
	fs, fn, why := impactBackfillListSteps(root, false)
	if why != "" {
		t.Status, t.Reason = "取不到", "全量档真源："+why
		return t
	}
	ts, tn, why2 := impactBackfillListSteps(root, true)
	if why2 != "" {
		t.Status, t.Reason = "取不到", "快速档真源："+why2
		return t
	}
	t.FullSteps, t.FullTotal, t.FastSteps, t.FastTotal, t.Status = fs, fn, ts, tn, "取值"
	return t
}

// impactBackfillAlign —— 齐不齐判据（口径见本件头注：**不许拿子集当全量**）。
// 逐字比两个集合（`impactStepSortedUniq` 已定序 ⇒ 直接逐行比），命中哪个档就把那个档名报出来。
func impactBackfillAlign(run impactGateRun, t impactBackfillTruth) (tier, why string) {
	if t.Status != "取值" {
		return "", t.Reason
	}
	got := make([]string, 0, len(run.Rows))
	for _, r := range run.Rows {
		got = append(got, r.Name)
	}
	got = impactStepSortedUniq(got)
	for _, c := range []struct {
		tier  string
		steps []string
		total int
	}{{"全量档", t.FullSteps, t.FullTotal}, {"快速档", t.FastSteps, t.FastTotal}} {
		if len(got) == len(c.steps) && strings.Join(got, "\n") == strings.Join(c.steps, "\n") {
			return c.tier, ""
		}
	}
	return "", fmt.Sprintf("结果表 %d 行（%d 个不同步名）⇔ 真源现跑（**全量档** %d 步 / **快速档** %d 步）"+
		"**两个档都对不上** ⇒ **不齐 ⇒ 取不到**（不许拿 `--scope` 子集当全量；也不许拿半份表当结论）",
		len(run.Rows), len(got), t.FullTotal, t.FastTotal)
}

// ---- ③ 预测侧（**闭集 = 门步名** · 复用 `B1`/`B2` 的现成件，不另造一套）----------------------

// impactBackfillPredictOf 算预测侧：③ 契约层（只读 `registry.json`）⇒ 命中的契约 id（`change_class=B`
// 的那些带 `red`）⇒ `B1` 的 join 投影（真源 `--list` 现跑 + 逐名 `--emit-cmd` 恰好 1 条）。
//
// ★ 契约 id 与「编译面」**不进对拍集**（不是步名 —— 拿两种粒度相减是错口径）；契约 id 只作**附键**记下来。
func impactBackfillPredictOf(root string, tgt *impactTarget) (steps, ids, redIDs []string,
	proj impactStepProjection, why string) {
	lay := impactLayerContract(root, tgt)
	if lay.Status != "取值" {
		return nil, nil, nil, proj, fmt.Sprintf("③ 契约层%s（%s）⇒ 预测侧**没跑**（不许当「不会红」）",
			lay.Status, impactFirstLine(lay.Detail))
	}
	for _, r := range lay.Rows {
		w := strings.TrimSpace(r["what"])
		if !strings.HasPrefix(w, "契约级:") {
			continue
		}
		id := strings.TrimPrefix(w, "契约级:")
		if id == "" {
			continue
		}
		ids = append(ids, id)
		if strings.TrimSpace(r["red"]) != "" {
			redIDs = append(redIDs, r["red"])
		}
	}
	ids = impactStepSortedUniq(ids)
	redIDs = impactStepSortedUniq(redIDs)
	proj = impactStepProjectionOf(root, redIDs, true) // 按需档：真源 `--list` 现跑 + 逐名探针
	if proj.Status != "取值" {
		return nil, ids, redIDs, proj, fmt.Sprintf("门步投影%s（%s）⇒ 预测侧**取不到**（宁少报不猜报）",
			proj.Status, proj.Reason)
	}
	return impactStepSortedUniq(proj.Steps), ids, redIDs, proj, ""
}

// ---- ④ 三个数（**纯函数**：同一份输入 ⇒ 三数逐字相同 ⇒ 判据① 的「数字可复算」）----------------

// impactBackfillCounts 三个整数（§5.2 字头不改：命中 / 漏报 / 虚报）。
type impactBackfillCounts struct {
	Hit        int
	Miss       int
	FalseAlarm int
}

// impactBackfillExcl 预测了、但那一档**不给结论 / 只报告** ⇒ 不进三数的步（逐条留名，免得被当成漏算）。
type impactBackfillExcl struct {
	Blocked []string
	Report  []string
}

// impactBackfillCountsOf 三数 + 排除面（**唯一判定口** · 抽出来是为了让成对负控能直接喂坏输入）。
//
//	命中  = 预测红 ∩ 真红（`FAIL`）
//	漏报  = 真红 \ 预测红        （`BLOCKED` **永远不在**真红那一侧 ⇒ 判据② 由构造保证 ✓）
//	虚报  = 预测红 \（真红 ∪ `BLOCKED` ∪ `REPORT`）—— 只有**判绿**（`PASS`）才是虚报的反证；
//	        `BLOCKED`（不给结论）与 `REPORT`（只报告）都**不是「判绿」** ⇒ 不计虚报，逐条单列。
func impactBackfillCountsOf(predict, fail, blocked, report []string) (impactBackfillCounts, impactBackfillExcl) {
	c := impactBackfillCounts{}
	ex := impactBackfillExcl{}
	inFail, inBlocked, inReport := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, s := range fail {
		inFail[s] = true
	}
	for _, s := range blocked {
		inBlocked[s] = true
	}
	for _, s := range report {
		inReport[s] = true
	}
	for _, s := range impactStepSortedUniq(predict) {
		switch {
		case inFail[s]:
			c.Hit++
		case inBlocked[s]:
			ex.Blocked = append(ex.Blocked, s)
		case inReport[s]:
			ex.Report = append(ex.Report, s)
		default:
			c.FalseAlarm++
		}
	}
	inPredict := map[string]bool{}
	for _, s := range predict {
		inPredict[s] = true
	}
	for _, s := range impactStepSortedUniq(fail) {
		if !inPredict[s] {
			c.Miss++
		}
	}
	return c, ex
}

// ---- ⑤ 回填件的一行（§5.5 十键**逐字** + 附键）---------------------------------------------

// impactActualPayload —— §5.3 逐字那一枚的形状：`{head_sha, 那次门禁的标识, 命中, 漏报, 虚报, blocked 步名[]}`。
//
// ★ 这一枚就是「**审计行第二枚字段（`impact_actual`）**」的**取值**（同一份取值落进回填件那一行 ——
// 见本文件头注「落点那一格的缺口」：审计行在**真写那一刻**落盘、那时门禁还没跑 ⇒ 不许事后回改历史行 ✗）。
type impactActualPayload struct {
	HeadSHA      string   `json:"head_sha"`
	GateRunID    string   `json:"gate_run_id"`
	Hit          int      `json:"hit"`
	Miss         int      `json:"miss"`
	FalseAlarm   int      `json:"false_alarm"`
	BlockedSteps []string `json:"blocked_steps"`
}

// impactBackfillRecord —— 回填件的一行。前十条 = §5.5 点名的键（**缺任一 ⇒ 该行不许写**）；
// 其后是**附键**：只记「这一枚是怎么来的」，**一格都不参与三数**（三数的口径在 `impactBackfillCountsOf`）。
type impactBackfillRecord struct {
	HeadSHA       string   `json:"head_sha"`
	Target        string   `json:"target"`
	PredictRed    []string `json:"predict_red"`
	ActualFail    []string `json:"actual_fail"`
	ActualBlocked []string `json:"actual_blocked"`
	Hit           int      `json:"hit"`
	Miss          int      `json:"miss"`
	FalseAlarm    int      `json:"false_alarm"`
	GateRunID     string   `json:"gate_run_id"`
	At            string   `json:"at"`

	Signal       string `json:"signal"`
	GateResults  string `json:"gate_results"`
	GateTableSHA string `json:"gate_results_sha256"`
	GateTier     string `json:"gate_tier"`
	StepsTotal   int    `json:"steps_total"`

	ActualReport []string `json:"actual_report"`
	PredictIDs   []string `json:"predict_contract_ids"`
	PredictExclB []string `json:"predict_excluded_blocked"`
	PredictExclR []string `json:"predict_excluded_report"`
	RateSlot     string   `json:"rate_slot"`
	BackfillPath string   `json:"backfill_path"`

	ImpactActual impactActualPayload `json:"impact_actual"`
}

// impactBackfillValidate 判据④的判定口：十键**逐格判**，缺哪一格就点名哪一格（**缺任一 ⇒ 不许写**）。
// `hit` / `miss` / `false_alarm` 是 `int` ⇒ 结构上恒在；这里只判「不是负数」（负数说明算错了）。
func impactBackfillValidate(rec impactBackfillRecord) (missing []string, ok bool) {
	if strings.TrimSpace(rec.HeadSHA) == "" {
		missing = append(missing, "head_sha")
	}
	if strings.TrimSpace(rec.Target) == "" {
		missing = append(missing, "target")
	}
	if rec.PredictRed == nil {
		missing = append(missing, "predict_red")
	}
	if rec.ActualFail == nil {
		missing = append(missing, "actual_fail")
	}
	if rec.ActualBlocked == nil {
		missing = append(missing, "actual_blocked")
	}
	if strings.TrimSpace(rec.GateRunID) == "" {
		missing = append(missing, "gate_run_id")
	}
	if strings.TrimSpace(rec.At) == "" {
		missing = append(missing, "at")
	}
	if rec.Hit < 0 || rec.Miss < 0 || rec.FalseAlarm < 0 {
		missing = append(missing, "hit/miss/false_alarm（不许负数 —— 负数是算错的自证）")
	}
	return missing, len(missing) == 0
}

// impactBackfillPath 回填件的落点：`<状态目录>/<契约件里的 file>`（**不在仓内**）。
func impactBackfillPath(c impactBackfillContract) string {
	if strings.TrimSpace(c.File) == "" {
		return ""
	}
	return filepath.Join(stateDirOf(), c.File)
}

// impactBackfillAppendIfComplete 判据④的**唯一写口**：十键缺任一 ⇒ **一个字节都不写**（返回缺哪几格）。
// 追加只写（`O_APPEND|O_CREATE` · 一行一枚），**从不改历史行**。
func impactBackfillAppendIfComplete(path string, rec impactBackfillRecord) (missing []string, err error) {
	missing, ok := impactBackfillValidate(rec)
	if !ok {
		return missing, nil
	}
	if strings.TrimSpace(path) == "" {
		return []string{"落点（契约件里 `file` 空）"}, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	body, err := json.Marshal(rec)
	if err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if _, err := f.Write(append(body, '\n')); err != nil {
		return nil, err
	}
	return nil, f.Sync()
}

// impactBackfillHistory 回填件的**只读**汇总（随仓累积：攒假阳率 / 假阴率那一格的全部输入）。
// ★ 这一枚**只进本块的报表** —— 它不是预测 / 排序 / 裁条 / 退码的输入（冲突⑧ 的静态一半由
// `impactBackfillReadersScan` 守，运行期那一半在 `cli_impact_backfill_test.go` 里真跑）。
type impactBackfillHistory struct {
	Path                  string
	Status                string // 取值 / 空（还没攒过）/ 读不到
	Reason                string
	Lines                 int
	Broken                int
	Hit, Miss, FalseAlarm int
}

func impactBackfillHistoryOf(path string) impactBackfillHistory {
	h := impactBackfillHistory{Path: path}
	if strings.TrimSpace(path) == "" {
		h.Status, h.Reason = "读不到", "落点解析不出来（状态目录取不到）"
		return h
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			h.Status = "空"
			h.Reason = "还没攒过（**「还没攒」不是「读不到」**）"
			return h
		}
		h.Status, h.Reason = "读不到", err.Error()
		return h
	}
	for _, ln := range strings.Split(strings.TrimRight(string(b), "\n"), "\n") {
		if strings.TrimSpace(ln) == "" {
			continue
		}
		var row struct {
			Hit        int `json:"hit"`
			Miss       int `json:"miss"`
			FalseAlarm int `json:"false_alarm"`
		}
		if err := json.Unmarshal([]byte(ln), &row); err != nil {
			h.Broken++
			continue
		}
		h.Lines++
		h.Hit += row.Hit
		h.Miss += row.Miss
		h.FalseAlarm += row.FalseAlarm
	}
	h.Status = "取值"
	if h.Broken > 0 {
		h.Reason = fmt.Sprintf("有 %d 行解不开（**只报、不改** —— 历史行不许动 ✗）", h.Broken)
	}
	return h
}

// ---- ⑥ 红线自证（静态那一半）：回填件**只被读**，不作任何算法的输入 ---------------------------

// impactBackfillNoReadSources —— 预测 / 排序 / 裁条 / 退码那几条路径的源件（本件**逐个现读**）。
// 口径：这些件里对**回填件名**与**那两个碰件的函数名**的引用必须 **0 处**
// （本件自己不算 —— 它是唯一的写者与读历史者；读不到某一件 ⇒ 这一半**不给结论**，**不当 0 处**）。
var impactBackfillNoReadSources = []string{
	"family_impact.go",          // 退码与总接线
	"family_impact_layers.go",   // 六层取数
	"family_impact_steps.go",    // 步名真源与 join 投影（预测侧）
	"family_impact_timeface.go", // 时间面预测侧
	"family_impact_card.go",     // 卡片与四级排序
	"family_impact_cache.go",    // 落盘缓存与毫秒档
	"family_impact_contract.go", // 契约面
}

// impactBackfillScan 静态自证的结果（三态：✓ 0 处 / ✗ 越线 / 不给结论）。
type impactBackfillScan struct {
	Files  int
	Hits   []string // 越线：逐条
	Unread []string // 读不到的源件（这一半不给结论）
}

func (s impactBackfillScan) OK() bool { return len(s.Hits) == 0 && len(s.Unread) == 0 }

// impactBackfillScanPatterns —— 数哪几样：**回填件名** + 那两个**碰件的函数名**（写口与读口）。
func impactBackfillScanPatterns(file string) []string {
	return []string{file, "impactBackfillAppendIfComplete", "impactBackfillHistoryOf"}
}

// impactBackfillReadersScan 现读那些源件，数引用处数（**只读**）。
func impactBackfillReadersScan(root, file string) impactBackfillScan {
	s := impactBackfillScan{Files: len(impactBackfillNoReadSources)}
	for _, rel := range impactBackfillNoReadSources {
		p := filepath.Join(root, "core", "cmd", "zerg", rel)
		b, err := os.ReadFile(p)
		if err != nil {
			s.Unread = append(s.Unread, fmt.Sprintf("%s（**读不到** ⇒ 这一格不给结论，不是「0 处」）", rel))
			continue
		}
		src := string(b)
		for _, pat := range impactBackfillScanPatterns(file) {
			if strings.TrimSpace(pat) == "" {
				continue
			}
			if n := strings.Count(src, pat); n > 0 {
				s.Hits = append(s.Hits, fmt.Sprintf("%s：`%s` %d 处", rel, pat, n))
			}
		}
	}
	return s
}

// ---- ⑦ 出口：`B3` 的收口块（stderr · 人读 + 一枚机读行）------------------------------------

// impactBackfillSignal —— 机读行的形状（**定序 · 只出文本、不落盘** —— 落盘的那一枚是回填件那一行）。
type impactBackfillSignal struct {
	Signal        string              `json:"signal"`
	Target        string              `json:"target"`
	HeadSHA       string              `json:"head_sha"`
	GateResults   string              `json:"gate_results"`
	GateRunID     string              `json:"gate_run_id"`
	GateTier      string              `json:"gate_tier"`
	StepsTotal    int                 `json:"steps_total"`
	PredictRed    []string            `json:"predict_red"`
	PredictIDs    []string            `json:"predict_contract_ids"`
	ActualFail    []string            `json:"actual_fail"`
	ActualBlocked []string            `json:"actual_blocked"`
	ActualReport  []string            `json:"actual_report"`
	Hit           int                 `json:"hit"`
	Miss          int                 `json:"miss"`
	FalseAlarm    int                 `json:"false_alarm"`
	HitMissDen    int                 `json:"hit_miss_den"`
	PassDen       int                 `json:"pass_den"`
	RateSlot      string              `json:"rate_slot"`
	Wrote         bool                `json:"wrote"`
	BackfillPath  string              `json:"backfill_path"`
	HistoryLines  int                 `json:"history_lines"`
	ReadOnlyOK    bool                `json:"read_only_proof_ok"`
	ReadOnlyHits  []string            `json:"read_only_proof_hits"`
	NotInputTo    []string            `json:"not_input_to"`
	ImpactActual  impactActualPayload `json:"impact_actual"`
}

// emitImpactBackfillBlock —— `B3` 的收口块（**只在给了 `--gate-results` 时被调用**）。
//
// 三档出口（**都不改既有判据**）：
//
//	0 = 对拍跑通（干跑或已追加；**写失败也只到这一档** —— 判据「写回填失败不影响主流程退码」✓）
//	8 = **取不到**（契约件读不到 / 结果表读不到或不齐 / 真源现跑取不到 / 预测侧取不到）⇒ 不给结论
func emitImpactBackfillBlock(w io.Writer, inv *invocation, root string, tgt *impactTarget) int {
	fmt.Fprintf(w, "%s: `B3` 实测回填闭环（**预测 vs 实际** —— 设计-变更影响面-v1.6 §5.2/§5.3/§5.5 · "+
		"任务单-影响面实施-20260922 §三 `B3`）· ⚠ 冲突⑧ 裁定：**回填件只被读，不作任何算法的输入**\n", progName)
	fmt.Fprintf(w, "  触发点（`R57` 三候选择一）= **① 门禁跑完的收口** —— 本块就是那次门禁的收口动作；"+
		"`gate` 族透传面一个字不动（`gate.go` 铁律「只转发、不翻译」）\n")

	// ① 契约真源
	c, why := impactBackfillContractOf(root)
	if why != "" {
		inv.setErr("blocked", "backfill_contract_unreadable", why)
		fmt.Fprintf(w, "  ★ **不给结论（退码 8）**：%s ⇒ 十键判据与落点都**不猜**（「读不到」不当「没有要求」）\n", why)
		return exitBlocked
	}
	bfPath := impactBackfillPath(c)

	// ② 那次门禁的结果表（只读）
	run := impactGateResultsRead(root, strings.TrimSpace(inv.flagVal("--gate-results")))
	if run.Status != "取值" {
		inv.setErr("blocked", "gate_results_unreadable", run.Reason)
		fmt.Fprintf(w, "  ★ **不给结论（退码 8）**：那次门禁的结果表**取不到** —— %s\n", run.Reason)
		fmt.Fprintf(w, "    （**失败 ≠ 事实为零**：这一格今天就是「不知道」，**不许**读成「一条都没红」）\n")
		return exitBlocked
	}

	// ③ 真源现跑（两个档）+ 齐不齐
	truth := impactBackfillTruthPull(root)
	tier, why := impactBackfillAlign(run, truth)
	if why != "" {
		inv.setErr("blocked", "gate_results_incomplete", why)
		fmt.Fprintf(w, "  ★ **不给结论（退码 8）**：%s\n", why)
		return exitBlocked
	}

	// ④ 预测侧（闭集 = 门步名）
	steps, ids, _, proj, why := impactBackfillPredictOf(root, tgt)
	if why != "" {
		inv.setErr("blocked", "predict_side_unavailable", why)
		fmt.Fprintf(w, "  ★ **不给结论（退码 8）**：%s\n", why)
		return exitBlocked
	}

	// ⑤ 三个数（纯函数）+ 记录
	counts, excl := impactBackfillCountsOf(steps, run.Fail, run.Blocked, run.Report)
	head := impactHeadSHA(root)
	rec := impactBackfillRecord{
		HeadSHA:       head,
		Target:        tgt.Raw,
		PredictRed:    append([]string{}, steps...),
		ActualFail:    append([]string{}, run.Fail...),
		ActualBlocked: append([]string{}, run.Blocked...),
		Hit:           counts.Hit,
		Miss:          counts.Miss,
		FalseAlarm:    counts.FalseAlarm,
		GateRunID:     run.RunID,
		At:            impactEffectiveAt(),

		Signal:       "impact.backfill",
		GateResults:  run.Path,
		GateTableSHA: run.TableSHA,
		GateTier:     tier,
		StepsTotal:   len(run.Rows),

		ActualReport: append([]string{}, run.Report...),
		PredictIDs:   append([]string{}, ids...),
		PredictExclB: append([]string{}, excl.Blocked...),
		PredictExclR: append([]string{}, excl.Report...),
		RateSlot:     fmt.Sprintf("%s 第 %d 行（%s）", c.Slot.File, c.Slot.Line, c.Slot.Threshold),
		BackfillPath: bfPath,
		ImpactActual: impactActualPayload{
			HeadSHA: head, GateRunID: run.RunID,
			Hit: counts.Hit, Miss: counts.Miss, FalseAlarm: counts.FalseAlarm,
			BlockedSteps: append([]string{}, run.Blocked...),
		},
	}

	// ⑥ 干跑 vs 真写（**干跑一个字节都不落** · 写失败不影响退码）
	missing, complete := impactBackfillValidate(rec)
	realWrite := !inv.dryRun && inv.confirmGiven && inv.yes && inv.confirm == planHost()
	wrote, writeNote := false, ""
	if realWrite {
		switch {
		case !complete:
			writeNote = "**该行不许写**（判据④：缺 " + strings.Join(missing, " / ") + "）"
		default:
			m2, err := impactBackfillAppendIfComplete(bfPath, rec)
			switch {
			case err != nil:
				// ★ 判据「写回填失败不得影响主流程退码」：只点名，**不改退码**。
				writeNote = fmt.Sprintf("**追加失败**（%v）—— 只点名、**不改退码**（回填是附加动作）", err)
			case len(m2) > 0:
				missing = m2
				writeNote = "**该行不许写**（判据④：缺 " + strings.Join(m2, " / ") + "）"
			default:
				wrote = true
			}
		}
	} else if inv.dryRun {
		writeNote = "干跑（`--dry-run`）⇒ **一个字节都不落**"
	} else {
		writeNote = fmt.Sprintf("干跑（未给确认档 ⇒ 要 `--confirm=%s --yes` 才写）⇒ **一个字节都不落**", planHost())
	}

	// ⑦ 打印（人读）
	hist := impactBackfillHistoryOf(bfPath)
	fmt.Fprintf(w, "  本跑：目标=%s · head_sha=%s · 档位=%s（**真源 `--list` 现跑**：全量 %d 步 / 快速 %d 步 ⇒ "+
		"对齐「%s」）· 回填时刻=%s · 本块耗时=%s\n",
		tgt.Raw, dashIfEmpty(rec.HeadSHA), tier, truth.FullTotal, truth.FastTotal, tier,
		rec.At, proj.Cost.Round(time.Millisecond).String())
	fmt.Fprintf(w, "  ① 那次门禁（**只读**）：结果表 %s（目录 %s）· %d 行 · 结果表 sha256=%s · `gate_run_id`=%s\n",
		run.Path, run.Dir, len(run.Rows), run.TableSHA, run.RunID)
	fmt.Fprintf(w, "     FAIL（**真红**）%d 条：%s\n", len(run.Fail), orDashList(run.Fail))
	fmt.Fprintf(w, "     BLOCKED（**不给结论** —— 单列、**不进三数**）%d 条：%s\n", len(run.Blocked), orDashList(run.Blocked))
	fmt.Fprintf(w, "     REPORT（只报告 —— 同族单列）%d 条：%s\n", len(run.Report), orDashList(run.Report))
	fmt.Fprintf(w, "  ② 预测集 `predict_red[]`（**闭集 = 门步名** · 经 `B1` 的 join 投影 · 真源 `--list` 现跑）：%d 条：%s\n",
		len(steps), orDashList(steps))
	fmt.Fprintf(w, "     契约 id（**不进对拍集** —— 不是步名）：%s\n", orDashList(ids))
	fmt.Fprintf(w, "     未接步（契约 id 的 `gate` 里没有门脚本路径 ⇒ 那部分**预测不了** —— 漏报会照实出现）：%d 条\n",
		len(proj.Unjoined))
	fmt.Fprintf(w, "  ③ 三个数（§5.2 字头不改）：**命中(hit)=%d** · **漏报(miss)=%d** · **虚报(false_alarm)=%d**\n",
		counts.Hit, counts.Miss, counts.FalseAlarm)
	fmt.Fprintf(w, "     口径：命中=真阳（预测红 ∩ 真红）· 漏报=假阴（**这一格最贵**）· 虚报=假阳（预测红 ∩ 判绿）；"+
		"`BLOCKED`/`REPORT` 逐条单列、**不进三数**（判据②：`BLOCKED` 不计入漏报 ✓）\n")
	fmt.Fprintf(w, "  ④ 齐不齐：结果表 %d 行 ⇔ 真源现跑「%s」**逐字相等** ⇒ 齐（**不许拿子集当全量**；"+
		"对不上就退 8 不给结论）\n", len(run.Rows), tier)
	fmt.Fprintf(w, "  ⑤ 参照（**现成槽 · 本件不自设阈值** ✗）：%s —— 漏报同格是**绝对数**（现成槽那一条的左半）、"+
		"虚报同格是**费率**（右半 · 分母 = 判绿那一侧）；**本件只并排报，不判红绿**（`R30`：口径可定、阈值不可编）\n", rec.RateSlot)
	fmt.Fprintf(w, "     本件自攒（**只报**）：漏报 %d 条（真红 %d 条为分母）· 虚报 %d 条（判绿 %d 条为分母）\n",
		counts.Miss, counts.Hit+counts.Miss, counts.FalseAlarm, len(run.Pass))
	switch hist.Status {
	case "取值":
		if hist.Broken > 0 {
			fmt.Fprintf(w, "  ⑥ 累积（回填件**只读**那一半）：%s —— 已攒 %d 行（**解不开 %d 行**：只报、不改 ✗）· "+
				"累计 命中=%d 漏报=%d 虚报=%d\n", bfPath, hist.Lines, hist.Broken,
				hist.Hit, hist.Miss, hist.FalseAlarm)
		} else {
			fmt.Fprintf(w, "  ⑥ 累积（回填件**只读**那一半）：%s —— 已攒 %d 行 · "+
				"累计 命中=%d 漏报=%d 虚报=%d\n", bfPath, hist.Lines,
				hist.Hit, hist.Miss, hist.FalseAlarm)
		}
	case "空":
		fmt.Fprintf(w, "  ⑥ 累积（回填件**只读**那一半）：%s —— %s\n", bfPath, hist.Reason)
	default:
		fmt.Fprintf(w, "  ⑥ 累积（回填件**只读**那一半）：%s —— **读不到**：%s（「读不到」不当「还没攒」）\n", bfPath, hist.Reason)
	}
	if wrote {
		fmt.Fprintf(w, "  ⑦ 回填件：**已追加一行**（第 %d 行）→ %s（**追加式** · 历史行一个字都不改）\n", hist.Lines+1, bfPath)
	} else {
		fmt.Fprintf(w, "  ⑦ 回填件：**未写** —— %s（落点：%s）\n", writeNote, bfPath)
	}
	fmt.Fprintf(w, "  ⑧ 判据④（十键）：%s\n", impactBackfillKeysLine(c, missing))

	// ⑨ 红线自证：静态那一半（运行期那一半在 go test 里真跑）
	sc := impactBackfillReadersScan(root, c.File)
	switch {
	case sc.OK():
		fmt.Fprintf(w, "  ⑨ ★ **回填件只被读、不作任何算法的输入**（冲突⑧）：静态一半 —— 预测/排序/裁条/退码四条路径的 "+
			"%d 件源件里，对回填件名与那两个碰件的函数名的引用 **0 处** ✓（运行期那一半：改回填值 ⇒ 预测/排序/退码"+
			"逐字不变 · go test 真跑）\n", sc.Files)
	case len(sc.Unread) > 0 && len(sc.Hits) == 0:
		fmt.Fprintf(w, "  ⑨ ★ 静态一半**不给结论**：%d 件源件里有个读不到（%s）⇒ **不当 0 处**（「读不到」不许读成「干净」）\n",
			len(sc.Unread), strings.Join(sc.Unread, " · "))
	default:
		fmt.Fprintf(w, "  ⑨ ★✗ **越线**：静态一半查到引用 —— %s\n", strings.Join(sc.Hits, " · "))
	}

	// ⑩ 机读行
	sig := impactBackfillSignal{
		Signal: "impact.backfill", Target: rec.Target, HeadSHA: rec.HeadSHA,
		GateResults: rec.GateResults, GateRunID: rec.GateRunID, GateTier: rec.GateTier,
		StepsTotal: rec.StepsTotal, PredictRed: rec.PredictRed, PredictIDs: rec.PredictIDs,
		ActualFail: rec.ActualFail, ActualBlocked: rec.ActualBlocked, ActualReport: rec.ActualReport,
		Hit: rec.Hit, Miss: rec.Miss, FalseAlarm: rec.FalseAlarm,
		HitMissDen: rec.Hit + rec.Miss, PassDen: len(run.Pass), RateSlot: rec.RateSlot,
		Wrote: wrote, BackfillPath: bfPath, HistoryLines: hist.Lines,
		ReadOnlyOK: sc.OK(), ReadOnlyHits: append(append([]string{}, sc.Hits...), sc.Unread...),
		NotInputTo:   append([]string{}, c.NotInputTo...),
		ImpactActual: rec.ImpactActual,
	}
	if body, err := json.Marshal(sig); err != nil {
		fmt.Fprintf(w, "  机读行拼不出来：%v\n", err)
	} else {
		fmt.Fprintf(w, "  机读（stderr 一行 · 定序 · 不落盘 —— 落盘的那一枚是回填件那一行）：%s\n", string(body))
	}
	fmt.Fprintf(w, "  ★ 本块**不改任何既有判据**：不设阈值 ✗ · 不进门禁 ✗ · 不自动改「会红」的算法 ✗ · "+
		"不把 `BLOCKED` 混进漏报 ✗ · 不回改已发出的卡片 ✗ · 不改历史行 ✗；六键包封**一个键都不加** ✗"+
		"（缺口照实点名：`warnings[]`/`truncated` 今天没有落点 ⇒ 同一份取值走 stderr）\n")
	return exitOK
}

// impactBackfillKeysLine 十键那一行的自证文本（缺的逐格点名）。
func impactBackfillKeysLine(c impactBackfillContract, missing []string) string {
	if len(missing) == 0 {
		return fmt.Sprintf("**%d 键齐**（%s）⇒ 该行可写", len(c.RequiredKeys), strings.Join(c.RequiredKeys, " / "))
	}
	return fmt.Sprintf("**缺 %d 格**（%s）⇒ **该行不许写**", len(missing), strings.Join(missing, " / "))
}
