// meta_test.go — 四格判据（+ 两格护栏）：
//
//	① TestSaveLoadRoundTrip          正常存取（字段往返 + 文件是完整 JSON + 无 tmp 残留）
//	② TestConcurrentWritesNeverTorn  100 写者 + 8 读者：每次 Load 都必须解析出完整 JSON
//	③ TestSetPhaseMonotonic          phase 倒退被拒（全链两两验）+ 盘上倒退不改文件
//	④ TestCrashBetweenTmpAndRename   写一半被杀：tmp 已 fsync、rename 未发生 ⇒ Load 只看得到旧内容
//	⑤ TestFlockHeldByOtherProcessBlocksWriter  跨进程 flock 真的挡住第二个写者（子进程持锁）
//	⑥ TestLoadMissingAndSaveCreatesParent / TestValidateRejects  ErrNotFound 与结构自检
//	⑦ TestRunningToFailedLegal       running → failed 是前进（内存态 + 盘上都要通）
//	⑧ TestFailedRetryNeedsReset     failed → starting 被拒；ResetForRetry 才放行且 epoch=1
//	⑨ TestTombstoneWriteRead        墓碑写端/读端一致（同目录原子写、不留 tmp）
package cocoon

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// ── 夹具 ──────────────────────────────────────────────────────────────

func ptrTime(t time.Time) *time.Time { return &t }

// mustSave / mustLoad — 测试里失败即停（错误信息带上下文）。
func mustSave(t *testing.T, path string, m *Meta) {
	t.Helper()
	if err := Save(path, m); err != nil {
		t.Fatalf("Save(%s) 失败: %v", path, err)
	}
}

func mustLoad(t *testing.T, path string) *Meta {
	t.Helper()
	m, err := Load(path)
	if err != nil {
		t.Fatalf("Load(%s) 失败: %v", path, err)
	}
	return m
}

// ── ① 正常存取 ────────────────────────────────────────────────────────

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := MetaPath(dir)

	start := time.Date(2026, 10, 20, 12, 0, 0, 123456789, time.UTC)
	m := New("c-0001", "qwen3-4b-q4", "A1/会话-1")
	m.PID = 4242
	m.StartTime = ptrTime(start)
	m.Ports = map[int]int{8080: 38080, 8081: 38081}
	if err := m.SetPhase(PhaseRunning); err != nil { // created → running：前进，允许
		t.Fatalf("SetPhase(running) 失败: %v", err)
	}
	mustSave(t, path, m)

	got := mustLoad(t, path)

	if got.ID != m.ID || got.Phase != PhaseRunning || got.PID != m.PID ||
		got.Egg != m.Egg || got.Owner != m.Owner {
		t.Fatalf("标量字段往返不一致:\n got=%+v\nwant=%+v", got, m)
	}
	if !got.CreatedAt.Equal(m.CreatedAt) || got.CreatedAt.Location() != time.UTC {
		t.Fatalf("created_at 往返不一致: got=%v want=%v", got.CreatedAt, m.CreatedAt)
	}
	if got.StartTime == nil || !got.StartTime.Equal(start) {
		t.Fatalf("start_time 往返不一致: got=%v want=%v", got.StartTime, start)
	}
	if len(got.Ports) != 2 || got.Ports[8080] != 38080 || got.Ports[8081] != 38081 {
		t.Fatalf("ports 往返不一致: %v", got.Ports)
	}

	// 盘上是完整 JSON（含结尾换行），且没有 tmp 残件。
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读回原文失败: %v", err)
	}
	if !json.Valid(raw) {
		t.Fatalf("盘上不是完整 JSON:\n%s", raw)
	}
	if len(raw) == 0 || raw[len(raw)-1] != '\n' {
		t.Fatalf("结尾应是换行（便于人读/diff）: %q", raw)
	}
	if left, _ := filepath.Glob(filepath.Join(dir, tmpPrefix+"*")); len(left) != 0 {
		t.Fatalf("成功后不该留 tmp: %v", left)
	}

	// 覆盖写：第二次 Save 要整份替换，不留旧字段（Ports 被清空则 JSON 里键该消失）。
	//
	// ★ 顺带抓"假原子写"：Save 之前打开的 fd 在 Save 之后必须**仍看到旧内容**——
	//   rename 只换目录项、不换 inode，所以老 fd 钉在旧 inode 上；原地写(truncate+write)
	//   会让这个 fd 看到新内容。这条判据不靠概率，确定性地区分"rename"和"原地写"。
	held, err := os.Open(path)
	if err != nil {
		t.Fatalf("打开旧 inode 失败: %v", err)
	}
	defer held.Close()

	got.Ports = nil
	got.PID = 0
	mustSave(t, path, got)

	heldRaw, err := io.ReadAll(held)
	if err != nil {
		t.Fatalf("读旧 fd 失败: %v", err)
	}
	if !bytes.Equal(heldRaw, raw) {
		t.Fatalf("Save 不是 rename 语义（Save 前打开的 fd 看到了新内容 = 原地写）:\nheld=%s\nwant=%s", heldRaw, raw)
	}
	newRaw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读新文件失败: %v", err)
	}
	if bytes.Equal(newRaw, raw) {
		t.Fatalf("Save 后真源内容没变:\n%s", newRaw)
	}

	again := mustLoad(t, path)
	if again.Ports != nil || again.PID != 0 {
		t.Fatalf("覆盖写没整份替换: ports=%v pid=%d", again.Ports, again.PID)
	}
}

