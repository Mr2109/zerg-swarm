package backend

// hatch_path_test.go —— 批 2 验收：孵化器接入（开关 ZERG_HATCH，默认关）。
//
// 钉住的语义（任一条被改坏，本文件先红）：
//   - **默认关**：开关未设/空/"0" ⇒ hatchEnabled=false，doStart 走裸 exec 路径，孵化器一次都不被调用；
//   - 开关开 + 无实测档案 ⇒ 507 拒孵，且**不起任何单元**、不登记驻留（账本干净）；
//   - 开关开 + 两账读不到 ⇒ 507 拒孵（fail-closed，不拿估值凑）；
//   - 开关开 + 两账不够 ⇒ 507（insufficient memory）+ 写明差额；同样不起单元；
//   - 开关开 + 有档案 + 两账够 ⇒ 走孵化路径：单元名记进 subproc.Unit、就绪仍走既有判据、
//     孵化后调 VerifyEnclosure（拿不到 pid 记为「未核验」）；
//   - 收卵：sp.Unit 非空 ⇒ 走 Hatcher.Collect（Stop / ReapIdle 两条路都验）；空 ⇒ 走既有句柄路径。
//
// 假孵化器（fakeHatcher）替代真实 systemd：开发机上没有 systemd/bwrap，真实依赖只在 X3 上存在；
// 假孵化器按 spec 里的 --port 起一个只会应答 /health 与 /v1/chat/completions 的本地假引擎，
// 这样「就绪判据」（waitForReady + 功能预检）走的是**真实代码**，不是替身。

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Mr2109/zerg-swarm/agent/internal/hatch"
	"github.com/Mr2109/zerg-swarm/agent/internal/registry"
)

// ── 开关：默认关 ────────────────────────────────────────────────────────────

// TestHatchEnabled_DefaultOff 开关判据：只有显式真值才开；读不到/空/其它值一律关。
func TestHatchEnabled_DefaultOff(t *testing.T) {
	if hatchEnabledFromEnv(nil) {
		t.Fatal("getenv 为 nil（读不到环境）时必须按「关」处理")
	}
	cases := map[string]bool{
		"":      false,
		"0":     false,
		"false": false,
		"off":   false,
		"no":    false,
		"2":     false,
		"  ":    false,
		"1":     true,
		"true":  true,
		"TRUE":  true,
		" 1 ":   true,
		"yes":   true,
		"on":    true,
	}
	for v, want := range cases {
		got := hatchEnabledFromEnv(func(string) string { return v })
		if got != want {
			t.Errorf("%s=%q 应为 %v，实得 %v", EnvHatch, v, want, got)
		}
	}
	// 真实进程环境缺省：未设 ⇒ 关
	t.Setenv(EnvHatch, "")
	if hatchEnabled() {
		t.Fatalf("%s 为空时必须是关（默认关是硬要求）", EnvHatch)
	}
}

// ── 开关关：行为与现状一致 ──────────────────────────────────────────────────

// TestHatchOff_BehavesLikeBefore 开关关 ⇒ 即便档案齐备、即便是注入的假孵化器可用，
// doStart 也绝不走孵化路径：孵化器零调用，走的是既有裸 exec 路径（此处二进制不存在 ⇒ 500）。
func TestHatchOff_BehavesLikeBefore(t *testing.T) {
	t.Setenv(EnvHatch, "") // 显式关
	t.Setenv("ZERG_EGG_PROFILE_DIR", t.TempDir())
	writeHatchProfile(t, os.Getenv("ZERG_EGG_PROFILE_DIR"), "GLM-5.3-Flash", 4, 4) // 档案齐备也不行
	fake := &fakeHatcher{}
	m := newHatchTestManager(fake)

	entry := hatchTestEntry("GLM-5.3-Flash")
	resp, err := m.doStart("GLM-5.3-Flash", entry)
	if err != nil {
		t.Fatalf("doStart 返回 err=%v（既有路径用响应体表达失败）", err)
	}
	if got, _ := resp["status"].(int); got != 500 {
		t.Fatalf("开关关时应走既有裸 exec 路径（二进制不存在 ⇒ 500），实得 %v", resp)
	}
	if fake.hatchCalls != 0 || len(fake.collected) != 0 {
		t.Fatalf("开关关时孵化器一次都不许被调用：hatch=%d collect=%v", fake.hatchCalls, fake.collected)
	}
	for name, sp := range m.procs {
		if sp.Unit != "" {
			t.Fatalf("开关关时不许出现单元归属：%s.Unit=%q", name, sp.Unit)
		}
	}
}

