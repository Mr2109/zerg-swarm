// egg_decl_wire_test.go —— 反例用例：卵声明校验必须真的接在**生产路径**上
// （2026-09-15 第一枚卵真机实测缺陷 8）。
//
// 缺陷原文（报告 §12 缺陷 8）：`registry.ValidateEggDeclaration` / `ValidateEgg` 此前
// **在生产路径没有任何调用点**（全仓 grep 只有定义）⇒ 孵化前的「格式校验」（schema_version /
// env_req 必填项）实际不生效：env_req 缺 lib_paths（设计上是 Fatal）也照孵。
// 修法：接在 doStart 的闸门**之前**（§6.8.4「先判格式，再算账」），Fatal ⇒ 拒孵。
//
// 本文件钉住四件事：
//
//	① 缺必填项（env_req 少 lib_paths）⇒ 502 拒孵，且**不起任何单元**、不登记驻留；
//	② schema_version 认不得 ⇒ 502 拒孵（理由里写清认得的版本）；
//	③ 「先判格式」是真的：把闸门做成读不到、把内存需求开到荒谬，拒因仍必须是**声明**那一条
//	   （不许被内存预检 / 闸门抢先返回别的错误码——拒因必须可归因）；
//	④ 不许误伤：合规的新格式声明、以及遗留条目（三者全空的旧注册表）都照样往下走。
package backend

import (
	"errors"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/agent/internal/registry"
)

// eggEntryWithEnvReq 一枚「三证齐全」或按需缺项的新格式卵声明。
func eggEntryWithEnvReq(name string, mutate func(*registry.EnvReq)) *registry.ModelEntry {
	e := hatchTestEntry(name)
	e.SchemaVersion = registry.EggSchemaVersionCurrent
	e.IdleUnloadS = 600
	req := &registry.EnvReq{
		Weights:      []string{"/data/models/glm"},
		LibPaths:     []string{"/engine/lib"},
		MemlockKB:    8192,
		MmapMaxCount: 1048576,
		Env:          map[string]string{"LD_LIBRARY_PATH": "/engine/lib"},
	}
	if mutate != nil {
		mutate(req)
	}
	e.EnvReq = req
	return e
}

// 断言一条「拒孵」响应：HTTP 语义码 + code + 理由片段。
func assertReject(t *testing.T, resp map[string]interface{}, wantStatus int, wantCode string, wantParts ...string) {
	t.Helper()
	if got, _ := resp["status"].(int); got != wantStatus {
		t.Fatalf("应拒孵 %d，实得 %+v", wantStatus, resp)
	}
	if got, _ := resp["code"].(string); got != wantCode {
		t.Fatalf("code 应为 %q，实得 %+v", wantCode, resp)
	}
	msg, _ := resp["error"].(string)
	for _, p := range wantParts {
		if !strings.Contains(msg, p) {
			t.Fatalf("拒孵理由应含 %q，实得 %q", p, msg)
		}
	}
}

// ① 缺必填项（lib_paths）⇒ 502「卵声明校验不过」，不起单元、不登记驻留。
func TestEggDeclaration_WiredAndRejectsMissingRequiredField(t *testing.T) {
	t.Setenv(EnvHatch, "1")
	t.Setenv("ZERG_EGG_PROFILE_DIR", t.TempDir()) // 档案故意不给：拒因必须是**声明**，不是档案
	withHatchGateRead(t, 999, 999, errors.New("账读不到（本用例只验「先判格式」，账不该被读）"))
	fake := &fakeHatcher{}
	m := newHatchTestManager(fake)

	entry := eggEntryWithEnvReq("egg-missing-libpaths", func(r *registry.EnvReq) { r.LibPaths = nil })
	resp, err := m.doStart("egg-missing-libpaths", entry)
	if err != nil {
		t.Fatalf("doStart 返回 err=%v（拒孵走响应体）", err)
	}
	assertReject(t, resp, 502, "egg declaration rejected", "lib_paths")
	if fake.hatchCalls != 0 || len(fake.specs) != 0 {
		t.Fatalf("拒孵不得起任何单元：hatch=%d specs=%d", fake.hatchCalls, len(fake.specs))
	}
	if _, has := m.procs["egg-missing-libpaths"]; has {
		t.Fatalf("被拒孵的卵不得登记驻留：%v", keysOf(m.procs))
	}
}

