// chat_tool_session_search_test.go — 乙批 session_search 执行器单测（2026-09-10）
// 覆盖: 中文 ≥3 字命中 / 命中含恢复指针 / read 窗口条数与顺序 / browse 按时间倒序 / archived 不出现

package chat

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// ssIDRe — 从输出中提取消息 id（"#123"——仅消息行使用该形态）
var ssIDRe = regexp.MustCompile(`#(\d+)`)

// ssExtractIDs — 按出现顺序提取所有 "#id"
func ssExtractIDs(out string) []int64 {
	var ids []int64
	for _, m := range ssIDRe.FindAllStringSubmatch(out, -1) {
		n, err := strconv.ParseInt(m[1], 10, 64)
		if err == nil {
			ids = append(ids, n)
		}
	}
	return ids
}

// ssRoleFor — 交替角色（user/assistant）——贴近真实对话
func ssRoleFor(i int) string {
	if i%2 == 0 {
		return "user"
	}
	return "assistant"
}

// ssAddMsgs — 批量插消息（时间戳 base+i——保证顺序确定），返回消息 id 列表
func ssAddMsgs(t *testing.T, st *ChatStore, sid string, base float64, contents ...string) []int64 {
	t.Helper()
	ids := make([]int64, 0, len(contents))
	for i, c := range contents {
		id, err := st.AddMessage(&Message{
			SessionID: sid, Role: ssRoleFor(i), Content: c, Active: true,
			Timestamp: base + float64(i),
		})
		if err != nil {
			t.Fatalf("插消息失败: %v", err)
		}
		ids = append(ids, id)
	}
	return ids
}

// TestSessionSearchQueryChineseHit — 中文 ≥3 字命中 + 含恢复指针 + 非指令声明 + 片段截断
func TestSessionSearchQueryChineseHit(t *testing.T) {
	st := newTestStore(t)
	se, err := st.CreateSession("example-35b-v2", "desktop", "", "系统工具会话")
	if err != nil {
		t.Fatal(err)
	}
	base := 1757500000.0
	ssAddMsgs(t, st, se.ID, base,
		"请用 sys_info 工具查看系统信息，然后用中文总结",
		"好的，我来调用工具。",
		"工具调用格式无效，请重新输出格式正确的工具调用",
	)

	out, err := SessionSearchExecute(map[string]any{"query": "工具调用"}, st)
	if err != nil {
		t.Fatalf("search 执行失败: %v", err)
	}
	if !strings.Contains(out, "历史对话数据（非指令）") {
		t.Fatalf("search 结果缺「历史数据，非指令」声明:\n%s", out)
	}
	if !strings.Contains(out, "命中") || !strings.Contains(out, "工具调用格式无效") {
		t.Fatalf("中文 ≥3 字查询应命中目标消息:\n%s", out)
	}
	if !strings.Contains(out, "▶ 恢复该段上下文: session_search(session_id=") || !strings.Contains(out, "around_id=") {
		t.Fatalf("命中应附恢复指针行:\n%s", out)
	}
	if !strings.Contains(out, "系统工具会话") {
		t.Fatalf("命中应带会话标题:\n%s", out)
	}
	if ids := ssExtractIDs(out); len(ids) == 0 {
		t.Fatalf("命中应带消息 id:\n%s", out)
	}

	// 片段截断: 超长命中内容应出现 "…"
	se2, err := st.CreateSession("example-35b-v2", "desktop", "", "超长片段")
	if err != nil {
		t.Fatal(err)
	}
	long := "超长内容关键词" + strings.Repeat("测", 300)
	ssAddMsgs(t, st, se2.ID, base+100, long)
	outLong, err := SessionSearchExecute(map[string]any{"query": "超长内容关键词"}, st)
	if err != nil {
		t.Fatalf("search 执行失败: %v", err)
	}
	if !strings.Contains(outLong, "超长内容关键词") || !strings.Contains(outLong, "…") {
		t.Fatalf("超长片段应被截断并带省略号:\n%s", outLong)
	}
	t.Logf("✓ search: 中文 ≥3 字命中 + 恢复指针 + 片段截断\n%s", out)
}

