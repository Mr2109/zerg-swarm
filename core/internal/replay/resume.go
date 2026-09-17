// resume.go —— **部分成功后的补做**（E4：一次调用多效果 ⇒ 逐条判重、只补做没执行的那几条）。
//
// ── 场景（这段就是用例里的那个）─────────────────────────────────────────────
//
//	一次调用有两个效果：① 写文件 ② 发通知。第 ① 条做完、第 ② 条还没做就中断了。
//	按"调用"判重只有两种烂选择：整调用重跑（文件再写一遍，可能覆盖掉中间被别人改过的内容）
//	或整调用跳过（通知永远发不出去）。所以补做必须以 **(call_id, effect_index)** 为单位：
//	已发生的**一条都不重做**，没发生的**一条都不落下**。
//
// ── 三道纪律 ────────────────────────────────────────────────────────────────
//
//  1. 派生口径与 Dispatcher **完全相同**（PlanCallEffects 直接用 EffectKey + 递增 index），
//     否则"计划里的 key"与"执行时记的 key"对不上，判重会永远落空（看着像"都没发生过"）。
//  2. 执行前对**已发生**的每条做前置状态校验（E5）：状态不符 ⇒ 报"状态不匹配"且**一条都不执行**。
//     在错误的状态上补做，等于把 trace 记录的状态悄悄换掉。
//  3. 执行后**必须**记下 observed_after_hash：记不上就报错（"做了但没记账"比不做更危险 ——
//     下次启动会当它没做过，再补做一遍）。
package replay

import "fmt"

// EffectStore —— 判重 / 补做需要的**账本能力面**：内存档（*EffectLedger）与文件档
// （*FileEffectLedger，内嵌前者）都满足它。定义成接口是为了让"只要去重与状态校验"的调用方
// 不必知道账本落在哪一档（内存 / 文件 / 以后的本地 KV）。
type EffectStore interface {
	Applied(key string) (EffectEntry, bool)
	AppliedIndex(callID string, index int) (EffectEntry, bool)
	ClaimChecked(e EffectEntry, currentStateHash string) (EffectClaim, EffectEntry, error)
	VerifyState(key, currentStateHash string) error
	Complete(key, observedAfterHash string) error
}

// ResumePlan —— 一次调用**该做的全部效果**（含 (call_id, effect_index) 逐条标识）。
type ResumePlan struct {
	Tool    string        `json:"tool"`
	CallID  string        `json:"call_id,omitempty"`
	Entries []EffectEntry `json:"entries"` // 按 effect_index 升序（0 起、连续）
}

// PlanCallEffects —— 用与 Dispatcher 相同的口径派生效果清单（工具名 + 规范化参数 + 效果范围）。
//
// scopes 的**次序即 effect_index**：同一个调用同一份 scopes 派生出的清单必然逐字节相同
// （键是纯函数），所以补做期的清单与录制期的清单必然对上。
func PlanCallEffects(tool string, canonical []byte, callID string, scopes []string, mode Mode) ResumePlan {
	p := ResumePlan{Tool: tool, CallID: callID}
	p.Entries = make([]EffectEntry, 0, len(scopes))
	for i, s := range scopes {
		p.Entries = append(p.Entries, EffectEntry{
			Key:    EffectKey(tool, canonical, s),
			Tool:   tool,
			Scope:  s,
			CallID: callID,
			Index:  i,
			Mode:   mode,
		})
	}
	return p
}

// Pending —— 逐条判重：pending = 该补做的（保持 index 升序），done = 已发生的（附账本里**首次**那条事实）。
//
// 判重域 = 效果级：先按 (call_id, effect_index) 查，查不到再按 effect_key 查
// （同一效果换个 call_id 仍是同一效果 —— 与 Dispatcher 的去重域一致）。
// 本函数**只读**账本：补做计划不许顺手改状态。
func (p ResumePlan) Pending(l EffectStore) (pending []EffectEntry, done []EffectEntry) {
	if storeIsNil(l) {
		// 没有账本 ⇒ 一条都不能算"已发生"：这是保守的一侧（宁可重做也不静默少做），
		// 但调用方必须知道自己在没有判重依据的情况下补做。
		return append([]EffectEntry(nil), p.Entries...), nil
	}
	for _, e := range p.Entries {
		if first, ok := ledgerHit(l, p.CallID, e); ok {
			done = append(done, first)
			continue
		}
		pending = append(pending, e)
	}
	return pending, done
}

