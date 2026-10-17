// residency_ledger_test.go —— 驻留账本与「进程没了」巡检的判据件（缺口 `GAP-20261018-13`）。
//
// 一条约定：**本件不碰真实状态目录**（每个用例都把 `ZERG_EGG_RESIDENCY_LEDGER` 指到 `t.TempDir()`），
// 也**不启停任何真实卵**——巡检的对象是测试自己 spawn 的 `sleep`，进程生命周期由本件掌握。
//
// 四格（全部真跑，不是读源码）：
//
//	① 有活进程 ⇒ 落账（loaded）· 落盘 · 不误报
//	② 进程被杀 ⇒ 下一次巡检判 `vanished` ＋ **事件件里有一行** `reason=process_vanished`
//	   （本缺口的正题：只沉日志不算数 —— 日志会被心跳冲掉）
//	③ 显式卸载 ⇒ `released`（**不**判 vanished：人按的停不是掉线）
//	④ pid 拿不到 ⇒ **不判**（「不知道」不当「死了」）；合成 subproc（pid=0）不落账
package backend

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// syscallKillForTest 发 SIGKILL（测试自己的子进程；生产路径只读，不杀任何东西）。
func syscallKillForTest(pid int) error { return syscall.Kill(pid, syscall.SIGKILL) }

// resTestLedger 把账本与事件件指到临时目录（返回账本路径）。用例结束自动还原环境。
func resTestLedger(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "egg-residency.json")
	t.Setenv(EnvResidencyLedger, p)
	t.Setenv(EnvResidencyPatrolMinS, "0") // 0 ⇒ 走缺省 2s；本件用手工清零节流，不靠它
	return p
}

// resSleeper 测试自己的「引擎进程」（pid 与两个动作：等它退出 / 杀掉它）。
type resSleeper struct {
	pid  int
	done chan struct{}
}

// resTestSleeper 起一个真进程当「引擎」。
func resTestSleeper(t *testing.T) *resSleeper {
	t.Helper()
	cmd := exec.Command("sleep", "60")
	if err := cmd.Start(); err != nil {
		t.Fatalf("起测试进程失败（sleep 不在？）：%v", err)
	}
	s := &resSleeper{pid: cmd.Process.Pid, done: make(chan struct{})}
	go func() { _ = cmd.Wait(); close(s.done) }()
	t.Cleanup(func() { _ = cmd.Process.Kill(); <-s.done })
	return s
}

// wait 等它真的退出（被杀也算 —— 本件只关心「进程不在了」这一事实）。
func (s *resSleeper) wait() { <-s.done }

// kill 杀掉它（模拟真机上「llama-server 无声消失」）。
func (s *resSleeper) kill(t *testing.T) {
	t.Helper()
	if err := syscallKillForTest(s.pid); err != nil {
		t.Fatalf("杀测试进程 %d 失败：%v", s.pid, err)
	}
	s.wait()
}

// resForcePatrol 清掉巡检节流（同一个用例里要连跑两次）。
func resForcePatrol(m *Manager) {
	m.resMu.Lock()
	m.residencyQuietUntil = time.Time{}
	m.resMu.Unlock()
}

// ①② 活进程落账 → 进程没了判 vanished ＋ 事件件留一行。
func TestPatrolResidencyVanishedWritesEvent(t *testing.T) {
	ledger := resTestLedger(t)
	m := &Manager{procs: map[string]*subproc{}, loading: map[string]*loadWaiter{}}
	sl := resTestSleeper(t)
	pid := sl.pid

	m.procs["egg-a"] = &subproc{model: "egg-a", state: StateReady, enginePID: pid, port: 0}

	recs := m.PatrolResidency()
	if len(recs) != 1 {
		t.Fatalf("活进程必须落一条账（下限 1 条）· 实得 %d 条：fail-closed 判红", len(recs))
	}
	if recs[0].State != ResidencyStateLoaded {
		t.Fatalf("活进程的账必须是 %q，实得 %q", ResidencyStateLoaded, recs[0].State)
	}
	if recs[0].PID != pid {
		t.Fatalf("账本 pid 必须是实测 pid %d，实得 %d", pid, recs[0].PID)
	}
	if recs[0].LoadedAt == "" {
		t.Fatalf("装载时刻必须落（体检面的「上次装载时刻」靠它）")
	}
	if _, err := os.Stat(ledger); err != nil {
		t.Fatalf("账本必须**落盘**（证据要活过进程退出）：%v", err)
	}

	// 杀掉进程（模拟真机上「llama-server 无声消失」），再巡检一次。
	sl.kill(t)
	delete(m.procs, "egg-a")
	resForcePatrol(m)

	recs = m.PatrolResidency()
	if len(recs) != 1 {
		t.Fatalf("判 vanished 之后记录**不许被删**（它是证据）：实得 %d 条", len(recs))
	}
	if recs[0].State != ResidencyStateVanished {
		t.Fatalf("进程没了 ⇒ 账必须落 %q，实得 %q（reason=%s）", ResidencyStateVanished, recs[0].State, recs[0].Reason)
	}
	if recs[0].Reason != ResidencyReasonProcessVanished {
		t.Fatalf("reason 必须是 %q，实得 %q", ResidencyReasonProcessVanished, recs[0].Reason)
	}
	if recs[0].LastSeen == "" || recs[0].DetectedAt == "" {
		t.Fatalf("vanish 必须留下「最后见到活着」与「判出时刻」两格：%+v", recs[0])
	}

	// 事件件：**结构化一行**（不只日志）—— 缺口单点名的五格都在。
	evPath := ledger + ".events.jsonl"
	b, err := os.ReadFile(evPath)
	if err != nil {
		t.Fatalf("事件件必须落盘（%s）：%v", evPath, err)
	}
	line := strings.TrimSpace(string(b))
	if line == "" {
		t.Fatalf("事件件是空的 ⇒ 等于只沉了日志（本缺口原现象）")
	}
	var ev map[string]interface{}
	if err := json.Unmarshal([]byte(strings.Split(line, "\n")[0]), &ev); err != nil {
		t.Fatalf("事件行不是合法 JSON：%v · %q", err, line)
	}
	for _, k := range []string{"time", "egg", "model", "pid", "last_seen", "reason"} {
		if _, ok := ev[k]; !ok {
			t.Errorf("事件行缺 `%s` 格（缺口单点名的五格必须齐）：%q", k, line)
		}
	}
	if got, _ := ev["reason"].(string); got != ResidencyReasonProcessVanished {
		t.Errorf("事件 reason = %v（要 %q）", ev["reason"], ResidencyReasonProcessVanished)
	}
	if got, _ := ev["egg"].(string); got != "egg-a" {
		t.Errorf("事件 egg = %v（要 egg-a）", ev["egg"])
	}
	if got, _ := ev["pid"].(float64); int(got) != pid {
		t.Errorf("事件 pid = %v（要 %d）", ev["pid"], pid)
	}
}

