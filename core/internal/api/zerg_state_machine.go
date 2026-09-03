// Package api - v2.5.6 程序定量驱动 Agent: 流程状态机
// 方案轮→选定轮→执行轮→封闭轮——程序固定流程——半程序半LLM
// 2026-08-25 Mr2109——设计 docs/01-设计/设计-程序定量驱动Agent-20260824.md
package api

import (
	"fmt"
	"time"
)

// ZergPhase 状态机阶段
type ZergPhase string

const (
	PhaseInit    ZergPhase = "init"    // 任务建立
	PhasePlan    ZergPhase = "plan"    // 方案轮（模型出 1-3 方案）
	PhaseSelect  ZergPhase = "select"  // 选定轮（模型选一个）
	PhaseExecute ZergPhase = "execute" // 执行轮（受控循环——小任务逐个）
	PhaseClose   ZergPhase = "close"   // 封闭轮（报告——git commit）
	PhaseDone    ZergPhase = "done"    // 完成（复查通过）
	PhaseFailed  ZergPhase = "failed"  // 失败
)

// ZergPhaseFlow 状态机（阶段流转——程序固定）
type ZergPhaseFlow struct {
	Current ZergPhase
}

// NewZergPhaseFlow 新建状态机（init 起步）
func NewZergPhaseFlow() *ZergPhaseFlow {
	return &ZergPhaseFlow{Current: PhaseInit}
}

// Next 下一阶段（合法流转——程序固定——不合法返回错误）
// init → plan → select → execute → close → done
// 任意阶段 → failed（失败）
func (f *ZergPhaseFlow) Next() (ZergPhase, error) {
	transitions := map[ZergPhase]ZergPhase{
		PhaseInit:    PhasePlan,
		PhasePlan:    PhaseSelect,
		PhaseSelect:  PhaseExecute,
		PhaseExecute: PhaseClose,
		PhaseClose:   PhaseDone,
	}
	next, ok := transitions[f.Current]
	if !ok {
		return "", fmt.Errorf("非法流转: %s → ?（合法: init→plan→select→execute→close→done）", f.Current)
	}
	f.Current = next
	return next, nil
}

// To 跳转到指定阶段（重跑/回溯——允许）
func (f *ZergPhaseFlow) To(p ZergPhase) {
	f.Current = p
}

// IsTerminal 是否终态（done/failed——任务结束）
func (f *ZergPhaseFlow) IsTerminal() bool {
	return f.Current == PhaseDone || f.Current == PhaseFailed
}

// String 阶段名（UI/日志）
func (p ZergPhase) String() string {
	return string(p)
}

// ZergRound 一轮记录（task.jsonl 一行——ZergTaskLine 封装）
type ZergRound struct {
	Line ZergTaskLine
	File *ZergTaskFile
}

// NewZergRound 新建一轮（写 task.jsonl）
func NewZergRound(file *ZergTaskFile, phase ZergPhase, step string) *ZergRound {
	return &ZergRound{
		File: file,
		Line: ZergTaskLine{
			Phase: phase.String(),
			Step:  step,
			Time:  time.Now().Format(time.RFC3339),
		},
	}
}

// Commit 提交本轮（写总结——入 task.jsonl）
func (r *ZergRound) Commit(output map[string]interface{}, summary string) (int, error) {
	// 轮次 = 最大 round + 1（init 是 round 0——首轮即 round 1）
	lines, err := r.File.ReadAll()
	if err != nil {
		return 0, err
	}
	maxRound := 0
	for _, l := range lines {
		if l.Round > maxRound {
			maxRound = l.Round
		}
	}
	r.Line.Round = maxRound + 1
	r.Line.Output = output
	r.Line.Summary = summary
	if err := r.File.Append(r.Line); err != nil {
		return 0, err
	}
	return r.Line.Round, nil
}
