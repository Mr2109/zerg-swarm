// checkpoint.go — T5.5：**检查点粒度 + 耐久级别 + 版本标记**
// （设计稿 docs/01-设计/设计-内建调试版-v1.2.md §〇 F5/F6/F13 + Q15）
//
// ── 要治的缺口（F5 ★★★）──
//
// 「缺检查点粒度与耐久级别（而它是 pause/resume 的地基）」。**粒度含糊 = 恢复点含糊，
//
//	耐久含糊 = "看起来存了、杀进程后没有"** —— 这两种含糊都不会报错，只会在真要恢复的那天
//
// 变成一句"当时应该存了吧"。所以本文件把两件事**显式选定**，写进类型而不写进散文：
//
//	① **粒度：每步一份**（Q15 定案）。一步 = 一轮（一次推理 + 该轮全部工具执行）收尾后的完整状态。
//	   步内（推理中/工具执行中）**不落盘**：半个状态既不可执行也不可解释（"执行到一半"没有语义）。
//	② **耐久：sync | async | exit 三档，默认 sync**（口径见 Durability —— 三档的区别是
//	   "进程被杀之后还剩什么"，不是"性能选项"）。
//	③ **版本标记**（F13）：内核版本 / 行格式版本 / 模型与状态一起存；**行格式版本不认识 ⇒ 报错**，
//	   绝不按别的版本瞎猜（设计要求就是"恢复时路由到匹配的反序列化路径"）。
//
// ── 副作用与快照的原子性边界（F6；本文件只负责快照这一半，边界在此写死）──
//
//	**跨中断 exactly-once、跨崩溃 at-least-once**。判据落在**步边界**上：本文件的快照只在步边界写，
//	所以"从快照继续"永远不会接在半个副作用上；崩溃丢掉的是"没走完的那一步"，重跑该步的副作用
//	必须靠**效果账本 + 幂等键**（T4.1 internal/replay / T5.9 internal/audit）收敛 ——
//	本文件**不假装**自己能提供跨崩溃的 exactly-once（那是幂等键的活，不是快照的活）。
//
// ── 落盘形态（为什么是 JSONL + 两层摘要）──
//
//	· 每 run 一个文件 `<dir>/<run_id>.jsonl`：**追加写、从不改写历史**。
//	  同一步允许出现多行（步内终局快照例外）⇒ Load 取最后一行；ListCheckpoints 每步取最后一行。
//	· 两层摘要（都**在字节层复算**，不重新序列化再比 ⇒ 不会因 float/键序差异产生假损坏）：
//	  外层 record_digest = sha256(该行 payload 的原始字节) ⇒ **行内任何字段**（含 step_index）
//	  被改一个字节都抓得到；内层 state_digest = sha256(状态负载的原始字节) ⇒ 状态被改同样抓得到。
//	· 任何解析/摘要/版本失败 ⇒ ErrCheckpointCorrupt / ErrCheckpointVersion，**绝不静默跳过**。
//	  "静默跳过最后一行"是最坏的一种假绿：它让你从一份**你以为存在**的状态继续跑。
//	· AllowTornTail 是这条纪律的**唯一显式出口**（默认 false；与 internal/replay 的同类开关同款）：
//	  只有调用方**明确声明**"我知道上次是断电崩溃、末行可能被截断"时才容忍末行受损。
//
// ── 步号单调（为什么敢用内存态判）──
//
//	Save 拒绝"步号倒退"（否则 Load 取最后一行会**悄悄退回更早的一步**）。这条判据由一个前提支撑：
//	同一 session 同一 run **只有一个写者** —— 这正是 T5.6 的 session 级 lease 前置门要保证的事。
//	（进程外并发写同一 run 属误用；lease 的存在就是为了排除它。本文件不重复实现一遍互斥。）
package loopcore

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
	"github.com/Mr2109/zerg-swarm/core/internal/version"
)

// CheckpointSchema —— 快照**行格式**版本（F13：恢复时按它路由到匹配的反序列化路径）。
// 改动任何字段含义都要 +1；不认识的版本一律拒读（见 decodeLine）。
const CheckpointSchema = 1

