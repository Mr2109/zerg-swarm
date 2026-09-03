package gateway

import (
	"log"
)

// ActionModelRouter 动作级路由表：不同动作 → 不同模型。
// 设计参考 docs/规划-v2.3编排能力升级.md 第二节。
//
// 阶段 B MVP：代码常量，后续可迁移到 fleet.yaml 配置。
type ActionModelRouter struct {
	// actionModels 动作类型 → 目标模型名
	actionModels map[ActionType]string
}

// newActionModelRouter 创建默认路由表。
// 阶段 B 默认映射：
//   - 编码 → qwable-v1.q5_k_m（手）
//   - 工具调用/写作/研究 → example-35b（脑）
//   - 默认 → example-35b（脑，兜底）
func newActionModelRouter() *ActionModelRouter {
	r := &ActionModelRouter{
		actionModels: make(map[ActionType]string),
	}

	r.actionModels[ActionCoding] = "qwable-v1.q5_k_m"
	r.actionModels[ActionToolCall] = "example-35b"
	r.actionModels[ActionWriting] = "example-35b"
	r.actionModels[ActionResearch] = "example-35b"
	r.actionModels[ActionDefault] = "example-35b"

	return r
}

// resolveModel 给定动作类型，返回目标模型名。
// 如果该动作未配置路由，返回 ""（调用方回退到默认行为）。
func (r *ActionModelRouter) resolveModel(action ActionType) string {
	model, ok := r.actionModels[action]
	if !ok {
		return ""
	}
	log.Printf("🎯 动作路由: %s → %s", action.String(), model)
	return model
}
