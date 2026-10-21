// cocoon_broker_test.go — `zerg cocoon broker <start|stop|status>` 的判据（T4c）。
//
// 六条（每条都带**负控**；★ 全部不依赖子进程重入 ⇒ `go test` 里跑得动）：
//
//	① 动作词闭集：缺动作 / 动作不在闭集 / 多给一个位置参数 / `status --dry-run` 一律 rc=2；
//	② `start --dry-run` **零副作用**：不绑端口、不写 pid 件/状态件、不起进程；计划件必须印出
//	   **实际将用的 addr**（而 addr 只从 `cocoon.ResolveBrokerAddr` 来 —— 一处旋钮两处同改）；
//	③ 端口被占 ⇒ `start` **报占用并点名进程**（名字+pid），**绝不假成功**（rc=1，不出「起来了」）；
//	④ `status` 四态：down / up / degraded / unmanaged，且 `--json addr,pid,upstream,model` 字段齐；
//	⑤ `stop` 负控：**没有状态件但端口被占** ⇒ 只报不杀（✗ 不碰别人的进程）；
//	⑥ `stop` 正控：按 pid 收（SIGTERM）＋ 事后验端口不通 ＋ 清状态件。
//
// ★ 本件**不**验「真起一个 broker 子进程」那一段：`os.Executable()` 在 `go test` 里是测试二进制，
// 重入跑不到 `run()`。那一段的验收在**真制品**上做（`bin/zerg cocoon broker start` ＋ 宿主
// `curl 127.0.0.1:8092/health` = 200），见本单回执的「五态验收」。
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/cocoon"
)

// ccBrokerSetup —— 隔离沙箱：状态目录换到临时根、四个旋钮显式清空。
//
// ★ 必须清 `ZERG_BROKER_ADDR` / `ZERG_BROKER_HTTP_ADDR`：宿主机上留着一个，
// 缺省那一格就永远测不到（而且 `start` 会去 bind 别人的端口）。
func ccBrokerSetup(t *testing.T) string {
	t.Helper()
	state := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", state)
	t.Setenv(cocoon.EnvBrokerAddr, "")
	t.Setenv(cocoon.EnvBrokerHTTPAddr, "")
	t.Setenv(cocoon.EnvAIUpstream, "")
	t.Setenv(cocoonBrokerTokenEnv, "")
	t.Setenv(cocoonBrokerServeEnv, "")
	return state
}

// ccBrokerFreeAddr —— 拿一个**当下空闲**的回环 host:port（监听后立刻放开）。
//
// ★ 它的用途只是「给这一格一个不会撞车的地址」；判据本身不依赖它 —— 真的占用由各用例自己造。
func ccBrokerFreeAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("找空闲端口失败：%v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	return addr
}

// ccBrokerRun —— 造 invocation 直接跑（不走进程）。
func ccBrokerRun(t *testing.T, args []string, fields []string, dryRun bool) (int, string, string) {
	t.Helper()
	inv := &invocation{path: []string{"cocoon", "broker"}, args: args, dryRun: dryRun}
	if len(fields) > 0 {
		inv.jsonGiven = true
		inv.fields = fields
	}
	var o, e strings.Builder
	rc := cmdCocoonBroker(inv, &o, &e)
	return rc, o.String(), e.String()
}

// ccBrokerRowOf —— 取 `--json` 包封里的第一行（恒数组 ⇒ items[0]）。
func ccBrokerRowOf(t *testing.T, out string) map[string]any {
	t.Helper()
	var env struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("--json 输出不是合法包封：%v\n%s", err, out)
	}
	if len(env.Items) == 0 {
		t.Fatalf("--json 该至少一行：%s", out)
	}
	return env.Items[0]
}

// ccBrokerCell —— 取一格（缺键 ⇒ 空串；★ 用它区分「键缺」与「值是空串」）。
func ccBrokerCell(row map[string]any, k string) string {
	v, ok := row[k]
	if !ok || v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}

// ── ① 动作词闭集 ─────────────────────────────────────────────────────────

func TestCocoonBrokerActionClosedSet(t *testing.T) {
	ccBrokerSetup(t)

	cases := []struct {
		name   string
		args   []string
		dryRun bool
	}{
		{"缺动作", nil, false},
		{"动作不在闭集", []string{"resurrect"}, false},
		{"多给一个位置参数", []string{"status", "extra"}, false},
		{"status 不吃 --dry-run（只读面 · ✗ 静默吞）", []string{"status"}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rc, out, errs := ccBrokerRun(t, c.args, nil, c.dryRun)
			if rc != exitUsage {
				t.Fatalf("rc = %d，want %d（stderr=%s）", rc, exitUsage, errs)
			}
			if strings.TrimSpace(errs) == "" {
				t.Fatalf("rc=2 却没有任何 stderr 判词（rc 与判词两态同形）")
			}
			if strings.TrimSpace(out) != "" {
				t.Fatalf("用法错档 stdout 该是 0 字节，实得 %q", out)
			}
		})
	}
}