// 哨兵错误：调用方用 errors.Is 分流（不靠字符串匹配）。
var (
	// ErrNoCheckpoint —— 该 run 没有任何快照（**不是**"损坏"，两者必须分得开：
	// "没有" ⇒ 没什么可恢复；"损坏" ⇒ 有东西但读不了，绝不能当成"没有"往下跑）。
	ErrNoCheckpoint = errors.New("loopcore: 该 run 没有检查点")
	// ErrCheckpointCorrupt —— 快照损坏/截断/摘要不符/字段自相矛盾（**不得静默跳过**）。
	ErrCheckpointCorrupt = errors.New("loopcore: 检查点损坏")
	// ErrCheckpointVersion —— 行格式版本不认识（拒读，不猜）。
	ErrCheckpointVersion = errors.New("loopcore: 检查点版本不认识")
	// ErrCheckpointInvalid —— 参数非法（空 run_id / 步号 <1 / 步号倒退 / 耐久档不认识）。
	ErrCheckpointInvalid = errors.New("loopcore: 检查点参数非法")
)

// Durability —— 耐久分档（F5；三档的差别是"**进程被杀之后还剩什么**"，不是性能旋钮）。
type Durability string

const (
	// DurabilitySync —— **默认档**（Q15）：每一步写入前 fsync（macOS 上走 F_FULLFSYNC，见 durable_darwin.go）
	// 才返回 ⇒ "Save 返回了"就等于"这一步在盘上"。代价是每步一次同步刷盘（先求可恢复，性能后调）。
	DurabilitySync Durability = "sync"
	// DurabilityAsync —— 写进文件（OS 页缓存）即返回，**不 fsync**：进程被杀不丢（数据在 OS 手里），
	// **断电/宕机丢最近的若干步**。适合"每步刷盘太贵、又能接受丢尾部"的重活。
	DurabilityAsync Durability = "async"
	// DurabilityExit —— 全程只在**内存**，进程**正常退出**时 Flush 才落盘：杀进程/崩溃 ⇒
	// 从上次 Flush 之后的**全部**步骤都丢。只在"能接受从头再来"的场景用（默认档不是它，别误选）。
	DurabilityExit Durability = "exit"
)

// normalized —— 空串按**默认 sync** 解（Q15），未知档**报错**（不静默降级成 async：
// 静默降级的失败形态恰好是"以为在盘上、其实不在"）。
func (d Durability) normalized() (Durability, error) {
	switch d {
	case "":
		return DurabilitySync, nil
	case DurabilitySync, DurabilityAsync, DurabilityExit:
		return d, nil
	default:
		return "", fmt.Errorf("%w: 未知耐久档 %q（只支持 sync|async|exit）", ErrCheckpointInvalid, string(d))
	}
}

// VersionMarker —— 状态**版本标记**（F13：审批/暂停可能停放很久，期间内核与模型都会变）。
// 与状态一起存，恢复时先看它：行格式不认识 ⇒ 拒读（本层）；内核/模型变了 ⇒ **如实报出来**
// 交给调用方决定（本层不替它猜"兼容不兼容"）。
type VersionMarker struct {
	Schema int    `json:"schema"`           // = CheckpointSchema（行格式）
	Kernel string `json:"kernel,omitempty"` // 内核版本（internal/version.Tag）
	Commit string `json:"commit,omitempty"` // 构建身份（-ldflags 注入；未注入为 unknown）
	Model  string `json:"model,omitempty"`  // 模型标识（同一状态换模型续跑通常不是同一件事）
}

// DefaultVersionMarker —— 当前进程的版本标记（model 由调用方给；空 = 不声明模型，不编造）。
func DefaultVersionMarker(model string) VersionMarker {
	return VersionMarker{Schema: CheckpointSchema, Kernel: version.Tag, Commit: version.Commit, Model: model}
}

// CheckpointState —— **可序列化的状态负载**（每一步收尾后的完整体）。
//
// 只放"继续跑所需的最小充分集"：消息序列 + 已完成轨迹 + 已完成步数 + 模型。
// **不放**：Deps 里的函数（Infer/Exec/Gate 无法序列化）、采集器句柄、任何"恢复时需要重新接线"的东西 ——
// 恢复是"把状态喂回同一个内核"，不是"把内核序列化"。
type CheckpointState struct {
	RunID       string           `json:"run_id"`
	Session     string           `json:"session,omitempty"`
	StepIndex   int              `json:"step_index"` // 已完成的步数（= 已收尾的轮次）；≥1
	Round       int              `json:"round"`      // 与 StepIndex 同义保留（轮次语义可读）
	Messages    []map[string]any `json:"messages,omitempty"`
	Traces      []Trace          `json:"traces,omitempty"`
	ExitKind    string           `json:"exit_kind,omitempty"` // 空 = 进行中；非空 = 该终局
	TotalTokens int64            `json:"total_tokens,omitempty"`
	Model       string           `json:"model,omitempty"`
}

