// meta.go — 茧（cocoon）元数据真源：`<茧目录>/meta.json`
//
// ★ 四条硬约定（A5 的删除/清扫直依赖这四条，改这里必须同步改 A5）：
//
//	① 权威在盘：meta.json 是茧状态的唯一真源。内存里的 *Meta 只是它的一份拷贝，
//	  "茧还在不在 / 到哪一步了"的回答一律先 Load，不许凭手里的对象回答
//	  （对象可能来自几分钟前，且别的进程可能已经把它推到 deleting/gone）。
//	② 先写 meta 再建目录：进某一步之前，先把这一步写进 meta 并落盘（fsync 过），
//	  再做这一步的副作用。新建茧的顺序是 Save(phase=created) → 再 MkdirAll 茧的工作目录/
//	  解包/绑端口；删除的顺序见 ③。反过来（先建目录、后写 meta）一崩就留下**无主目录**——
//	  清扫器只能靠猜，永远分不清"正在创建"和"创建失败没登记"。
//	③ 先标 deleting 再删：删除流程第一步是 UpdatePhase(path, PhaseDeleting) 落盘，
//	  中间才动手删（工作目录 / 端口 / PID 相关物），最后 UpdatePhase(path, PhaseGone)
//	  并删掉 meta 本身。⇒ 崩在中间最坏留下 phase=deleting 的残骸：**可判、可重入重删**；
//	  绝不会留下"目录已删、meta 还写着 running"的谎（那是清扫器最容易踩的坑）。
//	④ 删目录前先留墓碑：目录一没，`<id>/meta.json` 就跟着没了，盘上再无证据。所以搬平/删除
//	  之前要先 WriteTombstone(root, m) 把同一份 meta 原子落到 `<root>/<id>.meta.json`；
//	  之后"有 meta 无目录"才可判（对账靠 ReadTombstone）。
//
// ★ 写入安全（Rename-Atomic）：
//
//	同目录 tmp → write → fsync(tmp) → close → rename → fsync(dir)。
//	读者要么看到旧 JSON、要么看到新 JSON，**看不到半截**；rename 是原子的，
//	不存在"文件被截断成 0 字节"的窗口。fsync(tmp) 保证 rename 之前内容已在盘上，
//	否则崩溃可能留下"文件名对了、内容是空块"的文件。
//	唯一可能残留的脏件是 `.meta-*.tmp`——它不在真源路径上，读者看不见，可被清扫器按前缀回收。
//
// ★ 并发保护（两级）：
//
//	进程内 = 按绝对路径的互斥锁（同进程两个 goroutine 不会同时进临界区）；
//	跨进程 = flock（锁件 `<meta>.lock`，常驻不删）。**不锁 meta.json 本身**：
//	它每次保存都被 rename 换掉 inode，锁住在旧 inode 上等于没锁。
//	先拿进程内锁再拿 flock，顺序固定（唯一顺序，不会死锁）。
package cocoon

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/unix"
)

const (
	// MetaFileName — 元数据文件名（落在茧目录下）。
	MetaFileName = "meta.json"
	// lockSuffix — flock 锁件后缀：`<meta>.lock`（与真源同目录，常驻不删）。
	lockSuffix = ".lock"
	// tmpPrefix / tmpSuffix — 半截件的命名面（`.meta-<rand>.tmp`）。清扫器按此前缀回收。
	tmpPrefix = ".meta-"
	tmpSuffix = ".tmp"

	dirPerm  = 0o755
	filePerm = 0o644
)

// Phase — 茧的生命周期阶段。单调前进，不允许倒退（SetPhase 校验）；
// 唯一合法的倒退是 failed→starting，只走 ResetForRetry（见该函数）。
type Phase string

const (
	// PhaseCreated — 已登记、目录/资源还没就绪。
	PhaseCreated Phase = "created"
	// PhaseStarting — 正在起（解包/绑端口/拉进程）。
	PhaseStarting Phase = "starting"
	// PhaseRunning — 已起且健康。
	PhaseRunning Phase = "running"
	// PhaseFailed — 起过但已不健康（进程不在/没端口），终态之一；重试必须走 ResetForRetry。
	PhaseFailed Phase = "failed"
	// PhaseStopping — 正在停（优雅停 → 兜底杀）。
	PhaseStopping Phase = "stopping"
	// PhaseDeleting — 已标记待删（A5 的重入点：看到它就说明删到一半崩过）。
	PhaseDeleting Phase = "deleting"
	// PhaseGone — 已删净（此后 meta 也该被删掉；留在盘上只是墓碑）。
	PhaseGone Phase = "gone"
)