// ── 开关开：闸门拒孵一律不起单元 ────────────────────────────────────────────

// TestHatchOn_NoProfile507AndNoUnit 开关开 + 无实测档案 ⇒ 507 拒孵，不起任何单元、不登记驻留。
func TestHatchOn_NoProfile507AndNoUnit(t *testing.T) {
	t.Setenv(EnvHatch, "1")
	t.Setenv("ZERG_EGG_PROFILE_DIR", t.TempDir()) // 空档案目录
	fake := &fakeHatcher{}
	m := newHatchTestManager(fake)

	resp, err := m.doStart("GLM-5.3-Flash", hatchTestEntry("GLM-5.3-Flash"))
	if err != nil {
		t.Fatalf("doStart 返回 err=%v", err)
	}
	if got, _ := resp["status"].(int); got != 507 {
		t.Fatalf("无实测档案必须 507 拒孵（§8.4 标定铁律），实得 %v", resp)
	}
	if code, _ := resp["code"].(string); code != "no measured profile" {
		t.Fatalf("code 应为 no measured profile，实得 %q", code)
	}
	if msg, _ := resp["error"].(string); !strings.Contains(msg, "实测档案") {
		t.Fatalf("理由要写清「无实测档案」，实得 %q", msg)
	}
	if fake.hatchCalls != 0 {
		t.Fatalf("拒孵时不得起任何单元，实得 Hatch 被调 %d 次", fake.hatchCalls)
	}
	if len(m.procs) != 0 {
		t.Fatalf("拒孵时不得登记驻留，实得 %v", keysOf(m.procs))
	}
}

// TestHatchOn_GateUnreadable507 开关开 + 两账读不到（预算未标定 / 读不到 GTT）⇒ 507 拒孵。
func TestHatchOn_GateUnreadable507(t *testing.T) {
	t.Setenv(EnvHatch, "1")
	dir := t.TempDir()
	t.Setenv("ZERG_EGG_PROFILE_DIR", dir)
	writeHatchProfile(t, dir, "GLM-5.3-Flash", 4, 4)
	withHatchGateRead(t, 0, 0, fmt.Errorf("GTT 预算未标定"))
	fake := &fakeHatcher{}
	m := newHatchTestManager(fake)

	resp, _ := m.doStart("GLM-5.3-Flash", hatchTestEntry("GLM-5.3-Flash"))
	if got, _ := resp["status"].(int); got != 507 {
		t.Fatalf("账读不到必须 fail-closed 507，实得 %v", resp)
	}
	if code, _ := resp["code"].(string); code != "hatch gate unreadable" {
		t.Fatalf("code 应为 hatch gate unreadable，实得 %q", code)
	}
	if fake.hatchCalls != 0 || len(m.procs) != 0 {
		t.Fatalf("拒孵时不得起单元/登记驻留：hatch=%d procs=%v", fake.hatchCalls, keysOf(m.procs))
	}
}

// TestHatchOn_GateInsufficient507 开关开 + 两账不够 ⇒ 507 + 差额，仍不起单元。
func TestHatchOn_GateInsufficient507(t *testing.T) {
	t.Setenv(EnvHatch, "1")
	dir := t.TempDir()
	t.Setenv("ZERG_EGG_PROFILE_DIR", dir)
	writeHatchProfile(t, dir, "GLM-5.3-Flash", 32.7, 40) // 两账都远不够
	withHatchGateRead(t, 1.0, 1.0, nil)
	fake := &fakeHatcher{}
	m := newHatchTestManager(fake)

	resp, _ := m.doStart("GLM-5.3-Flash", hatchTestEntry("GLM-5.3-Flash"))
	if got, _ := resp["status"].(int); got != 507 {
		t.Fatalf("双闸门未过必须 507，实得 %v", resp)
	}
	if code, _ := resp["code"].(string); code != "insufficient memory" {
		t.Fatalf("code 应为 insufficient memory（与既有 507 口径一致），实得 %q", code)
	}
	msg, _ := resp["error"].(string)
	for _, want := range []string{"GTT", "内存", "缺"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("差额说明应含 %q，实得 %q", want, msg)
		}
	}
	if fake.hatchCalls != 0 || len(m.procs) != 0 {
		t.Fatalf("拒孵时不得起单元/登记驻留：hatch=%d procs=%v", fake.hatchCalls, keysOf(m.procs))
	}
}