// Snapshot —— 一份检查点（= 落盘一行的解码结果；字段对齐设计稿 F5 的 `{run_id, step_index,
// state_digest, created_at, durability, version_marker}`，载荷另加 state）。
type Snapshot struct {
	RunID         string          `json:"run_id"`
	StepIndex     int             `json:"step_index"`
	StateDigest   string          `json:"state_digest"`
	CreatedAt     time.Time       `json:"created_at"`
	Durability    Durability      `json:"durability"`
	VersionMarker VersionMarker   `json:"version_marker"`
	State         CheckpointState `json:"state"`
}

// SnapshotMeta —— 列举用的一行元信息（不解析整个状态；Lines = 该步在文件里有几行）。
type SnapshotMeta struct {
	StepIndex   int        `json:"step_index"`
	CreatedAt   time.Time  `json:"created_at"`
	Durability  Durability `json:"durability"`
	StateDigest string     `json:"state_digest"`
	ExitKind    string     `json:"exit_kind,omitempty"`
	Lines       int        `json:"lines"`
}

// cpPayload —— 落盘行的 payload（对外即 Snapshot；内部把 state 保持为**原始字节**以便复算摘要）。
type cpPayload struct {
	RunID         string          `json:"run_id"`
	StepIndex     int             `json:"step_index"`
	StateDigest   string          `json:"state_digest"`
	CreatedAt     time.Time       `json:"created_at"`
	Durability    Durability      `json:"durability"`
	VersionMarker VersionMarker   `json:"version_marker"`
	State         json.RawMessage `json:"state"`
}

// cpRecord —— 落盘行的外层（v + 整行摘要 + payload）。
type cpRecord struct {
	V       int             `json:"v"`
	Digest  string          `json:"record_digest"`
	Payload json.RawMessage `json:"payload"`
}

// CheckpointStore —— 检查点存储（每 run 一个 JSONL；append-only）。
type CheckpointStore struct {
	// Dir —— 落盘目录（空 = statepath.File("loopcore/checkpoints")，乙批 T2 的持久状态目录纪律）。
	Dir string
	// Now —— 时钟注入（nil = time.Now）。时间只用来打 CreatedAt，**不参与任何判定**。
	Now func() time.Time
	// AllowTornTail —— 容忍末行被截断（默认 false = 报错）。唯一出口，见文件头。
	AllowTornTail bool

	durability Durability
	syncFn     func(*os.File) error // 注入点：sync 档每步调它（测试用它钉住"真的刷了盘"）

	mu       sync.Mutex
	files    map[string]*os.File   // runID → 已打开的追加句柄
	pending  map[string][]Snapshot // exit 档：只在内存，Flush 才落盘
	lastStep map[string]int        // runID → 已知最大步号（防步号倒退）
}

// NewCheckpointStore —— dir 空 = statepath.File("loopcore/checkpoints")；durability 空 = **sync**（默认档）。
// 未知耐久档 ⇒ 报错（不静默降级）。
func NewCheckpointStore(dir string, durability Durability) (*CheckpointStore, error) {
	d, err := durability.normalized()
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(dir) == "" {
		dir = statepath.File(filepath.Join("loopcore", "checkpoints"))
	}
	return &CheckpointStore{
		Dir:        dir,
		durability: d,
		files:      map[string]*os.File{},
		pending:    map[string][]Snapshot{},
		lastStep:   map[string]int{},
	}, nil
}

// Durability —— 本 store 实际生效的耐久档（空档构造时已归一到 sync ⇒ 返回值恒为三档之一）。
func (s *CheckpointStore) Durability() Durability { return s.durability }

func (s *CheckpointStore) clockNow() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// ── 写入 ────────────────────────────────────────────────────────────────────

