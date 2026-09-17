// grants_test.go — T5.3 实证：授权粒度三档（once / session / project + TTL）、可枚举、可吊销、
// 可序列化重启继承。**六条用例逐条钉死**，且每条都带反例或对侧断言
// （只断"该不认的不认"会漏掉"把一切都拒了"这种假绿 —— 与 T5.1/T5.2 用例同一纪律）。
//
// 纪律：now 一律**注入**（不用 time.Now）—— 时间由调用方给，判定才是纯函数、才可回放。
// 本包不碰文件系统、不 import 任何业务包。
package policy

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// ── 夹具 ────────────────────────────────────────────────────────────────────

// grantNow — 用例的固定时点（全部判定都以它为基准）。
func grantNow() time.Time { return time.Date(2026, 9, 17, 10, 0, 0, 0, time.UTC) }

// grantCtx — 固定上下文：会话 sess-1、项目 /repo/zerg。
func grantCtx() GrantContext {
	return GrantContext{SessionID: "sess-1", ProjectID: "/repo/zerg"}
}

// mustIssue — 记账（失败即 Fatal：这些前置记账不该失败）。
func mustIssue(t *testing.T, s GrantStore, g Grant, now time.Time) Grant {
	t.Helper()
	got, err := s.Issue(g, now)
	if err != nil {
		t.Fatalf("Issue(%s/%s, %s) 不应失败：%v", g.Tool, g.Specifier, g.Scope, err)
	}
	if got.ID == "" {
		t.Fatalf("Issue 返回的记录没有 id（可吊销性的前提）：%+v", got)
	}
	return got
}

// grantIDs — 枚举面里当前有哪些 id。
func grantIDs(s GrantStore) []string {
	var out []string
	for _, g := range s.List() {
		out = append(out, g.ID)
	}
	return out
}

// mustJSON — 序列化（失败即 Fatal）。
func mustJSON(t *testing.T, s GrantStore) []byte {
	t.Helper()
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("账本序列化失败：%v", err)
	}
	return b
}

// ── ① once：用一次就作废 ────────────────────────────────────────────────────

func TestGrants_OnceIsUsedUpAfterFirstUse(t *testing.T) {
	now := grantNow()
	s := NewMemoryGrantStore(grantCtx())
	g := mustIssue(t, s, Grant{Tool: "bash", Specifier: "rm -rf /tmp/build", Scope: ScopeOnce,
		IssuedBy: "user", RuleID: "never.rm"}, now)

	// 正例：第一次判定通过，并且**用掉**（Consume 是"判定 + 记账"同一个动作）。
	used, ok := s.Consume("bash", "rm -rf /tmp/build", now)
	if !ok {
		t.Fatal("once 授权第一次就应当被认（正例）")
	}
	if used.ID != g.ID {
		t.Fatalf("Consume 返回的记录 id = %q，期望 %q（接线批要靠它回显是哪一条在放行）", used.ID, g.ID)
	}

	// 反例：同一条、同一时点、第二次 —— 必不认（once 绝不复用）。
	if _, ok := s.Consume("bash", "rm -rf /tmp/build", now); ok {
		t.Error("once 授权第二次仍被认 ⇒ 被复用（once 的全部意义就是不复用）")
	}
	if s.IsGranted("bash", "rm -rf /tmp/build", now) {
		t.Error("once 用掉之后 IsGranted 仍为真 ⇒ 只读查询与消费口径不一致")
	}
	// 事实仍可枚举：用掉了不等于抹掉（用户要能看到"这一条已用，不再生效"）。
	list := s.List()
	if len(list) != 1 || !list[0].Used {
		t.Errorf("用掉的 once 授权应仍在枚举面里且标记 Used：%+v", list)
	}
	// 反例：另一个匹配式**不被**这条 once 覆盖（逐字匹配，不放宽）。
	if s.IsGranted("bash", "rm -rf /tmp/other", now) {
		t.Error("once 授权覆盖到了别的匹配式 ⇒ 授权面被悄悄放大（逐字匹配是硬要求）")
	}
}

// ── ② session：本会话内有效，跨会话无效 ─────────────────────────────────────