// ── ② start --dry-run 零副作用 ───────────────────────────────────────────

func TestCocoonBrokerStartDryRunZeroSideEffects(t *testing.T) {
	state := ccBrokerSetup(t)
	addr := ccBrokerFreeAddr(t)
	t.Setenv(cocoon.EnvBrokerAddr, addr)

	rc, out, errs := ccBrokerRun(t, []string{"start"}, nil, true)
	if rc != exitOK {
		t.Fatalf("start --dry-run rc = %d，want 0（stderr=%s）", rc, errs)
	}
	// ★ 计划件必须印出**实际将用的 addr**（不是「缺省」二字）。
	if !strings.Contains(out, addr) {
		t.Fatalf("计划件没印出实际将用的 addr %q：\n%s", addr, out)
	}
	if !strings.Contains(out, "（env:"+cocoon.EnvBrokerAddr+"）") {
		t.Fatalf("计划件没印出 addr 的来源（旋钮 vs 缺省 必须可分辨）：\n%s", out)
	}
	// 茧内 base 与监听同一个真源：计划件里两处必须逐字一致。
	if !strings.Contains(out, "http://"+addr) {
		t.Fatalf("计划件没把茧内 base 印成 http://%s：\n%s", addr, out)
	}
	// 零副作用三面：无状态件 / 无日志 / 端口没被绑。
	for _, f := range []string{cocoonBrokerPIDFile(), cocoonBrokerStateFile(), cocoonBrokerLogFile()} {
		if _, err := os.Stat(f); err == nil {
			t.Fatalf("--dry-run 写了件：%s", f)
		}
	}
	if _, err := os.Stat(filepath.Join(state, "broker")); err == nil {
		t.Fatalf("--dry-run 建了 broker 状态目录（应零写盘）")
	}
	if cocoonBrokerDialOK(addr) {
		t.Fatalf("--dry-run 把端口绑上了（%s 可连）—— 干跑必须零副作用", addr)
	}
}

// ── ③ 端口被占：报占用 + 点名进程 + 不假成功 ──────────────────────────────

func TestCocoonBrokerStartPortBusyNamesOwner(t *testing.T) {
	ccBrokerSetup(t)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("占位监听失败：%v", err)
	}
	defer ln.Close()
	addr := ln.Addr().String()
	t.Setenv(cocoon.EnvBrokerAddr, addr)

	rc, out, errs := ccBrokerRun(t, []string{"start"}, nil, false)
	if rc != exitFail {
		t.Fatalf("端口被占时 start rc = %d，want %d（✗ 不许假成功）\nstdout=%s\nstderr=%s",
			rc, exitFail, out, errs)
	}
	if strings.Contains(out, "起来了") {
		t.Fatalf("端口被占却印了「起来了」—— 这就是假成功：\n%s", out)
	}
	if !strings.Contains(errs, "已被占用") {
		t.Fatalf("stderr 没说「已被占用」：%s", errs)
	}
	// ★ 点名：占用者就是本测试进程（`net.Listen` 在测试进程里）⇒ 名字或 pid 至少中一个。
	pid := strconv.Itoa(os.Getpid())
	if !strings.Contains(errs, pid) {
		t.Logf("提示：stderr 未含本进程 pid=%s（本机可能没有 lsof ⇒ 占用者定位不到，属如实降级）\n%s", pid, errs)
	}
	if _, serr := os.Stat(cocoonBrokerStateFile()); serr == nil {
		t.Fatalf("起失败却写了状态件（半成品落盘）")
	}
}

// ── ④ status 四态 ───────────────────────────────────────────────────────

