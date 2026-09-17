// effect_resume_test.go —— T4.3 的实证：**效果级幂等**（E4/E5）。
//
//	④ TestT43_PerEffectDedupAndStateHashes   逐效果判重 (call_id, effect_index) + 前后置状态哈希
//	                                          （状态不符 ⇒ 报"状态不匹配"，**不**继续）
//	⑤ TestT43_FileLedgerDurableAcrossReopen  去重存储两档：内存 + 本地文件（落盘再读仍判已发生；坏账本拒开；
//	                                          落盘失败 ⇒ 撤销认领，不得执行）
//	⑥ TestT43_PartialSuccessResumeOnlyPending "先写文件再发通知"部分成功 ⇒ **只补做没执行的那一条**
//	                                          （真文件 + 真 sha256 钉住"已完成的一条都没重做"）
//
// 纪律：每条都配反例 —— "判重永远返回已发生"（少做）与"判重永远返回新"（重做）都是假绿，两个方向都要挡。
package replay

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── ④ 逐效果判重 + 前后置状态哈希（E4/E5）─────────────────────────────────

func TestT43_PerEffectDedupAndStateHashes(t *testing.T) {
	dir := t.TempDir()
	norm := DefaultNormalizer(dir)
	args := map[string]any{"call_id": "c1", "command": "mkdir wd && echo a > wd/a.txt && notify"}
	canonical := mustNormalize(t, norm, "bash", args).Canonical
	scopes := []string{"fs:mkdir wd", "fs:write wd/a.txt", "proc:notify"}

	// 计划口径：与 Dispatcher 同款 —— 独立复算 effect_key，逐个钉住 (index, key, scope)
	plan := PlanCallEffects("bash", canonical, "c1", scopes, ModeRecord)
	if len(plan.Entries) != len(scopes) {
		t.Fatalf("一次调用 %d 个效果，实得 %d 条", len(scopes), len(plan.Entries))
	}
	for i, e := range plan.Entries {
		if want := indepEffectKey("bash", canonical, scopes[i]); e.Key != want {
			t.Errorf("第 %d 条效果键口径不符：%s vs 独立复算 %s", i, e.Key, want)
		}
		if e.Index != i || e.Scope != scopes[i] || e.CallID != "c1" {
			t.Errorf("第 %d 条效果的索引/范围/调用 id 不符：%+v", i, e)
		}
	}

	l := NewEffectLedger()
	// 首次认领：记下**前置**状态哈希
	claim, first, err := l.ClaimChecked(plan.Entries[0], "state-A")
	if err != nil || claim != EffectNew {
		t.Fatalf("首次认领应为 new 且无错：claim=%s err=%v", claim, err)
	}
	if first.PreconditionStateHash != "state-A" || first.ObservedAfterHash != "" {
		t.Errorf("首次认领应只记下前置状态：%+v", first)
	}
	// 逐效果判重：按 (call_id, effect_index) 查得到，且**只**有第 0 条
	if _, ok := l.AppliedIndex("c1", 0); !ok {
		t.Error("按 (call_id, effect_index) 应能查到第 0 条")
	}
	if _, ok := l.AppliedIndex("c1", 1); ok {
		t.Error("第 1 条没认领过，不该查到（否则「逐效果」退化成「整调用」）")
	}
	if _, ok := l.AppliedIndex("c2", 0); ok {
		t.Error("别的 call_id 的同一序号不该查到")
	}

	// 正常分支：同状态再来一次 ⇒ 已发生（恢复路径的正常分支，不是错误）
	claim2, again, err := l.ClaimChecked(plan.Entries[0], "state-A")
	if err != nil {
		t.Fatalf("状态相符时第二次认领不该报错：%v", err)
	}
	if claim2 != EffectAlreadyApplied || again.Key != first.Key || again.PreconditionStateHash != "state-A" {
		t.Errorf("应返回「已发生」并附**首次**那条事实：claim=%s entry=%+v", claim2, again)
	}
	if l.Len() != 1 {
		t.Errorf("已发生不得再认领其它条目，账本应仍为 1 条，实得 %d", l.Len())
	}

	// 反例（E5 核心）：**状态不符** ⇒ 报"状态不匹配"，而**不是**回一句"已发生"把效果悄悄跳过
	claim3, _, err := l.ClaimChecked(plan.Entries[0], "state-B")
	if err == nil {
		t.Fatal("前置状态不符必须报错 —— 否则回放会安静地跑在错误的状态上")
	}
	if !errors.Is(err, ErrStateMismatch) {
		t.Fatalf("应为 ErrStateMismatch，实得 %v", err)
	}
	if claim3 != EffectClaim("") {
		t.Errorf("状态不符时不许给出「已发生」这个结论（实得 %s）—— 那会让调用方直接跳过", claim3)
	}
	var sm *StateMismatchError
	if !errors.As(err, &sm) {
		t.Fatalf("应为 *StateMismatchError，实得 %T", err)
	}
	if !sm.IsEffect || sm.Want != "state-A" || sm.Got != "state-B" || sm.Index != 0 || sm.Scope != scopes[0] {
		t.Errorf("错误里应指明是哪条效果与两侧状态：%+v", sm)
	}
	for _, want := range []string{"状态不匹配", "state-A", "state-B"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("错误文本缺 %q：%s", want, err.Error())
		}
	}
	// 反例：录了前置、本次没给 ⇒ fail-closed（"无法校验"不等于"校验通过"）
	if _, _, err := l.ClaimChecked(plan.Entries[0], ""); !errors.Is(err, ErrStateMismatch) {
		t.Errorf("本次没给状态也应报不匹配（fail-closed），实得 %v", err)
	}
	// 只读校验（回放 / 补做前用）
	if err := l.VerifyState(first.Key, "state-A"); err != nil {
		t.Errorf("状态相符时 VerifyState 应通过：%v", err)
	}
	if err := l.VerifyState(first.Key, "state-Z"); !errors.Is(err, ErrStateMismatch) {
		t.Errorf("状态不符应报不匹配，实得 %v", err)
	}
	if err := l.VerifyState("sha256:不存在的键", "state-A"); !errors.Is(err, ErrEffectLedger) {
		t.Errorf("查不到的键应报 ErrEffectLedger（没记录 ≠ 状态对得上），实得 %v", err)
	}

	// 执行后：记 observed_after_hash（E5 的后半段）
	if err := l.Complete(first.Key, "after-1"); err != nil {
		t.Fatalf("收尾记状态应成功：%v", err)
	}
	if err := l.Complete(first.Key, "after-1"); err != nil {
		t.Errorf("同一份事实重复收尾应幂等：%v", err)
	}
	// 反例：同一个效果出现两个不同的"执行后状态" ⇒ 它被重做过 ⇒ 必须报错（不许覆盖成最后一次）
	if err := l.Complete(first.Key, "after-2"); err == nil {
		t.Error("同一效果两个执行后状态 = 被重做过，必须报错而不是覆盖")
	}
	// 反例：空哈希 ⇒ 报错（"没记到"与"记了个空"是两件事）
	if err := l.Complete(first.Key, "   "); err == nil {
		t.Error("空 observed_after_hash 应报错")
	}
	// 反例：没认领就收尾 ⇒ 报错（执行与认领脱节）
	if err := l.Complete(plan.Entries[1].Key, "after-x"); !errors.Is(err, ErrEffectLedger) {
		t.Errorf("未认领的键收尾应报 ErrEffectLedger，实得 %v", err)
	}
	// 反例：已收尾的不许 Release（它确实执行过）
	if err := l.Release(first.Key); err == nil {
		t.Error("已记下执行后状态的效果不许释放（否则会被重做）")
	}

	// 可序列化往返：两个状态哈希都要在（前后置是事实的一部分，不是运行期缓存）
	blob, err := l.MarshalLedger()
	if err != nil {
		t.Fatalf("MarshalLedger：%v", err)
	}
	loaded, err := LoadEffectLedger(blob)
	if err != nil {
		t.Fatalf("LoadEffectLedger：%v", err)
	}
	got, ok := loaded.Applied(first.Key)
	if !ok {
		t.Fatal("往返后条目丢了")
	}
	if got.PreconditionStateHash != "state-A" || got.ObservedAfterHash != "after-1" {
		t.Errorf("前后置状态哈希必须逐字往返：%+v", got)
	}

	// 逐效果：第 1 条独立认领，与第 0 条互不串线
	if claim, _, err := l.ClaimChecked(plan.Entries[1], "state-A"); err != nil || claim != EffectNew {
		t.Fatalf("第 1 条效果应独立认领：claim=%s err=%v", claim, err)
	}
	if _, ok := l.AppliedIndex("c1", 1); !ok {
		t.Error("第 1 条认领后应能按 (call_id, effect_index) 查到")
	}
	if l.Len() != 2 {
		t.Errorf("账本应为 2 条，实得 %d", l.Len())
	}
	// 去重域是**效果级**：同一效果换个 call_id 仍是同一效果（状态相符 ⇒ 已发生）
	cross := plan.Entries[1] // 这条已经认领过（第 1 条）
	cross.CallID = "c9"
	if claim, _, err := l.ClaimChecked(cross, "state-A"); err != nil || claim != EffectAlreadyApplied {
		t.Errorf("同一效果换 call_id 仍应判已发生：claim=%s err=%v", claim, err)
	}
	// 反例（对侧）：换 call_id 也不放松状态校验
	if _, _, err := l.ClaimChecked(cross, "state-C"); !errors.Is(err, ErrStateMismatch) {
		t.Errorf("换 call_id 后状态不符仍须报不匹配，实得 %v", err)
	}
	// "已认领、未收尾"必须可见（不是静默当它已完成）
	unfinished := l.Unfinished()
	if len(unfinished) != 1 || unfinished[0].Key != plan.Entries[1].Key {
		t.Errorf("应恰有 1 条未收尾（第 1 条效果），实得 %+v", unfinished)
	}
}