// Save —— 落一份**步快照**（每步收尾调用一次；步号必须 ≥1 且**不得倒退**）。
//
// 耐久语义（Durability）：
//
//	sync  —— 写文件 + fsync 之后才返回；返回即"在盘上"。
//	async —— 写文件（进 OS 页缓存）后返回；杀进程不丢，断电丢尾部。
//	exit  —— **不进文件**，进内存；进程正常退出时必须调 Flush（否则全部丢）。
//
// 失败一律**如实返回**（不吞错）：sync 档连 fsync 都失败 ⇒ 返回错误 ——
// "以为存住了"正是这个任务要治的病。
func (s *CheckpointStore) Save(st CheckpointState) (*Snapshot, error) {
	if s == nil {
		return nil, fmt.Errorf("%w: store 为 nil", ErrCheckpointInvalid)
	}
	if strings.TrimSpace(st.RunID) == "" {
		return nil, fmt.Errorf("%w: run_id 为空（拒绝写一份无法归属的快照）", ErrCheckpointInvalid)
	}
	if st.StepIndex <= 0 {
		return nil, fmt.Errorf("%w: step_index=%d（0 表示一步都没走完，步内不落盘——见文件头粒度口径）",
			ErrCheckpointInvalid, st.StepIndex)
	}
	stateBytes, err := json.Marshal(st)
	if err != nil {
		return nil, fmt.Errorf("%w: 状态无法序列化：%v", ErrCheckpointInvalid, err)
	}
	snap := Snapshot{
		RunID:         st.RunID,
		StepIndex:     st.StepIndex,
		StateDigest:   digestBytes(stateBytes),
		CreatedAt:     s.clockNow(),
		Durability:    s.durability,
		VersionMarker: DefaultVersionMarker(st.Model),
		State:         st,
	}
	payload := cpPayload{
		RunID: snap.RunID, StepIndex: snap.StepIndex, StateDigest: snap.StateDigest,
		CreatedAt: snap.CreatedAt, Durability: snap.Durability,
		VersionMarker: snap.VersionMarker, State: stateBytes,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("%w: 快照无法序列化：%v", ErrCheckpointInvalid, err)
	}
	line, err := json.Marshal(cpRecord{V: CheckpointSchema, Digest: digestBytes(payloadBytes), Payload: payloadBytes})
	if err != nil {
		return nil, fmt.Errorf("%w: 记录无法序列化：%v", ErrCheckpointInvalid, err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if prev := s.lastStep[st.RunID]; prev != 0 && st.StepIndex < prev {
		// 步号倒退 ⇒ Load 取最后一行时会**悄悄退回更早的一步** ⇒ 宁可拒绝写
		return nil, fmt.Errorf("%w: 步号倒退（已写 step=%d，本次 step=%d）", ErrCheckpointInvalid, prev, st.StepIndex)
	}
	if s.durability == DurabilityExit {
		s.pending[st.RunID] = append(s.pending[st.RunID], snap)
		s.lastStep[st.RunID] = st.StepIndex
		return &snap, nil
	}
	f, err := s.fileLocked(st.RunID)
	if err != nil {
		return nil, err
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		return nil, fmt.Errorf("loopcore: 写检查点 %s：%w", s.pathLocked(st.RunID), err)
	}
	if s.durability == DurabilitySync {
		if err := s.syncLocked(f); err != nil {
			return nil, fmt.Errorf("loopcore: 同步检查点 %s（sync 档要求写入即持久，刷盘失败不谎报成功）：%w",
				s.pathLocked(st.RunID), err)
		}
	}
	s.lastStep[st.RunID] = st.StepIndex
	return &snap, nil
}

// Flush —— 把内存里未落盘的快照写出（**exit 档唯一的落盘时机**：进程正常退出时调用）。
// sync/async 档是幂等的 no-op（它们的每次 Save 已经写了文件）。
// Flush 一律走**全同步**（写 + fsync）：它的语义是"我现在就要它落盘"。
func (s *CheckpointStore) Flush() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	runs := make([]string, 0, len(s.pending))
	for runID := range s.pending {
		runs = append(runs, runID)
	}
	sort.Strings(runs) // 顺序确定（不靠 map 迭代序）
	var firstErr error
	for _, runID := range runs {
		snaps := s.pending[runID]
		if len(snaps) == 0 {
			continue
		}
		f, err := s.fileLocked(runID)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		for _, snap := range snaps {
			line, err := encodeSnapshot(snap)
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				break
			}
			if _, err := f.Write(append(line, '\n')); err != nil {
				if firstErr == nil {
					firstErr = err
				}
				break
			}
		}
		if err := s.syncLocked(f); err != nil && firstErr == nil {
			firstErr = err
		}
		delete(s.pending, runID)
	}
	return firstErr
}

