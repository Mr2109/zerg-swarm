package gateway

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/config"
	"github.com/Mr2109/zerg-swarm/core/internal/resources"
	"github.com/Mr2109/zerg-swarm/core/internal/store"
)

// ── 批 3：X3 让位改成"只卸够"，地址取自配置 ─────────────────────────────────

// captureUnload 起一个假 X3 agent：记录收到的 /unload 请求体。
type unloadCapture struct {
	mu      sync.Mutex
	bodies  [][]byte
	reasons []string
}

func (c *unloadCapture) record(b []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	cp := make([]byte, len(b))
	copy(cp, b)
	c.bodies = append(c.bodies, cp)
}

func (c *unloadCapture) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.bodies)
}

// models 返回最后一次请求里的 models 清单；无（全卸）返回 nil。
func (c *unloadCapture) models(t *testing.T) []string {
	t.Helper()
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.bodies) == 0 {
		t.Fatal("没有任何 /unload 请求")
	}
	last := c.bodies[len(c.bodies)-1]
	if len(last) == 0 {
		return nil
	}
	var req struct {
		Models []string `json:"models"`
	}
	if err := json.Unmarshal(last, &req); err != nil {
		t.Fatalf("请求体不是预期 JSON: %q (%v)", string(last), err)
	}
	return req.Models
}

