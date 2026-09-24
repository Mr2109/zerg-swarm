package gateway

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/config"
	"github.com/Mr2109/zerg-swarm/core/internal/routepin"
	"github.com/Mr2109/zerg-swarm/core/internal/store"
)

// route_parity_test.go —— **改前/改后对拍**（评审铁律 ④：「未命中显式指定时，行为必须与今天逐字一致」）。
//
// 判据怎么成立（三件一起，缺一不成立）：
//
//	① 一批**固定输入**（模型 / 会话 / prompt / 必需能力 / 熔断态 / 排除本机态 —— 顺序写死），
//	   跑**同一条** `pickRoute`，把每一次的读数（host / port / URL / 错误原文）按序拼成一个指纹；
//	② 指纹与**金标准常量**（`routeParityGolden`）逐字比 —— 金标准取自**改动之前**那一版的同一批
//	   输入（现读可复算：见回执「改前/改后对拍」那一节的两次打印）；
//	③ 覆盖表指到**不存在的件**（`ZERG_ROUTE_PINS` → 临时目录里的一个空名）⇒ 「未命中」这一档
//	   在两侧都是同一档。
//
// 为什么不是「跑一遍老二进制比一比」：老制品只会起在 8082（主控在用）⇒ 起不了第二实例；
// 且 `go test` 要的是**环境无关**的可复跑判据（口令跑真机群 = 把门钉在别人的机器上）。
// 这一份对拍把「行为逐字一致」钉在**源码那一版**上，是这套限制下最硬的取法。
//
// 自证会红（变异）：把 `pickRoute` 里那道门挪到「未命中时也走一遍打分」或改任一优先级顺序，
// 指纹立刻与金标准不符 ⇒ 本用例红。
func routeParityConfig() *config.FleetConfig {
	return &config.FleetConfig{
		Models: map[string][]config.ModelCandidate{
			"m-双候选": {
				{Host: "Mr2109", Backend: "llama-server", File: "/m/a.gguf", MemGb: 21},
				{Host: "x3", Backend: "llama-server", File: "/m/a.gguf", MemGb: 21},
			},
			"m-三候选": {
				{Host: "Mr2109", Backend: "llama-server", File: "/m/b.gguf", MemGb: 17},
				{Host: "x3", Backend: "llama-server", File: "/m/b.gguf", MemGb: 17},
				{Host: "mini1", Backend: "llama-server", File: "/m/b.gguf", MemGb: 8},
			},
			"m-单候选": {
				{Host: "x3", Backend: "llama-server", File: "/m/c.gguf", MemGb: 5},
			},
		},
		Fleet: map[string]config.FleetNode{
			"Mr2109":  {Host: "127.0.0.1", Port: 8100},
			"x3":    {Host: "<worker-ip>", Port: 8100},
			"mini1": {Host: "<worker-ip>", Port: 8100},
		},
		Aliases: map[string]string{"别名": "m-双候选"},
	}
}

// routeParityFingerprint —— 跑固定输入批，出指纹（**不许有 map 遍历顺序 / 墙钟参与**：
// 读数按调用次序逐行拼，任何一个读数变了指纹就变）。
func routeParityFingerprint(t *testing.T) []string {
	t.Helper()
	var out []string
	rec := func(tag string, r *RouteResult, err error) {
		if err != nil {
			out = append(out, tag+" => ERR "+err.Error())
			return
		}
		if r == nil {
			out = append(out, tag+" => nil,nil")
			return
		}
		out = append(out, fmt.Sprintf("%s => %s:%d %s", tag, r.Host, r.Port, r.URL))
	}

	// 甲组：无覆盖表（空表 ⇒ 未命中那一档）。**每一枚 Gateway 各自起一套**（互不污染计数）。
	g := &Gateway{config: routeParityConfig(), roundRobin: map[string]int{}, failCounts: map[string]int{},
		failSince: map[string]time.Time{}, store: &store.Store{}}
	for i := 0; i < 4; i++ { // 连跑四趟：轮询/账本/cache-aware 的**顺序效应**也要落进指纹
		r, err := g.pickRoute("m-双候选", "", "")
		rec(fmt.Sprintf("双候选#%d", i), r, err)
	}
	for i := 0; i < 4; i++ {
		r, err := g.pickRoute("m-三候选", "", "")
		rec(fmt.Sprintf("三候选#%d", i), r, err)
	}
	r, err := g.pickRoute("别名", "", "") // 别名解析那一支
	rec("别名", r, err)
	r, err = g.pickRoute("m-双候选", "会话-甲", "") // 会话粘性：第一次绑定
	rec("粘性#首次", r, err)
	r, err = g.pickRoute("m-双候选", "会话-甲", "") // 第二次：直接复用绑定
	rec("粘性#复用", r, err)
	r, err = g.pickRoute("m-双候选", "", "前缀命中探针：一段固定的 prompt") // cache-aware 那一支
	rec("前缀", r, err)
	r, err = g.pickRoute("m-单候选", "", "")
	rec("单候选", r, err)
	r, err = g.pickRoute("m-双候选", "", "", "vision") // 必需能力 + 无能力快照 ⇒ 门槛那一支
	rec("必需能力", r, err)
	r, err = g.pickRoute("压根没有的模型", "", "")
	rec("不在表", r, err)

	// 乙组：某台熔断 ⇒ 择优要跳过它（熔断那一支）。
	gTrip := &Gateway{config: routeParityConfig(), roundRobin: map[string]int{},
		failCounts: map[string]int{"Mr2109": 99}, failSince: map[string]time.Time{"Mr2109": time.Now()},
		store: &store.Store{}}
	for i := 0; i < 3; i++ {
		r, err := gTrip.pickRoute("m-双候选", "", "")
		rec(fmt.Sprintf("熔断跳过#%d", i), r, err)
	}
	r, err = gTrip.pickRoute("m-三候选", "", "")
	rec("熔断跳过#三候选", r, err)

	// 丙组：排除本机模式（`ExcludeLocal`）那一支。
	gEx := &Gateway{config: routeParityConfig(), roundRobin: map[string]int{}, failCounts: map[string]int{},
		failSince: map[string]time.Time{}, store: &store.Store{}, excludeLocal: true}
	for i := 0; i < 3; i++ {
		r, err := gEx.pickRoute("m-双候选", "", "")
		rec(fmt.Sprintf("排除本机#%d", i), r, err)
	}

	return out
}

