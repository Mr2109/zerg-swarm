package main

// cocoon_backend_test.go —— 虫茧命令面的**后端选档**与**能力位驱动**两格（M1）。
//
// 四格（每一格都带负控 —— 哑化对应判据即红）：
//
//	① `TestCocoonBackendSelection`：`ZERG_COCOON_BACKEND` ∈ 空/fake/auto/nsjail/未知 的解析面。
//	   负控：`auto` 在非 linux 上必须回 fake（不许回 nsjail）；**显式** `nsjail` 在非 linux 上
//	   必须报错且错里带 `ErrBackendUnsupported`（✗ 绝不静默降级成假后端）。
//	② `TestCocoonOpenNsjailRefusedOffLinux`：非 linux 上 `open` 带 `nsjail` ⇒ 退 1、状态根零条目、
//	   人面判词点「只在 linux 上可用」，且 `--dry-run` **同拒**（干跑放行、真跑才拒 = 计划件骗人）。
//	③ `TestCocoonOpenSkipsPortsWhenNoExpose`：后端自报 `Capabilities.Expose=false` ⇒
//	   契约的 `[[端口]]` 被**跳过并说明**（stderr 点明理由）、meta.Ports 为空、
//	   **一次 Expose 都没调**、readiness 改直连回环仍能探到（真监听端口）。
//	④ `TestCocoonInspectCapabilityFace`：inspect 打印后端名 + 能力面四真（含 PeerIdentity /
//	   KillMechanism / Expose）；后端自报不出来时**如实印「能力面不可用」且 rc 仍 0**
//	   （✗ 不填一张全 false 的表冒充「后端在，只是弱」）。
//	⑤ `TestCocoonOpenPassesCocoonDir`（T3b）：`open` 把**茧目录**（真源根下的 `<id>/`）交给后端
//	   （`CocoonDirSetter`）—— 时机是「先建目录再传」、`--dry-run` 一次都不传；`close` 传同一个。
//	   负控：后端拒收 ⇒ 退 1 且**一次 Provision 都没发生**。
//	⑥ `TestCocoonInspectOverlayBit`（T3b）：inspect 人面印 `可写层 overlay=<bool>（<理由>）`
//	   （假后端 false 且带理由；真后端 true）；`--json` **键表不动**（不加 overlay 两键）。

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/cocoon"
)

// ccBeRestoreFactory —— 换掉造后端的那一刻，务必换回生产路径（否则后续格的选档全被污染）。
func ccBeRestoreFactory(t *testing.T) {
	t.Helper()
	old := cocoonBackendFactory
	t.Cleanup(func() { cocoonBackendFactory = old })
}

// ── ① 选档矩阵 ──────────────────────────────────────────────────────────

func TestCocoonBackendSelection(t *testing.T) {
	onLinux := runtime.GOOS == cocoon.PlatformLinux

	cases := []struct {
		env      string
		wantName string
		wantErr  bool
		why      string
	}{
		{"", cocoonBackendFake, false, "空 = 保留既有行为（脚手架）"},
		{"  ", cocoonBackendFake, false, "纯空白按空处理"},
		{cocoonBackendFake, cocoonBackendFake, false, "显式 fake"},
		{cocoonBackendAuto, map[bool]string{true: cocoonBackendNsjail, false: cocoonBackendFake}[onLinux], false,
			"auto：linux ⇒ nsjail，其余 ⇒ fake（换档只允许发生在这一档）"},
		{cocoonBackendNsjail, cocoonBackendNsjail, !onLinux,
			"显式 nsjail：非 linux 必须拒（✗ 不静默降级）"},
		{"vz", "vz", true, "未知后端（VZ 未接）⇒ 拒，不许当成 fake"},
	}

	for _, tc := range cases {
		t.Run(fmt.Sprintf("env=%q", tc.env), func(t *testing.T) {
			t.Setenv(cocoonBackendEnv, tc.env)

			got := cocoonBackendName()
			if got != tc.wantName {
				t.Fatalf("cocoonBackendName() = %q，want %q（%s）", got, tc.wantName, tc.why)
			}

			be, err := cocoonBackendByName(got)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("后端 %q 在本平台应当被拒，却拿到了 %T（%s）", got, be, tc.why)
				}
				if be != nil {
					t.Fatalf("被拒的后端不许交回非 nil 实现（拿到 %T）", be)
				}
				if got == cocoonBackendNsjail && !strings.Contains(err.Error(), cocoon.ErrBackendUnsupported.Error()) {
					t.Fatalf("拒执理由必须带 ErrBackendUnsupported 的原话（否则上层无法分类）：%v", err)
				}
				// ★ 负控：拒执时**不许**退回假后端（那等于把「没有隔离」冒充成「有隔离」）。
				if strings.Contains(err.Error(), "降级") && be != nil {
					t.Fatal("拒执路径上竟交回了后端实例")
				}
				return
			}
			if err != nil {
				t.Fatalf("后端 %q 应当可用，却报错：%v", got, err)
			}
			if be == nil {
				t.Fatalf("后端 %q 返回 nil 且无错（最糟的一种：调用方会在 nil 上炸）", got)
			}
			if name := be.(interface{ Name() string }).Name(); name != got {
				t.Fatalf("后端署名 %q 与选档名 %q 不符（输出里的 backend= 会撒谎）", name, got)
			}
		})
	}
}