// phaseRank — 单调序（值越大越靠后）。只许在链尾追加，**不许改已有序**。
//
//	failed=3 插在 running 之后：running→failed 是一次**真实的失败降级**（进程没了），
//	所以它必须算"前进"才不会被 SetPhase 当倒退拒掉。代价是 failed 之后的档
//	（stopping/deleting/gone）也是前进，符合语义（失败的茧照样要被回收）。
//	反过来 failed→starting 是**倒退**，被 SetPhase 拒——重试是唯一合法的倒退，
//	它不走 SetPhase，走 ResetForRetry（见该函数；靠 epoch 让"第几次尝试"可判）。
var phaseRank = map[Phase]int{
	PhaseCreated:  0,
	PhaseStarting: 1,
	PhaseRunning:  2,
	PhaseFailed:   3,
	PhaseStopping: 4,
	PhaseDeleting: 5,
	PhaseGone:     6,
}

// phaseSeq — 与 phaseRank 同源的链（给校验/日志按序打印）。
var phaseSeq = []Phase{
	PhaseCreated, PhaseStarting, PhaseRunning, PhaseFailed, PhaseStopping, PhaseDeleting, PhaseGone,
}

// Phases — 完整 phase 链（created→starting→running→failed→stopping→deleting→gone）的副本。
func Phases() []Phase {
	out := make([]Phase, len(phaseSeq))
	copy(out, phaseSeq)
	return out
}

// Valid — 是不是已知 phase。
func (p Phase) Valid() bool {
	_, ok := phaseRank[p]
	return ok
}

func (p Phase) rank() (int, bool) {
	r, ok := phaseRank[p]
	return r, ok
}

// String — 落盘用的字面量（空 phase 打印成 "<空>"，便于报错看得懂）。
func (p Phase) String() string {
	if p == "" {
		return "<空>"
	}
	return string(p)
}

var (
	// ErrNotFound — meta 不存在（目录不在，或文件不在）。
	ErrNotFound = errors.New("cocoon: meta 不存在")
	// ErrInvalidMeta — 结构非法（id 空 / phase 未知 / 端口越界 / JSON 解析失败）。
	ErrInvalidMeta = errors.New("cocoon: meta 非法")
	// ErrUnknownPhase — 目标 phase 不在已知链上。
	ErrUnknownPhase = errors.New("cocoon: 未知 phase")
	// ErrPhaseRegression — 目标 phase 排在当前之后（倒退）——被拒。
	ErrPhaseRegression = errors.New("cocoon: phase 不允许倒退")
	// ErrNotRetryable — 当前 phase 不是 failed，不允许重试（ResetForRetry 唯一入口的错误）。
	ErrNotRetryable = errors.New("cocoon: 仅 failed 可重试")
)

