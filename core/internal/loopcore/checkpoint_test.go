// checkpoint_test.go — T5.5 实证：**检查点粒度（每步）+ 耐久分档（sync/async/exit）+ 版本标记**。
//
// 三条用例（每条都带反例，防"只断该拒的拒了"这种假绿）：
//
//	① 杀进程后恢复 ⇒ **停在同一步**（另起 store 实例 = 新进程，没有内存态；从最后一份继续）
//	② 快照损坏 ⇒ **报错不静默**（改一字节 / 截断末行 / 版本不认识 / 无快照，四者判词必须分得开）
//	③ 耐久档位生效（sync 档**写入即持久**；async 杀进程不丢但**不 fsync**；exit 档未 Flush **新进程读不到**）
//
// 纪律：本文件不 mock 文件系统 —— 真目录（t.TempDir）+ 真字节读（os.ReadFile）+ 独立复算摘要。
package loopcore

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// ── 测试夹具 ────────────────────────────────────────────────────────────────

// inferRecorder — 逐轮记录"这一轮收到的消息"（用来断言"历史真的接上了"，而不是从零开始）。
type inferRecorder struct {
	mu    sync.Mutex
	seen  [][]map[string]any
	calls int
}

func (r *inferRecorder) snapshot(msgs []map[string]any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	cp := append([]map[string]any(nil), msgs...)
	r.seen = append(r.seen, cp)
}

func (r *inferRecorder) firstMsgs() []map[string]any {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.seen) == 0 {
		return nil
	}
	return r.seen[0]
}

func (r *inferRecorder) rounds() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls
}

// scriptedInfer — 按脚本逐轮返回（脚本用尽后重复最后一条），并把每轮入参记进 rec。
func scriptedInfer(script []Response, rec *inferRecorder) Infer {
	i := 0
	return func(ctx context.Context, model, sysPrompt string, msgs []map[string]any,
		onDelta func(deltaType, text string), tools []map[string]any) (*Response, error) {
		if rec != nil {
			rec.snapshot(msgs)
		}
		r := script[len(script)-1]
		if i < len(script) {
			r = script[i]
		}
		i++
		out := r
		return &out, nil
	}
}

// toolRound — 造一轮"调一次工具"的模型响应。
func toolRound(id, name, argKey, argVal string) Response {
	return Response{ToolCalls: []ToolCall{{
		ID: id, Name: name, Args: map[string]any{argKey: argVal},
		RawArgs: `{"` + argKey + `":"` + argVal + `"}`,
	}}}
}

func mustSave(t *testing.T, s *CheckpointStore, st CheckpointState) *Snapshot {
	t.Helper()
	snap, err := s.Save(st)
	if err != nil {
		t.Fatalf("Save 失败：%v", err)
	}
	return snap
}

// ── 用例①：杀进程后恢复 ⇒ 停在同一步 ────────────────────────────────────────

