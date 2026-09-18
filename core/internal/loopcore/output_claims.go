// output_claims.go — B 项⑥（B6）：把**已造好却从未接线**的引用命中校验接到**成员产出路径**上。
//
// 接线前的实测（本轮现读取证，命令与原始输出见回报「前提自证」）：
//
//	grep -rn 'VerifyClaims(' core --include=*.go | grep -v _test | grep -v 'func VerifyClaims'
//	⇒ 0 行（`VerifyClaims` 只在 receipt.go:68 定义、只在测试里被调用；退出码 1）
//
// 也就是说：四态裁决算得出来，却**从来没有任何产出路径问过它** —— 本文件就是那一步接线。
//
// ── 成员与产出路径是谁（逐字有据，不猜）──
//
//	· 成员＝**CA 子进程**（《设计-协作骨架 v2.1》§4.1-3；《任务表-协作骨架v2.1仓内核对结论》表#13
//	  逐字：「`core/internal/api/master_scheduler.go:60`（`cmd *exec.Cmd`＝运行中的 CA 进程）→ 成员＝CA 子进程」）；
//	· CA 的执行循环**就是本包 `Run`**（`core/internal/agent/ca_loop.go:215` RunWithKernel ⇒ `loopcore.Run`）；
//	· ⇒ **本包 `Run` 的产出口就是成员产出路径**：接线接在那里，不另开旁路。
//
// 产出口有**多个**（自然收尾 `return res` ×2 / 轮数用尽 / 墙钟 / 坏格式 / 守卫升级）⇒ 逐出口改必漏，
// 故接线用 run.go 顶部那个 defer 收口（与同文件 T1.3 观测 defer 同一纪律）。
//
// ── 判定口径（只写**能从仓内代码/设计稿逐字确认**的部分）──
//
//	① 裁决一律走既有 `VerifyClaims`（receipt.go:68，**一行不改**）：
//	   keep=有据 · revise=缺引用 · retract=假回执 · send_back=片段对不上（无据）。
//	② 「命中失败」= **{retract, send_back}**（闭集）：
//	   · receipt.go 判据注释逐字：「引用指向不存在的回执 ⇒ retract（假回执）」、
//	     「回执存在但片段对不上 ⇒ send_back（无据）」（设计稿把无据判为**永久错误 ⇒ 不重试**）；
//	   · revise（缺引用）**不算**命中失败：同源注释把它单列为「缺引用：要求补（不是无据）」，
//	     设计稿 v1.2.2 §第1步 也只把「命不中」判无据 ⇒ 两回事，不许混。
//	③ 回执集合＝**本轮真实工具轨迹**（`Trace.CallID`＝回执ID · `Trace.Result`＝回执原文）。
//	   口径：**只收有 ID 的轨迹**；`Result` 为空也**照收**——空 Result 是真实事实（那次调用没有原文），
//	   若把它剔掉，「无据（send_back）」会被误判成「假回执（retract）」，而两者处置不同
//	   （retract=撤回该主张 / send_back=拆小步）。
//	④ 命中失败 ⇒ 该产出**不得当自然完成交付** ⇒ 交**既有四终局**（`ChooseTerminal`，**一行不改**）。
//	   `ApplyOutputClaimFacts` 只往**既有的两个事实位**写事实，落哪一终局仍由既有优先级决定：
//	   本文件**不新建裁决机制、不新造状态名、不替 ChooseTerminal 排序**。
//	⑤ 说明一律**低基数**（条数 + 四态名），不回显主张/片段原文 —— 那可能夹带请求体原文、凭据或私有路径
//	    （与 sendback_code.go 的观测出口同一条纪律；设计稿 §第1步：「只标记、不搬运」）。
//
// ── 不确定项（**没有自己发明**，逐条待父代理定；见回报）──
//
//	· **引用格式谁定**：设计稿 v1.2.2 §第1步/§Q5 说引用块 `{主张, 依据=回执ID, 片段}` 由
//	  **受控解码（grammar/json_schema）/ 合成工具**保证格式；该载体**尚未落地**（现网产出是自由文本）
//	  ⇒ 本批**不从自由文本里发明一套格式**（不写正则、不猜标记）：引用事实由调用方经
//	  `Deps.OutputClaims` 显式给出 —— 有则判、无则判定面与接线前**逐字节一致**。
//	· **「产出无引用」怎么算**：`Deps.OutputClaims` 空 = 本轮**没有引用载体**（现网全部如此）⇒ 不判、不改行为。
//	  B6 验收判据原文是「产出无引用**必走四终局**」，但那要求「有载体而零条引用」与「根本没有载体」
//	  二者可区分 —— 当前两者都表现为空切片 ⇒ 该口径取决于载体落地形态，本批**不自行裁定**（登记为待定）。
package loopcore

import (
	"fmt"
	"strings"
)

