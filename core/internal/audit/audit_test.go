// audit_test.go — T5.9 实证：**独立审计层**（与人批同寿 · append-only 哈希链 · 写入前必经脱敏）。
//
// 五条用例（每条都带反例或对侧断言 —— 只断"该拒的拒了"会漏掉"把一切都拒了"这种假绿）：
//
//	⑥ 改一条历史 ⇒ `Verify` 报错（删一条 / 换序也报错；复原后必须重新自洽）
//	⑦ 序号单调且不跳（链逐条接上；调用方自填 seq/hash/prev_hash 被拒；幂等键只查询不拒写）
//	⑧ **反例**：未接脱敏钩子时 `Append` 必须报错（防偷工）；脱敏器报错/删键/加键同样拒写
//	⑨ 审计层**不随 `ZERG_DEBUG_LEVEL` 关闭**而消失（运行时 + 源码守卫两条证据）
//	⑩ 脱敏发生在**哈希之前**（落库与导出都不含原文；入参指纹与 gen_ai.* 对齐可查）
//
// 纪律：At 一律**注入**（不用 time.Now）—— 本包不读系统钟、不读环境变量，于是"记了什么"
// 完全由输入决定（可测、可回放、可核对）。用例用**假脱敏器**（真脱敏器在 internal/obs/redact，
// 接线批接它 —— 本包只留接口）。
package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ── 夹具 ────────────────────────────────────────────────────────────────────

// auditT0 — 用例的固定时点。
func auditT0() time.Time { return time.Date(2026, 9, 17, 11, 0, 0, 0, time.UTC) }

// testRedactor — 假脱敏器：把 mask 子串遮成 ***，可注入"报错 / 删键 / 加键"三种反例行为。
type testRedactor struct {
	name string
	mask string
	drop string // 要"删掉"的键（反例）
	add  bool   // 要凭空加一个键（反例）
	fail bool   // 要报错（反例）
}

func (r *testRedactor) Name() string {
	if r.name == "" {
		return "test-redactor/v1"
	}
	return r.name
}

func (r *testRedactor) Redact(fields map[string]string) (map[string]string, error) {
	if r.fail {
		return nil, fmt.Errorf("脱敏器内部错误（用例注入）")
	}
	out := make(map[string]string, len(fields))
	for k, v := range fields {
		if r.drop != "" && k == r.drop {
			continue // 删键
		}
		if r.mask != "" {
			v = strings.ReplaceAll(v, r.mask, "***")
		}
		out[k] = v
	}
	if r.add {
		out["偷加的字段"] = "x"
	}
	return out, nil
}

// newTestStore — 账本 + 假脱敏器（mask=hunter2，用来验证"原文没进记录"）。
func newTestStore(t *testing.T) (*Store, *testRedactor) {
	t.Helper()
	red := &testRedactor{mask: "hunter2"}
	return NewStore(red), red
}

// mustAppend — 记一条（失败即 Fatal：这些前置记录不该失败）。
func mustAppend(t *testing.T, s *Store, r Record) Record {
	t.Helper()
	got, err := s.Append(r)
	if err != nil {
		t.Fatalf("Append(%s/%s) 不应失败：%v", r.Decision, r.IdempotencyKey, err)
	}
	if got.Seq == 0 || got.Hash == "" || got.PrevHash == got.Hash {
		t.Fatalf("落库后的记录字段不对（seq/hash 必须由账本填）：%+v", got)
	}
	return got
}

// auditRec — 第 n 条样例记录（同一串输入必然得到同样的记录 —— 便于核对）。
func auditRec(n int, dec Decision) Record {
	return Record{
		At:             auditT0().Add(time.Duration(n) * time.Second),
		Subject:        "sess-1",
		Tool:           "bash",
		ArgsDigest:     "sha256:2656ff0e0000000000000000000000000000000000000000000000000000ffff",
		Decision:       dec,
		RuleID:         "never.rm",
		Mode:           "default",
		ModeEffect:     "default:无命中⇒ask（档位默认不是 allow）",
		IdempotencyKey: fmt.Sprintf("sess-1:bash#%d", n),
		Note:           "地板条目 never.rm：rm 永不自动批，需人批",
	}
}