func TestGrants_SessionValidInItsSessionOnly(t *testing.T) {
	now := grantNow()
	s := NewMemoryGrantStore(grantCtx())
	mustIssue(t, s, Grant{Tool: "read", Specifier: "./.env", Scope: ScopeSession,
		IssuedBy: "user", RuleID: "never.secret.dotenv_read"}, now)

	// 正例：本会话内、任意次（session 不是一次性的：同一条连查三次都认）。
	for i := 0; i < 3; i++ {
		if !s.IsGranted("read", "./.env", now) {
			t.Fatalf("session 授权在本会话内第 %d 次查询就失效了（session 不是一次性）", i+1)
		}
	}

	// 反例：换一个会话 —— 同一条记录（用序列化真搬过去）必不认。
	data := mustJSON(t, s)
	other, rep, err := LoadGrants(data, GrantContext{SessionID: "sess-2", ProjectID: "/repo/zerg"}, now)
	if err != nil {
		t.Fatalf("载入到另一个会话不应报错（只是不恢复那条授权）：%v", err)
	}
	if other.IsGranted("read", "./.env", now) {
		t.Error("session 授权跨会话仍然生效 ⇒ 授权会随会话漂移（用户以为只在本会话放开）")
	}
	if rep.Total != 1 || rep.Kept != 0 || rep.DroppedScope != 1 {
		t.Errorf("跨会话载入的过滤记账不对：%s（期望 共 1 / 恢复 0 / 归属不符 1）", rep)
	}

	// 反例第二面：没有会话归属的账本里，session 授权**记不进去**（记了也永不生效 ⇒ 直接报错）。
	if _, err := NewMemoryGrantStore(GrantContext{ProjectID: "/repo/zerg"}).
		Issue(Grant{Tool: "read", Specifier: "./.env", Scope: ScopeSession, IssuedBy: "user"}, now); err == nil {
		t.Error("没有会话归属却记了 session 授权 ⇒ 静默失效（这类条目永远不该入账）")
	}
}

// ── ③ project + TTL：到期即不认（含边界）────────────────────────────────────

func TestGrants_ProjectTTLExpiry(t *testing.T) {
	now := grantNow()
	ttl := now.Add(30 * time.Minute)
	s := NewMemoryGrantStore(grantCtx())
	mustIssue(t, s, Grant{Tool: "bash", Specifier: "npm install*", Scope: ScopeProject,
		ExpiresAt: ttl, IssuedBy: "user", RuleID: "never.pkg.npm_install"}, now)

	// 正例：到期前 1ns 仍认（不是"一旦有 TTL 就立刻失效"）。
	if !s.IsGranted("bash", "npm install*", ttl.Add(-time.Nanosecond)) {
		t.Error("TTL 未到就失效了 ⇒ TTL 语义反了（用户会被无谓地重新追问）")
	}
	// 边界：**恰等于到期时刻**即不认（判据是 now.Before(ExpiresAt)，边界取"不认"= fail-closed）。
	if s.IsGranted("bash", "npm install*", ttl) {
		t.Error("恰在到期时刻仍然认 ⇒ TTL 边界偏宽（等于把 TTL 悄悄延长）")
	}
	// 反例：到期后必不认（**这条就是 T5.3 的验收：TTL 到期 ⇒ 重新询问**）。
	if s.IsGranted("bash", "npm install*", ttl.Add(time.Hour)) {
		t.Error("TTL 到期后仍然生效 ⇒ 过期授权被当长期授权用（授权粒度失去意义）")
	}
	// 过期条目不进"有效"面，但**仍是事实**：枚举里看得到它（含失效时间），Prune 才收起。
	list := s.List()
	if len(list) != 1 || !list[0].ExpiresAt.Equal(ttl) {
		t.Errorf("过期条目的失效时间应能回显（用户要看到「什么时候作废」）：%+v", list)
	}
	if n := s.Prune(ttl); n != 1 {
		t.Errorf("Prune 在到期时点应清掉 1 条，实际清掉 %d 条", n)
	}
	if got := s.List(); len(got) != 0 {
		t.Errorf("Prune 之后枚举面应为空（失效事实已收起）：%+v", got)
	}
	// 反例：记账侧的 fail-closed —— 一开始就无效的授权不许成账。
	if _, err := s.Issue(Grant{Tool: "bash", Specifier: "npm install*", Scope: ScopeProject,
		ExpiresAt: now.Add(-time.Second), IssuedBy: "user"}, now); err == nil {
		t.Error("已过期的授权还能入账 ⇒ 账本里会出现「一开始就无效」的条目")
	}
	if _, err := s.Issue(Grant{Tool: "", Scope: ScopeProject, IssuedBy: "user"}, now); err == nil {
		t.Error("工具名为空的授权还能入账 ⇒ 判不了的东西不该成账")
	}
	if _, err := s.Issue(Grant{Tool: "bash", Scope: ScopeProject}, now); err == nil {
		t.Error("issued_by 为空的授权还能入账 ⇒ 授权必须能回答「谁批的」（审计层的前提）")
	}
	if _, err := s.Issue(Grant{Tool: "bash", Scope: "forever", IssuedBy: "user"}, now); err == nil {
		t.Error("不认识的粒度还能入账 ⇒ 粒度取值必须三档之一（不猜）")
	}
}