// ── ② 100 并发写 + 读者：读到的永远是完整 JSON ─────────────────────────

func TestConcurrentWritesNeverTorn(t *testing.T) {
	dir := t.TempDir()
	path := MetaPath(dir)

	seed := New("c-0002", "egg-conc", "A1")
	seed.Ports = map[int]int{8080: 38080, 8081: 38081, 8082: 38082}
	mustSave(t, path, seed)

	const (
		writers        = 100
		readers        = 8
		readsPerReader = 40
	)
	errCh := make(chan error, writers+readers)
	var wg sync.WaitGroup

	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			m, err := Load(path) // 读-改-写：靠 flock + 进程内锁保证不互相覆盖
			if err != nil {
				errCh <- fmt.Errorf("写者 %d Load: %w", i, err)
				return
			}
			if err := m.SetPhase(PhaseRunning); err != nil {
				errCh <- fmt.Errorf("写者 %d SetPhase: %w", i, err)
				return
			}
			m.PID = 9000 + i
			m.Ports = map[int]int{8080: 38080 + i}
			if err := Save(path, m); err != nil {
				errCh <- fmt.Errorf("写者 %d Save: %w", i, err)
			}
		}(i)
	}

	for r := 0; r < readers; r++ {
		wg.Add(1)
		go func(r int) {
			defer wg.Done()
			for k := 0; k < readsPerReader; k++ {
				m, err := Load(path)
				if err != nil {
					// 半截文件会在这里炸（unexpected end of JSON input / 字段缺失）
					errCh <- fmt.Errorf("读者 %d 第 %d 次 Load: %w", r, k, err)
					return
				}
				if m.ID != seed.ID || m.Egg != seed.Egg || m.Owner != seed.Owner {
					errCh <- fmt.Errorf("读者 %d 第 %d 次读到不完整的 meta: %+v", r, k, m)
					return
				}
				if m.Phase != PhaseCreated && m.Phase != PhaseRunning {
					errCh <- fmt.Errorf("读者 %d 第 %d 次读到怪 phase: %q", r, k, m.Phase)
					return
				}
				if len(m.Ports) == 0 {
					errCh <- fmt.Errorf("读者 %d 第 %d 次读到空 ports", r, k)
					return
				}
			}
		}(r)
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Error(err)
	}
	if t.Failed() {
		return
	}

	// 收尾：最后一份也是完整 JSON，且 100 个写者的结论落在合法区间内（谁最后赢都行）。
	final := mustLoad(t, path)
	if final.PID < 9000 || final.PID > 9000+writers-1 {
		t.Fatalf("终态 PID 不像任何写者写的: %d", final.PID)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读回终态失败: %v", err)
	}
	if !json.Valid(raw) {
		t.Fatalf("终态不是完整 JSON:\n%s", raw)
	}
	if left, _ := filepath.Glob(filepath.Join(dir, tmpPrefix+"*")); len(left) != 0 {
		t.Fatalf("并发写后残留 tmp: %v", left)
	}
}

// ── ③ phase 单调：倒退被拒 ────────────────────────────────────────────