// fakeX3Agent 起假 agent 并把 fleet 配置里的 x3 指向它（地址来自配置——这本身就是"不硬编码"的证据）。
func fakeX3Agent(t *testing.T) (*unloadCapture, *config.FleetConfig) {
	t.Helper()
	cap := &unloadCapture{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		cap.record(body)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(srv.Close)

	addr := strings.TrimPrefix(srv.URL, "http://")
	host, port := addr, 0
	if i := strings.LastIndex(addr, ":"); i > 0 {
		host = addr[:i]
		p, err := strconv.Atoi(addr[i+1:])
		if err != nil {
			t.Fatalf("解析测试服务地址失败: %v", err)
		}
		port = p
	}
	cfg := &config.FleetConfig{
		Models: map[string][]config.ModelCandidate{
			ds4ModelName: {{Host: "x3", File: "/data/models/ds4.gguf", MemGb: ds4MemGB}},
			"ornith":     {{Host: "x3", File: "/data/models/ornith.gguf", MemGb: 22}},
		},
		Fleet: map[string]config.FleetNode{
			"x3": {Host: host, Port: port},
		},
	}
	return cap, cfg
}

// x3Snapshot 造一个 X3 心跳快照。
func x3Snapshot(t *testing.T, st *store.Store, model string, memTotal, memAvail, gpuPct float64, active int, residents []resources.ResidentEntry) {
	t.Helper()
	m := model
	st.ReceiveHeartbeat(store.HeartbeatRequest{
		Machine:        "x3",
		Model:          &m,
		ActiveRequests: active,
		Healthy:        true,
		MemTotalGb:     memTotal,
		MemAvailableGb: memAvail,
		GpuPct:         gpuPct,
		Resident:       residents,
	})
}

func gatewayWith(t *testing.T, cfg *config.FleetConfig, st *store.Store) *Gateway {
	t.Helper()
	return &Gateway{config: cfg, store: st, roundRobin: map[string]int{}, failCounts: map[string]int{}}
}

func resident(alias string, memGb float64, lruS float64, managed bool, req int) resources.ResidentEntry {
	return resources.ResidentEntry{
		Alias:        alias,
		Managed:      managed,
		State:        resources.StateReady,
		MemGb:        memGb,
		LastUsedAgoS: lruS,
		ReqCount:     req,
		Source:       "managed",
	}
}

// 只卸够（反例优先）：三个等大驻留、只要腾 2G → 只点名最旧的那一个。
func TestX3Yield_JustEnoughNotAll(t *testing.T) {
	cap, cfg := fakeX3Agent(t)
	st := store.NewStore()
	x3Snapshot(t, st, "other-model", 128, 20, 5, 0, []resources.ResidentEntry{
		resident("A", 8, 300, true, 0),
		resident("B", 8, 200, true, 0),
		resident("C", 8, 100, true, 0),
	})
	g := gatewayWith(t, cfg, st)

	g.ensureX3RoomForFile("/data/models/target.gguf", 22) // 缺口 = 22-20 = 2G

	got := cap.models(t)
	if len(got) != 1 || got[0] != "A" {
		t.Fatalf("只卸够：缺口 2G 应只点名最旧的 A，实得 %v", got)
	}
}

// 只卸够（多驻留）：DS4 需要 86G、可用 45G、占用 83G → 只该让位那个更大的驻留（小的留着）。
func TestEnsureDS4Room_YieldsOnlyTheBigOne(t *testing.T) {
	cap, cfg := fakeX3Agent(t)
	st := store.NewStore()
	x3Snapshot(t, st, "glm-4.7-flash", 128, 45, 10, 0, []resources.ResidentEntry{
		resident("big", 86, 900, true, 0), // 更旧、更大 → 先走
		resident("small", 5, 10, true, 0),
	})
	g := gatewayWith(t, cfg, st)

	if ok := g.ensureDS4Room(); !ok {
		t.Fatal("让位请求应已发出（返回 true）")
	}
	got := cap.models(t)
	if len(got) != 1 || got[0] != "big" {
		t.Fatalf("只卸够：应只卸 big，小的 small 留着，实得 %v", got)
	}
}

// 红线②：未托管进程（子端如实报 managed=false）绝不出现在让位计划里。
func TestX3Yield_NeverTargetsUnmanaged(t *testing.T) {
	cap, cfg := fakeX3Agent(t)
	st := store.NewStore()
	x3Snapshot(t, st, "manual-thing", 128, 20, 5, 0, []resources.ResidentEntry{
		resident("unmanaged@127.0.0.1:9001", 60, 5000, false, 0), // 未托管：最旧最大也不许动
		resident("ours", 4, 10, true, 0),
	})
	g := gatewayWith(t, cfg, st)

	g.ensureX3RoomForFile("/data/models/target.gguf", 80) // 需要腾 60G

	got := cap.models(t)
	for _, name := range got {
		if strings.HasPrefix(name, "unmanaged@") {
			t.Fatalf("红线②被破：未托管进程被点了名 %v（等于去接管别人的进程）", got)
		}
	}
	if len(got) != 1 || got[0] != "ours" {
		t.Fatalf("只该卸我们自己的 ours，实得 %v", got)
	}
}

// 红线①/③：在飞项与 pin 未到期项不进计划；一个都不能动时干脆不发请求。
func TestX3Yield_InflightAndPinProtected(t *testing.T) {
	cap, cfg := fakeX3Agent(t)
	st := store.NewStore()

	busy := resident("busy", 86, 9999, true, 2) // 在飞 + 最旧 + 最大
	pinned := resident("pinned", 17, 9998, true, 0)
	pinned.Pinned = true
	pinned.PinRemainS = 600
	idle := resident("idle", 4, 5, true, 0)
	x3Snapshot(t, st, "some-model", 128, 20, 5, 0, []resources.ResidentEntry{busy, pinned, idle})
	g := gatewayWith(t, cfg, st)

	g.ensureX3RoomForFile("/data/models/target.gguf", 40) // 需要腾 20G
	got := cap.models(t)
	if len(got) != 1 || got[0] != "idle" {
		t.Fatalf("在飞/pin 未到期者不该被点名，只该卸 idle，实得 %v", got)
	}

	// 只有一个在飞项 + 内存不够 → 什么都不发（宁可 507，也不杀活跃推理）
	cap2, cfg2 := fakeX3Agent(t)
	st2 := store.NewStore()
	x3Snapshot(t, st2, "some-model", 128, 20, 5, 0, []resources.ResidentEntry{busy})
	g2 := gatewayWith(t, cfg2, st2)
	g2.ensureX3RoomForFile("/data/models/target.gguf", 80)
	if n := cap2.count(); n != 0 {
		t.Fatalf("无可动作项时必须一个请求都不发，实得 %d 个", n)
	}
}

// 红线①（快照级）：快照报有在飞请求 → 让位整体跳过（既有铁律，保留）。
func TestX3Yield_SkipsWhenSnapshotInflight(t *testing.T) {
	cap, cfg := fakeX3Agent(t)
	st := store.NewStore()
	x3Snapshot(t, st, "some-model", 128, 10, 5, 3, []resources.ResidentEntry{resident("idle", 4, 10, true, 0)})
	g := gatewayWith(t, cfg, st)

	if ok := g.yieldX3To(g.snapshotFor("x3"), 70, "test"); ok {
		t.Fatal("快照报在飞请求时应拒绝让位（返回 false）")
	}
	if n := cap.count(); n != 0 {
		t.Fatalf("在飞时不得发任何卸载请求，实得 %d 个", n)
	}
}

// 兼容回退（如实写明）：子端未上报驻留账本（批 2 之前的版本）→ 退回旧口径"全卸"，
// 而不是装作"已按五档让位"。
func TestX3Yield_LegacyFallbackWhenLedgerAbsent(t *testing.T) {
	cap, cfg := fakeX3Agent(t)
	st := store.NewStore()
	x3Snapshot(t, st, "some-model", 128, 10, 5, 0, nil) // 无 resident[]
	g := gatewayWith(t, cfg, st)

	g.ensureX3RoomForFile("/data/models/target.gguf", 86)
	if n := cap.count(); n != 1 {
		t.Fatalf("账本缺席时应退回旧口径发一次全卸请求，实得 %d 个", n)
	}
	if got := cap.models(t); got != nil {
		t.Fatalf("旧口径 = 不带目标清单（全卸），实得 %v", got)
	}
}

// 无硬编码：地址必须来自 fleet.yaml（本用例把 x3 指向本地假服务，请求照样到达）。
func TestAgentURL_ComesFromConfig(t *testing.T) {
	_, cfg := fakeX3Agent(t)
	g := gatewayWith(t, cfg, store.NewStore())
	url := g.agentURLFor("x3", "/unload")
	node := cfg.Fleet["x3"]
	want := "http://" + node.Host + ":" + strconv.Itoa(node.Port) + "/unload"
	if url != want {
		t.Fatalf("地址应完全由配置拼出：want %s got %s", want, url)
	}
	if strings.Contains(url, "<worker-ip>") {
		t.Fatalf("不得出现硬编码地址：%s", url)
	}
}

// 无硬编码（源码级断言）：gateway 包的生产代码里不得再出现 <worker-ip> 或字面 http:// 地址。
// 注释行不参与扫描——注释里保留旧值是为了留下"改过什么"的痕迹（traceability），不是代码。
func TestNoHardcodedAgentURLInSource(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	scanned := 0
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		scanned++
		var code []string
		for _, line := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "//") {
				continue
			}
			code = append(code, line)
		}
		src := strings.Join(code, "\n")
		if strings.Contains(src, "<worker-ip>") {
			t.Fatalf("%s 的代码里仍含硬编码 IP <worker-ip>——地址必须来自 fleet.yaml", f)
		}
		if strings.Contains(src, `"http://10.`) || strings.Contains(src, `"http://192.`) {
			t.Fatalf("%s 的代码里仍含硬编码 http 地址", f)
		}
	}
	if scanned == 0 {
		t.Fatal("没有扫到任何生产源文件——断言无效")
	}
}