// ── ④ Revoke：吊销**立刻**生效 ──────────────────────────────────────────────

func TestGrants_RevokeTakesEffectImmediately(t *testing.T) {
	now := grantNow()
	s := NewMemoryGrantStore(grantCtx())
	a := mustIssue(t, s, Grant{Tool: "write", Specifier: "./gateway/fleet.yaml", Scope: ScopeProject, IssuedBy: "user"}, now)
	b := mustIssue(t, s, Grant{Tool: "bash", Specifier: "terraform destroy*", Scope: ScopeProject, IssuedBy: "user"}, now)

	if !s.IsGranted("write", "./gateway/fleet.yaml", now) || !s.IsGranted("bash", "terraform destroy*", now) {
		t.Fatal("两条授权都该在生效（前置条件不成立）")
	}
	if !s.Revoke(a.ID) {
		t.Fatalf("Revoke(%q) 应返回 true", a.ID)
	}

	// 反例：被吊销的那条**同一时点**立刻不认（不用等 TTL）。
	if s.IsGranted("write", "./gateway/fleet.yaml", now) {
		t.Error("吊销后仍然生效 ⇒ 吊而不销")
	}
	if _, ok := s.Consume("write", "./gateway/fleet.yaml", now); ok {
		t.Error("吊销后 Consume 仍能拿到授权 ⇒ 两条查询口径不一致")
	}
	// 对侧：另一条**不许**被连带吊销（按 id 精确吊销，不是"清空整个账本"）。
	if !s.IsGranted("bash", "terraform destroy*", now) {
		t.Error("吊销一条时把别的授权一起吊销了 ⇒ 吊销面过宽")
	}
	// 枚举面里也不该再看到它（撤了就该看不见，用户才信）。
	if ids := strings.Join(grantIDs(s), ","); strings.Contains(ids, a.ID) {
		t.Errorf("吊销后的条目仍在枚举面里：%v", grantIDs(s))
	}
	if n := len(s.List()); n != 1 {
		t.Errorf("吊销后应剩 1 条，实际 %d 条", n)
	}
	// 反例：吊销不存在的 id / 空 id 都返回 false（不 panic、不误伤）。
	if s.Revoke(b.ID+"-不存在") || s.Revoke("") {
		t.Error("吊销不存在的 id 返回了 true ⇒ 调用方会以为撤销成功了")
	}
	if !s.IsGranted("bash", "terraform destroy*", now) {
		t.Error("吊销一个不存在的 id 却把别的授权弄丢了")
	}
}

// ── ⑤ 序列化 ⇒ 反序列化：重启继承（有效的不丢、失效的不恢复）──────────────