// routeParityGolden —— **金标准**：改动之前那一版在同一批输入上的读数（逐行）。
//
// 取法（现读可复算）：把本件与 `route_pin.go` 先摆在盘上、`gateway.go` 回到改动前那一版
// （`git stash push -- core/internal/gateway/gateway.go`），跑
// `ZERG_ROUTE_PARITY_PRINT=1 go test ./internal/gateway/ -run TestRouteDefault_ABParity`
// 打印出来逐字粘在这里；改动落回后再跑一次 ⇒ 两跑逐行相同 ⇒ 用例绿。
//
// ★ 金标准**只许在「默认择优那一段真的改了」时更新**（那一天必然是一次明说的行为变更）。
const routeParityGolden = `双候选#0 => Mr2109:8100 http://127.0.0.1:8100/infer
双候选#1 => Mr2109:8100 http://127.0.0.1:8100/infer
双候选#2 => Mr2109:8100 http://127.0.0.1:8100/infer
双候选#3 => Mr2109:8100 http://127.0.0.1:8100/infer
三候选#0 => Mr2109:8100 http://127.0.0.1:8100/infer
三候选#1 => Mr2109:8100 http://127.0.0.1:8100/infer
三候选#2 => Mr2109:8100 http://127.0.0.1:8100/infer
三候选#3 => Mr2109:8100 http://127.0.0.1:8100/infer
别名 => Mr2109:8100 http://127.0.0.1:8100/infer
粘性#首次 => Mr2109:8100 http://127.0.0.1:8100/infer
粘性#复用 => Mr2109:8100 http://127.0.0.1:8100/infer
前缀 => Mr2109:8100 http://127.0.0.1:8100/infer
单候选 => x3:8100 http://<worker-ip>:8100/infer
必需能力 => ERR capability gate rejected: model "m-双候选" on engine "llama.cpp" lacks proven capability "vision" (no_snapshot) — capability gate: no capability snapshot — cannot verify "vision" for engine "llama.cpp" (fail-closed) — blocked candidates: Mr2109(engine=llama.cpp):no_snapshot; x3(engine=llama.cpp):no_snapshot
不在表 => ERR model 压根没有的模型 not found in the routing table
熔断跳过#0 => x3:8100 http://<worker-ip>:8100/infer
熔断跳过#1 => x3:8100 http://<worker-ip>:8100/infer
熔断跳过#2 => x3:8100 http://<worker-ip>:8100/infer
熔断跳过#三候选 => x3:8100 http://<worker-ip>:8100/infer
排除本机#0 => Mr2109:8100 http://127.0.0.1:8100/infer
排除本机#1 => Mr2109:8100 http://127.0.0.1:8100/infer
排除本机#2 => Mr2109:8100 http://127.0.0.1:8100/infer`

// TestRouteDefault_ABParity —— 未命中显式指定时，行为与改动前**逐字一致**。
//
// `ZERG_ROUTE_PARITY_PRINT=1` ⇒ 只打印现跑指纹（供改前/改后各跑一次、人眼逐行比），不出断言。
func TestRouteDefault_ABParity(t *testing.T) {
	// 覆盖表指到一个**不存在**的件 ⇒ 「未命中」这一档（两侧同档）。
	t.Setenv(routepin.EnvPath, strings.TrimRight(t.TempDir(), "/")+"/不存在的覆盖表.json")
	got := strings.Join(routeParityFingerprint(t), "\n")
	if os.Getenv("ZERG_ROUTE_PARITY_PRINT") == "1" {
		fmt.Println("=== route parity fingerprint (begin) ===")
		fmt.Println(got)
		fmt.Println("=== route parity fingerprint (end) ===")
		return
	}
	if routeParityGolden == "REPLACED-BY-CAPTURE" {
		t.Fatal("金标准还没落（先跑一次 ZERG_ROUTE_PARITY_PRINT=1 取现读指纹）")
	}
	if got != routeParityGolden {
		want := strings.Split(routeParityGolden, "\n")
		have := strings.Split(got, "\n")
		bad := 0
		for i := 0; i < len(want) || i < len(have); i++ {
			w := "<缺>"
			h := "<缺>"
			if i < len(want) {
				w = want[i]
			}
			if i < len(have) {
				h = have[i]
			}
			if w != h {
				bad++
				t.Errorf("第 %d 行起就不一样了：\n  改前(金标准): %s\n  改后(现跑)  : %s", i+1, w, h)
				if bad >= 5 {
					t.Error("（相异行太多，先修前 5 行）")
					break
				}
			}
		}
		t.Fatalf("未命中显式指定时的行为**与改动前不一致**（共 %d 行现跑 · %d 行金标准）", len(have), len(want))
	}
}
