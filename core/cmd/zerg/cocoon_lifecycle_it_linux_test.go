//go:build linux

// cocoon_lifecycle_it_linux_test.go — 命令面接**真 rootfs** 的真机格（T3b）。
//
// 只在「真 linux + root + 有 nsjail + 指了一个镜像根目录」时跑；缺任何一样都 Skip 并说清缺什么
// （缺前置就 Skip 是本仓的惯例：假绿比不跑更糟，所以 Skip 的理由必须能被读到）。
//
// 环境变量（与 internal/cocoon 的真机四格同形）：
//
//	ZERG_COCOON_IT_ROOTFS  必需。已解包的最小 rootfs 目录
//	ZERG_COCOON_IT_BASE    可选。茧根父目录（缺省 t.TempDir()）。★ 该盘必须能当 overlay 的
//	                       **上层**：tmpfs/ext4/xfs 可以；容器里的 overlayfs（/tmp 常是它）不行。
//
// 本格打的是**命令面**（不是后端 API）：`cmdCocoonOpen` 拿契约（`[茧].rootfs`）起茧之后，
//
//	① 实例根落在**茧目录**里（`<状态根>/<id>/<后端签发的实例 id>`）⇒ BaseDir 真的接到了命令面；
//	② `merged` 是**真 overlay 挂载点**（不是同名空目录）；
//	③ 茧内 `/usr` 只读（EROFS）、`/tmp` 可写，且新写的文件出现在**茧目录下的 upper** 里；
//	④ `inspect` 人面印 `overlay=true`（能力位来自后端自报，不是平台名猜的）；
//	⑤ `close` 收得掉：phase=gone 且实例根消失（Destroy 用的也是同一个 BaseDir）。
//
// ★ 进程模型（T3c）：服务走**后台档**起（`ExecOpts.Detach`：setsid + 不等退出），所以
// 「CLI 退了服务还在」这件事本身不再靠测试进程一直活着。本格仍然是在**同一进程里**调命令函数
// （便于直接读 meta/盘面），进程级验收（真 CLI 退出后宿主 curl 得 200、close 收干净、负控）
// 走 `scripts/` 外的验收脚本；本格另加一条「open 之后后端签出的 pid 真活着」的断言。
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/cocoon"
)

// ccITSpecTpl — 真 rootfs 的最小契约（`%s` = 镜像根）：一个长驻服务，四步探针一次做完。
const ccITSpecTpl = `[茧]
name   = "shop"
image  = "debian:bookworm"
rootfs = "%s"

[[服务]]
name  = "api"
entry = ["/bin/sh", "-c", "echo from-image; (echo no > /usr/probe-from-cli) 2>/dev/null || echo USR_DENIED; echo cmdface > /opt/cmdface.txt; echo tmpx > /tmp/cmdface.txt; sleep 600"]

[资源]
cpu     = 1
mem_gb  = 2
disk_gb = 10
`