// Close —— Flush + 关闭所有句柄（进程正常退出的收口）。
func (s *CheckpointStore) Close() error {
	if s == nil {
		return nil
	}
	err := s.Flush()
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, f := range s.files {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = cerr
		}
		delete(s.files, id)
	}
	return err
}

// encodeSnapshot —— Snapshot → 落盘行字节（与 Save 同一套两层摘要）。
func encodeSnapshot(snap Snapshot) ([]byte, error) {
	stateBytes, err := json.Marshal(snap.State)
	if err != nil {
		return nil, err
	}
	if snap.StateDigest == "" {
		snap.StateDigest = digestBytes(stateBytes)
	}
	payloadBytes, err := json.Marshal(cpPayload{
		RunID: snap.RunID, StepIndex: snap.StepIndex, StateDigest: snap.StateDigest,
		CreatedAt: snap.CreatedAt, Durability: snap.Durability,
		VersionMarker: snap.VersionMarker, State: stateBytes,
	})
	if err != nil {
		return nil, err
	}
	return json.Marshal(cpRecord{V: CheckpointSchema, Digest: digestBytes(payloadBytes), Payload: payloadBytes})
}

// ── 读取 ────────────────────────────────────────────────────────────────────

// Load —— 取该 run **最后一份**快照（= 恢复点）。没有快照 ⇒ ErrNoCheckpoint（与"损坏"分开）。
//
// 内存里尚未落盘的行（exit 档）也在候选内 —— 同进程内接着跑不该看不到自己刚写的状态；
// 而**另一个 store 实例（= 模拟杀进程后重启）看不到它们**，这正是 exit 档的真实语义（见用例③）。
func (s *CheckpointStore) Load(runID string) (*Snapshot, error) {
	snaps, _, err := s.readAll(runID)
	if err != nil {
		return nil, err
	}
	if len(snaps) == 0 {
		return nil, fmt.Errorf("%w: run=%s", ErrNoCheckpoint, runID)
	}
	last := snaps[len(snaps)-1]
	return &last, nil
}

// LoadStep —— 取指定步的最后一份快照（该步不存在 ⇒ ErrNoCheckpoint）。
func (s *CheckpointStore) LoadStep(runID string, step int) (*Snapshot, error) {
	snaps, _, err := s.readAll(runID)
	if err != nil {
		return nil, err
	}
	var found *Snapshot
	for i := range snaps {
		if snaps[i].StepIndex == step {
			cp := snaps[i]
			found = &cp
		}
	}
	if found == nil {
		return nil, fmt.Errorf("%w: run=%s step=%d", ErrNoCheckpoint, runID, step)
	}
	return found, nil
}

// ListCheckpoints —— 列举该 run 的**每一步**（每步取最后一份；按步号升序）。
// 空 run（无快照）⇒ 空切片 + nil 错（"列举"没东西不是错误；"恢复"没东西才是 —— 见 Load）。
func (s *CheckpointStore) ListCheckpoints(runID string) ([]SnapshotMeta, error) {
	_, metas, err := s.readAll(runID)
	if err != nil {
		return nil, err
	}
	return metas, nil
}

