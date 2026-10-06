package agent

// net_tools_bridge_timeout_test.go — A-06（2026-10-07）：
// searx 桥子进程「超时」＋「空结果」两条 fail-closed 的回归牙。
//
// 病灶（改前）：
//   - exec.Command(python, …) 无超时 ⇒ 桥「挂起不答」时 web_search 永久阻塞；
//   - 桥回 {"results": []}（服务不可达 / 引擎全失败）时 Go 侧静默回「（无搜索结果）」= fail-open。
//
// 三腿（全部用**仓外**夹具真起进程 —— ✗ 不起真 searxng）：
//   ① 正控：夹具回正常 JSON ⇒ 无错、能格式化（证明注入路径可用、超时闸不误伤正常返回）。
//   ② 超时：夹具「挂起不答」（sleep 3600）⇒ 必须在 searxBridgeTimeout 内返回带具名原因「超时」的 error。
//   ③ 空结果：夹具回 {"results": []} ⇒ 必须返回带具名原因「空结果」的 error（✗ 不得 nil）。
//
// 负控：父代理跑 `go test -overlay`（把超时闸哑化成无上限 / 把空结果分支还原成
// `return "（无搜索结果）", nil`）⇒ 对应腿必红（逐字见回执）。
// 夹具注入口 = 包内变量 searxBridgePython（**不 export ZERG_***，也不改生产行为）。

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeBridgeFixture 在**仓外**临时目录落一个假桥可执行件（POSIX shell），返回其路径。
// 它充当「python 解释器」被 exec 直接调起（收到的参数被忽略），从而**真起一个进程**（非 in-proc mock）。
func writeBridgeFixture(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "fake_searx_bridge.sh")
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatalf("写仓外夹具失败: %v", err)
	}
	return p
}

// withBridgeFixture 注入假桥（包内变量 —— 测试隔离），测试结束自动复原。
func withBridgeFixture(t *testing.T, fixture string) {
	t.Helper()
	// A2 步3：默认路已切 Go 客户端直连服务（HTTP 8888）⇒ 本批「假桥」用例须显式开回退旋钮，
	// 才走旧 Python 桥（searx_search.py）。这就是回退旋钮的回归牙之一。
	t.Setenv(searxLegacyBridgeEnv, "1")
	old := searxBridgePython
	searxBridgePython = fixture
	t.Cleanup(func() { searxBridgePython = old })
}

// withBridgeTimeout 临时改超时值（包内变量 —— 测试隔离），测试结束自动复原。
func withBridgeTimeout(t *testing.T, d time.Duration) {
	t.Helper()
	old := searxBridgeTimeout
	searxBridgeTimeout = d
	t.Cleanup(func() { searxBridgeTimeout = old })
}

// TestSearxBridgeFixturePositiveControl —— 腿①：假桥回正常 JSON ⇒ 正常返回。
func TestSearxBridgeFixturePositiveControl(t *testing.T) {
	withBridgeFixture(t, writeBridgeFixture(t,
		`printf '{"results":[{"title":"t1","url":"http://example.com/1","content":"c1","score":1.0}]}'`))
	withBridgeTimeout(t, 5*time.Second)

	out, err := WebSearch("正控", 3)
	if err != nil {
		t.Fatalf("正控：正常 JSON 不应报错，实得 %v", err)
	}
	if !strings.Contains(out, "http://example.com/1") {
		t.Fatalf("正控：输出未含夹具结果: %q", out)
	}
}