// storeIsNil —— 账本是否为"没有账本"（含 Go 接口里的 typed-nil 陷阱：`(*EffectLedger)(nil)`
// 作为接口值**不等于** nil，直接调方法会在 nil 互斥锁上炸）。列在这里是为了让 nil 判断**显式**。
func storeIsNil(l EffectStore) bool {
	switch v := l.(type) {
	case nil:
		return true
	case *EffectLedger:
		return v == nil
	case *FileEffectLedger:
		return v == nil
	}
	return false
}

// ledgerHit —— 该效果是否已在账本里（逐效果口径 + 跨调用去重域）。
func ledgerHit(l EffectStore, callID string, e EffectEntry) (EffectEntry, bool) {
	if callID != "" {
		if hit, ok := l.AppliedIndex(callID, e.Index); ok {
			return hit, true
		}
	}
	return l.Applied(e.Key)
}

// VerifyState —— 补做前的前置状态校验（E5）：对**已发生**的每一条比状态指纹。
//
// 状态不符 / 没记 / 本次没给 ⇒ 返回 *StateMismatchError（调用方据此**一条都不补做**）。
// 已发生的条目为空 ⇒ 无可校验（返回 nil；第一次做这件事时没有"录制时的前置状态"可比）。
func (p ResumePlan) VerifyState(l EffectStore, currentStateHash string) error {
	if storeIsNil(l) {
		return nil
	}
	_, done := p.Pending(l)
	for _, e := range done {
		if err := l.VerifyState(e.Key, currentStateHash); err != nil {
			return err
		}
	}
	return nil
}

// ResumeResult —— 补做结果（三条清单都晒出来：待做 / 已发生 / 本次真做了）。
type ResumeResult struct {
	Pending  []EffectEntry `json:"pending"`  // 判重后**该补做**的
	Done     []EffectEntry `json:"done"`     // 判为已发生、**一条都没重做**的
	Executed []EffectEntry `json:"executed"` // 本次真正执行了的（= Pending 减去中途失败的）
}

// ResumePending —— 补做执行器：**只对没发生过的效果**调 exec；每条执行后记 observed_after_hash。
//
//	· 执行前：plan.VerifyState（状态不符 ⇒ 立刻返回错误，一条都不执行）；
//	· 执行中：按 effect_index 升序，逐条 ClaimChecked → exec → Complete(observe)；
//	      exec 失败 ⇒ **立即停**，且**认领保留**（保守一侧：exec 报了错也可能已经把副作用做了一半，
//	        重做会来第二遍；宁可少做不可重做）—— 这条"已认领、未收尾"会出现在
//	        EffectLedger.Unfinished() 里，必须由上层核实后走 Release（放行重做）或人工补偿；
//	      observe 失败/为空 ⇒ 报错（做了但记不上状态 = 下次会当它没做，必须当场暴露）；
//	· 已发生的一条也不重做（不调 exec，不改账本）。
//
// exec / observe 由调用方给：本包**不猜**"这个效果怎么做"，也不碰调用方的状态存储。
func ResumePending(plan ResumePlan, l EffectStore, currentStateHash string,
	exec func(e EffectEntry) error, observe func(e EffectEntry) (string, error)) (ResumeResult, error) {
	if exec == nil || observe == nil {
		return ResumeResult{}, fmt.Errorf("%w: 补做需要 exec 与 observe 都给（拒绝半配置）", ErrEffectLedger)
	}
	if storeIsNil(l) {
		return ResumeResult{}, fmt.Errorf("%w: 补做需要账本（没有账本就无法判重，也无法记状态哈希）", ErrEffectLedger)
	}
	if err := plan.VerifyState(l, currentStateHash); err != nil {
		return ResumeResult{}, err
	}
	res := ResumeResult{}
	pending, done := plan.Pending(l)
	res.Done = done
	res.Pending = pending
	for _, e := range pending {
		if _, _, err := l.ClaimChecked(e, currentStateHash); err != nil {
			return res, err
		}
		if err := exec(e); err != nil {
			return res, fmt.Errorf(
				"补做效果 %s 失败：%v —— 认领**保留**（exec 报错也可能已经把副作用做了一半，重做会来第二遍）；"+
					"这条已出现在 Unfinished() 里，请核实后 Release（放行重做）或人工补偿", e.String(), err)
		}
		after, err := observe(e)
		if err != nil {
			return res, fmt.Errorf("效果 %s 执行后取不到状态哈希（做了但记不上 ⇒ 下次会被当没做过）：%w", e.String(), err)
		}
		if err := l.Complete(e.Key, after); err != nil {
			return res, err
		}
		res.Executed = append(res.Executed, e)
	}
	return res, nil
}