func TestSetPhaseMonotonic(t *testing.T) {
	t.Run("整链前进 + 幂等", func(t *testing.T) {
		m := New("c-0003", "egg-x", "A1")
		if m.Phase != PhaseCreated {
			t.Fatalf("New 该是 created，实为 %q", m.Phase)
		}
		for _, p := range Phases() {
			if err := m.SetPhase(p); err != nil {
				t.Fatalf("前进到 %s 被拒: %v", p, err)
			}
			if err := m.SetPhase(p); err != nil { // 同 phase 幂等
				t.Fatalf("%s 原地重设被拒: %v", p, err)
			}
			if m.Phase != p {
				t.Fatalf("phase 没写进: got=%q want=%q", m.Phase, p)
			}
		}
	})

	t.Run("任意倒退逐格被拒", func(t *testing.T) {
		seq := Phases()
		for i := range seq {
			for j := 0; j < i; j++ {
				m := &Meta{ID: "c-0003", Phase: seq[i]}
				err := m.SetPhase(seq[j])
				if !errors.Is(err, ErrPhaseRegression) {
					t.Fatalf("%s → %s 应被拒（ErrPhaseRegression），实得 %v", seq[i], seq[j], err)
				}
				if m.Phase != seq[i] {
					t.Fatalf("%s → %s 被拒后阶段被改坏了: %q", seq[i], seq[j], m.Phase)
				}
			}
		}
	})

	t.Run("未知 phase 被拒", func(t *testing.T) {
		m := New("c-0003", "egg-x", "A1")
		for _, bad := range []Phase{"", "hatched", "RUNNING", "running "} {
			if err := m.SetPhase(bad); !errors.Is(err, ErrUnknownPhase) {
				t.Fatalf("SetPhase(%q) 应报 ErrUnknownPhase，实得 %v", bad, err)
			}
			if m.Phase != PhaseCreated {
				t.Fatalf("被拒后阶段被改坏了: %q", m.Phase)
			}
		}
	})

	t.Run("零值对象首写放行", func(t *testing.T) {
		m := &Meta{ID: "c-0003"} // phase = ""
		if err := m.SetPhase(PhaseStarting); err != nil {
			t.Fatalf("空态首写被拒: %v", err)
		}
		if m.Phase != PhaseStarting {
			t.Fatalf("首写没写进: %q", m.Phase)
		}
	})

	t.Run("盘上倒退被拒且文件不动", func(t *testing.T) {
		dir := t.TempDir()
		path := MetaPath(dir)
		mustSave(t, path, New("c-0003", "egg-x", "A1"))
		if _, err := UpdatePhase(path, PhaseRunning); err != nil {
			t.Fatalf("UpdatePhase(running) 失败: %v", err)
		}
		before, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("读原文失败: %v", err)
		}

		if _, err := UpdatePhase(path, PhaseStarting); !errors.Is(err, ErrPhaseRegression) {
			t.Fatalf("盘上倒退应报 ErrPhaseRegression，实得 %v", err)
		}
		after, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("读原文失败: %v", err)
		}
		if !bytes.Equal(before, after) {
			t.Fatalf("被拒的 UpdatePhase 改动了 meta.json:\n before=%s\n after=%s", before, after)
		}
		if got := mustLoad(t, path); got.Phase != PhaseRunning {
			t.Fatalf("被拒后盘上 phase 变了: %q", got.Phase)
		}
	})
}

// ── ④ 写一半被杀：tmp 在、真源没动 ────────────────────────────────────