// ③ 显式卸载 ⇒ released（不判 vanished）。
func TestPatrolResidencyReleasedNotVanished(t *testing.T) {
	resTestLedger(t)
	m := &Manager{procs: map[string]*subproc{}, loading: map[string]*loadWaiter{}}
	sl := resTestSleeper(t)
	m.procs["egg-b"] = &subproc{model: "egg-b", state: StateReady, enginePID: sl.pid}
	m.PatrolResidency()

	// 显式卸载（生产入口 = evictSubprocLocked；这里直接打它的唯一账本动作，避免真去停进程）。
	m.markResidencyReleased("egg-b", "TTL 到期(空闲>5m0s)")
	sl.kill(t)
	delete(m.procs, "egg-b")
	resForcePatrol(m)

	recs := m.PatrolResidency()
	if len(recs) != 1 {
		t.Fatalf("记录不许被删（下限 1 条）· 实得 %d", len(recs))
	}
	if recs[0].State != ResidencyStateReleased {
		t.Fatalf("显式卸载过的卵必须停在 %q（✗ 不许改判 vanished —— 人按的停不是掉线）：实得 %q（reason=%s）",
			ResidencyStateReleased, recs[0].State, recs[0].Reason)
	}
}

// ④ pid 拿不到的记录 ⇒ 不判（不把「不知道」当「死了」）；合成 subproc（pid=0）不落账。
func TestPatrolResidencyUnknownPidIsNotJudged(t *testing.T) {
	ledger := resTestLedger(t)
	// 手写一份账（pid=0），模拟「装载时没拿到进程证据」。
	doc := `{"schema":"zerg-egg-residency/1","updated_at":"2026-10-18T00:00:00Z","records":[
	  {"egg_id":"egg-c","model":"egg-c","pid":0,"port":9000,"state":"loaded","last_seen":"2026-10-18T00:00:00Z"}]}`
	if err := os.WriteFile(ledger, []byte(doc), 0o644); err != nil {
		t.Fatalf("写夹具账本失败：%v", err)
	}
	m := &Manager{procs: map[string]*subproc{}, loading: map[string]*loadWaiter{}}
	// 合成 subproc：pid 恒 0（enginePID/proc 都没有）⇒ **不许**落账。
	m.procs["ghost"] = &subproc{model: "ghost", state: StateReady, port: 9400}

	recs := m.PatrolResidency()
	if len(recs) != 1 {
		t.Fatalf("只有夹具那一条（pid=0 的合成 subproc 不许落账，下限 1 上限 1 ⇒ fail-closed 判红）：实得 %d 条", len(recs))
	}
	if recs[0].EggID != "egg-c" {
		t.Fatalf("夹具记录必须原样保留：%+v", recs[0])
	}
	if recs[0].State != ResidencyStateLoaded {
		t.Fatalf("pid 拿不到 ⇒ **不判**（必须留在 %q），实得 %q", ResidencyStateLoaded, recs[0].State)
	}
	if !strings.Contains(recs[0].Detail, "pid 未知") {
		t.Fatalf("「不判」的理由要留痕（Detail 里说明 pid 未知）：%q", recs[0].Detail)
	}
}