// MemberReceipts — 把本轮工具轨迹变成**回执集**（回执ID = `Trace.CallID`；原文 = `Trace.Result`）。
//
// 只收有 ID 的轨迹：没有 ID 的回执**引用不到**（沿用铁律「无 id 的委派一律拒绝」同一纪律）。
// `Result` 为空**照收**（见文件头 ③：剔掉会把「无据」误判成「假回执」）。
func MemberReceipts(traces []Trace) []Receipt {
	out := make([]Receipt, 0, len(traces))
	for _, t := range traces {
		id := strings.TrimSpace(t.CallID)
		if id == "" {
			continue
		}
		out = append(out, Receipt{ID: id, Tool: t.Name, Output: t.Result})
	}
	return out
}

// OutputClaimCheck — 成员产出的引用命中校验结果（产出口的**唯一事实载体**，纯数据）。
type OutputClaimCheck struct {
	// Declared — 调用方是否给出引用载体（false ⇒ 本产出未声明引用 ⇒ 不判、不改行为）。
	Declared bool
	// Results — 逐条裁决（`VerifyClaims` 的产物，顺序与入参一致）。
	Results []ClaimVerdictResult
	// Failed — **命中失败**子集（闭集：{retract, send_back}）。
	Failed []ClaimVerdictResult
}

// HasFailure — 是否有命中失败（假回执 / 无据）。
func (c OutputClaimCheck) HasFailure() bool { return len(c.Failed) > 0 }

// VerifyOutputClaims — 对成员产出的引用跑**既有**四态裁决（`VerifyClaims`，一行不改）。
//
// 无引用载体（`claims` 空）⇒ `Declared=false` 且**不产生任何裁决** ⇒ 调用方行为与接线前一致（零回归）。
func VerifyOutputClaims(claims []Claim, traces []Trace) OutputClaimCheck {
	chk := OutputClaimCheck{Declared: len(claims) > 0}
	if !chk.Declared {
		return chk
	}
	chk.Results = VerifyClaims(claims, MemberReceipts(traces))
	for _, r := range chk.Results {
		switch r.Verdict {
		case VerdictRetract, VerdictSendBack:
			chk.Failed = append(chk.Failed, r)
		}
	}
	return chk
}

// ApplyOutputClaimFacts — 命中失败 ⇒ 把事实写进**既有事实位**（`TerminalInput` 不加字段）。
//
// 两条事实都能从产出本身确认（不猜）：
//
//	· `HasRemainingWork=true`：无据那部分＝**未覆盖部分** ⇒ 仍有「可独立完成」的余下工作。
//	  与设计稿 v1.2.2 §第1步「无据 ⇒ 不重试 ⇒ 走**拆小步**」、§5.4-4「宣布完成却无可命中引用 ⇒ 判无据
//	  ⇒ **降级/拆小步**」**同向**；也与既有 `degrade` 的口径逐字相合（「做完能做的，如实标注未覆盖部分」）。
//	· `NeedsHumanJudgment=true`：产出无据 ⇒ 不能由执行者自决为「完成」。
//
// 除这两个既有字段外**不动任何东西**：落哪一终局仍由既有 `ChooseTerminal` 按既有优先级决定。
// 返回 (改后事实, 是否命中失败)；无命中失败 ⇒ 原样返回（调用方零回归）。
func ApplyOutputClaimFacts(chk OutputClaimCheck, in TerminalInput) (TerminalInput, bool) {
	if !chk.HasFailure() {
		return in, false
	}
	in.HasRemainingWork = true
	in.NeedsHumanJudgment = true
	return in, true
}

// ClaimFailureDetail — 命中失败的**可行动说明**（低基数：只报条数与四态名）。
//
// **不回显**主张原文与片段原文：那是成员产出的自由文本，可能夹带请求体原文、凭据或私有路径
// （与 sendback_code.go 的观测出口同一条纪律）。四态名与条数已足够让读日志的人与统计脚本对齐。
func ClaimFailureDetail(chk OutputClaimCheck) string {
	if !chk.HasFailure() {
		return ""
	}
	by := CountByVerdict(chk.Failed) // 既有汇总函数（receipt.go），不另造计数口径
	return fmt.Sprintf("产出引用命中失败 %d 条（假回执 retract %d · 无据 send_back %d）；裁决共 %d 条 ⇒ 不得当完成交付，按拆小步重交",
		len(chk.Failed), by[string(VerdictRetract)], by[string(VerdictSendBack)], len(chk.Results))
}

// AppendTerminalNote — 把终局说明附到产出正文（既有两处出口共用同一形态，免得两处写法漂移）。
//
// 与 run.go 里原有写法**逐字节等价**：正文为空 ⇒ 说明即正文；否则空行分隔后追加。
func AppendTerminalNote(content, note string) string {
	if content == "" {
		return note
	}
	return content + "\n\n" + note
}
