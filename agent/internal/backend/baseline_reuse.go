// baseline_reuse.go —— M10：请求的模型与基线服务身份一致时「直接复用」。
//
// 设计依据：《设计-子端服务切换与基线服务声明》§11 M10 ——
//
//	「身份一致直接代理」：请求的模型如果就是某个手工起的基线服务（端口上跑着同一个权重），
//	就不该再去借坑、停它、再加载一份 —— 直接用它的端口（不借、不停、不加载）。
//
// 安全立场（Mr2109 2026-09-14 拍板口径）：
//
//	**外部项绝不进驱逐/停服路径**。复用登记出来的驻留项带 `external=true` 且 **不带任何进程句柄**
//	（proc 为 nil）⇒ 结构上就无从 kill；此外在唯一的动进程处（evictSubprocLocked）与借用挑目标处
//	各加一道显式守卫，并逐条配测试。
//
// 为什么匹配必须**保守**：复用错了等于把请求发给另一个模型（静默的语义错误，比加载慢更糟）。
// 因此只在「归一化后完全同名」时成立，不做前缀/包含/相似度匹配。
package backend

import "strings"

// normalizeIdentity 把身份归一化成可比较的形式：取最后一段路径、去空白、小写。
//
// 归一化的理由：同一个权重的身份在不同来源写法不同 ——
//
//	· 基线侧来自 `/v1/models`（llama 报权重全路径 `/data/models/k2/k2horizon-q4_k_m.gguf`）
//	· 请求侧来自注册表条目（可能只写权重名或另一段路径）
//
// 取 basename 能让两者对齐；小写化是为了跨来源大小写差异不误判。
func normalizeIdentity(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.LastIndexByte(s, '/'); i >= 0 {
		s = s[i+1:]
	}
	return strings.ToLower(s)
}

// baselineReuseMatch 判断两个身份是否是「同一个权重」。空值一律不算匹配。
func baselineReuseMatch(a, b string) bool {
	na, nb := normalizeIdentity(a), normalizeIdentity(b)
	return na != "" && na == nb
}

// baselineReuseCandidateLocked 在基线服务里找一个「与请求身份一致」的可复用项。
//
// wantIdentities：请求侧可能的身份写法（条目路径、模型名等）——任一命中即可。
// skipPorts：已被本端**托管**占用的端口（不能复用别人的进程占的端口）。
//
// 纯函数式：只读入参、只返回结果，不碰进程与网络 ⇒ 可脱离真机单测。
func baselineReuseCandidateLocked(services []BaselineService, wantIdentities []string, skipPorts map[int]bool) (BaselineService, bool) {
	for _, svc := range services {
		if svc.Port <= 0 || !svc.Listening {
			continue // 没在听 = 现在不可用，复用没意义（但也不去动它）
		}
		if skipPorts[svc.Port] {
			continue
		}
		for _, want := range wantIdentities {
			if baselineReuseMatch(want, svc.Identity) {
				return svc, true
			}
		}
	}
	return BaselineService{}, false
}

// baselineReuseIdentities 列出「请求这个模型时，可能出现的身份写法」。
//
// 目前两个来源：注册表条目里的权重路径、模型别名。多给几个不增加风险 ——
// 匹配本身是**完全同名**判定，多写几个只提高命中率，不会放宽匹配。
func baselineReuseIdentities(modelName string, weightPath string) []string {
	out := make([]string, 0, 2)
	if weightPath != "" {
		out = append(out, weightPath)
	}
	if modelName != "" {
		out = append(out, modelName)
	}
	return out
}