// backupRecords / restoreRecords — 用例直接篡改账本内部（同包）：先备份、后复原。
func backupRecords(s *Store) []Record {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Record(nil), s.records...)
}

func restoreRecords(s *Store, backup []Record) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = append([]Record(nil), backup...)
}

// ── ⑥ 改一条历史 ⇒ Verify 报错 ─────────────────────────────────────────────

func TestAudit_TamperedHistoryFailsVerify(t *testing.T) {
	s, red := newTestStore(t)
	for i := 1; i <= 3; i++ {
		mustAppend(t, s, auditRec(i, DecisionAsk))
	}
	if err := s.Verify(); err != nil {
		t.Fatalf("刚写完的账本必须自洽（否则下面的红分不清是谁的锅）：%v", err)
	}
	backup := backupRecords(s)

	// ① 改写一条历史：把"要问人"改成"放行"——正是最诱人的那一改。
	s.mu.Lock()
	s.records[0].Decision = DecisionAllow
	s.mu.Unlock()
	err := s.Verify()
	if err == nil {
		t.Fatal("改写历史后 Verify 竟通过 ⇒ append-only 形同虚设")
	}
	if !strings.Contains(err.Error(), "seq=1") || !strings.Contains(err.Error(), "改写") {
		t.Errorf("报错必须指出是哪一条、为什么（取证靠它）：%v", err)
	}
	restoreRecords(s, backup)
	if err := s.Verify(); err != nil {
		t.Fatalf("复原历史后必须重新自洽（对侧断言：刚才的红不是账本本来就坏）：%v", err)
	}

	// ② 删掉中间一条（append-only 的另一半：不许消失）。
	s.mu.Lock()
	s.records = append(s.records[:1], s.records[2:]...)
	s.mu.Unlock()
	if err := s.Verify(); err == nil {
		t.Error("删掉中间一条后 Verify 竟通过 ⇒ 记录消失无人发现")
	}
	restoreRecords(s, backup)
	if err := s.Verify(); err != nil {
		t.Fatalf("复原后必须自洽：%v", err)
	}

	// ③ 换序（把"要问人"那条挪到后面去）。
	s.mu.Lock()
	s.records[0], s.records[1] = s.records[1], s.records[0]
	s.mu.Unlock()
	if err := s.Verify(); err == nil {
		t.Error("换序后 Verify 竟通过 ⇒ 顺序可被改写")
	}
	restoreRecords(s, backup)

	// ④ 外部路径同样被挡：篡改后的账本**载入即被拒**（坏链不许接回服务）。
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("序列化失败：%v", err)
	}
	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		t.Fatalf("反序列化失败：%v", err)
	}
	st.Records[2].Decision = DecisionApprove // 把一次"要问人"洗成"某人批过"
	tampered, err := json.Marshal(st)
	if err != nil {
		t.Fatalf("序列化篡改版失败：%v", err)
	}
	if _, err := LoadAudit(tampered, red); err == nil {
		t.Error("篡改后的账本竟能载入 ⇒ 审计可以被「洗白」")
	}
	// 对侧：未篡改的能载入、能校验、能接着记（载入不是只读摆设）。
	loaded, err := LoadAudit(data, red)
	if err != nil {
		t.Fatalf("未篡改的账本必须能载入：%v", err)
	}
	if err := loaded.Verify(); err != nil {
		t.Errorf("载入后的账本必须自洽：%v", err)
	}
	if got := mustAppend(t, loaded, auditRec(4, DecisionApprove)); got.Seq != 4 {
		t.Errorf("载入后接着记必须从尾部续上序号：%d", got.Seq)
	}
}

// ── ⑦ 序号单调且不跳 ───────────────────────────────────────────────────────