// Meta — 茧的元数据。
//
// ★ 不可值拷贝：内嵌互斥锁（保护 SetPhase 的"读-校验-写"与 Save 的取快照）。
//
//	一律以 *Meta 传递；JSON 只认导出字段，锁不进盘。
type Meta struct {
	mu sync.Mutex

	// ID — 茧 ID（全局唯一，也是目录名/寻址键）。
	ID string `json:"id"`
	// Phase — 生命周期阶段（单调，见 SetPhase）。
	Phase Phase `json:"phase"`
	// Epoch — 第几次起：每次 ResetForRetry（failed→starting）加一；缺省 0 = 从没重试过。
	// 用途：让"同一份 meta 的第 N 次尝试"可判（旧 epoch 的进程/端口/日志不许算到新尝试头上）。
	Epoch int `json:"epoch"`
	// PID — 茧主进程 PID；0 = 还没起（或已停）。
	PID int `json:"pid"`
	// StartTime — 首次进 running 的时刻；nil = 还没起过。
	StartTime *time.Time `json:"start_time,omitempty"`
	// Ports — 端口映射：键 = 茧内端口，值 = 宿主端口（A3 端口分配器填）。
	Ports map[int]int `json:"ports,omitempty"`
	// Egg — 卵（模型/镜像）标识。
	Egg string `json:"egg"`
	// SpecSHA — 起茧时那份 `cocoon.toml` 的**原始字节** sha256（小写 hex · 64 位 · 见 speclock.go）。
	// 空 = 未登记（老 meta / 脚本手写 ⇒ 「无从比对」，✗ 不当成一致）。
	// ★ 这一格是判据 9（v2 §E）的唯一真源：`open` 靠它拒掉改动过的契约，`inspect` 靠它报漂移。
	SpecSHA string `json:"spec_sha,omitempty"`
	// SpecDir — 契约所在目录（`<SpecDir>/cocoon.toml`，已 realpath）。
	// `inspect` 只有茧 id、没有项目路径 ⇒ 回读契约的唯一入口就是这一格。
	// 空 = 未登记（与 SpecSHA 同进退；两格有一格空就判「未锁定」）。
	SpecDir string `json:"spec_dir,omitempty"`
	// Backend — 起这个茧的后端名（`fake` / `nsjail`）。空 = 未登记（老 meta / 假后端时代写的）。
	//
	// ★ 为什么要落盘：`close` 是**另一个进程**，它必须知道该把哪个后端的句柄交回去。
	//   只按 GOOS 猜会把「这个茧其实是假后端开的」写成 nsjail 收尾（收了个不存在的东西）。
	Backend string `json:"backend,omitempty"`
	// BackendInstance — 后端**签发**的实例 id（`Instance.ID` 原样存回；✗ 不许自己拼）。
	// 空 = 该后端不落句柄（假后端：实例账只在进程内）+ 老 meta。
	BackendInstance string `json:"backend_instance,omitempty"`
	// BackendPIDs — 后端签发的、当前还在跑的各 exec 主进程 pid（宿主侧）。
	//
	// ★ 跨进程 `close` 的唯一依据：起服务的进程（`cocoon open`）退出后，后端的内存记账
	// 就没了；pid 不落盘，close 只能「什么都没收」却报成功（那是最糟的一种谎）。
	BackendPIDs []int `json:"backend_pids,omitempty"`
	// CreatedAt — 登记时刻（UTC）。
	CreatedAt time.Time `json:"created_at"`
	// Owner — 归属者（派单方/会话 id，用于回收与审计）。
	Owner string `json:"owner"`
}

// Now — 仓内口径的墙钟时间：UTC 且剥掉单调钟读数（Round(0)）。
// 目的：JSON 往返后逐位相等——带单调钟的 time.Time 往返后 `==` 会假红。
func Now() time.Time { return time.Now().UTC().Round(0) }

// New — 造一个 phase=created 的新 meta（CreatedAt=Now()）。
// 落盘顺序见本文件抬头 ②：先 Save 它，再建茧目录。
func New(id, egg, owner string) *Meta {
	return &Meta{
		ID:        id,
		Phase:     PhaseCreated,
		Egg:       egg,
		Owner:     owner,
		CreatedAt: Now(),
	}
}

// MetaPath — 茧目录 → meta 真源路径。
func MetaPath(dir string) string { return filepath.Join(dir, MetaFileName) }

