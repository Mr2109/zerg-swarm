package backend

// default_pin_test.go —— 缺口 GAP-20261019-01 的两态验收（反例优先）。
//
//	① 钉住/声明为默认的模型在驻留时：任何会驱逐它的装载**必须被拒**且**逐字点名**；
//	② 未声明默认 / 总闸 `ZERG_DEFAULT_PIN=off` ⇒ 与改动前一致（不拦）；
//	③ `/unload` 不带 models 不得静默停掉钉住的默认模型（负控）；
//	④ `Stop()`（agent 退出那一手）必须把账本记成 `released` 且 why 带 requester。

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Mr2109/zerg-swarm/agent/internal/registry"
)

// dpinRegistry 造一份最小注册表（defaultModel 非空 ⇒ 该条目带 default: true）。
func dpinRegistry(t *testing.T, defaultModel string) *registry.Registry {
	t.Helper()
	dir := t.TempDir()
	body := ""
	for _, name := range []string{"Qwen", "Other"} {
		body += name + ":\n  file: /tmp/" + name + ".gguf\n"
		if name == defaultModel {
			body += "  default: true\n"
		}
	}
	p := filepath.Join(dir, "agent_models.yaml")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	reg, err := registry.New(p)
	if err != nil {
		t.Fatalf("注册表加载失败: %v", err)
	}
	return reg
}

// dpinManager 造一个不启后台循环、不起真进程的 Manager。
func dpinManager(reg *registry.Registry, def string, procs map[string]*subproc) *Manager {
	return &Manager{
		procs:        procs,
		loading:      map[string]*loadWaiter{},
		registry:     reg,
		machine:      "edge-node",
		maxResident:  1,
		defaultModel: def,
	}
}

// ① 钉住态：会驱逐默认模型的装载被拒，且拒语逐字点名两边。
func TestDefaultPin_RejectsEvictingLoadByName(t *testing.T) {
	t.Setenv(EnvDefaultPin, "")
	t.Setenv(EnvDefaultModel, "")
	reg := dpinRegistry(t, "Qwen")
	m := dpinManager(reg, "Qwen", map[string]*subproc{
		"Qwen": {model: "Qwen", state: StateReady, port: 9400, entry: &registry.ModelEntry{}, lastUsed: time.Now()},
	})

	res, err := m.Start("Other")
	if err != nil {
		t.Fatalf("拒装不应是 error（要如实回结果体）: %v", err)
	}
	if res["ok"] != false || res["status"] != 409 {
		t.Fatalf("应回 409 ok=false，实得 %#v", res)
	}
	msg, _ := res["error"].(string)
	if !strings.Contains(msg, "会驱逐钉住的默认模型 Qwen") || !strings.Contains(msg, "拒装 Other") {
		t.Fatalf("拒语必须逐字点名「会驱逐钉住的默认模型 Qwen，拒装 Other」，实得: %q", msg)
	}
	if res["pinned"] != "Qwen" || res["requester"] != "Other" {
		t.Fatalf("机器面字段应带 pinned=Qwen/requester=Other，实得 %#v", res)
	}
	if _, ok := m.procs["Qwen"]; !ok {
		t.Fatalf("被钉住的默认模型不许被摘掉（拒绝发生在任何驱逐之前），现 procs=%v", keysOf(m.procs))
	}
}