func TestCrashBetweenTmpAndRename(t *testing.T) {
	dir := t.TempDir()
	path := MetaPath(dir)

	old := New("c-0004", "egg-x", "A1")
	old.PID = 111
	mustSave(t, path, old)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读原文失败: %v", err)
	}

	// 新状态：pid=222、已 running，序列化好准备替换。
	next := New("c-0004", "egg-x", "A1")
	next.PID = 222
	next.StartTime = ptrTime(Now())
	if err := next.SetPhase(PhaseRunning); err != nil {
		t.Fatalf("SetPhase 失败: %v", err)
	}
	data, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	data = append(data, '\n')

	// 只做 Save 的前半段（写 tmp + fsync），**不 rename** —— 这就是被 SIGKILL 的瞬间。
	tmp, err := writeTmp(dir, data)
	if err != nil {
		t.Fatalf("writeTmp 失败: %v", err)
	}
	if _, err := os.Stat(tmp); err != nil {
		t.Fatalf("tmp 该在盘上: %v", err)
	}
	// 结构护栏：tmp 必须与真源同目录、且带 `.meta-*.tmp` 名（清扫器按此前缀回收）；
	// 一旦有人把 tmp 改成"就地写"或换目录，这条立刻红。
	if filepath.Dir(tmp) != dir {
		t.Fatalf("tmp 必须与真源同目录: tmp=%s dir=%s", tmp, dir)
	}
	if b := filepath.Base(tmp); !strings.HasPrefix(b, tmpPrefix) || !strings.HasSuffix(b, tmpSuffix) {
		t.Fatalf("tmp 命名不合约定（%s*%s）: %s", tmpPrefix, tmpSuffix, b)
	}
	if tmp == path {
		t.Fatalf("tmp 不许就是真源路径: %s", tmp)
	}

	// 判据一：读者看不见新内容，读到的是旧的、完整的。
	got := mustLoad(t, path)
	if got.PID != 111 {
		t.Fatalf("半截写入被读者看见了: pid=%d（应是 111）", got.PID)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读原文失败: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("真源被动了:\n before=%s\n after=%s", before, after)
	}

	// 判据二：tmp 自己是完整 JSON（清扫器/排障能认出它是谁）。
	tb, err := os.ReadFile(tmp)
	if err != nil {
		t.Fatalf("读 tmp 失败: %v", err)
	}
	if !json.Valid(tb) {
		t.Fatalf("tmp 应是完整 JSON: %s", tb)
	}
	var peek Meta
	if err := json.Unmarshal(tb, &peek); err != nil || peek.PID != 222 {
		t.Fatalf("tmp 解析异常: err=%v pid=%d", err, peek.PID)
	}

	// 判据三（两态的分界）：rename 一发生，新内容立刻可见 —— 证明"半截可见"只可能来自
	// 非 rename 的写法，rename 本身没有中间态。
	if err := os.Rename(tmp, path); err != nil {
		t.Fatalf("rename 失败: %v", err)
	}
	if got := mustLoad(t, path); got.PID != 222 || got.Phase != PhaseRunning {
		t.Fatalf("rename 后应看见新内容: pid=%d phase=%q", got.PID, got.Phase)
	}

	// 判据四：残件不影响后续正常写。
	third := New("c-0004", "egg-x", "A1")
	third.PID = 333
	mustSave(t, path, third)
	if got := mustLoad(t, path); got.PID != 333 {
		t.Fatalf("后续 Save 没生效: pid=%d", got.PID)
	}
	if left, _ := filepath.Glob(filepath.Join(dir, tmpPrefix+"*")); len(left) != 0 {
		t.Fatalf("Save 应清掉自己的 tmp，残留: %v", left)
	}
}

// ── ⑤ 跨进程 flock：另一个进程持独占锁时，本进程写不进去 ──────────────

const flockHelperEnv = "ZERG_COCOON_FLOCK_HELPER_PATH"

// runFlockHelper — 子进程：拿独占 flock → 打印 "locked" → 睡到被杀。
func runFlockHelper(path string) {
	f, err := os.OpenFile(path+lockSuffix, os.O_CREATE|os.O_RDWR, filePerm)
	if err != nil {
		fmt.Fprintf(os.Stderr, "helper open: %v\n", err)
		os.Exit(3)
	}
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX); err != nil {
		fmt.Fprintf(os.Stderr, "helper flock: %v\n", err)
		os.Exit(4)
	}
	fmt.Println("locked")
	os.Stdout.Sync()
	time.Sleep(30 * time.Second)
	os.Exit(0)
}

