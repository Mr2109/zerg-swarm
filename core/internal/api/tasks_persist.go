package api

// tasks_persist.go — 任务状态持久化（v2.5.5 P1-5 补 + 2026-08-20 完整版）
// 2026-09-18 修（实调发现缺陷）: 原写死 /tmp/zerg-tasks.json —— 多实例共享同一份文件、
// 互相覆盖任务历史（沙箱金丝雀实测把它回写了）。改为 statepath 统一状态目录派生
// （ZERG_STATE_DIR → ~/.zerg/state/zerg-tasks.json）。
// 迁移兼容（首次）: 新路径不存在而旧 /tmp/zerg-tasks.json 存在 ⇒ **读旧一次**（不丢历史）；
// **写只写新路径**；旧文件**不删、不改**。新路径已存在 ⇒ 旧路径完全不看。
// 启动 → 加载回 history + 恢复排队任务（重启不丢——AutoCUT 反复丢问题治本）
// 2026-10-03 GAP-20261003-04 刀1（T-07）: 恢复四分区（History/Queue/Running/Waiting）与两处 fail-closed
// 白名单判据正确对待新终态 cancelled 与正交标记 closed_*（设计 §2.5 / §4.3 D）——可恢复集 ∩ 终态集 == 空集;
// 非白名单项不再静默丢弃 ⇒ 保留进 history（未知态降级: 不 panic、不丢件、不复活）。
// 2026-10-21 任务状态机修——三条落盘不变式（由 normalizeTaskTime 与分区去重强制）:
//   I1 时间与状态自洽: 非终态 ⟹ completed_at 为零值（终态时刻只由 applyStatus 写）；
//   I2 created_at 只补零值、**永不覆写**（原「重启时间作为新创建」把创建时间改成本次重启时刻）；
//   I3 同一 ID 只落一个分区: queue/running/waiting 与 history **互不相交**（原同在途又留档 ⇒ 双份）。

import (
	"container/heap"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
)

// tasksStateFileName 任务持久化文件在统一状态目录下的文件名。
const tasksStateFileName = "zerg-tasks.json"

// tasksLegacyDefaultPath 旧硬编码落点字面量（v2.5.5 起写死 —— 多实例共享同一份、互相覆盖历史）。
// 只作**首次迁移读取**来源：读一次；不删、不改、永不写入。
const tasksLegacyDefaultPath = "/tmp/zerg-tasks.json"

// legacyTasksFile 旧路径（包级变量 = 上面的字面量；迁移用例/测试进程隔离可切换）。
var legacyTasksFile = tasksLegacyDefaultPath

// tasksFile 任务持久化文件（包级变量——测试可切换隔离路径——2026-08-28）
// 语义（2026-09-18）: 非空 ⇒ 直接用该路径（测试隔离/显式覆盖）；
// 空 ⇒ 走 statepath 统一状态目录派生（生产默认，见 tasksWritePath）。
var tasksFile = ""

// tasksWritePath 写路径：永远是统一状态目录（ZERG_STATE_DIR → ~/.zerg/state/<tasksStateFileName>）。
// 永不写旧 /tmp 路径——多实例各写各的（不再互相覆盖历史）。
func tasksWritePath() string {
	if p := strings.TrimSpace(tasksFile); p != "" {
		return p
	}
	return statepath.File(tasksStateFileName)
}

// tasksReadPath 读路径：统一状态目录优先；新路径不存在且旧 /tmp 存在 ⇒ 读旧一次（迁移兼容）。
// 新路径存在 ⇒ 旧路径完全不看（不 stat、不读——旧内容不夹除）。
// 显式覆盖 tasksFile（测试隔离）时不退旧路径：覆盖即「我已指定唯一来源」，避免测试读真机 /tmp。
func tasksReadPath() string {
	p := tasksWritePath()
	if _, err := os.Stat(p); err == nil {
		return p // 新路径已存在——旧路径完全不看
	}
	if strings.TrimSpace(tasksFile) != "" {
		return p
	}
	legacy := strings.TrimSpace(legacyTasksFile)
	if legacy == "" {
		return p
	}
	if _, err := os.Stat(legacy); err != nil {
		return p // 无旧文件——首次运行（空历史）
	}
	log.Printf("📜 任务持久化首次迁移: 读旧 %s（只读一次——旧文件保留不删；写入只落 %s）\n", legacy, p)
	return legacy
}