func TestAudit_SeqMonotonicAndContiguous(t *testing.T) {
	s, _ := newTestStore(t)
	for i := 1; i <= 5; i++ {
		got := mustAppend(t, s, auditRec(i, DecisionAsk))
		if got.Seq != int64(i) {
			t.Fatalf("第 %d 条的 seq=%d（必须 1 起、单调、连续、不跳）", i, got.Seq)
		}
		if i == 1 && got.PrevHash != "" {
			t.Errorf("第一条的 prev_hash 必须为空：%q", got.PrevHash)
		}
	}
	all := s.Read(0)
	if len(all) != 5 || s.Len() != 5 {
		t.Fatalf("账本应有 5 条：Read=%d Len=%d", len(all), s.Len())
	}
	for i, r := range all {
		if r.Seq != int64(i+1) {
			t.Errorf("第 %d 条 seq=%d", i+1, r.Seq)
		}
		if i > 0 && r.PrevHash != all[i-1].Hash {
			t.Errorf("链没接上：seq=%d 的 prev_hash=%s ≠ 上一条 hash=%s", r.Seq, r.PrevHash, all[i-1].Hash)
		}
		if r.Hash == "" {
			t.Errorf("seq=%d 没有哈希（append-only 的链断了）：%+v", r.Seq, r)
		}
	}
	if n := s.Read(2); len(n) != 2 || n[0].Seq != 1 || n[1].Seq != 2 {
		t.Errorf("Read(2) 应给前两条：%+v", n)
	}
	if len(s.Read(-1)) != 5 || len(s.Read(99)) != 5 {
		t.Error("n<=0 或 n 超过总数时应给全部")
	}
	if err := s.Verify(); err != nil {
		t.Fatalf("链必须自洽：%v", err)
	}

	// 反例：调用方自填 seq/hash/prev_hash/redactor ⇒ 拒写
	//（否则"改历史"可以从 Append 接口绕进来，append-only 就只剩一半）。
	for _, bad := range []struct {
		why string
		mut func(*Record)
	}{
		{"自填 seq", func(r *Record) { r.Seq = 42 }},
		{"自填 prev_hash", func(r *Record) { r.PrevHash = "sha256:00" }},
		{"自填 hash", func(r *Record) { r.Hash = "sha256:00" }},
		{"自填 redactor", func(r *Record) { r.Redactor = "伪造的脱敏器" }},
	} {
		r := auditRec(9, DecisionAllow)
		bad.mut(&r)
		if _, err := s.Append(r); err == nil {
			t.Errorf("%s：竟被接受（这些字段只能由账本写）", bad.why)
		}
	}
	if s.Len() != 5 {
		t.Errorf("反例不该改动账本：%d 条", s.Len())
	}

	// 幂等键：**重复不拒写**（重放/重问都是真的发生过），但查询必须给"首次出现"的那条。
	dup := auditRec(10, DecisionAsk)
	dup.IdempotencyKey = all[0].IdempotencyKey
	if _, err := s.Append(dup); err != nil {
		t.Fatalf("幂等键重复不该拒写（拒写 = 丢事实）：%v", err)
	}
	if s.Len() != 6 {
		t.Errorf("重复键的记录也要落库：%d 条", s.Len())
	}
	if got, ok := s.LookupIdempotency(all[0].IdempotencyKey); !ok || got.Seq != 1 {
		t.Errorf("幂等键查询应给首次出现的那条：seq=%d ok=%v", got.Seq, ok)
	}
	if _, ok := s.LookupIdempotency("没出现过的键"); ok {
		t.Error("未知幂等键不该命中")
	}
	if _, ok := s.LookupIdempotency("   "); ok {
		t.Error("空幂等键不该命中")
	}

	// 反例：内部篡改（跳号 / 拆链）也必须被 Verify 抓到。
	backup := backupRecords(s)
	s.mu.Lock()
	s.records[3].Seq = 9
	s.mu.Unlock()
	if err := s.Verify(); err == nil {
		t.Error("seq 跳号后 Verify 竟通过")
	} else if !strings.Contains(err.Error(), "seq") {
		t.Errorf("跳号报错应点名 seq：%v", err)
	}
	restoreRecords(s, backup)
	s.mu.Lock()
	s.records[2].PrevHash = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
	s.mu.Unlock()
	if err := s.Verify(); err == nil {
		t.Error("链被拆后 Verify 竟通过")
	} else if !strings.Contains(err.Error(), "prev_hash") {
		t.Errorf("拆链报错应点名 prev_hash：%v", err)
	}
	restoreRecords(s, backup)
	if err := s.Verify(); err != nil {
		t.Fatalf("复原后必须自洽：%v", err)
	}
}