// Validate — 结构自检（不碰盘、不判业务）。Save 前自动调用。
func (m *Meta) Validate() error {
	if m == nil {
		return fmt.Errorf("%w: meta 为 nil", ErrInvalidMeta)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.validateLocked()
}

func (m *Meta) validateLocked() error {
	if m.ID == "" {
		return fmt.Errorf("%w: id 为空", ErrInvalidMeta)
	}
	if !m.Phase.Valid() {
		return fmt.Errorf("%w: phase %q 不在链上（合法值 %v）", ErrInvalidMeta, string(m.Phase), Phases())
	}
	if m.PID < 0 {
		return fmt.Errorf("%w: pid %d 为负", ErrInvalidMeta, m.PID)
	}
	if m.Epoch < 0 {
		return fmt.Errorf("%w: epoch %d 为负", ErrInvalidMeta, m.Epoch)
	}
	for _, pid := range m.BackendPIDs {
		if pid <= 0 {
			return fmt.Errorf("%w: backend_pids 里出现非正 pid（%d）", ErrInvalidMeta, pid)
		}
	}
	for inner, host := range m.Ports {
		if inner <= 0 || inner > 65535 {
			return fmt.Errorf("%w: 茧内端口 %d 越界（1..65535）", ErrInvalidMeta, inner)
		}
		if host < 0 || host > 65535 {
			return fmt.Errorf("%w: 宿主端口 %d 越界（0..65535）", ErrInvalidMeta, host)
		}
	}
	return nil
}

// SetPhase — 只改**内存态**：校验单调后写入 m.Phase。
//
// 规则：同 phase 幂等（nil）；向后一格或多格前进都行；倒退一律 ErrPhaseRegression；
// 未知 phase 一律 ErrUnknownPhase。空 phase（零值对象）视为"没有历史"，任意已知 phase 都算首写。
//
// 要落盘请用 UpdatePhase(path, next)（读-校验-写 在同一把锁内）；直接 Save 一个已 SetPhase
// 的对象也可以，但并发写者之间会互相覆盖（后写的赢）。
func (m *Meta) SetPhase(next Phase) error {
	if m == nil {
		return fmt.Errorf("%w: meta 为 nil", ErrInvalidMeta)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.setPhaseLocked(next)
}

func (m *Meta) setPhaseLocked(next Phase) error {
	nextRank, ok := phaseRank[next]
	if !ok {
		return fmt.Errorf("%w: %q（合法值 %v）", ErrUnknownPhase, string(next), Phases())
	}
	curRank, known := m.Phase.rank()
	if !known {
		if m.Phase == "" {
			m.Phase = next // 零值对象：首写
			return nil
		}
		return fmt.Errorf("%w: 当前 phase %q 不在链上，拒绝迁移", ErrInvalidMeta, string(m.Phase))
	}
	if nextRank < curRank {
		return fmt.Errorf("%w: %s → %s（只许前进；删到一半崩了也走 %s→%s）",
			ErrPhaseRegression, m.Phase, next, PhaseDeleting, PhaseGone)
	}
	m.Phase = next // 前进或原地（幂等）
	return nil
}

// Load — 读 path 的 meta（path 一般是 MetaPath(茧目录)）。
//
// 失败：ErrNotFound（目录/文件不在）/ ErrInvalidMeta（JSON 坏或字段非法）/ 底层 IO 错误。
// 持共享 flock 读：保证不会读到"另一个进程正在 rename 的中间态"（rename 本身原子，锁是为了
// 让同进程多 goroutine 与跨进程写者之间的可见顺序确定）。
func Load(path string) (*Meta, error) {
	var out *Meta
	err := withLock(path, false, func() error {
		m, err := loadLocked(path)
		if err != nil {
			return err
		}
		out = m
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func loadLocked(path string) (*Meta, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("%w: %s", ErrNotFound, path)
		}
		return nil, err
	}
	var m Meta
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("%w: 解析 %s 失败: %v", ErrInvalidMeta, path, err)
	}
	if err := m.validateLocked(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &m, nil
}

// Save — 原子写 path（父目录不存在则建；那是**登记目录**，不是茧的工作目录——工作目录按抬头 ② 晚建）。
func Save(path string, m *Meta) error {
	return withLock(path, true, func() error { return saveLocked(path, m) })
}

func saveLocked(path string, m *Meta) error {
	if m == nil {
		return fmt.Errorf("%w: meta 为 nil", ErrInvalidMeta)
	}
	// 取快照 = 校验 + 序列化（都在 m.mu 内，很快）；fsync 不放锁内，别让 IO 卡住别的读。
	m.mu.Lock()
	err := m.validateLocked()
	var data []byte
	if err == nil {
		data, err = json.MarshalIndent(m, "", "  ")
	}
	m.mu.Unlock()
	if err != nil {
		return err
	}
	data = append(data, '\n')

	dir := filepath.Dir(path)
	tmp, err := writeTmp(dir, data)
	if err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("cocoon: rename %s → %s 失败: %w", tmp, path, err)
	}
	return syncDir(dir)
}

// writeTmp — 半截件的唯一制造点：写 tmp + fsync(file)，**不 rename**。
// 崩在这之后、rename 之前 = 盘上多一个 `.meta-*.tmp`，meta.json 仍是旧的（读者看不到脏数据）。
// 单测用它模拟"写一半被杀"。
func writeTmp(dir string, data []byte) (string, error) {
	f, err := os.CreateTemp(dir, tmpPrefix+"*"+tmpSuffix)
	if err != nil {
		return "", err
	}
	name := f.Name()
	if err := f.Chmod(filePerm); err != nil {
		_ = f.Close()
		_ = os.Remove(name)
		return "", err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(name)
		return "", err
	}
	if err := f.Sync(); err != nil { // 内容先落盘，rename 才有意义
		_ = f.Close()
		_ = os.Remove(name)
		return "", err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(name)
		return "", err
	}
	return name, nil
}

// syncDir — 目录 fsync：让 rename 本身落盘（不然掉电后可能回退到旧文件名映射）。
// 少数文件系统不支持目录 fsync（EINVAL/ENOTSUP）——那不算失败（内容已 rename 到位）；其余错误照实上报。
func syncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	if err := d.Sync(); err != nil {
		if errors.Is(err, unix.EINVAL) || errors.Is(err, unix.ENOTSUP) {
			return nil
		}
		return err
	}
	return nil
}

// UpdatePhase — 读-校验-写 在同一把锁（进程内锁 + 独占 flock）内完成。
//
// 这是 A5 里"先标 deleting"唯一该走的入口：并发写者不会互相覆盖，
// 也不会出现"两个进程各自读到 running、各自写回不同结论"。
// 失败不改盘（SetPhase 报错时直接返回，meta.json 原样）。
func UpdatePhase(path string, next Phase) (*Meta, error) {
	var out *Meta
	err := withLock(path, true, func() error {
		m, err := loadLocked(path)
		if err != nil {
			return err
		}
		if err := m.SetPhase(next); err != nil {
			return err
		}
		if err := saveLocked(path, m); err != nil {
			return err
		}
		out = m
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ResetForRetry — 唯一合法的倒退：failed → starting，并把 Epoch 加一。
//
// 为什么单开一个入口，而不是放宽 SetPhase：SetPhase 的单调护栏是给"对账/回收"用的
// （A2/A5 只能把茧往前推，不许把结论改回去）；重试是**主动开新一次尝试**，它推翻"失败"
// 这个结论——所以不许任何调用方顺手做到，必须显式走这里。
//
// 规则：
//
//	仅当盘上 phase == failed 才放行；其余（created/starting/running/stopping/deleting/gone）
//	一律 ErrNotRetryable。running 想重试得先被判定落 failed（先判死、再重来），
//	这样"为什么重试"在盘上始终有据可查。
//	放行时写回 starting、Epoch+1，读-校验-写 在同一把锁内；失败不改盘。
//
// 重试之所以要 epoch：同一份 meta 会经历多次尝试，旧的进程/端口/日志属于旧 epoch，
// 只有 epoch 能区分"这是第几次起"，不然 A3/A5 会把上一轮的残迹算到这一轮头上。
//
// 有意不动 PID/Ports：它们是"上一次为什么失败"的证据，重试方（A3）在起新进程时整份覆盖；
// 这里抹掉只会让排障少一条线索。
func ResetForRetry(path string) error {
	return withLock(path, true, func() error {
		m, err := loadLocked(path)
		if err != nil {
			return err
		}
		m.mu.Lock()
		if m.Phase != PhaseFailed {
			cur := m.Phase
			m.mu.Unlock()
			return fmt.Errorf("%w: 当前 phase=%s（重试只从 failed 起）", ErrNotRetryable, cur)
		}
		m.Phase = PhaseStarting // 这一处**有意**绕开 SetPhase：唯一合法的倒退
		m.Epoch++
		m.mu.Unlock()
		return saveLocked(path, m)
	})
}

// WriteTombstone — 墓碑写端：把 m 原子写到 root/<id>.meta.json（平铺面）。
//
// 平铺面为什么必须能被写：meta 与目录同址 ⇒ 目录一删，证据跟着没了；"有 meta 无目录"
// 这一类就没有任何可判面。删目录前先把 meta 搬成墓碑（本函数），盘上才留下
// "这个茧曾经存在、到哪一步了"的唯一凭据——对账（Reconcile）和清扫（A5）都靠它。
//
// 写法与 Save 同一套：同目录 tmp → fsync(file) → rename → fsync(dir)，
// 读者只可能看到旧墓碑或新墓碑，看不到半截（不会出现"墓碑写着 running 的残句"）。
// 锁件是 `<id>.meta.json.lock`，与 meta 的锁件互不相干——两份文件本来就该各自独立。
func WriteTombstone(root string, m *Meta) error {
	if m == nil {
		return fmt.Errorf("%w: meta 为 nil", ErrInvalidMeta)
	}
	if m.ID == "" {
		return fmt.Errorf("%w: id 为空（墓碑文件名靠它）", ErrInvalidMeta)
	}
	// 墓碑路径由 id 拼出来 ⇒ id 里带路径分隔符就能写到 root 外面去（不许）。
	if m.ID == "." || m.ID == ".." || strings.ContainsAny(m.ID, `/\`) {
		return fmt.Errorf("%w: id %q 含路径分隔符/为 . 或 ..", ErrInvalidMeta, m.ID)
	}
	path := FlatMetaPath(root, m.ID)
	return withLock(path, true, func() error { return saveLocked(path, m) })
}

// ReadTombstone — 墓碑读端：读 root/<id>.meta.json。
// 错误口径与 Load 完全一致（ErrNotFound / ErrInvalidMeta / 底层 IO），不另立一套。
func ReadTombstone(root, id string) (*Meta, error) { return Load(FlatMetaPath(root, id)) }

// withLock — 两级锁的取得顺序（唯一顺序，防死锁）：进程内按路径的互斥 → 跨进程 flock。
func withLock(path string, exclusive bool, fn func() error) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	unlockMem := lockInProcess(abs)
	defer unlockMem()

	dir := filepath.Dir(abs)
	if exclusive {
		if err := os.MkdirAll(dir, dirPerm); err != nil {
			return err
		}
	}
	unlockFile, err := flockFile(abs, exclusive)
	if err != nil {
		return err
	}
	defer unlockFile()

	return fn()
}

// flockFile — 跨进程锁：锁件 `<path>.lock`（常驻不删——删了会和后来的持有者产生竞态）。
// 读用共享锁、写用独占锁。
func flockFile(path string, exclusive bool) (func(), error) {
	lockPath := path + lockSuffix
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, filePerm)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("%w: %s（目录不存在，锁件建不出来）", ErrNotFound, path)
		}
		return nil, err
	}
	how := unix.LOCK_SH
	if exclusive {
		how = unix.LOCK_EX
	}
	if err := unix.Flock(int(f.Fd()), how); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("cocoon: flock %s 失败: %w", lockPath, err)
	}
	return func() {
		_ = unix.Flock(int(f.Fd()), unix.LOCK_UN)
		_ = f.Close()
	}, nil
}

// pathLock — 进程内按路径互斥 + 引用计数（用完即回收，长期跑不涨表）。
type pathLock struct {
	mu   sync.Mutex
	refs int
}

var (
	locksMu sync.Mutex
	locks   = map[string]*pathLock{}
)

// lockInProcess — 取 path 的进程内锁，返回释放函数。
// 引用计数含"正在等锁"的 goroutine ⇒ 最后一个人走时才回收，不会把别人手里的锁从表里删掉。
func lockInProcess(key string) func() {
	locksMu.Lock()
	pl, ok := locks[key]
	if !ok {
		pl = &pathLock{}
		locks[key] = pl
	}
	pl.refs++
	locksMu.Unlock()

	pl.mu.Lock()
	return func() {
		pl.mu.Unlock()
		locksMu.Lock()
		pl.refs--
		if pl.refs == 0 {
			delete(locks, key)
		}
		locksMu.Unlock()
	}
}