// TestHatchOn_SpecValidateRejected502 声明校验不过（schema_version=0 认不得）⇒ 502 拒孵，不起单元。
func TestHatchOn_SpecValidateRejected502(t *testing.T) {
	t.Setenv(EnvHatch, "1")
	t.Setenv(EnvWorkDirRoot, t.TempDir())
	dir := t.TempDir()
	t.Setenv("ZERG_EGG_PROFILE_DIR", dir)
	writeHatchProfile(t, dir, "GLM-5.3-Flash", 4, 4)
	withHatchGateRead(t, 100, 100, nil)
	fake := &fakeHatcher{}
	m := newHatchTestManager(fake)

	entry := hatchTestEntry("GLM-5.3-Flash")
	entry.SchemaVersion = 0 // 未声明的遗留条目 ⇒ 认不得的格式，拒孵
	resp, _ := m.doStart("GLM-5.3-Flash", entry)
	if got, _ := resp["status"].(int); got != 502 {
		t.Fatalf("声明认不得必须 502 拒孵，实得 %v", resp)
	}
	if code, _ := resp["code"].(string); code != "hatch spec rejected" {
		t.Fatalf("code 应为 hatch spec rejected，实得 %q", code)
	}
	if fake.hatchCalls != 0 || len(m.procs) != 0 {
		t.Fatalf("拒孵时不得起单元/登记驻留：hatch=%d procs=%v", fake.hatchCalls, keysOf(m.procs))
	}
}

// ── 开关开：走孵化路径 ──────────────────────────────────────────────────────