func TestFlockHeldByOtherProcessBlocksWriter(t *testing.T) {
	if p := os.Getenv(flockHelperEnv); p != "" { // 子进程分支
		runFlockHelper(p)
		return
	}
	if testing.Short() {
		t.Skip("短跑跳过：要起子进程")
	}

	dir := t.TempDir()
	path := MetaPath(dir)
	mustSave(t, path, New("c-0005", "egg-x", "A1"))

	cmd := exec.Command(os.Args[0], "-test.run=^TestFlockHeldByOtherProcessBlocksWriter$")
	cmd.Env = append(os.Environ(), flockHelperEnv+"="+path)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("取 stdout 管道失败: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("起子进程失败: %v", err)
	}
	killed := false
	stop := func() {
		if killed {
			return
		}
		killed = true
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
	}
	defer stop()

	// 等子进程把锁拿到手（最多 10s）——子进程若提前退了，ReadString 返回 EOF，别挂死。
	type readRes struct {
		line string
		err  error
	}
	resCh := make(chan readRes, 1)
	go func() {
		line, err := bufio.NewReader(stdout).ReadString('\n')
		resCh <- readRes{line, err}
	}()
	select {
	case r := <-resCh:
		if r.err != nil || r.line != "locked\n" {
			t.Fatalf("子进程没报 locked: line=%q err=%v", r.line, r.err)
		}
	case <-time.After(10 * time.Second):
		t.Fatalf("等子进程加锁超时")
	}

	// 本进程以非阻塞独占方式抢同一把锁 —— 必须被拒（= 跨进程互斥真的生效）。
	f, err := os.OpenFile(path+lockSuffix, os.O_CREATE|os.O_RDWR, filePerm)
	if err != nil {
		t.Fatalf("开锁件失败: %v", err)
	}
	defer f.Close()
	err = unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
	if !errors.Is(err, unix.EWOULDBLOCK) && !errors.Is(err, unix.EAGAIN) {
		t.Fatalf("另一个进程持锁时本进程竟抢到锁: err=%v", err)
	}

	// 子进程一死，锁自动释放 → 本进程能拿；Save 也就通得过。
	name := cmd.Process.Pid
	stop()
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		t.Fatalf("持锁子进程(pid=%d)已退出，锁该释放，实得 %v", name, err)
	}
	_ = unix.Flock(int(f.Fd()), unix.LOCK_UN)

	m := mustLoad(t, path)
	m.PID = 555
	mustSave(t, path, m)
	if got := mustLoad(t, path); got.PID != 555 {
		t.Fatalf("锁释放后 Save 没生效: pid=%d", got.PID)
	}
}

// ── ⑥ 护栏：不存在 / 结构自检 ─────────────────────────────────────────

func TestLoadMissingAndSaveCreatesParent(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "nope", MetaFileName)
	if _, err := Load(missing); !errors.Is(err, ErrNotFound) {
		t.Fatalf("目录不存在应报 ErrNotFound，实得 %v", err)
	}

	// Save 会建**登记目录**（深两层的父目录也建）。
	deep := filepath.Join(dir, "a", "b", MetaFileName)
	mustSave(t, deep, New("c-0006", "egg-x", "A1"))
	if got := mustLoad(t, deep); got.ID != "c-0006" {
		t.Fatalf("深目录存取失败: %+v", got)
	}

	// 文件在、内容是垃圾 ⇒ ErrInvalidMeta（不是"假装不存在"）。
	if err := os.WriteFile(deep, []byte("{不是 json"), filePerm); err != nil {
		t.Fatalf("写垃圾失败: %v", err)
	}
	if _, err := Load(deep); !errors.Is(err, ErrInvalidMeta) {
		t.Fatalf("坏 JSON 应报 ErrInvalidMeta，实得 %v", err)
	}

	// 文件在但字段非法（phase 不在链上）⇒ ErrInvalidMeta。
	if err := os.WriteFile(deep, []byte(`{"id":"x","phase":"zombie"}`), filePerm); err != nil {
		t.Fatalf("写非法字段失败: %v", err)
	}
	if _, err := Load(deep); !errors.Is(err, ErrInvalidMeta) {
		t.Fatalf("未知 phase 应报 ErrInvalidMeta，实得 %v", err)
	}
}

func TestValidateRejects(t *testing.T) {
	base := func() *Meta { return New("c-0007", "egg-x", "A1") }

	cases := []struct {
		name string
		mut  func(*Meta)
	}{
		{"空 id", func(m *Meta) { m.ID = "" }},
		{"未知 phase", func(m *Meta) { m.Phase = "zombie" }},
		{"负 pid", func(m *Meta) { m.PID = -1 }},
		{"茧内端口 0", func(m *Meta) { m.Ports = map[int]int{0: 38080} }},
		{"茧内端口越界", func(m *Meta) { m.Ports = map[int]int{70000: 38080} }},
		{"宿主端口越界", func(m *Meta) { m.Ports = map[int]int{8080: 70000} }},
	}
	for _, c := range cases {
		m := base()
		c.mut(m)
		if err := m.Validate(); !errors.Is(err, ErrInvalidMeta) {
			t.Fatalf("%s: 应报 ErrInvalidMeta，实得 %v", c.name, err)
		}
		// 非法 meta 不许落盘（盘上留半份"看着像真的"的 meta 比报错更坏）。
		dir := t.TempDir()
		path := MetaPath(dir)
		if err := Save(path, m); !errors.Is(err, ErrInvalidMeta) {
			t.Fatalf("%s: Save 应报 ErrInvalidMeta，实得 %v", c.name, err)
		}
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("%s: 非法 meta 不该落盘，stat err=%v", c.name, err)
		}
	}

	if err := base().Validate(); err != nil {
		t.Fatalf("合法 meta 被误判: %v", err)
	}
	if err := Save(MetaPath(t.TempDir()), nil); !errors.Is(err, ErrInvalidMeta) {
		t.Fatalf("nil meta 应报 ErrInvalidMeta，实得 %v", err)
	}
}