// ── ② 非 linux 上显式要 nsjail：零副作用拒执（dry-run 同拒）────────────────

func TestCocoonOpenNsjailRefusedOffLinux(t *testing.T) {
	if runtime.GOOS == cocoon.PlatformLinux {
		t.Skip("本格只验非 linux 的拒执面（linux 上 nsjail 是真后端）")
	}
	proj, root := ccLifeSetup(t)
	t.Setenv(cocoonBackendEnv, cocoonBackendNsjail)

	rc, out, errs := ccLifeRun(t, cmdCocoonOpen, []string{"cocoon", "open"}, []string{proj},
		[]string{"id", "phase"}, nil)
	if rc != exitFail {
		t.Fatalf("显式 nsjail 在 %s 上应当退 1，得 rc=%d · stderr=%s", runtime.GOOS, rc, errs)
	}
	if out != "" {
		t.Fatalf("被拒的 open 不该往 stdout 写东西（会被人当成功读数）：%q", out)
	}
	for _, want := range []string{cocoonBackendNsjail, cocoon.PlatformLinux, "未接"} {
		if !strings.Contains(errs, want) {
			t.Errorf("stderr 没含 %q（判词要能让人知道为什么拒）：%s", want, errs)
		}
	}
	// 零副作用：状态根一个条目都不许多（meta / 目录 / 日志都不许建）。
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		if entries, rerr := os.ReadDir(root); rerr == nil && len(entries) != 0 {
			t.Fatalf("被拒的 open 在状态根里留了条目（应零写入）：%v", entries)
		}
	}

	// `--dry-run` 同拒：干跑放行、真跑才拒 ⇒ 计划件就是在骗人。
	rc, out, _ = ccLifeRun(t, cmdCocoonOpen, []string{"cocoon", "open"}, []string{proj}, nil,
		map[string]string{"--dry-run": "true"})
	if rc != exitFail {
		t.Fatalf("后端不可用时 `--dry-run` 也须退 1，得 %d（stdout=%q）", rc, out)
	}
	if strings.Contains(out, "计划件") {
		t.Fatalf("后端不可用却出了计划件（那会让人以为这条链能跑）：%s", out)
	}
}

// ── ③ 能力位驱动端口面：Expose=false ⇒ 跳过 [[端口]] + readiness 直连 ────────

// ccNoExposeBackend — 演「有隔离、但不做端口映射」的后端（就是 linux/nsjail 的能力形状）。
//
// ★ 它把 Exec 改成「挂到 ctx 取消」：假后端那套「立刻 RC=0」会让编排层在就绪前就看到退出，
// 从而永远打不到 readiness 那条路（本格要验的正是那条路上的 ProbeAddr）。
type ccNoExposeBackend struct {
	*cocoon.FakeBackend
	exposeCalls int
}

func (b *ccNoExposeBackend) Capabilities() (cocoon.Capabilities, error) {
	return cocoon.Capabilities{
		Platform:            cocoon.HostPlatform(),
		Isolation:           "stub-noexpose",
		PeerIdentity:        cocoon.PeerIdentityStrong,
		PeerMechanism:       "stub（本格不验身份机制）",
		ProcessTreeKill:     true,
		KillMechanism:       "stub（本格不验回收机制）",
		SymlinkNoFollow:     true,
		SymlinkMechanism:    "stub",
		CrossMountGuard:     true,
		CrossMountMechanism: "stub",
		// ★ 被测的那一位：不映射端口。
		Expose:          false,
		ExposeMechanism: "stub：无 DNAT（茧与调用方同网络栈，茧内端口即宿主端口）",
	}, nil
}

func (b *ccNoExposeBackend) Expose(context.Context, cocoon.Instance, int, string) (int, error) {
	b.exposeCalls++
	return 0, fmt.Errorf("%w: stub 不映射端口", cocoon.ErrBackendUnsupported)
}

func (b *ccNoExposeBackend) Exec(ctx context.Context, _ cocoon.Instance, _ []string, _ cocoon.ExecOpts) (cocoon.ExecResult, error) {
	<-ctx.Done() // 长驻：一直挂到被停
	return cocoon.ExecResult{RC: 0}, nil
}

