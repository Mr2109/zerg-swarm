// lease.go — T5.6：**同一会话「并发恢复」互斥**（session 级 lease / 执行前置门）
// （设计稿 docs/01-设计/设计-内建调试版-v1.2-20260917.md §〇 F7 ★★★ + Q16）
//
// ── 要治的缺口（实测，不是推测）──
//
// 「k 个恢复者 ⇒ **k 次受闸副作用**；36/40 格 saturation=1.0；跨主机 10/10；
//
//	窗口 = 节点自身执行时间」—— 也就是说：只要两个恢复者**同时**跑起来，
//	同一批节点就会被执行两遍，而两边的日志都写着"恢复成功"。
//	这类双跑**不会**被"事后去重"救回来：副作用已经发生了。
//
// ── 所以本层的语义只有一句（写进注释也写进用例）──
//
//	**抢占必须在任何节点执行之前完成**：lease 是**执行前置门**，不是事后去重器。
//	同一 session 的第二个获取者 ⇒ **直接拒绝**（专项错误码 ErrLeaseHeld + **当前持有者**），
//	于是一条节点都不会执行 —— 这正好与 F6 的边界对齐：跨中断 exactly-once、跨崩溃 at-least-once。
//
// ── 为什么是 session 级，不是 run 级 / 步骤级 ──
//
//	· 恢复的入口是**会话**（人来点"继续"、程序按会话重放），不是某个 run id ——
//	  按 run 加锁会漏掉"同一会话开了两个 run 都去恢复"这种真实的双跑。
//	· 步骤级太细：步骤之间的窗口照样允许两条恢复路径交替推进同一个会话。
//
// ── 落盘与互斥实现（如实）──
//
//	· 租约记录**必须落盘**（它要跨进程、跨"上一次崩溃"存活）⇒ 写临时文件 → fsync → rename → fsync 目录。
//	· 读改写全程持锁：进程内 mutex + **跨进程 flock**（锁文件常驻不删 —— 删锁文件会与并发持有者竞态，
//	  同 internal/memory 的写入安全层纪律）。
//	· 记录损坏/版本不认识 ⇒ **fail-closed**（拒绝对该会话发租约）：此时"发出去"比"发不出去"更危险 ——
//	  发出去就等于允许两个执行者同时跑（F2 的"解析失败按最严"同款）。
//	· **本实现是本机的**（flock + 本地文件）。跨主机互斥需要共享文件系统或外部 KV/etcd 那一档 ——
//	  设计稿实测的"跨主机 10/10"正说明这一档必须补，属**未接项**（见本文件末尾）。
//
// ── TTL 与"接管" ──
//
//	· TTL 到期 ⇒ 座位空出，**另一方可以接管**（Acquire 成功，fence 递增）。
//	· 已过期 ⇒ **Renew 拒绝**（"过期后悄悄复活"是危险的：期间可能已被接管）；
//	  要接着干就重新 Acquire（fence+1，别人拿不到 ⇒ 安全）。
//	· 时间一律**注入**（Acquire/Renew/Release/Holder 都收 now 参数；store 另有可注入的 Now 兜底）——
//	  租约判定不读系统钟，于是 TTL 到期/接管这些事**可测、可复现**。
package loopcore

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
)

// LeaseSchema —— 租约记录格式版本（不认识 ⇒ 拒用该会话的租约，fail-closed）。
const LeaseSchema = 1

// DefaultLeaseTTL —— 默认租约时长（Deps.LeaseTTL ≤0 时用它）。取 5 分钟：
// 长于绝大多数"一步"（推理 + 工具），短于"人发现异常并介入"的时间尺度。
const DefaultLeaseTTL = 5 * time.Minute

// 哨兵错误：调用方用 errors.Is 分流。
var (
	// ErrLeaseHeld —— 该会话已被**其他**恢复者持有（抢占被拒；错误里带当前持有者）。
	ErrLeaseHeld = errors.New("loopcore: 会话已被其他恢复者持有（并发恢复被拒，未执行任何节点）")
	// ErrLeaseNotHolder —— 续期/释放时不是当前持有者（可能已被接管或已过期）。
	ErrLeaseNotHolder = errors.New("loopcore: 不是该会话租约的当前持有者")
	// ErrLeaseCorrupt —— 租约记录损坏/摘要不符/版本不认识（fail-closed：拒绝发租约）。
	ErrLeaseCorrupt = errors.New("loopcore: 租约记录损坏")
	// ErrLeaseInvalid —— 参数非法（空 session/holder、ttl ≤0）。
	ErrLeaseInvalid = errors.New("loopcore: 租约参数非法")
)