// ── ⑧ 反例：未接脱敏钩子 ⇒ Append 必须报错 ─────────────────────────────────

func TestAudit_AppendWithoutRedactorIsRefused(t *testing.T) {
	// 反例（本批"防偷工"的那一条）：没接脱敏器 ⇒ **拒写**。
	s := NewStore(nil)
	if _, err := s.Append(auditRec(1, DecisionAsk)); err == nil {
		t.Fatal("未接脱敏钩子时 Append 竟成功 ⇒ 生产可以在没有脱敏的情况下记审计（F14 的写入时脱敏形同虚设）")
	} else if !strings.Contains(err.Error(), "脱敏") {
		t.Errorf("报错必须点明是脱敏器没接：%v", err)
	}
	if s.Len() != 0 {
		t.Errorf("被拒的记录不该落库：%d 条", s.Len())
	}
	if s.RedactorName() != "" {
		t.Errorf("没接脱敏器时不该有脱敏器标识：%q", s.RedactorName())
	}
	// 对侧：空账本是**自洽**的（拒写不等于账本坏了）。
	if err := s.Verify(); err != nil {
		t.Errorf("空账本 Verify 必须通过：%v", err)
	}

	// 接上钩子后，**同一份记录**必须能记（证明刚才拒的是"没接钩子"，不是记录本身不合法）。
	red := &testRedactor{mask: "hunter2"}
	s.SetRedactor(red)
	if _, err := s.Append(auditRec(1, DecisionAsk)); err != nil {
		t.Fatalf("接上脱敏器后同一份记录应当能记：%v", err)
	}
	if s.Len() != 1 || s.RedactorName() != red.Name() {
		t.Errorf("落库状态不对：Len=%d redactor=%q", s.Len(), s.RedactorName())
	}

	// 脱敏器**报错** ⇒ 拒写（**绝不回退写原文**）。
	red.fail = true
	if _, err := s.Append(auditRec(2, DecisionAllow)); err == nil {
		t.Error("脱敏器报错时 Append 竟成功 ⇒ 回退写了原文")
	} else if !strings.Contains(err.Error(), "脱敏") {
		t.Errorf("报错必须点明脱敏环节：%v", err)
	}
	red.fail = false
	if s.Len() != 1 {
		t.Errorf("被拒的记录不该落库：%d 条", s.Len())
	}

	// 脱敏器**删键** ⇒ 拒写（读的人不许分不清"没采到"和"被删了"）。
	red.drop = "note"
	if _, err := s.Append(auditRec(2, DecisionAllow)); err == nil {
		t.Error("脱敏器删键时 Append 竟成功 ⇒ 字段集合可被静默改小")
	} else if !strings.Contains(err.Error(), "删掉") {
		t.Errorf("报错应点明是删键：%v", err)
	}
	red.drop = ""

	// 脱敏器**加键** ⇒ 拒写（不许往审计里塞未经审计的字段）。
	red.add = true
	if _, err := s.Append(auditRec(2, DecisionAllow)); err == nil {
		t.Error("脱敏器加键时 Append 竟成功 ⇒ 字段集合可被静默改大")
	} else if !strings.Contains(err.Error(), "加") {
		t.Errorf("报错应点明是加键：%v", err)
	}
	red.add = false
	if s.Len() != 1 {
		t.Errorf("三种反例都不该落库：%d 条", s.Len())
	}

	// 记录本身必须答得出的四件事：谁 / 凭什么 / 什么决策 / 幂等键（缺一 ⇒ 拒写）。
	for _, bad := range []struct {
		why string
		mut func(*Record)
	}{
		{"At 零值（时间没注入）", func(r *Record) { r.At = time.Time{} }},
		{"主体为空", func(r *Record) { r.Subject = "  " }},
		{"决策非法", func(r *Record) { r.Decision = Decision("probably-fine") }},
		{"依据全空（rule_id 与 judge 都空）", func(r *Record) { r.RuleID, r.Judge = "", "" }},
		{"幂等键为空", func(r *Record) { r.IdempotencyKey = " " }},
	} {
		r := auditRec(3, DecisionDeny)
		bad.mut(&r)
		if _, err := s.Append(r); err == nil {
			t.Errorf("%s：竟被接受（审计记录必须答得出这件事）", bad.why)
		}
	}
	if s.Len() != 1 {
		t.Errorf("字段级反例也不该落库：%d 条", s.Len())
	}
	// 对侧：依据只需**其中一个**（人批填 judge、规则判定填 rule_id）。
	r := auditRec(4, DecisionApprove)
	r.RuleID = ""
	r.Judge = "user"
	if _, err := s.Append(r); err != nil {
		t.Errorf("只有 judge 没有 rule_id 应被接受（人批的依据就是人）：%v", err)
	}
}