func TestCocoonOpenSkipsPortsWhenNoExpose(t *testing.T) {
	proj, root := ccLifeSetup(t)

	// 真监听一个回环端口当作「茧内服务」：readiness 探测要能真连上它。
	ln, err := net.Listen("tcp", cocoon.DefaultBind+":0")
	if err != nil {
		t.Fatalf("起监听失败: %v", err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port

	spec := strings.Replace(ccLifeSpec, `entry = ["uvicorn", "app:api"]`,
		fmt.Sprintf("entry = [\"/bin/sh\", \"-c\", \"sleep 600\"]\nreadiness = { port = %d, timeout = \"5s\" }", port), 1)
	spec = strings.Replace(spec, "inner = 8000", fmt.Sprintf("inner = %d", port), 1)
	if spec == ccLifeSpec || !strings.Contains(spec, "readiness = {") {
		t.Fatalf("夹具没改到（ccLifeSpec 形状变了？）:\n%s", spec)
	}
	if err := os.WriteFile(filepath.Join(proj, cocoon.SpecFileName), []byte(spec), 0o644); err != nil {
		t.Fatal(err)
	}

	stub := &ccNoExposeBackend{FakeBackend: cocoon.NewFakeBackend()}
	ccBeRestoreFactory(t)
	cocoonBackendFactory = func(string) (cocoon.Backend, error) { return stub, nil }
	t.Setenv(cocoonBackendEnv, cocoonBackendNsjail) // 名字任选：工厂已被换掉

	rc, out, errs := ccLifeRun(t, cmdCocoonOpen, []string{"cocoon", "open"}, []string{proj},
		[]string{"id", "phase", "ports", "backend"}, nil)
	if rc != exitOK {
		t.Fatalf("open rc=%d 期望 0 · stderr=%s", rc, errs)
	}
	rows := ccLifeRows(t, out)
	if got := ccLifeCell(rows[0], "phase"); got != string(cocoon.PhaseRunning) {
		t.Fatalf("readiness 直连应当探到（phase=%q 期望 running）：stderr=%s", got, errs)
	}
	// ★ 判据一：一次 Expose 都没调（端口面由能力位决定，不是「照着契约无脑调」）。
	if stub.exposeCalls != 0 {
		t.Fatalf("后端自报 Expose=false，却仍调了 %d 次 Expose（能力位形同虚设）", stub.exposeCalls)
	}
	// ★ 判据二：跳过要**说出来**（不是静默丢掉）。
	if !strings.Contains(errs, "不映射端口") {
		t.Fatalf("stderr 没说明「不映射端口 ⇒ 跳过 [[端口]]」：%s", errs)
	}
	// ★ 判据三：meta.Ports 为空，且机器面上是「—」（三值纪律：没有就说没有）。
	id := ccLifeCell(rows[0], "id")
	m, err := cocoon.Load(cocoon.MetaPath(filepath.Join(root, id)))
	if err != nil {
		t.Fatalf("meta 读不到：%v", err)
	}
	if len(m.Ports) != 0 {
		t.Fatalf("后端不映射端口，meta.Ports 却非空：%v", m.Ports)
	}
	if got := ccLifeCell(rows[0], "ports"); got != "—" {
		t.Fatalf("ports=%q 期望 —（不许印一个假的宿主端口）", got)
	}

	// 收尾（走 close，验同一套实例/句柄面）：进程（这里是 stub 的 ctx 等待）要被停掉。
	if rc, _, errs := ccLifeRun(t, cmdCocoonClose, []string{"cocoon", "close"}, []string{id}, nil, nil); rc != exitOK {
		t.Fatalf("close rc=%d · %s", rc, errs)
	}
	if ph, ok := ccLifeMetaPhase(t, root, id); !ok || ph != cocoon.PhaseGone {
		t.Fatalf("close 后 phase=%q 期望 gone", ph)
	}
}

// ── ④ inspect：后端名 + 能力面（四真 / PeerIdentity / KillMechanism / Expose）──

func TestCocoonInspectCapabilityFace(t *testing.T) {
	_, root := ccLifeSetup(t)

	// 假后端的茧：能力面必须照实印 none（✗ 不冒充 strong），且 Expose 位要看得见。
	id := "cc-caps-1"
	dir := filepath.Join(root, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	m := cocoon.New(id, "", "t")
	if err := m.SetPhase(cocoon.PhaseRunning); err != nil {
		t.Fatal(err)
	}
	m.Backend = cocoonBackendFake
	m.BackendInstance = "fake-1"
	if err := cocoon.Save(cocoon.MetaPath(dir), m); err != nil {
		t.Fatal(err)
	}

	rc, out, errs := ccLifeRun(t, cmdCocoonInspect, []string{"cocoon", "inspect"}, []string{id},
		[]string{"backend", "backend_instance", "platform", "isolation", "peer_identity",
			"peer_mechanism", "process_tree_kill", "kill_mechanism", "symlink_nofollow",
			"cross_mount_guard", "expose", "expose_mechanism", "caps_error"}, nil)
	if rc != exitOK {
		t.Fatalf("inspect rc=%d · %s", rc, errs)
	}
	row := ccLifeRows(t, out)[0]
	if got := ccLifeCell(row, "backend"); got != cocoonBackendFake {
		t.Fatalf("backend=%q 期望 %q（后端名要如实署名）", got, cocoonBackendFake)
	}
	if got := ccLifeCell(row, "backend_instance"); got != "fake-1" {
		t.Fatalf("backend_instance=%q 期望 fake-1（后端签发的句柄要原样印）", got)
	}
	if got := ccLifeCell(row, "peer_identity"); got != string(cocoon.PeerIdentityNone) {
		t.Fatalf("假后端 peer_identity=%q 期望 none（✗ 不许冒充 strong）", got)
	}
	if got := ccLifeCell(row, "kill_mechanism"); got == "" || got == "—" {
		t.Fatalf("kill_mechanism=%q 不许空（「没有回收机制」也要写清）", got)
	}
	if got := ccLifeCell(row, "expose"); got != "true" {
		t.Fatalf("假后端 expose=%q 期望 true（它真分配宿主端口号）", got)
	}
	if got := ccLifeCell(row, "expose_mechanism"); got == "" || got == "—" {
		t.Fatalf("expose_mechanism=%q 不许空", got)
	}
	if got := ccLifeCell(row, "caps_error"); got != "—" {
		t.Fatalf("能力面读得出时 caps_error 应为 —，得 %q", got)
	}
	// 人面也要印（四真 + 端口面）。
	_, human, _ := ccLifeRun(t, cmdCocoonInspect, []string{"cocoon", "inspect"}, []string{id}, nil, nil)
	for _, want := range []string{"对端身份=none", "整树回收=", "不跟链接=", "不跨挂载=", "端口面"} {
		if !strings.Contains(human, want) {
			t.Errorf("人面没印 %q：\n%s", want, human)
		}
	}

	// 负控：meta 说它是 nsjail 的茧、而本平台给不出 nsjail 能力 ⇒ 如实印「不可用」，
	// **rc 仍 0**（观察面不拒），且三真位印 —（✗ 不许填一张「全 false 表」冒充弱后端）。
	if runtime.GOOS == cocoon.PlatformLinux {
		t.Skip("linux 上 nsjail 能力面读得出来，本负控只验非 linux")
	}
	id2 := "cc-caps-2"
	dir2 := filepath.Join(root, id2)
	if err := os.MkdirAll(dir2, 0o755); err != nil {
		t.Fatal(err)
	}
	m2 := cocoon.New(id2, "", "t")
	if err := m2.SetPhase(cocoon.PhaseRunning); err != nil {
		t.Fatal(err)
	}
	m2.Backend = cocoonBackendNsjail
	if err := cocoon.Save(cocoon.MetaPath(dir2), m2); err != nil {
		t.Fatal(err)
	}
	rc, out, errs = ccLifeRun(t, cmdCocoonInspect, []string{"cocoon", "inspect"}, []string{id2},
		[]string{"backend", "peer_identity", "kill_mechanism", "caps_error"}, nil)
	if rc != exitOK {
		t.Fatalf("能力面不可用时 inspect 仍须 rc=0（观察面不拒），得 %d · %s", rc, errs)
	}
	row = ccLifeRows(t, out)[0]
	if got := ccLifeCell(row, "backend"); got != cocoonBackendNsjail {
		t.Fatalf("backend=%q 期望 %q（按 meta 那一格署名，不按当前 env 猜）", got, cocoonBackendNsjail)
	}
	if got := ccLifeCell(row, "caps_error"); got == "" || got == "—" {
		t.Fatalf("能力面不可用时 caps_error 必须写清原因，得 %q", got)
	}
	if got := ccLifeCell(row, "peer_identity"); got != "—" {
		t.Fatalf("能力面不可用时 peer_identity 应印 —（✗ 不许编一个 none/false 的表），得 %q", got)
	}
	_, human, _ = ccLifeRun(t, cmdCocoonInspect, []string{"cocoon", "inspect"}, []string{id2}, nil, nil)
	if !strings.Contains(human, "能力面不可用") {
		t.Fatalf("人面没说明能力面不可用：\n%s", human)
	}
}

// ── ⑤ 命令面把茧目录交给后端（rootfs 的落点 · T3b）──────────────────────────

// ccDirRecorder — 演「要茧目录」的后端（nsjail 那一档的形状），三件事：
//
//	① 记下命令面交来的目录（判「传没传、传的是哪个」）；
//	② **当场**校验该目录已存在（命令面必须「先建目录再传」—— 本桩不替它建）；
//	③ 数 Provision（判「拒收之后有没有偷偷起实例」）。
type ccDirRecorder struct {
	*cocoon.FakeBackend
	mu         sync.Mutex
	dirs       []string
	provisions int
	reject     error // 非 nil ⇒ 演「后端拒收这个目录」
}

func (b *ccDirRecorder) SetCocoonDir(dir string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.reject != nil {
		return b.reject
	}
	fi, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("命令面给的茧目录 %s 读不到（时序错：要先建目录再传）: %w", dir, err)
	}
	if !fi.IsDir() {
		return fmt.Errorf("命令面给的茧目录 %s 不是目录", dir)
	}
	b.dirs = append(b.dirs, dir)
	return nil
}

func (b *ccDirRecorder) Provision(ctx context.Context, spec *cocoon.Spec) (cocoon.Instance, error) {
	b.mu.Lock()
	b.provisions++
	b.mu.Unlock()
	return b.FakeBackend.Provision(ctx, spec)
}

func (b *ccDirRecorder) calls() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]string(nil), b.dirs...)
}