// TestHatchOn_HatchesThroughHatcher 开关开 + 有档案 + 两账够 ⇒ 起单元、单元名进 subproc、
// 就绪走既有判据（端口轮询 + 功能预检，跑的是真实代码）、孵化后调封闭性核验。
func TestHatchOn_HatchesThroughHatcher(t *testing.T) {
	t.Setenv(EnvHatch, "1")
	workRoot := t.TempDir()
	t.Setenv(EnvWorkDirRoot, workRoot)
	dir := t.TempDir()
	t.Setenv("ZERG_EGG_PROFILE_DIR", dir)
	writeHatchProfile(t, dir, "GLM-5.3-Flash", 4, 4)
	withHatchGateRead(t, 100, 100, nil)

	fake := &fakeHatcher{mainPID: os.Getpid(), enclose: enclosedReport()}
	m := newHatchTestManager(fake)
	defer fake.closeEngines()

	entry := hatchTestEntry("GLM-5.3-Flash")
	resp, err := m.doStart("GLM-5.3-Flash", entry)
	if err != nil {
		t.Fatalf("doStart 返回 err=%v", err)
	}
	if ok, _ := resp["ok"].(bool); !ok {
		t.Fatalf("孵化路径应就绪，实得 %v", resp)
	}
	if fake.hatchCalls != 1 {
		t.Fatalf("孵化器应恰好被调 1 次，实得 %d", fake.hatchCalls)
	}
	sp := m.procs["GLM-5.3-Flash"]
	if sp == nil {
		t.Fatal("孵化出的卵应在驻留清单里")
	}
	if want := hatch.UnitName("GLM-5.3-Flash"); sp.Unit != want {
		t.Fatalf("单元名应记进 subproc.Unit（%q），实得 %q", want, sp.Unit)
	}
	if sp.state != StateReady {
		t.Fatalf("孵化成功后就绪状态应为 ready，实得 %q", sp.state)
	}
	if sp.proc != nil {
		t.Fatal("孵化路径没有本端进程句柄（sp.proc 必须仍为 nil，不得改其语义）")
	}
	if got, _ := resp["port"].(int); got != sp.port {
		t.Fatalf("响应端口应与 subproc 一致，实得 %v / %d", resp["port"], sp.port)
	}
	// 声明确实按映射规则发出去了（宿主权重路径不得出现在参数里）
	spec := fake.specs[0]
	if spec.EggID != "GLM-5.3-Flash" || spec.WeightPath != "/data/models/glm" {
		t.Fatalf("孵化声明不对：%+v", spec)
	}
	if spec.EnginePathInSpace != "/engine/zerg-acceptance-noop-binary" {
		t.Fatalf("空间内引擎路径不对：%q", spec.EnginePathInSpace)
	}
	if got := argAfter(spec.EngineArgs, "-m"); got != "/models/GLM-5.3-Flash-Q4_K_M.gguf" {
		t.Fatalf("参数里的权重路径应改写成空间内路径，实得 %q", got)
	}
	for _, a := range spec.EngineArgs {
		if strings.Contains(a, "/data/models/") {
			t.Fatalf("参数里不得残留宿主权重路径：%q", a)
		}
	}
	// 工作目录：WorkDir 是**空间内**路径；宿主每卵目录经可写绑定落进空间（§6.6「一次性」侧）
	if spec.WorkDir != "/work" {
		t.Fatalf("WorkDir 应是空间内路径 /work（宿主路径填它 ⇒ 真孵化 --chdir 必失败），实得 %q", spec.WorkDir)
	}
	if want := filepath.Join(workRoot, "GLM-5.3-Flash") + ":/work"; len(spec.ExtraRWBinds) == 0 || spec.ExtraRWBinds[0] != want {
		t.Fatalf("宿主一次性工作目录应可写绑到 /work（%q），实得 %v", want, spec.ExtraRWBinds)
	}
	// 封闭性核验：孵化后确实核了（pid 取自单元主进程），且**实测通过 ⇒ 观测面记已验证**
	if fake.verifyHits != 1 || fake.verifyPID != os.Getpid() {
		t.Fatalf("孵化后应核一次封闭性（pid 取 MainPID），实得 hits=%d pid=%d", fake.verifyHits, fake.verifyPID)
	}
	if !sp.enclosureVerified || sp.enclosureNote == "" {
		t.Fatalf("核验通过时应记 enclosure_verified=true 且留痕非空，实得 verified=%v note=%q",
			sp.enclosureVerified, sp.enclosureNote)
	}
	obs := m.EggObservations()
	if len(obs) != 1 || !obs[0].EnclosureVerified || obs[0].EnclosureNote == "" {
		t.Fatalf("观测面应能看见「已核验」这一条事实，实得 %+v", obs)
	}
}

