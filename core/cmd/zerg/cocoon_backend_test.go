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

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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