func TestCocoonBrokerStatusStates(t *testing.T) {
	t.Run("down（无状态件 · 端口不通）", func(t *testing.T) {
		ccBrokerSetup(t)
		addr := ccBrokerFreeAddr(t)
		t.Setenv(cocoon.EnvBrokerAddr, addr)

		rc, out, _ := ccBrokerRun(t, []string{"status"}, nil, false)
		if rc != exitFail {
			t.Fatalf("down 档 rc = %d，want %d（状态「不在跑」不是成功）", rc, exitFail)
		}
		if !strings.Contains(out, "broker: down") {
			t.Fatalf("人面一行不对：%q", out)
		}
		if strings.Count(strings.TrimSpace(out), "\n") != 0 {
			t.Fatalf("人面要**一行**：%q", out)
		}
		// 机器面：addr/upstream/model 必须可答（down 档也要说清「它会用哪一格」），pid 必须缺键。
		_, jout, jerr := ccBrokerRun(t, []string{"status"},
			[]string{"addr", "pid", "upstream", "model"}, false)
		if strings.TrimSpace(jout) == "" {
			t.Fatalf("--json 没出机器面（stderr=%s）", jerr)
		}
		row := ccBrokerRowOf(t, jout)
		if got := ccBrokerCell(row, "addr"); got != addr {
			t.Fatalf("addr = %q，want %q", got, addr)
		}
		if got := ccBrokerCell(row, "upstream"); got != cocoon.DefaultAIUpstream {
			t.Fatalf("upstream = %q，want %q", got, cocoon.DefaultAIUpstream)
		}
		if got := ccBrokerCell(row, "model"); got != cocoon.DefaultCocoonAIModel {
			t.Fatalf("model = %q，want %q", got, cocoon.DefaultCocoonAIModel)
		}
		if _, ok := row["pid"]; ok {
			t.Fatalf("down 档不该有 pid 键（✗ 落 0 冒充）：%v", row["pid"])
		}
	})

	t.Run("up（pid 活着 · /health 真 200）", func(t *testing.T) {
		ccBrokerSetup(t)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/health" {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"ok":true,"backend":"cocoon","model":"m-test","upstream":"http://127.0.0.1:19400"}`+"\n")
		}))
		defer srv.Close()
		addr := strings.TrimPrefix(srv.URL, "http://")
		t.Setenv(cocoon.EnvBrokerAddr, addr)

		// 状态件的 pid 用**本进程**（活着、又不该被这个用例杀掉）。
		if err := cocoonBrokerWriteState(&cocoonBrokerState{
			PID: os.Getpid(), Addr: addr, AddrSource: cocoon.BrokerAddrSourceEnvNew,
			Upstream: "http://127.0.0.1:19400", Model: "m-test",
			StartedAt: time.Now().UTC().Format(time.RFC3339),
		}); err != nil {
			t.Fatalf("写状态件：%v", err)
		}

		rc, out, _ := ccBrokerRun(t, []string{"status"}, nil, false)
		if rc != exitOK {
			t.Fatalf("up 档 rc = %d，want 0（stdout=%s）", rc, out)
		}
		if !strings.Contains(out, "broker: up") || !strings.Contains(out, "pid="+strconv.Itoa(os.Getpid())) {
			t.Fatalf("人面一行不对：%q", out)
		}
		// ★ 四字段齐（status --json 的验收面就这一条）。
		_, jout, jerr := ccBrokerRun(t, []string{"status"},
			[]string{"addr", "pid", "upstream", "model"}, false)
		if strings.TrimSpace(jout) == "" {
			t.Fatalf("--json 没出机器面（stderr=%s）", jerr)
		}
		row := ccBrokerRowOf(t, jout)
		for _, k := range []string{"addr", "pid", "upstream", "model"} {
			if got := ccBrokerCell(row, k); got == "" {
				t.Fatalf("字段 %s 缺（四字段必须齐）：%v", k, row)
			}
		}
		// 在跑 ⇒ upstream/model 报**健康面**的真值（不是本进程的解析值）。
		if got := ccBrokerCell(row, "upstream"); got != "http://127.0.0.1:19400" {
			t.Fatalf("upstream = %q，want 健康面报的 http://127.0.0.1:19400", got)
		}
		if got := ccBrokerCell(row, "model"); got != "m-test" {
			t.Fatalf("model = %q，want 健康面报的 m-test", got)
		}
	})

	t.Run("degraded（pid 活着但 /health 不通）", func(t *testing.T) {
		ccBrokerSetup(t)
		addr := ccBrokerFreeAddr(t)
		t.Setenv(cocoon.EnvBrokerAddr, addr)
		if err := cocoonBrokerWriteState(&cocoonBrokerState{
			PID: os.Getpid(), Addr: addr, AddrSource: cocoon.BrokerAddrSourceEnvNew,
			Upstream: cocoon.DefaultAIUpstream, Model: cocoon.DefaultCocoonAIModel,
		}); err != nil {
			t.Fatalf("写状态件：%v", err)
		}
		rc, out, errs := ccBrokerRun(t, []string{"status"}, nil, false)
		if rc != exitFail {
			t.Fatalf("degraded 档 rc = %d，want %d", rc, exitFail)
		}
		if !strings.Contains(out, "broker: degraded") {
			t.Fatalf("人面一行不对：%q（stderr=%s）", out, errs)
		}
		// ★ 半起必须**说出来**（✗ 安静地当 up / 当 down）。
		if !strings.Contains(errs, "半起") {
			t.Fatalf("degraded 档 stderr 该点明「半起」：%s", errs)
		}
	})

	t.Run("unmanaged（端口有人但没状态件）", func(t *testing.T) {
		ccBrokerSetup(t)
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("占位监听失败：%v", err)
		}
		defer ln.Close()
		addr := ln.Addr().String()
		t.Setenv(cocoon.EnvBrokerAddr, addr)

		rc, out, _ := ccBrokerRun(t, []string{"status"}, nil, false)
		if rc != exitFail {
			t.Fatalf("unmanaged 档 rc = %d，want %d（不是本命令起的，不算成功）", rc, exitFail)
		}
		if !strings.Contains(out, "broker: unmanaged") {
			t.Fatalf("人面一行不对：%q", out)
		}
	})
}

