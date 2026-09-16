// outbudget.go — 输出预算动态分配（2026-09-17 Mr2109 拍：写死 max_tokens 不科学，须动态）
//
// 设计稿：docs/01-设计/设计-输出预算动态分配-20260917.md
//
// 背景（实测）：adapter 把 max_tokens 写死 2000 ⇒ 思考型模型思考一开就吃光额度 ⇒ 正文为空
//
//	（直连实测 X3 105.5s / Mr2109 69.7s，两条皆 content='' + finish=length）。
//
// 唯一公式（不写死）：
//
//	reserve = max(ctx×10%, 2048)          安全余量（防超长/防显存意外）
//	avail   = ctx − promptEst − reserve    本次真正可给的输出空间
//	maxOut  = clamp(avail × thinkFactor, 1024, min(ctx−reserve, 32768))
//	提示超限（avail ≤ 0）⇒ 返回 0 交由调用方**拒绝并教学式报错**（不静默截断）
package gateway

import "strings"

// 估算：中文/代码混合下，1 token ≈ 4 字节（保守取小 ⇒ 宁可少估提示、也别把额度虚报大）
const bytesPerToken = 4

// 思考型模型的放大系数（无实测档案时先用 1.5 兜底；有档案后按档案覆盖）。
// key 为模型名子串（小写匹配）。
var thinkFactorByModel = map[string]float64{
	"qwen3.8-27b": 1.5,
	"qwen3":       1.5,
	"deepseek":    1.5,
	"gemma":       1.0,
	"example-35b-v2":      1.0,
}

// thinkFactor — 该模型的思考占用系数（1.0 = 非思考型/无需放大）
func thinkFactor(model string) float64 {
	m := strings.ToLower(model)
	for k, v := range thinkFactorByModel {
		if strings.Contains(m, k) {
			return v
		}
	}
	return 1.0
}

// DynamicMaxTokens — 按剩余空间动态给出输出额度。
//
// 返回 (maxOut, ctxUsed, promptEst)：
//
//	maxOut == 0 ⇒ 提示已超限，调用方必须**拒绝并教学式报错**（不得静默截断、不得退回写死值）。
func DynamicMaxTokens(model string, ctx, promptEst int) (int, int, int) {
	if ctx <= 0 {
		ctx = defaultCtxWindow // 卵未声明 ⇒ 保守默认（并在日志标注来源，不假设 256k）
	}
	if promptEst < 0 {
		promptEst = 0
	}
	reserve := ctx / 10
	if reserve < minReserveTokens {
		reserve = minReserveTokens
	}
	avail := ctx - promptEst - reserve
	if avail <= 0 {
		return 0, ctx, promptEst
	}
	upper := ctx - reserve
	if upper > maxOutCeiling {
		upper = maxOutCeiling
	}
	out := int(float64(avail) * thinkFactor(model))
	if out > upper {
		out = upper
	}
	if out < minOutTokens {
		out = minOutTokens
	}
	if out > upper { // 极小上下文时 minOutTokens 可能超过 upper ⇒ 以 upper 为准（仍保证能说一句话）
		out = upper
	}
	return out, ctx, promptEst
}

// EstimatePromptTokens — 用请求体字节数保守估算提示 tokens（宁小不大：错估小只会让我们少留一点余量）
func EstimatePromptTokens(bodyBytes int) int {
	if bodyBytes <= 0 {
		return 0
	}
	return bodyBytes / bytesPerToken
}

const (
	defaultCtxWindow = 32768 // 卵未声明上下文时的保守默认（不假设 256k）
	minReserveTokens = 2048  // 安全余量下限
	maxOutCeiling    = 32768 // 单次输出上限（防"一次吐到天荒地老"）
	minOutTokens     = 1024  // 最小输出额度（保证还能说一句话）
)