func TestCocoonOpenWithRootfsIT(t *testing.T) {
	rootfs := strings.TrimSpace(os.Getenv("ZERG_COCOON_IT_ROOTFS"))
	if rootfs == "" {
		t.Skip("未设 ZERG_COCOON_IT_ROOTFS（真 rootfs 目录）：本格只在 linux 真机/特权容器里跑")
	}
	if os.Geteuid() != 0 {
		t.Skip("需要 root：overlay mount + chroot 都要它")
	}
	if _, err := exec.LookPath("nsjail"); err != nil {
		t.Skipf("找不到 nsjail（%v）", err)
	}
	if fi, err := os.Stat(rootfs); err != nil || !fi.IsDir() {
		t.Skipf("ZERG_COCOON_IT_ROOTFS=%s 不是一个可读目录（err=%v）", rootfs, err)
	}

	base := strings.TrimSpace(os.Getenv("ZERG_COCOON_IT_BASE"))
	if base == "" {
		base = t.TempDir()
	}
	proj := filepath.Join(base, "proj")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatalf("建项目目录: %v", err)
	}
	if err := os.WriteFile(filepath.Join(proj, cocoon.SpecFileName),
		[]byte(fmt.Sprintf(ccITSpecTpl, rootfs)), 0o644); err != nil {
		t.Fatalf("写契约: %v", err)
	}
	root := filepath.Join(base, "cocoons")
	t.Setenv(cocoonRootEnv, root)
	t.Setenv(cocoonAllowRootsEnv, base)
	t.Setenv(cocoonBackendEnv, cocoonBackendNsjail)
	t.Setenv(cocoonTokenEnv, "")
	t.Setenv(cocoonAIBaseEnv, "")

	rc, out, errs := ccLifeRun(t, cmdCocoonOpen, []string{"cocoon", "open"}, []string{proj},
		[]string{"id", "phase", "backend"}, nil)
	if rc != exitOK {
		t.Fatalf("open rc=%d 期望 0（真 rootfs 起不来就不许报成功）· stdout=%q · stderr=%s", rc, out, errs)
	}
	rows := ccLifeRows(t, out)
	if len(rows) != 1 {
		t.Fatalf("open --json 该回恰一行，得到 %d 行：%s", len(rows), out)
	}
	id := ccLifeCell(rows[0], "id")
	if id == "" {
		t.Fatalf("open 没给 id：%s", out)
	}
	cdir := filepath.Join(root, id)

	m, err := cocoon.Load(cocoon.MetaPath(cdir))
	if err != nil {
		t.Fatalf("读 meta: %v", err)
	}
	if m.Backend != cocoonBackendNsjail || m.BackendInstance == "" {
		t.Fatalf("meta 该记着真后端与它签发的句柄：backend=%q instance=%q", m.Backend, m.BackendInstance)
	}

	// ① 实例根在茧目录里（命令面把 BaseDir 接到了 — 本单的主判据）。
	instRoot := filepath.Join(cdir, m.BackendInstance)
	if got, want := filepath.Dir(instRoot), cdir; got != want {
		t.Fatalf("实例根 %q 的父目录=%q，want 茧目录 %q（BaseDir 没接到命令面就是这种形状）", instRoot, got, want)
	}

	// ①b 后台档签出的 pid 必须落盘且真活着（T3c 的主判据在命令面；这里验「签出的句柄是真的」）。
	if m.PID <= 0 || len(m.BackendPIDs) == 0 {
		t.Fatalf("后台档该把后端签出的 pid 落进 meta：pid=%d backend_pids=%v", m.PID, m.BackendPIDs)
	}
	if !ccITProcAlive(m.BackendPIDs[0]) {
		t.Fatalf("open 返回后 pid %d 已经不在（后台档没生效：服务又被起它的进程带走了）", m.BackendPIDs[0])
	}

	// ②③ 服务真跑起来：merged 是 overlay 挂载点、日志有茧内输出、upper 有茧内新写的文件。
	merged := filepath.Join(instRoot, "merged")
	if !ccITWaitMount(merged) {
		t.Fatalf("%s 不是 overlay 挂载点（服务没起 / overlay 没挂）· stderr=%s", merged, errs)
	}
	// 服务是 goroutine 起的 ⇒ 日志/upper 都要等它写完（轮询，不 sleep 定值）。
	logPath := filepath.Join(cdir, "logs", "api.log")
	if !ccITWaitText(logPath, "from-image") {
		logText, _ := os.ReadFile(logPath)
		t.Fatalf("茧内没跑起来（%s 里等不到 from-image）：%q", logPath, string(logText))
	}
	if !ccITWaitText(logPath, "USR_DENIED") {
		logText, _ := os.ReadFile(logPath)
		t.Fatalf("茧内写 /usr 应当被拒（EROFS）：%q", string(logText))
	}
	// ★ 写 /tmp 走的是「每座茧一个可写目录」那条 bind（<实例根>/tmp），**不经 overlay**；
	//   要判「upper 里真有茧内新写的文件」，得写一个只读面之外的正常目录（/opt）。
	upperFile := filepath.Join(instRoot, "upper", "opt", "cmdface.txt")
	if !ccITWaitFile(upperFile) {
		t.Fatalf("可写层没落在茧目录的 upper 里（%s 等不到）", upperFile)
	}
	up, rerr := os.ReadFile(upperFile)
	if rerr != nil {
		t.Fatalf("读 upper 里的 %s 失败：%v", upperFile, rerr)
	}
	if strings.TrimSpace(string(up)) != "cmdface" {
		t.Fatalf("upper 里的内容不对：%q", string(up))
	}
	if !ccITWaitText(filepath.Join(instRoot, "tmp", "cmdface.txt"), "tmpx") {
		t.Fatalf("茧内 /tmp 应当可写（落在 <实例根>/tmp）")
	}

	// ④ inspect 的可写层位来自后端自报（真后端 = true + overlayfs 机制）。
	_, human, _ := ccLifeRun(t, cmdCocoonInspect, []string{"cocoon", "inspect"}, []string{id}, nil, nil)
	if !strings.Contains(human, "overlay=true") {
		t.Fatalf("真后端 inspect 该印 overlay=true：\n%s", human)
	}
	if !strings.Contains(human, "overlayfs") {
		t.Fatalf("overlay=true 时该印机制（overlayfs）：\n%s", human)
	}

	// ⑤ close 收得掉（Destroy 也用同一个 BaseDir；收不掉会留一堆 upper/merged）。
	//
	// ★ 最多试三次：`close` 在**真世界里是另一个进程**，那时服务进程早已被父进程退出带走；
	//   本格是在**同一个进程**里连调 open→close，服务的 Exec goroutine 还在跑，Destroy 杀完
	//   立刻删 cgroup 目录会撞「device or resource busy」（内核还没把进程从 cgroup 里摘干净）。
	//   这条时序缺口是「同进程连调」才有的（真路径见 T3 的跨进程收尾格），本格不拿它当判据，
	//   只用有限重试把「收了之后盘上确实干净」验完。
	var lastErr string
	closed := false
	for attempt := 1; attempt <= 3; attempt++ {
		rc, cout, cerrs := ccLifeRun(t, cmdCocoonClose, []string{"cocoon", "close"}, []string{id}, nil, nil)
		if rc == exitOK {
			closed = true
			break
		}
		lastErr = fmt.Sprintf("第 %d 次：rc=%d stdout=%q stderr=%s", attempt, rc, cout, cerrs)
		time.Sleep(300 * time.Millisecond)
	}
	if !closed {
		t.Fatalf("close 三次都没收干净（%s）", lastErr)
	}
	if ph, ok := ccLifeMetaPhase(t, root, id); !ok || ph != cocoon.PhaseGone {
		t.Fatalf("close 后 phase=%q（期望 gone）", ph)
	}
	if _, err := os.Stat(instRoot); !os.IsNotExist(err) {
		t.Fatalf("close 后实例根 %s 还在（stat err=%v）", instRoot, err)
	}
}