// persistedTasks 落盘结构（完整版——history + queue + running + waiting）
//
// ★ 分区不变式（2026-10-21 任务状态机修 · I3）: 四个分区是**同一 ID 空间的一个划分**——
// 同一个任务 ID 在一次落盘里**只出现在一个分区**（在途: queue/running/waiting；留档: history）。
// 原先 history 与 queue 各存一份（同一指针、内容相同）⇒ 252 原始条 / 175 去重 / 77 件重复，
// 文件体积虚增近一倍，且「同一件两处读回」走两条不同恢复路径（queue 重新派发 vs history 原样保留）。
// 为什么保留 history 分区而不是删掉: 它是 UI 列表与「最近 100 件」上限的载体
// （ListTasksHandler / TaskDetailHandler 读它），删分区要连带改消费者；「只落一份」零改动达同一目的。
// 为什么保留四个分区而不是并成一个 map: 恢复动作按分区不同（queue 重派 / waiting 继续等 /
// running 标 failed / history 原样保留）——分区就是「怎么恢复」的判据，与 classifyTaskStatus 白名单一一对应（§2.5）。
type persistedTasks struct {
	SavedAt time.Time        `json:"saved_at"`
	History map[string]*Task `json:"history"`
	Queue   []*Task          `json:"queue"`   // 排队中任务（重启恢复重新派发）
	Running map[string]*Task `json:"running"` // 运行中任务（重启标 failed——无法恢复执行——但记录）
	Waiting map[string]*Task `json:"waiting"` // v2.5.6 故障自愈: 环境故障挂起任务（重启恢复——继续等）
}

// dupHistorySkipLogged 分区去重的告警只打一次（否则每次落盘刷一行）。
var dupHistorySkipLogged bool

// normalizeTaskTime 时间字段不变式（**唯一兜底口** · 2026-10-21 任务状态机修）。
// 只做「不让脏值出去」，不发明历史:
//   - 非终态（queued/paused/running/waiting_retry）⇒ CompletedAt **一律清零**；
//   - waiting_retry ⇒ WaitingSince 缺失时按序回退: 旧 CompletedAt → now（等待窗计时源）；
//   - 终态 ⇒ 不强补 CompletedAt（存量件缺就缺——补一个假时刻比缺更坏）；
//   - CreatedAt 为零的旧件补 now（**只补零值，绝不覆写**）；
//   - classUnknown 原样放过（未知态不猜、不动——T-07 不丢件）。
func normalizeTaskTime(t *Task, now time.Time) {
	if t == nil || classifyTaskStatus(t.Status) == classUnknown {
		return
	}
	if t.CreatedAt.IsZero() {
		t.CreatedAt = now
	}
	if t.IsTerminal() {
		t.WaitingSince = nil
		return
	}
	if t.Status == StatusWaitingRetry {
		if t.WaitingSince == nil {
			ws := now
			if !t.CompletedAt.IsZero() {
				ws = t.CompletedAt // 旧件兼容: 旧实现把挂起时刻写在 completed_at 上
			}
			t.WaitingSince = &ws
		}
		t.CompletedAt = time.Time{}
		return
	}
	t.CompletedAt = time.Time{}
	t.WaitingSince = nil
}