// ② schema_version 认不得 ⇒ 502 拒孵，理由写清「认不得 99 / 本子端认得 1」。
func TestEggDeclaration_RejectsUnknownSchemaVersion(t *testing.T) {
	t.Setenv(EnvHatch, "1")
	t.Setenv("ZERG_EGG_PROFILE_DIR", t.TempDir())
	withHatchGateRead(t, 999, 999, errors.New("账读不到"))
	fake := &fakeHatcher{}
	m := newHatchTestManager(fake)

	entry := eggEntryWithEnvReq("egg-v99", nil)
	entry.SchemaVersion = 99
	resp, err := m.doStart("egg-v99", entry)
	if err != nil {
		t.Fatalf("doStart 返回 err=%v", err)
	}
	assertReject(t, resp, 502, "egg declaration rejected", "schema_version=99", "期望 1")
	if fake.hatchCalls != 0 {
		t.Fatalf("拒孵不得起任何单元：hatch=%d", fake.hatchCalls)
	}
}

// ③ 「先判格式，再算账」：闸门读不到 + 内存需求荒谬（这两个都会各自拒孵），拒因仍必须是声明那一条。
func TestEggDeclaration_FormatJudgedBeforeAccounts(t *testing.T) {
	t.Setenv(EnvHatch, "1")
	t.Setenv("ZERG_EGG_PROFILE_DIR", t.TempDir())
	withHatchGateRead(t, 0, 0, errors.New("账读不到（算账侧本来也会拒）"))
	fake := &fakeHatcher{}
	m := newHatchTestManager(fake)

	entry := eggEntryWithEnvReq("egg-v99-huge", nil)
	entry.SchemaVersion = 99
	entry.MemGB = 1e9 // 内存预检（算账）必拒；声明（格式）先判 ⇒ 拒因是格式那一条
	resp, err := m.doStart("egg-v99-huge", entry)
	if err != nil {
		t.Fatalf("doStart 返回 err=%v", err)
	}
	assertReject(t, resp, 502, "egg declaration rejected", "schema_version=99")
}

// ④a 合规的新格式声明不许被这把新闸门误伤：放行到下一个决策点（此处是闸门读不到 ⇒ 507）。
func TestEggDeclaration_CompliantDeclarationPassesOnward(t *testing.T) {
	t.Setenv(EnvHatch, "1")
	dir := t.TempDir()
	t.Setenv("ZERG_EGG_PROFILE_DIR", dir)
	writeHatchProfile(t, dir, "egg-ok", 10, 10) // 档案齐备 ⇒ 下一个拒因才会是「账读不到」
	withHatchGateRead(t, 0, 0, errors.New("账读不到"))
	fake := &fakeHatcher{}
	m := newHatchTestManager(fake)

	resp, err := m.doStart("egg-ok", eggEntryWithEnvReq("egg-ok", nil))
	if err != nil {
		t.Fatalf("doStart 返回 err=%v", err)
	}
	assertReject(t, resp, 507, "hatch gate unreadable", "算不出就不装")
}

// ④b 遗留条目（schema_version/env_req/idle_unload_s 三者全空）不许被误伤：照孵 + 必须报出告警。
func TestEggDeclaration_LegacyEntryNotRejected(t *testing.T) {
	t.Setenv(EnvHatch, "1")
	t.Setenv("ZERG_EGG_PROFILE_DIR", t.TempDir())
	withHatchGateRead(t, 0, 0, errors.New("账读不到"))
	fake := &fakeHatcher{}
	m := newHatchTestManager(fake)

	e := hatchTestEntry("egg-legacy")
	e.SchemaVersion = 0 // 三者全空 = 遗留条目（P1 之前写的注册表）
	resp, err := m.doStart("egg-legacy", e)
	if err != nil {
		t.Fatalf("doStart 返回 err=%v", err)
	}
	if code, _ := resp["code"].(string); code == "egg declaration rejected" {
		t.Fatalf("遗留条目（三者全空）必须按告警放行，实得 %+v", resp)
	}
	if got, _ := resp["status"].(int); got == 502 {
		t.Fatalf("遗留条目不该被声明闸门拒：%+v", resp)
	}
}

// ⑤ 开关关 ⇒ 声明闸门一次都不执行（离线路径逐字不变）：坏声明也只走既有路径。
func TestEggDeclaration_NotJudgedWhenHatchOff(t *testing.T) {
	t.Setenv(EnvHatch, "") // 显式关
	withHatchGateRead(t, 0, 0, errors.New("开关关时账根本不该被读"))
	fake := &fakeHatcher{}
	m := newHatchTestManager(fake)

	entry := eggEntryWithEnvReq("egg-off", func(r *registry.EnvReq) { r.LibPaths = nil })
	entry.SchemaVersion = 99
	resp, err := m.doStart("egg-off", entry)
	if err != nil {
		t.Fatalf("doStart 返回 err=%v", err)
	}
	if code, _ := resp["code"].(string); code == "egg declaration rejected" {
		t.Fatalf("开关关时不许出现孵化专属的拒孵码（既有路径逐字不变），实得 %+v", resp)
	}
	if fake.hatchCalls != 0 {
		t.Fatalf("开关关时孵化器一次都不许被调用：hatch=%d", fake.hatchCalls)
	}
}