// ── ⑨ 审计层不随 ZERG_DEBUG_LEVEL 关闭而消失 ────────────────────────────────

func TestAudit_SurvivesDebugLevelOff(t *testing.T) {
	// ① 运行时证据：把调试级别拨到各档（含"关掉"的写法），审计照记、照校验、照导出。
	for _, lvl := range []string{"off", "0", "OFF", "quiet"} {
		t.Setenv("ZERG_DEBUG_LEVEL", lvl)
		s, _ := newTestStore(t)
		for i := 1; i <= 2; i++ {
			mustAppend(t, s, auditRec(i, DecisionApprove))
		}
		if err := s.Verify(); err != nil {
			t.Fatalf("调试级别=%s 时审计链不自洽：%v", lvl, err)
		}
		if got := s.Read(2); len(got) != 2 {
			t.Fatalf("调试级别=%s 时审计记录没记上（只剩 %d 条）⇒ 留痕挂在了调试开关上", lvl, len(got))
		}
		if b, err := s.ExportJSONL(); err != nil || len(b) == 0 {
			t.Fatalf("调试级别=%s 时导出为空：%v", lvl, err)
		}
	}

	// ② 源码守卫（更强的一条）：本包**根本不读环境变量** ⇒「挂到调试级别上」这件事
	// 在实现里不可能发生（哪怕将来有人想加，这条守卫先红）。
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("扫不到源码：%v", err)
	}
	scanned := 0
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("读 %s 失败：%v", f, err)
		}
		src := string(b)
		for _, forbidden := range []string{
			`os.Getenv(`,
			`os.LookupEnv(`,
			`os.Environ(`,
			`"ZERG_`, // 读环境变量必须写字面量：出现带引号的 ZERG_ 前缀即违规
		} {
			if strings.Contains(src, forbidden) {
				t.Errorf("%s 出现 %s ⇒ 审计层的生死被环境开关接管了（F14 要治的就是这个）", f, forbidden)
			}
		}
		scanned++
	}
	if scanned == 0 {
		t.Fatal("没扫到任何源码 ⇒ 这条守卫是空的（永远绿）")
	}
}

// ── ⑩ 脱敏发生在哈希之前 + 入参指纹 + gen_ai.* 对齐 ────────────────────────