// TestHatchOn_EnclosureUnreadableStillServes 读不到核验证据（拿不到 pid / 读不到 mountinfo）⇒
// **不拒服务**（卵照常对外），但必须**在观测面标出来**（enclosure_verified=false + note）——
// 「没读到」与「读到不符」是两回事，不得混为一谈（§6.9 静默失效不得当凭据）。
func TestHatchOn_EnclosureUnreadableStillServes(t *testing.T) {
	cases := []struct {
		name         string
		fake         *fakeHatcher
		wantNote     string
		wantVerifyNo int // 该情形下核验应被调用几次
	}{
		{"拿不到引擎 pid", &fakeHatcher{}, "拿不到引擎 pid", 0},
		{"读不到 mountinfo", &fakeHatcher{mainPID: os.Getpid(),
			verifyErr: fmt.Errorf("读 /proc/%d/mountinfo 失败：no such process", os.Getpid())}, "读不到", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(EnvHatch, "1")
			t.Setenv(EnvWorkDirRoot, t.TempDir())
			dir := t.TempDir()
			t.Setenv("ZERG_EGG_PROFILE_DIR", dir)
			writeHatchProfile(t, dir, "GLM-5.3-Flash", 4, 4)
			withHatchGateRead(t, 100, 100, nil)

			fake := tc.fake
			m := newHatchTestManager(fake)
			defer fake.closeEngines()

			resp, err := m.doStart("GLM-5.3-Flash", hatchTestEntry("GLM-5.3-Flash"))
			if err != nil {
				t.Fatalf("doStart 返回 err=%v", err)
			}
			if ok, _ := resp["ok"].(bool); !ok {
				t.Fatalf("读不到核验证据不得拒服务（卵照常对外），实得 %v", resp)
			}
			sp := m.procs["GLM-5.3-Flash"]
			if sp == nil {
				t.Fatal("读不到核验证据时卵仍应在驻留清单里（不拒服务）")
			}
			if sp.state != StateReady {
				t.Fatalf("读不到核验证据时应照常就绪，实得 %q", sp.state)
			}
			if sp.enclosureVerified {
				t.Fatal("读不到证据时绝不许声称「已核验」（未核验不等于通过）")
			}
			if !strings.Contains(sp.enclosureNote, tc.wantNote) {
				t.Fatalf("留痕应写明 %q，实得 %q", tc.wantNote, sp.enclosureNote)
			}
			if fake.verifyHits != tc.wantVerifyNo {
				t.Fatalf("核验调用次数应为 %d，实得 %d", tc.wantVerifyNo, fake.verifyHits)
			}
			// 观测面必须看得见（这正是本批的理由：不能只写日志）
			obs := m.EggObservations()
			if len(obs) != 1 {
				t.Fatalf("观测面应有 1 枚卵，实得 %+v", obs)
			}
			if obs[0].EnclosureVerified {
				t.Error("观测面不得把「读不到」当成「已核验」")
			}
			if !strings.Contains(obs[0].EnclosureNote, "未核验") {
				t.Errorf("观测面 enclosure_note 应写明「未核验：…」，实得 %q", obs[0].EnclosureNote)
			}
		})
	}
}

// TestHatchOn_EnclosureMismatchCollectsAndRefuses 实读且不符 ⇒ **收卵 + 拒孵**：
// 返 502、错误里写清哪一项不符、单元真被收（Collect）、驻留清单干净（不得继续对外服务）。
func TestHatchOn_EnclosureMismatchCollectsAndRefuses(t *testing.T) {
	t.Setenv(EnvHatch, "1")
	t.Setenv(EnvWorkDirRoot, t.TempDir())
	dir := t.TempDir()
	t.Setenv("ZERG_EGG_PROFILE_DIR", dir)
	writeHatchProfile(t, dir, "GLM-5.3-Flash", 4, 4)
	withHatchGateRead(t, 100, 100, nil)

	// 实读到的 mountinfo 判定不符：/data 漏进来 + /work 不是可写落点（两种典型形态）
	fake := &fakeHatcher{mainPID: os.Getpid(), enclose: hatch.EnclosureReport{
		ModelsReadOnly:  true,
		DataHidden:      false,
		HomeHidden:      true,
		TmpIsTmpfs:      true,
		NewPIDNamespace: true,
		WorkReadWrite:   false,
		KVDiskReadWrite: true,
	}}
	m := newHatchTestManager(fake)
	defer fake.closeEngines()

	resp, err := m.doStart("GLM-5.3-Flash", hatchTestEntry("GLM-5.3-Flash"))
	if err != nil {
		t.Fatalf("doStart 返回 err=%v", err)
	}
	if got, _ := resp["status"].(int); got != 502 {
		t.Fatalf("核验不符必须拒孵（502），实得 %v", resp)
	}
	if code, _ := resp["code"].(string); code != "enclosure verification failed" {
		t.Fatalf("code 应为 enclosure verification failed，实得 %q", code)
	}
	msg, _ := resp["error"].(string)
	for _, want := range []string{"已收卵", "/data", "/work"} {
		if !strings.Contains(msg, want) {
			t.Errorf("错误信息应含 %q（写清哪一项不符 + 已收卵），实得 %q", want, msg)
		}
	}
	if want := hatch.UnitName("GLM-5.3-Flash"); len(fake.collected) != 1 || fake.collected[0] != want {
		t.Fatalf("核验不符必须收卵（走 Collect），实得 %v", fake.collected)
	}
	if len(m.procs) != 0 {
		t.Fatalf("拒孵后不得留在驻留清单（不许继续对外服务），实得 %v", keysOf(m.procs))
	}
	if now := m.EggObservations(); len(now) != 0 {
		t.Fatalf("收卵后的卵不得再出现在观测面，实得 %+v", now)
	}
}