// readAll —— 读该 run 的全部行（文件 + 内存 pending），逐行严格校验，返回按行序的快照与按步归并的元信息。
func (s *CheckpointStore) readAll(runID string) ([]Snapshot, []SnapshotMeta, error) {
	if s == nil {
		return nil, nil, fmt.Errorf("%w: store 为 nil", ErrCheckpointInvalid)
	}
	if strings.TrimSpace(runID) == "" {
		return nil, nil, fmt.Errorf("%w: run_id 为空", ErrCheckpointInvalid)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	var snaps []Snapshot
	path := s.pathLocked(runID)
	if data, err := os.ReadFile(path); err == nil {
		lines := bytes.Split(data, []byte{'\n'})
		for i, line := range lines {
			if len(bytes.TrimSpace(line)) == 0 {
				// 空行只在**文件末尾**合法（末行必有 \n ⇒ 最后一段为空）——夹在中间说明文件被动过
				if i == len(lines)-1 {
					continue
				}
				// 半截末行（进程崩在写中途）会落到这里：显式开关才容忍，否则报错
				if s.AllowTornTail && i == len(lines)-2 {
					continue
				}
				return nil, nil, fmt.Errorf("%w: %s 第 %d 行是空行（文件被改写/插入过？）",
					ErrCheckpointCorrupt, path, i+1)
			}
			snap, err := decodeLine(runID, line)
			if err != nil {
				if s.AllowTornTail && i == len(lines)-1 {
					continue // 显式声明容忍末行截断（唯一出口）
				}
				// 版本不认识要能被单独认出来（调用方可能想"换个版本的代码再读一次"），
				// 所以它不被塞进"损坏"里 —— 两个哨兵错误分得开。
				if errors.Is(err, ErrCheckpointVersion) {
					return nil, nil, fmt.Errorf("%s 第 %d 行：%w", path, i+1, err)
				}
				return nil, nil, fmt.Errorf("%w: %s 第 %d 行：%w", ErrCheckpointCorrupt, path, i+1, err)
			}
			snaps = append(snaps, *snap)
		}
	} else if !os.IsNotExist(err) {
		return nil, nil, fmt.Errorf("loopcore: 读检查点 %s：%w", path, err)
	}
	// exit 档：内存里还没落盘的行也算（同进程内"自己刚写的"必须看得到）
	snaps = append(snaps, s.pending[runID]...)

	metas := []SnapshotMeta{}
	idx := map[int]int{} // step → metas 下标
	for _, snap := range snaps {
		if s.lastStep[runID] < snap.StepIndex {
			s.lastStep[runID] = snap.StepIndex
		}
		if at, ok := idx[snap.StepIndex]; ok {
			metas[at].CreatedAt = snap.CreatedAt
			metas[at].StateDigest = snap.StateDigest
			metas[at].ExitKind = snap.State.ExitKind
			metas[at].Lines++
			continue
		}
		idx[snap.StepIndex] = len(metas)
		metas = append(metas, SnapshotMeta{
			StepIndex: snap.StepIndex, CreatedAt: snap.CreatedAt, Durability: snap.Durability,
			StateDigest: snap.StateDigest, ExitKind: snap.State.ExitKind, Lines: 1,
		})
	}
	sort.Slice(metas, func(i, j int) bool { return metas[i].StepIndex < metas[j].StepIndex })
	return snaps, metas, nil
}

// decodeLine —— 严格解码一行（两层摘要 + 版本 + 字段自洽）；任何异常都返回错误，绝不静默跳过。
func decodeLine(wantRunID string, line []byte) (*Snapshot, error) {
	var rec cpRecord
	if err := json.Unmarshal(line, &rec); err != nil {
		return nil, fmt.Errorf("行不是合法 JSON（截断/篡改）：%v", err)
	}
	if rec.V != CheckpointSchema {
		return nil, fmt.Errorf("%w: 行格式 v=%d（本实现只认 v=%d）", ErrCheckpointVersion, rec.V, CheckpointSchema)
	}
	if len(rec.Payload) == 0 {
		return nil, errors.New("payload 缺失")
	}
	if got := digestBytes(rec.Payload); got != rec.Digest {
		return nil, fmt.Errorf("整行摘要不符（期望 %s，实测 %s）", rec.Digest, got)
	}
	var p cpPayload
	if err := json.Unmarshal(rec.Payload, &p); err != nil {
		return nil, fmt.Errorf("payload 不是合法 JSON：%v", err)
	}
	if p.RunID != wantRunID {
		return nil, fmt.Errorf("run_id 不符（行内 %q，请求 %q）", p.RunID, wantRunID)
	}
	if p.StepIndex <= 0 {
		return nil, fmt.Errorf("step_index=%d 非法", p.StepIndex)
	}
	if len(p.State) == 0 {
		return nil, errors.New("state 缺失")
	}
	if got := digestBytes(p.State); got != p.StateDigest {
		return nil, fmt.Errorf("状态摘要不符（期望 %s，实测 %s）", p.StateDigest, got)
	}
	if p.VersionMarker.Schema != CheckpointSchema {
		return nil, fmt.Errorf("%w: 标记里的行格式 v=%d", ErrCheckpointVersion, p.VersionMarker.Schema)
	}
	var st CheckpointState
	if err := json.Unmarshal(p.State, &st); err != nil {
		return nil, fmt.Errorf("state 反序列化失败：%v", err)
	}
	if st.StepIndex != p.StepIndex || st.RunID != p.RunID {
		return nil, fmt.Errorf("state 与记录不自洽（state.run_id=%q step=%d / 记录 run_id=%q step=%d）",
			st.RunID, st.StepIndex, p.RunID, p.StepIndex)
	}
	return &Snapshot{
		RunID: p.RunID, StepIndex: p.StepIndex, StateDigest: p.StateDigest, CreatedAt: p.CreatedAt,
		Durability: p.Durability, VersionMarker: p.VersionMarker, State: st,
	}, nil
}

// ── 内部：路径 / 句柄 / 刷盘 ────────────────────────────────────────────────

func (s *CheckpointStore) pathLocked(runID string) string {
	return filepath.Join(s.Dir, safeFileName(runID)+".jsonl")
}

func (s *CheckpointStore) fileLocked(runID string) (*os.File, error) {
	if f, ok := s.files[runID]; ok {
		return f, nil
	}
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		return nil, fmt.Errorf("loopcore: 建检查点目录 %s：%w", s.Dir, err)
	}
	f, err := os.OpenFile(s.pathLocked(runID), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("loopcore: 打开检查点 %s：%w", s.pathLocked(runID), err)
	}
	// 首次创建时把目录项本身也刷一次（linux/darwin 上 rename/新建的可见性靠它）
	if err := s.syncDirLocked(); err != nil {
		_ = f.Close()
		return nil, err
	}
	s.files[runID] = f
	return f, nil
}