// ── ⑤ 去重存储两档：内存 + 本地文件 ────────────────────────────────────────

func TestT43_FileLedgerDurableAcrossReopen(t *testing.T) {
	dir := t.TempDir()
	norm := DefaultNormalizer(dir)
	path := filepath.Join(dir, "effects.json")
	canonical := mustNormalize(t, norm, "bash", map[string]any{
		"call_id": "c1", "command": "echo a > out/a.txt && notify",
	}).Canonical
	plan := PlanCallEffects("bash", canonical, "c1", []string{"fs:write out/a.txt", "proc:notify"}, ModeRecord)
	e0, e1 := plan.Entries[0], plan.Entries[1]

	// ① 内存档 + 文件档：首次打开（文件还不存在）应是**空**账本，不是错误
	f, err := OpenFileEffectLedger(path)
	if err != nil {
		t.Fatalf("OpenFileEffectLedger（首次）：%v", err)
	}
	if f.Len() != 0 {
		t.Errorf("首次打开应是空账本，实得 %d 条", f.Len())
	}
	if f.Path() != path {
		t.Errorf("路径不符：%s", f.Path())
	}
	if _, _, err := f.ClaimPersisted(e0, "state-A"); err != nil {
		t.Fatalf("认领 + 落盘：%v", err)
	}
	// 独立证据：**直接读文件**看这条事实在不在（不经过被测代码的读路径）
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("落盘后应能看到账本文件：%v", err)
	}
	if !strings.Contains(string(raw), e0.Key) {
		t.Errorf("认领必须先落盘（跨崩溃才判得出「已发生」）：文件里没有 %s", e0.Key)
	}
	if err := f.CompletePersisted(e0.Key, "after-1"); err != nil {
		t.Fatalf("收尾 + 落盘：%v", err)
	}
	raw2, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读账本：%v", err)
	}
	if !strings.Contains(string(raw2), "after-1") {
		t.Error("执行后状态也必须落盘（否则重启后看不出这条效果收没收尾）")
	}

	// ② 模拟"进程重启"：另开一个账本对象（不共享内存）——落盘再读后仍判"已发生"
	g, err := OpenFileEffectLedger(path)
	if err != nil {
		t.Fatalf("OpenFileEffectLedger（重启）：%v", err)
	}
	if g.Len() != 1 {
		t.Fatalf("重启后应读回 1 条，实得 %d", g.Len())
	}
	claim, entry, err := g.ClaimPersisted(e0, "state-A")
	if err != nil {
		t.Fatalf("重启后同一效果应判「已发生」（不是错误）：%v", err)
	}
	if claim != EffectAlreadyApplied {
		t.Errorf("落盘再读后必须仍判为已发生，实得 %s", claim)
	}
	if entry.PreconditionStateHash != "state-A" || entry.ObservedAfterHash != "after-1" {
		t.Errorf("前后置状态哈希必须逐字往返：%+v", entry)
	}
	// 对侧：**没发生过**的那一条照样判"新"（防止"一律已发生"的假绿）
	if claim, _, err := g.ClaimPersisted(e1, "state-A"); err != nil || claim != EffectNew {
		t.Errorf("没落盘的条目应判为 new：claim=%s err=%v", claim, err)
	}
	if g.Len() != 2 {
		t.Errorf("账本应为 2 条，实得 %d", g.Len())
	}
	// 反例：状态不符 ⇒ 落盘档同样 fail-closed
	if _, _, err := g.ClaimPersisted(e0, "state-B"); !errors.Is(err, ErrStateMismatch) {
		t.Errorf("落盘档也必须拦状态不符，实得 %v", err)
	}

	// 反例：坏账本 ⇒ **拒绝打开**（不许静默退化成空账本：那会把做过的效果全当没做过）
	badDup := filepath.Join(dir, "bad-dup.json")
	if err := os.WriteFile(badDup, []byte(`{"v":1,"effects":[{"key":"k"},{"key":"k"}]}`), 0o644); err != nil {
		t.Fatalf("造坏账本：%v", err)
	}
	if _, err := OpenFileEffectLedger(badDup); err == nil {
		t.Error("重复 key 的账本必须拒绝打开")
	}
	badJSON := filepath.Join(dir, "bad-json.json")
	if err := os.WriteFile(badJSON, []byte("{"), 0o644); err != nil {
		t.Fatalf("造坏账本：%v", err)
	}
	if _, err := OpenFileEffectLedger(badJSON); err == nil {
		t.Error("非法 JSON 的账本必须拒绝打开")
	}
	// 反例：「没有账本」不许写成「空账本」
	if err := SaveEffectLedger(filepath.Join(dir, "nil.json"), nil); err == nil {
		t.Error("SaveEffectLedger(nil) 应报错（不存在的账本 ≠ 空账本）")
	}

	// ③ 落盘失败 ⇒ **撤销认领**（先落盘后认领：写不下去就不许执行）
	sub := filepath.Join(dir, "sub")
	subPath := filepath.Join(sub, "effects.json")
	h, err := OpenFileEffectLedger(subPath)
	if err != nil {
		t.Fatalf("OpenFileEffectLedger（子目录）：%v", err)
	}
	if _, _, err := h.ClaimPersisted(e0, "state-A"); err != nil {
		t.Fatalf("第一次认领（会建目录）：%v", err)
	}
	// 把父目录换成一个**普通文件** ⇒ 之后任何落盘都必然失败（与权限无关，root 下同样失败）
	if err := os.RemoveAll(sub); err != nil {
		t.Fatalf("移除目录：%v", err)
	}
	if err := os.WriteFile(sub, []byte("不是目录\n"), 0o644); err != nil {
		t.Fatalf("把目录换成文件：%v", err)
	}
	claim4, _, err := h.ClaimPersisted(e1, "state-A")
	if err == nil {
		t.Fatal("落盘失败必须报错（先落盘后认领）")
	}
	if claim4 == EffectNew {
		t.Error("落盘失败时不许回 new —— 调用方会据此执行一个记不住的效果")
	}
	if _, ok := h.Applied(e1.Key); ok {
		t.Error("落盘失败必须撤销内存认领，否则重启后会当它发生过 ⇒ 效果静默少做")
	}
	if h.Len() != 1 {
		t.Errorf("撤销后账本应回到 1 条，实得 %d", h.Len())
	}
	// 恢复现场：账本仍可继续用（撤销没把账本搞坏）
	if err := os.Remove(sub); err != nil {
		t.Fatalf("移除占位文件：%v", err)
	}
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("重建目录：%v", err)
	}
	if claim5, _, err := h.ClaimPersisted(e1, "state-A"); err != nil || claim5 != EffectNew {
		t.Fatalf("恢复后应能重新认领：claim=%s err=%v", claim5, err)
	}
	if err := h.CompletePersisted(e1.Key, "after-1"); err != nil {
		t.Fatalf("恢复后收尾：%v", err)
	}
	k, err := OpenFileEffectLedger(subPath)
	if err != nil {
		t.Fatalf("重新打开：%v", err)
	}
	if k.Len() != 2 {
		t.Errorf("两条效果都该落盘，实得 %d", k.Len())
	}
}

