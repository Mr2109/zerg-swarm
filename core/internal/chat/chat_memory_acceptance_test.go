// chat_memory_acceptance_test.go — 乙批验收（2026-09-10）：设计稿 §3「乙批验收」①-⑤
//
// ① 写入一条事实 → 新会话的记忆块出现且内容正确
// ② 超预算 / 匹配歧义 / 批量清空 三类拒绝各返回正确指引
// ③ 连续 3 次失败后第 4 次返回终止态（不阻塞本轮回复）
// ④ session_search 能找回被软删（旧分支）的消息
// ⑤ 会话内写记忆**不改变**已发出请求的 system prompt（字节比对）
//
// 运行：cd core && go test ./internal/chat/ -run BatchB -v

package chat

import (
	"encoding/json"
	"strings"
	"testing"

	"zerg/core/internal/memory"
)

// acIsolateMemory — 记忆存储指向临时目录（绝不写真实 ~/.zerg/memory）
func acIsolateMemory(t *testing.T) {
	t.Helper()
	old := memStore
	memStore = memory.Open(t.TempDir())
	t.Cleanup(func() { memStore = old })
}

// acApply — 走工具入口执行一次 memory 调用并解析结构化结果
func acApply(t *testing.T, args map[string]any) memory.Result {
	t.Helper()
	out, err := MemoryToolExecute(args)
	if err != nil {
		t.Fatalf("memory 工具执行异常: %v", err)
	}
	var r memory.Result
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatalf("结果不是合法 JSON(%v): %s", err, out)
	}
	return r
}

// ── ① 写入 → 记忆块可见 ─────────────────────────────────────────────────

func TestBatchB_Acceptance1_WriteThenVisible(t *testing.T) {
	acIsolateMemory(t)
	BeginMemoryTurn("acc1")

	r := acApply(t, map[string]any{
		"action": "add", "target": "memory",
		"content": "验收①：Mr2109要求报告用中文、带命令与数字",
	})
	if !r.Success {
		t.Fatalf("写入应成功，实际: %+v", r)
	}
	if r.Usage == "" || r.EntryCount != 1 {
		t.Fatalf("成功应答应含 usage 与 entry_count，实际: %+v", r)
	}

	// 新会话（不同 sessionID）取块 → 内容必须出现
	block := MemoryBlock("acc1-new-session")
	if !strings.Contains(block, "报告用中文") {
		t.Fatalf("记忆块未出现写入内容:\n%s", block)
	}
	t.Logf("✓ ① 记忆块内容正确（%d 字符）:\n%s", len(block), block)
}

// ── ② 三类拒绝的指引 ────────────────────────────────────────────────────

func TestBatchB_Acceptance2_Refusals(t *testing.T) {
	acIsolateMemory(t)
	BeginMemoryTurn("acc2")

	// ②-1 超预算（memory 2200）——须给"先 consolidate"的指引
	over := acApply(t, map[string]any{
		"action": "add", "target": "memory",
		"content": strings.Repeat("字", 2300),
	})
	if over.Success {
		t.Fatal("超预算应拒写")
	}
	if over.Hint == "" || !strings.Contains(over.Hint+over.Error, "consolidate") {
		t.Fatalf("超预算应给 consolidate 指引，实际 hint=%q err=%q", over.Hint, over.Error)
	}
	t.Logf("✓ ②-1 超预算: %s | %s", over.Error, over.Hint)

	// ②-2 匹配歧义（两条都含同一片段）——须回带现有条目
	acApply(t, map[string]any{"action": "add", "target": "memory", "content": "重复条目 A 内容"})
	acApply(t, map[string]any{"action": "add", "target": "memory", "content": "重复条目 B 内容"})
	amb := acApply(t, map[string]any{"action": "replace", "target": "memory", "old_text": "重复条目", "content": "新内容"})
	if amb.Success {
		t.Fatal("歧义匹配应拒写")
	}
	if len(amb.CurrentEntries) < 2 {
		t.Fatalf("歧义应回带现有条目，实际 %d 条", len(amb.CurrentEntries))
	}
	t.Logf("✓ ②-2 歧义: %s（回带 %d 条）", amb.Error, len(amb.CurrentEntries))

	// ②-3 缺 old_text
	miss := acApply(t, map[string]any{"action": "remove", "target": "memory"})
	if miss.Success || !strings.Contains(miss.Hint+miss.Error, "old_text") {
		t.Fatalf("缺 old_text 应拒写并提示重发，实际: %+v", miss)
	}
	t.Logf("✓ ②-3 缺 old_text: %s", miss.Hint+miss.Error)

	// ②-4 一次 batch 清空全部条目
	all := acApply(t, map[string]any{
		"action": "remove", "target": "memory", "operations": []any{
			map[string]any{"action": "remove", "old_text": "重复条目 A"},
			map[string]any{"action": "remove", "old_text": "重复条目 B"},
		},
	})
	if all.Success || all.Hint == "" {
		t.Fatalf("批量清空应拒写并给单次 remove 指引，实际: %+v", all)
	}
	t.Logf("✓ ②-4 批量清空: %s", all.Hint)
}