func TestCheckpoint_ResumeStopsAtSameStepAfterKill(t *testing.T) {
	dir := t.TempDir()
	const runID, sess = "run-kill", "sess-kill"

	// 默认耐久档（空档 ⇒ sync，Q15）：每一步都在盘上 ⇒ 杀进程才有东西可恢复。
	st, err := NewCheckpointStore(dir, "")
	if err != nil {
		t.Fatalf("建 store：%v", err)
	}
	if st.Durability() != DurabilitySync {
		t.Fatalf("空档必须归一到 sync（默认档）：实得 %s", st.Durability())
	}

	execN := 0
	exec := func(ctx context.Context, name string, args map[string]any) (string, string, error) {
		execN++
		return "工具输出-" + name, "1ms", nil
	}
	// 两轮工具调用 ⇒ 轮数用尽退出（max_rounds = **可继续**的终局，正是"杀在半路"的形状）
	script := []Response{
		toolRound("c1", "read", "path", "a.txt"),
		toolRound("c2", "read", "path", "b.txt"),
	}
	res := Run(context.Background(), Config{MaxRounds: 2}, "m1", "sys", nil,
		Deps{Infer: scriptedInfer(script, nil), Exec: exec, Checkpoints: st, RunID: runID, Session: sess})
	if res.ExitKind != "max_rounds" {
		t.Fatalf("前置条件不成立：应轮数用尽退出，实得 %q（err=%s）", res.ExitKind, res.Err)
	}
	if res.CheckpointErr != "" {
		t.Fatalf("快照不该失败：%s", res.CheckpointErr)
	}
	if execN != 2 {
		t.Fatalf("前置条件：两次工具执行，实得 %d", execN)
	}

	metas, err := st.ListCheckpoints(runID)
	if err != nil {
		t.Fatalf("ListCheckpoints：%v", err)
	}
	if len(metas) != 2 || metas[0].StepIndex != 1 || metas[1].StepIndex != 2 {
		t.Fatalf("每步一份快照：期望 steps=[1,2]，实得 %+v", metas)
	}
	if metas[1].ExitKind != "max_rounds" || metas[1].Lines != 2 {
		t.Fatalf("第 2 步应有 2 行（步快照 + 终局快照），终局带 ExitKind：%+v", metas[1])
	}
	if metas[0].ExitKind != "" || metas[0].Lines != 1 {
		t.Fatalf("第 1 步是步中快照（无终局）：%+v", metas[0])
	}

	// ── 模拟杀进程：**另起一个 store 实例**（新进程：没有内存态、没有打开的句柄）──
	st2, err := NewCheckpointStore(dir, "")
	if err != nil {
		t.Fatalf("建 store2：%v", err)
	}
	snap, err := st2.Load(runID)
	if err != nil {
		t.Fatalf("杀进程后 Load：%v", err)
	}
	if snap.StepIndex != 2 {
		t.Fatalf("恢复点必须停在同一步（step=2）：实得 %d", snap.StepIndex)
	}
	if len(snap.State.Traces) != 2 || len(snap.State.Messages) != 4 {
		t.Fatalf("状态必须完整（2 条轨迹 / 4 条消息）：traces=%d messages=%d",
			len(snap.State.Traces), len(snap.State.Messages))
	}
	if snap.VersionMarker.Schema != CheckpointSchema || snap.VersionMarker.Kernel == "" {
		t.Fatalf("版本标记必须随快照存下（F13）：%+v", snap.VersionMarker)
	}
	if snap.Durability != DurabilitySync {
		t.Fatalf("快照应记录生效的耐久档：%s", snap.Durability)
	}
	// 独立复核：直接读文件字节（不信内存、不信 API），行数与 state_digest 都对得上
	raw, err := os.ReadFile(filepath.Join(dir, runID+".jsonl"))
	if err != nil {
		t.Fatalf("读快照文件：%v", err)
	}
	if n := strings.Count(string(raw), "\n"); n != 3 {
		t.Fatalf("盘上应有 3 行（step1 / step2 / step2 终局）：实得 %d", n)
	}
	if !strings.Contains(string(raw), snap.StateDigest) {
		t.Fatalf("盘上的行里应含 state_digest=%s", snap.StateDigest)
	}

	// ── 从最后一份继续（先认领、再恢复；本例先只验状态接续，lease 见 lease_test.go）──
	rec := &inferRecorder{}
	stepsRun := 0
	exec2 := func(ctx context.Context, name string, args map[string]any) (string, string, error) {
		stepsRun++
		return "续跑输出", "1ms", nil
	}
	st3, _ := NewCheckpointStore(dir, "")
	res2 := Run(context.Background(), Config{MaxRounds: 2}, "m1", "sys", nil, Deps{
		Infer: scriptedInfer([]Response{
			toolRound("c3", "write", "path", "c.txt"),
			{Content: "续跑完成"},
		}, rec),
		Exec: exec2, Checkpoints: st3, RunID: runID, Session: sess, Resume: snap,
	})
	if res2.ExitKind != "natural" || res2.Content != "续跑完成" {
		t.Fatalf("续跑应自然收尾：kind=%q content=%q err=%s", res2.ExitKind, res2.Content, res2.Err)
	}
	if stepsRun != 1 {
		t.Fatalf("续跑只应执行**新的一步**：实得 %d", stepsRun)
	}
	first := rec.firstMsgs()
	if len(first) != 4 {
		t.Fatalf("续跑的第一轮必须接到 4 条历史消息（不是从零开始）：实得 %d", len(first))
	}
	if !strings.Contains(first[1]["content"].(string), "工具输出-read") {
		t.Fatalf("历史里的工具回执必须原样接着：%v", first[1])
	}
	final3, err := st3.Load(runID)
	if err != nil {
		t.Fatalf("续跑后 Load：%v", err)
	}
	if final3.StepIndex != 4 {
		t.Fatalf("续跑应从第 3 步开始并在终局追到第 4 步：实得 %d", final3.StepIndex)
	}
	metas3, _ := st3.ListCheckpoints(runID)
	if len(metas3) != 4 {
		t.Fatalf("续跑后应有 4 步：%+v", metas3)
	}
	// 反例（完成过的 run 不再跑第二遍）：续跑已给终答 ⇒ 再恢复一次必须被拒，且一步都不执行
	st4, _ := NewCheckpointStore(dir, "")
	snapDone, err := st4.Load(runID)
	if err != nil {
		t.Fatalf("Load 终局快照：%v", err)
	}
	if Resumable(snapDone.State.ExitKind) {
		t.Fatalf("natural 终局不该被判为「还有活要干」：%q", snapDone.State.ExitKind)
	}
	rec2 := &inferRecorder{}
	res3 := Run(context.Background(), Config{MaxRounds: 2}, "m1", "sys", nil, Deps{
		Infer: scriptedInfer([]Response{{Content: "不该跑到这里"}}, rec2), Exec: exec2,
		Checkpoints: st4, RunID: runID, Session: sess, Resume: snapDone,
	})
	if res3.ExitKind != "already_done" || rec2.rounds() != 0 {
		t.Fatalf("已完成 run 的再次恢复必须被拒且零推理：kind=%q rounds=%d", res3.ExitKind, rec2.rounds())
	}
}

