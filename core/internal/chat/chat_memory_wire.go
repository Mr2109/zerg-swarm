// chat_memory_wire.go — 乙批接线（2026-09-10）：记忆体系的对话层落点
//
// 设计：docs/01-设计/设计-虫族记忆体系-20260910.md §3.1（MemoryStore 接线）/ §3.2（memory 工具）/ §3.3（session_search）
//
// 三件事：
//  1) 记忆存储单例（两级作用域：global + agents/<id>）——目录 ~/.zerg/memory/（ZERG_MEMORY_DIR 可覆盖）
//  2) 记忆块 MemoryBlock(sessionID)：参与系统提示 volatile 层，且**会话内字节冻结**（写盘不改已发出请求的提示）
//  3) 两个工具执行器：memory（写回）/ session_search（检索对话历史）
//
// 可见性：两个工具都**非常驻**——挂在 chatToolRegistry 的 deferred 目录里（L0=false），
// 模型经 tool_search 发现后调用；经 RegisterChatExtraTools 自动进入 agent.ExtraTool 通道（CA/对话共用）。

package chat

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"zerg/core/internal/memory"
)

var (
	wireMu    sync.Mutex
	wireStore *ChatStore // 对话库（api 层注入；未注入则惰性 OpenChatStore）
	// memStore — 记忆存储单例（条目式热记忆 + 预算 + 出处分级）
	memStore = memory.Open(memory.DefaultRoot())
)

// SetWireStore — api 层启动时注入对话库（session_search 用）
func SetWireStore(s *ChatStore) {
	wireMu.Lock()
	defer wireMu.Unlock()
	wireStore = s
}

// wireChatStore — 取对话库（未注入则惰性打开；打开失败返回 nil，由执行器给出可行动错误）
func wireChatStore() *ChatStore {
	wireMu.Lock()
	defer wireMu.Unlock()
	if wireStore != nil {
		return wireStore
	}
	s, err := OpenChatStore()
	if err != nil {
		return nil
	}
	wireStore = s
	return wireStore
}

// MemoryStoreInst — 记忆存储实例（测试/其它包取用）
func MemoryStoreInst() *memory.Store { return memStore }

// MemoryBlock — 记忆块（会话内冻结：同一会话内多次调用返回同一字节串）
// sessionID 为空时退化为实时渲染（无会话上下文——如一次性任务）。
func MemoryBlock(sessionID string) string {
	if sessionID == "" {
		return memStore.Block("global")
	}
	return memStore.FrozenBlock(sessionID, "global")
}

// ResetMemoryBlock — 压缩/会话切换后重建冻结快照（下一次取块时重新渲染）
func ResetMemoryBlock(sessionID string) { memStore.ResetFrozen(sessionID) }

// BeginMemoryTurn — 每轮开始：清零"每轮失败计数"（对齐 Hermes 的 reset_consolidation_failures）
// 语义：连续失败 ≥3 次后，第 4 次 memory 调用返回终止态 {success:false,done:true}，
// 避免记忆副作用阻塞本轮回复。
func BeginMemoryTurn(sessionID string) { memStore.BeginTurn(sessionID) }

// memoryScope — 解析作用域：global（默认）/ agent（需 agent_id）
// 两级作用域（Mr2109拍板）：global=环境/约定/偏好；agent=项目事实。
func memoryScope(args map[string]any) (string, error) {
	scope := "global"
	if s, ok := args["scope"].(string); ok && s != "" {
		scope = s
	}
	switch scope {
	case "global":
		return "global", nil
	case "agent":
		id, _ := args["agent_id"].(string)
		if id == "" {
			return "", fmt.Errorf("scope=agent 需要 agent_id（项目事实按 agent 隔离；不传则请用 scope=global）")
		}
		return "agents/" + id, nil
	default:
		return "", fmt.Errorf("scope 只支持 global / agent，收到 %q", scope)
	}
}

// memoryOps — 解析单个 op（支持单条与 operations[] 批量两种形态）
func memoryOps(args map[string]any) ([]memory.Op, error) {
	var out []memory.Op
	if raw, ok := args["operations"].([]any); ok && len(raw) > 0 {
		for i, item := range raw {
			m, ok := item.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("operations[%d] 不是对象", i+1)
			}
			out = append(out, memory.Op{
				Action:  strArg(m, "action"),
				Content: strArg(m, "content"),
				OldText: strArg(m, "old_text"),
				Source:  strArg(m, "source"),
			})
		}
		return out, nil
	}
	out = append(out, memory.Op{
		Action:  strArg(args, "action"),
		Content: strArg(args, "content"),
		OldText: strArg(args, "old_text"),
		Source:  strArg(args, "source"),
	})
	return out, nil
}

func strArg(m map[string]any, k string) string {
	if v, ok := m[k].(string); ok {
		return v
	}
	return ""
}

// MemoryToolExecute — memory 工具执行器（返回结构化 JSON——对齐 Hermes 的应答形状）
// 参数：action(add|replace|remove) / target(memory|user) / content? / old_text? /
//       operations[]（批量原子）/ scope(global|agent) / agent_id? / source(user|model|tool|web)
func MemoryToolExecute(args map[string]any) (string, error) {
	target := memory.Target("memory")
	if t, ok := args["target"].(string); ok && t != "" {
		target = memory.Target(t)
	}
	scope, err := memoryScope(args)
	if err != nil {
		b, _ := json.Marshal(memory.Result{Success: false, Error: err.Error()})
		return string(b), nil // 参数类错误按结构化失败返回（模型可自纠），不算工具异常
	}
	ops, err := memoryOps(args)
	if err != nil {
		b, _ := json.Marshal(memory.Result{Success: false, Error: err.Error()})
		return string(b), nil
	}
	// 2026-09-10 边界③修正：失败计数按会话隔离（_session_id 由 chat_handlers 在调用前注入；
	// 缺省空串=默认键，单会话行为与旧版等价）
	sessionKey, _ := args["_session_id"].(string)
	res := memStore.ApplyFor(sessionKey, scope, target, ops)
	b, merr := json.Marshal(res)
	if merr != nil {
		return "", merr
	}
	return string(b), nil
}

// SessionSearchToolExec — session_search 工具执行器（search / read / browse 三模式 + 恢复指针）
func SessionSearchToolExec(args map[string]any) (string, error) {
	st := wireChatStore()
	if st == nil {
		return "", fmt.Errorf("对话库未就绪：session_search 无法检索（请确认主控已初始化对话库）")
	}
	return SessionSearchExecute(args, st)
}

// MemoryRoot — 记忆根目录（UI/文档/验收用）
func MemoryRoot() string {
	if d := os.Getenv("ZERG_MEMORY_DIR"); d != "" {
		return d
	}
	return memStore.Root()
}