func TestGrants_SerdeRestartInheritance(t *testing.T) {
	now := grantNow()
	s := NewMemoryGrantStore(grantCtx())
	live := mustIssue(t, s, Grant{Tool: "read", Specifier: "./src/**", Scope: ScopeProject,
		IssuedBy: "user", RuleID: "never.secret.ssh_read"}, now)
	expiring := mustIssue(t, s, Grant{Tool: "bash", Specifier: "go build*", Scope: ScopeSession,
		ExpiresAt: now.Add(time.Minute), IssuedBy: "user"}, now)
	onceUsed := mustIssue(t, s, Grant{Tool: "bash", Specifier: "git status", Scope: ScopeOnce, IssuedBy: "user"}, now)
	if _, ok := s.Consume("bash", "git status", now); !ok {
		t.Fatal("前置：once 授权应先被消费掉")
	}

	data := mustJSON(t, s)
	// 序列化是"事实全量"：用掉的那条**仍在文件里**（带 Used 标记），是**载入**时才按失效过滤。
	var st GrantState
	if err := json.Unmarshal(data, &st); err != nil {
		t.Fatalf("序列化产物应是合法的 GrantState：%v", err)
	}
	if len(st.Grants) != 3 || st.Version != GrantStateVersion || st.NextSeq != 4 {
		t.Fatalf("序列化产物应是「3 条事实 + 版本 %d + 水位 4」：%+v", GrantStateVersion, st)
	}
	foundUsed := false
	for _, g := range st.Grants {
		if g.ID == onceUsed.ID {
			foundUsed = true
			if !g.Used {
				t.Error("用掉的 once 授权在序列化产物里没有 Used 标记 ⇒ 载入侧无从判断该不该恢复")
			}
		}
	}
	if !foundUsed {
		t.Errorf("序列化把用掉的 once 条目抹掉了（事实不该在写出时丢）：%+v", st.Grants)
	}

	// 重启：**同一会话/同一项目** + 过了一分钟（那条 session 授权已经过期）。
	after := now.Add(time.Minute)
	re, rep, err := LoadGrants(data, grantCtx(), after)
	if err != nil {
		t.Fatalf("同上下文重启后载入不应失败：%v", err)
	}

	// 正例：有效的那条**完整恢复**（连失效时间、issued_by、rule_id 都一字不差）。
	got, ok := re.Lookup("read", "./src/**", after)
	if !ok {
		t.Fatal("重启后有效的 project 授权丢了 ⇒ 「跨重启存活」没做到（用户会被重新追问）")
	}
	if got.ID != live.ID || got.IssuedBy != live.IssuedBy || got.Scope != live.Scope || got.RuleID != live.RuleID {
		t.Errorf("恢复的记录与原文不一致：got=%+v want=%+v", got, live)
	}
	// 反例：到期的一条**不恢复**（重启不是"把过期的东西复活"）。
	if re.IsGranted("bash", "go build*", after) {
		t.Error("重启后把已过期的授权恢复了 ⇒ 重启成了延长授权的手段")
	}
	// 反例：用掉的一次性授权**不恢复**。
	if re.IsGranted("bash", "git status", after) {
		t.Error("重启后把已用掉的 once 授权复活了 ⇒ once 变成「每次重启送一次」")
	}
	if rep.Total != 3 || rep.Kept != 1 || rep.DroppedExpired != 1 || rep.DroppedUsed != 1 {
		t.Errorf("载入记账不对：%s（期望 共 3 / 恢复 1 / 过期 1 / 已用 1）", rep)
	}

	// 重启后新授权的编号必须**接着往下走**（不能与旧 id 撞车，否则吊销会吊销错一条）。
	fresh := mustIssue(t, re, Grant{Tool: "bash", Specifier: "go test*", Scope: ScopeProject, IssuedBy: "user"}, after)
	if fresh.ID == expiring.ID || fresh.Seq <= expiring.Seq {
		t.Errorf("重启后新授权复用了旧编号（新 id=%s，旧 id=%s）⇒ 吊销可能吊销错一条", fresh.ID, expiring.ID)
	}

	// 再序列化一轮再载入：**往返稳定**（不因为多走一遍就掉条目）。
	re2, rep2, err := LoadGrants(mustJSON(t, re), grantCtx(), after)
	if err != nil {
		t.Fatalf("二次载入失败：%v", err)
	}
	if !re2.IsGranted("read", "./src/**", after) || !re2.IsGranted("bash", "go test*", after) {
		t.Errorf("往返一趟后授权丢了：%s", rep2)
	}

	// 反例（fail-closed）：坏 JSON / 不认识的版本 / 编号水位不自洽 ⇒ **报错**，不静默当空账本。
	if _, _, err := LoadGrants([]byte("{不是 JSON"), grantCtx(), now); err == nil {
		t.Error("坏 JSON 载入没有报错 ⇒ 会静默变成「空账本 = 全都要重新问」或更坏")
	}
	badVer, _ := json.Marshal(GrantState{Version: GrantStateVersion + 1, NextSeq: 1})
	if _, _, err := LoadGrants(badVer, grantCtx(), now); err == nil {
		t.Error("不认识的账本版本没有报错 ⇒ 按未知格式解析（等于猜）")
	}
	badSeq, _ := json.Marshal(GrantState{Version: GrantStateVersion, NextSeq: 1, Grants: []Grant{
		{ID: "project:read(./a)#7", Tool: "read", Specifier: "./a", Scope: ScopeProject,
			ProjectID: "/repo/zerg", IssuedBy: "user", Seq: 7},
	}})
	if _, _, err := LoadGrants(badSeq, grantCtx(), now); err == nil {
		t.Error("编号水位 ≤ 最大序号却没有报错 ⇒ 重启后会发出撞车的 id")
	}
}