func (b *ccDirRecorder) provisionCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.provisions
}

func TestCocoonOpenPassesCocoonDir(t *testing.T) {
	proj, root := ccLifeSetup(t)
	stub := &ccDirRecorder{FakeBackend: cocoon.NewFakeBackend()}
	ccBeRestoreFactory(t)
	cocoonBackendFactory = func(string) (cocoon.Backend, error) { return stub, nil }
	t.Setenv(cocoonBackendEnv, cocoonBackendFake) // 名字任选：工厂已被换掉

	// ★ 负控一：`--dry-run` 一次都不传，也不建任何目录（干跑承诺零副作用）。
	rc, out, errs := ccLifeRun(t, cmdCocoonOpen, []string{"cocoon", "open"}, []string{proj}, nil,
		map[string]string{"--dry-run": "true"})
	if rc != exitOK {
		t.Fatalf("--dry-run rc=%d 期望 0 · stderr=%s", rc, errs)
	}
	if got := stub.calls(); len(got) != 0 {
		t.Fatalf("--dry-run 竟把茧目录交给了后端 %v（干跑必须零副作用）", got)
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("--dry-run 竟建出了状态根 %s（stat err=%v）", root, err)
	}

	// 真跑：目录必须 = 真源根下的 <id>/（**茧目录本身**，不是它的父目录）。
	rc, out, errs = ccLifeRun(t, cmdCocoonOpen, []string{"cocoon", "open"}, []string{proj},
		[]string{"id", "phase", "dir"}, nil)
	if rc != exitOK {
		t.Fatalf("open rc=%d 期望 0 · stderr=%s", rc, errs)
	}
	id := ccLifeCell(ccLifeRows(t, out)[0], "id")
	if id == "" {
		t.Fatalf("open 没给 id：%s", out)
	}
	want := filepath.Join(root, id)
	got := stub.calls()
	if len(got) != 1 {
		t.Fatalf("open 应当把茧目录交给后端恰一次，得到 %d 次：%v", len(got), got)
	}
	if got[0] != want {
		t.Fatalf("交给后端的茧目录=%q，want %q（真源根下的 <id>/）", got[0], want)
	}
	if filepath.Dir(got[0]) != root {
		t.Fatalf("茧目录的父目录应当是状态根 %q，得到 %q（传成状态根就落错层）", root, filepath.Dir(got[0]))
	}
	// 它必须真是**这个茧的目录**（meta 在里面），否则「交给后端的」就是别的路径。
	if _, err := os.Stat(filepath.Join(got[0], "meta.json")); err != nil {
		t.Fatalf("%s 里没有 meta.json（交给后端的不是茧目录）：%v", got[0], err)
	}
	if stub.provisionCount() != 1 {
		t.Fatalf("Provision 次数=%d，want 1", stub.provisionCount())
	}

	// `close` 也要传同一个目录：跨进程收尾靠后端用 BaseDir 算茧根（否则实例根收不掉）。
	if rc, _, errs := ccLifeRun(t, cmdCocoonClose, []string{"cocoon", "close"}, []string{id}, nil, nil); rc != exitOK {
		t.Fatalf("close rc=%d · %s", rc, errs)
	}
	got = stub.calls()
	if len(got) != 2 || got[1] != want {
		t.Fatalf("close 应当把同一个茧目录再交一次（%q），得到 %v", want, got)
	}

	// ★ 负控二：后端拒收 ⇒ 退 1、**一次 Provision 都没发生**（不许「先 Provision 再看」）。
	reject := &ccDirRecorder{FakeBackend: cocoon.NewFakeBackend()}
	reject.reject = fmt.Errorf("stub：不接受茧目录（本格验拒收面）")
	cocoonBackendFactory = func(string) (cocoon.Backend, error) { return reject, nil }
	rc, out, errs = ccLifeRun(t, cmdCocoonOpen, []string{"cocoon", "open"}, []string{proj},
		[]string{"id", "phase"}, nil)
	if rc != exitFail {
		t.Fatalf("后端拒收茧目录时 open 应当退 1，得 rc=%d · stdout=%q · stderr=%s", rc, out, errs)
	}
	if reject.provisionCount() != 0 {
		t.Fatalf("后端拒收之后仍 Provision 了 %d 次（顺序错：应当先交目录再 Provision）",
			reject.provisionCount())
	}
	if !strings.Contains(errs, "后端不接受茧目录") {
		t.Fatalf("stderr 没点明「后端不接受茧目录」：%s", errs)
	}
}