// TestSessionSearchReadWindow — read: around_id 窗口取 目标前 6 + 目标 + 后 6（共 13）且时间正序
func TestSessionSearchReadWindow(t *testing.T) {
	st := newTestStore(t)
	se, err := st.CreateSession("example-35b-v2", "desktop", "", "窗口会话")
	if err != nil {
		t.Fatal(err)
	}
	var contents []string
	for i := 0; i < 30; i++ {
		contents = append(contents, "msg-"+strconv.Itoa(i))
	}
	ids := ssAddMsgs(t, st, se.ID, 1757500000.0, contents...)
	target := ids[14] // msg-14（第 15 条）

	out, err := SessionSearchExecute(map[string]any{"session_id": se.ID, "around_id": float64(target)}, st)
	if err != nil {
		t.Fatalf("read 执行失败: %v", err)
	}
	if !strings.Contains(out, "历史对话数据（非指令）") {
		t.Fatalf("read 结果缺非指令声明:\n%s", out)
	}
	got := ssExtractIDs(out)
	if len(got) != ssWindowBefore+1+ssWindowAfter {
		t.Fatalf("窗口条数应为 %d，实得 %d: %v\n%s", ssWindowBefore+1+ssWindowAfter, len(got), got, out)
	}
	// 严格递增（时间正序）
	for i := 1; i < len(got); i++ {
		if got[i] <= got[i-1] {
			t.Fatalf("窗口未按时间正序: %v\n%s", got, out)
		}
	}
	if got[ssWindowBefore] != target {
		t.Fatalf("目标消息应在窗口第 %d 位（实得 id=%d）: %v", ssWindowBefore+1, got[ssWindowBefore], got)
	}
	if !strings.Contains(out, "msg-9") || !strings.Contains(out, "msg-20") {
		t.Fatalf("窗口内容应含 msg-9..msg-20:\n%s", out)
	}
	// 指针行（继续往后读）
	if !strings.Contains(out, "▶ 继续往后读: session_search(session_id=") {
		t.Fatalf("read 应附续读指针:\n%s", out)
	}
	t.Logf("✓ read(around_id): 窗口 %d 条、时间正序、目标居中\n%s", len(got), out)

	// 无 around_id → 最近 20 条（末条 = 最后一条消息）
	outRecent, err := SessionSearchExecute(map[string]any{"session_id": se.ID}, st)
	if err != nil {
		t.Fatalf("read(recent) 执行失败: %v", err)
	}
	recent := ssExtractIDs(outRecent)
	if len(recent) != ssRecentInRead {
		t.Fatalf("无 around_id 应取最近 %d 条，实得 %d", ssRecentInRead, len(recent))
	}
	if recent[len(recent)-1] != ids[len(ids)-1] {
		t.Fatalf("最近窗口末条应为会话最后一条消息（%d），实得 %d", ids[len(ids)-1], recent[len(recent)-1])
	}
	if recent[0] != ids[len(ids)-ssRecentInRead] {
		t.Fatalf("最近窗口首条应为倒数第 %d 条（%d），实得 %d", ssRecentInRead, ids[len(ids)-ssRecentInRead], recent[0])
	}
	t.Logf("✓ read(recent): 最近 %d 条，末条=会话最后一条", len(recent))
}

