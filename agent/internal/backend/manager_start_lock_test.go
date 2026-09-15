// manager_start_lock_test.go —— 反例用例：`Manager.Start` 自锁死（2026-09-15 第一枚卵真机实测缺陷 1）。
//
// 缺陷原文（报告 §12 缺陷 1）：Start 里先 `m.mu.Lock()`，紧接着调 `m.RequestModel(...)`，
// 而后者自己再 `m.mu.Lock()`（p2_lifecycle.go RequestModel）——sync.Mutex 不可重入 ⇒
// **第一次 /load 就死**，且同一把锁被 /status、心跳、巡检、事件循环争用 ⇒ 整个子端僵死
// （真机：`curl /load` 290s 无返回；SIGQUIT dump：g444 停在 RequestModel:279，g8/g9/g11/g784 全等同一把锁）。
//
// 本文件钉住三件事（任一条被改回去，这里先红）：
//
//	① Start 在「进入正常加载路径」之前**不得持锁**（持锁调 RequestModel = 自锁死）；
//	② 未知模型（最省事的路径：连 spawn 都不做）也必须**立刻**返回 404，不是 5s/120s 无返回；
//	③ 并发 Start 后 m.mu 必须已释放（锁配对：删掉 Start 头上的 Lock 而漏删某个 Unlock，
//	   会让 mutex 处于已释放态被二次 Unlock ⇒ panic；反之漏删 Lock 则这里永久卡住）。
//
// 未打补丁的副本上：①②③ 全超时失败（原始输出见交付证据）；打上补丁后全绿。
package backend

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Mr2109/zerg-swarm/agent/internal/registry"
)