// Lease —— 一份 session 级租约（落盘事实）。
//
// Fence 是**认领代数**：每次成功取得 +1（接管也算；续期不变）。持有者应记住自己拿到的 fence ——
// 用它识别"我是不是已经被别人接管了"（陈旧持有者的写必须被判无效；
// 与 F6 的效果账本/幂等键配合就是完整的"先认领后执行"）。
//
// **单调性的确切范围（不夸大）**：fence 在**一整段未释放的持有序列**内单调递增；
// **Release 之后座位空出、代数从 1 重新开始**（记录被删除 ⇒ 没有"历史代数"可依）。
// 挡住陈旧持有者的第一道防线因此是**持有者身份**（Renew/Release 都要求 holder 自己）：
// 被接管或被释放之后，前任的 Renew/Release 一律 ErrLeaseNotHolder —— 这条不依赖 fence。
// 若要跨 Release 也识别"上一代的陈旧执行者"，需要持久化 epoch（见文件末"未接项"）。
type Lease struct {
	V          int       `json:"v"`
	SessionID  string    `json:"session_id"`
	Holder     string    `json:"holder"`
	Fence      uint64    `json:"fence"`
	AcquiredAt time.Time `json:"acquired_at"`
	ExpiresAt  time.Time `json:"expires_at"`
	TTLMillis  int64     `json:"ttl_ms"`
}

// Active —— 在 now 时刻该租约是否仍然有效（**判据只有时间**：本层不猜持有者是否还活着）。
func (l *Lease) Active(now time.Time) bool {
	if l == nil {
		return false
	}
	return now.Before(l.ExpiresAt)
}

// HeldError —— 抢占/续期/释放被拒的**专项错误**：带原因 + **当前持有者**（谁挡住了我，必须能答）。
// errors.Is(err, ErrLeaseHeld) / errors.Is(err, ErrLeaseNotHolder) 都可用（Unwrap）。
type HeldError struct {
	Lease  *Lease // 当前租约（nil = 不存在/已释放）
	Reason string // 人读判词
	cause  error  // ErrLeaseHeld / ErrLeaseNotHolder
}

func (e *HeldError) Error() string {
	holder, until := "（无）", "（无）"
	if e.Lease != nil {
		holder = e.Lease.Holder
		until = e.Lease.ExpiresAt.Format(time.RFC3339Nano)
	}
	return fmt.Sprintf("%v：当前持有者=%s 到期=%s（原因：%s）", e.cause, holder, until, e.Reason)
}

func (e *HeldError) Unwrap() error { return e.cause }

// Ledger 语义提醒：本层的 fence 与 T5.9 审计层的 IdempotencyKey 是两件事 ——
// fence 说"谁是当前唯一执行者"，幂等键说"这个副作用做没做过"。两者都要有，不能互相替代。

// LeaseStore —— session 级租约存储（本机：本地文件 + flock；跨主机见文件头"未接项"）。
type LeaseStore struct {
	// Dir —— 租约目录（空 = statepath.File("loopcore/leases")）。
	Dir string
	// Now —— 时钟兜底（各方法传入的 now 为零值时用它；它也为零 ⇒ time.Now）。
	Now func() time.Time
	// syncFn —— 注入的刷盘实现（默认 fullSync；测试用它钉住"租约记录真的落盘了"）。
	syncFn func(*os.File) error

	mu        sync.Mutex
	dirSynced bool
}

// NewLeaseStore —— dir 空 = statepath.File("loopcore/leases")。
func NewLeaseStore(dir string) *LeaseStore {
	if strings.TrimSpace(dir) == "" {
		dir = statepath.File(filepath.Join("loopcore", "leases"))
	}
	return &LeaseStore{Dir: dir}
}