// TestSearxBridgeTimeoutFailsClosed —— 腿②：桥「挂起不答」⇒ 超时内返回带具名原因「超时」的 error。
// 看门狗：即便超时闸被哑化（无上限），本用例也在 watchdog 内 t.Fatalf 带行号（✗ 不死等）。
func TestSearxBridgeTimeoutFailsClosed(t *testing.T) {
	withBridgeFixture(t, writeBridgeFixture(t, "sleep 10")) // 挂起不答
	withBridgeTimeout(t, 150*time.Millisecond)

	type res struct {
		out string
		err error
		d   time.Duration
	}
	ch := make(chan res, 1)
	go func() {
		t0 := time.Now()
		out, err := WebSearch("超时负控", 3)
		ch <- res{out, err, time.Since(t0)}
	}()

	var r res
	select {
	case r = <-ch:
	case <-time.After(3 * time.Second):
		t.Fatalf("桥挂起不答却未在 3s 看门狗内返回 —— 超时闸失效（挂起未被拦）")
	}
	if r.err == nil {
		t.Fatalf("桥挂起不答应 fail-closed 报超时 error，实得 nil（out=%q）", r.out)
	}
	if !strings.Contains(r.err.Error(), "超时") {
		t.Fatalf("超时错误未带具名原因「超时」: %v", r.err)
	}
	if r.d > 2*time.Second {
		t.Fatalf("超时未在阈值附近生效：耗时 %s（阈值 150ms）", r.d)
	}
	t.Logf("超时腿：耗时 %s → %v", r.d.Round(time.Millisecond), r.err)
}

// TestSearxBridgeEmptyFailsClosed —— 腿③：桥回 {"results": []}（服务不可达/引擎全失败）⇒
// 必须返回带具名原因「空结果」的 error（✗ 不得静默回「（无搜索结果）」）。
func TestSearxBridgeEmptyFailsClosed(t *testing.T) {
	withBridgeFixture(t, writeBridgeFixture(t, `printf '{"results": []}'`))
	withBridgeTimeout(t, 5*time.Second)

	out, err := WebSearch("空结果负控", 3)
	if err == nil {
		t.Fatalf("空结果应 fail-closed 报 error，实得 nil（out=%q）—— 静默「无搜索结果」= fail-open", out)
	}
	if !strings.Contains(err.Error(), "空结果") {
		t.Fatalf("空结果错误未带具名原因「空结果」: %v", err)
	}
	t.Logf("空结果腿：%v", err)
}

// TestSearxBridgeV2EmptyFailsClosed —— 腿③（V2 路径）：webSearchBridge 空结果同样 fail-closed。
func TestSearxBridgeV2EmptyFailsClosed(t *testing.T) {
	withBridgeFixture(t, writeBridgeFixture(t, `printf '{"results": []}'`))
	withBridgeTimeout(t, 5*time.Second)

	r, err := webSearchBridge("空结果负控 v2", 3, "all", "")
	if err == nil {
		t.Fatalf("webSearchBridge 空结果应 fail-closed 报 error，实得 nil（r=%+v）", r)
	}
	if !strings.Contains(err.Error(), "空结果") {
		t.Fatalf("webSearchBridge 空结果错误未带具名原因「空结果」: %v", err)
	}
}

// TestSearxBridgeV2TimeoutFailsClosed —— 腿②（V2 路径）：webSearchBridge 桥挂起 ⇒ 超时内 error。
func TestSearxBridgeV2TimeoutFailsClosed(t *testing.T) {
	withBridgeFixture(t, writeBridgeFixture(t, "sleep 10"))
	withBridgeTimeout(t, 150*time.Millisecond)

	type res struct {
		err error
		d   time.Duration
	}
	ch := make(chan res, 1)
	go func() {
		t0 := time.Now()
		_, err := webSearchBridge("超时负控 v2", 3, "all", "")
		ch <- res{err, time.Since(t0)}
	}()
	select {
	case r := <-ch:
		if r.err == nil {
			t.Fatalf("webSearchBridge 桥挂起不答应报超时 error，实得 nil")
		}
		if !strings.Contains(r.err.Error(), "超时") {
			t.Fatalf("webSearchBridge 超时错误未带具名原因「超时」: %v", r.err)
		}
		t.Logf("超时腿 v2：耗时 %s → %v", r.d.Round(time.Millisecond), r.err)
	case <-time.After(3 * time.Second):
		t.Fatalf("webSearchBridge 桥挂起不答却未在 3s 看门狗内返回 —— 超时闸失效")
	}
}