// ── ⑥ 可枚举 + 匹配口径（不放宽）────────────────────────────────────────────

func TestGrants_ListIsEnumerableAndMatchingIsExact(t *testing.T) {
	now := grantNow()
	s := NewMemoryGrantStore(grantCtx())
	whole := mustIssue(t, s, Grant{Tool: "BASH", Scope: ScopeProject, IssuedBy: "user", RuleID: "never.rm"}, now) // 整工具（无匹配式）
	mustIssue(t, s, Grant{Tool: "bash", Specifier: "cat a.txt", Scope: ScopeProject, IssuedBy: "user"}, now)      // 具体一条
	other := mustIssue(t, s, Grant{Tool: "read", Specifier: "./.env", Scope: ScopeSession, IssuedBy: "ci", Note: "夜间跑"}, now)

	// 可枚举：三条都能数出来，且带「谁批的 / 批了什么 / 什么粒度 / 何时作废 / 哪条规则」。
	list := s.List()
	if len(list) != 3 {
		t.Fatalf("枚举面应有 3 条，实际 %d 条：%+v", len(list), list)
	}
	seen := map[string]Grant{}
	for _, g := range list {
		seen[g.ID] = g
		if g.IssuedBy == "" || !g.Scope.Valid() || g.Tool == "" || g.Seq == 0 || g.ID != grantID(g, g.Seq) {
			t.Errorf("枚举出来的记录不自洽（用户/审计要能回答「谁批的、批了什么、什么粒度」）：%+v", g)
		}
	}
	if seen[whole.ID].Tool != "bash" { // 工具名归一化（大小写不敏感）
		t.Errorf("入账时工具名没有归一化：%q", seen[whole.ID].Tool)
	}
	if seen[other.ID].SessionID != "sess-1" || seen[whole.ID].ProjectID != "/repo/zerg" {
		t.Errorf("归属没有按上下文写进记录：%+v / %+v", seen[other.ID], seen[whole.ID])
	}
	// 回显是副本：改返回值不许污染账本（否则 UI 一次手滑就把授权改没了）。
	list[0].Tool = "被改了"
	if s.List()[0].Tool == "被改了" {
		t.Error("List 返回的是内部切片 ⇒ 调用方改返回值会污染账本")
	}

	// 匹配正例一：无匹配式 = 该工具的任何调用都覆盖。
	if !s.IsGranted("bash", "rm -rf /whatever", now) {
		t.Error("整工具授权没有覆盖该工具的调用（无匹配式 = 任何调用）")
	}
	// 匹配正例二：逐字相同命中；两侧空白归一后也命中（否则「用户批了但没生效」）。
	narrow := NewMemoryGrantStore(grantCtx())
	mustIssue(t, narrow, Grant{Tool: "bash", Specifier: "cat a.txt", Scope: ScopeProject, IssuedBy: "user"}, now)
	if !narrow.IsGranted(" bash ", " cat a.txt ", now) {
		t.Error("首尾空白（工具名/匹配式两侧）应归一后仍命中（正例不成立）")
	}
	// 反例：有匹配式时**逐字**算 —— 批准 `cat a.txt` **不许**覆盖别的任何东西。
	for _, spec := range []string{"cat b.txt", "cat a.txt.bak", "rm -rf /", "cat  a.txt", "cat", ""} {
		if narrow.IsGranted("bash", spec, now) {
			t.Errorf("批准 `cat a.txt` 却放行了 `%s` ⇒ 授权面被悄悄放大（逐字匹配是硬要求）", spec)
		}
	}
	if narrow.IsGranted("bashx", "cat a.txt", now) {
		t.Error("工具名不同却命中了（工具名必须逐字相等）")
	}
}
