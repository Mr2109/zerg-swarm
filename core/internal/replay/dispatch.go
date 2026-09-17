// dispatch.go —— 工具执行的**唯一分发口**（E2：主用工具层 Dispatcher/ReplayTool）。
//
// 三态：
//
//	ModeLive   真发、不录  —— 与今天的行为等价（本包对它是纯包装：调用方不接就等于没接）。
//	ModeRecord 真发 + 录   —— 走 Live 拿真结果，然后把**事实**写进 trace（含错误路径）。
//	ModeReplay 只读录播    —— 只查 trace，**绝不调用 Live、绝不写 trace、绝不写效果账本**。
//
// 「绝不真发」不是靠文档自律，而是结构性的：回放分支里根本没有任何路径能走到 d.Live
// （用例③用"会真写文件的 hook + 计数 + 文件 sha256"钉住这一点）。
package replay

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Mode —— 分发模式。
type Mode string

const (
	// ModeLive —— 真发不录。
	ModeLive Mode = "live"
	// ModeRecord —— 真发并录制。
	ModeRecord Mode = "record"
	// ModeReplay —— 只读录播（绝不真发）。
	ModeReplay Mode = "replay"
)

// Source —— 结果的来源（调用方据此判断"这是真做的还是录播/去重"）。
type Source string

const (
	// SourceLive —— 真发得到的结果。
	SourceLive Source = "live"
	// SourceRecorded —— 回放命中录播。
	SourceRecorded Source = "recorded"
	// SourceDeduped —— 该效果已发生过 ⇒ 未执行、未重放（恢复路径的正常分支，不是错误）。
	SourceDeduped Source = "deduped"
)

// Result —— 一次工具执行的结果（与 agent 的 ToolCallResult 同形，但**不 import agent**：
// 接线时由适配层转换，叶包不反向依赖业务包）。
type Result struct {
	Body       []byte
	Err        string
	DurationMS int64
	Source     Source
	Seq        int           // 命中的 trace 序号（录制/回放才有）
	Effects    []EffectRange // 本次（或录播里）的效果
	Deduped    []EffectEntry // 已发生过的效果（Source == SourceDeduped 时非空；回放期只**报告**）
}

// ToolFunc —— "真发"的实现（由接线方提供；本包自己绝不执行任何工具）。
type ToolFunc func(ctx context.Context, call Call, args map[string]any) Result

// Dispatcher —— 录制/回放/真发的唯一入口。
type Dispatcher struct {
	Mode   Mode
	Norm   Normalizer
	Player *Player  // ModeReplay 必填
	Trace  *Trace   // ModeRecord 必填
	Live   ToolFunc // ModeLive/ModeRecord 必填；ModeReplay 下**装上也不会被调用**（用例③钉住）
	// Effects —— 效果账本（可选）。装上 ⇒ 先认领后执行：同一 effect_key 第二次出现时
	// **整体跳过真发**并返回 SourceDeduped。
	Effects *EffectLedger
	// EffectScope —— 由一次调用派生**效果范围**（E4 的"效果范围"）。返回空切片 = 本调用无副作用；
	// nil = 调用方未声明 ⇒ 本包**不判重也不假设无副作用**（"猜有没有副作用"猜错的方向是危险那一侧）。
	EffectScope func(call Call, args map[string]any) []string
	// StateHash —— 当前状态的指纹（E5）。非 nil 时：录制路径写进
	// PreconditionHash / ObservedAfterHash；回放路径（配 VerifyPrecondition）用它做前置校验。
	StateHash func() string
	// VerifyPrecondition —— 回放时校验前置状态（E5）。默认 false = 不做校验（老 trace 没记前置）。
	VerifyPrecondition bool
	// Now —— 取当前时间（只用于 DurationMS；可注入以便测试/冻结时钟，E9）。
	Now func() time.Time
}

func (d *Dispatcher) now() time.Time {
	if d.Now != nil {
		return d.Now()
	}
	return time.Now()
}

// Execute —— 唯一入口：规范化 → 按模式分发。
//
// 规范化在任何模式**之前**做：参数都规范不了（含不支持的类型）就没法录也没法匹配 ⇒ 直接报错，
// 绝不放行到 Live（否则会出现"录不进 trace 但确实执行了"的黑洞）。
func (d *Dispatcher) Execute(ctx context.Context, tool string, args map[string]any) (Result, error) {
	call, err := d.Norm.Normalize(tool, args)
	if err != nil {
		return Result{}, err
	}
	switch d.Mode {
	case ModeReplay:
		return d.replay(call, args)
	case ModeRecord:
		return d.record(ctx, call, args)
	case ModeLive:
		return d.live(ctx, call, args)
	default:
		// 未知模式静默按 live 跑 = 最坏的一种"配置错了但看起来正常"。
		return Result{}, fmt.Errorf("replay: 未知模式 %q（拒绝静默放行）", d.Mode)
	}
}

// ── 回放：只读 ──────────────────────────────────────────────────────────────