// ── ⑦ running → failed 是前进 ─────────────────────────────────────────

func TestRunningToFailedLegal(t *testing.T) {
	// 链上必须真有 failed，且排在 running 之后（否则"失败降级"要么未知、要么被判倒退）。
	rankRunning, okR := PhaseRunning.rank()
	rankFailed, okF := PhaseFailed.rank()
	if !okR || !okF {
		t.Fatalf("running/failed 必须都在链上: running=%v failed=%v", okR, okF)
	}
	if rankFailed <= rankRunning {
		t.Fatalf("failed 必须排在 running 之后: running=%d failed=%d", rankFailed, rankRunning)
	}

	t.Run("内存态", func(t *testing.T) {
		m := New("c-0008", "egg-x", "A1")
		for _, p := range []Phase{PhaseStarting, PhaseRunning, PhaseFailed} {
			if err := m.SetPhase(p); err != nil {
				t.Fatalf("created → … → %s 被拒: %v", p, err)
			}
		}
		if m.Phase != PhaseFailed {
			t.Fatalf("phase 没落到 failed: %q", m.Phase)
		}
		// failed 之后照常往前走（失败的茧照样要被回收）。
		for _, p := range []Phase{PhaseStopping, PhaseDeleting, PhaseGone} {
			if err := m.SetPhase(p); err != nil {
				t.Fatalf("failed → %s 被拒（回收链断了）: %v", p, err)
			}
		}
	})

	t.Run("盘上", func(t *testing.T) {
		dir := t.TempDir()
		path := MetaPath(dir)
		m := New("c-0008", "egg-x", "A1")
		m.PID = 4242
		m.Ports = map[int]int{8080: 38080}
		if err := m.SetPhase(PhaseRunning); err != nil {
			t.Fatalf("SetPhase(running) 失败: %v", err)
		}
		mustSave(t, path, m)

		got, err := UpdatePhase(path, PhaseFailed)
		if err != nil {
			t.Fatalf("running → failed 应放行，实得 %v", err)
		}
		if got.Phase != PhaseFailed {
			t.Fatalf("返回值 phase = %q，want failed", got.Phase)
		}
		if on := mustLoad(t, path); on.Phase != PhaseFailed {
			t.Fatalf("盘上 phase = %q，want failed", on.Phase)
		}
		// 这一档不碰 epoch：只有重试才动它。
		if on := mustLoad(t, path); on.Epoch != 0 {
			t.Fatalf("running→failed 不该改 epoch，got %d", on.Epoch)
		}
	})
}

// ── ⑧ failed → starting 只能走 ResetForRetry（并 epoch+1） ─────────────