// ── 用例②：快照损坏 ⇒ 报错不静默 ─────────────────────────────────────────────

func TestCheckpoint_CorruptMustErrorNotSilent(t *testing.T) {
	dir := t.TempDir()
	const runID = "run-corrupt"
	mk := func(t *testing.T, mut func(string) string) string {
		t.Helper()
		d := t.TempDir()
		s, err := NewCheckpointStore(d, DurabilitySync)
		if err != nil {
			t.Fatalf("建 store：%v", err)
		}
		for step := 1; step <= 2; step++ {
			mustSave(t, s, CheckpointState{RunID: runID, Session: "s", StepIndex: step,
				Round: step, Messages: []map[string]any{{"role": "user", "content": "hi"}},
				Model: "m1"})
		}
		if err := s.Close(); err != nil {
			t.Fatalf("Close：%v", err)
		}
		path := filepath.Join(d, runID+".jsonl")
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("读快照：%v", err)
		}
		out := raw
		if mut != nil {
			out = []byte(mut(string(raw)))
		}
		if err := os.WriteFile(path, out, 0o644); err != nil {
			t.Fatalf("写回：%v", err)
		}
		return d
	}

	t.Run("正常快照必须读得出（防把一切都拒了）", func(t *testing.T) {
		d := mk(t, nil)
		s, _ := NewCheckpointStore(d, DurabilitySync)
		snap, err := s.Load(runID)
		if err != nil || snap.StepIndex != 2 {
			t.Fatalf("正常快照应可读：snap=%+v err=%v", snap, err)
		}
	})

	t.Run("改一字节（同长度替换，JSON 仍合法）⇒ 报错且判词指出摘要不符", func(t *testing.T) {
		d := mk(t, func(s string) string { return strings.Replace(s, `"model":"m1"`, `"model":"m2"`, 1) })
		s, _ := NewCheckpointStore(d, DurabilitySync)
		_, err := s.Load(runID)
		if !errors.Is(err, ErrCheckpointCorrupt) {
			t.Fatalf("必须报 ErrCheckpointCorrupt：%v", err)
		}
		if !strings.Contains(err.Error(), "摘要不符") {
			t.Fatalf("判词应指出摘要不符（而不是含糊的解析失败）：%v", err)
		}
		if errors.Is(err, ErrNoCheckpoint) {
			t.Fatal("损坏**不能**被当成「没有快照」（那会让调用方以为没什么可恢复）")
		}
	})

	t.Run("改 step_index ⇒ 整行摘要必须抓到（两层摘要的存在理由）", func(t *testing.T) {
		d := mk(t, func(s string) string { return strings.Replace(s, `"step_index":2`, `"step_index":9`, 1) })
		s, _ := NewCheckpointStore(d, DurabilitySync)
		if _, err := s.Load(runID); !errors.Is(err, ErrCheckpointCorrupt) {
			t.Fatalf("改步号必须被整行摘要抓到：%v", err)
		}
	})

	t.Run("末行截断 ⇒ 报错；AllowTornTail 是唯一显式出口（能读到前一行）", func(t *testing.T) {
		d := mk(t, func(s string) string { return s[:len(s)-20] })
		s, _ := NewCheckpointStore(d, DurabilitySync)
		_, err := s.Load(runID)
		if !errors.Is(err, ErrCheckpointCorrupt) {
			t.Fatalf("截断必须报错（静默跳过最坏：会从你以为存在的状态继续跑）：%v", err)
		}
		lenient, _ := NewCheckpointStore(d, DurabilitySync)
		lenient.AllowTornTail = true
		snap, err := lenient.Load(runID)
		if err != nil || snap.StepIndex != 1 {
			t.Fatalf("显式容忍末行截断时应读到第 1 步：snap=%+v err=%v", snap, err)
		}
	})

	t.Run("行格式版本不认识 ⇒ ErrCheckpointVersion（与「损坏」分得开）", func(t *testing.T) {
		d := mk(t, func(s string) string { return strings.Replace(s, `{"v":1,`, `{"v":99,`, 1) })
		s, _ := NewCheckpointStore(d, DurabilitySync)
		_, err := s.Load(runID)
		if !errors.Is(err, ErrCheckpointVersion) {
			t.Fatalf("必须报 ErrCheckpointVersion：%v", err)
		}
	})

	t.Run("没有快照 ⇒ ErrNoCheckpoint（不是损坏）", func(t *testing.T) {
		s, _ := NewCheckpointStore(t.TempDir(), DurabilitySync)
		if _, err := s.Load("run-none"); !errors.Is(err, ErrNoCheckpoint) {
			t.Fatalf("必须报 ErrNoCheckpoint：%v", err)
		}
		metas, err := s.ListCheckpoints("run-none")
		if err != nil || len(metas) != 0 {
			t.Fatalf("列举没有快照的 run 是空表 + 无错：%+v %v", metas, err)
		}
	})

	t.Run("字段自洽：state 与记录不符 ⇒ 报错", func(t *testing.T) {
		// 把 payload 里的 run_id 改掉（整行摘要会先抓到；这条用于确认"摘要先于自洽检查"）
		d := mk(t, func(s string) string { return strings.Replace(s, `"run_id":"run-corrupt"`, `"run_id":"run-other"`, 1) })
		s, _ := NewCheckpointStore(d, DurabilitySync)
		if _, err := s.Load(runID); !errors.Is(err, ErrCheckpointCorrupt) {
			t.Fatalf("run_id 被改必须报错：%v", err)
		}
	})

	t.Run("参数非法：空 run_id / step<1 / 步号倒退 / 耐久档不认识", func(t *testing.T) {
		s, _ := NewCheckpointStore(t.TempDir(), DurabilitySync)
		if _, err := s.Save(CheckpointState{RunID: "", StepIndex: 1}); !errors.Is(err, ErrCheckpointInvalid) {
			t.Fatalf("空 run_id 应报 ErrCheckpointInvalid：%v", err)
		}
		if _, err := s.Save(CheckpointState{RunID: runID, StepIndex: 0}); !errors.Is(err, ErrCheckpointInvalid) {
			t.Fatalf("step<1（步内不落盘）应报 ErrCheckpointInvalid：%v", err)
		}
		mustSave(t, s, CheckpointState{RunID: runID, StepIndex: 3})
		if _, err := s.Save(CheckpointState{RunID: runID, StepIndex: 2}); !errors.Is(err, ErrCheckpointInvalid) {
			t.Fatalf("步号倒退会让恢复点悄悄后退 ⇒ 必须拒绝：%v", err)
		}
		if _, err := NewCheckpointStore(t.TempDir(), Durability("eventual")); !errors.Is(err, ErrCheckpointInvalid) {
			t.Fatalf("未知耐久档必须报错（不静默降级）：%v", err)
		}
		// 路径安全：run_id 带路径分隔符 ⇒ 不走路径穿越（落到哈希文件名）
		safe, _ := NewCheckpointStore(t.TempDir(), DurabilitySync)
		mustSave(t, safe, CheckpointState{RunID: "../../etc/passwd", StepIndex: 1})
		if _, err := os.Stat(filepath.Join(safe.Dir, "..", "..", "etc", "passwd.jsonl")); err == nil {
			t.Fatal("run_id 不该能穿越目录")
		}
	})

	_ = dir
}