// syncLocked —— 调注入的刷盘实现（默认 fullSync：darwin 上 F_FULLFSYNC）。
func (s *CheckpointStore) syncLocked(f *os.File) error {
	if s.syncFn != nil {
		return s.syncFn(f)
	}
	return fullSync(f)
}

func (s *CheckpointStore) syncDirLocked() error {
	d, err := os.Open(s.Dir)
	if err != nil {
		return nil // 目录刷不动不阻断写入（数据文件本身的 fsync 才是主保证）
	}
	defer d.Close()
	_ = d.Sync()
	return nil
}

// safeFileName —— run_id → 文件名（只允许安全字符，其余一律走哈希；**不做路径穿越**）。
func safeFileName(runID string) string {
	ok := runID != "" && runID != "." && runID != ".."
	for _, r := range runID {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			ok = false
			break
		}
	}
	if ok {
		return runID
	}
	sum := sha256.Sum256([]byte(runID))
	return "h-" + hex.EncodeToString(sum[:8])
}

// digestBytes —— sha256（带 "sha256:" 前缀，便于人读与日志比对）。
func digestBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// Resumable —— 该终局**是否还有活要干**（决定"能不能拿这份快照继续"）。
//
// 判据直接取 terminal.go 的四终局语义（**不另立一套**）：
//
//	""                —— 中途被杀/出错退出（一步的边界上），有活 ⇒ 可继续
//	max_rounds        —— 轮数用尽但**没给终答**（TerminalResume：有进展且有明确待办）⇒ 可继续
//	wall_clock        —— 墙钟用尽，同上 ⇒ 可继续
//	round_timeout     —— 单轮超时后走的是"引导收尾"，仍有待办 ⇒ 可继续
//	stream_broken     —— 流转中断，未给终答 ⇒ 可继续
//	其余（natural / bad_format / empty_args / loopguard_escalate）—— 都已**给出终答** ⇒ 已完成，
//	  再"恢复"一次就是重复劳动（这也是"并发两次恢复 ⇒ 效果恰好一次"的**第二道闸**：
//	  即使两次恢复在时间上错开、lease 已释放，完成过的 run 也不会被再跑一遍）。
func Resumable(exitKind string) bool {
	switch exitKind {
	case "", "max_rounds", "wall_clock", "round_timeout", "stream_broken":
		return true
	}
	return false
}