func TestAudit_RedactorHookAppliedBeforeHashing(t *testing.T) {
	s, red := newTestStore(t)
	r0 := auditRec(1, DecisionApprove)
	r0.Judge = "user:hunter2@example.com"
	r0.Note = "用户在弹窗里批了 hunter2 那条命令"
	r0.ArgsDigest = "sha256:hunter2hunter2hunter2hunter2hunter2hunter2hunter2hunter2hunter2"
	r0.Attrs = map[string]string{"gen_ai.request.model": "hunter2-model"}
	got := mustAppend(t, s, r0)

	// ① 落库的记录里**不含原文**（脱敏在哈希之前 ⇒ 记录与哈希域里都没有原文）。
	for _, f := range []struct{ name, v string }{
		{"judge", got.Judge}, {"note", got.Note}, {"args_digest", got.ArgsDigest},
		{"attr.gen_ai.request.model", got.Attrs["gen_ai.request.model"]},
	} {
		if strings.Contains(f.v, "hunter2") {
			t.Errorf("%s 里还留着原文 ⇒ 脱敏没在写入前发生：%q", f.name, f.v)
		}
	}
	if got.Judge != "user:***@example.com" {
		t.Errorf("脱敏器只应改值、不改结构：judge=%q", got.Judge)
	}
	if got.Redactor != red.Name() {
		t.Errorf("记录必须带脱敏器标识（事后要能回答「这条是谁脱敏的」）：%q", got.Redactor)
	}
	// ② 导出面（给 SIEM 的那一份）同样不含原文。
	if b, err := s.ExportJSONL(); err != nil {
		t.Fatalf("导出失败：%v", err)
	} else if strings.Contains(string(b), "hunter2") {
		t.Error("导出里出现原文 ⇒ 脱敏没在写入前发生")
	}
	// ③ 链仍然自洽（脱敏不改变 append-only 的性质）。
	if err := s.Verify(); err != nil {
		t.Fatalf("脱敏后链必须自洽：%v", err)
	}
	// ④ 记录里的字段**确实被脱敏改写过**（对侧：防止"脱敏器根本没被调用"）。
	if got.Note == r0.Note {
		t.Error("note 一个字节都没变 ⇒ 脱敏钩子可能根本没被调用")
	}

	// ⑤ gen_ai.* 对齐（导出到 SIEM 用标准属性名）。
	ga := got.GenAIAttrs()
	for _, k := range []string{"gen_ai.operation.name", "gen_ai.tool.name", "gen_ai.tool.call.id", "gen_ai.agent.id"} {
		if _, ok := ga[k]; !ok {
			t.Errorf("缺标准属性 %s", k)
		}
	}
	if ga["gen_ai.tool.name"] != "bash" || ga["gen_ai.agent.id"] != "sess-1" {
		t.Errorf("标准属性取值不对：%+v", ga)
	}
	if _, ok := ga["zerg.decision"]; !ok {
		t.Error("非标准字段应加 zerg. 前缀进导出：缺 zerg.decision")
	}

	// ⑥ 入参指纹：同参数字典 ⇒ 同指纹（键序无关）；不同 ⇒ 不同；没有入参 ⇒ 空串。
	f1, err := ArgsFingerprint(map[string]any{"command": "rm -rf /tmp/x", "path": "./a"})
	if err != nil || f1 == "" {
		t.Fatalf("指纹算不出：%q %v", f1, err)
	}
	f2, err := ArgsFingerprint(map[string]any{"path": "./a", "command": "rm -rf /tmp/x"})
	if err != nil {
		t.Fatalf("指纹算不出：%v", err)
	}
	if f1 != f2 {
		t.Error("同一次调用（只是构造顺序不同）得到了两个指纹 ⇒ 审计对不上账")
	}
	f3, err := ArgsFingerprint(map[string]any{"command": "rm -rf /tmp/y"})
	if err != nil {
		t.Fatalf("指纹算不出：%v", err)
	}
	if f3 == f1 {
		t.Error("不同入参得到了同一个指纹")
	}
	if empty, err := ArgsFingerprint(nil); err != nil || empty != "" {
		t.Errorf("没有入参应返回空串而不是报错/编造：%q %v", empty, err)
	}
	if _, err := ArgsFingerprint(map[string]any{"bad": make(chan int)}); err == nil {
		t.Error("算不出指纹时应报错（不许把原文当指纹写进审计）")
	}
	if !strings.HasPrefix(f1, "sha256:") {
		t.Errorf("指纹口径应是 sha256 前缀（与 policy 的 ArgsDigest 同源）：%q", f1)
	}
}