// ── 用例③：耐久档位生效 ────────────────────────────────────────────────────

// syncSpy — 注入的刷盘实现（数次数 + 记文件名 ⇒ "真的刷盘了吗"变成可判事实）。
type syncSpy struct {
	mu    sync.Mutex
	calls int
	files []string
}

func (s *syncSpy) fn() func(*os.File) error {
	return func(f *os.File) error {
		s.mu.Lock()
		s.calls++
		s.files = append(s.files, f.Name())
		s.mu.Unlock()
		return fullSync(f) // 真刷（不是 no-op）
	}
}

func (s *syncSpy) n() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

func TestCheckpoint_DurabilityLevels(t *testing.T) {
	const runID = "run-dur"
	st := func(runID string) CheckpointState {
		return CheckpointState{RunID: runID, Session: "s", StepIndex: 1, Round: 1,
			Messages: []map[string]any{{"role": "user", "content": "x"}}, Model: "m1"}
	}

	t.Run("sync：写入即持久（刷了盘 + 新实例立刻读得到）", func(t *testing.T) {
		dir := t.TempDir()
		s, err := NewCheckpointStore(dir, DurabilitySync)
		if err != nil {
			t.Fatalf("建 store：%v", err)
		}
		spy := &syncSpy{}
		s.syncFn = spy.fn()
		mustSave(t, s, st(runID))
		if spy.n() == 0 {
			t.Fatal("sync 档必须真的刷盘（否则「写入即持久」是句空话）")
		}
		// 另起实例（= 新进程）立即可读 ⇒ 这就是"sync 下写入即持久"的可判形式
		fresh, _ := NewCheckpointStore(dir, DurabilitySync)
		snap, err := fresh.Load(runID)
		if err != nil || snap.StepIndex != 1 || snap.Durability != DurabilitySync {
			t.Fatalf("sync 档应立即可见：snap=%+v err=%v", snap, err)
		}
	})

	t.Run("async：不刷盘，但进程被杀也读得到（数据在 OS 手里）", func(t *testing.T) {
		dir := t.TempDir()
		s, err := NewCheckpointStore(dir, DurabilityAsync)
		if err != nil {
			t.Fatalf("建 store：%v", err)
		}
		spy := &syncSpy{}
		s.syncFn = spy.fn()
		mustSave(t, s, st(runID))
		if spy.n() != 0 {
			t.Fatalf("async 档不该 fsync（这正是它比 sync 便宜的地方）：calls=%d", spy.n())
		}
		fresh, _ := NewCheckpointStore(dir, DurabilityAsync)
		snap, err := fresh.Load(runID)
		if err != nil || snap.StepIndex != 1 || snap.Durability != DurabilityAsync {
			t.Fatalf("async 档应写到文件（杀进程不丢）：snap=%+v err=%v", snap, err)
		}
	})

	t.Run("exit：未 Flush 前**新进程读不到**；Flush 后读得到（语义完整，不是「没实现」）", func(t *testing.T) {
		dir := t.TempDir()
		s, err := NewCheckpointStore(dir, DurabilityExit)
		if err != nil {
			t.Fatalf("建 store：%v", err)
		}
		spy := &syncSpy{}
		s.syncFn = spy.fn()
		mustSave(t, s, st(runID))
		if spy.n() != 0 {
			t.Fatalf("exit 档在 Flush 之前不该有任何刷盘：calls=%d", spy.n())
		}
		// 同实例看得到（内存里），新实例看不到（= 杀进程后丢了）—— 这两个断言合起来才是 exit 档的真实语义
		if snap, err := s.Load(runID); err != nil || snap.StepIndex != 1 {
			t.Fatalf("同实例应看得到内存里的快照：%+v %v", snap, err)
		}
		killed, _ := NewCheckpointStore(dir, DurabilityExit)
		if _, err := killed.Load(runID); !errors.Is(err, ErrNoCheckpoint) {
			t.Fatalf("杀进程（新实例）应读不到未 Flush 的快照：%v", err)
		}
		if err := s.Flush(); err != nil {
			t.Fatalf("Flush：%v", err)
		}
		if spy.n() == 0 {
			t.Fatal("Flush 必须真刷盘（它唯一的语义就是「现在要它在盘上」）")
		}
		after, _ := NewCheckpointStore(dir, DurabilityExit)
		snap, err := after.Load(runID)
		if err != nil || snap.StepIndex != 1 || snap.Durability != DurabilityExit {
			t.Fatalf("Flush 后新实例应读得到：snap=%+v err=%v", snap, err)
		}
		// 幂等：再 Flush 一次不重复写
		raw1, _ := os.ReadFile(filepath.Join(dir, runID+".jsonl"))
		if err := s.Flush(); err != nil {
			t.Fatalf("二次 Flush：%v", err)
		}
		raw2, _ := os.ReadFile(filepath.Join(dir, runID+".jsonl"))
		if string(raw1) != string(raw2) {
			t.Fatal("Flush 必须幂等（重复调用不该重复落盘）")
		}
	})
}