// ② 三态不拦：总闸 off / 未声明默认 / 装的就是默认模型 / 默认模型没驻留 / 多槽有空位。
func TestDefaultPin_GuardOnlyWhenItWouldEvict(t *testing.T) {
	t.Setenv(EnvDefaultModel, "")
	reg := dpinRegistry(t, "Qwen")
	base := func() map[string]*subproc {
		return map[string]*subproc{
			"Qwen": {model: "Qwen", state: StateReady, port: 9400, entry: &registry.ModelEntry{}},
		}
	}

	t.Run("总闸 off", func(t *testing.T) {
		t.Setenv(EnvDefaultPin, "off")
		if got := dpinManager(reg, "Qwen", base()).defaultPinConflict("Other"); got != nil {
			t.Fatalf("总闸 off 时不得拦，实得 %#v", got)
		}
	})
	t.Run("未声明默认", func(t *testing.T) {
		t.Setenv(EnvDefaultPin, "")
		if got := dpinManager(reg, "", base()).defaultPinConflict("Other"); got != nil {
			t.Fatalf("无默认模型时不得拦，实得 %#v", got)
		}
	})
	t.Run("装的就是默认模型", func(t *testing.T) {
		t.Setenv(EnvDefaultPin, "")
		if got := dpinManager(reg, "Qwen", base()).defaultPinConflict("Qwen"); got != nil {
			t.Fatalf("重装/复用默认模型本身不得拦，实得 %#v", got)
		}
	})
	t.Run("默认模型没驻留", func(t *testing.T) {
		t.Setenv(EnvDefaultPin, "")
		if got := dpinManager(reg, "Qwen", map[string]*subproc{}).defaultPinConflict("Other"); got != nil {
			t.Fatalf("默认模型不在驻留表时不得拦，实得 %#v", got)
		}
	})
	t.Run("多槽有空位", func(t *testing.T) {
		t.Setenv(EnvDefaultPin, "")
		m := dpinManager(reg, "Qwen", base())
		m.maxResident = 2
		if got := m.defaultPinConflict("Other"); got != nil {
			t.Fatalf("上限=2 且只驻留 1 枚（不驱逐）时不得拦，实得 %#v", got)
		}
	})
}

// ③ 负控：/unload 不带 models（Unload(nil)）不得静默停掉钉住的默认模型。
func TestUnloadAll_DoesNotSilentlyStopPinnedDefault(t *testing.T) {
	t.Setenv(EnvDefaultPin, "")
	t.Setenv(EnvDefaultModel, "")
	t.Setenv(EnvResidencyLedger, "off") // 本用例只验驻留表，不落账
	reg := dpinRegistry(t, "Qwen")
	m := dpinManager(reg, "Qwen", map[string]*subproc{
		"Qwen":  {model: "Qwen", state: StateReady, port: 9400, entry: &registry.ModelEntry{}, pinUntil: time.Now().Add(time.Hour)},
		"Other": {model: "Other", state: StateReady, port: 9401, entry: &registry.ModelEntry{}},
	})

	res := m.Unload(nil)
	stopped, _ := res["stopped"].([]string)
	skipped, _ := res["skipped"].([]string)
	if len(stopped) != 1 || stopped[0] != "Other" {
		t.Fatalf("全卸应只停未钉的 Other，实得 stopped=%v", stopped)
	}
	if len(skipped) != 1 || skipped[0] != "Qwen" {
		t.Fatalf("钉住的默认模型必须留在 skipped 并给原因，实得 skipped=%v", skipped)
	}
	if _, ok := m.procs["Qwen"]; !ok {
		t.Fatalf("钉住的默认模型被静默停掉了（负控失败）")
	}
	reasons, _ := res["reasons"].(map[string]string)
	if !strings.Contains(reasons["Qwen"], "pin 未到期") {
		t.Fatalf("skipped 的原因必须说清是 pin 未到期，实得 %q", reasons["Qwen"])
	}
}