// ── ⑥ 部分成功 ⇒ 只补做未执行的那几条（不重做已完成的）─────────────────────

func TestT43_PartialSuccessResumeOnlyPending(t *testing.T) {
	dir := t.TempDir()
	norm := DefaultNormalizer(dir)
	ledgerPath := filepath.Join(dir, "effects.json")
	outFile := filepath.Join(dir, "out", "answer.txt")
	notifyFile := filepath.Join(dir, "notify.log")

	// 一次调用的两个效果：① 写文件 ② 发通知（"先写文件再发通知"）
	canonical := mustNormalize(t, norm, "bash", map[string]any{
		"call_id": "c1", "command": "echo answer > out/answer.txt && notify",
	}).Canonical
	scopes := []string{"fs:write out/answer.txt", "proc:notify"}
	plan := PlanCallEffects("bash", canonical, "c1", scopes, ModeRecord)

	// 第 0 条做完了（真写文件 + 记状态）；第 1 条还没做就中断
	led, err := OpenFileEffectLedger(ledgerPath)
	if err != nil {
		t.Fatalf("OpenFileEffectLedger：%v", err)
	}
	if claim, _, err := led.ClaimPersisted(plan.Entries[0], "state-A"); err != nil || claim != EffectNew {
		t.Fatalf("第 0 条认领：claim=%s err=%v", claim, err)
	}
	if err := os.MkdirAll(filepath.Dir(outFile), 0o755); err != nil {
		t.Fatalf("建目录：%v", err)
	}
	if err := os.WriteFile(outFile, []byte("answer v1\n"), 0o644); err != nil {
		t.Fatalf("写文件：%v", err)
	}
	hashAfterWrite := buildStateHash(t, outFile)
	if err := led.CompletePersisted(plan.Entries[0].Key, hashAfterWrite); err != nil {
		t.Fatalf("第 0 条收尾：%v", err)
	}
	if _, err := os.Stat(notifyFile); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("场景前提：通知此时还没发出去（%v）", err)
	}

	// 崩溃重启：新账本对象 + 真副作用计数器 + 真 sha256
	led2, err := OpenFileEffectLedger(ledgerPath)
	if err != nil {
		t.Fatalf("重启打开账本：%v", err)
	}
	writes, notifies := 0, 0
	exec := func(e EffectEntry) error {
		switch e.Index {
		case 0:
			writes++
			// 重做会覆盖文件内容：靠 sha256 抓住（不是靠"函数被调过"这种自我叙述）
			return os.WriteFile(outFile, []byte("answer v1\n"), 0o644)
		default:
			notifies++
			f, err := os.OpenFile(notifyFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
			if err != nil {
				return err
			}
			defer f.Close()
			_, err = f.WriteString("通知：answer.txt 已写好\n")
			return err
		}
	}
	observe := func(e EffectEntry) (string, error) {
		if _, err := os.Stat(notifyFile); err != nil {
			return "", err
		}
		return buildStateHash(t, outFile, notifyFile), nil
	}

	// 计划口径：只该有一条待做（第 1 条）
	pending, done := plan.Pending(led2)
	if len(pending) != 1 || pending[0].Index != 1 {
		t.Fatalf("只该补做第 1 条（通知），实得 %+v", pending)
	}
	if len(done) != 1 || done[0].Index != 0 || done[0].ObservedAfterHash != hashAfterWrite {
		t.Fatalf("第 0 条应判为已发生且带执行后状态哈希，实得 %+v", done)
	}

	res, err := ResumePending(plan, led2, "state-A", exec, observe)
	if err != nil {
		t.Fatalf("补做：%v", err)
	}
	if writes != 0 {
		t.Errorf("已完成的第 0 条（写文件）**不得重做**，实得重做 %d 次", writes)
	}
	if notifies != 1 {
		t.Errorf("只该补做没执行的那一条（通知一次），实得 %d 次", notifies)
	}
	if len(res.Executed) != 1 || res.Executed[0].Index != 1 {
		t.Errorf("本次真做的应只有第 1 条：%+v", res.Executed)
	}
	if len(res.Done) != 1 || len(res.Pending) != 1 {
		t.Errorf("结果里应晒出 1 条已发生 + 1 条待做：%+v", res)
	}
	// 独立证据（不靠被测代码自述）：文件逐字节没变 + 通知恰有一行
	if got := buildStateHash(t, outFile); got != hashAfterWrite {
		t.Errorf("已完成效果的文件被改动了：%s != %s", got, hashAfterWrite)
	}
	if n := countLines(t, notifyFile); n != 1 {
		t.Errorf("通知应恰有 1 行，实得 %d 行", n)
	}
	if e, ok := led2.Applied(plan.Entries[1].Key); !ok || e.ObservedAfterHash == "" {
		t.Error("补做的效果必须记下执行后状态哈希（否则下次会当它没做过）")
	}
	if un := led2.Unfinished(); len(un) != 0 {
		t.Errorf("补做完成后不该有未收尾条目：%+v", un)
	}

	// 反例 A：恢复流程再跑一遍 ⇒ 什么都不做（不重做、通知不重复）
	res2, err := ResumePending(plan, led2, "state-A", exec, observe)
	if err != nil {
		t.Fatalf("第二趟恢复：%v", err)
	}
	if writes != 0 || notifies != 1 {
		t.Errorf("重跑恢复流程不得重做任何效果：writes=%d notifies=%d", writes, notifies)
	}
	if len(res2.Pending) != 0 || len(res2.Executed) != 0 {
		t.Errorf("全做完后不该有待做（pending=%d executed=%d）", len(res2.Pending), len(res2.Executed))
	}
	if len(res2.Done) != 2 {
		t.Errorf("两条都应判为已发生，实得 %+v", res2.Done)
	}
	if got := buildStateHash(t, outFile); got != hashAfterWrite {
		t.Error("重跑恢复流程改动了文件（假绿：靠 sha256 才看得出来）")
	}
	if n := countLines(t, notifyFile); n != 1 {
		t.Errorf("通知重复了（%d 行）", n)
	}

	// 反例 B：状态不符 ⇒ 报"状态不匹配"，且**一条都不补做**
	{
		dirB := t.TempDir()
		ledB, err := OpenFileEffectLedger(filepath.Join(dirB, "effects.json"))
		if err != nil {
			t.Fatalf("B：%v", err)
		}
		outB := filepath.Join(dirB, "answer.txt")
		if err := os.WriteFile(outB, []byte("v1\n"), 0o644); err != nil {
			t.Fatalf("B：%v", err)
		}
		if claim, _, err := ledB.ClaimPersisted(plan.Entries[0], "state-A"); err != nil || claim != EffectNew {
			t.Fatalf("B：第 0 条认领 claim=%s err=%v", claim, err)
		}
		if err := ledB.CompletePersisted(plan.Entries[0].Key, buildStateHash(t, outB)); err != nil {
			t.Fatalf("B：%v", err)
		}
		writesB, notifiesB := 0, 0
		execB := func(e EffectEntry) error {
			if e.Index == 0 {
				writesB++
			} else {
				notifiesB++
			}
			return nil
		}
		if _, err := ResumePending(plan, ledB, "state-B", execB, observe); !errors.Is(err, ErrStateMismatch) {
			t.Fatalf("B：状态不符必须报 ErrStateMismatch，实得 %v", err)
		}
		if writesB != 0 || notifiesB != 0 {
			t.Errorf("B：状态不符时一条都不许补做（writes=%d notifies=%d）", writesB, notifiesB)
		}
	}

	// 反例 C：执行后取不到状态 ⇒ 报错，且该效果**不许**被记成已收尾（"做了但没记账"必须可见）
	{
		dirC := t.TempDir()
		ledC, err := OpenFileEffectLedger(filepath.Join(dirC, "effects.json"))
		if err != nil {
			t.Fatalf("C：%v", err)
		}
		if _, err := ResumePending(plan, ledC, "state-A",
			func(e EffectEntry) error { return nil },
			func(e EffectEntry) (string, error) { return "", nil }); err == nil {
			t.Fatal("C：执行后取不到状态必须报错")
		}
		if un := ledC.Unfinished(); len(un) != 1 {
			t.Errorf("C：该效果应留在「未收尾」里等处置，实得 %+v", un)
		}
	}

	// 反例 D：exec 失败 ⇒ 认领**保留**（宁可少做不可重做），且必须在 Unfinished() 里可见；
	//           核实"确实没发生"后 Release ⇒ 下次补做才重试
	{
		dirD := t.TempDir()
		ledD, err := OpenFileEffectLedger(filepath.Join(dirD, "effects.json"))
		if err != nil {
			t.Fatalf("D：%v", err)
		}
		tried := 0
		failExec := func(e EffectEntry) error {
			tried++
			if e.Index == 0 {
				return nil
			}
			return errors.New("通知服务不可用")
		}
		obs := func(e EffectEntry) (string, error) { return "state-after", nil }
		resD, err := ResumePending(plan, ledD, "state-A", failExec, obs)
		if err == nil {
			t.Fatal("D：exec 失败必须报错（不许静默跳过）")
		}
		if len(resD.Executed) != 1 || resD.Executed[0].Index != 0 {
			t.Errorf("D：第 0 条应已执行并收尾，实得 %+v", resD.Executed)
		}
		if tried != 2 {
			t.Errorf("D：应执行到失败的那一条为止（试了 %d 次）", tried)
		}
		if e, ok := ledD.Applied(plan.Entries[1].Key); !ok || e.ObservedAfterHash != "" {
			t.Errorf("D：失败的条目应「已认领、未收尾」，实得 %+v", e)
		}
		if un := ledD.Unfinished(); len(un) != 1 || un[0].Index != 1 {
			t.Errorf("D：未收尾条目必须可见（不是静默当它已完成）：%+v", un)
		}
		// 保守一侧：再跑恢复流程**不会**自动重做它（可能已经做了一半），需要人核实后 Release
		resD2, err := ResumePending(plan, ledD, "state-A", failExec, obs)
		if err != nil {
			t.Fatalf("D：第二趟不该报错：%v", err)
		}
		if tried != 2 {
			t.Errorf("D：未 Release 前不得自动重做（试了 %d 次）", tried)
		}
		if len(resD2.Pending) != 0 {
			t.Errorf("D：未 Release 前它仍算「已发生」，实得 pending=%+v", resD2.Pending)
		}
		// 人工核实"确实没发生" ⇒ Release ⇒ 下次补做才重试
		if err := ledD.Release(plan.Entries[1].Key); err != nil {
			t.Fatalf("D：Release：%v", err)
		}
		okExec := func(e EffectEntry) error { tried++; return nil }
		if _, err := ResumePending(plan, ledD, "state-A", okExec, obs); err != nil {
			t.Fatalf("D：Release 后应能补做：%v", err)
		}
		if tried != 3 {
			t.Errorf("D：Release 后应重试一次（试了 %d 次）", tried)
		}
		if e, ok := ledD.Applied(plan.Entries[1].Key); !ok || e.ObservedAfterHash == "" {
			t.Error("D：补做成功后必须记下执行后状态哈希")
		}
		if un := ledD.Unfinished(); len(un) != 0 {
			t.Errorf("D：全部收尾后不该有未收尾条目：%+v", un)
		}
	}
}

// countLines —— 数文件行数（空文件 0 行）——部分成功场景的独立计数证据。
func countLines(t *testing.T, path string) int {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读 %s：%v", path, err)
	}
	n := 0
	for _, line := range strings.Split(strings.TrimRight(string(b), "\n"), "\n") {
		if strings.TrimSpace(line) != "" {
			n++
		}
	}
	return n
}
