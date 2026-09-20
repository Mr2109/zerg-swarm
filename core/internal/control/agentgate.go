// agentgate.go — 把控制层的 Gate 接到 **CA 侧工具门**（`agent.ToolGater`）上。
//
// 为什么要有这一个薄壳（§二十一 已红第 9 条 · 批 C 的 T-31）：M3 的规则表装在
// `core/internal/control`（三态 + rules.yaml），而 CA 侧的问门接口在 `core/internal/agent`
// （`type ToolGater interface { Check(toolName, args, agent string) (Decision, error) }`）——
// 两边的**类型不同**（`GateDecision` vs `Decision`），中间没有桥 ⇒ 于是 `Agent.SetGate(...)`
// **一个调用方都没有**（判据② 的原话：`.SetGate(` 零调用方）——「规则写了、没接电」。
//
// 口径三条（缺一即错）：
//
//	① **映射只做形状转换，不做语义发明**：allow/block/require_approval 三态**逐字**搬过去；
//	   任何别的值（含空）⇒ `block`（fail-closed —— 认不出来的判定不许当放行）。
//	② **没有 gate 就是 block**（不是 allow）：`SetGate(nil)` 或构造失败时，工具门拿到的
//	   必须是「拦」，否则「配置没装上」会静默退化成全放行。
//	③ **规则表装载失败不吞**：`NewAgentGateFromRepo` 返回错误，调用方（`zerg-agent` 主程序）
//	   要么显式降级、要么拒启 —— 但**不许**把「没装规则」当成「没规则要装」。
package control

import (
	"fmt"

	"github.com/Mr2109/zerg-swarm/core/internal/agent"
)

// AgentGate —— 控制层 Gate 的 CA 侧适配器（实现 agent.ToolGater）。
type AgentGate struct {
	inner *Gate
}

// NewAgentGate 用一张已装载的规则表造适配器。`nil` 规则表 ⇒ 一个**恒拦**的门（口径②）。
func NewAgentGate(g *Gate) *AgentGate { return &AgentGate{inner: g} }

// NewAgentGateFromRepo 从居仓的规则表真源装载（`<仓根>/core/internal/control/rules.yaml`）。
// 为什么钉在仓根：网关侧（`core/internal/gateway`）就是这么找它的（同一份文件 = 同一张表），
// 两处各写一个相对路径就会漂。
func NewAgentGateFromRepo(repoRoot string) (*AgentGate, error) {
	g, err := NewGateFromFile(fmt.Sprintf("%s/core/internal/control/rules.yaml", repoRoot))
	if err != nil {
		return nil, fmt.Errorf("装载控制层规则表失败（M3 gate 不接电 ⇒ 工具门只能拒）：%w", err)
	}
	return NewAgentGate(g), nil
}

// Check 实现 agent.ToolGater：把控制层判定搬到 CA 侧。
func (a *AgentGate) Check(toolName, args, agentName string) (agent.Decision, error) {
	if a == nil || a.inner == nil {
		// 口径②：没有规则表 ⇒ 拦（不是放行）
		return agent.Decision{Action: string(ActionBlock), Message: "控制层规则表未装载（M3 gate 未接电）⇒ 拒"}, nil
	}
	d := a.inner.Check(toolName, args, agentName)
	switch d.Action {
	case ActionAllow, ActionBlock, ActionRequireApproval:
		return agent.Decision{Action: string(d.Action), Message: d.Message}, nil
	default:
		// 口径①：认不出的判定 ⇒ 拦
		return agent.Decision{
			Action:  string(ActionBlock),
			Message: fmt.Sprintf("控制层给出未知判定 %q（规则表可能有拼错）⇒ 按拦处理", d.Action),
		}, nil
	}
}
