// chat_tool_gate.go —— 对话层的**工具门接电**（§二十一 已红第 9 条的同一种病 · 2026-09-21 补）。
//
// 病征（《缺口-命令面-20260921.md》§十.3 第 3 条 · **实据不是推论**）：`delete_file` 与
// `download` **一个闸门都没有** —— 现跑全仓 `gate.Check("…")` 只有 10 个名字，二者不在其中；
// 它们在 `chat_tool_extra.go:95` / `:327` 的调用点**直接执行**（`res(deleteFile(...))` /
// `res(downloadFile(...))`）。默认档 `block` 对它们**一点用都没有**：默认档只管
// 「有人来问门、而这个名字没登记」那条路径 —— 而它们**根本不来问门**。
// 「规则写了、电没接」，这就是那条已红在对话层的形态。
//
// 本件的口径（与 `core/internal/control/agentgate.go` 逐字同口径，不自造第四种语义）：
//
//	① **三态逐字搬**：`allow` 才放行；`block` 拒；`require_approval` **无审批即不执行**
//	   （与 CA 的 `bash` 侧、网关侧同一句话 —— 绝不当放行）；
//	② **装不上就是拦**：规则表装载失败 ⇒ 恒拦门（fail-closed），不许静默退化成全放行；
//	③ **不硬禁、不删能力**：判据在**表里**（`rules.yaml`），代码只负责问门。要放行走下一条
//	   逃生门（人签的批准件），或由人改表（模型改不动表）。
//
// 逃生门（**人**能开、模型开不了）：`<状态目录>/approvals/<工具名>.json`
//
//	{"approver":"Mr2109","approved_at":"2026-09-21T…","scope":"*","note":"理由"}
//
// `scope` 取 `"*"` 或**参数串的子串**（把批准压到具体那一个路径 / URL 上）。
// 为什么是「人签的一枚件」而不是命令行旗标、也不是参数里的 `approved=true`：工具调用的参数
// **来自模型** —— 模型自己写得出的「批准」等于没有批准（不可伪造性 = 唯一的判据）。
package chat

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Mr2109/zerg-swarm/core/internal/agent"
	"github.com/Mr2109/zerg-swarm/core/internal/control"
	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
)

// gateAgentName —— 对话层问门时报的身份（`rules.yaml` 里带 `scope:` 的条目按它过滤）。
const gateAgentName = "chat"

var (
	chatGateMu     sync.Mutex
	chatGateLoaded bool
	chatGateInst   *control.AgentGate
)

// chatToolGate —— 对话层的门（**只装载一次** · 真源 = `<仓根>/core/internal/control/rules.yaml`，
// 与网关、CA 读的是**同一份文件** ⇒ 三处一张表，不另立第二张）。
//
// 装载失败的处置：打一条显式告警（**不静默**）并照旧接上一个**恒拦**门 ——
// 「读不到规则表就变成全放行」正是那条已红的形态，绝不重演。
func chatToolGate() *control.AgentGate {
	chatGateMu.Lock()
	defer chatGateMu.Unlock()
	if !chatGateLoaded {
		g, err := control.NewAgentGateFromRepo(statepath.WorkspaceRoot())
		if err != nil {
			fmt.Fprintf(os.Stderr, "⚠️ 对话层工具门：控制层规则表装载失败 ⇒ 按**恒拦**处理（%v）\n", err)
			chatGateInst = control.NewAgentGate(nil)
		} else {
			chatGateInst = g
		}
		chatGateLoaded = true
	}
	return chatGateInst
}

// approvalTicket —— 人签的批准件（逃生门的载体）。
type approvalTicket struct {
	Approver   string `json:"approver"`    // 人名（空 = 废票）
	ApprovedAt string `json:"approved_at"` // 签的时间（只回显，不解析时钟）
	Scope      string `json:"scope"`       // `*` 或参数串的子串
	Note       string `json:"note"`
}

// approvalPath —— 批准件的落点（`<状态目录>/approvals/<工具名>.json`）。
func approvalPath(tool string) string {
	return filepath.Join(statepath.Dir(), "approvals", tool+".json")
}

// approvalGranted —— 读批准件判「这一次」放不放行。四种「不算批准」照实分开报：
// 件不存在 / 解析不了 / 人名为空（废票）/ scope 不覆盖本次参数。**读不到一律不算批准。**
func approvalGranted(tool, args string) (bool, string) {
	p := approvalPath(tool)
	b, err := os.ReadFile(p)
	if err != nil {
		return false, ""
	}
	var tk approvalTicket
	if err := json.Unmarshal(b, &tk); err != nil {
		return false, fmt.Sprintf("批准件解析不了（%s）⇒ 不算批准", p)
	}
	if strings.TrimSpace(tk.Approver) == "" {
		return false, fmt.Sprintf("批准件没写 `approver`（%s）⇒ 不算批准", p)
	}
	sc := strings.TrimSpace(tk.Scope)
	if sc != "*" && (sc == "" || !strings.Contains(args, sc)) {
		return false, fmt.Sprintf("批准件 %s 的 scope=%q 不覆盖本次参数 ⇒ 不算批准", p, tk.Scope)
	}
	return true, fmt.Sprintf("人签批准件在册：approver=%s approved_at=%s scope=%s（%s）",
		tk.Approver, tk.ApprovedAt, tk.Scope, p)
}

// chatGateVerdict —— 把一个 gate 判定 (+ err) 翻成**给人的拒因原文**并判「拒不拒」。
//
// ★ 唯一的实现：调用点不许各写一份文案（同一个判定在两处写两句话 = 两套口径的开始）。
// err != nil（问门本身失败）时**按拒处理**，不按放行（fail-closed）。
func chatGateVerdict(tool, args string, d agent.Decision, err error) (string, bool) {
	if err != nil {
		return fmt.Sprintf("工具 %s 问门失败（%v）⇒ 按拒处理（fail-closed）", tool, err), true
	}
	switch d.Action {
	case string(control.ActionAllow):
		return "", false
	case string(control.ActionBlock):
		return fmt.Sprintf("工具 %s 被控制层拦下（action=block）：%s —— 这是**硬拦**；"+
			"要放行得由人改 core/internal/control/rules.yaml 里那条规则（模型改不动规则表）", tool, d.Message), true
	case string(control.ActionRequireApproval):
		if ok, note := approvalGranted(tool, args); ok {
			if note != "" {
				fmt.Fprintf(os.Stderr, "🔓 对话层工具门[%s]：require_approval → 放行 —— %s\n", tool, note)
			}
			return "", false // 逃生门在册 ⇒ 放行（能力没被删）
		}
		_, extra := approvalGranted(tool, args)
		msg := fmt.Sprintf("工具 %s 需要审批（action=require_approval）—— **无审批即不执行**：%s", tool, d.Message)
		if extra != "" {
			msg += "（" + extra + "）"
		}
		msg += fmt.Sprintf("；要放行：由人写一枚批准件 %s"+
			`（内容 {"approver":"<人名>","approved_at":"<时间>","scope":"*","note":"<理由>"}）`+
			" —— 模型写不出这枚件（它是**人签**的逃生门）", approvalPath(tool))
		return msg, true
	default:
		// 认不出的判定（含空）⇒ 拒（fail-closed · 与 agentgate.go 口径①逐字同）。
		return fmt.Sprintf("工具 %s 拿到控制层的未知判定 %q ⇒ 按拒处理（规则表可能拼错）", tool, d.Action), true
	}
}