// ── ⑥ inspect 印可写层（overlay）位 · `--json` 键表不动（T3b）────────────────

// ccOverlayBackend — 演「能叠可写层」的后端（能力表全绿的那一档），只为正控那条人面。
type ccOverlayBackend struct{ *cocoon.FakeBackend }

func (b *ccOverlayBackend) Capabilities() (cocoon.Capabilities, error) {
	c := cocoon.Capabilities{
		Platform:            cocoon.HostPlatform(),
		Isolation:           "stub-overlay",
		PeerIdentity:        cocoon.PeerIdentityStrong,
		PeerMechanism:       "stub（本格不验身份机制）",
		ProcessTreeKill:     true,
		KillMechanism:       "stub（本格不验回收机制）",
		SymlinkNoFollow:     true,
		SymlinkMechanism:    "stub",
		CrossMountGuard:     true,
		CrossMountMechanism: "stub",
		Expose:              true,
		ExposeMechanism:     "stub（本格不验端口面）",
		Overlay:             true,
		OverlayMechanism:    "overlayfs（stub：lower=rootfs · upper 在茧目录）",
	}
	if err := c.Validate(); err != nil {
		return cocoon.Capabilities{}, err
	}
	return c, nil
}

func TestCocoonInspectOverlayBit(t *testing.T) {
	_, root := ccLifeSetup(t)

	// 假后端的茧：可写层 = false，且**必须带理由**（false 不许是一个裸位）。
	id := "cc-overlay-1"
	dir := filepath.Join(root, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	m := cocoon.New(id, "", "t")
	if err := m.SetPhase(cocoon.PhaseRunning); err != nil {
		t.Fatal(err)
	}
	m.Backend = cocoonBackendFake
	if err := cocoon.Save(cocoon.MetaPath(dir), m); err != nil {
		t.Fatal(err)
	}

	_, human, errs := ccLifeRun(t, cmdCocoonInspect, []string{"cocoon", "inspect"}, []string{id}, nil, nil)
	if !strings.Contains(human, "可写层") {
		t.Fatalf("人面没印「可写层」这一行：\n%s", human)
	}
	if !strings.Contains(human, "overlay=false") {
		t.Fatalf("假后端的 overlay 位应当印 false：\n%s", human)
	}
	if !strings.Contains(human, "假后端") {
		t.Fatalf("overlay=false 必须带理由（OverlayMechanism），人面里没看到：\n%s · stderr=%s", human, errs)
	}

	// ★ 机器面**键表不动**：inspect 的 --json 字段里不许冒出 overlay 两键
	//   （加键要同批刷契约基线与文档，本单不做 ⇒ 这里把「不动」钉成判据）。
	if f := strings.Join(cocoonInspectFields, ","); strings.Contains(f, "overlay") {
		t.Fatalf("cocoonInspectFields 被加了 overlay 键：%v（本单承诺 --json 键表不动）", cocoonInspectFields)
	}
	rc, out, errs := ccLifeRun(t, cmdCocoonInspect, []string{"cocoon", "inspect"}, []string{id},
		cocoonInspectFields, nil)
	if rc != exitOK {
		t.Fatalf("inspect rc=%d · %s", rc, errs)
	}
	row := ccLifeRows(t, out)[0]
	for _, k := range []string{"overlay", "overlay_mechanism"} {
		if _, ok := row[k]; ok {
			t.Fatalf("--json 行里冒出了 %q 键（键表约定不动）：%v", k, row)
		}
	}

	// 正控：后端自报 overlay=true ⇒ 同一行印 true + 机制（证明这一格不是写死的 false）。
	oval := &ccOverlayBackend{FakeBackend: cocoon.NewFakeBackend()}
	ccBeRestoreFactory(t)
	cocoonBackendFactory = func(string) (cocoon.Backend, error) { return oval, nil }
	id2 := "cc-overlay-2"
	dir2 := filepath.Join(root, id2)
	if err := os.MkdirAll(dir2, 0o755); err != nil {
		t.Fatal(err)
	}
	m2 := cocoon.New(id2, "", "t")
	if err := m2.SetPhase(cocoon.PhaseRunning); err != nil {
		t.Fatal(err)
	}
	m2.Backend = "stub-overlay"
	if err := cocoon.Save(cocoon.MetaPath(dir2), m2); err != nil {
		t.Fatal(err)
	}
	_, human, errs = ccLifeRun(t, cmdCocoonInspect, []string{"cocoon", "inspect"}, []string{id2}, nil, nil)
	if !strings.Contains(human, "overlay=true") {
		t.Fatalf("后端报 Overlay=true 时人面应当印 true：\n%s · stderr=%s", human, errs)
	}
	if !strings.Contains(human, "overlayfs（stub") {
		t.Fatalf("overlay=true 时人面应当印机制原文：\n%s", human)
	}
}

// ── ⑦ 后台档（T3c）：open 必须走「脱钩起服务 + 验活」这条路 ────────────────
//
// 这一组的靶子是 T3b 的实测假绿：`cocoon open` 报成功、盘上 pid:0、茧里什么都没有
// （服务 spawn 在 CLI 自己的 goroutine 里，main 一返回就被带走）。四格：
//
//	① 真跑档：open 必须把 `Detach=true` 传给后端（否则服务又被 CLI 带走）；
//	   且 `--json` 的 logs 格按三值纪律取值（假后端不开日志 ⇒ `—`）。
//	② 正控：后端自报 pid 且探针说「在」⇒ open 成功、meta 记 running + 真 pid。
//	③ 负控甲：后端有判活面却没给 pid ⇒ 退 1、meta failed（✗ 不许按成功报）。
//	④ 负控乙：pid 已退出（rc=7）⇒ 退 1、stderr 点明「已退出（RC=7）」、meta 不写 running。

// ccDetachBackend — 演**真后台档**后端：Exec 立刻签出 pid，判活面按探针说话。
type ccDetachBackend struct {
	*cocoon.FakeBackend
	mu      sync.Mutex
	pid     int  // 0 = 不签 pid（演「后端嘴上支持、其实没给」）
	exited  bool // 判活面的读数
	exitRC  int
	polls   int
	detach  bool
	execOps cocoon.ExecOpts
}

func (b *ccDetachBackend) Exec(ctx context.Context, inst cocoon.Instance, argv []string, opts cocoon.ExecOpts) (cocoon.ExecResult, error) {
	b.mu.Lock()
	b.detach, b.execOps = opts.Detach, opts
	pid := b.pid
	b.mu.Unlock()
	if !opts.Detach || pid == 0 {
		// 不走后台档 / 不签 pid：保持假后端原语义（立刻 RC=0 的「一次性进程」）。
		return b.FakeBackend.Exec(ctx, inst, argv, opts)
	}
	return cocoon.ExecResult{RC: 0, PID: pid}, nil
}

func (b *ccDetachBackend) InstancePIDs(cocoon.Instance) []int {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.pid == 0 {
		return nil
	}
	return []int{b.pid}
}

func (b *ccDetachBackend) DetachedStatus(pid int) (bool, bool, cocoon.ExecResult) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.polls++
	if pid != b.pid {
		return false, true, cocoon.ExecResult{RC: 1}
	}
	if b.exited {
		return false, true, cocoon.ExecResult{RC: b.exitRC}
	}
	return true, false, cocoon.ExecResult{}
}