// ccITWaitText — 等文件里出现 want（服务是 goroutine 起的，日志是异步写进去的）。
func ccITWaitText(path, want string) bool {
	for i := 0; i < 500; i++ {
		if b, err := os.ReadFile(path); err == nil && strings.Contains(string(b), want) {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}

// ccITWaitFile — 等文件出现且非空。
func ccITWaitFile(path string) bool {
	for i := 0; i < 500; i++ {
		if fi, err := os.Stat(path); err == nil && fi.Size() > 0 {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}

// ccITWaitMount —— 等到 path 出现在 /proc/mounts 里（服务是 goroutine 起的，挂载要等它做完）。
func ccITWaitMount(path string) bool {
	for i := 0; i < 100; i++ { // 最坏 ~10s：真机起 nsjail 是毫秒级，这个窗只防抖
		if b, err := os.ReadFile("/proc/mounts"); err == nil {
			for _, line := range strings.Split(string(b), "\n") {
				f := strings.Fields(line)
				if len(f) >= 3 && f[1] == path && f[2] == "overlay" {
					return true
				}
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

// ccITProcAlive — pid 还在 /proc 里且不是僵尸（判活要 fail-closed：读不到就算死）。
func ccITProcAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return false
	}
	i := strings.LastIndexByte(string(b), ')')
	if i < 0 || i+2 >= len(b) {
		return false
	}
	st := string(b)[i+2]
	return st != 'Z' && st != 'X'
}