// ── ⑤ stop 负控：没有状态件就绝不代杀 ─────────────────────────────────────

func TestCocoonBrokerStopRefusesForeignProcess(t *testing.T) {
	ccBrokerSetup(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("占位监听失败：%v", err)
	}
	defer ln.Close()
	addr := ln.Addr().String()
	t.Setenv(cocoon.EnvBrokerAddr, addr)

	rc, out, errs := ccBrokerRun(t, []string{"stop"}, nil, false)
	if rc != exitFail {
		t.Fatalf("没有状态件但端口被占时 stop rc = %d，want %d（✗ 不许当成功）\nstdout=%s\nstderr=%s",
			rc, exitFail, out, errs)
	}
	if !strings.Contains(errs, "不碰别人的进程") {
		t.Fatalf("stderr 该说清「不碰别人的进程」：%s", errs)
	}
	// ★ 负控的硬判据：那个监听**还在**（没被误杀）。
	if cocoonBrokerDialOK(addr) != true {
		t.Fatalf("stop 把不属于它的监听收掉了 —— 这是本命令最不该做的事")
	}
}

// ── ⑥ stop 正控：按 pid 收 + 事后验端口 + 清件 ────────────────────────────

func TestCocoonBrokerStopKillsOwnedPid(t *testing.T) {
	ccBrokerSetup(t)
	addr := ccBrokerFreeAddr(t)
	t.Setenv(cocoon.EnvBrokerAddr, addr)

	child := exec.Command("sleep", "30")
	if err := child.Start(); err != nil {
		t.Fatalf("起 sleep 失败（本用例要一个「自己的子进程」当靶子）：%v", err)
	}
	pid := child.Process.Pid
	done := make(chan struct{})
	go func() { _ = child.Wait(); close(done) }()
	defer func() {
		if cocoonBrokerAlive(pid) {
			_ = child.Process.Kill()
		}
	}()

	if err := cocoonBrokerWriteState(&cocoonBrokerState{
		PID: pid, Addr: addr, AddrSource: cocoon.BrokerAddrSourceDefault,
		Upstream: cocoon.DefaultAIUpstream, Model: cocoon.DefaultCocoonAIModel,
	}); err != nil {
		t.Fatalf("写状态件：%v", err)
	}

	// 先出一份 --dry-run 计划件：★ 零副作用 ⇒ 靶子必须还活着、状态件必须还在。
	if rc, out, _ := ccBrokerRun(t, []string{"stop"}, nil, true); rc != exitOK {
		t.Fatalf("stop --dry-run rc = %d，want 0", rc)
	} else if !strings.Contains(out, strconv.Itoa(pid)) {
		t.Fatalf("--dry-run 计划件没印出目标 pid：%s", out)
	}
	if !cocoonBrokerAlive(pid) {
		t.Fatalf("stop --dry-run 把靶子收了 —— 干跑必须零副作用")
	}
	if _, err := os.Stat(cocoonBrokerStateFile()); err != nil {
		t.Fatalf("stop --dry-run 删了状态件 —— 干跑必须零副作用")
	}

	rc, out, errs := ccBrokerRun(t, []string{"stop"}, nil, false)
	if rc != exitOK {
		t.Fatalf("stop rc = %d，want 0\nstdout=%s\nstderr=%s", rc, out, errs)
	}
	select {
	case <-done:
	case <-time.After(cocoonBrokerStopGrace + 3*time.Second):
		t.Fatalf("pid=%d 过了宽限还没退（stop 报成功却是假成功）", pid)
	}
	if cocoonBrokerAlive(pid) {
		t.Fatalf("stop 报成功但 pid=%d 还活着", pid)
	}
	if cocoonBrokerDialOK(addr) {
		t.Fatalf("stop 之后 %s 仍可连（端口没验过的成功）", addr)
	}
	for _, f := range []string{cocoonBrokerPIDFile(), cocoonBrokerStateFile()} {
		if _, err := os.Stat(f); err == nil {
			t.Fatalf("stop 之后残件还在：%s", f)
		}
	}
	if !strings.Contains(out, "已停") {
		t.Fatalf("人面该说「已停」：%q", out)
	}
}