// ── ③ 终止态（每轮失败 ≥3 次）──────────────────────────────────────────

func TestBatchB_Acceptance3_TerminalState(t *testing.T) {
	acIsolateMemory(t)
	BeginMemoryTurn("acc3")

	// 零匹配 = 可修复失败（计入每轮上限）
	failArgs := map[string]any{"action": "remove", "target": "memory", "old_text": "根本不存在的事实"}
	for i := 1; i <= 3; i++ {
		r := acApply(t, failArgs)
		if r.Success {
			t.Fatalf("第 %d 次应失败", i)
		}
		if r.Done {
			t.Fatalf("第 %d 次不应进入终止态（上限 3 次）", i)
		}
	}
	r4 := acApply(t, failArgs)
	if !r4.Done {
		t.Fatalf("第 4 次应返回终止态 {success:false,done:true}，实际: %+v", r4)
	}
	if !strings.Contains(r4.Error, "停止重试") {
		t.Fatalf("终止态应明确“停止重试、先回答用户”，实际: %q", r4.Error)
	}
	t.Logf("✓ ③ 终止态: %s", r4.Error)
}

// ── ④ session_search 找回软删（旧分支）的消息 ────────────────────────────

func TestBatchB_Acceptance4_SessionSearchFindsSoftDeleted(t *testing.T) {
	st := newTestStore(t)
	prev := wireStore
	SetWireStore(st)
	t.Cleanup(func() { SetWireStore(prev) })

	se, err := st.CreateSession("example-35b-v2", "desktop", "", "乙批验收④会话")
	if err != nil {
		t.Fatal(err)
	}
	ids := ssAddMsgs(t, st, se.ID, 1757600000.0,
		"第一轮：关于部署脚本的讨论",
		"第二轮：备份策略是每天 03:00 全量",
		"第三轮：检索用 FTS5 trigram",
	)
	// 软删旧分支：截断 ids[0] 之后的消息（模拟重新生成/压缩后的旧分支）
	if _, err := st.SoftDeleteAfter(se.ID, ids[0]); err != nil {
		t.Fatal(err)
	}

	out, err := SessionSearchToolExec(map[string]any{"query": "备份策略"})
	if err != nil {
		t.Fatalf("session_search 执行失败: %v", err)
	}
	if !strings.Contains(out, "备份策略") {
		t.Fatalf("软删（active=0）的消息应仍可搜到，实际输出:\n%s", out)
	}
	if !strings.Contains(out, "▶ 恢复该段上下文") {
		t.Fatalf("结果应含恢复指针，实际输出:\n%s", out)
	}
	t.Logf("✓ ④ 找回软删消息 + 恢复指针:\n%s", out)
}

// ── ⑤ 会话内字节稳定（写记忆不改已发出的 system prompt）────────────────

func TestBatchB_Acceptance5_FrozenWithinSession(t *testing.T) {
	acIsolateMemory(t)
	const sid = "acc5"
	BeginMemoryTurn(sid)

	before := MemoryBlock(sid) // 首次构建（可能为空）
	if r := acApply(t, map[string]any{"action": "add", "target": "memory", "content": "验收⑤：这条写入不得改变本会话已发出的提示"}); !r.Success {
		t.Fatalf("写入应成功: %+v", r)
	}
	after := MemoryBlock(sid)
	if before != after {
		t.Fatalf("会话内记忆块发生字节变化（缓存前缀会失效）:\n前: %q\n后: %q", before, after)
	}

	// 压缩/换会话后重建 → 新内容出现
	ResetMemoryBlock(sid)
	rebuilt := MemoryBlock(sid)
	if rebuilt == after {
		t.Fatal("ResetFrozen 后应重新渲染")
	}
	if !strings.Contains(rebuilt, "验收⑤") {
		t.Fatalf("重建后的块应含新条目:\n%s", rebuilt)
	}
	t.Logf("✓ ⑤ 会话内字节稳定；重建后含新条目（%d → %d 字符）", len(after), len(rebuilt))
}

// ── 注册面：两个工具都是 deferred（非常驻）+ 可执行 ──────────────────────

func TestBatchB_ToolsRegisteredAsDeferred(t *testing.T) {
	for _, name := range []string{"memory", "session_search"} {
		if !DeferredToolNames[name] {
			t.Fatalf("%s 应注册为 deferred 工具（L0=false）", name)
		}
		if _, ok := ChatExtraToolDefs()[name]; !ok {
			t.Fatalf("%s 缺少工具 schema（ChatExtraToolDefs）", name)
		}
		if ChatToolCategoryOf(name) == "其他" {
			t.Fatalf("%s 应有明确类别（tool_search 分类索引用）", name)
		}
	}
	t.Log("✓ 注册面: memory / session_search 均为 deferred 且有 schema 与类别")
}