func TestFailedRetryNeedsReset(t *testing.T) {
	// mkFailed — 盘上造一份 phase=failed 的 meta（epoch 从 0 起）。
	mkFailed := func(t *testing.T) (dir, path string) {
		t.Helper()
		dir = t.TempDir()
		path = MetaPath(dir)
		m := New("c-0009", "egg-x", "A1")
		if err := m.SetPhase(PhaseRunning); err != nil {
			t.Fatalf("SetPhase(running) 失败: %v", err)
		}
		mustSave(t, path, m)
		if _, err := UpdatePhase(path, PhaseFailed); err != nil {
			t.Fatalf("UpdatePhase(failed) 失败: %v", err)
		}
		return dir, path
	}

	t.Run("failed→starting 被拒且不改盘", func(t *testing.T) {
		_, path := mkFailed(t)

		// 内存态：SetPhase 也认这是倒退。
		m := mustLoad(t, path)
		if err := m.SetPhase(PhaseStarting); !errors.Is(err, ErrPhaseRegression) {
			t.Fatalf("failed → starting 应报 ErrPhaseRegression，实得 %v", err)
		}
		if m.Phase != PhaseFailed {
			t.Fatalf("被拒后内存态被改坏了: %q", m.Phase)
		}

		before, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("读原文失败: %v", err)
		}
		if _, err := UpdatePhase(path, PhaseStarting); !errors.Is(err, ErrPhaseRegression) {
			t.Fatalf("盘上 failed → starting 应报 ErrPhaseRegression，实得 %v", err)
		}
		after, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("读原文失败: %v", err)
		}
		if !bytes.Equal(before, after) {
			t.Fatalf("被拒的 UpdatePhase 改动了 meta.json:\n before=%s\n after=%s", before, after)
		}
	})

	t.Run("ResetForRetry 放行且 epoch=1", func(t *testing.T) {
		_, path := mkFailed(t)

		if err := ResetForRetry(path); err != nil {
			t.Fatalf("failed 上 ResetForRetry 应放行，实得 %v", err)
		}
		got := mustLoad(t, path)
		if got.Phase != PhaseStarting {
			t.Fatalf("重试后盘上 phase = %q，want starting", got.Phase)
		}
		if got.Epoch != 1 {
			t.Fatalf("重试后 epoch = %d，want 1", got.Epoch)
		}

		// 再试一次：此刻是 starting（不是 failed）⇒ 拒，且 epoch 不许涨。
		if err := ResetForRetry(path); !errors.Is(err, ErrNotRetryable) {
			t.Fatalf("starting 上 ResetForRetry 应报 ErrNotRetryable，实得 %v", err)
		}
		if again := mustLoad(t, path); again.Epoch != 1 || again.Phase != PhaseStarting {
			t.Fatalf("被拒的重试改动了盘: phase=%q epoch=%d", again.Phase, again.Epoch)
		}

		// 走完一轮再失败：epoch 累加到 2（"第几次起"必须可判）。
		if _, err := UpdatePhase(path, PhaseRunning); err != nil {
			t.Fatalf("starting → running 失败: %v", err)
		}
		if _, err := UpdatePhase(path, PhaseFailed); err != nil {
			t.Fatalf("running → failed 失败: %v", err)
		}
		if err := ResetForRetry(path); err != nil {
			t.Fatalf("第二轮 ResetForRetry 失败: %v", err)
		}
		if got := mustLoad(t, path); got.Epoch != 2 || got.Phase != PhaseStarting {
			t.Fatalf("第二轮重试后 phase=%q epoch=%d，want starting/2", got.Phase, got.Epoch)
		}
	})

	t.Run("非 failed 一律拒", func(t *testing.T) {
		for _, p := range Phases() {
			if p == PhaseFailed {
				continue
			}
			dir := t.TempDir()
			path := MetaPath(dir)
			m := New("c-0009", "egg-x", "A1")
			if err := m.SetPhase(p); err != nil {
				t.Fatalf("SetPhase(%s) 失败: %v", p, err)
			}
			mustSave(t, path, m)
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("读原文失败: %v", err)
			}

			if err := ResetForRetry(path); !errors.Is(err, ErrNotRetryable) {
				t.Fatalf("%s 上 ResetForRetry 应报 ErrNotRetryable，实得 %v", p, err)
			}
			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("读原文失败: %v", err)
			}
			if !bytes.Equal(before, after) {
				t.Fatalf("%s 上被拒的 ResetForRetry 改动了盘", p)
			}
		}

		// meta 不存在 ⇒ ErrNotFound（不是"拒绝了"，是根本没这份账）。
		if err := ResetForRetry(MetaPath(t.TempDir())); !errors.Is(err, ErrNotFound) {
			t.Fatalf("meta 不存在应报 ErrNotFound，实得 %v", err)
		}
	})
}

// ── ⑨ 墓碑：写端 / 读端一致 ───────────────────────────────────────────