func (b *ccDetachBackend) setExited(rc int) {
	b.mu.Lock()
	b.exited, b.exitRC = true, rc
	b.mu.Unlock()
}

func ccDetachSetup(t *testing.T, stub *ccDetachBackend) (proj, root string) {
	t.Helper()
	proj, root = ccLifeSetup(t)
	ccBeRestoreFactory(t)
	cocoonBackendFactory = func(string) (cocoon.Backend, error) { return stub, nil }
	return proj, root
}

// ① 真跑档：Detach 真的传下去了；logs 格按三值纪律。
func TestCocoonOpenUsesDetachedExec(t *testing.T) {
	be := cocoon.NewFakeBackend()
	proj, root := ccLifeSetup(t)
	ccBeRestoreFactory(t)
	cocoonBackendFactory = func(string) (cocoon.Backend, error) { return be, nil }

	rc, out, errs := ccLifeRun(t, cmdCocoonOpen, []string{"cocoon", "open"}, []string{proj},
		[]string{"id", "phase", "logs"}, nil)
	if rc != exitOK {
		t.Fatalf("open rc=%d 期望 0（stderr=%s）", rc, errs)
	}
	id := ccLifeCell(ccLifeRows(t, out)[0], "id")
	// 假后端：拿它的 LastExec 读这次 Exec 的入参（真后端走的是同一个 ExecOpts）。
	// 实例 id 由后端自己签发（fake-N）⇒ 从 meta.backend_instance 原样取回，✗ 不自己拼。
	m, merr := cocoon.Load(cocoon.MetaPath(filepath.Join(root, id)))
	if merr != nil {
		t.Fatalf("读 meta: %v", merr)
	}
	last, ok := be.LastExec(cocoon.Instance{ID: m.BackendInstance})
	if !ok {
		t.Fatalf("后端一次 Exec 都没拿到（服务没起？instance=%q）", m.BackendInstance)
	}
	if !last.Opts.Detach {
		t.Fatalf("open 的服务 Exec 必须带 Detach=true（否则 CLI 一退服务就被带走 · T3b 假绿形态）：%+v", last.Opts)
	}
	if !strings.HasSuffix(last.Opts.LogPath, string(filepath.Separator)+"logs"+string(filepath.Separator)+"api.log") {
		t.Fatalf("日志必须落 <茧目录>/logs/api.log，得到 %q", last.Opts.LogPath)
	}
	// 假后端不起进程 ⇒ logs/ 不会被建出来 ⇒ 机器面必须说「没有」（不是给个不存在的路径）。
	if got := ccLifeCell(ccLifeRows(t, out)[0], "logs"); got != cocoonDash {
		t.Fatalf("假后端的 logs 格该是 %q（三值纪律），得到 %q", cocoonDash, got)
	}
}