// enclosedReport 一份「七项全过」的核验报告（真实核验由 hatch 包读 mountinfo 得出；
// 单测里用替身给确定值，好把上层的三态分级单独钉住）。
func enclosedReport() hatch.EnclosureReport {
	return hatch.EnclosureReport{
		ModelsReadOnly:  true,
		DataHidden:      true,
		HomeHidden:      true,
		TmpIsTmpfs:      true,
		NewPIDNamespace: true,
		WorkReadWrite:   true,
		KVDiskReadWrite: true,
	}
}

// TestHatchOn_VerifyWithoutPIDIsNotPassed 拿不到 pid ⇒ 三态里的「读不到」：
// 记「未核验」（绝不判通过）、且给出一句话留痕（供观测面 enclosure_note）。
func TestHatchOn_VerifyWithoutPIDIsNotPassed(t *testing.T) {
	fake := &fakeHatcher{} // mainPID=0 ⇒ MainPID 报错
	m := newHatchTestManager(fake)
	state, note := m.verifyEnclosure("zerg-x")
	if fake.verifyHits != 0 {
		t.Fatal("拿不到 pid 时不得调用核验（更不得当成通过）")
	}
	if state != enclosureUnreadable {
		t.Fatalf("拿不到 pid 时必须按「读不到」处置（不是通过、也不是不符），实得 %v", state)
	}
	if !strings.Contains(note, "未核验") || !strings.Contains(note, "拿不到引擎 pid") {
		t.Fatalf("留痕应写明未核验与原因，实得 %q", note)
	}
}

// TestHatchOn_VerifyMismatchIsThreeState 三态不许塌成一个布尔：
// 实读不符 ⇒ enclosureMismatch（调用方据此收卵拒孵）；读不到 ⇒ enclosureUnreadable（不拒服务）。
func TestHatchOn_VerifyMismatchIsThreeState(t *testing.T) {
	notEnclosed := hatch.EnclosureReport{
		ModelsReadOnly: true, DataHidden: true, HomeHidden: true,
		TmpIsTmpfs: true, NewPIDNamespace: true, WorkReadWrite: true, KVDiskReadWrite: false,
	}
	fake := &fakeHatcher{mainPID: os.Getpid(), enclose: notEnclosed}
	m := newHatchTestManager(fake)
	state, note := m.verifyEnclosure("zerg-x")
	if state != enclosureMismatch {
		t.Fatalf("实读不符必须判 mismatch，实得 %v", state)
	}
	if !strings.Contains(note, "核验不符") || !strings.Contains(note, "/kvdisk") {
		t.Fatalf("留痕应点名不符的那一项（/kvdisk），实得 %q", note)
	}

	unreadable := &fakeHatcher{mainPID: os.Getpid(), verifyErr: fmt.Errorf("read failed")}
	m2 := newHatchTestManager(unreadable)
	if st, _ := m2.verifyEnclosure("zerg-x"); st != enclosureUnreadable {
		t.Fatalf("读不到必须判 unreadable（与不符分开），实得 %v", st)
	}

	// 通过 ⇒ verified
	ok := &fakeHatcher{mainPID: os.Getpid(), enclose: enclosedReport()}
	m3 := newHatchTestManager(ok)
	st3, note3 := m3.verifyEnclosure("zerg-x")
	if st3 != enclosureVerified {
		t.Fatalf("七项全过应判 verified，实得 %v", st3)
	}
	if !strings.Contains(note3, "封闭性核验通过") {
		t.Fatalf("通过时的留痕应写结论与实测值，实得 %q", note3)
	}
}

// ── 收卵：Unit 非空走 Collect（Stop / ReapIdle 两条路）────────────────────────