// saveTasksLocked 保存任务状态（调用方持锁）
// v2.5.6 故障自愈: 增加 waiting 参数（环境故障挂起任务——重启恢复继续等）
// GAP-20261003-13/-14 修复（2026-10-03）:
//
//	① 改为**返回 error** —— 原先 marshal/mkdir/write 三种失败全静默吞掉，调用方不知落盘成败，
//	   内存已改而盘上仍旧值（静默分叉，无从察觉）。
//	② 落盘改为**原子替换** —— 原先 os.WriteFile 直写目标路径：崩溃/断电会留下半文件或 0 字节；
//	   现改为「同目录临时件写全 → fsync 文件 → rename 覆盖 → fsync 目录」。
func saveTasksLocked(queue TaskQueue, running map[string]*Task, history map[string]*Task, waiting ...map[string]*Task) error {
	// 完整版（2026-08-20）: history + queue + running 都存——重启不丢任何任务
	data := persistedTasks{
		SavedAt: time.Now(),
		History: make(map[string]*Task, len(history)),
		Queue:   []*Task{},
		Running: make(map[string]*Task, len(running)),
		Waiting: make(map[string]*Task),
	}
	// ① 在途件先落（queue → running → waiting）——先定「谁算在途」，history 再据此避重
	// 注: queue 是**切片头按值传**，此处的 Pop 只作用于局部副本，调用方的 s.queue 全程未动。
	for queue.Len() > 0 {
		t := queue.Pop().(*Task)
		data.Queue = append(data.Queue, t)
	}
	for id, t := range running {
		data.Running[id] = t
	}
	if len(waiting) > 0 {
		for id, t := range waiting[0] {
			data.Waiting[id] = t
		}
	}
	// ② history 只收「不在途」的件——同一 ID 只落一份（I3）
	live := make(map[string]*Task, len(data.Queue)+len(data.Running)+len(data.Waiting))
	for _, t := range data.Queue {
		if t != nil {
			live[t.ID] = t
		}
	}
	for id, t := range data.Running {
		live[id] = t
	}
	for id, t := range data.Waiting {
		live[id] = t
	}
	skipped := 0
	for id, t := range history {
		live0, dup := live[id]
		if !dup {
			data.History[id] = t
			continue
		}
		skipped++ // 只落一份（在途侧）——恢复时从在途分区回队列
		// ★ 但丢掉的那一份里可能有**在途那份已经丢掉的信息**: 旧实现每次重启都把在途件的
		//   created_at 覆写成「现在」，而留档副本没人碰 ⇒ 留档那份才是真创建时间
		//   （真机 77 件重复**逐字段比对只差 created_at**，全部是留档侧更早）。
		//   重启只会把 created_at 往后推，故取较早者必是未被覆写的原值——不是发明数据。
		if t != nil && live0 != nil && !t.CreatedAt.IsZero() &&
			(live0.CreatedAt.IsZero() || t.CreatedAt.Before(live0.CreatedAt)) {
			live0.CreatedAt = t.CreatedAt
		}
	}
	if skipped > 0 && !dupHistorySkipLogged {
		dupHistorySkipLogged = true
		log.Printf("🧹 任务持久化: %d 件同时在途且留档 —— 按分区不变式只落一份（在途侧；本进程只报一次）\n", skipped)
	}
	// 时间字段兜底归一（与内存是**同一批指针** ⇒ 修完盘上，内存同步；见 normalizeTaskTime）
	now := data.SavedAt
	for _, t := range data.Queue {
		normalizeTaskTime(t, now)
	}
	for _, t := range data.Running {
		normalizeTaskTime(t, now)
	}
	for _, t := range data.Waiting {
		normalizeTaskTime(t, now)
	}
	for _, t := range data.History {
		normalizeTaskTime(t, now)
	}
	buf, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("任务持久化序列化失败: %w", err)
	}
	// 写只写新路径（统一状态目录）——旧 /tmp 路径永不写（2026-09-18）
	dst := tasksWritePath()
	dir := filepath.Dir(dst)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("任务持久化目录不可用（%s）: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".tasks-*.tmp")
	if err != nil {
		return fmt.Errorf("任务持久化临时件创建失败（%s）: %w", dir, err)
	}
	tmpName := tmp.Name()
	fail := func(step string, e error) error {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("任务持久化%s失败（%s）: %w", step, tmpName, e)
	}
	if _, err := tmp.Write(buf); err != nil {
		return fail("写入临时件", err)
	}
	if err := tmp.Chmod(0o644); err != nil {
		return fail("设置权限", err)
	}
	if err := tmp.Sync(); err != nil {
		return fail("fsync", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("任务持久化关闭临时件失败（%s）: %w", tmpName, err)
	}
	if err := os.Rename(tmpName, dst); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("任务持久化原子替换失败（%s → %s）: %w", tmpName, dst, err)
	}
	// 目录 fsync: 保证 rename 本身落盘（打不开目录不算失败——rename 已成功）
	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		_ = d.Close()
	}
	return nil
}

// ============ 恢复面态分类（2026-10-03 GAP-20261003-04 刀1 · T-07 / T-19）============
//
// 设计 §2.5（恢复是四段逻辑）· §4.3 D（不变式: 可恢复集 ∩ 终态集 == 空集 · 三集覆盖全枚举）。
// ★ 分类是恢复四分区的**唯一真源**: 分区决定「怎么恢复」，分类决定「该不该恢复、恢复去哪」。
// ★ 覆盖代码面七态: 可恢复集 = {queued, paused, waiting_retry}、终态集 = {done, failed, cancelled}、
//
//	过渡集 = {running}，三者两两不交且并为全集; 认不出的串一律 classUnknown（fail-safe——绝不当成 queued）。
type taskStatusClass int