func (d *Dispatcher) replay(call Call, args map[string]any) (Result, error) {
	if d.Player == nil {
		// 回放模式却没有录播源 = 配置错误。此时"退化成真发"是绝对不允许的兜底。
		return Result{}, fmt.Errorf("%w: 回放模式下 Player 缺失（拒绝退化成真发）", ErrReplayMiss)
	}
	var rec Record
	var err error
	if d.VerifyPrecondition {
		rec, err = d.Player.LookupChecked(call, d.currentStateHash())
	} else {
		rec, err = d.Player.Lookup(call)
	}
	if err != nil {
		return Result{}, err
	}
	res := Result{
		Body:       rec.ResultBody,
		Err:        rec.ErrText, // E10：录制时的失败/超时**原样**返回
		DurationMS: rec.DurationMS,
		Source:     SourceRecorded,
		Seq:        rec.Seq,
		Effects:    rec.Effects,
	}
	// 回放期账本**只读**：只报告"这些效果此前已发生过"，不认领、不改账本（否则"回放"就写了状态）。
	if d.Effects != nil {
		for _, ef := range rec.Effects {
			if e, ok := d.Effects.Applied(ef.Key); ok {
				res.Deduped = append(res.Deduped, e)
			}
		}
	}
	return res, nil
}

// ── 录制 / 真发 ─────────────────────────────────────────────────────────────

func (d *Dispatcher) record(ctx context.Context, call Call, args map[string]any) (Result, error) {
	if d.Trace == nil || d.Live == nil {
		return Result{}, errors.New("replay: 录制模式需要 Trace 与 Live 都装上（拒绝半配置）")
	}
	entries := d.effectEntries(call, args, ModeRecord)
	ranges := rangesOf(entries)
	deduped, done, err := d.claimBeforeExecute(entries, ranges)
	if err != nil {
		return Result{}, err
	}
	if done {
		return deduped, nil // 先认领后执行：任一效果已发生 ⇒ 整调用不再真发
	}
	pre := d.currentStateHash()
	start := d.now()
	res := d.Live(ctx, call, args)
	res.DurationMS = d.now().Sub(start).Milliseconds()
	res.Source = SourceLive
	res.Effects = ranges
	after := d.currentStateHash()
	// 执行后收尾（E5）：把 observed_after_hash 记进账本。记不上 ⇒ 报错（不许"做了但没记账"）。
	if err := d.completeAfterExecute(entries, after); err != nil {
		return Result{}, err
	}
	rec := Record{
		CallID:            callIDOf(args),
		Tool:              call.Tool,
		ArgsCanonical:     call.Canonical,
		ArgsDigest:        call.ArgsDigest,
		ResultBody:        res.Body,
		ResultDigest:      Digest(res.Body),
		ErrText:           res.Err,
		DurationMS:        res.DurationMS,
		PreconditionHash:  pre,
		ObservedAfterHash: after,
		Effects:           ranges,
	}
	written, err := d.Trace.Append(rec)
	if err != nil {
		return Result{}, err
	}
	res.Seq = written.Seq
	return res, nil
}

func (d *Dispatcher) live(ctx context.Context, call Call, args map[string]any) (Result, error) {
	if d.Live == nil {
		return Result{}, errors.New("replay: 真发模式需要 Live（拒绝静默空转）")
	}
	entries := d.effectEntries(call, args, ModeLive)
	ranges := rangesOf(entries)
	deduped, done, err := d.claimBeforeExecute(entries, ranges)
	if err != nil {
		return Result{}, err
	}
	if done {
		return deduped, nil
	}
	start := d.now()
	res := d.Live(ctx, call, args)
	res.DurationMS = d.now().Sub(start).Milliseconds()
	res.Source = SourceLive
	res.Effects = ranges
	if err := d.completeAfterExecute(entries, d.currentStateHash()); err != nil {
		return Result{}, err
	}
	return res, nil
}

// effectEntries —— 派生效果条目（E4）：效果范围由调用方声明（`EffectScope`）；
// 未声明（nil）⇒ 空 ⇒ 不判重也不假设无副作用。键 = EffectKey(工具名, 规范化参数, 效果范围)。
func (d *Dispatcher) effectEntries(call Call, args map[string]any, mode Mode) []EffectEntry {
	if d.EffectScope == nil {
		return nil
	}
	scopes := d.EffectScope(call, args)
	callID := callIDOf(args)
	out := make([]EffectEntry, 0, len(scopes))
	for i, s := range scopes {
		out = append(out, EffectEntry{
			Key:    EffectKey(call.Tool, call.Canonical, s),
			Tool:   call.Tool,
			Scope:  s,
			CallID: callID,
			Index:  i,
			Mode:   mode,
		})
	}
	return out
}

func rangesOf(entries []EffectEntry) []EffectRange {
	out := make([]EffectRange, 0, len(entries))
	for i, e := range entries {
		out = append(out, EffectRange{Index: i, Scope: e.Scope, Key: e.Key})
	}
	return out
}