// TestHatchCollect_StopAndHandlePaths Stop：孵化卵（Unit 非空、无句柄）⇒ 收卵走 Collect；
// 裸 exec 卵（Unit 空）⇒ 既有句柄路径（此处句柄为 nil ⇒ 什么都不做，且绝不调用 Collect）。
func TestHatchCollect_StopAndHandlePaths(t *testing.T) {
	fake := &fakeHatcher{}
	m := newHatchTestManager(fake)
	m.procs = map[string]*subproc{
		"hatched": {model: "hatched", state: StateReady, Unit: "zerg-hatched", port: 9101},
		"bare":    {model: "bare", state: StateReady, port: 9102},
	}
	m.Stop()
	if len(fake.collected) != 1 || fake.collected[0] != "zerg-hatched" {
		t.Fatalf("孵化卵收卵应走 Collect（幂等），实得 %v", fake.collected)
	}
	if fake.activeChecks == 0 {
		t.Fatal("收卵后必须再问一次单元是否真在跑（§6.9：Collect 报成功不等于收干净了）")
	}
	if len(m.procs) != 0 {
		t.Fatalf("Stop 后驻留清单应为空，实得 %v", keysOf(m.procs))
	}
}

// TestHatchCollect_StillActiveIsReported 收卵后单元仍在跑 ⇒ 如实留痕（不许把「停命令成功」当收干净）。
func TestHatchCollect_StillActiveIsReported(t *testing.T) {
	fake := &fakeHatcher{stillActive: true}
	m := newHatchTestManager(fake)
	m.collectUnit("zerg-stuck")
	if len(fake.collected) != 1 || fake.collected[0] != "zerg-stuck" {
		t.Fatalf("Collect 应被调用一次，实得 %v", fake.collected)
	}
	if fake.activeChecks != 1 {
		t.Fatalf("应恰好复核一次单元活性，实得 %d", fake.activeChecks)
	}
	// 未设 Unit 的空串：不许触发任何收卵动作
	m.collectUnit("")
	if len(fake.collected) != 1 {
		t.Fatalf("空单元名不得触发 Collect，实得 %v", fake.collected)
	}
}

// TestHatchCollect_ReapIdle 空窗到期收卵：孵化卵走 Collect（且仍遵守 P2 的锁内赢权语义）。
func TestHatchCollect_ReapIdle(t *testing.T) {
	fake := &fakeHatcher{}
	m := newHatchTestManager(fake)
	now := time.Now()
	m.procs = map[string]*subproc{
		"hatched": {
			model:    "hatched",
			state:    StateIdleArmed, // 空窗计时中
			Unit:     "zerg-hatched",
			port:     9103,
			lastUsed: now.Add(-time.Hour),
			entry:    &registry.ModelEntry{IdleUnloadS: 60},
		},
	}
	m.SetIdleTTL(time.Minute)

	var hookStates []string
	m.stopHook = func(sp *subproc) { hookStates = append(hookStates, sp.state) }

	reaped := m.ReapIdle(now)
	if len(reaped) != 1 || reaped[0] != "hatched" {
		t.Fatalf("空窗到期的孵化卵应被收走，实得 %v", reaped)
	}
	if len(fake.collected) != 1 || fake.collected[0] != "zerg-hatched" {
		t.Fatalf("ReapIdle 收卵应走 Collect，实得 %v", fake.collected)
	}
	if len(hookStates) != 1 || hookStates[0] != StateDraining {
		t.Fatalf("收卵时状态必须是 draining（锁内赢权语义不变），实得 %v", hookStates)
	}
	if _, still := m.procs["hatched"]; still {
		t.Fatal("收卵后不得留在驻留清单里")
	}
}

// ── 工具与替身 ──────────────────────────────────────────────────────────────

// hatchTestEntry 一枚「不会真起进程」的卵声明：cmd 指向不存在的二进制（裸 exec 路径会快速 500），
// 引擎路径是绝对路径（孵化映射只要绝对路径，不要求宿主上真有这个文件）。
func hatchTestEntry(name string) *registry.ModelEntry {
	e := &registry.ModelEntry{
		Backend:       "llama-server",
		File:          "/data/models/glm/GLM-5.3-Flash-Q4_K_M.gguf",
		SchemaVersion: registry.EggSchemaVersionCurrent,
		Cmd:           registry.CmdString("/nonexistent/zerg-acceptance-noop-binary -m {file} --port {port}"),
	}
	e.SetEggNameForTest(name)
	return e
}