func (s *LeaseStore) resolveNow(now time.Time) time.Time {
	if !now.IsZero() {
		return now
	}
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// Acquire —— **执行前置门**：认领该会话（必须在任何节点执行之前调用）。
//
// 语义（每条都有对应用例）：
//
//	① 无租约 ⇒ 取得（fence=1）。
//	② 有租约且**未过期**且持有者是别人 ⇒ **拒绝**（*HeldError，errors.Is ErrLeaseHeld，带当前持有者）。
//	   —— 这条就是"k 个恢复者 ⇒ k 次副作用"的解药：落败者在**这一步**就出局，一条节点都不执行。
//	③ 有租约但**已过期**（now ≥ ExpiresAt）⇒ 接管（fence+1）。
//	④ 持有者是自己 ⇒ 续期（fence+1，视为一次新的取得；持有者以最新返回的 fence 为准）。
//	⑤ 记录损坏/版本不认识 ⇒ **拒绝**（ErrLeaseCorrupt）：宁可谁都别跑，也不能放两个执行者进来。
//
// 时间用**注入的 now**（传零值 ⇒ 用 store 的时钟）。
func (s *LeaseStore) Acquire(sessionID, holder string, ttl time.Duration, now time.Time) (*Lease, error) {
	at := s.resolveNow(now)
	if err := validateLeaseArgs(sessionID, holder, ttl); err != nil {
		return nil, err
	}
	var out *Lease
	err := s.withLock(sessionID, func() error {
		cur, err := s.readLocked(sessionID)
		if err != nil {
			return err // fail-closed（损坏 ⇒ 不发租约）
		}
		if cur != nil && cur.Holder != holder && at.Before(cur.ExpiresAt) {
			return &HeldError{Lease: cur, cause: ErrLeaseHeld,
				Reason: "该会话已有活跃恢复者（抢占必须在任何节点执行之前完成；落败者不执行、不写状态）"}
		}
		fence := uint64(1)
		if cur != nil {
			// fence 递增有两种情形，都记在同一处（不另外加字段，避免两份真相）：
			//   同一持有者再次取得（续期）/ 前任已过期而被接管。
			fence = cur.Fence + 1
		}
		nl := &Lease{V: LeaseSchema, SessionID: sessionID, Holder: holder, Fence: fence,
			AcquiredAt: at, ExpiresAt: at.Add(ttl), TTLMillis: ttl.Milliseconds()}
		if err := s.writeLocked(nl); err != nil {
			return err
		}
		out = nl
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Enter —— Acquire 的**执行前置门**形态：拿到租约并返回释放函数（调用方 defer release()）。
// 用它比手工 Acquire/Release 更难写错顺序：**门没拿到就直接返回错误**，调用方拿不到执行的机会。
func (s *LeaseStore) Enter(sessionID, holder string, ttl time.Duration, now time.Time) (release func(), lease *Lease, err error) {
	l, err := s.Acquire(sessionID, holder, ttl, now)
	if err != nil {
		return nil, nil, err
	}
	return func() { _ = s.Release(sessionID, holder) }, l, nil
}

// Renew —— 续期（**只对当前持有者**、**只在未过期时**）。
// 已过期 ⇒ 拒绝（ErrLeaseNotHolder）：过期后的座位可能已被接管，"悄悄复活"会让两个执行者同时在跑。
func (s *LeaseStore) Renew(sessionID, holder string, ttl time.Duration, now time.Time) (*Lease, error) {
	at := s.resolveNow(now)
	if err := validateLeaseArgs(sessionID, holder, ttl); err != nil {
		return nil, err
	}
	var out *Lease
	err := s.withLock(sessionID, func() error {
		cur, err := s.readLocked(sessionID)
		if err != nil {
			return err
		}
		if cur == nil {
			return &HeldError{cause: ErrLeaseNotHolder, Reason: "租约不存在（未取得或已被释放）"}
		}
		if cur.Holder != holder {
			return &HeldError{Lease: cur, cause: ErrLeaseNotHolder, Reason: "持有者不匹配（可能已被接管）"}
		}
		if !at.Before(cur.ExpiresAt) {
			return &HeldError{Lease: cur, cause: ErrLeaseNotHolder,
				Reason: "租约已过期 ⇒ 拒绝续期（需重新 Acquire：过期期间座位可能已被接管）"}
		}
		nl := &Lease{V: LeaseSchema, SessionID: sessionID, Holder: holder, Fence: cur.Fence,
			AcquiredAt: cur.AcquiredAt, ExpiresAt: at.Add(ttl), TTLMillis: ttl.Milliseconds()}
		if err := s.writeLocked(nl); err != nil {
			return err
		}
		out = nl
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Release —— 释放（**只允许持有者释放自己的**；不是持有者 ⇒ ErrLeaseNotHolder，
// **绝不静默删掉别人的租约** —— 那等于把门给拆了）。不存在 ⇒ 幂等返回 nil。
func (s *LeaseStore) Release(sessionID, holder string) error {
	if strings.TrimSpace(sessionID) == "" || strings.TrimSpace(holder) == "" {
		return fmt.Errorf("%w: session/holder 为空", ErrLeaseInvalid)
	}
	return s.withLock(sessionID, func() error {
		cur, err := s.readLocked(sessionID)
		if err != nil {
			return err
		}
		if cur == nil {
			return nil
		}
		if cur.Holder != holder {
			return &HeldError{Lease: cur, cause: ErrLeaseNotHolder, Reason: "只允许持有者释放自己的租约"}
		}
		if err := os.Remove(s.pathLocked(sessionID)); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("loopcore: 删除租约 %s：%w", s.pathLocked(sessionID), err)
		}
		_ = s.syncDir()
		return nil
	})
}

// Holder —— 查询当前租约（不存在 ⇒ (nil, nil)）。**原样返回**（含已过期者）：
// 是否仍有效由调用方用 Lease.Active(now) 判 —— 本层不替它猜"持有者还活着吗"。
func (s *LeaseStore) Holder(sessionID string) (*Lease, error) {
	if strings.TrimSpace(sessionID) == "" {
		return nil, fmt.Errorf("%w: session 为空", ErrLeaseInvalid)
	}
	var out *Lease
	err := s.withLock(sessionID, func() error {
		cur, err := s.readLocked(sessionID)
		if err != nil {
			return err
		}
		out = cur
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ── 内部：锁 / 读 / 写 ──────────────────────────────────────────────────────

// withLock —— 进程内 mutex + 跨进程 flock（锁文件常驻不删）。
func (s *LeaseStore) withLock(sessionID string, fn func() error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	lockPath := filepath.Join(s.Dir, "locks", safeFileName(sessionID)+".lock")
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		return fmt.Errorf("loopcore: 建租约目录 %s：%w", filepath.Dir(lockPath), err)
	}
	lf, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return fmt.Errorf("loopcore: 打开租约锁 %s：%w", lockPath, err)
	}
	defer lf.Close()
	if err := syscall.Flock(int(lf.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("loopcore: 加租约锁 %s：%w", lockPath, err)
	}
	defer func() { _ = syscall.Flock(int(lf.Fd()), syscall.LOCK_UN) }()
	return fn()
}

func (s *LeaseStore) pathLocked(sessionID string) string {
	return filepath.Join(s.Dir, safeFileName(sessionID)+".lease.json")
}

// leaseRecord —— 落盘形态：v + 记录摘要 + 租约原始字节（摘要**在字节层**复算 ⇒ 改一个字节就抓到）。
type leaseRecord struct {
	V      int             `json:"v"`
	Digest string          `json:"record_digest"`
	Lease  json.RawMessage `json:"lease"`
}

func (s *LeaseStore) readLocked(sessionID string) (*Lease, error) {
	data, err := os.ReadFile(s.pathLocked(sessionID))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // 无租约（**不是**损坏 —— 两者对调用方的含义完全不同）
		}
		return nil, fmt.Errorf("loopcore: 读租约 %s：%w", s.pathLocked(sessionID), err)
	}
	var rec leaseRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return nil, fmt.Errorf("%w: %s 不是合法 JSON（%v）—— fail-closed：拒绝发租约",
			ErrLeaseCorrupt, s.pathLocked(sessionID), err)
	}
	if rec.V != LeaseSchema {
		return nil, fmt.Errorf("%w: %s 版本 v=%d（本实现只认 v=%d）—— fail-closed",
			ErrLeaseCorrupt, s.pathLocked(sessionID), rec.V, LeaseSchema)
	}
	if len(rec.Lease) == 0 || digestBytes(rec.Lease) != rec.Digest {
		return nil, fmt.Errorf("%w: %s 记录摘要不符 —— fail-closed", ErrLeaseCorrupt, s.pathLocked(sessionID))
	}
	var l Lease
	if err := json.Unmarshal(rec.Lease, &l); err != nil {
		return nil, fmt.Errorf("%w: %s 租约体反序列化失败（%v）", ErrLeaseCorrupt, s.pathLocked(sessionID), err)
	}
	if l.SessionID != sessionID {
		return nil, fmt.Errorf("%w: %s 里的 session_id=%q 与请求 %q 不符",
			ErrLeaseCorrupt, s.pathLocked(sessionID), l.SessionID, sessionID)
	}
	if strings.TrimSpace(l.Holder) == "" || l.Fence == 0 || l.ExpiresAt.IsZero() {
		return nil, fmt.Errorf("%w: %s 租约字段自相矛盾（holder/fence/expires_at）", ErrLeaseCorrupt, s.pathLocked(sessionID))
	}
	return &l, nil
}

// writeLocked —— 原子写（temp → fsync → rename → fsync 目录）。
func (s *LeaseStore) writeLocked(l *Lease) error {
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		return fmt.Errorf("loopcore: 建租约目录 %s：%w", s.Dir, err)
	}
	body, err := json.Marshal(l)
	if err != nil {
		return fmt.Errorf("%w: 租约序列化失败：%v", ErrLeaseInvalid, err)
	}
	recBytes, err := json.Marshal(leaseRecord{V: LeaseSchema, Digest: digestBytes(body), Lease: body})
	if err != nil {
		return fmt.Errorf("%w: 租约记录序列化失败：%v", ErrLeaseInvalid, err)
	}
	path := s.pathLocked(l.SessionID)
	tmp, err := os.CreateTemp(s.Dir, ".lease-*.tmp")
	if err != nil {
		return fmt.Errorf("loopcore: 建临时租约文件：%w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(recBytes); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("loopcore: 写临时租约文件：%w", err)
	}
	// 租约必须**真落盘**才返回（它要在崩溃后继续挡住第二个执行者）
	sync := s.syncFn
	if sync == nil {
		sync = fullSync
	}
	if err := sync(tmp); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("loopcore: 租约刷盘失败（不能「看起来认领成功」）：%w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("loopcore: 关闭临时租约文件：%w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("loopcore: 落租约 %s：%w", path, err)
	}
	return s.syncDir()
}

func (s *LeaseStore) syncDir() error {
	if s.dirSynced {
		return nil
	}
	d, err := os.Open(s.Dir)
	if err != nil {
		return nil // 目录刷不动不阻断（数据文件本身的 fsync 才是主保证）
	}
	defer d.Close()
	_ = d.Sync()
	s.dirSynced = true
	return nil
}

// validateLeaseArgs —— 参数校验（空 session / 空 holder / ttl ≤0 一律**报错**：
// "holder 为空当成功"会让两个无名执行者都以为自己拿到了门）。
func validateLeaseArgs(sessionID, holder string, ttl time.Duration) error {
	if strings.TrimSpace(sessionID) == "" {
		return fmt.Errorf("%w: session 为空（session 级互斥按会话归属，空会话不该有租约）", ErrLeaseInvalid)
	}
	if strings.TrimSpace(holder) == "" {
		return fmt.Errorf("%w: holder 为空（说不清「谁持有」的租约不是租约）", ErrLeaseInvalid)
	}
	if ttl <= 0 {
		return fmt.Errorf("%w: ttl=%s（租约必须有到期时间，否则崩溃一次就永久锁死）", ErrLeaseInvalid, ttl)
	}
	return nil
}

// defaultHolder —— 认领者标识兜底（host/pid）：Deps.Holder 为空时用。
// 为什么不是"空着也算"：认领者说不清自己是谁 ⇒ 接管与排障都无从谈起（也不拿它当判定依据）。
func defaultHolder() string {
	h, err := os.Hostname()
	if err != nil || strings.TrimSpace(h) == "" {
		h = "unknown-host"
	}
	return fmt.Sprintf("%s/%d", h, os.Getpid())
}

// ── 未接项（如实，不假装已做） ──────────────────────────────────────────────
//
//	· **跨主机互斥**：本实现靠本机文件 + flock。设计稿实测的"跨主机 10/10 双跑"要根治，
//	  需要把同一套语义坐到共享文件系统（NFS/共享卷）或外部 KV（etcd/redis，带 TTL 与 CAS）上。
//	  接口形状不变（Acquire/Renew/Release/Holder），只需换后端。
//	· **心跳自动续期**：长跑的恢复者应在 TTL 内定时 Renew（本层只提供 Renew，不替调用方起定时器）。
//	· **跨 Release 的陈旧识别**：Release 会删记录 ⇒ fence 归 1。要跨"释放/重建"识别上一代执行者，
//	  需要一个**只增不减的 epoch**（独立 sidecar 或外部 KV）。当前靠持有者身份挡（见 Lease 注释），
//	  如实记在此处，不假装已经有。
//	· **效果账本接线**：lease 保证"同一时刻只有一个执行者"，跨崩溃的重跑仍须靠幂等键
//	  （internal/replay 的 EffectKey/Ledger 与 internal/audit 的 IdempotencyKey）。