// ② 正控：有 pid 且在 ⇒ 成功，meta 记 running + 真 pid。
func TestCocoonOpenDetachedAliveSucceeds(t *testing.T) {
	stub := &ccDetachBackend{FakeBackend: cocoon.NewFakeBackend(), pid: 4242}
	proj, root := ccDetachSetup(t, stub)

	rc, out, errs := ccLifeRun(t, cmdCocoonOpen, []string{"cocoon", "open"}, []string{proj},
		[]string{"id", "phase", "pid"}, nil)
	if rc != exitOK {
		t.Fatalf("pid 在跑时 open 该成功，得 rc=%d · stderr=%s", rc, errs)
	}
	row := ccLifeRows(t, out)[0]
	if got := ccLifeCell(row, "phase"); got != string(cocoon.PhaseRunning) {
		t.Fatalf("phase=%q 期望 running", got)
	}
	if got := ccLifeCell(row, "pid"); got != "4242" {
		t.Fatalf("pid=%q 期望 4242（后端签出的真 pid 必须落盘）", got)
	}
	m, err := cocoon.Load(cocoon.MetaPath(filepath.Join(root, ccLifeCell(row, "id"))))
	if err != nil {
		t.Fatalf("读 meta: %v", err)
	}
	if m.PID != 4242 || len(m.BackendPIDs) != 1 || m.BackendPIDs[0] != 4242 {
		t.Fatalf("meta 该记 pid=4242 与 backend_pids=[4242]，得到 %d / %v", m.PID, m.BackendPIDs)
	}
	stub.mu.Lock()
	polls, detach := stub.polls, stub.detach
	stub.mu.Unlock()
	if !detach {
		t.Fatalf("detach 没传下去")
	}
	if polls == 0 {
		t.Fatalf("一次判活探针都没打（验活形同虚设）")
	}
}