func newHatchTestManager(fake *fakeHatcher) *Manager {
	m := newEvictTestManager(1, map[string]*subproc{})
	m.hatcher = fake
	return m
}

// writeHatchProfile 写一份能过校验的实测档案（文件名按 monitor.EggProfilePath 的规则）。
func writeHatchProfile(t *testing.T, dir, eggID string, peakGtt, peakMem float64) string {
	t.Helper()
	body := fmt.Sprintf(`weight_size_gb: 90
peak_gtt_gb: %v
peak_mem_gb: %v
load_seconds: 120
throughput_tok_s: 4.8
suggested_idle_unload_s: 600
schema_version: 1
measured_at: 2026-09-15T10:00:00+08:00
machine: x3
calib_runs: 3
`, peakGtt, peakMem)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, eggID+".yaml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// withHatchGateRead 注入确定的两账读数（不依赖开发机的 sysfs / 内存）。
func withHatchGateRead(t *testing.T, gtt, mem float64, readErr error) {
	t.Helper()
	old := hatchGateRead
	hatchGateRead = func() (float64, float64, error) { return gtt, mem, readErr }
	t.Cleanup(func() { hatchGateRead = old })
}

// fakeHatcher 假孵化器：只记账 + 按声明里的端口起一个假引擎，绝不碰真实 systemd。
type fakeHatcher struct {
	specs        []hatch.Spec
	hatchCalls   int
	collected    []string
	activeChecks int
	stillActive  bool // true ⇒ 收卵后复核仍报「在跑」（用于验「Collect 成功 ≠ 收干净」的留痕）
	mainPID      int
	enclose      hatch.EnclosureReport
	verifyErr    error // 非 nil ⇒ 核验读不到（模拟 /proc/<pid>/mountinfo 读失败）
	verifyPID    int
	verifyHits   int
	engines      []*http.Server
}

func (f *fakeHatcher) Hatch(_ context.Context, spec hatch.Spec) (string, error) {
	f.hatchCalls++
	f.specs = append(f.specs, spec)
	unit := hatch.UnitName(spec.EggID)
	port, ok := argIntAfter(spec.EngineArgs, "--port")
	if !ok {
		return unit, fmt.Errorf("假孵化器拿不到端口参数：%v", spec.EngineArgs)
	}
	srv, err := startFakeEngine(port)
	if err != nil {
		return unit, err
	}
	f.engines = append(f.engines, srv)
	return unit, nil
}

func (f *fakeHatcher) Collect(_ context.Context, unit string) error {
	f.collected = append(f.collected, unit)
	return nil
}

func (f *fakeHatcher) Active(_ context.Context, _ string) (bool, error) {
	f.activeChecks++
	return f.stillActive, nil
}

func (f *fakeHatcher) MainPID(_ context.Context, _ string) (int, error) {
	if f.mainPID <= 0 {
		return 0, fmt.Errorf("假孵化器没有主进程 pid")
	}
	return f.mainPID, nil
}

func (f *fakeHatcher) VerifyEnclosure(pid int) (hatch.EnclosureReport, error) {
	f.verifyHits++
	f.verifyPID = pid
	if f.verifyErr != nil {
		return hatch.EnclosureReport{}, f.verifyErr // 读不到 ≠ 不符（上层要按「未核验」处置）
	}
	return f.enclose, nil
}

func (f *fakeHatcher) closeEngines() {
	for _, s := range f.engines {
		_ = s.Close()
	}
}

// startFakeEngine 在指定端口起一个只会应答 /health 与 /v1/chat/completions 的假引擎——
// 好让「就绪判据」（waitForReady + 功能预检）跑的是真实代码。
func startFakeEngine(port int) (*http.Server, error) {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return nil, fmt.Errorf("假引擎监听 %d 失败：%w", port, err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")
	})
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"p"}}]}`)
	})
	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(ln) }()
	return srv, nil
}

// argIntAfter 取 flag 后面的整数值（找不到返回 false）。
func argIntAfter(args []string, flag string) (int, bool) {
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			if n, err := strconv.Atoi(args[i+1]); err == nil {
				return n, true
			}
		}
	}
	return 0, false
}