func TestTombstoneWriteRead(t *testing.T) {
	root := t.TempDir()

	m := New("c-tomb", "egg-tomb", "A2/测试")
	m.PID = 4242
	m.StartTime = ptrTime(time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC))
	m.Ports = map[int]int{8080: 38080, 8081: 38081}
	m.Epoch = 2
	for _, p := range []Phase{PhaseStarting, PhaseRunning, PhaseFailed} {
		if err := m.SetPhase(p); err != nil {
			t.Fatalf("SetPhase(%s) 失败: %v", p, err)
		}
	}
	if err := WriteTombstone(root, m); err != nil {
		t.Fatalf("WriteTombstone 失败: %v", err)
	}

	// 落点必须是平铺面 `<root>/<id>.meta.json`，且**不许**顺手造出茧目录/meta.json
	// （不然墓碑就成了假目录，对账会把它判成"一致"）。
	want := filepath.Join(root, "c-tomb.meta.json")
	if got := FlatMetaPath(root, "c-tomb"); got != want {
		t.Fatalf("墓碑路径 = %s，want %s", got, want)
	}
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("墓碑没写出来: %v", err)
	}
	if _, err := os.Stat(MetaPath(filepath.Join(root, "c-tomb"))); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("写墓碑不该造茧目录/meta.json（err=%v）", err)
	}

	// 盘上是完整 JSON、结尾换行、没留 tmp（与 Save 同一套原子写）。
	raw, err := os.ReadFile(want)
	if err != nil {
		t.Fatalf("读墓碑失败: %v", err)
	}
	if !json.Valid(raw) {
		t.Fatalf("墓碑不是完整 JSON:\n%s", raw)
	}
	if len(raw) == 0 || raw[len(raw)-1] != '\n' {
		t.Fatalf("墓碑结尾应是换行: %q", raw)
	}
	if left, _ := filepath.Glob(filepath.Join(root, tmpPrefix+"*")); len(left) != 0 {
		t.Fatalf("写墓碑后残留 tmp: %v", left)
	}

	// 读回一致（逐字段；epoch 必须一起往返，否则"第几次起"会丢）。
	got, err := ReadTombstone(root, "c-tomb")
	if err != nil {
		t.Fatalf("ReadTombstone 失败: %v", err)
	}
	if got.ID != m.ID || got.Phase != PhaseFailed || got.Epoch != 2 || got.PID != m.PID ||
		got.Egg != m.Egg || got.Owner != m.Owner {
		t.Fatalf("墓碑字段不一致:\n got=%+v\nwant=%+v", got, m)
	}
	if !got.CreatedAt.Equal(m.CreatedAt) {
		t.Fatalf("created_at 往返不一致: got=%v want=%v", got.CreatedAt, m.CreatedAt)
	}
	if got.StartTime == nil || !got.StartTime.Equal(*m.StartTime) {
		t.Fatalf("start_time 往返不一致: got=%v want=%v", got.StartTime, m.StartTime)
	}
	if len(got.Ports) != 2 || got.Ports[8080] != 38080 || got.Ports[8081] != 38081 {
		t.Fatalf("ports 往返不一致: %v", got.Ports)
	}

	// 覆盖写：墓碑也能被推进（gone），且仍是 rename 语义（老 fd 看不到新内容）。
	held, err := os.Open(want)
	if err != nil {
		t.Fatalf("打开旧墓碑失败: %v", err)
	}
	defer held.Close()
	if err := got.SetPhase(PhaseGone); err != nil {
		t.Fatalf("failed → gone 失败: %v", err)
	}
	if err := WriteTombstone(root, got); err != nil {
		t.Fatalf("第二次 WriteTombstone 失败: %v", err)
	}
	heldRaw, err := io.ReadAll(held)
	if err != nil {
		t.Fatalf("读旧 fd 失败: %v", err)
	}
	if !bytes.Equal(heldRaw, raw) {
		t.Fatalf("墓碑写入不是 rename 语义（老 fd 看到新内容 = 原地写）:\nheld=%s\nwant=%s", heldRaw, raw)
	}
	if after := mustTombstone(t, root, "c-tomb"); after.Phase != PhaseGone {
		t.Fatalf("墓碑覆盖写没生效: phase=%q", after.Phase)
	}

	// 护栏：不存在 ⇒ ErrNotFound；id 带路径分隔符 ⇒ 拒（别写到 root 外面去）。
	if _, err := ReadTombstone(root, "nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("墓碑不存在应报 ErrNotFound，实得 %v", err)
	}
	for _, bad := range []string{"", "..", "a/b", `a\b`} {
		bm := New("ok", "egg-x", "A1")
		bm.ID = bad
		if err := WriteTombstone(root, bm); !errors.Is(err, ErrInvalidMeta) {
			t.Fatalf("WriteTombstone(id=%q) 应报 ErrInvalidMeta，实得 %v", bad, err)
		}
	}
	if err := WriteTombstone(root, nil); !errors.Is(err, ErrInvalidMeta) {
		t.Fatalf("WriteTombstone(nil) 应报 ErrInvalidMeta，实得 %v", err)
	}
}

// mustTombstone — 读墓碑失败即停。
func mustTombstone(t *testing.T, root, id string) *Meta {
	t.Helper()
	m, err := ReadTombstone(root, id)
	if err != nil {
		t.Fatalf("ReadTombstone(%s) 失败: %v", id, err)
	}
	return m
}