// 快照写入失败必须如实上报（不静默：磁盘/权限问题会让"恢复点"不成立）。
func TestCheckpoint_SaveFailureIsReportedNotSilent(t *testing.T) {
	// 用一个**文件**当目录 ⇒ MkdirAll 必失败
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatalf("造夹具：%v", err)
	}
	s, err := NewCheckpointStore(filepath.Join(blocker, "cp"), DurabilitySync)
	if err != nil {
		t.Fatalf("建 store：%v", err)
	}
	events := []string{}
	res := Run(context.Background(), Config{MaxRounds: 1}, "m1", "sys", nil, Deps{
		Infer: scriptedInfer([]Response{toolRound("c1", "read", "path", "a")}, nil),
		Exec: func(ctx context.Context, name string, args map[string]any) (string, string, error) {
			return "out", "1ms", nil
		},
		Checkpoints: s, RunID: "run-fail", Session: "sess",
		Events: func(ev, payload string) { events = append(events, ev) },
	})
	if res.CheckpointErr == "" {
		t.Fatal("快照写不成必须如实记进 Result（不许静默）")
	}
	found := false
	for _, ev := range events {
		if ev == "checkpoint_failed" {
			found = true
		}
	}
	if !found {
		t.Fatalf("必须发 checkpoint_failed 事件：%v", events)
	}
	if res.ExitKind != "max_rounds" {
		t.Fatalf("快照失败不该改变循环行为（它不是一个新失败模式）：%q", res.ExitKind)
	}
}