// claimBeforeExecute —— **先认领后执行**（F6）：全部效果都先认领；任一已经是"已发生" ⇒
// 整调用不执行（部分执行没有好语义：一次工具调用是一个原子动作，只做一半比不做更糟）。
//
// 注意：命中"已发生"时**不再认领其余效果**（它们没有被执行，账本里就不该有）。
//
// VerifyPrecondition 打开时（E5）：认领/命中的同时**校验前置状态**，不符 ⇒ 返回错误
// （*StateMismatchError）且**一条都不执行** —— 在错误的状态上跳过一个效果，比报错危险得多。
func (d *Dispatcher) claimBeforeExecute(entries []EffectEntry, ranges []EffectRange) (Result, bool, error) {
	if d.Effects == nil || len(entries) == 0 {
		return Result{}, false, nil
	}
	// 只读检查一遍：任一条已发生 ⇒ 整体跳过（不认领、不执行、不真发）
	for _, e := range entries {
		first, ok := d.Effects.Applied(e.Key)
		if !ok {
			continue
		}
		if d.VerifyPrecondition {
			if err := d.Effects.VerifyState(e.Key, d.currentStateHash()); err != nil {
				return Result{}, false, err
			}
		}
		return d.dedupedResult(ranges, first), true, nil
	}
	for _, e := range entries {
		if d.VerifyPrecondition {
			claim, first, err := d.Effects.ClaimChecked(e, d.currentStateHash())
			if err != nil {
				return Result{}, false, err
			}
			if claim == EffectAlreadyApplied {
				return d.dedupedResult(ranges, first), true, nil
			}
			continue
		}
		if claim, first := d.Effects.Claim(e); claim == EffectAlreadyApplied {
			return d.dedupedResult(ranges, first), true, nil
		}
	}
	return Result{}, false, nil
}

// completeAfterExecute —— 执行后把 observed_after_hash 记进账本（E5 的收尾）。
//
// 装了账本 + 装了 StateHash 才记；StateHash 返回空串 ⇒ 报错（"装了却拿不出状态"是配置错，
// 不许用空串糊过去 —— 那样下次启动会当这条效果没收尾）。
func (d *Dispatcher) completeAfterExecute(entries []EffectEntry, after string) error {
	if d.Effects == nil || len(entries) == 0 || d.StateHash == nil {
		return nil
	}
	if after == "" {
		return fmt.Errorf("%w: StateHash 已装上却返回空串，无法记下 observed_after_hash（拒绝用空串糊过去）",
			ErrEffectLedger)
	}
	for _, e := range entries {
		if err := d.Effects.Complete(e.Key, after); err != nil {
			return err
		}
	}
	return nil
}

// dedupedResult —— "已发生"的统一返回形态：不执行、不报错，但把"是哪条效果"说清楚。
func (d *Dispatcher) dedupedResult(all []EffectRange, first EffectEntry) Result {
	res := Result{Source: SourceDeduped, Effects: all, Deduped: []EffectEntry{first}}
	return res
}

func (d *Dispatcher) currentStateHash() string {
	if d.StateHash == nil {
		return ""
	}
	return d.StateHash()
}

// callIDOf —— 调用方可在参数里带 "call_id" 声明"这是同一次模型调用的哪一条"（E4 的逐效果判重需要它）。
// 它**去噪字段**里也有同名项 ⇒ 不参与匹配指纹（每次调用都不同，留着只会让命中率归零）。
func callIDOf(args map[string]any) string {
	if v, ok := args["call_id"].(string); ok {
		return v
	}
	return ""
}

// ── 组装助手 ────────────────────────────────────────────────────────────────

// NewRecorder —— 新建 trace 并造一个录制态 Dispatcher（最常用的一组装法）。
func NewRecorder(tracePath string, norm Normalizer, live ToolFunc) (*Dispatcher, error) {
	t, err := Create(tracePath)
	if err != nil {
		return nil, err
	}
	return &Dispatcher{Mode: ModeRecord, Norm: norm, Trace: t, Live: live, Effects: NewEffectLedger()}, nil
}

// NewReplayer —— 按消费游标建回放态 Dispatcher（返回新游标 ⇒ 调用方可直接落盘，E6）。
func NewReplayer(tracePath string, norm Normalizer, from int) (*Dispatcher, Cursor, error) {
	p, cur, err := PlayerFromTrace(tracePath, norm, from)
	if err != nil {
		return nil, Cursor{}, err
	}
	return &Dispatcher{Mode: ModeReplay, Norm: norm, Player: p}, cur, nil
}

// Describe —— 一行诊断（日志里看清"这次到底是哪一态"）。
func (d *Dispatcher) Describe() string {
	var b strings.Builder
	fmt.Fprintf(&b, "mode=%s", d.Mode)
	if d.Player != nil {
		fmt.Fprintf(&b, " 录播=%d 条", d.Player.Len())
	}
	if d.Trace != nil {
		fmt.Fprintf(&b, " trace=%s(%d)", d.Trace.Path(), d.Trace.Len())
	}
	if d.Effects != nil {
		fmt.Fprintf(&b, " 账本=%d", d.Effects.Len())
	}
	if d.VerifyPrecondition {
		b.WriteString(" 校验前置=on")
	}
	return b.String()
}

// Close —— 收口（仅关闭录制态的 trace；回放态无资源）。
func (d *Dispatcher) Close() error {
	if d.Trace != nil {
		return d.Trace.Close()
	}
	return nil
}