// ③ 负控甲：有判活面却没 pid ⇒ 退 1、meta failed。
func TestCocoonOpenDetachedNeedsPid(t *testing.T) {
	stub := &ccDetachBackend{FakeBackend: cocoon.NewFakeBackend(), pid: 0}
	proj, root := ccDetachSetup(t, stub)

	rc, out, errs := ccLifeRun(t, cmdCocoonOpen, []string{"cocoon", "open"}, []string{proj},
		[]string{"id", "phase"}, nil)
	if rc != exitFail {
		t.Fatalf("后端没给 pid 时 open 必须退 1，得 rc=%d（stdout=%q）", rc, out)
	}
	if out != "" {
		t.Fatalf("失败的 open 不许往 stdout 写东西：%q", out)
	}
	if !strings.Contains(errs, "没有给出任何 pid") {
		t.Fatalf("stderr 该点明「没有给出任何 pid」：%s", errs)
	}
	ph, ok := ccLifeMetaPhase(t, root, firstCocoonID(t, root))
	if !ok || ph == cocoon.PhaseRunning {
		t.Fatalf("meta 不该是 running（phase=%q ok=%v）", ph, ok)
	}
}

// ④ 负控乙：pid 已退 ⇒ 退 1、点明退出码、meta 不写 running。
func TestCocoonOpenDetachedExitedFails(t *testing.T) {
	stub := &ccDetachBackend{FakeBackend: cocoon.NewFakeBackend(), pid: 4242, exited: true, exitRC: 7}
	proj, root := ccDetachSetup(t, stub)

	rc, out, errs := ccLifeRun(t, cmdCocoonOpen, []string{"cocoon", "open"}, []string{proj},
		[]string{"id", "phase"}, nil)
	if rc != exitFail {
		t.Fatalf("进程已退时 open 必须退 1，得 rc=%d（stdout=%q）", rc, out)
	}
	for _, want := range []string{"已退出", "RC=7"} {
		if !strings.Contains(errs, want) {
			t.Fatalf("stderr 该含 %q（把退出码说出来）：%s", want, errs)
		}
	}
	id := firstCocoonID(t, root)
	ph, ok := ccLifeMetaPhase(t, root, id)
	if !ok {
		t.Fatalf("meta 该在（路径 %s）", cocoon.MetaPath(filepath.Join(root, id)))
	}
	if ph == cocoon.PhaseRunning {
		t.Fatalf("进程已退却把 meta 写成 running（这正是本单要消灭的假绿）")
	}
	m, _ := cocoon.Load(cocoon.MetaPath(filepath.Join(root, id)))
	if m != nil && m.PID != 0 {
		t.Fatalf("失败的茧不该记 pid（pid=%d）", m.PID)
	}
}

// firstCocoonID — 状态根下唯一那个茧目录名（失败路径只留一个）。
func firstCocoonID(t *testing.T, root string) string {
	t.Helper()
	ents, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("读状态根 %s: %v", root, err)
	}
	var ids []string
	for _, e := range ents {
		if e.IsDir() {
			ids = append(ids, e.Name())
		}
	}
	if len(ids) != 1 {
		t.Fatalf("状态根下该恰有 1 个茧目录，得到 %v", ids)
	}
	return ids[0]
}