// startLockTestManager 造一台可走完 Start 前半段的 Manager。
//
// 条目里的 cmd 指向**不存在的二进制**：即便流程真走到 spawn 也只会快速返回 500
// （绝不在这台机器上拉起任何引擎）——本用例只关心「锁有没有自锁死」，不关心 spawn 成败。
func startLockTestManager(t *testing.T, names ...string) *Manager {
	t.Helper()
	dir := t.TempDir()
	var sb strings.Builder
	for _, n := range names {
		fmt.Fprintf(&sb, "%s:\n  backend: llama-server\n  file: /nonexistent/%s.gguf\n  mem_gb: 1\n"+
			"  cmd: /nonexistent/zerg-start-lock-noop -m {file} --port {port}\n", n, n)
	}
	path := filepath.Join(dir, "agent_models.yaml")
	if err := os.WriteFile(path, []byte(sb.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	reg, err := registry.New(path)
	if err != nil {
		t.Fatalf("加载测试注册表失败: %v", err)
	}
	// NewManager：孵化开关未设 ⇒ 关（既有裸 exec 路径），启动 GC 也不动作。
	return NewManager(reg, "x3")
}

// ① 未知模型：最省事的一条路径，必须秒回 404 —— 自锁死时这里会永久卡住。
func TestStart_UnknownModel_NoSelfDeadlock(t *testing.T) {
	m := startLockTestManager(t, "egga")

	type res struct {
		resp map[string]interface{}
	}
	done := make(chan res, 1)
	go func() {
		resp, _ := m.Start("ghost-model")
		done <- res{resp: resp}
	}()

	select {
	case got := <-done:
		if got.resp == nil || got.resp["status"] != 404 {
			t.Fatalf("未知模型应返回 404，实得 %+v", got.resp)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Start 5s 未返回：Start 持 m.mu 调 RequestModel（不可重入）⇒ 自锁死；" +
			"真机上这就是「第一次 /load 整子端僵死」")
	}

	// ③ 锁必须已释放（TryLock 成功 = 没人持有）。
	if !m.mu.TryLock() {
		t.Fatal("Start 返回后 m.mu 仍被持有：Unlock 未配对（/status、心跳、巡检会一起僵死）")
	}
	m.mu.Unlock()
}

// ② 并发 /load（同模型 + 异模型 + 未知模型混着来）都必须返回，且不得互相卡死。
func TestStart_ConcurrentLoads_NoSelfDeadlock(t *testing.T) {
	m := startLockTestManager(t, "egg-a", "egg-b")

	models := []string{"egg-a", "egg-b", "ghost-model"}
	const perModel = 4

	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		bad  []string
		seen = map[string]int{}
	)
	for _, model := range models {
		for i := 0; i < perModel; i++ {
			wg.Add(1)
			go func(model string) {
				defer wg.Done()
				done := make(chan map[string]interface{}, 1)
				go func() {
					resp, _ := m.Start(model)
					done <- resp
				}()
				select {
				case resp := <-done:
					mu.Lock()
					seen[model]++
					mu.Unlock()
					if resp == nil {
						mu.Lock()
						bad = append(bad, model+"：Start 返回 nil 响应")
						mu.Unlock()
					}
				case <-time.After(30 * time.Second):
					mu.Lock()
					bad = append(bad, model+"：Start 30s 未返回（自锁死 / 加载槽没合上）")
					mu.Unlock()
				}
			}(model)
		}
	}
	wg.Wait()

	if len(bad) > 0 {
		t.Fatalf("并发 /load 出现卡死：%v", bad)
	}
	for _, model := range models {
		if seen[model] != perModel {
			t.Fatalf("模型 %s 应返回 %d 次，实得 %d（有请求被吞）", model, perModel, seen[model])
		}
	}
	if !m.mu.TryLock() {
		t.Fatal("并发 Start 后 m.mu 仍被持有：与 /status、心跳、巡检争用的那把锁没放")
	}
	m.mu.Unlock()

	// 加载槽也要收干净（合并等待的账本不许留残）。
	m.mu.Lock()
	leftover := len(m.loading)
	m.mu.Unlock()
	if leftover != 0 {
		t.Fatalf("加载槽未收干净，残留 %d 个", leftover)
	}
}

// ④ 结构性断言：Start 在取锁之前就调了 RequestModel（顺序反了就是自锁死）。
// 这条断言防的是「把 m.mu.Lock() 挪回函数头上」这类手滑——功能性用例只会在超时上体现，
// 这里直接把顺序钉住。
func TestStart_LockOrder_RequestModelBeforeLock(t *testing.T) {
	src, err := readFileForAssertion("manager.go")
	if err != nil {
		t.Fatalf("读 manager.go 失败: %v", err)
	}
	body := funcBody(t, src, "func (m *Manager) Start(modelName string)")
	code := codeOnly(body)
	iReq := strings.Index(code, "m.RequestModel(modelName)")
	iLock := strings.Index(code, "m.mu.Lock()")
	if iReq < 0 || iLock < 0 {
		t.Fatalf("Start 的**代码**里应同时出现 RequestModel 与 m.mu.Lock（iReq=%d iLock=%d）", iReq, iLock)
	}
	if iLock < iReq {
		t.Fatal("Start 在调用 RequestModel 之前就持了 m.mu：RequestModel 自己会加锁 ⇒ " +
			"不可重入 ⇒ 自锁死（真机缺陷 1 复现）")
	}
}

// codeOnly 去掉整行注释后的源码（结构性断言只认代码——注释里提到 Lock 不算数）。
func codeOnly(src string) string {
	var sb strings.Builder
	for _, line := range strings.Split(src, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "//") {
			continue
		}
		sb.WriteString(line)
		sb.WriteString("\n")
	}
	return sb.String()
}

// funcBody 取某个顶层函数从签名到下一个顶层 func 之间的源码（结构性断言用）。
func funcBody(t *testing.T, src, sig string) string {
	t.Helper()
	i := strings.Index(src, sig)
	if i < 0 {
		t.Fatalf("源码里找不到 %q", sig)
	}
	rest := src[i:]
	if j := strings.Index(rest[1:], "\nfunc "); j >= 0 {
		return rest[:j+1]
	}
	return rest
}