// ④ Stop()（agent 退出）记 released，且 why 带 requester；不是 vanished。
func TestStop_RecordsReleasedWithRequester(t *testing.T) {
	t.Setenv(EnvDefaultPin, "")
	t.Setenv(EnvDefaultModel, "")
	ledger := filepath.Join(t.TempDir(), "egg-residency.json")
	t.Setenv(EnvResidencyLedger, ledger)
	doc := `{"schema":"zerg-egg-residency/1","updated_at":"2026-10-19T00:00:00Z","records":[` +
		`{"egg_id":"Qwen","model":"Qwen","pid":4242,"port":9400,"state":"loaded","loaded_at":"2026-10-19T00:00:00Z"}]}`
	if err := os.WriteFile(ledger, []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	m := dpinManager(dpinRegistry(t, "Qwen"), "Qwen", map[string]*subproc{
		"Qwen": {model: "Qwen", state: StateReady, port: 9400, entry: &registry.ModelEntry{}},
	})
	m.Stop()

	ev, err := os.ReadFile(ledger + ".events.jsonl")
	if err != nil {
		t.Fatalf("未落事件行: %v", err)
	}
	line := string(ev)
	if !strings.Contains(line, `"reason":"released"`) {
		t.Fatalf("Stop() 必须记 released（✗ vanished），实得: %s", line)
	}
	if !strings.Contains(line, "requester=agentd") {
		t.Fatalf("released 的 why 必须带 requester，实得: %s", line)
	}
}

// ⑤ 默认模型就绪即上钉 + 巡检续期（不新起常驻机制）。
func TestDefaultPin_AutoPinAndRenew(t *testing.T) {
	t.Setenv(EnvDefaultPin, "")
	t.Setenv(EnvDefaultModel, "")
	t.Setenv(EnvDefaultPinTTL, "60")
	m := dpinManager(dpinRegistry(t, "Qwen"), "Qwen", map[string]*subproc{
		"Qwen": {model: "Qwen", state: StateReady, port: 9400, entry: &registry.ModelEntry{}},
	})
	m.ensureDefaultPin("Qwen")
	remain, ok := m.PinRemainS("Qwen")
	if !ok || remain <= 0 {
		t.Fatalf("默认模型就绪后应自动上钉，实得 ok=%v remain=%v", ok, remain)
	}
	// 走到 TTL 之后：巡检续期必须把它拉回满 TTL（而不是让它自然到期变回可驱逐）。
	m.renewDefaultPin(time.Now().Add(2 * time.Minute))
	remain2, ok := m.PinRemainS("Qwen")
	if !ok || remain2 < 50 {
		t.Fatalf("巡检应续期回满 TTL(60s)，实得 ok=%v remain=%v", ok, remain2)
	}
}

// ⑥ 可分辨面：ServedInfo 只认实际驻留的那一枚；Pinned 头三态闭集。
func TestServedInfo_ReportsResidentModel(t *testing.T) {
	t.Setenv(EnvDefaultPin, "")
	t.Setenv(EnvDefaultModel, "")
	m := dpinManager(dpinRegistry(t, "Qwen"), "Qwen", map[string]*subproc{
		"Qwen": {model: "Qwen", state: StateReady, port: 9400, entry: &registry.ModelEntry{}, pinUntil: time.Now().Add(time.Hour)},
	})
	si, ok := m.ServedInfo("Qwen")
	if !ok || si.Model != "Qwen" || si.Host != "edge-node" || si.Port != 9400 {
		t.Fatalf("ServedInfo 读数不符: ok=%v %#v", ok, si)
	}
	if got := si.PinnedHeaderValue(); got != "default" {
		t.Fatalf("默认模型的 X-Zerg-Served-Pinned 应为 default，实得 %q", got)
	}
	if _, ok := m.ServedInfo("Other"); ok {
		t.Fatalf("未驻留的模型不得报出 served 读数（✗ 拿请求里的 model 顶替）")
	}
}

// ⑦ 注册表判据：两枚 default ⇒ 拒绝加载并逐字点名（不猜一个）。
func TestRegistry_RejectsMultipleDefaults(t *testing.T) {
	dir := t.TempDir()
	body := "A:\n  file: /tmp/a.gguf\n  default: true\nB:\n  file: /tmp/b.gguf\n  default: true\n"
	p := filepath.Join(dir, "agent_models.yaml")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := registry.New(p)
	if err == nil || !strings.Contains(err.Error(), "至多一枚") || !strings.Contains(err.Error(), "A, B") {
		t.Fatalf("两枚 default 必须拒加载并点名 A/B，实得 err=%v", err)
	}
}