const (
	classRecoverQueue taskStatusClass = iota // queued / paused —— 可恢复（重新入 queue）
	classRecoverWait                         // waiting_retry —— 可恢复（回 waiting 继续等）
	classTransitional                        // running —— 过渡态（重启无法续跑 ⇒ 标 failed 记录）
	classTerminal                            // done / failed / cancelled —— 终态（原样保留、绝不派发）
	classUnknown                             // 其余 —— 未知态（降级: 原样保留 + 告警、不 panic、不丢件）
)

// classifyTaskStatus 归类裸 status 串（fail-safe: 认不出 ⇒ classUnknown）。
func classifyTaskStatus(s string) taskStatusClass {
	switch s {
	case StatusQueued, StatusPaused:
		return classRecoverQueue
	case StatusWaitingRetry:
		return classRecoverWait
	case StatusRunning:
		return classTransitional
	case StatusDone, StatusFailed, StatusCancelled:
		return classTerminal
	default:
		return classUnknown
	}
}

// parkUnrecovered 把「不从本分区恢复」的持久化项**保留进 history**（T-07「未知态降级不 panic、不丢件」）。
// 用于 queue / waiting 分区的非白名单项与 running 分区项:
//   - 终态（done/failed/cancelled）: 原样保留——**绝不覆写 status**（正交 closed_* 随 *Task 一并带过）;
//   - 过渡态 running: 重启无法恢复执行 ⇒ 标 failed 记录（与 Running 分区同口径）;
//   - 未知态 / 落错分区的可恢复态: 原样保留 + 响亮告警（不 panic、不丢件、**不跨分区重路由**）。
//
// ★ 只落 history、**绝不入 queue / waiting** ⇒ 不变式「可恢复集 ∩ 终态集 == 空集」不破、终态不复活、未知态不丢件。
// ★ 与 history 分区同 id 去重（保先到者——避免同一件两处读回时互相覆盖）。
func (s *MasterScheduler) parkUnrecovered(t *Task, from string) {
	if t == nil {
		return
	}
	if _, exists := s.history[t.ID]; exists {
		return
	}
	switch classifyTaskStatus(t.Status) {
	case classTerminal:
		// 原样保留——不改 status（覆写会丢「到底成没成」/退场语义——§O-8 / T-05）。
	case classTransitional:
		t.FailReason = "主控重启——运行中任务中断（未完成）"
		t.applyStatus(StatusFailed, time.Now())
	case classRecoverQueue, classRecoverWait:
		log.Printf("⚠️ 任务持久化恢复: %s 分区项 %s 状态 %q 属可恢复集却落在非对应分区 —— 保守保留进 history（不派发、不跨分区重路由）\n", from, t.ID, t.Status)
	default: // classUnknown
		log.Printf("⚠️ 任务持久化恢复: %s 分区项 %s 状态 %q 非已知态 —— 原样保留进 history（不派发、不丢件）\n", from, t.ID, t.Status)
	}
	s.history[t.ID] = t
}