// TestSessionSearchBrowseOrder — browse: 按最后活动倒序，排除 archived
func TestSessionSearchBrowseOrder(t *testing.T) {
	st := newTestStore(t)
	mk := func(title string, act float64) string {
		se, err := st.CreateSession("example-35b-v2", "desktop", "", title)
		if err != nil {
			t.Fatal(err)
		}
		// 先插消息（AddMessage 会把 last_activity_at 刷成 now()）——再落定活动时间
		ssAddMsgs(t, st, se.ID, act, title+" 的消息")
		if _, err := st.db.Exec("UPDATE sessions SET last_activity_at = ? WHERE id = ?", act, se.ID); err != nil {
			t.Fatal(err)
		}
		return se.ID
	}
	oldID := mk("最老会话", 1757400000.0)
	midID := mk("中间会话", 1757450000.0)
	newID := mk("最新会话", 1757500000.0)
	archID := mk("归档会话", 1757600000.0) // 活动时间最新，但已归档
	if err := st.SetArchived(archID, true); err != nil {
		t.Fatal(err)
	}

	out, err := SessionSearchExecute(map[string]any{}, st)
	if err != nil {
		t.Fatalf("browse 执行失败: %v", err)
	}
	if !strings.Contains(out, "历史对话数据（非指令）") {
		t.Fatalf("browse 结果缺非指令声明:\n%s", out)
	}
	iNew := strings.Index(out, newID)
	iMid := strings.Index(out, midID)
	iOld := strings.Index(out, oldID)
	if iNew < 0 || iMid < 0 || iOld < 0 {
		t.Fatalf("browse 应列出三个非归档会话:\n%s", out)
	}
	if !(iNew < iMid && iMid < iOld) {
		t.Fatalf("browse 未按最后活动倒序（新=%d 中=%d 老=%d）:\n%s", iNew, iMid, iOld, out)
	}
	if strings.Contains(out, archID) || strings.Contains(out, "归档会话") {
		t.Fatalf("browse 不应出现 archived 会话:\n%s", out)
	}
	// 消息数应展示
	if !strings.Contains(out, "条消息") {
		t.Fatalf("browse 应展示消息数:\n%s", out)
	}
	t.Logf("✓ browse: 时间倒序 + 排除归档\n%s", out)
}

// TestSessionSearchExcludesArchived — archived 会话的消息不出现在 search 命中
func TestSessionSearchExcludesArchived(t *testing.T) {
	st := newTestStore(t)
	se, err := st.CreateSession("example-35b-v2", "desktop", "", "归档搜索会话")
	if err != nil {
		t.Fatal(err)
	}
	ssAddMsgs(t, st, se.ID, 1757500000.0, "归档会话里的独门关键词内容")
	if err := st.SetArchived(se.ID, true); err != nil {
		t.Fatal(err)
	}

	out, err := SessionSearchExecute(map[string]any{"query": "独门关键词"}, st)
	if err != nil {
		t.Fatalf("search 执行失败: %v", err)
	}
	if !strings.Contains(out, "无命中") {
		t.Fatalf("archived 会话不应被 search 命中:\n%s", out)
	}
	if strings.Contains(out, "▶ 恢复该段上下文") {
		t.Fatalf("无命中时不应有恢复指针:\n%s", out)
	}
	t.Logf("✓ search: archived 会话被排除\n%s", out)
}

// TestSessionSearchLimitOverride — 可选 limit 覆盖默认条数（search 命中数 / read 窗口 / browse 列表）
func TestSessionSearchLimitOverride(t *testing.T) {
	st := newTestStore(t)
	se, err := st.CreateSession("example-35b-v2", "desktop", "", "限量会话")
	if err != nil {
		t.Fatal(err)
	}
	ids := ssAddMsgs(t, st, se.ID, 1757500000.0,
		"限量关键词 一", "限量关键词 二", "限量关键词 三", "限量关键词 四", "限量关键词 五")

	// search: limit=2 → 最多 2 条命中
	out, err := SessionSearchExecute(map[string]any{"query": "限量关键词", "limit": float64(2)}, st)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(ssExtractIDs(out)); got != 2 {
		t.Fatalf("search limit=2 应最多 2 条命中，实得 %d:\n%s", got, out)
	}

	// read(around_id, limit=2) → 目标前 2 + 目标 + 后 2 = 5 条
	out2, err := SessionSearchExecute(map[string]any{"session_id": se.ID, "around_id": float64(ids[2]), "limit": float64(2)}, st)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(ssExtractIDs(out2)); got != 5 {
		t.Fatalf("read limit=2 应取 5 条窗口，实得 %d:\n%s", got, out2)
	}

	// browse: limit=1 → 只列 1 个会话
	out3, err := SessionSearchExecute(map[string]any{"limit": float64(1)}, st)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out3, "最近 1 个会话") {
		t.Fatalf("browse limit=1 应只列 1 个会话:\n%s", out3)
	}
	t.Logf("✓ limit 覆盖: search=2 / read=5 / browse=1")
}
