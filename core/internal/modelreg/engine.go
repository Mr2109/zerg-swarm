package modelreg

import "strings"

// ── 引擎维度的规范化（待修补 #11）──────────────────────────────────────
//
// 为什么需要它：同一条能力在不同引擎上可以真假不同（实测 vision=true 是在
// llama.cpp 侧探到的，而 vLLM 侧没挂 mmproj）。能力断言因此必须带**引擎维度**，
// 路由硬门槛按**目标引擎**取能力。
//
// 两个命名面必须收敛到同一个词，否则门槛永远对不上、只会盲目 fail-closed：
//   - 探测侧：`probe --engine`（engine_recipes 的键名，如 "llama.cpp" / "vllm"）；
//   - 路由侧：fleet.yaml 候选的 `backend`（如 "llama-server" / "ds4-server"）。
//
// CanonicalEngine 把两边收到的名字收敛成同一个标签。未识别的引擎**原样返回**
// （不做映射猜测——猜错会把"没证据"变成"误判有证据"，比不映射更危险）。
func CanonicalEngine(name string) string {
	s := strings.TrimSpace(name)
	switch strings.ToLower(s) {
	case "llama.cpp", "llama", "llamacpp", "llama-server", "llama_server", "llamaserver", "llama-cpp":
		return "llama.cpp"
	case "vllm", "v-llm":
		return "vllm"
	case "sglang", "sglang-server":
		return "sglang"
	case "ollama":
		return "ollama"
	case "ds4", "ds4-server", "ds4_server":
		return "ds4"
	}
	return s
}