// loadPersistedHistory 加载历史任务 + 恢复排队（启动时调）
func (s *MasterScheduler) loadPersistedHistory() {
	// 读路径：新（统一状态目录）优先；新缺失 + 旧 /tmp 在 ⇒ 读旧一次（2026-09-18 迁移兼容）
	// GAP-20261003-14 修复（2026-10-03）: 读失败**不许静默** —— 首次运行（件不存在）是正常路径，
	// 其余（权限/IO）与**反序列化失败（半文件/截断）**一律响亮报出：原先 Unmarshal 失败静默 return
	// ⇒ 盘上件一旦损坏就是整份 history+queue 无声全丢，无从察觉（文件保留不覆盖，待查）。
	readPath := tasksReadPath()
	buf, err := os.ReadFile(readPath)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("⚠️ 任务持久化读盘失败（%s）: %v —— 本次不恢复任何任务（盘上件保留未动）\n", readPath, err)
		}
		return
	}
	var data persistedTasks
	if err := json.Unmarshal(buf, &data); err != nil {
		log.Printf("❌ 任务持久化文件损坏（%s · %d 字节）: %v —— 本次不恢复任何任务（件保留待查，未被覆盖）\n", readPath, len(buf), err)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, t := range data.History {
		if t == nil {
			continue // T-07: 降级不 panic（null 项跳过、不影响其它件）
		}
		if _, exists := s.history[id]; !exists {
			// v2.5.6 2026-08-28: 重启时运行中任务无法恢复执行——标 failed（防 UI 残留 running 误导）
			if t.Status == "running" {
				t.FailReason = "重启中断（运行中任务无法恢复——重新触发）"
				t.applyStatus(StatusFailed, time.Now())
			}
			// T-07（设计 §4.3 B）: 未知态降级——不当作 queued、不 panic、不丢件; 原样保留 + 响亮告警。
			if classifyTaskStatus(t.Status) == classUnknown {
				log.Printf("⚠️ 任务持久化恢复: history 分区项 %s 状态 %q 非已知态 —— 原样保留（不派发、不丢件）\n", id, t.Status)
			}
			s.history[id] = t
		}
	}
	// 恢复排队任务（重新入队——重启后继续执行——不停丢）
	// ★ 白名单 fail-closed（设计 §2.5 / §4.1「Queue 分区白名单【不动】」——**判据未放宽**）:
	//   只认 queued/paused ⇒ 终态 cancelled/done/failed 与未知态**天生不命中、绝不被重新派发**。
	//   T-07: 非白名单项不再**静默丢弃** ⇒ 保留进 history（不丢件、不复活）。
	restored := 0
	for _, t := range data.Queue {
		if t == nil {
			continue // T-07: 降级不 panic
		}
		if t.Status == "queued" || t.Status == "paused" {
			t.Status = "queued"
			// 2026-10-21 修: 原先此处「重启时间作为新创建（避免时长异常）」把 CreatedAt **覆写**成本次重启时刻
			// ⇒ 采样里出现 completed 比 created 早 24 天（存量 85 件）、UI 耗时算出负数。
			// 耗时的根因是 completed_at 被写进非终态（见 runTask），不在 created_at——创建时间首次写入后不可覆写。
			if t.CreatedAt.IsZero() {
				t.CreatedAt = time.Now() // 只补旧件零值——绝不覆写
			}
			heap.Push(&s.queue, t)
			restored++
			continue
		}
		s.parkUnrecovered(t, "queue")
	}
	// 运行中任务（重启前在跑——无法恢复执行——标 failed 记录——防悬空）
	// ★ 设计 §2.5 原为「无条件标 failed」; 此处经 parkUnrecovered 加**终态/未知态守卫**（T-05/§O-8）——
	//   cancelled/done/failed 若因任何路径余留在 Running 分区，**绝不覆写**（覆写会永久丢掉「到底成没成」、
	//   退场语义与 closed_*）; 过渡态 running 仍标 failed。正常态（分区内只有 running）下与「无条件」等价。
	for _, t := range data.Running {
		if t == nil {
			continue // T-07: 降级不 panic
		}
		s.parkUnrecovered(t, "running")
	}
	// v2.5.6 故障自愈: 恢复挂起任务（waiting_retry——重启后继续等环境恢复——不丢）
	// ★ 白名单 fail-closed（判据未放宽）: 只认 waiting_retry; 非白名单项保留进 history（T-07 不丢件）。
	waitingRestored := 0
	for id, t := range data.Waiting {
		if t == nil {
			continue // T-07: 降级不 panic
		}
		if t.Status == "waiting_retry" {
			s.waiting[id] = t
			waitingRestored++
			continue
		}
		s.parkUnrecovered(t, "waiting")
	}
	// 2026-10-21: 恢复面统一校正时间字段（非终态清 completed_at / 旧挂起件补 WaitingSince / 零 created_at 补齐）
	nowRestore := time.Now()
	for _, t := range s.queue {
		normalizeTaskTime(t, nowRestore)
	}
	for _, t := range s.running {
		normalizeTaskTime(t, nowRestore)
	}
	for _, t := range s.waiting {
		normalizeTaskTime(t, nowRestore)
	}
	for _, t := range s.history {
		normalizeTaskTime(t, nowRestore)
	}
	if len(data.History) > 0 || restored > 0 || waitingRestored > 0 {
		log.Printf("📜 恢复任务: 历史 %d + 排队 %d + 挂起 %d（持久化完整版——重启不丢）\n", len(data.History), restored, waitingRestored)
	}
	// 2026-09-13 并发修复: 恢复只入队——不在此处派发。
	// 构造期派发 = 实例还没交给调用方就已有 goroutine 在跑任务、改 queue/running/history
	// （数据竞争 + 无停止手段）；排队任务改由调用方显式 Start() 派发（生产: main.go 装配完成后）。
	// 恢复调度器（waiting 扫描）同样由 Start() 启动。
}
